package types_test

// Coverage for Category.String across all enum members and the ValidationError
// Error/Unwrap formatting (with and without a cause).
//
// @MX:SPEC: SPEC-REL-001 — G1 release coverage gate

import (
	"errors"
	"testing"

	"github.com/elymas/universal-search/pkg/types"
)

func TestCategoryString_AllMembers(t *testing.T) {
	tests := []struct {
		cat  types.Category
		want string
	}{
		{types.CategoryTransient, "transient"},
		{types.CategoryPermanent, "permanent"},
		{types.CategoryRateLimited, "rate_limited"},
		{types.CategoryUnavailable, "unavailable"},
		{types.Category(999), "unknown"}, // out-of-range -> default branch
	}
	for _, tt := range tests {
		if got := tt.cat.String(); got != tt.want {
			t.Errorf("Category(%d).String() = %q, want %q", tt.cat, got, tt.want)
		}
	}
}

func TestValidationError_ErrorAndUnwrap(t *testing.T) {
	cause := errors.New("empty value")

	// With a cause: message includes field + cause, and Unwrap returns it.
	withCause := &types.ValidationError{Field: "ID", Cause: cause}
	if got := withCause.Error(); got == "" {
		t.Error("ValidationError.Error() returned empty string")
	}
	if !errors.Is(withCause, cause) {
		t.Error("ValidationError must unwrap to its Cause")
	}
	if withCause.Unwrap() != cause {
		t.Error("Unwrap() must return the exact Cause")
	}

	// Without a cause: message uses the "invalid" fallback, Unwrap is nil.
	noCause := &types.ValidationError{Field: "URL"}
	if got := noCause.Error(); got == "" {
		t.Error("ValidationError.Error() (no cause) returned empty string")
	}
	if noCause.Unwrap() != nil {
		t.Error("Unwrap() must be nil when there is no Cause")
	}

	// Nil receiver must not panic.
	var nilVE *types.ValidationError
	if nilVE.Error() != "<nil>" {
		t.Errorf("nil ValidationError.Error() = %q, want <nil>", nilVE.Error())
	}
}
