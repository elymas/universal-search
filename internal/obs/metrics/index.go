// Package metrics — Index-layer metric collectors.
// Owned by SPEC-IDX-001; lives under internal/obs/metrics/ to preserve the
// import-boundary test in SPEC-OBS-001 (TestNoDirectPrometheusImportOutsideObs).
//
// REQ-IDX-011: usearch_index_ops_total, usearch_index_op_duration_seconds,
// usearch_index_fusion_duration_seconds
package metrics

import "github.com/prometheus/client_golang/prometheus"

// indexOpBuckets covers per-store operation latency:
// qdrant (target <200ms), meili (target <300ms), pg (target <100ms).
var indexOpBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.2, 0.3, 0.5, 1.0}

// indexFusionBuckets covers RRF fusion latency which is CPU-bound and fast.
var indexFusionBuckets = []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05}

// indexCollectors bundles the three index metric vectors created per Registry.
// Stored as Registry.IndexOps / IndexOpDuration / IndexFusionDuration fields.
type indexCollectors struct {
	ops            *prometheus.CounterVec
	opDuration     *prometheus.HistogramVec
	fusionDuration prometheus.Histogram
}

// registerIndex creates the three index metric collectors, registers them on r,
// and returns them for storage on the owning Registry.
func registerIndex(r *prometheus.Registry) indexCollectors {
	ops := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "usearch_index_ops_total",
			Help: "Total index operations, partitioned by store, operation, and outcome.",
		},
		[]string{"store", "op", "outcome"},
	)
	opDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "usearch_index_op_duration_seconds",
			Help:    "Per-store index operation latency distribution.",
			Buckets: indexOpBuckets,
		},
		[]string{"store", "op"},
	)
	fusionDuration := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "usearch_index_fusion_duration_seconds",
			Help:    "RRF fusion latency distribution.",
			Buckets: indexFusionBuckets,
		},
	)

	r.MustRegister(ops, opDuration, fusionDuration)

	// Pre-initialise with placeholder values so families appear in /metrics
	// output even before any real index operations are performed.
	for _, store := range []string{"qdrant", "meili", "pg"} {
		for _, op := range []string{"search", "upsert"} {
			ops.WithLabelValues(store, op, "success").Add(0)
			opDuration.WithLabelValues(store, op).Observe(0)
		}
	}
	fusionDuration.Observe(0)

	return indexCollectors{ops: ops, opDuration: opDuration, fusionDuration: fusionDuration}
}
