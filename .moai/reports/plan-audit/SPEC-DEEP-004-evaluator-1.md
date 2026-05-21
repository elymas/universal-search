# SPEC-DEEP-004 Cross-Validation Report

**Evaluator**: evaluator-active (independent skeptical quality assessment)
**Iteration**: 1 (post-v0.1.1 amendment)
**Date**: 2026-05-21
**Harness**: thorough (cross_validate_with_evaluator_active: true)

---

Cross-validation Verdict: **AGREE-WITH-PASS**

Final recommendation: **AGREE-PASS-WITH-PATCH** (2 new minor findings; neither overrides PASS on must-pass criteria)

---

## 1. Context Anchors Verified

| Anchor | Claim in SPEC | Verified Result |
|--------|--------------|-----------------|
| roadmap.md M5 SPEC-DEEP-004 row | "per-user per-day cap, Haiku pre-screen, prompt-cache reuse" | All 3 axes covered by REQs. owner: expert-backend matches. |
| product.md §6 cost metric | "LLM cost per /deep query ≤ $0.50" | research §1.2: worst-case $0.19, average $0.07 — both within $0.50. SPEC provides daily aggregate cap, not per-query cap. Coherent: $5/day tenant cap handles ~26 worst-case calls which self-bounds per-query cost. |
| SPEC-DEEP-002 frontmatter shape | sibling M5 SPEC convention | key order matches exactly (id, version, status, created, updated, author, priority, issue_number, title, milestone, owner, methodology, coverage_target, depends_on, blocks). |
| SPEC-LLM-001 cost-tracking lineage | REQ-DEEP4-012 references llm.Client | LLM-001 is `status: implemented`, confirms `internal/llm.Client` is the single Go-side choke point. DEEP-004's ledger write hooks this correctly. |
| SPEC-OBS-001 metric naming convention | NFR-DEEP4-009 "usearch_deep_*" pattern | OBS-001 is `status: implemented`. The `usearch_<domain>_<noun>_<unit>` pattern is referenced in NFR-009 as the binding convention. All newmetrics in research §8.1 follow this pattern. |
| SPEC-IR-001 status | §6.1 says "(implemented)" | IR-001 frontmatter: `status: implemented` (commit 8a20b68). CONFIRMED — this was the D8 patch. |

---

## 2. Plan-Auditor Defect Disposition (D1..D12)

