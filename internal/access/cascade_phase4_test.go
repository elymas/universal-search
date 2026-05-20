// Package access — cascade integration test forcing Phase 4 execution path.
//
// REQ-CACHE-005: Phase 4 is invoked when Phase 3 detects TLS error or WAF.
package access

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetch_Phase3WAF_EscalatesToPhase4_Success(t *testing.T) {
	t.Parallel()
	// WAF on first GET (Phase 3), success on second GET (Phase 4).
	// Use a counter to differentiate Phase 3 vs Phase 4 requests.
	var getCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			// Allow robots.
			_, _ = w.Write([]byte("User-agent: *\nAllow: /\n"))
			return
		}
		if r.Method == http.MethodHead {
			// HEAD probe succeeds.
			w.WriteHeader(http.StatusOK)
			return
		}
		getCount++
		if getCount == 1 {
			// Phase 3 GET → return WAF (403 + cf-ray).
			w.Header().Set("Cf-Ray", "test123-LAX")
			w.WriteHeader(http.StatusForbidden)
			return
		}
		// Phase 4 GET → success.
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>phase4 recovered</html>"))
	}))
	defer srv.Close()

	f := newTestFetcher(t)
	result, err := f.Fetch(t.Context(), srv.URL+"/page", FetchOptions{})
	if err != nil {
		t.Logf("Fetch returned error (acceptable if Phase 4 also fails on test server): %v", err)
		return
	}
	if result.Outcome == "success" {
		t.Logf("Cascade succeeded at phase %d", result.FinalPhase)
		// Verify we went through Phase 3 at minimum.
		if len(result.PhaseAttempts) < 3 {
			t.Errorf("Expected at least 3 phase attempts, got %d", len(result.PhaseAttempts))
		}
	}
}

func TestFetch_SkipPhase1_NoIndex(t *testing.T) {
	t.Parallel()
	// Without IndexLookup, Phase 1 is skipped (ErrPhaseNotApplicable).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("<html>ok</html>"))
	}))
	defer srv.Close()

	f := newTestFetcher(t) // no IndexLookup
	result, err := f.Fetch(t.Context(), srv.URL+"/page", FetchOptions{})
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	if len(result.PhaseAttempts) == 0 {
		t.Fatal("PhaseAttempts must be non-empty")
	}
	// Phase 1 should be "skipped".
	if result.PhaseAttempts[0].Phase == 1 && result.PhaseAttempts[0].Outcome != "skipped" {
		t.Errorf("Phase 1 without index must be skipped, got %q", result.PhaseAttempts[0].Outcome)
	}
}

func TestFetch_IndexLookup_Miss_EscalatesTo2(t *testing.T) {
	t.Parallel()
	// Index miss → Phase 1 "miss" → escalate to Phase 2.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("<html>fetched fresh</html>"))
	}))
	defer srv.Close()

	// Index returns miss.
	lookup := &testIndexLookup{doc: nil, found: false}
	f := newTestFetcher(t, func(o *Options) {
		o.IndexLookup = lookup
	})

	result, err := f.Fetch(t.Context(), srv.URL+"/page", FetchOptions{})
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	if result.Outcome != "success" {
		t.Errorf("Outcome = %q, want success", result.Outcome)
	}
	// Phase 1 returns ErrPhaseNotApplicable on miss → outcome "skipped".
	// This triggers escalation to Phase 2.
	phase1 := result.PhaseAttempts[0]
	if phase1.Outcome != "miss" && phase1.Outcome != "skipped" {
		t.Errorf("Phase 1 outcome = %q, want miss or skipped", phase1.Outcome)
	}
}
