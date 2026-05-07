// Package social — integration-style tests for Search() and X stub.
// REQ-ADP6-001..010: full Search contract, X disabled stub, concurrent safety.
package social

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/elymas/universal-search/pkg/types"
)

// --- Bluesky Search tests ---

// TestBlueskySearchHappyPath verifies Search returns 25 docs from the happy-path fixture.
func TestBlueskySearchHappyPath(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile(testdataPath + "bluesky_search_response.json")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer ts.Close()

	a, _ := NewBluesky(BlueskyOptions{
		BaseURL:    ts.URL,
		HTTPClient: ts.Client(),
	})

	q := types.Query{Text: "golang"}
	docs, err := a.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search: unexpected error: %v", err)
	}
	if len(docs) != 25 {
		t.Errorf("len(docs): got %d, want 25", len(docs))
	}
}

// TestBlueskySearchEmptyResult verifies Search returns empty slice on no posts.
func TestBlueskySearchEmptyResult(t *testing.T) {
	t.Parallel()
	body, _ := os.ReadFile(testdataPath + "bluesky_search_response_empty.json")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer ts.Close()

	a, _ := NewBluesky(BlueskyOptions{BaseURL: ts.URL, HTTPClient: ts.Client()})
	docs, err := a.Search(context.Background(), types.Query{Text: "noresults"})
	if err != nil {
		t.Fatalf("Search: unexpected error: %v", err)
	}
	if len(docs) != 0 {
		t.Errorf("len(docs): got %d, want 0", len(docs))
	}
}

// TestBlueskySearchEmptyQueryReturnsError verifies empty query text is rejected.
func TestBlueskySearchEmptyQueryReturnsError(t *testing.T) {
	t.Parallel()
	a, _ := NewBluesky(BlueskyOptions{})
	_, err := a.Search(context.Background(), types.Query{Text: ""})
	if err == nil {
		t.Fatal("Search: expected error for empty query, got nil")
	}
	var se *types.SourceError
	if !errors.As(err, &se) {
		t.Fatalf("Search empty query: expected *types.SourceError, got %T", err)
	}
	if se.Category != types.CategoryPermanent {
		t.Errorf("Search empty query: Category got %v, want %v", se.Category, types.CategoryPermanent)
	}
}

// TestBlueskySearchWhitespaceQueryReturnsError verifies whitespace-only query is rejected.
func TestBlueskySearchWhitespaceQueryReturnsError(t *testing.T) {
	t.Parallel()
	a, _ := NewBluesky(BlueskyOptions{})
	_, err := a.Search(context.Background(), types.Query{Text: "   "})
	if err == nil {
		t.Fatal("Search: expected error for whitespace query, got nil")
	}
}

// TestBlueskySearchHTTP429RateLimited verifies 429 maps to CategoryRateLimited.
func TestBlueskySearchHTTP429RateLimited(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	a, _ := NewBluesky(BlueskyOptions{BaseURL: ts.URL, HTTPClient: ts.Client()})
	_, err := a.Search(context.Background(), types.Query{Text: "golang"})
	if err == nil {
		t.Fatal("Search: expected error for 429, got nil")
	}
	var se *types.SourceError
	if !errors.As(err, &se) {
		t.Fatalf("Search 429: expected *types.SourceError, got %T", err)
	}
	if se.Category != types.CategoryRateLimited {
		t.Errorf("Search 429: Category got %v, want %v", se.Category, types.CategoryRateLimited)
	}
}

// TestBlueskySearchHTTP500Unavailable verifies 500 maps to CategoryUnavailable.
func TestBlueskySearchHTTP500Unavailable(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	a, _ := NewBluesky(BlueskyOptions{BaseURL: ts.URL, HTTPClient: ts.Client()})
	_, err := a.Search(context.Background(), types.Query{Text: "golang"})
	if err == nil {
		t.Fatal("Search: expected error for 500, got nil")
	}
	var se *types.SourceError
	if !errors.As(err, &se) {
		t.Fatalf("Search 500: expected *types.SourceError, got %T", err)
	}
	if se.Category != types.CategoryUnavailable {
		t.Errorf("Search 500: Category got %v, want %v", se.Category, types.CategoryUnavailable)
	}
}

