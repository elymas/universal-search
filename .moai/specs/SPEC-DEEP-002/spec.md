---
id: SPEC-DEEP-002
version: 0.1.1
status: draft
created: 2026-05-21
updated: 2026-05-21
author: limbowl
priority: P0
issue_number: 18
title: Multi-agent /deep pipeline (Researcher/Reviewer/Writer/Verifier)
milestone: M5 — /deep multi-agent
owner: expert-backend
methodology: tdd
coverage_target: 85
depends_on: [SPEC-DEEP-001, SPEC-SYN-002, SPEC-SYN-004, SPEC-FAN-001, SPEC-LLM-001, SPEC-CORE-001, SPEC-OBS-001, SPEC-IR-001]
blocks: [SPEC-DEEP-003, SPEC-DEEP-004]
---

# SPEC-DEEP-002: Multi-agent /deep pipeline (Researcher/Reviewer/Writer/Verifier)

## HISTORY

- 2026-05-21 (v0.1.1 audit patches, limbowl via manager-spec):
  plan-auditor iter-1 PASS with 1 MAJOR + 7 MINOR. Applied 4 targeted
  patches:
  - P-M1: REQ-DEEP2-009 split into 009a (max-retry exhaustion) and
    009b (non-Verifier agent error abort).
  - P-N1: REQ-DEEP2-001 rewritten with explicit SHALL verbs to align
    with REQ-002..REQ-012 EARS convention.
  - P-N2: §1.1 decision #7 footnote added explaining the
    `final_token → verifier_result` event-name refinement vs
    research.md §1.
  - P-N3: REQ-DEEP2-008 outcome label set narrowed to {success, error}
    for the per-attempt histogram. Retry counting moved to a separate
    counter metric.
  No other REQ/NFR/Exclusion/Scenario changes. Version 0.1.0 → 0.1.1.

- 2026-05-21 (initial draft v0.1.0, limbowl via manager-spec):
  First EARS-formatted SPEC for the M5 multi-agent `/deep` surface.
  복잡한 비판적-사고형 질의를 위해 4개 에이전트(Researcher → Reviewer
  → Writer → Verifier)가 순차적으로 협업하는 파이프라인을 새로 만든다.
  핵심 8개 아키텍처 결정사항은 Phase 0.5 사용자 인터뷰(Round 1+2)에서
  고정(pinned)된 상태이며 본 SPEC은 그 결정을 EARS 요구사항으로
  번역한다. 요지:
  (1) 오케스트레이션은 NEW Go 모듈 `internal/deepagent/` 전담 — 새로운
  Python 사이드카는 추가하지 않고 기존 STORM 사이드카도 확장하지
  않는다.
  (2) 파이프라인은 순차(sequential) Researcher → Reviewer → Writer →
  Verifier, Writer는 Verifier rejection 시 최대 2회 재시도(초기 1회 +
  재시도 2회 = 총 3회 Writer 호출 상한).
  (3) 모드 분기: 신규 `/deep?mode=agents` 엔드포인트 변형. 기존
  `/deep?mode=storm`(SPEC-DEEP-001) 동작은 무손상 보존. 기본 모드는
  하위호환성 보존을 위해 `mode=storm`이며 `mode=agents`는 M5 opt-in.
  (4) 검색은 기존 `internal/fanout`(FAN-001) 게이트웨이 결과를
  Researcher가 그대로 소비 — 신규 어댑터/검색 로직 없음. Reviewer
  단계의 drill-down follow-up 검색도 없음.
  (5) 모델 티어링: Researcher=Haiku, Reviewer=Haiku, Writer=Sonnet,
  Verifier=Sonnet — LiteLLM 모델 별칭을 env-var로 라우팅
  (`DEEP_AGENT_<ROLE>_MODEL`).
  (6) Verifier 게이트: SYN-002 faithfulness validator 재사용,
  `uncited_sentences_count == 0` 일 때만 PASS. 다차원 LLM-as-judge
  점수화 없음.
  (7) 스트리밍: 기존 `internal/streamsynth`(SYN-004) 위에 신규 SSE
  이벤트 타입(`agent_started`, `agent_completed`, `retry_started`,
  `verifier_result`) 적층. Writer/Verifier 재시도 루프는 사전버퍼링되어
  Verifier PASS 이후에만 섹션 이벤트가 전송된다(스트림 도중 재시도
  semantics 단순화 — 본 SPEC §1 Resolved Design Choices RDC-6 참조).
  (8) 비용 가드는 Prometheus 메트릭과 hardcoded `max_retries=2`만 본
  SPEC 책임. Per-user quota, daily budget, Haiku 사전 스크리닝은
  SPEC-DEEP-004(M5 후속)가 담당하며 본 SPEC에서 명시적으로 제외.

  Companion artifacts:
  - `.moai/specs/SPEC-DEEP-002/research.md` — Phase 0.5 deep research
    (627 lines, 9 sections — pinned decisions, code reuse map, new
    components, env-vars, metrics, SSE events, 10 risks, refs)
  - `.moai/specs/SPEC-DEEP-002/plan.md` — implementation plan with
    7 milestones, TDD test catalog, MX tag plan
  - `.moai/specs/SPEC-DEEP-002/acceptance.md` — Given/When/Then
    scenarios (8 main + 4 edge cases)
  - `.moai/specs/SPEC-DEEP-002/spec-compact.md` — compact view

  12 EARS REQs (10 × P0 + 2 × P1), 4 NFRs, ≤5 modules(Endpoint /
  Pipeline / LLM Routing / Verifier Gate / Streaming &
  Observability). Methodology: TDD (per quality.yaml), coverage
  target 85%, harness: standard. Owner: expert-backend.
  `issue_number: 0` 상태이며 plan-auditor 리뷰 + annotation
  cycle 통과 후 status `draft → approved` 전이.

---

## 1. Overview

본 SPEC은 M5 milestone의 두 번째 deliverable인 `/deep` 멀티-에이전트
파이프라인을 정의한다. SPEC-DEEP-001이 STORM 사이드카 기반의
wiki-style 장문 보고서를 생성하는 반면(`mode=storm`), DEEP-002은
feynman pattern을 적용한 4-에이전트 비판적-사고 워크플로우를
제공한다(`mode=agents`). 두 모드는 `/deep` 동일 엔드포인트에서
`?mode=` 쿼리 파라미터로 분기하며 상호 무손상 공존한다.

### 1.1 Pinned Architectural Decisions

