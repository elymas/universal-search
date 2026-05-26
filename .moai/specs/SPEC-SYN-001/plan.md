# SPEC-SYN-001 Implementation Plan (Post-Hoc)

Generated: 2026-05-26 (reverse-engineered from implemented code)
Methodology: TDD (RED-GREEN-REFACTOR) — completed 2026-05-04
Coverage target: 80% (achieved: Go 86.3%, Python 86%)
Harness: standard
Status: implemented (verified against `services/researcher/` and
`internal/synthesis/`)

---

## 1. Overview

This plan.md is a post-hoc summary of SPEC-SYN-001 (basic synthesis v0)
that already shipped (status: `implemented`, spec.md HISTORY 2026-05-04,
`implemented_at: 2026-05-04`). The original RED-GREEN-REFACTOR cycle
has completed; this document reconstructs the milestone breakdown so
SPEC-SYN-001 has the canonical 3-file SPEC layout that newer SPECs
(SYN-002, SYN-003, SYN-004) follow.

SPEC-SYN-001 delivers two halves of the synthesis gateway closing the
M2 exit criterion (`usearch query "hello world"` returns Reddit + HN
results with **one synthesized paragraph + citations**):

1. **Python sidecar** at `services/researcher/` — FastAPI app exposing
   `POST /synthesize` and `GET /health`. Implements an
   extracted-scaffold citation assembler (gpt-researcher single-pass
   local-doc mode equivalent; fallback path per SPEC §11.1 — gpt-
   researcher not installed due to uv workspace conflict). LiteLLM
   proxy gateway with injectable `httpx.AsyncBaseTransport`. Strips
   out-of-range markers with WARN log via `_process_markers()`.
   Degraded mode returns a bullet-list within 2s on
   `httpx.ConnectError`.

2. **Go HTTP client** at `internal/synthesis/` — context-timeout HTTP
   client; exponential backoff retry (2 retries, 500ms / 1500ms ±10%
   jitter); retries on net errors / 5xx only; never retries on 4xx.
   Per-call observability via Prometheus collectors
   `SynthesisCalls{outcome}`, `SynthesisLatency{outcome}`,
   `SynthesisCost` (no labels) registered via
   `internal/obs/metrics/synthesis.go`, plus OTel span `synthesis.call`.

The implementation establishes the **Python sidecar pattern** reused by
all subsequent SPECs (SPEC-IDX-002 embedder, SPEC-IDX-003 tokenizer-ko,
SPEC-SYN-002 faithfulness extension, SPEC-DEEP-* M5).

---

## 2. Phase Breakdown (Post-Hoc Reconstruction)

### Phase A — Python Sidecar Foundation (Pydantic + Gateway + Obs)

Files (implemented):

- `services/researcher/pyproject.toml` — `fastapi>=0.115`,
  `uvicorn[standard]>=0.30`, `pydantic>=2.9`, `httpx>=0.27`,
  `openai>=1.50`. (`gpt-researcher>=0.10` listed but extracted
  scaffold used in lieu of installed package — see HISTORY 2026-05-04.)
- `services/researcher/src/researcher/__init__.py` (9 LOC).
- `services/researcher/src/researcher/models.py` (84 LOC) —
  `SynthesizeRequest{request_id, query, lang, docs:
  list[NormalizedDocPayload]}`, `SynthesizeResponse{request_id, text,
  citations, model, provider, cost_usd, prompt_tokens,
  completion_tokens, latency_ms, degraded, notice}`, `Citation{marker,
  doc_id, url, title}`. Pydantic v2 with `ConfigDict(extra="forbid",
  str_strip_whitespace=True)`.
- `services/researcher/src/researcher/gateway.py` (98 LOC) — `Gateway`
  class wrapping the OpenAI SDK client wired to LiteLLM proxy via
  `OPENAI_BASE_URL=$LITELLM_BASE_URL` and
  `OPENAI_API_KEY=$LITELLM_API_KEY`. Injectable
  `httpx.AsyncBaseTransport` for testing.
