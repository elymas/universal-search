// Package obs_test — SPEC-SYN-002 faithfulness re-export tests.
//
// Verifies that Obs re-exports the two new faithfulness collectors
// declared by SPEC-SYN-002 §2.1(h).
package obs_test

import (
	"context"
	"testing"

	"github.com/elymas/universal-search/internal/obs"
)

// TestObsSynthesisFaithfulnessOutcomesReexport verifies that Obs.SynthesisFaithfulnessOutcomes()
// returns a non-nil counter after Init. SPEC-SYN-002 §2.1(h).
func TestObsSynthesisFaithfulnessOutcomesReexport(t *testing.T) {
	t.Parallel()

	cfg := obs.Config{ServiceName: "test", ServiceVersion: "0.0.0"}
	o, shutdown, err := obs.Init(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer func() { _ = shutdown(context.Background()) }()

	if o.SynthesisFaithfulnessOutcomes() == nil {
		t.Fatal("Obs.SynthesisFaithfulnessOutcomes() returned nil (SPEC-SYN-002 §2.1(h))")
	}
}

// TestObsSynthesisFaithfulnessRetriesReexport verifies that Obs.SynthesisFaithfulnessRetries()
// returns a non-nil counter after Init. SPEC-SYN-002 §2.1(h).
func TestObsSynthesisFaithfulnessRetriesReexport(t *testing.T) {
	t.Parallel()

	cfg := obs.Config{ServiceName: "test", ServiceVersion: "0.0.0"}
	o, shutdown, err := obs.Init(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer func() { _ = shutdown(context.Background()) }()

	if o.SynthesisFaithfulnessRetries() == nil {
		t.Fatal("Obs.SynthesisFaithfulnessRetries() returned nil (SPEC-SYN-002 §2.1(h))")
	}
}

// TestObsSynthesisFaithfulnessOutcomesNilSafe verifies that calling
// Obs.SynthesisFaithfulnessOutcomes() on a nil Obs does not panic.
// SPEC-SYN-002 §2.1(h).
func TestObsSynthesisFaithfulnessOutcomesNilSafe(t *testing.T) {
	t.Parallel()

	var o *obs.Obs
	result := o.SynthesisFaithfulnessOutcomes()
	if result != nil {
		t.Errorf("nil Obs.SynthesisFaithfulnessOutcomes() = %v, want nil", result)
	}
}

// TestObsSynthesisFaithfulnessRetriesNilSafe verifies that calling
// Obs.SynthesisFaithfulnessRetries() on a nil Obs does not panic.
// SPEC-SYN-002 §2.1(h).
func TestObsSynthesisFaithfulnessRetriesNilSafe(t *testing.T) {
	t.Parallel()

	var o *obs.Obs
	result := o.SynthesisFaithfulnessRetries()
	if result != nil {
		t.Errorf("nil Obs.SynthesisFaithfulnessRetries() = %v, want nil", result)
	}
}
