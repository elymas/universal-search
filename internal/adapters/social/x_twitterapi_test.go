package social

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/elymas/universal-search/pkg/types"
)

// TestTwitterAPIProviderSearchTweets verifies the advanced_search response is
// parsed into XTweet and pagination cursor honors has_next_page.
func TestTwitterAPIProviderSearchTweets(t *testing.T) {
	const body = `{
		"tweets": [
			{"id":"1","url":"https://x.com/u/status/1","text":"sonnet5 on partner API",
			 "createdAt":"Tue Jun 10 07:00:00 +0000 2026",
			 "likeCount":12,"retweetCount":3,"replyCount":1,"quoteCount":2,
			 "author":{"userName":"alice"}}
		],
		"has_next_page": true,
		"next_cursor": "CURSOR2"
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-API-Key"); got != "k" {
			t.Errorf("X-API-Key = %q, want %q", got, "k")
		}
		if got := r.URL.Query().Get("queryType"); got != "Latest" {
			t.Errorf("queryType = %q, want Latest", got)
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	prov, err := NewTwitterAPIProvider(TwitterAPIOptions{APIKey: "k", BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("NewTwitterAPIProvider: %v", err)
	}

	tweets, cursor, err := prov.SearchTweets(context.Background(), types.Query{Text: "sonnet5"})
	if err != nil {
		t.Fatalf("SearchTweets: %v", err)
	}
	if len(tweets) != 1 {
		t.Fatalf("len(tweets) = %d, want 1", len(tweets))
	}
	got := tweets[0]
	if got.ID != "1" || got.AuthorHandle != "alice" || got.LikeCount != 12 || got.RepostCount != 3 {
		t.Errorf("tweet mismatch: %+v", got)
	}
	if cursor != "CURSOR2" {
		t.Errorf("cursor = %q, want CURSOR2", cursor)
	}
}

// TestTwitterAPIProviderNoCursorWhenNoNextPage verifies cursor is empty when
// has_next_page is false even if next_cursor is populated.
func TestTwitterAPIProviderNoCursorWhenNoNextPage(t *testing.T) {
	const body = `{"tweets":[],"has_next_page":false,"next_cursor":"X"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	prov, _ := NewTwitterAPIProvider(TwitterAPIOptions{APIKey: "k", BaseURL: srv.URL})
	_, cursor, err := prov.SearchTweets(context.Background(), types.Query{Text: "q"})
	if err != nil {
		t.Fatalf("SearchTweets: %v", err)
	}
	if cursor != "" {
		t.Errorf("cursor = %q, want empty", cursor)
	}
}
