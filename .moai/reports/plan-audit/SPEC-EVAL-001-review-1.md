# SPEC Review Report: SPEC-EVAL-001
Iteration: 1/3
Verdict: PASS-WITH-FINDINGS
Overall Score: 0.88

> Reasoning context (amendment self-report) was treated as a claim to verify, NOT as evidence. All three stale-fix claims were checked against live code per M1 Context Isolation; the spec's own assertions were not trusted.

## Must-Pass Results

- [PASS] MP-1 REQ number consistency: REQ-EVAL1-001 … REQ-EVAL1-011, 3-digit zero-padded, sequential, no gaps, no duplicates. Tables at spec.md:339-341 (001-003), :347-350 (004-007), :356-359 (008-011). Count "11 EARS REQs" matches (spec.md:188).
- [PASS] MP-2 EARS format compliance: 10/11 active REQs match a valid EARS pattern with correct label (Ubiquitous 001/002/004/005/008; Optional 003/011; State-Driven 006; Event-Driven 007/009). Evidence: spec.md:339 "The system SHALL maintain…", spec.md:349 "WHILE the judge model is unavailable… the runner SHALL…", spec.md:350 "WHEN the judge model returns…". Lone exception is REQ-EVAL1-010, labeled "Ubiquitous" but worded "The system WILL (in V1.1)…" (spec.md:358) — future-tense WILL, not normative SHALL. Because 010 is explicitly DEFERRED to V1.1 with zero V1 acceptance obligation (carries "— (deferred)"), it imposes no active requirement; treated as a documented exception, downgraded to a minor finding rather than a firewall failure.
- [PASS] MP-3 YAML frontmatter validity: id (string `SPEC-EVAL-001`), version (string `0.2.0`), status (string `draft`), priority (string `P1`), created (`2026-05-22`), updated (`2026-05-29`), plus author/methodology/coverage_target/depends_on/blocks (spec.md:2-17). Judged against the in-use MoAI project SPEC schema (identical to design/constitution.md and sibling SPECs), NOT the generic rubric: this project uses `created`/`updated` (not `created_at`) and `milestone`/`related`/`owner` in place of a `labels` field. All schema fields present, correctly typed, internally consistent. Field-name divergence from the generic rubric is informational, not a defect.
- [N/A] MP-4 Section 22 language neutrality: N/A — single-project (universal-search), not multi-language tooling content. The only tool names (gopls/pyright/tsserver) appear in an unrelated rules file, not this SPEC. The SPEC's "16 languages" is not in scope here.

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.90 | 0.75–1.0 | Requirements are single-interpretation with explicit field schemas (spec.md:339 query record schema; :347 judge I/O contract; :356 exit-code mapping). Minor ambiguity: NFR-003 monthly-cost prose references deferred nightly runs (spec.md:369). |
| Completeness | 0.95 | 1.0 | All sections present: HISTORY (:22), Overview/WHY (:205, :222), WHAT (:248), EARS REQUIREMENTS (:333), NFRs (:363), Exclusions w/ 10+ specific entries (:375-477), Acceptance index (:479), Dependencies (:507), Files (:583). |
| Testability | 0.88 | 0.75–1.0 | Every active AC is binary-testable with named tests (spec.md:339-359 acceptance summaries; acceptance.md AC-001..016 Given/When/Then). Thresholds concrete (≥0.85 mean, ≥0.50 floor, ±0.02/±0.05 variance, ≤$0.50, ≤15min, exit 0/1/2/3). No weasel words in normative ACs. |
| Traceability | 0.85 | 0.75–1.0 | Every active REQ + every NFR maps to ≥1 AC and ≥1 plan phase; coverage matrix complete (acceptance.md:373-390). One wrong cross-reference (AC-005 cites §5.9; see D3 below). No orphan REQ, no AC referencing a deleted REQ. |

## Stale-Fix Verification (against live code)

- **Fix 1 — CONFIRMED-RESOLVED.** Live code: `services/researcher/src/researcher/faithfulness_endpoint.py:1` exists and self-attributes to "SPEC-DEEP-002 REQ-DEEP2-006". The path `faithfulness.py` does NOT exist at the researcher location; it exists only at an unrelated service `services/storm/src/storm/faithfulness.py` (verified via filesystem). Spec correctly references `faithfulness_endpoint.py` + `internal/synthesis/faithfulness.go` (`CheckFaithfulness`, faithfulness.go:40) and added SPEC-DEEP-002 to `depends_on` (spec.md:15, :518-528).
- **Fix 2 — CONFIRMED-RESOLVED.** Live code: `synthesis.py:24` `_MARKER_RE = re.compile(r"\[(\d+)\]")`; `faithfulness_endpoint.py:47` `re.split(r"(?<=[.!?])\s+", text.strip())` — ASCII/Latin punctuation only, NO CJK boundary handling. Spec correctly states there is NO CJK segmentation in the structural checker and frames Korean segmentation as an EVAL-001-side design requirement (spec.md:99-112, REQ-EVAL1-005(a) at :348, plan.md:146-153).
- **Fix 3 — CONFIRMED-RESOLVED.** Live code: `internal/synthesis/types.go:38-50` `Result{… Text string; Citations []Citation …}`; :52-58 `Citation{Marker int; DocID string; URL string; Title string}`. Spec uses exactly `synthesis.Result{Text, Citations}` + `Citation{Marker int, DocID, URL, Title}` (spec.md:308-310, :513). The rejected `SynthesizeResponse` is the Python-side Pydantic model (synthesis.py:18), correctly NOT used for the Go consumer contract.

