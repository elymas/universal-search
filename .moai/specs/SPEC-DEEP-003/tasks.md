---
spec_id: SPEC-DEEP-003
version: 0.1.2
status: planned
methodology: TDD
total_tasks: 44
---

# Tasks — Tree exploration for /deep multi-agent

This document is the TDD task decomposition of SPEC-DEEP-003 v0.1.2.
Every REQ-DEEP3-XXX and NFR-DEEP3-XXX has at least one dedicated task.
Sequencing follows RED → GREEN → REFACTOR within each phase.
Concurrency-sensitive tasks are tagged `[RACE]` and MUST run under
`go test -race`. Goroutine-leak-sensitive tasks are tagged `[GOLEAK]`
and MUST be wrapped with `goleak.VerifyNone(t)`.

## Task Index

| ID | Phase | Title | REQ refs | Test refs | Blocked by |
|----|-------|-------|----------|-----------|------------|
| T-A-001 | A | [RED] Tree type set (Node, NodeStatus, NodeCitation, NodeClaim, TreeResult, FlattenedClaim) JSON round-trip + exhaustive switch tests | REQ-010, REQ-013 | TestTreeTypesJSONRoundTrip, TestNodeStatusExhaustiveSwitch, TestFlattenedClaimLineageInvariant | — |
| T-A-002 | A | [GREEN] Tree types implementation (tree_types.go) with cost fields | REQ-010, REQ-013 | (T-A-001 PASS) | T-A-001 |
| T-A-003 | A | [RED] BudgetTracker signature + PreCheck/Release tests (token cap, structural cap, 25% headroom) | REQ-006, REQ-007, NFR-004 | TestBudgetPreCheckTokenExceeded, TestBudgetPreCheckStructuralCap, TestBudgetPreCheckHeadroomConservative | T-A-002 |
| T-A-004 | A | [RED][RACE] Reservation lock concurrency tests (N=8 goroutine pre-check serialization, atomic release) | REQ-006, REQ-007, NFR-003 | TestBudgetReservationLockSerializesSiblings, TestBudgetReservationReleaseOnComplete | T-A-002 |
| T-A-005 | A | [RED] Root-node seed pre-check test (DEEP_TREE_ROOT_TOKEN_ESTIMATE simulation) | REQ-006 | TestBudgetRootSeedEstimate, TestExpandTreeRootSeedTriggersImmediateBudgetFail (root-seed-only slice) | T-A-002 |
| T-A-006 | A | [GREEN] BudgetTracker minimal impl with sync.Mutex reservation lock | REQ-006, REQ-007 | (T-A-003..005 PASS) | T-A-003, T-A-004, T-A-005 |
| T-A-007 | A | [REFACTOR][RACE] Verify lock contention is not pathological; rerun `go test -race -count=10` on budget_test.go | REQ-006, REQ-007, NFR-003 | (same as T-A-004 stress mode) | T-A-006 |
| T-B-001 | B | [RED] ExpandTree happy-path test (mocked Researcher, breadth=4 depth=3, 85 nodes, max_depth=3) | REQ-001, REQ-003 | TestExpandTreeHappyPath | T-A-006 |
| T-B-002 | B | [RED] ExpandTree input validation tests (breadth=9 → HTTP 400, depth=6 → HTTP 400, leaf node skips decompose) | REQ-002, REQ-003 (leaf) | TestExpandTreeInvalidBreadth, TestExpandTreeInvalidDepth | T-B-001 |
| T-B-003 | B | [RED][RACE] BFS ordering + per-depth errgroup join test (depth N+1 starts only after depth N join) | REQ-004 | TestExpandTreeBFSOrdering | T-B-001 |
| T-B-004 | B | [RED][RACE][GOLEAK] Concurrent breadth test (N=breadth goroutines per level, goroutine ID tracking, no leak on context cancel) | REQ-004 | TestExpandTreeConcurrentBreadth | T-B-003 |
| T-B-005 | B | [RED][RACE] Concurrent breadth budget race-free test, 100 iterations, worst-case breadth=8 depth=2 budget=60K, deterministic `final_total ≤ budget_cap` invariant | REQ-006, REQ-007, NFR-003 | TestExpandTreeConcurrentBreadthBudgetRaceFree | T-A-007, T-B-004 |
| T-B-006 | B | [RED] Budget exhaustion + partial-tree return test (budget=20K, average 2000/node, expects HTTP 200 degraded + flattened claims) | REQ-006, REQ-008, REQ-010 | TestExpandTreeBudgetExceeded, TestExpandTreePartialReturn | T-B-005 |
| T-B-007 | B | [RED] Per-depth latency p95 + tree e2e p95 measurement test (50-iter mock) | NFR-001, NFR-002 | TestExpandTreeLatencyP95, TestExpandTreeEndToEndLatencyP95 | T-B-001 |
| T-B-008 | B | [RED] Citation disjointness invariant test (pairwise slice identity check) | REQ-009b | TestCitationsDisjointlyOwned | T-B-001 |
| T-B-009 | B | [RED] FlattenedClaim lineage property test (hypothesis-go, 100 random trees, lineage_path[0]==root_query, len==depth+1, reverse-traversable) | REQ-009a, REQ-010 | TestFlattenedClaimLineageProperty | T-B-001 |
| T-B-010 | B | [RED] Per-node CostUSD computed on complete + tree.TotalCostUSD derivation test | REQ-013 | TestNodeCostUSDComputedOnComplete, TestTreeTotalCostUSDSumsCompletedNodes | T-A-002 |
| T-B-011 | B | [RED] In-memory tree memory footprint under 100MB at worst-case (breadth=8 depth=5) + frontier truncation on overshoot | NFR-009 | TestTreeMemoryFootprintUnder100MBWorstCase | T-B-001 |
| T-B-012 | B | [RED] Edge: depth=1 single-level tree (root + breadth leaves, no decompose at depth=1) | REQ-003 (leaf) | TestExpandTreeDepthOneSingleLevel | T-B-001 |
| T-B-013 | B | [GREEN] ExpandTree implementation with fresh errgroup per depth level (BFS invariant), reservation-lock-guarded pre-check, atomic release in single critical section, flattenWithLineage helper, memory-footprint guard | REQ-001..010, REQ-013, NFR-001..004, NFR-009 | (all Phase B RED tests PASS) | T-B-001..012 |
| T-B-014 | B | [REFACTOR] Extract flattenWithLineage helper, add `@MX:ANCHOR: ExpandTree contract`, `@MX:WARN + @MX:REASON: goroutine pool cancellation propagation`, `@MX:ANCHOR: BudgetTracker.PreCheck (3-cap simultaneous enforcement)`, `@MX:NOTE: flattenWithLineage O(N) per claim` | — | (no new tests; rerun Phase B suite with -race) | T-B-013 |
| T-C-001 | C | [RED] Python `/decompose_query` returns breadth sub_queries test (LiteLLM mock) | REQ-003 (sub-query gen), REQ-009a | test_decompose_query_returns_breadth_sub_queries | — |
| T-C-002 | C | [RED] Python endpoint truncates excess sub-queries when LLM returns >breadth | REQ-003 | test_decompose_query_truncates_excess | T-C-001 |
| T-C-003 | C | [RED] Python endpoint validates input (breadth=0 → HTTP 400 on the sidecar; root_query/parent_query/parent_evidence_summary required fields) | REQ-009a | test_decompose_query_validates_input, test_decompose_query_requires_prompt_context_fields | T-C-001 |
| T-C-004 | C | [GREEN] deep_tree.py FastAPI route + LiteLLM gateway reuse + hardcoded prompt template; register in app.py; add `# @MX:NOTE: in-house decomposition prompt` | REQ-003, REQ-009a | (T-C-001..003 PASS via pytest) | T-C-001..003 |
| T-C-005 | C | [RED] Go-side HTTP client `researcher_http.go` integration test against httptest stub (request body includes root_query + parent_query + parent_evidence_summary, response parsed into sub_queries) | REQ-009a | TestResearcherHTTPDecomposeRoundTrip, TestResearcherHTTPPropagatesPromptContext | T-C-004 |
| T-C-006 | C | [GREEN] researcher_http.go implementation + wire into ExpandTree as Researcher interface | REQ-009a | (T-C-005 PASS) | T-C-005 |
| T-D-001 | D | [RED] Persistence atomic flush on depth-level join (`.tmp` + rename) + per-node Status transition flush | REQ-011a | TestPersistenceAtomicFlushOnDepthJoin, TestPersistencePerNodeTransitionFlush | T-B-013 |
| T-D-002 | D | [RED] JSON round-trip test (write tree → load → struct equality, including ReservedTokens + CostUSD fields) | REQ-010, REQ-011a, REQ-013 | TestPersistenceJSONRoundTrip | T-B-013 |
| T-D-003 | D | [RED] Postgres `deep_runs` summary row insert test (sqlx mock; total_cost_usd derived from sum(Node.CostUSD)) | REQ-011a, REQ-013 | TestPersistencePostgresInsert | T-B-013 |
| T-D-004 | D | [RED] SIGTERM mid-expand → Postgres row finalized with `status="failed"|"partial"`, completed_at within ±5s of crash | REQ-011a, NFR-008 | TestPersistenceCrashFinalizesPostgresRow | T-D-001 |
| T-D-005 | D | [RED] Reload-mode reclassify test (Pending/Expanding → Failed in-memory only; disk tree.json unchanged) | REQ-011b, NFR-008 | TestPersistenceReclassifyOnReload | T-D-001 |
| T-D-006 | D | [RED] Reload-mode read-only enforcement test (additional expand on reloaded tree returns error) | REQ-011b | TestPersistenceReloadTreeRejectsExpandAttempt | T-D-005 |
| T-D-007 | D | [RED] tree.json gzip size ≤ 200KB sample inspection test (default config sample fixture) | NFR-007 | TestPersistenceJSONGzipSizeBound | T-D-002 |
| T-D-008 | D | [GREEN] persistence.go implementation (atomic flush, JSON marshal, Postgres insert, reload reclassify) + `@MX:WARN + @MX:REASON: write-tmp-then-rename SIGTERM durability` | REQ-011a, REQ-011b, REQ-013, NFR-007, NFR-008 | (T-D-001..007 PASS) | T-D-001..007 |
| T-D-009 | D | Migration `deploy/postgres/migrations/0002_deep_runs.up.sql` + `0002_deep_runs.down.sql` (golang-migrate, OQ-1 RESOLVED) | REQ-011a | (manual: `migrate up && migrate down` clean round-trip) | T-D-003 |
| T-D-010 | D | [REFACTOR] persistence.go cleanup; rerun full Phase D suite; verify `goleak.VerifyNone(t)` on cancellation tests | — | (no new tests) | T-D-008 |
| T-E-001 | E | [RED] Metrics registration test (3 collectors registered: histogram, counter, plus existing DEEP-002 collectors untouched) | REQ-012, NFR-005 | TestMetricsRegistration | T-B-013 |
| T-E-002 | E | [RED] Cardinality bounded test (depth ∈ {0..5}, outcome ∈ {success, failed, budget_exceeded}; no user-input label values) | REQ-012, NFR-005 | TestMetricsCardinalityBounded, TestNoUnboundedLabels (extends `internal/obs/metrics/metrics_test.go`) | T-E-001 |
| T-E-003 | E | [RED] Per-node histogram observation + counter increment integration test against ExpandTree | REQ-012, NFR-005 | TestExpandTreeMetricsObserved | T-B-013, T-E-001 |
| T-E-004 | E | [RED] OTel span linkage test (parent_span_id propagation, trace tree depth == expansion depth) | REQ-012, NFR-006 | TestOTelSpanParentLinkage, TestOTelTraceDepthMatchesTreeDepth | T-B-013 |
| T-E-005 | E | [GREEN] tree_metrics.go collectors + pre-declared labels; modify `internal/obs/metrics/metrics.go` (registerDeepTree helper); modify `internal/obs/obs.go` (re-export); wire histogram/counter/OTel hooks into ExpandTree | REQ-012, NFR-005, NFR-006 | (T-E-001..004 PASS) | T-E-001..004 |
| T-E-006 | E | [RED] Integration end-to-end happy path test via httptest + stubbed sidecar (`tests/integration/deep_tree_test.go`); validates SSE/JSON wire compatibility with DEEP-002 | REQ-001, REQ-003, REQ-011a, REQ-012 | TestDeepTreeEndToEndHappyPath, TestDeepTreeDEEP002RegressionGreen | T-E-005, T-C-006 |
| T-E-007 | E | [RED] Integration fallback header + body invariant test (X-Deep-Tree-Fallback emitted out-of-band, body byte-identical to DEEP-002 single-shot fixture; 3 sub-scenarios: breadth=0, depth=0, both-zero) | REQ-005 | TestExpandTreeBreadthZeroFallback, TestExpandTreeDepthZeroFallback, TestExpandTreeBreadthAndDepthZeroFallback, TestFallbackHeaderEmittedAndBodyUnchanged | T-E-006 |
| T-E-008 | E | [GREEN] Wire fallback header emission + modify `internal/deepagent/agents.go` (DEEP-002) Researcher tree-mode branching; modify `internal/deepagent/config.go` (DEEP-002) DEEP_TREE_* env-var loader; modify `.env.example` + create `.moai/config/sections/deep.yaml` (tree.enabled, breadth/depth defaults, budgets, pricing.{model}) | REQ-001, REQ-005, REQ-013 | (T-E-007 PASS, T-E-006 still PASS) | T-E-007 |
| T-E-009 | E | [REFACTOR] Final pass — rerun full suite under `go test -race ./internal/deepagent/...`, `pytest services/researcher/tests/`, `go test -tags=integration ./tests/...`; verify pre-submission self-review checklist (plan.md §6); update progress.md | — | (entire RED+GREEN suite PASS) | T-E-008 |

