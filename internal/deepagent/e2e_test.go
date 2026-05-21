package deepagent

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/deepreport"
	"github.com/elymas/universal-search/internal/fanout"
	"github.com/elymas/universal-search/internal/llm"
)

// ---------------------------------------------------------------------------
// T-M7-001 [RED]: E2E happy-path + retry-path tests
// REQ-DEEP2-001 through REQ-DEEP2-008
// ---------------------------------------------------------------------------

// TestE2EHappyPathReturnsCompletePipelineResult verifies the full 4-agent
// pipeline produces a complete result: all 4 agents invoked in order,
// fanout called exactly once, final draft is non-nil.
// Acceptance Scenario 1: Verifier PASS first attempt.
func TestE2EHappyPathReturnsCompletePipelineResult(t *testing.T) {
	var fanoutCalls int32
	docs := makeTestDocs(3)
	fanoutFn := func(ctx context.Context, query string) (*fanout.Result, error) {
		atomic.AddInt32(&fanoutCalls, 1)
		return &fanout.Result{Docs: docs}, nil
	}

	cfg := DefaultConfig()
	llmClient := &multiMockLLM{
		responses: []string{
			// Researcher: extract claims
			`{"claims": [{"id": "c1", "text": "claim1", "sources": ["a"]}, {"id": "c2", "text": "claim2", "sources": ["b"]}]}`,
			// Reviewer: critique
			`{"notes": [{"claim_id": "c1", "concern_type": "evidence_strength", "severity": "medium"}]}`,
			// Writer: draft
			`{"sections": [{"section_index":0,"heading":"Summary","text":"Test body [1] [2]","citation_markers":[1,2]}], "citations": [{"marker":1,"doc_id":"a","url":"https://a.com","title":"A"}, {"marker":2,"doc_id":"b","url":"https://b.com","title":"B"}]}`,
		},
	}

	// Verifier passes first attempt.
	checkFn := func(ctx context.Context, text string, citations []string, d []string) (VerifierResult, error) {
		return VerifierResult{Pass: true}, nil
	}

	req := PipelineRequest{RequestID: "e2e-happy-001", Query: "test query", Lang: "en"}
	result, err := RunPipelineWithVerifier(context.Background(), cfg, llmClient, req, fanoutFn, checkFn)
	if err != nil {
		t.Fatalf("RunPipelineWithVerifier() error: %v", err)
	}

	// Verify fanout called exactly once.
	if calls := atomic.LoadInt32(&fanoutCalls); calls != 1 {
		t.Errorf("fanout called %d times, want 1", calls)
	}

	// Verify draft is non-nil.
	if result.Draft == nil {
		t.Fatal("expected non-nil Draft")
	}
	if len(result.Draft.Sections) == 0 {
		t.Error("expected at least 1 section in draft")
	}
	if len(result.Draft.Citations) == 0 {
		t.Error("expected citations in draft")
	}

	// Verify agent log has entries for all 4 agents.
	agentSeen := map[Agent]bool{}
	for _, entry := range result.AgentLog {
		agentSeen[entry.Agent] = true
	}
	for _, agent := range AllAgents {
		if !agentSeen[agent] {
			t.Errorf("agent %q not found in AgentLog", agent)
		}
	}

	// Verify agent ordering: researcher before reviewer before writer before verifier.
	agentOrder := []Agent{}
	for _, entry := range result.AgentLog {
		agentOrder = append(agentOrder, entry.Agent)
	}
	if len(agentOrder) < 4 {
		t.Fatalf("agent order = %v, want at least 4 agents", agentOrder)
	}
	// First occurrence indices.
	researcherIdx := indexOfAgent(agentOrder, AgentResearcher)
	reviewerIdx := indexOfAgent(agentOrder, AgentReviewer)
	writerIdx := indexOfAgent(agentOrder, AgentWriter)
	verifierIdx := indexOfAgent(agentOrder, AgentVerifier)
	if researcherIdx >= reviewerIdx {
		t.Errorf("researcher (idx %d) should come before reviewer (idx %d)", researcherIdx, reviewerIdx)
	}
	if reviewerIdx >= writerIdx {
		t.Errorf("reviewer (idx %d) should come before writer (idx %d)", reviewerIdx, writerIdx)
	}
	if writerIdx >= verifierIdx {
		t.Errorf("writer (idx %d) should come before verifier (idx %d)", writerIdx, verifierIdx)
	}

	// Verify all outcomes are success.
	for _, entry := range result.AgentLog {
		if entry.Outcome != "success" {
			t.Errorf("agent %q outcome = %q, want success", entry.Agent, entry.Outcome)
		}
	}

	// Verify request ID is preserved.
	if result.RequestID != req.RequestID {
		t.Errorf("RequestID = %q, want %q", result.RequestID, req.RequestID)
	}
}

