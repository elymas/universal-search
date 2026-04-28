// Package hn — parseHits field mapping tests.
// REQ-ADP2-005: TestParseHitsFieldMapping, TestParseHitsFiltersNonStoryTags, etc.
package hn

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

var fixedRetrievedAt = time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)

// TestParseHitsHappyPath verifies that 25 stories are parsed from the golden fixture.
func TestParseHitsHappyPath(t *testing.T) {
	t.Parallel()
	body := readFixture(t, "testdata/search_response.json")
	docs, _, err := parseHits(body, fixedRetrievedAt)
	if err != nil {
		t.Fatalf("parseHits() unexpected error: %v", err)
	}
	if len(docs) != 25 {
		t.Errorf("len(docs) = %d; want 25", len(docs))
	}
	for i, doc := range docs {
		if verr := doc.Validate(); verr != nil {
			t.Errorf("docs[%d].Validate() = %v", i, verr)
		}
	}
}

// TestParseHitsFieldMapping verifies per-field mapping for specific fixture docs.
func TestParseHitsFieldMapping(t *testing.T) {
	t.Parallel()

	// Test link post (objectID 39000001).
	t.Run("link_post", func(t *testing.T) {
		t.Parallel()
		body := readFixture(t, "testdata/search_response.json")
		docs, _, err := parseHits(body, fixedRetrievedAt)
		if err != nil {
			t.Fatalf("parseHits error: %v", err)
		}
		// First doc in fixture is a link post.
		d := docs[0]
		if d.ID != "39000001" {
			t.Errorf("ID = %q; want %q", d.ID, "39000001")
		}
		if d.SourceID != "hackernews" {
			t.Errorf("SourceID = %q; want %q", d.SourceID, "hackernews")
		}
		if d.URL != "https://example.com/why-go-is-great" {
			t.Errorf("URL = %q; want external URL", d.URL)
		}
		if d.Title != "Why Go is great" {
			t.Errorf("Title = %q; want %q", d.Title, "Why Go is great")
		}
		if d.Author != "pg" {
			t.Errorf("Author = %q; want %q", d.Author, "pg")
		}
		if d.Lang != "" {
			t.Errorf("Lang = %q; want empty", d.Lang)
		}
		if d.DocType != types.DocTypePost {
			t.Errorf("DocType = %v; want DocTypePost", d.DocType)
		}
		if d.Citations != nil {
			t.Errorf("Citations = %v; want nil", d.Citations)
		}
		if d.Hash != "" {
			t.Errorf("Hash = %q; want empty", d.Hash)
		}
		if d.RetrievedAt != fixedRetrievedAt {
			t.Errorf("RetrievedAt = %v; want %v", d.RetrievedAt, fixedRetrievedAt)
		}
		expectedPublishedAt := time.Unix(1713169931, 0).UTC()
		if d.PublishedAt != expectedPublishedAt {
			t.Errorf("PublishedAt = %v; want %v", d.PublishedAt, expectedPublishedAt)
		}
	})

	// Test self-post URL construction via dedicated fixture.
	t.Run("self_post_permalink", func(t *testing.T) {
		t.Parallel()
		body := readFixture(t, "testdata/search_response_self_post.json")
		docs, _, err := parseHits(body, fixedRetrievedAt)
		if err != nil {
			t.Fatalf("parseHits error: %v", err)
		}
		if len(docs) != 1 {
			t.Fatalf("len(docs) = %d; want 1", len(docs))
		}
		d := docs[0]
		if d.URL != "https://news.ycombinator.com/item?id=12345" {
			t.Errorf("URL = %q; want HN permalink", d.URL)
		}
	})

	// Test deleted author (empty author field).
	t.Run("deleted_author", func(t *testing.T) {
		t.Parallel()
		body := readFixture(t, "testdata/search_response_deleted_author.json")
		docs, _, err := parseHits(body, fixedRetrievedAt)
		if err != nil {
			t.Fatalf("parseHits error: %v", err)
		}
		if len(docs) != 1 {
			t.Fatalf("len(docs) = %d; want 1", len(docs))
		}
		d := docs[0]
		if d.Author != "" {
			t.Errorf("Author = %q; want empty (deleted user)", d.Author)
		}
		if verr := d.Validate(); verr != nil {
			t.Errorf("Validate() = %v; want nil (deleted author is valid)", verr)
		}
	})
}