다음 8개 결정사항은 Phase 0.5 사용자 인터뷰에서 확정되었다. 본 SPEC은
이 결정사항들을 EARS 요구사항으로 번역할 뿐이며 재논의하지 않는다.

1. **Orchestration host**: NEW Go module `internal/deepagent/`. 새로운
   Python 사이드카 추가 없음, 기존 STORM 사이드카 확장 없음.
2. **Pipeline shape**: Sequential 4-agent pipeline (Researcher →
   Reviewer → Writer → Verifier). Writer는 Verifier rejection 시
   최대 2회 재시도(총 3회 Writer 호출 상한).
3. **Mode dispatch**: 신규 `/deep?mode=agents` 엔드포인트 변형.
   DEEP-001 `/deep?mode=storm` 무손상. 미지정시 기본 `mode=storm`
   (DEEP-001 동작 보존, `mode=agents`는 M5 opt-in surface).
4. **Retrieval source**: Researcher는 기존 `internal/fanout`(FAN-001)
   결과를 소비. 신규 검색/어댑터 코드 없음. Reviewer의 drill-down
   follow-up 검색 없음.
5. **Per-role LLM model tiering**: Researcher=Haiku, Reviewer=Haiku,
   Writer=Sonnet, Verifier=Sonnet. LiteLLM 모델 별칭을 env-var
   `DEEP_AGENT_<ROLE>_MODEL`로 라우팅.
6. **Verifier gate**: SYN-002 faithfulness validator 재사용. PASS
   조건은 `uncited_sentences_count == 0` (이진 게이트). 다차원
   LLM-as-judge 점수화(coverage/coherence 등) 없음.
7. **Streaming**: 기존 `internal/streamsynth`(SYN-004) 위에 신규
   step-level SSE 이벤트(`agent_started`, `agent_completed`,
   `retry_started`, `verifier_result`)[¹] 적층. `final_token` 등
   기존 SYN-004 이벤트는 호환 유지.
8. **Cost guards**: ONLY Prometheus 메트릭 + hardcoded
   `max_retries=2`. Per-user quota, daily budget, Haiku 사전
   스크리닝은 본 SPEC 범위 밖이며 SPEC-DEEP-004의 책임.

[¹] Event taxonomy refinement: research.md §1 decision #7은 본래
이벤트 목록을 `(agent_started, agent_completed, retry_started,
final_token)`으로 제안했다. 본 SPEC은 SPEC 작성 단계에서 SYN-004와의
호환성 분석을 거쳐 `final_token`(token-level streaming 암시) 대신
`verifier_result`(Verifier 게이트의 PASS/FAIL outcome 통지)을 채택한다.
이는 RDC-6의 "Writer/Verifier 재시도 루프 사전버퍼링" 결정과 정합한다
(Writer의 token-level streaming을 emit하지 않음). 본 deviation의 의도와
근거는 §8 Exclusions의 "Token-level streaming from agent LLM calls"
항목에 명시되어 있다.

### 1.2 Motivation

M4(`/synthesize` 단문 합성)와 M5의 DEEP-001(STORM 장문 wiki)이 RAG
파이프라인의 "검색-요약" 축을 다룬다면, DEEP-002의 멀티-에이전트
워크플로우는 "비판적 검증"을 추가한다. 단일-LLM 합성에서 흔히 발생하는
hallucination/오답을 (a) Researcher가 fanout 결과로부터 검증가능한
근거를 추출하고, (b) Reviewer가 그 근거의 일관성·완전성을 비평하며,
(c) Writer가 비평을 반영해 최종 답안을 합성하고, (d) Verifier가
SYN-002 faithfulness 게이트로 인용 정확성을 검증하는 4단계로 분산해
검출/억제한다. Verifier rejection 시 Writer가 재시도하는 GAN-style
피드백 루프가 결정적 차별화 포인트다.

### 1.3 Resolved Design Choices

research.md §7의 10개 open question에 대한 본 SPEC의 결정. 분류:
(a) 본 SPEC 내 해결, (b) 구현시 재량, (c) 후속 SPEC에 위임.

- **RDC-1 (Verifier SYN-002 Go-side wrapper)** — (a) Resolved.
  REQ-DEEP2-006은 신규 Go 함수
  `internal/synthesis.CheckFaithfulness(ctx, text, citations, docs) (FaithfulnessResult, error)`
  를 정의하고, 이 함수가 researcher 사이드카(또는 services/researcher
  Python 코드)의 신규 endpoint `POST /faithfulness_check`를
  호출한다. Verifier 에이전트는 이 Go 래퍼만 호출하면 된다. 기존
  `/synthesize` endpoint는 재사용하지 않는다(분리된 책임).
- **RDC-2 (Reviewer role)** — (a) Resolved. Reviewer는 Researcher가
  수집·구조화한 근거(fanout 결과의 부분집합 + 추출된 핵심 claim)에
  대해 LLM-driven critique-only 단계로 정의된다(REQ-DEEP2-002).
  Reviewer는 신규 검색을 시작하지 않으며, Researcher가 전달한
  근거 외부에 접근하지 않는다. 출력은 구조화된 비평 노트
  (`claim_id`, `concern_type`, `severity`) 리스트로, Writer 프롬프트
  컨텍스트에 주입된다. LLM 재생성(rewrite)은 Writer 단계에서만 발생.
- **RDC-3 (Context cancellation)** — (a) Resolved. Orchestrator는
  매 에이전트 호출 직전에 `ctx.Err()`을 확인한다(REQ-DEEP2-002).
  취소 시 `pipeline_cancelled` SSE 이벤트를 발행한 뒤 스트림을
  종료한다. 부분 결과는 응답 본문에 누출하지 않는다.
- **RDC-4 (Writer retry semantics)** — (a) Resolved. "max 2 retries"는
  초기 호출 1회 + 재시도 2회 = 총 3회 Writer 호출 상한을
  의미한다(REQ-DEEP2-003). `DEEP_AGENT_MAX_RETRIES` env-var의
  기본값 2는 이 의미를 따른다. DEEP-001의 `deepreport.Client` 2회
  재시도 패턴과 의미가 동일하다.
