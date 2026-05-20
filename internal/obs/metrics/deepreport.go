// Package metrics — Deep report metric collectors.
// Owned by SPEC-DEEP-001 M6; lives under internal/obs/metrics/ per SPEC-OBS-001 boundary.
//
// Declares two metric families:
//
//	usearch_deep_outcomes_total{outcome}    CounterVec (6 values)
//	usearch_deep_latency_seconds           Histogram (8 buckets)
//
// outcome label values (pre-declared, per SPEC-DEEP-001 NFR-DEEP1-003):
//
//	success, deadline_exceeded, budget_exceeded,
//	error_invalid, error_upstream, error_unresolved_citations_threshold
//
// @MX:NOTE: [AUTO] Cardinality: outcome label NAME is pre-existing in metrics_test.go:257.
// Only new VALUES are added; no allowlist amendment required (NFR-OBS-002).
package metrics

import "github.com/prometheus/client_golang/prometheus"

// deepReportCollectors bundles the deep report metric vectors.
type deepReportCollectors struct {
	outcomes *prometheus.CounterVec
	latency  prometheus.Histogram
}

// deepReportLatencyBuckets covers realistic long-form report latency.
// Aligned with NFR-DEEP1-001: p50 <= 180s, p95 <= 300s.
var deepReportLatencyBuckets = []float64{5, 15, 30, 60, 120, 180, 240, 300}

// deepReportOutcomeValues are the 6 pre-declared outcome label values.
var deepReportOutcomeValues = []string{
	"success",
	"deadline_exceeded",
	"budget_exceeded",
	"error_invalid",
	"error_upstream",
	"error_unresolved_citations_threshold",
}

// registerDeepReport creates the deep report metric collectors, registers
// them on r, and returns them for storage on the owning Registry.
// Mirrors the registerStreamSynth pattern at internal/obs/metrics/streamsynth.go.
func registerDeepReport(r *prometheus.Registry) deepReportCollectors {
	outcomes := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "usearch_deep_outcomes_total",
			Help: "Total deep report generation outcomes, partitioned by outcome label.",
		},
		[]string{"outcome"},
	)
	latency := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "usearch_deep_latency_seconds",
			Help:    "Distribution of deep report generation latency in seconds.",
			Buckets: deepReportLatencyBuckets,
		},
	)

	r.MustRegister(outcomes, latency)

	// Pre-initialise outcome label values so all families appear in /metrics
	// output even before any report generation calls are made (REQ-OBS-004).
	// Follows the SYN-004 pattern at streamsynth.go:48-56.
	for _, outcome := range deepReportOutcomeValues {
		outcomes.WithLabelValues(outcome).Add(0)
	}
	latency.Observe(0)

	return deepReportCollectors{
		outcomes: outcomes,
		latency:  latency,
	}
}
