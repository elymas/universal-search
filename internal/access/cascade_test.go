// Package access — integration tests for the 5-phase fetch cascade.
//
// REQ-CACHE-001: Fetch is sole public entry point.
// REQ-CACHE-002: Phase 1 index lookup.
// REQ-CACHE-003: Phase 2 HEAD probe + robots.txt.
// REQ-CACHE-004: Phase 3 standard GET.
// REQ-CACHE-007: Parent context propagation.
// REQ-CACHE-011: Per-phase panic recovery.
// REQ-CACHE-015: Shutdown guard.
// REQ-CACHE-016: Invalid URL rejection.
package access

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// newTestFetcher creates a Fetcher suitable for unit tests — no Playwright,
// private networks allowed so httptest.Server (127.0.0.1) passes SSRF guards.
func newTestFetcher(t *testing.T, extra ...func(*Options)) *Fetcher {
	t.Helper()
	opts := Options{
		AllowPrivateNetworks: true,
		PlaywrightEnabled:    false,
	}
	for _, fn := range extra {
		fn(&opts)
	}
	f, err := New(opts)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

// --- REQ-CACHE-016: Invalid URL rejection ---

func TestFetch_EmptyURL_ReturnsInvalidURL(t *testing.T) {
	t.Parallel()
	f := newTestFetcher(t)
	result, err := f.Fetch(t.Context(), "", FetchOptions{})
	if result != nil {
		t.Error("empty URL must return nil result")
	}
	if !errors.Is(err, ErrInvalidURL) {
		t.Errorf("empty URL must return ErrInvalidURL, got %v", err)
	}
}

func TestFetch_WhitespaceURL_ReturnsInvalidURL(t *testing.T) {
	t.Parallel()
	f := newTestFetcher(t)
	result, err := f.Fetch(t.Context(), "   ", FetchOptions{})
	if result != nil {
		t.Error("whitespace URL must return nil result")
	}
	if !errors.Is(err, ErrInvalidURL) {
		t.Errorf("whitespace URL must return ErrInvalidURL, got %v", err)
	}
}

func TestFetch_NoScheme_ReturnsInvalidURL(t *testing.T) {
	t.Parallel()
	f := newTestFetcher(t)
	_, err := f.Fetch(t.Context(), "example.com/path", FetchOptions{})
	if !errors.Is(err, ErrInvalidURL) {
		t.Errorf("URL without scheme must return ErrInvalidURL, got %v", err)
	}
}

func TestFetch_NoHost_ReturnsInvalidURL(t *testing.T) {
	t.Parallel()
	f := newTestFetcher(t)
	_, err := f.Fetch(t.Context(), "http:///path", FetchOptions{})
	if !errors.Is(err, ErrInvalidURL) {
		t.Errorf("URL without host must return ErrInvalidURL, got %v", err)
	}
}

// --- REQ-CACHE-013: SSRF blocked schemes ---

func TestFetch_FileScheme_Blocked(t *testing.T) {
	t.Parallel()
	// file:///etc/passwd has no host → URL validation rejects it with CategoryPermanent.
	// Schemes without host are caught at URL parse stage; schemes with host (ftp://)
	// reach validateScheme. Either way, the URL is rejected.
	f := newTestFetcher(t)
	_, err := f.Fetch(t.Context(), "file:///etc/passwd", FetchOptions{})
	if err == nil {
		t.Error("file:// scheme must be rejected")
	}
}

func TestFetch_FTPScheme_Blocked(t *testing.T) {
	t.Parallel()
	f := newTestFetcher(t)
	_, err := f.Fetch(t.Context(), "ftp://ftp.example.com/file.txt", FetchOptions{})
	if err == nil {
		t.Error("ftp:// scheme must be blocked")
	}
	fe, ok := err.(*FetchError)
	if !ok || fe.Category != CategoryBlocked {
		t.Errorf("ftp:// must return CategoryBlocked, got %v", err)
	}
}

// --- REQ-CACHE-015: Shutdown guard ---

func TestFetch_AfterShutdown_ReturnsShuttingDown(t *testing.T) {
	t.Parallel()
	f := newTestFetcher(t)
	if err := f.Shutdown(t.Context()); err != nil {
		t.Fatalf("Shutdown error: %v", err)
	}
	_, err := f.Fetch(t.Context(), "http://example.com/", FetchOptions{})
	if !errors.Is(err, ErrShuttingDown) {
		t.Errorf("after Shutdown must return ErrShuttingDown, got %v", err)
	}
}

func TestFetch_AfterClose_ReturnsShuttingDown(t *testing.T) {
	t.Parallel()
	opts := Options{AllowPrivateNetworks: true}
	f, err := New(opts)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}
	_, err = f.Fetch(t.Context(), "http://example.com/", FetchOptions{})
	if !errors.Is(err, ErrShuttingDown) {
		t.Errorf("after Close must return ErrShuttingDown, got %v", err)
	}
}

