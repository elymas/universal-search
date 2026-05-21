# SPEC-DEEP-003 Deep Research

Generated: 2026-05-21T00:00:00Z
Author: manager-spec (Phase 0.5 deep research, synthesised from internal SPEC corpus + upstream gpt-researcher conventions)
Consumed by: manager-spec (Phase 1B SPEC authoring), plan-auditor (Phase 2.3 audit)

---

## 1. Tree Exploration Architecture

본 SPEC은 `/deep?mode=agents` 파이프라인(SPEC-DEEP-002)의 Researcher 노드가
호출하는 "deep-research tree" 모드를 도입한다. 단일-shot fanout이 사용자
쿼리에 대한 단일 doc set을 반환하는 반면, deep-research tree는 사용자
쿼리를 root node로 두고 LLM이 sub-query를 breadth-first로 확장한 뒤
각 sub-query에 대해 다시 fanout을 실행하여 multi-level evidence tree를
생성한다.

### 1.1 Node 데이터 구조 (Go-side)

```
type NodeStatus string

const (
    NodeStatusPending        NodeStatus = "pending"
    NodeStatusExpanding      NodeStatus = "expanding"
    NodeStatusComplete       NodeStatus = "complete"
    NodeStatusBudgetExceeded NodeStatus = "budget_exceeded"
    NodeStatusFailed         NodeStatus = "failed"
)

type Node struct {
    ID            string             // ULID-like, deterministic per (run_id, parent_id, breadth_index)
    RunID         string             // SPEC-DEEP-002 request_id로부터 파생
    ParentID      string             // root node의 ParentID == ""
    Depth         int                // root = 0
    BreadthIndex  int                // 0..(breadth-1)
    Query         string             // sub-query (LLM-generated for non-root, user-provided for root)
    Status        NodeStatus
    Citations     []NodeCitation     // 본 node에서 수집된 doc 인용 (doc_id → fanout 결과 매핑)
    Claims        []NodeClaim        // 본 node에서 추출된 검증 가능한 claim
    ChildIDs      []string           // 자식 노드 ID 리스트 (BFS 순서)
    TokensUsed    int                // 본 node가 사용한 LLM 토큰 (prompt + completion)
    CostUSD       float64            // 본 node 누적 cost
    ExpandedAt    time.Time          // expand 시작 시점 (zero = 미시작)
    CompletedAt   time.Time          // expand 종료 시점 (zero = 진행 중 또는 미시작)
    FailureReason string             // Status == failed 시 사유
}

type NodeCitation struct {
    DocID  string // pkg/types.NormalizedDoc.ID 참조
    Marker int    // 트리 전체에서 1-indexed (lineage 추적용)
    Source string // adapter source_id (예: "hn", "reddit")
}

type NodeClaim struct {
    Sentence  string
    Markers   []int  // NodeCitation.Marker 참조
}
```

### 1.2 Go vs Python 경계

DEEP-002의 §1.1 결정사항 D1(Orchestration host = Go module)을 계승한다.
본 SPEC도 동일 원칙을 따른다:

- **Go-side (`internal/deepagent/`)**: tree orchestration, BFS expansion
  loop, budget enforcement, persistence, observability instrumentation,
  Writer로 전달할 평탄화(lineage 보존) 자료 구조 생성.
- **Python-side (`services/researcher/`)**: per-node sub-query
  generation을 위한 LLM 호출 wrapper. gpt-researcher upstream의
  query-decomposition convention을 thin layer로 노출한다. tree
  구조 자체는 Python sidecar에서 관리하지 않는다(상태는 Go-side에서
  중앙집중 관리).

Reason: tree orchestration을 Python으로 옮기면 (a) 상태가 두 곳에
분산되어 cancellation/persistence가 복잡해지고, (b) Go의 goroutine pool
패턴(errgroup + bounded concurrency)이 Python asyncio보다 본 use case에
서 더 명확하며, (c) DEEP-002의 `internal/deepagent/orchestrator.go`와
인접 모듈로 두는 편이 코드 재사용에 유리하다.

