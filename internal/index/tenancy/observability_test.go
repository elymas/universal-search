package tenancy

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestRegisterTenantCollectorsRegistersAll(t *testing.T) {
	t.Parallel()
	r := prometheus.NewRegistry()
	tc := RegisterTenantCollectors(r)

	if tc.TokenIssued == nil {
		t.Error("TokenIssued should be non-nil")
	}
	if tc.TokenRevoked == nil {
		t.Error("TokenRevoked should be non-nil")
	}
	if tc.TokenValidationFailures == nil {
		t.Error("TokenValidationFailures should be non-nil")
	}
	if tc.BackfillOps == nil {
		t.Error("BackfillOps should be non-nil")
	}
}

func TestTenantCollectorsIncrement(t *testing.T) {
	t.Parallel()
	r := prometheus.NewRegistry()
	tc := RegisterTenantCollectors(r)

	tc.TokenIssued.WithLabelValues("meili", "success").Inc()
	tc.TokenRevoked.WithLabelValues("meili").Inc()
	tc.TokenValidationFailures.WithLabelValues("meili", "rejected").Inc()
	tc.BackfillOps.WithLabelValues("pg", "success").Inc()

	// Verify the metrics are registered and collectible.
	mfs, err := r.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	names := make(map[string]bool)
	for _, mf := range mfs {
		names[mf.GetName()] = true
	}
	for _, name := range []string{
		"usearch_index_tenant_token_issued_total",
		"usearch_index_tenant_token_revoked_total",
		"usearch_index_tenant_token_validation_failures_total",
		"usearch_index_tenant_backfill_total",
	} {
		if !names[name] {
			t.Errorf("metric %q not found in registry", name)
		}
	}
}
