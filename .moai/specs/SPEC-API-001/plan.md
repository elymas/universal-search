# SPEC-API-001 Implementation Plan

## Technical Approach

Make `cmd/usearch-api` a real HTTP server that reuses the CLI's proven search
pipeline. The work splits into a prerequisite refactor (extract the shared
assembly) and then endpoint wiring, lifecycle, and deployment.

## Key Engineering Risk & Required Refactor

### The cmd-package import barrier (Decision Point D2)

The proven pipeline assembly lives in `cmd/usearch/query.go`:

- `buildProductionRegistry()` — registers 9+ adapters with env gating
  (reddit, hn, arxiv, github, youtube, searxng, bluesky, naver, koreanews).
- `buildRouter(reg)` → `router.New(router.Options{Registry: reg})`.
- `buildProductionSynth()` → `synthesis.New(...)` to the researcher sidecar, with
  a `nopSynthClient` degraded fallback.

These are in `package main` under `cmd/usearch/`. **Go does not allow one `main`
package to import another**, so `cmd/usearch-api/` cannot reuse them directly.

**Recommendation:** extract the pipeline assembly into a new shared package, e.g.
`internal/searchpipe/` (name TBD during run), exposing:

- `BuildProductionRegistry() *adapters.Registry`
- `BuildRouter(reg *adapters.Registry) (*router.Router, error)`
- `BuildProductionSynth() SynthClient` (interface, not the cmd-local `synthClientIface`)
- optionally a single `Build(ctx) (*Pipeline, error)` that returns all three plus a
  `Run(ctx, query, sources)` convenience used by both the HTTP handler and the CLI.

Then `cmd/usearch/query.go` is refactored to call the extracted package (its local
helpers become thin wrappers or are deleted), guaranteeing NFR-API-001 (single
source of truth). This refactor MUST keep the existing CLI tests green — it is a
behavior-preserving move, validated by the CLI's existing suite before any HTTP
code is added.

The synthesis seam needs care: the CLI uses a cmd-local `synthClientIface`
(`Synthesize(ctx, query, lang, docs) (synthResult, error)`) while the streaming
handler uses `handlers.SynthesisClient` (`Synthesize(ctx interface{}, req
synthesis.Request) (synthesis.Result, error)`). The extracted package should
expose the concrete `*synthesis.Client` (or an interface over its
`Synthesize(ctx, query, lang, docs) (synthesis.Result, ...)` signature) so both the
buffered JSON handler and the SSE handler can adapt to it without re-deriving wiring.

## Milestones (priority-ordered, no time estimates)

### M0 — Shared pipeline extraction (Priority: High, blocks all else)
- Create `internal/searchpipe/` (or equivalent) with the three build helpers moved
  verbatim from `cmd/usearch/query.go`.
- Refactor `cmd/usearch/query.go` to consume the extracted package.
- Run the existing CLI test suite — must stay green (characterization gate).
- Covers REQ-API-002, NFR-API-001.

### M1 — Server lifecycle (Priority: High)
- Replace the stub `main()`: build the pipeline, mount routes, `ListenAndServe` on
  `USEARCH_API_PORT` (default `:8080`).
- Add `--healthcheck` flag handling (probe + exit) and a `GET /healthz` route.
- Add `SIGINT`/`SIGTERM` graceful shutdown with a bounded drain + `obs` shutdown.
- Correct the stale `SPEC-IR-001` package comment and remove the
  "not implemented (see SPEC-IR-001)" stderr line.
- Covers REQ-API-001, REQ-API-003, REQ-API-004, REQ-API-005, REQ-API-014, REQ-API-015.

### M2 — Buffered search endpoint (Priority: High)
- Implement `GET /api/query`: parse `q`/`sources`, run Classify → Dispatch →
  Synthesize via the shared pipeline, marshal to the frontend `SearchResult` shape.
- Map `synthesis.Result` + fanout docs into `{answer, citations[], query,
  sources_used[], elapsed_ms}`; citation `{index, title, url, snippet, source}`.
- Source-filter intersection + `400` on unknown adapter.
- Degraded-mode behavior on synthesis failure.
- Covers REQ-API-006, REQ-API-007, REQ-API-010, REQ-API-013.

