---
id: SPEC-OBS-001
title: Observability Baseline
version: 0.1.0
milestone: M1 — Foundation
status: implemented
priority: P0
owner: expert-performance
methodology: tdd
coverage_target: 85
created: 2026-04-24
updated: 2026-04-26
approved_by: limbowl
approved_at: 2026-04-24
depends_on: [SPEC-BOOT-001]
blocks: [SPEC-SYN-004, SPEC-EVAL-002]
---

# SPEC-OBS-001: Observability Baseline

## 1. Purpose

SPEC-BOOT-001 established the `internal/obs/` package as an empty stub and
reserved it for the full observability implementation. SPEC-DEP-001 reserved
`github.com/prometheus/client_golang` as a future Go dependency pinned to
this SPEC. SPEC-OBS-001 fills that reservation and delivers the **observability
baseline** for every current and future Universal Search process:

- **Structured logging** via `log/slog` (stdlib), JSON output, level from env.
- **Request-ID propagation** via `context.Context`, wired through HTTP
  ingress and egress middleware, embedded in every log record and trace
  span.
- **Metrics** via `github.com/prometheus/client_golang`, exposed on a
  dedicated admin port at `/metrics` in Prometheus scrape-compatible text
  format, with bounded cardinality discipline.
- **Distributed tracing** via OpenTelemetry Go SDK, OTLP gRPC exporter,
  W3C TraceContext propagator, sampling-controlled, no-op when
  `OTLP_ENDPOINT` is unset.
- **Public API in `internal/obs/`** (Logger, Tracer, named metrics, request-ID
  helpers) consumed uniformly by every M2+ domain package without direct
  imports of Prometheus or OTel libraries.
- **Optional Loki forwarding seam** (REQ-OBS-007, stretch) — env var is
  reserved; actual sink is implemented via Promtail sidecar in a later
  deploy SPEC or added as a custom handler when needed.

Completion unblocks SPEC-SYN-004 (streaming synthesis — needs span
propagation across goroutines) and SPEC-EVAL-002 (adapter reliability
dashboard — needs per-adapter metrics). Every M2+ package (`internal/router`,
`internal/fanout`, `internal/adapters/*`, etc.) builds on this foundation;
without it, there is no shared convention for logging, tracing, or metrics,
and each package would roll its own.

## 2. Scope

### 2.1 In-Scope

| Ref | Item |
|-----|------|
| a | `internal/obs/` package layout: `log/`, `metrics/`, `trace/`, `reqid/` subpackages with top-level `obs.go` entrypoint |
| b | slog JSON handler with level from `LOG_LEVEL` env, context-aware enrichment (request_id, trace_id, span_id auto-injection) |
| c | ULID-based request ID generator, `WithRequestID(ctx, id)`/`RequestID(ctx)` context helpers |
| d | HTTP ingress middleware: read or generate `X-Request-ID`, bind to context |
| e | HTTP egress (`http.RoundTripper` wrapper): write `X-Request-ID` + W3C `traceparent` on outbound calls |
| f | Prometheus metric registry (singleton) and named collectors: `usearch_http_requests_total`, `usearch_http_request_duration_seconds`, `usearch_fanout_goroutines_inflight`, `usearch_adapter_calls_total`, `usearch_adapter_call_duration_seconds`, `usearch_build_info` |
| g | Admin HTTP server exposing `/metrics` via `promhttp.Handler()` on configurable port (default 9090) |
| h | OTel TracerProvider init with OTLP gRPC exporter, composite W3C TraceContext + Baggage propagator, `ParentBased(TraceIDRatioBased(0.1))` sampler |
| i | No-op tracer provider when `OTLP_ENDPOINT` unset (zero external traffic by default) |
| j | Configurable `Init(ctx, cfg) (shutdown, err)` function wiring everything; used by `cmd/usearch`, `cmd/usearch-api`, `cmd/usearch-mcp` main functions |
| k | `LOKI_ENDPOINT` env var reserved and documented; no-op stub handler if set, real handler deferred |
| l | Compose delta: add `prometheus` service + `deploy/prometheus/prometheus.yml` scrape config |
| m | `.env.example` additions: `LOG_LEVEL`, `USEARCH_ADMIN_PORT`, `OTLP_ENDPOINT`, `OTLP_SAMPLE_RATIO`, `LOKI_ENDPOINT`, `PROMETHEUS_PORT` |

### 2.2 Out-of-Scope

- Grafana dashboards, Grafana provisioning — belongs to SPEC-EVAL-002 (M8)
- Loki service in docker-compose or Promtail sidecar — deferred to deploy SPEC
- OTel Collector service in docker-compose — developer runs optionally, not shipped by default
- Alerting rules, Alertmanager configuration — post-V1 SPEC
- SLO definitions, burn-rate alerts — post-V1 SPEC
- OTel metrics API / Prometheus bridge exporter (`go.opentelemetry.io/otel/exporters/prometheus`) — explicit architectural non-goal; metrics stay on client_golang
- Python services' observability (`services/researcher`, `services/storm`, `services/embedder`) — each Python service emits its own JSON logs; cross-service trace propagation works via W3C TraceContext headers but Python-side SDK wiring is owned by those services' SPECs
- Web UI RUM (Real User Monitoring) — belongs to a future Web UI SPEC
- Log retention, index rotation, cost control on storage backends — infrastructure concerns

