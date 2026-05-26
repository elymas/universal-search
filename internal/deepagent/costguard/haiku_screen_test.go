package costguard

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"
)

// mockLLMCaller is a test double for LLMCaller.
type mockLLMCaller struct {
	response string
	err      error
	calls    atomic.Int32
	model    string
}

func (m *mockLLMCaller) Complete(_ context.Context, model, _, _ string) (string, error) {
	m.model = model
	m.calls.Add(1)
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

// failingLLMCaller always returns an error.
type failingLLMCaller struct {
	calls atomic.Int32
}

func (f *failingLLMCaller) Complete(_ context.Context, _, _, _ string) (string, error) {
	f.calls.Add(1)
	return "", fmt.Errorf("LLM service unavailable (5xx)")
}

// ---------------------------------------------------------------------------
// Phase C: Haiku Pre-Screen Client + Circuit Breaker
// ---------------------------------------------------------------------------

// TestHaikuScreenCallsLLMClientWithHaikuModel verifies that the screen
// calls the LLM client with the configured Haiku model.
// REQ-DEEP4-003.
func TestHaikuScreenCallsLLMClientWithHaikuModel(t *testing.T) {
	t.Parallel()

	mock := &mockLLMCaller{
		response: `{"score": 7, "rationale": "complex research", "suggested_mode": "deep"}`,
	}
	cfg := DefaultHaikuScreenConfig()
	cfg.Model = "claude-haiku-4-5"

	screen := NewHaikuScreen(mock, cfg)
	_, err := screen.Screen(context.Background(), "quantum computing advances")
	if err != nil {
		t.Fatalf("Screen: %v", err)
	}

	if mock.model != "claude-haiku-4-5" {
		t.Errorf("model: got %q, want %q", mock.model, "claude-haiku-4-5")
	}
	if mock.calls.Load() != 1 {
		t.Errorf("expected 1 LLM call, got %d", mock.calls.Load())
	}
}

// TestHaikuScreenParsesScoreFromJSON verifies that the screen correctly
// parses score, rationale, and suggested_mode from the JSON response.
// REQ-DEEP4-003.
func TestHaikuScreenParsesScoreFromJSON(t *testing.T) {
	t.Parallel()

	mock := &mockLLMCaller{
		response: `{"score": 8, "rationale": "requires multi-source analysis", "suggested_mode": "deep"}`,
	}
	screen := NewHaikuScreen(mock, DefaultHaikuScreenConfig())

	resp, err := screen.Screen(context.Background(), "analyze global economic trends")
	if err != nil {
		t.Fatalf("Screen: %v", err)
	}

	if resp.Score != 8 {
		t.Errorf("score: got %d, want 8", resp.Score)
	}
	if resp.Rationale != "requires multi-source analysis" {
		t.Errorf("rationale: got %q", resp.Rationale)
	}
	if resp.SuggestedMode != "deep" {
		t.Errorf("suggested_mode: got %q, want %q", resp.SuggestedMode, "deep")
	}
}

// TestHaikuScreenParseFailureIncrementsCounterAndFailsOpen verifies that
// invalid JSON from Haiku causes fail-open behavior.
// REQ-DEEP4-003: parse failure -> fail open.
func TestHaikuScreenParseFailureIncrementsCounterAndFailsOpen(t *testing.T) {
	t.Parallel()

	mock := &mockLLMCaller{
		response: `this is not valid json at all`,
	}
	screen := NewHaikuScreen(mock, DefaultHaikuScreenConfig())

	resp, err := screen.Screen(context.Background(), "test query")
	if err != nil {
		t.Fatalf("expected no error on parse failure (fail-open), got: %v", err)
	}

	if resp.Outcome != ScreenFailOpen {
		t.Errorf("outcome: got %d, want ScreenFailOpen (%d)", resp.Outcome, ScreenFailOpen)
	}
}

// TestScreenScoreThresholds tests the 3-way threshold branching.
// REQ-DEEP4-004: >=6 proceed / 4-5 suggest basic / <4 reject.
func TestScreenScoreThresholds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		score       int
		wantOutcome ScreenOutcome
		wantMode    string
	}{
		{
			name:        "score_7_proceeds",
			score:       7,
			wantOutcome: ScreenProceed,
			wantMode:    "deep",
		},
		{
			name:        "score_5_suggests_basic",
			score:       5,
			wantOutcome: ScreenSuggestBasic,
			wantMode:    "basic",
		},
		{
			name:        "score_3_rejects",
			score:       3,
			wantOutcome: ScreenReject,
			wantMode:    "reject",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			resp := ScreenResult{
				Score:         tc.score,
				Rationale:     "test rationale",
				SuggestedMode: tc.wantMode,
			}
			raw, _ := json.Marshal(resp)

			mock := &mockLLMCaller{response: string(raw)}
			screen := NewHaikuScreen(mock, DefaultHaikuScreenConfig())

			got, err := screen.Screen(context.Background(), "test query")
			if err != nil {
				t.Fatalf("Screen: %v", err)
			}
			if got.Outcome != tc.wantOutcome {
				t.Errorf("outcome: got %d, want %d", got.Outcome, tc.wantOutcome)
			}
			if got.SuggestedMode != tc.wantMode {
				t.Errorf("suggested_mode: got %q, want %q", got.SuggestedMode, tc.wantMode)
			}
		})
	}
}

// TestHaikuScreenBreakerOpensAfter5ConsecutiveFailures verifies that
// after 5 consecutive failures the circuit breaker opens and subsequent
// calls are short-circuited for 30 seconds.
// REQ-DEEP4-005.
func TestHaikuScreenBreakerOpensAfter5ConsecutiveFailures(t *testing.T) {
	t.Parallel()

	mock := &failingLLMCaller{}
	cfg := DefaultHaikuScreenConfig()
	cfg.MaxConsecutiveFails = 5
	cfg.OpenDurationMs = 30000 // 30 seconds

	screen := NewHaikuScreen(mock, cfg)

	// First 5 calls should hit the LLM and fail.
	for i := 0; i < 5; i++ {
		resp, err := screen.Screen(context.Background(), "test query")
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i+1, err)
		}
		if resp.Outcome != ScreenFailOpen {
			t.Errorf("call %d: expected ScreenFailOpen, got %d", i+1, resp.Outcome)
		}
	}

	// After 5 consecutive failures, breaker should be open.
	if screen.State() != CircuitOpen {
		t.Error("expected circuit breaker to be Open after 5 consecutive failures")
	}

	// The 6th call should NOT reach the LLM (breaker open).
	callsBefore := mock.calls.Load()
	resp, err := screen.Screen(context.Background(), "test query")
	if err != nil {
		t.Fatalf("call 6: unexpected error: %v", err)
	}
	if resp.Outcome != ScreenFailOpen {
		t.Errorf("call 6: expected ScreenFailOpen (breaker open), got %d", resp.Outcome)
	}
	callsAfter := mock.calls.Load()
	if callsAfter != callsBefore {
		t.Errorf("call 6: expected no new LLM call, got %d additional calls", callsAfter-callsBefore)
	}
}
