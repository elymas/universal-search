// Package metrics — Deep tree metric collectors.
// Owned by SPEC-DEEP-003 Phase E; lives under internal/obs/metrics/ per SPEC-OBS-001 boundary.
//
// Declares two metric families for tree-mode /deep exploration:
//
//	usearch_deep_tree_node_expand_seconds{depth, outcome}  HistogramVec (6 depths x 3 outcomes = 18 series)
//	usearch_deep_tree_total_tokens{outcome}                CounterVec  (2 outcomes)
//
// @MX:NOTE: [AUTO] Cardinality: depth and outcome label NAMES are bounded enums.
// All label VALUES are pre-declared at registration (NFR-DEEP3-005).
package metrics

import "github.com/prometheus/client_golang/prometheus"

// deepTreeCollectors bundles the deep tree metric vectors.
type deepTreeCollectors struct {
	nodeExpand  *prometheus.HistogramVec
	totalTokens *prometheus.CounterVec
}

// deepTreeDurationBuckets covers tree node expansion durations.
var deepTreeDurationBuckets = []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60}

// deepTreeDepthValues are the 6 pre-declared depth label values.
var deepTreeDepthValues = []string{"0", "1", "2", "3", "4", "5"}

// deepTreeOutcomeValues are the 3 pre-declared outcome label values for histogram.
var deepTreeOutcomeValues = []string{"success", "failed", "budget_exceeded"}

// deepTreeCounterOutcomeValues are the 2 outcome label values for the token counter.
var deepTreeCounterOutcomeValues = []string{"success", "failed"}

// registerDeepTree creates the deep tree metric collectors, registers them on r,
// and returns them for storage on the owning Registry.
func registerDeepTree(r *prometheus.Registry) deepTreeCollectors {
	nodeExpand := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "usearch_deep_tree_node_expand_seconds",
			Help:    "Distribution of deep tree node expansion durations in seconds, partitioned by depth and outcome.",
			Buckets: deepTreeDurationBuckets,
		},
		[]string{"depth", "outcome"},
	)

	totalTokens := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "usearch_deep_tree_total_tokens",
			Help: "Total tokens consumed by deep tree expansion, partitioned by outcome.",
		},
		[]string{"outcome"},
	)

	r.MustRegister(nodeExpand, totalTokens)

	// Pre-initialise label values so all families appear in /metrics
	// output even before any real observations (REQ-OBS-004).
	for _, depth := range deepTreeDepthValues {
		for _, outcome := range deepTreeOutcomeValues {
			nodeExpand.WithLabelValues(depth, outcome).Observe(0)
		}
	}
	for _, outcome := range deepTreeCounterOutcomeValues {
		totalTokens.WithLabelValues(outcome).Add(0)
	}

	return deepTreeCollectors{
		nodeExpand:  nodeExpand,
		totalTokens: totalTokens,
	}
}