## 3. EARS Requirements

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| REQ-OBS-001 | Ubiquitous | The `internal/obs/log` package SHALL provide a slog JSON handler whose minimum level is read at `Init` time from the `LOG_LEVEL` environment variable (valid values: `DEBUG`, `INFO`, `WARN`, `ERROR`; default `INFO`); every log record SHALL be emitted to stderr as a single line of valid JSON. | P0 | `TestLoggerEmitsJSON` verifies JSON output shape; `TestLevelFromEnv` verifies level binding for each valid value plus the default. |
| REQ-OBS-002 | Ubiquitous | Every HTTP or gRPC request entering a Universal Search process SHALL carry an `X-Request-ID` value (generated as a ULID if the inbound request omits the header), bound to `context.Context` via `obs.WithRequestID`, propagated on all downstream HTTP egress calls via the wrapped `http.RoundTripper`, and automatically included as a `request_id` attribute on every slog record produced in that request's context. | P0 | `TestRequestIDPropagatesThroughContext` verifies ctx round-trip; `TestIngressGeneratesWhenAbsent` verifies header auto-generation; `TestIngressPreservesWhenPresent` verifies passthrough; `TestSlogRecordIncludesRequestID` verifies attribute injection; `TestEgressWritesHeader` verifies RoundTripper emits `X-Request-ID`. |
| REQ-OBS-003 | Event-Driven | WHEN an HTTP request starts or ends, a fanout goroutine spawns or completes, or an adapter call resolves, the corresponding Prometheus counter (`usearch_http_requests_total`, `usearch_adapter_calls_total`), histogram (`usearch_http_request_duration_seconds`, `usearch_adapter_call_duration_seconds`), or gauge (`usearch_fanout_goroutines_inflight`) SHALL be updated via the `internal/obs/metrics` package-exposed collectors. | P0 | `TestHTTPMiddlewareIncrementsCounter` and `TestHTTPMiddlewareRecordsDuration` verify HTTP path; `TestFanoutGaugeInflight` verifies goroutine counting via Inc/Dec; `TestAdapterCallOutcomeLabels` verifies bounded label set `{success, failure, timeout}`. |
| REQ-OBS-004 | Ubiquitous | An admin HTTP server SHALL expose `/metrics` in Prometheus scrape-compatible text-exposition format on a configurable internal port (default 9090, override via `USEARCH_ADMIN_PORT`), bound only to the same interface as the main process listener; the response SHALL be HTTP 200 and include at minimum the six registered metric families plus `usearch_build_info`. | P0 | `TestMetricsEndpointExposes200` verifies status + Content-Type `text/plain; version=0.0.4`; `TestMetricsIncludesAllFamilies` verifies every named metric family is present in the response body. |
| REQ-OBS-005 | Event-Driven | WHEN `OTLP_ENDPOINT` is set to a non-empty value at `Init` time, the `internal/obs/trace` package SHALL initialize an OpenTelemetry `TracerProvider` with an OTLP gRPC exporter targeting that endpoint, install a composite W3C TraceContext + Baggage propagator via `otel.SetTextMapPropagator`, apply `ParentBased(TraceIDRatioBased(OTLP_SAMPLE_RATIO))` sampling (default ratio 0.1), and return a shutdown closure that flushes pending spans on process exit. | P0 | `TestOTLPInitializesWhenEndpointSet` verifies non-nil TracerProvider + propagator registration; `TestOTLPNoOpWhenEndpointUnset` verifies no-op provider and zero outbound bytes; `TestSampleRatioFromEnv` verifies ratio binding; `TestShutdownFlushesSpans` verifies batcher flush. |
| REQ-OBS-006 | Ubiquitous | The `internal/obs` package SHALL expose a stable public API consisting of `Init(ctx, Config) (shutdown, error)`, `Logger(ctx) *slog.Logger`, `Tracer(name string) trace.Tracer`, `WithRequestID(ctx, id) context.Context`, `RequestID(ctx) string`, and the six named metric collectors as exported package-level variables — all consumed by `cmd/usearch*` mains and every M2+ domain package without direct import of `github.com/prometheus/client_golang` or `go.opentelemetry.io/otel` outside `internal/obs/`. | P0 | `TestPublicAPISurface` verifies symbol presence via `go/types` reflection on the package; `TestNoDirectPrometheusImportOutsideObs` and `TestNoDirectOtelImportOutsideObs` verify import boundaries via `go list -deps`. |
| REQ-OBS-007 | Event-Driven / Optional | WHEN `LOKI_ENDPOINT` is set to a non-empty value at `Init` time, the `internal/obs/log` package MAY forward a copy of each slog record to a Loki-compatible sink; for V1, this requirement is satisfied by reserving the env var, logging an INFO message at `Init` announcing the seam, and exposing a registration hook (`log.RegisterHandler(slog.Handler)`) — actual Loki transport is a stretch goal deferred to a subsequent deploy SPEC. | P2 | `TestLokiEnvReserved` verifies the env var is read at Init without error; `TestRegisterHandlerHookSeam` verifies the hook accepts a custom slog.Handler and subsequent records are tee'd to it. |

## 4. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| NFR-OBS-001 | Performance | Instrumentation overhead of enabled slog + metrics middleware + no-op tracer (default `OTLP_ENDPOINT` unset) SHALL add less than 5% to p99 latency on a synthetic `go test -bench` benchmark against a stub HTTP handler that returns a fixed 1KB JSON body at 1000 concurrent requests. When `OTLP_ENDPOINT` is set with sample ratio 0.1, overhead SHALL stay under 8% p99 (sampling amortizes exporter cost). Benchmark lives at `internal/obs/bench/bench_test.go` and runs in CI. |
| NFR-OBS-002 | Cardinality Safety | No Prometheus label MAY carry per-request-ID, per-user-ID, per-URL (raw path), per-query-text, or any other unbounded value. Label value sets are enumerable at startup: `method` ∈ HTTP method enum; `route` ∈ route-template registry (known set at compile time); `status_class` ∈ {2xx, 3xx, 4xx, 5xx}; `adapter` ∈ bounded registry (≤20 values at V1); `outcome` ∈ {success, failure, timeout}; `adapter_class` ∈ {web, social, academic, korean, mixed}. Per-request trace linking uses Prometheus exemplars or OTel span attributes, not labels. |
| NFR-OBS-003 | Reproducibility | All observability runtime dependencies SHALL be pinned to exact versions in `go.mod` (new direct deps): `github.com/prometheus/client_golang v1.20.x`, `go.opentelemetry.io/otel v1.30.x`, `go.opentelemetry.io/otel/sdk v1.30.x`, `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.30.x`, `github.com/oklog/ulid/v2 v2.1.x`. Exact patch versions are selected at run-phase pinning; `go.sum` captures transitive checksums. The Prometheus scrape-target entry in `deploy/prometheus/prometheus.yml` SHALL be static (no service discovery) for dev reproducibility. |

