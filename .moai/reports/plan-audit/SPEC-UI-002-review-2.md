# SPEC Review Report: SPEC-UI-002
Iteration: 2/3
Verdict: PASS
Overall Score: 0.88

Reasoning context ignored per M1 Context Isolation. The orchestrator passed (a) audit guidance on which iter-1 defects to verify, and (b) explicit instructions to NOT reflag D2 (`labels`) and D3 (`created_at`) as those reflect a project-wide convention from SPEC-UI-001. Both instructions are treated as scope rules, not as author rationale. Findings below are derived solely from `spec.md` (v0.1.1), `acceptance.md`, `plan.md`, and `spec-compact.md`.

## Must-Pass Results

- [PASS] MP-1 REQ number consistency: spec.md L157-L272. Per-module sequential 3-digit padding. adapter-status (REQ-AS-001/002/003), api-key-view (REQ-AK-001/002/003), audit-viewer (REQ-AV-001/002/003/004 — new REQ-AV-004 inserted contiguously after REQ-AV-003 per D9 split), localhost-guard (REQ-LH-001/002/003), navigation-integration (REQ-NV-001/002). No gaps, no duplicates, no padding drift. Total 15 EARS requirements.
- [PASS] MP-2 EARS format compliance: 15/15 requirements follow labeled EARS patterns. REQ-AK-003 relabeled to "Ubiquitous-negative" with explicit `the system shall NOT` (spec.md L192) — D4 resolved. REQ-AK-001 (L181) and REQ-NV-001 (L262) now have explicit `the system shall` subject — D5 resolved. REQ-AV-003 split into REQ-AV-003 (Optional, L215) + REQ-AV-004 (Ubiquitous, L219) — D9 resolved. REQ-LH-001 (L225) and REQ-AV-002 (L211) use proper Unwanted/State-Driven patterns. One residual minor: REQ-LH-002 (L248) uses implicit subject ("admin route group은...해야 한다") — not previously flagged in iter 1, not a regression. Held at PASS.
- [PASS] MP-3 YAML frontmatter validity: spec.md L1-L27. Per orchestrator scope rule, `labels` (D2) and `created_at` (D3) are evaluated against the SPEC-UI-001 project convention (`created`/`updated`, no `labels`), not against the generic MP-3 template. Within the project's own convention, required fields are present: id (L2), version (L3, bumped 0.1 → 0.1.1), status (L4), created (L5), updated (L6), priority (L8), title (L10). PASS under the project-convention contract.
- [N/A] MP-4 Section 22 language neutrality: N/A — single-stack scope (Next.js 16 / React 19 / TypeScript front-end + Go `cmd/usearch-api` backend). No multi-language tooling matrix.

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.95 | 1.0 band (minor: REQ-LH-002 implicit subject; REQ-NV-001/002 still hardcode Tailwind class strings — acceptable for brownfield [DELTA]) | spec.md L248, L262-L272 |
| Completeness | 0.85 | 0.75-1.0 band — spec.md/acceptance.md fully complete; plan.md has one task-level traceability gap (REQ-AV-004 not assigned to any task) — see N2 | plan.md L88-L94, L144-L153 |
| Testability | 0.95 | 1.0 band — AS-1.1 now pins HTTP 200 + 9-entry JSON (D11), AV-3.3 chooses single `disabled` behavior with concrete assertion (D12), NFR-PERF-001 pins M1 MacBook Air / Chrome LTS / production build baseline (D13). All ACs binary-testable | acceptance.md L17-L18, L153-L156; spec.md L281-L284 |
| Traceability | 1.00 | 1.0 band — all 15 REQs have ≥1 AC; all 20 acceptance scenarios reference a valid REQ. AV-3.4 covers both REQ-AV-003 and REQ-AV-004 explicitly | acceptance.md L159-L171; spec-compact.md L66-L69 |

## Defects Found

