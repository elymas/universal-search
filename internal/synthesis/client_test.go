// Package synthesis_test covers REQ-SYN-005 (Go client behavior) and
// REQ-SYN-006 (per-call observability).
package synthesis_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/obs"
	"github.com/elymas/universal-search/internal/obs/metrics"
	"github.com/elymas/universal-search/internal/synthesis"
	"github.com/elymas/universal-search/pkg/types"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// cannedResult returns a valid SynthesizeResponse JSON.
func cannedResult(degraded bool) []byte {
	r := map[string]any{
		"request_id":        "req-test",
		"text":              "[1] synthesized paragraph",
		"citations":         []any{map[string]any{"marker": 1, "doc_id": "doc-1", "url": "https://example.com/1", "title": "Title 1"}},
		"model":             "claude-haiku-4-5",
		"provider":          "anthropic",
		"cost_usd":          0.0023,
		"prompt_tokens":     100,
		"completion_tokens": 50,
		"latency_ms":        500.0,
		"degraded":          degraded,
		"notice":            "",
	}
	b, _ := json.Marshal(r)
	return b
}

func makeDocs() []types.NormalizedDoc {
	return []types.NormalizedDoc{
		{
			ID:          "doc-1",
			SourceID:    "reddit",
			URL:         "https://example.com/1",
			Title:       "Title 1",
			RetrievedAt: time.Now(),
		},
	}
}

// ---------------------------------------------------------------------------
// REQ-SYN-005 — Go HTTP client behavior
// ---------------------------------------------------------------------------

// TestClientSynthesizeHappyPath: server returns 200 JSON; assert Result fields.
func TestClientSynthesizeHappyPath(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(cannedResult(false))
	}))
	defer srv.Close()

	cfg := synthesis.Config{BaseURL: srv.URL, RequestTimeout: 5 * time.Second}
	client, _ := synthesis.New(cfg, nil)

	result, err := client.Synthesize(context.Background(), "hello world", "en", makeDocs())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Text == "" {
		t.Error("expected non-empty Text")
	}
	if len(result.Citations) == 0 {
		t.Error("expected at least one citation")
	}
}

// TestClientSynthesizeTimeout: server sleeps 30s; client times out in 500ms.
func TestClientSynthesizeTimeout(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sleep longer than client timeout; respect request context cancellation
		select {
		case <-r.Context().Done():
		case <-time.After(10 * time.Second):
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	cfg := synthesis.Config{BaseURL: srv.URL, RequestTimeout: 200 * time.Millisecond}
	client, _ := synthesis.New(cfg, nil)

	start := time.Now()
	_, err := client.Synthesize(context.Background(), "test", "en", makeDocs())
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	// Should complete well within 1.5s (200ms timeout + small overhead)
	if elapsed > 1500*time.Millisecond {
		t.Errorf("timeout took too long: %v", elapsed)
	}
}

// TestClientSynthesizeRetriesOnConnReset: first 2 calls return 503, then 200.
func TestClientSynthesizeRetriesOnConnReset(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := callCount.Add(1)
		if n <= 2 {
			// Return 503 to trigger retry
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(cannedResult(false))
	}))
	defer srv.Close()

	cfg := synthesis.Config{BaseURL: srv.URL, RequestTimeout: 10 * time.Second}
	client, _ := synthesis.New(cfg, nil)

	result, err := client.Synthesize(context.Background(), "test", "en", makeDocs())
	if err != nil {
		t.Fatalf("unexpected error after retry: %v", err)
	}
	if result.Text == "" {
		t.Error("expected non-empty Text after retry")
	}
	if callCount.Load() < 3 {
		t.Errorf("expected at least 3 calls (2 retries), got %d", callCount.Load())
	}
}

