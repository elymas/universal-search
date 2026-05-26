# SPEC-OBS-001 Acceptance — Given/When/Then Scenarios

Created: 2026-04-24
Updated: 2026-04-26 (post-implementation reconciliation)
Author: limbowl (via manager-spec)
Status: implemented

## 0. Document Purpose

Given/When/Then acceptance scenarios for SPEC-OBS-001 — the
observability baseline (slog + Prometheus + OTel + request-ID
propagation) consumed by every M2+ domain package.

## 1. Coverage Matrix

| AC | Scenario | REQs covered |
|----|----------|--------------|
| AC-001 | slog JSON handler with LOG_LEVEL env binding | REQ-OBS-001 |
| AC-002 | X-Request-ID propagation across ingress/ctx/egress/slog | REQ-OBS-002 |
| AC-003 | HTTP middleware updates counter + histogram per request | REQ-OBS-003 |
| AC-004 | Fanout gauge Inc/Dec returns to baseline | REQ-OBS-003 |
| AC-005 | Adapter call outcome labels bounded to {success, failure, timeout} | REQ-OBS-003 |
| AC-006 | /metrics endpoint exposes all baseline collectors | REQ-OBS-004 |
| AC-007 | OTel init: no-op when endpoint unset; real exporter when set | REQ-OBS-005 |
| AC-008 | Public API surface stable; import-boundary enforced | REQ-OBS-006 |
| AC-009 | LOKI_ENDPOINT env reserved + RegisterHandler hook | REQ-OBS-007 |
| NFR-001 | Instrumentation overhead < 5% (sampled < 8%) | NFR-OBS-001 |
| NFR-002 | Cardinality safety — allowlist enforced | NFR-OBS-002 |
| NFR-003 | Reproducibility — pinned deps + static scrape config | NFR-OBS-003 |

## 2. Definition of Done

- [x] All 7 EARS REQs (6 P0 + 1 P2) have green tests.
- [x] All 3 NFRs validated.
- [x] Coverage ≥ 85% (achieved: obs 86.5% / log 89.6% / metrics 89.7%
      / reqid 95.2% / trace 90.5%).
- [x] `go test -race ./internal/obs/...` clean.
- [x] Public API surface verified via `TestPublicAPISurface`
      (Init, Config, Logger, Tracer, WithRequestID, RequestID,
      HTTPRequests, HTTPRequestDuration, FanoutInflight, AdapterCalls,
      AdapterCallDuration, BuildInfo).
- [x] `TestNoDirectPrometheusImportOutsideObs` and
      `TestNoDirectOtelImportOutsideObs` pass.
