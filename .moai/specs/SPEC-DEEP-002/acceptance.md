---
id: SPEC-DEEP-002
version: 0.1.2
created: 2026-05-21
author: limbowl (via manager-spec)
companion_to: spec.md
---

# SPEC-DEEP-002 Acceptance Criteria

Companion artifact for `.moai/specs/SPEC-DEEP-002/spec.md`.

본 문서는 SPEC-DEEP-002의 acceptance scenarios를 Given/When/Then
형식으로 정의한다. 각 scenario는 plan.md §3 TDD Plan의 테스트 이름과
교차참조된다. 테스트가 모두 GREEN 상태일 때 SPEC은 implementation
완료로 간주된다.

---

## 1. Definition of Done

SPEC-DEEP-002는 다음 항목이 모두 충족될 때 **DONE** 상태로 전이한다:

- [ ] `internal/deepagent/` 패키지 신규 생성, 7개 파일 (orchestrator.go,
      agents.go, types.go, prompts.go, config.go, metrics.go, sse.go)
      + 3개 test 파일
- [ ] `internal/synthesis/faithfulness.go` + 테스트 파일 생성
- [ ] `internal/streamsynth/agent_events.go` + 테스트 파일 생성
- [ ] `services/researcher/src/researcher/faithfulness_endpoint.py`
      + 테스트 파일 생성
- [ ] `cmd/usearch-api/handlers/deep_agents_handler.go` + 테스트 파일 생성
- [ ] `cmd/usearch-api/handlers/synthesis.go` 의 `?mode=` 라우팅 추가
- [ ] `internal/obs/metrics/deepagent.go` + 테스트 파일; 3개 신규
      collectors 등록
- [ ] `internal/obs/metrics/metrics.go` `registerDeepAgent` 추가
- [ ] `internal/obs/obs.go` re-export 추가
- [ ] `.env.example` `DEEP_AGENT_*` 8개 env-var 문서화
- [ ] 모든 15개 EARS REQ (REQ-DEEP2-001 ~ 008, 009a-SSE, 009a-Buffered, 009b-SSE, 009b-Buffered, 010 ~ 012) 에 대응하는 GREEN 테스트 존재
- [ ] 4개 NFR (NFR-DEEP2-001 ~ 004) 검증 테스트 존재
- [ ] `go test -race ./internal/deepagent/... ./internal/synthesis/...
      ./internal/streamsynth/... ./internal/obs/metrics/...
      ./cmd/usearch-api/...` 통과
- [ ] Coverage ≥ 85% on all new Go files
- [ ] `pytest --cov=researcher services/researcher/tests/` ≥ 85% on
      new Python files
- [ ] SPEC-DEEP-001 acceptance suite 100% green (regression)
- [ ] SPEC-SYN-002, SPEC-SYN-004 acceptance suites 100% green (regression)
- [ ] TRUST 5 gates passed (tested, readable, unified, secured, trackable)
- [ ] LSP gates passed (zero errors, zero type errors, zero lint errors
      per `quality.yaml` run-phase thresholds)
- [ ] @MX tags applied per plan.md §6
- [ ] Conventional commit `feat(deep): SPEC-DEEP-002 multi-agent
      pipeline` references this SPEC ID
- [ ] Pre-submission self-review confirms no simpler approach achieves
      the same result
- [ ] **Prod-latency smoke test executed during /moai sync** (NFR-DEEP2-001
      budget (b), P-M2): end-to-end p95 ≤ 60 s measured on staging
      deployment against a corpus of 20–50 fanout docs with Verifier
      passing first or second attempt. Pass threshold: p95 ≤ 60 s
      across at least 20 sample requests. Fail threshold: p95 > 60 s
      triggers SPEC re-evaluation (NOT a unit test — operational gate)

---

## 2. Acceptance Scenarios (Given-When-Then)

### Scenario 1 — Happy path: Verifier PASS on first attempt, full SSE stream

**Given**
- `DEEP_AGENT_MAX_RETRIES=2`, 4개 model alias env-var 기본값
- `POST /deep?mode=agents` 요청, 본문 `{request_id: "req-001",
  query: "What are the tradeoffs of GraphQL vs REST?", lang: "en"}`
