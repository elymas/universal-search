# SPEC-SYN-002 Acceptance Criteria

Companion artifact for `.moai/specs/SPEC-SYN-002/spec.md`.
Version: 0.1.0 (draft)
Created: 2026-05-09
Author: limbowl (via manager-spec)

---

## 1. Definition of Done

SPEC-SYN-002 is **DONE** when ALL of the following hold:

- [ ] `services/researcher/src/researcher/faithfulness.py` exists and
      exports the four documented functions.
- [ ] `services/researcher/tests/test_faithfulness.py` passes with
      ≥85% coverage on the new module.
- [ ] All five EARS REQs (REQ-SYN2-001 through REQ-SYN2-005) have
      corresponding green tests.
- [ ] Both NFRs (NFR-SYN2-001 latency, NFR-SYN2-002 idempotence) are
      verified by automated tests.
- [ ] SPEC-SYN-001 acceptance suite remains 100% green (regression
      check) — `pytest -q services/researcher/tests/`.
- [ ] Go-side `internal/obs/metrics/synthesis.go` registers two new
      collectors; cardinality allowlist unchanged.
- [ ] `RESEARCHER_FAITHFULNESS_MODE` env var documented in
      `.env.example`.
- [ ] Conventional commit `feat(synthesis): SPEC-SYN-002 citation
      faithfulness enforcement` references this SPEC ID.
- [ ] @MX tags applied per `plan.md` §3.6.
- [ ] TRUST 5 gates passed (tested, readable, unified, secured,
      trackable).
- [ ] Pre-submission self-review per `workflow-modes.md` confirms no
      simpler approach achieves the same result.

---

## 2. Acceptance Scenarios (Given-When-Then)

### Scenario 1 — Happy path: clean output passes through

**Given**
- `RESEARCHER_FAITHFULNESS_MODE=strip` (default)
- A `/synthesize` request with `query="What is GPT-4?"`,
  `lang="en"`, and 3 valid `docs[]`
- The LLM (mocked) returns: `"GPT-4 is a multimodal model from OpenAI [1]. It was released in 2023 [2]. It supports image input [3]."`

**When**
- The Python sidecar processes the request

**Then**
- HTTP status 200
- `response.text == "GPT-4 is a multimodal model from OpenAI [1]. It was released in 2023 [2]. It supports image input [3]."`
- `response.citations` length == 3 with `marker ∈ {1, 2, 3}` and
  `doc_id` values matching `docs[0..2].id`
- `response.notice == ""` (no stripping occurred)
- `gateway.complete()` invoked exactly **once** (no retry triggered)
- Counter `usearch_synthesis_faithfulness_outcomes_total{outcome="accepted"}` += 1
- Counter `usearch_synthesis_faithfulness_retries_total` unchanged
- JSON log record contains
  `{faithfulness_action: "accepted", uncited_sentences_count: 0,
    retry_attempted: false}`

### Scenario 2 — Strip mode: un-cited sentence removed

**Given**
- `RESEARCHER_FAITHFULNESS_MODE=strip`
- A `/synthesize` request with 2 docs
- The LLM (mocked) returns:
  `"GPT-4 was released in 2023 [1]. It is widely used. Anthropic released Claude later [2]."`
  (sentence 2 has no marker)
- The retry-pass LLM (mocked) ALSO returns un-cited content (worst case)

**When**
- The Python sidecar processes the request

**Then**
- HTTP status 200
- After retry: `response.text == "GPT-4 was released in 2023 [1]. Anthropic released Claude later [2]."`
- `response.citations` contains markers `[1]` and `[2]`
- `response.notice == "1 uncited sentence(s) stripped"`
- `gateway.complete()` invoked exactly **twice** (initial + retry)
- Counter `usearch_synthesis_faithfulness_retries_total` += 1
- Counter `usearch_synthesis_faithfulness_outcomes_total{outcome="retry_failed"}` += 1
- JSON log record contains
  `{faithfulness_action: "stripped", uncited_sentences_count: 1,
    retry_attempted: true}`
- `cost_usd` reflects the SUM of both LLM calls (REQ-SYN-006
  "exactly once per top-level invocation" preserved at the
  per-request level)

### Scenario 3 — Reject mode: 422 after retry failure

**Given**
- `RESEARCHER_FAITHFULNESS_MODE=reject`
- A `/synthesize` request with 2 docs
- The LLM (mocked) returns un-cited content on both first and retry
  passes

**When**
- The Python sidecar processes the request

