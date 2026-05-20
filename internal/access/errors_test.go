// Package access — unit tests for error types and sentinel errors.
//
// REQ-CACHE-001: ErrAllPhasesFailed, ErrShuttingDown
// REQ-CACHE-016: ErrInvalidURL
package access

import (
	"errors"
	"testing"
)

func TestFetchError_Error_WithCause(t *testing.T) {
	t.Parallel()
	cause := errors.New("root cause")
	fe := &FetchError{
		Category: CategoryUnavailable,
		Reason:   "network error",
		Cause:    cause,
	}
	got := fe.Error()
	want := "unavailable: network error: root cause"
	if got != want {
		t.Errorf("FetchError.Error() = %q, want %q", got, want)
	}
}

func TestFetchError_Error_WithoutCause(t *testing.T) {
	t.Parallel()
	fe := &FetchError{
		Category: CategoryPermanent,
		Reason:   "404 not found",
	}
	got := fe.Error()
	want := "permanent: 404 not found"
	if got != want {
		t.Errorf("FetchError.Error() = %q, want %q", got, want)
	}
}

func TestFetchError_Unwrap(t *testing.T) {
	t.Parallel()
	cause := errors.New("root")
	fe := &FetchError{Cause: cause}
	if !errors.Is(fe, cause) {
		t.Errorf("errors.Is(fe, cause) = false, want true")
	}
}

func TestSentinelErrors_NotNil(t *testing.T) {
	t.Parallel()
	sentinels := []error{
		ErrAllPhasesFailed,
		ErrPhaseNotApplicable,
		ErrPlaywrightUnavailable,
		ErrShuttingDown,
		ErrInvalidURL,
	}
	for _, e := range sentinels {
		if e == nil {
			t.Errorf("sentinel error must not be nil: got nil")
		}
	}
}

func TestErrorCategories_StringValues(t *testing.T) {
	t.Parallel()
	cases := []struct {
		cat  ErrorCategory
		want string
	}{
		{CategoryBlocked, "blocked"},
		{CategoryPermanent, "permanent"},
		{CategoryRateLimited, "rate_limited"},
		{CategoryUnavailable, "unavailable"},
		{CategoryTimeout, "timeout"},
	}
	for _, tc := range cases {
		if string(tc.cat) != tc.want {
			t.Errorf("ErrorCategory(%q) string = %q, want %q", tc.cat, string(tc.cat), tc.want)
		}
	}
}
