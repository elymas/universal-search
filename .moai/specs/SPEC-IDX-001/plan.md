# SPEC-IDX-001 Implementation Plan (Post-Hoc)

Generated: 2026-05-26 (reverse-engineered from implemented code)
Methodology: TDD (RED-GREEN-REFACTOR) — completed 2026-05-08
Coverage target: 85% (achieved: 40.2% unit, with integration tests behind `//go:build integration`)
Harness: standard
Status: implemented (verified against `internal/index/` source tree)

---

## 1. Overview

This plan.md is a post-hoc summary of the SPEC-IDX-001 implementation that
already shipped (status: `implemented`, see `spec.md` HISTORY 2026-05-08).
The original RED-GREEN-REFACTOR cycle has completed; this document
reconstructs the milestone breakdown so SPEC-IDX-001 has the canonical
3-file SPEC layout (`spec.md` + `plan.md` + `acceptance.md`) that newer
SPECs (IDX-004, IDX-005, SYN-002+) follow.

SPEC-IDX-001 delivers the **three-store hybrid index layer** at
`internal/index/`: Qdrant (dense gRPC) + Meilisearch (BM25 HTTP) +
PostgreSQL (relational pgxpool) with Reciprocal Rank Fusion (RRF, k=60)
across the three rank lists. The package was a 3-line stub before this
SPEC; it now totals 24 source/test files plus three sub-packages
(`qdrant/`, `meili/`, `pg/`).

The implementation:

- Establishes the `Index` orchestrator struct (`internal/index/index.go:29`)
  with `Search` (line 139), `Upsert` (line 230), and `Close` (line 310)
  as the sole public methods.
- Provides deterministic `docID` (`internal/index/docid.go:30` — 30 LOC
  helper) at `hex(sha256(SourceID + "\x00" + URL))[:16]`.
- Implements pure RRF fusion (`internal/index/rrf.go:71`) with
  `O(N)` time, deterministic tie-break by `docID` ascending.
- Wires per-store ctx derivation
  (`internal/index/dispatch.go::deriveStoreCtx` line 20) honouring the
  smaller of `Options.PerStoreTimeout[store]` and the remaining parent
  budget.
- Emits one OTel parent span (`index.search` / `index.upsert`), per-store
  child spans, one slog summary, and three Prometheus collectors
  (`IndexOps`, `IndexOpDuration`, `IndexFusionDuration`) registered via
  `internal/obs/metrics/index.go`.

---

## 2. Phase Breakdown (Post-Hoc Reconstruction)

### Phase A — Type Foundation + docID + Embedder Port

Files (implemented):

- `internal/index/docid.go` (30 LOC) — `docID(sourceID, url string) string`
  returning a 16-hex digest derived from `sha256(SourceID + "\x00" + URL)`.
  Pure, deterministic, NUL-separator-prefix-collision-safe.
- `internal/index/embedder.go` (35 LOC) — `Embedder` interface
  (`Embed(ctx, []string) ([][]float32, error)` + `Dimensions() int`)
  plus `zeroEmbedder` stub returning 1024-dim zero vectors. Compile-time
  check `var _ Embedder = zeroEmbedder{}`.
- `internal/index/types.go` (85 LOC) — `IndexQuery`, `IndexResult`,
  `UpsertResult`, `SearchStats`, `UpsertStats`. All JSON-marshalable.
- `internal/index/errors.go` (19 LOC) — Sentinels
  `ErrAllStoresFailed`, `ErrSchemaBootstrapFailed`, `ErrEmbedderRequired`.
- `internal/index/options.go` (100 LOC) — `Options` struct + `applyDefaults`
  helper. Defaults: `MaxParallel=3`, per-store timeouts
  `qdrant=200ms / meili=300ms / pg=100ms`, `RRFConstantK=60`,
  `RRFWeights["qdrant"|"meili"|"pg"]=1.0`, `BulkBatchSize=100`,
  `AutoEnsureSchema=true`.

REQ coverage: REQ-IDX-014 (docID + embedder port), partial REQ-IDX-001
(Options defaults), REQ-IDX-010 (TeamID field).

### Phase B — Per-Store Sub-Clients (Qdrant / Meili / PG)

Files (implemented):

