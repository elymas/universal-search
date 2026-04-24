# Research — SPEC-OBS-001 Observability Baseline

Author: limbowl (via manager-spec, plan phase)
Date: 2026-04-24
Status: Research complete, feeds into `.moai/specs/SPEC-OBS-001/spec.md`
Scope: Structured logging, request-ID propagation, Prometheus metrics, OpenTelemetry
tracing, optional Loki forwarding, Grafana deferral — for Go 1.23+ on Universal
Search monorepo (`github.com/elymas/universal-search`).

---

## 1. Go Observability Stack Options (2026)

The 2026 Go observability landscape offers three mature structured-logging options.
We compare against Universal Search's constraints: Go 1.23 baseline, minimal
external dependencies, context.Context-aware propagation, zero-alloc hot paths
during fanout.

| Library | Status 2026 | Context Support | External Deps | Notes |
|---------|-------------|-----------------|---------------|-------|
| `log/slog` (stdlib, Go 1.21+) | Stable, default | First-class via `LogAttrs(ctx, ...)` | Zero | JSON + text handlers built in |
| `github.com/rs/zerolog` | Maintained | Via `zerolog.Ctx(ctx)` | Single lib | Zero-alloc; pre-dates slog |
| `go.uber.org/zap` | Maintained | Via `zap.With(...)` | Single lib | Two APIs (Logger/SugaredLogger); more ceremony |

**Recommendation: `log/slog` (stdlib).**

Rationale:

- `tech.md` §3 already lists `slog (stdlib)` as the project's logging choice
  with rationale "structured JSON, zero external dep". SPEC must honor this.
- Stdlib `slog` supports JSON handler with configurable level via
  `slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: ...})`.
- `context.Context` integration: slog provides `slog.SetDefault(...)` plus
  helper patterns using `ctx` as attribute source. We will build a thin
  `internal/obs/log` wrapper that auto-extracts request ID and span ID from
  context and injects them as slog attributes on every record.
- Go 1.23 is the declared baseline (`go.mod`, `REQ-BOOT-001`); slog is
  available natively.
- slog handler composition (via `slog.Handler` interface) allows:
  - Primary JSON handler to stderr
  - Optional tee to Loki handler when `LOKI_ENDPOINT` is set
  - Optional attribute enrichment handler for request ID and span ID

Benchmarks (from slog proposal, Go 1.21 release notes, confirmed by independent
reports in 2024-2025): slog JSON handler is within 2x of zerolog on typical
workloads; for our scale (request-level logging, not tight inner loops) the
overhead is negligible.

**Decision: slog (stdlib) as the only logger. No fallback to zerolog/zap.**

---

## 2. Prometheus client_golang

### 2.1 Library Selection

- Repository: `github.com/prometheus/client_golang`
- Pinned version target: **v1.20.x** (latest stable minor as of 2026-04-24;
  v1.20.0 released late 2024 introduced the new `promhttp` handler opts and
  refined `NewCounterVec`/`NewHistogramVec` labeling; subsequent v1.20.z
  patches track fixes).
- License: Apache-2.0 (on allowlist per SPEC-DEP-001 REQ-DEP-004).
- Already reserved as a future-dep in SPEC-DEP-001 §6.1 Future-Dependencies
  table, mapped to SPEC-OBS-001. That reservation is now consumed.

### 2.2 HTTP Handler Pattern

Standard idiom (documented at
<https://pkg.go.dev/github.com/prometheus/client_golang/prometheus/promhttp>):

```go
import (
    "net/http"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

// Mount promhttp.Handler() at /metrics on a dedicated admin mux,
// listening on a separate port (default 9090). Never expose on the public
// API port — /metrics must be scrape-target-only, behind ingress ACL in prod.
adminMux := http.NewServeMux()
adminMux.Handle("/metrics", promhttp.Handler())
go http.ListenAndServe(":9090", adminMux)
```

Rationale for port separation:

- Mixing `/metrics` with public API routes creates cardinality and auth
  pollution risk.
- Kubernetes `serviceMonitor` and Docker `prometheus.yml` scrape configs
  expect dedicated admin endpoints.
- Future `usearch-api` and `usearch-mcp` binaries will each expose their
  own admin port; `/metrics` is per-process, not per-service.

### 2.3 Metric Types to Expose (V1 Baseline)

From Universal Search's architectural context (`structure.md` §2 Service
Topology, `tech.md` §5 Hybrid Ranking, `roadmap.md` M3 fanout):