- [x] `TestNoUnboundedLabels` passes with the documented allowlist.
- [x] Compose stack includes `prometheus` service with healthcheck.
- [x] `.env.example` documents all 6 new observability vars.
- [x] `docs/dependencies.md` updated with new direct deps (regen
      via SPEC-DEP-001's manifest generator).
- [x] TRUST 5 gates green; 18 @MX tags applied across 5 source files.

## 3. Functional Scenarios

### AC-001 — Structured logging

Maps to REQ-OBS-001.

#### AC-001.1: JSON output

- **Given** a logger constructed with `log.New(w, level)`.
- **When** the logger emits any record.
- **Then** captured handler output parses as JSON; contains `time`,
  `level`, `msg` (stdlib slog JSON keys).

#### AC-001.2: LOG_LEVEL env binding

- **Given** `LOG_LEVEL` is one of `{"", "DEBUG", "INFO", "WARN",
  "ERROR", "garbage"}`.
- **When** `LevelFromEnv` is called.
- **Then** the parsed level is `Debug/Info/Warn/Error` for the
  matching keys; default `INFO` for `""` and `"garbage"`.

#### AC-001.3: Level filtering

- **Given** level set to `WARN`.
- **When** the logger emits records at all four levels.
- **Then** DEBUG and INFO records are NOT in captured output; WARN
  and ERROR records ARE present.

### AC-002 — Request ID propagation

Maps to REQ-OBS-002.

#### AC-002.1: ctx propagation across goroutine

- **Given** `WithRequestID(ctx, "X")`.
- **When** a goroutine reads `reqid.FromContext(derivedCtx)`.
- **Then** the value is `"X"`.

#### AC-002.2: ingress generates ULID when absent

- **Given** an inbound HTTP request without `X-Request-ID`.
- **When** wrapped by `reqid.Middleware`.
- **Then** the response header has a 26-char ULID matching regex
  `^[0-9A-HJKMNP-TV-Z]{26}$`.

#### AC-002.3: ingress preserves when present

- **Given** an inbound request with `X-Request-ID: REQ-FIXED-123`.
- **Then** the response header equals `REQ-FIXED-123`.

#### AC-002.4: egress writes header

- **Given** an `http.Client{Transport: reqid.NewTransport(...)}` and a
  ctx with `WithRequestID(ctx, "EGRESS-42")`.
- **When** the client sends a request to a stub upstream.
- **Then** the upstream receives `X-Request-ID: EGRESS-42`.

#### AC-002.5: slog enrichment

- **Given** the enriched slog handler and a ctx with request_id set.
- **When** the logger emits a record.
- **Then** captured JSON contains `"request_id":"..."` matching the
  ctx value.
- **And** when an active OTel span is present, the record also
  contains `trace_id` and `span_id`.

### AC-003 — HTTP metric events

Maps to REQ-OBS-003.

#### AC-003.1: counter increments

- **Given** the HTTP middleware wrapping a stub handler.
- **When** 3 requests are made.
- **Then** `HTTPRequests.WithLabelValues("GET", "/x", "2xx")` counter
  delta == 3.

#### AC-003.2: histogram records duration

- **Then** `HTTPRequestDuration` histogram sample count delta == 3;
  observed bucket sum > 0; values within ±10% of measured elapsed.

#### AC-003.3: fanout gauge Inc/Dec

- **Given** the `FanoutInflight` gauge.
- **When** `Inc()` then `Dec()` is called.
- **Then** the gauge returns to its baseline value (delta == 0).

#### AC-003.4: adapter outcome labels

- **Given** the `AdapterCalls` counter with allowed labels.
- **When** a label outside `{success, failure, timeout}` is
  attempted.
- **Then** the static analysis test `TestNoUnboundedLabels` fails
  (allowlist enforcement at test time, not runtime panic).

### AC-004 — Admin /metrics endpoint

Maps to REQ-OBS-004.

#### AC-004.1: 200 + content-type

- **Given** the admin server bound to a random port.
- **When** GET `/metrics` is requested.
- **Then** status 200; `Content-Type` begins with `text/plain`
  (specifically `text/plain; version=0.0.4`); body non-empty.

#### AC-004.2: all baseline families present

- **Then** the response body contains substring matches for:
  - `usearch_http_requests_total`
  - `usearch_http_request_duration_seconds`
  - `usearch_fanout_goroutines_inflight`
  - `usearch_adapter_calls_total`
  - `usearch_adapter_call_duration_seconds`
  - `usearch_build_info`

#### AC-004.3: admin port configurable

- **Given** `USEARCH_ADMIN_PORT=19090` at Init.
- **Then** the listener is bound to port 19090.
- **And** with `addr=":0"`, the actual bound port is reported via
  `Obs.AdminAddr` (> 0).

### AC-005 — OTel initialization

Maps to REQ-OBS-005.

#### AC-005.1: real exporter when endpoint set

- **Given** `cfg.OTLPEndpoint = "localhost:4317"`.
- **When** `trace.Init(ctx, cfg)` runs.
- **Then** the global tracer provider is NOT a no-op; the propagator
  is the composite `TraceContext + Baggage`.

#### AC-005.2: no-op when endpoint unset

- **Given** `cfg.OTLPEndpoint = ""`.
- **Then** the global tracer provider is `noop.NewTracerProvider()`;
  zero outbound bytes (verified via test-side gRPC stub observer).

#### AC-005.3: sample ratio from env

- **Given** `OTLP_SAMPLE_RATIO=0.5`.
- **When** 10,000 synthetic traces are emitted.
- **Then** the sampled count is in `[4500, 5500]` (±5% tolerance).

#### AC-005.4: shutdown flushes spans

- **Given** an in-memory exporter (`sdktrace/tracetest.NewInMemoryExporter`).
- **When** a span is started, ended, and `shutdown(ctx)` is called.
- **Then** the exporter observes the span (BatchSpanProcessor
  flushed).

### AC-006 — Public API surface

Maps to REQ-OBS-006.

#### AC-006.1: symbol presence

- **When** `TestPublicAPISurface` runs via `go/types.Lookup`.
- **Then** each named symbol exists with the expected kind:
  - Functions: `Init`, `Logger`, `Tracer`, `WithRequestID`,
    `RequestID`
  - Types: `Config`, `Obs`
  - Variables (re-exports from `metrics.Registry`): `HTTPRequests`,
    `HTTPRequestDuration`, `FanoutInflight`, `AdapterCalls`,
    `AdapterCallDuration`, `BuildInfo`

#### AC-006.2: prometheus import boundary

- **When** `TestNoDirectPrometheusImportOutsideObs` walks
  `go list -deps -json` output.
- **Then** any non-obs package depending on
  `github.com/prometheus/client_golang/...` fails the test.

#### AC-006.3: otel import boundary

- **Analogous to AC-006.2** for `go.opentelemetry.io/otel/...`.

### AC-007 — Loki seam (REQ-OBS-007, P2)

- **Given** `LOKI_ENDPOINT=http://loki:3100` at Init.
- **Then** no error; captured boot log contains the seam
  announcement message `"loki endpoint reserved; transport not yet
  implemented"`.
- **And** `log.RegisterHandler(customHandler)` causes subsequent
  records to be tee'd to `customHandler` (verified via buffered test
  handler).