**Then**
- HTTP status **422**
- Response body: `{"error": "un_cited_output", "detail": "1 sentence(s) without citations after retry", "uncited_count": 1}`
- Response body does NOT contain a `text` field
- `gateway.complete()` invoked exactly **twice**
- Counter `usearch_synthesis_faithfulness_outcomes_total{outcome="rejected"}` += 1
- Counter `usearch_synthesis_faithfulness_retries_total` += 1
- Exactly 1 WARN-level log record with attributes
  `{request_id, faithfulness_action: "rejected",
    uncited_sentences_count: 1, retry_attempted: true}`
- Go-side `internal/synthesis.Client.Synthesize` returns
  `synthesis.ErrInvalidRequest` (because 422 maps to that sentinel
  per SPEC-SYN-001 client.go:150-152)

### Scenario 4 — Retry succeeds: stricter prompt produces clean output

**Given**
- `RESEARCHER_FAITHFULNESS_MODE=strip` or `reject`
- A `/synthesize` request with 2 docs
- The first-pass LLM (mocked) returns:
  `"GPT-4 was released in 2023 [1]. Claude is also popular."`
  (sentence 2 un-cited)
- The retry-pass LLM (mocked, sees stricter system prompt) returns:
  `"GPT-4 was released in 2023 [1]. Claude is also popular [2]."`

**When**
- The Python sidecar processes the request

**Then**
- HTTP status 200 (regardless of mode, because retry succeeded)
- `response.text == "GPT-4 was released in 2023 [1]. Claude is also popular [2]."`
- `response.citations` length == 2
- `response.notice == ""` (no stripping needed post-retry)
- `gateway.complete()` invoked exactly **twice**
- The second call's `messages[0]["content"]` (system message)
  contains substring `"Every sentence MUST end with at least one
  citation marker"`
- Counter `usearch_synthesis_faithfulness_retries_total` += 1
- Counter `usearch_synthesis_faithfulness_outcomes_total{outcome="retry_succeeded"}` += 1
- JSON log record:
  `{faithfulness_action: "accepted", uncited_sentences_count: 0,
    retry_attempted: true}`

### Scenario 5 — Mode=off backward-compatibility bypass

**Given**
- `RESEARCHER_FAITHFULNESS_MODE=off`
- A `/synthesize` request with 2 docs
- The LLM (mocked) returns un-cited content:
  `"Sentence A. Sentence B [1]."`

**When**
- The Python sidecar processes the request

**Then**
- HTTP status 200
- `response.text == "Sentence A. Sentence B [1]."` (verbatim)
- `response.citations` length == 1 with `marker=1`
- `response.notice == ""` (SPEC-SYN-001 behavior preserved)
- `gateway.complete()` invoked exactly **once**
- `enforce_faithfulness()` is NOT invoked (verifiable via
  `unittest.mock.patch`)
- All `usearch_synthesis_faithfulness_*` counters unchanged
- JSON log record:
  `{faithfulness_action: "off", uncited_sentences_count: 0,
    retry_attempted: false}` (per plan.md §10 D3 / REQ-SYN2-003;
  attribute MUST be present with value `"off"`)

### Scenario 6 — Korean prose with mixed punctuation

**Given**
- `RESEARCHER_FAITHFULNESS_MODE=strip`
- A `/synthesize` request with `lang="ko"` and 2 docs
- The LLM returns Korean prose:
  `"GPT-4는 2023년에 출시되었습니다 [1]. 클로드는 그 이후에 출시되었습니다."`
  (sentence 2 un-cited)
- Retry pass returns: `"GPT-4는 2023년에 출시되었습니다 [1]. 클로드는 그 이후에 출시되었습니다 [2]."`

**When**
- Sentence segmentation regex `[.!?。！？]\s+|[.!?。！？]$` is applied

**Then**
- Sentence boundary detected at the period after `습니다` (English-style
  period, not full-width)
- Both sentences correctly identified
- Retry path triggers; retry succeeds
- HTTP 200; `response.text` matches retry output verbatim
- `response.citations` has 2 entries

---

## 3. Edge Cases

### Edge Case 1 — All sentences un-cited, mode=strip → empty output → 422

**Given**
- `RESEARCHER_FAITHFULNESS_MODE=strip`
- LLM returns `"Sentence A. Sentence B. Sentence C."` (zero markers)
- Retry returns same un-cited output

**Then**
- After retry, all 3 sentences would be stripped → `gated_text == ""`.
- Per plan.md §10 D1 (resolves §9 Q3): the service SHALL raise
  `UncitedOutputError` and return HTTP 422 with body
  `{"error": "un_cited_output", "detail": "3 sentence(s) without
  citations after retry", "uncited_count": 3}` — identical to the
  REQ-SYN2-004 reject-mode contract. **No degraded fallback.**
