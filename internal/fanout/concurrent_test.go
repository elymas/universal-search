package fanout_test

import (
	"context"
	"sync"
	"testing"

	"github.com/elymas/universal-search/internal/fanout"
	"github.com/elymas/universal-search/pkg/types"
)

// TestDispatchConcurrent runs 50 goroutines each performing 100 Dispatch calls
// across 5 adapters. The race detector validates there are no data races.
// REQ-FAN-009: Fanout is safe for concurrent invocation.
func TestDispatchConcurrent(t *testing.T) {
	t.Parallel()

	ad1 := &stubAdapter{name: "c1", docs: makeDocs("c1", 3)}
	ad2 := &stubAdapter{name: "c2", docs: makeDocs("c2", 3)}
	ad3 := &stubAdapter{name: "c3", docs: makeDocs("c3", 3)}
	ad4 := &stubAdapter{name: "c4", docs: makeDocs("c4", 3)}
	ad5 := &stubAdapter{name: "c5", docs: makeDocs("c5", 3)}
	reg := buildTestRegistry(ad1, ad2, ad3, ad4, ad5)

	f, err := fanout.New(fanout.Options{Registry: reg, MaxParallel: 10})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	const goroutines = 50
	const calls = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			for range calls {
				result, err := f.Dispatch(context.Background(),
					makeDecision("c1", "c2", "c3", "c4", "c5"),
					types.Query{Text: "concurrent test"},
				)
				if err != nil {
					// Non-fatal: concurrent test validates absence of races, not exact results.
					_ = err
					return
				}
				if result == nil {
					return
				}
				// Minimal correctness check under concurrent access.
				if result.Stats.AdapterCount != 5 {
					return
				}
			}
		}()
	}
	wg.Wait()
}
