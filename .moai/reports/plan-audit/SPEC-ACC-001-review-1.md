# SPEC Review Report: SPEC-ACC-001
Iteration: 1/3
Verdict: PASS-WITH-FINDINGS
Overall Score: 0.86

> Reasoning context ignored per M1 Context Isolation. The author's summary
> passed in the invocation was NOT treated as evidence; every claim was
> re-verified against the four SPEC files and the live repository.

## Must-Pass Results

- **[PASS] MP-1 REQ number consistency**: IDs are `010, 011, 012, 013, 020, 021, 022, 030, 031, 040` (spec.md:L282-L291). No duplicates; consistent 3-digit zero-padding. The 10/20/30 decade grouping (gaps 014-019, 023-029, 032-039) is a **deliberate concern-grouped scheme**, not accidental omission — declared at spec.md:L71 ("grouped in the 10/20/30 numbering scheme") and consistent with the sibling SPEC-CACHE-001 convention. This is the intended use of grouped EARS numbering, not an MP-1 violation.
- **[PASS] MP-2 EARS format compliance**: All 10 REQs carry inline REQ-IDs, all use `SHALL`, none contain double-negatives, each matches an EARS surface form (spec.md:L282-L291). Ubiquitous: 010, 013. Event-Driven: 011, 012, 020, 030. State-Driven: 021. Optional: 022. Unwanted: 031, 040. **Caveat (see D3)**: REQ-ACC-022's `WHERE` is used for a Verdict-value state condition, not an optional *feature* — semantically off-pattern though syntactically valid. Does not fail MP-2.
- **[PASS] MP-3 YAML frontmatter validity**: `id` (L2), `version` 0.1.0 (L4), `status` draft (L6), `priority` P1 (L7), `created`/`updated` (L11-L12) all present and correctly typed. The project convention uses `created`/`updated` (verified against SPEC-CACHE-001:L11-12 and SPEC-API-001), NOT the generic `created_at`; priority uses the project's `P0/P1/P3` scheme. Frontmatter is byte-shape identical to the implemented SPEC-CACHE-001. `labels` is absent (present in ADP-010/CLI-003 but absent in CACHE-001/API-001) — optional in this project; noted as minor (D6).
- **[N/A] MP-4 Section 22 language neutrality**: N/A — single-project Go SPEC. The 16-language tooling rule does not apply. Note: the SPEC's own "No-Site-Name rule" (§2.3, REQ-ACC-040) imposes an analogous vendor-generic constraint and is enforced by a unit-test tripwire (spec.md:L291).

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.75 | 0.75 (minor ambiguity in 1-2 requirements) | §2.5 prose mapping contradicts the §6.3 truth table for one input row (D1); REQ-ACC-022 pattern mislabel (D3). Core REQs otherwise single-interpretation with explicit truth table (spec.md:L489-L498). |
| Completeness | 1.0 | 1.0 (all sections + frontmatter + exclusions) | HISTORY L20, Purpose L82, Scope L146, EARS L278, NFR L295, Acceptance L305, Technical Approach L403, Exclusions L561, Dependencies L632, Risks L664, Open Questions L678 (7 OQs w/ defaults+owners). |
| Testability | 0.75 | 0.75 (one AC not precisely binary) | Most ACs name a concrete Go test + assertion. NFR-ACC-002 absolute `==2` assertion is fragile (D2); NFR-ACC-001 pins amd64 on an arm64 dev host (D5). |
| Traceability | 1.0 | 1.0 (every REQ↔AC bidirectional) | Each REQ-ACC-0xx has a §5 acceptance block (L307-L381) AND an acceptance.md Given-When-Then scenario with a named test. 3 NFRs likewise. No orphan ACs, no uncovered REQs. |

## Cited-Code Accuracy (adversarial verification — all claims checked against repo)

Every current-code claim in spec.md HISTORY / §1 / §12 and research.md §1 was verified line-by-line. **All accurate:**

