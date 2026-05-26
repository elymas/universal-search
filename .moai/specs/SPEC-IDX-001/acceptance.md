# SPEC-IDX-001 Acceptance Scenarios

Generated: 2026-05-26 (reverse-engineered from implemented code)
Format: Given / When / Then with file:line references to verifying tests.
Status: SPEC implemented (2026-05-08); scenarios reflect realised behaviour.

This document enumerates the testable acceptance criteria for SPEC-IDX-001,
mapped 1:1 to the EARS requirements in `spec.md` §3 and the NFRs in §4.
Each scenario is implemented as one or more Go tests under
`internal/index/`, `internal/index/qdrant/`, `internal/index/meili/`, or
`internal/index/pg/`.

---

## AC-001 — Index construction validates Embedder

**Coverage**: REQ-IDX-001 (Ubiquitous)

### Given

- A caller imports `internal/index` and calls `index.New(ctx, opts)`
  with `opts.Embedder = nil`.

### When

`index.New(ctx, opts)` is invoked.

### Then

- Returns `(nil, ErrEmbedderRequired)`.
- No store sub-clients are constructed (no goroutines leak, no network
  calls issued).
- `errors.Is(err, ErrEmbedderRequired)` is `true`.

**Verifying test**: `TestNewRequiresEmbedder` in
`internal/index/index_test.go`.

---

## AC-002 — Options defaults populated for zero-value caller

**Coverage**: REQ-IDX-001 (Ubiquitous)

### Given

- A caller passes `Options{Embedder: zeroEmbedder{}}` with every other
  field left at the zero value (`MaxParallel=0`, empty `PerStoreTimeout`
  map, etc.).

### When

`index.New(ctx, opts)` is invoked and the returned `*Index.opts` is
inspected.

### Then

- `opts.MaxParallel == 3` (default).
- `opts.PerStoreTimeout["qdrant"] == 200 * time.Millisecond`.
- `opts.PerStoreTimeout["meili"] == 300 * time.Millisecond`.
- `opts.PerStoreTimeout["pg"] == 100 * time.Millisecond`.
- `opts.RRFConstantK == 60`.
- `opts.RRFWeights["qdrant" | "meili" | "pg"] == 1.0`.
- `opts.BulkBatchSize == 100`.

**Verifying test**: `TestNewNormalisesDefaults` in
`internal/index/options_test.go` and `index_test.go`.

---

## AC-003 — Schema bootstrap failure wraps with sentinel

**Coverage**: REQ-IDX-001, REQ-IDX-008 (Ubiquitous)

### Given

- Options with `AutoEnsureSchema = true` and a PG client configured to
  reject `EnsureSchema` (e.g., wrong DSN or simulated migration error).

### When

`index.New(ctx, opts)` is invoked.

### Then

- Returns `(nil, err)` where `errors.Is(err, ErrSchemaBootstrapFailed)`.
- The underlying store error is included via `%w` wrapping.
- `idx.Close()` is invoked internally to release the already-constructed
  Qdrant and Meili clients (no leak).

**Verifying test**: `TestNewSchemaBootstrapFailureWraps` in
`internal/index/index_test.go`.

---

## AC-004 — Close orderly shuts down all three sub-clients

**Coverage**: REQ-IDX-001 (Ubiquitous)

### Given

- A constructed `*Index` instance.

### When

`idx.Close()` is invoked.

### Then

- `pg.Client.Close()` is invoked (void return).
- `qdrant.Client.Close()` is invoked; its error (if any) is captured as
  the first non-nil error.
- `meili.Client.Close()` is invoked (no-op for HTTP, returns nil).
- All three closes are attempted regardless of intermediate errors.
- `idx.Close()` returns the first non-nil error or `nil` if all succeed.

**Verifying test**: `TestCloseClosesAllStores` in
`internal/index/index_test.go`.

---

## AC-005 — Search returns ErrAllStoresFailed only when all three fail with empty rank lists

**Coverage**: REQ-IDX-001, REQ-IDX-006, REQ-IDX-009 (Event-Driven)

### Given

- Three sub-store stubs configured to return errors and zero-length
  rank lists on `Search`.

### When

`idx.Search(ctx, IndexQuery{Text: "anything"})` is invoked.

### Then

- Embedder produces a 1024-dim vector once for the query text.
- All three `parallelSearch` goroutines complete with non-nil errors and
  empty rank lists.
