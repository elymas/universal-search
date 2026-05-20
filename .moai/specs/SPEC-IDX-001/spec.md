---
id: SPEC-IDX-001
title: Hybrid Index Layer (Qdrant + Meilisearch + PostgreSQL)
version: 0.1.0
milestone: M3 — Fanout, adapters, index
status: implemented
priority: P0
owner: expert-backend
methodology: tdd
coverage_target: 85
created: 2026-05-04
updated: 2026-05-08
author: limbowl
issue_number: null
depends_on: [SPEC-CORE-001, SPEC-BOOT-001, SPEC-OBS-001]
blocks: [SPEC-IDX-002, SPEC-IDX-003, SPEC-CACHE-001, SPEC-IDX-004, SPEC-IDX-005]
---

# SPEC-IDX-001: Hybrid Index Layer (Qdrant + Meilisearch + PostgreSQL)

## HISTORY

- 2026-05-08 (implemented v0.1, manager-tdd): RED-GREEN-REFACTOR TDD cycle complete.
  All EARS REQs covered by unit tests (docid, embedder, rrf, options, index, dispatch,
  observability, qdrant/client, meili/client, pg/client). Integration tests behind
  `//go:build integration` tag covering Docker round-trip. `go vet` + `golangci-lint`
  clean (0 issues). Build successful. Unit coverage: 40.2% internal/index (integration-
  dependent code excluded). Prometheus metrics registered (REQ-IDX-011).

- 2026-05-04 (initial draft v0.1, limbowl via manager-spec):
  First EARS-formatted SPEC for the M3 hybrid index layer. Drafted after
  deep research into the existing-code state
  (`.moai/specs/SPEC-IDX-001/research.md`, every claim file:line-cited or
  URL-cited). Builds on SPEC-CORE-001 (`pkg/types.NormalizedDoc` 15-field
  struct at `pkg/types/normalized_doc.go:40-56`,
  `NormalizedDoc.Validate` at `pkg/types/normalized_doc.go:63-77`,
  `NormalizedDoc.CanonicalHash` at `pkg/types/normalized_doc.go:91-106`),
  SPEC-BOOT-001 (Qdrant/Meili/PG compose services pre-running with
  healthchecks at `deploy/docker-compose.yml:31-87`), and SPEC-OBS-001
  (`obs.Logger`, `obs.Tracer`, named Prometheus collectors with bounded
  cardinality allowlist at `internal/obs/metrics/metrics.go:171-176`).

  User-locked decisions baked in:

  - **D1 Three-store architecture**: Qdrant (dense vectors via gRPC),
    Meilisearch (sparse / BM25-like keyword via HTTP), PostgreSQL
    (relational metadata via pgx). Rationale: V1 needs semantic +
    keyword + relational filtering with separate operational profiles.
    Single-store options rejected per research §3.1. The roadmap row at
    `.moai/project/roadmap.md:55` is verbatim: "Qdrant + Meilisearch +
    PG Go clients, RRF fusion, ingestion pipeline".
  - **D2 Go client libraries**: `github.com/qdrant/go-client v1.17.0`
    (Apache-2.0, server-pin compatible with `qdrant/qdrant:v1.16.3`),
    `github.com/meilisearch/meilisearch-go v0.36.2` (MIT, server-pin
    compatible with `getmeili/meilisearch:v1.42.1`),
    `github.com/jackc/pgx/v5` (MIT, server-pin compatible with
    `postgres:16.13-alpine3.23`). All three verified production-ready,
    no pre-1.0 risk. Research §2.
  - **D3 Deterministic doc_id**: `doc_id = hex(sha256(SourceID + "\x00" +
    URL))[:16]` (16 hex chars = 64 bits). Stable across goroutines and
    processes; enables parallel ingestion with idempotent upsert
    semantics; NEVER consumes a DB sequence (which would serialise
    ingestion through PG). Research §3.3.
  - **D4 Two-key idempotency on PG**: `docs.doc_id` is PRIMARY KEY
    (stable across content edits); `docs.content_hash UNIQUE` is the
    `NormalizedDoc.CanonicalHash()` value (stable across mirror URLs
    that re-emit identical content). `INSERT … ON CONFLICT
    (content_hash) DO NOTHING` provides cross-process replay safety.
    Research §3.3.
  - **D5 RRF fusion**: Reciprocal Rank Fusion (Cormack/Clarke/Buettcher
    SIGIR 2009) with `k=60` (paper default), per-ranker weights default
    `1.0` (uniform), all four configurable via
    `.moai/config/sections/index.yaml`. Score-normalisation /
    Borda count / learned-to-rank rejected for v0 per research §2.4.
  - **D6 Per-store timeout policy**: Hardcoded defaults
    `qdrant=200ms, meili=300ms, pg=100ms` in
    `Options.PerStoreTimeout`, with parent ctx (the caller's overall
    budget) taking precedence — per-store ctx inherits
    `min(perStoreTimeout, remainingTimeToParent)`. Mirrors
    SPEC-FAN-001 §2.5 derivation. Research §3.5.
  - **D7 Soft-fail partial-result**: When a store fails or times out
    during retrieval, IDX-001 records the error in
    `IndexResult.PerStoreErrors[store]`, proceeds with the remaining
    stores' rank lists for RRF, and returns `(*IndexResult, nil)` at
    the call level. Hard error (`ErrAllStoresFailed`) only when all
    three stores fail. Mirrors SPEC-FAN-001 partial-result discipline.
    Research §3.6.
  - **D8 Embedder is a port (deferred to SPEC-IDX-002)**: IDX-001
    declares an `Embedder` interface (`Embed(ctx, []string) ([][]float32,
    error)`, `Dimensions() int`) and ships a stub `zeroEmbedder` that
    returns zero-vectors of dimension 1024. SPEC-IDX-002 (BGE-M3)
    constructs the real embedder; the constructor signature does NOT
    change. The Qdrant collection is created once with `vector_size=1024`
    matching BGE-M3's output. Research §3.7.
  - **D9 Synchronous Upsert**: `Upsert([]NormalizedDoc)` is synchronous;
    fans into the three stores in parallel via errgroup; returns
    aggregate result. No async queue, no Asynq integration in v0.
    Streaming ingest (channel-based) is deferred to SPEC-DEEP-003 / IDX-005
    if measured. Research §3.8.
  - **D10 Multi-tenancy reservation**: PG `docs` table includes
    `team_id TEXT NULL` from day one. v0 inserts NULL universally
    (single-tenant); SPEC-IDX-004 (M6 per
    `.moai/project/roadmap.md:84`) flips the column to `NOT NULL`,
    adds Qdrant payload-based partitioning, Meili tenant tokens, and
    PG row-level security. v0 declares the column surface but does NOT
    enforce visibility. Research §3.9.
  - **D11 Observability**: ONE new metric family group emitted by
    IDX-001: `usearch_index_ops_total{store, op, outcome}` (Counter),
    `usearch_index_op_duration_seconds{store, op}` (Histogram),
    `usearch_index_fusion_duration_seconds` (Histogram, no labels).
    Bounded label values: `store ∈ {qdrant, meili, pg, fusion}`,
    `op ∈ {upsert, search, ensure_schema}`,
    `outcome ∈ {success, failure, timeout}`. Cardinality allowlist
    extended with `store` and `op`. ONE OTel parent span
    `index.search` / `index.upsert` per call. ONE slog summary record
    at the end of each public method. Mirrors the LLM-001 pattern
    (`internal/obs/metrics/llm.go`) for sole-emitter discipline.
    Research §1.5.
  - **D12 Meilisearch async write semantics**: Production calls fire
    `AddDocuments` and return without waiting for indexing. Eventual
    consistency is the contract (acceptable for V1: indexing latency
    is ~100-500ms; queries see new docs within a couple of seconds).
    Tests use `WaitForTask(taskUID)` to synchronise assertions.
    Research §6.7.

  Resolved deferrals (carried to SPEC body Open Questions §11):

  - The CLI / FAN-001 integration question (does FAN-001 internally
    call `Index.Upsert` or does the CLI orchestrate after `Dispatch`
    returns?) is OQ §11.5; recommended default is "CLI orchestrates"
    so FAN-001 stays single-domain.
  - The cache-hit retrieval branch (does CLI call `Index.Search`
    BEFORE fanout?) is post-V1; M3 v0.1 does retrieval AFTER fanout
    (re-rank path).
  - The multi-tenancy enforcement is SPEC-IDX-004 (M6).
  - The streaming ingestion API (`IngestBatch(<-chan NormalizedDoc)`)
    is deferred to SPEC-DEEP-003 / SPEC-IDX-005.

  14 EARS REQs (12 × P0 + 2 × P1) covering Ubiquitous (REQ-IDX-001/008/011/014),
  Event-Driven (REQ-IDX-005/006/007/009/013), State-Driven (REQ-IDX-002/012),
  Optional (REQ-IDX-003/010), and Unwanted (REQ-IDX-004) patterns. 5 NFRs
  (NFR-IDX-001 ingest throughput, NFR-IDX-002 retrieval latency budget,
  NFR-IDX-003 race-clean concurrent invocation, NFR-IDX-004 zero
  goroutine leaks, NFR-IDX-005 alloc/op ceiling on retrieval hot path).
  9 Open Questions carried forward from research.md §6 for plan-auditor
  challenge.

  Three new Go module dependencies introduced by IDX-001 (run-phase pins
  per SPEC-DEP-001 REQ-DEP-007):
  - `github.com/qdrant/go-client v1.17.0`
  - `github.com/meilisearch/meilisearch-go v0.36.2`
  - `github.com/jackc/pgx/v5` (latest stable v5.x)

  Insertion point: M3 retrieval-foundation SPEC. Parallel with
  SPEC-FAN-001 (gateway, already approved per
  `.moai/specs/SPEC-FAN-001/spec.md:6`). Blocks SPEC-IDX-002 (embedder
  service plugs into the `Embedder` port), SPEC-IDX-003 (Korean
  tokenization plugs into the Meili index settings), SPEC-CACHE-001
  (5-phase access fallback consumes `Index.Search` for the index-hit
  phase), SPEC-IDX-004 (multi-tenancy enforcement of the `team_id`
  column reserved here), and SPEC-IDX-005 (team-shared answer reuse).

  Harness level: standard (single domain, ~25 source files in
  `internal/index/`, three new Go module deps with verified upstream
  stability, three migration files under `deploy/postgres/migrations/`,
  one new optional config file `.moai/config/sections/index.yaml` for
  default tunables — see §6.7). Sprint Contract optional. Ready for
  plan-auditor review and annotation cycle.

---

## 1. Purpose

SPEC-CORE-001 published the typed adapter contract (`pkg/types.NormalizedDoc`
15-field canonical struct at `pkg/types/normalized_doc.go:40-56`,
`Validate` at `pkg/types/normalized_doc.go:63-77`, `CanonicalHash` at
`pkg/types/normalized_doc.go:91-106`, `pkg/types.Adapter` 4-method
interface at `pkg/types/adapter.go:28-45`), and the `internal/adapters.Registry`
sole-emitter `wrappedAdapter` (`internal/adapters/registry.go:172-263`).
SPEC-BOOT-001 brought up the compose stack with Qdrant / Meilisearch /
PostgreSQL services pre-running and healthchecked at
`deploy/docker-compose.yml:31-87`. SPEC-OBS-001 published `obs.Logger`,
`obs.Tracer`, and the bounded-cardinality named-collector pattern at
`internal/obs/metrics/metrics.go:171-176`. SPEC-FAN-001 (status: approved
per `.moai/specs/SPEC-FAN-001/spec.md:6`) defines `fanout.Result.Docs` as
the canonical post-dispatch slice IDX-001 consumes for ingestion.

The `internal/index/` package is currently a 3-line stub
(`internal/index/index.go:1-3`); the directory has zero sub-packages.
`.moai/project/structure.md:35-38` reserves
`internal/index/{qdrant,meilisearch,postgres}` as the target tree.

SPEC-IDX-001 fills `internal/index/` with the **three-store hybrid index
layer** that:

1. Provides three Go clients — Qdrant (gRPC), Meilisearch (HTTP), PostgreSQL
   (pgx) — each with construction, schema-bootstrap, idempotent upsert,
   and per-store retrieval methods; each isolated in its own sub-package
   (`internal/index/qdrant`, `internal/index/meili`, `internal/index/pg`).
2. Defines a deterministic `doc_id = hex(sha256(SourceID + "\x00" + URL))[:16]`
   so the same logical document carries the same identifier across
   stores, processes, and replays — enabling parallel multi-goroutine
   ingestion with no DB sequence.
3. Exposes a top-level `*Index` orchestrator with `Search(ctx, IndexQuery)
   (*IndexResult, error)` and `Upsert(ctx, []NormalizedDoc) (*UpsertResult,
   error)` as the sole public surfaces.
4. Implements parallel per-store `Search` with `errgroup.SetLimit(3)` +
   per-store `context.WithTimeout` derivation; suppresses per-store
   errors so a single store failure does not cancel siblings; collects
   partial results.