---

## Phase A — Types Foundation + Budget Model

Goal: Define tree representation structs (with cost/budget fields) and the
budget tracker with reservation-lock semantics. No expansion logic yet.

Files touched (NEW): `internal/deepagent/tree_types.go`,
`internal/deepagent/tree_types_test.go`, `internal/deepagent/budget.go`,
`internal/deepagent/budget_test.go`.

### T-A-001 [RED]
- Test: TestTreeTypesJSONRoundTrip, TestNodeStatusExhaustiveSwitch, TestFlattenedClaimLineageInvariant (struct-level)
- REQ: REQ-DEEP3-010, REQ-DEEP3-013
- Scope: Author RED tests that pin the struct schema: Node fields (Depth, ParentID, BreadthIndex, Query, Status, Citations, Claims, TokensUsed, ReservedTokens, CostUSD), Tree-level fields (TotalTokensUsed, TotalReservedTokens, TotalCostUSD), TreeResult, FlattenedClaim with lineage_path + source_node_id. Verify NodeStatus enum exhaustive switch.
- Acceptance: Tests fail (compile error or unexported field) — confirms RED.
- Files touched: `internal/deepagent/tree_types_test.go` [NEW]

### T-A-002 [GREEN]
- Test: T-A-001 turns green
- REQ: REQ-DEEP3-010, REQ-DEEP3-013
- Scope: Implement struct definitions (fields only, no methods) and NodeStatus enum (`Pending`, `Expanding`, `Complete`, `Failed`, `BudgetExceeded`).
- Acceptance: T-A-001 PASS via `go test ./internal/deepagent/`.
- Files touched: `internal/deepagent/tree_types.go` [NEW]

