// Package metrics provides the fanout partial, adapter health, and circuit
// state metric families for SPEC-EVAL-002.
//
// REQ-EVAL2-003: Three new metric families for adapter reliability monitoring.
// NFR-EVAL2-001: Bounded cardinality (≤ 500 series total).
package metrics

import "github.com/prometheus/client_golang/prometheus"

// fanoutPartialCollectors holds the SPEC-EVAL-002 metric families.
type fanoutPartialCollectors struct {
	fanoutPartial *prometheus.CounterVec
	healthStatus  *prometheus.GaugeVec
	circuitState  *prometheus.GaugeVec
}

// registerFanoutPartial creates and registers the three metric families
// for adapter reliability monitoring (SPEC-EVAL-002 REQ-EVAL2-003).
//
// @MX:ANCHOR: [AUTO] Fanout partial metric registration; callers: NewRegistry, tests
// @MX:REASON: fan_in >= 3; all three EVAL-002 families are registered here
func registerFanoutPartial(pr *prometheus.Registry) fanoutPartialCollectors {
	fanoutPartial := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "usearch_fanout_partial_total",
			Help: "Number of fanout dispatches where the named adapter contributed an error to Result.AdapterErrors. Incremented once per adapter per dispatch.",
		},
		[]string{"adapter"},
	)

	healthStatus := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "usearch_adapter_health_status",
			Help: "Per-adapter health status: 1.0=healthy (≥0.95 7d success rate), 0.5=degraded (0.85-0.95), 0.0=unhealthy (<0.85). Updated by the admin health endpoint or a recording-rule companion job.",
		},
		[]string{"adapter"},
	)

	circuitState := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "usearch_adapter_circuit_state",
			Help: "Per-adapter circuit breaker state: 1 when the adapter is in the named state. States: closed (healthy), open (failing), half_open (probing).",
		},
		[]string{"adapter", "state"},
	)

	pr.MustRegister(fanoutPartial, healthStatus, circuitState)

	// Pre-initialise with placeholder label values so metric families appear
	// in /metrics output from first scrape (SPEC-OBS-001 pattern).
	fanoutPartial.WithLabelValues("").Add(0)
	healthStatus.WithLabelValues("").Set(0)
	for _, state := range []string{"closed", "open", "half_open"} {
		circuitState.WithLabelValues("", state).Set(0)
	}

	return fanoutPartialCollectors{
		fanoutPartial: fanoutPartial,
		healthStatus:  healthStatus,
		circuitState:  circuitState,
	}
}
