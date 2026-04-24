// Package bench_test provides NFR-OBS-001 benchmarks for the observability
// middleware overhead.
//
// NFR-OBS-001: Instrumented HTTP handler overhead <= 1ms p99.
// These benchmarks establish baseline vs instrumented vs traced comparisons.
package bench_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/elymas/universal-search/internal/obs/metrics"
	obstrace "github.com/elymas/universal-search/internal/obs/trace"
	"go.opentelemetry.io/otel"
)

// stub handler that writes 200 OK immediately.
var stubHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

// BenchmarkHTTPStubBaseline measures the raw httptest round-trip overhead
// without any instrumentation.
func BenchmarkHTTPStubBaseline(b *testing.B) {
	srv := httptest.NewServer(stubHandler)
	defer srv.Close()

	b.ResetTimer()
	for b.Loop() {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

// BenchmarkHTTPStubInstrumented measures the overhead of the Prometheus
// HTTPMiddleware wrapping the stub handler.
func BenchmarkHTTPStubInstrumented(b *testing.B) {
	reg := metrics.NewRegistry()
	handler := metrics.HTTPMiddleware(reg, "/bench", stubHandler)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	b.ResetTimer()
	for b.Loop() {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/bench", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

// BenchmarkHTTPStubTracedSampled measures the overhead of an always-sampled
// OTel span wrapping an HTTP handler (no OTLP export — no-op provider).
func BenchmarkHTTPStubTracedSampled(b *testing.B) {
	// Use no-op provider so we measure OTel API overhead without network I/O.
	_, err := obstrace.Init(context.Background(), obstrace.Config{
		ServiceName:  "bench",
		OTLPEndpoint: "", // no-op
	})
	if err != nil {
		b.Fatal(err)
	}

	tr := otel.Tracer("bench")
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tr.Start(r.Context(), "bench-span")
		_ = ctx
		stubHandler.ServeHTTP(w, r)
		span.End()
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	b.ResetTimer()
	for b.Loop() {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}
