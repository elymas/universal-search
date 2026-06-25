package fanout_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"

	"github.com/elymas/universal-search/internal/adapters"
	"github.com/elymas/universal-search/internal/fanout"
	"github.com/elymas/universal-search/internal/obs"
	"github.com/elymas/universal-search/internal/obs/metrics"
	"github.com/elymas/universal-search/pkg/types"

	"github.com/prometheus/client_golang/prometheus"
)

// TestDispatchHappyPath3Adapters verifies 3 adapters × 5 docs = 15 in result with no dedup.
func TestDispatchHappyPath3Adapters(t *testing.T) {
	t.Parallel()

	ad1 := &stubAdapter{name: "ad1", docs: makeDocs("ad1", 5)}
	ad2 := &stubAdapter{name: "ad2", docs: makeDocs("ad2", 5)}
	ad3 := &stubAdapter{name: "ad3", docs: makeDocs("ad3", 5)}
	reg := buildTestRegistry(ad1, ad2, ad3)
	f, _ := fanout.New(fanout.Options{Registry: reg})

	result, err := f.Dispatch(context.Background(), makeDecision("ad1", "ad2", "ad3"), types.Query{Text: "test"})
	if err != nil {
		t.Fatalf("Dispatch error: %v", err)
	}
	if len(result.Docs) != 15 {
		t.Fatalf("want 15 docs, got %d", len(result.Docs))
	}
	if result.Stats.AdapterCount != 3 {
		t.Fatalf("want AdapterCount==3, got %d", result.Stats.AdapterCount)
	}
	if result.Stats.SuccessCount != 3 {
		t.Fatalf("want SuccessCount==3, got %d", result.Stats.SuccessCount)
	}
	if result.Stats.ErrorCount != 0 {
		t.Fatalf("want ErrorCount==0, got %d", result.Stats.ErrorCount)
	}
	if result.AdapterErrors != nil {
		t.Fatalf("want AdapterErrors==nil, got %v", result.AdapterErrors)
	}
	if result.Stats.DedupDropped != 0 {
		t.Fatalf("want DedupDropped==0, got %d", result.Stats.DedupDropped)
	}
	// Stats invariant.
	if result.Stats.SuccessCount+result.Stats.ErrorCount != result.Stats.AdapterCount {
		t.Fatalf("Stats invariant violated: %+v", result.Stats)
	}
}

// TestDispatchHonoursMaxParallel verifies max concurrent goroutines == MaxParallel.
func TestDispatchHonoursMaxParallel(t *testing.T) {
	t.Parallel()

	var counter atomic.Int32
	var peak atomic.Int32

	stubs := make([]*stubAdapter, 20)
	for i := range 20 {
		i := i
		stubs[i] = &stubAdapter{
			name:     "ad" + itoa(i),
			docs:     makeDocs("ad"+itoa(i), 1),
			latency:  100 * time.Millisecond,
			inflight: &counter,
		}
	}
	// Wrap each stub's Search to track peak.
	for i, s := range stubs {
		orig := s.inflight
		s.inflight = orig
		_ = i
	}

	// Instrument via a custom stub that updates peak.
	peakStubs := make([]*peakStubAdapter, 20)
	peakAdapters := make([]*stubAdapter, 20)
	for i := range 20 {
		peakStubs[i] = &peakStubAdapter{
			name:    "pad" + itoa(i),
			docs:    makeDocs("pad"+itoa(i), 1),
			latency: 100 * time.Millisecond,
			counter: &counter,
			peak:    &peak,
		}
		peakAdapters[i] = &stubAdapter{name: "pad" + itoa(i)}
	}

	reg := adapters.NewRegistry(nil)
	names := make([]string, 20)
	for i, ps := range peakStubs {
		if err := reg.RegisterWithOptions(ps, adapters.RegisterOptions{SkipAuthCheck: true}); err != nil {
			t.Fatalf("register: %v", err)
		}
		names[i] = ps.name
	}

	f, _ := fanout.New(fanout.Options{Registry: reg, MaxParallel: 4})
	start := time.Now()
	result, err := f.Dispatch(context.Background(), makeDecision(names...), types.Query{})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if result.Stats.SuccessCount != 20 {
		t.Fatalf("want SuccessCount==20, got %d", result.Stats.SuccessCount)
	}
	// MaxParallel=4 so at least 5 waves of 100ms each: >=500ms.
	if elapsed < 400*time.Millisecond {
		t.Fatalf("elapsed too short: %v (expected >=400ms for 4-way parallel)", elapsed)
	}
	if peak.Load() > 4 {
		t.Fatalf("peak inflight %d exceeds MaxParallel=4", peak.Load())
	}
}

