# SPEC Review Report: SPEC-DEEP-003

Iteration: 1/3
Harness level: thorough (max_iterations: 3, require_must_pass: true, cross_validate_with_evaluator_active: true)
Auditor: plan-auditor (independent, bias-prevention M1–M6 active)
Reasoning context ignored per M1 Context Isolation.

**Verdict: FAIL**

Overall Score: 0.66

---

## Executive Summary

The SPEC is structurally strong: 12 EARS-formatted REQs (no numbering gaps), 8 quantitative NFRs, 9 specific exclusions, full traceability matrix in `acceptance.md`, frontmatter complete and consistent with sibling SPEC-DEEP-002, no emoji, no time estimates, Korean prose with English identifiers respected. Architecture decisions are pinned with concrete defaults (breadth=4, depth=3, token budget 60K, node timeout 30s).

However, **three defects rise to BLOCKER severity** and block PASS:

1. **Cross-reference error to SPEC-DEEP-002**: spec.md attributes Researcher single-shot behavior to "DEEP-002 REQ-002" in five locations. REQ-DEEP2-002 actually defines the *orchestrator sequencing* requirement. The Researcher single-shot fanout behavior is defined in **REQ-DEEP2-005**. Without correcting these references, any implementer following DEEP-003 will derive Researcher contracts from the wrong DEEP-002 clause.

2. **Internal contradiction on `depth=0` handling**: REQ-DEEP3-002 enforces `depth ∈ [1, 5]` with HTTP 400 on violation and explicitly carves out only `breadth=0`. REQ-DEEP3-005 contradicts this by stating "`depth=0`이 지정된 경우도 동일하게 fallback 한다" (HTTP 200 fallback). `acceptance.md` Scenario 5.2 contradicts REQ-005 again by asserting `depth=0` returns HTTP 400 ("REQ-005의 fallback과 별도"). Scenario 5.6 (in spec.md and acceptance.md) only covers `breadth=0` fallback, leaving `depth=0` testably undefined. Three sources, three different stances.

3. **Writer agent REQ attribution error**: spec.md L104 attributes "Writer [DEEP-002 REQ-006, FlattenedClaim 소비]" — but REQ-DEEP2-006 defines the Verifier's `CheckFaithfulness` invocation, not Writer. Writer's primary contract in DEEP-002 spans REQ-DEEP2-003 (retry) and others.

These are not stylistic — they create concrete implementation ambiguity. MP-2/MP-1/MP-3 PASS, but the consistency dimension (CN-1, CN-2) fails on contradiction.

---

## Must-Pass Results

- **[PASS] MP-1 REQ number consistency**: REQ-DEEP3-001 through REQ-DEEP3-012, sequential, no gaps, no duplicates. Evidence: spec.md L155, 166, 176, 190, 202, 213, 223, 232, 244, 253, 266, 277. 12 REQs counted via `grep -c "^\*\*REQ-DEEP3-" spec.md`.

- **[PASS] MP-2 EARS format compliance**: All 12 REQs match an EARS pattern, each tagged with its pattern in parentheses:
  - Ubiquitous (002, 009, 010): use "SHALL"
  - Event-Driven (001, 006): use "WHEN ... SHALL"
  - State-Driven (003, 004, 007, 011): use "WHILE ... SHALL"
  - Conditional (005, 008): use "IF ... SHALL"
  - Optional (012): uses "WHERE ... SHALL"
  Evidence: spec.md L155–288. Note that one REQ in EARS canonical taxonomy is labeled "Unwanted" (using "IF ... THEN ... SHALL") not "Conditional"; the SPEC uses "Conditional" as a synonym which is acceptable but mildly nonstandard — minor.

- **[PASS] MP-3 YAML frontmatter validity**: All required fields present and well-typed.
  - `id: SPEC-DEEP-003` ✓
  - `version: 0.1.0` ✓
  - `status: draft` ✓
  - `created: 2026-05-21`, `updated: 2026-05-21` ✓ (project uses `created`/`updated` not `created_at`; consistent with SPEC-DEEP-002 frontmatter)
  - `author: limbowl` ✓
  - `priority: P0` ✓
  - `issue_number: 0` ✓ (documented as pending in Exclusions L344; consistent with DEEP-002 v0.1.0 history pattern)
  - `title`, `milestone`, `owner: expert-backend`, `methodology: tdd`, `coverage_target: 85`, `depends_on: [8 SPECs]`, `blocks: [SPEC-DEEP-004]` ✓
  Evidence: spec.md L1–17.

