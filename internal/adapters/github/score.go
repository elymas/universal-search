// Package github — score normalization for the GitHub adapter.
// REQ-ADP4-005: normalizeScore maps integer counts to [0.0, 1.0].
package github

import "math"

// tanhDivisor is the divisor applied to the integer count before math.Tanh.
// Score=100 → ~0.881. Consistent with ADP-001/002.
//
// @MX:NOTE: [AUTO] Empirical: stars/comments=100 → ~0.881. Changing this
// shifts the entire score distribution and requires SPEC-IDX-001 RRF retuning.
// @MX:SPEC: SPEC-ADP-004
const tanhDivisor = 100.0

// scoreCenter is the midpoint of the output range [0.0, 1.0].
// A repo with 0 stars (or issue with 0 comments) maps to exactly 0.5.
//
// @MX:NOTE: [AUTO] Semantic center: count=0 → 0.5 (neutral). Changing this
// re-centers the distribution and requires SPEC-IDX-001 RRF retuning.
// @MX:SPEC: SPEC-ADP-004
const scoreCenter = 0.5

// normalizeScore maps an integer count (stars, comments) to [0.0, 1.0] using
// the hyperbolic tangent formula: clamp(0.5 + 0.5*tanh(count/100.0), 0.0, 1.0).
//
// Properties:
//   - count = 0   → 0.5 (neutral)
//   - count = 100 → ~0.881 (good signal)
//   - count = 1000 → ~1.0 (saturated)
//
// Deterministic pure function: no state, no I/O.
func normalizeScore(count int) float64 {
	v := scoreCenter + scoreCenter*math.Tanh(float64(count)/tanhDivisor)
	return math.Max(0.0, math.Min(1.0, v))
}