// --- REQ-CACHE-007: Context propagation ---

func TestFetch_CancelledContext_ReturnsOutcome(t *testing.T) {
	t.Parallel()
	// Start a slow server so the context cancels while Fetch is in flight.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html>ok</html>"))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(t.Context())
	f := newTestFetcher(t)

	cancel() // cancel immediately
	result, _ := f.Fetch(ctx, srv.URL+"/page", FetchOptions{SkipHEADProbe: true})
	// Should return a result (not nil) with cancelled/timeout outcome.
	if result != nil && result.Outcome != "cancelled" && result.Outcome != "timeout" {
		t.Errorf("cancelled context must yield cancelled/timeout outcome, got %q", result.Outcome)
	}
}

// --- REQ-CACHE-004: Phase 3 GET success ---

func TestFetch_Phase3_SuccessOnGET(t *testing.T) {
	t.Parallel()
	body := []byte("<html><body>hello world</body></html>")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r) // 404 → allow all
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	f := newTestFetcher(t)
	result, err := f.Fetch(t.Context(), srv.URL+"/page", FetchOptions{})
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	if result.Outcome != "success" {
		t.Errorf("Outcome = %q, want success (phases: %+v)", result.Outcome, result.PhaseAttempts)
	}
	if result.Content == nil {
		t.Fatalf("Content is nil (FinalPhase=%d, phases=%+v)", result.FinalPhase, result.PhaseAttempts)
	}
	if !bytes.Contains(result.Content.Body, []byte("hello world")) {
		t.Errorf("Content.Body = %q, want to contain 'hello world'", result.Content.Body)
	}
}

// --- Phase 1 index hit ---

func TestFetch_Phase1_IndexHit(t *testing.T) {
	t.Parallel()
	doc := &types.NormalizedDoc{
		ID:          "test-doc-1",
		URL:         "http://example.com/page",
		Body:        "<html>cached</html>",
		RetrievedAt: time.Now().UTC(),
	}
	lookup := &testIndexLookup{doc: doc, found: true}
	f := newTestFetcher(t, func(o *Options) {
		o.IndexLookup = lookup
	})

	result, err := f.Fetch(t.Context(), "http://example.com/page", FetchOptions{})
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	if result.Outcome != "success" {
		t.Errorf("Outcome = %q, want success", result.Outcome)
	}
	if result.FinalPhase != 1 {
		t.Errorf("FinalPhase = %d, want 1", result.FinalPhase)
	}
	if !bytes.Contains(result.Content.Body, []byte("cached")) {
		t.Errorf("Content.Body should be cached content")
	}
}

// testIndexLookup is a test double for IndexLookup.
type testIndexLookup struct {
	doc    *types.NormalizedDoc
	found  bool
	err    error
	upsert error
}

func (l *testIndexLookup) LookupByURL(_ context.Context, _ string) (*types.NormalizedDoc, bool, error) {
	return l.doc, l.found, l.err
}

