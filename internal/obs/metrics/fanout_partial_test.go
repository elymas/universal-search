// Package metrics_test tests the fanout partial, adapter health, and circuit
// state metric families (SPEC-EVAL-002 REQ-EVAL2-003, NFR-EVAL2-001).
package metrics_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/elymas/universal-search/internal/obs/metrics"
)

// TestFanoutPartialMetricFamiliesRegister verifies that the three new metric
// families from SPEC-EVAL-002 REQ-EVAL2-003 are registered and exposed via
// the /metrics endpoint.
func TestFanoutPartialMetricFamiliesRegister(t *testing.T) {
	t.Parallel()

	reg := metrics.NewRegistry()

	// Verify the collectors are non-nil.
	if reg.FanoutPartial == nil {
		t.Error("FanoutPartial counter is nil")
	}
	if reg.AdapterHealthStatus == nil {
		t.Error("AdapterHealthStatus gauge is nil")
	}
	if reg.AdapterCircuitState == nil {
		t.Error("AdapterCircuitState gauge is nil")
	}

	// Verify the metric families appear in /metrics output.
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
		"usearch_fanout_partial_total",
		"usearch_adapter_health_status",
		"usearch_adapter_circuit_state",
	} {
		if !strings.Contains(bodyStr, name) {
			t.Errorf("metric %q not found in /metrics output", name)
		}
	}
}

// TestFanoutPartialCounterIncrements verifies that the fanout partial counter
// increments correctly per adapter label.
func TestFanoutPartialCounterIncrements(t *testing.T) {
	t.Parallel()

	reg := metrics.NewRegistry()

	reg.FanoutPartial.WithLabelValues("reddit").Inc()
	reg.FanoutPartial.WithLabelValues("reddit").Inc()
	reg.FanoutPartial.WithLabelValues("naver").Inc()

	got := CounterValue(t, reg.Prometheus, "usearch_fanout_partial_total",
		map[string]string{"adapter": "reddit"})
	if want := 2.0; got != want {
		t.Errorf("reddit partial count: got %.0f, want %.0f", got, want)
	}

	got = CounterValue(t, reg.Prometheus, "usearch_fanout_partial_total",
		map[string]string{"adapter": "naver"})
	if want := 1.0; got != want {
		t.Errorf("naver partial count: got %.0f, want %.0f", got, want)
	}
}

// TestAdapterHealthStatusGaugeSet verifies that the health status gauge
// correctly records 1.0 (healthy), 0.5 (degraded), 0.0 (unhealthy).
func TestAdapterHealthStatusGaugeSet(t *testing.T) {
	t.Parallel()

	reg := metrics.NewRegistry()

	reg.AdapterHealthStatus.WithLabelValues("reddit").Set(1.0)
	reg.AdapterHealthStatus.WithLabelValues("naver").Set(0.5)
	reg.AdapterHealthStatus.WithLabelValues("youtube").Set(0.0)

	got := GaugeValue(t, reg.Prometheus, "usearch_adapter_health_status",
		map[string]string{"adapter": "reddit"})
	if want := 1.0; got != want {
		t.Errorf("reddit health: got %v, want %v", got, want)
	}

	got = GaugeValue(t, reg.Prometheus, "usearch_adapter_health_status",
		map[string]string{"adapter": "naver"})
	if want := 0.5; got != want {
		t.Errorf("naver health: got %v, want %v", got, want)
	}

	got = GaugeValue(t, reg.Prometheus, "usearch_adapter_health_status",
		map[string]string{"adapter": "youtube"})
	if want := 0.0; got != want {
		t.Errorf("youtube health: got %v, want %v", got, want)
	}
}

// TestAdapterCircuitStateGauge verifies the circuit state gauge with bounded
// state values {closed, open, half_open}.
func TestAdapterCircuitStateGauge(t *testing.T) {
	t.Parallel()

	reg := metrics.NewRegistry()

	reg.AdapterCircuitState.WithLabelValues("reddit", "closed").Set(1)
	reg.AdapterCircuitState.WithLabelValues("reddit", "open").Set(0)
	reg.AdapterCircuitState.WithLabelValues("naver", "half_open").Set(1)

	got := GaugeValue(t, reg.Prometheus, "usearch_adapter_circuit_state",
		map[string]string{"adapter": "reddit", "state": "closed"})
	if want := 1.0; got != want {
		t.Errorf("reddit closed: got %v, want %v", got, want)
	}

	got = GaugeValue(t, reg.Prometheus, "usearch_adapter_circuit_state",
		map[string]string{"adapter": "reddit", "state": "open"})
	if want := 0.0; got != want {
		t.Errorf("reddit open: got %v, want %v", got, want)
	}

	got = GaugeValue(t, reg.Prometheus, "usearch_adapter_circuit_state",
		map[string]string{"adapter": "naver", "state": "half_open"})
	if want := 1.0; got != want {
		t.Errorf("naver half_open: got %v, want %v", got, want)
	}
}

// NOTE: TestStateEnumBoundedThreeValues (NFR-EVAL2-001) lives in
// adapter_reliability_test.go, which asserts the stronger invariant that the
// circuit_state `state` label exposes EXACTLY three distinct values. The
// duplicate definition that previously lived here was removed during the
// v1.0.0 integration merge; the bounded values are still exercised end-to-end
// via TestFanoutPartialMetricFamiliesRegister and TestAdapterCircuitStateGauge
// above.

// TestCardinalityBudget12AdaptersUnder200Series verifies the cardinality budget
// for the three new metric families with 12 adapters (NFR-EVAL2-001).
//
// Expected series count:
//   - usearch_fanout_partial_total: 12 adapters × 1 = 12 series
//   - usearch_adapter_health_status: 12 adapters × 1 = 12 series
//   - usearch_adapter_circuit_state: 12 adapters × 3 states = 36 series
//   - Total: 60 series (well under 200-series budget for new families)
func TestCardinalityBudget12AdaptersUnder200Series(t *testing.T) {
	t.Parallel()

	reg := metrics.NewRegistry()

	// Simulate 12 adapters (the production adapter count per SPEC-ADP-001..009).
	adapters := []string{
		"reddit", "hackernews", "arxiv", "github",
		"youtube", "bluesky", "x", "searxng",
		"naver", "korea_news", "daum", "rss",
	}

	for _, a := range adapters {
		reg.FanoutPartial.WithLabelValues(a).Add(0)
		reg.AdapterHealthStatus.WithLabelValues(a).Set(1.0)
		for _, state := range []string{"closed", "open", "half_open"} {
			reg.AdapterCircuitState.WithLabelValues(a, state).Set(0)
		}
	}

	// Gather all metric families and count series for our three families.
	mfs, err := reg.Prometheus.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	totalSeries := 0
	targetFamilies := map[string]bool{
		"usearch_fanout_partial_total":    true,
		"usearch_adapter_health_status":   true,
		"usearch_adapter_circuit_state":   true,
	}

	for _, mf := range mfs {
		if !targetFamilies[mf.GetName()] {
			continue
		}
		totalSeries += len(mf.GetMetric())
	}

	if totalSeries > 200 {
		t.Errorf("total series for new EVAL-002 families: got %d, want <= 200 (NFR-EVAL2-001)", totalSeries)
	}

	// Also verify exact expected count: 1 (placeholder) + 12 + 1 (placeholder) + 12
	// + 3 (placeholder) + 36 = 65 (pre-initialised placeholder series included).
	if totalSeries != 65 {
		t.Errorf("expected exactly 65 series (12+12+36 + 5 pre-init placeholders), got %d", totalSeries)
	}
}
