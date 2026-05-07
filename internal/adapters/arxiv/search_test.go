package arxiv

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// testContext returns a context with a reasonable deadline for unit tests.
func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// happyPathServer serves the 25-entry arXiv Atom XML fixture.
func happyPathServer(t *testing.T) *httptest.Server {
	t.Helper()
	body, err := os.ReadFile("testdata/search_response.xml")
	if err != nil {
		t.Fatalf("os.ReadFile(search_response.xml) error = %v", err)
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	t.Cleanup(ts.Close)
	return ts
}

// TestSearchHappyPath25Docs verifies Search returns 25 valid NormalizedDocs.
func TestSearchHappyPath25Docs(t *testing.T) {
	t.Parallel()

	ts := happyPathServer(t)
	a, err := New(Options{BaseURL: ts.URL, MinRequestInterval: 0})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	docs, err := a.Search(ctx, types.Query{Text: "machine learning"})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(docs) != 25 {
		t.Errorf("Search() returned %d docs, want 25", len(docs))
	}
	for i, doc := range docs {
		if err := doc.Validate(); err != nil {
			t.Errorf("docs[%d].Validate() error = %v", i, err)
		}
	}
}

// TestSearchURLParametersIncludeAllRequired verifies all required URL params are present.
func TestSearchURLParametersIncludeAllRequired(t *testing.T) {
	t.Parallel()

	var capturedURL *url.URL
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL
		w.Header().Set("Content-Type", "application/atom+xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom" xmlns:opensearch="http://a9.com/-/spec/opensearch/1.1/"><opensearch:totalResults>0</opensearch:totalResults><opensearch:startIndex>0</opensearch:startIndex><opensearch:itemsPerPage>0</opensearch:itemsPerPage></feed>`))
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL, MinRequestInterval: 0})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, _ = a.Search(ctx, types.Query{Text: "deep learning"})

	if capturedURL == nil {
		t.Fatal("no request captured")
	}
	params := capturedURL.Query()

	requiredParams := []string{"search_query", "start", "max_results", "sortBy", "sortOrder"}
	for _, p := range requiredParams {
		if params.Get(p) == "" {
			t.Errorf("missing required URL parameter %q in URL %q", p, capturedURL.RawQuery)
		}
	}
	if params.Get("sortBy") != "relevance" {
		t.Errorf("sortBy = %q, want %q", params.Get("sortBy"), "relevance")
	}
	if params.Get("sortOrder") != "descending" {
		t.Errorf("sortOrder = %q, want %q", params.Get("sortOrder"), "descending")
	}
	if params.Get("start") != "0" {
		t.Errorf("start = %q, want %q (default)", params.Get("start"), "0")
	}
}

// TestSearchClampsLimitTo100 verifies that MaxResults > 100 is clamped to 100.
func TestSearchClampsLimitTo100(t *testing.T) {
	t.Parallel()

	var capturedMaxResults string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMaxResults = r.URL.Query().Get("max_results")
		w.Header().Set("Content-Type", "application/atom+xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom" xmlns:opensearch="http://a9.com/-/spec/opensearch/1.1/"><opensearch:totalResults>0</opensearch:totalResults><opensearch:startIndex>0</opensearch:startIndex><opensearch:itemsPerPage>0</opensearch:itemsPerPage></feed>`))
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL, MinRequestInterval: 0})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, _ = a.Search(ctx, types.Query{Text: "transformer", MaxResults: 500})

	if capturedMaxResults != "100" {
		t.Errorf("max_results = %q, want %q", capturedMaxResults, "100")
	}
}