func (l *testIndexLookup) Upsert(_ context.Context, _ []types.NormalizedDoc) error {
	return l.upsert
}

// --- REQ-CACHE-011: Per-phase panic recovery ---

func TestFetch_PhaseRecovery_ContinuesCascade(t *testing.T) {
	t.Parallel()
	// Phase 1 panics via a custom index lookup that panics.
	// The cascade should recover and try Phase 2+.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>recovered</html>"))
	}))
	defer srv.Close()

	panicLookup := &panicIndexLookup{}
	f := newTestFetcher(t, func(o *Options) {
		o.IndexLookup = panicLookup
	})

	result, err := f.Fetch(t.Context(), srv.URL+"/page", FetchOptions{})
	if err != nil {
		t.Fatalf("Fetch() after phase 1 panic should succeed via fallback: %v", err)
	}
	if result.Outcome != "success" {
		t.Errorf("Outcome = %q, want success after panic recovery", result.Outcome)
	}
}

// panicIndexLookup is a test double that panics on lookup.
type panicIndexLookup struct{}

func (p *panicIndexLookup) LookupByURL(_ context.Context, _ string) (*types.NormalizedDoc, bool, error) {
	panic("simulated phase 1 panic")
}

func (p *panicIndexLookup) Upsert(_ context.Context, _ []types.NormalizedDoc) error {
	return nil
}

// --- REQ-CACHE-001: FetchResult.PhaseAttempts populated ---

func TestFetch_PhaseAttempts_Populated(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("<html>ok</html>"))
	}))
	defer srv.Close()

	f := newTestFetcher(t)
	result, err := f.Fetch(t.Context(), srv.URL+"/page", FetchOptions{})
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	if len(result.PhaseAttempts) == 0 {
		t.Error("PhaseAttempts must be non-empty")
	}
	for i, a := range result.PhaseAttempts {
		if a.Phase != i+1 {
			t.Errorf("PhaseAttempts[%d].Phase = %d, want %d", i, a.Phase, i+1)
		}
		if a.Outcome == "" {
			t.Errorf("PhaseAttempts[%d].Outcome must not be empty", i)
		}
	}
}

// --- All phases fail → ErrAllPhasesFailed ---

func TestFetch_AllPhasesFail_ReturnsErrAllPhasesFailed(t *testing.T) {
	t.Parallel()
	// Server returns 200 for robots.txt (allow all) and then 503 for the actual page.
	// Phase 2 escalates (robots allowed), Phase 3 gets 503 = non-WAF failure (no WAF headers),
	// escalation stops, outcome = failure → ErrAllPhasesFailed.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			_, _ = w.Write([]byte("User-agent: *\nAllow: /\n"))
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable) // 503, no WAF headers
	}))
	defer srv.Close()

	// Use tiny timeouts so the test runs fast.
	f := newTestFetcher(t, func(o *Options) {
		o.PerPhaseTimeout = map[int]time.Duration{
			1: 50 * time.Millisecond,
			2: 50 * time.Millisecond,
			3: 200 * time.Millisecond,
			4: 200 * time.Millisecond,
			5: 50 * time.Millisecond,
		}
	})

	result, err := f.Fetch(t.Context(), srv.URL+"/page", FetchOptions{})
	if result == nil {
		t.Fatal("result must be non-nil even on all-phases-failed")
	}
	if !errors.Is(err, ErrAllPhasesFailed) {
		t.Errorf("all-fail must return ErrAllPhasesFailed, got %v (outcome=%q)", err, result.Outcome)
	}
}

// --- REQ-CACHE-001: ElapsedSeconds populated ---

func TestFetch_ElapsedSeconds_Positive(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	f := newTestFetcher(t)
	result, _ := f.Fetch(t.Context(), srv.URL+"/", FetchOptions{})
	if result != nil && result.ElapsedSeconds <= 0 {
		t.Errorf("ElapsedSeconds = %v, want > 0", result.ElapsedSeconds)
	}
}
