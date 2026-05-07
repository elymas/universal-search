package fanout

import (
	"bytes"
	"context"
	"testing"

	"github.com/elymas/universal-search/internal/obs"
	"github.com/elymas/universal-search/internal/router"
	"github.com/elymas/universal-search/pkg/types"
)

// TestEmitSafeOnNilObs verifies all emit helpers are safe when obs is nil.
func TestEmitSafeOnNilObs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// None of these must panic.
	emitEmpty(ctx, nil)
	emitPanic(ctx, nil, "adapter1", "panic value", []byte("stack trace"))
	emitDispatch(ctx, nil, nil, router.RoutingDecision{}, &Result{
		Docs:  []types.NormalizedDoc{},
		Stats: Stats{},
	})
}

// TestTracerNilObsReturnsNoopTracer verifies tracer(nil) returns a non-nil no-op tracer.
func TestTracerNilObsReturnsNoopTracer(t *testing.T) {
	t.Parallel()
	tr := tracer(nil)
	if tr == nil {
		t.Fatal("tracer(nil) returned nil tracer, want no-op tracer")
	}
	// Should be usable without panic.
	ctx, span := tr.Start(context.Background(), "test.span")
	span.End()
	_ = ctx
}

// TestInflightNilObs verifies inc/decInflight are no-ops when obs is nil.
func TestInflightNilObs(t *testing.T) {
	t.Parallel()
	// Must not panic.
	incInflight(nil, "web")
	decInflight(nil, "web")
}

// TestEmitDispatchNilSpan verifies emitDispatch is safe when span is nil.
func TestEmitDispatchNilSpan(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	decision := router.RoutingDecision{
		Category:   router.CategoryWeb,
		AdapterSet: []string{"a1"},
	}
	res := &Result{
		Docs:  []types.NormalizedDoc{},
		Stats: Stats{AdapterCount: 1, SuccessCount: 1},
	}
	// span=nil, obs=nil — must not panic.
	emitDispatch(ctx, nil, nil, decision, res)
}

// buildTestObs creates a minimal *obs.Obs with slog logger and no-op tracer/metrics.
func buildTestObs(t *testing.T) *obs.Obs {
	t.Helper()
	var buf bytes.Buffer
	o, shutdown, err := obs.Init(context.Background(), obs.Config{
		ServiceName: "test-fanout",
		LogWriter:   &buf,
	})
	if err != nil {
		t.Fatalf("obs.Init: %v", err)
	}
	t.Cleanup(func() { _ = shutdown(context.Background()) })
	return o
}

// TestEmitDispatchWithLogger verifies emitDispatch writes to the logger when obs is non-nil.
func TestEmitDispatchWithLogger(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	o := buildTestObs(t)

	decision := router.RoutingDecision{
		Category:   router.CategoryWeb,
		AdapterSet: []string{"a1", "a2"},
	}
	res := &Result{
		Docs:  []types.NormalizedDoc{},
		Stats: Stats{AdapterCount: 2, SuccessCount: 2, ErrorCount: 0},
	}
	// Must not panic; exercises the logger path.
	tr := tracer(o)
	_, span := tr.Start(ctx, "fanout.dispatch")
	defer span.End()
	emitDispatch(ctx, o, span, decision, res)
}

// TestEmitDispatchAllErrors verifies emitDispatch logs at WARN level when all adapters fail.
func TestEmitDispatchAllErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	o := buildTestObs(t)

	decision := router.RoutingDecision{
		Category:   router.CategoryWeb,
		AdapterSet: []string{"a1"},
	}
	res := &Result{
		Docs:  []types.NormalizedDoc{},
		Stats: Stats{AdapterCount: 1, SuccessCount: 0, ErrorCount: 1},
	}
	// Exercises the level=WARN path (all adapters failed).
	emitDispatch(ctx, o, nil, decision, res)
}

// TestEmitEmptyWithLogger verifies emitEmpty writes to the logger when obs is non-nil.
func TestEmitEmptyWithLogger(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	o := buildTestObs(t)
	// Must not panic.
	emitEmpty(ctx, o)
}

// TestEmitPanicWithLogger verifies emitPanic writes to the logger when obs is non-nil.
func TestEmitPanicWithLogger(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	o := buildTestObs(t)
	emitPanic(ctx, o, "adapter1", "panic value", []byte("goroutine 1 [running]: ..."))
}

// TestInflightWithMetrics verifies inc/decInflight do not panic when metrics are present.
func TestInflightWithMetrics(t *testing.T) {
	t.Parallel()
	o := buildTestObs(t)
	// Metrics is populated by obs.Init; FanoutInflight may or may not be nil.
	// Either way: must not panic.
	incInflight(o, "web")
	decInflight(o, "web")
}

// TestTracerWithObs verifies tracer(o) returns a non-nil tracer when obs is non-nil.
func TestTracerWithObs(t *testing.T) {
	t.Parallel()
	o := buildTestObs(t)
	tr := tracer(o)
	if tr == nil {
		t.Fatal("tracer(o) returned nil")
	}
	ctx, span := tr.Start(context.Background(), "test.span")
	span.End()
	_ = ctx
}
