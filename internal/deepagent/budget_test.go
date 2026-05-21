package deepagent

import (
	"sync"
	"testing"
)

// T-A-003 [RED]: Budget pre-check tests
// REQ-DEEP3-006: BudgetTracker enforces token budget, structural cap, headroom.
// REQ-DEEP3-007: PreCheck is atomic (read + decision + reservation under lock).
// NFR-DEEP3-004: Estimated cost uses parent.TokensUsed * breadth * 1.25 (25% headroom).

func TestBudgetPreCheckTokenExceeded(t *testing.T) {
	t.Parallel()

	// Total budget is 10000 tokens, and 9500 already used.
	// A new node requesting ~1875 tokens (1500 * 1.25) should be denied.
	tracker := &BudgetTracker{
		TotalTokensUsed:    9500,
		TotalReservedTokens: 0,
		TotalCostUSD:       0.0,
		TotalNodes:         5,
	}

	parent := &Node{
		ID:             "parent-1",
		Depth:          1,
		TokensUsed:     1500,
		ReservedTokens: 0,
	}

	cfg := BudgetConfig{
		TokenBudget:       10000,
		DefaultBreadth:    3,
		DefaultDepth:      3,
		RootTokenEstimate: 5000,
	}

	decision := tracker.PreCheck(parent, cfg)
	if decision.Allowed {
		t.Errorf("PreCheck returned Allowed=true, want false (token budget exceeded)")
	}
	if decision.Reason == "" {
		t.Error("PreCheck denial reason is empty, want non-empty reason")
	}
}

func TestBudgetPreCheckStructuralCap(t *testing.T) {
	t.Parallel()

	// With breadth=3, depth=3: max nodes = 1 + 3 + 9 + 27 = 40.
	// If we already have 40 nodes, any new expansion should be denied.
	tracker := &BudgetTracker{
		TotalTokensUsed:    1000,
		TotalReservedTokens: 0,
		TotalCostUSD:       0.01,
		TotalNodes:         40,
	}

	parent := &Node{
		ID:             "parent-1",
		Depth:          2,
		TokensUsed:     500,
		ReservedTokens: 0,
	}

	cfg := BudgetConfig{
		TokenBudget:       100000, // Plenty of token budget
		DefaultBreadth:    3,
		DefaultDepth:      3,
		RootTokenEstimate: 5000,
	}

	decision := tracker.PreCheck(parent, cfg)
	if decision.Allowed {
		t.Errorf("PreCheck returned Allowed=true, want false (structural cap exceeded)")
	}
	if decision.Reason == "" {
		t.Error("PreCheck denial reason is empty, want non-empty reason")
	}
}

func TestBudgetPreCheckHeadroomConservative(t *testing.T) {
	t.Parallel()

	// Headroom: estimated cost = parent.TokensUsed * breadth * 1.25
	// Parent used 1000 tokens, breadth=3 → estimated = 1000 * 3 * 1.25 = 3750
	// Budget = 5000, already used 1000, reserved 0 → remaining = 4000
	// 3750 <= 4000 → should be allowed.
	tracker := &BudgetTracker{
		TotalTokensUsed:    1000,
		TotalReservedTokens: 0,
		TotalCostUSD:       0.01,
		TotalNodes:         2,
	}

	parent := &Node{
		ID:             "parent-1",
		Depth:          1,
		TokensUsed:     1000,
		ReservedTokens: 0,
	}

	cfg := BudgetConfig{
		TokenBudget:       5000,
		DefaultBreadth:    3,
		DefaultDepth:      3,
		RootTokenEstimate: 5000,
	}

	decision := tracker.PreCheck(parent, cfg)
	if !decision.Allowed {
		t.Errorf("PreCheck returned Allowed=false, want true (headroom sufficient). Reason: %s", decision.Reason)
	}

	// Verify the reserved tokens were incremented by the estimated amount.
	expectedReserved := int64(1000 * 3 * 1.25) // 3750
	if tracker.TotalReservedTokens != expectedReserved {
		t.Errorf("TotalReservedTokens = %d, want %d", tracker.TotalReservedTokens, expectedReserved)
	}
}

// T-A-004 [RED][RACE]: Concurrency tests for budget reservation lock
// REQ-DEEP3-006: Reservation lock serializes sibling expansion.
// REQ-DEEP3-007: Atomic read+decision+reservation increment under lock.
// NFR-DEEP3-003: No race conditions under concurrent PreCheck calls.

