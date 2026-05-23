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
	"strings"
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

	// ErrAdapterNotFound indicates the requested adapter ID does not exist
	// in the registry.
	ErrAdapterNotFound = errors.New("adapters: adapter not found")
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

// UpstreamError wraps an error from an adapter's upstream dependency (e.g.,
// Healthcheck failure). Used by the admin API to return 502 with a sanitized
// error message.
type UpstreamError struct {
	AdapterID string
	Err       error
}

func (e *UpstreamError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("adapter %q upstream error: %v", e.AdapterID, e.Err)
}

func (e *UpstreamError) Unwrap() error { return e.Err }

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
	disabled map[string]bool          // adapter name -> disabled state
	obs      *obs.Obs
}

// NewRegistry constructs an empty Registry. obs may be nil; the wrappedAdapter
// degrades gracefully (no metrics, no log, no span recording) when components
// are missing — see TestWrappedAdapterSafeOnNilObs.
func NewRegistry(o *obs.Obs) *Registry {
	return &Registry{
		adapters: make(map[string]types.Adapter),
		disabled: make(map[string]bool),
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

// AdapterAdminView is a read-only snapshot of one adapter for the admin API.
// CRITICAL: Never includes actual secret values — only the source identifier
// (env var name) and a boolean indicating whether the key is set.
//
// @MX:NOTE: [AUTO] Fields with zero values indicate metadata not yet tracked
// by the registry (e.g., SuccessCount, LastSync). These will be populated
// as observability tracking is added.
// @MX:SPEC: SPEC-UI-002 REQ-AS-001, REQ-AK-001, REQ-AS-003
type AdapterAdminView struct {
	// ID is the stable adapter identifier (matches Adapter.Name()).
	ID string `json:"id"`
	// Status is one of: "connected", "auth_required", "disabled", "error".
	// Returns "connected" for registered adapters; refined status requires
	// health check integration (future work).
	Status string `json:"status"`
	// LastSync is the time of the last successful Search call. Zero value
	// means the adapter has not been called or sync tracking is not enabled.
	LastSync time.Time `json:"last_sync"`
	// SuccessCount is the number of successful Search calls. Zero until
	// per-adapter call tracking is implemented.
	SuccessCount int64 `json:"success_count"`
	// FailCount is the number of failed Search calls. Zero until
	// per-adapter call tracking is implemented.
	FailCount int64 `json:"fail_count"`
	// LastError is the error message from the most recent failed call.
	// Empty string until per-adapter error tracking is implemented.
	LastError string `json:"last_error"`
	// SecretSource identifies where the adapter's credentials come from
	// (e.g., the env var name like "REDDIT_CLIENT_SECRET"). Empty for
	// adapters that do not require auth.
	SecretSource string `json:"secret_source"`
	// KeySet reports whether all required auth env vars are present.
	// Always true for adapters that do not require auth.
	KeySet bool `json:"key_set"`
	// SecretValue is ALWAYS empty. This field exists only to verify in tests
	// that no secret value ever leaks into the admin view.
	SecretValue string `json:"-"`
}

// SnapshotForAdmin returns a read-only view of every registered adapter for the
// admin API. The returned slice is sorted by ID. Each entry contains metadata
// about the adapter's status, auth configuration, and call statistics.
//
// CRITICAL SECURITY: This method MUST NOT include actual secret values in any
// field. Only the source identifier (env var name) and set/unset boolean are
// exposed.
//
// @MX:ANCHOR: [AUTO] Admin snapshot; callers: admin handler, tests
// @MX:REASON: fan_in >= 3; sole data source for the admin adapters endpoint.
// Leaking secrets here would expose them via the admin API.
// @MX:SPEC: SPEC-UI-002 REQ-AS-001, REQ-AK-001
func (r *Registry) SnapshotForAdmin() []AdapterAdminView {
	r.mu.RLock()
	defer r.mu.RUnlock()

	views := make([]AdapterAdminView, 0, len(r.adapters))
	for name, a := range r.adapters {
		caps := a.Capabilities()

		secretSource := ""
		keySet := true // default: no auth required means key is "set"

		if caps.RequiresAuth && len(caps.AuthEnvVars) > 0 {
			// Report the env var name(s) as the source identifier.
			secretSource = strings.Join(caps.AuthEnvVars, ",")
			// Check if ALL declared auth env vars are set.
			keySet = true
			for _, ev := range caps.AuthEnvVars {
				if _, ok := os.LookupEnv(ev); !ok {
					keySet = false
					break
				}
			}
		}

		status := "connected"
		if r.disabled[name] {
			status = "disabled"
		}

		views = append(views, AdapterAdminView{
			ID:           name,
			Status:       status,
			SecretSource: secretSource,
			KeySet:       keySet,
			// LastSync, SuccessCount, FailCount, LastError: zero values
			// until per-adapter call tracking is implemented.
		})
	}

	sort.Slice(views, func(i, j int) bool {
		return views[i].ID < views[j].ID
	})

	return views
}

// Resync refreshes the status of a single adapter by running its Healthcheck.
// Returns the updated AdapterAdminView. Returns an error if the adapter is not
// found (ErrAdapterNotFound) or if the health check fails (ErrUpstreamError).
//
// @MX:SPEC: SPEC-UI-002 REQ-AS-002
func (r *Registry) Resync(ctx context.Context, id string) (*AdapterAdminView, error) {
	a, ok := r.Get(id)
	if !ok {
		return nil, ErrAdapterNotFound
	}

	if err := a.Healthcheck(ctx); err != nil {
		return nil, &UpstreamError{AdapterID: id, Err: err}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	caps := a.Capabilities()
	secretSource := ""
	keySet := true
	if caps.RequiresAuth && len(caps.AuthEnvVars) > 0 {
		secretSource = strings.Join(caps.AuthEnvVars, ",")
		for _, ev := range caps.AuthEnvVars {
			if _, ok := os.LookupEnv(ev); !ok {
				keySet = false
				break
			}
		}
	}

	status := "connected"
	if r.disabled[id] {
		status = "disabled"
	}

	view := &AdapterAdminView{
		ID:           id,
		Status:       status,
		SecretSource: secretSource,
		KeySet:       keySet,
	}
	return view, nil
}

// ToggleEnabled toggles the enabled/disabled state of the named adapter.
// Returns the updated AdapterAdminView. Returns ErrAdapterNotFound if the
// adapter does not exist.
//
// @MX:SPEC: SPEC-UI-002 REQ-AK-002
func (r *Registry) ToggleEnabled(ctx context.Context, id string) (*AdapterAdminView, error) {
	a, ok := r.Get(id)
	if !ok {
		return nil, ErrAdapterNotFound
	}

	r.mu.Lock()
	r.disabled[id] = !r.disabled[id]
	r.mu.Unlock()

	caps := a.Capabilities()
	secretSource := ""
	keySet := true
	if caps.RequiresAuth && len(caps.AuthEnvVars) > 0 {
		secretSource = strings.Join(caps.AuthEnvVars, ",")
		for _, ev := range caps.AuthEnvVars {
			if _, ok := os.LookupEnv(ev); !ok {
				keySet = false
				break
			}
		}
	}

	r.mu.RLock()
	status := "connected"
	if r.disabled[id] {
		status = "disabled"
	}
	r.mu.RUnlock()

	view := &AdapterAdminView{
		ID:           id,
		Status:       status,
		SecretSource: secretSource,
		KeySet:       keySet,
	}
	return view, nil
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
