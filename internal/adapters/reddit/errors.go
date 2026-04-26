// Package reddit — error sentinels and Retry-After parsing.
// REQ-ADP-003: parseRetryAfter implements RFC 7231 §7.1.3.
// REQ-ADP-008: ErrInvalidQuery sentinel for empty/whitespace query rejection.
package reddit

import (
	"errors"
	"net/http"
	"strconv"
	"time"
)

// ErrInvalidQuery is returned by Search when the query text is empty or
// contains only Unicode whitespace runes. Wrapped in *types.SourceError
// with Category=CategoryPermanent.
var ErrInvalidQuery = errors.New("reddit: query text empty or whitespace-only")

// maxRetryAfter caps the Retry-After duration at 60 seconds per REQ-ADP-003.
const maxRetryAfter = 60 * time.Second

// defaultRetryAfter is used when the Retry-After header is absent or
// malformed. 5 seconds per REQ-ADP-003.
const defaultRetryAfter = 5 * time.Second

// parseRetryAfter parses the Retry-After header value per RFC 7231 §7.1.3.
// Supports both integer-seconds and HTTP-date forms. Returns:
//   - The parsed duration capped at maxRetryAfter (60s)
//   - defaultRetryAfter (5s) on parse failure, missing header, or negative value
//
// @MX:NOTE: [AUTO] RFC 7231 §7.1.3: integer-seconds tried first, then HTTP-date.
// Cap at 60s prevents the adapter from blocking callers for excessive periods.
// @MX:SPEC: SPEC-ADP-001
func parseRetryAfter(header string, now time.Time) time.Duration {
	if header == "" {
		return defaultRetryAfter
	}

	// Try integer seconds first (most common form from Reddit).
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
