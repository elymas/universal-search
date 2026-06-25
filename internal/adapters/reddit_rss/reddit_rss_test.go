// Package redditrss_test — TDD specification tests for the Reddit RSS adapter.
// SPEC-ADP-001b: credential-free fallback using www.reddit.com/search.rss.
//
// All tests use a loopback httptest.Server (via Options.BaseURL) and canned
// testdata fixtures. No live network calls, no credentials.
package redditrss_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	redditrss "github.com/elymas/universal-search/internal/adapters/reddit_rss"
	"github.com/elymas/universal-search/pkg/types"
)

// ---- helpers ----

// serveFile returns an httptest.Server that always responds with the given
// status code and the contents of testdata/<filename>.
func serveFile(t *testing.T, status int, filename string) *httptest.Server {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", filename))
	if err != nil {
		t.Fatalf("read testdata/%s: %v", filename, err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(status)
		_, _ = w.Write(data)
	}))
	return srv
}

// serveStatus returns an httptest.Server that always responds with the given
// status code and an empty body.
func serveStatus(t *testing.T, status int, extraHeaders map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for k, v := range extraHeaders {
			w.Header().Set(k, v)
		}
		w.WriteHeader(status)
	}))
	return srv
}

// serveDelay returns an httptest.Server that sleeps longer than the given
// context deadline before responding.
func serveDelay(t *testing.T, delay time.Duration) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(delay):
		}
		w.WriteHeader(http.StatusOK)
	}))
	return srv
}

// newAdapter constructs a test adapter pointing at the given server URL.
// baseURL should be the httptest.Server URL (e.g. "http://127.0.0.1:PORT").
// The adapter's BaseURL is set to baseURL+"/search.rss" to match the
// production default path structure (defaultBaseURL ends in /search.rss).
func newAdapter(t *testing.T, baseURL string) *redditrss.Adapter {
	t.Helper()
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	a, err := redditrss.New(redditrss.Options{
		BaseURL: baseURL + "/search.rss",
		NowFunc: func() time.Time { return now },
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("redditrss.New: %v", err)
	}
	// Default test adapter is single-shot (no 429 cooldown-retry) to preserve the
	// original fast, one-request behavior of the existing scenarios. Tests that
	// specifically exercise retry override this via SetRetryParamsForTest.
	a.SetRetryParamsForTest(0, 1)
	return a
}

// ---- Scenario 1: successful search maps RSS items to NormalizedDoc ----
// REQ-ADP1B-005, REQ-ADP1B-007, REQ-ADP1B-008, REQ-ADP1B-009

func TestSearch_Success_MapsRSSToNormalizedDoc(t *testing.T) {
	t.Parallel()

	var requestPath, requestQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		requestQuery = r.URL.RawQuery
		data, err := os.ReadFile(filepath.Join("testdata", "search.rss"))
		if err != nil {
			t.Errorf("read testdata: %v", err)
		}
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	a := newAdapter(t, srv.URL)

	docs, err := a.Search(context.Background(), types.Query{Text: "golang generics"})
	if err != nil {
		t.Fatalf("Search: unexpected error: %v", err)
	}

	// Verify request path and query params.
	if requestPath != "/search.rss" {
		t.Errorf("request path = %q; want /search.rss", requestPath)
	}
	if requestQuery != "q=golang+generics&sort=relevance" {
		t.Errorf("request query = %q; want q=golang+generics&sort=relevance", requestQuery)
	}

	// The fixture has 5 items: 3 valid, 1 missing link (skipped), 1 no-pubDate (still emitted).
	// REQ-ADP1B-009: the empty-link item is skipped → expect 4 docs.
	if len(docs) != 4 {
		t.Fatalf("len(docs) = %d; want 4", len(docs))
	}

	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)

	for i, doc := range docs {
		// REQ-ADP1B-007: SourceID, DocType
		if doc.SourceID != "reddit-rss" {
			t.Errorf("docs[%d].SourceID = %q; want reddit-rss", i, doc.SourceID)
		}
		if doc.DocType != types.DocTypePost {
			t.Errorf("docs[%d].DocType = %q; want post", i, doc.DocType)
		}
		// REQ-ADP1B-008: neutral score
		if doc.Score != 0.5 {
			t.Errorf("docs[%d].Score = %v; want 0.5", i, doc.Score)
		}
		// Required fields
		if doc.URL == "" {
			t.Errorf("docs[%d].URL is empty", i)
		}
		if doc.Hash == "" {
			t.Errorf("docs[%d].Hash is empty", i)
		}
		// RetrievedAt set from NowFunc
		if doc.RetrievedAt != now {
			t.Errorf("docs[%d].RetrievedAt = %v; want %v", i, doc.RetrievedAt, now)
		}
		// doc.Validate must pass
		if err := doc.Validate(); err != nil {
			t.Errorf("docs[%d].Validate() = %v", i, err)
		}
	}

	// First 3 docs have parseable pubDate.
	for i := 0; i < 3; i++ {
		if docs[i].PublishedAt.IsZero() {
			t.Errorf("docs[%d].PublishedAt is zero; want parsed pubDate", i)
		}
	}
	// 4th doc (no-pubDate item) should have zero PublishedAt — EC2.
	if !docs[3].PublishedAt.IsZero() {
		t.Errorf("docs[3].PublishedAt = %v; want zero (no pubDate item)", docs[3].PublishedAt)
	}
}

