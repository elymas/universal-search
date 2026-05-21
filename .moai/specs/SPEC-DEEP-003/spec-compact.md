# SPEC-DEEP-003 (Compact)

Version: 0.1.1
Status: draft
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
- **REQ-DEEP3-005** (Conditional): IF `breadth=0` OR `depth=0` 지정 시 DEEP-002 REQ-005 single-shot fallback (HTTP 200, invalid-range 아님). 응답에 `{tree: {disabled: true, mode: "single-shot-fallback", reason: "breadth_zero"|"depth_zero"}}` metadata. 동시 지정 시 `reason: "breadth_zero"` 우선.

### Budget Enforcement

- **REQ-DEEP3-006** (Event): WHEN pre-check 시점에 (a) token budget 초과 OR (b) structural cap 초과 시 frontier 노드를 `NodeStatusBudgetExceeded`로 mark. estimated cost = `parent.TokensUsed * breadth * 1.25`. Root node는 `DEEP_TREE_ROOT_TOKEN_ESTIMATE`(default 5000) seed로 pre-check.
- **REQ-DEEP3-007** (State): WHILE expand 중 매 LLM 호출 후 `node.TokensUsed` accumulate, `tree.TotalTokensUsed` atomic increment. fanout cost는 미포함.
- **REQ-DEEP3-008** (Conditional): IF root complete + frontier budget_exceeded인 경우 부분 트리 → Writer. HTTP 200(degraded). 응답 `usage: {budget_exceeded: true, ...}`.

### Citation Lineage

- **REQ-DEEP3-009** (Ubiquitous): 모든 노드 expand 시 `{root_query, parent_query, parent_evidence_summary}`를 `/decompose_query` request에 SHALL 포함. `Node.Citations[i]`는 해당 노드의 fanout 결과만 (타 노드 cross-reference 금지).
- **REQ-DEEP3-010** (Ubiquitous): 트리 완료 후 `TreeResult{root_query, total_nodes, max_depth_reached, status, flattened_claims, citations}`를 Writer로 전달. `FlattenedClaim{text, markers, lineage_path: []string, source_node_id}`.

### Persistence & Observability

- **REQ-DEEP3-011** (State): WHILE expand 중 매 노드 transition 시 `.moai/runs/{run_id}/tree.json` atomic flush. 완료 후 Postgres `deep_runs` summary row insert (`deploy/postgres/migrations/`). Reload 경로(DEEP-004 audit)에서 `Status ∈ {Pending, Expanding}` 노드를 `Failed`로 reclassify(in-memory, read-only).
- **REQ-DEEP3-012** (Optional): WHERE OTel 활성, 매 노드 span 발행 + parent linkage. Prometheus histogram `usearch_deep_tree_node_expand_seconds{depth, outcome}`(cardinality 6×3=18) + counter `usearch_deep_tree_total_tokens{outcome}`(cardinality 2) emit.

---

## Non-Functional Requirements

- **NFR-DEEP3-001**: 단일 노드 expand p95 ≤ 30s
- **NFR-DEEP3-002**: 트리 end-to-end p95 ≤ 4min (default config)
- **NFR-DEEP3-003**: 토큰 budget enforcement — `sum(node.TokensUsed) ≤ DEEP_TREE_TOKEN_BUDGET`
- **NFR-DEEP3-004**: structural cap — 노드 수 ≤ `1 + sum(breadth^i, i=1..depth)`
- **NFR-DEEP3-005**: Prometheus cardinality bounded — `depth ∈ {0..5}`, `outcome ∈ {success, failed, budget_exceeded}`
- **NFR-DEEP3-006**: OTel span linkage — child가 parent의 span_id 참조
- **NFR-DEEP3-007**: tree.json size ≤ 200KB compressed
- **NFR-DEEP3-008**: crash recovery — partial tree readable, `Status != Complete` 노드를 `Failed`로 reclassify

---

## Exclusions

- Real-time UI tree visualization (M7 SPEC-UI-001)
- User-editable tree (interactive prune/manual add 불가)
- Resume of crashed tree (no resume — fresh run_id only)
- Per-node cost cap (DEEP-004 책임)
- Cross-tree query deduplication (M6 SPEC-IDX-005)
- Automatic depth/breadth tuning by ML (M8 SPEC-EVAL-002)
- gpt-researcher PyPI package integration (in-house implementation)
- Hierarchical Writer prompt (flattened-with-lineage only)
- GitHub Issue tracking on this SPEC (issue_number: 0)

---

## Acceptance Scenarios

- §5.1 Happy path (default breadth=4, depth=3, ≤240s, 85 nodes) — REQ-001/003/004/011/012
- §5.2 Invalid breadth=9 → HTTP 400 — REQ-002
- §5.3 Budget exhaustion mid-tree → 부분 트리, HTTP 200 — REQ-006/007/008/010
- §5.4 Lineage traceability — property test 100 iterations — REQ-009/010
- §5.5 Crash recovery — partial tree.json reclassify — REQ-011, NFR-008
- §5.6 (edge) breadth=0 OR depth=0 fallback (HTTP 200, single-shot-fallback mode) — REQ-005
- §5.7 (edge) depth=1 single-level — REQ-003/009

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
- `deploy/postgres/migrations/{NN}_deep_runs.up.sql` + `down.sql` — Postgres schema (sequence/tool 핀은 §8 Open Questions)

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
