package fanout_test

import (
	"context"
	"errors"
	"io"
	"testing"

	dto "github.com/prometheus/client_model/go"

	"github.com/elymas/universal-search/internal/adapters"
	"github.com/elymas/universal-search/internal/fanout"
	"github.com/elymas/universal-search/internal/obs"
	"github.com/elymas/universal-search/internal/obs/metrics"
	"github.com/elymas/universal-search/pkg/types"
)

// newTestObs builds a fully-initialised obs bundle (logger + metrics + tracer
// provider) so fanout's tracer() call does not dereference a nil provider.
func newTestObs(t *testing.T) *obs.Obs {
	t.Helper()
	o, shutdown, err := obs.Init(context.Background(), obs.Config{
		ServiceName: "fanout-test",
		LogLevel:    "ERROR",
		LogWriter:   io.Discard,
	})
	if err != nil {
		t.Fatalf("obs.Init: %v", err)
	}
	t.Cleanup(func() { _ = shutdown(context.Background()) })
	return o
}

// buildTestRegistryWithObs builds an adapter registry whose wrappedAdapters and
// fanout share the supplied obs bundle so metric emission is observable.
func buildTestRegistryWithObs(o *obs.Obs, stubs ...*stubAdapter) *adapters.Registry {
	reg := adapters.NewRegistry(o)
	for _, s := range stubs {
		if err := reg.RegisterWithOptions(s, adapters.RegisterOptions{SkipAuthCheck: true}); err != nil {
			panic("buildTestRegistryWithObs: " + err.Error())
		}
	}
	return reg
}

// partialCounterValue reads usearch_fanout_partial_total{adapter} from the
// shared registry.
func partialCounterValue(t *testing.T, reg *metrics.Registry, adapter string) float64 {
	t.Helper()
	m := &dto.Metric{}
	if err := reg.FanoutPartial.WithLabelValues(adapter).Write(m); err != nil {
		t.Fatalf("write counter: %v", err)
	}
	return m.GetCounter().GetValue()
}

// TestFanoutPartialCounterEmission verifies REQ-EVAL2-004 / AC-002: with 3
// adapters where 1 fails, the failing adapter's partial counter increments by
// exactly 1 and the passing adapters' counters are unchanged.
func TestFanoutPartialCounterEmission(t *testing.T) {
	t.Parallel()

	o := newTestObs(t)

	failing := &stubAdapter{name: "failing", err: errors.New("boom")}
	ok1 := &stubAdapter{name: "ok1", docs: makeDocs("ok1", 2)}
	ok2 := &stubAdapter{name: "ok2", docs: makeDocs("ok2", 2)}
	reg := buildTestRegistryWithObs(o, failing, ok1, ok2)

	f, err := fanout.New(fanout.Options{Registry: reg, Obs: o})
	if err != nil {
		t.Fatalf("fanout.New: %v", err)
	}

	beforeFailing := partialCounterValue(t, o.Metrics, "failing")
	beforeOK1 := partialCounterValue(t, o.Metrics, "ok1")
	beforeOK2 := partialCounterValue(t, o.Metrics, "ok2")

	result, err := f.Dispatch(context.Background(), makeDecision("failing", "ok1", "ok2"), types.Query{Text: "q"})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if result.Stats.ErrorCount != 1 {
		t.Fatalf("want ErrorCount==1, got %d", result.Stats.ErrorCount)
	}

	if got := partialCounterValue(t, o.Metrics, "failing"); got != beforeFailing+1 {
		t.Errorf("failing partial counter: got %v, want %v", got, beforeFailing+1)
	}
	if got := partialCounterValue(t, o.Metrics, "ok1"); got != beforeOK1 {
		t.Errorf("ok1 partial counter changed: got %v, want %v", got, beforeOK1)
	}
	if got := partialCounterValue(t, o.Metrics, "ok2"); got != beforeOK2 {
		t.Errorf("ok2 partial counter changed: got %v, want %v", got, beforeOK2)
	}
}

// TestFanoutPartialCounterFullSuccessNoIncrement verifies that a dispatch with
// no adapter errors leaves all partial counters untouched (AdapterErrors nil).
func TestFanoutPartialCounterFullSuccessNoIncrement(t *testing.T) {
	t.Parallel()

	o := newTestObs(t)
	a1 := &stubAdapter{name: "s1", docs: makeDocs("s1", 1)}
	a2 := &stubAdapter{name: "s2", docs: makeDocs("s2", 1)}
	reg := buildTestRegistryWithObs(o, a1, a2)

	f, _ := fanout.New(fanout.Options{Registry: reg, Obs: o})

	before1 := partialCounterValue(t, o.Metrics, "s1")
	before2 := partialCounterValue(t, o.Metrics, "s2")

	result, err := f.Dispatch(context.Background(), makeDecision("s1", "s2"), types.Query{})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if result.AdapterErrors != nil {
		t.Fatalf("want AdapterErrors==nil on full success, got %v", result.AdapterErrors)
	}

	if got := partialCounterValue(t, o.Metrics, "s1"); got != before1 {
		t.Errorf("s1 partial counter changed on full success: got %v, want %v", got, before1)
	}
	if got := partialCounterValue(t, o.Metrics, "s2"); got != before2 {
		t.Errorf("s2 partial counter changed on full success: got %v, want %v", got, before2)
	}
}
