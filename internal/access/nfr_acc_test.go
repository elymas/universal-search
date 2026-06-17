// Package access — SPEC-ACC-001 NFR verification tests.
//
// NFR-ACC-001: detectProfiles + validatePage pure-path performance
// (BenchmarkDetectAndValidate, excluded from coverage).
// NFR-ACC-002: no new outbound network operation — the profile/verdict
// additions issue zero additional requests vs the CACHE-001 baseline
// for the same fixture (TestNoNewNetworkCalls).
package access

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// cache001BaselineRequestCount is the measured CACHE-001 request count
// for the TestNoNewNetworkCalls fixture (a clean 200 HTML page that
// succeeds at Phase 3). The cascade issues exactly three origin
// requests for this fixture: robots.txt + a Phase 2 HEAD probe + the
// Phase 3 GET. The SPEC-ACC-001 detectProfiles/validatePage additions
// are pure functions over the already-fetched response + body, so this
// count MUST NOT grow. Captured on the green SPEC-ACC-001 run; update
// only if the cascade's phase sequencing itself changes (which is out
// of scope for this SPEC).
const cache001BaselineRequestCount = 3

// TestNoNewNetworkCalls asserts that a clean 200 page fetch issues
// exactly cache001BaselineRequestCount requests to the origin server —
// i.e. the SPEC-ACC-001 detectProfiles/validatePage additions introduce
// zero new network operations (NFR-ACC-002). The functions operate on
// the response + body Phase 3 already fetched.
func TestNoNewNetworkCalls(t *testing.T) {
	// NOT t.Parallel — asserts on a shared atomic counter.
	var requests atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		// Real page: > minRealPageBytes + a success selector → StrongOK,
		// no escalation, single Phase 3 request.
		body := append([]byte("<html><body><main><article><h1>ok</h1><p>"),
			bytes.Repeat([]byte("content paragraph. "), 60)...)
		body = append(body, []byte("</p></article></main></body></html>")...)
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	f := newTestFetcher(t)
	_, _ = f.Fetch(t.Context(), srv.URL+"/page", FetchOptions{})

	got := requests.Load()
	// The POINT of the assertion is that detectProfiles/validatePage
	// added zero new network ops — we compare against the captured
	// CACHE-001 baseline total (robots + HEAD probe + Phase 3 GET),
	// not a magic number.
	if got != cache001BaselineRequestCount {
		t.Errorf("NFR-ACC-002 violation: got %d origin requests, baseline %d (profile/verdict additions must add zero network ops)",
			got, cache001BaselineRequestCount)
	}
}

// BenchmarkDetectAndValidate measures the combined pure-path cost of
// detectProfiles + validatePage (NFR-ACC-001). The 1ms ceiling is
// platform-agnostic — both functions are pure body/header scans.
func BenchmarkDetectAndValidate(b *testing.B) {
	// Build a representative Akamai-challenged response + a real page
	// body from the fixtures inline (avoiding testdata I/O in the hot
	// loop).
	resp := &http.Response{
		StatusCode: 200,
		Header: http.Header{
			"Set-Cookie":   []string{"_abck=AAA~~-1~-1~-1~"},
			"Content-Type": []string{"text/html"},
		},
	}
	body := append([]byte("<html><body><main><article><h1>ok</h1><p>"),
		bytes.Repeat([]byte("content paragraph. "), 60)...)
	body = append(body, []byte("</p></article></main></body></html>")...)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		hits := detectProfiles(resp, body)
		top := topHitOrNil(hits)
		_ = validatePage(resp, body, top)
	}
}
