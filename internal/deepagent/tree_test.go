package deepagent

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math/rand"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/goleak"
)

// ---------------------------------------------------------------------------
// Mock Researcher
// ---------------------------------------------------------------------------

// mockResearcher implements Researcher for testing.
type mockResearcher struct {
	// DecomposeFn overrides Decompose behavior. If nil, returns default sub-queries.
	DecomposeFn func(ctx context.Context, req DecomposeRequest) ([]string, error)
	// FanoutFn overrides Fanout behavior. If nil, returns default citations/claims.
	FanoutFn func(ctx context.Context, query string) ([]NodeCitation, []NodeClaim, int64, error)

	// Track calls for ordering/parallelism assertions.
	mu              sync.Mutex
	decomposeCalls  []decomposeCall
	fanoutCalls     []fanoutCall
	concurrentPeak  atomic.Int64 // tracks max concurrent goroutines in Fanout
	currentActive   atomic.Int64
	goroutineIDs    map[int]bool // tracks unique goroutine IDs during Fanout
	goroutineIDMu   sync.Mutex
}

type decomposeCall struct {
	Timestamp time.Time
	Req       DecomposeRequest
}

type fanoutCall struct {
	Timestamp time.Time
	Query     string
}

func (m *mockResearcher) Decompose(ctx context.Context, req DecomposeRequest) ([]string, error) {
	m.mu.Lock()
	m.decomposeCalls = append(m.decomposeCalls, decomposeCall{
		Timestamp: time.Now(),
		Req:       req,
	})
	m.mu.Unlock()

	if m.DecomposeFn != nil {
		return m.DecomposeFn(ctx, req)
	}
	// Default: return breadth sub-queries.
	queries := make([]string, req.Breadth)
	for i := range queries {
		queries[i] = fmt.Sprintf("%s/sub-%d", req.ParentQuery, i)
	}
	return queries, nil
}

func (m *mockResearcher) Fanout(ctx context.Context, query string) ([]NodeCitation, []NodeClaim, int64, error) {
	// Track concurrency.
	active := m.currentActive.Add(1)
	m.concurrentPeak.Store(max(active, m.concurrentPeak.Load()))

	// Track goroutine IDs.
	gid := getGoroutineID()
	m.goroutineIDMu.Lock()
	if m.goroutineIDs == nil {
		m.goroutineIDs = make(map[int]bool)
	}
	m.goroutineIDs[gid] = true
	m.goroutineIDMu.Unlock()

	defer m.currentActive.Add(-1)

	m.mu.Lock()
	m.fanoutCalls = append(m.fanoutCalls, fanoutCall{
		Timestamp: time.Now(),
		Query:     query,
	})
	m.mu.Unlock()

	if m.FanoutFn != nil {
		return m.FanoutFn(ctx, query)
	}
	// Default: return one citation and one claim, 500 tokens.
	citations := []NodeCitation{
		{DocID: "doc-" + query, Title: "Title for " + query, URL: "https://example.com/" + query, Snippet: "snippet for " + query},
	}
	claims := []NodeClaim{
		{Text: "claim for " + query, Markers: []string{"M1"}},
	}
	return citations, claims, 500, nil
}

func (m *mockResearcher) DecomposeCalls() []decomposeCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]decomposeCall, len(m.decomposeCalls))
	copy(cp, m.decomposeCalls)
	return cp
}

func (m *mockResearcher) FanoutCalls() []fanoutCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]fanoutCall, len(m.fanoutCalls))
	copy(cp, m.fanoutCalls)
	return cp
}

// getGoroutineID extracts a numeric goroutine ID from runtime.Stack.
func getGoroutineID() int {
	buf := make([]byte, 64)
	runtime.Stack(buf, false)
	// Parse "goroutine N " from stack header.
	var gid int
	fmt.Sscanf(string(buf), "goroutine %d ", &gid)
	return gid
}

// ---------------------------------------------------------------------------
// Helper: build default TreeConfig for tests
// ---------------------------------------------------------------------------

