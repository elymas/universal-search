package deepagent

import (
	"os"
	"testing"
)

// T-M1-001 [RED]: Config env-var loading + defaults tests
// REQ-DEEP2-004: Each agent uses env-var resolved model alias.
// Centralized in config.go. NO os.Getenv in agents.

func TestConfigLoadsAllFourModelAliasesFromEnv(t *testing.T) {
	os.Setenv("DEEP_AGENT_RESEARCHER_MODEL", "claude-3-5-haiku-test")
	os.Setenv("DEEP_AGENT_REVIEWER_MODEL", "claude-3-5-haiku-reviewer")
	os.Setenv("DEEP_AGENT_WRITER_MODEL", "claude-3-5-sonnet-writer")
	os.Setenv("DEEP_AGENT_VERIFIER_MODEL", "claude-3-5-sonnet-verifier")
	os.Setenv("DEEP_AGENT_MAX_RETRIES", "3")
	os.Setenv("DEEP_AGENT_WRITER_RETRY_DELAY_MS", "750")
	os.Setenv("DEEP_AGENT_VERIFIER_TIMEOUT_MS", "15000")
	os.Setenv("DEEP_AGENT_FAITHFULNESS_URL", "http://localhost:9000/faithfulness_check")
	t.Cleanup(func() {
		os.Unsetenv("DEEP_AGENT_RESEARCHER_MODEL")
		os.Unsetenv("DEEP_AGENT_REVIEWER_MODEL")
		os.Unsetenv("DEEP_AGENT_WRITER_MODEL")
		os.Unsetenv("DEEP_AGENT_VERIFIER_MODEL")
		os.Unsetenv("DEEP_AGENT_MAX_RETRIES")
		os.Unsetenv("DEEP_AGENT_WRITER_RETRY_DELAY_MS")
		os.Unsetenv("DEEP_AGENT_VERIFIER_TIMEOUT_MS")
		os.Unsetenv("DEEP_AGENT_FAITHFULNESS_URL")
	})

	cfg, err := NewConfigFromEnv()
	if err != nil {
		t.Fatalf("NewConfigFromEnv() returned error: %v", err)
	}

	if cfg.ResearcherModel != "claude-3-5-haiku-test" {
		t.Errorf("ResearcherModel = %q, want %q", cfg.ResearcherModel, "claude-3-5-haiku-test")
	}
	if cfg.ReviewerModel != "claude-3-5-haiku-reviewer" {
		t.Errorf("ReviewerModel = %q, want %q", cfg.ReviewerModel, "claude-3-5-haiku-reviewer")
	}
	if cfg.WriterModel != "claude-3-5-sonnet-writer" {
		t.Errorf("WriterModel = %q, want %q", cfg.WriterModel, "claude-3-5-sonnet-writer")
	}
	if cfg.VerifierModel != "claude-3-5-sonnet-verifier" {
		t.Errorf("VerifierModel = %q, want %q", cfg.VerifierModel, "claude-3-5-sonnet-verifier")
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", cfg.MaxRetries)
	}
	if cfg.WriterRetryDelayMs != 750 {
		t.Errorf("WriterRetryDelayMs = %d, want 750", cfg.WriterRetryDelayMs)
	}
	if cfg.VerifierTimeoutMs != 15000 {
		t.Errorf("VerifierTimeoutMs = %d, want 15000", cfg.VerifierTimeoutMs)
	}
	if cfg.FaithfulnessURL != "http://localhost:9000/faithfulness_check" {
		t.Errorf("FaithfulnessURL = %q, want %q", cfg.FaithfulnessURL, "http://localhost:9000/faithfulness_check")
	}
}

func TestConfigFallsBackToDefaultsWhenEnvAbsent(t *testing.T) {
	// Ensure all DEEP_AGENT_* env vars are unset.
	os.Unsetenv("DEEP_AGENT_RESEARCHER_MODEL")
	os.Unsetenv("DEEP_AGENT_REVIEWER_MODEL")
	os.Unsetenv("DEEP_AGENT_WRITER_MODEL")
	os.Unsetenv("DEEP_AGENT_VERIFIER_MODEL")
	os.Unsetenv("DEEP_AGENT_MAX_RETRIES")
	os.Unsetenv("DEEP_AGENT_WRITER_RETRY_DELAY_MS")
	os.Unsetenv("DEEP_AGENT_VERIFIER_TIMEOUT_MS")
	os.Unsetenv("DEEP_AGENT_FAITHFULNESS_URL")

	cfg, err := NewConfigFromEnv()
	if err != nil {
		t.Fatalf("NewConfigFromEnv() returned error: %v", err)
	}

	// Default model values per SPEC-DEEP-002 research.md Section 4.
	if cfg.ResearcherModel != "claude-3-5-haiku-20241022" {
		t.Errorf("ResearcherModel default = %q, want %q", cfg.ResearcherModel, "claude-3-5-haiku-20241022")
	}
	if cfg.ReviewerModel != "claude-3-5-haiku-20241022" {
		t.Errorf("ReviewerModel default = %q, want %q", cfg.ReviewerModel, "claude-3-5-haiku-20241022")
	}
	if cfg.WriterModel != "claude-3-5-sonnet-20241022" {
		t.Errorf("WriterModel default = %q, want %q", cfg.WriterModel, "claude-3-5-sonnet-20241022")
	}
	if cfg.VerifierModel != "claude-3-5-sonnet-20241022" {
		t.Errorf("VerifierModel default = %q, want %q", cfg.VerifierModel, "claude-3-5-sonnet-20241022")
	}
	if cfg.MaxRetries != 2 {
		t.Errorf("MaxRetries default = %d, want 2", cfg.MaxRetries)
	}
	if cfg.WriterRetryDelayMs != 500 {
		t.Errorf("WriterRetryDelayMs default = %d, want 500", cfg.WriterRetryDelayMs)
	}
	if cfg.VerifierTimeoutMs != 30000 {
		t.Errorf("VerifierTimeoutMs default = %d, want 30000", cfg.VerifierTimeoutMs)
	}
	if cfg.FaithfulnessURL != "http://localhost:8001/faithfulness_check" {
		t.Errorf("FaithfulnessURL default = %q, want %q", cfg.FaithfulnessURL, "http://localhost:8001/faithfulness_check")
	}
}
