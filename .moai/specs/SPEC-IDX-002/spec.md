---
id: SPEC-IDX-002
title: Embedding Service (BGE-M3 Python sidecar)
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
depends_on: [SPEC-BOOT-001, SPEC-OBS-001]
blocks: [SPEC-IDX-001, SPEC-CACHE-001]
---

# SPEC-IDX-002: Embedding Service (BGE-M3 Python sidecar)

## HISTORY

- 2026-05-04 (initial draft v0.1, limbowl via manager-spec):
  First EARS-formatted SPEC for the M3 embedding service. Drafted after
  research phase (`.moai/specs/SPEC-IDX-002/research.md`); every claim
  file:line-cited or URL-cited. Builds on SPEC-BOOT-001 (the empty
  `services/embedder/` scaffold + `services/embedder/Dockerfile` +
  `services/embedder/pyproject.toml` per spec.md:99-105) and SPEC-OBS-001
  (the Registry + cardinality allowlist + import-boundary pattern at
  `internal/obs/metrics/metrics.go:55-178`).

  Coordinates with SPEC-IDX-001 (the M3 RRF / hybrid index consumer of
  `[]EmbeddingResult`) and SPEC-CACHE-001 (the 5-phase access fallback
  that does NOT consume embeddings directly but blocks until embedder
  is reachable for downstream re-ranking). SPEC-IDX-003 (Korean
  tokenization) is a SEPARATE sidecar concerning Meilisearch's lexical
  tokenizer; SPEC-IDX-002's BGE-M3 internal multilingual tokenization
  (XLM-RoBERTa) is independent.

  User-locked decisions (verified in research §2 + §3 + §4 + §5):

  - **D1 Inference path**: vanilla PyTorch via `FlagEmbedding>=1.3.0`
    `BGEM3FlagModel` API. ONNX Runtime path (research §2.2) and vLLM
    path (research §2.3) both deferred to follow-up SPECs (Open
    Questions §11.1).
  - **D2 Output modes**: per-request flags `return_dense` (default
    true), `return_sparse` (default false), `return_colbert_vecs`
    (default false). Single forward pass when multiple modes are
    requested. Storage cost decisions (whether to populate sparse +
    ColBERT in the index) belong to SPEC-IDX-001.
  - **D3 GPU vs CPU**: CPU is the default (V1 deploy targets
    self-hosted teams per `tech.md:93`). GPU is opt-in via a
    `deploy/docker-compose.gpu.yml` overlay file. NFR thresholds
    (NFR-IDX-002) are set against the CPU 4 vCPU floor.
  - **D4 Cache**: in-process LRU bounded by entry count. Redis +
    disk backends are future opt-ins via `EMBEDDER_CACHE_BACKEND`
    env (Open Question §11.4). Cache key is
    `sha256(text + model_version + mode_flags)`.
  - **D5 Observability**: NEW metric family `usearch_embedder_*` with
    `outcome` (already allowlisted) and `mode` (NEW label name —
    requires cardinality allowlist amendment per §6.4). Cache-hit
    counter has NO labels (mirrors `SynthesisCost` precedent at
    `internal/obs/metrics/metrics.go:60-61`).
  - **D6 Concurrency**: uvicorn `--workers 1` with async event loop;
    inference is naturally serialised inside the worker. Compute
    parallelism is via SEPARATE replicas (M9 Helm chart concern), not
    multiple workers within a single process.
  - **D7 Korean text handling**: BGE-M3's internal XLM-RoBERTa
    tokenizer handles Korean natively; the sidecar passes Korean
    text through verbatim with no special preprocessing. The
    `tech.md:144-153` Korean tokenizer risk is delegated to
    SPEC-IDX-003 (Meilisearch lexical tokenization, separate
    concern).
  - **D8 Subprocess lifecycle**: model loaded ONCE inside FastAPI
    `lifespan` startup; freed during shutdown. No per-request
    subprocess. `restart: unless-stopped` in compose handles crash
    recovery.

  13 EARS REQs (12 × P0 + 1 × P1) covering all five EARS patterns
  (Ubiquitous, Event-Driven, State-Driven, Optional, Unwanted),
  6 NFRs (throughput, p50 latency, cache hit ratio, race-clean,
  goroutine-leak-clean, memory ceiling), 8 Open Questions carried
  forward from research.md §8 for plan-auditor challenge.
  Greenfield Python module + Greenfield Go module — no behaviour
  to preserve (per workflow-modes.md §Brownfield Enhancement).

  Insertion point: M3 row at `.moai/project/roadmap.md:56`.
  Parallelizable with SPEC-IDX-001 + SPEC-IDX-003 per
  `roadmap.md:122-128`. Harness level: standard (single-domain Python
  + single-domain Go + 1 compose entry + no security/payment/PII
  keywords).

  Status: `draft`. Plan-auditor cycle skipped (orchestrator handles
  audit pass per session pattern). Ready for plan-auditor review and
  annotation cycle on subsequent invocation.

---

## 1. Purpose

The M3 row at `.moai/project/roadmap.md:55-58` reads:

> SPEC-IDX-002 | Embedding service | BGE-M3 Python sidecar, batched
> inference, cache | expert-backend

SPEC-IDX-002 fills `services/embedder/` (an empty scaffold provisioned
by SPEC-BOOT-001 per spec.md:99-105) with a **BGE-M3 embedding
sidecar** plus a **Go-side HTTP client** at `internal/embedder/` that
makes embeddings consumable by SPEC-IDX-001 (hybrid index ingestion)
and by future SPEC-DEEP-* (M5 deep-research embedding-based source
selection).

The embedding sidecar:

1. Loads `BAAI/bge-m3` once at startup via `FlagEmbedding.BGEM3FlagModel`
   (research §2.1; verified
   https://huggingface.co/BAAI/bge-m3 — MIT license, 1024-dim dense,
   8192 max tokens, multilingual incl. Korean).
2. Exposes a single FastAPI HTTP endpoint `POST /embed` accepting a
   list of texts and per-request mode flags
   (`return_dense`/`return_sparse`/`return_colbert_vecs`).
3. Returns the three vector representations BGE-M3 supports:
   - **Dense**: 1024-dim float32 array per input text.
   - **Sparse**: dict mapping token-id → IDF-weighted float per input
     text.
   - **ColBERT** (multi-vector): list of 1024-dim vectors per input
     text (one per token).
4. Caches embeddings in an in-process LRU keyed on
   `sha256(text + model_version + mode_flags)` with bounded entry
   count (research §4.1). Cache hits short-circuit the inference path
   and increment a dedicated counter.
5. Emits per-call observability per the
   `internal/obs/metrics/synthesis.go` precedent: 1 slog record + 1
   counter increment + 1 histogram observation + 1 OTel span on every
   `/embed` call, on both the Python service side and the Go client
   side. NEW metric family `usearch_embedder_*` registered via
   `internal/obs/metrics/embedder.go`.
6. Supports optional GPU acceleration via a compose overlay file; CPU
   FP32 is the default. The Python sidecar reads `EMBEDDER_DEVICE`
   (default `cpu`) and `EMBEDDER_USE_FP16` (default `false` for CPU,
   recommended `true` for GPU per BGE-M3 README).
7. Provides a healthcheck endpoint that returns 200 only after the
   model has finished loading (model-load takes 5-30 seconds on first
   boot from Hugging Face Hub).

Completion unblocks SPEC-IDX-001 (the M3 RRF + hybrid index that
populates Qdrant with dense vectors and Meilisearch with sparse
weights) and SPEC-CACHE-001 (which depends on the embedding stage
being reliably reachable, though it does not consume embeddings
directly). Future SPEC-DEEP-* (M5) reuses this sidecar for
embedding-based source selection in deep research.

This SPEC is on the M3 critical path: without it, SPEC-IDX-001 cannot
produce dense vectors and the hybrid retrieval design at
`tech.md:39-50` cannot be exercised.

---

## 2. Scope

### 2.1 In-Scope

| Ref | Item |
|-----|------|
| a | `services/embedder/src/embedder/` package layout: `app.py` (FastAPI app + lifespan + routes), `models.py` (Pydantic v2 request/response models), `embed.py` (BGEM3FlagModel wrapper + inference logic), `cache.py` (LRU cache + key derivation), `obs.py` (slog-equivalent JSON logger + per-call timing helpers), `__main__.py` (uvicorn entrypoint), `__init__.py` (replace empty stub with package doc + `__version__`) |
| b | `services/embedder/pyproject.toml` runtime dependency additions: `fastapi>=0.115`, `uvicorn[standard]>=0.30`, `pydantic>=2.9`, `FlagEmbedding>=1.3.0`, `numpy>=1.26`, `cachetools>=5.5` |
| c | FastAPI `POST /embed` endpoint accepting `EmbedRequest` and returning `EmbedResponse` with status codes per §4.6 |
| d | FastAPI `GET /health` endpoint returning 200 after model load completes; 503 with `{"status":"loading","reason":"model not ready"}` while the model is still being downloaded/initialised |
| e | `BGEM3FlagModel` integration: model loaded once during `lifespan` startup; `EMBEDDER_MODEL` (default `BAAI/bge-m3`), `EMBEDDER_DEVICE` (default `cpu`), `EMBEDDER_USE_FP16` (default `false`), `EMBEDDER_MAX_LENGTH` (default `8192`) read at startup |
| f | Per-request mode flags: `return_dense` (default `true`), `return_sparse` (default `false`), `return_colbert_vecs` (default `false`); all three may be `true` simultaneously (single forward pass) |
| g | LRU cache (`cachetools.LRUCache`) keyed on `sha256(text + model_version + mode_flags)`; bounded by `EMBEDDER_CACHE_MAX_ENTRIES` (default `10000`); cache disabled when env value is `0`; cache hit short-circuits the inference path |
| h | `internal/embedder/types.go` — Go-side value types `Request`, `Response`, `Embedding{Dense, Sparse, ColBERT}`, error sentinels (`ErrInvalidRequest`, `ErrSidecarUnreachable`, `ErrTimeout`, `ErrModelLoadFailed`, `ErrOutOfMemory`) |
| i | `internal/embedder/config.go` — env binder for `EMBEDDER_BASE_URL` (default `http://localhost:8082`) and `EMBEDDER_REQUEST_TIMEOUT_SECONDS` (default `15`), mirroring the `internal/synthesis/config.go` pattern |
| j | `internal/embedder/client.go` — Go HTTP client struct `Client` with `Embed(ctx, req EmbedRequest) (EmbedResponse, error)` method; `*http.Client{Timeout}` + exponential backoff retry (2 retries on connection-level errors); identical observability emit pattern to `internal/synthesis/client.go:196-240` |
| k | `internal/embedder/embedder.go` — replace nonexistent path with package doc + value-type re-exports |
| l | `internal/obs/metrics/embedder.go` — NEW file declaring `EmbedderCalls *prometheus.CounterVec{outcome,mode}`, `EmbedderLatency *prometheus.HistogramVec{outcome,mode}`, `EmbedderCacheHits prometheus.Counter` (no labels). Mirrors `internal/obs/metrics/synthesis.go:1-60` pattern |
| m | `internal/obs/metrics/metrics.go` — minor edit: add 3 fields to `Registry`, call `registerEmbedder(pr)` from `NewRegistry()`, append `mode` to `labelNames` allowlist |
| n | `services/embedder/Dockerfile` — multi-stage rebuild on `python:3.11-slim` with explicit non-root `appuser`; healthcheck calls `/health`; `EXPOSE 8082`; copies HF cache directory ownership to `appuser`; mirrors `services/researcher/Dockerfile:1-30` shape |
| o | `deploy/docker-compose.yml` delta: new `embedder` service entry (port `8082`, healthcheck on `/health`, named volume `embedder_models` for HF model cache, `restart: unless-stopped`, `networks: [app]`); env mapping of all `EMBEDDER_*` keys |
| p | `deploy/docker-compose.gpu.yml` overlay file: NEW; adds `deploy.resources.reservations.devices` NVIDIA stanza for the embedder service plus `EMBEDDER_DEVICE: cuda:0` and `EMBEDDER_USE_FP16: "true"` env overrides |
| q | Root `.env.example` additions: `EMBEDDER_BASE_URL`, `EMBEDDER_REQUEST_TIMEOUT_SECONDS`, `EMBEDDER_PORT`, `EMBEDDER_MODEL`, `EMBEDDER_MODEL_VERSION`, `EMBEDDER_DEVICE`, `EMBEDDER_USE_FP16`, `EMBEDDER_BATCH_SIZE`, `EMBEDDER_MAX_LENGTH`, `EMBEDDER_CACHE_MAX_ENTRIES`, `EMBEDDER_LOG_LEVEL` |
| r | `services/embedder/tests/` test files: `test_app.py` (HTTP endpoint contract via FastAPI `TestClient`), `test_embed.py` (BGEM3FlagModel wrapper with stubbed model), `test_cache.py` (LRU cache hit/miss + key derivation), `test_obs.py` (slog-equivalent JSON record shape), `test_models.py` (Pydantic v2 validation) |
| s | `internal/embedder/client_test.go` — Go HTTP client integration tests against an in-process FastAPI fixture launched via `httptest.NewServer` returning canned JSON; covers happy path, timeout, retry, model-loading 503 passthrough, and connection refused |

