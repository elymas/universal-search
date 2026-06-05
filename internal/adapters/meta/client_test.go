package meta

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// ---------------------------------------------------------------------------
// REQ-ADP10-002: Request headers (Bearer, User-Agent, Accept)
// ---------------------------------------------------------------------------

// TestSearchThreadsSetsBearerToken verifies the Authorization: Bearer header is set.
func TestSearchThreadsSetsBearerToken(t *testing.T) {
	var capturedAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer ts.Close()

	a, _ := NewThreads(ThreadsOptions{
		AccessToken: "my-secret-token",
		BaseURL:     ts.URL,
		HTTPClient:  ts.Client(),
	})
	_, _ = a.Search(t.Context(), types.Query{Text: "test"})

	want := "Bearer my-secret-token"
	if capturedAuth != want {
		t.Errorf("Authorization = %q, want %q", capturedAuth, want)
	}
}

// TestSearchThreadsSetsCustomUserAgent verifies the User-Agent header.
func TestSearchThreadsSetsCustomUserAgent(t *testing.T) {
	var capturedUA string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer ts.Close()

	a, _ := NewThreads(ThreadsOptions{
		AccessToken: "t",
		BaseURL:     ts.URL,
		HTTPClient:  ts.Client(),
	})
	_, _ = a.Search(t.Context(), types.Query{Text: "test"})

	if !strings.HasPrefix(capturedUA, "usearch/") {
		t.Errorf("User-Agent = %q, want prefix 'usearch/'", capturedUA)
	}
	if !strings.Contains(capturedUA, "github.com") {
		t.Errorf("User-Agent = %q, want to contain repo URL", capturedUA)
	}
}

// TestSearchThreadsSetsAcceptJSON verifies the Accept header.
func TestSearchThreadsSetsAcceptJSON(t *testing.T) {
	var capturedAccept string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAccept = r.Header.Get("Accept")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer ts.Close()

	a, _ := NewThreads(ThreadsOptions{
		AccessToken: "t",
		BaseURL:     ts.URL,
		HTTPClient:  ts.Client(),
	})
	_, _ = a.Search(t.Context(), types.Query{Text: "test"})

	if capturedAccept != "application/json" {
		t.Errorf("Accept = %q, want 'application/json'", capturedAccept)
	}
}

// ---------------------------------------------------------------------------
// REQ-ADP10-003: categorizeStatus truth table
// ---------------------------------------------------------------------------

// TestCategorizeStatusTable verifies the HTTP status to Category mapping.
func TestCategorizeStatusTable(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		category types.Category
	}{
		{"401 Permanent", 401, types.CategoryPermanent},
		{"403 Permanent", 403, types.CategoryPermanent},
		{"400 Permanent", 400, types.CategoryPermanent},
		{"404 Permanent", 404, types.CategoryPermanent},
		{"429 RateLimited", 429, types.CategoryRateLimited},
		{"500 Unavailable", 500, types.CategoryUnavailable},
		{"503 Unavailable", 503, types.CategoryUnavailable},
		{"0 Unavailable (network)", 0, types.CategoryUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			se := categorizeStatus("threads", tt.status, 0, nil)
			if se.Category != tt.category {
				t.Errorf("status %d: Category = %v, want %v", tt.status, se.Category, tt.category)
			}
			if se.Adapter != "threads" {
				t.Errorf("Adapter = %q, want 'threads'", se.Adapter)
			}
			if se.HTTPStatus != tt.status {
				t.Errorf("HTTPStatus = %d, want %d", se.HTTPStatus, tt.status)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// REQ-ADP10-003: Redirect allowlist
// ---------------------------------------------------------------------------

// TestSearchThreadsRejectsCrossDomainRedirect verifies cross-domain redirect rejection.
func TestSearchThreadsRejectsCrossDomainRedirect(t *testing.T) {
	// Target server (attacker.com equivalent).
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer target.Close()

	// Main server that redirects to the target (cross-domain).
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer ts.Close()

	// Use a client that applies our redirect allowlist.
	client := &http.Client{
		CheckRedirect: threadsRedirectAllowlist,
	}

	a, _ := NewThreads(ThreadsOptions{
		AccessToken: "t",
		BaseURL:     ts.URL,
		HTTPClient:  client,
	})
	_, err := a.Search(t.Context(), types.Query{Text: "test"})
	if err == nil {
		t.Fatal("expected error on cross-domain redirect, got nil")
	}

	var se *types.SourceError
	if !errors.As(err, &se) {
		t.Fatalf("expected *types.SourceError, got %T: %v", err, err)
	}
	if se.Category != types.CategoryPermanent {
		t.Errorf("Category = %v, want Permanent", se.Category)
	}
}

// ---------------------------------------------------------------------------
// REQ-ADP10-003: parseRetryAfter table
// ---------------------------------------------------------------------------

// TestParseRetryAfterTable verifies Retry-After header parsing.
func TestParseRetryAfterTable(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name   string
		input  string
		expect time.Duration
	}{
		{"empty defaults to 5s", "", 5 * time.Second},
		{"30s integer", "30", 30 * time.Second},
		{"capped at 60s", "999", 60 * time.Second},
		{"negative defaults to 5s", "-1", 5 * time.Second},
		{"zero defaults to 5s", "0", 5 * time.Second},
		{"malformed defaults to 5s", "not-a-number", 5 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRetryAfter(tt.input, now)
			if got != tt.expect {
				t.Errorf("parseRetryAfter(%q) = %v, want %v", tt.input, got, tt.expect)
			}
		})
	}
}