### 1.3 Persistence

- **Sidecar JSON**: 매 노드 expand 완료 시점에 `.moai/runs/{run_id}/tree.json`
  으로 atomic flush. 부분 트리 복원을 지원하기 위함.
- **Postgres `deep_runs` 요약 row**: `{run_id, query, breadth, depth,
  total_nodes, total_tokens, total_cost_usd, status, started_at,
  completed_at}` — 운영자가 트리 사용량을 집계할 수 있도록.
- Schema migration은 SPEC-IDX-001(M2 완료)의 migration framework를
  사용. 본 SPEC은 `migrations/0NN_deep_runs.up.sql`만 추가.

---

## 2. gpt-researcher Deep-Research-Tree Pattern Study

### 2.1 Upstream Convention

gpt-researcher(stanford-oval/storm과는 별개 프로젝트, assafelovic/gpt-researcher)
는 "deep research" 모드에서 query decomposition을 통한 multi-level
evidence collection을 제공한다. 본 SPEC은 upstream의 다음 컨벤션을
참조한다:

- **Default breadth = 4**: root query를 4개의 sub-query로 확장.
- **Default depth = 3**: depth=0(root) → depth=1(sub-query) →
  depth=2(sub-sub-query). 최대 트리 노드 수 = `breadth^depth` (4^3=64
  potential nodes, 실제 cap은 breadth×depth 곱이 아닌 structural cap이
  관리).
- **Sub-query generation prompt**: LLM에 root query + 현재 노드의
  query + 부모의 evidence summary를 주고, 다양한 관점의 N개 sub-query를
  요청한다.
- **Per-node budget**: gpt-researcher는 사용자 정의 cost/token cap을
  per-run으로 enforce.

### 2.2 현재 코드베이스의 reference points

`services/researcher/src/researcher/` 디렉토리는 현재 단일-shot synthesis
sidecar(`synthesis.py`)만 구현되어 있으며, gpt-researcher 통합 자체는
아직 미수행이다. 본 SPEC은 다음 두 옵션 중 하나를 채택해야 한다:

- **Option A (권장)**: gpt-researcher의 query-decomposition logic을
  본 SPEC에서 신규 도입 — `services/researcher/src/researcher/deep_tree.py`
  module을 추가하고, LLM prompt template만 upstream에서 영감을 받아
  in-house 구현. gpt-researcher 패키지를 dependency로 추가하지 않는다.
- **Option B**: gpt-researcher PyPI 패키지를 `services/researcher/pyproject.toml`
  에 추가하고 그 query-decomposition API를 직접 호출.

Option A를 채택한다. 근거: (1) gpt-researcher는 자체 LLM/검색 통합을
강제하는데 본 프로젝트는 LiteLLM proxy(SPEC-LLM-001)와 fanout
gateway(SPEC-FAN-001)를 이미 보유하고 있어 충돌이 발생한다. (2)
DEEP-001의 STORM 통합 시 upstream 라이브러리 의존성으로 인해 LM 라우팅을
별도로 wiring해야 했던 비용을 회피한다. (3) decomposition prompt만
in-house로 가지면 prompt evolution이 single source of truth로 유지된다.

### 2.3 Researcher 사이드카 신규 endpoint

- `POST /decompose_query` — 본 SPEC이 새로 정의.
  - Input: `{run_id, node_id, root_query, parent_query, parent_evidence_summary,
            breadth, lang}`
  - Output: `{sub_queries: [{query: string, rationale: string}], tokens_used,
              cost_usd}`
- 본 endpoint는 LiteLLM proxy를 통해 LLM 호출하며, model alias는 본
  SPEC이 정의하는 env-var `DEEP_TREE_DECOMPOSE_MODEL`(default
  `claude-3-5-haiku-20241022`)로 라우팅.