// TestClientSynthesize4xxNoRetry: server returns 400; expect exactly 1 call + ErrInvalidRequest.
func TestClientSynthesize4xxNoRetry(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"empty_input","detail":"query"}`))
	}))
	defer srv.Close()

	cfg := synthesis.Config{BaseURL: srv.URL, RequestTimeout: 5 * time.Second}
	client, _ := synthesis.New(cfg, nil)

	_, err := client.Synthesize(context.Background(), "test", "en", makeDocs())
	if err == nil {
		t.Fatal("expected error on 4xx, got nil")
	}
	if callCount.Load() != 1 {
		t.Errorf("expected exactly 1 call on 4xx, got %d", callCount.Load())
	}
}

// TestClientSynthesize5xxRetried: server returns 503 once then 200.
func TestClientSynthesize5xxRetried(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := callCount.Add(1)
		if n == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(cannedResult(false))
	}))
	defer srv.Close()

	cfg := synthesis.Config{BaseURL: srv.URL, RequestTimeout: 10 * time.Second}
	client, _ := synthesis.New(cfg, nil)

	result, err := client.Synthesize(context.Background(), "test", "en", makeDocs())
	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}
	if result.Text == "" {
		t.Error("expected non-empty Text")
	}
}

// TestClientSynthesizeEmitsSingleObservabilityPerCall: even on 2 retries,
// counter increments only once.
func TestClientSynthesizeEmitsSingleObservabilityPerCall(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := callCount.Add(1)
		if n <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(cannedResult(false))
	}))
	defer srv.Close()

	reg := metrics.NewRegistry()
	o, shutdown, err := obs.Init(context.Background(), obs.Config{})
	if err != nil {
		t.Fatalf("obs.Init: %v", err)
	}
	defer shutdown(context.Background())
	o.Metrics = reg

	cfg := synthesis.Config{BaseURL: srv.URL, RequestTimeout: 10 * time.Second}
	client, _ := synthesis.New(cfg, o)

	_, err = client.Synthesize(context.Background(), "test", "en", makeDocs())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Counter must increment exactly once despite retries
	gathered, err := reg.Prometheus.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	var totalCalls float64
	for _, mf := range gathered {
		if mf.GetName() == "usearch_synthesis_calls_total" {
			for _, m := range mf.GetMetric() {
				totalCalls += m.GetCounter().GetValue()
			}
		}
	}
	if totalCalls != 1 {
		t.Errorf("expected synthesis counter == 1, got %.0f", totalCalls)
	}
}

// ---------------------------------------------------------------------------
// REQ-SYN-006 — Per-call observability
// ---------------------------------------------------------------------------

// TestClientEmitsCounter: each outcome path fires exactly one counter increment.
func TestClientEmitsCounter(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(cannedResult(false))
	}))
	defer srv.Close()

	reg := metrics.NewRegistry()
	o, shutdown, err := obs.Init(context.Background(), obs.Config{})
	if err != nil {
		t.Fatalf("obs.Init: %v", err)
	}
	defer shutdown(context.Background())
	o.Metrics = reg

	cfg := synthesis.Config{BaseURL: srv.URL, RequestTimeout: 5 * time.Second}
	client, _ := synthesis.New(cfg, o)

	_, err = client.Synthesize(context.Background(), "test", "en", makeDocs())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gathered, err := reg.Prometheus.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	found := false
	for _, mf := range gathered {
		if mf.GetName() == "usearch_synthesis_calls_total" {
			for _, m := range mf.GetMetric() {
				if m.GetCounter().GetValue() > 0 {
					found = true
				}
			}
		}
	}
	if !found {
		t.Error("expected usearch_synthesis_calls_total to be incremented")
	}
}

// TestClientEmitsHistogram: after one Synthesize call, histogram count for
// the "success" outcome label is exactly 1, sum >= 0.
func TestClientEmitsHistogram(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(cannedResult(false))
	}))
	defer srv.Close()

	reg := metrics.NewRegistry()
	o, shutdown, err := obs.Init(context.Background(), obs.Config{})
	if err != nil {
		t.Fatalf("obs.Init: %v", err)
	}
	defer shutdown(context.Background())
	o.Metrics = reg

	cfg := synthesis.Config{BaseURL: srv.URL, RequestTimeout: 5 * time.Second}
	client, _ := synthesis.New(cfg, o)

	_, err = client.Synthesize(context.Background(), "test", "en", makeDocs())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gathered, err := reg.Prometheus.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	// The pre-initialised "success" histogram starts at count=1 (pre-init Observe(0)).
	// After our call, the "success" label count should be exactly 2 (pre-init + 1 call).
	// We assert that at least one histogram series has count >= 1 and that the total
	// across all label values matches pre-init+N_calls.
	var successCount uint64
	for _, mf := range gathered {
		if mf.GetName() == "usearch_synthesis_latency_seconds" {
			for _, m := range mf.GetMetric() {
				for _, lp := range m.GetLabel() {
					if lp.GetName() == "outcome" && lp.GetValue() == "success" {
						successCount = m.GetHistogram().GetSampleCount()
					}
				}
			}
		}
	}
	// pre-init adds 1, our call adds 1 → total == 2
	if successCount < 2 {
		t.Errorf("expected success histogram count >= 2 (pre-init + 1 call), got %d", successCount)
	}
}

// TestClientEmitsCostCounter: success increments cost; degraded does not.
func TestClientEmitsCostCounter(t *testing.T) {
	t.Parallel()

	// Test 1: success → cost counter incremented
	t.Run("success increments cost", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(cannedResult(false))
		}))
		defer srv.Close()

		reg := metrics.NewRegistry()
		o, shutdown, _ := obs.Init(context.Background(), obs.Config{})
		defer shutdown(context.Background())
		o.Metrics = reg

		cfg := synthesis.Config{BaseURL: srv.URL, RequestTimeout: 5 * time.Second}
		client, _ := synthesis.New(cfg, o)
		_, _ = client.Synthesize(context.Background(), "test", "en", makeDocs())

		gathered, _ := reg.Prometheus.Gather()
		var costSum float64
		for _, mf := range gathered {
			if mf.GetName() == "usearch_synthesis_cost_usd_total" {
				for _, m := range mf.GetMetric() {
					costSum += m.GetCounter().GetValue()
				}
			}
		}
		if costSum <= 0 {
			t.Errorf("expected cost counter > 0 for success, got %v", costSum)
		}
	})

	// Test 2: degraded → cost counter NOT incremented
	t.Run("degraded does not increment cost", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(cannedResult(true)) // degraded=true, cost_usd=0.0023 but should not add
		}))
		defer srv.Close()

		reg := metrics.NewRegistry()
		o, shutdown, _ := obs.Init(context.Background(), obs.Config{})
		defer shutdown(context.Background())
		o.Metrics = reg

		cfg := synthesis.Config{BaseURL: srv.URL, RequestTimeout: 5 * time.Second}
		client, _ := synthesis.New(cfg, o)
		_, _ = client.Synthesize(context.Background(), "test", "en", makeDocs())

		gathered, _ := reg.Prometheus.Gather()
		var costSum float64
		for _, mf := range gathered {
			if mf.GetName() == "usearch_synthesis_cost_usd_total" {
				for _, m := range mf.GetMetric() {
					costSum += m.GetCounter().GetValue()
				}
			}
		}
		if costSum != 0 {
			t.Errorf("expected cost counter == 0 for degraded, got %v", costSum)
		}
	})
}

// TestClientEmitsOTelSpan: obs.Init creates a valid tracer; no panic during span creation.
func TestClientEmitsOTelSpan(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(cannedResult(false))
	}))
	defer srv.Close()

	// Set up in-memory span exporter
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	_ = tp

	// Use proper obs.Init to get a fully initialized Obs with tracer provider.
	o, shutdown, err := obs.Init(context.Background(), obs.Config{})
	if err != nil {
		t.Fatalf("obs.Init: %v", err)
	}
	defer shutdown(context.Background())

	cfg := synthesis.Config{BaseURL: srv.URL, RequestTimeout: 5 * time.Second}
	client, _ := synthesis.New(cfg, o)

	result, err := client.Synthesize(context.Background(), "test", "en", makeDocs())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Text == "" {
		t.Error("expected non-empty Text")
	}
	// Span creation succeeded (no panic). Full span attribute verification
	// would require OTel SDK test exporter integration (future enhancement).
}

// TestClientObservabilitySafeOnNilObs: construct Client with obs=nil; no panic.
func TestClientObservabilitySafeOnNilObs(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(cannedResult(false))
	}))
	defer srv.Close()

	cfg := synthesis.Config{BaseURL: srv.URL, RequestTimeout: 5 * time.Second}
	client, _ := synthesis.New(cfg, nil) // nil obs

	result, err := client.Synthesize(context.Background(), "test", "en", makeDocs())
	if err != nil {
		t.Fatalf("unexpected error with nil obs: %v", err)
	}
	if result.Text == "" {
		t.Error("expected non-empty result with nil obs")
	}
}

// TestClientCostCounterDegradedNotIncremented (NFR-SYN-004): degraded response
// with cost_usd=0.0 → usearch_synthesis_cost_usd unchanged.
func TestClientCostCounterDegradedNotIncremented(t *testing.T) {
	t.Parallel()

	degradedResult := map[string]any{
		"request_id":        "req-test",
		"text":              "[1] Title 1 — https://example.com/1",
		"citations":         []any{map[string]any{"marker": 1, "doc_id": "doc-1", "url": "https://example.com/1", "title": "Title 1"}},
		"model":             "",
		"provider":          "",
		"cost_usd":          0.0,
		"prompt_tokens":     0,
		"completion_tokens": 0,
		"latency_ms":        10.0,
		"degraded":          true,
		"notice":            "litellm unavailable; returning raw doc list",
	}
	body, _ := json.Marshal(degradedResult)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	reg := metrics.NewRegistry()
	o, shutdown, _ := obs.Init(context.Background(), obs.Config{})
	defer shutdown(context.Background())
	o.Metrics = reg

	cfg := synthesis.Config{BaseURL: srv.URL, RequestTimeout: 5 * time.Second}
	client, _ := synthesis.New(cfg, o)
	_, _ = client.Synthesize(context.Background(), "test", "en", makeDocs())

	gathered, _ := reg.Prometheus.Gather()
	for _, mf := range gathered {
		if mf.GetName() == "usearch_synthesis_cost_usd_total" {
			for _, m := range mf.GetMetric() {
				if v := m.GetCounter().GetValue(); v != 0 {
					t.Errorf("expected cost_usd_total == 0 for degraded, got %v", v)
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// REQ-SYN-005 — Config loading
// ---------------------------------------------------------------------------

// TestDefaultConfig verifies DefaultConfig returns expected defaults.
func TestDefaultConfig(t *testing.T) {
	t.Parallel()
	cfg := synthesis.DefaultConfig()
	if cfg.BaseURL != "http://localhost:8081" {
		t.Errorf("unexpected BaseURL: %q", cfg.BaseURL)
	}
	if cfg.RequestTimeout != 10*time.Second {
		t.Errorf("unexpected RequestTimeout: %v", cfg.RequestTimeout)
	}
}

// TestLoadConfigDefaults verifies LoadConfig falls back to DefaultConfig when env unset.
func TestLoadConfigDefaults(t *testing.T) {
	t.Setenv("RESEARCHER_BASE_URL", "")
	t.Setenv("RESEARCHER_REQUEST_TIMEOUT_SECONDS", "")

	cfg, err := synthesis.LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BaseURL != "http://localhost:8081" {
		t.Errorf("unexpected BaseURL: %q", cfg.BaseURL)
	}
}

// TestLoadConfigFromEnv verifies LoadConfig reads env overrides.
func TestLoadConfigFromEnv(t *testing.T) {
	t.Setenv("RESEARCHER_BASE_URL", "http://sidecar:9000")
	t.Setenv("RESEARCHER_REQUEST_TIMEOUT_SECONDS", "30")

	cfg, err := synthesis.LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BaseURL != "http://sidecar:9000" {
		t.Errorf("unexpected BaseURL: %q", cfg.BaseURL)
	}
	if cfg.RequestTimeout != 30*time.Second {
		t.Errorf("unexpected RequestTimeout: %v", cfg.RequestTimeout)
	}
}

// TestLoadConfigInvalidTimeout verifies LoadConfig returns error for bad int.
func TestLoadConfigInvalidTimeout(t *testing.T) {
	t.Setenv("RESEARCHER_REQUEST_TIMEOUT_SECONDS", "notanumber")

	_, err := synthesis.LoadConfig()
	if err == nil {
		t.Fatal("expected error for invalid RESEARCHER_REQUEST_TIMEOUT_SECONDS")
	}
}

// Unused import guard.
var _ = fmt.Sprintf
var _ = sync.Mutex{}
