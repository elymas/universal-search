# SPEC-IDX-003 Acceptance Scenarios

Generated: 2026-05-26 (reverse-engineered from implemented code)
Format: Given / When / Then with verifying test references.
Status: SPEC implemented; scenarios reflect realised behaviour.

This document enumerates the testable acceptance criteria for
SPEC-IDX-003 (Korean tokenization via mecab-ko sidecar + dual-shard
Meili + Go-side routing). Verifying tests live under
`services/tokenizer-ko/tests/`, `internal/index/tokenizer/`,
`internal/index/router/`, and `internal/index/meili/`.

---

## AC-001 — `/tokenize` endpoint contract conforms to Pydantic v2 schema

**Coverage**: REQ-IDX-003-001 (Ubiquitous)

### Given

- The Python sidecar is running on port 8083 with the
  `pymecab_ko.Tagger` loaded.

### When

`POST /tokenize` is invoked with `{"request_id":"r1","text":"안녕하세요"}`.

### Then

- HTTP 200 returned with body matching `TokenizeResponse{request_id,
  tokens, joined, morpheme_count, latency_ms, dict_version}`.
- `joined == " ".join(tokens)` (invariant).
- `morpheme_count == len(tokens)`.
- `dict_version` is a non-empty string.
- `latency_ms >= 0`.

### Boundary: Extra field rejected

POST with `{..., "unexpected": 1}` returns HTTP 422 (Pydantic
validation).

**Verifying tests**: `test_tokenize_happy_path`,
`test_tokenize_extra_field_rejected`,
`test_joined_equals_space_join_of_tokens`,
`test_tokenize_response_shape_matches_schema` in
`services/tokenizer-ko/tests/test_app.py`.

---

## AC-002 — mecab-ko produces expected morphemes from golden fixtures

**Coverage**: REQ-IDX-003-002 (Event-Driven)

### Given

- Sidecar running with mecab-ko-dic loaded.
- A golden-morpheme fixture file
  `services/tokenizer-ko/tests/fixtures/golden_morphemes.json`
  containing ≥ 30 Korean strings with expected morpheme arrays.

### When

`POST /tokenize` is invoked with `text: "ChatGPT 사용법"`.

### Then

- `tokens` contains at least 3 morphemes whose surface forms cover the
  input characters (exact split fixture-driven).
- `morpheme_count == len(tokens)`.
- `EOS` line is NOT in the returned tokens.

### Boundary: Concurrent requests are consistent

50 concurrent `POST /tokenize` requests for the same input via
`asyncio.gather` return identical tokens (proves `asyncio.Lock` correctly
serializes Tagger access).

**Verifying tests**: `test_tokenize_korean_morphemes`,
`test_morpheme_count_matches_tokens_length`,
`test_concurrent_tokenize_safe`, `test_eos_lines_excluded` in
`services/tokenizer-ko/tests/test_tokenize.py`.

---

## AC-003 — Lifespan raises when mecab-ko-dic load fails

**Coverage**: REQ-IDX-003-003 (State-Driven)

### Given

- `pymecab_ko.Tagger.__init__` patched to raise `RuntimeError("dict
  load failed")`.

### When

The FastAPI lifespan startup hook runs.

### Then

- Lifespan raises before the app accepts requests.
- Container HEALTHCHECK fails.
- Docker-compose `restart: unless-stopped` policy attempts restart.

### Boundary: Degraded health when Tagger init catches but leaves None

When Tagger init is caught and `tagger` is left `None`, `/health`
returns 503 with `{"status":"degraded","reason":"<msg>"}`.

**Verifying tests**: `test_lifespan_raises_when_dict_missing`,
`test_health_returns_503_when_tagger_unhealthy` in
`services/tokenizer-ko/tests/test_app.py`.

---

## AC-004 — Empty or oversize input is rejected

**Coverage**: REQ-IDX-003-004 (Unwanted)

### Given

- Sidecar running with Tagger loaded.