- 요청 헤더 `Accept: text/event-stream`
- Mocked `fanout.Dispatch()`이 25개의 valid NormalizedDoc 반환
- Mocked `llm.Client.Complete()` (Haiku, Sonnet 둘 다)이 정상 응답
- Mocked researcher 사이드카 `POST /faithfulness_check`이
  `{uncited_sentences_count: 0, uncited_sentences: [], outcome_ok: true}` 반환

**When**
- 핸들러가 요청을 처리

**Then**
- HTTP 응답 헤더: `Content-Type: text/event-stream`,
  `Cache-Control: no-cache`, `Connection: keep-alive`
- SSE 이벤트 시퀀스(순서대로):
  1. `agent_started{agent: "researcher"}`
  2. `agent_completed{agent: "researcher", outcome: "success"}`
  3. `agent_started{agent: "reviewer"}`
  4. `agent_completed{agent: "reviewer", outcome: "success"}`
  5. `agent_started{agent: "writer"}`
  6. `agent_completed{agent: "writer", outcome: "success"}`
  7. `agent_started{agent: "verifier"}`
  8. `verifier_result{result: "pass"}`
  9. `agent_completed{agent: "verifier", outcome: "success"}`
  10. (이후 SYN-004 sequence:) `section_start{section_index: 0}`,
      `sentence ×N`, `section_done{section_index: 0}`, ... per section
  11. `done{request_id, total_sections, total_sentences, latency_ms,
      cost_usd, schema_version: 1}`
- 모든 신규 이벤트 payload는 `schema_version: 1` 및 `request_id` 필드 포함
- Counter `usearch_deep_agent_duration_seconds{agent="researcher",
  outcome="success"}` += 1 (각 agent별)
- Counter `usearch_deep_agent_verifier_gate_results_total{result="pass"}` += 1
- Counter `usearch_deep_agent_retries_total{agent="writer"}` 변화 없음
- Counter `usearch_deep_outcomes_total{outcome="success"}` += 1
- 모든 4개 agent가 호출됨; 호출 순서는 Researcher → Reviewer → Writer → Verifier
- `fanout.Dispatch()` 정확히 1회 호출됨 (REQ-DEEP2-005)
- Tests asserting this scenario: `TestE2EHappyPathReturns200WithCompleteStream`,
  `TestOrchestratorRunsAgentsInOrder`, `TestSSEEmitsAgentStartedAndCompletedForEachAgent`,
  `TestSSESectionEventsOnlyAfterVerifierPass`

### Scenario 2 — Retry path: Verifier rejects iter 1, PASS on iter 2

**Given**
- 동일 설정 (Scenario 1과 같음)
- Mocked `POST /faithfulness_check`이 1번째 호출에서
  `{uncited_sentences_count: 3, uncited_sentences: ["...", "...", "..."],
  outcome_ok: false}` 반환, 2번째 호출에서 `outcome_ok: true` 반환

**When**
- 핸들러가 요청을 처리

**Then**
- SSE 이벤트 시퀀스에 다음이 포함됨:
  - `agent_started{verifier}` → `verifier_result{result: "fail_uncited"}`
    → `agent_completed{verifier, outcome: "success"}` (Verifier 자체는 정상
    동작; rejection은 outcome="success"로 처리되며 result 라벨이 fail_uncited)
  - `retry_started{agent: "writer", retry_count: 1, reason:
    "verifier_rejection"}`
  - `agent_started{writer}` (2nd attempt) →
    `agent_completed{writer, outcome: "success"}`
  - `agent_started{verifier}` (2nd Verifier call) →
    `verifier_result{result: "pass"}` →
    `agent_completed{verifier, outcome: "success"}`
  - 이후 SYN-004 section/sentence/done sequence
- Counter `usearch_deep_agent_retries_total{agent="writer"}` += 1
- Counter `usearch_deep_agent_verifier_gate_results_total{result="fail_uncited"}` += 1
- Counter `usearch_deep_agent_verifier_gate_results_total{result="pass"}` += 1
- `writer` agent의 `usearch_deep_agent_duration_seconds` histogram에
  2개의 관측값 기록 (1st와 2nd Writer 호출)
