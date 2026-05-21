# Plan Audit: SPEC-DEEP-002 (Iteration 2/3)

Date: 2026-05-21
Auditor: plan-auditor (adversarial mode)
Harness: standard
Previous: .moai/reports/plan-audit/SPEC-DEEP-002-review-1.md (iter 1 PASS, 1 MAJOR + 7 MINOR)
Patch state: v0.1.0 → v0.1.1
Context Isolation: enforced (no author reasoning consumed; only spec.md, plan.md, acceptance.md, spec-compact.md, and iter-1 report consulted)

## Verdict: PASS

## Summary

All four claimed v0.1.1 patches (P-M1 REQ-009 split, P-N1 REQ-001 SHALL conversion, P-N2 §1.1 footnote, P-N3 cardinality narrowing) are VERIFIED in spec.md and propagate correctly to plan.md and spec-compact.md. M1 from iter-1 is resolved (REQ-DEEP2-009 cleanly split into 009a/009b with separate IF/THEN preconditions, separate test groupings, and per-REQ scenario coverage). One MINOR regression is introduced by incomplete P-N3 propagation: `acceptance.md:L458` still asserts the pre-narrowing cardinality `16` for `usearch_deep_agent_duration_seconds`, contradicting `spec.md:L305` (`4×2=8`) and `spec-compact.md:L108` (same). Carried-over MINORs N4–N7 from iter 1 remain unaddressed by design per the author's stated patch scope ("No other REQ/NFR/Exclusion/Scenario changes"). No BLOCKER, no MAJOR, and all must-pass dimensions clear the threshold.

## Patch Verification Results

| Patch | Status | Notes |
|---|---|---|
| P-M1 (REQ-009 split) | VERIFIED | `spec.md:L284` REQ-DEEP2-009a single-IF/THEN for max-retry exhaustion; `spec.md:L285` REQ-DEEP2-009b single-IF/THEN for Researcher/Reviewer/Verifier non-recoverable error. `spec.md:L260` (§2.1) lists "REQ-002, 003, 005, 009a, 009b, 012" in Pipeline module. `spec.md:L328` Scenario 3 → REQ-009a, `spec.md:L333` Scenario 8 → REQ-009b. `acceptance.md:L38` confirms "13개 EARS REQ … 009a, 009b". `plan.md:L287-296` maps `TestMaxRetryExhaustionReturns503`/`TestMaxRetryExhaustionEmitsPipelineFailedSSE` → 009a and `TestResearcherErrorAbortsAndReturns503`/`TestReviewerErrorAbortsAndReturns503`/`TestVerifierErrorAbortsAndReturns503` → 009b. `spec-compact.md:L59-70` lists both as separate bullets. REQ count 12 → 13 confirmed (`spec-compact.md:L28` "EARS Requirements (13)"). |
| P-N1 (REQ-001 SHALL) | VERIFIED | `spec.md:L273` now reads "도입 SHALL 한다 … 라우팅 SHALL 하며 … 처리 SHALL 한다 … 수용 SHALL 하되 … emit SHALL 한다" — explicit SHALL verbs interleaved with Korean prose. Indicative-mood verbs from v0.1.0 (`도입한다`, `처리한다`, `이다`) eliminated from the normative clauses. Semantic content unchanged: `?mode=` introduction, `mode=agents` → `deep_agents_handler` routing, `{request_id, query, lang}` schema with `docs[]` removed, SSE+`?stream=false` JSON output paths — all preserved. `spec-compact.md:L32-36` mirror updated identically. |
| P-N2 (§1.1 footnote) | VERIFIED | `spec.md:L124` marker `verifier_result`)[¹] embedded in decision #7. `spec.md:L130-138` footnote body correctly states research.md §1 originally proposed `final_token` and this SPEC substitutes `verifier_result` after SYN-004 compatibility analysis, linking to RDC-6's pre-buffering decision. `spec.md:L431-433` §8 Exclusions entry for the deviation REMAINS intact ("`final_token` SSE 이벤트 명칭이 research.md §6에 언급되었으나, 본 SPEC은 이 이름의 이벤트를 emit하지 않는다"). `spec-compact.md:L211` cross-references the footnote. Footnote satisfies iter-1 N2 recommendation. |
| P-N3 (cardinality narrowing) | PARTIAL | `spec.md:L305` REQ-DEEP2-008 narrows duration histogram outcome to `{success, error}` (2 values), separates retry attribution to `usearch_deep_agent_retries_total` counter (`agent` label, value `writer` only, cardinality 1), explicitly notes "per-attempt histogram observes one bucket sample per agent invocation regardless of retry status, with retry counts tracked separately by the `usearch_deep_agent_retries_total` counter below; cardinality 4×2=8". `spec.md:L314` NFR-002 aligned: `outcome ∈ {success, error}`. `spec-compact.md:L106-113` mirrors. No remaining `outcome="retried"` or `outcome="timeout"` reference on the duration histogram in any of the four docs (the `fail_timeout` label on `usearch_deep_agent_verifier_gate_results_total` is unrelated and correctly retained). **Stale value remains in `acceptance.md:L458`**: "`usearch_deep_agent_duration_seconds{agent, outcome}` (cardinality 16)" — should be 8 after narrowing. Cross-doc inconsistency. See A1 below. |
| Version bump | VERIFIED | `spec.md:L3` 0.1.1, `plan.md:L3` 0.1.1, `acceptance.md:L3` 0.1.1, `spec-compact.md:L3` `Version: 0.1.1`. `spec.md:L23-36` new HISTORY entry dated 2026-05-21 enumerating the four patches. `spec.md:L38-87` v0.1.0 entry remains intact (correctly preserved as historical record). |

