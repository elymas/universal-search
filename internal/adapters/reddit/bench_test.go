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
// 25-document fixture. The benchmark aims for p50 ≤ 5ms / allocs ≤ 250.
//
// @MX:NOTE: [AUTO] Performance sentinel — if allocs/op regresses beyond 250
// or ns/op beyond 5_000_000, investigate transformation or JSON decode path.
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