- 기존 `/synthesize` 와 `/faithfulness_check`(DEEP-002 추가) endpoint는
  무영향.

---

## 3. Budget Model

### 3.1 세 가지 cap dimension

본 SPEC은 single dimension cap만 사용하는 대신 동시 다중 cap을
도입한다(RDC-3 결정 사항):

1. **Total token budget** (`DEEP_TREE_TOKEN_BUDGET`, default 60000):
   tree 전체에서 소비할 수 있는 prompt + completion 토큰의 상한.
   per-node `Node.TokensUsed` 합산이 이 값을 초과하면 expand 중단.
2. **Per-node timeout** (`DEEP_TREE_NODE_TIMEOUT_MS`, default 30000):
   단일 노드의 expand가 이 시간 이상 소요되면 해당 노드를
   `NodeStatusFailed`로 표시하고 다음 노드로 진행.
3. **Structural cap** (breadth × depth): 정적 상한.
   `breadth=4, depth=3` → 최대 노드 수 ≤ 1 + 4 + 16 + 64 = 85
   (root 1 + 4 children + 16 grandchildren + 64 great-grandchildren).
   하지만 본 SPEC은 conservative하게 default `breadth=4, depth=3`을 채택
   하여 운영 cap을 80개 노드 미만으로 유지한다.

### 3.2 Enforcement points

- **Pre-expansion check**: 각 노드 expand 직전, 다음 조건들을 평가:
  (a) `current_total_tokens + estimated_node_cost > token_budget`,
  (b) `len(visited_nodes) ≥ structural_cap`. 조건 (a)는 추정 비용이라
  완벽하지 않으므로 conservative하게 estimated cost = (parent's tokens *
  breadth) + headroom 25%로 산정.
- **Post-expansion measure**: 노드 expand 완료 후 실제 `TokensUsed`
  반영. 다음 노드 시작 시점에 (a) 조건이 다시 평가됨.
- **Per-node timeout**: `context.WithTimeout(ctx, DEEP_TREE_NODE_TIMEOUT_MS)`
  으로 격리.

### 3.3 DEEP-004와의 상호작용

본 SPEC은 **per-run budget**만 enforce한다. DEEP-004는 본 SPEC이 노출하는
`Node.TokensUsed`, `Node.CostUSD` 메트릭을 소비하여 per-user/per-day cap을
계산한다. 본 SPEC은 다음 hooks를 노출한다:

- Prometheus counter `usearch_deep_tree_tokens_total{run_id_bucket}` —
  run_id를 hashing하여 bucket으로 변환(cardinality bound).
- HTTP response body의 `usage` 필드에 per-run `{total_tokens, total_cost_usd,
  total_nodes, max_depth_reached}`를 포함.

---

## 4. Citation Lineage Across Tree

### 4.1 Lineage 추적 구조

모든 leaf claim은 root까지 traceable해야 한다. 본 SPEC은 다음 invariant
를 유지한다:

- 매 `Node.Claims[i].Markers[j]`는 해당 노드의 `Node.Citations`를 1-indexed
  참조한다(노드-로컬 marker).
- 전역 lineage 추적은 tree traversal로 reconstruct: leaf claim의 marker →
  leaf node의 doc_id → leaf node의 query → parent node의 query → ... →
  root query.
- Writer(DEEP-002 REQ-006) 단계에서 tree를 flatten할 때, 각 claim에
  `LineagePath []string` 필드를 부여(예: `["root: 양자컴퓨터 응용",
  "depth1: 양자컴퓨터 의료 응용", "depth2: 양자컴퓨터 단백질 폴딩"]`).

### 4.2 Writer가 트리를 소비하는 방법

본 SPEC은 Writer 자체를 정의하지 않지만(DEEP-002 REQ-006 소관), Writer가
소비할 자료 구조 contract를 정의한다:

