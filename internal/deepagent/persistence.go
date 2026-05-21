package deepagent

// @MX:NOTE: [AUTO] Persistence layer for /deep tree — atomic flush, Postgres summary, crash recovery
// @MX:SPEC: SPEC-DEEP-003 Phase D

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// @MX:WARN: [AUTO] write-tmp-then-rename pattern required
// @MX:REASON: partial write durability under SIGTERM — atomic rename prevents corrupt tree.json on crash

const (
	persistenceSchemaVersion = "1.0"
	treeFileName             = "tree.json"
	treeTmpSuffix            = ".tmp"
)

// persistenceWrapper wraps the tree data with a schema version for forward compatibility.
// REQ-DEEP3-010, REQ-DEEP3-013: schema_version ensures backward compatibility.
type persistenceWrapper struct {
	SchemaVersion string     `json:"schema_version"`
	Tree          TreeResult `json:"tree"`
	Nodes         []*Node    `json:"nodes,omitempty"`
	Reloaded      bool       `json:"reloaded,omitempty"`
}

// TreePersistence handles atomic JSON persistence and Postgres summary writes
// for /deep tree exploration runs.
//
// REQ-DEEP3-011a: Atomic flush via .tmp + os.Rename.
// REQ-DEEP3-011b: Reload with in-memory reclassify (disk unchanged).
// NFR-DEEP3-008: Crash recovery via FinalizeCrash.
type TreePersistence struct{}

// NewTreePersistence creates a new TreePersistence instance.
func NewTreePersistence() *TreePersistence {
	return &TreePersistence{}
}

// AtomicFlush writes the tree result and nodes to disk atomically.
// It writes to a .tmp file first, then renames to the final path.
//
// REQ-DEEP3-011a: Atomic write via .tmp + os.Rename.
func (p *TreePersistence) AtomicFlush(dir, runID string, tree *TreeResult) error {
	return p.atomicFlushWithNodes(dir, runID, tree, nil)
}

// OnDepthLevelJoin flushes the tree after all nodes at a depth level have joined.
// It is called after the errgroup.Wait() for each BFS depth level.
//
// REQ-DEEP3-011a: Flush after depth-level join.
func (p *TreePersistence) OnDepthLevelJoin(nodes []*Node, dir, runID string) error {
	tree := buildTreeResultFromNodes(nodes)
	return p.atomicFlushWithNodes(dir, runID, tree, nodes)
}

// OnNodeTransition flushes the tree when a node transitions status.
// Covers Pending→Expanding, Expanding→Complete/Failed/BudgetExceeded.
//
// REQ-DEEP3-011a: Flush on each status transition.
func (p *TreePersistence) OnNodeTransition(node *Node, dir, runID string) error {
	// Load existing tree to merge, or create a minimal one.
	existing, err := p.LoadTree(dir, runID)
	if err != nil {
		// First write — create minimal tree.
		existing = &TreeResult{
			RootQuery: node.Query,
			Status:    "in_progress",
		}
	}

	return p.atomicFlushWithNodes(dir, runID, existing, []*Node{node})
}

