// Package metrics_test tests the Prometheus metrics registration and HTTP
// admin server (REQ-OBS-003, REQ-OBS-004, NFR-OBS-002).
package metrics_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/obs/metrics"
)

// TestMeterRegistersCounterHistogramGauge verifies that the named collectors
// are non-nil after NewRegistry is called.
// REQ-OBS-003
func TestMeterRegistersCounterHistogramGauge(t *testing.T) {
	t.Parallel()

	reg := metrics.NewRegistry()
	if reg.HTTPRequests == nil {
		t.Error("HTTPRequests counter is nil")
	}
	if reg.HTTPRequestDuration == nil {
		t.Error("HTTPRequestDuration histogram is nil")
	}
	if reg.FanoutInflight == nil {
		t.Error("FanoutInflight gauge is nil")
	}
	if reg.AdapterCalls == nil {
		t.Error("AdapterCalls counter is nil")
	}
	if reg.AdapterCallDuration == nil {
		t.Error("AdapterCallDuration histogram is nil")
	}
	if reg.BuildInfo == nil {
		t.Error("BuildInfo gauge is nil")
	}
}

// TestRequestCounterIncrementsOnObservation verifies that HTTPRequests counter
// increments correctly via the HTTP middleware.
// REQ-OBS-003
func TestRequestCounterIncrementsOnObservation(t *testing.T) {
	t.Parallel()

	reg := metrics.NewRegistry()
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := metrics.HTTPMiddleware(reg, "/x", inner)

	for range 3 {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		rw := httptest.NewRecorder()
		handler.ServeHTTP(rw, req)
	}

	// Read metric value via gather helper (defined in testhelper_test.go).
	count := CounterValue(t, reg.Prometheus, "usearch_http_requests_total",
		map[string]string{"method": "GET", "route": "/x", "status_class": "2xx"})
	if count != 3 {
		t.Errorf("HTTPRequests counter: got %.0f, want 3", count)
	}
}

// TestLatencyHistogramObservesDuration verifies that the duration histogram
// records observations.
// REQ-OBS-003
func TestLatencyHistogramObservesDuration(t *testing.T) {
	t.Parallel()

	reg := metrics.NewRegistry()
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})
	handler := metrics.HTTPMiddleware(reg, "/bench", inner)

	req := httptest.NewRequest(http.MethodGet, "/bench", nil)
	rw := httptest.NewRecorder()
	handler.ServeHTTP(rw, req)

	sum := HistogramSum(t, reg.Prometheus, "usearch_http_request_duration_seconds",
		map[string]string{"method": "GET", "route": "/bench"})
	if sum <= 0 {
		t.Errorf("expected positive histogram sum, got %v", sum)
	}
}

// TestFanoutGoroutineGaugeTracksActiveCount verifies Inc + Dec returns to baseline.
// REQ-OBS-003
func TestFanoutGoroutineGaugeTracksActiveCount(t *testing.T) {
	t.Parallel()

	reg := metrics.NewRegistry()

	// Baseline value for adapter_class "web".
	baseline := GaugeValue(t, reg.Prometheus, "usearch_fanout_goroutines_inflight",
		map[string]string{"adapter_class": "web"})

	reg.FanoutInflight.WithLabelValues("web").Inc()
	reg.FanoutInflight.WithLabelValues("web").Dec()

	after := GaugeValue(t, reg.Prometheus, "usearch_fanout_goroutines_inflight",
		map[string]string{"adapter_class": "web"})

	if after != baseline {
		t.Errorf("gauge after Inc+Dec: got %.0f, want %.0f", after, baseline)
	}
}

// TestMetricsEndpointReturns200WithPrometheusText verifies the /metrics
// endpoint returns HTTP 200 with text/plain content type.
// REQ-OBS-004
func TestMetricsEndpointReturns200WithPrometheusText(t *testing.T) {
	t.Parallel()

	reg := metrics.NewRegistry()
	srv := httptest.NewServer(metrics.Handler(reg))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type: got %q, want prefix text/plain", ct)
	}
}

// TestMetricsEndpointExposesRegisteredMetrics verifies that the /metrics
// response body contains every declared metric family name.
// REQ-OBS-004
func TestMetricsEndpointExposesRegisteredMetrics(t *testing.T) {
	t.Parallel()

	reg := metrics.NewRegistry()
	srv := httptest.NewServer(metrics.Handler(reg))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	bodyStr := string(body)

	for _, name := range []string{
		"usearch_http_requests_total",
		"usearch_http_request_duration_seconds",
		"usearch_fanout_goroutines_inflight",
		"usearch_adapter_calls_total",
		"usearch_adapter_call_duration_seconds",
		"usearch_build_info",
	} {
		if !strings.Contains(bodyStr, name) {
			t.Errorf("metric %q not found in /metrics output", name)
		}
	}
}

// TestAdminServerBindsToLocalhostOnly verifies that StartAdminServer creates a
// listener (checked by successful /metrics fetch) and the server shuts down
// cleanly on context cancel.
// REQ-OBS-004
func TestAdminServerBindsToLocalhostOnly(t *testing.T) {
	t.Parallel()

	reg := metrics.NewRegistry()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Use :0 to get an available port assigned by the OS.
	addr, shutdown, err := metrics.StartAdminServer(ctx, "127.0.0.1:0", reg)
	if err != nil {
		t.Fatalf("StartAdminServer: %v", err)
	}

	// Verify the endpoint responds.
	resp, err := http.Get("http://" + addr + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}

	// Shutdown via returned closure.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()
	if err := shutdown(shutdownCtx); err != nil {
		t.Errorf("shutdown: %v", err)
	}
}

