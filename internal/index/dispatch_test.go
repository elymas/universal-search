// Package index — unit tests for dispatch helpers (REQ-IDX-006, REQ-IDX-009).
package index

import (
	"context"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/index/meili"
	"github.com/elymas/universal-search/internal/index/pg"
	"github.com/elymas/universal-search/internal/index/qdrant"
	"github.com/elymas/universal-search/pkg/types"
)

// --- deriveStoreCtx ---

func TestDeriveStoreCtx_PerStoreBound(t *testing.T) {
	t.Parallel()
	idx := &Index{
		opts: applyDefaults(Options{Embedder: zeroEmbedder{}}),
	}
	ctx, cancel := idx.deriveStoreCtx(context.Background(), "qdrant")
	defer cancel()

	dl, ok := ctx.Deadline()
	if !ok {
		t.Fatal("no deadline set on store context")
	}
	remaining := time.Until(dl)
	if remaining > 200*time.Millisecond+5*time.Millisecond {
		t.Errorf("qdrant ctx deadline too far: %v", remaining)
	}
}

func TestDeriveStoreCtx_ParentDeadlineShorter(t *testing.T) {
	t.Parallel()
	idx := &Index{
		opts: applyDefaults(Options{Embedder: zeroEmbedder{}}),
	}
	// Parent deadline in 50ms (less than qdrant default 200ms).
	parent, parentCancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer parentCancel()

	ctx, cancel := idx.deriveStoreCtx(parent, "qdrant")
	defer cancel()

	dl, ok := ctx.Deadline()
	if !ok {
		t.Fatal("no deadline")
	}
	remaining := time.Until(dl)
	if remaining > 55*time.Millisecond {
		t.Errorf("store ctx should use parent deadline (~50ms), got %v", remaining)
	}
}

func TestDeriveStoreCtx_ImmediateCancel_WhenRemaining0(t *testing.T) {
	t.Parallel()
	idx := &Index{
		opts: applyDefaults(Options{Embedder: zeroEmbedder{}}),
	}
	// Parent already past deadline.
	parent, parentCancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer parentCancel()
	time.Sleep(5 * time.Millisecond) // ensure parent is expired

	ctx, cancel := idx.deriveStoreCtx(parent, "qdrant")
	defer cancel()

	select {
	case <-ctx.Done():
		// Expected: immediately done.
	default:
		t.Fatal("expected context to be immediately cancelled")
	}
}

// --- Filter builders ---

func TestBuildQdrantFilter_Nil(t *testing.T) {
	t.Parallel()
	f := buildQdrantFilter(IndexQuery{})
	if f != nil {
		t.Fatalf("expected nil filter for empty query, got %+v", f)
	}
}

func TestBuildQdrantFilter_WithSourceID(t *testing.T) {
	t.Parallel()
	f := buildQdrantFilter(IndexQuery{SourceID: "src1"})
	if f == nil {
		t.Fatal("expected non-nil filter")
	}
	if f.SourceID != "src1" {
		t.Errorf("SourceID = %q, want %q", f.SourceID, "src1")
	}
}

func TestBuildMeiliFilter_Empty(t *testing.T) {
	t.Parallel()
	s := buildMeiliFilter(IndexQuery{})
	if s != "" {
		t.Fatalf("expected empty filter string, got %q", s)
	}
}

func TestBuildMeiliFilter_WithLang(t *testing.T) {
	t.Parallel()
	s := buildMeiliFilter(IndexQuery{Lang: "ko"})
	if s != `lang = "ko"` {
		t.Errorf("filter = %q, want %q", s, `lang = "ko"`)
	}
}

func TestBuildMeiliFilter_Multi(t *testing.T) {
	t.Parallel()
	s := buildMeiliFilter(IndexQuery{SourceID: "s1", Lang: "en"})
	// Both conditions must appear; order may vary.
	if len(s) == 0 {
		t.Fatal("expected non-empty filter")
	}
}

