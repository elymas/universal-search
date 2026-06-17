// Package github — search integration tests against httptest stubs.
// Tests #9–25, 41–55, 60–62: REQ-ADP4-002..010, NFR-ADP4-002..004.
package github

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	gogithub "github.com/google/go-github/v73/github"

	"github.com/elymas/universal-search/pkg/types"
)

// TestSearchCodeIntentHappyPath25Hits verifies 25 NormalizedDocs from /search/code.
func TestSearchCodeIntentHappyPath25Hits(t *testing.T) {
	t.Parallel()
	srv := newCodeStubServer(t, "search_code_25.json", 0)
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	docs, err := a.Search(context.Background(), testQuery("adapter", "code", 25, ""))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(docs) != 25 {
		t.Errorf("expected 25 docs, got %d", len(docs))
	}
}

// TestSearchIssuesIntentHappyPath25Hits verifies 25 NormalizedDocs from /search/issues.
func TestSearchIssuesIntentHappyPath25Hits(t *testing.T) {
	t.Parallel()
	srv := newIssueStubServer(t, "search_issues_25.json", 0)
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	docs, err := a.Search(context.Background(), testQuery("goroutine", "issues", 25, ""))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(docs) != 25 {
		t.Errorf("expected 25 docs, got %d", len(docs))
	}
}

// TestSearchReposIntentHappyPath25Hits verifies 25 NormalizedDocs from /search/repositories.
func TestSearchReposIntentHappyPath25Hits(t *testing.T) {
	t.Parallel()
	srv := newRepoStubServer(t, "search_repos_25.json", 0)
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	docs, err := a.Search(context.Background(), testQuery("golang", "repos", 25, ""))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(docs) != 25 {
		t.Errorf("expected 25 docs, got %d", len(docs))
	}
}

// TestSearchDefaultIntentIsRepos verifies that when no kind filter is set,
// the adapter calls /search/repositories.
func TestSearchDefaultIntentIsRepos(t *testing.T) {
	t.Parallel()
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		writeJSONFile(w, "testdata/search_repos_25.json")
	}))
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	q := types.Query{Text: "golang", MaxResults: 5}
	_, err := a.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if !strings.Contains(capturedPath, "repositories") {
		t.Errorf("expected path to contain 'repositories', got %q", capturedPath)
	}
}

// TestSearchClampsPerPageTo100 verifies that MaxResults > 100 is clamped to 100.
func TestSearchClampsPerPageTo100(t *testing.T) {
	t.Parallel()
	var capturedPerPage string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPerPage = r.URL.Query().Get("per_page")
		writeJSONFile(w, "testdata/search_repos_25.json")
	}))
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	_, err := a.Search(context.Background(), testQuery("golang", "repos", 500, ""))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if capturedPerPage != "100" {
		t.Errorf("per_page = %q, want 100 (clamped)", capturedPerPage)
	}
}

// TestSearchDefaultsPerPageTo25 verifies that MaxResults == 0 defaults to per_page=25.
func TestSearchDefaultsPerPageTo25(t *testing.T) {
	t.Parallel()
	var capturedPerPage string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPerPage = r.URL.Query().Get("per_page")
		writeJSONFile(w, "testdata/search_repos_25.json")
	}))
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	_, err := a.Search(context.Background(), testQuery("golang", "repos", 0, ""))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if capturedPerPage != "25" {
		t.Errorf("per_page = %q, want 25 (default)", capturedPerPage)
	}
}

// TestSearchSetsPageWhenCursorPresent verifies that Cursor="3" → page=3.
func TestSearchSetsPageWhenCursorPresent(t *testing.T) {
	t.Parallel()
	var capturedPage string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPage = r.URL.Query().Get("page")
		writeJSONFile(w, "testdata/search_repos_25.json")
	}))
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	_, err := a.Search(context.Background(), testQuery("golang", "repos", 25, "3"))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if capturedPage != "3" {
		t.Errorf("page = %q, want 3", capturedPage)
	}
}

// TestSearchSetsPage1WhenCursorEmpty verifies that empty Cursor → page=1.
func TestSearchSetsPage1WhenCursorEmpty(t *testing.T) {
	t.Parallel()
	var capturedPage string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPage = r.URL.Query().Get("page")
		writeJSONFile(w, "testdata/search_repos_25.json")
	}))
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	_, err := a.Search(context.Background(), testQuery("golang", "repos", 25, ""))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if capturedPage != "1" && capturedPage != "" {
		t.Errorf("page = %q, want 1 or empty (defaults to 1)", capturedPage)
	}
}

// TestSearchPrimaryRateLimitMapsToCategory verifies 403 + ratelimit → CategoryRateLimited.
func TestSearchPrimaryRateLimitMapsToCategory(t *testing.T) {
	t.Parallel()
	srv := newRateLimitStub(t, false)
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	_, err := a.Search(context.Background(), testQuery("golang", "repos", 5, ""))
	if err == nil {
		t.Fatal("expected rate limit error, got nil")
	}
	assertSourceErrorCategory(t, err, types.CategoryRateLimited)
}