- Counter `usearch_deep_outcomes_total{outcome="success"}` += 1 (최종)
- Tests: `TestE2ERetryPathRecordsCorrectMetricsAndEvents`,
  `TestOrchestratorRetriesWriterOnVerifierReject`,
  `TestOrchestratorEmitsRetryStartedBeforeRetryCall`,
  `TestRetriesCounterIncrementsExactlyOncePerRetry`,
  `TestSSEEmitsRetryStartedBeforeWriterRetry`

### Scenario 3 — Max-retry exhaustion (split: SSE vs Buffered per P-B2)

본 시나리오는 stream state에 따라 두 갈래로 분리된다.

#### Scenario 3-SSE — SSE 활성 상태에서 max-retry exhaustion (REQ-DEEP2-009a-SSE)

**Given**
- 동일 설정 (Accept: text/event-stream — SSE 활성, response headers
  이미 flushed)
- Mocked `POST /faithfulness_check`이 모든 3회 호출에서
  `{uncited_sentences_count: 2, outcome_ok: false}` 반환

**When**
- 핸들러가 요청을 처리

**Then**
- Writer가 정확히 3회 호출됨 (초기 1 + 재시도 2)
- Verifier가 정확히 3회 호출됨
- SSE 이벤트 마지막은 `pipeline_failed{failed_agent: "writer",
  reason: "verifier_rejection_exhausted", attempts: 3, uncited_count: 2,
  retry_count: 2}`
- HTTP 상태는 **200으로 유지** (response headers 이미 flushed;
  SSE 프로토콜이 retroactive status code change 금지). 핸들러는
  retroactive status change 시도 없이 stream 종료 후 connection close
- Client는 마지막 SSE 이벤트로 failure 인지
- Counter `usearch_deep_agent_retries_total{agent="writer"}` += 2
- Counter `usearch_deep_agent_verifier_gate_results_total{result="fail_uncited"}` += 3
- Counter `usearch_deep_outcomes_total{outcome="error_pipeline_failed"}` += 1
- Tests: `TestSSEMaxRetryEmitsTerminalEvent`,
  `TestSSEMaxRetryHttpStatusStays200`,
  `TestErrorOutcomeCounterIncrementsExactlyOnce`

#### Scenario 3-Buffered — Buffered 응답에서 max-retry exhaustion (REQ-DEEP2-009a-Buffered)

**Given**
- 동일 설정 단 `?stream=false` 또는 `Accept`이 text/event-stream을
  명시하지 않음 (response headers 아직 flushed 되지 않은 상태)
- 동일하게 `POST /faithfulness_check`이 3회 모두 fail 반환

**When**
- 핸들러가 요청을 처리

**Then**
- Writer 3회, Verifier 3회 호출 (3-SSE와 동일)
- HTTP **503** with `Content-Type: application/json` body:
  ```json
  {
    "error": "pipeline_failed",
    "detail": "Writer exhausted 3 attempts; Verifier still rejected",
    "uncited_count": 2,
    "attempts": 3,
    "retry_count": 2
  }
  ```
- SSE writer/heartbeat goroutine constructor 호출 카운트: 0
- Counter 증감은 3-SSE와 동일
- Tests: `TestBufferedMaxRetryReturns503`,
  `TestBufferedMaxRetryJsonBodyShape`,
  `TestErrorOutcomeCounterIncrementsExactlyOnce`

### Scenario 4 — Context cancellation mid-pipeline

**Given**
- 동일 설정
- 핸들러 진입 후 Researcher 완료, Reviewer 진행 중에 client가
  HTTP connection close (TCP RST) → `r.Context().Done()` fires

**When**
- Orchestrator가 Reviewer 결과 수신 후 Writer 호출 직전 `ctx.Err()` 확인

**Then**
- Writer/Verifier 미호출
- SSE 마지막 이벤트는 `pipeline_cancelled{at_agent: "writer"}`
  (P-M1 semantics: Reviewer 완료 직후 inter-agent boundary에서 cancel
  감지 시 next-would-be agent = writer; 만약 cancel이 어떤 agent의
  LLM mid-flight 시점에 감지되면 in-progress agent 이름 사용)
