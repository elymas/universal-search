# SPEC-IDX-001 Research — Hybrid Index Layer (Qdrant + Meilisearch + PostgreSQL)

**Status**: Research-complete; ready for SPEC drafting
**Author**: limbowl (via manager-spec)
**Created**: 2026-05-04
**Milestone**: M3 — Fanout, adapters, index
**Depends on**: SPEC-CORE-001, SPEC-BOOT-001, SPEC-OBS-001
**Parallelizable with**: SPEC-FAN-001 (M3 gateway), SPEC-IDX-002 (BGE-M3 embedder), SPEC-IDX-003 (Korean tokenization)

---

## 0. Research Mandate

SPEC-IDX-001 (Hybrid Index Layer) is the M3 retrieval foundation. It replaces
the 3-line stub at `internal/index/index.go:1-3` with a production package
tree at `internal/index/{qdrant,meili,pg}/` plus the top-level `internal/index/`
orchestrator that:

1. Provides three Go clients (Qdrant gRPC, Meilisearch HTTP, PostgreSQL via
   `pgx`) backed by the compose services already wired by SPEC-BOOT-001
   (`deploy/docker-compose.yml:31-87`).
2. Defines a deterministic `doc_id` derived from `SourceID + URL` (SHA-256
   first 16 hex), so parallel ingestion across goroutines and across stores
   produces a stable identifier without a DB sequence.
3. Implements an idempotent `Upsert([]NormalizedDoc)` pipeline that fans the
   same logical document into all three stores (`Qdrant.UpsertPoints`,
   `Meili.AddDocuments`, `PG INSERT … ON CONFLICT (content_hash) DO NOTHING`).
4. Implements a parallel `Search(query)` that issues per-store fetches with
   per-store timeout, fuses the rank lists via Reciprocal Rank Fusion (RRF,
   `k=60`), and returns ranked `[]NormalizedDoc` to the caller.
5. Reserves the per-team `team_id` field on the PG `docs` table as `NULL` in
   v0; SPEC-IDX-004 (M6 multi-tenancy per `.moai/project/roadmap.md:84`) is
   the SPEC that will enforce visibility rules.
6. Defers the embedding step (BGE-M3 inference) to SPEC-IDX-002 — IDX-001
   declares an `Embedder` interface port and a stub implementation that
   returns a zero-vector; the Qdrant collection is created with
   `vector_size=1024` to match BGE-M3, but populated vectors only land when
   IDX-002 wires the real embedder.
7. Closes the M3 exit-criterion infrastructure (`.moai/project/roadmap.md:150`
   — "`usearch query` returns fused results across ≥5 adapters") together
   with SPEC-FAN-001 (multi-source fanout) and SPEC-CACHE-001 (5-phase
   access fallback). FAN-001 produces `[]NormalizedDoc` from live adapters;
   IDX-001 ingests them and serves the retrieval path.

This research catalogs (a) the existing-code state IDX-001 must respect,
(b) the external library landscape (qdrant-go-client, meilisearch-go,
jackc/pgx) with verified versions, (c) the design alternatives and rejection
rationale, (d) the integration surface — does FAN-001 wrap IDX-001 or does a
new SPEC-RETRIEVE-001 own that orchestration, (e) the ingestion pipeline
architecture, (f) race / leak / cancellation analysis, (g) multi-tenancy
reservation for M6, and (h) open questions that the plan-auditor will
challenge.

Every claim is either file-cited (e.g.,
`internal/obs/metrics/metrics.go:89-95`) or URL-cited from verified web
sources. No invented facts.

---

## 1. Existing-Code State

### 1.1 The `internal/index/` Stub

The index package is currently a 3-line stub:

```go
// Package index is the stub for the hybrid index layer (Qdrant + Meilisearch + PG).
// Full implementation lands in SPEC-IDX-001.
package index
```

Source: `internal/index/index.go:1-3`. Reservation made by SPEC-BOOT-001 and
reaffirmed in `.moai/project/structure.md:35-38`:

```
│   ├── index/                    # Hybrid index client (SPEC-IDX)
│   │   ├── qdrant/
│   │   ├── meilisearch/
│   │   └── postgres/
```

The package directory contains exactly one file (`index.go`). No tests, no
sub-packages. SPEC-IDX-001 fills the package end-to-end, mirroring the depth
that SPEC-LLM-001 and SPEC-FAN-001 set for `internal/llm/` and
`internal/fanout/` respectively.

### 1.2 The `NormalizedDoc` Input Contract (Inherited)

The data shape every adapter produces, inherited from SPEC-CORE-001:

```go
type NormalizedDoc struct {
    ID          string         `json:"id"`
    SourceID    string         `json:"source_id"`
    URL         string         `json:"url"`
    Title       string         `json:"title"`
    Body        string         `json:"body"`
    Snippet     string         `json:"snippet"`
    PublishedAt time.Time      `json:"published_at"`
    RetrievedAt time.Time      `json:"retrieved_at"`
    Author      string         `json:"author"`
    Score       float64        `json:"score"`
    Lang        string         `json:"lang"`
    DocType     DocType        `json:"doc_type"`
    Citations   []string       `json:"citations,omitempty"`
    Metadata    map[string]any `json:"metadata,omitempty"`
    Hash        string         `json:"hash"`
}
```

Source: `pkg/types/normalized_doc.go:40-56`.

Required fields per `Validate` (`pkg/types/normalized_doc.go:63-77`):
`{ID, SourceID, URL, RetrievedAt}`. SPEC-IDX-001 calls `doc.Validate()`
once at the ingestion boundary; invalid docs are rejected with a typed
error and never reach any of the three stores.

`CanonicalHash()` (`pkg/types/normalized_doc.go:91-106`) returns 16 hex
chars derived from `{SourceID, URL, Title, Body}` (NUL-separated SHA-256,
truncated to first 16 hex). This hash is content-only; `Metadata` is
intentionally excluded so adapter-specific enrichment cannot produce false
dedup misses.

[HARD] IDX-001 uses `CanonicalHash()` as the `content_hash` column on the PG
`docs` table (UNIQUE constraint; provides cross-process idempotency for
`INSERT … ON CONFLICT DO NOTHING`).

### 1.3 The `pkg/types` SDK Boundary (Frozen)

`.moai/project/structure.md:160` declares: "Breaking changes here require a
major-version bump for any external Go consumer building their own Adapter
implementation."

