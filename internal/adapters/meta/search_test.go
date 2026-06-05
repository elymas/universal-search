package meta

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
	"go.uber.org/goleak"
)

// ---------------------------------------------------------------------------
// REQ-ADP10-002: Threads search happy path
// ---------------------------------------------------------------------------

// TestSearchThreadsHappyPath25Posts verifies the full happy path with 25 docs.
func TestSearchThreadsHappyPath25Posts(t *testing.T) {
	body := loadFixture(t, "threads_keyword_search_response.json")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer ts.Close()

	a, _ := NewThreads(ThreadsOptions{
		AccessToken: "t",
		BaseURL:     ts.URL,
		HTTPClient:  ts.Client(),
	})
	docs, err := a.Search(t.Context(), types.Query{Text: "threads"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(docs) != 25 {
		t.Fatalf("got %d docs, want 25", len(docs))
	}
	for _, d := range docs {
		if verr := d.Validate(); verr != nil {
			t.Errorf("doc %s Validate: %v", d.ID, verr)
		}
	}
}

// TestSearchThreadsURLParametersRequired verifies required params are always present.
func TestSearchThreadsURLParametersRequired(t *testing.T) {
	var capturedURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer ts.Close()

	a, _ := NewThreads(ThreadsOptions{
		AccessToken: "t",
		BaseURL:     ts.URL,
		HTTPClient:  ts.Client(),
	})
	_, _ = a.Search(t.Context(), types.Query{Text: "test query"})

	if !strings.Contains(capturedURL, "q=") {
		t.Error("URL missing 'q' parameter")
	}
	if !strings.Contains(capturedURL, "limit=") {
		t.Error("URL missing 'limit' parameter")
	}
	if !strings.Contains(capturedURL, "search_type=TOP") {
		t.Error("URL missing 'search_type=TOP'")
	}
	if !strings.Contains(capturedURL, "search_mode=KEYWORD") {
		t.Error("URL missing 'search_mode=KEYWORD'")
	}
}

// TestSearchThreadsClampsLimitTo100 verifies MaxResults > 100 is clamped.
func TestSearchThreadsClampsLimitTo100(t *testing.T) {
	var capturedURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer ts.Close()

	a, _ := NewThreads(ThreadsOptions{
		AccessToken: "t",
		BaseURL:     ts.URL,
		HTTPClient:  ts.Client(),
	})
	_, _ = a.Search(t.Context(), types.Query{Text: "test", MaxResults: 500})

	if !strings.Contains(capturedURL, "limit=100") {
		t.Errorf("URL = %q, want limit=100", capturedURL)
	}
}

// TestSearchThreadsClampsLimitToMin1 verifies MaxResults < 1 is clamped to 1.
func TestSearchThreadsClampsLimitToMin1(t *testing.T) {
	var capturedURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer ts.Close()

	a, _ := NewThreads(ThreadsOptions{
		AccessToken: "t",
		BaseURL:     ts.URL,
		HTTPClient:  ts.Client(),
	})
	_, _ = a.Search(t.Context(), types.Query{Text: "test", MaxResults: -5})

	if !strings.Contains(capturedURL, "limit=1") {
		t.Errorf("URL = %q, want limit=1 (clamped from -5)", capturedURL)
	}
}

// TestSearchThreadsDefaultsLimitTo25 verifies MaxResults=0 defaults to 25.
func TestSearchThreadsDefaultsLimitTo25(t *testing.T) {
	var capturedURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer ts.Close()

	a, _ := NewThreads(ThreadsOptions{
		AccessToken: "t",
		BaseURL:     ts.URL,
		HTTPClient:  ts.Client(),
	})
	_, _ = a.Search(t.Context(), types.Query{Text: "test", MaxResults: 0})

	if !strings.Contains(capturedURL, "limit=25") {
		t.Errorf("URL = %q, want limit=25", capturedURL)
	}
}

// ---------------------------------------------------------------------------
// REQ-ADP10-003: HTTP error mapping
// ---------------------------------------------------------------------------

func testHTTPStatusMapping(t *testing.T, statusCode int, wantCategory types.Category) {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(statusCode)
	}))
	defer ts.Close()

	a, _ := NewThreads(ThreadsOptions{
		AccessToken: "t",
		BaseURL:     ts.URL,
		HTTPClient:  ts.Client(),
	})
	_, err := a.Search(t.Context(), types.Query{Text: "test"})
	if err == nil {
		t.Fatalf("expected error for HTTP %d, got nil", statusCode)
	}
	var se *types.SourceError
	if !errors.As(err, &se) {
		t.Fatalf("expected *types.SourceError, got %T: %v", err, err)
	}
	if se.Category != wantCategory {
		t.Errorf("HTTP %d: Category = %v, want %v", statusCode, se.Category, wantCategory)
	}
	if se.HTTPStatus != statusCode {
		t.Errorf("HTTP %d: HTTPStatus = %d, want %d", statusCode, se.HTTPStatus, statusCode)
	}
}

