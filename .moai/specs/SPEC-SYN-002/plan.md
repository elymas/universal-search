# SPEC-SYN-002 Implementation Plan

Companion artifact for `.moai/specs/SPEC-SYN-002/spec.md`.
Version: 0.1.0 (draft)
Created: 2026-05-09
Author: limbowl (via manager-spec)

---

## 1. Overview

SPEC-SYN-002 modifies the SPEC-SYN-001 synthesis pipeline to enforce
per-sentence `doc_id` provenance. It is a **brownfield enhancement**:
existing files are extended; one new module is added; no public API
changes; observability surface area extends with two new collectors.

Methodology: **TDD (RED-GREEN-REFACTOR)** per
`.moai/config/sections/quality.yaml` `development_mode: tdd`.
Coverage target: 85% (matches project default).

Harness level: **standard**. Sprint Contract optional but recommended
because the change touches the LLM-trust boundary.

---

## 2. Milestones (priority-based, no time estimates)

### Milestone 1 [Priority High] — Faithfulness module foundation

Goal: a pure-Python `faithfulness.py` module with the four exposed
functions, fully unit-tested, with no synthesis-pipeline integration
yet.

Deliverables:
- [NEW] `services/researcher/src/researcher/faithfulness.py`
- [NEW] `services/researcher/tests/test_faithfulness.py`
- All RED tests for sentence segmentation, un-cited detection, and
  the `EnforcementOutcome` enum.
- All GREEN implementations for the pure-Python helpers (no LLM
  calls; deterministic).
- Property tests via `hypothesis>=6` for NFR-SYN2-002 idempotence.
- Coverage ≥ 85% on the new module.

Exit criterion: `pytest -q services/researcher/tests/test_faithfulness.py`
green; module is reusable and importable but not yet wired.

### Milestone 2 [Priority High] — Pipeline integration (mode=strip default)

Goal: the synthesis pipeline calls `enforce_faithfulness()` and
returns stripped or accepted output. No retry path yet.

Deliverables:
- [MODIFY] `services/researcher/src/researcher/synthesis.py:synthesize()`
  invokes `enforce_faithfulness()` post-`_process_markers`.
- [MODIFY] `services/researcher/src/researcher/synthesis.py:build_prompt()`
  appends the stricter system prompt directive (REQ-SYN2-002 c).
- [MODIFY] `services/researcher/tests/test_synthesis.py` — strip-mode
  unit tests.
- [MODIFY] `services/researcher/tests/test_app.py` — strip-mode HTTP
  tests.
- Notice field populated when stripping occurs.
- SPEC-SYN-001 acceptance suite still green (regression check).

Exit criterion: full Python test suite green;
`RESEARCHER_FAITHFULNESS_MODE=strip` (default) produces clean output;
no retry yet.

### Milestone 3 [Priority High] — Single-retry path

Goal: when first-pass output has un-cited sentences, the pipeline
re-prompts once with the stricter system prompt; on second-pass
success, the second response is returned.

Deliverables:
- [MODIFY] `enforce_faithfulness()` exposes a `retry_required`
  outcome that the caller acts on.
- [MODIFY] `synthesis.py:synthesize()` implements the
  re-prompt-then-re-enforce loop bounded to one retry.
- [MODIFY] `gateway.py` — no schema change, but tests confirm the
  retry path adds a second `gateway.complete()` invocation.
- New tests: `test_uncited_triggers_retry`, `test_retry_success`,
  `test_retry_failure_strip`, `test_retry_failure_reject` (latter
  becomes meaningful in M4).
- Cost accumulates naturally (two `x-litellm-response-cost` values
  summed in `cost_usd`).

Exit criterion: retry path executed correctly across all three
fallback modes (`strip` / `reject` / `off`-bypass).

### Milestone 4 [Priority High] — Reject mode (HTTP 422)

Goal: with `RESEARCHER_FAITHFULNESS_MODE=reject`, post-retry failure
returns HTTP 422 with structured error body.

Deliverables:
- [MODIFY] `services/researcher/src/researcher/app.py` — handle
  `EnforcementOutcome.REJECTED` from `synthesize()` return value (or
  raise a custom `UncitedOutputError` and catch in
  `synthesize_endpoint`); return `JSONResponse(status_code=422, ...)`
  with the documented body.