[HARD] SPEC-IDX-001 introduces ZERO changes to `pkg/types`. The `Embedder`
port, the `Index` orchestrator type, the `IndexQuery` / `IndexResult` value
types, and the per-store error sentinels all live under `internal/index/`.
Down-stream consumers (CLI, MCP, future SPEC-RETRIEVE-001) reach the index
through `internal/index.Index` — not through `pkg/types`.

### 1.4 Compose Stack State (Inherited from SPEC-BOOT-001)

The three target services are already running and pinned in
`deploy/docker-compose.yml`:

| Service | Image | Port | Healthcheck | Volume |
|---------|-------|------|-------------|--------|
| qdrant | `qdrant/qdrant:v1.16.3` (`docker-compose.yml:32`) | 6333 (HTTP), 6334 (gRPC) (lines 33-35) | `wget http://localhost:6333/readyz` (line 39) | `qdrant_data:/qdrant/storage` (line 37) |
| meilisearch | `getmeili/meilisearch:v1.42.1` (`docker-compose.yml:50`) | 7700 (line 52) | `wget http://localhost:7700/health` (line 59) | `meili_data:/meili_data` (line 57) |
| postgres | `postgres:16.13-alpine3.23` (`docker-compose.yml:70`) | 5432 (line 72) | `pg_isready -U $POSTGRES_USER -d $POSTGRES_DB` (line 80) | `pg_data:/var/lib/postgresql/data` (line 78) |

All three carry `restart: unless-stopped`, healthchecks with
`interval=10s, timeout=5s, retries=5, start_period=30s`, and join the same
`app` bridge network (lines 18-20). The Meilisearch master key flows from
`${MEILI_MASTER_KEY}` (line 54) — no hardcoded secret. PostgreSQL credentials
flow from `${POSTGRES_USER}/${POSTGRES_PASSWORD}/${POSTGRES_DB}` (lines 74-76).

REQ-BOOT-004 acceptance was satisfied in the SPEC-BOOT-001 implementation
(lines 117-123 of that SPEC); IDX-001 may assume the stack is reachable
during run-phase integration tests.

[HARD] IDX-001 introduces **zero compose deltas**. The clients connect to
the existing services. The only new on-disk artifact is a single migration
file (`deploy/postgres/migrations/0001_create_docs.sql` or equivalent),
applied by IDX-001's bootstrap routine on first startup.

### 1.5 Observability Baseline (Inherited from SPEC-OBS-001)

`internal/obs/metrics/metrics.go` declares the `Registry` struct that
exposes named collectors. SPEC-FAN-001 inherited the `FanoutInflight` Gauge
without adding new metric families. SPEC-LLM-001 added one new family
(`LLMCalls`/`LLMCost`/`LLMLatency`).

[HARD] SPEC-IDX-001 introduces ONE new metric family group and extends the
cardinality allowlist (`internal/obs/metrics/metrics.go:171`):

- `usearch_index_ops_total{store, op, outcome}` — Counter
- `usearch_index_op_duration_seconds{store, op}` — Histogram
- `usearch_index_fusion_duration_seconds` — Histogram (no labels)

Bounded label values:
- `store ∈ {qdrant, meili, pg, fusion}` (4 values)
- `op ∈ {upsert, search, ensure_schema}` (3 values)
- `outcome ∈ {success, failure, timeout}` (3 values, reusing the existing
  `outcome` allowlist value already pinned at `metrics.go:172`)

Total label-value cardinality: ≤ 4 × 3 × 3 = 36 series per query path —
well within Prometheus single-instance limits.

[HARD] IDX-001 does NOT register per-store sub-metrics (e.g., per-Qdrant-
collection counters). Per-collection / per-tenant cardinality is a
multi-tenancy concern owned by SPEC-IDX-004 (M6).

### 1.6 RoutingDecision and FAN-001 Output (Pre-condition)

SPEC-IR-001 publishes `router.RoutingDecision` (`internal/router/routing_decision.go:23-37`).
SPEC-FAN-001 v0.1 (status: approved) consumes the `AdapterSet` and produces
`fanout.Result{Docs, AdapterErrors, Stats}` per
`.moai/specs/SPEC-FAN-001/spec.md:296` (REQ-FAN-001 / REQ-FAN-002).

IDX-001's ingestion path consumes `[]NormalizedDoc` — typically the
`fanout.Result.Docs` slice, but the contract is independent. The retrieval
path (`Index.Search`) returns its own `IndexResult{Docs, Stats}` value type
that IS NOT identical to `fanout.Result` (different Stats semantics, no
adapter-keyed error map). The integration question — whether FAN-001's
caller invokes `Index.Upsert(result.Docs)` synchronously or asynchronously,
and whether `Index.Search` is invoked BEFORE fanout (cache-hit path) or
ONLY for re-ranking — is deferred to Open Question §6.5.

### 1.7 Existing Concurrency Dependencies

Pinned in `go.mod` (verified at `go.mod:33`):

- `golang.org/x/sync v0.20.0` (indirect): provides `errgroup` and
  `semaphore`. Already used by SPEC-FAN-001.
- `go.uber.org/goleak v1.3.0` (indirect, `go.mod:30`): goroutine-leak
  verification. Already used by Reddit / HN adapter benchmarks.

NOT pinned (NEW direct dependencies introduced by IDX-001):

- `github.com/qdrant/go-client` (vector store gRPC client)
- `github.com/meilisearch/meilisearch-go` (Meili HTTP client)
- `github.com/jackc/pgx/v5` (PostgreSQL driver + pool)

All three are ASF/MIT licensed and verified production-ready (§2 below).

### 1.8 The `Adapter` Concurrent-Safety Contract (Already Met)

ADP-001 REQ-ADP-011 (`.moai/specs/SPEC-ADP-001/spec.md:373-374`) guarantees
50 goroutines × one `Search` against a single `*Adapter` is race-clean. This
is upstream of IDX-001 — adapters produce docs, fanout merges them, IDX-001
ingests them. The race-cleanliness IDX-001 owns is internal: 50 caller
goroutines × one `Index.Search` and 10 ingester goroutines × one
`Index.Upsert` against a single `*Index` instance are race-clean (NFR-IDX-003
below).

---

## 2. External Library Survey

### 2.1 `github.com/qdrant/go-client`

URL: https://github.com/qdrant/go-client
Verified: 2026-05-04 via WebFetch.

**Status**: v1.17.0 stable (released 2026-02-19), Apache-2.0 license.

**Module path**: `github.com/qdrant/go-client`. Install via `go get -u`.

