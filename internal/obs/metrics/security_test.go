package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestSecurityCollectorsRegistered(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	if r.Security == nil || r.Security.SSRFBlocks == nil || r.Security.SecurityEvents == nil {
		t.Fatal("security collectors must be registered on the Registry")
	}
}

func TestRecordSSRFBlockIncrements(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	r.Security.RecordSSRFBlock("hostname_allowlist", "access")
	got := testutil.ToFloat64(r.Security.SSRFBlocks.WithLabelValues("hostname_allowlist", "access"))
	if got != 1 {
		t.Errorf("ssrf_blocks_total{reason=hostname_allowlist,component=access} = %v, want 1", got)
	}
}

func TestRecordSSRFBlockRejectsUnknownLabels(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	// Unknown reason/component must be ignored (cardinality protection).
	r.Security.RecordSSRFBlock("bogus_reason", "access")
	r.Security.RecordSSRFBlock("scheme", "bogus_component")
	// The valid placeholder series stays at 0.
	if got := testutil.ToFloat64(r.Security.SSRFBlocks.WithLabelValues("scheme", "access")); got != 0 {
		t.Errorf("unknown-label records must not increment a valid series; got %v", got)
	}
}

func TestRecordSecurityEventIncrements(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	r.Security.RecordSecurityEvent("ssrf.blocked", "medium")
	got := testutil.ToFloat64(r.Security.SecurityEvents.WithLabelValues("ssrf.blocked", "medium"))
	if got != 1 {
		t.Errorf("security_event_total{type=ssrf.blocked,severity=medium} = %v, want 1", got)
	}
}

func TestRecordRateLimitExceededIncrements(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	r.Security.RecordRateLimitExceeded("known")
	got := testutil.ToFloat64(r.Security.RateLimitExceeded.WithLabelValues("known"))
	if got != 1 {
		t.Errorf("ratelimit_exceeded_total{tenant_id_class=known} = %v, want 1", got)
	}
}

func TestRecordRateLimitExceededRejectsUnknownClass(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	// A raw tenant_id (not a bounded class) must be rejected to protect the cap.
	r.Security.RecordRateLimitExceeded("tenant-xyz-123")
	if got := testutil.ToFloat64(r.Security.RateLimitExceeded.WithLabelValues("known")); got != 0 {
		t.Errorf("unknown class must not increment a valid series; got %v", got)
	}
}

func TestRateLimitExceededCardinalityCap(t *testing.T) {
	t.Parallel()
	// REQ-SEC-014 / NFR-SEC-007: tenant_id_class is a 2-value bounded bucket.
	if len(rateLimitTenantClasses) != 2 {
		t.Errorf("tenant_id_class cardinality = %d, want exactly 2 (known|unknown)", len(rateLimitTenantClasses))
	}
}

func TestSecurityEventCardinalityCap(t *testing.T) {
	t.Parallel()
	// REQ-SEC-017 / NFR-SEC-007: <= 28 unique (type, severity) combinations.
	combos := len(securityEventTypes) * len(securityEventSeverities)
	if combos > 28 {
		t.Errorf("security_event_total cardinality = %d, exceeds cap 28", combos)
	}
	ssrfCombos := len(ssrfBlockReasons) * len(ssrfBlockComponents)
	if ssrfCombos > 15 {
		t.Errorf("ssrf_blocks_total cardinality = %d, exceeds cap 15", ssrfCombos)
	}
}
