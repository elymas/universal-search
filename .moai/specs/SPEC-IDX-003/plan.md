# SPEC-IDX-003 Implementation Plan (Post-Hoc)

Generated: 2026-05-26 (reverse-engineered from implemented code)
Methodology: TDD (RED-GREEN-REFACTOR) — completed 2026-05-08
Coverage target: 85%
Harness: standard
Status: implemented (verified against `services/tokenizer-ko/` +
`internal/index/tokenizer/` + `internal/index/router/` +
`internal/index/meili/korean_shard.go`)

---

## 1. Overview

This plan.md is a post-hoc summary of SPEC-IDX-003 (Korean tokenization)
that already shipped. The original RED-GREEN-REFACTOR cycle has
completed; this document reconstructs the milestone breakdown so
SPEC-IDX-003 has the canonical 3-file SPEC layout.

SPEC-IDX-003 delivers three coordinated components for the Korean-first
M3 exit criterion (`Korean query returns Naver results ranked first`):

1. **Python sidecar** at `services/tokenizer-ko/` — FastAPI app wrapping
   `pymecab-ko` (mecab-ko + mecab-ko-dic bundled) on port 8083. Exposes
   `POST /tokenize` and `GET /health`. Single-doc, synchronous; uses
   `asyncio.Lock` to serialize Tagger access (pymecab-ko is thread-
   unsafe per research §5.1).

2. **Meilisearch dual-shard configuration** — `usearch_docs` (default,
   from SPEC-IDX-001) plus `usearch_docs_ko` (Korean shard) configured
   with 11 Korean particle stop-words (reused from
   `internal/router/korean.go` SPEC-IR-001 list) and pre-tokenized text
   from the sidecar.

3. **Go-side index-time + query-time routing** — `internal/index/router/`
   with `IndexShardForDoc(doc) []Shard` (index-time) and
   `QueryShardsForText(text) []Shard` (query-time). RRF merge function
   at `merge.go::MergeRRF` (k=60) for ambiguous-band queries that hit
   both shards. HTTP client at `internal/index/tokenizer/client.go`
   communicates with the sidecar. Meili-side shard settings managed via
   `internal/index/meili/korean_shard.go::EnsureKoreanIndexSettings`.

The implementation reuses SPEC-IR-001's `router.HangulRatio`,
`router.RatioHigh = 0.30`, and `router.RatioLow = 0.10` without
duplication.

---

## 2. Phase Breakdown (Post-Hoc Reconstruction)

### Phase A — Python Sidecar Foundation

Files (implemented):

- `services/tokenizer-ko/pyproject.toml` — `fastapi>=0.115`,
  `uvicorn[standard]>=0.30`, `pydantic>=2.9`, `mecab-ko>=1.0,<2.0`,
  Python `>=3.11`.
- `services/tokenizer-ko/src/tokenizer_ko/__init__.py` (9 LOC) —
  Package doc + `__version__`.
- `services/tokenizer-ko/src/tokenizer_ko/models.py` (43 LOC) —
  `TokenizeRequest{request_id, text}`, `TokenizeResponse{request_id,
  tokens, joined, morpheme_count, latency_ms, dict_version}` Pydantic
  v2 models with `ConfigDict(extra="forbid",
  str_strip_whitespace=True)`.
- `services/tokenizer-ko/src/tokenizer_ko/tokenize.py` (125 LOC) —
  `async def tokenize(text, tagger, lock)`. Acquires `asyncio.Lock`,
  calls `tagger.parse(text)`, skips `EOS` and empty lines, extracts
  surface forms (column 0), returns the morpheme list + space-joined
  string.
- `services/tokenizer-ko/src/tokenizer_ko/obs.py` (118 LOC) —
  JSON-formatted `logging` setup; `Timer` context manager;
  `log_tokenize(record)` helper writing the documented attribute set.
- `services/tokenizer-ko/src/tokenizer_ko/app.py` (181 LOC) — FastAPI
  app with `lifespan` constructing the `pymecab_ko.Tagger` once at
  startup (raises if dict load fails → container HEALTHCHECK fails →
  docker-compose restart). Routes `POST /tokenize` and `GET /health`.