### When

1. `POST /tokenize` with `text: "   "` (whitespace stripped to empty).
2. `POST /tokenize` with `text` of 65,537 bytes (over
   `MAX_INPUT_BYTES=65536`).

### Then

- Case 1: HTTP 400 + body `{"error":"invalid_input","detail":"text"}`.
- Case 2: HTTP 400 + body `{"error":"invalid_input","detail":"size"}`.
- No Tagger.parse call for either case (verified via mock).
- ONE WARN-level structured log record per failure with `{request_id,
  error}`.
- Go-side counter `usearch_tokenizer_calls_total{outcome="error_invalid"}`
  incremented once.

**Verifying tests**: `test_empty_text_returns_400`,
`test_oversize_input_returns_400`, `test_invalid_input_no_tagger_call`,
`test_invalid_input_logs_warn` in
`services/tokenizer-ko/tests/test_app.py`.

---

## AC-005 — Index-time routing writes Korean docs to `usearch_docs_ko` only

**Coverage**: REQ-IDX-003-005 (Event-Driven)

### Given

- Three docs:
  - `D1: {Lang: "en", Body: "hello world"}` (English).
  - `D2: {Lang: "ko", Body: "안녕하세요"}` (Korean high-Hangul).
  - `D3: {Lang: "ko", Body: "buy iphone 16 pro"}` (Korean source, English
    body — hybrid).

### When

`IndexShardForDoc(d)` is called for each, then `Upsert` dispatches.

### Then

- `D1` → `[ShardDefault]` → written to `usearch_docs` only; tokenizer
  sidecar NOT called.
- `D2` → `[ShardKo]` → written to `usearch_docs_ko` only; tokenizer
  sidecar called once; indexed doc has tokenized fields (e.g.,
  `Body: "안녕 하세요"`).
- `D3` → `[ShardKo, ShardDefault]` (dual-write) → original doc to
  `usearch_docs`, tokenized doc to `usearch_docs_ko`.
- For dual-written `D3`, the `usearch_docs` copy has `Body: "buy iphone
  16 pro"` (original); the `usearch_docs_ko` copy has space-joined
  morphemes from the sidecar.

**Verifying tests**: `TestIndexerDefaultShardEnglish`,
`TestIndexerKoreanShardLang`, `TestIndexerDualWriteHybridDoc`,
`TestIndexerCallsTokenizerForKoreanShard`,
`TestIndexerSkipsTokenizerForDefaultShard`,
`TestIndexerOriginalAndTokenizedFieldsDistinct` in
`internal/index/router/shard_test.go`.

---

## AC-006 — Query-time routing dispatches to the correct shard(s)

**Coverage**: REQ-IDX-003-006 (Event-Driven)

### Given

- Eight query strings covering the §2.3 query-table cases.

### When

`QueryShardsForText(text)` is called for each.

### Then

| `text` | HangulRatio | Returned shards |
|--------|-------------|-----------------|
| `"hello world"` | 0 | `[ShardDefault]` |
| `"안녕하세요"` | 1.0 | `[ShardKo]` |
| `"best Korean LLM 모델"` | ~0.20 (ambiguous) | `[ShardKo, ShardDefault]` |
| `""` | 0 (degenerate) | `[ShardDefault]` |
| `"A"` | 0 | `[ShardDefault]` |
| `"가나다 hello world test"` | ~0.19 (ambiguous) | `[ShardKo, ShardDefault]` |
| `"가나다라마"` | 1.0 | `[ShardKo]` |
| `"hello 안녕"` | 0.5 | `[ShardKo]` |

### Boundary: Parallel fanout for 2-shard cases

When `QueryShardsForText` returns 2 shards, the orchestrator issues
parallel Meili calls (overlapping execution windows ≤ 5ms apart in the
test stub).

### Boundary: Query text not pre-tokenized

The Meili stub records the query body; assertion: it equals the input
text verbatim (no client-side tokenization).

