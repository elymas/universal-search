// Package adapters — per-adapter call telemetry snapshot (SPEC-EVAL-002
// REQ-EVAL2-010a). Reads the in-process usearch_adapter_calls_total CounterVec
// (SPEC-OBS-001) via the Prometheus registry's Gather() so the admin handler
// can surface real success/fail counts without a separate counter store.
package adapters

import (
	"github.com/prometheus/client_golang/prometheus"
)

// adapterCallStats holds the success/fail tallies for a single adapter derived
// from the usearch_adapter_calls_total counter family.
type adapterCallStats struct {
	success int64
	fail    int64
}

// successRate returns success / (success + fail) in [0.0, 1.0]. Returns 0 when
// no calls have been recorded. The denominator includes ALL six outcome values
// per HISTORY D1 (success + the five non-success outcomes); `transient` counts
// as a failed call.
func (s adapterCallStats) successRate() float64 {
	total := s.success + s.fail
	if total == 0 {
		return 0
	}
	return float64(s.success) / float64(total)
}

// callStatsByAdapter gathers usearch_adapter_calls_total from pr and returns a
// per-adapter success/fail tally. Outcome "success" counts toward success; the
// other five canonical outcomes (failure/timeout/rate_limited/unavailable/
// transient) count toward fail (HISTORY D1). Returns an empty map when the
// family is absent (e.g., nil registry caller already guarded).
//
// @MX:NOTE: [AUTO] SPEC-EVAL-002 REQ-EVAL2-010a in-process counter snapshot.
// Reads the existing usearch_adapter_calls_total family; introduces no new
// counter store (run-phase decision: Collector.Gather over sync/atomic).
// @MX:SPEC: SPEC-EVAL-002
func callStatsByAdapter(pr *prometheus.Registry) map[string]adapterCallStats {
	out := make(map[string]adapterCallStats)
	if pr == nil {
		return out
	}
	families, err := pr.Gather()
	if err != nil {
		return out
	}
	for _, fam := range families {
		if fam.GetName() != "usearch_adapter_calls_total" {
			continue
		}
		for _, m := range fam.GetMetric() {
			var adapter, outcome string
			for _, lp := range m.GetLabel() {
				switch lp.GetName() {
				case "adapter":
					adapter = lp.GetValue()
				case "outcome":
					outcome = lp.GetValue()
				}
			}
			if adapter == "" {
				continue // skip the empty-string pre-initialisation placeholder
			}
			val := int64(m.GetCounter().GetValue())
			stat := out[adapter]
			if outcome == "success" {
				stat.success += val
			} else {
				stat.fail += val
			}
			out[adapter] = stat
		}
	}
	return out
}

// callStats returns the per-adapter telemetry snapshot for this registry,
// reading from the wired obs.Metrics registry. Returns an empty map when obs or
// its metrics registry is nil (graceful degradation, matches wrappedAdapter).
func (r *Registry) callStats() map[string]adapterCallStats {
	if r.obs == nil || r.obs.Metrics == nil {
		return map[string]adapterCallStats{}
	}
	return callStatsByAdapter(r.obs.Metrics.Prometheus)
}
