---
id: SPEC-DEEP-003
version: 0.1.2
status: draft
created: 2026-05-21
updated: 2026-05-21
author: limbowl
title: Implementation Plan — Tree exploration for /deep multi-agent
---

# SPEC-DEEP-003 Implementation Plan

본 문서는 SPEC-DEEP-003(트리 익스플로러) 구현을 위한 TDD 기반 5-phase
plan이다. quality.yaml의 `development_mode: tdd` 설정과 정합한다(RED →
GREEN → REFACTOR).

---

## 1. Phase Breakdown

본 SPEC은 다음 5개 Phase로 분해된다. Phase 간 dependency가 있으므로
sequential 진행을 권장한다. Phase 내부는 TDD cycle.

### Phase A — 데이터 구조 + Budget Model (Priority High)

**목표**: tree 표현 자료구조와 budget tracker를 코드 없이 정의하고
RED 테스트 작성.

**Tasks**:

- A1: `internal/deepagent/tree_types.go` 신규 — `Node`, `NodeStatus`,
  `NodeCitation`, `NodeClaim`, `TreeResult`, `FlattenedClaim` struct
  정의(필드만, 메소드 없음). `Node`는 다음 cost/budget 필드를 포함
  SHALL 한다: `TokensUsed int64`(actual LLM 토큰 누계),
  `ReservedTokens int64`(REQ-DEEP3-006 reservation lock에서 기록되는
  pre-check 예약치), `CostUSD float64`(REQ-DEEP3-013 per-node cost).
  Tree 수준 필드: `TotalTokensUsed int64`, `TotalReservedTokens int64`,
  `TotalCostUSD float64`.
- A2: `internal/deepagent/tree_types_test.go` — JSON marshaling
  round-trip 테스트, NodeStatus enum exhaustive switch 테스트.
- A3: `internal/deepagent/budget.go` — `BudgetTracker` struct,
  `PreCheck(node) BudgetDecision` 메소드 시그니처(구현 미수행).
- A4: `internal/deepagent/budget_test.go` — RED 테스트들:
  `TestBudgetPreCheckTokenExceeded`, `TestBudgetPreCheckStructuralCap`,
  `TestBudgetPreCheckHeadroomConservative`(25% headroom 검증),
  `TestBudgetReservationLockSerializesSiblings`(동시 N=8 goroutine
  pre-check 호출 시 race-free reservation/decision verification),
  `TestBudgetReservationReleaseOnComplete`(actual < reserved 케이스에서
  잉여 release 검증).
- A5: A3 GREEN 구현 — `BudgetTracker` 메소드 minimal impl.
- A6: REFACTOR — 코드 정리, A2/A4 테스트 통과 확인.

**Verification**: `go test -race ./internal/deepagent/...` 통과,
A2+A4 테스트 모두 PASS.

### Phase B — Orchestrator Loop (Priority High)

**목표**: BFS expand loop + errgroup concurrency 구현.

**Tasks**:

- B1: `internal/deepagent/tree.go` 시그니처 — `func ExpandTree(ctx,
  cfg TreeConfig, root Query, researcher Researcher) (TreeResult, error)`.
- B2: `internal/deepagent/tree_test.go` — RED 테스트들:
  `TestExpandTreeHappyPath` (default breadth=4, depth=3, mocked
  Researcher, expected 85 nodes), `TestExpandTreeBFSOrdering`(depth N+1
  expand는 depth N 완료 후만 시작), `TestExpandTreeConcurrentBreadth`
  (동일 depth의 N개 노드가 parallel — goroutine ID 추적).
- B3: B1 GREEN — `errgroup.WithContext` + BFS queue 구현. mock
  Researcher interface로 LLM/fanout dependency 격리. **ExpandTree는
  매 depth level마다 새로운 `errgroup`을 생성 SHALL 한다 — 동일 depth의
  모든 sibling goroutine이 `Wait()`로 join된 후에야 depth N+1로 진행
  한다(BFS invariant per REQ-DEEP3-004). 이는 cross-depth race를
  원천 차단하고 REQ-DEEP3-006 reservation lock과 함께 budget invariant를
  보존한다.**
