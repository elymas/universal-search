// Package metrics — Deep agent metric collectors.
// Owned by SPEC-DEEP-002 M6; lives under internal/obs/metrics/ per SPEC-OBS-001 boundary.
//
// Declares three metric families for the multi-agent pipeline:
//
//	usearch_deep_agent_duration_seconds{agent, outcome}  HistogramVec (4 agents x 2 outcomes = 8 series)
//	usearch_deep_agent_retries_total{agent}              CounterVec  (1 value: agent="writer")
//	usearch_deep_agent_verifier_gate_results_total{result} CounterVec (3 values)
//
// Also extends the existing usearch_deep_outcomes_total with two new values:
// empty_corpus, error_pipeline_failed.
//
// @MX:NOTE: [AUTO] Cardinality: agent and result label NAMES are added to metrics_test.go allowlist.
// All label VALUES are pre-declared at registration (NFR-DEEP2-002).
package metrics

import "github.com/prometheus/client_golang/prometheus"

// deepAgentCollectors bundles the deep agent metric vectors.
type deepAgentCollectors struct {
	duration         *prometheus.HistogramVec
	retries          *prometheus.CounterVec
	verifierGate     *prometheus.CounterVec
}

// deepAgentDurationBuckets covers realistic agent call durations.
// REQ-DEEP2-008: buckets [0.5, 1, 2, 5, 10, 30, 60, 120].
var deepAgentDurationBuckets = []float64{0.5, 1, 2, 5, 10, 30, 60, 120}

// deepAgentLabelValues are the 4 pre-declared agent label values.
// NFR-DEEP2-002: bounded enum from deepagent.Agent constants.
var deepAgentLabelValues = []string{"researcher", "reviewer", "writer", "verifier"}

// deepAgentOutcomeValues are the 2 pre-declared outcome label values for duration.
var deepAgentOutcomeValues = []string{"success", "error"}

// deepAgentRetryLabelValues are the pre-declared agent values for retries counter.
// Only writer retries are tracked (REQ-DEEP2-003).
var deepAgentRetryLabelValues = []string{"writer"}

// deepAgentVerifierGateResultValues are the 3 pre-declared result label values.
// REQ-DEEP2-006: result in {pass, fail_uncited, fail_error}.
var deepAgentVerifierGateResultValues = []string{"pass", "fail_uncited", "fail_error"}

// deepOutcomeExtValues are the new outcome values added to usearch_deep_outcomes_total.
// REQ-DEEP2-008: extend existing collector with empty_corpus and error_pipeline_failed.
var deepOutcomeExtValues = []string{"empty_corpus", "error_pipeline_failed"}

// registerDeepAgent creates the deep agent metric collectors, registers
// them on r, extends the existing DeepReportOutcomes with new label values,
// and returns them for storage on the owning Registry.
// Mirrors the registerDeepReport pattern at internal/obs/metrics/deepreport.go.
func registerDeepAgent(r *prometheus.Registry, deepOutcomes *prometheus.CounterVec) deepAgentCollectors {
	duration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "usearch_deep_agent_duration_seconds",
			Help:    "Distribution of deep agent call durations in seconds, partitioned by agent and outcome.",
			Buckets: deepAgentDurationBuckets,
		},
		[]string{"agent", "outcome"},
	)

	retries := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "usearch_deep_agent_retries_total",
			Help: "Total deep agent retries, partitioned by agent. Only writer retries are tracked.",
		},
		[]string{"agent"},
	)

	verifierGate := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "usearch_deep_agent_verifier_gate_results_total",
			Help: "Total verifier gate results, partitioned by result (pass, fail_uncited, fail_error).",
		},
		[]string{"result"},
	)

	r.MustRegister(duration, retries, verifierGate)

	// Pre-initialise label values so all families appear in /metrics
	// output even before any agent calls are made (REQ-OBS-004).
	// Follows the SYN-004 pattern at streamsynth.go:48-56.

	// Duration histogram: 4 agents x 2 outcomes = 8 pre-declared series.
	for _, agent := range deepAgentLabelValues {
		for _, outcome := range deepAgentOutcomeValues {
			duration.WithLabelValues(agent, outcome).Observe(0)
		}
	}

	// Retries counter: only writer.
	for _, agent := range deepAgentRetryLabelValues {
		retries.WithLabelValues(agent).Add(0)
	}

	// Verifier gate results: 3 values.
	for _, result := range deepAgentVerifierGateResultValues {
		verifierGate.WithLabelValues(result).Add(0)
	}

	// Extend existing DeepReportOutcomes with new values.
	if deepOutcomes != nil {
		for _, outcome := range deepOutcomeExtValues {
			deepOutcomes.WithLabelValues(outcome).Add(0)
		}
	}

	return deepAgentCollectors{
		duration:     duration,
		retries:      retries,
		verifierGate: verifierGate,
	}
}
