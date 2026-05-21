---
id: SPEC-DEEP-002
version: 0.1.1
created: 2026-05-21
author: limbowl (via manager-spec)
companion_to: spec.md
---

# SPEC-DEEP-002 Implementation Plan

본 문서는 SPEC-DEEP-002의 구현 계획서다. spec.md가 "무엇을 만들지"를
정의한다면 plan.md는 "어떤 순서로 어떻게 만들지"를 정의한다. 모든
요구사항 ID(REQ-DEEP2-NNN, NFR-DEEP2-NNN)는 spec.md를 참조하며, 본
문서에서 재정의하지 않는다.

---

## 1. Overview

DEEP-002의 구현은 다음 4개 축으로 진행된다:

- **Go 모듈 신규 생성**: `internal/deepagent/` (orchestrator, 4 agents,
  config, metrics, sse, types, prompts) + `internal/synthesis/`의
  `faithfulness.go` Go-side wrapper.
- **Python 사이드카 확장**: `services/researcher/` 에 신규 endpoint
  `POST /faithfulness_check` 추가 (기존 SYN-002 enforcement 로직 재사용).
- **SSE 이벤트 타입 적층**: `internal/streamsynth/agent_events.go` —
  기존 SYN-004 이벤트와 호환되는 신규 step-level 이벤트.
- **HTTP 핸들러 통합**: `cmd/usearch-api/handlers/synthesis.go`에
  `?mode=` 라우팅 추가, 신규 `deep_agents_handler.go` 구현.

Methodology: **TDD (RED-GREEN-REFACTOR)** per
`.moai/config/sections/quality.yaml` `development_mode: tdd`.
Coverage target: **85%** per `coverage_target` SPEC frontmatter.
Harness level: **standard** (multi-domain backend, no
security/payment keywords, multi-file new module).

---

## 2. Milestones (Priority-based, NO Time Estimates)

마일스톤은 의존성 순서대로 정렬되어 있다. 각 마일스톤은 RED-GREEN-REFACTOR
주기를 따른다. 우선순위는 본 SPEC 내부 ordering이며 외부 release
prioritization과는 별개다.

### M1 — Foundation: Types, Config, Prompts, LLM Routing  [Priority High]

- `internal/deepagent/types.go` — `Agent` enum-like type
  (`AgentResearcher`, `AgentReviewer`, `AgentWriter`,
  `AgentVerifier`), `PipelineRequest`, `PipelineResult`,
  `AgentOutcome`, `AgentLogEntry`, `FaithfulnessResult` (returned by
  `internal/synthesis.CheckFaithfulness`)
- `internal/deepagent/config.go` — `Config` struct loading
  `DEEP_AGENT_*` env-vars with defaults per spec.md §6
- `internal/deepagent/prompts.go` — System + user prompt templates
  per role (Researcher: evidence extraction; Reviewer: critique-only;
  Writer: synthesis with critique injection; Verifier: structured
  call to CheckFaithfulness — no prompt needed but kept for symmetry)
- `internal/deepagent/config_test.go` — env-var loading, default
  fallback tests
- Test count: 6-8 tests (config × 4 model aliases, defaults, retry
  count parsing, faithfulness URL)

Acceptance:
- REQ-DEEP2-004 (LLM routing scaffolding) RED tests fail without
  implementation
- `TestConfigLoadsAllFourModelAliasesFromEnv` GREEN
- `TestNoDirectOsGetenvInAgentsPackage` GREEN (grep-style enforcement)

### M2 — Researcher + Reviewer Agents (Haiku Tier)  [Priority High]

- `internal/deepagent/agents.go` — `Researcher(ctx, cfg, req,
  fanoutFn) (ResearcherOutput, error)` and `Reviewer(ctx, cfg,
  llmClient, research ResearcherOutput) (ReviewerCritique, error)`
- Researcher 동작:
  1. `fanoutFn(ctx, query, registry, router)` 호출 (1회만, REQ-DEEP2-005)
  2. Empty `Result.Docs` 시 `ResearcherOutput{IsEmpty: true}` 반환
     (REQ-DEEP2-012의 short-circuit 입력)
  3. Non-empty 시 LLM (Haiku) 호출하여 docs에서 핵심 claim 추출
  4. 출력: `{Claims []Claim, Evidence []NormalizedDocPayload, IsEmpty bool}`
