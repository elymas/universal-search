# SPEC-IDX-002 Implementation Plan (Post-Hoc)

Generated: 2026-05-26 (reverse-engineered from implemented code)
Methodology: TDD (RED-GREEN-REFACTOR) — completed 2026-05-08
Coverage target: 85% (achieved: Python sidecar 93%, Go client 89.3%)
Harness: standard
Status: implemented (verified against `services/embedder/` + `internal/embedder/`)

---

## 1. Overview

This plan.md is a post-hoc summary of the SPEC-IDX-002 implementation
that already shipped (status: `implemented`, spec.md HISTORY 2026-05-08).
The original RED-GREEN-REFACTOR cycle has completed; this document
reconstructs the milestone breakdown so SPEC-IDX-002 has the canonical
3-file SPEC layout that newer SPECs follow.

SPEC-IDX-002 delivers two halves of the BGE-M3 embedding gateway:

1. **Python sidecar** at `services/embedder/` — FastAPI app loading
   `BAAI/bge-m3` via `FlagEmbedding.BGEM3FlagModel` once at startup;
   exposes `POST /embed` and `GET /health`; in-process LRU cache keyed
   on `sha256(text + model_version + mode_flags)`; CPU FP32 default
   with GPU FP16 overlay.

2. **Go HTTP client** at `internal/embedder/` — context-timeout HTTP
   client with exponential backoff retry (2 retries, 500ms + 1500ms ±
   10% jitter); 4xx → no retry; 5xx → retry; per-call observability
   (counter + histogram + cache-hit counter + OTel span).

The IDX-002 surface plugs into SPEC-IDX-001's `Embedder` port via an
adapter that calls `client.Embed(ctx, req)` and converts the
`[][]float64` response to `[][]float32` matching the IDX-001 port
signature.

---

## 2. Phase Breakdown (Post-Hoc Reconstruction)

### Phase A — Python Sidecar Foundation (Pydantic Models + Cache + Obs)

Files (implemented):

- `services/embedder/pyproject.toml` — Runtime deps: `fastapi>=0.115`,
  `uvicorn[standard]>=0.30`, `pydantic>=2.9`, `FlagEmbedding>=1.3.0`,
  `numpy>=1.26`, `cachetools>=5.5`. Dev deps for pytest.
- `services/embedder/src/embedder/__init__.py` (7 LOC) — Package doc +
  `__version__`.
- `services/embedder/src/embedder/models.py` (57 LOC) — `EmbedRequest`,
  `EmbedResponse` Pydantic v2 models with `ConfigDict(extra="forbid",
  str_strip_whitespace=False)`.
- `services/embedder/src/embedder/cache.py` (75 LOC) — In-process LRU
  cache via `cachetools.LRUCache(maxsize=EMBEDDER_CACHE_MAX_ENTRIES)`;
  key derivation `sha256(text.strip() + "\n" + model + "\n" + version +
  "\n" + mode_flags)`. Cache disabled when `EMBEDDER_CACHE_MAX_ENTRIES=0`.
- `services/embedder/src/embedder/obs.py` (131 LOC) — JSON-formatted
  `logging` setup; `Timer` context manager; `log_embed(record: dict)`
  helper that writes the documented attribute set per `/embed`
  invocation.

REQ coverage: REQ-IDX-002-001 (endpoint contract), REQ-IDX-002-004
(cache + key derivation), REQ-IDX-002-006a (Python observability).

### Phase B — BGE-M3 Inference + FastAPI Wiring

Files (implemented):

- `services/embedder/src/embedder/embed.py` (171 LOC) — `Embedder` class
  wrapping `BGEM3FlagModel.encode`. Handles `return_dense`,
  `return_sparse`, `return_colbert_vecs` flags. Validates: empty texts
  → HTTP 400 `empty_input`; `len(texts) > 256` → `batch_too_large`;
  per-text length check via tokenizer fast-pass → `text_too_long`. OOM
  detection wraps inference in try/except for `torch.cuda.OutOfMemoryError`
  + `MemoryError`.
