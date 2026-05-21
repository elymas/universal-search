// Package hn — HTTP client helper tests.
// TestCategorizeStatusTable, TestParseRetryAfterTable, redirect allowlist tests,
// User-Agent and Accept header tests.
package hn

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// TestCategorizeStatusTable verifies the HTTP status → Category truth table.
func TestCategorizeStatusTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status           int
		expectedCategory types.Category
	}{
		{200, types.CategoryUnknown}, // unexpected in normal flow (Search drains 200)
		{401, types.CategoryPermanent},
		{403, types.CategoryPermanent},
		{404, types.CategoryPermanent},
		{429, types.CategoryRateLimited},
		{500, types.CategoryUnavailable},
		{503, types.CategoryUnavailable},
		{0, types.CategoryUnavailable}, // network-layer error
	}

	for _, tc := range tests {
		se := categorizeStatus(tc.status, 5*time.Second, errors.New("cause"))
		if se.Category != tc.expectedCategory {
			t.Errorf("categorizeStatus(%d) category = %v, want %v",
				tc.status, se.Category, tc.expectedCategory)
		}
		if se.Adapter != "hackernews" {
			t.Errorf("categorizeStatus(%d) Adapter = %q, want %q",
				tc.status, se.Adapter, "hackernews")
		}
		if se.HTTPStatus != tc.status {
			t.Errorf("categorizeStatus(%d) HTTPStatus = %d, want %d",
				tc.status, se.HTTPStatus, tc.status)
		}
	}

	// 429 with retryAfter must surface the duration.
	se429 := categorizeStatus(429, 30*time.Second, errors.New("rate limited"))
	if se429.RetryAfter != 30*time.Second {
		t.Errorf("categorizeStatus(429) RetryAfter = %v, want 30s", se429.RetryAfter)
	}
}

// TestParseRetryAfterTable verifies the Retry-After parser over 6 input shapes.
func TestParseRetryAfterTable(t *testing.T) {
	t.Parallel()

	// Anchor time: 30s before the HTTP-date fixture value.
	now := time.Date(2026, 10, 21, 7, 27, 30, 0, time.UTC)

	tests := []struct {
		name    string
		header  string
		wantMin time.Duration
		wantMax time.Duration
	}{
		{
			name:    "integer seconds",
			header:  "30",
			wantMin: 30 * time.Second,
			wantMax: 30 * time.Second,
		},
		{
			name:    "HTTP date 30s in future",
			header:  "Wed, 21 Oct 2026 07:28:00 GMT",
			wantMin: 25 * time.Second, // ±5s clock-drift allowance
			wantMax: 35 * time.Second,
		},
		{
			name:    "missing header",
			header:  "",
			wantMin: 5 * time.Second,
			wantMax: 5 * time.Second,
		},
		{
			name:    "malformed header",
			header:  "not-a-date-or-number",
			wantMin: 5 * time.Second,
			wantMax: 5 * time.Second,
		},
		{
			name:    "value exceeds 60s cap",
			header:  "999",
			wantMin: 60 * time.Second,
			wantMax: 60 * time.Second,
		},
		{
			name:    "negative value",
			header:  "-10",
			wantMin: 5 * time.Second,
			wantMax: 5 * time.Second,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := parseRetryAfter(tc.header, now)
			if got < tc.wantMin || got > tc.wantMax {
				t.Errorf("parseRetryAfter(%q) = %v, want [%v, %v]",
					tc.header, got, tc.wantMin, tc.wantMax)
			}
		})
	}
}