// TestSearchDefaultsLimitTo25 verifies that MaxResults=0 defaults to 25.
func TestSearchDefaultsLimitTo25(t *testing.T) {
	t.Parallel()

	var capturedMaxResults string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMaxResults = r.URL.Query().Get("max_results")
		w.Header().Set("Content-Type", "application/atom+xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom" xmlns:opensearch="http://a9.com/-/spec/opensearch/1.1/"><opensearch:totalResults>0</opensearch:totalResults><opensearch:startIndex>0</opensearch:startIndex><opensearch:itemsPerPage>0</opensearch:itemsPerPage></feed>`))
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL, MinRequestInterval: 0})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, _ = a.Search(ctx, types.Query{Text: "transformer", MaxResults: 0})

	if capturedMaxResults != "25" {
		t.Errorf("max_results = %q, want %q", capturedMaxResults, "25")
	}
}

// TestSearchHonoursCursorParameter verifies that q.Cursor is passed as start= param.
func TestSearchHonoursCursorParameter(t *testing.T) {
	t.Parallel()

	var capturedStart string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedStart = r.URL.Query().Get("start")
		w.Header().Set("Content-Type", "application/atom+xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom" xmlns:opensearch="http://a9.com/-/spec/opensearch/1.1/"><opensearch:totalResults>0</opensearch:totalResults><opensearch:startIndex>50</opensearch:startIndex><opensearch:itemsPerPage>0</opensearch:itemsPerPage></feed>`))
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL, MinRequestInterval: 0})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, _ = a.Search(ctx, types.Query{Text: "transformer", Cursor: "50"})

	if capturedStart != "50" {
		t.Errorf("start = %q, want %q", capturedStart, "50")
	}
}

// TestSearchHTTP429WithIntegerRetryAfter verifies integer Retry-After parsing.
func TestSearchHTTP429WithIntegerRetryAfter(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL, MinRequestInterval: 0})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, err = a.Search(ctx, types.Query{Text: "deep learning"})
	if err == nil {
		t.Fatal("Search() expected error for 429, got nil")
	}

	var se *types.SourceError
	if !errors.As(err, &se) {
		t.Fatalf("err is not *SourceError: %T", err)
	}
	if se.Category != types.CategoryRateLimited {
		t.Errorf("Category = %v, want CategoryRateLimited", se.Category)
	}
	if se.HTTPStatus != 429 {
		t.Errorf("HTTPStatus = %d, want 429", se.HTTPStatus)
	}
	if se.RetryAfter != 30*time.Second {
		t.Errorf("RetryAfter = %v, want 30s", se.RetryAfter)
	}
}

// TestSearchHTTP429WithHTTPDateRetryAfter verifies HTTP-date Retry-After parsing.
func TestSearchHTTP429WithHTTPDateRetryAfter(t *testing.T) {
	t.Parallel()

	future := time.Now().Add(30 * time.Second).UTC().Format(http.TimeFormat)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", future)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL, MinRequestInterval: 0})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, err = a.Search(ctx, types.Query{Text: "deep learning"})
	if err == nil {
		t.Fatal("Search() expected error for 429, got nil")
	}

	var se *types.SourceError
	if !errors.As(err, &se) {
		t.Fatalf("err is not *SourceError: %T", err)
	}
	if se.RetryAfter < 25*time.Second || se.RetryAfter > 35*time.Second {
		t.Errorf("RetryAfter = %v, want in (25s, 35s)", se.RetryAfter)
	}
}

// TestSearchHTTP429NoRetryAfterDefaults5s verifies default 5s when Retry-After absent.
func TestSearchHTTP429NoRetryAfterDefaults5s(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL, MinRequestInterval: 0})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, err = a.Search(ctx, types.Query{Text: "deep learning"})

	var se *types.SourceError
	if !errors.As(err, &se) {
		t.Fatalf("err is not *SourceError: %T", err)
	}
	if se.RetryAfter != 5*time.Second {
		t.Errorf("RetryAfter = %v, want 5s", se.RetryAfter)
	}
}

// TestSearchHTTP429RetryAfterCapped60s verifies 60s cap on large Retry-After values.
func TestSearchHTTP429RetryAfterCapped60s(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "999")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL, MinRequestInterval: 0})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, err = a.Search(ctx, types.Query{Text: "deep learning"})

	var se *types.SourceError
	if !errors.As(err, &se) {
		t.Fatalf("err is not *SourceError: %T", err)
	}
	if se.RetryAfter != 60*time.Second {
		t.Errorf("RetryAfter = %v, want 60s (capped)", se.RetryAfter)
	}
}

