// Package metrics — adapter reliability collectors (SPEC-EVAL-002).
//
// REQ-EVAL2-003: three new metric families layered on top of the existing
// usearch_adapter_calls_total Counter (SPEC-OBS-001):
//   - usearch_fanout_partial_total   (CounterVec, label: adapter)
//   - usearch_adapter_health_status  (GaugeVec, label: adapter; 1.0/0.5/0.0)
//   - usearch_adapter_circuit_state  (GaugeVec, labels: adapter, state)
//
// The circuit-state family is registered for forward compatibility only; no
// upstream emits it in V1 (amendment A2). It stays at the default `closed`
// value until a future resilience SPEC wires real circuit transitions.
//
// NFR-EVAL2-001: the only new label NAME introduced is `state`; `adapter`,
// `outcome`, and `reason` are reused from the existing allowlist.
package metrics

import "github.com/prometheus/client_golang/prometheus"

// adapterReliabilityCollectors bundles the three SPEC-EVAL-002 metric families.
type adapterReliabilityCollectors struct {
	fanoutPartial *prometheus.CounterVec
	healthStatus  *prometheus.GaugeVec
	circuitState  *prometheus.GaugeVec
}

// circuitStates enumerates the bounded value set for the `state` label.
// Keeping this list closed enforces the cardinality budget (12 adapters × 3
// states = 36 series ceiling).
var circuitStates = []string{"closed", "open", "half_open"}

// registerAdapterReliability registers the three SPEC-EVAL-002 metric families
// on pr and returns handles to them. Mirrors the registerLLM/registerRouter
// helper pattern used elsewhere in this package.
//
// @MX:ANCHOR: [AUTO] SPEC-EVAL-002 adapter-reliability collectors; callers: NewRegistry, tests, fanout dispatch
// @MX:REASON: fan_in >= 3; sole registration point for the partial/health/circuit families that the dashboard + alerts depend on
// @MX:SPEC: SPEC-EVAL-002 REQ-EVAL2-003
func registerAdapterReliability(pr *prometheus.Registry) *adapterReliabilityCollectors {
	fanoutPartial := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "usearch_fanout_partial_total",
			Help: "Total fanout dispatches in which this adapter contributed an error to the partial-result set, partitioned by adapter.",
		},
		[]string{"adapter"},
	)

	healthStatus := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "usearch_adapter_health_status",
			Help: "Adapter health status derived from the 7d success-rate thresholds: 1.0 healthy, 0.5 degraded, 0.0 unhealthy.",
		},
		[]string{"adapter"},
	)

	circuitState := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "usearch_adapter_circuit_state",
			Help: "Adapter circuit-breaker state (1.0 when the adapter is in that state). Registered for forward compatibility; no upstream emits it in V1 (SPEC-EVAL-002 amendment A2).",
		},
		[]string{"adapter", "state"},
	)

	pr.MustRegister(fanoutPartial, healthStatus, circuitState)

	// Pre-initialise each family with a placeholder so the metric family
	// appears in /metrics output even before any real observation (REQ-OBS-004
	// pattern shared with the existing collectors). The circuit-state family is
	// pre-initialised to the default `closed` value for every enum member so the
	// family is queryable from V1 even though nothing emits real transitions.
	fanoutPartial.WithLabelValues("").Add(0)
	healthStatus.WithLabelValues("").Set(0)
	for _, st := range circuitStates {
		circuitState.WithLabelValues("", st).Set(0)
	}

	return &adapterReliabilityCollectors{
		fanoutPartial: fanoutPartial,
		healthStatus:  healthStatus,
		circuitState:  circuitState,
	}
}
