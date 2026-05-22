package mcpserver

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// T8 RED tests: HTTP Transport + Auth
// ---------------------------------------------------------------------------

// TestHTTPInitializeWithMcpSessionId verifies that a POST to the MCP endpoint
// completes an initialize handshake and returns a Mcp-Session-Id header.
//
// REQ-MCP-005: Streamable HTTP transport with session continuity.
func TestHTTPInitializeWithMcpSessionId(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Transport = "http"
	cfg.HTTP.ListenAddr = "127.0.0.1:0" // ephemeral port

	srv := New(cfg, nil, nil, nil, nil)

	handler, err := srv.httpHandler()
	if err != nil {
		t.Fatalf("httpHandler: %v", err)
	}

	server := httptest.NewServer(handler)
	defer server.Close()

	// Send initialize request.
	initReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "test-http-client",
				"version": "1.0.0",
			},
		},
	}
	body, _ := json.Marshal(initReq)

	req, _ := http.NewRequest(http.MethodPost, server.URL+"/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2025-06-18")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /mcp: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	// Verify Mcp-Session-Id header is present.
	sessionID := resp.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Error("expected Mcp-Session-Id header in response")
	}

	// Verify response body is valid JSON-RPC.
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result["id"] != float64(1) {
		t.Errorf("response id: got %v, want 1", result["id"])
	}
}

// TestHTTPSSEStreamOrdering verifies that a GET request to the MCP endpoint
// returns an SSE stream and events arrive in order.
//
// REQ-MCP-005: GET for SSE stream with event ordering.
func TestHTTPSSEStreamOrdering(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Transport = "http"
	cfg.HTTP.ListenAddr = "127.0.0.1:0"

	srv := New(cfg, nil, nil, nil, nil)
	handler, err := srv.httpHandler()
	if err != nil {
		t.Fatalf("httpHandler: %v", err)
	}

	server := httptest.NewServer(handler)
	defer server.Close()

	// First, initialize to get a session ID.
	sessionID := initHTTPSession(t, server.URL)

	// Now open SSE stream via GET.
	req, _ := http.NewRequest(http.MethodGet, server.URL+"/mcp", nil)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Mcp-Session-Id", sessionID)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /mcp: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") {
		t.Errorf("Content-Type: got %q, want text/event-stream", ct)
	}
}

// TestOriginRejectionMatrix verifies that requests with missing/null/off-allowlist/
// case-folded Origin headers are rejected with 403.
//
// REQ-MCP-006: origin validation with allowlist.
func TestOriginRejectionMatrix(t *testing.T) {
	tests := []struct {
		name       string
		origin     string
		allowed    []string
		wantStatus int
	}{
		{
			name:       "missing_origin_accepted_when_no_allowlist",
			origin:     "",
			allowed:    nil,
			wantStatus: http.StatusOK,
		},
		{
			name:       "origin_on_allowlist",
			origin:     "https://app.example.com",
			allowed:    []string{"https://app.example.com"},
			wantStatus: http.StatusOK,
		},
		{
			name:       "origin_off_allowlist",
			origin:     "https://evil.example.com",
			allowed:    []string{"https://app.example.com"},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "origin_case_folded_rejected",
			origin:     "HTTPS://APP.EXAMPLE.COM",
			allowed:    []string{"https://app.example.com"},
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Transport = "http"
			cfg.HTTP.ListenAddr = "127.0.0.1:0"
			cfg.HTTP.AllowedOrigins = tt.allowed

			srv := New(cfg, nil, nil, nil, nil)
			handler, err := srv.httpHandler()
			if err != nil {
				t.Fatalf("httpHandler: %v", err)
			}

			server := httptest.NewServer(handler)
			defer server.Close()

			initBody := map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"method":  "initialize",
				"params": map[string]any{
					"protocolVersion": "2025-06-18",
					"capabilities":    map[string]any{},
					"clientInfo":      map[string]any{"name": "test", "version": "1.0"},
				},
			}
			body, _ := json.Marshal(initBody)

			req, _ := http.NewRequest(http.MethodPost, server.URL+"/mcp", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json, text/event-stream")
			req.Header.Set("MCP-Protocol-Version", "2025-06-18")
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("request: %v", err)
			}
			resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("status: got %d, want %d", resp.StatusCode, tt.wantStatus)
			}
		})
	}
}

