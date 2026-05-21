package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/elymas/universal-search/internal/deepagent"
)

// T-M5-011 [RED]: Buffered fallback tests.
// REQ-DEEP2-010: Buffered fallback when ?stream=false or no SSE Accept.

func TestBufferedFallbackNoSSEAccept(t *testing.T) {
	// No Accept: text/event-stream → buffered JSON.
	handler := &DeepHandler{
		pipelineFn: func(ctx context.Context, cfg deepagent.Config, llmClient interface{}, req deepagent.PipelineRequest, fanoutFn interface{}) (deepagent.PipelineResult, error) {
			return deepagent.PipelineResult{
				RequestID: req.RequestID,
				Draft: &deepagent.WriterDraft{
					Sections:  []deepagent.DraftSection{{SectionIndex: 0, Heading: "H", Text: "T"}},
					Citations: []deepagent.DraftCitation{},
				},
				AgentLog: []deepagent.AgentLogEntry{
					{Agent: deepagent.AgentResearcher, Outcome: "success", DurationMs: 100},
				},
			}, nil
		},
	}

	body := `{"query": "test"}`
	req := httptest.NewRequest(http.MethodPost, "/deep?mode=agents", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No Accept header → defaults to JSON.

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if resp["request_id"] == nil {
		t.Error("expected request_id in JSON response")
	}
}

func TestBufferedFallbackStreamFalseOverride(t *testing.T) {
	// ?stream=false overrides Accept: text/event-stream.
	handler := &DeepHandler{
		pipelineFn: func(ctx context.Context, cfg deepagent.Config, llmClient interface{}, req deepagent.PipelineRequest, fanoutFn interface{}) (deepagent.PipelineResult, error) {
			return deepagent.PipelineResult{
				RequestID: req.RequestID,
				Draft:     &deepagent.WriterDraft{},
				AgentLog:  []deepagent.AgentLogEntry{},
			}, nil
		},
	}

	body := `{"query": "test"}`
	req := httptest.NewRequest(http.MethodPost, "/deep?mode=agents&stream=false", strings.NewReader(body))
	req.Header.Set("Accept", "text/event-stream") // SSE accept, but stream=false overrides.
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json (stream=false override)", ct)
	}
}

func TestBufferedErrorReturns503(t *testing.T) {
	// REQ-DEEP2-009b: Non-SSE buffered path returns 503 on pipeline failure.
	handler := &DeepHandler{
		pipelineFn: func(ctx context.Context, cfg deepagent.Config, llmClient interface{}, req deepagent.PipelineRequest, fanoutFn interface{}) (deepagent.PipelineResult, error) {
			return deepagent.PipelineResult{
				RequestID: req.RequestID,
				AgentLog: []deepagent.AgentLogEntry{
					{Agent: deepagent.AgentWriter, Outcome: "error", Error: "exhausted"},
				},
			}, fmt.Errorf("pipeline: max retry exhausted after 3 attempts: verifier_rejection_exhausted")
		},
	}

	body := `{"query": "test"}`
	req := httptest.NewRequest(http.MethodPost, "/deep?mode=agents", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if resp["error"] == nil {
		t.Error("expected error field in 503 response")
	}
	if resp["failed_agent"] == nil {
		t.Error("expected failed_agent field in 503 response")
	}
}
