package fanout_test

import (
	"context"
	"testing"

	"go.uber.org/goleak"

	"github.com/elymas/universal-search/internal/fanout"
	"github.com/elymas/universal-search/pkg/types"
)

// TestMain enables goleak to detect goroutine leaks across the entire test suite.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m,
		// Ignore goroutines from test runner machinery and goleak's own retry mechanism.
		goleak.IgnoreTopFunction("testing.tRunner.func1"),
		goleak.IgnoreTopFunction("time.Sleep"),
	)
}

// BenchmarkDispatch5Adapters measures throughput of a 5-adapter fanout dispatch.
// REQ-FAN-011 (p50 < 10ms, p99 < 50ms for in-memory stubs).
func BenchmarkDispatch5Adapters(b *testing.B) {
	ad1 := &stubAdapter{name: "b1", docs: makeDocs("b1", 10)}
	ad2 := &stubAdapter{name: "b2", docs: makeDocs("b2", 10)}
	ad3 := &stubAdapter{name: "b3", docs: makeDocs("b3", 10)}
	ad4 := &stubAdapter{name: "b4", docs: makeDocs("b4", 10)}
	ad5 := &stubAdapter{name: "b5", docs: makeDocs("b5", 10)}
	reg := buildTestRegistry(ad1, ad2, ad3, ad4, ad5)

	f, err := fanout.New(fanout.Options{Registry: reg, MaxParallel: 8})
	if err != nil {
		b.Fatalf("New: %v", err)
	}

	ctx := context.Background()
	decision := makeDecision("b1", "b2", "b3", "b4", "b5")
	q := types.Query{Text: "bench"}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			result, err := f.Dispatch(ctx, decision, q)
			if err != nil {
				b.Errorf("Dispatch: %v", err)
				return
			}
			if result.Stats.SuccessCount != 5 {
				b.Errorf("want 5 success, got %d", result.Stats.SuccessCount)
				return
			}
		}
	})
}
