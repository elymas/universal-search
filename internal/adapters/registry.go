// Package adapters — Registry, RegisterOptions, RegistryError, and the
// internal wrappedAdapter that emits per-call observability for every
// registered Adapter.Search.
//
// REQ-CORE-003: Register / RegisterWithOptions / Get / List with concurrency
// safety, duplicate detection, and auth env-var validation.
// REQ-CORE-004: wrappedAdapter emits one counter, one histogram, one OTel
// span, and one slog record per Search call. Underlying error preserved.
// REQ-CORE-005: sync.RWMutex; List returns sorted names.
// REQ-CORE-006: AuthEnvVars validated unless RegisterOptions.SkipAuthCheck.
package adapters

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/elymas/universal-search/internal/obs"
	"github.com/elymas/universal-search/pkg/types"
)

// Sentinel errors returned via *RegistryError on Register failures.
var (
	// ErrDuplicateAdapter indicates another adapter is already registered
	// under the requested name.
	ErrDuplicateAdapter = errors.New("adapters: duplicate adapter name")

	// ErrMissingAuth indicates one or more environment variables listed in
	// Capabilities.AuthEnvVars are not set in the process environment.
	ErrMissingAuth = errors.New("adapters: required auth env var not set")
)

// RegisterOptions tunes Registry.RegisterWithOptions. The zero value matches
// Register's behavior (full auth env validation).
type RegisterOptions struct {
	// SkipAuthCheck bypasses Capabilities.AuthEnvVars validation. Useful in
	// tests, dev environments, and pre-flight registration before secrets
	// are loaded.
	SkipAuthCheck bool
}

// RegistryError wraps a sentinel error with the operation name and adapter
// name involved. Recover via errors.As; categorise via errors.Is against
// ErrDuplicateAdapter / ErrMissingAuth.
type RegistryError struct {
	// Op is the operation that failed (currently always "register").
	Op string
	// Name is the adapter name (Adapter.Name()) involved.
	Name string
	// Cause is the underlying sentinel.
	Cause error
}

// Error returns a formatted message.
func (e *RegistryError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("registry %s %q: %v", e.Op, e.Name, e.Cause)
}

// Unwrap returns the inner Cause for use with errors.Is / errors.As.
func (e *RegistryError) Unwrap() error { return e.Cause }

// Registry is the concurrency-safe adapter registry. Mirrors the Router
// pattern at internal/llm/router.go:148-198.
//
// @MX:ANCHOR: [AUTO] Adapter registry; callers: cmd mains, FAN-001 fanout,
// IR-001 router, tests
// @MX:REASON: fan_in >= 3; sole sanctioned source of Adapter instances at
// runtime. Wrapping ensures every Search call emits observability uniformly.
// @MX:SPEC: SPEC-CORE-001
type Registry struct {
	mu       sync.RWMutex
	adapters map[string]types.Adapter // values are *wrappedAdapter
	obs      *obs.Obs
}

// NewRegistry constructs an empty Registry. obs may be nil; the wrappedAdapter
// degrades gracefully (no metrics, no log, no span recording) when components
// are missing — see TestWrappedAdapterSafeOnNilObs.
func NewRegistry(o *obs.Obs) *Registry {
	return &Registry{
		adapters: make(map[string]types.Adapter),
		obs:      o,
	}
}

// Register stores a new Adapter under its Name(). Returns *RegistryError
// wrapping ErrDuplicateAdapter on name collision or ErrMissingAuth when the
// adapter declares RequiresAuth=true and its AuthEnvVars are not set.
//
// @MX:ANCHOR: [AUTO] Adapter registration entry point; callers: cmd mains,
// FAN-001, IR-001, tests
// @MX:REASON: fan_in >= 3 across runtime + tests; the wrappedAdapter is only
// produced via this path
// @MX:SPEC: SPEC-CORE-001
func (r *Registry) Register(a types.Adapter) error {
	return r.RegisterWithOptions(a, RegisterOptions{})
}

