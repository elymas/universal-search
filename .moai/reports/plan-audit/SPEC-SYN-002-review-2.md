# SPEC-SYN-002 Plan Audit Report — Iteration 2

Verdict: PASS

Reasoning context ignored per M1 Context Isolation. This audit consumed only the five SPEC artifacts under `.moai/specs/SPEC-SYN-002/` (spec.md, plan.md, acceptance.md, spec-compact.md, research.md) and the iteration-1 review report.

---

## Iteration-1 Defect Resolution Table

| Defect ID | Description | Status | Evidence |
|-----------|-------------|--------|----------|
| MAJOR-1 | HISTORY count must say "5 EARS REQs" (was "4") | RESOLVED | `spec.md:36` — "5 EARS REQs (4 × P0 + 1 × P1) covering all five EARS patterns" |
| MAJOR-2 | `faithfulness_action` enum consistent across 4 docs (must include `off`) | RESOLVED | `spec.md:96` — `{accepted, stripped, rejected, off}` (4 values); `plan.md:534-538` D3 — Canonical 4-value set: `{accepted, stripped, rejected, off}`; `spec-compact.md:114-115` — `{accepted, stripped, rejected, off}` (4 values); `acceptance.md` Scenarios 1/2/3/4/5 use only the canonical 4-value set |
| MAJOR-3 | `outcome` metric label cardinality identical across 4 docs (5 labels, mode=off does NOT emit) | RESOLVED | `spec.md:97` — 5 values `{accepted, stripped, rejected, retry_succeeded, retry_failed}`; `plan.md:263-265` — 5 values + bypass note; `spec-compact.md:107-109` — 5 values + bypass note; `acceptance.md:393-394` — `5 enum values: accepted, stripped, rejected, retry_succeeded, retry_failed; mode=off bypasses the counter entirely per REQ-SYN2-003` |
| MAJOR-4 | plan.md §9 Q3 has a decision; acceptance.md Edge Cases 1/7 align | RESOLVED | `plan.md:452-454` — Q3 marked "RESOLVED — see §10 Decisions D1"; `plan.md:469-498` — D1 specifies HTTP 422 for empty post-strip; `acceptance.md:206-218` Edge Case 1 — explicit reference to "plan.md §10 D1 (resolves §9 Q3)" with HTTP 422 + `outcome="rejected"` counter; `acceptance.md:286-301` Edge Case 7 — same resolution pattern |
| MINOR-1 | REQ-SYN2-001 sentence-split regex/pattern fully specified | RESOLVED | `spec.md:158` — "segmented by the canonical regex pattern `[.!?。！？]\\s+\|[.!?。！？]$` (English + Korean punctuation, terminal-position alternative included)" |
| MINOR-2 | NFR-SYN2-001 threshold + delegation to acceptance.md §4.4 | RESOLVED | `spec.md:168` — explicit p99 ≤ 50 ms gate, p95 ≤ 14 s end-to-end with retry, plus "Detailed test method (iteration counts, percentile assertions) is specified in `acceptance.md` §4.4."; `acceptance.md:379-385` provides the 50-iteration benchmark spec |

All six iteration-1 defects RESOLVED. No regressions introduced.

---

## New Defects Found

| Severity | Dimension | File:Section | Defect | Suggested Fix |
|----------|-----------|--------------|--------|---------------|
| MINOR | Cross-doc consistency (§9 ↔ §10 ↔ acceptance §5) | `plan.md:443-446` (§9 Q1) and `acceptance.md:166-169` (Scenario 5 parenthetical) | `plan.md` §9 Q1 still phrased as an unresolved question ("decision deferred to run-phase iteration 1") even though `plan.md` §10 D3 and `spec.md` REQ-SYN2-003 both normatively require `faithfulness_action: "off"` in mode=off. `acceptance.md` Scenario 5 propagates the false uncertainty by saying "(or attribute omitted entirely; decision deferred to run phase per plan.md §9)" — directly contradicting REQ-SYN2-003's SHALL clause but reading as test-time ambiguity. | Either close `plan.md` §9 Q1 by pointing to §10 D3 (similar to the §9 Q3 → D1 cross-link already used), and remove the parenthetical from `acceptance.md` Scenario 5. The normative answer is unambiguous: include `faithfulness_action: "off"`. |
| MINOR | Cross-doc consistency (spec.md §2.1(a) ↔ plan.md §3.2) | `spec.md:92` and `plan.md:183-188` | `spec.md` §2.1(a) declares `EnforcementOutcome` enum has four values: `(accepted, retry_required, stripped, rejected)`. `plan.md` §3.2 declares the same enum with five values, adding `OFF = "off"`. Per `plan.md` §3.3 line 212, `synthesize()` short-circuits before invoking `enforce_faithfulness()` when `mode == "off"`, so the `OFF` enum value is never returned (vestigial). | Drop `OFF` from the enum in `plan.md` §3.2 to align with `spec.md` §2.1(a) and the actual call sites in §3.3. The boolean log attribute `faithfulness_action == "off"` is the canonical mode-off signal — no enum value is needed. |

---

## Other Audit Dimensions Re-checked

