// Package github — score normalization tests.
// Tests #56–57: REQ-ADP4-005 normalizeScore.
package github

import (
	"math"
	"testing"
)

// TestNormalizeScoreTable verifies 7 known values within ±0.001.
func TestNormalizeScoreTable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		input      int
		wantApprox float64
	}{
		{0, 0.500},
		{100, 0.881},
		{1000, 1.000},
		{-1000, 0.000},
		{50, 0.731},
		{-50, 0.269},
		{10, 0.550},
	}
	for _, tc := range cases {
		got := normalizeScore(tc.input)
		if math.Abs(got-tc.wantApprox) > 0.001 {
			t.Errorf("normalizeScore(%d) = %.6f, want %.3f ± 0.001",
				tc.input, got, tc.wantApprox)
		}
		if got < 0.0 || got > 1.0 {
			t.Errorf("normalizeScore(%d) = %.6f out of [0,1]", tc.input, got)
		}
	}
}

// TestNormalizeScoreDeterministic verifies that two calls with the same input
// produce identical float64 output (no hidden state).
func TestNormalizeScoreDeterministic(t *testing.T) {
	t.Parallel()
	for _, v := range []int{0, 42, -42, 1000, -1000} {
		a := normalizeScore(v)
		b := normalizeScore(v)
		if a != b {
			t.Errorf("normalizeScore(%d) non-deterministic: %v vs %v", v, a, b)
		}
	}
}
