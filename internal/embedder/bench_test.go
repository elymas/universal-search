package embedder_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/embedder"
	"go.uber.org/goleak"
)

// TestMain uses goleak to detect goroutine leaks across all embedder tests (NFR-IDX-006).
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// BenchmarkClientEmbed measures single-call latency against a stub server.
func BenchmarkClientEmbed(b *testing.B) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Write a minimal valid response for benchmark speed.
		w.Write([]byte(`{"request_id":"bench","dense":[[0.1]],"model":"BAAI/bge-m3","model_version":"latest","device":"cpu","latency_ms":1.0,"cache_hits":0,"cache_misses":1}`))
	}))
	b.Cleanup(srv.Close)

	cfg := embedder.Config{
		BaseURL:        srv.URL,
		RequestTimeout: 5 * time.Second,
	}
	c, err := embedder.New(cfg, nil)
	if err != nil {
		b.Fatal(err)
	}

	req := embedder.Request{
		RequestID:   "bench",
		Texts:       []string{"benchmark text"},
		ReturnDense: true,
		BatchSize:   32,
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := c.Embed(context.Background(), req)
			if err != nil {
				b.Error(err)
			}
		}
	})
}