## 4. Non-Functional Acceptance

### NFR-OBS-001 — Performance budget

- `internal/obs/bench/bench_test.go` defines:
  - `BenchmarkHTTPStubBaseline` — no middleware
  - `BenchmarkHTTPStubInstrumented` — full stack with OTLP unset
  - `BenchmarkHTTPStubTracedSampled` — OTLP set with sample ratio 0.1
- p99 overhead `(Instrumented - Baseline) / Baseline` < 0.05.
- Sampled-tracing p99 overhead < 0.08.
- Benchmarks run in CI on scheduled weekly job (not per-PR).

### NFR-OBS-002 — Cardinality safety

- `TestNoUnboundedLabels` is a static analysis test that walks all
  `*prometheus.{Counter,Gauge,Histogram,Summary}Vec` registrations in
  `internal/obs/metrics/`.
- For each label name, asserts presence in the allowlist map.
- Allowlist (baseline): `{method, route, status_class, adapter_class,
  adapter, outcome, version, commit, go_version}`.
- Extended by downstream SPECs (`provider`, `model`, `phase`, ...);
  every extension requires a test update.

### NFR-OBS-003 — Reproducibility

- All new Go direct deps pinned to exact minor versions in `go.mod`.
- `docs/dependencies.md` (via SPEC-DEP-001) lists the new deps.
- `deploy/prometheus/prometheus.yml` uses `static_configs` only (no
  DNS, no `file_sd`); scrape targets known at deploy time.

## 5. Edge Cases

### EC-001 — Metric cardinality from M2+ packages

- A new SPEC adds a label without updating the allowlist.
- `TestNoUnboundedLabels` fails the CI run for that SPEC's PR; the
  reviewer either approves the cardinality increase (and the
  allowlist is updated) or pushes back on the label design.

