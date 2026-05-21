// Package metrics_test — SPEC-DEEP-002 M6: Deep agent metric collector tests.
// REQ-DEEP2-008, NFR-DEEP2-002
package metrics_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/elymas/universal-search/internal/obs/metrics"
)

// ---------------------------------------------------------------------------
// T-M6-001 [RED]: 3 new collector registration + label pre-declaration tests
// ---------------------------------------------------------------------------

// TestThreeNewCollectorsRegisteredAtStartup verifies that the three new
// DEEP-002 collectors (DeepAgentDuration, DeepAgentRetries,
// DeepAgentVerifierGateResults) are registered and non-nil after NewRegistry.
// REQ-DEEP2-008, test #57.
func TestThreeNewCollectorsRegisteredAtStartup(t *testing.T) {
	t.Parallel()

	reg := metrics.NewRegistry()

	if reg.DeepAgentDuration == nil {
		t.Error("DeepAgentDuration histogram is nil after NewRegistry")
	}
	if reg.DeepAgentRetries == nil {
		t.Error("DeepAgentRetries counter is nil after NewRegistry")
	}
	if reg.DeepAgentVerifierGateResults == nil {
		t.Error("DeepAgentVerifierGateResults counter is nil after NewRegistry")
	}
}

// TestAllAgentLabelValuesPreDeclaredAtRegistration verifies that all label
// values are pre-declared via the WithLabelValues(v).Add(0) pattern so they
// appear in /metrics output even before any real observations.
// REQ-DEEP2-008, test #58.
// NFR-DEEP2-002: bounded enum pre-declaration.
func TestAllAgentLabelValuesPreDeclaredAtRegistration(t *testing.T) {
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

	// Verify DeepAgentDuration histogram appears with agent and outcome labels.
	if !strings.Contains(bodyStr, "usearch_deep_agent_duration_seconds") {
		t.Error("usearch_deep_agent_duration_seconds not found in /metrics output")
	}

	// Verify all agent label values are pre-declared.
	for _, agent := range []string{"researcher", "reviewer", "writer", "verifier"} {
		if !strings.Contains(bodyStr, `agent="`+agent+`"`) {
			t.Errorf("agent label value %q not pre-declared in /metrics output", agent)
		}
	}

	// Verify outcome label values for duration histogram.
	if !strings.Contains(bodyStr, `outcome="success"`) {
		t.Error(`outcome="success" not pre-declared in /metrics output`)
	}
	if !strings.Contains(bodyStr, `outcome="error"`) {
		t.Error(`outcome="error" not pre-declared in /metrics output`)
	}

	// Verify DeepAgentRetries counter with agent="writer".
	if !strings.Contains(bodyStr, "usearch_deep_agent_retries_total") {
		t.Error("usearch_deep_agent_retries_total not found in /metrics output")
	}

	// Verify DeepAgentVerifierGateResults counter with result label values.
	if !strings.Contains(bodyStr, "usearch_deep_agent_verifier_gate_results_total") {
		t.Error("usearch_deep_agent_verifier_gate_results_total not found in /metrics output")
	}
	for _, result := range []string{"pass", "fail_uncited", "fail_error"} {
		if !strings.Contains(bodyStr, `result="`+result+`"`) {
			t.Errorf("result label value %q not pre-declared in /metrics output", result)
		}
	}
}

// TestDeepOutcomesExtendedWithEmptyCorpusAndPipelineFailed verifies that the
// existing usearch_deep_outcomes_total collector is extended with two new
// outcome values: "empty_corpus" and "error_pipeline_failed".
// REQ-DEEP2-008, test #60.
func TestDeepOutcomesExtendedWithEmptyCorpusAndPipelineFailed(t *testing.T) {
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

	// Verify the two new outcome label values are pre-declared.
	for _, outcome := range []string{"empty_corpus", "error_pipeline_failed"} {
		if !strings.Contains(bodyStr, `outcome="`+outcome+`"`) {
			t.Errorf("outcome %q not found in usearch_deep_outcomes_total /metrics output", outcome)
		}
	}
}

// TestDeepAgentDurationBuckets verifies the histogram buckets match
// REQ-DEEP2-008: [0.5, 1, 2, 5, 10, 30, 60, 120].
func TestDeepAgentDurationBuckets(t *testing.T) {
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

	for _, bucket := range []string{"0.5", "1", "2", "5", "10", "30", "60", "120"} {
		if !strings.Contains(bodyStr, `le="`+bucket+`"`) {
			t.Errorf("bucket le=%q not found in /metrics output for usearch_deep_agent_duration_seconds", bucket)
		}
	}
}

// ---------------------------------------------------------------------------
// T-M6-003 [RED]: Cardinality guard TestNoUnboundedLabels regression test
// ---------------------------------------------------------------------------

// TestCardinalityGuardRemainsGreen verifies that adding new DEEP-002 label
// names ("agent", "result") does not break the existing cardinality guard.
// NFR-DEEP2-002, test #61.
func TestCardinalityGuardRemainsGreen(t *testing.T) {
	// This is a wrapper around TestNoUnboundedLabels to ensure it stays green
	// after adding the new DEEP-002 label names.
	TestNoUnboundedLabels(t)
}