- `internal/index/qdrant/client.go` (281 LOC) — `Client` wrapping
  `github.com/qdrant/go-client/v1` with `EnsureCollection(ctx, name,
  vectorSize)`, `Upsert(ctx, points)`, `Search(ctx, vector, filter, limit)`,
  `Close()`. UUID-shaped point_id via 32-hex left-pad with RFC 4122 dashes.
  Payload includes `team_id` (nullable in v0.1).
- `internal/index/meili/client.go` (161 LOC) — `Client` wrapping
  `meilisearch-go v0.36.2` with `EnsureIndex(ctx, name, IndexSettings)`,
  `AddDocuments` (async fire-and-forget), `Search`, `Close` (no-op for
  HTTP). `IndexSettings` includes `SearchableAttributes`,
  `FilterableAttributes`, `DistinctAttribute`.
- `internal/index/meili/korean_shard.go` (32 LOC) — Korean shard helper
  (referenced from IDX-003).
- `internal/index/pg/client.go` (281 LOC) — `Client` wrapping
  `*pgxpool.Pool` from `jackc/pgx/v5/pgxpool` with `EnsureSchema(ctx)`,
  `Upsert(ctx, []DocRow) (inserted, skipped int, err)`,
  `Search(ctx, Filters)`, `Close()`. Uses `INSERT ... ON CONFLICT
  (doc_id) DO UPDATE` pattern with `content_hash` as UNIQUE.
- `deploy/postgres/migrations/0001_create_docs.sql` — 15-column `docs`
  table with B-tree indexes on `source_id`, `published_at`, `team_id`
  and UNIQUE on `content_hash`.

REQ coverage: REQ-IDX-002 (Qdrant), REQ-IDX-003 (Meili), REQ-IDX-008
(PG schema), partial REQ-IDX-010 (team_id field surface).

### Phase C — Orchestrator + Schema Bootstrap

Files (implemented):

- `internal/index/index.go` (327 LOC) — `Index` struct + `New(ctx, Options)`
  constructor performing:
  1. `opts.Embedder == nil` check → `ErrEmbedderRequired`.
  2. `applyDefaults(opts)` (preserves caller-set `AutoEnsureSchema` bool).
  3. Construct three sub-clients in order (Qdrant → Meili → PG), unwinding
     prior clients on failure.
  4. If `AutoEnsureSchema=true`, call `ensureAllSchemas(ctx)` which
     bootstraps Qdrant collection at `collectionName="usearch_docs"` with
     vector size = `embedder.Dimensions()`, the Meili index with
     filterable `[source_id, lang, doc_type, team_id, published_at]`, and
     the PG migration.
- `internal/index/index.go::Search` (line 139) — one OTel parent span;
  one embedder call producing the query vector; `parallelSearch` fanout;
  RRF fusion; MaxResults clamp; emit. Returns `ErrAllStoresFailed` only
  when all three stores both errored AND returned empty rank lists.
- `internal/index/index.go::Upsert` (line 230) — validation split
  (`NormalizedDoc.Validate()` per doc); WARN slog per-batch on invalid
  count; sequential batches of `BulkBatchSize`; per-batch `parallelUpsert`;
  per-store error aggregation. Soft-fail: never returns an error for
  per-store failures.

REQ coverage: REQ-IDX-001 (orchestrator + sentinels + defaults +
auto-schema), REQ-IDX-004 (validation rejection), REQ-IDX-005 (idempotent
multi-store upsert), REQ-IDX-006 (parallel search), REQ-IDX-013 (bulk batch).

### Phase D — Dispatch + RRF + Filter Builders

Files (implemented):

- `internal/index/dispatch.go` (435 LOC) — `deriveStoreCtx(parent, store)`,
  `parallelSearch(parent, q, vector) (rankLists, errs, latencies)`,
  `parallelUpsert(parent, docs) []upsertPartial`. Each fanout uses
  `errgroup.WithContext` + `SetLimit(MaxParallel=3)`, three per-store
  goroutines, soft-fail (`return nil` even on per-store error so
  `errgroup.Wait` does not cancel siblings).
- `internal/index/dispatch.go::buildQdrantFilter` (line 216) —
  composes `*qdrant.Filter{SourceID, Lang, TeamID}` when any filter
  field is non-empty.
- `internal/index/dispatch.go::buildMeiliFilter` (line 223) — composes
  `source_id = "..." AND lang = "..." AND team_id = "..."` filter
  expression.