- **RDC-5 (Model alias resolution)** — (a) Resolved.
  `internal/deepagent/config.go`에서 4개 env-var
  (`DEEP_AGENT_RESEARCHER_MODEL`, `_REVIEWER_MODEL`,
  `_WRITER_MODEL`, `_VERIFIER_MODEL`)을 중앙집중식으로 로드하고
  `deepagent.Config{ResearcherModel, ReviewerModel, WriterModel,
  VerifierModel}`로 노출한다. 개별 에이전트는 LiteLLM 라우터의
  `deploy/litellm/config.yaml` 설정에 접근하지 않으며,
  `llm.Client.Complete()` 호출시 `Request.Model`에 해당 별칭을
  지정한다. DEEP-001의 `STORM_MODEL_OUTLINE`/`STORM_MODEL_ARTICLE`
  네이밍을 그대로 계승.
- **RDC-6 (Streaming with retry)** — (a) Resolved. Writer/Verifier
  재시도 루프 전체는 사전버퍼링된다. SSE는 각 에이전트 시작/종료
  이벤트(`agent_started`, `agent_completed`)와 재시도 알림
  (`retry_started`, `verifier_result`)만 실시간 전송하며, 본문
  section/sentence 이벤트는 Verifier가 PASS 한 이후의 최종 Report에
  대해서만 emit된다. 따라서 클라이언트는 같은 `section_index`로
  중복된 콘텐츠를 받지 않는다(REQ-DEEP2-007의 이벤트 순서 정의).
- **RDC-7 (Per-request global budget cap)** — (c) Deferred to
  SPEC-DEEP-004. 본 SPEC은 메트릭으로 비용을 가시화할 뿐 cap을
  enforce 하지 않는다(§8 Exclusions 참조).
- **RDC-8 (Metrics cardinality)** — (a) Resolved. NFR-DEEP2-002가
  명시적으로 enumerable label set을 강제한다. `internal/deepagent`
  내부에 Go enum-like type (e.g.,
  `type Agent string; const (AgentResearcher Agent = "researcher" …)`)
  으로 라벨 값을 고정한다.
- **RDC-9 (Error propagation)** — (a) Resolved. REQ-DEEP2-003은
  Verifier rejection만 Writer 재시도를 트리거한다. Verifier rejection이
  최대 시도 횟수까지 누적된 max-retry exhaustion은 REQ-DEEP2-009a가
  관할하며, Researcher/Reviewer/Verifier 자체의 비복구성 에러(timeout,
  LLM 오류 등)는 REQ-DEEP2-009b가 즉시 파이프라인을 중단하고
  `pipeline_failed` SSE 이벤트 + HTTP 503 응답으로 종료시킨다. 두
  unwanted REQ는 서로 다른 IF preconditon (max-retry exhaustion vs
  agent-error abort)을 다루며 별개의 테스트 그룹을 갖는다.
- **RDC-10 (Heartbeat timing)** — (b) Implementation discretion.
  SYN-004의 heartbeat goroutine을 handler 진입 시점에 시작해
  파이프라인 전체 기간 동안 유지하는 것이 권장 패턴이지만, 정확한
  시작 타이밍(orchestrator 호출 전후) 및 간격 튜닝은 구현 단계에서
  결정한다.

---

## 2. File Impact Map

| Marker | Path | Purpose |
|--------|------|---------|
| [NEW] | `internal/deepagent/orchestrator.go` | 4-agent sequential pipeline orchestration + retry loop |
| [NEW] | `internal/deepagent/agents.go` | Researcher, Reviewer, Writer, Verifier agent functions |
| [NEW] | `internal/deepagent/types.go` | `Agent`(enum), `PipelineRequest`, `PipelineResult`, `AgentOutcome`, internal state types |
| [NEW] | `internal/deepagent/prompts.go` | Per-role LLM prompt templates (system + user messages) |
| [NEW] | `internal/deepagent/config.go` | `DEEP_AGENT_*` env-var loader, `Config` struct |
| [NEW] | `internal/deepagent/metrics.go` | Prometheus collector wrappers (agent_duration, agent_retries, verifier_gate_results) |
| [NEW] | `internal/deepagent/sse.go` | Helper that emits agent-level SSE events via `streamsynth` |
| [NEW] | `internal/deepagent/orchestrator_test.go` | Happy path, retry, max-retry-exhaustion, cancellation, empty-corpus tests |
| [NEW] | `internal/deepagent/agents_test.go` | Per-agent unit tests with mocked `llm.Client` and `fanout.Dispatch` |
| [NEW] | `internal/deepagent/config_test.go` | Env-var loading, default fallbacks |
| [NEW] | `internal/synthesis/faithfulness.go` | Go wrapper `CheckFaithfulness(ctx, text, citations, docs)` calling researcher sidecar `POST /faithfulness_check` |
| [NEW] | `internal/synthesis/faithfulness_test.go` | Wrapper unit tests with mocked HTTP server |
| [NEW] | `internal/streamsynth/agent_events.go` | Payload structs for `agent_started`, `agent_completed`, `retry_started`, `verifier_result`, `pipeline_failed`, `pipeline_cancelled` SSE events |
| [NEW] | `internal/streamsynth/agent_events_test.go` | JSON marshalling round-trip tests for event payloads |
| [NEW] | `cmd/usearch-api/handlers/deep_agents_handler.go` | HTTP handler for `/deep?mode=agents` — content negotiation, SSE setup, orchestrator dispatch, buffered fallback |
| [NEW] | `cmd/usearch-api/handlers/deep_agents_handler_test.go` | Integration tests via `httptest` |
| [NEW] | `services/researcher/src/researcher/faithfulness_endpoint.py` | Python-side `POST /faithfulness_check` endpoint (calls existing SYN-002 logic) |
| [NEW] | `services/researcher/tests/test_faithfulness_endpoint.py` | Python endpoint unit tests |
| [MODIFY] | `cmd/usearch-api/handlers/synthesis.go` | Add `?mode=` query parsing; route `mode=storm` → DEEP-001 (existing), `mode=agents` → new `deep_agents_handler` |
| [MODIFY] | `cmd/usearch-api/main.go` | Register new agents collectors via `deepagent.RegisterMetrics(pr)` |
| [MODIFY] | `internal/obs/metrics/metrics.go` | Add `registerDeepAgent(pr)` helper |
| [MODIFY] | `internal/obs/obs.go` | Re-export `obs.DeepAgentDuration`, `obs.DeepAgentRetries`, `obs.DeepAgentVerifierGateResults` |
| [MODIFY] | `.env.example` | Add `DEEP_AGENT_*` env-var documentation |
| [MODIFY] | `internal/streamsynth/longform.go` | Add public helper to emit pre-section agent events before walking sections (called by `deep_agents_handler` after Verifier PASS) |
| [EXISTING — UNCHANGED] | `internal/fanout/` (FAN-001) | Read-only consumer; Researcher calls `fanout.Dispatch()` |
| [EXISTING — UNCHANGED] | `internal/llm/client.go` (LLM-001) | Read-only consumer; all 4 agents route through `llm.Client.Complete()` |
| [EXISTING — UNCHANGED] | `internal/deepreport/` (DEEP-001) | Not used by DEEP-002 — Writer assembles its own Report struct, no STORM sidecar call |
| [EXISTING — UNCHANGED] | `internal/sse/writer.go` (SYN-004) | Read-only consumer |
| [EXISTING — UNCHANGED] | `services/storm/` (DEEP-001) | Not touched by DEEP-002; `mode=storm` route remains DEEP-001 path |

