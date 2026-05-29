// Package adapters_test — Registry and wrappedAdapter tests for SPEC-CORE-001
// REQ-CORE-003/004/005/006 + NFR-CORE-002.
package adapters_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/elymas/universal-search/internal/adapters"
	"github.com/elymas/universal-search/internal/obs"
	"github.com/elymas/universal-search/pkg/types"
)

// fakeAdapter is a programmable test adapter. Each test constructs its own
// fake to exercise specific Search outcomes without coupling to noop's
// behavior.
type fakeAdapter struct {
	name        string
	caps        types.Capabilities
	searchFn    func(ctx context.Context, q types.Query) ([]types.NormalizedDoc, error)
	calls       atomic.Int64
	healthErr   error
	healthCalls atomic.Int64
}

func (f *fakeAdapter) Name() string { return f.name }
func (f *fakeAdapter) Healthcheck(_ context.Context) error {
	f.healthCalls.Add(1)
	return f.healthErr
}
func (f *fakeAdapter) Capabilities() types.Capabilities { return f.caps }
func (f *fakeAdapter) Search(ctx context.Context, q types.Query) ([]types.NormalizedDoc, error) {
	f.calls.Add(1)
	if f.searchFn != nil {
		return f.searchFn(ctx, q)
	}
	return nil, nil
}

// newFake constructs a fakeAdapter with default Capabilities.
func newFake(name string) *fakeAdapter {
	return &fakeAdapter{
		name: name,
		caps: types.Capabilities{
			SourceID:          name,
			DisplayName:       name,
			DocTypes:          []types.DocType{types.DocTypeOther},
			SupportedLangs:    []string{"en"},
			DefaultMaxResults: 10,
		},
	}
}

// initObs constructs a fresh Obs bundle for the test. Per-Registry isolation
// is required (see internal/obs/metrics/metrics.go:46-50 comment).
func initObs(t *testing.T, w io.Writer) *obs.Obs {
	t.Helper()
	o, shutdown, err := obs.Init(context.Background(), obs.Config{
		ServiceName: "test",
		LogLevel:    "DEBUG",
		LogWriter:   w,
	})
	if err != nil {
		t.Fatalf("obs.Init: %v", err)
	}
	t.Cleanup(func() { _ = shutdown(context.Background()) })
	return o
}

// REQ-CORE-003: Register succeeds for new adapter; Get returns wrapped instance.
func TestRegisterSucceedsForNewAdapter(t *testing.T) {
	t.Parallel()
	o := initObs(t, io.Discard)
	r := adapters.NewRegistry(o)

	a := newFake("alpha")
	if err := r.Register(a); err != nil {
		t.Fatalf("Register: %v", err)
	}
	got, ok := r.Get("alpha")
	if !ok {
		t.Fatal("Get(alpha) = !ok, want ok")
	}
	if got == nil {
		t.Fatal("Get(alpha) returned nil")
	}
	if got.Name() != "alpha" {
		t.Errorf("Name() = %q, want alpha", got.Name())
	}
	// Calling Search on the returned (wrapped) adapter must reach the underlying
	// fake — this proves the wrapper delegates.
	_, _ = got.Search(context.Background(), types.Query{})
	if got, want := a.calls.Load(), int64(1); got != want {
		t.Errorf("inner Search calls = %d, want %d", got, want)
	}
}

// REQ-CORE-003: Duplicate name returns *RegistryError wrapping ErrDuplicateAdapter.
func TestRegisterRejectsDuplicateName(t *testing.T) {
	t.Parallel()
	o := initObs(t, io.Discard)
	r := adapters.NewRegistry(o)

	a1 := newFake("dup")
	if err := r.Register(a1); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	a2 := newFake("dup")
	err := r.Register(a2)
	if err == nil {
		t.Fatal("second Register: got nil error, want *RegistryError")
	}
	if !errors.Is(err, adapters.ErrDuplicateAdapter) {
		t.Errorf("errors.Is(err, ErrDuplicateAdapter) = false, want true; err = %v", err)
	}
	var regErr *adapters.RegistryError
	if !errors.As(err, &regErr) {
		t.Fatalf("errors.As(*RegistryError) = false; err = %v", err)
	}
	if regErr.Op != "register" {
		t.Errorf("RegistryError.Op = %q, want %q", regErr.Op, "register")
	}
	if regErr.Name != "dup" {
		t.Errorf("RegistryError.Name = %q, want %q", regErr.Name, "dup")
	}
}

