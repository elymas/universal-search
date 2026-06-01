# SPEC Review Report: SPEC-SEC-001
Iteration: 1/3
Verdict: **FAIL**
Overall Score: 0.42

> Reasoning context from the SPEC author was ignored per M1 Context Isolation. This audit is based solely on spec.md / plan.md / acceptance.md and verification against the live codebase.

Adversarial stance: the SPEC was assumed defective until proven otherwise with evidence. Two CRITICAL and five MAJOR findings were confirmed against the running code. As the M8 terminal P0 gate that unblocks SPEC-REL-001 + SPEC-DEPLOY-001, approval here is high-consequence; the central premise of REQ-SEC-017/Phase 5 is factually wrong about the existing system, and two irreversible operations lack guards.

---

## Must-Pass Results

- **[PASS] MP-1 REQ number consistency**: REQ-SEC-001 … REQ-SEC-018 all present, sequential, no gaps, no duplicates (verified end-to-end via `grep -oE "REQ-SEC-[0-9]{3}" | sort | uniq -c`). NFR-SEC-001…007 complete. Consistent 3-digit zero-padding.
- **[PASS] MP-2 EARS format compliance**: All 18 REQs use `SHALL` + a recognizable EARS keyword (Ubiquitous / WHEN / IF-THEN / WHERE). No free-form or informal requirements. Three TYPE-LABEL mislabels exist (see m1/m2/m3) but the underlying syntax is still EARS-recognizable, so MP-2 is not a hard fail. Rubric band ~0.85.
- **[FAIL] MP-3 YAML frontmatter validity**: `id`, `version`, `status`, `priority` present; date field is `created:` (ISO `2026-05-22`) rather than `created_at`; **`labels` field is absent**. Strict MP-3 treats a missing required field as FAIL. NOTE: this project's SPEC schema uses `milestone`/`owner`/`methodology` instead of `labels`, so this is a schema-convention mismatch rather than a substantive defect — recorded as Minor, not a headline blocker (the CRITICAL findings independently drive the FAIL).
- **[FAIL] MP-4 Section 22 language neutrality**: N/A as a 16-language enumeration check (single Go project), BUT a related neutrality-class failure exists: REQ-SEC-017 / Phase 5 hardcode a hash-chain model that contradicts the already-shipped AUTH-003 implementation (see C1). Recorded under C1 rather than MP-4. MP-4 itself: **N/A — single-language SPEC**.

---

## Category Scores (0.0–1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.55 | 0.50 | Requirements are individually readable but the document contradicts itself on the core refactor: §1.1/§7.2 use `internal/cache/access/` while HISTORY/REQ-SEC-007 use `internal/access/` (D-A). spec.md:311, :686-687 vs :38, :398. |
| Completeness | 0.60 | 0.50–0.75 | All sections present; but spec.md §5 lists only 15 scenarios while acceptance.md has 19; REQ-SEC-006 + NFR-SEC-001/002/003/005 missing from §5 (covered only in acceptance.md). Phase 5 omits task #2 (numbering jumps 1→3). plan.md:250-258. |
| Testability | 0.45 | 0.25–0.50 | REQ-SEC-007 / §5.4 acceptance ("all **9** REQ-CACHE-013 tests pass unchanged") is unverifiable — the real count is 22 (D-C). DDD PRESERVE acceptance is built on a false premise. spec.md:398, :534. |
| Traceability | 0.40 | 0.25–0.50 | REQ-SEC-017/Phase 5 trace to a non-existent migration and duplicate an existing package (C1). `depends_on` omits SPEC-DEP-001 (deps-audit.yml ground truth, foundational) and lists SPEC-SYN-002 only as `related` though REQ-SEC-015 is a hard code dependency on the SYN-002 flow. spec.md:15, :416. |

---

## Defects Found

### CRITICAL

