package deepagent

import (
	"context"
	"crypto/sha256"
	"fmt"
	"runtime"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

// @MX:NOTE: [AUTO] BFS tree expansion orchestrator for /deep tree exploration
// @MX:SPEC: SPEC-DEEP-003

// TreeResearcher defines the interface for tree expansion operations.
// The BFS loop calls Decompose to generate sub-queries and Fanout to
// retrieve citations and claims for each node.
type TreeResearcher interface {
	Decompose(ctx context.Context, req DecomposeRequest) ([]string, error)
	Fanout(ctx context.Context, query string) ([]NodeCitation, []NodeClaim, int64, error)
}

// DecomposeRequest is the input to the Decompose operation.
type DecomposeRequest struct {
	RootQuery             string
	ParentQuery           string
	ParentEvidenceSummary string
	Breadth               int
}

// TreeConfig holds configuration for the BFS tree expansion.
type TreeConfig struct {
	Breadth            int
	Depth              int
	TokenBudget        int64
	NodeTimeoutMs      int
	RootTokenEstimate  int64
	ModelPricePerToken float64
	RunID              string
	// Hooks provides optional observability callbacks. Nil-safe.
	// @MX:NOTE: [AUTO] Hook pattern decouples tree logic from metrics/OTel; callers pass nil for unit tests
	Hooks TreeHooks
}

// treeState holds mutable state during tree expansion.
type treeState struct {
	tracker *BudgetTracker
	nodes   []*Node
	mu      sync.Mutex // protects nodes slice for concurrent appends
}

// ExpandTree performs a BFS tree expansion from rootQuery with the given configuration.
// It creates a deterministic root node, then expands level by level using errgroup
// for concurrent fanout within each depth level.
//
// @MX:ANCHOR: [AUTO] ExpandTree — BFS tree expansion contract; downstream Writer dependency
// @MX:REASON: Tree expansion is the primary entry point for /deep multi-agent; Writer consumes TreeResult
// @MX:SPEC: SPEC-DEEP-003
func ExpandTree(ctx context.Context, cfg TreeConfig, rootQuery string, researcher TreeResearcher) (*TreeResult, error) {
	// Input validation: breadth in [1, 8], depth in [1, 5].
	if cfg.Breadth < 1 || cfg.Breadth > 8 {
		return nil, fmt.Errorf("invalid_tree_config: breadth=%d is out of range [1, 8]", cfg.Breadth)
	}
	if cfg.Depth < 1 || cfg.Depth > 5 {
		return nil, fmt.Errorf("invalid_tree_config: depth=%d is out of range [1, 5]", cfg.Depth)
	}

	// Create root node with deterministic ID.
	rootID := fmt.Sprintf("%x", sha256.Sum256([]byte(cfg.RunID+"root")))
	root := &Node{
		ID:           rootID,
		Depth:        0,
		ParentID:     "",
		Query:        rootQuery,
		Status:       NodeStatusPending,
		BreadthIndex: 0,
	}

	// Initialize budget tracker.
	budgetCfg := BudgetConfig{
		TokenBudget:      cfg.TokenBudget,
		DefaultBreadth:   cfg.Breadth,
		DefaultDepth:     cfg.Depth,
		RootTokenEstimate: cfg.RootTokenEstimate,
	}
	tracker := &BudgetTracker{}

	state := &treeState{
		tracker: tracker,
		nodes:   []*Node{},
	}

	// Process root node via fanout.
	if err := processNode(ctx, root, 0, cfg, budgetCfg, state, researcher); err != nil {
		return nil, fmt.Errorf("root node expansion failed: %w", err)
	}

	// BFS loop: expand level by level.
	// currentDepth is the depth of the parent nodes being decomposed.
	// Children will be at currentDepth+1.
	for currentDepth := 0; currentDepth < cfg.Depth; currentDepth++ {
		// Collect all completed nodes at currentDepth.
		var frontier []*Node
		state.mu.Lock()
		for _, n := range state.nodes {
			if n.Depth == currentDepth && n.Status == NodeStatusComplete {
				frontier = append(frontier, n)
			}
		}
		state.mu.Unlock()

		if len(frontier) == 0 {
			break
		}

		// Decompose all frontier nodes to get child queries.
		var children []*Node
		for _, parent := range frontier {
			evidenceSummary := buildEvidenceSummary(parent)

			subQueries, err := researcher.Decompose(ctx, DecomposeRequest{
				RootQuery:             rootQuery,
				ParentQuery:           parent.Query,
				ParentEvidenceSummary: evidenceSummary,
				Breadth:               cfg.Breadth,
			})
			if err != nil {
				parent.Status = NodeStatusFailed
				continue
			}

			for i, sq := range subQueries {
				childID := fmt.Sprintf("%x", sha256.Sum256([]byte(cfg.RunID+parent.ID+fmt.Sprintf("-%d", i))))
				child := &Node{
					ID:           childID,
					Depth:        currentDepth + 1,
					ParentID:     parent.ID,
					BreadthIndex: i,
					Query:        sq,
					Status:       NodeStatusPending,
				}
				children = append(children, child)
			}
		}

		if len(children) == 0 {
			break
		}

		// Process all children concurrently using errgroup.
		// @MX:WARN: [AUTO] Bounded concurrency via errgroup per depth level
		// @MX:REASON: Parent context cancellation propagates through all child goroutines; errgroup.Wait ensures depth-level join before advancing to next depth
		g, gCtx := errgroup.WithContext(ctx)

		for _, child := range children {
			child := child // capture loop variable
			// Look up parent's actual tokens for budget estimation.
			parentTokens := findParentTokens(state, child.ParentID)
			g.Go(func() error {
				nodeCtx, cancel := context.WithTimeout(gCtx, time.Duration(cfg.NodeTimeoutMs)*time.Millisecond)
				defer cancel()

				return processNode(nodeCtx, child, parentTokens, cfg, budgetCfg, state, researcher)
			})
		}

		// Wait for all children at this depth level before proceeding (BFS invariant).
		if err := g.Wait(); err != nil {
			_ = err // individual node errors handled in processNode
		}

		// Memory guard: check in-memory footprint after each depth level.
		if err := checkMemoryGuard(); err != nil {
			// Truncate remaining frontier to restore memory bound.
			truncateFrontier(state)
		}
	}

	// Build final result.
	return buildTreeResult(rootQuery, state, cfg), nil
}

// processNode handles budget pre-check, fanout, and release for a single node.
func processNode(ctx context.Context, node *Node, parentTokens int64, cfg TreeConfig, budgetCfg BudgetConfig, state *treeState, researcher TreeResearcher) error {
	start := time.Now()

	// For non-root nodes, create a pre-check proxy with parent's token usage
	// so PreCheck estimates correctly: parent.TokensUsed * breadth * 1.25.
	preCheckNode := node
	if parentTokens > 0 {
		preCheckNode = &Node{
			ID:         node.ID,
			Depth:      node.Depth,
			ParentID:   node.ParentID,
			TokensUsed: parentTokens, // Use parent's actual tokens for estimation
		}
	}

	// Pre-check budget.
	decision := state.tracker.PreCheck(preCheckNode, budgetCfg)
	if !decision.Allowed {
		node.Status = NodeStatusBudgetExceeded
		state.mu.Lock()
		state.nodes = append(state.nodes, node)
		state.mu.Unlock()
		// Hook: budget exceeded.
		if cfg.Hooks.OnNodeBudgetExceeded != nil {
			cfg.Hooks.OnNodeBudgetExceeded(node)
		}
		return nil // not an error, just budget exceeded
	}

	// Record reservation amount based on pre-check estimate.
	node.ReservedTokens = getReservedAmount(node, parentTokens, budgetCfg)

	// Transition to expanding.
	node.Status = NodeStatusExpanding

	// Call fanout.
	citations, claims, tokensUsed, err := researcher.Fanout(ctx, node.Query)
	if err != nil {
		node.Status = NodeStatusFailed
		state.tracker.ReleaseOnComplete(node, 0)
		state.mu.Lock()
		state.nodes = append(state.nodes, node)
		state.mu.Unlock()
		// Hook: node failed.
		if cfg.Hooks.OnNodeFailed != nil {
			cfg.Hooks.OnNodeFailed(node, time.Since(start))
		}
		return fmt.Errorf("fanout failed for node %s: %w", node.ID, err)
	}

	// Copy citations to ensure per-node slice ownership (REQ-DEEP3-009b).
	nodeCitations := make([]NodeCitation, len(citations))
	copy(nodeCitations, citations)

	// Copy claims similarly.
	nodeClaims := make([]NodeClaim, len(claims))
	copy(nodeClaims, claims)

	node.Citations = nodeCitations
	node.Claims = nodeClaims
	node.TokensUsed = tokensUsed
	node.Status = NodeStatusComplete

	// Compute CostUSD: TokensUsed * ModelPricePerToken (REQ-DEEP3-013).
	node.CostUSD = float64(tokensUsed) * cfg.ModelPricePerToken

	// Release reservation and update budget atomically (REQ-DEEP3-007).
	state.tracker.ReleaseOnComplete(node, tokensUsed)

	// Add to nodes list.
	state.mu.Lock()
	state.nodes = append(state.nodes, node)
	state.mu.Unlock()

	// Hook: node complete.
	if cfg.Hooks.OnNodeComplete != nil {
		cfg.Hooks.OnNodeComplete(node, time.Since(start))
	}

	return nil
}

// findParentTokens returns the TokensUsed of the parent node identified by parentID.
func findParentTokens(state *treeState, parentID string) int64 {
	state.mu.Lock()
	defer state.mu.Unlock()
	for _, n := range state.nodes {
		if n.ID == parentID {
			return n.TokensUsed
		}
	}
	return 0
}

// getReservedAmount returns the tokens reserved for this node's expansion.
func getReservedAmount(node *Node, parentTokens int64, cfg BudgetConfig) int64 {
	if node.Depth == 0 && node.ParentID == "" {
		return cfg.RootTokenEstimate
	}
	return int64(float64(parentTokens) * float64(cfg.DefaultBreadth) * 1.25)
}

// buildEvidenceSummary creates a text summary from a parent node's research results.
func buildEvidenceSummary(node *Node) string {
	if len(node.Claims) == 0 {
		return ""
	}
	summary := ""
	for _, c := range node.Claims {
		if summary != "" {
			summary += "; "
		}
		summary += c.Text
	}
	return summary
}

// checkMemoryGuard checks if in-memory tree state exceeds 100MB.
func checkMemoryGuard() error {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	const memoryLimitMB = 100
	memMB := m.Alloc / 1024 / 1024
	if memMB > uint64(memoryLimitMB) {
		return fmt.Errorf("memory guard: %d MB exceeds %d MB limit", memMB, memoryLimitMB)
	}
	return nil
}

// truncateFrontier marks pending/expanding nodes as BudgetExceeded to reduce memory.
func truncateFrontier(state *treeState) {
	state.mu.Lock()
	defer state.mu.Unlock()
	for _, n := range state.nodes {
		if n.Status == NodeStatusPending || n.Status == NodeStatusExpanding {
			n.Status = NodeStatusBudgetExceeded
		}
	}
}

// buildTreeResult constructs the final TreeResult from the expansion state.
func buildTreeResult(rootQuery string, state *treeState, cfg TreeConfig) *TreeResult {
	result := &TreeResult{
		RootQuery: rootQuery,
		Status:    "complete",
	}

	maxDepth := 0
	var allCitations []NodeCitation

	// Build node map for lineage reconstruction.
	nodeMap := make(map[string]*Node)
	state.mu.Lock()
	for _, node := range state.nodes {
		nodeMap[node.ID] = node
	}
	state.mu.Unlock()

	for _, node := range state.nodes {
		result.TotalNodes++
		result.TotalTokensUsed += node.TokensUsed
		result.TotalCostUSD += node.CostUSD

		if node.Depth > maxDepth {
			maxDepth = node.Depth
		}

		// Collect citations from completed nodes.
		if node.Status == NodeStatusComplete {
			allCitations = append(allCitations, node.Citations...)
		}

		// Build flattened claims with lineage.
		for _, claim := range node.Claims {
			lineage := flattenWithLineage(node, nodeMap)
			result.FlattenedClaims = append(result.FlattenedClaims, FlattenedClaim{
				Text:         claim.Text,
				Markers:      claim.Markers,
				LineagePath:  lineage,
				SourceNodeID: node.ID,
			})
		}

		// Check for budget exceeded status.
		if node.Status == NodeStatusBudgetExceeded {
			result.Status = "budget_exceeded"
		}
	}

	result.MaxDepthReached = maxDepth
	result.Citations = allCitations
	result.TotalReservedTokens = state.tracker.TotalReservedTokens

	return result
}

// flattenWithLineage reconstructs the lineage path from a node to the root.
//
// @MX:NOTE: [AUTO] Lineage path reconstruction; O(N) per claim where N is tree depth
// @MX:SPEC: SPEC-DEEP-003
func flattenWithLineage(node *Node, nodeMap map[string]*Node) []string {
	var path []string
	current := node
	for current != nil {
		path = append([]string{current.ID}, path...)
		if current.ParentID == "" {
			break
		}
		current = nodeMap[current.ParentID]
	}
	return path
}