- The orchestrator returns `(nil, ErrAllStoresFailed)`.

### Boundary: Partial Failure

If 1 or 2 of the three stores fail but at least one returns a non-empty
rank list:

- Returns `(*IndexResult, nil)`.
- `result.PerStoreErrors[failedStore] != nil` (only failed stores
  recorded).
- `result.Docs` is the RRF-fused output of the surviving rank lists.
- `result.Stats.PerStoreCounts[failedStore] == 0`.

**Verifying tests**: `TestSearchAlwaysReturnsResultOrAllStoresFailed`
(`index_test.go`), `TestSearchPerStoreTimeoutDoesNotKillOthers`
(`dispatch_test.go`), `TestSearchAllStoresFailReturnsErr`
(`dispatch_test.go`).

---

## AC-006 — Qdrant EnsureCollection is idempotent + payload round-trip

**Coverage**: REQ-IDX-002 (State-Driven)

### Given

- A fresh testcontainers Qdrant instance.
- `qdrant.NewClient(cfg)` returns a `*Client`.

### When

1. `client.EnsureCollection(ctx, "usearch_docs", 1024)` is invoked twice.
2. `client.Upsert(ctx, []Point{P1})` is invoked with `P1` carrying a
   synthetic 1024-dim vector and a payload with `source_id`, `url`,
   `title`, `lang`, `doc_type`, `published_at`, `retrieved_at`,
   `team_id`, `content_hash` keys.
3. `client.Search(ctx, vector, nil, 10)` is invoked.

### Then

- First `EnsureCollection` creates the collection with
  `vector_size=1024, distance=Cosine, on_disk_payload=true`.
- Second `EnsureCollection` returns `nil` (no-op, no schema mutation).
- `Search` returns the upserted point with payload preserved.

**Verifying tests**: `TestQdrantEnsureCollectionIdempotent`,
`TestQdrantUpsertRoundTrip`, `TestQdrantPayloadFiltering` in
`internal/index/qdrant/client_test.go`.

---

## AC-007 — Meilisearch EnsureIndex applies documented settings

**Coverage**: REQ-IDX-003 (Optional)

### Given

- A fresh Meilisearch instance accessible via HTTP.

### When

1. `meili.NewClient(cfg).EnsureIndex(ctx, "usearch_docs", IndexSettings{
   SearchableAttributes: ["title","body","snippet"],
   FilterableAttributes: ["source_id","lang","doc_type","team_id","published_at"],
   DistinctAttribute: "doc_id"})` is invoked.
2. The settings are re-fetched via `index.GetSettings()`.
3. `AddDocuments(ctx, "usearch_docs", []Document{D1})` is invoked (async
   fire-and-forget); test calls `WaitForTask(taskUID)` then `Search`.

### Then

- Index is created with the documented settings exactly.
- A second `EnsureIndex` call is a no-op.
- `AddDocuments` returns `(*TaskInfo, nil)` without blocking.
- `Search(ctx, "alice", ...)` (after `WaitForTask`) returns documents
  matching the query text.

**Verifying tests**: `TestMeiliEnsureIndexCreatesWithSettings`,
`TestMeiliEnsureIndexIdempotent`, `TestMeiliAddDocumentsAsync`,
`TestMeiliSearchTextMatch` in
`internal/index/meili/client_test.go`.

---

## AC-008 — Upsert validation rejects invalid docs, processes the rest

**Coverage**: REQ-IDX-004 (Unwanted)

### Given

- A batch of 5 `NormalizedDoc` values: 3 valid + 2 invalid (one missing
  `URL`, one with zero `RetrievedAt`).

### When

`idx.Upsert(ctx, docs)` is invoked.

### Then

- `result.Stats.DocCount == 5` (all docs entered the pipeline).
- `result.Stats.SkippedCount == 2` (validation rejections).
- `result.PerStoreErrors["validation"] != nil` and references the count
  of validation errors (`"2 validation error(s): first=…"`).
- ZERO per-store writes for the 2 invalid docs in any store.
- The 3 valid docs are visible via a subsequent `Search` (after Meili
  `WaitForTask`).
- Exactly ONE WARN slog record per batch (not per doc) with attributes
  `{batch_size: 5, skipped_count: 2}`.

**Verifying test**: `TestUpsertRejectsInvalidDocs` in
`internal/index/index_test.go`.

---

