// Package social — tests for score normalization.
// REQ-ADP6-006: normalizeScore maps (likeCount + repostCount) to [0.0, 1.0].
package social

import (
	"math"
	"testing"
)

// TestNormalizeScoreTable verifies the tanh-based score formula.
func TestNormalizeScoreTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		likes      int
		reposts    int
		wantApprox float64
		epsilon    float64
	}{
		{
			name:       "zero engagement maps to 0.5",
			likes:      0,
			reposts:    0,
			wantApprox: 0.5,
			epsilon:    1e-9,
		},
		{
			name:       "likes=100 reposts=0 maps near 0.88",
			likes:      100,
			reposts:    0,
			wantApprox: 0.5 + 0.5*math.Tanh(1.0), // tanh(100/100) = tanh(1)
			epsilon:    1e-9,
		},
		{
			name:       "likes=0 reposts=100 same as 100 likes",
			likes:      0,
			reposts:    100,
			wantApprox: 0.5 + 0.5*math.Tanh(1.0), // tanh(100/100) = tanh(1)
			epsilon:    1e-9,
		},
		{
			name:       "combined likes+reposts=100",
			likes:      50,
			reposts:    50,
			wantApprox: 0.5 + 0.5*math.Tanh(1.0), // tanh(100/100) = tanh(1)
			epsilon:    1e-9,
		},
		{
			name:       "high engagement saturates near 1.0",
			likes:      1000,
			reposts:    500,
			wantApprox: 1.0,
			epsilon:    0.001,
		},
		{
			name:       "result always >= 0",
			likes:      0,
			reposts:    0,
			wantApprox: 0.5,
			epsilon:    0.5, // just verify it's in [0,1]
		},
		{
			name:       "result always <= 1",
			likes:      1000000,
			reposts:    1000000,
			wantApprox: 1.0,
			epsilon:    1e-9,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := normalizeScore(tc.likes, tc.reposts)
			if got < 0.0 || got > 1.0 {
				t.Errorf("normalizeScore(%d, %d) = %f: out of [0,1] range", tc.likes, tc.reposts, got)
			}
			diff := math.Abs(got - tc.wantApprox)
			if diff > tc.epsilon {
				t.Errorf("normalizeScore(%d, %d) = %f, want %f ± %f", tc.likes, tc.reposts, got, tc.wantApprox, tc.epsilon)
			}
		})
	}
}