// RegisterWithOptions is Register with explicit options.
//
// @MX:WARN: [AUTO] Duplicate-name detection is a load-bearing invariant —
// silent overwrite would invalidate FAN-001's adapter routing table mid-flight.
// @MX:REASON: callers may expect overwrite semantics from common map-style
// APIs; this implementation deliberately rejects duplicates.
// @MX:SPEC: SPEC-CORE-001
func (r *Registry) RegisterWithOptions(a types.Adapter, opts RegisterOptions) error {
	name := a.Name()
	caps := a.Capabilities()

	if !opts.SkipAuthCheck && caps.RequiresAuth {
		for _, ev := range caps.AuthEnvVars {
			if _, ok := os.LookupEnv(ev); !ok {
				return &RegistryError{Op: "register", Name: name, Cause: ErrMissingAuth}
			}
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.adapters[name]; exists {
		return &RegistryError{Op: "register", Name: name, Cause: ErrDuplicateAdapter}
	}
	r.adapters[name] = &wrappedAdapter{inner: a, obs: r.obs}
	return nil
}

// Get returns the wrapped Adapter registered under name. The second return
// value reports whether the name was found.
//
// @MX:ANCHOR: [AUTO] Adapter lookup; callers: FAN-001 fanout, IR-001 router,
// tests
// @MX:REASON: fan_in >= 3; every per-name dispatch flows through Get
// @MX:SPEC: SPEC-CORE-001
func (r *Registry) Get(name string) (types.Adapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.adapters[name]
	return a, ok
}

// List returns the registered adapter names in lexicographic order. Sort
// order is deterministic so downstream consumers (FAN-001 fanout dispatch,
// IR-001 routing-table dump) can rely on stable iteration.
func (r *Registry) List() []string {
	r.mu.RLock()
	names := make([]string, 0, len(r.adapters))
	for n := range r.adapters {
		names = append(names, n)
	}
	r.mu.RUnlock()
	sort.Strings(names)
	return names
}

// wrappedAdapter delegates Adapter methods to inner while emitting metrics,
// span, and slog per Search call. The wrapper is unexported — only the
// Register path constructs it, ensuring every production Adapter access goes
// through the observability layer.
type wrappedAdapter struct {
	inner types.Adapter
	obs   *obs.Obs
}

func (w *wrappedAdapter) Name() string                     { return w.inner.Name() }
func (w *wrappedAdapter) Capabilities() types.Capabilities { return w.inner.Capabilities() }
func (w *wrappedAdapter) Healthcheck(ctx context.Context) error {
	return w.inner.Healthcheck(ctx)
}

// Search runs the underlying adapter with full per-call observability.
// Mirrors internal/llm/client.go:230-252 emitObservability shape.
//
// Emissions per call (each guarded by nil checks):
//   - 1 OTel span "adapter.search" with adapter.name / adapter.outcome /
//     adapter.result_count attributes.
//   - 1 Prometheus counter increment on AdapterCalls{adapter,outcome}.
//   - 1 Prometheus histogram observation on AdapterCallDuration{adapter}.
//   - 1 slog record at INFO (success) or WARN (non-success).
//
// The underlying error is returned unmodified — callers get errors.Is(err,
// originalErr) == true.
func (w *wrappedAdapter) Search(ctx context.Context, q types.Query) ([]types.NormalizedDoc, error) {
	name := w.inner.Name()

	tracer := w.tracer()
	spanCtx, span := tracer.Start(ctx, "adapter.search",
		oteltrace.WithAttributes(attribute.String("adapter.name", name)))
	defer span.End()

	start := time.Now()
	docs, err := w.inner.Search(spanCtx, q)
	elapsed := time.Since(start).Seconds()

	outcome := types.OutcomeFromError(err)
	span.SetAttributes(
		attribute.String("adapter.outcome", outcome),
		attribute.Int("adapter.result_count", len(docs)),
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, outcome)
	}

	w.emit(spanCtx, name, outcome, elapsed, len(docs), err)
	return docs, err
}

// emit records the per-call metrics + slog event. Pulled into a helper so
// nil-guard noise stays out of Search.
func (w *wrappedAdapter) emit(ctx context.Context, name, outcome string, elapsed float64, count int, err error) {
	if w.obs == nil {
		return
	}
	if reg := w.obs.Metrics; reg != nil {
		if reg.AdapterCalls != nil {
			reg.AdapterCalls.WithLabelValues(name, outcome).Inc()
		}
		if reg.AdapterCallDuration != nil {
			reg.AdapterCallDuration.WithLabelValues(name).Observe(elapsed)
		}
	}
	if w.obs.Logger == nil {
		return
	}
	level := slog.LevelInfo
	if err != nil {
		level = slog.LevelWarn
	}
	attrs := []slog.Attr{
		slog.String("adapter", name),
		slog.String("outcome", outcome),
		slog.Float64("elapsed_seconds", elapsed),
		slog.Int("result_count", count),
	}
	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
	}
	w.obs.Logger.LogAttrs(ctx, level, "adapter call", attrs...)
}

// tracer returns the OTel tracer for this wrapper. When obs is nil we fall
// back to the global no-op provider so spans still get created (and the
// caller does not need a separate code path) — the no-op spans are not
// recording and emit nothing.
func (w *wrappedAdapter) tracer() oteltrace.Tracer {
	if w.obs == nil {
		return otel.Tracer("adapter")
	}
	return w.obs.Tracer("adapter")
}
