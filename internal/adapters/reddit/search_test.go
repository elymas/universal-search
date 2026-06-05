package reddit

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// testContext returns a context with a reasonable deadline for testing.
func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// happyPathServer returns an httptest.Server that serves the 25-doc fixture.
func happyPathServer(t *testing.T) *httptest.Server {
	t.Helper()
	body, err := os.ReadFile("testdata/search_response.json")
	if err != nil {
		t.Fatalf("os.ReadFile(search_response.json) error = %v", err)
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	t.Cleanup(ts.Close)
	return ts
}

// TestSearchHappyPath25Docs verifies that Search returns 25 valid NormalizedDocs.
func TestSearchHappyPath25Docs(t *testing.T) {
	t.Parallel()

	ts := happyPathServer(t)
	a, err := New(Options{BaseURL: ts.URL, SkipAuthCheck: true})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	docs, err := a.Search(ctx, types.Query{Text: "golang"})
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

// TestSearchURLParametersIncludeAllRequired verifies all 7 required URL params.
func TestSearchURLParametersIncludeAllRequired(t *testing.T) {
	t.Parallel()

	var capturedURL *url.URL
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"after":null,"children":[]}}`))
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL, SkipAuthCheck: true})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, _ = a.Search(ctx, types.Query{Text: "golang test"})

	if capturedURL == nil {
		t.Fatal("no request captured")
	}
	params := capturedURL.Query()

	requiredParams := []string{"q", "sort", "t", "type", "limit", "include_over_18"}
	for _, p := range requiredParams {
		if params.Get(p) == "" {
			t.Errorf("missing required URL parameter %q in URL %q", p, capturedURL.RawQuery)
		}
	}
	if params.Get("sort") != "relevance" {
		t.Errorf("sort = %q, want %q", params.Get("sort"), "relevance")
	}
	if params.Get("t") != "all" {
		t.Errorf("t = %q, want %q", params.Get("t"), "all")
	}
	if params.Get("type") != "link" {
		t.Errorf("type = %q, want %q", params.Get("type"), "link")
	}
}

// TestSearchClampsLimitTo100 verifies that MaxResults > 100 is clamped.
func TestSearchClampsLimitTo100(t *testing.T) {
	t.Parallel()

	var capturedLimit string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedLimit = r.URL.Query().Get("limit")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"after":null,"children":[]}}`))
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL, SkipAuthCheck: true})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, _ = a.Search(ctx, types.Query{Text: "golang", MaxResults: 500})

	if capturedLimit != "100" {
		t.Errorf("limit = %q, want %q", capturedLimit, "100")
	}
}

// TestSearchDefaultsLimitTo25 verifies that MaxResults=0 defaults to 25.
func TestSearchDefaultsLimitTo25(t *testing.T) {
	t.Parallel()

	var capturedLimit string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedLimit = r.URL.Query().Get("limit")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"after":null,"children":[]}}`))
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL, SkipAuthCheck: true})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, _ = a.Search(ctx, types.Query{Text: "golang", MaxResults: 0})

	if capturedLimit != "25" {
		t.Errorf("limit = %q, want %q", capturedLimit, "25")
	}
}

// TestSearchHonoursCursorParameter verifies that q.Cursor is passed as after= param.
func TestSearchHonoursCursorParameter(t *testing.T) {
	t.Parallel()

	var capturedAfter string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAfter = r.URL.Query().Get("after")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"after":null,"children":[]}}`))
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL, SkipAuthCheck: true})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, _ = a.Search(ctx, types.Query{Text: "golang", Cursor: "t3_abc"})

	if capturedAfter != "t3_abc" {
		t.Errorf("after = %q, want %q", capturedAfter, "t3_abc")
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

	a, err := New(Options{BaseURL: ts.URL, SkipAuthCheck: true})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, err = a.Search(ctx, types.Query{Text: "golang"})
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

	// Set Retry-After to 30 seconds in the future from now.
	future := time.Now().Add(30 * time.Second).UTC().Format(http.TimeFormat)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", future)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL, SkipAuthCheck: true})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, err = a.Search(ctx, types.Query{Text: "golang"})
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

// TestSearchHTTP429NoRetryAfterDefaults5s verifies default 5s when header absent.
func TestSearchHTTP429NoRetryAfterDefaults5s(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL, SkipAuthCheck: true})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, err = a.Search(ctx, types.Query{Text: "golang"})

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

	a, err := New(Options{BaseURL: ts.URL, SkipAuthCheck: true})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, err = a.Search(ctx, types.Query{Text: "golang"})

	var se *types.SourceError
	if !errors.As(err, &se) {
		t.Fatalf("err is not *SourceError: %T", err)
	}
	if se.RetryAfter != 60*time.Second {
		t.Errorf("RetryAfter = %v, want 60s (capped)", se.RetryAfter)
	}
}

// TestSearchHTTP429NoInternalRetry verifies that the adapter sends only 1 request.
func TestSearchHTTP429NoInternalRetry(t *testing.T) {
	t.Parallel()

	var requestCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL, SkipAuthCheck: true})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, _ = a.Search(ctx, types.Query{Text: "golang"})

	if count := atomic.LoadInt32(&requestCount); count != 1 {
		t.Errorf("request count = %d, want 1 (no internal retry)", count)
	}
}