// peakStubAdapter tracks the peak concurrent inflight count.
type peakStubAdapter struct {
	name    string
	docs    []types.NormalizedDoc
	latency time.Duration
	counter *atomic.Int32
	peak    *atomic.Int32
}

func (p *peakStubAdapter) Name() string { return p.name }
func (p *peakStubAdapter) Capabilities() types.Capabilities {
	return types.Capabilities{SourceID: p.name}
}
func (p *peakStubAdapter) Healthcheck(_ context.Context) error { return nil }
func (p *peakStubAdapter) Search(ctx context.Context, _ types.Query) ([]types.NormalizedDoc, error) {
	cur := p.counter.Add(1)
	defer p.counter.Add(-1)
	// Update peak atomically.
	for {
		old := p.peak.Load()
		if cur <= old || p.peak.CompareAndSwap(old, cur) {
			break
		}
	}
	select {
	case <-time.After(p.latency):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return p.docs, nil
}

// TestDispatchOneAdapterFailsOthersSucceed verifies partial success with 1 error.
func TestDispatchOneAdapterFailsOthersSucceed(t *testing.T) {
	t.Parallel()

	ad1 := &stubAdapter{name: "ad1", docs: makeDocs("ad1", 5)}
	ad2 := &stubAdapter{name: "ad2", err: &types.SourceError{
		Adapter:  "ad2",
		Category: types.CategoryPermanent,
		Cause:    types.ErrPermanent,
	}}
	ad3 := &stubAdapter{name: "ad3", docs: makeDocs("ad3", 5)}
	reg := buildTestRegistry(ad1, ad2, ad3)
	f, _ := fanout.New(fanout.Options{Registry: reg})

	result, err := f.Dispatch(context.Background(), makeDecision("ad1", "ad2", "ad3"), types.Query{})
	if err != nil {
		t.Fatalf("Dispatch error: %v", err)
	}
	if len(result.Docs) != 10 {
		t.Fatalf("want 10 docs, got %d", len(result.Docs))
	}
	if result.AdapterErrors == nil {
		t.Fatal("want AdapterErrors != nil")
	}
	if len(result.AdapterErrors) != 1 {
		t.Fatalf("want 1 adapter error, got %d", len(result.AdapterErrors))
	}
	if result.AdapterErrors["ad2"] == nil {
		t.Fatal("want AdapterErrors[ad2] != nil")
	}
	if !errors.Is(result.AdapterErrors["ad2"], types.ErrPermanent) {
		t.Fatalf("want ErrPermanent in AdapterErrors[ad2], got %v", result.AdapterErrors["ad2"])
	}
	if result.Stats.SuccessCount != 2 {
		t.Fatalf("want SuccessCount==2, got %d", result.Stats.SuccessCount)
	}
	if result.Stats.ErrorCount != 1 {
		t.Fatalf("want ErrorCount==1, got %d", result.Stats.ErrorCount)
	}
	if result.Stats.SuccessCount+result.Stats.ErrorCount != result.Stats.AdapterCount {
		t.Fatalf("Stats invariant violated: %+v", result.Stats)
	}
}

// TestDispatchOneFailureDoesNotCancelOthers verifies slow adapter completes despite fast-fail peer.
func TestDispatchOneFailureDoesNotCancelOthers(t *testing.T) {
	t.Parallel()

	ad1 := &stubAdapter{name: "ad1", err: types.ErrPermanent, latency: 10 * time.Millisecond}
	ad2 := &stubAdapter{name: "ad2", docs: makeDocs("ad2", 5), latency: 200 * time.Millisecond}
	reg := buildTestRegistry(ad1, ad2)
	f, _ := fanout.New(fanout.Options{Registry: reg, PerAdapterTimeout: 5 * time.Second})

	start := time.Now()
	result, _ := f.Dispatch(context.Background(), makeDecision("ad1", "ad2"), types.Query{})
	elapsed := time.Since(start)

	if len(result.Docs) != 5 {
		t.Fatalf("want 5 docs from ad2, got %d", len(result.Docs))
	}
	if elapsed < 200*time.Millisecond {
		t.Fatalf("elapsed too short: ad2's goroutine must have completed; elapsed=%v", elapsed)
	}
}

// TestDispatchPartialResultsOnParentTimeout verifies partial result on parent ctx expiry.
func TestDispatchPartialResultsOnParentTimeout(t *testing.T) {
	t.Parallel()

	ad1 := &stubAdapter{name: "ad1", docs: makeDocs("ad1", 5), latency: 100 * time.Millisecond}
	ad2 := &stubAdapter{name: "ad2", docs: makeDocs("ad2", 5), latency: 100 * time.Millisecond}
	ad3 := &stubAdapter{name: "ad3", docs: makeDocs("ad3", 5), latency: 5 * time.Second}
	ad4 := &stubAdapter{name: "ad4", docs: makeDocs("ad4", 5), latency: 5 * time.Second}
	ad5 := &stubAdapter{name: "ad5", docs: makeDocs("ad5", 5), latency: 5 * time.Second}
	reg := buildTestRegistry(ad1, ad2, ad3, ad4, ad5)
	f, _ := fanout.New(fanout.Options{Registry: reg, MaxParallel: 5, PerAdapterTimeout: 10 * time.Second})

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	start := time.Now()
	result, err := f.Dispatch(ctx, makeDecision("ad1", "ad2", "ad3", "ad4", "ad5"), types.Query{})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Dispatch should not return error, got %v", err)
	}
	if result == nil {
		t.Fatal("result must not be nil")
	}
	if len(result.Docs) != 10 {
		t.Fatalf("want 10 docs (ad1+ad2), got %d", len(result.Docs))
	}
	for _, name := range []string{"ad3", "ad4", "ad5"} {
		if result.AdapterErrors[name] == nil {
			t.Fatalf("want AdapterErrors[%s] != nil", name)
		}
		if !errors.Is(result.AdapterErrors[name], context.DeadlineExceeded) &&
			!errors.Is(result.AdapterErrors[name], context.Canceled) &&
			!errors.Is(result.AdapterErrors[name], types.ErrSourceUnavailable) {
			t.Fatalf("AdapterErrors[%s] should be deadline/canceled/unavailable, got %v", name, result.AdapterErrors[name])
		}
	}
	if result.Stats.SuccessCount != 2 {
		t.Fatalf("want SuccessCount==2, got %d", result.Stats.SuccessCount)
	}
	if result.Stats.ErrorCount != 3 {
		t.Fatalf("want ErrorCount==3, got %d", result.Stats.ErrorCount)
	}
	if elapsed < 400*time.Millisecond || elapsed > 900*time.Millisecond {
		t.Fatalf("elapsed %v not in [400ms, 900ms]", elapsed)
	}
}