- Reviewer 동작:
  1. Researcher의 `Claims + Evidence`만 입력으로 받음
  2. **fanout.Dispatch 호출 금지** (test enforced)
  3. LLM (Haiku)로 critique-only: `[]{ClaimID, Concern, Severity}`
  4. 출력: `ReviewerCritique{Notes []CritiqueNote}`
- `internal/deepagent/agents_test.go` — mocked `llm.Client` + mocked
  `fanout.Dispatch`로 단위 테스트
- Test count: 10-12 tests (researcher: fanout 호출 횟수, empty corpus,
  claim 추출 / reviewer: fanout 미호출 검증, critique 구조)

Acceptance:
- `TestResearcherCallsFanoutDispatchExactlyOnce` GREEN
- `TestReviewerDoesNotCallFanout` GREEN
- REQ-DEEP2-005 (Researcher consumes fanout) covered

### M3 — Writer Agent (Sonnet Tier)  [Priority High]

- `internal/deepagent/agents.go` — `Writer(ctx, cfg, llmClient,
  research ResearcherOutput, critique ReviewerCritique, retryHint
  *VerifierFeedback) (WriterDraft, error)`
- Writer 동작:
  1. Researcher evidence + Reviewer critique + (optional) retry hint를
     context로 결합
  2. **fanout.Dispatch 호출 금지** (test enforced)
  3. LLM (Sonnet) 호출하여 sentence-level citation을 포함한 final
     draft 생성
  4. 출력: `WriterDraft{Sections []Section, Citations []Citation,
     CostUSD float64, Model, Provider string}`
- Draft assembly: Writer 출력을 `streamsynth`가 walk 가능한
  `deepreport.Report`-compatible 구조로 변환 (단, `deepreport`
  패키지는 import하지 않음 — NFR-DEEP2-003 격리 보장)
- Test count: 8-10 tests (writer: fanout 미호출, retry hint 적용,
  draft 구조 well-formedness, citation marker 1-indexed)

Acceptance:
- `TestWriterDoesNotCallFanout` GREEN
- `TestWriterAcceptsRetryHintAndPrependsToContext` GREEN
- `TestWriterDraftSectionsHaveSentenceLevelCitations` GREEN

### M4 — Verifier Agent + SYN-002 Wrapper + Retry Loop  [Priority High]

- `services/researcher/src/researcher/faithfulness_endpoint.py` —
  신규 endpoint `POST /faithfulness_check`. 기존 `faithfulness.py`의
  enforcement 로직을 호출만 함 (단일 책임 보존)
- `services/researcher/tests/test_faithfulness_endpoint.py` — Python
  unit tests
- `internal/synthesis/faithfulness.go` — `CheckFaithfulness(ctx, text,
  citations, docs) (FaithfulnessResult, error)`. Go-side HTTP 클라이언트
  로 사이드카 endpoint 호출. `internal/synthesis/client.go` 패턴 재사용
- `internal/synthesis/faithfulness_test.go` — `httptest.Server`로
  사이드카 시뮬레이션
- `internal/deepagent/agents.go` — `Verifier(ctx, cfg, draft
  WriterDraft, docs []NormalizedDocPayload) (VerifierResult, error)`
- Verifier 동작:
  1. `synthesis.CheckFaithfulness(ctx, draft.SerializedText,
     draft.Citations, docs)` 호출
  2. `result.UncitedSentencesCount == 0` → `VerifierResult{Pass: true}`
  3. > 0 → `VerifierResult{Pass: false, Feedback: &VerifierFeedback{
     UncitedSentences: ..., UncitedCount: N}}`
- `internal/deepagent/orchestrator.go` — retry loop:
  - 초기 Writer 호출 1회 + 재시도 최대 2회 (총 3회 상한;
    `cfg.MaxRetries + 1`)
  - Verifier rejection 시 `retry_started` SSE 이벤트 emit + 재시도
  - Max-retry exhaustion 시 `REQ-DEEP2-009` path (HTTP 503 +
    `pipeline_failed` SSE)
- Test count: 12-15 tests (verifier: PASS/FAIL 분기, 사이드카 호출,
  orchestrator: retry count, max-retry exhaustion, non-verifier error 처리)

