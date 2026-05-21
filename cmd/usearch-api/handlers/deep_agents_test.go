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
	"github.com/elymas/universal-search/internal/deepreport"
)

// T-M5-009 [RED]: Handler routing + mode dispatch + storm-default tests.
// REQ-DEEP2-001: POST /deep?mode=agents routes to multi-agent pipeline.
// REQ-DEEP2-011: /deep?mode=storm unchanged. Default = storm.

// mockDeepPipelineFn captures calls for handler testing.
type mockDeepPipelineFn struct {
	called bool
	result deepagent.PipelineResult
	err    error
}

func (m *mockDeepPipelineFn) RunPipeline(ctx context.Context, cfg deepagent.Config, llmClient interface{}, req deepagent.PipelineRequest, fanoutFn interface{}) (deepagent.PipelineResult, error) {
	m.called = true
	return m.result, m.err
}

// mockStormClient returns a canned deepreport.Report.
type mockStormClient struct {
	report deepreport.Report
	err    error
}

func TestDeepEndpointModeAgentsSSE(t *testing.T) {
	// POST /deep?mode=agents with Accept: text/event-stream should produce SSE events.
	draft := deepagent.WriterDraft{
		Sections: []deepagent.DraftSection{
			{SectionIndex: 0, Heading: "Test", Text: "Body [1]", CitationMarkers: []int{1}},
		},
		Citations: []deepagent.DraftCitation{
			{Marker: 1, DocID: "d1", URL: "https://example.com", Title: "Doc 1"},
		},
		Model:    "test-model",
		Provider: "test-provider",
		CostUSD:  0.01,
	}

	handler := &DeepHandler{
		pipelineFn: func(ctx context.Context, cfg deepagent.Config, llmClient interface{}, req deepagent.PipelineRequest, fanoutFn interface{}) (deepagent.PipelineResult, error) {
			return deepagent.PipelineResult{
				RequestID: req.RequestID,
				AgentLog: []deepagent.AgentLogEntry{
					{Agent: deepagent.AgentResearcher, Outcome: "success", DurationMs: 500},
				},
				Draft: &draft,
			}, nil
		},
		stormClient: nil,
	}

	body := `{"query": "test query", "lang": "en"}`
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
		t.Error("expected agent_completed event in SSE output")
	}
	if !strings.Contains(raw, "event: section_start") {
		t.Error("expected section_start event in SSE output")
	}
}

func TestDeepEndpointModeStormDefault(t *testing.T) {
	// POST /deep (no mode param) should use storm mode (default).
	// This is a JSON response test since storm returns a full report.
	report := deepreport.Report{
		RequestID: "storm-test",
		Title:     "Test Report",
		Sections:  []deepreport.Section{},
		Citations: []deepreport.Citation{},
		Model:     "storm-model",
		Provider:  "storm-provider",
		CostUSD:   0.02,
	}

	handler := &DeepHandler{
		pipelineFn: nil, // agents mode should not be called
		stormClient: &mockStormReportFn{report: report},
	}

	body := `{"query": "test query", "lang": "en"}`
	req := httptest.NewRequest(http.MethodPost, "/deep", strings.NewReader(body))
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
	if resp["title"] != "Test Report" {
		t.Errorf("title = %v, want Test Report", resp["title"])
	}
}

func TestDeepEndpointModeAgentsBufferedError(t *testing.T) {
	// REQ-DEEP2-009b: Non-SSE buffered path returns 503 on pipeline failure.
	handler := &DeepHandler{
		pipelineFn: func(ctx context.Context, cfg deepagent.Config, llmClient interface{}, req deepagent.PipelineRequest, fanoutFn interface{}) (deepagent.PipelineResult, error) {
			return deepagent.PipelineResult{
				RequestID: req.RequestID,
				AgentLog: []deepagent.AgentLogEntry{
					{Agent: deepagent.AgentWriter, Outcome: "error", Error: "verifier_rejection_exhausted"},
				},
			}, fmt.Errorf("pipeline: max retry exhausted after 3 attempts: verifier_rejection_exhausted")
		},
		stormClient: nil,
	}

	body := `{"query": "test query", "lang": "en"}`
	req := httptest.NewRequest(http.MethodPost, "/deep?mode=agents", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No Accept: text/event-stream → buffered path.

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestDeepEndpointModeAgentsSSEStreamFalse(t *testing.T) {
	// REQ-DEEP2-010: ?stream=false forces buffered JSON even with SSE Accept.
	handler := &DeepHandler{
		pipelineFn: func(ctx context.Context, cfg deepagent.Config, llmClient interface{}, req deepagent.PipelineRequest, fanoutFn interface{}) (deepagent.PipelineResult, error) {
			return deepagent.PipelineResult{
				RequestID: req.RequestID,
				Draft: &deepagent.WriterDraft{
					Sections: []deepagent.DraftSection{
						{SectionIndex: 0, Heading: "H", Text: "T", CitationMarkers: []int{}},
					},
				},
			}, nil
		},
		stormClient: nil,
	}

	body := `{"query": "test query"}`
	req := httptest.NewRequest(http.MethodPost, "/deep?mode=agents&stream=false", strings.NewReader(body))
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Should be JSON, not SSE.
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestDeepEndpointOnlyPostAllowed(t *testing.T) {
	handler := &DeepHandler{}

	req := httptest.NewRequest(http.MethodGet, "/deep?mode=agents", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

// mockStormReportFn is a test double for storm report generation.
type mockStormReportFn struct {
	report deepreport.Report
	err    error
}

func (m *mockStormReportFn) GenerateReport(ctx context.Context, req deepreport.Request) (deepreport.Report, error) {
	return m.report, m.err
}