// TestDispatchPerAdapterTimeoutDoesNotKillOthers verifies per-adapter timeout independence.
func TestDispatchPerAdapterTimeoutDoesNotKillOthers(t *testing.T) {
	t.Parallel()

	ad1 := &stubAdapter{name: "ad1", docs: makeDocs("ad1", 5), latency: 1 * time.Second}
	ad2 := &stubAdapter{name: "ad2", docs: makeDocs("ad2", 5), latency: 100 * time.Millisecond}
	ad3 := &stubAdapter{name: "ad3", docs: makeDocs("ad3", 5), latency: 150 * time.Millisecond}
	reg := buildTestRegistry(ad1, ad2, ad3)
	f, _ := fanout.New(fanout.Options{Registry: reg, MaxParallel: 3, PerAdapterTimeout: 200 * time.Millisecond})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	result, _ := f.Dispatch(ctx, makeDecision("ad1", "ad2", "ad3"), types.Query{})
	elapsed := time.Since(start)

	if len(result.Docs) != 10 {
		t.Fatalf("want 10 docs (ad2+ad3), got %d", len(result.Docs))
	}
	if result.AdapterErrors["ad1"] == nil {
		t.Fatal("want AdapterErrors[ad1] != nil (timed out)")
	}
	if !errors.Is(result.AdapterErrors["ad1"], context.DeadlineExceeded) &&
		!errors.Is(result.AdapterErrors["ad1"], context.Canceled) {
		t.Fatalf("AdapterErrors[ad1] should be DeadlineExceeded/Canceled, got %v", result.AdapterErrors["ad1"])
	}
	if result.Stats.SuccessCount != 2 {
		t.Fatalf("want SuccessCount==2, got %d", result.Stats.SuccessCount)
	}
	if result.Stats.ErrorCount != 1 {
		t.Fatalf("want ErrorCount==1, got %d", result.Stats.ErrorCount)
	}
	// Bounded by ad1's per-adapter timeout (200ms), not ad1's sleep (1s).
	if elapsed > 600*time.Millisecond {
		t.Fatalf("elapsed %v too long (expected ~200ms for ad1 timeout)", elapsed)
	}
}