| Metric Name | Type | Labels | Purpose |
|-------------|------|--------|---------|
| `usearch_http_requests_total` | Counter | `method`, `route`, `status_class` (2xx/4xx/5xx) | Request volume, error rate |
| `usearch_http_request_duration_seconds` | Histogram | `method`, `route` | Latency distribution; default buckets + exemplars for trace linking |
| `usearch_fanout_goroutines_inflight` | Gauge | `adapter_class` (web/social/academic/korean) | Active fanout concurrency |
| `usearch_adapter_calls_total` | Counter | `adapter`, `outcome` (success/failure/timeout) | Adapter reliability — feeds SPEC-EVAL-002 dashboard |
| `usearch_adapter_call_duration_seconds` | Histogram | `adapter` | Per-adapter latency |
| `usearch_build_info` | Gauge (value always 1) | `version`, `commit`, `go_version` | Standard `_build_info` pattern for version tracking |

Bucket strategy for histograms:

- HTTP latency: `prometheus.DefBuckets` (0.005s to 10s) — standard choice.
- Adapter call duration: custom buckets `[0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30]`
  seconds. Rationale: adapter calls have bimodal distribution (fast API calls
  ~100ms, slow scraping ~5-30s); default buckets lose resolution in both tails.

**Cardinality discipline (enforced by NFR-OBS-002, see spec.md):**

- NO per-user, per-query, per-URL, or per-request-ID labels. These explode
  the cardinality budget.
- `route` label is the route template (e.g., `/v1/query`), never the raw
  path with path params inlined.
- `adapter` is a bounded enum (≤20 values at V1 target); new adapters require
  registry update.
- For per-request tracing (which is legitimately high cardinality), use
  Prometheus **exemplars** (see §2.4) or the OTel trace pipeline (see §3).

### 2.4 Exemplars (Trace Linking)

Prometheus native histogram exemplars (stable since client_golang v1.16):

```go
reqDuration := prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
        Name: "usearch_http_request_duration_seconds",
        // ...
    }, []string{"method", "route"},
)
// On observation, attach traceID as exemplar label:
reqDuration.WithLabelValues(method, route).(prometheus.ExemplarObserver).
    ObserveWithExemplar(
        elapsed.Seconds(),
        prometheus.Labels{"trace_id": spanCtx.TraceID().String()},
    )
```

Exemplars are sampled, low-cardinality, and Grafana's Trace-to-Metrics
correlation works natively with them. This is the intended bridge between
high-cardinality trace data and low-cardinality metric data.

### 2.5 Reference Implementations

- <https://github.com/prometheus/client_golang/tree/main/examples/simple> —
  canonical minimal server pattern.
- <https://github.com/charmbracelet/crush> (pinned LSP client upstream per
  `.claude/rules/moai/core/lsp-client.md`) uses client_golang for internal
  observability; confirms pattern fits Go monorepos of this shape.
- `go.opentelemetry.io/otel/exporters/prometheus` — OTel bridge exporter,
  considered but **not chosen** for V1 (see §3.4 below).

---

## 3. OpenTelemetry Go SDK

### 3.1 Library Selection

