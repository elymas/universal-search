// Package metrics — Embedder metric collectors.
// Owned by SPEC-IDX-002; lives under internal/obs/metrics/ to preserve the
// import-boundary invariant set by SPEC-OBS-001.
//
// Declares three metric families:
//
//	usearch_embedder_calls_total{outcome,mode}       Counter
//	usearch_embedder_latency_seconds{outcome,mode}   Histogram
//	usearch_embedder_cache_hits_total                Counter (no labels)
package metrics

import "github.com/prometheus/client_golang/prometheus"

// embedderCallBuckets covers the expected embedding latency range:
// cache hits < 5ms, CPU inference 100ms-2s.
var embedderCallBuckets = []float64{0.005, 0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5}

// embedderCollectors bundles the three embedder metric vectors.
type embedderCollectors struct {
	calls     *prometheus.CounterVec
	latency   *prometheus.HistogramVec
	cacheHits prometheus.Counter
}

// registerEmbedder creates the three embedder metric collectors, registers
// them on r, and returns them for storage on the owning Registry.
// Mirrors the registerSynthesis pattern at internal/obs/metrics/synthesis.go.
func registerEmbedder(r *prometheus.Registry) embedderCollectors {
	calls := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "usearch_embedder_calls_total",
			Help: "Total embedder calls, partitioned by outcome and mode.",
		},
		[]string{"outcome", "mode"},
	)
	latency := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "usearch_embedder_latency_seconds",
			Help:    "Embedder call latency distribution.",
			Buckets: embedderCallBuckets,
		},
		[]string{"outcome", "mode"},
	)
	cacheHits := prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "usearch_embedder_cache_hits_total",
			Help: "Cumulative cache hits for embedder requests (no labels).",
		},
	)

	r.MustRegister(calls, latency, cacheHits)

	// Pre-initialise with placeholder values so families appear in /metrics
	// output even before any embedder calls are made (REQ-OBS-004).
	calls.WithLabelValues("success", "dense").Add(0)
	latency.WithLabelValues("success", "dense").Observe(0)

	return embedderCollectors{calls: calls, latency: latency, cacheHits: cacheHits}
}
