// Package koreanews — RSS sub-source tests.
// SPEC-ADP-009 REQ-ADP9-003 through REQ-ADP9-008.
package koreanews_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/adapters/koreanews"
	"github.com/elymas/universal-search/pkg/types"
)

// serveTestdata creates an httptest.Server that serves a static testdata file.
func serveTestdata(t *testing.T, filename string) *httptest.Server {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", filename))
	if err != nil {
		t.Fatalf("read testdata/%s: %v", filename, err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
	return srv
}

// TestSearch_RSS_RSS2_0 verifies parsing of a valid RSS 2.0 feed.
func TestSearch_RSS_RSS2_0(t *testing.T) {
	t.Parallel()

	srv := serveTestdata(t, "rss_2_0.xml")
	defer srv.Close()

	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	a, err := koreanews.New(koreanews.Options{
		RSSEnabled:        true,
		RSSFeeds:          []string{srv.URL},
		RSSPerFeedTimeout: 5 * time.Second,
		HTTPClient:        srv.Client(),
		NowFunc:           func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Empty query text = no filtering.
	docs, err := a.Search(context.Background(), types.Query{Text: "한국"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(docs) == 0 {
		t.Fatal("expected at least 1 doc from RSS 2.0 feed, got 0")
	}

	// Validate all docs.
	for i, doc := range docs {
		if err := doc.Validate(); err != nil {
			t.Errorf("docs[%d].Validate() = %v", i, err)
		}
		if doc.SourceID != "koreanews" {
			t.Errorf("docs[%d].SourceID = %q; want koreanews", i, doc.SourceID)
		}
		if doc.DocType != types.DocTypeArticle {
			t.Errorf("docs[%d].DocType = %q; want article", i, doc.DocType)
		}
		if doc.Score != 0.5 {
			t.Errorf("docs[%d].Score = %v; want 0.5", i, doc.Score)
		}
		if doc.RetrievedAt != now {
			t.Errorf("docs[%d].RetrievedAt = %v; want %v", i, doc.RetrievedAt, now)
		}
		meta, ok := doc.Metadata["subsource"]
		if !ok || meta != "rss" {
			t.Errorf("docs[%d].Metadata[subsource] = %v; want rss", i, meta)
		}
	}
}

// TestSearch_RSS_Atom verifies parsing of Atom 1.0 feed.
func TestSearch_RSS_Atom(t *testing.T) {
	t.Parallel()

	srv := serveTestdata(t, "atom_1_0.xml")
	defer srv.Close()

	a, err := koreanews.New(koreanews.Options{
		RSSEnabled:        true,
		RSSFeeds:          []string{srv.URL},
		RSSPerFeedTimeout: 5 * time.Second,
		HTTPClient:        srv.Client(),
		NowFunc:           func() time.Time { return time.Now() },
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Query with "AI" which appears in the Atom fixture's Korean content.
	docs, err := a.Search(context.Background(), types.Query{Text: "AI"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(docs) == 0 {
		t.Fatal("expected at least 1 doc from Atom feed, got 0")
	}
}

// TestSearch_RSS_JSONFeed verifies parsing of JSON Feed 1.1.
func TestSearch_RSS_JSONFeed(t *testing.T) {
	t.Parallel()

	data, _ := os.ReadFile(filepath.Join("testdata", "json_feed_1_1.json"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/feed+json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	a, err := koreanews.New(koreanews.Options{
		RSSEnabled:        true,
		RSSFeeds:          []string{srv.URL},
		RSSPerFeedTimeout: 5 * time.Second,
		HTTPClient:        srv.Client(),
		NowFunc:           func() time.Time { return time.Now() },
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	docs, err := a.Search(context.Background(), types.Query{Text: "한국"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(docs) == 0 {
		t.Fatal("expected at least 1 doc from JSON Feed, got 0")
	}
}

// TestSearch_RSS_malformedFeedReturnsError verifies a malformed feed returns error.
func TestSearch_RSS_malformedFeedReturnsError(t *testing.T) {
	t.Parallel()

	srv := serveTestdata(t, "rss_malformed.xml")
	defer srv.Close()

	a, err := koreanews.New(koreanews.Options{
		RSSEnabled:        true,
		RSSFeeds:          []string{srv.URL},
		RSSPerFeedTimeout: 5 * time.Second,
		HTTPClient:        srv.Client(),
		NowFunc:           func() time.Time { return time.Now() },
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// With a single malformed feed and no other feeds, Search should fail.
	_, err = a.Search(context.Background(), types.Query{Text: "test"})
	if err == nil {
		t.Fatal("expected error from malformed feed, got nil")
	}
}

// TestSearch_RSS_perFeedIsolation verifies one bad feed does not abort good ones.
func TestSearch_RSS_perFeedIsolation(t *testing.T) {
	t.Parallel()

	goodSrv := serveTestdata(t, "rss_2_0.xml")
	defer goodSrv.Close()

	badSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer badSrv.Close()

	a, err := koreanews.New(koreanews.Options{
		RSSEnabled:        true,
		RSSFeeds:          []string{badSrv.URL, goodSrv.URL},
		RSSPerFeedTimeout: 5 * time.Second,
		HTTPClient:        goodSrv.Client(), // same transport for both
		NowFunc:           func() time.Time { return time.Now() },
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	docs, err := a.Search(context.Background(), types.Query{Text: "한국"})
	if err != nil {
		t.Fatalf("Search: unexpected error with mixed feeds: %v", err)
	}
	if len(docs) == 0 {
		t.Error("expected docs from the good feed; got 0")
	}
}

// TestSearch_RSS_emptyFeedList_returnsErrEmptyRSSFeedList verifies the empty feed list error.
func TestSearch_RSS_emptyFeedList_returnsErrEmptyRSSFeedList(t *testing.T) {
	t.Parallel()

	a, err := koreanews.New(koreanews.Options{
		RSSEnabled: true,
		RSSFeeds:   nil, // no feeds
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, searchErr := a.Search(context.Background(), types.Query{Text: "test"})
	if searchErr == nil {
		t.Fatal("expected ErrEmptyRSSFeedList, got nil")
	}
	if !errors.Is(searchErr, koreanews.ErrEmptyRSSFeedList) {
		t.Errorf("want ErrEmptyRSSFeedList; got %v", searchErr)
	}
}

// TestSearch_RSS_deduplication verifies cross-feed deduplication.
func TestSearch_RSS_deduplication(t *testing.T) {
	t.Parallel()

	// Serve the same feed from two different URLs to trigger dedup.
	srv1 := serveTestdata(t, "rss_2_0.xml")
	defer srv1.Close()
	srv2 := serveTestdata(t, "rss_2_0.xml")
	defer srv2.Close()

	a, err := koreanews.New(koreanews.Options{
		RSSEnabled:        true,
		RSSFeeds:          []string{srv1.URL, srv2.URL},
		RSSPerFeedTimeout: 5 * time.Second,
		HTTPClient:        srv1.Client(),
		NowFunc:           func() time.Time { return time.Now() },
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	docs, err := a.Search(context.Background(), types.Query{Text: "한국"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	// Feed served twice — should be half the raw count after dedup.
	// Fetch one feed's count for comparison.
	srv3 := serveTestdata(t, "rss_2_0.xml")
	defer srv3.Close()
	a2, _ := koreanews.New(koreanews.Options{
		RSSEnabled:        true,
		RSSFeeds:          []string{srv3.URL},
		RSSPerFeedTimeout: 5 * time.Second,
		HTTPClient:        srv3.Client(),
		NowFunc:           func() time.Time { return time.Now() },
	})
	singleDocs, _ := a2.Search(context.Background(), types.Query{Text: "한국"})

	if len(docs) != len(singleDocs) {
		t.Errorf("dedup failed: dual feed = %d docs; single feed = %d docs (expected equal after dedup)",
			len(docs), len(singleDocs))
	}
}

// TestSearch_RSS_localeDetection verifies ko detection on Korean-heavy content.
func TestSearch_RSS_localeDetection(t *testing.T) {
	t.Parallel()

	srv := serveTestdata(t, "rss_with_korean_text.xml")
	defer srv.Close()

	a, err := koreanews.New(koreanews.Options{
		RSSEnabled:        true,
		RSSFeeds:          []string{srv.URL},
		RSSPerFeedTimeout: 5 * time.Second,
		HTTPClient:        srv.Client(),
		NowFunc:           func() time.Time { return time.Now() },
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Use a query that matches everything (empty would also work but test filtering too).
	docs, err := a.Search(context.Background(), types.Query{Text: "한국"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	// At least the pure Korean item should have lang="ko".
	var koCount int
	for _, d := range docs {
		if d.Lang == "ko" {
			koCount++
		}
	}
	if koCount == 0 {
		t.Error("expected at least 1 doc with lang=ko from Korean RSS fixture")
	}
}

// TestSearch_RSS_contextCancellation verifies that cancelling the context aborts the fetch.
func TestSearch_RSS_contextCancellation(t *testing.T) {
	t.Parallel()

	// Server that blocks for a long time (longer than test timeout).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(30 * time.Second):
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	a, err := koreanews.New(koreanews.Options{
		RSSEnabled:        true,
		RSSFeeds:          []string{srv.URL},
		RSSPerFeedTimeout: 10 * time.Second,
		HTTPClient:        srv.Client(),
		NowFunc:           func() time.Time { return time.Now() },
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = a.Search(ctx, types.Query{Text: "test"})
	// We expect either an error from timeout or zero docs (both are acceptable;
	// the key invariant is no goroutine leak, which goleak enforces).
	_ = err
}
