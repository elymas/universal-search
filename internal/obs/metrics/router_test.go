// Package metrics_test validates the Intent Router collectors registered by
// SPEC-IR-001 in router.go.
package metrics_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/elymas/universal-search/internal/obs/metrics"
)

// TestRouterCollectorsAreRegistered asserts the new router collectors are
// non-nil after NewRegistry (REQ-IR-006).
func TestRouterCollectorsAreRegistered(t *testing.T) {
	t.Parallel()
	reg := metrics.NewRegistry()
	if reg.RouterClassifications == nil {
		t.Error("RouterClassifications counter is nil")
	}
	if reg.RouterClassificationDuration == nil {
		t.Error("RouterClassificationDuration histogram is nil")
	}
}

// TestRouterCollectorsExposedInMetricsEndpoint asserts the new families
// appear in /metrics output (REQ-IR-006).
func TestRouterCollectorsExposedInMetricsEndpoint(t *testing.T) {
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
		"usearch_router_classifications_total",
		"usearch_router_classification_duration_seconds",
	} {
		if !strings.Contains(bodyStr, name) {
			t.Errorf("metric %q not found in /metrics output", name)
		}
	}
}

// TestRouterCollectorsCardinality asserts the only label name is `outcome`
// (already in SPEC-OBS-001 allowlist; SPEC-IR-001 adds no new label name).
func TestRouterCollectorsCardinality(t *testing.T) {
	t.Parallel()
	reg := metrics.NewRegistry()
	for _, label := range reg.AllLabelNames() {
		// Sanity: assert the existing allowlist still holds (regression guard).
		switch label {
		case "method", "route", "status_class",
			"adapter_class", "adapter", "outcome",
			"version", "commit", "go_version",
			"provider", "model",
			"mode": // Embedder mode label (SPEC-IDX-002); bounded to 4 values.
			// allowlisted
		default:
			t.Errorf("unexpected label %q in cardinality allowlist", label)
		}
	}
}

// TestRouterCounterIncrementsObservably asserts an increment via the public
// API is reflected in the gathered counter value.
func TestRouterCounterIncrementsObservably(t *testing.T) {
	t.Parallel()
	reg := metrics.NewRegistry()
	reg.RouterClassifications.WithLabelValues("classified_web").Inc()
	reg.RouterClassifications.WithLabelValues("classified_web").Inc()
	got := CounterValue(t, reg.Prometheus, "usearch_router_classifications_total",
		map[string]string{"outcome": "classified_web"})
	if got != 2 {
		t.Errorf("counter: got %v, want 2", got)
	}
}

// TestRouterDurationObservesValue asserts the duration histogram records
// observations.
func TestRouterDurationObservesValue(t *testing.T) {
	t.Parallel()
	reg := metrics.NewRegistry()
	reg.RouterClassificationDuration.WithLabelValues("classified_web").Observe(0.05)
	got := HistogramSum(t, reg.Prometheus, "usearch_router_classification_duration_seconds",
		map[string]string{"outcome": "classified_web"})
	if got <= 0 {
		t.Errorf("histogram sum: got %v, want > 0", got)
	}
}
