# SPEC-DEEP-002 Compact Summary

Version: 0.1.2
Status: planned

One-page distillation of `.moai/specs/SPEC-DEEP-002/spec.md` for
loading-into-context efficiency.

---

## Identity

- **ID**: SPEC-DEEP-002
- **Title**: Multi-agent /deep pipeline (Researcher/Reviewer/Writer/Verifier)
- **Status**: planned
- **Priority**: P0
- **Milestone**: M5 — /deep multi-agent
- **Methodology**: TDD (RED-GREEN-REFACTOR), coverage ≥85%
- **Harness**: standard
- **Owner**: expert-backend
- **Depends on**: SPEC-DEEP-001, SPEC-SYN-002, SPEC-SYN-004,
  SPEC-FAN-001, SPEC-LLM-001, SPEC-CORE-001, SPEC-OBS-001, SPEC-IR-001
- **Blocks**: SPEC-DEEP-003, SPEC-DEEP-004
- **Issue**: 0 (no GH tracking)

---

## EARS Requirements (15)

### Endpoint Module

- **REQ-DEEP2-001** (Ubiquitous): `cmd/usearch-api`는 `POST /deep`에
  `?mode=` 쿼리 파라미터를 도입 SHALL 한다. `mode=agents` 값은 신규
  `deep_agents_handler`로 라우팅 SHALL 한다. 요청 본문 `{request_id,
  query, lang}`를 수용 SHALL 하되 `docs[]` 필드는 제거. 응답은 SSE
  기본 또는 `?stream=false` 버퍼드 JSON 형식 중 하나로 emit SHALL.
- **REQ-DEEP2-010** (Optional, P1): WHERE `?stream=false`이거나
  `Accept`이 `text/event-stream`을 advertise하지 않을 때, 핸들러는
  buffered JSON 응답으로 fallback. SSE writer/heartbeat 미인스턴스화.
- **REQ-DEEP2-011** (Ubiquitous): `mode=storm` 경로는 SPEC-DEEP-001
  동작을 **schema-identically AND semantically equivalently** 보존
  (P-M6: same event types/order, same field names/types; non-deterministic
  fields like request_id/timestamps/durations/costs MAY differ in value).
  두 모드 간 mutable global state 없음. `?mode=` absent → default `storm`
  (backward compat).

### Pipeline Module

- **REQ-DEEP2-002** (State-Driven): WHILE `mode=agents` 요청 진행 중,
  orchestrator는 Researcher → Reviewer → Writer → Verifier를 순차
  실행. 매 agent 호출 직전 `ctx.Err()` 확인, cancel 시 `pipeline_cancelled`
  SSE emit 후 종료. Reviewer/Writer는 fanout 미호출.
- **REQ-DEEP2-003** (Event-Driven): WHEN Verifier가 `uncited_sentences_count
  > 0`를 반환 AND Writer 시도 횟수 < `MaxRetries + 1` (default 3), THEN
  orchestrator가 Writer를 재호출하며 `retry_started` SSE emit.
  `usearch_deep_agent_retries_total{agent="writer"}` += 1 per retry.
  ONLY Verifier rejection만 Writer retry 트리거 — 다른 agent의 에러는
  REQ-DEEP2-009b-SSE / REQ-DEEP2-009b-Buffered로 surface.
- **REQ-DEEP2-005** (Ubiquitous): Researcher는 `fanout.Dispatch()`를
  파이프라인당 정확히 1회 호출, `Result.Docs []NormalizedDoc`를
  downstream으로 전달. 다른 retrieval 메커니즘 사용 금지.
- **REQ-DEEP2-009a-SSE** (Unwanted): IF Writer가 max 3회 호출됨 AND
  Verifier가 최종 시도에서도 reject AND SSE 활성 (headers already
  flushed), THEN handler emits terminal `pipeline_failed{
  failed_agent:"writer", reason:"verifier_rejection_exhausted",
  attempts, uncited_count, retry_count}` SSE event. HTTP 200 stays
  on wire (SSE forbids retroactive status change).
  `usearch_deep_agent_verifier_gate_results_total{result="fail_uncited"}`
  은 매 rejection마다 증가, `usearch_deep_outcomes_total{outcome="error_pipeline_failed"}`
  은 정확히 1회 증가.
