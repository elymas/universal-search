package naver

import (
	"testing"
)

// TestDefaultScore verifies that the constant defaultScore equals 0.5.
// REQ-ADP8-005: fixed neutral score for all Naver results.
func TestDefaultScore(t *testing.T) {
	const want = 0.5
	if defaultScore != want {
		t.Errorf("defaultScore = %v, want %v", defaultScore, want)
	}
}
