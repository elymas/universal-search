Verdict: FAIL

# SPEC-SYN-002 Review Report
Iteration: 1/3
Overall Score: 0.62

Reasoning context ignored per M1 Context Isolation. Audit performed against spec.md / plan.md / acceptance.md / research.md / spec-compact.md only, plus targeted file/line verification of internal references against the live codebase.

---

## Must-Pass Results

- [PASS] **MP-1 REQ number consistency**: REQ-SYN2-001..005 are sequential, no gaps, no duplicates, consistent zero-padding (no padding actually — 3-digit `001` form throughout). Evidence: spec.md:158-162.
- [PASS] **MP-2 EARS format compliance**: All 5 REQs match a distinct EARS pattern. Ubiquitous (REQ-SYN2-001 "The Python sidecar SHALL", spec.md:158); Event-Driven (REQ-SYN2-002 "WHEN... THEN", spec.md:159); State-Driven (REQ-SYN2-003 "WHILE...", spec.md:160); Unwanted (REQ-SYN2-004 "IF... THEN", spec.md:161); Optional (REQ-SYN2-005 "WHERE...", spec.md:162). Each clause uses "SHALL" correctly and carries a triggering condition where required.
- [PASS] **MP-3 YAML frontmatter validity**: id (line 2), version (line 3), status (line 4), created (line 5), priority (line 8) — all present with correct types. `labels` field is absent but `milestone`, `owner`, `methodology`, `coverage_target`, `depends_on`, `blocks` are populated; the project's SPEC schema appears to favor these over `labels`. Acceptable per common project frontmatter usage.
- [N/A] **MP-4 Section 22 language neutrality**: SPEC scopes only the Python `services/researcher` sidecar + Go `internal/obs` collectors (Python + Go only). Single-language tooling neutrality is not at stake. N/A.

---

## Category Scores (0.0–1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.75 | 0.75 | Most REQs are unambiguous (REQ-SYN2-002/003/004 give exact failure semantics). Two minor ambiguities: REQ-SYN2-001 quotes an incomplete sentence-segmentation regex (spec.md:158); NFR-SYN2-001 conflates threshold statement with implementation test details (spec.md:168). |
| Completeness | 0.50 | 0.50 | All major sections present (HISTORY/Purpose/Scope/EARS/Acceptance/Out-of-Scope/References). However multiple cross-document inconsistencies exist around the `faithfulness_action` and `outcome` enum cardinality between spec.md §2.1(e), §2.1(f), REQ-SYN2-003, plan.md §3.5, spec-compact.md, and acceptance.md §4.5 (see Defects D2/D3). |
| Testability | 0.85 | 0.75 (band) | Acceptance scenarios are concrete with inputs/outputs/counter assertions/log payload assertions (acceptance.md:42-191). Edge cases enumerated 1–10. One scope leak: Edge Case 1 codifies behavior not in any REQ (D4). |
| Traceability | 0.80 | 0.75 (band) | Every REQ has at least one acceptance entry (acceptance.md:39-191). Every NFR has dedicated tests. plan.md §6 enumerates File Impact mapping. Internal file:line refs (research.md §7) verified against live code: synthesis.py:66-118 ✓ (`_process_markers`), synthesis.py:192 ✓ (call site), synthesis.py:208 ✓ (return), models.py:55-63 ✓ (Citation), Citation.doc_id at line 61 ✓. Minor gap: NFR-SYN2-001 sub-claim "≤ 100 ms vs SPEC-SYN-001 baseline" referenced in test but no recorded baseline value. |

Numeric average ≈ 0.725, but rubric bands cap relevant dimensions and Completeness is dragged down by cross-doc enum contradictions. Overall = 0.62.

---

## Defects Found

