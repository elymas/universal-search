// Package social — benchmarks and TestMain for goroutine leak detection.
package social

import (
	"os"
	"testing"
	"time"

	"go.uber.org/goleak"
)

// TestMain sets up goleak to detect goroutine leaks in all tests in this package.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// BenchmarkParseSearchPosts25Docs measures parse throughput for the 25-post fixture.
func BenchmarkParseSearchPosts25Docs(b *testing.B) {
	body, err := os.ReadFile(testdataPath + "bluesky_search_response.json")
	if err != nil {
		b.Fatalf("ReadFile: %v", err)
	}

	retrievedAt := time.Now()
	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		_, _, err := parseSearchPosts(body, retrievedAt)
		if err != nil {
			b.Fatalf("parseSearchPosts: %v", err)
		}
	}
}
