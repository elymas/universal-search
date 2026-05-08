// Package access — unit tests for Phase 3 GET-specific logic.
//
// REQ-CACHE-004: WAF detection, TLS error classification, body reading.
package access

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsWAFResponse_Cloudflare403(t *testing.T) {
	t.Parallel()
	resp := &http.Response{
		StatusCode: 403,
		Header:     http.Header{"Cf-Ray": []string{"abc123-LAX"}},
	}
	if !isWAFResponse(resp) {
		t.Error("403 with cf-ray header must be WAF")
	}
}

func TestIsWAFResponse_503WithNoWAFHeaders(t *testing.T) {
	t.Parallel()
	resp := &http.Response{
		StatusCode: 503,
		Header:     http.Header{},
	}
	if isWAFResponse(resp) {
		t.Error("503 without WAF headers must NOT be WAF")
	}
}

func TestIsWAFResponse_200_NotWAF(t *testing.T) {
	t.Parallel()
	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Cf-Ray": []string{"abc"}},
	}
	if isWAFResponse(resp) {
		t.Error("200 must NOT be WAF even with WAF headers")
	}
}

func TestIsTLSError_TLSString(t *testing.T) {
	t.Parallel()
	cases := []struct {
		msg  string
		want bool
	}{
		{"tls: certificate verify failed", true},
		{"TLS handshake timeout", true},
		{"x509: certificate signed by unknown authority", true},
		{"connection refused", false},
		{"EOF", false},
		{"", false},
	}
	for _, tc := range cases {
		got := isTLSError(&stubError{tc.msg})
		if got != tc.want {
			t.Errorf("isTLSError(%q) = %v, want %v", tc.msg, got, tc.want)
		}
	}
}

// stubError is a simple error implementation for testing.
type stubError struct{ msg string }

func (e *stubError) Error() string { return e.msg }

func TestReadBody_Limit(t *testing.T) {
	t.Parallel()
	// readBody limits to maxBytes.
	body := make([]byte, 1024*1024) // 1MB
	for i := range body {
		body[i] = 'a'
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL) //nolint:noctx
	if err != nil {
		t.Fatalf("http.Get error: %v", err)
	}
	defer resp.Body.Close()

	limit := int64(512 * 1024) // 512KB limit
	got, err := readBody(resp, limit)
	if err != nil {
		t.Fatalf("readBody error: %v", err)
	}
	if int64(len(got)) > limit {
		t.Errorf("readBody returned %d bytes, want at most %d", len(got), limit)
	}
}

func TestPhase3Get_200_Success(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>phase3 ok</html>"))
	}))
	defer srv.Close()

	content, attempt, err := phase3Get(
		t.Context(),
		srv.URL+"/page",
		FetchOptions{AllowPrivateNetworks: true},
		Options{AllowPrivateNetworks: true, MaxBodyBytes: defaultMaxBodyBytes, RedirectMaxHops: 5},
	)
	if err != nil {
		t.Fatalf("phase3Get error: %v", err)
	}
	if content == nil {
		t.Fatal("content must not be nil")
	}
	if attempt != nil {
		t.Error("attempt must be nil on success")
	}
	if len(content.Body) == 0 {
		t.Error("content body must not be empty")
	}
}

func TestPhase3Get_404_PermanentError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	_, _, err := phase3Get(
		t.Context(),
		srv.URL+"/missing",
		FetchOptions{AllowPrivateNetworks: true},
		Options{AllowPrivateNetworks: true, MaxBodyBytes: defaultMaxBodyBytes, RedirectMaxHops: 5},
	)
	if err == nil {
		t.Fatal("phase3Get 404 must return error")
	}
	fe, ok := err.(*FetchError)
	if !ok || fe.Category != CategoryPermanent {
		t.Errorf("404 must return CategoryPermanent, got %v", err)
	}
}

func TestPhase3Get_429_RateLimited(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	_, _, err := phase3Get(
		t.Context(),
		srv.URL+"/page",
		FetchOptions{AllowPrivateNetworks: true},
		Options{AllowPrivateNetworks: true, MaxBodyBytes: defaultMaxBodyBytes, RedirectMaxHops: 5},
	)
	if err == nil {
		t.Fatal("429 must return error")
	}
	fe, ok := err.(*FetchError)
	if !ok || fe.Category != CategoryRateLimited {
		t.Errorf("429 must return CategoryRateLimited, got %v", err)
	}
}
