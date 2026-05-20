# SPEC-DEEP-001 Acceptance Criteria

Companion artifact for `.moai/specs/SPEC-DEEP-001/spec.md`.
Version: 0.3.1
Created: 2026-05-10
Author: limbowl (via manager-spec)

---

## 1. Definition of Done

SPEC-DEEP-001 is **DONE** when ALL of the following hold:

- [ ] `services/storm/src/storm/` package exists with all 9
      documented modules (`__main__.py`, `app.py`, `models.py`,
      `gateway.py`, `obs.py`, `pipeline.py`, `inject_rm.py`,
      `citation_translator.py`, `faithfulness.py`).
- [ ] `services/storm/tests/` passes with ≥85% coverage on every
      new module.
- [ ] `internal/deepreport/`, `internal/streamsynth/longform.go`,
      `internal/obs/metrics/deepreport.go` exist and pass
      `go test -race ./...` with ≥85% coverage.
- [ ] All 6 EARS REQs (REQ-DEEP1-001 through REQ-DEEP1-006) have
      corresponding green tests.
- [ ] All 3 NFRs (NFR-DEEP1-001 latency, NFR-DEEP1-002 invariant,
      NFR-DEEP1-003 exactly-once outcome counter) are verified by
      automated tests.
- [ ] SPEC-SYN-001 / SPEC-SYN-002 / SPEC-SYN-004 acceptance test
      suites remain 100% green (regression check) — single-paragraph
      synthesis path is unchanged.
- [ ] Two new Prometheus collectors (`DeepReportOutcomes`,
      `DeepReportLatency`) registered; the 6 new `outcome` values
      pre-initialised per the SYN-004 `streamsynth.go:48-56`
      pattern; no cardinality allowlist amendment required (the
      `outcome` label NAME is pre-existing — see plan.md §3.5).
- [ ] `STORM_*` env vars documented in `services/storm/.env.example`.
- [ ] Conventional commit `feat(deep): SPEC-DEEP-001 STORM long-form
      report sidecar` references this SPEC ID.
- [ ] @MX tags applied per `plan.md` §3.7.
- [ ] TRUST 5 gates passed (tested, readable, unified, secured,
      trackable).
- [ ] LSP gates passed (zero errors, zero type errors, zero lint
      errors per `quality.yaml` run-phase thresholds).
- [ ] Pre-submission self-review per `workflow-modes.md` confirms
      no simpler approach achieves the same result.
- [ ] Sprint Contract artifacts captured in `.moai/sprints/` per
      design constitution §11 (harness=thorough requires Sprint
      Contracts).
- [ ] M5 exit criterion ≥10 cited sources verified on at least one
      golden topic (a real run via mocked LM is acceptable for
      acceptance; a real-LM run is part of /moai sync verification).

---

## 2. Acceptance Scenarios (Given-When-Then)

### Scenario 1 — Happy path: structured report with 3 sections, 12 sentences, 14 cited sources

**Given**
- `STORM_FAITHFULNESS_MODE=strip` (default)
- A `POST /generate_report` request with
  `query="What is GPT-4?"`, `lang="en"`, and 30 valid `docs[]`
  (varied URLs spanning 14 distinct sources)
- The mocked STORM pipeline produces a 3-section article with 12
  sentences total, all sentences correctly cited via URLs that
  resolve to 14 of the 30 input docs

**When**
- The Python sidecar processes the request

**Then**
- HTTP status 200
- `response.title` is non-empty
- `response.sections` length == 3
- Sum of `response.sections[].sentences` length == 12
- `response.citations` length == 14, sorted by `marker`
  ascending, 1-indexed
- Every `[N]` in `response.sections[].text` resolves to a
  `Citation.doc_id` in `response.citations`
