package youtube

import (
	"os"
	"testing"
	"time"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// BenchmarkParseSearchResponse25Videos measures parse throughput for a 25-item
// response payload. Target: < 1 ms/op per NFR-ADP5-001.
func BenchmarkParseSearchResponse25Videos(b *testing.B) {
	data, err := os.ReadFile("testdata/search_response.json")
	if err != nil {
		b.Fatalf("read fixture: %v", err)
	}
	fixedTime := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := parseSearchResponse(data, fixedTime, 0, "en")
		if err != nil {
			b.Fatalf("parseSearchResponse: %v", err)
		}
	}
}
