package llm

// Internal coverage for the pure helpers redactKey (secret redaction) and
// buildMessages (Request → OpenAI message param construction across all roles).
//
// @MX:SPEC: SPEC-REL-001 — G1 release coverage gate

import "testing"

func TestRedactKey(t *testing.T) {
	t.Run("empty key returns input unchanged", func(t *testing.T) {
		in := "no secret here"
		if got := redactKey(in, ""); got != in {
			t.Errorf("redactKey with empty key = %q, want unchanged", got)
		}
	})
	t.Run("redacts all occurrences", func(t *testing.T) {
		got := redactKey("token=sk-abc and again sk-abc", "sk-abc")
		if got != "token=[REDACTED] and again [REDACTED]" {
			t.Errorf("redactKey = %q", got)
		}
	})
	t.Run("no match leaves string intact", func(t *testing.T) {
		if got := redactKey("clean", "missing"); got != "clean" {
			t.Errorf("redactKey = %q, want clean", got)
		}
	})
}

func TestExtractHTTPStatus(t *testing.T) {
	if got := extractHTTPStatus(nil); got != 0 {
		t.Errorf("extractHTTPStatus(nil) = %d, want 0", got)
	}
	if got := extractHTTPStatus(errTest("rate limited: 429 too many")); got != 429 {
		t.Errorf("extractHTTPStatus(429) = %d, want 429", got)
	}
	if got := extractHTTPStatus(errTest("connection reset, no code")); got != 0 {
		t.Errorf("extractHTTPStatus(no code) = %d, want 0", got)
	}
}

type errTest string

func (e errTest) Error() string { return string(e) }

func TestBuildMessages(t *testing.T) {
	t.Run("system prompt plus all roles", func(t *testing.T) {
		req := Request{
			System: "you are helpful",
			Messages: []Message{
				{Role: "user", Content: "hi"},
				{Role: "assistant", Content: "hello"},
				{Role: "system", Content: "stay terse"},
				{Role: "unknown", Content: "ignored"}, // no matching case
			},
		}
		msgs := buildMessages(req)
		// system prompt + 3 recognised roles = 4 (the unknown role is skipped).
		if len(msgs) != 4 {
			t.Errorf("buildMessages produced %d messages, want 4", len(msgs))
		}
	})

	t.Run("no system prompt", func(t *testing.T) {
		req := Request{Messages: []Message{{Role: "user", Content: "hi"}}}
		if got := len(buildMessages(req)); got != 1 {
			t.Errorf("buildMessages produced %d messages, want 1", got)
		}
	})
}
