// Package metrics — Synthesis metric collectors.
// Owned by SPEC-SYN-001 (calls/latency/cost); extended by SPEC-SYN-002
// (faithfulness outcomes + retries). Lives under internal/obs/metrics/ to
// preserve the import-boundary invariant set by SPEC-OBS-001.
//
// Declares five metric families:
//
//	usearch_synthesis_calls_total{outcome}                          Counter
//	usearch_synthesis_latency_seconds{outcome}                      Histogram
//	usearch_synthesis_cost_usd_total                                Counter (no labels)
//	usearch_synthesis_faithfulness_outcomes_total{outcome}          Counter  (SPEC-SYN-002)
//	usearch_synthesis_faithfulness_retries_total                    Counter  (SPEC-SYN-002)
package metrics

import (
	"github.com/elymas/universal-search/internal/synthesis/citation"
	"github.com/prometheus/client_golang/prometheus"
)

// synthesisCallBuckets covers the expected synthesis latency range:
// degraded fast-path < 100ms, normal LLM calls 1-10s.
var synthesisCallBuckets = []float64{0.05, 0.1, 0.5, 1, 2.5, 5, 8, 10, 15, 30}

// synthesisCollectors bundles the five synthesis metric vectors.
// Stored as Registry.SynthesisCalls / SynthesisLatency / SynthesisCost /
// SynthesisFaithfulnessOutcomes / SynthesisFaithfulnessRetries fields.
type synthesisCollectors struct {
	calls                *prometheus.CounterVec
	latency              *prometheus.HistogramVec
	cost                 prometheus.Counter
	faithfulnessOutcomes *prometheus.CounterVec
	faithfulnessRetries  prometheus.Counter
}

// registerSynthesis creates the five synthesis metric collectors, registers
// them on r, and returns them for storage on the owning Registry.
// Mirrors the registerLLM pattern at internal/obs/metrics/llm.go.
// Extended by SPEC-SYN-002 §2.1(g) to add faithfulness collectors.
func registerSynthesis(r *prometheus.Registry) synthesisCollectors {
	calls := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "usearch_synthesis_calls_total",
			Help: "Total synthesis calls, partitioned by outcome.",
		},
		[]string{"outcome"},
	)
	latency := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "usearch_synthesis_latency_seconds",
			Help:    "Synthesis call latency distribution.",
			Buckets: synthesisCallBuckets,
		},
		[]string{"outcome"},
	)
	cost := prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "usearch_synthesis_cost_usd_total",
			Help: "Cumulative synthesis LLM cost in USD (no labels; degraded calls excluded).",
		},
	)

	r.MustRegister(calls, latency, cost)

	// Pre-initialise with placeholder values so families appear in /metrics
	// output even before any synthesis calls are made (REQ-OBS-004).
	calls.WithLabelValues("success").Add(0)
	latency.WithLabelValues("success").Observe(0)

	// SPEC-SYN-002 §2.1(f,g): register faithfulness collectors via the
	// citation package to share the exact metric names and pre-init logic.
	// @MX:NOTE: [AUTO] citation.RegisterFaithfulnessMetrics owns the metric names;
	// do not duplicate them here — single source of truth in internal/synthesis/citation.
	fm, err := citation.RegisterFaithfulnessMetrics(r)
	if err != nil {
		// This is a programming error (duplicate registration); panic is appropriate
		// here to match the MustRegister pattern used for the existing collectors above.
		panic("metrics: registerSynthesis faithfulness: " + err.Error())
	}

	return synthesisCollectors{
		calls:                calls,
		latency:              latency,
		cost:                 cost,
		faithfulnessOutcomes: fm.Outcomes,
		faithfulnessRetries:  fm.Retries,
	}
}