### T-A-003 [RED]
- Test: TestBudgetPreCheckTokenExceeded, TestBudgetPreCheckStructuralCap, TestBudgetPreCheckHeadroomConservative
- REQ: REQ-DEEP3-006, REQ-DEEP3-007, NFR-DEEP3-004
- Scope: Write RED tests for BudgetTracker.PreCheck and Release semantics. Verify token cap detection, structural cap detection (1 + Σ breadth^i), and 25% headroom on `estimated_next_cost = parent.TokensUsed * breadth * 1.25`.
- Acceptance: Tests fail (BudgetTracker undefined).
- Files touched: `internal/deepagent/budget_test.go` [NEW]

### T-A-004 [RED][RACE]
- Test: TestBudgetReservationLockSerializesSiblings, TestBudgetReservationReleaseOnComplete
- REQ: REQ-DEEP3-006 (reservation lock), REQ-DEEP3-007 (atomic release), NFR-DEEP3-003
- Scope: Write `-race`-mode tests that spawn N=8 concurrent goroutines calling PreCheck on a shared BudgetTracker; assert (read + decision + reservation) is atomic — no goroutine observes intermediate state — and that release atomically reconciles ReservedTokens delta on completion. Use `sync.WaitGroup` + invariant checks.
- Acceptance: Tests must execute under `go test -race -count=10 ./internal/deepagent/...` without race detector firing once T-A-006 is green.
- Files touched: `internal/deepagent/budget_test.go` [NEW]

### T-A-005 [RED]
- Test: TestBudgetRootSeedEstimate, TestExpandTreeRootSeedTriggersImmediateBudgetFail (root-seed-slice scoped to BudgetTracker only)
- REQ: REQ-DEEP3-006 (root-node seed clause)
- Scope: Verify that when parent.TokensUsed is unavailable (root case), PreCheck uses DEEP_TREE_ROOT_TOKEN_ESTIMATE (default 5000, overridable via deep.yaml). Cover budget=4000 / seed=5000 → immediate fail.
- Acceptance: Tests fail until T-A-006 implements seed lookup.
- Files touched: `internal/deepagent/budget_test.go` [NEW]

### T-A-006 [GREEN]
- Test: T-A-003, T-A-004, T-A-005 turn green
- REQ: REQ-DEEP3-006, REQ-DEEP3-007
- Scope: Implement BudgetTracker with internal `sync.Mutex` (reservation lock). PreCheck holds the lock during (read TotalTokensUsed + decision + reservation increment). ReleaseOnComplete holds the same lock for (TotalTokensUsed += delta; TotalReservedTokens -= (Reserved - actual)) as a single critical section. Read root seed from `deep.yaml` `pricing` map injection.
- Acceptance: All Phase A RED tests PASS under `go test -race -count=5`.
- Files touched: `internal/deepagent/budget.go` [NEW]

### T-A-007 [REFACTOR][RACE]
- Test: Phase A suite, stress mode
- REQ: REQ-DEEP3-006, REQ-DEEP3-007, NFR-DEEP3-003
- Scope: Rerun `go test -race -count=10 ./internal/deepagent/budget_test.go`. Verify lock contention is not pathological — average PreCheck latency under N=8 concurrent callers < 1ms. Add `@MX:ANCHOR: BudgetTracker.PreCheck (3-cap simultaneous enforcement)` annotation.
- Acceptance: 10 race-mode iterations pass; no goroutine starvation observed.
- Files touched: `internal/deepagent/budget.go` (annotation), `internal/deepagent/budget_test.go` (stress harness)

---

## Phase B — Orchestrator Loop (ExpandTree)

Goal: BFS expand loop with per-depth errgroup join, budget integration,
flattenWithLineage, and edge-case handling. Mocked Researcher so LLM/fanout
dependencies are isolated.

Files touched (NEW): `internal/deepagent/tree.go`,
`internal/deepagent/tree_test.go`.

