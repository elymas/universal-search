// Package naver — benchmark and goroutine-leak guard tests.
// goleak is imported here so all test files in this package share the
// TestMain setup for goroutine-leak detection.
package naver

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

// BenchmarkParseBlogResponse25Items measures parsing performance for the standard
// 25-document blog fixture. Target: p50 ≤ 5ms / allocs ≤ 500.
//
// @MX:NOTE: [AUTO] Performance sentinel — if allocs/op or ns/op regress beyond
// the targets, investigate the transformation or JSON decode path.
// @MX:SPEC: SPEC-ADP-008
func BenchmarkParseBlogResponse25Items(b *testing.B) {
	body, err := os.ReadFile("testdata/search_response_blog.json")
	if err != nil {
		b.Fatalf("os.ReadFile() error = %v", err)
	}

	retrievedAt := time.Now().UTC()

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		docs, parseErr := parseBlogResponse(body, retrievedAt)
		if parseErr != nil {
			b.Fatalf("parseBlogResponse() error = %v", parseErr)
		}
		if len(docs) != 25 {
			b.Fatalf("parseBlogResponse() returned %d docs, want 25", len(docs))
		}
	}
}