- **[N/A] MP-4 Section 22 Language Neutrality**: This SPEC scopes Go (`internal/deepagent/`) + Python (`services/researcher/`) tooling and explicitly names them per architecture decision §1.1.1. No multi-language LSP enumeration is in scope. Auto-passes per auditor M5 N/A rule.

---

## Category Scores (rubric-anchored, 0.0–1.0)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.65 | 0.50–0.75 (multiple ambiguities; some require interpretation) | depth=0 inconsistency (D2); REQ cross-refs (D1, D3); REQ-DEEP3-006 root pre-check seed value undefined (D9); see findings below |
| Completeness | 0.85 | 0.75–1.0 | All sections present (HISTORY L21, Overview §1, FR §2, NFR §3, Exclusions §4 with 9 entries, Acceptance §5, Dependencies §6, Files §7, Open Questions §8, Configuration §9, References §10). Frontmatter complete. Minor: §3 NFR-DEEP3-008 reload behavior under-specified (D7). |
| Testability | 0.80 | 0.75–1.0 | Every REQ has scenario coverage in acceptance.md §5.1–5.7 with named test functions (e.g., `TestExpandTreeHappyPath` at acceptance.md L67). NFR-DEEP3-001..008 each have quantitative assertion (p95 latency, byte size, label cardinality, ≤200 KB compressed). Minor: NFR-DEEP3-002 (4-min p95) uses 25-iteration mock — borderline statistical power for p95 (D10). |
| Repo Alignment | 0.65 | 0.50–0.75 | `migrations/` directory does not exist yet in repo (verified via `ls`). `0NN` placeholder is not pinned (D6). `internal/deepagent/` is being introduced by DEEP-002 which is still draft v0.1.1 — DEEP-003's MODIFY targets depend on DEEP-002 landing first (declared correctly via `depends_on`). `services/researcher/src/researcher/` exists; new `deep_tree.py` is a fresh-file addition consistent with synthesis.py pattern. DEEP-004 referencing claim is overstated (D5). |

Overall composite (equal weight): (0.65 + 0.85 + 0.80 + 0.65) / 4 = **0.74**, but firewalled by BLOCKER defects → Verdict FAIL.

---

## Findings Table

