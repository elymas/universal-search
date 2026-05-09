// Package metrics — Stream synthesis metric collectors.
// Owned by SPEC-SYN-004; lives under internal/obs/metrics/ per SPEC-OBS-001 boundary.
//
// Declares two metric families:
//
//	usearch_syn004_outcomes_total{outcome}          CounterVec
//	usearch_syn004_sentences_emitted               Histogram
//
// outcome label allowlist (5 values, per SPEC-OBS-001 cardinality discipline):
//
//	streamed_complete, client_disconnect, write_timeout,
//	error_upstream, accept_fallback_to_json
package metrics

import "github.com/prometheus/client_golang/prometheus"

// streamSynthCollectors bundles the two stream synthesis metric vectors.
type streamSynthCollectors struct {
	outcomes         *prometheus.CounterVec
	sentencesEmitted prometheus.Histogram
}

// sentencesEmittedBuckets covers realistic per-stream sentence counts.
var sentencesEmittedBuckets = []float64{0, 1, 2, 3, 5, 8, 13}

// registerStreamSynth creates the two stream synthesis metric collectors, registers
// them on r, and returns them for storage on the owning Registry.
func registerStreamSynth(r *prometheus.Registry) streamSynthCollectors {
	outcomes := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "usearch_syn004_outcomes_total",
			Help: "Total streaming synthesis outcomes, partitioned by outcome label.",
		},
		[]string{"outcome"},
	)
	sentencesEmitted := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "usearch_syn004_sentences_emitted",
			Help:    "Distribution of sentences emitted per streaming synthesis call.",
			Buckets: sentencesEmittedBuckets,
		},
	)

	r.MustRegister(outcomes, sentencesEmitted)

	// Pre-initialise outcome label values so all families appear in /metrics
	// output even before any streaming calls are made (REQ-OBS-004).
	for _, outcome := range []string{
		"streamed_complete",
		"client_disconnect",
		"write_timeout",
		"error_upstream",
		"accept_fallback_to_json",
	} {
		outcomes.WithLabelValues(outcome).Add(0)
	}

	return streamSynthCollectors{
		outcomes:         outcomes,
		sentencesEmitted: sentencesEmitted,
	}
}