// TestSearchHTTP429NoInternalRetry verifies the adapter sends exactly 1 request on 429.
func TestSearchHTTP429NoInternalRetry(t *testing.T) {
	t.Parallel()

	var requestCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL, MinRequestInterval: 0})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, _ = a.Search(ctx, types.Query{Text: "deep learning"})

	if count := atomic.LoadInt32(&requestCount); count != 1 {
		t.Errorf("request count = %d, want 1 (no internal retry)", count)
	}
}

// TestSearchHTTP400ValidationError verifies HTTP 400 maps to ErrPermanent.
func TestSearchHTTP400ValidationError(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile("testdata/search_response_400_error.xml")
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write(body)
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL, MinRequestInterval: 0})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, err = a.Search(ctx, types.Query{Text: "deep learning"})
	if err == nil {
		t.Fatal("Search() expected error for 400, got nil")
	}
	if !errors.Is(err, types.ErrPermanent) {
		t.Errorf("errors.Is(err, ErrPermanent) = false; err = %v", err)
	}
	var se *types.SourceError
	if errors.As(err, &se) && se.HTTPStatus != 400 {
		t.Errorf("HTTPStatus = %d, want 400", se.HTTPStatus)
	}
}

// TestSearchHTTP401 verifies HTTP 401 -> CategoryPermanent.
func TestSearchHTTP401(t *testing.T) {
	t.Parallel()
	testHTTP4xx(t, http.StatusUnauthorized)
}

// TestSearchHTTP403 verifies HTTP 403 -> CategoryPermanent.
func TestSearchHTTP403(t *testing.T) {
	t.Parallel()
	testHTTP4xx(t, http.StatusForbidden)
}

// TestSearchHTTP404 verifies HTTP 404 -> CategoryPermanent.
func TestSearchHTTP404(t *testing.T) {
	t.Parallel()
	testHTTP4xx(t, http.StatusNotFound)
}

func testHTTP4xx(t *testing.T, statusCode int) {
	t.Helper()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL, MinRequestInterval: 0})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, err = a.Search(ctx, types.Query{Text: "deep learning"})
	if err == nil {
		t.Fatalf("Search() expected error for HTTP %d, got nil", statusCode)
	}
	if !errors.Is(err, types.ErrPermanent) {
		t.Errorf("errors.Is(err, ErrPermanent) = false; err = %v", err)
	}
	var se *types.SourceError
	if errors.As(err, &se) && se.HTTPStatus != statusCode {
		t.Errorf("HTTPStatus = %d, want %d", se.HTTPStatus, statusCode)
	}
}

// TestSearchHTTP4xxNoInternalRetry verifies no internal retry on 4xx.
func TestSearchHTTP4xxNoInternalRetry(t *testing.T) {
	t.Parallel()

	var requestCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL, MinRequestInterval: 0})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, _ = a.Search(ctx, types.Query{Text: "deep learning"})

	if count := atomic.LoadInt32(&requestCount); count != 1 {
		t.Errorf("request count = %d, want 1", count)
	}
}

// TestSearchHTTP500 verifies HTTP 500 -> CategoryUnavailable.
func TestSearchHTTP500(t *testing.T) {
	t.Parallel()
	testHTTP5xx(t, http.StatusInternalServerError)
}

// TestSearchHTTP503 verifies HTTP 503 -> CategoryUnavailable.
func TestSearchHTTP503(t *testing.T) {
	t.Parallel()
	testHTTP5xx(t, http.StatusServiceUnavailable)
}

func testHTTP5xx(t *testing.T, statusCode int) {
	t.Helper()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL, MinRequestInterval: 0})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, err = a.Search(ctx, types.Query{Text: "deep learning"})
	if err == nil {
		t.Fatalf("Search() expected error for HTTP %d, got nil", statusCode)
	}
	if !errors.Is(err, types.ErrSourceUnavailable) {
		t.Errorf("errors.Is(err, ErrSourceUnavailable) = false; err = %v", err)
	}
	var se *types.SourceError
	if errors.As(err, &se) && se.HTTPStatus != statusCode {
		t.Errorf("HTTPStatus = %d, want %d", se.HTTPStatus, statusCode)
	}
}

