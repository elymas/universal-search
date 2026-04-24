// Package config provides configuration loading for the LLM client.
// It reads LITELLM_* environment variables and provides validated Config.
//
// REQ-LLM-005: MasterKey from LITELLM_MASTER_KEY
// NFR-LLM-003: PerRequestCapUSD from LITELLM_BUDGET_USD (default 0.50)
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all configuration for the LLM client.
// @MX:ANCHOR: [AUTO] LLM client config; callers: llm.New, cmd/usearch, config_test
// @MX:REASON: fan_in >= 3; single struct holds all LLM gateway parameters
type Config struct {
	// BaseURL is the LiteLLM proxy endpoint (default: http://localhost:4000).
	BaseURL string
	// MasterKey is the Bearer token for the LiteLLM proxy (LITELLM_MASTER_KEY).
	MasterKey string
	// PerRequestCapUSD is the per-request cost cap in USD (LITELLM_BUDGET_USD, default 0.50).
	// Set to 0 to disable cap checking.
	PerRequestCapUSD float64
	// TimeoutSeconds is the HTTP timeout for LLM requests (default 30s).
	TimeoutSeconds int
}

// Defaults returns a Config with production-safe defaults.
func Defaults() Config {
	return Config{
		BaseURL:          "http://localhost:4000",
		PerRequestCapUSD: 0.50,
		TimeoutSeconds:   30,
	}
}

// Load reads configuration from environment variables, applying defaults
// for any unset values. Returns error if env values are invalid.
func Load() (Config, error) {
	cfg := Defaults()

	if v := os.Getenv("LITELLM_BASE_URL"); v != "" {
		cfg.BaseURL = v
	}
	if v := os.Getenv("LITELLM_MASTER_KEY"); v != "" {
		cfg.MasterKey = v
	}
	if v := os.Getenv("LITELLM_TIMEOUT_SECONDS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("config: invalid LITELLM_TIMEOUT_SECONDS %q: %w", v, err)
		}
		cfg.TimeoutSeconds = n
	}
	if v := os.Getenv("LITELLM_BUDGET_USD"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return Config{}, fmt.Errorf("config: invalid LITELLM_BUDGET_USD %q: %w", v, err)
		}
		if f < 0 {
			return Config{}, fmt.Errorf("config: LITELLM_BUDGET_USD must be >= 0, got %v", f)
		}
		cfg.PerRequestCapUSD = f
	}

	return cfg, nil
}
