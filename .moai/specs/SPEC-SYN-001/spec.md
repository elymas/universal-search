---
id: SPEC-SYN-001
title: Basic synthesis v0
version: 0.1.0
milestone: M2 — First end-to-end slice
status: draft
priority: P0
owner: expert-backend
methodology: tdd
coverage_target: 80
created: 2026-04-28
updated: 2026-04-28
author: limbowl
issue_number: null
depends_on: [SPEC-CORE-001, SPEC-LLM-001, SPEC-BOOT-001]
blocks: [SPEC-SYN-002, SPEC-SYN-003, SPEC-SYN-004]
---

# SPEC-SYN-001: Basic synthesis v0

## HISTORY

- 2026-04-28 (initial draft v0.1, limbowl via manager-spec):
  First EARS-formatted SPEC for the synthesis layer. Establishes the
  Python sidecar pattern (`services/researcher/`) wrapping
  `gpt-researcher` (Apache-2.0) in single-pass local-doc mode for the
  M2 exit criterion. Builds on SPEC-CORE-001 (`pkg/types.NormalizedDoc`
  input contract), SPEC-LLM-001 (LiteLLM proxy at `:4000` is the LLM
  routing point for both Go-side and Python-side calls per SPEC-LLM-001
  §2.2 Out-of-Scope), SPEC-BOOT-001 (compose service entries +
  `internal/synthesis/` 4-line stub). Coordinates with SPEC-IR-001
  (`RoutingDecision.Lang` flows in as a hint) and SPEC-OBS-001 (new
  `usearch_synthesis_*` metric family registered via
  `internal/obs/metrics/`). Companion research artifact at
  `.moai/specs/SPEC-SYN-001/research.md`. 7 EARS REQs (6 × P0 + 1 × P1),
  4 NFRs, citation faithfulness scoring deferred to SPEC-SYN-002,
  streaming deferred to SPEC-SYN-004, deep / multi-step research
  deferred to SPEC-DEEP-* (M5). Ready for plan-auditor review and
  annotation cycle.

---

## 1. Purpose

The M2 exit criterion in `.moai/project/roadmap.md` §5 reads:

> `usearch query "hello world"` returns Reddit + HN results with one
> synthesized paragraph + citations.

SPEC-ADP-001 (Reddit adapter, implemented), a future ADP-002 (HN
adapter), and SPEC-IR-001 (Intent Router, implemented) cover the
**retrieval + routing** half of that criterion. SPEC-CLI-001 wires the
CLI surface. SPEC-SYN-001 fills the remaining gap: **the synthesized
paragraph + citations**.

SPEC-SYN-001 is the *value-delivery step* — it is what turns raw
`[]NormalizedDoc` results into an answer the user can read. Without
it, the M2 slice still surfaces a list of links; with it, the system
behaves as a research meta-agent.

This SPEC delivers:

- A **Python sidecar service** at `services/researcher/` exposing a
  single FastAPI POST endpoint `/synthesize` that accepts a query plus
  pre-fetched `[]NormalizedDoc` and returns one synthesized paragraph
  with inline `[1]..[N]` citation markers and a marker→doc mapping.
- A **gpt-researcher integration** in single-pass local-document mode.
  The Python sidecar bypasses gpt-researcher's own retrievers (the Go
  orchestration plane has already retrieved the documents) and uses
  the library's prompt scaffolds for citation assembly only. Deep /
  multi-step research is reserved for SPEC-DEEP-* (M5).
- A **Go-side HTTP client** at `internal/synthesis/client.go` that calls
  the sidecar with context timeout and basic retry, returning a typed
  `SynthesisResult` to the CLI / API consumers.
- **LLM routing via the LiteLLM proxy** at `LITELLM_BASE_URL`
  (default `http://litellm:4000`) — established by SPEC-LLM-001 — so
  cost, retries, fallback, and provider routing happen exactly once,
  in one place, regardless of whether the call originates from Go or
  Python.
- **Per-call observability** consistent with SPEC-OBS-001: one slog
  record + one Prometheus counter increment + one histogram observation
  + one OTel span on every synthesize call, on both the Python service
  side and the Go client side. A new metric family
  `usearch_synthesis_*` is registered via `internal/obs/metrics/`.
- **Graceful degradation** when the LiteLLM proxy is unavailable: the
  service returns `degraded=true` with a deterministic bullet-list of
  doc titles + URLs as the synthesized text, so the CLI surface still
  yields *something* readable to the user.

Completion unblocks SPEC-SYN-002 (citation faithfulness scoring,
DeepEval gate ≥0.85), SPEC-SYN-003 (chunking + embedding-based source
selection), and SPEC-SYN-004 (streaming synthesis), all in M4. It does
NOT unblock M5 deep-research SPECs directly — those depend on the
broader Python sidecar pattern, which this SPEC establishes, but use
their own multi-step pipelines.

This SPEC is on the M2 critical path: without it, M2 does not exit.

---

## 2. Scope

### 2.1 In-Scope