- `services/embedder/src/embedder/app.py` (315 LOC) — FastAPI app with
  `lifespan` async context manager loading the model once at startup
  (records `embedder.ready` slog event). Routes `POST /embed` and
  `GET /health`; 503 returned during load; 503 on `embed` if model not
  ready. Per-call observability (slog + Timer).
- `services/embedder/src/embedder/__main__.py` (20 LOC) — Uvicorn
  entrypoint binding port `EMBEDDER_PORT` (default 8082).
- `services/embedder/Dockerfile` — Multi-stage on `python:3.11-slim`,
  non-root `appuser`, HEALTHCHECK on `/health`, EXPOSE 8082, volume
  for HF model cache.
- `deploy/docker-compose.yml` — New `embedder` service (port 8082,
  healthcheck, `embedder_models` volume, env mapping for all
  `EMBEDDER_*` keys).
- `deploy/docker-compose.gpu.yml` — GPU overlay (NVIDIA device
  reservation + `EMBEDDER_DEVICE=cuda:0` + `EMBEDDER_USE_FP16=true`).

REQ coverage: REQ-IDX-002-002 (BGE-M3 inference + order preservation),
REQ-IDX-002-003 (loading state 503), REQ-IDX-002-007 (invalid input
rejection), REQ-IDX-002-008 (empty modes rejection), REQ-IDX-002-009
(OOM handling), REQ-IDX-002-010 (Korean text pass-through),
REQ-IDX-002-012 (model loaded once), REQ-IDX-002-013 (model version
pinning).

### Phase C — Go HTTP Client + Retry + Observability

Files (implemented):

- `internal/embedder/types.go` (72 LOC) — `Request`, `Response`, error
  sentinels (`ErrInvalidRequest`, `ErrSidecarUnreachable`, `ErrTimeout`,
  `ErrModelLoadFailed`, `ErrOutOfMemory`), `ModeLabel(req) string`
  helper deriving the Prometheus `mode` label.
- `internal/embedder/config.go` (41 LOC) — `Config` struct +
  `ConfigFromEnv()` reading `EMBEDDER_BASE_URL` (default
  `http://localhost:8082`) and `EMBEDDER_REQUEST_TIMEOUT_SECONDS`
  (default 15).
- `internal/embedder/client.go` (257 LOC) — `Client` struct with
  `New(cfg, *obs.Obs)` constructor; `Embed(ctx, req) (Response, error)`
  method. Retry policy:
  - `maxRetries = 2`; base 500ms × 3 multiplier per retry; ±10% jitter.
  - Retry triggers: `*net.OpError`, `*url.Error`, HTTP 5xx.
  - No retry: HTTP 4xx → `ErrInvalidRequest`.
  - Special handling: HTTP 503 → `ErrModelLoadFailed`; HTTP 500 with
    body `{"error":"oom"}` → `ErrOutOfMemory`.
  - Per-call observability emits ONCE per top-level call (not per
    retry).
- `internal/embedder/embedder.go` (22 LOC) — Package doc + symbol
  re-export commentary.
- `internal/embedder/client_test.go` (377 LOC) — `httptest.NewServer`
  fixtures for happy path, timeout, retry, 4xx no-retry, 5xx retry,
  503 model loading, 500 OOM, mode label derivation, observability
  emission single-per-call, nil-safe obs.
- `internal/embedder/concurrent_test.go` (75 LOC) — 50 goroutines × 100
  Embed calls under `go test -race` (NFR-IDX-005).
- `internal/embedder/bench_test.go` (54 LOC) — `TestMain` calls
  `goleak.VerifyTestMain(m)` (NFR-IDX-006).

REQ coverage: REQ-IDX-002-005 (Go HTTP client behaviour), REQ-IDX-002-006b
(Go observability), REQ-IDX-002-011 (concurrent safety).

### Phase D — Observability Wiring + Tests

Files (implemented):