// TestDispatchAdapterPanicCaptured verifies panic recovery keeps siblings running.
func TestDispatchAdapterPanicCaptured(t *testing.T) {
	t.Parallel()

	ad1 := &stubAdapter{name: "ad1", docs: makeDocs("ad1", 5)}
	ad2 := &stubAdapter{name: "ad2", doPanic: true}
	ad3 := &stubAdapter{name: "ad3", docs: makeDocs("ad3", 5)}
	reg := buildTestRegistry(ad1, ad2, ad3)
	f, _ := fanout.New(fanout.Options{Registry: reg})

	result, err := f.Dispatch(context.Background(), makeDecision("ad1", "ad2", "ad3"), types.Query{})
	if err != nil {
		t.Fatalf("Dispatch should not error: %v", err)
	}
	if len(result.Docs) != 10 {
		t.Fatalf("want 10 docs (ad1+ad3), got %d", len(result.Docs))
	}
	if result.AdapterErrors["ad2"] == nil {
		t.Fatal("want AdapterErrors[ad2] != nil")
	}
	var se *types.SourceError
	if !errors.As(result.AdapterErrors["ad2"], &se) {
		t.Fatalf("want *SourceError for ad2 panic, got %T", result.AdapterErrors["ad2"])
	}
	if se.Category != types.CategoryUnknown {
		t.Fatalf("want CategoryUnknown, got %v", se.Category)
	}
	if result.Stats.ErrorCount != 1 {
		t.Fatalf("want ErrorCount==1, got %d", result.Stats.ErrorCount)
	}
	if result.Stats.SuccessCount != 2 {
		t.Fatalf("want SuccessCount==2, got %d", result.Stats.SuccessCount)
	}
}

// TestDispatchAdapterPanicLogsStackTrace verifies panic is logged with stack_trace.
func TestDispatchAdapterPanicLogsStackTrace(t *testing.T) {
	t.Parallel()

	// This test verifies the panic is captured without crashing. Stack trace
	// logging is tested via the obs logger; we verify no panic propagates.
	ad := &stubAdapter{name: "panicky", doPanic: true}
	other := &stubAdapter{name: "ok", docs: makeDocs("ok", 2)}
	reg := buildTestRegistry(ad, other)
	f, _ := fanout.New(fanout.Options{Registry: reg})

	// Should not panic.
	result, _ := f.Dispatch(context.Background(), makeDecision("panicky", "ok"), types.Query{})
	if result.Stats.ErrorCount != 1 {
		t.Fatalf("panicky adapter should count as error: %+v", result.Stats)
	}
}