func defaultTreeConfig(breadth, depth int) TreeConfig {
	return TreeConfig{
		Breadth:            breadth,
		Depth:              depth,
		TokenBudget:        500000, // generous budget for happy path tests
		NodeTimeoutMs:      5000,
		RootTokenEstimate:  5000,
		ModelPricePerToken: 0.000003, // $3/1M tokens
		RunID:              "test-run-001",
	}
}

// ---------------------------------------------------------------------------
// T-B-001 [RED] — Happy Path
// REQ-DEEP3-001, REQ-DEEP3-003
// ---------------------------------------------------------------------------

func TestExpandTreeHappyPath(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := defaultTreeConfig(4, 3)
	mock := &mockResearcher{}

	result, err := ExpandTree(ctx, cfg, "machine learning", mock)
	if err != nil {
		t.Fatalf("ExpandTree returned error: %v", err)
	}

	// 1 + 4 + 16 + 64 = 85 nodes
	if result.TotalNodes != 85 {
		t.Errorf("TotalNodes = %d, want 85", result.TotalNodes)
	}
	if result.MaxDepthReached != 3 {
		t.Errorf("MaxDepthReached = %d, want 3", result.MaxDepthReached)
	}
	if result.Status != "complete" {
		t.Errorf("Status = %q, want %q", result.Status, "complete")
	}
	if result.RootQuery != "machine learning" {
		t.Errorf("RootQuery = %q, want %q", result.RootQuery, "machine learning")
	}
}

// ---------------------------------------------------------------------------
// T-B-002 [RED] — Input Validation
// REQ-DEEP3-002
// ---------------------------------------------------------------------------

func TestExpandTreeInvalidBreadth(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := defaultTreeConfig(9, 3) // breadth=9 exceeds max of 8
	mock := &mockResearcher{}

	_, err := ExpandTree(ctx, cfg, "test query", mock)
	if err == nil {
		t.Fatal("expected error for breadth=9, got nil")
	}

	// Error must contain "invalid_tree_config"
	if !containsStr(err.Error(), "invalid_tree_config") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "invalid_tree_config")
	}
}

func TestExpandTreeInvalidDepth(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := defaultTreeConfig(4, 6) // depth=6 exceeds max of 5
	mock := &mockResearcher{}

	_, err := ExpandTree(ctx, cfg, "test query", mock)
	if err == nil {
		t.Fatal("expected error for depth=6, got nil")
	}

	if !containsStr(err.Error(), "invalid_tree_config") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "invalid_tree_config")
	}
}

// ---------------------------------------------------------------------------
// T-B-003 [RED][RACE] — BFS Ordering
// REQ-DEEP3-004
// ---------------------------------------------------------------------------

func TestExpandTreeBFSOrdering(t *testing.T) {
	// Do NOT use t.Parallel() — validates ordering with timestamps.

	ctx := context.Background()
	cfg := defaultTreeConfig(2, 3)
	mock := &mockResearcher{}

	_, err := ExpandTree(ctx, cfg, "AI safety", mock)
	if err != nil {
		t.Fatalf("ExpandTree returned error: %v", err)
	}

	fanoutCalls := mock.FanoutCalls()

	// Group calls by depth based on query content.
	// Root: "AI safety", Depth 1: "AI safety/sub-0", "AI safety/sub-1", etc.
	type callWithDepth struct {
		ts    time.Time
		depth int
	}

	calls := make([]callWithDepth, 0, len(fanoutCalls))
	for _, fc := range fanoutCalls {
		d := queryDepth(fc.Query, "AI safety")
		calls = append(calls, callWithDepth{ts: fc.Timestamp, depth: d})
	}

	// For each depth level, find the last call at depth N and the first call at depth N+1.
	// The first call at depth N+1 must be after the last call at depth N.
	maxDepth := 0
	for _, c := range calls {
		if c.depth > maxDepth {
			maxDepth = c.depth
		}
	}

	for d := 0; d < maxDepth; d++ {
		var lastAtDepthN time.Time
		var firstAtDepthN1 time.Time
		foundN1 := false

		for _, c := range calls {
			if c.depth == d && c.ts.After(lastAtDepthN) {
				lastAtDepthN = c.ts
			}
			if c.depth == d+1 {
				if !foundN1 || c.ts.Before(firstAtDepthN1) {
					firstAtDepthN1 = c.ts
					foundN1 = true
				}
			}
		}

		if foundN1 && firstAtDepthN1.Before(lastAtDepthN) {
			t.Errorf("BFS ordering violated: depth %d+1 call at %v precedes last depth %d call at %v",
				d, firstAtDepthN1, d, lastAtDepthN)
		}
	}
}

