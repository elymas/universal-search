// Package synthcluster_test — RED phase tests for SimHash computation.
// SPEC-SYN-003 REQ-SYN3-001: deterministic 64-bit Charikar SimHash.
package synthcluster_test

import (
	"testing"

	"github.com/elymas/universal-search/internal/synthcluster"
)

// test_simhash_deterministic: simHash64 must return identical values for
// byte-identical inputs across 100 invocations (REQ-SYN3-001).
func TestSimHashDeterministic(t *testing.T) {
	t.Parallel()
	text := "Hello, world! 안녕하세요."
	first := synthcluster.SimHash64(text)
	for i := range 100 {
		got := synthcluster.SimHash64(text)
		if got != first {
			t.Fatalf("iteration %d: SimHash64 not deterministic: got %d, want %d", i, got, first)
		}
	}
}

// test_simhash_different_inputs: similar but not identical texts should
// often produce different hashes (not a strong guarantee — just smoke).
func TestSimHashDifferentInputs(t *testing.T) {
	t.Parallel()
	a := synthcluster.SimHash64("The quick brown fox jumps over the lazy dog")
	b := synthcluster.SimHash64("The quick brown fox jumps over the lazy cat")
	if a == b {
		// SimHash collisions are possible but should be rare for clearly different text.
		t.Log("WARN: SimHash64 produced the same hash for clearly different inputs — may indicate implementation bug")
	}
}

// TestSimHashEmptyString: empty string should not panic and should be deterministic.
func TestSimHashEmptyString(t *testing.T) {
	t.Parallel()
	h1 := synthcluster.SimHash64("")
	h2 := synthcluster.SimHash64("")
	if h1 != h2 {
		t.Fatalf("SimHash64(\"\") not deterministic: %d != %d", h1, h2)
	}
}

// TestSimHashHammingDistance: HammingDistance utility must count differing bits.
func TestSimHashHammingDistance(t *testing.T) {
	t.Parallel()
	tests := []struct {
		a, b uint64
		want int
	}{
		{0, 0, 0},
		{0xFFFFFFFFFFFFFFFF, 0, 64},
		{0b1010, 0b1100, 2}, // bits 1,2 differ; bits 3 differ from bit 3 perspective
		{1, 0, 1},
	}
	for _, tc := range tests {
		got := synthcluster.HammingDistance(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("HammingDistance(%016x, %016x) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}
