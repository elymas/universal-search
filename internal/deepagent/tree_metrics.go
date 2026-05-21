package deepagent

// @MX:NOTE: [AUTO] Prometheus + OTel observability for /deep tree expansion
// @MX:SPEC: SPEC-DEEP-003 Phase E

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Pre-declared label values for bounded cardinality.
// REQ-DEEP3-012, NFR-DEEP3-005: No user-input strings reach labels.
// Total cardinality = 6 depths x 3 outcomes (histogram) + 2 outcomes (counter) = 20.

var (
	treeMetricsDepthValues   = []string{"0", "1", "2", "3", "4", "5"}
	treeMetricsOutcomeValues = []string{"success", "failed", "budget_exceeded"}
)

// treeMetricsCounterOutcomeValues are the outcome label values for the total_tokens counter.
var treeMetricsCounterOutcomeValues = []string{"success", "failed"}

// treeNodeExpandBuckets covers tree node expansion durations.
// Nodes at depth 0 are typically faster (single fanout); deeper nodes
// involve decompose + fanout so can be slower.
var treeNodeExpandBuckets = []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60}

// TreeMetricsRecorder records Prometheus metrics for /deep tree expansion.
// It wraps two collectors and provides typed helper methods.
//
// REQ-DEEP3-012: usearch_deep_tree_node_expand_seconds{depth, outcome} histogram
// REQ-DEEP3-012: usearch_deep_tree_total_tokens{outcome} counter
// NFR-DEEP3-005: All label values are pre-declared constants.
type TreeMetricsRecorder struct {
	nodeExpand *prometheus.HistogramVec
	totalTokens *prometheus.CounterVec
}

// NewTreeMetricsRecorder creates and registers tree metrics on the given Prometheus registry.
// Returns nil-safe recorder; all methods are no-ops if pr is nil.
func NewTreeMetricsRecorder(pr *prometheus.Registry) *TreeMetricsRecorder {
	if pr == nil {
		return &TreeMetricsRecorder{}
	}

	nodeExpand := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "usearch_deep_tree_node_expand_seconds",
			Help:    "Distribution of deep tree node expansion durations in seconds, partitioned by depth and outcome.",
			Buckets: treeNodeExpandBuckets,
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

	pr.MustRegister(nodeExpand, totalTokens)

	// Pre-initialise label values so metric families appear in /metrics
	// output even before any real observations (REQ-OBS-004).
	for _, depth := range treeMetricsDepthValues {
		for _, outcome := range treeMetricsOutcomeValues {
			nodeExpand.WithLabelValues(depth, outcome).Observe(0)
		}
	}
	for _, outcome := range treeMetricsCounterOutcomeValues {
		totalTokens.WithLabelValues(outcome).Add(0)
	}

	return &TreeMetricsRecorder{
		nodeExpand: nodeExpand,
		totalTokens: totalTokens,
	}
}

// RecordNodeExpand records the duration of a single node expansion.
// depth is the node depth level (0-5). outcome must be one of the pre-declared values.
func (r *TreeMetricsRecorder) RecordNodeExpand(depth int, outcome string, dur time.Duration) {
	if r.nodeExpand == nil {
		return
	}
	depthStr := fmt.Sprintf("%d", depth)
	r.nodeExpand.WithLabelValues(depthStr, outcome).Observe(dur.Seconds())
}

// RecordTotalTokens increments the total tokens counter by the given amount.
func (r *TreeMetricsRecorder) RecordTotalTokens(outcome string, tokens int64) {
	if r.totalTokens == nil {
		return
	}
	r.totalTokens.WithLabelValues(outcome).Add(float64(tokens))
}

// TreeHooks provides optional callbacks for observability integration into ExpandTree.
// All hooks are nil-safe — ExpandTree checks before calling.
//
// @MX:NOTE: [AUTO] Hook pattern decouples tree logic from observability; tests pass nil hooks
type TreeHooks struct {
	// OnNodeComplete is called when a node finishes expansion successfully.
	OnNodeComplete func(node *Node, dur time.Duration)
	// OnNodeFailed is called when a node expansion fails.
	OnNodeFailed func(node *Node, dur time.Duration)
	// OnNodeBudgetExceeded is called when a node is denied due to budget.
	OnNodeBudgetExceeded func(node *Node)
}

// startNodeSpan creates an OTel span for a tree node expansion.
// If parentSpan is non-nil, the new span is linked as a child.
// When OTel is not configured (test environments), returns context.Background() and a no-op span.
//
// REQ-DEEP3-012, NFR-DEEP3-006: Each child node's span references parent's span_id.
func startNodeSpan(ctx context.Context, node *Node, parentSpan trace.Span) (context.Context, trace.Span) {
	tracer := otel.Tracer("deep-tree")
	spanName := fmt.Sprintf("tree.expand.depth_%d.node_%s", node.Depth, node.ID)

	opts := []trace.SpanStartOption{
		trace.WithAttributes(
			attribute.String("tree.node.id", node.ID),
			attribute.Int("tree.node.depth", node.Depth),
			attribute.String("tree.node.parent_id", node.ParentID),
			attribute.String("tree.node.query", node.Query),
		),
	}

	// If parentSpan is valid, link via parent context.
	if parentSpan != nil && parentSpan.SpanContext().IsValid() {
		parentCtx := trace.ContextWithSpan(ctx, parentSpan)
		return tracer.Start(parentCtx, spanName, opts...)
	}

	return tracer.Start(ctx, spanName, opts...)
}
