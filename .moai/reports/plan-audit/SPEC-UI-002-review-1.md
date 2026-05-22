# SPEC Review Report: SPEC-UI-002
Iteration: 1/3
Verdict: FAIL
Overall Score: 0.72

Reasoning context ignored per M1 Context Isolation. The orchestrator passed audit hints (areas to focus on) which I treat as audit guidance, not author rationale. Findings below are derived solely from `spec.md`, `acceptance.md`, `plan.md`, and `spec-compact.md`.

## Must-Pass Results

- [PASS] MP-1 REQ number consistency: spec.md L135-L227. Per-module sequential numbering with consistent 3-digit zero-padding. adapter-status (REQ-AS-001/002/003), api-key-view (REQ-AK-001/002/003), audit-viewer (REQ-AV-001/002/003), localhost-guard (REQ-LH-001/002/003), navigation-integration (REQ-NV-001/002). No gaps, no duplicates inside each module.
- [PASS] MP-2 EARS format compliance: 13/14 requirements follow the labeled EARS pattern with explicit trigger/condition keywords (WHEN/WHILE/WHERE/IF). One mislabel detected (REQ-AK-003 — see D4 below) and two minor implicit-subject cases (see D5). Verdict held at PASS because pattern adherence is dominant; defects are flagged for revision.
- [FAIL] MP-3 YAML frontmatter validity: spec.md L1-L27. Required field `labels` is entirely absent. Required field `created_at` is named `created` (L5) — non-conformant to the required key name. Other required fields are present (id L2, version L3, status L4, priority L8). This is a hard MP-3 failure and triggers an overall FAIL regardless of other scores.
- [N/A] MP-4 Section 22 language neutrality: N/A — SPEC is scoped to a single-stack web admin surface (Next.js 16 / TypeScript front-end + Go backend `cmd/usearch-api`). No multi-language tooling enumeration is required.

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.75 | 0.75 band — minor ambiguity in REQ-LH-001 ("source IP" not pinned to RemoteAddr); REQ-AV-003 stacks two WHERE clauses in one requirement | spec.md L199-L202, L192-L195 |
| Completeness | 0.50 | 0.50 band — frontmatter missing `labels`; negative acceptance scenarios missing for invalid adapter ID on resync/toggle and 5xx backend failures even though plan.md prescribes those test cases | spec.md L1-L27; plan.md L78, L85; acceptance.md (absent) |
| Testability | 0.75 | 0.75 band — most ACs are binary-testable with concrete URLs, fixture counts, and status codes; a few use unmeasurable phrases ("정상 응답한다", AC AS-1.1 L17) and NFR-A11Y-001 mixes baseline reference with concrete attributes | acceptance.md L17, L142; spec.md L238-L240 |
| Traceability | 1.00 | 1.0 band — every REQ has ≥1 AC, every AC references a valid REQ. Cross-referenced 14 REQs → 17 scenarios. No orphans, no dangling references | acceptance.md L11-L186; spec-compact.md L42-L62 |

## Defects Found

D1. spec.md:L220 — Factual inconsistency: REQ-NV-001 states "SPEC-UI-001이 만든 **4개 기존 항목**(Search/History/Sources)" but only enumerates 3 items. acceptance.md NV-5.1 L166-L170 correctly states "(Search, History, Sources) **3개 항목**" and "정확히 4개 항목이 표시되며 마지막이 Admin 항목". The "4" in spec.md L220 is a typo for "3". This contradicts the acceptance criterion that downstream tests will assert against. Severity: **major**.

D2. spec.md:L1-L27 — YAML frontmatter missing the `labels` field entirely. MP-3 must-pass criterion violation. Severity: **critical** (hard FAIL trigger).

D3. spec.md:L5 — YAML frontmatter uses key name `created` instead of the required `created_at`. MP-3 requires the exact key. Severity: **major**.

D4. spec.md:L170-L172 — REQ-AK-003 is labeled "(Unwanted, P0)" but its body uses the Ubiquitous negative pattern ("the system shall NOT ... 렌더링하지 않아야 한다"). True Unwanted form is "IF [undesired condition], then the system shall [response]". Either re-label as Ubiquitous-negative or rewrite as "IF an admin UI component attempts to render a secret edit field, the system shall block render." Severity: **minor** (labeling).

