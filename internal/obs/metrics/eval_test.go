// Package metrics_test validates the SPEC-EVAL-001 eval benchmark collectors.
package metrics_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/elymas/universal-search/internal/obs/metrics"
)

// TestEvalCollectorsAreRegistered asserts the eval collectors are non-nil.
// SPEC-EVAL-001 observability.
func TestEvalCollectorsAreRegistered(t *testing.T) {
	t.Parallel()
	reg := metrics.NewRegistry()
	if reg.EvalRuns == nil {
		t.Error("EvalRuns counter is nil")
	}
	if reg.EvalScore == nil {
		t.Error("EvalScore gauge is nil")
	}
}

// TestEvalCollectorsExposedInMetricsEndpoint asserts the families appear in
// /metrics output even before any observation (REQ-OBS-004).
func TestEvalCollectorsExposedInMetricsEndpoint(t *testing.T) {
	t.Parallel()
	reg := metrics.NewRegistry()
	srv := httptest.NewServer(metrics.Handler(reg))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	bodyStr := string(body)
	for _, name := range []string{"usearch_eval_runs_total", "usearch_eval_score_gauge"} {
		if !strings.Contains(bodyStr, name) {
			t.Errorf("metric %q not found in /metrics output", name)
		}
	}
}

// TestEvalCollectorsAddNoNewLabel asserts the eval family reuses the existing
// `outcome` label and adds no new label name to the cardinality allowlist
// (NFR-OBS-002).
func TestEvalCollectorsAddNoNewLabel(t *testing.T) {
	t.Parallel()
	reg := metrics.NewRegistry()
	reg.EvalRuns.WithLabelValues("pass").Inc()
	reg.EvalScore.Set(0.86)

	// `outcome` is already allowlisted; assert no surprise label appeared.
	allowed := map[string]bool{
		"method": true, "route": true, "status_class": true,
		"adapter_class": true, "adapter": true, "outcome": true,
		"version": true, "commit": true, "go_version": true,
		"provider": true, "model": true, "mode": true,
		"store": true, "op": true, "shard": true,
		"agent": true, "result": true, "reason": true,
		"trigger": true, "reason_class": true,
		// SPEC-EVAL-002 (state) + SPEC-SEC-001 (component/type/severity/tenant_id_class);
		// all bounded enums, pre-existing in the allowlist before eval registration.
		"state": true, "component": true, "type": true,
		"severity": true, "tenant_id_class": true,
	}
	for _, label := range reg.AllLabelNames() {
		if !allowed[label] {
			t.Errorf("unexpected label %q in allowlist after eval registration", label)
		}
	}
}