Acceptance:
- `TestVerifierPassWhenUncitedCountZero` GREEN
- `TestOrchestratorRetriesWriterOnVerifierReject` GREEN
- `TestMaxRetryExhaustionReturns503` GREEN (REQ-DEEP2-009a)
- `TestNonVerifierErrorsDoNotTriggerRetry` GREEN (REQ-DEEP2-009b)
- `TestResearcherErrorAbortsAndReturns503`, `TestReviewerErrorAbortsAndReturns503`, `TestVerifierErrorAbortsAndReturns503` GREEN (REQ-DEEP2-009b)
- REQ-DEEP2-003, 006, 009a, 009b covered

### M5 — SSE Event Extension + Handler Integration  [Priority High]

- `internal/streamsynth/agent_events.go` — payload structs:
  `AgentStartedPayload`, `AgentCompletedPayload`,
  `RetryStartedPayload`, `VerifierResultPayload`,
  `PipelineFailedPayload`, `PipelineCancelledPayload`. 모두
  `schema_version: 1` 필드 포함
- `internal/streamsynth/agent_events_test.go` — JSON marshalling
  round-trip
- `internal/deepagent/sse.go` — `EmitAgentEvent(w *sse.Writer, event
  AgentEvent) error` helper. orchestrator가 매 단계에서 호출
- `internal/streamsynth/longform.go` — 신규 public 함수 `StreamFinalReport(ctx, w, draft WriterDraft, agentLog []AgentLogEntry)` 추가
  (Verifier PASS 후 호출, section/sentence/done 이벤트 emit)
- `cmd/usearch-api/handlers/synthesis.go` — `?mode=` 쿼리 파싱:
  - absent or `storm` → 기존 DEEP-001 path (변경 없음, REQ-DEEP2-011)
  - `agents` → 신규 `deep_agents_handler` 호출
- `cmd/usearch-api/handlers/deep_agents_handler.go` — 신규 핸들러
  - Accept header content negotiation (`text/event-stream` vs JSON)
  - SSE writer + heartbeat goroutine start (handler 진입 시점,
    orchestrator 호출 전 — RDC-10)
  - Orchestrator 호출, 단계별 SSE 이벤트 emit
  - Context cancellation → `pipeline_cancelled` (RDC-3)
  - Buffered fallback (`?stream=false` or no SSE Accept) → JSON 응답
- `cmd/usearch-api/handlers/deep_agents_handler_test.go` — end-to-end
  integration via `httptest`
- Test count: 15-18 tests (SSE 이벤트 순서, 페이로드 검증, 모드 분기,
  fallback path, cancellation, heartbeat)

Acceptance:
- REQ-DEEP2-001, 007, 010, 011 covered
- `TestDeepHandlerRoutesModeAgentsToOrchestrator` GREEN
- `TestSSEEmitsAgentStartedAndCompletedForEachAgent` GREEN
- `TestSSESectionEventsOnlyAfterVerifierPass` GREEN

### M6 — Prometheus Metrics + Cardinality Safety  [Priority Medium]

- `internal/obs/metrics/deepagent.go` — 신규 collectors:
  - `DeepAgentDuration *prometheus.HistogramVec{agent, outcome}`
    (buckets per spec.md REQ-DEEP2-008)
  - `DeepAgentRetries *prometheus.CounterVec{agent}` (agent="writer"
    pre-declared only)
  - `DeepAgentVerifierGateResults *prometheus.CounterVec{result}`
    (result ∈ {pass, fail_uncited, fail_timeout} pre-declared)
- 라벨 값 pre-declaration via `.WithLabelValues(value).Add(0)` 패턴
  per SYN-004 `streamsynth.go:48-56`
- 기존 `usearch_deep_outcomes_total{outcome}` 확장: 신규 값 `empty_corpus`,
  `error_pipeline_failed` 추가 (DEEP-001 collector 재사용; 본 SPEC은
  pre-declaration만 수행)
- `internal/obs/metrics/metrics.go` — `registerDeepAgent(pr)` 헬퍼 추가
- `internal/obs/obs.go` — re-export `obs.DeepAgentDuration`,
  `obs.DeepAgentRetries`, `obs.DeepAgentVerifierGateResults`
- `internal/obs/metrics/deepagent_test.go` — 카디널리티 검증, 등록 검증
- 기존 `metrics_test.go::TestNoUnboundedLabels` (allowlist) 변경
  여부 검증 — 신규 label NAME (`result`)이 allowlist에 이미 있거나
  추가해야 함 (NFR-DEEP2-002)
- Test count: 6-8 tests

