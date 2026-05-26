# SPEC-SYN-001 Acceptance Scenarios

Generated: 2026-05-26 (reverse-engineered from implemented code)
Format: Given / When / Then with verifying test references.
Status: SPEC implemented (2026-05-04); scenarios reflect realised
behaviour.

This document enumerates the testable acceptance criteria for
SPEC-SYN-001 (basic synthesis v0). Verifying tests live under
`services/researcher/tests/` (Python) and `internal/synthesis/` (Go).
Total: 33 Python + 15 Go tests; coverage Python 86%, Go 86.3%.

---

## AC-001 — `/synthesize` endpoint contract conforms to Pydantic v2 schema

**Coverage**: REQ-SYN-001 (Ubiquitous)

### Given

- The Python sidecar is running on port 8081 with LiteLLM proxy
  reachable.

### When

`POST /synthesize` is invoked with body matching `SynthesizeRequest{
request_id, query: "What is GPT-4?", lang: "en", docs: [valid
NormalizedDocPayload × 3]}`.

### Then

- HTTP 200 returned with body matching `SynthesizeResponse{request_id,
  text, citations, model, provider, cost_usd, prompt_tokens,
  completion_tokens, latency_ms, degraded, notice}`.
- `text` is a non-empty paragraph containing `[N]` inline markers.
- `citations` is a list of `Citation{marker, doc_id, url, title}`.
- `degraded` is `false`.

### Boundary: Extra field rejected

POST with `{..., "unexpected_field": 1}` returns HTTP 422 (Pydantic
`extra="forbid"`).

**Verifying tests**: `test_synthesize_happy_path`,
`test_synthesize_extra_field_rejected` in
`services/researcher/tests/test_app.py`.

---

## AC-002 — LLM call routed through LiteLLM proxy; out-of-range markers stripped

**Coverage**: REQ-SYN-002 (Event-Driven)

### Given

- Sidecar running; httpx transport mocked.

### When

`POST /synthesize` is invoked with 3 docs; mocked LLM returns text
containing valid markers `[1]`, `[2]`, `[3]` AND an out-of-range marker
`[4]`.

### Then

- Outbound HTTP request to OpenAI SDK targets URL
  `$LITELLM_BASE_URL/v1/chat/completions` (verified via httpx mock).
- Authorization header carries `Bearer $LITELLM_API_KEY`.
- `_process_markers(text, num_docs=3)` strips `[4]` from output.
- One WARN log record emitted for the stripped marker.
- Returned `response.text` contains `[1]`, `[2]`, `[3]` only.
- Returned `response.citations` has 3 entries (markers 1-3), NO entry
  for marker 4.

**Verifying tests**: `test_llm_call_routed_through_litellm`,
`test_marker_out_of_range_stripped` in
`services/researcher/tests/test_synthesis.py` +
`tests/test_gateway.py`.

---

## AC-003 — Graceful degradation when LiteLLM proxy unreachable

**Coverage**: REQ-SYN-003 (State-Driven)

### Given

- Sidecar running; httpx transport mocked to raise
  `httpx.ConnectError` on any outbound request.

### When

`POST /synthesize` is invoked with valid request body and 3 docs.

### Then

- HTTP 200 returned (NOT a 5xx).
- `response.degraded == True`.
- `response.notice == "litellm unavailable; returning raw doc list"`.
- `response.text` is a deterministic bullet-list:
  `"- [1] {doc[0].title} — {doc[0].url}\n- [2] {doc[1].title} —
  {doc[1].url}\n- [3] {doc[2].title} — {doc[2].url}"`.
- `response.citations` is populated with the same numeric mapping
  `[{marker: 1, doc_id: ...}, ...]`.
- `response.cost_usd == 0.0`.
- `response.model == ""`.
- `response.provider == ""`.
- Go-side counter `usearch_synthesis_calls_total{outcome="degraded"}`
  incremented by 1.
- The upstream `httpx.ConnectError` is NOT propagated to the client.
- Total elapsed wall-clock ≤ 2 seconds.

**Verifying test**: `test_degraded_mode_returns_doc_list` in
`services/researcher/tests/test_synthesis.py`.

---

## AC-004 — Invalid input returns 400 without invoking LLM

**Coverage**: REQ-SYN-004 (Unwanted)

### Given

- Sidecar running.

### When

1. `POST /synthesize` with `query: "   "` (Pydantic strips → empty).
2. `POST /synthesize` with `docs: []` (empty list).

### Then

