// Package llm_test — provider registry tests.
// REQ-LLM-002
package llm_test

import (
	"testing"

	"github.com/elymas/universal-search/internal/llm"
)

// TestProviderRegistryHasAllModelClasses verifies all ModelClass values have a priority list.
// REQ-LLM-002
func TestProviderRegistryHasAllModelClasses(t *testing.T) {
	t.Parallel()

	classes := []llm.ModelClass{
		llm.DeepResearch,
		llm.Summary,
		llm.Classify,
		llm.Embed,
	}

	for _, class := range classes {
		refs := llm.ProvidersFor(class)
		if len(refs) == 0 {
			t.Errorf("no providers configured for ModelClass %q", class)
		}
	}
}

// TestDeepResearchPrimaryIsAnthropic verifies DeepResearch routes to Anthropic first.
// REQ-LLM-002
func TestDeepResearchPrimaryIsAnthropic(t *testing.T) {
	t.Parallel()

	refs := llm.ProvidersFor(llm.DeepResearch)
	if len(refs) == 0 {
		t.Fatal("no providers for DeepResearch")
	}
	if refs[0].Provider != "anthropic" {
		t.Errorf("DeepResearch primary: got %q, want %q", refs[0].Provider, "anthropic")
	}
	if refs[0].Model != "claude-opus-4-7" {
		t.Errorf("DeepResearch model: got %q, want %q", refs[0].Model, "claude-opus-4-7")
	}
}

// TestUnknownModelClassReturnsEmpty verifies graceful handling of unknown classes.
func TestUnknownModelClassReturnsEmpty(t *testing.T) {
	t.Parallel()

	refs := llm.ProvidersFor("nonexistent")
	if refs != nil {
		t.Errorf("expected nil for unknown class, got %v", refs)
	}
}