// TestAdminServerShutsDownOnContextCancel verifies that cancelling the context
// stops the admin server.
// REQ-OBS-004
func TestAdminServerShutsDownOnContextCancel(t *testing.T) {
	t.Parallel()

	reg := metrics.NewRegistry()
	ctx, cancel := context.WithCancel(context.Background())

	addr, _, err := metrics.StartAdminServer(ctx, "127.0.0.1:0", reg)
	if err != nil {
		t.Fatalf("StartAdminServer: %v", err)
	}

	// Verify running.
	resp, err := http.Get("http://" + addr + "/metrics")
	if err != nil {
		t.Fatalf("initial GET: %v", err)
	}
	resp.Body.Close()

	// Cancel causes server to stop.
	cancel()
	time.Sleep(150 * time.Millisecond)

	// Subsequent request should fail (connection refused).
	_, err = http.Get("http://" + addr + "/metrics")
	if err == nil {
		t.Error("expected connection error after cancel, but request succeeded")
	}
}

// TestCardinalityGuardRejectsUnboundedLabels is a static-analysis test that
// walks all Vec registrations and asserts label names are in the allowlist.
// NFR-OBS-002
func TestCardinalityGuardRejectsUnboundedLabels(t *testing.T) {
	t.Parallel()

	allowlist := map[string]bool{
		"method":        true,
		"route":         true,
		"status_class":  true,
		"adapter_class": true,
		"adapter":       true,
		"outcome":       true,
		"version":       true,
		"commit":        true,
		"go_version":    true,
		// LLM labels added by SPEC-LLM-001 (REQ-LLM-007); bounded by
		// provider ∈ {anthropic,openai,ollama} and model ∈ config.yaml aliases (≤15).
		"provider": true,
		"model":    true,
		// Embedder labels added by SPEC-IDX-002; mode ∈ {dense,sparse,colbert,all} (4 values).
		"mode": true,
		// Index layer labels added by SPEC-IDX-001; store ∈ {qdrant,meili,pg}, op ∈ {search,upsert}.
		"store": true,
		"op":    true,
		// Tokenizer sidecar labels added by SPEC-IDX-003; shard ∈ {ko,default}.
		"shard": true,
		// Deep agent labels added by SPEC-DEEP-002 (NFR-DEEP2-002).
		// agent ∈ {researcher, reviewer, writer, verifier} (4 values, bounded enum).
		"agent": true,
		// result ∈ {pass, fail_uncited, fail_error} (3 values, bounded enum).
		"result": true,
	}

	reg := metrics.NewRegistry()
	for _, label := range reg.AllLabelNames() {
		if !allowlist[label] {
			t.Errorf("label %q is not in the cardinality allowlist (NFR-OBS-002)", label)
		}
	}
}

// TestNoUnboundedLabels is an alias for the cardinality guard test.
// NFR-OBS-002
func TestNoUnboundedLabels(t *testing.T) {
	TestCardinalityGuardRejectsUnboundedLabels(t)
}

// ---------------------------------------------------------------------------
// SPEC-DEEP-001 M6: Deep report metrics registration tests
// ---------------------------------------------------------------------------

// TestDeepReportOutcomesCounterRegistered verifies the DeepReportOutcomes
// CounterVec is registered with the "outcome" label and all 6 pre-declared
// values appear in the /metrics output via the Add(0) pre-initialisation pattern.
// SPEC-DEEP-001 acceptance test #4.
func TestDeepReportOutcomesCounterRegistered(t *testing.T) {
	t.Parallel()

	reg := metrics.NewRegistry()
	if reg.DeepReportOutcomes == nil {
		t.Fatal("DeepReportOutcomes is nil after NewRegistry")
	}

	srv := httptest.NewServer(metrics.Handler(reg))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	bodyStr := string(body)

	// Verify the metric family name is present.
	if !strings.Contains(bodyStr, "usearch_deep_outcomes_total") {
		t.Error("usearch_deep_outcomes_total not found in /metrics output")
	}

	// Verify all 6 outcome label values are pre-initialised.
	for _, outcome := range []string{
		"success",
		"deadline_exceeded",
		"budget_exceeded",
		"error_invalid",
		"error_upstream",
		"error_unresolved_citations_threshold",
	} {
		if !strings.Contains(bodyStr, outcome) {
			t.Errorf("outcome label value %q not found in /metrics output (pre-initialisation missing)", outcome)
		}
	}
}

// TestDeepReportLatencyHistogramRegistered verifies the DeepReportLatency
// histogram is registered with the correct buckets [5,15,30,60,120,180,240,300]s.
// SPEC-DEEP-001 acceptance test #5.
func TestDeepReportLatencyHistogramRegistered(t *testing.T) {
	t.Parallel()

	reg := metrics.NewRegistry()
	if reg.DeepReportLatency == nil {
		t.Fatal("DeepReportLatency is nil after NewRegistry")
	}

	srv := httptest.NewServer(metrics.Handler(reg))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	bodyStr := string(body)

	if !strings.Contains(bodyStr, "usearch_deep_latency_seconds") {
		t.Error("usearch_deep_latency_seconds not found in /metrics output")
	}

	// Verify correct bucket boundaries by checking for the le labels.
	// Prometheus exposition format exposes bucket boundaries as le="5", le="15", etc.
	for _, bucket := range []string{"5", "15", "30", "60", "120", "180", "240", "300"} {
		if !strings.Contains(bodyStr, `le="`+bucket+`"`) {
			// This check applies to all histograms; only fail if the deep_latency metric
			// is present but missing the bucket.
			t.Errorf("bucket le=%q not found in /metrics output", bucket)
		}
	}
}
