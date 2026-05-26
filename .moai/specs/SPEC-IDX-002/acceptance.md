# SPEC-IDX-002 Acceptance Scenarios

Generated: 2026-05-26 (reverse-engineered from implemented code)
Format: Given / When / Then with verifying test references.
Status: SPEC implemented (2026-05-08); scenarios reflect realised behaviour.

This document enumerates the testable acceptance criteria for SPEC-IDX-002
(BGE-M3 embedding sidecar + Go HTTP client). Each scenario maps 1:1 to
the EARS requirements in `spec.md` §3 and the NFRs in §3
(Non-Functional Requirements table). Verifying tests live under
`services/embedder/tests/` (Python) and `internal/embedder/` (Go).

---

## AC-001 — `/embed` endpoint contract conforms to Pydantic v2 schema

**Coverage**: REQ-IDX-002-001 (Ubiquitous)

### Given

- The Python sidecar is running on port 8082 with the model loaded
  (lifespan startup complete).

### When

`POST /embed` is invoked with a body matching `EmbedRequest{request_id,
texts: ["foo", "bar", "baz"], return_dense: true, return_sparse: false,
return_colbert_vecs: false, batch_size: 32}`.

### Then

- HTTP 200 returned with body matching `EmbedResponse{request_id, dense,
  sparse, colbert, model, model_version, device, latency_ms, cache_hits,
  cache_misses}`.
- `len(response.dense) == 3` and `len(response.dense[i]) == 1024` for
  each `i`.
- `cache_hits + cache_misses == 3`.

### Boundary: Extra field rejected

POST with `{..., "unexpected_field": 1}` returns HTTP 422 (FastAPI
Pydantic validation).

### Boundary: Whitespace preserved at validation layer

POST with `{"texts": ["  hello  "]}` is NOT pre-stripped at validation
(strip happens inside the cache layer, not at Pydantic validation).

**Verifying tests**: `test_embed_happy_path`,
`test_embed_extra_field_rejected`, `test_embed_text_whitespace_preserved`
in `services/embedder/tests/test_app.py`.

---

## AC-002 — BGE-M3 inference produces 1024-dim vectors in request order

**Coverage**: REQ-IDX-002-002 (Event-Driven)

### Given

- Sidecar running with the model loaded; cache empty.

### When

`POST /embed` is invoked with `texts: ["foo", "bar", "baz"]` and
`return_dense: true`.

### Then

- `len(response.dense[0]) == 1024` (and same for indices 1, 2).
- The Python sidecar invokes
  `BGEM3FlagModel.encode(["foo", "bar", "baz"], batch_size=32, ...)`
  with texts in input order.
- Response vectors at index `i` correspond to `request.texts[i]` (order
  preserved after a permuted-input round-trip).

### Boundary: All texts cache hit

If all texts are pre-filled in the cache, ZERO model inference calls
occur (verified via mock).

**Verifying tests**: `test_embed_dense_returns_1024_dim`,
`test_embed_response_order_matches_request_order`,
`test_embed_skipped_when_all_cached` in
`services/embedder/tests/test_app.py` and `test_embed.py`.

---

## AC-003 — Loading state gates `/embed` and `/health` correctly

**Coverage**: REQ-IDX-002-003 (State-Driven)

### Given

- The model loader is patched to sleep 1 second (simulating slow load).
- Lifespan startup has NOT yet completed.

### When

1. `GET /health` is invoked.
2. `POST /embed` is invoked.

### Then

- Case 1: HTTP 503 + body `{"status":"loading","reason":"model not
  ready"}`.
- Case 2: HTTP 503 + body `{"error":"model_loading","detail":"model is
  still initialising; retry shortly"}`.

### When (after load completes)

`GET /health` is invoked again.

### Then

- HTTP 200 + body `{"status":"ok","model":"<name>",
  "model_version":"<version>","device":"<device>"}`.
- One INFO slog record `embedder.ready` emitted on transition.

**Verifying tests**: `test_health_returns_503_during_loading`,
`test_health_returns_200_after_load`, `test_embed_returns_503_during_loading`,
`test_health_records_ready_log` in `services/embedder/tests/test_app.py`.

---

## AC-004 — LRU cache hits skip inference, miss tuple is stored

**Coverage**: REQ-IDX-002-004 (Event-Driven)

### Given

- Sidecar running with default `EMBEDDER_CACHE_MAX_ENTRIES=10000`.
- Cache pre-filled with `(text="foo", dense+sparse=false, mode_flags="d=1,s=0,c=0")`.

### When

`POST /embed` is invoked with `{"texts": ["foo"], "return_dense": true}`.

### Then

- Model `BGEM3FlagModel.encode` invoked ZERO times (verified via mock).
- Response: `cache_hits == 1`, `cache_misses == 0`.
- Counter `obs.EmbedderCacheHits` (no labels) incremented by 1.

