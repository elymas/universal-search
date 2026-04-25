// Package llm_test — client observability, secret handling, and cost tests.
// REQ-LLM-002: Complete/Stream/Embed round-trips.
// REQ-LLM-003: Per-call slog event, counter, histogram, OTel span.
// REQ-LLM-005: Bearer auth; key never logged.
// REQ-LLM-006: Cost header extraction.
// NFR-LLM-003: Budget cap.
package llm_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/llm"
	"github.com/elymas/universal-search/internal/llm/config"
	"github.com/elymas/universal-search/internal/obs"
)

// makeClientWithLogWriter creates a client whose slog output goes to w.
func makeClientWithLogWriter(t *testing.T, serverURL string, w io.Writer) llm.Client {
	t.Helper()
	cfg := config.Config{
		BaseURL:          serverURL,
		MasterKey:        "test-master-key",
		PerRequestCapUSD: 10.0,
		TimeoutSeconds:   5,
	}
	o, shutdown, err := obs.Init(context.Background(), obs.Config{
		ServiceName: "test-client",
		LogLevel:    "DEBUG",
		LogWriter:   w,
	})
	if err != nil {
		t.Fatalf("obs.Init: %v", err)
	}
	t.Cleanup(func() { _ = shutdown(context.Background()) })

	c, err := llm.New(cfg, o)
	if err != nil {
		t.Fatalf("llm.New: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// TestClientCompleteRoundTripsToLiteLLM verifies a successful complete round-trip.
// REQ-LLM-002
func TestClientCompleteRoundTripsToLiteLLM(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(validChatResponse("hello world", "claude-sonnet-4-6")))
	}))
	defer srv.Close()

	c := makeTestClient(t, srv.URL)
	resp, err := c.Complete(context.Background(), llm.Request{
		Class:    llm.Summary,
		Messages: []llm.Message{{Role: "user", Content: "hello"}},
	})
	if err != nil && !errors.Is(err, llm.ErrBudgetExceeded) {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Text != "hello world" {
		t.Errorf("text: got %q, want %q", resp.Text, "hello world")
	}
	if resp.Provider == "" {
		t.Error("provider must not be empty")
	}
	if resp.Model == "" {
		t.Error("model must not be empty")
	}
	if resp.PromptTokens <= 0 {
		t.Errorf("prompt_tokens: got %d, want > 0", resp.PromptTokens)
	}
	if resp.LatencyMs < 0 {
		t.Errorf("latency_ms: got %d, want >= 0", resp.LatencyMs)
	}
}

// TestClientCompleteEmitsSlogEvent verifies slog INFO event with provider/model/tokens/latency.
// REQ-LLM-003
func TestClientCompleteEmitsSlogEvent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(validChatResponse("ok", "claude-sonnet-4-6")))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	c := makeClientWithLogWriter(t, srv.URL, &buf)
	_, _ = c.Complete(context.Background(), llm.Request{
		Class:    llm.Summary,
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})

	logOutput := buf.String()
	for _, want := range []string{"llm call", "provider", "model", "prompt_tokens", "latency_ms"} {
		if !strings.Contains(logOutput, want) {
			t.Errorf("slog output missing %q; got:\n%s", want, logOutput)
		}
	}
}

