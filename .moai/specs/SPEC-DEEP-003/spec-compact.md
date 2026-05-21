# SPEC-DEEP-003 (Compact)

Version: 0.1.2
Status: planned
Owner: expert-backend
Milestone: M5 — /deep multi-agent
Methodology: tdd, coverage_target: 85
depends_on: SPEC-DEEP-001, SPEC-DEEP-002, SPEC-SYN-001, SPEC-SYN-004, SPEC-LLM-001, SPEC-OBS-001, SPEC-FAN-001, SPEC-CORE-001
blocks: SPEC-DEEP-004

Title: Tree exploration for /deep multi-agent (configurable breadth/depth, budget cap)

---

## Functional Requirements

### Tree Initialization

- **REQ-DEEP3-001** (Event): WHEN 요청 body에 `{tree: {breadth, depth}}` OR `deep.yaml`의 `tree.enabled: true`인 경우, root Node(`Depth=0, ParentID="", Query=request.query`)를 SHALL 생성한다. ID는 deterministic(`hash(run_id || "root")`).
- **REQ-DEEP3-002** (Ubiquitous): `breadth ∈ [1, 8]` AND `depth ∈ [1, 5]` 범위 검증, 위반 시 HTTP 400 `{"error": "invalid_tree_config", ...}` 반환. `breadth=0` AND `depth=0` 입력은 본 범위 위반 대상이 아닌 fallback signal로 REQ-DEEP3-005가 처리.

### Node Expansion

- **REQ-DEEP3-003** (State): WHILE root이 pending인 동안 BFS expand 수행 — (a) Status transition, (b) `fanout.Dispatch`, (c) `/decompose_query` LLM call로 breadth개 sub-query, (d) child Node 생성, (e) `Depth < depth` 시 재귀. `Depth == depth` 노드는 sub-query 생성 skip(leaf).
- **REQ-DEEP3-004** (State): WHILE expand 중 동일 depth 노드를 `errgroup.WithContext` parallel pool로 expand. Pool size = breadth. 각 노드 `context.WithTimeout(DEEP_TREE_NODE_TIMEOUT_MS)` 격리. BFS invariant(depth N+1은 depth N 완료 후).
- **REQ-DEEP3-005** (Conditional): IF `breadth=0` OR `depth=0` 지정 시 DEEP-002 REQ-005 single-shot fallback (HTTP 200, invalid-range 아님). Fallback signal은 응답 header `X-Deep-Tree-Fallback: breadth_zero | depth_zero | disabled`로 out-of-band emit. 응답 body는 DEEP-002 contract와 byte-identical 유지(body mutation 금지). 동시 지정 시 header value `breadth_zero` 우선.

### Budget Enforcement

- **REQ-DEEP3-006** (Event): WHEN pre-check 시점에 (a) `tree.TotalTokensUsed + tree.TotalReservedTokens + estimated_next_cost > budget` OR (b) structural cap 초과 시 frontier 노드를 `NodeStatusBudgetExceeded`로 mark. Pre-check는 reservation lock(exclusive mutex)에서 (read + decision + reservation) atomic 수행. Pre-check success 시 `estimated_next_cost`를 `Node.ReservedTokens`에 기록 + `tree.TotalReservedTokens` increment. Sibling 동시 dispatch에서 race-free. estimated cost = `parent.TokensUsed * breadth * 1.25`. Root node는 `DEEP_TREE_ROOT_TOKEN_ESTIMATE`(default 5000) seed로 pre-check.
- **REQ-DEEP3-007** (State): WHILE expand 중 매 LLM 호출 후 `node.TokensUsed` accumulate. Post-LLM accumulation은 reservation lock에서 (a) `tree.TotalTokensUsed += delta` AND (b) `tree.TotalReservedTokens -= (ReservedTokens - TokensUsed)` 두 동작을 single atomic critical section에서 수행 — actual 가산 + reservation 해제 동시 처리. fanout cost는 미포함.
- **REQ-DEEP3-008** (Conditional): IF root complete + frontier budget_exceeded인 경우 부분 트리 → Writer. HTTP 200(degraded). 응답 `usage: {budget_exceeded: true, ...}`.

### Citation Lineage