// TestSearchAbuseRateLimitMapsToCategory verifies abuse detection → CategoryRateLimited.
func TestSearchAbuseRateLimitMapsToCategory(t *testing.T) {
	t.Parallel()
	srv := newRateLimitStub(t, true)
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	_, err := a.Search(context.Background(), testQuery("golang", "repos", 5, ""))
	if err == nil {
		t.Fatal("expected abuse rate limit error, got nil")
	}
	assertSourceErrorCategory(t, err, types.CategoryRateLimited)
}

// TestSearchRateLimitRetryAfterCapped90s verifies Retry-After=999 → capped to 90s.
func TestSearchRateLimitRetryAfterCapped90s(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return 403 with X-RateLimit-Remaining=0 and huge Retry-After.
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(999*time.Second).Unix(), 10))
		w.Header().Set("Retry-After", "999")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"message":"API rate limit exceeded"}`)
	}))
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	_, err := a.Search(context.Background(), testQuery("golang", "repos", 5, ""))
	if err == nil {
		t.Fatal("expected rate limit error, got nil")
	}
	se := mustSourceError(t, err)
	if se.RetryAfter > maxRetryAfter {
		t.Errorf("RetryAfter = %v, want ≤ %v", se.RetryAfter, maxRetryAfter)
	}
}

// TestSearchRateLimitNegativeOrZeroDefaults5s verifies that a past/zero Reset
// time results in defaultRetryAfter (5s).
func TestSearchRateLimitNegativeOrZeroDefaults5s(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		// Reset is in the past.
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(-60*time.Second).Unix(), 10))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"message":"API rate limit exceeded"}`)
	}))
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	_, err := a.Search(context.Background(), testQuery("golang", "repos", 5, ""))
	if err == nil {
		t.Fatal("expected rate limit error, got nil")
	}
	se := mustSourceError(t, err)
	if se.RetryAfter < defaultRetryAfter {
		t.Errorf("RetryAfter = %v, want ≥ %v (default)", se.RetryAfter, defaultRetryAfter)
	}
}