### T-B-001 [RED]
- Test: TestExpandTreeHappyPath
- REQ: REQ-DEEP3-001, REQ-DEEP3-003
- Scope: RED test — call ExpandTree(ctx, cfg{breadth=4, depth=3}, root_query, mockResearcher). Assert: total_nodes==85, max_depth_reached==3, all NodeStatusComplete.
- Acceptance: Test fails — ExpandTree undefined.
- Files touched: `internal/deepagent/tree_test.go` [NEW]

### T-B-002 [RED]
- Test: TestExpandTreeInvalidBreadth (breadth=9), TestExpandTreeInvalidDepth (depth=6); also covers leaf-node skip-decompose for depth=N node
- REQ: REQ-DEEP3-002, REQ-DEEP3-003 (leaf node)
- Scope: RED tests — invalid range returns structured error `{"error": "invalid_tree_config", ...}` mapping to HTTP 400. Leaf-depth node (Depth == cfg.depth) must skip `/decompose_query` and only call fanout.
- Acceptance: Tests fail.
- Files touched: `internal/deepagent/tree_test.go`

### T-B-003 [RED][RACE]
- Test: TestExpandTreeBFSOrdering
- REQ: REQ-DEEP3-004 (BFS invariant — depth N+1 starts only after depth N join)
- Scope: Inject timing instrumentation into mockResearcher (timestamp on each call). Assert no depth-N+1 timestamp precedes any depth-N completion timestamp. This validates the "fresh errgroup per depth level" design (plan.md §B3).
- Acceptance: Test fails until T-B-013 implements per-depth errgroup join.
- Files touched: `internal/deepagent/tree_test.go`

### T-B-004 [RED][RACE][GOLEAK]
- Test: TestExpandTreeConcurrentBreadth
- REQ: REQ-DEEP3-004
- Scope: Verify that within a single depth level, breadth goroutines execute in parallel (goroutine ID tracking via runtime.Stack snapshot). Wrap with `defer goleak.VerifyNone(t)` to detect goroutine leaks on context cancellation.
- Acceptance: Test passes under `-race`; no goroutine leak.
- Files touched: `internal/deepagent/tree_test.go`

### T-B-005 [RED][RACE]
- Test: TestExpandTreeConcurrentBreadthBudgetRaceFree (100 iterations)
- REQ: REQ-DEEP3-006, REQ-DEEP3-007, NFR-DEEP3-003
- Scope: Worst-case sibling race test per acceptance §5.3 sub-scenario. Config: breadth=8, depth=2, DEEP_TREE_TOKEN_BUDGET=60000. Loop 100 iterations; each iteration asserts `final_total = sum(node.TokensUsed for completed) ≤ 60000`. Reservation lock + atomic release must enforce this deterministically (overshoot==0).
- Acceptance: 100/100 iterations satisfy the invariant under `go test -race -run TestExpandTreeConcurrentBreadthBudgetRaceFree -count=3`.
- Files touched: `internal/deepagent/tree_test.go`

### T-B-006 [RED]
- Test: TestExpandTreeBudgetExceeded, TestExpandTreePartialReturn
- REQ: REQ-DEEP3-006, REQ-DEEP3-008, REQ-DEEP3-010
- Scope: budget=20000, average 2000 tokens/node. Assert HTTP 200 (degraded), tree.status=="budget_exceeded", flattened_claims non-empty (root + early nodes complete), Writer receives partial tree.
- Acceptance: Tests fail until GREEN T-B-013.
- Files touched: `internal/deepagent/tree_test.go`

### T-B-007 [RED]
- Test: TestExpandTreeLatencyP95 (per-node), TestExpandTreeEndToEndLatencyP95 (tree)
- REQ: NFR-DEEP3-001, NFR-DEEP3-002
- Scope: 50-iter mock test per NFR. Per-node p95 ≤ 30s. End-to-end p95 ≤ 240s (default config). Use injected sleep-driver Researcher with deterministic distribution.
- Acceptance: Statistical bounds satisfied across 50 iterations.
- Files touched: `internal/deepagent/tree_test.go`

### T-B-008 [RED]
- Test: TestCitationsDisjointlyOwned
- REQ: REQ-DEEP3-009b
- Scope: For every pair (a, b) of nodes in a completed tree, assert `&a.Citations[0]` does not lie within `b.Citations`'s memory range (slice identity disjoint). Same doc_id in different slices is allowed.
- Acceptance: Test fails until GREEN copies fanout results into per-node owned slices.
- Files touched: `internal/deepagent/tree_test.go`

### T-B-009 [RED]
- Test: TestFlattenedClaimLineageProperty (hypothesis-go property test, 100 random trees)
- REQ: REQ-DEEP3-009a, REQ-DEEP3-010
- Scope: For 100 random (breadth, depth) configurations: every leaf claim has lineage_path[0] == root_query prefix; len(lineage_path) == source_node.depth + 1; reverse-traversal via source_node_id reaches root (ParentID == "").
- Acceptance: 100/100 random trees satisfy invariants.
- Files touched: `internal/deepagent/tree_test.go`

### T-B-010 [RED]
- Test: TestNodeCostUSDComputedOnComplete, TestTreeTotalCostUSDSumsCompletedNodes
- REQ: REQ-DEEP3-013
- Scope: After ExpandTree completes, each completed node has CostUSD == TokensUsed * pricing[DEEP_TREE_DECOMPOSE_MODEL]. tree.TotalCostUSD == sum(node.CostUSD for node.Status==Complete). Unregistered model → CostUSD==0 + warning logged.
- Acceptance: Tests fail until GREEN computes per-node cost.
- Files touched: `internal/deepagent/tree_test.go`

### T-B-011 [RED]
- Test: TestTreeMemoryFootprintUnder100MBWorstCase
- REQ: NFR-DEEP3-009
- Scope: Force worst-case config (breadth=8, depth=5). Measure in-memory state size at each depth-level join (per acceptance §5.8). Assert ≤ 100MB. If 100MB exceeded, frontier truncation (mark Frontier as BudgetExceeded) must restore < 100MB before next depth level. Counter `usearch_deep_tree_total_tokens{outcome="budget_exceeded"}` += 1.
- Acceptance: Memory bound respected; truncation behavior triggered when injected node footprint exceeds threshold.
- Files touched: `internal/deepagent/tree_test.go`

### T-B-012 [RED]
- Test: TestExpandTreeDepthOneSingleLevel
- REQ: REQ-DEEP3-003 (leaf node skip-decompose)
- Scope: cfg{breadth=4, depth=1}. Assert total_nodes==5, max_depth_reached==1, all 4 leaves have non-empty Citations (fanout result), no /decompose_query call at depth=1. Prometheus depth label values == {0, 1}.
- Acceptance: Test fails until GREEN.
- Files touched: `internal/deepagent/tree_test.go`