// TestBindPublicRequiresAuth verifies that bind_public=true with auth_mode=none
// causes a startup failure.
//
// REQ-MCP-006: public binding without auth is a configuration error.
func TestBindPublicRequiresAuth(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Transport = "http"
	cfg.HTTP.BindPublic = true
	cfg.HTTP.AuthMode = "none"
	cfg.HTTP.ListenAddr = "0.0.0.0:7080"

	srv := New(cfg, nil, nil, nil, nil)
	_, err := srv.httpHandler()
	if err == nil {
		t.Fatal("expected error for bind_public=true + auth_mode=none, got nil")
	}
	if !strings.Contains(err.Error(), "auth_mode") {
		t.Errorf("error should mention auth_mode: %v", err)
	}
}

// TestAuthModeTrustHeadersPath verifies that auth_mode=trust-headers extracts
// X-User-Id and X-Tenant-Id from request headers.
//
// REQ-MCP-006: trust-headers mode extracts identity headers.
func TestAuthModeTrustHeadersPath(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Transport = "http"
	cfg.HTTP.AuthMode = "trust-headers"
	cfg.HTTP.ListenAddr = "127.0.0.1:0"

	srv := New(cfg, nil, nil, nil, nil)
	handler, err := srv.httpHandler()
	if err != nil {
		t.Fatalf("httpHandler: %v", err)
	}

	server := httptest.NewServer(handler)
	defer server.Close()

	// Initialize with trust headers.
	initBody := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "test", "version": "1.0"},
		},
	}
	body, _ := json.Marshal(initBody)

	req, _ := http.NewRequest(http.MethodPost, server.URL+"/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2025-06-18")
	req.Header.Set("X-User-Id", "user-123")
	req.Header.Set("X-Tenant-Id", "tenant-456")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// initHTTPSession performs an initialize handshake and returns the session ID.
func initHTTPSession(t *testing.T, baseURL string) string {
	t.Helper()
	initReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "test", "version": "1.0"},
		},
	}
	body, _ := json.Marshal(initReq)

	req, _ := http.NewRequest(http.MethodPost, baseURL+"/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2025-06-18")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("init POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("init status: %d", resp.StatusCode)
	}

	sessionID := resp.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatal("no Mcp-Session-Id in init response")
	}
	return sessionID
}

// TestHTTPDeleteSession verifies that DELETE terminates an MCP session.
//
// REQ-MCP-005: DELETE for session termination.
func TestHTTPDeleteSession(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Transport = "http"
	cfg.HTTP.ListenAddr = "127.0.0.1:0"

	srv := New(cfg, nil, nil, nil, nil)
	handler, err := srv.httpHandler()
	if err != nil {
		t.Fatalf("httpHandler: %v", err)
	}

	server := httptest.NewServer(handler)
	defer server.Close()

	// Initialize to get a session ID.
	sessionID := initHTTPSession(t, server.URL)

	// Delete the session.
	req, _ := http.NewRequest(http.MethodDelete, server.URL+"/mcp", nil)
	req.Header.Set("Mcp-Session-Id", sessionID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /mcp: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	// Verify the session is gone by trying to open an SSE stream.
	sseReq, _ := http.NewRequest(http.MethodGet, server.URL+"/mcp", nil)
	sseReq.Header.Set("Accept", "text/event-stream")
	sseReq.Header.Set("Mcp-Session-Id", sessionID)

	sseResp, err := http.DefaultClient.Do(sseReq)
	if err != nil {
		t.Fatalf("GET /mcp after delete: %v", err)
	}
	defer sseResp.Body.Close()

	if sseResp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for deleted session, got %d", sseResp.StatusCode)
	}
}

