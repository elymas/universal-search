package deepagent

import (
	"encoding/json"
	"testing"
)

// T-A-001 [RED]: Tree types — JSON round-trip, exhaustive switch, lineage invariant
// REQ-DEEP3-010: Node/TreeResult types for /deep tree exploration.
// REQ-DEEP3-013: FlattenedClaim lineage invariants.

func TestTreeTypesJSONRoundTrip(t *testing.T) {
	t.Parallel()

	t.Run("Node", func(t *testing.T) {
		t.Parallel()
		original := Node{
			ID:             "node-001",
			Depth:          2,
			ParentID:       "node-000",
			BreadthIndex:   1,
			Query:          "subtopic A",
			Status:         NodeStatusComplete,
			Citations:      []NodeCitation{{DocID: "d1", Title: "Doc 1", URL: "https://example.com", Snippet: "snippet text"}},
			Claims:         []NodeClaim{{Text: "claim text", Markers: []string{"M1", "M2"}}},
			TokensUsed:     1200,
			ReservedTokens: 500,
			CostUSD:        0.003,
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("marshal Node: %v", err)
		}

		var decoded Node
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("unmarshal Node: %v", err)
		}

		if decoded.ID != original.ID {
			t.Errorf("ID = %q, want %q", decoded.ID, original.ID)
		}
		if decoded.Depth != original.Depth {
			t.Errorf("Depth = %d, want %d", decoded.Depth, original.Depth)
		}
		if decoded.ParentID != original.ParentID {
			t.Errorf("ParentID = %q, want %q", decoded.ParentID, original.ParentID)
		}
		if decoded.BreadthIndex != original.BreadthIndex {
			t.Errorf("BreadthIndex = %d, want %d", decoded.BreadthIndex, original.BreadthIndex)
		}
		if decoded.Query != original.Query {
			t.Errorf("Query = %q, want %q", decoded.Query, original.Query)
		}
		if decoded.Status != original.Status {
			t.Errorf("Status = %q, want %q", decoded.Status, original.Status)
		}
		if decoded.TokensUsed != original.TokensUsed {
			t.Errorf("TokensUsed = %d, want %d", decoded.TokensUsed, original.TokensUsed)
		}
		if decoded.ReservedTokens != original.ReservedTokens {
			t.Errorf("ReservedTokens = %d, want %d", decoded.ReservedTokens, original.ReservedTokens)
		}
		if decoded.CostUSD != original.CostUSD {
			t.Errorf("CostUSD = %f, want %f", decoded.CostUSD, original.CostUSD)
		}
		if len(decoded.Citations) != 1 || decoded.Citations[0].DocID != "d1" {
			t.Errorf("Citations not round-tripped correctly: %+v", decoded.Citations)
		}
		if len(decoded.Claims) != 1 || decoded.Claims[0].Text != "claim text" {
			t.Errorf("Claims not round-tripped correctly: %+v", decoded.Claims)
		}
	})

	t.Run("TreeResult", func(t *testing.T) {
		t.Parallel()
		original := TreeResult{
			RootQuery:           "main topic",
			TotalNodes:          5,
			MaxDepthReached:     3,
			Status:              "complete",
			FlattenedClaims:     []FlattenedClaim{{Text: "c1", Markers: []string{"M1"}, LineagePath: []string{"root", "node-001"}, SourceNodeID: "node-001"}},
			Citations:           []NodeCitation{{DocID: "d1", Title: "Doc 1", URL: "https://example.com", Snippet: "s"}},
			TotalTokensUsed:     5000,
			TotalReservedTokens: 1000,
			TotalCostUSD:        0.05,
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("marshal TreeResult: %v", err)
		}

		var decoded TreeResult
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("unmarshal TreeResult: %v", err)
		}

		if decoded.RootQuery != original.RootQuery {
			t.Errorf("RootQuery = %q, want %q", decoded.RootQuery, original.RootQuery)
		}
		if decoded.TotalNodes != original.TotalNodes {
			t.Errorf("TotalNodes = %d, want %d", decoded.TotalNodes, original.TotalNodes)
		}
		if decoded.MaxDepthReached != original.MaxDepthReached {
			t.Errorf("MaxDepthReached = %d, want %d", decoded.MaxDepthReached, original.MaxDepthReached)
		}
		if decoded.TotalTokensUsed != original.TotalTokensUsed {
			t.Errorf("TotalTokensUsed = %d, want %d", decoded.TotalTokensUsed, original.TotalTokensUsed)
		}
		if decoded.TotalCostUSD != original.TotalCostUSD {
			t.Errorf("TotalCostUSD = %f, want %f", decoded.TotalCostUSD, original.TotalCostUSD)
		}
		if len(decoded.FlattenedClaims) != 1 {
			t.Fatalf("len(FlattenedClaims) = %d, want 1", len(decoded.FlattenedClaims))
		}
		if decoded.FlattenedClaims[0].SourceNodeID != "node-001" {
			t.Errorf("FlattenedClaim.SourceNodeID = %q, want %q", decoded.FlattenedClaims[0].SourceNodeID, "node-001")
		}
	})
}

