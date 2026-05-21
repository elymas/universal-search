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

// T-M5-013 [RED]: Empty fanout SSE event sequence tests.
// REQ-DEEP2-012: Empty fanout → IsEmpty=true, no section events.

func TestEmptyFanoutSSEEventSequence(t *testing.T) {
	handler := &DeepHandler{
		pipelineFn: func(ctx context.Context, cfg deepagent.Config, llmClient interface{}, req deepagent.PipelineRequest, fanoutFn interface{}) (deepagent.PipelineResult, error) {
			return deepagent.PipelineResult{
				RequestID: req.RequestID,
				IsEmpty:   true,
				AgentLog: []deepagent.AgentLogEntry{
					{Agent: deepagent.AgentResearcher, Outcome: "empty_corpus", DurationMs: 50},
				},
			}, nil
		},
	}

	body := `{"query": "no results query"}`
	req := httptest.NewRequest(http.MethodPost, "/deep?mode=agents", strings.NewReader(body))
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	raw := rec.Body.String()
	if !strings.Contains(raw, "event: agent_completed") {
		t.Error("expected agent_completed event for empty corpus")
	}
	if strings.Contains(raw, "event: section_start") {
		t.Error("section_start should not appear for empty corpus")
	}
}

func TestEmptyFanoutBufferedJSON(t *testing.T) {
	handler := &DeepHandler{
		pipelineFn: func(ctx context.Context, cfg deepagent.Config, llmClient interface{}, req deepagent.PipelineRequest, fanoutFn interface{}) (deepagent.PipelineResult, error) {
			return deepagent.PipelineResult{
				RequestID: req.RequestID,
				IsEmpty:   true,
				AgentLog: []deepagent.AgentLogEntry{
					{Agent: deepagent.AgentResearcher, Outcome: "empty_corpus", DurationMs: 50},
				},
			}, nil
		},
	}

	body := `{"query": "no results"}`
	req := httptest.NewRequest(http.MethodPost, "/deep?mode=agents", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if resp["status"] != "empty_corpus" {
		t.Errorf("status = %v, want empty_corpus", resp["status"])
	}
}

// T-M5-015 [RED]: Cancellation pipeline_cancelled terminal SSE test.
// REQ-DEEP2-009a: Context cancellation → pipeline_cancelled terminal event.

func TestCancellationSSEEmitsPipelineCancelled(t *testing.T) {
	handler := &DeepHandler{
		pipelineFn: func(ctx context.Context, cfg deepagent.Config, llmClient interface{}, req deepagent.PipelineRequest, fanoutFn interface{}) (deepagent.PipelineResult, error) {
			return deepagent.PipelineResult{
				RequestID: req.RequestID,
				AgentLog: []deepagent.AgentLogEntry{
					{Agent: deepagent.AgentResearcher, Outcome: "error", Error: "context cancelled"},
				},
			}, fmt.Errorf("pipeline: context cancelled before reviewer: context canceled")
		},
	}

	body := `{"query": "test cancel"}`
	req := httptest.NewRequest(http.MethodPost, "/deep?mode=agents", strings.NewReader(body))
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (SSE always 200)", rec.Code, http.StatusOK)
	}

	raw := rec.Body.String()
	if !strings.Contains(raw, "event: pipeline_failed") {
		t.Error("expected pipeline_failed event for cancelled context")
	}
}

// T-M5-017 [RED]: pipeline_failed terminal SSE test.
// REQ-DEEP2-009a: SSE active → terminal pipeline_failed event, HTTP stays 200.

func TestPipelineFailedSSEStays200(t *testing.T) {
	handler := &DeepHandler{
		pipelineFn: func(ctx context.Context, cfg deepagent.Config, llmClient interface{}, req deepagent.PipelineRequest, fanoutFn interface{}) (deepagent.PipelineResult, error) {
			return deepagent.PipelineResult{
				RequestID: req.RequestID,
				AgentLog: []deepagent.AgentLogEntry{
					{Agent: deepagent.AgentResearcher, Outcome: "success", DurationMs: 100},
					{Agent: deepagent.AgentReviewer, Outcome: "success", DurationMs: 200},
					{Agent: deepagent.AgentWriter, Outcome: "success", DurationMs: 300},
					{Agent: deepagent.AgentWriter, Outcome: "success", DurationMs: 350},
					{Agent: deepagent.AgentWriter, Outcome: "success", DurationMs: 400},
					{Agent: deepagent.AgentWriter, Outcome: "error", Error: "verifier_rejection_exhausted"},
				},
			}, fmt.Errorf("pipeline: max retry exhausted after 3 attempts: verifier_rejection_exhausted")
		},
	}

	body := `{"query": "test fail"}`
	req := httptest.NewRequest(http.MethodPost, "/deep?mode=agents", strings.NewReader(body))
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// REQ-DEEP2-009a: SSE active → HTTP stays 200.
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (SSE stays 200 on pipeline_failed)", rec.Code, http.StatusOK)
	}

	raw := rec.Body.String()
	if !strings.Contains(raw, "event: pipeline_failed") {
		t.Error("expected pipeline_failed terminal event")
	}
	if strings.Contains(raw, "event: section_start") {
		t.Error("section_start should not appear on pipeline failure")
	}
}