- **REQ-DEEP2-009a-Buffered** (Unwanted): IF Writer max-retry exhaustion
  AND response is buffered (headers NOT yet flushed), THEN handler
  returns HTTP 503 + JSON body `{error: "pipeline_failed", detail,
  uncited_count, attempts, retry_count}`. 동일 Prometheus counter 증감.
- **REQ-DEEP2-009b-SSE** (Unwanted): IF Researcher/Reviewer/Verifier
  자체가 비복구성 에러 (LLM 업스트림 실패, timeout, panic) 반환 AND
  SSE 활성, THEN handler emits terminal `pipeline_failed{failed_agent,
  reason}` SSE event. HTTP 200 stays. Writer retry 트리거 금지.
  `usearch_deep_outcomes_total{outcome="error_pipeline_failed"}` += 1.
- **REQ-DEEP2-009b-Buffered** (Unwanted): IF agent (non-Verifier-verdict)
  error AND response is buffered, THEN HTTP 503 + JSON body `{error:
  "pipeline_failed", failed_agent: <name>, detail}`. Writer retry 금지.
  동일 Prometheus counter 증감.
- **REQ-DEEP2-012** (Unwanted): IF `fanout.Dispatch()`가 빈
  `Result.Docs` 반환, THEN Researcher가 자체 LLM 호출을 **SKIP**하고
  `IsEmpty:true` 즉시 반환 (P-M3: extract할 claim 없음 — 비용/지연
  절감). Reviewer/Writer/Verifier 미호출, HTTP 200 with `{final:
  {sections: [], citations: []}, agent_log: [{agent: "researcher",
  outcome: "empty_corpus"}]}`. `usearch_deep_outcomes_total{outcome="empty_corpus"}`
  += 1. **Histogram label vs SSE field naming clarification**: histogram
  outcome label은 bounded enum `{success, error}`이므로 `success`로
  관찰됨; SSE `agent_completed.outcome` JSON field는 cardinality 제약
  없으므로 `"empty_corpus"` 값 사용.

### LLM Routing Module

- **REQ-DEEP2-004** (Ubiquitous): 각 agent는 env-var로 해결된 LiteLLM
  model alias 사용: `DEEP_AGENT_RESEARCHER_MODEL`,
  `_REVIEWER_MODEL`, `_WRITER_MODEL`, `_VERIFIER_MODEL`. 중앙집중식
  loading은 `internal/deepagent/config.go`. 개별 agent의 `os.Getenv`
  직접 호출 금지. 모든 LLM 호출은 singleton `llm.Client` 경유.

### Verifier Gate Module

- **REQ-DEEP2-006** (Ubiquitous): Verifier는 신규 Go 함수
  `internal/synthesis.CheckFaithfulness(ctx, text, citations, docs)`
  를 호출, 이는 researcher 사이드카 신규 endpoint `POST
  /faithfulness_check`에 POST. Python endpoint는 기존 SYN-002
  `enforce_faithfulness` 로직 재사용. Verifier는 `UncitedSentencesCount
  == 0`일 때만 PASS (binary gate). 추가 차원 (coverage, coherence)
  점수화 없음. `usearch_deep_agent_verifier_gate_results_total{result}`
  counter는 invocation 당 1회 증가 (`pass`, `fail_uncited`, `fail_error`
  중 하나). `fail_error`는 모든 Verifier infra 실패 (timeout, sidecar
  5xx, transport error, wrapper error)를 cover.

### Streaming & Observability Module

- **REQ-DEEP2-007** (State-Driven): WHILE SSE 활성, 핸들러는 다음
  순서로 이벤트 emit: `agent_started{researcher}` →
  `agent_completed{researcher}` → ... (reviewer, writer, verifier) →
  Verifier rejection 시 `verifier_result{fail_uncited}` +
  `retry_started{writer}` 이후 재시도 → 최종 PASS 시 SYN-004 sequence
  (`section_start`, `sentence`, `section_done`, `done`). Failure는
  `pipeline_failed`, cancel은 `pipeline_cancelled` terminal event.
  모든 신규 payload에 `schema_version: 1`. Heartbeat는 handler 진입
  시점부터 유지.