- `response.degraded == false`
- `response.notice == ""`
- `response.cost_usd > 0` (matches mock-summed cost)
- `response.latency_ms > 0`
- `response.schema_version == 1`
- Counter `usearch_deep_outcomes_total{outcome="success"}` += 1
- Histogram `usearch_deep_latency_seconds` observation recorded
- Counter `usearch_storm_faithfulness_outcomes_total{outcome="accepted"}`
  += 1
- Counter `usearch_storm_unresolved_citations_total` unchanged
- JSON log record contains
  `{request_id, outcome: "success", sections_count: 3,
    sentences_count: 12, citations_count: 14}`

### Scenario 2 — Faithfulness strip mode: un-cited sentences removed

**Given**
- `STORM_FAITHFULNESS_MODE=strip`
- Request with 10 `docs[]`
- Mocked STORM pipeline produces 2 sections; section 1 has 4
  sentences (sentences 1, 3 cited; sentences 2, 4 un-cited);
  section 2 has 3 sentences all cited

**When**
- The Python sidecar processes the request

**Then**
- HTTP status 200
- `response.sections` length == 2
- `response.sections[0].sentences` length == 2 (sentences 2, 4
  stripped)
- `response.sections[0].text` contains only the 2 cited sentences,
  joined by single space
- `response.sections[1].sentences` length == 3 (unchanged)
- `response.notice` == `"2 uncited sentence(s) stripped across 1
  section(s)"`
- Counter `usearch_storm_faithfulness_outcomes_total{outcome="stripped"}`
  += 1
- Counter `usearch_deep_outcomes_total{outcome="success"}` += 1

### Scenario 3 — Faithfulness reject mode: HTTP 422 on un-cited content

**Given**
- `STORM_FAITHFULNESS_MODE=reject`
- Request with 10 `docs[]`
- Mocked STORM pipeline produces 2 sections; one section has 1
  un-cited sentence

**When**
- The Python sidecar processes the request

**Then**
- HTTP status **422**
- Response body matches schema:
  `{"error": "un_cited_long_form",
    "detail": "1 uncited sentence(s) across 1 section(s)",
    "uncited_count": 1, "sections_affected": 1}`
- Response body does NOT contain `sections`, `text`, or
  `citations` fields (no leakage)
- Counter `usearch_storm_faithfulness_outcomes_total{outcome="rejected"}`
  += 1
- Counter `usearch_deep_outcomes_total{outcome="error_invalid"}` += 1
  (422 maps to `error_invalid` on Go-side per REQ-DEEP1-001)
- Go-side `internal/deepreport.Client.GenerateReport` returns
  `errors.Is(err, deepreport.ErrInvalidRequest) == true`
- Exactly 1 WARN-level log record with attributes
  `{request_id, faithfulness_action: "rejected",
    uncited_count: 1, sections_affected: 1}`

### Scenario 4 — Faithfulness off mode: gate bypassed

**Given**
- `STORM_FAITHFULNESS_MODE=off`
- Request with 10 `docs[]`
- Mocked STORM pipeline produces 2 sections with mixed cited /
  un-cited sentences

**When**
- The Python sidecar processes the request

**Then**
- HTTP status 200
- `response.sections` contain ALL original sentences (no
  stripping)
- Un-cited sentences appear verbatim
- `response.notice == ""` (no stripping)
- Counter `usearch_storm_faithfulness_outcomes_total{outcome="off"}`
  += 1 (DEEP-001 emits counter in mode=off, unlike SYN-002 — see
  plan.md §10 D1)
- Counter `usearch_deep_outcomes_total{outcome="success"}` += 1

### Scenario 5 — Deadline exceeded: HTTP 504 after 5 min

**Given**
- `STORM_MAX_LATENCY_MS=300000` (5 min)
- Mocked STORM pipeline blocks for 301 s

**When**
- The Python sidecar processes the request

**Then**
- HTTP status **504**
- Response body matches schema:
  `{"error": "deadline_exceeded",
    "detail": "STORM pipeline exceeded 300000 ms deadline",
    "elapsed_ms": <approx 300000>,
    "partial_sections_completed": <N>}`
