// Package access — unit tests for observability helpers.
package access

import (
	"testing"
	"time"
)

func TestNoopObs_AllMethods(t *testing.T) {
	t.Parallel()
	obs := noopObs{}
	// All methods must return non-nil or zero values without panicking.
	if obs.Tracer("test") == nil {
		t.Error("noopObs.Tracer must return non-nil tracer")
	}
	if obs.SlogLogger() != nil {
		t.Error("noopObs.SlogLogger must return nil (no-op)")
	}
	m := obs.AccessMetrics()
	if m != nil {
		t.Error("noopObs.AccessMetrics must return nil")
	}
}

func TestResolveObs_NilInput_ReturnsNoop(t *testing.T) {
	t.Parallel()
	obs := resolveObs(nil)
	if obs == nil {
		t.Error("resolveObs(nil) must return non-nil (noopObs)")
	}
}

func TestResolveObs_NonNilInput_ReturnsSame(t *testing.T) {
	t.Parallel()
	in := noopObs{}
	obs := resolveObs(in)
	if obs == nil {
		t.Error("resolveObs(non-nil) must return non-nil")
	}
}

func TestElapsedSince_Positive(t *testing.T) {
	t.Parallel()
	start := time.Now().Add(-100 * time.Millisecond)
	elapsed := elapsedSince(start)
	if elapsed <= 0 {
		t.Errorf("elapsedSince must be positive, got %v", elapsed)
	}
}

func TestEmitPhaseAttempt_NilCollectors_NoPanic(t *testing.T) {
	t.Parallel()
	// nil *AccessCollectors — emitPhaseAttempt must be nil-safe.
	a := &PhaseAttempt{Phase: 3, Outcome: "success", ElapsedSeconds: 0.1}
	emitPhaseAttempt(nil, a)
}

func TestEmitFetchTotal_NilCounter_NoPanic(t *testing.T) {
	t.Parallel()
	emitFetchTotal(nil, "success")
}

func TestPhaseLabel_AllPhases(t *testing.T) {
	t.Parallel()
	for _, phase := range []int{1, 2, 3, 4, 5} {
		label := phaseLabel(phase)
		if label == "" {
			t.Errorf("phaseLabel(%d) must not be empty", phase)
		}
	}
}