| Ref | Item |
|-----|------|
| a | `services/researcher/src/researcher/` package layout: `app.py` (FastAPI app + lifespan), `models.py` (Pydantic v2 request/response), `synthesis.py` (gpt-researcher local-doc invocation + citation assembly), `gateway.py` (OpenAI SDK client wired to LiteLLM proxy), `obs.py` (slog-equivalent JSON logger + per-call timing helpers), `__main__.py` (uvicorn entrypoint) |
| b | `services/researcher/pyproject.toml` runtime dependency additions: `fastapi>=0.115`, `uvicorn[standard]>=0.30`, `pydantic>=2.9`, `httpx>=0.27`, `gpt-researcher>=0.10` (or extracted equivalent — see Open Question §11.1), `openai>=1.50` |
| c | FastAPI POST `/synthesize` endpoint — accepts `SynthesizeRequest`, returns `SynthesizeResponse`; status codes per §4.6 |
| d | FastAPI GET `/health` endpoint — returns `{"status":"ok","version":"0.1.0"}` on success or 503 with `{"status":"degraded","reason":"litellm unreachable"}` when upstream LiteLLM ping fails |
| e | gpt-researcher integration in `report_source="local"` mode with `OPENAI_BASE_URL=$LITELLM_BASE_URL` and `OPENAI_API_KEY=$LITELLM_API_KEY` so all LLM traffic routes through the LiteLLM proxy |
| f | Citation assembly: numeric `[N]` inline markers + a `citations` list mapping each marker to `{doc_id, url, title}`; markers MUST reference doc IDs present in the input |
| g | `internal/synthesis/client.go` — Go HTTP client struct `Client` with `Synthesize(ctx, query, lang, docs []types.NormalizedDoc) (Result, error)` method; uses `net/http` with a `*http.Client{Timeout: 10s}` and exponential backoff retry (2 retries) on connection-level errors |
| h | `internal/synthesis/types.go` — Go-side value types `Request`, `Result`, `Citation`, error sentinels |
| i | `internal/synthesis/config.go` — env binder for `RESEARCHER_BASE_URL` and `RESEARCHER_REQUEST_TIMEOUT_SECONDS` (mirroring the `internal/llm/config/config.go` pattern) |
| j | `internal/obs/metrics/synthesis.go` — NEW file declaring `SynthesisCalls *prometheus.CounterVec{outcome}`, `SynthesisLatency *prometheus.HistogramVec{outcome}`, `SynthesisCost prometheus.Counter` (no labels). Lives in `internal/obs/metrics/` to preserve the SPEC-OBS-001 import-boundary test |
| k | `internal/obs/metrics/metrics.go` — minor edit: add 3 fields to `Registry`, call `registerSynthesis(pr)` from `NewRegistry()` |
| l | `services/researcher/Dockerfile` — multi-stage rebuild on `python:3.11-slim` with explicit non-root user; healthcheck calls `/health` |
| m | `deploy/docker-compose.yml` delta: new `researcher` service entry (port `8081`, `depends_on: [litellm]`, healthcheck on `/health`); env mapping `LITELLM_BASE_URL=http://litellm:4000`, `LITELLM_API_KEY=${LITELLM_MASTER_KEY}`, `RESEARCHER_PORT=8081` |
| n | Root `.env.example` additions: `RESEARCHER_BASE_URL`, `RESEARCHER_REQUEST_TIMEOUT_SECONDS`, `RESEARCHER_PORT`, `RESEARCHER_MODEL_DEFAULT`, `RESEARCHER_TIMEOUT_SECONDS` |
| o | `services/researcher/tests/` test files: `test_app.py` (HTTP endpoint contract via FastAPI `TestClient`), `test_synthesis.py` (citation-assembly logic with mocked LLM), `test_gateway.py` (OpenAI SDK transport wired to httpx mock), `test_obs.py` (slog-equivalent JSON record shape) |
| p | `internal/synthesis/client_test.go` — Go HTTP client integration tests against an in-process FastAPI fixture launched via `httptest.NewServer` returning canned JSON; covers happy path, timeout, retry, degraded-mode passthrough, and connection refused |

### 2.2 Out-of-Scope (Exclusions — What NOT to Build)

[HARD] This SPEC explicitly excludes the following items. Each has a known
destination SPEC; this list prevents scope creep into SYN-001.

- **Citation faithfulness scoring / DeepEval CI gate** — measuring
  whether each claim is grounded in cited sources is its own concern.
  → SPEC-SYN-002 (M4).
- **Streaming responses (SSE / chunked)** — V1 returns the full
  synthesized paragraph in a single JSON response. → SPEC-SYN-004 (M4).
- **Embedding-based source ranking / chunking** — V1 stuffs all
  `Snippet` (or truncated `Body`) text into one prompt. → SPEC-SYN-003 (M4).
- **STORM-style multi-perspective deep research** — requires multi-step
  LLM workflows, perspective generation, refinement loops. → SPEC-DEEP-001 (M5).
- **Multi-agent pipelines (Researcher / Reviewer / Writer / Verifier)**
  → SPEC-DEEP-002 (M5).
- **Tree exploration with depth/breadth budgets** → SPEC-DEEP-003 (M5).
- **`/deep` quota + cost guard with Haiku pre-screen** → SPEC-DEEP-004 (M5).
- **Korean-locale synthesis quality benchmarking** → SPEC-EVAL-003 (M8).
- **Per-tenant synthesis budgets / virtual keys** — V1 inherits the
  global per-request budget cap from SPEC-LLM-001 NFR-LLM-003.
  → SPEC-AUTH-002 (M6).
- **Pre-flight token estimation** — same rationale as SPEC-LLM-001 §11.5;
  V1 is post-flight only.
- **Adapter invocation, fanout, dedup, partial-result assembly** —
  synthesis consumes whatever `[]NormalizedDoc` is passed in. → SPEC-FAN-001 (M3).
- **HTTP / gRPC exposure of synthesis to external clients** — only
  `internal/synthesis/client.go` and (future) MCP server consume the
  Python sidecar. The sidecar binds to a private compose network port
  and is not directly exposed.
- **Caching of synthesis results** by query hash, doc-set hash, or any
  other key. v0 is a pure functional synthesizer.
- **Hot-reload of prompts / system messages** — prompt scaffolds ship
  as Python source.
- **Tool-use / structured-output API for citation extraction** — v0
  uses string-prompt JSON output with parser-side validation. → Future
  SPEC if measured value.
- **Cardinality allowlist amendment for new label names** — SPEC-SYN-001
  reuses the existing `outcome` label name; no new label names introduced.
- **Direct `internal/llm.Client` consumption from Go** — V1 synthesis
  goes Go HTTP → Python sidecar → OpenAI SDK → LiteLLM. The Go-side
  `internal/llm.Client` is NOT in the synthesis call path.
- **GitHub Issue tracking on this SPEC** (`issue_number: null`).

---

## 3. EARS Requirements