// TestBlueskySearchXRPCErrorEnvelope verifies XRPC error body (HTTP 200 with {error,message})
// is treated as an error.
func TestBlueskySearchXRPCErrorEnvelope(t *testing.T) {
	t.Parallel()
	body, _ := os.ReadFile(testdataPath + "bluesky_search_response_xrpc_error.json")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK) // 200 but error envelope
		_, _ = w.Write(body)
	}))
	defer ts.Close()

	a, _ := NewBluesky(BlueskyOptions{BaseURL: ts.URL, HTTPClient: ts.Client()})
	_, err := a.Search(context.Background(), types.Query{Text: "golang"})
	if err == nil {
		t.Fatal("Search: expected error for XRPC error envelope, got nil")
	}
}

// TestBlueskySearchMalformedJSON verifies malformed response body returns error.
func TestBlueskySearchMalformedJSON(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"cursor":"abc","posts":[{"uri":"at://did:plc:user001/app.bsky.feed.post/3abc`))
	}))
	defer ts.Close()

	a, _ := NewBluesky(BlueskyOptions{BaseURL: ts.URL, HTTPClient: ts.Client()})
	_, err := a.Search(context.Background(), types.Query{Text: "golang"})
	if err == nil {
		t.Fatal("Search: expected error for malformed JSON, got nil")
	}
}

// TestBlueskySearchContextCancelled verifies context cancellation propagates.
func TestBlueskySearchContextCancelled(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block until request context is cancelled.
		<-r.Context().Done()
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	a, _ := NewBluesky(BlueskyOptions{BaseURL: ts.URL, HTTPClient: ts.Client()})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := a.Search(ctx, types.Query{Text: "golang"})
	if err == nil {
		t.Fatal("Search: expected error for cancelled context, got nil")
	}
}

// TestBlueskySearchRequestParams verifies query string parameters are set correctly.
func TestBlueskySearchRequestParams(t *testing.T) {
	t.Parallel()

	var capturedQuery string
	var capturedLang string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.Query().Get("q")
		capturedLang = r.URL.Query().Get("lang")
		// Return empty posts to avoid parse errors.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"posts":[]}`))
	}))
	defer ts.Close()

	a, _ := NewBluesky(BlueskyOptions{BaseURL: ts.URL, HTTPClient: ts.Client()})

	q := types.Query{
		Text: "golang concurrency",
		Lang: "en",
	}
	_, _ = a.Search(context.Background(), q)

	if capturedQuery != "golang concurrency" {
		t.Errorf("query param q: got %q, want %q", capturedQuery, "golang concurrency")
	}
	if capturedLang != "en" {
		t.Errorf("query param lang: got %q, want %q", capturedLang, "en")
	}
}

// TestBlueskySearchCursorPropagation verifies cursor from Query.Cursor is sent as param.
func TestBlueskySearchCursorPropagation(t *testing.T) {
	t.Parallel()

	var capturedCursor string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCursor = r.URL.Query().Get("cursor")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"posts":[]}`))
	}))
	defer ts.Close()

	a, _ := NewBluesky(BlueskyOptions{BaseURL: ts.URL, HTTPClient: ts.Client()})

	q := types.Query{
		Text:   "golang",
		Cursor: "mycursor123",
	}
	_, _ = a.Search(context.Background(), q)

	if capturedCursor != "mycursor123" {
		t.Errorf("cursor param: got %q, want %q", capturedCursor, "mycursor123")
	}
}

// TestBlueskySearchMaxResults verifies limit parameter is sent.
func TestBlueskySearchMaxResults(t *testing.T) {
	t.Parallel()

	var capturedLimit string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedLimit = r.URL.Query().Get("limit")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"posts":[]}`))
	}))
	defer ts.Close()

	a, _ := NewBluesky(BlueskyOptions{BaseURL: ts.URL, HTTPClient: ts.Client()})

	q := types.Query{Text: "golang", MaxResults: 10}
	_, _ = a.Search(context.Background(), q)

	if capturedLimit == "" {
		t.Error("limit param: not sent, expected a value")
	}
}

