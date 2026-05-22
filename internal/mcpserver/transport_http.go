package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// httpHandler creates an http.Handler for the Streamable HTTP transport.
// Returns an error if the configuration is invalid.
//
// REQ-MCP-005: Streamable HTTP transport per MCP 2025-06-18 spec.
// REQ-MCP-006: Origin validation and auth mode dispatch.
func (s *Server) httpHandler() (http.Handler, error) {
	// Validate configuration.
	if s.cfg.HTTP.BindPublic && s.cfg.HTTP.AuthMode == "none" {
		return nil, fmt.Errorf("mcpserver: bind_public=true requires auth_mode != none (security)")
	}

	mux := http.NewServeMux()
	path := s.cfg.HTTP.EndpointPath
	if path == "" {
		path = "/mcp"
	}

	h := &httpTransport{
		cfg:      s.cfg,
		sessions: make(map[string]*httpSession),
		logger:   s.logger(),
	}
	mux.HandleFunc(path, h.handle)

	return mux, nil
}

// httpTransport manages the Streamable HTTP transport state.
type httpTransport struct {
	cfg       Config
	mu        sync.Mutex
	sessions  map[string]*httpSession
	logger    *slog.Logger
	sessionID atomic.Int64
}

// httpSession represents an active MCP session.
type httpSession struct {
	id        string
	userID    string
	tenantID  string
	createdAt time.Time
}

// handle routes HTTP requests to POST or GET handlers.
func (h *httpTransport) handle(w http.ResponseWriter, r *http.Request) {
	// Validate MCP-Protocol-Version header.
	pv := r.Header.Get("MCP-Protocol-Version")
	if pv != "" && pv != "2025-06-18" {
		http.Error(w, "unsupported protocol version", http.StatusBadRequest)
		return
	}

	// Origin validation (REQ-MCP-006).
	if !h.validateOrigin(r) {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return
	}

	switch r.Method {
	case http.MethodPost:
		h.handlePost(w, r)
	case http.MethodGet:
		h.handleGet(w, r)
	case http.MethodDelete:
		h.handleDelete(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// validateOrigin checks the Origin header against the allowlist.
// REQ-MCP-006: missing Origin is allowed when no allowlist is configured.
func (h *httpTransport) validateOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	allowed := h.cfg.HTTP.AllowedOrigins

	// No allowlist configured: allow all.
	if len(allowed) == 0 {
		return true
	}

	// Missing Origin: allow (browser-sent on same-origin).
	if origin == "" {
		return true
	}

	// Check against allowlist (exact match, case-sensitive).
	for _, a := range allowed {
		if origin == a {
			return true
		}
	}

	return false
}

// handlePost processes JSON-RPC requests via POST.
// REQ-MCP-005: POST for JSON-RPC requests with Mcp-Session-Id.
func (h *httpTransport) handlePost(w http.ResponseWriter, r *http.Request) {
	// Auth mode dispatch.
	userID, tenantID := h.extractIdentity(r)

	// Read request body.
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse JSON-RPC request.
	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Get or create session.
	sessionID := r.Header.Get("Mcp-Session-Id")
	var session *httpSession
	if sessionID != "" {
		h.mu.Lock()
		session = h.sessions[sessionID]
		h.mu.Unlock()
	}
	if session == nil {
		sessionID = fmt.Sprintf("mcp-sess-%d", h.sessionID.Add(1))
		session = &httpSession{
			id:        sessionID,
			userID:    userID,
			tenantID:  tenantID,
			createdAt: time.Now(),
		}
		h.mu.Lock()
		h.sessions[sessionID] = session
		h.mu.Unlock()
	}

	// Process the request.
	method, _ := req["method"].(string)
	params, _ := req["params"].(map[string]any)

	result := h.processRequest(r.Context(), method, params, req)

	// Write response.
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Mcp-Session-Id", sessionID)

	respBytes, _ := json.Marshal(result)
	w.Write(respBytes)
}

// handleGet opens an SSE stream for the session.
// REQ-MCP-005: GET for SSE stream.
func (h *httpTransport) handleGet(w http.ResponseWriter, r *http.Request) {
	sessionID := r.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		http.Error(w, "Mcp-Session-Id required for SSE", http.StatusBadRequest)
		return
	}

	h.mu.Lock()
	session, ok := h.sessions[sessionID]
	h.mu.Unlock()

	if !ok || session == nil {
		http.Error(w, "unknown session", http.StatusNotFound)
		return
	}

	// Set SSE headers.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Mcp-Session-Id", sessionID)

	// Write a heartbeat event to confirm stream is open.
	fmt.Fprintf(w, "event: heartbeat\ndata: {}\n\n")
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// Block until context is done (client disconnects).
	<-r.Context().Done()
}

// handleDelete terminates an MCP session.
func (h *httpTransport) handleDelete(w http.ResponseWriter, r *http.Request) {
	sessionID := r.Header.Get("Mcp-Session-Id")
	if sessionID != "" {
		h.mu.Lock()
		delete(h.sessions, sessionID)
		h.mu.Unlock()
	}
	w.WriteHeader(http.StatusOK)
}

// extractIdentity extracts user/tenant identity from request headers.
func (h *httpTransport) extractIdentity(r *http.Request) (userID, tenantID string) {
	switch h.cfg.HTTP.AuthMode {
	case "trust-headers":
		userID = r.Header.Get("X-User-Id")
		tenantID = r.Header.Get("X-Tenant-Id")
	case "jwt":
		// JWT auth deferred to AUTH-001 integration.
		// For now, fall through to empty defaults.
	case "none":
		// No identity extraction.
	default:
		// Unknown auth mode: no identity.
	}
	return userID, tenantID
}

// processRequest handles JSON-RPC method dispatch.
func (h *httpTransport) processRequest(ctx context.Context, method string, params map[string]any, raw map[string]any) map[string]any {
	id := raw["id"]

	switch method {
	case "initialize":
		return h.handleInitialize(id, params)
	case "notifications/initialized":
		// Notification, no response needed, but we return empty for safety.
		return map[string]any{"jsonrpc": "2.0"}
	case "tools/list":
		return h.handleToolsList(id)
	default:
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"error":   map[string]any{"code": -32601, "message": "Method not found"},
		}
	}
}

// handleInitialize processes the initialize method.
func (h *httpTransport) handleInitialize(id any, params map[string]any) map[string]any {
	clientName := ""
	clientVersion := ""
	if ci, ok := params["clientInfo"].(map[string]any); ok {
		clientName, _ = ci["name"].(string)
		clientVersion, _ = ci["version"].(string)
	}

	return map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result": map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{},
			"serverInfo": map[string]any{
				"name":    "usearch-mcp",
				"version": "0.1.0",
			},
			"_meta": map[string]any{
				"client_name":    clientName,
				"client_version": clientVersion,
			},
		},
	}
}

// handleToolsList returns the list of registered tools.
func (h *httpTransport) handleToolsList(id any) map[string]any {
	return map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result": map[string]any{
			"tools": []any{},
		},
	}
}

// httpTransportAdapter adapts the httpTransport to the mcp.Transport interface.
// This is used when the MCP SDK is running in HTTP mode.
type httpTransportAdapter struct {
	addr     string
	listener net.Listener
	handler  http.Handler
}
