# SPEC-OBS-001 Plan — Post-Hoc Implementation Summary

Created: 2026-04-24
Updated: 2026-04-26 (post-implementation reconciliation)
Author: limbowl (via manager-spec)
Status: implemented
Methodology: TDD (RED-GREEN-REFACTOR)
Coverage Target: 85%

## 0. Plan Scope

Reverse-engineered description of how SPEC-OBS-001 was implemented as
the observability foundation for every M2+ package. Delivered on
2026-04-26 (commit 0234b71). Read alongside spec.md (requirements)
and acceptance.md (Given/When/Then scenarios).

## 1. Approach Summary

The `internal/obs` package was decomposed into four sub-packages
(`log`, `metrics`, `reqid`, `trace`) plus a top-level orchestrator
`obs.go`. `obs.Init(ctx, Config)` constructs the four subsystems in
order and returns an `*Obs` bundle plus an idempotent shutdown
closure. Structured logging uses `log/slog` with a JSON handler;
metrics use `github.com/prometheus/client_golang` with named
collectors stored in a `*metrics.Registry`; request IDs use ULID
generation with `context.Context` propagation; tracing uses OTel SDK
with OTLP gRPC exporter (or a no-op TracerProvider when
`OTLPEndpoint` is unset). The public surface is consumed by every M2+
domain package without those packages importing prometheus or otel
libraries directly (import-boundary enforcement via tests).

## 2. Reference Implementations (consumed)

This SPEC creates the patterns that other SPECs follow. Nothing to
mirror.

## 3. Package Layout (as implemented)

```
internal/obs/
├── obs.go                          # Top-level Init + Config + Obs bundle + re-exports
├── obs_test.go                     # Package-level integration + API surface tests
├── obs_faithfulness_test.go        # Faithfulness re-export tests (SPEC-SYN-002 follow-on)
├── log/
│   ├── log.go                      # New(w, level) → *slog.Logger; LevelFromEnv
│   ├── enrich.go                   # Context-enrichment handler (request_id/trace_id/span_id)
│   ├── register.go                 # RegisterHandler hook (REQ-OBS-007 Loki seam)
│   └── log_test.go
├── metrics/
│   ├── metrics.go                  # Registry struct + NewRegistry + cardinality allowlist
│   ├── http.go                     # HTTP middleware (counter + histogram + exemplar)
│   ├── server.go                   # StartAdminServer → /metrics + /healthz
│   ├── llm.go                      # SPEC-LLM-001 collectors (LLMCalls, LLMCost, LLMLatency)
│   ├── access.go                   # SPEC-CACHE-001 collectors (Phase*, AccessFetchTotal)
│   ├── synthesis.go                # SPEC-SYN-001 collectors
│   ├── synthcluster.go             # SPEC-SYN-003 collectors
│   ├── tokenizer.go                # SPEC-SYN-* collectors
│   └── metrics_test.go             # TestNoUnboundedLabels static analysis
├── trace/
│   ├── trace.go                    # Init → OTel TracerProvider + propagator + Tracer
│   ├── attrs.go                    # SemConv helper attributes
│   └── trace_test.go
├── reqid/
│   ├── reqid.go                    # ULID gen + WithContext + FromContext
│   ├── http.go                     # Ingress middleware (X-Request-ID)
│   ├── client.go                   # Egress NewTransport (RoundTripper wrapper)
│   └── reqid_test.go
└── bench/
    └── (NFR-OBS-001 benchmark; runs in scheduled-weekly CI bench job)
```

`deploy/prometheus/prometheus.yml` created with static scrape
configs for `usearch-api` (port 9090) and `usearch-mcp` (port 9092).

`deploy/docker-compose.yml` extended with a `prometheus` service:
`prom/prometheus:v2.54.1`, port `${PROMETHEUS_PORT:-9091}:9090`,
volume mounts for the config + `prometheus_data` named volume.

