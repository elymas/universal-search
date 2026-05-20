package naver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// TestSearch_BlogHappyPath verifies end-to-end blog search with httptest server.
// REQ-ADP8-002.
func TestSearch_BlogHappyPath(t *testing.T) {
	t.Parallel()
	body := mustReadTestFile(t, "testdata/search_response_blog.json")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	a := newTestAdapter(t, srv.URL)

	docs, err := a.Search(context.Background(), types.Query{Text: "golang"})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(docs) != 25 {
		t.Fatalf("len(docs) = %d, want 25", len(docs))
	}

	// Validate all docs.
	for i, doc := range docs {
		docCopy := doc
		if err := docCopy.Validate(); err != nil {
			t.Errorf("docs[%d].Validate() error = %v", i, err)
		}
	}
}

// TestSearch_NewsVertical verifies vertical dispatch to news endpoint.
// REQ-ADP8-002.
func TestSearch_NewsVertical(t *testing.T) {
	t.Parallel()
	body := mustReadTestFile(t, "testdata/search_response_news.json")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	a := newTestAdapter(t, srv.URL)

	docs, err := a.Search(context.Background(), types.Query{
		Text: "뉴스",
		Filters: []types.Filter{
			{Key: filterKeyVertical, Value: verticalNews},
		},
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(docs) != 25 {
		t.Fatalf("len(docs) = %d, want 25", len(docs))
	}
	if docs[0].DocType != types.DocTypeArticle {
		t.Errorf("docs[0].DocType = %v, want DocTypeArticle", docs[0].DocType)
	}
}

// TestSearch_WebVertical verifies vertical dispatch to web endpoint.
func TestSearch_WebVertical(t *testing.T) {
	t.Parallel()
	body := mustReadTestFile(t, "testdata/search_response_web.json")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	a := newTestAdapter(t, srv.URL)

	docs, err := a.Search(context.Background(), types.Query{
		Text: "검색",
		Filters: []types.Filter{
			{Key: filterKeyVertical, Value: verticalWeb},
		},
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(docs) != 25 {
		t.Fatalf("len(docs) = %d, want 25", len(docs))
	}
}

// TestSearch_ShopVertical verifies vertical dispatch to shop endpoint.
func TestSearch_ShopVertical(t *testing.T) {
	t.Parallel()
	body := mustReadTestFile(t, "testdata/search_response_shop.json")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	a := newTestAdapter(t, srv.URL)

	docs, err := a.Search(context.Background(), types.Query{
		Text: "상품",
		Filters: []types.Filter{
			{Key: filterKeyVertical, Value: verticalShop},
		},
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(docs) != 25 {
		t.Fatalf("len(docs) = %d, want 25", len(docs))
	}
}

// TestSearch_CursorPropagation verifies cursor is passed as start= parameter.
// REQ-ADP8-007.
func TestSearch_CursorPropagation(t *testing.T) {
	t.Parallel()
	body := mustReadTestFile(t, "testdata/search_response_blog_pagination.json")

	var gotStart string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotStart = r.URL.Query().Get("start")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	a := newTestAdapter(t, srv.URL)

	docs, err := a.Search(context.Background(), types.Query{
		Text:   "golang",
		Cursor: "26",
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if gotStart != "26" {
		t.Errorf("start param = %q, want %q", gotStart, "26")
	}

	// Last doc should have next_cursor = "51".
	if len(docs) == 0 {
		t.Fatal("no docs returned")
	}
	last := docs[len(docs)-1]
	if last.Metadata == nil {
		t.Fatal("last doc Metadata nil")
	}
	if cursor, ok := last.Metadata["next_cursor"]; !ok || cursor != "51" {
		t.Errorf("next_cursor = %v, want %q", last.Metadata["next_cursor"], "51")
	}
}

// TestSearch_EmptyResultsReturnsNil verifies empty result set returns nil slice,
// not error. REQ-ADP8-006.
func TestSearch_EmptyResultsReturnsNil(t *testing.T) {
	t.Parallel()
	body := mustReadTestFile(t, "testdata/search_response_blog_empty.json")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	a := newTestAdapter(t, srv.URL)

	docs, err := a.Search(context.Background(), types.Query{Text: "nobody finds this"})
	if err != nil {
		t.Fatalf("Search() error = %v, want nil", err)
	}
	if len(docs) != 0 {
		t.Errorf("len(docs) = %d, want 0", len(docs))
	}
}

// TestSearch_HTTP401_CategoryPermanent verifies 401 maps to CategoryPermanent.
// REQ-ADP8-003: auth failure is permanent.
func TestSearch_HTTP401_CategoryPermanent(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	a := newTestAdapter(t, srv.URL)

	_, err := a.Search(context.Background(), types.Query{Text: "test"})
	if err == nil {
		t.Fatal("Search() error = nil, want error for 401")
	}
	se, ok := err.(*types.SourceError)
	if !ok {
		t.Fatalf("error type = %T, want *types.SourceError", err)
	}
	if se.Category != types.CategoryPermanent {
		t.Errorf("Category = %v, want CategoryPermanent", se.Category)
	}
	if se.HTTPStatus != http.StatusUnauthorized {
		t.Errorf("HTTPStatus = %d, want 401", se.HTTPStatus)
	}
}

// TestSearch_HTTP403_CategoryPermanent verifies 403 maps to CategoryPermanent.
func TestSearch_HTTP403_CategoryPermanent(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	a := newTestAdapter(t, srv.URL)

	_, err := a.Search(context.Background(), types.Query{Text: "test"})
	se, ok := err.(*types.SourceError)
	if !ok || se.Category != types.CategoryPermanent {
		t.Errorf("Search() for 403: category = %v, want CategoryPermanent", se.Category)
	}
}

// TestSearch_HTTP429_CategoryRateLimited verifies 429 maps to CategoryRateLimited.
// REQ-ADP8-003.
func TestSearch_HTTP429_CategoryRateLimited(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	a := newTestAdapter(t, srv.URL)

	_, err := a.Search(context.Background(), types.Query{Text: "test"})
	if err == nil {
		t.Fatal("Search() error = nil, want error for 429")
	}
	se, ok := err.(*types.SourceError)
	if !ok {
		t.Fatalf("error type = %T, want *types.SourceError", err)
	}
	if se.Category != types.CategoryRateLimited {
		t.Errorf("Category = %v, want CategoryRateLimited", se.Category)
	}
	if se.RetryAfter != 30*time.Second {
		t.Errorf("RetryAfter = %v, want 30s", se.RetryAfter)
	}
}

// TestSearch_HTTP500_CategoryUnavailable verifies 5xx maps to CategoryUnavailable.
func TestSearch_HTTP500_CategoryUnavailable(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	a := newTestAdapter(t, srv.URL)

	_, err := a.Search(context.Background(), types.Query{Text: "test"})
	se, ok := err.(*types.SourceError)
	if !ok || se.Category != types.CategoryUnavailable {
		t.Errorf("Search() for 500: category = %v, want CategoryUnavailable", se.Category)
	}
}

// TestSearch_ContextCancellation verifies cancelled context propagates correctly.
func TestSearch_ContextCancellation(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Slow handler — never completes before cancel.
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := newTestAdapter(t, srv.URL)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := a.Search(ctx, types.Query{Text: "test"})
	if err == nil {
		t.Error("Search() error = nil, want error for cancelled context")
	}
}

// TestSearch_ConcurrentSafety verifies concurrent Search calls don't race.
// REQ-ADP8-011: goroutine-safe after construction.
func TestSearch_ConcurrentSafety(t *testing.T) {
	t.Parallel()
	body := mustReadTestFile(t, "testdata/search_response_blog.json")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	a := newTestAdapter(t, srv.URL)

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)
	errs := make(chan error, goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()
			_, err := a.Search(context.Background(), types.Query{Text: "test"})
			if err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent Search() error = %v", err)
	}
}

// TestFilterVertical verifies vertical selection from Query.Filters.
func TestFilterVertical(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		filters []types.Filter
		want    string
	}{
		{"default (no filter)", nil, verticalBlog},
		{"explicit blog", []types.Filter{{Key: filterKeyVertical, Value: "blog"}}, verticalBlog},
		{"news", []types.Filter{{Key: filterKeyVertical, Value: "news"}}, verticalNews},
		{"web", []types.Filter{{Key: filterKeyVertical, Value: "web"}}, verticalWeb},
		{"shop", []types.Filter{{Key: filterKeyVertical, Value: "shop"}}, verticalShop},
		{"datalab", []types.Filter{{Key: filterKeyVertical, Value: "datalab"}}, verticalDataLab},
		{"unknown value defaults to blog", []types.Filter{{Key: filterKeyVertical, Value: "invalid"}}, verticalBlog},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := filterVertical(tc.filters)
			if got != tc.want {
				t.Errorf("filterVertical(%v) = %q, want %q", tc.filters, got, tc.want)
			}
		})
	}
}

// TestBuildSearchURL verifies URL construction with query parameters.
func TestBuildSearchURL(t *testing.T) {
	t.Parallel()

	u := buildSearchURL("https://openapi.naver.com/v1/search/blog.json", types.Query{
		Text:       "golang 테스트",
		MaxResults: 10,
		Cursor:     "26",
	})

	if u == "" {
		t.Fatal("buildSearchURL() returned empty string")
	}

	// Spot-check the URL contains expected parameters.
	for _, want := range []string{"query=", "display=10", "start=26"} {
		found := false
		if len(u) >= len(want) {
			for i := 0; i <= len(u)-len(want); i++ {
				if u[i:i+len(want)] == want {
					found = true
					break
				}
			}
		}
		if !found {
			t.Errorf("buildSearchURL() = %q, missing %q", u, want)
		}
	}
}

// TestParseStartCursor verifies cursor-to-start integer parsing.
func TestParseStartCursor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		cursor string
		want   int
	}{
		{"", 1},
		{"1", 1},
		{"26", 26},
		{"abc", 1},
		{"-5", 1},
		{"0", 1},
		{"100", 100},
	}

	for _, tc := range tests {
		tc := tc
		t.Run("cursor_"+tc.cursor, func(t *testing.T) {
			t.Parallel()
			got := parseStartCursor(tc.cursor)
			if got != tc.want {
				t.Errorf("parseStartCursor(%q) = %d, want %d", tc.cursor, got, tc.want)
			}
		})
	}
}

// mustReadTestFile reads a file using the package-level mustReadFile helper.
func mustReadTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}
	return data
}
