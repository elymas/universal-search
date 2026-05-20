// Package synthesis — environment configuration for the synthesis client.
//
// Mirrors the pattern at internal/llm/config/config.go.
// REQ-SYN-005: RESEARCHER_BASE_URL + RESEARCHER_REQUEST_TIMEOUT_SECONDS.
package synthesis

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the synthesis HTTP client.
//
// @MX:ANCHOR: [AUTO] Synthesis client config; callers: New, cmd/usearch, client_test
// @MX:REASON: fan_in >= 3; single struct holds all sidecar connection parameters.
type Config struct {
	// BaseURL is the researcher sidecar endpoint (default: http://localhost:8081).
	BaseURL string
	// RequestTimeout is the per-call context timeout (default: 10s).
	RequestTimeout time.Duration
}

// DefaultConfig returns a Config with production-safe defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:        "http://localhost:8081",
		RequestTimeout: 10 * time.Second,
	}
}

// LoadConfig reads RESEARCHER_BASE_URL and RESEARCHER_REQUEST_TIMEOUT_SECONDS
// from environment variables. Unset values fall back to DefaultConfig values.
func LoadConfig() (Config, error) {
	cfg := DefaultConfig()

	if v := os.Getenv("RESEARCHER_BASE_URL"); v != "" {
		cfg.BaseURL = v
	}
	if v := os.Getenv("RESEARCHER_REQUEST_TIMEOUT_SECONDS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("synthesis config: invalid RESEARCHER_REQUEST_TIMEOUT_SECONDS %q: %w", v, err)
		}
		cfg.RequestTimeout = time.Duration(n) * time.Second
	}

	return cfg, nil
}