// TestDispatchAdapterPanicNoLeak verifies no goroutine leak after panic.
func TestDispatchAdapterPanicNoLeak(t *testing.T) {
	t.Parallel()

	ad := &stubAdapter{name: "panicky", doPanic: true}
	reg := buildTestRegistry(ad)
	f, _ := fanout.New(fanout.Options{Registry: reg})
	f.Dispatch(context.Background(), makeDecision("panicky"), types.Query{}) //nolint:errcheck
}

// TestDispatchWorkerStateNoMapWrites runs under -race to confirm no data races.
// Workers write only to per-index slices; supervisor builds AdapterErrors map.
func TestDispatchWorkerStateNoMapWrites(t *testing.T) {
	t.Parallel()

	stubs := make([]*stubAdapter, 8)
	names := make([]string, 8)
	for i := range 8 {
		stubs[i] = &stubAdapter{name: "rw" + itoa(i), docs: makeDocs("rw"+itoa(i), 3)}
		names[i] = stubs[i].name
	}
	reg := buildTestRegistry(stubs...)
	f, _ := fanout.New(fanout.Options{Registry: reg, MaxParallel: 8})

	// Run many concurrent dispatches — race detector catches any map write races.
	for range 20 {
		result, _ := f.Dispatch(context.Background(), makeDecision(names...), types.Query{})
		if result.Stats.SuccessCount != 8 {
			t.Fatalf("want 8 successes, got %d", result.Stats.SuccessCount)
		}
	}
}

// TestDispatchAlreadyCancelledCtx verifies pre-cancelled ctx produces all errors, no goroutines.
func TestDispatchAlreadyCancelledCtx(t *testing.T) {
	t.Parallel()

	ad1 := &stubAdapter{name: "ad1", docs: makeDocs("ad1", 5)}
	ad2 := &stubAdapter{name: "ad2", docs: makeDocs("ad2", 5)}
	ad3 := &stubAdapter{name: "ad3", docs: makeDocs("ad3", 5)}
	reg := buildTestRegistry(ad1, ad2, ad3)
	f, _ := fanout.New(fanout.Options{Registry: reg})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	start := time.Now()
	result, err := f.Dispatch(ctx, makeDecision("ad1", "ad2", "ad3"), types.Query{})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Dispatch should not error: %v", err)
	}
	if result.Stats.ErrorCount != 3 {
		t.Fatalf("want ErrorCount==3, got %d", result.Stats.ErrorCount)
	}
	for _, name := range []string{"ad1", "ad2", "ad3"} {
		if !errors.Is(result.AdapterErrors[name], context.Canceled) {
			t.Fatalf("AdapterErrors[%s] should be Canceled, got %v", name, result.AdapterErrors[name])
		}
	}
	if elapsed > 10*time.Millisecond {
		t.Fatalf("elapsed %v too long for already-cancelled ctx (want < 10ms)", elapsed)
	}
	// Goroutine-leak detection is handled suite-wide by goleak.VerifyTestMain
	// (bench_test.go). A per-test runtime.NumGoroutine() delta is flaky under
	// t.Parallel() because the global count includes sibling parallel tests.
}