- `services/researcher/src/researcher/obs.py` (88 LOC) — JSON
  `setup_logging`, `log_synthesis(record: dict)` helper writing the
  documented 12-attribute set per `/synthesize` invocation.

REQ coverage: REQ-SYN-001 (endpoint contract), REQ-SYN-002a (LiteLLM
routing surface), REQ-SYN-006a (Python observability).

### Phase B — FastAPI App + Synthesis Pipeline

Files (implemented):

- `services/researcher/src/researcher/synthesis.py` (220 LOC) —
  `synthesize(req, gateway)` async function. Builds the LLM prompt
  with citation directives + per-doc snippets; invokes
  `gateway.complete(...)`; calls `_process_markers(text, num_docs)` to
  strip out-of-range `[N]` markers with WARN log; assembles
  `citations` list mapping each marker to `{doc_id, url, title}`. On
  `httpx.ConnectError`, returns `SynthesizeResponse(degraded=True,
  notice="litellm unavailable; returning raw doc list", text=bulleted,
  citations=...)`.
- `services/researcher/src/researcher/app.py` (99 LOC) — FastAPI app
  with `lifespan` calling `setup_logging`. Routes `POST /synthesize`
  and `GET /health`. REQ-SYN-004 validation: 400 on empty `query` or
  zero `docs`. Catch-all exception handler returns 500 with
  `{"error":"internal_error", "detail":...}` (does NOT leak stack
  traces). Optional `lang` system-message directive (REQ-SYN-007).
  Includes routers for faithfulness (SYN-002) and deep_tree (DEEP-003).
- `services/researcher/src/researcher/__main__.py` (25 LOC) — Uvicorn
  entrypoint on `RESEARCHER_PORT` (default 8081).
- `services/researcher/Dockerfile` — Multi-stage `python:3.11-slim`
  with non-root user, healthcheck on `/health`.
- `deploy/docker-compose.yml` — `researcher` service on port 8081,
  `depends_on: [litellm]`, healthcheck, env: `LITELLM_BASE_URL`,
  `LITELLM_API_KEY`, `RESEARCHER_PORT`.

REQ coverage: REQ-SYN-001 (endpoint), REQ-SYN-002 (LiteLLM routing +
marker handling), REQ-SYN-003 (graceful degradation), REQ-SYN-004
(invalid input rejection), REQ-SYN-007 (lang hint propagation).

### Phase C — Go HTTP Client + Retry + Observability

Files (implemented):

- `internal/synthesis/types.go` (68 LOC) — `Request`, `Result`,
  `Citation`, error sentinels (`ErrInvalidRequest`,
  `ErrSidecarUnreachable`, `ErrTimeout`).
- `internal/synthesis/config.go` (50 LOC) — `Config` struct +
  `ConfigFromEnv()` reading `RESEARCHER_BASE_URL` (default
  `http://localhost:8081`) and `RESEARCHER_REQUEST_TIMEOUT_SECONDS`
  (default 10).
- `internal/synthesis/client.go` (280 LOC) — `Client` struct with
  `New(cfg, *obs.Obs) (*Client, error)`; `Synthesize(ctx, query,
  lang, docs) (Result, error)` method:
  - Wall-clock timeout via `context.WithTimeout(ctx,
    cfg.RequestTimeout)`.
  - Retry policy: `maxRetries = 2` on `*net.OpError` / `*url.Error`
    types + HTTP 5xx; backoff `retryBase=500ms × retryMult=3 ± 10%
    jitter`.
  - No retry on HTTP 4xx → returns `ErrInvalidRequest`.
  - Per-call observability via deferred `emitObs(ctx, span, outcome,
    elapsed, reqID, query, docs, Result{})`:
    - `obs.SynthesisCalls.WithLabelValues(outcome).Inc()` once per
      top-level call (NOT per retry).
    - `obs.SynthesisLatency.WithLabelValues(outcome).Observe(elapsed_seconds)`
      once per top-level call.
    - `obs.SynthesisCost.Add(response.cost_usd)` when `cost_usd > 0`.
    - OTel span `synthesis.call` with 8 documented attributes.