Acceptance:
- REQ-DEEP2-008, NFR-DEEP2-002 covered
- `TestThreeNewCollectorsRegisteredAtStartup` GREEN
- `TestCardinalityGuardRemainsGreen` GREEN

### M7 — End-to-End Integration + Documentation  [Priority Medium]

- End-to-end happy-path test from `httptest.Server` → handler →
  orchestrator → mocked agents → SSE stream assertions
- End-to-end failure-path tests: retry, max-retry exhaustion, empty
  corpus, cancellation
- `.env.example` 업데이트 (spec.md §6 env-var 8개 + 기존 inherited)
- `services/researcher/README.md` 업데이트 (신규 `/faithfulness_check`
  endpoint 문서화)
- HISTORY 항목 추가 (spec.md HISTORY): v0.1.0 → implemented 전이는
  /moai sync 단계에서 처리
- Test count: 8-10 tests (e2e scenarios from acceptance.md)

Acceptance:
- acceptance.md의 모든 Scenario 1-8 + Edge Case 1-4 GREEN
- Backward compat 회귀: DEEP-001 acceptance suite 100% green
- Coverage: `go test -coverprofile ./internal/deepagent/...
  ./internal/synthesis/... ./cmd/usearch-api/handlers/...` ≥ 85%
- `services/researcher/tests/` coverage ≥ 85% on new files

---

## 3. TDD Plan (RED-phase Test Catalog)

TDD methodology requires RED tests written BEFORE implementation.
다음 목록은 본 SPEC의 EARS 요구사항을 검증할 테스트 이름의
exhaustive catalog다. Go test 명명 규약(`TestXxx`)을 따른다.

### 3.1 Endpoint Module (REQ-001, 010, 011)

1. `TestDeepHandlerRoutesModeAgentsToOrchestrator` — `?mode=agents`
   시 신규 핸들러로 라우팅됨을 검증
2. `TestDeepHandlerDefaultsToModeStorm` — `?mode=` 없으면 DEEP-001
   path로 라우팅됨
3. `TestDeepHandlerRequestSchemaMatchesContract` — `{request_id,
   query, lang}` 본문 파싱; `docs[]` 무시
4. `TestDeepHandlerBufferedFallbackReturnsFinalReport` —
   `?stream=false`로 JSON 응답
5. `TestDeepHandlerBufferedFallbackHasNoSSEOverhead` — SSE writer
   constructor mock 호출 횟수 0
6. `TestDeepHandlerStormModeUnchanged` — DEEP-001 acceptance 회귀
7. `TestDeepHandlerModeAbsentDefaultsToStorm` — REQ-DEEP2-011 default
8. `TestDeepHandlerNoSharedMutableStateBetweenModes` — package-level
   var grep + mode-switching race test

### 3.2 Pipeline Module (REQ-002, 003, 005, 009a, 009b, 012)

9. `TestOrchestratorRunsAgentsInOrder` — Researcher → Reviewer →
   Writer → Verifier 호출 순서 (mock spy로 검증)
10. `TestOrchestratorHaltsOnContextCancelBetweenAgents` —
    `ctx.Cancel()` 후 다음 agent 미호출
11. `TestReviewerDoesNotCallFanout` — Reviewer 구현에서 `fanout`
    import 부재 또는 mock 호출 횟수 0
12. `TestWriterDoesNotCallFanout` — 동일 패턴
13. `TestOrchestratorRetriesWriterOnVerifierReject` — Verifier mock이
    1번째 호출에서 fail → Writer 2번째 호출 발생
14. `TestOrchestratorEmitsRetryStartedBeforeRetryCall` — SSE 이벤트
    순서: `retry_started` < Writer 2nd call
15. `TestRetriesCounterIncrementsExactlyOncePerRetry` — Prometheus
    counter 증가량 검증
16. `TestNonVerifierErrorsDoNotTriggerRetry` — Researcher mock이
    에러 반환 → orchestrator immediate abort, Writer 미호출 (REQ-DEEP2-009b)
17. `TestResearcherCallsFanoutDispatchExactlyOnce` — mock spy
18. `TestResearcherUsesNoOtherRetrievalSource` — `internal/deepagent`
    패키지 grep로 retrieval 관련 import 부재 검증 (`http.Client`,
    `internal/adapters/*` 등)
19. `TestResearcherDocsAreImmutableInDownstream` — Researcher 출력의
    slice가 downstream에서 mutate되지 않음 (race test)
