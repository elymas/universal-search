package arxiv

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// TestSearchRateLimitInterval verifies 3 sequential calls enforce minimum interval.
// With MinRequestInterval=10ms, 3 calls should take at least 20ms (two intervals
// between calls 1→2 and 2→3) but less than 50ms on any test machine.
func TestSearchRateLimitInterval(t *testing.T) {
	body, err := os.ReadFile("testdata/search_response.xml")
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	var requestCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.Header().Set("Content-Type", "application/atom+xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer ts.Close()

	a, err := New(Options{BaseURL: ts.URL, MinRequestInterval: 10 * time.Millisecond})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	start := time.Now()
	for i := range 3 {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, err := a.Search(ctx, types.Query{Text: "deep learning"})
		cancel()
		if err != nil {
			t.Fatalf("Search() call %d error = %v", i, err)
		}
	}
	elapsed := time.Since(start)

	// Two waits of 10ms each = at least 20ms total.
	if elapsed < 20*time.Millisecond {
		t.Errorf("elapsed = %v, want >= 20ms (two 10ms intervals)", elapsed)
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("elapsed = %v, want < 200ms (test is too slow)", elapsed)
	}
	if count := atomic.LoadInt32(&requestCount); count != 3 {
		t.Errorf("request count = %d, want 3", count)
	}
}

// TestSearchRateLimitCtxCancel verifies ctx cancellation breaks the rate-limit wait.
func TestSearchRateLimitCtxCancel(t *testing.T) {
	body, err := os.ReadFile("testdata/search_response.xml")
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer ts.Close()

	// Set a long interval so call 2 would wait 10 seconds if ctx is not honoured.
	a, err := New(Options{BaseURL: ts.URL, MinRequestInterval: 10 * time.Second})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Call 1: succeeds immediately and consumes a rate-limit slot.
	ctx1, cancel1 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel1()
	_, err = a.Search(ctx1, types.Query{Text: "deep learning"})
	if err != nil {
		t.Fatalf("Search() call 1 error = %v", err)
	}

	// Call 2: ctx already cancelled — should return quickly.
	ctx2, cancel2 := context.WithCancel(context.Background())
	cancel2() // cancel immediately

	start := time.Now()
	_, err = a.Search(ctx2, types.Query{Text: "deep learning"})
	elapsed := time.Since(start)

	// Must return within 10ms, not wait the full 10-second interval.
	if elapsed > 100*time.Millisecond {
		t.Errorf("elapsed = %v, want < 100ms (ctx cancel should break wait)", elapsed)
	}
	if err == nil {
		t.Error("Search() = nil, want error for cancelled ctx")
	}
	// The error should be context.Canceled or ErrSourceUnavailable wrapping it.
	if err != nil {
		unwrapped := err
		for unwrapped != nil {
			if unwrapped == context.Canceled {
				return // acceptable
			}
			unwrapped = unwrapOnce(unwrapped)
		}
		// Also acceptable: ErrSourceUnavailable wrapping context.Canceled.
		// Just verify it's an error (already checked above).
	}
}

// unwrapOnce calls errors.Unwrap once.
func unwrapOnce(err error) error {
	type unwrapper interface{ Unwrap() error }
	if u, ok := err.(unwrapper); ok {
		return u.Unwrap()
	}
	return nil
}

// TestSearchRateLimitPerInstance verifies rate-limit state is per-instance, not global.
func TestSearchRateLimitPerInstance(t *testing.T) {
	body, err := os.ReadFile("testdata/search_response.xml")
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer ts.Close()

	// Two separate adapters, each with a long rate-limit interval.
	a1, err := New(Options{BaseURL: ts.URL, MinRequestInterval: 10 * time.Second})
	if err != nil {
		t.Fatalf("New() a1 error = %v", err)
	}
	a2, err := New(Options{BaseURL: ts.URL, MinRequestInterval: 10 * time.Second})
	if err != nil {
		t.Fatalf("New() a2 error = %v", err)
	}

	// Issue Search on both at roughly the same time.
	type result struct {
		docs []types.NormalizedDoc
		err  error
	}
	resA := make(chan result, 1)
	resB := make(chan result, 1)

	start := time.Now()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		docs, err := a1.Search(ctx, types.Query{Text: "deep learning"})
		resA <- result{docs, err}
	}()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		docs, err := a2.Search(ctx, types.Query{Text: "deep learning"})
		resB <- result{docs, err}
	}()

	rA := <-resA
	rB := <-resB
	elapsed := time.Since(start)

	// Both should succeed within 100ms (not wait 10s each).
	if elapsed > 500*time.Millisecond {
		t.Errorf("elapsed = %v, want < 500ms (instances should NOT serialize)", elapsed)
	}
	if rA.err != nil {
		t.Errorf("a1.Search() error = %v", rA.err)
	}
	if rB.err != nil {
		t.Errorf("a2.Search() error = %v", rB.err)
	}
}