- `internal/synthesis/synthesis.go` (6 LOC) — Package doc.
- `internal/synthesis/client_test.go` (582 LOC) — `httptest.NewServer`
  fixtures for happy path, timeout, retry on conn-reset, 4xx no-retry,
  5xx retry, degraded passthrough, observability single-per-call,
  nil-safe obs.

REQ coverage: REQ-SYN-005 (Go HTTP client behaviour), REQ-SYN-006b
(Go observability).

### Phase D — Observability Wiring + Tests

Files (implemented):

- `internal/obs/metrics/synthesis.go` (NEW) — `SynthesisCalls
  *prometheus.CounterVec{outcome}`, `SynthesisLatency
  *prometheus.HistogramVec{outcome}`, `SynthesisCost
  prometheus.Counter` (no labels). Registered via `registerSynthesis(r)`.
- `internal/obs/metrics/metrics.go` — Three new fields added to
  `Registry`; `registerSynthesis(pr)` invocation. ZERO new label names
  introduced (reuses `outcome` from allowlist).
- `services/researcher/tests/test_app.py` — `TestClient`-based
  endpoint contract tests (REQ-SYN-001/003/004/007).
- `services/researcher/tests/test_synthesis.py` — Citation-assembly
  logic with mocked LLM (REQ-SYN-002).
- `services/researcher/tests/test_gateway.py` — OpenAI SDK transport
  wired to httpx mock (REQ-SYN-002 LiteLLM routing).
- `services/researcher/tests/test_obs.py` — JSON log record shape
  (REQ-SYN-006).

REQ coverage: REQ-SYN-006 (full observability).

---

## 3. Test Catalog Summary

| Phase | Python Tests | Go Tests | REQs Covered | NFRs Covered |
|-------|--------------|----------|--------------|--------------|
| A | test_models, test_gateway, test_obs | — | 001, 006a | — |
| B | test_app, test_synthesis | — | 001, 002, 003, 004, 007 | 001, 002, 003 |
| C | — | client_test (582 LOC) | 005, 006b | 004 |
| D | (allowlist check) | (metrics_test) | 006 | — |
| **Totals** | **33 Python tests (86% coverage)** | **15 Go tests (86.3% coverage)** | **7 / 7** | **4 / 4** |

---

## 4. Risk Mitigation Table

| Risk | Realised? | Resolution |
|------|-----------|------------|
| gpt-researcher uv workspace conflict | Yes | Per SPEC §11.1 fallback: extracted-scaffold pattern reused gpt-researcher's prompt scaffolds for citation assembly without installing the package. Verified by `test_synthesis.py` mocked-LLM tests. |
| LiteLLM proxy unreachable mid-request | Yes (by design) | `httpx.ConnectError` caught in `synthesis.py`; returns 200 + `degraded=true` + bullet-list `text`. Counter `usearch_synthesis_calls_total{outcome="degraded"}` +1. |
| Out-of-range `[N]` markers in LLM output | Yes | `_process_markers(text, num_docs)` strips markers where `N > len(docs)` with WARN log. Stripped markers do NOT appear in returned `citations`. |
| Go-side retry storm under transient 5xx | No | Exponential backoff (500ms → 1500ms) + max 2 retries + ±10% jitter; 4xx never retried. |
| Stack trace leak on unhandled exception | No | Catch-all exception handler returns 500 with `{"error":"internal_error", "detail": str(exc)}`; logs the error at ERROR level. |
| Observability emitted per-retry instead of per-call | No | `emitObs` invoked via `defer` ONCE per `Synthesize` call (outermost timer). |
| Empty input not validated | No | Pydantic `str_strip_whitespace=True` strips query; explicit 400 check for empty query + empty docs at endpoint entry. |

---

## 5. MX Tag Plan (Applied in Source)

### 5.1 @MX:ANCHOR

- `internal/synthesis/client.go::Client` (line 39) — `@MX:ANCHOR`
  (synthesis client public API; callers: `cmd/usearch`, CLI, tests).
  `@MX:REASON`: fan_in ≥ 3; all Go-side synthesis calls flow through
  `Synthesize`.

