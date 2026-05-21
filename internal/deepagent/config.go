// Package deepagent provides the multi-agent /deep pipeline
// (Researcher/Reviewer/Writer/Verifier) for SPEC-DEEP-002.
//
// REQ-DEEP2-004: Each agent uses env-var resolved model alias.
// All DEEP_AGENT_* env-vars are loaded here; individual agents
// must not call os.Getenv directly.
package deepagent

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all configuration for the deep agent pipeline.
// @MX:NOTE: [AUTO] All DEEP_AGENT_* env-vars loaded here; agents must not call os.Getenv (REQ-DEEP2-004)
type Config struct {
	// ResearcherModel is the LiteLLM model alias for the Researcher agent.
	ResearcherModel string
	// ReviewerModel is the LiteLLM model alias for the Reviewer agent.
	ReviewerModel string
	// WriterModel is the LiteLLM model alias for the Writer agent.
	WriterModel string
	// VerifierModel is the LiteLLM model alias for the Verifier agent.
	VerifierModel string
	// MaxRetries is the maximum number of Writer retries on Verifier rejection.
	// Total attempts = MaxRetries + 1 (default: 2 retries = 3 total attempts).
	MaxRetries int
	// WriterRetryDelayMs is the backoff delay between Writer retries in milliseconds.
	WriterRetryDelayMs int
	// VerifierTimeoutMs is the timeout for the faithfulness check call in milliseconds.
	VerifierTimeoutMs int
	// FaithfulnessURL is the URL for the researcher sidecar faithfulness endpoint.
	FaithfulnessURL string
}

// DefaultConfig returns a Config with production-safe defaults
// per SPEC-DEEP-002 research.md Section 4.
func DefaultConfig() Config {
	return Config{
		ResearcherModel:    "claude-3-5-haiku-20241022",
		ReviewerModel:      "claude-3-5-haiku-20241022",
		WriterModel:        "claude-3-5-sonnet-20241022",
		VerifierModel:      "claude-3-5-sonnet-20241022",
		MaxRetries:         2,
		WriterRetryDelayMs: 500,
		VerifierTimeoutMs:  30000,
		FaithfulnessURL:    "http://localhost:8001/faithfulness_check",
	}
}

// NewConfigFromEnv reads all DEEP_AGENT_* environment variables.
// Unset values fall back to DefaultConfig values.
func NewConfigFromEnv() (Config, error) {
	cfg := DefaultConfig()

	if v := os.Getenv("DEEP_AGENT_RESEARCHER_MODEL"); v != "" {
		cfg.ResearcherModel = v
	}
	if v := os.Getenv("DEEP_AGENT_REVIEWER_MODEL"); v != "" {
		cfg.ReviewerModel = v
	}
	if v := os.Getenv("DEEP_AGENT_WRITER_MODEL"); v != "" {
		cfg.WriterModel = v
	}
	if v := os.Getenv("DEEP_AGENT_VERIFIER_MODEL"); v != "" {
		cfg.VerifierModel = v
	}
	if v := os.Getenv("DEEP_AGENT_MAX_RETRIES"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("deepagent config: invalid DEEP_AGENT_MAX_RETRIES %q: %w", v, err)
		}
		cfg.MaxRetries = n
	}
	if v := os.Getenv("DEEP_AGENT_WRITER_RETRY_DELAY_MS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("deepagent config: invalid DEEP_AGENT_WRITER_RETRY_DELAY_MS %q: %w", v, err)
		}
		cfg.WriterRetryDelayMs = n
	}
	if v := os.Getenv("DEEP_AGENT_VERIFIER_TIMEOUT_MS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("deepagent config: invalid DEEP_AGENT_VERIFIER_TIMEOUT_MS %q: %w", v, err)
		}
		cfg.VerifierTimeoutMs = n
	}
	if v := os.Getenv("DEEP_AGENT_FAITHFULNESS_URL"); v != "" {
		cfg.FaithfulnessURL = v
	}

	return cfg, nil
}