**Protocol**: gRPC under the hood. Quoted from README: "Internally, the
high-level client uses a low-level gRPC client to interact with Qdrant."
This means IDX-001 connects to port `6334` (gRPC), NOT `6333` (HTTP) — both
ports are exposed by the compose service (`deploy/docker-compose.yml:34-35`).

**API surface** (high-level):
- `qdrant.NewClient(*qdrant.Config) (*qdrant.Client, error)` — constructor
- `client.CreateCollection(ctx, *qdrant.CreateCollection)` — collection setup
- `client.Upsert(ctx, *qdrant.UpsertPoints)` — idempotent point write
- `client.Query(ctx, *qdrant.QueryPoints)` — vector search
- `client.HealthCheck(ctx)` — readiness probe

**Server compatibility**: The Go client is maintained in lockstep with the
Qdrant server. Compose pins `qdrant/qdrant:v1.16.3`; client v1.17.0 is
backward-compatible with server v1.16+ per Qdrant's API stability guarantee
(qdrant.tech documentation, §1.4 above). The pin gap (server v1.16.3 vs
client v1.17.0) is intentional — the Go client tracks ahead of the server
to absorb new features without server-side upgrade.

**Idempotent upsert**: Point IDs must be either UUIDs or unsigned integers
(qdrant docs, §2.5 below). IDX-001 uses the SHA-256 prefix as a UUID-shaped
hex string (32 chars; passed via `qdrant.PointId{Uuid: hexStr}` after
re-formatting to UUID v5 shape) — confirms during run phase whether UUID
parsing accepts our 32-char hex without dashes; if not, dashes are inserted
at positions 8/12/16/20 to satisfy RFC 4122 textual form.

[HARD] IDX-001 selects qdrant-go-client v1.17.0 as the Qdrant client.
Rejection of alternatives: HTTP-only client via `net/http` is possible but
loses the gRPC streaming benefit and requires manual schema serialisation.
The official library handles both.

### 2.2 `github.com/meilisearch/meilisearch-go`

URL: https://github.com/meilisearch/meilisearch-go
Verified: 2026-05-04 via WebFetch.

**Status**: v0.36.2 (released 2026-04-13). MIT license. Go 1.21+ required;
project uses Go 1.25 per `go.mod:3`.

**Module path**: `github.com/meilisearch/meilisearch-go`.

**Server compatibility**: "Guarantees compatibility with version v1.x of
Meilisearch" per README. Compose pins `getmeili/meilisearch:v1.42.1`
(`deploy/docker-compose.yml:50`) — well within the v1.x guarantee.

**API surface**:
- `meilisearch.New(host, opts ...Option) ServiceManager` — client factory
- `client.Index(uid)` → `IndexManager` — handle to a specific index
- `index.AddDocuments(documents, primaryKey *string)` — idempotent upsert
- `index.UpdateSettings(*Settings)` — searchable attributes, ranking rules,
  distinct attribute, stop-words
- `index.Search(query, *SearchRequest) (*SearchResponse, error)` — full
  text search with ranking, filtering, faceting