- [MODIFY] `synthesize()` signature: returns `SynthesizeResponse` on
  success/strip, raises `UncitedOutputError` on reject path.
- New tests: `test_reject_mode_returns_422_with_error_body`,
  `test_reject_mode_increments_counter`, `test_reject_mode_logs_warn`.
- `mode=off` short-circuits before `enforce_faithfulness()` is called
  (REQ-SYN2-003).

Exit criterion: all four mode behaviors verified end-to-end via
HTTP tests.

### Milestone 5 [Priority Medium] — Observability surface

Goal: two new Prometheus collectors registered and exercised; JSON
log records carry the three new attributes; existing
`usearch_synthesis_calls_total` semantics unchanged.

Deliverables:
- [MODIFY] `internal/obs/metrics/synthesis.go` — add
  `SynthesisFaithfulnessOutcomes *prometheus.CounterVec{outcome}` and
  `SynthesisFaithfulnessRetries prometheus.Counter`.
- [MODIFY] `internal/obs/metrics/metrics.go` — register both via
  the existing `registerSynthesis(pr)` helper (single edit point per
  SPEC-OBS-001 import-boundary discipline).
- [MODIFY] `internal/obs/obs.go` — re-export both collectors as
  `obs.SynthesisFaithfulnessOutcomes` and `obs.SynthesisFaithfulnessRetries`.
- Python sidecar: `obs.py` — `log_synthesis()` payload extends with
  `uncited_sentences_count`, `faithfulness_action`, `retry_attempted`.
- Go-side change: NONE on the client (faithfulness happens entirely
  in Python). Outcome enum on Go side stays
  `{success, degraded, error_invalid, error_timeout, error_unreachable}`.
- New Python tests: `test_log_record_carries_faithfulness_attrs`,
  `test_metric_increments_per_mode_outcome`.

Exit criterion: Go test suite green for the two new collectors;
Python log records contain all expected attributes; cardinality
allowlist unchanged.

### Milestone 6 [Priority Medium] — Documentation and examples

Goal: `.env.example`, README snippet, sample request/response
showcasing the three modes.

Deliverables:
- [MODIFY] `.env.example` — `RESEARCHER_FAITHFULNESS_MODE=strip`
  with comment listing valid values.
- [MODIFY] `services/researcher/README.md` (if exists) — section on
  faithfulness behavior + mode selection.
- Sample curl snippets for `mode=reject` 422 path and `mode=strip`
  200 path.

Exit criterion: documentation reflects implemented behavior; running
the sample requests produces the documented responses.

---

## 3. Technical Approach

### 3.1 Module boundary diagram

```
┌────────────────────────────────────────────────────────────────────┐
│                         services/researcher/                       │
│                                                                    │
│  app.py:synthesize_endpoint                                        │
│   └→ synthesis.py:synthesize                                       │
│        ├→ build_prompt(query, lang, docs)        [MODIFY]          │
│        ├→ gateway.complete(...)                  [EXISTING]        │
│        ├→ _process_markers(text, docs)           [EXISTING]        │
│        ├→ ★ enforce_faithfulness(text, docs, mode) [NEW INTEGRATION]│
│        │   └→ faithfulness.py:enforce_faithfulness [NEW MODULE]    │
│        │      ├→ split_sentences(text)                             │
│        │      ├→ find_uncited_sentences(text)                      │
│        │      └→ EnforcementOutcome enum                           │
│        ├→ [retry path] gateway.complete(...)     [single retry]    │
│        ├→ [retry path] _process_markers + enforce_faithfulness     │
│        └→ SynthesizeResponse(...) OR raise UncitedOutputError      │
└────────────────────────────────────────────────────────────────────┘
```

### 3.2 [NEW] `faithfulness.py` API surface

```
class EnforcementOutcome(StrEnum):
    ACCEPTED         = "accepted"
    RETRY_REQUIRED   = "retry_required"
    STRIPPED         = "stripped"
    REJECTED         = "rejected"
    # Note: mode=off short-circuits in synthesize() before
    # enforce_faithfulness() is invoked (see §3.3 line 212), so
    # no OFF enum value is required. Mode-off logging uses the
    # `faithfulness_action: "off"` log attribute (see §10 D3).

def split_sentences(text: str) -> list[str]: ...
def find_uncited_sentences(text: str) -> list[tuple[int, str]]: ...
def enforce_faithfulness(
    text: str,
    docs: list[NormalizedDocPayload],
    mode: Literal["strip", "reject", "off"],
    retry_attempted: bool,
) -> tuple[EnforcementOutcome, str, int]:  # (outcome, possibly-modified-text, uncited_count)
    ...
```

