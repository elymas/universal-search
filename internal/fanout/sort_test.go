package fanout

import (
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// makeSortDoc builds a NormalizedDoc with specified score, sourceID, and retrievedAt offset.
func makeSortDoc(id string, score float64, sourceID string, offsetSec int) types.NormalizedDoc {
	return types.NormalizedDoc{
		ID:          id,
		SourceID:    sourceID,
		URL:         "https://example.com/" + id,
		Title:       "title " + id,
		Body:        "body",
		RetrievedAt: time.Unix(1_000_000+int64(offsetSec), 0),
		Score:       score,
	}
}

// TestSortPrimaryScoreDescending verifies higher-score docs come first.
func TestSortPrimaryScoreDescending(t *testing.T) {
	t.Parallel()
	docs := []types.NormalizedDoc{
		makeSortDoc("low", 0.2, "src", 0),
		makeSortDoc("high", 0.9, "src", 0),
		makeSortDoc("mid", 0.5, "src", 0),
	}
	sortDocs(docs)
	wantOrder := []string{"high", "mid", "low"}
	for i, want := range wantOrder {
		if docs[i].ID != want {
			t.Errorf("docs[%d].ID = %q, want %q", i, docs[i].ID, want)
		}
	}
}

// TestSortSecondaryAdapterAscending verifies SourceID ascending as tie-breaker.
func TestSortSecondaryAdapterAscending(t *testing.T) {
	t.Parallel()
	docs := []types.NormalizedDoc{
		makeSortDoc("z-doc", 0.5, "zzz", 0),
		makeSortDoc("a-doc", 0.5, "aaa", 0),
		makeSortDoc("m-doc", 0.5, "mmm", 0),
	}
	sortDocs(docs)
	wantOrder := []string{"a-doc", "m-doc", "z-doc"}
	for i, want := range wantOrder {
		if docs[i].ID != want {
			t.Errorf("docs[%d].ID = %q, want %q", i, docs[i].ID, want)
		}
	}
}

// TestSortTertiaryRetrievedAtDescending verifies newer RetrievedAt wins same score+sourceID.
func TestSortTertiaryRetrievedAtDescending(t *testing.T) {
	t.Parallel()
	docs := []types.NormalizedDoc{
		makeSortDoc("old", 0.5, "src", 100), // older
		makeSortDoc("new", 0.5, "src", 200), // newer
		makeSortDoc("mid", 0.5, "src", 150),
	}
	sortDocs(docs)
	wantOrder := []string{"new", "mid", "old"}
	for i, want := range wantOrder {
		if docs[i].ID != want {
			t.Errorf("docs[%d].ID = %q, want %q (RetrievedAt desc)", i, docs[i].ID, want)
		}
	}
}

// TestSortStableForEqualKeys verifies sort.SliceStable preserves order for equal 3-key tuples.
func TestSortStableForEqualKeys(t *testing.T) {
	t.Parallel()
	// All docs have equal score, sourceID, and retrievedAt.
	ts := time.Unix(1_000_000, 0)
	docs := []types.NormalizedDoc{
		{ID: "first", SourceID: "src", URL: "https://example.com/1", Score: 0.5, RetrievedAt: ts, Title: "t", Body: "b"},
		{ID: "second", SourceID: "src", URL: "https://example.com/2", Score: 0.5, RetrievedAt: ts, Title: "t", Body: "b"},
		{ID: "third", SourceID: "src", URL: "https://example.com/3", Score: 0.5, RetrievedAt: ts, Title: "t", Body: "b"},
	}
	sortDocs(docs)
	// Stable sort must preserve original order.
	wantOrder := []string{"first", "second", "third"}
	for i, want := range wantOrder {
		if docs[i].ID != want {
			t.Errorf("docs[%d].ID = %q, want %q (stable order violated)", i, docs[i].ID, want)
		}
	}
}

// TestSortCombined verifies all three sort keys working together.
func TestSortCombined(t *testing.T) {
	t.Parallel()
	docs := []types.NormalizedDoc{
		makeSortDoc("s1-low-old", 0.3, "aaa", 100),
		makeSortDoc("s2-high-mid", 0.9, "zzz", 150),
		makeSortDoc("s3-mid-new", 0.5, "mmm", 200),
		makeSortDoc("s4-high-new", 0.9, "aaa", 200),
	}
	sortDocs(docs)
	// Expected: s4-high-new (0.9, aaa) first, then s2-high-mid (0.9, zzz),
	// then s3-mid-new (0.5, mmm), then s1-low-old (0.3, aaa).
	wantOrder := []string{"s4-high-new", "s2-high-mid", "s3-mid-new", "s1-low-old"}
	for i, want := range wantOrder {
		if docs[i].ID != want {
			t.Errorf("docs[%d].ID = %q, want %q", i, docs[i].ID, want)
		}
	}
}

// TestSortEmptyInput verifies empty slice does not panic.
func TestSortEmptyInput(t *testing.T) {
	t.Parallel()
	sortDocs(nil)
	sortDocs([]types.NormalizedDoc{})
}

// TestSortSingleDoc verifies single-element slice does not change.
func TestSortSingleDoc(t *testing.T) {
	t.Parallel()
	docs := []types.NormalizedDoc{makeSortDoc("only", 0.5, "src", 0)}
	sortDocs(docs)
	if docs[0].ID != "only" {
		t.Fatalf("single-doc sort mutated ID: got %q", docs[0].ID)
	}
}
