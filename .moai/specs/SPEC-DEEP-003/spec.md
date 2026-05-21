---
id: SPEC-DEEP-003
version: 0.1.2
status: completed
created: 2026-05-21
updated: 2026-05-22
author: limbowl
priority: P0
issue_number: 19
title: Tree exploration for /deep multi-agent (configurable breadth/depth, budget cap)
milestone: M5 — /deep multi-agent
owner: expert-backend
methodology: tdd
coverage_target: 85
depends_on: [SPEC-DEEP-001, SPEC-DEEP-002, SPEC-SYN-001, SPEC-SYN-004, SPEC-LLM-001, SPEC-OBS-001, SPEC-FAN-001, SPEC-CORE-001]
blocks: [SPEC-DEEP-004]
---

# SPEC-DEEP-003: Tree exploration for /deep multi-agent (configurable breadth/depth, budget cap)

## HISTORY

- 2026-05-21: OQ-1 RESOLVED — golang-migrate/migrate pinned with sequence `0002` (`0002_deep_runs.up.sql` + `0002_deep_runs.down.sql`). T-D-009 unblocked. spec.md §8, plan.md D3, tasks.md T-D-009/Verification synchronized.
- 2026-05-21: status draft → planned. tasks.md generated (44 tasks across Phases A-D + Python sidecar + Observability — Phase A 7, Phase B 14, Phase C 6, Phase D 10, Phase E 9). Full REQ/NFR coverage matrix attached; 6 concurrency-sensitive tasks tagged for `go test -race`.
- 2026-05-21 (v0.1.2 audit patches iter-2, limbowl via manager-spec):
  plan-auditor iter-2 review identified 3 BLOCKER + 4 MAJOR + 3 MINOR
  findings remaining after v0.1.1. This revision applies all BLOCKER
  fixes, all MAJOR fixes, and 3 selected MINOR fixes. Status remains
  `draft` pending re-audit.
  - P-B1 (Budget pre-check concurrency race): REQ-DEEP3-006 +
    REQ-DEEP3-007 + NFR-DEEP3-003 hardened with reservation lock
    semantics. Pre-check now holds exclusive mutex on
    `tree.TotalTokensUsed` during (read + decision + reservation) and
    atomically reserves `estimated_next_cost`. Added `Node.ReservedTokens`
    field. Post-LLM accumulation releases reservation atomically in the
    same critical section. New acceptance §5.3 sub-scenario verifies
    `final_total ≤ budget_cap` under worst-case parallel breadth race.
  - P-B2 (REQ-005 fallback wire contract mutation): REQ-DEEP3-005
    rewritten to emit fallback signal via HTTP response header
    `X-Deep-Tree-Fallback: breadth_zero | depth_zero | disabled`
    (out-of-band side channel). Response SSE/JSON body remains
    byte-identical to DEEP-002 contract. Acceptance §5.6 sub-scenarios
    updated to verify header presence + body unchanged (new
    `TestFallbackHeaderEmittedAndBodyUnchanged`).
  - P-B3 (REQ-011 EARS structure broken): REQ-DEEP3-011 split into
    011a (State-Driven, expansion-phase flush) + 011b (Event-Driven,
    reload-phase reclassify). Numbering of REQ-012 preserved.
    Acceptance §5.5 split mapping into 011a (flush) and 011b
    (reclassify) assertions.
  - P-M1 (Per-node cost cap exclusion firmness): §4 Exclusions
    Per-node cost cap entry hedged — drops firm "input" assertion to
    mirror §6.2 hedge language. DEEP-004 interface binding deferred to
    DEEP-004 implementation stage by both SPEC authors jointly.
  - P-M2 (Cost calculation): New REQ-DEEP3-013 added — per-node
    `Node.CostUSD = node.TokensUsed * model_price_per_token` computed
    on expand completion, lookup via `.moai/config/sections/deep.yaml`
    `pricing.{model}` map. `Node.CostUSD` added to plan.md A1 type
    list. REQ-DEEP3-011a updated — `total_cost_usd` is derived from
    summed `node.CostUSD` for the Postgres summary row.
  - P-M3 (NFR-007 size bound ambiguity): NFR-DEEP3-007 closing
    sentence rewritten — 200 KB ceiling is inherently bounded by
    NFR-004 + NFR-003. Removed "violation 시 expand 중단" runtime
    check (overshoot signals an upstream budget enforcement defect, not
    a separate size gate).
  - P-M4 (Memory bound on in-memory tree): New NFR-DEEP3-009 added —
    single-tree in-memory state (`Node[]` + `Citations` + `Claims`)
    SHALL NOT exceed 100 MB at worst-case (`breadth=8, depth=5`).
    Measured at each depth-level join; overshoot triggers frontier
    truncation. Acceptance §4.X gate added.
  - P-N1 (REQ-009 split): REQ-DEEP3-009 split into 009a (prompt
    context flow) + 009b (citation slice disjointness). Preserves
    semantic content of v0.1.1 unification; clarifies test boundary.
  - P-N3 (Plan B3 explicitness): plan.md Phase B3 explicit —
    "ExpandTree creates a fresh `errgroup` per depth level, joining
    before advancing to depth N+1."
  - P-N5 (Stale issue_number): §4 Exclusions last bullet
    `issue_number: 0` reference replaced with `issue_number: 19`
    (matches frontmatter; v0.1.1 already patched header but missed
    body callout).
  - Deferred from iter-2: MINOR/NIT items not listed in iter-2
    findings (rationale: low-priority editorial cleanups).
  Companion artifact updates: plan.md, acceptance.md, spec-compact.md
  all synced. spec-compact.md priorities updated. Review report:
  `.moai/reports/plan-audit/SPEC-DEEP-003-review-2.md`.