- Response body does NOT contain `text`, `sections`, or
  `citations` fields
- Counter `usearch_deep_outcomes_total{outcome="deadline_exceeded"}`
  += 1
- Histogram observation recorded near the 300 s bucket
- Exactly 1 WARN-level log record with attributes
  `{request_id, reason: "deadline_exceeded",
    elapsed_ms: <approx 300000>,
    partial_sections_completed: <N>}`
- Go-side client returns
  `errors.Is(err, deepreport.ErrDeadlineExceeded) == true`
- Goroutine-leak detector PASS (no leaked goroutines from the
  cancellation path)

### Scenario 6 — Budget exceeded: HTTP 402 mid-pipeline

**Given**
- `STORM_MAX_COST_USD=2.50`
- Mocked LiteLLM responses produce a cumulative cost of 2.51 USD
  after the third internal call (mid-research stage)

**When**
- The Python sidecar processes the request

**Then**
- HTTP status **402**
- Response body matches schema:
  `{"error": "budget_exceeded",
    "detail": "cumulative cost 2.51 USD exceeded cap 2.50 USD",
    "cost_usd": 2.51, "cap_usd": 2.50}`
- Counter `usearch_deep_outcomes_total{outcome="budget_exceeded"}`
  += 1
- Go-side client returns
  `errors.Is(err, deepreport.ErrBudgetExceeded) == true`
- WARN-level log record with attributes
  `{request_id, reason: "budget_exceeded", cost_usd, cap_usd}`

### Scenario 7 — SSE streaming: section + sentence + done events

**Given**
- Request via `cmd/usearch-api POST /deep` with header
  `Accept: text/event-stream`
- Mocked sidecar response: 2 sections (3 sentences in section 1,
  2 sentences in section 2), 4 citations

**When**
- `internal/streamsynth.StreamLongFormReport` walks the report

**Then**
- Response headers `Content-Type: text/event-stream`,
  `Cache-Control: no-cache`, `Connection: keep-alive`
- Event sequence on the wire (in order):
  1. `event: section_start` for section 0 (heading, level)
  2. `event: sentence` ×3 for section 0 (each carries
     `section_index: 0`, `sentence_index: 0..2`, `citations[]`)
  3. `event: section_done` for section 0 (sentences_emitted: 3)
  4. `event: section_start` for section 1
  5. `event: sentence` ×2 for section 1
  6. `event: section_done` for section 1 (sentences_emitted: 2)
  7. `event: done` with payload
     `{request_id, total_sections: 2, total_sentences: 5,
       latency_ms, model, provider, cost_usd, schema_version: 1}`
- Every event terminated by `\n\n` (W3C SSE wire format)
- Heartbeat `: ping\n\n` emitted at
  `SYN004_SSE_HEARTBEAT_MS` interval if stream lasts long enough
- No `event: sentence` whose `text` lacks a `[N]` marker reaches
  the wire (SYN-002 invariant preserved)
- Counter `usearch_syn004_outcomes_total{outcome="streamed_complete"}`
  += 1 (inherited)
- Counter `usearch_deep_outcomes_total{outcome="success"}` += 1

### Scenario 8 — Accept-header fallback: buffered JSON response

**Given**
- Request via `cmd/usearch-api POST /deep` WITHOUT
  `Accept: text/event-stream` (header absent or
  `application/json`)
- Same mocked sidecar response as Scenario 7

**When**
- The handler processes the request

**Then**
- HTTP status 200
- `Content-Type: application/json`
- Response body matches `GenerateReportResponse` schema
  byte-equivalent (modulo `request_id`) to a direct sidecar
  `POST /generate_report` call with the same input
- No SSE writer constructed (verified via mock-call counter)
- No heartbeat goroutine launched (verified via mock-call counter)
- Counter `usearch_syn004_outcomes_total{outcome="accept_fallback_to_json"}`
  += 1 (inherited)
