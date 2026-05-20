// Package metrics — SynthCluster metric collectors.
// Owned by SPEC-SYN-003; lives under internal/obs/metrics/ to preserve the
// import-boundary invariant set by SPEC-OBS-001.
//
// Declares two metric families:
//
//	usearch_synthcluster_outcomes_total{outcome, mode}   CounterVec
//	usearch_synthcluster_members                         Histogram
//
// Counter semantics (mode-exclusive outcomes — SPEC-SYN-003 §3 Counter Semantics):
//
//	simhash_only mode: only "simhash_clustered" fires
//	hybrid mode (success): only "hybrid_refined" fires
//	hybrid mode (degraded): only "embedding_fallback" fires
//	off mode: only "passthrough" fires
//
// @MX:NOTE: [AUTO] Cardinality allowlist amendment: label "mode" with values
// "simhash_only"/"hybrid"/"off" is a new addition to NFR-OBS-002 allowlist.
// Registered via registerSynthCluster(pr) in metrics.go.
package metrics

import "github.com/prometheus/client_golang/prometheus"

// synthClusterMemberBuckets covers expected cluster-size distribution:
// singleton (1) skipped, typical small clusters (2-10), large (20-50).
var synthClusterMemberBuckets = []float64{1, 2, 3, 5, 10, 20, 50}

// synthClusterCollectors bundles the synthcluster metric vectors.
// Stored as Registry.SynthClusterOutcomes / SynthClusterMembers fields.
type synthClusterCollectors struct {
	outcomes *prometheus.CounterVec
	members  prometheus.Histogram
}

// registerSynthCluster creates the two synthcluster metric collectors, registers
// them on r, and returns them for storage on the owning Registry.
// Mirrors the registerSynthesis pattern at internal/obs/metrics/synthesis.go.
//
// @MX:ANCHOR: [AUTO] SynthCluster metric registration; callers: NewRegistry, tests
// @MX:REASON: fan_in >= 3; all synthcluster Prometheus collection routes through here
func registerSynthCluster(r *prometheus.Registry) synthClusterCollectors {
	outcomes := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "usearch_synthcluster_outcomes_total",
			Help: "Total synthcluster outcomes, partitioned by outcome and mode (SPEC-SYN-003 counter-exclusivity rule).",
		},
		[]string{"outcome", "mode"},
	)
	members := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "usearch_synthcluster_members",
			Help:    "Cluster-size distribution (number of docs per cluster, including representative).",
			Buckets: synthClusterMemberBuckets,
		},
	)

	r.MustRegister(outcomes, members)

	// Pre-initialise with placeholder values so families appear in /metrics
	// output even before any clustering calls are made (REQ-OBS-004).
	outcomes.WithLabelValues("passthrough", "off").Add(0)
	outcomes.WithLabelValues("simhash_clustered", "simhash_only").Add(0)
	outcomes.WithLabelValues("hybrid_refined", "hybrid").Add(0)
	outcomes.WithLabelValues("embedding_fallback", "hybrid").Add(0)
	members.Observe(0)

	return synthClusterCollectors{outcomes: outcomes, members: members}
}