D5. spec.md:L159-L164, L218-L222 — REQ-AK-001 and REQ-NV-001 use implicit subject ("표시해야 한다", "추가해야 한다") without an explicit "the system shall" subject. Acceptable Korean phrasing but EARS strictly requires an explicit subject. Severity: **minor**.

D6. acceptance.md (no entry) — Missing negative-path acceptance scenarios for invalid adapter IDs on the resync and toggle endpoints. plan.md:L78 prescribes "존재하지 않는 ID → 404" and plan.md:L85 prescribes "알 수 없는 ID 거부" as test cases, but acceptance.md has no Given/When/Then scenario covering these. /moai run will lack a SPEC-level acceptance to drive these tests; only the plan task description carries the expectation. Severity: **major** (gap between plan and acceptance contract).

D7. spec.md:L199-L202 — REQ-LH-001 says "source IP가 127.0.0.1 또는 ::1(loopback)이 아니면" without specifying that "source IP" must be derived from `RemoteAddr` and that `X-Forwarded-For` MUST be ignored. acceptance.md LH-4.3 (L144-L149) and plan.md L41, L182 both require RemoteAddr-only trust, but the normative requirement is silent. A naive implementer reading only the REQ could (incorrectly) honor `X-Forwarded-For: 127.0.0.1`, defeating the entire loopback gate. The acceptance test then asserts behavior not explicitly mandated by the requirement. Severity: **major** (security-critical normative gap).

D8. spec.md:L218-L222, L226-L228 — REQ-NV-001 and REQ-NV-002 hardcode implementation specifics: exact file path `web/src/components/sidebar-nav.tsx`, exact variable name `NAV_ITEMS`, exact Tailwind class strings `bg-accent text-accent-foreground`. Acceptable for a [DELTA] brownfield SPEC, but technically violates RQ-3 (requirements as behavior, not implementation). Consider extracting to plan.md only. Severity: **minor**.

D9. spec.md:L192-L195 — REQ-AV-003 stacks two WHERE branches in one requirement ("WHERE ... 지원하면 ... WHERE ... 지원하지 않으면 ... fallback"). EARS Optional pattern is "WHERE [feature exists], the system shall [response]" — single-branch. This should be split into REQ-AV-003 (cursor path) and REQ-AV-004 (limit/offset fallback, Ubiquitous). Severity: **minor**.

D10. acceptance.md (no entry) — No scenario covers backend 5xx failure on `POST /api/admin/adapters/{id}/toggle`, `POST /api/admin/adapters/{id}/resync`, or `GET /api/admin/audit/queries`. Only the 403 (loopback) and 200 (happy path) paths are exercised. LH-4.4 covers the network/403 fallback for the page load, but per-action failures (e.g., toggle POST returns 500) are unspecified. Severity: **minor**.

D11. acceptance.md:L17 — AC AS-1.1 uses phrase "정상 응답한다" without an explicit status code or body shape contract. Should be "응답 상태 200 with JSON payload conforming to {…schema reference}". Severity: **minor** (testability weasel).

D12. acceptance.md:L106-L111 — AV-3.3 "페이지네이션 컨트롤은 비활성 상태이거나 노출되지 않는다" admits two distinct UI behaviors (disabled vs absent). A binary tester needs one. Severity: **minor** (ambiguous AC).

D13. spec.md:L236-L237 — NFR-PERF-001 says "1초 이내" measured at "로컬 환경 기준" — "로컬 환경" is not pinned (machine spec, network, dataset size). For a performance NFR this is unmeasurable in practice. Severity: **minor**.

## Chain-of-Verification Pass

Second-look findings (re-read each module end-to-end):

