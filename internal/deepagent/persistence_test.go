package deepagent

// @MX:SPEC: SPEC-DEEP-003 Phase D — Persistence + Crash Recovery tests

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"go.uber.org/goleak"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// sampleTreeResult builds a TreeResult for testing.
func sampleTreeResult() *TreeResult {
	return &TreeResult{
		RootQuery:       "machine learning",
		TotalNodes:      3,
		MaxDepthReached: 1,
		Status:          "complete",
		FlattenedClaims: []FlattenedClaim{
			{
				Text:         "ML is a subset of AI",
				Markers:      []string{"M1"},
				LineagePath:  []string{"root-id", "child-0-id"},
				SourceNodeID: "child-0-id",
			},
		},
		Citations: []NodeCitation{
			{DocID: "doc-1", Title: "ML Intro", URL: "https://example.com/ml", Snippet: "Machine learning intro"},
		},
		TotalTokensUsed:     1500,
		TotalReservedTokens: 3000,
		TotalCostUSD:        0.0045,
	}
}

// sampleNodes builds a slice of Nodes for testing.
func sampleNodes() []*Node {
	return []*Node{
		{
			ID:             "root-id",
			Depth:          0,
			ParentID:       "",
			BreadthIndex:   0,
			Query:          "machine learning",
			Status:         NodeStatusComplete,
			TokensUsed:     500,
			ReservedTokens: 500,
			CostUSD:        0.0015,
			Citations:      []NodeCitation{{DocID: "doc-r", Title: "Root", URL: "https://e.com/r", Snippet: "root snippet"}},
			Claims:         []NodeClaim{{Text: "root claim", Markers: []string{"M1"}}},
		},
		{
			ID:             "child-0-id",
			Depth:          1,
			ParentID:       "root-id",
			BreadthIndex:   0,
			Query:          "machine learning/sub-0",
			Status:         NodeStatusComplete,
			TokensUsed:     500,
			ReservedTokens: 2500,
			CostUSD:        0.0015,
			Citations:      []NodeCitation{{DocID: "doc-c0", Title: "Child 0", URL: "https://e.com/c0", Snippet: "c0 snippet"}},
			Claims:         []NodeClaim{{Text: "child claim 0", Markers: []string{"M2"}}},
		},
		{
			ID:             "child-1-id",
			Depth:          1,
			ParentID:       "root-id",
			BreadthIndex:   1,
			Query:          "machine learning/sub-1",
			Status:         NodeStatusComplete,
			TokensUsed:     500,
			ReservedTokens: 2500,
			CostUSD:        0.0015,
			Citations:      []NodeCitation{{DocID: "doc-c1", Title: "Child 1", URL: "https://e.com/c1", Snippet: "c1 snippet"}},
			Claims:         []NodeClaim{{Text: "child claim 1", Markers: []string{"M3"}}},
		},
	}
}

// ---------------------------------------------------------------------------
// T-D-001 [RED] — Atomic Flush
// REQ-DEEP3-011a
// ---------------------------------------------------------------------------

func TestPersistenceAtomicFlushOnDepthJoin(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	p := NewTreePersistence()
	runID := "test-flush-depth"

	nodes := sampleNodes()

	// OnDepthLevelJoin should atomically write tree.json
	err := p.OnDepthLevelJoin(nodes, dir, runID)
	if err != nil {
		t.Fatalf("OnDepthLevelJoin returned error: %v", err)
	}

	// Verify tree.json exists (not .tmp)
	finalPath := filepath.Join(dir, runID, "tree.json")
	if _, err := os.Stat(finalPath); os.IsNotExist(err) {
		t.Errorf("tree.json not found at %s", finalPath)
	}

	// Verify no .tmp file remains
	tmpPath := filepath.Join(dir, runID, "tree.json.tmp")
	if _, err := os.Stat(tmpPath); err == nil {
		t.Error("tree.json.tmp still exists, atomic rename did not complete")
	}

	// Verify the file content is valid JSON with schema_version
	data, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatalf("failed to read tree.json: %v", err)
	}

	var wrapper persistenceWrapper
	if err := json.Unmarshal(data, &wrapper); err != nil {
		t.Fatalf("failed to unmarshal tree.json: %v", err)
	}

	if wrapper.SchemaVersion != "1.0" {
		t.Errorf("schema_version = %q, want %q", wrapper.SchemaVersion, "1.0")
	}

	// Verify reconstructed tree data matches expectations from sampleNodes
	if wrapper.Tree.RootQuery != "machine learning" {
		t.Errorf("root_query = %q, want %q", wrapper.Tree.RootQuery, "machine learning")
	}
	if wrapper.Tree.TotalNodes != 3 {
		t.Errorf("total_nodes = %d, want 3", wrapper.Tree.TotalNodes)
	}
	if wrapper.Tree.MaxDepthReached != 1 {
		t.Errorf("max_depth_reached = %d, want 1", wrapper.Tree.MaxDepthReached)
	}
	if wrapper.Tree.TotalTokensUsed != 1500 {
		t.Errorf("total_tokens_used = %d, want 1500", wrapper.Tree.TotalTokensUsed)
	}
	// TotalCostUSD: 3 nodes * 0.0015 = 0.0045 (with floating-point tolerance)
	delta := 0.0001
	if wrapper.Tree.TotalCostUSD < 0.0045-delta || wrapper.Tree.TotalCostUSD > 0.0045+delta {
		t.Errorf("total_cost_usd = %f, want approximately 0.0045", wrapper.Tree.TotalCostUSD)
	}
	// Claims from all 3 nodes
	if len(wrapper.Tree.FlattenedClaims) != 3 {
		t.Errorf("len(flattened_claims) = %d, want 3", len(wrapper.Tree.FlattenedClaims))
	}
	// Citations from completed nodes
	if len(wrapper.Tree.Citations) != 3 {
		t.Errorf("len(citations) = %d, want 3", len(wrapper.Tree.Citations))
	}
}

