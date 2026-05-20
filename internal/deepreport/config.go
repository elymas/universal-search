package deepreport

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the deepreport HTTP client.
type Config struct {
	// SidecarURL is the STORM sidecar endpoint (default: http://localhost:8001).
	SidecarURL string
	// Timeout is the per-call context timeout (default: 360s).
	Timeout time.Duration
}

// DefaultConfig returns a Config with production-safe defaults.
func DefaultConfig() Config {
	return Config{
		SidecarURL: "http://localhost:8001",
		Timeout:    360 * time.Second,
	}
}

// NewConfigFromEnv reads STORM_SIDECAR_URL and STORM_SIDECAR_TIMEOUT_SECONDS
// from environment variables. Unset values fall back to DefaultConfig values.
func NewConfigFromEnv() (Config, error) {
	cfg := DefaultConfig()

	if v := os.Getenv("STORM_SIDECAR_URL"); v != "" {
		cfg.SidecarURL = v
	}
	if v := os.Getenv("STORM_SIDECAR_TIMEOUT_SECONDS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("deepreport config: invalid STORM_SIDECAR_TIMEOUT_SECONDS %q: %w", v, err)
		}
		cfg.Timeout = time.Duration(n) * time.Second
	}

	return cfg, nil
}
