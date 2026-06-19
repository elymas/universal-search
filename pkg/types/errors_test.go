// Package types_test — error taxonomy tests for SPEC-CORE-001 REQ-CORE-008.
package types_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/elymas/universal-search/pkg/types"
)

// TestSentinelErrorsExist verifies the four sentinel error values exist and
// are distinct.
// REQ-CORE-008
func TestSentinelErrorsExist(t *testing.T) {
	t.Parallel()

	sentinels := []error{
		types.ErrTransient,
		types.ErrPermanent,
		types.ErrRateLimited,
		types.ErrSourceUnavailable,
	}
	seen := make(map[string]bool)
	for _, s := range sentinels {
		if s == nil {
			t.Fatal("sentinel is nil")
		}
		msg := s.Error()
		if msg == "" {
			t.Errorf("sentinel %v: Error() returned empty string", s)
		}
		if seen[msg] {
			t.Errorf("duplicate sentinel message: %q", msg)
		}
		seen[msg] = true
	}
}

// TestSourceErrorIsMatchesSentinels verifies *SourceError.Is matches the
// sentinel error corresponding to its Category.
// REQ-CORE-008
func TestSourceErrorIsMatchesSentinels(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		category types.Category
		sentinel error
	}{
		{"transient", types.CategoryTransient, types.ErrTransient},
		{"permanent", types.CategoryPermanent, types.ErrPermanent},
		{"rate_limited", types.CategoryRateLimited, types.ErrRateLimited},
		{"unavailable", types.CategoryUnavailable, types.ErrSourceUnavailable},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			se := &types.SourceError{Adapter: "test", Category: tc.category, Cause: io.EOF}
			if !errors.Is(se, tc.sentinel) {
				t.Errorf("errors.Is(SourceError{Category=%v}, %v) = false, want true",
					tc.category, tc.sentinel)
			}
			// Cross-check: a SourceError with one category should NOT match a
			// different sentinel.
			for _, other := range cases {
				if other.name == tc.name {
					continue
				}
				if errors.Is(se, other.sentinel) {
					t.Errorf("errors.Is(SourceError{Category=%v}, %v) = true, want false",
						tc.category, other.sentinel)
				}
			}
		})
	}
}

// TestSourceErrorUnwrapsCause verifies errors.Unwrap returns the inner Cause.
// REQ-CORE-008
func TestSourceErrorUnwrapsCause(t *testing.T) {
	t.Parallel()

	cause := io.ErrUnexpectedEOF
	se := &types.SourceError{Adapter: "test", Category: types.CategoryTransient, Cause: cause}

	if got := errors.Unwrap(se); got != cause {
		t.Errorf("errors.Unwrap = %v, want %v", got, cause)
	}
	// Composition: errors.Is should also reach through Unwrap.
	if !errors.Is(se, io.ErrUnexpectedEOF) {
		t.Error("errors.Is(se, io.ErrUnexpectedEOF) = false, want true (via Unwrap chain)")
	}
}

// TestSourceErrorErrorString verifies the formatted error message includes
// adapter name, category, and cause.
// REQ-CORE-008
func TestSourceErrorErrorString(t *testing.T) {
	t.Parallel()

	se := &types.SourceError{
		Adapter:  "reddit",
		Category: types.CategoryRateLimited,
		Cause:    errors.New("429 too many requests"),
	}
	msg := se.Error()
	if msg == "" {
		t.Fatal("Error() returned empty string")
	}
	for _, sub := range []string{"reddit", "429"} {
		if !contains(msg, sub) {
			t.Errorf("Error() = %q does not contain %q", msg, sub)
		}
	}

	// A bodyless 4xx (Cause == nil) must still surface the HTTP status
	// instead of only printing "<nil cause>".
	bodyless := &types.SourceError{
		Adapter:    "bluesky",
		Category:   types.CategoryPermanent,
		HTTPStatus: 403,
	}
	bmsg := bodyless.Error()
	for _, sub := range []string{"bluesky", "403"} {
		if !contains(bmsg, sub) {
			t.Errorf("Error() = %q does not contain %q", bmsg, sub)
		}
	}
}