| Defect | Severity (original) | Disposition | Evidence |
|--------|--------------------|-----------  |----------|
| D1 | MAJOR | CONFIRMED-RESOLVED-IN-V011 | REQ-DEEP4-010 now uses "decision event log (stderr-emitted JSON line)"; NFR-DEEP4-006 retitled "Ledger row durability" and explicitly states the durability commitment applies to Postgres rows only, not stderr lines. acceptance.md §5.1 also uses updated terminology. spec-compact.md mirrors. |
| D2 | MAJOR | CONFIRMED-RESOLVED-IN-V011 | §4 Exclusions now has a 10th item "Degraded-path 비용 제한 부재" with explicit intent statement ("이는 의도된 design choice이며 abuse vector가 아니다") and the cost-asymmetry rationale ($0.002 /basic vs $0.05-$0.20 /deep ~1/30). spec-compact.md mirrors the 10th exclusion. |
| D3 | MINOR | CONFIRMED-RESOLVED-IN-V011 | REQ-DEEP4-003 pattern label is now "Event-Driven"; lead-in rewritten as "WHEN /deep 요청이 cap-check를 통과한 직후". |
| D4 | MINOR | CONFIRMED-RESOLVED-IN-V011 | REQ-DEEP4-014 now specifies: health probe at `costguard.redis.health_check_interval_ms` (default 5000ms) using PING or trivial EXISTS, 3 consecutive successes trigger 1 RehydrateWindow job. spec-compact.md mirrors. |
| D5 | MINOR | CONFIRMED-RESOLVED-IN-V011 | NFR-DEEP4-002 now contains two explicit sub-budgets: (a) success path Redis INCR + Asynq enqueue p95 ≤ 50ms, (b) fail-closed path (3x exponential backoff 250ms+500ms+1000ms) p99 ≤ 2000ms. Postgres write-behind explicitly excluded from both budgets. |
| D6 | MINOR | CONFIRMED-RESOLVED-IN-V011 | REQ-DEEP4-012 now specifies the cache-key salt via "LiteLLM's custom cache-key callback (or cost guard wrapper layer)" producing `SHA256(tenant_id ‖ intent_category ‖ model ‖ messages_json)` and explicitly states: "messages 페이로드 자체를 변경하지 SHALL NOT 한다 — LLM은 salt 값을 인지하지 못하고 prompt completion이 오염되지 않는다." plan.md Phase D test `TestCacheKeyPrefixIncludesTenantAndIntent` updated to match. |
| D7 | MINOR | CONFIRMED-RESOLVED-IN-V011 | NFR-DEEP4-009 now reads "SPEC-DEEP-001이 소유하고 SPEC-DEEP-002 REQ-DEEP2-008이 outcome label을 확장한 기존 usearch_deep_outcomes_total". spec-compact.md: "owned by SPEC-DEEP-001 and extended by SPEC-DEEP-002 REQ-DEEP2-008". |
| D8 | MINOR | CONFIRMED-RESOLVED-IN-V011 | §6.1 SPEC-IR-001 entry now reads "(implemented)". IR-001 frontmatter verified as `status: implemented`. |
| D9 | MINOR | CONFIRMED NOT RESOLVED (DEFERRED) | §6.1 SPEC-DEEP-003 entry still says "(draft) — tree exploration의 breadth/depth가 cap 차원 설계에 직접 반영(최악-경우 호출 횟수 추산)". No clarification added regarding design-input vs runtime-code dependency distinction. HISTORY entry documents deferral as "NIT / cosmetic". |
| D10 | MINOR | CONFIRMED-RESOLVED-IN-V011 | §1.3 is a new subsection "Haiku Pre-Screen vs SPEC-IR-001 Intent Router (Orthogonality)" with explicit execution sequence: cap-check → Haiku pre-screen → (proceed 시) IR-001 category 조회 → LLM 호출. §6.1 IR-001 entry also states the orthogonality directly. |
| D11 | MINOR | CONFIRMED-RESOLVED-IN-V011 | §6.3 now extends to cover REQ-DEEP4-010 decision event log JSON line schema: mandatory fields (timestamp, event_type, request_id, tenant_id, user_id, decision) are SHALL-stated; schema is additive; AUTH-003 MUST NOT rename/remove listed fields. plan.md Phase F test `TestCapExceededIncrementsCounterAndDecisionLog` already verifies these fields. |
| D12 | NIT | CONFIRMED NOT RESOLVED (DEFERRED) | REQ-DEEP4-002 and REQ-DEEP4-005 still have no main GWT scenarios. acceptance.md footnotes (lines 343-348) retain the unit-test-only coverage note. This matches plan-auditor's recommendation to "leave as-is". |

**Summary**: 10 of 12 defects resolved in v0.1.1. D9 and D12 explicitly deferred per HISTORY, consistent with plan-auditor's "optional cosmetic" classification.

---

## 3. Independent Findings (Defects Plan-Auditor Missed)

### NF-1 [MINOR] — Footer version string not updated to v0.1.1

**Location**: spec.md line 416

The document footer reads `*End of SPEC-DEEP-004 v0.1.0 (draft)*` even though the HISTORY section declares this is v0.1.1. Every other version reference (frontmatter `version: 0.1.1`, HISTORY bullets, spec-compact.md first line) correctly shows v0.1.1. This creates a contradiction visible on the last line of the document.

**Recommended fix**: Update footer to `*End of SPEC-DEEP-004 v0.1.1 (draft)*`.

---

### NF-2 [MINOR] — Degrade decision event log asserted in acceptance but not mandated by any REQ

**Location**: acceptance.md §5.6; REQ-DEEP4-010; REQ-DEEP4-011

acceptance.md §5.6 (degraded-path scenario) includes a decision event log sample:
```json
{"event_type":"cap.evaluation","decision":"degrade",...}
```

REQ-DEEP4-010 mandates decision event log emission only for the cap_exceeded (deny) case: "decision event log에 decision='deny' event를 SHALL 출력한다". REQ-DEEP4-011 (degraded path) mandates HTTP 200 response, `X-Deep-Degraded` header, ledger row `outcome="degraded"`, and counter increment — but does NOT mandate a decision event log entry.

research.md §8.3 states "JSON line per cap event(allow/deny/degrade)를 stderr로 출력" (all three event types), but this is research context, not a binding REQ. The SPEC's binding REQs only mandate the deny case.

The consequence: an implementer reading only spec.md and acceptance.md faces a contradictory signal — the acceptance scenario shows a degrade decision log entry, but no REQ requires it. The test `TestCapExceededIncrementsCounterAndDecisionLog` in plan.md Phase F covers the deny case only.

**Recommended fix**: Either (a) add to REQ-DEEP4-011 a SHALL clause mandating decision event log emission for "degrade" decisions (consistent with research intent), or (b) remove the decision event log sample from acceptance.md §5.6 to prevent implementer confusion. Option (a) is preferred for observability completeness.