func TestBudgetReservationLockSerializesSiblings(t *testing.T) {
	// Do NOT use t.Parallel() — this is a concurrency test.

	// Budget: 30000 tokens, 8 concurrent siblings, each estimating ~1875 tokens.
	// Total if all allowed: 8 * 1875 = 15000. Budget remaining = 30000 - 5000 = 25000.
	// All should be allowed, and TotalReservedTokens must be exactly 8 * 1875 = 15000.
	tracker := &BudgetTracker{
		TotalTokensUsed:    5000,
		TotalReservedTokens: 0,
		TotalCostUSD:       0.05,
		TotalNodes:         5,
	}

	parent := &Node{
		ID:             "parent-1",
		Depth:          1,
		TokensUsed:     500,
		ReservedTokens: 0,
	}

	cfg := BudgetConfig{
		TokenBudget:       30000,
		DefaultBreadth:    3,
		DefaultDepth:      3,
		RootTokenEstimate: 5000,
	}

	var wg sync.WaitGroup
	results := make([]BudgetDecision, 8)

	for i := range 8 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = tracker.PreCheck(parent, cfg)
		}(i)
	}
	wg.Wait()

	allowedCount := 0
	for _, d := range results {
		if d.Allowed {
			allowedCount++
		}
	}

	// All 8 should be allowed since total reserved (15000) fits in remaining budget (25000).
	if allowedCount != 8 {
		t.Errorf("allowed %d of 8 siblings, want 8", allowedCount)
	}

	// TotalReservedTokens must be exactly 8 * 1875 = 15000 (no double-counting, no lost updates).
	expectedReserved := int64(8 * 500 * 3 * 1.25) // 15000
	if tracker.TotalReservedTokens != expectedReserved {
		t.Errorf("TotalReservedTokens = %d, want %d (race condition or non-atomic update)", tracker.TotalReservedTokens, expectedReserved)
	}
}

func TestBudgetReservationReleaseOnComplete(t *testing.T) {
	// Do NOT use t.Parallel() — tests stateful mutation.

	// Initial state: 1000 used, 3750 reserved.
	tracker := &BudgetTracker{
		TotalTokensUsed:    1000,
		TotalReservedTokens: 3750,
		TotalCostUSD:       0.01,
		TotalNodes:         3,
	}

	node := &Node{
		ID:             "node-1",
		Depth:          1,
		TokensUsed:     0,
		ReservedTokens: 3750, // was reserved during PreCheck
	}

	// Node completes with actual usage of 2000 tokens.
	// Delta = 2000 - 0 = 2000 (new usage).
	// Reserved delta = 3750 - 2000 = 1750 (excess reservation returned).
	tracker.ReleaseOnComplete(node, 2000)

	// TotalTokensUsed should increase by 2000.
	if tracker.TotalTokensUsed != 3000 {
		t.Errorf("TotalTokensUsed = %d, want 3000", tracker.TotalTokensUsed)
	}

	// TotalReservedTokens should decrease by (3750 - 2000) = 1750.
	// So: 3750 - 1750 = 2000.
	if tracker.TotalReservedTokens != 2000 {
		t.Errorf("TotalReservedTokens = %d, want 2000", tracker.TotalReservedTokens)
	}
}

// T-A-005 [RED]: Root node seed estimate tests
// REQ-DEEP3-006: Root node pre-check uses DEEP_TREE_ROOT_TOKEN_ESTIMATE (default 5000).

func TestBudgetRootSeedEstimate(t *testing.T) {
	t.Parallel()

	// Root node (parent is nil or depth=0) uses RootTokenEstimate instead of parent.TokensUsed.
	tracker := &BudgetTracker{
		TotalTokensUsed:    0,
		TotalReservedTokens: 0,
		TotalCostUSD:       0.0,
		TotalNodes:         0,
	}

	// Root node — no parent, depth 0.
	rootNode := &Node{
		ID:         "root",
		Depth:      0,
		ParentID:   "",
		TokensUsed: 0,
	}

	cfg := BudgetConfig{
		TokenBudget:       20000,
		DefaultBreadth:    3,
		DefaultDepth:      3,
		RootTokenEstimate: 5000,
	}

	decision := tracker.PreCheck(rootNode, cfg)
	if !decision.Allowed {
		t.Errorf("PreCheck for root node returned Allowed=false, want true. Reason: %s", decision.Reason)
	}

	// For root, estimate = RootTokenEstimate (5000), not parent.TokensUsed * breadth * 1.25.
	if tracker.TotalReservedTokens != 5000 {
		t.Errorf("TotalReservedTokens = %d, want 5000 (RootTokenEstimate)", tracker.TotalReservedTokens)
	}
}

func TestBudgetRootSeedTriggersImmediateBudgetFail(t *testing.T) {
	t.Parallel()

	// Budget = 4000, root estimate = 5000 → immediate deny.
	tracker := &BudgetTracker{
		TotalTokensUsed:    0,
		TotalReservedTokens: 0,
		TotalCostUSD:       0.0,
		TotalNodes:         0,
	}

	rootNode := &Node{
		ID:         "root",
		Depth:      0,
		ParentID:   "",
		TokensUsed: 0,
	}

	cfg := BudgetConfig{
		TokenBudget:       4000,
		DefaultBreadth:    3,
		DefaultDepth:      3,
		RootTokenEstimate: 5000,
	}

	decision := tracker.PreCheck(rootNode, cfg)
	if decision.Allowed {
		t.Error("PreCheck for root node returned Allowed=true, want false (budget too small for root seed)")
	}
}
