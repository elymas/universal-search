---
id: SPEC-IDX-003
title: Korean Tokenization (mecab-ko + Meili plugin)
version: 0.1.0
milestone: M3 — Fanout, adapters, index
status: draft
priority: P0
owner: expert-backend
methodology: tdd
coverage_target: 85
created: 2026-05-04
updated: 2026-05-04
author: limbowl
issue_number: null
depends_on: [SPEC-BOOT-001, SPEC-IDX-001]
blocks: [SPEC-EVAL-003 (Korean-locale benchmark), SPEC-IDX-004 (multi-tenant ko shard handling)]
---

# SPEC-IDX-003: Korean Tokenization (mecab-ko + Meili plugin)

## HISTORY

- 2026-05-04 (initial draft v0.1.0, limbowl via manager-spec):
  First EARS-formatted SPEC for the Korean tokenization layer. M3 SPEC
  delivering the index-side guarantee for the M3 exit criterion
  (`.moai/project/roadmap.md:150` — "Korean query returns Naver results
  ranked first"). Three coordinated components: (1) `services/tokenizer-ko/`
  Python FastAPI sidecar wrapping `pymecab-ko` (mecab-ko + bundled
  mecab-ko-dic) on port 8083; (2) Meilisearch dual-index configuration
  with `usearch-ko` Korean shard alongside the default `usearch` index
  from SPEC-IDX-001; (3) Go-side index-time and query-time routing in
  `internal/index/tokenizer/` and `internal/index/router/`, reusing
  SPEC-IR-001's `router.HangulRatio` for query-language detection (no
  duplicated Hangul logic). Architectural decision (research §3): Path B
  sidecar pre-tokenization selected over Path A native Meili
  Charabia/Lindera due to mecab-ko-dic quality + 10-20× throughput
  headroom (research §3.3). Path A retained as graceful-degradation
  fallback when sidecar unhealthy. Reuses SPEC-CORE-001
  `NormalizedDoc.Lang` as routing key, SPEC-LLM-001 graceful-degradation
  patterns, SPEC-OBS-001 observability conventions (one new metric
  family `usearch_tokenizer_*`, one new label `shard ∈ {ko, default}`
  on the index-routing counter — cardinality allowlist amendment
  required, see §6.4). 13 EARS REQs (10 P0 + 2 P1 + 1 P2), 7 NFRs.
  Research artifact at `.moai/specs/SPEC-IDX-003/research.md`. Ready
  for plan-auditor review and annotation cycle.

---

## 1. Purpose

The M3 exit criterion in `.moai/project/roadmap.md:150` reads:

> All 12+ adapters pass contract tests; `usearch query` returns fused
> results across ≥5 adapters; **Korean query returns Naver results
> ranked first**.

The "Korean query returns Naver results ranked first" promise has two
necessary conditions:

1. **Index-side**: When a Korean query reaches the retrieval layer,
   Korean documents (those with `NormalizedDoc.Lang == "ko"`) must be
   *findable* by their Korean morphemes. Meilisearch's default tokenizer
   (whitespace + Unicode segmentation) splits Korean text by syllable
   character — losing morpheme boundaries — which makes Korean queries
   match poorly on Korean docs. mecab-ko-style morphological analysis
   restores morpheme boundaries.

2. **Rank-fusion-side**: Once Korean docs match, the cross-adapter rank
   fusion (SPEC-FAN-001 / future SPEC-IDX-001 RRF) must rank Korean
   adapters' results first for Korean queries. This is OUT OF SCOPE
   for SPEC-IDX-003 — it is a fanout-layer decision.

SPEC-IDX-003 is **necessary but not sufficient** for the M3 exit. It
delivers (1) and unblocks (2). Without it, Korean queries return
near-empty result sets from Meili regardless of how well the rank
fusion is tuned.

This SPEC delivers three coordinated components:

- A **Python sidecar service** at `services/tokenizer-ko/` exposing
  `POST /tokenize` over FastAPI on port 8083 (compose-internal). The
  sidecar wraps `pymecab-ko` (PyPI; bundles mecab-ko + mecab-ko-dic)
  to perform Korean morphological segmentation. Inputs: a single
  Korean text string. Outputs: a space-joined string of morphemes
  plus per-morpheme POS tags for diagnostic use.

- A **Meilisearch dual-shard configuration**: the existing `usearch`
  index (SPEC-IDX-001) for non-Korean docs, plus a new `usearch-ko`
  index for Korean docs. The `usearch-ko` index is configured with
  Korean stop-words (the 11-particle list reused from SPEC-IR-001)
  and consumes pre-tokenized text from the sidecar.

- **Go-side index-time and query-time routing** in
  `internal/index/tokenizer/` (HTTP client to sidecar) and
  `internal/index/router/` (lang-based shard selection). Index-time:
  `NormalizedDoc.Lang == "ko"` → call sidecar → write to `usearch-ko`.
  Query-time: `router.HangulRatio(query) ≥ 0.30` → query `usearch-ko`;
  `< 0.10` → query `usearch`; ambiguous band → query both shards and
  merge by Reciprocal Rank Fusion (k=60).

The SPEC reuses (does NOT duplicate):

- `internal/router/korean.go::HangulRatio` and `::KoreanSignals` for
  query-time language detection.
- SPEC-IR-001 ratio thresholds (`ratio_high = 0.30`, `ratio_low = 0.10`).
- SPEC-CORE-001 `NormalizedDoc.Lang` BCP-47 field as the index-time
  routing key.
- SPEC-SYN-001 Python sidecar layout (FastAPI + Pydantic v2 + lifespan
  + Dockerfile + compose entry).
- SPEC-OBS-001 metric family registration pattern under
  `internal/obs/metrics/`.

Completion unblocks **SPEC-EVAL-003** (Korean-locale benchmark — needs
working Korean retrieval to measure precision@K) and **SPEC-IDX-004**
(multi-tenant ko shard handling — needs the ko shard to exist before
team-scoping it). It does NOT unblock SPEC-FAN-001 (parallel SPEC; FAN
delivers fanout independent of Korean tokenization quality).

This SPEC is on the M3 critical path for the Korean-first promise.

---

## 2. Scope

### 2.1 In-Scope

| Ref | Item |
|-----|------|
| a | `services/tokenizer-ko/src/tokenizer_ko/` package layout: `app.py` (FastAPI app + lifespan + routes), `models.py` (Pydantic v2 request/response), `tokenize.py` (pymecab-ko Tagger wrapper + asyncio.Lock serialization), `obs.py` (JSON log records), `__main__.py` (uvicorn entrypoint), `__init__.py` (package doc + version) |
| b | `services/tokenizer-ko/pyproject.toml` runtime deps: `fastapi>=0.115`, `uvicorn[standard]>=0.30`, `pydantic>=2.9`, `mecab-ko>=1.0,<2.0` (pymecab-ko on PyPI; bundles mecab-ko-dic), Python `>=3.11` |
| c | FastAPI `POST /tokenize` endpoint accepting `TokenizeRequest{request_id: str, text: str}` and returning `TokenizeResponse{request_id, tokens: list[str], joined: str, morpheme_count: int, latency_ms: float, dict_version: str}` |
| d | FastAPI `GET /health` endpoint returning `{"status":"ok","version":"0.1.0","dict_version":"<bundled>","tokenizer":"mecab-ko"}` on success or 503 with `{"status":"degraded","reason":"<cause>"}` when Tagger fails to load |
| e | Korean particle stop-word source: reuse the 11-entry list from `internal/router/korean.go:18` exposed via a new Go-side helper `internal/index/tokenizer.KoreanStopWords() []string`. The same list is POSTed to Meili's `usearch-ko` settings under `stopWords`. NO duplicate list maintained in the Python sidecar; the Go-side ingestion path owns the canonical list |
| f | `internal/index/tokenizer/types.go` — Go-side `Request`, `Result`, error sentinels (`ErrInvalidInput`, `ErrSidecarUnreachable`, `ErrTimeout`) |
| g | `internal/index/tokenizer/config.go` — env binder for `TOKENIZER_KO_BASE_URL` (default `http://tokenizer-ko:8083`) and `TOKENIZER_KO_REQUEST_TIMEOUT_SECONDS` (default 1) |
| h | `internal/index/tokenizer/client.go` — Go HTTP client struct `Client` with `Tokenize(ctx, text string) (Result, error)` method; uses `*http.Client{Timeout: 1s}` and exponential backoff retry (2 retries on connection-level errors with 100 ms / 300 ms ± 10% jitter) |
| i | `internal/index/router/shard.go` — `Shard` enum (`ShardDefault`, `ShardKorean`); `IndexShardForDoc(d types.NormalizedDoc) Shard` (index-time routing); `QueryShardsForText(text string) []Shard` (query-time routing returning 1 or 2 shards) |
| j | `internal/index/router/shard.go::QueryShardsForText` thresholds: reuse `ratio_high = 0.30` and `ratio_low = 0.10` constants from SPEC-IR-001 (imported via `router.RatioHigh`, `router.RatioLow` — exported in SPEC-IR-001 if not already, else mirrored as a Go-side constant with `// matches SPEC-IR-001` comment) |
| k | `internal/index/router/merge.go` — RRF merge function `MergeRRF(results map[Shard][]types.NormalizedDoc, k int) []types.NormalizedDoc` with `k = 60` per `.moai/project/tech.md:141`. Used only when `QueryShardsForText` returns 2 shards |
| l | Meili settings management: `internal/index/meili/korean_shard.go` declares the Korean shard config function `EnsureKoreanIndexSettings(ctx, client *meili.Client) error` that idempotently posts to `PATCH /indexes/usearch-ko/settings`: `stopWords` = the 11 Korean particles, `localizedAttributes = [{"attributePatterns": ["title","body","snippet"], "locales": ["kor"]}]` (or `["ko"]` per Open Question §11.1), `searchableAttributes = ["title","body","snippet"]`. Idempotency is verified by reading current settings before write and skipping if equal |
| m | `internal/obs/metrics/tokenizer.go` — NEW file declaring `TokenizerCalls *prometheus.CounterVec{outcome}`, `TokenizerLatency *prometheus.HistogramVec{outcome}`, `IndexShardWrites *prometheus.CounterVec{shard,outcome}` collectors and `registerTokenizer(pr) tokenizerCollectors` helper — owned by SPEC-IDX-003, lives under `internal/obs/metrics/` to preserve the import-boundary test from SPEC-OBS-001 REQ-OBS-006 |
| n | `internal/obs/metrics/metrics.go` — minor edit: add 3 fields to `Registry`, call `registerTokenizer(pr)` from `NewRegistry()`. The `shard` label is a NEW label name and SPEC-IDX-003 amends the cardinality allowlist (`labelNames` slice) to include it. Allowed values are exactly two: `ko`, `default` |
| o | `services/tokenizer-ko/Dockerfile` — multi-stage build on `python:3.11-slim`, non-root user, `HEALTHCHECK CMD curl -f http://localhost:8083/health || exit 1`. Mirrors `services/researcher/Dockerfile:1-29` |
| p | `deploy/docker-compose.yml` delta: new `tokenizer-ko` service entry on port 8083, joins `app` network, healthcheck `curl -f /health`, `restart: unless-stopped`, `start_period: 20s` to allow mecab-ko-dic dictionary load |
| q | Root `.env.example` additions: `TOKENIZER_KO_PORT=8083`, `TOKENIZER_KO_BASE_URL=http://tokenizer-ko:8083`, `TOKENIZER_KO_REQUEST_TIMEOUT_SECONDS=1`, `TOKENIZER_KO_LOG_LEVEL=INFO` |
| r | `services/tokenizer-ko/tests/` test files: `test_app.py` (HTTP endpoint contract via FastAPI `TestClient`), `test_tokenize.py` (mecab-ko output assertions on Korean fixture set), `test_obs.py` (JSON log record shape), `tests/fixtures/golden_morphemes.json` (≥ 30 Korean strings with expected morpheme arrays) |
| s | `internal/index/tokenizer/client_test.go` — Go HTTP client integration tests against an in-process FastAPI fixture launched via `httptest.NewServer` returning canned JSON; covers happy path, timeout, retry, connection refused, degraded-mode passthrough |
| t | `internal/index/router/shard_test.go` — table-driven tests for `IndexShardForDoc` (5 cases) and `QueryShardsForText` (8 cases incl. ambiguous band) |
| u | `internal/index/router/merge_test.go` — RRF merge tests with synthetic two-shard result sets |
| v | `services/tokenizer-ko/.env.example`, `services/tokenizer-ko/README.md` |

