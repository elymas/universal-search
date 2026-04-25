// Package llm_test — public API surface and contract tests.
// Verifies exported types, sentinel errors, and label safety.
package llm_test

import (
	"errors"
	"testing"

	"github.com/elymas/universal-search/internal/llm"
)

// TestPublicAPISurface verifies that the exported constants and errors are defined.
func TestPublicAPISurface(t *testing.T) {
	// ModelClass constants must be non-empty.
	classes := []llm.ModelClass{llm.DeepResearch, llm.Summary, llm.Classify, llm.Embed}
	for _, c := range classes {
		if string(c) == "" {
			t.Errorf("ModelClass constant is empty")
		}
	}

	// Sentinel errors must be distinct and non-nil.
	sentinels := []error{
		llm.ErrBudgetExceeded,
		llm.ErrStreamBackpressureTimeout,
		llm.ErrAllProvidersFailed,
		llm.ErrModelNotConfigured,
	}
	for i, e := range sentinels {
		if e == nil {
			t.Errorf("sentinel[%d] is nil", i)
		}
		for j, other := range sentinels {
			if i != j && errors.Is(e, other) {
				t.Errorf("sentinel[%d] matches sentinel[%d]", i, j)
			}
		}
	}
}

// TestResponseTypeFields verifies zero-value Response is valid (no panics, fields accessible).
func TestResponseTypeFields(t *testing.T) {
	var r llm.Response
	_ = r.Text
	_ = r.Provider
	_ = r.Model
	_ = r.FinishReason
	_ = r.PromptTokens
	_ = r.CompletionTokens
	_ = r.LatencyMs
	_ = r.CostUSD
}

// TestEmbedResponseTypeFields verifies zero-value EmbedResponse is valid.
func TestEmbedResponseTypeFields(t *testing.T) {
	var r llm.EmbedResponse
	_ = r.Vectors
	_ = r.Provider
	_ = r.Model
	_ = r.PromptTokens
	_ = r.LatencyMs
}

// TestDeltaTypeFields verifies zero-value Delta is valid.
func TestDeltaTypeFields(t *testing.T) {
	var d llm.Delta
	_ = d.Content
	_ = d.FinishReason
	_ = d.Err
}

// TestRequestTypeFields verifies zero-value Request is valid.
func TestRequestTypeFields(t *testing.T) {
	var r llm.Request
	_ = r.Class
	_ = r.System
	_ = r.Messages
	_ = r.MaxTokens
	_ = r.Temperature
	_ = r.Override
}

// TestMessageRoles verifies known role strings are non-empty.
func TestMessageRoles(t *testing.T) {
	roles := []string{"user", "assistant", "system"}
	for _, role := range roles {
		m := llm.Message{Role: role, Content: "test"}
		if m.Role != role {
			t.Errorf("Role: got %q, want %q", m.Role, role)
		}
	}
}

// TestModelClassDistinct verifies all ModelClass constants are distinct.
func TestModelClassDistinct(t *testing.T) {
	classes := map[llm.ModelClass]bool{}
	for _, c := range []llm.ModelClass{llm.DeepResearch, llm.Summary, llm.Classify, llm.Embed} {
		if classes[c] {
			t.Errorf("duplicate ModelClass: %q", c)
		}
		classes[c] = true
	}
}

// TestNoSensitiveDataInProviderLabels verifies provider names do not embed secrets.
// REQ-LLM-003 — bounded label cardinality, no PII in metric labels.
func TestNoSensitiveDataInProviderLabels(t *testing.T) {
	// ProviderRef provider names must be short plain strings.
	refs := llm.ProvidersFor(llm.Summary)
	for _, ref := range refs {
		if len(ref.Provider) > 50 {
			t.Errorf("provider name too long (possible secret): %q", ref.Provider)
		}
		if len(ref.Model) > 100 {
			t.Errorf("model name too long (possible secret): %q", ref.Model)
		}
	}
}

// TestErrBudgetExceededWrappable verifies errors.Is works through wrapping.
// NFR-LLM-003
func TestErrBudgetExceededWrappable(t *testing.T) {
	wrapped := errors.Join(llm.ErrBudgetExceeded, errors.New("extra"))
	if !errors.Is(wrapped, llm.ErrBudgetExceeded) {
		t.Error("ErrBudgetExceeded must be detectable through errors.Is after wrapping")
	}
}