// TestSearchRateLimitNoInternalRetry verifies exactly 1 outbound request on rate limit.
func TestSearchRateLimitNoInternalRetry(t *testing.T) {
	t.Parallel()
	var requestCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(30*time.Second).Unix(), 10))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"message":"API rate limit exceeded"}`)
	}))
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	_, _ = a.Search(context.Background(), testQuery("golang", "repos", 5, ""))

	if n := atomic.LoadInt32(&requestCount); n != 1 {
		t.Errorf("outbound requests = %d, want 1 (no internal retry)", n)
	}
}

// TestSearchHTTP4xx verifies 4xx status codes map to CategoryPermanent.
func TestSearchHTTP4xx(t *testing.T) {
	t.Parallel()
	cases := []int{401, 403, 404, 422}
	for _, code := range cases {
		code := code
		t.Run(fmt.Sprintf("HTTP%d", code), func(t *testing.T) {
			t.Parallel()
			srv := newStatusStub(t, code, `{"message":"error"}`)
			defer srv.Close()
			a := newTestAdapter(t, srv.URL)

			_, err := a.Search(context.Background(), testQuery("golang", "repos", 5, ""))
			if err == nil {
				t.Fatalf("expected error for %d, got nil", code)
			}
			se := mustSourceError(t, err)
			if se.Category != types.CategoryPermanent {
				t.Errorf("HTTP %d: Category = %v, want CategoryPermanent", code, se.Category)
			}
			if se.HTTPStatus != code {
				t.Errorf("HTTP %d: HTTPStatus = %d, want %d", code, se.HTTPStatus, code)
			}
		})
	}
}

// TestSearchHTTP5xx verifies 5xx status codes map to CategoryUnavailable.
func TestSearchHTTP5xx(t *testing.T) {
	t.Parallel()
	cases := []int{500, 503}
	for _, code := range cases {
		code := code
		t.Run(fmt.Sprintf("HTTP%d", code), func(t *testing.T) {
			t.Parallel()
			srv := newStatusStub(t, code, `{"message":"server error"}`)
			defer srv.Close()
			a := newTestAdapter(t, srv.URL)

			_, err := a.Search(context.Background(), testQuery("golang", "repos", 5, ""))
			if err == nil {
				t.Fatalf("expected error for %d, got nil", code)
			}
			se := mustSourceError(t, err)
			if se.Category != types.CategoryUnavailable {
				t.Errorf("HTTP %d: Category = %v, want CategoryUnavailable", code, se.Category)
			}
			if se.HTTPStatus != code {
				t.Errorf("HTTP %d: HTTPStatus = %d, want %d", code, se.HTTPStatus, code)
			}
		})
	}
}

// TestSearchConnectionRefused verifies connection refused → CategoryUnavailable, HTTPStatus=0.
func TestSearchConnectionRefused(t *testing.T) {
	t.Parallel()
	a, err := New(Options{
		BaseURL:       "http://127.0.0.1:1/", // Port 1 should be connection refused.
		SkipAuthCheck: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, serr := a.Search(context.Background(), testQuery("golang", "repos", 5, ""))
	if serr == nil {
		t.Fatal("expected error for connection refused, got nil")
	}
	se := mustSourceError(t, serr)
	if se.Category != types.CategoryUnavailable {
		t.Errorf("Category = %v, want CategoryUnavailable", se.Category)
	}
	if se.HTTPStatus != 0 {
		t.Errorf("HTTPStatus = %d, want 0 for network error", se.HTTPStatus)
	}
}

// TestSearchUnavailablePreservesUnderlyingError verifies Cause is preserved.
func TestSearchUnavailablePreservesUnderlyingError(t *testing.T) {
	t.Parallel()
	a, err := New(Options{
		BaseURL:       "http://127.0.0.1:1/",
		SkipAuthCheck: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, serr := a.Search(context.Background(), testQuery("golang", "repos", 5, ""))
	if serr == nil {
		t.Fatal("expected error, got nil")
	}
	se := mustSourceError(t, serr)
	if se.Cause == nil {
		t.Error("Cause should not be nil for network error")
	}
}

// TestSearchSinceFilterAddsCreatedQualifier verifies since filter → created:>=<RFC3339>.
func TestSearchSinceFilterAddsCreatedQualifier(t *testing.T) {
	t.Parallel()
	var capturedQ string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQ = r.URL.Query().Get("q")
		writeJSONFile(w, "testdata/search_repos_25.json")
	}))
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	q := types.Query{
		Text:       "golang",
		MaxResults: 5,
		Filters:    []types.Filter{{Key: "since", Value: "2026-01-01T00:00:00Z"}},
	}
	_, err := a.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if !strings.Contains(capturedQ, "created:>=2026-01-01T00:00:00Z") {
		t.Errorf("q = %q, want to contain 'created:>=2026-01-01T00:00:00Z'", capturedQ)
	}
}

// TestSearchLanguageFilterAddsLanguageQualifier verifies language filter → language:<value>.
func TestSearchLanguageFilterAddsLanguageQualifier(t *testing.T) {
	t.Parallel()
	var capturedQ string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQ = r.URL.Query().Get("q")
		writeJSONFile(w, "testdata/search_repos_25.json")
	}))
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	q := types.Query{
		Text:       "search",
		MaxResults: 5,
		Filters:    []types.Filter{{Key: "language", Value: "go"}},
	}
	_, err := a.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if !strings.Contains(capturedQ, "language:go") {
		t.Errorf("q = %q, want to contain 'language:go'", capturedQ)
	}
}

// TestSearchRepoFilterAddsRepoQualifier verifies repo filter → repo:owner/name.
func TestSearchRepoFilterAddsRepoQualifier(t *testing.T) {
	t.Parallel()
	var capturedQ string
	srv := newIssueStubServerCapture(t, "search_issues_25.json", 0, &capturedQ)
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	q := types.Query{
		Text:       "bug",
		MaxResults: 5,
		Filters: []types.Filter{
			{Key: "kind", Value: "issues"},
			{Key: "repo", Value: "golang/go"},
		},
	}
	_, err := a.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if !strings.Contains(capturedQ, "repo:golang/go") {
		t.Errorf("q = %q, want to contain 'repo:golang/go'", capturedQ)
	}
}

// TestSearchMultipleFiltersJoinedBySpace verifies two filters → single space separator.
func TestSearchMultipleFiltersJoinedBySpace(t *testing.T) {
	t.Parallel()
	var capturedQ string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQ = r.URL.Query().Get("q")
		writeJSONFile(w, "testdata/search_repos_25.json")
	}))
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	q := types.Query{
		Text:       "go",
		MaxResults: 5,
		Filters: []types.Filter{
			{Key: "language", Value: "go"},
			{Key: "org", Value: "golang"},
		},
	}
	_, err := a.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	// Both qualifiers must be in q, separated by space (not +).
	if !strings.Contains(capturedQ, "language:go") || !strings.Contains(capturedQ, "org:golang") {
		t.Errorf("q = %q, want both language:go and org:golang", capturedQ)
	}
	// No "+" separator — GitHub search uses space.
	if strings.Contains(capturedQ, "+language") || strings.Contains(capturedQ, "+org") {
		t.Errorf("q = %q, must not contain '+' as separator", capturedQ)
	}
}

// TestSearchUnknownFilterIgnored verifies unknown filter key → no qualifier append.
func TestSearchUnknownFilterIgnored(t *testing.T) {
	t.Parallel()
	var capturedQ string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQ = r.URL.Query().Get("q")
		writeJSONFile(w, "testdata/search_repos_25.json")
	}))
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	q := types.Query{
		Text:       "golang",
		MaxResults: 5,
		Filters:    []types.Filter{{Key: "unknown_key", Value: "some_value"}},
	}
	_, err := a.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	// The unknown key should not appear in q.
	if strings.Contains(capturedQ, "unknown_key") || strings.Contains(capturedQ, "some_value") {
		t.Errorf("q = %q, should not contain unknown filter", capturedQ)
	}
}

// TestSearchMalformedSinceDropped verifies bad RFC 3339 → no qualifier append.
func TestSearchMalformedSinceDropped(t *testing.T) {
	t.Parallel()
	var capturedQ string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQ = r.URL.Query().Get("q")
		writeJSONFile(w, "testdata/search_repos_25.json")
	}))
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	q := types.Query{
		Text:       "golang",
		MaxResults: 5,
		Filters:    []types.Filter{{Key: "since", Value: "not-a-date"}},
	}
	_, err := a.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if strings.Contains(capturedQ, "created:") {
		t.Errorf("q = %q, malformed since should not produce created: qualifier", capturedQ)
	}
}

// TestSearchEmptyFilterValueDropped verifies empty Value → no qualifier append.
func TestSearchEmptyFilterValueDropped(t *testing.T) {
	t.Parallel()
	var capturedQ string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQ = r.URL.Query().Get("q")
		writeJSONFile(w, "testdata/search_repos_25.json")
	}))
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	q := types.Query{
		Text:       "golang",
		MaxResults: 5,
		Filters:    []types.Filter{{Key: "language", Value: ""}},
	}
	_, err := a.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if strings.Contains(capturedQ, "language:") {
		t.Errorf("q = %q, empty value should not produce language: qualifier", capturedQ)
	}
}

// TestSearchTopicFilterAddsTopicQualifier verifies topic filter → topic:<value> for repos intent;
// silently dropped for code and issues intents.
func TestSearchTopicFilterAddsTopicQualifier(t *testing.T) {
	t.Parallel()
	var capturedQ string
	srv := newRepoStubServerCapture(t, "search_repos_25.json", 0, &capturedQ)
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	_, err := a.Search(context.Background(), types.Query{
		Text:       "kubernetes",
		MaxResults: 5,
		Filters:    []types.Filter{{Key: "topic", Value: "cncf"}},
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if !strings.Contains(capturedQ, "topic:cncf") {
		t.Errorf("q = %q, want topic:cncf qualifier", capturedQ)
	}
}

// TestSearchStateFilterAddsStateQualifier verifies state filter → state:open/closed for issues.
func TestSearchStateFilterAddsStateQualifier(t *testing.T) {
	t.Parallel()
	var capturedQ string
	srv := newIssueStubServerCapture(t, "search_issues_25.json", 0, &capturedQ)
	defer srv.Close()
	a := newTestAdapterWithURL(t, srv.URL)

	_, err := a.Search(context.Background(), types.Query{
		Text:       "bug",
		MaxResults: 5,
		Filters: []types.Filter{
			{Key: "kind", Value: "issues"},
			{Key: "state", Value: "open"},
		},
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if !strings.Contains(capturedQ, "state:open") {
		t.Errorf("q = %q, want state:open qualifier", capturedQ)
	}
}

// TestSearchUserFilterAppendsUser verifies user filter appended across all intents.
func TestSearchUserFilterAppendsUser(t *testing.T) {
	t.Parallel()
	var capturedQ string
	srv := newRepoStubServerCapture(t, "search_repos_25.json", 0, &capturedQ)
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	_, err := a.Search(context.Background(), types.Query{
		Text:       "cli",
		MaxResults: 5,
		Filters:    []types.Filter{{Key: "user", Value: "torvalds"}},
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if !strings.Contains(capturedQ, "user:torvalds") {
		t.Errorf("q = %q, want user:torvalds qualifier", capturedQ)
	}
}

// TestSearchStateInvalidValueDropped verifies state filter with invalid value (not open/closed) is dropped.
func TestSearchStateInvalidValueDropped(t *testing.T) {
	t.Parallel()
	var capturedQ string
	srv := newIssueStubServerCapture(t, "search_issues_25.json", 0, &capturedQ)
	defer srv.Close()
	a := newTestAdapterWithURL(t, srv.URL)

	_, err := a.Search(context.Background(), types.Query{
		Text:       "bug",
		MaxResults: 5,
		Filters: []types.Filter{
			{Key: "kind", Value: "issues"},
			{Key: "state", Value: "pending"},
		},
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if strings.Contains(capturedQ, "state:") {
		t.Errorf("q = %q, invalid state value should be dropped", capturedQ)
	}
}

// TestSearchFiltersOnlyAppendForApplicableIntent verifies is_pr ignored on code intent;
// applied on issues intent.
func TestSearchFiltersOnlyAppendForApplicableIntent(t *testing.T) {
	t.Parallel()

	var codeQ, issueQ string
	codeSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		codeQ = r.URL.Query().Get("q")
		writeJSONFile(w, "testdata/search_code_25.json")
	}))
	defer codeSrv.Close()

	issueSrv := newIssueStubServerCapture(t, "search_issues_25.json", 0, &issueQ)
	defer issueSrv.Close()

	codeAdapter := newTestAdapterWithURL(t, codeSrv.URL)
	issueAdapter := newTestAdapterWithURL(t, issueSrv.URL)

	filter := types.Filter{Key: "is_pr", Value: "true"}

	// Code intent: is_pr should be ignored.
	_, err := codeAdapter.Search(context.Background(), types.Query{
		Text: "search", MaxResults: 5,
		Filters: []types.Filter{{Key: "kind", Value: "code"}, filter},
	})
	if err != nil {
		t.Fatalf("code search: %v", err)
	}
	if strings.Contains(codeQ, "is:pr") {
		t.Errorf("code intent: q = %q, should not contain 'is:pr'", codeQ)
	}

	// Issues intent: is_pr should be applied.
	_, err = issueAdapter.Search(context.Background(), types.Query{
		Text: "search", MaxResults: 5,
		Filters: []types.Filter{{Key: "kind", Value: "issues"}, filter},
	})
	if err != nil {
		t.Fatalf("issues search: %v", err)
	}
	if !strings.Contains(issueQ, "is:pr") {
		t.Errorf("issues intent: q = %q, should contain 'is:pr'", issueQ)
	}
}

// TestSearchEmptyQueryRejectedNoHTTP verifies empty/whitespace query → ErrPermanent,
// zero HTTP requests.
func TestSearchEmptyQueryRejectedNoHTTP(t *testing.T) {
	t.Parallel()
	var requestCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		writeJSONFile(w, "testdata/search_repos_25.json")
	}))
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	queries := []string{"", "   ", "\t", "\n", "  \t  \n  "}
	for _, qtext := range queries {
		_, err := a.Search(context.Background(), types.Query{Text: qtext, MaxResults: 5})
		if err == nil {
			t.Errorf("query %q: expected error, got nil", qtext)
			continue
		}
		se := mustSourceError(t, err)
		if se.Category != types.CategoryPermanent {
			t.Errorf("query %q: Category = %v, want CategoryPermanent", qtext, se.Category)
		}
	}
	if n := atomic.LoadInt32(&requestCount); n != 0 {
		t.Errorf("outbound requests = %d, want 0 (validation before network)", n)
	}
}

// TestSearchInvalidCursorRejectedNoHTTP verifies invalid cursor → ErrPermanent,
// zero requests.
func TestSearchInvalidCursorRejectedNoHTTP(t *testing.T) {
	t.Parallel()
	var requestCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		writeJSONFile(w, "testdata/search_repos_25.json")
	}))
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	invalidCursors := []string{"abc", "-1", "0", "1.5", "two"}
	for _, cursor := range invalidCursors {
		_, err := a.Search(context.Background(), testQuery("golang", "repos", 5, cursor))
		if err == nil {
			t.Errorf("cursor %q: expected error, got nil", cursor)
			continue
		}
		se := mustSourceError(t, err)
		if se.Category != types.CategoryPermanent {
			t.Errorf("cursor %q: Category = %v, want CategoryPermanent", cursor, se.Category)
		}
	}
	if n := atomic.LoadInt32(&requestCount); n != 0 {
		t.Errorf("outbound requests = %d, want 0", n)
	}
}

// TestSearchInvalidIntentRejectedNoHTTP verifies kind="users" → ErrInvalidIntent,
// zero requests.
func TestSearchInvalidIntentRejectedNoHTTP(t *testing.T) {
	t.Parallel()
	var requestCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		writeJSONFile(w, "testdata/search_repos_25.json")
	}))
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	_, err := a.Search(context.Background(), types.Query{
		Text:       "golang",
		MaxResults: 5,
		Filters:    []types.Filter{{Key: "kind", Value: "users"}},
	})
	if err == nil {
		t.Fatal("expected error for invalid intent, got nil")
	}
	se := mustSourceError(t, err)
	if se.Category != types.CategoryPermanent {
		t.Errorf("Category = %v, want CategoryPermanent", se.Category)
	}
	if n := atomic.LoadInt32(&requestCount); n != 0 {
		t.Errorf("outbound requests = %d, want 0", n)
	}
}

// TestSearchConcurrentSafe verifies 50 concurrent goroutines × 1 stub;
// race-clean; 50 requests; valid docs (REQ-ADP4-010).
func TestSearchConcurrentSafe(t *testing.T) {
	t.Parallel()
	var requestCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		writeJSONFile(w, "testdata/search_repos_25.json")
	}))
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	const goroutines = 50
	errs := make([]error, goroutines)
	docCounts := make([]int, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := range goroutines {
		go func(idx int) {
			defer wg.Done()
			docs, err := a.Search(context.Background(), testQuery("golang", "repos", 25, ""))
			errs[idx] = err
			if docs != nil {
				docCounts[idx] = len(docs)
			}
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: Search error: %v", i, err)
		}
	}
	for i, count := range docCounts {
		if count != 25 {
			t.Errorf("goroutine %d: expected 25 docs, got %d", i, count)
		}
	}
	if n := atomic.LoadInt32(&requestCount); n != goroutines {
		t.Errorf("requests = %d, want %d", n, goroutines)
	}
}

// TestSearchE2ELatencyStubP95 verifies that p95 latency across 100 invocations
// against a local stub is ≤ 200ms. (NFR-ADP4-002)
func TestSearchE2ELatencyStubP95(t *testing.T) {
	t.Parallel()
	srv := newRepoStubServer(t, "search_repos_25.json", 0)
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	const iterations = 100
	latencies := make([]time.Duration, iterations)
	for i := range iterations {
		start := time.Now()
		_, err := a.Search(context.Background(), testQuery("golang", "repos", 25, ""))
		latencies[i] = time.Since(start)
		if err != nil {
			t.Fatalf("Search[%d]: %v", i, err)
		}
	}

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	p95 := latencies[int(float64(iterations)*0.95)]
	if p95 > 200*time.Millisecond {
		t.Errorf("p95 latency = %v, want ≤ 200ms", p95)
	}
}

// TestSearchNoGoroutineLeakOnCancel verifies no goroutine leak on mid-flight
// context cancel. (NFR-ADP4-003)
func TestSearchNoGoroutineLeakOnCancel(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		// Slow response — context cancellation should abort.
		select {
		case <-r.Context().Done():
			return
		case <-time.After(5 * time.Second):
			writeJSONFile(w, "testdata/search_repos_25.json")
		}
	}))
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := a.Search(ctx, testQuery("golang", "repos", 5, ""))
		done <- err
	}()

	// Wait for request to start, then cancel.
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Log("server not started; cancelling anyway")
	}
	cancel()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Search did not return after context cancel")
	}

	// Give any cleanup goroutines time to exit.
	// Package-level leak detection is handled by goleak.VerifyTestMain in TestMain.
	time.Sleep(100 * time.Millisecond)
}

// TestSearchNoLeakedFileDescriptors verifies FD delta ≤ 5 over 100 calls.
// (NFR-ADP4-004)
func TestSearchNoLeakedFileDescriptors(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("FD leak test only on linux/darwin")
	}

	srv := newRepoStubServer(t, "search_repos_25.json", 0)
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	// Warm up.
	for range 5 {
		_, _ = a.Search(context.Background(), testQuery("golang", "repos", 5, ""))
	}

	fdBefore := countFDs(t)
	for range 100 {
		_, _ = a.Search(context.Background(), testQuery("golang", "repos", 5, ""))
	}
	fdAfter := countFDs(t)

	delta := fdAfter - fdBefore
	if delta > 5 {
		t.Errorf("FD delta = %d after 100 calls, want ≤ 5", delta)
	}
}

// --- helpers ---

// testQuery builds a Query for test use.
func testQuery(text, intent string, maxResults int, cursor string) types.Query {
	q := types.Query{
		Text:       text,
		MaxResults: maxResults,
		Cursor:     cursor,
	}
	if intent != "" {
		q.Filters = append(q.Filters, types.Filter{Key: "kind", Value: intent})
	}
	return q
}

// newTestAdapter creates an Adapter pointed at url (no token required).
func newTestAdapter(tb testing.TB, rawURL string) *Adapter {
	tb.Helper()
	return newTestAdapterWithURL(tb, rawURL)
}

// newTestAdapterWithURL is an alias for flexibility.
func newTestAdapterWithURL(tb testing.TB, rawURL string) *Adapter {
	tb.Helper()
	if !strings.HasSuffix(rawURL, "/") {
		rawURL += "/"
	}
	a, err := New(Options{
		BaseURL:       rawURL,
		SkipAuthCheck: true,
	})
	if err != nil {
		tb.Fatalf("New: %v", err)
	}
	return a
}

// newRepoStubServer creates a stub server serving repository search results.
// nextPage > 0 adds a Link header indicating a subsequent page.
func newRepoStubServer(tb testing.TB, fixture string, nextPage int) *httptest.Server {
	tb.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if nextPage > 0 {
			// Add pagination Link header.
			linkURL := fmt.Sprintf("%s%s?q=%s&page=%d&per_page=%s",
				"http://"+r.Host, r.URL.Path,
				r.URL.Query().Get("q"),
				nextPage,
				r.URL.Query().Get("per_page"),
			)
			w.Header().Set("Link", fmt.Sprintf(`<%s>; rel="next"`, linkURL))
		}
		writeJSONFile(w, "testdata/"+fixture)
	}))
}

// newCodeStubServer creates a stub server serving code search results.
func newCodeStubServer(tb testing.TB, fixture string, nextPage int) *httptest.Server {
	tb.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if nextPage > 0 {
			linkURL := fmt.Sprintf("%s%s?q=%s&page=%d&per_page=%s",
				"http://"+r.Host, r.URL.Path,
				r.URL.Query().Get("q"),
				nextPage,
				r.URL.Query().Get("per_page"),
			)
			w.Header().Set("Link", fmt.Sprintf(`<%s>; rel="next"`, linkURL))
		}
		writeJSONFile(w, "testdata/"+fixture)
	}))
}

// newIssueStubServer creates a stub server serving issue search results.
func newIssueStubServer(tb testing.TB, fixture string, nextPage int) *httptest.Server {
	tb.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if nextPage > 0 {
			linkURL := fmt.Sprintf("%s%s?q=%s&page=%d&per_page=%s",
				"http://"+r.Host, r.URL.Path,
				r.URL.Query().Get("q"),
				nextPage,
				r.URL.Query().Get("per_page"),
			)
			w.Header().Set("Link", fmt.Sprintf(`<%s>; rel="next"`, linkURL))
		}
		writeJSONFile(w, "testdata/"+fixture)
	}))
}

// newRepoStubServerCapture creates a stub server serving repo results, capturing q.
func newRepoStubServerCapture(tb testing.TB, fixture string, nextPage int, capturedQ *string) *httptest.Server {
	tb.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*capturedQ = r.URL.Query().Get("q")
		writeJSONFile(w, "testdata/"+fixture)
	}))
}

// newIssueStubServerCapture captures the q query parameter.
func newIssueStubServerCapture(tb testing.TB, fixture string, nextPage int, capturedQ *string) *httptest.Server {
	tb.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*capturedQ = r.URL.Query().Get("q")
		writeJSONFile(w, "testdata/"+fixture)
	}))
}

// newStatusStub creates a stub server returning the given status code with body.
func newStatusStub(tb testing.TB, status int, body string) *httptest.Server {
	tb.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}))
}

// newRateLimitStub creates a stub that returns 403 indicating rate limit.
// If abuse is true, it returns an abuse rate limit response.
func newRateLimitStub(tb testing.TB, abuse bool) *httptest.Server {
	tb.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if abuse {
			// Abuse rate limit: 403 with specific documentation URL.
			w.Header().Set("Retry-After", "30")
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, `{"message":"You have exceeded a secondary rate limit","documentation_url":"https://docs.github.com/rest/overview/rate-limits-for-the-rest-api#about-secondary-rate-limits"}`)
		} else {
			// Primary rate limit: 403 with X-RateLimit-Remaining=0.
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(30*time.Second).Unix(), 10))
			w.Header().Set("X-RateLimit-Limit", "5000")
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, `{"message":"API rate limit exceeded","documentation_url":"https://docs.github.com/rest/overview/resources-in-the-rest-api#rate-limiting"}`)
		}
	}))
}

// writeJSONFile writes a test fixture file as an HTTP response.
func writeJSONFile(w http.ResponseWriter, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		http.Error(w, "fixture not found: "+path, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// assertSourceErrorCategory checks that err is a *types.SourceError with the
// expected category.
func assertSourceErrorCategory(t testing.TB, err error, want types.Category) {
	t.Helper()
	se := mustSourceError(t, err)
	if se.Category != want {
		t.Errorf("Category = %v, want %v", se.Category, want)
	}
}

// mustSourceError extracts the *types.SourceError from err or fails.
func mustSourceError(t testing.TB, err error) *types.SourceError {
	t.Helper()
	se, ok := err.(*types.SourceError)
	if !ok {
		t.Fatalf("expected *types.SourceError, got %T: %v", err, err)
	}
	return se
}

// countFDs counts open file descriptors on darwin/linux.
func countFDs(t *testing.T) int {
	t.Helper()
	dir := fmt.Sprintf("/proc/%d/fd", os.Getpid())
	if runtime.GOOS == "darwin" {
		dir = "/dev/fd"
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Logf("countFDs: %v", err)
		return 0
	}
	return len(entries)
}

// Ensure gogithub package is used in the test file.
var _ = gogithub.NewClient

// --- REQ-ADP4a-001 / 004 / 005: commit search tests ---

// TestSearchCommitIntentHappyPath verifies kind=commit routes to /search/commits,
// returns N docs that all pass Validate, and honours per_page/page rules
// (REQ-ADP4a-001 / AC-001..003).
func TestSearchCommitIntentHappyPath(t *testing.T) {
	t.Parallel()
	var capturedPath, capturedPerPage, capturedPage string
	var requestCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		capturedPath = r.URL.Path
		capturedPerPage = r.URL.Query().Get("per_page")
		capturedPage = r.URL.Query().Get("page")
		writeJSONFile(w, "testdata/search_commits_response.json")
	}))
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	docs, err := a.Search(context.Background(), testQuery("fix bug", "commit", 500, ""))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if got, want := len(docs), 3; got != want {
		t.Errorf("doc count = %d, want %d (fixture items)", got, want)
	}
	if capturedPath != "/search/commits" {
		t.Errorf("observed path = %q, want /search/commits", capturedPath)
	}
	if capturedPerPage != "100" {
		t.Errorf("per_page = %q, want 100 (MaxResults=500 clamped)", capturedPerPage)
	}
	for i, d := range docs {
		if err := d.Validate(); err != nil {
			t.Errorf("doc[%d] Validate: %v", i, err)
		}
	}
	if n := atomic.LoadInt32(&requestCount); n != 1 {
		t.Errorf("requests = %d, want 1", n)
	}

	// Cursor -> page translation.
	_, err = a.Search(context.Background(), testQuery("fix bug", "commit", 25, "3"))
	if err != nil {
		t.Fatalf("Search with cursor: %v", err)
	}
	if capturedPage != "3" {
		t.Errorf("page with Cursor=3 = %q, want 3", capturedPage)
	}
}

// TestSearchCommitRateLimited verifies that a 403 + rate-limit headers under
// kind=commit maps to CategoryRateLimited via the reused categorizeError
// rosetta (REQ-ADP4a-004 / AC-010).
func TestSearchCommitRateLimited(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(30*time.Second).Unix(), 10))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"message":"API rate limit exceeded"}`)
	}))
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	_, err := a.Search(context.Background(), testQuery("fix bug", "commit", 5, ""))
	if err == nil {
		t.Fatal("expected rate-limit error, got nil")
	}
	se := mustSourceError(t, err)
	if se.Category != types.CategoryRateLimited {
		t.Errorf("Category = %v, want CategoryRateLimited", se.Category)
	}
	if se.RetryAfter <= 0 {
		t.Errorf("RetryAfter = %v, want > 0", se.RetryAfter)
	}
	if se.RetryAfter > maxRetryAfter {
		t.Errorf("RetryAfter = %v, want <= %v (90s cap)", se.RetryAfter, maxRetryAfter)
	}
}

