// Package llm — exponential backoff retry logic.
// REQ-LLM-004: max 3 retries, backoff 250ms/500ms/1000ms ±10% jitter.
// 401/403/400/404 are non-retryable.
package llm

import (
	"context"
	"errors"
	"math/rand"
	"net/http"
	"time"
)

// nonRetryableStatusCodes are returned immediately with no retry and no fallthrough.
var nonRetryableStatusCodes = map[int]bool{
	http.StatusBadRequest:   true, // 400
	http.StatusUnauthorized: true, // 401
	http.StatusForbidden:    true, // 403
	http.StatusNotFound:     true, // 404
}

// backoffDurations is the exact backoff schedule per REQ-LLM-004 [HARD].
var backoffDurations = []time.Duration{
	250 * time.Millisecond,
	500 * time.Millisecond,
	1000 * time.Millisecond,
}

// maxRetries is the hard limit per REQ-LLM-004.
const maxRetries = 3

// httpStatusError carries an HTTP status code for retryability checks.
type httpStatusError struct {
	code int
	msg  string
}

func (e *httpStatusError) Error() string { return e.msg }

// isNonRetryable returns true when err must not be retried and must not fall through.
func isNonRetryable(err error) bool {
	if err == nil {
		return false
	}
	var hse *httpStatusError
	if errors.As(err, &hse) {
		return nonRetryableStatusCodes[hse.code]
	}
	return false
}

// jitter adds ±10% random jitter to d.
func jitter(d time.Duration) time.Duration {
	// ±10% of d
	delta := float64(d) * 0.1
	jitterVal := (rand.Float64()*2 - 1) * delta // #nosec G404 -- non-cryptographic jitter for retry/backoff, not a security context
	return d + time.Duration(jitterVal)
}

// withRetry calls fn up to maxRetries times with exponential backoff.
// If fn returns a non-retryable error it is returned immediately.
// If ctx is cancelled during backoff, context.Canceled is returned.
//
// @MX:WARN: [AUTO] Retry loop with sleeps; must respect ctx cancellation
// @MX:REASON: failure to check ctx.Done allows goroutine to outlive request lifetime
func withRetry(ctx context.Context, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}
		if isNonRetryable(err) {
			return err
		}
		lastErr = err

		if attempt < maxRetries-1 {
			wait := jitter(backoffDurations[attempt])
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
		}
	}
	return lastErr
}
