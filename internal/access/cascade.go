// Package access — 5-phase cascade orchestrator.
//
// REQ-CACHE-001: Fetch is the sole public entry point.
// REQ-CACHE-007: Parent ctx propagation and per-phase context derivation.
// REQ-CACHE-011: Per-phase panic recovery.
// REQ-CACHE-016: Invalid URL rejection.
//
// @MX:ANCHOR: [AUTO] Fetch is the sole entry point for the 5-phase cascade.
// @MX:REASON: fan_in >= 4 (CLI, MCP, RetrieveOrchestrator, adapters). Signature
// change ripples to all consumers. REQ-CACHE-001.
// @MX:SPEC: SPEC-CACHE-001
package access

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"runtime/debug"
	"strings"
	"time"

	oteltrace "go.opentelemetry.io/otel/trace"
)

// Fetch executes the 5-phase content-fetch cascade for the given URL.
//
// It is the sole public entry point of the access package.
// Returns a non-nil *FetchResult on every call except when the URL is invalid
// (REQ-CACHE-016) or when the Fetcher is shutting down.
//
// On all-phases-failed: returns (*FetchResult{...}, ErrAllPhasesFailed).
// On success: returns (*FetchResult{Content: <content>}, nil).
// On invalid URL: returns (nil, *FetchError{CategoryPermanent}).
//
// @MX:ANCHOR: [AUTO] Sole public entry point; high fan_in.
// @MX:REASON: contract boundary; signature change ripples to all consumers.
// @MX:SPEC: SPEC-CACHE-001
func (f *Fetcher) Fetch(ctx context.Context, rawURL string, opts FetchOptions) (*FetchResult, error) {
	// REQ-CACHE-015: reject new calls when shutting down.
	select {
	case <-f.shutdownCh:
		return nil, ErrShuttingDown
	default:
	}

	// REQ-CACHE-016: reject empty, whitespace, or unparseable URLs.
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		f.logInvalidURL(ctx, rawURL)
		return nil, &FetchError{Category: CategoryPermanent, Reason: "invalid URL", Cause: ErrInvalidURL}
	}
	u, err := url.Parse(trimmed)
	if err != nil || u.Scheme == "" || u.Host == "" {
		f.logInvalidURL(ctx, rawURL)
		return nil, &FetchError{Category: CategoryPermanent, Reason: "invalid URL", Cause: ErrInvalidURL}
	}

	// REQ-CACHE-013: SSRF pre-flight checks.
	if err := validateScheme(u); err != nil {
		return nil, err
	}
	if err := validateHost(ctx, u, f.opts, opts); err != nil {
		return nil, err
	}

	// Start OTel parent span.
	tracer := f.obs.Tracer("access")
	spanCtx, span := tracer.Start(ctx, "access.fetch",
		oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
	defer span.End()

	start := time.Now()
	result := &FetchResult{}
	m := f.obs.AccessMetrics()

	// REQ-CACHE-007: bail immediately if parent ctx is already done.
	if err := ctx.Err(); err != nil {
		result.Outcome = "cancelled"
		result.ElapsedSeconds = elapsedSince(start)
		emitFetchTotal(m, result.Outcome)
		emitSlog(f.obs.SlogLogger(), spanCtx, result, u.Host)
		return result, nil
	}

	for phaseNum := 1; phaseNum <= 5; phaseNum++ {
		// Check parent ctx before each phase.
		if err := ctx.Err(); err != nil {
			result.Outcome = contextOutcome(err)
			break
		}

		phaseCtx, cancel := f.derivePhaseCtx(spanCtx, phaseNum)

		// Start per-phase OTel child span.
		_, phaseSpan := tracer.Start(phaseCtx, spanName(phaseNum))

		attempt := f.runPhase(phaseCtx, phaseNum, u, opts)
		phaseSpan.End()
		cancel()

		result.PhaseAttempts = append(result.PhaseAttempts, attempt)
		result.FinalPhase = phaseNum

		// Emit per-phase metrics.
		emitPhaseAttempt(m, &attempt)

		// Check success: only break if the phase produced content AND
		// escalation is not needed. Phase 2 "success" means robots-allowed
		// (no content) and MUST escalate to Phase 3 for the actual body.
		if attempt.Outcome == "success" && !f.shouldEscalatePhase(&attempt) {
			result.Outcome = "success"
			result.Content = attempt.content
			f.cacheWriteThrough(attempt.content)
			break
		}

		// Decide whether to try the next phase.
		if !f.shouldEscalatePhase(&attempt) {
			result.Outcome = attempt.Outcome
			if result.Outcome == "" {
				result.Outcome = "failure"
			}
			break
		}
	}

	// If we ran all 5 phases without success.
	if result.Outcome == "" {
		result.Outcome = "failure"
	}

	result.ElapsedSeconds = elapsedSince(start)
	emitFetchTotal(m, result.Outcome)
	emitSlog(f.obs.SlogLogger(), spanCtx, result, u.Host)

	if result.Content == nil && result.Outcome == "failure" {
		return result, ErrAllPhasesFailed
	}
	return result, nil
}