### EC-002 — OTLP gRPC exporter unreachable

- `OTLPEndpoint` is set but the endpoint refuses connection.
- `BatchSpanProcessor` buffers; periodic flush logs export errors at
  WARN; process continues; no startup block.

### EC-003 — `/metrics` exposed publicly

- Documented mitigation: admin port defaults to
  `127.0.0.1:9090`; production deploys bind to mesh-internal
  interfaces only.

### EC-004 — Goroutine leak in HTTP middleware

- The middleware does NOT spawn goroutines; it wraps the handler
  synchronously. No leak path.

### EC-005 — `obs.Init` partial failure

- Admin server starts; OTel init fails.
- `Init` returns `(nil, nil, err)` after invoking
  `shutdownAdmin(ctx)` to release the bound port.

### EC-006 — Shutdown called twice

- `shutdown(ctx)` is idempotent via the `shutdownCalled` flag;
  subsequent calls return nil.

### EC-007 — Adapter call counter with non-allowlisted outcome

- The static analyser at test time catches the violation; runtime is
  not panicked (we don't want production to crash on cardinality
  error).

### EC-008 — `host.docker.internal` on Linux

- Prometheus service config includes
  `extra_hosts: "host.docker.internal:host-gateway"` so the same
  scrape config works on Docker Desktop (macOS/Windows) and Linux.

### EC-009 — Slog enrichment with nil ctx

- `reqid.FromContext(nil)` returns empty string; the enrichment
  handler skips the attribute when empty; no panic.

### EC-010 — Histogram exemplar without OTel span

- `WithExemplar` is called only when an active span is present in
  ctx; otherwise, plain `Observe` is used (no exemplar attached).

## 6. Quality Gate Criteria

| Criterion | Threshold | Source |
|-----------|-----------|--------|
| Coverage (`internal/obs/`) | ≥ 85% (achieved 86.5%) | quality.yaml |
| Coverage (`log/`) | ≥ 85% (achieved 89.6%) | quality.yaml |
| Coverage (`metrics/`) | ≥ 85% (achieved 89.7%) | quality.yaml |
| Coverage (`reqid/`) | ≥ 85% (achieved 95.2%) | quality.yaml |
| Coverage (`trace/`) | ≥ 85% (achieved 90.5%) | quality.yaml |
| `go vet ./internal/obs/...` | clean | go.md |
| `golangci-lint run` | zero issues | go.md |
| `go test -race ./internal/obs/...` | clean | NFR-OBS-001 |
| `TestNoUnboundedLabels` | passes with allowlist | NFR-OBS-002 |
| `TestNoDirectPrometheusImportOutsideObs` | passes | REQ-OBS-006 |
| `TestNoDirectOtelImportOutsideObs` | passes | REQ-OBS-006 |
| Bench p99 overhead (unsampled) | < 5% | NFR-OBS-001 |
| Bench p99 overhead (sampled 10%) | < 8% | NFR-OBS-001 |
| 18 @MX tags applied | yes | plan.md §8 |
| TRUST 5 gates | all green | constitution |

## 7. Out-of-Scope Confirmations

Restated from spec.md §2.2:

- Grafana dashboards / provisioning → SPEC-EVAL-002 (M8)
- Loki service / Promtail sidecar → future deploy SPEC
- OTel Collector service in compose → developer-optional, not shipped
- Alerting rules / Alertmanager → post-V1 SPEC
- SLO definitions / burn-rate alerts → post-V1 SPEC
- OTel metrics API / Prometheus bridge exporter → explicit
  architectural non-goal (metrics stay on client_golang)
- Python services' observability → owned by each service's SPEC
- Web UI RUM → future Web UI SPEC
- Log retention / index rotation / storage cost control →
  infrastructure concerns (deploy SPEC)

---

*End of acceptance.md (post-hoc).*