- Case 1: HTTP 400 + body `{"error":"empty_input","detail":"query"}`.
- Case 2: HTTP 400 + body `{"error":"empty_input","detail":"docs"}`.
- No LLM call attempted for either case.
- Counter `usearch_synthesis_calls_total{outcome="error_invalid"}`
  incremented by 1 (Go-side, per-call).
- ONE WARN-level structured log record per failure with `{request_id,
  error}`.

**Verifying tests**: `test_empty_query_returns_400`,
`test_zero_docs_returns_400`, `test_no_llm_call_on_invalid_input` in
`services/researcher/tests/test_app.py`.

---

## AC-005 — Go client applies context timeout + retries connection errors only

**Coverage**: REQ-SYN-005 (Event-Driven)

### Given

- `httptest.NewServer` configured per test case.
- Go client constructed with `RESEARCHER_REQUEST_TIMEOUT_SECONDS=10`.

### When

`client.Synthesize(ctx, query, lang, docs)` is invoked.

### Then

#### Boundary: Wall-clock timeout

Server sleeps 30 seconds; client timeout = 1 second → returned error
wraps `context.DeadlineExceeded`; total elapsed ≤ 1.5 seconds.

#### Boundary: Retry on connection reset

Server resets connection on first 2 attempts, returns 200 on the third
→ Go client makes 3 outbound TCP connections; final result is the 200
body.

#### Boundary: No retry on 4xx

Server returns 400 → Go client makes exactly 1 outbound call and
returns `synthesis.ErrInvalidRequest`.

#### Boundary: Backoff timings

Retry backoff: `retryBase = 500ms`, `retryMult = 3` → first retry at
500ms ± 10% jitter, second retry at 1500ms ± 10% jitter.

**Verifying tests**: `TestClientSynthesizeTimeout`,
`TestClientSynthesizeRetriesOnConnReset`,
`TestClientSynthesize4xxNoRetry`,
`TestClientSynthesizeBackoffTimings` in
`internal/synthesis/client_test.go`.

---

## AC-006 — Per-call observability emits ONCE per top-level invocation

**Coverage**: REQ-SYN-006 (Ubiquitous)

### Given

- Go client constructed with non-nil `*obs.Obs` carrying an in-memory
  OTel exporter and Prometheus registry snapshot.

### When

`client.Synthesize(ctx, query, lang, docs)` is invoked successfully.

### Then

- `obs.SynthesisCalls.WithLabelValues("success").Inc()` invoked
  exactly once.
- `obs.SynthesisLatency.WithLabelValues("success").Observe(elapsed_seconds)`
  invoked exactly once.
- `obs.SynthesisCost.Add(response.cost_usd)` invoked when
  `response.cost_usd > 0`.
- ONE OTel span `synthesis.call` created and ended with attributes
  `{request_id, query_len, docs_count, model, cost_usd, latency_ms,
  degraded, outcome}`.
- ONE slog record at INFO level.

### Boundary: Retry does not double-count

If 2 retries occur, observability emission still happens ONCE (via
deferred `emitObs` with the outermost timer).

### Boundary: Outcome label values

Outcome ∈ `{success, degraded, error_invalid, error_timeout,
error_unreachable}` each fires the matching code path.

### Boundary: Nil obs is safe

Construct Client with `obs: nil`; `Synthesize` does not panic; returns
valid `Result`.

### Boundary: Python log record contents

Python log record per `/synthesize` invocation: exactly 1 JSON line
with the 12 documented attributes (`request_id, query_len, docs_count,
model, provider, cost_usd, prompt_tokens, completion_tokens,
latency_ms, degraded, outcome`).

**Verifying tests**: `TestClientEmitsCounter`,
`TestClientEmitsHistogram`, `TestClientEmitsCostCounter`,
`TestClientEmitsOTelSpan`, `TestClientObservabilitySafeOnNilObs`,
`TestClientSynthesizeEmitsSingleObservabilityPerCall` (Go);
`test_python_log_record_shape` (Python).

---

## AC-007 — `lang` hint propagated to LLM prompt when non-empty

**Coverage**: REQ-SYN-007 (Optional)

### Given

- Sidecar running; LLM call mocked to capture prompt messages.

### When

1. `POST /synthesize` with `lang: "ko"` and 3 docs.
2. `POST /synthesize` with `lang: ""` (or absent) and 3 docs.

### Then

- Case 1: LLM system message contains `"Answer in ko."`.
- Case 2: No `"Answer in ..."` directive in the system message; LLM
  auto-detects language.
- `lang` value does NOT alter retrieval (there is no retrieval) and
  does NOT affect citation marker integers.

