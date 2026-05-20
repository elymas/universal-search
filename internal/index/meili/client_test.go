// Package meili — unit tests for Meilisearch client helpers (REQ-IDX-003, REQ-IDX-010).
package meili

import (
	"testing"
)

func TestNewClient_DefaultIndexName(t *testing.T) {
	t.Parallel()
	c, err := NewClient(Config{Endpoint: "http://localhost:17700"})
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}
	if c.IndexName() != "usearch_docs" {
		t.Errorf("IndexName = %q, want %q", c.IndexName(), "usearch_docs")
	}
}

func TestNewClient_CustomIndexName(t *testing.T) {
	t.Parallel()
	c, err := NewClient(Config{Endpoint: "http://localhost:17700", IndexName: "custom_idx"})
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}
	if c.IndexName() != "custom_idx" {
		t.Errorf("IndexName = %q, want %q", c.IndexName(), "custom_idx")
	}
}

func TestClose_NoOp(t *testing.T) {
	t.Parallel()
	c, _ := NewClient(Config{Endpoint: "http://localhost:17700"})
	if err := c.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
}

func TestDocument_IsMap(t *testing.T) {
	t.Parallel()
	d := Document{"doc_id": "abc", "title": "test"}
	if d["doc_id"] != "abc" {
		t.Fatalf("unexpected value: %v", d["doc_id"])
	}
}
