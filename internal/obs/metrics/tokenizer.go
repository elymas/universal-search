// Package metrics — Tokenizer sidecar metric collectors.
// Owned by SPEC-IDX-003; lives under internal/obs/metrics/ to preserve the
// import-boundary invariant set by SPEC-OBS-001.
//
// Declares:
//
//	usearch_tokenizer_calls_total{outcome}           Counter
//	usearch_tokenizer_latency_seconds{outcome}       Histogram
//	usearch_index_shard_writes_total{shard}          Counter
package metrics

import "github.com/prometheus/client_golang/prometheus"

// tokenizerCallBuckets covers the expected sidecar round-trip range:
// p50 target <5ms, tail up to 100ms.
var tokenizerCallBuckets = []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5}

// tokenizerCollectors bundles the three tokenizer metric collectors.
// Stored as Registry.TokenizerCalls / TokenizerLatency / IndexShardWrites fields.
type tokenizerCollectors struct {
	calls       *prometheus.CounterVec
	latency     *prometheus.HistogramVec
	shardWrites *prometheus.CounterVec
}

// registerTokenizer creates the three tokenizer metric collectors, registers
// them on r, and returns them for storage on the owning Registry.
//
// New label: "shard" ∈ {"ko", "default"} (cardinality-allowlisted).
func registerTokenizer(r *prometheus.Registry) tokenizerCollectors {
	calls := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "usearch_tokenizer_calls_total",
			Help: "Total tokenizer sidecar calls, partitioned by outcome.",
		},
		[]string{"outcome"},
	)
	latency := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "usearch_tokenizer_latency_seconds",
			Help:    "Tokenizer sidecar round-trip latency distribution.",
			Buckets: tokenizerCallBuckets,
		},
		[]string{"outcome"},
	)
	shardWrites := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "usearch_index_shard_writes_total",
			Help: "Total document write operations, partitioned by target shard.",
		},
		[]string{"shard"},
	)

	r.MustRegister(calls, latency, shardWrites)

	// Pre-initialise with placeholder values so families appear in /metrics
	// output even before any real tokenizer calls are recorded.
	for _, outcome := range []string{"success", "sidecar_unavailable", "timeout", "error"} {
		calls.WithLabelValues(outcome).Add(0)
		latency.WithLabelValues(outcome).Observe(0)
	}
	for _, shard := range []string{"default", "ko"} {
		shardWrites.WithLabelValues(shard).Add(0)
	}

	return tokenizerCollectors{calls: calls, latency: latency, shardWrites: shardWrites}
}