- `TreeResult` struct: `{root_query, total_nodes, max_depth_reached,
  status, flattened_claims: []FlattenedClaim, citations: []Citation}`
- `FlattenedClaim`: `{text, markers, lineage_path, source_node_id}`
- Writer는 `flattened_claims`만 prompt에 주입하며, tree 자체를 prompt
  context로 들이지 않는다(token efficiency).
- 결정: Writer는 hierarchical(tree-shaped) prompt를 받지 않고
  평탄화(flatten with lineage)를 받는다. 근거: (a) tree-shaped prompt는
  LLM context 효율이 떨어진다, (b) lineage_path가 인용 출처를 충분히
  설명한다, (c) DEEP-002의 Writer는 이미 "evidence + critique" 평면
  context를 받는 패턴이라 트리 구조 도입은 변경 비용이 크다.

---

## 5. Reference Implementations

### 5.1 Internal references

| New File | Closest Internal Analog | Pattern Borrowed |
|----------|-------------------------|------------------|
| `internal/deepagent/tree.go` | `internal/deepagent/orchestrator.go` (DEEP-002, planned) | Sequential orchestration with cancellation checks |
| `internal/deepagent/budget.go` | `internal/llm/router.go` (LLM-001 circuit breaker pattern) | State machine with thresholds |
| `internal/deepagent/persistence.go` | `internal/deepreport/types.go` (DEEP-001) | JSON marshaling with schema version |
| `services/researcher/src/researcher/deep_tree.py` | `services/researcher/src/researcher/synthesis.py` | FastAPI route + LiteLLM gateway pattern |
| `internal/fanout/dispatch.go` | (no new file) | Researcher가 각 노드에서 호출하는 read-only API |

### 5.2 External references (verify URLs via WebFetch in Run phase)

- `https://github.com/assafelovic/gpt-researcher` — upstream deep-research
  tree pattern (decomposition prompt 영감)
- `https://github.com/assafelovic/gpt-researcher/blob/master/docs/docs/gpt-researcher/multi_agents/langgraph.md`
  — multi-agent + tree expansion 통합 예시
- (Note: 본 SPEC은 gpt-researcher 패키지를 dependency로 추가하지 않음 —
  Option A 결정 사항. URL은 prompt design 참조용)

### 5.3 SPEC dependencies (re-used contracts)

- SPEC-DEEP-001(implemented): STORM sidecar 패턴 (Python sidecar +
  FastAPI + LiteLLM gateway). 본 SPEC의 `deep_tree.py`가 동일 패턴 차용.
- SPEC-DEEP-002(draft v0.1.1): `internal/deepagent/` 모듈 레이아웃,
  Researcher 에이전트 hook point, SSE 이벤트 패턴.
- SPEC-SYN-001/SYN-004: synthesis sidecar의 기존 FastAPI 패턴.
- SPEC-LLM-001: 모든 LLM 호출이 `llm.Client.Complete()`를 경유.
- SPEC-OBS-001: Prometheus cardinality safety (`NFR-OBS-002`).
- SPEC-FAN-001: 각 노드에서 `fanout.Dispatch()`로 검색.
- SPEC-CORE-001: `NormalizedDoc` shape (각 노드의 citation source).

---

## 6. Pinned Decisions (Do Not Re-debate)

본 절은 본 SPEC 작성 시점에 manager-spec이 확정한 7개 architectural
decision을 기록한다. 본 SPEC 본문은 이 결정사항을 EARS 요구사항으로
번역할 뿐 재논의하지 않는다.

### D1. Tree orchestration language

**결정**: Go module `internal/deepagent/tree.go` 신규 추가. Python sidecar
는 per-node sub-query generation을 위한 thin LLM wrapper만 제공.

**대안 검토**:

- (B) Python sidecar에 tree orchestration 전체 위임: Reject. Go-side에서
  cancellation/state를 관리하는 DEEP-002 패턴과 일관성 깨짐.