- B4: REFACTOR — 코드 정리.

**Verification**: B2 테스트 모두 PASS, `-race` 통과.

**MX Tag Plan (Phase B)**:

- `tree.go:ExpandTree` 함수는 high fan_in(BFS expansion entry point)
  이므로 `@MX:ANCHOR: tree expansion contract; downstream Writer 의존`.
- `tree.go` 내 goroutine pool spawn 지점은 `@MX:WARN: bounded
  concurrency via errgroup; @MX:REASON: parent context cancellation
  propagation through child goroutines`.

### Phase C — Python Sub-Agent Integration (Priority High)

**목표**: `services/researcher/` 사이드카에 `/decompose_query` endpoint
추가.

**Tasks**:

- C1: `services/researcher/tests/test_deep_tree.py` 신규 — RED 테스트:
  `test_decompose_query_returns_breadth_sub_queries`(LiteLLM mock,
  4개 sub_queries 응답 검증), `test_decompose_query_truncates_excess`
  (LLM이 6개 반환 시 4개로 truncate), `test_decompose_query_validates_input`
  (breadth=0 → HTTP 400).
- C2: `services/researcher/src/researcher/deep_tree.py` GREEN 구현 —
  FastAPI route, LiteLLM gateway 재사용, prompt template hardcoded.
- C3: `services/researcher/src/researcher/app.py` MODIFY — 신규 route
  등록.
- C4: Go-side `internal/deepagent/tree.go` C-side HTTP client(`Researcher`
  interface 구현체)를 통한 통합 — `internal/deepagent/researcher_http.go`
  신규.
- C5: `internal/deepagent/researcher_http_test.go` — httptest server로
  stub, 통합 검증.

**Verification**: `pytest services/researcher/tests/test_deep_tree.py`
+ `go test ./internal/deepagent/...` 모두 PASS.

### Phase D — Persistence + Recovery (Priority Medium)

**목표**: tree.json atomic flush + Postgres summary insert.

**Tasks**:

- D1: `internal/deepagent/persistence_test.go` — RED:
  `TestPersistenceAtomicFlush`(write `.tmp` + rename 검증),
  `TestPersistenceJSONRoundTrip`(load partial tree, `NodeStatusFailed`
  reclassify), `TestPersistencePostgresInsert`(sqlx mock으로 row insert
  검증).
- D2: `internal/deepagent/persistence.go` GREEN 구현.
- D3: `deploy/postgres/migrations/0002_deep_runs.up.sql` + `0002_deep_runs.down.sql`
  신규(repo 표준 디렉토리, golang-migrate format — 기존 `0001_create_docs.sql`
  next sequence). spec.md §8 OQ-1 RESOLVED.
- D4: B Phase의 `tree.go`에 persistence hook 통합 — 매 노드 transition
  시 flush call.

**Verification**: D1 테스트 PASS, migration up/down clean.

**MX Tag Plan (Phase D)**:

- `persistence.go:AtomicFlush` 함수는 file I/O race condition 잠재
  지점이므로 `@MX:WARN: write-tmp-then-rename pattern required;
  @MX:REASON: partial write durability under SIGTERM`.

### Phase E — Observability Instrumentation (Priority Medium)

**목표**: Prometheus metrics + OTel spans.

**Tasks**:

- E1: `internal/deepagent/tree_metrics_test.go` — RED:
  `TestMetricsRegistration`(3개 collector 등록 검증),
  `TestMetricsCardinalityBounded`(label values pre-declared, no user
  input).
- E2: `internal/deepagent/tree_metrics.go` GREEN — collector 정의 +
  pre-declaration.
- E3: `internal/obs/metrics/metrics.go` MODIFY — `registerDeepTree(pr)`
  helper 등록.
- E4: `internal/obs/obs.go` MODIFY — re-export.
- E5: `tree.go`에 OTel span 발행 + Prometheus histogram observation
  hook 통합.
