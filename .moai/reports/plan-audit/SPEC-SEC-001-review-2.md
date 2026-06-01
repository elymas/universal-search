# SPEC Review Report: SPEC-SEC-001
Iteration: 2/3
Verdict: **PASS-WITH-FINDINGS**
Overall Score: 0.88

> Reasoning context from the SPEC author / amendment self-report was ignored per M1 Context Isolation. Each prior finding was re-verified against the LIVE codebase (`internal/audit/chain.go`, `deploy/postgres/migrations/0003_audit_events.sql`, `internal/access/ssrf.go`, `internal/access/types.go`) and the LIVE v0.2.0 spec/plan/acceptance files — NOT against the amendment's claims.

Adversarial stance maintained: I assumed the amendment was incomplete and/or had introduced regressions until proven otherwise. Both prior CRITICALs are genuinely resolved against ground truth. All five MAJORs are resolved in the authoritative contract surfaces. Two NEW minor regressions were introduced by the amendment (stale count leftovers), and one pre-existing P2 minor remains. None block status transition.

---

## Must-Pass Results

- **[PASS] MP-1 REQ number consistency**: `grep -oE "REQ-SEC-[0-9]{3}" | sort -u` → REQ-SEC-001…018 (18 distinct, sequential, no gaps/dups) + REQ-SEC-005a sub-ID. NFR-SEC-001…007 complete. AC-001…015 sequential. No renumbering orphaned any acceptance scenario (cross-checked acceptance REQ refs = full 001-018 + 005a set). spec.md:452-495.
- **[PASS] MP-2 EARS format compliance**: All REQs use SHALL + EARS keyword; the three prior type-label mismatches are corrected (REQ-SEC-003/012 = Conditional IF-THEN spec.md:454,479; REQ-SEC-018 = Ubiquitous SHALL NOT spec.md:495). Rubric band ~0.95.
- **[PASS] MP-3 YAML frontmatter validity**: `id`, `version: 0.2.0`, `status: draft`, `created: 2026-05-22`, `priority: P0` present (spec.md:2-8). The project schema uses `milestone`/`owner`/`methodology` in place of `labels`, and `created` in place of `created_at` — recorded as a schema-convention mismatch (Minor), consistent with review-1's treatment. Not a substantive blocker; not headline.
- **[N/A] MP-4 Section 22 language neutrality**: N/A — single-language (Go) SPEC. The review-1 neutrality-class concern was the AUTH-003 chain-model conflict, tracked under C1 and now resolved.

---

## Category Scores (0.0–1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.90 | 0.75–1.0 | Core self-contradiction (ghost path / signature) is gone. Document now reads consistently on the central refactor. Residual: plan.md:122 "9개 test" vs plan.md:236/259/606 "22개" internal disagreement (N2). |
| Completeness | 0.90 | 0.75–1.0 | §5 expanded 15→20 scenarios (spec.md:600-621) covering REQ-SEC-006 (§5.16) + NFR-SEC-001/002/003/005 (§5.17-5.20). Residual: acceptance DoD still references §5.1..§5.15 (N1); REQ-SEC-006 has no plan phase task (m4, pre-existing P2). |
| Testability | 0.88 | 0.75–1.0 | "22 tests" + `fopts`/`RedirectMaxHops` now correct in REQ-SEC-007 (spec.md:469), §5.4 (spec.md:605), AC-004 (acceptance.md:100-109), DoD (acceptance.md:398). The DDD PRESERVE premise is now factually correct. Residual: plan.md:122 leftover "9" undercounts the Phase-1 characterization target (N2). |
| Traceability | 0.92 | 0.75–1.0 | `depends_on` now includes SPEC-DEP-001 + SPEC-SYN-002 (spec.md:15). Every REQ has ≥1 AC (acceptance matrix acceptance.md:413+). C1 phantom-migration trace removed; Phase 5 traces to real `internal/audit/chain.go` + `0003_audit_events.sql`. |

---

## Prior Findings Status (verified against ground truth)