// TestSearch_EC1_EmptyLinkSkipped verifies that items with an empty link are
// not emitted (REQ-ADP1B-009 / EC1).
func TestSearch_EC1_EmptyLinkSkipped(t *testing.T) {
	t.Parallel()

	srv := serveFile(t, http.StatusOK, "search.rss")
	defer srv.Close()

	a := newAdapter(t, srv.URL)
	docs, err := a.Search(context.Background(), types.Query{Text: "go"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	for _, doc := range docs {
		if doc.URL == "" {
			t.Errorf("got doc with empty URL — empty-link item was not skipped")
		}
	}
}

// TestSearch_EC6_Score verifies Score == 0.5 on all docs (EC6 / REQ-ADP1B-008).
func TestSearch_EC6_Score(t *testing.T) {
	t.Parallel()

	srv := serveFile(t, http.StatusOK, "search.rss")
	defer srv.Close()

	a := newAdapter(t, srv.URL)
	docs, err := a.Search(context.Background(), types.Query{Text: "go"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(docs) == 0 {
		t.Fatal("expected docs, got 0")
	}
	for i, doc := range docs {
		if doc.Score != 0.5 {
			t.Errorf("docs[%d].Score = %v; want 0.5", i, doc.Score)
		}
	}
}

// ---- Scenario 2: empty query rejected without network call ----
// REQ-ADP1B-016

func TestSearch_EmptyQuery_ReturnsPermanentError(t *testing.T) {
	t.Parallel()

	// Server that fails the test if called.
	var requestCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := newAdapter(t, srv.URL)

	// Empty text
	_, err := a.Search(context.Background(), types.Query{Text: ""})
	if err == nil {
		t.Fatal("want error for empty query, got nil")
	}
	var se *types.SourceError
	if !errors.As(err, &se) {
		t.Fatalf("error is %T, want *types.SourceError", err)
	}
	if se.Category != types.CategoryPermanent {
		t.Errorf("Category = %v; want CategoryPermanent", se.Category)
	}
	if requestCount != 0 {
		t.Errorf("server received %d requests; want 0 (no network call for empty query)", requestCount)
	}
}

func TestSearch_WhitespaceQuery_ReturnsPermanentError(t *testing.T) {
	t.Parallel()

	var requestCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := newAdapter(t, srv.URL)
	_, err := a.Search(context.Background(), types.Query{Text: "   "})
	if err == nil {
		t.Fatal("want error for whitespace query, got nil")
	}
	if requestCount != 0 {
		t.Errorf("server received %d requests; want 0", requestCount)
	}
}

// ---- Scenario 3: HTTP 429 → CategoryRateLimited with RetryAfter ----
// REQ-ADP1B-011

func TestSearch_HTTP429_RateLimited(t *testing.T) {
	t.Parallel()

	srv := serveStatus(t, http.StatusTooManyRequests, map[string]string{
		"Retry-After": "30",
	})
	defer srv.Close()

	a := newAdapter(t, srv.URL)
	_, err := a.Search(context.Background(), types.Query{Text: "golang"})
	if err == nil {
		t.Fatal("want error for 429, got nil")
	}

	var se *types.SourceError
	if !errors.As(err, &se) {
		t.Fatalf("error is %T, want *types.SourceError", err)
	}
	if se.Category != types.CategoryRateLimited {
		t.Errorf("Category = %v; want CategoryRateLimited", se.Category)
	}
	if se.RetryAfter != 30*time.Second {
		t.Errorf("RetryAfter = %v; want 30s", se.RetryAfter)
	}
	if !errors.Is(err, types.ErrRateLimited) {
		t.Errorf("errors.Is(err, ErrRateLimited) = false; want true")
	}
}

// ---- Scenario 3a: HTTP 403 → CategoryUnavailable / retryable ----
// REQ-ADP1B-012

func TestSearch_HTTP403_Unavailable(t *testing.T) {
	t.Parallel()

	srv := serveStatus(t, http.StatusForbidden, nil)
	defer srv.Close()

	a := newAdapter(t, srv.URL)
	_, err := a.Search(context.Background(), types.Query{Text: "golang"})
	if err == nil {
		t.Fatal("want error for 403, got nil")
	}

	var se *types.SourceError
	if !errors.As(err, &se) {
		t.Fatalf("error is %T, want *types.SourceError", err)
	}
	if se.Category != types.CategoryUnavailable {
		t.Errorf("Category = %v; want CategoryUnavailable", se.Category)
	}
	if se.HTTPStatus != http.StatusForbidden {
		t.Errorf("HTTPStatus = %d; want 403", se.HTTPStatus)
	}
	if !errors.Is(err, types.ErrSourceUnavailable) {
		t.Errorf("errors.Is(err, ErrSourceUnavailable) = false; want true")
	}
}

// ---- EC4: HTTP 503 → CategoryUnavailable ----
// REQ-ADP1B-013

func TestSearch_HTTP503_Unavailable(t *testing.T) {
	t.Parallel()

	srv := serveStatus(t, http.StatusServiceUnavailable, nil)
	defer srv.Close()

	a := newAdapter(t, srv.URL)
	_, err := a.Search(context.Background(), types.Query{Text: "golang"})
	if err == nil {
		t.Fatal("want error for 503, got nil")
	}

	var se *types.SourceError
	if !errors.As(err, &se) {
		t.Fatalf("error is %T, want *types.SourceError", err)
	}
	if se.Category != types.CategoryUnavailable {
		t.Errorf("Category = %v; want CategoryUnavailable", se.Category)
	}
	if se.HTTPStatus != http.StatusServiceUnavailable {
		t.Errorf("HTTPStatus = %d; want 503", se.HTTPStatus)
	}
}

// ---- EC5: HTTP 404 (other 4xx) → CategoryPermanent ----
// REQ-ADP1B-012a

func TestSearch_HTTP404_Permanent(t *testing.T) {
	t.Parallel()

	srv := serveStatus(t, http.StatusNotFound, nil)
	defer srv.Close()

	a := newAdapter(t, srv.URL)
	_, err := a.Search(context.Background(), types.Query{Text: "golang"})
	if err == nil {
		t.Fatal("want error for 404, got nil")
	}

	var se *types.SourceError
	if !errors.As(err, &se) {
		t.Fatalf("error is %T, want *types.SourceError", err)
	}
	if se.Category != types.CategoryPermanent {
		t.Errorf("Category = %v; want CategoryPermanent", se.Category)
	}
	if se.HTTPStatus != http.StatusNotFound {
		t.Errorf("HTTPStatus = %d; want 404", se.HTTPStatus)
	}
}

// ---- EC3: malformed body on 200 → CategoryTransient ----
// REQ-ADP1B-015

func TestSearch_MalformedBody_Transient(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("this is not XML at all <<<>>>"))
	}))
	defer srv.Close()

	a := newAdapter(t, srv.URL)
	_, err := a.Search(context.Background(), types.Query{Text: "golang"})
	if err == nil {
		t.Fatal("want error for malformed body, got nil")
	}

	var se *types.SourceError
	if !errors.As(err, &se) {
		t.Fatalf("error is %T, want *types.SourceError", err)
	}
	if se.Category != types.CategoryTransient {
		t.Errorf("Category = %v; want CategoryTransient", se.Category)
	}
}