// TestSearchThreadsHTTP401 verifies 401 maps to Permanent.
func TestSearchThreadsHTTP401(t *testing.T) {
	testHTTPStatusMapping(t, http.StatusUnauthorized, types.CategoryPermanent)
}

// TestSearchThreadsHTTP403 verifies 403 maps to Permanent.
func TestSearchThreadsHTTP403(t *testing.T) {
	testHTTPStatusMapping(t, http.StatusForbidden, types.CategoryPermanent)
}

// TestSearchThreadsHTTP4xx verifies 400/404 map to Permanent.
func TestSearchThreadsHTTP4xx(t *testing.T) {
	testHTTPStatusMapping(t, http.StatusBadRequest, types.CategoryPermanent)
	testHTTPStatusMapping(t, http.StatusNotFound, types.CategoryPermanent)
}

// TestSearchThreadsHTTP5xx verifies 500/503 map to Unavailable.
func TestSearchThreadsHTTP5xx(t *testing.T) {
	testHTTPStatusMapping(t, http.StatusInternalServerError, types.CategoryUnavailable)
	testHTTPStatusMapping(t, http.StatusServiceUnavailable, types.CategoryUnavailable)
}

// TestSearchThreadsHTTP429WithRetryAfter verifies 429 with Retry-After header.
func TestSearchThreadsHTTP429WithRetryAfter(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	a, _ := NewThreads(ThreadsOptions{
		AccessToken: "t",
		BaseURL:     ts.URL,
		HTTPClient:  ts.Client(),
	})
	_, err := a.Search(t.Context(), types.Query{Text: "test"})
	if err == nil {
		t.Fatal("expected error for 429, got nil")
	}
	var se *types.SourceError
	if !errors.As(err, &se) {
		t.Fatalf("expected *types.SourceError, got %T", err)
	}
	if se.Category != types.CategoryRateLimited {
		t.Errorf("Category = %v, want RateLimited", se.Category)
	}
	if se.RetryAfter != 30*time.Second {
		t.Errorf("RetryAfter = %v, want 30s", se.RetryAfter)
	}
}

// TestSearchThreadsHTTP429Defaults5s verifies 429 without Retry-After defaults to 5s.
func TestSearchThreadsHTTP429Defaults5s(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	a, _ := NewThreads(ThreadsOptions{
		AccessToken: "t",
		BaseURL:     ts.URL,
		HTTPClient:  ts.Client(),
	})
	_, err := a.Search(t.Context(), types.Query{Text: "test"})
	var se *types.SourceError
	if !errors.As(err, &se) {
		t.Fatalf("expected *types.SourceError, got %T", err)
	}
	if se.RetryAfter != 5*time.Second {
		t.Errorf("RetryAfter = %v, want 5s", se.RetryAfter)
	}
}

// TestSearchThreadsHTTP429Capped60s verifies 429 with large Retry-After is capped.
func TestSearchThreadsHTTP429Capped60s(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "999")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	a, _ := NewThreads(ThreadsOptions{
		AccessToken: "t",
		BaseURL:     ts.URL,
		HTTPClient:  ts.Client(),
	})
	_, err := a.Search(t.Context(), types.Query{Text: "test"})
	var se *types.SourceError
	if !errors.As(err, &se) {
		t.Fatalf("expected *types.SourceError, got %T", err)
	}
	if se.RetryAfter != 60*time.Second {
		t.Errorf("RetryAfter = %v, want 60s (capped)", se.RetryAfter)
	}
}

