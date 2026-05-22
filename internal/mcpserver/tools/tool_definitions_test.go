package tools

import (
	"fmt"
	"testing"

	"github.com/elymas/universal-search/internal/orchestrator"
	"github.com/elymas/universal-search/pkg/types"
)

// TestDeepResearchToolDefinition verifies the deep_research tool definition.
func TestDeepResearchToolDefinition(t *testing.T) {
	tool := DeepResearchTool()
	if tool == nil {
		t.Fatal("expected non-nil tool")
	}
	if tool.Name != "deep_research" {
		t.Errorf("name: got %q, want deep_research", tool.Name)
	}
	if tool.Description == "" {
		t.Error("expected non-empty description")
	}
}

// TestGetCitationToolDefinition verifies the get_citation tool definition.
func TestGetCitationToolDefinition(t *testing.T) {
	tool := GetCitationTool()
	if tool == nil {
		t.Fatal("expected non-nil tool")
	}
	if tool.Name != "get_citation" {
		t.Errorf("name: got %q, want get_citation", tool.Name)
	}
	if tool.Description == "" {
		t.Error("expected non-empty description")
	}
}

// TestListSourcesToolDefinition verifies the list_sources tool definition.
func TestListSourcesToolDefinition(t *testing.T) {
	tool := ListSourcesTool()
	if tool == nil {
		t.Fatal("expected non-nil tool")
	}
	if tool.Name != "list_sources" {
		t.Errorf("name: got %q, want list_sources", tool.Name)
	}
	if tool.Description == "" {
		t.Error("expected non-empty description")
	}
}

// TestSearchToolDefinition verifies the search tool definition.
func TestSearchToolDefinition(t *testing.T) {
	tool := SearchTool()
	if tool == nil {
		t.Fatal("expected non-nil tool")
	}
	if tool.Name != "search" {
		t.Errorf("name: got %q, want search", tool.Name)
	}
	if tool.Description == "" {
		t.Error("expected non-empty description")
	}
}

// TestStoreDocs verifies the StoreDocs convenience function.
func TestStoreDocs(t *testing.T) {
	cache := NewDocCache()
	docs := []types.NormalizedDoc{
		{ID: "d1", Title: "Doc One"},
		{ID: "d2", Title: "Doc Two"},
	}

	StoreDocs(cache, docs)

	doc, ok := cache.Get("d1")
	if !ok {
		t.Fatal("expected d1 in cache")
	}
	if doc.Title != "Doc One" {
		t.Errorf("title: got %q, want Doc One", doc.Title)
	}

	doc2, ok := cache.Get("d2")
	if !ok {
		t.Fatal("expected d2 in cache")
	}
	if doc2.Title != "Doc Two" {
		t.Errorf("title: got %q, want Doc Two", doc2.Title)
	}
}

// TestNoopLogger verifies noopLogger returns a non-nil logger.
func TestNoopLogger(t *testing.T) {
	logger := noopLogger()
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
}

// TestMapOrchestratorErrorNoAdapters verifies the "no adapters matched" branch.
func TestMapOrchestratorErrorNoAdapters(t *testing.T) {
	err := fmt.Errorf("orchestrator: no adapters matched")
	mcpErr := mapOrchestratorError(err, nil)
	if mcpErr == nil {
		t.Fatal("expected non-nil error")
	}
	if mcpErr.Code != ErrCodeNoAdaptersMatched {
		t.Errorf("code: got %d, want %d", mcpErr.Code, ErrCodeNoAdaptersMatched)
	}
}

// TestMapOrchestratorErrorNoAdaptersWithCategory verifies category extraction
// from result.
func TestMapOrchestratorErrorNoAdaptersWithCategory(t *testing.T) {
	err := fmt.Errorf("orchestrator: no adapters matched")
	result := &orchestrator.SearchResult{Category: "technical"}
	mcpErr := mapOrchestratorError(err, result)
	if mcpErr.Data["query_category"] != "technical" {
		t.Errorf("query_category: got %v, want technical", mcpErr.Data["query_category"])
	}
}

// TestMapOrchestratorErrorAllAdaptersFailed verifies the "all adapters failed" branch.
func TestMapOrchestratorErrorAllAdaptersFailed(t *testing.T) {
	err := fmt.Errorf("orchestrator: all adapters failed")
	mcpErr := mapOrchestratorError(err, nil)
	if mcpErr == nil {
		t.Fatal("expected non-nil error")
	}
	if mcpErr.Code != ErrCodeAllAdaptersFailed {
		t.Errorf("code: got %d, want %d", mcpErr.Code, ErrCodeAllAdaptersFailed)
	}
}

// TestMapOrchestratorErrorAllAdaptersWithErrors verifies error extraction from
// the AdapterErrors map.
func TestMapOrchestratorErrorAllAdaptersWithErrors(t *testing.T) {
	err := fmt.Errorf("orchestrator: all adapters failed")
	result := &orchestrator.SearchResult{
		AdapterErrors: map[string]error{
			"reddit": fmt.Errorf("timeout"),
			"hn":     fmt.Errorf("500"),
		},
	}
	mcpErr := mapOrchestratorError(err, result)
	errs, ok := mcpErr.Data["errors"].([]string)
	if !ok {
		t.Fatalf("expected []string in data.errors, got %T", mcpErr.Data["errors"])
	}
	if len(errs) != 2 {
		t.Errorf("expected 2 error strings, got %d", len(errs))
	}
}

// TestMapOrchestratorErrorGeneric verifies the default branch for unknown errors.
func TestMapOrchestratorErrorGeneric(t *testing.T) {
	err := fmt.Errorf("something unexpected")
	mcpErr := mapOrchestratorError(err, nil)
	if mcpErr == nil {
		t.Fatal("expected non-nil error")
	}
	if mcpErr.Code != -32603 {
		t.Errorf("code: got %d, want -32603", mcpErr.Code)
	}
}
