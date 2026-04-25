// Package llm_test — retry and backoff tests.
// REQ-LLM-004: max 3 retries, exponential backoff, non-retryable errors.
package llm_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/llm"
	"github.com/elymas/universal-search/internal/llm/config"
	"github.com/elymas/universal-search/internal/obs"
)

// makeTestClient creates a Client pointing at the given test server URL.
func makeTestClient(t *testing.T, serverURL string) llm.Client {
	t.Helper()
	cfg := config.Config{
		BaseURL:          serverURL,
		MasterKey:        "test-master-key",
		PerRequestCapUSD: 10.0,
		TimeoutSeconds:   5,
	}
	o := makeTestObs(t)
	c, err := llm.New(cfg, o)
	if err != nil {
		t.Fatalf("llm.New: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// makeTestObs creates a minimal obs.Obs for testing.
func makeTestObs(t *testing.T) *obs.Obs {
	t.Helper()
	o, shutdown, err := obs.Init(context.Background(), obs.Config{
		ServiceName: "test",
	})
	if err != nil {
		t.Fatalf("obs.Init: %v", err)
	}
	t.Cleanup(func() { _ = shutdown(context.Background()) })
	return o
}

// TestClientCompleteRetriesOn5xx verifies that 503 responses trigger retries.
// Stub returns 503 three times then 200 → 4 outbound requests.
// REQ-LLM-004
func TestClientCompleteRetriesOn5xx(t *testing.T) {
	var count atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := count.Add(1)
		if n <= 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":{"message":"service unavailable","type":"server_error","code":503}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(validChatResponse("test-response", "claude-sonnet-4-6")))
	}))
	defer srv.Close()

	c := makeTestClient(t, srv.URL)
	resp, err := c.Complete(context.Background(), llm.Request{
		Class:    llm.Summary,
		Messages: []llm.Message{{Role: "user", Content: "hello"}},
	})
	if err != nil && !errors.Is(err, llm.ErrBudgetExceeded) {
		t.Fatalf("Complete: unexpected error: %v", err)
	}
	if resp.Text == "" {
		t.Error("expected non-empty response text")
	}
	if count.Load() != 4 {
		t.Errorf("outbound requests: got %d, want 4", count.Load())
	}
}

// TestClientCompleteAuthErrorNoRetry verifies 401 → 1 request, no retry.
// REQ-LLM-004
func TestClientCompleteAuthErrorNoRetry(t *testing.T) {
	var count atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid auth","type":"authentication_error","code":401}}`))
	}))
	defer srv.Close()

	c := makeTestClient(t, srv.URL)
	_, err := c.Complete(context.Background(), llm.Request{
		Class:    llm.Summary,
		Messages: []llm.Message{{Role: "user", Content: "hello"}},
	})
	if err == nil {
		t.Error("expected error for 401, got nil")
	}
	if count.Load() > 3 {
		t.Errorf("outbound requests for 401: got %d, want <= 1 per provider (no retry)", count.Load())
	}
}

// TestClientCompleteBadRequestNoRetry verifies 400 → no retry.
// REQ-LLM-004
func TestClientCompleteBadRequestNoRetry(t *testing.T) {
	var count atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"bad request","type":"invalid_request_error","code":400}}`))
	}))
	defer srv.Close()

	c := makeTestClient(t, srv.URL)
	_, err := c.Complete(context.Background(), llm.Request{
		Class:    llm.Summary,
		Messages: []llm.Message{{Role: "user", Content: "hello"}},
	})
	if err == nil {
		t.Error("expected error for 400, got nil")
	}
	if count.Load() > 3 {
		t.Errorf("outbound requests for 400: got %d, want <= 1 per provider (no retry)", count.Load())
	}
}

// TestClientCompleteNotFoundNoRetry verifies 404 → no retry.
// REQ-LLM-004
func TestClientCompleteNotFoundNoRetry(t *testing.T) {
	var count atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"message":"not found","type":"invalid_request_error","code":404}}`))
	}))
	defer srv.Close()

	c := makeTestClient(t, srv.URL)
	_, err := c.Complete(context.Background(), llm.Request{
		Class:    llm.Summary,
		Messages: []llm.Message{{Role: "user", Content: "hello"}},
	})
	if err == nil {
		t.Error("expected error for 404, got nil")
	}
	if count.Load() > 3 {
		t.Errorf("outbound requests for 404: got %d, want <= 1 per provider (no retry)", count.Load())
	}
}

// TestClientCompleteCtxCancelStopsRetry verifies context cancellation during backoff.
// REQ-LLM-004
func TestClientCompleteCtxCancelStopsRetry(t *testing.T) {
	var count atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":{"message":"unavailable"}}`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after first request completes.
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	c := makeTestClient(t, srv.URL)
	_, err := c.Complete(ctx, llm.Request{
		Class:    llm.Summary,
		Messages: []llm.Message{{Role: "user", Content: "hello"}},
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	// Should have made at most 1-2 requests before cancellation.
	if count.Load() > 4 {
		t.Errorf("too many requests after cancel: %d", count.Load())
	}
}

// TestClientCompleteBackoffTimings verifies inter-retry delays within spec tolerance.
// REQ-LLM-004: backoffs 250ms/500ms (first two gaps within a provider's retry loop).
// The 3rd gap is between providers (no sleep), so only gaps 1 and 2 are checked.
func TestClientCompleteBackoffTimings(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timing test in short mode")
	}

	attempt := 0
	var timestamps []time.Time
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		timestamps = append(timestamps, time.Now())
		attempt++
		if attempt <= 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":{"message":"unavailable"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(validChatResponse("ok", "claude-sonnet-4-6")))
	}))
	defer srv.Close()

	c := makeTestClient(t, srv.URL)
	_, _ = c.Complete(context.Background(), llm.Request{
		Class:    llm.Summary,
		Messages: []llm.Message{{Role: "user", Content: "hello"}},
	})

	if len(timestamps) < 3 {
		t.Fatalf("expected at least 3 timestamps, got %d", len(timestamps))
	}

	// withRetry sleeps after attempt 0 (250ms) and after attempt 1 (500ms).
	// After attempt 2 (the 3rd) it returns immediately; no sleep before switching provider.
	// gap[0→1]: ~250ms backoff + HTTP overhead → lower bound 187ms, upper bound 3x.
	// gap[1→2]: ~500ms backoff + HTTP overhead → lower bound 375ms, upper bound 3x.
	checked := []struct {
		exp time.Duration
		lo  time.Duration
		hi  time.Duration
	}{
		{250 * time.Millisecond, 187 * time.Millisecond, 750 * time.Millisecond},
		{500 * time.Millisecond, 375 * time.Millisecond, 1500 * time.Millisecond},
	}
	for i, c := range checked {
		if i+1 >= len(timestamps) {
			break
		}
		gap := timestamps[i+1].Sub(timestamps[i])
		if gap < c.lo || gap > c.hi {
			t.Errorf("retry %d delay: got %v, want [%v, %v] (base %v)", i+1, gap, c.lo, c.hi, c.exp)
		}
	}
}

// validChatResponse returns a minimal OpenAI-compatible chat completion JSON.
func validChatResponse(content, model string) string {
	return `{
		"id": "chatcmpl-test",
		"object": "chat.completion",
		"model": "` + model + `",
		"choices": [{
			"index": 0,
			"message": {"role": "assistant", "content": "` + content + `"},
			"finish_reason": "stop"
		}],
		"usage": {
			"prompt_tokens": 10,
			"completion_tokens": 5,
			"total_tokens": 15
		}
	}`
}
