// Package hn — score normalization tests.
// TestNormalizeScoreTable, TestNormalizeScoreDeterministic.
package hn

import (
	"math"
	"testing"
)

// TestNormalizeScoreTable validates the Tanh score formula over 7 representative
// point values. Expected values are computed from clamp(0.5 + 0.5*tanh(x/100.0), 0, 1).
func TestNormalizeScoreTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		points int
		// expected is the approximate value; assertion uses ±0.001 tolerance.
		expected float64
	}{
		{-1000, 0.5 + 0.5*math.Tanh(-1000.0/100.0)}, // saturated low
		{-10, 0.5 + 0.5*math.Tanh(-10.0/100.0)},
		{0, 0.5},   // neutral: exactly 0.5
		{10, 0.5 + 0.5*math.Tanh(10.0/100.0)},
		{100, 0.5 + 0.5*math.Tanh(100.0/100.0)},  // ~0.881
		{1000, 0.5 + 0.5*math.Tanh(1000.0/100.0)}, // saturated high
		{10000, 1.0}, // expected clamp to 1.0
	}

	const tol = 0.001
	for _, tc := range cases {
		got := normalizeScore(tc.points)
		// Clamp expected to [0, 1] for the comparison (matches formula).
		exp := math.Max(0.0, math.Min(1.0, tc.expected))
		if math.Abs(got-exp) > tol {
			t.Errorf("normalizeScore(%d) = %.6f; want %.6f ±%.3f", tc.points, got, exp, tol)
		}
		// Output must always be in [0, 1].
		if got < 0.0 || got > 1.0 {
			t.Errorf("normalizeScore(%d) = %.6f; out of [0,1] range", tc.points, got)
		}
	}
}

// TestNormalizeScoreDeterministic verifies that two calls with the same input
// return byte-equal float64 output.
func TestNormalizeScoreDeterministic(t *testing.T) {
	t.Parallel()

	testPoints := []int{0, 1, 100, 500, 1000, -50}
	for _, p := range testPoints {
		v1 := normalizeScore(p)
		v2 := normalizeScore(p)
		if v1 != v2 {
			t.Errorf("normalizeScore(%d) not deterministic: got %.20f then %.20f", p, v1, v2)
		}
	}
}
