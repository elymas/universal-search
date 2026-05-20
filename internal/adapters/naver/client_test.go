package naver

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// TestCategorizeStatusTable tests categorizeStatus with all relevant HTTP status codes.
// REQ-ADP8-003: status code to Category mapping.
func TestCategorizeStatusTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status   int
		want     types.Category
		hasRetry bool
	}{
		{http.StatusOK, types.CategoryUnknown, false}, // 200 unexpected here
		{http.StatusBadRequest, types.CategoryPermanent, false},
		{http.StatusUnauthorized, types.CategoryPermanent, false},
		{http.StatusForbidden, types.CategoryPermanent, false},
		{http.StatusNotFound, types.CategoryPermanent, false},
		{http.StatusTooManyRequests, types.CategoryRateLimited, true},
		{http.StatusInternalServerError, types.CategoryUnavailable, false},
		{http.StatusBadGateway, types.CategoryUnavailable, false},
		{http.StatusServiceUnavailable, types.CategoryUnavailable, false},
		{0, types.CategoryUnavailable, false}, // network error
	}

	for _, tc := range tests {
		tc := tc
		t.Run(fmt.Sprintf("status_%d", tc.status), func(t *testing.T) {
			t.Parallel()
			var retryAfter time.Duration
			if tc.hasRetry {
				retryAfter = 10 * time.Second
			}
			se := categorizeStatus(tc.status, retryAfter, fmt.Errorf("test error"))
			if se.Category != tc.want {
				t.Errorf("categorizeStatus(%d) category = %v, want %v", tc.status, se.Category, tc.want)
			}
			if tc.hasRetry && se.RetryAfter == 0 {
				t.Errorf("categorizeStatus(%d) RetryAfter = 0, want non-zero", tc.status)
			}
			if se.Adapter != "naver" {
				t.Errorf("categorizeStatus(%d) Adapter = %q, want %q", tc.status, se.Adapter, "naver")
			}
			if se.HTTPStatus != tc.status {
				t.Errorf("categorizeStatus(%d) HTTPStatus = %d, want %d", tc.status, se.HTTPStatus, tc.status)
			}
		})
	}
}

// TestRedirectAllowlist_AllowsNaverHost verifies the redirect allowlist permits
// openapi.naver.com redirects up to 3 hops.
func TestRedirectAllowlist_AllowsNaverHost(t *testing.T) {
	t.Parallel()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://openapi.naver.com/v1/search/blog.json", nil)
	via := []*http.Request{req} // 1 hop
	err := redirectAllowlist(req, via)
	if err != nil {
		t.Errorf("redirectAllowlist() error = %v, want nil for openapi.naver.com", err)
	}
}

// TestRedirectAllowlist_RejectsForeignHost verifies cross-domain redirects are blocked.
func TestRedirectAllowlist_RejectsForeignHost(t *testing.T) {
	t.Parallel()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://evil.com/steal", nil)
	via := []*http.Request{{}} // 1 hop, foreign host
	err := redirectAllowlist(req, via)
	if err == nil {
		t.Error("redirectAllowlist() error = nil, want error for foreign host")
	}
	if !strings.Contains(err.Error(), "cross-domain redirect") {
		t.Errorf("redirectAllowlist() error = %q, want 'cross-domain redirect'", err.Error())
	}
}

// TestRedirectAllowlist_RejectsAfter3Hops verifies max 3 hops enforcement.
func TestRedirectAllowlist_RejectsAfter3Hops(t *testing.T) {
	t.Parallel()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://openapi.naver.com/path", nil)
	via := []*http.Request{{}, {}, {}} // 3 prior hops = 4th redirect = rejected
	err := redirectAllowlist(req, via)
	if err == nil {
		t.Error("redirectAllowlist() error = nil, want error after 3 hops")
	}
	if !strings.Contains(err.Error(), "too many redirects") {
		t.Errorf("redirectAllowlist() error = %q, want 'too many redirects'", err.Error())
	}
}

// TestDoRequest_SetsAuthHeaders verifies doRequest sets the Naver auth headers.
// REQ-ADP8-004: X-Naver-Client-Id and X-Naver-Client-Secret must be set.
func TestDoRequest_SetsAuthHeaders(t *testing.T) {
	t.Parallel()

	var gotID, gotSecret string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = r.Header.Get("X-Naver-Client-Id")
		gotSecret = r.Header.Get("X-Naver-Client-Secret")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"total":0,"start":1,"display":0,"items":[]}`))
	}))
	defer srv.Close()

	a, err := New(Options{
		ClientID:     "my-client-id",
		ClientSecret: "my-client-secret",
		BaseURLBlog:  srv.URL,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	resp, err := a.doRequest(req)
	if err != nil {
		t.Fatalf("doRequest() error = %v", err)
	}
	defer resp.Body.Close()

	if gotID != "my-client-id" {
		t.Errorf("X-Naver-Client-Id = %q, want %q", gotID, "my-client-id")
	}
	if gotSecret != "my-client-secret" {
		t.Errorf("X-Naver-Client-Secret = %q, want %q", gotSecret, "my-client-secret")
	}
}