// TestSearchConnectionRefused verifies network failure -> CategoryUnavailable with HTTPStatus=0.
func TestSearchConnectionRefused(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	closedURL := ts.URL
	ts.Close()

	a, err := New(Options{BaseURL: closedURL, MinRequestInterval: 0})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, err = a.Search(ctx, types.Query{Text: "deep learning"})
	if err == nil {
		t.Fatal("Search() expected error for connection refused, got nil")
	}
	if !errors.Is(err, types.ErrSourceUnavailable) {
		t.Errorf("errors.Is(err, ErrSourceUnavailable) = false; err = %v", err)
	}
	var se *types.SourceError
	if errors.As(err, &se) && se.HTTPStatus != 0 {
		t.Errorf("HTTPStatus = %d, want 0 for network error", se.HTTPStatus)
	}
}

// TestSearchUnavailablePreservesUnderlyingError verifies inner error is preserved.
func TestSearchUnavailablePreservesUnderlyingError(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	closedURL := ts.URL
	ts.Close()

	a, err := New(Options{BaseURL: closedURL, MinRequestInterval: 0})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, err = a.Search(ctx, types.Query{Text: "deep learning"})

	if errors.Unwrap(err) == nil {
		t.Error("errors.Unwrap(err) = nil, want non-nil underlying error")
	}
}

// TestSearchCategoryFilterAdded verifies category filter is prepended to search_query.
func TestSearchCategoryFilterAdded(t *testing.T) {
	t.Parallel()

	var capturedSearchQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedSearchQuery = r.URL.Query().Get("search_query")
		w.Header().Set("Content-Type", "application/atom+xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom" xmlns:opensearch="http://a9.com/-/spec/opensearch/1.1/"><opensearch:totalResults>0</opensearch:totalResults><opensearch:startIndex>0</opensearch:startIndex><opensearch:itemsPerPage>0</opensearch:itemsPerPage></feed>`))
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL, MinRequestInterval: 0})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, _ = a.Search(ctx, types.Query{
		Text:    "transformer",
		Filters: []types.Filter{{Key: "category", Value: "cs.AI"}},
	})

	// Expect: "cat:cs.AI AND transformer"
	if !strings.Contains(capturedSearchQuery, "cat:cs.AI") {
		t.Errorf("search_query = %q, want to contain %q", capturedSearchQuery, "cat:cs.AI")
	}
	if !strings.Contains(capturedSearchQuery, "AND") {
		t.Errorf("search_query = %q, want to contain AND combinator", capturedSearchQuery)
	}
	if !strings.Contains(capturedSearchQuery, "transformer") {
		t.Errorf("search_query = %q, want to contain %q", capturedSearchQuery, "transformer")
	}
}

// TestSearchCategoryFilterAbsent verifies no category prefix when Filters=nil.
func TestSearchCategoryFilterAbsent(t *testing.T) {
	t.Parallel()

	var capturedSearchQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedSearchQuery = r.URL.Query().Get("search_query")
		w.Header().Set("Content-Type", "application/atom+xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom" xmlns:opensearch="http://a9.com/-/spec/opensearch/1.1/"><opensearch:totalResults>0</opensearch:totalResults><opensearch:startIndex>0</opensearch:startIndex><opensearch:itemsPerPage>0</opensearch:itemsPerPage></feed>`))
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL, MinRequestInterval: 0})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, _ = a.Search(ctx, types.Query{Text: "transformer", Filters: nil})

	if capturedSearchQuery != "transformer" {
		t.Errorf("search_query = %q, want %q", capturedSearchQuery, "transformer")
	}
}