**Verifying tests**: `TestQueryShardSelection` (table-driven),
`TestQueryFanoutIssuesParallelMeiliCalls`,
`TestQueryDoesNotPreTokenize` in
`internal/index/router/shard_test.go`.

---

## AC-007 — RRF merge across two shards is deterministic

**Coverage**: REQ-IDX-003-007 (Ubiquitous)

### Given

- Two ranked result lists:
  - `usearch_docs`: `[docA, docB, docC]`.
  - `usearch_docs_ko`: `[docB, docD, docE]`.

### When

`MergeRRF(results, k=60)` is invoked.

### Then

Hand-computed RRF scores:

- `docA`: rank 0 in shard1 → `1/(60+1) = 1/61 ≈ 0.01639`.
- `docB`: rank 1 in shard1 + rank 0 in shard2 → `1/62 + 1/61 ≈ 0.03253`.
- `docC`: rank 2 in shard1 → `1/63 ≈ 0.01587`.
- `docD`: rank 1 in shard2 → `1/62 ≈ 0.01613`.
- `docE`: rank 2 in shard2 → `1/63 ≈ 0.01587`.

Sorted output: `[docB, docA ≈ docD, docC ≈ docE]`.

### Boundary: Dedup by canonical hash

Same `NormalizedDoc.CanonicalHash()` appearing in both shards is
counted once with the merged RRF score.

### Boundary: MaxResults clamp

100 docs across 2 shards with `MaxResults=10` returns ≤ 10 docs.

### Boundary: Stable tie-break

Docs with identical RRF scores preserve insertion order.

**Verifying tests**: `TestMergeRRFTwoShards`,
`TestMergeDedupByCanonicalHash`, `TestMergeRespectsMaxResults`,
`TestMergeStableForTies` in `internal/index/router/merge_test.go`.

---

## AC-008 — Sidecar unreachable triggers Lindera fallback

**Coverage**: REQ-IDX-003-008 (State-Driven)

### Given

- A Korean doc to be indexed; `httptest.NewServer` returns 503 on every
  `/tokenize` request.

### When

The indexer attempts to write the Korean doc to `usearch_docs_ko`.

### Then