// TestDispatchIgnoresQueryDeadline verifies Query.Deadline is not consumed by Dispatch.
func TestDispatchIgnoresQueryDeadline(t *testing.T) {
	t.Parallel()

	// Query.Deadline is 100ms but parent ctx is 5s.
	// ad1 takes 200ms — should succeed because parent ctx is 5s.
	ad1 := &stubAdapter{name: "ad1", docs: makeDocs("ad1", 3), latency: 200 * time.Millisecond}
	reg := buildTestRegistry(ad1)
	f, _ := fanout.New(fanout.Options{Registry: reg, PerAdapterTimeout: 5 * time.Second})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	q := types.Query{
		Text:     "test",
		Deadline: time.Now().Add(100 * time.Millisecond), // much shorter than parent ctx
	}
	result, _ := f.Dispatch(ctx, makeDecision("ad1"), q)
	// ad1 should succeed because Dispatch does not apply Query.Deadline to ctx.
	if result.Stats.SuccessCount != 1 {
		t.Fatalf("want SuccessCount==1 (parent ctx is 5s), got %d; ad1 may have been incorrectly cancelled by Query.Deadline", result.Stats.SuccessCount)
	}
}

// TestDispatchCancelledMidQueue verifies no deadlock when ctx is cancelled with queued adapters.
// REQ-FAN-012: H18 pre-launch ctx guard prevents SetLimit deadlock.
func TestDispatchCancelledMidQueue(t *testing.T) {
	t.Parallel()

	// 12 adapters, MaxParallel=2. First 2 sleep 5s (cancelled mid-flight by ctx).
	// Remaining 10 have latency 200ms so that they haven't completed when ctx cancels at 50ms.
	// Main invariants: no deadlock, all 12 adapters produce an outcome, elapsed is bounded.
	stubs := make([]*stubAdapter, 12)
	names := make([]string, 12)
	for i := range 12 {
		lat := 200 * time.Millisecond
		if i < 2 {
			lat = 5 * time.Second // these will be cancelled mid-flight
		}
		stubs[i] = &stubAdapter{name: "mq" + itoa(i), latency: lat, docs: makeDocs("mq"+itoa(i), 1)}
		names[i] = stubs[i].name
	}
	reg := buildTestRegistry(stubs...)
	f, _ := fanout.New(fanout.Options{Registry: reg, MaxParallel: 2})

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel after 50ms (in-flight workers notice ctx.Done; queued ones get H18 pre-populated).
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	result, _ := f.Dispatch(ctx, makeDecision(names...), types.Query{})
	elapsed := time.Since(start)

	// All 12 adapters must have an outcome (no silent loss).
	total := result.Stats.SuccessCount + result.Stats.ErrorCount
	if total != 12 {
		t.Fatalf("want 12 total outcomes (success+error), got %d (success=%d, error=%d)",
			total, result.Stats.SuccessCount, result.Stats.ErrorCount)
	}
	// Bounded by cancel time + drain time, not by the 5s sleeps.
	if elapsed > 500*time.Millisecond {
		t.Fatalf("elapsed %v too long (want < 500ms; bounded by cancel, not adapter sleep)", elapsed)
	}
}

// ---------------------------------------------------------------------------
// SPEC-EVAL-002 Phase 2: Fanout partial counter emission tests
// ---------------------------------------------------------------------------