| ID | Severity | Location | Description | Fix Recommendation |
|----|----------|----------|-------------|--------------------|
| D1 | **BLOCKER** | spec.md L98, L103, L129, L162–163, L204 (5 sites) | Cross-references "DEEP-002 REQ-002" for Researcher single-shot behavior are incorrect. REQ-DEEP2-002 defines the *orchestrator sequence* + Reviewer/Writer no-retrieval invariants. The Researcher single-shot fanout contract is defined in **REQ-DEEP2-005** (verified at SPEC-DEEP-002 spec.md L283: "Researcher agent SHALL invoke fanout.Dispatch ... exactly once per pipeline invocation"). | Replace all "DEEP-002 REQ-002" references that pertain to Researcher with **"DEEP-002 REQ-005"**. Specifically: L98 "[DEEP-002 REQ-002]" → "[DEEP-002 REQ-005]"; L129 "Researcher 에이전트(REQ-002)" → "Researcher 에이전트(REQ-005)"; L162–163 "DEEP-002 REQ-002의 single-shot fanout 동작" → "DEEP-002 REQ-005의 single-shot fanout 동작"; L204 "SPEC-DEEP-002 REQ-002의 single-shot Researcher 동작" → "SPEC-DEEP-002 REQ-005의 single-shot Researcher 동작". Audit acceptance.md and spec-compact.md for the same pattern. |
| D2 | **BLOCKER** | spec.md L166–172 (REQ-002) ↔ L202–209 (REQ-005) ↔ acceptance.md L108–109 (Scenario 5.2) | Internal contradiction on `depth=0` handling. REQ-002 enforces `depth ∈ [1, 5]` with HTTP 400; carve-out mentions only `breadth=0`. REQ-005 says `depth=0` also falls back to single-shot (HTTP 200). acceptance.md §5.2 says `depth=0` returns HTTP 400 ("REQ-005의 fallback과 별도"). Scenario 5.6 only tests `breadth=0`. Implementer cannot resolve which response code is correct. | Pick ONE policy and enforce it everywhere. Recommended: make REQ-005 a `breadth=0 OR depth=0` fallback (HTTP 200) and update REQ-DEEP3-002 carve-out to read "`breadth=0` AND `depth=0`은 별도 처리로 REQ-005가 다루며 범위 위반에 해당하지 않는다." Then update acceptance.md L108–109 to drop the `depth=0 → HTTP 400` assertion, and rename Scenario 5.6 to "breadth=0 OR depth=0 fallback". Alternatively, narrow REQ-005 to only `breadth=0` (drop the "`depth=0`이 지정된 경우도 동일하게 fallback" sentence). Either way, all three sources MUST agree. |
| D3 | **BLOCKER** | spec.md L104 | "Writer [DEEP-002 REQ-006, FlattenedClaim 소비]" attribution is wrong. REQ-DEEP2-006 defines the Verifier's `CheckFaithfulness` invocation (verified at SPEC-DEEP-002 spec.md L298). Writer's contract in DEEP-002 is distributed across REQ-DEEP2-003 (Writer retry trigger) and REQ-DEEP2-009a (max-retry exhaustion). | Replace "[DEEP-002 REQ-006, FlattenedClaim 소비]" with a correct attribution, e.g., "[DEEP-002 Writer agent, FlattenedClaim 소비; primary contract via REQ-DEEP2-003 retry semantics]". If the intent is to point at the Writer's input contract (FlattenedClaim), no single DEEP-002 REQ owns that contract — state that DEEP-003 itself introduces FlattenedClaim (REQ-DEEP3-010) and Writer consumes it. |
| D4 | **MAJOR** | spec.md L460–463 (§6.2 blocks) ↔ SPEC-DEEP-004/spec.md L244 | DEEP-003 claims DEEP-004 consumes `Node.TokensUsed` and `usearch_deep_tree_total_tokens` metrics, but SPEC-DEEP-004/spec.md only contains one generic reference ("tree exploration의 breadth/depth가 cap 차원"). The specific consumption contract is not declared from DEEP-004's side. The `blocks` relation is bi-directionally consistent at the frontmatter level (DEEP-004.depends_on includes DEEP-003) but the *semantic contract* is asymmetric. | Either (a) weaken DEEP-003 §6.2 to say "DEEP-004 references this SPEC as an upstream dependency; specific metric/field consumption is TBD in DEEP-004 implementation", or (b) coordinate with DEEP-004's author to add an explicit "consumes Node.TokensUsed and usearch_deep_tree_total_tokens from SPEC-DEEP-003" line in DEEP-004 §6.1. Option (a) is the lower-risk fix for this SPEC. |
| D5 | **MAJOR** | spec.md L274, L483–484 | Migration file naming uses `0NN_deep_runs.up.sql` / `0NN_deep_runs.down.sql` placeholder. `migrations/` directory does not exist in the repo (verified via `ls /Users/masterp/Projects/superwork/universal-search/migrations/` — empty/absent). The migration tool, numbering convention, and the source of `NN` (next-available counter or hash) are unpinned. §1.1 claims decisions are "concrete and pinned" but the migration sequencing is a deferred decision. | Either (a) pin the migration tool (e.g., golang-migrate, sqlx-migrate) and the next sequence number now, OR (b) move this decision to §8 "Open Questions" rather than presenting it as pinned. Add a one-sentence reference to the migration policy (which directory, which tool, which numbering scheme) — possibly delegate to a `SPEC-INFRA-001` or similar if one exists, otherwise document inline. |
| D6 | **MAJOR** | spec.md L213–221 (REQ-DEEP3-006) | Pre-check formula `estimated_next_cost = parent.TokensUsed * breadth * 1.25` is undefined for the **root** node, where no `parent.TokensUsed` exists prior to root expand. The first pre-check timing (before root vs after root) is not specified. Without this, the budget cap is non-deterministic on small trees and the algorithm cannot be implemented unambiguously. | Add an explicit clause to REQ-DEEP3-006: "For the root node, the pre-check is skipped (root expand is always attempted)" OR "For the root node, `estimated_next_cost` is seeded from a configurable constant `DEEP_TREE_ROOT_TOKEN_ESTIMATE` (default 5000)". Update the env-var table §9 accordingly if option two is chosen. |
| D7 | **MAJOR** | spec.md L302 (NFR-DEEP3-008) ↔ acceptance.md L222–229 (Scenario 5.5 Then) | Crash recovery semantics are split: NFR-DEEP3-008 says the *reload logic* (used by SPEC-DEEP-004) reclassifies `Status != Complete` nodes as `Failed`. But this reload logic is not declared as a contract anywhere in DEEP-003's REQs — it appears only as an NFR side-comment. Scenario 5.5 Then assumes the reload behavior is implemented as part of DEEP-003 (test `TestPersistenceReclassifyOnReload`). NFR-DEEP3-008 also says "Resume 기능은 본 SPEC 범위 밖 (§4 Exclusions)" — consistent — but the reload-and-reclassify capability that DEEP-004 depends on is not separately specified. | Promote the reload-and-reclassify behavior to a functional REQ (or add it to REQ-DEEP3-011) so it has implementation status and traceable testability. Example: a new clause in REQ-DEEP3-011 stating "On reload, the persistence layer SHALL reclassify any Node with `Status ∈ {Pending, Expanding}` to `Failed` and return the tree as read-only." |
| D8 | MINOR | spec.md L244–251 (REQ-DEEP3-009) | "타 노드의 doc cross-reference는 by construction 금지된다" is enforced "by construction" but no explicit invariant test is named in acceptance.md. If two sibling nodes' fanout returns happen to overlap on the same `doc_id`, both `Node.Citations` slices will contain the same `doc_id` — this is duplication across nodes, not cross-reference. The wording is ambiguous about whether this is allowed. | Clarify: "Each `Node.Citations` SHALL contain only the doc_ids returned by that node's own `fanout.Dispatch` call. Cross-node sharing of doc_ids by independent fanout returns is permitted (and expected for popular docs); only *referencing another node's Citations slice* is prohibited." Add a test `TestNodeCitationsAreDisjointlyOwned` to acceptance.md §5.4. |
| D9 | MINOR | spec.md L268 (REQ-DEEP3-011) | "매 노드 transition 시" (every node transition) flush, but the trigger set is only "`Status` transition to one of `{Complete, Failed, BudgetExceeded}`". Pending → Expanding transitions are excluded — fine, but not explicitly stated. Reader may infer flush on every transition including the in-progress Expanding state. | Explicitly state: "Flush is triggered only on terminal transitions (`Status → {Complete, Failed, BudgetExceeded}`); `Pending → Expanding` transitions are in-memory only." |
| D10 | MINOR | spec.md L296 (NFR-DEEP3-002) | "p95 ≤ 4 min" measured by 25-iteration mock — 25 samples is borderline for p95 (95th percentile of 25 = 23.75-th element, so really capturing the 92nd percentile in practice). The earlier NFR-DEEP3-001 uses 50-iteration which is acceptable. | Increase NFR-DEEP3-002 iteration count to 50 to match NFR-DEEP3-001, OR explicitly note "p95 here estimated via 25-iteration mock as a smoke test; tighter validation deferred to load test phase". |
| D11 | MINOR | spec.md L141 ↔ L155–288 | §2 intro says "12개 functional requirement를 5개 모듈로 분류한다" but the module list at L143–151 says "Tree Initialization (REQ-001, 002)" / "Node Expansion (REQ-003, 004, 005)" — a 12-REQ count where REQ-005 is the breadth=0 fallback, which logically belongs to "Tree Initialization" (it's a special-case of input handling) rather than "Node Expansion." | Either keep as-is (the categorization is editorial) or move REQ-005 grouping to "Tree Initialization" since it diverts the request *before* any expand begins. |
| D12 | MINOR | spec.md L19 / Section header | Document uses section heading "## 1.0 Overview" (with `.0` suffix). All sibling SPECs (DEEP-002, DEEP-004) use "## 1. Overview". | Change "## 1.0 Overview" → "## 1. Overview" for consistency with sibling M5 SPECs. |
| D13 | NIT | spec.md L506 | Configuration table column header "Owner" lists REQ-IDs (e.g., "REQ-DEEP3-001"), not actual owners (e.g., "expert-backend"). | Rename column header from "Owner" to "Referenced By" or "Source REQ". |
| D14 | NIT | spec.md L344–345 | Exclusion bullet "**GitHub Issue tracking on this SPEC** (`issue_number: 0`)" is a process note, not a product exclusion. | Move this to HISTORY (where it is also noted at L71–72) or remove from §4. |
| D15 | NIT | spec.md L482 | `.moai/config/sections/deep.yaml` (NEW) — adjacent SPECs typically place per-SPEC config in this directory but no precedent for "deep.yaml" exists. Naming overlap with potential future SPEC-DEEP-* configs may conflict (e.g., would SPEC-DEEP-004 also write to deep.yaml?). | Confirm whether this is a single shared "/deep" feature config (then naming is fine) or SPEC-specific (then rename to `deep-tree.yaml`). |

---

## Chain-of-Verification Pass

Second-look findings (M6 self-critique):

- Re-read REQs 001–012 end-to-end: All 12 contain explicit "SHALL" verbs and an EARS trigger. No skim — confirmed by sequential read of spec.md L155–288.
- Re-checked REQ number sequencing: `grep "REQ-DEEP3-" spec.md` returned 12 instances 001..012 with no gaps or duplicates.
- Re-checked traceability: For each REQ-DEEP3-NNN, located at least one scenario in acceptance.md §5.1–5.7 and at least one named test function in acceptance.md and plan.md §2 catalog. Confirmed REQ-007 (token accumulation) traces to §5.3 `TestExpandTreeBudgetExceeded`.
- Re-checked exclusions specificity: All 9 exclusion entries are specific products/behaviors with rationale (e.g., "UI tree visualization → M7 SPEC-UI-001 책임"). None vague.
- Re-checked contradictions: Identified D2 (depth=0) as the dominant internal contradiction. No other contradictions found within the document.
- Re-checked cross-references to SPEC-DEEP-002: Found 5 occurrences of "DEEP-002 REQ-002" all referring to Researcher behavior — confirmed via SPEC-DEEP-002/spec.md L281 that REQ-DEEP2-002 is the *orchestrator sequence*, not Researcher single-shot. The correct reference is REQ-DEEP2-005 per L283.
- Re-checked exclusions vs included REQs: Exclusion "Per-node cost cap → DEEP-004" is consistent with REQ-DEEP3-007 (which scopes only tree-wide token cap). No conflict.
- Re-checked the dependency lineup: depends_on = [DEEP-001, DEEP-002, SYN-001, SYN-004, LLM-001, OBS-001, FAN-001, CORE-001]. All 8 SPECs exist in `.moai/specs/`. SYN-001 (synthesis sidecar pattern) and SYN-004 (SSE wire format) are real and used in the way described. No fictional dependencies.
- Re-checked the `blocks: [SPEC-DEEP-004]` claim: SPEC-DEEP-004/spec.md L15 has DEEP-003 in its depends_on. Bi-directional consistency confirmed at frontmatter level. Semantic claim about `Node.TokensUsed` consumption is overstated → flagged as D4 (MAJOR).
- Re-verified frontmatter: 15 fields present; values type-correct; consistent with DEEP-002 v0.1.1 conventions.

No new BLOCKER defects discovered in this pass. The first-pass findings stand. The audit's conclusions are stable.

---

## Regression Check

N/A — iteration 1 (no prior report).

---

## Recommendation

**Required for PASS (BLOCKER fixes, must be addressed):**

1. **Fix D1** — Replace all 5 occurrences of "DEEP-002 REQ-002" referring to Researcher with "DEEP-002 REQ-005" at spec.md L98, L103, L129, L162–163, L204. Audit `spec-compact.md` and `acceptance.md` for the same substring.
2. **Fix D2** — Resolve the `depth=0` contradiction. Recommended: extend REQ-005 carve-out in REQ-DEEP3-002 to include `depth=0` (so REQ-002 reads "`breadth=0` AND `depth=0`은 별도 처리"), update acceptance.md Scenario 5.2 to drop the `depth=0 → HTTP 400` assertion, and rename Scenario 5.6 → "breadth=0 OR depth=0 fallback" with both cases tested.
3. **Fix D3** — Replace "[DEEP-002 REQ-006, FlattenedClaim 소비]" at spec.md L104 with a correct attribution. Suggested: "[DEEP-002 Writer agent; FlattenedClaim contract introduced by REQ-DEEP3-010]".

**Strongly recommended (MAJOR fixes, will be re-audited in iteration 2):**

4. **Fix D4** — Soften §6.2 claim or coordinate with DEEP-004 author to add the consumption contract on DEEP-004's side.
5. **Fix D5** — Either pin the migration tool/numbering scheme in §1.1 or move to §8 Open Questions.
6. **Fix D6** — Add a root-node seed clause to REQ-DEEP3-006.
7. **Fix D7** — Promote reload-and-reclassify behavior to a functional REQ (extend REQ-DEEP3-011 or add REQ-DEEP3-013).

**Optional polish (MINOR/NIT, not blocking):**

8. Apply D8–D15 fixes as editorial improvements.

After fixes, re-submit for iteration 2 audit. The structural foundation (EARS compliance, traceability matrix, NFR quantification, exclusions specificity, frontmatter completeness) is strong — defects are corrective rather than rewriting.

---

*End of audit report — iteration 1/3.*