- `services/tokenizer-ko/src/tokenizer_ko/__main__.py` (29 LOC) —
  Uvicorn entrypoint binding port `TOKENIZER_KO_PORT` (default 8083).
- `services/tokenizer-ko/Dockerfile` — Multi-stage on `python:3.11-slim`,
  non-root user, HEALTHCHECK `curl -f http://localhost:8083/health`.
- `deploy/docker-compose.yml` — New `tokenizer-ko` service on port
  8083, joins `app` network, `start_period: 20s` to allow dict load.

REQ coverage: REQ-IDX-003-001 (endpoint contract), REQ-IDX-003-002
(mecab-ko tokenization), REQ-IDX-003-003 (lifespan failure on dict),
REQ-IDX-003-004 (empty/oversize input rejection), REQ-IDX-003-009a
(Python observability).

### Phase B — Go-Side Tokenizer HTTP Client

Files (implemented):

- `internal/index/tokenizer/types.go` (38 LOC) — `Request`, `Result`,
  error sentinels (`ErrInvalidInput`, `ErrSidecarUnreachable`,
  `ErrTimeout`).
- `internal/index/tokenizer/config.go` (54 LOC) — `Config` struct +
  `ConfigFromEnv()` reading `TOKENIZER_KO_BASE_URL` (default
  `http://tokenizer-ko:8083`) and `TOKENIZER_KO_REQUEST_TIMEOUT_SECONDS`
  (default 1).
- `internal/index/tokenizer/client.go` (105 LOC) — `Client` struct with
  `New(cfg, *obs.Obs)`; `Tokenize(ctx, text string) (Result, error)`
  method. Retry policy: 2 retries on connection-level errors, 100ms +
  300ms ± 10% jitter. Per-call observability emit (counter + histogram
  + OTel span).
- `internal/index/tokenizer/client_test.go` (241 LOC) — `httptest`
  fixtures for happy path, timeout, retry, conn-refused, degraded
  fallback, observability emission, nil-safe obs.
- `internal/index/tokenizer/main_test.go` (11 LOC) — `goleak.VerifyTestMain(m)`.

REQ coverage: REQ-IDX-003-008 (sidecar-unreachable fallback),
REQ-IDX-003-009b (Go observability), NFR-IDX-003-006 (goleak).

### Phase C — Shard Routing (Index-Time + Query-Time)

Files (implemented):

- `internal/index/router/shard.go` (93 LOC) — `Shard` type
  (`ShardDefault = "default"`, `ShardKo = "ko"`),
  `IndexShardForDoc(doc) []Shard` (5-case decision table per spec
  §2.3), `QueryShardsForText(text) []Shard` (3-case decision table).
  Pure functions; reuse `internal/router.HangulRatio`,
  `RatioHigh`, `RatioLow` constants.
- `internal/index/router/shard_test.go` (113 LOC) — Table-driven tests
  for index-time (5 cases) and query-time (8 cases including ambiguous
  band).
- `internal/index/router/merge.go` (74 LOC) — `MergeRRF(results
  map[Shard][]NormalizedDoc, k int) []NormalizedDoc`. Uses `k = 60`
  matching `.moai/project/tech.md:141`. Deduplicates by
  `NormalizedDoc.CanonicalHash()`. Sorts by RRF score descending.
  Pure function.
- `internal/index/router/merge_test.go` (116 LOC) — Table tests with
  hand-computed RRF scores; dedup-by-canonical-hash; MaxResults
  respect; stable-for-ties.
- `internal/index/router/main_test.go` (11 LOC) — `goleak.VerifyTestMain(m)`.
- `internal/index/router/testdata/` — Korean golden fixture set
  (5 docs + 3 queries).

REQ coverage: REQ-IDX-003-005 (index-time routing), REQ-IDX-003-006
(query-time routing + parallel fanout), REQ-IDX-003-007 (RRF merge),
REQ-IDX-003-012 (query does not filter by Lang field), REQ-IDX-003-013
(Korean query index-side recall), NFR-IDX-003-004 (Korean-first
golden set).

