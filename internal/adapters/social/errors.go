// Package social — error sentinels and Retry-After parsing.
// REQ-ADP6-003: parseRetryAfter implements RFC 7231 §7.1.3.
// REQ-ADP6-007: ErrXDisabled / ErrXProviderNotConfigured sentinels.
package social

import (
	"net/http"
	"strconv"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// ErrXDisabled is returned by X Search/Healthcheck when the X_ENABLED env var
// is not set (or empty). Wrapped in *types.SourceError with CategoryPermanent.
//
// @MX:NOTE: [AUTO] Sentinel for X-disabled state; check with errors.Is.
// @MX:SPEC: SPEC-ADP-006
var ErrXDisabled = &types.SourceError{
	Adapter:  "x",
	Category: types.CategoryPermanent,
	Cause:    errXDisabledCause,
}

// ErrXProviderNotConfigured is returned by X Search when X_ENABLED="true" but
// no provider is wired. Wrapped in *types.SourceError with CategoryPermanent.
//
// @MX:NOTE: [AUTO] Sentinel for X-enabled-but-unimplemented state.
// @MX:SPEC: SPEC-ADP-006
var ErrXProviderNotConfigured = &types.SourceError{
	Adapter:  "x",
	Category: types.CategoryPermanent,
	Cause:    errXProviderNotConfiguredCause,
}

// Underlying cause errors for the sentinels above.
// These are private; callers use errors.Is against the sentinel vars.
var (
	errXDisabledCause              = xSentinelError("x: X adapter is disabled (X_ENABLED env var not set)")
	errXProviderNotConfiguredCause = xSentinelError("x: X adapter enabled but no provider wired")
)

// xSentinelError is a string-based error type for X sentinel causes.
type xSentinelError string

func (e xSentinelError) Error() string { return string(e) }

// maxRetryAfter caps the Retry-After duration at 60 seconds per REQ-ADP6-003.
const maxRetryAfter = 60 * time.Second

// defaultRetryAfter is used when the Retry-After header is absent or malformed.
const defaultRetryAfter = 5 * time.Second

// parseRetryAfter parses the Retry-After header value per RFC 7231 §7.1.3.
// Supports both integer-seconds and HTTP-date forms. Returns:
//   - The parsed duration capped at maxRetryAfter (60s)
//   - defaultRetryAfter (5s) on parse failure, missing header, or non-positive value
//
// @MX:NOTE: [AUTO] RFC 7231 §7.1.3: integer-seconds tried first, then HTTP-date.
// @MX:SPEC: SPEC-ADP-006
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
