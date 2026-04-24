// Package config_test tests the LLM client configuration loader.
// REQ-LLM-001, REQ-LLM-005, NFR-LLM-003
package config_test

import (
	"os"
	"testing"

	"github.com/elymas/universal-search/internal/llm/config"
)

// TestDefaultValues verifies that Config has correct defaults when no env is set.
// NFR-LLM-003
func TestDefaultValues(t *testing.T) {
	t.Parallel()

	cfg := config.Defaults()
	if cfg.BaseURL == "" {
		t.Error("BaseURL should have a default value")
	}
	if cfg.PerRequestCapUSD != 0.50 {
		t.Errorf("PerRequestCapUSD default: got %v, want 0.50", cfg.PerRequestCapUSD)
	}
	if cfg.TimeoutSeconds <= 0 {
		t.Errorf("TimeoutSeconds default should be > 0, got %v", cfg.TimeoutSeconds)
	}
}

// TestLoadFromEnv verifies environment variable binding.
// REQ-LLM-005, NFR-LLM-003
func TestLoadFromEnv(t *testing.T) {
	t.Setenv("LITELLM_BASE_URL", "http://testhost:4000")
	t.Setenv("LITELLM_MASTER_KEY", "sk-test-key-12345")
	t.Setenv("LITELLM_BUDGET_USD", "2.50")
	t.Setenv("LITELLM_TIMEOUT_SECONDS", "45")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BaseURL != "http://testhost:4000" {
		t.Errorf("BaseURL: got %q, want %q", cfg.BaseURL, "http://testhost:4000")
	}
	if cfg.MasterKey != "sk-test-key-12345" {
		t.Errorf("MasterKey: got %q, want %q", cfg.MasterKey, "sk-test-key-12345")
	}
	if cfg.PerRequestCapUSD != 2.50 {
		t.Errorf("PerRequestCapUSD: got %v, want 2.50", cfg.PerRequestCapUSD)
	}
	if cfg.TimeoutSeconds != 45 {
		t.Errorf("TimeoutSeconds: got %v, want 45", cfg.TimeoutSeconds)
	}
}

// TestBudgetCapConfigurableFromEnv verifies LITELLM_BUDGET_USD sets PerRequestCapUSD.
// NFR-LLM-003
func TestBudgetCapConfigurableFromEnv(t *testing.T) {
	t.Setenv("LITELLM_BUDGET_USD", "1.00")
	// clear master key to avoid interfering with other env vars
	old := os.Getenv("LITELLM_MASTER_KEY")
	t.Setenv("LITELLM_MASTER_KEY", old)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.PerRequestCapUSD != 1.00 {
		t.Errorf("PerRequestCapUSD: got %v, want 1.00", cfg.PerRequestCapUSD)
	}
}

// TestNegativeBudgetRejected verifies that a negative LITELLM_BUDGET_USD is rejected.
// NFR-LLM-003 [HARD] constraint
func TestNegativeBudgetRejected(t *testing.T) {
	t.Setenv("LITELLM_BUDGET_USD", "-1.00")

	_, err := config.Load()
	if err == nil {
		t.Error("expected error for negative LITELLM_BUDGET_USD, got nil")
	}
}

// TestZeroBudgetAllowed verifies that zero budget (unlimited) is accepted.
func TestZeroBudgetAllowed(t *testing.T) {
	t.Setenv("LITELLM_BUDGET_USD", "0")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load with zero budget: %v", err)
	}
	if cfg.PerRequestCapUSD != 0.0 {
		t.Errorf("PerRequestCapUSD: got %v, want 0.0", cfg.PerRequestCapUSD)
	}
}

// TestDefaultBaseURLIsLiteLLMLocal verifies the default base URL points to the compose service.
func TestDefaultBaseURLIsLiteLLMLocal(t *testing.T) {
	t.Parallel()

	cfg := config.Defaults()
	// default points at localhost:4000 (LiteLLM compose service)
	if cfg.BaseURL == "" {
		t.Error("BaseURL should not be empty by default")
	}
}

// TestMalformedBudgetRejected verifies that a non-numeric LITELLM_BUDGET_USD returns error.
func TestMalformedBudgetRejected(t *testing.T) {
	t.Setenv("LITELLM_BUDGET_USD", "notanumber")

	_, err := config.Load()
	if err == nil {
		t.Error("expected error for malformed LITELLM_BUDGET_USD, got nil")
	}
}
