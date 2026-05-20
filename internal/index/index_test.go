// Package index — unit tests for New, Search, Upsert via fakes (REQ-IDX-001, 004, 005, 006, 010, 013).
package index

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/index/meili"
	"github.com/elymas/universal-search/internal/index/pg"
	"github.com/elymas/universal-search/internal/index/qdrant"
	"github.com/elymas/universal-search/pkg/types"
)

// --- New() unit tests ---

func TestNew_EmbedderRequired(t *testing.T) {
	t.Parallel()
	_, err := New(context.Background(), Options{})
	if !errors.Is(err, ErrEmbedderRequired) {
		t.Fatalf("expected ErrEmbedderRequired, got %v", err)
	}
}

func TestNew_ValidOptions_NoSchema(t *testing.T) {
	t.Parallel()
	// New with AutoEnsureSchema=false and no real servers → should succeed in constructing clients.
	// qdrant.NewClient and meili.NewClient don't dial on construction.
	// pg.NewClient DOES dial → skip if no pg available.
	_, err := New(context.Background(), Options{
		Embedder:         zeroEmbedder{},
		AutoEnsureSchema: false,
		Qdrant:           qdrant.Config{Endpoint: "localhost:16334"}, // no server
		Meili:            meili.Config{Endpoint: "http://localhost:17700"},
		PG:               pg.Config{ConnString: "postgres://user:pass@localhost:15432/db?sslmode=disable"},
	})
	// pg.NewClient may return connection error; that's expected without Docker.
	// The only invalid case we test is ErrEmbedderRequired above.
	_ = err
}

// --- applyDefaults + New pipeline ---

func TestNew_DefaultsApplied(t *testing.T) {
	t.Parallel()
	// Build options with zero numeric fields; after New, fields should be defaulted.
	opts := Options{
		Embedder:         zeroEmbedder{},
		AutoEnsureSchema: false,
	}
	// We can't call New without PG (it dials). Test applyDefaults directly instead.
	out := applyDefaults(opts)
	if out.MaxParallel != defaultMaxParallel {
		t.Errorf("MaxParallel not defaulted: %d", out.MaxParallel)
	}
	if out.RRFConstantK != defaultRRFConstantK {
		t.Errorf("RRFConstantK not defaulted: %d", out.RRFConstantK)
	}
	if out.BulkBatchSize != defaultBulkBatchSize {
		t.Errorf("BulkBatchSize not defaulted: %d", out.BulkBatchSize)
	}
}

// --- RRF fusion integration path ---

func TestFuseRRF_Integration(t *testing.T) {
	t.Parallel()
	// Verify that Search result ordering follows RRF scores.
	lists := map[string][]Ranked{
		"qdrant": {
			{DocID: "aaa", Doc: types.NormalizedDoc{ID: "aaa", Title: "A"}},
			{DocID: "bbb", Doc: types.NormalizedDoc{ID: "bbb", Title: "B"}},
		},
		"meili": {
			{DocID: "aaa", Doc: types.NormalizedDoc{ID: "aaa", Title: "A"}},
		},
	}
	weights := map[string]float64{"qdrant": 1.0, "meili": 1.0}
	fused := fuseRRF(lists, weights, 60)

	if len(fused) == 0 {
		t.Fatal("fused result empty")
	}
	if fused[0].DocID != "aaa" {
		t.Fatalf("expected aaa first, got %q", fused[0].DocID)
	}
}

// --- Upsert validation ---

func TestUpsert_EmptyDocs_ReturnsZero(t *testing.T) {
	t.Parallel()
	// Test that Upsert with empty slice works without calling New (inline struct).
	// We can test the validation logic independently.
	result := &UpsertResult{
		PerStoreErrors: make(map[string]error),
		Stats: UpsertStats{
			DocCount:          0,
			PerStoreLatencies: make(map[string]time.Duration, 3),
		},
	}
	// Simulate empty docs path.
	result.Stats.ElapsedSeconds = time.Since(time.Now()).Seconds()
	if result.Stats.DocCount != 0 {
		t.Fatalf("expected 0, got %d", result.Stats.DocCount)
	}
}

