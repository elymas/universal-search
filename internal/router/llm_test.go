// Package router_test validates the LLM-fallback adjudicator.
package router_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/llm"
	"github.com/elymas/universal-search/internal/router"
)

// fakeLLMClient is a minimal mock of llm.Client used by router tests.
type fakeLLMClient struct {
	completeFn func(ctx context.Context, req llm.Request) (llm.Response, error)
	calls      int
}

func (f *fakeLLMClient) Complete(ctx context.Context, req llm.Request) (llm.Response, error) {
	f.calls++
	if f.completeFn != nil {
		return f.completeFn(ctx, req)
	}
	return llm.Response{Text: `{"category":"web","confidence":0.7,"rationale":"default"}`}, nil
}

func (f *fakeLLMClient) Stream(_ context.Context, _ llm.Request) (<-chan llm.Delta, error) {
	return nil, errors.New("not used")
}

func (f *fakeLLMClient) Embed(_ context.Context, _ llm.EmbedRequest) (llm.EmbedResponse, error) {
	return llm.EmbedResponse{}, errors.New("not used")
}
func (f *fakeLLMClient) Close() error { return nil }

// TestLLMFallbackParsesValidJSON asserts a clean JSON response decodes into
// a RoutingDecision-shaped result.
func TestLLMFallbackParsesValidJSON(t *testing.T) {
	t.Parallel()

	fake := &fakeLLMClient{
		completeFn: func(_ context.Context, _ llm.Request) (llm.Response, error) {
			return llm.Response{Text: `{"category":"academic","confidence":0.92,"rationale":"arXiv-style topic"}`}, nil
		},
	}
	cat, conf, rationale, err := router.LLMClassifyForTest(context.Background(), fake, "transformer paper", "", 2*time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if cat != router.CategoryAcademic {
		t.Errorf("category: got %q, want academic", cat)
	}
	if conf != 0.92 {
		t.Errorf("confidence: got %v, want 0.92", conf)
	}
	if rationale == "" {
		t.Error("rationale should be non-empty")
	}
}

// TestLLMFallbackStripsCodeFence asserts a ```json-wrapped response parses.
func TestLLMFallbackStripsCodeFence(t *testing.T) {
	t.Parallel()
	fake := &fakeLLMClient{
		completeFn: func(_ context.Context, _ llm.Request) (llm.Response, error) {
			return llm.Response{Text: "```json\n{\"category\":\"web\",\"confidence\":0.7}\n```"}, nil
		},
	}
	cat, conf, _, err := router.LLMClassifyForTest(context.Background(), fake, "test", "", 2*time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if cat != router.CategoryWeb || conf != 0.7 {
		t.Errorf("got (%q, %v), want (web, 0.7)", cat, conf)
	}
}

// TestLLMFallbackRejectsInvalidEnum asserts a category outside the 6-value
// enum returns ErrLLMParse.
func TestLLMFallbackRejectsInvalidEnum(t *testing.T) {
	t.Parallel()
	fake := &fakeLLMClient{
		completeFn: func(_ context.Context, _ llm.Request) (llm.Response, error) {
			return llm.Response{Text: `{"category":"vehicle","confidence":0.8}`}, nil
		},
	}
	_, _, _, err := router.LLMClassifyForTest(context.Background(), fake, "test", "", 2*time.Second)
	if !errors.Is(err, router.ErrLLMParse) {
		t.Errorf("got %v, want ErrLLMParse", err)
	}
}

// TestLLMFallbackHandlesNonJSON asserts non-JSON output returns ErrLLMParse.
func TestLLMFallbackHandlesNonJSON(t *testing.T) {
	t.Parallel()
	fake := &fakeLLMClient{
		completeFn: func(_ context.Context, _ llm.Request) (llm.Response, error) {
			return llm.Response{Text: `category: web`}, nil
		},
	}
	_, _, _, err := router.LLMClassifyForTest(context.Background(), fake, "test", "", 2*time.Second)
	if !errors.Is(err, router.ErrLLMParse) {
		t.Errorf("got %v, want ErrLLMParse", err)
	}
}

// TestLLMFallbackClampsConfidence asserts an out-of-range LLM confidence is
// clamped to [0,1].
func TestLLMFallbackClampsConfidence(t *testing.T) {
	t.Parallel()
	fake := &fakeLLMClient{
		completeFn: func(_ context.Context, _ llm.Request) (llm.Response, error) {
			return llm.Response{Text: `{"category":"web","confidence":1.7}`}, nil
		},
	}
	_, conf, _, err := router.LLMClassifyForTest(context.Background(), fake, "test", "", 2*time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if conf != 1.0 {
		t.Errorf("clamped confidence: got %v, want 1.0", conf)
	}
}

// TestLLMFallbackTruncatesRationale asserts a >200-char rationale is
// truncated.
func TestLLMFallbackTruncatesRationale(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("x", 500)
	fake := &fakeLLMClient{
		completeFn: func(_ context.Context, _ llm.Request) (llm.Response, error) {
			return llm.Response{Text: `{"category":"web","confidence":0.5,"rationale":"` + long + `"}`}, nil
		},
	}
	_, _, rat, err := router.LLMClassifyForTest(context.Background(), fake, "test", "", 2*time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(rat) > 200 {
		t.Errorf("rationale length: got %d, want ≤ 200", len(rat))
	}
}

// TestLLMFallbackHonorsTimeout asserts a slow LLM is cancelled at the
// internal deadline and returns ErrLLMTimeout.
func TestLLMFallbackHonorsTimeout(t *testing.T) {
	t.Parallel()
	fake := &fakeLLMClient{
		completeFn: func(ctx context.Context, _ llm.Request) (llm.Response, error) {
			select {
			case <-time.After(3 * time.Second):
				return llm.Response{Text: `{"category":"web","confidence":0.5}`}, nil
			case <-ctx.Done():
				return llm.Response{}, ctx.Err()
			}
		},
	}
	start := time.Now()
	_, _, _, err := router.LLMClassifyForTest(context.Background(), fake, "test", "", 200*time.Millisecond)
	elapsed := time.Since(start)
	if !errors.Is(err, router.ErrLLMTimeout) {
		t.Errorf("err: got %v, want ErrLLMTimeout", err)
	}
	if elapsed > 600*time.Millisecond {
		t.Errorf("elapsed: got %v, want ≤ 600ms", elapsed)
	}
}

// TestLLMFallbackHandlesCircuitBreakerOpen asserts that ErrAllProvidersFailed
// is wrapped as ErrLLMUnavailable.
func TestLLMFallbackHandlesCircuitBreakerOpen(t *testing.T) {
	t.Parallel()
	fake := &fakeLLMClient{
		completeFn: func(_ context.Context, _ llm.Request) (llm.Response, error) {
			return llm.Response{}, llm.ErrAllProvidersFailed
		},
	}
	_, _, _, err := router.LLMClassifyForTest(context.Background(), fake, "test", "", 2*time.Second)
	if !errors.Is(err, router.ErrLLMUnavailable) {
		t.Errorf("got %v, want ErrLLMUnavailable", err)
	}
}

// TestLLMFallbackUsesOverrideModel asserts the INTENT_ROUTER_LLM_MODEL value
// flows into Request.Override.
func TestLLMFallbackUsesOverrideModel(t *testing.T) {
	t.Parallel()
	var seenOverride string
	fake := &fakeLLMClient{
		completeFn: func(_ context.Context, req llm.Request) (llm.Response, error) {
			seenOverride = req.Override
			return llm.Response{Text: `{"category":"web","confidence":0.5}`}, nil
		},
	}
	_, _, _, err := router.LLMClassifyForTest(context.Background(), fake, "test", "claude-haiku-4-5", 2*time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if seenOverride != "claude-haiku-4-5" {
		t.Errorf("override: got %q, want %q", seenOverride, "claude-haiku-4-5")
	}
}

// TestLLMFallbackSystemPromptDeterministic asserts the system prompt is
// byte-for-byte identical across calls — required for prompt-cache hits at
// the LiteLLM/Anthropic boundary.
func TestLLMFallbackSystemPromptDeterministic(t *testing.T) {
	t.Parallel()
	a := router.ClassifySystemPrompt()
	b := router.ClassifySystemPrompt()
	if a != b {
		t.Error("system prompt is not deterministic")
	}
	// Sanity: the prompt should comfortably exceed the 1024-token cache
	// minimum. We approximate via byte length (4 chars/token rule of thumb).
	if got := len(a); got < 4096 {
		t.Errorf("system prompt length: got %d bytes, want ≥ 4096 (≈1024 tokens)", got)
	}
}
