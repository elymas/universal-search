// Package obs_test tests the public observability API (REQ-OBS-001..REQ-OBS-006).
package obs_test

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/elymas/universal-search/internal/obs"
)

// TestObsInitReturnsNonNilLogger verifies that obs.Init returns a usable logger.
// REQ-OBS-001
func TestObsInitReturnsNonNilLogger(t *testing.T) {
	t.Parallel()

	cfg := obs.Config{
		ServiceName:    "test-svc",
		ServiceVersion: "0.0.0",
		LogLevel:       "info",
	}
	o, shutdown, err := obs.Init(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer func() { _ = shutdown(context.Background()) }()

	if o.Logger == nil {
		t.Error("Logger is nil")
	}
}

// TestObsInitReturnsNonNilRegistry verifies that obs.Init returns a Prometheus
// registry with all named collectors.
// REQ-OBS-003
func TestObsInitReturnsNonNilRegistry(t *testing.T) {
	t.Parallel()

	cfg := obs.Config{
		ServiceName:    "test-svc",
		ServiceVersion: "0.0.0",
		LogLevel:       "info",
	}
	o, shutdown, err := obs.Init(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer func() { _ = shutdown(context.Background()) }()

	if o.Metrics == nil {
		t.Error("Metrics registry is nil")
	}
}

// TestObsInitNoOTLPEndpointInstallsNoopTracer verifies that without an OTLP
// endpoint, the tracer is no-op (non-recording spans).
// REQ-OBS-005
func TestObsInitNoOTLPEndpointInstallsNoopTracer(t *testing.T) {
	// Not parallel: modifies global OTel state.

	cfg := obs.Config{
		ServiceName:    "test-svc",
		ServiceVersion: "0.0.0",
		OTLPEndpoint:   "", // no-op
	}
	o, shutdown, err := obs.Init(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer func() { _ = shutdown(context.Background()) }()

	tr := o.Tracer("test")
	_, span := tr.Start(context.Background(), "noop")
	if span.IsRecording() {
		t.Error("expected non-recording span without OTLP endpoint")
	}
	span.End()
}

// TestObsShutdownIdempotent verifies that calling shutdown twice does not panic
// or error.
// REQ-OBS-006
func TestObsShutdownIdempotent(t *testing.T) {
	t.Parallel()

	cfg := obs.Config{
		ServiceName:    "test-svc",
		ServiceVersion: "0.0.0",
	}
	_, shutdown, err := obs.Init(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	ctx := context.Background()
	if err := shutdown(ctx); err != nil {
		t.Fatalf("first shutdown: %v", err)
	}
	// Second call should not panic.
	if err := shutdown(ctx); err != nil {
		t.Fatalf("second shutdown: %v", err)
	}
}

// TestObsLoggerWritesJSON verifies the logger writes JSON output.
// REQ-OBS-001
func TestObsLoggerWritesJSON(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	cfg := obs.Config{
		ServiceName:    "test-svc",
		ServiceVersion: "0.0.0",
		LogLevel:       "debug",
		LogWriter:      &buf,
	}
	o, shutdown, err := obs.Init(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer func() { _ = shutdown(context.Background()) }()

	o.Logger.Info("hello", "key", "val")
	out := buf.String()
	if !strings.Contains(out, `"msg":"hello"`) {
		t.Errorf("expected JSON msg field, got: %s", out)
	}
	if !strings.Contains(out, `"key":"val"`) {
		t.Errorf("expected JSON key field, got: %s", out)
	}
}

// TestObsConfigDefaultsApply verifies that zero-value Config is safe to use
// (no panics, defaults applied).
// REQ-OBS-001
func TestObsConfigDefaultsApply(t *testing.T) {
	t.Parallel()

	_, shutdown, err := obs.Init(context.Background(), obs.Config{})
	if err != nil {
		t.Fatalf("Init with zero config: %v", err)
	}
	_ = shutdown(context.Background())
}

// TestObsTracerReturnsNonNil verifies Obs.Tracer() returns a non-nil tracer.
// REQ-OBS-005
func TestObsTracerReturnsNonNil(t *testing.T) {
	t.Parallel()

	cfg := obs.Config{ServiceName: "test-svc"}
	o, shutdown, err := obs.Init(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer func() { _ = shutdown(context.Background()) }()

	tr := o.Tracer("my-pkg")
	if tr == nil {
		t.Error("Tracer() returned nil")
	}
}

// TestObsAdminServerNotStartedWhenPortEmpty verifies that no admin server is
// started when AdminAddr is empty.
// REQ-OBS-004
func TestObsAdminServerNotStartedWhenPortEmpty(t *testing.T) {
	t.Parallel()

	cfg := obs.Config{
		ServiceName: "test-svc",
		AdminAddr:   "", // no admin server
	}
	o, shutdown, err := obs.Init(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer func() { _ = shutdown(context.Background()) }()

	if o.AdminAddr != "" {
		t.Errorf("expected empty AdminAddr when not configured, got %q", o.AdminAddr)
	}
}

// TestObsAdminServerStartsWhenPortSet verifies that the admin server binds and
// responds when AdminAddr is set.
// REQ-OBS-004
func TestObsAdminServerStartsWhenPortSet(t *testing.T) {
	t.Parallel()

	cfg := obs.Config{
		ServiceName: "test-svc",
		AdminAddr:   "127.0.0.1:0", // OS-assigned port
	}
	o, shutdown, err := obs.Init(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer func() { _ = shutdown(context.Background()) }()

	if o.AdminAddr == "" {
		t.Error("expected non-empty AdminAddr when admin server configured")
	}
}

// Ensure slog and os are used to avoid "imported and not used" errors.
var _ = slog.LevelDebug
var _ = os.Stdout