### Boundary: Different mode flags produce different cache slots

`POST /embed` with `{"texts": ["foo"], "return_dense": true,
"return_sparse": true}` → cache MISS (different mode_flags → different
cache key).

### Boundary: Cache disabled when env=0

Launching with `EMBEDDER_CACHE_MAX_ENTRIES=0` causes every request to
run inference, no LRU storage.

### Boundary: LRU eviction

With `EMBEDDER_CACHE_MAX_ENTRIES=2`, the third distinct text evicts the
first; querying the first again is a cache miss.

**Verifying tests**: `test_cache_hit_skips_inference`,
`test_cache_key_includes_mode_flags`,
`test_cache_disabled_when_max_entries_zero`, `test_cache_lru_eviction`,
`TestClientEmitsCacheHitCounter` (Go) in
`services/embedder/tests/test_cache.py` and
`internal/embedder/client_test.go`.

---

## AC-005 — Go client applies context timeout + retry on connection errors

**Coverage**: REQ-IDX-002-005 (Event-Driven)

### Given

- `httptest.NewServer` configured to sleep 30 seconds.
- Go client constructed with `EMBEDDER_REQUEST_TIMEOUT_SECONDS=1`.

### When

`client.Embed(ctx, req)` is invoked.

### Then

- Returned error wraps `context.DeadlineExceeded`.
- Total elapsed wall-clock ≤ 1.5 seconds.

### Boundary: Retry on connection reset

Stub server resets connection on first 2 attempts then returns 200; Go
client makes 3 outbound TCP connections; final result is the 200 body.

### Boundary: No retry on 4xx

Stub server returns 400; Go client makes exactly 1 outbound call and
returns `embedder.ErrInvalidRequest`.

### Boundary: Retry on 5xx

Stub server returns 503 then 200; Go client retries and returns success.

### Boundary: 503 mapped to ErrModelLoadFailed

Stub returns 503 on all attempts; final error wraps
`embedder.ErrModelLoadFailed`.

### Boundary: 500 OOM mapped to ErrOutOfMemory

Stub returns 500 with body `{"error":"oom","detail":"..."}`; client
returns `embedder.ErrOutOfMemory`.

**Verifying tests**: `TestClientEmbedTimeout`,
`TestClientEmbedRetriesOnConnReset`, `TestClientEmbed4xxNoRetry`,
`TestClientEmbed5xxRetried`, `TestClientEmbed503ModelLoading`,
`TestClientEmbed500OOM` in `internal/embedder/client_test.go`.

---

## AC-006 — Per-call observability emits ONCE per top-level invocation

**Coverage**: REQ-IDX-002-006 (Ubiquitous)

### Given

- Go client constructed with non-nil `*obs.Obs` carrying an in-memory
  OTel exporter and a Prometheus registry snapshot.

### When

`client.Embed(ctx, req)` is invoked successfully (200 response on first
attempt).

### Then

- `obs.EmbedderCalls.WithLabelValues("success", mode).Inc()` invoked
  exactly once with `mode` derived from request flags.
- `obs.EmbedderLatency.WithLabelValues("success", mode).Observe(elapsed_seconds)`
  invoked exactly once.
- `obs.EmbedderCacheHits.Add(response.cache_hits)` invoked when
  `response.cache_hits > 0`.
- One OTel span `embedder.call` created and ended with attributes
  `{request_id, texts_count, mode, cache_hits, cache_misses,
  latency_ms, outcome, model}`.
- ONE slog record at INFO level.

### Boundary: Retry does not double-count

