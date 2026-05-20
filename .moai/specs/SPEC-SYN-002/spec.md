---
id: SPEC-SYN-002
version: 0.1.0
status: implemented
created: 2026-05-09
updated: 2026-05-09
implemented: 2026-05-09
author: limbowl
priority: P0
issue_number: 0
title: Citation faithfulness enforcement
milestone: M4 — Basic Synthesis Hardening
owner: expert-backend
methodology: tdd
coverage_target: 85
depends_on: [SPEC-SYN-001, SPEC-CORE-001, SPEC-LLM-001]
blocks: [SPEC-EVAL-001]
---

# SPEC-SYN-002: Citation faithfulness enforcement

## HISTORY

- 2026-05-09 — status draft → approved (plan-auditor PASS iter 2, 2 MINOR fixes applied)
- 2026-05-09 (initial draft v0.1, limbowl via manager-spec):
  First EARS-formatted SPEC for citation faithfulness enforcement.
  Modifies SPEC-SYN-001's Python sidecar synthesis pipeline to add
  per-sentence `doc_id` provenance enforcement at the
  `_process_markers` chokepoint. Adds a NEW
  `services/researcher/src/researcher/faithfulness.py` module wired
  between LLM completion and response assembly. Reuses existing
  `Citation.doc_id` schema (no API breakage). Behaviour gates via
  `RESEARCHER_FAITHFULNESS_MODE ∈ {strip, reject, off}` env var,
  default `strip` (best UX). Single-retry policy bounded to preserve
  NFR-SYN-001 p50 ≤ 8s. Companion research artifact at
  `.moai/specs/SPEC-SYN-002/research.md` — 47 internal file:line
  references + 5 external sources (gpt-researcher, Anthropic
  citations, RAGAS, LangChain, pysbd). 5 EARS REQs (4 × P0 + 1 × P1)
  covering all five EARS patterns, 2 NFRs. Explicitly delegates
  semantic faithfulness scoring to SPEC-EVAL-001 (M4, RAGAS /
  DeepEval) and SPEC-SYN-003 (chunking). Coordinates with
  SPEC-SYN-001 (preserves NFR-SYN-002 invariants), SPEC-OBS-001
  (extends `usearch_synthesis_*` metric family with two new
  faithfulness counters; cardinality allowlist unchanged). No GitHub
  issue tracking on this SPEC (`issue_number: 0`). Ready for
  plan-auditor review and annotation cycle.

---

## 1. Purpose

`.moai/project/roadmap.md` line 64 declares M4 SPEC-SYN-002:

> Citation faithfulness | enforce `doc_id` trace on every synthesized
> claim, reject un-cited LLM output | expert-backend

SPEC-SYN-001 (implemented at commit `7fc338d`) delivered the
**structural** marker→doc mapping in
`services/researcher/src/researcher/synthesis.py:_process_markers`:
every `[N]` resolves to a real input doc, and out-of-range markers
are stripped. This is necessary but not sufficient. A four-sentence
paragraph that places a single `[1]` at the end satisfies SPEC-SYN-001
but leaves three sentences un-attributed.

SPEC-SYN-002 promotes the contract from structural to **behavioral**:

- Every **claim** (sentence) in the synthesized output SHALL carry at
  least one valid `[N]` marker that resolves to an input doc's `id`.
- LLM output failing this gate SHALL trigger one re-prompt with a
  stricter system message, OR (configurable) be cleaned by stripping
  un-cited sentences, OR (configurable) be rejected outright with
  HTTP 422.
- The Go-side `internal/synthesis.Client` and Python `/synthesize`
  endpoint contracts SHALL be preserved (no schema changes); only the
  *content* of `Result.Text` and `Result.Citations` changes.

This SPEC is **structural faithfulness only**. Whether the cited doc
actually *supports* the claim (semantic faithfulness) is delegated to
SPEC-EVAL-001 (M4, RAGAS / DeepEval scorer at ≥0.85 per
`roadmap.md` line 151).

Completion delivers an inline-enforced citation-density gate that
reduces the visible hallucination surface for the M4 user-visible
synthesis quality bar.

---

## 2. Scope

### 2.1 In-Scope