func TestPersistencePerNodeTransitionFlush(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	p := NewTreePersistence()
	runID := "test-flush-transition"

	transitions := []NodeStatus{
		NodeStatusExpanding,
		NodeStatusComplete,
		NodeStatusFailed,
		NodeStatusBudgetExceeded,
	}

	for i, status := range transitions {
		node := &Node{
			ID:        fmt.Sprintf("node-%d", i),
			Depth:     0,
			ParentID:  "",
			Query:     fmt.Sprintf("query-%d", i),
			Status:    status,
			TokensUsed: 100,
		}

		err := p.OnNodeTransition(node, dir, runID)
		if err != nil {
			t.Fatalf("OnNodeTransition(%s) returned error: %v", status, err)
		}

		// Verify tree.json was written
		finalPath := filepath.Join(dir, runID, "tree.json")
		if _, err := os.Stat(finalPath); os.IsNotExist(err) {
			t.Errorf("tree.json not found after transition to %s", status)
		}

		// Verify no .tmp remains
		tmpPath := filepath.Join(dir, runID, "tree.json.tmp")
		if _, err := os.Stat(tmpPath); err == nil {
			t.Errorf("tree.json.tmp still exists after transition to %s", status)
		}
	}
}

// ---------------------------------------------------------------------------
// T-D-002 [RED] — JSON Round-Trip
// REQ-DEEP3-010, REQ-DEEP3-011a, REQ-DEEP3-013
// ---------------------------------------------------------------------------

func TestPersistenceJSONRoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	p := NewTreePersistence()
	runID := "test-roundtrip"

	original := sampleTreeResult()

	// Write
	err := p.AtomicFlush(dir, runID, original)
	if err != nil {
		t.Fatalf("AtomicFlush returned error: %v", err)
	}

	// Read back
	loaded, err := p.LoadTree(dir, runID)
	if err != nil {
		t.Fatalf("LoadTree returned error: %v", err)
	}

	// Verify struct equality for all fields
	if loaded.RootQuery != original.RootQuery {
		t.Errorf("RootQuery = %q, want %q", loaded.RootQuery, original.RootQuery)
	}
	if loaded.TotalNodes != original.TotalNodes {
		t.Errorf("TotalNodes = %d, want %d", loaded.TotalNodes, original.TotalNodes)
	}
	if loaded.MaxDepthReached != original.MaxDepthReached {
		t.Errorf("MaxDepthReached = %d, want %d", loaded.MaxDepthReached, original.MaxDepthReached)
	}
	if loaded.Status != original.Status {
		t.Errorf("Status = %q, want %q", loaded.Status, original.Status)
	}
	if loaded.TotalTokensUsed != original.TotalTokensUsed {
		t.Errorf("TotalTokensUsed = %d, want %d", loaded.TotalTokensUsed, original.TotalTokensUsed)
	}
	if loaded.TotalReservedTokens != original.TotalReservedTokens {
		t.Errorf("TotalReservedTokens = %d, want %d", loaded.TotalReservedTokens, original.TotalReservedTokens)
	}
	if loaded.TotalCostUSD != original.TotalCostUSD {
		t.Errorf("TotalCostUSD = %f, want %f", loaded.TotalCostUSD, original.TotalCostUSD)
	}
	if len(loaded.FlattenedClaims) != len(original.FlattenedClaims) {
		t.Errorf("len(FlattenedClaims) = %d, want %d", len(loaded.FlattenedClaims), len(original.FlattenedClaims))
	}
	if len(loaded.Citations) != len(original.Citations) {
		t.Errorf("len(Citations) = %d, want %d", len(loaded.Citations), len(original.Citations))
	}

	// Verify schema version in raw file
	finalPath := filepath.Join(dir, runID, "tree.json")
	data, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatalf("failed to read tree.json: %v", err)
	}
	if !bytes.Contains(data, []byte(`"schema_version":"1.0"`)) && !bytes.Contains(data, []byte(`"schema_version": "1.0"`)) {
		t.Error("tree.json does not contain schema_version field")
	}
}

