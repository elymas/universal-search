// Package router_test validates the SPEC-IR-001 sentinel errors.
package router_test

import (
	"errors"
	"testing"

	"github.com/elymas/universal-search/internal/router"
)

// TestSentinelErrorsExist asserts that every documented sentinel error is
// declared, non-nil, and exposes a non-empty message. Required by REQ-IR-005
// (ErrInvalidQuery), REQ-IR-007 (ErrLLMTimeout), implicit New() validation
// (ErrAdapterRegistryEmpty), REQ-IR-003 (ErrLLMUnavailable), and REQ-IR-002
// LLM-fallback parse errors (ErrLLMParse).
func TestSentinelErrorsExist(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
	}{
		{"ErrInvalidQuery", router.ErrInvalidQuery},
		{"ErrLLMTimeout", router.ErrLLMTimeout},
		{"ErrLLMUnavailable", router.ErrLLMUnavailable},
		{"ErrAdapterRegistryEmpty", router.ErrAdapterRegistryEmpty},
		{"ErrLLMParse", router.ErrLLMParse},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.err == nil {
				t.Fatalf("%s is nil", tc.name)
			}
			if tc.err.Error() == "" {
				t.Errorf("%s has empty message", tc.name)
			}
			if !errors.Is(tc.err, tc.err) {
				t.Errorf("%s does not satisfy errors.Is identity", tc.name)
			}
		})
	}
}

// TestSentinelErrorsAreDistinct asserts each sentinel is identity-distinct
// (no two sentinels point to the same error value).
func TestSentinelErrorsAreDistinct(t *testing.T) {
	t.Parallel()

	all := []error{
		router.ErrInvalidQuery,
		router.ErrLLMTimeout,
		router.ErrLLMUnavailable,
		router.ErrAdapterRegistryEmpty,
		router.ErrLLMParse,
	}
	for i := range all {
		for j := i + 1; j < len(all); j++ {
			if errors.Is(all[i], all[j]) {
				t.Errorf("sentinels at indices %d and %d alias each other", i, j)
			}
		}
	}
}
