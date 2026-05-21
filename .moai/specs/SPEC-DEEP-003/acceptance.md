---
id: SPEC-DEEP-003
version: 0.1.2
status: draft
created: 2026-05-21
updated: 2026-05-21
author: limbowl
title: Acceptance Criteria — Tree exploration for /deep multi-agent
---

# SPEC-DEEP-003 Acceptance Criteria

본 문서는 SPEC-DEEP-003의 acceptance scenario를 Given/When/Then 형식으로
정의한다. 모든 시나리오는 `internal/deepagent/tree_test.go`,
`tests/integration/deep_tree_test.go`, 또는 `services/researcher/tests/
test_deep_tree.py`의 단일 테스트 함수로 검증 가능해야 한다.

---

## Scenario 5.1 — 정상 트리 확장 (default config: breadth=4, depth=3)

**REQ Coverage**: REQ-DEEP3-001, 003, 004, 011a, 012, 013; NFR-DEEP3-001,
002, 005, 006

### Given

- 사용자가 `/deep?mode=agents` 엔드포인트에 다음 요청을 전송:
  ```json
  {
    "request_id": "test-run-001",
    "query": "양자컴퓨터의 신약 개발 응용 현황",
    "lang": "ko",
    "tree": {"breadth": 4, "depth": 3}
  }
  ```
- Fanout adapter 4개(`reddit`, `hn`, `arxiv`, `searxng`) 등록 완료.
- LiteLLM proxy(SPEC-LLM-001) 정상 동작, `DEEP_TREE_DECOMPOSE_MODEL=
  claude-3-5-haiku-20241022` 응답 시간 평균 5초.
- `DEEP_TREE_TOKEN_BUDGET=60000`(default), `DEEP_TREE_NODE_TIMEOUT_MS=
  30000`(default).

### When

- 트리 익스플로러가 root Node 생성 후 BFS expand를 수행한다.
- 각 노드에서 (a) fanout.Dispatch 호출, (b) `/decompose_query` 호출하여
  4개 sub-query 생성, (c) 자식 Node 4개 생성.

### Then

- 응답 HTTP 200, body의 `status == "success"`.
- `total_nodes == 85` (1 + 4 + 16 + 64 = 85, depth=3까지 fully expanded).
- `max_depth_reached == 3`.
- End-to-end wall-clock ≤ 240초 (NFR-DEEP3-002 p95).
- Prometheus `usearch_deep_tree_node_expand_seconds{depth, outcome}`
  histogram에 85회 observation 기록.
- Prometheus `usearch_deep_tree_total_tokens{outcome="pass"}` 카운터
  += 1.
- OTel trace tree depth = 3, 모든 자식 span이 부모 span의
  `parent_span_id`를 가진다.
- `.moai/runs/test-run-001/tree.json` 파일이 generate되며 모든 노드가
  `Status=NodeStatusComplete`.
- Postgres `deep_runs` 테이블에 단일 row insert: `{run_id="test-run-001",
  query="양자컴퓨터의 신약 개발 응용 현황", breadth=4, depth=3,
  total_nodes=85, total_tokens=<actual>, total_cost_usd=<derived>,
  status="success", ...}`.
- 모든 완료 노드는 `Node.CostUSD` 필드를 가지며,
  `Node.CostUSD == Node.TokensUsed * pricing.{DEEP_TREE_DECOMPOSE_MODEL}`
  관계가 성립한다(REQ-DEEP3-013). 트리 수준 `TotalCostUSD`는
  `sum(node.CostUSD for node where Status == NodeStatusComplete)`로
  derive되며 Postgres `total_cost_usd` 컬럼과 일치한다.

**Test functions**: `TestExpandTreeHappyPath`, `TestExpandTreeLatencyP95`,
`TestExpandTreeMetricsObserved`, `TestNodeCostUSDComputedOnComplete`,
`TestTreeTotalCostUSDSumsCompletedNodes`
(`internal/deepagent/tree_test.go`, `tree_metrics_test.go`).

---

## Scenario 5.2 — 구조적 cap 초과로 expand 거부

