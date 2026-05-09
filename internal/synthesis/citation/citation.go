// Package citation implements the Go-side citation faithfulness verification layer
// for SPEC-SYN-002.
//
// This package provides:
//   - FaithfulnessOutcome enum (used as Prometheus label values)
//   - EnforcementMode enum (maps RESEARCHER_FAITHFULNESS_MODE env var)
//   - FaithfulnessMetrics struct with two collectors:
//     usearch_synthesis_faithfulness_outcomes_total{outcome}
//     usearch_synthesis_faithfulness_retries_total
//
// The Python sidecar performs the actual sentence-level enforcement;
// this package owns only the Go-side metrics and mode parsing contract.
// See SPEC-SYN-002 §2.1(f), §2.1(g), §2.1(h).
package citation

import (
	"errors"
	"fmt"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

// --- FaithfulnessOutcome ---

// FaithfulnessOutcome enumerates the possible outcomes of the faithfulness gate.
// Each value maps directly to an `outcome` Prometheus label per SPEC-SYN-002 §2.1(f).
//
// @MX:ANCHOR: [AUTO] Outcome label contract; callers: RecordOutcome, RegisterFaithfulnessMetrics, obs re-exports
// @MX:REASON: fan_in >= 3; label values are frozen once published to Prometheus — changing them breaks dashboards
// @MX:SPEC: SPEC-SYN-002 §2.1(f) REQ-SYN2-004
type FaithfulnessOutcome int

const (
	// OutcomeAccepted means the LLM output passed the faithfulness gate without modification.
	OutcomeAccepted FaithfulnessOutcome = iota
	// OutcomeStripped means one or more un-cited sentences were removed (mode=strip).
	OutcomeStripped
	// OutcomeRejected means the request was rejected with HTTP 422 (mode=reject).
	OutcomeRejected
	// OutcomeRetrySucceeded means the single retry attempt produced a fully-cited output.
	OutcomeRetrySucceeded
	// OutcomeRetryFailed means the single retry also failed; fallback mode applied.
	OutcomeRetryFailed
	// OutcomeOff means faithfulness gate was bypassed (mode=off).
	OutcomeOff
)

// String returns the Prometheus label value for this outcome.
// Label strings are frozen after first Prometheus scrape — do not rename.
// @MX:NOTE: [AUTO] Each case maps to a label value used in dashboards and alerts.
func (o FaithfulnessOutcome) String() string {
	switch o {
	case OutcomeAccepted:
		return "accepted"
	case OutcomeStripped:
		return "stripped"
	case OutcomeRejected:
		return "rejected"
	case OutcomeRetrySucceeded:
		return "retry_succeeded"
	case OutcomeRetryFailed:
		return "retry_failed"
	case OutcomeOff:
		return "off"
	default:
		return fmt.Sprintf("unknown(%d)", int(o))
	}
}

// AllOutcomes returns all defined FaithfulnessOutcome values.
// Used by tests and pre-initialisation of Prometheus label sets.
func AllOutcomes() []FaithfulnessOutcome {
	return []FaithfulnessOutcome{
		OutcomeAccepted,
		OutcomeStripped,
		OutcomeRejected,
		OutcomeRetrySucceeded,
		OutcomeRetryFailed,
		OutcomeOff,
	}
}

// --- EnforcementMode ---

// EnforcementMode represents the value of the RESEARCHER_FAITHFULNESS_MODE env var.
// Controls what happens when un-cited sentences are detected.
// See SPEC-SYN-002 §2.1(d), REQ-SYN2-003.
type EnforcementMode int

const (
	// ModeStrip removes un-cited sentences (default behavior).
	ModeStrip EnforcementMode = iota
	// ModeReject returns HTTP 422 when un-cited sentences remain after retry.
	ModeReject
	// ModeOff bypasses the faithfulness gate entirely (emergency rollback).
	ModeOff
)

// String returns the canonical env-var string for this mode.
func (m EnforcementMode) String() string {
	switch m {
	case ModeStrip:
		return "strip"
	case ModeReject:
		return "reject"
	case ModeOff:
		return "off"
	default:
		return fmt.Sprintf("unknown(%d)", int(m))
	}
}

// ErrUnknownMode is returned by ParseModeFromEnv when the input does not match
// any known mode value. The returned mode defaults to ModeStrip.
var ErrUnknownMode = errors.New("citation: unknown faithfulness mode")

// ParseModeFromEnv parses the RESEARCHER_FAITHFULNESS_MODE env-var value.
// Empty string returns ModeStrip (the default). Matching is case-insensitive.
// On unknown value returns (ModeStrip, ErrUnknownMode).
//
// @MX:ANCHOR: [AUTO] Mode parsing entry point; callers: obs re-exports, synthesize(), tests
// @MX:REASON: fan_in >= 3; single parsing point for env-var — must never silently ignore invalid values
// @MX:SPEC: SPEC-SYN-002 §2.1(d) REQ-SYN2-003
func ParseModeFromEnv(raw string) (EnforcementMode, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "strip":
		return ModeStrip, nil
	case "reject":
		return ModeReject, nil
	case "off":
		return ModeOff, nil
	default:
		return ModeStrip, fmt.Errorf("%w: %q (valid: strip, reject, off)", ErrUnknownMode, raw)
	}
}