### Functional Requirements

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| REQ-SYN-001 | Ubiquitous | The Python sidecar SHALL expose a single HTTP endpoint `POST /synthesize` accepting `application/json` requests matching the Pydantic model `SynthesizeRequest{request_id: str, query: str, lang: str \| None, docs: list[NormalizedDocPayload]}` and returning HTTP 200 with `application/json` matching `SynthesizeResponse{request_id, text, citations, model, provider, cost_usd, prompt_tokens, completion_tokens, latency_ms, degraded, notice}`. Pydantic config SHALL be `ConfigDict(extra="forbid", str_strip_whitespace=True)`. | P0 | `test_synthesize_happy_path` POSTs valid request, asserts 200 + response shape; `test_synthesize_extra_field_rejected` asserts 422 on unknown fields. |
| REQ-SYN-002 | Event-Driven | WHEN a `/synthesize` request arrives with at least one `docs[]` entry whose `id`, `source_id`, `url`, and `retrieved_at` are non-empty, the service SHALL invoke gpt-researcher in single-pass local-document mode (or equivalent prompt scaffold) with the LLM transport routed through the LiteLLM proxy at `LITELLM_BASE_URL` using the bearer token `LITELLM_API_KEY`, and SHALL produce a synthesized text whose every `[N]` marker references an integer `N ∈ [1, len(docs)]`; markers outside that range SHALL be stripped from the output, logged at WARN level, and SHALL NOT appear in the returned `citations` list. | P0 | `test_llm_call_routed_through_litellm` mocks httpx and asserts outbound URL == `LITELLM_BASE_URL/v1/chat/completions`; `test_marker_out_of_range_stripped`. |
| REQ-SYN-003 | State-Driven | WHILE the LiteLLM proxy at `LITELLM_BASE_URL` is unreachable (httpx ConnectError, or non-2xx response from a startup ping), the service SHALL still respond to `/synthesize` requests with HTTP 200, populated `degraded=true`, `notice="litellm unavailable; returning raw doc list"`, `text` set to a deterministic bullet-list of `[N] {title} — {url}` (one line per input doc), `citations` populated with the same numeric mapping, `cost_usd=0.0`, `model=""`, `provider=""`. The service SHALL NOT propagate the upstream error to the client. | P0 | `test_degraded_mode_returns_doc_list` injects a fake httpx transport that raises `ConnectError`; asserts 200 + `degraded=true` + bullet-list `text`; counter `usearch_synthesis_calls_total{outcome="degraded"}` +1. |
| REQ-SYN-004 | Unwanted | IF a `/synthesize` request body has `query == ""` (after `str_strip_whitespace`) OR `len(docs) == 0`, THEN the service SHALL return HTTP 400 with body `{"error":"empty_input","detail":"<which field>"}`, SHALL NOT invoke the LLM, SHALL increment `usearch_synthesis_calls_total{outcome="error_invalid"}` exactly once, and SHALL emit one WARN-level structured log record with attributes `{request_id, error}`. | P0 | `test_empty_query_returns_400`; `test_zero_docs_returns_400`; assert no LLM call. |
| REQ-SYN-005 | Event-Driven | WHEN the Go-side caller `internal/synthesis.Client.Synthesize(ctx, ...)` invokes the sidecar, the Go client SHALL apply a wall-clock timeout from `RESEARCHER_REQUEST_TIMEOUT_SECONDS` (default 10) via `context.WithTimeout`, SHALL retry on `net.OpError` and `*url.Error` types up to 2 times with exponential backoff (500 ms, 1500 ms ± 10% jitter), SHALL NOT retry on HTTP 4xx responses, and SHALL emit one slog record + one `obs.SynthesisCalls{outcome}` increment + one `obs.SynthesisLatency{outcome}` observation + one OTel span `synthesis.call` per top-level invocation (NOT per retry). | P0 | `TestClientSynthesizeTimeout`, `TestClientSynthesizeRetriesOnConnReset`, `TestClientSynthesize4xxNoRetry`, `TestClientSynthesizeEmitsSingleObservabilityPerCall`. |
| REQ-SYN-006 | Ubiquitous | The Python sidecar AND the Go client SHALL emit per-`/synthesize` invocation observability: (a) the sidecar logs a single JSON record at INFO with `{request_id, query_len, docs_count, model, provider, cost_usd, prompt_tokens, completion_tokens, latency_ms, degraded, outcome}`; outcome ∈ `{success, degraded, error_invalid, error_timeout, error_unreachable}`; (b) the Go client increments `obs.SynthesisCalls.WithLabelValues(outcome).Inc()` exactly once, observes `obs.SynthesisLatency.WithLabelValues(outcome).Observe(elapsed_seconds)` exactly once, calls `obs.SynthesisCost.Add(response.cost_usd)` when `response.cost_usd > 0`, and creates+ends one OTel span `synthesis.call` with attributes `{request_id, query_len, docs_count, model, cost_usd, latency_ms, degraded, outcome}`. The Go client SHALL be nil-safe across `obs.Obs`, individual collectors, and `obs.Logger` per the pattern at `internal/llm/client.go:244-251`. | P0 | `test_python_log_record_shape`; `TestClientEmitsCounter`, `TestClientEmitsHistogram`, `TestClientEmitsCostCounter`, `TestClientEmitsOTelSpan`, `TestClientObservabilitySafeOnNilObs`. |
| REQ-SYN-007 | Optional | WHERE the request includes a non-empty `lang` field (BCP-47 language tag), the service SHALL pass that hint into the LLM prompt as a system-message instruction (`"Answer in {lang}."`); WHERE `lang` is empty or absent, the service SHALL omit the language directive and let the LLM auto-detect from the query and docs. The `lang` value SHALL NOT alter retrieval behavior (there is no retrieval) and SHALL NOT affect the citation marker integers. | P1 | `test_lang_hint_propagated_to_prompt` mocks LLM call and asserts system message contains `"Answer in ko."`; `test_lang_empty_omits_directive`. |

### Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| NFR-SYN-001 | Performance (p50 latency) | The Python sidecar SHALL complete a `/synthesize` call with p50 ≤ 8 seconds end-to-end on a 10-document input (each doc with `Snippet` ≤ 280 chars) when the LLM model is `claude-haiku-4-5` (or the `RESEARCHER_MODEL_DEFAULT` env override) and the LiteLLM proxy is reachable. Measured via integration test `test_synthesize_p50_latency_under_limit` against a stub LiteLLM proxy that returns within 1.5–4 s with realistic jitter; 50 sequential calls; assert `durations[25] ≤ 8.0` (p50 = index 25 of 50). |
| NFR-SYN-002 | Citation grounding (foundation only) | Every `[N]` marker that appears in `SynthesizeResponse.text` SHALL have a corresponding entry in `SynthesizeResponse.citations` whose `marker == N` and whose `doc_id` is the `id` field of one of the input `docs`. Markers that fail this check SHALL be removed from `text` before returning. Faithfulness — i.e. whether each *claim* is grounded in its cited source — is OUT OF SCOPE here and is the subject of SPEC-SYN-002. SPEC-SYN-001 only guarantees the structural mapping. |
| NFR-SYN-003 | Graceful degradation | WHEN the LiteLLM proxy is unreachable (per REQ-SYN-003 detection) the service SHALL respond within 2 seconds (because the upstream ping fails fast — no retry-storm tolerated) with the deterministic bullet-list payload. The `text` field of the degraded response SHALL be ≤ `len(docs) * 320` characters and SHALL contain exactly one line per input doc. The Go client SHALL surface the degraded response to its caller without converting it into an error. |
| NFR-SYN-004 | Cost emission | The Python sidecar SHALL forward the `x-litellm-response-cost` header value (per SPEC-LLM-001 REQ-LLM-006) into `SynthesizeResponse.cost_usd` as a decimal float, defaulting to `0.0` when the header is absent or malformed (logged at DEBUG / WARN respectively, never raising). The Go client SHALL increment `usearch_synthesis_cost_usd` (Prometheus counter, no labels) by exactly `response.cost_usd` per successful call; in degraded mode the counter SHALL NOT be incremented. |