- `phase4_tls.go:46-51` standard `crypto/tls.Config` (TLS12/TLS13) — CONFIRMED (phase4_tls.go:46-51).
- `phase4_tls.go:19-29` `browserUserAgent` Chrome-130-macOS + `jsChallengePatterns` (4 substrings) — CONFIRMED.
- `containsJSChallenge` — research claims lines 144-153; actual lines 144-153 — CONFIRMED exactly.
- `phase3_get.go` `wafHeaders` (cf-ray, x-akamai-, x-served-by) — CONFIRMED (phase3_get.go:21-25).
- `phase3_get.go` `isWAFResponse` (403/503 AND header prefix) — CONFIRMED (phase3_get.go:88-96, 186-200).
- `types.go` binary `isWAF bool` on `PhaseAttempt` alongside `isTLSError`/`isJSChallenge` — CONFIRMED (types.go:75-80).
- `escalation.go` case 3 `prev.isTLSError || prev.isWAF` — CONFIRMED (escalation.go:31-33).
- No `cascade_waf.go` source; only `cascade_waf_test.go` fixture — CONFIRMED (directory listing).
- `errors.go` `FetchError` + `isTLSSignal/isWAFSignal/isJSChallengeSignal` — CONFIRMED (errors.go:59-72).
- `cascade.go` Phase-3 signal threading — CONFIRMED (cascade.go:250-273).
- `go.mod` has NO utls/CycleTLS/refraction-networking — CONFIRMED (grep empty).
- SPEC-CACHE-001 exists; its TLS-impersonation deferral to CACHE-001b is real (CACHE-001 spec.md:L289-291, L1345-1351). Minor: SPEC cites "CACHE-001 OQ §8.8"; CACHE-001 actually phrases it as "research OQ §8.8" — substantively correct, cosmetically imprecise.

**No invented files, symbols, or line numbers were found.** This is unusually disciplined research grounding.

## Defects Found

D1. spec.md:L260-L270 vs L489-L498 — **§2.5 prose Verdict mapping contradicts the §6.3 truth table.** §2.5 defines `VerdictStrongOK` as "L4 present AND NOT L1 AND NOT L3" (L260-261), which would classify input (L1=0, L2=1, L3=0, L4=1) as StrongOK; the §6.3 table classifies that same input as `VerdictUnknown` (row L498/L497, "tiny but real-selectored — ambiguous"). Additionally §2.5 defines `VerdictChallenge` as "L1 present OR L3 present" (L264), which textually subsumes the Blocked rows — only the table's L2 column disambiguates them, and that disambiguation is not stated in the §2.5 prose. REQ-ACC-020 binds to §6.3 ("per the AND-gated truth table in §6.3", L286), so the table is authoritative and the implementation is unambiguous; but the §2.5 summary is misleading and will mislead a reader/implementer who reads it first. — Severity: medium

D2. acceptance.md:L255 / spec.md:L300, L391 — **NFR-ACC-002 uses a fragile absolute assertion `serverRequestCount == 2`.** The cascade's earlier phases (Phase 1 index, Phase 2 robots/probe) can issue their own requests; acceptance.md L255 itself hedges "robots/HEAD per existing test discipline" in a parenthetical while the hard assertion remains `== 2`. The intent ("no NEW network ops vs the CACHE-001 baseline", spec.md:L300) should be expressed RELATIVE to the measured CACHE-001 baseline, not as a hard-coded count that may not match the real Phase 1→4 request total. As written the test risks being either wrong or silently tuned to pass. — Severity: medium

D3. spec.md:L288 — **REQ-ACC-022 mislabels a state condition as the EARS `Optional` pattern.** `WHERE` in EARS denotes an optional *feature* ("Where [feature exists], the system shall…"). Here `WHERE a Phase 3/4 response Verdict is VerdictWeakOK` is a runtime state/value condition, semantically a State-Driven (`WHILE`) or Event-Driven (`WHEN`) requirement. The "all five EARS patterns" claim (spec.md:L71) leans on this single REQ to cover Optional. Surface form is valid so MP-2 is not failed, but the pattern choice is incorrect and the five-pattern coverage claim is therefore weak. — Severity: medium

D4. spec.md:L156, L289-L290, L524-L535, L671 — **Avoid-list machinery (REQ-ACC-030/031) is near-zero-value in v0.1 — over-engineering / premature coupling.** The SPEC repeatedly concedes there is no impersonation library and thus effectively no candidate set to filter (§6.5 L532-535 "the avoid-list's practical effect in v0.1 is to RECORD the avoided fingerprints"; Risk row L671 "Avoid-list has no real effect in v0.1 … Likelihood High"). REQ-ACC-031's exhaustion-fallback test (L290) guards against emptying a set that is already ~1 element, making it largely tautological in v0.1. ~20% of the REQ surface (030, 031) plus the `TLSAvoidList` field, the filter step, and two+ tests build infrastructure whose real value is explicitly deferred to SPEC-CACHE-001b. Consider deferring the avoid-list to CACHE-001b (where the candidate set is actually born) and shipping only the two gaps that have v0.1 behavior (profiles + validator). Risk is low (it is forward-compatible data plumbing the author acknowledged), so this is a judgment recommendation, not a blocker. — Severity: major