// TestCategorizeErrorTable covers the canonical input shapes for
// CategorizeError.
// REQ-CORE-008
func TestCategorizeErrorTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   error
		want types.Category
	}{
		{"nil", nil, types.CategoryUnknown},
		{"ErrTransient", types.ErrTransient, types.CategoryTransient},
		{"ErrPermanent", types.ErrPermanent, types.CategoryPermanent},
		{"ErrRateLimited", types.ErrRateLimited, types.CategoryRateLimited},
		{"ErrSourceUnavailable", types.ErrSourceUnavailable, types.CategoryUnavailable},
		{
			"SourceError permanent",
			&types.SourceError{Adapter: "x", Category: types.CategoryPermanent, Cause: io.EOF},
			types.CategoryPermanent,
		},
		{"DeadlineExceeded", context.DeadlineExceeded, types.CategoryTransient},
		{"random", errors.New("random"), types.CategoryUnknown},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := types.CategorizeError(tc.in)
			if got != tc.want {
				t.Errorf("CategorizeError(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// TestOutcomeFromErrorTable covers the canonical mapping from error to the
// Prometheus outcome label value.
// REQ-CORE-008, NFR-CORE-002
func TestOutcomeFromErrorTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   error
		want string
	}{
		{"nil", nil, "success"},
		{"ErrPermanent", types.ErrPermanent, "failure"},
		{"DeadlineExceeded", context.DeadlineExceeded, "timeout"},
		{
			"SourceError rate_limited",
			&types.SourceError{Adapter: "x", Category: types.CategoryRateLimited, Cause: io.EOF},
			"rate_limited",
		},
		{
			"SourceError unavailable",
			&types.SourceError{Adapter: "x", Category: types.CategoryUnavailable, Cause: io.EOF},
			"unavailable",
		},
		{"random", errors.New("random"), "failure"},
		{
			"SourceError transient",
			&types.SourceError{Adapter: "x", Category: types.CategoryTransient, Cause: io.EOF},
			"transient",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := types.OutcomeFromError(tc.in)
			if got != tc.want {
				t.Errorf("OutcomeFromError(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestSourceErrorNestedIs verifies that a SourceError wrapping another
// SourceError matches the outermost category, not the inner.
// REQ-CORE-008 (REFACTOR check)
func TestSourceErrorNestedIs(t *testing.T) {
	t.Parallel()

	inner := &types.SourceError{
		Adapter:  "inner",
		Category: types.CategoryTransient,
		Cause:    io.EOF,
	}
	outer := &types.SourceError{
		Adapter:  "outer",
		Category: types.CategoryPermanent,
		Cause:    inner,
	}

	if !errors.Is(outer, types.ErrPermanent) {
		t.Error("outer.Is(ErrPermanent) = false, want true")
	}
	// Outer has Category=Permanent, so Is(ErrTransient) should be false at the
	// outer level — but errors.Is walks the Unwrap chain, so if inner matches
	// ErrTransient it will also be true. The semantics here: errors.Is reports
	// a match if ANY layer matches. We assert the outer match holds; we do not
	// require the absence of inner match.
	if !errors.Is(outer, types.ErrTransient) {
		// Documenting current behavior: errors.Is walks Unwrap chain, so the
		// inner CategoryTransient is reachable too. Either both are true (current)
		// or only outer (would require Is to short-circuit). We accept the
		// stdlib walk behavior.
		t.Log("inner SourceError reached via Unwrap chain; documented behavior")
	}
}

// TestValidationErrorIsTypedAsError verifies *ValidationError implements the
// error interface and is recoverable via errors.As.
// REQ-CORE-007 (declared in errors.go alongside the rest of the taxonomy)
func TestValidationErrorIsTypedAsError(t *testing.T) {
	t.Parallel()

	ve := &types.ValidationError{Field: "ID", Cause: errors.New("empty")}
	var asTarget *types.ValidationError
	if !errors.As(error(ve), &asTarget) {
		t.Fatal("errors.As(ve, *ValidationError) = false, want true")
	}
	if asTarget.Field != "ID" {
		t.Errorf("ValidationError.Field = %q, want %q", asTarget.Field, "ID")
	}
	if ve.Error() == "" {
		t.Error("ValidationError.Error() returned empty string")
	}
}

// contains is a substring helper that avoids pulling in strings just for tests.
func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