// REQ-CORE-003: Registry state is unchanged when Register fails.
func TestRegisterStateUnchangedOnError(t *testing.T) {
	t.Parallel()
	o := initObs(t, io.Discard)
	r := adapters.NewRegistry(o)

	if err := r.Register(newFake("dup")); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	preList := r.List()
	if err := r.Register(newFake("dup")); err == nil {
		t.Fatal("expected error on duplicate")
	}
	postList := r.List()
	if len(preList) != len(postList) {
		t.Errorf("List length changed: pre=%d post=%d", len(preList), len(postList))
	}
}

// REQ-CORE-005: List returns adapter names in sorted order.
func TestListReturnsSortedNames(t *testing.T) {
	t.Parallel()
	o := initObs(t, io.Discard)
	r := adapters.NewRegistry(o)

	for _, n := range []string{"zeta", "alpha", "mu"} {
		if err := r.Register(newFake(n)); err != nil {
			t.Fatalf("Register %q: %v", n, err)
		}
	}
	got := r.List()
	want := []string{"alpha", "mu", "zeta"}
	if len(got) != len(want) {
		t.Fatalf("List length = %d, want %d", len(got), len(want))
	}
	for i, n := range want {
		if got[i] != n {
			t.Errorf("List[%d] = %q, want %q", i, got[i], n)
		}
	}
}

// REQ-CORE-003 + REQ-CORE-006: Auth env var validation rejects when var is unset.
func TestRegisterValidatesAuthEnvVars(t *testing.T) {
	// Not parallel: reads/mutates process env.
	const envVar = "USEARCH_TEST_NEVER_SET_VAR_X"
	_ = os.Unsetenv(envVar)

	o := initObs(t, io.Discard)
	r := adapters.NewRegistry(o)

	a := newFake("authed")
	a.caps.RequiresAuth = true
	a.caps.AuthEnvVars = []string{envVar}

	err := r.Register(a)
	if err == nil {
		t.Fatal("Register: got nil, want error for missing env var")
	}
	if !errors.Is(err, adapters.ErrMissingAuth) {
		t.Errorf("errors.Is(err, ErrMissingAuth) = false, want true; err = %v", err)
	}
	if _, ok := r.Get("authed"); ok {
		t.Error("adapter was registered despite auth failure")
	}
}

// REQ-CORE-003 + REQ-CORE-006: SkipAuthCheck bypasses env validation.
func TestRegisterAllowsSkipAuthCheck(t *testing.T) {
	t.Parallel()
	o := initObs(t, io.Discard)
	r := adapters.NewRegistry(o)

	a := newFake("authed")
	a.caps.RequiresAuth = true
	a.caps.AuthEnvVars = []string{"USEARCH_TEST_NEVER_SET_VAR_Y"}

	if err := r.RegisterWithOptions(a, adapters.RegisterOptions{SkipAuthCheck: true}); err != nil {
		t.Fatalf("RegisterWithOptions: %v", err)
	}
	if _, ok := r.Get("authed"); !ok {
		t.Error("Get returned !ok; want adapter present")
	}
}