// TestBlueskySearchConcurrentSafe verifies concurrent Search calls do not race.
// Use -race flag to detect data races.
func TestBlueskySearchConcurrentSafe(t *testing.T) {
	t.Parallel()
	body, _ := os.ReadFile(testdataPath + "bluesky_search_response.json")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer ts.Close()

	a, _ := NewBluesky(BlueskyOptions{BaseURL: ts.URL, HTTPClient: ts.Client()})

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)
	errs := make(chan error, goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()
			_, err := a.Search(context.Background(), types.Query{Text: "golang"})
			if err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent Search: unexpected error: %v", err)
	}
}

// --- X stub tests ---

// TestXSearchDisabledByDefault verifies Search returns ErrXDisabled when env not set.
func TestXSearchDisabledByDefault(t *testing.T) {
	t.Parallel()
	// EnvLookup returns "" (env var not set).
	a, _ := NewX(XOptions{
		EnvLookup: func(string) string { return "" },
	})

	_, err := a.Search(context.Background(), types.Query{Text: "test"})
	if err == nil {
		t.Fatal("X Search: expected ErrXDisabled, got nil")
	}
	if !errors.Is(err, ErrXDisabled) {
		t.Errorf("X Search: expected errors.Is(err, ErrXDisabled), got %v", err)
	}
}

// TestXSearchProviderNotConfigured verifies Search returns ErrXProviderNotConfigured
// when env="true" but no provider is wired.
func TestXSearchProviderNotConfigured(t *testing.T) {
	t.Parallel()
	// EnvLookup returns "true" (env var set to enable X) but no provider wired.
	a, _ := NewX(XOptions{
		EnvLookup: func(string) string { return "true" },
	})

	_, err := a.Search(context.Background(), types.Query{Text: "test"})
	if err == nil {
		t.Fatal("X Search: expected ErrXProviderNotConfigured, got nil")
	}
	if !errors.Is(err, ErrXProviderNotConfigured) {
		t.Errorf("X Search: expected errors.Is(err, ErrXProviderNotConfigured), got %v", err)
	}
}

// TestXSearchConcurrentEnvLookupSafe verifies concurrent X Search calls with
// EnvLookup injection are goroutine-safe (NO t.Setenv).
func TestXSearchConcurrentEnvLookupSafe(t *testing.T) {
	t.Parallel()
	a, _ := NewX(XOptions{
		EnvLookup: func(string) string { return "" },
	})

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()
			_, _ = a.Search(context.Background(), types.Query{Text: "concurrent"})
		}()
	}
	wg.Wait()
	// No race detected = test passes.
}

// TestXHealthcheckDisabled verifies Healthcheck returns error when X is disabled.
func TestXHealthcheckDisabled(t *testing.T) {
	t.Parallel()
	a, _ := NewX(XOptions{
		EnvLookup: func(string) string { return "" },
	})

	// Healthcheck should return an error for the disabled X stub.
	err := a.Healthcheck(context.Background())
	if err == nil {
		t.Error("X Healthcheck: expected error for disabled adapter, got nil")
	}
}

// TestBlueskySearchNoPanicOnNilBody verifies Search handles empty/nil body gracefully.
func TestBlueskySearchNoPanicOnNilBody(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Send empty body.
	}))
	defer ts.Close()

	a, _ := NewBluesky(BlueskyOptions{BaseURL: ts.URL, HTTPClient: ts.Client()})

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Search panicked: %v", r)
		}
	}()

	_, _ = a.Search(context.Background(), types.Query{Text: "golang"})
}

// --- JSON sentinel: ensure types package JSON is importable ---

// TestTypesQueryJSONRoundtrip is a sanity check that types.Query is usable.
func TestTypesQueryJSONRoundtrip(t *testing.T) {
	t.Parallel()
	q := types.Query{Text: "hello", MaxResults: 5}
	b, err := json.Marshal(q)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var q2 types.Query
	if err := json.Unmarshal(b, &q2); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if q2.Text != q.Text {
		t.Errorf("roundtrip Text: got %q, want %q", q2.Text, q.Text)
	}
}