// TestSearchThreadsConnectionRefused verifies connection error yields Unavailable.
func TestSearchThreadsConnectionRefused(t *testing.T) {
	// Connect to a port that's not listening.
	a, _ := NewThreads(ThreadsOptions{
		AccessToken: "t",
		BaseURL:     "http://127.0.0.1:1",
	})
	_, err := a.Search(t.Context(), types.Query{Text: "test"})
	if err == nil {
		t.Fatal("expected error for connection refused, got nil")
	}
	var se *types.SourceError
	if !errors.As(err, &se) {
		t.Fatalf("expected *types.SourceError, got %T: %v", err, err)
	}
	if se.Category != types.CategoryUnavailable {
		t.Errorf("Category = %v, want Unavailable", se.Category)
	}
	if se.HTTPStatus != 0 {
		t.Errorf("HTTPStatus = %d, want 0 for network error", se.HTTPStatus)
	}
}

// TestSearchThreadsNoInternalRetry verifies only one request is made per call.
func TestSearchThreadsNoInternalRetry(t *testing.T) {
	var count atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	a, _ := NewThreads(ThreadsOptions{
		AccessToken: "t",
		BaseURL:     ts.URL,
		HTTPClient:  ts.Client(),
	})
	_, _ = a.Search(t.Context(), types.Query{Text: "test"})

	if got := count.Load(); got != 1 {
		t.Errorf("request count = %d, want 1 (no internal retry)", got)
	}
}

// ---------------------------------------------------------------------------
// REQ-ADP10-004: Graph error envelope + empty data
// ---------------------------------------------------------------------------

// TestSearchThreadsGraphErrorEnvelope verifies 200 + error envelope → Permanent.
func TestSearchThreadsGraphErrorEnvelope(t *testing.T) {
	body := loadFixture(t, "threads_keyword_search_response_graph_error.json")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer ts.Close()

	a, _ := NewThreads(ThreadsOptions{
		AccessToken: "t",
		BaseURL:     ts.URL,
		HTTPClient:  ts.Client(),
	})
	_, err := a.Search(t.Context(), types.Query{Text: "test"})
	if err == nil {
		t.Fatal("expected error from Graph error envelope, got nil")
	}
	var se *types.SourceError
	if !errors.As(err, &se) {
		t.Fatalf("expected *types.SourceError, got %T", err)
	}
	if se.Category != types.CategoryPermanent {
		t.Errorf("Category = %v, want Permanent", se.Category)
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "Unsupported get request") {
		t.Errorf("error message missing Graph message: %s", errMsg)
	}
	if !strings.Contains(errMsg, "code 12") {
		t.Errorf("error message missing code: %s", errMsg)
	}
}

// TestSearchThreadsEmptyDataIsEmptyResult verifies empty data returns (nil, nil).
func TestSearchThreadsEmptyDataIsEmptyResult(t *testing.T) {
	body := loadFixture(t, "threads_keyword_search_response_empty.json")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer ts.Close()

	a, _ := NewThreads(ThreadsOptions{
		AccessToken: "t",
		BaseURL:     ts.URL,
		HTTPClient:  ts.Client(),
	})
	docs, err := a.Search(t.Context(), types.Query{Text: "sensitive keyword"})
	if err != nil {
		t.Fatalf("unexpected error for empty data: %v", err)
	}
	if docs != nil {
		t.Errorf("expected nil docs for empty data, got %d", len(docs))
	}
}

// ---------------------------------------------------------------------------
// REQ-ADP10-006: Optional since/until filters
// ---------------------------------------------------------------------------

// TestSearchThreadsSinceFilterAdded verifies valid since filter is added to URL.
func TestSearchThreadsSinceFilterAdded(t *testing.T) {
	var capturedURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer ts.Close()

	a, _ := NewThreads(ThreadsOptions{
		AccessToken: "t",
		BaseURL:     ts.URL,
		HTTPClient:  ts.Client(),
	})
	_, _ = a.Search(t.Context(), types.Query{
		Text: "test",
		Filters: []types.Filter{
			{Key: "since", Value: "2026-01-01T00:00:00Z"},
		},
	})

	if !strings.Contains(capturedURL, "since=") {
		t.Errorf("URL missing 'since' filter: %s", capturedURL)
	}
}

// TestSearchThreadsUntilFilterAdded verifies valid until filter is added to URL.
func TestSearchThreadsUntilFilterAdded(t *testing.T) {
	var capturedURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer ts.Close()

	a, _ := NewThreads(ThreadsOptions{
		AccessToken: "t",
		BaseURL:     ts.URL,
		HTTPClient:  ts.Client(),
	})
	_, _ = a.Search(t.Context(), types.Query{
		Text: "test",
		Filters: []types.Filter{
			{Key: "until", Value: "2026-06-01T00:00:00Z"},
		},
	})

	if !strings.Contains(capturedURL, "until=") {
		t.Errorf("URL missing 'until' filter: %s", capturedURL)
	}
}

