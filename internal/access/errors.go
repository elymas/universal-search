// Package access — sentinel errors for the 5-phase content-fetch cascade.
//
// REQ-CACHE-001: ErrAllPhasesFailed, ErrPlaywrightUnavailable, ErrShuttingDown
// REQ-CACHE-016: ErrInvalidURL
package access

import "errors"

// ErrAllPhasesFailed is returned by Fetch when all five phases fail or are
// all skipped. The accompanying *FetchResult has PhaseAttempts populated.
var ErrAllPhasesFailed = errors.New("access: all 5 phases failed")

// ErrPhaseNotApplicable is an internal sentinel used when a phase is skipped.
// It is never surfaced to callers.
var ErrPhaseNotApplicable = errors.New("access: phase not applicable")

// ErrPhaseMiss is returned by Phase 1 when the index exists but the URL is
// not found in it (cache miss). Distinct from ErrPhaseNotApplicable (no index).
var ErrPhaseMiss = errors.New("access: phase cache miss")

// ErrPlaywrightUnavailable is returned by New when PlaywrightEnabled == true,
// playwright.Install fails, and AutoInstallPlaywright == false.
var ErrPlaywrightUnavailable = errors.New("access: playwright not installed")

// ErrShuttingDown is returned by Fetch when called after Shutdown.
var ErrShuttingDown = errors.New("access: fetcher is shutting down")

// ErrInvalidURL is returned by Fetch when the URL is empty, whitespace-only,
// or fails url.Parse. Wraps CategoryPermanent.
var ErrInvalidURL = errors.New("access: invalid URL")

// ErrorCategory classifies a FetchError so that the cascade can decide
// whether to escalate to the next phase.
type ErrorCategory string

const (
	// CategoryBlocked means the URL was blocked (SSRF guard or robots.txt).
	// The cascade does NOT escalate from a blocked error.
	CategoryBlocked ErrorCategory = "blocked"

	// CategoryPermanent means the server returned a 4xx that indicates the
	// resource does not exist (e.g., 404, 410). No escalation.
	CategoryPermanent ErrorCategory = "permanent"

	// CategoryRateLimited means the server returned HTTP 429.
	// No escalation — rate limiting is the caller's responsibility.
	CategoryRateLimited ErrorCategory = "rate_limited"

	// CategoryUnavailable means a transient error (5xx, network, TLS).
	// The cascade MAY escalate to the next phase depending on the predicates
	// in shouldEscalate.
	CategoryUnavailable ErrorCategory = "unavailable"

	// CategoryTimeout means the phase context deadline expired.
	// The cascade does NOT escalate after a timeout.
	CategoryTimeout ErrorCategory = "timeout"
)

// FetchError carries structured error information from a single phase attempt.
// It implements the error interface so it can be returned directly.
type FetchError struct {
	Category   ErrorCategory
	Reason     string
	HTTPStatus int
	Cause      error

	// Internal escalation signals — set by dispatchPhase when the error
	// carries phase-level metadata that shouldEscalate needs.
	isTLSSignal         bool
	isJSChallengeSignal bool
	// profileHits carries the WAF detection result (SPEC-ACC-001); nil
	// when no WAF was detected.
	profileHits []ProfileHit
	// verdict carries the 4-layer page-validity classification
	// (SPEC-ACC-001); empty string when validatePage was not run.
	verdict Verdict
}

func (e *FetchError) Error() string {
	if e.Cause != nil {
		return string(e.Category) + ": " + e.Reason + ": " + e.Cause.Error()
	}
	return string(e.Category) + ": " + e.Reason
}

func (e *FetchError) Unwrap() error { return e.Cause }
