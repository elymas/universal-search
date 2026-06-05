# SPEC-API-001 Research

Ground-truth investigation backing the SPEC. All file/line citations below were
verified by reading the current `main` working tree on 2026-06-04.

## 1. Stale-reference correction (CRITICAL)

`cmd/usearch-api/main.go` carries a misleading SPEC reference:

- Line 2 (package comment): `// Full implementation lands in SPEC-IR-001.`
- Line 50 (stderr): `usearch-api: not implemented (see SPEC-IR-001)`
- Lines 39, 45, 48 also attribute "full server mux registration" / "server.ListenAndServe"
  to SPEC-IR-001.

**This is wrong.** SPEC-IR-001 is "Intent Router v0", an explicitly **library-only**
SPEC. Confirmed from `.moai/specs/SPEC-IR-001/spec.md`:

- L26–27: "M2 entry-point SPEC; library-only exposure (no HTTP endpoint)."
- L136–139 (Out of Scope): "HTTP / gRPC endpoint exposure. The Router is a Go
  library ... SPEC-API-001 for HTTP, SPEC-MCP-001 (M7) for MCP."
- L468: "Cmd `usearch-api` and `usearch-mcp` mains — IR-001 is library only;
  HTTP/MCP exposure is out of scope."

So SPEC-IR-001 itself already names **SPEC-API-001** as the owner of the HTTP
server. The HTTP API server had **no** SPEC until now. SPEC-API-001 owns it.
REQ-API-015 mandates correcting the stale comment/stderr string to `SPEC-API-001`
during implementation. (The earlier `c2c6fd2 docs: map existing codebase` commit
did not catch this drift.)

## 2. Stub state (the problem)

`cmd/usearch-api/main.go`:
- L23–29: `obs.Init` with service name `"usearch-api"`, reading `LOG_LEVEL`,
  `OTLP_ENDPOINT`, admin addr from `USEARCH_ADMIN_PORT`.
- L40–41: builds a mux, registers `/query/stream` with
  `handlers.NewSynthesisHandler(nil, ...)` — **nil synthesis client**.
- L45–46: `reg := adapters.NewRegistry(nil)` — **empty registry**, then mounts
  admin routes (loopback-wrapped).
- L48–51: `_ = mux`, prints "not implemented", `os.Exit(0)`. **Never ListenAndServe.**

Net effect: the frontend has no backend. The only working search path is the CLI.

## 3. Frontend contract (must satisfy)

`web/src/lib/api.ts`:
- Base URL: `NEXT_PUBLIC_API_URL || "http://localhost:8080"`.
- `searchQuery` → `GET /api/query?q=&sources=` → `SearchResult`
  `{answer, citations[], query, sources_used[], elapsed_ms}`.
- `searchStream` → `EventSource(GET /api/query/stream?q=&sources=)`.
- `fetchSources` → `GET /api/sources` → `AdapterInfo[]`
  `{name, category, enabled, latency_ms?}`.
- `fetchHistory` → `GET /api/history` → array of `{query, timestamp, id}`.
- Citation shape: `{index, title, url, snippet, source}`.

`web/src/lib/sse-client.ts`:
- Listens for events `sentence`, `citation`, `complete`, `error`; parses JSON
  payloads. `complete` reads `parsed.elapsed_ms`; `error` reads `parsed.message`;
  `sentence` reads `parsed.text`; `citation` is parsed as a `Citation`.
- Reconnect backoff: max 5 attempts, `1000 * 2^n` ms (1–16s observed; capped at 5).

**Route-prefix mismatch:** the stub registered bare `/query/stream`, but the
frontend calls `/api/query/stream`. SPEC adopts the frontend `/api/...` prefix as
canonical (Decision Point D3).

**Event-name mismatch:** streamsynth emits `event: done`; the frontend listens for
`complete`. The server must map `done → complete` (REQ-API-009). The frontend has
no `done` listener, so an unmapped server would silently never fire completion.

## 4. The working CLI core to reuse

`cmd/usearch/query.go` `Execute` (L111) is the proven pipeline:
- L162–166: registry — `buildProductionRegistry()` (L458) registers reddit, hn,
  arxiv, github (token-gated), youtube (env-gated), searxng, bluesky, naver,
  koreanews. Uses `adapters.NewRegistry(nil)` + `reg.Register(a)`.
- L179: `buildRouter(reg)` (L518) → `router.New(router.Options{Registry: reg})`.
- L186–192: `rtr.Classify(spanCtx, router.RouterQuery{Query: types.Query{Text: prompt}})`
  → `decision` with `Category`, `AdapterSet`, `Lang`.
- L195: `intersectSources(decision.AdapterSet, flags.Source)` — source-filter logic.
- L220: `fanout.New(fanout.Options{Registry: reg})`.
- L228: `f.Dispatch(spanCtx, fanoutDecision, types.Query{Text: prompt})` →
  `fanout.Result{Docs, AdapterErrors, Stats}` (`internal/fanout/result.go`).