## 5. Acceptance Criteria

### REQ-OBS-001 — Structured Logging

- `internal/obs/log/log.go` constructs a `*slog.Logger` whose handler is
  `slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level, AddSource: false})`.
- Level parser maps `LOG_LEVEL` env to `slog.LevelDebug/Info/Warn/Error`;
  invalid or missing value defaults to `slog.LevelInfo`.
- Test asserts `json.Unmarshal` of a captured line succeeds and yields a
  map containing `time`, `level`, `msg` (stdlib slog JSON keys).
- Test captures output to a buffered handler, logs one record at each level,
  asserts that records below the configured level are omitted.
- The returned `*slog.Logger` is the default exposed via `obs.Logger(ctx)`.

### REQ-OBS-002 — Request ID Propagation

- Generator: `github.com/oklog/ulid/v2`, monotonic-entropy source, 26-char
  Crockford Base32 string.
- Context helpers: unexported key type, `WithRequestID(ctx, id)` and
  `RequestID(ctx) string` (empty string if unset).
- HTTP ingress middleware `reqid.Middleware(next http.Handler) http.Handler`:
  - Reads `X-Request-ID` from incoming request.
  - If absent or empty, generates a new ULID.
  - Calls `WithRequestID(r.Context(), id)` and passes to `next`.
  - Writes the final ID to response `X-Request-ID` header.
- HTTP egress `reqid.Transport(next http.RoundTripper) http.RoundTripper`:
  - On `RoundTrip(req)`, reads ID from `req.Context()`; if non-empty,
    sets `req.Header["X-Request-ID"] = [id]`.
- Slog enrichment handler: wraps an inner handler; on each `Handle(ctx, r)`,
  reads `RequestID(ctx)` and appends as `slog.String("request_id", id)` attr
  when non-empty. Similarly reads current OTel span from ctx and appends
  `trace_id` + `span_id` when a recording span exists.
- Tests:
  - `TestIngressGeneratesWhenAbsent`: request without header → response
    header has valid ULID.
  - `TestIngressPreservesWhenPresent`: inbound ID `REQ-FIXED-123` →
    response header equals `REQ-FIXED-123`.
  - `TestEgressWritesHeader`: context with ID `EGRESS-42` + `http.Client`
    using wrapped transport against a stub upstream → upstream received
    `X-Request-ID: EGRESS-42`.
  - `TestSlogRecordIncludesRequestID`: log with enriched ctx → captured JSON
    contains `"request_id":"..."`.
  - `TestRequestIDPropagatesThroughContext`: chained goroutine via
    `go func() { ... obs.RequestID(ctx) ... }()` sees the same ID.

### REQ-OBS-003 — Metric Event Instrumentation

- `metrics.HTTPRequests` is `*prometheus.CounterVec` with labels
  `[]string{"method","route","status_class"}`.
- `metrics.HTTPRequestDuration` is `*prometheus.HistogramVec` with labels
  `[]string{"method","route"}` and `prometheus.DefBuckets`.
- `metrics.FanoutInflight` is `*prometheus.GaugeVec` with labels
  `[]string{"adapter_class"}`.
- `metrics.AdapterCalls` is `*prometheus.CounterVec` with labels
  `[]string{"adapter","outcome"}`.
- `metrics.AdapterCallDuration` is `*prometheus.HistogramVec` with labels
  `[]string{"adapter"}` and buckets `[0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30]` (seconds).
- `metrics.BuildInfo` is `*prometheus.GaugeVec` with labels
  `[]string{"version","commit","go_version"}`; always set to 1 at Init.
- HTTP middleware records both counter + histogram + exemplar (trace_id from
  OTel span if present).
- Tests:
  - `TestHTTPMiddlewareIncrementsCounter`: 3 stub calls → counter value 3.
  - `TestHTTPMiddlewareRecordsDuration`: duration observed within ±10%
    of measured elapsed.
  - `TestFanoutGaugeInflight`: `Inc`/`Dec` pair returns gauge to baseline.
  - `TestAdapterCallOutcomeLabels`: only `success/failure/timeout` are
    accepted; any other value triggers a panic at registration time
    (via a test-side validator that scans recorded label values).

### REQ-OBS-004 — /metrics HTTP Endpoint

- `metrics.StartAdminServer(ctx, addr string) (shutdown func(context.Context) error)`
  returns a shutdown closure; the server mounts only `/metrics` plus a
  `/healthz` liveness endpoint (returns 200 OK).
- Handler uses `promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg})`
  with a non-default registry (enables test isolation).
- Tests:
  - `TestMetricsEndpointExposes200`: GET `/metrics` → status 200,
    `Content-Type` starts with `text/plain`, body is non-empty.
  - `TestMetricsIncludesAllFamilies`: body contains every declared metric
    name via substring assertion.
  - `TestAdminPortConfigurable`: Init with `USEARCH_ADMIN_PORT=19090` →
    listener bound to 19090.

### REQ-OBS-005 — OTel Initialization