**C1 — AUTH-003 chain reinvention → CONFIRMED-RESOLVED.**
Ground-truth verification:
- `internal/audit/chain.go:29` defines `ComputeThisHash(prevHash, evt) = SHA256(prevHash + canonical_json)`; `VerifyChain` at :102; per-tenant `AcquireAdvisoryLock` at :124. The chain the spec now claims to reuse genuinely exists.
- `deploy/postgres/migrations/0003_audit_events.sql:33-34` defines `prev_hash TEXT, this_hash TEXT`; append-only UPDATE/DELETE triggers at :56-72. The "ADD prev_hash" migration the v0.1.0 plan proposed is genuinely redundant.
- REQ-SEC-017 is narrowed to "emit the SEC-001 7-type taxonomy INTO the existing AUTH-003 chain/table" — spec.md:332-337, §1.1 (spec.md:378 `internal/security/events/` = "7-type logger", NOT a chain), Phase 5 (plan.md:273-336).
- Phantom artifacts REMOVED: `ops/migrations/20260522_audit_prev_hash.sql`, `internal/security/events/merkle.go`, `.github/workflows/audit-verify.yml`, `cmd/audit-verify/main.go`, `BenchmarkMerkleVerify1M` — all struck-through (`~~...~~`) in plan.md:291-299 and stated REMOVED in spec.md:338-340, :50. No live reinvention language remains (the only mentions are in HISTORY/REMOVED markers describing the fix).
- Verify-budget reconciled: the invented ≤30s/1M is gone; NFR-SEC-004 (spec.md:506) now inherits AUTH-003 NFR-AUTH3-007's ≤30min / 600K-2M / 90d. acceptance.md:302 confirms "NO separate SEC-001 1M-row / 30s benchmark."
- Chain semantics aligned to per-(tenant, event_type), `audit.hash_chain.enabled` default false (spec.md:48-49), matching chain.go.

**C2 — git history-rewrite guard → CONFIRMED-RESOLVED.**
REQ-SEC-005a (spec.md:462) is a new Unwanted-pattern requirement specifying all five guards: (1) named human approval gate, (2) `refs/backup/<date>` snapshot + mirror clone, (3) staging/dry-run validation, (4) tested rollback, (5) team coordination notice — with explicit "BLOCKED if any guard missing." REQ-SEC-005 (spec.md:461) now gates the destructive path on REQ-SEC-005a. §5.3 (spec.md:604) and AC-003 verify all five. No destructive step left unguarded.

**M1 — `internal/cache/access/` ghost path → CONFIRMED-RESOLVED.**
`grep "internal/cache/access"` across all three files returns only spec.md:60 (a HISTORY line describing the correction). §1.1 (spec.md:374,379), §7 "Files to Modify", and REQ-SEC-007 all use the real `internal/access/`. Live dir confirmed: `internal/access/` exists with `ssrf.go`/`dialer.go`; no `internal/cache/access/`.

**M2 — REQ-SEC-007 signature/path → CONFIRMED-RESOLVED.**
Ground truth: `internal/access/ssrf.go:31` `validateHost(ctx, u, opts, fopts FetchOptions)`; :61 `validateRedirect(next, opts, fopts, hopCount)`; `RedirectMaxHops` is the real option (ssrf.go:62); `FetchOptions.AllowPrivateNetworks` is load-bearing (types.go:17, used in OR-guard ssrf.go:32). REQ-SEC-007 (spec.md:469) now PRESERVES `fopts FetchOptions`, keeps `RedirectMaxHops` ("NOT renamed"), and explicitly names the two `fopts` tests. The wrong `MaxRedirects` rename is gone (only negative mentions "NOT renamed to MaxRedirects" remain: acceptance.md:109, plan.md:232, spec.md:63). plan.md no longer says "signature 유지 (caller 변경 없음)" — plan.md:254 explicitly retracts the v0.1.0 contradiction. spec↔plan contract now agrees.

**M3 — test count 9→22 → RESOLVED in authoritative surfaces; one stale leftover (see N2).**
22 appears correctly in REQ-SEC-007 (spec.md:417,469), §5.4 (spec.md:605), §6.1 (spec.md:634), AC-004 (acceptance.md:100,105), DoD (acceptance.md:398), and plan.md:236,259,606. The single remaining "9개 test" is plan.md:122 (Phase 1 task 1) — flagged as N2.