// TestSearchHTTP401 verifies HTTP 401 triggers refresh+retry, exhausting to
// CategoryUnavailable (ADP-001a: 401 is recoverable, not Permanent).
func TestSearchHTTP401(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL, SkipAuthCheck: true})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, err = a.Search(ctx, types.Query{Text: "golang"})
	if err == nil {
		t.Fatalf("Search() expected error for HTTP 401, got nil")
	}
	// ADP-001a: 401 now exhausts to CategoryUnavailable, not Permanent.
	if !errors.Is(err, types.ErrSourceUnavailable) {
		t.Errorf("errors.Is(err, ErrSourceUnavailable) = false, want true; err = %v", err)
	}
	if !errors.Is(err, ErrTokenRefreshExhausted) {
		t.Errorf("errors.Is(err, ErrTokenRefreshExhausted) = false, want true; err = %v", err)
	}
	var se *types.SourceError
	if errors.As(err, &se) && se.HTTPStatus != 401 {
		t.Errorf("HTTPStatus = %d, want 401", se.HTTPStatus)
	}
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

	a, err := New(Options{BaseURL: ts.URL, SkipAuthCheck: true})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, err = a.Search(ctx, types.Query{Text: "golang"})
	if err == nil {
		t.Fatalf("Search() expected error for HTTP %d, got nil", statusCode)
	}
	if !errors.Is(err, types.ErrPermanent) {
		t.Errorf("errors.Is(err, ErrPermanent) = false, want true; err = %v", err)
	}
	var se *types.SourceError
	if errors.As(err, &se) && se.HTTPStatus != statusCode {
		t.Errorf("HTTPStatus = %d, want %d", se.HTTPStatus, statusCode)
	}
}

// TestSearchHTTP4xxNoInternalRetry verifies that 4xx responses don't trigger retry.
func TestSearchHTTP4xxNoInternalRetry(t *testing.T) {
	t.Parallel()

	var requestCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL, SkipAuthCheck: true})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, _ = a.Search(ctx, types.Query{Text: "golang"})

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

	a, err := New(Options{BaseURL: ts.URL, SkipAuthCheck: true})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, err = a.Search(ctx, types.Query{Text: "golang"})
	if err == nil {
		t.Fatalf("Search() expected error for HTTP %d, got nil", statusCode)
	}
	if !errors.Is(err, types.ErrSourceUnavailable) {
		t.Errorf("errors.Is(err, ErrSourceUnavailable) = false, want true; err = %v", err)
	}
	var se *types.SourceError
	if errors.As(err, &se) && se.HTTPStatus != statusCode {
		t.Errorf("HTTPStatus = %d, want %d", se.HTTPStatus, statusCode)
	}
}

// TestSearchConnectionRefused verifies network failure -> CategoryUnavailable with HTTPStatus=0.
func TestSearchConnectionRefused(t *testing.T) {
	t.Parallel()

	// Start and immediately close a server to get a valid-but-closed address.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	closedURL := ts.URL
	ts.Close()

	a, err := New(Options{BaseURL: closedURL, SkipAuthCheck: true})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, err = a.Search(ctx, types.Query{Text: "golang"})
	if err == nil {
		t.Fatal("Search() expected error for connection refused, got nil")
	}
	if !errors.Is(err, types.ErrSourceUnavailable) {
		t.Errorf("errors.Is(err, ErrSourceUnavailable) = false, want true; err = %v", err)
	}
	var se *types.SourceError
	if errors.As(err, &se) && se.HTTPStatus != 0 {
		t.Errorf("HTTPStatus = %d, want 0 for network error", se.HTTPStatus)
	}
}

// TestSearchUnavailablePreservesUnderlyingError verifies that the inner error is preserved.
func TestSearchUnavailablePreservesUnderlyingError(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	closedURL := ts.URL
	ts.Close()

	a, err := New(Options{BaseURL: closedURL, SkipAuthCheck: true})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, err = a.Search(ctx, types.Query{Text: "golang"})

	if errors.Unwrap(err) == nil {
		t.Error("errors.Unwrap(err) = nil, want non-nil underlying error")
	}
}

// TestSearchNSFWFilterTrueIncludesOver18 verifies nsfw=true sends include_over_18=true.
func TestSearchNSFWFilterTrueIncludesOver18(t *testing.T) {
	t.Parallel()
	testNSFWFilter(t, []types.Filter{{Key: "nsfw", Value: "true"}}, "true")
}

// TestSearchNSFWFilterFalseExcludes verifies nsfw=false sends include_over_18=false.
func TestSearchNSFWFilterFalseExcludes(t *testing.T) {
	t.Parallel()
	testNSFWFilter(t, []types.Filter{{Key: "nsfw", Value: "false"}}, "false")
}

// TestSearchNSFWFilterAbsentDefaultsExclude verifies absent filter defaults to false.
func TestSearchNSFWFilterAbsentDefaultsExclude(t *testing.T) {
	t.Parallel()
	testNSFWFilter(t, nil, "false")
}