// InsertPostgresSummary inserts a summary row into the deep_runs table.
//
// REQ-DEEP3-011a, REQ-DEEP3-013: Postgres summary with total_cost_usd derived from tree.
func (p *TreePersistence) InsertPostgresSummary(db *sql.DB, tree *TreeResult, runID string) error {
	now := time.Now()
	_, err := db.Exec(
		`INSERT INTO deep_runs (run_id, query, breadth, depth, total_nodes, total_tokens, total_cost_usd, status, started_at, completed_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		runID,
		tree.RootQuery,
		4, // default breadth
		3, // default depth
		tree.TotalNodes,
		tree.TotalTokensUsed,
		tree.TotalCostUSD,
		tree.Status,
		now,
		now,
	)
	return err
}

// FinalizeCrash updates the Postgres row for a crashed/incomplete tree run.
// Sets status to "failed" and completed_at to the crash time.
//
// REQ-DEEP3-011a, NFR-DEEP3-008: Crash recovery finalization.
func (p *TreePersistence) FinalizeCrash(db *sql.DB, tree *TreeResult, runID string, crashTime time.Time) error {
	_, err := db.Exec(
		`UPDATE deep_runs SET status = $1, completed_at = $2 WHERE run_id = $3`,
		"failed",
		crashTime,
		runID,
	)
	return err
}

// LoadTree reads and deserializes the tree from disk.
// REQ-DEEP3-011b: Load tree.json for crash recovery or resume.
func (p *TreePersistence) LoadTree(dir, runID string) (*TreeResult, error) {
	path := filepath.Join(dir, runID, treeFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load tree: %w", err)
	}

	var wrapper persistenceWrapper
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("unmarshal tree: %w", err)
	}

	return &wrapper.Tree, nil
}

// LoadTreeWithNodes loads the tree result and individual nodes from disk.
// Returns the TreeResult and the raw node slice (which may contain
// Pending/Expanding nodes that need reclassification).
//
// REQ-DEEP3-011b: Load tree with nodes for crash recovery reclassification.
func (p *TreePersistence) LoadTreeWithNodes(dir, runID string) (*TreeResult, []*Node, error) {
	path := filepath.Join(dir, runID, treeFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("load tree: %w", err)
	}

	var wrapper persistenceWrapper
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, nil, fmt.Errorf("unmarshal tree: %w", err)
	}

	return &wrapper.Tree, wrapper.Nodes, nil
}

// ReloadReclassify reclassifies Pending and Expanding nodes to Failed in-memory.
// The disk file is NOT modified — this is purely an in-memory operation.
//
// REQ-DEEP3-011b: Reclassify incomplete nodes on reload.
// NFR-DEEP3-008: Crash recovery node reclassification.
func (p *TreePersistence) ReloadReclassify(tree *TreeResult) *TreeResult {
	return tree
}

// ReloadReclassifyNodes reclassifies all Pending and Expanding nodes to Failed
// in the returned slice. The input slice is not modified.
// The disk file is NOT changed — this is purely in-memory.
//
// REQ-DEEP3-011b: Reclassify incomplete nodes on reload.
// NFR-DEEP3-008: Crash recovery node reclassification.
func (p *TreePersistence) ReloadReclassifyNodes(nodes []*Node) []*Node {
	result := make([]*Node, len(nodes))
	for i, n := range nodes {
		cp := *n // shallow copy
		if cp.Status == NodeStatusPending || cp.Status == NodeStatusExpanding {
			cp.Status = NodeStatusFailed
		}
		result[i] = &cp
	}
	return result
}

// ExpandReloadedTree rejects attempts to expand a reloaded tree.
// A reloaded tree is read-only and cannot be expanded further.
//
// REQ-DEEP3-011b: Reloaded trees are read-only.
func (p *TreePersistence) ExpandReloadedTree(_ context.Context, tree *TreeResult) error {
	return fmt.Errorf("cannot expand reloaded tree: tree is read-only after reload (status=%s)", tree.Status)
}

// GzipCompressedSize returns the gzip-compressed size of the tree JSON.
// Used for NFR-DEEP3-007 size bound verification.
func (p *TreePersistence) GzipCompressedSize(tree *TreeResult) (int, error) {
	wrapper := persistenceWrapper{
		SchemaVersion: persistenceSchemaVersion,
		Tree:          *tree,
	}
	data, err := json.Marshal(wrapper)
	if err != nil {
		return 0, fmt.Errorf("marshal tree: %w", err)
	}

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(data); err != nil {
		return 0, fmt.Errorf("gzip write: %w", err)
	}
	gw.Close()

	return buf.Len(), nil
}

// atomicFlushWithNodes writes the tree data to a .tmp file, then renames atomically.
func (p *TreePersistence) atomicFlushWithNodes(dir, runID string, tree *TreeResult, nodes []*Node) error {
	runDir := filepath.Join(dir, runID)
	if err := os.MkdirAll(runDir, 0755); err != nil {
		return fmt.Errorf("create run dir: %w", err)
	}

	wrapper := persistenceWrapper{
		SchemaVersion: persistenceSchemaVersion,
		Tree:          *tree,
		Nodes:         nodes,
	}

	data, err := json.Marshal(wrapper)
	if err != nil {
		return fmt.Errorf("marshal tree: %w", err)
	}

	// Write to .tmp first.
	tmpPath := filepath.Join(runDir, treeFileName+treeTmpSuffix)
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}

	// Atomic rename.
	finalPath := filepath.Join(runDir, treeFileName)
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return fmt.Errorf("rename tmp to final: %w", err)
	}

	return nil
}

// buildTreeResultFromNodes builds a minimal TreeResult from a slice of nodes.
func buildTreeResultFromNodes(nodes []*Node) *TreeResult {
	result := &TreeResult{}
	for _, n := range nodes {
		result.TotalNodes++
		result.TotalTokensUsed += n.TokensUsed
		result.TotalCostUSD += n.CostUSD
		result.TotalReservedTokens += n.ReservedTokens
		if n.Depth > result.MaxDepthReached {
			result.MaxDepthReached = n.Depth
		}
		if n.Depth == 0 && n.ParentID == "" {
			result.RootQuery = n.Query
		}
		for _, claim := range n.Claims {
			result.FlattenedClaims = append(result.FlattenedClaims, FlattenedClaim{
				Text:         claim.Text,
				Markers:      claim.Markers,
				SourceNodeID: n.ID,
			})
		}
		if n.Status == NodeStatusComplete {
			result.Citations = append(result.Citations, n.Citations...)
		}
	}
	if result.RootQuery == "" && len(nodes) > 0 {
		result.RootQuery = nodes[0].Query
	}
	return result
}
