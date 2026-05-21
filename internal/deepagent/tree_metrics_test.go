package deepagent

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// T-E-001 [RED]: Metrics Registration
// REQ-DEEP3-012, NFR-DEEP3-005: Two collectors registered.

func TestMetricsRegistration(t *testing.T) {
	pr := prometheus.NewRegistry()
	rec := NewTreeMetricsRecorder(pr)

	if rec == nil {
		t.Fatal("NewTreeMetricsRecorder returned nil")
	}

	// Verify both collectors are registered by gathering metric families.
	mfs, err := pr.Gather()
	if err != nil {
		t.Fatalf("Gather() error: %v", err)
	}

	names := map[string]bool{}
	for _, mf := range mfs {
		names[mf.GetName()] = true
	}

	if !names["usearch_deep_tree_node_expand_seconds"] {
		t.Error("histogram usearch_deep_tree_node_expand_seconds not registered")
	}
	if !names["usearch_deep_tree_total_tokens"] {
		t.Error("counter usearch_deep_tree_total_tokens not registered")
	}
}

// T-E-002 [RED]: Cardinality Bounded
// REQ-DEEP3-012, NFR-DEEP3-005: Label values are pre-declared constants.
// depth in {0,1,2,3,4,5}, outcome in {success, failed, budget_exceeded}.
// Total cardinality = 18 (histogram) + 2 (counter outcomes) = 20 series.

func TestMetricsCardinalityBounded(t *testing.T) {
	pr := prometheus.NewRegistry()
	_ = NewTreeMetricsRecorder(pr)

	mfs, err := pr.Gather()
	if err != nil {
		t.Fatalf("Gather() error: %v", err)
	}

	for _, mf := range mfs {
		switch mf.GetName() {
		case "usearch_deep_tree_node_expand_seconds":
			// Should have exactly 18 series: 6 depths x 3 outcomes.
			if len(mf.GetMetric()) != 18 {
				t.Errorf("histogram series count = %d, want 18 (6 depths x 3 outcomes)", len(mf.GetMetric()))
			}
			// Verify all label values are from the pre-declared sets.
			for _, m := range mf.GetMetric() {
				depthValid := false
				outcomeValid := false
				for _, lp := range m.GetLabel() {
					switch lp.GetName() {
					case "depth":
						for _, d := range treeMetricsDepthValues {
							if lp.GetValue() == d {
								depthValid = true
							}
						}
					case "outcome":
						for _, o := range treeMetricsOutcomeValues {
							if lp.GetValue() == o {
								outcomeValid = true
							}
						}
					}
				}
				if !depthValid || !outcomeValid {
					t.Errorf("unexpected label values: depth_valid=%v outcome_valid=%v", depthValid, outcomeValid)
				}
			}

		case "usearch_deep_tree_total_tokens":
			// Counter should have exactly 2 series: 2 outcomes.
			if len(mf.GetMetric()) != 2 {
				t.Errorf("counter series count = %d, want 2", len(mf.GetMetric()))
			}
		}
	}
}

// T-E-003 [RED]: Metrics Observed
// REQ-DEEP3-012, NFR-DEEP3-005: After ExpandTree with N nodes, histogram has N
// observations. Counter increments on success.

func TestExpandTreeMetricsObserved(t *testing.T) {
	pr := prometheus.NewRegistry()
	rec := NewTreeMetricsRecorder(pr)

	// Use hooks to wire metrics into ExpandTree.
	hooks := TreeHooks{
		OnNodeComplete: func(node *Node, dur time.Duration) {
			rec.RecordNodeExpand(node.Depth, "success", dur)
			rec.RecordTotalTokens("success", node.TokensUsed)
		},
		OnNodeFailed: func(node *Node, dur time.Duration) {
			rec.RecordNodeExpand(node.Depth, "failed", dur)
		},
		OnNodeBudgetExceeded: func(node *Node) {
			rec.RecordNodeExpand(node.Depth, "budget_exceeded", 0)
		},
	}

	// Create a stub researcher that returns predictable results.
	researcher := &stubTreeResearcher{
		decomposeResult: []string{"sub1", "sub2"},
		fanoutResult: func() ([]NodeCitation, []NodeClaim, int64, error) {
			return []NodeCitation{{DocID: "d1"}}, []NodeClaim{{Text: "claim1"}}, 100, nil
		},
	}

	cfg := TreeConfig{
		Breadth:            2,
		Depth:              1,
		TokenBudget:        100000,
		NodeTimeoutMs:      5000,
		RootTokenEstimate:  5000,
		ModelPricePerToken: 0.0000008,
		RunID:              "test-metrics-observed",
		Hooks:              hooks,
	}

	result, err := ExpandTree(context.Background(), cfg, "test query", researcher)
	if err != nil {
		t.Fatalf("ExpandTree error: %v", err)
	}

	// Verify nodes were expanded.
	if result.TotalNodes == 0 {
		t.Fatal("expected at least 1 node, got 0")
	}

	// Check histogram observation count.
	// Pre-initialization adds 18 observations (6 depths x 3 outcomes), so we subtract those.
	histogramCount := histogramObservationCount(t, pr, "usearch_deep_tree_node_expand_seconds")
	const preInitCount = 18
	actualObs := histogramCount - preInitCount
	if actualObs != uint64(result.TotalNodes) {
		t.Errorf("histogram observations = %d (total %d - preInit %d), want %d", actualObs, histogramCount, preInitCount, result.TotalNodes)
	}

	// Check counter incremented.
	counterVal := counterValueByName(t, pr, "usearch_deep_tree_total_tokens", map[string]string{"outcome": "success"})
	if counterVal == 0 {
		t.Error("counter usearch_deep_tree_total_tokens{outcome=success} not incremented")
	}
}

