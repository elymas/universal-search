// Package youtube — view-count score normalization.
// REQ-ADP5-005 / D5: Tanh-of-log10 formula distinct from Reddit/HN's Tanh-of-(score/100).
package youtube

import "math"

// log10Divisor is the denominator in the Tanh-of-log10 formula.
// Empirical value: spreads [0, 10B] view-count range linearly across [0, 10]
// so Tanh can squish into [0.5, 1.0] with meaningful gradient at every decade.
//
// @MX:NOTE: [AUTO] Magic constant. Revisit after SPEC-IDX-001 RRF integration
// measures ranking quality. Open Question §11.5 tracks the trigger.
// @MX:SPEC: SPEC-ADP-005
const log10Divisor = 5.0

// scoreCenter is the midpoint score (zero-engagement → 0.5).
//
// @MX:NOTE: [AUTO] Matches Reddit/HN semantic: zero views = neutral score.
// @MX:SPEC: SPEC-ADP-005
const scoreCenter = 0.5

// normalizeViewScore maps a YouTube view count to a score in [0.0, 1.0] using
// the Tanh-of-log10 formula:
//
//	Score = clamp(0.5 + 0.5 * tanh(log10(viewCount + 1) / 5.0), 0.0, 1.0)
//
// Properties:
//   - viewCount=0 → Score=0.5 (neutral; the +1 prevents log10(0)).
//   - viewCount=100K → Score≈0.881 (inflection point).
//   - viewCount=10B → Score≈0.982 (saturation; not 1.0).
//   - Pure function: no state, no I/O, no time.
func normalizeViewScore(viewCount int64) float64 {
	if viewCount < 0 {
		viewCount = 0
	}
	log := math.Log10(float64(viewCount) + 1)
	score := scoreCenter + scoreCenter*math.Tanh(log/log10Divisor)
	if score < 0.0 {
		return 0.0
	}
	if score > 1.0 {
		return 1.0
	}
	return score
}