- Re-verified REQ number sequencing module-by-module (not just spot-check): adapter-status 001/002/003, api-key-view 001/002/003, audit-viewer 001/002/003, localhost-guard 001/002/003, navigation-integration 001/002. Confirmed.
- Re-verified traceability for every single REQ (not sampled): all 14 REQs have ≥1 AC; all 17 ACs reference an existing REQ. Confirmed.
- Re-checked Exclusions section for specificity (not just presence): §2.2 L107-L124 enumerates 7 concrete prohibitions with explicit examples (`web/admin/`, recharts, `/admin/adapters/[id]/page.tsx`, etc.); §9 L317-L326 restates them as PR-reject criteria. Specificity is good.
- Re-checked for cross-requirement contradictions: REQ-LH-002 mandates 127.0.0.1 bind by default, while REQ-LH-001 enforces loopback via middleware — these are complementary defense layers, not contradictory. No internal contradictions found.
- New defect surfaced during second pass: D11 (testability of "정상 응답한다"), D12 (ambiguous either/or AC in AV-3.3), D13 (unmeasurable "로컬 환경" baseline in NFR-PERF-001). Added to defect list.
- Re-checked [DELTA] markers against actual file references: spec.md lists 5 [DELTA] targets (sidebar-nav.tsx, lib/api.ts, registry.go, audit/, cmd/usearch-api or internal/api/handlers); plan.md echoes them in Tasks A2/A3/B3/C1/C2. New files (admin/page.tsx, admin/_components/*, internal/api/admin/*) correctly omit [DELTA]. Markers are consistent and correct.

## Regression Check

Iteration 1 — N/A (first audit).

## Recommendation

This SPEC FAILS audit primarily on MP-3 (YAML frontmatter completeness) and on a substantive factual inconsistency (D1) plus a security-critical normative gap (D7). The bulk of the document is well-structured: EARS labeling is mostly correct, traceability is complete, exclusions are concrete, and the brownfield [DELTA] markers are accurate.

Required fixes before iteration 2 (in priority order):

1. **(D2, MP-3)** Add `labels` field to frontmatter (array form: `labels: [admin, ui, m7-surfaces, brownfield, security]` or similar). spec.md:L1-L27.
2. **(D3, MP-3)** Rename `created:` to `created_at:` at spec.md:L5.
3. **(D1, major)** Fix spec.md:L220 — change "4개 기존 항목(Search/History/Sources)" to "3개 기존 항목(Search/History/Sources)" so it agrees with acceptance.md NV-5.1.
4. **(D7, major)** Rewrite REQ-LH-001 (spec.md:L199-L202) to explicitly mandate that loopback detection MUST be performed against `RemoteAddr` only and that `X-Forwarded-For`, `X-Real-IP`, and `Forwarded` headers MUST be ignored by the admin middleware. Without this in the requirement text, the LH-4.3 security test asserts behavior not normatively required.
5. **(D6, major)** Add two new acceptance scenarios under Module 1/2:
   - `AS-1.4 — Re-sync on unknown adapter ID (REQ-AS-002) [negative]`: GIVEN no adapter `ghost` is registered, WHEN POST `/api/admin/adapters/ghost/resync`, THEN response is 404 and no other adapter is affected.
   - `AK-2.4 — Toggle on unknown adapter ID (REQ-AK-002) [negative]`: GIVEN no adapter `ghost` is registered, WHEN POST `/api/admin/adapters/ghost/toggle`, THEN response is 404.
6. **(D4, minor)** Either relabel REQ-AK-003 as "Ubiquitous (negative)" or restructure it into IF/THEN Unwanted form. spec.md:L170-L172.
7. **(D9, minor)** Split REQ-AV-003 into two requirements: one Optional for cursor support, one Ubiquitous for limit/offset baseline. spec.md:L192-L195.
8. **(D5, minor)** Add explicit "the system shall" subjects to REQ-AK-001 (L159) and REQ-NV-001 (L218).
9. **(D10, minor)** Add at least one acceptance scenario covering admin action 5xx failure (UI must surface an error toast/message without exposing stack trace).
10. **(D11, D12, D13, minor)** Tighten the weasel phrases: AS-1.1 "정상 응답한다" → "응답 코드 200 + 9 entry JSON"; AV-3.3 "비활성 상태이거나 노출되지 않는다" → choose one; NFR-PERF-001 "로컬 환경 기준" → pin a concrete baseline (e.g., "loopback fetch, fixture of 9 adapters + 50 audit rows, M1 MacBook or equivalent").

When the SPEC returns for iteration 2, this report's D1–D13 should each be addressable line-by-line. Defects D2, D3, D1, D7, D6 are blocking; the rest are quality improvements.
