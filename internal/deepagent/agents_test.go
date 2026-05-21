package deepagent

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"time"

	"github.com/elymas/universal-search/internal/deepreport"
	"github.com/elymas/universal-search/internal/fanout"
	"github.com/elymas/universal-search/internal/llm"
	"github.com/elymas/universal-search/pkg/types"
)

// T-M2-001 [RED]: Researcher fanout + immutability tests
// REQ-DEEP2-005: Researcher calls fanout.Dispatch() EXACTLY once.

// mockFanoutFn counts calls and returns canned docs.
func mockFanoutFn(docs []types.NormalizedDoc) func(ctx context.Context, query string) (*fanout.Result, error) {
	return func(ctx context.Context, query string) (*fanout.Result, error) {
		return &fanout.Result{Docs: docs}, nil
	}
}

func makeTestDocs(n int) []types.NormalizedDoc {
	docs := make([]types.NormalizedDoc, n)
	for i := range docs {
		docs[i] = types.NormalizedDoc{
			ID:          string(rune('a' + i)),
			SourceID:    "test-source",
			URL:         "https://example.com/" + string(rune('a'+i)),
			Title:       "Test Doc " + string(rune('A'+i)),
			Body:        "Body of test document " + string(rune('A'+i)),
			RetrievedAt: time.Now(),
		}
	}
	return docs
}

func TestResearcherCallsFanoutDispatchExactlyOnce(t *testing.T) {
	var callCount int32
	docs := makeTestDocs(3)
	fanoutFn := func(ctx context.Context, query string) (*fanout.Result, error) {
		atomic.AddInt32(&callCount, 1)
		return &fanout.Result{Docs: docs}, nil
	}

	cfg := DefaultConfig()
	req := PipelineRequest{RequestID: "test-1", Query: "test query", Lang: "en"}
	llmClient := &mockLLMClient{}

	_, err := Researcher(context.Background(), cfg, llmClient, req, fanoutFn)
	if err != nil {
		t.Fatalf("Researcher() error: %v", err)
	}

	if count := atomic.LoadInt32(&callCount); count != 1 {
		t.Errorf("fanoutFn called %d times, want exactly 1", count)
	}
}

func TestResearcherDocsAreImmutableInDownstream(t *testing.T) {
	docs := makeTestDocs(2)
	fanoutFn := mockFanoutFn(docs)

	cfg := DefaultConfig()
	req := PipelineRequest{RequestID: "test-2", Query: "test query", Lang: "en"}
	llmClient := &mockLLMClient{response: `{"claims": [{"id": "c1", "text": "test claim", "sources": ["a"]}]}`}

	out, err := Researcher(context.Background(), cfg, llmClient, req, fanoutFn)
	if err != nil {
		t.Fatalf("Researcher() error: %v", err)
	}

	// Mutate the returned slice; original docs should be unaffected.
	if len(out.Evidence) > 0 {
		out.Evidence[0].Title = "MUTATED"
		if docs[0].Title == "MUTATED" {
			t.Error("mutating ResearcherOutput.Evidence affected original docs — slices share backing array")
		}
	}
}

// mockLLMClient is a test double for llm.Client.
type mockLLMClient struct {
	response string
	err      error
	calls    int32
}

func (m *mockLLMClient) Complete(ctx context.Context, req llm.Request) (llm.Response, error) {
	atomic.AddInt32(&m.calls, 1)
	if m.err != nil {
		return llm.Response{}, m.err
	}
	return llm.Response{
		Text:             m.response,
		Model:            "test-model",
		Provider:         "test-provider",
		PromptTokens:     100,
		CompletionTokens: 50,
		CostUSD:          0.001,
	}, nil
}

func (m *mockLLMClient) Stream(ctx context.Context, req llm.Request) (<-chan llm.Delta, error) {
	ch := make(chan llm.Delta, 1)
	ch <- llm.Delta{Content: m.response, FinishReason: "stop"}
	close(ch)
	return ch, nil
}

func (m *mockLLMClient) Embed(ctx context.Context, req llm.EmbedRequest) (llm.EmbedResponse, error) {
	return llm.EmbedResponse{}, nil
}

func (m *mockLLMClient) Close() error { return nil }

// T-M2-003 [RED]: Reviewer no-fanout + critique-only tests
// REQ-DEEP2-002: Reviewer does NOT call fanout. Critique-only.