**M4 — Phase 5 coordination + staged fail-closed → CONFIRMED-RESOLVED.**
Phase 5 (plan.md:273-336) adds a CROSS-SPEC COORDINATION GATE (AUTH-003 owner sign-off, plan.md:301-309) and staged fail-closed activation (plan.md:322-328: opt-in `audit.hash_chain.fail_closed` default false, post-backfill verify, operator unlock, alert-first lock-later). Task numbering is now sequential 1-6 (no skipped #2). AC-013 (acceptance.md:286-304) + EC-003 (acceptance.md:375) mirror this. NFR-SEC-004 (spec.md:506) encodes the staged lockdown.

**M5 — §5 ↔ acceptance sync → CONFIRMED-RESOLVED (with a new DoD leftover, N1).**
spec.md §5 now lists §5.1..§5.20 (20 scenarios), covering REQ-SEC-006 (§5.16), NFR-SEC-001 (§5.17), NFR-SEC-002 (§5.18), NFR-SEC-003 (§5.19), NFR-SEC-005 (§5.20). acceptance.md:16 correctly references "§5.1..§5.20." The sync direction is now complete; however acceptance.md:392 DoD still says "§5.1..§5.15" — see N1.

**EARS labels → CONFIRMED-RESOLVED.** REQ-SEC-003 = Conditional (IF-THEN) (spec.md:454); REQ-SEC-012 = Conditional (IF-THEN) (spec.md:479); REQ-SEC-018 = Ubiquitous (SHALL NOT) (spec.md:495). All match their grammar.

**depends_on → CONFIRMED-RESOLVED.** spec.md:15 `depends_on` now includes both SPEC-DEP-001 and SPEC-SYN-002. spec.md:675,681,699 document the related→depends_on promotion.

---

## New Findings (amendment-introduced + residual)

**N1. acceptance.md:392 — DoD checklist references "§5.1..§5.15" but §5 now has 20 scenarios (§5.1..§5.20) — Severity: minor (amendment regression).**
The amendment expanded spec.md §5 from 15 to 20 scenarios but left the Definition-of-Done item "All 15 scenario index entries (§5.1..§5.15) in spec.md are implemented as automated tests." This now under-scopes the DoD by 5 scenarios (the four NFR scenarios §5.17-5.20 + REQ-SEC-006 §5.16) and self-contradicts acceptance.md:16 ("§5.1..§5.20"). DoD would falsely pass without the new scenarios' tests. Fix: change to "All 20 scenario index entries (§5.1..§5.20)."

**N2. plan.md:122 — Phase 1 task 1 still says "REQ-CACHE-013 9개 test" — Severity: minor (M3 leftover).**
Authoritative count is 22 everywhere else (plan.md:236,259,606; spec; acceptance). The single Phase-1 ANALYZE task still says "9개 test의 input/output 패턴 기록," which would under-scope the characterization-test baseline at the exact step where it is captured. Internal plan.md contradiction. Fix: 9 → 22.

**m4 (pre-existing, carried from review-1). spec.md:463 REQ-SEC-006 — no assigned plan phase task — Severity: minor (P2/Optional).**
`grep "REQ-SEC-006 | native | push protection"` in plan.md returns no phase task. REQ-SEC-006 IS covered by §5.16 (spec.md:617) and AC (acceptance.md:334), and it is P2/Optional (GitHub native scanning, config-only), so impact is low. Recommend a one-line Phase 13 (Operator docs) note or a Phase 2 config step for completeness.

**m5 (pre-existing, carried). Frontmatter (spec.md:2-8)** — `created` not `created_at`; no `labels` (project schema uses `milestone`/`owner`). MP-3 technicality, not substantive.

No CRITICAL or MAJOR regressions were introduced by the amendment. The two new findings are both stale-number leftovers from incomplete propagation of the M3/M5 edits.

---

## Chain-of-Verification Pass

Second-look findings (re-read + re-verified, not skimmed):
- Re-counted ALL REQ-SEC / NFR-SEC / AC IDs via `grep | sort -u`: 18 REQs (001-018) + 005a, 7 NFRs, 15 ACs — no gaps, no dups, no renumber orphan. The amendment did NOT renumber requirements (a common regression vector) — confirmed.
- Re-opened the LIVE code (not the spec's claims) for C1: chain.go ComputeThisHash/VerifyChain/AcquireAdvisoryLock and 0003 prev_hash/this_hash both physically present — the spec's reuse premise is now factually grounded.
- Re-opened LIVE ssrf.go + types.go for M2: confirmed `fopts FetchOptions` and `RedirectMaxHops` are the real names; the spec now matches byte-for-byte.
- Specifically hunted for amendment regressions across all 3 files: found N1 (DoD §5.15 stale) and N2 (plan "9" stale) by cross-referencing every "15" / "9" / "§5.1" occurrence against the expanded §5. Both are count-propagation misses, not logic errors.
- Verified the phantom artifacts are not merely renamed-and-hidden: searched `merkle|20260522|audit-verify|BenchmarkMerkle` — every hit is inside a `~~strikethrough~~` or a "REMOVED"/HISTORY context. No live reinvention survives.
- Checked spec↔plan↔acceptance version stamps: all three carry v0.2.0 (spec.md:3, plan.md:3/793, acceptance.md:3/446). HISTORY consistent.
No defect was missed on first pass beyond N1/N2/m4 already listed.

---

## Regression Check (Iteration 2)

Defects from review-1:
- **C1** (phantom migration / duplicate chain) — **RESOLVED**: verified against chain.go:29/102/124 + 0003_audit_events.sql:33-34; REQ-SEC-017/Phase 5 narrowed to emit-into-existing; phantom artifacts struck. Was a stagnation-watch item — progress confirmed, NOT stagnant.
- **C2** (unguarded git rewrite) — **RESOLVED**: REQ-SEC-005a five guards (spec.md:462). NOT stagnant.
- **M1** (ghost path) — **RESOLVED**: spec.md:60 is the only remaining mention (HISTORY).
- **M2** (signature) — **RESOLVED**: spec.md:469 preserves fopts + RedirectMaxHops.
- **M3** (test count) — **RESOLVED** in contract surfaces; **partial leftover** plan.md:122 (N2).
- **M4** (Phase 5 coordination) — **RESOLVED**: plan.md:301-336.
- **M5** (§5 sync) — **RESOLVED**; **new DoD leftover** acceptance.md:392 (N1).
- **EARS labels** — **RESOLVED**.
- **depends_on** — **RESOLVED**.
No defect appears unchanged across both iterations → no stagnation/blocking-defect flag.

---

## Remaining Must-Fix (ordered)

These are NON-BLOCKING for status transition (all minor), but should be cleaned up before/at run-phase kickoff:

1. **[N1] acceptance.md:392** — change "All 15 scenario index entries (§5.1..§5.15)" → "All 20 scenario index entries (§5.1..§5.20)" so the DoD does not under-scope the 5 new NFR/REQ-SEC-006 scenario tests.
2. **[N2] plan.md:122** — change "REQ-CACHE-013 9개 test" → "22개 test" to match the rest of the document and the verified live count.
3. **[m4] plan.md** — add a one-line Phase task (Phase 2 config or Phase 13 docs) for REQ-SEC-006 (GitHub native scanning enable/document), or annotate it explicitly as config-only with no code phase.

---

## Recommendation

**Status transition recommendation: approve (draft → approved).**

Rationale: Both review-1 CRITICALs are resolved against ground-truth code (not just the self-report) — C1's reuse premise is physically confirmed in `internal/audit/chain.go` + `0003_audit_events.sql`, and C2's five guards are concretely specified in REQ-SEC-005a. All five MAJORs, the EARS labels, and depends_on are resolved in the authoritative contract surfaces (spec REQ table + acceptance matrix). The amendment introduced no CRITICAL/MAJOR regression — only two minor stale-count leftovers (N1 DoD §5.15, N2 plan "9개") and one carried P2 (REQ-SEC-006 plan task). None of these can mislead the run phase about correctness; they only risk slightly under-scoping test breadth, which the run-phase characterization step will surface anyway.

The SPEC is implementable as written. I recommend approving the draft and folding N1/N2/m4 into the run-phase kickoff checklist rather than forcing a third amend cycle. If a clean v0.2.1 is preferred before approval, the three fixes above are mechanical (number-edit only).
