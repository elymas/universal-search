package deepagent

import (
	"fmt"
	"sync"
)

// @MX:ANCHOR: [AUTO] BudgetTracker.PreCheck — 3-cap simultaneous enforcement (token, structural, headroom); high fan_in (called per node per depth level)
// @MX:REASON: Every tree expansion node calls PreCheck; budget enforcement must be atomic to prevent overspend under concurrent sibling expansion
// @MX:SPEC: SPEC-DEEP-003

// BudgetConfig holds configuration for budget tracking.
type BudgetConfig struct {
	// TokenBudget is the maximum total tokens allowed for the entire tree exploration.
	TokenBudget int64
	// DefaultBreadth is the default number of child nodes per expansion.
	DefaultBreadth int
	// DefaultDepth is the maximum depth of the tree.
	DefaultDepth int
	// RootTokenEstimate is the token estimate for the root node expansion.
	// Default: 5000 (DEEP_TREE_ROOT_TOKEN_ESTIMATE).
	RootTokenEstimate int64
}

// BudgetDecision represents the outcome of a budget pre-check.
type BudgetDecision struct {
	Allowed bool
	Reason  string
}

// BudgetTracker tracks token usage and enforces budget constraints
// for the /deep tree exploration. It is safe for concurrent use.
//
// REQ-DEEP3-006: Budget cap enforcement.
// REQ-DEEP3-007: Atomic read+decision+reservation under mutex.
type BudgetTracker struct {
	mu                  sync.Mutex
	TotalTokensUsed     int64
	TotalReservedTokens int64
	TotalCostUSD        float64
	TotalNodes          int
}

// PreCheck determines whether a node expansion is allowed within budget.
// It holds the reservation lock during read + decision + reservation increment,
// ensuring atomic budget enforcement for concurrent sibling expansions.
//
// For root nodes (Depth == 0, ParentID == ""), the estimate is cfg.RootTokenEstimate.
// For non-root nodes, the estimate is parent.TokensUsed * breadth * 1.25 (25% headroom).
//
// Structural cap: max nodes = 1 + sum(breadth^i for i=1..depth).
func (bt *BudgetTracker) PreCheck(node *Node, cfg BudgetConfig) BudgetDecision {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	// Calculate estimated tokens for this expansion.
	var estimated int64
	if node.Depth == 0 && node.ParentID == "" {
		// Root node uses seed estimate.
		estimated = cfg.RootTokenEstimate
	} else {
		// Non-root: parent.TokensUsed * breadth * 1.25 (25% headroom).
		estimated = int64(float64(node.TokensUsed) * float64(cfg.DefaultBreadth) * 1.25)
	}

	// Check token budget: remaining must cover the estimate.
	remaining := cfg.TokenBudget - bt.TotalTokensUsed - bt.TotalReservedTokens
	if estimated > remaining {
		return BudgetDecision{
			Allowed: false,
			Reason:  fmt.Sprintf("token budget exceeded: estimated %d, remaining %d (used %d, reserved %d, budget %d)", estimated, remaining, bt.TotalTokensUsed, bt.TotalReservedTokens, cfg.TokenBudget),
		}
	}

	// Check structural cap: max nodes = 1 + sum(breadth^i for i=1..depth).
	maxNodes := 1
	accum := 1
	for i := 1; i <= cfg.DefaultDepth; i++ {
		accum *= cfg.DefaultBreadth
		maxNodes += accum
	}
	if bt.TotalNodes >= maxNodes {
		return BudgetDecision{
			Allowed: false,
			Reason:  fmt.Sprintf("structural cap exceeded: %d nodes, max %d (breadth=%d, depth=%d)", bt.TotalNodes, maxNodes, cfg.DefaultBreadth, cfg.DefaultDepth),
		}
	}

	// All checks passed — reserve the tokens.
	bt.TotalReservedTokens += estimated

	return BudgetDecision{
		Allowed: true,
		Reason:  "",
	}
}

// ReleaseOnComplete reconciles a completed node's actual token usage against
// its reservation. It atomically updates TotalTokensUsed and adjusts
// TotalReservedTokens by the difference between reserved and actual usage.
//
// This must be called when a node finishes processing (success or failure).
func (bt *BudgetTracker) ReleaseOnComplete(node *Node, actualTokens int64) {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	// Add actual usage to total.
	bt.TotalTokensUsed += actualTokens

	// Return excess reservation: reserved - actual.
	// If actual > reserved (rare), this would be negative, which means
	// we under-reserved — we still subtract the delta (adding to reserved is wrong).
	excess := node.ReservedTokens - actualTokens
	if excess > 0 {
		bt.TotalReservedTokens -= excess
	}

	bt.TotalNodes++
}