---

### NF-3 [NIT] — research.md §5.2 migration filename differs from spec.md

**Location**: research.md §5.2; spec.md §7.1; REQ-DEEP4-006

research.md §5.2 specifies path `deploy/postgres/migrations/0002_create_cost_ledger.sql` while spec.md §7.1 [NEW] and REQ-DEEP4-006 specify `deploy/postgres/migrations/0002_cost_ledger.sql` (without `create_`). research.md is a Phase 0.5 reference artifact and does not require update, but the discrepancy could confuse someone cross-referencing both documents.

**Note**: plan-auditor already validated the spec.md path as canonical (report line 98-99). The NIT only applies to research.md being a frozen reference with a stale filename. No fix required in spec.md.

---

## 4. Dimension Scores

### 4.1 Functionality (40%)

**Score: 0.88 / 1.0 — PASS**

Rubric anchor: 0.75 band = all stated scope REQs present with named tests; 1.0 band = complete scope + no over-scope + concrete cost-coherence with product.md.

Evidence:
- Roadmap M5 scope line "per-user per-day cap, Haiku pre-screen, prompt-cache reuse" is covered by: REQ-001/002/009 (cap), REQ-003/004/005 (Haiku screen), REQ-012/013 (cache reuse). No scope gap.
- Supporting infrastructure (cost ledger REQ-006-008, cap enforcement REQ-010-011, Redis failure REQ-014) is a natural and necessary extension of the stated scope, not over-scope.
- product.md §6 cost metric "≤ $0.50 per /deep" is coherent with SPEC's design: research §1.2 shows max $0.19 per call, well within $0.50. The daily $5 tenant cap enforces abuse prevention at the aggregate level without contradicting the per-query budget.
- 14 REQs all have named unit tests; acceptance coverage matrix complete for 14 REQs (REQ-002/005 unit-test-only per plan-auditor's accepted recommendation).
- Minor deduction: footer version string inconsistency (NF-1) and degrade decision log spec-acceptance inconsistency (NF-2).

### 4.2 Security (25%)

**Score: 0.87 / 1.0 — PASS**

Rubric anchor: OWASP Top 10 — no Critical/High findings.

Evidence:
- **X-User-Id header injection (pre-AUTH-001)**: The risk that any caller can set `X-User-Id` to bypass per-user cap buckets is acknowledged and deliberately accepted in §4 Exclusion #4. The rationale (self-hosted V1, upper proxy validates; AUTH-001 M6 closes the gap) is appropriate for the stated deployment context. No critical severity finding for a V1 self-hosted product.
- **Cross-tenant cache key collision**: Addressed by REQ-012 `SHA256(tenant_id ‖ intent_category ‖ model ‖ messages_json)` salt, with test `TestCacheKeyPrefixIncludesTenantAndIntent` validating different tenant_ids produce different cache keys.
- **TOCTOU race on cap check**: Addressed by Redis Lua script in REQ-009 (single-call atomic eval + increment + TTL refresh) and NFR-004 requiring the 100-goroutine race test.
- **Degraded-path cap bypass**: Documented in Exclusion #10 with cost-asymmetry bound (~1/30 of /deep cost). Attacker must explicitly opt-in via `X-Allow-Degrade: 1` header and accept /basic quality. Risk is self-limiting and documented.
- **PII in Prometheus metrics**: NFR-007 explicitly prohibits it. Tenant label is whitelisted; high-cardinality values are pushed to OTel span attributes (which have separate cardinality semantics per NFR-010).
- **Decision event log PII**: user_id and tenant_id appear in stderr. This is required for audit purposes and aligns with the SPEC-AUTH-003 audit subsystem contract. Security of the log collection pipeline is correctly deferred to the log infrastructure.
- **No credential/secret handling in scope**: All secrets (Redis connection, Postgres DSN) are via environment variables per existing SPEC-BOOT-001 conventions.

No OWASP Critical or High findings.

### 4.3 Craft (20%)

**Score: 0.85 / 1.0 — PASS**

Rubric anchor: 0.75 band = EARS format quality high, all NFRs testable, coverage target stated; 1.0 band = additionally no spec-acceptance inconsistencies.

Evidence:
- EARS format: all 14 REQs have correct pattern labels (D3 resolved) and explicit SHALL verbs.
- NFR testability: all 10 NFRs have measurement mechanisms (histogram quantiles, race tests, named metric names). NFR-006 (durability) is an infrastructure configuration requirement tested at integration level; this is appropriate given Postgres `synchronous_commit` is a server setting.
- TDD test catalog in plan.md covers all 14 REQs and all 10 NFRs (plan.md §3 test catalog table confirms 28 tests: 14 REQ coverage + 10 NFR coverage).
- Coverage target: 85% stated and justified (new package `internal/deepagent/costguard/` only).
- Deduction: NF-1 (footer version string) and NF-2 (degrade decision log spec-acceptance inconsistency) are minor craft defects.
- v0.1.1 patches verified as applied: all 10 claimed patches confirmed in document text.

### 4.4 Consistency (15%)

**Score: 0.90 / 1.0 — PASS**

Rubric anchor: 0.75 band = frontmatter and HISTORY format match sibling SPEC; 1.0 band = additionally no naming deviations.

Evidence:
- Frontmatter key order matches SPEC-DEEP-002 exactly (14 keys, same sequence).
- HISTORY format: date, version label, prose explanation, bullet list of patches. Consistent with DEEP-002 v0.1.1 HISTORY format.
- Korean prose / English identifier rule: consistently applied throughout all 5 documents (spec.md, plan.md, acceptance.md, research.md, spec-compact.md).
- No emoji in any artifact.
- No human-time estimates (behavioral timeouts like 200ms, 5000ms, 30s are system parameters, not predictions — consistent with project rules).
- No XML tags in user-facing content.
- M5 milestone scope respected: AUTH-001 deferred to M6, SPEC-AUDIT-002 deferred to M8, SPEC-COST-OPT-001 deferred to M8. No scope creep into adjacent milestones.
- Prometheus metric prefix `usearch_deep_*` consistent with OBS-001 convention (NFR-009).
- Minor deduction: research.md §5.2 migration filename (NF-3 NIT) and deferred D9 DEEP-003 dependency type ambiguity.

---

## 5. Cross-Validation Matrix

| Evaluator Dimension | Score | Verdict |
|---------------------|-------|---------|
| Functionality (40%) | 0.88 | PASS |
| Security (25%) | 0.87 | PASS |
| Craft (20%) | 0.85 | PASS |
| Consistency (15%) | 0.90 | PASS |
| **Weighted Overall** | **0.87** | **PASS** |

Security dimension does not FAIL (no Critical/High OWASP findings) — hard threshold not triggered.

---

## 6. Findings Summary

| ID | Severity | Source | Location | Description |
|----|----------|--------|----------|-------------|
| NF-1 | MINOR | New (evaluator) | spec.md:416 | Footer reads "v0.1.0" — should be v0.1.1 |
| NF-2 | MINOR | New (evaluator) | acceptance.md §5.6; REQ-DEEP4-011 | Degrade decision event log shown in acceptance but not mandated by any REQ |
| NF-3 | NIT | New (evaluator) | research.md §5.2 | Migration filename `0002_create_cost_ledger.sql` differs from canonical `0002_cost_ledger.sql` in spec.md — research.md is a frozen artifact; no spec.md fix needed |
| D9 | MINOR | Deferred from plan-audit | spec.md §6.1 | SPEC-DEEP-003 dependency strength (design-input vs runtime-code) still unclarified |
| D12 | NIT | Deferred from plan-audit | acceptance.md:343-348 | REQ-002/005 still unit-test-only without GWT scenarios |

---

## 7. Recommendations

1. **Apply NF-1 patch** (low-effort): Change spec.md line 416 footer from `v0.1.0` to `v0.1.1`. No REQ or NFR changes needed.

2. **Apply NF-2 patch** (architectural clarity): Add to REQ-DEEP4-011 a SHALL clause: "decision event log에 decision='degrade' event를 SHALL 출력한다" — aligning acceptance.md §5.6, research §8.3, and the binding REQ text. Alternatively, remove the decision event log sample from acceptance.md §5.6. Option A is recommended for observability completeness.

3. **NF-3, D9, D12**: No action required before status `draft → approved`. These are cosmetic or explicitly deferred items.

4. **NF-1 and NF-2 do not block status transition** but should be addressed in a v0.1.2 patch before the SPEC is handed to manager-tdd for implementation, to prevent implementer confusion.

---

## 8. Disagreement Analysis

The plan-auditor's PASS verdict at 0.86 overall is confirmed. This evaluator's independent weighted score is 0.87, within noise of the plan-auditor's 0.86. The two new MINOR findings (NF-1, NF-2) do not rise to the level of must-pass criterion violations:

- NF-1 is purely cosmetic (version string in footer).
- NF-2 is a spec-acceptance consistency gap that creates implementer ambiguity but does not affect the testability of the 14 REQs or the 10 NFRs.

Neither finding is a security violation, an EARS format failure, or a coverage mechanism gap.

**No must-pass criteria violations identified.**

---

*End of SPEC-DEEP-004 evaluator cross-validation report, iteration 1.*