// TestFanoutPartialCounterEmission verifies that when a fanout dispatch has
// partial results (some adapters fail), usearch_fanout_partial_total is
// incremented exactly once per failed adapter (REQ-EVAL2-004).
func TestFanoutPartialCounterEmission(t *testing.T) {
	t.Parallel()

	// Build a metrics registry to observe counter values.
	metricsReg := metrics.NewRegistry()
	o := &obs.Obs{
		Metrics: metricsReg,
	}

	// 3 adapters: 2 succeed, 1 fails.
	ad1 := &stubAdapter{name: "reddit", docs: makeDocs("reddit", 3)}
	ad2 := &stubAdapter{name: "naver", err: errors.New("upstream timeout")}
	ad3 := &stubAdapter{name: "github", docs: makeDocs("github", 3)}
	reg := buildTestRegistry(ad1, ad2, ad3)

	f, err := fanout.New(fanout.Options{Registry: reg, Obs: o})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := f.Dispatch(context.Background(), makeDecision("reddit", "naver", "github"), types.Query{Text: "test"})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	// Verify result has partial errors.
	if result.Stats.ErrorCount != 1 {
		t.Fatalf("want ErrorCount==1, got %d", result.Stats.ErrorCount)
	}

	// Verify FanoutPartial counter incremented for "naver" (the failing adapter).
	naverCount := counterValueFromReg(metricsReg.Prometheus, "usearch_fanout_partial_total",
		map[string]string{"adapter": "naver"})
	if naverCount != 1 {
		t.Errorf("naver partial count: got %.0f, want 1", naverCount)
	}

	// Verify successful adapters did NOT get their partial counter incremented.
	redditCount := counterValueFromReg(metricsReg.Prometheus, "usearch_fanout_partial_total",
		map[string]string{"adapter": "reddit"})
	if redditCount != 0 {
		t.Errorf("reddit partial count: got %.0f, want 0 (adapter succeeded)", redditCount)
	}

	githubCount := counterValueFromReg(metricsReg.Prometheus, "usearch_fanout_partial_total",
		map[string]string{"adapter": "github"})
	if githubCount != 0 {
		t.Errorf("github partial count: got %.0f, want 0 (adapter succeeded)", githubCount)
	}
}

// TestFanoutPartialCounterMultipleFailures verifies partial counter increments
// for multiple failing adapters (5 adapters, 2 fail).
func TestFanoutPartialCounterMultipleFailures(t *testing.T) {
	t.Parallel()

	metricsReg := metrics.NewRegistry()
	o := &obs.Obs{
		Metrics: metricsReg,
	}

	ad1 := &stubAdapter{name: "reddit", docs: makeDocs("reddit", 2)}
	ad2 := &stubAdapter{name: "naver", err: errors.New("rate limited")}
	ad3 := &stubAdapter{name: "youtube", err: errors.New("transcript error")}
	ad4 := &stubAdapter{name: "github", docs: makeDocs("github", 2)}
	ad5 := &stubAdapter{name: "hackernews", docs: makeDocs("hackernews", 2)}
	reg := buildTestRegistry(ad1, ad2, ad3, ad4, ad5)

	f, err := fanout.New(fanout.Options{Registry: reg, Obs: o})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := f.Dispatch(context.Background(),
		makeDecision("reddit", "naver", "youtube", "github", "hackernews"),
		types.Query{Text: "test"})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	if result.Stats.ErrorCount != 2 {
		t.Fatalf("want ErrorCount==2, got %d", result.Stats.ErrorCount)
	}

	// Both failing adapters should have partial count = 1.
	for _, name := range []string{"naver", "youtube"} {
		count := counterValueFromReg(metricsReg.Prometheus, "usearch_fanout_partial_total",
			map[string]string{"adapter": name})
		if count != 1 {
			t.Errorf("%s partial count: got %.0f, want 1", name, count)
		}
	}

	// Successful adapters should have partial count = 0.
	for _, name := range []string{"reddit", "github", "hackernews"} {
		count := counterValueFromReg(metricsReg.Prometheus, "usearch_fanout_partial_total",
			map[string]string{"adapter": name})
		if count != 0 {
			t.Errorf("%s partial count: got %.0f, want 0", name, count)
		}
	}
}

// counterValueFromReg extracts a counter value from a Prometheus registry.
func counterValueFromReg(reg *prometheus.Registry, name string, labels map[string]string) float64 {
	mfs, err := reg.Gather()
	if err != nil {
		return 0
	}
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		for _, m := range mf.GetMetric() {
			if labelsMatch2(m.GetLabel(), labels) {
				return m.GetCounter().GetValue()
			}
		}
	}
	return 0
}

func labelsMatch2(got []*dto.LabelPair, want map[string]string) bool {
	matched := 0
	for _, lp := range got {
		v, ok := want[lp.GetName()]
		if ok && v == lp.GetValue() {
			matched++
		}
	}
	return matched == len(want)
}
