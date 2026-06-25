// Package redditrss — Options for the Reddit RSS adapter.
// SPEC-ADP-001b REQ-ADP1B-001..004.
package redditrss

import (
	"net/http"
	"time"
)

const (
	// defaultBaseURL is the production Reddit search RSS endpoint base.
	defaultBaseURL = "https://www.reddit.com/search.rss"

	// defaultTimeout is the per-request timeout (REQ-ADP1B-004).
	defaultTimeout = 10 * time.Second

	// defaultCooldown is the wait before re-issuing a request after HTTP 429.
	// Reddit's anonymous RSS endpoint rate-limits hard (~1 req/short window/IP),
	// so a single Search secures this cooldown before its next attempt. A
	// Retry-After header, when larger, takes precedence.
	defaultCooldown = 5 * time.Second

	// defaultMaxAttempts is the total number of Search attempts (initial + retries)
	// made on repeated HTTP 429 responses before giving up.
	defaultMaxAttempts = 3

	// defaultUserAgentVersion is the version string embedded in the default UA.
	defaultUserAgentVersion = "0.1.0"

	// userAgentTemplate mirrors the OAuth adapter's UA format at
	// internal/adapters/reddit/reddit.go:23 but with reddit-rss branding.
	// Format: usearch-reddit-rss/<version> (+https://...)
	userAgentTemplate = "usearch-reddit-rss/%s (+https://github.com/elymas/universal-search)"
)

// Options configures the Reddit RSS adapter. All zero-value fields receive
// defaults applied by applyDefaults.
type Options struct {
	// BaseURL overrides the production search.rss base for testing
	// (REQ-ADP1B-002). Set to an httptest.Server URL in tests.
	BaseURL string

	// UserAgent overrides the default User-Agent header (REQ-ADP1B-003).
	// When empty the default UA is used.
	UserAgent string

	// UserAgentVersion fills the %s slot in the default UA template.
	// Defaults to defaultUserAgentVersion.
	UserAgentVersion string

	// Timeout is the per-request deadline (REQ-ADP1B-004).
	// Defaults to 10 seconds.
	Timeout time.Duration

	// HTTPClient overrides the default http.Client. When nil the default
	// client (with redirect allowlist) is constructed automatically.
	HTTPClient *http.Client

	// NowFunc is the clock source for RetrievedAt. Defaults to time.Now.
	NowFunc func() time.Time
}

// applyDefaults fills zero-value fields with production defaults.
func (o *Options) applyDefaults() {
	if o.BaseURL == "" {
		o.BaseURL = defaultBaseURL
	}
	if o.Timeout <= 0 {
		o.Timeout = defaultTimeout
	}
	if o.UserAgentVersion == "" {
		o.UserAgentVersion = defaultUserAgentVersion
	}
	if o.NowFunc == nil {
		o.NowFunc = time.Now
	}
}