- 응답 connection은 이미 client side에서 close; server는 stream 종료
- Counter `usearch_deep_outcomes_total` 증가 없음 (cancel은 zero-or-one,
  SYN-004의 `usearch_syn004_outcomes_total{outcome="client_disconnect"}` += 1)
- Goroutine leak 없음 (race detector test)
- LLM Client mock의 호출 카운트: Researcher 1, Reviewer 1, Writer 0,
  Verifier 0 (orphan LLM call 없음)
- Tests: `TestOrchestratorHaltsOnContextCancelBetweenAgents`,
  `TestSSEEmitsPipelineCancelledOnContextCancel`

### Scenario 5 — Mode coexistence: `/deep?mode=storm` unchanged

**Given**
- DEEP-002 코드가 main branch에 머지된 상태
- `POST /deep?mode=storm` 요청 (DEEP-001 contract 준수: `{request_id,
  query, lang, docs: [...]}` 본문)
- `Accept: text/event-stream`

**When**
- 핸들러가 요청을 처리

**Then**
- Routing이 DEEP-001 path (`internal/deepreport`) 로 분기
- DEEP-002 코드 (`internal/deepagent/`) 미호출
- 응답이 DEEP-001 spec.md의 SSE sequence와 **schema-identical AND
  semantically equivalent** (P-M6): same event types in same order,
  same field names per event, same field types. Non-deterministic
  fields (`request_id`, timestamps, `duration_ms`, `latency_ms`,
  `cost_usd`) MAY differ in value across runs
- DEEP-001 acceptance suite Scenario 1 (Happy path: structured report
  with 3 sections, 12 sentences, 14 cited sources)가 100% green 상태로
  재실행 가능
- Counter `usearch_deep_outcomes_total{outcome="success"}` += 1 (DEEP-001
  emits this; DEEP-002 path는 같은 collector를 공유)
- Counter `usearch_deep_agent_*` 변화 없음 (DEEP-002 collectors는
  `mode=agents`에서만 활성)
- 동일하게 `?mode=` parameter가 absent인 요청은 `mode=storm` default로
  처리됨 (REQ-DEEP2-011)
- Tests: `TestDeepHandlerStormModeUnchanged`,
  `TestDeepHandlerModeAbsentDefaultsToStorm`,
  `TestDeepHandlerNoSharedMutableStateBetweenModes`,
  `TestDeep001AcceptanceSuiteRemainsGreen`,
  `TestStormModeResponseSchemaIdenticalPrePostDeep002`

### Scenario 6 — Buffered fallback: `?stream=false`

**Given**
- 동일 설정 (Scenario 1)
- 요청 URL: `POST /deep?mode=agents&stream=false`
- `Accept` 헤더 부재 또는 `application/json`

**When**
- 핸들러가 요청을 처리

**Then**
- SSE writer/heartbeat goroutine constructor 호출 카운트: 0
- HTTP 응답 헤더: `Content-Type: application/json` (NOT text/event-stream)
- HTTP 200 응답 본문 (JSON):
  ```
  {
    "request_id": "req-001",
    "final": {
      "sections": [...],
      "citations": [...],
      "model": "claude-3-5-sonnet-20241022",
      "provider": "anthropic",
      "cost_usd": <total>,
      "latency_ms": <total>
    },
    "agent_log": [
      {"agent": "researcher", "outcome": "success", "duration_ms": ...},
      {"agent": "reviewer", "outcome": "success", "duration_ms": ...},
      {"agent": "writer", "outcome": "success", "duration_ms": ...},
      {"agent": "verifier", "outcome": "success", "duration_ms": ...,
       "result": "pass"}
    ],
    "schema_version": 1
  }
  ```
- 본문은 SSE 케이스의 누적된 `agent_completed` payload + 최종 `done`
  payload를 합친 형태와 정보적으로 동등 (information-equivalent)
- Tests: `TestDeepHandlerBufferedFallbackReturnsFinalReport`,
  `TestDeepHandlerBufferedFallbackHasNoSSEOverhead`