## AC-009 — Upsert empty batch returns zero-stats result

**Coverage**: REQ-IDX-004 (Unwanted boundary)

### Given

- An empty doc slice.

### When

`idx.Upsert(ctx, []types.NormalizedDoc{})` is invoked.

### Then

- Returns `(*UpsertResult, nil)`.
- `result.Stats.DocCount == 0`.
- `result.Stats.SkippedCount == 0`.
- `result.PerStoreErrors` is empty (length 0 map).
- No slog records emitted.

**Verifying test**: `TestUpsertEmptyBatchReturnsZeroes` in
`internal/index/index_test.go`.

---

## AC-010 — Upsert idempotent on replay (ON CONFLICT DO UPDATE)

**Coverage**: REQ-IDX-005 (Event-Driven)

### Given

- 3 valid docs ingested once via `Upsert`.

### When

The same 3 docs are ingested a second time (identical SourceID + URL +
content).

### Then

- First call: PG affected-rows = 3.
- Second call: `result.Stats.DocCount == 3` (pipeline entered) but PG
  affected-rows = 0 (ON CONFLICT no-op since `content_hash` is
  unchanged).
- Qdrant overwrite is silent (same point_id).
- Meili `AddDocuments` upserts by primaryKey (silent overwrite).
- Subsequent `Search` returns the 3 docs once each (no duplication).

### Boundary: Edited content

If a doc with the same SourceID + URL but EDITED Title/Body is
re-ingested:

- `doc_id` is unchanged.
- `content_hash` changes.
- PG `INSERT … ON CONFLICT (doc_id) DO UPDATE` UPDATES the row's
  `title`, `body`, `content_hash`, `retrieved_at`.
- Qdrant point overwritten with the new payload.
- Meili document overwritten.
- `Search` returns the doc with the EDITED Title.

**Verifying tests**: `TestUpsertHappyPath3Docs`,
`TestUpsertIdempotentOnReplay`, `TestUpsertEditedTitleUpdatesAllStores`
in `internal/index/index_test.go`.

---

## AC-011 — Per-store timeout cancels its goroutine without killing siblings

**Coverage**: REQ-IDX-006, REQ-IDX-009 (soft-fail)

### Given

- Options with `PerStoreTimeout["meili"] = 1 * time.Millisecond`
  (forced timeout); `qdrant` and `pg` have generous timeouts.
- Stores rigged so Qdrant + PG succeed and Meili sleeps longer than 1ms.

### When

`idx.Search(ctx, IndexQuery{Text: "x"})` is invoked.

### Then

- `result.PerStoreErrors["meili"]` is non-nil and wraps
  `context.DeadlineExceeded`.
- `result.PerStoreErrors["qdrant"]` and `["pg"]` are absent (only
  non-nil errors are retained in the returned map).
- `result.Docs` contains the RRF-fused output of qdrant + pg rank lists.
- `result.Stats.PerStoreCounts["meili"] == 0`.

**Verifying test**: `TestSearchPerStoreTimeoutDoesNotKillOthers` in
`internal/index/dispatch_test.go`.

---

## AC-012 — Soft-fail discipline for Upsert (per-store errors do not abort)

**Coverage**: REQ-IDX-009 (Upsert path)

### Given

- A batch of 3 valid docs.
- Qdrant client rigged to fail; Meili and PG succeed.

### When

`idx.Upsert(ctx, docs)` is invoked.

### Then

- Returns `(*UpsertResult, nil)` — Upsert NEVER returns an error.
- `result.PerStoreErrors["qdrant"] != nil`.
- `result.PerStoreErrors["meili"]` and `["pg"]` are absent.
- `result.Stats.SuccessCount == 2`.
- `result.Stats.ErrorCount == 1`.
- Subsequent `Search` returns docs from Meili + PG even though Qdrant
  has stale state.

**Verifying tests**: `TestUpsertOneStoreFailsOthersSucceed`,
`TestUpsertAllStoresFailReturnsResultWithThreeErrors` in
`internal/index/dispatch_test.go`.

---

## AC-013 — RRF formula matches Cormack/Clarke/Buettcher SIGIR 2009

**Coverage**: REQ-IDX-007 (Event-Driven)

### Given

- Two rank lists:
  - `qdrant`: `[d1, d2, d3]` (ranks 1, 2, 3).
  - `meili`: `[d1, d4, d5]` (ranks 1, 2, 3).