- (C) 별도 신규 service: Reject. 운영 부담 증가, 본 SPEC의 inherent
  복잡도를 정당화하지 못함.

**근거**: DEEP-002 §1.1 D1과 동일 원칙. 트리 상태(Node[], BFS queue,
budget counters)는 Go-side에서 single source of truth.

### D2. Default (breadth, depth)

**결정**: `breadth=4, depth=3`. config override 가능 (`.moai/config/sections/deep.yaml`
신규 + per-request body field).

**대안 검토**:

- (B) breadth=3, depth=4: Reject. 트리 노드 수가 1+3+9+27+81=121로
  증가하여 token budget 위반 위험 큼.
- (C) breadth=5, depth=2: Reject. depth=2는 sub-sub-query 탐색을 막아
  multi-hop reasoning을 차단.

**근거**: gpt-researcher upstream convention과 일치. 60K token budget
하에서 default cap 미만 노드 수 유지 가능.

### D3. Budget enforcement

**결정**: 세 가지 cap을 simultaneously enforce — (a) total token budget,
(b) per-node timeout, (c) structural breadth×depth cap.

**대안 검토**:

- (B) token budget만 사용: Reject. 단일 노드 hang으로 전체 트리 진행
  불가 위험.
- (C) timeout만 사용: Reject. 토큰 cost overrun 위험.

**근거**: 세 cap은 서로 다른 failure mode(cost / latency / structural)
를 커버한다. 비용 부담은 minimal — 각 cap 평가는 O(1).

### D4. Tree persistence

**결정**: JSON sidecar(`.moai/runs/{run_id}/tree.json`) + Postgres
summary row(`deep_runs` 테이블).

**대안 검토**:

- (B) Postgres에 트리 전체 저장: Reject. JSONB 컬럼이 5MB 이상 grow할
  가능성, Postgres index 부담.
- (C) Redis ephemeral: Reject. 트리는 audit 목적상 영구 보관 필요(M8
  EVAL SPEC이 본 트리를 재평가).

**근거**: JSON 파일은 cheap, Postgres summary는 운영 대시보드 용이.

### D5. Sub-query generation

**결정**: 별도 LLM role을 신설하지 않고, DEEP-002 Researcher 에이전트(REQ-002)
를 per-node 재사용. Researcher는 per-call에 한 노드의 sub-query 생성을
담당하며, 트리 orchestration은 본 SPEC의 `tree.go`가 관리.

**대안 검토**:

- (B) 신규 "Decomposer" 에이전트 추가: Reject. 에이전트 수 증가는
  DEEP-002의 sequential 4-agent 패턴을 깨고, prompt template duplication
  발생.
- (C) tree.go가 직접 LLM 호출: Reject. DEEP-002의 agent 추상화 layer를
  bypass하면 SSE 이벤트 라우팅이 복잡해짐.

**근거**: Researcher 에이전트는 이미 "evidence collection" 책임을 가지며
sub-query 생성은 evidence collection의 sub-step.

### D6. Budget exhaustion 시 동작

**결정**: 잔여 frontier 노드를 `NodeStatusBudgetExceeded`로 표시하고
부분 트리를 Writer에 반환. 전체 abort하지 않는다.

**대안 검토**:

- (B) HTTP 503 abort: Reject. 사용자가 받을 부분 결과의 가치를 폐기.
- (C) Best-effort continue with degraded budget: Reject. Budget invariant
  violation, DEEP-004 cost guard contract 깨짐.

**근거**: 사용자 경험 우선. 부분 트리도 Writer가 유의미한 답을 생성할
수 있음(DEEP-002 REQ-012의 empty corpus short-circuit과 대조 — empty
는 0 노드, budget exhausted는 N>0 노드).

### D7. Concurrent expansion

**결정**: 각 depth level에서 `breadth`개의 노드를 goroutine pool로
parallel expand. `errgroup.WithContext`로 격리.

