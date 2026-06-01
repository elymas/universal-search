// Package metrics — SPEC-EVAL-001 benchmark collectors.
// Lives under internal/obs/metrics/ to preserve the import-boundary invariant
// set by SPEC-OBS-001. Reuses the existing `outcome` label; adds no new label
// name to the cardinality allowlist (NFR-OBS-002).
//
// Declares two metric families:
//
//	usearch_eval_runs_total{outcome}   Counter   (outcome in {pass, fail, null})
//	usearch_eval_score_gauge           Gauge     (last aggregate mean score)
package metrics

import "github.com/prometheus/client_golang/prometheus"

// evalCollectors bundles the eval benchmark metric vectors.
type evalCollectors struct {
	runs  *prometheus.CounterVec
	score prometheus.Gauge
}

// registerEval creates the eval collectors, registers them on r, and returns
// them for storage on the owning Registry. Mirrors the registerSynthesis
// pattern at internal/obs/metrics/synthesis.go.
func registerEval(r *prometheus.Registry) evalCollectors {
	runs := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "usearch_eval_runs_total",
			Help: "Total SPEC-EVAL-001 benchmark runs, partitioned by outcome (pass|fail|null).",
		},
		[]string{"outcome"},
	)
	score := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "usearch_eval_score_gauge",
			Help: "Most recent SPEC-EVAL-001 aggregate mean faithfulness score.",
		},
	)

	r.MustRegister(runs, score)

	// Pre-initialise so families appear in /metrics before any run (REQ-OBS-004).
	runs.WithLabelValues("pass").Add(0)
	score.Set(0)

	return evalCollectors{runs: runs, score: score}
}