---

## 4. Acceptance Criteria

### REQ-SYN-001 — `/synthesize` Endpoint Contract

- File `services/researcher/src/researcher/app.py` declares a FastAPI
  application; route `POST /synthesize` is registered.
- File `services/researcher/src/researcher/models.py` declares
  `NormalizedDocPayload`, `SynthesizeRequest`, `Citation`,
  `SynthesizeResponse` Pydantic v2 models with
  `ConfigDict(extra="forbid", str_strip_whitespace=True)`.
- `NormalizedDocPayload` has the 15-field shape mirroring
  `pkg/types.NormalizedDoc` (snake_case JSON; UTC ISO-8601 datetimes).
- `test_synthesize_happy_path`: POST with 3 valid docs returns 200 +
  response with `text`, `citations[].marker ∈ {1,2,3}`, all citation
  `doc_id` values present in the input.
- `test_synthesize_extra_field_rejected`: POST with `{...,
  "unexpected_field": 1}` returns 422 (FastAPI Pydantic validation).
- `test_synthesize_response_shape_matches_schema`: returned JSON
  validates against `SynthesizeResponse.model_json_schema()`.

### REQ-SYN-002 — gpt-researcher integration + LiteLLM routing

- `test_llm_call_routed_through_litellm`: mock `httpx.AsyncClient`
  records the outbound URL of every LLM call; assert each URL begins
  with `LITELLM_BASE_URL/v1/chat/completions`.
- `test_authorization_header_sent`: assert the outbound LLM request
  carries `Authorization: Bearer $LITELLM_API_KEY`.
- `test_marker_out_of_range_stripped`: feed an LLM response that
  references `[5]` while only 3 docs were supplied; assert returned
  `text` no longer contains `[5]` and `citations` has no `marker == 5`
  entry; assert exactly one WARN log record emitted.
- `test_marker_zero_stripped`: same check for `[0]`.
- `test_doc_id_resolution_uses_input_ids`: `citations[i].doc_id ==
  request.docs[citations[i].marker - 1].id`.
- `test_no_retrieval_attempted`: assert gpt-researcher / OpenAI SDK is
  not invoked with any retriever-mode flag (i.e. no web fetch happens).

### REQ-SYN-003 — Degraded Mode

- `test_degraded_mode_returns_doc_list`: inject a fake httpx transport
  that raises `httpx.ConnectError` on any `litellm:4000` request;
  assert `/synthesize` responds 200 within 2 seconds, body has
  `degraded=true`, `notice="litellm unavailable; returning raw doc
  list"`, `text` is exactly `\n`-joined `[N] {title} — {url}` lines,
  `citations` populated, `cost_usd=0.0`, `model=""`, `provider=""`.
- `test_degraded_mode_increments_counter`: assert
  `usearch_synthesis_calls_total{outcome="degraded"}` +1 (Go-side).
- `test_degraded_mode_does_not_call_llm`: mock LLM transport asserts
  zero outbound LLM requests in degraded mode.

### REQ-SYN-004 — Empty Input Rejection

- `test_empty_query_returns_400`: POST with `query: "   "` returns 400
  + body `{"error":"empty_input","detail":"query"}`.
- `test_zero_docs_returns_400`: POST with `docs: []` returns 400 +
  `detail: "docs"`.
- `test_empty_input_no_llm_call`: assert no LLM call happens for either
  failure case.
- `test_empty_input_logs_warn`: captured JSON log records contain
  exactly one WARN entry per failure with `{request_id, error}`.

### REQ-SYN-005 — Go-side HTTP Client Behavior

- `TestClientSynthesizeHappyPath`: `httptest.NewServer` returns canned
  200 JSON; assert returned `Result.Text` and `Result.Citations` match.
- `TestClientSynthesizeTimeout`: server sleeps 30 seconds; client
  configured with 1-second timeout; assert returned error wraps
  `context.DeadlineExceeded` and total elapsed ≤ 1.5 s.
- `TestClientSynthesizeRetriesOnConnReset`: server resets connection on
  first 2 attempts then returns 200; assert client retries twice
  (3 outbound TCP connections observed) and final result is the 200
  body.
- `TestClientSynthesize4xxNoRetry`: server returns 400; assert client
  makes exactly 1 outbound call and returns
  `synthesis.ErrInvalidRequest`.