// shouldEscalatePhase wraps shouldEscalate with Phase 5 PlaywrightEnabled guard.
func (f *Fetcher) shouldEscalatePhase(prev *PhaseAttempt) bool {
	if prev.Phase == 4 && prev.isJSChallenge && !f.opts.PlaywrightEnabled {
		// Phase 5 is disabled; cannot escalate.
		return false
	}
	return shouldEscalate(prev)
}

// runPhase dispatches to the appropriate phase function and wraps any panic
// into a PhaseAttempt with Outcome="failure" (REQ-CACHE-011).
//
// @MX:WARN: [AUTO] Per-phase panic recovery: removing defer recover() invalidates
// REQ-CACHE-011 (cascade must survive Phase panics) and NFR-CACHE-005 (zero leaks).
// @MX:REASON: Playwright Phase 5 may segfault as a Go panic; the cascade must
// not crash the process.
// @MX:SPEC: SPEC-CACHE-001
func (f *Fetcher) runPhase(ctx context.Context, phaseNum int, u *url.URL, opts FetchOptions) (attempt PhaseAttempt) {
	rawURL := u.String()
	attempt.Phase = phaseNum
	attempt.StartedAt = time.Now()

	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			attempt.Outcome = "failure"
			attempt.ElapsedSeconds = elapsedSince(attempt.StartedAt)
			attempt.Error = fmt.Sprintf("phase %d panicked: %v", phaseNum, r)
			logger := f.obs.SlogLogger()
			if logger != nil {
				logger.LogAttrs(ctx, slog.LevelWarn, "access.runPhase: panic recovered",
					slog.Int("phase", phaseNum),
					slog.String("panic_value", fmt.Sprintf("%v", r)),
					slog.String("stack_trace", stack),
				)
			}
		}
	}()

	content, phaseErr := f.dispatchPhase(ctx, phaseNum, rawURL, u, opts)
	attempt.ElapsedSeconds = elapsedSince(attempt.StartedAt)

	if phaseErr == nil && content != nil {
		attempt.Outcome = "success"
		attempt.content = content
		return
	}

	// Classify the error into an outcome.
	switch phaseErr {
	case ErrPhaseNotApplicable:
		attempt.Outcome = "skipped"
	case ErrPhaseMiss:
		attempt.Outcome = "miss"
	default:
		if ferr, ok := phaseErr.(*FetchError); ok {
			switch ferr.Category {
			case CategoryBlocked:
				attempt.Outcome = "blocked"
			case CategoryTimeout:
				attempt.Outcome = "timeout"
			default:
				if phaseNum == 1 {
					attempt.Outcome = "miss"
				} else {
					attempt.Outcome = "failure"
				}
			}
		} else {
			// Propagate escalation signals from phase-specific errors.
			attempt.Outcome = "failure"
		}
	}

	// Copy escalation signals from FetchError into the attempt for shouldEscalate.
	if fe, ok := phaseErr.(*FetchError); ok && fe != nil {
		attempt.isTLSError = fe.isTLSSignal
		attempt.isWAF = fe.isWAFSignal
		attempt.isJSChallenge = fe.isJSChallengeSignal
	}

	if phaseErr != nil {
		attempt.Error = phaseErr.Error()
	}
	return
}

