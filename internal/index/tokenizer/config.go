package tokenizer

import (
	"os"
	"strconv"
	"time"
)

// Config holds all tuneable parameters for the tokenizer Client.
type Config struct {
	// BaseURL is the HTTP base URL of the sidecar (e.g. "http://localhost:8083").
	// Populated from TOKENIZER_KO_BASE_URL; defaults to "http://localhost:8083".
	BaseURL string

	// Timeout is the per-request deadline including retries.
	// Populated from TOKENIZER_KO_TIMEOUT_MS (milliseconds); defaults to 500ms.
	Timeout time.Duration

	// MaxRetries is the number of additional attempts after the first failure.
	// Populated from TOKENIZER_KO_MAX_RETRIES; defaults to 2.
	MaxRetries int

	// RetryBaseDelay is the initial back-off delay between attempts.
	// Subsequent attempts multiply by 3x (100ms → 300ms).
	// Defaults to 100ms.
	RetryBaseDelay time.Duration
}

// DefaultConfig returns a Config populated from environment variables, falling
// back to documented defaults when a variable is absent or malformed.
func DefaultConfig() Config {
	cfg := Config{
		BaseURL:        "http://localhost:8083",
		Timeout:        500 * time.Millisecond,
		MaxRetries:     2,
		RetryBaseDelay: 100 * time.Millisecond,
	}

	if v := os.Getenv("TOKENIZER_KO_BASE_URL"); v != "" {
		cfg.BaseURL = v
	}
	if v := os.Getenv("TOKENIZER_KO_TIMEOUT_MS"); v != "" {
		if ms, err := strconv.ParseInt(v, 10, 64); err == nil && ms > 0 {
			cfg.Timeout = time.Duration(ms) * time.Millisecond
		}
	}
	if v := os.Getenv("TOKENIZER_KO_MAX_RETRIES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			cfg.MaxRetries = n
		}
	}

	return cfg
}
