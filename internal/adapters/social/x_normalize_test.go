// Package social — tests for X (Twitter) tweet normalization.
// SPEC-ADP-006-XENABLE: REQ-XEN-006 field mapping tests.
package social

import (
	"math"
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// --- Test 17: TestNormalizeXTweetsFieldMapping ---
func TestNormalizeXTweetsFieldMapping(t *testing.T) {
	t.Parallel()

	retrievedAt := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)

	fixtures := []struct {
		name  string
		tweet XTweet
	}{
		{
			name: "typical tweet",
			tweet: XTweet{
				ID:           "12345",
				Text:         "Hello world! This is a typical tweet with some content.",
				AuthorHandle: "gopher",
				URL:          "https://x.com/gopher/status/12345",
				LikeCount:    50,
				RepostCount:  20,
				ReplyCount:   5,
				QuoteCount:   3,
				CreatedAt:    "2026-06-04T10:00:00Z",
			},
		},
		{
			name: "empty author with URL fallback",
			tweet: XTweet{
				ID:           "67890",
				Text:         "No author tweet",
				AuthorHandle: "",
				URL:          "https://x.com/someone/status/67890",
				LikeCount:    0,
				RepostCount:  0,
				ReplyCount:   0,
				QuoteCount:   0,
				CreatedAt:    "",
			},
		},
		{
			name: "high engagement",
			tweet: XTweet{
				ID:           "99999",
				Text:         "Viral tweet with lots of engagement from the community!",
				AuthorHandle: "viral_user",
				URL:          "",
				LikeCount:    5000,
				RepostCount:  2000,
				ReplyCount:   500,
				QuoteCount:   300,
				CreatedAt:    "2026-06-03T08:30:00Z",
			},
		},
		{
			name: "zero engagement",
			tweet: XTweet{
				ID:           "00001",
				Text:         "Brand new tweet with zero interaction",
				AuthorHandle: "newbie",
				URL:          "",
				LikeCount:    0,
				RepostCount:  0,
				ReplyCount:   0,
				QuoteCount:   0,
				CreatedAt:    "2026-06-04T11:59:59Z",
			},
		},
	}

	for _, fx := range fixtures {
		t.Run(fx.name, func(t *testing.T) {
			t.Parallel()
			docs, err := normalizeXTweets([]XTweet{fx.tweet}, "", retrievedAt)
			if err != nil {
				t.Fatalf("normalizeXTweets: unexpected error: %v", err)
			}
			if len(docs) != 1 {
				t.Fatalf("len(docs): got %d, want 1", len(docs))
			}
			doc := docs[0]
			tw := fx.tweet

			// ID
			wantID := "x:" + tw.ID
			if doc.ID != wantID {
				t.Errorf("ID: got %q, want %q", doc.ID, wantID)
			}
			// SourceID
			if doc.SourceID != "x" {
				t.Errorf("SourceID: got %q, want %q", doc.SourceID, "x")
			}
			// URL
			wantURL := tw.URL
			if wantURL == "" {
				if tw.AuthorHandle != "" {
					wantURL = "https://x.com/" + tw.AuthorHandle + "/status/" + tw.ID
				} else {
					wantURL = "https://x.com/i/status/" + tw.ID
				}
			}
			if doc.URL != wantURL {
				t.Errorf("URL: got %q, want %q", doc.URL, wantURL)
			}
			// Title (first 280 runes, same as Snippet)
			if len(tw.Text) <= 280 {
				if doc.Title != tw.Text {
					t.Errorf("Title: got %q, want %q", doc.Title, tw.Text)
				}
			}
			// Body
			if doc.Body != tw.Text {
				t.Errorf("Body: got %q, want %q", doc.Body, tw.Text)
			}
			// Snippet
			if len(tw.Text) <= 280 {
				if doc.Snippet != tw.Text {
					t.Errorf("Snippet: got %q, want %q", doc.Snippet, tw.Text)
				}
			}
			// PublishedAt
			if tw.CreatedAt != "" {
				wantTime, parseErr := time.Parse(time.RFC3339, tw.CreatedAt)
				if parseErr == nil {
					if !doc.PublishedAt.Equal(wantTime.UTC()) {
						t.Errorf("PublishedAt: got %v, want %v", doc.PublishedAt, wantTime.UTC())
					}
				}
			}
			// RetrievedAt
			if !doc.RetrievedAt.Equal(retrievedAt) {
				t.Errorf("RetrievedAt: got %v, want %v", doc.RetrievedAt, retrievedAt)
			}
			// Author
			if doc.Author != tw.AuthorHandle {
				t.Errorf("Author: got %q, want %q", doc.Author, tw.AuthorHandle)
			}
			// Lang
			if doc.Lang != "" {
				t.Errorf("Lang: got %q, want empty", doc.Lang)
			}
			// DocType
			if doc.DocType != types.DocTypePost {
				t.Errorf("DocType: got %q, want %q", doc.DocType, types.DocTypePost)
			}
			// Hash
			if doc.Hash != "" {
				t.Errorf("Hash: got %q, want empty", doc.Hash)
			}
			// Score (check with normalizeScore)
			wantScore := normalizeScore(tw.LikeCount, tw.RepostCount)
			if doc.Score != wantScore {
				t.Errorf("Score: got %v, want %v", doc.Score, wantScore)
			}
		})
	}
}

