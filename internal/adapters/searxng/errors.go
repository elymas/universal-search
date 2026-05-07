// Package searxng — error sentinels and Retry-After parsing.
// REQ-ADP7-003: parseRetryAfter implements RFC 7231 §7.1.3.
// REQ-ADP7-008: ErrInvalidQuery / ErrInvalidCursor for input validation.
package searxng

import (
	"errors"
	"net/http"
	"strconv"
	"time"
)

// ErrInvalidQuery is returned by Search when the query text is empty or
// contains only Unicode whitespace runes. Wrapped in *types.SourceError
// with Category=CategoryPermanent.
var ErrInvalidQuery = errors.New("searxng: query text empty or whitespace-only")

// ErrInvalidCursor is returned by Search when the cursor is non-empty but
// does not parse as a positive integer (≥ 1). Wrapped in *types.SourceError
// with Category=CategoryPermanent.
var ErrInvalidCursor = errors.New("searxng: cursor must be positive integer page (>=1)")

// maxRetryAfter caps the Retry-After duration at 60 seconds per REQ-ADP7-003.
const maxRetryAfter = 60 * time.Second

// defaultRetryAfter is used when the Retry-After header is absent or
// malformed. 5 seconds per REQ-ADP7-003.
const defaultRetryAfter = 5 * time.Second

// parseRetryAfter parses the Retry-After header value per RFC 7231 §7.1.3.
// Supports both integer-seconds and HTTP-date forms. Returns:
//   - The parsed duration capped at maxRetryAfter (60s)
//   - defaultRetryAfter (5s) on parse failure, missing header, or non-positive value
//
// @MX:NOTE: [AUTO] RFC 7231 §7.1.3: integer-seconds tried first, then HTTP-date.
// Cap at 60s prevents the adapter from blocking callers for excessive periods.
// @MX:SPEC: SPEC-ADP-007
func parseRetryAfter(header string, now time.Time) time.Duration {
	if header == "" {
		return defaultRetryAfter
	}

	// Try integer seconds first (most common form).
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