- **REQ-DEEP3-009a** (Ubiquitous): 모든 노드 expand 시 `{root_query, parent_query, parent_evidence_summary}`를 `/decompose_query` request에 SHALL 포함 (prompt context flow).
- **REQ-DEEP3-009b** (Ubiquitous): `Node.Citations[i]`는 해당 노드의 fanout 결과만 SHALL 포함. 슬라이스 identity disjoint(타 노드 슬라이스 cross-reference/inherit 금지). 동일 doc_id가 sibling 노드 슬라이스에 우연히 등장하는 것은 허용.
- **REQ-DEEP3-010** (Ubiquitous): 트리 완료 후 `TreeResult{root_query, total_nodes, max_depth_reached, status, flattened_claims, citations}`를 Writer로 전달. `FlattenedClaim{text, markers, lineage_path: []string, source_node_id}`.

### Persistence & Observability

- **REQ-DEEP3-011a** (State): WHILE expand 중 매 depth-level join 직후 + 노드 transition 시점에 `.moai/runs/{run_id}/tree.json` atomic flush. 완료 후 Postgres `deep_runs` summary row insert(`{..., total_cost_usd, ...}` 포함; `total_cost_usd`는 REQ-DEEP3-013의 per-node `Node.CostUSD` 합산으로 derive). Migration 디렉토리: `deploy/postgres/migrations/`.
- **REQ-DEEP3-011b** (Event): WHEN persistence layer가 reload mode로 invoke(SPEC-DEEP-004 audit entry point)되는 경우 `Status ∈ {Pending, Expanding}` 노드를 in-memory에서 `Failed`로 reclassify, 트리를 read-only로 반환. Disk tree.json은 불변.
- **REQ-DEEP3-012** (Optional): WHERE OTel 활성, 매 노드 span 발행 + parent linkage. Prometheus histogram `usearch_deep_tree_node_expand_seconds{depth, outcome}`(cardinality 6×3=18) + counter `usearch_deep_tree_total_tokens{outcome}`(cardinality 2) emit.

### Cost Accounting

- **REQ-DEEP3-013** (Ubiquitous): 매 노드 expand 완료 시 `Node.CostUSD = Node.TokensUsed * model_price_per_token(DEEP_TREE_DECOMPOSE_MODEL)` SHALL 계산. `model_price_per_token`은 `.moai/config/sections/deep.yaml` `pricing.{model}` map에서 lookup(미등록 시 0.0). `tree.TotalCostUSD = sum(node.CostUSD for completed)` derive.

---

## Non-Functional Requirements

- **NFR-DEEP3-001**: 단일 노드 expand p95 ≤ 30s
- **NFR-DEEP3-002**: 트리 end-to-end p95 ≤ 4min (default config)
- **NFR-DEEP3-003**: 토큰 budget enforcement — `sum(node.TokensUsed) ≤ DEEP_TREE_TOKEN_BUDGET`. REQ-DEEP3-006 reservation lock + REQ-DEEP3-007 atomic release semantics가 sibling 동시 race-free 보장.
- **NFR-DEEP3-004**: structural cap — 노드 수 ≤ `1 + sum(breadth^i, i=1..depth)`
- **NFR-DEEP3-005**: Prometheus cardinality bounded — `depth ∈ {0..5}`, `outcome ∈ {success, failed, budget_exceeded}`
- **NFR-DEEP3-006**: OTel span linkage — child가 parent의 span_id 참조
- **NFR-DEEP3-007**: tree.json size ≤ 200KB compressed (NFR-003+NFR-004 inherently bound; 별도 runtime size check 없음)
- **NFR-DEEP3-008**: crash recovery — partial tree readable, `Status != Complete` 노드를 `Failed`로 reclassify
- **NFR-DEEP3-009**: in-memory tree state ≤ 100MB worst-case(`breadth=8, depth=5`). 매 depth-level 완료 시점 측정; 초과 시 frontier truncation으로 복원.

---

## Exclusions