- `TestClientSynthesizeEmitsSingleObservabilityPerCall`: even when 2
  retries occur, `obs.SynthesisCalls` increments only once and
  `obs.SynthesisLatency` records only one observation (the outermost
  call's elapsed time).
- `TestClientSynthesize5xxRetried`: server returns 503 then 200; assert
  retry occurs and success is returned.

### REQ-SYN-006 — Per-Call Observability

Python:
- `test_python_log_record_shape`: capture stdout via `capsys`; assert
  exactly 1 JSON line per `/synthesize` invocation with the 11
  documented attributes; `outcome` value is one of the 5 enums.
- `test_python_log_redacts_api_key`: assert no log record contains the
  `LITELLM_API_KEY` substring.

Go:
- `TestClientEmitsCounter`: outcome ∈ `{success, degraded,
  error_invalid, error_timeout, error_unreachable}` each fires the
  matching code path; counter increments by 1 each.
- `TestClientEmitsHistogram`: histogram count == 1 per call, sum > 0.
- `TestClientEmitsCostCounter`: success with `cost_usd=0.0023` →
  `usearch_synthesis_cost_usd` += 0.0023; degraded → no increment.
- `TestClientEmitsOTelSpan`: in-memory span exporter captures one span
  named `synthesis.call` with the 8 documented attributes.
- `TestClientObservabilitySafeOnNilObs`: construct Client with
  `obs: nil`; call does not panic; returns valid `Result`.

### REQ-SYN-007 — Language Hint (P1)

- `test_lang_hint_propagated_to_prompt`: request with `lang="ko"`;
  inspect captured LLM prompt; assert system message contains
  `"Answer in ko."`.
- `test_lang_empty_omits_directive`: request with `lang=""`; assert
  system message does NOT contain any `Answer in` directive.
- `test_lang_unknown_value_passes_through`: request with `lang="xx"`
  (invalid BCP-47) is accepted and passed through verbatim — Pydantic
  validation does NOT enforce BCP-47 well-formedness in V1.
- `test_lang_does_not_alter_citation_markers`: same input docs with
  and without `lang`; resulting `citations[].marker` mapping is
  identical (markers depend on input ordering, not on lang).

### NFR-SYN-001 — p50 Latency

- `test_synthesize_p50_latency_under_limit`: 50 sequential calls
  against a stub LiteLLM that responds with mean 2.5 s + jitter; sort
  durations; assert `durations[25] ≤ 8.0`.
- Stub LiteLLM lives in the test as a FastAPI fixture returning the
  minimum valid OpenAI chat-completion JSON with
  `x-litellm-response-cost: 0.0023`.

### NFR-SYN-002 — Marker→Doc Mapping Integrity

- Property test (`hypothesis>=6`): for arbitrary input `docs` with
  arbitrary LLM-generated `text` containing `[N]` markers, assert that
  every marker N in the returned `text` maps to a real input doc and
  that no marker references a doc absent from the input. Generator
  produces docs with valid `id` strings + adversarial LLM completions
  (markers in/out of range, repeated markers, mixed text).
- Negative test: hand-crafted LLM output `"Foo [1] bar [99] baz [2]"`
  with 2-doc input → returned `text` contains `[1]` and `[2]` only;
  `[99]` is removed.

### NFR-SYN-003 — Degraded-Mode Bound

- `test_degraded_response_within_2_seconds`: with mocked
  `httpx.ConnectError`, measure wall-clock time from POST to response;
  assert ≤ 2000 ms.
- `test_degraded_text_size_bound`: with 10 docs, assert
  `len(response.text) ≤ 10 * 320` and exactly 10 lines.

### NFR-SYN-004 — Cost Emission

- `test_cost_extracted_from_litellm_header`: stub returns
  `x-litellm-response-cost: 0.0042` → response `cost_usd == 0.0042`.
- `test_cost_missing_defaults_zero`: stub omits header → `cost_usd ==
  0.0`, no error, DEBUG log emitted (matches SPEC-LLM-001 REQ-LLM-006
  pattern).
- `test_cost_malformed_logs_warn`: stub returns
  `x-litellm-response-cost: notanumber` → `cost_usd == 0.0`, WARN log
  emitted, no exception raised.
- `TestClientCostCounterDegradedNotIncremented`: degraded response with
  `cost_usd=0.0` → `usearch_synthesis_cost_usd` unchanged.

---

## 5. Technical Approach

### 5.1 Files to Create

**Python sidecar** (`services/researcher/`):

- `src/researcher/__main__.py` — `python -m researcher` invokes
  `uvicorn.run(app, host="0.0.0.0", port=int(os.getenv("RESEARCHER_PORT","8081")))`.
- `src/researcher/app.py` — FastAPI app with `lifespan` async context
  manager that constructs the OpenAI SDK client wired to LiteLLM and
  pings `/health` once at startup; routes: `POST /synthesize`, `GET /health`.
- `src/researcher/models.py` — Pydantic v2 models (`NormalizedDocPayload`,
  `SynthesizeRequest`, `Citation`, `SynthesizeResponse`).
- `src/researcher/synthesis.py` — `async def synthesize(req:
  SynthesizeRequest, gateway: Gateway) -> SynthesizeResponse`:
  builds prompt via gpt-researcher's local-doc scaffold (or extracted
  equivalent), calls `gateway.complete(...)`, parses JSON output,
  validates marker→doc mapping per NFR-SYN-002, returns response.
- `src/researcher/gateway.py` — `class Gateway`: thin wrapper around
  `openai.AsyncOpenAI` with `base_url=$LITELLM_BASE_URL`,
  `api_key=$LITELLM_API_KEY`. Exposes `async complete(messages,
  model, lang) -> tuple[text, cost_usd, usage_dict, provider, model]`.
- `src/researcher/obs.py` — JSON-formatted stdlib `logging` setup +
  `class Timer` context manager; exposes `log_synthesis(record: dict)`.
- `src/researcher/__init__.py` — replace stub with package doc +
  `__version__` re-export.
- `tests/test_app.py` — FastAPI `TestClient` covering REQ-SYN-001/003/004/006/007.
- `tests/test_synthesis.py` — citation assembly logic, marker validation
  (REQ-SYN-002, NFR-SYN-002).
- `tests/test_gateway.py` — httpx mock for OpenAI SDK transport,
  asserting `LITELLM_BASE_URL` routing (REQ-SYN-002).
- `tests/test_obs.py` — JSON log record shape (REQ-SYN-006).

**Go-side**:

- `internal/synthesis/types.go` — `Request`, `Result`, `Citation`,
  error sentinels (`ErrInvalidRequest`, `ErrSidecarUnreachable`,
  `ErrTimeout`).
- `internal/synthesis/config.go` — env loader for
  `RESEARCHER_BASE_URL`, `RESEARCHER_REQUEST_TIMEOUT_SECONDS`.
- `internal/synthesis/client.go` — `Client` struct + `New(cfg, *obs.Obs)`
  + `Synthesize(ctx, query, lang, docs []types.NormalizedDoc) (Result, error)`.
- `internal/synthesis/synthesis.go` — replace 4-line stub with package
  doc + value-type re-exports.
- `internal/synthesis/client_test.go` — REQ-SYN-005 + REQ-SYN-006
  Go-side tests against `httptest.NewServer` returning canned JSON.
- `internal/obs/metrics/synthesis.go` — `SynthesisCalls`,
  `SynthesisLatency`, `SynthesisCost` collectors; `registerSynthesis(pr)`
  helper.

**Modified**:

- `services/researcher/pyproject.toml` — runtime deps additions per §2.1(b).
- `services/researcher/Dockerfile` — adds `CMD ["python","-m","researcher"]`
  (already present), adjusts `EXPOSE` if needed; adds `HEALTHCHECK
  CMD curl -f http://localhost:8081/health || exit 1`.
- `deploy/docker-compose.yml` — new `researcher` service entry.
- `internal/obs/metrics/metrics.go` — register synthesis collectors.
- `internal/obs/obs.go` — re-export `obs.SynthesisCalls`,
  `obs.SynthesisLatency`, `obs.SynthesisCost`.
- `.env.example` — append synthesis env vars.

### 5.2 Citation assembly algorithm

```
inputs:  query: str, lang: str, docs: list[NormalizedDocPayload]
output:  text: str, citations: list[Citation], cost_usd: float, ...

1. Build prompt:
   system = "You are a research synthesizer. Cite each fact with [N] where N
            is the 1-indexed source number from the SOURCES list. Use only
            facts present in the sources. Output one paragraph (4-8
            sentences). Do not invent sources."
   if lang: system += f"\n\nAnswer in {lang}."
   user = "QUESTION: {query}\n\nSOURCES:\n" +
           "\n".join(f"[{i+1}] {d.title}\n  URL: {d.url}\n  EXCERPT: {d.snippet or d.body[:1000]}"
                    for i, d in enumerate(docs))

2. response, cost, usage, provider, model = await gateway.complete(...)

3. Extract markers via regex /\[(\d+)\]/g; validate each N ∈ [1, len(docs)].
   - For each invalid marker: strip from text, log WARN.
4. Build citations: for each unique valid marker N (sorted asc):
        Citation(marker=N, doc_id=docs[N-1].id, url=docs[N-1].url, title=docs[N-1].title)

5. Return SynthesizeResponse(...)
```

