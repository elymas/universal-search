// Package router — outcome label enumeration and helpers.
//
// This file declares the bounded set of outcome label values used by the
// Intent Router metrics (registered in internal/obs/metrics/router.go). It
// MUST NOT import prometheus/client_golang directly — keeping the metric
// declarations in internal/obs/metrics/ preserves the import-boundary
// invariant set by SPEC-OBS-001 REQ-OBS-006.
//
// SPEC-IR-001: REQ-IR-006.
package router

import (
	"context"
	"errors"
)

// Outcome label values. These are the ten exhaustive label values for the
// `outcome` label on usearch_router_classifications_total /
// usearch_router_classification_duration_seconds. No other value is emitted.
//
// @MX:NOTE: [AUTO] Static label-value enumeration. Bounded cardinality (10
// values). Adding a new value requires a SPEC amendment.
// @MX:SPEC: SPEC-IR-001
const (
	OutcomeClassifiedWeb      = "classified_web"
	OutcomeClassifiedSocial   = "classified_social"
	OutcomeClassifiedAcademic = "classified_academic"
	OutcomeClassifiedKorean   = "classified_korean"
	OutcomeClassifiedMixed    = "classified_mixed"
	OutcomeClassifiedUnknown  = "classified_unknown"
	OutcomeErrorInvalid       = "error_invalid"
	OutcomeErrorTimeout       = "error_timeout"
	OutcomeErrorBreakerOpen   = "error_breaker_open"
	OutcomeErrorParse         = "error_parse"
)

// OutcomeFromDecision returns the canonical outcome label for the given
// (decision, err) pair. Total over the 10-value enumeration. Mirrors the
// pkg/types.OutcomeFromError shape but uses router-specific outcomes.
//
// Mapping order (first match wins):
//  1. err == ErrInvalidQuery / context.Canceled       → error_invalid
//  2. err == ErrLLMTimeout / context.DeadlineExceeded → error_timeout
//  3. err == ErrLLMUnavailable                        → error_breaker_open
//  4. err == ErrLLMParse                              → error_parse
//  5. otherwise — Category dispatch                   → classified_<C>
func OutcomeFromDecision(dec RoutingDecision, err error) string {
	switch {
	case errors.Is(err, ErrInvalidQuery), errors.Is(err, context.Canceled):
		return OutcomeErrorInvalid
	case errors.Is(err, ErrLLMTimeout), errors.Is(err, context.DeadlineExceeded):
		return OutcomeErrorTimeout
	case errors.Is(err, ErrLLMUnavailable):
		return OutcomeErrorBreakerOpen
	case errors.Is(err, ErrLLMParse):
		return OutcomeErrorParse
	}
	switch dec.Category {
	case CategoryWeb:
		return OutcomeClassifiedWeb
	case CategorySocial:
		return OutcomeClassifiedSocial
	case CategoryAcademic:
		return OutcomeClassifiedAcademic
	case CategoryKorean:
		return OutcomeClassifiedKorean
	case CategoryMixed:
		return OutcomeClassifiedMixed
	default:
		return OutcomeClassifiedUnknown
	}
}
