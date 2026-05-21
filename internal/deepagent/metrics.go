package deepagent

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// MetricsRecorder records Prometheus metrics for the deep agent pipeline.
// It wraps the three collectors from internal/obs/metrics and provides
// typed helper methods so the orchestrator does not need to know about
// label value strings.
//
// REQ-DEEP2-008: All label values are bounded enums from types.go Agent constants.
// NFR-DEEP2-002: No user-derived strings in label positions.
type MetricsRecorder struct {
	duration    *prometheus.HistogramVec
	retries     *prometheus.CounterVec
	verifierGate *prometheus.CounterVec
}

// NewMetricsRecorder creates a MetricsRecorder from the three Prometheus collectors.
// Any nil collector is safely ignored (no-op recording).
func NewMetricsRecorder(duration *prometheus.HistogramVec, retries *prometheus.CounterVec, verifierGate *prometheus.CounterVec) *MetricsRecorder {
	return &MetricsRecorder{
		duration:     duration,
		retries:      retries,
		verifierGate: verifierGate,
	}
}

// RecordAgentDuration records the duration of a single agent call.
// agent must be one of the bounded Agent constants.
// outcome must be "success" or "error".
func (m *MetricsRecorder) RecordAgentDuration(agent Agent, outcome string, dur time.Duration) {
	if m.duration == nil {
		return
	}
	m.duration.WithLabelValues(string(agent), outcome).Observe(dur.Seconds())
}

// RecordWriterRetry increments the writer retries counter by 1.
// REQ-DEEP2-003: Only writer retries are tracked.
func (m *MetricsRecorder) RecordWriterRetry() {
	if m.retries == nil {
		return
	}
	m.retries.WithLabelValues("writer").Inc()
}

// RecordVerifierGateResult records a verifier gate result.
// result must be one of "pass", "fail_uncited", "fail_error".
func (m *MetricsRecorder) RecordVerifierGateResult(result string) {
	if m.verifierGate == nil {
		return
	}
	m.verifierGate.WithLabelValues(result).Inc()
}