// TestParseHitsFiltersNonStoryTags verifies that hits without "story" in _tags
// are silently skipped.
func TestParseHitsFiltersNonStoryTags(t *testing.T) {
	t.Parallel()
	body := readFixture(t, "testdata/search_response_with_comments.json")
	docs, _, err := parseHits(body, fixedRetrievedAt)
	if err != nil {
		t.Fatalf("parseHits error: %v", err)
	}
	// Fixture has 2 story hits and 2 comment hits; only 2 should be returned.
	if len(docs) != 2 {
		t.Errorf("len(docs) = %d; want 2 (comments filtered)", len(docs))
	}
	for _, d := range docs {
		if d.SourceID != "hackernews" {
			t.Errorf("unexpected doc SourceID = %q", d.SourceID)
		}
	}
}

// TestParseHitsSelfPostUsesPermalink verifies that self-posts (url=="") use the HN permalink.
func TestParseHitsSelfPostUsesPermalink(t *testing.T) {
	t.Parallel()
	body := readFixture(t, "testdata/search_response_self_post.json")
	docs, _, err := parseHits(body, fixedRetrievedAt)
	if err != nil {
		t.Fatalf("parseHits error: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("len(docs) = %d; want 1", len(docs))
	}
	want := "https://news.ycombinator.com/item?id=12345"
	if docs[0].URL != want {
		t.Errorf("URL = %q; want %q", docs[0].URL, want)
	}
}

// TestParseHitsHTMLBodyStripped verifies that story_text HTML is stripped for Body.
func TestParseHitsHTMLBodyStripped(t *testing.T) {
	t.Parallel()
	body := readFixture(t, "testdata/search_response_self_post.json")
	docs, _, err := parseHits(body, fixedRetrievedAt)
	if err != nil {
		t.Fatalf("parseHits error: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("len(docs) = %d; want 1", len(docs))
	}
	// story_text = "<p>Hello <b>world</b></p>&amp; goodbye"
	wantBody := "Hello world& goodbye"
	if docs[0].Body != wantBody {
		t.Errorf("Body = %q; want %q", docs[0].Body, wantBody)
	}
}

// TestParseHitsPaginationCursor verifies the next_cursor is set on the last doc
// when more pages exist.
func TestParseHitsPaginationCursor(t *testing.T) {
	t.Parallel()
	body := readFixture(t, "testdata/search_response_pagination.json")
	docs, nextCursor, err := parseHits(body, fixedRetrievedAt)
	if err != nil {
		t.Fatalf("parseHits error: %v", err)
	}
	if len(docs) == 0 {
		t.Fatal("no docs returned")
	}
	if nextCursor != "1" {
		t.Errorf("nextCursor = %q; want %q", nextCursor, "1")
	}
	// Only the last doc should have next_cursor.
	last := docs[len(docs)-1]
	cursorVal, ok := last.Metadata["next_cursor"]
	if !ok {
		t.Error("last doc missing Metadata[next_cursor]")
	} else if cursorVal != "1" {
		t.Errorf("last doc Metadata[next_cursor] = %v; want %q", cursorVal, "1")
	}
	// Earlier docs should NOT have next_cursor.
	for i := 0; i < len(docs)-1; i++ {
		if _, exists := docs[i].Metadata["next_cursor"]; exists {
			t.Errorf("docs[%d] unexpectedly has next_cursor", i)
		}
	}
}

// TestParseHitsNoCursorOnLastPage verifies no next_cursor is set when on the last page.
func TestParseHitsNoCursorOnLastPage(t *testing.T) {
	t.Parallel()
	// search_response.json has page=0, nbPages=1 (last page).
	body := readFixture(t, "testdata/search_response.json")
	docs, nextCursor, err := parseHits(body, fixedRetrievedAt)
	if err != nil {
		t.Fatalf("parseHits error: %v", err)
	}
	if nextCursor != "" {
		t.Errorf("nextCursor = %q; want empty (last page)", nextCursor)
	}
	for i, d := range docs {
		if _, exists := d.Metadata["next_cursor"]; exists {
			t.Errorf("docs[%d] has next_cursor on last page", i)
		}
	}
}

// TestParseHitsHashEmpty verifies all returned docs have Hash == "".
func TestParseHitsHashEmpty(t *testing.T) {
	t.Parallel()
	body := readFixture(t, "testdata/search_response.json")
	docs, _, err := parseHits(body, fixedRetrievedAt)
	if err != nil {
		t.Fatalf("parseHits error: %v", err)
	}
	for i, d := range docs {
		if d.Hash != "" {
			t.Errorf("docs[%d].Hash = %q; want empty", i, d.Hash)
		}
	}
}

// TestParseHitsMetadataKeys verifies all 4 required metadata keys are present.
func TestParseHitsMetadataKeys(t *testing.T) {
	t.Parallel()
	body := readFixture(t, "testdata/search_response.json")
	docs, _, err := parseHits(body, fixedRetrievedAt)
	if err != nil {
		t.Fatalf("parseHits error: %v", err)
	}
	requiredKeys := []string{"num_comments", "points", "tags", "external_url"}
	for i, d := range docs {
		for _, key := range requiredKeys {
			if _, ok := d.Metadata[key]; !ok {
				t.Errorf("docs[%d].Metadata missing key %q", i, key)
			}
		}
	}
}

// TestParseHitsDeletedAuthor verifies that an empty author field passes Validate().
func TestParseHitsDeletedAuthor(t *testing.T) {
	t.Parallel()
	body := readFixture(t, "testdata/search_response_deleted_author.json")
	docs, _, err := parseHits(body, fixedRetrievedAt)
	if err != nil {
		t.Fatalf("parseHits error: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("len(docs) = %d; want 1", len(docs))
	}
	if verr := docs[0].Validate(); verr != nil {
		t.Errorf("Validate() = %v; want nil", verr)
	}
}

// TestParseHitsMalformedJSON verifies that truncated/malformed JSON returns a
// *SourceError with CategoryPermanent.
func TestParseHitsMalformedJSON(t *testing.T) {
	t.Parallel()
	body := readFixture(t, "testdata/search_response_malformed.json")
	_, _, err := parseHits(body, fixedRetrievedAt)
	if err == nil {
		t.Fatal("parseHits() expected error for malformed JSON; got nil")
	}
	var se *types.SourceError
	if !isSourceError(err, &se) {
		t.Fatalf("expected *types.SourceError; got %T: %v", err, err)
	}
	if se.Category != types.CategoryPermanent {
		t.Errorf("Category = %v; want CategoryPermanent", se.Category)
	}
}

// TestParseHitsEmptyFixture verifies that empty hits returns nil slice, no error.
func TestParseHitsEmptyFixture(t *testing.T) {
	t.Parallel()
	body := readFixture(t, "testdata/search_response_empty.json")
	docs, nextCursor, err := parseHits(body, fixedRetrievedAt)
	if err != nil {
		t.Fatalf("parseHits() unexpected error: %v", err)
	}
	if docs != nil {
		t.Errorf("docs = %v; want nil", docs)
	}
	if nextCursor != "" {
		t.Errorf("nextCursor = %q; want empty", nextCursor)
	}
}

// TestParseHitsSnippetTruncation verifies Snippet is truncated to 280 runes.
func TestParseHitsSnippetTruncation(t *testing.T) {
	t.Parallel()
	// Build a fixture with a very long story_text.
	longText := strings.Repeat("a", 400)
	fixture := `{"hits":[{"objectID":"1","title":"T","url":"","author":"a","points":1,"story_text":"` +
		longText + `","num_comments":0,"created_at_i":1713169931,"_tags":["story"]}],"nbHits":1,"page":0,"nbPages":1,"hitsPerPage":25}`
	docs, _, err := parseHits([]byte(fixture), fixedRetrievedAt)
	if err != nil {
		t.Fatalf("parseHits error: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("len(docs) = %d; want 1", len(docs))
	}
	if len([]rune(docs[0].Snippet)) > 280 {
		t.Errorf("Snippet rune length = %d; want <= 280", len([]rune(docs[0].Snippet)))
	}
	if !strings.HasSuffix(docs[0].Snippet, "...") {
		t.Error("truncated Snippet should end with ...")
	}
}

// readFixture reads a testdata file and returns its contents. Fails the test on error.
func readFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("readFixture(%q): %v", path, err)
	}
	return data
}

// isSourceError checks if err is a *types.SourceError and sets the pointer.
func isSourceError(err error, se **types.SourceError) bool {
	if err == nil {
		return false
	}
	e, ok := err.(*types.SourceError)
	if ok {
		*se = e
	}
	return ok
}
