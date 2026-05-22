## SPEC-AUTH-002 Progress

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
| §5.1 enforcer init + member read query:basic | pending |
| §5.2 AUTH-001 JWT context + header fallback | pending |
| §5.3 empty team_id default fallback / 400 | pending |
| §5.4 observer write deny (deny-by-default + role hierarchy) | pending |
| §5.5 EnforceMiddleware admin-only endpoints | pending |
| §5.6 IndexQuery TeamID wiring activates IDX-001 3-store filter | pending |
| §5.7 team_shared accessible / personal V1.1 denied | pending |
| §5.8 admin reload + member CRUD | pending |
| §5.9 decision audit 3-surface emit | pending |
| Edge1 concurrency + latency NFR | pending |
| Edge2 LoadPolicy atomic replace on failure | pending |

## Joint Invariants (must remain true throughout implementation)

- [ ] `costguard.UserIDKey` / `costguard.TenantIDKey` / `auth.ClaimsKey` semantics unchanged (AUTH-001 forward-compat)
- [ ] AUTH-001 existing tests pass unchanged after AUTH-002 ship
- [ ] IDX-001 schema unchanged (no migration added by AUTH-002; separate 0003_casbin_rules.sql is for policy storage only)
- [ ] IDX-001 existing tests pass unchanged (IndexQuery.TeamID field already exists; AUTH-002 only wires source)
- [ ] DEEP-004 cost_ledger schema unchanged (user-level cost guard orthogonal to team-level RBAC)
- [ ] AUTH-003 forward-compat: stderr JSON line schema is additive-only

## Notes

- This SPEC is the M6 release-gate second deliverable (direct downstream of AUTH-001, direct blocker for IDX-004 / IDX-005).
- Direct blocker for: IDX-004, IDX-005, AUTH-003.
- Implementation requires foreground subagent (writes files) — `run_in_background: false`.
- Recommended worktree isolation per CLAUDE.md Worktree Isolation Rules: SPEC implementation cross-file changes → use `Agent(isolation: "worktree")`.
- Casbin minor version (v2.103.x) to be pinned exactly via `go get` at start of Phase A.
- AUTH-001 JWT claim names (`team_id`, `roles`) require joint review with AUTH-001 author before Phase C (see Open Question 1 in spec.md §8).

---

*Progress file initialized 2026-05-22. To be updated by manager-tdd at each phase boundary.*
