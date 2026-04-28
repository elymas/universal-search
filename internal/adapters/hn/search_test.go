// Package hn — Search hot-path integration tests.
// Covers REQ-ADP2-002/003/007/008/009/010: URL params, HTTP error mapping,
// filter handling, empty-query/invalid-cursor rejection, concurrency safety,
// and goroutine-leak detection.
package hn

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/elymas/universal-search/pkg/types"
)

// searchTestContext returns a context with a 10-second deadline for search tests.
func searchTestContext(t *testing.T) context.Context {
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
		t.Fatalf("os.ReadFile(search_response.json): %v", err)
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	t.Cleanup(ts.Close)
	return ts
}

// emptyResponseJSON is a minimal Algolia envelope with no hits.
const emptyResponseJSON = `{"hits":[],"nbHits":0,"page":0,"nbPages":0,"hitsPerPage":25}`

// TestSearchHappyPath25Docs verifies Search returns 25 valid NormalizedDocs
// from the standard fixture.
func TestSearchHappyPath25Docs(t *testing.T) {
	t.Parallel()

	ts := happyPathServer(t)
	a, err := New(Options{BaseURL: ts.URL})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	ctx := searchTestContext(t)
	docs, err := a.Search(ctx, types.Query{Text: "golang"})
	if err != nil {
		t.Fatalf("Search(): %v", err)
	}
	if len(docs) != 25 {
		t.Errorf("Search() returned %d docs, want 25", len(docs))
	}
	for i, doc := range docs {
		if err := doc.Validate(); err != nil {
			t.Errorf("docs[%d].Validate(): %v", i, err)
		}
	}
}

// TestSearchURLParametersQuery verifies that the query= parameter is set.
func TestSearchURLParametersQuery(t *testing.T) {
	t.Parallel()

	var capturedURL *url.URL
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(emptyResponseJSON))
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	ctx := searchTestContext(t)
	_, _ = a.Search(ctx, types.Query{Text: "golang test"})

	if capturedURL == nil {
		t.Fatal("no request captured")
	}
	params := capturedURL.Query()

	if got := params.Get("query"); got != "golang test" {
		t.Errorf("query = %q, want %q", got, "golang test")
	}
	if got := params.Get("tags"); got != "story" {
		t.Errorf("tags = %q, want %q", got, "story")
	}
	if got := params.Get("hitsPerPage"); got == "" {
		t.Error("hitsPerPage param missing")
	}
}

// TestSearchClampsHitsPerPageTo100 verifies that MaxResults > 100 is clamped.
func TestSearchClampsHitsPerPageTo100(t *testing.T) {
	t.Parallel()

	var capturedHPP string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHPP = r.URL.Query().Get("hitsPerPage")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(emptyResponseJSON))
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	ctx := searchTestContext(t)
	_, _ = a.Search(ctx, types.Query{Text: "golang", MaxResults: 500})

	if capturedHPP != "100" {
		t.Errorf("hitsPerPage = %q, want %q", capturedHPP, "100")
	}
}

// TestSearchDefaultsHitsPerPageTo25 verifies that MaxResults=0 defaults to 25.
func TestSearchDefaultsHitsPerPageTo25(t *testing.T) {
	t.Parallel()

	var capturedHPP string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHPP = r.URL.Query().Get("hitsPerPage")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(emptyResponseJSON))
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	ctx := searchTestContext(t)
	_, _ = a.Search(ctx, types.Query{Text: "golang", MaxResults: 0})

	if capturedHPP != "25" {
		t.Errorf("hitsPerPage = %q, want %q", capturedHPP, "25")
	}
}