- `internal/obs/metrics/embedder.go` (NEW) — Declares `EmbedderCalls
  *prometheus.CounterVec{outcome, mode}`, `EmbedderLatency
  *prometheus.HistogramVec{outcome, mode}`, `EmbedderCacheHits
  prometheus.Counter` (no labels). Registered via `registerEmbedder(r)`
  called from `NewRegistry`.
- `internal/obs/metrics/metrics.go` — Three new fields added to
  `Registry`; `registerEmbedder(pr)` invocation; `mode` appended to
  `labelNames` cardinality allowlist (4 enum values: dense, sparse,
  colbert, all).
- `internal/obs/obs.go::HasTracer()` — New method to enable nil-safe
  tracer fallback (also reused by IDX-001).
- `services/embedder/tests/test_models.py` — Pydantic validation.
- `services/embedder/tests/test_cache.py` — LRU hit/miss + key
  derivation + disabled cache + LRU eviction + p99 latency NFR.
- `services/embedder/tests/test_obs.py` — Log record shape.
- `services/embedder/tests/test_embed.py` — BGEM3FlagModel wrapper
  (stubbed model).
- `services/embedder/tests/test_app.py` — FastAPI endpoint contract
  via `TestClient`.
- `services/embedder/tests/test_throughput.py` — NFR-IDX-002-001 (slow
  marker).
- `services/embedder/tests/test_latency.py` — NFR-IDX-002-002 p50
  latency (slow marker).
- `services/embedder/tests/test_memory.py` — NFR-IDX-002-007 memory
  ceiling (slow marker).

REQ coverage: REQ-IDX-002-006 (full observability), REQ-IDX-002-001 +
REQ-IDX-002-004 acceptance suite.

---

## 3. Test Catalog Summary

| Phase | Python Tests | Go Tests | REQs Covered | NFRs Covered |
|-------|--------------|----------|--------------|--------------|
| A | test_models, test_cache, test_obs | — | 001, 004, 006a | 003, 004 |
| B | test_app, test_embed | — | 002, 003, 007-010, 012, 013 | 001, 002, 007 |
| C | — | client_test, concurrent_test, bench_test | 005, 006b, 011 | 005, 006 |
| D | (allowlist test) | (metrics_test) | 006 | — |
| **Totals** | **62 tests (93% coverage)** | **15 tests (89.3% coverage)** | **13 / 13** | **7 / 7** |

---

## 4. Risk Mitigation Table

| Risk (from spec.md §10) | Realised? | Resolution |
|-------------------------|-----------|------------|
| Model load takes 5-30s on first boot from HF Hub | Yes | `lifespan` startup blocks until ready; `/health` returns 503 during; `embedder.ready` slog signals completion. Docker `HEALTHCHECK` polls until ready. |
| `BGEM3FlagModel` thread-unsafety with uvicorn workers | Yes | uvicorn `--workers 1` enforced; inference naturally serialised on the asyncio event loop. Compute parallelism via SEPARATE replicas. |
| `torch.cuda.OutOfMemoryError` crashes process | No | OOM caught in inference wrapper; HTTP 500 + body `{"error":"oom"}` returned; process stays up; cached entries preserved. |
| Cache key collision across mode flag combinations | No | Mode flags included in cache key (`"d=1,s=0,c=0"` style suffix); dense-only and dense+sparse get distinct slots. |
| Korean text triggers wrong tokenisation path | No | BGE-M3 internal XLM-RoBERTa tokenizer handles Korean natively; sidecar passes text verbatim with zero pre-processing. |
| Go-side retry storm under transient 5xx | No | Exponential backoff (500ms → 1500ms) + max 2 retries + ±10% jitter; 4xx never retried. |
| pgxpool background goroutine triggers goleak alarm | N/A | IDX-002 has no PG dependency. |
| HF model version drift | No | `EMBEDDER_MODEL_VERSION` env binds `revision` kwarg in `BGEM3FlagModel(revision=...)`; unset = latest, logged at startup. |
| Memory blowout from large ColBERT vectors | Mitigated | ColBERT mode default OFF; per-text ColBERT storage ~1MB for 256-token text documented in §2.3. |

---

## 5. MX Tag Plan (Applied in Source)