### Scenario 7 — Empty fanout corpus: short-circuit happy path

**Given**
- 동일 설정 (Scenario 1)
- Mocked `fanout.Dispatch()`이 `Result{Docs: [], Stats: {...},
  AdapterErrors: nil}` 반환 (adapters 정상 동작하나 모든 결과가 0건)

**When**
- 핸들러가 요청을 처리

**Then**
- Researcher가 fanout 결과를 확인 — empty 시 자체 LLM 호출을 **SKIP**하고
  (P-M3: 추출할 claim 없음 + 비용/지연 절감), `ResearcherOutput{IsEmpty:
  true}` 즉시 반환
- Orchestrator가 Reviewer/Writer/Verifier 미호출 (short-circuit)
- LLM Client mock의 호출 카운트: 0 (Researcher LLM도 호출하지 않음 —
  REQ-DEEP2-012 P-M3에 의해 명시적으로 SKIP)
- HTTP 200 응답 본문 (JSON, `?stream=false` or SSE both):
  ```
  {
    "request_id": "req-001",
    "final": {"sections": [], "citations": []},
    "agent_log": [{"agent": "researcher", "outcome": "empty_corpus"}],
    "schema_version": 1
  }
  ```
- SSE 케이스: 이벤트 시퀀스는
  - `agent_started{researcher}`
  - `agent_completed{researcher, outcome: "empty_corpus"}`
  - `done{total_sections: 0, total_sentences: 0, schema_version: 1}`
  (`pipeline_failed` 이벤트 emit 하지 않음 — empty corpus는 error가
  아닌 degenerate but non-error outcome)
- Counter `usearch_deep_outcomes_total{outcome="empty_corpus"}` += 1
- Counter `usearch_deep_agent_duration_seconds{agent="researcher",
  outcome="success"}` += 1 (researcher 자체는 정상 종료 — histogram
  outcome 라벨은 bounded enum `{success, error}`에 한정되므로
  `empty_corpus`가 아닌 `success`. P-M3 clarification: Prometheus
  label vs SSE JSON field는 별개 시스템 — SSE의 `agent_completed.outcome`
  필드가 `"empty_corpus"` 값을 가지더라도 histogram 라벨은 `success` 유지)
- 다른 agent의 counter 증가 없음
- Tests: `TestEmptyFanoutShortCircuitsPipeline`,
  `TestEmptyFanoutResponseShape`,
  `TestEmptyFanoutOutcomeCounterIncrements`,
  `TestEmptyFanoutSSEEventSequence`,
  `TestEmptyFanoutResearcherSkipsLLMInvocation`,
  `TestEmptyFanoutHistogramOutcomeIsSuccess`

### Scenario 8 — Researcher error aborts pipeline (non-Verifier failure)

**Given**
- 동일 설정 (Scenario 1)
- Mocked `fanout.Dispatch()`이 정상 동작 (25개 docs)
- Mocked `llm.Client.Complete()` for Researcher가 first call에서
  `ErrUpstreamFailure` 반환

**When**
- 핸들러가 요청을 처리

**Then**
- Orchestrator가 Researcher error를 catch
- Reviewer/Writer/Verifier 미호출 (REQ-DEEP2-009b-SSE / REQ-DEEP2-009b-Buffered:
  non-Verifier errors do not trigger retry)
- Stream state에 따라 두 갈래:
  - SSE 활성 시: terminal `pipeline_failed{failed_agent: "researcher",
    reason: "upstream_llm_error"}` SSE event, HTTP 200 stays
    (REQ-DEEP2-009b-SSE)
  - Buffered 시: HTTP 503 응답 `{"error": "pipeline_failed",
    "detail": "researcher failed: upstream LLM error", "failed_agent":
    "researcher"}` (REQ-DEEP2-009b-Buffered)
- Counter `usearch_deep_agent_duration_seconds{agent="researcher",
  outcome="error"}` += 1
- Counter `usearch_deep_agent_retries_total{agent="writer"}` 변화 없음
- Counter `usearch_deep_outcomes_total{outcome="error_pipeline_failed"}` += 1
- Tests: `TestSSEResearcherErrorEmitsTerminalEvent`,
  `TestBufferedResearcherErrorReturns503`,
  `TestNonVerifierErrorsDoNotTriggerRetry`

