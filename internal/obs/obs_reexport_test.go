// Package obs_test — coverage for the metric re-export accessors and tracer
// helpers on Obs. Verifies two behavioral contracts for every accessor:
//  1. After Init, the accessor returns the live collector (non-nil).
//  2. On a zero-value / nil-Metrics Obs, the accessor returns nil (no panic) —
//     this is the "safe for tests" contract documented on each method.
//
// @MX:SPEC: SPEC-REL-001 — G1 release coverage gate
package obs_test

import (
	"context"
	"testing"

	"github.com/elymas/universal-search/internal/obs"
)

func newInitedObs(t *testing.T) *obs.Obs {
	t.Helper()
	o, shutdown, err := obs.Init(context.Background(), obs.Config{
		ServiceName:    "test",
		ServiceVersion: "0.0.0",
	})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() { _ = shutdown(context.Background()) })
	return o
}

// TestObsReexports_NonNilAfterInit asserts every re-export accessor returns a
// live collector after Init. A nil here means the collector was never wired
// into the registry — a real regression.
func TestObsReexports_NonNilAfterInit(t *testing.T) {
	o := newInitedObs(t)

	checks := map[string]func() bool{
		"SynthesisCalls":                func() bool { return o.SynthesisCalls() != nil },
		"SynthesisLatency":              func() bool { return o.SynthesisLatency() != nil },
		"SynthesisCost":                 func() bool { return o.SynthesisCost() != nil },
		"TokenizerCalls":                func() bool { return o.TokenizerCalls() != nil },
		"TokenizerLatency":              func() bool { return o.TokenizerLatency() != nil },
		"IndexShardWrites":              func() bool { return o.IndexShardWrites() != nil },
		"SynthesisFaithfulnessOutcomes": func() bool { return o.SynthesisFaithfulnessOutcomes() != nil },
		"SynthesisFaithfulnessRetries":  func() bool { return o.SynthesisFaithfulnessRetries() != nil },
		"StreamSynthOutcomes":           func() bool { return o.StreamSynthOutcomes() != nil },
		"StreamSynthSentencesEmitted":   func() bool { return o.StreamSynthSentencesEmitted() != nil },
		"SynthClusterOutcomes":          func() bool { return o.SynthClusterOutcomes() != nil },
		"SynthClusterMembers":           func() bool { return o.SynthClusterMembers() != nil },
		"DeepReportOutcomes":            func() bool { return o.DeepReportOutcomes() != nil },
		"DeepReportLatency":             func() bool { return o.DeepReportLatency() != nil },
		"DeepAgentDuration":             func() bool { return o.DeepAgentDuration() != nil },
		"DeepAgentRetries":              func() bool { return o.DeepAgentRetries() != nil },
		"DeepAgentVerifierGateResults":  func() bool { return o.DeepAgentVerifierGateResults() != nil },
		"DeepTreeNodeExpand":            func() bool { return o.DeepTreeNodeExpand() != nil },
		"DeepTreeTotalTokens":           func() bool { return o.DeepTreeTotalTokens() != nil },
	}

	for name, ok := range checks {
		if !ok() {
			t.Errorf("%s() returned nil after Init; collector not wired", name)
		}
	}
}

// TestObsReexports_NilSafeOnZeroValue asserts every accessor returns nil
// (without panicking) when Metrics is unset. This is the documented safe path
// used by tests that only wire a subset of the Obs bundle.
func TestObsReexports_NilSafeOnZeroValue(t *testing.T) {
	o := &obs.Obs{} // Metrics == nil

	checks := map[string]func() bool{
		"SynthesisCalls":                func() bool { return o.SynthesisCalls() == nil },
		"SynthesisLatency":              func() bool { return o.SynthesisLatency() == nil },
		"SynthesisCost":                 func() bool { return o.SynthesisCost() == nil },
		"TokenizerCalls":                func() bool { return o.TokenizerCalls() == nil },
		"TokenizerLatency":              func() bool { return o.TokenizerLatency() == nil },
		"IndexShardWrites":              func() bool { return o.IndexShardWrites() == nil },
		"SynthesisFaithfulnessOutcomes": func() bool { return o.SynthesisFaithfulnessOutcomes() == nil },
		"SynthesisFaithfulnessRetries":  func() bool { return o.SynthesisFaithfulnessRetries() == nil },
		"StreamSynthOutcomes":           func() bool { return o.StreamSynthOutcomes() == nil },
		"StreamSynthSentencesEmitted":   func() bool { return o.StreamSynthSentencesEmitted() == nil },
		"SynthClusterOutcomes":          func() bool { return o.SynthClusterOutcomes() == nil },
		"SynthClusterMembers":           func() bool { return o.SynthClusterMembers() == nil },
		"DeepReportOutcomes":            func() bool { return o.DeepReportOutcomes() == nil },
		"DeepReportLatency":             func() bool { return o.DeepReportLatency() == nil },
		"DeepAgentDuration":             func() bool { return o.DeepAgentDuration() == nil },
		"DeepAgentRetries":              func() bool { return o.DeepAgentRetries() == nil },
		"DeepAgentVerifierGateResults":  func() bool { return o.DeepAgentVerifierGateResults() == nil },
		"DeepTreeNodeExpand":            func() bool { return o.DeepTreeNodeExpand() == nil },
		"DeepTreeTotalTokens":           func() bool { return o.DeepTreeTotalTokens() == nil },
	}

	for name, ok := range checks {
		if !ok() {
			t.Errorf("%s() should return nil on zero-value Obs", name)
		}
	}
}

// TestObs_HasTracer covers the tracer-presence predicate across the three
// states: nil receiver, partially-wired (no tracer), and fully Init-ed.
func TestObs_HasTracer(t *testing.T) {
	var nilObs *obs.Obs
	if nilObs.HasTracer() {
		t.Error("nil Obs should report HasTracer() == false")
	}

	partial := &obs.Obs{} // no tracerProvider wired
	if partial.HasTracer() {
		t.Error("zero-value Obs should report HasTracer() == false")
	}

	inited := newInitedObs(t)
	if !inited.HasTracer() {
		t.Error("Init-ed Obs should report HasTracer() == true")
	}
}

// TestObs_Tracer covers both the fallback (global no-op) path on a partially
// wired Obs and the wired-provider path after Init.
func TestObs_Tracer(t *testing.T) {
	partial := &obs.Obs{}
	if partial.Tracer("fallback") == nil {
		t.Error("Tracer() should fall back to a non-nil global tracer when provider is nil")
	}

	inited := newInitedObs(t)
	if inited.Tracer("wired") == nil {
		t.Error("Tracer() should return a non-nil tracer from the wired provider")
	}
}