- **REQ-DEEP2-008** (Ubiquitous): 3개 신규 Prometheus collectors
  등록: `usearch_deep_agent_duration_seconds{agent, outcome}`
  (outcome ∈ {success, error}, cardinality 4×2=8 — per-attempt
  histogram, retry 추적은 별도 counter), `usearch_deep_agent_retries_total{agent="writer"}`
  (cardinality 1 — retry 귀속 단일 source), `usearch_deep_agent_verifier_gate_results_total{result}`
  (cardinality 3, result ∈ {pass, fail_uncited, fail_error}). 모든
  라벨 값은 startup 시 pre-declared. 기존 `usearch_deep_outcomes_total{outcome}`에
  `empty_corpus`, `error_pipeline_failed` 추가.

---

## Non-Functional Requirements (4)

- **NFR-DEEP2-001 Performance**: 두 갈래 budget (P-M2): **(a) Go-side
  orchestration overhead p95 ≤ 1s** with mocked LLM + faithfulness
  (50-iteration statistical test in `internal/deepagent/orchestrator_test.go`);
  **(b) end-to-end prod p95 ≤ 60s** for 20-50 doc corpus on staging
  (Verifier passes 1st or 2nd attempt, default model tiers, verified
  via /moai sync smoke test — operational gate, not unit test).
  Max-retry exhaustion은 budget violation으로 간주하지 않음.
- **NFR-DEEP2-002 Cardinality Safety**: 모든 label 값은 bounded enum
  집합에서만 (`agent ∈ {researcher, reviewer, writer, verifier}`,
  `outcome ∈ {success, error}`, `result ∈ {pass, fail_uncited,
  fail_error}`), startup 시 pre-declared. Go enum-like type
  (`type Agent string` + `const`)로 컴파일 타임 enforce. User-derived
  string은 label position에 등장 금지.
- **NFR-DEEP2-003 Backward Compatibility**: DEEP-001 acceptance suite
  100% green 유지. `/deep?mode=storm` 응답 **schema-identical AND
  semantically equivalent** pre/post-DEEP-002 (P-M6: same event types/order,
  same field names/types per event; non-deterministic fields may differ
  in value). `?mode=` absent → default `storm`. 두 path 간 mutable
  global state 없음. `internal/deepagent` 가 `internal/deepreport` 미import.
- **NFR-DEEP2-004 Cost Visibility**: `usearch_deep_agent_duration_seconds`
  은 agent 라벨로 쿼리 가능. `usearch_deep_agent_retries_total`로 retry
  anomaly 탐지. `cost_usd` 필드는 모든 `agent_completed` payload에 포함.
  Per-user/daily 집계는 본 SPEC 범위 외.

---

## Acceptance Scenarios (8 main + 4 edge)

| Scenario | Coverage |
|----------|----------|
| 1. Happy path: Verifier PASS first attempt | REQ-001, 002, 005, 006, 007, 008 |
| 2. Retry path: reject iter 1, PASS iter 2 | REQ-003, 007, 008 |
| 3-SSE. Max-retry exhaustion + SSE active → terminal SSE, HTTP 200 stays | REQ-009a-SSE, 007, 008 |
| 3-Buffered. Max-retry exhaustion + buffered → HTTP 503 + JSON | REQ-009a-Buffered, 008 |
| 4. Context cancellation mid-pipeline | REQ-002, 007 |
| 5. Mode coexistence: `?mode=storm` unchanged | REQ-011 |
| 6. Buffered fallback (`?stream=false`) | REQ-010 |
| 7. Empty fanout corpus short-circuit | REQ-012 |
| 8. Researcher error aborts (non-Verifier failure) | REQ-009b-SSE, 009b-Buffered |
| Edge 1. Reviewer error no retry, abort | REQ-009b-SSE, 009b-Buffered |
| Edge 2. Verifier infra 5xx (fail_error) no retry, abort | REQ-009b-SSE, 009b-Buffered |
| Edge 3. Env-var absent → model defaults | REQ-004 |
| Edge 4. Concurrent `storm` + `agents` requests isolated | REQ-011, NFR-DEEP2-003 |

