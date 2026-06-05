// Package meta — error sentinels and Retry-After parsing.
// REQ-ADP10-003: parseRetryAfter implements RFC 7231 §7.1.3.
// REQ-ADP10-008: ErrFacebookNotSupported / ErrFacebookDisabled sentinels.
// REQ-ADP10-009: ErrInvalidQuery sentinel for empty-query rejection.
package meta

import (
	"errors"
	"net/http"
	"strconv"
	"time"
)

// ErrInvalidQuery is returned when query text is empty or whitespace-only.
var ErrInvalidQuery = errors.New("meta: query text empty or whitespace-only")

// ErrThreadsTokenMissing is returned by NewThreads when neither opts.AccessToken
// nor the THREADS_ACCESS_TOKEN environment variable provides a non-empty token.
var ErrThreadsTokenMissing = errors.New("meta/threads: THREADS_ACCESS_TOKEN not set")

// ErrFacebookNotSupported is the permanent error returned by Facebook Search.
// The message documents the external blocker so operators understand this is
// a permanent platform limitation, not a transient failure or missing config.
var ErrFacebookNotSupported = errors.New("meta/facebook: the official Facebook Graph API exposes no public-post keyword search endpoint; scraping excluded per tech.md:147")

// ErrFacebookDisabled is returned by Facebook Healthcheck (no endpoint to probe).
var ErrFacebookDisabled = errors.New("meta/facebook: adapter disabled — no viable Facebook search path")

// maxRetryAfter caps the Retry-After duration at 60 seconds.
const maxRetryAfter = 60 * time.Second

// defaultRetryAfter is used when the Retry-After header is absent or malformed.
const defaultRetryAfter = 5 * time.Second

// parseRetryAfter parses the Retry-After header value per RFC 7231 §7.1.3.
// Supports both integer-seconds and HTTP-date forms. Returns the parsed
// duration capped at maxRetryAfter (60s), or defaultRetryAfter (5s) on failure.
func parseRetryAfter(header string, now time.Time) time.Duration {
	if header == "" {
		return defaultRetryAfter
	}

	// Try integer seconds first.
	if secs, err := strconv.Atoi(header); err == nil {
		if secs <= 0 {
			return defaultRetryAfter
		}
		d := time.Duration(secs) * time.Second
		if d > maxRetryAfter {
			return maxRetryAfter
		}
		return d
	}

	// Try HTTP-date form (e.g., "Wed, 21 Oct 2026 07:28:00 GMT").
	t, err := http.ParseTime(header)
	if err != nil {
		return defaultRetryAfter
	}
	d := t.Sub(now)
	if d <= 0 {
		return defaultRetryAfter
	}
	if d > maxRetryAfter {
		return maxRetryAfter
	}
	return d
}
