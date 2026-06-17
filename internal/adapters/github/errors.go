// Package github — error sentinels and Retry-After parsing.
// REQ-ADP4-003: parseRetryAfter implements RFC 7231 §7.1.3 with 90s cap.
// REQ-ADP4-008: ErrInvalidQuery / ErrInvalidCursor / ErrInvalidIntent sentinels.
package github

import (
	"errors"
	"net/http"
	"strconv"
	"time"
)

// ErrMissingToken is returned by New when Token is empty and SkipAuthCheck is false.
var ErrMissingToken = errors.New("github: token required; set USEARCH_GITHUB_TOKEN env var")

// ErrInvalidQuery is returned by Search when query text is empty or whitespace-only.
var ErrInvalidQuery = errors.New("github: query text empty or whitespace-only")

// ErrInvalidCursor is returned by Search when Cursor is non-empty but not a positive integer.
var ErrInvalidCursor = errors.New("github: cursor must be a positive integer page number")

// ErrInvalidIntent is returned by Search when kind filter is set to an unrecognized value.
var ErrInvalidIntent = errors.New("github: kind filter must be one of: code, issues, repos, commit")

// maxRetryAfter caps the Retry-After duration at 90 seconds per REQ-ADP4-003.
// GitHub's secondary-rate-limit documentation recommends several minutes; 90s is a
// conservative ceiling that keeps user-facing tail latency reasonable.
const maxRetryAfter = 90 * time.Second

// defaultRetryAfter is used when the Retry-After header is absent or
// malformed. 5 seconds, consistent with ADP-001/002.
const defaultRetryAfter = 5 * time.Second

// parseRetryAfter parses the Retry-After header value per RFC 7231 §7.1.3.
// Supports both integer-seconds and HTTP-date forms. Returns:
//   - The parsed duration capped at maxRetryAfter (90s)
//   - defaultRetryAfter (5s) on parse failure, missing header, or negative value
func parseRetryAfter(header string, now time.Time) time.Duration {
	if header == "" {
		return defaultRetryAfter
	}

	// Try integer seconds first (most common).
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

	// Try HTTP-date form.
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