- Real-time UI tree visualization (M7 SPEC-UI-001)
- User-editable tree (interactive prune/manual add 불가)
- Resume of crashed tree (no resume — fresh run_id only)
- Per-node/per-user/per-day cost cap, prompt-cache reuse (DEEP-004 책임). 본 SPEC은 `Node.TokensUsed`, `Node.CostUSD`, metrics를 노출만; DEEP-004와의 구속력 있는 인터페이스 합의는 DEEP-004 구현 단계에서 공동 확정
- Cross-tree query deduplication (M6 SPEC-IDX-005)
- Automatic depth/breadth tuning by ML (M8 SPEC-EVAL-002)
- gpt-researcher PyPI package integration (in-house implementation)
- Hierarchical Writer prompt (flattened-with-lineage only)
- GitHub Issue 19 운영 관리 자동화 (issue_number: 19; frontmatter 보유)

---

## Acceptance Scenarios

- §5.1 Happy path (default breadth=4, depth=3, ≤240s, 85 nodes) — REQ-001/003/004/011a/012/013
- §5.2 Invalid breadth=9 → HTTP 400 — REQ-002
- §5.3 Budget exhaustion + reservation race-free (deterministic `final_total ≤ budget_cap`) — REQ-006/007/008/010, NFR-003
- §5.4 Lineage traceability + citation disjointness — property test 100 iterations — REQ-009a/009b/010
- §5.5 Crash recovery — atomic flush per depth (011a) + reload reclassify (011b) — REQ-011a/011b, NFR-008
- §5.6 (edge) breadth=0 OR depth=0 fallback — header `X-Deep-Tree-Fallback` emit, body byte-identical — REQ-005
- §5.7 (edge) depth=1 single-level — REQ-003/009a/009b
- §5.8 (NFR gate) in-memory tree ≤ 100MB worst-case — NFR-009

---

## Files to Create / Modify

### [NEW]

- `internal/deepagent/tree.go` — BFS orchestrator
- `internal/deepagent/budget.go` — 3-cap budget tracker
- `internal/deepagent/persistence.go` — tree.json + Postgres
- `internal/deepagent/tree_types.go` — Node, TreeResult, FlattenedClaim types
- `internal/deepagent/tree_metrics.go` — Prometheus collectors
- `internal/deepagent/tree_test.go`
- `internal/deepagent/budget_test.go`
- `internal/deepagent/persistence_test.go`
- `internal/deepagent/researcher_http.go` — Go HTTP client for `/decompose_query`
- `internal/deepagent/researcher_http_test.go`
- `tests/integration/deep_tree_test.go` — end-to-end integration
- `services/researcher/src/researcher/deep_tree.py` — Python `/decompose_query` endpoint
- `services/researcher/tests/test_deep_tree.py`
- `.moai/config/sections/deep.yaml` — tree mode config
- `deploy/postgres/migrations/0002_deep_runs.up.sql` + `0002_deep_runs.down.sql` — Postgres schema (golang-migrate, §8 OQ-1 RESOLVED)

### [MODIFY]

- `internal/deepagent/agents.go` (DEEP-002) — Researcher tree mode branching
- `internal/deepagent/config.go` (DEEP-002) — `DEEP_TREE_*` env-var loader
- `internal/obs/metrics/metrics.go` — `registerDeepTree(pr)` helper
- `internal/obs/obs.go` — re-export DeepTreeNodeExpand, DeepTreeTotalTokens
- `services/researcher/src/researcher/app.py` — route registration
- `.env.example` — `DEEP_TREE_*` env-var documentation

### [EXISTING — UNCHANGED]

- `internal/fanout/` (FAN-001) — read-only consumer
- `internal/llm/client.go` (LLM-001) — read-only consumer
- `cmd/usearch-api/handlers/deep_agents_handler.go` (DEEP-002) — 변경 불필요

---

## Configuration / Env Variables

- `DEEP_TREE_ENABLED` (default `false`)
- `DEEP_TREE_DEFAULT_BREADTH` (default `4`)
- `DEEP_TREE_DEFAULT_DEPTH` (default `3`)
- `DEEP_TREE_TOKEN_BUDGET` (default `60000`)
- `DEEP_TREE_ROOT_TOKEN_ESTIMATE` (default `5000` — root node pre-check seed)
- `DEEP_TREE_NODE_TIMEOUT_MS` (default `30000`)
- `DEEP_TREE_DECOMPOSE_MODEL` (default `claude-3-5-haiku-20241022`)
- `DEEP_TREE_PERSISTENCE_DIR` (default `.moai/runs`)

---

*End of spec-compact.md.*
