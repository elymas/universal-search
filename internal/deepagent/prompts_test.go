package deepagent

import (
	"strings"
	"testing"
)

// T-M1-003 [RED]: Prompts smoke tests
// REQ-DEEP2-004: Each role has stable prompt template names.

func TestPromptTemplatesExistForAllRoles(t *testing.T) {
	// Only LLM-backed agents need non-empty prompts.
	// Verifier uses SYN-002 binary gate directly, no LLM prompt needed.
	templates := map[Agent]func() string{
		AgentResearcher: ResearcherSystemPrompt,
		AgentReviewer:   ReviewerSystemPrompt,
		AgentWriter:     WriterSystemPrompt,
	}

	for agent, fn := range templates {
		prompt := fn()
		if prompt == "" {
			t.Errorf("prompt template for %q returned empty string", agent)
		}
	}
}

func TestVerifierSystemPromptIsEmpty(t *testing.T) {
	// Verifier does not use LLM — calls CheckFaithfulness directly.
	if VerifierSystemPrompt() != "" {
		t.Error("VerifierSystemPrompt should be empty (Verifier uses SYN-002 gate, not LLM)")
	}
}

func TestResearcherPromptMentionsEvidence(t *testing.T) {
	prompt := ResearcherSystemPrompt()
	if !strings.Contains(strings.ToLower(prompt), "evidence") {
		t.Error("Researcher system prompt should mention 'evidence'")
	}
}

func TestReviewerPromptMentionsCritique(t *testing.T) {
	prompt := ReviewerSystemPrompt()
	if !strings.Contains(strings.ToLower(prompt), "critique") {
		t.Error("Reviewer system prompt should mention 'critique'")
	}
}

func TestWriterPromptMentionsCitation(t *testing.T) {
	prompt := WriterSystemPrompt()
	if !strings.Contains(strings.ToLower(prompt), "citation") {
		t.Error("Writer system prompt should mention 'citation'")
	}
}