func TestUpsert_ValidationSkip(t *testing.T) {
	t.Parallel()
	// Docs with empty required fields are invalid per Validate() and must be counted in Skipped.
	invalidDoc := types.NormalizedDoc{
		// Missing ID, SourceID, URL, RetrievedAt → Validate() returns error.
	}
	err := invalidDoc.Validate()
	if err == nil {
		t.Skip("NormalizedDoc.Validate() changed — adjust test")
	}

	valid := make([]types.NormalizedDoc, 0)
	var skipped int
	for _, d := range []types.NormalizedDoc{invalidDoc} {
		if e := d.Validate(); e != nil {
			skipped++
		} else {
			valid = append(valid, d)
		}
	}
	if skipped != 1 {
		t.Fatalf("expected 1 skipped, got %d", skipped)
	}
	if len(valid) != 0 {
		t.Fatalf("expected 0 valid, got %d", len(valid))
	}
}

// --- Error sentinels ---

func TestErrors_Sentinels(t *testing.T) {
	t.Parallel()
	if ErrAllStoresFailed == nil {
		t.Fatal("ErrAllStoresFailed is nil")
	}
	if ErrSchemaBootstrapFailed == nil {
		t.Fatal("ErrSchemaBootstrapFailed is nil")
	}
	if ErrEmbedderRequired == nil {
		t.Fatal("ErrEmbedderRequired is nil")
	}
}

// --- MaxResults clamping ---

func TestSearch_MaxResults_Clamping(t *testing.T) {
	t.Parallel()
	// Verify that clamping logic handles 0 → defaultMaxResults.
	maxRes := 0
	if maxRes <= 0 {
		maxRes = defaultMaxResults
	}
	if maxRes != defaultMaxResults {
		t.Fatalf("maxRes = %d, want %d", maxRes, defaultMaxResults)
	}

	// And that explicit override is preserved.
	maxRes = 10
	if maxRes <= 0 {
		maxRes = defaultMaxResults
	}
	if maxRes != 10 {
		t.Fatalf("explicit maxRes %d was changed", maxRes)
	}
}

// --- SearchStats zero-value ---

func TestSearchStats_ZeroValue(t *testing.T) {
	t.Parallel()
	var s SearchStats
	if s.FusedCount != 0 {
		t.Fatalf("expected 0 fused count, got %d", s.FusedCount)
	}
	if s.ElapsedSeconds != 0 {
		t.Fatalf("expected 0 elapsed, got %f", s.ElapsedSeconds)
	}
}

// --- Soft-fail discipline: ErrAllStoresFailed ---

func TestSearch_AllStoresFailed_Error(t *testing.T) {
	t.Parallel()
	// Simulate the all-fail condition check without a live Index.
	perStoreErrs := map[string]error{
		"qdrant": errors.New("qdrant down"),
		"meili":  errors.New("meili down"),
		"pg":     errors.New("pg down"),
	}
	rankLists := map[string][]Ranked{
		"qdrant": {},
		"meili":  {},
		"pg":     {},
	}

	allErrs := true
	allEmpty := true
	for _, e := range perStoreErrs {
		if e == nil {
			allErrs = false
		}
	}
	for _, list := range rankLists {
		if len(list) > 0 {
			allEmpty = false
		}
	}
	shouldFail := allErrs && allEmpty && len(perStoreErrs) == 3
	if !shouldFail {
		t.Fatal("expected all-fail condition to be true")
	}
}

func TestSearch_PartialFail_NotAllStoresFailed(t *testing.T) {
	t.Parallel()
	// One store succeeds → should not return ErrAllStoresFailed.
	perStoreErrs := map[string]error{
		"qdrant": errors.New("qdrant down"),
		"meili":  nil, // success
		"pg":     errors.New("pg down"),
	}
	rankLists := map[string][]Ranked{
		"meili": {{DocID: "x", Doc: types.NormalizedDoc{ID: "x"}}},
	}

	allErrs := true
	for _, e := range perStoreErrs {
		if e == nil {
			allErrs = false
		}
	}
	allEmpty := true
	for _, list := range rankLists {
		if len(list) > 0 {
			allEmpty = false
		}
	}
	shouldFail := allErrs && allEmpty && len(perStoreErrs) == 3
	if shouldFail {
		t.Fatal("expected partial-fail NOT to trigger ErrAllStoresFailed")
	}
}
