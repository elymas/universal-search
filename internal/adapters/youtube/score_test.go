package youtube

import (
	"math"
	"testing"
)

func TestNormalizeViewScoreTable(t *testing.T) {
	t.Parallel()
	// Expected values computed from the SPEC-ADP-005 §2.3 formula:
	// Score = clamp(0.5 + 0.5 * tanh(log10(viewCount + 1) / 5.0), 0.0, 1.0)
	// Verified via Python: math.tanh(math.log10(v+1)/5.0)*0.5 + 0.5
	// Note: SPEC §2.3 table shows rounded tanh values; exact Go/Python results
	// differ slightly for large v due to the +1 inside log10. Tolerance is ±0.001.
	cases := []struct {
		viewCount int64
		want      float64
	}{
		{0, 0.500000},              // log10(1)=0, tanh(0)=0, Score=0.5
		{1, 0.530067},              // log10(2)≈0.301, tanh(0.0602)≈0.0601, Score≈0.530
		{100, 0.690344},            // log10(101)≈2.004, tanh(0.4009)≈0.381, Score≈0.690
		{10_000, 0.832021},         // log10(10001)≈4.000, tanh(0.800)≈0.664, Score≈0.832
		{1_000_000, 0.916827},      // log10(1000001)≈6.000, tanh(1.200)≈0.834, Score≈0.917
		{100_000_000, 0.960834},    // log10(1e8+1)≈8.000, tanh(1.600)≈0.922, Score≈0.961
		{10_000_000_000, 0.982014}, // log10(1e10+1)≈10.000, tanh(2.000)≈0.964, Score≈0.982
	}
	const tol = 0.001
	for _, tc := range cases {
		got := normalizeViewScore(tc.viewCount)
		if math.Abs(got-tc.want) > tol {
			t.Errorf("normalizeViewScore(%d) = %.6f, want %.6f (±%.3f)", tc.viewCount, got, tc.want, tol)
		}
	}
}

func TestNormalizeViewScoreDeterministic(t *testing.T) {
	t.Parallel()
	v := int64(50_000)
	s1 := normalizeViewScore(v)
	s2 := normalizeViewScore(v)
	if s1 != s2 {
		t.Errorf("normalizeViewScore(%d) not deterministic: %v vs %v", v, s1, s2)
	}
}

func TestNormalizeViewScoreZeroIs05(t *testing.T) {
	t.Parallel()
	got := normalizeViewScore(0)
	if got != 0.5 {
		t.Errorf("normalizeViewScore(0) = %v, want 0.5 exactly", got)
	}
}

func TestNormalizeViewScoreInRange(t *testing.T) {
	t.Parallel()
	for _, v := range []int64{0, 1, 1000, 1_000_000, math.MaxInt64} {
		got := normalizeViewScore(v)
		if got < 0.0 || got > 1.0 {
			t.Errorf("normalizeViewScore(%d) = %v out of [0,1]", v, got)
		}
	}
}
