## SPEC-IDX-004 Progress

- Started: 2026-05-22
- Phase 0.5 complete: research.md drafted (~840 lines, 12 sections, 10 pinned decisions, 10 open questions, external citations to Qdrant Tiered Multitenancy + Meili tenant tokens + meilisearch-go SDK)
- Phase 0.9: Go project detected → moai-lang-go (planned)
- Phase 0.95: Standard Mode (~20 files, 1 domain backend, TDD)
- Harness: standard (evaluator=final-pass, effort=high)
- UltraThink: active
- SPEC status: draft → pending plan-auditor cycle
- Phase 1 pending: manager-strategy plan validation
- Phase 1.5 complete: tasks.md generated (6 phases, T-001 through T-006)
- Phase 1.6 pending: acceptance criteria → TaskList registration
- Phase 1.7 pending: file stubs creation in internal/index/tenancy/, internal/index/auth/, internal/index/tenant/, internal/index/backfill/, cmd/usearch/admin/
- Phase 2 pending: TDD implementation across 6 phases (A-F)
  - Phase A: context & tenancy mode foundation (6 tests)
  - Phase B: PG migration foundation (6 tests)
  - Phase C: dispatch enforcement + Upsert silent overwrite (10 tests)
  - Phase D: Qdrant Tiered Multitenancy + filter builder + __public__ sentinel (9 tests)
  - Phase E: Meili tenant tokens + cache + Korean shard application (11 tests)
  - Phase F: backfill CLI + tier-promote CLI + observability wiring + cross-team integration test (18 tests, including critical NFR-001 integration test)
- Phase 2.5 pending: quality validation
  - Coverage target: 85%
  - go vet / golangci-lint / go test -race: PASS required
  - M6 cross-team isolation gate: TestCrossTeamIsolationEndToEnd PASS required (NFR-IDX4-001)
- Phase 2.75 pending: pre-review quality gate

## M6 Release Gate Status (IDX-004's contribution)

This SPEC is the **ENABLING INVARIANT** for the M6 exit criterion ("shared
index dedup hits ≥30%"). IDX-005 (the M6 PRIMARY DRIVER) cannot ship safely
until IDX-004's multi-tenancy enforcement is verified. PR merge gates:

- [ ] Phase A-E RED tests written
- [ ] Phase A-E GREEN tests passing
- [ ] Phase F RED tests written (including `TestCrossTeamIsolationEndToEnd`)
- [ ] Phase F GREEN tests passing
- [ ] **`TestCrossTeamIsolationEndToEnd` PASS** (CRITICAL SECURITY INVARIANT — NFR-IDX4-001)
- [ ] `TestRetrievalLatencyDegradationWithinBudget` PASS (NFR-IDX4-003, NFR-IDX4-006)
- [ ] PG migrations 0004 + 0005 idempotent + backfill verified (REQ-IDX4-010)
- [ ] backfill CLI dry-run + execute + resume PASS (REQ-IDX4-011, NFR-IDX4-005)
- [ ] Meili tenant token concurrency safe (`goleak.VerifyNone` PASS, NFR-IDX4-004)
- [ ] `__public__` sentinel rejected in 4 entry points (REQ-IDX4-007)
- [ ] AUTH-001/002 ship-before forward-compat verified (env var fallback + default visibility) (NFR-IDX4-008)
- [ ] Coverage ≥85% on new packages
- [ ] All quality gates (go vet / golangci-lint / go test -race) PASS
- [ ] LSP gate: zero errors / type errors / lint errors
- [ ] cardinality allowlist `TestNoUnboundedLabels` PASS (NFR-IDX4-007)
- [ ] plan-auditor cycle: HIGH = 0, MEDIUM ≤ 2

All above PASS → IDX-004 ship → IDX-005 implementation can safely proceed
→ IDX-005's `TestDedupHitRateAt30PctOnSyntheticTraffic` becomes measurable
→ M6 GA gate eligible.

## Cross-SPEC Dependency Status

- **SPEC-IDX-001** (implemented): reservation surface exists (`.moai/specs/SPEC-IDX-001/spec.md:6,226-258`); IDX-004 transitions reservation to enforcement.
- **SPEC-IDX-002** (implemented): BGE-M3 embedder team-agnostic; no IDX-004 change required.
- **SPEC-IDX-003** (implemented): Korean shard already has `team_id` filterable; IDX-004 adds `user_id` filterable + applies tenant tokens to both shards.
- **SPEC-AUTH-001** (draft, M6): JWT context key supplier. IDX-004 consumes `authctx.TeamIDKey`/`UserIDKey` with `INDEX_DEFAULT_TEAM` env var fallback for forward-compat (NFR-IDX4-008).
- **SPEC-AUTH-002** (draft, M6): `Adapter.Visibility()` hook supplier. IDX-004 defines interface + DI seam only; v1.0 defaults to `team_shared` for unimplemented adapters.
- **SPEC-OBS-001** (implemented): cardinality allowlist mechanism (`internal/obs/metrics/metrics.go:171-176`); IDX-004 extends with `team_id_hashed`, `visibility`, `tier` labels.
- **SPEC-DEEP-004** (implemented, M5): context key convention precedent (`costguard.UserIDKey`) + cost_ledger `tenant_id` column (semantic parallel to docs.team_id; column rename deferred to AUTH-003).
- **SPEC-IDX-005** (draft, M6, blocked by IDX-004): immediate downstream consumer. IDX-005's NFR-IDX5-004 (cross-tenant leak == 0) is load-bearing on IDX-004's NFR-IDX4-001.

## Deferred Decisions

The following decisions are recorded in §8 Open Questions of spec.md but are NOT
blockers for plan-auditor PASS:

1. Default team identifier (`INDEX_DEFAULT_TEAM` env var; fallback `"default"`)
2. Token TTL (15min default; 5min~1h tunable via config; first-30-day tuning)
3. team_id format validation (`^[a-zA-Z0-9_-]{1,64}$` + reserved sentinel reject; AUTH-001 JWT claim format align)
4. Dedicated tier promotion trigger (manual list v1.0; doc-count auto deferred to SPEC-IDX-007)
5. user_id ingest path for stateful vs stateless adapters (both supported; AUTH-002 + adapter-level decision)

All deferred decisions are tunable post-V1 from production data; none block
the M6 release gate.

---

*Last updated: 2026-05-22 (initial draft after research.md completion + 6-file SPEC package generation).*