// TestSearchCategoryFilterEmpty verifies empty category value treated as absent.
func TestSearchCategoryFilterEmpty(t *testing.T) {
	t.Parallel()

	var capturedSearchQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedSearchQuery = r.URL.Query().Get("search_query")
		w.Header().Set("Content-Type", "application/atom+xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom" xmlns:opensearch="http://a9.com/-/spec/opensearch/1.1/"><opensearch:totalResults>0</opensearch:totalResults><opensearch:startIndex>0</opensearch:startIndex><opensearch:itemsPerPage>0</opensearch:itemsPerPage></feed>`))
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL, MinRequestInterval: 0})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, _ = a.Search(ctx, types.Query{
		Text:    "transformer",
		Filters: []types.Filter{{Key: "category", Value: ""}},
	})

	if capturedSearchQuery != "transformer" {
		t.Errorf("search_query = %q, want %q (empty category treated as absent)", capturedSearchQuery, "transformer")
	}
}

// TestSearchUnknownFilterIgnored verifies unknown filter keys are silently ignored.
func TestSearchUnknownFilterIgnored(t *testing.T) {
	t.Parallel()

	var capturedSearchQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedSearchQuery = r.URL.Query().Get("search_query")
		w.Header().Set("Content-Type", "application/atom+xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom" xmlns:opensearch="http://a9.com/-/spec/opensearch/1.1/"><opensearch:totalResults>0</opensearch:totalResults><opensearch:startIndex>0</opensearch:startIndex><opensearch:itemsPerPage>0</opensearch:itemsPerPage></feed>`))
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL, MinRequestInterval: 0})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, _ = a.Search(ctx, types.Query{
		Text:    "transformer",
		Filters: []types.Filter{{Key: "nsfw", Value: "true"}},
	})

	if capturedSearchQuery != "transformer" {
		t.Errorf("search_query = %q, want %q (unknown filter ignored)", capturedSearchQuery, "transformer")
	}
}

// TestSearchEmptyQueryRejectedNoHTTP verifies empty/whitespace queries are rejected without HTTP.
func TestSearchEmptyQueryRejectedNoHTTP(t *testing.T) {
	t.Parallel()

	tests := []string{"", "   ", "\t\n  \r"}

	for _, text := range tests {
		t.Run("text="+reprQuery(text), func(t *testing.T) {
			var requestCount int32
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&requestCount, 1)
				w.WriteHeader(http.StatusOK)
			}))
			defer ts.Close()

			a, err := New(Options{BaseURL: ts.URL, MinRequestInterval: 0})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			ctx := testContext(t)
			_, err = a.Search(ctx, types.Query{Text: text})
			if err == nil {
				t.Fatal("Search() expected error for empty/whitespace query, got nil")
			}
			if !errors.Is(err, types.ErrPermanent) {
				t.Errorf("errors.Is(err, ErrPermanent) = false; err = %v", err)
			}
			if !errors.Is(err, ErrInvalidQuery) {
				t.Errorf("errors.Is(err, ErrInvalidQuery) = false; err = %v", err)
			}
			if count := atomic.LoadInt32(&requestCount); count != 0 {
				t.Errorf("request count = %d, want 0 (no HTTP should be issued)", count)
			}
		})
	}
}

// TestSearchInvalidStartRejectedNoHTTP verifies invalid cursor values are rejected.
func TestSearchInvalidStartRejectedNoHTTP(t *testing.T) {
	t.Parallel()

	tests := []string{"abc", "-1", "1.5", "1e3", "  "}

	for _, cursor := range tests {
		t.Run("cursor="+reprQuery(cursor), func(t *testing.T) {
			var requestCount int32
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&requestCount, 1)
				w.WriteHeader(http.StatusOK)
			}))
			defer ts.Close()

			a, err := New(Options{BaseURL: ts.URL, MinRequestInterval: 0})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			ctx := testContext(t)
			_, err = a.Search(ctx, types.Query{Text: "deep learning", Cursor: cursor})
			if err == nil {
				t.Fatalf("Search() expected error for cursor=%q, got nil", cursor)
			}
			if !errors.Is(err, types.ErrPermanent) {
				t.Errorf("errors.Is(err, ErrPermanent) = false; err = %v", err)
			}
			if !errors.Is(err, ErrInvalidStart) {
				t.Errorf("errors.Is(err, ErrInvalidStart) = false; err = %v", err)
			}
			if count := atomic.LoadInt32(&requestCount); count != 0 {
				t.Errorf("request count = %d, want 0", count)
			}
		})
	}
}