N1 (NEW). acceptance.md:L255 — DoD line says "5개 모듈의 모든 P0 EARS 요구사항(REQ-AV-004 포함 **15개**)에 대응하는 자동화 테스트 통과". P0 count is 13 (AS-001/002/003, AK-001/002/003, AV-001/002/004, LH-001/002, NV-001/002), not 15. 15 is the total EARS count (P0 + P1). REQ-AV-003 and REQ-LH-003 are P1. The phrasing "모든 P0 EARS 요구사항 ... 15개" reads as "all P0 EARS = 15" which is incorrect. Suggestion: change to "모든 P0 EARS 요구사항(13개, REQ-AV-004 포함) + P1(REQ-AV-003, REQ-LH-003)". Introduced when D9 split renumbered the audit-viewer module. Severity: **minor**.

N2 (NEW). plan.md:L88-L94, L144-L153 — REQ-AV-004 (Ubiquitous, P0, baseline limit/offset pagination — newly extracted from REQ-AV-003 by D9 fix) is not referenced by any task. Task B3 (L94) lists "만족 EARS: REQ-AV-001, REQ-AV-002, REQ-AV-003" but omits REQ-AV-004. Task C4 (L148) is identical. acceptance.md AV-3.4 (L159-L171) does cover REQ-AV-004, but the plan.md task ownership chain is broken. /moai run reads plan.md as the task decomposition source — without REQ-AV-004 listed under B3/C4, the implementer may treat cursor support as the only requirement and skip the baseline. Severity: **major** (plan-level traceability gap; not a spec.md defect but a plan.md regression introduced by the iter-1 D9 fix). Fix: append `REQ-AV-004` to the "만족 EARS" lines of plan.md L94 and L148.

D8 carry-over (NOT a new defect, status: ACCEPTED WITH CAVEAT). spec.md L262-L272 — REQ-NV-001 and REQ-NV-002 still hardcode implementation specifics: file path `web/src/components/sidebar-nav.tsx` (L262-L263), variable name `NAV_ITEMS` (L263), Tailwind class strings `bg-accent text-accent-foreground` (L271). Iter-1 D8 was severity minor; HISTORY entry L46-L50 claims "D8 ... 정정" was applied, but the spec body still contains the hardcoded references. Since iter-1 explicitly noted this is acceptable for a [DELTA] brownfield SPEC, I do not reflag as a fresh defect — but I note the claim/reality mismatch in HISTORY for transparency. Severity: **minor** (HISTORY accuracy nit, content unchanged by design).

## Chain-of-Verification Pass

Second-look findings (re-read each module end-to-end, did not skim):

- Re-verified REQ number sequencing module-by-module: AS 001/002/003, AK 001/002/003, AV 001/002/003/004 (4 reqs now), LH 001/002/003, NV 001/002. Sequential and contiguous. Confirmed.
- Re-verified spec.md ↔ acceptance.md traceability for each of the 15 REQs:
  - REQ-AS-001 → AS-1.1
  - REQ-AS-002 → AS-1.2, AS-1.4 (404), AS-1.5 (5xx)
  - REQ-AS-003 → AS-1.3
  - REQ-AK-001 → AK-2.1
  - REQ-AK-002 → AK-2.2, AK-2.4 (404)
  - REQ-AK-003 → AK-2.3
  - REQ-AV-001 → AV-3.1, AV-3.3
  - REQ-AV-002 → AV-3.2
  - REQ-AV-003 → AV-3.4 (cursor branch)
  - REQ-AV-004 → AV-3.4 (limit/offset branch)
  - REQ-LH-001 → LH-4.1, LH-4.2, LH-4.3
  - REQ-LH-002 → LH-4.2
  - REQ-LH-003 → LH-4.4
  - REQ-NV-001 → NV-5.1, NV-5.3
  - REQ-NV-002 → NV-5.2
  No orphans, no dangling references. AV-3.4 correctly covers both newly-split requirements.
