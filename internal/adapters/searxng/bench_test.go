package searxng_test

import (
	"os"
	"testing"
	"time"

	"go.uber.org/goleak"
)

// TestMain enables goroutine leak detection for the entire test suite.
// goleak.VerifyTestMain ensures no goroutines are leaked after all tests complete.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// BenchmarkParseSearch10Results measures the parse throughput for 10-result responses.
func BenchmarkParseSearch10Results(b *testing.B) {
	body, err := os.ReadFile("testdata/search_response.json")
	if err != nil {
		b.Fatalf("ReadFile: %v", err)
	}
	now := time.Now().UTC()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		docs, _, err := exportedParseSearch(body, now, 1)
		if err != nil {
			b.Fatalf("parseSearch: %v", err)
		}
		if len(docs) == 0 {
			b.Fatal("no docs")
		}
	}
}

// BenchmarkParseSearchEmpty measures the parse throughput for empty responses.
func BenchmarkParseSearchEmpty(b *testing.B) {
	body, err := os.ReadFile("testdata/search_response_empty.json")
	if err != nil {
		b.Fatalf("ReadFile: %v", err)
	}
	now := time.Now().UTC()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _, err := exportedParseSearch(body, now, 1)
		if err != nil {
			b.Fatalf("parseSearch: %v", err)
		}
	}
}