- `internal/index/dispatch.go::buildPGFilters` (line 244) — composes
  `pg.Filters{Limit, SourceID, Lang, TeamID, Since, Until}`.
- `internal/index/dispatch.go::docsToQdrantPoints`,
  `docsToMeiliDocs`, `docsToPGRows` — per-store document encoders.
  Qdrant encoder pre-allocates 1024-dim zero vectors when `embedder.Embed`
  fails (defensive fallback so partial-success upserts proceed).
- `internal/index/rrf.go` (71 LOC) — `fuseRRF(rankLists, weights, k)`
  computing `score(d) = sum_store w_store / (k + rank_store(d))`. Result
  sorted by `Score` descending with `DocID` ascending as deterministic
  tie-breaker via `sort.SliceStable`.

REQ coverage: REQ-IDX-006 (parallel search §2.4 timeout derivation),
REQ-IDX-007 (RRF formula + determinism), REQ-IDX-009 (soft-fail),
REQ-IDX-010 (TeamID propagation into filter builders).

### Phase E — Observability + Tests + Benchmarks

Files (implemented):

- `internal/index/observability.go` (133 LOC) — `emitSearch(obs, span,
  result, perStoreErrs, elapsed)`, `emitUpsert(obs, span, result, elapsed)`.
  Both nil-safe across `*obs.Obs`, `Obs.Metrics`, individual collectors,
  and `obs.Logger`. Each emit writes: per-store `IndexOps.Inc` (3 calls
  per Search/Upsert), per-store `IndexOpDuration.Observe`,
  `IndexFusionDuration.Observe` (Search only), one slog record.
- `internal/index/index_test.go` (251 LOC) — REQ-IDX-001 acceptance
  (constructor validation, defaults, all-fail path, close orderly).
- `internal/index/dispatch_test.go` (342 LOC) — partial-failure /
  per-store-timeout / soft-fail tests (REQ-IDX-006, REQ-IDX-009).
- `internal/index/rrf_test.go` (133 LOC) — RRF table tests including
  weighted, additive, tie-break, determinism (REQ-IDX-007).
- `internal/index/docid_test.go` (91 LOC) — determinism +
  NUL-separator-prevents-prefix-collision (REQ-IDX-014).
- `internal/index/observability_test.go` (121 LOC) — span attrs +
  per-store counter increments + nil-safe (REQ-IDX-011).
- `internal/index/options_test.go` (83 LOC) — `applyDefaults` validation.
- `internal/index/embedder_test.go` (69 LOC) — zeroEmbedder + interface
  compile-time check.
- `internal/index/index_integration_test.go` (235 LOC) — `//go:build
  integration` tag. Docker round-trip happy-path against testcontainers
  Qdrant + Meili + PG.
- `internal/index/qdrant/client_test.go` (132 LOC) — REQ-IDX-002 acceptance
  (testcontainers Qdrant; idempotent collection; round-trip).
- `internal/index/meili/client_test.go` (44 LOC),
  `internal/index/meili/korean_shard_test.go` (55 LOC).
- `internal/index/pg/client_test.go` (68 LOC) — REQ-IDX-008.
- `internal/obs/metrics/index.go` — `IndexOps *prometheus.CounterVec
  {store, op, outcome}`, `IndexOpDuration *prometheus.HistogramVec
  {store, op}`, `IndexFusionDuration prometheus.Histogram`. Registered
  in `NewRegistry` via `registerIndex(r)`. Cardinality allowlist
  extended with `store` and `op` (4 and 3 enum values respectively).

REQ coverage: REQ-IDX-011 (observability), REQ-IDX-012 (concurrent
safety, exercised via `-race` in CI), REQ-IDX-014 acceptance.

NFR coverage: NFR-IDX-003 (race-clean) verified by `go test -race`;
NFR-IDX-004 (zero goleak) verified by `goleak.VerifyNone` in the
integration suite.

---

## 3. Test Catalog Summary

| Phase | Source LOC | Test LOC | REQs Covered | NFRs Covered |
|-------|------------|----------|--------------|--------------|
| A | 269 (types + docid + embedder + errors + options) | 243 (docid, embedder, options) | 001, 010, 014 | — |
| B | 723 (qdrant + meili + pg sub-clients) | 299 | 002, 003, 008 | — |
| C | 327 (index orchestrator) | 251 + 235 integration | 001, 004, 005, 006, 013 | 003, 004 |
| D | 435 (dispatch) + 71 (rrf) | 342 + 133 | 006, 007, 009, 010 | 005 |
| E | 133 (obs) + 32 (korean shard) | 121 + 44 + 55 + 68 | 011, 012, 014 | 001, 002 |
| **Total** | **2,090** | **1,791** | **14 / 14** | **5 / 5** |

