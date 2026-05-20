// Package embedder — environment-based configuration.
//
// Mirrors internal/synthesis/config.go pattern per SPEC-IDX-002 §2.1(i).
package embedder

import (
	"os"
	"strconv"
	"time"
)

// Config holds configuration for the embedder HTTP client.
type Config struct {
	// BaseURL is the HTTP base URL for the embedding sidecar.
	// Env: EMBEDDER_BASE_URL (default: http://localhost:8082).
	BaseURL string

	// RequestTimeout is the wall-clock timeout per top-level Embed call.
	// Env: EMBEDDER_REQUEST_TIMEOUT_SECONDS (default: 15).
	RequestTimeout time.Duration
}

// ConfigFromEnv builds a Config from environment variables.
func ConfigFromEnv() Config {
	baseURL := os.Getenv("EMBEDDER_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8082"
	}

	timeoutSecs := 15
	if v := os.Getenv("EMBEDDER_REQUEST_TIMEOUT_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			timeoutSecs = n
		}
	}

	return Config{
		BaseURL:        baseURL,
		RequestTimeout: time.Duration(timeoutSecs) * time.Second,
	}
}
