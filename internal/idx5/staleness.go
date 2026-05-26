package idx5

import (
	"time"
)

// CategoryTTL maps document categories to their time-to-live in seconds.
// REQ-IDX5-003 D2: per-category staleness TTL defaults.
var defaultCategoryTTLs = map[string]int{
	"web":      3600,    // 1 hour
	"social":   1800,    // 30 minutes
	"academic": 2592000, // 30 days
	"korean":   3600,    // 1 hour
	"mixed":    7200,    // 2 hours
	"unknown":  7200,    // 2 hours (default)
}

// EvaluateStaleness determines the staleness classification of a cached answer.
// REQ-IDX5-003: age < 0.5*TTL → fresh; 0.5*TTL <= age < TTL → soft-stale;
// age >= TTL → hard-stale. force_stale=true always overrides to hard-stale.
//
// @MX:NOTE: [AUTO] Per-category TTLs define a freshness trade-off: aggressive
// for social (30m), relaxed for academic (30d). The 0.5*TTL soft boundary
// triggers async refresh while still serving the cached response.
// @MX:SPEC: SPEC-IDX-005
func EvaluateStaleness(ca *CachedAnswer, now time.Time, categoryTTLs map[string]int) Staleness {
	if ca.ForceStale {
		return HardStale
	}

	ttl := effectiveTTL(ca, categoryTTLs)
	age := now.Sub(ca.CreatedAt).Seconds()

	if age >= float64(ttl) {
		return HardStale
	}
	if age >= float64(ttl)/2.0 {
		return SoftStale
	}
	return Fresh
}

// effectiveTTL returns the TTL to use for this cached answer.
// If the category has a specific TTL, use that; otherwise use the stored TTL.
func effectiveTTL(ca *CachedAnswer, categoryTTLs map[string]int) int {
	if ttl, ok := categoryTTLs[ca.Category]; ok {
		return ttl
	}
	if ca.TTLSeconds > 0 {
		return ca.TTLSeconds
	}
	return 7200 // default 2 hours
}