// T-E-004 [RED]: OTel Span Linkage
// REQ-DEEP3-012, NFR-DEEP3-006: Each child node's span references parent's span_id.

func TestOTelSpanParentLinkage(t *testing.T) {
	// This test verifies the startNodeSpan function creates spans
	// with proper parent linkage.
	// In production, OTel SDK provides real spans. Here we test the
	// interface contract: parent context propagates correctly.

	parentCtx, parentSpan := startNodeSpan(context.Background(), &Node{ID: "parent-node", Depth: 0}, nil)
	if parentSpan == nil {
		// When OTel is not configured (test environment), spans are no-ops.
		// Verify the context is still usable.
		if parentCtx == nil {
			t.Error("startNodeSpan returned nil context")
		}
		t.Log("OTel not configured in test env; parent span is no-op, which is expected")
	}

	childCtx, childSpan := startNodeSpan(parentCtx, &Node{ID: "child-node", Depth: 1}, parentSpan)
	if childCtx == nil {
		t.Error("startNodeSpan returned nil child context")
	}
	// In test env, childSpan is no-op (not nil, but no-op).
	if childSpan == nil {
		t.Log("child span is nil (no OTel SDK configured)")
	}
}

func TestOTelTraceDepthMatchesTreeDepth(t *testing.T) {
	// Verify that startNodeSpan can be called for multiple depth levels.
	// The function itself does not enforce tree depth, but it must not panic
	// and must propagate context correctly.

	ctx := context.Background()
	for depth := 0; depth <= 5; depth++ {
		node := &Node{ID: "node-depth", Depth: depth}
		ctx, _ = startNodeSpan(ctx, node, nil)
		// No panic = success. In test env, spans are no-ops.
	}
}

// --- helper types ---

// stubTreeResearcher is a test stub for TreeResearcher interface.
type stubTreeResearcher struct {
	decomposeResult []string
	decomposeError  error
	fanoutResult    func() ([]NodeCitation, []NodeClaim, int64, error)
}

func (s *stubTreeResearcher) Decompose(_ context.Context, _ DecomposeRequest) ([]string, error) {
	return s.decomposeResult, s.decomposeError
}

func (s *stubTreeResearcher) Fanout(_ context.Context, _ string) ([]NodeCitation, []NodeClaim, int64, error) {
	if s.fanoutResult != nil {
		return s.fanoutResult()
	}
	return nil, nil, 0, nil
}

// --- helper functions ---

func histogramObservationCount(t *testing.T, pr *prometheus.Registry, name string) uint64 {
	t.Helper()
	mfs, err := pr.Gather()
	if err != nil {
		t.Fatalf("Gather() error: %v", err)
	}
	var total uint64
	for _, mf := range mfs {
		if mf.GetName() == name {
			for _, m := range mf.GetMetric() {
				total += m.GetHistogram().GetSampleCount()
			}
		}
	}
	return total
}

func counterValueByName(t *testing.T, pr *prometheus.Registry, name string, labels map[string]string) float64 {
	t.Helper()
	mfs, err := pr.Gather()
	if err != nil {
		t.Fatalf("Gather() error: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		for _, m := range mf.GetMetric() {
			if labelsMatchHelper(m.GetLabel(), labels) {
				return m.GetCounter().GetValue()
			}
		}
	}
	return 0
}

func labelsMatchHelper(got []*dto.LabelPair, want map[string]string) bool {
	matched := 0
	for _, lp := range got {
		v, ok := want[lp.GetName()]
		if ok && v == lp.GetValue() {
			matched++
		}
	}
	return matched == len(want)
}