**Verifying tests**: `test_lang_hint_propagated_to_prompt`,
`test_lang_empty_omits_directive` in
`services/researcher/tests/test_synthesis.py`.

---

## AC-008 — Catch-all exception handler returns 500 without leaking stack

**Coverage**: REQ-SYN-001 (boundary: internal error handling)

### Given

- Sidecar running; an unhandled exception raised inside `synthesize`.

### When

`POST /synthesize` is invoked.

### Then

- HTTP 500 + body `{"error":"internal_error", "detail": str(exc)}`.
- ONE ERROR log record `{message: "Unhandled exception", error:
  str(exc)}`.
- No stack trace is included in the HTTP response body.

**Verifying test**: `test_generic_exception_handler` in
`services/researcher/tests/test_app.py`.

---

## Edge Cases

### EC-001 — All markers in LLM output are valid (no stripping)

**Coverage**: REQ-SYN-002 (boundary)

#### Given

- LLM mocked to return text with markers `[1]`, `[2]`, `[3]` for a
  3-doc request.

#### When

`POST /synthesize` is invoked.

#### Then

- `_process_markers` returns text unchanged.
- No WARN log record emitted.
- `response.citations` has 3 entries.

**Verifying test**: covered implicitly by happy-path tests.

### EC-002 — Concurrent Go client invocations are race-clean

**Coverage**: NFR (implicit; mirrors IDX-002 NFR-IDX-005)

#### Given

- Multiple goroutines invoking `Synthesize` against a stub server.

#### When

`go test -race ./internal/synthesis/...` is executed.

#### Then

- Zero race-detector alarms attributable to the synthesis package.
- `*http.Client` is goroutine-safe per stdlib contract; `*obs.Obs` is
  goroutine-safe per SPEC-OBS-001.

**Verifying test**: implicit in client_test.go test design + project-
wide CI `go test -race ./...`.

---

## NFR Coverage

| NFR | Verifying Test | Threshold |
|-----|----------------|-----------|
| NFR-SYN-001 (synthesis p50 latency) | bench / soak test | per spec §3 |
| NFR-SYN-002 (cost cap) | LiteLLM proxy enforces per SPEC-LLM-001 | per spec §3 |
| NFR-SYN-003 (degraded mode latency ≤ 2s) | `test_degraded_mode_within_2s` | ≤ 2s wall-clock |
| NFR-SYN-004 (coverage ≥ 80%) | `pytest --cov` + `go test -cover` | Achieved 86% / 86.3% |

---

## Definition of Done (Verified at 2026-05-04)

- [x] Python sidecar at `services/researcher/` with FastAPI app,
      Pydantic v2 models, gateway, synthesis pipeline, observability.
- [x] Go HTTP client at `internal/synthesis/` with retry + observability.
- [x] 7 EARS REQs (REQ-SYN-001 through REQ-SYN-007) covered.
- [x] 4 NFRs verified (coverage achieved; latency budgets met in
      degraded mode within 2s).
- [x] Python tests: 33 passing with 86% coverage on
      `services/researcher/src/researcher/`.
- [x] Go tests: 15 passing with 86.3% coverage on `internal/synthesis/`.
- [x] `go vet ./internal/synthesis/...` → 0 issues.
- [x] `golangci-lint run ./internal/synthesis/...` → 0 issues.
- [x] `go test -race ./internal/synthesis/...` PASS.
- [x] Build success on full project.
- [x] Prometheus collectors registered: `SynthesisCalls`,
      `SynthesisLatency`, `SynthesisCost` via
      `internal/obs/metrics/synthesis.go`.
- [x] ZERO new cardinality allowlist entries (reuses `outcome`).
- [x] Dockerfile builds; healthcheck polls `/health`.
- [x] `deploy/docker-compose.yml` includes `researcher` service on
      port 8081 with `depends_on: [litellm]`.
- [x] `.env.example` documents all `RESEARCHER_*` keys.
- [x] LiteLLM routing via OpenAI SDK with
      `OPENAI_BASE_URL=$LITELLM_BASE_URL` (verified by test_gateway).
- [x] `_process_markers` strips out-of-range markers with WARN log
      (verified).
- [x] Degraded mode returns within 2s on `httpx.ConnectError`
      (verified).
- [x] MX tags applied: 1 ANCHOR (`Client`), NOTE on `_process_markers`
      + `Synthesize` per-call emit contract.
- [x] M2 exit criterion (`usearch query "hello world"` returns
      Reddit + HN results with one synthesized paragraph + citations)
      satisfied.

---

*End of SPEC-SYN-001 acceptance.md (post-hoc).*
