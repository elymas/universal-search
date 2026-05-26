package costguard

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// LLMCaller abstracts the LLM completion API for Haiku pre-screen.
// This allows testing with mocks.
type LLMCaller interface {
	Complete(ctx context.Context, model string, systemPrompt string, userMessage string) (string, error)
}

// CircuitState represents the state of the circuit breaker.
type CircuitState int

const (
	CircuitClosed   CircuitState = iota // Normal operation
	CircuitOpen                         // Failing, reject calls
	CircuitHalfOpen                     // Testing recovery
)

// HaikuScreen uses a Haiku-tier model to pre-screen /deep requests.
// REQ-DEEP4-003: calls Haiku, parses score JSON, fails open on parse error.
// REQ-DEEP4-004: score >= 6 proceed / 4-5 suggest basic / < 4 reject.
// REQ-DEEP4-005: circuit breaker opens after 5 consecutive failures for 30s.
type HaikuScreen struct {
	llm               LLMCaller
	model             string
	thresholdProceed  int
	thresholdSuggest  int
	timeout           time.Duration
	failOpenOnTimeout bool

	// Circuit breaker state.
	mu               sync.Mutex
	state            CircuitState
	consecutiveFails int
	maxFails         int
	openUntil        time.Time
	openDuration     time.Duration
}

// HaikuScreenConfig holds configuration for the Haiku pre-screen.
type HaikuScreenConfig struct {
	Model               string
	ThresholdProceed    int
	ThresholdSuggest    int
	TimeoutMs           int
	FailOpenOnTimeout   bool
	MaxConsecutiveFails int
	OpenDurationMs      int
}

// DefaultHaikuScreenConfig returns defaults per SPEC-DEEP-004 §3.
func DefaultHaikuScreenConfig() HaikuScreenConfig {
	return HaikuScreenConfig{
		Model:               "claude-haiku-4-5",
		ThresholdProceed:    6,
		ThresholdSuggest:    4,
		TimeoutMs:           200,
		FailOpenOnTimeout:   true,
		MaxConsecutiveFails: 5,
		OpenDurationMs:      30000,
	}
}

// NewHaikuScreen creates a new HaikuScreen with the given LLM caller and config.
func NewHaikuScreen(llm LLMCaller, cfg HaikuScreenConfig) *HaikuScreen {
	timeout := time.Duration(cfg.TimeoutMs) * time.Millisecond
	if timeout == 0 {
		timeout = 200 * time.Millisecond
	}
	maxFails := cfg.MaxConsecutiveFails
	if maxFails == 0 {
		maxFails = 5
	}
	openDur := time.Duration(cfg.OpenDurationMs) * time.Millisecond
	if openDur == 0 {
		openDur = 30 * time.Second
	}
	return &HaikuScreen{
		llm:               llm,
		model:             cfg.Model,
		thresholdProceed:  cfg.ThresholdProceed,
		thresholdSuggest:  cfg.ThresholdSuggest,
		timeout:           timeout,
		failOpenOnTimeout: cfg.FailOpenOnTimeout,
		state:             CircuitClosed,
		maxFails:          maxFails,
		openDuration:      openDur,
	}
}

// ScreenResult represents the outcome of the Haiku screen.
type ScreenOutcome int

const (
	ScreenProceed      ScreenOutcome = iota // Score >= proceed_threshold
	ScreenSuggestBasic                      // Score >= suggest_threshold but < proceed
	ScreenReject                            // Score < suggest_threshold
	ScreenFailOpen                          // Circuit breaker or timeout
)

// ScreenResponse holds the result of the screen evaluation.
type ScreenResponse struct {
	Outcome       ScreenOutcome
	Score         int
	Rationale     string
	SuggestedMode string
}

// haikuSystemPrompt is the fixed system prompt for pre-screening.
const haikuSystemPrompt = `You are a query classifier. Rate whether the following query requires multi-step deep research (score 0-10).
Score >= 6: deep research warranted.
Score 4-5: basic synthesis sufficient.
Score < 4: simple query, reject.

Respond ONLY with valid JSON: {"score": <int>, "rationale": "<string>", "suggested_mode": "deep"|"basic"|"reject"}`

// Screen evaluates the query using the Haiku model.
// REQ-DEEP4-003: calls LLM, parses JSON, fails open on error.
// REQ-DEEP4-004: score-based branching.
func (h *HaikuScreen) Screen(ctx context.Context, query string) (ScreenResponse, error) {
	// Check circuit breaker state.
	if !h.allowRequest() {
		return ScreenResponse{Outcome: ScreenFailOpen}, nil
	}

	// Apply timeout.
	screenCtx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	response, err := h.llm.Complete(screenCtx, h.model, haikuSystemPrompt, query)
	if err != nil {
		h.onFailure()
		if h.failOpenOnTimeout {
			return ScreenResponse{Outcome: ScreenFailOpen}, nil
		}
		return ScreenResponse{}, fmt.Errorf("haiku screen: %w", err)
	}

	result, parseErr := h.parseResponse(response)
	if parseErr != nil {
		// REQ-DEEP4-003: parse failure increments counter and fails open.
		h.onFailure()
		return ScreenResponse{Outcome: ScreenFailOpen}, nil
	}

	h.onSuccess()

	return h.evaluateScore(result), nil
}

// parseResponse parses the JSON response from the Haiku model.
func (h *HaikuScreen) parseResponse(raw string) (ScreenResult, error) {
	var result ScreenResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return ScreenResult{}, fmt.Errorf("parse haiku response: %w", err)
	}
	return result, nil
}

// evaluateScore maps the score to a screen outcome.
// REQ-DEEP4-004: >= proceed_threshold proceed / >= suggest_threshold suggest basic / < reject.
func (h *HaikuScreen) evaluateScore(result ScreenResult) ScreenResponse {
	if result.Score >= h.thresholdProceed {
		return ScreenResponse{
			Outcome:       ScreenProceed,
			Score:         result.Score,
			Rationale:     result.Rationale,
			SuggestedMode: "deep",
		}
	}
	if result.Score >= h.thresholdSuggest {
		return ScreenResponse{
			Outcome:       ScreenSuggestBasic,
			Score:         result.Score,
			Rationale:     result.Rationale,
			SuggestedMode: "basic",
		}
	}
	return ScreenResponse{
		Outcome:       ScreenReject,
		Score:         result.Score,
		Rationale:     result.Rationale,
		SuggestedMode: "reject",
	}
}

// allowRequest checks the circuit breaker state.
func (h *HaikuScreen) allowRequest() bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.state == CircuitClosed {
		return true
	}
	if h.state == CircuitOpen {
		if time.Now().After(h.openUntil) {
			h.state = CircuitHalfOpen
			return true
		}
		return false
	}
	// HalfOpen: allow one request to test.
	return true
}

// onFailure records a failure for the circuit breaker.
// @MX:WARN: [AUTO] Circuit breaker failure accumulation can degrade cap effectiveness
// @MX:REASON: open window 30s means only cap-check guards during breaker-open
func (h *HaikuScreen) onFailure() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.consecutiveFails++
	if h.consecutiveFails >= h.maxFails {
		h.state = CircuitOpen
		h.openUntil = time.Now().Add(h.openDuration)
	}
}

// onSuccess resets the circuit breaker on success.
func (h *HaikuScreen) onSuccess() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.consecutiveFails = 0
	h.state = CircuitClosed
}

// State returns the current circuit breaker state (for testing/observability).
func (h *HaikuScreen) State() CircuitState {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.state
}
