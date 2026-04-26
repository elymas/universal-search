package reddit

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

var testRetrievedAt = time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)

// TestParseListingFieldMapping verifies the field mapping for various fixture types.
func TestParseListingFieldMapping(t *testing.T) {
	t.Parallel()

	// Use the deleted post fixture to check [deleted] author handling.
	deletedBody, err := os.ReadFile("testdata/search_response_deleted_post.json")
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	docs, _, parseErr := parseListing(deletedBody, testRetrievedAt)
	if parseErr != nil {
		t.Fatalf("parseListing() error = %v", parseErr)
	}
	if len(docs) != 1 {
		t.Fatalf("parseListing() returned %d docs, want 1", len(docs))
	}
	doc := docs[0]

	// Verify required fields.
	if doc.ID != "t3_deleted_001" {
		t.Errorf("ID = %q, want %q", doc.ID, "t3_deleted_001")
	}
	if doc.SourceID != "reddit" {
		t.Errorf("SourceID = %q, want %q", doc.SourceID, "reddit")
	}
	wantURL := "https://www.reddit.com/r/golang/comments/deleted_001/deleted_post/"
	if doc.URL != wantURL {
		t.Errorf("URL = %q, want %q", doc.URL, wantURL)
	}
	if doc.Title != "Deleted Post Title" {
		t.Errorf("Title = %q, want %q", doc.Title, "Deleted Post Title")
	}
	if doc.Author != "[deleted]" {
		t.Errorf("Author = %q, want %q", doc.Author, "[deleted]")
	}
	if doc.Lang != "" {
		t.Errorf("Lang = %q, want %q", doc.Lang, "")
	}
	if doc.DocType != types.DocTypePost {
		t.Errorf("DocType = %v, want %v", doc.DocType, types.DocTypePost)
	}
	if doc.Citations != nil {
		t.Errorf("Citations = %v, want nil", doc.Citations)
	}
	if doc.Hash != "" {
		t.Errorf("Hash = %q, want %q", doc.Hash, "")
	}
	if doc.RetrievedAt != testRetrievedAt {
		t.Errorf("RetrievedAt = %v, want %v", doc.RetrievedAt, testRetrievedAt)
	}
	wantPublishedAt := time.Unix(1700030001, 0).UTC()
	if doc.PublishedAt != wantPublishedAt {
		t.Errorf("PublishedAt = %v, want %v", doc.PublishedAt, wantPublishedAt)
	}

	// Validate() should pass even for deleted post.
	if err := doc.Validate(); err != nil {
		t.Errorf("Validate() error = %v", err)
	}
}

