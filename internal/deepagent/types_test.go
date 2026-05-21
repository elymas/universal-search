package deepagent

import (
	"testing"
)

// T-M1-003 [RED]: Agent enum + types + prompts smoke tests
// NFR-DEEP2-002: Agent enum is bounded set {researcher, reviewer, writer, verifier}.
// Compile-time enforced via type constraints.

func TestAgentTypeIsStringEnumBoundedTo4Values(t *testing.T) {
	// Verify all four agents exist and have correct string values.
	agents := map[Agent]string{
		AgentResearcher: "researcher",
		AgentReviewer:   "reviewer",
		AgentWriter:     "writer",
		AgentVerifier:   "verifier",
	}
	for agent, expected := range agents {
		if string(agent) != expected {
			t.Errorf("Agent %q string value = %q, want %q", agent, string(agent), expected)
		}
	}

	// Verify AllAgents contains exactly 4 entries.
	if len(AllAgents) != 4 {
		t.Errorf("len(AllAgents) = %d, want 4", len(AllAgents))
	}
}

func TestAgentValuesAreBounded(t *testing.T) {
	// Verify AllAgents contains the exact expected set.
	expected := map[Agent]bool{
		AgentResearcher: true,
		AgentReviewer:   true,
		AgentWriter:     true,
		AgentVerifier:   true,
	}
	for _, a := range AllAgents {
		if !expected[a] {
			t.Errorf("unexpected agent in AllAgents: %q", a)
		}
	}
}

func TestPipelineRequestFields(t *testing.T) {
	req := PipelineRequest{
		RequestID: "test-123",
		Query:     "test query",
		Lang:      "ko",
	}
	if req.RequestID != "test-123" {
		t.Errorf("RequestID = %q, want %q", req.RequestID, "test-123")
	}
	if req.Query != "test query" {
		t.Errorf("Query = %q, want %q", req.Query, "test query")
	}
	if req.Lang != "ko" {
		t.Errorf("Lang = %q, want %q", req.Lang, "ko")
	}
}

func TestAgentOutcomeFields(t *testing.T) {
	outcome := AgentOutcome{
		Agent:   AgentResearcher,
		Model:   "test-model",
		CostUSD: 0.005,
	}
	if outcome.Agent != AgentResearcher {
		t.Errorf("Agent = %q, want %q", outcome.Agent, AgentResearcher)
	}
	if outcome.Model != "test-model" {
		t.Errorf("Model = %q, want %q", outcome.Model, "test-model")
	}
	if outcome.CostUSD != 0.005 {
		t.Errorf("CostUSD = %f, want 0.005", outcome.CostUSD)
	}
}

func TestResearcherOutputIsEmpty(t *testing.T) {
	out := ResearcherOutput{IsEmpty: true}
	if !out.IsEmpty {
		t.Error("IsEmpty should be true")
	}
}

func TestReviewerCritiqueFields(t *testing.T) {
	critique := ReviewerCritique{
		Notes: []CritiqueNote{
			{ClaimID: "c1", ConcernType: "unsupported", Severity: "high"},
		},
	}
	if len(critique.Notes) != 1 {
		t.Fatalf("len(Notes) = %d, want 1", len(critique.Notes))
	}
	if critique.Notes[0].ClaimID != "c1" {
		t.Errorf("ClaimID = %q, want %q", critique.Notes[0].ClaimID, "c1")
	}
}

func TestWriterDraftFields(t *testing.T) {
	draft := WriterDraft{
		Sections:   []DraftSection{{Heading: "Intro", Text: "body"}},
		Citations:  []DraftCitation{{Marker: 1, DocID: "d1"}},
		CostUSD:    0.01,
		Model:      "sonnet",
		Provider:   "openai",
	}
	if len(draft.Sections) != 1 {
		t.Errorf("len(Sections) = %d, want 1", len(draft.Sections))
	}
	if draft.Sections[0].Heading != "Intro" {
		t.Errorf("Heading = %q, want %q", draft.Sections[0].Heading, "Intro")
	}
	if len(draft.Citations) != 1 {
		t.Errorf("len(Citations) = %d, want 1", len(draft.Citations))
	}
	if draft.Citations[0].Marker != 1 {
		t.Errorf("Marker = %d, want 1", draft.Citations[0].Marker)
	}
}

func TestVerifierResultFields(t *testing.T) {
	vr := VerifierResult{
		Pass: true,
	}
	if !vr.Pass {
		t.Error("Pass should be true")
	}

	vrFail := VerifierResult{
		Pass: false,
		Feedback: &VerifierFeedback{
			UncitedCount:     3,
			UncitedSentences: []string{"s1", "s2", "s3"},
		},
	}
	if vrFail.Pass {
		t.Error("Pass should be false")
	}
	if vrFail.Feedback.UncitedCount != 3 {
		t.Errorf("UncitedCount = %d, want 3", vrFail.Feedback.UncitedCount)
	}
}