### 2.2 Out-of-Scope (Exclusions — What NOT to Build)

[HARD] This SPEC explicitly excludes the following items. Each has a known
destination SPEC; this list prevents scope creep into IDX-003.

- **Korean-locale benchmarking and precision@K measurement** — SPEC-IDX-003
  delivers the index plumbing; SPEC-EVAL-003 (M8) measures the
  Korean-first promise.
- **Cross-adapter rank fusion guaranteeing "Naver ranked first"** — that
  promise is enforced at the fanout/RRF layer. → SPEC-FAN-001 (M3 — Naver
  adapter ordering) and SPEC-IDX-001 (M3 — RRF fusion across shards).
- **Multi-tenant Korean shard isolation** (per-team `usearch-ko-{team_id}`
  indices) → SPEC-IDX-004 (M6).
- **Korean synonym handling** (e.g., 스타트업 ↔ 창업 ↔ startup) → future
  SPEC if SPEC-EVAL-003 measures synonym-driven recall gaps.
- **Custom mecab-ko user dictionary** (`-u user.dic`) for domain-specific
  terms → research §6.2; deferred to future SPEC.
- **Streaming tokenization** (`POST /tokenize/stream` SSE) — V1 returns a
  single JSON response. No measured value at expected doc sizes.
- **Batch tokenization endpoint** (`POST /tokenize/batch` accepting an
  array) — V1 single-doc only; the Go ingestion path fans out via
  goroutines. Revisit if measured throughput < target.
- **Lindera-ko-dic native Meili tokenization (Path A)** — Charabia /
  Lindera Korean is retained ONLY as the graceful-degradation fallback
  when the `tokenizer-ko` sidecar is unhealthy. Not the primary path.
  → research §3.3 for trade-off rationale.
- **khaiii / KOMORAN / OKT alternative tokenizers** → research §2.2-2.4;
  rejected for V1.
- **Hot-reload of Korean stop-words list** — list ships hard-coded in
  `internal/router/korean.go`. → future SPEC if drift becomes a concern.
- **Hangul Jamo Extended-B (U+D7B0–D7FF) support** — IR-001's
  `isHangulRune` ranges are reused as-is. → research §10 Open Question.
- **Tokenization caching** by text hash — V1 is stateless. Future SPEC
  if measured value.
- **Pre-flight token estimation / cost prediction** — tokenization is
  CPU-only; no per-call cost.
- **Direct exposure of `tokenizer-ko` to external clients** — sidecar
  binds to compose-internal port 8083, not exposed via reverse proxy.
- **Synthesis re-tokenization** — SPEC-SYN-001 / future SPEC-SYN-003 may
  need Korean-aware tokenization for chunking; that is their concern,
  not SPEC-IDX-003's.
- **GitHub Issue tracking on this SPEC** (`issue_number: null`).

### 2.3 Routing Decision Tables

[HARD] The following two tables specify the index-time and query-time
routing logic precisely. Acceptance tests (§4) drive these tables verbatim.

**Index-time routing — `IndexShardForDoc(d types.NormalizedDoc) Shard`**:

| `d.Lang`           | `HangulRatio(d.Body)` | Result                                                  |
|--------------------|-----------------------|---------------------------------------------------------|
| `"ko"`             | `≥ 0.10`              | `ShardKorean` (write to `usearch-ko` only)              |
| `"ko"`             | `< 0.10`              | `[ShardDefault, ShardKorean]` (defensive dual-write — research §7.2 mixed-source-but-English-content) |
| `"en"`, `"ja"`, ...| any                   | `ShardDefault`                                          |
| `""` (unknown)     | `≥ 0.30`              | `ShardKorean` (defensive routing — adapter forgot Lang) |
| `""` (unknown)     | `< 0.30`              | `ShardDefault`                                          |

**Query-time routing — `QueryShardsForText(text string) []Shard`**:

| `HangulRatio(text)` | Result                                |
|---------------------|---------------------------------------|
| `≥ 0.30`            | `[ShardKorean]`                       |
| `< 0.10`            | `[ShardDefault]`                      |
| `0.10 ≤ r < 0.30`   | `[ShardDefault, ShardKorean]` (RRF)   |

Tie-break for index-time `Lang == "ko"` AND `HangulRatio(Body) < 0.10`:
the doc IS dual-written. The intuition is that a Korean source (Naver
shopping listing, Korean blog with English-only title) should be findable
by both Korean queries (its source-language hint matches) and English
queries (its actual text content matches).

---

## 3. EARS Requirements

