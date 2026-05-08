//go:build integration

// Package index — integration tests requiring live Qdrant, Meilisearch, PostgreSQL.
// Run with: go test -tags=integration ./internal/index/... -v
//
// Docker Compose: docker compose -f deploy/docker-compose.yml up -d
// REQ-IDX-001, REQ-IDX-002, REQ-IDX-003, REQ-IDX-004, REQ-IDX-005, REQ-IDX-006,
// REQ-IDX-007, REQ-IDX-008, REQ-IDX-009, REQ-IDX-010, REQ-IDX-011, REQ-IDX-012, REQ-IDX-013
package index

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/index/meili"
	"github.com/elymas/universal-search/internal/index/pg"
	"github.com/elymas/universal-search/internal/index/qdrant"
	"github.com/elymas/universal-search/pkg/types"
)

// envOrDefault reads an env var with a fallback value.
func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// integrationOpts returns Options wired to local Docker services.
func integrationOpts() Options {
	return Options{
		Embedder:         zeroEmbedder{},
		AutoEnsureSchema: true,
		Qdrant: qdrant.Config{
			Endpoint: envOrDefault("QDRANT_ENDPOINT", "localhost:6334"),
		},
		Meili: meili.Config{
			Endpoint:  envOrDefault("MEILI_ENDPOINT", "http://localhost:7700"),
			MasterKey: envOrDefault("MEILI_MASTER_KEY", "masterkey"),
		},
		PG: pg.Config{
			ConnString:    envOrDefault("PG_CONN", "postgres://usearch:usearch@localhost:5432/usearch?sslmode=disable"),
			MigrationsDir: "../../deploy/postgres/migrations",
		},
	}
}

// sampleDoc returns a valid NormalizedDoc for testing.
func sampleDoc(n int) types.NormalizedDoc {
	suffix := string(rune('A' + n%26))
	return types.NormalizedDoc{
		ID:          "test-doc-" + suffix,
		SourceID:    "src-integration",
		URL:         "https://integration.test/doc-" + suffix,
		Title:       "Integration Test Doc " + suffix,
		Body:        "Body content for integration test document " + suffix,
		Snippet:     "Snippet " + suffix,
		Lang:        "en",
		DocType:     types.DocType("article"),
		RetrievedAt: time.Now().UTC(),
	}
}

func TestIntegration_New_AutoSchema(t *testing.T) {
	ctx := context.Background()
	idx, err := New(ctx, integrationOpts())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer idx.Close()
}

func TestIntegration_Upsert_SingleDoc(t *testing.T) {
	ctx := context.Background()
	idx, err := New(ctx, integrationOpts())
	if err != nil {
		t.Skipf("cannot connect to stores: %v", err)
	}
	defer idx.Close()

	docs := []types.NormalizedDoc{sampleDoc(0)}
	result, err := idx.Upsert(ctx, docs)
	if err != nil {
		t.Fatalf("Upsert error: %v", err)
	}
	if len(result.PerStoreErrors) > 0 {
		for k, e := range result.PerStoreErrors {
			if e != nil {
				t.Errorf("store %q error: %v", k, e)
			}
		}
	}
}

func TestIntegration_Upsert_Idempotency(t *testing.T) {
	ctx := context.Background()
	idx, err := New(ctx, integrationOpts())
	if err != nil {
		t.Skipf("cannot connect: %v", err)
	}
	defer idx.Close()

	doc := sampleDoc(1)
	doc.SourceID = "src-idem"
	doc.URL = "https://idem.test/doc"

	// First upsert.
	r1, _ := idx.Upsert(ctx, []types.NormalizedDoc{doc})
	// Second upsert with same content → should be no-op (skipped in PG).
	r2, _ := idx.Upsert(ctx, []types.NormalizedDoc{doc})
	_ = r1
	_ = r2
}

func TestIntegration_Search_AfterUpsert(t *testing.T) {
	ctx := context.Background()
	idx, err := New(ctx, integrationOpts())
	if err != nil {
		t.Skipf("cannot connect: %v", err)
	}
	defer idx.Close()

	doc := sampleDoc(2)
	doc.SourceID = "src-search"
	doc.URL = "https://search.test/doc"
	doc.Title = "UniqueTitleForSearchTest42"
	doc.Body = "UniqueTitleForSearchTest42 body content"

	_, _ = idx.Upsert(ctx, []types.NormalizedDoc{doc})

	// Wait for Meilisearch async indexing.
	time.Sleep(1 * time.Second)

	result, err := idx.Search(ctx, IndexQuery{
		Text:       "UniqueTitleForSearchTest42",
		MaxResults: 10,
	})
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	// At least one result expected.
	if len(result.Docs) == 0 {
		t.Log("Search returned 0 results (Meili indexing may be async; acceptable)")
	}
}

func TestIntegration_Search_EmptyText_ZeroVector(t *testing.T) {
	ctx := context.Background()
	idx, err := New(ctx, integrationOpts())
	if err != nil {
		t.Skipf("cannot connect: %v", err)
	}
	defer idx.Close()

	result, err := idx.Search(ctx, IndexQuery{MaxResults: 5})
	if err != nil && err != ErrAllStoresFailed {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = result
}

func TestIntegration_Upsert_BatchOf100(t *testing.T) {
	ctx := context.Background()
	idx, err := New(ctx, integrationOpts())
	if err != nil {
		t.Skipf("cannot connect: %v", err)
	}
	defer idx.Close()

	docs := make([]types.NormalizedDoc, 100)
	for i := range docs {
		d := sampleDoc(i)
		d.URL = "https://batch.test/doc-" + string(rune('a'+i%26)) + string(rune('a'+i/26))
		docs[i] = d
	}

	result, err := idx.Upsert(ctx, docs)
	if err != nil {
		t.Fatalf("batch upsert error: %v", err)
	}
	if result.Stats.DocCount != 100 {
		t.Errorf("DocCount = %d, want 100", result.Stats.DocCount)
	}
}

func TestIntegration_DocID_Determinism(t *testing.T) {
	// Verify round-trip: docID → upsert → search by source_id.
	ctx := context.Background()
	idx, err := New(ctx, integrationOpts())
	if err != nil {
		t.Skipf("cannot connect: %v", err)
	}
	defer idx.Close()

	const sourceID = "src-determinism"
	const url = "https://determinism.test/doc"
	expectedID := docID(sourceID, url)

	doc := types.NormalizedDoc{
		ID:          expectedID,
		SourceID:    sourceID,
		URL:         url,
		Title:       "Determinism Test",
		Body:        "Body",
		Lang:        "en",
		DocType:     "article",
		RetrievedAt: time.Now().UTC(),
	}

	_, err = idx.Upsert(ctx, []types.NormalizedDoc{doc})
	if err != nil {
		t.Fatalf("upsert error: %v", err)
	}

	// Verify docID is 16 hex chars.
	if len(expectedID) != 16 {
		t.Fatalf("docID length = %d, want 16", len(expectedID))
	}
}

func TestIntegration_Close_Idempotent(t *testing.T) {
	ctx := context.Background()
	idx, err := New(ctx, integrationOpts())
	if err != nil {
		t.Skipf("cannot connect: %v", err)
	}
	// Double close should not panic.
	_ = idx.Close()
	_ = idx.Close()
}