### 2.2 Out-of-Scope (Exclusions — What NOT to Build)

[HARD] This SPEC explicitly excludes the following items. Each has a known
destination SPEC; this list prevents scope creep into IDX-002 (the M3
embedding gateway).

- **RRF fusion / hybrid ranking** (combining dense + sparse + ColBERT
  scores into a single ranked list). → SPEC-IDX-001 (M3). Embedder
  output is RRF input.
- **Vector storage** (writing dense vectors to Qdrant, sparse weights
  to Meilisearch, ColBERT vectors to a multi-vector store). →
  SPEC-IDX-001 (M3). The sidecar is a pure compute service.
- **Korean lexical tokenizer integration** (mecab-ko / khaiii in front
  of Meilisearch). → SPEC-IDX-003 (M3). The sidecar's BGE-M3 internal
  XLM-RoBERTa tokenizer is independent.
- **Cross-encoder reranking** (BGE-reranker-v2-m3 for top-50 → top-10
  refinement). → Future SPEC if measured value; SPEC-IDX-002 ships
  bi-encoder embeddings only.
- **Streaming embeddings** (SSE / chunked response for very large
  batches). → Out of v0.1; the V1 contract is request/response JSON.
- **gRPC contract** between Go and Python. → Out of v0.1; HTTP/JSON
  is sufficient. Future SPEC if measured.
- **Batch persistence to disk** (writing embeddings to a parquet file
  or vector store as a side effect of `/embed`). → SPEC-IDX-001
  (M3) owns ingestion.
- **Multi-tenant embedding quotas** (per-team / per-user rate limits).
  → SPEC-AUTH-002 (M6).
- **Per-tenant model selection** (different teams use different
  embedding models). → Out of V1; one model globally.
- **Fine-tuning / domain adaptation** of BGE-M3. → Out of V1; the
  sidecar consumes a static model.
- **ONNX export pipeline** (Path B in research §2.2). → Future
  SPEC-IDX-002a if measured throughput is the constraint.
- **vLLM-based embedding server** (Path C in research §2.3). →
  Future SPEC-IDX-002b if GPU throughput becomes the bottleneck.
- **Redis-backed cache** (`EMBEDDER_CACHE_BACKEND=redis`). → Future
  opt-in via `EMBEDDER_CACHE_BACKEND` env; v0.1 ships in-process LRU
  only.
- **Disk-persistent cache** (`EMBEDDER_CACHE_BACKEND=disk`). → Future
  opt-in.
- **Cross-replica cache sharing**. → Future SPEC if multi-instance
  deployments require it.
- **`/embed/stream` SSE endpoint**. → Out of v0.1.
- **HTTP / gRPC exposure to external clients** outside the
  Universal Search trust boundary. → SPEC-MCP-001 (M7) and future
  SPEC-API-001. The sidecar binds to a private compose network port.
- **Security hardening** (TLS termination, mTLS, IP allowlist). →
  SPEC-SEC-001 (M8). v0.1 trusts the compose network.
- **Embedding-version migration** (recomputing the index when the
  BGE-M3 version is bumped). → SPEC-IDX-001 (M3) or future SPEC.
- **Cardinality allowlist amendment** for label names other than
  `mode`. SPEC-IDX-002 introduces ONLY the `mode` label name; no
  others.
- **`pkg/types` extension**. SPEC-IDX-002 adds zero exported types
  to the SDK boundary per `.moai/project/structure.md:160-165`.
- **GitHub Issue tracking on this SPEC** (`issue_number: null`).

### 2.3 Multi-Vector Output Contract

[HARD] The sidecar's three-mode output is structurally rich and worth
specifying explicitly so SPEC-IDX-001 can consume without ambiguity.

For an input list `texts` of length `N`:

**Dense mode** (`return_dense=true`):
- Response field `dense`: `list[list[float]]` of shape `[N, 1024]`.
- Each vector is L2-normalised by BGE-M3 (verified BGE-M3 README).
- Suitable for cosine similarity in Qdrant.

**Sparse mode** (`return_sparse=true`):
- Response field `sparse`: `list[dict[str, float]]` of length `N`.
- Each dict maps a token-id (as a stringified integer) to its IDF
  weight (positive float).
- Empty dict `{}` is valid (no salient tokens above threshold).
- Suitable for hybrid retrieval with Meilisearch's BM25 (the
  integration mapping lives in SPEC-IDX-001 / SPEC-IDX-003).

**ColBERT mode** (`return_colbert_vecs=true`):
- Response field `colbert`: `list[list[list[float]]]` of shape
  `[N, T_i, 1024]` where `T_i` is the post-tokenisation length of
  the i-th text (≤ `EMBEDDER_MAX_LENGTH`).
- Each row vector is L2-normalised.
- Storage cost ≈ 4 × `T_i` × 1024 bytes per text. For `T_i = 256`,
  this is ~1 MB per text — significantly larger than dense.
- SPEC-IDX-001 chooses whether to populate this; default OFF.

When ALL THREE flags are `false`, the sidecar SHALL return HTTP 400
with `{"error":"empty_modes","detail":"at least one of return_dense, return_sparse, return_colbert_vecs must be true"}`
(REQ-IDX-IDX-002-009).

### 2.4 Cache Key Derivation

[HARD] The cache key is a deterministic SHA-256 hex digest computed from:

```
key_input = f"{text.strip()}\n{model_name}\n{model_version}\n{mode_flags}"
key = hashlib.sha256(key_input.encode("utf-8")).hexdigest()
```

Where:

- `text.strip()` — leading/trailing whitespace removed; we do NOT
  lowercase or strip punctuation (BGE-M3 is case-sensitive and
  punctuation-aware).
- `model_name` — `EMBEDDER_MODEL` env value (default `BAAI/bge-m3`).
- `model_version` — resolved at boot via Hugging Face Hub commit hash
  if `EMBEDDER_MODEL_VERSION=latest`, otherwise the explicit env
  value. Logged at INFO once at startup.
- `mode_flags` — concatenation of literal strings reflecting the
  request's mode flags, e.g., `"d=1,s=0,c=0"` for dense-only,
  `"d=1,s=1,c=0"` for dense+sparse, `"d=1,s=1,c=1"` for all-three.

Properties:

- Different mode flag combinations get DIFFERENT cache slots even for
  the same text — a dense-only call does not satisfy a subsequent
  dense+sparse call.
- Model version changes invalidate the cache automatically.
- Text-only changes (a trailing newline added by accident) DO produce
  different keys, but `text.strip()` mitigates the most common case
  (incidental whitespace from upstream JSON parsers).

Cache value:

- `(dense: list[float] | None, sparse: dict[str, float] | None, colbert: list[list[float]] | None)`
  tuple; only the fields requested by the original mode flags are
  populated. Cache values are returned VERBATIM on hit.

### 2.5 Cache-Hit Semantics

[HARD] When all texts in a request are cache hits:

- The sidecar SHALL respond within 5 ms wall-clock (NFR-IDX-003).
- The `/embed` response field `cache_hits = N` (where `N = len(texts)`)
  and `cache_misses = 0`.
- The Go-side `obs.EmbedderCacheHits` counter is incremented by `N`.
- The histogram observation is recorded with `outcome="success"` and
  `mode={dense|sparse|colbert|all}` per the dominant requested mode.
- NO model inference happens; CPU/GPU utilisation stays flat.

When PARTIAL cache hits occur (some texts hit, others miss):

- The sidecar SHALL run inference on the missed texts ONLY.
- Cache misses are added to the cache after inference.
- Response fields: `cache_hits = K_hits`, `cache_misses = K_misses`,
  with `K_hits + K_misses = len(texts)`.
- Order is preserved: response vectors at index `i` correspond to
  request texts at index `i`, regardless of cache hit/miss.

### 2.6 Loading State

[HARD] The model takes 5-30 seconds to load on first boot (download
~1.4 GB from Hugging Face Hub) and 1-3 seconds on warm boot (model
cached on disk). During loading:

- `GET /health` returns HTTP 503 with body
  `{"status":"loading","reason":"model not ready"}`. No retry-after
  header (callers poll at their own cadence).
