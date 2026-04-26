// Package router_test validates the outcome label enumeration and helper.
package router_test

import (
	"context"
	"errors"
	"testing"

	"github.com/elymas/universal-search/internal/router"
)

// TestOutcomeFromDecisionTable enumerates all 10 outcome paths.
func TestOutcomeFromDecisionTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		dec  router.RoutingDecision
		err  error
		want string
	}{
		{
			name: "web",
			dec:  router.RoutingDecision{Category: router.CategoryWeb},
			want: "classified_web",
		},
		{
			name: "social",
			dec:  router.RoutingDecision{Category: router.CategorySocial},
			want: "classified_social",
		},
		{
			name: "academic",
			dec:  router.RoutingDecision{Category: router.CategoryAcademic},
			want: "classified_academic",
		},
		{
			name: "korean",
			dec:  router.RoutingDecision{Category: router.CategoryKorean},
			want: "classified_korean",
		},
		{
			name: "mixed",
			dec:  router.RoutingDecision{Category: router.CategoryMixed},
			want: "classified_mixed",
		},
		{
			name: "unknown",
			dec:  router.RoutingDecision{Category: router.CategoryUnknown},
			want: "classified_unknown",
		},
		{
			name: "error_invalid",
			err:  router.ErrInvalidQuery,
			want: "error_invalid",
		},
		{
			name: "error_timeout",
			err:  router.ErrLLMTimeout,
			want: "error_timeout",
		},
		{
			name: "error_breaker_open",
			err:  router.ErrLLMUnavailable,
			want: "error_breaker_open",
		},
		{
			name: "error_parse",
			err:  router.ErrLLMParse,
			want: "error_parse",
		},
		{
			name: "context_canceled_falls_through_to_invalid_when_no_dec",
			err:  context.Canceled,
			want: "error_invalid",
		},
		{
			name: "wrapped_unavailable_via_errorsIs",
			err:  errors.Join(router.ErrLLMUnavailable, errors.New("provider down")),
			want: "error_breaker_open",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := router.OutcomeFromDecision(tc.dec, tc.err)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestOutcomeConstantsExposed asserts each outcome constant is exported and
// matches the documented spec.md §5.5 string.
func TestOutcomeConstantsExposed(t *testing.T) {
	t.Parallel()

	want := map[string]string{
		"OutcomeClassifiedWeb":      router.OutcomeClassifiedWeb,
		"OutcomeClassifiedSocial":   router.OutcomeClassifiedSocial,
		"OutcomeClassifiedAcademic": router.OutcomeClassifiedAcademic,
		"OutcomeClassifiedKorean":   router.OutcomeClassifiedKorean,
		"OutcomeClassifiedMixed":    router.OutcomeClassifiedMixed,
		"OutcomeClassifiedUnknown":  router.OutcomeClassifiedUnknown,
		"OutcomeErrorInvalid":       router.OutcomeErrorInvalid,
		"OutcomeErrorTimeout":       router.OutcomeErrorTimeout,
		"OutcomeErrorBreakerOpen":   router.OutcomeErrorBreakerOpen,
		"OutcomeErrorParse":         router.OutcomeErrorParse,
	}
	expected := map[string]string{
		"OutcomeClassifiedWeb":      "classified_web",
		"OutcomeClassifiedSocial":   "classified_social",
		"OutcomeClassifiedAcademic": "classified_academic",
		"OutcomeClassifiedKorean":   "classified_korean",
		"OutcomeClassifiedMixed":    "classified_mixed",
		"OutcomeClassifiedUnknown":  "classified_unknown",
		"OutcomeErrorInvalid":       "error_invalid",
		"OutcomeErrorTimeout":       "error_timeout",
		"OutcomeErrorBreakerOpen":   "error_breaker_open",
		"OutcomeErrorParse":         "error_parse",
	}
	for name, got := range want {
		if got != expected[name] {
			t.Errorf("%s: got %q, want %q", name, got, expected[name])
		}
	}
}