- Default `k = 60`, weights all `1.0`.

### When

`fuseRRF(rankLists, weights, 60)` is invoked.

### Then

- `score(d1) = 1/61 + 1/61 = 2/61 ≈ 0.03279`.
- `score(d2) = 1/62 ≈ 0.01613`.
- `score(d3) = 1/63 ≈ 0.01587`.
- `score(d4) = 1/62 ≈ 0.01613`.
- `score(d5) = 1/63 ≈ 0.01587`.
- Output order: `[d1, d2 (tie d4), d3 (tie d5)]`. Tie-break is `DocID`
  ascending; deterministic via `sort.SliceStable`.

### Boundary: Same input → byte-equal output

Running `fuseRRF` twice on the same input produces byte-equal output.

**Verifying tests**: `TestRRFSingleStoreReturnsRankOrder`,
`TestRRFTwoStoresAdditive`, `TestRRFWeightedFavoursHigherWeight`,
`TestRRFTieBreakDocIDAscending`, `TestRRFDeterministic`,
`TestRRFKCanonical60` in `internal/index/rrf_test.go`.

---

## AC-014 — PG migration 0001 creates the documented schema

**Coverage**: REQ-IDX-008 (Ubiquitous)

### Given

- A fresh testcontainers PG instance with no schema applied.

### When

`pg.Client.EnsureSchema(ctx)` is invoked.

### Then

- The `docs` table is created with all 15 columns matching the SPEC §3
  REQ-IDX-008 type list (verified via `information_schema.columns`).
- The following indexes exist: UNIQUE on `content_hash`, B-tree on
  `source_id`, `published_at`, `team_id`.
- A second `EnsureSchema` call is a no-op (no error).
- If a column is manually dropped, the next `EnsureSchema` returns
  `errors.Is(err, ErrSchemaBootstrapFailed)`.

**Verifying tests**: `TestPGEnsureSchemaCreatesAllColumns`,
`TestPGEnsureSchemaIdempotent`, `TestPGEnsureSchemaIndexesExist` in
`internal/index/pg/client_test.go`.

---

## AC-015 — TeamID filter propagates to all three stores (v0.1 reservation)

**Coverage**: REQ-IDX-010 (Optional)

### Given

- 5 docs inserted via `Upsert`; v0.1 sets `team_id = NULL` on every
  doc (universal single-tenant).

### When

1. `Search(ctx, IndexQuery{TeamID: ""})` is invoked.
2. `Search(ctx, IndexQuery{TeamID: "team-A"})` is invoked.

### Then

- Case 1: All 5 docs returned (empty TeamID matches NULL rows).
- Case 2: Empty result (non-empty TeamID excludes NULL rows in v0.1).
- Qdrant payload includes `team_id` key with NULL value (verified via
  `payload.team_id` field).
- Meili `index.GetSettings().FilterableAttributes` includes `team_id`.
- PG `SELECT team_id FROM docs WHERE doc_id = ANY($1)` returns NULL for
  every doc.

**Verifying tests**: `TestSearchEmptyTeamIDMatchesAll`,
`TestSearchTeamIDFilterMatchesNoneInV0`,
`TestUpsertSetsTeamIDNullInV0`, `TestQdrantPayloadIncludesTeamIDField`,
`TestMeiliFilterableAttributesIncludesTeamID` in
`internal/index/index_test.go` and the sub-package test files.

(Note: SPEC-IDX-004 subsequently flipped this behaviour to enforce
team_id; the v0.1 behaviour documented here is the surface IDX-004
builds on.)

---

## AC-016 — Per-call observability emits OTel span + Prometheus + slog

**Coverage**: REQ-IDX-011 (Ubiquitous)

### Given

- An `*Index` constructed with a non-nil `*obs.Obs` carrying an
  in-memory OTel exporter and a Prometheus registry snapshot.

### When

`idx.Search(ctx, IndexQuery{Text: "x"})` is invoked once.

### Then

- Exactly ONE OTel parent span named `index.search` with attributes
  `{index.op, index.fused_count, index.errors_count,
  index.elapsed_seconds}`.
- Per-store child spans propagate via ctx (verified via SpanContext
  parent link).
- 3 `IndexOps.WithLabelValues(store, "search", outcome).Inc()` calls
  (one per store; outcome derived from per-store error).