// TestSearchNSFWFilterUnknownValueDefaultsExclude verifies unknown value defaults to false.
func TestSearchNSFWFilterUnknownValueDefaultsExclude(t *testing.T) {
	t.Parallel()
	testNSFWFilter(t, []types.Filter{{Key: "nsfw", Value: "maybe"}}, "false")
}

func testNSFWFilter(t *testing.T, filters []types.Filter, wantIncludeOver18 string) {
	t.Helper()

	var capturedValue string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedValue = r.URL.Query().Get("include_over_18")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"after":null,"children":[]}}`))
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL, SkipAuthCheck: true})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := testContext(t)
	_, _ = a.Search(ctx, types.Query{Text: "golang", Filters: filters})

	if capturedValue != wantIncludeOver18 {
		t.Errorf("include_over_18 = %q, want %q", capturedValue, wantIncludeOver18)
	}
}

// TestSearchEmptyQueryRejectedNoHTTP verifies that empty/whitespace queries are
// rejected without issuing any HTTP request.
func TestSearchEmptyQueryRejectedNoHTTP(t *testing.T) {
	t.Parallel()

	tests := []string{"", "   ", "\t\n  \r"}

	for _, text := range tests {
		t.Run("text="+repr(text), func(t *testing.T) {
			var requestCount int32
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&requestCount, 1)
				w.WriteHeader(http.StatusOK)
			}))
			defer ts.Close()

			a, err := New(Options{BaseURL: ts.URL, SkipAuthCheck: true})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			ctx := testContext(t)
			_, err = a.Search(ctx, types.Query{Text: text})
			if err == nil {
				t.Fatal("Search() expected error for empty/whitespace query, got nil")
			}
			if !errors.Is(err, types.ErrPermanent) {
				t.Errorf("errors.Is(err, ErrPermanent) = false, want true; err = %v", err)
			}
			if !errors.Is(err, ErrInvalidQuery) {
				t.Errorf("errors.Is(err, ErrInvalidQuery) = false, want true; err = %v", err)
			}
			if count := atomic.LoadInt32(&requestCount); count != 0 {
				t.Errorf("request count = %d, want 0 (no HTTP should be issued)", count)
			}
		})
	}
}

// TestSearchConcurrentSafe verifies no race conditions with 50 concurrent goroutines.
func TestSearchConcurrentSafe(t *testing.T) {
	// Note: do NOT call t.Parallel() here — the race detector runs on the whole test
	// binary and this test uses goroutines internally.

	body, err := os.ReadFile("testdata/search_response.json")
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	var requestCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL, SkipAuthCheck: true})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	const numGoroutines = 50
	var wg sync.WaitGroup
	var barrier sync.WaitGroup
	barrier.Add(1) // All goroutines wait on this.

	type result struct {
		docs []types.NormalizedDoc
		err  error
	}
	results := make([]result, numGoroutines)

	for i := range numGoroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			barrier.Wait() // Wait for the start signal.
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			docs, err := a.Search(ctx, types.Query{Text: "golang"})
			results[idx] = result{docs: docs, err: err}
		}(i)
	}

	barrier.Done() // Release all goroutines simultaneously.
	wg.Wait()

	// Assertions.
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

// TestSearchE2ELatencyStubP95 verifies that p95 latency is <= 200ms against a stub.
func TestSearchE2ELatencyStubP95(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile("testdata/search_response.json")
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL, SkipAuthCheck: true})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	const numInvocations = 100
	durations := make([]time.Duration, numInvocations)

	for i := range numInvocations {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		start := time.Now()
		_, err := a.Search(ctx, types.Query{Text: "golang"})
		durations[i] = time.Since(start)
		cancel()
		if err != nil {
			t.Fatalf("Search() invocation %d error = %v", i, err)
		}
	}

	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	p95 := durations[94] // index 94 = 95th percentile (0-indexed)
	const maxP95 = 200 * time.Millisecond
	if p95 > maxP95 {
		t.Errorf("p95 latency = %v, want <= %v", p95, maxP95)
	}
}

// TestSearchNoGoroutineLeakOnCancel verifies no goroutine leak on context cancellation.
// Note: goleak is added in bench_test.go to keep test files organized.
func TestSearchNoGoroutineLeakOnCancel(t *testing.T) {
	// goleak is imported in bench_test.go
	// This test verifies the behavior: stub delays 200ms, ctx cancelled at 50ms.

	// We use a channel to delay the response.
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

	a, err := New(Options{BaseURL: ts.URL, SkipAuthCheck: true})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, _ = a.Search(ctx, types.Query{Text: "golang"})

	// Give the HTTP client time to clean up.
	time.Sleep(10 * time.Millisecond)

	// goleak.VerifyNone is called from the TestMain in bench_test.go
	// or via explicit goleak import. Here we just verify the adapter
	// doesn't hang after cancellation.
}

// repr returns a printable representation of a string for test names.
func repr(s string) string {
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
			r = append(r, []byte("_")...)
		default:
			r = append(r, byte(c))
		}
	}
	return string(r)
}
