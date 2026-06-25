// Package redditrss_test — 429 cooldown-retry tests for the Reddit RSS adapter.
// SPEC-ADP-001b: Reddit's anonymous RSS endpoint rate-limits hard (~1 req per
// short window per IP). A single Search now secures a cooldown and re-issues the
// request up to maxAttempts times on HTTP 429 only; other errors are not retried.
package redditrss_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// fixtureBytes loads testdata/search.rss (4-doc fixture).
func fixtureBytes(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "search.rss"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return data
}

// TestSearch_RetriesOn429ThenSucceeds: 2x 429 then 200 → retry yields docs.
func TestSearch_RetriesOn429ThenSucceeds(t *testing.T) {
	t.Parallel()
	data := fixtureBytes(t)
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	a := newAdapter(t, srv.URL)
	a.SetRetryParamsForTest(time.Millisecond, 3) // tiny cooldown, 3 attempts

	docs, err := a.Search(context.Background(), types.Query{Text: "golang"})
	if err != nil {
		t.Fatalf("Search: unexpected error after retry: %v", err)
	}
	if len(docs) != 4 {
		t.Fatalf("len(docs) = %d; want 4", len(docs))
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("server calls = %d; want 3 (2 retries)", got)
	}
}

// TestSearch_429ExhaustsAttempts: always 429 → CategoryRateLimited after maxAttempts.
func TestSearch_429ExhaustsAttempts(t *testing.T) {
	t.Parallel()
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	a := newAdapter(t, srv.URL)
	a.SetRetryParamsForTest(time.Millisecond, 3)

	_, err := a.Search(context.Background(), types.Query{Text: "golang"})
	if err == nil {
		t.Fatal("Search: expected rate-limited error, got nil")
	}
	var se *types.SourceError
	if !errors.As(err, &se) || se.Category != types.CategoryRateLimited {
		t.Fatalf("error = %v; want CategoryRateLimited", err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("server calls = %d; want 3 (maxAttempts)", got)
	}
}

// TestSearch_NonRateLimitedNotRetried: 500 → returned after a single attempt.
func TestSearch_NonRateLimitedNotRetried(t *testing.T) {
	t.Parallel()
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	a := newAdapter(t, srv.URL)
	a.SetRetryParamsForTest(time.Millisecond, 3)

	_, err := a.Search(context.Background(), types.Query{Text: "golang"})
	if err == nil {
		t.Fatal("Search: expected error, got nil")
	}
	var se *types.SourceError
	if !errors.As(err, &se) || se.Category != types.CategoryUnavailable {
		t.Fatalf("error = %v; want CategoryUnavailable", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("server calls = %d; want 1 (no retry on non-429)", got)
	}
}

// TestSearch_ContextDeadlineDuringCooldown: ctx expires while waiting the
// cooldown → Search stops early with a context-shaped error, no extra request.
func TestSearch_ContextDeadlineDuringCooldown(t *testing.T) {
	t.Parallel()
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	a := newAdapter(t, srv.URL)
	a.SetRetryParamsForTest(200*time.Millisecond, 3) // cooldown >> ctx deadline

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := a.Search(ctx, types.Query{Text: "golang"})
	if err == nil {
		t.Fatal("Search: expected context error, got nil")
	}
	var se *types.SourceError
	if !errors.As(err, &se) || se.Category != types.CategoryTransient {
		t.Fatalf("error = %v; want CategoryTransient (deadline during cooldown)", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("server calls = %d; want 1 (cooldown exceeded ctx deadline)", got)
	}
}