- 2026-05-21 (v0.1.1 audit patches, limbowl via manager-spec):
  plan-auditor iter-1 (`.moai/reports/plan-audit/SPEC-DEEP-003-review-1.md`)
  반환 verdict FAIL (overall 0.66). 3 BLOCKER + 4 MAJOR + 5 MINOR +
  3 NIT 발견. 본 리비전에서 BLOCKER + MAJOR 전부 + MINOR 일부를
  patch:
  - P-B1 (D1): "DEEP-002 REQ-002" → "DEEP-002 REQ-005" 일괄 치환.
    Researcher single-shot 동작은 REQ-DEEP2-005가 owner이고
    REQ-DEEP2-002는 orchestrator sequence + Reviewer no-retrieval
    invariant. spec.md 5개 site 치환 — Researcher 다이어그램,
    아키텍처 결정사항 §1.1.5, REQ-DEEP3-001 fallback 설명,
    REQ-DEEP3-005 본문. Reviewer 다이어그램은 Reviewer no-retrieval
    invariant가 REQ-DEEP2-002 소속이므로 "REQ-002, no-retrieval
    invariant"로 부연만 추가. patching 중 v0.1.0 HISTORY 본문에 동일
    `Researcher 에이전트(REQ-002)` 참조 추가 site 발견 → 일관성 위해
    함께 정정(원 history의 사실 기록을 정확히 유지하기 위함).
    acceptance.md L256, spec-compact.md L26도 동일 패턴 치환.
  - P-B2 (D2): `depth=0` 모순 해소. 단일 정책 채택 —
    "`breadth=0` OR `depth=0` → REQ-DEEP3-005 single-shot fallback
    (HTTP 200)". REQ-DEEP3-002 carve-out 절을 명시적으로
    "`breadth=0` AND `depth=0` 입력은 별도 처리로 REQ-DEEP3-005가
    다루며 본 범위 위반에 해당하지 않는다"로 갱신. REQ-DEEP3-005는
    이미 두 케이스 모두 fallback으로 다루므로 표현만 강조. acceptance.md
    §5.2의 "`depth=0` → HTTP 400" 주장 삭제 → "`depth=6` → HTTP 400"만
    invalid-range로 잔존. §5.6 시나리오 명칭을 "breadth=0 fallback"
    →"breadth=0 OR depth=0 fallback"으로 rename, depth=0 sub-scenario
    추가. spec.md §5 인덱스 + spec-compact.md acceptance 요약도 mirror.
  - P-B3 (D3): spec.md L104 Writer attribution 정정.
    "[DEEP-002 REQ-006, FlattenedClaim 소비]" →
    "[DEEP-002 Writer agent; primary contract via REQ-DEEP2-003 retry
    semantics; FlattenedClaim contract introduced by REQ-DEEP3-010]".
    REQ-DEEP2-006은 Verifier의 CheckFaithfulness invocation owner이며
    Writer 계약과 무관.
  - P-M1 (D4): §6.2 DEEP-004 consumption claim 약화.
    "DEEP-004는 본 SPEC이 노출하는 ... 메트릭을 input으로 받아 quota를
    계산. 본 SPEC의 budget hooks 없이 DEEP-004는 구현 불가" 표현을
    "DEEP-004는 본 SPEC을 capacity planning (cap dimension calibration)
    상류 의존성으로 참조한다. 구체적인 메트릭/필드 소비(Node.TokensUsed,
    usearch_deep_tree_total_tokens)는 가용함을 본 SPEC이 문서화하나
    구속력 있는 인터페이스 합의는 DEEP-004 구현 단계로 연기한다"로
    완화.
  - P-M2 (D5): 마이그레이션 디렉토리 경로 정정. 본 repo는
    `deploy/postgres/migrations/`(이미 `0001_create_docs.sql` 존재)을
    표준 디렉토리로 사용. 마이그레이션 도구 핀은 본 SPEC의 책임 범위가
    아니므로 §1.1.4 + §7 + §8 Open Questions로 이관. 다음 시퀀스 번호와
    도구 선택은 §8 Open Questions에서 미해결 항목으로 명시.
  - P-M3 (D6): REQ-DEEP3-006 root-node seed 절 추가. "root node는
    `parent.TokensUsed`가 없으므로 pre-check는 `DEEP_TREE_ROOT_TOKEN_ESTIMATE`
    (default 5000, `.moai/config/sections/deep.yaml`에서 override 가능)
    seed로 시뮬레이션한다" 명시. §9 env-var 표에 `DEEP_TREE_ROOT_TOKEN_ESTIMATE`
    행 추가. acceptance.md §5.3 sub-scenario에 root seed 검증 케이스 추가.
  - P-M4 (D7): NFR-DEEP3-008만의 reload-and-reclassify 동작을 functional
    REQ로 승격. REQ-DEEP3-011에 새 절 추가: "persistence layer는 reload
    경로(SPEC-DEEP-004 audit 기능에서 invoke)에서 `Status ∈ {Pending,
    Expanding}` 노드를 `Failed`로 reclassify SHALL 하고 트리를 read-only
    로 반환한다." NFR-DEEP3-008은 quality attribute로만 잔존(reload된
    트리의 readable 보장). acceptance.md §5.5에 reclassify assertion
    강화.
  - P-N1 (D8): REQ-DEEP3-009 by-construction 문구 명료화 — sibling
    노드의 fanout 결과로 동일 doc_id가 우연히 양 슬라이스에 등장하는 것은
    허용, "타 노드 Citations 슬라이스를 reference하는" 행위만 금지로
    rewording.
  - P-N2 (D10): NFR-DEEP3-002 측정 반복 25 → 50으로 상향 (p95 통계적
    검출력 확보, NFR-DEEP3-001과 정합).
  - P-N3 (D12): "## 1.0 Overview" → "## 1. Overview"(sibling
    DEEP-002/DEEP-004과 일관).
  - Deferred: D9 (MINOR — Status transition trigger 표현, 본문 의미상
    이미 명확), D11 (MINOR — REQ-005 모듈 분류 editorial), D13/D14/D15
    (NIT — Owner 컬럼 명, Exclusion 처리, deep.yaml naming).
  - 알림: research.md(Phase 0.5 artifact, immutable historical record)에
    `migrations/0NN_deep_runs.up.sql` 표현이 잔존하나 spec.md/plan.md
    canonical 경로는 `deploy/postgres/migrations/`로 동기화 완료.
    Research artifact는 작성 시점의 의사결정 기록으로 보존하며 수정하지
    않는다.
  본 리비전은 위 patch 외 REQ/NFR/Exclusion 구조 변경 없음.
  Review report: .moai/reports/plan-audit/SPEC-DEEP-003-review-1.md
- 2026-05-21 (initial draft v0.1.0, limbowl via manager-spec):
  First EARS-formatted SPEC for the M5 third deliverable —
  deep-research tree exploration on top of the `/deep?mode=agents`
  pipeline (SPEC-DEEP-002). Sub-query를 BFS로 확장하는 multi-level
  evidence collection을 도입하여 multi-hop reasoning을 지원한다.
  7개 핵심 아키텍처 결정사항은 `.moai/specs/SPEC-DEEP-003/research.md`
  §6에서 pinned 상태이며 본 SPEC은 그 결정을 EARS 요구사항으로
  번역한다. 요지:
  (1) Tree orchestration은 신규 Go 모듈 `internal/deepagent/tree.go`
  전담. Python sidecar는 per-node sub-query generation을 위한
  thin LLM wrapper(`POST /decompose_query`)만 제공.
  (2) Default `breadth=4, depth=3` (gpt-researcher upstream
  convention과 일치). `.moai/config/sections/deep.yaml` 신규 + per-request
  body field로 override 가능.
  (3) Budget enforcement는 세 cap을 simultaneously 적용 — total
  token budget(default 60K) + per-node timeout(default 30s) +
  structural breadth×depth cap.
  (4) Tree persistence는 JSON sidecar
  (`.moai/runs/{run_id}/tree.json`) + Postgres `deep_runs` summary
  row. 트리 자체는 audit 목적상 영구 보관.
  (5) Sub-query 생성은 DEEP-002 Researcher 에이전트(REQ-005)를
  per-node 재사용. 별도 "Decomposer" 에이전트 신설하지 않음.
  (6) Budget exhaustion 시 잔여 frontier 노드를
  `NodeStatusBudgetExceeded`로 표시하고 부분 트리를 Writer에 반환
  (전체 abort 금지).
  (7) Concurrent expansion — 각 depth level에서 `breadth`개의 노드를
  goroutine pool(`errgroup.WithContext`)로 parallel expand. FAN-001의
  bounded concurrency 패턴 차용.

  본 SPEC은 DEEP-002의 Researcher 에이전트가 `tree mode`(per-request
  flag)로 호출될 때만 활성화되며, default(single-shot) mode와 무손상
  공존한다. DEEP-002의 SSE 이벤트 taxonomy는 변경 없이 그대로
  사용되며, 본 SPEC은 Researcher 노드 내부 동작만 확장한다.

  Companion artifacts:
  - `.moai/specs/SPEC-DEEP-003/research.md` — Phase 0.5 deep
    research (7 sections — node 데이터 구조, gpt-researcher 패턴
    분석, budget model, citation lineage, reference impls, 7개
    pinned decisions, 6개 risks)
  - `.moai/specs/SPEC-DEEP-003/plan.md` — TDD task sequence, 5단계
    phase breakdown, MX tag plan
  - `.moai/specs/SPEC-DEEP-003/acceptance.md` — Given/When/Then
    scenarios (5 main + 2 edge cases)
  - `.moai/specs/SPEC-DEEP-003/spec-compact.md` — compact view

  12 EARS REQs (10 × P0 + 2 × P1), 8 NFRs, ≥6 exclusions, ≥5
  acceptance scenarios. Methodology: TDD (per quality.yaml),
  coverage target 85%, harness: standard. Owner: expert-backend.
  `issue_number: 0` 상태이며 plan-auditor 리뷰 + annotation cycle
  통과 후 status `draft → approved` 전이.