**대안 검토**:

- (B) 완전 sequential: Reject. depth=3, breadth=4 시 latency가 노드 수
  비례하게 증가. p95 ≤ 4분 목표 달성 불가.
- (C) 무제한 concurrent: Reject. fanout dispatch overload 위험.

**근거**: bounded concurrency가 latency-throughput tradeoff의 sweet
spot. errgroup pattern은 FAN-001(`internal/fanout/dispatch.go`)에서
이미 활용 중이라 코드베이스 일관성 확보.

---

## 7. Risks & Mitigations

### 7.1 Risk: Unbounded branching

**현상**: LLM이 prompt instruction을 무시하고 breadth 초과 sub-query를
emit할 수 있음.

**완화**:

- Sub-query generation prompt에 breadth를 strict cap으로 명시.
- Python sidecar의 `/decompose_query` 응답에서 `len(sub_queries) > breadth`
  이면 excess를 truncate(warning log).
- Structural cap이 strict invariant — pre-expansion check에서 `len(visited)
  >= breadth^(depth+1)` 위반 시 reject.

### 7.2 Risk: Citation drift across deep nodes

**현상**: depth=3 노드의 claim이 root query와 무관한 doc을 인용할 수 있음.

**완화**:

- 매 노드의 `Citations`는 해당 노드의 fanout 결과만 포함(타 노드 doc
  cross-reference 금지 by construction).
- Writer가 받는 `FlattenedClaim`에 `lineage_path` 필수 포함 — 사용자에게
  drift가 visible.
- Acceptance scenario 5.4가 lineage traceability를 검증.

### 7.3 Risk: Cost blowout

**현상**: depth=3 + breadth=4 시 LLM 호출 횟수가 (1 + 4 + 16 + 64) = 85
sub-query generation + 85 fanout = 약 85 LLM 호출.

**완화**:

- Researcher는 Haiku tier(SPEC-DEEP-002 D5)로 고정 — Sonnet 대비 ~12x
  저렴.
- `DEEP_TREE_TOKEN_BUDGET` default 60K는 약 0.10 USD 미만 cost를 의미
  (Haiku 가격 기준).
- DEEP-004가 per-user/day cap을 layer up.

### 7.4 Risk: Latency p95 미달

**현상**: 노드 수 80개 sequential 시 약 80 × 5초 = 400초로 p95 ≤ 4분
초과.

**완화**:

- D7의 concurrent expansion(`breadth`개 parallel per depth level)로
  latency가 depth에 비례(약 depth × per-node = 3 × 30s = 90s 평균).
- Per-node timeout 30s가 long tail 차단.
- Acceptance scenario 5.1이 default config 하에서 p95 ≤ 4분 검증.

### 7.5 Risk: Persistence sidecar crash 시 데이터 손실

**현상**: 트리 expand 중 server crash 시 `.moai/runs/{run_id}/tree.json`
이 partial state로 남음.

**완화**:

- 매 노드 완료 시점에 atomic flush (write to `.tmp`, rename).
- 트리 reload 시 `Status != NodeStatusComplete` 노드는 `NodeStatusFailed`
  로 reclassify.
- Resume은 본 SPEC 범위 밖(§4 Exclusions) — 단순 부분 복원만 지원.

### 7.6 Risk: Researcher 사이드카 신규 endpoint deployment drift

**현상**: `services/researcher/`가 deploy되지 않은 환경에서 본 SPEC의
tree.go가 404를 받음.

**완화**:

- `services/researcher/Dockerfile` health check가 `/decompose_query`
  endpoint 존재 확인.
- Go-side에서 startup 시 readyz probe로 endpoint 가용성 검증.
- 미가용 시 본 SPEC 모드는 disabled — Researcher 에이전트가
  legacy single-shot fanout으로 fallback (config flag로 제어).

---

**End of Research Document — SPEC-DEEP-003 v0.1.0**
