## SPEC-AUTH-001 Progress

- Status: not_started
- Created: 2026-05-22
- Methodology: TDD (per .moai/config/sections/quality.yaml development_mode: tdd)
- Coverage target: 85%
- Harness: standard
- Owner: expert-security

## Phase Status

| Phase | Description | Status |
|-------|-------------|--------|
| Phase 0.5 | Research (research.md) | complete |
| Phase 1 | SPEC draft (spec.md) | complete (draft) |
| Phase 1.5 | Task decomposition (tasks.md) | complete |
| Phase 1.6 | Acceptance scenarios (acceptance.md) | complete |
| Phase 2 | plan-auditor review | pending |
| Phase 2.5 | Annotation cycle (user review) | pending |
| Phase 3 | TDD implementation (T-001 through T-006) | pending |
| Phase 3.5 | Quality validation (coverage, lint, race) | pending |
| Phase 3.75 | Pre-submission self-review | pending |
| Phase 4 | /moai sync (docs, PR) | pending |

## Acceptance Criteria Tracking

| Scenario | Status |
|----------|--------|
| §5.1 valid JWT → 200 | pending |
| §5.2 permissive + missing → anonymous | pending |
| §5.3 JWKS rotation forced fetch | pending |
| §5.4 expired → 401 (no anonymous fallback) | pending |
| §5.5 JWT sub takes precedence over header (anti-spoof) | pending |
| §5.6 tenant.mode=claim → TenantIDKey | pending |
| §5.7 strict / disabled / allowlist | pending |
| §5.8 logout + revocation | pending |
| §5.9 SSRF block + callback rate-limit | pending |
| Edge1 clock skew boundary | pending |
| Edge2 revocation Redis fail-open/closed | pending |

## Joint Invariants (must remain true throughout implementation)

- [ ] `cost_ledger.user_id` TEXT column schema unchanged (DEEP-004 REQ-DEEP4-002)
- [ ] No new migration file added under `deploy/postgres/migrations/` by this SPEC
- [ ] DEEP-004 existing tests (TestIdentityMiddlewareReadsXUserId, TestIdentityMiddlewareDefaultsAnonymous) pass unchanged
- [ ] `costguard.UserIDKey` context key reused (no new key for user_id)
- [ ] AUTH-003 forward-compat: stderr JSON line schema is additive-only

## Notes

- This SPEC is the M6 release-gate first deliverable.
- Direct blocker for: AUTH-002, AUTH-003, IDX-004, IDX-005.
- Implementation requires foreground subagent (writes files) — `run_in_background: false`.
- Recommended worktree isolation per CLAUDE.md Worktree Isolation Rules: SPEC implementation cross-file changes → use `Agent(isolation: "worktree")`.

---

*Progress file initialized 2026-05-22. To be updated by manager-tdd at each phase boundary.*
