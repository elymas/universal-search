package meta

import (
	"os"
	"testing"
	"time"

	"go.uber.org/goleak"
)

// TestMain runs all tests with a goroutine leak check.
// NFR-ADP10-003: no goroutine leaks across both sub-sources.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// BenchmarkParseKeywordSearch25Docs measures parse performance.
// NFR-ADP10-001: median wall-clock <= 5ms for 25 posts.
func BenchmarkParseKeywordSearch25Docs(b *testing.B) {
	body, err := os.ReadFile("testdata/threads_keyword_search_response.json")
	if err != nil {
		b.Fatalf("read fixture: %v", err)
	}
	retrievedAt := time.Now().UTC()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parseKeywordSearch(body, retrievedAt)
		if err != nil {
			b.Fatalf("parseKeywordSearch: %v", err)
		}
	}
}