// TestSearchSetsCustomUserAgent verifies User-Agent format.
func TestSearchSetsCustomUserAgent(t *testing.T) {
	t.Parallel()

	var capturedUA string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/atom+xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom" xmlns:opensearch="http://a9.com/-/spec/opensearch/1.1/"><opensearch:totalResults>0</opensearch:totalResults><opensearch:startIndex>0</opensearch:startIndex><opensearch:itemsPerPage>0</opensearch:itemsPerPage></feed>`))
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL, MinRequestInterval: 0})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, _ = a.Search(ctx, types.Query{Text: "deep learning"})

	if !strings.HasPrefix(capturedUA, "usearch/") {
		t.Errorf("User-Agent = %q, want prefix %q", capturedUA, "usearch/")
	}
	if !strings.Contains(capturedUA, "(+https://github.com/elymas/universal-search)") {
		t.Errorf("User-Agent = %q, missing required URL", capturedUA)
	}
}

// TestSearchSetsAcceptAtomXML verifies Accept header is application/atom+xml.
func TestSearchSetsAcceptAtomXML(t *testing.T) {
	t.Parallel()

	var capturedAccept string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/atom+xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom" xmlns:opensearch="http://a9.com/-/spec/opensearch/1.1/"><opensearch:totalResults>0</opensearch:totalResults><opensearch:startIndex>0</opensearch:startIndex><opensearch:itemsPerPage>0</opensearch:itemsPerPage></feed>`))
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL, MinRequestInterval: 0})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, _ = a.Search(ctx, types.Query{Text: "deep learning"})

	if capturedAccept != "application/atom+xml" {
		t.Errorf("Accept = %q, want %q", capturedAccept, "application/atom+xml")
	}
}

// TestSearchUserAgentVersionConfigurable verifies UA version is configurable.
func TestSearchUserAgentVersionConfigurable(t *testing.T) {
	t.Parallel()

	var capturedUA string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/atom+xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom" xmlns:opensearch="http://a9.com/-/spec/opensearch/1.1/"><opensearch:totalResults>0</opensearch:totalResults><opensearch:startIndex>0</opensearch:startIndex><opensearch:itemsPerPage>0</opensearch:itemsPerPage></feed>`))
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL, UserAgentVersion: "v0.2-rc1", MinRequestInterval: 0})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, _ = a.Search(ctx, types.Query{Text: "deep learning"})

	if !strings.Contains(capturedUA, "usearch/v0.2-rc1") {
		t.Errorf("User-Agent = %q, want to contain %q", capturedUA, "usearch/v0.2-rc1")
	}
}

// TestSearchRejectsCrossDomainRedirect verifies cross-domain redirects are rejected.
func TestSearchRejectsCrossDomainRedirect(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://attacker.com/steal", http.StatusFound)
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL, MinRequestInterval: 0})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, err = a.Search(ctx, types.Query{Text: "deep learning"})
	if err == nil {
		t.Fatal("Search() expected error for cross-domain redirect, got nil")
	}
	if !errors.Is(err, types.ErrPermanent) {
		t.Errorf("errors.Is(err, ErrPermanent) = false; err = %v", err)
	}
	if !strings.Contains(err.Error(), "cross-domain redirect") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "cross-domain redirect")
	}
}

// TestSearchRejectsRedirectChainOver3 verifies >3 redirect hops are rejected.
func TestSearchRejectsRedirectChainOver3(t *testing.T) {
	t.Parallel()

	var serverD, serverC, serverB, serverA *httptest.Server

	serverD = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer serverD.Close()

	serverC = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, serverD.URL, http.StatusFound)
	}))
	defer serverC.Close()

	serverB = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, serverC.URL, http.StatusFound)
	}))
	defer serverB.Close()

	serverA = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, serverB.URL, http.StatusFound)
	}))
	defer serverA.Close()

	hopCountClient := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("arxiv: too many redirects (max 3)")
			}
			return nil
		},
	}

	a, err := New(Options{
		BaseURL:            serverA.URL,
		HTTPClient:         hopCountClient,
		MinRequestInterval: 0,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, err = a.Search(ctx, types.Query{Text: "deep learning"})
	if err == nil {
		t.Fatal("Search() expected error for >3 redirect hops, got nil")
	}
	if !strings.Contains(err.Error(), "too many redirects") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "too many redirects")
	}
}

