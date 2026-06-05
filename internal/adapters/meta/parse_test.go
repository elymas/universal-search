package meta

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

// ---------------------------------------------------------------------------
// REQ-ADP10-005: NormalizedDoc field mapping
// ---------------------------------------------------------------------------

// TestParseKeywordSearchFieldMapping verifies the §6.5 mapping against multiple fixtures.
func TestParseKeywordSearchFieldMapping(t *testing.T) {
	retrievedAt := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		fixture string
		check   func(t *testing.T, docs []types.NormalizedDoc)
	}{
		{
			"happy path text post",
			"threads_keyword_search_response.json",
			func(t *testing.T, docs []types.NormalizedDoc) {
				if len(docs) != 25 {
					t.Fatalf("got %d docs, want 25", len(docs))
				}
				// Verify first doc fields.
				d := docs[0]
				if d.ID != "threads:t_001" {
					t.Errorf("ID = %q, want 'threads:t_001'", d.ID)
				}
				if d.SourceID != "threads" {
					t.Errorf("SourceID = %q, want 'threads'", d.SourceID)
				}
				if d.URL != "https://www.threads.net/@alice/post/t_001" {
					t.Errorf("URL = %q", d.URL)
				}
				if d.Author != "alice" {
					t.Errorf("Author = %q, want 'alice'", d.Author)
				}
				if d.Body != "Hello Threads world!" {
					t.Errorf("Body = %q", d.Body)
				}
				if d.DocType != types.DocTypePost {
					t.Errorf("DocType = %v, want post", d.DocType)
				}
				if d.Hash != "" {
					t.Errorf("Hash = %q, want empty", d.Hash)
				}
				if d.Lang != "" {
					t.Errorf("Lang = %q, want empty", d.Lang)
				}
				if d.PublishedAt.Year() != 2026 {
					t.Errorf("PublishedAt = %v", d.PublishedAt)
				}
				if !d.RetrievedAt.Equal(retrievedAt) {
					t.Errorf("RetrievedAt = %v, want %v", d.RetrievedAt, retrievedAt)
				}
			},
		},
		{
			"media_type mixed",
			"threads_keyword_search_response_with_media.json",
			func(t *testing.T, docs []types.NormalizedDoc) {
				if len(docs) != 3 {
					t.Fatalf("got %d docs, want 3", len(docs))
				}
				// IMAGE post.
				if docs[0].Metadata["media_type"] != "IMAGE" {
					t.Errorf("media_type[0] = %v, want IMAGE", docs[0].Metadata["media_type"])
				}
				// VIDEO post.
				if docs[1].Metadata["media_type"] != "VIDEO" {
					t.Errorf("media_type[1] = %v, want VIDEO", docs[1].Metadata["media_type"])
				}
				// TEXT post.
				if docs[2].Metadata["media_type"] != "TEXT" {
					t.Errorf("media_type[2] = %v, want TEXT", docs[2].Metadata["media_type"])
				}
			},
		},
		{
			"optional fields (has_replies, is_reply, is_quote_post)",
			"threads_keyword_search_response.json",
			func(t *testing.T, docs []types.NormalizedDoc) {
				// doc index 2 (t_003) has has_replies=true
				if v, ok := docs[2].Metadata["has_replies"]; !ok || v != true {
					t.Errorf("has_replies = %v, want true", v)
				}
				// doc index 3 (t_004) has is_reply=true
				if v, ok := docs[3].Metadata["is_reply"]; !ok || v != true {
					t.Errorf("is_reply = %v, want true", v)
				}
				// doc index 4 (t_005) has is_quote_post=true
				if v, ok := docs[4].Metadata["is_quote_post"]; !ok || v != true {
					t.Errorf("is_quote_post = %v, want true", v)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := loadFixture(t, tt.fixture)
			docs, err := parseKeywordSearch(body, retrievedAt)
			if err != nil {
				t.Fatalf("parseKeywordSearch: %v", err)
			}
			tt.check(t, docs)
		})
	}
}

// TestParseKeywordSearchScoreNeutral verifies every doc has Score == 0.5.
func TestParseKeywordSearchScoreNeutral(t *testing.T) {
	body := loadFixture(t, "threads_keyword_search_response.json")
	docs, err := parseKeywordSearch(body, time.Now().UTC())
	if err != nil {
		t.Fatalf("parseKeywordSearch: %v", err)
	}
	for _, d := range docs {
		if d.Score != 0.5 {
			t.Errorf("Score = %f, want 0.5 (doc %s)", d.Score, d.ID)
		}
	}
}

// TestParseKeywordSearchLangEmpty verifies every doc has Lang == "".
func TestParseKeywordSearchLangEmpty(t *testing.T) {
	body := loadFixture(t, "threads_keyword_search_response.json")
	docs, err := parseKeywordSearch(body, time.Now().UTC())
	if err != nil {
		t.Fatalf("parseKeywordSearch: %v", err)
	}
	for _, d := range docs {
		if d.Lang != "" {
			t.Errorf("Lang = %q, want empty (doc %s)", d.Lang, d.ID)
		}
	}
}

