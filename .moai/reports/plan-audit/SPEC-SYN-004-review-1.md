# SPEC-SYN-004 Audit Report — Iteration 1

Verdict: FAIL

Reasoning context ignored per M1 Context Isolation. This audit was performed against `spec.md`, `plan.md`, `acceptance.md`, `research.md`, and `spec-compact.md` only.

---

## Must-Pass Summary

| MP | Criterion | Result | Evidence |
|----|-----------|--------|----------|
| MP-1 | REQ number consistency | PASS | spec.md L183-187: REQ-SYN4-001..005 sequential, no gaps, no duplicates |
| MP-2 | EARS pattern coverage | PASS (with MINOR caveat) | All 5 EARS pattern labels present; REQ-001 label "Ubiquitous" is borderline (carries WHEN trigger) — see D2 |
| MP-3 | Frontmatter validity | PASS | id/version/status/priority/title/depends_on/owner all present (spec.md L2-16). Frontmatter uses `created` (not `created_at`) and lacks `labels` — accepted as project convention (matches sibling SPECs in this repo); not a defect |
| MP-4 | Section 22 language neutrality | N/A | SPEC is single-language scoped (Go server, regex covers Korean+English data plane only); no multi-language tooling enumeration required |
| Critical | SYN-002 invariant preservation (audit task #8) | PASS | spec.md L34, L183-184 declares un-cited content NEVER reaches the wire as a HARD constraint. Acceptance `test_no_uncited_sentence_emitted` (acceptance.md §3.1 / spec.md L183) and §4.1.1 edge case explicitly test the un-cited path |

---

## Defect Table

| # | Severity | Dimension | File:Section | Defect | Suggested Fix |
|---|----------|-----------|--------------|--------|---------------|
| D1 | MAJOR | #9 Failure modes (slow client / backpressure) | spec.md §3 EARS table (L183-187), §2.1 row e (L105), §2.1 row g (L107) | The `write_timeout` outcome is a declared label value of `usearch_syn004_outcomes_total` and `SYN004_SSE_WRITE_TIMEOUT_MS` is a configured env var, but **no EARS REQ governs the slow-client / write-deadline behavior**. R1 (spec.md L353-356) names the mitigation, plan.md §6 R1 names the implementation, acceptance.md §4.5.3 describes the edge case — but the contract is undocumented at the requirement level. A v0 ship behavior (apply per-write deadline → cancel parent ctx → emit `event: error` → increment `outcome="write_timeout"`) has zero EARS coverage and therefore zero binding acceptance criterion in §3. | Add REQ-SYN4-006 (Unwanted): "IF a single SSE write to the client connection blocks longer than `SYN004_SSE_WRITE_TIMEOUT_MS` milliseconds (default 5000), THEN the server SHALL cancel the parent synthesis ctx, SHALL increment `usearch_syn004_outcomes_total{outcome="write_timeout"}` exactly once, SHALL attempt to emit one `event: error` payload before connection teardown, AND SHALL release heartbeat + watcher goroutines cleanly." Add a corresponding scenario in acceptance.md §3 (promote §4.5.3 from edge case to first-class scenario). |
| D2 | MINOR | #1 EARS compliance | spec.md L183 (REQ-SYN4-001) | Labeled "Ubiquitous" but the requirement contains a WHEN clause ("WHEN the request `Accept` header advertises `text/event-stream`"). Per audit M3 rubric, this is the Event-Driven pattern, not Ubiquitous. The other clauses (wire format conformance, citation invariant) ARE ubiquitous — the requirement multiplexes patterns. | Either (a) split into REQ-SYN4-001a Ubiquitous (wire format + citation invariant — applies to every SSE response) and REQ-SYN4-001b Event-Driven (WHEN Accept header advertises text/event-stream → SSE headers are emitted), or (b) relabel as Event-Driven and move the always-on invariants into a separate Ubiquitous REQ. |
| D3 | MINOR | #1/#2 Clarity & Testability | spec.md L183 (REQ-SYN4-001) | Single REQ contains four distinct testable claims: (i) headers Content-Type/Cache-Control/Connection, (ii) W3C wire-format conformance, (iii) HARD citation invariant, (iv) preservation of SYN-001 NFR-SYN-002 + SYN-002 REQ-SYN2-001. Failure of any one claim is a violation of REQ-SYN4-001, but traceability and test-isolation are weakened. | Decompose REQ-SYN4-001 into 2-3 narrower REQs. The HARD citation-invariant clause in particular merits its own REQ given its constitutional weight (it is the most-cited contract in the SPEC). |
| D4 | MINOR | #12 Concurrency safety | spec.md L186 (REQ-SYN4-004), plan.md §3 P0-D (L177-181) | REQ-SYN4-004 mandates "release all goroutines (heartbeat + main writer) cleanly", but plan.md §P0-D introduces a third goroutine (the `r.Context().Done()` watcher) that is not enumerated in the REQ. Acceptance §3.4 also covers heartbeat goroutine leak only — no explicit assertion on the watcher goroutine. | Update REQ-SYN4-004 to enumerate the three goroutines (main writer, heartbeat, disconnect watcher) and assert no-leak for all three. Update acceptance.md §3.4 `test_client_disconnect_releases_heartbeat_goroutine` to also cover the watcher goroutine (rename or add a parallel test). |
| D5 | MINOR | #2 Acceptance testability (race condition) | acceptance.md §4.5.2 (L313-316) | The "disconnect after `event: done` already written" race window mentions "REQ-SYN4-002 + REQ-SYN4-004 mutual exclusion" but no REQ formally states this mutual-exclusion invariant. This is a behavior that ships in v0 (counter never double-increments per request) but has no EARS-level guarantee. | Add to either REQ-SYN4-002 or REQ-SYN4-004 (or as a new NFR): "the outcome counter SHALL be incremented at most once per request lifecycle". Promote §4.5.2 to a Scenario H in §3. |
| D6 | MINOR | #2 Acceptance testability | acceptance.md §3.1 / §3.2 / §3.3 / §3.6 | Several scenarios assert "the response body emits exactly N `event: sentence` events", but the segmentation regex `[.!?。！？]\s+|[.!?。！？]$` has known edge cases (e.g. abbreviations: "Dr. Smith arrived [1]." would segment incorrectly). The SPEC inherits the regex verbatim from SYN-002 (research.md §4.2, L237). No acceptance scenario tests pathological abbreviation-rich text. | Add an edge case (acceptance.md §4.2.6): "Abbreviation-heavy text (`Dr. Smith confirmed [1]. Mr. Park noted [2].`) — segmenter behavior matches SYN-002 reference output exactly". Or explicitly defer to SYN-002 with a note: "segmentation correctness is owned by SYN-002; SYN-004 inherits its regex behavior verbatim." |

---

## Audit Dimension Scores (rubric-anchored)

| Dimension | Score | Band | Evidence |
|-----------|-------|------|----------|
| Clarity | 0.75 | Most REQs unambiguous; REQ-001 multiplexes claims (D2/D3) | spec.md L183-187 |
| Completeness | 0.75 | Slow-client behavior missing at REQ level (D1); all sections present | spec.md §3, §2.1 row e+g, §6 R1 |
| Testability | 0.75 | Most scenarios binary-testable with concrete thresholds (e.g. `SYN004_DISCONNECT_CANCEL_MS + 100 ms` jitter); D1 + D5 leave gaps | acceptance.md §3, §4 |
| Traceability | 0.90 | Every REQ has acceptance scenarios; every scenario maps to REQ; test file paths concrete | acceptance.md §3 each scenario header maps REQ IDs |

---

## Per-Audit-Task Findings

1. **EARS compliance** — All 5 patterns represented, REQ IDs unique. REQ-001 label mismatch noted (D2). PASS with MINOR.
2. **Acceptance testability** — Concrete G/W/T scenarios; jitter bounds and counter assertions are precise. Missing slow-client scenario at first-class level (D1). MOSTLY PASS.
3. **Exclusions** — Twelve genuinely out-of-scope items (spec.md L120-173) including WebSocket, token-streaming, Last-Event-ID, multi-subscriber broadcast, etc. PASS.
4. **Cross-doc consistency** — spec REQ → plan milestone (P0-A..E) → acceptance scenario alignment is tight. PASS.
5. **Delta markers** — [MODIFY], [NEW], [EXISTING — UNCHANGED] applied throughout §2.1 (L101-116). PASS.
6. **MX tag plan** — ANCHOR (StreamSynthesize + cmd/usearch-api entrypoint), WARN (writer goroutine concurrency with REASON), NOTE (heartbeat default) all enumerated in plan.md §3 P0-E (L210-220) and acceptance.md §6 (L364). PASS.
7. **Research grounding** — Verified file:line refs throughout: `cmd/usearch-api/main.go:1-40`, `internal/synthesis/client.go:39-100`, `services/researcher/src/researcher/synthesis.py:151-220`, `gateway.py:36-72`, `synthesis.py:66-118`. SPEC-SYN-001 and SPEC-SYN-002 contracts cited at line ranges. PASS.
8. **CRITICAL — SYN-002 invariant preservation** — Stated as `[HARD constraint]` (spec.md L183), encoded as REQ-SYN4-001 invariant clause, tested explicitly via `test_no_uncited_sentence_emitted` and `test_syn002_invariant_preserved_under_streaming`, plus property test NFR-SYN4-002 (b)(c). Acceptance §4.1.1 covers synthetic injection of an un-cited sentence and asserts strip-mode behavior. PASS.
9. **Failure modes** — Client disconnect (REQ-SYN4-004) testable with concrete deadline + jitter ✓; proxy buffering (REQ-SYN4-003 + Scenario F) verifiable via simulated proxy ✓; **slow-client / backpressure NOT REQ-encoded** (D1). PARTIAL.
10. **Pipeline integration** — IR-001 stub status honestly noted (spec.md L66-67, research.md §2.1), Python sidecar streaming gap honestly excluded (spec.md L141-147; research.md §6). PASS.
11. **Backward compat** — REQ-SYN4-005 explicit + 5 acceptance tests including `test_accept_text_html_falls_back_to_json`. PASS.
12. **Concurrency safety** — Heartbeat ctx-cancel ✓, mutex on Writer ✓, disconnect cancel deadline ✓; watcher goroutine not explicitly enumerated in REQ-SYN4-004 (D4). MOSTLY PASS.

---

## Chain-of-Verification Pass

Re-read of REQ table after initial scan confirmed:
- All 5 REQs verified end-to-end (not spot-checked).
- Traceability verified: every REQ has 4-6 named acceptance tests in spec.md §3 right column AND a matching scenario header in acceptance.md §3.
- Exclusions section re-read for specificity — all 12 entries name the alternative SPEC or rationale; none vague.
- Searched for `write_timeout` references across spec.md and acceptance.md to confirm D1 (no EARS REQ governs it). Confirmed: only metric label, plan risk row, and acceptance edge case — never a SHALL clause.
- Scanned for contradictions between REQs: REQ-001 declares Accept-header trigger for SSE, REQ-005 declares fallback for non-SSE — these are mutually exclusive and exhaustive. No contradiction.

Second-pass discovery: D6 (abbreviation edge case in segmentation) added on re-read of §4.2.

---

## Regression Check

N/A — iteration 1.

---

## Recommendation

This SPEC is high-quality work but FAILS audit on the strict criterion (PASS = 0 BLOCKER + 0 MAJOR). The single MAJOR (D1) is straightforward to remediate:

1. Add **REQ-SYN4-006** (Unwanted, P0): governs slow-client write deadline behavior. Use language analogous to REQ-SYN4-004 with `SYN004_SSE_WRITE_TIMEOUT_MS` as the threshold.
2. Promote acceptance.md §4.5.3 to a first-class **Scenario H** under §3, with G/W/T structure parallel to Scenario D (client disconnect).
3. Address MINOR defects D2-D6 in the same revision pass:
   - Split REQ-SYN4-001 into focused sub-REQs (D2/D3).
   - Enumerate all three goroutines in REQ-SYN4-004 cleanup clause (D4).
   - Add at-most-once outcome counter invariant to either REQ-SYN4-002 or as NFR (D5).
   - Add abbreviation edge case to acceptance §4.2 or explicitly inherit segmentation correctness from SYN-002 (D6).

After remediation, the SPEC is expected to PASS iteration 2 cleanly. The foundation is sound: SYN-002 invariant preservation is treated with appropriate constitutional weight, exclusions are disciplined, research grounding is verified at file:line precision, and concurrency hazards (heartbeat, disconnect, mutex) are surfaced at REQ level.

---

*End of audit report — iteration 1.*