---

## 3. Edge Cases

### Edge Case 1 — Reviewer LLM 호출 실패는 retry 트리거 없이 abort

**Given**
- Researcher 정상 완료
- Mocked `llm.Client.Complete()` for Reviewer가 transient 5xx 반환

**Then**
- Writer/Verifier 미호출 (Reviewer 자체에 retry 없음, REQ-DEEP2-009b-SSE / REQ-DEEP2-009b-Buffered)
- Stream state에 따라 두 갈래:
  - SSE 활성 시: terminal `pipeline_failed{failed_agent:"reviewer"}` SSE event, HTTP 200 stays (REQ-DEEP2-009b-SSE)
  - Buffered 시: HTTP 503 + JSON body (REQ-DEEP2-009b-Buffered)
- Counter `usearch_deep_agent_duration_seconds{agent="reviewer",
  outcome="error"}` += 1
- Test: `TestSSEReviewerErrorEmitsTerminalEvent`,
  `TestBufferedReviewerErrorReturns503`,
  `TestNonVerifierErrorsDoNotTriggerRetry` (parameterized over
  researcher/reviewer/verifier)

### Edge Case 2 — Verifier 자체 error (faithfulness endpoint 5xx)는 Writer retry 트리거 없이 abort

**Given**
- Writer 정상 완료, draft 생성
- Mocked `POST /faithfulness_check`이 HTTP 503 반환 (사이드카 자체
  에러; faithfulness verdict는 아님). 본 케이스는 timeout이 아닌 일반
  5xx infra error 임에 유의 — P-B1으로 인해 metric 라벨 이름이 변경됨.

**Then**
- Writer 재시도 미발생 (Verifier rejection이 아닌 Verifier infra error)
- Stream state에 따라 두 갈래 분기:
  - SSE 활성 시 (REQ-DEEP2-009b-SSE): terminal `pipeline_failed{
    failed_agent: "verifier", reason: "faithfulness_endpoint_unavailable"}`
    SSE event, HTTP 200 stays
  - Buffered 시 (REQ-DEEP2-009b-Buffered): HTTP 503 + JSON body
- Counter `usearch_deep_agent_verifier_gate_results_total{result="fail_error"}`
  += 1 (P-B1: `fail_error` covers all Verifier infra failures —
  timeouts, 5xx, transport errors, wrapper errors — distinct from
  the verdict `fail_uncited`)
- Counter `usearch_deep_outcomes_total{outcome="error_pipeline_failed"}` += 1
- Test: `TestSSEVerifierErrorEmitsTerminalEvent`,
  `TestBufferedVerifierErrorReturns503`,
  `TestCheckFaithfulnessWrapperHandlesSidecar5xx`

### Edge Case 3 — Env-var 부재 시 model alias 기본값 적용

**Given**
- 4개 `DEEP_AGENT_*_MODEL` env-var 모두 unset
- 다른 설정 정상

**Then**
- `Config.ResearcherModel == "claude-3-5-haiku-20241022"`
- `Config.ReviewerModel == "claude-3-5-haiku-20241022"`
- `Config.WriterModel == "claude-3-5-sonnet-20241022"`
- `Config.VerifierModel == "claude-3-5-sonnet-20241022"`
- 4개 agent의 `llm.Client.Complete()` 호출에 각 default 모델 alias 전달
- Tests: `TestConfigFallsBackToDefaultsWhenEnvAbsent`,
  `TestAgentsReceiveResolvedModelFromOrchestrator`

### Edge Case 4 — 동시 요청 간 mode 격리 (mode=storm + mode=agents 병렬)

**Given**
- 동시에 2개 요청 도착:
  - Request A: `POST /deep?mode=storm` (DEEP-001 path)
  - Request B: `POST /deep?mode=agents` (DEEP-002 path)
- 두 요청이 같은 `request_id` 사용 시도 (악의적 client)