20. `TestMaxRetryExhaustionReturns503` — 3회 Writer 호출 모두 fail →
    HTTP 503 (REQ-DEEP2-009a)
21. `TestMaxRetryExhaustionEmitsPipelineFailedSSE` — terminal 이벤트
    (REQ-DEEP2-009a)
22. `TestResearcherErrorAbortsAndReturns503` — Researcher mock error
    (REQ-DEEP2-009b)
23. `TestVerifierErrorAbortsAndReturns503` — Verifier 자체 error
    (faithfulness endpoint 5xx) (REQ-DEEP2-009b)
23a. `TestReviewerErrorAbortsAndReturns503` — Reviewer mock error
    (REQ-DEEP2-009b)
24. `TestErrorOutcomeCounterIncrementsExactlyOnce` —
    `usearch_deep_outcomes_total{outcome="error_pipeline_failed"}`
    (REQ-DEEP2-009a/009b 공유 collector)
25. `TestEmptyFanoutShortCircuitsPipeline` — `Result.Docs == []` 시
    Reviewer/Writer/Verifier 미호출
26. `TestEmptyFanoutResponseShape` — JSON body 구조
27. `TestEmptyFanoutOutcomeCounterIncrements` —
    `usearch_deep_outcomes_total{outcome="empty_corpus"}` += 1
28. `TestEmptyFanoutSSEEventSequence` — `agent_started{researcher}` →
    `agent_completed{researcher, empty_corpus}` → `done{total_sections: 0}`

### 3.3 LLM Routing Module (REQ-004)

29. `TestConfigLoadsAllFourModelAliasesFromEnv` — 4개 env-var 설정 후
    Config 검증
30. `TestConfigFallsBackToDefaultsWhenEnvAbsent` — env-var unset 시
    기본값 적용
31. `TestAgentsReceiveResolvedModelFromOrchestrator` — orchestrator가
    각 agent 호출 시 `cfg.<RoleModel>` 전달
32. `TestNoDirectOsGetenvInAgentsPackage` — `internal/deepagent/`
    packages grep로 `os.Getenv` 호출 부재 검증 (`config.go` 제외)
33. `TestAllAgentsCallSingletonLLMClient` — `llm.Client` mock spy로
    호출 검증

### 3.4 Verifier Gate Module (REQ-006)

34. `TestVerifierCallsCheckFaithfulnessExactlyOnce` — mock spy
35. `TestVerifierPassWhenUncitedCountZero` — return `Pass: true`
36. `TestVerifierFailWhenUncitedCountPositive` — return `Pass: false`
    with feedback
37. `TestVerifierDoesNotPerformAdditionalScoring` — coverage/coherence
    LLM 호출 부재 (LLM mock 호출 횟수 == 1, faithfulness 전용)
38. `TestFaithfulnessEndpointReusesExistingSYN002Logic` — Python-side
    pytest: 신규 endpoint가 기존 `enforce_faithfulness` 함수 호출
39. `TestCheckFaithfulnessWrapperHandlesSidecar5xx` — Go wrapper
    error mapping
40. `TestVerifierGateResultsCounterIncrementsOncePerInvocation` —
    Prometheus counter

### 3.5 Streaming & Observability Module (REQ-007, 008)

41. `TestSSEEmitsAgentStartedAndCompletedForEachAgent` — 8 events
    minimum (4 agent × {started, completed})
42. `TestSSEEmitsRetryStartedBeforeWriterRetry` — 이벤트 순서
43. `TestSSEEmitsVerifierResultPerVerifierInvocation` — Verifier 호출
    횟수 == `verifier_result` 이벤트 개수
44. `TestSSEEmitsPipelineFailedOnExhaustion` — REQ-009 terminal event
45. `TestSSEEmitsPipelineCancelledOnContextCancel` — REQ-002 path
46. `TestSSESectionEventsOnlyAfterVerifierPass` — section_start 이벤트
    timestamp > 모든 agent_completed{verifier} 이벤트 timestamp
47. `TestSSEHeartbeatContinuesDuringAgentPhases` — handler 진입 ~
    종료 동안 `: ping` 이벤트 ≥ 1 (시간 mock으로 fast-forward)
48. `TestAllNewEventPayloadsCarrySchemaVersionAndRequestId` — 모든
    신규 payload 검증
49. `TestThreeNewCollectorsRegisteredAtStartup` — `registerDeepAgent`
    호출 후 prometheus registry 검증
