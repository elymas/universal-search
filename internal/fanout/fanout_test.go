package fanout_test

import (
	"context"
	"errors"
	"testing"

	"github.com/elymas/universal-search/internal/fanout"
	"github.com/elymas/universal-search/internal/router"
	"github.com/elymas/universal-search/pkg/types"
)

// TestDispatchAlwaysReturnsResult verifies that *Result is non-nil for success, partial, and failure paths.
func TestDispatchAlwaysReturnsResult(t *testing.T) {
	t.Parallel()
	ad1 := &stubAdapter{name: "ad1", docs: makeDocs("ad1", 3)}
	ad2 := &stubAdapter{name: "ad2", err: errors.New("boom")}
	reg := buildTestRegistry(ad1, ad2)
	f, err := fanout.New(fanout.Options{Registry: reg})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx := context.Background()
	for i := range 10 {
		result, _ := f.Dispatch(ctx, makeDecision("ad1", "ad2"), types.Query{Text: "test"})
		if result == nil {
			t.Fatalf("iteration %d: Dispatch returned nil result", i)
		}
	}
}

// TestDispatchEmptyAdapterSetRejected verifies ErrEmptyAdapterSet on empty AdapterSet.
func TestDispatchEmptyAdapterSetRejected(t *testing.T) {
	t.Parallel()
	reg := buildTestRegistry(&stubAdapter{name: "ad1"})
	f, err := fanout.New(fanout.Options{Registry: reg})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx := context.Background()

	// nil AdapterSet.
	result, dErr := f.Dispatch(ctx, router.RoutingDecision{AdapterSet: nil}, types.Query{})
	if !errors.Is(dErr, fanout.ErrEmptyAdapterSet) {
		t.Fatalf("nil AdapterSet: want ErrEmptyAdapterSet, got %v", dErr)
	}
	if result.Stats.AdapterCount != 0 {
		t.Fatalf("nil AdapterSet: want AdapterCount==0, got %d", result.Stats.AdapterCount)
	}
	if len(result.Docs) != 0 {
		t.Fatalf("nil AdapterSet: want len(Docs)==0, got %d", len(result.Docs))
	}
	if len(result.AdapterErrors) != 0 {
		t.Fatalf("nil AdapterSet: want len(AdapterErrors)==0, got %d", len(result.AdapterErrors))
	}

	// empty slice AdapterSet.
	result2, dErr2 := f.Dispatch(ctx, router.RoutingDecision{AdapterSet: []string{}}, types.Query{})
	if !errors.Is(dErr2, fanout.ErrEmptyAdapterSet) {
		t.Fatalf("empty AdapterSet: want ErrEmptyAdapterSet, got %v", dErr2)
	}
	if result2.Stats.AdapterCount != 0 {
		t.Fatalf("empty AdapterSet: want AdapterCount==0, got %d", result2.Stats.AdapterCount)
	}
	// Goroutine-leak detection is handled suite-wide by goleak.VerifyTestMain
	// (bench_test.go). A per-test runtime.NumGoroutine() delta is flaky under
	// t.Parallel() because the global count includes sibling parallel tests.
}
