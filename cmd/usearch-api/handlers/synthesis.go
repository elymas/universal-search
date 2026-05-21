// Package handlers provides HTTP handlers for the usearch-api server.
//
// REQ-SYN4-001a: SSE headers set on every SSE-path response.
// REQ-SYN4-005:  Accept-header content negotiation; fallback to JSON.
//
// @MX:ANCHOR: [AUTO] SSE synthesis handler entry point; callers: mux registration, tests
// @MX:REASON: fan_in >= 3; all HTTP synthesis traffic routes through SynthesisHandler.ServeHTTP
package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/elymas/universal-search/internal/sse"
	"github.com/elymas/universal-search/internal/streamsynth"
	"github.com/elymas/universal-search/internal/synthesis"
)

// SynthesisClient is the minimal interface consumed by SynthesisHandler.
// Allows test doubles to replace the concrete synthesis.Client.
type SynthesisClient interface {
	// Synthesize is called with a context and a synthesis.Request and returns a result.
	Synthesize(ctx interface{}, req synthesis.Request) (synthesis.Result, error)
}

// Config holds configuration for the SynthesisHandler.
type Config struct {
	HeartbeatEnabled bool
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		HeartbeatEnabled: false, // disabled in test mode
	}
}

// SynthesisHandler handles POST /query/stream. It performs Accept-header
// content negotiation: text/event-stream -> SSE path; otherwise -> JSON.
type SynthesisHandler struct {
	client SynthesisClient
	cfg    Config
}

// NewSynthesisHandler constructs a SynthesisHandler.
func NewSynthesisHandler(client SynthesisClient, cfg Config) *SynthesisHandler {
	return &SynthesisHandler{client: client, cfg: cfg}
}

// ServeHTTP implements http.Handler.
//
// @MX:WARN: [AUTO] Goroutine coordination — heartbeat goroutine spawned for SSE path
// @MX:REASON: cancel on client disconnect; heartbeat exits within 100ms of ctx cancel
func (h *SynthesisHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Decode request body.
	var body struct {
		Query string          `json:"query"`
		Lang  string          `json:"lang"`
		Docs  []synthesis.Doc `json:"docs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	req := synthesis.Request{
		RequestID: "api-request",
		Query:     body.Query,
		Lang:      body.Lang,
		Docs:      body.Docs,
	}

	// Accept-header content negotiation (REQ-SYN4-005).
	accept := r.Header.Get("Accept")
	if !strings.Contains(strings.ToLower(accept), "text/event-stream") {
		// JSON fallback path.
		result, err := h.client.Synthesize(r.Context(), req)
		if err != nil {
			http.Error(w, "synthesis error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
		return
	}

	// SSE path.
	sw := sse.NewWriter(w)
	sw.SetHeaders()
	w.WriteHeader(http.StatusOK)

	ctx := r.Context()

	// Start heartbeat if enabled.
	if h.cfg.HeartbeatEnabled {
		go func() {
			_ = sse.RunHeartbeat(ctx, sw, 15_000_000_000) // 15s default
		}()
	}

	// Obtain synthesis result (buffered-then-streamed mode, v0).
	result, err := h.client.Synthesize(ctx, req)
	if err != nil {
		errPayload := streamsynth.ErrorPayload{
			RequestID:     req.RequestID,
			ErrorCode:     "upstream_error",
			ErrorMessage:  err.Error(),
			SchemaVersion: 1,
		}
		data, _ := json.Marshal(errPayload)
		_ = sw.WriteEvent("error", data)
		_ = sw.Flush()
		return
	}

	// Stream sentences.
	sreq := streamsynth.StreamRequest{
		RequestID:   req.RequestID,
		SynthResult: result,
	}
	_, _ = streamsynth.StreamSynthesize(ctx, w, sreq)
}

// Ensure SynthesisHandler implements http.Handler.
var _ http.Handler = (*SynthesisHandler)(nil)