**C1. spec.md:416 (REQ-SEC-017) + plan.md:245-279 (Phase 5) — Phantom migration / duplicate hash-chain against an already-shipped AUTH-003 implementation — Severity: critical**
Verified against live code:
- `deploy/postgres/migrations/0003_audit_events.sql` already defines `audit_events(... prev_hash TEXT, this_hash TEXT, schema_version INT ...)` — the `prev_hash` column the plan proposes to "ADD" already exists (SPEC-AUTH-003 REQ-AUTH3-001).
- `internal/audit/chain.go:29` already implements `ComputeThisHash(prevHash, evt) = SHA256(prev_hash || canonical_json(row_minus_hashes))` plus chain verification and a daily `audit.chain_verify` job (SPEC-AUTH-003 REQ-AUTH3-008). Phase 5 proposes a redundant `internal/security/events/merkle.go`.
- Plan targets `ops/migrations/20260522_audit_prev_hash.sql` (plan.md:254) but the real migrations directory is `deploy/postgres/migrations/`.
- Chain semantics conflict: AUTH-003 chains per `(tenant_id, event_type)` with `pg_advisory_xact_lock`; REQ-SEC-017 describes a single global "SHA-256 of the previous row" chain — incompatible.
- Verify-budget conflict: NFR-SEC-004 says ≤ 30s / 1M rows; AUTH-003 NFR-AUTH3-007 says ≤ 30 min / 600K-2M rows for the same chain.
- Default conflict: AUTH-003 `audit.hash_chain.enabled` default `false`; NFR-SEC-004 makes it fail-closed (audit-write lockdown on break) — effectively mandatory.
Running a migration to add an existing column, with a fail-closed audit-write lockdown semantics layered on a chain that already has different semantics, is a high-blast-radius destructive operation built on a factually wrong model. Must be reconciled before any implementation.

**C2. plan.md:146-173 (Phase 2) + spec.md:391 (REQ-SEC-005) — `git filter-repo` + force-push on `main` with no concrete guard — Severity: critical**
REQ-SEC-005(b) and Phase 2 Task 3 invoke git-history rewrite via `git filter-repo` followed by force-push on `main`. The only guard text is "(requires force-push approval)" (spec.md:391) — no approval mechanism (who, how enforced), no mandatory backup ref before rewrite, no staging/dry-run validation, no rollback path. A force-push on `main` rewrites every commit SHA for all collaborators and is irreversible. Per the destructive-operation rule, a destructive op without an explicit guard is a CRITICAL finding. Requires: explicit human approval gate, mandatory `refs/backup/*` snapshot before `filter-repo`, dry-run on a clone, and a documented rollback.

### MAJOR

**M1 (D-A) — CONFIRMED. spec.md:311, :686, :687 — references non-existent `internal/cache/access/` — Severity: major**
Glob verified: `internal/access/` exists; `internal/cache/access/` does NOT. `internal/access/ssrf.go` and `internal/access/dialer.go` are the real files. The document is internally contradictory: HISTORY (L38), REQ-SEC-007 (L398), and Exclusions (L555, L702, L796) use the correct `internal/access/`, but the actionable §7.2 "Files to Modify" table (L686-687) and §1.1 (L311) point at the phantom path. The run phase would chase a non-existent directory.

**M2 (D-B) — CONFIRMED. spec.md:398 (REQ-SEC-007) vs internal/access/ssrf.go:31,61 + plan.md:227-229 — signature change misrepresented as behavior-identical — Severity: major**
Actual current (unexported) signatures:
- `validateHost(ctx context.Context, u *url.URL, opts Options, fopts FetchOptions) error` (ssrf.go:31)
- `validateRedirect(next *url.URL, opts Options, fopts FetchOptions, hopCount int) error` (ssrf.go:61)
REQ-SEC-007 specifies `ValidateHost(ctx, u, opts)` (drops `fopts`) and `ValidateRedirect(prev, next, opts, hopCount)` (adds `prev`, drops `fopts`), and renames the option `RedirectMaxHops`→`MaxRedirects`.
The dropped `fopts FetchOptions.AllowPrivateNetworks` is a real per-call behavior input (types.go:17; guard logic `opts.AllowPrivateNetworks || fopts.AllowPrivateNetworks`, ssrf.go:32, dialer.go:35). Two existing tests exercise exactly that path — `TestValidateHost_FetchOptions_AllowPrivate` (ssrf_test.go:155) and `TestPinnedDialContext_FetchOptions_AllowPrivate` (dialer_test.go:52) — so REQ-SEC-007's claim that "all REQ-CACHE-013 tests SHALL continue to pass" under this signature is **false**.
Additionally, plan.md:229 states the refactor will keep "기존 함수 signature는 유지 (caller 변경 없음)" — which directly **contradicts** REQ-SEC-007's new exported signatures. Spec and plan disagree on the central refactor contract.