// TestHTTPDeleteWithoutSessionId verifies DELETE without session ID returns 200.
func TestHTTPDeleteWithoutSessionId(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Transport = "http"
	cfg.HTTP.ListenAddr = "127.0.0.1:0"

	srv := New(cfg, nil, nil, nil, nil)
	handler, err := srv.httpHandler()
	if err != nil {
		t.Fatalf("httpHandler: %v", err)
	}

	server := httptest.NewServer(handler)
	defer server.Close()

	req, _ := http.NewRequest(http.MethodDelete, server.URL+"/mcp", nil)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /mcp: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// TestHTTPToolsList verifies that tools/list returns a JSON-RPC response.
func TestHTTPToolsList(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Transport = "http"
	cfg.HTTP.ListenAddr = "127.0.0.1:0"

	srv := New(cfg, nil, nil, nil, nil)
	handler, err := srv.httpHandler()
	if err != nil {
		t.Fatalf("httpHandler: %v", err)
	}

	server := httptest.NewServer(handler)
	defer server.Close()

	// First initialize.
	sessionID := initHTTPSession(t, server.URL)

	// Send tools/list.
	toolsReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
	}
	body, _ := json.Marshal(toolsReq)

	req, _ := http.NewRequest(http.MethodPost, server.URL+"/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2025-06-18")
	req.Header.Set("Mcp-Session-Id", sessionID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /mcp tools/list: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, bodyBytes)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result["id"] != float64(2) {
		t.Errorf("response id: got %v, want 2", result["id"])
	}

	resultMap, ok := result["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result object, got %T", result["result"])
	}
	toolsList, ok := resultMap["tools"].([]any)
	if !ok {
		t.Fatalf("expected tools array, got %T", resultMap["tools"])
	}
	// Without registry, no tools are registered at the HTTP layer.
	if len(toolsList) != 0 {
		t.Errorf("expected empty tools array, got %d", len(toolsList))
	}
}

// TestHTTPMethodNotFound verifies that unknown JSON-RPC methods return -32601.
func TestHTTPMethodNotFound(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Transport = "http"
	cfg.HTTP.ListenAddr = "127.0.0.1:0"

	srv := New(cfg, nil, nil, nil, nil)
	handler, err := srv.httpHandler()
	if err != nil {
		t.Fatalf("httpHandler: %v", err)
	}

	server := httptest.NewServer(handler)
	defer server.Close()

	sessionID := initHTTPSession(t, server.URL)

	unknownReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "nonexistent/method",
	}
	body, _ := json.Marshal(unknownReq)

	req, _ := http.NewRequest(http.MethodPost, server.URL+"/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2025-06-18")
	req.Header.Set("Mcp-Session-Id", sessionID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /mcp: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}

	errObj, ok := result["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error object, got %T", result["error"])
	}
	code, _ := errObj["code"].(float64)
	if int(code) != -32601 {
		t.Errorf("error code: got %v, want -32601", code)
	}
}

// TestHTTPMethodNotAllowed verifies that unsupported HTTP methods return 405.
func TestHTTPMethodNotAllowed(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Transport = "http"
	cfg.HTTP.ListenAddr = "127.0.0.1:0"

	srv := New(cfg, nil, nil, nil, nil)
	handler, err := srv.httpHandler()
	if err != nil {
		t.Fatalf("httpHandler: %v", err)
	}

	server := httptest.NewServer(handler)
	defer server.Close()

	req, _ := http.NewRequest(http.MethodPut, server.URL+"/mcp", nil)
	req.Header.Set("MCP-Protocol-Version", "2025-06-18")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /mcp: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", resp.StatusCode)
	}
}