**REQ Coverage**: REQ-DEEP3-002

### Given

- 사용자가 `/deep?mode=agents`에 다음 요청을 전송:
  ```json
  {
    "request_id": "test-run-002",
    "query": "test query",
    "tree": {"breadth": 9, "depth": 3}
  }
  ```
- `breadth=9`는 허용 범위 `[1, 8]`을 초과.

### When

- 트리 익스플로러가 input validation을 수행한다.

### Then

- 응답 HTTP 400.
- 응답 body:
  ```json
  {
    "error": "invalid_tree_config",
    "detail": "breadth=9 exceeds maximum 8",
    "breadth": 9,
    "depth": 3
  }
  ```
- 트리 expand는 시작되지 않는다 — `.moai/runs/test-run-002/tree.json`
  파일이 생성되지 않는다.
- Postgres `deep_runs` 테이블에 row가 insert되지 않는다.
- Prometheus histogram observation이 기록되지 않는다(0회).
- 동일하게, `depth=6` 요청에 대해서도 HTTP 400 반환(range overflow).
  `breadth=0` 또는 `depth=0`은 REQ-DEEP3-005 fallback signal로 별도
  처리되므로 본 시나리오의 invalid-range 검증 대상이 아니다(§5.6 참조).

**Test functions**: `TestExpandTreeInvalidBreadth`,
`TestExpandTreeInvalidDepth` (`internal/deepagent/tree_test.go`).

---

## Scenario 5.3 — 토큰 budget 소진 mid-tree → 부분 트리 반환 + reservation lock race-free

**REQ Coverage**: REQ-DEEP3-006 (reservation lock), 007 (atomic release),
008, 010; NFR-DEEP3-003 (race-free budget enforcement)

### Given

- 사용자가 `/deep?mode=agents`에 다음 요청을 전송:
  ```json
  {
    "request_id": "test-run-003",
    "query": "복잡한 multi-hop query",
    "tree": {"breadth": 4, "depth": 3}
  }
  ```
- `DEEP_TREE_TOKEN_BUDGET=20000` (default 60K보다 낮음).
- 각 노드 평균 2000 tokens 소비(85 노드 × 2000 = 170K, budget 8.5x
  초과).
- 모든 sibling 노드는 동일 depth에서 `errgroup.WithContext`로 동시
  dispatch(REQ-DEEP3-004).

### When

- 트리 익스플로러가 BFS expand를 진행. REQ-DEEP3-006의 reservation lock
  semantics에 따라 매 노드 dispatch 직전 pre-check가 (read +
  decision + reservation)을 atomic하게 수행하며, REQ-DEEP3-007의 atomic
  release semantics가 노드 완료 시점에 reservation/actual delta를 동시에
  반영한다.
- Budget exhaustion이 발생하면 잔여 frontier 노드를
  `NodeStatusBudgetExceeded`로 mark.

### Then (deterministic bounds)

- 응답 HTTP 200 (degraded success, 5xx error 아님).
- 응답 body의 `tree.status == "budget_exceeded"`.
- **Hard invariant (NFR-DEEP3-003)**: `final_total_tokens =
  sum(node.TokensUsed for completed nodes) ≤ DEEP_TREE_TOKEN_BUDGET =
  20000`. 본 invariant는 sibling 동시 race 조건 하에서도 SHALL 보존된다
  (reservation lock이 race window 자체를 제거).
- **Completed nodes bound**: `total_nodes_completed ≤
  floor(DEEP_TREE_TOKEN_BUDGET / average_node_tokens) = floor(20000 /
  2000) = 10`. Reservation overhead로 인해 실제 값은 더 작을 수 있다
  (lower bound는 1 — 최소 root는 항상 complete; root estimate 5000 +
  budget 20000 충분).
- **Skipped nodes**: `total_nodes_skipped = 85 -
  total_nodes_completed`.
- Writer가 `TreeResult.flattened_claims`(완료된 노드들의 claims)에서
  답변 생성 — empty corpus가 아니므로 정상 답변(root은 항상 complete).
