// Package obs is the public observability API for Universal Search.
// It wires together structured logging (slog), Prometheus metrics, and
// OpenTelemetry tracing into a single Init/shutdown lifecycle.
//
// REQ-OBS-001: slog JSON logger
// REQ-OBS-003: Prometheus metrics registry
// REQ-OBS-004: Admin HTTP server (/metrics, /healthz)
// REQ-OBS-005: OTel TracerProvider
// REQ-OBS-006: Shutdown idempotency
package obs

import (
	"context"
	"io"
	"log/slog"
	"os"
	"runtime"

	obslog "github.com/elymas/universal-search/internal/obs/log"
	"github.com/elymas/universal-search/internal/obs/metrics"
	obstrace "github.com/elymas/universal-search/internal/obs/trace"
	"github.com/prometheus/client_golang/prometheus"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// Config holds all observability initialisation parameters.
type Config struct {
	ServiceName    string
	ServiceVersion string
	GitCommit      string

	// LogLevel is the minimum log level: DEBUG, INFO, WARN, ERROR. Default INFO.
	LogLevel string
	// LogWriter is the log output destination. Defaults to os.Stderr.
	LogWriter io.Writer

	// AdminAddr is the TCP address for the Prometheus admin server
	// (e.g. "127.0.0.1:9090"). Leave empty to disable.
	AdminAddr string

	// OTLPEndpoint is the gRPC endpoint for OTel trace export (e.g. "localhost:4317").
	// Leave empty to install a no-op tracer provider.
	OTLPEndpoint string
	// SampleRatio is the trace sampling ratio [0.0, 1.0]. Default 0.1.
	SampleRatio float64
}

// Obs bundles the initialised observability components for use by callers.
//
// @MX:ANCHOR: [AUTO] Central obs bundle; callers: cmd mains, HTTP handlers, tests
// @MX:REASON: fan_in >= 3; single struct passed to all instrumentation call sites
type Obs struct {
	// Logger is the application-wide structured logger.
	Logger *slog.Logger
	// Metrics is the Prometheus registry with all named collectors.
	Metrics *metrics.Registry
	// AdminAddr is the actual address the admin server is listening on.
	// Empty when no admin server was started.
	AdminAddr string

	tracerProvider func(name string) oteltrace.Tracer
}

// Tracer returns a named OTel tracer from the initialised provider.
func (o *Obs) Tracer(name string) oteltrace.Tracer {
	return o.tracerProvider(name)
}

// HasTracer reports whether this Obs bundle has a tracer provider wired.
// Returns false for zero-value or partially-initialised Obs bundles used in tests.
func (o *Obs) HasTracer() bool {
	return o != nil && o.tracerProvider != nil
}

// Init initialises all observability subsystems from cfg and returns an Obs
// bundle and a shutdown function. The shutdown function is idempotent and safe
// to call multiple times.
//
// @MX:ANCHOR: [AUTO] Obs lifecycle entry point; callers: cmd/usearch, cmd/usearch-api, cmd/usearch-mcp, tests
// @MX:REASON: fan_in >= 3; wires slog+prometheus+otel in a single call
func Init(ctx context.Context, cfg Config) (*Obs, func(context.Context) error, error) {
	// --- Logger ---
	w := cfg.LogWriter
	if w == nil {
		w = os.Stderr
	}
	level := obslog.LevelFromEnv(cfg.LogLevel)
	logger := obslog.New(w, level)

	// --- Metrics ---
	reg := metrics.NewRegistry()
	reg.BuildInfo.WithLabelValues(cfg.ServiceVersion, cfg.GitCommit, goVersion()).Set(1)

	// --- Admin server ---
	var adminAddr string
	var shutdownAdmin func(context.Context) error
	if cfg.AdminAddr != "" {
		addr, sd, err := metrics.StartAdminServer(ctx, cfg.AdminAddr, reg)
		if err != nil {
			return nil, nil, err
		}
		adminAddr = addr
		shutdownAdmin = sd
	}

	// --- Tracing ---
	traceCfg := obstrace.Config{
		ServiceName:    cfg.ServiceName,
		ServiceVersion: cfg.ServiceVersion,
		GitCommit:      cfg.GitCommit,
		OTLPEndpoint:   cfg.OTLPEndpoint,
		SampleRatio:    cfg.SampleRatio,
	}
	shutdownTrace, err := obstrace.Init(ctx, traceCfg)
	if err != nil {
		if shutdownAdmin != nil {
			_ = shutdownAdmin(ctx)
		}
		return nil, nil, err
	}

	o := &Obs{
		Logger:         logger,
		Metrics:        reg,
		AdminAddr:      adminAddr,
		tracerProvider: obstrace.Tracer,
	}

	var shutdownCalled bool
	shutdown := func(ctx context.Context) error {
		if shutdownCalled {
			return nil
		}
		shutdownCalled = true

		var firstErr error
		if shutdownAdmin != nil {
			if err := shutdownAdmin(ctx); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		if err := shutdownTrace(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
		return firstErr
	}

	return o, shutdown, nil
}

// goVersion returns the Go runtime version string (e.g. "go1.25.0").
func goVersion() string {
	return runtime.Version()
}

// SynthesisCalls re-exports the synthesis calls counter from the Metrics registry.
// Returns nil when Metrics is nil (safe for tests).
func (o *Obs) SynthesisCalls() *prometheus.CounterVec {
	if o == nil || o.Metrics == nil {
		return nil
	}
	return o.Metrics.SynthesisCalls
}

// SynthesisLatency re-exports the synthesis latency histogram from the Metrics registry.
// Returns nil when Metrics is nil (safe for tests).
func (o *Obs) SynthesisLatency() *prometheus.HistogramVec {
	if o == nil || o.Metrics == nil {
		return nil
	}
	return o.Metrics.SynthesisLatency
}

// SynthesisCost re-exports the synthesis cost counter from the Metrics registry.
// Returns nil when Metrics is nil (safe for tests).
func (o *Obs) SynthesisCost() prometheus.Counter {
	if o == nil || o.Metrics == nil {
		return nil
	}
	return o.Metrics.SynthesisCost
}

// TokenizerCalls re-exports the tokenizer calls counter from the Metrics registry.
// Returns nil when Metrics is nil (safe for tests).
func (o *Obs) TokenizerCalls() *prometheus.CounterVec {
	if o == nil || o.Metrics == nil {
		return nil
	}
	return o.Metrics.TokenizerCalls
}

// TokenizerLatency re-exports the tokenizer latency histogram from the Metrics registry.
// Returns nil when Metrics is nil (safe for tests).
func (o *Obs) TokenizerLatency() *prometheus.HistogramVec {
	if o == nil || o.Metrics == nil {
		return nil
	}
	return o.Metrics.TokenizerLatency
}

// IndexShardWrites re-exports the shard write counter from the Metrics registry.
// Returns nil when Metrics is nil (safe for tests).
func (o *Obs) IndexShardWrites() *prometheus.CounterVec {
	if o == nil || o.Metrics == nil {
		return nil
	}
	return o.Metrics.IndexShardWrites
}

// SynthesisFaithfulnessOutcomes re-exports the faithfulness outcomes CounterVec
// from the Metrics registry. Returns nil when Metrics is nil (safe for tests).
// SPEC-SYN-002 §2.1(h).
func (o *Obs) SynthesisFaithfulnessOutcomes() *prometheus.CounterVec {
	if o == nil || o.Metrics == nil {
		return nil
	}
	return o.Metrics.SynthesisFaithfulnessOutcomes
}

// SynthesisFaithfulnessRetries re-exports the faithfulness retries Counter
// from the Metrics registry. Returns nil when Metrics is nil (safe for tests).
// SPEC-SYN-002 §2.1(h).
func (o *Obs) SynthesisFaithfulnessRetries() prometheus.Counter {
	if o == nil || o.Metrics == nil {
		return nil
	}
	return o.Metrics.SynthesisFaithfulnessRetries
}