func TestReviewerDoesNotCallFanout(t *testing.T) {
	cfg := DefaultConfig()
	llmClient := &mockLLMClient{
		response: `{"notes": [{"claim_id": "c1", "concern_type": "unsupported", "severity": "high"}]}`,
	}

	research := ResearcherOutput{
		Claims: []Claim{
			{ID: "c1", Text: "test claim", Sources: []string{"doc1"}},
		},
		Evidence: []deepreport.NormalizedDocPayload{
			{ID: "doc1", Title: "Doc 1", URL: "https://example.com/1"},
		},
		IsEmpty: false,
	}

	// Reviewer signature does NOT include a fanoutFn parameter.
	// This is the compile-time enforcement that Reviewer cannot call fanout.
	critique, err := Reviewer(context.Background(), cfg, llmClient, research)
	if err != nil {
		t.Fatalf("Reviewer() error: %v", err)
	}

	if len(critique.Notes) == 0 {
		t.Error("Reviewer returned no critique notes")
	}

	if critique.Notes[0].ClaimID != "c1" {
		t.Errorf("Note ClaimID = %q, want %q", critique.Notes[0].ClaimID, "c1")
	}
	if critique.Notes[0].ConcernType != "unsupported" {
		t.Errorf("Note ConcernType = %q, want %q", critique.Notes[0].ConcernType, "unsupported")
	}
	if critique.Notes[0].Severity != "high" {
		t.Errorf("Note Severity = %q, want %q", critique.Notes[0].Severity, "high")
	}
}

// T-M2-005 [RED]: Empty fanout short-circuit tests
// REQ-DEEP2-012: Empty fanout → Researcher skips LLM, returns IsEmpty:true.

func TestEmptyFanoutShortCircuitsPipeline(t *testing.T) {
	// Fanout returns empty docs.
	fanoutFn := func(ctx context.Context, query string) (*fanout.Result, error) {
		return &fanout.Result{Docs: []types.NormalizedDoc{}}, nil
	}

	cfg := DefaultConfig()
	req := PipelineRequest{RequestID: "test-empty", Query: "no results query", Lang: "en"}
	llmClient := &mockLLMClient{} // Should NOT be called.

	out, err := Researcher(context.Background(), cfg, llmClient, req, fanoutFn)
	if err != nil {
		t.Fatalf("Researcher() error: %v", err)
	}

	if !out.IsEmpty {
		t.Error("ResearcherOutput.IsEmpty should be true for empty fanout")
	}
}

func TestEmptyFanoutResearcherSkipsLLMInvocation(t *testing.T) {
	// Fanout returns empty docs.
	fanoutFn := func(ctx context.Context, query string) (*fanout.Result, error) {
		return &fanout.Result{Docs: []types.NormalizedDoc{}}, nil
	}

	cfg := DefaultConfig()
	req := PipelineRequest{RequestID: "test-empty-llm", Query: "no results query", Lang: "en"}
	llmClient := &mockLLMClient{} // calls starts at 0.

	_, err := Researcher(context.Background(), cfg, llmClient, req, fanoutFn)
	if err != nil {
		t.Fatalf("Researcher() error: %v", err)
	}

	// Verify LLM was NOT called.
	if count := atomic.LoadInt32(&llmClient.calls); count != 0 {
		t.Errorf("LLM was called %d times on empty fanout, want 0", count)
	}
}

// T-M3-001 [RED]: Writer no-fanout + retry hint + draft well-formedness tests
// REQ-DEEP2-002: Writer does NOT call fanout.

func TestWriterDoesNotCallFanout(t *testing.T) {
	cfg := DefaultConfig()
	llmClient := &mockLLMClient{
		response: `{"sections": [{"section_index": 0, "heading": "Test", "text": "Body [1]", "citation_markers": [1]}], "citations": [{"marker": 1, "doc_id": "d1", "url": "https://example.com", "title": "Doc 1"}]}`,
	}

	research := ResearcherOutput{
		Claims: []Claim{{ID: "c1", Text: "claim", Sources: []string{"d1"}}},
		Evidence: []deepreport.NormalizedDocPayload{
			{ID: "d1", Title: "Doc 1", URL: "https://example.com"},
		},
	}
	critique := ReviewerCritique{Notes: []CritiqueNote{}}

	// Writer signature does NOT include FanoutFn — compile-time enforcement.
	draft, err := Writer(context.Background(), cfg, llmClient, research, critique, nil)
	if err != nil {
		t.Fatalf("Writer() error: %v", err)
	}

	if len(draft.Sections) != 1 {
		t.Errorf("len(draft.Sections) = %d, want 1", len(draft.Sections))
	}
	if len(draft.Citations) != 1 {
		t.Errorf("len(draft.Citations) = %d, want 1", len(draft.Citations))
	}
	if draft.Citations[0].Marker != 1 {
		t.Errorf("Citation Marker = %d, want 1", draft.Citations[0].Marker)
	}
}

