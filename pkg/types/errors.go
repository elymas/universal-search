// Package types — adapter error taxonomy.
// REQ-CORE-008: Sentinel errors, Category enum, *SourceError, CategorizeError, OutcomeFromError.
// REQ-CORE-007: *ValidationError typed error for NormalizedDoc.Validate.
package types

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Sentinel adapter errors. Adapters and downstream consumers compare against
// these via errors.Is to make routing/retry decisions.
//
// @MX:NOTE: [AUTO] These four sentinels are the canonical category targets for
// errors.Is. Wrap raw HTTP/network errors in *SourceError to expose category
// without leaking source-specific details.
var (
	// ErrTransient indicates a temporary failure that may succeed on retry
	// (e.g., 5xx, network blip, timeout already classified at a lower layer).
	ErrTransient = errors.New("adapter: transient failure")

	// ErrPermanent indicates a non-retryable failure (e.g., 400, 401, 403, 404).
	ErrPermanent = errors.New("adapter: permanent failure")

	// ErrRateLimited indicates the source rejected the call due to quota.
	// Honour RetryAfter on *SourceError when present.
	ErrRateLimited = errors.New("adapter: rate limited")

	// ErrSourceUnavailable indicates the upstream source is unreachable
	// (DNS failure, dial timeout, 503 with no retry-after).
	ErrSourceUnavailable = errors.New("adapter: source unavailable")
)

// Category enumerates the high-level reasons an adapter call may fail.
// CategoryUnknown is reserved for nil errors and uncategorised failures.
type Category int

// Category values.
const (
	// CategoryUnknown means the error did not match any sentinel and could
	// not be classified. Maps to outcome="failure" in OutcomeFromError.
	CategoryUnknown Category = iota
	// CategoryTransient is retryable.
	CategoryTransient
	// CategoryPermanent is not retryable.
	CategoryPermanent
	// CategoryRateLimited indicates quota exhaustion.
	CategoryRateLimited
	// CategoryUnavailable indicates the source is offline.
	CategoryUnavailable
)

// String returns the lowercase canonical name of the Category.
func (c Category) String() string {
	switch c {
	case CategoryTransient:
		return "transient"
	case CategoryPermanent:
		return "permanent"
	case CategoryRateLimited:
		return "rate_limited"
	case CategoryUnavailable:
		return "unavailable"
	default:
		return "unknown"
	}
}

// SourceError carries adapter-specific context alongside a Category.
// Implementations of Adapter.Search SHOULD wrap raw errors in *SourceError
// so callers (FAN-001 retry policy, registry wrappedAdapter) can classify
// outcomes uniformly via errors.Is / errors.As.
type SourceError struct {
	// Adapter is the source identifier (matches Adapter.Name()).
	Adapter string
	// Category is the canonical error category.
	Category Category
	// HTTPStatus is the upstream HTTP status code (0 if not HTTP-based).
	HTTPStatus int
	// Cause is the underlying error. Preserved through Unwrap.
	Cause error
	// RetryAfter is the source-suggested retry delay (0 if not specified).
	RetryAfter time.Duration
}

// Error returns a formatted message including adapter name, category, and cause.
func (e *SourceError) Error() string {
	if e == nil {
		return "<nil>"
	}
	cause := "<nil cause>"
	if e.Cause != nil {
		cause = e.Cause.Error()
	}
	// Surface the upstream HTTP status when present so a bodyless 4xx/5xx
	// (Cause == nil) is still diagnosable instead of printing "<nil cause>".
	if e.HTTPStatus != 0 {
		return fmt.Sprintf("adapter %q: %s (HTTP %d): %s", e.Adapter, e.Category, e.HTTPStatus, cause)
	}
	return fmt.Sprintf("adapter %q: %s: %s", e.Adapter, e.Category, cause)
}

// Unwrap returns the underlying Cause for use with errors.Is / errors.As.
func (e *SourceError) Unwrap() error { return e.Cause }

// Is reports whether target matches this error's Category sentinel.
// Used by errors.Is(err, ErrTransient) etc.
func (e *SourceError) Is(target error) bool {
	if e == nil || target == nil {
		return false
	}
	switch e.Category {
	case CategoryTransient:
		return target == ErrTransient
	case CategoryPermanent:
		return target == ErrPermanent
	case CategoryRateLimited:
		return target == ErrRateLimited
	case CategoryUnavailable:
		return target == ErrSourceUnavailable
	}
	return false
}

// CategorizeError returns the Category for an arbitrary error. Inspection order:
//  1. nil → CategoryUnknown
//  2. *SourceError (via errors.As) → its Category
//  3. context.DeadlineExceeded → CategoryTransient
//  4. errors.Is against the four sentinels
//  5. fallback → CategoryUnknown
//
// @MX:ANCHOR: [AUTO] Error classifier; callers: registry wrappedAdapter, FAN-001 retry policy, tests
// @MX:REASON: fan_in >= 3; sole canonical mapping from error to Category
// @MX:SPEC: SPEC-CORE-001
func CategorizeError(err error) Category {
	if err == nil {
		return CategoryUnknown
	}
	var se *SourceError
	if errors.As(err, &se) {
		return se.Category
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return CategoryTransient
	}
	switch {
	case errors.Is(err, ErrTransient):
		return CategoryTransient
	case errors.Is(err, ErrPermanent):
		return CategoryPermanent
	case errors.Is(err, ErrRateLimited):
		return CategoryRateLimited
	case errors.Is(err, ErrSourceUnavailable):
		return CategoryUnavailable
	}
	return CategoryUnknown
}

// OutcomeFromError returns the canonical Prometheus outcome label value for
// the given error.
//
// Mapping:
//   - nil                              → "success"
//   - context.DeadlineExceeded         → "timeout"
//   - CategoryRateLimited              → "rate_limited"
//   - CategoryUnavailable              → "unavailable"
//   - CategoryTransient                → "transient"
//   - CategoryPermanent / Unknown / etc → "failure"
//
// The five primary values (success, failure, timeout, rate_limited,
// unavailable) plus the internal "transient" form the complete enumeration
// per NFR-CORE-002. Downstream consumers MUST NOT invent new label values.
//
// @MX:NOTE: [AUTO] Canonical mapping from error to Prometheus outcome label.
// Downstream consumers must not fabricate alternative label values.
// @MX:SPEC: SPEC-CORE-001
func OutcomeFromError(err error) string {
	if err == nil {
		return "success"
	}
	// Timeout is checked before category because context.DeadlineExceeded does
	// not carry a *SourceError envelope in the common case.
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	switch CategorizeError(err) {
	case CategoryRateLimited:
		return "rate_limited"
	case CategoryUnavailable:
		return "unavailable"
	case CategoryTransient:
		return "transient"
	default:
		return "failure"
	}
}

// ValidationError is returned by NormalizedDoc.Validate when a required field
// is missing. Recover the field name via errors.As.
// REQ-CORE-007.
type ValidationError struct {
	// Field is the name of the offending struct field (e.g., "ID").
	Field string
	// Cause is the underlying reason (e.g., "empty", "zero time").
	Cause error
}

// Error returns a formatted message describing the validation failure.
func (e *ValidationError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Cause != nil {
		return fmt.Sprintf("validation: field %q: %s", e.Field, e.Cause)
	}
	return fmt.Sprintf("validation: field %q invalid", e.Field)
}

// Unwrap returns the underlying Cause.
func (e *ValidationError) Unwrap() error { return e.Cause }