// TestSearchHonoursCursorParameter verifies that q.Cursor is passed as page= param.
func TestSearchHonoursCursorParameter(t *testing.T) {
	t.Parallel()

	var capturedPage string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPage = r.URL.Query().Get("page")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(emptyResponseJSON))
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	ctx := searchTestContext(t)
	_, _ = a.Search(ctx, types.Query{Text: "golang", Cursor: "3"})

	if capturedPage != "3" {
		t.Errorf("page = %q, want %q", capturedPage, "3")
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

	a, err := New(Options{BaseURL: ts.URL})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	ctx := searchTestContext(t)
	_, searchErr := a.Search(ctx, types.Query{Text: "golang"})
	if searchErr == nil {
		t.Fatal("Search() expected error for 429, got nil")
	}

	var se *types.SourceError
	if !errors.As(searchErr, &se) {
		t.Fatalf("error is not *SourceError: %T — %v", searchErr, searchErr)
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

	a, err := New(Options{BaseURL: ts.URL})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	ctx := searchTestContext(t)
	_, searchErr := a.Search(ctx, types.Query{Text: "golang"})
	if searchErr == nil {
		t.Fatal("Search() expected error for 429, got nil")
	}

	var se *types.SourceError
	if !errors.As(searchErr, &se) {
		t.Fatalf("error is not *SourceError: %T — %v", searchErr, searchErr)
	}
	if se.Category != types.CategoryRateLimited {
		t.Errorf("Category = %v, want CategoryRateLimited", se.Category)
	}
	// RetryAfter should be roughly 30s (within ±10s clock drift).
	if se.RetryAfter < 20*time.Second || se.RetryAfter > 60*time.Second {
		t.Errorf("RetryAfter = %v, want ~30s", se.RetryAfter)
	}
}

// TestSearchHTTP429MissingRetryAfterUsesDefault verifies the 5s default.
func TestSearchHTTP429MissingRetryAfterUsesDefault(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 429 with no Retry-After header.
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	ctx := searchTestContext(t)
	_, searchErr := a.Search(ctx, types.Query{Text: "golang"})
	if searchErr == nil {
		t.Fatal("Search() expected error, got nil")
	}

	var se *types.SourceError
	if !errors.As(searchErr, &se) {
		t.Fatalf("error is not *SourceError: %T", searchErr)
	}
	if se.RetryAfter != 5*time.Second {
		t.Errorf("RetryAfter = %v, want 5s", se.RetryAfter)
	}
}

// TestSearchHTTP4xxMappedToPermanent verifies 4xx → CategoryPermanent.
func TestSearchHTTP4xxMappedToPermanent(t *testing.T) {
	t.Parallel()

	for _, status := range []int{400, 401, 403, 404} {
		status := status
		t.Run(http.StatusText(status), func(t *testing.T) {
			t.Parallel()
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
			}))
			defer ts.Close()

			a, err := New(Options{BaseURL: ts.URL})
			if err != nil {
				t.Fatalf("New(): %v", err)
			}

			ctx := searchTestContext(t)
			_, searchErr := a.Search(ctx, types.Query{Text: "golang"})
			if searchErr == nil {
				t.Fatalf("Search() expected error for %d, got nil", status)
			}

			var se *types.SourceError
			if !errors.As(searchErr, &se) {
				t.Fatalf("error is not *SourceError: %T", searchErr)
			}
			if se.Category != types.CategoryPermanent {
				t.Errorf("status %d: Category = %v, want CategoryPermanent", status, se.Category)
			}
			if se.HTTPStatus != status {
				t.Errorf("status %d: HTTPStatus = %d, want %d", status, se.HTTPStatus, status)
			}
		})
	}
}

// TestSearchHTTP5xxMappedToUnavailable verifies 5xx → CategoryUnavailable.
func TestSearchHTTP5xxMappedToUnavailable(t *testing.T) {
	t.Parallel()

	for _, status := range []int{500, 502, 503, 504} {
		status := status
		t.Run(http.StatusText(status), func(t *testing.T) {
			t.Parallel()
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
			}))
			defer ts.Close()

			a, err := New(Options{BaseURL: ts.URL})
			if err != nil {
				t.Fatalf("New(): %v", err)
			}

			ctx := searchTestContext(t)
			_, searchErr := a.Search(ctx, types.Query{Text: "golang"})
			if searchErr == nil {
				t.Fatalf("Search() expected error for %d, got nil", status)
			}

			var se *types.SourceError
			if !errors.As(searchErr, &se) {
				t.Fatalf("error is not *SourceError: %T", searchErr)
			}
			if se.Category != types.CategoryUnavailable {
				t.Errorf("status %d: Category = %v, want CategoryUnavailable", status, se.Category)
			}
		})
	}
}

// TestSearchConnectionRefused verifies that a network-level failure maps to
// CategoryUnavailable with HTTPStatus=0.
func TestSearchConnectionRefused(t *testing.T) {
	t.Parallel()

	// Point to a port with nothing listening.
	a, err := New(Options{BaseURL: "http://127.0.0.1:1"})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	ctx := searchTestContext(t)
	_, searchErr := a.Search(ctx, types.Query{Text: "golang"})
	if searchErr == nil {
		t.Fatal("Search() expected error for connection refused, got nil")
	}

	var se *types.SourceError
	if !errors.As(searchErr, &se) {
		t.Fatalf("error is not *SourceError: %T — %v", searchErr, searchErr)
	}
	if se.Category != types.CategoryUnavailable {
		t.Errorf("Category = %v, want CategoryUnavailable", se.Category)
	}
	if se.HTTPStatus != 0 {
		t.Errorf("HTTPStatus = %d, want 0", se.HTTPStatus)
	}
}