func TestBuildPGFilters_Limit(t *testing.T) {
	t.Parallel()
	f := buildPGFilters(IndexQuery{}, 25)
	if f.Limit != 25 {
		t.Errorf("Limit = %d, want 25", f.Limit)
	}
}

// --- Document converters ---

func TestDocsToQdrantPoints_Length(t *testing.T) {
	t.Parallel()
	docs := []types.NormalizedDoc{
		{ID: "1", SourceID: "s", URL: "u", Title: "T", Body: "B", RetrievedAt: time.Now()},
		{ID: "2", SourceID: "s", URL: "u2", Title: "T2", Body: "B2", RetrievedAt: time.Now()},
	}
	points := docsToQdrantPoints(docs, zeroEmbedder{})
	if len(points) != 2 {
		t.Fatalf("expected 2 points, got %d", len(points))
	}
}

func TestDocsToQdrantPoints_IDDeterminism(t *testing.T) {
	t.Parallel()
	doc := types.NormalizedDoc{ID: "1", SourceID: "src", URL: "http://x.com", Title: "T", RetrievedAt: time.Now()}
	pts1 := docsToQdrantPoints([]types.NormalizedDoc{doc}, zeroEmbedder{})
	pts2 := docsToQdrantPoints([]types.NormalizedDoc{doc}, zeroEmbedder{})
	if pts1[0].ID != pts2[0].ID {
		t.Fatalf("non-deterministic ID: %q != %q", pts1[0].ID, pts2[0].ID)
	}
}

func TestDocsToMeiliDocs_Length(t *testing.T) {
	t.Parallel()
	docs := []types.NormalizedDoc{
		{ID: "1", SourceID: "s", URL: "u", Title: "T", RetrievedAt: time.Now()},
	}
	mdocs := docsToMeiliDocs(docs)
	if len(mdocs) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(mdocs))
	}
	if _, ok := mdocs[0]["doc_id"]; !ok {
		t.Fatal("meili doc missing doc_id")
	}
}

func TestDocsToPGRows_Length(t *testing.T) {
	t.Parallel()
	docs := []types.NormalizedDoc{
		{ID: "1", SourceID: "s", URL: "u", Title: "T", RetrievedAt: time.Now()},
		{ID: "2", SourceID: "s2", URL: "u2", Title: "T2", RetrievedAt: time.Now()},
	}
	rows := docsToPGRows(docs)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
}

func TestPayloadToDoc_StringExtraction(t *testing.T) {
	t.Parallel()
	payload := map[string]any{
		"source_id": "src-x",
		"url":       "https://x.com",
		"title":     "Test Title",
		"lang":      "en",
		"doc_type":  "article",
	}
	doc := payloadToDoc("test-id", payload)
	if doc.ID != "test-id" {
		t.Errorf("ID = %q, want %q", doc.ID, "test-id")
	}
	if doc.SourceID != "src-x" {
		t.Errorf("SourceID = %q, want %q", doc.SourceID, "src-x")
	}
}

func TestPGRowToNormalizedDoc_PublishedAt(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC().Truncate(time.Second)
	row := pg.DocRow{
		DocID:       "d",
		SourceID:    "s",
		URL:         "u",
		RetrievedAt: now,
		PublishedAt: &now,
	}
	doc := pgRowToNormalizedDoc(row)
	if doc.PublishedAt.IsZero() {
		t.Fatal("PublishedAt should not be zero")
	}
}

func TestMeiliDocToNormalizedDoc(t *testing.T) {
	t.Parallel()
	d := meili.Document{
		"doc_id":    "mid",
		"source_id": "s",
		"title":     "Title",
		"body":      "Body text",
		"lang":      "ja",
	}
	doc := meiliDocToNormalizedDoc(d)
	if doc.ID != "mid" {
		t.Errorf("ID = %q, want %q", doc.ID, "mid")
	}
	if doc.Body != "Body text" {
		t.Errorf("Body = %q", doc.Body)
	}
}

