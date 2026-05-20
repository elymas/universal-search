// Package koreanews — composite Search dispatcher tests.
// SPEC-ADP-009 REQ-ADP9-001, REQ-ADP9-006, REQ-ADP9-010.
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

// TestSearch_invalidQuery_emptyText verifies ErrInvalidQuery on empty text.
func TestSearch_invalidQuery_emptyText(t *testing.T) {
	t.Parallel()

	a, _ := koreanews.New(koreanews.Options{
		RSSEnabled: true,
		RSSFeeds:   []string{"https://feed.example.com"},
	})

	_, err := a.Search(context.Background(), types.Query{Text: ""})
	if err == nil {
		t.Fatal("expected ErrInvalidQuery on empty text, got nil")
	}
	if !errors.Is(err, koreanews.ErrInvalidQuery) {
		t.Errorf("want ErrInvalidQuery; got %v", err)
	}
}

// TestSearch_invalidQuery_whitespaceOnly verifies ErrInvalidQuery on whitespace-only text.
func TestSearch_invalidQuery_whitespaceOnly(t *testing.T) {
	t.Parallel()

	a, _ := koreanews.New(koreanews.Options{
		RSSEnabled: true,
		RSSFeeds:   []string{"https://feed.example.com"},
	})

	_, err := a.Search(context.Background(), types.Query{Text: "   \t\n"})
	if err == nil {
		t.Fatal("expected ErrInvalidQuery on whitespace text, got nil")
	}
	if !errors.Is(err, koreanews.ErrInvalidQuery) {
		t.Errorf("want ErrInvalidQuery; got %v", err)
	}
}

// TestSearch_ErrInvalidQuery_isCategoryPermanent verifies CategoryPermanent wrapping.
func TestSearch_ErrInvalidQuery_isCategoryPermanent(t *testing.T) {
	t.Parallel()

	a, _ := koreanews.New(koreanews.Options{
		RSSEnabled: true,
		RSSFeeds:   []string{"https://feed.example.com"},
	})

	_, err := a.Search(context.Background(), types.Query{Text: ""})
	var se *types.SourceError
	if !errors.As(err, &se) {
		t.Fatalf("expected *types.SourceError; got %T", err)
	}
	if se.Category != types.CategoryPermanent {
		t.Errorf("Category = %v; want CategoryPermanent", se.Category)
	}
}

// TestSearch_daumEnabled_errorSilencedWhenRSSSucceeds verifies Daum errors are
// silently dropped when RSS provides results.
func TestSearch_daumEnabled_errorSilencedWhenRSSSucceeds(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(filepath.Join("testdata", "rss_2_0.xml"))
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	rssSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
	defer rssSrv.Close()

	a, err := koreanews.New(koreanews.Options{
		RSSEnabled:        true,
		RSSFeeds:          []string{rssSrv.URL},
		DaumEnabled:       true, // stub will return ErrDaumDisabled
		RSSPerFeedTimeout: 5 * time.Second,
		HTTPClient:        rssSrv.Client(),
		NowFunc:           func() time.Time { return time.Now() },
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	docs, searchErr := a.Search(context.Background(), types.Query{Text: "한국"})
	if searchErr != nil {
		t.Fatalf("Search should succeed when RSS provides docs; got error: %v", searchErr)
	}
	if len(docs) == 0 {
		t.Error("expected docs from RSS, got 0")
	}
}

// TestSearch_sortedByScoreDesc verifies results are sorted by score descending.
func TestSearch_sortedByScoreDesc(t *testing.T) {
	t.Parallel()

	data, _ := os.ReadFile(filepath.Join("testdata", "rss_2_0.xml"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	a, _ := koreanews.New(koreanews.Options{
		RSSEnabled:        true,
		RSSFeeds:          []string{srv.URL},
		RSSPerFeedTimeout: 5 * time.Second,
		HTTPClient:        srv.Client(),
		NowFunc:           func() time.Time { return time.Now() },
	})

	docs, err := a.Search(context.Background(), types.Query{Text: "한국"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	for i := 1; i < len(docs); i++ {
		if docs[i].Score > docs[i-1].Score {
			t.Errorf("docs not sorted by score desc: docs[%d].Score=%v > docs[%d].Score=%v",
				i, docs[i].Score, i-1, docs[i-1].Score)
		}
	}
}

// TestSearch_allDisabled_noPanic verifies no panic when all sub-sources disabled.
func TestSearch_allDisabled_noPanic(t *testing.T) {
	t.Parallel()

	a, err := koreanews.New(koreanews.Options{
		RSSEnabled:  false,
		KNCEnabled:  false,
		DaumEnabled: false,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// With all sub-sources disabled, there's no error path — returns zero docs.
	docs, searchErr := a.Search(context.Background(), types.Query{Text: "test"})
	_ = searchErr // may be nil or an error; no panic is the key invariant
	_ = docs
}

// TestSearch_contextAlreadyCancelled verifies cancelled context is handled gracefully.
func TestSearch_contextAlreadyCancelled(t *testing.T) {
	t.Parallel()

	data, _ := os.ReadFile(filepath.Join("testdata", "rss_2_0.xml"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	a, _ := koreanews.New(koreanews.Options{
		RSSEnabled:        true,
		RSSFeeds:          []string{srv.URL},
		RSSPerFeedTimeout: 5 * time.Second,
		HTTPClient:        srv.Client(),
		NowFunc:           func() time.Time { return time.Now() },
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before Search

	_, _ = a.Search(ctx, types.Query{Text: "test"})
	// No assertion on err: the key invariant is no goroutine leak (goleak).
}