// TestSearchFilterSince verifies the since filter maps to created_at_i>=.
func TestSearchFilterSince(t *testing.T) {
	t.Parallel()

	var capturedNumericFilters string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedNumericFilters = r.URL.Query().Get("numericFilters")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(emptyResponseJSON))
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	ctx := searchTestContext(t)
	q := types.Query{
		Text:    "golang",
		Filters: []types.Filter{{Key: "since", Value: "1700000000"}},
	}
	_, _ = a.Search(ctx, q)

	want := "created_at_i>=1700000000"
	if capturedNumericFilters != want {
		t.Errorf("numericFilters = %q, want %q", capturedNumericFilters, want)
	}
}

// TestSearchFilterMinPoints verifies the min_points filter maps to points>=.
func TestSearchFilterMinPoints(t *testing.T) {
	t.Parallel()

	var capturedNumericFilters string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedNumericFilters = r.URL.Query().Get("numericFilters")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(emptyResponseJSON))
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	ctx := searchTestContext(t)
	q := types.Query{
		Text:    "golang",
		Filters: []types.Filter{{Key: "min_points", Value: "50"}},
	}
	_, _ = a.Search(ctx, q)

	want := "points>=50"
	if capturedNumericFilters != want {
		t.Errorf("numericFilters = %q, want %q", capturedNumericFilters, want)
	}
}

// TestSearchFilterBothSinceAndMinPoints verifies both filters compose with comma separator.
func TestSearchFilterBothSinceAndMinPoints(t *testing.T) {
	t.Parallel()

	var capturedNumericFilters string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedNumericFilters = r.URL.Query().Get("numericFilters")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(emptyResponseJSON))
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	ctx := searchTestContext(t)
	q := types.Query{
		Text: "golang",
		Filters: []types.Filter{
			{Key: "since", Value: "1700000000"},
			{Key: "min_points", Value: "100"},
		},
	}
	_, _ = a.Search(ctx, q)

	want := "created_at_i>=1700000000,points>=100"
	if capturedNumericFilters != want {
		t.Errorf("numericFilters = %q, want %q", capturedNumericFilters, want)
	}
}

// TestSearchUnknownFilterIgnored verifies that unknown filter keys are silently ignored.
func TestSearchUnknownFilterIgnored(t *testing.T) {
	t.Parallel()

	var capturedNumericFilters string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedNumericFilters = r.URL.Query().Get("numericFilters")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(emptyResponseJSON))
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	ctx := searchTestContext(t)
	q := types.Query{
		Text:    "golang",
		Filters: []types.Filter{{Key: "unknown_key", Value: "whatever"}},
	}
	_, _ = a.Search(ctx, q)

	if capturedNumericFilters != "" {
		t.Errorf("numericFilters = %q, want empty string for unknown filter", capturedNumericFilters)
	}
}

// TestSearchMalformedFilterValueIgnored verifies that non-integer filter values are dropped.
func TestSearchMalformedFilterValueIgnored(t *testing.T) {
	t.Parallel()

	var capturedNumericFilters string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedNumericFilters = r.URL.Query().Get("numericFilters")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(emptyResponseJSON))
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	ctx := searchTestContext(t)
	q := types.Query{
		Text:    "golang",
		Filters: []types.Filter{{Key: "since", Value: "not-a-number"}},
	}
	_, _ = a.Search(ctx, q)

	if capturedNumericFilters != "" {
		t.Errorf("numericFilters = %q, want empty for malformed value", capturedNumericFilters)
	}
}

// TestSearchNegativeFilterValueIgnored verifies that negative filter values are dropped.
func TestSearchNegativeFilterValueIgnored(t *testing.T) {
	t.Parallel()

	var capturedNumericFilters string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedNumericFilters = r.URL.Query().Get("numericFilters")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(emptyResponseJSON))
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	ctx := searchTestContext(t)
	q := types.Query{
		Text:    "golang",
		Filters: []types.Filter{{Key: "min_points", Value: "-5"}},
	}
	_, _ = a.Search(ctx, q)

	if capturedNumericFilters != "" {
		t.Errorf("numericFilters = %q, want empty for negative value", capturedNumericFilters)
	}
}

// TestSearchNilFiltersOmitsNumericFilters verifies that nil Filters causes no numericFilters param.
func TestSearchNilFiltersOmitsNumericFilters(t *testing.T) {
	t.Parallel()

	var capturedQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(emptyResponseJSON))
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	ctx := searchTestContext(t)
	_, _ = a.Search(ctx, types.Query{Text: "golang", Filters: nil})

	params, _ := url.ParseQuery(capturedQuery)
	if params.Has("numericFilters") {
		t.Errorf("unexpected numericFilters param when Filters is nil: %q", capturedQuery)
	}
}