## Findings by Severity

### BLOCKER

None.

### MAJOR

None. (Iter-1 M1 resolved by P-M1; no new MAJOR introduced.)

### MINOR

- [A1] (NEW, iter-2) `acceptance.md:L458` — Quality Gate 4.5 still asserts `usearch_deep_agent_duration_seconds{agent, outcome}` cardinality `16`. After P-N3 narrowed `outcome` from `{success, error, timeout, retried}` (4 values) to `{success, error}` (2 values), the correct cardinality is `4 × 2 = 8`, matching `spec.md:L305` and `spec-compact.md:L108`. Cross-document inconsistency introduced by incomplete propagation of P-N3. Fix: change "(cardinality 16)" → "(cardinality 8)" at `acceptance.md:L458`.

- [A2] (NEW, iter-2, very minor) `plan.md:L295-296` — TDD catalog uses non-sequential numbering "23a." for `TestReviewerErrorAbortsAndReturns503` (inserted between items 23 and 24 by P-M1). The total test count at `plan.md:L370` still reads "57개" but with the 23a insertion the actual count is 58. Either renumber 23a → 24 (and shift subsequent items) or update the total to 58. Cosmetic; does not affect SPEC semantics or testability.

- [N3-partial] (carried from iter 1, partially resolved) Original iter-1 N3 flagged that REQ-008 declared `outcome ∈ {success, error, timeout, retried}` without scenarios exercising `retried`/`timeout`. P-N3 resolves the labelling side by narrowing the bounded set. Scenario coverage is now consistent with the narrowed set (`outcome="success"` and `outcome="error"` are both exercised in acceptance.md scenarios 1, 2, 3, 8 and edge cases 1, 2). Treated as RESOLVED.

- [N4] (carried from iter 1, unaddressed by design) `acceptance.md:L302` typo `TestEmptyFanoutOutconeCounterIncrements` (should be `…OutcomeCounterIncrements` per `plan.md:L303`). Not in the author's stated patch scope; carried forward.

- [N5] (carried from iter 1, unaddressed by design) `spec.md:L284-285` REQ-009a/009b normative text still bundles HTTP 503 + SSE behavior without disambiguating that HTTP 503 applies to the buffered/JSON path while the SSE path returns HTTP 200 with terminal `pipeline_failed` event. `acceptance.md:L156-160` (Scenario 3) clarifies the SSE-vs-JSON HTTP status correctly, so the implementation contract is unambiguous, but the SPEC text alone is still ambiguous. Carried forward.

- [N6] (carried from iter 1, unaddressed by design) `spec.md:L347-348` env-vars `DEEP_AGENT_WRITER_RETRY_DELAY_MS` (owned by REQ-003) and `DEEP_AGENT_VERIFIER_TIMEOUT_MS` (owned by REQ-006) are not mentioned in their owning REQ texts. Carried forward.

