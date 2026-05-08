// Package access — concurrency tests for the 5-phase cascade.
//
// NFR-CACHE-001: Fetcher is safe for concurrent use.
// NFR-CACHE-005: No goroutine leaks (write-through WG).
package access

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestFetch_ConcurrentFetches(t *testing.T) {
	// NOT t.Parallel() — this test stresses concurrency internally.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>concurrent ok</html>"))
	}))
	defer srv.Close()

	f := newTestFetcher(t)

	const goroutines = 10
	var wg sync.WaitGroup
	errCh := make(chan error, goroutines)

	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := f.Fetch(t.Context(), srv.URL+"/page", FetchOptions{})
			if err != nil {
				errCh <- err
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent Fetch error: %v", err)
	}
}

func TestShutdown_WhileFetchInFlight(t *testing.T) {
	t.Parallel()
	// Trigger shutdown while a write-through goroutine may be active.
	lookup := &countingIndexLookup{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("<html>ok</html>"))
	}))
	defer srv.Close()

	f, err := New(Options{
		AllowPrivateNetworks: true,
		CacheWriteThrough:    true,
		IndexLookup:          lookup,
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Fetch and trigger write-through.
	_, _ = f.Fetch(t.Context(), srv.URL+"/page", FetchOptions{})

	// Shutdown should drain without deadlock.
	if err := f.Shutdown(t.Context()); err != nil {
		t.Errorf("Shutdown() error: %v", err)
	}
}