// TestSearchEmptyQueryTable verifies the empty/whitespace-only query rejection table.
func TestSearchEmptyQueryTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		text string
	}{
		{"empty string", ""},
		{"single space", " "},
		{"tab only", "\t"},
		{"newline only", "\n"},
		{"multiple spaces", "   "},
		{"mixed whitespace", " \t\n "},
	}

	a, err := New(Options{BaseURL: "http://127.0.0.1:1"}) // unreachable; no HTTP should fire
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := searchTestContext(t)
			_, searchErr := a.Search(ctx, types.Query{Text: tc.text})
			if searchErr == nil {
				t.Fatal("Search() expected error for empty/whitespace query, got nil")
			}
			var se *types.SourceError
			if !errors.As(searchErr, &se) {
				t.Fatalf("error is not *SourceError: %T", searchErr)
			}
			if se.Category != types.CategoryPermanent {
				t.Errorf("Category = %v, want CategoryPermanent", se.Category)
			}
			if !errors.Is(searchErr, ErrInvalidQuery) {
				t.Errorf("error does not wrap ErrInvalidQuery: %v", searchErr)
			}
		})
	}
}

// TestSearchInvalidCursorTable verifies non-integer and negative cursor rejection.
func TestSearchInvalidCursorTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		cursor string
	}{
		{"non-integer", "abc"},
		{"float", "1.5"},
		{"negative integer", "-1"},
		{"negative large", "-100"},
		{"empty with space", " "},
		{"t3_ reddit style", "t3_abc123"},
	}

	a, err := New(Options{BaseURL: "http://127.0.0.1:1"}) // unreachable
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := searchTestContext(t)
			_, searchErr := a.Search(ctx, types.Query{Text: "golang", Cursor: tc.cursor})
			if searchErr == nil {
				t.Fatalf("Search() with cursor %q expected error, got nil", tc.cursor)
			}
			var se *types.SourceError
			if !errors.As(searchErr, &se) {
				t.Fatalf("error is not *SourceError: %T", searchErr)
			}
			if se.Category != types.CategoryPermanent {
				t.Errorf("cursor %q: Category = %v, want CategoryPermanent", tc.cursor, se.Category)
			}
			if !errors.Is(searchErr, ErrInvalidCursor) {
				t.Errorf("cursor %q: error does not wrap ErrInvalidCursor: %v", tc.cursor, searchErr)
			}
		})
	}
}

// TestSearchConcurrentSafety verifies that 50 concurrent Search calls complete
// without data race (run with -race). REQ-ADP2-010.
func TestSearchConcurrentSafety(t *testing.T) {
	t.Parallel()

	ts := happyPathServer(t)
	a, err := New(Options{BaseURL: ts.URL})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	const goroutines = 50
	var wg sync.WaitGroup
	var errCount atomic.Int64

	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			ctx := context.Background()
			_, searchErr := a.Search(ctx, types.Query{Text: "golang", MaxResults: 25})
			if searchErr != nil {
				errCount.Add(1)
			}
		}()
	}
	wg.Wait()

	if n := errCount.Load(); n > 0 {
		t.Errorf("%d/%d concurrent Search() calls returned errors", n, goroutines)
	}
}

// TestSearchNoGoroutineLeak verifies that Search does not leak goroutines
// introduced by the adapter itself. HTTP transport and httptest server goroutines
// are excluded from the check because they are lifecycle-managed externally.
func TestSearchNoGoroutineLeak(t *testing.T) {
	// Capture goroutines running before the test body (net/http transport goroutines
	// from parallel tests may already be live). goleak.IgnoreCurrent() marks
	// those as baseline so only new goroutines introduced by this test are checked.
	defer goleak.VerifyNone(t,
		goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"),
		goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"),
		goleak.IgnoreTopFunction("net/http/httptest.(*Server).goServe.func1"),
		goleak.IgnoreTopFunction("net.(*netFD).accept"),
		goleak.IgnoreTopFunction("internal/poll.(*FD).Accept"),
	)

	body, err := os.ReadFile("testdata/search_response.json")
	if err != nil {
		t.Fatalf("os.ReadFile: %v", err)
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))

	a, err := New(Options{BaseURL: ts.URL})
	if err != nil {
		ts.Close()
		t.Fatalf("New(): %v", err)
	}

	ctx := searchTestContext(t)
	_, err = a.Search(ctx, types.Query{Text: "golang", MaxResults: 25})
	ts.Close() // Close before goleak check so server accept-loop exits.
	if err != nil {
		t.Fatalf("Search(): %v", err)
	}
}