- `trace.Init(ctx, cfg) (shutdown, error)`:
  - If `cfg.OTLPEndpoint == ""` → set `noop.NewTracerProvider()` as global,
    return trivial shutdown.
  - Else → `otlptracegrpc.New(ctx, WithEndpoint(cfg.OTLPEndpoint),
    WithInsecure())`, wrap in `sdktrace.BatchSpanProcessor`, install via
    `sdktrace.NewTracerProvider(WithBatcher(...), WithResource(...),
    WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))))`.
  - Install composite propagator `propagation.NewCompositeTextMapPropagator(
    TraceContext{}, Baggage{})` via `otel.SetTextMapPropagator`.
- `trace.Tracer(name string) trace.Tracer` returns `otel.Tracer(name)`.
- Tests:
  - `TestOTLPInitializesWhenEndpointSet`: set endpoint → global provider
    is not noop; propagator is composite.
  - `TestOTLPNoOpWhenEndpointUnset`: unset → provider is noop; outbound
    bytes captured via test-side gRPC stub observer = 0.
  - `TestSampleRatioFromEnv`: `OTLP_SAMPLE_RATIO=0.5` → sampler sampled
    approximately 50% on 10000 synthetic traces (±5% tolerance).
  - `TestShutdownFlushesSpans`: start span, end span, call shutdown →
    exporter observes the span (via in-memory exporter fixture from
    `sdktrace/tracetest`).

### REQ-OBS-006 — Public API Surface

- Exported symbols of `internal/obs` (verified via `go doc` or AST scan):
  `Init`, `Config`, `Logger`, `Tracer`, `WithRequestID`, `RequestID`,
  `HTTPRequests`, `HTTPRequestDuration`, `FanoutInflight`, `AdapterCalls`,
  `AdapterCallDuration`, `BuildInfo`.
- Import discipline:
  - `go list -deps ./...` → `github.com/prometheus/client_golang/...`
    appears ONLY under import paths beginning with
    `github.com/elymas/universal-search/internal/obs`.
  - Same for `go.opentelemetry.io/otel/...`.
- Tests:
  - `TestPublicAPISurface`: at compile time, assert each named symbol
    exists with expected kind (func vs var) via `go/types.Lookup`.
  - `TestNoDirectPrometheusImportOutsideObs`: walk `go list -deps -json`
    output; any non-obs package depending on `prometheus/client_golang`
    fails the test.
  - `TestNoDirectOtelImportOutsideObs`: analogous for OTel packages.
- Note: at the time SPEC-OBS-001 runs, M2+ packages do not yet exist, so
  the import-boundary tests pass trivially. The tests remain in place to
  guard subsequent SPECs.

### REQ-OBS-007 — Loki Seam (Stretch / P2)

- `Init` reads `LOKI_ENDPOINT`; if non-empty, logs at INFO level:
  `"loki endpoint reserved; transport not yet implemented"
  seam=log.RegisterHandler`.
- `log.RegisterHandler(h slog.Handler)` registers a tee handler; subsequent
  records emitted via `obs.Logger(ctx)` are forwarded to `h` in addition
  to the primary JSON handler.
- Tests:
  - `TestLokiEnvReserved`: Init with `LOKI_ENDPOINT=http://loki:3100` →
    no error; captured boot log contains the seam announcement.
  - `TestRegisterHandlerHookSeam`: register a buffered test handler; log
    3 records; buffer contains exactly 3 records matching the primary
    output's parsed fields.

### NFR-OBS-001 — Performance Budget

- `internal/obs/bench/bench_test.go` defines `BenchmarkHTTPStubBaseline`
  (no middleware) and `BenchmarkHTTPStubInstrumented` (full stack,
  OTLP unset).
- Acceptance: p99 overhead (`(Instrumented - Baseline) / Baseline`) < 0.05
  over 1000 concurrent clients × 100 iterations each.
- `BenchmarkHTTPStubTracedSampled` runs with `OTLP_ENDPOINT=127.0.0.1:0`
  (null gRPC sink) and sample ratio 0.1; overhead < 0.08.
- Benchmarks run in CI via `.github/workflows/go.yml` bench job on
  scheduled weekly runs (not per-PR to keep CI fast).

### NFR-OBS-002 — Cardinality Safety

- `TestNoUnboundedLabels` is a static-analysis test that:
  - Walks all `*prometheus.{Counter,Gauge,Histogram,Summary}Vec` registrations
    in `internal/obs/metrics/`.
  - For each label name, asserts the label appears in an allowlist map
    defined in the test file.
  - Fails if any new label is introduced without being added to the
    allowlist (forcing reviewer attention on cardinality).
- Allowlist: `{method, route, status_class, adapter_class, adapter,
  outcome, version, commit, go_version}`.

### NFR-OBS-003 — Reproducibility

- `go.mod` pins each new direct dep to an exact minor version; patch
  updates handled by Renovate (per SPEC-DEP-001 REQ-DEP-006).
- `docs/dependencies.md` (from SPEC-DEP-001 REQ-DEP-007) includes the new
  Go deps in the next manifest regeneration — verified by
  `scripts/gen-deps-manifest.sh` idempotency check.
- `deploy/prometheus/prometheus.yml` uses static_configs only (no DNS,
  no file_sd); scrape targets are known at deploy time.

## 6. Technical Approach

### 6.1 Package Layout (to be created by run phase)