### Functional Requirements

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| REQ-IDX-003-001 | Ubiquitous | The Python sidecar SHALL expose `POST /tokenize` accepting `application/json` matching `TokenizeRequest{request_id: str, text: str}` and returning HTTP 200 with `application/json` matching `TokenizeResponse{request_id, tokens: list[str], joined: str, morpheme_count: int, latency_ms: float, dict_version: str}`. Pydantic config SHALL be `ConfigDict(extra="forbid", str_strip_whitespace=True)`. The `joined` field SHALL be exactly `" ".join(tokens)`. | P0 | `test_tokenize_happy_path` POSTs Korean text, asserts 200 + response shape; `test_tokenize_extra_field_rejected` asserts 422 on unknown fields; `test_joined_equals_space_join_of_tokens` asserts the invariant. |
| REQ-IDX-003-002 | Event-Driven | WHEN `POST /tokenize` is invoked with non-empty text, the sidecar SHALL invoke `pymecab_ko.Tagger.parse(text)` (serialized via asyncio.Lock to honour pymecab-ko thread-unsafety per research §5.1), extract surface forms (the first column of each MeCab line) ignoring lines `EOS` and empty lines, and return them in input order. The returned `morpheme_count` SHALL equal `len(tokens)`. | P0 | `test_tokenize_korean_morphemes` feeds `"ChatGPT 사용법"` and asserts tokens contain at least `["ChatGPT","사용","법"]` (or equivalent — pymecab-ko's exact split is fixture-driven); `test_morpheme_count_matches_tokens_length`; `test_concurrent_tokenize_safe` issues 50 concurrent requests; assertions hold. |
| REQ-IDX-003-003 | State-Driven | WHILE the `pymecab_ko.Tagger` instance is unable to load mecab-ko-dic at startup (FileNotFoundError, RuntimeError from native binding) the FastAPI app SHALL refuse to start by raising during `lifespan` startup; the container's HEALTHCHECK SHALL fail; docker-compose SHALL restart per `restart: unless-stopped` policy. | P0 | `test_lifespan_raises_when_dict_missing` patches `pymecab_ko.Tagger` to raise `RuntimeError`; assert `lifespan` raises before app accepts requests. |
| REQ-IDX-003-004 | Unwanted | IF `POST /tokenize` is invoked with `text == ""` (after `str_strip_whitespace`) OR `len(text) > MAX_INPUT_BYTES` (default 65536), THEN the sidecar SHALL return HTTP 400 with body `{"error":"invalid_input","detail":"<which>"}`, SHALL NOT invoke the Tagger, SHALL increment `usearch_tokenizer_calls_total{outcome="error_invalid"}` exactly once (Go-side; sidecar emits its own log), and SHALL emit one WARN-level structured log record with attributes `{request_id, error}`. | P0 | `test_empty_text_returns_400`; `test_oversize_input_returns_400`; assert no Tagger call. |
| REQ-IDX-003-005 | Event-Driven | WHEN the Go ingestion path receives a `NormalizedDoc` and computes `IndexShardForDoc(d)` per §2.3 table, the indexer SHALL: (a) for `[ShardDefault]` write to `usearch` only, (b) for `[ShardKorean]` call `tokenizer.Client.Tokenize(ctx, d.Body)`, replace `d.Title`/`d.Body`/`d.Snippet` with their tokenized (space-joined) forms, then write to `usearch-ko` only, (c) for `[ShardDefault, ShardKorean]` (defensive dual-write) write the original doc to `usearch` AND the tokenized doc to `usearch-ko`. The pre-tokenized fields and the original fields SHALL NOT be conflated — each shard receives only its appropriate doc shape. | P0 | `TestIndexerDefaultShardEnglish`, `TestIndexerKoreanShardLang`, `TestIndexerDualWriteHybridDoc`, `TestIndexerCallsTokenizerForKoreanShard`, `TestIndexerSkipsTokenizerForDefaultShard`, `TestIndexerOriginalAndTokenizedFieldsDistinct`. |
| REQ-IDX-003-006 | Event-Driven | WHEN the Go query path is invoked with a `text` string and computes `QueryShardsForText(text)` per §2.3 table, the query orchestrator SHALL issue one Meili query per returned shard (in parallel via errgroup) with the same query text and `MaxResults`, AND SHALL NOT pre-tokenize the query text before sending to Meili (Meili's per-shard settings handle Korean tokenization on the index side; query-side pre-tokenization is unnecessary for `usearch-ko` because Meili treats query text consistently with how it treated the indexed text — but SPEC-IDX-003 keeps queries un-tokenized for query-side simplicity, validated in run-phase via fixture matches). | P0 | `TestQueryShardSelection` table-driven over the §2.3 query table; `TestQueryFanoutIssuesParallelMeiliCalls`; `TestQueryDoesNotPreTokenize`. |
| REQ-IDX-003-007 | Ubiquitous | WHEN `QueryShardsForText` returns 2 shards, the orchestrator SHALL merge the per-shard `[]NormalizedDoc` results via Reciprocal Rank Fusion `score(d) = Σ 1 / (60 + rank_i(d))` with `k = 60` (matching `.moai/project/tech.md:141`), de-duplicate by `NormalizedDoc.CanonicalHash()`, sort descending by RRF score, and return the top `MaxResults`. | P0 | `TestMergeRRFTwoShards` synthesizes two ranked lists and asserts merged top-K matches hand-computed RRF scores; `TestMergeDedupByCanonicalHash` (same doc in both shards is counted once). |
| REQ-IDX-003-008 | State-Driven | WHILE the `tokenizer-ko` sidecar is unreachable (Go HTTP client returns connection-level error after retry exhaustion) OR returns 5xx, the indexer SHALL still write Korean-shard-bound docs to `usearch-ko` BUT with the un-tokenized text (Meili's native Charabia/Lindera tokenization handles them — research §3.3 fallback), set `Metadata["tokenizer"] = "lindera_fallback"` on the indexed doc, and increment `usearch_tokenizer_calls_total{outcome="error_unreachable"}` exactly once. The indexer SHALL NOT block the ingestion pipeline. | P0 | `TestIndexerDegradedSidecarFallback` injects fake HTTP server returning 503; assert doc is still written to `usearch-ko` with `Metadata["tokenizer"] == "lindera_fallback"`; assert `error_unreachable` counter +1. |
| REQ-IDX-003-009 | Ubiquitous | The Python sidecar AND the Go client SHALL emit per-`/tokenize` invocation observability: (a) the sidecar logs a single JSON record at INFO with `{request_id, text_len, morpheme_count, latency_ms, outcome}`; outcome ∈ `{success, error_invalid, error_internal}`; (b) the Go client increments `obs.TokenizerCalls.WithLabelValues(outcome).Inc()` exactly once, observes `obs.TokenizerLatency.WithLabelValues(outcome).Observe(elapsed_seconds)` exactly once, and creates+ends one OTel span `tokenizer.tokenize` with attributes `{request_id, text_len, morpheme_count, latency_ms, outcome}`. The Go client SHALL be nil-safe across `obs.Obs`, individual collectors, and `obs.Logger` per the pattern at `internal/llm/client.go:244-251`. | P0 | `test_python_log_record_shape`; `TestClientEmitsCounter`, `TestClientEmitsHistogram`, `TestClientEmitsOTelSpan`, `TestClientObservabilitySafeOnNilObs`. |
| REQ-IDX-003-010 | Ubiquitous | WHEN any Korean-shard write occurs (success or fallback), the indexer SHALL increment `obs.IndexShardWrites.WithLabelValues(shard, outcome).Inc()` exactly once where `shard ∈ {ko, default}` and `outcome ∈ {success, fallback, error}`; the metric provides cross-shard write-rate observability for ops dashboards. | P0 | `TestEmitIndexShardWrites` × 6 (all combinations of shard × outcome). |
| REQ-IDX-003-011 | Optional | WHERE the Meili instance supports `localizedAttributes` (verified at run-phase by inspecting Meili version response — see Open Question §11.1), the indexer SHALL apply the locale code `kor` to `usearch-ko` index settings; WHERE Meili rejects `kor`, the indexer SHALL fall back to `ko`; WHERE Meili rejects both, the indexer SHALL log a WARN, omit `localizedAttributes` entirely, and proceed with `stopWords` + pre-tokenization only (the primary pre-tokenization path does not require `localizedAttributes` to function). | P1 | `TestMeiliSettingsAcceptsKor`, `TestMeiliSettingsFallsBackToKo`, `TestMeiliSettingsTolerantOfBoth`. |
| REQ-IDX-003-012 | Event-Driven | WHEN a Korean query results in a `usearch-ko` shard query, the index layer SHALL NOT exclude or down-rank docs whose `Lang == "ko"` field value is missing or empty; matching is purely on tokenized content, not on `Lang` metadata. The `Lang` field is consumed at INDEX-TIME (routing) but ignored at QUERY-TIME (matching). | P0 | `TestQueryDoesNotFilterByLangField` indexes a doc to `usearch-ko` with `Lang=""` and verifies it remains queryable. |
| REQ-IDX-003-013 | Ubiquitous | WHEN a Korean query (per §2.3 query table = `[ShardKorean]` only) is issued AND at least one indexed doc on `usearch-ko` has tokenized content matching at least one query morpheme, the top-K result set returned by the index layer SHALL contain at least one such matching doc. The index layer SHALL NOT silently exclude Korean docs from a Korean query. | P0 | `TestKoreanQueryReturnsAtLeastOneKoreanDoc` — golden fixture: 5 Korean docs indexed, query "ChatGPT 사용법", asserts ≥ 1 doc returned. |

### Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| NFR-IDX-003-001 | Sidecar throughput | The Python sidecar SHALL sustain ≥ 1000 `/tokenize` requests per second on a single worker on commodity x86 (4 vCPU, 8 GB RAM), measured by `services/tokenizer-ko/tests/test_throughput.py` (slow-marked, skipped on default `pytest -q`) issuing 10000 sequential async-batched requests of 100 Korean chars each. Measurement window: median over 3 runs. |
| NFR-IDX-003-002 | Single-doc tokenize p50 latency | The Python sidecar SHALL complete a single `/tokenize` call with p50 ≤ 5 ms end-to-end (HTTP request to response) on a 100-character Korean input when running on the same hardware. Measurement: 200 sequential calls, sort durations, assert `durations[100] ≤ 0.005` seconds. |
| NFR-IDX-003-003 | Batch tokenize p50 (via fanout) | The Go-side fanout calling the sidecar in parallel SHALL achieve p50 ≤ 50 ms wall-clock for a batch of 100 documents (200 chars each), measured by `internal/index/tokenizer/bench_test.go::BenchmarkBatchTokenize100Docs`. Measurement: 100 batches; 50th percentile of wall-clock per batch. |
| NFR-IDX-003-004 | Korean-first index-side guarantee | Per REQ-IDX-003-013, golden Korean test fixtures (≥ 5 fixed Korean docs covering varied morpheme classes) MUST yield ≥ 1 match for their corresponding Korean query; this NFR formalizes the M3 exit-criterion contribution at the index layer. The fixture set lives at `internal/index/router/testdata/korean_golden.json`; CI runs the assertion on every push. |
| NFR-IDX-003-005 | Cross-shard query latency | When `QueryShardsForText` returns 2 shards (ambiguous band), the orchestrator's wall-clock budget SHALL be ≤ 200 ms p95 from `Search` entry to merged result return, against the in-process Meili stub that responds with median 80 ms + jitter to ±30 ms. Measurement: `internal/index/router/bench_test.go::BenchmarkCrossShardQuery`. |
| NFR-IDX-003-006 | goleak discipline | The Go-side test suite for `internal/index/tokenizer/` and `internal/index/router/` SHALL invoke `goleak.VerifyNone(t)` (or `goleak.VerifyTestMain(m)` in `TestMain`) to confirm no goroutine leaks across the package's tests. |
| NFR-IDX-003-007 | race-clean | `go test -race ./internal/index/...` SHALL be clean. The Go-side concurrent paths (parallel Meili shard queries, parallel sidecar tokenization fanout, concurrent counter increments) MUST NOT trigger the race detector. Runs in CI on every push. |

---

## 4. Acceptance Criteria

### REQ-IDX-003-001 — `/tokenize` Endpoint Contract

- File `services/tokenizer-ko/src/tokenizer_ko/app.py` declares a FastAPI
  application; route `POST /tokenize` is registered.
- File `services/tokenizer-ko/src/tokenizer_ko/models.py` declares
  `TokenizeRequest` and `TokenizeResponse` Pydantic v2 models with
  `ConfigDict(extra="forbid", str_strip_whitespace=True)`.
- `test_tokenize_happy_path`: POST with `{"request_id":"r1","text":"안녕하세요"}`
  returns 200 + response with `tokens`, `joined == " ".join(tokens)`,
  `morpheme_count == len(tokens)`, `dict_version` non-empty string,
  `latency_ms ≥ 0`.
- `test_tokenize_extra_field_rejected`: POST with `{...,"unexpected": 1}`
  returns 422 (FastAPI Pydantic validation).
- `test_joined_equals_space_join_of_tokens`: assert
  `response["joined"] == " ".join(response["tokens"])` for arbitrary
  Korean inputs.
- `test_tokenize_response_shape_matches_schema`: returned JSON validates
  against `TokenizeResponse.model_json_schema()`.

### REQ-IDX-003-002 — mecab-ko Tokenization

- `test_tokenize_korean_morphemes`: feed `"ChatGPT 사용법"` → assert
  tokens list contains at least 3 morphemes whose surface forms cover
  the input characters; exact split is fixture-driven against
  `tests/fixtures/golden_morphemes.json`.
- `test_morpheme_count_matches_tokens_length`: assert
  `response["morpheme_count"] == len(response["tokens"])`.
- `test_concurrent_tokenize_safe`: launch 50 concurrent
  `POST /tokenize` requests via `asyncio.gather` against the
  TestClient; assert all return 200 and tokenization is consistent
  (same input → same output across concurrent calls).
- `test_eos_lines_excluded`: when mecab-ko output contains `EOS` line,
  assert it is NOT in the returned `tokens`.

### REQ-IDX-003-003 — Lifespan Failure on Missing Dict

- `test_lifespan_raises_when_dict_missing`: `monkeypatch` replaces
  `pymecab_ko.Tagger` with a class that raises `RuntimeError("dict load
  failed")` on construction; assert `lifespan` startup raises before
  the app accepts requests.
- `test_health_returns_503_when_tagger_unhealthy`: when Tagger fails to
  init (caught and Tagger left None), `/health` returns 503 with
  `{"status":"degraded","reason":"<msg>"}`.

### REQ-IDX-003-004 — Empty / Oversize Input Rejection

- `test_empty_text_returns_400`: POST with `text: "   "` returns 400 +
  body `{"error":"invalid_input","detail":"text"}`.
- `test_oversize_input_returns_400`: POST with `text` of 65537 bytes
  returns 400 + `detail: "size"`.
- `test_invalid_input_no_tagger_call`: assert no Tagger.parse call for
  either failure case (mock the Tagger).
- `test_invalid_input_logs_warn`: captured JSON log records contain
  exactly one WARN entry per failure with `{request_id, error}`.

### REQ-IDX-003-005 — Index-Time Routing

- `TestIndexerDefaultShardEnglish`: `NormalizedDoc{Lang:"en", Body:"hello"}`
  is written to `usearch` only; tokenizer sidecar NOT called.
- `TestIndexerKoreanShardLang`: `NormalizedDoc{Lang:"ko", Body:"안녕하세요"}`
  is written to `usearch-ko` only; tokenizer sidecar called once;
  the indexed doc has tokenized fields (mock sidecar returns
  `"안녕 하세요"`); `usearch` receives nothing.
- `TestIndexerDualWriteHybridDoc`: `NormalizedDoc{Lang:"ko",
  Body:"buy iphone 16 pro"}` (Korean source, English-only body, Hangul
  ratio < 0.10) is written to BOTH `usearch` (original) AND
  `usearch-ko` (tokenized via sidecar).
- `TestIndexerCallsTokenizerForKoreanShard`: counter assertion that
  sidecar HTTP call count == number of Korean-shard writes.
- `TestIndexerSkipsTokenizerForDefaultShard`: counter assertion that
  English-only docs do NOT trigger sidecar calls.
- `TestIndexerOriginalAndTokenizedFieldsDistinct`: a dual-written doc's
  `usearch` copy and `usearch-ko` copy have different `Body` field
  values (one original, one space-joined morphemes).

### REQ-IDX-003-006 — Query-Time Routing + Parallel Fanout

- `TestQueryShardSelection`: table-driven over §2.3 query table, 8 cases:
  - `text="hello world"` → `[ShardDefault]`
  - `text="안녕하세요"` (high Hangul) → `[ShardKorean]`
  - `text="best Korean LLM 모델"` (ambiguous) → `[ShardDefault, ShardKorean]`
  - `text=""` → `[ShardDefault]` (degenerate; ratio = 0)
  - `text="A"` (single English char) → `[ShardDefault]`
  - `text="가나다 hello world test"` (low Hangul) → `[ShardDefault, ShardKorean]`
    (ratio = 3/16 ≈ 0.19 — in ambiguous band)
  - `text="가나다라마"` (all Hangul) → `[ShardKorean]`
  - `text="hello 안녕"` (50% Hangul) → `[ShardKorean]`
- `TestQueryFanoutIssuesParallelMeiliCalls`: in-memory Meili stub records
  call timestamps; assert two parallel calls have overlapping execution
  windows (both started within 5 ms of each other).
- `TestQueryDoesNotPreTokenize`: the Meili stub records the query body;
  assert it equals the input text verbatim (no tokenization applied).

### REQ-IDX-003-007 — RRF Merge Two Shards

- `TestMergeRRFTwoShards`: synthesize
  `usearch_results = [docA, docB, docC]` and
  `usearch_ko_results = [docB, docD, docE]`; compute RRF scores manually
  with `k=60`:
  - `docA` rank 0 in shard1 → `1/(60+0+1) = 1/61`
  - `docB` rank 1 in shard1 + rank 0 in shard2 → `1/62 + 1/61`
  - `docC` rank 2 in shard1 → `1/63`
  - `docD` rank 1 in shard2 → `1/62`
  - `docE` rank 2 in shard2 → `1/63`
  - sorted: docB > docA ≈ docD > docC ≈ docE
  - assert returned order matches.
- `TestMergeDedupByCanonicalHash`: feed the same `NormalizedDoc` value
  in both shards; assert merged result contains it once with the merged
  RRF score.
- `TestMergeRespectsMaxResults`: 100 docs in 2 shards → `MaxResults=10`
  → returned slice has length ≤ 10.
- `TestMergeStableForTies`: docs with identical RRF scores preserve
  insertion order (deterministic tiebreak).

### REQ-IDX-003-008 — Sidecar-Unreachable Fallback

- `TestIndexerDegradedSidecarFallback`: stub `httptest.Server` returns
  503 on every `/tokenize`; index a Korean doc; assert: (a) doc is
  written to `usearch-ko` with un-tokenized text, (b)
  `doc.Metadata["tokenizer"] == "lindera_fallback"`, (c) Go-side
  counter `usearch_tokenizer_calls_total{outcome="error_unreachable"}`
  +1, (d) total elapsed ≤ 1.5 s (timeout + retries bounded).
- `TestIndexerDegradedSidecarConnRefused`: server is closed; same
  assertions; outcome label is still `error_unreachable`.
- `TestIndexerDegradedSidecarTimeout`: server sleeps 5 s; client
  timeout 1 s; same assertions; total elapsed ≤ 1.5 s.

### REQ-IDX-003-009 — Per-Call Observability

Python:
- `test_python_log_record_shape`: capture stdout; assert exactly 1 JSON
  line per `/tokenize` invocation with the 5 documented attributes;
  `outcome` value is one of the 3 enums.
- `test_python_log_no_pii`: assert log records do NOT contain the
  request `text` value (only `text_len`).

Go:
- `TestClientEmitsCounter`: outcome ∈ `{success, error_invalid,
  error_unreachable, error_timeout}` each fires the matching code
  path; counter increments by 1 each.
- `TestClientEmitsHistogram`: histogram count == 1 per call, sum > 0.
- `TestClientEmitsOTelSpan`: in-memory span exporter captures one span
  named `tokenizer.tokenize` with the 5 documented attributes.
- `TestClientObservabilitySafeOnNilObs`: construct Client with
  `obs: nil`; call does not panic; returns valid `Result`.

### REQ-IDX-003-010 — Per-Shard-Write Counter

- `TestEmitIndexShardWrites`: 6 sub-tests covering the cartesian
  product `shard ∈ {ko, default}` × `outcome ∈ {success, fallback,
  error}`; each fires the matching code path; counter increments by 1
  each.
- `TestIndexShardWritesCardinality`: assert exactly 6 unique
  `(shard, outcome)` label combinations are observed in the test
  registry (cardinality discipline per NFR-OBS-002).

### REQ-IDX-003-011 — `localizedAttributes` Probing

- `TestMeiliSettingsAcceptsKor`: stub Meili accepts
  `locales: ["kor"]` (returns 200); assert request body sent matches.
- `TestMeiliSettingsFallsBackToKo`: stub returns 400 on `kor`, 200 on
  `ko`; assert two PATCH calls observed (first with `kor`, second with
  `ko`).
- `TestMeiliSettingsTolerantOfBoth`: stub returns 400 on both; assert
  WARN log + fallback to no `localizedAttributes` (only `stopWords`
  and `searchableAttributes` set); index settings function still
  returns nil error.

### REQ-IDX-003-012 — Query Does Not Filter by Lang

- `TestQueryDoesNotFilterByLangField`: index a Korean doc with
  `Lang=""` to `usearch-ko` (via a path that bypasses
  `IndexShardForDoc` — direct write fixture); assert subsequent
  Korean query returns the doc.

### REQ-IDX-003-013 — Korean Query Index-Side Recall

- `TestKoreanQueryReturnsAtLeastOneKoreanDoc`: golden fixture indexes
  5 Korean docs covering: news article, Naver-blog-style post, Korean
  shopping listing, mixed Korean-English post, pure Hangul post. For
  each query in `internal/index/router/testdata/korean_golden.json`
  (e.g., "ChatGPT 사용법", "AI 추천", "서울 날씨"), assert at least
  one fixture doc is returned in the top-10 result set.
- `TestKoreanQueryFiltersByContent`: the same query for non-Korean
  content does NOT spuriously match; index the same 5 fixtures;
  query "weather forecast" returns 0 from the Korean shard (English
  content gets routed to `usearch`, not `usearch-ko`).

### NFR-IDX-003-001 — Sidecar Throughput

- `services/tokenizer-ko/tests/test_throughput.py::test_throughput_1000rps`
  marked `@pytest.mark.slow`; runs 10000 async-batched requests; asserts
  `total_seconds < 10.0` (≥ 1000 RPS).
- Skipped on default `pytest -q`; included only when `pytest -m slow` or
  CI scheduled-weekly job.

### NFR-IDX-003-002 — Single-Doc p50 Latency

- `test_tokenize_p50_latency_under_5ms`: 200 sequential
  `POST /tokenize` calls of 100-char Korean text; sort durations;
  assert `durations[100] ≤ 0.005` seconds.
- Marked `@pytest.mark.slow`; runs in CI scheduled-weekly job.

### NFR-IDX-003-003 — Batch p50 via Go Fanout

- `internal/index/tokenizer/bench_test.go::BenchmarkBatchTokenize100Docs`
  measures wall-clock for parallel fanout of 100 docs through the Go
  client to a stub server that mimics the sidecar's typical 4 ms
  response time; reports p50 ≤ 50 ms.

### NFR-IDX-003-004 — Korean-First Index-Side

- `internal/index/router/router_test.go::TestKoreanFirstGoldenSet` runs
  the golden fixtures from REQ-IDX-003-013 acceptance and asserts
  every Korean query returns ≥ 1 Korean fixture in top-K. CI gate on
  every push.

### NFR-IDX-003-005 — Cross-Shard Latency

- `internal/index/router/bench_test.go::BenchmarkCrossShardQuery` issues
  100 cross-shard queries against the in-process stub; sorts elapsed
  durations; asserts `durations[95] ≤ 0.200` seconds.

### NFR-IDX-003-006 — goleak

- `internal/index/tokenizer/main_test.go::TestMain` invokes
  `goleak.VerifyTestMain(m)`.
- `internal/index/router/main_test.go::TestMain` invokes
  `goleak.VerifyTestMain(m)`.

### NFR-IDX-003-007 — race-clean

- CI workflow `.github/workflows/go.yml` (existing, SPEC-BOOT-001)
  runs `go test -race ./internal/index/...`; SPEC-IDX-003 PR must pass.

---

## 5. Technical Approach

### 5.1 Files to Create

**Python sidecar** (`services/tokenizer-ko/`):

- `src/tokenizer_ko/__main__.py` — `python -m tokenizer_ko` runs
  `uvicorn.run(app, host="0.0.0.0", port=int(os.getenv("TOKENIZER_KO_PORT","8083")))`.
- `src/tokenizer_ko/__init__.py` — package doc + `__version__`.
- `src/tokenizer_ko/app.py` — FastAPI app with `lifespan` async
  context manager that constructs the `pymecab_ko.Tagger` once at
  startup and exposes it via app state; routes `POST /tokenize`,
  `GET /health`.
- `src/tokenizer_ko/models.py` — Pydantic v2 models
  (`TokenizeRequest`, `TokenizeResponse`).
- `src/tokenizer_ko/tokenize.py` — `async def tokenize(text: str,
  tagger, lock: asyncio.Lock) -> TokenizeResponse`: acquires `lock`,
  calls `tagger.parse(text)`, parses MeCab output (skip `EOS` and
  empty lines, take column 0), returns response.
- `src/tokenizer_ko/obs.py` — JSON-formatted stdlib `logging` setup +
  `class Timer` context manager; exposes `log_tokenize(record: dict)`.
- `tests/test_app.py` — FastAPI `TestClient` covering REQ-IDX-003-001/003/004/009.
- `tests/test_tokenize.py` — golden-morpheme tests (REQ-IDX-003-002).
- `tests/test_obs.py` — log record shape tests (REQ-IDX-003-009).
- `tests/test_throughput.py` — slow-marked NFR-IDX-003-001.
- `tests/fixtures/golden_morphemes.json` — ≥ 30 Korean strings with
  expected morpheme arrays.

**Go-side**:

- `internal/index/tokenizer/types.go` — `Request`, `Result`, error
  sentinels (`ErrInvalidInput`, `ErrSidecarUnreachable`, `ErrTimeout`).
- `internal/index/tokenizer/config.go` — env loader for
  `TOKENIZER_KO_BASE_URL`, `TOKENIZER_KO_REQUEST_TIMEOUT_SECONDS`.
- `internal/index/tokenizer/client.go` — `Client` struct + `New(cfg, *obs.Obs)`
  + `Tokenize(ctx, text string) (Result, error)`.
- `internal/index/tokenizer/client_test.go` — REQ-IDX-003-005/008/009 tests.
- `internal/index/tokenizer/main_test.go` — `goleak.VerifyTestMain(m)`.
- `internal/index/tokenizer/bench_test.go` — NFR-IDX-003-003.
- `internal/index/router/shard.go` — `Shard` enum,
  `IndexShardForDoc`, `QueryShardsForText`.
- `internal/index/router/shard_test.go` — REQ-IDX-003-005/006 tests.
- `internal/index/router/merge.go` — `MergeRRF` function.
- `internal/index/router/merge_test.go` — REQ-IDX-003-007 tests.
- `internal/index/router/router_test.go` — orchestration test
  including NFR-IDX-003-004 golden-set check.
- `internal/index/router/main_test.go` — `goleak.VerifyTestMain(m)`.
- `internal/index/router/bench_test.go` — NFR-IDX-003-005.
- `internal/index/router/testdata/korean_golden.json` — 5 Korean docs +
  3 Korean queries with expected match-presence.
- `internal/index/meili/korean_shard.go` — `EnsureKoreanIndexSettings`
  function (idempotent settings PATCH).
- `internal/index/meili/korean_shard_test.go` — REQ-IDX-003-011 tests.
- `internal/obs/metrics/tokenizer.go` — `TokenizerCalls`,
  `TokenizerLatency`, `IndexShardWrites` collectors;
  `registerTokenizer(pr)` helper.

**Modified**:

- `services/tokenizer-ko/pyproject.toml` — runtime deps per §2.1(b).
- `services/tokenizer-ko/Dockerfile` — multi-stage on `python:3.11-slim`,
  HEALTHCHECK on `/health`.
- `services/tokenizer-ko/.env.example`, `services/tokenizer-ko/README.md`.
- `deploy/docker-compose.yml` — new `tokenizer-ko` service entry.
- `internal/obs/metrics/metrics.go` — register tokenizer collectors;
  amend `labelNames` allowlist to include `shard`.
- `internal/obs/obs.go` — re-export `obs.TokenizerCalls`,
  `obs.TokenizerLatency`, `obs.IndexShardWrites`.
- `internal/index/index.go` — replace stub with package doc and
  re-exports.
- `.env.example` — append tokenizer env vars.
- `internal/router/korean.go` — minor edit: export `RatioHigh = 0.30`
  and `RatioLow = 0.10` constants if not already exported, so
  `internal/index/router/shard.go` can consume them without
  duplicating.

### 5.2 Tokenization Algorithm (Python)

```python
async def tokenize(text: str, tagger, lock: asyncio.Lock) -> dict:
    if not text:
        raise ValueError("empty text")
    if len(text.encode("utf-8")) > MAX_INPUT_BYTES:
        raise ValueError("size")
    started = time.perf_counter()
    async with lock:
        # pymecab-ko Tagger.parse is C-blocking; run in thread to avoid
        # blocking the asyncio loop.
        raw = await asyncio.to_thread(tagger.parse, text)
    latency_ms = (time.perf_counter() - started) * 1000.0
    tokens: list[str] = []
    for line in raw.splitlines():
        if not line or line == "EOS":
            continue
        # MeCab output: surface\tfeatures
        surface = line.split("\t", 1)[0]
        if surface:
            tokens.append(surface)
    return {
        "request_id": ...,
        "tokens": tokens,
        "joined": " ".join(tokens),
        "morpheme_count": len(tokens),
        "latency_ms": latency_ms,
        "dict_version": _dict_version,
    }
```

`_dict_version` is read once at startup from the Tagger's
`dictionary_info()` method (or falls back to `pymecab-ko` package
version when introspection fails).

### 5.3 Go HTTP Client Sketch

```go
// internal/index/tokenizer/client.go (sketch — final shape in run phase)

type Client struct {
    httpClient *http.Client
    baseURL    string
    obs        *obs.Obs
}

func New(cfg Config, o *obs.Obs) (*Client, error) {
    return &Client{
        httpClient: &http.Client{Timeout: cfg.RequestTimeout},
        baseURL:    cfg.BaseURL,
        obs:        o,
    }, nil
}

func (c *Client) Tokenize(ctx context.Context, text string) (Result, error) {
    ctx, cancel := context.WithTimeout(ctx, c.httpClient.Timeout)
    defer cancel()

    ctx, span := c.obs.Tracer("tokenizer").Start(ctx, "tokenizer.tokenize")
    defer span.End()

    started := time.Now()
    var outcome string
    defer func() {
        c.emitObs(ctx, outcome, time.Since(started), len(text), /*...*/)
    }()

    body, err := json.Marshal(buildPayload(ctx, text))
    if err != nil {
        outcome = "error_invalid"
        return Result{}, fmt.Errorf("tokenizer: marshal: %w", err)
    }

    var resp Result
    err = withRetry(ctx, 2, func() error {
        return c.doOnce(ctx, body, &resp)
    })
    switch {
    case errors.Is(err, context.DeadlineExceeded):
        outcome = "error_timeout"
    case errors.Is(err, ErrSidecarUnreachable):
        outcome = "error_unreachable"
    case err == nil:
        outcome = "success"
    default:
        outcome = "error_invalid"
    }
    return resp, err
}
```

### 5.4 Compose Service Entry

```yaml
tokenizer-ko:
  build:
    context: ../services/tokenizer-ko
  ports:
    - "${TOKENIZER_KO_PORT:-8083}:8083"
  environment:
    TOKENIZER_KO_PORT: "8083"
    TOKENIZER_KO_LOG_LEVEL: INFO
  healthcheck:
    test: ["CMD", "curl", "-f", "http://localhost:8083/health"]
    interval: 30s
    timeout: 5s
    retries: 3
    start_period: 20s
  restart: unless-stopped
  networks:
    - app
```

Note: no `depends_on` — `tokenizer-ko` is a leaf service (no upstream
deps; mecab-ko-dic is bundled in the image).

### 5.5 RRF Merge Sketch

```go
// internal/index/router/merge.go (sketch — final shape in run phase)

const RRFK = 60

func MergeRRF(perShard map[Shard][]types.NormalizedDoc, maxResults int) []types.NormalizedDoc {
    scores := map[string]float64{}     // canonicalHash -> RRF score
    docs := map[string]types.NormalizedDoc{}
    insertionOrder := map[string]int{}
    nextOrder := 0

    for _, results := range perShard {
        for rank, d := range results {
            h := d.CanonicalHash()
            scores[h] += 1.0 / float64(RRFK + rank + 1)
            if _, exists := docs[h]; !exists {
                docs[h] = d
                insertionOrder[h] = nextOrder
                nextOrder++
            }
        }
    }

    type scoredDoc struct {
        hash  string
        score float64
        order int
    }
    sorted := make([]scoredDoc, 0, len(scores))
    for h, s := range scores {
        sorted = append(sorted, scoredDoc{h, s, insertionOrder[h]})
    }
    sort.SliceStable(sorted, func(i, j int) bool {
        if sorted[i].score == sorted[j].score {
            return sorted[i].order < sorted[j].order
        }
        return sorted[i].score > sorted[j].score
    })

    out := make([]types.NormalizedDoc, 0, min(len(sorted), maxResults))
    for _, sd := range sorted {
        if len(out) >= maxResults {
            break
        }
        out = append(out, docs[sd.hash])
    }
    return out
}
```

### 5.6 Outcome Enumeration

`TokenizerCalls` outcome label values (4):

| Outcome             | Triggered by |
|---------------------|--------------|
| `success`           | 200 response, valid body |
| `error_invalid`     | 400 response (empty/oversize input) |
| `error_unreachable` | Connection refused / 5xx after retry |
| `error_timeout`     | Context deadline exceeded |

`IndexShardWrites` label values:
- `shard ∈ {ko, default}` (NEW label name, requires allowlist amendment)
- `outcome ∈ {success, fallback, error}`

Exactly 6 cardinality cells; static, bounded.

### 5.7 MX Tag Plan

| File | Tag | Reason |
|------|-----|--------|
| `internal/index/router/shard.go::IndexShardForDoc` | @MX:ANCHOR | fan_in ≥ 3 expected (indexer + tests + future SPEC-IDX-001 consumer). @MX:REASON: sole sanctioned index-time routing decision |
| `internal/index/router/shard.go::QueryShardsForText` | @MX:ANCHOR | fan_in ≥ 3 expected. @MX:REASON: sole sanctioned query-time shard router |
| `internal/index/router/merge.go::MergeRRF` | @MX:ANCHOR | fan_in ≥ 2 (router + future fanout); @MX:REASON: cross-shard merge contract |
| `internal/index/tokenizer/client.go::Client.Tokenize` | @MX:ANCHOR | fan_in ≥ 3 (indexer fanout, tests, future re-tokenization job). @MX:REASON: sole sidecar entry |
| `internal/index/router/shard.go::ratioHigh, ratioLow` | @MX:NOTE | Magic constants 0.30 / 0.10 reused from SPEC-IR-001 |
| `internal/index/tokenizer/client.go::doOnce` | @MX:WARN | retry path silently degrades on connection error → fallback; @MX:REASON: caller may not observe sidecar failure unless they inspect Metadata["tokenizer"] |
| `services/tokenizer-ko/src/tokenizer_ko/tokenize.py::tokenize` | @MX:NOTE (Python) | asyncio.Lock serialization pattern; pymecab-ko thread-unsafety justification |

Per `.claude/rules/moai/workflow/mx-tag-protocol.md`: `[AUTO]` prefix on
agent-generated tags; `@MX:REASON` mandatory for ANCHOR + WARN;
`@MX:SPEC: SPEC-IDX-003` on all tags. Per
`.moai/config/sections/language.yaml` (`code_comments: en`), all @MX
descriptions in English.

### 5.8 Harness Level

Per `.moai/config/sections/harness.yaml` auto-routing: 13 REQs (10 P0 +
2 P1 + 1 P2) + 7 NFRs touching 2 packages (Python sidecar + Go-side
multi-package — `internal/index/tokenizer/`, `internal/index/router/`,
`internal/index/meili/`) + 1 new compose service + 2 new metric
families + 1 cardinality allowlist amendment + cross-language
integration = **standard** harness with thorough recommendation.
Sprint Contract recommended.

---

## 6. File Impact

### 6.1 Created (24 files)

| Path | Purpose |
|------|---------|
| `services/tokenizer-ko/src/tokenizer_ko/__init__.py` | Package doc + `__version__` |
| `services/tokenizer-ko/src/tokenizer_ko/__main__.py` | Uvicorn entrypoint |
| `services/tokenizer-ko/src/tokenizer_ko/app.py` | FastAPI app + lifespan + routes (REQ-IDX-003-001/003/004) |
| `services/tokenizer-ko/src/tokenizer_ko/models.py` | Pydantic v2 models (REQ-IDX-003-001) |
| `services/tokenizer-ko/src/tokenizer_ko/tokenize.py` | mecab-ko wrapper (REQ-IDX-003-002) |
| `services/tokenizer-ko/src/tokenizer_ko/obs.py` | JSON log records (REQ-IDX-003-009) |
| `services/tokenizer-ko/pyproject.toml` | Runtime deps + pytest config |
| `services/tokenizer-ko/Dockerfile` | Multi-stage Python 3.11-slim build |
| `services/tokenizer-ko/.env.example` | Service-local env template |
| `services/tokenizer-ko/README.md` | Service overview + run instructions |
| `services/tokenizer-ko/tests/test_app.py` | Endpoint contract tests |
| `services/tokenizer-ko/tests/test_tokenize.py` | Tokenization correctness tests |
| `services/tokenizer-ko/tests/test_obs.py` | Log shape tests |
| `services/tokenizer-ko/tests/test_throughput.py` | Slow-marked NFR-IDX-003-001 |
| `services/tokenizer-ko/tests/fixtures/golden_morphemes.json` | ≥ 30 Korean strings + expected morphemes |
| `internal/index/tokenizer/types.go` | Go value types + sentinels |
| `internal/index/tokenizer/config.go` | Env binding |
| `internal/index/tokenizer/client.go` | Go HTTP client (REQ-IDX-003-005/008/009) |
| `internal/index/tokenizer/client_test.go` | Go client RED tests |
| `internal/index/tokenizer/main_test.go` | goleak setup |
| `internal/index/tokenizer/bench_test.go` | NFR-IDX-003-003 |
| `internal/index/router/shard.go` | Index + query routing (REQ-IDX-003-005/006/012) |
| `internal/index/router/shard_test.go` | Routing tests |
| `internal/index/router/merge.go` | RRF merge (REQ-IDX-003-007) |
| `internal/index/router/merge_test.go` | RRF tests |
| `internal/index/router/router_test.go` | Integration + NFR-IDX-003-004 golden set |
| `internal/index/router/main_test.go` | goleak setup |
| `internal/index/router/bench_test.go` | NFR-IDX-003-005 |
| `internal/index/router/testdata/korean_golden.json` | 5 docs + 3 queries fixture |
| `internal/index/meili/korean_shard.go` | Meili settings PATCH (REQ-IDX-003-011, §2.1(l)) |
| `internal/index/meili/korean_shard_test.go` | Settings probe tests |
| `internal/obs/metrics/tokenizer.go` | New metric family |

### 6.2 Modified (6 files)

| Path | Change |
|------|--------|
| `internal/router/korean.go` | Export `RatioHigh = 0.30` and `RatioLow = 0.10` constants if not exported (consumed by `internal/index/router/shard.go`) |
| `internal/obs/metrics/metrics.go` | Register tokenizer collectors in `NewRegistry()`; amend `labelNames` allowlist to include `shard` (NEW label name — see §6.4) |
| `internal/obs/obs.go` | Add `TokenizerCalls`, `TokenizerLatency`, `IndexShardWrites` re-exports |
| `internal/index/index.go` | Replace stub with package doc + ANCHOR + re-exports |
| `deploy/docker-compose.yml` | Add `tokenizer-ko` service per §5.4 |
| `.env.example` | Append `TOKENIZER_KO_*` env vars |

### 6.3 Unchanged (by design)

- `internal/router/*` — no API change; SPEC-IDX-003 IMPORTS the package
  for `HangulRatio`, `RatioHigh`, `RatioLow`. The `Router` struct is
  not consumed.
- `internal/llm/*`, `internal/synthesis/*` — not in tokenization path.
- `pkg/types/*` — `NormalizedDoc.Lang` is the contract, unchanged.
- `services/researcher/*` — synthesis sidecar; independent of tokenization.
- `deploy/litellm/config.yaml` — no LLM dependency.

### 6.4 Cardinality Allowlist Amendment

[HARD] SPEC-IDX-003 amends the SPEC-OBS-001 cardinality allowlist
(`internal/obs/metrics/metrics.go:147-154` — exact line numbers verified
at run-phase) to include the new label name `shard`.

The amendment:
- Adds `"shard"` to the existing `labelNames` allow-list slice.
- Adds a comment block documenting that `shard` has exactly 2 values
  (`ko`, `default`) — bounded cardinality.
- Updates the test `TestNoUnboundedLabels` (or equivalent SPEC-OBS-001
  assertion) to accept the new label.

The amendment is in scope of SPEC-IDX-003 because:
- The new metric `IndexShardWrites{shard, outcome}` cannot exist without it.
- The SPEC-OBS-001 cardinality discipline test would otherwise fail.
- SPEC-OBS-001 documents the expectation that downstream SPECs may
  request allowlist amendments with bounded enumerations.

The `shard` label name is reserved exclusively for `IndexShardWrites`.
Adding `shard` to other collectors is OUT OF SCOPE.

---

## 7. Test Plan

Development follows RED-GREEN-REFACTOR per `quality.development_mode:
tdd`. Coverage target 85% per SPEC frontmatter.

Representative RED-phase tests (in addition to the acceptance criteria
already enumerated in §4):

| # | Test | Layer | REQ | Assertion |
|---|------|-------|-----|-----------|
| 1 | `test_tokenize_happy_path` | `tests/test_app.py` | REQ-IDX-003-001 | 200 + valid response shape |
| 2 | `test_tokenize_extra_field_rejected` | `tests/test_app.py` | REQ-IDX-003-001 | 422 on unknown field |
| 3 | `test_joined_equals_space_join_of_tokens` | `tests/test_app.py` | REQ-IDX-003-001 | invariant |
| 4 | `test_tokenize_korean_morphemes` | `tests/test_tokenize.py` | REQ-IDX-003-002 | golden morphemes |
| 5 | `test_concurrent_tokenize_safe` | `tests/test_app.py` | REQ-IDX-003-002 | 50 concurrent OK |
| 6 | `test_eos_lines_excluded` | `tests/test_tokenize.py` | REQ-IDX-003-002 | EOS not in tokens |
| 7 | `test_lifespan_raises_when_dict_missing` | `tests/test_app.py` | REQ-IDX-003-003 | startup raises |
| 8 | `test_health_returns_503_when_tagger_unhealthy` | `tests/test_app.py` | REQ-IDX-003-003 | 503 + degraded reason |
| 9 | `test_empty_text_returns_400` | `tests/test_app.py` | REQ-IDX-003-004 | 400 + error=invalid_input |
| 10 | `test_oversize_input_returns_400` | `tests/test_app.py` | REQ-IDX-003-004 | 400 + detail=size |
| 11 | `test_python_log_record_shape` | `tests/test_obs.py` | REQ-IDX-003-009 | 1 JSON log per call, 5 attrs |
| 12 | `test_python_log_no_pii` | `tests/test_obs.py` | REQ-IDX-003-009 | text not in log |
| 13 | `TestIndexerDefaultShardEnglish` | `internal/index/router/router_test.go` | REQ-IDX-003-005 | english → usearch only |
| 14 | `TestIndexerKoreanShardLang` | `internal/index/router/router_test.go` | REQ-IDX-003-005 | korean → usearch-ko |
| 15 | `TestIndexerDualWriteHybridDoc` | `internal/index/router/router_test.go` | REQ-IDX-003-005 | dual write |
| 16 | `TestIndexerCallsTokenizerForKoreanShard` | `internal/index/router/router_test.go` | REQ-IDX-003-005 | sidecar called |
| 17 | `TestIndexerOriginalAndTokenizedFieldsDistinct` | `internal/index/router/router_test.go` | REQ-IDX-003-005 | distinct content |
| 18 | `TestQueryShardSelection` | `internal/index/router/shard_test.go` | REQ-IDX-003-006 | table-driven |
| 19 | `TestQueryFanoutIssuesParallelMeiliCalls` | `internal/index/router/router_test.go` | REQ-IDX-003-006 | parallel timing |
| 20 | `TestQueryDoesNotPreTokenize` | `internal/index/router/router_test.go` | REQ-IDX-003-006 | verbatim text |
| 21 | `TestMergeRRFTwoShards` | `internal/index/router/merge_test.go` | REQ-IDX-003-007 | hand-computed RRF |
| 22 | `TestMergeDedupByCanonicalHash` | `internal/index/router/merge_test.go` | REQ-IDX-003-007 | dedup |
| 23 | `TestMergeRespectsMaxResults` | `internal/index/router/merge_test.go` | REQ-IDX-003-007 | bounded |
| 24 | `TestMergeStableForTies` | `internal/index/router/merge_test.go` | REQ-IDX-003-007 | deterministic order |
| 25 | `TestIndexerDegradedSidecarFallback` | `internal/index/router/router_test.go` | REQ-IDX-003-008 | 503 → lindera_fallback |
| 26 | `TestIndexerDegradedSidecarConnRefused` | `internal/index/router/router_test.go` | REQ-IDX-003-008 | conn refused → fallback |
| 27 | `TestIndexerDegradedSidecarTimeout` | `internal/index/router/router_test.go` | REQ-IDX-003-008 | timeout → fallback |
| 28 | `TestClientEmitsCounter` | `internal/index/tokenizer/client_test.go` | REQ-IDX-003-009 | 4 outcome enums |
| 29 | `TestClientEmitsHistogram` | `internal/index/tokenizer/client_test.go` | REQ-IDX-003-009 | count == 1, sum > 0 |
| 30 | `TestClientEmitsOTelSpan` | `internal/index/tokenizer/client_test.go` | REQ-IDX-003-009 | span attrs |
| 31 | `TestClientObservabilitySafeOnNilObs` | `internal/index/tokenizer/client_test.go` | REQ-IDX-003-009 | no panic |
| 32 | `TestEmitIndexShardWrites` | `internal/index/router/router_test.go` | REQ-IDX-003-010 | 6 cells |
| 33 | `TestIndexShardWritesCardinality` | `internal/index/router/router_test.go` | REQ-IDX-003-010 | exactly 6 unique combos |
| 34 | `TestMeiliSettingsAcceptsKor` | `internal/index/meili/korean_shard_test.go` | REQ-IDX-003-011 | kor accepted |
| 35 | `TestMeiliSettingsFallsBackToKo` | `internal/index/meili/korean_shard_test.go` | REQ-IDX-003-011 | kor → ko fallback |
| 36 | `TestMeiliSettingsTolerantOfBoth` | `internal/index/meili/korean_shard_test.go` | REQ-IDX-003-011 | both rejected → WARN |
| 37 | `TestQueryDoesNotFilterByLangField` | `internal/index/router/router_test.go` | REQ-IDX-003-012 | Lang="" still queryable |
| 38 | `TestKoreanQueryReturnsAtLeastOneKoreanDoc` | `internal/index/router/router_test.go` | REQ-IDX-003-013 | golden ≥1 match |
| 39 | `TestKoreanQueryFiltersByContent` | `internal/index/router/router_test.go` | REQ-IDX-003-013 | English query → 0 from ko shard |
| 40 | `test_throughput_1000rps` | `tests/test_throughput.py` (slow) | NFR-IDX-003-001 | ≥ 1000 RPS |
| 41 | `test_tokenize_p50_latency_under_5ms` | `tests/test_app.py` (slow) | NFR-IDX-003-002 | p50 ≤ 5 ms |
| 42 | `BenchmarkBatchTokenize100Docs` | `internal/index/tokenizer/bench_test.go` | NFR-IDX-003-003 | p50 ≤ 50 ms |
| 43 | `TestKoreanFirstGoldenSet` | `internal/index/router/router_test.go` | NFR-IDX-003-004 | ≥1 match per query |
| 44 | `BenchmarkCrossShardQuery` | `internal/index/router/bench_test.go` | NFR-IDX-003-005 | p95 ≤ 200 ms |
| 45 | `TestMain` (goleak) | `internal/index/tokenizer/main_test.go` | NFR-IDX-003-006 | no leaks |
| 46 | `TestMain` (goleak) | `internal/index/router/main_test.go` | NFR-IDX-003-006 | no leaks |

Python: `pytest -q services/tokenizer-ko/tests/` with
`asyncio_mode="auto"`. Coverage via `pytest --cov=tokenizer_ko
--cov-report=term-missing`, target 85%.

Go: `go test -race ./internal/index/...` against `httptest.NewServer`
returning canned JSON. Coverage via `go test -coverprofile=...`,
target 85%.

Brownfield note: `internal/index/index.go` is a 4-line stub. Per
`workflow-modes.md` §Brownfield Enhancement, no characterization tests
needed; RED tests are written against the planned package surface.

---

## 8. Dependencies

### 8.1 Upstream SPEC Dependencies

- **SPEC-BOOT-001 (implemented)**: provides `services/` workspace,
  `deploy/docker-compose.yml` with Meili service, `internal/index/`
  4-line stub, `services/tokenizer-ko/` directory will be created
  following the pattern.
- **SPEC-IDX-001 (in flight, parallel SPEC)**: provides the Go-side
  Meilisearch client (`internal/index/meili/client.go` or equivalent)
  that SPEC-IDX-003 calls via `EnsureKoreanIndexSettings`. SPEC-IDX-001
  delivers the default `usearch` index; SPEC-IDX-003 layers on the
  `usearch-ko` shard via the same client. SOFT dep — SPEC-IDX-003 spec
  can be drafted without SPEC-IDX-001 spec being final, but
  run-phase implementation requires SPEC-IDX-001's Meili client API
  to exist.

### 8.2 Coordinating SPECs (no hard dependency)

- **SPEC-CORE-001 (implemented)**: `pkg/types.NormalizedDoc.Lang`
  field. Unchanged.
- **SPEC-IR-001 (implemented)**: `internal/router.HangulRatio` /
  `RatioHigh` / `RatioLow`. SPEC-IDX-003 imports; minor edit to
  `internal/router/korean.go` to export the constants if they aren't
  already (currently package-level constants per `rules.go`; export
  needed).
- **SPEC-OBS-001 (implemented)**: provides `obs.Logger`, `obs.Tracer`,
  metric registry, and `internal/obs/metrics/` location. SPEC-IDX-003
  adds one new metric file under `internal/obs/metrics/tokenizer.go`
  and amends the cardinality allowlist (§6.4).
- **SPEC-SYN-001 (implemented)**: Python sidecar pattern reference.
  No code dependency; SPEC-IDX-003 mirrors the layout.

### 8.3 Downstream Blocked SPECs

- **SPEC-EVAL-003 (M8)**: Korean-locale benchmark. Cannot measure
  Korean retrieval precision without working Korean indexing, which
  SPEC-IDX-003 delivers.
- **SPEC-IDX-004 (M6)**: multi-tenant Korean shard handling. The
  `usearch-ko` index is a precondition for per-team `usearch-ko-{team}`
  variants.

### 8.4 External Dependencies (run-phase pins)

New Python runtime dependencies (`services/tokenizer-ko/pyproject.toml`):

```
fastapi >= 0.115
uvicorn[standard] >= 0.30
pydantic >= 2.9
mecab-ko >= 1.0, < 2.0     # pymecab-ko (PyPI name "mecab-ko"); bundles mecab-ko-dic
```

No new Go module dependencies (uses stdlib `net/http`,
`encoding/json`, `sort`, `context`, `time`, `errors`).

No new external services beyond the new `tokenizer-ko` compose entry.
The existing Meilisearch service (SPEC-BOOT-001) handles both shards.

---

## 9. Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| pymecab-ko native install fails in `python:3.11-slim` Docker without `build-essential` | High | High | Dockerfile installs `apt-get install -y --no-install-recommends build-essential` in builder stage; pymecab-ko ships pre-built wheels for x86_64 manylinux (verified §2.1 — pip install works without source build on Linux). Multi-stage build keeps final image lean. |
| pymecab-ko Tagger thread-unsafety causes data corruption under parallel asyncio | Medium | High | asyncio.Lock serializes parse calls (§5.1); concurrent test (REQ-IDX-003-002 acceptance) verifies. Trade-off: caps single-worker throughput at ~10K-20K ops/sec; NFR-IDX-003-001 target is 1K, 10× headroom. |
| Lindera fallback (Path A) recall is materially worse than Path B | High | Medium | NFR-IDX-003-004 golden set + REQ-IDX-003-013 contract guarantee at least one match — but the SPEC accepts that fallback mode degrades quality. Document in `Metadata["tokenizer"]` field for ops awareness. SPEC-EVAL-003 (M8) measures the gap. |
| Meili `localizedAttributes` API rejects both `kor` and `ko` | Low | Low | REQ-IDX-003-011 fallback path 3: omit `localizedAttributes` entirely; pre-tokenized Korean content is still searchable via Meili default tokenizer applied to space-separated morphemes. |
| `tokenizer-ko` sidecar OOM on large doc batch | Medium | Medium | NFR-IDX-003-001 throughput test bounds memory; container limit `mem_limit: 1g` documented in `services/tokenizer-ko/README.md`. Streaming / batching deferred (§2.2 exclusions). |
| Cardinality allowlist amendment gets reverted by an OBS-001-aware refactor | Low | Medium | §6.4 amendment includes a NOTE comment in `metrics.go` explaining `shard` is bounded; SPEC-IDX-003's `TestIndexShardWritesCardinality` test catches drift. |
| Pre-tokenized Korean text loses Meili's prefix-search ability | Medium | Low | Meili supports prefix search on whitespace-separated tokens; pre-tokenized morphemes are space-separated, so prefix search continues to work at the morpheme level (not at the syllable level — but for Korean, morpheme-level prefix is the right granularity). Verified at run-phase. |
| Defensive dual-write (Lang=ko AND HangulRatio<0.10) doubles storage for hybrid docs | Low | Low | Bounded by adapter behavior — only Korean adapters with English-only content trigger this. Estimated < 5% of total Korean-source corpus. Acceptable cost for findability. |
| mecab-ko-dic version drift between pymecab-ko releases changes morpheme boundaries | Medium | Medium | §6.1 upgrade SOP requires golden-set integration test on every dictionary version bump. Version reported in `/health` for ops dashboards. |
| Cross-shard RRF merge eliminates legitimately-relevant docs from one shard | Low | Medium | NFR-IDX-003-004 golden set verifies recall; if observed in measurement, switch to score-normalized fusion (Open Question §11.6). |
| HangulRatio threshold tuning (0.30 / 0.10) misclassifies real-world Korean queries | Medium | Medium | Reused from SPEC-IR-001 with proven golden fixtures. SPEC-EVAL-003 measures; thresholds are package constants tunable via SPEC amendment. |
| Sidecar healthcheck `start_period: 20s` insufficient for cold start on slow hosts | Low | Low | Document in `services/tokenizer-ko/README.md`; tune up to `start_period: 60s` if observed. |

---

## 10. Open Questions

The following are explicitly unresolved at SPEC-approval time and
documented in research §8 rather than pre-decided. They do not block
SPEC approval. Restated here for SPEC consumers:

1. **Locale code for `localizedAttributes`** — `kor` (ISO 639-3) vs
   `ko` (ISO 639-1). REQ-IDX-003-011 codifies the run-phase probe.

2. **mecab-ko-dic dictionary version** — pymecab-ko bundles a fixed
   version. Upgrade is a future-SPEC concern.

3. **Custom user dictionary** — deferred to future SPEC.

4. **Sidecar workers count** — V1 `--workers 1`; revisit if
   throughput insufficient.

5. **Synonym handling** — deferred to future SPEC.

6. **Cross-shard score normalization** — V1 pure rank-based RRF;
   revisit post-M3.

7. **Re-tokenization on dictionary upgrade** — V1 manual full re-index;
   future SPEC may incrementalize.

8. **Health-check upstream Meili reachability** — V1 tokenizer
   `/health` does NOT probe Meili.

9. **Defensive routing threshold** — 0.10 reuse from IR-001; revisit
   post-M3 traffic measurement.

10. **Hangul Jamo Extended-B (U+D7B0–D7FF)** — NOT in IR-001's range;
    out of scope V1.

11. **Locale code for Meili — `kor` vs `ko`** (restated for emphasis).
    REQ-IDX-003-011 specifies the fallback chain.

---

## 11. References

### Internal

- `pkg/types/normalized_doc.go:51` — `Lang` field (BCP-47 routing key).
- `internal/router/korean.go:18` — Korean particle list (reused for
  Meili stop-words).
- `internal/router/korean.go:27-39` — `isHangulRune` block ranges.
- `internal/router/korean.go:46-64` — `HangulRatio` (consumed by
  query-time routing).
- `internal/router/korean.go:70-85` — `ParticleDensity`.
- `internal/router/korean.go:89-91` — `KoreanSignals`.
- `internal/router/rules.go` — IR-001 ratio thresholds reused.
- `internal/synthesis/client.go:230-252` — observability emit pattern
  to mirror.
- `internal/synthesis/types.go` — Go-side request/response pattern.
- `internal/llm/client.go:244-251` — nil-safe obs pattern.
- `internal/obs/metrics/router.go`, `internal/obs/metrics/synthesis.go`
  — precedent for new metric family file.
- `internal/obs/metrics/metrics.go:147-154` — cardinality allowlist
  (amended by §6.4).
- `services/researcher/Dockerfile:1-29` — Python sidecar Dockerfile
  pattern.
- `services/researcher/src/researcher/app.py:27-46` — FastAPI lifespan
  pattern.
- `deploy/docker-compose.yml:48-66` — Meilisearch service entry.
- `deploy/docker-compose.yml:165-188` — researcher compose pattern.
- `internal/index/index.go` — current 4-line stub.
- `.moai/specs/SPEC-CORE-001/spec.md` — `NormalizedDoc.Lang` semantics.
- `.moai/specs/SPEC-IR-001/spec.md` — Hangul detection; ratio
  thresholds.
- `.moai/specs/SPEC-SYN-001/spec.md` — Python sidecar architectural
  pattern.
- `.moai/specs/SPEC-OBS-001/spec.md` — observability + cardinality
  discipline.
- `.moai/specs/SPEC-BOOT-001/spec.md` — services/ workspace + compose.
- `.moai/specs/SPEC-IDX-003/research.md` — companion research artifact.
- `.moai/project/roadmap.md:57` — SPEC-IDX-003 row.
- `.moai/project/roadmap.md:150` — M3 exit criterion.
- `.moai/project/tech.md:50` — Korean tokenizer = mecab-ko.
- `.moai/project/tech.md:141` — RRF formula `k=60`.
- `.moai/project/tech.md:167` — Decision Log: mecab-ko sidecar.
- `.claude/rules/moai/languages/python.md` — Pydantic v2 conventions.
- `.claude/rules/moai/languages/go.md` — Go 1.23+ conventions.
- `.claude/rules/moai/workflow/mx-tag-protocol.md` — @MX tag rules.

### External (verified via WebFetch 2026-05-04)

- `https://www.meilisearch.com/docs/reference/api/settings` — Per-index
  settings: `stopWords`, `separatorTokens`, `nonSeparatorTokens`,
  `dictionary`, `localizedAttributes`. PATCH `/indexes/{uid}/settings`.
- `https://github.com/meilisearch/charabia` — Charabia v0.9.9
  (2025-11-24); Korean via lindera + KO-dict; ~2 MiB/sec throughput.
- `https://github.com/lindera/lindera` — Lindera v3.0.7 (2026-04-24);
  lindera-ko-dic dictionary support.
- `https://github.com/meilisearch/meilisearch` — Meilisearch v1.43.0
  (2026-05-04 stable).
- `https://github.com/SamuraiT/mecab-python3` — Japanese-only;
  recommends pymecab-ko for Korean.
- `https://github.com/NoUnique/pymecab-ko` — pymecab-ko v1.0.2
  (2025-09-23); Python 3.6+; mecab-ko-dic bundled; pip install
  mecab-ko.
- `https://en.wikipedia.org/wiki/Hangul_Syllables` — Hangul Unicode
  block ranges.

---

*End of SPEC-IDX-003 v0.1 (draft)*