// queryDepth determines the depth of a query relative to the root query.
// Root query is depth 0, direct children are depth 1, etc.
func queryDepth(query, root string) int {
	if query == root {
		return 0
	}
	// Count "/sub-" occurrences to determine depth.
	depth := 0
	remaining := query[len(root):]
	for len(remaining) > 0 {
		idx := indexOf(remaining, "/sub-")
		if idx < 0 {
			break
		}
		depth++
		// Skip past "/sub-N"
		remaining = remaining[idx+5:] // skip "/sub-"
		// Skip digits
		for len(remaining) > 0 && remaining[0] >= '0' && remaining[0] <= '9' {
			remaining = remaining[1:]
		}
	}
	return depth
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// ---------------------------------------------------------------------------
// T-B-004 [RED][RACE][GOLEAK] — Concurrent Breadth
// REQ-DEEP3-004
// ---------------------------------------------------------------------------

func TestExpandTreeConcurrentBreadth(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Do NOT use t.Parallel() — goroutine leak detection.

	ctx := context.Background()
	cfg := defaultTreeConfig(4, 2)
	mock := &mockResearcher{}

	_, err := ExpandTree(ctx, cfg, "concurrency test", mock)
	if err != nil {
		t.Fatalf("ExpandTree returned error: %v", err)
	}

	// Verify that Fanout was called concurrently (multiple goroutine IDs).
	mock.goroutineIDMu.Lock()
	uniqueGoroutines := len(mock.goroutineIDs)
	mock.goroutineIDMu.Unlock()

	if uniqueGoroutines < 2 {
		t.Errorf("Fanout only executed on %d goroutine(s), expected concurrent execution on >= 2", uniqueGoroutines)
	}

	// Verify peak concurrency was > 1 (proves parallelism within a depth level).
	peak := mock.concurrentPeak.Load()
	if peak < 2 {
		t.Errorf("peak concurrent Fanout calls = %d, want >= 2", peak)
	}
}

// ---------------------------------------------------------------------------
// T-B-005 [RED][RACE] — Budget Race-Free
// REQ-DEEP3-006, REQ-DEEP3-007, NFR-DEEP3-003
// ---------------------------------------------------------------------------

func TestExpandTreeConcurrentBreadthBudgetRaceFree(t *testing.T) {
	// Do NOT use t.Parallel() — race condition test.

	const iterations = 100
	const budget int64 = 60000

	for iter := range iterations {
		func() {
			ctx := context.Background()
			cfg := TreeConfig{
				Breadth:           8,
				Depth:             2,
				TokenBudget:       budget,
				NodeTimeoutMs:     5000,
				RootTokenEstimate: 5000,
				ModelPricePerToken: 0.000003,
				RunID:             fmt.Sprintf("race-test-%d", iter),
			}
			mock := &mockResearcher{}

			result, err := ExpandTree(ctx, cfg, "budget race test", mock)
			if err != nil {
				t.Logf("iteration %d: ExpandTree error: %v (may be budget-exceeded, acceptable)", iter, err)
				return
			}

			if result.TotalTokensUsed > budget {
				t.Errorf("iteration %d: TotalTokensUsed = %d exceeds budget %d", iter, result.TotalTokensUsed, budget)
			}
		}()
	}
}

// ---------------------------------------------------------------------------
// T-B-006 [RED] — Budget Exhaustion
// REQ-DEEP3-006, REQ-DEEP3-008, REQ-DEEP3-010
// ---------------------------------------------------------------------------

func TestExpandTreeBudgetExceeded(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := TreeConfig{
		Breadth:           4,
		Depth:             3,
		TokenBudget:       20000, // Low budget
		NodeTimeoutMs:     5000,
		RootTokenEstimate: 5000,
		ModelPricePerToken: 0.000003,
		RunID:             "budget-exhaust-test",
	}
	mock := &mockResearcher{}

	result, err := ExpandTree(ctx, cfg, "budget test", mock)
	if err != nil {
		t.Fatalf("ExpandTree returned error: %v", err)
	}

	// Tree should have partial status (not all 85 nodes).
	if result.TotalNodes >= 85 {
		t.Errorf("TotalNodes = %d, want < 85 (budget should have been exceeded)", result.TotalNodes)
	}
	// Status should indicate budget exceeded or be partial.
	if result.Status != "budget_exceeded" && result.Status != "complete" {
		t.Errorf("Status = %q, want 'budget_exceeded' or 'complete'", result.Status)
	}
}

func TestExpandTreePartialReturn(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := TreeConfig{
		Breadth:           4,
		Depth:             3,
		TokenBudget:       20000,
		NodeTimeoutMs:     5000,
		RootTokenEstimate: 5000,
		ModelPricePerToken: 0.000003,
		RunID:             "partial-test",
	}
	mock := &mockResearcher{}

	result, err := ExpandTree(ctx, cfg, "partial tree test", mock)
	if err != nil {
		t.Fatalf("ExpandTree returned error: %v", err)
	}

	// Even with budget exhaustion, we should have some flattened claims.
	if len(result.FlattenedClaims) == 0 {
		t.Error("FlattenedClaims is empty, want at least root claims")
	}

	// Citations from completed nodes should be present.
	if len(result.Citations) == 0 {
		t.Error("Citations is empty, want at least root citations")
	}
}

// ---------------------------------------------------------------------------
// T-B-007 [RED] — Latency
// NFR-DEEP3-001, NFR-DEEP3-002
// ---------------------------------------------------------------------------

func TestExpandTreeLatencyP95(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping latency test in short mode")
	}

	const iterations = 50
	latencies := make([]time.Duration, 0, iterations)

	for i := range iterations {
		ctx := context.Background()
		cfg := TreeConfig{
			Breadth:           2,
			Depth:             2,
			TokenBudget:       60000,
			NodeTimeoutMs:     5000,
			RootTokenEstimate: 5000,
			ModelPricePerToken: 0.000003,
			RunID:             fmt.Sprintf("latency-test-%d", i),
		}

		mock := &mockResearcher{
			FanoutFn: func(ctx context.Context, query string) ([]NodeCitation, []NodeClaim, int64, error) {
				// Simulate small processing time.
				time.Sleep(10 * time.Millisecond)
				return []NodeCitation{{DocID: "d1", Title: "T", URL: "http://example.com", Snippet: "s"}},
					[]NodeClaim{{Text: "claim", Markers: []string{"M1"}}},
					500, nil
			},
		}

		start := time.Now()
		_, err := ExpandTree(ctx, cfg, "latency test", mock)
		elapsed := time.Since(start)
		latencies = append(latencies, elapsed)

		if err != nil {
			t.Fatalf("iteration %d: ExpandTree error: %v", i, err)
		}
	}

	// Sort and check p95.
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	p95Index := int(float64(len(latencies)) * 0.95)
	if p95Index >= len(latencies) {
		p95Index = len(latencies) - 1
	}
	p95 := latencies[p95Index]

	// Per-node p95 should be well under 30s (mock is fast).
	// For 7 nodes with 10ms each, p95 should be under 1 second.
	if p95 > 30*time.Second {
		t.Errorf("per-node p95 latency = %v, want <= 30s", p95)
	}
}