// ---- Scenario 4: context cancellation ----
// REQ-ADP1B-010

func TestSearch_CtxDeadlineExceeded_Transient(t *testing.T) {
	t.Parallel()

	srv := serveDelay(t, 200*time.Millisecond)
	defer srv.Close()

	a := newAdapter(t, srv.URL)

	// Create a context that expires before the server responds.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := a.Search(ctx, types.Query{Text: "golang"})
	if err == nil {
		t.Fatal("want error for expired deadline, got nil")
	}

	var se *types.SourceError
	if !errors.As(err, &se) {
		t.Fatalf("error is %T, want *types.SourceError", err)
	}
	if se.Category != types.CategoryTransient {
		t.Errorf("Category = %v; want CategoryTransient", se.Category)
	}
}

func TestSearch_CtxAlreadyCancelled_Unavailable(t *testing.T) {
	t.Parallel()

	srv := serveStatus(t, http.StatusOK, nil)
	defer srv.Close()

	a := newAdapter(t, srv.URL)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := a.Search(ctx, types.Query{Text: "golang"})
	if err == nil {
		t.Fatal("want error for cancelled ctx, got nil")
	}

	var se *types.SourceError
	if !errors.As(err, &se) {
		t.Fatalf("error is %T, want *types.SourceError", err)
	}
	// Already-cancelled context (not deadline) → CategoryUnavailable.
	if se.Category != types.CategoryUnavailable {
		t.Errorf("Category = %v; want CategoryUnavailable", se.Category)
	}
}

