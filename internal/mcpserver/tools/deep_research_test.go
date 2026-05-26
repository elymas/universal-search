package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/deepagent"
	"github.com/elymas/universal-search/internal/deepagent/costguard"
)

// ---------------------------------------------------------------------------
// Stubs and mocks for deep_research tests
// ---------------------------------------------------------------------------

// mockCapChecker records calls and returns a configurable result.
type mockCapChecker struct {
	mu         sync.Mutex
	called     bool
	calledWith capCallArgs
	result     costguard.CapResult
	err        error
}

type capCallArgs struct {
	tenantID string
	userID   string
	costUSD  float64
}

func (m *mockCapChecker) EvaluateAtomic(ctx context.Context, tenantID, userID string, costUSD float64) (costguard.CapResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.called = true
	m.calledWith = capCallArgs{tenantID: tenantID, userID: userID, costUSD: costUSD}
	return m.result, m.err
}

func (m *mockCapChecker) wasCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.called
}

// mockPipelineRunner records calls and returns a configurable result.
type mockPipelineRunner struct {
	mu     sync.Mutex
	called bool
	result deepagent.PipelineResult
	err    error
}

func (m *mockPipelineRunner) run(ctx context.Context, req deepagent.PipelineRequest) (deepagent.PipelineResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.called = true
	return m.result, m.err
}

// mockNotifier captures progress notifications.
type mockNotifier struct {
	mu            sync.Mutex
	notifications []notifyEntry
}

type notifyEntry struct {
	method string
	data   map[string]any
}

func (m *mockNotifier) notify(method string, data map[string]any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifications = append(m.notifications, notifyEntry{method: method, data: data})
}

func (m *mockNotifier) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.notifications)
}

// mockAuditWriter captures audit log lines.
type mockAuditWriter struct {
	mu    sync.Mutex
	lines []string
}

func (m *mockAuditWriter) write(line string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lines = append(m.lines, line)
	return nil
}

func (m *mockAuditWriter) getLines() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.lines))
	copy(out, m.lines)
	return out
}

// ---------------------------------------------------------------------------
// T7 RED tests
// ---------------------------------------------------------------------------

