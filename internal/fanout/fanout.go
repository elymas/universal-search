// Package fanout implements the multi-source fanout orchestrator.
// SPEC-FAN-001: goroutine pool, per-adapter timeout, partial-result assembly,
// deduplication, and per-call observability.
package fanout

import (
	"context"
	"time"

	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/elymas/universal-search/internal/adapters"
	"github.com/elymas/universal-search/internal/obs"
	"github.com/elymas/universal-search/internal/router"
	"github.com/elymas/universal-search/pkg/types"
)

// Fanout is an immutable post-construction struct. All exported methods are
// safe for concurrent invocation by multiple goroutines (REQ-FAN-009).
//
// @MX:ANCHOR: [AUTO] Sole entry point for all multi-source fanout dispatches.
// @MX:REASON: contract boundary; signature change ripples to CLI-001 + MCP-001 + IDX-001.
// fan_in >= 3 (cmd/usearch/query.go, tests, future SPEC-MCP-001).
// @MX:SPEC: SPEC-FAN-001
type Fanout struct {
	registry          *adapters.Registry
	obs               *obs.Obs
	maxParallel       int
	perAdapterTimeout time.Duration
	defaultDeadline   time.Duration
}

// New constructs a Fanout from opts, normalising zero-value fields to documented
// defaults. Returns (nil, ErrAdapterRegistryEmpty) when opts.Registry is nil or
// contains zero adapters (REQ-FAN-001).
func New(opts Options) (*Fanout, error) {
	if opts.Registry == nil || len(opts.Registry.List()) == 0 {
		return nil, ErrAdapterRegistryEmpty
	}
	return &Fanout{
		registry:          opts.Registry,
		obs:               opts.Obs,
		maxParallel:       firstNonZeroInt(opts.MaxParallel, defaultMaxParallel),
		perAdapterTimeout: firstNonZeroDuration(opts.PerAdapterTimeout, defaultPerAdapterTimeout),
		defaultDeadline:   firstNonZeroDuration(opts.DefaultDeadline, defaultDeadline),
	}, nil
}

// Dispatch executes a fan-out search across the adapters in decision.AdapterSet.
// It is the sole public entry point for the fanout package.
//
// Behaviour:
//   - Returns (result, ErrEmptyAdapterSet) when decision.AdapterSet is empty (REQ-FAN-008).
//   - Returns (*Result, nil) for every other invocation — including partial-result
//     and all-failure cases (REQ-FAN-003, REQ-FAN-004).
//   - Does NOT consume Query.Deadline; the caller MUST apply it to parentCtx (REQ-FAN-013 / §2.7).
//
// @MX:WARN: [AUTO] Dispatch spawns up to MaxParallel goroutines via errgroup.
// @MX:REASON: removing per-goroutine defer recover()/FanoutInflight.Dec() invalidates
// NFR-FAN-003 zero-leak guarantee. The suppress-error idiom (workers return nil)
// prevents first-error cancellation (D1 locked decision).
// @MX:SPEC: SPEC-FAN-001
func (f *Fanout) Dispatch(
	ctx context.Context,
	decision router.RoutingDecision,
	q types.Query,
) (*Result, error) {
	// REQ-FAN-008: reject empty AdapterSet immediately.
	if len(decision.AdapterSet) == 0 {
		emitEmpty(ctx, f.obs)
		return &Result{
			Docs:  []types.NormalizedDoc{},
			Stats: Stats{AdapterCount: 0},
		}, ErrEmptyAdapterSet
	}

	// Start OTel parent span (REQ-FAN-010).
	tr := tracer(f.obs)
	spanCtx, span := tr.Start(ctx, "fanout.dispatch",
		oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
	defer span.End()

	start := time.Now()

	// Hot path: errgroup orchestration (dispatch.go).
	res, _ := dispatch(spanCtx, f.obs, f.registry, f.maxParallel, f.perAdapterTimeout, decision, q)

	// Post-dispatch: dedup + sort + stats.
	res.Docs, res.Stats.DedupDropped = dedupDocs(res.Docs)
	sortDocs(res.Docs)
	res.Stats.ElapsedSeconds = time.Since(start).Seconds()

	// Observability: span attributes + slog summary (REQ-FAN-010).
	emitDispatch(spanCtx, f.obs, span, decision, res)

	return res, nil
}
