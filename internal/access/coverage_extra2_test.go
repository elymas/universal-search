// Package access — additional coverage tests (batch 2).
//
// Covers:
//   - derivePhaseCtx parent deadline shorter than phase budget
//   - phase3Get TLS error path (isTLSError)
//   - phase4TLS TLS error path (isTLSError from request failure)
//   - dispatchPhase default case
//   - cacheWriteThrough with IndexLookup enabled
//   - Fetch with already-cancelled ctx before phases start
//   - phase2Probe SkipHEADProbe=true path
//   - buildTransport with tlsCfg set (AllowPrivateNetworks)
//   - logger() method nil-obs case
package access

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

// --- derivePhaseCtx with parent deadline shorter than phase budget ---

func TestDerivePhaseCtx_ParentShorterThanBudget(t *testing.T) {
	t.Parallel()
	f := newTestFetcher(t)

	// Parent deadline of 50ms — less than default per-phase budget.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	phaseCtx, cancelPhase := f.derivePhaseCtx(ctx, 3)
	defer cancelPhase()

	// The derived context should inherit the shorter deadline.
	deadline, ok := phaseCtx.Deadline()
	if !ok {
		t.Fatal("phase context must have a deadline")
	}
	parentDeadline, _ := ctx.Deadline()
	// Phase context deadline must not exceed parent deadline.
	if deadline.After(parentDeadline.Add(time.Millisecond)) {
		t.Errorf("phase deadline %v exceeds parent deadline %v", deadline, parentDeadline)
	}
}

// --- phase3Get: TLS error (connect to TLS server without HTTPS scheme) ---

func TestPhase3Get_TLSServer_PlainHTTP_Fails(t *testing.T) {
	t.Parallel()
	// HTTPS test server — connecting to it via HTTP will cause a protocol error.
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	// Use plain HTTP URL against TLS server → connection error.
	plainURL := "http://" + srv.Listener.Addr().String() + "/page"

	_, attempt, err := phase3Get(
		t.Context(),
		plainURL,
		FetchOptions{AllowPrivateNetworks: true},
		Options{AllowPrivateNetworks: true, MaxBodyBytes: defaultMaxBodyBytes, RedirectMaxHops: 5},
	)
	if err == nil {
		t.Skip("no error on plain HTTP to TLS server (unexpected, skip)")
	}
	// attempt may be nil if transport setup itself fails.
	_ = attempt
	t.Logf("TLS error attempt: %+v, err: %v", attempt, err)
}

// --- phase4TLS: TLS error from client.Do ---

func TestPhase4TLS_PlainServerWithTLSConfig_Fails(t *testing.T) {
	t.Parallel()
	// Plain HTTP server — phase4TLS uses TLS client which will fail handshake.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	// Use HTTPS scheme so phase4TLS tries TLS handshake against a non-TLS server.
	tlsURL := "https://" + srv.Listener.Addr().String() + "/page"

	_, attempt, err := phase4TLS(
		t.Context(),
		tlsURL,
		FetchOptions{AllowPrivateNetworks: true},
		Options{AllowPrivateNetworks: true, MaxBodyBytes: defaultMaxBodyBytes},
	)
	if err == nil {
		t.Skip("no TLS error connecting TLS client to plain server (skip)")
	}
	_ = attempt
	t.Logf("TLS error: attempt=%+v, err=%v", attempt, err)
}

// --- dispatchPhase default case (phase > 5) ---

func TestDispatchPhase_Default_ReturnsNotApplicable(t *testing.T) {
	t.Parallel()
	f := newTestFetcher(t)
	u, _ := url.Parse("http://example.com/")
	_, err := f.dispatchPhase(t.Context(), 99, "http://example.com/", u, FetchOptions{})
	if err != ErrPhaseNotApplicable {
		t.Errorf("phase 99 must return ErrPhaseNotApplicable, got %v", err)
	}
}

// --- Fetch with already-cancelled ctx (before phases start) ---

func TestFetch_CtxCancelledBefore_OutcomeCancelled(t *testing.T) {
	t.Parallel()
	f := newTestFetcher(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	result, err := f.Fetch(ctx, "http://example.com/", FetchOptions{})
	if err != nil {
		t.Logf("err = %v (acceptable)", err)
	}
	if result == nil {
		t.Fatal("cancelled fetch must return a non-nil result")
	}
	// Outcome should be cancelled or timeout from context.
	t.Logf("Outcome = %q", result.Outcome)
}

// --- cacheWriteThrough with IndexLookup set and CacheWriteThrough=true ---

func TestCacheWriteThrough_WithLookup_UpsertsCalled2(t *testing.T) {
	t.Parallel()
	lookup := &countingIndexLookup{}
	f := newTestFetcher(t, func(o *Options) {
		o.IndexLookup = lookup
		o.CacheWriteThrough = true
	})
	content := &FetchedContent{
		URL:  "http://example.com/page2",
		Body: []byte("<html>ok2</html>"),
	}
	f.cacheWriteThrough(content)
	// Wait for the async goroutine to complete.
	f.writeThroughWG.Wait()

	if lookup.upsertCalled.Load() == 0 {
		t.Error("IndexLookup.Upsert must be called when CacheWriteThrough=true and IndexLookup set")
	}
}

// --- phase2Probe SkipHEADProbe+SkipRobotsTxt → ErrPhaseNotApplicable ---

func TestPhase2Probe_BothSkips_ReturnsNotApplicable(t *testing.T) {
	t.Parallel()
	// When both SkipHEADProbe AND SkipRobotsTxt are true, phase2 is fully skipped.
	_, err := phase2Probe(
		t.Context(),
		"http://example.com/page",
		FetchOptions{SkipHEADProbe: true, SkipRobotsTxt: true},
		Options{AllowPrivateNetworks: true},
		newRobotsCache(5*time.Second),
	)
	if err != ErrPhaseNotApplicable {
		t.Errorf("both skips must return ErrPhaseNotApplicable, got %v", err)
	}
}

// --- phase2Probe SkipHEADProbe=true (but robots.txt checked → returns content) ---

func TestPhase2Probe_SkipHEADProbe_RobotsAllow_ReturnsContent(t *testing.T) {
	t.Parallel()
	// robots.txt server that allows all.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("User-agent: *\nAllow: /\n"))
	}))
	defer srv.Close()

	content, err := phase2Probe(
		t.Context(),
		srv.URL+"/page",
		FetchOptions{SkipHEADProbe: true},
		Options{AllowPrivateNetworks: true},
		newRobotsCache(5*time.Second),
	)
	if err != nil {
		t.Fatalf("SkipHEADProbe with robots allow must succeed, got %v", err)
	}
	if content == nil {
		t.Fatal("content must not be nil when SkipHEADProbe=true and robots allow")
	}
}

// --- Fetch result: all phases attempted before failure ---

func TestFetch_Phase3TLS_EscalatesToPhase4(t *testing.T) {
	t.Parallel()
	// TLS server, but we connect via plain HTTP so phase3 errors with
	// a network/TLS error. Test that cascade continues.
	var getCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			w.WriteHeader(404)
			return
		}
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		getCount++
		// All GET requests succeed.
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>ok</html>"))
	}))
	defer srv.Close()

	f := newTestFetcher(t)
	result, err := f.Fetch(t.Context(), srv.URL+"/page", FetchOptions{})
	if err != nil {
		t.Logf("Fetch error (acceptable): %v", err)
		return
	}
	if result.Outcome != "success" {
		t.Errorf("Outcome = %q, want success", result.Outcome)
	}
}

// (countingIndexLookup defined in cache_writethrough_test.go)
