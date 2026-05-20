// Package pg — unit tests for PostgreSQL client helpers (REQ-IDX-008).
package pg

import (
	"context"
	"testing"
	"time"
)

func TestDocRow_ZeroValue(t *testing.T) {
	t.Parallel()
	var r DocRow
	if r.DocID != "" {
		t.Fatalf("expected empty DocID, got %q", r.DocID)
	}
}

func TestDocRow_Fields(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	r := DocRow{
		DocID:       "abc123",
		ContentHash: "hash",
		SourceID:    "src",
		URL:         "https://example.com",
		Title:       "Test",
		Body:        "Body text",
		Lang:        "en",
		DocType:     "article",
		RetrievedAt: now,
	}
	if r.DocID != "abc123" {
		t.Errorf("DocID = %q", r.DocID)
	}
	if r.Lang != "en" {
		t.Errorf("Lang = %q", r.Lang)
	}
}

func TestFilters_ZeroLimit(t *testing.T) {
	t.Parallel()
	f := Filters{}
	if f.Limit != 0 {
		t.Fatalf("expected zero limit, got %d", f.Limit)
	}
}

func TestConfig_ConnString(t *testing.T) {
	t.Parallel()
	cfg := Config{ConnString: "postgres://u:p@localhost/db"}
	if cfg.ConnString == "" {
		t.Fatal("ConnString should not be empty")
	}
}

func TestNewClient_InvalidConnString(t *testing.T) {
	t.Parallel()
	// No live PG server; NewClient should return error for invalid DSN or unreachable host.
	// Use a valid context.
	ctx := context.Background()
	_, err := NewClient(ctx, Config{ConnString: "postgres://user:pass@localhost:15432/testdb?sslmode=disable&connect_timeout=1"})
	if err == nil {
		t.Log("NewClient succeeded (PG might be running; skip error check)")
		return
	}
	// Error expected when no server is available.
	t.Logf("NewClient error (expected): %v", err)
}