- E6: Integration test `tests/integration/deep_tree_test.go` 신규 —
  end-to-end happy path + budget exhaustion 시나리오 + Prometheus
  scrape 검증.

**Verification**: E1 + E6 PASS, `go test -tags=integration ./tests/...`
통과.

---

## 2. TDD Test Catalog

본 SPEC의 acceptance scenario를 다음 테스트 함수로 분해한다:

| Acceptance | Test Function | File |
|-----------|---------------|------|
| §5.1 Happy path | `TestExpandTreeHappyPath` | `internal/deepagent/tree_test.go` |
| §5.1 Latency p95 | `TestExpandTreeLatencyP95` (50-iter mock) | `internal/deepagent/tree_test.go` |
| §5.1 Metrics observation | `TestExpandTreeMetricsObserved` | `internal/deepagent/tree_metrics_test.go` |
| §5.2 Input validation | `TestExpandTreeInvalidBreadth` | `internal/deepagent/tree_test.go` |
| §5.2 Input validation | `TestExpandTreeInvalidDepth` | `internal/deepagent/tree_test.go` |
| §5.3 Budget exhaustion | `TestExpandTreeBudgetExceeded` | `internal/deepagent/tree_test.go` |
| §5.3 Partial tree return | `TestExpandTreePartialReturn` | `internal/deepagent/tree_test.go` |
| §5.3 Root-node seed pre-check | `TestExpandTreeRootSeedTriggersImmediateBudgetFail` | `internal/deepagent/tree_test.go` |
| §5.3 Parallel race race-free | `TestExpandTreeConcurrentBreadthBudgetRaceFree` (worst-case `breadth=8`, deterministic `final_total ≤ budget_cap`) | `internal/deepagent/tree_test.go` |
| §5.3 Reservation lock unit | `TestBudgetReservationLockSerializesSiblings`, `TestBudgetReservationReleaseOnComplete` | `internal/deepagent/budget_test.go` |
| §5.4 Lineage traceability | `TestFlattenedClaimLineageInvariant` | `internal/deepagent/tree_types_test.go` |
| §5.4 Lineage property test | `TestFlattenedClaimLineageProperty` (hypothesis-go) | `internal/deepagent/tree_test.go` |
| §5.4 Citation disjointness (REQ-009b) | `TestCitationsDisjointlyOwned` | `internal/deepagent/tree_test.go` |
| §5.5 Atomic flush per depth (REQ-011a) | `TestPersistenceAtomicFlushOnDepthJoin` | `internal/deepagent/persistence_test.go` |
| §5.5 Crash recovery reclassify (REQ-011b) | `TestPersistenceReclassifyOnReload` | `internal/deepagent/persistence_test.go` |
| §5.6a (edge) breadth=0 fallback | `TestExpandTreeBreadthZeroFallback` | `internal/deepagent/tree_test.go` |
| §5.6b (edge) depth=0 fallback | `TestExpandTreeDepthZeroFallback` | `internal/deepagent/tree_test.go` |
| §5.6c (edge) breadth=0 AND depth=0 | `TestExpandTreeBreadthAndDepthZeroFallback` | `internal/deepagent/tree_test.go` |
| §5.6 header + body invariant | `TestFallbackHeaderEmittedAndBodyUnchanged` (B2: header out-of-band, body byte-identical to DEEP-002) | `tests/integration/deep_tree_test.go` |
| §5.7 (edge) depth=1 | `TestExpandTreeDepthOneSingleLevel` | `internal/deepagent/tree_test.go` |
| §4 NFR-009 memory bound | `TestTreeMemoryFootprintUnder100MBWorstCase` | `internal/deepagent/tree_test.go` |
| §5.1 cost coverage (REQ-013) | `TestNodeCostUSDComputedOnComplete` + `TestTreeTotalCostUSDSumsCompletedNodes` | `internal/deepagent/tree_test.go` |

---