`.env.example` (root) appended with 6 observability vars: `LOG_LEVEL`,
`USEARCH_ADMIN_PORT`, `OTLP_ENDPOINT`, `OTLP_SAMPLE_RATIO`,
`LOKI_ENDPOINT`, `PROMETHEUS_PORT`.

`go.mod` gained 8+ new direct dependencies (per spec.md §6.5):
`prometheus/client_golang v1.20.x`, `prometheus/client_model`,
`prometheus/common`, `otel v1.30.x`, `otel/sdk`, `otel/trace`,
`otel/exporters/otlp/otlptrace/otlptracegrpc`, `google.golang.org/grpc`,
`oklog/ulid/v2`.

## 4. Key Implementation Files (file:line refs)

### Top-level orchestrator
- `internal/obs/obs.go:1-149` — Package doc; `Config` struct (12 fields);
  `Obs` bundle struct (`Logger`, `Metrics`, `AdminAddr`,
  `tracerProvider`); `Init(ctx, cfg) (*Obs, shutdown, err)`.
- `internal/obs/obs.go:65-73` — `Tracer(name)` returns the OTel tracer;
  `HasTracer()` reports whether wired (false for zero-value Obs in
  tests).
- `internal/obs/obs.go:130-146` — Idempotent shutdown via
  `shutdownCalled` flag; first non-nil error returned; admin server
  and tracer flushed in order.
- `internal/obs/obs.go:156-339` — `(*Obs)` accessor methods for
  domain-specific collectors added by downstream SPECs (Synthesis,
  Tokenizer, IndexShard, StreamSynth, SynthCluster, DeepReport,
  DeepAgent, DeepTree, etc.). All nil-safe.

### Logging (`internal/obs/log/`)
- `log.go::New(w, level) *slog.Logger` — constructs
  `slog.NewJSONHandler(w, &HandlerOptions{Level: level, AddSource:
  false})` wrapping the writer; default writer is `os.Stderr`.
- `log.go::LevelFromEnv(override)` — parses LOG_LEVEL env (DEBUG/INFO/
  WARN/ERROR); override wins; invalid → INFO.
- `enrich.go` — handler wrapper that injects `request_id` (from
  `reqid.FromContext(ctx)`) and `trace_id`/`span_id` (from active OTel
  span) as slog attributes.
- `register.go` — `RegisterHandler(h slog.Handler)` tees subsequent
  records to a registered custom handler (REQ-OBS-007 Loki seam).

### Metrics (`internal/obs/metrics/`)
- `metrics.go::Registry` struct holds 6 baseline collectors
  (`HTTPRequests`, `HTTPRequestDuration`, `FanoutInflight`,
  `AdapterCalls`, `AdapterCallDuration`, `BuildInfo`) plus N
  domain-specific collectors added by later SPECs.
- `metrics.go::NewRegistry() *Registry` constructs and registers all
  collectors on a non-default `*prometheus.Registry` (test isolation).
- `metrics.go::cardinality allowlist` — static map enforced by
  `TestNoUnboundedLabels`; original set: `{method, route,
  status_class, adapter_class, adapter, outcome, version, commit,
  go_version}`. Extended by later SPECs (`provider`, `model`,
  `phase`, etc.).
- `http.go` — HTTP middleware: counter (`HTTPRequests`) + histogram
  (`HTTPRequestDuration`) + exemplar (trace_id from active OTel span
  when present).
- `server.go::StartAdminServer(ctx, addr, reg) (string, shutdown, err)`
  — binds an admin HTTP server on `addr` (e.g., `127.0.0.1:9090`),
  mounts `/metrics` via `promhttp.HandlerFor(reg)` and `/healthz`
  returning 200 OK.

### Tracing (`internal/obs/trace/`)
- `trace.go::Init(ctx, cfg) (shutdown, err)`:
  - If `cfg.OTLPEndpoint == ""` → install `noop.NewTracerProvider()`
    as global; return trivial shutdown.
  - Else → `otlptracegrpc.New(ctx, WithEndpoint(cfg.OTLPEndpoint),
    WithInsecure())` → `sdktrace.BatchSpanProcessor` →
    `sdktrace.NewTracerProvider(WithBatcher, WithResource,
    WithSampler(ParentBased(TraceIDRatioBased(ratio))))`.
  - Install composite propagator
    `propagation.NewCompositeTextMapPropagator(TraceContext{},
    Baggage{})` via `otel.SetTextMapPropagator`.