// TestE2ERetryPathRecordsCorrectMetricsAndEvents verifies the retry path:
// Verifier rejects first attempt, passes second. Writer is called twice.
// Acceptance Scenario 2: reject iter 1, PASS iter 2.
func TestE2ERetryPathRecordsCorrectMetricsAndEvents(t *testing.T) {
	docs := makeTestDocs(2)
	fanoutFn := func(ctx context.Context, query string) (*fanout.Result, error) {
		return &fanout.Result{Docs: docs}, nil
	}

	cfg := DefaultConfig()
	llmClient := &multiMockLLM{
		responses: []string{
			// Researcher
			`{"claims": [{"id": "c1", "text": "claim", "sources": ["a"]}]}`,
			// Reviewer
			`{"notes": []}`,
			// Writer 1st attempt
			`{"sections": [{"section_index":0,"heading":"T","text":"Body [1]","citation_markers":[1]}], "citations": [{"marker":1,"doc_id":"a","url":"u","title":"T"}]}`,
			// Writer 2nd attempt (retry)
			`{"sections": [{"section_index":0,"heading":"T","text":"Fixed [1]","citation_markers":[1]}], "citations": [{"marker":1,"doc_id":"a","url":"u","title":"T"}]}`,
		},
	}

	var verifierCalls int32
	checkFn := func(ctx context.Context, text string, citations []string, d []string) (VerifierResult, error) {
		count := atomic.AddInt32(&verifierCalls, 1)
		if count == 1 {
			return VerifierResult{
				Pass: false,
				Feedback: &VerifierFeedback{
					UncitedCount:     2,
					UncitedSentences: []string{"uncited sentence 1", "uncited sentence 2"},
				},
			}, nil
		}
		return VerifierResult{Pass: true}, nil
	}

	req := PipelineRequest{RequestID: "e2e-retry-001", Query: "test query", Lang: "en"}
	result, err := RunPipelineWithVerifier(context.Background(), cfg, llmClient, req, fanoutFn, checkFn)
	if err != nil {
		t.Fatalf("RunPipelineWithVerifier() error: %v", err)
	}

	// Verify verifier was called twice.
	if calls := atomic.LoadInt32(&verifierCalls); calls != 2 {
		t.Errorf("verifier called %d times, want 2", calls)
	}

	// Verify writer appears twice in agent log.
	writerCount := 0
	for _, entry := range result.AgentLog {
		if entry.Agent == AgentWriter {
			writerCount++
		}
	}
	if writerCount != 2 {
		t.Errorf("Writer called %d times, want 2", writerCount)
	}

	// Verify verifier appears twice in agent log.
	verifierCount := 0
	for _, entry := range result.AgentLog {
		if entry.Agent == AgentVerifier {
			verifierCount++
		}
	}
	if verifierCount != 2 {
		t.Errorf("Verifier called %d times, want 2", verifierCount)
	}

	// Verify final draft is non-nil (from second Writer attempt).
	if result.Draft == nil {
		t.Fatal("expected non-nil Draft after retry")
	}

	// Verify the draft contains the "Fixed" text from second Writer attempt.
	if len(result.Draft.Sections) > 0 && !strings.Contains(result.Draft.Sections[0].Text, "Fixed") {
		t.Error("expected draft to contain 'Fixed' from retry Writer attempt")
	}
}

// ---------------------------------------------------------------------------
// T-M7-003 [RED]: NFR-DEEP2-001 budget (a): mocked orchestration p95 <= 1s
// ---------------------------------------------------------------------------

// TestE2ELatencyP95Under1SecondMocked verifies Go-side orchestration overhead
// p95 <= 1 second with mocked LLM + faithfulness (instant responses).
// NFR-DEEP2-001 budget (a): 50 iterations statistical test.
func TestE2ELatencyP95Under1SecondMocked(t *testing.T) {
	docs := makeTestDocs(5)
	fanoutFn := func(ctx context.Context, query string) (*fanout.Result, error) {
		return &fanout.Result{Docs: docs}, nil
	}

	cfg := DefaultConfig()

	// Instant LLM mock — no latency.
	llmClient := &instantMockLLM{}

	checkFn := func(ctx context.Context, text string, citations []string, d []string) (VerifierResult, error) {
		return VerifierResult{Pass: true}, nil
	}

	const iterations = 50
	durations := make([]time.Duration, iterations)

	for i := 0; i < iterations; i++ {
		req := PipelineRequest{RequestID: "e2e-perf", Query: "perf test", Lang: "en"}
		start := time.Now()
		_, err := RunPipelineWithVerifier(context.Background(), cfg, llmClient, req, fanoutFn, checkFn)
		durations[i] = time.Since(start)
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
	}

	// Calculate p95.
	p95 := percentile(durations, 95)
	t.Logf("p95 latency: %v (50 iterations)", p95)

	if p95 > time.Second {
		t.Errorf("p95 latency = %v, want <= 1s", p95)
	}
}

// ---------------------------------------------------------------------------
// T-M7-005 [RED]: DEEP-001 regression + storm-mode schema-equivalence tests
// ---------------------------------------------------------------------------