### Phase D — Meili Korean Shard Settings + Observability

Files (implemented):

- `internal/index/meili/korean_shard.go` (32 LOC) — `EnsureKoreanIndexSettings(ctx,
  client *meili.Client) error` that idempotently patches
  `usearch_docs_ko` settings: `stopWords` = 11 Korean particles from
  the shared list, `localizedAttributes = [{...locales: ["kor"]}]` (or
  `["ko"]` fallback), `searchableAttributes = ["title","body","snippet"]`.
- `internal/index/meili/korean_shard_test.go` (55 LOC) — settings
  acceptance / fallback to `ko` / tolerant-of-both.
- `internal/obs/metrics/tokenizer.go` (NEW) — Declares `TokenizerCalls
  *prometheus.CounterVec{outcome}`, `TokenizerLatency
  *prometheus.HistogramVec{outcome}`, `IndexShardWrites
  *prometheus.CounterVec{shard, outcome}`. Registered via
  `registerTokenizer(pr)`.
- `internal/obs/metrics/metrics.go` — Three new fields added to
  `Registry`; `registerTokenizer(pr)` invocation; `shard` label
  appended to cardinality allowlist (2 enum values: ko, default).
- `services/tokenizer-ko/tests/test_app.py` — REQ-IDX-003-001/003/004/009
  acceptance via `TestClient`.
- `services/tokenizer-ko/tests/test_tokenize.py` — Golden-morpheme
  fixtures (REQ-IDX-003-002).
- `services/tokenizer-ko/tests/test_obs.py` — Log record shape
  (REQ-IDX-003-009).
- `services/tokenizer-ko/tests/fixtures/golden_morphemes.json` — ≥ 30
  Korean strings with expected morpheme arrays.

REQ coverage: REQ-IDX-003-010 (per-shard write counter), REQ-IDX-003-011
(localizedAttributes probing), full observability surface.

---

## 3. Test Catalog Summary

| Phase | Python Tests | Go Tests | REQs Covered | NFRs Covered |
|-------|--------------|----------|--------------|--------------|
| A | test_app (sidecar), test_tokenize (golden), test_obs | — | 001, 002, 003, 004, 009a | 001, 002 |
| B | — | client_test, main_test | 008, 009b | 006 |
| C | — | shard_test, merge_test | 005, 006, 007, 012, 013 | 004, 005, 007 |
| D | — | korean_shard_test | 010, 011 | — |
| **Totals** | **Python (slow incl)** | **Go (race-clean)** | **13 / 13** | **7 / 7** |

---

## 4. Risk Mitigation Table