- `POST /embed` returns HTTP 503 with body
  `{"error":"model_loading","detail":"model is still initialising; retry shortly"}`.
- The Go-side client observes outcome `error_unavailable` and the
  caller (SPEC-IDX-001) MUST retry with exponential backoff.

Once loading completes:

- `GET /health` returns HTTP 200 with body
  `{"status":"ok","model":"<name>","model_version":"<version>","device":"<device>"}`.
- `POST /embed` accepts requests normally.

The healthcheck transition is recorded once at INFO (`embedder.ready`
slog event) — this is the canonical "service is now serving"
signal for ops.

---

## 3. EARS Requirements

### Functional Requirements

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| REQ-IDX-002-001 | Ubiquitous | The Python sidecar SHALL expose a single HTTP endpoint `POST /embed` accepting `application/json` requests matching the Pydantic v2 model `EmbedRequest{request_id: str, texts: list[str], return_dense: bool = true, return_sparse: bool = false, return_colbert_vecs: bool = false, batch_size: int = 32}` and returning HTTP 200 with `application/json` matching `EmbedResponse{request_id, dense, sparse, colbert, model, model_version, device, latency_ms, cache_hits, cache_misses}`. Pydantic config SHALL be `ConfigDict(extra="forbid", str_strip_whitespace=False)` (text-strip happens inside the cache layer, not at validation time). | P0 | `test_embed_happy_path` POSTs valid request, asserts 200 + response shape; `test_embed_extra_field_rejected` asserts 422 on unknown fields; `test_embed_text_whitespace_preserved` asserts that text with leading/trailing whitespace is NOT pre-stripped at the validation layer. |
| REQ-IDX-002-002 | Event-Driven | WHEN a `/embed` request arrives with `len(texts) >= 1` and at least one of `return_dense / return_sparse / return_colbert_vecs` is `true`, the service SHALL invoke `BGEM3FlagModel.encode(texts, batch_size=req.batch_size, max_length=EMBEDDER_MAX_LENGTH, return_dense=..., return_sparse=..., return_colbert_vecs=...)` for the cache-missed subset, MAY skip inference entirely when all texts hit the cache (per §2.5), and SHALL return vectors in REQUEST ORDER (response index `i` corresponds to `request.texts[i]`). | P0 | `test_embed_dense_returns_1024_dim` asserts `len(resp.dense[0]) == 1024`; `test_embed_response_order_matches_request_order` asserts vectors at index `i` correspond to texts at index `i` after a permuted-input round-trip; `test_embed_skipped_when_all_cached` mocks the model to assert zero inference calls when every text is a cache hit. |
| REQ-IDX-002-003 | State-Driven | WHILE the model is still loading at startup (the `lifespan` startup hook has not yet completed) the service SHALL respond to `GET /health` with HTTP 503 + body `{"status":"loading","reason":"model not ready"}` and to `POST /embed` with HTTP 503 + body `{"error":"model_loading","detail":"model is still initialising; retry shortly"}`. After loading completes, the service SHALL respond to `GET /health` with HTTP 200 + body `{"status":"ok","model":<name>,"model_version":<version>,"device":<device>}` and accept `POST /embed` requests normally. | P0 | `test_health_returns_503_during_loading` injects a slow model loader; asserts 503; `test_health_returns_200_after_load` waits for the lifespan startup to complete; asserts 200; `test_embed_returns_503_during_loading` asserts the embed endpoint is gated; `test_health_records_ready_log` asserts a single INFO-level `embedder.ready` slog record on transition. |
| REQ-IDX-002-004 | Event-Driven | WHEN a `/embed` request's `(text_normalised, model_name, model_version, mode_flags)` tuple matches an existing entry in the in-process LRU cache, the service SHALL return the cached vectors WITHOUT invoking the model, increment `cache_hits` in the response by 1 per hit text, AND increment `obs.EmbedderCacheHits` (a Counter with no labels) by the per-call hit count. WHEN the tuple is not in the cache, the service SHALL run inference, store the result in the cache (subject to LRU eviction), and increment `cache_misses` accordingly. The cache is bounded by `EMBEDDER_CACHE_MAX_ENTRIES` (default 10000); when the env value is `0`, the cache SHALL be disabled and every request SHALL run inference. | P0 | `test_cache_hit_skips_inference` mocks the model and asserts no inference call on the second identical request; `test_cache_key_includes_mode_flags` asserts that a dense-only request and a dense+sparse request for the same text produce DIFFERENT cache slots; `test_cache_disabled_when_max_entries_zero` runs with `EMBEDDER_CACHE_MAX_ENTRIES=0` and asserts every request runs inference; `TestClientEmitsCacheHitCounter` (Go side) asserts `obs.EmbedderCacheHits` increments. |
| REQ-IDX-002-005 | Event-Driven | WHEN the Go-side caller `internal/embedder.Client.Embed(ctx, req)` invokes the sidecar, the Go client SHALL apply a wall-clock timeout from `EMBEDDER_REQUEST_TIMEOUT_SECONDS` (default 15) via `context.WithTimeout`, SHALL retry on `*net.OpError` and `*url.Error` types up to 2 times with exponential backoff (500 ms, 1500 ms ± 10% jitter), SHALL NOT retry on HTTP 4xx responses (returns `ErrInvalidRequest`), SHALL retry on HTTP 5xx (transient), and SHALL emit one slog record + one `obs.EmbedderCalls{outcome,mode}` increment + one `obs.EmbedderLatency{outcome,mode}` observation + one OTel span `embedder.call` per top-level invocation (NOT per retry). | P0 | `TestClientEmbedTimeout`, `TestClientEmbedRetriesOnConnReset`, `TestClientEmbed4xxNoRetry`, `TestClientEmbed5xxRetried`, `TestClientEmbedEmitsSingleObservabilityPerCall`. |
| REQ-IDX-002-006 | Ubiquitous | The Python sidecar AND the Go client SHALL emit per-`/embed` invocation observability: (a) the sidecar logs a single JSON record at INFO with `{request_id, texts_count, return_dense, return_sparse, return_colbert_vecs, cache_hits, cache_misses, latency_ms, model, model_version, device, outcome}`; outcome ∈ `{success, error_invalid, error_oom, error_loading, error_internal}`; (b) the Go client increments `obs.EmbedderCalls.WithLabelValues(outcome, mode).Inc()` exactly once per top-level call where mode ∈ `{dense, sparse, colbert, all}` (`all` when ≥ 2 modes are requested), observes `obs.EmbedderLatency.WithLabelValues(outcome, mode).Observe(elapsed_seconds)` exactly once, calls `obs.EmbedderCacheHits.Add(response.cache_hits)` when `response.cache_hits > 0`, and creates+ends one OTel span `embedder.call` with attributes `{request_id, texts_count, mode, cache_hits, cache_misses, latency_ms, outcome, model}`. The Go client SHALL be nil-safe across `obs.Obs`, individual collectors, and `obs.Logger` per the pattern at `internal/synthesis/client.go:208-226`. | P0 | `test_python_log_record_shape`; `TestClientEmitsCounter`, `TestClientEmitsHistogram`, `TestClientEmitsCacheHitCounter`, `TestClientEmitsOTelSpan`, `TestClientObservabilitySafeOnNilObs`. |
| REQ-IDX-002-007 | Unwanted | IF a `/embed` request body has `len(texts) == 0` OR `len(texts) > 256` OR any text exceeds the BGE-M3 max-length of 8192 tokens AFTER tokenisation, THEN the service SHALL return HTTP 400 with body `{"error":"<code>","detail":"<which>"}` where `<code>` ∈ `{empty_input, batch_too_large, text_too_long}`, SHALL NOT invoke the model, SHALL increment `obs.EmbedderCalls{outcome="error_invalid"}` exactly once, AND SHALL emit one WARN-level structured log record with attributes `{request_id, error}`. The 8192-token check MAY be performed via the model's tokenizer in a fast pre-pass (the tokenizer-only call is ~1 ms per text) or deferred to inference-time error handling — the run-phase author chooses. | P0 | `test_empty_texts_returns_400`; `test_too_many_texts_returns_400`; `test_text_too_long_returns_400` (text ≥ 100k chars); assert no model inference call in any failure case. |
| REQ-IDX-002-008 | Unwanted | IF a `/embed` request has `return_dense == false` AND `return_sparse == false` AND `return_colbert_vecs == false` (all modes off), THEN the service SHALL return HTTP 400 with body `{"error":"empty_modes","detail":"at least one of return_dense, return_sparse, return_colbert_vecs must be true"}`, SHALL NOT invoke the model, AND SHALL NOT increment any counters except `obs.EmbedderCalls{outcome="error_invalid",mode="dense"}` once (the default-mode label since no real mode applies). | P0 | `test_no_modes_requested_returns_400`. |
| REQ-IDX-002-009 | State-Driven | WHILE the model has reported an out-of-memory condition during inference (e.g., `torch.cuda.OutOfMemoryError` on GPU or a Python `MemoryError` on CPU), the service SHALL respond to `/embed` with HTTP 500 + body `{"error":"oom","detail":"inference out of memory; retry with smaller batch_size"}`, SHALL log one ERROR-level structured record with the captured exception class name, SHALL increment `obs.EmbedderCalls{outcome="error_oom",mode}` once, AND SHALL NOT crash the process or evict cached entries. The OOM SHALL be recoverable: the next request with a smaller batch_size SHOULD succeed without process restart. | P0 | `test_oom_returns_500`; `test_oom_does_not_crash_process` (raise `MemoryError` from a mocked encode; verify subsequent normal call succeeds). |
| REQ-IDX-002-010 | Event-Driven | WHEN a Korean-language text is passed to `/embed` (any text containing Hangul code points U+AC00–U+D7A3 or U+1100–U+11FF), the service SHALL pass the text VERBATIM to `BGEM3FlagModel.encode` without any pre-tokenisation, lowercasing, or character normalisation, AND SHALL produce dense vectors with the same shape and L2-normalisation as for any other language. Korean text handling is internal to BGE-M3's XLM-RoBERTa tokenizer (per BGE-M3 README); SPEC-IDX-002 makes no Korean-specific code path. | P0 | `test_korean_text_dense_shape` posts `["안녕하세요"]` and asserts `len(resp.dense[0]) == 1024`; `test_korean_text_passed_verbatim_to_model` mocks the model and asserts the Korean string is the unchanged argument; `test_mixed_korean_english_succeeds` posts `["안녕 hello 안녕"]` and asserts a normal 1024-dim response. |
| REQ-IDX-002-011 | State-Driven | WHILE multiple goroutines invoke the same `internal/embedder.Client` concurrently (N ≥ 1 caller goroutines each calling `Embed(ctx, req)`), each call SHALL execute independently with no shared mutable state in the request path (the `*Client` struct is immutable post-construction; `*http.Client` is goroutine-safe per stdlib contract; `obs.Obs` is goroutine-safe per SPEC-OBS-001), the cumulative effect SHALL be N independent embed dispatches with no race-detector alarms attributable to `internal/embedder/`, AND `obs.EmbedderCalls` / `obs.EmbedderLatency` / `obs.EmbedderCacheHits` updates SHALL be atomic. | P0 | `TestClientEmbedConcurrent` in `internal/embedder/concurrent_test.go`: 50 caller goroutines × 100 calls × stub server returning canned 200 JSON = 5,000 invocations under `go test -race`. Assert: zero race-detector alarms; `goleak.VerifyNone(t)` clean. |
| REQ-IDX-002-012 | Ubiquitous | The Python sidecar SHALL load the BGE-M3 model exactly ONCE during the FastAPI `lifespan` startup hook and SHALL release it during the `lifespan` shutdown hook. The model load SHALL log one INFO-level `embedder.model_loaded` record with `{model, model_version, device, use_fp16, load_seconds}` attributes. The model SHALL be stored as a module-level or app-state singleton; the singleton SHALL NOT be reloaded mid-process. | P0 | `test_model_loaded_once_at_startup` instruments `BGEM3FlagModel.__init__` and asserts call count == 1 across the test session; `test_model_freed_at_shutdown` triggers FastAPI shutdown and asserts `del model` (or equivalent reference release). |
| REQ-IDX-002-013 | Optional | WHERE `EMBEDDER_MODEL_VERSION` is set to a non-empty, non-`latest` value at startup, the sidecar SHALL pass that value as the `revision` argument to `BGEM3FlagModel(model_name, ..., revision=<value>)` (verified API: FlagEmbedding inherits the `revision` kwarg from huggingface_hub) AND SHALL include the resolved version in the `EmbedResponse.model_version` field. WHERE `EMBEDDER_MODEL_VERSION` is unset or `latest`, the sidecar SHALL load the latest available revision and SHALL log the resolved commit hash at startup. | P1 | `test_model_version_pinned`: set `EMBEDDER_MODEL_VERSION=abc123` env; mock the model loader to capture the kwarg; assert `revision="abc123"` was passed; `test_model_version_latest_default` asserts unset env triggers the default code path. |

### Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| NFR-IDX-001 | Performance (CPU throughput) | The Python sidecar running on a 4 vCPU CPU container with FP32 SHALL achieve sustained throughput of at least 30 documents/second for dense-only embedding of 256-token texts at `batch_size=32`. Verified via `tests/test_throughput.py` (slow marker; opt-in via `pytest -m slow`) running 1000 inference calls and asserting `1000 / total_seconds >= 30`. The benchmark MAY be skipped in default CI and run on a scheduled weekly job, mirroring the cadence of NFR-OBS-001 (`internal/obs/bench/bench_test.go`). |
| NFR-IDX-002 | Performance (warm p50 latency) | The Python sidecar SHALL respond to a `/embed` request containing a single 256-token text with `return_dense=true` and `return_sparse=return_colbert_vecs=false` within p50 ≤ 500 ms wall-clock on a 4 vCPU CPU container with FP32, and within p50 ≤ 100 ms on a single NVIDIA T4-class GPU with FP16. Measured via `tests/test_latency.py::test_p50_latency_cpu` (50 sequential calls; assert `sorted(durations)[25] ≤ 0.5`). The GPU branch is `@pytest.mark.gpu` skipped when `EMBEDDER_DEVICE != cuda:0`. |
| NFR-IDX-003 | Performance (cache-hit fast path) | When a `/embed` request's texts are ALL cache hits (per §2.5), the sidecar SHALL respond within p99 ≤ 5 ms wall-clock on either CPU or GPU configurations. Measured via `tests/test_cache.py::test_cache_hit_latency` (100 sequential identical requests; assert `max(durations[1:]) ≤ 0.005`). The first call is excluded (cache miss) — only subsequent hits are bounded. |
| NFR-IDX-004 | Cache hit ratio in steady state | After a 5-minute warm-up period of representative production traffic (the `tests/test_cache.py::test_steady_state_hit_ratio` simulation: 10000 calls drawn from a Zipf distribution over 1000 unique texts), the cache hit ratio (`hits / (hits + misses)`) SHALL be ≥ 30%. This is a foundation NFR; SPEC-IDX-001's actual production profile may exceed this. The NFR exists to prevent a future "we forgot to wire the cache" regression. |
| NFR-IDX-005 | Race-clean concurrent invocation | `internal/embedder/concurrent_test.go::TestClientEmbedConcurrent` SHALL execute successfully under `go test -race ./internal/embedder/...` with the workload defined in REQ-IDX-002-011: 50 caller goroutines × 100 `Embed` calls each, each call against a stub `httptest.NewServer` returning canned 200 JSON. Race-detector alarms attributable to the embedder package SHALL be zero. Cumulative call count: 5,000 embed invocations. |
| NFR-IDX-006 | Zero goroutine leaks (Go client) | The Go client SHALL pass `goleak.VerifyNone(t)` after every test that invokes `Embed`, including the success path, the timeout path, the retry path, the model-loading 503 path, and the OOM passthrough path. `internal/embedder/bench_test.go::TestMain` SHALL invoke `goleak.VerifyTestMain(m)` (mirrors `internal/adapters/reddit/bench_test.go` pattern). The Go client itself SHALL launch zero detached background goroutines. |
| NFR-IDX-007 | Memory ceiling per worker | The Python sidecar in CPU FP32 configuration SHALL stay under 4 GB resident memory across a 1-hour soak test invoking `/embed` with `texts_count = 32, return_dense+sparse+colbert all true` at 5 RPS. Measured via the `slow` test marker `tests/test_memory.py::test_soak_memory` reading `/proc/self/status` `VmRSS`. The 4 GB ceiling decomposes as: ~3 GB model weights + activations + ~500 MB cache + ~500 MB headroom. GPU FP16 ceiling is 2 GB GPU memory + 1 GB CPU memory; tested via the `@pytest.mark.gpu` branch. |

---

## 4. Acceptance Criteria

### REQ-IDX-002-001 — `/embed` Endpoint Contract

- File `services/embedder/src/embedder/app.py` declares a FastAPI
  application; route `POST /embed` is registered.
- File `services/embedder/src/embedder/models.py` declares
  `EmbedRequest` and `EmbedResponse` Pydantic v2 models with
  `ConfigDict(extra="forbid", str_strip_whitespace=False)`.
- `EmbedRequest` has the 6-field shape: `request_id: str`, `texts:
  list[str]`, `return_dense: bool = True`, `return_sparse: bool =
  False`, `return_colbert_vecs: bool = False`, `batch_size: int = 32`.
- `EmbedResponse` has the 10-field shape: `request_id, dense, sparse,
  colbert, model, model_version, device, latency_ms, cache_hits,
  cache_misses`.
- `test_embed_happy_path`: POST with 3 valid texts returns 200 +
  response with `dense[i]` of length 1024 for `i in {0,1,2}`,
  `cache_hits + cache_misses == 3`.
- `test_embed_extra_field_rejected`: POST with `{..., "unexpected_field":
  1}` returns 422 (FastAPI Pydantic validation).
- `test_embed_response_shape_matches_schema`: returned JSON validates
  against `EmbedResponse.model_json_schema()`.

### REQ-IDX-002-002 — BGE-M3 Inference + Order Preservation

- `test_embed_dense_returns_1024_dim`: any non-empty text → `len(resp.dense[0]) == 1024`.
- `test_embed_response_order_matches_request_order`: post `["foo","bar","baz"]`;
  inspect captured model call args; assert the Python list passed to
  `model.encode` is `["foo","bar","baz"]` in that order; assert
  `resp.dense[0]` corresponds to `"foo"`, etc.
- `test_embed_skipped_when_all_cached`: pre-fill the cache with a
  fixed input; second identical request triggers ZERO model calls
  (mock counts).
- `test_embed_partial_cache_hit_only_misses_inferred`: pre-fill cache
  with text A; post `[A, B]`; assert model call args == `[B]` only.

### REQ-IDX-002-003 — Loading State

- `test_health_returns_503_during_loading`: mock `BGEM3FlagModel`
  initialiser to sleep 1 second; before the lifespan startup
  completes, GET `/health` returns 503 + body
  `{"status":"loading","reason":"model not ready"}`.
- `test_health_returns_200_after_load`: wait for the lifespan startup
  to complete; GET `/health` returns 200 + body
  `{"status":"ok","model":...,"model_version":...,"device":...}`.
- `test_embed_returns_503_during_loading`: same pre-condition as the
  first; POST `/embed` returns 503 + body
  `{"error":"model_loading","detail":"model is still initialising; retry shortly"}`.
- `test_health_records_ready_log`: capture stdout via `capsys`; assert
  exactly one INFO-level `embedder.ready` JSON record with the model
  metadata.

### REQ-IDX-002-004 — Cache

- `test_cache_hit_skips_inference`: pre-fill cache; second identical
  request triggers zero model calls; `cache_hits == 1`,
  `cache_misses == 0`.
- `test_cache_key_includes_mode_flags`: post `text="foo"` with
  `return_dense=true, return_sparse=false`; then post the same text
  with `return_dense=true, return_sparse=true`; assert two cache
  entries (different keys).
- `test_cache_disabled_when_max_entries_zero`: launch app with
  `EMBEDDER_CACHE_MAX_ENTRIES=0`; every request runs inference even
  for repeated text.
- `test_cache_lru_eviction`: launch app with
  `EMBEDDER_CACHE_MAX_ENTRIES=2`; post 3 distinct texts; assert the
  first text is evicted (subsequent identical request misses).
- `TestClientEmitsCacheHitCounter` (Go side): server returns
  `cache_hits=2`; assert `obs.EmbedderCacheHits` += 2.

### REQ-IDX-002-005 — Go-side HTTP Client Behavior

- `TestClientEmbedHappyPath`: `httptest.NewServer` returns canned 200
  JSON; assert returned `Response.Dense` and metadata match.
- `TestClientEmbedTimeout`: server sleeps 30 seconds; client
  configured with 1-second timeout; assert returned error wraps
  `context.DeadlineExceeded` and total elapsed ≤ 1.5 s.
- `TestClientEmbedRetriesOnConnReset`: server resets connection on
  first 2 attempts then returns 200; assert client retries twice (3
  outbound TCP connections observed) and final result is the 200
  body.
- `TestClientEmbed4xxNoRetry`: server returns 400; assert client
  makes exactly 1 outbound call and returns `embedder.ErrInvalidRequest`.