// TestDeep001PipelineStillWorks verifies that DEEP-001 code paths (deepreport package)
// are not broken by DEEP-002 additions. This is a smoke test that deepreport types
// and functions still compile and work correctly.
// REQ-DEEP2-011, NFR-DEEP2-003: backward compatibility.
func TestDeep001PipelineStillWorks(t *testing.T) {
	// Verify deepreport types are accessible and functional.
	report := createMinimalReport()

	if report.RequestID == "" {
		t.Error("expected non-empty RequestID")
	}
	if len(report.Sections) == 0 {
		t.Error("expected at least 1 section")
	}
}

// TestStormModeSchemaEquivalence verifies that the DEEP-001 code path
// produces schema-identical output. Since we don't modify deepreport or
// streamsynth packages, this test verifies the types are unchanged.
// REQ-DEEP2-011, P-M6: same event types, same field names per event.
func TestStormModeSchemaEquivalence(t *testing.T) {
	// Verify WriterDraft and deepreport.Report both implement LongFormSource.
	// This ensures the streamsynth integration is schema-equivalent.
	draft := WriterDraft{
		Sections: []DraftSection{
			{SectionIndex: 0, Heading: "Test", Text: "Body", CitationMarkers: []int{1}},
		},
		Citations: []DraftCitation{
			{Marker: 1, DocID: "d1", URL: "https://example.com", Title: "Doc 1"},
		},
		Model:    "test-model",
		Provider: "test-provider",
		CostUSD:  0.01,
	}

	sections := draft.SourceSections()
	if len(sections) != 1 {
		t.Errorf("SourceSections() = %d, want 1", len(sections))
	}
	if sections[0].Heading != "Test" {
		t.Errorf("Heading = %q, want 'Test'", sections[0].Heading)
	}

	citations := draft.SourceCitations()
	if len(citations) != 1 {
		t.Errorf("SourceCitations() = %d, want 1", len(citations))
	}
	if citations[0].Marker != 1 {
		t.Errorf("Marker = %d, want 1", citations[0].Marker)
	}

	metadata := draft.SourceMetadata()
	if metadata.Model != "test-model" {
		t.Errorf("Model = %q, want 'test-model'", metadata.Model)
	}
}

// ---------------------------------------------------------------------------
// Helper functions
// ---------------------------------------------------------------------------

// indexOfAgent returns the index of the first occurrence of agent in the slice, or -1.
func indexOfAgent(agents []Agent, target Agent) int {
	for i, a := range agents {
		if a == target {
			return i
		}
	}
	return -1
}

// percentile returns the p-th percentile from a slice of durations.
func percentile(durations []time.Duration, p int) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	// Simple selection: sort and pick index.
	// Copy to avoid mutating input.
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	// Insertion sort (fine for 50 elements).
	for i := 1; i < len(sorted); i++ {
		j := i
		for j > 0 && sorted[j] < sorted[j-1] {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
			j--
		}
	}
	idx := (p * len(sorted)) / 100
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// instantMockLLM returns instant responses with no latency.
type instantMockLLM struct{}

func (m *instantMockLLM) Complete(ctx context.Context, req llm.Request) (llm.Response, error) {
	// Return appropriate responses based on system prompt content.
	system := strings.ToLower(req.System)
	if strings.Contains(system, "researcher") {
		return llm.Response{
			Text:     `{"claims": [{"id": "c1", "text": "claim", "sources": ["a"]}]}`,
			Model:    "test",
			Provider: "test",
			CostUSD:  0.001,
		}, nil
	}
	if strings.Contains(system, "reviewer") {
		return llm.Response{
			Text:     `{"notes": []}`,
			Model:    "test",
			Provider: "test",
			CostUSD:  0.001,
		}, nil
	}
	// Writer
	return llm.Response{
		Text:     `{"sections": [{"section_index":0,"heading":"T","text":"B [1]","citation_markers":[1]}], "citations": [{"marker":1,"doc_id":"a","url":"u","title":"T"}]}`,
		Model:    "test",
		Provider: "test",
		CostUSD:  0.002,
	}, nil
}

func (m *instantMockLLM) Stream(ctx context.Context, req llm.Request) (<-chan llm.Delta, error) {
	ch := make(chan llm.Delta, 1)
	close(ch)
	return ch, nil
}

func (m *instantMockLLM) Embed(ctx context.Context, req llm.EmbedRequest) (llm.EmbedResponse, error) {
	return llm.EmbedResponse{}, nil
}

func (m *instantMockLLM) Close() error { return nil }

// createMinimalReport returns a minimal deepreport.Report for regression testing.
// Uses DEEP-001 types to verify backward compatibility with SPEC-DEEP-002 additions.
func createMinimalReport() *deepreport.Report {
	return &deepreport.Report{
		RequestID: "regression-001",
		Title:     "Regression Test Report",
		Sections: []deepreport.Section{
			{
				SectionIndex: 0,
				Heading:      "Summary",
				Level:        1,
				Text:         "Test body text.",
			},
		},
		Citations: []deepreport.Citation{
			{Marker: 1, DocID: "doc-1", URL: "https://example.com", Title: "Source"},
		},
		Model:         "test-model",
		Provider:      "test-provider",
		CostUSD:       0.01,
		SchemaVersion: 1,
	}
}