- Prometheus `usearch_deep_tree_total_tokens{outcome="budget_exceeded"}`
  += 1.
- `tree.json`에 모든 85 노드가 기록 — 일부는 `NodeStatusComplete`,
  나머지는 `NodeStatusBudgetExceeded`.

### Root node seed sub-scenario

추가로 `DEEP_TREE_TOKEN_BUDGET=4000`, `DEEP_TREE_ROOT_TOKEN_ESTIMATE=
5000` 설정 하에서 첫 root expand 직전 pre-check가 즉시 fail해야 한다
(`estimated_next_cost = 5000 > budget 4000`). 응답은 빈 트리 + `usage:
{budget_exceeded: true, total_nodes_completed: 0, ...}`. 본 sub-scenario
는 REQ-DEEP3-006의 root seed clause를 검증한다.

### Concurrent breadth race sub-scenario (B1 verification)

추가로 `breadth=8, depth=2, DEEP_TREE_TOKEN_BUDGET=60000` 설정 하에서
worst-case 동시 race를 검증한다:

- root(actual=2000, reserved=5000 → release 3000 후 reserved=0,
  cumulative=2000) 완료 후 8개 depth-1 sibling이 errgroup.WithContext로
  동시 dispatch.
- 각 sibling의 `estimated_next_cost = parent.TokensUsed * breadth *
  1.25 = 2000 * 8 * 1.25 = 20000`.
- Pre-check 순서(reservation lock으로 serialize): cumulative=2000,
  reserved=0. Sibling 1 통과 → reserved=20000. Sibling 2 시점 cumulative
  + reserved + 20000 = 42000 < 60000 → reserved=40000. Sibling 3 시점
  42000+20000=62000 > 60000 → BudgetExceeded. Sibling 4~8 동일.
- 결과: depth-1에서 최대 2 sibling이 complete 가능(actual 사용량은 약
  4000 tokens 추가). 잔여 6 sibling은 BudgetExceeded.
- **Critical assertion**: `final_total = sum(node.TokensUsed for
  completed) ≤ 60000` 성립(reservation lock으로 overshoot 0).
- Test: `TestExpandTreeConcurrentBreadthBudgetRaceFree`는 본 시나리오를
  100회 반복 실행하여 매 iteration `final_total ≤ budget_cap`을 assertion
  한다.

**Test functions**: `TestExpandTreeBudgetExceeded`,
`TestExpandTreePartialReturn`,
`TestExpandTreeRootSeedTriggersImmediateBudgetFail`,
`TestExpandTreeConcurrentBreadthBudgetRaceFree`
(`internal/deepagent/tree_test.go`),
`TestBudgetReservationLockSerializesSiblings`,
`TestBudgetReservationReleaseOnComplete`
(`internal/deepagent/budget_test.go`).

---

## Scenario 5.4 — 인용 lineage가 모든 leaf claim에서 root까지 추적 가능

**REQ Coverage**: REQ-DEEP3-009a (prompt context flow), REQ-DEEP3-009b
(citation slice disjointness), REQ-DEEP3-010

### Given

- Scenario 5.1과 동일 조건으로 정상 트리 확장 완료.
- 85개 노드 모두 `NodeStatusComplete`.
- 각 노드가 평균 3개의 `NodeClaim` 보유 → 총 ~255 claims.

### When

- 트리 익스플로러가 `flattenWithLineage(tree) -> TreeResult` 변환을
  수행한다.
- Writer가 `TreeResult.flattened_claims`를 inspection.

### Then

- 모든 `FlattenedClaim.lineage_path`에 대해:
  - (a) `lineage_path[0]`이 root query 문자열을 prefix로 포함
    (예: `"root: 양자컴퓨터의 신약 개발 응용 현황"`).
  - (b) source_node가 depth=k인 경우 `len(lineage_path) == k+1`.
  - (c) `source_node_id`로 시작하여 부모 ParentID를 따라 traverse하면
    root node(`ParentID == ""`)에 도달 가능.