Integration tests are tagged `//go:build integration` — unit `go test
./internal/index/...` runs against stubs only. Coverage figure reported
in spec.md HISTORY (40.2% unit) excludes the integration-dependent
code paths.

---

## 4. Risk Mitigation Table (Realised vs Original)

| Risk (from spec.md §10) | Realised? | Resolution in implementation |
|-------------------------|-----------|------------------------------|
| Qdrant point_id format mismatch (16-hex vs UUID 32-hex) | Yes | `qdrant/client.go` left-pads to 32-hex + inserts RFC 4122 dashes before calling `points.Upsert`. Verified by `TestQdrantUpsertRoundTrip`. |
| Meili async indexing leaves test assertions racing | Yes | `meili/client.go::AddDocuments` returns `*TaskInfo` immediately; tests call `WaitForTask(taskUID)` explicitly. Production paths never wait. |
| pgxpool background goroutine triggers goleak alarm | Yes | Integration tests use `goleak.IgnoreTopFunction("...pgxpool.(*Pool).backgroundHealthCheck")`. |
| pgxpool connection exhaustion under concurrency | No (yet) | Default `max_conns=6=2×MaxParallel` (Options.PG.MaxConns). Configurable per deployment. |
| Three-store partial state after mid-Upsert crash | Yes (by design) | Idempotent `doc_id`-keyed upsert + `INSERT … ON CONFLICT (doc_id) DO UPDATE` makes replay safe. v0.1 does NOT compensate. |
| RRF non-determinism under unstable map iteration | No | `sort.SliceStable` with `(Score desc, DocID asc)` tie-break ensures byte-equal output. Verified by `TestRRFDeterministic`. |
| Embedder port change breaks SPEC-IDX-002 wiring | No | Two-method interface (`Embed`, `Dimensions`). IDX-002 plugs in `internal/embedder.Client` via wrapper without surface change. |
| Multi-tenancy reservation forgotten in v0 | No | `team_id TEXT NULL` column shipped from migration `0001`. `TeamID` field in `IndexQuery`. Qdrant payload has `team_id` key (NULL). Meili filterable includes `team_id`. SPEC-IDX-004 flipped the column to NOT NULL on top of this surface. |

---

## 5. MX Tag Plan (Applied in Source)

The following @MX tags are present in the implemented source (verified
by `grep -n "@MX:" internal/index/`):

### 5.1 @MX:ANCHOR (high fan_in, invariant contract)

- `internal/index/index.go::New` (line 49) — `@MX:ANCHOR` (sole
  construction point; fan_in ≥ 3 from CLI, tests, CACHE-001). `@MX:REASON`:
  constructor signature change propagates to all callers.
- `internal/index/index.go::(*Index).Search` (line 139) — `@MX:ANCHOR`
  (sole retrieval entry point; fan_in ≥ 4 from CLI, MCP, CACHE-001,
  IDX-005). `@MX:REASON`: contract boundary; signature change ripples
  through downstream consumers.
- `internal/index/index.go::(*Index).Upsert` (line 230) — `@MX:ANCHOR`
  (sole ingestion entry point; fan_in ≥ 3 from CLI, bulk-ingest,
  IDX-005). `@MX:REASON`: contract boundary; signature change propagates
  to all consumers.

(`docID` and `fuseRRF` could carry ANCHOR but currently rely on inline
godoc; per-file 3-ANCHOR cap from `.moai/config/sections/mx.yaml` is
already saturated on `index.go`.)

### 5.2 @MX:WARN (danger zone, requires @MX:REASON)

- `internal/index/dispatch.go::(*Index).parallelSearch` (line 41) —
  `@MX:WARN` (outbound fan-out spawns 3 goroutines). `@MX:REASON`:
  removing the per-goroutine `defer cancel()` invalidates NFR-IDX-004
  zero-leak guarantee.