// dispatchPhase routes to the correct phase function and handles phase-specific
// escalation signal extraction.
func (f *Fetcher) dispatchPhase(
	ctx context.Context,
	phaseNum int,
	rawURL string,
	u *url.URL,
	opts FetchOptions,
) (*FetchedContent, error) {
	switch phaseNum {
	case 1:
		return phase1Index(ctx, f.opts.IndexLookup, rawURL)

	case 2:
		content, err := phase2Probe(ctx, rawURL, opts, f.opts, f.robotsCache)
		if err == ErrPhaseNotApplicable {
			return nil, ErrPhaseNotApplicable
		}
		return content, err

	case 3:
		content, attempt, err := phase3Get(ctx, rawURL, opts, f.opts)
		if attempt != nil {
			// Copy escalation signals back to the caller's runPhase via a
			// temporary mechanism: store in the FetchError itself is not
			// ideal, so we embed the signals in the returned attempt and
			// merge them in runPhase. For simplicity, we use the content
			// nil + err path and embed the signals in phaseAttempt which
			// runPhase already reads from its local variable.
			//
			// NOTE: The runPhase wrapper reads attempt.isTLSError and
			// attempt.isWAF from the PhaseAttempt returned by runPhase itself,
			// not from dispatchPhase. We need to propagate them differently.
			//
			// Solution: return a special error type that carries the signals.
			if attempt.isTLSError || attempt.isWAF {
				fe, ok := err.(*FetchError)
				if ok {
					fe.isTLSSignal = attempt.isTLSError
					fe.isWAFSignal = attempt.isWAF
				}
			}
		}
		return content, err

	case 4:
		content, attempt, err := phase4TLS(ctx, rawURL, opts, f.opts)
		if attempt != nil && attempt.isJSChallenge {
			fe, ok := err.(*FetchError)
			if ok {
				fe.isJSChallengeSignal = true
			}
		}
		return content, err

	case 5:
		return f.phase5Browser(ctx, rawURL, opts)

	default:
		return nil, ErrPhaseNotApplicable
	}
}

// derivePhaseCtx creates a child context whose deadline is the minimum of the
// per-phase budget and the remaining parent deadline (§2.4).
func (f *Fetcher) derivePhaseCtx(parent context.Context, phase int) (context.Context, context.CancelFunc) {
	budget := f.opts.PerPhaseTimeout[phase]
	if budget == 0 {
		budget = defaultPerPhaseTimeout[phase]
	}

	if pDeadline, ok := parent.Deadline(); ok {
		if remaining := time.Until(pDeadline); remaining > 0 && remaining < budget {
			budget = remaining
		} else if remaining <= 0 {
			// Parent deadline already expired.
			ctx, cancel := context.WithCancel(parent)
			cancel()
			return ctx, cancel
		}
	}

	if budget <= 0 {
		ctx, cancel := context.WithCancel(parent)
		cancel()
		return ctx, cancel
	}

	return context.WithTimeout(parent, budget)
}

// logInvalidURL emits a WARN slog record for an invalid URL (REQ-CACHE-016).
func (f *Fetcher) logInvalidURL(ctx context.Context, rawURL string) {
	logger := f.obs.SlogLogger()
	if logger == nil {
		return
	}
	logger.LogAttrs(ctx, slog.LevelWarn, "access.fetch: invalid URL rejected",
		slog.String("url", rawURL),
	)
}

// contextOutcome maps a context error to the FetchResult.Outcome string.
func contextOutcome(err error) string {
	if err == context.DeadlineExceeded {
		return "timeout"
	}
	return "cancelled"
}
