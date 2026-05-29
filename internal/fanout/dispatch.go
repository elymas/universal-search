// Package fanout — Dispatch hot path: errgroup orchestration, per-adapter ctx
// derivation, partial-result collection, panic recovery.
// SPEC-FAN-001 §2.5, §2.6, §2.7, REQ-FAN-002..013.
package fanout

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/elymas/universal-search/internal/obs"
	"github.com/elymas/universal-search/internal/router"
	"github.com/elymas/universal-search/pkg/types"
)

// deriveAdapterCtx builds a per-adapter context per SPEC-FAN-001 §2.5.
//
// adapterDeadline = min(perAdapterTimeout, timeUntil(parentCtx.Deadline()))
// If the parent has no deadline, perAdapterTimeout is used as-is.
// If the remaining time is <= 0 (parent already past deadline), the returned
// context is immediately cancelled.
//
// @MX:NOTE: [AUTO] §2.5 per-adapter ctx derivation: min(perAdapterTimeout, remainingParentDeadline).
// The parent ctx propagation is preserved — cancelling the parent cancels all
// per-adapter ctxs via Go's context inheritance graph.
// @MX:SPEC: SPEC-FAN-001
func deriveAdapterCtx(parent context.Context, perAdapterTimeout time.Duration) (context.Context, context.CancelFunc) {
	deadline := perAdapterTimeout
	if pDeadline, ok := parent.Deadline(); ok {
		if remaining := time.Until(pDeadline); remaining < deadline {
			deadline = remaining
		}
	}
	if deadline <= 0 {
		// Parent is already past its deadline; return an immediately-cancelled ctx.
		ctx, cancel := context.WithCancel(parent)
		cancel()
		return ctx, cancel
	}
	return context.WithTimeout(parent, deadline)
}

// assembleResult builds a *Result from the per-index slices post-eg.Wait().
// The supervisor (Dispatch goroutine) is the SOLE writer of Result.AdapterErrors.
// No worker ever writes to a map directly (SPEC-FAN-001 §2.6 H1 fix).
func assembleResult(adapterSet []string, perDocs [][]types.NormalizedDoc, perErr []error) *Result {
	n := len(adapterSet)
	res := &Result{
		Stats: Stats{AdapterCount: n},
	}

	var allDocs []types.NormalizedDoc
	var errMap map[string]error

	for i, name := range adapterSet {
		if perErr[i] != nil {
			res.Stats.ErrorCount++
			if errMap == nil {
				errMap = make(map[string]error, n)
			}
			errMap[name] = perErr[i]
		} else {
			res.Stats.SuccessCount++
			allDocs = append(allDocs, perDocs[i]...)
		}
	}

	res.Docs = allDocs
	if res.Docs == nil {
		res.Docs = []types.NormalizedDoc{}
	}
	// §2.6 H17 fix: AdapterErrors is nil when ErrorCount == 0.
	res.AdapterErrors = errMap
	return res
}

// dispatch is the inner Dispatch implementation extracted for testability.
//
// @MX:WARN: [AUTO] Dispatch spawns up to MaxParallel goroutines via errgroup.
// @MX:REASON: removing the per-goroutine defer recover()/defer FanoutInflight.Dec()
// invalidates NFR-FAN-003 zero-leak guarantee; the errgroup suppress-error idiom
// (workers return nil) is load-bearing to prevent first-error cancellation killing siblings.
// @MX:SPEC: SPEC-FAN-001
func dispatch(
	ctx context.Context,
	o *obs.Obs,
	registry interface {
		Get(name string) (types.Adapter, bool)
	},
	maxParallel int,
	perAdapterTimeout time.Duration,
	decision router.RoutingDecision,
	q types.Query,
) (*Result, error) {
	// §2.6: pre-allocated per-index slices. Workers write ONLY to their own index i.
	n := len(decision.AdapterSet)
	perAdapterDocs := make([][]types.NormalizedDoc, n)
	perAdapterErr := make([]error, n)

	classLabel := string(decision.Category)

	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(maxParallel)

	for i, name := range decision.AdapterSet {
		i, name := i, name // capture loop vars

		// §2.5 H18 + REQ-FAN-012/013: pre-launch ctx guard.
		// Check BEFORE every eg.Go call to prevent SetLimit deadlock
		// when the queue is full and ctx is already cancelled.
		if err := ctx.Err(); err != nil {
			perAdapterErr[i] = &types.SourceError{
				Adapter:  name,
				Category: types.CategoryUnavailable,
				Cause:    err,
			}
			continue
		}

		eg.Go(func() error {
			incInflight(o, classLabel)
			defer decInflight(o, classLabel)

			// D7: per-goroutine panic recovery converts panics into *SourceError{CategoryUnknown}.
			defer func() {
				if r := recover(); r != nil {
					perAdapterErr[i] = &types.SourceError{
						Adapter:  name,
						Category: types.CategoryUnknown,
						Cause:    fmt.Errorf("adapter %q panicked: %v", name, r),
					}
					emitPanic(egCtx, o, name, r, debug.Stack())
				}
			}()

			ad, ok := registry.Get(name)
			if !ok {
				perAdapterErr[i] = fmt.Errorf("%w: %s", ErrAdapterNotFound, name)
				return nil // suppress error to prevent first-error cancellation
			}

			adapterCtx, cancel := deriveAdapterCtx(egCtx, perAdapterTimeout)
			defer cancel()

			docs, err := ad.Search(adapterCtx, q)
			if err != nil {
				perAdapterErr[i] = err
				return nil // suppress: D1 suppress-error idiom
			}
			perAdapterDocs[i] = docs
			return nil
		})
	}

	// Wait for all in-flight workers. Errors are collected per-index, not via eg.
	_ = eg.Wait()

	// Supervisor builds Result.AdapterErrors from per-index slices.
	// No worker ever touched a map directly (§2.6 H1 fix).
	result := assembleResult(decision.AdapterSet, perAdapterDocs, perAdapterErr)

	// SPEC-EVAL-002 REQ-EVAL2-004: increment partial-result counter once per
	// failed adapter after all workers complete and before returning to caller.
	// @MX:NOTE: [AUTO] SPEC-EVAL-002 REQ-EVAL2-004 fanout partial counter emission
	emitPartialResultCounters(o, result)

	return result, nil
}