- Counter `usearch_deep_outcomes_total{outcome="success"}` += 1

### Scenario 9 — Korean-language long-form: mixed punctuation segmentation

**Given**
- Request with `lang="ko"` and 15 Korean-language `docs[]`
- Mocked STORM pipeline produces 3 sections with Korean prose:
  ```
  Section 1 sentence 1: "GPT-4는 2023년에 출시되었습니다 [1]."
  Section 1 sentence 2: "그 이전 모델은 GPT-3.5였습니다 [2]."
  Section 2 sentence 1: "한국어 처리가 개선되었습니다 [3]."
  ...
  ```
  (all sentences cited; mix of `.` (English-style period) and
  `。` (full-width period))

**When**
- The Python sidecar processes the request with default
  `STORM_FAITHFULNESS_MODE=strip`

**Then**
- Sentence segmentation by canonical regex
  `[.!?。！？]\s+|[.!?。！？]$` correctly identifies all sentences
  regardless of punctuation style
- Faithfulness gate accepts all sentences (all cited)
- HTTP 200; response shape well-formed
- Counter `usearch_storm_faithfulness_outcomes_total{outcome="accepted"}`
  += 1
- SSE path emits sentences in correct order with Korean text
  preserved verbatim (UTF-8 throughout)

### Scenario 10 — End-to-end via cmd/usearch-api: M5 exit criterion

**Given**
- `STORM_FAITHFULNESS_MODE=strip`, default knobs
- A real or mocked-but-realistic STORM pipeline producing a
  report with 5 sections totaling ≥ 20 sentences
- Input docs containing ≥ 15 distinct sources

**When**
- POST request to `cmd/usearch-api /deep` with
  `Accept: text/event-stream`

**Then**
- Stream completes successfully with `event: done`
- `response.citations` length ≥ 10 (M5 exit criterion per
  `roadmap.md` line 154)
- End-to-end latency ≤ 300 s (NFR-DEEP1-001 p95)
- All other invariants from previous scenarios hold

---

## 3. Edge Cases

### Edge Case 1 — Empty `docs[]` array

**Given**
- Request with `docs == []`

**Then**
- The Python sidecar SHALL respond with HTTP 422 and body
  `{"error": "invalid_request", "detail": "docs[] is empty"}`.
- Counter `usearch_deep_outcomes_total{outcome="error_invalid"}`
  += 1.
- Go-side client returns `errors.Is(err,
  deepreport.ErrInvalidRequest) == true`.

### Edge Case 2 — Empty `query` string

**Given**
- Request with `query == ""`

**Then**
- Same as Edge Case 1 — 422 with `{"error": "invalid_request",
  "detail": "query is empty"}`.

### Edge Case 3 — All sentences un-cited, mode=strip → all sections empty → 422

**Given**
- `STORM_FAITHFULNESS_MODE=strip`
- Mocked STORM pipeline produces 2 sections; ALL sentences
  un-cited (worst case)

**Then**
- Empty-section removal causes `response.sections` to be empty.
- The sidecar SHALL respond with HTTP 422 body
  `{"error": "un_cited_long_form",
    "detail": "<N> uncited sentence(s) across <S> section(s)",
    "uncited_count": N, "sections_affected": S}` even though
  configured mode was `strip` — empty long-form is a degenerate
  outcome handled identically to the `reject` path. (Parallels
  SPEC-SYN-002 Edge Case 1 D1 decision: empty post-strip text
  raises UncitedOutputError.)
- Counter `usearch_storm_faithfulness_outcomes_total{outcome="rejected"}`
  += 1 (the terminal outcome is reject, regardless of configured
  mode).

### Edge Case 4 — STORM produces a marker outside the references list

**Given**
- Mocked STORM article text: `"Foo [99] bar [1]."` with
  `storm_refs = [{n: 1, url: "https://example.com"}]` (only
  ref 1 is in the refs list)

**Then**
- Citation translator strips `[99]` (no matching storm_ref →
  unresolved → counter +1).