If 2 retries occur, observability emission still happens ONCE (after
final result, with the outermost call's elapsed time).

### Boundary: Nil obs is safe

Construct Client with `obs: nil`; `Embed` does not panic; returns valid
Response.

**Verifying tests**: `TestClientEmitsCounter`,
`TestClientEmitsHistogram`, `TestClientEmitsCacheHitCounter`,
`TestClientEmitsOTelSpan`, `TestClientObservabilitySafeOnNilObs`,
`TestClientEmbedEmitsSingleObservabilityPerCall`,
`TestClientModeLabelDerivation` in
`internal/embedder/client_test.go`.

Python side: `test_python_log_record_shape` asserts exactly 1 JSON line
per `/embed` invocation with the 12 documented attributes (no PII
leakage).

---

## AC-007 — Invalid input rejected with structured error response

**Coverage**: REQ-IDX-002-007 (Unwanted), REQ-IDX-002-008 (Unwanted)

### Given

- Sidecar running with model loaded.

### When

1. `POST /embed` with `texts: []` (empty list).
2. `POST /embed` with `texts: [str(i) for i in range(257)]` (over
   batch limit).
3. `POST /embed` with one text of 100,000 characters (over BGE-M3 max
   8192 tokens after tokenisation).
4. `POST /embed` with `return_dense=false, return_sparse=false,
   return_colbert_vecs=false`.

### Then

- Case 1: HTTP 400 + `{"error":"empty_input","detail":"..."}`.
- Case 2: HTTP 400 + `{"error":"batch_too_large","detail":"..."}`.
- Case 3: HTTP 400 + `{"error":"text_too_long","detail":"..."}`.
- Case 4: HTTP 400 + `{"error":"empty_modes","detail":"at least one of
  return_dense, return_sparse, return_colbert_vecs must be true"}`.
- No model inference call for any case.
- `obs.EmbedderCalls{outcome="error_invalid", mode}` incremented
  exactly once per case.
- One WARN-level structured log record per failure with `{request_id,
  error}` (no input text content logged — privacy bound).

**Verifying tests**: `test_empty_texts_returns_400`,
`test_too_many_texts_returns_400`, `test_text_too_long_returns_400`,
`test_no_modes_requested_returns_400`, `test_invalid_input_no_inference`,
`test_invalid_input_logs_warn` in
`services/embedder/tests/test_app.py`.

---

## AC-008 — OOM does not crash the process

**Coverage**: REQ-IDX-002-009 (State-Driven)

### Given

- Sidecar running; encode is mocked to raise `MemoryError` on next call.

### When

`POST /embed` is invoked.

### Then

- HTTP 500 + body `{"error":"oom","detail":"inference out of memory;
  retry with smaller batch_size"}`.
- ONE ERROR slog record with captured exception class name.
- `obs.EmbedderCalls{outcome="error_oom", mode}` incremented once.
- Process does NOT crash; cached entries are not evicted.
- Subsequent `POST /embed` with smaller batch_size SUCCEEDS without
  process restart.

**Verifying tests**: `test_oom_returns_500`,
`test_oom_does_not_crash_process` in
`services/embedder/tests/test_embed.py`.

---

## AC-009 — Korean text passes through to BGE-M3 verbatim

**Coverage**: REQ-IDX-002-010 (Event-Driven)

### Given

- Sidecar running with model loaded; encode mocked to capture inputs.

### When

`POST /embed` is invoked with `texts: ["안녕하세요"]`.

### Then

- `BGEM3FlagModel.encode` receives the Korean string UNCHANGED (no
  pre-tokenisation, no lowercasing, no character normalisation).
- Response: `len(dense[0]) == 1024` and the vector is L2-normalised.

### Boundary: Mixed Korean-English

`POST /embed` with `texts: ["안녕 hello 안녕"]` → normal 1024-dim
response.

**Verifying tests**: `test_korean_text_dense_shape`,
`test_korean_text_passed_verbatim_to_model`,
`test_mixed_korean_english_succeeds` in
`services/embedder/tests/test_embed.py`.

---

## AC-010 — Concurrent Go client invocations are race-clean

**Coverage**: REQ-IDX-002-011 (State-Driven), NFR-IDX-005, NFR-IDX-006

### Given

- 50 caller goroutines × 100 `Embed` calls each against a stub
  `httptest.NewServer` returning canned 200 JSON = 5,000 invocations.

### When

`go test -race ./internal/embedder/...` is executed.

### Then

- Zero race-detector alarms attributable to the embedder package.
- `goleak.VerifyNone(t)` clean at test end (verified via
  `TestMain` invoking `goleak.VerifyTestMain(m)`).
- All 5,000 calls return successfully.
- Counter increments are atomic (Prometheus counter implementation).

**Verifying tests**: `TestClientEmbedConcurrent` in
`internal/embedder/concurrent_test.go`; `TestMain` in
`internal/embedder/bench_test.go`.

---

## AC-011 — Model loaded exactly once at startup, released on shutdown

**Coverage**: REQ-IDX-002-012 (Ubiquitous)

### Given

- `BGEM3FlagModel.__init__` is instrumented with a call counter.

### When

The FastAPI lifespan startup runs.

### Then

- Call count == 1 at startup.
- ONE INFO slog record `embedder.model_loaded` with `{model,
  model_version, device, use_fp16, load_seconds}` attributes.
- The model is stored as a module-level singleton.

### Boundary: Shutdown releases model

FastAPI shutdown hook triggers `del model` (or equivalent reference
release).

**Verifying tests**: `test_model_loaded_once_at_startup`,
`test_model_freed_at_shutdown` in
`services/embedder/tests/test_app.py`.

---

## AC-012 — Model version pinning via `EMBEDDER_MODEL_VERSION` env

**Coverage**: REQ-IDX-002-013 (Optional)

### Given

- `EMBEDDER_MODEL_VERSION=abc123` env set at startup.

### When

Lifespan startup runs.

### Then

- `BGEM3FlagModel(model_name, revision="abc123", ...)` invoked
  (verified via mock).
- `EmbedResponse.model_version == "abc123"`.

### Boundary: Unset or `latest` uses default revision

`EMBEDDER_MODEL_VERSION` unset → loads latest available revision;
resolved commit hash logged at startup.

**Verifying tests**: `test_model_version_pinned`,
`test_model_version_latest_default` in
`services/embedder/tests/test_app.py`.

---

## Edge Cases

### EC-001 — Partial cache hit (some texts cached, some not)

**Coverage**: REQ-IDX-002-004 (boundary)

#### Given

- Cache pre-filled with text "A"; text "B" not cached.

#### When

`POST /embed` is invoked with `texts: ["A", "B"]`.

#### Then

- `BGEM3FlagModel.encode` invoked with `["B"]` only (cache hit for "A"
  skipped).
- Response order preserved: `response.dense[0]` corresponds to "A"
  (from cache), `response.dense[1]` corresponds to "B" (from inference).
- Cache after call contains both "A" and "B".
- Response: `cache_hits == 1`, `cache_misses == 1`.

**Verifying test**: `test_embed_partial_cache_hit_only_misses_inferred`
in `services/embedder/tests/test_app.py`.

### EC-002 — Cache hit p99 latency under 5ms

**Coverage**: NFR-IDX-003

#### Given

- 100 sequential identical `POST /embed` requests.

#### When

The first call (cache miss) is excluded; the subsequent 99 calls (all
cache hits) are measured.

#### Then

- `max(durations[1:]) ≤ 0.005` seconds (5 ms wall-clock).

**Verifying test**: `test_cache_hit_latency` in
`services/embedder/tests/test_cache.py`.

---

## NFR Coverage

| NFR | Verifying Test | Threshold |
|-----|----------------|-----------|
| NFR-IDX-001 (CPU throughput) | `services/embedder/tests/test_throughput.py::test_throughput_30rps_dense` | ≥ 30 docs/sec on 4 vCPU CPU, FP32 |
| NFR-IDX-002 (warm p50 latency CPU) | `services/embedder/tests/test_latency.py::test_p50_latency_cpu` | p50 ≤ 500 ms |
| NFR-IDX-003 (cache-hit p99 latency) | `test_cache_hit_latency` | p99 ≤ 5 ms |
| NFR-IDX-004 (steady-state cache hit ratio) | `test_steady_state_hit_ratio` | ≥ 30% after 5-min warmup |
| NFR-IDX-005 (race-clean) | `TestClientEmbedConcurrent` + `go test -race` | 0 alarms |
| NFR-IDX-006 (zero goroutine leaks) | `goleak.VerifyTestMain` in `bench_test.go` | 0 leaks |
| NFR-IDX-007 (memory ceiling CPU FP32) | `test_soak_memory` | ≤ 4 GB resident over 1-hour soak |

Tests marked `@pytest.mark.slow` (throughput, latency, soak) run on the
weekly CI cadence, not on every PR.

---

## Definition of Done (Verified at 2026-05-08)

- [x] Python sidecar 62 tests passing with 93% coverage on
      `services/embedder/src/embedder/`.
- [x] Go client 15 tests passing with 89.3% coverage on
      `internal/embedder/`.
- [x] 13 EARS REQs (REQ-IDX-002-001 through REQ-IDX-002-013) covered
      by tests.
- [x] 7 NFRs verified (race-clean + goleak in default CI; throughput +
      latency + cache-hit + memory in scheduled-weekly CI).
- [x] `go vet ./internal/embedder/...` → 0 issues.
- [x] `golangci-lint run ./internal/embedder/...` → 0 issues.
- [x] `go test -race ./internal/embedder/...` PASS.
- [x] Build success on full project.
- [x] Prometheus collectors registered: `EmbedderCalls`,
      `EmbedderLatency`, `EmbedderCacheHits` via
      `internal/obs/metrics/embedder.go`.
- [x] Cardinality allowlist extended with `mode` (4 enum values:
      dense, sparse, colbert, all).
- [x] `services/embedder/Dockerfile` builds successfully; healthcheck
      polls `/health`.
- [x] `deploy/docker-compose.yml` includes embedder service on port
      8082 with `embedder_models` volume.
- [x] `deploy/docker-compose.gpu.yml` overlay provides NVIDIA device
      reservation + FP16 env overrides.
- [x] `.env.example` documents all `EMBEDDER_*` keys.
- [x] MX tags applied: 1 ANCHOR (`Client`), 1 NOTE (`ModeLabel`).
- [x] No regression in dependent packages.

---

*End of SPEC-IDX-002 acceptance.md (post-hoc).*
