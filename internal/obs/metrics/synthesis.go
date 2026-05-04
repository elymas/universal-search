// Package metrics — Synthesis metric collectors.
// Owned by SPEC-SYN-001; lives under internal/obs/metrics/ to preserve the
// import-boundary invariant set by SPEC-OBS-001.
//
// Declares three metric families:
//
//	usearch_synthesis_calls_total{outcome}         Counter
//	usearch_synthesis_latency_seconds{outcome}     Histogram
//	usearch_synthesis_cost_usd_total               Counter (no labels)
package metrics

import "github.com/prometheus/client_golang/prometheus"

// synthesisCallBuckets covers the expected synthesis latency range:
// degraded fast-path < 100ms, normal LLM calls 1-10s.
var synthesisCallBuckets = []float64{0.05, 0.1, 0.5, 1, 2.5, 5, 8, 10, 15, 30}

// synthesisCollectors bundles the three synthesis metric vectors.
// Stored as Registry.SynthesisCalls / SynthesisLatency / SynthesisCost fields.
type synthesisCollectors struct {
	calls   *prometheus.CounterVec
	latency *prometheus.HistogramVec
	cost    prometheus.Counter
}

// registerSynthesis creates the three synthesis metric collectors, registers
// them on r, and returns them for storage on the owning Registry.
// Mirrors the registerLLM pattern at internal/obs/metrics/llm.go.
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

	return synthesisCollectors{calls: calls, latency: latency, cost: cost}
}
