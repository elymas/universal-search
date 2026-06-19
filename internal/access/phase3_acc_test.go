// Package access — SPEC-ACC-001 Phase 3 integration tests.
//
// REQ-ACC-020: Phase 3 sets attempt.verdict.
// REQ-ACC-021: a silent-200 challenge is NOT counted as success.
// REQ-ACC-022: a WeakOK 200 (JSON body, no selector) IS success.
package access

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestPhase3SetsVerdict: a 200 with the page_strong_ok.html fixture
// (real page, <main>, > 512 bytes, cleared sensor) → the returned
// content signals VerdictStrongOK. REQ-ACC-020.
//
// phase3Get returns (content, attempt, err); on a clean success the
// verdict is carried on the attempt only when an attempt is returned.
// For a pure-success 200 (no escalation), phase3Get returns attempt=nil,
// so we verify success + content instead; the verdict path is exercised
// via validatePage directly in validity_test.go and via the
// silent-200 challenge case below (where an attempt IS returned).
func TestPhase3SetsVerdict(t *testing.T) {
	t.Parallel()
	realPage := []byte(`<html><body><main id="content"><article><h1>real</h1><p>` +
		strings.Repeat("content paragraph. ", 60) +
		"</p></article></main></body></html>")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write(realPage)
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
		t.Fatal("content must not be nil for a real page")
	}
	// A clean StrongOK 200 returns attempt=nil (no escalation needed).
	// The verdict was computed internally and, being StrongOK, did not
	// trigger the silent-200 branch.
	if attempt != nil {
		t.Errorf("clean StrongOK 200 must return attempt=nil, got %+v", attempt)
	}
}

// TestPhase3Silent200ChallengeNotSuccess: server returns HTTP 200 with
// Akamai _abck=~-1~ cookie + a sub-512-byte challenge body. The phase
// must NOT return success content; the returned attempt must carry
// VerdictBlocked (L3 yes + L2 yes) so the cascade escalates.
// REQ-ACC-021.
func TestPhase3Silent200ChallengeNotSuccess(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Set-Cookie", "_abck=AAA~~-1~-1~-1~")
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(200)
		_, _ = w.Write([]byte("<html><body>short</body></html>")) // < 512 bytes
	}))
	defer srv.Close()

	content, attempt, err := phase3Get(
		t.Context(),
		srv.URL+"/page",
		FetchOptions{AllowPrivateNetworks: true},
		Options{AllowPrivateNetworks: true, MaxBodyBytes: defaultMaxBodyBytes, RedirectMaxHops: 5},
	)
	if content != nil {
		t.Errorf("silent-200 challenge must NOT return content, got %+v", content)
	}
	if err == nil {
		t.Error("silent-200 challenge must return a FetchError")
	}
	if attempt == nil {
		t.Fatal("attempt must not be nil for a silent-200 challenge")
	}
	if attempt.verdict != VerdictBlocked {
		t.Errorf("attempt.verdict = %q, want VerdictBlocked", attempt.verdict)
	}
	if !shouldEscalate(attempt) {
		t.Error("silent-200 Blocked attempt must escalate to Phase 4")
	}
}

// TestPhase3Silent200ChallengeNotSuccess_LargeBody covers the
// VerdictChallenge variant: _abck=~-1~ on a normal-size body (L3 yes,
// NOT L2) → VerdictChallenge, also not success. REQ-ACC-021.
func TestPhase3Silent200ChallengeNotSuccess_LargeBody(t *testing.T) {
	t.Parallel()
	bigChallenge := []byte("<html><body>" + strings.Repeat("paragraph ", 80) + "</body></html>")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Set-Cookie", "_abck=AAA~~-1~-1~-1~")
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(200)
		_, _ = w.Write(bigChallenge)
	}))
	defer srv.Close()

	content, attempt, err := phase3Get(
		t.Context(),
		srv.URL+"/page",
		FetchOptions{AllowPrivateNetworks: true},
		Options{AllowPrivateNetworks: true, MaxBodyBytes: defaultMaxBodyBytes, RedirectMaxHops: 5},
	)
	if content != nil {
		t.Errorf("silent-200 challenge must NOT return content, got %+v", content)
	}
	if err == nil {
		t.Error("silent-200 challenge must return a FetchError")
	}
	if attempt == nil || attempt.verdict != VerdictChallenge {
		t.Errorf("attempt.verdict = %q, want VerdictChallenge", verdictOrEmpty(attempt))
	}
	if attempt != nil && !shouldEscalate(attempt) {
		t.Error("silent-200 Challenge attempt must escalate")
	}
}

// TestPhase3WeakOKIsSuccess: a 200 with a JSON body (no challenge, no
// success selector, normal size) → success + VerdictWeakOK semantics.
// phase3Get returns (content, nil, nil) on a clean WeakOK; the verdict
// is only surfaced on the attempt when escalation is needed, so for the
// pure-success path we verify content + err==nil. REQ-ACC-022.
func TestPhase3WeakOKIsSuccess(t *testing.T) {
	t.Parallel()
	jsonBody := []byte(`{"status":"ok","items":[1,2,3,4,5,6,7,8,9,10],"meta":{"count":10,"page":1,"total":100,"cursor":"abc","extra":"` +
		strings.Repeat("padding", 40) + `"}}`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(jsonBody)
	}))
	defer srv.Close()

	content, _, err := phase3Get(
		t.Context(),
		srv.URL+"/api",
		FetchOptions{AllowPrivateNetworks: true},
		Options{AllowPrivateNetworks: true, MaxBodyBytes: defaultMaxBodyBytes, RedirectMaxHops: 5},
	)
	if err != nil {
		t.Fatalf("WeakOK JSON 200 must succeed, got err=%v", err)
	}
	if content == nil {
		t.Fatal("WeakOK JSON 200 must return content")
	}
	if string(content.Body) != string(jsonBody) {
		t.Error("WeakOK JSON body must be returned verbatim")
	}
}

// verdictOrEmpty is a small helper to avoid nil-deref in error messages.
func verdictOrEmpty(a *PhaseAttempt) Verdict {
	if a == nil {
		return ""
	}
	return a.verdict
}