// REQ-CORE-006: 4-cell truth table for auth validation.
func TestRegisterRequiresAuthEnvVarsTable(t *testing.T) {
	// Not parallel: mutates process env.
	const envVar = "USEARCH_CORE_TEST_AUTH_VAR"
	const dummy = "dummy"

	cases := []struct {
		name         string
		requiresAuth bool
		setEnv       bool
		skip         bool
		wantErr      bool
	}{
		{"req+set+nofalse", true, true, false, false},
		{"req+unset+nofalse", true, false, false, true},
		{"req+unset+true", true, false, true, false},
		{"noreq+unset+false", false, false, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setEnv {
				t.Setenv(envVar, dummy)
			} else {
				_ = os.Unsetenv(envVar)
			}
			o := initObs(t, io.Discard)
			r := adapters.NewRegistry(o)
			a := newFake("a")
			a.caps.RequiresAuth = tc.requiresAuth
			a.caps.AuthEnvVars = []string{envVar}
			err := r.RegisterWithOptions(a, adapters.RegisterOptions{SkipAuthCheck: tc.skip})
			if tc.wantErr && err == nil {
				t.Errorf("got nil, want error")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("got error %v, want nil", err)
			}
		})
	}
}

// REQ-CORE-004: wrappedAdapter emits one counter increment per outcome.
func TestWrappedAdapterEmitsCounterSuccess(t *testing.T) {
	t.Parallel()
	o := initObs(t, io.Discard)
	r := adapters.NewRegistry(o)
	a := newFake("metricfake-success")
	a.searchFn = func(_ context.Context, _ types.Query) ([]types.NormalizedDoc, error) {
		return nil, nil
	}
	if err := r.Register(a); err != nil {
		t.Fatalf("Register: %v", err)
	}
	w, _ := r.Get("metricfake-success")

	before := readCounter(t, o, a.name, "success")
	_, err := w.Search(context.Background(), types.Query{})
	if err != nil {
		t.Errorf("Search: %v", err)
	}
	after := readCounter(t, o, a.name, "success")
	if after-before != 1 {
		t.Errorf("counter delta = %v, want 1", after-before)
	}
}

func TestWrappedAdapterEmitsCounterFailure(t *testing.T) {
	t.Parallel()
	o := initObs(t, io.Discard)
	r := adapters.NewRegistry(o)
	a := newFake("metricfake-failure")
	a.searchFn = func(_ context.Context, _ types.Query) ([]types.NormalizedDoc, error) {
		return nil, &types.SourceError{Adapter: a.name, Category: types.CategoryPermanent, Cause: errors.New("permanent")}
	}
	if err := r.Register(a); err != nil {
		t.Fatalf("Register: %v", err)
	}
	w, _ := r.Get("metricfake-failure")

	before := readCounter(t, o, a.name, "failure")
	_, _ = w.Search(context.Background(), types.Query{})
	after := readCounter(t, o, a.name, "failure")
	if after-before != 1 {
		t.Errorf("counter delta (failure) = %v, want 1", after-before)
	}
}

func TestWrappedAdapterEmitsCounterTimeout(t *testing.T) {
	t.Parallel()
	o := initObs(t, io.Discard)
	r := adapters.NewRegistry(o)
	a := newFake("metricfake-timeout")
	a.searchFn = func(_ context.Context, _ types.Query) ([]types.NormalizedDoc, error) {
		return nil, context.DeadlineExceeded
	}
	if err := r.Register(a); err != nil {
		t.Fatalf("Register: %v", err)
	}
	w, _ := r.Get("metricfake-timeout")

	before := readCounter(t, o, a.name, "timeout")
	_, _ = w.Search(context.Background(), types.Query{})
	after := readCounter(t, o, a.name, "timeout")
	if after-before != 1 {
		t.Errorf("counter delta (timeout) = %v, want 1", after-before)
	}
}

func TestWrappedAdapterEmitsCounterRateLimited(t *testing.T) {
	t.Parallel()
	o := initObs(t, io.Discard)
	r := adapters.NewRegistry(o)
	a := newFake("metricfake-rate")
	a.searchFn = func(_ context.Context, _ types.Query) ([]types.NormalizedDoc, error) {
		return nil, &types.SourceError{Adapter: a.name, Category: types.CategoryRateLimited, Cause: errors.New("429")}
	}
	if err := r.Register(a); err != nil {
		t.Fatalf("Register: %v", err)
	}
	w, _ := r.Get("metricfake-rate")

	before := readCounter(t, o, a.name, "rate_limited")
	_, _ = w.Search(context.Background(), types.Query{})
	after := readCounter(t, o, a.name, "rate_limited")
	if after-before != 1 {
		t.Errorf("counter delta (rate_limited) = %v, want 1", after-before)
	}
}

