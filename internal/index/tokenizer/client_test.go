package tokenizer_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/index/tokenizer"
)

// happyHandler returns a well-formed TokenizeResponse for any POST /tokenize.
func happyHandler(tokens []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/tokenize" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		joined := strings.Join(tokens, " ")
		resp := map[string]any{
			"request_id":     "test-req",
			"tokens":         tokens,
			"joined":         joined,
			"morpheme_count": len(tokens),
			"latency_ms":     1.0,
			"dict_version":   "mecab-ko-1.0",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

// TestClientTokenize_HappyPath verifies a successful round trip.
// REQ-IDX-003-006
func TestClientTokenize_HappyPath(t *testing.T) {
	t.Parallel()
	tokens := []string{"안녕", "하세요"}
	srv := httptest.NewServer(happyHandler(tokens))
	defer srv.Close()

	cfg := tokenizer.Config{
		BaseURL:        srv.URL,
		Timeout:        2 * time.Second,
		MaxRetries:     0,
		RetryBaseDelay: 10 * time.Millisecond,
	}
	client := tokenizer.NewClient(cfg)

	result, err := client.Tokenize(context.Background(), "req-1", "안녕하세요")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Tokens) != 2 {
		t.Errorf("expected 2 tokens, got %d", len(result.Tokens))
	}
	if result.Joined != "안녕 하세요" {
		t.Errorf("unexpected joined: %q", result.Joined)
	}
	if result.MorphemeCount != 2 {
		t.Errorf("expected morpheme_count=2, got %d", result.MorphemeCount)
	}
}

// TestClientTokenize_InvalidInput verifies ErrInvalidInput on 400 responses.
// REQ-IDX-003-004
func TestClientTokenize_InvalidInput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_input","detail":"text"}`))
	}))
	defer srv.Close()

	cfg := tokenizer.Config{
		BaseURL:        srv.URL,
		Timeout:        2 * time.Second,
		MaxRetries:     0,
		RetryBaseDelay: 10 * time.Millisecond,
	}
	client := tokenizer.NewClient(cfg)

	_, err := client.Tokenize(context.Background(), "req-bad", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid input") {
		t.Errorf("expected ErrInvalidInput, got: %v", err)
	}
}

// TestClientTokenize_RetriesOnTransientFailure verifies up to MaxRetries attempts.
// REQ-IDX-003-006: exponential backoff 100ms/300ms ± 10% jitter.
func TestClientTokenize_RetriesOnTransientFailure(t *testing.T) {
	t.Parallel()
	var callCount atomic.Int32
	tokens := []string{"서울"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		if n < 3 {
			// First two calls fail with 503.
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		happyHandler(tokens)(w, r)
	}))
	defer srv.Close()

	cfg := tokenizer.Config{
		BaseURL:        srv.URL,
		Timeout:        5 * time.Second,
		MaxRetries:     2,
		RetryBaseDelay: 1 * time.Millisecond, // fast for tests
	}
	client := tokenizer.NewClient(cfg)

	result, err := client.Tokenize(context.Background(), "req-retry", "서울")
	if err != nil {
		t.Fatalf("unexpected error after retry: %v", err)
	}
	if callCount.Load() != 3 {
		t.Errorf("expected 3 calls, got %d", callCount.Load())
	}
	_ = result
}

// TestClientTokenize_SidecarUnreachable verifies ErrSidecarUnreachable when all retries fail.
func TestClientTokenize_SidecarUnreachable(t *testing.T) {
	t.Parallel()
	// Use a server that immediately closes connections.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	cfg := tokenizer.Config{
		BaseURL:        srv.URL,
		Timeout:        2 * time.Second,
		MaxRetries:     1,
		RetryBaseDelay: 1 * time.Millisecond,
	}
	client := tokenizer.NewClient(cfg)

	_, err := client.Tokenize(context.Background(), "req-fail", "test")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unreachable") && !strings.Contains(err.Error(), "503") {
		t.Errorf("expected unreachable/503 error, got: %v", err)
	}
}

// TestClientTokenize_ContextCancellation verifies early exit on ctx cancel.
func TestClientTokenize_ContextCancellation(t *testing.T) {
	t.Parallel()
	// Slow server that sleeps.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := tokenizer.Config{
		BaseURL:        srv.URL,
		Timeout:        10 * time.Second,
		MaxRetries:     0,
		RetryBaseDelay: 1 * time.Millisecond,
	}
	client := tokenizer.NewClient(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.Tokenize(ctx, "req-cancel", "서울")
	if err == nil {
		t.Fatal("expected error from context cancellation, got nil")
	}
}

// TestClientTokenize_RequestIDEchoed verifies request_id propagation.
func TestClientTokenize_RequestIDEchoed(t *testing.T) {
	t.Parallel()
	wantID := "my-unique-req-id"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		resp := map[string]any{
			"request_id":     body["request_id"],
			"tokens":         []string{"서울"},
			"joined":         "서울",
			"morpheme_count": 1,
			"latency_ms":     0.5,
			"dict_version":   "test",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cfg := tokenizer.Config{
		BaseURL:        srv.URL,
		Timeout:        2 * time.Second,
		MaxRetries:     0,
		RetryBaseDelay: 1 * time.Millisecond,
	}
	client := tokenizer.NewClient(cfg)

	result, err := client.Tokenize(context.Background(), wantID, "서울")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequestID != wantID {
		t.Errorf("request_id not echoed: got %q, want %q", result.RequestID, wantID)
	}
}

// TestDefaultConfig verifies env-var binding with defaults.
func TestDefaultConfig(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv.
	t.Setenv("TOKENIZER_KO_BASE_URL", "http://custom:9999")
	t.Setenv("TOKENIZER_KO_TIMEOUT_MS", "750")
	t.Setenv("TOKENIZER_KO_MAX_RETRIES", "3")

	cfg := tokenizer.DefaultConfig()
	if cfg.BaseURL != "http://custom:9999" {
		t.Errorf("BaseURL: got %q", cfg.BaseURL)
	}
	if cfg.Timeout != 750*time.Millisecond {
		t.Errorf("Timeout: got %v", cfg.Timeout)
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries: got %d", cfg.MaxRetries)
	}
}