### 5.2 @MX:NOTE / @MX:WARN

- `services/researcher/src/researcher/synthesis.py::_process_markers`
  — `# @MX:NOTE` documents the marker stripping invariant (per
  REQ-SYN-002).
- `internal/synthesis/client.go::Synthesize` — `// @MX:NOTE` documents
  the per-call observability emit-once contract (per REQ-SYN-006).

---

## 6. File Touch Order (as realised)

1. Phase A: `pyproject.toml` → `models.py` → `gateway.py` → `obs.py`
   → `tests/test_models.py` → `tests/test_gateway.py` →
   `tests/test_obs.py`.
2. Phase B: `synthesis.py` → `app.py` → `__main__.py` →
   `tests/test_synthesis.py` → `tests/test_app.py` → `Dockerfile` →
   `deploy/docker-compose.yml` → `.env.example`.
3. Phase C: `internal/synthesis/types.go` → `config.go` → `client.go`
   → `synthesis.go` → `client_test.go`.
4. Phase D: `internal/obs/metrics/synthesis.go` →
   `internal/obs/metrics/metrics.go` (registerSynthesis).

---

## 7. Coverage and Quality Gates (Achieved)

- Python coverage: 86% on `services/researcher/src/researcher/`
  (target 80%).
- Go coverage: 86.3% on `internal/synthesis/` (target 80%).
- `go vet ./internal/synthesis/...` → 0 issues.
- `golangci-lint run ./internal/synthesis/...` → 0 issues.
- `go test -race ./internal/synthesis/...` → PASS.
- `pytest -q services/researcher/tests/` → 33 passing.
- ZERO new cardinality allowlist entries (reuses `outcome`).
- Build success on full project.

---

## 8. Pre-submission Self-Review

Verified at implementation time:

- `/synthesize` validates request first (empty query / zero docs →
  400) before any LLM call.
- `_process_markers` strips out-of-range markers with WARN log and
  does NOT include them in `citations`.
- Degraded mode returns within 2s on `httpx.ConnectError` (the
  caller's fallback path is exercised in tests).
- Go client's `emitObs` is invoked via `defer` so observability emits
  exactly once per top-level call (retries do not double-count).
- Catch-all exception handler returns 500 with structured body (no
  stack trace leak).
- `lang` system-message directive propagated when non-empty; omitted
  when empty/absent (verified in tests).
- LiteLLM routing happens through OpenAI SDK with
  `OPENAI_BASE_URL=$LITELLM_BASE_URL` (verified by `test_gateway.py`).
- No direct `internal/llm.Client` consumption in the synthesis Go
  client (HTTP indirection enforced by architecture).

---

## 9. Downstream Wiring

- **SPEC-SYN-002** (M4 implemented) — Faithfulness scoring extends
  this synthesis pipeline. Adds
  `services/researcher/src/researcher/faithfulness.py` +
  `faithfulness_endpoint.py` (mounted in `app.py` as
  `app.include_router(faithfulness_router)`). Extends observability
  with two new collectors. Reuses the SYN-001 sidecar shape.
- **SPEC-SYN-003** (M4) — Chunking + embedding-based source selection
  extends `synthesize` to pre-select doc subsets.
- **SPEC-SYN-004** (M4) — Streaming synthesis via SSE; replaces the
  `JSONResponse` return path with a chunked stream. Reuses citation
  assembly logic.
- **SPEC-DEEP-001** (M5) — STORM-style multi-perspective deep research
  reuses the Python sidecar pattern; mounts as
  `app.include_router(deep_tree_router)` (DEEP-003 Phase C
  scaffolding already in `app.py:44`).
- **SPEC-DEEP-002** (M5) — Multi-agent pipelines reuse the gateway +
  obs surface.
- **CLI** (`cmd/usearch/query.go`) consumes
  `internal/synthesis.Client.Synthesize` after `fanout.Dispatch` →
  `index.Search` re-rank, then prints the synthesized paragraph with
  inline citation markers.

---

*End of SPEC-SYN-001 plan.md (post-hoc).*