```
internal/obs/
├── obs.go                # top-level Init, Config, re-exports
├── log/
│   ├── log.go            # slog handler construction, level parsing, Default()
│   ├── enrich.go         # handler that injects request_id/trace_id/span_id from ctx
│   ├── register.go       # RegisterHandler hook (REQ-OBS-007 seam)
│   └── log_test.go
├── metrics/
│   ├── metrics.go        # registry, named Counter/Gauge/Histogram vars
│   ├── http.go           # HTTP middleware (counter + histogram + exemplar)
│   ├── server.go         # StartAdminServer, /metrics + /healthz mux
│   └── metrics_test.go
├── trace/
│   ├── trace.go          # Init, shutdown, Tracer()
│   ├── attrs.go          # SemConv helpers
│   └── trace_test.go
├── reqid/
│   ├── reqid.go          # ULID gen, WithRequestID, FromContext
│   ├── http.go           # ingress Middleware
│   ├── client.go         # egress Transport (RoundTripper wrapper)
│   └── reqid_test.go
└── bench/
    └── bench_test.go     # NFR-OBS-001 benchmarks
```

### 6.2 Top-Level API Sketch (`internal/obs/obs.go`)

```go
// Package obs provides the observability baseline for Universal Search:
// structured logging (slog), request-ID propagation, Prometheus metrics,
// and OpenTelemetry tracing. All Universal Search processes initialize
// this package in main() via Init(ctx, cfg) and consume its exported
// symbols for telemetry. Domain packages MUST NOT import prometheus or
// otel libraries directly; they consume obs exclusively.
package obs

import (
    "context"
    "log/slog"

    "go.opentelemetry.io/otel/trace"

    "github.com/elymas/universal-search/internal/obs/log"
    "github.com/elymas/universal-search/internal/obs/metrics"
    tracepkg "github.com/elymas/universal-search/internal/obs/trace"
    "github.com/elymas/universal-search/internal/obs/reqid"
)

type Config struct {
    ServiceName    string
    ServiceVersion string
    GitCommit      string
    LogLevel       string // overrides LOG_LEVEL env
    AdminAddr      string // default ":9090"
    OTLPEndpoint   string // overrides OTLP_ENDPOINT env
    OTLPSampleRate float64
    LokiEndpoint   string
}

func Init(ctx context.Context, cfg Config) (shutdown func(context.Context) error, err error) {
    // 1. log (consumes cfg.LogLevel + LOG_LEVEL env fallback)
    // 2. metrics (register collectors, start admin server)
    // 3. trace (OTLP init or no-op)
    // 4. reqid (no init, stateless)
    // 5. Return composed shutdown that stops admin server + flushes traces.
}

// Re-exports for consumers.
func Logger(ctx context.Context) *slog.Logger { return log.FromContext(ctx) }
func Tracer(name string) trace.Tracer          { return tracepkg.Tracer(name) }
func WithRequestID(ctx context.Context, id string) context.Context {
    return reqid.WithContext(ctx, id)
}
func RequestID(ctx context.Context) string { return reqid.FromContext(ctx) }

// Named collector re-exports.
var (
    HTTPRequests         = metrics.HTTPRequests
    HTTPRequestDuration  = metrics.HTTPRequestDuration
    FanoutInflight       = metrics.FanoutInflight
    AdapterCalls         = metrics.AdapterCalls
    AdapterCallDuration  = metrics.AdapterCallDuration
    BuildInfo            = metrics.BuildInfo
)
```

### 6.3 Main Integration Sketch

Each `cmd/usearch*/main.go` will gain an Init block:

```go
func main() {
    ctx := context.Background()
    shutdown, err := obs.Init(ctx, obs.Config{
        ServiceName:    "usearch-api",
        ServiceVersion: version, // build-time ldflag
        GitCommit:      commit,  // build-time ldflag
    })
    if err != nil {
        // Log to stderr with plain fmt (obs not initialized); exit 1.
    }
    defer func() {
        shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        _ = shutdown(shutdownCtx)
    }()
    // ... rest of main
}
```

### 6.4 Compose Delta

New file `deploy/prometheus/prometheus.yml`:

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s
scrape_configs:
  - job_name: 'usearch-api'
    static_configs:
      - targets: ['host.docker.internal:9090']
        labels: { service: 'usearch-api' }
  - job_name: 'usearch-mcp'
    static_configs:
      - targets: ['host.docker.internal:9092']
        labels: { service: 'usearch-mcp' }
```

Additions to `deploy/docker-compose.yml`:

```yaml
services:
  prometheus:
    image: prom/prometheus:v2.54.1
    ports:
      - "${PROMETHEUS_PORT:-9091}:9090"
    volumes:
      - ./prometheus/prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - prometheus_data:/prometheus
    healthcheck:
      test: ["CMD-SHELL", "wget -qO- http://localhost:9090/-/ready || exit 1"]
      interval: 10s
      timeout: 5s
      retries: 5
      start_period: 30s
    restart: unless-stopped
    networks:
      - app

volumes:
  prometheus_data: {}
