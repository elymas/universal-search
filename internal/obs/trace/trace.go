// Package trace provides OpenTelemetry TracerProvider initialization for
// Universal Search. When OTLP_ENDPOINT is set, an OTLP gRPC exporter is
// configured with a BatchSpanProcessor and ParentBased(TraceIDRatioBased)
// sampler. When unset, a no-op provider is installed and callers pay zero cost.
//
// REQ-OBS-005: OTel TracerProvider init, OTLP gRPC exporter, W3C propagator,
// configurable sample ratio, no-op when endpoint unset.
package trace

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// Config holds trace initialisation parameters.
type Config struct {
	ServiceName    string
	ServiceVersion string
	GitCommit      string
	// OTLPEndpoint is the gRPC endpoint for the OTLP exporter (e.g. "localhost:4317").
	// If empty, a no-op provider is installed.
	OTLPEndpoint string
	// SampleRatio is the trace sampling ratio [0.0, 1.0]. Default 0.1.
	SampleRatio float64
}

// Init initialises the global OTel TracerProvider from cfg.
// If cfg.OTLPEndpoint is empty a no-op provider is installed; the returned
// shutdown closure is a no-op and no external traffic is generated.
//
// @MX:ANCHOR: [AUTO] OTel global state mutation; callers: obs.Init, cmd mains, tests
// @MX:REASON: fan_in >= 3; sets package-level OTel globals (provider + propagator)
func Init(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	if cfg.OTLPEndpoint == "" {
		otel.SetTracerProvider(noop.NewTracerProvider())
		// Install W3C propagator even in no-op mode so header extraction works.
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		))
		return func(context.Context) error { return nil }, nil
	}

	exp, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	return initWithExporter(ctx, cfg, exp)
}

// InitWithExporter is like Init but accepts a pre-built SpanExporter, which
// enables injecting an in-memory exporter in tests without a real OTLP endpoint.
// It uses a SimpleSpanProcessor (synchronous) so spans are available immediately
// after span.End() — no batch-flush needed in tests.
func InitWithExporter(ctx context.Context, cfg Config, exp sdktrace.SpanExporter) (func(context.Context) error, error) {
	return initWithProcessor(ctx, cfg, sdktrace.NewSimpleSpanProcessor(exp))
}

// InitWithBatchExporter is like InitWithExporter but wraps exp in a
// BatchSpanProcessor, matching the production code path. Useful for verifying
// that batch flush occurs correctly in integration tests.
// Note: call shutdown before reading spans; InMemoryExporter.Shutdown resets its store.
func InitWithBatchExporter(ctx context.Context, cfg Config, exp sdktrace.SpanExporter) (func(context.Context) error, error) {
	return initWithExporter(ctx, cfg, exp)
}

// initWithExporter wires the TracerProvider around the given exporter using a
// BatchSpanProcessor — the production path for OTLP gRPC export.
func initWithExporter(ctx context.Context, cfg Config, exp sdktrace.SpanExporter) (func(context.Context) error, error) {
	return initWithProcessor(ctx, cfg, sdktrace.NewBatchSpanProcessor(exp))
}

// initWithProcessor builds a TracerProvider around the given SpanProcessor.
func initWithProcessor(ctx context.Context, cfg Config, sp sdktrace.SpanProcessor) (func(context.Context) error, error) {
	ratio := cfg.SampleRatio
	if ratio <= 0 {
		ratio = 0.1
	}

	res, _ := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
			semconv.ServiceVersionKey.String(cfg.ServiceVersion),
		),
	)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(sp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(
			sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio)),
		),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}

// Tracer returns a named tracer from the global TracerProvider.
// Domain packages call this instead of otel.Tracer directly.
func Tracer(name string) oteltrace.Tracer {
	return otel.Tracer(name)
}