- Result: text `"Foo  bar [1]."` (with double space; collapsed in
  segmentation).
- Counter `usearch_storm_unresolved_citations_total` += 1.

### Edge Case 5 — STORM marker references a real ref, but ref URL not in `docs[]`

**Given**
- Mocked STORM article text: `"Foo [1]."` with
  `storm_refs = [{n: 1, url: "https://wikipedia.org/wiki/foo"}]`
  but `docs[]` contains only Reddit + HN URLs (no Wikipedia)

**Then**
- Citation translator: `canonicalize_url("https://wikipedia.org/wiki/foo")`
  has no match in `docs[].url` set.
- `[1]` is stripped from text; counter +1.
- The sentence "Foo " becomes un-cited; faithfulness gate (mode
  default `strip`) removes the sentence.
- Net result: section may become empty (Edge Case 3 path), or
  retain only other cited sentences.

### Edge Case 6 — Single-section, single-sentence report

**Given**
- Mocked STORM pipeline produces 1 section with 1 cited sentence

**Then**
- Response well-formed: `sections` length 1, `sentences` length 1.
- SSE path emits: `section_start, sentence, section_done, done` —
  4 events total.
- All invariants hold (NFR-DEEP1-002).

### Edge Case 7 — Section heading is empty string

**Given**
- Mocked STORM produces a section with `heading: ""`

**Then**
- Per NFR-DEEP1-002 (d), the sidecar SHALL filter out
  empty-heading sections before response serialization.
- If filtering leaves zero sections → Edge Case 3 path (HTTP 422).
- Test: `test_property_section_heading_non_empty`.

### Edge Case 8 — Concurrent requests with different modes

**Given**
- Two concurrent `/generate_report` requests
- Request A reads `STORM_FAITHFULNESS_MODE=strip` at start
- Mid-request, env var mutated to `reject` (operator change)
- Request B reads `reject`

**Then**
- Each request uses the value at its own start (per-request local
  variable; no shared mutable state across requests).
- Behavior deterministic per-request.
- Tested via concurrent invocation with mode-mutation between
  request acceptances.

### Edge Case 9 — Mode env var typo

**Given**
- `STORM_FAITHFULNESS_MODE=stript` (typo)

**Then**
- The Python sidecar SHALL log a WARN-level message
  `"Unknown STORM_FAITHFULNESS_MODE='stript'; defaulting to 'strip'"`
  and proceed with `mode=strip`.
- Documented in run phase as defensive default; preferred over
  raising on startup which would block the service. Mirrors SYN-002
  Edge Case 8.

### Edge Case 10 — Client disconnects mid-stream during 4-min STORM run

**Given**
- Client opens SSE connection; sidecar begins STORM pipeline
- After ~120 s (mid-research, no sections produced yet), client
  closes connection

**Then**
- Disconnect-watcher goroutine in `streamsynth` detects
  `r.Context().Done()` (inherited from SYN-004 REQ-SYN4-004).
- Parent ctx cancel propagates to `internal/deepreport.Client.GenerateReport`,
  which propagates to the sidecar via TCP close.
- Sidecar's `asyncio.wait_for` task receives cancellation; STORM
  pipeline cancelled; LLM calls in-flight cancelled.
- Counter `usearch_syn004_outcomes_total{outcome="client_disconnect"}`
  += 1 (inherited from SYN-004).
- Counter `usearch_deep_outcomes_total` follows the
  **disconnect carve-out** of NFR-DEEP1-003: at-most-once when the
  client disconnects mid-stream. In practice this means the Go-side
  counter MAY remain at zero for the request (the handler returns
  from streaming before the terminal outcome guard fires). The
  sidecar may independently emit `outcome="error_upstream"` on its
  own observation channel; this is NOT a double-count because the
  Go-side and Python-side counters are distinct families per
  NFR-DEEP1-003 (A) vs (B). The exactly-once guarantee on the
  Go-side counter applies only to non-disconnect terminal states
  (success, deadline_exceeded, budget_exceeded, error_invalid,
  error_upstream, error_unresolved_citations_threshold) — not to
  client-disconnect mid-stream.