// TestSearchThreadsSinceFilterDroppedWhenMalformed verifies malformed since is dropped.
func TestSearchThreadsSinceFilterDroppedWhenMalformed(t *testing.T) {
	var capturedURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer ts.Close()

	a, _ := NewThreads(ThreadsOptions{
		AccessToken: "t",
		BaseURL:     ts.URL,
		HTTPClient:  ts.Client(),
	})
	_, _ = a.Search(t.Context(), types.Query{
		Text: "test",
		Filters: []types.Filter{
			{Key: "since", Value: "yesterday"},
		},
	})

	if strings.Contains(capturedURL, "since=") {
		t.Errorf("URL should NOT contain malformed 'since': %s", capturedURL)
	}
}

// TestSearchThreadsUnknownFilterIgnored verifies unknown filter keys are ignored.
func TestSearchThreadsUnknownFilterIgnored(t *testing.T) {
	var capturedURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer ts.Close()

	a, _ := NewThreads(ThreadsOptions{
		AccessToken: "t",
		BaseURL:     ts.URL,
		HTTPClient:  ts.Client(),
	})
	_, _ = a.Search(t.Context(), types.Query{
		Text: "test",
		Filters: []types.Filter{
			{Key: "tag", Value: "x"},
		},
	})

	if strings.Contains(capturedURL, "tag=") {
		t.Errorf("URL should NOT contain unknown filter 'tag': %s", capturedURL)
	}
}

// ---------------------------------------------------------------------------
// REQ-ADP10-007: Concurrent safety
// ---------------------------------------------------------------------------

// TestSearchThreadsConcurrentSafe verifies 50 concurrent goroutines are safe.
func TestSearchThreadsConcurrentSafe(t *testing.T) {
	body := loadFixture(t, "threads_keyword_search_response.json")
	var requestCount atomic.Int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer ts.Close()

	a, _ := NewThreads(ThreadsOptions{
		AccessToken: "t",
		BaseURL:     ts.URL,
		HTTPClient:  ts.Client(),
	})

	var wg sync.WaitGroup
	errCh := make(chan error, 50)

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			docs, err := a.Search(context.Background(), types.Query{Text: "concurrent test"})
			if err != nil {
				errCh <- err
				return
			}
			if len(docs) != 25 {
				errCh <- fmt.Errorf("got %d docs, want 25", len(docs))
			}
		}()
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent search error: %v", err)
	}

	if got := requestCount.Load(); got != 50 {
		t.Errorf("request count = %d, want 50", got)
	}
}

// TestSearchBothSubSourcesConcurrent verifies Threads + Facebook instances concurrently.
func TestSearchBothSubSourcesConcurrent(t *testing.T) {
	body := loadFixture(t, "threads_keyword_search_response.json")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer ts.Close()

	threadsAdapter, _ := NewThreads(ThreadsOptions{
		AccessToken: "t",
		BaseURL:     ts.URL,
		HTTPClient:  ts.Client(),
	})
	facebookAdapter, _ := NewFacebook(FacebookOptions{})

	var wg sync.WaitGroup
	errCh := make(chan error, 100)

	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			docs, err := threadsAdapter.Search(context.Background(), types.Query{Text: "test"})
			if err != nil {
				errCh <- fmt.Errorf("threads: %v", err)
			} else if len(docs) != 25 {
				errCh <- fmt.Errorf("threads: got %d docs, want 25", len(docs))
			}
		}()
		go func() {
			defer wg.Done()
			_, err := facebookAdapter.Search(context.Background(), types.Query{Text: "test"})
			if err == nil {
				errCh <- fmt.Errorf("facebook: expected error, got nil")
			} else if !errors.Is(err, ErrFacebookNotSupported) {
				errCh <- fmt.Errorf("facebook: wrong error: %v", err)
			}
		}()
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("sub-source concurrent error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// REQ-ADP10-008: Facebook not-supported
// ---------------------------------------------------------------------------

// TestSearchFacebookAlwaysNotSupported verifies every invocation returns ErrFacebookNotSupported.
func TestSearchFacebookAlwaysNotSupported(t *testing.T) {
	a, _ := NewFacebook(FacebookOptions{})

	queries := []types.Query{
		{Text: "facebook"},
		{Text: "meta"},
		{Text: ""},
		{Text: "test", MaxResults: 100},
	}

	for _, q := range queries {
		_, err := a.Search(t.Context(), q)
		if !errors.Is(err, ErrFacebookNotSupported) {
			t.Errorf("Search(%q): err = %v, want ErrFacebookNotSupported", q.Text, err)
		}
		if !errors.Is(err, types.ErrPermanent) {
			t.Errorf("Search(%q): err should satisfy ErrPermanent", q.Text)
		}
	}
}

// TestSearchFacebookMakesNoHTTPRequest verifies zero HTTP requests.
func TestSearchFacebookMakesNoHTTPRequest(t *testing.T) {
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		called = true
	}))
	defer ts.Close()

	a, _ := NewFacebook(FacebookOptions{})
	_, _ = a.Search(t.Context(), types.Query{Text: "test"})

	if called {
		t.Error("Facebook adapter made an HTTP request, expected zero")
	}
}