| Ref | Item |
|-----|------|
| a | [NEW] `services/researcher/src/researcher/faithfulness.py` — pure-Python module exposing `split_sentences(text)`, `find_uncited_sentences(text)`, `enforce_faithfulness(text, docs, mode, retry_state)`, `EnforcementOutcome` enum (`accepted`, `retry_required`, `stripped`, `rejected`). No LLM calls; pure regex + string ops. |
| b | [MODIFY] `services/researcher/src/researcher/synthesis.py:synthesize()` — invoke `enforce_faithfulness()` between `_process_markers()` (line 192) and the final `SynthesizeResponse(...)` return (lines 208–220). On `retry_required`, re-call `gateway.complete()` once with a stricter system prompt, then re-run `_process_markers` + `enforce_faithfulness` over the second response. |
| c | [MODIFY] `services/researcher/src/researcher/synthesis.py:build_prompt()` — extend the system message with: "Every sentence MUST end with at least one citation marker [N] that references a source from the SOURCES list. Sentences without citation markers will be rejected." |
| d | [NEW] `RESEARCHER_FAITHFULNESS_MODE` env var: `strip` (default), `reject`, `off`. Loaded by `synthesize()` once per request. Documented in `.env.example`. |
| e | [MODIFY] `services/researcher/src/researcher/obs.py` — extend the `log_synthesis()` JSON record with three additional attributes: `uncited_sentences_count: int`, `faithfulness_action: str ∈ {accepted, stripped, rejected, off}` (4 values — the final action taken on the response; retry status is captured separately via `retry_attempted: bool`), `retry_attempted: bool`. |
| f | [NEW] Two Prometheus collectors in `internal/obs/metrics/synthesis.go`: `SynthesisFaithfulnessOutcomes *prometheus.CounterVec{outcome}` (label values: `accepted`, `stripped`, `rejected`, `retry_succeeded`, `retry_failed`) and `SynthesisFaithfulnessRetries prometheus.Counter` (no labels — total retry count). Cardinality allowlist unchanged: `outcome` label is already declared. |
| g | [MODIFY] `internal/obs/metrics/metrics.go` — register the two new collectors via the existing `registerSynthesis(pr)` helper added by SPEC-SYN-001. |
| h | [MODIFY] `internal/obs/obs.go` — re-export the two new collector handles for caller convenience (`obs.SynthesisFaithfulnessOutcomes`, `obs.SynthesisFaithfulnessRetries`). |
| i | [EXISTING — UNCHANGED] `services/researcher/src/researcher/models.py` `Citation` schema. `doc_id` field already exists (line 61). No API breakage. |
| j | [EXISTING — UNCHANGED] `internal/synthesis/types.go` `Result` and `Citation` Go shapes. No JSON schema change. |
| k | [EXISTING — UNCHANGED] `internal/synthesis/client.go:Synthesize` outcome enum. Faithfulness outcomes are recorded only on the Python side; Go-side `outcome` remains `success` (cited or stripped) or `error_*` (server 4xx/5xx). |
| l | [NEW] `services/researcher/tests/test_faithfulness.py` — unit tests for `split_sentences`, `find_uncited_sentences`, `enforce_faithfulness` in all three modes, plus property tests via `hypothesis`. |
| m | [MODIFY] `services/researcher/tests/test_synthesis.py` — add integration tests covering the post-`_process_markers` faithfulness gate (mode=strip, mode=reject, mode=off) and the single-retry path. |
| n | [MODIFY] `services/researcher/tests/test_app.py` — add HTTP-level tests for `mode=reject` returning HTTP 422 with body `{"error":"un_cited_output","detail":<n> sentences without citations}` and for `mode=strip` returning 200 with stripped text. |

### 2.2 Out-of-Scope (Exclusions — What NOT to Build)

[HARD] This SPEC explicitly excludes the following items. Each has a
known destination SPEC; this list prevents scope creep.

- **Semantic faithfulness scoring** — measuring whether the cited
  doc *supports* the claim (i.e. RAGAS-style faithfulness ratio).
  → SPEC-EVAL-001 (M4, DeepEval CI gate at ≥0.85).
- **Modifying retrieval, fanout, or adapter behavior** — synthesis
  consumes whatever `[]NormalizedDoc` is passed in. → SPEC-FAN-001
  (implemented), SPEC-ADP-* (per adapter).