### 5.3 Go HTTP client sketch

```go
// internal/synthesis/client.go (sketch — final shape in run phase)

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

func (c *Client) Synthesize(ctx context.Context, query string, lang string, docs []types.NormalizedDoc) (Result, error) {
    ctx, cancel := context.WithTimeout(ctx, c.httpClient.Timeout)
    defer cancel()

    ctx, span := c.obs.Tracer("synthesis").Start(ctx, "synthesis.call")
    defer span.End()

    started := time.Now()
    var outcome string
    defer func() {
        c.emitObs(ctx, outcome, time.Since(started), /*...*/)
    }()

    body, err := json.Marshal(buildPayload(ctx, query, lang, docs))
    if err != nil {
        outcome = "error_invalid"
        return Result{}, fmt.Errorf("synthesis: marshal: %w", err)
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
    case err != nil && resp.Degraded:
        outcome = "degraded"
    case err == nil && resp.Degraded:
        outcome = "degraded"
    case err == nil:
        outcome = "success"
    default:
        outcome = "error_invalid"
    }
    return resp, err
}
```

### 5.4 Compose service entry

```yaml
researcher:
  build:
    context: ../services/researcher
  ports:
    - "8081:8081"
  environment:
    LITELLM_BASE_URL: http://litellm:4000
    LITELLM_API_KEY: ${LITELLM_MASTER_KEY}
    RESEARCHER_PORT: "8081"
    RESEARCHER_MODEL_DEFAULT: claude-haiku-4-5
    RESEARCHER_TIMEOUT_SECONDS: "8"
    RESEARCHER_LOG_LEVEL: INFO
  depends_on:
    litellm:
      condition: service_healthy
  healthcheck:
    test: ["CMD", "curl", "-f", "http://localhost:8081/health"]
    interval: 30s
    timeout: 5s
    retries: 3
```

### 5.5 Harness Level

Per `.moai/config/sections/harness.yaml` auto-routing: 7 REQs (6 × P0 +
1 × P1) + 4 NFRs across 2 languages (Python sidecar + Go client) + 1
new compose service + new metric family = **standard** harness. Sprint
Contract optional but recommended.

---

## 6. File Impact

### 6.1 Created

| Path | Purpose |
|------|---------|
| `services/researcher/src/researcher/app.py` | FastAPI app + routes (REQ-SYN-001) |
| `services/researcher/src/researcher/models.py` | Pydantic v2 models (REQ-SYN-001) |
| `services/researcher/src/researcher/synthesis.py` | gpt-researcher integration (REQ-SYN-002) |
| `services/researcher/src/researcher/gateway.py` | OpenAI SDK transport via LiteLLM (REQ-SYN-002, NFR-SYN-004) |
| `services/researcher/src/researcher/obs.py` | JSON log records (REQ-SYN-006) |
| `services/researcher/src/researcher/__main__.py` | Uvicorn entrypoint |
| `services/researcher/tests/test_app.py` | Endpoint contract tests |
| `services/researcher/tests/test_synthesis.py` | Citation assembly tests |
| `services/researcher/tests/test_gateway.py` | LiteLLM routing tests |
| `services/researcher/tests/test_obs.py` | Log record shape tests |
| `internal/synthesis/types.go` | Go value types + error sentinels |
| `internal/synthesis/config.go` | Env binding |
| `internal/synthesis/client.go` | Go HTTP client (REQ-SYN-005, REQ-SYN-006) |
| `internal/synthesis/client_test.go` | Go client RED tests |
| `internal/obs/metrics/synthesis.go` | New metric family declaration |

### 6.2 Modified

| Path | Change |
|------|--------|
| `services/researcher/src/researcher/__init__.py` | Replace stub with package doc + version |
| `services/researcher/pyproject.toml` | Add runtime deps (fastapi, uvicorn, pydantic, httpx, gpt-researcher, openai) |
| `services/researcher/Dockerfile` | Add HEALTHCHECK + multi-stage if needed |
| `internal/synthesis/synthesis.go` | Replace stub with package doc + re-exports |
| `internal/obs/metrics/metrics.go` | Register synthesis collectors in `NewRegistry()` |
| `internal/obs/obs.go` | Add `SynthesisCalls`, `SynthesisLatency`, `SynthesisCost` re-exports |
| `deploy/docker-compose.yml` | Add `researcher` service |
| `.env.example` | Append `RESEARCHER_*` env vars |

### 6.3 Unchanged (by design)

- `internal/llm/*` — synthesis does not consume the Go-side LLM client;
  Python sidecar talks to LiteLLM directly via OpenAI SDK.
- `internal/router/*`, `internal/adapters/*`, `internal/fanout/*` —
  no API change; synthesis is downstream of all of them.
- `pkg/types/*` — `NormalizedDoc` is the input contract, unchanged.
- `deploy/litellm/config.yaml` — `claude-haiku-4-5` already declared.

---

## 7. Test Plan

Development follows RED-GREEN-REFACTOR per `quality.development_mode:
tdd`. Coverage target 80% per SPEC frontmatter (lower than the 85%
project default because the gpt-researcher integration includes
prompt-assembly code paths that are exercised primarily through
end-to-end tests, not unit tests).

Representative RED-phase tests (in addition to the acceptance criteria
already enumerated in §4):