func TestWrappedAdapterEmitsCounterUnavailable(t *testing.T) {
	t.Parallel()
	o := initObs(t, io.Discard)
	r := adapters.NewRegistry(o)
	a := newFake("metricfake-unavail")
	a.searchFn = func(_ context.Context, _ types.Query) ([]types.NormalizedDoc, error) {
		return nil, &types.SourceError{Adapter: a.name, Category: types.CategoryUnavailable, Cause: errors.New("503")}
	}
	if err := r.Register(a); err != nil {
		t.Fatalf("Register: %v", err)
	}
	w, _ := r.Get("metricfake-unavail")

	before := readCounter(t, o, a.name, "unavailable")
	_, _ = w.Search(context.Background(), types.Query{})
	after := readCounter(t, o, a.name, "unavailable")
	if after-before != 1 {
		t.Errorf("counter delta (unavailable) = %v, want 1", after-before)
	}
}

// REQ-CORE-004: Histogram observes the elapsed time.
func TestWrappedAdapterEmitsHistogram(t *testing.T) {
	t.Parallel()
	o := initObs(t, io.Discard)
	r := adapters.NewRegistry(o)
	a := newFake("histofake")
	a.searchFn = func(_ context.Context, _ types.Query) ([]types.NormalizedDoc, error) {
		time.Sleep(2 * time.Millisecond)
		return nil, nil
	}
	if err := r.Register(a); err != nil {
		t.Fatalf("Register: %v", err)
	}
	w, _ := r.Get("histofake")

	beforeCount, beforeSum := readHistogram(t, o, a.name)
	_, _ = w.Search(context.Background(), types.Query{})
	afterCount, afterSum := readHistogram(t, o, a.name)

	if afterCount-beforeCount != 1 {
		t.Errorf("histogram count delta = %d, want 1", afterCount-beforeCount)
	}
	if afterSum-beforeSum <= 0 {
		t.Errorf("histogram sum delta = %v, want > 0", afterSum-beforeSum)
	}
}

// REQ-CORE-004: OTel span captured with expected attributes.
// Not parallel: mutates global OTel TracerProvider.
func TestWrappedAdapterCreatesOTelSpan(t *testing.T) {
	o := initObs(t, io.Discard)
	// initObs installs a no-op TracerProvider; override with an SDK provider
	// AFTER initObs has run so the global remains the SDK one for the duration
	// of this test. Restored on cleanup.
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(sdktrace.NewSimpleSpanProcessor(exp)),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		otel.SetTracerProvider(prev)
		_ = tp.Shutdown(context.Background())
	})

	r := adapters.NewRegistry(o)
	a := newFake("spanfake")
	a.searchFn = func(_ context.Context, _ types.Query) ([]types.NormalizedDoc, error) {
		return []types.NormalizedDoc{{ID: "x"}, {ID: "y"}}, nil
	}
	if err := r.Register(a); err != nil {
		t.Fatalf("Register: %v", err)
	}
	w, _ := r.Get("spanfake")
	_, _ = w.Search(context.Background(), types.Query{})

	spans := exp.GetSpans()
	if len(spans) == 0 {
		t.Fatal("no spans captured")
	}
	var found bool
	for _, s := range spans {
		if s.Name != "adapter.search" {
			continue
		}
		found = true
		seen := map[string]bool{}
		for _, kv := range s.Attributes {
			seen[string(kv.Key)] = true
		}
		for _, want := range []string{"adapter.name", "adapter.outcome", "adapter.result_count"} {
			if !seen[want] {
				t.Errorf("span missing attribute %q", want)
			}
		}
	}
	if !found {
		t.Fatalf("span %q not found among %d spans", "adapter.search", len(spans))
	}
}