### 5.1 @MX:ANCHOR

- `internal/embedder/client.go::Client` (line 41) — `@MX:ANCHOR`
  (Embedder client public API; callers: SPEC-IDX-001, tests, future
  SPEC-DEEP-*). `@MX:REASON`: fan_in ≥ 3; all Go-side embed calls flow
  through `Embed`.

### 5.2 @MX:NOTE

- `internal/embedder/types.go::ModeLabel` (line 49) — `@MX:NOTE`
  (called from `client.go` emitObs; used in metric label derivation).

### 5.3 (No @MX:WARN tags required)

The Go client uses standard `*http.Client` + per-call ctx; no goroutine
fan-out beyond stdlib HTTP transport pool. The Python sidecar's
`lifespan` lifecycle is FastAPI-managed.

---

## 6. File Touch Order (as realised)

1. Phase A: `pyproject.toml` → `models.py` → `cache.py` → `obs.py`
   → `tests/test_models.py` → `tests/test_cache.py` → `tests/test_obs.py`.
2. Phase B: `embed.py` → `app.py` → `__main__.py` → `tests/test_embed.py`
   → `tests/test_app.py` → `Dockerfile` → `deploy/docker-compose.yml`
   → `deploy/docker-compose.gpu.yml` → `.env.example`.
3. Phase C: `internal/embedder/types.go` → `config.go` → `client.go`
   → `embedder.go` → `client_test.go` → `concurrent_test.go`
   → `bench_test.go`.
4. Phase D: `internal/obs/metrics/embedder.go` →
   `internal/obs/metrics/metrics.go` (registerEmbedder + allowlist) →
   `internal/obs/obs.go` (HasTracer method) → Python slow-mark NFR
   tests.

---

## 7. Coverage and Quality Gates (Achieved)

- Python coverage: 93% on `services/embedder/src/embedder/`.
- Go coverage: 89.3% on `internal/embedder/`.
- `go vet ./internal/embedder/...` → 0 issues.
- `golangci-lint run ./internal/embedder/...` → 0 issues.
- `go test -race ./internal/embedder/...` → PASS.
- `goleak.VerifyTestMain` clean (no background goroutines from
  `*http.Client`).
- Python `pytest -q services/embedder/tests/` → 62 passing.
- Python `mypy --strict` → clean (per project default).
- Cardinality allowlist test (`TestNoUnboundedLabels`) → PASS after
  `mode` label addition.

---

## 8. Pre-submission Self-Review

Verified at implementation time:

- Sidecar `/embed` validates request first; OOM caught and returned as
  HTTP 500 with structured body (process stays up).
- Cache key includes mode flags (no cross-mode contamination).
- Go client emits observability ONCE per top-level call regardless of
  retry count.
- Mode label derivation correct: 1 mode → that mode's name; ≥ 2 modes
  → `"all"`; 0 modes → `"dense"` fallback (validation already rejects).
- `Embed` 4xx mapped to `ErrInvalidRequest` (no retry); 503 mapped to
  `ErrModelLoadFailed`; 500 with `{"error":"oom"}` mapped to
  `ErrOutOfMemory`.
- Korean text path is identical to English path (no `if hangul:` branch).
- Model loaded exactly once via `lifespan` (`embedder.model_loaded`
  slog event); freed on shutdown.

---

## 9. Downstream Wiring

- **SPEC-IDX-001** consumes IDX-002 via an `Embedder` interface adapter
  that wraps `internal/embedder.Client.Embed` and converts the
  `[][]float64` response to `[][]float32` (IDX-001 port signature).
  Adapter lives near the CLI bootstrap (`cmd/usearch/`) — the
  `internal/index` package is unaware of `internal/embedder`.
- **Future SPEC-DEEP-***: Reuses the same sidecar for embedding-based
  source selection in deep research workflows.
- **SPEC-CACHE-001** depends on the sidecar being reachable but does
  NOT consume embeddings directly — only blocks until `/health` reports
  ready.

---

*End of SPEC-IDX-002 plan.md (post-hoc).*