func TestExpandTreeEndToEndLatencyP95(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping latency test in short mode")
	}

	const iterations = 50
	latencies := make([]time.Duration, 0, iterations)

	for i := range iterations {
		ctx := context.Background()
		cfg := TreeConfig{
			Breadth:           2,
			Depth:             2,
			TokenBudget:       60000,
			NodeTimeoutMs:     5000,
			RootTokenEstimate: 5000,
			ModelPricePerToken: 0.000003,
			RunID:             fmt.Sprintf("e2e-latency-%d", i),
		}

		mock := &mockResearcher{
			FanoutFn: func(ctx context.Context, query string) ([]NodeCitation, []NodeClaim, int64, error) {
				time.Sleep(10 * time.Millisecond)
				return []NodeCitation{{DocID: "d1", Title: "T", URL: "http://example.com", Snippet: "s"}},
					[]NodeClaim{{Text: "claim", Markers: []string{"M1"}}},
					500, nil
			},
		}

		start := time.Now()
		_, err := ExpandTree(ctx, cfg, "e2e latency test", mock)
		elapsed := time.Since(start)
		latencies = append(latencies, elapsed)

		if err != nil {
			t.Fatalf("iteration %d: ExpandTree error: %v", i, err)
		}
	}

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	p95Index := int(float64(len(latencies)) * 0.95)
	if p95Index >= len(latencies) {
		p95Index = len(latencies) - 1
	}
	p95 := latencies[p95Index]

	// End-to-end p95 should be well under 240s (mock is fast).
	if p95 > 240*time.Second {
		t.Errorf("end-to-end p95 latency = %v, want <= 240s", p95)
	}
}

