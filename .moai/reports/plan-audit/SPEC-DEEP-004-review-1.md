# SPEC Review Report: SPEC-DEEP-004

Iteration: 1/3
Harness level: thorough (require_must_pass: true, cross_validate_with_evaluator_active: true)
Verdict: **PASS** (with 2 MAJOR findings recommended for resolution before status `draft → approved`)
Overall Score: 0.86

Reasoning context ignored per M1 Context Isolation. Audit relies only on spec.md, plan.md,
acceptance.md, research.md, spec-compact.md, plus cross-reference anchors specified by the
caller (roadmap.md, sibling SPECs, product.md, OBS-001 cardinality test).

---

## Must-Pass Results

- **[PASS] MP-1 REQ number consistency** — 14 REQs are sequential `REQ-DEEP4-001`..`REQ-DEEP4-014`,
  no gaps, no duplicates, consistent three-digit zero-padding (spec.md:126,127,133,134,135,141–143,
  149–151,157–159).
- **[PASS] MP-2 EARS format compliance** — Every REQ contains an explicit `SHALL` verb and a
  recognised EARS pattern label (`Ubiquitous` × 7, `Event-Driven` × 3, `Optional` × 2,
  `Unwanted` × 2). Verified by line-scan of spec.md:126–159. One minor pattern-label drift on
  REQ-DEEP4-003 (declared Ubiquitous but contains the implicit trigger "cap-check를 통과한
  직후" which is event-driven semantics) — captured below as N1.
- **[PASS] MP-3 YAML frontmatter validity** — All 8 required fields plus the 6 caller-required
  extended fields are present and well-typed (spec.md:2–16):
  `id` (string), `version` (string `0.1.0`), `status` (`draft`), `created` (ISO date
  `2026-05-21`), `priority` (`P0`), `milestone` (`M5 — /deep multi-agent`), `owner`
  (`expert-backend`), `methodology` (`tdd`), `coverage_target` (`85`, int),
  `depends_on` (7-element list), `blocks` (empty list — justified in §6.2). Schema uses
  `created` not `created_at`, matching sibling SPEC-DEEP-002 convention — acceptable.
- **[N/A] MP-4 Section 22 language neutrality** — N/A: single-language Go project. The Go-side
  scope is explicit in §7.3 ("Python 사이드카 — 본 SPEC v1 범위 밖") and Exclusion #8.

---

## Category Scores (rubric-anchored, 0.0–1.0)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.85 | 0.75 band | Pinned decisions §1.1 are concrete (spec.md:88–107). Two definitional collisions noted (D2, D3 audit-log terminology). |
| Completeness | 0.90 | 1.0 band | HISTORY, WHY (§1.2), WHAT (§1, §2), HOW (§7), REQUIREMENTS (§2), ACCEPTANCE (§5 + acceptance.md), Exclusions (§4, 9 items), Dependencies (§6), Open Questions (§8) all present. |
| Testability | 0.90 | 1.0 band | Every REQ has named tests (spec.md:126–159). Concurrent race test (`TestCapCheckConcurrent100RequestsNoRace`) and Edge case for cap boundary (acceptance.md:286–326) cover the highest-risk surface. |
| Traceability | 0.88 | 1.0 band | Acceptance Coverage Matrix (acceptance.md:330–341) lists every REQ. REQ-002 and REQ-005 lack a main GWT scenario but are explicitly footnoted as unit-test-only (acceptance.md:343–348) and have named tests in spec.md. |

Overall: PASS. The two MAJOR findings (D1 audit-log conflation, D2 degraded-path loophole) do
not break must-pass criteria but should be patched before the SPEC ships to manager-tdd.

---

## Defects Found

| # | Severity | Location | Description | Fix Recommendation |
|---|----------|----------|-------------|---------------------|
| D1 | **MAJOR** | spec.md:150 (REQ-DEEP4-010) ↔ spec.md:172 (NFR-DEEP4-006) | Terminology conflation between two distinct "audit log" concepts. REQ-010 uses "audit log" to mean a stderr-emitted JSON line (`{"event_type":"cap.evaluation", ...}`, confirmed in acceptance.md:44–47). NFR-006 ("Audit log durability") talks about Postgres `cost_ledger` rows with `synchronous_commit=on`. These are two different artifacts with different durability stories. A reader could believe NFR-006's fsync guarantee applies to stderr JSON lines. | Either (a) rename NFR-006 to "Ledger row durability" and rename REQ-010's "audit log" to "decision event log" (preferred), or (b) introduce a §6.4 "Audit Artifacts" subsection defining the two distinct artifacts and their durability contracts separately. |
| D2 | **MAJOR** | spec.md:151 (REQ-DEEP4-011) | Degraded-path quota-bypass loophole is unaddressed. The SPEC explicitly states that `/basic` calls made via `X-Allow-Degrade: 1` after a cap hit are recorded with `outcome="degraded"` but "cap 평가에는 산입 SHALL NOT 된다". A caller who has hit cap can therefore call `/deep?X-Allow-Degrade:1` repeatedly and consume unlimited `/basic` budget. Exclusion §4 enumerates 9 items but no item names this surface as accepted risk. | Either (a) add a 10th exclusion item explicitly accepting the loophole as deliberate (with rationale: degraded `/basic` cost is ~$0.002 vs `/deep` $0.07–$0.19, so abuse is bounded by `/basic`'s own cap), or (b) add a secondary cap on degraded calls per window (e.g., `costguard.degrade.max_per_day`). The audit recommends option (a) given the cost asymmetry — but it must be made explicit. |
| D3 | MINOR | spec.md:133 (REQ-DEEP4-003) | EARS pattern label drift. Declared `Ubiquitous` but the requirement begins with an implicit trigger "`/deep` 요청이 cap-check를 통과한 직후" — this is Event-Driven semantics. | Relabel to `Event-Driven` and rewrite the lead-in as "WHEN a `/deep` request has passed cap-check, the cost guard SHALL ...". |
| D4 | MINOR | spec.md:159 (REQ-DEEP4-014) | Redis recovery-detection mechanism unspecified. The REQ states "Redis 복구 시 Asynq job `costguard.RehydrateWindow`가 ... 자동 재구성 SHALL 한다" but how recovery is detected (health-check poll? circuit-breaker half-open probe? Asynq retry on enqueue?) is not defined. | Add a sentence: "Redis 복구는 `costguard.redis.health_check_interval_ms` (기본 5000ms) 주기 health probe로 감지하며, 연속 N회 성공 후 RehydrateWindow를 1회 트리거한다." |
| D5 | MINOR | spec.md:168 (NFR-DEEP4-002) ↔ spec.md:142 (REQ-DEEP4-007) | Latency budget vs failure-path retry conflict. NFR-002 requires Redis INCR + Asynq enqueue p95 ≤ 50ms. REQ-007 mandates "Redis INCR 실패 시 동기적으로 retry(최대 3회 exponential backoff)" — three retries with exponential backoff can take 1750ms minimum (250+500+1000) before the fail-closed return. The NFR does not state whether its budget covers the failure path. | Amend NFR-002 to specify "success path only" or split into NFR-002a (success p95 ≤ 50ms) + NFR-002b (failure-closed total wall-clock ≤ 2000ms p99). |
| D6 | MINOR | spec.md:157 (REQ-DEEP4-012) + plan.md:121 | `cache_key` prefix mechanism implementation is hazy and risks prompt contamination. REQ-012 says prefix is added "on top of" LiteLLM's default SHA256(model + messages_json). Plan.md Phase D test `TestCacheKeyPrefixIncludesTenantAndIntent` shows the prefix `[tenant={id}][intent={cat}]` is auto-attached to `messages[0]`. Mutating the actual prompt text to influence cache key partitioning means the LLM also sees the prefix, which can alter completions and confuse downstream prompt-engineering audits. | Specify in REQ-012 either (a) the prefix is injected as a non-prompt-visible cache-key salt (requires LiteLLM cache-key hook or wrapper layer that intercepts before SHA256 but does NOT touch messages), or (b) explicitly accept that messages[0] is mutated and add a unit test asserting the prefix is a structured token sequence that the LLM treats as a no-op (e.g., `<system tag>` style). The current text is ambiguous. |
| D7 | MINOR | spec.md:175 (NFR-DEEP4-009) | Factual attribution error. NFR-009 says "`usearch_deep_calls_total` is separate from SPEC-DEEP-002 REQ-DEEP2-008의 기존 `usearch_deep_outcomes_total`". The `usearch_deep_outcomes_total` collector is originally registered by SPEC-DEEP-001 (verified in DEEP-001 spec.md:99,138,340,342,344 and DEEP-002 spec.md:305 explicitly says "The existing `usearch_deep_outcomes_total{outcome}` counter (SPEC-DEEP-001) SHALL be extended..."). DEEP-002 REQ-008 extends it, does not own it. | Change "SPEC-DEEP-002 REQ-DEEP2-008의 기존" to "SPEC-DEEP-001 (extended by SPEC-DEEP-002 REQ-DEEP2-008)". |
| D8 | MINOR | spec.md:252 (§6.1 SPEC-IR-001 entry) | Stale dependency status. SPEC §6.1 marks SPEC-IR-001 as `(draft)` but IR-001 is `status: implemented` per its frontmatter (`SPEC-IR-001/spec.md:6`, implemented in commit 8a20b68). | Update to `SPEC-IR-001 (implemented)`. |
| D9 | MINOR | spec.md:244 (§6.1 SPEC-DEEP-003 entry) | Dependency strength overstated relative to actual contract surface. The SPEC's claim is "tree exploration의 breadth/depth가 cap 차원 설계에 직접 반영(최악-경우 호출 횟수 추산)" — i.e., DEEP-003 informs capacity planning, not a code-level interface. DEEP-004 does NOT consume DEEP-003's exported metrics (`usearch_deep_tree_*`) or its `TreeResult` type. Calling this `depends_on` is conservative but conflates "design-input dependency" with "runtime-code dependency". | Either (a) keep `depends_on` and clarify the §6.1 entry as "design-input only: capacity-planning anchor, no runtime API consumption", or (b) demote to a soft reference (move out of `depends_on`, note in §1.2 motivation). |
| D10 | MINOR | spec.md:253 (§6.1 SPEC-IR-001) ↔ REQ-DEEP4-003, REQ-DEEP4-012 | Relationship between Haiku pre-screen and IR-001 intent router is not stated explicitly. REQ-012 says cache_key prefix includes `intent_category` (which must originate from IR-001's `RoutingDecision.Category`), but REQ-003 (Haiku screen) makes no call into IR-001. The audit prompt's question — "does Haiku pre-screen plug into intent router or is it parallel?" — has no explicit answer in the SPEC. | Add to §1 Overview or §6.1 IR-001 entry: "Haiku pre-screen and IR-001 are orthogonal gates: IR-001 classifies query into intent category (consumed only as a cache-key prefix); Haiku pre-screen judges deep-warranted-ness independently. The two are NOT chained." |
| D11 | MINOR | spec.md:268 (§6.3 forward-compat) + plan.md:196 | SPEC-AUTH-003 audit-log schema forward-compat mentioned in plan.md ("REFACTOR: audit log schema를 SPEC-AUTH-003 (M6)와 align하도록 별도 struct에 centralize") but NOT committed in spec.md. The spec.md §6.3 only addresses AUTH-001 forward-compat for `cost_ledger.user_id`. The audit-log JSON line shape (REQ-010) has no documented forward-compat statement for AUTH-003 consumption. | Extend §6.3 with a paragraph: "Audit log JSON line schema (REQ-DEEP4-010) SHALL include the fields required by SPEC-AUTH-003 (timestamp ISO-8601, event_type, request_id, tenant_id, user_id, decision). Schema is additive; AUTH-003 may add fields but SHALL NOT rename or remove the listed fields." |
| D12 | NIT | acceptance.md:343–348 | REQ-DEEP4-002 (AUTH-001 forward-compat) and REQ-DEEP4-005 (Haiku breaker) have no main Given/When/Then scenario, only footnoted unit-test coverage. Per the AC-5 traceability rule this is preserved (named tests exist in spec.md REQ table), but consistency with the seven other REQs which DO have GWT scenarios is broken. | Optional: add §5.8 (REQ-002 forward-compat smoke scenario) and §5.9 (REQ-005 breaker open scenario) for symmetry, or leave as-is and accept the unit-test-only coverage. Recommend: leave as-is. |

---

## Chain-of-Verification Pass

Second-look re-read targeting the audit prompt's specific items:

- **REQ numbering**: re-scanned all 14 entries end-to-end (not spot-check). Confirmed sequential
  with no gaps or duplicates.
- **EARS verbs**: re-scanned each REQ for explicit `SHALL`. Every REQ contains it. REQ-003
  pattern label drift noted as D3.
- **Traceability**: cross-checked acceptance.md:330–341 matrix against spec.md REQ-table
  Acceptance column. Every REQ has at least one named test. REQ-002/REQ-005 GWT gap noted as D12.
- **Exclusions specificity**: counted spec.md:185–214 — 9 items, each ties to a successor SPEC
  or rationale (M6 AUTH-001, M8 EVAL-002, M6 AUTH-003, M7 AUTH-004, M9 EVAL-003, M8 COST-OPT-001).
  Confirmed ≥ 7 threshold met. PASS.
- **Pinned decisions concreteness**: D1 (header name + DEFAULT 'anonymous'), D2 (Lua script +
  Asynq batch), D3 (20 calls OR $5/day), D4 (6/4 thresholds + hot-reload), D5 (LiteLLM only),
  D6 (429 default, opt-in degrade), D7 (90d hot retention, M8 archival). All concrete with
  numbers, file paths, env-var names, or successor SPEC handoffs.
- **Forward-compat AUTH-001**: spec.md:127 (REQ-002) + spec.md:264–268 (§6.3) commit
  `cost_ledger.user_id` opaque TEXT NOT NULL DEFAULT 'anonymous' with no schema migration
  required at V1→V1.1. Concrete, schema-level. PASS.
- **Forward-compat AUTH-003**: spec.md is silent on audit-log JSON line schema forward-compat —
  noted as D11.
- **Cap atomicity**: spec.md:149 REQ-009 + NFR-004 + Edge case (acceptance.md:286–326)
  specify Redis Lua script single-call atomic execution and a 100-goroutine concurrency test.
  Specified, not waved. PASS.
- **Cap dollars as tunable**: §1.1 D3 + §8 Open Question Q1 + REQ-009 deep.yaml keys +
  NFR-008 hot-reload. Documented as policy that the dollar floors are operational config,
  not hardcoded constants. PASS.
- **Migration file path**: `deploy/postgres/migrations/0002_cost_ledger.sql` — verified
  directory exists with existing `0001_create_docs.sql`. Path corrected from prompt's
  earlier `internal/index/postgres/migrations/` suggestion. PASS.
- **Korean / English identifier rule**: prose throughout is Korean, all identifiers, REQ IDs,
  metric names, and code paths are English. PASS.
- **No emoji**: verified absent in spec.md, plan.md, acceptance.md, spec-compact.md.
- **No time estimates**: 30-second circuit-breaker window, 200ms timeout etc. are behavioral
  timeouts (system response budgets), not human-time predictions. Conforms to the rule.

New defects discovered during second pass:
- D2 (degraded-path loophole) was missed during first scan and surfaced during the
  re-read of REQ-DEEP4-011.
- D5 (NFR-002 vs REQ-007 retry budget collision) was missed during first scan and
  surfaced during cross-NFR/REQ consistency check.
- D6 (cache_key prefix prompt-contamination risk) was missed during first scan; surfaced
  when reading plan.md Phase D test description.
- D10 (Haiku ↔ IR-001 relationship explicitness) was the audit-prompt's direct question
  but the SPEC has no explicit answer; flagged.
- D11 (AUTH-003 audit-log forward-compat absent from spec.md) was specifically asked by
  the prompt and is not committed in the SPEC.

Chain-of-Verification result: 5 additional defects added during second pass, none promoting
to BLOCKER. Verdict unchanged: PASS with 2 MAJOR + 8 MINOR + 1 NIT to address before status
transition.

---

## Regression Check (Iteration 2+ only)

N/A — iteration 1.

---

## Recommendation

**PASS** with the following pre-approval patches recommended (numbered for manager-spec
to address):

1. **Resolve D1 (MAJOR)**: Disambiguate "audit log" vs "ledger row" terminology in
   REQ-DEEP4-010 and NFR-DEEP4-006. Preferred: rename NFR-006 to "Ledger row durability" and
   REQ-010's reference to "decision event log".

2. **Resolve D2 (MAJOR)**: Add a 10th Exclusions entry making the degraded-path
   cap-bypass loophole explicit, or add a `costguard.degrade.max_per_day` cap. Recommend
   the Exclusions option given `/basic` cost asymmetry (~30× cheaper).

3. **Patch D3, D7, D8 (MINOR, low-effort)**: Trivial relabel/attribution fixes.

4. **Patch D4, D5, D6 (MINOR, architectural clarity)**: Specify Redis recovery detection,
   clarify NFR-002 latency-budget scope, clarify cache_key-prefix implementation mechanism.

5. **Patch D10, D11 (MINOR, integration clarity)**: Explicitly state Haiku ↔ IR-001
   orthogonality, and commit AUTH-003 audit-log schema forward-compat in §6.3.

6. **D9 and D12 are optional cosmetic improvements** — manager-spec may defer.

Rationale for PASS verdict despite the MAJOR findings: both MAJOR items are about
**documentation clarity and risk-acknowledgment**, not about specification incorrectness.
The 14 REQs are well-formed EARS, the 10 NFRs cover the requested surface (latency,
atomicity, drift, durability, PII, hot-reload, naming), the 9 exclusions are specific
and forward-handed to successor SPECs, the schema-level AUTH-001 forward-compat is
concretely committed, and the cap race condition is addressed with named Lua-script
atomicity + a concurrency test. The MAJOR items can be patched in a v0.1.1 amendment
similar to the SPEC-DEEP-002 v0.1.0 → v0.1.1 cycle without invalidating any pinned
decision or REQ structure.

Evidence supporting PASS:
- MP-1: spec.md:126–159, 14 sequential REQ IDs verified.
- MP-2: every REQ contains explicit `SHALL`; pattern labels match the requirement bodies
  with one minor exception (D3).
- MP-3: spec.md:2–17, all required + caller-required extended fields present and well-typed.
- MP-4: N/A (single-language Go project).

---

*End of SPEC-DEEP-004 review report, iteration 1.*