// REQ-CORE-004: slog record emitted with expected attributes.
func TestWrappedAdapterEmitsSlogRecord(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	o := initObs(t, &buf)
	r := adapters.NewRegistry(o)
	a := newFake("logfake")
	a.searchFn = func(_ context.Context, _ types.Query) ([]types.NormalizedDoc, error) {
		return []types.NormalizedDoc{{ID: "z"}}, nil
	}
	if err := r.Register(a); err != nil {
		t.Fatalf("Register: %v", err)
	}
	w, _ := r.Get("logfake")
	_, _ = w.Search(context.Background(), types.Query{})

	out := buf.String()
	if !strings.Contains(out, `"adapter":"logfake"`) {
		t.Errorf("slog output missing adapter attribute: %s", out)
	}
	if !strings.Contains(out, `"outcome":"success"`) {
		t.Errorf("slog output missing outcome attribute: %s", out)
	}
	// At least one valid JSON line should parse.
	scanned := false
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err == nil {
			scanned = true
			break
		}
	}
	if !scanned {
		t.Errorf("no parseable JSON line in slog output: %s", out)
	}
}

// TestWrappedAdapterEmitsFailureClassAttribute verifies SPEC-EVAL-002
// REQ-EVAL2-005 / AC-003: a failing Search emits a `failure_class` slog
// attribute alongside the existing `outcome` label. A TLS error must classify
// as failure_class="tls" while outcome stays "failure".
func TestWrappedAdapterEmitsFailureClassAttribute(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	o := initObs(t, &buf)
	r := adapters.NewRegistry(o)
	a := newFake("tlsfake")
	a.searchFn = func(_ context.Context, _ types.Query) ([]types.NormalizedDoc, error) {
		return nil, &types.SourceError{
			Adapter:  a.name,
			Category: types.CategoryUnavailable,
			Cause:    tls.RecordHeaderError{Msg: "first record does not look like a TLS handshake"},
		}
	}
	if err := r.Register(a); err != nil {
		t.Fatalf("Register: %v", err)
	}
	w, _ := r.Get("tlsfake")
	_, _ = w.Search(context.Background(), types.Query{})

	out := buf.String()
	if !strings.Contains(out, `"failure_class":"tls"`) {
		t.Errorf("slog output missing failure_class=tls attribute: %s", out)
	}
	if !strings.Contains(out, `"outcome":"unavailable"`) {
		t.Errorf("slog output missing outcome attribute: %s", out)
	}
}

// TestWrappedAdapterSuccessHasNoFailureClass verifies a successful call does
// NOT emit a failure_class attribute (it is only added on the error path).
func TestWrappedAdapterSuccessHasNoFailureClass(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	o := initObs(t, &buf)
	r := adapters.NewRegistry(o)
	a := newFake("okfake")
	a.searchFn = func(_ context.Context, _ types.Query) ([]types.NormalizedDoc, error) {
		return []types.NormalizedDoc{{ID: "1"}}, nil
	}
	if err := r.Register(a); err != nil {
		t.Fatalf("Register: %v", err)
	}
	w, _ := r.Get("okfake")
	_, _ = w.Search(context.Background(), types.Query{})

	if strings.Contains(buf.String(), "failure_class") {
		t.Errorf("successful call should not emit failure_class: %s", buf.String())
	}
}

// REQ-CORE-004: Underlying error preserved through the wrapper.
func TestWrappedAdapterPreservesUnderlyingError(t *testing.T) {
	t.Parallel()
	o := initObs(t, io.Discard)
	r := adapters.NewRegistry(o)
	a := newFake("errpreserve")
	original := errors.New("specific failure")
	a.searchFn = func(_ context.Context, _ types.Query) ([]types.NormalizedDoc, error) {
		return nil, original
	}
	if err := r.Register(a); err != nil {
		t.Fatalf("Register: %v", err)
	}
	w, _ := r.Get("errpreserve")

	_, got := w.Search(context.Background(), types.Query{})
	if !errors.Is(got, original) {
		t.Errorf("errors.Is(returned, original) = false; returned = %v", got)
	}
}