- This wording follows the SPEC-SYN-004 NFR-SYN4-003 precedent
  verbatim.
- Goroutine-leak detector PASS (all 3 goroutines released within
  100 ms of ctx cancel) per REQ-DEEP1-004b.

### Edge Case 11 — Sidecar returns malformed JSON

**Given**
- Sidecar returns HTTP 200 with body `"not json"` (defective
  upstream)

**Then**
- Go-side client wraps the JSON parse error as
  `fmt.Errorf("deepreport: parse response: %w", err)`.
- Counter `usearch_deep_outcomes_total{outcome="error_upstream"}`
  += 1.
- Returned error is NOT one of the named sentinels (it is a
  wrapped JSON error); caller distinguishes via
  `errors.Is(err, deepreport.ErrInvalidRequest) == false &&
   errors.Is(err, deepreport.ErrSidecarUnreachable) == false`.

### Edge Case 12 — Per-call override caps below env-var ceilings

**Given**
- `STORM_MAX_LATENCY_MS=300000` (5 min env-var ceiling)
- Request body specifies `"max_latency_ms": 60000` (1 min
  per-call cap; below ceiling)

**Then**
- Sidecar honors the per-call cap (60 s).
- Pipeline cancels at 60 s if not complete; HTTP 504.
- This validates the per-call override mechanism that
  SPEC-DEEP-004 will use for per-user quota enforcement.

### Edge Case 13 — Per-call override exceeds env-var ceiling

**Given**
- `STORM_MAX_LATENCY_MS=300000`
- Request body specifies `"max_latency_ms": 600000` (10 min;
  above ceiling)

**Then**
- Sidecar SHALL clamp the effective cap to the env-var ceiling
  (300 000 ms / 5 min); per-call override CANNOT exceed env-var.
- Behavior: pipeline cancels at 300 s if not complete; HTTP 504.
- WARN-level log record `{request_id, reason:
  "per_call_override_clamped", requested_max_latency_ms: 600000,
  effective_max_latency_ms: 300000}`.

### Edge Case 14 — STORM upstream library raises unexpected exception

**Given**
- Mocked `STORMWikiRunner.run()` raises `RuntimeError("upstream
  bug")`

**Then**
- Pipeline catches the exception, logs at ERROR level, returns
  HTTP 503 with body
  `{"error": "upstream_failure",
    "detail": "STORM pipeline raised: RuntimeError: upstream bug",
    "request_id": "..."}`.
- Counter `usearch_deep_outcomes_total{outcome="error_upstream"}`
  += 1.
- Go-side client wraps as
  `fmt.Errorf("deepreport: upstream: %w", err)`.

### Edge Case 15 — Single document corpus (degenerate retrieval)

**Given**
- Request with `docs == [single_doc]`

**Then**
- The injected RM returns the single doc for any sub-query
  (top-k clamped to 1).
- STORM pipeline runs but with limited grounding; expected output
  has all citations pointing to the single doc.
- Faithfulness gate accepts (all sentences cite [1]).
- Test asserts response well-formed; not a target operational
  case but should not crash.

---

## 4. Quality Gate Criteria

### 4.1 Code Quality

- [ ] Python: `ruff check services/storm/` exits 0
- [ ] Python: `ruff format --check services/storm/` exits 0
- [ ] Python: `mypy --strict services/storm/src/` exits 0 (run
      phase decision: opt-in if mypy already used elsewhere; else
      defer to a follow-up)
- [ ] Go: `gofmt -d internal/deepreport/ internal/streamsynth/longform.go
      internal/obs/metrics/deepreport.go` empty diff
- [ ] Go: `golangci-lint run ./internal/deepreport/...
      ./internal/streamsynth/... ./internal/obs/metrics/...` exits 0