func TestWriterAcceptsRetryHintAndPrependsToContext(t *testing.T) {
	cfg := DefaultConfig()

	var capturedPrompt string
	llmClient := &spyLLMClient{onComplete: func(req llm.Request) {
		for _, msg := range req.Messages {
			if msg.Role == "user" {
				capturedPrompt = msg.Content
			}
		}
	}}

	research := ResearcherOutput{
		Claims:   []Claim{{ID: "c1", Text: "claim", Sources: []string{"d1"}}},
		Evidence: []deepreport.NormalizedDocPayload{{ID: "d1", Title: "Doc 1"}},
	}
	critique := ReviewerCritique{}
	retryHint := &VerifierFeedback{
		UncitedCount:     2,
		UncitedSentences: []string{"uncited sentence 1", "uncited sentence 2"},
	}

	_, err := Writer(context.Background(), cfg, llmClient, research, critique, retryHint)
	if err != nil {
		t.Fatalf("Writer() error: %v", err)
	}

	// Verify retry hint was included in the LLM prompt.
	if capturedPrompt == "" {
		t.Fatal("no user message captured from LLM call")
	}
	if !contains(capturedPrompt, "uncited sentence 1") {
		t.Error("retry hint sentences not found in LLM prompt context")
	}
}

func TestWriterDraftSectionsHaveSentenceLevelCitations(t *testing.T) {
	cfg := DefaultConfig()
	llmClient := &mockLLMClient{
		response: `{"sections": [{"section_index": 0, "heading": "Section 1", "text": "First claim [1]. Second claim [2].", "citation_markers": [1, 2]}], "citations": [{"marker": 1, "doc_id": "d1", "url": "https://a.com", "title": "A"}, {"marker": 2, "doc_id": "d2", "url": "https://b.com", "title": "B"}]}`,
	}

	research := ResearcherOutput{
		Claims:   []Claim{{ID: "c1", Text: "claim", Sources: []string{"d1", "d2"}}},
		Evidence: []deepreport.NormalizedDocPayload{{ID: "d1"}, {ID: "d2"}},
	}
	critique := ReviewerCritique{}

	draft, err := Writer(context.Background(), cfg, llmClient, research, critique, nil)
	if err != nil {
		t.Fatalf("Writer() error: %v", err)
	}

	// Verify citation markers are 1-indexed.
	for _, sec := range draft.Sections {
		for _, m := range sec.CitationMarkers {
			if m < 1 {
				t.Errorf("citation marker %d is not 1-indexed", m)
			}
		}
	}

	// Verify each citation has a valid marker >= 1.
	for _, cit := range draft.Citations {
		if cit.Marker < 1 {
			t.Errorf("citation marker %d should be >= 1", cit.Marker)
		}
	}
}

// spyLLMClient captures LLM request details for assertions.
type spyLLMClient struct {
	onComplete func(req llm.Request)
}

func (s *spyLLMClient) Complete(ctx context.Context, req llm.Request) (llm.Response, error) {
	if s.onComplete != nil {
		s.onComplete(req)
	}
	return llm.Response{
		Text:     `{"sections": [], "citations": []}`,
		Model:    "test",
		Provider: "test",
		CostUSD:  0.001,
	}, nil
}

func (s *spyLLMClient) Stream(ctx context.Context, req llm.Request) (<-chan llm.Delta, error) {
	ch := make(chan llm.Delta, 1)
	close(ch)
	return ch, nil
}

func (s *spyLLMClient) Embed(ctx context.Context, req llm.EmbedRequest) (llm.EmbedResponse, error) {
	return llm.EmbedResponse{}, nil
}

func (s *spyLLMClient) Close() error { return nil }

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// T-M4-005 [RED]: Verifier PASS/FAIL gate + counter tests
// REQ-DEEP2-006: Verifier calls CheckFaithfulness; PASS iff uncited_count == 0.

func TestVerifierPassWhenUncitedCountZero(t *testing.T) {
	cfg := DefaultConfig()
	draft := WriterDraft{
		Sections: []DraftSection{
			{SectionIndex: 0, Heading: "Test", Text: "Cited text [1].", CitationMarkers: []int{1}},
		},
		Citations: []DraftCitation{
			{Marker: 1, DocID: "d1", URL: "https://example.com", Title: "Doc 1"},
		},
	}
	docs := []deepreport.NormalizedDocPayload{
		{ID: "d1", Title: "Doc 1", Body: "Document body text"},
	}

	// Use a mock faithfulness check that always returns PASS.
	mockFn := func(ctx context.Context, text string, citations []string, docs []string) (VerifierResult, error) {
		return VerifierResult{Pass: true}, nil
	}

	result, err := VerifierWithChecker(context.Background(), cfg, draft, docs, mockFn)
	if err != nil {
		t.Fatalf("Verifier() error: %v", err)
	}
	if !result.Pass {
		t.Error("expected Pass=true when uncited_count == 0")
	}
	if result.Feedback != nil {
		t.Error("expected nil Feedback on PASS")
	}
}