- 3 `IndexOpDuration.WithLabelValues(store, "search").Observe(...)` calls.
- 1 `IndexFusionDuration.Observe(...)` call.
- 1 slog record at INFO with `{request_id, op:"search", fused_count,
  store_counts, errors_count, elapsed_seconds}`.

### Boundary: Nil Obs

If `*Index` is constructed with `Obs: nil`:

- `Search` and `Upsert` do not panic.
- Return valid `*IndexResult` / `*UpsertResult`.
- No emission attempted.

**Verifying tests**: `TestEmitParentSpanWithAttributes`,
`TestEmitChildStoreSpansAreChildren`, `TestEmitIndexOpsCounterPerStore`,
`TestEmitFusionDurationHistogramPerCall`, `TestEmitSlogIncludesRequestID`,
`TestEmitSafeOnNilObs`, `TestNoNewMetricFamilies` in
`internal/index/observability_test.go`.

---

## AC-017 — docID is deterministic + 16 hex chars + NUL-separator collision-safe

**Coverage**: REQ-IDX-014 (Ubiquitous)

### Given

- `docID("reddit", "https://example.com/a")` is computed twice.

### When

Both calls are inspected.

### Then

- Same input → byte-equal output.
- Output is exactly 16 hex chars (64 bits).
- Varying SourceID OR URL produces a distinct doc_id.
- `docID("redd", "ithttps://x") != docID("reddit", "https://x")` —
  the NUL separator (`\x00`) prevents the prefix-collision attack.

**Verifying tests**: `TestDocIDDeterministic`,
`TestDocIDInputSensitive`, `TestDocIDLength16`,
`TestDocIDNullSeparatorPreventsCollision` in
`internal/index/docid_test.go`.

---

## AC-018 — Embedder port is satisfied by zeroEmbedder stub

**Coverage**: REQ-IDX-014 (port contract)

### Given

- `zeroEmbedder{}` is the v0.1 default Embedder implementation.

### When

1. Compile-time check `var _ Embedder = zeroEmbedder{}` is included in
   `embedder.go`.
2. `zeroEmbedder{}.Dimensions()` is called.
3. `zeroEmbedder{}.Embed(ctx, []string{"a", "b", "c"})` is called.

### Then

- The compile-time check succeeds (the interface contract is met).
- `Dimensions() == 1024`.
- `Embed` returns a `[][]float32` of length 3, each inner vector of
  length 1024, all elements `0.0`.

**Verifying tests**: `TestZeroEmbedderDimensions1024`,
`TestZeroEmbedderEmbedReturnsZeros`,
`TestEmbedderInterfaceImplementedByZeroEmbedder` in
`internal/index/embedder_test.go`.

---

## AC-019 — Bulk batch discipline splits large input into BulkBatchSize chunks

**Coverage**: REQ-IDX-013 (Event-Driven)

### Given

- 250 valid docs with `BulkBatchSize = 100`.

### When

`idx.Upsert(ctx, docs)` is invoked.

### Then

- 3 sequential batches dispatched: [100, 100, 50].
- `result.Stats.DocCount == 250`.
- All 250 docs visible via subsequent `Search` (after Meili WaitForTask).
- `result.Stats.PerStoreLatencies["qdrant"]` is the SUM of per-batch
  latencies, not single-call latency.

**Verifying test**: `TestUpsertBatchesLargeInput` in
`internal/index/index_test.go`.

---

## Edge Cases

### EC-001 — Concurrent Search + Upsert races

**Coverage**: REQ-IDX-012 (State-Driven), NFR-IDX-003

#### Given

- One `*Index` instance against testcontainers-backed stores.
- 50 caller goroutines each issuing 50 `Search` calls (= 2,500
  retrievals) plus 10 ingester goroutines each issuing 50 `Upsert`
  calls × 10 docs each (= 500 calls × 10 docs = 5,000 docs).

#### When

`go test -race ./internal/index/...` runs the concurrent workload.

#### Then

- Zero race-detector alarms attributable to the `internal/index`
  package.
- Every successful `*IndexResult` has `Stats.PerStoreCounts` populated
  for 3 stores (some counts may be 0 on partial failure).
- `goleak.VerifyNone(t)` at test end (with the documented pgxpool
  ignore-function).
- `IndexOps` counter values are monotonically non-decreasing.

**Verifying test**: `TestSearchUpsertConcurrent` (executed under
`go test -race`).