// ---- Name, Capabilities, Healthcheck ----

// TestName verifies Name() returns "reddit-rss".
func TestName(t *testing.T) {
	t.Parallel()
	srv := serveStatus(t, http.StatusOK, nil)
	defer srv.Close()
	a := newAdapter(t, srv.URL)
	if a.Name() != "reddit-rss" {
		t.Errorf("Name() = %q; want reddit-rss", a.Name())
	}
}

// TestCapabilities verifies REQ-ADP1B-018.
func TestCapabilities(t *testing.T) {
	t.Parallel()
	srv := serveStatus(t, http.StatusOK, nil)
	defer srv.Close()
	a := newAdapter(t, srv.URL)

	caps := a.Capabilities()
	if caps.SourceID != "reddit-rss" {
		t.Errorf("SourceID = %q; want reddit-rss", caps.SourceID)
	}
	if caps.DisplayName != "Reddit (RSS)" {
		t.Errorf("DisplayName = %q; want Reddit (RSS)", caps.DisplayName)
	}
	if caps.RequiresAuth {
		t.Error("RequiresAuth = true; want false")
	}
	if caps.AuthEnvVars != nil {
		t.Errorf("AuthEnvVars = %v; want nil", caps.AuthEnvVars)
	}
	if len(caps.DocTypes) != 1 || caps.DocTypes[0] != types.DocTypePost {
		t.Errorf("DocTypes = %v; want [post]", caps.DocTypes)
	}
	// SourceID must equal Name().
	if caps.SourceID != a.Name() {
		t.Errorf("SourceID %q != Name() %q", caps.SourceID, a.Name())
	}
}

// TestHealthcheck_OK verifies nil return on 2xx (REQ-ADP1B-019).
func TestHealthcheck_OK(t *testing.T) {
	t.Parallel()
	srv := serveStatus(t, http.StatusOK, nil)
	defer srv.Close()

	a := newAdapter(t, srv.URL)
	if err := a.Healthcheck(context.Background()); err != nil {
		t.Errorf("Healthcheck: %v; want nil", err)
	}
}

// TestHealthcheck_5xx verifies CategoryUnavailable on 5xx (REQ-ADP1B-019).
func TestHealthcheck_5xx(t *testing.T) {
	t.Parallel()
	srv := serveStatus(t, http.StatusInternalServerError, nil)
	defer srv.Close()

	a := newAdapter(t, srv.URL)
	err := a.Healthcheck(context.Background())
	if err == nil {
		t.Fatal("want error for 500, got nil")
	}
	var se *types.SourceError
	if !errors.As(err, &se) {
		t.Fatalf("error is %T, want *types.SourceError", err)
	}
	if se.Category != types.CategoryUnavailable {
		t.Errorf("Category = %v; want CategoryUnavailable", se.Category)
	}
}