- `trace.go::Tracer(name) trace.Tracer` returns `otel.Tracer(name)`.

### Request ID (`internal/obs/reqid/`)
- `reqid.go::New() string` — ULID via `github.com/oklog/ulid/v2` with
  monotonic entropy; returns 26-char Crockford Base32.
- `reqid.go::WithContext(ctx, id)` / `FromContext(ctx) string` —
  unexported key type; `FromContext` returns empty string when unset.
- `http.go::Middleware(next http.Handler) http.Handler` — reads
  `X-Request-ID` from inbound request; generates ULID if absent;
  binds to ctx; writes back to response header.
- `client.go::NewTransport(next http.RoundTripper) http.RoundTripper`
  — on RoundTrip, reads ID from `req.Context()`; if non-empty, sets
  `req.Header["X-Request-ID"] = [id]`.

## 5. Integration Points

| Upstream SPEC | Consumed via |
|---------------|--------------|
| SPEC-BOOT-001 | `internal/obs/obs.go` stub replaced; `deploy/docker-compose.yml` extended with Prometheus service; `cmd/usearch*/main.go` gain `obs.Init` blocks; `.env.example` extended |
| SPEC-DEP-001 | `docs/dependencies.md` regenerated with new direct deps; audit CI runs on expanded `go.mod` |

| Downstream SPEC | Provides |
|-----------------|----------|
| SPEC-LLM-001 | `LLMCalls/LLMCost/LLMLatency` registered via `registerLLM(r)` from `NewRegistry`; cardinality allowlist extended with `provider`, `model` |
| SPEC-CORE-001 | `AdapterCalls/AdapterCallDuration` already registered baseline; cardinality allowlist contains `adapter`, `outcome` |
| SPEC-CACHE-001 | New `Access*` collectors registered via `registerAccess(r)`; cardinality allowlist extended with `phase`, `blocked` |
| SPEC-IR-001 | Router observability calls `obs.Logger(ctx)` + `obs.Tracer("usearch.router")` + named counters |
| SPEC-FAN-001 | `FanoutInflight` gauge baseline + per-adapter timing |
| SPEC-SYN-001/002/003/004 | New `Synthesis*` collectors via `registerSynthesis` + `registerSynthCluster` + `registerStreamSynth` |
| SPEC-DEEP-001/002/003/004 | New `DeepReport*` / `DeepAgent*` / `DeepTree*` collectors |
| Every M2+ HTTP handler | Wraps with `reqid.Middleware` (ingress); wraps `http.Client.Transport` with `reqid.NewTransport` (egress) |
| Every M2+ slog call | `obs.Logger(ctx)` returns the enriched logger |
| Every M2+ OTel span | `obs.Tracer(name).Start(ctx, ...)` |
| SPEC-SYN-004 | Span context propagation across SSE streams (REQ-OBS-005 propagator must be composite) |
| SPEC-EVAL-002 | Reads `AdapterCalls/AdapterCallDuration` for reliability dashboard |

## 6. Data Structures and Interfaces

### Top-level (`obs.go`)
```go
type Config struct {
    ServiceName    string
    ServiceVersion string
    GitCommit      string
    LogLevel       string         // override LOG_LEVEL env; default INFO
    LogWriter      io.Writer      // default os.Stderr
    AdminAddr      string         // e.g. "127.0.0.1:9090"; empty disables admin server
    OTLPEndpoint   string         // empty → no-op tracer provider
    SampleRatio    float64        // default 0.1
}

type Obs struct {
    Logger    *slog.Logger
    Metrics   *metrics.Registry
    AdminAddr string  // actual listening address (empty when no admin server)
    // unexported tracerProvider
}

func (o *Obs) Tracer(name string) oteltrace.Tracer
func (o *Obs) HasTracer() bool

func Init(ctx context.Context, cfg Config) (*Obs, func(context.Context) error, error)
```

