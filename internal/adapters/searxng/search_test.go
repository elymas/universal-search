package searxng_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/adapters/searxng"
	"github.com/elymas/universal-search/pkg/types"
)

// happyPathResponse builds a minimal valid SearXNG JSON response with n results.
func happyPathResponse(n int) []byte {
	type hit struct {
		URL     string  `json:"url"`
		Title   string  `json:"title"`
		Content string  `json:"content"`
		Engine  string  `json:"engine"`
		Score   float64 `json:"score"`
	}
	type resp struct {
		Query   string `json:"query"`
		Results []hit  `json:"results"`
	}
	r := resp{Query: "test"}
	for i := 0; i < n; i++ {
		r.Results = append(r.Results, hit{
			URL:     fmt.Sprintf("https://example.com/%d", i),
			Title:   fmt.Sprintf("Title %d", i),
			Content: fmt.Sprintf("Content %d", i),
			Engine:  "google",
			Score:   0.5,
		})
	}
	b, _ := json.Marshal(r)
	return b
}

// TestSearchHappyPath10Results verifies a 10-result response is fully returned.
func TestSearchHappyPath10Results(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(happyPathResponse(10))
	}))
	defer srv.Close()

	a, err := searxng.New(searxng.Options{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	docs, err := a.Search(testCtx(t), types.Query{Text: "test query"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(docs) != 10 {
		t.Errorf("len(docs) = %d, want 10", len(docs))
	}
}

// TestSearchURLAlwaysHasQAndFormat verifies q and format=json are always present.
func TestSearchURLAlwaysHasQAndFormat(t *testing.T) {
	t.Parallel()
	var capturedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"query":"test","results":[]}`))
	}))
	defer srv.Close()

	a, err := searxng.New(searxng.Options{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = a.Search(testCtx(t), types.Query{Text: "my query"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	parsed, err := url.ParseRequestURI(capturedURL)
	if err != nil {
		t.Fatalf("ParseRequestURI(%q): %v", capturedURL, err)
	}
	q := parsed.Query()
	if q.Get("q") == "" {
		t.Error("URL missing 'q' parameter")
	}
	if q.Get("format") != "json" {
		t.Errorf("format = %q, want %q", q.Get("format"), "json")
	}
}

// TestSearchOmitsPagenoWhenCursorEmpty verifies pageno is absent when cursor="".
func TestSearchOmitsPagenoWhenCursorEmpty(t *testing.T) {
	t.Parallel()
	var capturedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"query":"test","results":[]}`))
	}))
	defer srv.Close()

	a, err := searxng.New(searxng.Options{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = a.Search(testCtx(t), types.Query{Text: "test", Cursor: ""})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	parsed, _ := url.ParseRequestURI(capturedURL)
	if parsed.Query().Get("pageno") != "" {
		t.Errorf("pageno = %q, want absent when cursor=''", parsed.Query().Get("pageno"))
	}
}

// TestSearchSetsPagenoWhenCursorPresent verifies pageno=cursor when cursor non-empty.
func TestSearchSetsPagenoWhenCursorPresent(t *testing.T) {
	t.Parallel()
	var capturedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"query":"test","results":[]}`))
	}))
	defer srv.Close()

	a, err := searxng.New(searxng.Options{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = a.Search(testCtx(t), types.Query{Text: "test", Cursor: "3"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	parsed, _ := url.ParseRequestURI(capturedURL)
	if got := parsed.Query().Get("pageno"); got != "3" {
		t.Errorf("pageno = %q, want %q", got, "3")
	}
}

// TestSearchSetsPagenoExplicitOne verifies cursor="1" → pageno=1 (H5 audit fix).
func TestSearchSetsPagenoExplicitOne(t *testing.T) {
	t.Parallel()
	var capturedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"query":"test","results":[]}`))
	}))
	defer srv.Close()

	a, err := searxng.New(searxng.Options{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = a.Search(testCtx(t), types.Query{Text: "test", Cursor: "1"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	parsed, _ := url.ParseRequestURI(capturedURL)
	if got := parsed.Query().Get("pageno"); got != "1" {
		t.Errorf("pageno = %q, want %q (H5: cursor=1 → explicit pageno=1)", got, "1")
	}
}

// TestSearchHTTP429RateLimited verifies 429 → CategoryRateLimited.
func TestSearchHTTP429RateLimited(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	a, err := searxng.New(searxng.Options{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, searchErr := a.Search(testCtx(t), types.Query{Text: "test"})
	if searchErr == nil {
		t.Fatal("want error for 429")
	}
	var se *types.SourceError
	if !asSourceError(searchErr, &se) {
		t.Fatalf("error type = %T, want *SourceError", searchErr)
	}
	if se.Category != types.CategoryRateLimited {
		t.Errorf("Category = %v, want CategoryRateLimited", se.Category)
	}
}

// TestSearchHTTP429WithRetryAfterParsed verifies RetryAfter is populated on 429.
func TestSearchHTTP429WithRetryAfterParsed(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "15")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	a, err := searxng.New(searxng.Options{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, searchErr := a.Search(testCtx(t), types.Query{Text: "test"})
	var se *types.SourceError
	if !asSourceError(searchErr, &se) {
		t.Fatalf("error type = %T", searchErr)
	}
	if se.RetryAfter != 15*time.Second {
		t.Errorf("RetryAfter = %v, want 15s", se.RetryAfter)
	}
}

// TestSearchHTTP403WithRetryAfterIsRateLimited verifies REQ-ADP7-007.
func TestSearchHTTP403WithRetryAfterIsRateLimited(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "10")
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	a, err := searxng.New(searxng.Options{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, searchErr := a.Search(testCtx(t), types.Query{Text: "test"})
	var se *types.SourceError
	if !asSourceError(searchErr, &se) {
		t.Fatalf("error type = %T", searchErr)
	}
	if se.Category != types.CategoryRateLimited {
		t.Errorf("Category = %v, want CategoryRateLimited (REQ-ADP7-007)", se.Category)
	}
}

// TestSearchHTTP403WithoutRetryAfterIsPermanent verifies 403 without Retry-After → Permanent.
func TestSearchHTTP403WithoutRetryAfterIsPermanent(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	a, err := searxng.New(searxng.Options{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, searchErr := a.Search(testCtx(t), types.Query{Text: "test"})
	var se *types.SourceError
	if !asSourceError(searchErr, &se) {
		t.Fatalf("error type = %T", searchErr)
	}
	if se.Category != types.CategoryPermanent {
		t.Errorf("Category = %v, want CategoryPermanent", se.Category)
	}
}

// TestSearchHTTP4xxPermanent verifies 4xx non-special codes → Permanent.
func TestSearchHTTP4xxPermanent(t *testing.T) {
	t.Parallel()
	for _, code := range []int{400, 404, 410, 422} {
		code := code
		t.Run(fmt.Sprintf("%d", code), func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
			}))
			defer srv.Close()

			a, err := searxng.New(searxng.Options{BaseURL: srv.URL})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			_, searchErr := a.Search(testCtx(t), types.Query{Text: "test"})
			var se *types.SourceError
			if !asSourceError(searchErr, &se) {
				t.Fatalf("error type = %T", searchErr)
			}
			if se.Category != types.CategoryPermanent {
				t.Errorf("HTTP %d: Category = %v, want CategoryPermanent", code, se.Category)
			}
		})
	}
}

// TestSearchHTTP5xxUnavailable verifies 5xx → Unavailable.
func TestSearchHTTP5xxUnavailable(t *testing.T) {
	t.Parallel()
	for _, code := range []int{500, 502, 503} {
		code := code
		t.Run(fmt.Sprintf("%d", code), func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
			}))
			defer srv.Close()

			a, err := searxng.New(searxng.Options{BaseURL: srv.URL})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			_, searchErr := a.Search(testCtx(t), types.Query{Text: "test"})
			var se *types.SourceError
			if !asSourceError(searchErr, &se) {
				t.Fatalf("error type = %T", searchErr)
			}
			if se.Category != types.CategoryUnavailable {
				t.Errorf("HTTP %d: Category = %v, want CategoryUnavailable", code, se.Category)
			}
		})
	}
}

// TestSearchConnectionRefused verifies network error → Unavailable with HTTPStatus=0.
func TestSearchConnectionRefused(t *testing.T) {
	t.Parallel()
	// Use a closed server so connection is refused.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srvURL := srv.URL
	srv.Close() // Close immediately so subsequent dials fail.

	a, err := searxng.New(searxng.Options{BaseURL: srvURL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, searchErr := a.Search(ctx, types.Query{Text: "test"})
	if searchErr == nil {
		t.Fatal("want error for connection refused")
	}
	var se *types.SourceError
	if !asSourceError(searchErr, &se) {
		t.Fatalf("error type = %T", searchErr)
	}
	if se.Category != types.CategoryUnavailable {
		t.Errorf("Category = %v, want CategoryUnavailable for network error", se.Category)
	}
	if se.HTTPStatus != 0 {
		t.Errorf("HTTPStatus = %d, want 0 for network error", se.HTTPStatus)
	}
}

// TestSearchUnavailablePreservesUnderlyingError verifies Cause is set on Unavailable.
func TestSearchUnavailablePreservesUnderlyingError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srvURL := srv.URL
	srv.Close()

	a, err := searxng.New(searxng.Options{BaseURL: srvURL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, searchErr := a.Search(ctx, types.Query{Text: "test"})
	var se *types.SourceError
	if !asSourceError(searchErr, &se) {
		t.Fatalf("error type = %T", searchErr)
	}
	if se.Cause == nil {
		t.Error("Cause = nil, want underlying error")
	}
}

// TestSearchEmptyQueryRejectedNoHTTP verifies ErrInvalidQuery without HTTP request.
func TestSearchEmptyQueryRejectedNoHTTP(t *testing.T) {
	t.Parallel()
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a, err := searxng.New(searxng.Options{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	tests := []struct {
		name string
		text string
	}{
		{"empty string", ""},
		{"whitespace only", "   "},
		{"tabs", "\t\t"},
		{"mixed whitespace", " \t\n "},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, searchErr := a.Search(testCtx(t), types.Query{Text: tc.text})
			if searchErr == nil {
				t.Fatal("want error for empty/whitespace query")
			}
			if called {
				t.Error("HTTP request was made for invalid query, want no HTTP request")
			}
			var se *types.SourceError
			if !asSourceError(searchErr, &se) {
				t.Fatalf("error type = %T, want *SourceError", searchErr)
			}
			if se.Category != types.CategoryPermanent {
				t.Errorf("Category = %v, want CategoryPermanent", se.Category)
			}
			if !errors.Is(searchErr, types.ErrPermanent) {
				t.Errorf("errors.Is(ErrPermanent) = false, want true")
			}
		})
	}
}

// TestSearchInvalidCursorRejectedNoHTTP verifies ErrInvalidCursor without HTTP request.
func TestSearchInvalidCursorRejectedNoHTTP(t *testing.T) {
	t.Parallel()
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a, err := searxng.New(searxng.Options{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	tests := []struct {
		name   string
		cursor string
	}{
		{"negative", "-1"},
		{"zero", "0"},
		{"non-numeric", "abc"},
		{"float", "1.5"},
		{"empty-not-invalid", ""}, // cursor="" is valid (first page)
	}
	for _, tc := range tests {
		tc := tc
		// Only test cases expected to fail.
		if tc.cursor == "" {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, searchErr := a.Search(testCtx(t), types.Query{Text: "test", Cursor: tc.cursor})
			if searchErr == nil {
				t.Fatalf("cursor=%q: want error, got nil", tc.cursor)
			}
			if called {
				t.Error("HTTP request was made for invalid cursor, want no HTTP request")
			}
			var se *types.SourceError
			if !asSourceError(searchErr, &se) {
				t.Fatalf("error type = %T, want *SourceError", searchErr)
			}
			if se.Category != types.CategoryPermanent {
				t.Errorf("cursor=%q: Category = %v, want CategoryPermanent", tc.cursor, se.Category)
			}
		})
	}
}

// TestSearchConcurrentSafe verifies concurrent Search calls don't race.
func TestSearchConcurrentSafe(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(happyPathResponse(3))
	}))
	defer srv.Close()

	a, err := searxng.New(searxng.Options{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_, _ = a.Search(testCtx(t), types.Query{Text: "concurrent test"})
		}()
	}
	wg.Wait()
}

// TestSearchSetsCustomUserAgent verifies User-Agent header is set.
func TestSearchSetsCustomUserAgent(t *testing.T) {
	t.Parallel()
	var capturedUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"query":"test","results":[]}`))
	}))
	defer srv.Close()

	a, err := searxng.New(searxng.Options{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, _ = a.Search(testCtx(t), types.Query{Text: "test"})

	if capturedUA == "" {
		t.Error("User-Agent header is empty")
	}
	if !strings.Contains(capturedUA, "usearch/") {
		t.Errorf("User-Agent = %q, want usearch/ prefix", capturedUA)
	}
}

// TestSearchSetsAcceptJSON verifies Accept: application/json header is set.
func TestSearchSetsAcceptJSON(t *testing.T) {
	t.Parallel()
	var capturedAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"query":"test","results":[]}`))
	}))
	defer srv.Close()

	a, err := searxng.New(searxng.Options{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, _ = a.Search(testCtx(t), types.Query{Text: "test"})

	if capturedAccept != "application/json" {
		t.Errorf("Accept = %q, want %q", capturedAccept, "application/json")
	}
}

// TestSearchUserAgentVersionConfigurable verifies UserAgentVersion is used in UA.
func TestSearchUserAgentVersionConfigurable(t *testing.T) {
	t.Parallel()
	var capturedUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"query":"test","results":[]}`))
	}))
	defer srv.Close()

	a, err := searxng.New(searxng.Options{
		BaseURL:          srv.URL,
		UserAgentVersion: "v1.2.3-custom",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, _ = a.Search(testCtx(t), types.Query{Text: "test"})

	if !strings.Contains(capturedUA, "v1.2.3-custom") {
		t.Errorf("User-Agent = %q, want v1.2.3-custom component", capturedUA)
	}
}

// TestSearchNoAuthorizationHeader verifies Authorization header is never sent.
func TestSearchNoAuthorizationHeader(t *testing.T) {
	t.Parallel()
	var capturedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"query":"test","results":[]}`))
	}))
	defer srv.Close()

	a, err := searxng.New(searxng.Options{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, _ = a.Search(testCtx(t), types.Query{Text: "test"})

	if capturedAuth != "" {
		t.Errorf("Authorization = %q, want empty (no auth)", capturedAuth)
	}
}

// TestSearchNoGoroutineLeakOnCancel verifies no goroutine leak when context is cancelled.
// Note: goleak.VerifyTestMain in bench_test.go handles suite-level leak detection.
// This test verifies the cancel path specifically.
func TestSearchNoGoroutineLeakOnCancel(t *testing.T) {
	t.Parallel()
	// Slow server that blocks.
	ready := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(ready)
		// Hold connection until client disconnects.
		<-r.Context().Done()
	}))
	defer srv.Close()

	a, err := searxng.New(searxng.Options{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := a.Search(ctx, types.Query{Text: "test"})
		done <- err
	}()

	// Wait until server is reached, then cancel.
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("server not reached in time")
	}
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Error("Search returned nil after context cancel")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Search did not return after context cancel")
	}
}

