package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/elymas/universal-search/internal/deepagent"
	"github.com/elymas/universal-search/internal/deepreport"
	"github.com/elymas/universal-search/internal/sse"
	"github.com/elymas/universal-search/internal/streamsynth"
)

// DeepPipelineFn is the function signature for running the multi-agent pipeline.
// Abstracted for testability — production callers pass a wrapper around RunPipeline.
type DeepPipelineFn func(ctx context.Context, cfg deepagent.Config, llmClient interface{}, req deepagent.PipelineRequest, fanoutFn interface{}) (deepagent.PipelineResult, error)

// DeepStormFn is the function signature for generating a STORM report.
type DeepStormFn interface {
	GenerateReport(ctx context.Context, req deepreport.Request) (deepreport.Report, error)
}

// deepRequestBody is the JSON body for POST /deep.
type deepRequestBody struct {
	Query string `json:"query"`
	Lang  string `json:"lang"`
}

// DeepHandler handles POST /deep with ?mode= query parameter.
// REQ-DEEP2-001: mode=agents routes to multi-agent pipeline.
// REQ-DEEP2-011: mode=storm (default) uses STORM sidecar unchanged.
type DeepHandler struct {
	pipelineFn  DeepPipelineFn
	stormClient DeepStormFn
}

// NewDeepHandler constructs a DeepHandler with the given pipeline function and storm client.
func NewDeepHandler(pipelineFn DeepPipelineFn, stormClient DeepStormFn) *DeepHandler {
	return &DeepHandler{
		pipelineFn:  pipelineFn,
		stormClient: stormClient,
	}
}

// ServeHTTP implements http.Handler.
// REQ-DEEP2-001: POST /deep?mode=agents routing.
// REQ-DEEP2-011: /deep?mode=storm unchanged. Default = storm.
func (h *DeepHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Decode request body.
	var body deepRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "storm" // REQ-DEEP2-011: default = storm.
	}

	switch mode {
	case "agents":
		h.handleAgentsMode(w, r, body)
	case "storm":
		h.handleStormMode(w, r, body)
	default:
		http.Error(w, fmt.Sprintf("unknown mode: %s", mode), http.StatusBadRequest)
	}
}

// handleAgentsMode processes the multi-agent pipeline.
// REQ-DEEP2-001: Routes to multi-agent pipeline.
// REQ-DEEP2-009a: SSE active → terminal pipeline_failed event, HTTP stays 200.
// REQ-DEEP2-009b: Buffered → HTTP 503 + JSON body.
// REQ-DEEP2-010: ?stream=false forces buffered JSON even with SSE Accept.
func (h *DeepHandler) handleAgentsMode(w http.ResponseWriter, r *http.Request, body deepRequestBody) {
	requestID := fmt.Sprintf("deep-%d", r.Context().Value("request_id"))

	streamParam := r.URL.Query().Get("stream")
	acceptSSE := strings.Contains(strings.ToLower(r.Header.Get("Accept")), "text/event-stream")
	wantSSE := streamParam != "false" && acceptSSE

	req := deepagent.PipelineRequest{
		RequestID: requestID,
		Query:     body.Query,
		Lang:      body.Lang,
	}

	cfg := deepagent.DefaultConfig()

	result, err := h.pipelineFn(r.Context(), cfg, nil, req, nil)
	if err != nil {
		h.handlePipelineError(w, r, result, err, wantSSE, requestID)
		return
	}

	if result.IsEmpty {
		if wantSSE {
			h.writeSSEEmptyResponse(w, requestID)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"request_id": requestID,
				"status":     "empty_corpus",
			})
		}
		return
	}

	if wantSSE {
		h.streamResultSSE(w, r, result)
	} else {
		h.writeBufferedResult(w, result)
	}
}

// handleStormMode processes the STORM sidecar report.
// REQ-DEEP2-011: /deep?mode=storm unchanged from SPEC-DEEP-001.
func (h *DeepHandler) handleStormMode(w http.ResponseWriter, r *http.Request, body deepRequestBody) {
	if h.stormClient == nil {
		http.Error(w, "storm mode not configured", http.StatusServiceUnavailable)
		return
	}

	acceptSSE := strings.Contains(strings.ToLower(r.Header.Get("Accept")), "text/event-stream")

	req := deepreport.Request{
		RequestID: fmt.Sprintf("storm-%d", r.Context().Value("request_id")),
		Query:     body.Query,
		Lang:      body.Lang,
	}

	report, err := h.stormClient.GenerateReport(r.Context(), req)
	if err != nil {
		http.Error(w, "storm report error", http.StatusInternalServerError)
		return
	}

	if acceptSSE {
		sw := sse.NewWriter(w)
		sw.SetHeaders()
		w.WriteHeader(http.StatusOK)
		_, _ = streamsynth.StreamLongFormReport(r.Context(), sw, report.RequestID, report)
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(report)
	}
}