### EC-002 — Cancelled parent ctx mid-flight

**Coverage**: REQ-IDX-006 (boundary)

#### Given

- A parent ctx with a near-zero deadline (e.g., 1 ns).

#### When

`idx.Search(ctx, IndexQuery{Text: "x"})` is invoked.

#### Then

- `deriveStoreCtx(parent, store)` short-circuits: since
  `remaining < perStoreTimeout` and may be `<= 0`, the function returns
  an already-cancelled ctx (pre-cancel branch).
- All per-store goroutines observe `context.DeadlineExceeded` immediately.
- `result.PerStoreErrors` has 3 entries; rank lists are all empty.
- Returns `(nil, ErrAllStoresFailed)`.

**Verifying test**: covered by `TestSearchAllStoresFailReturnsErr` with
synthetic timeout in `dispatch_test.go`.

---

## Coverage Matrix

| Scenario | REQ-001 | 002 | 003 | 004 | 005 | 006 | 007 | 008 | 009 | 010 | 011 | 012 | 013 | 014 |
|----------|---------|-----|-----|-----|-----|-----|-----|-----|-----|-----|-----|-----|-----|-----|
| AC-001 | ✓ | | | | | | | | | | | | | |
| AC-002 | ✓ | | | | | | | | | | | | | |
| AC-003 | ✓ | | | | | | | ✓ | | | | | | |
| AC-004 | ✓ | | | | | | | | | | | | | |
| AC-005 | ✓ | | | | | ✓ | | | ✓ | | | | | |
| AC-006 | | ✓ | | | | | | | | | | | | |
| AC-007 | | | ✓ | | | | | | | | | | | |
| AC-008 | | | | ✓ | | | | | | | | | | |
| AC-009 | | | | ✓ | | | | | | | | | | |
| AC-010 | | | | | ✓ | | | | | | | | | |
| AC-011 | | | | | | ✓ | | | ✓ | | | | | |
| AC-012 | | | | | | | | | ✓ | | | | | |
| AC-013 | | | | | | | ✓ | | | | | | | |
| AC-014 | | | | | | | | ✓ | | | | | | |
| AC-015 | | | | | | | | | | ✓ | | | | |
| AC-016 | | | | | | | | | | | ✓ | | | |
| AC-017 | | | | | | | | | | | | | | ✓ |
| AC-018 | | | | | | | | | | | | | | ✓ |
| AC-019 | | | | | | | | | | | | | ✓ | |
| EC-001 | | | | | | | | | | | | ✓ | | |
| EC-002 | | | | | | ✓ | | | | | | | | |

NFR coverage:

- NFR-IDX-001 (ingest throughput): exercised by integration benchmarks
  (not part of standard CI).
- NFR-IDX-002 (retrieval latency): per-store latencies in
  `result.Stats.StoreLatencies` provide telemetry; production budgets
  enforced via NFR-IDX-006 in IDX-004.
- NFR-IDX-003 (race-clean): EC-001 plus `go test -race`.
- NFR-IDX-004 (zero goleak): integration suite with pgxpool ignore.
- NFR-IDX-005 (alloc/op): bench tests behind `//go:build integration`.

---

## Definition of Done (Verified at 2026-05-08)

- [x] 24 source/test files under `internal/index/` and sub-packages
      created.
- [x] 14 EARS REQs (REQ-IDX-001 through REQ-IDX-014) covered by tests.
- [x] 5 NFRs verified (race-clean + goleak via integration; throughput
      + latency + alloc/op behind `//go:build integration`).
- [x] `go vet ./internal/index/...` returns 0 issues.
- [x] `golangci-lint run ./internal/index/...` returns 0 issues.
- [x] `go test -race ./internal/index/...` PASS.
- [x] Build success on full project.
- [x] Prometheus collectors registered: `IndexOps`, `IndexOpDuration`,
      `IndexFusionDuration` via `internal/obs/metrics/index.go`.
- [x] Cardinality allowlist extended with `store` (4 enum values) and
      `op` (3 enum values).
- [x] MX tags applied: 3 ANCHOR (`New`, `Search`, `Upsert`), 2 WARN
      (`parallelSearch`, `parallelUpsert`), NOTE on per-store ctx
      derivation magic constants.
- [x] No regression in 14 dependent packages.

---

*End of SPEC-IDX-001 acceptance.md (post-hoc).*
