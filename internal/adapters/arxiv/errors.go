package arxiv

import (
	"errors"
	"net/http"
	"strconv"
	"time"
)

// ErrInvalidQuery is returned when the query text is empty or whitespace-only.
var ErrInvalidQuery = errors.New("arxiv: query text empty or whitespace-only")

// ErrInvalidStart is returned when the cursor (start parameter) is not a valid
// non-negative integer string.
var ErrInvalidStart = errors.New("arxiv: cursor is not a valid non-negative integer")

const (
	maxRetryAfter     = 60 * time.Second
	defaultRetryAfter = 5 * time.Second
)

// parseRetryAfter parses a Retry-After header value per RFC 7231 §7.1.3.
// It tries integer seconds first, then HTTP-date. Returns defaultRetryAfter
// on any parse failure. Caps the result at maxRetryAfter.
func parseRetryAfter(header string, now time.Time) time.Duration {
	if header == "" {
		return defaultRetryAfter
	}

	// Try integer seconds.
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

	// Try HTTP-date.
	if t, err := http.ParseTime(header); err == nil {
		d := t.Sub(now)
		if d <= 0 {
			return defaultRetryAfter
		}
		if d > maxRetryAfter {
			return maxRetryAfter
		}
		return d
	}

	return defaultRetryAfter
}