### 2.1 Module Boundaries (≤5 Requirement Modules)

본 SPEC의 EARS 요구사항은 다음 5개 모듈로 분류된다:

1. **Endpoint** — HTTP surface, mode dispatch, content negotiation (REQ-001, 010, 011)
2. **Pipeline** — Agent ordering, retry loop, cancellation, empty corpus (REQ-002, 003, 005, 009a, 009b, 012)
3. **LLM Routing** — Model alias resolution per role (REQ-004)
4. **Verifier Gate** — SYN-002 faithfulness reuse (REQ-006)
5. **Streaming & Observability** — SSE event types, Prometheus metrics (REQ-007, 008)

---

## 3. EARS Requirements

### 3.1 Endpoint Module

| ID | Pattern | Requirement | Priority | Acceptance |
|----|---------|-------------|----------|------------|
| REQ-DEEP2-001 | Ubiquitous | `cmd/usearch-api`는 `POST /deep` 엔드포인트에 `?mode=` 쿼리 파라미터를 도입 SHALL 한다. `mode=agents` 값은 DEEP-002의 4-에이전트 파이프라인으로 라우팅 SHALL 하며, 신규 핸들러 `cmd/usearch-api/handlers/deep_agents_handler.go`가 이를 처리 SHALL 한다. 요청 본문 스키마는 DEEP-001과 동일하게 `{request_id, query, lang}`를 수용 SHALL 하되, `docs[]` 필드는 더 이상 받지 않으며(Researcher가 자체적으로 fanout 호출), 응답은 `text/event-stream` SSE(기본) 또는 `?stream=false` 버퍼드 JSON(`{request_id, final: {…}, agent_log: […], schema_version: 1}`) 형식 중 하나로 emit SHALL 한다. | P0 | `TestDeepHandlerRoutesModeAgentsToOrchestrator`, `TestDeepHandlerDefaultsToModeStorm`, `TestDeepHandlerRequestSchemaMatchesContract` |
| REQ-DEEP2-010 | Optional | WHERE 요청 쿼리 파라미터가 `?stream=false`이거나 `Accept` 헤더가 `text/event-stream`을 명시적으로 advertise하지 않을 때, the handler SHALL fall back to a buffered JSON response containing the SAME data fields that would have been emitted in the final `agent_completed{agent: verifier}` SSE event PLUS the full `final` Report payload (sections, citations, model/provider/cost metadata). HTTP 200 with `Content-Type: application/json`. No SSE writer or heartbeat goroutine is instantiated on this path. | P1 | `TestDeepHandlerBufferedFallbackReturnsFinalReport`, `TestDeepHandlerBufferedFallbackHasNoSSEOverhead` |
| REQ-DEEP2-011 | Ubiquitous | The `mode=storm` route SHALL preserve SPEC-DEEP-001 behavior byte-identically. The `/deep?mode=agents` and `/deep?mode=storm` code paths SHALL share NO mutable state — no global variables, no shared singletons that mutate across requests. DEEP-001 acceptance suite SHALL remain 100% green after DEEP-002 lands (regression check). When `?mode=` is absent, the handler SHALL default to `mode=storm` to preserve backward compatibility with pre-DEEP-002 clients. | P0 | `TestDeepHandlerStormModeUnchanged` (regression via DEEP-001 acceptance suite), `TestDeepHandlerModeAbsentDefaultsToStorm`, `TestDeepHandlerNoSharedMutableStateBetweenModes` |

### 3.2 Pipeline Module

