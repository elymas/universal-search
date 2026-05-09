// Package citation_test contains specification tests for SPEC-SYN-002.
//
// These tests define the citation faithfulness verification contract
// that the Go-side metrics layer must satisfy.
// Tests are written RED-first before implementation exists.
package citation_test

import (
	"strings"
	"testing"

	"github.com/elymas/universal-search/internal/synthesis/citation"
)

// --- FaithfulnessOutcome enum tests ---

func TestOutcomeStringValues(t *testing.T) {
	t.Parallel()
	// Verify each outcome produces a stable, non-empty string label
	// (used as prometheus label values — must not change after release).
	cases := []struct {
		outcome citation.FaithfulnessOutcome
		want    string
	}{
		{citation.OutcomeAccepted, "accepted"},
		{citation.OutcomeStripped, "stripped"},
		{citation.OutcomeRejected, "rejected"},
		{citation.OutcomeRetrySucceeded, "retry_succeeded"},
		{citation.OutcomeRetryFailed, "retry_failed"},
		{citation.OutcomeOff, "off"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			got := tc.outcome.String()
			if got != tc.want {
				t.Errorf("outcome.String() = %q, want %q", got, tc.want)
			}
		})
	}
}

// --- MetricLabels tests (outcome label values for Prometheus) ---

func TestAllOutcomeLabelsAreValidPrometheusLabels(t *testing.T) {
	t.Parallel()
	// Prometheus label values must be non-empty and contain no whitespace.
	for _, o := range citation.AllOutcomes() {
		s := o.String()
		if s == "" {
			t.Errorf("outcome %v has empty label string", o)
		}
		if strings.ContainsAny(s, " \t\n") {
			t.Errorf("outcome label %q contains whitespace", s)
		}
	}
}

// --- FaithfulnessMetrics constructor tests ---

func TestNewFaithfulnessMetrics_ReturnsNonNil(t *testing.T) {
	t.Parallel()
	m := citation.NewFaithfulnessMetrics()
	if m == nil {
		t.Fatal("NewFaithfulnessMetrics() returned nil")
	}
}

func TestNewFaithfulnessMetrics_OutcomesCounterNonNil(t *testing.T) {
	t.Parallel()
	m := citation.NewFaithfulnessMetrics()
	if m.Outcomes == nil {
		t.Fatal("FaithfulnessMetrics.Outcomes is nil")
	}
}

func TestNewFaithfulnessMetrics_RetriesCounterNonNil(t *testing.T) {
	t.Parallel()
	m := citation.NewFaithfulnessMetrics()
	if m.Retries == nil {
		t.Fatal("FaithfulnessMetrics.Retries is nil")
	}
}

// --- Increment tests (verifies counter API works without panic) ---

func TestRecordOutcome_AcceptedDoesNotPanic(t *testing.T) {
	t.Parallel()
	m := citation.NewFaithfulnessMetrics()
	// Should not panic — accepted outcome increments the Outcomes counter.
	m.RecordOutcome(citation.OutcomeAccepted)
}

func TestRecordOutcome_StrippedDoesNotPanic(t *testing.T) {
	t.Parallel()
	m := citation.NewFaithfulnessMetrics()
	m.RecordOutcome(citation.OutcomeStripped)
}

func TestRecordOutcome_RejectedDoesNotPanic(t *testing.T) {
	t.Parallel()
	m := citation.NewFaithfulnessMetrics()
	m.RecordOutcome(citation.OutcomeRejected)
}

func TestRecordOutcome_RetrySucceededDoesNotPanic(t *testing.T) {
	t.Parallel()
	m := citation.NewFaithfulnessMetrics()
	m.RecordOutcome(citation.OutcomeRetrySucceeded)
}

func TestRecordOutcome_RetryFailedDoesNotPanic(t *testing.T) {
	t.Parallel()
	m := citation.NewFaithfulnessMetrics()
	m.RecordOutcome(citation.OutcomeRetryFailed)
}

func TestRecordRetry_DoesNotPanic(t *testing.T) {
	t.Parallel()
	m := citation.NewFaithfulnessMetrics()
	// Should not panic — retries counter increments.
	m.RecordRetry()
}

// --- Mode enum tests ---

func TestModeStringValues(t *testing.T) {
	t.Parallel()
	cases := []struct {
		mode citation.EnforcementMode
		want string
	}{
		{citation.ModeStrip, "strip"},
		{citation.ModeReject, "reject"},
		{citation.ModeOff, "off"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			got := tc.mode.String()
			if got != tc.want {
				t.Errorf("mode.String() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseModeFromEnv_KnownValues(t *testing.T) {
	t.Parallel()
	cases := []struct {
		input string
		want  citation.EnforcementMode
		err   bool
	}{
		{"strip", citation.ModeStrip, false},
		{"reject", citation.ModeReject, false},
		{"off", citation.ModeOff, false},
		{"STRIP", citation.ModeStrip, false},   // case-insensitive
		{"REJECT", citation.ModeReject, false}, // case-insensitive
		{"OFF", citation.ModeOff, false},       // case-insensitive
		{"", citation.ModeStrip, false},        // empty => default strip
		{"unknown", citation.ModeStrip, true},  // invalid => error + default
	}
	for _, tc := range cases {
		tc := tc
		t.Run("input_"+tc.input, func(t *testing.T) {
			t.Parallel()
			got, err := citation.ParseModeFromEnv(tc.input)
			if tc.err && err == nil {
				t.Errorf("ParseModeFromEnv(%q): expected error, got nil", tc.input)
			}
			if !tc.err && err != nil {
				t.Errorf("ParseModeFromEnv(%q): unexpected error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("ParseModeFromEnv(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// --- RegisterFaithfulnessMetrics tests (for metrics.Registry integration) ---

func TestRegisterFaithfulnessMetrics_ReturnsNonNilMetrics(t *testing.T) {
	t.Parallel()
	reg := citation.NewPrometheusRegistry()
	m, err := citation.RegisterFaithfulnessMetrics(reg)
	if err != nil {
		t.Fatalf("RegisterFaithfulnessMetrics() error: %v", err)
	}
	if m == nil {
		t.Fatal("RegisterFaithfulnessMetrics() returned nil FaithfulnessMetrics")
	}
}

func TestRegisterFaithfulnessMetrics_DoubleRegisterReturnsError(t *testing.T) {
	t.Parallel()
	reg := citation.NewPrometheusRegistry()
	_, err := citation.RegisterFaithfulnessMetrics(reg)
	if err != nil {
		t.Fatalf("first registration failed: %v", err)
	}
	_, err = citation.RegisterFaithfulnessMetrics(reg)
	if err == nil {
		t.Fatal("second RegisterFaithfulnessMetrics() should return error on duplicate registration")
	}
}

// --- MetricName contract tests (names must match SPEC-SYN-002 §2.1(f)) ---

func TestOutcomesMetricName(t *testing.T) {
	t.Parallel()
	const wantName = "usearch_synthesis_faithfulness_outcomes_total"
	got := citation.OutcomesMetricName()
	if got != wantName {
		t.Errorf("OutcomesMetricName() = %q, want %q", got, wantName)
	}
}

func TestRetriesMetricName(t *testing.T) {
	t.Parallel()
	const wantName = "usearch_synthesis_faithfulness_retries_total"
	got := citation.RetriesMetricName()
	if got != wantName {
		t.Errorf("RetriesMetricName() = %q, want %q", got, wantName)
	}
}
