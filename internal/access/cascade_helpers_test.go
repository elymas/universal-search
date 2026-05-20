// Package access — unit tests for cascade helpers (contextOutcome, derivePhaseCtx, spanName).
package access

import (
	"context"
	"testing"
	"time"
)

func TestContextOutcome_DeadlineExceeded(t *testing.T) {
	t.Parallel()
	if got := contextOutcome(context.DeadlineExceeded); got != "timeout" {
		t.Errorf("contextOutcome(DeadlineExceeded) = %q, want timeout", got)
	}
}

func TestContextOutcome_Cancelled(t *testing.T) {
	t.Parallel()
	if got := contextOutcome(context.Canceled); got != "cancelled" {
		t.Errorf("contextOutcome(Canceled) = %q, want cancelled", got)
	}
}

func TestDerivePhaseCtx_RespectsParentDeadline(t *testing.T) {
	t.Parallel()
	f := &Fetcher{opts: Options{}}
	f.opts.applyDefaults()

	// Parent deadline of 50ms — phase budget is 10s for Phase 3.
	// The derived ctx should cap at the parent's remaining time.
	parent, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	ctx, cancelPhase := f.derivePhaseCtx(parent, 3)
	defer cancelPhase()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Error("derived ctx must have a deadline")
	}
	if time.Until(deadline) > 60*time.Millisecond {
		t.Errorf("deadline %v should be capped by parent deadline", time.Until(deadline))
	}
}

func TestDerivePhaseCtx_UsesPhaseDefault_WhenNoParentDeadline(t *testing.T) {
	t.Parallel()
	f := &Fetcher{opts: Options{}}
	f.opts.applyDefaults()

	ctx, cancel := f.derivePhaseCtx(t.Context(), 3) // Phase 3 default = 10s
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Error("derived ctx must have a deadline from phase budget")
	}
	// Should be roughly 10s from now.
	remaining := time.Until(deadline)
	if remaining < 9*time.Second || remaining > 11*time.Second {
		t.Errorf("Phase 3 deadline remaining = %v, want ~10s", remaining)
	}
}

func TestDerivePhaseCtx_ExpiredParent_ReturnsCancelled(t *testing.T) {
	t.Parallel()
	f := &Fetcher{opts: Options{}}
	f.opts.applyDefaults()

	// Create an already-expired parent.
	parent, cancel := context.WithTimeout(t.Context(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(5 * time.Millisecond) // ensure expired

	ctx, cancelPhase := f.derivePhaseCtx(parent, 3)
	defer cancelPhase()

	if ctx.Err() == nil {
		t.Error("derived ctx from expired parent should already be cancelled")
	}
}

func TestSpanName_AllPhases(t *testing.T) {
	t.Parallel()
	cases := []struct {
		phase int
		want  string
	}{
		{1, "access.phase1"},
		{2, "access.phase2"},
		{3, "access.phase3"},
		{4, "access.phase4"},
		{5, "access.phase5"},
	}
	for _, tc := range cases {
		got := spanName(tc.phase)
		if got != tc.want {
			t.Errorf("spanName(%d) = %q, want %q", tc.phase, got, tc.want)
		}
	}
}