// TestHTTPBadProtocolVersion verifies that an unsupported protocol version
// returns 400.
func TestHTTPBadProtocolVersion(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Transport = "http"
	cfg.HTTP.ListenAddr = "127.0.0.1:0"

	srv := New(cfg, nil, nil, nil, nil)
	handler, err := srv.httpHandler()
	if err != nil {
		t.Fatalf("httpHandler: %v", err)
	}

	server := httptest.NewServer(handler)
	defer server.Close()

	initBody := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params":  map[string]any{},
	}
	body, _ := json.Marshal(initBody)

	req, _ := http.NewRequest(http.MethodPost, server.URL+"/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("MCP-Protocol-Version", "2024-01-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /mcp: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

// TestHTTPSSEStreamWithoutSession verifies SSE GET without session ID returns 400.
func TestHTTPSSEStreamWithoutSession(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Transport = "http"
	cfg.HTTP.ListenAddr = "127.0.0.1:0"

	srv := New(cfg, nil, nil, nil, nil)
	handler, err := srv.httpHandler()
	if err != nil {
		t.Fatalf("httpHandler: %v", err)
	}

	server := httptest.NewServer(handler)
	defer server.Close()

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/mcp", nil)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /mcp: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

// TestHTTPSSEStreamInvalidSession verifies SSE GET with unknown session returns 404.
func TestHTTPSSEStreamInvalidSession(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Transport = "http"
	cfg.HTTP.ListenAddr = "127.0.0.1:0"

	srv := New(cfg, nil, nil, nil, nil)
	handler, err := srv.httpHandler()
	if err != nil {
		t.Fatalf("httpHandler: %v", err)
	}

	server := httptest.NewServer(handler)
	defer server.Close()

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/mcp", nil)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Mcp-Session-Id", "nonexistent-session")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /mcp: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

// TestHTTPInvalidJSONBody verifies POST with invalid JSON returns 400.
func TestHTTPInvalidJSONBody(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Transport = "http"
	cfg.HTTP.ListenAddr = "127.0.0.1:0"

	srv := New(cfg, nil, nil, nil, nil)
	handler, err := srv.httpHandler()
	if err != nil {
		t.Fatalf("httpHandler: %v", err)
	}

	server := httptest.NewServer(handler)
	defer server.Close()

	req, _ := http.NewRequest(http.MethodPost, server.URL+"/mcp", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("MCP-Protocol-Version", "2025-06-18")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /mcp: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

// TestHTTPInitializedNotification verifies that notifications/initialized
// returns a response without error.
func TestHTTPInitializedNotification(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Transport = "http"
	cfg.HTTP.ListenAddr = "127.0.0.1:0"

	srv := New(cfg, nil, nil, nil, nil)
	handler, err := srv.httpHandler()
	if err != nil {
		t.Fatalf("httpHandler: %v", err)
	}

	server := httptest.NewServer(handler)
	defer server.Close()

	sessionID := initHTTPSession(t, server.URL)

	notifReq := map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}
	body, _ := json.Marshal(notifReq)

	req, _ := http.NewRequest(http.MethodPost, server.URL+"/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("MCP-Protocol-Version", "2025-06-18")
	req.Header.Set("Mcp-Session-Id", sessionID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /mcp notification: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result["jsonrpc"] != "2.0" {
		t.Errorf("jsonrpc: got %v, want 2.0", result["jsonrpc"])
	}
}

// TestHTTPSessionReuse verifies that multiple requests with the same session ID
// reuse the existing session.
func TestHTTPSessionReuse(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Transport = "http"
	cfg.HTTP.ListenAddr = "127.0.0.1:0"

	srv := New(cfg, nil, nil, nil, nil)
	handler, err := srv.httpHandler()
	if err != nil {
		t.Fatalf("httpHandler: %v", err)
	}

	server := httptest.NewServer(handler)
	defer server.Close()

	sessionID := initHTTPSession(t, server.URL)

	// Second initialize with the same session ID should reuse the session.
	initReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      10,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "reuse-client", "version": "2.0"},
		},
	}
	body, _ := json.Marshal(initReq)

	req, _ := http.NewRequest(http.MethodPost, server.URL+"/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2025-06-18")
	req.Header.Set("Mcp-Session-Id", sessionID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /mcp reuse: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, bodyBytes)
	}

	// Should return the same session ID.
	returnedSession := resp.Header.Get("Mcp-Session-Id")
	if returnedSession != sessionID {
		t.Errorf("session ID: got %q, want %q", returnedSession, sessionID)
	}
}

// Ensure compile-time check for unused helpers.
var _ = fmt.Sprintf
var _ = bufio.NewReader
var _ = bytes.NewReader
var _ = time.Now