- HTTP status **422** (regardless of configured mode being `strip`)
- Counter `usearch_synthesis_faithfulness_outcomes_total{outcome="rejected"}` += 1
- Counter `usearch_synthesis_faithfulness_retries_total` += 1
- WARN-level log record with attributes
  `{request_id, faithfulness_action: "rejected",
    uncited_sentences_count: 3, retry_attempted: true}`
- Counter `usearch_synthesis_calls_total{outcome="degraded"}` is NOT
  incremented (this is a faithfulness-quality failure, not a
  connection / degraded-mode failure).

### Edge Case 2 — Single-sentence output, fully cited

**Given**
- LLM returns `"GPT-4 is a multimodal model [1]."` (1 sentence,
  cited)

**Then**
- `enforce_faithfulness()` returns `(ACCEPTED, text, 0)`.
- No retry, no strip.
- Counter `usearch_synthesis_faithfulness_outcomes_total{outcome="accepted"}` += 1.

### Edge Case 3 — Single-sentence output, un-cited, mode=reject

**Given**
- LLM returns `"GPT-4 is a multimodal model."` (1 sentence,
  un-cited) on both passes
- `RESEARCHER_FAITHFULNESS_MODE=reject`

**Then**
- HTTP 422 with body `{"error": "un_cited_output", "detail": "1
  sentence(s) without citations after retry", "uncited_count": 1}`.

### Edge Case 4 — Marker outside doc range survives `_process_markers`?

This SHOULD NEVER HAPPEN — `_process_markers` (SPEC-SYN-001) strips
out-of-range markers before `enforce_faithfulness` is invoked. We
verify this invariant via:

**Given**
- LLM returns `"Foo [99] bar [1]."` with 2 docs

**Then**
- After `_process_markers`: text == `"Foo bar [1]."` (marker `[99]` stripped)
- After `enforce_faithfulness`: depending on segmentation, either
  the entire text is one sentence (cited by `[1]` at the end) → ACCEPTED
- No double-counting of stripped markers.

### Edge Case 5 — Multi-marker sentence

**Given**
- LLM returns `"GPT-4 and Claude were both released in 2023 [1] [2]."`
  (one sentence with two markers)

**Then**
- `find_uncited_sentences()` returns empty list (sentence has at
  least one marker — in fact two).
- ACCEPTED.
- This is structural-only check; SPEC-SYN-002 does NOT verify whether
  one sentence with two markers is semantically supported by both.
  That gap belongs to SPEC-EVAL-001.

### Edge Case 6 — Sentence ending without trailing space (end-of-text)

**Given**
- LLM returns `"GPT-4 is a multimodal model [1]"` (no trailing
  whitespace, no terminal period — adversarial LLM output)

**Then**
- Regex `[.!?。！？]\s+|[.!?。！？]$` does NOT match (no terminal
  punctuation at all).
- `split_sentences` returns the full text as one "sentence".
- This single segment contains `[1]` → ACCEPTED.
- Acceptable behavior; well-formed prose typically ends with
  punctuation. Document as known minor edge.

### Edge Case 7 — Empty post-`_process_markers` text

**Given**
- `_process_markers` somehow produced an empty string (all markers
  out-of-range and all text was just markers)
- Example LLM output: `"[5] [6] [7]"` with 2 docs → all stripped →
  `text == ""`

**Then**
- `enforce_faithfulness("", docs, mode, retry_attempted)` returns
  `(REJECTED, "", 0)` regardless of configured mode (empty input
  text has no citable sentence).
- Caller path: same as Edge Case 1 — `synthesize()` raises
  `UncitedOutputError(uncited_count=0)` and the service responds
  with HTTP 422 (per plan.md §10 D1).
- Counter `usearch_synthesis_faithfulness_outcomes_total{outcome="rejected"}` += 1.
- No degraded fallback is emitted on this path.

### Edge Case 8 — Mode env var typo

**Given**
- `RESEARCHER_FAITHFULNESS_MODE=stript` (typo)

**Then**
- The Python sidecar SHALL log a WARN-level message
  `"Unknown RESEARCHER_FAITHFULNESS_MODE='stript'; defaulting to 'strip'"`
  and proceed with `mode=strip`.
- Documented in run phase as defensive default; preferred over
  raising on startup which would block the service.

### Edge Case 9 — Concurrent requests with different modes

**Given**
- Two concurrent `/synthesize` requests
- Request A reads `RESEARCHER_FAITHFULNESS_MODE=strip` at request
  start
- Mid-request, env var is mutated to `reject` (operator change)
- Request B reads `reject`

**Then**
- Each request uses the value at its own start; no shared mutable
  state. Per-request `mode` is a local variable.
- Behavior deterministic per-request.

### Edge Case 10 — Retry path encounters LiteLLM ConnectError