50. `TestAllAgentLabelValuesPreDeclaredAtRegistration` — pre-declare
    pattern 검증
51. `TestNoLabelValueDerivedFromUserInput` — grep `WithLabelValues`
    호출에 외부 input 변수 부재
52. `TestDeepOutcomesExtendedWithEmptyCorpusAndPipelineFailed` —
    기존 collector에 신규 라벨 값 등록 확인
53. `TestCardinalityGuardRemainsGreen` —
    `metrics_test.go::TestNoUnboundedLabels` 통과

### 3.6 End-to-End / Property Tests (NFR-DEEP2-001, 003)

54. `TestE2EHappyPathReturns200WithCompleteStream` (httptest)
55. `TestE2ERetryPathRecordsCorrectMetricsAndEvents`
56. `TestE2ELatencyP95Under60Seconds` (mocked-LM 50 iterations)
57. `TestDeep001AcceptanceSuiteRemainsGreen` (regression umbrella —
    DEEP-001 test suite 호출)

총 테스트 카운트 추정: **57개** (TDD RED phase에서 모두 fail
상태로 작성).

---

## 4. Risks and Mitigations

research.md §7의 10개 risk를 본 SPEC에서 어떻게 다루는지 정리한다.
spec.md §1.3 Resolved Design Choices (RDC-1 ~ RDC-10) 와 일대일
대응한다.