// TestSearchFollowsAllowlistRedirect verifies that the adapter follows redirects
// within the allowed-host set (hn.algolia.com → news.ycombinator.com).
func TestSearchFollowsAllowlistRedirect(t *testing.T) {
	t.Parallel()

	// Destination server serves a valid empty response.
	dest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"hits":[],"nbHits":0,"page":0,"nbPages":0,"hitsPerPage":25}`))
	}))
	defer dest.Close()

	// Origin server redirects to the destination within the allowed set.
	// Since both test servers run on 127.0.0.1, we test that the redirect
	// policy allows the same host (httptest uses 127.0.0.1, which is loopback,
	// not in the allowlist). We test the allow-policy directly via redirectAllowlist.

	// Construct a fake request to a host in the allowlist.
	req, err := http.NewRequest(http.MethodGet, "https://hn.algolia.com/api/v1/search?query=test", nil)
	if err != nil {
		t.Fatalf("http.NewRequest: %v", err)
	}
	// Single hop: via is empty — should be allowed.
	if err := redirectAllowlist(req, nil); err != nil {
		t.Errorf("redirectAllowlist(algolia, nil) unexpected error: %v", err)
	}

	// Redirect to news.ycombinator.com — also allowed.
	req2, err := http.NewRequest(http.MethodGet, "https://news.ycombinator.com/item?id=123", nil)
	if err != nil {
		t.Fatalf("http.NewRequest: %v", err)
	}
	if err := redirectAllowlist(req2, []*http.Request{req}); err != nil {
		t.Errorf("redirectAllowlist(hn, 1 hop) unexpected error: %v", err)
	}
}

// TestSearchRejectsCrossDomainRedirect verifies that cross-domain redirects
// (outside allowedRedirectHosts) are rejected with a CategoryPermanent error.
func TestSearchRejectsCrossDomainRedirect(t *testing.T) {
	t.Parallel()

	// Target server redirects to an external domain (evil.example.com).
	evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://evil.example.com/steal", http.StatusFound)
	}))
	defer evil.Close()

	a, err := New(Options{BaseURL: evil.URL})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := searchTestContext(t)
	_, searchErr := a.Search(ctx, types.Query{Text: "golang"})
	if searchErr == nil {
		t.Fatal("Search() expected error for cross-domain redirect, got nil")
	}

	var se *types.SourceError
	if !errors.As(searchErr, &se) {
		t.Fatalf("error is not *SourceError: %T — %v", searchErr, searchErr)
	}
	if se.Category != types.CategoryPermanent {
		t.Errorf("Category = %v, want CategoryPermanent", se.Category)
	}
}

// TestSearchRejectsRedirectChainOver3 verifies that redirect chains exceeding
// 3 hops are rejected.
func TestSearchRejectsRedirectChainOver3(t *testing.T) {
	t.Parallel()

	// Build a chain of 4 redirects; the 4th hop should be rejected.
	// We test the redirectAllowlist policy directly.
	req, err := http.NewRequest(http.MethodGet, "https://hn.algolia.com/", nil)
	if err != nil {
		t.Fatalf("http.NewRequest: %v", err)
	}

	via := []*http.Request{req, req, req} // 3 prior hops
	if err := redirectAllowlist(req, via); err == nil {
		t.Error("redirectAllowlist with 3 prior hops should return error")
	}
}

// TestSearchSetsCustomUserAgent verifies the User-Agent header format.
func TestSearchSetsCustomUserAgent(t *testing.T) {
	t.Parallel()

	var capturedUA string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"hits":[],"nbHits":0,"page":0,"nbPages":0,"hitsPerPage":25}`))
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := searchTestContext(t)
	_, _ = a.Search(ctx, types.Query{Text: "golang"})

	if !strings.HasPrefix(capturedUA, "usearch/") {
		t.Errorf("User-Agent = %q, want prefix %q", capturedUA, "usearch/")
	}
	if !strings.Contains(capturedUA, "(+https://github.com/elymas/universal-search)") {
		t.Errorf("User-Agent = %q, missing required suffix URL", capturedUA)
	}
}

// TestSearchSetsAcceptJSON verifies the Accept header is application/json.
func TestSearchSetsAcceptJSON(t *testing.T) {
	t.Parallel()

	var capturedAccept string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"hits":[],"nbHits":0,"page":0,"nbPages":0,"hitsPerPage":25}`))
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := searchTestContext(t)
	_, _ = a.Search(ctx, types.Query{Text: "golang"})

	if capturedAccept != "application/json" {
		t.Errorf("Accept = %q, want %q", capturedAccept, "application/json")
	}
}

// TestSearchUserAgentVersionConfigurable verifies the UA version token is configurable.
func TestSearchUserAgentVersionConfigurable(t *testing.T) {
	t.Parallel()

	var capturedUA string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"hits":[],"nbHits":0,"page":0,"nbPages":0,"hitsPerPage":25}`))
	}))
	defer ts.Close()

	const customVersion = "v9.9.9"
	a, err := New(Options{BaseURL: ts.URL, UserAgentVersion: customVersion})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := searchTestContext(t)
	_, _ = a.Search(ctx, types.Query{Text: "golang"})

	if !strings.Contains(capturedUA, customVersion) {
		t.Errorf("User-Agent = %q, want version %q in string", capturedUA, customVersion)
	}
}