**Given**
- First-pass LLM call succeeds with un-cited output
- Retry-pass LLM call raises `httpx.ConnectError` (LiteLLM became
  unreachable mid-request)

**Then**
- The retry catch block (existing in SPEC-SYN-001 synthesis.py
  lines 174–190) triggers degraded-mode fallback.
- `degraded=true`, bullet-list response.
- Counter `usearch_synthesis_calls_total{outcome="degraded"}` += 1.
- Counter `usearch_synthesis_faithfulness_outcomes_total` does NOT
  increment (faithfulness gate did not complete; the request is
  degraded, not stripped/rejected).
- This preserves SPEC-SYN-001 NFR-SYN-003 graceful degradation.

---

## 4. Quality Gate Criteria

### 4.1 Code Quality

- [ ] Python: `ruff check services/researcher/` exits 0
- [ ] Python: `ruff format --check services/researcher/` exits 0
- [ ] Go: `gofmt -d internal/obs/metrics/synthesis.go` empty diff
- [ ] Go: `golangci-lint run ./internal/obs/metrics/...` exits 0
- [ ] No new `# type: ignore` or `// nolint` directives without
      `@MX:WARN: [AUTO] @MX:REASON: ...` justification

### 4.2 Test Coverage

- [ ] `pytest --cov=researcher --cov-report=term-missing
      services/researcher/tests/` reports ≥85% on
      `faithfulness.py` and ≥85% on `synthesis.py`
- [ ] `go test -coverprofile=cover.out ./internal/obs/metrics/...`
      reports ≥85%
- [ ] All 20 RED-phase tests (per plan.md §7) green

### 4.3 Backward Compatibility

- [ ] SPEC-SYN-001 acceptance test suite (existing
      `test_app.py`, `test_synthesis.py`, `test_gateway.py`,
      `test_obs.py`, `internal/synthesis/client_test.go`) remains
      100% green
- [ ] `SynthesizeResponse` JSON schema unchanged (no fields added
      or removed; only `notice` and `text` *content* may differ)
- [ ] Go-side `Result` struct unchanged

### 4.4 Performance

- [ ] `test_faithfulness_gate_latency` (50 iterations, 12-sentence
      input): p99 ≤ 50 ms
- [ ] `test_synthesize_p95_with_retry_under_limit` (50 calls
      forcing retry): p95 ≤ 14.0 s
- [ ] `test_no_retry_path_perf`: end-to-end overhead vs.
      SPEC-SYN-001 baseline ≤ 100 ms

### 4.5 Observability

- [ ] Two new Prometheus collectors registered and scraped via
      `/metrics`:
  - `usearch_synthesis_faithfulness_outcomes_total{outcome="..."}`
    (5 enum values: accepted, stripped, rejected, retry_succeeded,
    retry_failed; mode=off bypasses the counter entirely per
    REQ-SYN2-003 — see plan.md §10 D2)
  - `usearch_synthesis_faithfulness_retries_total` (no labels)
- [ ] JSON log records carry the three new attributes
      (`uncited_sentences_count`, `faithfulness_action`,
      `retry_attempted`) on every `/synthesize` invocation
- [ ] Cardinality allowlist (per SPEC-OBS-001) unchanged

### 4.6 Security

- [ ] No PII or API keys in log records (per SPEC-SYN-001
      `test_python_log_redacts_api_key` regression)
- [ ] LLM input still passes through `_process_markers` first;
      faithfulness gate is the second guard, not a replacement
- [ ] 422 reject-mode response body contains no `text` content
      (no leakage of un-validated LLM output to client)

### 4.7 Documentation

- [ ] `.env.example` updated with `RESEARCHER_FAITHFULNESS_MODE`
- [ ] `services/researcher/README.md` (if exists) section updated
- [ ] Sample curl snippets for each mode in run-phase commit
      message or PR description

---

## 5. Out-of-Scope Verification

The following SHALL be verified as NOT implemented in this SPEC
(scope discipline):

- [ ] No RAGAS-style semantic faithfulness scoring (defer SPEC-EVAL-001)
- [ ] No char-span citations (no Anthropic-API-style char offsets)
- [ ] No retrieval / fanout / adapter changes
- [ ] No multi-claim-per-sentence detection
- [ ] No `SynthesizeResponse` schema breakage
- [ ] No Go-side outcome enum extension on
      `usearch_synthesis_calls_total` (Python-side only adds the new
      faithfulness counters)
- [ ] No more than one retry per request (FROZEN at `max_retries=1`)
- [ ] No streaming-aware faithfulness (defer SPEC-SYN-004)
- [ ] No GitHub issue tracking (`issue_number: 0`)

---

*End of SPEC-SYN-002 acceptance v0.1*
