// Package hn — NFR-ADP2-001 benchmark: parse latency and allocation budget.
// Run: go test -bench=BenchmarkParseHits25Hits -benchmem ./internal/adapters/hn/
// Target: p95 ≤ 5ms, ≤ 500 allocs/op per NFR-ADP2-001.
package hn

import (
	"os"
	"testing"
	"time"
)

// BenchmarkParseHits25Hits measures the parseHits() cost for the 25-story
// fixture. NFR-ADP2-001 requires ≤ 5ms wall time and ≤ 500 allocs/op.
func BenchmarkParseHits25Hits(b *testing.B) {
	body, err := os.ReadFile("testdata/search_response.json")
	if err != nil {
		b.Fatalf("os.ReadFile(search_response.json): %v", err)
	}

	now := time.Now().UTC()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _, err := parseHits(body, now)
		if err != nil {
			b.Fatalf("parseHits error: %v", err)
		}
	}
}