// TestSearchEmptyResultsNoError verifies empty results return nil error with nil slice.
func TestSearchEmptyResultsNoError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"query":"test","results":[]}`))
	}))
	defer srv.Close()

	a, err := searxng.New(searxng.Options{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	docs, err := a.Search(testCtx(t), types.Query{Text: "test"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if docs != nil {
		t.Errorf("docs = %v, want nil for empty results", docs)
	}
}

// TestSearchMalformedJSONResponseIsPermanent verifies malformed JSON → Permanent error.
func TestSearchMalformedJSONResponseIsPermanent(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"query": "test", "results": [{"url": "https://bad`))
	}))
	defer srv.Close()

	a, err := searxng.New(searxng.Options{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, searchErr := a.Search(testCtx(t), types.Query{Text: "test"})
	if searchErr == nil {
		t.Fatal("want error for malformed JSON")
	}
	var se *types.SourceError
	if !asSourceError(searchErr, &se) {
		t.Fatalf("error type = %T, want *SourceError", searchErr)
	}
	if se.Category != types.CategoryPermanent {
		t.Errorf("Category = %v, want CategoryPermanent for malformed JSON", se.Category)
	}
}

// TestSearchDocTypesAllArticle verifies all returned docs have DocTypeArticle (H1 audit).
func TestSearchDocTypesAllArticle(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(happyPathResponse(5))
	}))
	defer srv.Close()

	a, err := searxng.New(searxng.Options{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	docs, err := a.Search(testCtx(t), types.Query{Text: "test"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	for i, d := range docs {
		if d.DocType != types.DocTypeArticle {
			t.Errorf("docs[%d].DocType = %v, want DocTypeArticle", i, d.DocType)
		}
	}
}

// TestSearchAdapterNameInError verifies SourceError.Adapter = "searxng".
func TestSearchAdapterNameInError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	a, err := searxng.New(searxng.Options{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, searchErr := a.Search(testCtx(t), types.Query{Text: "test"})
	var se *types.SourceError
	if !asSourceError(searchErr, &se) {
		t.Fatalf("error type = %T", searchErr)
	}
	if se.Adapter != "searxng" {
		t.Errorf("Adapter = %q, want %q", se.Adapter, "searxng")
	}
}