// ---------------------------------------------------------------------------
// T-D-003 [RED] — Postgres Insert
// REQ-DEEP3-011a, REQ-DEEP3-013
// ---------------------------------------------------------------------------

func TestPersistencePostgresInsert(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	p := NewTreePersistence()
	tree := sampleTreeResult()
	runID := "pg-insert-test"

	// Expect INSERT with the required columns
	mock.ExpectExec(`INSERT INTO deep_runs`).
		WithArgs(
			runID,                       // run_id
			tree.RootQuery,              // query
			4,                           // breadth (default)
			3,                           // depth (default)
			tree.TotalNodes,             // total_nodes
			tree.TotalTokensUsed,        // total_tokens
			tree.TotalCostUSD,           // total_cost_usd
			tree.Status,                 // status
			sqlmock.AnyArg(),            // started_at
			sqlmock.AnyArg(),            // completed_at
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = p.InsertPostgresSummary(db, tree, runID)
	if err != nil {
		t.Fatalf("InsertPostgresSummary returned error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %v", err)
	}
}

// ---------------------------------------------------------------------------
// T-D-004 [RED] — Crash Finalization
// REQ-DEEP3-011a, NFR-DEEP3-008
// ---------------------------------------------------------------------------

func TestPersistenceCrashFinalizesPostgresRow(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	p := NewTreePersistence()
	tree := sampleTreeResult()
	tree.Status = "partial" // Simulate incomplete tree
	runID := "crash-test"

	// Expect UPDATE to finalize the crashed row
	mock.ExpectExec(`UPDATE deep_runs`).
		WithArgs(
			"failed",                    // status should be "failed"
			sqlmock.AnyArg(),            // completed_at (within +-5s of now)
			runID,                       // run_id
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	crashTime := time.Now()
	err = p.FinalizeCrash(db, tree, runID, crashTime)
	if err != nil {
		t.Fatalf("FinalizeCrash returned error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %v", err)
	}
}

// ---------------------------------------------------------------------------
// T-D-005 [RED] — Reload Reclassify
// REQ-DEEP3-011b, NFR-DEEP3-008
// ---------------------------------------------------------------------------

func TestPersistenceReclassifyOnReload(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	p := NewTreePersistence()
	runID := "reclassify-test"

	// Write a wrapped JSON with nodes in Pending/Expanding state.
	persisted := persistenceWrapper{
		SchemaVersion: "1.0",
		Tree: TreeResult{
			RootQuery:       "test reclassify",
			TotalNodes:      3,
			MaxDepthReached: 1,
			Status:          "partial",
			TotalTokensUsed: 500,
			TotalCostUSD:    0.0015,
		},
		Nodes: []*Node{
			{ID: "n0", Status: NodeStatusComplete, Query: "q0", Depth: 0},
			{ID: "n1", Status: NodeStatusPending, Query: "q1", Depth: 1},
			{ID: "n2", Status: NodeStatusExpanding, Query: "q2", Depth: 1},
		},
	}

	runDir := filepath.Join(dir, runID)
	if err := os.MkdirAll(runDir, 0755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(persisted)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "tree.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	// Compute original disk hash
	originalHash := fileHash(filepath.Join(runDir, "tree.json"))

	// Load tree with nodes for reclassify verification.
	tree, nodes, err := p.LoadTreeWithNodes(dir, runID)
	if err != nil {
		t.Fatalf("LoadTreeWithNodes returned error: %v", err)
	}

	// Reclassify Pending/Expanding nodes to Failed in-memory.
	reclassifiedNodes := p.ReloadReclassifyNodes(nodes)

	// Verify node-level reclassification.
	for _, n := range reclassifiedNodes {
		if n.ID == "n1" || n.ID == "n2" {
			if n.Status != NodeStatusFailed {
				t.Errorf("node %s status = %q, want %q (reclassified from Pending/Expanding)", n.ID, n.Status, NodeStatusFailed)
			}
		} else {
			// Complete node should remain unchanged.
			if n.Status != NodeStatusComplete {
				t.Errorf("node %s status = %q, want %q (should not be reclassified)", n.ID, n.Status, NodeStatusComplete)
			}
		}
	}

	// Verify the TreeResult pointer was returned correctly.
	if tree == nil {
		t.Fatal("LoadTreeWithNodes returned nil TreeResult")
	}

	// Verify disk hash unchanged (reclassify is in-memory only).
	newHash := fileHash(filepath.Join(runDir, "tree.json"))
	if originalHash != newHash {
		t.Error("tree.json was modified on disk after ReloadReclassifyNodes — should be in-memory only")
	}
}

func fileHash(path string) string {
	data, _ := os.ReadFile(path)
	if data == nil {
		return ""
	}
	return fmt.Sprintf("%x", data)
}

// ---------------------------------------------------------------------------
// T-D-006 [RED] — Reload Read-Only
// REQ-DEEP3-011b
// ---------------------------------------------------------------------------

func TestPersistenceReloadTreeRejectsExpandAttempt(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	p := NewTreePersistence()
	runID := "reload-readonly-test"

	tree := sampleTreeResult()

	// Write the tree
	err := p.AtomicFlush(dir, runID, tree)
	if err != nil {
		t.Fatalf("AtomicFlush returned error: %v", err)
	}

	// Load the tree — it should be marked as reloaded/read-only
	loaded, err := p.LoadTree(dir, runID)
	if err != nil {
		t.Fatalf("LoadTree returned error: %v", err)
	}

	// Attempting to expand the reloaded tree should return an error
	// and no goroutine should be spawned.
	ctx := newTestContext()
	err = p.ExpandReloadedTree(ctx, loaded)
	if err == nil {
		t.Error("ExpandReloadedTree returned nil error, expected rejection for read-only tree")
	}

	// Verify the error message is descriptive
	if err != nil && !containsStr(err.Error(), "read-only") && !containsStr(err.Error(), "reload") {
		t.Errorf("error = %q, want to mention read-only or reload", err.Error())
	}
}

// newTestContext returns a context for testing.
func newTestContext() context.Context {
	return context.Background()
}

// ---------------------------------------------------------------------------
// T-D-007 [RED] — Size Bound
// NFR-DEEP3-007
// ---------------------------------------------------------------------------

func TestPersistenceJSONGzipSizeBound(t *testing.T) {
	t.Parallel()

	// Generate an 85-node tree (worst case: breadth=4, depth=3).
	tree := &TreeResult{
		RootQuery:       "size bound test with a reasonably long query string",
		TotalNodes:      85,
		MaxDepthReached: 3,
		Status:          "complete",
		TotalTokensUsed: 42500,
		TotalCostUSD:    0.1275,
		FlattenedClaims: make([]FlattenedClaim, 85),
		Citations:       make([]NodeCitation, 85),
	}

	// Fill with realistic data
	for i := range 85 {
		tree.FlattenedClaims[i] = FlattenedClaim{
			Text:         fmt.Sprintf("Claim %d: A substantive research claim about topic %d with sufficient detail to simulate real data", i, i),
			Markers:      []string{"M1", "M2"},
			LineagePath:  []string{fmt.Sprintf("node-root"), fmt.Sprintf("node-d1-%d", i%4), fmt.Sprintf("node-d2-%d", i%16), fmt.Sprintf("node-d3-%d", i)},
			SourceNodeID: fmt.Sprintf("node-d3-%d", i),
		}
		tree.Citations[i] = NodeCitation{
			DocID:   fmt.Sprintf("doc-%d", i),
			Title:   fmt.Sprintf("Document Title %d: A Research Paper on Topic %d", i, i),
			URL:     fmt.Sprintf("https://example.com/docs/%d", i),
			Snippet: fmt.Sprintf("This is a snippet for document %d containing enough text to represent a real citation snippet", i),
		}
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(tree)
	if err != nil {
		t.Fatalf("failed to marshal tree: %v", err)
	}

	// Gzip compress
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(jsonData); err != nil {
		t.Fatalf("gzip write error: %v", err)
	}
	gw.Close()

	compressedSize := buf.Len()
	const maxKB = 200
	if compressedSize > maxKB*1024 {
		t.Errorf("gzip compressed size = %d bytes (%d KB), exceeds %d KB limit", compressedSize, compressedSize/1024, maxKB)
	}

	t.Logf("85-node tree: JSON=%d bytes, gzip=%d bytes (%d KB)", len(jsonData), compressedSize, compressedSize/1024)
}

// ---------------------------------------------------------------------------
// T-D-010 [REFACTOR] — Race detection for crash/cancellation tests
// ---------------------------------------------------------------------------

func TestPersistenceCrashFinalizationRaceFree(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	p := NewTreePersistence()
	tree := sampleTreeResult()
	tree.Status = "partial"
	runID := "crash-race-test"

	mock.ExpectExec(`UPDATE deep_runs`).
		WithArgs(
			"failed",
			sqlmock.AnyArg(),
			runID,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = p.FinalizeCrash(db, tree, runID, time.Now())
	if err != nil {
		t.Fatalf("FinalizeCrash returned error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %v", err)
	}
}