### T-B-013 [GREEN]
- Test: All Phase B RED tests (T-B-001..012) turn green
- REQ: REQ-DEEP3-001, REQ-DEEP3-002, REQ-DEEP3-003, REQ-DEEP3-004, REQ-DEEP3-006, REQ-DEEP3-007, REQ-DEEP3-008, REQ-DEEP3-009a, REQ-DEEP3-009b, REQ-DEEP3-010, REQ-DEEP3-013, NFR-DEEP3-001, NFR-DEEP3-002, NFR-DEEP3-003, NFR-DEEP3-004, NFR-DEEP3-009
- Scope: Implement ExpandTree(ctx, cfg, root_query, researcher) — root Node creation with deterministic ID `hash(run_id || "root")`, BFS expand loop with a fresh `errgroup.WithContext` per depth level (joining before advancing to depth N+1), reservation-lock-guarded pre-check + atomic release, per-node `context.WithTimeout(DEEP_TREE_NODE_TIMEOUT_MS)`, flattenWithLineage helper, citation slice ownership (per-node copy), CostUSD computation on completion, in-memory footprint guard with frontier truncation, input range validation (breadth ∈ [1,8], depth ∈ [1,5]).
- Acceptance: All Phase B RED tests PASS under `go test -race ./internal/deepagent/...`.
- Files touched: `internal/deepagent/tree.go` [NEW]

### T-B-014 [REFACTOR]
- Test: Rerun Phase B suite under `-race`
- REQ: All Phase B REQs
- Scope: Extract flattenWithLineage as a private helper. Add MX tags: `@MX:ANCHOR: ExpandTree tree expansion contract — downstream Writer dependency` on ExpandTree; `@MX:WARN: bounded concurrency via errgroup` + `@MX:REASON: parent context cancellation propagation through child goroutines` on goroutine pool spawn; `@MX:ANCHOR: BudgetTracker.PreCheck (3-cap simultaneous enforcement)` on PreCheck (Phase A reinforcement); `@MX:NOTE: flattenWithLineage — lineage path reconstruction algorithm, complexity O(N) per claim` on helper.
- Acceptance: All tests still PASS; MX annotations present (grep verification).
- Files touched: `internal/deepagent/tree.go`, `internal/deepagent/budget.go`

---

## Phase C — Python Sub-Agent (decompose_query Sidecar)

Goal: FastAPI `/decompose_query` route in `services/researcher/` plus the
Go-side HTTP client wiring it into ExpandTree's Researcher interface.

Language tooling: Python (pytest + ruff). Run `pytest services/researcher/tests/test_deep_tree.py` and `ruff check services/researcher/src/researcher/deep_tree.py` independently from the Go test suite.

Files touched (NEW): `services/researcher/src/researcher/deep_tree.py`,
`services/researcher/tests/test_deep_tree.py`,
`internal/deepagent/researcher_http.go`,
`internal/deepagent/researcher_http_test.go`.
Files touched (MODIFY): `services/researcher/src/researcher/app.py`.

### T-C-001 [RED]
- Test: test_decompose_query_returns_breadth_sub_queries (pytest)
- REQ: REQ-DEEP3-003 (sub-query generation), REQ-DEEP3-009a (prompt context flow)
- Scope: POST `/decompose_query` with `{root_query, parent_query, parent_evidence_summary, breadth=4}`. Mock LiteLLM gateway to return 4 sub-queries. Assert HTTP 200 + JSON `{sub_queries: [4 items]}`.
- Acceptance: Test fails — endpoint undefined.
- Files touched: `services/researcher/tests/test_deep_tree.py` [NEW]

### T-C-002 [RED]
- Test: test_decompose_query_truncates_excess
- REQ: REQ-DEEP3-003
- Scope: LLM mock returns 6 sub-queries; endpoint must truncate to 4 (breadth=4) and emit a warning log.
- Acceptance: Test fails until GREEN.
- Files touched: `services/researcher/tests/test_deep_tree.py`

### T-C-003 [RED]
- Test: test_decompose_query_validates_input, test_decompose_query_requires_prompt_context_fields
- REQ: REQ-DEEP3-009a
- Scope: breadth=0 → HTTP 400 on sidecar layer. Missing root_query, parent_query, or parent_evidence_summary → HTTP 400 with field-level detail (FastAPI Pydantic validation).
- Acceptance: Tests fail until GREEN.
- Files touched: `services/researcher/tests/test_deep_tree.py`

### T-C-004 [GREEN]
- Test: T-C-001..003 turn green
- REQ: REQ-DEEP3-003, REQ-DEEP3-009a
- Scope: Implement FastAPI route in deep_tree.py — Pydantic request model with `root_query`, `parent_query`, `parent_evidence_summary`, `breadth` fields; LiteLLM gateway reuse (synthesis.py pattern); hardcoded decomposition prompt template inspired by gpt-researcher upstream (in-house, no PyPI dependency). Register route in app.py. Add `# @MX:NOTE: in-house decomposition prompt; gpt-researcher convention reference per research §6 D5`.
- Acceptance: `pytest services/researcher/tests/test_deep_tree.py` PASS; `ruff check` clean.
- Files touched: `services/researcher/src/researcher/deep_tree.py` [NEW], `services/researcher/src/researcher/app.py` [MODIFY]

### T-C-005 [RED]
- Test: TestResearcherHTTPDecomposeRoundTrip, TestResearcherHTTPPropagatesPromptContext
- REQ: REQ-DEEP3-009a
- Scope: Go-side HTTP client test against `httptest.NewServer` stub. Assert request body includes root_query + parent_query + parent_evidence_summary; response parsed into `[]string` sub_queries.
- Acceptance: Tests fail until T-C-006.
- Files touched: `internal/deepagent/researcher_http_test.go` [NEW]

### T-C-006 [GREEN]
- Test: T-C-005 turns green; ExpandTree integration via Researcher interface
- REQ: REQ-DEEP3-009a
- Scope: Implement researcher_http.go — `ResearcherHTTPClient` struct implementing the Researcher interface defined in tree.go; configurable endpoint URL via DEEP_TREE_DECOMPOSE_URL. Wire into ExpandTree.
- Acceptance: T-C-005 PASS; Phase B suite still green with HTTP client substituted for mock.
- Files touched: `internal/deepagent/researcher_http.go` [NEW]

---

## Phase D — Persistence + Crash Recovery

Goal: Atomic flush to `.moai/runs/{run_id}/tree.json` on every depth-level
join + Postgres `deep_runs` summary insert. Reload-mode reclassify.