// --- Test 18: TestNormalizeXTweetsURLFallback ---
func TestNormalizeXTweetsURLFallback(t *testing.T) {
	t.Parallel()
	retrievedAt := time.Now().UTC()

	t.Run("empty handle fallback to generic URL", func(t *testing.T) {
		t.Parallel()
		tweet := XTweet{
			ID:           "abc123",
			Text:         "No author tweet",
			AuthorHandle: "",
			URL:          "",
			LikeCount:    0,
			RepostCount:  0,
			CreatedAt:    "2026-06-04T12:00:00Z",
		}
		docs, err := normalizeXTweets([]XTweet{tweet}, "", retrievedAt)
		if err != nil {
			t.Fatalf("normalizeXTweets: %v", err)
		}
		want := "https://x.com/i/status/abc123"
		if docs[0].URL != want {
			t.Errorf("URL: got %q, want %q", docs[0].URL, want)
		}
	})

	t.Run("handle present constructs handle URL", func(t *testing.T) {
		t.Parallel()
		tweet := XTweet{
			ID:           "def456",
			Text:         "Has author tweet",
			AuthorHandle: "gopher",
			URL:          "",
			LikeCount:    0,
			RepostCount:  0,
			CreatedAt:    "2026-06-04T12:00:00Z",
		}
		docs, err := normalizeXTweets([]XTweet{tweet}, "", retrievedAt)
		if err != nil {
			t.Fatalf("normalizeXTweets: %v", err)
		}
		want := "https://x.com/gopher/status/def456"
		if docs[0].URL != want {
			t.Errorf("URL: got %q, want %q", docs[0].URL, want)
		}
	})

	t.Run("provider URL takes precedence", func(t *testing.T) {
		t.Parallel()
		tweet := XTweet{
			ID:           "ghi789",
			Text:         "Has provider URL",
			AuthorHandle: "gopher",
			URL:          "https://x.com/gopher/status/ghi789?s=20",
			LikeCount:    0,
			RepostCount:  0,
			CreatedAt:    "2026-06-04T12:00:00Z",
		}
		docs, err := normalizeXTweets([]XTweet{tweet}, "", retrievedAt)
		if err != nil {
			t.Fatalf("normalizeXTweets: %v", err)
		}
		want := "https://x.com/gopher/status/ghi789?s=20"
		if docs[0].URL != want {
			t.Errorf("URL: got %q, want %q", docs[0].URL, want)
		}
	})
}

