package reddit

import (
	"math"
	"testing"
)

func TestNormalizeScoreTable(t *testing.T) {
	t.Parallel()

	// Expected values computed from clamp(0.5 + 0.5*tanh(score/100.0), 0.0, 1.0)
	tests := []struct {
		score    int
		expected float64
	}{
		{-1000, 0.0},    // tanh(-10) = -1.0 -> 0.5+0.5*(-1) = 0.0
		{-10, 0.450166}, // tanh(-0.1) = -0.099668 -> 0.5-0.049834
		{0, 0.500000},   // tanh(0) = 0.0 -> 0.5
		{10, 0.549834},  // tanh(0.1) = 0.099668 -> 0.5+0.049834
		{100, 0.880797}, // tanh(1.0) = 0.761594 -> 0.5+0.380797
		{1000, 1.0},     // tanh(10) = 1.0 -> 0.5+0.5*1 = 1.0
		{10000, 1.0},    // tanh(100) = 1.0 -> saturated
	}

	const tolerance = 0.001
	for _, tc := range tests {
		got := normalizeScore(tc.score)
		if math.Abs(got-tc.expected) > tolerance {
			t.Errorf("normalizeScore(%d) = %f, want %f (tolerance ±%f)",
				tc.score, got, tc.expected, tolerance)
		}
		// Verify the result is in [0.0, 1.0]
		if got < 0.0 || got > 1.0 {
			t.Errorf("normalizeScore(%d) = %f is outside [0.0, 1.0]", tc.score, got)
		}
	}
}

func TestNormalizeScoreDeterministic(t *testing.T) {
	t.Parallel()

	scores := []int{-1000, -10, 0, 10, 100, 1000, 10000, 42, -42}
	for _, s := range scores {
		first := normalizeScore(s)
		second := normalizeScore(s)
		if first != second {
			t.Errorf("normalizeScore(%d) is not deterministic: first=%v second=%v", s, first, second)
		}
	}
}