// TestFacebookNotSupportedMessageDocumentsBlocker verifies error message content.
func TestFacebookNotSupportedMessageDocumentsBlocker(t *testing.T) {
	a, _ := NewFacebook(FacebookOptions{})
	_, err := a.Search(t.Context(), types.Query{Text: "test"})

	errMsg := err.Error()
	if !strings.Contains(errMsg, "Facebook Graph API") {
		t.Errorf("error message missing 'Facebook Graph API': %s", errMsg)
	}
	if !strings.Contains(errMsg, "no public-post keyword search") {
		t.Errorf("error message missing 'no public-post keyword search': %s", errMsg)
	}
}

// ---------------------------------------------------------------------------
// REQ-ADP10-009: Empty/whitespace query rejection
// ---------------------------------------------------------------------------

// TestSearchThreadsEmptyQueryRejectedNoHTTP verifies empty queries are rejected without HTTP.
func TestSearchThreadsEmptyQueryRejectedNoHTTP(t *testing.T) {
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		requestCount++
	}))
	defer ts.Close()

	a, _ := NewThreads(ThreadsOptions{
		AccessToken: "t",
		BaseURL:     ts.URL,
		HTTPClient:  ts.Client(),
	})

	for _, text := range []string{"", "   ", "\t\n"} {
		requestCount = 0
		_, err := a.Search(t.Context(), types.Query{Text: text})
		if err == nil {
			t.Errorf("text %q: expected error, got nil", text)
		}
		if !errors.Is(err, ErrInvalidQuery) {
			t.Errorf("text %q: err = %v, want ErrInvalidQuery", text, err)
		}
		if !errors.Is(err, types.ErrPermanent) {
			t.Errorf("text %q: err should satisfy ErrPermanent", text)
		}
		if requestCount != 0 {
			t.Errorf("text %q: made %d HTTP requests, want 0", text, requestCount)
		}
	}
}

// ---------------------------------------------------------------------------
// NFR-ADP10-002: Secret handling — token not in error messages
// ---------------------------------------------------------------------------

// TestThreadsTokenNotInErrorMessages verifies the token is absent from error outputs.
func TestThreadsTokenNotInErrorMessages(t *testing.T) {
	secret := "super-secret-token-abc123"

	tests := []struct {
		name       string
		statusCode int
	}{
		{"401", http.StatusUnauthorized},
		{"403", http.StatusForbidden},
		{"500", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer ts.Close()

			a, _ := NewThreads(ThreadsOptions{
				AccessToken: secret,
				BaseURL:     ts.URL,
				HTTPClient:  ts.Client(),
			})
			_, err := a.Search(t.Context(), types.Query{Text: "test"})
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			errMsg := err.Error()
			if strings.Contains(errMsg, secret) {
				t.Errorf("token leaked into error message: %s", errMsg)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// NFR-ADP10-003: No goroutine leak on ctx cancel
// ---------------------------------------------------------------------------

// TestSearchThreadsNoGoroutineLeakOnCancel verifies no goroutine leak after ctx cancel.
func TestSearchThreadsNoGoroutineLeakOnCancel(t *testing.T) {
	defer goleak.VerifyNone(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Simulate slow response.
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer ts.Close()

	a, _ := NewThreads(ThreadsOptions{
		AccessToken: "t",
		BaseURL:     ts.URL,
		HTTPClient:  ts.Client(),
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	_, _ = a.Search(ctx, types.Query{Text: "test"})
}
