package deepagent

// @MX:NOTE: [AUTO] Node types for /deep tree exploration; BFS orchestration domain model
// @MX:SPEC: SPEC-DEEP-003

// NodeStatus represents the lifecycle state of a tree exploration node.
type NodeStatus string

const (
	// NodeStatusPending indicates the node is queued for expansion.
	NodeStatusPending NodeStatus = "pending"
	// NodeStatusExpanding indicates the node is currently being processed.
	NodeStatusExpanding NodeStatus = "expanding"
	// NodeStatusComplete indicates the node has been fully processed.
	NodeStatusComplete NodeStatus = "complete"
	// NodeStatusFailed indicates the node processing failed.
	NodeStatusFailed NodeStatus = "failed"
	// NodeStatusBudgetExceeded indicates expansion was denied due to budget.
	NodeStatusBudgetExceeded NodeStatus = "budget_exceeded"
)

// Node represents a single node in the /deep tree exploration.
// Each node corresponds to a sub-query at a specific depth and breadth position.
type Node struct {
	ID             string       `json:"id"`
	Depth          int          `json:"depth"`
	ParentID       string       `json:"parent_id"`
	BreadthIndex   int          `json:"breadth_index"`
	Query          string       `json:"query"`
	Status         NodeStatus   `json:"status"`
	Citations      []NodeCitation `json:"citations"`
	Claims         []NodeClaim  `json:"claims"`
	TokensUsed     int64        `json:"tokens_used"`
	ReservedTokens int64        `json:"reserved_tokens"`
	CostUSD        float64      `json:"cost_usd"`
}

// NodeCitation represents a source document cited by a node.
type NodeCitation struct {
	DocID   string `json:"doc_id"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// NodeClaim represents a claim extracted from a node's research.
type NodeClaim struct {
	Text    string   `json:"text"`
	Markers []string `json:"markers"`
}

// TreeResult is the final output of a /deep tree exploration run.
type TreeResult struct {
	RootQuery           string           `json:"root_query"`
	TotalNodes          int              `json:"total_nodes"`
	MaxDepthReached     int              `json:"max_depth_reached"`
	Status              string           `json:"status"`
	FlattenedClaims     []FlattenedClaim `json:"flattened_claims"`
	Citations           []NodeCitation   `json:"citations"`
	TotalTokensUsed     int64            `json:"total_tokens_used"`
	TotalReservedTokens int64            `json:"total_reserved_tokens"`
	TotalCostUSD        float64          `json:"total_cost_usd"`
}

// FlattenedClaim is a claim with its full lineage path from the root node.
// REQ-DEEP3-013: LineagePath must end with SourceNodeID.
type FlattenedClaim struct {
	Text         string   `json:"text"`
	Markers      []string `json:"markers"`
	LineagePath  []string `json:"lineage_path"`
	SourceNodeID string   `json:"source_node_id"`
}