**Then**
- 각 핸들러는 독립적 처리 (handler-level local state만 사용)
- Request A의 응답은 DEEP-001 spec.md sequence; Request B는 DEEP-002
- Prometheus collectors 간 상호간섭 없음: `mode=storm`은 DEEP-001 collector를,
  `mode=agents`는 DEEP-002 collector를 증가
- 단, `usearch_deep_outcomes_total{outcome="success"}`는 공유 collector
  이므로 둘 다 += 1 (이는 의도된 동작; 본 SPEC의 NFR-DEEP2-003 격리는
  package-level mutable state에 한정)
- `internal/deepagent` package가 `internal/deepreport`를 import하지
  않음을 grep으로 검증
- Tests: `TestDeepHandlerNoSharedMutableStateBetweenModes`,
  `TestE2ECoexistenceUnderConcurrentLoad`

---

## 4. Quality Gate Criteria

### 4.1 Code Quality

- [ ] Go: `gofmt -d internal/deepagent/ internal/synthesis/faithfulness.go
      internal/streamsynth/agent_events.go cmd/usearch-api/handlers/deep_agents_handler.go
      internal/obs/metrics/deepagent.go` empty diff
- [ ] Go: `golangci-lint run ./internal/deepagent/... ./internal/synthesis/...
      ./internal/streamsynth/... ./internal/obs/metrics/... ./cmd/usearch-api/...`
      exits 0
- [ ] Python: `ruff check services/researcher/src/researcher/faithfulness_endpoint.py
      services/researcher/tests/test_faithfulness_endpoint.py` exits 0
- [ ] Python: `ruff format --check` same paths empty diff
- [ ] 모든 `// nolint` 또는 `# type: ignore` 지시문에 `@MX:WARN: [AUTO]
      @MX:REASON: ...` 정당화 동반

### 4.2 Test Coverage

- [ ] `go test -coverprofile=cover.out ./internal/deepagent/...`
      coverage ≥ 85%
- [ ] `go test -coverprofile=cover.out ./internal/synthesis/...`
      coverage ≥ 85% (faithfulness.go에 한정)
- [ ] `pytest --cov=researcher services/researcher/tests/test_faithfulness_endpoint.py`
      coverage ≥ 85%
- [ ] 모든 66개 RED-phase test (plan.md §3 catalog) GREEN
- [ ] Race tests (`go test -race`) GREEN for orchestrator concurrency
- [ ] Goroutine leak test (`go.uber.org/goleak`) GREEN on cancellation paths

### 4.3 Backward Compatibility (Regression)

- [ ] SPEC-DEEP-001 acceptance suite (`services/storm/tests/`,
      `internal/deepreport/`, `internal/streamsynth/longform_test.go`)
      remains 100% green
- [ ] SPEC-SYN-002 acceptance suite remains 100% green (특히
      `services/researcher/src/researcher/faithfulness.py` 동작 무변경)
- [ ] SPEC-SYN-004 acceptance suite remains 100% green (SSE 기존 이벤트
      타입 호환)
- [ ] SPEC-FAN-001, SPEC-LLM-001, SPEC-CORE-001 acceptance suites remain 100% green
- [ ] `/deep?mode=storm` HTTP 응답 **schema-identical AND semantically
      equivalent** pre/post-DEEP-002 (P-M6): same event types in same
      order, same field names per event, same field types per field;
      non-deterministic fields (request_id, timestamps, durations, costs)
      MAY differ in value

### 4.4 Performance (NFR-DEEP2-001 — split budgets per P-M2)

- [ ] **NFR-DEEP2-001 budget (a)** — Go-side orchestration overhead
      p95 ≤ 1s with mocked LLM + faithfulness sidecar:
      `TestE2ELatencyP95Under1SecondMocked` (50 iterations) GREEN
- [ ] **NFR-DEEP2-001 budget (b)** — end-to-end prod p95 ≤ 60s on
      staging corpus 20-50 docs, Verifier passes on 1st or 2nd attempt:
      verified via /moai sync staging smoke test (≥ 20 sample requests).
      Pass: p95 ≤ 60s. Fail: p95 > 60s triggers SPEC re-evaluation.
      Operational gate (NOT a unit test)