## 3. Risk Mitigation Table

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| LLM이 breadth 초과 sub-query emit | Medium | Medium | Python sidecar `/decompose_query`에서 truncate + warning log (Phase C2) |
| Goroutine leak on context cancel | Low | High | `goleak.VerifyNone(t)` 모든 Phase B 테스트에 적용 |
| Postgres migration drift | Low | Medium | `deploy/postgres/migrations/0002_*` versioning(golang-migrate, §8 OQ-1 RESOLVED) + `down.sql` 짝 보장 (Phase D3) |
| Tree.json corruption mid-flush | Low | Medium | Atomic write-tmp-rename pattern + `@MX:WARN` annotation (Phase D1) |
| Prometheus cardinality explosion | Low | High | Pre-declared enumerable label values + `metrics_test.go` cardinality guard (Phase E1) |
| OTel span overflow on large tree | Low | Low | Span batch flush + sample rate config (SPEC-OBS-001 inheritance) |

---

## 4. MX Tag Strategy Summary

본 SPEC 구현 중 추가될 MX 태그:

| Function | Tag Type | Reason |
|----------|----------|--------|
| `tree.go:ExpandTree` | `@MX:ANCHOR` | BFS expansion entry point; downstream Writer 계약 |
| `tree.go` goroutine pool spawn | `@MX:WARN` + `@MX:REASON` | Parent context cancellation propagation |
| `persistence.go:AtomicFlush` | `@MX:WARN` + `@MX:REASON` | Write durability under SIGTERM |
| `budget.go:PreCheck` | `@MX:ANCHOR` | Budget invariant — 3-cap simultaneous enforcement |
| `tree.go:flattenWithLineage` (helper) | `@MX:NOTE` | Lineage path reconstruction algorithm; complexity O(N) per claim |
| `deep_tree.py:decompose_query` (Python) | `# @MX:NOTE` | Sub-query generation prompt; in-house implementation per research §6 D5 |

---

## 5. Reference Implementation Map

| 신규 파일 | 가장 가까운 internal analog | 차용 패턴 |
|----------|---------------------------|----------|
| `internal/deepagent/tree.go` | `internal/fanout/dispatch.go` | `errgroup.WithContext` bounded concurrency |
| `internal/deepagent/budget.go` | `internal/llm/router.go` | State machine with threshold pre-check |
| `internal/deepagent/persistence.go` | `internal/deepreport/types.go` | JSON marshaling with schema version field |
| `internal/deepagent/tree_metrics.go` | `internal/obs/metrics/deepreport.go` (DEEP-001) | Collector registration + label pre-declaration |
| `services/researcher/src/researcher/deep_tree.py` | `services/researcher/src/researcher/synthesis.py` | FastAPI route + LiteLLM gateway pattern |
| `tests/integration/deep_tree_test.go` | (no existing M5 integration test) | `httptest.NewServer` + stub sidecar |

---

## 6. Pre-submission Self-Review Checklist

Run phase 완료 직전 다음 체크리스트를 수행한다:

- [ ] 모든 REQ-DEEP3-001..013(+009a/009b/011a/011b split 포함)이 적어도
  하나의 테스트 함수로 cover됨
- [ ] 모든 NFR-DEEP3-001..009가 quantitative assertion으로 검증됨
- [ ] coverage report ≥ 85% (per quality.yaml coverage_target)
- [ ] `go test -race ./internal/deepagent/...` PASS
- [ ] `pytest services/researcher/tests/test_deep_tree.py` PASS
- [ ] Integration test `tests/integration/deep_tree_test.go` PASS
- [ ] DEEP-002 acceptance suite 100% green (regression check)
- [ ] Prometheus cardinality guard test PASS
- [ ] `goleak.VerifyNone(t)` PASS on all cancellation tests
- [ ] `.moai/specs/SPEC-DEEP-003/progress.md` 진행 상황 기록
- [ ] tree.json sample 파일 inspection — gzip 압축 후 ≤ 200 KB (NFR-DEEP3-007)

---

*End of plan.md.*