### 3.3 [MODIFY] `synthesis.py:synthesize()` flow

```
async def synthesize(req, gateway):
    mode = os.environ.get("RESEARCHER_FAITHFULNESS_MODE", "strip")

    # 1. First pass (existing flow, unchanged)
    text_raw, cost, usage, provider, model = await gateway.complete(...)
    cleaned_text, citations = _process_markers(text_raw, req.docs)

    # 2. NEW: faithfulness gate
    if mode == "off":
        return _build_response(req, cleaned_text, citations, cost, usage, ...)

    outcome, gated_text, uncited_count = enforce_faithfulness(
        cleaned_text, req.docs, mode=mode, retry_attempted=False,
    )
    if outcome == EnforcementOutcome.ACCEPTED:
        return _build_response(...)

    # 3. NEW: retry path
    if outcome == EnforcementOutcome.RETRY_REQUIRED:
        text_raw_2, cost_2, usage_2, ... = await gateway.complete(...stricter prompt...)
        cleaned_text_2, citations_2 = _process_markers(text_raw_2, req.docs)
        outcome, gated_text, uncited_count = enforce_faithfulness(
            cleaned_text_2, req.docs, mode=mode, retry_attempted=True,
        )
        # Increment retry counter
        # Sum costs
        cost = cost + cost_2

    # 4. NEW: terminal outcome handling
    if outcome == EnforcementOutcome.REJECTED:
        raise UncitedOutputError(uncited_count=uncited_count)

    # outcome ∈ {ACCEPTED, STRIPPED}
    return _build_response(..., text=gated_text, ...)
```

### 3.4 [MODIFY] `app.py` — error handler for `UncitedOutputError`

```
@app.exception_handler(UncitedOutputError)
async def uncited_output_handler(request, exc):
    return JSONResponse(
        status_code=422,
        content={
            "error": "un_cited_output",
            "detail": f"{exc.uncited_count} sentence(s) without citations after retry",
            "uncited_count": exc.uncited_count,
        },
    )
```

### 3.5 [NEW] Two Prometheus collectors

```go
// internal/obs/metrics/synthesis.go (additions)

SynthesisFaithfulnessOutcomes *prometheus.CounterVec  // labels: [outcome]
SynthesisFaithfulnessRetries  prometheus.Counter

// outcome label values: accepted, stripped, rejected, retry_succeeded, retry_failed
// (5 values; mode=off bypasses the metric entirely — counter is NOT emitted in mode=off
//  per REQ-SYN2-003. See §10 Decisions D2.)
```

Cardinality: 5 enum values × 1 metric = 5 series. Below the
SPEC-OBS-001 cap of 64. Allowlist unchanged (`outcome` already
declared).

### 3.6 MX Tag Plan

Per `.claude/rules/moai/workflow/mx-tag-protocol.md`:

| Target | Tag | Rationale |
|--------|-----|-----------|
| `faithfulness.py:enforce_faithfulness()` | `@MX:ANCHOR` | Public API; fan_in ≥ 3 (synthesis.py first pass + retry pass + tests); LLM-trust boundary |
| `faithfulness.py:enforce_faithfulness()` | `@MX:WARN` | LLM-trust boundary — accepts un-validated LLM output and decides reject/strip; @MX:REASON: Single source of truth for citation provenance enforcement |
| `synthesis.py:synthesize()` (already has ANCHOR from SPEC-SYN-001) | UPDATE | fan_in unchanged; SPEC reference extended to include SPEC-SYN-002 |
| `synthesis.py:build_prompt()` | `@MX:NOTE` | Contains the strict-mode prompt directive whose wording governs retry success rate |
| `app.py:uncited_output_handler` (new) | `@MX:NOTE` | Maps domain exception to HTTP 422; reject-mode contract surface |
| `internal/obs/metrics/synthesis.go` `SynthesisFaithfulnessOutcomes` | `@MX:NOTE` | Counter cardinality discipline note (5 enum values; mode=off bypasses counter per REQ-SYN2-003) |

No `@MX:TODO` planned (TDD GREEN phase resolves them inline).

---

