// Package access — unit tests for NewAccessCollectors and metric registration.
package access

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestNewAccessCollectors_Registers(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	c := NewAccessCollectors(reg)
	if c == nil {
		t.Fatal("NewAccessCollectors must return non-nil")
	}
	if c.AccessPhaseAttempts == nil {
		t.Error("AccessPhaseAttempts must be non-nil")
	}
	if c.AccessPhaseDuration == nil {
		t.Error("AccessPhaseDuration must be non-nil")
	}
	if c.AccessFetchTotal == nil {
		t.Error("AccessFetchTotal must be non-nil")
	}
}

func TestNewAccessCollectors_MetricsGatherable(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	c := NewAccessCollectors(reg)

	// Record a sample metric to ensure it's gathereable.
	emitPhaseAttempt(c, &PhaseAttempt{Phase: 3, Outcome: "success", ElapsedSeconds: 0.5})
	emitFetchTotal(c, "success")

	mf, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather() error: %v", err)
	}
	if len(mf) == 0 {
		t.Error("Gather() returned no metric families after recording")
	}
}

func TestEmitSlog_NilLogger_NoPanic(t *testing.T) {
	t.Parallel()
	result := &FetchResult{
		Outcome:        "success",
		ElapsedSeconds: 0.1,
		FinalPhase:     3,
	}
	// emitSlog with nil logger must not panic.
	emitSlog(nil, t.Context(), result, "example.com")
}