// TestDeepResearchRoutesCapMiddleware verifies that DeepResearchHandler calls
// CapChecker.EvaluateAtomic before running the pipeline.
//
// REQ-MCP-009: deep_research routes through costguard cap middleware.
func TestDeepResearchRoutesCapMiddleware(t *testing.T) {
	capCheck := &mockCapChecker{
		result: costguard.CapResult{Allowed: true, RemainingCalls: 10, RemainingUSD: 5.0},
	}
	pipeline := &mockPipelineRunner{
		result: deepagent.PipelineResult{
			RequestID: "test-req-1",
			Draft: &deepagent.WriterDraft{
				Sections: []deepagent.DraftSection{
					{SectionIndex: 0, Heading: "Summary", Text: "Test summary"},
				},
			},
		},
	}
	notif := &mockNotifier{}
	audit := &mockAuditWriter{}

	handler := DeepResearchHandler(capCheck, pipeline.run, notif.notify, audit.write)

	_, _, err := handler(context.Background(), nil, DeepResearchInput{Query: "test deep research"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !capCheck.wasCalled() {
		t.Fatal("CapChecker.EvaluateAtomic was not called")
	}
	if capCheck.calledWith.tenantID == "" {
		t.Error("expected non-empty tenantID in cap check call")
	}
}

// TestDeepResearchSharesQuotaWithHTTP verifies that when CapChecker denies a
// request, the MCP tool returns error code -32000 (usearch.cap_exceeded).
//
// REQ-MCP-009: MCP deep_research shares quota counters with HTTP /deep endpoint.
func TestDeepResearchSharesQuotaWithHTTP(t *testing.T) {
	capCheck := &mockCapChecker{
		result: costguard.CapResult{
			Allowed:  false,
			Exceeded: costguard.DimensionCalls,
		},
	}
	pipeline := &mockPipelineRunner{}
	notif := &mockNotifier{}
	audit := &mockAuditWriter{}

	handler := DeepResearchHandler(capCheck, pipeline.run, notif.notify, audit.write)

	_, _, err := handler(context.Background(), nil, DeepResearchInput{Query: "test"})
	if err == nil {
		t.Fatal("expected error when cap exceeded, got nil")
	}

	mcpErr, ok := err.(*MCPError)
	if !ok {
		t.Fatalf("expected *MCPError, got %T", err)
	}
	if mcpErr.Code != ErrCodeCapExceeded {
		t.Errorf("error code: got %d, want %d", mcpErr.Code, ErrCodeCapExceeded)
	}
	if mcpErr.Namespace != "usearch.cap_exceeded" {
		t.Errorf("namespace: got %q, want %q", mcpErr.Namespace, "usearch.cap_exceeded")
	}

	// Pipeline should NOT have been called when capped.
	if pipeline.called {
		t.Error("pipeline should not be called when cap is exceeded")
	}
}

// TestDeepResearchProgressStageCount verifies that the handler emits at least 4
// progress notifications (notifications/message) during pipeline execution.
//
// REQ-MCP-010: progress events via notifications/message at each pipeline stage boundary.
func TestDeepResearchProgressStageCount(t *testing.T) {
	capCheck := &mockCapChecker{
		result: costguard.CapResult{Allowed: true, RemainingCalls: 10},
	}
	pipeline := &mockPipelineRunner{
		result: deepagent.PipelineResult{
			RequestID: "test-req-2",
			Draft: &deepagent.WriterDraft{
				Sections: []deepagent.DraftSection{
					{SectionIndex: 0, Heading: "Summary", Text: "Test summary"},
				},
			},
		},
	}
	notif := &mockNotifier{}
	audit := &mockAuditWriter{}

	handler := DeepResearchHandler(capCheck, pipeline.run, notif.notify, audit.write)

	_, _, err := handler(context.Background(), nil, DeepResearchInput{Query: "test stages"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stageCount := notif.count()
	if stageCount < 4 {
		t.Errorf("expected >= 4 progress notifications, got %d", stageCount)
	}

	// Verify all notifications use the correct method.
	for i, n := range notif.notifications {
		if n.method != "notifications/message" {
			t.Errorf("notification[%d]: method got %q, want %q", i, n.method, "notifications/message")
		}
	}
}

// TestDeepResearchAuditLineSchema verifies that the audit logger emits a JSON
// line conforming to the decision-event schema.
//
// REQ-DEEP4-010: audit line schema conformance.
func TestDeepResearchAuditLineSchema(t *testing.T) {
	capCheck := &mockCapChecker{
		result: costguard.CapResult{Allowed: true, RemainingCalls: 10},
	}
	pipeline := &mockPipelineRunner{
		result: deepagent.PipelineResult{
			RequestID: "test-req-3",
			Draft: &deepagent.WriterDraft{
				Sections: []deepagent.DraftSection{
					{SectionIndex: 0, Heading: "Summary", Text: "Test summary"},
				},
			},
		},
	}
	notif := &mockNotifier{}
	audit := &mockAuditWriter{}

	handler := DeepResearchHandler(capCheck, pipeline.run, notif.notify, audit.write)

	_, _, err := handler(context.Background(), nil, DeepResearchInput{Query: "test audit"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := audit.getLines()
	if len(lines) == 0 {
		t.Fatal("expected at least one audit line, got 0")
	}

	// Parse the first audit line as JSON.
	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("audit line is not valid JSON: %v\nline: %q", err, lines[0])
	}

	// Verify required schema fields.
	requiredFields := []string{
		"event_type", "tool_name", "mcp_transport",
		"client_name", "timestamp", "request_id",
	}
	for _, field := range requiredFields {
		if _, ok := entry[field]; !ok {
			t.Errorf("missing required field %q in audit line", field)
		}
	}

	// Verify event_type value.
	if et, _ := entry["event_type"].(string); et != "mcp.tool_call" {
		t.Errorf("event_type: got %q, want %q", et, "mcp.tool_call")
	}

	// Verify tool_name value.
	if tn, _ := entry["tool_name"].(string); tn != "deep_research" {
		t.Errorf("tool_name: got %q, want %q", tn, "deep_research")
	}
}

// TestDeepResearchEmptyQueryRejected verifies that an empty query returns an
// input validation error.
func TestDeepResearchEmptyQueryRejected(t *testing.T) {
	capCheck := &mockCapChecker{
		result: costguard.CapResult{Allowed: true, RemainingCalls: 10},
	}
	pipeline := &mockPipelineRunner{}
	notif := &mockNotifier{}
	audit := &mockAuditWriter{}

	handler := DeepResearchHandler(capCheck, pipeline.run, notif.notify, audit.write)

	_, _, err := handler(context.Background(), nil, DeepResearchInput{Query: ""})
	if err == nil {
		t.Fatal("expected error for empty query, got nil")
	}

	mcpErr, ok := err.(*MCPError)
	if !ok {
		t.Fatalf("expected *MCPError, got %T", err)
	}
	if mcpErr.Code != -32602 {
		t.Errorf("error code: got %d, want -32602 (invalid params)", mcpErr.Code)
	}
}

// TestDeepResearchPipelineErrorMapped verifies that a pipeline error is mapped
// to an MCP error.
func TestDeepResearchPipelineErrorMapped(t *testing.T) {
	capCheck := &mockCapChecker{
		result: costguard.CapResult{Allowed: true, RemainingCalls: 10},
	}
	pipeline := &mockPipelineRunner{
		err: fmt.Errorf("pipeline: researcher failed: context deadline exceeded"),
	}
	notif := &mockNotifier{}
	audit := &mockAuditWriter{}

	handler := DeepResearchHandler(capCheck, pipeline.run, notif.notify, audit.write)

	_, _, err := handler(context.Background(), nil, DeepResearchInput{Query: "test error"})
	if err == nil {
		t.Fatal("expected error when pipeline fails, got nil")
	}
}

// TestDeepResearchOutputSchema verifies the output structure matches
// DeepResearchOutput schema.
func TestDeepResearchOutputSchema(t *testing.T) {
	capCheck := &mockCapChecker{
		result: costguard.CapResult{Allowed: true, RemainingCalls: 10},
	}
	pipeline := &mockPipelineRunner{
		result: deepagent.PipelineResult{
			RequestID: "test-req-4",
			Draft: &deepagent.WriterDraft{
				Sections: []deepagent.DraftSection{
					{SectionIndex: 0, Heading: "Summary", Text: "A comprehensive analysis of the topic."},
					{SectionIndex: 1, Heading: "Details", Text: "Further details here."},
				},
				Citations: []deepagent.DraftCitation{
					{Marker: 1, DocID: "doc-1", URL: "http://example.com/1", Title: "Source 1"},
				},
			},
		},
	}
	notif := &mockNotifier{}
	audit := &mockAuditWriter{}

	handler := DeepResearchHandler(capCheck, pipeline.run, notif.notify, audit.write)

	_, output, err := handler(context.Background(), nil, DeepResearchInput{Query: "test output"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.Summary == "" {
		t.Error("expected non-empty summary in output")
	}
	if output.Stats.LatencyMs < 0 {
		t.Error("expected latency_ms >= 0")
	}

	// Verify report text is constructed from draft sections.
	if output.Report == "" {
		t.Error("expected non-empty report in output")
	}

	// Verify citations are mapped from draft citations.
	if len(output.Citations) == 0 {
		t.Error("expected at least one citation in output")
	}
	if output.Citations[0].DocID != "doc-1" {
		t.Errorf("citation doc_id: got %q, want %q", output.Citations[0].DocID, "doc-1")
	}
}

// TestDeepResearchCappedAuditLine verifies that a cap-exceeded call still
// produces an audit line.
func TestDeepResearchCappedAuditLine(t *testing.T) {
	capCheck := &mockCapChecker{
		result: costguard.CapResult{
			Allowed:  false,
			Exceeded: costguard.DimensionUSD,
		},
	}
	pipeline := &mockPipelineRunner{}
	notif := &mockNotifier{}
	audit := &mockAuditWriter{}

	handler := DeepResearchHandler(capCheck, pipeline.run, notif.notify, audit.write)

	_, _, _ = handler(context.Background(), nil, DeepResearchInput{Query: "capped"})

	lines := audit.getLines()
	if len(lines) == 0 {
		t.Fatal("expected audit line even when capped")
	}

	var entry map[string]any
	_ = json.Unmarshal([]byte(lines[0]), &entry)
	if outcome, _ := entry["outcome"].(string); outcome != "capped" {
		t.Errorf("audit outcome: got %q, want %q", outcome, "capped")
	}
}

// Ensure imports are used (compile-time check).
var _ = time.Now
var _ = strings.Contains
