package audit

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds Prometheus collectors for the audit subsystem.
// REQ-AUTH3-009: all usearch_audit_* metrics.
// NFR-AUTH3-006: no PII in labels; bounded enums only.
type Metrics struct {
	// EventsTotal counts all audit events by type, decision, source.
	EventsTotal *prometheus.CounterVec

	// WriteDuration tracks audit write latency.
	WriteDuration prometheus.Histogram

	// ReconcilePollsTotal tracks LiteLLM reconciliation poll outcomes.
	ReconcilePollsTotal *prometheus.CounterVec

	// ReconcileLagSeconds tracks reconciliation lag.
	ReconcileLagSeconds prometheus.Gauge

	// S3ExportDurationSeconds tracks S3 export latency.
	S3ExportDurationSeconds prometheus.Histogram

	// S3ExportRowsTotal tracks rows exported to S3.
	S3ExportRowsTotal prometheus.Counter

	// S3ExportBytesTotal tracks bytes exported to S3.
	S3ExportBytesTotal prometheus.Counter

	// ChainViolationsTotal counts hash chain violations.
	ChainViolationsTotal prometheus.Counter

	// PartitionDropTotal counts partitions dropped.
	PartitionDropTotal prometheus.Counter

	// ReplayRequestsTotal counts replay endpoint invocations by outcome.
	ReplayRequestsTotal *prometheus.CounterVec
}

// RegisterMetrics creates and registers all audit Prometheus collectors.
func RegisterMetrics(reg *prometheus.Registry) *Metrics {
	eventsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "usearch_audit_events_total",
			Help: "Total audit events emitted, partitioned by event_type, decision, and source.",
		},
		[]string{"event_type", "decision", "source"},
	)

	writeDuration := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "usearch_audit_write_duration_seconds",
			Help:    "Audit event write latency distribution.",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1},
		},
	)

	reconcilePolls := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "usearch_audit_reconcile_polls_total",
			Help: "Total LiteLLM reconciliation polls by outcome.",
		},
		[]string{"outcome"},
	)

	reconcileLag := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "usearch_audit_reconcile_lag_seconds",
			Help: "LiteLLM reconciliation lag in seconds.",
		},
	)

	s3ExportDuration := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "usearch_audit_s3_export_duration_seconds",
			Help:    "S3 export duration distribution.",
			Buckets: []float64{10, 30, 60, 120, 300},
		},
	)

	s3ExportRows := prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "usearch_audit_s3_export_rows_total",
			Help: "Total rows exported to S3.",
		},
	)

	s3ExportBytes := prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "usearch_audit_s3_export_bytes_total",
			Help: "Total bytes exported to S3.",
		},
	)

	chainViolations := prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "usearch_audit_chain_violations_total",
			Help: "Total hash chain violations detected.",
		},
	)

	partitionDrop := prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "usearch_audit_partition_drop_total",
			Help: "Total audit partitions dropped.",
		},
	)

	replayRequests := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "usearch_audit_replay_requests_total",
			Help: "Total replay requests by outcome.",
		},
		[]string{"outcome"},
	)

	reg.MustRegister(
		eventsTotal,
		writeDuration,
		reconcilePolls,
		reconcileLag,
		s3ExportDuration,
		s3ExportRows,
		s3ExportBytes,
		chainViolations,
		partitionDrop,
		replayRequests,
	)

	// Pre-initialize label values per SPEC-OBS-001 convention.
	eventsTotal.WithLabelValues("auth.login", "allow", "go").Add(0)
	reconcilePolls.WithLabelValues("success").Add(0)
	reconcilePolls.WithLabelValues("error").Add(0)
	replayRequests.WithLabelValues("allowed").Add(0)
	replayRequests.WithLabelValues("denied").Add(0)
	replayRequests.WithLabelValues("error").Add(0)

	return &Metrics{
		EventsTotal:             eventsTotal,
		WriteDuration:           writeDuration,
		ReconcilePollsTotal:     reconcilePolls,
		ReconcileLagSeconds:     reconcileLag,
		S3ExportDurationSeconds: s3ExportDuration,
		S3ExportRowsTotal:       s3ExportRows,
		S3ExportBytesTotal:      s3ExportBytes,
		ChainViolationsTotal:    chainViolations,
		PartitionDropTotal:      partitionDrop,
		ReplayRequestsTotal:     replayRequests,
	}
}
