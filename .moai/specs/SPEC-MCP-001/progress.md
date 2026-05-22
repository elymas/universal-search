## SPEC-MCP-001 Progress

- Started: 2026-05-22
- Phase 1 complete: Strategy analysis approved. 10 TDD tasks (T1-T10) mapped to 16 EARS REQs.
- Phase 1.5 complete: Task decomposition in run workflow agent output.
- Key design decisions:
  - costguard: Call CapChecker.EvaluateAtomic directly (bypass chi middleware chain, share Redis Lua counters)
  - shared orchestrator: Extract from cmd/usearch/query.go to internal/orchestrator/search.go
  - get_citation: V1 uses in-memory fanout result cache, TBD marker for IDX-001 GetByDocID
  - HTTP auth: trust-headers mode pre-AUTH-001, jwt mode delegates to AUTH-001 middleware
- Harness: standard
- Methodology: TDD
- Coverage target: 85%

### Session 2 (2026-05-23): T1-T6 Implementation

TDD Cycles Completed:
- T1: SDK Pin + Server Scaffolding - MCP SDK v1.6.1 pinned, server.go/config.go created, main.go updated, 3 tests pass (TestStdioInitializeRoundTrip, TestTransportFlagDefaultsStdio, TestVersionFlagPreserved)
- T2: Tool Types + Error Mapping - types.go with 7 input/output structs, errors.go with 10 error mappings per REQ-MCP-016, 12 tests pass
- T3: list_sources Tool - wraps adapters.Registry.List() with deterministic sort, 2 tests pass
- T4: get_citation Tool - DocCache for in-memory resolution, ErrCitationNotFound sentinel, 2 tests pass
- T5: Shared Orchestrator Extraction - internal/orchestrator/search.go extracted, CLI tests pass (no regression), 3 tests pass
- T6: search Tool - wraps shared orchestrator, populates DocCache for citation resolution, 2 tests pass

Test Summary:
- internal/mcpserver: 5 tests (3 integration + 2 unit)
- internal/mcpserver/tools: 10 tests (unit)
- internal/orchestrator: 3 tests (unit)
- Total: 28 tests, all passing with -race
- CLI regression: cmd/usearch tests pass unchanged

Remaining: T7 (deep_research), T8 (HTTP transport), T9 (observability), T10 (shutdown+NFR)