func TestVerifierFailWhenUncitedCountPositive(t *testing.T) {
	cfg := DefaultConfig()
	draft := WriterDraft{
		Sections: []DraftSection{
			{SectionIndex: 0, Heading: "Test", Text: "Uncited sentence. Cited [1].", CitationMarkers: []int{1}},
		},
		Citations: []DraftCitation{
			{Marker: 1, DocID: "d1", URL: "https://example.com", Title: "Doc 1"},
		},
	}
	docs := []deepreport.NormalizedDocPayload{
		{ID: "d1", Title: "Doc 1", Body: "Document body text"},
	}

	mockFn := func(ctx context.Context, text string, citations []string, docs []string) (VerifierResult, error) {
		return VerifierResult{
			Pass: false,
			Feedback: &VerifierFeedback{
				UncitedCount:     1,
				UncitedSentences: []string{"Uncited sentence."},
			},
		}, nil
	}

	result, err := VerifierWithChecker(context.Background(), cfg, draft, docs, mockFn)
	if err != nil {
		t.Fatalf("Verifier() error: %v", err)
	}
	if result.Pass {
		t.Error("expected Pass=false when uncited_count > 0")
	}
	if result.Feedback == nil {
		t.Fatal("expected non-nil Feedback on FAIL")
	}
	if result.Feedback.UncitedCount != 1 {
		t.Errorf("Feedback.UncitedCount = %d, want 1", result.Feedback.UncitedCount)
	}
}

func TestVerifierCallsCheckFaithfulnessExactlyOnce(t *testing.T) {
	cfg := DefaultConfig()
	draft := WriterDraft{
		Sections:  []DraftSection{{SectionIndex: 0, Heading: "Test", Text: "Text [1].", CitationMarkers: []int{1}}},
		Citations: []DraftCitation{{Marker: 1, DocID: "d1", URL: "https://example.com", Title: "D1"}},
	}
	docs := []deepreport.NormalizedDocPayload{{ID: "d1", Title: "D1", Body: "body"}}

	var callCount int32
	mockFn := func(ctx context.Context, text string, citations []string, docs []string) (VerifierResult, error) {
		atomic.AddInt32(&callCount, 1)
		return VerifierResult{Pass: true}, nil
	}

	_, err := VerifierWithChecker(context.Background(), cfg, draft, docs, mockFn)
	if err != nil {
		t.Fatalf("Verifier() error: %v", err)
	}

	if count := atomic.LoadInt32(&callCount); count != 1 {
		t.Errorf("CheckFaithfulness called %d times, want exactly 1", count)
	}
}

func TestVerifierDoesNotPerformAdditionalScoring(t *testing.T) {
	// Verifier should not make any LLM calls — only faithfulness check.
	cfg := DefaultConfig()
	draft := WriterDraft{
		Sections:  []DraftSection{{SectionIndex: 0, Heading: "T", Text: "X [1].", CitationMarkers: []int{1}}},
		Citations: []DraftCitation{{Marker: 1, DocID: "d1", URL: "u", Title: "T"}},
	}
	docs := []deepreport.NormalizedDocPayload{{ID: "d1"}}

	mockFn := func(ctx context.Context, text string, citations []string, docs []string) (VerifierResult, error) {
		return VerifierResult{Pass: true}, nil
	}

	// Verify this does NOT need an LLM client at all.
	_, err := VerifierWithChecker(context.Background(), cfg, draft, docs, mockFn)
	if err != nil {
		t.Fatalf("Verifier() error: %v", err)
	}
}

func TestVerifierReturnsErrorOnCheckFaithfulnessFailure(t *testing.T) {
	cfg := DefaultConfig()
	draft := WriterDraft{
		Sections:  []DraftSection{{SectionIndex: 0, Heading: "T", Text: "X [1].", CitationMarkers: []int{1}}},
		Citations: []DraftCitation{{Marker: 1, DocID: "d1", URL: "u", Title: "T"}},
	}
	docs := []deepreport.NormalizedDocPayload{{ID: "d1"}}

	mockFn := func(ctx context.Context, text string, citations []string, docs []string) (VerifierResult, error) {
		return VerifierResult{}, fmt.Errorf("sidecar 5xx")
	}

	_, err := VerifierWithChecker(context.Background(), cfg, draft, docs, mockFn)
	if err == nil {
		t.Fatal("expected error when CheckFaithfulness fails")
	}
}
