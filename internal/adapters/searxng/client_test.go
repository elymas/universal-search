package searxng_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/adapters/searxng"
	"github.com/elymas/universal-search/pkg/types"
)

// TestCategorizeStatusTable tests the HTTP status → Category mapping via the
// Search hot path (externally observable through the returned *SourceError).
func TestCategorizeStatusTable(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		statusCode     int
		retryAfter     string
		wantCategory   types.Category
		wantRetryAfter bool
	}{
		{
			name:         "429 rate limited",
			statusCode:   http.StatusTooManyRequests,
			wantCategory: types.CategoryRateLimited,
		},
		{
			name:           "429 with Retry-After",
			statusCode:     http.StatusTooManyRequests,
			retryAfter:     "30",
			wantCategory:   types.CategoryRateLimited,
			wantRetryAfter: true,
		},
		{
			name:           "403 with Retry-After promoted to RateLimited (REQ-ADP7-007)",
			statusCode:     http.StatusForbidden,
			retryAfter:     "10",
			wantCategory:   types.CategoryRateLimited,
			wantRetryAfter: true,
		},
		{
			name:         "403 without Retry-After is Permanent",
			statusCode:   http.StatusForbidden,
			wantCategory: types.CategoryPermanent,
		},
		{
			name:         "404 is Permanent",
			statusCode:   http.StatusNotFound,
			wantCategory: types.CategoryPermanent,
		},
		{
			name:         "400 is Permanent",
			statusCode:   http.StatusBadRequest,
			wantCategory: types.CategoryPermanent,
		},
		{
			name:         "500 is Unavailable",
			statusCode:   http.StatusInternalServerError,
			wantCategory: types.CategoryUnavailable,
		},
		{
			name:         "503 is Unavailable",
			statusCode:   http.StatusServiceUnavailable,
			wantCategory: types.CategoryUnavailable,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.retryAfter != "" {
					w.Header().Set("Retry-After", tc.retryAfter)
				}
				w.WriteHeader(tc.statusCode)
			}))
			defer srv.Close()

			a, err := searxng.New(searxng.Options{BaseURL: srv.URL})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			_, searchErr := a.Search(testCtx(t), types.Query{Text: "test"})
			if searchErr == nil {
				t.Fatal("Search returned nil error, want *SourceError")
			}
			var se *types.SourceError
			if !asSourceError(searchErr, &se) {
				t.Fatalf("error type = %T, want *SourceError", searchErr)
			}
			if se.Category != tc.wantCategory {
				t.Errorf("Category = %v, want %v", se.Category, tc.wantCategory)
			}
			if tc.wantRetryAfter && se.RetryAfter == 0 {
				t.Error("RetryAfter = 0, want > 0")
			}
		})
	}
}

// TestParseRetryAfterTable tests parseRetryAfter via the 429 response path.
func TestParseRetryAfterTable(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		header  string
		wantMin time.Duration
		wantMax time.Duration
	}{
		{
			name:    "integer seconds 30",
			header:  "30",
			wantMin: 30 * time.Second,
			wantMax: 30 * time.Second,
		},
		{
			name:    "integer seconds > 60 capped to 60",
			header:  "120",
			wantMin: 60 * time.Second,
			wantMax: 60 * time.Second,
		},
		{
			name:    "integer 0 returns default 5s",
			header:  "0",
			wantMin: 5 * time.Second,
			wantMax: 5 * time.Second,
		},
		{
			name:    "negative integer returns default 5s",
			header:  "-5",
			wantMin: 5 * time.Second,
			wantMax: 5 * time.Second,
		},
		{
			name:    "empty header returns default 5s",
			header:  "",
			wantMin: 5 * time.Second,
			wantMax: 5 * time.Second,
		},
		{
			name:    "garbage string returns default 5s",
			header:  "garbage",
			wantMin: 5 * time.Second,
			wantMax: 5 * time.Second,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.header != "" {
					w.Header().Set("Retry-After", tc.header)
				}
				w.WriteHeader(http.StatusTooManyRequests)
			}))
			defer srv.Close()

			a, err := searxng.New(searxng.Options{BaseURL: srv.URL})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			_, searchErr := a.Search(testCtx(t), types.Query{Text: "test"})
			var se *types.SourceError
			if !asSourceError(searchErr, &se) {
				t.Fatalf("error type = %T, want *SourceError", searchErr)
			}
			if se.RetryAfter < tc.wantMin || se.RetryAfter > tc.wantMax {
				t.Errorf("RetryAfter = %v, want [%v, %v]", se.RetryAfter, tc.wantMin, tc.wantMax)
			}
		})
	}
}

// TestSearchFollowsAllowlistRedirect verifies that redirects to allowlisted hosts are followed.
func TestSearchFollowsAllowlistRedirect(t *testing.T) {
	t.Parallel()
	// Use a server that immediately redirects to itself (same host = localhost).
	redirected := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !redirected {
			redirected = true
			http.Redirect(w, r, r.URL.String(), http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"query":"test","results":[]}`))
	}))
	defer srv.Close()

	a, err := searxng.New(searxng.Options{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Should succeed — localhost is in the allowlist.
	docs, err := a.Search(testCtx(t), types.Query{Text: "test"})
	if err != nil {
		t.Errorf("Search with allowlist redirect: %v", err)
	}
	_ = docs
}

// TestSearchRejectsCrossDomainRedirect verifies cross-domain redirects are blocked.
func TestSearchRejectsCrossDomainRedirect(t *testing.T) {
	t.Parallel()
	// Redirect to an external host (not in allowlist).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://external-evil-host.example.com/steal", http.StatusFound)
	}))
	defer srv.Close()

	a, err := searxng.New(searxng.Options{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, searchErr := a.Search(testCtx(t), types.Query{Text: "test"})
	if searchErr == nil {
		t.Fatal("Search returned nil error, want cross-domain redirect error")
	}
	var se *types.SourceError
	if !asSourceError(searchErr, &se) {
		t.Fatalf("error type = %T, want *SourceError", searchErr)
	}
	// Cross-domain redirect is a policy violation → Permanent.
	if se.Category != types.CategoryPermanent {
		t.Errorf("Category = %v, want CategoryPermanent for cross-domain redirect", se.Category)
	}
}

// TestSearchRejectsRedirectChainOver3 verifies that more than 3 redirect hops are rejected.
func TestSearchRejectsRedirectChainOver3(t *testing.T) {
	t.Parallel()
	hops := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hops++
		if hops <= 5 {
			http.Redirect(w, r, r.URL.String(), http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"query":"test","results":[]}`))
	}))
	defer srv.Close()

	a, err := searxng.New(searxng.Options{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, searchErr := a.Search(testCtx(t), types.Query{Text: "test"})
	if searchErr == nil {
		t.Fatal("Search returned nil error, want too-many-redirects error")
	}
}
