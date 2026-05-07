// Package social — tests for Bluesky JSON → []NormalizedDoc transform.
// REQ-ADP6-006: field mapping, cursor, 6 required metadata keys.
package social

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// testdataPath is the prefix for all test fixture files.
const testdataPath = "testdata/"

// TestParseSearchPostsHappyPath verifies the 25-post fixture produces 25 docs
// with correct field mapping.
func TestParseSearchPostsHappyPath(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile(testdataPath + "bluesky_search_response.json")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	retrievedAt := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	docs, cursor, parseErr := parseSearchPosts(body, retrievedAt)
	if parseErr != nil {
		t.Fatalf("parseSearchPosts: unexpected error: %v", parseErr)
	}

	if len(docs) != 25 {
		t.Errorf("len(docs): got %d, want 25", len(docs))
	}

	if cursor != "3jzfcijpj2z2a" {
		t.Errorf("cursor: got %q, want %q", cursor, "3jzfcijpj2z2a")
	}

	// Verify first doc field mapping (alice.bsky.social post).
	first := docs[0]
	if err := first.Validate(); err != nil {
		t.Errorf("docs[0].Validate(): %v", err)
	}

	// ID = rkey from AT-URI.
	if first.ID == "" {
		t.Error("docs[0].ID: got empty string, want rkey")
	}

	// SourceID = "bluesky".
	if first.SourceID != "bluesky" {
		t.Errorf("docs[0].SourceID: got %q, want %q", first.SourceID, "bluesky")
	}

	// URL contains bsky.app.
	if first.URL == "" || len(first.URL) < 10 {
		t.Errorf("docs[0].URL: got %q, want non-empty bsky.app URL", first.URL)
	}

	// Title MUST be empty string (AT spec has no headline field — H2 fix).
	if first.Title != "" {
		t.Errorf("docs[0].Title: got %q, want empty string (AT spec has no headline)", first.Title)
	}

	// Body = record.text.
	if first.Body == "" {
		t.Error("docs[0].Body: got empty, want record.text content")
	}

	// Snippet = truncateRunes(record.text, 280).
	if first.Snippet == "" {
		t.Error("docs[0].Snippet: got empty")
	}

	// PublishedAt must be set.
	if first.PublishedAt.IsZero() {
		t.Error("docs[0].PublishedAt: got zero time")
	}

	// RetrievedAt must equal the injected time.
	if !first.RetrievedAt.Equal(retrievedAt) {
		t.Errorf("docs[0].RetrievedAt: got %v, want %v", first.RetrievedAt, retrievedAt)
	}

	// Author = author.displayName or handle.
	if first.Author == "" {
		t.Error("docs[0].Author: got empty")
	}

	// Score in [0.0, 1.0].
	if first.Score < 0 || first.Score > 1 {
		t.Errorf("docs[0].Score: got %f, out of [0,1]", first.Score)
	}

	// DocType = "post" per REQ-ADP6-005.
	if first.DocType != types.DocTypePost {
		t.Errorf("docs[0].DocType: got %q, want %q", first.DocType, types.DocTypePost)
	}

	// Hash = "" (set by consumer).
	if first.Hash != "" {
		t.Errorf("docs[0].Hash: got %q, want empty (set by consumer)", first.Hash)
	}

	// Check 6 required metadata keys.
	requiredMeta := []string{"handle", "post_uri", "repost_count", "like_count", "posted_at", "sub_source"}
	for _, key := range requiredMeta {
		if _, ok := first.Metadata[key]; !ok {
			t.Errorf("docs[0].Metadata: missing required key %q", key)
		}
	}

	// sub_source = "bluesky".
	if subSrc, ok := first.Metadata["sub_source"]; !ok || subSrc != "bluesky" {
		t.Errorf("docs[0].Metadata[sub_source]: got %v, want %q", subSrc, "bluesky")
	}
}

// TestParseSearchPostsEmpty verifies empty posts array returns nil slice.
func TestParseSearchPostsEmpty(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile(testdataPath + "bluesky_search_response_empty.json")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	retrievedAt := time.Now()
	docs, cursor, parseErr := parseSearchPosts(body, retrievedAt)
	if parseErr != nil {
		t.Fatalf("parseSearchPosts: unexpected error: %v", parseErr)
	}
	if len(docs) != 0 {
		t.Errorf("len(docs): got %d, want 0", len(docs))
	}
	if cursor != "" {
		t.Errorf("cursor: got %q, want empty", cursor)
	}
}

