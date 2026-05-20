package fanout

import (
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// makeDoc builds a single NormalizedDoc for dedup tests.
func makeDoc(id, sourceID, rawURL string) types.NormalizedDoc {
	return types.NormalizedDoc{
		ID:          id,
		SourceID:    sourceID,
		URL:         rawURL,
		Title:       "title " + id,
		Body:        "body",
		RetrievedAt: time.Now(),
		Score:       0.5,
	}
}

// TestDedupSameURLFirstWins verifies that among duplicate URLs, the first doc is kept.
func TestDedupSameURLFirstWins(t *testing.T) {
	t.Parallel()
	d1 := makeDoc("d1", "s1", "https://example.com/page")
	d2 := makeDoc("d2", "s2", "https://example.com/page")
	d3 := makeDoc("d3", "s1", "https://example.com/other")

	got, dropped := dedupDocs([]types.NormalizedDoc{d1, d2, d3})
	if len(got) != 2 {
		t.Fatalf("want 2 docs, got %d", len(got))
	}
	if dropped != 1 {
		t.Fatalf("want 1 dropped, got %d", dropped)
	}
	// First occurrence wins.
	if got[0].ID != "d1" {
		t.Fatalf("want first doc ID=d1, got %q", got[0].ID)
	}
}

// TestDedupTrackingParamsStripped verifies tracking-param URLs are merged.
func TestDedupTrackingParamsStripped(t *testing.T) {
	t.Parallel()
	d1 := makeDoc("d1", "s1", "https://example.com/page?utm_source=google")
	d2 := makeDoc("d2", "s2", "https://example.com/page?utm_medium=cpc")
	d3 := makeDoc("d3", "s1", "https://example.com/page") // canonical base

	got, dropped := dedupDocs([]types.NormalizedDoc{d1, d2, d3})
	if len(got) != 1 {
		t.Fatalf("want 1 doc (all canonical to same URL), got %d", len(got))
	}
	if dropped != 2 {
		t.Fatalf("want 2 dropped, got %d", dropped)
	}
	if got[0].ID != "d1" {
		t.Fatalf("want first-occurrence d1, got %q", got[0].ID)
	}
}

// TestDedupHashFallbackOnUnparseableURL verifies unparseable URLs fall back to CanonicalHash.
// CanonicalHash includes SourceID+URL+Title+Body, so two identical docs from the same
// source with the same URL will have the same hash and be deduped.
func TestDedupHashFallbackOnUnparseableURL(t *testing.T) {
	t.Parallel()
	// Two docs with identical content from the same source (same hash) → dedup.
	raw := "not-a-valid-url" // no scheme and no host → canonicalURL returns error
	d1 := makeDoc("h1", "s1", raw)
	// d2 is same content as d1 — same SourceID, same URL, same title/body prefix → same hash.
	d2 := types.NormalizedDoc{
		ID:          "h2",
		SourceID:    d1.SourceID, // same source
		URL:         d1.URL,      // same URL
		Title:       d1.Title,    // same title
		Body:        d1.Body,     // same body
		RetrievedAt: d1.RetrievedAt,
		Score:       d1.Score,
	}
	d3 := makeDoc("h3", "s1", "another-not-a-url")

	got, dropped := dedupDocs([]types.NormalizedDoc{d1, d2, d3})
	if len(got) != 2 {
		t.Fatalf("want 2 docs (d1+d3), got %d", len(got))
	}
	if dropped != 1 {
		t.Fatalf("want 1 dropped, got %d", dropped)
	}
	if got[0].ID != "h1" {
		t.Fatalf("want first-occurrence h1, got %q", got[0].ID)
	}
}

// TestDedupMixedValidInvalidURL verifies valid and invalid URLs use disjoint namespaces.
// A valid URL "url:..." and an invalid URL that hashes to the same string
// must NOT be treated as duplicates.
func TestDedupMixedValidInvalidURL(t *testing.T) {
	t.Parallel()
	validDoc := makeDoc("v1", "s1", "https://example.com/page")
	invalidDoc := makeDoc("i1", "s2", "not-a-url")

	got, dropped := dedupDocs([]types.NormalizedDoc{validDoc, invalidDoc})
	if len(got) != 2 {
		t.Fatalf("want 2 docs (disjoint namespaces), got %d", len(got))
	}
	if dropped != 0 {
		t.Fatalf("want 0 dropped, got %d", dropped)
	}
}

// TestDedupKeyDeterministic verifies dedupKey returns the same value on repeated calls.
func TestDedupKeyDeterministic(t *testing.T) {
	t.Parallel()
	d := makeDoc("det1", "src", "https://example.com/page?z=3&a=1&utm_source=x#frag")
	first := dedupKey(d)
	for i := range 10 {
		got := dedupKey(d)
		if got != first {
			t.Fatalf("non-deterministic: call 0=%q, call %d=%q", first, i, got)
		}
	}
}

// TestDedupEmptyInput verifies empty input returns empty output with 0 dropped.
func TestDedupEmptyInput(t *testing.T) {
	t.Parallel()
	got, dropped := dedupDocs(nil)
	if len(got) != 0 {
		t.Fatalf("want 0 docs, got %d", len(got))
	}
	if dropped != 0 {
		t.Fatalf("want 0 dropped, got %d", dropped)
	}
}

// TestDedupNoDuplicates verifies no docs dropped when all URLs are distinct.
func TestDedupNoDuplicates(t *testing.T) {
	t.Parallel()
	docs := []types.NormalizedDoc{
		makeDoc("a", "s1", "https://example.com/a"),
		makeDoc("b", "s1", "https://example.com/b"),
		makeDoc("c", "s1", "https://example.com/c"),
	}
	got, dropped := dedupDocs(docs)
	if len(got) != 3 {
		t.Fatalf("want 3 docs, got %d", len(got))
	}
	if dropped != 0 {
		t.Fatalf("want 0 dropped, got %d", dropped)
	}
}

// TestDedupPreservesOrder verifies that surviving docs preserve input order.
func TestDedupPreservesOrder(t *testing.T) {
	t.Parallel()
	d1 := makeDoc("first", "s1", "https://example.com/a")
	d2 := makeDoc("dup1", "s2", "https://example.com/a")
	d3 := makeDoc("second", "s1", "https://example.com/b")
	d4 := makeDoc("dup2", "s3", "https://example.com/b")
	d5 := makeDoc("third", "s1", "https://example.com/c")

	got, dropped := dedupDocs([]types.NormalizedDoc{d1, d2, d3, d4, d5})
	if len(got) != 3 {
		t.Fatalf("want 3 docs, got %d", len(got))
	}
	if dropped != 2 {
		t.Fatalf("want 2 dropped, got %d", dropped)
	}
	wantIDs := []string{"first", "second", "third"}
	for i, id := range wantIDs {
		if got[i].ID != id {
			t.Errorf("got[%d].ID = %q, want %q", i, got[i].ID, id)
		}
	}
}
