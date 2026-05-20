// Package social — tests for HTTP client construction, redirect allowlist, headers.
// REQ-ADP6-003/009/010: timeout, SSRF guard, User-Agent, reqid transport.
package social

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// TestCategorizeStatusTable verifies the HTTP-status → Category mapping.
func TestCategorizeStatusTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		status       int
		retryAfter   time.Duration
		wantCategory types.Category
	}{
		{
			name:         "429 maps to CategoryRateLimited",
			status:       429,
			retryAfter:   10 * time.Second,
			wantCategory: types.CategoryRateLimited,
		},
		{
			name:         "400 maps to CategoryPermanent",
			status:       400,
			wantCategory: types.CategoryPermanent,
		},
		{
			name:         "404 maps to CategoryPermanent",
			status:       404,
			wantCategory: types.CategoryPermanent,
		},
		{
			name:         "403 maps to CategoryPermanent",
			status:       403,
			wantCategory: types.CategoryPermanent,
		},
		{
			name:         "500 maps to CategoryUnavailable",
			status:       500,
			wantCategory: types.CategoryUnavailable,
		},
		{
			name:         "503 maps to CategoryUnavailable",
			status:       503,
			wantCategory: types.CategoryUnavailable,
		},
		{
			name:         "0 (network error) maps to CategoryUnavailable",
			status:       0,
			wantCategory: types.CategoryUnavailable,
		},
		{
			name:         "302 (unexpected) maps to CategoryUnknown",
			status:       302,
			wantCategory: types.CategoryUnknown,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			se := categorizeStatus("bluesky", tc.status, tc.retryAfter, nil)
			if se.Category != tc.wantCategory {
				t.Errorf("categorizeStatus(%d): Category got %v, want %v", tc.status, se.Category, tc.wantCategory)
			}
			if se.Adapter != "bluesky" {
				t.Errorf("categorizeStatus(%d): Adapter got %q, want %q", tc.status, se.Adapter, "bluesky")
			}
		})
	}
}

// TestCategorizeStatusRateLimitedRetryAfter verifies RetryAfter is preserved on 429.
func TestCategorizeStatusRateLimitedRetryAfter(t *testing.T) {
	t.Parallel()
	want := 15 * time.Second
	se := categorizeStatus("bluesky", 429, want, nil)
	if se.RetryAfter != want {
		t.Errorf("categorizeStatus(429).RetryAfter: got %v, want %v", se.RetryAfter, want)
	}
}

// TestRedirectAllowlistAccepted verifies allowed Bluesky hosts pass the redirect check.
func TestRedirectAllowlistAccepted(t *testing.T) {
	t.Parallel()
	allowed := []string{
		"public.api.bsky.app",
		"api.bsky.app",
		"bsky.app",
	}
	for _, host := range allowed {
		host := host
		t.Run(host, func(t *testing.T) {
			t.Parallel()
			req, _ := http.NewRequest(http.MethodGet, "https://"+host+"/path", nil)
			via := []*http.Request{{}}
			if err := blueskyRedirectAllowlist(req, via); err != nil {
				t.Errorf("blueskyRedirectAllowlist(%q): unexpected error: %v", host, err)
			}
		})
	}
}

// TestRedirectAllowlistRejectedForeignHost verifies non-Bluesky hosts are rejected.
func TestRedirectAllowlistRejectedForeignHost(t *testing.T) {
	t.Parallel()
	req, _ := http.NewRequest(http.MethodGet, "https://evil.example.com/path", nil)
	via := []*http.Request{{}}
	if err := blueskyRedirectAllowlist(req, via); err == nil {
		t.Error("blueskyRedirectAllowlist: expected error for foreign host, got nil")
	}
}

// TestRedirectAllowlistTooManyHops verifies max 3 redirect hops.
func TestRedirectAllowlistTooManyHops(t *testing.T) {
	t.Parallel()
	req, _ := http.NewRequest(http.MethodGet, "https://bsky.app/path", nil)
	via := make([]*http.Request, 3)
	if err := blueskyRedirectAllowlist(req, via); err == nil {
		t.Error("blueskyRedirectAllowlist: expected error for 3+ hops, got nil")
	}
}

// TestDoRequestSetsHeaders verifies User-Agent and Accept headers are set.
func TestDoRequestSetsHeaders(t *testing.T) {
	t.Parallel()
	var capturedUA, capturedAccept string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUA = r.Header.Get("User-Agent")
		capturedAccept = r.Header.Get("Accept")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	a, _ := NewBluesky(BlueskyOptions{
		BaseURL:    ts.URL,
		HTTPClient: ts.Client(),
	})

	req, _ := http.NewRequest(http.MethodGet, ts.URL, nil)
	resp, err := a.doRequest(req)
	if err != nil {
		t.Fatalf("doRequest: unexpected error: %v", err)
	}
	resp.Body.Close()

	if capturedUA == "" {
		t.Error("doRequest: User-Agent header not set")
	}
	if capturedAccept != "application/json" {
		t.Errorf("doRequest: Accept header: got %q, want %q", capturedAccept, "application/json")
	}
}