- Repository: `go.opentelemetry.io/otel` (core) + separate exporter modules
- Pinned version target (core API/SDK): **v1.30.x** (latest stable as of
  2026-04-24; tracked at <https://github.com/open-telemetry/opentelemetry-go/releases>).
- Exporter module: `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc`
  at matching v1.30.x.
- Propagator: `go.opentelemetry.io/otel/propagation` (W3C TraceContext +
  Baggage; part of core).
- License: Apache-2.0.

### 3.2 Wiring Pattern

Standard initializer (to live in `internal/obs/trace/`):

```go
// Init reads OTLP_ENDPOINT; if unset, returns a no-op tracer provider.
// If set, initializes OTLP gRPC exporter → BatchSpanProcessor → TracerProvider.
func Init(ctx context.Context, svc, version string) (func(context.Context) error, error) {
    endpoint := os.Getenv("OTLP_ENDPOINT")
    if endpoint == "" {
        otel.SetTracerProvider(noop.NewTracerProvider())
        return func(context.Context) error { return nil }, nil
    }
    exp, err := otlptracegrpc.New(ctx,
        otlptracegrpc.WithEndpoint(endpoint),
        otlptracegrpc.WithInsecure(), // configurable via OTLP_INSECURE
    )
    if err != nil { return nil, err }
    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exp),
        sdktrace.WithResource(resource.NewSchemaless(
            attribute.String("service.name", svc),
            attribute.String("service.version", version),
        )),
        sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(0.1))),
    )
    otel.SetTracerProvider(tp)
    otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
        propagation.TraceContext{}, propagation.Baggage{},
    ))
    return tp.Shutdown, nil
}
```

Key points:

- **No-op by default**: When `OTLP_ENDPOINT` is unset, we install a no-op
  tracer provider. Callers use `otel.Tracer("obs")` and get span-shaped
  no-ops. Hot paths pay zero cost beyond pointer dereferences.
- **Batch processor**: Avoids per-span gRPC round-trips; `BatchSpanProcessor`
  default batch size 512, timeout 5s — appropriate for request-scoped tracing.
- **Sampler**: `ParentBased(TraceIDRatioBased(0.1))` = honor upstream sampling
  decisions if a parent exists; otherwise sample 10%. Sampling ratio is
  configurable (`OTLP_SAMPLE_RATIO`, default 0.1 per NFR-OBS-001).
- **Propagator**: W3C TraceContext + Baggage composite. This is the MCP-era
  default; `traceparent`/`tracestate` headers propagate across process
  boundaries (Go HTTP client ↔ Python sidecar ↔ LiteLLM proxy).
- **Shutdown**: Returned closure must be deferred in `main()` of each binary;
  flushes pending spans before process exit.

### 3.3 Span Attributes

Convention (OpenTelemetry semantic conventions 1.27+):

| Attribute | Value Source | Example |
|-----------|--------------|---------|
| `http.request.method` | Request method | `GET` |
| `http.route` | Route template | `/v1/query` |
| `http.response.status_code` | Status code | `200` |
| `service.name` | Build-time constant | `usearch-api` |
| `usearch.request_id` | From context (REQ-OBS-002) | `01HW...` |
| `usearch.adapter` | Fanout child spans | `reddit` |
| `usearch.query.intent` | Router classification | `web`, `korean` |

Custom attributes use `usearch.*` prefix (semantic conventions reserve
standard namespaces; custom must be domain-qualified).

### 3.4 OTel Metrics: Deferred

OpenTelemetry offers a metrics API (`go.opentelemetry.io/otel/metric`) and
Prometheus exporter bridge (`go.opentelemetry.io/otel/exporters/prometheus`).
We **defer** this path for V1 because:

- `tech.md` §3 specifies `prometheus client_golang` directly (not via OTel
  bridge). Honoring architectural constraint.
- OTel metrics SDK for Go was stabilized (v1.0) in 2023 but the bridge
  exporter adds a translation layer without clear upside for our target stack
  (Prometheus → Grafana).
- Direct client_golang gives us exemplars, native histograms (optional
  future), and full control over label schema without OTel instrument-type
  ceremony.
- Traces via OTel + Metrics via Prometheus is an accepted 2026 pattern
  (see Grafana Labs "Observability 2.0" reference architecture).

**Decision: OTel for traces only. Prometheus client_golang for metrics.**
Revisit in a post-V1 SPEC if multi-backend metrics export becomes a
requirement.

---

## 4. Loki Integration (Optional, Stretch Goal)

`tech.md` §3 Observability table lists Loki as optional
("slog → Loki (optional)"). Two implementation paths considered:

### Path A: Promtail Sidecar (recommended when Loki is enabled)

- Deploy `grafana/promtail` as a docker-compose service or k8s sidecar.
- Universal Search processes emit JSON logs to stderr (slog default).
- Promtail scrapes container stdout/stderr and forwards to Loki.
- Zero coupling between Go code and Loki — slog handler stays pure.
- Promtail handles batching, backpressure, label mapping.
- Operational ownership: SPEC-EVAL-002 (M8) or dedicated deploy SPEC.

### Path B: Direct slog → Loki Handler

- Implement custom `slog.Handler` that buffers records and POSTs to
  Loki's `/loki/api/v1/push` endpoint.
- Libraries surveyed:
  - `github.com/magefreeze/slog-loki` — small community project, limited
    maintenance as of 2026-04.
  - `github.com/samber/slog-loki` — part of samber's slog-* suite, MIT,
    actively maintained, v2 supports streaming.
- Tradeoff: Go code owns the Loki client (backpressure, retry, batching).
  Adds failure mode (Loki down → slog handler errors) unless carefully
  gated with a fallback handler.

**V1 decision: Neither path is built in SPEC-OBS-001.** The SPEC exposes
`LOKI_ENDPOINT` as a gating env var (REQ-OBS-007, marked P2/optional) and
leaves the handler implementation as a TODO. Production deployments use
Promtail sidecar (Path A) added via deploy SPEC. The `internal/obs/log`
package exposes a handler-registration seam so Path B can drop in later
without API changes.

---

## 5. Request ID Propagation

### 5.1 Design Choice: `context.Context`-Carried Request ID

Standard Go idiom for request-scoped values. Trade-offs:

| Carrier | Pros | Cons |
|---------|------|------|
| `context.Context` (recommended) | Idiomatic, type-safe via unexported key, crosses package boundaries | Requires ctx plumbing |
| Goroutine-local (via runtime patch) | Transparent | Hacky, breaks Go semantics |
| HTTP middleware only | Simple at edge | Lost in business logic |

Universal Search is goroutine-heavy (fanout pool per `structure.md`), so
context is the only safe carrier. HTTP middleware injects the request ID
into context at the edge; downstream code reads it with
`obs.RequestID(ctx)`.

### 5.2 Generator: ULID vs UUID vs nanoid

Candidates:

| Generator | Length | Sortable | Entropy | Library |
|-----------|--------|----------|---------|---------|
| ULID | 26 chars | Time-sortable (first 48 bits = ms timestamp) | 80 bits | `github.com/oklog/ulid/v2` |
| UUIDv7 | 36 chars | Time-sortable (draft RFC, finalized as RFC 9562 in 2024) | 74 bits | `github.com/google/uuid` v1.6+ (supports v7) |
| UUIDv4 | 36 chars | Not sortable | 122 bits | `github.com/google/uuid` |
| nanoid | 21 chars default | Not sortable | configurable | `github.com/matoous/go-nanoid` |

**Recommendation: ULID via `github.com/oklog/ulid/v2`.**

- Universally decodable (Base32 Crockford variant, no padding).
- Sortable (makes log line grep and span-ID linking natural).
- Stable library, Apache-2.0.
- 26-char ID fits comfortably in `traceparent`-adjacent custom headers.
- Open Question tracked in spec.md §11: ULID vs UUIDv7 (UUIDv7 is equally
  valid; default choice is ULID for shorter header footprint).

### 5.3 Propagation Boundaries

| Boundary | Carrier | Header |
|----------|---------|--------|
| HTTP ingress | `X-Request-ID` header; middleware reads or generates | `X-Request-ID` |
| HTTP egress (to adapters, Python sidecars, LiteLLM) | Go client middleware writes `X-Request-ID` + W3C `traceparent` | `X-Request-ID` + `traceparent` |
| gRPC (future SPEC-IR-001) | `grpc.Metadata["x-request-id"]` + OTel propagator | Metadata |
| Goroutine fanout | `context.Context` carries both request ID and span context |
| Log records | slog handler auto-injects `request_id` attribute from ctx | — |
| Metrics | NOT used as label (see §2.3 cardinality discipline); use exemplars |
| Traces | `usearch.request_id` span attribute |

### 5.4 Header Choice

`X-Request-ID` is the de facto standard (used by nginx, AWS ELB, Heroku,
GitHub, and most API gateways). Alternative `X-Correlation-ID` is used
primarily in the .NET ecosystem. `X-Request-ID` is the right default for
a Go service with broad upstream compatibility.

---

## 6. Reference Implementations from Roadmap-Adjacent Projects

### 6.1 charmbracelet/crush (LSP client upstream, already in moai-constitution)

- Uses `log/slog` with a custom handler for structured output.
- Propagates request-equivalent IDs through `context.Context`.
- Confirms slog-first pattern is current idiom for high-quality 2026 Go codebases.

### 6.2 gpt-researcher (services/researcher, M1 dep per SPEC-BOOT-001)

- Python project (FastAPI + logging stdlib).
- Will consume `traceparent` headers from Go caller and emit to STDOUT
  in JSON (per FastAPI convention). Promtail can scrape both Go and
  Python logs uniformly.
- No direct Go-side work here, but informs the cross-language propagator
  requirement (W3C TraceContext, not Go-specific format).

### 6.3 searxng (compose service, AGPL service boundary)

- Runs in its own container. We do not modify SearXNG; we just ensure
  our Go adapter (SPEC-ADP-007) forwards `X-Request-ID` when calling
  SearXNG's HTTP API.

### 6.4 Existing MoAI code (moai-adk-go)

- `internal/lsp/core/` (powernap-based) uses zerolog in some paths; this
  was a pre-SPEC-OBS-001 choice. Universal Search is a separate monorepo
  and adopts slog for consistency with `tech.md` — no cross-contamination.

---

## 7. Existing Stub Analysis: `internal/obs/obs.go`

Current state (verified 2026-04-24):

```go
// Package obs is the stub for observability (slog, Prometheus, OpenTelemetry).
// Full implementation lands in SPEC-OBS-001.
package obs
```

Two lines of content (package declaration + doc comment). No exported types,
no imports. Created by SPEC-BOOT-001 as a reserved package root.

### 7.1 Proposed Package Layout

```
internal/obs/
├── obs.go                  # package-level Init() entrypoint, exports Logger/Meter/Tracer/WithRequestID
├── log/
│   ├── log.go              # slog.Handler construction (JSON, level from env)
│   ├── enrich.go           # middleware handler that injects request_id, trace_id, span_id from ctx
│   └── log_test.go         # TestLoggerEmitsJSON, TestLevelFromEnv
├── metrics/
│   ├── metrics.go          # prometheus.Registerer; exported var Registry + named collectors
│   ├── http.go             # HTTP middleware: Counter + Histogram with route template extraction
│   ├── server.go           # adminServer exposes /metrics on configurable port
│   └── metrics_test.go     # TestMetricsEndpointExposes200, TestNoUnboundedLabels
├── trace/
│   ├── trace.go            # OTel TracerProvider init, propagator setup, shutdown closure
│   ├── trace_test.go       # TestOTLPInitializesWhenEndpointSet, TestNoOpWhenEndpointUnset
│   └── attrs.go            # Semantic-convention attribute builders
└── reqid/
    ├── reqid.go            # ULID generator, context helpers (WithRequestID, FromContext)
    ├── http.go             # HTTP middleware: read/generate X-Request-ID header
    ├── client.go           # http.RoundTripper wrapper that writes X-Request-ID on egress
    └── reqid_test.go       # TestRequestIDPropagatesThroughContext, TestGenerateIsULID
```

### 7.2 Top-Level API (in `obs.go`)

```go
package obs

import (
    "context"
    "github.com/elymas/universal-search/internal/obs/log"
    "github.com/elymas/universal-search/internal/obs/metrics"
    "github.com/elymas/universal-search/internal/obs/trace"
    "github.com/elymas/universal-search/internal/obs/reqid"
)

// Config is read from environment at Init time.
type Config struct {
    ServiceName    string // e.g., "usearch-api"
    ServiceVersion string
    LogLevel       string // DEBUG|INFO|WARN|ERROR; default INFO
    AdminAddr      string // e.g., ":9090"; default ":9090"
    OTLPEndpoint   string // empty → no-op tracer
    LokiEndpoint   string // empty → no Loki handler (always empty in V1)
}

// Init initializes the full observability stack from environment and Config,
// starts the admin /metrics server, and returns a shutdown closure.
func Init(ctx context.Context, cfg Config) (shutdown func(context.Context) error, err error)

// Convenience re-exports for downstream consumers.
var (
    WithRequestID = reqid.WithContext
    RequestID     = reqid.FromContext
    Logger        = log.Default // returns context-aware *slog.Logger
    Tracer        = trace.Tracer
    // Metric names exported for use by business code
    HTTPRequests         = metrics.HTTPRequests
    HTTPRequestDuration  = metrics.HTTPRequestDuration
    AdapterCalls         = metrics.AdapterCalls
    AdapterCallDuration  = metrics.AdapterCallDuration
    FanoutInflight       = metrics.FanoutInflight
)
```

This layout minimizes surface area for M2+ domain packages. They consume
`obs.Logger(ctx)`, `obs.Tracer(...)`, and the named metrics directly; they
never import `github.com/prometheus/client_golang` or `go.opentelemetry.io/otel`
themselves. Centralization prevents cardinality drift and propagator
divergence.

---

## 8. Compose Integration

### 8.1 Current Compose State

`deploy/docker-compose.yml` (SPEC-BOOT-001 + SPEC-DEP-001 baseline) runs six
services: qdrant, meilisearch, postgres, redis, searxng, litellm. No
Prometheus, no Grafana, no Loki.

### 8.2 Changes Required by SPEC-OBS-001 Run Phase

Minimal additions to enable metrics scraping in dev:

```yaml
# Additions to deploy/docker-compose.yml (not written in plan phase):
services:
  # ── Prometheus (metrics scraper) ──
  prometheus:
    image: prom/prometheus:v2.54.1  # or latest stable at run-phase pinning
    ports:
      - "${PROMETHEUS_PORT:-9091}:9090"   # dev host port 9091 to avoid clash with in-process /metrics
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

New file `deploy/prometheus/prometheus.yml`:

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'usearch-api'
    static_configs:
      - targets: ['host.docker.internal:9090']  # dev: Go binary runs on host
        labels:
          service: 'usearch-api'
  - job_name: 'usearch-mcp'
    static_configs:
      - targets: ['host.docker.internal:9092']
        labels:
          service: 'usearch-mcp'
```

**Grafana is deferred.** `roadmap.md` M8 has SPEC-EVAL-002 (Adapter reliability
dashboard); Grafana provisioning is the natural home for that SPEC. SPEC-OBS-001
exposes metrics; visualization is out of scope.

**Loki is deferred.** Added via future deploy SPEC if `LOKI_ENDPOINT` path is
adopted. V1 path: Promtail sidecar when needed.

**OTel collector is optional in dev.** If `OTLP_ENDPOINT` is unset, no
collector is needed. If set (developer wants to see traces), developer runs
their own `otel/opentelemetry-collector-contrib` + Tempo or Jaeger
manually — not in `docker-compose.yml` by default. Keep dev stack minimal.

### 8.3 `.env.example` Additions

```
# Observability (SPEC-OBS-001)
LOG_LEVEL=INFO                          # DEBUG | INFO | WARN | ERROR
USEARCH_ADMIN_PORT=9090                 # /metrics endpoint port for usearch-api
OTLP_ENDPOINT=                          # empty → tracing disabled (no-op)
OTLP_SAMPLE_RATIO=0.1                   # 0.0-1.0, default 0.1
LOKI_ENDPOINT=                          # empty → no Loki forwarding (V1)
PROMETHEUS_PORT=9091                    # compose-side Prometheus host port
```

---

## 9. Prior Art and References

External authoritative sources:

1. Go slog package:
   <https://pkg.go.dev/log/slog>
2. Prometheus client_golang:
   <https://pkg.go.dev/github.com/prometheus/client_golang/prometheus>
3. Prometheus promhttp:
   <https://pkg.go.dev/github.com/prometheus/client_golang/prometheus/promhttp>
4. OpenTelemetry Go SDK:
   <https://pkg.go.dev/go.opentelemetry.io/otel>
5. OpenTelemetry OTLP gRPC trace exporter:
   <https://pkg.go.dev/go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc>
6. W3C TraceContext specification (used by OTel propagator):
   <https://www.w3.org/TR/trace-context/>
7. Prometheus instrumentation best practices (cardinality, naming):
   <https://prometheus.io/docs/practices/instrumentation/>
8. Prometheus naming conventions:
   <https://prometheus.io/docs/practices/naming/>
9. Grafana Loki pushing logs guide (Path A reference):
   <https://grafana.com/docs/loki/latest/send-data/>
10. ULID specification:
    <https://github.com/ulid/spec>
11. RFC 9562 (UUIDv7, for Open Question in spec.md §11):
    <https://www.rfc-editor.org/rfc/rfc9562>
12. OpenTelemetry semantic conventions for HTTP:
    <https://opentelemetry.io/docs/specs/semconv/http/>

Internal references:

- `.moai/project/tech.md` §3 Observability — stack choice authoritative
- `.moai/project/structure.md` §3 Bounded Contexts — `internal/obs/` owner
- `.moai/project/roadmap.md` M1 — SPEC-OBS-001 owner: expert-performance
- `.moai/specs/SPEC-BOOT-001/spec.md` §7 File Impact — `internal/obs/obs.go` stub
- `.moai/specs/SPEC-DEP-001/spec.md` §6.1 — client_golang reserved for SPEC-OBS-001

---

## 10. Research Conclusions (feed into spec.md)

1. **Logging**: slog (stdlib), JSON handler, level from `LOG_LEVEL` env, context-aware enrichment.
2. **Request ID**: ULID via `github.com/oklog/ulid/v2`, carried in `context.Context`, surfaced as `X-Request-ID` header at HTTP boundaries, auto-injected into every slog record and as `usearch.request_id` span attribute.
3. **Metrics**: prometheus client_golang v1.20.x, dedicated `/metrics` admin server on port 9090 (configurable), bounded cardinality labels, exemplars for trace linking.
4. **Tracing**: OpenTelemetry Go v1.30.x core + OTLP gRPC trace exporter, W3C TraceContext propagator, 10% default sampling, no-op when `OTLP_ENDPOINT` unset.
5. **Metrics via OTel bridge**: Deferred (explicit non-goal for V1).
6. **Loki**: Deferred; env var reserved (`LOKI_ENDPOINT`). Production path: Promtail sidecar.
7. **Package layout**: `internal/obs/{log,metrics,trace,reqid}/` with thin top-level `obs.go` exporting `Init`, `Logger`, `Tracer`, `WithRequestID`, and named metrics for M2+ consumers.
8. **Compose delta**: add Prometheus + `deploy/prometheus/prometheus.yml`. Grafana and Loki deferred.
9. **NFRs**: instrumentation overhead under 5% p99 on synthetic benchmark; no unbounded label values; all tool versions pinned in go.mod.
10. **Open questions for spec.md §11**: ULID vs UUIDv7; Prometheus metric naming namespace (`usearch_*` vs OTel-bridge `otel_*`); `/metrics` auth strategy for production; default sampling ratio.

Ready to hand off to `.moai/specs/SPEC-OBS-001/spec.md`.
