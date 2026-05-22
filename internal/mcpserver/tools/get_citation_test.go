package tools

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// TestGetCitationResolvesValidDocID verifies that get_citation resolves a
// valid doc_id to its full citation (REQ-MCP-012).
func TestGetCitationResolvesValidDocID(t *testing.T) {
	cache := NewDocCache()
	now := time.Now()
	cache.Store([]types.NormalizedDoc{
		{
			ID:          "doc-123",
			Title:       "Test Document",
			URL:         "https://example.com/doc",
			SourceID:    "reddit",
			Snippet:     "A test snippet",
			Score:       0.95,
			RetrievedAt: now,
		},
	})

	handler := GetCitationHandler(cache)
	_, output, err := handler(context.Background(), nil, GetCitationInput{DocID: "doc-123"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.DocID != "doc-123" {
		t.Errorf("doc_id: got %q, want doc-123", output.DocID)
	}
	if output.Title != "Test Document" {
		t.Errorf("title: got %q, want 'Test Document'", output.Title)
	}
	if output.URL != "https://example.com/doc" {
		t.Errorf("url: got %q", output.URL)
	}
	if output.Source != "reddit" {
		t.Errorf("source: got %q, want reddit", output.Source)
	}
	if output.Score != 0.95 {
		t.Errorf("score: got %f, want 0.95", output.Score)
	}
}

// TestGetCitationNotFoundError verifies that an unknown doc_id returns a
// citation_not_found error (REQ-MCP-012).
func TestGetCitationNotFoundError(t *testing.T) {
	cache := NewDocCache() // empty cache

	handler := GetCitationHandler(cache)
	_, _, err := handler(context.Background(), nil, GetCitationInput{DocID: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for unknown doc_id, got nil")
	}

	// Verify it's a citation not found error.
	if !errors.Is(err, ErrCitationNotFound) {
		t.Errorf("expected ErrCitationNotFound, got: %v", err)
	}
}