- [N7] (carried from iter 1, unaddressed by design) `spec.md:L274` REQ-DEEP2-010 still uses "WHERE" Optional preamble for a request-time predicate that is semantically Event-Driven. Carried forward.

## Regression Check

Re-applied all 8 audit dimensions from iter 1:

**A. EARS Compliance** — IMPROVED. REQ-001 now uses explicit SHALL (P-N1 resolves N1). REQ-009 split into two well-formed Unwanted patterns, each with single IF/THEN (P-M1 resolves M1). N7 unchanged. **Pass.**

**B. Traceability** — Re-verified end-to-end (not spot-checked):
  - REQ-001 → Scenario 1, tests in `plan.md:L249-262` (§3.1)
  - REQ-002 → Scenarios 1, 4, tests `plan.md:L266-275` (item 9, 10)
  - REQ-003 → Scenario 2, tests `plan.md:L273-280` (items 13-16)
  - REQ-004 → Edge 3, tests `plan.md:L310-319` (§3.3 items 29-33)
  - REQ-005 → Scenario 1, tests `plan.md:L281-286` (items 17-19)
  - REQ-006 → Scenario 1, tests `plan.md:L323-334` (§3.4)
  - REQ-007 → Scenarios 1-4, tests `plan.md:L338-350` (§3.5)
  - REQ-008 → Scenarios 1-3, tests `plan.md:L351-360` (§3.5)
  - REQ-009a → Scenario 3, tests `TestMaxRetryExhaustionReturns503`, `TestMaxRetryExhaustionEmitsPipelineFailedSSE`, `TestErrorOutcomeCounterIncrementsExactlyOnce` (`plan.md:L287-289, L297-299`)
  - REQ-009b → Scenario 8 + Edge 1 + Edge 2, tests `TestResearcherErrorAbortsAndReturns503`, `TestReviewerErrorAbortsAndReturns503`, `TestVerifierErrorAbortsAndReturns503`, `TestNonVerifierErrorsDoNotTriggerRetry` (`plan.md:L279-280, L291-296`)
  - REQ-010 → Scenario 6, tests `plan.md:L255-258` (items 4, 5)
  - REQ-011 → Scenario 5 + Edge 4, tests `plan.md:L259-262` (items 6, 7, 8)
  - REQ-012 → Scenario 7, tests `plan.md:L300-307` (items 25-28)
  All 13 REQs covered by ≥1 scenario AND ≥1 named test. depends_on list (8 SPECs) unchanged from iter 1. **Pass.**

**C. Internal Consistency** — One new inconsistency (A1: acceptance.md L458 cardinality). All four files version-bumped. Module breakdown (§2.1) lists REQs correctly. No version drift across companion docs. **Pass with MINOR.**

**D. Completeness** — 13 REQs, 4 NFRs, 8 main + 4 edge scenarios, ~14 Exclusions entries, all required frontmatter fields present. HISTORY has v0.1.0 + v0.1.1 entries dated 2026-05-21. **Pass.**

