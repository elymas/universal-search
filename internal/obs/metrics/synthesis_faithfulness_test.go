// Package metrics_test — SPEC-SYN-002 faithfulness collector tests.
//
// These tests verify that the two new Prometheus collectors declared by
// SPEC-SYN-002 §2.1(f,g) are registered correctly on the shared Registry.
package metrics_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/elymas/universal-search/internal/obs/metrics"
)

// TestRegistryExposesFaithfulnessOutcomesMetric verifies that
// usearch_synthesis_faithfulness_outcomes_total appears in /metrics output
// after NewRegistry is called. REQ-OBS-004 + SPEC-SYN-002 §2.1(f).
func TestRegistryExposesFaithfulnessOutcomesMetric(t *testing.T) {
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

	const wantName = "usearch_synthesis_faithfulness_outcomes_total"
	if !strings.Contains(string(body), wantName) {
		t.Errorf("metric %q not found in /metrics output (SPEC-SYN-002 §2.1(f))", wantName)
	}
}

// TestRegistryExposesFaithfulnessRetriesMetric verifies that
// usearch_synthesis_faithfulness_retries_total appears in /metrics output.
// SPEC-SYN-002 §2.1(f).
func TestRegistryExposesFaithfulnessRetriesMetric(t *testing.T) {
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

	const wantName = "usearch_synthesis_faithfulness_retries_total"
	if !strings.Contains(string(body), wantName) {
		t.Errorf("metric %q not found in /metrics output (SPEC-SYN-002 §2.1(f))", wantName)
	}
}

// TestRegistrySynthesisFaithfulnessOutcomesFieldNonNil verifies that
// SynthesisFaithfulnessOutcomes is non-nil on a freshly created Registry.
// SPEC-SYN-002 §2.1(g).
func TestRegistrySynthesisFaithfulnessOutcomesFieldNonNil(t *testing.T) {
	t.Parallel()

	reg := metrics.NewRegistry()
	if reg.SynthesisFaithfulnessOutcomes == nil {
		t.Fatal("Registry.SynthesisFaithfulnessOutcomes is nil after NewRegistry() (SPEC-SYN-002 §2.1(g))")
	}
}

// TestRegistrySynthesisFaithfulnessRetriesFieldNonNil verifies that
// SynthesisFaithfulnessRetries is non-nil on a freshly created Registry.
// SPEC-SYN-002 §2.1(g).
func TestRegistrySynthesisFaithfulnessRetriesFieldNonNil(t *testing.T) {
	t.Parallel()

	reg := metrics.NewRegistry()
	if reg.SynthesisFaithfulnessRetries == nil {
		t.Fatal("Registry.SynthesisFaithfulnessRetries is nil after NewRegistry() (SPEC-SYN-002 §2.1(g))")
	}
}