### Logging
```go
package log
func New(w io.Writer, level slog.Level) *slog.Logger
func LevelFromEnv(override string) slog.Level
func FromContext(ctx context.Context) *slog.Logger
func RegisterHandler(h slog.Handler)  // REQ-OBS-007 seam
```

### Metrics
```go
package metrics

type Registry struct {
    // Baseline (BOOT-001 reservation, OBS-001 registration)
    HTTPRequests        *prometheus.CounterVec      // labels: method, route, status_class
    HTTPRequestDuration *prometheus.HistogramVec    // labels: method, route
    FanoutInflight      *prometheus.GaugeVec        // labels: adapter_class
    AdapterCalls        *prometheus.CounterVec      // labels: adapter, outcome
    AdapterCallDuration *prometheus.HistogramVec    // labels: adapter
    BuildInfo           *prometheus.GaugeVec        // labels: version, commit, go_version

    // Domain-specific (registered by later SPECs via register* helpers)
    LLMCalls, LLMCost, LLMLatency               // SPEC-LLM-001
    SynthesisCalls, SynthesisCost, ...           // SPEC-SYN-001
    AccessPhaseAttempts, AccessPhaseDuration, ...// SPEC-CACHE-001
    // ... etc.
}

func NewRegistry() *Registry
func StartAdminServer(ctx context.Context, addr string, reg *Registry) (string, func(context.Context) error, error)
```

### Request ID
```go
package reqid

func New() string                                   // ULID, 26-char Crockford Base32
func WithContext(ctx context.Context, id string) context.Context
func FromContext(ctx context.Context) string        // "" if unset
func Middleware(next http.Handler) http.Handler     // ingress
func NewTransport(next http.RoundTripper) http.RoundTripper  // egress
```

### Tracing
```go
package trace

type Config struct {
    ServiceName, ServiceVersion, GitCommit string
    OTLPEndpoint                            string  // empty → no-op
    SampleRatio                             float64
}

func Init(ctx context.Context, cfg Config) (func(context.Context) error, error)
func Tracer(name string) oteltrace.Tracer
```

## 7. Test Coverage Notes

Test inventory (~28 representative tests per spec.md §8):
- `log/log_test.go` — `TestLoggerEmitsJSON`, `TestLevelFromEnv`,
  `TestLogBelowLevelSuppressed`, `TestSlogRecordIncludesRequestID`,
  `TestLokiEnvReserved`, `TestRegisterHandlerHookSeam`.
- `reqid/reqid_test.go` — `TestRequestIDPropagatesThroughContext`,
  `TestIngressGeneratesWhenAbsent` (regex
  `^[0-9A-HJKMNP-TV-Z]{26}$`), `TestIngressPreservesWhenPresent`,
  `TestEgressWritesHeader`.
- `metrics/metrics_test.go` — `TestHTTPMiddlewareIncrementsCounter`,
  `TestHTTPMiddlewareRecordsDuration`, `TestFanoutGaugeInflight`,
  `TestAdapterCallOutcomeLabels`, `TestMetricsEndpointExposes200`,
  `TestMetricsIncludesAllFamilies`, `TestAdminPortConfigurable`,
  `TestNoUnboundedLabels`.
- `trace/trace_test.go` — `TestOTLPInitializesWhenEndpointSet`,
  `TestOTLPNoOpWhenEndpointUnset`, `TestSampleRatioFromEnv`,
  `TestShutdownFlushesSpans`.
- `obs_test.go` — `TestPublicAPISurface` (via `go/types` reflection),
  `TestNoDirectPrometheusImportOutsideObs`,
  `TestNoDirectOtelImportOutsideObs` (via `go list -deps -json` walk).