// TestSearchCommitIntentAcceptedNotRejected verifies kind=commit is accepted
// (not ErrInvalidIntent) and the stub observes >=1 request (REQ-ADP4a-005 / AC-012).
func TestSearchCommitIntentAcceptedNotRejected(t *testing.T) {
	t.Parallel()
	var requestCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		writeJSONFile(w, "testdata/search_commits_response.json")
	}))
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	docs, err := a.Search(context.Background(), testQuery("fix bug", "commit", 5, ""))
	if err != nil {
		se, ok := err.(*types.SourceError)
		if ok && se.Cause == ErrInvalidIntent {
			t.Fatalf("kind=commit must be accepted, got ErrInvalidIntent: %v", err)
		}
		// A different error is acceptable only if the stub failed; we treat any
		// non-ErrInvalidIntent error as acceptable for this test's purpose, but
		// flag unexpected failures.
		if !ok {
			t.Fatalf("Search: unexpected error: %v", err)
		}
	}
	if len(docs) == 0 {
		t.Error("expected at least 1 doc for kind=commit")
	}
	if n := atomic.LoadInt32(&requestCount); n < 1 {
		t.Errorf("requests = %d, want >= 1 (commit search must be issued)", n)
	}
}

// TestSearchInvalidIntentUpdatedMessage verifies kind=users is still rejected
// with ErrInvalidIntent, zero requests, and the updated message contains
// "commit" (REQ-ADP4a-005 / AC-011, AC-013).
func TestSearchInvalidIntentUpdatedMessage(t *testing.T) {
	t.Parallel()
	var requestCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		writeJSONFile(w, "testdata/search_commits_response.json")
	}))
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	_, err := a.Search(context.Background(), types.Query{
		Text:       "golang",
		MaxResults: 5,
		Filters:    []types.Filter{{Key: "kind", Value: "users"}},
	})
	if err == nil {
		t.Fatal("expected error for invalid intent, got nil")
	}
	se := mustSourceError(t, err)
	if se.Category != types.CategoryPermanent {
		t.Errorf("Category = %v, want CategoryPermanent", se.Category)
	}
	if se.Cause != ErrInvalidIntent {
		t.Errorf("Cause = %v, want ErrInvalidIntent", se.Cause)
	}
	// The updated message must enumerate "commit".
	if !strings.Contains(err.Error(), "commit") {
		t.Errorf("error message = %q, want it to contain substring \"commit\"", err.Error())
	}
	if !strings.Contains(ErrInvalidIntent.Error(), "commit") {
		t.Errorf("ErrInvalidIntent.Error() = %q, want it to contain \"commit\"", ErrInvalidIntent.Error())
	}
	if n := atomic.LoadInt32(&requestCount); n != 0 {
		t.Errorf("outbound requests = %d, want 0 (validation before network)", n)
	}
}
