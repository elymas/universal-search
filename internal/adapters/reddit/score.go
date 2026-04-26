// Package reddit — score normalization for Reddit adapter.
// REQ-ADP-006: Score normalizer maps Reddit integer scores to [0.0, 1.0].
package reddit

import "math"

// tanhDivisor is the divisor applied to the Reddit integer score before
// passing to math.Tanh. Empirically chosen so that score=100 maps to
// approximately 0.88, matching intuitive "good post" perception.
//
// @MX:NOTE: [AUTO] Empirical inflection point: score=100 -> ~0.88.
// Open Question §11.2 in SPEC-ADP-001 tracks revisit triggers post-M3.
// @MX:SPEC: SPEC-ADP-001
const tanhDivisor = 100.0

// scoreCenter is the midpoint of the output range [0.0, 1.0].
// A Reddit post with score=0 (neutral, zero net upvotes) maps to exactly 0.5.
//
// @MX:NOTE: [AUTO] Semantic center: score=0 -> 0.5 (neutral).
// Changing this constant shifts the entire score distribution and requires
// coordination with SPEC-IDX-001 RRF tuning.
// @MX:SPEC: SPEC-ADP-001
const scoreCenter = 0.5

// normalizeScore maps a Reddit integer score to the [0.0, 1.0] range using
// the hyperbolic tangent formula: clamp(0.5 + 0.5*tanh(score/100.0), 0.0, 1.0).
//
// Properties:
//   - score = 0  -> 0.5 (neutral)
//   - score = 100 -> ~0.881 (good post)
//   - score = 1000 -> ~1.0 (saturated)
//   - score = -1000 -> ~0.0 (saturated negative)
//
// Deterministic pure function: no state, no I/O, same input always produces
// identical float64 output.
func normalizeScore(redditScore int) float64 {
	v := scoreCenter + scoreCenter*math.Tanh(float64(redditScore)/tanhDivisor)
	// clamp to [0.0, 1.0] to guard against floating-point edge cases
	return math.Max(0.0, math.Min(1.0, v))
}