// TestParseKeywordSearchHashEmpty verifies every doc has Hash == "".
func TestParseKeywordSearchHashEmpty(t *testing.T) {
	body := loadFixture(t, "threads_keyword_search_response.json")
	docs, err := parseKeywordSearch(body, time.Now().UTC())
	if err != nil {
		t.Fatalf("parseKeywordSearch: %v", err)
	}
	for _, d := range docs {
		if d.Hash != "" {
			t.Errorf("Hash = %q, want empty (doc %s)", d.Hash, d.ID)
		}
	}
}

// TestParseKeywordSearchMetadataKeys verifies the 5 required Metadata keys.
func TestParseKeywordSearchMetadataKeys(t *testing.T) {
	body := loadFixture(t, "threads_keyword_search_response.json")
	docs, err := parseKeywordSearch(body, time.Now().UTC())
	if err != nil {
		t.Fatalf("parseKeywordSearch: %v", err)
	}

	requiredKeys := []string{"username", "permalink", "media_type", "posted_at", "sub_source"}
	for _, d := range docs {
		for _, key := range requiredKeys {
			if _, ok := d.Metadata[key]; !ok {
				t.Errorf("doc %s: missing required metadata key %q", d.ID, key)
			}
		}
		if d.Metadata["sub_source"] != "threads" {
			t.Errorf("doc %s: sub_source = %v, want 'threads'", d.ID, d.Metadata["sub_source"])
		}
	}
}

// TestParseKeywordSearchSnippetTruncation verifies >280-rune text is truncated.
func TestParseKeywordSearchSnippetTruncation(t *testing.T) {
	longText := strings.Repeat("A", 300) // 300 ASCII chars = 300 runes
	post := keywordSearchPost{
		ID:        "truncate_test",
		Username:  "user",
		Text:      longText,
		Permalink: "https://example.com",
		Timestamp: "2026-05-01T10:00:00Z",
		MediaType: "TEXT",
	}
	doc := mapPostToNormalizedDoc(post, time.Now().UTC())

	if len(doc.Snippet) > 280 {
		// Check rune count, not byte count.
		snippetRunes := len([]rune(doc.Snippet))
		if snippetRunes != 280 {
			t.Errorf("Snippet rune count = %d, want 280", snippetRunes)
		}
	}
	// Verify Title == Snippet.
	if doc.Title != doc.Snippet {
		t.Errorf("Title != Snippet: Title=%q, Snippet=%q", doc.Title, doc.Snippet)
	}
	// Body should be the full text.
	if doc.Body != longText {
		t.Error("Body should be the full text, not truncated")
	}
}

// TestParseKeywordSearchMalformedJSON verifies malformed JSON returns Permanent error.
func TestParseKeywordSearchMalformedJSON(t *testing.T) {
	body := loadFixture(t, "threads_keyword_search_response_malformed.json")
	_, err := parseKeywordSearch(body, time.Now().UTC())
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
	var se *types.SourceError
	if !errors.As(err, &se) {
		t.Fatalf("expected *types.SourceError, got %T: %v", err, err)
	}
	if se.Category != types.CategoryPermanent {
		t.Errorf("Category = %v, want Permanent", se.Category)
	}
}

// ---------------------------------------------------------------------------
// REQ-ADP10-004: Error envelope + empty data
// ---------------------------------------------------------------------------

// TestParseKeywordSearchErrorBeforeData verifies error envelope is checked before data.
func TestParseKeywordSearchErrorBeforeData(t *testing.T) {
	// Body with both error and data — error should take precedence.
	body := []byte(`{"error":{"message":"permission denied","type":"OAuthException","code":12},"data":[{"id":"x","username":"u","text":"t","permalink":"https://example.com","timestamp":"2026-01-01T00:00:00Z","media_type":"TEXT"}]}`)
	_, err := parseKeywordSearch(body, time.Now().UTC())
	if err == nil {
		t.Fatal("expected error from Graph error envelope, got nil")
	}
	var se *types.SourceError
	if !errors.As(err, &se) {
		t.Fatalf("expected *types.SourceError, got %T: %v", err, err)
	}
	if se.Category != types.CategoryPermanent {
		t.Errorf("Category = %v, want Permanent", se.Category)
	}
	// Error message should contain the Graph error message and code.
	errMsg := err.Error()
	if !strings.Contains(errMsg, "permission denied") {
		t.Errorf("error message missing 'permission denied': %s", errMsg)
	}
	if !strings.Contains(errMsg, "code 12") {
		t.Errorf("error message missing 'code 12': %s", errMsg)
	}
}

// TestParseKeywordSearchEmptyDataReturnsNil verifies empty data returns (nil, nil).
func TestParseKeywordSearchEmptyDataReturnsNil(t *testing.T) {
	body := loadFixture(t, "threads_keyword_search_response_empty.json")
	docs, err := parseKeywordSearch(body, time.Now().UTC())
	if err != nil {
		t.Fatalf("unexpected error for empty data: %v", err)
	}
	if docs != nil {
		t.Errorf("expected nil docs for empty data, got %d", len(docs))
	}
}
