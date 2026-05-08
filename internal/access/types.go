// Package access — public types for the 5-phase content-fetch cascade.
//
// REQ-CACHE-001: FetchOptions, FetchResult, FetchedContent, PhaseAttempt types.
package access

import "time"

// FetchOptions holds per-call options that override the fetcher-level defaults.
type FetchOptions struct {
	// UserAgent overrides the default user agent for this call.
	UserAgent string
	// SkipRobotsTxt disables robots.txt checking for this call (test-only).
	SkipRobotsTxt bool
	// SkipHEADProbe disables the HEAD probe step in Phase 2 (test-only).
	SkipHEADProbe bool
	// AllowPrivateNetworks overrides the fetcher-level setting per-call.
	AllowPrivateNetworks bool
}

// FetchResult is the output of a Fetch call. It is non-nil for every
// invocation that does not return ErrInvalidURL.
//
// For a successful fetch: Content is non-nil, Outcome == "success", Err == nil.
// For a partially-failed cascade: Content == nil, PhaseAttempts is populated.
// For all-failed: Content == nil, Fetch also returns ErrAllPhasesFailed.
type FetchResult struct {
	// Content is the fetched document; nil when all phases failed.
	Content *FetchedContent
	// PhaseAttempts records each phase that was attempted (including skipped).
	PhaseAttempts []PhaseAttempt
	// FinalPhase is the last phase that ran (0 = none, 1-5 = phase number).
	FinalPhase int
	// Outcome is one of: success, failure, timeout, blocked, cancelled.
	Outcome string
	// ElapsedSeconds is the total wall-clock time for the Fetch call.
	ElapsedSeconds float64
}

// FetchedContent holds the raw content returned by a successful phase.
// JSON-marshalable for diagnostic dumps.
type FetchedContent struct {
	// URL is the final URL after any redirects.
	URL string `json:"url"`
	// Body is the raw response body, capped at Options.MaxBodyBytes.
	Body []byte `json:"body,omitempty"`
	// ContentType is the MIME type reported by the server.
	ContentType string `json:"content_type,omitempty"`
	// StatusCode is the HTTP status code (0 for Phase 1 index hits).
	StatusCode int `json:"status_code,omitempty"`
	// FetchedAt is when this content was retrieved (UTC).
	FetchedAt time.Time `json:"fetched_at"`
	// Headers holds selected response headers (e.g., Cache-Control).
	Headers map[string]string `json:"headers,omitempty"`
}

// PhaseAttempt records the outcome of a single phase execution.
// JSON-marshalable for diagnostic dumps.
type PhaseAttempt struct {
	// Phase is the phase number (0 = pre-flight SSRF, 1-5 = cascade phases).
	Phase int `json:"phase"`
	// StartedAt is when this phase began.
	StartedAt time.Time `json:"started_at"`
	// ElapsedSeconds is how long this phase took.
	ElapsedSeconds float64 `json:"elapsed_seconds"`
	// Outcome is one of: success, miss, skipped, blocked, failure, timeout, cancelled.
	Outcome string `json:"outcome"`
	// Error is the serialised *FetchError on failure; empty on success.
	Error string `json:"error,omitempty"`

	// content holds the fetched content on success; unexported to keep JSON clean.
	content *FetchedContent

	// escalateTLS reports whether the phase error warrants TLS escalation.
	// Set by phase3 when a TLS handshake or WAF-pattern error is detected.
	isTLSError bool
	// isJSChallenge reports whether the phase error is a JS-challenge response.
	// Set by phase4 when a JS challenge body is detected.
	isJSChallenge bool
	// isWAF reports whether the error is a WAF-gated response.
	isWAF bool
}