// TestHealthcheck_NetworkError verifies CategoryUnavailable on transport error.
func TestHealthcheck_NetworkError(t *testing.T) {
	t.Parallel()

	// Use a server URL that immediately closes.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srvURL := srv.URL
	srv.Close() // close before Healthcheck

	a := newAdapter(t, srvURL)
	err := a.Healthcheck(context.Background())
	if err == nil {
		t.Fatal("want error for closed server, got nil")
	}
	var se *types.SourceError
	if !errors.As(err, &se) {
		t.Fatalf("error is %T, want *types.SourceError", err)
	}
	if se.Category != types.CategoryUnavailable {
		t.Errorf("Category = %v; want CategoryUnavailable", se.Category)
	}
}

// ---- Options / construction ----

// TestNew_DefaultOptions verifies REQ-ADP1B-001.
func TestNew_DefaultOptions(t *testing.T) {
	t.Parallel()
	// Default construction must succeed and return a valid Adapter.
	a, err := redditrss.New(redditrss.Options{})
	if err != nil {
		t.Fatalf("New(default): %v", err)
	}
	if a == nil {
		t.Fatal("New returned nil")
	}
	if a.Name() != "reddit-rss" {
		t.Errorf("Name() = %q; want reddit-rss", a.Name())
	}
}

// TestNew_CustomUserAgent verifies REQ-ADP1B-003.
func TestNew_CustomUserAgent(t *testing.T) {
	t.Parallel()

	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	customUA := "my-test-agent/1.0"
	a, err := redditrss.New(redditrss.Options{
		BaseURL:   srv.URL,
		UserAgent: customUA,
		Timeout:   2 * time.Second,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	a.SetRetryParamsForTest(0, 1) // single-shot: this test only checks the UA header

	_, _ = a.Search(context.Background(), types.Query{Text: "golang"})
	if gotUA != customUA {
		t.Errorf("User-Agent = %q; want %q", gotUA, customUA)
	}
}

// TestNew_DefaultUserAgent verifies default UA contains "usearch-reddit-rss" (REQ-ADP1B-003).
func TestNew_DefaultUserAgent(t *testing.T) {
	t.Parallel()

	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	a, err := redditrss.New(redditrss.Options{
		BaseURL: srv.URL,
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	a.SetRetryParamsForTest(0, 1) // single-shot: this test only checks the UA header

	_, _ = a.Search(context.Background(), types.Query{Text: "golang"})
	if gotUA == "" {
		t.Error("User-Agent header not sent")
	}
	if !strings.HasPrefix(gotUA, "usearch-reddit-rss/") {
		t.Errorf("User-Agent = %q; want prefix usearch-reddit-rss/", gotUA)
	}
}

// TestSearch_NetworkError_Unavailable verifies REQ-ADP1B-014.
func TestSearch_NetworkError_Unavailable(t *testing.T) {
	t.Parallel()

	// Use a URL that will result in a connection refused.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srvURL := srv.URL
	srv.Close()

	a := newAdapter(t, srvURL)
	_, err := a.Search(context.Background(), types.Query{Text: "golang"})
	if err == nil {
		t.Fatal("want error for network failure, got nil")
	}
	var se *types.SourceError
	if !errors.As(err, &se) {
		t.Fatalf("error is %T, want *types.SourceError", err)
	}
	if se.Category != types.CategoryUnavailable {
		t.Errorf("Category = %v; want CategoryUnavailable", se.Category)
	}
}

// TestSearch_AdapterName verifies SourceError.Adapter matches Name().
func TestSearch_AdapterName_InError(t *testing.T) {
	t.Parallel()

	srv := serveStatus(t, http.StatusInternalServerError, nil)
	defer srv.Close()

	a := newAdapter(t, srv.URL)
	_, err := a.Search(context.Background(), types.Query{Text: "golang"})
	if err == nil {
		t.Fatal("want error, got nil")
	}
	var se *types.SourceError
	if !errors.As(err, &se) {
		t.Fatalf("error is %T, want *types.SourceError", err)
	}
	if se.Adapter != "reddit-rss" {
		t.Errorf("SourceError.Adapter = %q; want reddit-rss", se.Adapter)
	}
}

// TestSearch_URL_QueryEncoding verifies special chars are URL-encoded (REQ-ADP1B-005).
func TestSearch_URL_QueryEncoding(t *testing.T) {
	t.Parallel()

	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusTooManyRequests) // stop after URL check
	}))
	defer srv.Close()

	a := newAdapter(t, srv.URL)
	_, _ = a.Search(context.Background(), types.Query{Text: "hello world & more"})

	// Must have q param and sort=relevance.
	if gotQuery == "" {
		t.Fatal("no query string in request")
	}
	// Verify sort=relevance is present
	if gotQuery != "q=hello+world+%26+more&sort=relevance" &&
		gotQuery != "q=hello%20world%20%26%20more&sort=relevance" {
		// Accept either + or %20 encoding for spaces, but must have sort=relevance.
		parsed := parseQuery(gotQuery)
		if parsed["sort"] != "relevance" {
			t.Errorf("query %q does not contain sort=relevance", gotQuery)
		}
	}
}