### M3 — Streaming search endpoint (Priority: High)
- Register `GET /api/query/stream` (replacing the stub bare `/query/stream`).
- Reuse `handlers.SynthesisHandler` / `streamsynth.StreamSynthesize`, wiring a
  **real** synthesis client (today it is nil).
- Adapt the request: frontend uses a `GET` with `q`/`sources` query params and
  EventSource; the existing handler decodes a JSON POST body — reconcile by adding
  a GET entry that builds the synthesis request from query params then runs the
  pipeline, or by an adapter that pre-fills docs from Dispatch before streaming.
- Map streamsynth event names (`sentence`/`done`/`error`) → frontend names
  (`sentence`/`citation`/`complete`/`error`) per REQ-API-009.
- Covers REQ-API-008, REQ-API-009, REQ-API-010, REQ-API-013.

### M4 — Sources + history endpoints (Priority: Medium)
- `GET /api/sources`: map `registry.List()` + per-adapter `Capabilities()` into
  `{name, category, enabled, latency_ms?}` (no secret leakage, NFR-API-005).
- `GET /api/history`: return `[]` with 200.
- Covers REQ-API-011, REQ-API-012.

### M5 — Deployment (Priority: Medium)
- Add a `usearch-api` service block to `deploy/docker-compose.yml`: build from
  `deploy/Dockerfile.usearch-api`, expose `8080`, `depends_on` researcher
  (`service_healthy`) and searxng, wire `RESEARCHER_BASE_URL`,
  `USEARCH_SEARXNG_URL`, `LOG_LEVEL`, `OTLP_ENDPOINT`, etc.
- Verify the Dockerfile `HEALTHCHECK --healthcheck` now resolves against REQ-API-004.
- Covers REQ-API-016.

### M6 — Tests (Priority: High, runs alongside M2–M4)
- `httptest`-based handler tests using fake registry/router/synthesis doubles
  (the synthesis fake already has precedent: `handlers.SynthesisClient` interface
  and the CLI's `withSynth`/`withRegistry` injection options).
- Cover: buffered query happy path + unknown-adapter 400, SSE event-name mapping,
  degraded synthesis, `/api/sources` shape, `/api/history` empty, healthcheck flag.
- No live network in CI.
- Covers REQ-API-017 and acceptance scenarios.

## Files Touched (anticipated; finalized in run)

| Path | Change |
|------|--------|
| `internal/searchpipe/` (new) | Extracted pipeline assembly (M0) |
| `cmd/usearch/query.go` | Refactor to consume extracted package (M0) |
| `cmd/usearch-api/main.go` | Real server, lifecycle, healthcheck, route mounts, stale-ref fix (M1–M4) |
| `cmd/usearch-api/handlers/` | Query + sources + history handlers; wire real synth into SSE handler (M2–M4) |
| `cmd/usearch-api/*_test.go` | httptest handler tests (M6) |
| `deploy/docker-compose.yml` | usearch-api service block (M5) |

## Risks

- **R1 — Refactor regression:** moving CLI helpers could break CLI behavior. Mitigate
  by running the full CLI suite as a gate before HTTP work (M0 exit criterion).
- **R2 — Synthesis seam mismatch:** two different `Synthesize` signatures exist
  (cmd-local vs handler). Mitigate by exposing the concrete `*synthesis.Client` from
  the shared package and adapting at each call site.
- **R3 — GET-vs-POST streaming mismatch:** frontend EventSource is `GET` with query
  params; the existing SSE handler reads a JSON POST body. Mitigate with a GET entry
  that assembles the synthesis request from the pipeline (Dispatch docs) before
  delegating to `streamsynth`.
- **R4 — Event-name drift:** streamsynth emits `done`; frontend listens for
  `complete`. The mapping (REQ-API-009) must be covered by a test (M6) to prevent
  silent breakage.
- **R5 — Admin coupling:** the stub also mounts admin routes with a nil registry.
  Keep them mounted against the real registry but out of acceptance scope; do not
  let admin wiring block the search path.

## @MX Tag Targets

- `internal/searchpipe.Build*` — high fan_in (CLI + API) → `@MX:ANCHOR`.
- New server `main()` graceful-shutdown goroutine → `@MX:WARN` (goroutine + ctx).
- SSE GET handler → `@MX:ANCHOR` (frontend-facing boundary).
