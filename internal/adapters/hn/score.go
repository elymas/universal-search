// Package hn — score normalization for the Hacker News adapter.
// REQ-ADP2-005: normalizeScore maps HN integer points to [0.0, 1.0].
package hn

import "math"

// tanhDivisor is the divisor applied to the HN integer points before
// passing to math.Tanh. Identical to SPEC-ADP-001 §2.3; HN points
// distribution behaves similarly to Reddit upvotes for the typical [0, ~5000]
// range, so the formula's inflection point at 100 remains operationally meaningful.
//
// @MX:NOTE: [AUTO] Empirical inflection point: points=100 -> ~0.88.
// Identical formula to internal/adapters/reddit/score.go. Rule of three deferred
// to SPEC-ADP-REFAC-001 post-M3. Open Question §11.5 in SPEC-ADP-002 tracks
// revisit triggers if HN-specific calibration is needed.
// @MX:SPEC: SPEC-ADP-002
const tanhDivisor = 100.0

// scoreCenter is the midpoint of the output range [0.0, 1.0].
// A post with points=0 maps to exactly 0.5 (neutral).
//
// @MX:NOTE: [AUTO] Semantic center: points=0 -> 0.5 (neutral).
// Changing this constant shifts the entire score distribution and requires
// coordination with SPEC-IDX-001 RRF tuning.
// @MX:SPEC: SPEC-ADP-002
const scoreCenter = 0.5

// normalizeScore maps a HN integer points value to the [0.0, 1.0] range using
// the hyperbolic tangent formula: clamp(0.5 + 0.5*tanh(points/100.0), 0.0, 1.0).
//
// Properties:
//   - points = 0    -> 0.5 (neutral, new submission)
//   - points = 100  -> ~0.881 (well-received post)
//   - points = 1000 -> ~1.0 (saturated, very popular)
//
// Deterministic pure function: no state, no I/O.
func normalizeScore(points int) float64 {
	v := scoreCenter + scoreCenter*math.Tanh(float64(points)/tanhDivisor)
	// clamp to [0.0, 1.0] to guard against floating-point edge cases
	return math.Max(0.0, math.Min(1.0, v))
}