| Dimension | Result | Evidence |
|-----------|--------|----------|
| EARS compliance | PASS | All 5 REQs cleanly map to one of the five EARS patterns (Ubiquitous/Event-Driven/State-Driven/Unwanted/Optional). `spec.md:158-162` table lists the pattern label per REQ; reviewed REQ text confirms structural conformance. |
| Acceptance testability | PASS | Every REQ has named pytest cases; `acceptance.md` §2 contains 6 G/W/T scenarios; §3 contains 10 Edge Cases; §4 contains 7 quality-gate sub-checklists. Each line item is binary-testable. No weasel words ("appropriate", "reasonable") in the normative scenarios. |
| Exclusions specificity | PASS | `spec.md:107-148` has 13 specific Out-of-Scope bullets each pointing to a destination SPEC (SPEC-EVAL-001, SPEC-FAN-001, SPEC-SYN-004, etc.) or a FROZEN constraint. |
| Cross-document consistency | PARTIAL | All MAJOR-grade inconsistencies from iteration 1 are resolved; two MINOR residual inconsistencies remain (above table). |
| Delta markers ([NEW]/[MODIFY]/[EXISTING]) | PASS | `spec.md:91-105` table uses `[NEW]`, `[MODIFY]`, `[EXISTING — UNCHANGED]` per item. `plan.md:344-376` File Impact tables split into NEW/MODIFY/UNCHANGED. |
| MX plan | PASS | `plan.md:272-285` §3.6 specifies 6 concrete MX targets with tag types (`@MX:ANCHOR`, `@MX:WARN` with REASON, `@MX:NOTE`) and rationales aligned with `mx-tag-protocol.md` rules (fan_in ≥ 3, LLM-trust boundary). |
| Research grounding | PASS | `spec.md:319-338` cites 10 specific internal file:line references (e.g., `synthesis.py:66-118`, `synthesis.py:192`, `models.py:55-63`, `roadmap.md:64`); 5 external WebFetch-verified sources with dates. |
| Scope creep | PASS | All non-trivial additions trace to a numbered REQ or §2.1 in-scope item. No drive-by changes. SPEC-SYN-001 invariants preserved per REQ-SYN2-001 + acceptance §4.3 regression check. |
| Failure modes | PASS | `acceptance.md` Edge Cases 1, 7, 10 cover empty-output, empty-post-`_process_markers`, and LiteLLM `ConnectError` during retry. `spec.md` §6 + `research.md` §5 list named risks. |

---

## Chain-of-Verification Pass

Second-look findings:

- Re-read all five REQ texts (spec.md:158-162) end-to-end. All conform to claimed EARS patterns; no informal language smuggled in.
- Re-counted `outcome` label values across all 4 docs (spec/plan/spec-compact/acceptance §4.5): 5 in all four. No drift.
- Re-counted `faithfulness_action` enum across all 4 docs: 4 in all four (`accepted, stripped, rejected, off`); however the parenthetical in acceptance Scenario 5 introduces the residual ambiguity captured in MINOR #1.
- Verified plan.md §9 Q3 cross-link to §10 D1 is functional; verified §9 Q1 has no equivalent cross-link to §10 D3 — surfaced as MINOR #1.
- Verified `EnforcementOutcome` enum values across spec.md and plan.md — discovered 4-vs-5 vestigial inconsistency surfaced as MINOR #2.
- Confirmed `_process_markers` line citation (`synthesis.py:192`) is consistent across spec/plan/research.
- Confirmed REQ-SYN2-002 retry success path (`outcome="retry_succeeded"`) and retry failure path (`outcome="retry_failed"`) both present in acceptance scenarios 4 and 2 respectively. No metric label leak.
- Confirmed `cardinality allowlist unchanged` claim — `outcome` label name is reused; only new label values added, which `SPEC-OBS-001` discipline allows per `spec.md:144-147`.

No additional defects beyond the two MINOR items listed above.

---

## Rationale

All four MAJOR and both MINOR defects from iteration 1 are fully resolved with verifiable evidence in the four primary documents. The author propagated each fix systematically:

- HISTORY count corrected to "5 EARS REQs".
- `faithfulness_action` enum normalized to the canonical 4-value set including `off`, with `plan.md` §10 D3 establishing the resolution rationale and propagation map.
- `outcome` metric label normalized to 5 values across all four documents, with consistent "mode=off bypasses the counter" notes in spec.md §2.1(f), plan.md §3.5 + §10 D2, spec-compact.md, and acceptance.md §4.5.
- plan.md §9 Q3 (empty-strip output) closed via §10 D1, propagated to acceptance.md Edge Cases 1 and 7 with explicit cross-references to plan.md §10 D1.
- REQ-SYN2-001 now specifies the canonical regex `[.!?。！？]\s+|[.!?。！？]$` directly inline.
- NFR-SYN2-001 contains the p99 ≤ 50 ms gate threshold and explicitly delegates test methodology to acceptance.md §4.4.

Two new MINOR residuals were identified — both are editorial cleanup items that do not undermine implementation feasibility or testability:

1. plan.md §9 Q1 should be closed with a cross-link to §10 D3 (mirror of §9 Q3 → §10 D1 pattern), and the matching parenthetical in acceptance.md Scenario 5 should be removed. The normative answer is already binding via REQ-SYN2-003 SHALL clause.
2. plan.md §3.2 EnforcementOutcome enum should drop the vestigial OFF value to match spec.md §2.1(a) and the actual call site in §3.3.

Per the user's PASS criteria (0 BLOCKER + 0 MAJOR; up to 5 MINOR allowed), iteration 2 satisfies the bar. Both MINOR items can be addressed during run-phase iteration 1 cleanup or in a tail-end annotation cycle without blocking implementation entry. The SPEC is implementation-ready.

---

*End of SPEC-SYN-002 plan-auditor review-2*
