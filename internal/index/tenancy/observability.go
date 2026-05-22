package tenancy

import (
	"github.com/prometheus/client_golang/prometheus"
)

// TenantCollectors holds Prometheus collectors for tenant token metrics.
// REQ-IDX4-009: 3 metric families for tenant token lifecycle.
type TenantCollectors struct {
	// TokenIssued counts tenant token issuance events.
	TokenIssued *prometheus.CounterVec
	// TokenRevoked counts tenant token revocation events.
	TokenRevoked *prometheus.CounterVec
	// TokenValidationFailures counts tenant token validation failures.
	TokenValidationFailures *prometheus.CounterVec
	// BackfillOps counts backfill operations per store.
	BackfillOps *prometheus.CounterVec
}

// RegisterTenantCollectors creates and registers tenant token metrics on the given registry.
func RegisterTenantCollectors(r *prometheus.Registry) *TenantCollectors {
	tc := &TenantCollectors{
		TokenIssued: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "usearch_index_tenant_token_issued_total",
				Help: "Total tenant token issuance events partitioned by tier and outcome.",
			},
			[]string{"tier", "outcome"},
		),
		TokenRevoked: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "usearch_index_tenant_token_revoked_total",
				Help: "Total tenant token revocation events partitioned by tier.",
			},
			[]string{"tier"},
		),
		TokenValidationFailures: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "usearch_index_tenant_token_validation_failures_total",
				Help: "Total tenant token validation failures partitioned by tier and outcome.",
			},
			[]string{"tier", "outcome"},
		),
		BackfillOps: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "usearch_index_tenant_backfill_total",
				Help: "Total backfill operations partitioned by store and outcome.",
			},
			[]string{"store", "outcome"},
		),
	}

	r.MustRegister(
		tc.TokenIssued,
		tc.TokenRevoked,
		tc.TokenValidationFailures,
		tc.BackfillOps,
	)

	// Pre-initialise with placeholder label values.
	tc.TokenIssued.WithLabelValues("meili", "success").Add(0)
	tc.TokenRevoked.WithLabelValues("meili").Add(0)
	tc.TokenValidationFailures.WithLabelValues("meili", "rejected").Add(0)
	tc.BackfillOps.WithLabelValues("pg", "success").Add(0)

	return tc
}