---

## 1. Overview

본 SPEC은 M5 milestone의 세 번째 deliverable인 `/deep` 트리 익스플로러를
정의한다. SPEC-DEEP-002의 4-에이전트 파이프라인이 sequential linear path
하나를 실행하는 반면, 본 SPEC은 Researcher 에이전트 단계 안에 BFS
sub-query 확장을 도입하여 multi-hop reasoning이 필요한 query에 대해
다층 evidence를 수집한다.

### 사용자 가치

- Multi-hop reasoning이 필요한 query(예: "양자컴퓨터의 신약 개발 응용은
  현재 어느 단계인가?")에 대해, single-shot fanout이 놓치는 중간 단계의
  근거(양자컴퓨터 기초 → 단백질 폴딩 알고리즘 → 신약 후보 스크리닝)를
  자동 수집.
- Configurable breadth/depth로 사용자가 탐색 깊이를 통제.
- Budget cap이 cost runaway를 차단(DEEP-004가 per-user cap을 layer up).

### M5 파이프라인 내 위치

```
/deep?mode=agents [SPEC-DEEP-002]
  → Researcher [DEEP-002 REQ-005]
      │
      ├─ tree mode (본 SPEC, per-request flag로 활성화)
      │    └─ BFS expand → multi-level Node[] → flatten with lineage
      │
      └─ single-shot mode (DEEP-002 default, fanout 결과를 그대로 사용)
  → Reviewer [DEEP-002 REQ-002, no-retrieval invariant]
  → Writer [DEEP-002 Writer agent; primary contract via REQ-DEEP2-003 retry semantics; FlattenedClaim contract introduced by REQ-DEEP3-010]
  → Verifier [DEEP-002 REQ-006]
```

본 SPEC은 Researcher 에이전트 내부 동작만 확장하며, Reviewer/Writer/
Verifier의 contract는 변경하지 않는다.

### 1.1 Architecture Decisions (Pinned)

다음 7개 결정사항은 research.md §6에서 확정되었다. 본 SPEC은 이
결정사항들을 EARS 요구사항으로 번역할 뿐이며 재논의하지 않는다.

1. **Tree orchestration host**: NEW Go file `internal/deepagent/tree.go`
   (DEEP-002의 `internal/deepagent/` 모듈 내). Python sidecar는
   per-node sub-query generation을 위한 thin LLM wrapper(`POST
   /decompose_query`)만 제공.
2. **Default (breadth, depth)**: `breadth=4, depth=3`. config override
   는 `.moai/config/sections/deep.yaml`(신규) + per-request body field
   `{tree: {breadth, depth}}`.
3. **Budget enforcement**: 세 가지 cap을 simultaneously enforce — (a)
   total token budget(`DEEP_TREE_TOKEN_BUDGET`, default 60000), (b)
   per-node timeout(`DEEP_TREE_NODE_TIMEOUT_MS`, default 30000), (c)
   structural breadth×depth cap.
4. **Tree persistence**: JSON sidecar(`.moai/runs/{run_id}/tree.json`)
   atomic flush per node + Postgres `deep_runs` summary row. Migration
   파일은 repo 표준 디렉토리 `deploy/postgres/migrations/`에 배치(기존
   `0001_create_docs.sql` 와 동일 위치). 다음 sequence 번호와 migration
   도구(예: golang-migrate, goose, sqlx-migrate) 선택은 본 SPEC 범위
   밖의 프로젝트 전체 인프라 결정이므로 §8 Open Questions로 이관.
5. **Sub-query generation**: DEEP-002 Researcher 에이전트(REQ-005)를
   per-node 재사용. 별도 "Decomposer" 에이전트 신설 없음.
6. **Budget exhaustion 시 동작**: 잔여 frontier 노드를
   `NodeStatusBudgetExceeded`로 표시하고 부분 트리를 Writer로 반환
   (전체 abort 금지).
7. **Concurrent expansion**: 각 depth level에서 `breadth`개의 노드를
   goroutine pool(`errgroup.WithContext`)로 parallel expand.

---

## 2. Functional Requirements

본 SPEC은 15개 functional requirement(REQ-001..012 + 009a/b split +
011a/b split + 013)를 6개 모듈로 분류한다:

- **Tree Initialization** (REQ-001, 002): 트리 root 생성, mode dispatch
- **Node Expansion** (REQ-003, 004, 005): BFS expand loop, parallel
  concurrency, sub-query generation
- **Budget Enforcement** (REQ-006, 007, 008): 3-dimension cap with
  reservation lock semantics, partial return
- **Citation Lineage** (REQ-009a, 009b, 010): prompt context flow,
  citation slice disjointness, Writer 소비 contract
- **Persistence & Observability** (REQ-011a, 011b, 012): JSON sidecar
  flush + reload-mode reclassify, Postgres row, Prometheus metrics, OTel
  spans
- **Cost Accounting** (REQ-013): per-node CostUSD, tree-wide
  TotalCostUSD derivation

### 2.1 Tree Initialization Module

**REQ-DEEP3-001** (Event-Driven):
WHEN `/deep?mode=agents` 요청 본문에 `{tree: {breadth, depth}}` 객체가
존재하거나 `.moai/config/sections/deep.yaml`의 `tree.enabled: true`가
설정된 경우, 트리 익스플로러는 root Node(`Depth=0, ParentID="",
BreadthIndex=0, Query=request.query, Status=NodeStatusPending`)를
SHALL 생성한다. Root Node의 ID는 `run_id`(= DEEP-002 request_id)로부터
deterministic하게 도출(`hash(run_id || "root")`) SHALL 한다. 본
요구사항이 만족되지 않을 때(즉 tree mode disabled) Researcher는 DEEP-002
REQ-005의 single-shot fanout 동작을 변경 없이 수행 SHALL 한다.
(Acceptance §5.1)

**REQ-DEEP3-002** (Ubiquitous):
트리 익스플로러는 모든 요청에 대해 `breadth ∈ [1, 8]` AND `depth ∈ [1,
5]` 범위를 SHALL 검증한다. 범위 밖 값이 들어오면 HTTP 400 응답 body
`{"error": "invalid_tree_config", "detail": "<reason>", "breadth": <N>,
"depth": <M>}`를 SHALL 반환한다. `breadth=0` AND `depth=0` 입력은 별도
처리로 REQ-DEEP3-005가 다루며 본 요구사항의 범위 위반에 해당하지 않는다
(두 값 모두 single-shot fallback signal로 정상 HTTP 200 응답을 받는다).
(Acceptance §5.6 edge)

### 2.2 Node Expansion Module

**REQ-DEEP3-003** (State-Driven):
WHILE root Node가 `NodeStatusPending` 상태인 동안, 트리 익스플로러는
다음 BFS expand 알고리즘을 SHALL 수행한다: (a) root Node를 `Status=
NodeStatusExpanding`으로 transition. (b) root Node에 대해
`fanout.Dispatch(ctx, root.Query, registry, router)` 호출(SPEC-FAN-001
재사용). (c) 결과 docs를 root Node의 Citations로 저장. (d) Python
sidecar `POST /decompose_query` 호출하여 `breadth`개의 sub-query 생성.
(e) 각 sub-query에 대해 child Node 생성(`Depth=1, ParentID=root.ID,
BreadthIndex=i, Status=NodeStatusPending`). (f) `Status=NodeStatusComplete`로
transition. (g) child Node frontier에 대해 동일 알고리즘을 재귀 적용 —
`Depth < depth` 조건 만족 시에만. `Depth == depth` 노드는 sub-query
생성 단계(d-e)를 SHALL skip하고 fanout만 수행한다(leaf node 동작).
(Acceptance §5.1)

**REQ-DEEP3-004** (State-Driven):
WHILE 트리 expand가 진행 중인 동안, 동일 depth level의 노드들은
`errgroup.WithContext`로 wrapped된 goroutine pool에서 parallel expand
SHALL 된다. Pool size는 `breadth`로 고정 SHALL 되며, 각 노드의 expand
는 `context.WithTimeout(ctx, DEEP_TREE_NODE_TIMEOUT_MS)` 으로 격리
SHALL 된다. Depth N+1 노드의 expand는 Depth N의 모든 노드가
`NodeStatusComplete` OR `NodeStatusFailed` OR `NodeStatusBudgetExceeded`
상태에 도달한 후에만 시작 SHALL 한다(BFS invariant). 단일 노드가 timeout
에 도달하면 해당 노드만 `NodeStatusFailed`로 표시되고 sibling 노드의
expand는 영향 받지 SHALL 않는다.
(Acceptance §5.1, §5.5)

**REQ-DEEP3-005** (Conditional):
IF 사용자가 deep.yaml 또는 request body에서 `breadth=0` OR `depth=0`을
지정한 경우, 트리 익스플로러는 트리 모드를 disable하고 SPEC-DEEP-002
REQ-005의 single-shot Researcher 동작으로 SHALL fallback한다. 두 입력
모두 invalid-range가 아닌 명시적 fallback signal로 해석된다(REQ-DEEP3-002
범위 검증의 대상이 아님). 이 fallback signal은 HTTP 응답 헤더
`X-Deep-Tree-Fallback: breadth_zero | depth_zero | disabled`를 통해
out-of-band side channel로 SHALL emit된다. 응답 SSE/JSON body는
DEEP-002 single-shot contract와 byte-identical하게 유지 SHALL 되며,
본 SPEC은 DEEP-002 응답 body 구조를 mutate하지 않는다. `breadth=0`
AND `depth=0`이 동시 지정된 경우 `X-Deep-Tree-Fallback: breadth_zero`가
우선 emit된다. 사용자는 트리 결과 대신 single-shot fanout 결과를
DEEP-002 contract 그대로 받는다.
(Acceptance §5.6 edge)

### 2.3 Budget Enforcement Module

**REQ-DEEP3-006** (Event-Driven):
WHEN 다음 노드 expand 직전 budget pre-check가 수행되는 시점에, 트리
익스플로러는 (a) `tree.TotalTokensUsed + tree.TotalReservedTokens +
estimated_next_cost > DEEP_TREE_TOKEN_BUDGET`인 경우, OR (b)
`len(visited_nodes) >= 1 + sum(breadth^i for i in 1..depth)`인 경우, 해당
노드와 그 frontier descendants를 `NodeStatusBudgetExceeded`로 SHALL 표시
하고 expand를 중단한다. Pre-check는 `tree.TotalTokensUsed`에 대해 exclusive
mutex(reservation lock)을 hold하는 동안 (read + decision + reservation)을
SHALL atomic하게 수행한다. Pre-check가 success로 판정되면 트리 익스플로러
는 동일 critical section 내에서 `estimated_next_cost`를 `Node.ReservedTokens`
필드에 SHALL 기록하고 `tree.TotalReservedTokens`를 atomic하게 increment
한다. 노드 완료 시 reservation은 REQ-DEEP3-007의 release 시퀀스로 reconcile
된다(`released = ReservedTokens - actual_TokensUsed`). 동일 depth level
의 sibling 노드들의 pre-check는 본 reservation lock에서 SHALL serialize
된다(`breadth=8` 동시 dispatch 하에서도 race-free). `estimated_next_cost`
는 conservative estimate로 `parent.TokensUsed * breadth * 1.25`로 산정
SHALL 한다. Root node의 경우 `parent.TokensUsed`가 존재하지 않으므로,
pre-check는 `DEEP_TREE_ROOT_TOKEN_ESTIMATE`(default 5000 tokens,
`.moai/config/sections/deep.yaml`에서 override 가능) seed 값으로 시뮬레이션
SHALL 한다. 이 seed는 typical research query expansion의 root cost를
근사한다.
(Acceptance §5.3)

**REQ-DEEP3-007** (State-Driven):
WHILE 노드의 expand가 진행 중인 동안, 트리 익스플로러는 각 LLM 호출
직전과 직후에 `node.TokensUsed`를 accumulate SHALL 한다. Post-LLM-call
accumulation은 reservation lock(REQ-DEEP3-006과 동일 mutex)을 hold하는
single critical section 내에서 (a) `tree.TotalTokensUsed`를 `node.TokensUsed`
delta만큼 atomic하게 increment SHALL 하고, 동시에 (b) 노드 완료 시점에
`released = Node.ReservedTokens - node.TokensUsed`를 산정하여
`tree.TotalReservedTokens`를 `released`만큼 atomic하게 decrement SHALL
한다. 즉, "actual 가산 + reservation 해제"는 단일 atomic section에서
수행되어 다른 sibling의 pre-check에 budget 변화가 일관되게 노출된다.
`fanout.Dispatch()` 호출의 cost(외부 adapter cost 포함)는 본 카운터에
SHALL 포함되지 않는다(token cap은 LLM cost만 추적, fanout adapter
overhead는 별도 SPEC-FAN-001 가시화).
(Acceptance §5.3)

**REQ-DEEP3-008** (Conditional):
IF budget exhaustion으로 인해 root Node가 `NodeStatusComplete`에
도달했으나 frontier 노드 일부가 `NodeStatusBudgetExceeded` 상태인 경우,
트리 익스플로러는 트리 결과를 Writer로 SHALL 전달하되 응답 body의
`usage` 필드에 `{budget_exceeded: true, total_tokens: <N>,
total_nodes_completed: <C>, total_nodes_skipped: <S>}` metadata를 SHALL
포함한다. HTTP status는 200 (degraded success) — 부분 트리도 유의미한
답변을 생성할 수 있으므로 5xx error로 분류하지 않는다.
(Acceptance §5.3)

### 2.4 Citation Lineage Module

**REQ-DEEP3-009a** (Ubiquitous):
트리 익스플로러는 모든 노드 expand 시 부모 노드의 query 컨텍스트를 자식
노드 prompt에 SHALL 포함한다(`{root_query, parent_query,
parent_evidence_summary}` 세 필드를 `/decompose_query` request에 전달).
이로써 sub-query 생성 시 LLM이 root context를 잃지 않는다.
(Acceptance §5.4 prompt context flow)

**REQ-DEEP3-009b** (Ubiquitous):
각 `Node.Citations`는 해당 노드 자신의 `fanout.Dispatch` 호출이 반환한
doc_id 만 포함 SHALL 한다. 서로 다른 노드의 독립 fanout 결과가 동일
doc_id를 우연히 포함하는 것은 허용된다(popular doc은 여러 sub-query에서
공통으로 검색될 수 있음). 금지되는 것은 *타 노드의 Citations 슬라이스를
직접 reference하거나 inherit하는* 행위뿐이다 — 각 노드의 Citations는
disjointly owned 슬라이스이다.
(Acceptance §5.4 citation disjointness invariant)

**REQ-DEEP3-010** (Ubiquitous):
트리 expand 완료 후, 트리 익스플로러는 `TreeResult` struct(`{root_query,
total_nodes, max_depth_reached, status, flattened_claims:
[]FlattenedClaim, citations: []Citation}`)를 SHALL 생성하여 Writer로
전달한다. `FlattenedClaim`은 `{text, markers, lineage_path:
[]string, source_node_id}` 필드를 SHALL 포함하며, `lineage_path`는
root에서 source_node까지의 query trace를 순서대로 담은 string slice
SHALL 이다(예: `["root: 양자컴퓨터 응용", "depth1: 양자컴퓨터 의료
응용", "depth2: 양자컴퓨터 단백질 폴딩"]`).
(Acceptance §5.4)

### 2.5 Persistence & Observability Module

**REQ-DEEP3-011a** (State-Driven):
WHILE 트리 expansion이 진행 중인 동안, persistence layer는 매 depth-level
join 직후 완료된 노드들의 상태를 `.moai/runs/{run_id}/tree.json`
snapshot으로 SHALL atomic flush한다(write to `.tmp` + rename pattern).
매 노드 완료 시점(`Status` transition to one of `{Complete, Failed,
BudgetExceeded}`)의 변화도 동일 atomic flush 경로를 통해 disk로
durably 반영 SHALL 된다. 트리 expand 완료 후, persistence layer는
Postgres `deep_runs` 테이블에 단일 summary row `{run_id, query, breadth,
depth, total_nodes, total_tokens, total_cost_usd, status, started_at,
completed_at}`을 SHALL insert한다. `total_cost_usd`는 REQ-DEEP3-013의
per-node `Node.CostUSD` 합산으로 derive SHALL 된다. Schema migration은
`deploy/postgres/migrations/`(repo 표준 디렉토리; 다음 시퀀스 번호와
도구 선택은 §8 Open Questions 참조)에 SHALL 제공된다.
(Acceptance §5.5 flush flow)

**REQ-DEEP3-011b** (Event-Driven):
WHEN persistence layer가 reload mode로 invoke되는 경우(entry point:
SPEC-DEEP-004 audit 기능), persistence layer는 `Status ∈ {Pending,
Expanding}` 노드를 in-memory에서 `Failed`로 SHALL reclassify하고 트리를
read-only로 반환한다. Reclassify는 in-memory 변환에 한정 SHALL 되며,
disk의 flush된 `tree.json` 원본 파일은 audit 무결성을 위해 불변으로
유지 SHALL 된다(overwrite 금지). Reload된 트리에 대한 추가 expand 시도
는 차단 SHALL 된다.
(Acceptance §5.5 reclassify flow)

**REQ-DEEP3-012** (Optional):
WHERE OpenTelemetry tracing이 활성화된 경우(SPEC-OBS-001 NFR-OBS-003),
트리 익스플로러는 매 노드에 대해 OTel span을 SHALL 발행하고 부모-자식
span linkage(`parent_span_id`)를 명시 SHALL 한다. 또한 Prometheus
histogram `usearch_deep_tree_node_expand_seconds{depth, outcome}`
(buckets `[0.5, 1, 2, 5, 10, 30, 60]`, label `depth ∈ {0, 1, 2, 3, 4,
5}`(6 values, bounded), `outcome ∈ {success, failed, budget_exceeded}`
(3 values, bounded), cardinality 6×3=18) AND counter
`usearch_deep_tree_total_tokens{outcome}`(label `outcome ∈ {pass,
budget_exceeded}`, cardinality 2)를 SHALL emit한다.
(Acceptance §5.1)

### 2.6 Cost Accounting Module

**REQ-DEEP3-013** (Ubiquitous):
트리 익스플로러는 매 노드 expand 완료 시 `Node.CostUSD = Node.TokensUsed *
model_price_per_token(DEEP_TREE_DECOMPOSE_MODEL)`을 SHALL 계산하여
`Node.CostUSD float64` 필드에 기록한다. `model_price_per_token`은
`.moai/config/sections/deep.yaml`의 `pricing.{model}` map에서 SHALL
조회한다(미등록 모델인 경우 `0.0`을 기록하고 warning log emit). 트리
전체 비용은 `tree.TotalCostUSD = sum(node.CostUSD for node in tree.nodes
if node.Status == NodeStatusComplete)`로 derive SHALL 된다.
`tree.TotalCostUSD`는 REQ-DEEP3-011a의 Postgres summary row
`total_cost_usd` 컬럼의 source이다. 본 REQ는 본 SPEC이 노출하는
하류(SPEC-DEEP-004) consumer 인터페이스의 cost dimension을 형성한다.
(Acceptance §5.1 cost coverage)

---

## 3. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| NFR-DEEP3-001 | Per-node latency p95 | 단일 노드의 expand 시간(LLM decompose call + fanout dispatch + persistence flush)은 **p95 ≤ 30 s** SHALL 이다. 측정은 `internal/deepagent/tree_test.go`의 50-iteration mock 테스트로 검증. 본 NFR은 `DEEP_TREE_NODE_TIMEOUT_MS=30000`의 ceiling과 일치한다. |
| NFR-DEEP3-002 | Tree end-to-end p95 latency | 트리 전체 expansion(root + 모든 depth)의 wall-clock 시간은 default config(`breadth=4, depth=3`) 하에서 **p95 ≤ 4 min**(240 s) SHALL 이다. M5 milestone exit criterion(5 min)을 안전 마진으로 충족한다. 측정은 `internal/deepagent/tree_test.go`의 50-iteration end-to-end mock으로 검증(NFR-DEEP3-001과 정합한 통계적 검출력). |
| NFR-DEEP3-003 | Token budget enforcement | 트리 전체에서 소비된 LLM 토큰 합(`sum(node.TokensUsed) for completed nodes`)은 `DEEP_TREE_TOKEN_BUDGET`(default 60000)을 **SHALL NOT** 초과한다. REQ-DEEP3-006의 reservation lock semantics이 본 invariant를 보장한다 — pre-check + reservation은 단일 atomic critical section 내에서 수행되어, sibling 노드들이 `breadth=8`로 동시 dispatch되어도 `tree.TotalTokensUsed + tree.TotalReservedTokens`가 budget cap을 초과하지 못한다. Pre-check는 conservative estimate(25% headroom)로 over-reserve 가능성을 흡수하며, 실측 `node.TokensUsed`가 reservation보다 작은 경우 REQ-DEEP3-007의 atomic release로 잉여분이 즉시 budget pool에 반환된다. |
| NFR-DEEP3-004 | Structural cap | 트리의 노드 수는 `1 + sum(breadth^i for i in 1..depth)`을 **SHALL NOT** 초과한다. Default(`breadth=4, depth=3`) 하에서 최대 85 노드(1+4+16+64), 최악 운영 환경(`breadth=8, depth=5`) 하에서 최대 37449 노드 — 이 경우 NFR-DEEP3-003 token budget이 먼저 enforcement된다. |
| NFR-DEEP3-005 | Observability — Prometheus histogram | `usearch_deep_tree_node_expand_seconds{depth, outcome}` histogram이 매 노드 완료 시 정확히 한 번 observation을 SHALL 기록한다. Cardinality는 NFR-OBS-002 enumerable label set을 준수 — `depth ∈ {0, 1, 2, 3, 4, 5}`, `outcome ∈ {success, failed, budget_exceeded}`로 컴파일 타임 bound. |
| NFR-DEEP3-006 | Observability — OTel span linkage | OTel tracing 활성 시, 매 노드 expand 동안 child span이 parent node의 span을 `parent_span_id` attribute로 SHALL 참조한다. Trace tree depth는 트리 expansion depth와 일치 SHALL 한다. |
| NFR-DEEP3-007 | Persistence size bound | `.moai/runs/{run_id}/tree.json` 파일은 gzip 압축 후 **SHALL NOT** 200 KB를 초과한다. NFR-DEEP3-004 + NFR-DEEP3-003이 inherently bound한다 — 평균 노드 당 ~2 KB(citations + claims) × 85 nodes = ~170 KB raw, gzip 압축 시 ~50 KB 예상. 200 KB 초과는 budget enforcement 결함(상류 NFR-003/NFR-004 위반)을 의미하며 별도 runtime size check를 두지 않는다. |
| NFR-DEEP3-008 | Crash recovery | Sidecar crash 시 partial tree.json은 readable SHALL 이다. Reload 로직(SPEC-DEEP-004의 audit 기능에서 사용)이 `Status != NodeStatusComplete` 노드를 `NodeStatusFailed`로 reclassify SHALL 한다. Resume 기능은 본 SPEC 범위 밖(§4 Exclusions). |
| NFR-DEEP3-009 | In-memory tree size bound | 단일 트리의 in-memory state(`Node[]` + `Citations` + `Claims` 합산)는 worst-case config(`breadth=8, depth=5`)에서 **SHALL NOT** 100 MB를 초과한다. 측정은 매 depth-level 완료 시점(REQ-DEEP3-011a의 atomic flush 직전)에 수행 SHALL 되며, 100 MB 초과 감지 시 frontier 노드를 truncation(`NodeStatusBudgetExceeded`로 mark)하여 복원 SHALL 한다. 본 NFR은 NFR-DEEP3-003의 token budget이 헐겁게 설정된 케이스에서 in-memory representation overhead가 OOM trigger 되지 않도록 last-resort guard 역할을 한다. |

---

## 4. Exclusions (What NOT to Build)

[HARD] 본 SPEC은 다음 항목을 명시적으로 제외한다. 각 항목은 후속 SPEC
또는 별도 트랙의 책임이다.

- **Real-time UI tree visualization** — 트리 expansion 진행 상황을
  사용자에게 시각화하는 frontend 위젯은 본 SPEC 범위 밖. M7의
  SPEC-UI-001(Web UI v1)의 책임. 본 SPEC은 SSE 이벤트도 추가하지
  않는다(DEEP-002의 기존 `agent_started`/`agent_completed` 이벤트
  taxonomy 그대로 사용 — Researcher 단계가 tree 모드에서도 single
  agent_started + 단일 agent_completed로 wrap된다).
- **User-editable tree** — 사용자가 expand 중에 특정 노드를 prune
  하거나 추가 sub-query를 manual 추가하는 interactive 기능은 본 SPEC
  범위 밖. 트리는 read-only 한 번의 expansion으로 fully determined된다.
- **Resume of crashed tree** — Server crash 후 트리 expand를 재개하는
  기능은 본 SPEC 범위 밖. partial tree.json은 `NodeStatusFailed`로
  reclassify되어 audit 목적만 served된다. 새로운 `/deep` 요청은 fresh
  run_id로 처음부터 expand 시작.
- **Per-node cost cap** — 본 SPEC은 tree-wide token budget만 enforce
  한다. Per-node/per-user/per-day cost cap, prompt-cache reuse는
  SPEC-DEEP-004의 책임. 본 SPEC은 노드/트리 token consumption(
  `Node.TokensUsed`, `Node.CostUSD`, `usearch_deep_tree_total_tokens`)을
  하류 consumer가 가용한 형태로 노출하나, DEEP-004와의 구속력 있는
  인터페이스 합의는 §6.2의 hedge에 따라 DEEP-004 구현 단계에서 양 SPEC
  저자가 공동 확정한다.
- **Cross-tree query deduplication** — 동일 사용자가 유사 쿼리를 연속
  으로 expand할 때 이전 트리의 노드를 재사용하는 cache는 본 SPEC 범위
  밖. M6의 SPEC-IDX-005(Team-shared answer reuse)의 책임.
- **Automatic depth/breadth tuning by ML** — query 복잡도에 따라
  breadth/depth를 ML로 동적 추정하는 기능은 본 SPEC 범위 밖. 본 SPEC은
  static config + per-request override만 지원. M8 SPEC-EVAL-002 후속
  연구 트랙.
- **gpt-researcher PyPI package integration** — `services/researcher/`
  `pyproject.toml`에 `gpt-researcher` 패키지를 의존성으로 추가하지
  않는다. Decomposition prompt template은 upstream 컨벤션에서 영감을
  받지만 in-house 구현(research.md §2.2 D Option A).
- **Hierarchical (tree-shaped) Writer prompt** — Writer는 트리 구조
  자체를 prompt context로 받지 않는다. 평탄화된 `FlattenedClaim`(REQ-010
  참조)만 받으며 트리 lineage는 `lineage_path` string array로 표현된다.
  Hierarchical prompt experimentation은 본 SPEC 범위 밖.
- **GitHub Issue tracking on this SPEC** (`issue_number: 19`) — 본
  SPEC은 frontmatter에 issue_number 19를 보유하고 있으며, GitHub Issue
  19의 운영 관리(예: 자동 close, label 동기화 자동화)는 본 SPEC의 책임
  범위 밖이다.

---

## 5. Acceptance Scenarios

상세 Given/When/Then scenarios는 `.moai/specs/SPEC-DEEP-003/acceptance.md`
에 정의되어 있다. 본 절은 인덱스를 제공한다.

### Scenario 5.1 — 정상 트리 확장 (default config: breadth=4, depth=3)

**Given** 사용자가 `/deep?mode=agents` 엔드포인트에 `{query: "양자컴퓨터의
신약 개발 응용 현황", tree: {breadth: 4, depth: 3}}` body로 요청.
fanout adapter 4개 등록, 각 노드 평균 LLM 응답 시간 5초.

**When** 트리 익스플로러가 expand를 수행.

**Then** 응답 HTTP 200, `Status=success`, `total_nodes=1+4+16+64=85`,
`max_depth_reached=3`, end-to-end wall-clock ≤ 240초(NFR-DEEP3-002 p95).
Prometheus `usearch_deep_tree_node_expand_seconds` histogram이 85회
observation 기록. OTel trace tree depth = 3.

Coverage: REQ-001, 003, 004, 011a, 012, 013; NFR-DEEP3-001, 002, 005, 006.

### Scenario 5.2 — 구조적 cap 초과로 expand 거부

**Given** 사용자가 `breadth=9, depth=3`(`breadth > 8` 범위 위반)로
요청.

**When** 트리 익스플로러가 input validation 수행.

**Then** HTTP 400 응답, body `{"error": "invalid_tree_config", "detail":
"breadth=9 exceeds maximum 8", "breadth": 9, "depth": 3}`. 트리 expand
시작되지 않음 — `tree.json` 파일 미생성, Postgres row 미생성.

Coverage: REQ-002.

### Scenario 5.3 — 토큰 budget 소진 mid-tree → 부분 트리 반환 + Writer 가용 + 동시 race-free

**Given** 사용자가 `breadth=4, depth=3`로 요청. `DEEP_TREE_TOKEN_BUDGET=
20000` (default 60000보다 낮은 환경 설정). 각 노드 평균 2000 토큰 소비
가정(85 nodes × 2000 = 170K 토큰, budget 8.5x 초과). Sibling 노드들은
errgroup으로 동시 dispatch.

**When** 트리 익스플로러가 depth=2까지 expand를 진행하다 budget
exhaustion 발생. REQ-DEEP3-006 reservation lock + REQ-DEEP3-007
atomic release semantics가 동시 sibling pre-check를 serialize.

**Then** 응답 HTTP 200(degraded success), body의 부분 트리. Writer가
부분 트리에서 `FlattenedClaim`을 받아 답변 생성. Prometheus
`usearch_deep_tree_total_tokens{outcome="budget_exceeded"}` += 1.
**Critical invariant**: `final_total = sum(node.TokensUsed for completed
nodes) ≤ DEEP_TREE_TOKEN_BUDGET = 20000` — 동시 sibling race 하에서도
overshoot 0(reservation lock으로 atomic enforcement).

Coverage: REQ-006, 007, 008, 010; NFR-003.

### Scenario 5.4 — 인용 lineage가 모든 leaf claim에서 root까지 추적 가능

**Given** 정상 트리 확장 완료(Scenario 5.1과 동일 조건).

**When** Writer가 받은 `TreeResult.flattened_claims`를 inspection.

**Then** 모든 `FlattenedClaim.lineage_path`는 (a) root query 문자열
prefix를 포함, (b) source_node가 depth=k인 경우 `len(lineage_path) ==
k+1`, (c) `source_node_id`로 트리를 reverse-traverse하면 root에 도달
가능. Property test(hypothesis-go)가 100개 random tree generation에
대해 invariant 검증.

Coverage: REQ-009a (prompt context), REQ-009b (citation disjointness), 010.

### Scenario 5.5 — Sidecar 크래시 시 tree.json 부분 복원

**Given** 트리 expand가 depth=2 진행 중 server SIGTERM 수신.

**When** Server가 graceful shutdown 수행. 이후 audit script가
`.moai/runs/{run_id}/tree.json` 로드.

**Then** 로드된 트리는 valid JSON. `Status != NodeStatusComplete` 노드는
`NodeStatusFailed`로 reclassify된 상태로 readable. Postgres `deep_runs`
row는 `status="failed", completed_at=<crash_time>`로 finalize.

Coverage: REQ-011a (flush), REQ-011b (reclassify); NFR-DEEP3-008.

### Edge Cases

- **Scenario 5.6 (edge): breadth=0 OR depth=0 fallback** — `breadth=0`
  또는 `depth=0` 지정 시 single-shot mode fallback (REQ-005). 두 입력
  모두 invalid-range가 아닌 fallback signal로 해석되어 HTTP 200 응답을
  받는다. Fallback signal은 응답 헤더 `X-Deep-Tree-Fallback`을 통해
  out-of-band emit; response body는 DEEP-002 single-shot contract와
  byte-identical하게 유지(`TestFallbackHeaderEmittedAndBodyUnchanged`로
  검증). 각각의 sub-scenario(breadth=0, depth=0, 두 값 동시) 검증.
- **Scenario 5.7 (edge): depth=1 single-level tree** — `depth=1` 지정
  시 root + breadth개 leaf만 존재하는 평탄 트리 — 정상 처리, lineage
  depth=1.

---

## 6. Dependencies & Blocks

### 6.1 depends_on (구현 전제)

- **SPEC-DEEP-001** (implemented): STORM sidecar 패턴 차용 — Python
  sidecar에 신규 endpoint 추가 시 `services/researcher/`의 FastAPI +
  LiteLLM gateway 패턴 재사용.
- **SPEC-DEEP-002** (draft v0.1.1): Researcher 에이전트 hook point.
  본 SPEC은 DEEP-002의 `internal/deepagent/agents.go` Researcher
  함수가 tree mode flag를 받을 때 본 SPEC의 `tree.go`를 호출한다.
- **SPEC-SYN-001**: synthesis sidecar 기존 FastAPI 패턴 — 신규
  `/decompose_query` endpoint 추가 시 동일 패턴.
- **SPEC-SYN-004**: SSE wire format — 본 SPEC은 신규 이벤트 추가하지
  않으나 DEEP-002의 기존 이벤트로 Researcher 단계 wrapping 보장.
- **SPEC-LLM-001**: `llm.Client.Complete()`를 통한 LLM 호출 contract
  — 본 SPEC의 sub-query 생성도 이 client 경유.
- **SPEC-OBS-001**: Prometheus cardinality safety(NFR-OBS-002) +
  OTel tracing(NFR-OBS-003) — NFR-DEEP3-005, 006이 enforce.
- **SPEC-FAN-001**: `fanout.Dispatch()` — 각 노드에서 검색 호출.
- **SPEC-CORE-001**: `NormalizedDoc` shape — `Node.Citations`의 doc
  reference 형식.

### 6.2 blocks (후속 SPEC이 본 SPEC을 필요로 함)

- **SPEC-DEEP-004**: per-user/per-day cost guard. DEEP-004는 본 SPEC을
  capacity planning(cap dimension calibration) 상류 의존성 앵커로
  참조한다. 구체적인 메트릭/필드 소비(`Node.TokensUsed`,
  `usearch_deep_tree_total_tokens`)는 본 SPEC이 가용함을 문서화하나,
  구속력 있는 인터페이스 합의는 DEEP-004 구현 단계에서 양 SPEC 저자가
  공동 확정한다(본 SPEC은 producer 책임, DEEP-004는 consumer 책임).

---

## 7. Files to Create / Modify

| Marker | Path | Purpose |
|--------|------|---------|
| [NEW] | `internal/deepagent/tree.go` | Tree orchestrator (BFS expand loop, errgroup pool, Status transition) |
| [NEW] | `internal/deepagent/budget.go` | 3-dimension budget tracker (token, timeout, structural) |
| [NEW] | `internal/deepagent/persistence.go` | tree.json atomic flush + Postgres summary insert |
| [NEW] | `internal/deepagent/tree_types.go` | `Node`, `NodeStatus`, `NodeCitation`, `NodeClaim`, `TreeResult`, `FlattenedClaim` types |
| [NEW] | `internal/deepagent/tree_metrics.go` | Prometheus histogram + counter registration |
| [NEW] | `internal/deepagent/tree_test.go` | Unit tests for tree orchestration (happy path, partial budget, depth=1 edge) |
| [NEW] | `internal/deepagent/budget_test.go` | Budget pre-check unit tests |
| [NEW] | `internal/deepagent/persistence_test.go` | Tree.json round-trip + Postgres mock tests |
| [NEW] | `tests/integration/deep_tree_test.go` | End-to-end integration test via `httptest` + stubbed sidecar |
| [NEW] | `services/researcher/src/researcher/deep_tree.py` | `POST /decompose_query` endpoint(thin LLM wrapper, LiteLLM gateway reuse) |
| [NEW] | `services/researcher/tests/test_deep_tree.py` | Python endpoint unit tests |
| [NEW] | `.moai/config/sections/deep.yaml` | Tree mode config(`tree.enabled`, `tree.breadth`, `tree.depth`, budget defaults, `pricing.{model}` map for REQ-DEEP3-013 cost lookup) |
| [NEW] | `deploy/postgres/migrations/0002_deep_runs.up.sql` | Postgres `deep_runs` 테이블 schema (golang-migrate format; §8 OQ-1 RESOLVED 참조) |
| [NEW] | `deploy/postgres/migrations/0002_deep_runs.down.sql` | Rollback migration (DROP TABLE deep_runs) |
| [MODIFY] | `internal/deepagent/agents.go` (DEEP-002) | Researcher 에이전트가 tree mode flag 평가하여 `tree.go` 호출 분기 추가 |
| [MODIFY] | `internal/deepagent/config.go` (DEEP-002) | `DEEP_TREE_*` env-var loader 추가 |
| [MODIFY] | `internal/obs/metrics/metrics.go` | `registerDeepTree(pr)` helper 등록 |
| [MODIFY] | `internal/obs/obs.go` | `obs.DeepTreeNodeExpand`, `obs.DeepTreeTotalTokens` re-export |
| [MODIFY] | `services/researcher/src/researcher/app.py` | 신규 `/decompose_query` route 등록 |
| [MODIFY] | `.env.example` | `DEEP_TREE_*` env-var 문서화 |
| [EXISTING — UNCHANGED] | `internal/fanout/` (FAN-001) | Read-only consumer |
| [EXISTING — UNCHANGED] | `internal/llm/client.go` (LLM-001) | Read-only consumer |
| [EXISTING — UNCHANGED] | `cmd/usearch-api/handlers/deep_agents_handler.go` (DEEP-002) | 변경 불필요 — Researcher 에이전트 내부에서 트리 mode 분기 |

---

## 8. Open Questions

- **OQ-1 (Migration tooling) — RESOLVED 2026-05-21**: Repo 표준 마이그레이션
  디렉토리는 `deploy/postgres/migrations/`(기존 `0001_create_docs.sql`
  precedent). 결정 사항:
  - Tool: **`golang-migrate/migrate`** (CLI + Go embed, file pair `*.up.sql`/`*.down.sql`)
  - Sequence: **`0002`** (next after existing `0001_create_docs.sql`)
  - File pair: `0002_deep_runs.up.sql` + `0002_deep_runs.down.sql`
  - 기존 `0001_create_docs.sql` adoption (rename to `0001_create_docs.up.sql` + create
    matching `down.sql`)은 별도 운영 task로 분리하며 본 SPEC 범위 밖. T-D-009는 신규
    `0002_*` 파일만 작성하면 unblock.

---

## 9. Configuration / Environment Variables

| Env Var | Default | Purpose | Owner |
|---------|---------|---------|-------|
| `DEEP_TREE_ENABLED` | `false` | Default behavior — tree mode opt-in per request | REQ-DEEP3-001 |
| `DEEP_TREE_DEFAULT_BREADTH` | `4` | Default breadth (override-able per request) | REQ-DEEP3-001, 002 |
| `DEEP_TREE_DEFAULT_DEPTH` | `3` | Default depth (override-able per request) | REQ-DEEP3-001, 002 |
| `DEEP_TREE_TOKEN_BUDGET` | `60000` | Total LLM token budget per tree | REQ-DEEP3-006; NFR-DEEP3-003 |
| `DEEP_TREE_ROOT_TOKEN_ESTIMATE` | `5000` | Root node pre-check seed (parent.TokensUsed가 없는 root 케이스에서 사용) | REQ-DEEP3-006 |
| `DEEP_TREE_NODE_TIMEOUT_MS` | `30000` | Per-node expand timeout | REQ-DEEP3-004; NFR-DEEP3-001 |
| `DEEP_TREE_DECOMPOSE_MODEL` | `claude-3-5-haiku-20241022` | LiteLLM model alias for sub-query generation | REQ-DEEP3-003 |
| `DEEP_TREE_PERSISTENCE_DIR` | `.moai/runs` | tree.json output directory | REQ-DEEP3-011a |

---

## 10. References

### 10.1 Internal SPEC Documents

- `.moai/specs/SPEC-DEEP-001/spec.md` — STORM sidecar 패턴 reference
- `.moai/specs/SPEC-DEEP-002/spec.md` — Multi-agent pipeline (본 SPEC의
  consumer)
- `.moai/specs/SPEC-DEEP-003/research.md` — Phase 0.5 deep research
- `.moai/specs/SPEC-SYN-001/spec.md` — synthesis sidecar 패턴
- `.moai/specs/SPEC-SYN-004/spec.md` — SSE wire format(본 SPEC은 신규
  이벤트 미추가)
- `.moai/specs/SPEC-FAN-001/spec.md` — fanout dispatch contract
- `.moai/specs/SPEC-LLM-001/spec.md` — LiteLLM client contract
- `.moai/specs/SPEC-OBS-001/spec.md` — observability cardinality safety
- `.moai/specs/SPEC-CORE-001/spec.md` — NormalizedDoc shape
- `.moai/project/roadmap.md` §M5 — SPEC-DEEP-003 row

### 10.2 External (verify URLs via WebFetch in Run phase)

- `https://github.com/assafelovic/gpt-researcher` — upstream deep-research
  tree convention (prompt design 영감)

### 10.3 Companion Artifacts

- `.moai/specs/SPEC-DEEP-003/plan.md` — TDD task sequence + MX tag plan
- `.moai/specs/SPEC-DEEP-003/acceptance.md` — Given/When/Then scenarios
- `.moai/specs/SPEC-DEEP-003/spec-compact.md` — compact view

---

*End of SPEC-DEEP-003 v0.1.1 (draft).*