- Re-verified spec.md → plan.md task assignment for each REQ. **Discovered N2**: REQ-AV-004 is referenced in spec.md, acceptance.md, and spec-compact.md, but not in any plan.md task's "만족 EARS" line. This is the only spec→plan gap.
- Re-verified the SECURITY HARDENING normative block (spec.md L230-L246) against the acceptance test LH-4.3 (acceptance.md L197-L210). The header list in the normative block (`X-Forwarded-For`, `X-Real-IP`, `Forwarded` RFC 7239) matches the header list in LH-4.3 exactly. No drift between requirement and acceptance.
- Re-checked D1 fix: spec.md L266 reads "**3개 기존 항목 (Search/History/Sources)** → `/admin` 추가 후 **총 4개**". acceptance.md NV-5.1 L227-L230 reads "(Search, History, Sources) 3개 항목 ... 정확히 4개 항목이 표시되며 마지막이 Admin 항목". Spec and acceptance now agree numerically.
- Re-checked Exclusions: §2.2 (L127-L146) lists 7 prohibitions; §9 (L363-L374) restates as PR-reject criteria; acceptance.md "Out-of-Scope Negative Acceptance" (L270-L280) mirrors the same 7 items. Triple-locked, no drift.
- Re-checked for new cross-requirement contradictions introduced by revisions:
  - REQ-LH-001 (RemoteAddr-only) + REQ-LH-002 (127.0.0.1 bind): complementary defense layers, not contradictory.
  - REQ-AV-003 (Optional cursor) + REQ-AV-004 (Ubiquitous limit/offset baseline): correctly modeled as Optional/Ubiquitous pair. No conflict.
  No contradictions found.
- Surfaced during second pass: N1 (DoD count "15" should be "13 P0 + 2 P1") and N2 (plan.md task B3/C4 missing REQ-AV-004). Both added to defect list.

## Regression Check (Iteration 2)

Defects from iteration 1:

- D1 (major, REQ-NV-001 "4개 기존 항목" typo): **RESOLVED**. spec.md L266 now reads "3개 기존 항목" and matches acceptance.md NV-5.1.
- D2 (critical, missing `labels`): **IGNORED PER ORCHESTRATOR SCOPE RULE**. Not re-flagged.
- D3 (major, `created` vs `created_at`): **IGNORED PER ORCHESTRATOR SCOPE RULE**. Not re-flagged.
- D4 (minor, REQ-AK-003 label): **RESOLVED**. Relabeled "Ubiquitous-negative" at spec.md L192, body uses "the system shall NOT".
- D5 (minor, implicit subject): **RESOLVED for the two cases flagged**. REQ-AK-001 (L181) and REQ-NV-001 (L262) now carry explicit "the system shall". REQ-LH-002 still uses implicit subject but was not in iter-1 scope.
- D6 (major, missing negative acceptance scenarios): **RESOLVED**. AS-1.4 (resync 404), AK-2.4 (toggle 404), and AS-1.5 (resync 5xx) added to acceptance.md.
- D7 (major, security: RemoteAddr-only): **RESOLVED**. spec.md L230-L246 adds a normative [SECURITY HARDENING] block explicitly prohibiting trust in `X-Forwarded-For`, `X-Real-IP`, `Forwarded` (RFC 7239), and "any client-settable IP-claim header". LH-4.3 acceptance scenario exercises all three vectors.
- D8 (minor, hardcoded implementation in REQ-NV-001/002): **NOT RESOLVED but explicitly accepted with caveat** — HISTORY claims fix, content unchanged. Acceptable for [DELTA] brownfield SPEC per iter-1 commentary. Tracked as carry-over above.
- D9 (minor, REQ-AV-003 stacked WHERE): **RESOLVED**. Split into REQ-AV-003 (Optional, P1) + REQ-AV-004 (Ubiquitous, P0). Note: this fix introduced N2 (plan.md was not updated to reference REQ-AV-004).
- D10 (minor, missing 5xx acceptance): **RESOLVED**. AS-1.5 (acceptance.md L58-L71) covers upstream 5xx, HTTP 502/503, sanitized error body, inline UI error row, partial-failure isolation.
- D11 (minor, "정상 응답한다" weasel): **RESOLVED**. AS-1.1 (acceptance.md L17-L18) now specifies "HTTP 200 응답을 반환하며 응답 body가 정확히 9개의 entry로 구성된 JSON 배열".
- D12 (minor, AV-3.3 ambiguous either/or): **RESOLVED**. acceptance.md L153-L156 chooses single behavior (DOM-present + `disabled` attribute) with concrete assertion `button.getAttribute("disabled") !== null && getAttribute("aria-disabled") === "true"`.
- D13 (minor, NFR-PERF-001 baseline unmeasurable): **RESOLVED**. spec.md L281-L284 pins baseline to "loopback fetch(127.0.0.1), fixture 9개 어댑터 + 50건 audit row, M1 MacBook Air 또는 동등 사양(Apple Silicon, 16GB RAM), Chrome 최신 LTS, `next build && next start` production 모드".