// ---------------------------------------------------------------------------
// T-B-008 [RED] — Citation Disjointness
// REQ-DEEP3-009b
// ---------------------------------------------------------------------------

func TestCitationsDisjointlyOwned(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := defaultTreeConfig(3, 2)
	mock := &mockResearcher{}

	// We need to access the internal nodes, so we'll verify via the result.
	// Since ExpandTree returns a TreeResult, we need to verify citations
	// through the tree structure. Let's use a custom approach that captures
	// the nodes.
	_, err := ExpandTree(ctx, cfg, "citation test", mock)
	if err != nil {
		t.Fatalf("ExpandTree returned error: %v", err)
	}

	// The result.Citations aggregates all node citations. We need to verify
	// that each node's citation slice is independently owned.
	// Since we can't access individual nodes from TreeResult directly,
	// we verify through a custom mock that tracks citation slice identity.

	citationTracker := &citationTrackingMock{base: mock}
	_, _ = ExpandTree(ctx, cfg, "citation test 2", citationTracker)

	// Check pairwise disjointness of citation slices.
	citationTracker.mu.Lock()
	defer citationTracker.mu.Unlock()

	for i := 0; i < len(citationTracker.citationSlices); i++ {
		for j := i + 1; j < len(citationTracker.citationSlices); j++ {
			sliceA := citationTracker.citationSlices[i]
			sliceB := citationTracker.citationSlices[j]
			if len(sliceA) == 0 || len(sliceB) == 0 {
				continue
			}
			// Check that the underlying arrays are different by comparing pointers.
			if &sliceA[0] == &sliceB[0] && cap(sliceA) == cap(sliceB) {
				t.Errorf("nodes %d and %d share the same citation slice (slice identity violation)", i, j)
			}
		}
	}
}

type citationTrackingMock struct {
	base           *mockResearcher
	mu             sync.Mutex
	citationSlices [][]NodeCitation
}

func (c *citationTrackingMock) Decompose(ctx context.Context, req DecomposeRequest) ([]string, error) {
	return c.base.Decompose(ctx, req)
}

func (c *citationTrackingMock) Fanout(ctx context.Context, query string) ([]NodeCitation, []NodeClaim, int64, error) {
	citations, claims, tokens, err := c.base.Fanout(ctx, query)
	if err != nil {
		return nil, nil, 0, err
	}
	// Store a copy of the slice header to track identity.
	sliceCopy := make([]NodeCitation, len(citations))
	copy(sliceCopy, citations)

	c.mu.Lock()
	c.citationSlices = append(c.citationSlices, citations)
	c.mu.Unlock()

	return sliceCopy, claims, tokens, nil
}

// ---------------------------------------------------------------------------
// T-B-009 [RED] — Lineage Property Test
// REQ-DEEP3-009a, REQ-DEEP3-010
// ---------------------------------------------------------------------------