D5. spec.md:L299, acceptance.md:L232 — **NFR-ACC-001 pins `amd64` for the benchmark gate.** The development/CI host here is darwin/arm64; an amd64-specific threshold may not be runnable on the actual gate machine. The 1 ms ceiling for a pure substring scan is trivially generous, so the risk is low, but the platform pin should be removed or made platform-agnostic. — Severity: minor

D6. spec.md:L2-L16 — **Frontmatter omits `labels`.** Present in SPEC-ADP-010 / SPEC-CLI-003 but absent in SPEC-CACHE-001 / SPEC-API-001, so it is optional in this project; harmless but recommended for discoverability/telemetry parity with newer SPECs. — Severity: minor

## Chain-of-Verification Pass

Second-look findings, by re-reading each section rather than spot-checking:

- **Re-read every REQ entry (010-040) end-to-end** (spec.md:L282-L291), not just the first few — confirms each has inline REQ-ID, SHALL, an EARS form, and a paired acceptance summary. The Optional mislabel (D3) was caught only on this full pass.
- **Re-checked REQ numbering end-to-end** — decade grouping confirmed deliberate (L71), not accidental; no dupes.
- **Verified traceability for every REQ AND every NFR** — cross-walked §3 → §5 → acceptance.md scenarios → named Go tests; the §8 TDD test table (L601-L623) and acceptance.md DoD checklist (L279-L302) provide a third confirming mapping. Zero orphans.
- **Enumerated all 16 truth-table input combinations** against §6.3 precedence (first-match-wins): the table is complete, deterministic, and internally non-contradictory. The defect is NOT in the table — it is the §2.5 prose summary diverging from the table (D1).
- **Re-read Exclusions (§2.2 + §7) for specificity, not just presence** — every excluded item names a concrete destination SPEC (CACHE-001a/b/c, EVAL-002, SEC-001) — specific, not vague.
- **Searched for inter-requirement contradictions** — none beyond the §2.5-vs-§6.3 doc mismatch (D1). REQ-ACC-021 (challenge 200 ≠ success) and REQ-ACC-022 (WeakOK = success) are complementary, not contradictory (disjoint Verdict values).
- **Adversarially verified ~12 cited file:line claims against the live repo** — all accurate; no invented symbols. This is the strongest part of the SPEC.

No new blocking defects surfaced in the second pass; D3 was upgraded from "unnoticed" to a logged finding.

## Recommendation (PASS-WITH-FINDINGS — annotate, do not re-architect)

No must-pass criterion failed and the cited code state is fully accurate, so this SPEC is approvable after the following non-blocking annotations are applied by manager-spec:

1. **D1 (medium):** Reconcile §2.5 prose with the §6.3 table. Either (a) rewrite the §2.5 `VerdictStrongOK`/`VerdictChallenge` bullets to add the `NOT L2` and L2-disambiguation conditions the table actually uses, or (b) replace the §2.5 prose mappings with a one-line pointer "§6.3 is authoritative; the bullets below are intuition only."
2. **D2 (medium):** Change NFR-ACC-002 from `serverRequestCount == 2` to a baseline-relative assertion ("equal to the CACHE-001 Phase-1→4 baseline request count for the same fixture"), and capture that baseline in the test. Remove the hedging parenthetical (acceptance.md:L255).
3. **D3 (medium):** Re-pattern REQ-ACC-022 as State-Driven (`WHILE a Phase 3/4 response Verdict is VerdictWeakOK, the cascade SHALL treat…`) OR Event-Driven, and either drop the "all five EARS patterns" claim (spec.md:L71) or find a genuine optional-feature requirement to carry the Optional pattern.
4. **D4 (major):** Decide explicitly: keep the avoid-list in ACC-001 (justify the v0.1 no-op as forward-compatible data plumbing in §6.5, which it partly does) OR defer REQ-ACC-030/031 + `TLSAvoidList` to SPEC-CACHE-001b alongside the impersonation library. Record the decision in Open Questions so it is a conscious choice rather than implicit scope.
5. **D5 (minor):** Remove the `amd64` pin from NFR-ACC-001 or make it host-agnostic.
6. **D6 (minor):** Add a `labels:` line to the frontmatter for parity with newer SPECs.

Rationale for PASS-WITH-FINDINGS: MP-1/MP-2/MP-3 pass with cited evidence; MP-4 N/A; Completeness and Traceability score 1.0; the research grounding is verifiably accurate to the line. The findings are doc-reconciliation, one test-assertion fix, one pattern relabel, and one scope judgment call — none require re-architecture.

---
*Audited 2026-06-17 by plan-auditor (iteration 1/3). Bias-prevention mechanisms M1-M6 active.*
