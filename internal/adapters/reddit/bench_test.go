// Package reddit — benchmark and goroutine-leak guard tests.
// goleak is imported here so all test files in this package share the
// TestMain setup for goroutine-leak detection.
package reddit

import (
	"os"
	"testing"
	"time"

	"go.uber.org/goleak"
)

// TestMain sets up goleak for the whole test binary.
// Any goroutine left running after each test is reported as a leak.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// BenchmarkParseListing25Docs measures parsing performance for the standard
// 25-document fixture. The benchmark aims for p50 ≤ 5ms / allocs ≤ 500
// (NFR-ADP-001 revised in HISTORY iteration 3 from ≤ 250 after empirical baseline).
//
// @MX:NOTE: [AUTO] Performance sentinel — if allocs/op regresses beyond 500
// or ns/op beyond 5_000_000, investigate transformation or JSON decode path.
// @MX:SPEC: SPEC-ADP-001
func BenchmarkParseListing25Docs(b *testing.B) {
	body, err := os.ReadFile("testdata/search_response.json")
	if err != nil {
		b.Fatalf("os.ReadFile() error = %v", err)
	}

	retrievedAt := time.Now().UTC()

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		docs, _, err := parseListing(body, retrievedAt)
		if err != nil {
			b.Fatalf("parseListing() error = %v", err)
		}
		if len(docs) != 25 {
			b.Fatalf("parseListing() returned %d docs, want 25", len(docs))
		}
	}
}