- [ ] No new `# type: ignore` or `// nolint` directives without
      `@MX:WARN: [AUTO] @MX:REASON: ...` justification

### 4.2 Test Coverage

- [ ] `pytest --cov=storm --cov-report=term-missing
      services/storm/tests/` reports ≥85% on every new module
- [ ] `go test -coverprofile=cover.out ./internal/deepreport/...
      ./internal/streamsynth/... ./internal/obs/metrics/...`
      reports ≥85%
- [ ] All 53 RED-phase tests (per plan.md §7) green
- [ ] Property tests via `hypothesis>=6` and `testing/quick` green
      at default `max_examples` (100 examples)
- [ ] Race tests (`go test -race`) green for all NFR-DEEP1-003
      assertions
- [ ] **REQ-DEEP1-004a (Python sidecar runtime cleanup)** —
      `services/storm/tests/test_caps.py::test_no_resource_leak_on_cancel_python`
      asserts (a) asyncio task cancelled cleanly via
      `asyncio.wait_for` propagation on both deadline and budget
      paths; (b) httpx `AsyncClient` connection pool returns all
      in-flight connections (verified by `lsof` socket count
      returning to baseline within 100 ms); (c)
      `threading.enumerate()` count returns to baseline within
      100 ms of cancellation
- [ ] **REQ-DEEP1-004b (Go client runtime cleanup)** —
      `internal/deepreport/client_test.go::TestNoGoroutineLeakOnCancel`
      uses `go.uber.org/goleak` `goleak.VerifyNone(t)` to assert
      zero leaked goroutines after `ctx.Done()`; the HTTP response
      body is drained and closed via `defer resp.Body.Close()` on
      both 504, 402, and ctx-cancel paths (verified by descriptor
      count assertion)

### 4.3 Backward Compatibility

- [ ] SPEC-SYN-001 acceptance suite (existing `test_app.py`,
      `test_synthesis.py`, `test_gateway.py`, `test_obs.py`,
      `internal/synthesis/client_test.go`) remains 100% green
- [ ] SPEC-SYN-002 acceptance suite (existing
      `test_faithfulness.py`) remains 100% green
- [ ] SPEC-SYN-004 acceptance suite (existing
      `internal/sse/`, `internal/streamsynth/`) remains 100% green
- [ ] Single-paragraph `/synthesize` endpoint behavior byte-identical
      pre/post-DEEP-001 (regression test on SPEC-SYN-001 acceptance)
- [ ] `internal/streamsynth/streamsynth.go` (single-paragraph
      stream) unchanged; `longform.go` is a sibling file

### 4.4 Performance

- [ ] `test_long_form_latency_p50_under_180s` (mocked-LM 50
      iterations): assert p50 ≤ 180 s
- [ ] `test_long_form_latency_p95_under_300s` (mocked-LM 50
      iterations): assert p95 ≤ 300 s
- [ ] `test_ttfb_section_start_within_5s_of_done`:
      buffered-then-streamed TTFB ≥ end-to-end latency − 5 s
- [ ] Heartbeat goroutine CPU overhead under 1% (inherited
      SYN-004 NFR; re-verified for long-form path)
- [ ] No memory leak under 100-iteration soak test (Python
      sidecar RSS stable after warmup)

### 4.5 Observability

- [ ] Two new Prometheus collectors registered and scraped via
      `cmd/usearch-api /metrics`:
  - `usearch_deep_outcomes_total{outcome="..."}` (6 enum values:
    success, deadline_exceeded, budget_exceeded, error_invalid,
    error_upstream, error_unresolved_citations_threshold —
    last is reserved per plan.md §10 D4)
  - `usearch_deep_latency_seconds` (histogram, 8 buckets)