// handlePipelineError handles pipeline errors per REQ-DEEP2-009a/009b.
func (h *DeepHandler) handlePipelineError(w http.ResponseWriter, r *http.Request, result deepagent.PipelineResult, pipelineErr error, wantSSE bool, requestID string) {
	if wantSSE {
		// REQ-DEEP2-009a: SSE active → terminal pipeline_failed event, HTTP stays 200.
		sw := sse.NewWriter(w)
		sw.SetHeaders()
		w.WriteHeader(http.StatusOK)

		payload := streamsynth.PipelineFailedPayload{
			RequestID:     requestID,
			FailedAgent:   h.extractFailedAgent(result),
			Reason:        pipelineErr.Error(),
			Attempts:      h.countAttempts(result),
			RetryCount:    h.countRetries(result),
			SchemaVersion: 1,
		}
		_ = streamsynth.EmitAgentEvent(sw, "pipeline_failed", payload)
	} else {
		// REQ-DEEP2-009b: Buffered → HTTP 503 + JSON body.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"request_id":   requestID,
			"error":        pipelineErr.Error(),
			"failed_agent": h.extractFailedAgent(result),
		})
	}
}

// streamResultSSE streams the pipeline result as SSE events.
func (h *DeepHandler) streamResultSSE(w http.ResponseWriter, r *http.Request, result deepagent.PipelineResult) {
	sw := sse.NewWriter(w)
	sw.SetHeaders()
	w.WriteHeader(http.StatusOK)

	// Emit agent lifecycle events from log.
	for _, entry := range result.AgentLog {
		_ = streamsynth.EmitAgentEvent(sw, "agent_completed", streamsynth.AgentCompletedPayload{
			RequestID:     result.RequestID,
			Agent:         string(entry.Agent),
			Outcome:       entry.Outcome,
			DurationMs:    entry.DurationMs,
			CostUSD:       entry.CostUSD,
			SchemaVersion: 1,
		})
	}

	// Stream long-form content from draft.
	if result.Draft != nil {
		_, _ = streamsynth.StreamLongFormReport(r.Context(), sw, result.RequestID, *result.Draft)
	}
}

// writeBufferedResult writes the pipeline result as JSON.
func (h *DeepHandler) writeBufferedResult(w http.ResponseWriter, result deepagent.PipelineResult) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"request_id": result.RequestID,
		"agent_log":  result.AgentLog,
		"draft":      result.Draft,
	})
}

// writeSSEEmptyResponse writes an empty corpus SSE response.
func (h *DeepHandler) writeSSEEmptyResponse(w http.ResponseWriter, requestID string) {
	sw := sse.NewWriter(w)
	sw.SetHeaders()
	w.WriteHeader(http.StatusOK)

	_ = streamsynth.EmitAgentEvent(sw, "agent_completed", streamsynth.AgentCompletedPayload{
		RequestID:     requestID,
		Agent:         "researcher",
		Outcome:       "empty_corpus",
		SchemaVersion: 1,
	})
}

// extractFailedAgent returns the agent name from the last error log entry.
func (h *DeepHandler) extractFailedAgent(result deepagent.PipelineResult) string {
	for i := len(result.AgentLog) - 1; i >= 0; i-- {
		if result.AgentLog[i].Outcome == "error" {
			return string(result.AgentLog[i].Agent)
		}
	}
	return "unknown"
}

// countAttempts returns the total number of Writer attempts from the log.
func (h *DeepHandler) countAttempts(result deepagent.PipelineResult) int {
	count := 0
	for _, entry := range result.AgentLog {
		if entry.Agent == deepagent.AgentWriter {
			count++
		}
	}
	return count
}

// countRetries returns the number of retries (attempts - 1).
func (h *DeepHandler) countRetries(result deepagent.PipelineResult) int {
	attempts := h.countAttempts(result)
	if attempts > 0 {
		return attempts - 1
	}
	return 0
}