// --- FaithfulnessMetrics ---

// outcomesMetricName is the canonical Prometheus metric name for faithfulness outcomes.
// Frozen per SPEC-SYN-002 §2.1(f).
const outcomesMetricName = "usearch_synthesis_faithfulness_outcomes_total"

// retriesMetricName is the canonical Prometheus metric name for faithfulness retries.
// Frozen per SPEC-SYN-002 §2.1(f).
const retriesMetricName = "usearch_synthesis_faithfulness_retries_total"

// OutcomesMetricName returns the Prometheus metric name for the outcomes counter.
// Exposed for test assertions.
func OutcomesMetricName() string { return outcomesMetricName }

// RetriesMetricName returns the Prometheus metric name for the retries counter.
// Exposed for test assertions.
func RetriesMetricName() string { return retriesMetricName }

// FaithfulnessMetrics holds the two Prometheus collectors declared by SPEC-SYN-002 §2.1(f).
//
//   - Outcomes: CounterVec{outcome} — one of the six FaithfulnessOutcome label values
//   - Retries: Counter (no labels) — total retry count across all requests
//
// @MX:ANCHOR: [AUTO] Faithfulness metric bundle; callers: RecordOutcome, RecordRetry, obs re-exports, metrics.Registry
// @MX:REASON: fan_in >= 3; both collectors must be registered exactly once per Registry
// @MX:SPEC: SPEC-SYN-002 §2.1(f)
type FaithfulnessMetrics struct {
	// Outcomes counts faithfulness gate outcomes, partitioned by outcome label.
	Outcomes *prometheus.CounterVec
	// Retries counts the total number of single-retry attempts.
	Retries prometheus.Counter
}

// NewFaithfulnessMetrics creates FaithfulnessMetrics with unregistered collectors.
// Suitable for test use where registration on a real registry is not required.
// For production use, call RegisterFaithfulnessMetrics instead.
func NewFaithfulnessMetrics() *FaithfulnessMetrics {
	outcomes := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: outcomesMetricName,
			Help: "Faithfulness gate outcomes per request, partitioned by outcome. SPEC-SYN-002.",
		},
		[]string{"outcome"},
	)
	retries := prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: retriesMetricName,
			Help: "Total faithfulness retry attempts. One retry max per request. SPEC-SYN-002.",
		},
	)
	m := &FaithfulnessMetrics{Outcomes: outcomes, Retries: retries}
	m.preInitOutcomes()
	return m
}

// preInitOutcomes pre-initialises all outcome label values to 0 so that the
// metric family appears in /metrics output before any synthesis calls (REQ-OBS-004).
func (m *FaithfulnessMetrics) preInitOutcomes() {
	for _, o := range AllOutcomes() {
		m.Outcomes.WithLabelValues(o.String()).Add(0)
	}
}

// RecordOutcome increments the outcomes counter for the given outcome.
func (m *FaithfulnessMetrics) RecordOutcome(o FaithfulnessOutcome) {
	m.Outcomes.WithLabelValues(o.String()).Inc()
}

// RecordRetry increments the retries counter by one.
func (m *FaithfulnessMetrics) RecordRetry() {
	m.Retries.Inc()
}

// NewPrometheusRegistry creates a new isolated prometheus.Registry for testing.
// Production code should use the shared metrics.Registry via RegisterFaithfulnessMetrics.
func NewPrometheusRegistry() *prometheus.Registry {
	return prometheus.NewRegistry()
}

// RegisterFaithfulnessMetrics creates and registers the two faithfulness collectors
// on the provided prometheus.Registry. Returns an error on duplicate registration
// rather than panicking (unlike MustRegister).
//
// This function is called by metrics.Registry via registerSynthesis to extend
// SPEC-SYN-001 collectors per SPEC-SYN-002 §2.1(g).
func RegisterFaithfulnessMetrics(reg *prometheus.Registry) (*FaithfulnessMetrics, error) {
	outcomes := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: outcomesMetricName,
			Help: "Faithfulness gate outcomes per request, partitioned by outcome. SPEC-SYN-002.",
		},
		[]string{"outcome"},
	)
	retries := prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: retriesMetricName,
			Help: "Total faithfulness retry attempts. One retry max per request. SPEC-SYN-002.",
		},
	)

	if err := reg.Register(outcomes); err != nil {
		return nil, fmt.Errorf("citation: register outcomes counter: %w", err)
	}
	if err := reg.Register(retries); err != nil {
		// Roll back already-registered outcomes to keep registry consistent.
		reg.Unregister(outcomes)
		return nil, fmt.Errorf("citation: register retries counter: %w", err)
	}

	m := &FaithfulnessMetrics{Outcomes: outcomes, Retries: retries}
	m.preInitOutcomes()
	return m, nil
}
