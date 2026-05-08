// Package index — observability helpers for emitting spans, metrics, and logs.
// SPEC-IDX-001 REQ-IDX-011 (scope item l).
package index

import (
	"context"
	"log/slog"
	"time"

	"github.com/elymas/universal-search/internal/obs"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// emitSearch writes the per-Search observability payload.
// Nil-safe across obs, obs.Metrics, individual collectors, and obs.Logger.
func emitSearch(o *obs.Obs, span trace.Span, result *IndexResult, perStoreErrs map[string]error, elapsed time.Duration) {
	if result == nil {
		return
	}

	errCount := 0
	for _, e := range perStoreErrs {
		if e != nil {
			errCount++
		}
	}

	// OTel span attributes.
	if span != nil {
		span.SetAttributes(
			attribute.String("index.op", "search"),
			attribute.Int("index.fused_count", result.Stats.FusedCount),
			attribute.Int("index.errors_count", errCount),
			attribute.Float64("index.elapsed_seconds", elapsed.Seconds()),
		)
	}

	// Prometheus counters and histograms.
	if o != nil && o.Metrics != nil {
		m := o.Metrics
		for store, cnt := range result.Stats.PerStoreCounts {
			outcome := "success"
			if perStoreErrs[store] != nil {
				outcome = "failure"
			}
			if m.IndexOps != nil {
				m.IndexOps.WithLabelValues(store, "search", outcome).Inc()
			}
			if m.IndexOpDuration != nil {
				if lat, ok := result.Stats.StoreLatencies[store]; ok {
					m.IndexOpDuration.WithLabelValues(store, "search").Observe(lat.Seconds())
				}
			}
			_ = cnt
		}
		if m.IndexFusionDuration != nil {
			m.IndexFusionDuration.Observe(result.Stats.FusionLatency.Seconds())
		}
	}

	// Structured log.
	if o != nil && o.Logger != nil {
		lvl := slog.LevelInfo
		if errCount > 0 {
			lvl = slog.LevelWarn
		}

		args := []any{
			"op", "search",
			"fused_count", result.Stats.FusedCount,
			"errors_count", errCount,
			"elapsed_seconds", elapsed.Seconds(),
		}
		for s, n := range result.Stats.PerStoreCounts {
			args = append(args, "store_"+s+"_count", n)
		}
		o.Logger.Log(context.Background(), lvl, "index.search", args...)
	}
}

// emitUpsert writes the per-Upsert observability payload.
// Nil-safe across obs, obs.Metrics, individual collectors, and obs.Logger.
func emitUpsert(o *obs.Obs, span trace.Span, result *UpsertResult, elapsed time.Duration) {
	if result == nil {
		return
	}

	errCount := 0
	for k, e := range result.PerStoreErrors {
		if k != "validation" && e != nil {
			errCount++
		}
	}

	// OTel span attributes.
	if span != nil {
		span.SetAttributes(
			attribute.String("index.op", "upsert"),
			attribute.Int("index.docs_count", result.Stats.DocCount),
			attribute.Int("index.errors_count", errCount),
			attribute.Float64("index.elapsed_seconds", elapsed.Seconds()),
		)
	}

	// Prometheus counters and histograms.
	if o != nil && o.Metrics != nil {
		m := o.Metrics
		for store, lat := range result.Stats.PerStoreLatencies {
			outcome := "success"
			if result.PerStoreErrors[store] != nil {
				outcome = "failure"
			}
			if m.IndexOps != nil {
				m.IndexOps.WithLabelValues(store, "upsert", outcome).Inc()
			}
			if m.IndexOpDuration != nil {
				m.IndexOpDuration.WithLabelValues(store, "upsert").Observe(lat.Seconds())
			}
		}
	}

	// Structured log.
	if o != nil && o.Logger != nil {
		o.Logger.Info("index.upsert",
			"op", "upsert",
			"doc_count", result.Stats.DocCount,
			"skipped_count", result.Stats.SkippedCount,
			"errors_count", errCount,
			"elapsed_seconds", elapsed.Seconds(),
		)
	}
}