| ID | Pattern | Requirement | Priority | Acceptance |
|----|---------|-------------|----------|------------|
| REQ-DEEP2-002 | State-Driven | WHILE a `mode=agents` request is in progress, the orchestrator SHALL execute the 4 agents strictly in sequence: Researcher → Reviewer → Writer → Verifier. The orchestrator SHALL check `ctx.Err()` immediately before invoking each agent; on context cancellation it SHALL halt at the next agent boundary, emit a `pipeline_cancelled` SSE event (per REQ-DEEP2-007), and return without invoking any further LLM calls. Reviewer SHALL NOT initiate any new retrieval; it consumes ONLY the structured evidence produced by Researcher. Writer SHALL NOT call `fanout.Dispatch()`; it consumes ONLY Researcher evidence + Reviewer critique notes. | P0 | `TestOrchestratorRunsAgentsInOrder`, `TestOrchestratorHaltsOnContextCancelBetweenAgents`, `TestReviewerDoesNotCallFanout`, `TestWriterDoesNotCallFanout` |
| REQ-DEEP2-003 | Event-Driven | WHEN Verifier returns `uncited_sentences_count > 0` AND the current Writer attempt count is less than `DEEP_AGENT_MAX_RETRIES + 1` (default 3 = 1 initial + 2 retries), THEN the orchestrator SHALL re-invoke Writer with the original evidence/critique context PLUS a retry hint summarising the Verifier feedback. The orchestrator SHALL emit a `retry_started{agent: "writer", retry_count: N, reason: "verifier_rejection"}` SSE event before the retry call. The counter `usearch_deep_agent_retries_total{agent="writer"}` SHALL increment by exactly 1 per retry invocation. ONLY Verifier rejection triggers Writer retry — errors from Researcher, Reviewer, or Verifier itself (timeout, LLM error, etc.) SHALL NOT trigger any retry and SHALL surface via REQ-DEEP2-009b instead. Max-retry exhaustion (Verifier rejection at every attempt up to and including the final allowed attempt) is governed by REQ-DEEP2-009a. | P0 | `TestOrchestratorRetriesWriterOnVerifierReject`, `TestOrchestratorEmitsRetryStartedBeforeRetryCall`, `TestRetriesCounterIncrementsExactlyOncePerRetry`, `TestNonVerifierErrorsDoNotTriggerRetry` |
| REQ-DEEP2-005 | Ubiquitous | The Researcher agent SHALL invoke `fanout.Dispatch(ctx, query, registry, router)` from `internal/fanout` (SPEC-FAN-001) exactly once per pipeline invocation, AND SHALL pass the returned `Result.Docs []NormalizedDoc` (de-duplicated, scored union from all configured adapters) to subsequent agents via in-memory pipeline state. No alternative retrieval mechanism (direct DB query, web scraper, vector store call, second-pass fanout) is permitted within DEEP-002. The Researcher SHALL convert `Result.Docs` to a `[]NormalizedDocPayload` slice that is compatible with downstream agents but distinct in lifecycle (no shared mutable references). | P0 | `TestResearcherCallsFanoutDispatchExactlyOnce`, `TestResearcherUsesNoOtherRetrievalSource`, `TestResearcherDocsAreImmutableInDownstream` |
| REQ-DEEP2-009a | Unwanted | IF Writer is invoked the maximum allowed times (`DEEP_AGENT_MAX_RETRIES + 1`, default 3) AND Verifier still returns `uncited_sentences_count > 0` after the final attempt, THEN the orchestrator SHALL abort the pipeline AND the handler SHALL return HTTP 503 `Content-Type: application/json` body `{"error": "pipeline_failed", "detail": "Writer exhausted <N> attempts; Verifier still rejected", "uncited_count": <N>, "attempts": <N>}`. The SSE stream SHALL emit a terminal `pipeline_failed` event before closing. The counter `usearch_deep_agent_verifier_gate_results_total{result="fail_uncited"}` SHALL increment for each Verifier rejection (including the final one), AND `usearch_deep_outcomes_total{outcome="error_pipeline_failed"}` SHALL increment exactly once. | P0 | `TestMaxRetryExhaustionReturns503`, `TestMaxRetryExhaustionEmitsPipelineFailedSSE`, `TestErrorOutcomeCounterIncrementsExactlyOnce` |
| REQ-DEEP2-009b | Unwanted | IF Researcher, Reviewer, or Verifier (the agent itself, distinct from Verifier's PASS/FAIL outcome) returns a non-recoverable error (LLM upstream failure, timeout, panic) at any pipeline stage, THEN the orchestrator SHALL abort the pipeline AND the handler SHALL return HTTP 503 `Content-Type: application/json` body `{"error": "pipeline_failed", "detail": "<agent> failed: <reason>", "failed_agent": "<name>"}` AND the SSE stream SHALL emit a terminal `pipeline_failed{failed_agent, reason}` event before closing. No Writer retry SHALL be triggered by such errors (Writer retry is gated exclusively by REQ-DEEP2-003 on Verifier rejection outcomes). The counter `usearch_deep_outcomes_total{outcome="error_pipeline_failed"}` SHALL increment exactly once per aborted pipeline. | P0 | `TestResearcherErrorAbortsAndReturns503`, `TestReviewerErrorAbortsAndReturns503`, `TestVerifierErrorAbortsAndReturns503`, `TestNonVerifierErrorsDoNotTriggerRetry` |
| REQ-DEEP2-012 | Unwanted | IF `fanout.Dispatch()` returns an empty `Result.Docs` slice (zero docs after all adapter dedup), THEN the orchestrator SHALL short-circuit the pipeline: Reviewer/Writer/Verifier SHALL NOT be invoked, the response SHALL be HTTP 200 with body `{"final": {"sections": [], "citations": []}, "agent_log": [{"agent": "researcher", "outcome": "empty_corpus"}], "schema_version": 1}`, and the counter `usearch_deep_outcomes_total{outcome="empty_corpus"}` SHALL increment exactly once. The SSE stream SHALL emit `agent_started{agent: "researcher"}`, then `agent_completed{agent: "researcher", outcome: "empty_corpus"}`, then a terminal `done` event with `total_sections: 0` — no `pipeline_failed` event (empty corpus is a degenerate but non-error outcome). | P0 | `TestEmptyFanoutShortCircuitsPipeline`, `TestEmptyFanoutResponseShape`, `TestEmptyFanoutOutcomeCounterIncrements`, `TestEmptyFanoutSSEEventSequence` |

### 3.3 LLM Routing Module

| ID | Pattern | Requirement | Priority | Acceptance |
|----|---------|-------------|----------|------------|
| REQ-DEEP2-004 | Ubiquitous | Each of the 4 agents SHALL use the LiteLLM model alias resolved from its corresponding environment variable: `DEEP_AGENT_RESEARCHER_MODEL` (default `claude-3-5-haiku-20241022`), `DEEP_AGENT_REVIEWER_MODEL` (default `claude-3-5-haiku-20241022`), `DEEP_AGENT_WRITER_MODEL` (default `claude-3-5-sonnet-20241022`), `DEEP_AGENT_VERIFIER_MODEL` (default `claude-3-5-sonnet-20241022`). Env-var loading SHALL be centralised in `internal/deepagent/config.go` and exposed via a `deepagent.Config` struct. Individual agents SHALL NOT call `os.Getenv()` directly; they SHALL receive their resolved model alias via the orchestrator's per-call context. All agent LLM invocations SHALL flow through the singleton `llm.Client` from SPEC-LLM-001 — direct vendor SDK calls are PROHIBITED. The configured model alias SHALL be passed as `Request.Model` to `llm.Client.Complete()`. | P0 | `TestConfigLoadsAllFourModelAliasesFromEnv`, `TestConfigFallsBackToDefaultsWhenEnvAbsent`, `TestAgentsReceiveResolvedModelFromOrchestrator`, `TestNoDirectOsGetenvInAgentsPackage`, `TestAllAgentsCallSingletonLLMClient` |

### 3.4 Verifier Gate Module

| ID | Pattern | Requirement | Priority | Acceptance |
|----|---------|-------------|----------|------------|
| REQ-DEEP2-006 | Ubiquitous | The Verifier agent SHALL invoke a NEW Go function `internal/synthesis.CheckFaithfulness(ctx, text, citations, docs) (FaithfulnessResult, error)` (defined by this SPEC) which posts to the researcher Python sidecar's NEW endpoint `POST /faithfulness_check` (also defined by this SPEC). The Python endpoint SHALL reuse the existing SYN-002 enforcement logic in `services/researcher/src/researcher/faithfulness.py` without modification (single source of truth preserved). `FaithfulnessResult` SHALL contain the fields `{UncitedSentencesCount int, UncitedSentences []string, OutcomeOK bool}`. The Verifier SHALL return PASS to the orchestrator IFF `result.UncitedSentencesCount == 0` (binary gate). No additional dimensions (coverage, coherence, factuality, LLM-as-judge scoring) SHALL be evaluated. The `usearch_deep_agent_verifier_gate_results_total{result}` counter SHALL emit exactly one increment per Verifier invocation with label values from {`pass`, `fail_uncited`, `fail_timeout`}. | P0 | `TestVerifierCallsCheckFaithfulnessExactlyOnce`, `TestVerifierPassWhenUncitedCountZero`, `TestVerifierFailWhenUncitedCountPositive`, `TestVerifierDoesNotPerformAdditionalScoring`, `TestFaithfulnessEndpointReusesExistingSYN002Logic`, `TestVerifierGateResultsCounterIncrementsOncePerInvocation` |

### 3.5 Streaming & Observability Module

| ID | Pattern | Requirement | Priority | Acceptance |
|----|---------|-------------|----------|------------|
| REQ-DEEP2-007 | State-Driven | WHILE the request `Accept` header advertises `text/event-stream` (case-insensitive) AND `?stream=false` is absent, the handler SHALL emit SSE events in the following order: (1) `agent_started{agent: researcher, timestamp_ms, schema_version: 1}` immediately on pipeline start; (2) `agent_completed{agent: researcher, outcome, duration_ms, error_message?}` after Researcher returns; (3) `agent_started{agent: reviewer}` then `agent_completed{agent: reviewer, ...}`; (4) one or more `agent_started{agent: writer}` + `agent_completed{agent: writer, ...}` cycles (per attempt); (5) `agent_started{agent: verifier}` + `verifier_result{result, details?}` + `agent_completed{agent: verifier, outcome}` per Verifier invocation; (6) `retry_started{agent: writer, retry_count: 1|2, reason: "verifier_rejection"}` BEFORE each Writer retry invocation; (7) on Verifier final PASS: the existing SYN-004 sequence (`section_start`, `sentence`, `section_done`, `done`) over the Writer's final Report (sections/citations buffered until PASS — see RDC-6). On unrecoverable failure (REQ-DEEP2-009) the terminal event SHALL be `pipeline_failed{failed_agent, reason}`. On context cancellation (REQ-DEEP2-002) the terminal event SHALL be `pipeline_cancelled{at_agent}`. All new event payloads SHALL carry `schema_version: 1` and the `request_id` field. The SYN-004 heartbeat `: ping\n\n` SHALL continue throughout the agent phases (i.e., the heartbeat goroutine starts at handler entry, not at first `section_start`). | P0 | `TestSSEEmitsAgentStartedAndCompletedForEachAgent`, `TestSSEEmitsRetryStartedBeforeWriterRetry`, `TestSSEEmitsVerifierResultPerVerifierInvocation`, `TestSSEEmitsPipelineFailedOnExhaustion`, `TestSSEEmitsPipelineCancelledOnContextCancel`, `TestSSESectionEventsOnlyAfterVerifierPass`, `TestSSEHeartbeatContinuesDuringAgentPhases`, `TestAllNewEventPayloadsCarrySchemaVersionAndRequestId` |
| REQ-DEEP2-008 | Ubiquitous | Three new Prometheus collectors SHALL be registered in `internal/obs/metrics/deepagent.go` and re-exported via `internal/obs/obs.go`: (1) `usearch_deep_agent_duration_seconds` (Histogram) with labels `{agent, outcome}` where `agent ∈ {researcher, reviewer, writer, verifier}` (4 values, bounded) AND `outcome ∈ {success, error}` (2 values, bounded) — the per-attempt histogram observes one bucket sample per agent invocation regardless of retry status, with retry counts tracked separately by the `usearch_deep_agent_retries_total` counter below; cardinality 4×2=8; buckets `[0.5, 1, 2, 5, 10, 30, 60, 120]` seconds. (2) `usearch_deep_agent_retries_total` (CounterVec) with label `agent` restricted to value `writer` only (1 value, bounded), cardinality 1; this counter is the canonical source of retry attribution. (3) `usearch_deep_agent_verifier_gate_results_total` (CounterVec) with label `result ∈ {pass, fail_uncited, fail_timeout}` (3 values, bounded), cardinality 3. All label values SHALL be pre-declared at registration time via `.WithLabelValues(value).Add(0)` per the SYN-004 `streamsynth.go:48-56` pattern. No label value SHALL be derived from user input, query content, adapter name, or model name. The existing `usearch_deep_outcomes_total{outcome}` counter (SPEC-DEEP-001) SHALL be extended with two new label values `empty_corpus` (REQ-DEEP2-012) and `error_pipeline_failed` (REQ-DEEP2-009a, REQ-DEEP2-009b), pre-declared identically. | P0 | `TestThreeNewCollectorsRegisteredAtStartup`, `TestAllAgentLabelValuesPreDeclaredAtRegistration`, `TestNoLabelValueDerivedFromUserInput`, `TestDeepOutcomesExtendedWithEmptyCorpusAndPipelineFailed`, `TestCardinalityGuardRemainsGreen` (`internal/obs/metrics/metrics_test.go::TestNoUnboundedLabels`) |

---

## 4. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| NFR-DEEP2-001 | Performance (end-to-end p95 latency) | The end-to-end latency from `cmd/usearch-api` request acceptance to terminal SSE event (`done` on success, `pipeline_failed` on failure, `pipeline_cancelled` on cancel) SHALL be **p95 ≤ 60 s** for a corpus of 20–50 fanout docs, assuming Verifier passes on the first or second attempt, with default model tiers (Researcher/Reviewer Haiku, Writer/Verifier Sonnet). 모델 별칭이 더 큰 모델로 오버라이드된 경우 또는 max-retry exhaustion 시나리오는 본 NFR의 budget violation으로 간주하지 않으며 REQ-DEEP2-009의 degraded path로 처리된다. DEEP-001의 NFR-DEEP1-001 (p50≤180s, p95≤300s)와 별도의 예산이며 DEEP-002의 multi-agent 오버헤드 + 재시도 가능성을 반영해 더 짧게 설정되었다(no STORM full-article generation, 짧은 합성 출력). 측정 방법: `services/researcher/tests/test_faithfulness_endpoint.py`의 모킹 + `internal/deepagent/orchestrator_test.go`의 50-iteration 통계 + `cmd/usearch-api/handlers/deep_agents_handler_test.go`의 end-to-end harness. |
| NFR-DEEP2-002 | Cardinality Safety | All Prometheus label values introduced by this SPEC SHALL come from bounded enumerable sets pre-declared at startup time, per NFR-OBS-002 from SPEC-OBS-001. Specifically: `agent ∈ {researcher, reviewer, writer, verifier}`, `outcome ∈ {success, error}`, `result ∈ {pass, fail_uncited, fail_timeout}`. Enum-like Go types (e.g., `type Agent string` with `const` declarations) SHALL enforce the bounded set at compile time within `internal/deepagent/types.go`. No user-derived strings (query content, model name override, adapter name, error message snippet) SHALL appear in label position. The existing cardinality guard test in `internal/obs/metrics/metrics_test.go` SHALL remain green without label-NAME allowlist amendments (only new label NAMES would require allowlist updates; DEEP-002 adds new VALUES to label names that may be pre-existing OR adds new label names that the test framework validates as bounded). |
| NFR-DEEP2-003 | Backward Compatibility | SPEC-DEEP-001 acceptance suite (`services/storm/tests/`, `internal/deepreport/`, `internal/streamsynth/longform_test.go`, `cmd/usearch-api/handlers/synthesis.go` regression) SHALL remain 100% green after DEEP-002 lands. The `/deep?mode=storm` request path SHALL produce byte-identical responses pre- and post-DEEP-002 (modulo `request_id`). The `?mode=` query parameter SHALL default to `storm` when absent. No shared mutable global state SHALL be introduced between the `storm` and `agents` code paths (separate handler functions, separate orchestrator entry points, separate metric collectors). Code review SHALL verify that `internal/deepagent/` imports do not include `internal/deepreport/` or `services/storm/`. |
| NFR-DEEP2-004 | Cost Visibility | The metric `usearch_deep_agent_duration_seconds{agent, outcome}` SHALL be queryable by `agent` label in Prometheus / Grafana, enabling operators to compute per-agent cost attribution (when combined with the per-agent model alias known via env-vars). The metric `usearch_deep_agent_retries_total{agent="writer"}` SHALL allow operators to detect anomalous retry rates indicative of prompt regression or model degradation. Per-request token-cost emission (in `agent_completed` SSE payload `cost_usd` field, summed from `llm.Client.Complete()` `Response.CostUSD`) SHALL be present on all successful agent completions. No per-user, per-day, or per-IP aggregation is required by this SPEC (deferred to SPEC-DEEP-004). |

---

## 5. Acceptance Criteria Summary

상세 Given/When/Then scenarios는 `.moai/specs/SPEC-DEEP-002/acceptance.md`에 정의되어 있다. 본 절은 인덱스만 제공한다.

| Scenario | Coverage |
|----------|----------|
| Scenario 1 — Happy path (Verifier PASS first attempt, SSE stream complete) | REQ-001, 002, 005, 006, 007, 008 |
| Scenario 2 — Retry path (Verifier rejects iter 1, PASS iter 2) | REQ-003, 007, 008 |
| Scenario 3 — Max-retry exhaustion (Verifier rejects all 3 attempts → 503) | REQ-009a, 007, 008 |
| Scenario 4 — Context cancellation mid-pipeline | REQ-002, 007 |
| Scenario 5 — Mode coexistence (`/deep?mode=storm` unchanged) | REQ-011 |
| Scenario 6 — Buffered fallback (`?stream=false`) | REQ-010 |
| Scenario 7 — Empty fanout corpus | REQ-012 |
| Scenario 8 — Researcher error aborts pipeline | REQ-009b |
| Edge Cases — Reviewer no-fanout, Writer no-fanout, default mode, env-var override fallback | REQ-002, 004, 005, 011 |

---

## 6. Configuration / Environment Variables

| Env Var | Default | Purpose | Owner |
|---------|---------|---------|-------|
| `DEEP_AGENT_RESEARCHER_MODEL` | `claude-3-5-haiku-20241022` | Researcher LLM model alias (Haiku tier) | REQ-DEEP2-004 |
| `DEEP_AGENT_REVIEWER_MODEL` | `claude-3-5-haiku-20241022` | Reviewer LLM model alias (Haiku tier) | REQ-DEEP2-004 |
| `DEEP_AGENT_WRITER_MODEL` | `claude-3-5-sonnet-20241022` | Writer LLM model alias (Sonnet tier) | REQ-DEEP2-004 |
| `DEEP_AGENT_VERIFIER_MODEL` | `claude-3-5-sonnet-20241022` | Verifier LLM model alias (Sonnet tier) | REQ-DEEP2-004 |
| `DEEP_AGENT_MAX_RETRIES` | `2` | Maximum Writer retries on Verifier rejection (total Writer attempts = this + 1) | REQ-DEEP2-003, RDC-4 |
| `DEEP_AGENT_WRITER_RETRY_DELAY_MS` | `500` | Backoff between Writer retries (exponential not required for v0) | REQ-DEEP2-003 |
| `DEEP_AGENT_VERIFIER_TIMEOUT_MS` | `30000` | Per-call timeout for the `POST /faithfulness_check` HTTP invocation | REQ-DEEP2-006 |
| `DEEP_AGENT_FAITHFULNESS_URL` | `http://researcher:8080/faithfulness_check` | Researcher sidecar endpoint for faithfulness check | REQ-DEEP2-006 |

Inherited from prior SPECs (NOT modified by DEEP-002):

- `LITELLM_BASE_URL`, `LITELLM_MASTER_KEY` — LLM-001 LiteLLM proxy auth (REQ-LLM-005)
- `SYN004_SSE_HEARTBEAT_MS`, `SYN004_DISCONNECT_CANCEL_MS`, `SYN004_SSE_WRITE_TIMEOUT_MS` — SYN-004 streaming knobs

Per-call request body overrides for `DEEP_AGENT_MAX_RETRIES` are NOT
supported in v0 (deferred to SPEC-DEEP-004 quota mechanism).

---

## 7. References

### 7.1 Internal SPEC Documents

- `.moai/specs/SPEC-DEEP-001/spec.md` — STORM long-form sidecar (sibling M5 SPEC; mode coexistence anchor)
- `.moai/specs/SPEC-DEEP-002/research.md` — Phase 0.5 deep research (627 lines, this SPEC's authoritative codebase context)
- `.moai/specs/SPEC-SYN-002/spec.md` — Citation faithfulness contract (Verifier gate reuses this)
- `.moai/specs/SPEC-SYN-004/spec.md` — SSE wire format and event taxonomy (extended by REQ-DEEP2-007)
- `.moai/specs/SPEC-FAN-001/spec.md` — Fanout dispatch contract (Researcher consumer)
- `.moai/specs/SPEC-LLM-001/spec.md` — LiteLLM client contract (all 4 agents route through this)
- `.moai/specs/SPEC-CORE-001/spec.md` — `NormalizedDoc` shape (Researcher output type)
- `.moai/specs/SPEC-OBS-001/spec.md` — Metrics cardinality safety (NFR-OBS-002 enforced by NFR-DEEP2-002)
- `.moai/specs/SPEC-IR-001/spec.md` — `/deep` verb routing (precondition for `?mode=` parameter)
- `.moai/project/roadmap.md` §M5 — DEEP-002 roadmap row

### 7.2 Implementation References (Reuse Map)

Source: research.md §2 and §8. See plan.md §5 for the full reference implementation mapping.

- `internal/deepreport/client.go` — Retry loop pattern for `orchestrator.go`
- `internal/synthesis/client.go` — Sidecar HTTP client pattern for `faithfulness.go`
- `internal/fanout/dispatch.go` — Researcher input
- `internal/streamsynth/longform.go` — Section event emission pattern (post-Verifier-PASS streaming)
- `internal/sse/writer.go` — Thread-safe SSE frame writer
- `internal/obs/metrics/deepreport.go` — Collector registration pattern

### 7.3 Companion Artifacts

- `.moai/specs/SPEC-DEEP-002/plan.md` — Milestones, TDD test catalog, MX tag plan
- `.moai/specs/SPEC-DEEP-002/acceptance.md` — Given/When/Then scenarios (8 main + 4 edge cases)
- `.moai/specs/SPEC-DEEP-002/spec-compact.md` — Compact view (~30% token reduction)

---

## 8. Exclusions (What NOT to Build)

[HARD] 본 SPEC은 다음 항목을 명시적으로 제외한다. 각 항목은 후속 SPEC
또는 별도 트랙의 책임이다.

- **Per-user / per-day quota enforcement** — 본 SPEC은 비용을 메트릭으로
  가시화할 뿐 cap을 enforce 하지 않는다. Per-user quota, daily budget,
  rate limiting은 SPEC-DEEP-004 (M5 후속 deliverable)의 책임.
- **Tree exploration with configurable breadth/depth** — Researcher의
  fan-out depth, Reviewer의 multi-pass critique, Writer의 multi-branch
  draft 탐색은 SPEC-DEEP-003 (M5 후속)의 책임. DEEP-002은 sequential
  4-agent의 single linear path 만 지원한다.
- **LLM-as-judge multi-dimensional scoring** — Verifier는 SYN-002
  faithfulness 이진 게이트만 사용한다. coverage, coherence, factuality,
  semantic-faithfulness(RAGAS/DeepEval) 등의 다차원 점수화는
  SPEC-EVAL-001 (M8)의 책임. DEEP-002의 Verifier가 LLM-as-judge로
  진화하는 변경은 본 SPEC의 RDC-2와 명시적으로 배치된다.
- **Reviewer follow-up retrieval / drill-down** — Reviewer는
  critique-only (Researcher가 전달한 evidence에 한정). Reviewer가
  추가 검색 호출(secondary fanout, web search, knowledge base lookup)을
  initiating하는 동작은 본 SPEC 범위 밖이며 SPEC-DEEP-003의 tree
  exploration과 결합되어 별도로 다뤄진다.
- **New Python sidecar** — Orchestration은 전적으로 Go-side
  (`internal/deepagent/`)에서 처리된다. 신규 Python service 추가는
  본 SPEC 범위 밖. Verifier가 호출하는 `POST /faithfulness_check`
  endpoint는 기존 `services/researcher` 사이드카에 추가되는 새 라우트
  일 뿐, 신규 service가 아니다.
- **STORM 사이드카 확장** — `services/storm/`는 본 SPEC에서 무손상
  보존된다. DEEP-002가 STORM 사이드카에 신규 endpoint를 추가하거나
  기존 endpoint의 동작을 변경하지 않는다.
- **`/deep?mode=storm` 동작 변경** — DEEP-001 경로는 read-only.
  REQ-DEEP2-011이 회귀 방지를 강제한다.
- **Token-level streaming from agent LLM calls** — Writer가 LLM
  스트리밍(token-by-token)으로 출력을 emit하는 동작은 본 SPEC 범위
  밖. Writer는 single-shot completion으로 호출되며, sentence/section
  분할은 Verifier PASS 이후 SYN-004 `streamsynth.StreamLongFormReport`
  가 담당한다. `final_token` SSE 이벤트 명칭이 research.md §6에
  언급되었으나, 본 SPEC은 이 이름의 이벤트를 emit하지 않는다(잘못된
  암시를 피하기 위해 본 §8에 명시적으로 제외).
- **Multi-agent retry coordination beyond Writer** — Researcher/Reviewer/Verifier의 retry는 본 SPEC이 지원하지 않는다.
  이들 agent의 transient error는 즉시 pipeline abort + 503 응답으로
  surfacing된다(REQ-DEEP2-009).
- **`/deep agents` CLI surface in `cmd/usearch`** — DEEP-002는 사이드카 +
  Go 클라이언트 + API 핸들러만 ship 한다. CLI verb `usearch deep --mode
  agents "..."`의 wiring은 SPEC-CLI-002 (M7)의 책임.
- **Non-LiteLLM LLM access** — 직접 vendor SDK 호출은
  SPEC-LLM-001에 의해 금지된다. 모든 4개 agent의 LLM 호출은
  `llm.Client.Complete()`를 경유해야 한다.
- **WebSocket / gRPC / NDJSON transport** — Long-form streaming은
  SYN-004와 동일하게 SSE 전용이다.
- **Resume / replay of in-progress pipelines (`Last-Event-ID`)** —
  지원하지 않는다. 각 `/deep?mode=agents` 호출은 fresh client
  connection.
- **MCP tool surface for `mode=agents`** — SPEC-MCP-001의 범위.
- **GitHub Issue tracking** (`issue_number: 0`).

---

*End of SPEC-DEEP-002 v0.1.1 (draft).*