```

Additions to `.env.example` (root):

```
# Observability (SPEC-OBS-001)
LOG_LEVEL=INFO
USEARCH_ADMIN_PORT=9090
OTLP_ENDPOINT=
OTLP_SAMPLE_RATIO=0.1
LOKI_ENDPOINT=
PROMETHEUS_PORT=9091
```

### 6.5 go.mod Impact

New direct dependencies (pinned at run-phase, exact versions captured here):

```
github.com/prometheus/client_golang v1.20.x
github.com/prometheus/client_model v0.6.x          // transitive but surfaces in types; pinned directly
github.com/prometheus/common v0.55.x               // transitive
go.opentelemetry.io/otel v1.30.x
go.opentelemetry.io/otel/sdk v1.30.x
go.opentelemetry.io/otel/trace v1.30.x
go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.30.x
go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.30.x
google.golang.org/grpc v1.66.x                     // required by OTLP gRPC exporter
github.com/oklog/ulid/v2 v2.1.x
```

Exact patch versions are determined at run-phase by running `go get` and
locking via `go mod tidy`. Per NFR-OBS-003, the run phase MUST update
`go.sum` and ensure `docs/dependencies.md` regeneration picks up the
entries.

### 6.6 Harness Level

Per `.moai/config/sections/harness.yaml` auto-routing logic, this SPEC
has 7 REQs + 3 NFRs touching 4 sub-packages + compose + env files =
**standard** harness level recommended. Sprint Contract (design.yaml
§11) is optional but recommended; Evaluator profile `default` applies.

## 7. File Impact

### 7.1 Created

| Path | Purpose |
|------|---------|
| `internal/obs/obs.go` | Replace stub; top-level Init, Config, re-exports (REQ-OBS-006) |
| `internal/obs/log/log.go` | slog handler factory + Default + FromContext (REQ-OBS-001) |
| `internal/obs/log/enrich.go` | Context-enrichment handler (REQ-OBS-002) |
| `internal/obs/log/register.go` | RegisterHandler seam (REQ-OBS-007) |
| `internal/obs/log/log_test.go` | RED tests for REQ-OBS-001, REQ-OBS-007 |
| `internal/obs/metrics/metrics.go` | Registry + named collectors (REQ-OBS-003, NFR-OBS-002) |
| `internal/obs/metrics/http.go` | HTTP middleware (REQ-OBS-003) |
| `internal/obs/metrics/server.go` | Admin server + /metrics handler (REQ-OBS-004) |
| `internal/obs/metrics/metrics_test.go` | RED tests for REQ-OBS-003, REQ-OBS-004, NFR-OBS-002 |
| `internal/obs/trace/trace.go` | OTel Init + shutdown + Tracer (REQ-OBS-005) |
| `internal/obs/trace/attrs.go` | SemConv attribute helpers |
| `internal/obs/trace/trace_test.go` | RED tests for REQ-OBS-005 |
| `internal/obs/reqid/reqid.go` | ULID gen + ctx helpers (REQ-OBS-002) |
| `internal/obs/reqid/http.go` | Ingress middleware (REQ-OBS-002) |
| `internal/obs/reqid/client.go` | Egress RoundTripper (REQ-OBS-002) |
| `internal/obs/reqid/reqid_test.go` | RED tests for REQ-OBS-002 |
| `internal/obs/bench/bench_test.go` | NFR-OBS-001 benchmarks |
| `deploy/prometheus/prometheus.yml` | Static scrape config |

### 7.2 Modified

| Path | Change |
|------|--------|
| `internal/obs/obs.go` | Replace 2-line stub with full Init + API (see §6.2) |
| `deploy/docker-compose.yml` | Add `prometheus` service + `prometheus_data` volume |
| `.env.example` | Append 6 new observability vars (§6.4) |
| `go.mod` / `go.sum` | Add direct deps per §6.5 |
| `cmd/usearch/main.go` | Insert `obs.Init` + deferred shutdown |
| `cmd/usearch-api/main.go` | Same (stub from SPEC-BOOT-001, now wires obs) |
| `cmd/usearch-mcp/main.go` | Same |
| `.github/workflows/go.yml` | Add scheduled weekly bench job for NFR-OBS-001 |
| `docs/dependencies.md` | Regenerated via `scripts/gen-deps-manifest.sh` (covers new deps) |

### 7.3 Unchanged (by design)

- `internal/router/`, `internal/fanout/`, `internal/adapters/*`,
  `internal/index/*`, `internal/llm/`, `internal/synthesis/`,
  `internal/auth/`, `internal/eval/` — all remain stubs; they begin
  consuming `obs.Logger`/`obs.Tracer`/named metrics in their own SPECs.

## 8. TDD Plan

Development follows RED-GREEN-REFACTOR per `quality.development_mode: tdd`.
Representative RED-phase tests, written before implementation, grouped by REQ:

| Test | Layer | REQ | Assertion |
|------|-------|-----|-----------|
| `TestLoggerEmitsJSON` | `log/log_test.go` | REQ-OBS-001 | Captured handler output parses as JSON and contains `time`, `level`, `msg` keys |
| `TestLevelFromEnv` | `log/log_test.go` | REQ-OBS-001 | Table-driven over `{"","DEBUG","INFO","WARN","ERROR","garbage"}`; asserts correct `slog.Level` or default |
| `TestLogBelowLevelSuppressed` | `log/log_test.go` | REQ-OBS-001 | Level=WARN → DEBUG/INFO records not emitted |
| `TestRequestIDPropagatesThroughContext` | `reqid/reqid_test.go` | REQ-OBS-002 | `WithRequestID(ctx, "X")` → `RequestID(derived ctx in goroutine)` returns `"X"` |
| `TestIngressGeneratesWhenAbsent` | `reqid/reqid_test.go` | REQ-OBS-002 | Inbound request sans header → response header is 26-char ULID (regex `^[0-9A-HJKMNP-TV-Z]{26}$`) |
| `TestIngressPreservesWhenPresent` | `reqid/reqid_test.go` | REQ-OBS-002 | Inbound `X-Request-ID: FIXED-123` → response same value |
| `TestEgressWritesHeader` | `reqid/reqid_test.go` | REQ-OBS-002 | `http.Client{Transport: obs.reqid.Transport(http.DefaultTransport)}` + ctx with ID → stub upstream receives header |
| `TestSlogRecordIncludesRequestID` | `log/log_test.go` | REQ-OBS-002 | Logger with enrich handler + ctx w/ ID → captured JSON has `request_id` field |
| `TestHTTPMiddlewareIncrementsCounter` | `metrics/metrics_test.go` | REQ-OBS-003 | 3 stub requests → `HTTPRequests.WithLabelValues("GET","/x","2xx")` counter == 3 |
| `TestHTTPMiddlewareRecordsDuration` | `metrics/metrics_test.go` | REQ-OBS-003 | Histogram bucket sum > 0; observed value within measurement tolerance |
| `TestFanoutGaugeInflight` | `metrics/metrics_test.go` | REQ-OBS-003 | `Inc()` then `Dec()` returns gauge to 0 |
| `TestAdapterCallOutcomeLabels` | `metrics/metrics_test.go` | REQ-OBS-003 | Labels outside `{success, failure, timeout}` trigger the static-analysis allowlist failure |
| `TestMetricsEndpointExposes200` | `metrics/metrics_test.go` | REQ-OBS-004 | GET `/metrics` → 200; `Content-Type` begins with `text/plain` |
| `TestMetricsIncludesAllFamilies` | `metrics/metrics_test.go` | REQ-OBS-004 | Response body contains each of the 6 metric family names |
| `TestAdminPortConfigurable` | `metrics/metrics_test.go` | REQ-OBS-004 | Init with `:0` (random) → listener address port > 0 |
| `TestOTLPInitializesWhenEndpointSet` | `trace/trace_test.go` | REQ-OBS-005 | Endpoint set → global provider type != noop; propagator is composite TraceContext+Baggage |
| `TestOTLPNoOpWhenEndpointUnset` | `trace/trace_test.go` | REQ-OBS-005 | Endpoint empty → global provider is noop; no exporter instantiated |
| `TestSampleRatioFromEnv` | `trace/trace_test.go` | REQ-OBS-005 | Ratio 0.5 over 10000 traces → sampled count in [4500, 5500] |
| `TestShutdownFlushesSpans` | `trace/trace_test.go` | REQ-OBS-005 | In-memory exporter (`sdktrace/tracetest.NewInMemoryExporter`) captures end-to-end span after shutdown |
| `TestPublicAPISurface` | `obs_test.go` (package-level) | REQ-OBS-006 | `go/types` lookup confirms each named symbol exists and has expected kind |
| `TestNoDirectPrometheusImportOutsideObs` | `obs_test.go` | REQ-OBS-006 | Walks `go list -deps -json`; asserts consumer packages of `prometheus/client_golang` are all under `internal/obs/` |
| `TestNoDirectOtelImportOutsideObs` | `obs_test.go` | REQ-OBS-006 | Analogous for OTel |
| `TestLokiEnvReserved` | `log/log_test.go` | REQ-OBS-007 | Init with `LOKI_ENDPOINT=http://loki:3100` → no error; captured INFO log announces seam |
| `TestRegisterHandlerHookSeam` | `log/log_test.go` | REQ-OBS-007 | Custom buffered handler registered → N log calls → buffer has N records |
| `TestNoUnboundedLabels` | `metrics/metrics_test.go` | NFR-OBS-002 | Static scan of registrations; label names outside allowlist fail |
| `BenchmarkHTTPStubBaseline` | `bench/bench_test.go` | NFR-OBS-001 | Baseline p99 latency |
| `BenchmarkHTTPStubInstrumented` | `bench/bench_test.go` | NFR-OBS-001 | Full-stack p99 overhead < 5% |
| `BenchmarkHTTPStubTracedSampled` | `bench/bench_test.go` | NFR-OBS-001 | Sampled tracing p99 overhead < 8% |

Coverage target: 85% per `quality.yaml:test_coverage_target`. Benchmarks
do not count toward coverage; they validate NFR-OBS-001 separately.

RED-GREEN-REFACTOR per requirement:

1. RED: Write failing test for REQ-OBS-N.
2. GREEN: Implement the minimal code to pass.
3. REFACTOR: Tidy, extract helpers if they remove duplication across REQs.

Brownfield note: `internal/obs/obs.go` exists as a 2-line stub. Per
workflow-modes.md §Brownfield Enhancement, RED tests for REQ-OBS-006 may
be written informed by the stub's package declaration; no characterization
tests are needed because the stub has no behavior to preserve.

## 9. Dependencies

### 9.1 Upstream SPEC Dependencies

- **SPEC-BOOT-001 (approved)**: provides `internal/obs/obs.go` stub,
  `cmd/usearch*` binaries, `deploy/docker-compose.yml` baseline.
  MUST be merged before SPEC-OBS-001 run phase begins.

### 9.2 Parallelizable

- **SPEC-DEP-001** (in draft): SPEC-OBS-001 adds new Go direct deps; the
  audit CI (SPEC-DEP-001 REQ-DEP-003) will run against the expanded
  `go.mod`. No blocking dependency in either direction — both can be
  under active development; whichever lands first informs the other's
  lockfile state. Per roadmap M1 parallelization table, SPEC-BOOT-001
  + SPEC-OBS-001 + SPEC-LLM-001 are explicitly 3-way parallelizable.

### 9.3 Downstream Blocked SPECs

- **SPEC-SYN-004 (M4 streaming synthesis)**: requires span context
  propagation across SSE streams.
- **SPEC-EVAL-002 (M8 adapter reliability dashboard)**: consumes
  `usearch_adapter_calls_total` and `usearch_adapter_call_duration_seconds`
  metrics.

All M2+ domain SPECs implicitly depend on SPEC-OBS-001 for logging/tracing
conventions, but only the two above are hard blocks (`blocks:` front-matter).

### 9.4 External Dependencies (run-phase pins)

New Go module dependencies (see §6.5 for pinning). None are optional;
all are required for REQ-OBS-003/004/005 implementation.

No external service dependencies at SPEC-OBS-001 runtime by default (OTLP
endpoint and Loki endpoint are both optional). For full-stack dev
validation, developer runs `docker compose up prometheus` after SPEC
implementation lands.

## 10. Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Cardinality explosion from new labels added in M2+ adapters | Medium | High (Prometheus storage blow-up, scrape timeouts) | `TestNoUnboundedLabels` allowlist enforced in CI; new labels require both test update and code review; documented in spec.md §NFR-OBS-002 |
| OTel Go SDK breaking changes between v1.30 and future v1.x | Low | Medium | OTel v1.x has stability guarantee for core API since v1.0 (2023); exporter module may shift; Renovate weekly bumps + integration tests catch drift |
| OTLP exporter connection failures block process startup | Medium | High (service down) | `trace.Init` MUST treat exporter connection as async — exporter construction succeeds even if gRPC connection not yet established; `BatchSpanProcessor` buffers and retries; failure logged, not panic |
| `/metrics` endpoint exposed publicly → info leak (goroutine counts, memory profile) | Medium | Medium | Default bind to admin port on same interface as main service (developer responsibility); production deployments (future deploy SPEC) bind admin port to localhost/mesh-internal only; documented in README run-phase addendum |
| Instrumentation overhead exceeds 5% budget | Low | Medium (silent perf regression) | Benchmark in CI (NFR-OBS-001); weekly scheduled run + regression gate; optimize hot path (pre-register label combinations via `WithLabelValues` caching) |
| Loki seam (REQ-OBS-007) adds complexity without being used | Medium | Low | V1 scope is intentionally minimal — env var reservation + hook only; no transport code, no Loki container in compose |
| ULID vs UUIDv7 choice (Open Question §11.1) causes ID format drift post-V1 | Low | Low | Public header is `X-Request-ID` — format-agnostic; switching generators is an internal refactor with no API break |
| Default 10% sampling hides low-frequency errors (sampling bias) | Medium | Medium | Add tail-sampling in future SPEC (OTel Collector `tail_sampling_processor`); V1 default 10% is a starting point; configurable via `OTLP_SAMPLE_RATIO` |
| `host.docker.internal` in `deploy/prometheus/prometheus.yml` does not resolve on Linux dev machines | Medium | Low | Documented in `.env.example` + compose comment; Linux users override via `--add-host` or edit scrape config; CI runs on GitHub Actions Ubuntu and uses `extra_hosts` mapping |

## 11. Open Questions

The following are explicitly unresolved and documented here rather than
pre-decided. They do not block SPEC approval.

1. **Request-ID generator: ULID vs UUIDv7**. Default choice is ULID
   (`github.com/oklog/ulid/v2`) for shorter 26-char header footprint.
   UUIDv7 (RFC 9562) is equally valid and slightly more interoperable
   with non-Go services. Decision deferred to run-phase implementer;
   format is encapsulated in `internal/obs/reqid/` and is an internal
   refactor post-V1. Resolution owner: run-phase implementer.

2. **Prometheus metric naming namespace**. Options:
   - **`usearch_*`** (proposed): brand-matched, clean separation from
     system metrics.
   - **`otel_*`** + bridge exporter: aligns with OTel conventions but
     requires the bridge we explicitly declined in §3.4.
   - **unprefixed** + `service` label: flatter hierarchy, harder to
     dashboard-filter.
   Default: `usearch_*`. Confirm at run phase; renaming post-V1 is a
   dashboard-breaking change.

3. **`/metrics` production authentication**. Dev: unauthenticated on
   localhost-bound admin port. Production: options are (a) reverse-proxy
   with basic auth, (b) mTLS with cert in scrape job, (c) Prometheus
   scrape from inside a mesh with no auth. Default: **no auth in
   SPEC-OBS-001**; production auth is a deploy SPEC concern.

4. **Default sampling ratio for traces**. 10% is a common starting
   point. Higher ratio gives better debugging fidelity at cost.
   Value is env-configurable; default can be revisited once we have
   real traffic volume data (post-M3 realistically).

5. **Exemplar sampling for metrics histograms**. Exemplars attach trace
   IDs to histogram observations (§2.4 research.md). Default: emit
   exemplar whenever an active OTel span is present in ctx.
   Alternative: sample exemplars independently to cap memory cost.
   Default is acceptable for V1 volumes; revisit if exemplar memory
   becomes measurable.

6. **`host.docker.internal` portability in `deploy/prometheus/prometheus.yml`**.
   Works on Docker Desktop (macOS, Windows) out of the box. Linux
   requires `extra_hosts: "host.docker.internal:host-gateway"` on the
   prometheus service. Default: include the `extra_hosts` entry in the
   compose delta so Linux + Docker Desktop both work. Resolution in
   run phase during compose delta implementation.

## 12. HISTORY

- 2026-04-26 — Implemented and merged in PR #3 (commit 0234b71). Coverage: obs 86.5% / log 89.6% / metrics 89.7% / reqid 95.2% / trace 90.5% (all ≥85% target). 18 @MX tags applied across 5 source files. Cardinality allowlist + AdapterCalls/AdapterCallDuration metric families now consumed by SPEC-CORE-001.

- 2026-04-24 (initial draft v0.1, limbowl via manager-spec):
  First EARS-formatted SPEC drafted after research phase. Scope derived
  from `.moai/project/roadmap.md` M1 row (owner: expert-performance,
  scope: slog→Loki optional, Prometheus metrics, OTel wiring, request-ID
  propagation). Built on SPEC-BOOT-001 (`internal/obs/obs.go` stub +
  `deploy/docker-compose.yml` six-service baseline) and coordinated
  with SPEC-DEP-001 (future-deps reservation for `prometheus/client_golang`).
  Research artifact at `.moai/specs/SPEC-OBS-001/research.md` captures
  stack rationale, reference implementations, and compose integration
  details. 7 EARS REQs (6 × P0 + 1 × P2), 3 NFRs, 28 representative
  RED tests, 6 Open Questions. Pinned Go module dep targets:
  client_golang v1.20.x, OTel v1.30.x core+sdk+otlptracegrpc, ulid/v2
  v2.1.x. Ready for plan-auditor review and annotation cycle.

---

*End of SPEC-OBS-001 v0.1*