- L252–258: `buildProductionSynth()` (L533) → `synth.Synthesize(spanCtx, prompt,
  decision.Lang, docs)`. Falls back to `nopSynthClient` when config/construction
  fails (degraded mode, REQ-CLI-009).
- L147–148: pipeline deadline via `context.WithTimeout`; L143–144 request ID via
  `reqid`.

## 5. The cmd-package import barrier (key risk)

`buildProductionRegistry`, `buildRouter`, `buildProductionSynth` are all in
`package main` under `cmd/usearch/`. **A Go `main` package cannot be imported by
another `main` package**, so `cmd/usearch-api/` cannot call them. They must be
**extracted into a shared `internal/` package** (e.g. `internal/searchpipe/`).
This is the single most important plan item (Decision Point D2 / plan M0). After
extraction, `cmd/usearch` is refactored to consume it so the two entry points share
one source of truth (NFR-API-001).

### Synthesis seam detail

Two `Synthesize` signatures coexist:
- CLI-local `synthClientIface`: `Synthesize(ctx, query, lang string, docs
  []types.NormalizedDoc) (synthResult, error)` — the CLI maps `synthesis.Result`
  into a local `synthResult`/`synthCitation` (`query.go` L554–568).
- `handlers.SynthesisClient` (`handlers/synthesis.go` L22–25): `Synthesize(ctx
  interface{}, req synthesis.Request) (synthesis.Result, error)`.

The shared package should expose the concrete `*synthesis.Client` (or an interface
over `Synthesize(ctx, query, lang, docs) (synthesis.Result, error)` — note the real
client returns `synthesis.Result`, see `internal/synthesis/client.go` L61 and
`internal/synthesis/types.go`) so both handlers can adapt without re-wiring.

## 6. Wire types reused

- `internal/streamsynth/streamsynth.go`: `SentencePayload{request_id,
  sentence_index, text, citations[], schema_version}`, `DonePayload{... total_sentences,
  latency_ms, model, provider, cost_usd ...}`, `ErrorPayload{... error_code,
  error_message ...}`. `StreamSynthesize(ctx, w, StreamRequest{RequestID, SynthResult})`
  emits `event: sentence` / `event: done` / `event: error`. Honors `ctx.Done()`
  (client disconnect). Skips uncited sentences (REQ-SYN4-001c).
- `internal/synthesis/types.go`: `Request`, `Result{text, citations[], model,
  provider, cost_usd, latency_ms, degraded, notice}`, `Citation{marker, doc_id,
  url, title}`.
- `pkg/types/normalized_doc.go` L40–56: `NormalizedDoc` — the shared result contract.

## 7. Sources endpoint mapping

`internal/adapters/registry.go` exposes:
- `List() []string` (L185) — registered adapter names.
- `Get(name) (types.Adapter, bool)` (L175) — to read `Capabilities()`.
- `SnapshotForAdmin() []AdapterAdminView` (L251) — **admin-flavored**, includes
  status/secret-source/key-set; gated behind loopback for admin only.

`/api/sources` is **public** and needs only `{name, category, enabled,
latency_ms?}`. It should be built from `List()` + per-adapter `Capabilities()`
(`pkg/types/capabilities.go` L38), NOT from `SnapshotForAdmin()` (which carries
admin/secret metadata). NFR-API-005: never leak secret values.

## 8. Deployment

- `deploy/Dockerfile.usearch-api`: distroless, non-root `65532`, `EXPOSE 8080 9090`,
  `HEALTHCHECK CMD ["/usearch-api", "--healthcheck"]` — the `--healthcheck` flag is
  **not implemented** today (REQ-API-004 implements it). Builds Go 1.24 from
  `./cmd/usearch-api`.
- `deploy/docker-compose.yml`: `usearch-api` is **absent**. Existing relevant
  services: `researcher` (synthesis sidecar, port 8081, healthcheck `/health`,
  depends on `litellm`), `searxng` (port 8080, depends on `redis`). The new
  `usearch-api` block should `depends_on` researcher (`service_healthy`) and
  searxng, wiring `RESEARCHER_BASE_URL`, `USEARCH_SEARXNG_URL`, `LOG_LEVEL`,
  `OTLP_ENDPOINT`. Note both searxng and usearch-api want host port 8080 — pick
  distinct host port mappings in compose.

## 9. Deferred (handlers exist but out of scope)

- `cmd/usearch-api/handlers/deep.go` — exists, **not registered**, nil deps. Deferred.
- `internal/api/admin/*` — adapters listing/toggle/resync/audit; mounted in the
  stub behind `LoopbackOnly`. The audit handler takes a **nil** `AuditQuerier`
  (main.go L81–83 TODO: "wire a real AuditQuerier when audit store gains a
  QueryEntries method"). Admin + audit deferred to a follow-up SPEC.

## 10. SPEC-ID uniqueness

`SPEC-API-001` is not present in `.moai/specs/` (verified by directory listing).
Dependencies referenced — `SPEC-IR-001`, `SPEC-FAN-001`, `SPEC-SYN-004`,
`SPEC-CORE-001` — all exist as directories.