// --- Ranked converters ---

func TestScoredPointsToRanked(t *testing.T) {
	t.Parallel()
	pts := []qdrant.ScoredPoint{
		{ID: "aabbccdd11223344", Score: 0.9, Payload: map[string]any{"source_id": "s", "title": "T"}},
	}
	ranked := scoredPointsToRanked(pts)
	if len(ranked) != 1 {
		t.Fatalf("expected 1, got %d", len(ranked))
	}
	if ranked[0].DocID != "aabbccdd11223344" {
		t.Errorf("DocID = %q", ranked[0].DocID)
	}
}

func TestMeiliDocsToRanked(t *testing.T) {
	t.Parallel()
	docs := []meili.Document{
		{"doc_id": "id1", "title": "Title"},
	}
	ranked := meiliDocsToRanked(docs)
	if len(ranked) != 1 {
		t.Fatalf("expected 1, got %d", len(ranked))
	}
	if ranked[0].DocID != "id1" {
		t.Errorf("DocID = %q", ranked[0].DocID)
	}
}

func TestPGRowsToRanked(t *testing.T) {
	t.Parallel()
	rows := []pg.DocRow{
		{DocID: "pgid1", SourceID: "s", URL: "u", RetrievedAt: time.Now()},
	}
	ranked := pgRowsToRanked(rows)
	if len(ranked) != 1 {
		t.Fatalf("expected 1, got %d", len(ranked))
	}
	if ranked[0].DocID != "pgid1" {
		t.Errorf("DocID = %q", ranked[0].DocID)
	}
}

func TestBuildPGFilters_WithSince(t *testing.T) {
	t.Parallel()
	since := time.Now().Add(-24 * time.Hour)
	f := buildPGFilters(IndexQuery{Since: since}, 10)
	if f.Since == nil {
		t.Fatal("Since should not be nil")
	}
}

func TestBuildPGFilters_WithUntil(t *testing.T) {
	t.Parallel()
	until := time.Now()
	f := buildPGFilters(IndexQuery{Until: until}, 10)
	if f.Until == nil {
		t.Fatal("Until should not be nil")
	}
}

func TestDocsToQdrantPoints_PublishedAt(t *testing.T) {
	t.Parallel()
	now := time.Now()
	pub := now.Add(-time.Hour)
	doc := types.NormalizedDoc{
		ID:          "1",
		SourceID:    "s",
		URL:         "u",
		Title:       "T",
		RetrievedAt: now,
		PublishedAt: pub,
	}
	pts := docsToQdrantPoints([]types.NormalizedDoc{doc}, zeroEmbedder{})
	if pts[0].Payload["published_at"] == nil {
		t.Fatal("published_at should be set")
	}
}

func TestDocsToPGRows_PublishedAtZero(t *testing.T) {
	t.Parallel()
	doc := types.NormalizedDoc{
		ID:          "1",
		SourceID:    "s",
		URL:         "u",
		RetrievedAt: time.Now(),
		// PublishedAt zero
	}
	rows := docsToPGRows([]types.NormalizedDoc{doc})
	if rows[0].PublishedAt != nil {
		t.Fatal("PublishedAt should be nil when zero")
	}
}

// --- qdrant filter builder ---

func TestBuildQdrantFilter_AllFields(t *testing.T) {
	t.Parallel()
	q := IndexQuery{SourceID: "s", Lang: "ko", TeamID: "t"}
	f := buildQdrantFilter(q)
	if f == nil {
		t.Fatal("expected non-nil filter")
	}
	fq := &qdrant.Filter{SourceID: "s", Lang: "ko", TeamID: "t"}
	if f.SourceID != fq.SourceID || f.Lang != fq.Lang || f.TeamID != fq.TeamID {
		t.Errorf("filter mismatch: got %+v", f)
	}
}