## 4. Risks (top 3, summary — full register in research.md §5)

1. **False-positive rejection** under strict gate on Korean prose
   — mitigated via default `mode=strip` and observable retry-rate
   counter.
2. **Multi-claim-single-`[N]` semantic gap** — explicitly out of
   scope; deferred to SPEC-EVAL-001.
3. **Hallucinated content under valid `[N]`** — explicitly out of
   scope; deferred to SPEC-EVAL-001.

---

## 5. Dependencies

### 5.1 Upstream SPEC dependencies (must be implemented)

- **SPEC-SYN-001** (implemented): provides
  `services/researcher/src/researcher/synthesis.py:_process_markers`,
  `Citation` Pydantic model, `Result.Citations` Go shape, the
  `synthesize` endpoint contract, and the `usearch_synthesis_*`
  metric family + cardinality discipline.
- **SPEC-CORE-001** (implemented): `pkg/types.NormalizedDoc.ID`
  (the `doc_id` source).
- **SPEC-LLM-001** (implemented): LiteLLM proxy + retry semantics;
  cost accumulation via `x-litellm-response-cost` header.

### 5.2 Coordinating SPECs (no hard dependency)

- **SPEC-OBS-001** (implemented): metric registration pattern; the
  two new faithfulness collectors follow the
  `internal/obs/metrics/synthesis.go` precedent.
- **SPEC-IR-001** (implemented): `lang` hint routing — unaffected;
  the strict-mode prompt directive layers on top of the existing
  language directive.

### 5.3 Downstream blocked SPECs

- **SPEC-EVAL-001** (M4): RAGAS-style semantic faithfulness scoring
  consumes SPEC-SYN-002's structural-faithfulness output as a
  baseline; the score numerator (cited statements) is well-defined
  only after SPEC-SYN-002 lands.

### 5.4 External dependencies (run-phase)

New Python runtime dependency: **none** (regex from `re`, optional
`hypothesis>=6` already present from SPEC-SYN-001 dev deps).
Optional fast-follow: `pysbd>=0.3.4` if regex segmentation produces
empirical false-positives ≥ 5%.

No new Go module dependencies.

---

## 6. File Impact

### 6.1 [NEW] Files to create

| Path | Purpose |
|------|---------|
| `services/researcher/src/researcher/faithfulness.py` | Pure-Python faithfulness module (REQ-SYN2-001/002/003/005) |
| `services/researcher/tests/test_faithfulness.py` | Unit + property tests for faithfulness module (NFR-SYN2-002) |

### 6.2 [MODIFY] Files to modify

| Path | Change |
|------|--------|
| `services/researcher/src/researcher/synthesis.py` | Insert `enforce_faithfulness()` call post-`_process_markers`; add retry loop; raise `UncitedOutputError` on reject path; extend `build_prompt()` system message |
| `services/researcher/src/researcher/app.py` | Register `UncitedOutputError` exception handler returning 422 |
| `services/researcher/src/researcher/obs.py` | Extend `log_synthesis()` payload with `uncited_sentences_count`, `faithfulness_action`, `retry_attempted` |
| `services/researcher/tests/test_synthesis.py` | Add tests for all four faithfulness modes + retry path |
| `services/researcher/tests/test_app.py` | Add HTTP-level tests for reject mode (422) and strip mode (200) |
| `internal/obs/metrics/synthesis.go` | Add `SynthesisFaithfulnessOutcomes` and `SynthesisFaithfulnessRetries` collectors |
| `internal/obs/metrics/metrics.go` | Register the two new collectors via `registerSynthesis(pr)` |
| `internal/obs/obs.go` | Re-export the two new collector handles |
| `.env.example` | Add `RESEARCHER_FAITHFULNESS_MODE=strip` with comment |

### 6.3 [EXISTING — UNCHANGED]

| Path | Reason |
|------|--------|
| `services/researcher/src/researcher/models.py` | `Citation.doc_id` already exists; no schema change |
| `services/researcher/src/researcher/gateway.py` | Retry path adds a second `gateway.complete()` call but no API change |
| `internal/synthesis/types.go` | Go-side `Result` and `Citation` shapes unchanged |
| `internal/synthesis/client.go` | Outcome enum on Go side unchanged; faithfulness happens entirely in Python sidecar |
| `internal/synthesis/client_test.go` | Existing tests still valid |
| `pkg/types/normalized_doc.go` | `ID` field already canonical |
| `deploy/docker-compose.yml` | No new service; env var added via .env.example |
| `services/researcher/Dockerfile` | No build change |

