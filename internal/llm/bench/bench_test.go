// Package bench — LLM client throughput benchmark.
// NFR-LLM-001: ≥100 RPS throughput target against a local stub.
package bench_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/elymas/universal-search/internal/llm"
	"github.com/elymas/universal-search/internal/llm/config"
	"github.com/elymas/universal-search/internal/obs"
)

// response is a pre-built valid chat completion JSON body.
var response = []byte(`{"id":"chatcmpl-bench","object":"chat.completion","model":"claude-sonnet-4-6","choices":[{"index":0,"message":{"role":"assistant","content":"bench"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":1,"total_tokens":6}}`)

func newBenchClient(b *testing.B, serverURL string) llm.Client {
	b.Helper()
	cfg := config.Config{
		BaseURL:          serverURL,
		MasterKey:        "bench-key",
		PerRequestCapUSD: 0, // unlimited
		TimeoutSeconds:   10,
	}
	o, _, err := obs.Init(context.Background(), obs.Config{
		ServiceName: "bench",
		LogLevel:    "ERROR", // suppress logs during bench
	})
	if err != nil {
		b.Fatalf("obs.Init: %v", err)
	}
	c, err := llm.New(cfg, o)
	if err != nil {
		b.Fatalf("llm.New: %v", err)
	}
	b.Cleanup(func() { _ = c.Close() })
	return c
}

// BenchmarkComplete measures Complete throughput against an in-process stub.
// NFR-LLM-001: Target ≥100 RPS (10ms/op or better).
func BenchmarkComplete(b *testing.B) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(response)
	}))
	defer srv.Close()

	c := newBenchClient(b, srv.URL)
	req := llm.Request{
		Class:    llm.Summary,
		Messages: []llm.Message{{Role: "user", Content: "bench prompt"}},
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := c.Complete(context.Background(), req)
			if err != nil {
				b.Errorf("Complete: %v", err)
			}
		}
	})
}

// BenchmarkCompleteSerial measures serial (single-goroutine) latency.
func BenchmarkCompleteSerial(b *testing.B) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(response)
	}))
	defer srv.Close()

	c := newBenchClient(b, srv.URL)
	req := llm.Request{
		Class:    llm.Summary,
		Messages: []llm.Message{{Role: "user", Content: "bench prompt"}},
	}

	b.ResetTimer()
	for range b.N {
		_, err := c.Complete(context.Background(), req)
		if err != nil {
			b.Errorf("Complete: %v", err)
		}
	}
}