// --- Test 19: TestNormalizeXTweetsHashEmpty ---
func TestNormalizeXTweetsHashEmpty(t *testing.T) {
	t.Parallel()
	retrievedAt := time.Now().UTC()

	tweets := makeXTweets(5)
	docs, err := normalizeXTweets(tweets, "cursor", retrievedAt)
	if err != nil {
		t.Fatalf("normalizeXTweets: %v", err)
	}

	for i, doc := range docs {
		if doc.Hash != "" {
			t.Errorf("doc[%d].Hash: got %q, want empty", i, doc.Hash)
		}
	}
}

// --- Test 20: TestNormalizeXTweetsMetadataKeys ---
func TestNormalizeXTweetsMetadataKeys(t *testing.T) {
	t.Parallel()
	retrievedAt := time.Now().UTC()

	tweet := XTweet{
		ID:           "meta-test",
		Text:         "Checking metadata keys",
		AuthorHandle: "metaman",
		URL:          "",
		LikeCount:    42,
		RepostCount:  13,
		ReplyCount:   7,
		QuoteCount:   2,
		CreatedAt:    "2026-06-04T12:00:00Z",
	}

	docs, err := normalizeXTweets([]XTweet{tweet}, "next-page", retrievedAt)
	if err != nil {
		t.Fatalf("normalizeXTweets: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("len(docs): got %d, want 1", len(docs))
	}

	meta := docs[0].Metadata

	// Required keys.
	requiredKeys := []string{
		"handle",
		"tweet_id",
		"like_count",
		"repost_count",
		"reply_count",
		"quote_count",
		"posted_at",
		"sub_source",
		"provider",
	}

	for _, key := range requiredKeys {
		if _, ok := meta[key]; !ok {
			t.Errorf("Metadata missing required key %q; got keys: %v", key, mapKeys(meta))
		}
	}

	// Check specific values.
	if meta["sub_source"] != "x" {
		t.Errorf("Metadata[sub_source]: got %v, want %q", meta["sub_source"], "x")
	}
	if meta["tweet_id"] != "meta-test" {
		t.Errorf("Metadata[tweet_id]: got %v, want %q", meta["tweet_id"], "meta-test")
	}
	if meta["handle"] != "metaman" {
		t.Errorf("Metadata[handle]: got %v, want %q", meta["handle"], "metaman")
	}

	// next_cursor should be on the last doc (this is the only doc).
	if nc, ok := meta["next_cursor"]; !ok {
		t.Error("Metadata missing next_cursor on last doc")
	} else if nc != "next-page" {
		t.Errorf("Metadata[next_cursor]: got %v, want %q", nc, "next-page")
	}
}

// --- Test 21: TestNormalizeXTweetsScore ---
func TestNormalizeXTweetsScore(t *testing.T) {
	t.Parallel()
	retrievedAt := time.Now().UTC()

	cases := []struct {
		name        string
		likeCount   int
		repostCount int
		wantTanh    float64
	}{
		{"zero engagement", 0, 0, 0.5},
		{"low engagement (10+5=15)", 10, 5, 0.0}, // computed below
		{"high engagement (100+50=150)", 100, 50, 0.0},
		{"saturated (5000+2000=7000)", 5000, 2000, 0.0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tweet := XTweet{
				ID:           "score-test",
				Text:         "score test",
				AuthorHandle: "scorer",
				LikeCount:    tc.likeCount,
				RepostCount:  tc.repostCount,
				CreatedAt:    "2026-06-04T12:00:00Z",
			}
			docs, err := normalizeXTweets([]XTweet{tweet}, "", retrievedAt)
			if err != nil {
				t.Fatalf("normalizeXTweets: %v", err)
			}

			// Compute expected score using the Tanh formula.
			x := float64(tc.likeCount + tc.repostCount)
			expected := math.Max(0.0, math.Min(1.0, 0.5+0.5*math.Tanh(x/100.0)))

			if math.Abs(docs[0].Score-expected) > 0.001 {
				t.Errorf("Score: got %v, want %v (tolerance ±0.001)", docs[0].Score, expected)
			}
		})
	}
}

// helper: extract map keys.
func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