**Idempotent semantics** (verified via Meilisearch docs reference page
https://www.meilisearch.com/docs/reference/api/documents): "POST
`/indexes/{uid}/documents` — Submitting a document with an existing primary
key value will replace that document. … This ensures predictable behavior
regardless of how many times the same request is sent."

[HARD] IDX-001 uses `doc_id` (the SHA-256-derived 16-hex string) as the
Meilisearch primary key. The `usearch_docs` index is configured with
`primaryKey: "doc_id"`, ensuring repeated upserts of the same logical
document are de-duplicated server-side.

**Async semantics**: All write operations return a `TaskInfo` (the Meili
server queues the work). Tests must call `client.WaitForTask(taskUID)` to
synchronise with indexing completion before asserting search visibility.
This is a Meili characteristic, NOT a client choice; IDX-001 acceptance
tests honour it.

### 2.3 `github.com/jackc/pgx/v5`

URL: https://github.com/jackc/pgx
Verified: 2026-05-04 via WebFetch.

**Status**: v5 is the latest stable major version. Quoted from README:
"`v5` is the latest stable major version. … Supports Go 1.25+ and PostgreSQL
14+ following their respective team support policies."

Compose pins `postgres:16.13-alpine3.23` (`deploy/docker-compose.yml:70`) —
within pgx's v14+ support.

**Module path**: `github.com/jackc/pgx/v5` (note `/v5` suffix — Go modules
major-version path requirement).

**Pool API**: `github.com/jackc/pgx/v5/pgxpool` — connection pooling with
`pgxpool.New(ctx, connString)` constructor and `pool.Acquire()`,
`pool.Query()`, `pool.QueryRow()` operations. Quoted: "The toolkit includes
`pgxpool` for connection pooling with after-connect hook support."

**Choice rationale**:
- pgx is pure Go; no CGo dep (matches the SPEC-DEP-001 "vendor minimal cgo"
  spirit).
- pgx supports PostgreSQL-specific features (`COPY` for bulk ingest,
  `LISTEN/NOTIFY` for future tail-following).
- The standard `database/sql` adapter is also exposed via
  `github.com/jackc/pgx/v5/stdlib` for libraries that need the `*sql.DB`
  shape — IDX-001 uses pgx's native API (`pgxpool.Pool`) for performance.

[HARD] IDX-001 selects `pgx/v5` + `pgxpool` as the PostgreSQL driver pair.
Rejection of alternatives: `database/sql` + `lib/pq` is the older idiom;
`lib/pq` is in maintenance mode per its README. `gorm` adds an ORM layer
that is unnecessary for IDX-001's single-table schema.

### 2.4 RRF (Reciprocal Rank Fusion)

**Source**: Cormack, Clarke, Buettcher. "Reciprocal Rank Fusion outperforms
Condorcet and Individual Rank Learning Methods." SIGIR 2009.
URL: https://plg.uwaterloo.ca/~gvcormac/cormacksigir09-rrf.pdf
(redirects to http://cormack.uwaterloo.ca/cormacksigir09-rrf.pdf —
verified accessible 2026-05-04 via WebFetch HTTP 200; PDF content not
inline-quotable here, but the algorithm is canonical and described
verbatim in the Microsoft Azure AI Search docs, OpenSearch docs, and
Elastic docs which all cite the same paper).

**Algorithm**:

```
RRFscore(d in D) = sum over r in R of 1 / (k + rank_r(d))
```

Where:
- `D` is the union of documents across rankers.
- `R` is the set of rankers (here: `{qdrant, meili, pg}`).
- `rank_r(d)` is the 1-indexed rank of document `d` in ranker `r`'s output
  (lower = better; documents not in `r`'s output contribute zero from
  that ranker).
- `k` is a constant; the original paper uses `k=60` and IDX-001 follows
  this default (configurable via `Options.RRFConstantK`).

**Properties**:
- Monotone in rank: a doc that ranks higher in any ranker scores higher.
- Robust to score-distribution differences across rankers (Qdrant cosine
  similarity in [0,1] vs Meili BM25-like raw float vs PG metadata filter
  match-or-not). RRF NEVER touches scores; it consumes ranks only.
- Deterministic: same input rank lists → byte-equal RRF output.
- O(N) time, O(N) space where N = total docs across all rankers.

**Weighting extension** (NOT in the original paper): IDX-001 adds optional
per-ranker weights `w_r ∈ [0, ∞)`:

```
RRFscore(d) = sum over r of w_r / (k + rank_r(d))
```

Defaulting all `w_r = 1.0` reduces to the original formula. Operators tune
weights via config (e.g., `qdrant=1.0, meili=0.7, pg=0.3` for a
"semantic-first" stack). REQ-IDX-007 covers the weight contract.

**Rejected alternatives**:

| Method | Rationale for rejection |
|--------|-------------------------|
| Score normalisation (z-score, min-max) | Score distributions differ wildly across stores; min-max is sensitive to outliers; z-score requires variance, which is itself unstable across small result sets |
| Borda count | Equivalent to RRF when k → ∞; RRF's k=60 specifically penalises very-deep ranks more, which we want |
| CombSUM / CombMNZ | Score-based; same fragility as direct normalisation |
| Learned-to-rank (LTR) over per-store features | Out of v0 scope; requires labelled training data (deferred to SPEC-EVAL-001 in M8 + a future SPEC-RANK-001) |

[HARD] IDX-001 selects RRF with `k=60` (paper default) and configurable
per-ranker weights for v0.

### 2.5 Comparison Summary (External Library Selection)

| Library | License | Module path | Server pin | API fit | New dep |
|---------|---------|-------------|------------|---------|---------|
| qdrant-go-client | Apache-2.0 | `github.com/qdrant/go-client` | server v1.16.3 (compose) | High | Yes |
| meilisearch-go | MIT | `github.com/meilisearch/meilisearch-go` | server v1.42.1 (compose) | High | Yes |
| pgx/v5 + pgxpool | MIT | `github.com/jackc/pgx/v5` | server v16.13 (compose) | High | Yes |
| RRF (in-package) | n/a | n/a | n/a | n/a | No (algorithm is ~30 LoC) |

Total new direct dependencies: 3. None are pre-1.0; all have major-version
stability guarantees.

---

## 3. Design Alternatives + Rejection Rationale

### 3.1 Single-Store vs Multi-Store

| Option | Description | Decision |
|--------|-------------|----------|
| A | Three independent stores (Qdrant + Meili + PG) with RRF fusion | **SELECTED** for v0.1 |
| B | Qdrant only (use Qdrant's sparse-vector support v1.7+ for keyword) | Rejected — collapses keyword precision; loses PG's relational filtering and audit; ties retrieval to a single vendor |
| C | PostgreSQL only (pg_vector + ts_vector) | Rejected — pg_vector at scale-of-team is operationally fine but at scale-of-V1-roadmap (12+ adapters, 1000 docs/sec ingest target) it consolidates indexing IO on one box; Qdrant + Meili partition the load |
| D | Meilisearch only (with hybrid embedding+keyword via Meili AI search) | Rejected — Meili's hybrid is recent and tied to OpenAI-compatible embedders; less control over the embedding pipeline (which is owned by SPEC-IDX-002) |

The three-store choice is explicit in `.moai/project/research.md:103-104`
(citing Qdrant multitenancy and Meili hybrid + multi-tenancy as anchor
docs). The roadmap row at `.moai/project/roadmap.md:55` reinforces:
"Qdrant + Meilisearch + PG Go clients, RRF fusion, ingestion pipeline."

### 3.2 RRF vs Other Fusion Methods

Already covered in §2.4 above. v0.1 uses RRF with k=60 and per-ranker
weights. Other methods (CombSUM, Borda, learned ranking) are explicitly
deferred.

### 3.3 doc_id Generation

| Option | Description | Decision |
|--------|-------------|----------|
| A | DB sequence (`SERIAL` / `BIGSERIAL` on PG `docs.id`) | Rejected — serialises ingestion through PG; defeats parallel multi-goroutine ingest |
| B | UUIDv4 random | Rejected — non-deterministic; same logical doc ingested twice gets two IDs; defeats idempotent upsert |
| C | UUIDv5 (namespace + name) over `{SourceID, URL}` | Considered; marginal benefit over option D; adds RFC 4122 formatting overhead |
| D | SHA-256 over `{SourceID, URL}`, take first 16 hex chars | **SELECTED** for v0.1 |
| E | Use `NormalizedDoc.CanonicalHash()` (content-based, includes Title+Body) | Rejected for doc_id — content-based hash means a Reddit edit (title change) gives a NEW doc_id, breaking referential integrity for citations; CanonicalHash is the `content_hash` column on PG (a separate field) |

[HARD] **doc_id formula**:

```
doc_id = hex(sha256(SourceID + "\x00" + URL))[:16]
```

- 16 hex chars = 64 bits = 1.8 × 10^19 possibilities. Birthday-collision
  probability at our V1 scale (estimate: 10^7 docs total in 12 months) is
  ~10^-7 — acceptable; revisited at scale.
- Stable across goroutines, processes, restarts: `SourceID` and `URL` are
  set by the adapter at retrieval time and never mutate within the doc's
  lifetime (per the SPEC-CORE-001 NormalizedDoc contract).
- Independent of `Title`, `Body`, `RetrievedAt`: a re-fetch of the same
  URL from the same source produces the same doc_id, enabling clean
  upsert semantics.

`content_hash` (a separate column) IS the content-based hash:

```
content_hash = NormalizedDoc.CanonicalHash() // 16 hex chars over {SourceID, URL, Title, Body}
```

The PG `docs` table places UNIQUE on `content_hash` (not on doc_id), so a
re-fetch with identical content is dropped via `INSERT … ON CONFLICT
(content_hash) DO NOTHING`. The PRIMARY KEY is `doc_id`. The two-field
discipline:

- `doc_id` PRIMARY KEY: stable across content edits (referential anchor).
- `content_hash` UNIQUE: stable across mirrors / cross-posts that re-emit
  the same content under a different URL → produces the same content_hash
  and the second INSERT no-ops.

### 3.4 Idempotent Upsert Across Three Stores

[HARD] The three stores have different idempotency semantics; IDX-001
unifies them on the `doc_id` key:

| Store | Mechanism | Idempotency unit |
|-------|-----------|-----------------|
| Qdrant | `Upsert(points)` with `point_id = doc_id` | Same `point_id` re-write replaces vector + payload atomically (Qdrant docs https://qdrant.tech/documentation/concepts/points/) |
| Meilisearch | `AddDocuments(documents, primary_key="doc_id")` | Same `doc_id` re-add replaces document; async-task-queued (client must `WaitForTask` for sync semantics in tests) |
| PostgreSQL | `INSERT ... ON CONFLICT (content_hash) DO NOTHING` | Same `content_hash` is a no-op; permits replays of the same logical doc via different URLs |

**Failure modes** (covered in REQ-IDX-005 + REQ-IDX-009):
- One store succeeds, others fail → partial state across stores. IDX-001
  records per-store error in the operation result (`UpsertResult.PerStoreErrors`),
  surfaces it to the caller, and emits an `outcome="failure"` counter
  increment on the failing store. v0.1 does NOT roll back successful
  writes (compensating actions are deferred to SPEC-IDX-005 M6 team-shared
  answer reuse, where consistency matters more). The `INSERT ... ON
  CONFLICT DO NOTHING` semantics on PG already make retries safe.

### 3.5 Per-Store Timeout and Parallel Retrieval

Mirrors the SPEC-FAN-001 pattern (§2.5 of that SPEC). Per-store ctx is:

```go
storeCtx = context.WithTimeout(parentCtx, min(perStoreTimeout, remainingParentBudget))
```

Defaults (from `.moai/config/sections/index.yaml`, NEW file in v0):

- `qdrant.timeout_ms`: 200ms (fast vector index)
- `meili.timeout_ms`: 300ms (full-text search slightly slower under load)
- `pg.timeout_ms`: 100ms (relational filter, bounded by index lookup)
- `fusion.budget_ms`: total fusion budget cap (300ms p95 target)

The parent ctx propagation is preserved: cancelling the parent cancels
every per-store ctx. A caller with a 50ms deadline against per-store 200ms
defaults sees stores time out at 50ms (whichever is smaller).

### 3.6 Error Handling and Soft-Fail

[HARD] IDX-001 follows the FAN-001 partial-result pattern. When a store
fails or times out during retrieval:

1. The store's contribution is empty (zero rank entries).
2. The error is recorded in `IndexResult.PerStoreErrors[storeName]`.
3. RRF proceeds with the remaining stores (if 2 of 3 fail, fusion is
   trivial — single ranker).
4. The call returns `(*IndexResult, nil)` — error at the call level only
   when ALL stores fail (returns `ErrAllStoresFailed`).

5. Observability: per-store outcome label is `failure` or `timeout`; one
   slog WARN record per failed store with the underlying error.

Rationale: a degraded retrieval (2 of 3 stores) is more useful than a hard
failure. The synthesis layer (SPEC-SYN-002) downgrades cite-confidence when
the index serves partial results.

### 3.7 Embedding Integration (Deferred to SPEC-IDX-002)

[HARD] IDX-001 declares an `Embedder` interface port:

```go
type Embedder interface {
    Embed(ctx context.Context, texts []string) ([][]float32, error)
    Dimensions() int  // returns 1024 for BGE-M3
}
```

IDX-001 ships a stub implementation (`zeroEmbedder`) that returns
`make([][]float32, len(texts))` with each vector pre-allocated to zero
floats of length `Dimensions()`. This permits Qdrant ingestion to proceed
(zero vectors are valid), keeping the ingestion pipeline end-to-end
testable in isolation from BGE-M3.

When SPEC-IDX-002 lands, the constructor accepts an `Embedder` argument
that is the real BGE-M3 client. No IDX-001 contract change required.

The Qdrant collection is created with `vector_size = 1024` (BGE-M3's output
dimension per its model card) and `distance = Cosine`. This is the IDX-002
schema; IDX-001 establishes it preemptively so the schema migration is a
one-time event, not a back-and-forth.

### 3.8 Ingestion Pipeline Architecture

| Option | Description | Decision |
|--------|-------------|----------|
| A | Synchronous `Upsert([]NormalizedDoc)` — caller blocks until all three stores acknowledge | **SELECTED** for v0.1 |
| B | Asynchronous via in-memory channel; background workers drain | Rejected — adds backpressure complexity; v0.1 ingest volume (12 adapters × ~25 docs/query × infrequent queries) does NOT need async |
| C | Asynchronous via Redis-backed queue (Asynq) | Rejected — adds Asynq dep that SPEC-DEP-001 reserves for /deep async (M5, SPEC-DEEP-004) |
| D | Bulk API with internal batching (auto-aggregate calls under N docs into one) | Considered; deferred to v0.2 if measured |

[HARD] v0.1 `Upsert` is synchronous. The caller passes a slice; IDX-001
fans into the three stores in parallel (errgroup), waits for all three,
returns the aggregate result. Per-store failure does not abort the others.

### 3.9 Per-Team Multi-Tenancy Reservation (Deferred to SPEC-IDX-004)

The PG `docs` schema includes a `team_id TEXT NULL` column from day one.
v0.1 inserts NULL universally (single-tenant). SPEC-IDX-004 (M6 per
`.moai/project/roadmap.md:84`) flips the column to `NOT NULL` with a default
team and adds:

- Qdrant payload-based partitioning (per Qdrant docs, §1.4 of this
  research's external citations) keyed on `team_id`.
- Meili per-tenant tokens via the SDK's tenant token API.
- PG row-level security policies keyed on `team_id`.

[HARD] v0.1 declares the column surface but does NOT enforce visibility.
This matches the `.moai/project/roadmap.md:84` scope: "SPEC-IDX-004 |
Shared index multi-tenancy — Qdrant Tiered Multitenancy config, Meili
per-tenant tokens, team visibility rules."

### 3.10 Query Parsing — NL Query → Per-Store Query

Each store consumes a different query shape:

| Store | Input | Mapping from NL Query |
|-------|-------|----------------------|
| Qdrant | `[]float32` vector + filters | `embedder.Embed([query.Text])[0]`; PG-style filters → Qdrant payload filter |
| Meilisearch | text query + filters | `query.Text` verbatim; filters via Meili filter expression |
| PostgreSQL | parameterised SQL | Filter-only path: WHERE clauses on `team_id`, `source_id`, `published_at`, `lang` |

The PG path is deliberately filter-only — it does NOT do full-text
matching. PG full-text (`tsvector`/`tsquery`) is competent but redundant
when Meilisearch already provides keyword search. PG's role is the
relational metadata anchor (audit, team scoping in M6, doc_id sequence
NOT — see §3.3).

[HARD] An `IndexQuery` value type wraps the three projections:

```go
type IndexQuery struct {
    Text      string
    Lang      string
    DocTypes  []types.DocType
    Since     time.Time
    Until     time.Time
    SourceID  string  // optional; restricts to one source
    TeamID    string  // optional; reserved for M6 visibility
    MaxResults int     // per-store cap; default 50
}
```

The `Index.Search(ctx, IndexQuery)` method does the projection internally.

---

## 4. Integration Surfaces

### 4.1 CLI / Fanout (Today)

Two integration questions remain unresolved at SPEC-IDX-001 approval time:

**Q1**: Does the CLI's `usearch query` invoke `Index.Search` BEFORE
fanout (cache-hit path) or AFTER fanout (re-rank path)?

**Q2**: Does FAN-001 internally call `Index.Upsert` after dispatch (write-
through indexing) or does a separate ingestion daemon consume FAN-001
output?

**Recommended defaults** (Open Question §6.5):

- Q1: AFTER fanout, in v0. v0.1 does NOT do cache-hit lookups (no per-team
  index → no team-scoped cache hits — that's M6's domain). Future
  SPEC-IDX-005 (M6 team-shared answer reuse) wires the cache-hit branch.
- Q2: Write-through. After FAN-001 returns, CLI calls
  `index.Upsert(result.Docs)` synchronously before synthesis. This keeps
  the indexing pipeline simple in v0.1; latency cost is bounded by NFR-IDX-001
  ingest throughput.

These are Open Questions, NOT decisions baked into SPEC-IDX-001's REQ table —
they affect callers, not the index module itself.

### 4.2 Future SPEC-RETRIEVE-001 (Hypothetical)

A possible follow-up SPEC could orchestrate "fanout-then-RRF-then-synthesis"
as a single retrieval pipeline. v0.1 declines to pre-bake that integration:
the `Index` type is a self-contained surface that can be composed in
multiple call patterns. If a SPEC-RETRIEVE-001 emerges, it can wrap
`Index.Search` without contract changes.

### 4.3 MCP / API (Future)

SPEC-MCP-001 (M7) exposes `usearch query` as an MCP tool. The MCP server
constructs one `*Index` on startup and reuses it across tool calls. The
concurrent-safe contract (REQ-IDX-012, see §1.8 above) makes this
straightforward — same pattern as FAN-001.

---

## 5. Race / Leak / Cancellation Analysis

### 5.1 Goroutine Lifetime

For one `Index.Search(ctx, query)` call:

- 1 supervisor goroutine (the caller running `Search`)
- 3 worker goroutines (one per store, gated by `errgroup` — fixed pool size 3)

Per `Index.Upsert(ctx, docs)`:

- 1 supervisor goroutine
- 3 worker goroutines (parallel write to qdrant/meili/pg)

For bulk `Upsert(docs)` where `len(docs) > BulkBatchSize` (default 100):

- 1 supervisor goroutine
- 3 worker goroutines per batch (sequential batches; `min(len/100, 1)` batches)

[HARD] All goroutines exit before `Search`/`Upsert` returns. `goleak.VerifyNone`
passes after every test.

### 5.2 Context Cancellation

Three context layers:

1. Caller's parent ctx — overall budget.
2. Errgroup-derived ctx — first-error-cancel suppressed (workers `return nil`
   even on store error to enable partial-result assembly per §3.6).
3. Per-store ctx — `context.WithTimeout(parentCtx, perStoreDeadline)`.

When the parent ctx is cancelled mid-flight:

- All in-flight per-store ctxs are also cancelled.
- Each store driver's outstanding query returns `context.Canceled` /
  `context.DeadlineExceeded` (verified in pgx, qdrant-go-client, and
  meilisearch-go through their ctx-aware methods).
- Workers complete their result-collection step quickly and exit.
- Errgroup.Wait returns. Supervisor merges partial RRF input, returns
  what's available.

### 5.3 Race Detector Workload

NFR-IDX-003 mandates `go test -race ./internal/index/...` clean under:

- 50 caller goroutines × 50 `Search` calls each (= 2,500 retrieval calls)
- 10 ingester goroutines × 50 `Upsert` calls each (= 500 ingest calls × ~10 docs/call)
- Total: 2,500 retrieval + 5,000 ingestion-doc round-trips, distributed
  across the three stores

The reference workload pattern is FAN-001's `TestDispatchConcurrent` (50 ×
100 × 5 adapters per `.moai/specs/SPEC-FAN-001/spec.md:765-783`). IDX-001's
race test sizes down per-call work (3 stores vs 5 adapters) and adds an
ingest path that FAN-001 does not have.

### 5.4 Connection Pool Discipline

Each store's Go client manages its own pool:

- **pgxpool**: bounded pool (default `max_conns = 10`); `Acquire` blocks under
  pressure. IDX-001 sets `max_conns = MaxParallel × 2 = 6` (default).
- **qdrant-go-client**: gRPC connection (a single channel handles
  multiplexed concurrent calls; no pool to size).
- **meilisearch-go**: HTTP client wrapping `http.Client.Do`; `MaxIdleConnsPerHost`
  defaults to 2 (Go stdlib). IDX-001 raises this to `MaxParallel = 3` per
  the host pattern in ADP-001 §6.5.

[HARD] **Pool-exhaustion handling**: If pgxpool returns `errors.Is(err,
context.DeadlineExceeded)` due to wait-for-acquire, IDX-001 reclassifies
this as a per-store timeout (not a transient error). REQ-IDX-009 acceptance
covers this case.

### 5.5 Leak Verification

`go.uber.org/goleak v1.3.0` is already pinned (`go.mod:30`). IDX-001 will:

- `TestMain` in `internal/index/bench_test.go` calls
  `goleak.VerifyTestMain(m)`. Pattern matches FAN-001's
  `internal/fanout/bench_test.go` (per `.moai/specs/SPEC-FAN-001/spec.md:843-849`).
- `TestSearchNoGoroutineLeakOnCancel` invokes `Search` with ctx cancelled
  mid-flight, asserts `goleak.VerifyNone(t)`.
- `TestUpsertNoGoroutineLeakOnPgFailure` injects a stub PG that errors,
  asserts no leak.

### 5.6 Resource Cleanup

The `*Index` type owns three resource handles:

- `*qdrant.Client` → `Close()` on shutdown (gRPC channel)
- `meilisearch.ServiceManager` → no close needed (HTTP-only; per the SDK)
- `*pgxpool.Pool` → `Close()` on shutdown (drains and closes connections)

[HARD] The `(*Index).Close() error` method MUST be called on process
shutdown. The CLI's `cmd/usearch/main.go` adds `defer idx.Close()` at the
construction site. Failing to close pgxpool leaks ~10 PG connections.

---

## 6. Open Questions (numbered)

These are explicitly UNRESOLVED at SPEC-approval time. They do NOT block
SPEC approval. Each has a recommended default and a one-line resolution
owner.

### 6.1 RRF constant `k`

The original paper uses `k=60`. Some implementations (Elastic, Microsoft
Azure) default to `k=60` as well; OpenSearch defaults to `k=10`. Lower k
weighs top-rank docs more aggressively.

**Recommended default**: `k=60` per the paper.

**Resolution owner**: SPEC-EVAL-001 (M8) author may override after benchmark
data on the citation-faithfulness golden set.

### 6.2 RRF per-ranker weights default

Should v0.1 ship with non-uniform weights to bias semantic-vs-keyword?

**Recommended default**: All weights = 1.0 (uniform). Operators tune via
`.moai/config/sections/index.yaml` if needed.

**Resolution owner**: SPEC-EVAL-001 (M8) author may propose tuned defaults.

### 6.3 doc_id collision threshold

64-bit doc_ids carry 1.8 × 10^19 possibilities. Birthday-collision
probability at 10^9 docs is ~5 × 10^-3 — non-trivial. v0.1's V1 scale (10^7
docs) keeps collisions under 10^-7. At scale, IDX-001 can extend to 24 hex
chars (96 bits) — backward-compat path: new ingests use longer IDs;
existing IDs continue to validate.

**Recommended default**: 16 hex chars (64 bits) for v0.1.

**Resolution owner**: future SPEC-IDX-006 author at scale.

### 6.4 Embedder dimension lock-in

Qdrant collection vector_size is fixed at creation. v0.1 sets 1024 (BGE-M3).
A future embedding model with different dimensions requires a Qdrant
collection migration (alias-swap pattern per Qdrant docs §1.4 above).

**Recommended default**: 1024 in v0.1.

**Resolution owner**: SPEC-IDX-002 (BGE-M3 wiring) author confirms the
dimension; if the model changes downstream, SPEC-IDX-002a author owns the
migration.

### 6.5 FAN-001 vs SPEC-RETRIEVE-001 integration

Does `Index.Upsert` get called from inside FAN-001's `Dispatch` or from
the CLI after `Dispatch` returns? Does `Index.Search` precede fanout
(cache-hit) or follow it (re-rank)?

**Recommended default**: Both paths are external to IDX-001 in v0.1. The
CLI orchestrates: `result := fanout.Dispatch(...)`; `index.Upsert(result.Docs)`;
`(synthesis or direct render)`. The cache-hit branch is post-V1 (SPEC-IDX-005
M6).

**Resolution owner**: SPEC-CLI-001 amendment author or new SPEC-RETRIEVE-001
author.

### 6.6 Bulk-ingest backpressure

v0.1 uses synchronous `Upsert([]NormalizedDoc)` with internal batching.
Should IDX-001 expose a streaming `IngestBatch(<-chan NormalizedDoc)` for
back-pressure-aware producers?

**Recommended default**: NO in v0.1. The synchronous API covers the M3
adapter ingestion volume. Streaming ingest is an M5 concern when /deep
queries produce thousands of intermediate docs.

**Resolution owner**: SPEC-DEEP-003 or SPEC-IDX-005 author.

### 6.7 Meilisearch `WaitForTask` in production path

Meili indexing is async. Production reads should NOT block on
`WaitForTask` (which can take seconds under load). IDX-001 intentionally
fires `AddDocuments` and returns; eventual consistency is the contract.
Tests use `WaitForTask` to synchronise assertions.

**Recommended default**: Async fire-and-forget in production; `WaitForTask`
in tests only.

**Resolution owner**: SPEC-IDX-001 author bakes this into REQ-IDX-005.

### 6.8 PG connection-pool sizing

Default `max_conns = 6` (= 2 × MaxParallel). At scale, this saturates
under heavy concurrent retrieval; symptom is pgx returning context-deadline
on Acquire.

**Recommended default**: 6 in v0.1; tuneable via `pg.max_conns` config.

**Resolution owner**: SPEC-IDX-001 author bakes this; future SPEC-IDX-005
M6 author may bump for team-scale.

### 6.9 Per-store circuit breaker

Should IDX-001 short-circuit a flapping store? FAN-001 explicitly defers
this to SPEC-EVAL-002 (M8). IDX-001 follows the same posture.

**Recommended default**: NO in v0.1. Each `Search` invokes every store
unless the store's per-call deadline is zero (already-cancelled parent ctx).

**Resolution owner**: SPEC-EVAL-002 (M8) author.

---

## 7. Sources and Citations

### External URLs (WebFetch verified 2026-05-04)

- https://github.com/qdrant/go-client — Qdrant Go client; v1.17.0 stable;
  Apache-2.0; gRPC-backed. Quoted in §2.1.
- https://github.com/meilisearch/meilisearch-go — Meili Go client; v0.36.2
  (2026-04-13); MIT; v1.x server compatibility. Quoted in §2.2.
- https://github.com/jackc/pgx — pgx PostgreSQL driver; v5 stable; MIT;
  PG 14+ support; pgxpool API. Quoted in §2.3.
- https://qdrant.tech/documentation/concepts/collections/ — Qdrant
  collection schema, distance metrics (Cosine/Dot/Euclid/Manhattan),
  named vectors, payload-based multitenancy. Quoted in §2.5 + §3.7 + §3.9.
- https://qdrant.tech/documentation/ — Qdrant overview; gRPC + REST APIs;
  Points/Collections/Payloads concepts. Quoted in §2.1.
- https://www.meilisearch.com/docs/reference/api/documents — Meili
  document API; idempotent upsert keyed on primary key. Quoted in §2.2 +
  §3.4.
- http://cormack.uwaterloo.ca/cormacksigir09-rrf.pdf — original RRF paper
  (Cormack/Clarke/Buettcher SIGIR 2009). PDF accessible via redirect from
  https://plg.uwaterloo.ca/~gvcormac/cormacksigir09-rrf.pdf; binary PDF
  not inline-quotable but the algorithm is canonical and described in
  §2.4.

### Internal Files (file:line cited)

- `internal/index/index.go:1-3` — current 3-line stub.
- `pkg/types/normalized_doc.go:40-56` — NormalizedDoc 15-field struct.
- `pkg/types/normalized_doc.go:63-77` — Validate (required fields).
- `pkg/types/normalized_doc.go:91-106` — CanonicalHash (content-only).
- `internal/obs/metrics/metrics.go:33-65` — observability collector
  declarations.
- `internal/obs/metrics/metrics.go:89-95` — FanoutInflight precedent.
- `internal/obs/metrics/metrics.go:171-176` — cardinality allowlist.
- `deploy/docker-compose.yml:31-46` — Qdrant compose service.
- `deploy/docker-compose.yml:48-66` — Meilisearch compose service.
- `deploy/docker-compose.yml:68-87` — PostgreSQL compose service.
- `go.mod:3` — Go 1.25.8 baseline.
- `go.mod:30` — `go.uber.org/goleak v1.3.0` indirect.
- `go.mod:33` — `golang.org/x/sync v0.20.0` indirect.
- `.moai/project/structure.md:35-38` — `internal/index/{qdrant,meilisearch,
  postgres}` reservation.
- `.moai/project/structure.md:160` — `pkg/types` SDK boundary clause
  (justifies leaving pkg/types untouched in §1.3).
- `.moai/project/research.md:103-104` — Qdrant multitenancy + Meili hybrid
  anchor docs.
- `.moai/project/roadmap.md:55` — M3 row "SPEC-IDX-001 | Hybrid index layer
  | Qdrant + Meilisearch + PG Go clients, RRF fusion, ingestion pipeline".
- `.moai/project/roadmap.md:84` — SPEC-IDX-004 multi-tenancy gate (M6).
- `.moai/project/roadmap.md:117-128` — M3 parallelization plan; IDX-001
  gated on FAN-001's spec.md approval (which has happened — FAN-001 status
  is `approved` per `.moai/specs/SPEC-FAN-001/spec.md:6`).
- `.moai/project/roadmap.md:150` — M3 exit criterion.
- `.moai/specs/SPEC-CORE-001/spec.md:139` — REQ-CORE-001 NormalizedDoc
  contract.
- `.moai/specs/SPEC-CORE-001/spec.md:141-142` — REQ-CORE-003/004 registry
  + wrappedAdapter sole-emitter pattern.
- `.moai/specs/SPEC-FAN-001/spec.md:296` — REQ-FAN-001 Fanout/Dispatch
  contract; IDX-001 ingestion path consumes `fanout.Result.Docs`.
- `.moai/specs/SPEC-FAN-001/spec.md:765-783` — TestDispatchConcurrent
  reference workload; NFR-IDX-003 follows the pattern.
- `.moai/specs/SPEC-FAN-001/spec.md:843-849` — `goleak.VerifyTestMain`
  reference pattern; NFR-IDX-004 reuses.
- `.moai/specs/SPEC-OBS-001/spec.md:88-93` — REQ-OBS-001..006 baseline
  collectors.
- `.moai/specs/SPEC-OBS-001/spec.md:101` — NFR-OBS-002 cardinality safety;
  IDX-001 extends the allowlist with `store` and `op`.
- `.moai/specs/SPEC-LLM-001/spec.md:548-572` — `internal/obs/metrics/llm.go`
  pattern; IDX-001 mirrors with `internal/obs/metrics/index.go`.
- `.moai/config/sections/quality.yaml` — `development_mode: tdd`,
  `test_coverage_target: 85`.
- `.moai/config/sections/language.yaml` — `documentation: en`,
  `code_comments: en`.

---

End of Research Document.

**Summary for SPEC Author**: SPEC-IDX-001 implements a three-store hybrid
retrieval layer at `internal/index/` consuming `[]NormalizedDoc` from
SPEC-CORE-001 and serving as the post-fanout indexing/retrieval target for
SPEC-FAN-001 output. The library choices are `github.com/qdrant/go-client
v1.17.0` (gRPC), `github.com/meilisearch/meilisearch-go v0.36.2`, and
`github.com/jackc/pgx/v5` + `pgxpool`. doc_id is deterministic
(`hex(sha256(SourceID|URL))[:16]`) so ingestion is parallel-safe; PG's
`content_hash` UNIQUE column enables idempotent INSERT-ON-CONFLICT
semantics across replays. Retrieval issues parallel per-store fetches with
per-store timeout (errgroup with first-error suppression mirroring
SPEC-FAN-001 §2.5/H18); the rank lists fuse via Reciprocal Rank Fusion
(Cormack/Clarke/Buettcher SIGIR 2009, k=60, configurable per-store
weights). Soft-fail under per-store failure (partial result; `nil` error at
the call level unless ALL three stores fail). Embedder is a port; v0.1
ships a zero-vector stub; SPEC-IDX-002 wires the BGE-M3 client. PG schema
includes a `team_id` column reserved as NULL in v0; SPEC-IDX-004 (M6)
enforces visibility. ONE new metric family group (`usearch_index_ops_total
{store, op, outcome}` + duration histograms); allowlist extended with
`store` and `op`. Race-safety is verified by `TestSearchUpsertConcurrent`
(50 retrieval × 10 ingest goroutines, race-clean). Goroutine-leak via
`goleak.VerifyTestMain` + per-test mid-cancel checks. The integration
question — whether FAN-001 internally calls `Index.Upsert` or the CLI
orchestrates — is deferred to OQ §6.5 with a "CLI orchestrates"
recommendation. Three new Go module dependencies (qdrant-go-client v1.17.0,
meilisearch-go v0.36.2, jackc/pgx/v5). SPEC body targets ~700-900 lines
covering 14 EARS REQs (12 P0 + 2 P1) + 5 NFRs.