## Defects Found

- **D1.** spec.md:188 — Priority tally is miscounted. The summary states "11 EARS REQs (4 × P0 + 5 × P1 + 2 × P2)" but the actual REQ priority labels are 5×P0 (001,002,004,005,008), 4×P1 (003,006,007,009), 1×P2 (011), + 1 deferred (010). The summary undercounts P0 and overcounts P1/P2. — Severity: minor
- **D2.** spec.md:369 (NFR-EVAL1-003) — Scope-reduction residue inside the contract: the monthly-cost projection still reads "~100 PRs/month + 30 nightly runs = $0.50 × 130 ≈ $65/month", but the nightly cron is DEFERRED to V1.1 (HISTORY D9). The informational figure now overstates V1 cost and references a deferred capability within the spec body itself (not just research.md). — Severity: minor
- **D3.** acceptance.md:118 — Wrong scenario cross-reference. AC-005 ("Aggregate pass", REQ-008/009) states "Maps to scenarios §5.5, §5.9 in spec.md (eval.yml trigger)". In spec.md §5, §5.9 is "Override applied" (REQ-EVAL1-003, spec.md:496), not an eval.yml-trigger scenario; the trigger scenario is §5.5 (spec.md:492). §5.9 is independently and correctly covered by AC-009. AC-005 should cite §5.5 only. No orphan results, but the mapping is factually incorrect. — Severity: minor
- **D4.** spec.md:358 — REQ-EVAL1-010 is labeled "Ubiquitous" yet worded "The system WILL (in V1.1)…" (future tense, non-normative WILL). A Ubiquitous EARS requirement must read "The system SHALL…". Acceptable as a deferred/forward-compat marker but the pattern label is inconsistent with the verb. Recommend relabeling to "(deferred)" with no EARS pattern claim. — Severity: minor
- **D5.** research.md:614, :388-391, :329 — research.md is stale (self-noted by the amendment). Line 614 still lists `services/researcher/src/researcher/faithfulness.py` (the nonexistent path); §6.3 (:388) still presents the nightly cron as "chosen" rather than deferred; cost model (:329) still assumes "30 nightly runs". (Note: research.md:126 `deepeval/metrics/faithfulness.py` is the DeepEval library's own module — a legitimate reference, NOT a codebase-path error.) research.md is context, not contract, so this does not block — but it should be refreshed before/with implementation to avoid misleading the run-phase agent. — Severity: minor

## Chain-of-Verification Pass

Second-look findings: Re-read end-to-end rather than spot-checking. New defects surfaced that the first pass missed:
- D1 (priority tally miscount at spec.md:188) — only caught by recomputing P0/P1/P2 counts directly from the §2 tables instead of trusting the summary line.
- D2 (NFR-003 nightly-cost residue inside the spec) — initially I had attributed all nightly residue to research.md; re-reading NFR-EVAL1-003 verbatim revealed the deferred-nightly assumption also lives in the active spec body.

Verified by re-reading: all 11 REQ entries individually (not skimmed); REQ sequencing end-to-end (001→011); traceability for every REQ and NFR via the coverage matrix (acceptance.md:373-390) cross-checked against §2 acceptance summaries; Exclusions section for specificity (10+ entries each with a named destination SPEC + rationale, spec.md:381-477); scope-reduction coherence across spec ↔ acceptance ↔ plan (corpus 50-80 with no surviving ≥200 assertion; nightly deferred in REQ-010/§5.11/AC-011/plan Phase 6 task 2/EC-003; override automation→manual in REQ-003/D8/EC-003). No contradiction found between active requirements. No orphan REQ; no AC references a deleted REQ.

## Scope-Reduction Coherence: clean (with 2 minor residues)

- Corpus ≥200→50-80: COHERENT. REQ-002 "V1 target 50-80 docs", TestCorpusSize "≥50 V1 floor", §7.1 "50-80 fixtures", ≥200 reframed as post-V1 goal. No surviving test/assertion requires ≥200.
- Nightly→V1.1 deferred: COHERENT in the contract surface (REQ-010 deferred, §5.11 removed, AC-011 deferred, EC-003 N/A, plan Phase 6 task 2 deferred), with deferral rationale stated (HISTORY D9). Residue: NFR-003 cost prose (D2) + research.md §6.3 (D5).
- Override automation→manual: COHERENT. REQ-003 manual list + simple cap≤5, no auto-expiry (`expires_at` advisory), HISTORY D8 updated, EC-003 marked N/A. Internally consistent.

## DeepEval Determinism: adequate

Determinism controls are specified, not hand-waved: `temperature=0, top_p=1, seed=42` passed through LiteLLM and FROZEN at the SPEC level (REQ-EVAL1-004 spec.md:347, HISTORY D7 spec.md:140-147, NFR-EVAL1-001 spec.md:367). A calibration/variance gate exists: ±0.02 → warn, ±0.05 → block CI (exit 2). A judge-bias gate exists: plan Phase 2 calibration sub-phase compares Haiku 4.5 vs Sonnet 4.5 on the 15 Korean queries and blocks Phase 3 if aggregate gap > 0.10 (plan.md:156-160, :491). Judge-unavailability is handled (null≠zero, exit 2, REQ-006). Residual risk (advisory, not blocking): seed=42 + temperature=0 does not fully guarantee determinism across provider-side model updates; the ±0.02/±0.05 tolerance band is the correct mitigation and is present.

## TDD Soundness: coherent

RED-GREEN-REFACTOR is applied per phase with named failing tests first (plan.md Phase 2 :120 RED → :136 GREEN; Phase 3 :176 RED → :199 GREEN; Phase 4/5 similar). The cross-language Go↔Python HTTP boundary is mockable in unit tests (deepeval call mocked, plan.md:46-50) with integration coverage deferred to the final phase that needs the real sidecar. 36 automated unit tests inventoried (plan.md:436-455). Under-specification (advisory): the CI sidecar bootstrap (plan.md:337-338 "boot researcher service in background") does not specify a readiness/health-check wait before the runner fires — a real flake source for cross-language CI. Recommend pinning a health-poll-until-ready step. Not blocking for Phase 0 gate.

## Traceability Gaps

- Orphan REQs: NONE. Every active REQ (001-009, 011) and every NFR (001-005) maps to ≥1 AC and ≥1 plan phase. REQ-010 is deferred and correctly carries no V1 AC obligation (AC-011 deferred).
- ACs referencing deleted REQs: NONE.
- Scenario-count consistency: CONSISTENT. spec §5 = 16 indexed, §5.11 explicitly removed → 15 active. acceptance.md = AC-001..016, AC-011 deferred → 15 active. DoD checklist (acceptance.md:352-353) states "15 active" and "§5.1..§5.10, §5.12..§5.16". All three sources agree.
- One incorrect cross-reference (D3, AC-005 → §5.9) that does not create an orphan.

## research.md Staleness: NOTED (minor)

Confirmed stale (D5): research.md:614 (nonexistent `faithfulness.py` path), §6.3 (nightly "chosen" not deferred), cost model assuming 30 nightly runs. Context-not-contract, so non-blocking; the amendment correctly self-disclosed this. Should be refreshed alongside implementation so the run-phase agent is not misled.

## Recommendation

PASS-WITH-FINDINGS. All three stale-codebase fixes are CONFIRMED-RESOLVED against live code, all scope reductions are coherent in the contract documents, traceability has no orphans and no dangling references, EARS/frontmatter/REQ-sequencing all pass the must-pass firewall, and DeepEval determinism + TDD strategy are adequately specified. The five findings are all minor (cosmetic/residue), none alters an active requirement or blocks the run phase.

status_transition_recommendation: **approve** (draft → approved). The Phase 0 gate is satisfied. The five minor findings (D1-D5) are recommended cleanups, best folded into the existing annotation cycle or a fast v0.2.1 touch-up; they are not preconditions for implementation.

### must_fix_before_implementation (ordered)
(empty — none block implementation)

Recommended-but-optional cleanups, in priority order:
1. D1 — correct the priority tally at spec.md:188 to "5 × P0 + 4 × P1 + 1 × P2 + 1 deferred".
2. D2 — reword NFR-EVAL1-003 (spec.md:369) monthly-cost line to drop the deferred 30-nightly-runs assumption (V1 = PR runs only).
3. D3 — fix AC-005 cross-reference (acceptance.md:118) from "§5.5, §5.9" to "§5.5".
4. D4 — relabel REQ-EVAL1-010 (spec.md:358) pattern from "Ubiquitous" to "(deferred)" or restate as SHALL when V1.1-rescoped.
5. D5 — refresh research.md (faithfulness.py path, §6.3 nightly status, nightly cost) before run phase.

🗿 MoAI <email@mo.ai.kr>