- `bench/bench_test.go` — `BenchmarkHTTPStubBaseline`,
  `BenchmarkHTTPStubInstrumented`, `BenchmarkHTTPStubTracedSampled`
  (NFR-OBS-001).

Coverage at completion (per spec.md HISTORY): obs 86.5% / log 89.6%
/ metrics 89.7% / reqid 95.2% / trace 90.5% (all ≥85% target).

## 8. MX Tag Plan (applied — 18 tags across 5 source files)

| File::Symbol | Tag | Reason |
|--------------|-----|--------|
| `obs.go::Obs` | @MX:ANCHOR | Central bundle; fan_in ≥ 3 (cmd mains, HTTP handlers, tests) |
| `obs.go::Init` | @MX:ANCHOR | Obs lifecycle entry point; called by every cmd binary |
| `metrics.go::Registry` | @MX:ANCHOR | Shared registry passed throughout; collectors registered here |
| `metrics.go::NewRegistry` | @MX:ANCHOR | Sole constructor for the metrics Registry |
| `trace.go::Init` | @MX:WARN | OTel SDK init; failure modes around exporter connection |
| `trace.go::Tracer` | @MX:NOTE | Returns OTel tracer via global provider |
| `reqid.go::New` | @MX:NOTE | ULID generator with monotonic entropy |
| `reqid/http.go::Middleware` | @MX:ANCHOR | Sole ingress entry for request-ID propagation |
| `reqid/client.go::NewTransport` | @MX:ANCHOR | Sole egress entry for X-Request-ID write |
| `log/log.go::New` | @MX:NOTE | slog handler factory |
| `log/enrich.go::Handle` | @MX:WARN | Enriches every log record with ctx-derived attributes; runs hot |

All tags: `[AUTO]` prefix, `@MX:SPEC: SPEC-OBS-001`,
`@MX:REASON:` mandatory for ANCHOR/WARN; `code_comments: en`.

## 9. Risks Realised

| Original Risk | Outcome |
|---------------|---------|
| Cardinality explosion from M2+ labels | `TestNoUnboundedLabels` allowlist enforces in CI; new labels require both test update and code review |
| OTel SDK breaking changes | OTel v1.x stability guarantee since v1.0; Renovate weekly bumps gate on CI; no breakage to date |
| OTLP exporter blocks startup | `trace.Init` treats exporter connection as async via `BatchSpanProcessor`; failure logged, not panic |
| `/metrics` public exposure | Default binds to `127.0.0.1:9090` (admin port); production deploys responsible for binding to mesh-internal only |
| Instrumentation overhead exceeds budget | Benchmarks in CI (NFR-OBS-001); weekly scheduled run + regression gate; held under 5% |
| Loki seam adds complexity without use | V1 scope intentionally minimal: env var + hook only; no transport code |
| ULID vs UUIDv7 drift | Public header is `X-Request-ID` (format-agnostic); switching is an internal refactor |
| Sampling bias hides errors | Default 10%; configurable; future tail-sampling SPEC will add |
| `host.docker.internal` portability on Linux | `extra_hosts: "host.docker.internal:host-gateway"` added to compose for Linux compatibility |

## 10. Self-Review Outcome

Resolved Open Questions (from spec.md §11):

- **Q1 ULID vs UUIDv7** → ULID chosen (shorter 26-char header
  footprint; internal refactor available later).
- **Q2 Metric naming namespace** → `usearch_*` prefix chosen
  (brand-matched; clean separation from system metrics).
- **Q3 `/metrics` production auth** → No auth in V1; production
  auth is a deploy SPEC concern (bind to mesh-internal interface).
- **Q4 Default sampling ratio** → 0.10; env-configurable;
  revisit post-M3 with real traffic volume.
- **Q5 Exemplar sampling** → Always-on when active OTel span is
  present in ctx (default acceptable for V1 volumes).
- **Q6 `host.docker.internal` portability** → `extra_hosts: "host.
  docker.internal:host-gateway"` included in compose so Linux +
  Docker Desktop both work.

---

*End of plan.md (post-hoc).*