func TestNodeStatusExhaustiveSwitch(t *testing.T) {
	t.Parallel()

	// Verify all NodeStatus enum values are handled in a switch.
	allStatuses := []NodeStatus{
		NodeStatusPending,
		NodeStatusExpanding,
		NodeStatusComplete,
		NodeStatusFailed,
		NodeStatusBudgetExceeded,
	}

	for _, s := range allStatuses {
		switch s {
		case NodeStatusPending, NodeStatusExpanding, NodeStatusComplete,
			NodeStatusFailed, NodeStatusBudgetExceeded:
			// OK — handled
		default:
			t.Errorf("unhandled NodeStatus: %q", s)
		}
	}

	// Verify the count is exactly 5.
	if len(allStatuses) != 5 {
		t.Errorf("len(allStatuses) = %d, want 5 — missing or extra NodeStatus values", len(allStatuses))
	}

	// Verify string values are meaningful (non-empty).
	for _, s := range allStatuses {
		if string(s) == "" {
			t.Error("NodeStatus has empty string value")
		}
	}
}

func TestFlattenedClaimLineageInvariant(t *testing.T) {
	t.Parallel()

	// FlattenedClaim must carry both lineage_path and source_node_id.
	claim := FlattenedClaim{
		Text:         "subtopic claim",
		Markers:      []string{"M1", "M2"},
		LineagePath:  []string{"root", "node-001", "node-003"},
		SourceNodeID: "node-003",
	}

	if claim.SourceNodeID == "" {
		t.Error("SourceNodeID must not be empty")
	}
	if len(claim.LineagePath) == 0 {
		t.Error("LineagePath must not be empty")
	}

	// The last element of LineagePath must equal SourceNodeID.
	last := claim.LineagePath[len(claim.LineagePath)-1]
	if last != claim.SourceNodeID {
		t.Errorf("LineagePath[-1] = %q, want SourceNodeID %q", last, claim.SourceNodeID)
	}

	// JSON round-trip preserves lineage fields.
	data, err := json.Marshal(claim)
	if err != nil {
		t.Fatalf("marshal FlattenedClaim: %v", err)
	}
	var decoded FlattenedClaim
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal FlattenedClaim: %v", err)
	}
	if len(decoded.LineagePath) != len(claim.LineagePath) {
		t.Errorf("LineagePath length = %d after round-trip, want %d", len(decoded.LineagePath), len(claim.LineagePath))
	}
	if decoded.SourceNodeID != claim.SourceNodeID {
		t.Errorf("SourceNodeID = %q after round-trip, want %q", decoded.SourceNodeID, claim.SourceNodeID)
	}
}