| # | Test | Layer | REQ | Assertion |
|---|------|-------|-----|-----------|
| 1 | `test_synthesize_happy_path` | `tests/test_app.py` | REQ-SYN-001 | 200 + valid response shape |
| 2 | `test_synthesize_extra_field_rejected` | `tests/test_app.py` | REQ-SYN-001 | 422 on unknown field |
| 3 | `test_llm_call_routed_through_litellm` | `tests/test_gateway.py` | REQ-SYN-002 | Outbound URL == LITELLM_BASE_URL |
| 4 | `test_authorization_header_sent` | `tests/test_gateway.py` | REQ-SYN-002 | Bearer token from LITELLM_API_KEY |
| 5 | `test_marker_out_of_range_stripped` | `tests/test_synthesis.py` | REQ-SYN-002 | `[5]` removed from text when 3 docs |
| 6 | `test_doc_id_resolution_uses_input_ids` | `tests/test_synthesis.py` | REQ-SYN-002 | `citations[i].doc_id == docs[marker-1].id` |
| 7 | `test_degraded_mode_returns_doc_list` | `tests/test_app.py` | REQ-SYN-003 | 200 + degraded=true + bullet list |
| 8 | `test_degraded_mode_does_not_call_llm` | `tests/test_app.py` | REQ-SYN-003 | LLM transport invocation count == 0 |
| 9 | `test_empty_query_returns_400` | `tests/test_app.py` | REQ-SYN-004 | 400 + error=empty_input |
| 10 | `test_zero_docs_returns_400` | `tests/test_app.py` | REQ-SYN-004 | 400 + detail=docs |
| 11 | `test_python_log_record_shape` | `tests/test_obs.py` | REQ-SYN-006 | 1 JSON log per call, 11 attrs |
| 12 | `test_python_log_redacts_api_key` | `tests/test_obs.py` | REQ-SYN-006 | API key never in log output |
| 13 | `test_lang_hint_propagated_to_prompt` | `tests/test_synthesis.py` | REQ-SYN-007 | system msg contains "Answer in ko." |
| 14 | `test_synthesize_p50_latency_under_limit` | `tests/test_app.py` (slow) | NFR-SYN-001 | 50 sequential calls, p50 ≤ 8s |
| 15 | `test_marker_property_holds_for_arbitrary_input` | `tests/test_synthesis.py` (hypothesis) | NFR-SYN-002 | Property: every marker maps to real doc |
| 16 | `test_degraded_response_within_2_seconds` | `tests/test_app.py` | NFR-SYN-003 | Wall-clock ≤ 2000 ms |
| 17 | `test_cost_extracted_from_litellm_header` | `tests/test_gateway.py` | NFR-SYN-004 | cost_usd == header value |
| 18 | `test_cost_missing_defaults_zero` | `tests/test_gateway.py` | NFR-SYN-004 | Missing header → 0.0, no error |
| 19 | `test_cost_malformed_logs_warn` | `tests/test_gateway.py` | NFR-SYN-004 | Garbage header → 0.0 + WARN log |
| 20 | `TestClientSynthesizeHappyPath` | `internal/synthesis/client_test.go` | REQ-SYN-005 | 200 JSON parsed into Result |
| 21 | `TestClientSynthesizeTimeout` | `client_test.go` | REQ-SYN-005 | Server sleeps; client times out |
| 22 | `TestClientSynthesizeRetriesOnConnReset` | `client_test.go` | REQ-SYN-005 | 2 retries on conn reset |
| 23 | `TestClientSynthesize4xxNoRetry` | `client_test.go` | REQ-SYN-005 | 400 → no retry, ErrInvalidRequest |
| 24 | `TestClientSynthesize5xxRetried` | `client_test.go` | REQ-SYN-005 | 503 then 200 → success |
| 25 | `TestClientSynthesizeEmitsSingleObservabilityPerCall` | `client_test.go` | REQ-SYN-005, REQ-SYN-006 | Counter +1, histogram +1 even on retry |
| 26 | `TestClientEmitsCounter` | `client_test.go` | REQ-SYN-006 | Each outcome enum increments correctly |
| 27 | `TestClientEmitsHistogram` | `client_test.go` | REQ-SYN-006 | count == 1, sum > 0 |
| 28 | `TestClientEmitsCostCounter` | `client_test.go` | REQ-SYN-006, NFR-SYN-004 | success increments, degraded does not |
| 29 | `TestClientEmitsOTelSpan` | `client_test.go` | REQ-SYN-006 | span name + 8 attrs |
| 30 | `TestClientObservabilitySafeOnNilObs` | `client_test.go` | REQ-SYN-006 | obs: nil → no panic |

Python: `pytest -q services/researcher/tests/` with `pytest-asyncio`
(`asyncio_mode="auto"`) and `httpx`'s built-in mock transport. Coverage
via `pytest --cov=researcher --cov-report=term-missing`, target 80%.

Go: `go test -race ./internal/synthesis/...` against
`httptest.NewServer` returning canned JSON. Coverage via `go test
-coverprofile=...`, target 80%.

Brownfield note: `internal/synthesis/synthesis.go` and
`services/researcher/src/researcher/__init__.py` exist as stubs with no
behavior to preserve. Per `workflow-modes.md` §Brownfield Enhancement,
characterization tests are not needed; RED tests are written against
the planned package surface.

---

## 8. Dependencies

### 8.1 Upstream SPEC Dependencies

- **SPEC-CORE-001 (implemented)**: provides
  `pkg/types.NormalizedDoc` shape — the request payload schema.
- **SPEC-LLM-001 (implemented)**: provides the LiteLLM proxy at
  `LITELLM_BASE_URL` (default `http://litellm:4000`),
  `LITELLM_MASTER_KEY` env convention, and the `x-litellm-response-cost`
  header semantics. Python sidecar reuses these; the Go-side
  `internal/llm` package is NOT in the synthesis call path.
- **SPEC-BOOT-001 (implemented)**: provides `services/researcher/`
  scaffold (pyproject + Dockerfile + stub) and
  `internal/synthesis/synthesis.go` 4-line stub.

### 8.2 Coordinating SPECs (no hard dependency)

- **SPEC-IR-001 (implemented)**: `RoutingDecision.Lang` flows in as
  the `lang` field hint (REQ-SYN-007). No code dependency — synthesis
  accepts an optional string.
- **SPEC-OBS-001 (implemented)**: provides `obs.Logger`, `obs.Tracer`,
  metric registry, and the `internal/obs/metrics/` location pattern.
  SPEC-SYN-001 adds `internal/obs/metrics/synthesis.go` following the
  SPEC-IR-001 precedent (`internal/obs/metrics/router.go`).

### 8.3 Downstream Blocked SPECs

- **SPEC-SYN-002 (M4)**: citation faithfulness scoring builds on
  SPEC-SYN-001's marker→doc structural mapping. NFR-SYN-002 is the
  foundation; SPEC-SYN-002 adds claim-level grounding measurement.
- **SPEC-SYN-003 (M4)**: chunking + embedding-based source selection
  replaces SPEC-SYN-001's "snippet stuffing" prompt.
- **SPEC-SYN-004 (M4)**: streaming synthesis adds
  `POST /synthesize/stream` (SSE) without breaking the V1 contract.

### 8.4 External Dependencies (run-phase pins)

New Python runtime dependencies:

```
fastapi >= 0.115
uvicorn[standard] >= 0.30
pydantic >= 2.9
httpx >= 0.27
gpt-researcher >= 0.10        # may be replaced by extracted equivalent — see Open Question §11.1
openai >= 1.50
```

No new Go module dependencies (uses stdlib `net/http`, `encoding/json`).

No new external services at SPEC-SYN-001 runtime by default — all LLM
traffic flows through the existing LiteLLM proxy from SPEC-LLM-001.