5. Fuses the per-store rank lists via Reciprocal Rank Fusion (Cormack,
   Clarke, Buettcher SIGIR 2009 — http://cormack.uwaterloo.ca/cormacksigir09-rrf.pdf),
   formula `RRFscore(d) = sum_r w_r / (k + rank_r(d))` with default `k=60`
   and uniform weights `w_r = 1.0`, all configurable.
6. Bootstraps the PG schema (`docs` table with `doc_id PRIMARY KEY`,
   `content_hash UNIQUE`, `source_id`, `url`, `title`, `lang`,
   `published_at`, `retrieved_at`, `team_id NULL`, `payload JSONB`) via
   migration files under `deploy/postgres/migrations/0001_create_docs.sql`.
7. Bootstraps the Qdrant collection (`usearch_docs` with
   `vector_size=1024, distance=Cosine, on_disk_payload=true`) idempotently
   on startup.
8. Bootstraps the Meilisearch index (`usearch_docs` with
   `primaryKey=doc_id`, searchable attributes `[title, body, snippet]`,
   filterable attributes `[source_id, lang, doc_type, team_id,
   published_at]`, distinct attribute `doc_id`) idempotently on startup.
9. Declares an `Embedder` interface port and ships a `zeroEmbedder` stub;
   SPEC-IDX-002 wires the BGE-M3 implementation. The Qdrant collection
   schema is sized for BGE-M3 (1024 dims) preemptively.
10. Reserves the per-team `team_id` column on PG; SPEC-IDX-004 (M6)
    enforces multi-tenancy visibility rules. v0 inserts NULL universally.
11. Emits per-call observability through ONE new metric family group
    (`usearch_index_ops_total{store, op, outcome}` + duration
    histograms), one OTel parent span (`index.search` /
    `index.upsert`), one slog summary record per call, and zero
    additional Prometheus metric families beyond this group.

The index does NOT classify (SPEC-IR-001 owns intent routing), does NOT
fan-out to live adapters (SPEC-FAN-001 owns), does NOT synthesise answers
(SPEC-SYN-001/002/003 owns), does NOT generate embeddings (SPEC-IDX-002
owns), does NOT provide a circuit breaker (SPEC-EVAL-002 M8 owns), does
NOT enforce per-team visibility (SPEC-IDX-004 M6 owns), does NOT cache
results in-process beyond the three stores' own caches.

Completion unblocks SPEC-IDX-002 (BGE-M3 embedder fills the `Embedder`
port), SPEC-IDX-003 (Korean tokenization extends the Meili index
settings), SPEC-CACHE-001 (5-phase access fallback consumes `Index.Search`
for the index-hit phase per `.moai/project/roadmap.md:58`), SPEC-IDX-004
(M6 multi-tenancy enforcement of the reserved `team_id` column), and
SPEC-IDX-005 (M6 team-shared answer reuse). Closes the M3 retrieval-
infrastructure half of the exit-criterion gate
(`.moai/project/roadmap.md:150` — "`usearch query` returns fused results
across ≥5 adapters") complementing SPEC-FAN-001's dispatch half.

This is the **retrieval-foundation SPEC** for M3. The shape laid down
here propagates into every M4+ synthesis SPEC (which consume `Index.Search`
output) and bounds the contract SPEC-IDX-002 / IDX-003 / IDX-004 / IDX-005
extend.

---

## 2. Scope

### 2.1 In-Scope

| Ref | Item |
|-----|------|
| a | `internal/index/index.go`: `Index` struct (immutable post-construction; holds three sub-clients + `*obs.Obs` + `Options`), `New(opts Options) (*Index, error)` constructor (validates non-nil sub-store configs, normalises Options defaults, calls each store's `EnsureSchema`), `Close() error` (orderly shutdown of pgxpool + qdrant gRPC channel + Meili HTTP client). |
| b | `internal/index/index.go`: public method surface — `Upsert(ctx context.Context, docs []types.NormalizedDoc) (*UpsertResult, error)` and `Search(ctx context.Context, q IndexQuery) (*IndexResult, error)`. |
| c | `internal/index/types.go`: `IndexQuery{Text, Lang, DocTypes, Since, Until, SourceID, TeamID, MaxResults}` value type for retrieval; `IndexResult{Docs []types.NormalizedDoc, PerStoreErrors map[string]error, Stats SearchStats}`; `UpsertResult{Inserted int, Skipped int, PerStoreErrors map[string]error, Stats UpsertStats}`; `SearchStats{StoreLatencies map[string]time.Duration, FusionLatency time.Duration, PerStoreCounts map[string]int, FusedCount int, ElapsedSeconds float64}`; `UpsertStats{DocCount int, PerStoreLatencies map[string]time.Duration, ElapsedSeconds float64}`. JSON-marshalable for diagnostic dumps. |
| d | `internal/index/options.go`: `Options{Qdrant qdrant.Config, Meili meili.Config, PG pg.Config, Embedder Embedder, Obs *obs.Obs, MaxParallel int, PerStoreTimeout map[string]time.Duration, RRFConstantK int, RRFWeights map[string]float64, BulkBatchSize int, AutoEnsureSchema bool}` with documented zero-value defaults (`MaxParallel=3`, per-store timeouts as in §6.6, `RRFConstantK=60`, `RRFWeights map[]=1.0`, `BulkBatchSize=100`, `AutoEnsureSchema=true`) and validation in `New`. |
| e | `internal/index/embedder.go`: `Embedder` interface + `zeroEmbedder` stub. The interface is a port; SPEC-IDX-002 ships the production implementation. `zeroEmbedder.Dimensions() int = 1024`; `zeroEmbedder.Embed(ctx, texts) [][]float32` returns `len(texts)` zero-vectors of length 1024. |
| f | `internal/index/qdrant/client.go`: `Client` wrapping `*qdrant.Client` from `github.com/qdrant/go-client v1.17.0`; methods `EnsureCollection(ctx, name string, vectorSize int) error`, `Upsert(ctx, points []Point) error`, `Search(ctx, vector []float32, filter *Filter, limit int) ([]ScoredPoint, error)`, `Close() error`. Connection via gRPC port 6334. |
| g | `internal/index/meili/client.go`: `Client` wrapping `meilisearch.ServiceManager` from `github.com/meilisearch/meilisearch-go v0.36.2`; methods `EnsureIndex(ctx, name string, settings IndexSettings) error`, `AddDocuments(ctx, name string, docs []Document) error` (async fire-and-forget per D12), `Search(ctx, name string, query string, opts SearchOptions) ([]Document, error)`, `Close() error` (no-op for HTTP client; preserved for symmetry). |
| h | `internal/index/pg/client.go`: `Client` wrapping `*pgxpool.Pool` from `github.com/jackc/pgx/v5/pgxpool`; methods `EnsureSchema(ctx) error` (runs migration files under `deploy/postgres/migrations/`), `Upsert(ctx, docs []DocRow) (inserted, skipped int, err error)` (uses `INSERT … ON CONFLICT (content_hash) DO NOTHING`), `Search(ctx, filters Filters) ([]DocRow, error)` (filter-only, no full-text), `Close() error`. |
| i | `internal/index/dispatch.go`: the parallel-fanout helpers. `parallelSearch(ctx, query) (perStore[]rank, perStoreErrs map[string]error, perStoreLat map[string]time.Duration)`; `parallelUpsert(ctx, docs) (perStorePartials []UpsertPartial, perStoreErrs map[string]error)`. Mirrors `internal/fanout/dispatch.go` errgroup discipline. |
| j | `internal/index/rrf.go`: `fuseRRF(rankLists map[string][]Ranked, weights map[string]float64, k int) []FusedDoc`. Pure function; `O(N)` time and space. RRF formula: `score(d) = sum_store w_store / (k + rank_store(d))` for each doc `d` appearing in any store's rank list. Returns slice sorted by `score` descending. |
| k | `internal/index/docid.go`: `docID(sourceID, url string) string` returning the 16-hex-char identifier. Pure function; deterministic; uses `crypto/sha256`. |
| l | `internal/index/observability.go`: `emitSearch(ctx, span, result, elapsed)` and `emitUpsert(ctx, span, result, elapsed)` helpers writing `index.search` / `index.upsert` span attributes and slog summary records. Mirrors the `internal/router/router.go:341-383::emit` pattern. Nil-safe across `Obs`, `Obs.Metrics`, `Obs.Logger`. |
| m | `internal/index/errors.go`: package-level sentinels — `ErrAllStoresFailed = errors.New("index: all three stores failed")` (returned by `Search` only when no rank list is non-empty), `ErrSchemaBootstrapFailed = errors.New("index: schema bootstrap failed")` (returned by `New` when `AutoEnsureSchema=true` and any store rejects schema), `ErrEmbedderRequired = errors.New("index: embedder is nil")` (returned by `New` when `opts.Embedder == nil`). |
| n | `internal/obs/metrics/index.go`: `IndexOps *prometheus.CounterVec` (labels `store, op, outcome`), `IndexOpDuration *prometheus.HistogramVec` (labels `store, op`), `IndexFusionDuration prometheus.Histogram`. Registered via `registerIndex(r *prometheus.Registry)` called from `NewRegistry`. Cardinality allowlist extended with `store` and `op`. |
| o | `deploy/postgres/migrations/0001_create_docs.sql`: PG schema migration (CREATE TABLE `docs` with the field set above; INDEX on `content_hash` UNIQUE, on `source_id`, on `published_at`, on `team_id`). |
| p | `.moai/config/sections/index.yaml`: NEW optional config file. Default values work without it; documented for operators who want to tune. |
| q | Test files (matching the implementation tree): `index_test.go`, `dispatch_test.go`, `rrf_test.go`, `docid_test.go`, `observability_test.go`, `concurrent_test.go`, `bench_test.go` at the package root, plus `client_test.go` per sub-package (`qdrant/`, `meili/`, `pg/`) using `httptest.NewServer` stubs (Meili) / mockable interfaces (Qdrant, PG via testcontainers-go). |

### 2.2 Out-of-Scope

[HARD] This SPEC explicitly excludes the following items. Each has a known
destination SPEC; this list prevents scope creep into IDX-001 (the M3
retrieval foundation).

- **BGE-M3 embedding inference** (the actual `Embedder` implementation
  that produces real vectors from text). → SPEC-IDX-002 (M3 per
  `.moai/project/roadmap.md:56`). v0.1 ships a `zeroEmbedder` stub; the
  Qdrant collection schema is reserved at `vector_size=1024` to match
  BGE-M3.
- **Korean tokenization** (mecab-ko sidecar, Meili custom tokenizer
  plugin, separate `ko` shard). → SPEC-IDX-003 (M3). v0.1 ships the
  default Meili tokenizer; Korean precision is accepted as v0 baseline.
- **5-phase access fallback** (insane-search pattern: index lookup →
  probe → TLS → Playwright). → SPEC-CACHE-001 (M3 per
  `.moai/project/roadmap.md:58`). CACHE-001 wraps `Index.Search` as the
  first phase.
- **Per-team multi-tenancy enforcement** (Qdrant payload-based
  partitioning, Meili tenant tokens, PG row-level security). →
  SPEC-IDX-004 (M6 per `.moai/project/roadmap.md:84`). v0.1 reserves
  the `team_id` column as NULL; visibility rules are deferred.
- **Team-shared answer reuse** (pre-fanout lookup in team index with
  staleness threshold). → SPEC-IDX-005 (M6).
- **Streaming retrieval** (channel-based, SSE, WebSocket result
  delivery from inside `Index.Search`). → SPEC-SYN-004 (M4 per
  `.moai/project/roadmap.md:66`).
- **Streaming bulk ingestion** (`IngestBatch(<-chan NormalizedDoc)`).
  → SPEC-DEEP-003 / SPEC-IDX-005 (deferred). v0.1's synchronous
  `Upsert([]NormalizedDoc)` covers M3 ingestion volume.
- **Compensating actions on partial-write failure** (rollback Qdrant
  on PG failure, etc.). → SPEC-IDX-005 (M6 team-shared answer reuse,
  where consistency matters more). v0.1 records per-store errors and
  surfaces them; the `INSERT … ON CONFLICT DO NOTHING` semantics make
  retries safe.
- **Per-store circuit breaker** (auto-disable a flapping store with
  half-open probe). → SPEC-EVAL-002 (M8). The index has no disable
  flag; every `Search` invokes every store.
- **Learned-to-rank (LTR)** (per-document feature scoring beyond RRF).
  → Future SPEC-RANK-001 (post-V1). v0.1 ships RRF; weights are
  operator-tuneable.
- **Pre-flight cost estimation** (estimated query cost before issuing).
  → Out of scope; index queries do not consume LLM tokens.
- **Cross-collection / cross-index migrations** (Qdrant alias swap,
  Meili index swap on schema change). → Out of v0.1; the schema is
  pinned at IDX-001 approval and changes via SPEC-IDX-006 (post-V1).
- **Cardinality allowlist amendment beyond `store` + `op`** — the two
  new labels are bounded enums (`store ∈ 4 values`, `op ∈ 3 values`).
  No additional unbounded labels.
- **HTTP / gRPC server exposure of `Index`**. → SPEC-MCP-001 (M7) and
  future SPEC-API-001. Index is a Go library only in v0.1.
- **In-process result cache** (LRU keyed on canonical query). → Out of
  v0.1; the three stores' own caches handle cold-vs-warm latency.
- **Backwards migration from existing data**. → Greenfield; no prior
  data exists in the compose volumes.
- **GitHub Issue tracking on this SPEC** (skipped per session pattern —
  orchestrator handles).

### 2.3 doc_id and content_hash Discipline

[HARD] The two-key idempotency model is the load-bearing invariant for
ingestion correctness:

| Key | Source | Stability | Role |
|-----|--------|-----------|------|
| `doc_id` | `hex(sha256(SourceID + "\x00" + URL))[:16]` | Across content edits (Title/Body change does NOT change doc_id) | PRIMARY KEY on PG; primary key on Meili; point ID on Qdrant |
| `content_hash` | `NormalizedDoc.CanonicalHash()` (= `hex(sha256(SourceID|URL|Title|Body))[:16]`) | Across mirror URLs that re-emit identical content | UNIQUE constraint on PG (enables INSERT-ON-CONFLICT replay safety); NOT used as Qdrant/Meili identifier |

**Worked examples**:

| Scenario | doc_id | content_hash | Behaviour on second ingest |
|----------|--------|--------------|----------------------------|
| Same SourceID + same URL + same content | unchanged | unchanged | Both keys match; Qdrant/Meili overwrite (idempotent); PG ON CONFLICT (content_hash) DO NOTHING — net zero ops |
| Same SourceID + same URL + edited Title | unchanged | CHANGED | doc_id matches; Qdrant/Meili overwrite (Title updated); PG ON CONFLICT no-op (content_hash differs from prior, so INSERT proceeds, BUT primary key (doc_id) collides — the row is updated via UPDATE … WHERE doc_id) |
| Same SourceID + different URL + same content | DIFFERENT | unchanged | Two different doc_ids; Qdrant/Meili insert TWO points; PG ON CONFLICT (content_hash) DO NOTHING — second INSERT is silently dropped |
| Different SourceID + same URL + same content | DIFFERENT | DIFFERENT (SourceID is in the hash) | Two doc_ids; two content_hashes; both inserts succeed across all stores |

The "edited Title" case is the trickiest. The PG INSERT statement uses a
two-stage upsert:

```sql
INSERT INTO docs (doc_id, content_hash, source_id, url, title, ...)
VALUES ($1, $2, $3, $4, $5, ...)
ON CONFLICT (content_hash) DO NOTHING
RETURNING doc_id;
-- If no row returned, doc_id is already present with different content_hash;
-- second statement updates content fields keyed on doc_id:
UPDATE docs SET title = $5, body = $6, content_hash = $2, retrieved_at = $7
WHERE doc_id = $1 AND content_hash != $2;
```

Single-shot SQL via `INSERT … ON CONFLICT (doc_id) DO UPDATE SET ...` is
equivalent and preferred; the two-stage form is shown for clarity. REQ-IDX-005
acceptance covers both branches.

### 2.4 Per-Store Timeout Derivation

[HARD] The per-store ctx is derived as:

```
storeDeadline = min(
    perStoreTimeout[store],            // Options.PerStoreTimeout (defaults §6.6)
    timeUntil(parentCtx.Deadline())    // remaining caller budget; ∞ if no parent deadline
)
storeCtx, cancel = context.WithTimeout(parentCtx, storeDeadline)
```

Properties (mirroring SPEC-FAN-001 §2.5):

- The PARENT ctx propagation is preserved: cancelling the parent cancels
  every per-store ctx via the Go context inheritance graph.
- The per-store timeout NEVER exceeds the caller's budget. A caller with a
  50ms deadline against a `qdrant.timeout=200ms` configuration sees the
  Qdrant call time out at 50ms.
- A caller with NO deadline gets the per-store floor.
- `defer cancel()` is mandatory immediately after `context.WithTimeout` —
  `go vet` enforces.

When the per-store ctx expires before the store call returns:

- The store driver returns either `context.DeadlineExceeded` (raw) or its
  own typed error wrapping the ctx error (pgx returns
  `ctx.Err()`-aware errors via the `Acquire`/query path; meilisearch-go
  propagates ctx via `*http.Request.WithContext`; qdrant-go-client
  propagates via gRPC's `context` aware unary call).
- The dispatch helper captures the error, stores it in the per-store
  partial result, and continues waiting on siblings.
- Observability emits `outcome="timeout"` for that store.

### 2.5 RRF Fusion Algorithm

[HARD] `rrf.go::fuseRRF` is deterministic and pure (input maps → output
slice, no I/O, no time, no randomness). Golden tests compute expected
output from input alone.

**Algorithm**:

1. Take input `rankLists map[string][]Ranked` where each `Ranked` carries
   `{DocID string, Doc types.NormalizedDoc}` — the slice is already
   sorted in store-rank order (rank 1 = first element).
2. For each `(store, list)` pair, iterate; for index `i ∈ [0, len(list))`:
   - `rank = i + 1` (1-indexed per the paper).
   - `weight = weights[store]` (default 1.0 if not set).
   - Accumulate into `scores[doc_id] += weight / (float64(k) + float64(rank))`.
   - Record `firstSeenDoc[doc_id] = list[i].Doc` (preserves doc body
     across stores; first occurrence wins for the returned NormalizedDoc).
3. Build output slice `[]FusedDoc{DocID, Score, Doc}` from `scores` map.
4. Sort by `Score` descending; tie-breaker `DocID` ascending (stable).
5. Return.

**Properties**:
- O(N) time, O(N) space where N = sum of `len(list)` across stores.
- Same input → byte-equal output (deterministic; map iteration is not
  observed because we build a slice and sort it).
- A doc appearing in 2 stores at rank 5 each (k=60) scores
  `1/(60+5) + 1/(60+5) = 2/65 ≈ 0.0308`.
- A doc appearing in 1 store at rank 1 scores `1/(60+1) ≈ 0.0164`.
- A doc appearing in 3 stores at rank 1 each scores
  `3/(60+1) = 3/61 ≈ 0.0492` — beats both above.

**Weighting**:
- Default `w_r = 1.0` for all `r`. Reduces to the original Cormack
  formula.
- Operators tune via `.moai/config/sections/index.yaml` `rrf.weights`
  map. Higher weight → more influence in the fused score.
- Negative or zero weights are rejected at config validation; weights
  must be `> 0`.

REQ-IDX-007 covers this pattern.

### 2.6 Soft-Fail Discipline (Partial Result Assembly)

[HARD] `Search` follows the SPEC-FAN-001 partial-result pattern:

When K (1, 2, or 3) of the three stores fail or time out:

- The failed stores contribute zero rank entries.
- `IndexResult.PerStoreErrors[storeName] = err` (non-nil).
- RRF runs over the remaining `(3-K)` rank lists. If K=3 (all fail),
  `Search` returns `(nil, ErrAllStoresFailed)`. Otherwise returns
  `(*IndexResult, nil)`.
- `IndexResult.Stats.PerStoreCounts[failedStore] = 0`.

When K (1, 2, or 3) stores fail or time out during `Upsert`:

- The failed stores' writes are recorded in
  `UpsertResult.PerStoreErrors[storeName]`.
- The successful stores' writes ARE COMMITTED (no compensation in v0.1).
- `Upsert` returns `(*UpsertResult, nil)` regardless of K.
- An aggregated `Stats.ErrorCount` and `Stats.SuccessCount` reflect the
  partial state (per-store granularity).

Rationale: a degraded retrieval (2 of 3 stores) is more useful than a
hard failure. A partially-written ingest preserves what succeeded; replay
on the next ingestion cycle reaches consistency via doc_id-keyed upsert
semantics.

### 2.7 Embedder Port Contract (Forward Compatibility with SPEC-IDX-002)

[HARD] The `Embedder` interface is the IDX-001 / IDX-002 boundary:

```go
package index

type Embedder interface {
    // Embed produces dense vectors for the given texts. Returns a slice
    // of length len(texts), each vector of length Dimensions(). Errors
    // surface to the caller; partial success is NOT supported (all-or-
    // nothing per call).
    Embed(ctx context.Context, texts []string) ([][]float32, error)

    // Dimensions returns the static embedding dimensionality. MUST match
    // the Qdrant collection vector_size at construction time.
    Dimensions() int
}
```

v0.1 ships `zeroEmbedder`:

```go
type zeroEmbedder struct{}

func (zeroEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
    out := make([][]float32, len(texts))
    for i := range out {
        out[i] = make([]float32, 1024)
    }
    return out, nil
}

func (zeroEmbedder) Dimensions() int { return 1024 }
```

[HARD] When `opts.Embedder == nil`, `New` returns
`(nil, ErrEmbedderRequired)`. Callers MUST inject either `zeroEmbedder{}`
or the SPEC-IDX-002 BGE-M3 implementation. The CLI's
`cmd/usearch/query.go` constructs the index with `zeroEmbedder` until
SPEC-IDX-002 lands; the swap is a constructor-argument change with no
SPEC-IDX-001 surface modification.

### 2.8 Multi-Tenancy Reservation (SPEC-IDX-004 Forward Compatibility)

[HARD] PG `docs` schema includes `team_id TEXT NULL` from day one. v0.1:

- Inserts pass `team_id = NULL` universally.
- Search filters via `IndexQuery.TeamID` are accepted but apply only when
  non-empty; empty TeamID matches all rows (single-tenant default).
- Qdrant payload includes `team_id: null` field (preserves payload shape
  forward-compatibly).
- Meili filterable attributes include `team_id`.

SPEC-IDX-004 (M6) flips:
- PG column to `team_id TEXT NOT NULL DEFAULT 'default'`.
- Adds Qdrant payload-based partitioning per the Qdrant docs at
  https://qdrant.tech/documentation/concepts/collections/ §"Multitenancy
  Strategy" (recommended pattern: one collection, payload filter on
  tenant key).
- Adds Meili tenant tokens (per `meilisearch-go` SDK token API).
- Adds PG row-level security policies keyed on `team_id`.

REQ-IDX-010 makes the v0 reservation testable.

---

## 3. EARS Requirements

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| REQ-IDX-001 | Ubiquitous | The package `internal/index` SHALL expose an `Index` struct constructed via `New(opts Options) (*Index, error)` and two public methods — `Search(ctx context.Context, q IndexQuery) (*IndexResult, error)` and `Upsert(ctx context.Context, docs []types.NormalizedDoc) (*UpsertResult, error)` — plus a `Close() error` shutdown method that closes the pgxpool, Qdrant gRPC channel, and Meilisearch HTTP client. `New` SHALL return `ErrEmbedderRequired` when `opts.Embedder == nil`. `New` SHALL return `ErrSchemaBootstrapFailed` (wrapping the underlying store error) when `opts.AutoEnsureSchema == true` and any of the three stores rejects schema initialisation. `New` SHALL normalise zero-valued Options fields to documented defaults (`MaxParallel=3`, per-store timeouts as in §6.6, `RRFConstantK=60`, uniform `RRFWeights[r]=1.0`, `BulkBatchSize=100`, `AutoEnsureSchema=true`). | P0 | `TestNewRequiresEmbedder` (nil Embedder → `errors.Is(err, ErrEmbedderRequired)`); `TestNewSchemaBootstrapFailureWraps` (PG client rejects EnsureSchema → `errors.Is(err, ErrSchemaBootstrapFailed)`); `TestNewNormalisesDefaults` (zero Options → documented defaults observed via reflection on returned `*Index`); `TestSearchAlwaysReturnsResultOrAllStoresFailed` (50 invocations across success/partial/all-failure paths; for partial assert `result != nil` AND `err == nil`; for all-failure assert `result == nil` AND `errors.Is(err, ErrAllStoresFailed)`); `TestCloseClosesAllStores` (verify pgxpool.Close, qdrant.Close, meili noop in sequence). All in `index_test.go`. |
| REQ-IDX-002 | State-Driven | WHILE the Qdrant client is constructed against the compose `qdrant` service, the `internal/index/qdrant.Client` SHALL provide `EnsureCollection(ctx, name string, vectorSize int) error` that creates the named collection idempotently (no-op if exists), with `vector_size = vectorSize` (1024 for BGE-M3 per §2.7), `distance = Cosine` (per Qdrant docs https://qdrant.tech/documentation/concepts/collections/), and `on_disk_payload = true`; the collection SHALL accept points with `point_id` formatted as a UUID-shaped 32-hex string derived by left-padding the 16-hex doc_id with zeros and inserting RFC 4122 dashes; payload schema SHALL include `source_id` (string), `url` (string), `title` (string), `lang` (string), `doc_type` (string), `published_at` (int64 unix), `retrieved_at` (int64 unix), `team_id` (string, nullable), and `content_hash` (string). | P0 | `TestQdrantEnsureCollectionIdempotent` (calls EnsureCollection twice on a fresh testcontainers Qdrant; second call is a no-op; no error); `TestQdrantUpsertRoundTrip` (upsert a point with synthetic 1024-dim vector; query by vector; assert returned point matches by ID and payload); `TestQdrantSearchHonoursLimit` (insert 50 points; query top-10; assert exactly 10 returned in score-descending order); `TestQdrantPayloadFiltering` (filter by `source_id == "reddit"`; assert only matching points returned). All in `internal/index/qdrant/client_test.go`. |
| REQ-IDX-003 | Optional | WHERE the Meilisearch index does not yet exist, the `internal/index/meili.Client` SHALL provide `EnsureIndex(ctx, name string, settings IndexSettings) error` that creates the index with `primaryKey = "doc_id"`, `searchableAttributes = ["title", "body", "snippet"]`, `filterableAttributes = ["source_id", "lang", "doc_type", "team_id", "published_at"]`, `distinctAttribute = "doc_id"`, and the default Meili ranking rules; the method SHALL be idempotent (no-op if the index exists with matching settings; PATCH if settings differ; CREATE if absent). The `AddDocuments` method SHALL use the async fire-and-forget pattern in production; tests synchronise via `WaitForTask(taskUID)`. | P1 | `TestMeiliEnsureIndexCreatesWithSettings` (calls EnsureIndex on a fresh Meili; assert index settings match expected via `index.GetSettings()`); `TestMeiliEnsureIndexIdempotent` (second call no-op); `TestMeiliAddDocumentsAsync` (production path: AddDocuments returns without WaitForTask; assert `TaskInfo.UID > 0`); `TestMeiliSearchTextMatch` (after WaitForTask, query for "alice"; assert matching docs returned). All in `internal/index/meili/client_test.go`. |
| REQ-IDX-004 | Unwanted | IF a doc passed to `Index.Upsert` has any of `{ID, SourceID, URL, RetrievedAt}` empty or zero (per `pkg/types.NormalizedDoc.Validate`), THEN the index SHALL skip that doc, increment `Stats.SkippedCount` by one, record a per-doc validation error in `UpsertResult.PerStoreErrors["validation"]` (the special "validation" pseudo-store key collects all pre-store rejections), AND SHALL NOT pass the invalid doc to any store driver. Other (valid) docs in the same batch SHALL proceed normally. The validation rejection SHALL be logged at WARN level once per batch (not once per invalid doc) with attributes `{request_id, batch_size, skipped_count}`. | P0 | `TestUpsertRejectsInvalidDocs` (batch of 5 docs: 3 valid + 2 invalid (one missing URL, one with zero RetrievedAt); assert `result.Stats.DocCount == 5`, `result.Stats.SkippedCount == 2`, `result.PerStoreErrors["validation"] != nil` and contains 2 errors, the 3 valid docs landed in all 3 stores via subsequent `Search`); `TestUpsertEmptyBatchReturnsZeroes` (batch of 0 docs; result.Stats fields all zero; no slog records emitted). |
| REQ-IDX-005 | Event-Driven | WHEN `Index.Upsert(ctx, docs)` is invoked with `len(docs) >= 1` (after validation rejection per REQ-IDX-004), the index SHALL compute `doc_id = hex(sha256(SourceID + "\x00" + URL))[:16]` for each doc, derive its `content_hash = doc.CanonicalHash()`, batch into chunks of `BulkBatchSize` docs, dispatch each batch to the three stores via `parallelUpsert` (errgroup with `SetLimit(3)`), and aggregate per-store results. PG SHALL use `INSERT ... ON CONFLICT (doc_id) DO UPDATE SET title = EXCLUDED.title, body = EXCLUDED.body, content_hash = EXCLUDED.content_hash, retrieved_at = EXCLUDED.retrieved_at WHERE docs.content_hash IS DISTINCT FROM EXCLUDED.content_hash` (idempotent: same content → no update; edited content → update). Qdrant SHALL use `client.Upsert` with point_id = doc_id (idempotent overwrite). Meilisearch SHALL use `index.AddDocuments` with primaryKey = doc_id (idempotent overwrite). Per-store errors SHALL be collected into `UpsertResult.PerStoreErrors`; per-store success counts SHALL feed `Stats.PerStoreLatencies`. | P0 | `TestUpsertHappyPath3Docs` (insert 3 valid docs; assert `Stats.DocCount == 3`, all 3 stores see all 3 docs); `TestUpsertIdempotentOnReplay` (insert same 3 docs twice; second call: `Stats.DocCount == 3` (entered the pipeline) but PG affected-rows == 0 from ON CONFLICT, Qdrant/Meili overwrite is a no-op for identical content); `TestUpsertEditedTitleUpdatesAllStores` (insert doc with Title="A"; insert again with Title="A-edited" same URL; query Search → returned doc has Title="A-edited" in all 3 stores after Meili WaitForTask); `TestUpsertSkipsRowOnPGFailure` (PG client errors on one batch; Qdrant/Meili succeed; result.PerStoreErrors["pg"] != nil; result.PerStoreErrors["qdrant"] == nil; result.PerStoreErrors["meili"] == nil; aggregate `Stats.ErrorCount == 1`). |
| REQ-IDX-006 | Event-Driven | WHEN `Index.Search(ctx, q)` is invoked with non-empty query Text, the index SHALL: (a) compute `embedding = embedder.Embed(ctx, []string{q.Text})[0]` (one-shot per call); (b) launch three goroutines via errgroup with `SetLimit(3)`, one per store; (c) derive per-store ctx via `context.WithTimeout(parentCtx, perStoreTimeout[store])` per §2.4; (d) Qdrant goroutine queries by vector + payload filter (source_id, lang, team_id, published_at range); (e) Meili goroutine queries by text + filter expression (same fields); (f) PG goroutine queries by filter only (no full-text, per §3.10 of research.md); (g) wait for all three or until parent ctx fires; (h) collect per-store rank lists into `rankLists map[string][]Ranked`; (i) fuse via `fuseRRF(rankLists, weights, k)`; (j) clamp output to `q.MaxResults` (default 50); (k) return `*IndexResult{Docs, PerStoreErrors, Stats}`. The function SHALL return `(nil, ErrAllStoresFailed)` ONLY when all three rank lists are empty AND every store recorded a non-nil error. | P0 | `TestSearchHappyPath3Stores` (insert 30 docs; query "alice"; assert all 3 stores returned non-empty rank lists; fused result has ≥1 doc; `result.Stats.PerStoreCounts["qdrant"] > 0`, `["meili"] > 0`, `["pg"] > 0`); `TestSearchHonoursMaxResults` (insert 100 docs; q.MaxResults=10; assert `len(result.Docs) <= 10`); `TestSearchPerStoreTimeoutDoesNotKillOthers` (q.Text = "x"; per-store timeout meili=1ms (forced timeout); assert `result.PerStoreErrors["meili"] != nil`, qdrant + pg succeed, result.Docs is non-empty); `TestSearchAllStoresFailReturnsErr` (all 3 stores rigged to fail; assert `errors.Is(err, ErrAllStoresFailed)`, `result == nil`). |
| REQ-IDX-007 | Event-Driven | WHEN `fuseRRF(rankLists, weights, k)` is called with `k > 0` and weights all `> 0`, the function SHALL compute `score(d) = sum_{store ∈ rankLists} weights[store] / (k + rank_store(d))` for each doc d appearing in any store's rank list, where `rank_store(d)` is the 1-indexed position in `rankLists[store]`. The output SHALL be sorted by `score` descending with `doc_id` ascending as the tie-breaker. Same input SHALL produce byte-equal output (determinism; `sort.SliceStable` over the score-decreasing key plus stable ID tie-break). The function SHALL accept k=60 as the caller-defaulted constant per Cormack/Clarke/Buettcher SIGIR 2009 (http://cormack.uwaterloo.ca/cormacksigir09-rrf.pdf). | P0 | `TestRRFSingleStoreReturnsRankOrder` (one store with 5 ranked docs; output has 5 docs in same order; scores are `1/(60+1), 1/(60+2), ...`); `TestRRFTwoStoresAdditive` (doc A at rank 1 in store1 + rank 1 in store2; doc B at rank 1 in store3 only; A's score = 2/61 > B's score = 1/61); `TestRRFWeightedFavoursHigherWeight` (weights={store1:2.0, store2:1.0}; doc A rank 1 in store1, doc B rank 1 in store2; A scores 2/61 > B scores 1/61); `TestRRFTieBreakDocIDAscending` (two docs with identical scores; output ordered by doc_id ascending); `TestRRFDeterministic` (run twice on same input; byte-equal output); `TestRRFKCanonical60` (standard k=60; verify against worked-example numbers). All in `rrf_test.go`. |
| REQ-IDX-008 | Ubiquitous | The PostgreSQL `docs` table SHALL be created idempotently by `internal/index/pg.Client.EnsureSchema` from migration files at `deploy/postgres/migrations/0001_create_docs.sql` containing exactly these columns with these types: `doc_id TEXT PRIMARY KEY`, `content_hash TEXT NOT NULL UNIQUE`, `source_id TEXT NOT NULL`, `url TEXT NOT NULL`, `title TEXT`, `body TEXT`, `snippet TEXT`, `lang TEXT`, `doc_type TEXT`, `published_at TIMESTAMPTZ`, `retrieved_at TIMESTAMPTZ NOT NULL`, `team_id TEXT NULL`, `payload JSONB`, `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`, `updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`. Indexes: B-tree on `source_id`, `published_at`, `team_id`; UNIQUE on `content_hash` (already declared). Migration files SHALL be applied via plain SQL execution at startup; v0.1 does NOT use a migration framework (deferred to a future SPEC-MIGRATE-001). Re-running `EnsureSchema` against an existing schema SHALL be a no-op; structural drift between the migration file and the live schema SHALL produce `ErrSchemaBootstrapFailed`. | P0 | `TestPGEnsureSchemaCreatesAllColumns` (fresh testcontainers PG; call EnsureSchema; query `information_schema.columns WHERE table_name='docs'`; assert all 15 columns with correct types); `TestPGEnsureSchemaIdempotent` (second call no-op; no error); `TestPGEnsureSchemaDetectsDrift` (manually drop column `team_id`; second EnsureSchema returns `ErrSchemaBootstrapFailed`); `TestPGEnsureSchemaIndexesExist` (assert UNIQUE on content_hash, B-tree on source_id, published_at, team_id). All in `internal/index/pg/client_test.go`. |
| REQ-IDX-009 | Event-Driven | WHEN any single store fails or times out during `Search`, the index SHALL record the per-store error in `IndexResult.PerStoreErrors[storeName]`, SHALL set `IndexResult.Stats.PerStoreCounts[storeName] = 0`, SHALL continue with the remaining store rank lists for RRF fusion, AND SHALL return `(*IndexResult, nil)` at the call level. The fanout-error-suppression idiom (workers `return nil` even on store error) SHALL prevent errgroup's first-error-cancel from killing siblings. WHEN the same condition applies during `Upsert`, the index SHALL record the per-store error in `UpsertResult.PerStoreErrors[storeName]`, SHALL NOT roll back successful sibling writes, AND SHALL return `(*UpsertResult, nil)`. | P0 | `TestSearchOneStoreTimesOutOthersSucceed` (qdrant timeout=1ms forced; meili+pg complete; result has 2-store fused docs; result.PerStoreErrors["qdrant"] is `errors.Is(*, context.DeadlineExceeded)`); `TestSearchOneStoreFailsOthersSucceed` (qdrant client returns a network error; meili+pg complete; same shape); `TestUpsertOneStoreFailsOthersSucceed` (qdrant client errors; meili+pg succeed; result.PerStoreErrors["qdrant"] != nil; result.Stats.SuccessCount == 2; subsequent Search returns docs from meili+pg even though qdrant has stale state); `TestSearchAllStoresFailReturnsErrAllStoresFailed` (each store rigged to fail; assert `errors.Is(err, ErrAllStoresFailed)`); `TestUpsertAllStoresFailReturnsResultWithThreeErrors` (each store fails; result is non-nil; result.PerStoreErrors has 3 entries; err == nil per soft-fail discipline for upsert). |
| REQ-IDX-010 | Optional | WHERE `IndexQuery.TeamID` is non-empty, the search SHALL apply a per-store filter restricting results to rows where `team_id == q.TeamID`. WHERE `q.TeamID` is empty, the search SHALL match all rows regardless of `team_id` (single-tenant default). The `Upsert` method SHALL set `team_id = NULL` for every doc in v0.1 (multi-tenancy enforcement is reserved for SPEC-IDX-004 M6). The PG `docs.team_id` column SHALL exist as `TEXT NULL` from REQ-IDX-008's migration; the Qdrant payload SHALL include the `team_id` field; the Meili filterable attributes SHALL include `team_id`. SPEC-IDX-004 (M6) flips the column to `NOT NULL`, populates from upsert input, and adds enforcement. | P1 | `TestSearchEmptyTeamIDMatchesAll` (insert 5 docs with team_id=NULL; query with q.TeamID=""; assert all 5 returned in result.Docs); `TestSearchTeamIDFilterMatchesNoneInV0` (query with q.TeamID="team-A"; v0.1 inserts NULL universally; assert result.Docs is empty — the filter excludes NULL rows); `TestUpsertSetsTeamIDNullInV0` (insert 3 docs; query PG directly: `SELECT team_id FROM docs WHERE doc_id = ANY($1)`; assert all rows have `team_id IS NULL`); `TestQdrantPayloadIncludesTeamIDField` (insert 1 doc; retrieve point payload; assert `team_id` key exists with NULL value); `TestMeiliFilterableAttributesIncludesTeamID` (assert via index.GetSettings()). |
| REQ-IDX-011 | Ubiquitous | The index SHALL emit per-`Search` and per-`Upsert` invocation: (a) one OTel parent span (`index.search` or `index.upsert`, kind = internal) with attributes `index.op` (= "search" or "upsert"), `index.docs_count` (or `index.fused_count`), `index.errors_count`, `index.elapsed_seconds`; per-store child spans `index.qdrant`, `index.meili`, `index.pg` are children via ctx propagation. (b) ONE counter increment on `obs.Metrics.IndexOps.WithLabelValues(store, op, outcome)` PER PER-STORE OPERATION (3 stores × 1 op = 3 increments per Search; 3 increments per Upsert batch — outcomes from the per-store partial result). (c) ONE histogram observation on `obs.Metrics.IndexOpDuration.WithLabelValues(store, op)` per per-store operation. (d) ONE histogram observation on `obs.Metrics.IndexFusionDuration` per Search call (no labels). (e) ONE slog record at level INFO (success or partial) or WARN (`ErrAllStoresFailed`) via `obs.Logger` with attributes `{request_id, op, fused_count, store_counts, errors_count, elapsed_seconds}`. The index SHALL be nil-safe across `obs.Obs`, `obs.Metrics`, individual collectors, and `obs.Logger` per the pattern at `internal/router/router.go:387-401` and `internal/llm/client.go:230-252`. The index SHALL NOT register or emit ANY new Prometheus metric family beyond `IndexOps`, `IndexOpDuration`, `IndexFusionDuration` (registered in `internal/obs/metrics/index.go`). | P0 | `TestEmitParentSpanWithAttributes` (in-memory OTel exporter; assert `index.search` span exists with all 4 attributes); `TestEmitChildStoreSpansAreChildren` (assert `index.qdrant`, `index.meili`, `index.pg` spans have `index.search` as parent via SpanContext); `TestEmitIndexOpsCounterPerStore` (3 stores × 1 search → 3 counter increments with correct (store, op, outcome) tuples); `TestEmitFusionDurationHistogramPerCall` (1 search → 1 fusion histogram observation; assert duration > 0); `TestEmitSlogIncludesRequestID` (ctx with `reqid.WithContext`; assert captured slog JSON contains the request_id and fused_count); `TestEmitSafeOnNilObs` (construct `*Index` with `Obs: nil`; Search/Upsert do not panic); `TestNoNewMetricFamilies` (snapshot `prometheus.Registry.Gather()` before+after; assert exactly 3 new families: IndexOps, IndexOpDuration, IndexFusionDuration; allowlist deltas: +`store`, +`op`). All in `observability_test.go`. |
| REQ-IDX-012 | State-Driven | WHILE the same `*Index` instance is invoked concurrently from N caller goroutines (N ≥ 1) — N caller goroutines each issuing a mix of `Search(ctx, q)` and `Upsert(ctx, docs)` calls — each call SHALL execute independently with no shared mutable state across calls (the `*Index` struct is immutable post-construction; the underlying `*qdrant.Client`, `meilisearch.ServiceManager`, and `*pgxpool.Pool` are themselves goroutine-safe per their own SDK contracts), the cumulative effect SHALL be N independent index operations with no race-detector alarms, AND the IndexOps counters SHALL never decrement across the workload. | P0 | `TestSearchUpsertConcurrent` in `concurrent_test.go`: 50 caller goroutines × 50 Search calls each + 10 ingester goroutines × 50 Upsert calls each (= 2,500 retrieval + 500 ingest × 10 docs/call = 5,000 ingestion-doc ops × 3 stores = 15,000 store invocations) under `go test -race ./internal/index/...`. Assertions: (1) zero race-detector alarms attributable to the index package; (2) every successful `*IndexResult` returned has `Stats.AdapterCount == 3`; (3) `goleak.VerifyNone(t)` after the test confirms zero residual goroutines; (4) IndexOps counter values are monotonically non-decreasing (verified by snapshotting at start + end of test). |
| REQ-IDX-013 | Event-Driven | WHEN `Index.Upsert(ctx, docs)` is invoked with `len(docs) > BulkBatchSize` (default 100), the index SHALL split docs into sequential batches of size `BulkBatchSize`, dispatch each batch to the three stores in parallel via the `parallelUpsert` helper, and aggregate `UpsertResult` across all batches (`Stats.DocCount` is total across batches; `Stats.PerStoreLatencies` is aggregate sum per store). The semaphore cap on parallel batches SHALL be 1 (sequential batches) in v0.1; intra-batch parallelism is bounded by `errgroup.SetLimit(3)` (one goroutine per store). | P1 | `TestUpsertBatchesLargeInput` (insert 250 docs with BulkBatchSize=100; assert 3 batches of [100, 100, 50]; final result.Stats.DocCount == 250; all 250 visible via subsequent Search after Meili WaitForTask); `TestUpsertHonoursBulkBatchSizeFromOptions` (BulkBatchSize=50; insert 150 docs; 3 batches; assert `Stats.PerStoreLatencies["qdrant"]` is sum-of-three-batch-latencies, not single-call latency). |
| REQ-IDX-014 | Ubiquitous | The package SHALL provide a deterministic `docID(sourceID, url string) string` function returning `hex(sha256(sourceID + "\x00" + url))[:16]`. The function SHALL be pure (no I/O, no time, no randomness), 16 hex chars (64 bits) wide, and stable across goroutines, processes, and replays — same input → byte-equal output. The `Embedder` interface SHALL be exposed as a port; the package SHALL ship a `zeroEmbedder` stub returning `make([][]float32, len(texts))` with each vector pre-allocated to 1024 zero floats. SPEC-IDX-002 wires the production BGE-M3 implementation by passing a different `Embedder` to `New`; the SPEC-IDX-001 surface does NOT change. | P0 | `TestDocIDDeterministic` (call docID("reddit", "https://example.com/a") twice; byte-equal output); `TestDocIDInputSensitive` (vary SourceID; vary URL; assert distinct doc_ids); `TestDocIDLength16` (assert `len(out) == 16`); `TestDocIDNullSeparatorPreventsCollision` (sourceID="redd", url="ithttps://x" must NOT collide with sourceID="reddit", url="https://x"); `TestZeroEmbedderDimensions1024` (call Dimensions(); assert == 1024); `TestZeroEmbedderEmbedReturnsZeros` (call Embed for 3 strings; assert 3 vectors of length 1024 each, all elements 0.0); `TestEmbedderInterfaceImplementedByZeroEmbedder` (compile-time check via `var _ Embedder = zeroEmbedder{}`). All in `docid_test.go` + `embedder_test.go`. |

---

## 4. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| NFR-IDX-001 | Performance (ingest throughput) | The synchronous `Index.Upsert` SHALL sustain at least 1000 docs/sec ingest throughput under a `BenchmarkUpsertBatch1000` workload that streams batches of 100 docs against testcontainers-backed Qdrant + Meili + PG instances on amd64. The benchmark runs as `go test -bench=BenchmarkUpsertBatch1000 -benchtime=10x -count=5 ./internal/index/...`. Median of 5 runs is the assertion value. The throughput is measured from the first batch start to the last batch's `Upsert` return, NOT to Meili index visibility (Meili is async per D12). Coverage of the benchmark in CI is the scheduled-weekly job (matching SPEC-OBS-001 NFR-OBS-001 cadence); per-PR runs use `-count=1 -benchtime=1x` smoke. |
| NFR-IDX-002 | Performance (retrieval latency budget) | `Index.Search` SHALL achieve p50 ≤ 100ms total latency (per-store calls + RRF fusion, pre-LLM-synthesis) and p95 ≤ 300ms total latency under a `BenchmarkSearchSteadyState` workload with 5,000 ingested docs and 100 concurrent search calls against testcontainers-backed stores on amd64. The benchmark runs as `go test -bench=BenchmarkSearchSteadyState -benchtime=100x -count=5`. Per-store contributions are reported in the bench output to inform NFR tuning. The 300ms p95 budget is an upper bound for the M3 retrieval-only path; LLM synthesis adds further latency owned by SPEC-SYN-004's NFRs. |
| NFR-IDX-003 | Race-clean concurrent invocation | `internal/index/concurrent_test.go::TestSearchUpsertConcurrent` SHALL execute successfully under `go test -race ./internal/index/...` with the workload defined in REQ-IDX-012: 50 caller goroutines × 50 Search calls each + 10 ingester goroutines × 50 Upsert calls each. Race-detector alarms attributable to the index package SHALL be zero. Cumulative call count: 2,500 retrieval invocations + 500 ingest invocations × ~10 docs/call = ~5,000 ingestion-doc ops, distributed across 3 stores = ~22,500 store invocations (including the 7,500 retrieval store calls). |
| NFR-IDX-004 | Zero goroutine leaks | The index SHALL pass `goleak.VerifyNone(t)` after every test that invokes `Search` or `Upsert`, including the success path, the partial-failure path, the all-failure path, the parent-ctx-cancellation mid-flight path, the per-store-timeout path, and the bulk-batch path. `internal/index/bench_test.go::TestMain` SHALL invoke `goleak.VerifyTestMain(m)` (mirrors `internal/fanout/bench_test.go` pattern). The index itself SHALL launch only the bounded errgroup workers (capped at `Options.MaxParallel = 3`); the pgxpool's internal connection-monitoring goroutines are documented exceptions and excluded via `goleak.IgnoreTopFunction("github.com/jackc/pgx/v5/pgxpool.(*Pool).backgroundHealthCheck")` (or equivalent function; exact ignore list confirmed during run phase against pgx v5.x). |
| NFR-IDX-005 | Allocation ceiling on retrieval hot path | `BenchmarkSearchSteadyState` SHALL report `allocs/op ≤ 5000` over a 5,000-doc index handling 100 concurrent search calls. This is a starting target derived from the RRF map (~50 allocs/doc × top-50), per-store ctx + cancel (~6 allocs/store × 3), errgroup state (~10 allocs/dispatch), and the slog/span emission (~50 allocs/dispatch). Adjustable downward in iteration 3 (HISTORY) once empirical baseline is established, mirroring the NFR-FAN-004 amendment pattern from SPEC-FAN-001 spec.md HISTORY. |

---

## 5. Acceptance Criteria

### REQ-IDX-001 — Index Construction and Public Surface

- File `internal/index/index.go` declares `Index` struct with the
  documented fields (`qd *qdrant.Client`, `me *meili.Client`,
  `pg *pg.Client`, `embedder Embedder`, `obs *obs.Obs`, `opts Options`).
- The compile-time signatures are in place:
  - `Search(ctx context.Context, q IndexQuery) (*IndexResult, error)`
  - `Upsert(ctx context.Context, docs []types.NormalizedDoc) (*UpsertResult, error)`
  - `Close() error`
- `New(Options{Embedder: nil})` returns `(nil, ErrEmbedderRequired)`.
- `New(Options{...})` with `AutoEnsureSchema: true` and a PG client that
  errors on EnsureSchema returns `(nil, err)` where
  `errors.Is(err, ErrSchemaBootstrapFailed)`.
- `New(Options{})` accepts zero-valued non-store fields and substitutes
  defaults: `MaxParallel=3`, per-store timeouts per §6.6,
  `RRFConstantK=60`, weights map populated with 1.0 per store key,
  `BulkBatchSize=100`, `AutoEnsureSchema=true`.
- `Close()` closes pgxpool, qdrant gRPC channel, and Meili client (no-op);
  returns the first non-nil error, with all close attempts run.
- `TestNewRequiresEmbedder`, `TestNewSchemaBootstrapFailureWraps`,
  `TestNewNormalisesDefaults`, `TestSearchAlwaysReturnsResultOrAllStoresFailed`,
  `TestCloseClosesAllStores` all pass.

### REQ-IDX-002 — Qdrant Sub-Client

- `internal/index/qdrant/client.go` exists with the named constructor
  `NewClient(cfg Config) (*Client, error)`.
- `EnsureCollection(ctx, "usearch_docs", 1024)` against a fresh
  testcontainers Qdrant creates the collection with
  `vector_size=1024, distance=Cosine, on_disk_payload=true`.
- Second call to `EnsureCollection` is a no-op (no error; collection
  unchanged).
- `Upsert(ctx, []Point{...})` with synthetic 1024-dim vectors round-trips
  correctly; `Search(ctx, vector, nil, 10)` returns the upserted points.
- Payload schema includes the documented fields; payload filter
  (`source_id == "reddit"`) restricts results.
- All four tests in §3 REQ-IDX-002 acceptance summary pass.

### REQ-IDX-003 — Meilisearch Sub-Client

- `internal/index/meili/client.go` exists with the named constructor.
- `EnsureIndex(ctx, "usearch_docs", IndexSettings{...})` against a fresh
  Meili creates the index with the documented settings (verified via
  `index.GetSettings()`).
- Second call is a no-op (settings match → PATCH skipped).
- `AddDocuments(ctx, "usearch_docs", []Document{...})` returns
  `(*TaskInfo, nil)` without blocking; production callers do not
  `WaitForTask`.
- After `WaitForTask(taskInfo.UID)` in the test, `Search(ctx, "alice", ...)`
  returns the matching docs.
- All four tests in §3 REQ-IDX-003 acceptance summary pass.

### REQ-IDX-004 — Validation Rejection

- `Index.Upsert(ctx, docs)` with 3 valid + 2 invalid docs returns
  `*UpsertResult` with `Stats.DocCount == 5`, `Stats.SkippedCount == 2`,
  `PerStoreErrors["validation"]` non-nil and containing 2 errors.
- The 3 valid docs are visible via subsequent `Search(ctx, q)`.
- ZERO PerStore counter increments for the 2 invalid docs in any store.
- ONE WARN slog record per batch (not per doc) with documented attributes.
- `TestUpsertRejectsInvalidDocs`, `TestUpsertEmptyBatchReturnsZeroes`
  pass.

### REQ-IDX-005 — Idempotent Multi-Store Upsert

- 3 valid docs ingested via `Upsert`: all 3 stores see all 3 docs after
  Meili `WaitForTask`. `Stats.DocCount == 3`. PG affected-rows on first
  insert: 3.
- Second `Upsert` of same 3 docs (replay): result.Stats.DocCount == 3 (in
  the pipeline) but PG affected-rows == 0 (ON CONFLICT DO NOTHING due
  to identical content_hash).
- `Upsert` of doc with edited Title (same SourceID + URL): doc_id
  unchanged; PG row's title and content_hash UPDATED via ON CONFLICT
  (doc_id) DO UPDATE branch; Qdrant point overwritten; Meili document
  overwritten on next WaitForTask.
- `Upsert` with PG client returning error: `result.PerStoreErrors["pg"]
  != nil`; qdrant + meili succeeded; `Stats.ErrorCount == 1`,
  `Stats.SuccessCount == 2`.
- All four tests in §3 REQ-IDX-005 acceptance summary pass.

### REQ-IDX-006 — Parallel Search and Fusion

- `Search(ctx, q{"alice", MaxResults: 10})` with 30 ingested docs:
  - All 3 stores produce non-empty rank lists.
  - `result.Docs` is non-empty and `len(result.Docs) <= 10`.
  - `result.Stats.PerStoreCounts["qdrant"] > 0`,
    `["meili"] > 0`, `["pg"] > 0`.
- Per-store timeout of 1ms forced for Meili: result has 2-store fused
  docs; `result.PerStoreErrors["meili"]` is non-nil and
  `errors.Is(*, context.DeadlineExceeded)`.
- All 3 stores rigged to fail: `errors.Is(err, ErrAllStoresFailed)`,
  `result == nil`.
- All four tests in §3 REQ-IDX-006 acceptance summary pass.

### REQ-IDX-007 — RRF Fusion Algorithm

- Single-store rank list of [d1@1, d2@2, d3@3]: output is [d1, d2, d3]
  with scores `1/61, 1/62, 1/63` (k=60).
- Two-store: doc A at rank 1 in store1 + rank 1 in store2; doc B at rank
  1 in store3 only. A's score = `1/61 + 1/61 = 2/61 ≈ 0.0328`; B's score
  = `1/61 ≈ 0.0164`. Output: A first, B second.
- Weights `{store1: 2.0, store2: 1.0}`, doc A rank 1 in store1, doc B
  rank 1 in store2: A's score = `2/61`; B's score = `1/61`.
- Two docs with identical scores: output ordered by doc_id ascending.
- Two runs on same input: byte-equal output.
- All six tests in §3 REQ-IDX-007 acceptance summary pass.

### REQ-IDX-008 — PostgreSQL Schema

- File `deploy/postgres/migrations/0001_create_docs.sql` exists and
  contains the documented column set.
- `EnsureSchema(ctx)` against fresh testcontainers PG creates all 15
  columns with correct types (verified via `information_schema.columns`).
- All required indexes exist (verified via `pg_indexes`).
- Second `EnsureSchema` call: no-op (no error).
- Drift case (manual column drop): `EnsureSchema` returns
  `errors.Is(err, ErrSchemaBootstrapFailed)`.
- All four tests in §3 REQ-IDX-008 acceptance summary pass.

### REQ-IDX-009 — Soft-Fail Partial-Result

- One-store-fails Search: result has 2-store fused docs;
  `PerStoreErrors[failedStore]` non-nil; `err == nil`.
- All-stores-fail Search: `result == nil`,
  `errors.Is(err, ErrAllStoresFailed)`.
- One-store-fails Upsert: result is non-nil; `PerStoreErrors[failedStore]`
  non-nil; siblings recorded normally; `err == nil`.
- All-stores-fail Upsert: result is non-nil with 3 entries in
  PerStoreErrors; `err == nil` (per soft-fail-for-upsert discipline,
  contrary to Search's all-fail path which DOES return error).
- All five tests in §3 REQ-IDX-009 acceptance summary pass.

### REQ-IDX-010 — Multi-Tenancy Reservation

- Empty `q.TeamID` in Search matches all rows including team_id=NULL.
- Non-empty `q.TeamID` filter excludes team_id=NULL rows (since v0.1
  inserts NULL universally, this means non-empty TeamID returns empty).
- All upserts in v0 set `team_id = NULL` in PG (verified via direct query).
- Qdrant payload includes `team_id` field (NULL value).
- Meili `index.GetSettings()` includes `team_id` in `filterableAttributes`.
- All five tests in §3 REQ-IDX-010 acceptance summary pass.

### REQ-IDX-011 — Per-Call Observability

- `TestEmitParentSpanWithAttributes`: in-memory OTel SpanRecorder; call
  Search; gather spans; assert one span named `index.search` with the 4
  documented attributes.
- `TestEmitChildStoreSpansAreChildren`: assert per-store spans
  (`index.qdrant`, `index.meili`, `index.pg`) have `index.search` as
  parent.
- `TestEmitIndexOpsCounterPerStore`: 3 stores × 1 search → 3 counter
  increments with correct (store, op, outcome) tuples.
- `TestEmitFusionDurationHistogramPerCall`: 1 search → 1 fusion histogram
  observation; assert duration > 0.
- `TestEmitSlogIncludesRequestID`: capture slog JSON; ctx with
  `reqid.WithContext(ctx, "TEST-IDX-REQ")`; assert exactly one slog record
  at INFO with attributes `request_id="TEST-IDX-REQ"`, `op="search"`, etc.
- `TestEmitSafeOnNilObs`: construct `*Index` with `Obs: nil`; Search/Upsert
  do not panic; return valid `*IndexResult`/`*UpsertResult`.
- `TestNoNewMetricFamilies`: snapshot the Prometheus registry's
  `Gather()` output before constructing Index; snapshot after Search +
  Upsert; assert exactly 3 new metric families: IndexOps,
  IndexOpDuration, IndexFusionDuration.

### REQ-IDX-012 — Concurrent-Safety State-Driven Contract

- `TestSearchUpsertConcurrent`:
  - Construct one `*Index` against testcontainers-backed stores.
  - Spawn 50 caller goroutines doing 50 Search calls each (= 2,500).
  - Spawn 10 ingester goroutines doing 50 Upsert calls each, each
    upsert with 10 docs (= 500 calls × 10 = 5,000 docs).
- Assertions:
  1. `go test -race` reports zero data-race alarms attributable to the
     index package.
  2. Every successful `*IndexResult.Stats.PerStoreCounts` reflects 3
     stores' contributions (≥0 each).
  3. `goleak.VerifyNone(t)` clean at test end.
  4. IndexOps counter values are monotonically non-decreasing
     (snapshot + assert).

### REQ-IDX-013 — Bulk Batch Discipline

- `Upsert(ctx, [250 docs])` with `BulkBatchSize=100`: 3 batches of
  [100, 100, 50]; final `Stats.DocCount == 250`.
- All 250 docs visible via `Search` after Meili `WaitForTask`.
- `Stats.PerStoreLatencies["qdrant"]` is sum of per-batch latencies, not
  the time of a single batch.
- `BulkBatchSize=50`, 150 docs: 3 batches of [50, 50, 50].

### REQ-IDX-014 — docID + Embedder Port Contract

- `docID("reddit", "https://example.com/a")` returns 16 hex chars; same
  input → byte-equal output across two calls.
- Different SourceIDs OR different URLs produce different doc_ids.
- NULL-separator prevents prefix collision: `docID("redd",
  "ithttps://x") != docID("reddit", "https://x")`.
- `zeroEmbedder.Dimensions() == 1024`.
- `zeroEmbedder.Embed(ctx, ["a", "b", "c"])` returns 3 zero-vectors of
  length 1024 each.
- Compile-time interface check: `var _ Embedder = zeroEmbedder{}`.

### NFR-IDX-001 — Ingest Throughput

- `BenchmarkUpsertBatch1000` invoked as
  `go test -bench=BenchmarkUpsertBatch1000 -benchtime=10x -count=5`
  on amd64 against testcontainers-backed stack.
- 1000 docs in 10 batches of 100 each.
- Median of 5 runs: total elapsed ≤ 1 second (= 1000 docs/sec).
- Bench reports `B/op` and `allocs/op` for telemetry.

### NFR-IDX-002 — Retrieval Latency

- `BenchmarkSearchSteadyState` with 5,000 pre-ingested docs.
- 100 concurrent search calls; per-call wall-clock measured.
- Median p50 ≤ 100ms; median p95 ≤ 300ms.
- Per-store latency contributions reported in bench output.

### NFR-IDX-003 — Race-Clean Concurrent Workload

- `TestSearchUpsertConcurrent` (REQ-IDX-012 acceptance) executes under
  `go test -race`; race-detector alarms attributable to the index
  package = 0.

### NFR-IDX-004 — Zero Goroutine Leaks

- `TestMain` in `bench_test.go`:
  ```
  func TestMain(m *testing.M) {
      goleak.VerifyTestMain(m,
          goleak.IgnoreTopFunction("github.com/jackc/pgx/v5/pgxpool.(*Pool).backgroundHealthCheck"),
      )
  }
  ```
  Mirrors `internal/fanout/bench_test.go`. Exact ignore list confirmed
  during run phase against pgx v5.x.
- Every Search/Upsert-invoking test SHALL pass `goleak.VerifyNone(t)`
  with the same ignore list.

### NFR-IDX-005 — Allocation Ceiling

- `BenchmarkSearchSteadyState` reports `allocs/op ≤ 5000` over a 5,000-
  doc index handling 100 concurrent search calls.

---

## 6. Technical Approach

### 6.1 Files to Modify

**Created (24 files)**:

- `internal/index/index.go` — `Index` struct, `New`, public method
  signatures
- `internal/index/index_test.go` — REQ-IDX-001 acceptance tests
- `internal/index/types.go` — `IndexQuery`, `IndexResult`, `UpsertResult`,
  `SearchStats`, `UpsertStats`
- `internal/index/options.go` — `Options` struct, defaults, validation
- `internal/index/options_test.go` — `New` validation tests
- `internal/index/embedder.go` — `Embedder` interface + `zeroEmbedder`
- `internal/index/embedder_test.go` — REQ-IDX-014 embedder tests
- `internal/index/dispatch.go` — `parallelSearch`, `parallelUpsert`
  helpers (errgroup orchestration, per-store ctx derivation)
- `internal/index/dispatch_test.go` — partial-failure / timeout tests
- `internal/index/rrf.go` — `fuseRRF` function
- `internal/index/rrf_test.go` — RRF table tests (REQ-IDX-007)
- `internal/index/docid.go` — `docID` deterministic generator
- `internal/index/docid_test.go` — REQ-IDX-014 docID tests
- `internal/index/observability.go` — `emitSearch` / `emitUpsert`
  helpers
- `internal/index/observability_test.go` — span/counter/slog tests
- `internal/index/concurrent_test.go` — NFR-IDX-003 race workload
- `internal/index/bench_test.go` — `BenchmarkUpsertBatch1000` +
  `BenchmarkSearchSteadyState` + `TestMain` with `goleak.VerifyTestMain`
- `internal/index/errors.go` — sentinel errors
- `internal/index/qdrant/client.go` — Qdrant sub-client
- `internal/index/qdrant/client_test.go` — REQ-IDX-002 acceptance
- `internal/index/meili/client.go` — Meilisearch sub-client
- `internal/index/meili/client_test.go` — REQ-IDX-003 acceptance
- `internal/index/pg/client.go` — PostgreSQL sub-client
- `internal/index/pg/client_test.go` — REQ-IDX-008 acceptance
- `internal/obs/metrics/index.go` — `IndexOps`, `IndexOpDuration`,
  `IndexFusionDuration` collectors
- `deploy/postgres/migrations/0001_create_docs.sql` — PG schema
- `.moai/config/sections/index.yaml` — NEW optional config (defaults)

**Modified (3 files)**:

- `internal/index/index.go` — replaces the 3-line stub at
  `internal/index/index.go:1-3`.
- `internal/obs/metrics/metrics.go` — call `registerIndex(r)` from
  `NewRegistry`; extend cardinality allowlist with `store` and `op`.
- `internal/obs/obs.go` — re-export `IndexOps`, `IndexOpDuration`,
  `IndexFusionDuration` from `obs.Obs.Metrics`.
- `go.mod` / `go.sum` — add three new direct dependencies.

**Unchanged (by design)**:

- `pkg/types/*` — no contract change required (NormalizedDoc is
  consumed as-is; Validate / CanonicalHash unchanged).
- `internal/router/router.go` — IR-001 is unchanged; IDX-001 does NOT
  depend on RoutingDecision.
- `internal/fanout/*` — FAN-001 is unchanged; IDX-001 consumes
  `[]NormalizedDoc` directly, NOT `fanout.Result`.
- `internal/adapters/*` — adapter contract unchanged.
- `deploy/docker-compose.yml` — Qdrant/Meili/PG services already
  present from SPEC-BOOT-001.
- `.moai/config/sections/quality.yaml` — `development_mode: tdd`,
  `test_coverage_target: 85` already in place.

### 6.2 Package Layout

```
internal/index/
├── index.go                                # Index struct, New, public method surface
├── index_test.go                           # REQ-IDX-001 acceptance
├── types.go                                # IndexQuery, IndexResult, UpsertResult, Stats
├── options.go                              # Options + defaults + validation
├── options_test.go
├── embedder.go                             # Embedder interface + zeroEmbedder stub
├── embedder_test.go
├── dispatch.go                             # parallelSearch, parallelUpsert helpers
├── dispatch_test.go                        # partial-failure, timeout, panic
├── rrf.go                                  # fuseRRF function
├── rrf_test.go
├── docid.go                                # docID deterministic generator
├── docid_test.go
├── observability.go                        # emitSearch, emitUpsert helpers
├── observability_test.go
├── errors.go                               # ErrAllStoresFailed, ErrEmbedderRequired, ErrSchemaBootstrapFailed
├── concurrent_test.go                      # NFR-IDX-003 race workload
├── bench_test.go                           # BenchmarkUpsertBatch1000, BenchmarkSearchSteadyState, TestMain (goleak)
├── qdrant/
│   ├── client.go                           # Qdrant sub-client (gRPC)
│   └── client_test.go                      # REQ-IDX-002 acceptance
├── meili/
│   ├── client.go                           # Meilisearch sub-client (HTTP)
│   └── client_test.go                      # REQ-IDX-003 acceptance
└── pg/
    ├── client.go                           # PostgreSQL sub-client (pgxpool)
    └── client_test.go                      # REQ-IDX-008 acceptance

internal/obs/metrics/
└── index.go                                # IndexOps/IndexOpDuration/IndexFusionDuration

deploy/postgres/migrations/
└── 0001_create_docs.sql                    # PG schema migration

.moai/config/sections/
└── index.yaml                              # NEW; optional operator tunables
```

### 6.3 Type Sketches (illustrative; final shapes in run phase)

```go
// internal/index/options.go
package index

const (
    defaultMaxParallel  = 3
    defaultRRFConstantK = 60
    defaultBulkBatchSize = 100
    defaultEmbedDim      = 1024
)

type Options struct {
    Qdrant            qdrant.Config
    Meili             meili.Config
    PG                pg.Config
    Embedder          Embedder
    Obs               *obs.Obs
    MaxParallel       int                          // default 3
    PerStoreTimeout   map[string]time.Duration     // see §6.6 defaults
    RRFConstantK      int                          // default 60
    RRFWeights        map[string]float64           // default 1.0 per store
    BulkBatchSize     int                          // default 100
    AutoEnsureSchema  bool                         // default true
}

// internal/index/index.go
type Index struct {
    qd       *qdrant.Client
    me       *meili.Client
    pg       *pg.Client
    embedder Embedder
    obs      *obs.Obs
    opts     Options
}

func New(opts Options) (*Index, error) {
    if opts.Embedder == nil {
        return nil, ErrEmbedderRequired
    }
    opts = applyDefaults(opts)
    qd, err := qdrant.NewClient(opts.Qdrant)
    if err != nil { return nil, fmt.Errorf("qdrant: %w", err) }
    me, err := meili.NewClient(opts.Meili)
    if err != nil { return nil, fmt.Errorf("meili: %w", err) }
    pgc, err := pg.NewClient(opts.PG)
    if err != nil { return nil, fmt.Errorf("pg: %w", err) }
    idx := &Index{qd: qd, me: me, pg: pgc, embedder: opts.Embedder, obs: opts.Obs, opts: opts}
    if opts.AutoEnsureSchema {
        if err := idx.ensureAllSchemas(context.Background()); err != nil {
            _ = idx.Close()
            return nil, fmt.Errorf("%w: %v", ErrSchemaBootstrapFailed, err)
        }
    }
    return idx, nil
}

// internal/index/types.go
type IndexQuery struct {
    Text       string
    Lang       string
    DocTypes   []types.DocType
    Since      time.Time
    Until      time.Time
    SourceID   string
    TeamID     string  // reserved for SPEC-IDX-004 enforcement
    MaxResults int     // default 50
}

type IndexResult struct {
    Docs           []types.NormalizedDoc
    PerStoreErrors map[string]error
    Stats          SearchStats
}

type UpsertResult struct {
    Inserted       int
    Skipped        int
    PerStoreErrors map[string]error  // "validation" key for pre-store rejections
    Stats          UpsertStats
}

type SearchStats struct {
    StoreLatencies   map[string]time.Duration
    FusionLatency    time.Duration
    PerStoreCounts   map[string]int
    FusedCount       int
    ElapsedSeconds   float64
}

type UpsertStats struct {
    DocCount          int
    SkippedCount      int
    SuccessCount      int  // per-store success count
    ErrorCount        int  // per-store error count
    PerStoreLatencies map[string]time.Duration
    ElapsedSeconds    float64
}

// internal/index/dispatch.go (Search hot path)
func (idx *Index) Search(ctx context.Context, q IndexQuery) (*IndexResult, error) {
    tracer := idx.tracer()
    spanCtx, span := tracer.Start(ctx, "index.search",
        oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
    defer span.End()

    start := time.Now()

    embedding, err := idx.embedder.Embed(spanCtx, []string{q.Text})
    if err != nil {
        return nil, fmt.Errorf("embed: %w", err)
    }
    vector := embedding[0]

    // Per-store ctxs derived per §2.4.
    rankLists, perStoreErrs, perStoreLat := idx.parallelSearch(spanCtx, q, vector)

    fusionStart := time.Now()
    fused := fuseRRF(rankLists, idx.opts.RRFWeights, idx.opts.RRFConstantK)
    fusionLat := time.Since(fusionStart)

    if len(fused) == 0 && len(perStoreErrs) == 3 {
        idx.emitSearchAllFailed(spanCtx, perStoreErrs, time.Since(start))
        return nil, ErrAllStoresFailed
    }

    // Clamp to MaxResults.
    if q.MaxResults > 0 && len(fused) > q.MaxResults {
        fused = fused[:q.MaxResults]
    }

    docs := make([]types.NormalizedDoc, len(fused))
    for i, f := range fused {
        docs[i] = f.Doc
    }
    stats := SearchStats{
        StoreLatencies: perStoreLat,
        FusionLatency:  fusionLat,
        PerStoreCounts: countsFromRankLists(rankLists),
        FusedCount:     len(fused),
        ElapsedSeconds: time.Since(start).Seconds(),
    }
    res := &IndexResult{Docs: docs, PerStoreErrors: perStoreErrs, Stats: stats}
    idx.emitSearch(spanCtx, span, res)
    return res, nil
}
```

### 6.4 Per-Store Context Derivation

```go
// internal/index/dispatch.go
func (idx *Index) deriveStoreCtx(parent context.Context, store string) (context.Context, context.CancelFunc) {
    deadline := idx.opts.PerStoreTimeout[store]
    if pDeadline, ok := parent.Deadline(); ok {
        if remaining := time.Until(pDeadline); remaining < deadline {
            deadline = remaining
        }
    }
    if deadline <= 0 {
        ctx, cancel := context.WithCancel(parent)
        cancel()
        return ctx, cancel
    }
    return context.WithTimeout(parent, deadline)
}
```

### 6.5 RRF Implementation Sketch

```go
// internal/index/rrf.go
type Ranked struct {
    DocID string
    Doc   types.NormalizedDoc
}

type FusedDoc struct {
    DocID string
    Doc   types.NormalizedDoc
    Score float64
}

func fuseRRF(rankLists map[string][]Ranked, weights map[string]float64, k int) []FusedDoc {
    scores := make(map[string]float64, 64)
    docs := make(map[string]types.NormalizedDoc, 64)
    for store, list := range rankLists {
        w, ok := weights[store]
        if !ok {
            w = 1.0
        }
        for i, r := range list {
            rank := i + 1 // 1-indexed
            scores[r.DocID] += w / (float64(k) + float64(rank))
            if _, seen := docs[r.DocID]; !seen {
                docs[r.DocID] = r.Doc
            }
        }
    }
    out := make([]FusedDoc, 0, len(scores))
    for id, s := range scores {
        out = append(out, FusedDoc{DocID: id, Doc: docs[id], Score: s})
    }
    sort.SliceStable(out, func(i, j int) bool {
        if out[i].Score != out[j].Score {
            return out[i].Score > out[j].Score
        }
        return out[i].DocID < out[j].DocID
    })
    return out
}
```

### 6.6 Default Per-Store Timeouts

```go
// internal/index/options.go
var defaultPerStoreTimeout = map[string]time.Duration{
    "qdrant": 200 * time.Millisecond,
    "meili":  300 * time.Millisecond,
    "pg":     100 * time.Millisecond,
}
```

The fusion budget `300ms` p95 (NFR-IDX-002) is bounded by `max(perStore)
= 300ms (meili) + RRF overhead < 5ms`.

### 6.7 Configuration

The index introduces ONE new optional config section:

```yaml
# .moai/config/sections/index.yaml (NEW; optional)
index:
  max_parallel: 3                       # NFR-IDX-003 / OQ §11.2
  per_store_timeout_ms:
    qdrant: 200                         # OQ §11.1
    meili: 300
    pg: 100
  rrf:
    constant_k: 60                      # paper default; OQ §11.1
    weights:                            # OQ §11.2
      qdrant: 1.0
      meili: 1.0
      pg: 1.0
  bulk_batch_size: 100                  # NFR-IDX-001 / OQ §11.6
  auto_ensure_schema: true              # OQ §11.7
qdrant:
  endpoint: localhost:6334              # gRPC
  api_key: ""                           # optional; for cloud Qdrant
  collection_name: usearch_docs
meili:
  endpoint: http://localhost:7700       # HTTP
  master_key: ${MEILI_MASTER_KEY}       # required
  index_name: usearch_docs
pg:
  conn_string: ${DATABASE_URL}
  max_conns: 6                          # 2 × MaxParallel
```

The CLI's `cmd/usearch/main.go` consumes this file and constructs
`Options` accordingly. v0.1 ships with all fields defaulted; a
SPEC-IDX-CFG-001 follow-up SPEC owns the full koanf-backed config loader
if needed.

### 6.8 MX Tag Plan

| File::Symbol | Tag | Reason |
|--------------|-----|--------|
| `index.go::(*Index).Search` | `@MX:ANCHOR` | Sole retrieval entry point. fan_in ≥ 4 (CLI today, MCP tomorrow, SPEC-CACHE-001 phase-0 lookup, future SPEC-RETRIEVE-001). `@MX:REASON: contract boundary; signature change ripples to CLI-001 + MCP-001 + CACHE-001 + IDX-005`. `@MX:SPEC: SPEC-IDX-001`. |
| `index.go::(*Index).Upsert` | `@MX:ANCHOR` | Sole ingestion entry point. fan_in ≥ 3 (CLI write-through, future bulk-ingest daemon, SPEC-IDX-005 team-shared answer reuse). `@MX:REASON: contract boundary; signature change ripples to consumers`. `@MX:SPEC: SPEC-IDX-001`. |
| `dispatch.go::parallelSearch` | `@MX:WARN` | Outbound fan-out spawns 3 goroutines. `@MX:REASON: removing the per-goroutine defer recover() / per-store ctx cancel sequence invalidates NFR-IDX-004 zero-leak guarantee`. |
| `dispatch.go::parallelUpsert` | `@MX:WARN` | Outbound fan-out spawns 3 goroutines per batch. `@MX:REASON: same goroutine-lifecycle invariant as parallelSearch`. |
| `dispatch.go::deriveStoreCtx` | `@MX:NOTE` | Magic constants (per-store defaults). The note documents §2.4 derivation rules and the pre-cancel branch when remaining ≤ 0. |
| `rrf.go::fuseRRF` | `@MX:ANCHOR` | Every fused result passes through this single transform. fan_in = 1 (Search) but invariant-bearing — bug here corrupts every IndexResult. `@MX:REASON: RRF formula and weighting invariant must not change without SPEC amendment`. |
| `docid.go::docID` | `@MX:ANCHOR` | Every ingested doc gets its identifier here. fan_in ≥ 5 (Upsert, qdrant.toPointID, meili.toDocument, pg.toRow, tests). `@MX:REASON: doc_id determinism is the load-bearing invariant for cross-store idempotency; changing the formula invalidates existing data`. `@MX:SPEC: SPEC-IDX-001`. |
| `qdrant/client.go::(*Client).Upsert` | `@MX:NOTE` | Documents the UUID-shaped point_id transformation from 16-hex doc_id. |
| `pg/client.go::(*Client).Upsert` | `@MX:NOTE` | Documents the two-key idempotency: ON CONFLICT (doc_id) DO UPDATE branch for content edits + UNIQUE (content_hash) for replay. |

All tags `[AUTO]`-prefixed, include `@MX:SPEC: SPEC-IDX-001`, follow
`code_comments: en` per `.moai/config/sections/language.yaml`. Per-file
hard limit (3 ANCHOR + 5 WARN per `.moai/config/sections/mx.yaml`):
respected.

### 6.9 Harness Level

Per `.moai/config/sections/harness.yaml` auto-routing: 14 EARS REQs
(12 × P0 + 2 × P1) + 5 NFRs touching 4 packages (`internal/index/`,
`internal/index/qdrant/`, `internal/index/meili/`, `internal/index/pg/`,
~25 source/test files) + 1 cross-package edit
(`internal/obs/metrics/{metrics.go,index.go}`, `internal/obs/obs.go`) +
1 SQL migration + 1 new optional config file = **standard** harness
level. Sprint Contract is OPTIONAL but recommended. Evaluator profile
`default` applies. Three new Go module dependencies — all verified
production-ready, no pre-1.0 risk — slightly elevate the harness
sensitivity but do not push to thorough.

---

## 7. What NOT to Build (Exclusions)

[HARD] This SPEC explicitly excludes the following items. Each has a known
destination SPEC; this list prevents scope creep into IDX-001.

- **BGE-M3 embedder implementation** → SPEC-IDX-002 (M3 per
  `.moai/project/roadmap.md:56`).
- **Korean tokenization (mecab-ko, custom Meili tokenizer)** →
  SPEC-IDX-003 (M3).
- **5-phase access fallback (insane-search pattern)** → SPEC-CACHE-001
  (M3 per `.moai/project/roadmap.md:58`).
- **Per-team multi-tenancy enforcement (Qdrant payload partitioning,
  Meili tenant tokens, PG row-level security)** → SPEC-IDX-004 (M6 per
  `.moai/project/roadmap.md:84`). v0.1 reserves `team_id` as NULL.
- **Team-shared answer reuse (pre-fanout cache hit)** → SPEC-IDX-005
  (M6 per `.moai/project/roadmap.md:85`).
- **Streaming retrieval / SSE / WebSocket result delivery** →
  SPEC-SYN-004 (M4 per `.moai/project/roadmap.md:66`).
- **Streaming bulk ingestion** → SPEC-DEEP-003 / SPEC-IDX-005 (deferred).
- **Compensating actions on partial-write failure (cross-store
  rollback)** → SPEC-IDX-005.
- **Per-store circuit breaker** → SPEC-EVAL-002 (M8).
- **Learned-to-rank (LTR) over per-store features** → future
  SPEC-RANK-001 (post-V1).
- **Cross-collection / cross-index migrations (alias swap)** →
  post-V1 SPEC-IDX-006.
- **In-process LRU cache for queries** → out of v0.1; the three stores'
  own caches handle warmup.
- **HTTP / gRPC API exposure of `Index`** → SPEC-MCP-001 (M7) and
  future SPEC-API-001.
- **Migration framework (golang-migrate / sql-migrate)** → out of v0.1;
  plain SQL execution at startup; future SPEC-MIGRATE-001.
- **Per-adapter ranking weight tuning** → out of v0.1; uniform `w_r =
  1.0`; SPEC-EVAL-001 (M8) author may propose tuned defaults.
- **Cardinality allowlist amendment beyond `store` + `op`** — both
  bounded enums.
- **GitHub Issue tracking on this SPEC** (skipped per session pattern).
- **CLI / FAN-001 wiring beyond construction at process init** — the
  decision of whether FAN-001 internally calls `Index.Upsert` or the CLI
  orchestrates is an Open Question (§11.5); v0.1 SPEC neither requires
  nor implements that wiring.

---

## 8. TDD Plan

Development follows RED-GREEN-REFACTOR per
`quality.development_mode: tdd` (`.moai/config/sections/quality.yaml`).
Representative RED-phase tests, written before implementation, grouped
by REQ. Total: ~50 tests + 2 benchmarks. Coverage target: 85% per
`quality.test_coverage_target`. Benchmarks do not count toward coverage.

| # | Test | Layer | REQ | Assertion |
|---|------|-------|-----|-----------|
| 1 | `TestNewRequiresEmbedder` | `index_test.go` | REQ-IDX-001 | `New(Options{Embedder:nil})` → `ErrEmbedderRequired` |
| 2 | `TestNewSchemaBootstrapFailureWraps` | `index_test.go` | REQ-IDX-001 | PG EnsureSchema errors → `errors.Is(err, ErrSchemaBootstrapFailed)` |
| 3 | `TestNewNormalisesDefaults` | `index_test.go` | REQ-IDX-001 | Zero Options non-store fields → defaults applied |
| 4 | `TestSearchAlwaysReturnsResultOrAllStoresFailed` | `index_test.go` | REQ-IDX-001 | Partial: `*IndexResult` non-nil + `err == nil`; All-fail: `nil` + `ErrAllStoresFailed` |
| 5 | `TestCloseClosesAllStores` | `index_test.go` | REQ-IDX-001 | Close invokes pgxpool.Close + qdrant.Close |
| 6 | `TestQdrantEnsureCollectionIdempotent` | `qdrant/client_test.go` | REQ-IDX-002 | Two calls; second is no-op |
| 7 | `TestQdrantUpsertRoundTrip` | `qdrant/client_test.go` | REQ-IDX-002 | Upsert + Search returns same point |
| 8 | `TestQdrantSearchHonoursLimit` | `qdrant/client_test.go` | REQ-IDX-002 | 50 points; query limit=10 → exactly 10 returned |
| 9 | `TestQdrantPayloadFiltering` | `qdrant/client_test.go` | REQ-IDX-002 | Filter by source_id |
| 10 | `TestMeiliEnsureIndexCreatesWithSettings` | `meili/client_test.go` | REQ-IDX-003 | Settings match expected |
| 11 | `TestMeiliEnsureIndexIdempotent` | `meili/client_test.go` | REQ-IDX-003 | Second call no-op |
| 12 | `TestMeiliAddDocumentsAsync` | `meili/client_test.go` | REQ-IDX-003 | Returns TaskInfo without WaitForTask |
| 13 | `TestMeiliSearchTextMatch` | `meili/client_test.go` | REQ-IDX-003 | After WaitForTask, query returns matching docs |
| 14 | `TestUpsertRejectsInvalidDocs` | `index_test.go` | REQ-IDX-004 | 3 valid + 2 invalid; Stats.SkippedCount=2 |
| 15 | `TestUpsertEmptyBatchReturnsZeroes` | `index_test.go` | REQ-IDX-004 | Empty batch; all Stats fields zero |
| 16 | `TestUpsertHappyPath3Docs` | `index_test.go` | REQ-IDX-005 | All 3 stores see all 3 docs |
| 17 | `TestUpsertIdempotentOnReplay` | `index_test.go` | REQ-IDX-005 | Second insert: PG affected-rows=0 |
| 18 | `TestUpsertEditedTitleUpdatesAllStores` | `index_test.go` | REQ-IDX-005 | doc_id stable; content updates |
| 19 | `TestUpsertSkipsRowOnPGFailure` | `index_test.go` | REQ-IDX-005 | PG fails; meili+qdrant succeed |
| 20 | `TestSearchHappyPath3Stores` | `index_test.go` | REQ-IDX-006 | All 3 stores contribute |
| 21 | `TestSearchHonoursMaxResults` | `index_test.go` | REQ-IDX-006 | Cap at 10 |
| 22 | `TestSearchPerStoreTimeoutDoesNotKillOthers` | `dispatch_test.go` | REQ-IDX-006 | Meili timeout; qdrant+pg succeed |
| 23 | `TestSearchAllStoresFailReturnsErr` | `dispatch_test.go` | REQ-IDX-006 | All fail → ErrAllStoresFailed |
| 24 | `TestRRFSingleStoreReturnsRankOrder` | `rrf_test.go` | REQ-IDX-007 | Identity case |
| 25 | `TestRRFTwoStoresAdditive` | `rrf_test.go` | REQ-IDX-007 | A in 2 stores > B in 1 store |
| 26 | `TestRRFWeightedFavoursHigherWeight` | `rrf_test.go` | REQ-IDX-007 | Weighted formula correctness |
| 27 | `TestRRFTieBreakDocIDAscending` | `rrf_test.go` | REQ-IDX-007 | Equal score → docID ascending |
| 28 | `TestRRFDeterministic` | `rrf_test.go` | REQ-IDX-007 | Two runs → byte-equal output |
| 29 | `TestRRFKCanonical60` | `rrf_test.go` | REQ-IDX-007 | k=60 verifies against worked numbers |
| 30 | `TestPGEnsureSchemaCreatesAllColumns` | `pg/client_test.go` | REQ-IDX-008 | All 15 columns with correct types |
| 31 | `TestPGEnsureSchemaIdempotent` | `pg/client_test.go` | REQ-IDX-008 | Second call no-op |
| 32 | `TestPGEnsureSchemaDetectsDrift` | `pg/client_test.go` | REQ-IDX-008 | Manual drop column → ErrSchemaBootstrapFailed |
| 33 | `TestPGEnsureSchemaIndexesExist` | `pg/client_test.go` | REQ-IDX-008 | UNIQUE on content_hash + B-tree on source_id, published_at, team_id |
| 34 | `TestSearchOneStoreTimesOutOthersSucceed` | `dispatch_test.go` | REQ-IDX-009 | Soft-fail with 2-store fusion |
| 35 | `TestSearchOneStoreFailsOthersSucceed` | `dispatch_test.go` | REQ-IDX-009 | Same shape; client error not timeout |
| 36 | `TestUpsertOneStoreFailsOthersSucceed` | `dispatch_test.go` | REQ-IDX-009 | Upsert no-rollback semantics |
| 37 | `TestSearchAllStoresFailReturnsErrAllStoresFailed` | `dispatch_test.go` | REQ-IDX-009 | All-fail Search returns err |
| 38 | `TestUpsertAllStoresFailReturnsResultWithThreeErrors` | `dispatch_test.go` | REQ-IDX-009 | All-fail Upsert returns result, no err |
| 39 | `TestSearchEmptyTeamIDMatchesAll` | `index_test.go` | REQ-IDX-010 | Empty TeamID matches NULL rows |
| 40 | `TestSearchTeamIDFilterMatchesNoneInV0` | `index_test.go` | REQ-IDX-010 | Non-empty TeamID excludes NULL |
| 41 | `TestUpsertSetsTeamIDNullInV0` | `index_test.go` | REQ-IDX-010 | All upserts set team_id=NULL |
| 42 | `TestQdrantPayloadIncludesTeamIDField` | `qdrant/client_test.go` | REQ-IDX-010 | Payload schema check |
| 43 | `TestMeiliFilterableAttributesIncludesTeamID` | `meili/client_test.go` | REQ-IDX-010 | Settings check |
| 44 | `TestEmitParentSpanWithAttributes` | `observability_test.go` | REQ-IDX-011 | `index.search` span with 4 attrs |
| 45 | `TestEmitChildStoreSpansAreChildren` | `observability_test.go` | REQ-IDX-011 | Per-store spans are children |
| 46 | `TestEmitIndexOpsCounterPerStore` | `observability_test.go` | REQ-IDX-011 | 3 stores → 3 counter increments |
| 47 | `TestEmitFusionDurationHistogramPerCall` | `observability_test.go` | REQ-IDX-011 | 1 search → 1 fusion observation |
| 48 | `TestEmitSlogIncludesRequestID` | `observability_test.go` | REQ-IDX-011 | request_id in slog |
| 49 | `TestEmitSafeOnNilObs` | `observability_test.go` | REQ-IDX-011 | Nil Obs → no panic |
| 50 | `TestNoNewMetricFamilies` | `observability_test.go` | REQ-IDX-011 | Exactly 3 new families |
| 51 | `TestSearchUpsertConcurrent` | `concurrent_test.go` | REQ-IDX-012, NFR-IDX-003 | 50×50 search + 10×50 upsert race-clean |
| 52 | `TestUpsertBatchesLargeInput` | `index_test.go` | REQ-IDX-013 | 250 docs / batch=100 → 3 batches |
| 53 | `TestUpsertHonoursBulkBatchSizeFromOptions` | `index_test.go` | REQ-IDX-013 | BulkBatchSize=50; 150 docs |
| 54 | `TestDocIDDeterministic` | `docid_test.go` | REQ-IDX-014 | Same input → byte-equal output |
| 55 | `TestDocIDInputSensitive` | `docid_test.go` | REQ-IDX-014 | Distinct inputs → distinct doc_ids |
| 56 | `TestDocIDLength16` | `docid_test.go` | REQ-IDX-014 | Output is 16 hex chars |
| 57 | `TestDocIDNullSeparatorPreventsCollision` | `docid_test.go` | REQ-IDX-014 | NUL separator is load-bearing |
| 58 | `TestZeroEmbedderDimensions1024` | `embedder_test.go` | REQ-IDX-014 | Returns 1024 |
| 59 | `TestZeroEmbedderEmbedReturnsZeros` | `embedder_test.go` | REQ-IDX-014 | All elements 0.0 |
| 60 | `TestEmbedderInterfaceImplementedByZeroEmbedder` | `embedder_test.go` | REQ-IDX-014 | Compile-time check |
| 61 | `BenchmarkUpsertBatch1000` | `bench_test.go` | NFR-IDX-001 | 1000 docs/sec sustained |
| 62 | `BenchmarkSearchSteadyState` | `bench_test.go` | NFR-IDX-002, NFR-IDX-005 | p50≤100ms, p95≤300ms, allocs/op≤5000 |
| 63 | `TestMain` (goleak.VerifyTestMain) | `bench_test.go` | NFR-IDX-004 | Package-level goroutine leak check |

RED-GREEN-REFACTOR per requirement:

1. RED: Write failing test for REQ-IDX-N.
2. GREEN: Implement minimal code to pass.
3. REFACTOR: Tidy; extract shared helpers if they remove duplication;
   keep file sizes manageable (target each `.go` file < 250 LoC
   excluding tests).

Greenfield note: `internal/index/` is a 3-line stub today; there is no
behaviour to preserve. The PG schema migration is a one-shot creation
event. Characterization tests are not needed; RED tests for REQ-IDX-001's
public API surface are written against the planned package surface.

---

## 9. Dependencies

### 9.1 Upstream SPEC Dependencies

- **SPEC-CORE-001 (implemented)**: provides `pkg/types.NormalizedDoc`,
  `Validate`, `CanonicalHash`, `pkg/types.DocType` enum,
  `pkg/types.Adapter` (consumed indirectly via FAN-001 output). HARD dep.
- **SPEC-BOOT-001 (implemented)**: provides Qdrant + Meilisearch +
  PostgreSQL compose services pre-running with healthchecks at
  `deploy/docker-compose.yml:31-87`. HARD dep — IDX-001 connects to
  these endpoints during integration tests and at runtime.
- **SPEC-OBS-001 (implemented)**: provides `obs.Logger`, `obs.Tracer`,
  `obs.WithRequestID`, the Prometheus registry via `obs.Metrics`, and
  the cardinality allowlist infrastructure that IDX-001 extends with
  `store` and `op` labels. HARD dep.

### 9.2 Parallelizable

- **SPEC-FAN-001 (approved per `.moai/specs/SPEC-FAN-001/spec.md:6`)**:
  IDX-001's ingestion path consumes `[]NormalizedDoc` from FAN-001's
  `Result.Docs`, but the contract is independent (slice-of-doc). FAN-001
  v0.1 implementation can proceed in parallel with IDX-001 v0.1
  implementation; they both gate the M3 exit criterion.
- **SPEC-IDX-002 (M3, blocks: this SPEC)**: IDX-002 plugs into IDX-001's
  `Embedder` port. Plan phase can begin as soon as IDX-001 spec.md is
  approved (the Embedder interface contract is in IDX-001).
- **SPEC-IDX-003 (M3, blocks: this SPEC)**: IDX-003 extends Meili index
  settings (custom tokenizer plugin + separate `ko` shard). Plan phase
  can begin as soon as IDX-001 spec.md is approved.
- **SPEC-CACHE-001 (M3, blocks: this SPEC)**: 5-phase access fallback's
  phase-0 calls `Index.Search`. Plan phase can begin once IDX-001
  spec.md is approved.

### 9.3 Downstream Blocked SPECs

- **SPEC-IDX-002** (M3): plugs into the `Embedder` port; the
  constructor change is `New(opts.Embedder = bgem3.New(...))` with no
  IDX-001 surface modification.
- **SPEC-IDX-003** (M3): extends Meili index settings with custom
  tokenizer; touches `internal/index/meili/client.go::EnsureIndex`
  signature additively.
- **SPEC-CACHE-001** (M3): wraps `Index.Search` as the index-hit phase
  of the 5-phase access fallback.
- **SPEC-IDX-004** (M6): flips `team_id` to `NOT NULL`, adds Qdrant
  payload partitioning, Meili tenant tokens, PG row-level security.
- **SPEC-IDX-005** (M6): team-shared answer reuse builds on
  `Index.Search` as the cache-hit branch.

### 9.4 External Dependencies (run-phase pins)

THREE new Go module dependencies introduced by IDX-001:

- `github.com/qdrant/go-client v1.17.0` — Qdrant gRPC client (Apache-2.0,
  verified 2026-05-04 via WebFetch — see research.md §2.1)
- `github.com/meilisearch/meilisearch-go v0.36.2` — Meilisearch HTTP
  client (MIT, verified 2026-05-04 — see research.md §2.2)
- `github.com/jackc/pgx/v5` (latest stable v5.x) — PostgreSQL driver +
  pgxpool (MIT, verified 2026-05-04 — see research.md §2.3)

ZERO new module dependencies for the rest of IDX-001:

- Go stdlib: `context`, `crypto/sha256`, `database/sql`, `encoding/hex`,
  `errors`, `fmt`, `runtime/debug`, `sort`, `strings`, `sync`, `time`
- `golang.org/x/sync/errgroup` (already pinned via `go.mod:33`
  `golang.org/x/sync v0.20.0`; SPEC-FAN-001 already uses)
- `pkg/types` (already pinned via SPEC-CORE-001)
- `internal/obs` and `internal/obs/reqid` and `internal/obs/metrics`
  (already pinned via SPEC-OBS-001)
- `go.opentelemetry.io/otel/{attribute,codes,trace}` (already pinned
  via SPEC-OBS-001)
- Test-only: `go.uber.org/goleak v1.3.0` (already pinned indirect via
  `go.mod:30`)
- Test-only: `github.com/testcontainers/testcontainers-go` (NEW indirect;
  pulled in for integration tests against real Qdrant/Meili/PG; build
  tag `integration` gates so `go test ./internal/index/...` without the
  tag uses stubs only)

---

## 10. Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Qdrant point_id format mismatch (16-hex doc_id vs UUID-shaped 32-hex) | Medium | High | §2.3 specifies left-pad + dash-insertion to satisfy RFC 4122 textual form; `TestQdrantUpsertRoundTrip` exercises end-to-end before commit |
| Meilisearch async indexing leaves test assertions racing | High | Medium | Production callers do NOT block; tests MUST call `WaitForTask(taskUID)` per D12; `TestMeiliAddDocumentsAsync` + WaitForTask pattern documented |
| pgxpool connection exhaustion under high concurrency | Medium | High | `max_conns = 6 = 2 × MaxParallel` default; configurable via `pg.max_conns`; pgx surfaces context-deadline on Acquire which IDX-001 reclassifies as a per-store timeout |
| pgxpool background goroutine triggers goleak alarm | High | Low | NFR-IDX-004 explicitly excludes via `goleak.IgnoreTopFunction`; exact ignore list confirmed during run phase |
| RRF k=60 default suboptimal for our domain | Low | Medium | Configurable via `index.yaml`; SPEC-EVAL-001 (M8) tunes after benchmark data |
| Three-store partial state after crash mid-Upsert | Medium | Medium | Idempotent doc_id-keyed upsert + `INSERT … ON CONFLICT` semantics → replay safety; v0.1 does NOT compensate; SPEC-IDX-005 owns consistency |
| Embedder port change breaks SPEC-IDX-002 wiring | Low | High | Interface is small (2 methods); SPEC-IDX-002 author reviews the interface during their plan phase |
| Multi-tenancy reservation forgotten in v0 → SPEC-IDX-004 has to retrofit visibility | Low | Medium | REQ-IDX-010 explicitly tests the `team_id` column existence and Qdrant/Meili filterable schema; SPEC-IDX-004 has a clean column to flip to NOT NULL |
| Bulk-batch sizing too aggressive (memory blowout on large batch) | Low | Low | Default 100 docs/batch × ~10 KB/doc ≈ 1 MB; multiple batches sequential per §6.5 |
| Qdrant-go-client v1.17.0 incompatibility with server v1.16.3 | Low | High | Verified compatible via Qdrant API stability guarantee; first integration test (TestQdrantEnsureCollectionIdempotent) catches incompatibility immediately |
| Meilisearch-go v0.36.2 incompatibility with server v1.42.1 | Low | High | Verified by README quote ("compatibility with version v1.x"); first integration test catches incompatibility |
| pgx v5 minor version drift | Low | Medium | Pin exact minor in run phase per SPEC-DEP-001 REQ-DEP-007 |
| RRF tie-breaker non-determinism under unstable map iteration | Low | Low | `sort.SliceStable` over score-decreasing key + docID ascending tie-break → byte-equal output for identical input; `TestRRFDeterministic` covers |
| Per-store timeout map missing key falls back to ∞ → over-long retrieval | Low | Medium | `applyDefaults` populates all three keys (`qdrant`, `meili`, `pg`); validation in `New` rejects missing-key Options |
| `INSERT ... ON CONFLICT (doc_id) DO UPDATE` PG performance under contention | Medium | Medium | M3 ingest target is 1000 docs/sec; PG handles ~10k inserts/sec on modest hardware with row-level UPSERT; revisit if NFR-IDX-001 fails |
| Qdrant on-disk payload (HDD-bound) slower than expected | Low | Low | `on_disk_payload=true` is a memory-saving knob; SSDs (default in dev/CI) keep payload-read latency under 1ms |
| Meili full-text reranker subordinates exact phrase to fuzzy match | Medium | Low | Default Meili ranking rules are sensible; SPEC-EVAL-003 (M8) tunes if Korean-locale benchmark exposes issues |
| Cardinality allowlist amendment rejected by NFR-OBS-002 test | Low | Low | `store ∈ 4 values, op ∈ 3 values` are hard enums; allowlist test extension is straightforward (`TestNoUnboundedLabels` already extended for LLM `provider` + `model`) |

---

## 11. Open Questions

These are explicitly UNRESOLVED at SPEC-approval time. Each has a
recommended default and a one-line resolution owner. They do NOT block
SPEC approval.

1. **RRF constant `k` default**. **Recommended default**: 60 per the
   Cormack/Clarke/Buettcher paper. Adopt v0 default; SPEC-EVAL-001
   (M8) author may override after citation-faithfulness benchmark.
   **Resolution owner**: SPEC-EVAL-001 author.

2. **RRF per-ranker weights default**. **Recommended default**: All
   weights = 1.0 (uniform). Operators tune via
   `.moai/config/sections/index.yaml`. **Resolution owner**:
   SPEC-EVAL-001 (M8) author may propose tuned defaults.

3. **doc_id width (16 hex / 64 bits)**. **Recommended default**: 16 hex
   chars in v0.1. Birthday-collision probability at V1 scale (10^7
   docs) is ~10^-7. At scale > 10^9 docs, extend to 24 hex chars
   (96 bits). **Resolution owner**: future SPEC-IDX-006 author at
   scale.

4. **Embedder dimension lock-in (1024)**. **Recommended default**: 1024
   in v0.1 matching BGE-M3. Qdrant collection vector_size is fixed at
   creation; future model changes require alias-swap migration.
   **Resolution owner**: SPEC-IDX-002 (BGE-M3 wiring) author.

5. **CLI / FAN-001 integration: write-through vs orchestrated**. Does
   FAN-001 internally call `Index.Upsert` after dispatch, or does the
   CLI orchestrate (`fanout.Dispatch` → `index.Upsert`)? **Recommended
   default**: CLI orchestrates. Keeps FAN-001 single-domain; SPEC-IDX-001
   stays consumable from multiple call patterns. The cache-hit
   retrieval branch (SPEC-IDX-005 M6) wires the BEFORE-fanout path.
   **Resolution owner**: SPEC-CLI-001 amendment author or new
   SPEC-RETRIEVE-001 author.

6. **Bulk-ingest backpressure**. v0.1 uses synchronous `Upsert` with
   internal batching. Should IDX-001 expose a streaming
   `IngestBatch(<-chan NormalizedDoc)` for back-pressure-aware
   producers? **Recommended default**: NO in v0.1. Synchronous API
   covers M3 adapter ingestion volume. **Resolution owner**:
   SPEC-DEEP-003 / SPEC-IDX-005 author.

7. **Meilisearch `WaitForTask` in production path**. v0.1 fires async
   without waiting; eventual consistency is the contract.
   **Recommended default**: Async fire-and-forget in production;
   `WaitForTask` in tests only. **Resolution owner**: SPEC-IDX-001
   author bakes this into REQ-IDX-005 + D12 of HISTORY.

8. **PG connection-pool sizing default**. v0.1 sets `max_conns = 6`
   (= 2 × MaxParallel). At scale, this saturates under heavy
   concurrent retrieval; symptom is pgx returning context-deadline on
   Acquire. **Recommended default**: 6 in v0.1; tuneable via
   `pg.max_conns` config. **Resolution owner**: SPEC-IDX-005 (M6)
   author may bump for team-scale.

9. **Per-store circuit breaker integration**. Should IDX-001 short-
   circuit a flapping store? FAN-001 explicitly defers this to
   SPEC-EVAL-002 (M8). IDX-001 follows the same posture. **Recommended
   default**: NO in v0.1. **Resolution owner**: SPEC-EVAL-002 (M8)
   author.

---

## 12. References

### External (URL-cited; verified per research.md §7)

- https://github.com/qdrant/go-client — Qdrant Go client v1.17.0
  (Apache-2.0, gRPC). Quoted in research §2.1.
- https://github.com/meilisearch/meilisearch-go — Meilisearch Go client
  v0.36.2 (MIT). Quoted in research §2.2.
- https://github.com/jackc/pgx — PostgreSQL driver v5 stable (MIT).
  Quoted in research §2.3.
- https://qdrant.tech/documentation/ — Qdrant overview; gRPC + REST
  APIs; Points/Collections/Payloads concepts. Quoted in research §2.1.
- https://qdrant.tech/documentation/concepts/collections/ — Qdrant
  collection schema, distance metrics, payload-based multitenancy.
  Quoted in research §2.5 + §3.7 + §3.9.
- https://www.meilisearch.com/docs — Meilisearch overview.
- https://www.meilisearch.com/docs/learn/getting_started/quick_start —
  Meilisearch quick start.
- https://www.meilisearch.com/docs/reference/api/documents — Meilisearch
  document API; idempotent upsert keyed on primary key. Quoted in
  research §2.2 + §3.4.
- http://cormack.uwaterloo.ca/cormacksigir09-rrf.pdf — Reciprocal Rank
  Fusion paper (Cormack/Clarke/Buettcher SIGIR 2009). Algorithm
  canonical; described in research §2.4.

### Internal (file:line cited)

- `.moai/specs/SPEC-IDX-001/research.md` — full research artifact
  (this SPEC's research sibling).
- `.moai/specs/SPEC-CORE-001/spec.md:139-146` — REQ-CORE-001..007
  NormalizedDoc / Validate / CanonicalHash / Adapter / Registry contract.
- `.moai/specs/SPEC-BOOT-001/spec.md:61-62` — REQ-BOOT-004 + REQ-BOOT-005
  compose service guarantees (Qdrant/Meili/PG running with
  healthchecks).
- `.moai/specs/SPEC-OBS-001/spec.md:88-93` — REQ-OBS-001..006 baseline
  collectors; IDX-001 mirrors LLM-001's pattern for sole-emitter
  discipline.
- `.moai/specs/SPEC-OBS-001/spec.md:101` — NFR-OBS-002 cardinality
  safety; IDX-001 extends allowlist with `store` and `op`.
- `.moai/specs/SPEC-LLM-001/spec.md:548-572` — `internal/obs/metrics/llm.go`
  pattern; IDX-001 mirrors with `internal/obs/metrics/index.go`.
- `.moai/specs/SPEC-FAN-001/spec.md:6` — FAN-001 status: approved.
- `.moai/specs/SPEC-FAN-001/spec.md:296` — REQ-FAN-001 Fanout/Dispatch
  contract.
- `.moai/specs/SPEC-FAN-001/spec.md:466-531` — §2.5 per-adapter timeout
  derivation; IDX-001 §2.4 mirrors.
- `.moai/specs/SPEC-FAN-001/spec.md:765-783` — TestDispatchConcurrent
  reference workload; IDX-001 NFR-IDX-003 mirrors.
- `.moai/specs/SPEC-FAN-001/spec.md:843-849` — `goleak.VerifyTestMain`
  pattern; IDX-001 NFR-IDX-004 reuses with pgxpool ignore list.
- `pkg/types/normalized_doc.go:40-56` — NormalizedDoc 15 fields.
- `pkg/types/normalized_doc.go:63-77` — Validate (required fields).
- `pkg/types/normalized_doc.go:91-106` — CanonicalHash (content-only).
- `internal/index/index.go:1-3` — current 3-line stub.
- `internal/obs/metrics/metrics.go:33-65` — observability collector
  declarations.
- `internal/obs/metrics/metrics.go:89-95` — FanoutInflight precedent.
- `internal/obs/metrics/metrics.go:171-176` — cardinality allowlist
  (extended by IDX-001).
- `internal/router/router.go:341-401` — `emit` + nil-safe pattern;
  IDX-001 `emitSearch`/`emitUpsert` mirror.
- `internal/llm/client.go:230-252` — observability emission pattern;
  IDX-001 mirrors.
- `deploy/docker-compose.yml:31-46` — Qdrant compose service.
- `deploy/docker-compose.yml:48-66` — Meilisearch compose service.
- `deploy/docker-compose.yml:68-87` — PostgreSQL compose service.
- `go.mod:3` — Go 1.25.8 baseline.
- `go.mod:30` — `go.uber.org/goleak v1.3.0` indirect.
- `go.mod:33` — `golang.org/x/sync v0.20.0` indirect.
- `.moai/project/structure.md:35-38` — `internal/index/{qdrant,meilisearch,
  postgres}` reservation.
- `.moai/project/structure.md:160` — `pkg/types` SDK boundary clause.
- `.moai/project/research.md:103-104` — Qdrant multitenancy + Meili
  hybrid anchor docs.
- `.moai/project/roadmap.md:55` — M3 row "SPEC-IDX-001 | Hybrid index
  layer | Qdrant + Meilisearch + PG Go clients, RRF fusion, ingestion
  pipeline".
- `.moai/project/roadmap.md:84` — SPEC-IDX-004 multi-tenancy gate (M6).
- `.moai/project/roadmap.md:117-128` — M3 parallelization plan;
  IDX-001 listed as parallelizable with FAN-001 once FAN-001 spec.md
  is approved (which has happened).
- `.moai/project/roadmap.md:150` — M3 exit criterion.
- `.moai/config/sections/quality.yaml` — `development_mode: tdd`,
  `test_coverage_target: 85`.
- `.moai/config/sections/harness.yaml` — auto-routing to standard
  level.
- `.moai/config/sections/language.yaml` — `documentation: en`,
  `code_comments: en`.

---

*End of SPEC-IDX-001 v0.1 (status: draft; pending plan-auditor cycle)*
