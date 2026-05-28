// Package fanout — per-Dispatch observability emission.
// SPEC-FAN-001 REQ-FAN-010, §6.5.
package fanout

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/elymas/universal-search/internal/obs"
	"github.com/elymas/universal-search/internal/obs/reqid"
	"github.com/elymas/universal-search/internal/router"
)

// tracer returns a named OTel tracer. Falls back to the global no-op provider
// when o is nil, matching the wrappedAdapter pattern at registry.go:258-263.
func tracer(o *obs.Obs) oteltrace.Tracer {
	if o == nil {
		return otel.Tracer("fanout")
	}
	return o.Tracer("fanout")
}

// incInflight increments FanoutInflight{adapter_class} if metrics are available.
func incInflight(o *obs.Obs, classLabel string) {
	if o == nil || o.Metrics == nil || o.Metrics.FanoutInflight == nil {
		return
	}
	o.Metrics.FanoutInflight.WithLabelValues(classLabel).Inc()
}

// decInflight decrements FanoutInflight{adapter_class} if metrics are available.
func decInflight(o *obs.Obs, classLabel string) {
	if o == nil || o.Metrics == nil || o.Metrics.FanoutInflight == nil {
		return
	}
	o.Metrics.FanoutInflight.WithLabelValues(classLabel).Dec()
}

// emitDispatch writes the fanout.dispatch span attributes and the slog summary
// record. Nil-safe across obs, obs.Metrics, and obs.Logger per REQ-FAN-010.
func emitDispatch(ctx context.Context, o *obs.Obs, span oteltrace.Span, decision router.RoutingDecision, result *Result) {
	if span != nil {
		span.SetAttributes(
			attribute.String("fanout.category", string(decision.Category)),
			attribute.Int("fanout.adapter_count", result.Stats.AdapterCount),
			attribute.Int("fanout.result_count", len(result.Docs)),
			attribute.Int("fanout.errors_count", result.Stats.ErrorCount),
			attribute.Int("fanout.dedup_dropped", result.Stats.DedupDropped),
			attribute.Float64("fanout.elapsed_seconds", result.Stats.ElapsedSeconds),
		)
		if result.Stats.ErrorCount > 0 {
			span.SetStatus(codes.Error, "partial result")
		} else {
			span.SetStatus(codes.Ok, "")
		}
	}

	if o == nil || o.Logger == nil {
		return
	}

	level := slog.LevelInfo
	if result.Stats.ErrorCount == result.Stats.AdapterCount && result.Stats.AdapterCount > 0 {
		level = slog.LevelWarn
	}

	rid := reqid.FromContext(ctx)
	o.Logger.LogAttrs(ctx, level, "fanout dispatch",
		slog.String("request_id", rid),
		slog.String("category", string(decision.Category)),
		slog.Int("adapter_count", result.Stats.AdapterCount),
		slog.Int("result_count", len(result.Docs)),
		slog.Int("errors_count", result.Stats.ErrorCount),
		slog.Int("dedup_dropped", result.Stats.DedupDropped),
		slog.Float64("elapsed_seconds", result.Stats.ElapsedSeconds),
	)
}

// emitEmpty logs a WARN when Dispatch is called with an empty AdapterSet.
func emitEmpty(ctx context.Context, o *obs.Obs) {
	if o == nil || o.Logger == nil {
		return
	}
	rid := reqid.FromContext(ctx)
	o.Logger.LogAttrs(ctx, slog.LevelWarn, "fanout dispatch",
		slog.String("request_id", rid),
		slog.String("error", ErrEmptyAdapterSet.Error()),
	)
}

// emitPanic logs a WARN with stack trace when an adapter panics.
func emitPanic(ctx context.Context, o *obs.Obs, adapterName string, recovered any, stack []byte) {
	if o == nil || o.Logger == nil {
		return
	}
	o.Logger.LogAttrs(ctx, slog.LevelWarn, "adapter panic recovered",
		slog.String("adapter", adapterName),
		slog.Any("panic_value", recovered),
		slog.String("stack_trace", string(stack)),
	)
}

// emitPartialResultCounters increments usearch_fanout_partial_total once per
// adapter that contributed an error to Result.AdapterErrors (SPEC-EVAL-002
// REQ-EVAL2-004). Called after eg.Wait() returns and before the result is
// returned to the caller.
func emitPartialResultCounters(o *obs.Obs, result *Result) {
	if o == nil || o.Metrics == nil || o.Metrics.FanoutPartial == nil {
		return
	}
	if result.AdapterErrors == nil {
		return
	}
	for adapterName := range result.AdapterErrors {
		o.Metrics.FanoutPartial.WithLabelValues(adapterName).Inc()
	}
}