Files touched (NEW): `internal/deepagent/persistence.go`,
`internal/deepagent/persistence_test.go`,
`deploy/postgres/migrations/0002_deep_runs.up.sql`,
`deploy/postgres/migrations/0002_deep_runs.down.sql`.

### T-D-001 [RED]
- Test: TestPersistenceAtomicFlushOnDepthJoin, TestPersistencePerNodeTransitionFlush
- REQ: REQ-DEEP3-011a
- Scope: Verify atomic flush — write to `.tmp` then rename — on every depth-level join AND on every node Status transition (Pending→Expanding, Expanding→{Complete,Failed,BudgetExceeded}). Inject filesystem mock to capture write/rename calls.
- Acceptance: Tests fail until GREEN.
- Files touched: `internal/deepagent/persistence_test.go` [NEW]

### T-D-002 [RED]
- Test: TestPersistenceJSONRoundTrip
- REQ: REQ-DEEP3-010, REQ-DEEP3-011a, REQ-DEEP3-013
- Scope: Marshal a full TreeResult (with ReservedTokens and CostUSD populated) → unmarshal → struct equality. Verify backward compatibility marker (schema version field) inspired by `internal/deepreport/types.go`.
- Acceptance: Tests fail until GREEN.
- Files touched: `internal/deepagent/persistence_test.go`

### T-D-003 [RED]
- Test: TestPersistencePostgresInsert
- REQ: REQ-DEEP3-011a, REQ-DEEP3-013
- Scope: sqlx mock — single INSERT into `deep_runs` with columns `{run_id, query, breadth, depth, total_nodes, total_tokens, total_cost_usd, status, started_at, completed_at}`. total_cost_usd derived from tree.TotalCostUSD.
- Acceptance: Tests fail until GREEN.
- Files touched: `internal/deepagent/persistence_test.go`

### T-D-004 [RED]
- Test: TestPersistenceCrashFinalizesPostgresRow
- REQ: REQ-DEEP3-011a, NFR-DEEP3-008
- Scope: Simulate SIGTERM mid-expand. After graceful shutdown, Postgres row finalized with `status="failed"` or `status="partial"`, `completed_at` within ±5s of crash time.
- Acceptance: Tests fail until GREEN.
- Files touched: `internal/deepagent/persistence_test.go`

### T-D-005 [RED]
- Test: TestPersistenceReclassifyOnReload
- REQ: REQ-DEEP3-011b, NFR-DEEP3-008
- Scope: Pre-populate tree.json with nodes in {Pending, Expanding, Complete} states. Invoke reload-mode entry point. Assert Pending/Expanding nodes → Failed in-memory; disk tree.json byte hash unchanged before and after reload (in-memory transform only).
- Acceptance: Tests fail until GREEN.
- Files touched: `internal/deepagent/persistence_test.go`

### T-D-006 [RED]
- Test: TestPersistenceReloadTreeRejectsExpandAttempt
- REQ: REQ-DEEP3-011b
- Scope: Attempt to expand a reloaded tree → returns error (read-only enforcement). Verify no goroutine spawned, no Researcher call made.
- Acceptance: Tests fail until GREEN.
- Files touched: `internal/deepagent/persistence_test.go`

### T-D-007 [RED]
- Test: TestPersistenceJSONGzipSizeBound
- REQ: NFR-DEEP3-007
- Scope: Generate a default-config tree.json sample (85 nodes, average 2KB payload each); gzip; assert compressed size ≤ 200KB. This is documentation-driven evidence — no runtime size gate (per NFR-DEEP3-007 rationale: NFR-003+NFR-004 bound the input).
- Acceptance: Test passes deterministically against fixture.
- Files touched: `internal/deepagent/persistence_test.go`

### T-D-008 [GREEN]
- Test: T-D-001..007 turn green
- REQ: REQ-DEEP3-011a, REQ-DEEP3-011b, REQ-DEEP3-013, NFR-DEEP3-007, NFR-DEEP3-008
- Scope: Implement persistence.go: AtomicFlush (write `.tmp` + os.Rename), JSON marshal with schema version, Postgres summary insert via sqlx, reload-mode reclassify (in-memory only, returns read-only tree handle). Wire into ExpandTree as `persistence.OnDepthLevelJoin(tree)` and `persistence.OnNodeTransition(node)` hooks. Add `@MX:WARN: write-tmp-then-rename pattern required` + `@MX:REASON: partial write durability under SIGTERM` on AtomicFlush.
- Acceptance: All Phase D RED tests PASS.
- Files touched: `internal/deepagent/persistence.go` [NEW]

### T-D-009
- Test: Manual — `migrate up && migrate down` round-trip
- REQ: REQ-DEEP3-011a
- Scope: Create `deploy/postgres/migrations/0002_deep_runs.up.sql` with the `deep_runs` table schema and matching `0002_deep_runs.down.sql`. Tool: `golang-migrate/migrate` (file pair format). Sequence `0002` (next after existing `0001_create_docs.sql`). OQ-1 RESOLVED 2026-05-21.
- Acceptance: `migrate -path deploy/postgres/migrations -database $DATABASE_URL up` then `down` clean round-trip; schema matches REQ-DEEP3-011a Postgres columns including `total_cost_usd numeric(10,4)`.
- Files touched: `deploy/postgres/migrations/0002_deep_runs.up.sql` [NEW], `deploy/postgres/migrations/0002_deep_runs.down.sql` [NEW]

### T-D-010 [REFACTOR]
- Test: Phase D suite rerun
- REQ: All Phase D REQs
- Scope: persistence.go cleanup; rerun full Phase D suite under `-race`; verify `goleak.VerifyNone(t)` on cancellation-related tests.
- Acceptance: All Phase D tests PASS; no goroutine leak.
- Files touched: `internal/deepagent/persistence.go`

---

## Phase E — Observability + Integration

Goal: Prometheus metrics + OTel spans + integration tests including
fallback header invariant + DEEP-002 regression.

Files touched (NEW): `internal/deepagent/tree_metrics.go`,
`internal/deepagent/tree_metrics_test.go`,
`tests/integration/deep_tree_test.go`,
`.moai/config/sections/deep.yaml`.
Files touched (MODIFY): `internal/obs/metrics/metrics.go`,
`internal/obs/obs.go`,
`internal/deepagent/agents.go` (DEEP-002),
`internal/deepagent/config.go` (DEEP-002),
`.env.example`.