// TestNoMasterKeyInLogs verifies the API key is never written to logs.
// REQ-LLM-005
func TestNoMasterKeyInLogs(t *testing.T) {
	const secretKey = "super-secret-master-key-12345"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"internal error with key super-secret-master-key-12345","type":"server_error"}}`))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	cfg := config.Config{
		BaseURL:          srv.URL,
		MasterKey:        secretKey,
		PerRequestCapUSD: 10.0,
		TimeoutSeconds:   5,
	}
	o, shutdown, err := obs.Init(context.Background(), obs.Config{
		ServiceName: "test-secret",
		LogLevel:    "DEBUG",
		LogWriter:   &buf,
	})
	if err != nil {
		t.Fatalf("obs.Init: %v", err)
	}
	defer func() { _ = shutdown(context.Background()) }()

	c, err := llm.New(cfg, o)
	if err != nil {
		t.Fatalf("llm.New: %v", err)
	}
	defer func() { _ = c.Close() }()

	_, _ = c.Complete(context.Background(), llm.Request{
		Class:    llm.Summary,
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})

	if strings.Contains(buf.String(), secretKey) {
		t.Errorf("master key leaked into logs:\n%s", buf.String())
	}
}

// TestAuthBearerHeaderSent verifies the Authorization header is set for outbound requests.
// REQ-LLM-005
func TestAuthBearerHeaderSent(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(validChatResponse("ok", "claude-sonnet-4-6")))
	}))
	defer srv.Close()

	c := makeTestClient(t, srv.URL)
	_, _ = c.Complete(context.Background(), llm.Request{
		Class:    llm.Summary,
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})

	if !strings.HasPrefix(gotAuth, "Bearer ") {
		t.Errorf("Authorization header: got %q, want Bearer ...", gotAuth)
	}
}

// TestClientCompleteExtractsCostHeader verifies x-litellm-response-cost parsing.
// REQ-LLM-006
func TestClientCompleteExtractsCostHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("x-litellm-response-cost", "0.001234")
		_, _ = w.Write([]byte(validChatResponse("cost test", "claude-sonnet-4-6")))
	}))
	defer srv.Close()

	c := makeTestClient(t, srv.URL)
	resp, err := c.Complete(context.Background(), llm.Request{
		Class:    llm.Summary,
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil && !errors.Is(err, llm.ErrBudgetExceeded) {
		t.Fatalf("Complete: %v", err)
	}
	if resp.CostUSD < 0.001 || resp.CostUSD > 0.002 {
		t.Errorf("CostUSD: got %f, want ~0.001234", resp.CostUSD)
	}
}

// TestBudgetCapExceededReturnsResponseAndError verifies NFR-LLM-003.
// ErrBudgetExceeded must be returned alongside (not instead of) the response.
// NFR-LLM-003
func TestBudgetCapExceededReturnsResponseAndError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("x-litellm-response-cost", "99.99")
		_, _ = w.Write([]byte(validChatResponse("expensive", "claude-sonnet-4-6")))
	}))
	defer srv.Close()

	cfg := config.Config{
		BaseURL:          srv.URL,
		MasterKey:        "test-master-key",
		PerRequestCapUSD: 0.001, // very low cap
		TimeoutSeconds:   5,
	}
	o := makeTestObs(t)
	c, err := llm.New(cfg, o)
	if err != nil {
		t.Fatalf("llm.New: %v", err)
	}
	defer func() { _ = c.Close() }()

	resp, err := c.Complete(context.Background(), llm.Request{
		Class:    llm.Summary,
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})

	if !errors.Is(err, llm.ErrBudgetExceeded) {
		t.Errorf("expected ErrBudgetExceeded, got %v", err)
	}
	// Response must not be discarded (NFR-LLM-003 HARD rule).
	if resp.Text == "" {
		t.Error("response text must not be empty even when budget exceeded")
	}
}

// TestMissingCostHeaderGraceful verifies that missing cost header does not error.
// REQ-LLM-006
func TestMissingCostHeaderGraceful(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(validChatResponse("no cost", "claude-sonnet-4-6")))
	}))
	defer srv.Close()

	c := makeTestClient(t, srv.URL)
	resp, err := c.Complete(context.Background(), llm.Request{
		Class:    llm.Summary,
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil && !errors.Is(err, llm.ErrBudgetExceeded) {
		t.Fatalf("Complete: %v", err)
	}
	if resp.CostUSD != 0 {
		t.Errorf("CostUSD with missing header: got %f, want 0", resp.CostUSD)
	}
}

// TestMalformedCostHeaderGraceful verifies malformed cost header does not fail the call.
// REQ-LLM-006
func TestMalformedCostHeaderGraceful(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("x-litellm-response-cost", "not-a-number")
		_, _ = w.Write([]byte(validChatResponse("malformed", "claude-sonnet-4-6")))
	}))
	defer srv.Close()

	c := makeTestClient(t, srv.URL)
	resp, err := c.Complete(context.Background(), llm.Request{
		Class:    llm.Summary,
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil && !errors.Is(err, llm.ErrBudgetExceeded) {
		t.Fatalf("Complete: %v", err)
	}
	if resp.CostUSD != 0 {
		t.Errorf("CostUSD with malformed header: got %f, want 0", resp.CostUSD)
	}
}

// TestClientCloseIsIdempotent verifies Close does not panic or error.
// REQ-LLM-002
func TestClientCloseIsIdempotent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := makeTestClient(t, srv.URL)
	if err := c.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}

// TestClientEmbedRoundTrip verifies embedding request.
// REQ-LLM-002
func TestClientEmbedRoundTrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"object": "list",
			"data": [
				{"object": "embedding", "index": 0, "embedding": [0.1, 0.2, 0.3]},
				{"object": "embedding", "index": 1, "embedding": [0.4, 0.5, 0.6]}
			],
			"model": "text-embedding-3-large",
			"usage": {"prompt_tokens": 4, "total_tokens": 4}
		}`))
	}))
	defer srv.Close()

	c := makeTestClient(t, srv.URL)
	resp, err := c.Embed(context.Background(), llm.EmbedRequest{
		Class: llm.Embed,
		Input: []string{"foo", "bar"},
	})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(resp.Vectors) != 2 {
		t.Errorf("vectors: got %d, want 2", len(resp.Vectors))
	}
	if len(resp.Vectors[0]) != 3 {
		t.Errorf("vector[0] dim: got %d, want 3", len(resp.Vectors[0]))
	}
	if resp.PromptTokens <= 0 {
		t.Errorf("prompt_tokens: got %d, want > 0", resp.PromptTokens)
	}
}