**M3 (D-C) — CONFIRMED. spec.md:398, :534 — "9 tests" undercount; real count is 22 — Severity: major**
Verified SSRF-direct test functions: `ssrf_test.go` = 14, `ssrf_redirect_test.go` = 5, `dialer_test.go` = 3 → **22 total** (matches the ~22 expectation; not 9). The DDD PRESERVE acceptance criterion "all 9 SPEC-CACHE-001 REQ-CACHE-013 tests pass" is wrong as written and would under-scope the characterization-test gate.

**M4. plan.md:251-258, :274-279 (Phase 5) — cross-SPEC schema amendment without coordination gate; drafting defects — Severity: major**
Phase 5 amends SPEC-AUTH-003's schema (`prev_hash`) but specifies no cross-SPEC approval/coordination with the AUTH-003 owner, despite SEC-001 depending on AUTH-003. Compounded by C1 (the column already exists). Phase 5 task list skips task #2 (jumps 1→3). The fail-closed lockdown (NFR-SEC-004) lacks a staged-rollout / post-backfill-verify-before-enable step, risking an audit subsystem lockout from a botched migration.

**M5. spec.md §5 (L531-545) vs acceptance.md — §5 table omits scenarios present in acceptance.md — Severity: major (downgraded to traceability-internal)**
spec.md §5 lists 15 scenarios and omits REQ-SEC-006 and NFR-SEC-001/002/003/005. These ARE covered in acceptance.md (verified: acceptance.md contains REQ-SEC-006 ×3, NFR-SEC-001 ×3, 002 ×2, 003 ×2, 005 ×2). So overall acceptance traceability is satisfied, but spec.md §5 is internally incomplete relative to its own acceptance.md, creating ambiguity about the authoritative acceptance set.

### MINOR

- **m1. spec.md:384 (REQ-SEC-003)** — labeled "State-Driven" but written as `IF … THEN` (Unwanted pattern). EARS type-label mismatch.
- **m2. spec.md:408 (REQ-SEC-012)** — labeled "State-Driven" but written as `IF … THEN`. Should be `WHILE` for State-Driven, or relabel Unwanted/Conditional. EARS type-label mismatch.
- **m3. spec.md:424 (REQ-SEC-018)** — labeled "Unwanted" but is a plain `SHALL NOT` (ubiquitous-negative); no `IF … THEN` trigger. EARS type-label mismatch.
- **m4. spec.md:392 (REQ-SEC-006)** — no §5 scenario and no clearly-assigned plan phase (orphan at the spec-§5 + plan level; inline acceptance cell + acceptance.md coverage exist). P2/Optional, low impact.
- **m5. spec.md:15 (`depends_on`)** — under-declares dependencies: SPEC-DEP-001 (deps-audit.yml ground truth, L94/L97) is absent; SPEC-SYN-002 is only `related` though REQ-SEC-015 (spec.md:416) is a hard code dependency that inserts into the SYN-002 citation flow.
- **m6. Frontmatter (spec.md:1-18)** — `created` instead of `created_at`; no `labels` field (project schema uses `milestone`/`owner`). MP-3 technicality.

### NIT
- **n1. spec.md:398, :414 (REQ-SEC-007, REQ-SEC-013)** — embed exact Go signatures and struct field lists (implementation HOW) inside requirements (RQ-4). Tolerable for an extraction-refactor contract, but the HOW is precisely where the D-B inaccuracy entered; prefer behavioral framing.

---

## Chain-of-Verification Pass