- [ ] Heartbeat goroutine CPU overhead < 1% (inherited SYN-004 NFR;
      재검증)

### 4.5 Observability (REQ-DEEP2-008, NFR-DEEP2-002)

- [ ] 3개 신규 collectors 등록 및 `cmd/usearch-api /metrics` endpoint
      에서 스크랩 가능:
  - `usearch_deep_agent_duration_seconds{agent, outcome}` (cardinality 8)
  - `usearch_deep_agent_retries_total{agent}` (cardinality 1)
  - `usearch_deep_agent_verifier_gate_results_total{result}` (cardinality 3)
- [ ] 기존 `usearch_deep_outcomes_total` 에 신규 라벨 값 `empty_corpus`,
      `error_pipeline_failed` 추가 (pre-declaration 검증)
- [ ] `internal/obs/metrics/metrics_test.go::TestNoUnboundedLabels`
      변경 없이 GREEN 또는 allowlist에 신규 label NAME `result` 추가
- [ ] JSON log records carry `agent` 및 `outcome` attributes on agent
      lifecycle events
- [ ] OTel span named `deepagent.run_pipeline` (Go-side) created and
      ended within handler scope

### 4.6 Security

- [ ] No PII or API keys in log records (regression discipline; LLM input
      content는 JSON log에 echo하지 않음)
- [ ] `LITELLM_MASTER_KEY` 절대로 log/span attribute/error message에
      등장하지 않음 (SPEC-LLM-001 REQ-LLM-005)
- [ ] HTTP 503 응답 본문에 LLM partial output 누출 없음 (REQ-DEEP2-009a-Buffered, REQ-DEEP2-009b-Buffered)
- [ ] Researcher가 받는 fanout 결과는 모두 `NormalizedDoc.Validate()`
      통과 (`pkg/types/normalized_doc.go`)
- [ ] 신규 `POST /faithfulness_check` endpoint는 Pydantic validation
      먼저 통과 후 SYN-002 logic 호출

### 4.7 Documentation

- [ ] `.env.example`에 `DEEP_AGENT_*` 8개 env-var 모두 explanatory comment
      포함하여 추가
- [ ] `services/researcher/README.md`에 신규 `POST /faithfulness_check`
      endpoint 문서 추가
- [ ] @MX 태그 plan.md §6에 따라 적용
- [ ] CHANGELOG 항목 추가 (`feat(deep): SPEC-DEEP-002 multi-agent
      Researcher/Reviewer/Writer/Verifier pipeline`)
- [ ] spec.md HISTORY 항목 추가 (v0.1.0 → implemented 전이는 /moai sync 단계)

---

## 5. Out-of-Scope Verification

다음 항목이 본 SPEC의 implementation에 포함되지 **않음**을 확인 (scope discipline):

- [ ] Per-user / per-day quota enforcement 미구현 (defer SPEC-DEEP-004)
- [ ] Tree exploration with breadth/depth knobs 미구현 (defer SPEC-DEEP-003)
- [ ] LLM-as-judge multi-dimensional scoring 미구현 (Verifier는 SYN-002
      binary gate만 사용; defer SPEC-EVAL-001)
- [ ] Reviewer drill-down retrieval 미구현 (Reviewer는 critique-only)
- [ ] 신규 Python 사이드카 추가 없음 (기존 `services/researcher`에
      endpoint만 추가)
- [ ] STORM 사이드카 (`services/storm/`) 변경 없음
- [ ] `/deep?mode=storm` 동작 변경 없음 (DEEP-001 path 무손상)
- [ ] Token-level streaming from agent LLM calls 미구현 (Writer single-shot)
- [ ] `usearch deep --mode agents` CLI surface 미구현 (defer SPEC-CLI-002)
- [ ] Non-LiteLLM LLM access 없음 (`llm.Client` singleton만 사용)
- [ ] WebSocket / gRPC / NDJSON transport 없음
- [ ] `Last-Event-ID` SSE resume 미지원
- [ ] MCP tool surface 없음 (defer SPEC-MCP-001)
- [ ] GitHub issue tracking 없음 (`issue_number: 0`)

---

*End of SPEC-DEEP-002 acceptance v0.1.1.*
