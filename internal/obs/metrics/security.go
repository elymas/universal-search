// Package metrics — security observability collectors (SPEC-SEC-001).
//
// REQ-SEC-009: usearch_security_ssrf_blocks_total{reason, component}
// REQ-SEC-017: usearch_security_event_total{type, severity}
// REQ-SEC-014: usearch_security_ratelimit_exceeded_total{tenant_id_class}
// NFR-SEC-007: bounded label cardinality (ssrf_blocks 5x3=15 + event 7x4=28 +
// ratelimit 2). tenant_id_class is a 2-value bucket (known|unknown) — the raw
// tenant_id is NEVER a label.
package metrics

import "github.com/prometheus/client_golang/prometheus"

// SecurityCollectors bundles the SPEC-SEC-001 security metric families.
//
// @MX:NOTE: [AUTO] Bounded label sets only (NFR-SEC-007). reason/component/type/
// severity are closed enums; raw identifiers (tenant_id, host) are NEVER labels.
type SecurityCollectors struct {
	// SSRFBlocks counts SSRF guard blocks, partitioned by reason and the
	// calling component. reason ∈ {scheme, private_ip, redirect_hop,
	// dns_rebind, hostname_allowlist}; component ∈ {access, auth, adapter}.
	SSRFBlocks *prometheus.CounterVec

	// SecurityEvents counts emitted security events, partitioned by type and
	// severity. type ∈ the 7-event taxonomy; severity ∈ {critical, high,
	// medium, low}.
	SecurityEvents *prometheus.CounterVec

	// RateLimitExceeded counts per-tenant rate-limit breaches (REQ-SEC-014),
	// partitioned ONLY by tenant_id_class ∈ {known, unknown}. The raw tenant_id
	// is never a label (NFR-SEC-007 cardinality protection).
	RateLimitExceeded *prometheus.CounterVec
}

// rateLimitTenantClasses is the bounded tenant_id_class label allowlist.
var rateLimitTenantClasses = []string{"known", "unknown"}

// ssrfBlockReasons is the bounded reason label allowlist (REQ-SEC-009).
var ssrfBlockReasons = []string{"scheme", "private_ip", "redirect_hop", "dns_rebind", "hostname_allowlist"}

// ssrfBlockComponents is the bounded component label allowlist.
var ssrfBlockComponents = []string{"access", "auth", "adapter"}

// securityEventTypes is the 7-type taxonomy (REQ-SEC-017).
var securityEventTypes = []string{
	"auth.failed", "auth.success", "ssrf.blocked", "secret.scan.finding",
	"ratelimit.exceeded", "rbac.denied", "prompt.sanitized",
}

// securityEventSeverities is the bounded severity label allowlist.
var securityEventSeverities = []string{"critical", "high", "medium", "low"}

// registerSecurity registers the security metric collectors on pr and
// pre-initialises placeholder series so the families appear in /metrics.
func registerSecurity(pr *prometheus.Registry) *SecurityCollectors {
	ssrfBlocks := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "usearch_security_ssrf_blocks_total",
			Help: "Total SSRF guard blocks, partitioned by reason and calling component.",
		},
		[]string{"reason", "component"},
	)

	securityEvents := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "usearch_security_event_total",
			Help: "Total security events emitted, partitioned by type and severity.",
		},
		[]string{"type", "severity"},
	)

	rateLimitExceeded := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "usearch_security_ratelimit_exceeded_total",
			Help: "Total per-tenant rate-limit breaches, partitioned by tenant_id_class (known|unknown).",
		},
		[]string{"tenant_id_class"},
	)

	pr.MustRegister(ssrfBlocks, securityEvents, rateLimitExceeded)

	// Pre-initialise the full bounded label space so cardinality is explicit
	// and the families are present even before the first real observation.
	for _, reason := range ssrfBlockReasons {
		for _, comp := range ssrfBlockComponents {
			ssrfBlocks.WithLabelValues(reason, comp).Add(0)
		}
	}
	for _, t := range securityEventTypes {
		for _, sev := range securityEventSeverities {
			securityEvents.WithLabelValues(t, sev).Add(0)
		}
	}
	for _, c := range rateLimitTenantClasses {
		rateLimitExceeded.WithLabelValues(c).Add(0)
	}

	return &SecurityCollectors{
		SSRFBlocks:        ssrfBlocks,
		SecurityEvents:    securityEvents,
		RateLimitExceeded: rateLimitExceeded,
	}
}

// RecordRateLimitExceeded increments the rate-limit-exceeded counter for the
// given tenant_id_class. Unknown class values are ignored to protect the
// cardinality cap (NFR-SEC-007).
func (s *SecurityCollectors) RecordRateLimitExceeded(tenantIDClass string) {
	if s == nil || s.RateLimitExceeded == nil {
		return
	}
	if !contains(rateLimitTenantClasses, tenantIDClass) {
		return
	}
	s.RateLimitExceeded.WithLabelValues(tenantIDClass).Inc()
}

// RecordSSRFBlock increments the SSRF block counter for the given reason and
// component. Unknown reason/component values are ignored to protect the
// cardinality cap (NFR-SEC-007).
func (s *SecurityCollectors) RecordSSRFBlock(reason, component string) {
	if s == nil || s.SSRFBlocks == nil {
		return
	}
	if !contains(ssrfBlockReasons, reason) || !contains(ssrfBlockComponents, component) {
		return
	}
	s.SSRFBlocks.WithLabelValues(reason, component).Inc()
}

// RecordSecurityEvent increments the security event counter for the given type
// and severity. Unknown values are ignored to protect the cardinality cap.
func (s *SecurityCollectors) RecordSecurityEvent(eventType, severity string) {
	if s == nil || s.SecurityEvents == nil {
		return
	}
	if !contains(securityEventTypes, eventType) || !contains(securityEventSeverities, severity) {
		return
	}
	s.SecurityEvents.WithLabelValues(eventType, severity).Inc()
}

func contains(set []string, v string) bool {
	for _, s := range set {
		if s == v {
			return true
		}
	}
	return false
}