// TestClientStreamBasic verifies stream returns deltas and closes channel.
// REQ-LLM-008
func TestClientStreamBasic(t *testing.T) {
	sseBody := "data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"model\":\"claude-sonnet-4-6\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"hello\"},\"finish_reason\":null}]}\n\n" +
		"data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"model\":\"claude-sonnet-4-6\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\" world\"},\"finish_reason\":null}]}\n\n" +
		"data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"model\":\"claude-sonnet-4-6\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
		"data: [DONE]\n\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = io.WriteString(w, sseBody)
	}))
	defer srv.Close()

	c := makeTestClient(t, srv.URL)
	ch, err := c.Stream(context.Background(), llm.Request{
		Class:    llm.Summary,
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var content strings.Builder
	timeout := time.After(5 * time.Second)
	for {
		select {
		case d, ok := <-ch:
			if !ok {
				goto done
			}
			if d.Err != nil {
				t.Errorf("stream delta error: %v", d.Err)
			}
			content.WriteString(d.Content)
		case <-timeout:
			t.Fatal("stream timed out")
		}
	}
done:
	if content.String() == "" {
		t.Error("expected non-empty streamed content")
	}
}

// TestRedactKeyInErrorMessages verifies API key is replaced with [REDACTED] in error text.
// REQ-LLM-005
func TestRedactKeyInErrorMessages(t *testing.T) {
	const secretKey = "my-very-secret-key-abc123"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"error with my-very-secret-key-abc123","type":"server_error"}}`))
	}))
	defer srv.Close()

	cfg := config.Config{
		BaseURL:          srv.URL,
		MasterKey:        secretKey,
		PerRequestCapUSD: 10.0,
		TimeoutSeconds:   5,
	}
	o := makeTestObs(t)
	c, err := llm.New(cfg, o)
	if err != nil {
		t.Fatalf("llm.New: %v", err)
	}
	defer func() { _ = c.Close() }()

	_, callErr := c.Complete(context.Background(), llm.Request{
		Class:    llm.Summary,
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})

	if callErr != nil && strings.Contains(callErr.Error(), secretKey) {
		t.Errorf("master key leaked into error message: %v", callErr)
	}
}

// TestClientCompleteUnknownClassFails verifies ErrModelNotConfigured for unknown class.
// REQ-LLM-002
func TestClientCompleteUnknownClassFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("{}"))
	}))
	defer srv.Close()

	c := makeTestClient(t, srv.URL)
	_, err := c.Complete(context.Background(), llm.Request{
		Class:    llm.ModelClass("unknown-class"),
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})
	if !errors.Is(err, llm.ErrModelNotConfigured) {
		t.Errorf("expected ErrModelNotConfigured, got %v", err)
	}
}

// TestClientCompleteModelOverride verifies req.Override replaces the default model.
// REQ-LLM-002
func TestClientCompleteModelOverride(t *testing.T) {
	var gotModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bodyStr := string(body)
		if idx := strings.Index(bodyStr, `"model":"`); idx >= 0 {
			rest := bodyStr[idx+len(`"model":"`):]
			end := strings.Index(rest, `"`)
			if end >= 0 {
				gotModel = rest[:end]
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(validChatResponse("overridden", gotModel)))
	}))
	defer srv.Close()

	c := makeTestClient(t, srv.URL)
	_, _ = c.Complete(context.Background(), llm.Request{
		Class:    llm.Summary,
		Override: "custom-model-v2",
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})

	if gotModel != "custom-model-v2" {
		t.Errorf("model override: got %q, want %q", gotModel, "custom-model-v2")
	}
}

// TestClientCompleteSystemPrompt verifies System field is included in messages.
// REQ-LLM-002
func TestClientCompleteSystemPrompt(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(validChatResponse("sys test", "claude-sonnet-4-6")))
	}))
	defer srv.Close()

	c := makeTestClient(t, srv.URL)
	_, _ = c.Complete(context.Background(), llm.Request{
		Class:    llm.Summary,
		System:   "You are a helpful assistant.",
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})

	if !strings.Contains(gotBody, "You are a helpful assistant.") {
		t.Errorf("system prompt not found in request body: %s", gotBody)
	}
}
