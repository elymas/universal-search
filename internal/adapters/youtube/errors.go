// Package youtube — error sentinels and Retry-After parsing.
// REQ-ADP5-003: parseRetryAfter implements RFC 7231 §7.1.3 with 30s default.
// REQ-ADP5-008: ErrInvalidQuery / ErrInvalidCursor / ErrCursorOverCap sentinels.
package youtube

import (
	"errors"
	"net/http"
	"strconv"
	"time"
)

// ErrInvalidQuery is returned by Search when the query text is empty or
// contains only Unicode whitespace runes. Wrapped in *types.SourceError
// with Category=CategoryPermanent.
var ErrInvalidQuery = errors.New("youtube: query text empty or whitespace-only")

// ErrInvalidCursor is returned by Search when q.Cursor is non-empty and
// does not parse as a non-negative integer. Wrapped in *types.SourceError
// with Category=CategoryPermanent.
var ErrInvalidCursor = errors.New("youtube: cursor must be non-negative integer offset")

// ErrCursorOverCap is returned by Search when max_results + cursor offset > 100.
// The cap bounds the yt-dlp ytsearchN: re-query cost per D7.
//
// @MX:NOTE: [AUTO] The 100-item pagination cap. D7 and Open Question §11.4
// document the revisit trigger: if telemetry shows users paginating heavily
// beyond 100, raise the cap with a sidecar performance test first.
// @MX:SPEC: SPEC-ADP-005
var ErrCursorOverCap = errors.New("youtube: max_results + cursor offset > 100")

// maxRetryAfter caps the Retry-After duration at 60 seconds per REQ-ADP5-003.
const maxRetryAfter = 60 * time.Second

// defaultRetryAfter is used when the Retry-After header is absent or
// malformed. 30s per REQ-ADP5-003 — longer than HN/Reddit's 5s because
// YouTube IP-blocks tend to last longer per https://github.com/yt-dlp/yt-dlp/issues/10128.
const defaultRetryAfter = 30 * time.Second

// parseRetryAfter parses the Retry-After header value per RFC 7231 §7.1.3.
// Supports both integer-seconds and HTTP-date forms. Returns:
//   - The parsed duration capped at maxRetryAfter (60s)
//   - defaultRetryAfter (30s) on parse failure, missing header, or negative value
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