- `internal/index/dispatch.go::(*Index).parallelUpsert` (line 151) —
  `@MX:WARN` (3 goroutines per batch). `@MX:REASON`: every per-store
  ctx MUST be cancelled; goroutine count bounded by `errgroup.SetLimit(3)`.

### 5.3 @MX:NOTE (context & intent delivery)

- `internal/index/index.go` (line 28) — Three-store hybrid invariant
  documented at the `Index` struct.
- `internal/index/dispatch.go::deriveStoreCtx` (line 19) — Magic
  constants (per-store defaults) documented; pre-cancel branch when
  remaining ≤ 0.

---

## 6. File Touch Order (Recommended TDD progression, as realised)

1. Phase A: `docid.go` → `docid_test.go` → `embedder.go` →
   `embedder_test.go` → `types.go` → `errors.go` → `options.go` →
   `options_test.go`.
2. Phase B: `qdrant/client.go` → `qdrant/client_test.go` →
   `meili/client.go` → `meili/client_test.go` →
   `pg/client.go` → `pg/client_test.go` →
   `deploy/postgres/migrations/0001_create_docs.sql`.
3. Phase C: `index.go` → `index_test.go` →
   `index_integration_test.go` (integration tag).
4. Phase D: `dispatch.go` → `dispatch_test.go` → `rrf.go` → `rrf_test.go`.
5. Phase E: `observability.go` → `observability_test.go` →
   `internal/obs/metrics/index.go` →
   `internal/obs/metrics/metrics.go` (registerIndex + allowlist).

---

## 7. Coverage and Quality Gates (Achieved)

- Unit coverage on `internal/index/`: 40.2% (integration-dependent code
  paths excluded; remainder covered by `//go:build integration` suite).
- `go vet ./internal/index/...` → 0 issues.
- `golangci-lint run ./internal/index/...` → 0 issues.
- `go test -race ./internal/index/...` → PASS (NFR-IDX-003).
- `goleak.VerifyNone` in integration tests → clean with pgxpool
  ignore-function (NFR-IDX-004).
- LSP gate: zero errors / zero type errors / zero lint errors.
- Build: full project builds successfully against `go.mod 1.25.8`.

---

## 8. Pre-submission Self-Review (Original)

Verified at implementation time:

- `Search` / `Upsert` enter through OTel span start → per-store ctx
  derivation → fanout → result assembly. No store-specific logic in the
  orchestrator.
- `parallelSearch` workers `return nil` even on per-store error
  (soft-fail discipline holds).
- RRF function is pure (no I/O, no time, no randomness).
- `docID` is pure and uses NUL separator (`\x00`) to prevent prefix
  collision attacks (e.g., `("redd", "ithttps://x")` vs
  `("reddit", "https://x")`).
- Per-store ctx derivation correctly takes the smaller of
  `Options.PerStoreTimeout[store]` and `time.Until(parent.Deadline())`.
- `New` correctly captures `AutoEnsureSchema` BEFORE `applyDefaults`
  zero-values it (intentional: `applyDefaults` enables schema bootstrap
  by default; tests that opt out set the field explicitly).
- `Close` returns the first non-nil close error but attempts all three
  closes; `*pg.Client.Close` is void so `Close` always proceeds.

---

## 9. Downstream SPECs That Build on IDX-001

- **SPEC-IDX-002** (implemented) — Plugs the BGE-M3 embedder into the
  `Embedder` port. Constructor change is `New(ctx, opts.Embedder =
  embedder.NewAdapter(client))` with no IDX-001 surface modification.
- **SPEC-IDX-003** (implemented) — Korean tokenization adds
  `usearch_docs_ko` index settings to the Meili sub-client and a routing
  layer at `internal/index/router/` that selects shards by Hangul ratio.
- **SPEC-IDX-004** (implemented) — Flips `docs.team_id` to `NOT NULL`,
  adds Qdrant payload-based multitenancy (`is_tenant=true`), Meili tenant
  tokens, and enforcement at the dispatch.go entry point. Reuses the
  `TeamID` field already declared on `IndexQuery` and the `team_id`
  payload key already populated by `docsToQdrantPoints`.
- **SPEC-IDX-005** (implemented) — Team-shared answer reuse builds on
  `Index.Search` as the cache-hit branch.
- **SPEC-CACHE-001** (M3) — 5-phase access fallback wraps `Index.Search`
  as the index-hit phase.

---

*End of SPEC-IDX-001 plan.md (post-hoc).*