---

## 9. Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| `gpt-researcher` package pulls a heavyweight transitive dep tree (Selenium, full LangChain) inflating Docker image > 2 GB | High | Medium | Open Question §11.1: fallback to extracting just the prompt scaffolds (~150 LoC of Python) if image size becomes a build-time issue |
| LLM hallucinates citation markers that don't map to any input doc | High | Medium | NFR-SYN-002 marker validation strips invalid markers; faithfulness scoring is SPEC-SYN-002's job |
| LLM emits non-JSON-parseable output despite system prompt directives | Medium | Medium | V1 uses regex marker extraction (not full JSON parsing of the prose) — the prose is plain text, only the marker numbers are extracted via `/\[(\d+)\]/g`. Resilient to LLM verbosity |
| LiteLLM proxy unreachable cascades into long timeouts on `/synthesize` | Medium | High | NFR-SYN-003 graceful degradation returns within 2 s with bullet-list payload; the gateway pings LiteLLM at startup and on every request via httpx with explicit `connect_timeout=2.0` |
| Cross-service trace propagation breaks if Python service does not honor `traceparent` header | Medium | Low | FastAPI middleware extracts `traceparent` and injects into the OpenAI SDK request via OTel Python SDK — covered by integration test in run phase |
| Claude Haiku 4.5 quality insufficient for citation faithfulness | Medium | Low (V1) / High (M4) | V1 only requires structural mapping (NFR-SYN-002), not faithfulness; SPEC-SYN-002 will measure and may bump default to Sonnet |
| Pydantic v2 `extra="forbid"` rejects benign client additions in future | Low | Low | Documented in `.env.example`; client and server evolve together; backward compat is checked in run-phase contract tests |
| Python sidecar Dockerfile reaches non-trivial build time (gpt-researcher install) | Medium | Low | Multi-stage build with deps cached as separate layer; CI caches the deps layer |
| Go client retry + sidecar internal retry (via LiteLLM) cause 9× call amplification | Low | Medium | Sidecar does NOT retry the LLM call (relies on LiteLLM internal retry); Go client retries only on connection-level errors, not on HTTP 5xx; documented in §5.3 |
| `usearch_synthesis_cost_usd` Counter (no labels) loses provider/model attribution | Low | Low | Provider/model attribution already exists on `usearch_llm_cost_usd_total{provider,model}` from SPEC-LLM-001; the synthesis-domain counter is a rollup, not a replacement |
| Pre-flight ping to LiteLLM at startup fails in dev environments without `.env` | Medium | Low | Health endpoint returns 503 with `reason="litellm unreachable"` without crashing the service; allows dev to bring up `researcher` and `litellm` independently |

---

## 10. Open Questions

The following are explicitly unresolved and documented here rather than
pre-decided. They do not block SPEC approval.

1. **gpt-researcher full install vs extracted scaffolds.** Default:
   install full `gpt-researcher >= 0.10` and live with the Docker image
   size. Revisit in M4 if image > 2 GB or transitive vulnerabilities
   surface in `pip-audit`.

2. **Default synthesis model.** Default: `claude-haiku-4-5`
   (`RESEARCHER_MODEL_DEFAULT`). SPEC-SYN-002 may bump to Sonnet based
   on faithfulness measurement.

3. **Per-request model override field in payload.** V1 accepts
   `request.model` (optional string) so callers can override the
   default. If unset, falls back to `RESEARCHER_MODEL_DEFAULT`. Open:
   should V1 expose this at all? Default: yes, exposed but not
   required.

4. **Snippet-only vs body-truncated prompt content.** V1 uses
   `Snippet` if present, else `Body[:1000]`. Open: should we use
   `Body` always and let the LLM provider's tokenizer truncate?
   Default: explicit truncation Python-side for cost predictability.
   SPEC-SYN-003 introduces proper chunking.

5. **Number of docs cap.** V1 accepts `len(docs) ≤ 50` (Pydantic
   validator). Open: lower it to 20 to avoid prompt bloat? Default: 50,
   with a documented WARN log when `len > 20`.

6. **Health endpoint upstream ping.** V1 pings LiteLLM `/health` at
   startup AND on every `/health` call. Open: should `/health` always
   return 200 if the FastAPI process is alive, regardless of upstream?
   Default: report degraded via 503 so docker-compose `service_healthy`
   condition is meaningful.

7. **Concurrent `/synthesize` handling.** V1 uses `uvicorn` with
   `--workers 1` and async handlers — concurrency comes from asyncio,
   not multiprocessing. Open: scale to multiple workers? Default: 1
   worker; revisit when adapter throughput requires it (M3).

8. **Cost counter granularity.** `usearch_synthesis_cost_usd` is a
   single counter without `provider`/`model` labels. Open: should we
   add `provider`/`model` labels to match the LLM-side counter?
   Default: no — the LLM-side counter already provides that
   attribution; the synthesis-domain counter is a coarser business
   rollup. Documented as a deliberate choice in §6.1.

---

## 11. References

Internal:

- `pkg/types/normalized_doc.go` — input shape (SPEC-CORE-001).
- `internal/llm/config/config.go` — env binding pattern.
- `internal/llm/client.go:230-252` — per-call observability emit pattern
  to mirror in Go-side synthesis client.
- `internal/obs/metrics/llm.go`, `internal/obs/metrics/router.go` —
  precedent for new metric family file under `internal/obs/metrics/`.
- `services/researcher/` — current Python scaffold.
- `internal/synthesis/synthesis.go` — current Go-side stub.
- `.moai/specs/SPEC-CORE-001/spec.md` — NormalizedDoc field semantics.
- `.moai/specs/SPEC-LLM-001/spec.md` §2.2 Out-of-Scope (Python services
  route through LiteLLM directly).
- `.moai/specs/SPEC-OBS-001/spec.md` — metric naming + cardinality
  discipline.
- `.moai/specs/SPEC-IR-001/spec.md` §2.1 — `services/researcher`
  reserved for synthesis (not classification).
- `.moai/project/roadmap.md` §M2 exit criterion.
- `.moai/specs/SPEC-SYN-001/research.md` — companion research artifact.

External:

- `https://github.com/assafelovic/gpt-researcher` — Apache-2.0;
  planner-executor-publisher pattern; local-doc mode via
  `report_source="local"`; OpenAI-compatible LLM via
  `OPENAI_BASE_URL`. Verified 2026-04-28.
- FastAPI 0.115+ patterns — `tiangolo/fastapi`. Per
  `.claude/rules/moai/languages/python.md`.
- Pydantic v2.9 — `pydantic/pydantic`.

---

*End of SPEC-SYN-001 v0.1 (draft)*