- `TestClientEmbed5xxRetried`: server returns 503 then 200; assert
  retry occurs and success is returned.
- `TestClientEmbedEmitsSingleObservabilityPerCall`: even when 2
  retries occur, `obs.EmbedderCalls` increments only once and
  `obs.EmbedderLatency` records only one observation (the outermost
  call's elapsed time).

### REQ-IDX-002-006 — Per-Call Observability

Python:
- `test_python_log_record_shape`: capture stdout via `capsys`; assert
  exactly 1 JSON line per `/embed` invocation with the 12 documented
  attributes; `outcome` value is one of the 5 enums.
- `test_python_log_redacts_no_text_content`: assert no log record
  contains the verbatim text of any input (privacy bound).

Go:
- `TestClientEmitsCounter`: outcome ∈ `{success, error_invalid,
  error_timeout, error_unreachable, error_oom, error_loading}` each
  fires the matching code path; counter increments by 1 each.
- `TestClientEmitsHistogram`: histogram count == 1 per call, sum > 0.
- `TestClientEmitsCacheHitCounter`: `cache_hits=5` → counter += 5;
  `cache_hits=0` → counter unchanged.
- `TestClientEmitsOTelSpan`: in-memory span exporter captures one span
  named `embedder.call` with the 8 documented attributes.
- `TestClientObservabilitySafeOnNilObs`: construct Client with
  `obs: nil`; call does not panic; returns valid `Response`.
- `TestClientModeLabelDerivation`: `return_dense=true` only → mode
  label `dense`; `return_dense=true,return_sparse=true` → mode label
  `all`; `return_sparse=true` only → `sparse`;
  `return_colbert_vecs=true` only → `colbert`.

### REQ-IDX-002-007 — Invalid Input Rejection

- `test_empty_texts_returns_400`: POST with `texts: []` returns 400 +
  body `{"error":"empty_input","detail":"texts is empty"}`.
- `test_too_many_texts_returns_400`: POST with `texts:
  [str(i) for i in range(257)]` returns 400 + body
  `{"error":"batch_too_large","detail":"len(texts)=257 exceeds 256"}`.
- `test_text_too_long_returns_400`: POST with one text of 100,000
  characters (exceeds 8192 tokens after tokenisation) returns 400 +
  body `{"error":"text_too_long","detail":"text at index 0 exceeds 8192 tokens"}`.
- `test_invalid_input_no_inference`: assert no model call happens for
  any of the three failure cases.
- `test_invalid_input_logs_warn`: captured JSON log records contain
  exactly one WARN entry per failure with `{request_id, error}`.

### REQ-IDX-002-008 — Empty Modes Rejection

- `test_no_modes_requested_returns_400`: POST with `return_dense=false,
  return_sparse=false, return_colbert_vecs=false` returns 400 + body
  `{"error":"empty_modes","detail":"at least one of return_dense, return_sparse, return_colbert_vecs must be true"}`.
- `test_empty_modes_no_inference`: assert no model call happens.

### REQ-IDX-002-009 — OOM Recovery

- `test_oom_returns_500`: mock `model.encode` to raise
  `MemoryError("out of memory")`; POST `/embed` returns 500 + body
  `{"error":"oom","detail":"inference out of memory; retry with smaller batch_size"}`.
- `test_oom_does_not_crash_process`: after raising MemoryError, the
  next `/embed` call against a non-mocked path succeeds (or the
  next call with the mock returning a valid result succeeds).
- `test_oom_logs_error`: captured JSON log contains exactly one ERROR
  record with the captured exception class name.
- `test_oom_increments_counter`: assert `obs.EmbedderCalls{outcome="error_oom"}` += 1.

### REQ-IDX-002-010 — Korean Text Handling

- `test_korean_text_dense_shape`: POST `["안녕하세요"]` returns 200 with
  `len(resp.dense[0]) == 1024`.
- `test_korean_text_passed_verbatim_to_model`: instrument the model
  call to capture args; assert the captured arg is the unchanged
  Korean string.
- `test_mixed_korean_english_succeeds`: POST `["안녕 hello 안녕"]` returns
  200 with valid 1024-dim dense vector.
- `test_korean_no_special_log_field`: assert the slog record has no
  `language`-specific fields beyond what every request emits.

### REQ-IDX-002-011 — Concurrent-Safety

- `TestClientEmbedConcurrent` in `internal/embedder/concurrent_test.go`:
  - Construct one `*Client` against a stub `httptest.NewServer` returning
    canned 200 JSON.
  - Spawn 50 caller goroutines via `sync.WaitGroup` barrier.
  - Each goroutine performs 100 `Embed` calls in a loop.
  - Total: 50 × 100 = 5,000 invocations.
- Assertions:
  1. The test executes successfully under `go test -race ./internal/embedder/...`;
     the race detector reports zero data-race alarms attributable to
     the embedder package.
  2. Every returned `Response` has `len(Dense) > 0`.
  3. `goleak.VerifyNone(t)` at the test's end confirms zero residual
     goroutines.

### REQ-IDX-002-012 — Model Lifecycle

- `test_model_loaded_once_at_startup`: instrument
  `BGEM3FlagModel.__init__` via monkeypatch; run 5 `/embed` calls;
  assert init call count == 1.
- `test_model_freed_at_shutdown`: trigger FastAPI lifespan shutdown
  (`async with TestClient(app) as c:` → exits the context); inspect
  app state to assert the model reference is released (the slot is
  None or absent).
- `test_model_load_log_record`: capture stdout; assert exactly one
  INFO-level `embedder.model_loaded` record with the 5 documented
  attributes.

### REQ-IDX-002-013 — Model Version Pin (P1)

- `test_model_version_pinned`: set env
  `EMBEDDER_MODEL_VERSION=abc123def`; mock the model loader to
  capture kwargs; assert `revision="abc123def"` was passed.
- `test_model_version_latest_default`: unset env; mock the loader
  capturing kwargs; assert `revision` is either absent or `"latest"`
  (run-phase author chooses the default).
- `test_model_version_in_response`: assert `EmbedResponse.model_version`
  matches the resolved value.

### NFR-IDX-001 — CPU Throughput

- `tests/test_throughput.py::test_throughput_cpu_dense_only` (slow):
  1000 sequential `/embed` calls each with a 256-token text;
  assert `1000 / total_seconds >= 30` on a 4-vCPU container.
- Marked `@pytest.mark.slow`; default CI run skips; weekly scheduled
  job runs.

### NFR-IDX-002 — Warm p50 Latency

- `tests/test_latency.py::test_p50_latency_cpu`: 50 sequential `/embed`
  calls with a single 256-token text; assert `sorted(durations)[25] ≤ 0.5`
  on CPU.
- `tests/test_latency.py::test_p50_latency_gpu` (gpu marker): same
  shape; assert `sorted(durations)[25] ≤ 0.1` on GPU.

### NFR-IDX-003 — Cache-Hit Fast Path

- `tests/test_cache.py::test_cache_hit_latency`: 100 sequential
  identical `/embed` requests; first is excluded; assert
  `max(durations[1:]) ≤ 0.005` (5 ms).

### NFR-IDX-004 — Steady-State Hit Ratio

- `tests/test_cache.py::test_steady_state_hit_ratio`: simulate 10000
  calls drawn from `numpy.random.zipf(a=1.5, size=10000)` mapped to
  1000 unique texts; assert `cache_hits / 10000 >= 0.30`.

### NFR-IDX-005 — Race-Clean Concurrent Workload

- `TestClientEmbedConcurrent` (REQ-IDX-002-011 acceptance) executes
  under `go test -race`; race-detector alarms attributable to the
  embedder package = 0.

### NFR-IDX-006 — Zero Goroutine Leaks (Go client)

- `TestMain` in `internal/embedder/bench_test.go`:
  ```
  func TestMain(m *testing.M) {
      goleak.VerifyTestMain(m)
  }
  ```
  Mirrors `internal/adapters/reddit/bench_test.go`.
- Every `Embed`-invoking test SHALL pass `goleak.VerifyNone(t)` at
  its end.

### NFR-IDX-007 — Memory Ceiling

- `tests/test_memory.py::test_soak_memory_cpu` (slow): 1-hour soak
  test invoking `/embed` with `texts_count=32, all_modes=true` at 5
  RPS; sample `/proc/self/status` VmRSS every 30 seconds; assert
  `max(VmRSS_MB) ≤ 4096`.
- `tests/test_memory.py::test_soak_memory_gpu` (slow + gpu): same
  shape; assert `max(GPU_MEM_MB) ≤ 2048` and
  `max(VmRSS_MB) ≤ 1024`.

---

## 5. Technical Approach

### 5.1 Files to Create

**Python sidecar** (`services/embedder/`):

- `src/embedder/__main__.py` — `python -m embedder` invokes
  `uvicorn.run(app, host="0.0.0.0", port=int(os.getenv("EMBEDDER_PORT","8082")))`.
- `src/embedder/app.py` — FastAPI app with `lifespan` async context
  manager that loads the model + initialises the cache; routes:
  `POST /embed`, `GET /health`.
- `src/embedder/models.py` — Pydantic v2 models (`EmbedRequest`,
  `EmbedResponse`).
- `src/embedder/embed.py` — `class Embedder`: thin wrapper around
  `BGEM3FlagModel` exposing `async embed(texts, return_dense,
  return_sparse, return_colbert_vecs, batch_size) -> tuple[dense,
  sparse, colbert]`. Handles OOM, per-token-length validation,
  Korean passthrough.
- `src/embedder/cache.py` — `class EmbedderCache`: bounded LRU
  (`cachetools.LRUCache(maxsize=N)`); provides `get(key) ->
  CachedValue | None`, `put(key, value)`, `key_for(text,
  model_name, model_version, mode_flags) -> str`.
- `src/embedder/obs.py` — JSON-formatted stdlib `logging` setup +
  `class Timer` context manager + `log_embed(record: dict)` helper.
- `src/embedder/__init__.py` — replace empty stub with package doc +
  `__version__`.
- `tests/test_app.py` — FastAPI `TestClient` covering REQ-IDX-002-001/003/004/006/007/008/010.
- `tests/test_embed.py` — `Embedder` wrapper logic, OOM, length
  validation (REQ-IDX-002-002, REQ-IDX-002-009).
- `tests/test_cache.py` — LRU cache hit/miss/eviction +
  NFR-IDX-003/004 (REQ-IDX-002-004).
- `tests/test_models.py` — Pydantic v2 validation table.
- `tests/test_obs.py` — JSON log record shape (REQ-IDX-002-006).
- `tests/test_throughput.py` (slow) — NFR-IDX-001.
- `tests/test_latency.py` (slow / gpu) — NFR-IDX-002.
- `tests/test_memory.py` (slow / gpu) — NFR-IDX-007.

**Go-side**:

- `internal/embedder/types.go` — `Request`, `Response`, `Embedding`,
  error sentinels (`ErrInvalidRequest`, `ErrSidecarUnreachable`,
  `ErrTimeout`, `ErrModelLoadFailed`, `ErrOutOfMemory`).
- `internal/embedder/config.go` — env loader for
  `EMBEDDER_BASE_URL` (default `http://localhost:8082`) and
  `EMBEDDER_REQUEST_TIMEOUT_SECONDS` (default `15`).
- `internal/embedder/client.go` — `Client` struct + `New(cfg, *obs.Obs)`
  + `Embed(ctx, req) (Response, error)`.
- `internal/embedder/embedder.go` — package doc + value-type re-exports.
- `internal/embedder/client_test.go` — REQ-IDX-002-005 +
  REQ-IDX-002-006 Go-side tests against `httptest.NewServer`
  returning canned JSON.
- `internal/embedder/concurrent_test.go` — NFR-IDX-005 race workload.
- `internal/embedder/bench_test.go` — `TestMain` with
  `goleak.VerifyTestMain` (NFR-IDX-006).
- `internal/obs/metrics/embedder.go` — `EmbedderCalls`,
  `EmbedderLatency`, `EmbedderCacheHits` collectors;
  `registerEmbedder(pr)` helper. Mirrors
  `internal/obs/metrics/synthesis.go:1-60`.

**Modified**:

- `services/embedder/pyproject.toml` — runtime deps additions per §2.1(b).
- `services/embedder/Dockerfile` — multi-stage rebuild + HEALTHCHECK +
  EXPOSE 8082; copy of `services/researcher/Dockerfile:1-30` shape
  with `services/embedder/` swapped in.
- `services/embedder/.env.example` — additional keys (port, device,
  fp16, max length, cache max, model version).
- `deploy/docker-compose.yml` — new `embedder` service entry;
  `embedder_models` named volume.
- `deploy/docker-compose.gpu.yml` — NEW file; GPU overlay.
- `internal/obs/metrics/metrics.go` — register embedder collectors +
  add `mode` to labelNames allowlist.
- `internal/obs/obs.go` — re-export `obs.EmbedderCalls`,
  `obs.EmbedderLatency`, `obs.EmbedderCacheHits`.
- `.env.example` — append embedder env vars.

### 5.2 Embedding Inference Algorithm

```
inputs:  texts: list[str], return_dense: bool, return_sparse: bool,
         return_colbert_vecs: bool, batch_size: int
output:  dense: list[list[float]] | None, sparse: list[dict] | None,
         colbert: list[list[list[float]]] | None, cache_hits: int,
         cache_misses: int

1. Validate:
   - len(texts) ∈ [1, 256]; else 400 (REQ-IDX-002-007).
   - At least one mode true; else 400 (REQ-IDX-002-008).
   - Each text token-length ≤ 8192 (fast pre-pass via tokenizer);
     else 400 (REQ-IDX-002-007).

2. Compute mode_flags string (e.g., "d=1,s=1,c=0").

3. Cache lookup per text:
   for i, text in enumerate(texts):
       key = cache.key_for(text.strip(), model_name, model_version, mode_flags)
       cached = cache.get(key)
       if cached:
           hits[i] = cached
           cache_hits += 1
       else:
           miss_indices.append(i)
           cache_misses += 1

4. Inference for cache misses only:
   if miss_indices:
       miss_texts = [texts[i] for i in miss_indices]
       try:
           result = model.encode(
               miss_texts,
               batch_size=batch_size,
               max_length=EMBEDDER_MAX_LENGTH,
               return_dense=return_dense,
               return_sparse=return_sparse,
               return_colbert_vecs=return_colbert_vecs,
           )
       except (MemoryError, torch.cuda.OutOfMemoryError) as e:
           return 500 + {"error": "oom", ...} (REQ-IDX-002-009)
       # Insert into cache
       for j, i in enumerate(miss_indices):
           cache.put(key_for(texts[i].strip(), ...), result_at(j))

5. Reassemble in request order:
   for i in range(len(texts)):
       if i in hits: pull from hits[i]
       else: pull from result at (j corresponding to i)

6. Return EmbedResponse.
```

### 5.3 Go HTTP Client Sketch

```go
// internal/embedder/client.go (sketch — final shape in run phase;
// mirrors internal/synthesis/client.go:28-280)

type Client struct {
    httpClient *http.Client
    baseURL    string
    o          *obs.Obs
}

func New(cfg Config, o *obs.Obs) (*Client, error) {
    return &Client{
        httpClient: &http.Client{Timeout: cfg.RequestTimeout},
        baseURL:    cfg.BaseURL,
        o:          o,
    }, nil
}

func (c *Client) Embed(ctx context.Context, req Request) (Response, error) {
    ctx, cancel := context.WithTimeout(ctx, c.httpClient.Timeout)
    defer cancel()

    var span oteltrace.Span
    if c.o != nil {
        ctx, span = c.o.Tracer("embedder").Start(ctx, "embedder.call")
        defer span.End()
    }

    started := time.Now()
    outcome := "error_unreachable"
    mode := deriveMode(req)  // dense / sparse / colbert / all

    defer func() {
        elapsed := time.Since(started)
        c.emitObs(ctx, span, outcome, mode, elapsed, req, Response{})
    }()

    body, err := json.Marshal(req)
    if err != nil {
        outcome = "error_invalid"
        return Response{}, fmt.Errorf("embedder: marshal: %w", err)
    }

    var resp Response
    err = withRetry(ctx, 2, func() error {
        return c.doOnce(ctx, body, &resp)
    })

    switch {
    case errors.Is(err, context.DeadlineExceeded):
        outcome = "error_timeout"
        return Response{}, fmt.Errorf("embedder: %w: %w", ErrTimeout, err)
    case errors.Is(err, ErrInvalidRequest):
        outcome = "error_invalid"
        return Response{}, err
    case errors.Is(err, ErrModelLoading):
        outcome = "error_loading"
        return Response{}, err
    case errors.Is(err, ErrOutOfMemory):
        outcome = "error_oom"
        return Response{}, err
    case err != nil:
        outcome = "error_unreachable"
        return Response{}, err
    default:
        outcome = "success"
    }

    if outcome == "success" && resp.CacheHits > 0 && c.o != nil {
        if c.o.Metrics != nil && c.o.Metrics.EmbedderCacheHits != nil {
            c.o.Metrics.EmbedderCacheHits.Add(float64(resp.CacheHits))
        }
    }

    return resp, nil
}
```

### 5.4 Compose Service Entry

```yaml
# deploy/docker-compose.yml — additions
volumes:
  # ... existing volumes ...
  embedder_models: {}

services:
  # ... existing services ...
  embedder:
    build:
      context: ../services/embedder
    ports:
      - "${EMBEDDER_PORT:-8082}:8082"
    environment:
      EMBEDDER_PORT: "8082"
      EMBEDDER_MODEL: ${EMBEDDER_MODEL:-BAAI/bge-m3}
      EMBEDDER_MODEL_VERSION: ${EMBEDDER_MODEL_VERSION:-latest}
      EMBEDDER_DEVICE: ${EMBEDDER_DEVICE:-cpu}
      EMBEDDER_USE_FP16: ${EMBEDDER_USE_FP16:-false}
      EMBEDDER_BATCH_SIZE: ${EMBEDDER_BATCH_SIZE:-32}
      EMBEDDER_MAX_LENGTH: ${EMBEDDER_MAX_LENGTH:-8192}
      EMBEDDER_CACHE_MAX_ENTRIES: ${EMBEDDER_CACHE_MAX_ENTRIES:-10000}
      EMBEDDER_LOG_LEVEL: ${EMBEDDER_LOG_LEVEL:-INFO}
    volumes:
      - embedder_models:/home/appuser/.cache/huggingface
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8082/health"]
      interval: 30s
      timeout: 5s
      retries: 5
      start_period: 60s   # Model load can take 30s on first boot
    mem_limit: 4g
    restart: unless-stopped
    networks:
      - app
```

```yaml
# deploy/docker-compose.gpu.yml — NEW overlay
services:
  embedder:
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: 1
              capabilities: [gpu]
    environment:
      EMBEDDER_DEVICE: cuda:0
      EMBEDDER_USE_FP16: "true"
```

GPU run: `docker compose -f docker-compose.yml -f docker-compose.gpu.yml up`.

### 5.5 Harness Level

Per `.moai/config/sections/harness.yaml` auto-routing: 13 REQs (12 ×
P0 + 1 × P1) + 7 NFRs across 2 languages (Python sidecar + Go client)
+ 1 new compose service + 1 new compose overlay + new metric family
with 1 new label name = **standard** harness. Sprint Contract optional
but recommended.

---

## 6. File Impact

### 6.1 Created

| Path | Purpose |
|------|---------|
| `services/embedder/src/embedder/app.py` | FastAPI app + routes (REQ-IDX-002-001) |
| `services/embedder/src/embedder/models.py` | Pydantic v2 models (REQ-IDX-002-001) |
| `services/embedder/src/embedder/embed.py` | BGEM3FlagModel wrapper (REQ-IDX-002-002, REQ-IDX-002-009, REQ-IDX-002-012) |
| `services/embedder/src/embedder/cache.py` | LRU cache + key derivation (REQ-IDX-002-004, NFR-IDX-003, NFR-IDX-004) |
| `services/embedder/src/embedder/obs.py` | JSON log records (REQ-IDX-002-006) |
| `services/embedder/src/embedder/__main__.py` | Uvicorn entrypoint |
| `services/embedder/tests/test_app.py` | Endpoint contract tests |
| `services/embedder/tests/test_embed.py` | Inference + OOM + length validation |
| `services/embedder/tests/test_cache.py` | LRU cache + steady-state hit ratio |
| `services/embedder/tests/test_models.py` | Pydantic validation |
| `services/embedder/tests/test_obs.py` | Log record shape |
| `services/embedder/tests/test_throughput.py` | NFR-IDX-001 (slow) |
| `services/embedder/tests/test_latency.py` | NFR-IDX-002 (slow / gpu) |
| `services/embedder/tests/test_memory.py` | NFR-IDX-007 (slow / gpu) |
| `internal/embedder/types.go` | Go value types + error sentinels |
| `internal/embedder/config.go` | Env binding |
| `internal/embedder/client.go` | Go HTTP client (REQ-IDX-002-005, REQ-IDX-002-006) |
| `internal/embedder/embedder.go` | Package doc + value-type re-exports |
| `internal/embedder/client_test.go` | Go client tests |
| `internal/embedder/concurrent_test.go` | NFR-IDX-005 race workload |
| `internal/embedder/bench_test.go` | TestMain with goleak (NFR-IDX-006) |
| `internal/obs/metrics/embedder.go` | New metric family declaration |
| `deploy/docker-compose.gpu.yml` | GPU overlay (D3 / Open Q §11.2) |

### 6.2 Modified

| Path | Change |
|------|--------|
| `services/embedder/src/embedder/__init__.py` | Replace empty stub with package doc + version |
| `services/embedder/pyproject.toml` | Add runtime deps (fastapi, uvicorn, pydantic, FlagEmbedding, numpy, cachetools) |
| `services/embedder/Dockerfile` | Add HEALTHCHECK + EXPOSE 8082 + multi-stage build |
| `services/embedder/.env.example` | Add port, device, fp16, max-length, cache-max, model-version |
| `internal/obs/metrics/metrics.go` | Register embedder collectors in `NewRegistry()`; append `mode` to labelNames |
| `internal/obs/obs.go` | Add `EmbedderCalls`, `EmbedderLatency`, `EmbedderCacheHits` re-exports |
| `deploy/docker-compose.yml` | Add `embedder` service + `embedder_models` volume |
| `.env.example` | Append `EMBEDDER_*` env vars |

### 6.3 Unchanged (by design)

- `internal/llm/*` — embedder does NOT consume the Go-side LLM client;
  this is a model-inference service, not an LLM-routing service.
- `internal/synthesis/*` — synthesis is independent of embedder; the
  synthesis path does NOT call into embedder in v0.1.
- `internal/router/*`, `internal/adapters/*`, `internal/fanout/*` — no
  API change; embedder is downstream of fanout (SPEC-IDX-001 calls
  embedder after fanout returns).
- `pkg/types/*` — no public types added (`pkg/types` SDK boundary
  preserved per `.moai/project/structure.md:160-165`).
- `deploy/litellm/config.yaml` — embedder does NOT route through
  LiteLLM (BGE-M3 is a local model, not an LLM API call).

---

## 7. Test Plan

Development follows RED-GREEN-REFACTOR per `quality.development_mode:
tdd`. Coverage target 85% per SPEC frontmatter (Python target ≥ 85%
per the project's `quality.test_coverage_target` and per
`.claude/rules/moai/languages/python.md` testing section).

Representative RED-phase tests (in addition to the acceptance criteria
already enumerated in §4):

| # | Test | Layer | REQ | Assertion |
|---|------|-------|-----|-----------|
| 1 | `test_embed_happy_path` | `tests/test_app.py` | REQ-IDX-002-001 | 200 + valid response shape |
| 2 | `test_embed_extra_field_rejected` | `tests/test_app.py` | REQ-IDX-002-001 | 422 on unknown field |
| 3 | `test_embed_dense_returns_1024_dim` | `tests/test_app.py` | REQ-IDX-002-002 | `len(resp.dense[0]) == 1024` |
| 4 | `test_embed_response_order_matches_request_order` | `tests/test_app.py` | REQ-IDX-002-002 | Vectors at index i correspond to texts at index i |
| 5 | `test_embed_skipped_when_all_cached` | `tests/test_cache.py` | REQ-IDX-002-002, REQ-IDX-002-004 | Zero model calls when fully cached |
| 6 | `test_embed_partial_cache_hit_only_misses_inferred` | `tests/test_cache.py` | REQ-IDX-002-004 | Only missed texts inferred |
| 7 | `test_health_returns_503_during_loading` | `tests/test_app.py` | REQ-IDX-002-003 | 503 + body when model not ready |
| 8 | `test_health_returns_200_after_load` | `tests/test_app.py` | REQ-IDX-002-003 | 200 + body after load |
| 9 | `test_embed_returns_503_during_loading` | `tests/test_app.py` | REQ-IDX-002-003 | 503 + body for /embed during load |
| 10 | `test_health_records_ready_log` | `tests/test_app.py` | REQ-IDX-002-003 | Single embedder.ready INFO log |
| 11 | `test_cache_hit_skips_inference` | `tests/test_cache.py` | REQ-IDX-002-004 | Second identical call → 0 inference |
| 12 | `test_cache_key_includes_mode_flags` | `tests/test_cache.py` | REQ-IDX-002-004 | Different modes → different cache slots |
| 13 | `test_cache_disabled_when_max_entries_zero` | `tests/test_cache.py` | REQ-IDX-002-004 | Every call infers when max=0 |
| 14 | `test_cache_lru_eviction` | `tests/test_cache.py` | REQ-IDX-002-004 | First text evicted at capacity |
| 15 | `test_empty_texts_returns_400` | `tests/test_app.py` | REQ-IDX-002-007 | 400 + error=empty_input |
| 16 | `test_too_many_texts_returns_400` | `tests/test_app.py` | REQ-IDX-002-007 | 400 + error=batch_too_large |
| 17 | `test_text_too_long_returns_400` | `tests/test_app.py` | REQ-IDX-002-007 | 400 + error=text_too_long |
| 18 | `test_no_modes_requested_returns_400` | `tests/test_app.py` | REQ-IDX-002-008 | 400 + error=empty_modes |
| 19 | `test_oom_returns_500` | `tests/test_embed.py` | REQ-IDX-002-009 | 500 + error=oom |
| 20 | `test_oom_does_not_crash_process` | `tests/test_embed.py` | REQ-IDX-002-009 | Subsequent call succeeds |
| 21 | `test_korean_text_dense_shape` | `tests/test_embed.py` | REQ-IDX-002-010 | 1024-dim for "안녕하세요" |
| 22 | `test_korean_text_passed_verbatim_to_model` | `tests/test_embed.py` | REQ-IDX-002-010 | Korean string unchanged in model call |
| 23 | `test_mixed_korean_english_succeeds` | `tests/test_embed.py` | REQ-IDX-002-010 | Mixed-script input → valid 1024-dim |
| 24 | `test_python_log_record_shape` | `tests/test_obs.py` | REQ-IDX-002-006 | 1 JSON log per call, 12 attrs |
| 25 | `test_model_loaded_once_at_startup` | `tests/test_app.py` | REQ-IDX-002-012 | init call count == 1 across requests |
| 26 | `test_model_freed_at_shutdown` | `tests/test_app.py` | REQ-IDX-002-012 | Reference released on shutdown |
| 27 | `test_model_version_pinned` | `tests/test_embed.py` | REQ-IDX-002-013 | revision kwarg propagated |
| 28 | `TestClientEmbedHappyPath` | `internal/embedder/client_test.go` | REQ-IDX-002-005 | 200 JSON parsed into Response |
| 29 | `TestClientEmbedTimeout` | `client_test.go` | REQ-IDX-002-005 | Server sleeps; client times out |
| 30 | `TestClientEmbedRetriesOnConnReset` | `client_test.go` | REQ-IDX-002-005 | 2 retries on conn reset |
| 31 | `TestClientEmbed4xxNoRetry` | `client_test.go` | REQ-IDX-002-005 | 400 → no retry, ErrInvalidRequest |
| 32 | `TestClientEmbed5xxRetried` | `client_test.go` | REQ-IDX-002-005 | 503 then 200 → success |
| 33 | `TestClientEmbedEmitsSingleObservabilityPerCall` | `client_test.go` | REQ-IDX-002-005, REQ-IDX-002-006 | Counter +1, histogram +1 even on retry |
| 34 | `TestClientEmitsCounter` | `client_test.go` | REQ-IDX-002-006 | Each outcome enum increments correctly |
| 35 | `TestClientEmitsHistogram` | `client_test.go` | REQ-IDX-002-006 | count == 1 per call, sum > 0 |
| 36 | `TestClientEmitsCacheHitCounter` | `client_test.go` | REQ-IDX-002-006 | cache_hits=N → counter += N |
| 37 | `TestClientEmitsOTelSpan` | `client_test.go` | REQ-IDX-002-006 | span name + 8 attrs |
| 38 | `TestClientObservabilitySafeOnNilObs` | `client_test.go` | REQ-IDX-002-006 | obs: nil → no panic |
| 39 | `TestClientModeLabelDerivation` | `client_test.go` | REQ-IDX-002-006 | dense/sparse/colbert/all |
| 40 | `TestClientEmbedConcurrent` | `concurrent_test.go` | REQ-IDX-002-011, NFR-IDX-005 | 50 × 100 race-clean |
| 41 | `test_throughput_cpu_dense_only` | `tests/test_throughput.py` | NFR-IDX-001 | ≥30 docs/sec on 4 vCPU |
| 42 | `test_p50_latency_cpu` | `tests/test_latency.py` | NFR-IDX-002 | p50 ≤ 500 ms CPU |
| 43 | `test_p50_latency_gpu` | `tests/test_latency.py` (gpu) | NFR-IDX-002 | p50 ≤ 100 ms GPU |
| 44 | `test_cache_hit_latency` | `tests/test_cache.py` | NFR-IDX-003 | p99 ≤ 5 ms cache hits |
| 45 | `test_steady_state_hit_ratio` | `tests/test_cache.py` | NFR-IDX-004 | ≥ 30% hit rate after warm-up |
| 46 | `test_soak_memory_cpu` | `tests/test_memory.py` (slow) | NFR-IDX-007 | VmRSS ≤ 4 GB after 1h soak |

Python: `pytest -q services/embedder/tests/` with `pytest-asyncio`
(`asyncio_mode="auto"`) and FastAPI `TestClient`. Coverage via
`pytest --cov=embedder --cov-report=term-missing`, target 85%.

Go: `go test -race ./internal/embedder/...` against
`httptest.NewServer` returning canned JSON. Coverage via `go test
-coverprofile=...`, target 85%.

Greenfield note: `services/embedder/src/embedder/` is empty;
`internal/embedder/` does not exist. Per `workflow-modes.md` §
Brownfield Enhancement, characterization tests are not needed; RED
tests are written against the planned package surface.

---

## 8. Dependencies

### 8.1 Upstream SPEC Dependencies

- **SPEC-BOOT-001 (implemented)**: provides `services/embedder/`
  scaffold (`pyproject.toml`, `Dockerfile`, `__init__.py`,
  `tests/test_version.py`) per spec.md:99-105 + REQ-BOOT-002.
- **SPEC-OBS-001 (implemented)**: provides `obs.Logger`, `obs.Tracer`,
  metric registry, the `internal/obs/metrics/` location pattern, and
  the cardinality allowlist mechanism. SPEC-IDX-002 adds
  `internal/obs/metrics/embedder.go` following the
  `internal/obs/metrics/synthesis.go` precedent.

### 8.2 Coordinating SPECs (no hard dependency)

- **SPEC-IDX-001 (M3, parallelizable)**: consumes
  `internal/embedder.Client.Embed` to populate the hybrid index. No
  hard code dependency in either direction during plan/research phase;
  IDX-001 can begin its plan phase as soon as IDX-002's spec.md is
  approved.
- **SPEC-IDX-003 (M3, parallelizable)**: Korean tokenization is
  independent — IDX-002's BGE-M3 internal tokenization handles Korean
  text natively per REQ-IDX-002-010; IDX-003 concerns Meilisearch's
  lexical tokenizer for keyword search.

### 8.3 Downstream Blocked SPECs

- **SPEC-IDX-001 (M3)**: hybrid index ingestion needs dense vectors
  from this SPEC.
- **SPEC-CACHE-001 (M3)**: 5-phase access fallback wraps fanout +
  ingestion; embedder availability is an indirect dependency.

### 8.4 External Dependencies (run-phase pins)

New Python runtime dependencies:

```
fastapi >= 0.115
uvicorn[standard] >= 0.30
pydantic >= 2.9
FlagEmbedding >= 1.3.0    # BAAI official; MIT license; provides BGEM3FlagModel
numpy >= 1.26
cachetools >= 5.5         # LRUCache for in-process cache (research §4.1)
```

Transitively pulled by `FlagEmbedding`:

- `torch >= 2.4` (PyTorch CPU or CUDA wheel)
- `transformers >= 4.40`
- `sentence-transformers >= 3.0` (used internally by FlagEmbedding)
- `huggingface_hub >= 0.24`

The actual transitive set is determined by `pip install FlagEmbedding`
and locked at run-phase via `uv lock` or `pip freeze` per
NFR-OBS-003 reproducibility convention.

No new Go module dependencies (uses stdlib `net/http`,
`encoding/json`, `crypto/sha256`, `errors`).

No new external services at SPEC-IDX-002 runtime (the embedder is a
self-contained process; it does NOT call LiteLLM, does NOT call
Hugging Face Hub at request time — only at startup model load).

---

## 9. Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| `FlagEmbedding` package pulls a heavyweight transitive dep tree (full PyTorch + transformers + sentence-transformers) inflating Docker image > 4 GB | High | Medium | Open Question §11.1: ONNX export path is the future optimisation. Multi-stage Dockerfile separates build deps from runtime; pip cache pruned. |
| Model download (~1.4 GB) fails on first boot due to network or HF Hub outage | Medium | High | Healthcheck reports 503 with `loading` status; restart loop retries; `EMBEDDER_MODEL_VERSION` pin to a known-good commit reduces stale-tag drift |
| OOM on CPU when batch_size + max_length too high | Medium | Medium | REQ-IDX-002-009 OOM recovery path; documented `EMBEDDER_BATCH_SIZE` default 32 (conservative); per-request batch cap at 256 (REQ-IDX-002-007) |
| Cache key collision across BGE-M3 versions (silent stale vectors) | Low | High | Cache key includes `model_version` (resolved HF commit hash); version change invalidates cache automatically |
| Concurrent `/embed` calls serialise on the GIL (no compute parallelism within a single uvicorn worker) | High (by design) | Low | Documented as Decision D6; multi-replica scaling is the answer (M9 Helm chart concern) |
| GPU memory leak when worker exits abnormally | Low | Low | `lifespan` shutdown hook explicitly frees `model.cpu(); del model`; Docker process termination releases GPU memory at the OS level |
| ColBERT mode response payloads are 50× larger than dense (storage at the caller side) | Medium | Medium | Default `return_colbert_vecs=false`; SPEC-IDX-001 chooses; documented in §2.3 |
| Sparse mode response uses string keys for token IDs (JSON serialisation choice) | Low | Low | Documented in §2.3; SPEC-IDX-001 deserialises as `dict[str, float]` and converts to `int` keys at consumption time |
| Korean text breaks BGE-M3's XLM-RoBERTa tokenizer in unexpected ways | Low | Medium | Verified by HF model card (multilingual incl. Korean); REQ-IDX-002-010 acceptance test covers; SPEC-EVAL-003 (M8) will measure faithfulness |
| Cache hit ratio NFR-IDX-004 (30%) is too aggressive for early M3 traffic | Medium | Low | NFR is a *foundation* — prevents regression. Run-phase author can lower if measured workload is too varied |
| Healthcheck poll storm during slow model load saturates the FastAPI event loop | Low | Low | `lifespan` startup loads model BEFORE accepting connections; FastAPI returns 503 immediately during the unbounded-poll window |
| Pydantic v2 `extra="forbid"` rejects benign client additions in future | Low | Low | Documented; client and server evolve together; backward compat is checked in run-phase contract tests |
| Go client retry + sidecar internal retry cause 9× call amplification | Low | Medium | Sidecar does NOT retry the model call (model is local; transient errors are OOM or model-load); Go client retries only on connection-level errors, not on HTTP 5xx |
| `mode` label cardinality blow-up (4 values × 5 outcomes = 20 series) | Low | Low | Bounded enum; `mode` is added to allowlist explicitly; 20-series cardinality is well within Prometheus comfort zone |
| Pre-flight tokenizer call for length validation adds ~1 ms per text overhead | Medium | Low | Runs on the cache-miss path only; cached requests skip; deferred to inference-time error handling is an alternative (Open Q §8.5 in research) |

---

## 10. Open Questions

The following are explicitly UNRESOLVED at SPEC-approval time. Each
has a recommended default and a one-line resolution owner. They do
NOT block SPEC approval.

1. **BGE-M3 inference path — FlagEmbedding vs ONNX vs vLLM.** Default:
   FlagEmbedding (research §2.1). Image bloat is acceptable in M3.
   ONNX path deferred to SPEC-IDX-002a; vLLM path deferred to
   SPEC-IDX-002b. Resolution owner: SPEC-IDX-002 run-phase implementer.

2. **GPU vs CPU default.** Default: CPU. GPU is opt-in via
   `deploy/docker-compose.gpu.yml`. Resolution owner: SPEC-DEPLOY-001
   (M9 Helm chart) author may flip for cloud profiles.

3. **Multi-vector default — return all three modes or only dense.**
   Default: per-request flags; default is dense-only. Resolution
   owner: SPEC-IDX-001 author chooses for the hybrid index.

4. **Cache backend — in-process LRU vs Redis vs disk.** Default:
   in-process LRU. Resolution owner: future SPEC-IDX-002b if
   multi-instance cache sharing measurably matters.

5. **Per-request batch cap.** Default: 256. Open: lower to 64 to keep
   p99 latency bounded? Resolution owner: run-phase author after
   NFR-IDX-002 baseline.

6. **Concurrent uvicorn workers.** Default: 1. Concurrency via async
   event loop; compute-parallel is via separate replicas. Resolution
   owner: SPEC-DEPLOY-001 (M9 Helm chart) author.

7. **Model version pinning.** Default: pin via
   `EMBEDDER_MODEL_VERSION` env to a specific HF commit hash;
   `latest` resolves at boot. Resolution owner: SPEC-IDX-002 run-phase
   author selects an exact pin.

8. **Healthcheck depth.** Default: shallow `GET /health` (model
   loaded? FastAPI alive? → 200). Open: include a self-test inference
   on a known input? Resolution owner: SPEC-IDX-002 run-phase author;
   default to shallow + separate `/healthcheck/deep` for self-tests.

---

## 11. References

Internal:

- `services/embedder/pyproject.toml:1-33` — current scaffold.
- `services/embedder/Dockerfile:1-22` — current single-stage build.
- `services/embedder/.env.example:1-11` — current env vars.
- `services/researcher/pyproject.toml:1-46` — precedent runtime deps shape.
- `services/researcher/Dockerfile:1-30` — precedent multi-stage build.
- `services/researcher/src/researcher/` — package layout precedent.
- `internal/synthesis/client.go:28-280` — Go HTTP client precedent
  (retry, observability, nil-safe Obs).
- `internal/synthesis/types.go:9-68` — Go-side types precedent.
- `internal/synthesis/config.go:14-50` — env binding precedent.
- `internal/obs/metrics/metrics.go:55-178` — Registry struct +
  registration pattern.
- `internal/obs/metrics/synthesis.go:1-60` — registerSynthesis
  precedent for new metric family.
- `internal/obs/metrics/metrics.go:169-176` — labelNames cardinality
  allowlist.
- `pkg/types/normalized_doc.go:40-56` — NormalizedDoc shape (NOT
  consumed by SPEC-IDX-002 directly).
- `deploy/docker-compose.yml:165-188` — `researcher` service entry
  precedent.
- `.env.example:77-87` — `RESEARCHER_*` env vars precedent.
- `.moai/project/tech.md:39-50` — BGE-M3 lock decision.
- `.moai/project/tech.md:144-153` — Korean tokenizer risk delegation.
- `.moai/project/structure.md:160-165` — `pkg/types` SDK boundary.
- `.moai/project/roadmap.md:55-58` — M3 placement.
- `.moai/project/roadmap.md:122-128` — M3 parallelization plan.
- `.moai/specs/SPEC-BOOT-001/spec.md:99-105` — services/embedder/
  scaffold provisioning.
- `.moai/specs/SPEC-OBS-001/spec.md:88-94` — observability bundle +
  cardinality allowlist precedent.
- `.moai/specs/SPEC-CORE-001/spec.md:139-146` — 5-value `outcome`
  enum reused.
- `.moai/specs/SPEC-SYN-001/spec.md` — Python sidecar precedent.
- `.moai/specs/SPEC-FAN-001/spec.md` — Go client concurrent-safety
  precedent.
- `.moai/specs/SPEC-IDX-002/research.md` — companion research artifact.

External (URL-cited; verified 2026-05-04):

- https://huggingface.co/BAAI/bge-m3 — BGE-M3 model card; dense dim
  1024; max length 8192; MIT license; FP16 supported; multilingual
  incl. Korean.
- https://github.com/FlagOpen/FlagEmbedding — Official BAAI repo;
  MIT-licensed; pip package `FlagEmbedding`; primary class
  `BGEM3FlagModel`.
- https://github.com/FlagOpen/FlagEmbedding/blob/master/research/BGE_M3/README.md
  — BGEM3FlagModel API: `BGEM3FlagModel(model_name, use_fp16=True,
  devices=['cuda:0'])`; `encode(sentences, batch_size, max_length,
  return_dense, return_sparse, return_colbert_vecs)`.
- https://onnxruntime.ai/docs/get-started/with-python.html — ONNX
  Runtime alternative path (research §2.2; deferred).
- https://www.uvicorn.org/ — ASGI server for FastAPI; default
  workers=1.
- https://docs.vllm.ai/en/latest/ — vLLM alternative path (research
  §2.3; deferred); BGE-M3 not specifically listed.

---

*End of SPEC-IDX-002 v0.1 (draft)*
