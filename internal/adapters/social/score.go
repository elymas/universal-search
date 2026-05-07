// Package social — score normalization for Bluesky/social adapters.
// REQ-ADP6-006: normalizeScore maps (likeCount + repostCount) to [0.0, 1.0].
package social

import "math"

// tanhDivisor is the divisor applied before math.Tanh.
// x = likeCount + repostCount; x=100 maps to ~0.881 (good engagement).
//
// @MX:NOTE: [AUTO] Empirical inflection: x=100 -> ~0.881.
// @MX:SPEC: SPEC-ADP-006
const tanhDivisor = 100.0

// scoreCenter is the midpoint of [0.0, 1.0]; zero engagement maps to 0.5.
const scoreCenter = 0.5

// normalizeScore maps Bluesky engagement metrics to [0.0, 1.0] using the
// hyperbolic tangent formula: clamp(0.5 + 0.5*tanh((likes+reposts)/100.0), 0, 1).
//
// Properties:
//   - likes=0, reposts=0 -> 0.5 (neutral)
//   - likes=100, reposts=0 -> ~0.881 (good post)
//   - likes=1000, reposts=500 -> ~1.0 (saturated)
func normalizeScore(likeCount, repostCount int) float64 {
	x := float64(likeCount + repostCount)
	v := scoreCenter + scoreCenter*math.Tanh(x/tanhDivisor)
	return math.Max(0.0, math.Min(1.0, v))
}
