package rbac

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// RBACMetrics holds Prometheus collectors for RBAC authorization events.
// NFR-AUTH2-003: All label names are bounded enums (result, reason_class).
type RBACMetrics struct {
	// Decisions counts RBAC policy evaluation results.
	// result ∈ {allow, deny, error}
	// reason_class ∈ {policy_matched, no_policy_matched, explicit_deny, empty_team}
	Decisions *prometheus.CounterVec

	// EvalDuration tracks RBAC policy evaluation latency.
	EvalDuration prometheus.Histogram

	// PolicyReload counts policy reload attempts.
	// outcome ∈ {success, failure}
	PolicyReload *prometheus.CounterVec
}

// NewRBACMetrics creates and registers RBAC metrics on the given registry.
// Returns nil if reg is nil (safe for tests without metrics).
func NewRBACMetrics(reg prometheus.Registerer) *RBACMetrics {
	if reg == nil {
		return nil
	}

	decisions := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "usearch_rbac_decisions_total",
			Help: "Total RBAC policy evaluation results.",
		},
		[]string{"result", "reason_class"},
	)

	evalDuration := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "usearch_rbac_eval_duration_seconds",
			Help:    "RBAC policy evaluation latency distribution.",
			Buckets: prometheus.DefBuckets,
		},
	)

	policyReload := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "usearch_rbac_policy_reload_total",
			Help: "Total RBAC policy reload attempts.",
		},
		[]string{"outcome"},
	)

	reg.MustRegister(decisions, evalDuration, policyReload)

	// Pre-initialise label values so metrics appear in /metrics output
	// even before any real observations.
	for _, result := range []string{"allow", "deny", "error"} {
		for _, rc := range []string{"policy_matched", "no_policy_matched", "explicit_deny", "empty_team"} {
			decisions.WithLabelValues(result, rc).Add(0)
		}
	}
	policyReload.WithLabelValues("success").Add(0)
	policyReload.WithLabelValues("failure").Add(0)

	return &RBACMetrics{
		Decisions:    decisions,
		EvalDuration: evalDuration,
		PolicyReload: policyReload,
	}
}

// RecordDecision records an RBAC policy evaluation result.
func (m *RBACMetrics) RecordDecision(d Decision, elapsed time.Duration) {
	if m == nil {
		return
	}

	result := "deny"
	if d.Allowed {
		result = "allow"
	}

	m.Decisions.WithLabelValues(result, d.ReasonClass).Inc()
	m.EvalDuration.Observe(elapsed.Seconds())
}

// RecordReload records a policy reload outcome.
func (m *RBACMetrics) RecordReload(success bool) {
	if m == nil {
		return
	}

	outcome := "failure"
	if success {
		outcome = "success"
	}
	m.PolicyReload.WithLabelValues(outcome).Inc()
}