// TestParseSearchPostsLangField verifies Lang field extraction.
func TestParseSearchPostsLangField(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile(testdataPath + "bluesky_search_response_with_lang.json")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	docs, _, parseErr := parseSearchPosts(body, time.Now())
	if parseErr != nil {
		t.Fatalf("parseSearchPosts: %v", parseErr)
	}
	if len(docs) != 3 {
		t.Fatalf("len(docs): got %d, want 3", len(docs))
	}

	// First post: langs=["ko"] -> Lang="ko"
	if docs[0].Lang != "ko" {
		t.Errorf("docs[0].Lang: got %q, want %q", docs[0].Lang, "ko")
	}
	// Second post: langs=["en"] -> Lang="en"
	if docs[1].Lang != "en" {
		t.Errorf("docs[1].Lang: got %q, want %q", docs[1].Lang, "en")
	}
	// Third post: no langs -> Lang=""
	if docs[2].Lang != "" {
		t.Errorf("docs[2].Lang: got %q, want %q", docs[2].Lang, "")
	}
}

// TestParseSearchPostsPagination verifies cursor is placed on last doc.
func TestParseSearchPostsPagination(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile(testdataPath + "bluesky_search_response_pagination.json")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	docs, cursor, parseErr := parseSearchPosts(body, time.Now())
	if parseErr != nil {
		t.Fatalf("parseSearchPosts: %v", parseErr)
	}
	if len(docs) != 2 {
		t.Fatalf("len(docs): got %d, want 2", len(docs))
	}

	if cursor != "nextpagecursor_abc123" {
		t.Errorf("cursor: got %q, want %q", cursor, "nextpagecursor_abc123")
	}

	// next_cursor is on the LAST doc.
	last := docs[len(docs)-1]
	if v, ok := last.Metadata["next_cursor"]; !ok || v != "nextpagecursor_abc123" {
		t.Errorf("last doc Metadata[next_cursor]: got %v, want %q", v, "nextpagecursor_abc123")
	}

	// next_cursor is NOT on the first doc.
	first := docs[0]
	if _, ok := first.Metadata["next_cursor"]; ok {
		t.Errorf("first doc Metadata[next_cursor]: should be absent")
	}
}

// TestParseSearchPostsHighEngagement verifies score saturation for viral posts.
func TestParseSearchPostsHighEngagement(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile(testdataPath + "bluesky_search_response_high_engagement.json")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	docs, _, parseErr := parseSearchPosts(body, time.Now())
	if parseErr != nil {
		t.Fatalf("parseSearchPosts: %v", parseErr)
	}
	if len(docs) != 1 {
		t.Fatalf("len(docs): got %d, want 1", len(docs))
	}

	// likes=1000, reposts=500 -> score near 1.0
	if docs[0].Score < 0.999 {
		t.Errorf("docs[0].Score: got %f, want near 1.0 for high engagement", docs[0].Score)
	}
}

// TestParseSearchPostsMalformed verifies malformed JSON returns a permanent SourceError.
func TestParseSearchPostsMalformed(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile(testdataPath + "bluesky_search_response_malformed.json")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	_, _, parseErr := parseSearchPosts(body, time.Now())
	if parseErr == nil {
		t.Fatal("parseSearchPosts: expected error for malformed JSON, got nil")
	}

	var se *types.SourceError
	if !errors.As(parseErr, &se) {
		t.Fatalf("parseSearchPosts malformed: expected *types.SourceError, got %T: %v", parseErr, parseErr)
	}
	if se.Category != types.CategoryPermanent {
		t.Errorf("parseSearchPosts malformed: Category got %v, want %v", se.Category, types.CategoryPermanent)
	}
}

// TestParseSearchPostsXRPCError verifies XRPC error envelope treated as error.
func TestParseSearchPostsXRPCError(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile(testdataPath + "bluesky_search_response_xrpc_error.json")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	// An XRPC error response is valid JSON but has {error, message} keys.
	// parseSearchPosts should detect this and return an error.
	_, _, parseErr := parseSearchPosts(body, time.Now())
	if parseErr == nil {
		t.Fatal("parseSearchPosts: expected error for XRPC error envelope, got nil")
	}
}

// TestNormalizedDocValidateAfterParse verifies all returned docs pass Validate().
func TestNormalizedDocValidateAfterParse(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile(testdataPath + "bluesky_search_response.json")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	docs, _, parseErr := parseSearchPosts(body, time.Now())
	if parseErr != nil {
		t.Fatalf("parseSearchPosts: %v", parseErr)
	}

	for i, doc := range docs {
		if err := doc.Validate(); err != nil {
			t.Errorf("docs[%d].Validate(): %v", i, err)
		}
	}
}