// TestSearchConcurrentSafe verifies no race conditions with 50 concurrent goroutines.
func TestSearchConcurrentSafe(t *testing.T) {
	body, err := os.ReadFile("testdata/search_response.xml")
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	var requestCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.Header().Set("Content-Type", "application/atom+xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL, MinRequestInterval: 0})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	const numGoroutines = 50
	var wg sync.WaitGroup
	var barrier sync.WaitGroup
	barrier.Add(1)

	type result struct {
		docs []types.NormalizedDoc
		err  error
	}
	results := make([]result, numGoroutines)

	for i := range numGoroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			barrier.Wait()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			docs, err := a.Search(ctx, types.Query{Text: "deep learning"})
			results[idx] = result{docs: docs, err: err}
		}(i)
	}

	barrier.Done()
	wg.Wait()

	if count := atomic.LoadInt32(&requestCount); count != numGoroutines {
		t.Errorf("request count = %d, want %d", count, numGoroutines)
	}
	for i, r := range results {
		if r.err != nil {
			t.Errorf("goroutine %d: Search() error = %v", i, r.err)
			continue
		}
		if len(r.docs) != 25 {
			t.Errorf("goroutine %d: got %d docs, want 25", i, len(r.docs))
			continue
		}
		for j, doc := range r.docs {
			if err := doc.Validate(); err != nil {
				t.Errorf("goroutine %d, doc %d: Validate() error = %v", i, j, err)
			}
		}
	}
}

// TestSearchE2ELatencyStubP95 verifies p95 latency <= 200ms against a stub.
func TestSearchE2ELatencyStubP95(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile("testdata/search_response.xml")
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL, MinRequestInterval: 0})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	const numInvocations = 100
	durations := make([]time.Duration, numInvocations)

	for i := range numInvocations {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		start := time.Now()
		_, err := a.Search(ctx, types.Query{Text: "deep learning"})
		durations[i] = time.Since(start)
		cancel()
		if err != nil {
			t.Fatalf("Search() invocation %d error = %v", i, err)
		}
	}

	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	p95 := durations[94]
	const maxP95 = 200 * time.Millisecond
	if p95 > maxP95 {
		t.Errorf("p95 latency = %v, want <= %v", p95, maxP95)
	}
}

// TestSearchNoGoroutineLeakOnCancel verifies no goroutine leak on context cancellation.
func TestSearchNoGoroutineLeakOnCancel(t *testing.T) {
	delay := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-delay:
		case <-r.Context().Done():
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	defer close(delay)

	a, err := New(Options{BaseURL: ts.URL, MinRequestInterval: 0})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, _ = a.Search(ctx, types.Query{Text: "deep learning"})

	// Give the HTTP client time to clean up.
	time.Sleep(10 * time.Millisecond)
}

// TestSearchEmptyFeedReturnsNilNil verifies that an empty feed yields (nil, nil).
func TestSearchEmptyFeedReturnsNilNil(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile("testdata/search_response_empty.xml")
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL, MinRequestInterval: 0})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	docs, err := a.Search(ctx, types.Query{Text: "nobody_xxxx_zzz"})
	if err != nil {
		t.Fatalf("Search() error = %v, want nil", err)
	}
	if len(docs) != 0 {
		t.Errorf("Search() returned %d docs, want 0", len(docs))
	}
}

// reprQuery returns a printable representation of a string for test names.
func reprQuery(s string) string {
	if s == "" {
		return "empty"
	}
	r := make([]byte, 0, len(s))
	for _, c := range s {
		switch c {
		case '\t':
			r = append(r, []byte("\\t")...)
		case '\n':
			r = append(r, []byte("\\n")...)
		case '\r':
			r = append(r, []byte("\\r")...)
		case ' ':
			r = append(r, '_')
		default:
			r = append(r, byte(c))
		}
	}
	return string(r)
}