### T-E-001 [RED]
- Test: TestMetricsRegistration
- REQ: REQ-DEEP3-012, NFR-DEEP3-005
- Scope: Verify 2 collectors registered (`usearch_deep_tree_node_expand_seconds`, `usearch_deep_tree_total_tokens`). Existing DEEP-002 collectors must remain untouched.
- Acceptance: Test fails until GREEN.
- Files touched: `internal/deepagent/tree_metrics_test.go` [NEW]

### T-E-002 [RED]
- Test: TestMetricsCardinalityBounded; extends `internal/obs/metrics/metrics_test.go::TestNoUnboundedLabels`
- REQ: REQ-DEEP3-012, NFR-DEEP3-005
- Scope: Verify label values are pre-declared compile-time constants — `depth ∈ {0,1,2,3,4,5}`, `outcome ∈ {success, failed, budget_exceeded}` for histogram; `outcome ∈ {pass, budget_exceeded}` for counter. No user-input strings reach the label.
- Acceptance: Test fails until GREEN; cardinality count == 18 + 2.
- Files touched: `internal/deepagent/tree_metrics_test.go`, extension to `internal/obs/metrics/metrics_test.go`

### T-E-003 [RED]
- Test: TestExpandTreeMetricsObserved
- REQ: REQ-DEEP3-012, NFR-DEEP3-005
- Scope: After ExpandTree completes with N nodes, histogram has N observations; counter `usearch_deep_tree_total_tokens{outcome="pass"}` += 1 on success, `{outcome="budget_exceeded"}` += 1 on degraded.
- Acceptance: Test fails until GREEN T-E-005.
- Files touched: `internal/deepagent/tree_metrics_test.go`

### T-E-004 [RED]
- Test: TestOTelSpanParentLinkage, TestOTelTraceDepthMatchesTreeDepth
- REQ: REQ-DEEP3-012, NFR-DEEP3-006
- Scope: Verify OTel tracing — each child node's span references parent node's span via `parent_span_id`. Trace tree depth equals expansion depth.
- Acceptance: Test fails until GREEN.
- Files touched: `internal/deepagent/tree_metrics_test.go`

### T-E-005 [GREEN]
- Test: T-E-001..004 turn green
- REQ: REQ-DEEP3-012, NFR-DEEP3-005, NFR-DEEP3-006
- Scope: Implement tree_metrics.go with pre-declared label values. Register collectors in `internal/obs/metrics/metrics.go::registerDeepTree(pr)`. Re-export in `internal/obs/obs.go` as `obs.DeepTreeNodeExpand`, `obs.DeepTreeTotalTokens`. Wire histogram observation + counter increment + OTel span emission into ExpandTree (per-node hooks).
- Acceptance: All Phase E unit tests PASS.
- Files touched: `internal/deepagent/tree_metrics.go` [NEW], `internal/obs/metrics/metrics.go` [MODIFY], `internal/obs/obs.go` [MODIFY], `internal/deepagent/tree.go` [MODIFY for hooks]

### T-E-006 [RED]
- Test: TestDeepTreeEndToEndHappyPath, TestDeepTreeDEEP002RegressionGreen
- REQ: REQ-DEEP3-001, REQ-DEEP3-003, REQ-DEEP3-011a, REQ-DEEP3-012
- Scope: End-to-end integration test via `httptest.NewServer` + stubbed Python sidecar. Default config request → assert HTTP 200 + 85 nodes + tree.json on disk + Postgres row + Prometheus scrape contains expected metrics. Run DEEP-002 acceptance fixtures unchanged — `mode=agents` single-shot response byte-identical to baseline.
- Acceptance: Both tests fail until GREEN T-E-008.
- Files touched: `tests/integration/deep_tree_test.go` [NEW]

### T-E-007 [RED]
- Test: TestExpandTreeBreadthZeroFallback, TestExpandTreeDepthZeroFallback, TestExpandTreeBreadthAndDepthZeroFallback, TestFallbackHeaderEmittedAndBodyUnchanged
- REQ: REQ-DEEP3-005
- Scope: For each of 3 sub-scenarios (breadth=0, depth=0, both-zero): assert HTTP 200, response header `X-Deep-Tree-Fallback ∈ {breadth_zero, depth_zero}` (both-zero → breadth_zero priority), response body byte-identical to DEEP-002 single-shot fixture (hash comparison). No tree.json file created on disk. The integration-level invariant: header is out-of-band side channel, body unchanged.
- Acceptance: Tests fail until GREEN T-E-008.
- Files touched: `tests/integration/deep_tree_test.go`

### T-E-008 [GREEN]
- Test: T-E-006, T-E-007 turn green
- REQ: REQ-DEEP3-001, REQ-DEEP3-005, REQ-DEEP3-013
- Scope: (a) Implement fallback header emission in DEEP-002 Researcher branching layer — when cfg.breadth==0 OR cfg.depth==0, return X-Deep-Tree-Fallback header + delegate to single-shot Researcher (REQ-DEEP2-005) without body mutation. (b) Modify `internal/deepagent/agents.go` (DEEP-002) — Researcher checks DEEP_TREE_ENABLED + request body `tree` field; routes to ExpandTree (tree.go) or single-shot. (c) Modify `internal/deepagent/config.go` — load `DEEP_TREE_*` env-vars. (d) Create `.moai/config/sections/deep.yaml` with `tree.enabled`, `tree.breadth`, `tree.depth`, budget defaults, `pricing.{model}` map for REQ-DEEP3-013 cost lookup, `DEEP_TREE_ROOT_TOKEN_ESTIMATE` override. (e) Update `.env.example` documenting all DEEP_TREE_* variables.
- Acceptance: T-E-006 + T-E-007 PASS; DEEP-002 regression suite still green.
- Files touched: `internal/deepagent/agents.go` [MODIFY], `internal/deepagent/config.go` [MODIFY], `.moai/config/sections/deep.yaml` [NEW], `.env.example` [MODIFY]

### T-E-009 [REFACTOR]
- Test: Full project suite under -race, integration, and Python
- REQ: All
- Scope: Final pass — `go test -race ./internal/deepagent/...`, `pytest services/researcher/tests/`, `go test -tags=integration ./tests/...`, verify all 11 items in plan.md §6 Pre-submission Self-Review Checklist, update `.moai/specs/SPEC-DEEP-003/progress.md`.
- Acceptance: All gates green; checklist 11/11 ticked.
- Files touched: `.moai/specs/SPEC-DEEP-003/progress.md`

---

## MX Tag Annotations (Plan)

Per plan.md §4 MX Tag Strategy. The following annotations MUST be present
after T-B-014 and T-D-008 REFACTOR passes:

| Function | Tag | Reason |
|----------|-----|--------|
| `internal/deepagent/tree.go::ExpandTree` | `@MX:ANCHOR` | BFS expansion entry point; downstream Writer contract; high fan_in |
| `internal/deepagent/tree.go` errgroup.Go spawn site | `@MX:WARN` + `@MX:REASON` | Bounded concurrency; parent context cancellation propagation through child goroutines |
| `internal/deepagent/budget.go::BudgetTracker.PreCheck` | `@MX:ANCHOR` | Budget invariant — 3-cap simultaneous enforcement; high fan_in (called by every ExpandTree node iteration) |
| `internal/deepagent/persistence.go::AtomicFlush` | `@MX:WARN` + `@MX:REASON` | Write-tmp-then-rename pattern required; partial write durability under SIGTERM |
| `internal/deepagent/tree.go::flattenWithLineage` | `@MX:NOTE` | Lineage path reconstruction algorithm; complexity O(N) per claim |
| `services/researcher/src/researcher/deep_tree.py::decompose_query` | `# @MX:NOTE` | In-house decomposition prompt; gpt-researcher convention inspiration per research.md §6 D5 |

---

## Verification Checklist

REQ Coverage Matrix (every REQ has ≥1 dedicated task):

- [ ] REQ-DEEP3-001 → T-B-001, T-B-013, T-E-008
- [ ] REQ-DEEP3-002 → T-B-002, T-B-013
- [ ] REQ-DEEP3-003 → T-B-001, T-B-002 (leaf), T-B-012, T-B-013, T-C-001, T-C-002, T-C-004
- [ ] REQ-DEEP3-004 → T-B-003, T-B-004, T-B-013
- [ ] REQ-DEEP3-005 → T-E-007, T-E-008
- [ ] REQ-DEEP3-006 → T-A-003, T-A-004, T-A-005, T-A-006, T-A-007, T-B-005, T-B-006, T-B-013
- [ ] REQ-DEEP3-007 → T-A-004, T-A-006, T-B-005, T-B-013
- [ ] REQ-DEEP3-008 → T-B-006, T-B-013
- [ ] REQ-DEEP3-009a → T-B-009, T-C-001, T-C-003, T-C-004, T-C-005, T-C-006
- [ ] REQ-DEEP3-009b → T-B-008, T-B-013
- [ ] REQ-DEEP3-010 → T-A-001, T-A-002, T-B-006, T-B-009, T-B-013, T-D-002
- [ ] REQ-DEEP3-011a → T-D-001, T-D-002, T-D-003, T-D-004, T-D-008, T-D-009, T-E-006
- [ ] REQ-DEEP3-011b → T-D-005, T-D-006, T-D-008
- [ ] REQ-DEEP3-012 → T-E-001, T-E-002, T-E-003, T-E-004, T-E-005, T-E-006
- [ ] REQ-DEEP3-013 → T-A-001, T-A-002, T-B-010, T-B-013, T-D-002, T-D-003, T-D-008, T-E-008

NFR Coverage (every NFR has ≥1 quantitative assertion task):

- [ ] NFR-DEEP3-001 (per-node p95 ≤ 30s) → T-B-007 (50-iter)
- [ ] NFR-DEEP3-002 (tree e2e p95 ≤ 4min) → T-B-007 (50-iter)
- [ ] NFR-DEEP3-003 (token budget race-free) → T-A-004, T-A-007, T-B-005 (100-iter race)
- [ ] NFR-DEEP3-004 (structural cap) → T-A-003, T-B-013
- [ ] NFR-DEEP3-005 (Prometheus cardinality) → T-E-001, T-E-002, T-E-003
- [ ] NFR-DEEP3-006 (OTel span linkage) → T-E-004, T-E-005
- [ ] NFR-DEEP3-007 (tree.json ≤ 200KB gzipped) → T-D-007
- [ ] NFR-DEEP3-008 (crash recovery) → T-D-004, T-D-005, T-D-006
- [ ] NFR-DEEP3-009 (in-memory ≤ 100MB) → T-B-011, T-B-013

Concurrency-Sensitive Tasks (require `go test -race`):

- [ ] T-A-004 — Reservation lock serializes siblings (Phase A budget_test)
- [ ] T-A-007 — Lock contention stress (10× -race iterations)
- [ ] T-B-003 — BFS ordering / per-depth errgroup join
- [ ] T-B-004 — Concurrent breadth + goleak
- [ ] T-B-005 — Concurrent breadth budget race-free (100 iterations)
- [ ] T-D-010 — Phase D suite under -race + goleak on cancellation

Python Sidecar Tasks (require `pytest` + `ruff`):

- [ ] T-C-001 — `/decompose_query` returns breadth sub-queries
- [ ] T-C-002 — Truncates excess sub-queries
- [ ] T-C-003 — Input validation + required prompt-context fields
- [ ] T-C-004 — GREEN implementation + app.py route registration

Definition of Done (mirrors acceptance.md):

- [ ] All Phase A, B, C, D, E task RED tests PASS after their corresponding GREEN.
- [ ] All 13 functional REQs + 2 split sub-REQs covered (REQ-001, 002, 003, 004, 005, 006, 007, 008, 009a, 009b, 010, 011a, 011b, 012, 013).
- [ ] All 9 NFRs validated by quantitative tests.
- [ ] Coverage report ≥ 85% (quality.yaml coverage_target).
- [ ] `go test -race ./internal/deepagent/...` PASS (zero data races).
- [ ] `goleak.VerifyNone(t)` PASS on all cancellation tests (T-B-004, T-D-010).
- [ ] `pytest services/researcher/tests/test_deep_tree.py` PASS; `ruff check` clean.
- [ ] `go test -tags=integration ./tests/...` PASS.
- [ ] DEEP-002 acceptance suite 100% green (regression — T-E-006).
- [ ] Prometheus cardinality guard PASS (T-E-002).
- [ ] tree.json sample gzip ≤ 200KB (T-D-007).
- [ ] Postgres migration up/down clean (T-D-009 — golang-migrate, 0002 sequence).
- [ ] All MX tag annotations present (T-B-014, T-D-008).
- [ ] `.moai/specs/SPEC-DEEP-003/progress.md` updated (T-E-009).

---

## Open Items (Blockers Before Run Phase)

- **OQ-1 — RESOLVED 2026-05-21**: golang-migrate/migrate, sequence `0002`. See spec.md §8. T-D-009 unblocked.
- (No remaining blockers)

---

*End of tasks.md (SPEC-DEEP-003 v0.1.2 — 44 tasks across 5 phases).*