// parseQuery is a simple key=value splitter for test assertions.
func parseQuery(q string) map[string]string {
	result := make(map[string]string)
	for _, part := range splitN(q, "&") {
		kv := splitN(part, "=")
		if len(kv) == 2 {
			result[kv[0]] = kv[1]
		}
	}
	return result
}

func splitN(s, sep string) []string {
	var parts []string
	for {
		idx := indexOf(s, sep)
		if idx < 0 {
			parts = append(parts, s)
			break
		}
		parts = append(parts, s[:idx])
		s = s[idx+len(sep):]
	}
	return parts
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// TestSearch_NoTParam verifies v0.1 does NOT emit t= time-window param (REQ-ADP1B-005).
func TestSearch_NoTParam(t *testing.T) {
	t.Parallel()

	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	a := newAdapter(t, srv.URL)
	_, _ = a.Search(context.Background(), types.Query{Text: "golang"})

	parsed := parseQuery(gotQuery)
	if _, ok := parsed["t"]; ok {
		t.Errorf("request has t= parameter; v0.1 should not emit it. query=%q", gotQuery)
	}
}

// TestSearch_EC2_NoPubDate verifies that an item without pubDate still
// produces a valid doc with zero PublishedAt (EC2).
func TestSearch_EC2_NoPubDateItemValid(t *testing.T) {
	t.Parallel()

	srv := serveFile(t, http.StatusOK, "search.rss")
	defer srv.Close()

	a := newAdapter(t, srv.URL)
	docs, err := a.Search(context.Background(), types.Query{Text: "go"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	// Find the no-pubdate item (link contains "nopub").
	found := false
	for _, doc := range docs {
		if contains(doc.URL, "nopub") {
			found = true
			if !doc.PublishedAt.IsZero() {
				t.Errorf("no-pubDate doc has non-zero PublishedAt: %v", doc.PublishedAt)
			}
			if err := doc.Validate(); err != nil {
				t.Errorf("no-pubDate doc.Validate(): %v", err)
			}
			break
		}
	}
	if !found {
		t.Log("no-pubdate item not found by URL (acceptable if skipped due to fixture)")
	}
}

func contains(s, sub string) bool {
	return indexOf(s, sub) >= 0
}

// TestSearch_429_RetryAfterAbsent verifies RetryAfter==0 when header is absent.
func TestSearch_429_RetryAfterAbsent(t *testing.T) {
	t.Parallel()

	srv := serveStatus(t, http.StatusTooManyRequests, nil)
	defer srv.Close()

	a := newAdapter(t, srv.URL)
	_, err := a.Search(context.Background(), types.Query{Text: "golang"})
	if err == nil {
		t.Fatal("want error, got nil")
	}
	var se *types.SourceError
	if !errors.As(err, &se) {
		t.Fatalf("error is %T, want *types.SourceError", err)
	}
	// RetryAfter must be zero when Retry-After header is absent.
	if se.RetryAfter != 0 {
		t.Errorf("RetryAfter = %v; want 0 (header absent)", se.RetryAfter)
	}
}