**Given-When-Then 상세 정의**: `.moai/specs/SPEC-DEEP-002/acceptance.md`
8개 main + 4개 edge cases.

---

## Files to Modify

**[NEW] Go**
- `internal/deepagent/`: orchestrator.go, agents.go, types.go,
  prompts.go, config.go, metrics.go, sse.go + 3 test files
- `internal/synthesis/faithfulness.go` + test
- `internal/streamsynth/agent_events.go` + test
- `internal/streamsynth/longform_source.go` — NEW `LongFormSource`
  interface (P-M7); both `deepreport.Report` and `deepagent.WriterDraft`
  implement it
- `cmd/usearch-api/handlers/deep_agents_handler.go` + test
- `internal/obs/metrics/deepagent.go` + test

**[NEW] Python**
- `services/researcher/src/researcher/faithfulness_endpoint.py`
- `services/researcher/tests/test_faithfulness_endpoint.py`

**[MODIFY]**
- `cmd/usearch-api/handlers/synthesis.go` (mode dispatch)
- `cmd/usearch-api/main.go` (register collectors)
- `internal/obs/metrics/metrics.go` (registerDeepAgent helper)
- `internal/obs/obs.go` (re-export collectors)
- `internal/streamsynth/longform.go` (add StreamFinalReport helper)
- `.env.example` (DEEP_AGENT_* documentation)

**[EXISTING — UNCHANGED]**
- `internal/fanout/` (Researcher consumer)
- `internal/llm/` (all agents consumer)
- `internal/deepreport/` (not touched)
- `internal/sse/` (consumer)
- `services/storm/` (not touched)

---

## Exclusions (What NOT to Build)

[HARD] 본 SPEC은 다음을 명시적으로 제외한다:

- **Per-user / per-day quota enforcement** → SPEC-DEEP-004
- **Tree exploration with breadth/depth knobs** → SPEC-DEEP-003
- **LLM-as-judge multi-dimensional scoring** (Verifier uses ONLY
  SYN-002 binary gate; no coverage/coherence/factuality scoring) →
  SPEC-EVAL-001
- **Reviewer follow-up retrieval / drill-down** — Reviewer는 critique-only,
  Researcher evidence에 한정 → 가능시 SPEC-DEEP-003 tree exploration과
  결합
- **New Python sidecar** — orchestration은 전적으로 Go-side;
  `/faithfulness_check`는 기존 `services/researcher` 사이드카에 추가
  되는 새 라우트일 뿐
- **STORM sidecar 확장** — `services/storm/`는 무손상 보존
- **`/deep?mode=storm` 동작 변경** — DEEP-001 경로 read-only
- **Token-level streaming from agent LLM calls** — Writer는 single-shot,
  `final_token` 이벤트 미emit (research.md §1/§6에 잘못된 암시 있었으나
  본 SPEC에서 제외; §1.1 footnote [¹] 참조)
- **Multi-agent retry coordination beyond Writer** — 다른 agent transient
  error는 즉시 abort: SSE 활성 시 terminal SSE event (REQ-DEEP2-009b-SSE),
  buffered 시 HTTP 503 (REQ-DEEP2-009b-Buffered)
- **`usearch deep --mode agents` CLI surface** → SPEC-CLI-002 (M7)
- **Non-LiteLLM LLM access** — 직접 vendor SDK 호출 금지
  (SPEC-LLM-001)
- **WebSocket / gRPC / NDJSON transport** — SSE 전용
- **`Last-Event-ID` SSE resume** — 미지원
- **MCP tool surface for `mode=agents`** → SPEC-MCP-001
- **GitHub Issue tracking** (`issue_number: 0`)

---

*Companion: spec.md, plan.md, acceptance.md, research.md*