- Hypothesis-go property test가 100개 random tree generation(varying
  breadth/depth)에 대해 invariant를 검증:
  - 모든 leaf claim의 `lineage_path[0]`이 root query.
  - lineage_path 길이가 source node의 depth와 일치.
- 어떤 `FlattenedClaim`도 `lineage_path == []` (empty)인 경우가 없다.
- **REQ-DEEP3-009b citation disjointness invariant**: 각 노드의
  `Node.Citations` 슬라이스는 다른 노드의 Citations 슬라이스를 reference
  로 inherit하지 않는다(독립 `fanout.Dispatch` 결과만 보유). 두 노드의
  Citations에 동일 doc_id가 우연히 등장하는 것은 허용되나, 슬라이스
  identity는 disjoint해야 한다. Test: 모든 노드 쌍 (a, b)에 대해
  `&a.Citations[0]`이 b.Citations 슬라이스에 속하지 않음을 검증.

**Test functions**: `TestFlattenedClaimLineageInvariant`,
`TestFlattenedClaimLineageProperty`,
`TestCitationsDisjointlyOwned` (`internal/deepagent/tree_test.go`,
`tree_types_test.go`).

---

## Scenario 5.5 — Sidecar 크래시 시 tree.json 부분 복원

**REQ Coverage**: REQ-DEEP3-011a (atomic flush per depth-level join),
REQ-DEEP3-011b (reload-mode reclassify); NFR-DEEP3-008

### Given

- 사용자가 `/deep?mode=agents`에 정상 요청 전송 (Scenario 5.1과 동일
  body).
- 트리 expand가 depth=2 진행 중 (약 20개 노드 `NodeStatusComplete`, 5개
  `NodeStatusExpanding`).
- 운영자가 server에 SIGTERM 전송.

### When (REQ-DEEP3-011a — expansion-phase atomic flush)

- Server가 graceful shutdown sequence를 수행한다.
- 진행 중이던 expand는 context cancellation으로 중단.
- 매 depth-level join 직후 + 노드 transition 시점에 atomic flush가
  수행되었으므로(REQ-DEEP3-011a), 디스크의 `.moai/runs/<run_id>/tree.json`
  은 완료된 노드들의 상태를 partial하게 보유한다.

### Then (REQ-DEEP3-011a — flush invariant)

- `tree.json` 파일은 valid JSON 구조 — flush된 디스크 원본은 불변.
- depth=1 노드들은 모두 `NodeStatusComplete`로 flush된 상태(SIGTERM
  이전에 depth-1 join 완료).
- depth=2의 5개 `NodeStatusExpanding` 노드는 SIGTERM 시점의 상태로
  disk에 부분 flush된 상태이거나, 가장 가까운 atomic flush 이전 상태로
  보존된다 — 어느 경우든 valid JSON parse가 가능하다.

### When + Then (REQ-DEEP3-011b — reload-mode reclassify)

- 이후 audit script(SPEC-DEEP-004 audit 기능)가 persistence layer를
  reload mode로 invoke한다.
- 로드 직후 reload 로직(REQ-DEEP3-011b)이 in-memory 변환을 수행:
  - `Status == NodeStatusComplete` 노드는 그대로 유지.
  - `Status ∈ {NodeStatusExpanding, NodeStatusPending}` 노드를
    `NodeStatusFailed`로 reclassify.
  - reload된 트리는 read-only로 반환 — 추가 expand 시도는 차단되고
    expand attempt는 panic 또는 error로 reject.
- 디스크 tree.json은 reclassify 결과로 overwrite되지 않는다 — audit
  무결성 보장(in-memory 변환에 한정).
- Postgres `deep_runs` row의 `status` 필드가 `"failed"` 또는
  `"partial"`로 finalize. `completed_at` 필드가 SIGTERM 시점 ± 5초.
- Resume 시도 불가 — 새 `/deep` 요청은 fresh run_id로 처음부터 expand
  (resume은 §4 Exclusions).

**Test functions**:
- REQ-011a: `TestPersistenceAtomicFlushOnDepthJoin`,
  `TestPersistenceCrashFinalizesPostgresRow`