Second-look findings (re-read + re-verified, not skimmed):
- Re-counted ALL REQ-SEC IDs via `grep | sort | uniq -c`: 001-018 present, sequential, no gaps/dups — MP-1 confirmed end-to-end (not spot-checked).
- Re-verified D-A by Glob: `internal/cache/access` returns "No such file or directory"; `internal/access/{ssrf,dialer}.go` exist.
- Re-read full `internal/access/ssrf.go` and `dialer.go` to confirm `fopts FetchOptions` is load-bearing (not vestigial) — it is (used in the guard OR-condition and in pinned-dial bypass).
- Re-checked the existing AUTH-003 migration file directly: `deploy/postgres/migrations/0003_audit_events.sql` contains `prev_hash` (grep -l confirmed) — strengthening C1 from "possible duplication" to "confirmed phantom migration".
- Re-examined acceptance.md (separate 435-line file) — this DOWNGRADED my initial orphan-NFR finding: the four NFRs and REQ-SEC-006 ARE covered there, so the issue is spec.md §5 internal incompleteness (M5), not a true traceability orphan. Adjusted severity accordingly to avoid a false-positive over-count.
- Checked contradiction between plan.md:229 ("keep signatures") and REQ-SEC-007 (new exported signatures) — confirmed direct spec↔plan contradiction, folded into M2.
No new defects beyond those listed; one initial finding was correctly downgraded after deeper verification.

---

## Regression Check (Iteration 2+ only)
N/A — iteration 1.

---

## Recommendation (amend-then-approve)

Status transition recommendation: **amend-then-approve** (do NOT transition draft→approved this iteration). The defects are salvageable but the two CRITICALs invalidate the run-phase contract as written.

Must-fix before implementation (ordered):

1. **[C1] Reconcile REQ-SEC-017 / Phase 5 with the existing AUTH-003 hash chain.** Decide explicitly: reuse `internal/audit/chain.go` + the existing `prev_hash`/`this_hash` columns, OR justify a separate chain. Remove the phantom "add prev_hash" migration; if any migration is genuinely needed, target `deploy/postgres/migrations/`. Reconcile chain semantics (per-tenant+event_type vs global), verify budget (NFR-SEC-004 30s/1M vs AUTH-003 NFR-AUTH3-007 30min/2M), and enabled-by-default vs fail-closed. Coordinate the amendment with the SPEC-AUTH-003 owner.
2. **[C2] Add concrete guards to the Phase 2 git-history rewrite.** Specify the approval gate mechanism (named approver/role + how enforced), a mandatory `refs/backup/<date>` snapshot before `git filter-repo`, a dry-run on a throwaway clone, and a documented rollback. No force-push on `main` without all four.
3. **[M2 + M1] Correct REQ-SEC-007.** Either (a) preserve the real signatures including `fopts FetchOptions` and `RedirectMaxHops`, framing the extraction as behavior-identical, or (b) declare it an API change (NOT a behavior-identical PRESERVE) and enumerate the migration of `Options`/`FetchOptions` merge + all call sites (cascade.go:60-63, phase3_get.go:61, phase4_tls.go:69). Fix every `internal/cache/access/` reference to `internal/access/` (spec.md:311, :686, :687). Resolve the spec↔plan signature contradiction (plan.md:229).
4. **[M3] Replace "9 tests" with the verified count (22)** in REQ-SEC-007 (spec.md:398) and §5.4 (spec.md:534); ensure the characterization-test gate enumerates all 22 (ssrf_test 14 / ssrf_redirect 5 / dialer 3).
5. **[M4] Phase 5:** add the cross-SPEC coordination gate, fix task numbering, and add a "verify chain after backfill before enabling fail-closed" staged-rollout step.
6. **[M5] Sync spec.md §5** to acceptance.md (add the 4 NFR + REQ-SEC-006 scenarios) or declare acceptance.md authoritative.
7. **[m1-m3] Fix EARS type labels** for REQ-SEC-003, 012, 018.
8. **[m5] Update `depends_on`**: add SPEC-DEP-001; promote SPEC-SYN-002 to `depends_on` (or justify keeping it `related`).

Re-audit at iteration 2 after amendments; the CRITICALs are stagnation-watch items.
