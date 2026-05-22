package tools

import (
	"context"
	"testing"

	"github.com/elymas/universal-search/internal/adapters"
	"github.com/elymas/universal-search/pkg/types"
)

// TestSearchToolWrapsSharedOrchestrator verifies that the search tool handler
// delegates to the shared orchestrator and returns properly formatted output
// (REQ-MCP-008).
func TestSearchToolWrapsSharedOrchestrator(t *testing.T) {
	reg := adapters.NewRegistry(nil)
	_ = reg.Register(&stubAdapter{
		name: "test-source",
		caps: types.Capabilities{
			SourceID:  "test-source",
			DocTypes:  []types.DocType{types.DocTypeArticle},
		},
		docs: []types.NormalizedDoc{
			{ID: "d1", Title: "Test Doc", URL: "http://example.com", SourceID: "test-source"},
		},
	})

	cache := NewDocCache()
	handler := SearchHandler(reg, cache)
	_, output, err := handler(context.Background(), nil, SearchInput{Query: "test query"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.Summary == "" {
		t.Error("expected non-empty summary")
	}
	if output.Stats.SourceCount == 0 {
		t.Error("expected source_count > 0")
	}
	if output.Stats.LatencyMs < 0 {
		t.Error("expected latency_ms >= 0")
	}

	// Verify cache was populated for get_citation.
	doc, ok := cache.Get("d1")
	if !ok {
		t.Fatal("expected doc d1 in cache after search")
	}
	if doc.Title != "Test Doc" {
		t.Errorf("cached doc title: got %q, want 'Test Doc'", doc.Title)
	}
}

// TestSearchToolEmptyRegistry verifies that search with an empty registry
// returns an appropriate error.
func TestSearchToolEmptyRegistry(t *testing.T) {
	reg := adapters.NewRegistry(nil)
	cache := NewDocCache()

	handler := SearchHandler(reg, cache)
	_, _, err := handler(context.Background(), nil, SearchInput{Query: "test"})
	if err == nil {
		t.Fatal("expected error for empty registry, got nil")
	}
}
