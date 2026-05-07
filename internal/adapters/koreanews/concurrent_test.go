// Package koreanews — concurrency and race-detection tests.
// SPEC-ADP-009 §11.5: adapter must be race-clean.
package koreanews_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/adapters/koreanews"
	"github.com/elymas/universal-search/pkg/types"
)

// TestSearch_concurrent verifies the adapter is safe under concurrent Search calls.
// Run with: go test -race ./internal/adapters/koreanews/...
func TestSearch_concurrent(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(filepath.Join("testdata", "rss_2_0.xml"))
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
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

	const goroutines = 10
	var wg sync.WaitGroup
	errs := make([]error, goroutines)

	for i := range goroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = a.Search(context.Background(), types.Query{Text: "한국"})
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: Search error: %v", i, err)
		}
	}
}

// TestSearch_parallelFeedFetches verifies MaxParallelFeeds limits concurrency correctly.
func TestSearch_parallelFeedFetches(t *testing.T) {
	t.Parallel()

	data, _ := os.ReadFile(filepath.Join("testdata", "rss_2_0.xml"))
	var mu sync.Mutex
	var concurrentPeak int
	var current int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		current++
		if current > concurrentPeak {
			concurrentPeak = current
		}
		mu.Unlock()

		// Small delay to let goroutines overlap.
		select {
		case <-time.After(10 * time.Millisecond):
		case <-r.Context().Done():
		}

		mu.Lock()
		current--
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	const feedCount = 8
	feeds := make([]string, feedCount)
	for i := range feeds {
		feeds[i] = srv.URL
	}

	const maxParallel = 3
	a, err := koreanews.New(koreanews.Options{
		RSSEnabled:        true,
		RSSFeeds:          feeds,
		MaxParallelFeeds:  maxParallel,
		RSSPerFeedTimeout: 5 * time.Second,
		HTTPClient:        srv.Client(),
		NowFunc:           func() time.Time { return time.Now() },
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Use empty query text to avoid filtering — we just want fetches to happen.
	// Actually we need non-empty text. Use a term that matches all items.
	_, _ = a.Search(context.Background(), types.Query{Text: "한국"})

	if concurrentPeak > maxParallel {
		t.Errorf("concurrentPeak = %d; want <= MaxParallelFeeds (%d)", concurrentPeak, maxParallel)
	}
}
