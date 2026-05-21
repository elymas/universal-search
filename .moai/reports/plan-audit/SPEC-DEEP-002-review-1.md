# Plan Audit: SPEC-DEEP-002 (Iteration 1/3)

Date: 2026-05-21
Auditor: plan-auditor (adversarial mode)
Harness: standard
Context Isolation: enforced (no author reasoning consumed)

## Verdict: PASS

## Summary

SPEC-DEEP-002 passes the adversarial audit with one MAJOR EARS anti-pattern finding (REQ-DEEP2-009 conflates two distinct Unwanted conditions in a single REQ) and several MINOR findings. All must-pass dimensions (A EARS, B Traceability, C Internal Consistency, E Pinning, F Risk Resolution, H Anti-patterns) clear the threshold. Risk resolution from research.md §7 is exhaustive (RDC-1 through RDC-10 map 1:1 to risks). Pinned architectural decisions deviate from research.md §1 in exactly one place (decision #7 substitutes `verifier_result` for `final_token`), and this deviation is explicitly documented in §8 Exclusions — not silent.

## Findings by Severity

### BLOCKER (must-fix to PASS)

None.

### MAJOR (should-fix; FAIL if 2+)

- [M1] spec.md:L256 — REQ-DEEP2-009 combines two structurally distinct Unwanted patterns into a single requirement: (a) "IF Writer is invoked the maximum allowed times AND Verifier still returns uncited_sentences_count > 0..." and (b) "Similarly, IF Researcher, Reviewer, or Verifier ... returns a non-recoverable error...". These are separate IF preconditions with separate response paths and separate test groupings (TestMaxRetryExhaustionReturns503 vs TestResearcherErrorAbortsAndReturns503 / TestVerifierErrorAbortsAndReturns503). Per EARS anti-pattern rules they should be split into REQ-DEEP2-009a (max-retry exhaustion) and REQ-DEEP2-009b (non-Verifier agent error). Recommended fix: split into two REQs in next revision; preserve test names. Not blocking because behavior is unambiguous and test catalog already covers both branches independently.

### MINOR (advisory; do not block)

- [N1] spec.md:L245 — REQ-DEEP2-001 (Ubiquitous) uses Korean indicative mood ("도입한다", "처리한다", "이다") rather than the explicit "SHALL" verb used by REQ-002 through REQ-012. Semantically equivalent in Korean SPEC convention, but inconsistent with the rest of the document and weaker for adversarial EARS enforcement. Recommend adding "SHALL" verbs explicitly: e.g., "`cmd/usearch-api` SHALL introduce a `?mode=` query parameter ... `mode=agents` SHALL route to ...".

- [N2] spec.md:L107-110 vs research.md:L17 — Pinned architectural decision #7 in spec.md §1.1 lists SSE event types `(agent_started, agent_completed, retry_started, verifier_result)` while research.md §1 decision #7 lists `(agent_started, agent_completed, retry_started, final_token)`. The deviation is intentional and explicitly documented in spec.md §8 Exclusions ("`final_token` SSE 이벤트 명칭이 research.md §6에 언급되었으나, 본 SPEC은 이 이름의 이벤트를 emit하지 않는다"). The §1.1 framing nonetheless implies verbatim pinning, which slightly misleads. Recommend a footnote at §1.1 #7 stating that the event-name refinement supersedes research.md.

- [N3] spec.md:L276 / acceptance.md:L132 — REQ-DEEP2-008 declares `outcome ∈ {success, error, timeout, retried}` for `usearch_deep_agent_duration_seconds`, but acceptance scenarios consistently use only `outcome="success"` and `outcome="error"`. The intended semantics of `outcome="retried"` and `outcome="timeout"` (which call sites emit them, and how they relate to the per-attempt histogram observation in retry scenarios) are not exercised by any acceptance scenario or named test. Recommend either adding a scenario that exercises `retried`/`timeout` labels or narrowing the bounded set in REQ-008.

- [N4] acceptance.md:L302 — Test name `TestEmptyFanoutOutconeCounterIncrements` is a typo. The corresponding test name in plan.md:L297 is `TestEmptyFanoutOutcomeCounterIncrements`. Fix the typo in acceptance.md to match plan.md.

- [N5] spec.md:L256 (REQ-DEEP2-009) and spec.md:L257 (REQ-DEEP2-012) — Both REQs prescribe HTTP status code AND SSE event behavior in a single REQ text without clarifying that the HTTP 503 / 200 only applies to the JSON/buffered path, while the SSE path returns HTTP 200 with a terminal pipeline_failed event. acceptance.md:L156-160 (Scenario 3) clarifies this correctly, but the SPEC text alone is ambiguous about HTTP status on the SSE path. Recommend explicit clause: "On the SSE path, HTTP status is 200 with terminal `pipeline_failed` event; on the buffered/JSON path, HTTP 503 with error body."

- [N6] spec.md:L318-319 — Env-vars `DEEP_AGENT_WRITER_RETRY_DELAY_MS` and `DEEP_AGENT_VERIFIER_TIMEOUT_MS` are owned by REQ-DEEP2-003 and REQ-DEEP2-006 respectively (per §6 Owner column), but the REQ texts do not mention these knobs. Either mention the knobs in the REQ texts or add an NFR governing their semantics.

- [N7] spec.md:L246 (REQ-DEEP2-010) — Optional pattern preamble starts with "WHERE 요청 쿼리 파라미터가 ..." but the EARS Optional pattern canonical form is "WHERE [feature exists], the [system] SHALL [response]". The condition here describes a request-time predicate ("if the request has stream=false"), which is semantically Event-Driven rather than Optional. Consider reclassifying as Event-Driven ("WHEN ?stream=false is present OR Accept does not advertise text/event-stream, the handler SHALL fall back...") for stricter EARS alignment.

## Dimension-by-Dimension Score

| Dimension | Score | Notes |
|---|---|---|
| A. EARS Compliance | PASS | 11/12 REQs use explicit SHALL with correct preambles. REQ-001 uses Korean indicative mood (N1). REQ-010 pattern selection borderline (N7). No weasel words in normative text. |
| B. Traceability | PASS | All 12 REQs covered by acceptance scenarios per §5 index. All 8 `depends_on` SPECs exist on disk (DEEP-001, SYN-002, SYN-004, FAN-001, LLM-001, CORE-001, OBS-001, IR-001). All File Impact Map entries map to a REQ via §2.1 module classification. All §6 env-vars carry Owner column reference to a REQ. |
| C. Internal Consistency | PASS | Frontmatter versions all 0.1.0. spec-compact.md REQ summaries align with spec.md (compact, not verbatim — appropriate for the format). plan.md §2 milestone file list ⊆ spec.md §2 File Impact Map. plan.md §3 TDD catalog (57 tests) covers every REQ at least once. Exclusions in spec-compact.md align with spec.md §8 (modulo summarization). One typo (N4). |
| D. Completeness | PASS | 8 main + 4 edge = 12 acceptance scenarios (≥6). §8 Exclusions has ~15 entries (≥4). 5 requirement modules (≤5). HISTORY first entry 2026-05-21. All 8 required frontmatter fields present (id, version, status, created, updated, author, priority, issue_number). |
| E. Architectural Pinning | PASS | spec.md §1.1 lists 8 pinned decisions matching research.md §1 with one documented refinement (decision #7 event taxonomy, see N2). §8 Exclusions correctly defers per-user quota → DEEP-004, tree exploration → DEEP-003, LLM-as-judge → SPEC-EVAL-001, drill-down → DEEP-003, no new sidecars, no DEEP-001 modifications. |
| F. Risk Resolution | PASS | All 10 risks from research.md §7 addressed via RDC-1 through RDC-10 in spec.md §1.3. 8 resolved (a), 1 deferred (c) to DEEP-004, 1 left to implementation discretion (b). plan.md §4 has explicit Risk-to-Resolution matrix. No risks left without one of (a)/(b)/(c). |
| G. NFR Realism | PASS | NFR-DEEP2-001 p95 ≤ 60s is tight but explicitly scoped to "Verifier passes on first or second attempt" and max-retry exhaustion is exempted. Estimated serial Haiku+Haiku+Sonnet+Sonnet ≈ 26-45s on first pass, ≈ 51-70s with one retry — borderline but plausible. NFR-DEEP2-002 enumerates bounded label sets (agent=4, outcome=4, result=3) with no per-request-ID labels. Enum-like Go types enforce compile-time bound. |
| H. EARS Anti-patterns | PASS | 11/12 REQs follow single-pattern, single-condition structure. REQ-009 combines two distinct Unwanted conditions (M1). No implementation-detail REQs. No subjective testability ("good", "fast"). REQ-002 and REQ-007 have multiple SHALL clauses but all under a single coherent WHILE preamble — acceptable as composite state-driven invariants. |

## PASS / FAIL Threshold

- PASS = zero BLOCKER findings AND fewer than 2 MAJOR findings AND all must-pass dimensions (A, B, C, E, F, H) are PASS
- Actual: 0 BLOCKER, 1 MAJOR, all must-pass dimensions PASS → **PASS**

## Next Action

Proceed to Phase 2.5 (annotation cycle with user for plan refinement).

Recommended (non-blocking) revisions for next minor version bump (v0.1.1 or before /moai sync):
1. Split REQ-DEEP2-009 into REQ-DEEP2-009a (max-retry exhaustion) and REQ-DEEP2-009b (non-Verifier agent error) — addresses M1. Preserve existing test names; update §3.5 Acceptance index and §5 Acceptance Criteria Summary accordingly.
2. Add explicit "SHALL" verbs to REQ-DEEP2-001 — addresses N1.
3. Footnote in spec.md §1.1 #7 noting that `verifier_result` supersedes `final_token` from research.md §1 — addresses N2.
4. Fix typo `TestEmptyFanoutOutconeCounterIncrements → TestEmptyFanoutOutcomeCounterIncrements` in acceptance.md:L302 — addresses N4.
5. Clarify HTTP status semantics on SSE vs JSON path in REQ-DEEP2-009 and REQ-DEEP2-012 — addresses N5.
6. Either narrow the `outcome` bounded set in REQ-DEEP2-008 or add scenarios exercising `outcome=retried` and `outcome=timeout` — addresses N3.
7. Reference `DEEP_AGENT_WRITER_RETRY_DELAY_MS` and `DEEP_AGENT_VERIFIER_TIMEOUT_MS` in their owning REQ texts — addresses N6.
8. Reclassify REQ-DEEP2-010 as Event-Driven or justify Optional choice — addresses N7.

None of these are blocking. Implementation may proceed against the v0.1.0 spec as-is.