---

## 7. Test Plan (development cycle)

Methodology: TDD RED-GREEN-REFACTOR per
`.moai/config/sections/quality.yaml`.

### RED phase order (failing tests first)

1. `test_split_sentences_english_periods` — basic sentence split.
2. `test_split_sentences_korean_punctuation` — `。！？`-aware split.
3. `test_find_uncited_sentences_returns_indices` — locate un-cited.
4. `test_enforce_faithfulness_accepted` — fully cited input → ACCEPTED.
5. `test_enforce_faithfulness_strip_mode` — un-cited sentences removed.
6. `test_enforce_faithfulness_reject_mode_first_pass` → RETRY_REQUIRED.
7. `test_enforce_faithfulness_reject_mode_after_retry` → REJECTED.
8. `test_enforce_faithfulness_off_mode` → OFF.
9. `test_enforce_faithfulness_idempotent` (NFR-SYN2-002).
10. `test_property_every_accepted_sentence_cited` (hypothesis).
11. `test_uncited_triggers_retry` (integration with synthesis.py).
12. `test_retry_uses_stricter_system_prompt`.
13. `test_retry_increments_counter_once`.
14. `test_strip_mode_response_2xx_with_notice` (HTTP).
15. `test_reject_mode_returns_422_with_error_body` (HTTP).
16. `test_mode_off_bypasses_gate` (HTTP + counter assertion).
17. `test_log_record_carries_faithfulness_attrs`.
18. `test_metric_increments_per_mode_outcome` (Go-side).
19. `test_faithfulness_gate_latency_under_50ms` (NFR-SYN2-001).
20. `test_synthesize_p95_with_retry_under_14s` (NFR-SYN2-001).

### GREEN phase

Implement minimum code to pass each test. Order: M1 → M2 → M3 → M4
→ M5. M6 (docs) is post-GREEN.

### REFACTOR phase

- Extract retry-path repetition into a helper (`_run_pass()` →
  returns `(outcome, text, citations, cost, usage)`).
- DRY the JSON log records via a single `_compose_log_record()` helper.
- Pre-submission self-review per `workflow-modes.md`: confirm
  abstractions earn their complexity; remove anything that doesn't.

### Coverage targets

- Python `services/researcher/`: ≥85% (project default).
- Go `internal/obs/metrics/synthesis.go`: ≥85%.
- Go `internal/synthesis/`: unchanged from SPEC-SYN-001.

---

## 8. Quality gates (TRUST 5)

| Pillar | Check |
|--------|-------|
| **Tested** | 20 RED-phase tests + property tests; ≥85% coverage on new module |
| **Readable** | `enforce_faithfulness()` ≤ 60 LOC; helper functions ≤ 30 LOC each; godoc/docstring on every exported function |
| **Unified** | `ruff check` green; `gofmt -d` empty; matches existing SPEC-SYN-001 style |
| **Secured** | No new external attack surface; LLM input still passes through existing 422 / structural-validation guards; no PII in log records (REQ-SYN-006 redaction continues to apply) |
| **Trackable** | Conventional commit `feat(synthesis): SPEC-SYN-002 citation faithfulness enforcement`; @MX tags applied per §3.6; SPEC reference in commit message |

---

## 9. Open Questions (to resolve in run phase)

1. ~~Should `mode=off` log records include `faithfulness_action: "off"`
   or omit the attribute entirely?~~
   **RESOLVED — see §10 D3.** The canonical `faithfulness_action`
   enum includes `"off"` as one of its 4 values, and REQ-SYN2-003
   normatively requires the attribute to be set to `"off"` in
   mode=off. No omission permitted.
2. Should the strict-mode system-prompt directive be locale-aware
   (Korean version "모든 문장은 [N] 인용 표시로 끝나야 합니다.")?
   Default: English-only directive (LLMs handle the cross-locale
   instruction reliably per SPEC-SYN-001 acceptance evidence).
   Decision deferred to run phase.
3. ~~When `mode=strip` produces an empty output (all sentences
   stripped), should the response be a degraded-mode bullet list?~~
   **RESOLVED — see §10 Decisions D1.**
