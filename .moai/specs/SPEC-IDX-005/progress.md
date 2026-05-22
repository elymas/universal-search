## SPEC-IDX-005 Progress

- Started: 2026-05-22
- Phase 0.5 complete: research.md drafted (~520 lines, 9 sections, 10 pinned decisions, 8 open questions)
- Phase 0.9: Go project detected → moai-lang-go (planned)
- Phase 0.95: Standard Mode (~22 files, 1 domain backend, TDD)
- Harness: standard (evaluator=final-pass, effort=high)
- UltraThink: active
- SPEC status: draft → pending plan-auditor cycle
- Phase 1 pending: manager-strategy plan validation
- Phase 1.5 complete: tasks.md generated (5 phases, T-001 through T-005)
- Phase 1.6 pending: acceptance criteria → TaskList registration
- Phase 1.7 pending: file stubs creation in internal/idx5/
- Phase 2 pending: TDD implementation across 5 phases (A-E)
  - Phase A: storage foundation + DocType enum + types (5 tests)
  - Phase B: lookup core + threshold + staleness (6 tests)
  - Phase C: hit serve + async write-back (7 tests)
  - Phase D: citation re-validation + feedback handler (8 tests)
  - Phase E: middleware wiring + observability + M6 exit gate test (12 tests, including critical NFR-001 integration test)
- Phase 2.5 pending: quality validation
  - Coverage target: 85%
  - go vet / golangci-lint / go test -race: PASS required
  - M6 exit gate: TestDedupHitRateAt30PctOnSyntheticTraffic PASS required
- Phase 2.75 pending: pre-review quality gate

## M6 Release Gate Status

This SPEC is the **PRIMARY DRIVER** of the M6 exit criterion ("shared index
dedup hits ≥30%"). PR merge gates:

- [ ] Phase A-D RED tests written
- [ ] Phase A-D GREEN tests passing
- [ ] Phase E RED tests written (including TestDedupHitRateAt30PctOnSyntheticTraffic)
- [ ] Phase E GREEN tests passing
- [ ] `TestDedupHitRateAt30PctOnSyntheticTraffic` PASS (PRIMARY GATE)
- [ ] `TestCrossTenantLookupReturnsZeroResults` + acceptance §5.6 PASS (CRITICAL SECURITY)
- [ ] Coverage ≥85% on internal/idx5/
- [ ] All quality gates (go vet / golangci-lint / go test -race) PASS
- [ ] LSP gate: zero errors / type errors / lint errors
- [ ] plan-auditor cycle: HIGH = 0, MEDIUM ≤ 2

All above PASS → M6 release candidate tag eligible.

---

*Last updated: 2026-05-22 (initial draft).*
