// Package arxiv — benchmark and goroutine-leak guard tests.
// goleak is imported here so all test files in this package share the
// TestMain setup for goroutine-leak detection.
package arxiv

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

// BenchmarkParseFeed25Entries measures parsing performance for the standard
// 25-entry arXiv Atom XML fixture.
// NFR-ADP3-001: median p50 <= 5ms / allocs/op <= 700.
//
// // @MX:NOTE: [AUTO] Performance sentinel — if allocs/op regresses beyond 700
// // or ns/op beyond 5_000_000, investigate XML decode or transform path.
// // @MX:SPEC: SPEC-ADP-003
func BenchmarkParseFeed25Entries(b *testing.B) {
	body, err := os.ReadFile("testdata/search_response.xml")
	if err != nil {
		b.Fatalf("os.ReadFile() error = %v", err)
	}

	retrievedAt := time.Now().UTC()

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		docs, parseErr := parseFeed(body, retrievedAt)
		if parseErr != nil {
			b.Fatalf("parseFeed() error = %v", parseErr)
		}
		if len(docs) != 25 {
			b.Fatalf("parseFeed() returned %d docs, want 25", len(docs))
		}
	}
}