**E. Architectural Pinning** — 8 decisions in §1.1 still align with research.md §1; the lone deviation (decision #7 event taxonomy) is now explicitly footnoted (P-N2 resolves N2). §8 Exclusions still defers per-user quota → DEEP-004, tree exploration → DEEP-003, LLM-as-judge → SPEC-EVAL-001. **Pass.**

**F. Risk Resolution** — All 10 RDCs from research.md §7 still mapped 1:1 (`spec.md:L156-217`). RDC-9 (`spec.md:L205-212`) updated to reference both REQ-009a and REQ-009b post-split, correctly distinguishing max-retry-exhaustion vs agent-error-abort branches. `plan.md:L391` Risk matrix row R9 updated to reference REQ-009a and REQ-009b. **Pass.**

**G. NFR Realism** — NFR-DEEP2-001 latency budget unchanged (p95 ≤ 60s, scoped to first/second Verifier attempt). NFR-DEEP2-002 cardinality calculation now consistent with P-N3 narrowing (`outcome ∈ {success, error}`). New total cardinality (8 + 1 + 3 = 12 across the three new collectors) is reduced from v0.1.0's 16 + ? + 3 = larger. **Pass.**

**H. EARS Anti-patterns** — REQ-009 split removes the iter-1 M1 conflation. No new anti-patterns introduced. REQ-009a, 009b, 012 are clean single-IF/single-THEN Unwanted patterns. **Pass.**

### Specific regression questions (per audit checklist):

- *Did P-M1 introduce a new traceability gap (REQ-009b without acceptance scenario)?*
  No. REQ-009b is covered by Scenario 8 (Researcher error, `acceptance.md:L305-330`), Edge Case 1 (Reviewer error, `acceptance.md:L336-348`), and Edge Case 2 (Verifier endpoint 5xx, `acceptance.md:L350-365`). Three independent named tests map to REQ-009b in `plan.md:L291-296`.

- *Did P-N1 inadvertently change REQ-001's semantic content?*
  No. Mode dispatch behavior unchanged — `?mode=` parameter introduction, `mode=agents` routing to `deep_agents_handler`, `{request_id, query, lang}` schema with `docs[]` removal, dual SSE/JSON output paths. Only verb-mood was rewritten to satisfy EARS SHALL convention.

- *Did P-N3 narrowing leave dangling references to `outcome="retried"` or `outcome="timeout"` elsewhere?*
  No on the duration histogram. The `fail_timeout` value on the unrelated `usearch_deep_agent_verifier_gate_results_total{result}` counter is retained and correct. However, the cardinality VALUE was not propagated to `acceptance.md:L458` (A1).

- *Did version bump miss any of the 4 files?*
  No. All four frontmatters carry 0.1.1.

## Dimension Scores

| Dimension | Score | Notes |
|---|---|---|
| A. EARS Compliance | PASS | All 13 REQs follow single-pattern, single-condition structure with explicit SHALL. REQ-001 SHALL conversion (P-N1) and REQ-009 split (P-M1) resolve iter-1 M1+N1. |
| B. Traceability | PASS | All 13 REQs covered by ≥1 scenario AND ≥1 named test. 8 `depends_on` entries unchanged and existing on disk. |
| C. Internal Consistency | PASS (with MINOR) | Frontmatter versions all 0.1.1. spec-compact.md mirrors spec.md (REQ-009a/b, footnote, cardinality narrowing). Module §2.1 listing correct. New inconsistency: acceptance.md:L458 cardinality stale (A1). |
| D. Completeness | PASS | 8 main + 4 edge scenarios (≥6), ~14 Exclusions (≥4), 5 modules (≤5), HISTORY first entry 2026-05-21, all 8 frontmatter fields present. |
| E. Architectural Pinning | PASS | 8 decisions match research.md §1 with one explicitly footnoted refinement (decision #7, P-N2). §8 Exclusions correctly defers per-user quota → DEEP-004, tree exploration → DEEP-003, LLM-as-judge → SPEC-EVAL-001. |
| F. Risk Resolution | PASS | All 10 RDCs from research.md §7 addressed. RDC-9 correctly bifurcated post-009-split. plan.md §4 Risk matrix updated. |
| G. NFR Realism | PASS | NFR-002 cardinality calculation aligned with narrowed outcome set. NFR-001 latency budget unchanged. |
| H. EARS Anti-patterns | PASS | No conflated Unwanted patterns. No weasel words. No implementation-detail REQs. REQ-009 split eliminates iter-1 M1. |

## PASS / FAIL Threshold

Same threshold as iter 1: PASS = 0 BLOCKER AND <2 MAJOR AND all must-pass dimensions (A, B, C, E, F, H) are PASS.

Actual: 0 BLOCKER, 0 MAJOR, all must-pass dimensions PASS → **PASS**.

## Next Action

Proceed to Phase 2.5 (annotation cycle with user).

Recommended (non-blocking) cleanup before /moai sync or v0.1.2 minor bump:

1. Fix A1: `acceptance.md:L458` change `(cardinality 16)` → `(cardinality 8)` for the `usearch_deep_agent_duration_seconds{agent, outcome}` line. One-line edit.
2. Fix A2: `plan.md:L295` renumber `23a.` → `24.` (shift subsequent items by +1) OR update `plan.md:L370` total from "57개" to "58개". Cosmetic.
3. Address carried-over MINORs N4–N7 from iter 1 at next opportunity (typo fix, HTTP-status disambiguation in REQ-009a/b normative text, env-var references in REQ-003/006 normative text, REQ-010 pattern reclassification). None blocking.

Implementation may proceed against v0.1.1 as-is.