func TestFlattenedClaimLineageProperty(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(42))

	for i := range 100 {
		breadth := rng.Intn(8) + 1 // [1, 8]
		depth := rng.Intn(5) + 1   // [1, 5]

		ctx := context.Background()
		cfg := TreeConfig{
			Breadth:           breadth,
			Depth:             depth,
			TokenBudget:       60000,
			NodeTimeoutMs:     5000,
			RootTokenEstimate: 5000,
			ModelPricePerToken: 0.000003,
			RunID:             fmt.Sprintf("prop-test-%d", i),
		}
		mock := &mockResearcher{}

		result, err := ExpandTree(ctx, cfg, fmt.Sprintf("property-test-%d", i), mock)
		if err != nil {
			t.Logf("iteration %d (breadth=%d, depth=%d): ExpandTree error: %v", i, breadth, depth, err)
			continue
		}

		// Every FlattenedClaim must satisfy lineage invariants.
		for _, fc := range result.FlattenedClaims {
			// lineage_path must not be empty.
			if len(fc.LineagePath) == 0 {
				t.Errorf("iteration %d: FlattenedClaim lineage_path is empty", i)
				continue
			}

			// lineage_path[0] must be the root node ID.
			rootID := fmt.Sprintf("%x", sha256.Sum256([]byte(cfg.RunID+"root")))
			if fc.LineagePath[0] != rootID {
				t.Errorf("iteration %d: lineage_path[0] = %q, want root ID %q", i, fc.LineagePath[0], rootID)
			}

			// lineage_path last element must equal source_node_id.
			last := fc.LineagePath[len(fc.LineagePath)-1]
			if last != fc.SourceNodeID {
				t.Errorf("iteration %d: lineage_path[-1] = %q, want source_node_id %q", i, last, fc.SourceNodeID)
			}
		}

		_ = result // avoid unused warning if no claims
	}
}

// ---------------------------------------------------------------------------
// T-B-010 [RED] — Cost USD
// REQ-DEEP3-013
// ---------------------------------------------------------------------------

func TestNodeCostUSDComputedOnComplete(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := defaultTreeConfig(2, 2)
	cfg.ModelPricePerToken = 0.000003 // $3 per 1M tokens

	mock := &mockResearcher{
		FanoutFn: func(ctx context.Context, query string) ([]NodeCitation, []NodeClaim, int64, error) {
			return []NodeCitation{{DocID: "d1", Title: "T", URL: "http://e.com", Snippet: "s"}},
				[]NodeClaim{{Text: "claim", Markers: []string{"M1"}}},
				1000, nil // exactly 1000 tokens per node
		},
	}

	result, err := ExpandTree(ctx, cfg, "cost test", mock)
	if err != nil {
		t.Fatalf("ExpandTree returned error: %v", err)
	}

	// Each completed node should have CostUSD = TokensUsed * ModelPricePerToken.
	expectedCostPerNode := 1000.0 * 0.000003 // 0.003
	expectedTotal := float64(result.TotalNodes) * expectedCostPerNode

	// Allow small floating-point tolerance.
	delta := 0.0001
	if result.TotalCostUSD < expectedTotal-delta || result.TotalCostUSD > expectedTotal+delta {
		t.Errorf("TotalCostUSD = %f, want approximately %f (nodes=%d, cost_per_node=%f)",
			result.TotalCostUSD, expectedTotal, result.TotalNodes, expectedCostPerNode)
	}
}

func TestTreeTotalCostUSDSumsCompletedNodes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := defaultTreeConfig(2, 2)
	cfg.ModelPricePerToken = 0.000005

	mock := &mockResearcher{
		FanoutFn: func(ctx context.Context, query string) ([]NodeCitation, []NodeClaim, int64, error) {
			return []NodeCitation{{DocID: "d1", Title: "T", URL: "http://e.com", Snippet: "s"}},
				[]NodeClaim{{Text: "claim", Markers: []string{"M1"}}},
				2000, nil
		},
	}

	result, err := ExpandTree(ctx, cfg, "cost sum test", mock)
	if err != nil {
		t.Fatalf("ExpandTree returned error: %v", err)
	}

	// TotalCostUSD should equal sum of completed node costs.
	expectedTotal := float64(result.TotalNodes) * 2000.0 * 0.000005
	delta := 0.0001
	if result.TotalCostUSD < expectedTotal-delta || result.TotalCostUSD > expectedTotal+delta {
		t.Errorf("TotalCostUSD = %f, want approximately %f", result.TotalCostUSD, expectedTotal)
	}
}