- REQ-011b: `TestPersistenceReclassifyOnReload`,
  `TestPersistenceReloadTreeRejectsExpandAttempt`
  (`internal/deepagent/persistence_test.go`).

---

## Edge Cases

### Scenario 5.6 (edge) — breadth=0 OR depth=0 fallback to single-shot (header-based signal)

**REQ Coverage**: REQ-DEEP3-005 (단일 정책: `breadth=0` OR `depth=0` →
single-shot fallback, HTTP 200, fallback signal은 HTTP 응답 header
`X-Deep-Tree-Fallback`을 통해 emit; response body는 DEEP-002 single-shot
contract와 byte-identical 유지)

#### Sub-scenario 5.6a — breadth=0

##### Given

- 사용자가 `/deep?mode=agents`에 `{tree: {breadth: 0, depth: 3}}` 전송.

##### When

- 트리 익스플로러가 input validation을 수행한다.

##### Then

- 응답 HTTP 200(`breadth=0`은 invalid가 아닌 fallback signal).
- DEEP-002 REQ-005의 single-shot Researcher 동작이 수행됨 — 단일
  `fanout.Dispatch` 호출 후 결과를 Writer로 전달.
- **응답 header**: `X-Deep-Tree-Fallback: breadth_zero` SHALL emit.
- **응답 body**: DEEP-002 single-shot contract와 byte-identical
  (본 SPEC은 body 구조를 mutate하지 않는다). 즉 SSE/JSON body에
  `tree.disabled`/`tree.mode`/`tree.reason` 같은 신규 필드가 존재 SHALL
  NOT 한다.
- `tree.json` 파일이 생성되지 않는다(트리 모드가 아닌 single-shot).

#### Sub-scenario 5.6b — depth=0

##### Given

- 사용자가 `/deep?mode=agents`에 `{tree: {breadth: 4, depth: 0}}` 전송.

##### When

- 트리 익스플로러가 input validation을 수행한다.

##### Then

- 응답 HTTP 200(`depth=0`도 invalid가 아닌 fallback signal).
- DEEP-002 REQ-005의 single-shot Researcher 동작이 수행됨.
- **응답 header**: `X-Deep-Tree-Fallback: depth_zero` SHALL emit.
- **응답 body**: DEEP-002 single-shot contract와 byte-identical.
- `tree.json` 파일이 생성되지 않는다.

#### Sub-scenario 5.6c — breadth=0 AND depth=0 동시 지정

##### Given

- 사용자가 `/deep?mode=agents`에 `{tree: {breadth: 0, depth: 0}}` 전송.

##### When

- 트리 익스플로러가 input validation을 수행한다.

##### Then

- 응답 HTTP 200, single-shot fallback 수행.
- **응답 header**: `X-Deep-Tree-Fallback: breadth_zero` 우선 emit
  (REQ-DEEP3-005 본문 명시).
- **응답 body**: DEEP-002 single-shot contract와 byte-identical.

#### Sub-scenario 5.6d — header + body invariant regression (B2)

##### Given

- DEEP-002 acceptance test suite의 single-shot 응답 fixture가 hash로
  capture되어 있음(byte-level reference).

##### When

- 본 SPEC의 fallback path가 트리거된 응답을 hash 비교한다(header 제외,
  body만 비교).

##### Then

- Body hash가 DEEP-002 reference fixture와 일치 SHALL — 본 SPEC이
  body를 mutate하지 않음을 회귀 검증.
- `X-Deep-Tree-Fallback` header는 별도로 존재 SHALL.

**Test functions**: `TestExpandTreeBreadthZeroFallback`,
`TestExpandTreeDepthZeroFallback`,
`TestExpandTreeBreadthAndDepthZeroFallback`
(`internal/deepagent/tree_test.go`),
`TestFallbackHeaderEmittedAndBodyUnchanged`
(`tests/integration/deep_tree_test.go`).

### Scenario 5.8 (NFR gate) — in-memory tree memory bound (NFR-DEEP3-009)