| Severity | Dimension | File:Section | Defect | Suggested Fix |
|----------|-----------|--------------|--------|---------------|
| MAJOR | Cross-document consistency | spec.md:36 (HISTORY) | HISTORY claims "4 EARS REQs (3 × P0 + 1 × P1)" but the document defines **5** REQs (REQ-SYN2-001..005, four P0 + one P1). spec.md:158-162 lists five rows. spec-compact.md:39 also says "EARS Requirements (5)". HISTORY is mathematically wrong. | Rewrite HISTORY line 36 to "5 EARS REQs (4 × P0 + 1 × P1) covering all five EARS patterns". |
| MAJOR | Cross-document consistency | spec.md:96 (§2.1 e) vs spec.md:160 (REQ-SYN2-003) vs spec-compact.md:113 | `faithfulness_action` enum mismatch. spec.md §2.1(e) defines `faithfulness_action: str ∈ {accepted, retry, stripped, rejected}` (4 values, no `off`). REQ-SYN2-003 (spec.md:160) explicitly mandates `faithfulness_action` SHALL be `"off"` in mode=off — referencing a value not in the §2.1(e) enum. spec-compact.md:113 lists 5 values including `off` and includes `retry` (which is itself absent from REQ-SYN2-002's enumeration of "accepted", "stripped", "rejected" outcome literals). The enum is undefined in any single source of truth. | Fix §2.1(e) to `∈ {accepted, retry_succeeded, retry_failed, stripped, rejected, off}` consistent with REQ semantics; remove ambiguous bare `retry`; align spec-compact.md and acceptance.md log-payload assertions. |
| MAJOR | Cross-document consistency / Observability | spec.md:97 (§2.1 f) vs plan.md:263 vs spec-compact.md:107 vs acceptance.md:383 | `usearch_synthesis_faithfulness_outcomes_total{outcome=...}` cardinality mismatch. spec.md §2.1(f) lists 5 outcome label values: `accepted, stripped, rejected, retry_succeeded, retry_failed`. plan.md §3.5 lists 6 values (adds `off`). spec-compact.md:107 lists 6 (with `off`). acceptance.md §4.5 line 383 lists 6 (with `off`). REQ-SYN2-003 (spec.md:160) actually states "the `usearch_synthesis_faithfulness_outcomes_total{outcome=...}` counter SHALL NOT increment" in mode=off — meaning `off` should NOT be a label value at all. Three of four documents contradict the REQ. | Pick one: either the counter never carries `outcome="off"` (delete it from plan.md:263, spec-compact.md:107, acceptance.md:383, and the cardinality calc "6 enum values × 1 metric") OR change REQ-SYN2-003 to allow incrementing `outcome="off"`. The current state has the metric definition contradicting the State-Driven REQ. |
| MAJOR | Scope creep / Acceptance ↔ REQ alignment | acceptance.md:196-213 (Edge Case 1) | Edge Case 1 prescribes a "degraded-style fallback with `degraded=true`, `notice="all sentences un-cited; returning raw doc list"`, `text` populated with bullet-list of input docs" when strip mode produces empty output. This behavior is **not in any REQ** of spec.md §3, and §2.1 does not list it as in-scope. plan.md §9 Q3 explicitly classifies it as an *open question* with default proposed (line 451-454: "Confirm in run-phase iteration 2"). acceptance.md treats it as already-decided binding behavior. The edge case also asserts a `usearch_synthesis_calls_total{outcome="degraded"}` increment that was a SPEC-SYN-001 concern, expanding the outcome accounting surface. Edge Case 7 (acceptance.md:281-291) compounds the same scope leak for empty post-`_process_markers` text. | Either (a) elevate the empty-output fallback to a new EARS REQ (e.g. REQ-SYN2-006 Optional / Unwanted) explicitly stating the degraded fallback path and its counter semantics, or (b) reduce Edge Case 1 / 7 to "behavior to be determined in run phase per plan.md §9 Q3" so it is not a binding gate. |
| MINOR | Clarity | spec.md:158 (REQ-SYN2-001) | Sentence-segmentation regex inside REQ-SYN2-001 reads `[.!?。！？]\s+` only, but the actual regex used everywhere else (spec.md:296, plan.md §3, research.md:247, acceptance.md:182) is `[.!?。！？]\s+|[.!?。！？]$` — including the end-of-text alternative. The REQ's truncated regex is testable but does not match the implementation pattern, which would mark the last sentence as un-segmented. | Replace REQ-SYN2-001's `[.!?。！？]\s+ or end-of-text` paraphrase with the literal regex `[.!?。！？]\s+|[.!?。！？]$` to match the canonical definition used downstream. |
| MINOR | Clarity / NFR style | spec.md:168 (NFR-SYN2-001) | NFR-SYN2-001 conflates the threshold ("p99 ≤ 50 ms gate, p95 ≤ 14 s end-to-end with retry") with implementation test details ("50 iterations on 12-sentence input; assert `durations_p99 ≤ 0.050s`"). Test specifications belong in acceptance.md §2/§4.4 (which already covers them at acceptance.md:370-376). The NFR statement should be method-agnostic. | Trim NFR-SYN2-001 to the three threshold sentences only; move the iteration count and assertion code to acceptance.md if not already duplicated there. |

---

## Chain-of-Verification Pass

Second-look re-read across each section (focus areas: REQ enumeration end-to-end, traceability for every REQ to AC, Exclusions specificity, contradictions across files):

1. Re-counted REQs in spec.md:158-162 → exactly 5 rows. HISTORY:36 says 4. Confirmed defect D1.
2. Re-mapped each REQ to acceptance.md sections:
   - REQ-SYN2-001 → acceptance.md:179-193 ✓
   - REQ-SYN2-002 → acceptance.md:195-213 ✓
   - REQ-SYN2-003 → acceptance.md:215-224 ✓
   - REQ-SYN2-004 → acceptance.md:226-240 ✓
   - REQ-SYN2-005 → acceptance.md:242-256 ✓
   No orphan REQs. No orphan ACs traceable to non-existent REQs.
3. Re-checked Exclusions §2.2 (spec.md:107-148) — 13 specific entries each with destination SPEC or rationale. Strong.
4. Searched for contradictions:
   - `faithfulness_action` enum: contradiction between §2.1(e) and REQ-SYN2-003 (D2 confirmed).
   - `outcome` label cardinality: contradiction between §2.1(f) and plan.md/spec-compact.md/acceptance.md and REQ-SYN2-003 (D3 confirmed).
   - HISTORY REQ count contradiction (D1 confirmed).
   - Edge Case 1 prescribing behavior absent from REQs (D4 confirmed).
5. Verified internal file:line references against live source:
   - `synthesis.py` is 220 lines; `_process_markers` at lines 66-118 ✓; call site `_process_markers(text_raw, req.docs)` at line 192 ✓; `SynthesizeResponse(...)` return starts at line 208 ✓.
   - `models.py` `Citation` class at lines 55-63 ✓; `doc_id` field at line 61 ✓.
   - Existing `@MX:ANCHOR` on `synthesize()` at synthesis.py:151 ✓ (matches plan.md §3.6 "UPDATE existing").
   No false claims found in the research grounding.
6. MX tag plan: plan.md §3.6 specifies @MX:ANCHOR + @MX:WARN both on `enforce_faithfulness()` (LLM-trust boundary). @MX:WARN reason cited per protocol mandatory-fields rule. Within per-file caps. Acceptable.

No new defects discovered in second pass. Existing four MAJOR + two MINOR confirmed.

---

## Regression Check

Iteration 1 — no prior report. N/A.

---

## Recommendation (FAIL — actionable fixes for manager-spec)

1. **Fix HISTORY arithmetic** (spec.md:36): change "4 EARS REQs (3 × P0 + 1 × P1)" to "5 EARS REQs (4 × P0 + 1 × P1)".
2. **Reconcile `faithfulness_action` enum** across documents (spec.md:96, spec.md:160, spec-compact.md:113, acceptance.md log-record assertions). Choose one canonical set: recommended `{accepted, retry_succeeded, retry_failed, stripped, rejected, off}` and update §2.1(e) to match. Replace every ambiguous bare `"retry"` literal in spec.md and spec-compact.md.
3. **Reconcile `outcome` label cardinality** (spec.md:97 vs plan.md:263 vs spec-compact.md:107 vs acceptance.md:383). Either (a) drop `off` from the metric label enumeration so REQ-SYN2-003 holds (preferred — keeps REQ-SYN2-003 truthful and the metric pure) and update plan.md §3.5 cardinality calc to "5 enum values × 1 metric = 5 series"; or (b) amend REQ-SYN2-003 to allow `outcome="off"` to increment.
4. **Resolve the empty-strip-output behavior** (acceptance.md Edge Case 1 + 7). Either (a) add a new REQ codifying the degraded fallback contract — recommended pattern: REQ-SYN2-006 (Unwanted) "IF mode=strip AND post-strip text is empty, THEN the service SHALL return SPEC-SYN-001 degraded-mode response with `notice='all sentences un-cited; returning raw doc list'`"; or (b) downgrade the edge case to "behavior deferred to run phase" so the empty-output policy is not gating acceptance.
5. **Replace truncated regex** in REQ-SYN2-001 (spec.md:158) with the canonical `[.!?。！？]\s+|[.!?。！？]$`.
6. **Trim NFR-SYN2-001** (spec.md:168) — keep thresholds only; move iteration counts and `durations[47] ≤ 14.0s` assertions to acceptance.md.

After these revisions, re-submit for iteration 2 audit. The structural EARS work, Exclusions discipline, and research grounding are strong; the failures are localized to enum/cardinality definitions and one out-of-scope acceptance edge case.

---

Report written to: `/Users/masterp/Projects/superwork/universal-search/.moai/reports/plan-audit/SPEC-SYN-002-review-1.md`