| Risk | Realised? | Resolution |
|------|-----------|------------|
| `pymecab-ko` thread-unsafety with uvicorn workers | Yes | `asyncio.Lock` serializes Tagger access; uvicorn `--workers 1`. |
| mecab-ko-dic load failure at startup | Yes | `lifespan` raises; container HEALTHCHECK fails; docker-compose restarts. |
| Sidecar unreachable mid-ingest | Yes | Indexer falls back to writing un-tokenized text to `usearch_docs_ko` (Meili's native Charabia/Lindera handles it); sets `Metadata["tokenizer"]="lindera_fallback"`; counter `error_unreachable` +1. |
| HangulRatio threshold drift between SPEC-IR-001 and IDX-003 | No | Direct import of `router.HangulRatio`, `router.RatioHigh`, `router.RatioLow` (no duplication). |
| Cross-shard query overhead doubles latency | Mitigated | Parallel fanout via errgroup; NFR-IDX-003-005 bounds cross-shard at p95 ≤ 200ms. |
| RRF merge non-determinism | No | `MergeRRF` uses stable sort with `CanonicalHash` tie-break. |
| Localized attributes API mismatch across Meili versions | Yes | Three-stage fallback: try `kor` → try `ko` → omit `localizedAttributes` (still works via `stopWords` + pre-tokenization). |

---

## 5. MX Tag Plan (Applied in Source)

### 5.1 @MX:ANCHOR

- `internal/index/router/shard.go::IndexShardForDoc` (line 37) —
  `@MX:ANCHOR` (index-time routing; fan_in ≥ 3 — index.Upsert, router
  tests, bench). `@MX:REASON`: all document writes pass through this
  function. `@MX:SPEC: SPEC-IDX-003`.
- `internal/index/router/shard.go::QueryShardsForText` (line 75) —
  `@MX:ANCHOR` (query-time shard selection; fan_in ≥ 3 — index.Search,
  router tests, bench). `@MX:REASON`: every search request routes
  through this function.

### 5.2 @MX:NOTE / @MX:WARN

- `internal/index/tokenizer/client.go::Tokenize` — `@MX:NOTE` (degraded
  fallback path on sidecar unreachable; see REQ-IDX-003-008).
- `services/tokenizer-ko/src/tokenizer_ko/tokenize.py` — `# @MX:WARN`
  (asyncio.Lock acquisition: removing the lock invalidates pymecab-ko
  thread-safety contract).

---

## 6. File Touch Order (as realised)

1. Phase A: `pyproject.toml` → `models.py` → `tokenize.py` → `obs.py`
   → `app.py` → `__main__.py` → `Dockerfile` →
   `deploy/docker-compose.yml`.
2. Phase B: `internal/index/tokenizer/types.go` → `config.go` →
   `client.go` → `client_test.go` → `main_test.go`.
3. Phase C: `internal/index/router/shard.go` → `shard_test.go` →
   `merge.go` → `merge_test.go` → `main_test.go` →
   `testdata/korean_golden.json`.
4. Phase D: `internal/index/meili/korean_shard.go` →
   `korean_shard_test.go` →
   `internal/obs/metrics/tokenizer.go` →
   `internal/obs/metrics/metrics.go` (registerTokenizer + shard
   allowlist).

---

## 7. Coverage and Quality Gates (Achieved)

- Python coverage on `services/tokenizer-ko/src/tokenizer_ko/`: ≥ 85%.
- Go coverage on `internal/index/tokenizer/` + `internal/index/router/`:
  ≥ 85%.
- `go vet ./internal/index/...` → 0 issues.
- `golangci-lint run ./internal/index/...` → 0 issues.
- `go test -race ./internal/index/...` PASS (NFR-IDX-003-007).
- `goleak.VerifyTestMain` clean for both Go packages (NFR-IDX-003-006).
- Cardinality allowlist test PASS after `shard` label addition.

---

## 8. Pre-submission Self-Review

Verified at implementation time:

- Index-time routing applies the §2.3 5-case decision table verbatim.
- Query-time routing applies the §2.3 3-case decision table; empty
  text degrades to `[ShardDefault]` (ratio = 0).
- RRF merge uses k=60 per `tech.md:141` (no SPEC override).
- Sidecar's `tokenize.py` skips `EOS` line; `joined ==
  " ".join(tokens)` invariant holds.
- Go client retries only on conn-level errors (not on 4xx); 5xx triggers
  degraded fallback path with `lindera_fallback` metadata.
- Lang field is consumed at INDEX-TIME (routing) but IGNORED at
  QUERY-TIME (matching is purely on tokenized content) — verified by
  `TestQueryDoesNotFilterByLangField`.
- 11 Korean particle stop-words sourced from
  `internal/router/korean.go:18` (single source of truth; no duplicate
  list in Python sidecar).

---

## 9. Downstream Wiring

- **SPEC-IDX-001** dispatches to `usearch_docs` only; IDX-003 adds the
  `usearch_docs_ko` shard and the routing layer that selects between
  them per HangulRatio.
- **SPEC-EVAL-003** (M8, Korean-locale benchmark) consumes the
  Korean-first golden fixture set (`testdata/korean_golden.json`) for
  precision@K measurement.
- **SPEC-IDX-004** (multi-tenant) layers `team_id` filter on TOP of
  `usearch_docs_ko`; the Korean shard is treated like the default shard
  for tenancy enforcement.
- **SPEC-FAN-001** (parallel SPEC): unchanged — fanout delivers
  `[]NormalizedDoc`; IDX-003 routes them per `IndexShardForDoc`.

---

*End of SPEC-IDX-003 plan.md (post-hoc).*