// TestParseListingFiltersNonT3Kinds verifies that non-t3 kinds are skipped.
func TestParseListingFiltersNonT3Kinds(t *testing.T) {
	t.Parallel()

	// Build a fixture with mixed kinds in-memory.
	mixedJSON := []byte(`{
		"data": {
			"after": null,
			"children": [
				{"kind": "t1", "data": {"name": "t1_comment", "permalink": "/r/golang/comments/xxx/", "title": "Comment", "selftext": "comment body", "created_utc": 1700000001, "author": "commenter", "score": 5, "subreddit": "golang", "over_18": false, "num_comments": 0, "upvote_ratio": 1.0, "url": "https://reddit.com/r/golang/comments/xxx/"}},
				{"kind": "t3", "data": {"name": "t3_post_a", "permalink": "/r/golang/comments/posta/title/", "title": "Post A", "selftext": "body a", "created_utc": 1700000002, "author": "author_a", "score": 100, "subreddit": "golang", "over_18": false, "num_comments": 5, "upvote_ratio": 0.9, "url": "https://www.reddit.com/r/golang/comments/posta/title/"}},
				{"kind": "t5", "data": {"name": "t5_subreddit", "permalink": "/r/golang/", "title": "golang subreddit", "selftext": "", "created_utc": 1700000003, "author": "reddit", "score": 0, "subreddit": "golang", "over_18": false, "num_comments": 0, "upvote_ratio": 1.0, "url": "https://www.reddit.com/r/golang/"}},
				{"kind": "t3", "data": {"name": "t3_post_b", "permalink": "/r/golang/comments/postb/title/", "title": "Post B", "selftext": "body b", "created_utc": 1700000004, "author": "author_b", "score": 200, "subreddit": "golang", "over_18": false, "num_comments": 10, "upvote_ratio": 0.95, "url": "https://www.reddit.com/r/golang/comments/postb/title/"}}
			]
		}
	}`)

	docs, _, err := parseListing(mixedJSON, testRetrievedAt)
	if err != nil {
		t.Fatalf("parseListing() error = %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("parseListing() returned %d docs, want 2 (only t3 kinds)", len(docs))
	}
	if docs[0].ID != "t3_post_a" {
		t.Errorf("docs[0].ID = %q, want %q", docs[0].ID, "t3_post_a")
	}
	if docs[1].ID != "t3_post_b" {
		t.Errorf("docs[1].ID = %q, want %q", docs[1].ID, "t3_post_b")
	}
}

// TestParseListingPaginationCursor verifies that data.after is set on the last doc.
func TestParseListingPaginationCursor(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile("testdata/search_response_pagination.json")
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	docs, cursor, parseErr := parseListing(body, testRetrievedAt)
	if parseErr != nil {
		t.Fatalf("parseListing() error = %v", parseErr)
	}
	if len(docs) == 0 {
		t.Fatal("parseListing() returned 0 docs, want > 0")
	}
	if cursor != "t3_pageN" {
		t.Errorf("cursor = %q, want %q", cursor, "t3_pageN")
	}

	// Only the last doc should have next_cursor.
	last := docs[len(docs)-1]
	if got, ok := last.Metadata["next_cursor"]; !ok || got != "t3_pageN" {
		t.Errorf("last doc Metadata[next_cursor] = %v (ok=%v), want %q", got, ok, "t3_pageN")
	}

	// Earlier docs should NOT have next_cursor.
	for i, doc := range docs[:len(docs)-1] {
		if _, ok := doc.Metadata["next_cursor"]; ok {
			t.Errorf("docs[%d] has next_cursor, want absent", i)
		}
	}
}

// TestParseListingNoCursorOnEmpty verifies no next_cursor when data.after is null.
func TestParseListingNoCursorOnEmpty(t *testing.T) {
	t.Parallel()

	// Use empty fixture (data.after = null).
	body, err := os.ReadFile("testdata/search_response_empty.json")
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	docs, cursor, parseErr := parseListing(body, testRetrievedAt)
	if parseErr != nil {
		t.Fatalf("parseListing() error = %v", parseErr)
	}
	if len(docs) != 0 {
		t.Errorf("parseListing() returned %d docs, want 0", len(docs))
	}
	if cursor != "" {
		t.Errorf("cursor = %q, want empty", cursor)
	}
}

// TestParseListingHashEmpty verifies that Hash is always empty string.
func TestParseListingHashEmpty(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile("testdata/search_response.json")
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	docs, _, parseErr := parseListing(body, testRetrievedAt)
	if parseErr != nil {
		t.Fatalf("parseListing() error = %v", parseErr)
	}
	for i, doc := range docs {
		if doc.Hash != "" {
			t.Errorf("docs[%d].Hash = %q, want %q", i, doc.Hash, "")
		}
	}
}

// TestParseListingMetadataKeys verifies that all 6 required metadata keys are present.
func TestParseListingMetadataKeys(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile("testdata/search_response.json")
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	docs, _, parseErr := parseListing(body, testRetrievedAt)
	if parseErr != nil {
		t.Fatalf("parseListing() error = %v", parseErr)
	}

	requiredKeys := []string{"subreddit", "over_18", "num_comments", "upvote_ratio", "external_url", "kind"}
	for i, doc := range docs {
		for _, key := range requiredKeys {
			if _, ok := doc.Metadata[key]; !ok {
				t.Errorf("docs[%d].Metadata missing required key %q", i, key)
			}
		}
		// kind must always be "t3"
		if kind, ok := doc.Metadata["kind"]; !ok || kind != "t3" {
			t.Errorf("docs[%d].Metadata[kind] = %v, want %q", i, kind, "t3")
		}
	}
}

// TestParseListingDeletedAuthor verifies that [deleted] author is returned as-is.
func TestParseListingDeletedAuthor(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile("testdata/search_response_deleted_post.json")
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	docs, _, parseErr := parseListing(body, testRetrievedAt)
	if parseErr != nil {
		t.Fatalf("parseListing() error = %v", parseErr)
	}
	if len(docs) != 1 {
		t.Fatalf("parseListing() returned %d docs, want 1", len(docs))
	}

	doc := docs[0]
	if doc.Author != "[deleted]" {
		t.Errorf("Author = %q, want %q", doc.Author, "[deleted]")
	}
	// Validate() must still pass (URL is the permalink, which is valid).
	if err := doc.Validate(); err != nil {
		t.Errorf("Validate() error = %v", err)
	}
}

// TestParseListingMalformedJSON verifies that malformed JSON returns a SourceError.
func TestParseListingMalformedJSON(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile("testdata/search_response_malformed.json")
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	_, _, parseErr := parseListing(body, testRetrievedAt)
	if parseErr == nil {
		t.Fatal("parseListing() expected error for malformed JSON, got nil")
	}

	var se *types.SourceError
	if !errors.As(parseErr, &se) {
		t.Fatalf("parseErr is not *types.SourceError: %T = %v", parseErr, parseErr)
	}
	if se.Category != types.CategoryPermanent {
		t.Errorf("SourceError.Category = %v, want CategoryPermanent", se.Category)
	}
}

// TestBuildSnippetTruncatesLongSelftext verifies that a selftext exceeding
// snippetMaxRunes (280) is truncated to exactly maxRunes characters with "..." suffix.
func TestBuildSnippetTruncatesLongSelftext(t *testing.T) {
	t.Parallel()

	// Build a string of 290 ASCII characters (> 280 rune limit).
	long := make([]byte, 290)
	for i := range long {
		long[i] = 'a'
	}
	got := buildSnippet(string(long), "title")
	if len(got) != snippetMaxRunes {
		t.Errorf("buildSnippet() len = %d, want %d (snippetMaxRunes)", len(got), snippetMaxRunes)
	}
	if got[len(got)-3:] != "..." {
		t.Errorf("buildSnippet() does not end with '...': %q", got[len(got)-3:])
	}
}

// TestBuildSnippetFallsBackToTitle verifies that an empty selftext uses the title.
func TestBuildSnippetFallsBackToTitle(t *testing.T) {
	t.Parallel()

	got := buildSnippet("", "my title")
	if got != "my title" {
		t.Errorf("buildSnippet(\"\", title) = %q, want %q", got, "my title")
	}
}

// TestClampLimitNegative verifies that a negative maxResults is clamped to 1.
func TestClampLimitNegative(t *testing.T) {
	t.Parallel()

	if got := clampLimit(-5); got != 1 {
		t.Errorf("clampLimit(-5) = %d, want 1", got)
	}
}

// TestParseListingOptionalMetadataKeys verifies that optional metadata fields
// (spoiler, locked, stickied, link_flair_text, post_hint, subreddit_name_prefixed, ups)
// are included when present.
func TestParseListingOptionalMetadataKeys(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"data": {
			"after": null,
			"children": [
				{"kind": "t3", "data": {
					"name": "t3_opt001",
					"permalink": "/r/golang/comments/opt001/test/",
					"title": "Optional Fields Post",
					"selftext": "body",
					"created_utc": 1700000001,
					"author": "author1",
					"score": 42,
					"subreddit": "golang",
					"over_18": false,
					"num_comments": 3,
					"upvote_ratio": 0.85,
					"url": "https://www.reddit.com/r/golang/comments/opt001/test/",
					"subreddit_name_prefixed": "r/golang",
					"ups": 40,
					"spoiler": true,
					"locked": true,
					"stickied": true,
					"link_flair_text": "Discussion",
					"post_hint": "self"
				}}
			]
		}
	}`)

	docs, _, err := parseListing(body, testRetrievedAt)
	if err != nil {
		t.Fatalf("parseListing() error = %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("parseListing() returned %d docs, want 1", len(docs))
	}

	doc := docs[0]
	optionalKeys := map[string]any{
		"subreddit_name_prefixed": "r/golang",
		"ups":                     40,
		"spoiler":                 true,
		"locked":                  true,
		"stickied":                true,
		"link_flair_text":         "Discussion",
		"post_hint":               "self",
	}
	for key, wantVal := range optionalKeys {
		got, ok := doc.Metadata[key]
		if !ok {
			t.Errorf("Metadata missing key %q", key)
			continue
		}
		if got != wantVal {
			t.Errorf("Metadata[%q] = %v (%T), want %v (%T)", key, got, got, wantVal, wantVal)
		}
	}
}

// TestTruncateRunesNoTruncationNeeded verifies that strings <= maxRunes are returned unchanged.
func TestTruncateRunesNoTruncationNeeded(t *testing.T) {
	t.Parallel()

	short := "hello"
	got := truncateRunes(short, 10)
	if got != short {
		t.Errorf("truncateRunes(%q, 10) = %q, want %q", short, got, short)
	}
}

// TestTruncateRunesUnicode verifies that truncation is rune-aware (multi-byte chars).
func TestTruncateRunesUnicode(t *testing.T) {
	t.Parallel()

	// Each '한' is 3 bytes in UTF-8 but 1 rune.
	// Build 10 '한' runes (10 runes, 30 bytes).
	s := "한한한한한한한한한한" // 10 Korean runes
	got := truncateRunes(s, 5)
	// Want first 2 runes + "..." = 5 runes total.
	wantRunes := 5
	if got == s {
		t.Fatal("truncateRunes() did not truncate")
	}
	if gotCount := len([]rune(got)); gotCount != wantRunes {
		t.Errorf("truncateRunes() rune count = %d, want %d; got = %q", gotCount, wantRunes, got)
	}
	if got[len(got)-3:] != "..." {
		t.Errorf("truncateRunes() does not end with '...': %q", got)
	}
}