// REQ-CORE-004: Wrapper does not panic when Obs is nil or partially populated.
func TestWrappedAdapterSafeOnNilObs(t *testing.T) {
	t.Parallel()
	r := adapters.NewRegistry(nil)
	a := newFake("nilobs")
	if err := r.Register(a); err != nil {
		t.Fatalf("Register: %v", err)
	}
	w, _ := r.Get("nilobs")

	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("Search panicked with nil Obs: %v", rec)
		}
	}()
	if _, err := w.Search(context.Background(), types.Query{}); err != nil {
		t.Errorf("Search with nil Obs: %v", err)
	}
}

// REQ-CORE-005: Concurrent Register / Get / List under -race.
func TestRegistryConcurrentReadWrite(t *testing.T) {
	t.Parallel()
	o := initObs(t, io.Discard)
	r := adapters.NewRegistry(o)

	stop := make(chan struct{})
	var wg sync.WaitGroup

	// 99 readers.
	for i := 0; i < 99; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_ = r.List()
					_, _ = r.Get("a-0")
				}
			}
		}()
	}
	// 1 writer.
	registered := atomic.Int64{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		i := 0
		for {
			select {
			case <-stop:
				return
			default:
				name := fmt.Sprintf("a-%d", i)
				if err := r.Register(newFake(name)); err == nil {
					registered.Add(1)
				}
				i++
			}
		}
	}()

	time.Sleep(200 * time.Millisecond)
	close(stop)
	wg.Wait()

	got := r.List()
	if int64(len(got)) != registered.Load() {
		t.Errorf("List length = %d, registered = %d", len(got), registered.Load())
	}
	for i := 1; i < len(got); i++ {
		if got[i-1] > got[i] {
			t.Errorf("List not sorted at index %d: %q > %q", i, got[i-1], got[i])
			break
		}
	}
}

// NFR-CORE-002: outcome label values are bounded to the enumerated set.
func TestAdapterOutcomeLabels(t *testing.T) {
	t.Parallel()
	allowed := map[string]bool{
		"success":      true,
		"failure":      true,
		"timeout":      true,
		"rate_limited": true,
		"unavailable":  true,
		"transient":    true, // documented internal value (NFR-CORE-002)
	}
	cases := []struct {
		name string
		err  error
	}{
		{"nil", nil},
		{"deadline", context.DeadlineExceeded},
		{"permanent", types.ErrPermanent},
		{"rate", &types.SourceError{Category: types.CategoryRateLimited}},
		{"unavail", &types.SourceError{Category: types.CategoryUnavailable}},
		{"trans", &types.SourceError{Category: types.CategoryTransient}},
		{"random", errors.New("x")},
	}
	for _, tc := range cases {
		o := types.OutcomeFromError(tc.err)
		if !allowed[o] {
			t.Errorf("OutcomeFromError(%s) = %q, not in allowlist", tc.name, o)
		}
	}
}

// readCounter reads a single counter cell value from o.Metrics.AdapterCalls.
func readCounter(t *testing.T, o *obs.Obs, adapter, outcome string) float64 {
	t.Helper()
	m := &dto.Metric{}
	if err := o.Metrics.AdapterCalls.WithLabelValues(adapter, outcome).Write(m); err != nil {
		t.Fatalf("counter Write: %v", err)
	}
	if m.Counter == nil {
		return 0
	}
	return m.Counter.GetValue()
}

// readHistogram returns (sample_count, sample_sum) for the histogram cell.
func readHistogram(t *testing.T, o *obs.Obs, adapter string) (uint64, float64) {
	t.Helper()
	m := &dto.Metric{}
	hv := o.Metrics.AdapterCallDuration.WithLabelValues(adapter)
	if w, ok := hv.(interface{ Write(*dto.Metric) error }); ok {
		if err := w.Write(m); err != nil {
			t.Fatalf("histogram Write: %v", err)
		}
	} else {
		t.Fatalf("histogram cell does not implement Write")
	}
	if m.Histogram == nil {
		return 0, 0
	}
	return m.Histogram.GetSampleCount(), m.Histogram.GetSampleSum()
}

