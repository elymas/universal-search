// Package index — unit tests for observability helpers (REQ-IDX-011).
package index

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/obs"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// fullObs builds a real Obs bundle (metrics + logger wired) for exercising the
// non-nil emission paths in emitSearch / emitUpsert.
func fullObs(t *testing.T) *obs.Obs {
	t.Helper()
	o, shutdown, err := obs.Init(context.Background(), obs.Config{
		ServiceName:    "index-obs-test",
		ServiceVersion: "test",
		LogLevel:       "INFO",
	})
	if err != nil {
		t.Fatalf("obs.Init: %v", err)
	}
	t.Cleanup(func() { _ = shutdown(context.Background()) })
	return o
}

// TestEmitSearch_FullPath drives the wired metrics + logger + span path with
// successful and failing per-store outcomes.
func TestEmitSearch_FullPath(t *testing.T) {
	o := fullObs(t)
	tp := sdktrace.NewTracerProvider()
	defer func() { _ = tp.Shutdown(context.Background()) }()
	_, span := tp.Tracer("test").Start(context.Background(), "search")
	defer span.End()

	result := &IndexResult{
		PerStoreErrors: map[string]error{"qdrant": nil, "meili": errors.New("boom")},
		Stats: SearchStats{
			PerStoreCounts: map[string]int{"qdrant": 3, "meili": 0},
			StoreLatencies: map[string]time.Duration{"qdrant": 12 * time.Millisecond},
			FusionLatency:  4 * time.Millisecond,
			FusedCount:     3,
			ElapsedSeconds: 0.02,
		},
	}
	perStoreErrs := map[string]error{"qdrant": nil, "meili": errors.New("boom")}
	// Must execute span attrs, prometheus inc/observe, and the WARN log branch
	// (errCount > 0) without panicking.
	emitSearch(o, span, result, perStoreErrs, 20*time.Millisecond)
}

// TestEmitUpsert_FullPath drives the wired metrics + logger + span path.
func TestEmitUpsert_FullPath(t *testing.T) {
	o := fullObs(t)
	tp := sdktrace.NewTracerProvider()
	defer func() { _ = tp.Shutdown(context.Background()) }()
	_, span := tp.Tracer("test").Start(context.Background(), "upsert")
	defer span.End()

	result := &UpsertResult{
		PerStoreErrors: map[string]error{"pg": nil, "qdrant": errors.New("write failed")},
		Stats: UpsertStats{
			DocCount:          5,
			SkippedCount:      1,
			SuccessCount:      4,
			PerStoreLatencies: map[string]time.Duration{"pg": 6 * time.Millisecond, "qdrant": 8 * time.Millisecond},
		},
	}
	emitUpsert(o, span, result, 14*time.Millisecond)
}

// TestEmitSearch_NilObs verifies nil-safety (no panic when obs is nil).
func TestEmitSearch_NilObs(t *testing.T) {
	t.Parallel()
	result := &IndexResult{
		PerStoreErrors: map[string]error{},
		Stats: SearchStats{
			PerStoreCounts: map[string]int{"qdrant": 2},
			StoreLatencies: map[string]time.Duration{"qdrant": 10 * time.Millisecond},
			FusedCount:     2,
			ElapsedSeconds: 0.01,
		},
	}
	// Must not panic.
	emitSearch(nil, nil, result, nil, 10*time.Millisecond)
}

func TestEmitSearch_NilResult_NoPanic(t *testing.T) {
	t.Parallel()
	// Nil result should be a no-op.
	emitSearch(nil, nil, nil, nil, 0)
}

func TestEmitUpsert_NilObs(t *testing.T) {
	t.Parallel()
	result := &UpsertResult{
		PerStoreErrors: map[string]error{},
		Stats: UpsertStats{
			DocCount:          3,
			SuccessCount:      3,
			PerStoreLatencies: map[string]time.Duration{"pg": 5 * time.Millisecond},
		},
	}
	// Must not panic.
	emitUpsert(nil, nil, result, 5*time.Millisecond)
}

func TestEmitUpsert_NilResult_NoPanic(t *testing.T) {
	t.Parallel()
	emitUpsert(nil, nil, nil, 0)
}

func TestEmitSearch_WithErrors(t *testing.T) {
	t.Parallel()
	perStoreErrs := map[string]error{
		"qdrant": nil,
		"meili":  nil,
		"pg":     nil,
	}
	result := &IndexResult{
		PerStoreErrors: perStoreErrs,
		Stats:          SearchStats{FusedCount: 0},
	}
	// Still must not panic.
	emitSearch(nil, nil, result, perStoreErrs, 0)
}

func TestEmitSearch_WithStoreCounts(t *testing.T) {
	t.Parallel()
	result := &IndexResult{
		PerStoreErrors: map[string]error{},
		Stats: SearchStats{
			PerStoreCounts: map[string]int{
				"qdrant": 3,
				"meili":  5,
				"pg":     2,
			},
			StoreLatencies: map[string]time.Duration{
				"qdrant": 10 * time.Millisecond,
				"meili":  20 * time.Millisecond,
				"pg":     5 * time.Millisecond,
			},
			FusionLatency: 1 * time.Millisecond,
			FusedCount:    5,
		},
	}
	emitSearch(nil, nil, result, map[string]error{}, 30*time.Millisecond)
}

func TestEmitUpsert_WithLatencies(t *testing.T) {
	t.Parallel()
	result := &UpsertResult{
		PerStoreErrors: map[string]error{},
		Stats: UpsertStats{
			DocCount:     5,
			SuccessCount: 5,
			PerStoreLatencies: map[string]time.Duration{
				"qdrant": 10 * time.Millisecond,
				"meili":  20 * time.Millisecond,
				"pg":     5 * time.Millisecond,
			},
			ElapsedSeconds: 0.035,
		},
	}
	emitUpsert(nil, nil, result, 35*time.Millisecond)
}

func TestEmitUpsert_WithValidationError(t *testing.T) {
	t.Parallel()
	result := &UpsertResult{
		PerStoreErrors: map[string]error{
			"validation": errors.New("1 validation error(s)"),
		},
		Stats: UpsertStats{
			DocCount:          3,
			SkippedCount:      1,
			SuccessCount:      2,
			PerStoreLatencies: map[string]time.Duration{},
		},
	}
	// Validation errors should not count as store errors.
	emitUpsert(nil, nil, result, 5*time.Millisecond)
}