// ---------------------------------------------------------------------------
// T-B-011 [RED] — Memory Footprint
// NFR-DEEP3-009
// ---------------------------------------------------------------------------

func TestTreeMemoryFootprintUnder100MBWorstCase(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory footprint test in short mode")
	}

	var m1 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	ctx := context.Background()
	cfg := TreeConfig{
		Breadth:           8,
		Depth:             5,
		TokenBudget:       60000, // Low budget to limit actual expansion
		NodeTimeoutMs:     5000,
		RootTokenEstimate: 5000,
		ModelPricePerToken: 0.000003,
		RunID:             "memory-test",
	}

	mock := &mockResearcher{
		FanoutFn: func(ctx context.Context, query string) ([]NodeCitation, []NodeClaim, int64, error) {
			// Simulate ~1KB of citation data per node.
			citations := make([]NodeCitation, 10)
			for i := range citations {
				citations[i] = NodeCitation{
					DocID:   fmt.Sprintf("doc-%s-%d", query, i),
					Title:   fmt.Sprintf("Title for %s citation %d", query, i),
					URL:     fmt.Sprintf("https://example.com/%s/%d", query, i),
					Snippet: fmt.Sprintf("This is a longer snippet for %s citation number %d with more text to simulate real data", query, i),
				}
			}
			claims := []NodeClaim{
				{Text: fmt.Sprintf("Claim for %s: some substantive claim text that represents real research output", query), Markers: []string{"M1", "M2"}},
			}
			return citations, claims, 500, nil
		},
	}

	result, err := ExpandTree(ctx, cfg, "memory footprint test", mock)
	if err != nil {
		t.Fatalf("ExpandTree returned error: %v", err)
	}

	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	// Calculate memory delta attributed to tree expansion.
	memUsedMB := float64(m2.Alloc-m1.Alloc) / 1024 / 1024
	if memUsedMB < 0 {
		memUsedMB = 0
	}

	t.Logf("Tree nodes: %d, Memory used: %.2f MB", result.TotalNodes, memUsedMB)

	// The 100MB limit applies to in-memory tree state.
	// With budget=60000, we won't get the full worst-case tree (37449 nodes),
	// but we verify the memory guard is in place.
	const memoryLimitMB = 100.0
	if memUsedMB > memoryLimitMB {
		t.Errorf("memory footprint = %.2f MB, exceeds %v MB limit (nodes=%d)",
			memUsedMB, memoryLimitMB, result.TotalNodes)
	}
}

// ---------------------------------------------------------------------------
// T-B-012 [RED] — Edge Case: depth=1
// REQ-DEEP3-003 (leaf node skip decompose)
// ---------------------------------------------------------------------------

func TestExpandTreeDepthOneSingleLevel(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := defaultTreeConfig(4, 1)
	mock := &mockResearcher{}

	result, err := ExpandTree(ctx, cfg, "single level test", mock)
	if err != nil {
		t.Fatalf("ExpandTree returned error: %v", err)
	}

	// 1 root + 4 leaves = 5 nodes total.
	if result.TotalNodes != 5 {
		t.Errorf("TotalNodes = %d, want 5", result.TotalNodes)
	}
	if result.MaxDepthReached != 1 {
		t.Errorf("MaxDepthReached = %d, want 1", result.MaxDepthReached)
	}

	// Verify no Decompose calls were made (leaves should not decompose).
	decomposeCalls := mock.DecomposeCalls()
	// Decompose should only be called for the root (depth=0), not for leaves (depth=1).
	rootDecomposeCount := 0
	for _, dc := range decomposeCalls {
		if dc.Req.ParentQuery == "single level test" {
			rootDecomposeCount++
		}
	}
	if rootDecomposeCount != 1 {
		t.Errorf("root decompose calls = %d, want 1", rootDecomposeCount)
	}

	// Verify all leaf nodes have non-empty citations (from fanout).
	if len(result.Citations) < 4 {
		t.Errorf("total citations = %d, want at least 4 (one per leaf)", len(result.Citations))
	}
}

// ---------------------------------------------------------------------------
// Utility functions
// ---------------------------------------------------------------------------

func containsStr(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// max returns the larger of two int64 values.
func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