- **Char-span citation tracking** (Anthropic-citations-API style with
  `start_char_index`/`end_char_index`). → Future SPEC if measured value;
  requires structured-output API. SPEC-SYN-001 §2.2 already excludes
  tool-use API for citation extraction.
- **Multi-claim-per-sentence detection** (one `[N]` covering two
  factual statements). → SPEC-EVAL-001 (semantic territory).
- **Hallucinated content under valid `[N]`** (the model invents a
  fact and cites a real doc that doesn't support it). → SPEC-EVAL-001.
- **Sentence segmentation library upgrade** (pysbd, spaCy). v0 uses a
  Unicode-aware regex covering English + Korean punctuation. Library
  upgrade is an M4-iteration fast-follow if empirics warrant.
- **More than one retry per request.** Single retry is FROZEN to
  preserve NFR-SYN-001 p50 ≤ 8s. Re-prompting strategies beyond a
  single stricter pass require their own SPEC.
- **Streaming-aware faithfulness gate** — V1 operates on the
  fully-assembled paragraph. → SPEC-SYN-004 (M4) handles streaming;
  faithfulness in SSE chunks is its concern.
- **Korean-locale benchmark fixtures** — adversarial Korean prompts
  belong in SPEC-EVAL-003 (M8).
- **Schema breaking changes to `SynthesizeResponse`** — V1 reuses the
  existing shape. New observability fields go through logs and
  Prometheus, not the public response schema.
- **Cross-process cost accounting changes** — `cost_usd` semantics
  unchanged. The retry path adds a second LLM call when triggered;
  the cost is summed naturally by the existing
  `x-litellm-response-cost` accumulation in `gateway.py`.
- **Cardinality allowlist amendment for new label values** —
  `outcome` label reuses existing values plus `retry_succeeded` and
  `retry_failed`; we keep the same label NAME (no allowlist change
  required per SPEC-OBS-001 discipline).
- **GitHub Issue tracking on this SPEC** (`issue_number: 0`).

---

## 3. EARS Requirements

### Functional Requirements

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| REQ-SYN2-001 | Ubiquitous | The Python sidecar SHALL ensure that every claim (sentence) in `SynthesizeResponse.text` carries at least one valid `[N]` marker resolving to a `doc_id` in the input `docs[]`, where a sentence is segmented by the canonical regex pattern `[.!?。！？]\s+|[.!?。！？]$` (English + Korean punctuation, terminal-position alternative included). The structural NFR-SYN-002 invariant from SPEC-SYN-001 (every `[N]` maps to a real doc) SHALL be preserved. | P0 | `test_every_sentence_has_marker` — accepted output passes regex-based per-sentence check; SPEC-SYN-001 acceptance tests still green. |
| REQ-SYN2-002 | Event-Driven | WHEN the Python sidecar detects one or more sentences in the LLM output that lack any `[N]` marker AND `RESEARCHER_FAITHFULNESS_MODE != "off"`, THEN the service SHALL invoke `enforce_faithfulness()` which SHALL: (a) attempt exactly one re-prompt to `gateway.complete()` with the stricter system prompt directive ("Every sentence MUST end with at least one citation marker [N]..."); (b) on retry success (zero un-cited sentences), accept the second response; (c) on retry failure, fall back to the configured `RESEARCHER_FAITHFULNESS_MODE` action — `strip` removes un-cited sentences from text + collapses whitespace, `reject` raises a 422 HTTP response, `off` is unreachable here by precondition. The retry SHALL increment `usearch_synthesis_faithfulness_retries_total` exactly once and SHALL be visible in the JSON log record (`retry_attempted: true`). | P0 | `test_uncited_triggers_retry`; `test_retry_success_returns_clean_output`; `test_retry_failure_strip_mode`; `test_retry_failure_reject_mode_returns_422`. |
| REQ-SYN2-003 | State-Driven | WHILE `RESEARCHER_FAITHFULNESS_MODE == "off"`, the service SHALL bypass the entire faithfulness gate, SHALL NOT invoke `enforce_faithfulness()`, SHALL NOT trigger any retries, and SHALL return the SPEC-SYN-001 contract output verbatim. The `faithfulness_action` log attribute SHALL be `"off"` and the `usearch_synthesis_faithfulness_outcomes_total{outcome=...}` counter SHALL NOT increment. This mode SHALL exist for backward compatibility and emergency rollback. | P0 | `test_mode_off_bypasses_gate` — un-cited output passes through unchanged; counter stays at 0. |
| REQ-SYN2-004 | Unwanted | IF the Python sidecar detects un-cited sentences AFTER one retry attempt AND `RESEARCHER_FAITHFULNESS_MODE == "reject"`, THEN the service SHALL respond with HTTP 422, body `{"error":"un_cited_output","detail":"<N> sentence(s) without citations after retry","uncited_count":<N>}`, SHALL increment `usearch_synthesis_faithfulness_outcomes_total{outcome="rejected"}`, SHALL emit a WARN-level structured log record with `{request_id, faithfulness_action:"rejected", uncited_sentences_count:<N>, retry_attempted:true}`, and SHALL NOT return any `text` content to the client. | P0 | `test_reject_mode_returns_422_with_error_body`; `test_reject_mode_increments_counter`; `test_reject_mode_logs_warn`. |
| REQ-SYN2-005 | Optional | WHERE the `RESEARCHER_FAITHFULNESS_MODE` env var declares `strip` (default), the service SHALL produce a `SynthesizeResponse.text` containing only sentences that carry at least one valid `[N]` marker, SHALL collapse the resulting whitespace runs to single spaces, SHALL preserve sentence ordering, AND SHALL list in the response notice field `"<N> uncited sentence(s) stripped"` when `N > 0` else leave notice empty per SPEC-SYN-001 contract. | P1 | `test_strip_mode_removes_uncited_sentences`; `test_strip_mode_preserves_order`; `test_strip_mode_notice_set_when_stripped`. |

### Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| NFR-SYN2-001 | Performance (faithfulness gate latency) | The `enforce_faithfulness()` function (excluding the retry LLM call) SHALL complete within p99 ≤ 50 ms on a 12-sentence paragraph with 10 input docs. The single-retry path (when triggered) SHALL not violate SPEC-SYN-001 NFR-SYN-001 (p50 ≤ 8s end-to-end). When the retry path triggers, the service SHALL still target p95 ≤ 14s end-to-end (one LLM call ≈ 4s + one re-prompt ≈ 4s + faithfulness overhead < 100ms). Detailed test method (iteration counts, percentile assertions) is specified in `acceptance.md` §4.4. |
| NFR-SYN2-002 | Property: gate idempotence | For any input `text` accepted by `enforce_faithfulness()` (outcome `accepted` or `stripped`), running `enforce_faithfulness()` a second time on the resulting text SHALL produce outcome `accepted` and identical text (idempotence). For arbitrary input `docs` and arbitrary LLM-generated `text` containing `[N]` markers, every sentence in `enforced_text` SHALL contain at least one `[N]` marker valid in `[1, len(docs)]`. Property test via `hypothesis>=6` over a generator producing realistic LLM-style outputs (mixed cited/uncited sentences, varying punctuation, English + Korean). |

---

## 4. Acceptance Criteria

Detailed Given/When/Then scenarios with edge cases live in
`.moai/specs/SPEC-SYN-002/acceptance.md`. This section enumerates the
acceptance gate per requirement.

### REQ-SYN2-001 — Ubiquitous: every-sentence-cited invariant

- File `services/researcher/src/researcher/faithfulness.py` exists with
  the four exposed functions documented in §2.1(a).
- `test_every_sentence_has_marker`: feed an LLM-style output where
  every sentence ends with `[1]`, `[2]`, etc.; assert
  `enforce_faithfulness(text, docs, mode="strip", retry_count=0)`
  returns `EnforcementOutcome.ACCEPTED`.
- `test_preserves_spec_syn001_invariants`: SPEC-SYN-001 acceptance
  test suite (existing `test_app.py`, `test_synthesis.py`) MUST
  remain green after SPEC-SYN-002 implementation. (Run `pytest -q
  services/researcher/tests/` post-implementation.)
- `test_doc_id_unchanged_in_citations`: post-enforcement `Citation.doc_id`
  values match `docs[marker - 1].id` exactly. NFR-SYN-002 invariant
  preserved.

### REQ-SYN2-002 — Event-Driven: detect un-cited → retry once

- `test_uncited_triggers_retry`: simulate LLM mock returning
  `"Sentence A. Sentence B [1]."` (one un-cited sentence); assert
  `gateway.complete()` is invoked exactly twice (initial + retry);
  assert second mock returns fully-cited output and that the second
  output is what the endpoint returns.
- `test_retry_uses_stricter_system_prompt`: capture the second
  `gateway.complete()` invocation's `messages[0]["content"]` and
  assert it contains the substring "Every sentence MUST end with at
  least one citation marker".
- `test_retry_increments_counter_once`: assert
  `usearch_synthesis_faithfulness_retries_total` == 1 per request
  that retried, regardless of outcome.
- `test_retry_log_attribute_set`: assert the JSON log record's
  `retry_attempted: true` is emitted on retry path.
- `test_no_retry_when_first_pass_clean`: first-pass output fully
  cited; assert `gateway.complete()` invoked exactly once and the
  retries counter unchanged.

### REQ-SYN2-003 — State-Driven: mode=off bypass

- `test_mode_off_bypasses_gate`: with `RESEARCHER_FAITHFULNESS_MODE=off`,
  feed an output with un-cited sentences; assert response text is
  returned verbatim, no retry, no counter increment, log
  `faithfulness_action: "off"`.
- `test_mode_off_skips_faithfulness_module`: assert
  `enforce_faithfulness()` is NOT called (use `unittest.mock.patch`).
- `test_mode_off_preserves_perf`: latency identical to SPEC-SYN-001
  baseline (within ±10 ms on the 50-call p50 benchmark).

### REQ-SYN2-004 — Unwanted: reject mode HTTP 422

- `test_reject_mode_returns_422_with_error_body`: POST request,
  mock LLM returns un-cited output twice (initial + retry both fail);
  assert HTTP 422; assert body matches `{"error": "un_cited_output",
  "detail": "<N> sentence(s) without citations after retry",
  "uncited_count": <N>}` schema.
- `test_reject_mode_increments_counter`: assert
  `usearch_synthesis_faithfulness_outcomes_total{outcome="rejected"}`
  +1.
- `test_reject_mode_logs_warn`: assert exactly 1 WARN-level log
  record with attributes `{request_id, faithfulness_action: "rejected",
  uncited_sentences_count: <N>, retry_attempted: true}`.
- `test_reject_mode_no_text_in_response`: assert response 422 body
  contains no `text` field; client receives only the error structure.

### REQ-SYN2-005 — Optional: strip mode (default)

- `test_strip_mode_removes_uncited_sentences`: input
  `"A. B [1]. C. D [2]."` → output `"B [1]. D [2]."` (sentences A and
  C stripped).
- `test_strip_mode_preserves_order`: 5-sentence input with sentences
  1, 3, 5 cited → output retains those three in original order.
- `test_strip_mode_notice_set_when_stripped`: response notice ==
  `"<N> uncited sentence(s) stripped"`; on retry-then-strip path,
  notice reflects post-retry strip count.
- `test_strip_mode_outcome_counter`: counter
  `outcome="stripped"` +1 (no retry triggered) or
  `outcome="retry_failed"` +1 (retry then strip).
- `test_strip_mode_response_2xx`: status code 200 (success);
  `Result.Citations` reflects only the surviving citations.

### NFR-SYN2-001 — Latency

- `test_faithfulness_gate_latency`: `enforce_faithfulness()` on
  12-sentence paragraph 50 iterations; assert p99 ≤ 50 ms.
- `test_synthesize_p95_with_retry_under_limit`: 50 calls with stub
  LLM forced into retry; assert p95 ≤ 14.0 s.
- `test_no_retry_path_perf`: clean first pass — assert end-to-end
  latency overhead ≤ 100 ms vs. SPEC-SYN-001 baseline.

### NFR-SYN2-002 — Idempotence + property

- `test_enforce_faithfulness_idempotent`: for accepted/stripped
  outputs, second invocation returns `accepted` + identical text.
- `test_property_every_accepted_sentence_cited` (hypothesis): for
  generated `(text, docs)` pairs, every sentence in
  `enforce_faithfulness(...)` accepted output contains at least one
  `[N]` with `1 <= N <= len(docs)`.

---

## 5. Technical Approach (high-level, no implementation code)

Detailed plan, file impact, and test plan live in
`.moai/specs/SPEC-SYN-002/plan.md`. High-level approach:

- **Insertion point**: between `_process_markers` (synthesis.py:192)
  and `SynthesizeResponse(...)` return (lines 208–220). The new
  `enforce_faithfulness(...)` call wraps the existing structural
  validation result without changing its semantics.
- **No schema change**: `Citation`, `SynthesizeResponse`, Go-side
  `Result` shapes unchanged. Faithfulness is a behavioral overlay
  that influences what `text` and `citations` *contain*, not their
  *shape*.
- **Single retry max**: bounded by `max_retries=1` constant in
  `enforce_faithfulness()`; not configurable. FROZEN.
- **Mode selection**: `RESEARCHER_FAITHFULNESS_MODE` env var, three
  values, default `strip`. Loaded once per request at the top of
  `synthesize()`.
- **Sentence segmentation**: regex `[.!?。！？]\s+|[.!?。！？]$` covering
  English + Korean punctuation. v0 simple regex; pysbd upgrade
  deferred to M4 fast-follow if empirics warrant.

---

## 6. Risks (top-level summary)

Detailed risk register lives in `.moai/specs/SPEC-SYN-002/research.md`
§5. Top three for SPEC-author attention:

1. **False-positive rejection rate** — strict gate may reject 10–25%
   of Korean-locale outputs from Haiku 4.5. Mitigated via default
   mode `strip` (best UX) + observable retry-rate counter
   (`usearch_synthesis_faithfulness_retries_total`).
2. **Multi-claim-single-`[N]` semantic gap** — out of scope; deferred
   to SPEC-EVAL-001 (RAGAS).
3. **Hallucinated content under valid `[N]`** — out of scope;
   deferred to SPEC-EVAL-001.

---

## 7. References

Internal:

- `services/researcher/src/researcher/synthesis.py:66-118` — current
  `_process_markers` (extension chokepoint).
- `services/researcher/src/researcher/synthesis.py:192` — insertion
  point for `enforce_faithfulness()` call.
- `services/researcher/src/researcher/models.py:55-63` — `Citation`
  schema (already has `doc_id` field; no schema change).
- `pkg/types/normalized_doc.go:29` — pre-declared SPEC-SYN-002 hook
  on `Citations` field.
- `internal/obs/metrics/synthesis.go` — extension target for two new
  faithfulness collectors.
- `.moai/specs/SPEC-SYN-001/spec.md` lines 137–179 — SPEC-SYN-001
  Out-of-Scope §2.2 (faithfulness explicitly deferred to SPEC-SYN-002).
- `.moai/specs/SPEC-SYN-001/spec.md` line 202 — NFR-SYN-002 (structural
  mapping invariant SPEC-SYN-002 must preserve).
- `.moai/project/roadmap.md:64` — SPEC-SYN-002 row.
- `.moai/project/roadmap.md:101` — SPEC-EVAL-001 row (RAGAS boundary).
- `.moai/project/roadmap.md:151` — M4 exit criterion.
- `.moai/specs/SPEC-SYN-002/research.md` — companion research artifact.

External:

- `https://github.com/assafelovic/gpt-researcher` — local-doc citation
  pattern. Verified via SPEC-SYN-001 §11 (2026-04-28).
- `https://docs.anthropic.com/en/docs/build-with-claude/citations` —
  per-claim provenance contract pattern. Verified WebFetch 2026-04-15.
- `https://docs.ragas.io/en/stable/concepts/metrics/faithfulness.html`
  — semantic-faithfulness boundary (defines what SPEC-SYN-002 is NOT).
  Verified WebFetch 2026-03-22.
- `https://python.langchain.com/docs/concepts/retrieval/#citations`
  — single-retry-then-fail pattern lifted for REQ-SYN2-002. Verified
  WebFetch 2026-04-30.
- `https://github.com/nipunsadvilkar/pySBD` — pysbd library
  (deferred fast-follow for sentence segmentation).

---

*End of SPEC-SYN-002 v0.1 (draft)*