**REQ Coverage**: NFR-DEEP3-009

#### Given

- 트리 익스플로러가 worst-case config(`breadth=8, depth=5`)로 expand
  실행 중. 각 노드의 평균 in-memory footprint(Node header + Citations
  + Claims)는 ~4 KB로 측정된 baseline.

#### When

- 매 depth-level join 직후(REQ-DEEP3-011a flush 직전), persistence
  layer가 트리 in-memory state 합산 크기를 측정한다.

#### Then

- 모든 depth-level 측정 시점에서 `sizeof(Node[]) + sizeof(Citations) +
  sizeof(Claims) ≤ 100 MB` SHALL.
- 만약 100 MB 초과가 감지되면, 트리 익스플로러는 frontier 노드를
  truncation(`NodeStatusBudgetExceeded`로 mark)하여 다음 depth-level
  전 100 MB 이하로 복원 SHALL 한다.
- `usearch_deep_tree_total_tokens{outcome="budget_exceeded"}` += 1
  emitted (memory bound도 budget_exceeded outcome으로 분류 — 별도
  outcome label 신설하지 않음).

**Test functions**: `TestTreeMemoryFootprintUnder100MBWorstCase`
(`internal/deepagent/tree_test.go`).

---

### Scenario 5.7 (edge) — depth=1 single-level tree

**REQ Coverage**: REQ-DEEP3-003, 009

#### Given

- 사용자가 `/deep?mode=agents`에 `{tree: {breadth: 4, depth: 1}}` 전송.

#### When

- 트리 익스플로러가 root + 4개 leaf만 expand한다.
- depth=1 노드는 sub-query 생성(d-e 단계)을 skip하고 fanout만 수행
  (leaf node 동작 per REQ-003 (g)).

#### Then

- 응답 HTTP 200, `total_nodes == 5` (1 + 4).
- `max_depth_reached == 1`.
- 모든 leaf node(depth=1)의 `Citations` 비어있지 않음 — fanout 결과
  포함.
- 모든 `FlattenedClaim.lineage_path` 길이가 1 또는 2 (root or
  root+depth1).
- Prometheus histogram `depth` label 값은 `{0, 1}`만 emit.

**Test functions**: `TestExpandTreeDepthOneSingleLevel`
(`internal/deepagent/tree_test.go`).

---

## Definition of Done

본 SPEC의 구현이 "complete" 상태로 전이하려면 다음 모든 항목을 만족해야
한다:

- [ ] 모든 acceptance scenario(5.1 ~ 5.8) 테스트 함수가 작성 및 PASS.
- [ ] 모든 REQ-DEEP3-001..013(+009a/009b/011a/011b split 포함)이 적어도
  한 테스트 함수로 cover됨(REQ-to-test traceability matrix).
- [ ] 모든 NFR-DEEP3-001..009가 quantitative assertion(latency
  percentile, cardinality bound, size bound, race-free budget invariant,
  in-memory footprint 등)으로 검증됨.
- [ ] Coverage report ≥ 85% (per quality.yaml coverage_target).
- [ ] `go test -race ./internal/deepagent/...` 통과(race condition
  없음).
- [ ] `goleak.VerifyNone(t)` 통과(goroutine leak 없음, 특히 cancellation
  테스트에서).
- [ ] DEEP-002 acceptance suite 100% green(regression check) —
  `/deep?mode=agents` single-shot 동작이 본 SPEC 도입 후에도 byte-
  identical.
- [ ] Prometheus cardinality guard(`internal/obs/metrics/metrics_test.go::
  TestNoUnboundedLabels`) 통과 — 본 SPEC이 도입하는 모든 label values가
  bounded.
- [ ] tree.json sample 파일 inspection — gzip 압축 후 ≤ 200 KB
  (NFR-DEEP3-007).
- [ ] Postgres `deep_runs` migration up/down clean(rollback 가능).
- [ ] `.moai/specs/SPEC-DEEP-003/progress.md`에 모든 phase 진행 상황
  기록됨.

---

*End of acceptance.md.*
