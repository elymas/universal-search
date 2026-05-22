package rbac

import (
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewRBACMetricsRegistersCollectors verifies that all three RBAC metrics
// are registered and non-nil.
// NFR-AUTH2-003.
func TestNewRBACMetricsRegistersCollectors(t *testing.T) {
	reg := prometheus.NewPedanticRegistry()
	m := NewRBACMetrics(reg)

	require.NotNil(t, m)
	assert.NotNil(t, m.Decisions)
	assert.NotNil(t, m.EvalDuration)
	assert.NotNil(t, m.PolicyReload)
}

// TestNewRBACMetricsNilRegistryReturnsNil verifies graceful nil handling.
func TestNewRBACMetricsNilRegistryReturnsNil(t *testing.T) {
	m := NewRBACMetrics(nil)
	assert.Nil(t, m)
}

// TestRecordDecisionIncrementsCounter verifies that RecordDecision increments
// the correct label combination.
// NFR-AUTH2-003.
func TestRecordDecisionIncrementsCounter(t *testing.T) {
	reg := prometheus.NewPedanticRegistry()
	m := NewRBACMetrics(reg)

	d := Decision{
		Allowed:     true,
		UserID:      "alice",
		TeamID:      "engineering",
		Resource:    "query:basic",
		Action:      "read",
		ReasonClass: "policy_matched",
	}

	m.RecordDecision(d, 5*time.Millisecond)

	// Verify the counter was incremented by gathering metrics.
	families, err := reg.Gather()
	require.NoError(t, err)

	found := false
	for _, f := range families {
		if f.GetName() == "usearch_rbac_decisions_total" {
			found = true
			// Only the exact label combo recorded above should be > 0.
			for _, sample := range f.GetMetric() {
				labels := labelMap(sample)
				if labels["result"] == "allow" && labels["reason_class"] == "policy_matched" {
					assert.Greater(t, sample.GetCounter().GetValue(), 0.0)
				}
			}
		}
	}
	assert.True(t, found, "usearch_rbac_decisions_total must be registered")
}

// TestRecordDecisionNilMetricsIsNoop verifies RecordDecision is safe on nil.
func TestRecordDecisionNilMetricsIsNoop(t *testing.T) {
	var m *RBACMetrics
	assert.NotPanics(t, func() {
		m.RecordDecision(Decision{Allowed: true, ReasonClass: "policy_matched"}, time.Millisecond)
	})
}

// TestRecordDecisionDenyRecordsCorrectly verifies deny result label.
func TestRecordDecisionDenyRecordsCorrectly(t *testing.T) {
	reg := prometheus.NewPedanticRegistry()
	m := NewRBACMetrics(reg)

	d := Decision{
		Allowed:     false,
		UserID:      "david",
		TeamID:      "engineering",
		Resource:    "audit_log",
		Action:      "read",
		ReasonClass: "no_policy_matched",
	}

	m.RecordDecision(d, 1*time.Millisecond)

	families, err := reg.Gather()
	require.NoError(t, err)

	for _, f := range families {
		if f.GetName() == "usearch_rbac_decisions_total" {
			for _, metric := range f.GetMetric() {
				labels := labelMap(metric)
				if labels["result"] == "deny" && labels["reason_class"] == "no_policy_matched" {
					assert.Greater(t, metric.GetCounter().GetValue(), 0.0)
				}
			}
		}
	}
}

// TestRecordReloadIncrementsCounter verifies reload outcome recording.
func TestRecordReloadIncrementsCounter(t *testing.T) {
	reg := prometheus.NewPedanticRegistry()
	m := NewRBACMetrics(reg)

	m.RecordReload(true)
	m.RecordReload(false)

	families, err := reg.Gather()
	require.NoError(t, err)

	for _, f := range families {
		if f.GetName() == "usearch_rbac_policy_reload_total" {
			var successFound, failureFound bool
			for _, metric := range f.GetMetric() {
				labels := labelMap(metric)
				if labels["outcome"] == "success" && metric.GetCounter().GetValue() > 0 {
					successFound = true
				}
				if labels["outcome"] == "failure" && metric.GetCounter().GetValue() > 0 {
					failureFound = true
				}
			}
			assert.True(t, successFound, "success outcome must be recorded")
			assert.True(t, failureFound, "failure outcome must be recorded")
		}
	}
}

// TestRecordReloadNilMetricsIsNoop verifies RecordReload is safe on nil.
func TestRecordReloadNilMetricsIsNoop(t *testing.T) {
	var m *RBACMetrics
	assert.NotPanics(t, func() {
		m.RecordReload(true)
	})
}

// TestEvalDurationHistogramRecordsObservation verifies latency tracking.
func TestEvalDurationHistogramRecordsObservation(t *testing.T) {
	reg := prometheus.NewPedanticRegistry()
	m := NewRBACMetrics(reg)

	m.RecordDecision(Decision{Allowed: true, ReasonClass: "policy_matched"}, 100*time.Millisecond)

	families, err := reg.Gather()
	require.NoError(t, err)

	for _, f := range families {
		if f.GetName() == "usearch_rbac_eval_duration_seconds" {
			assert.Greater(t, f.GetMetric()[0].GetHistogram().GetSampleSum(), 0.0)
		}
	}
}

// labelMap extracts metric labels into a map for easier assertions.
func labelMap(m *dto.Metric) map[string]string {
	labels := make(map[string]string)
	for _, l := range m.GetLabel() {
		labels[l.GetName()] = l.GetValue()
	}
	return labels
}