- [ ] Two Python-emitted counters scraped from sidecar
      `/metrics`:
  - `usearch_storm_faithfulness_outcomes_total{outcome="..."}` (4
    enum values: accepted, stripped, rejected, off — per plan.md
    §10 D1)
  - `usearch_storm_unresolved_citations_total` (no labels)
- [ ] JSON log records carry the four new attributes
      (`outcome`, `sections_count`, `sentences_count`,
      `citations_count`) on every successful `/generate_report`
      invocation
- [ ] Cardinality guard (`internal/obs/metrics/metrics_test.go:248
      TestCardinalityGuardRejectsUnboundedLabels`, alias
      `TestNoUnboundedLabels` line 284) remains green without
      modification — the `outcome` label NAME is pre-existing in
      the allowlist (line 257); DEEP-001 only adds 6 new VALUES on
      the existing allowlisted NAME, pre-initialised per the
      SYN-004 `streamsynth.go:48-56` pattern
- [ ] OTel spans named `deep.generate_report` (Go-side) and
      `storm.run` (Python-side) created and ended within their
      respective handlers; attributes mirror the slog records
- [ ] NFR-DEEP1-003 exactly-once outcome counter invariant
      verified by all 4 race-window tests
      (`test_outcome_counter_race_*`)

### 4.6 Security

- [ ] No PII or API keys in log records (parallel to SYN-001
      `test_python_log_redacts_api_key` regression discipline)
- [ ] `LITELLM_MASTER_KEY` never appears in any log, span
      attribute, or error message (parallel to SPEC-LLM-001
      REQ-LLM-005)
- [ ] HTTP 422 / 504 / 402 / 503 response bodies contain no
      `text` / `sections` content (no leakage of un-validated LLM
      output to client)
- [ ] LLM input still passes through Pydantic validation first;
      faithfulness gate is the second guard, not a replacement
- [ ] STORM's upstream library cannot make external network calls
      via its default RMs (we inject our own RM); verified via
      `test_inject_rm_no_external_http_calls`

### 4.7 Documentation

- [ ] `services/storm/README.md` updated with operator quickstart,
      env vars table, sample curl for SSE + JSON paths, expected
      latency / cost ranges, troubleshooting
- [ ] `services/storm/.env.example` lists all `STORM_*` env vars
      with explanatory comments
- [ ] `deploy/docker-compose.yml` `storm` service entry includes
      port mapping, depends_on, env_file
- [ ] @MX tags applied per plan.md §3.7
- [ ] CHANGELOG entry referencing SPEC-DEEP-001 added

---

## 5. Out-of-Scope Verification

The following SHALL be verified as NOT implemented in this SPEC
(scope discipline):

- [ ] No multi-agent Researcher/Reviewer/Writer/Verifier pipeline
      (defer SPEC-DEEP-002)
- [ ] No tree exploration with breadth/depth knobs (defer
      SPEC-DEEP-003)
- [ ] No per-user / per-day quota enforcement (defer SPEC-DEEP-004)
- [ ] No RAGAS-style semantic faithfulness scoring (defer
      SPEC-EVAL-001)
- [ ] No char-span citations (no Anthropic-API-style char offsets)
- [ ] No retrieval / fanout / adapter changes (DEEP-001 consumes
      pre-assembled `docs[]`)
- [ ] No multi-claim-per-sentence detection
- [ ] No `SynthesizeResponse` schema breakage (single-paragraph
      `/synthesize` unchanged)
- [ ] No token-level streaming from STORM internals
      (buffered-then-streamed v0)
- [ ] No `Last-Event-ID` SSE resume support
- [ ] No WebSocket / gRPC / NDJSON transport
- [ ] No `usearch deep "..."` CLI surface in `cmd/usearch` (defer
      SPEC-CLI-002)
- [ ] No MCP tool surface (defer SPEC-MCP-001)
- [ ] No GitHub issue tracking (`issue_number: 0`)
- [ ] No direct vendor LLM SDKs (LiteLLM proxy is the only path)

---

*End of SPEC-DEEP-001 acceptance v0.1*