- Doc is STILL written to `usearch_docs_ko` (with un-tokenized text;
  Meili's native Charabia/Lindera handles it as fallback).
- `doc.Metadata["tokenizer"] == "lindera_fallback"`.
- Go-side counter `usearch_tokenizer_calls_total{outcome="error_unreachable"}`
  incremented by 1.
- Total elapsed wall-clock ≤ 1.5 seconds (timeout + retries bounded).
- Ingestion pipeline NOT blocked (the next doc proceeds).

### Boundary: Connection refused

Stub server is closed; same assertions; outcome label remains
`error_unreachable`.

### Boundary: Timeout

Stub sleeps 5s; client timeout 1s; same assertions; total elapsed
≤ 1.5s.

**Verifying tests**: `TestIndexerDegradedSidecarFallback`,
`TestIndexerDegradedSidecarConnRefused`,
`TestIndexerDegradedSidecarTimeout` in
`internal/index/tokenizer/client_test.go`.

---

## AC-009 — Per-call observability emits once per invocation

**Coverage**: REQ-IDX-003-009 (Ubiquitous)

### Given

- Go client constructed with non-nil `*obs.Obs`.

### When

`client.Tokenize(ctx, text)` is invoked successfully.

### Then

- `obs.TokenizerCalls.WithLabelValues("success").Inc()` invoked once.
- `obs.TokenizerLatency.WithLabelValues("success").Observe(...)`
  invoked once.
- ONE OTel span `tokenizer.tokenize` created and ended with attributes
  `{request_id, text_len, morpheme_count, latency_ms, outcome}`.

### Boundary: Outcome labels cover the 4 cases

Each of `{success, error_invalid, error_unreachable, error_timeout}`
fires the matching code path; counter increments by 1 each.

### Boundary: Nil obs is safe

Construct Client with `obs: nil`; `Tokenize` does not panic; returns
valid `Result`.

### Boundary: No PII in Python log records

Python log records do NOT contain the request `text` value (only
`text_len`).

**Verifying tests**: `TestClientEmitsCounter`,
`TestClientEmitsHistogram`, `TestClientEmitsOTelSpan`,
`TestClientObservabilitySafeOnNilObs` (Go); `test_python_log_record_shape`,
`test_python_log_no_pii` (Python).

---

## AC-010 — Per-shard write counter cardinality bounded

**Coverage**: REQ-IDX-003-010 (Ubiquitous)

### Given

- All 6 combinations of `shard ∈ {ko, default}` × `outcome ∈ {success,
  fallback, error}`.

### When

The corresponding code paths fire.

### Then

- `obs.IndexShardWrites.WithLabelValues(shard, outcome).Inc()`
  incremented by 1 each.
- Exactly 6 unique `(shard, outcome)` label combinations are observed
  in the test registry (cardinality discipline per NFR-OBS-002).

**Verifying tests**: `TestEmitIndexShardWrites` (6 sub-tests),
`TestIndexShardWritesCardinality`.

---

## AC-011 — `localizedAttributes` probing with fallback

**Coverage**: REQ-IDX-003-011 (Optional)

### Given

- Meili stub configured to accept or reject `locales: ["kor"]`.

### When

`EnsureKoreanIndexSettings(ctx, client)` is invoked.

### Then

- Case A (`kor` accepted): one PATCH call with `locales: ["kor"]`.
- Case B (`kor` rejected, `ko` accepted): two PATCH calls; the second
  with `locales: ["ko"]`.
- Case C (both rejected): WARN log emitted + fallback to no
  `localizedAttributes` (only `stopWords` + `searchableAttributes`
  set); function still returns `nil`.

**Verifying tests**: `TestMeiliSettingsAcceptsKor`,
`TestMeiliSettingsFallsBackToKo`, `TestMeiliSettingsTolerantOfBoth`
in `internal/index/meili/korean_shard_test.go`.

---

## AC-012 — Query does NOT filter by Lang field at retrieval

**Coverage**: REQ-IDX-003-012 (Event-Driven)

### Given

- A Korean doc indexed to `usearch_docs_ko` with `Lang=""` (bypassing
  `IndexShardForDoc` via direct write fixture).

### When

A Korean query is issued via the Meili shard.

### Then

- The doc IS returned (matching is purely on tokenized content; `Lang`
  field is ignored at query time).

**Verifying test**: `TestQueryDoesNotFilterByLangField`.

---

## AC-013 — Korean query recalls at least one Korean doc

**Coverage**: REQ-IDX-003-013 (Ubiquitous), NFR-IDX-003-004 (M3 exit
contribution)

### Given

- 5 Korean docs indexed covering varied morpheme classes (news,
  Naver-blog, shopping listing, mixed Korean-English, pure Hangul) per
  the golden fixture at
  `internal/index/router/testdata/korean_golden.json`.

### When

Korean queries (e.g., "ChatGPT 사용법", "AI 추천", "서울 날씨") are
issued.

### Then

- ≥ 1 fixture doc returned in the top-10 result set per query.

### Boundary: No false-positive English match

Querying "weather forecast" against the same 5 Korean fixtures returns
0 from the Korean shard (English content gets routed to `usearch_docs`,
not `usearch_docs_ko`).

**Verifying tests**: `TestKoreanQueryReturnsAtLeastOneKoreanDoc`,
`TestKoreanQueryFiltersByContent`,
`TestKoreanFirstGoldenSet` (CI gate on every push).

---

## Edge Cases

### EC-001 — HangulRatio at exact boundary

**Coverage**: REQ-IDX-003-005, REQ-IDX-003-006 (boundary)

#### Given

- Text with HangulRatio exactly equal to `RatioHigh = 0.30` or
  `RatioLow = 0.10`.

#### When

`IndexShardForDoc` / `QueryShardsForText` are invoked.

#### Then

- `ratio >= RatioHigh` → `[ShardKo]` (boundary is inclusive on the
  high side).
- `ratio >= RatioLow` → ambiguous band → `[ShardKo, ShardDefault]`.
- `ratio < RatioLow` → `[ShardDefault]`.

### EC-002 — Race-clean concurrent Go invocations

**Coverage**: NFR-IDX-003-007 (race-clean)

#### Given

- 50 caller goroutines × 100 `Tokenize` calls each against a stub
  server.

#### When

`go test -race ./internal/index/...` is executed.

#### Then

- Zero race-detector alarms attributable to the tokenizer or router
  packages.
- `goleak.VerifyTestMain` clean for both `internal/index/tokenizer/`
  and `internal/index/router/`.

---

## NFR Coverage

| NFR | Verifying Test | Threshold |
|-----|----------------|-----------|
| NFR-IDX-003-001 (sidecar throughput) | `test_throughput_1000rps` (slow marker) | ≥ 1000 RPS |
| NFR-IDX-003-002 (single-doc p50 latency) | `test_tokenize_p50_latency_under_5ms` | p50 ≤ 5ms |
| NFR-IDX-003-003 (batch p50 via Go fanout) | `BenchmarkBatchTokenize100Docs` | p50 ≤ 50ms for 100 docs |
| NFR-IDX-003-004 (Korean-first index-side) | `TestKoreanFirstGoldenSet` | every Korean query returns ≥ 1 Korean fixture |
| NFR-IDX-003-005 (cross-shard latency) | `BenchmarkCrossShardQuery` | p95 ≤ 200ms |
| NFR-IDX-003-006 (goleak) | `goleak.VerifyTestMain` in both packages | 0 leaks |
| NFR-IDX-003-007 (race-clean) | `go test -race ./internal/index/...` | 0 alarms |

---

## Definition of Done (Verified at 2026-05-08)

- [x] Python sidecar at `services/tokenizer-ko/` with FastAPI app,
      mecab-ko Tagger, asyncio.Lock serialization.
- [x] Go HTTP client at `internal/index/tokenizer/` with retry + obs.
- [x] Go shard router at `internal/index/router/` with
      `IndexShardForDoc` + `QueryShardsForText` + `MergeRRF`.
- [x] Meili Korean shard config at
      `internal/index/meili/korean_shard.go`.
- [x] 13 EARS REQs (REQ-IDX-003-001 through REQ-IDX-003-013) covered.
- [x] 7 NFRs verified (race-clean + goleak default CI; throughput +
      latency + golden-set + cross-shard scheduled-weekly).
- [x] `go vet` + `golangci-lint` + `go test -race` all clean.
- [x] `pytest -q services/tokenizer-ko/tests/` PASS with ≥ 85% coverage.
- [x] Prometheus collectors registered: `TokenizerCalls`,
      `TokenizerLatency`, `IndexShardWrites` via
      `internal/obs/metrics/tokenizer.go`.
- [x] Cardinality allowlist extended with `shard` (2 enum values: ko,
      default).
- [x] Docker image builds; healthcheck polls `/health`.
- [x] `deploy/docker-compose.yml` includes `tokenizer-ko` on port 8083
      with `start_period: 20s`.
- [x] `.env.example` documents all `TOKENIZER_KO_*` keys.
- [x] 11 Korean particle stop-words sourced from
      `internal/router/korean.go` (no duplication).
- [x] MX tags applied: 2 ANCHOR (`IndexShardForDoc`, `QueryShardsForText`),
      WARN on Python `asyncio.Lock`, NOTE on Go fallback path.
- [x] No regression in dependent packages (SPEC-IDX-001 acceptance
      remains green).

---

*End of SPEC-IDX-003 acceptance.md (post-hoc).*
