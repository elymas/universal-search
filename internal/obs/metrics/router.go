// Package metrics — Intent Router collectors.
// Owned by SPEC-IR-001 but lives under internal/obs/metrics/ to preserve the
// import-boundary invariant set by SPEC-OBS-001 (no direct
// prometheus/client_golang import outside this package).
//
// Adds two collector families:
//
//	usearch_router_classifications_total{outcome}        Counter
//	usearch_router_classification_duration_seconds{outcome}  Histogram
//
// outcome is bounded to ten values (router/metrics.go declares the
// constants). No new label NAME is introduced — only the new metric NAMES.
// SPEC-OBS-001 NFR-OBS-002 cardinality allowlist is unchanged.
package metrics

import "github.com/prometheus/client_golang/prometheus"

// routerCallBuckets covers the bimodal latency distribution of the Intent
// Router: rule-based path < 1 ms, LLM-fallback path 1-3 s.
var routerCallBuckets = []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5}

// routerCollectors bundles the two router metric vectors created per
// Registry. Per-Registry instances avoid the global-variable race that
// surfaces under t.Parallel.
type routerCollectors struct {
	classifications *prometheus.CounterVec
	duration        *prometheus.HistogramVec
}

// registerRouter creates the two router metric collectors, registers them on
// r, and returns them for storage on the owning Registry. Mirrors the
// registerLLM pattern at internal/obs/metrics/llm.go.
func registerRouter(r *prometheus.Registry) routerCollectors {
	classifications := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "usearch_router_classifications_total",
			Help: "Total Intent Router classifications, partitioned by outcome.",
		},
		[]string{"outcome"},
	)
	duration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "usearch_router_classification_duration_seconds",
			Help:    "Intent Router classification latency distribution.",
			Buckets: routerCallBuckets,
		},
		[]string{"outcome"},
	)
	r.MustRegister(classifications, duration)

	// Pre-initialise with a placeholder so the families appear in /metrics
	// output even before the first classification.
	classifications.WithLabelValues("classified_unknown").Add(0)
	duration.WithLabelValues("classified_unknown").Observe(0)

	return routerCollectors{classifications: classifications, duration: duration}
}