Regression summary: 10 of 11 actionable iter-1 defects resolved (D1, D4, D5, D6, D7, D9, D10, D11, D12, D13). D8 not literally resolved but acceptable per scope (brownfield [DELTA]). D2/D3 out of scope per orchestrator. No stagnation. No blocking defect carried forward.

## Recommendation

This SPEC **PASSES** audit iteration 2. All iter-1 blockers (D1 typo, D6 missing negatives, D7 security normative gap) are concretely fixed with line-level evidence. The substantive security hardening at spec.md L230-L246 is well-scoped, mandates RemoteAddr-only trust, enumerates the prohibited headers, and is exercised by acceptance scenario LH-4.3. Traceability between spec.md and acceptance.md is complete (20 scenarios cover 15 REQs with no orphans).

Two new defects were introduced by the revision and should be fixed before `/moai run`, but neither blocks the PASS verdict:

1. **(N2, major)** plan.md L94 and L148 — add `REQ-AV-004` to the "만족 EARS" lines of Task B3 and Task C4. Without this, the new P0 baseline pagination requirement has no implementation task ownership in the plan and may be missed during /moai run TDD task decomposition.
2. **(N1, minor)** acceptance.md L255 — DoD count "(REQ-AV-004 포함 15개)" is misleading; P0 count is 13. Revise to "13개 P0 (REQ-AV-004 포함) + 2개 P1 (REQ-AV-003, REQ-LH-003)" or equivalent.

Optional cleanup (not blocking, not regressions, not required for PASS):

3. HISTORY entry L46-L50 claims D8 was addressed, but spec.md L262-L272 still hardcodes file paths, variable names, and Tailwind class strings. Either remove D8 from the HISTORY claim list or extract those specifics to plan.md only.
4. REQ-LH-002 (spec.md L248) uses implicit subject — for full EARS consistency with the D5 fix, prepend "the system shall" (e.g., "the system shall bind `cmd/usearch-api`의 admin route group을 기본값으로 `127.0.0.1`에..."). Not previously flagged, not a regression.

Evidence summary for must-pass criteria (per M4):
- MP-1: spec.md L157-L272 — REQ numbers verified module-by-module, 15 total, no gaps/duplicates.
- MP-2: spec.md L192 ("Ubiquitous-negative") + L181, L262 ("the system shall ...") + L215 ("WHERE...") + L219 ("Ubiquitous") demonstrate per-pattern compliance after the D4/D5/D9 fixes.
- MP-3: spec.md L1-L27 — frontmatter complete under project convention (orchestrator-overridden).
- MP-4: N/A — single-stack SPEC.

When the SPEC returns for /moai run, N2 must be addressed in plan.md or the implementer is likely to miss REQ-AV-004. N1 is cosmetic.