// TestSnapshotForAdminPopulatesCounts verifies SPEC-EVAL-002 REQ-EVAL2-010a:
// after recorded Search calls, SnapshotForAdmin fills success_count/fail_count/
// success_rate from telemetry instead of leaving them at 0 stubs.
func TestSnapshotForAdminPopulatesCounts(t *testing.T) {
	t.Parallel()
	o := initObs(t, io.Discard)
	r := adapters.NewRegistry(o)

	a := newFake("countfake")
	calls := 0
	a.searchFn = func(_ context.Context, _ types.Query) ([]types.NormalizedDoc, error) {
		calls++
		if calls <= 3 {
			return []types.NormalizedDoc{{ID: "ok"}}, nil
		}
		return nil, &types.SourceError{Adapter: "countfake", Category: types.CategoryPermanent, Cause: errors.New("boom")}
	}
	if err := r.Register(a); err != nil {
		t.Fatalf("Register: %v", err)
	}
	w, _ := r.Get("countfake")
	// 3 success, 2 failure.
	for i := 0; i < 5; i++ {
		_, _ = w.Search(context.Background(), types.Query{})
	}

	views := r.SnapshotForAdmin()
	var v *adapters.AdapterAdminView
	for i := range views {
		if views[i].ID == "countfake" {
			v = &views[i]
		}
	}
	if v == nil {
		t.Fatal("countfake missing from snapshot")
	}
	if v.SuccessCount != 3 {
		t.Errorf("SuccessCount: got %d, want 3", v.SuccessCount)
	}
	if v.FailCount != 2 {
		t.Errorf("FailCount: got %d, want 2", v.FailCount)
	}
	if v.SuccessRate < 0.59 || v.SuccessRate > 0.61 {
		t.Errorf("SuccessRate: got %v, want ~0.60", v.SuccessRate)
	}
}

// TestSnapshotForAdminNilObsSafe verifies the telemetry fill degrades to zero
// counts when obs/metrics are absent (preserves the nil-obs contract).
func TestSnapshotForAdminNilObsSafe(t *testing.T) {
	t.Parallel()
	r := adapters.NewRegistry(nil)
	a := newFake("nilobssnap")
	if err := r.Register(a); err != nil {
		t.Fatalf("Register: %v", err)
	}
	views := r.SnapshotForAdmin()
	if len(views) != 1 {
		t.Fatalf("want 1 view, got %d", len(views))
	}
	if views[0].SuccessCount != 0 || views[0].FailCount != 0 || views[0].SuccessRate != 0 {
		t.Errorf("nil-obs snapshot should have zero counts, got %+v", views[0])
	}
}

// TestHealthSnapshotStatusMapping verifies SPEC-EVAL-002 REQ-EVAL2-010b health
// status classification thresholds via recorded Search outcomes.
func TestHealthSnapshotStatusMapping(t *testing.T) {
	t.Parallel()
	o := initObs(t, io.Discard)
	r := adapters.NewRegistry(o)

	// healthy: 20/0 = 1.0; never-called: healthy (no evidence).
	healthy := newFake("h")
	healthy.searchFn = func(_ context.Context, _ types.Query) ([]types.NormalizedDoc, error) {
		return []types.NormalizedDoc{{ID: "x"}}, nil
	}
	uncalled := newFake("u")
	if err := r.Register(healthy); err != nil {
		t.Fatalf("Register healthy: %v", err)
	}
	if err := r.Register(uncalled); err != nil {
		t.Fatalf("Register uncalled: %v", err)
	}
	wh, _ := r.Get("h")
	for i := 0; i < 20; i++ {
		_, _ = wh.Search(context.Background(), types.Query{})
	}

	snap := r.HealthSnapshot()
	got := map[string]string{}
	for _, a := range snap {
		got[a.Name] = a.Status
		if a.CircuitState != "closed" {
			t.Errorf("%q circuit_state = %q, want closed", a.Name, a.CircuitState)
		}
	}
	if got["h"] != "healthy" {
		t.Errorf("h status = %q, want healthy", got["h"])
	}
	if got["u"] != "healthy" {
		t.Errorf("u (uncalled) status = %q, want healthy", got["u"])
	}
}