4. Cost counter on retry path: should the retry's `cost_usd` be
   counted toward `usearch_synthesis_cost_usd_total` exactly once per
   request (i.e., sum) or as two separate increments? Default: sum
   (single increment of `cost1 + cost2`) to preserve REQ-SYN-006
   "exactly once per top-level invocation".

---

## 10. Decisions (resolved during plan-auditor review-1)

These items were initially open questions or ambiguities that have
been resolved before run-phase entry. Each entry records the
resolution and its propagation.

### D1 — Empty post-strip output (resolves §9 Q3)

**Context**: When `mode=strip` and post-strip text is empty (all
sentences un-cited even after retry), what response should the
service return?

**Decision**: Return HTTP 422 with the structured error body
`{"error": "un_cited_output", "detail": "<N> sentence(s) without
citations after retry", "uncited_count": <N>}` — identical to the
REQ-SYN2-004 reject-mode contract. **No degraded fallback.**

**Rationale**:
- Simplest behavior — one terminal error contract for both
  `reject` mode and `strip` mode-with-empty-result.
- Consistent with REQ-SYN2-004 reject path; client error handling
  unifies (clients that handle 422 from reject mode automatically
  handle the empty-strip case).
- Avoids introducing a `degraded=true` overload into the
  faithfulness path (the SPEC-SYN-001 degraded shape exists for
  *connection* failures, not for *semantic-quality* failures).

**Propagation**:
- `acceptance.md` Edge Case 1 — rewritten to assert HTTP 422
  response and corresponding `outcome="rejected"` counter
  increment (regardless of configured mode, when post-strip text
  is empty).
- `acceptance.md` Edge Case 7 — rewritten consistently.
- `synthesize()` flow (§3.3): when `outcome == STRIPPED` and
  `gated_text == ""`, raise `UncitedOutputError(uncited_count=N)`
  identical to the reject path.

### D2 — `outcome` metric label cardinality (resolves cross-document mismatch)

**Context**: spec.md §2.1(f) listed 5 `outcome` label values;
plan.md §3.5, spec-compact.md, and acceptance.md §4.5 listed 6
(including `off`). REQ-SYN2-003 explicitly mandates that the
counter SHALL NOT increment when `mode == "off"`.

**Decision**: Drop `off` from the `outcome` label enumeration.
Canonical 5-label set: `{accepted, stripped, rejected,
retry_succeeded, retry_failed}`. In `mode=off`, the counter is
NOT emitted at all (full bypass per REQ-SYN2-003), so an
`outcome="off"` series can never exist.

**Rationale**:
- Keeps REQ-SYN2-003 truthful (counter SHALL NOT increment in
  mode=off).
- Reduces metric cardinality from 6 → 5 series.
- The "did mode=off bypass occur?" question is answered by log
  records (`faithfulness_action: "off"`), not by metrics.

**Propagation**:
- spec.md §2.1(f): 5-value list (already correct, no change).
- plan.md §3.5: 6 → 5; cardinality calc 6×1=6 → 5×1=5 (this file).
- spec-compact.md §Observability: `| off` removed.
- acceptance.md §4.5: enum count 6 → 5; `off` removed from list.

### D3 — `faithfulness_action` log enum (resolves §2.1(e) inconsistency)

**Context**: spec.md §2.1(e) listed 4 values
`{accepted, retry, stripped, rejected}` (with ambiguous bare
`retry`, no `off`). REQ-SYN2-003 requires `faithfulness_action
== "off"` in mode=off — referencing a value not in the enum.
spec-compact.md additionally listed `off`.

**Decision**: Canonical 4-value set:
`{accepted, stripped, rejected, off}`. The bare `retry` value is
removed; retry status is captured separately by the boolean
attribute `retry_attempted: bool`. The 4 values represent the
**final action taken** on the response.

**Rationale**:
- Each value maps unambiguously to a terminal outcome of the
  faithfulness gate.
- `retry` is not a terminal action; it is an in-flight transition.
  Capturing it as a boolean (`retry_attempted`) avoids enum
  pollution.
- `off` is included so REQ-SYN2-003's contract is satisfiable.

**Propagation**:
- spec.md §2.1(e): updated to 4 values (this revision).
- spec-compact.md §Observability: bare `retry` removed.
- acceptance.md scenarios: already use only the 4-value set —
  verified, no edits required.

---

*End of SPEC-SYN-002 plan v0.1*
