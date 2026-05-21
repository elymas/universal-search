// Package access — observability adapter and emission helpers.
//
// REQ-CACHE-010: ONE OTel parent span + per-phase child spans + Prometheus
// counters/histograms + slog summary record per Fetch invocation.
// Nil-safe across obs.Obs, obs.Metrics, individual collectors, and obs.Logger.
package access

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// ObsAdapter is the minimal observability interface the access package needs.
// It is satisfied by *obs.Obs and by testObsAdapter used in unit tests.
// Keeping this interface local avoids a direct import of internal/obs in the
// access package (which would create an import cycle via metrics).
type ObsAdapter interface {
	// Tracer returns a named OTel tracer.
	Tracer(name string) oteltrace.Tracer
	// SlogLogger returns the structured logger; nil-safe.
	SlogLogger() *slog.Logger
	// AccessMetrics returns the access-layer metric collectors; nil-safe.
	AccessMetrics() *AccessCollectors
}

// AccessCollectors bundles the three metric families for the access layer.
//
// @MX:NOTE: [AUTO] Three families only — AccessPhaseAttempts, AccessPhaseDuration,
// AccessFetchTotal. REQ-CACHE-010 prohibits registering additional families.
type AccessCollectors struct {
	// AccessPhaseAttempts counts per-phase attempts, partitioned by phase and outcome.
	AccessPhaseAttempts *prometheus.CounterVec
	// AccessPhaseDuration observes per-phase latency.
	AccessPhaseDuration *prometheus.HistogramVec
	// AccessFetchTotal counts whole-cascade outcomes.
	AccessFetchTotal *prometheus.CounterVec
}

// noopObs is the nil-safe fallback used when Options.Obs is nil.
type noopObs struct{}

func (noopObs) Tracer(_ string) oteltrace.Tracer { return noop.NewTracerProvider().Tracer("") }
func (noopObs) SlogLogger() *slog.Logger         { return nil }
func (noopObs) AccessMetrics() *AccessCollectors { return nil }

// emitPhaseAttempt records one counter + one histogram observation for a
// completed phase attempt.
func emitPhaseAttempt(m *AccessCollectors, attempt *PhaseAttempt) {
	if m == nil {
		return
	}
	phaseStr := strconv.Itoa(attempt.Phase)
	if m.AccessPhaseAttempts != nil {
		m.AccessPhaseAttempts.WithLabelValues(phaseStr, attempt.Outcome).Inc()
	}
	if m.AccessPhaseDuration != nil {
		m.AccessPhaseDuration.WithLabelValues(phaseStr).Observe(attempt.ElapsedSeconds)
	}
}

// emitFetchTotal records one counter observation for the whole-cascade outcome.
func emitFetchTotal(m *AccessCollectors, outcome string) {
	if m == nil || m.AccessFetchTotal == nil {
		return
	}
	m.AccessFetchTotal.WithLabelValues(outcome).Inc()
}

// emitSlog writes a single slog record summarising the completed Fetch call.
func emitSlog(logger *slog.Logger, ctx context.Context, result *FetchResult, urlHost string) {
	if logger == nil {
		return
	}
	level := slog.LevelInfo
	if result.Outcome != "success" {
		level = slog.LevelWarn
	}
	// Extract request_id from context if present (SPEC-OBS-001 reqid pattern).
	reqID := reqIDFromCtx(ctx)
	logger.LogAttrs(ctx, level, "access.fetch",
		slog.String("request_id", reqID),
		slog.String("url_host", urlHost),
		slog.Int("final_phase", result.FinalPhase),
		slog.String("outcome", result.Outcome),
		slog.Float64("elapsed_seconds", result.ElapsedSeconds),
		slog.Int("phase_attempt_count", len(result.PhaseAttempts)),
	)
}

// reqIDFromCtx extracts the request ID from context; returns empty string when absent.
// The key type matches internal/obs/reqid.contextKey to avoid import cycles.
func reqIDFromCtx(ctx context.Context) string {
	type reqIDKey struct{}
	if v := ctx.Value(reqIDKey{}); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// phaseDurationBuckets are tuned for the bimodal distribution:
// Phase 1-2 (sub-second), Phase 3-4 (seconds), Phase 5 (tens of seconds).
var phaseDurationBuckets = []float64{
	0.001, 0.005, 0.01, 0.05, 0.1, 0.2, 0.5, 1, 2.5, 5, 10, 15, 30,
}

// NewAccessCollectors creates the three access-layer metric families and
// registers them on the provided prometheus.Registry.
// Called from internal/obs/metrics/access.go → NewRegistry().
func NewAccessCollectors(r *prometheus.Registry) *AccessCollectors {
	attempts := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "usearch_access_phase_attempts_total",
			Help: "Total access phase attempts, partitioned by phase and outcome.",
		},
		[]string{"phase", "outcome"},
	)
	duration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "usearch_access_phase_duration_seconds",
			Help:    "Access phase latency distribution.",
			Buckets: phaseDurationBuckets,
		},
		[]string{"phase"},
	)
	total := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "usearch_access_fetch_total",
			Help: "Total access fetch calls, partitioned by outcome.",
		},
		[]string{"outcome"},
	)

	r.MustRegister(attempts, duration, total)

	// Pre-initialise placeholder label combinations so families appear in /metrics
	// output before any real access calls.
	for _, phase := range []string{"1", "2", "3", "4", "5"} {
		for _, outcome := range []string{"success", "failure", "timeout", "blocked", "miss", "skipped"} {
			attempts.WithLabelValues(phase, outcome).Add(0)
		}
		duration.WithLabelValues(phase).Observe(0)
	}
	for _, outcome := range []string{"success", "failure", "timeout", "blocked"} {
		total.WithLabelValues(outcome).Add(0)
	}

	return &AccessCollectors{
		AccessPhaseAttempts: attempts,
		AccessPhaseDuration: duration,
		AccessFetchTotal:    total,
	}
}

// spanName returns the OTel span name for a given phase number.
func spanName(phase int) string {
	return "access.phase" + strconv.Itoa(phase)
}

// phaseOutcomeForDuration extracts a duration-compatible label from a PhaseAttempt.
func phaseLabel(phase int) string { return strconv.Itoa(phase) }

// elapsedSince computes elapsed seconds since start.
func elapsedSince(start time.Time) float64 {
	return time.Since(start).Seconds()
}