| Risk # | Risk | Resolution | Mitigation |
|--------|------|------------|------------|
| R1 | Verifier SYN-002 Go-side wrapper 미존재 | (a) Resolved in REQ-DEEP2-006 | 신규 Go 함수 `internal/synthesis.CheckFaithfulness` + Python 사이드카 신규 endpoint `POST /faithfulness_check`; 기존 `faithfulness.py` 로직 재사용으로 single source of truth 보존 |
| R2 | Reviewer 역할 정의 부족 | (a) Resolved in REQ-DEEP2-002, RDC-2 | Reviewer는 critique-only, fanout 미호출, Researcher evidence 한정; 테스트 #11이 enforce |
| R3 | Context deadline propagation 불명확 | (a) Resolved in REQ-DEEP2-002, RDC-3 | Orchestrator가 매 agent 호출 직전 `ctx.Err()` 확인, `pipeline_cancelled` SSE 이벤트 후 종료; 부분 결과 응답 누출 금지 |
| R4 | Writer retry semantics off-by-one 위험 | (a) Resolved in REQ-DEEP2-003, RDC-4 | "max 2 retries" = 초기 1 + 재시도 2 = 총 3회 상한; `cfg.MaxRetries = 2`로 hardcoded; 테스트 #13, #20이 enforce |
| R5 | Model alias resolution 분산 | (a) Resolved in REQ-DEEP2-004, RDC-5 | `internal/deepagent/config.go`에서 중앙집중식 로딩, agent별 `os.Getenv` 직접 호출 금지 (테스트 #32) |
| R6 | Streaming retry semantics (섹션 재emit) | (a) Resolved in REQ-DEEP2-007, RDC-6 | Writer/Verifier 루프 사전버퍼링 — section/sentence 이벤트는 Verifier PASS 후에만 emit; 같은 section_index 중복 콘텐츠 불가능; 테스트 #46 enforce |
| R7 | 글로벌 per-request 비용 cap 부재 | (c) Deferred to SPEC-DEEP-004 | 본 SPEC은 cost 메트릭 가시화만 수행 (NFR-DEEP2-004); 명시적 cap enforcement는 spec.md §8 Exclusions에 deferral 명시 |
| R8 | 메트릭 카디널리티 unbounded 위험 | (a) Resolved in NFR-DEEP2-002, RDC-8 | Go enum-like type으로 컴파일 타임 enforcement; pre-declaration; `metrics_test.go::TestNoUnboundedLabels` 회귀 게이트 |
| R9 | Error propagation 정책 불명확 | (a) Resolved in REQ-DEEP2-003, REQ-DEEP2-009a, REQ-DEEP2-009b, RDC-9 | Verifier rejection만 Writer 재시도 트리거 (max-retry exhaustion은 REQ-009a path); 다른 모든 agent 에러는 즉시 abort + 503 (REQ-009b path); 테스트 #16, #22, #23, #23a enforce |
| R10 | Heartbeat 시작 타이밍 | (b) Acceptable runtime decision | Handler 진입 시점 시작이 권장; 정확한 타이밍은 구현 단계에서 결정. 테스트 #47이 "agent 단계 중 heartbeat 발생" 만 검증, 정확한 횟수는 미강제 |

추가 구현-단계 리스크 (research.md에 없음, 본 plan에서 식별):

| Risk | Mitigation |
|------|------------|
| Writer prompt regression으로 인한 retry 폭증 | NFR-DEEP2-004의 `usearch_deep_agent_retries_total{agent="writer"}` 메트릭으로 모니터링; 운영 단계에서 retry 비율 5% 초과 시 alert (본 SPEC 범위 외, runbook 책임) |
| Faithfulness endpoint timeout 회귀 | `DEEP_AGENT_VERIFIER_TIMEOUT_MS=30000` 기본값; SYN-002 enforcement 자체 latency가 30s 초과하지 않음을 SYN-002 본 spec에서 보장 |
| DEEP-001 코드 변경에 의한 회귀 | REQ-DEEP2-011 + NFR-DEEP2-003 + 테스트 #6, #57이 회귀 방지; `cmd/usearch-api/handlers/synthesis.go`의 mode dispatch만 add-only로 수정 |

---

## 5. Reference Implementations

research.md §8과 §2를 기반으로, 신규 파일별 가장 가까운 analog
파일을 식별한다. 구현자는 해당 analog의 패턴을 mimic하되, 동일한
import 경로를 사용하지는 않는다.

| New File | Closest Analog | Pattern to Reuse |
|----------|---------------|------------------|
| `internal/deepagent/orchestrator.go` | `internal/deepreport/client.go` | Retry loop with bounded attempts, error sentinel mapping, observability emission wrapper |
| `internal/deepagent/agents.go` (Researcher) | (precedent implicit from FAN-001 spec) | `fanout.Dispatch()` 호출 패턴, `Result.Docs` 소비 |
| `internal/deepagent/agents.go` (Writer) | `internal/synthesis/client.go` `Synthesize()` | LLM 호출, structured output 파싱, citation marker assembly |
| `internal/deepagent/agents.go` (Verifier) | `internal/synthesis/client.go` `emitObs` | Sidecar HTTP call wrapper, observability span |
| `internal/synthesis/faithfulness.go` | `internal/synthesis/client.go` | HTTP client with retry, ctx propagation, error sentinel |
| `internal/deepagent/metrics.go` | `internal/obs/metrics/deepreport.go` | Histogram + CounterVec collectors, `.WithLabelValues(...).Add(0)` pre-declaration |
| `internal/deepagent/sse.go` | `internal/streamsynth/longform.go` | `sse.Writer` 호출, JSON payload marshal, frame emit |
| `internal/streamsynth/agent_events.go` | `internal/streamsynth/longform.go` `SectionStartPayload` | Payload struct definition, JSON tags, `schema_version` 필드 |
| `cmd/usearch-api/handlers/deep_agents_handler.go` | `cmd/usearch-api/handlers/synthesis.go` | HTTP handler scaffolding, content negotiation, SSE response setup |
| `services/researcher/src/researcher/faithfulness_endpoint.py` | `services/researcher/src/researcher/app.py` | FastAPI route registration, Pydantic request/response models |

---

## 6. MX Tag Plan (Phase 3.5 Input)

GREEN phase에서 적용할 @MX 태그 후보. Run phase의 manager-tdd가
이 목록을 기반으로 GREEN-REFACTOR 사이에 태그를 add/update한다.
`code_comments: en` 설정이므로 모든 태그 description은 영어.

### 6.1 @MX:ANCHOR Candidates (Invariant Contracts)

- `internal/deepagent/orchestrator.go:RunPipeline()` — orchestrator
  entry point; pipeline의 모든 호출이 이 함수로 유입
  - `@MX:ANCHOR: Single entry for 4-agent pipeline orchestration`
  - `@MX:REASON: All /deep?mode=agents requests funnel here; agent
    ordering invariant (Researcher → Reviewer → Writer → Verifier)
    enforced; retry loop bounded`
  - `@MX:SPEC: SPEC-DEEP-002 REQ-DEEP2-002, REQ-DEEP2-003`
- `internal/synthesis/faithfulness.go:CheckFaithfulness()` —
  Go-side wrapper for SYN-002 enforcement
  - `@MX:ANCHOR: Single chokepoint for SYN-002 faithfulness gate
    invocation from Go side`
  - `@MX:REASON: Verifier agent (DEEP-002) is the only caller in
    v0; future SPECs may add additional callers but contract
    (UncitedSentencesCount → PASS/FAIL binary) is FROZEN`
  - `@MX:SPEC: SPEC-SYN-002, SPEC-DEEP-002 REQ-DEEP2-006`
- `cmd/usearch-api/handlers/deep_agents_handler.go:ServeHTTP()` —
  HTTP entry point with content negotiation
  - `@MX:ANCHOR: HTTP boundary for /deep?mode=agents`
  - `@MX:REASON: Mode dispatch happens here; backward compat with
    DEEP-001 path requires careful negotiation`
  - `@MX:SPEC: SPEC-DEEP-002 REQ-DEEP2-001, REQ-DEEP2-011`

### 6.2 @MX:WARN Candidates (Danger Zones)

- `internal/deepagent/orchestrator.go:retryLoop()` — Writer retry
  cost surface
  - `@MX:WARN: Bounded retry creates cost amplification surface`
  - `@MX:REASON: Each retry invokes Writer (Sonnet tier) — without
    SPEC-DEEP-004 quota enforcement, max 3 Sonnet calls per request
    is the only cost ceiling; monitor
    usearch_deep_agent_retries_total{agent="writer"}`
  - `@MX:SPEC: SPEC-DEEP-002 REQ-DEEP2-003, NFR-DEEP2-004`
- `internal/deepagent/sse.go:EmitAgentEvent()` — SSE emission inside
  streaming goroutine
  - `@MX:WARN: Concurrent writes from main pipeline + heartbeat
    goroutine to shared sse.Writer`
  - `@MX:REASON: sse.Writer mutex must be held; race on connection
    teardown can leak goroutines; ctx propagation required`
  - `@MX:SPEC: SPEC-SYN-004, SPEC-DEEP-002 REQ-DEEP2-007`
- `internal/deepagent/orchestrator.go` (context cancellation block)
  - `@MX:WARN: Context cancellation between agents must release all
    in-flight LLM call HTTP bodies`
  - `@MX:REASON: defer resp.Body.Close() in llm.Client must be
    upheld even on ctx.Done(); orphaned LLM calls otherwise burn
    budget without observable progress`
  - `@MX:SPEC: SPEC-DEEP-002 REQ-DEEP2-002, RDC-3`

### 6.3 @MX:NOTE Candidates (Context Delivery)

- `internal/deepagent/prompts.go` (each prompt template constant)
  - `@MX:NOTE: Prompt template — semantic intent: <role>'s task is to
    <what>`
  - One @MX:NOTE per role (Researcher/Reviewer/Writer/Verifier)
- `internal/deepagent/types.go:Agent` constants
  - `@MX:NOTE: Enum-like type; bounded label values for Prometheus
    cardinality safety (NFR-DEEP2-002)`
- `internal/deepagent/agents.go` Verifier→Writer feedback adapter
  - `@MX:NOTE: Verifier feedback to Writer retry hint conversion;
    structure: uncited sentence list → bullet-pointed prompt suffix`
- `internal/deepagent/config.go`
  - `@MX:NOTE: All DEEP_AGENT_* env-vars loaded here; individual
    agents must not call os.Getenv (REQ-DEEP2-004 RDC-5)`

### 6.4 @MX:TODO Candidates (Iteration Markers)

- M1 RED phase: `@MX:TODO: SPEC-DEEP-002 REQ-DEEP2-004 — implement
  Config struct loading; tests in config_test.go pending`
- M2 RED phase: similar for each milestone's RED tests
- All @MX:TODO removed in respective milestone's GREEN phase

### 6.5 Phase 3.5 Report Template

GREEN-REFACTOR 종료 시 manager-tdd가 다음 형식의 report 생성:

```
## @MX Tag Report — SPEC-DEEP-002 Run Phase — <timestamp>

### Tags Added (N)
- internal/deepagent/orchestrator.go:RunPipeline — @MX:ANCHOR
- internal/deepagent/orchestrator.go:retryLoop — @MX:WARN
- (etc.)

### Tags Removed (N)
- @MX:TODO in M1-M7 milestones (resolved in GREEN)

### Attention Required
- None expected; if cyclomatic complexity > 15 detected in
  orchestrator.go, add @MX:WARN
```

---

*End of SPEC-DEEP-002 plan v0.1.1.*
