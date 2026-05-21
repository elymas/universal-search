---
spec_id: SPEC-DEEP-002
version: 0.1.2
status: planned
methodology: TDD
total_tasks: 62
created: 2026-05-21
author: limbowl (via manager-spec)
companion_to: spec.md, plan.md, acceptance.md
---

# Tasks — Multi-agent /deep pipeline (Researcher → Reviewer → Writer → Verifier)

본 문서는 SPEC-DEEP-002의 TDD 작업 분해 (RED-GREEN-REFACTOR)다. plan.md §3
TDD Plan의 66개 테스트와 spec.md §3의 15개 EARS REQ + 4개 NFR을 실행 가능한
구현 작업으로 매핑한다. 각 task는 milestone (M1-M7) 단위로 분류되며 한
milestone 내에서는 RED → GREEN → REFACTOR 순서를 따른다.

본 문서가 implementation 단계의 입력이다 — `/moai run SPEC-DEEP-002` 실행 시
manager-tdd가 본 task index의 의존성 순서대로 작업을 수행한다.

---

## Task Index

| ID         | Phase    | Title                                                                     | REQ refs                                                 | Test refs (plan.md §3 #) | Blocked by |
|------------|----------|---------------------------------------------------------------------------|----------------------------------------------------------|--------------------------|------------|
| T-M1-001   | RED      | Config env-var loading + defaults tests                                   | REQ-DEEP2-004                                            | 37, 38, 40               | —          |
| T-M1-002   | GREEN    | Implement `internal/deepagent/config.go` + Config struct                  | REQ-DEEP2-004                                            | 37, 38, 40               | T-M1-001   |
| T-M1-003   | RED      | Agent enum + types + prompts smoke tests                                  | REQ-DEEP2-004, NFR-DEEP2-002                             | —                        | —          |
| T-M1-004   | GREEN    | Implement `internal/deepagent/types.go` + `prompts.go`                    | REQ-DEEP2-004, NFR-DEEP2-002                             | —                        | T-M1-003   |
| T-M1-005   | REFACTOR | Consolidate env parsing helpers; enforce no-os.Getenv-in-agents           | REQ-DEEP2-004                                            | 40                       | T-M1-002, T-M1-004 |
| T-M2-001   | RED      | Researcher fanout consumption + immutability tests                        | REQ-DEEP2-005                                            | 17, 18, 19               | T-M1-004   |
| T-M2-002   | GREEN    | Implement Researcher in `internal/deepagent/agents.go`                    | REQ-DEEP2-005                                            | 17, 18, 19               | T-M2-001   |
| T-M2-003   | RED      | Reviewer no-fanout + critique-only tests                                  | REQ-DEEP2-002                                            | 11                       | T-M1-004   |
| T-M2-004   | GREEN    | Implement Reviewer in `internal/deepagent/agents.go`                      | REQ-DEEP2-002                                            | 11                       | T-M2-003   |
| T-M2-005   | RED      | Empty fanout short-circuit + LLM skip tests (Researcher path)             | REQ-DEEP2-012                                            | 31, 35                   | T-M2-002   |
| T-M2-006   | GREEN    | Implement empty fanout SKIP in Researcher + orchestrator stub             | REQ-DEEP2-012                                            | 31, 35                   | T-M2-005   |
| T-M2-007   | RED      | Agent ordering scaffold + singleton llm.Client tests                      | REQ-DEEP2-002, REQ-DEEP2-004                             | 9 (partial), 41          | T-M2-002, T-M2-004 |
| T-M2-008   | GREEN    | Implement orchestrator skeleton (Researcher → Reviewer flow)              | REQ-DEEP2-002, REQ-DEEP2-004                             | 9 (partial), 41          | T-M2-007   |
| T-M2-009   | REFACTOR | Extract common agent invocation pattern (LLM wrapper, ctx propagation)    | REQ-DEEP2-002                                            | —                        | T-M2-008   |
| T-M3-001   | RED      | Writer no-fanout + retry hint + draft well-formedness tests               | REQ-DEEP2-002                                            | 12                       | T-M1-004   |
| T-M3-002   | GREEN    | Implement Writer in `internal/deepagent/agents.go`                        | REQ-DEEP2-002                                            | 12                       | T-M3-001   |
| T-M3-003   | RED      | `LongFormSource` interface compliance test for `WriterDraft`              | NFR-DEEP2-003                                            | —                        | T-M3-002   |
| T-M3-004   | GREEN    | Define `LongFormSource` interface in `internal/streamsynth/longform_source.go`; implement on `WriterDraft` | NFR-DEEP2-003 | —                        | T-M3-003   |
| T-M3-005   | REFACTOR | DRY Writer/Reviewer prompt assembly; verify no shared mutable state       | NFR-DEEP2-003                                            | —                        | T-M3-002, T-M3-004 |
| T-M4-001   | RED      | `CheckFaithfulness` Go wrapper unit tests (mock httptest server)          | REQ-DEEP2-006                                            | 47                       | T-M1-004   |
| T-M4-002   | GREEN    | Implement `internal/synthesis/faithfulness.go`                            | REQ-DEEP2-006                                            | 47                       | T-M4-001   |
| T-M4-003   | RED      | Python `POST /faithfulness_check` endpoint tests                          | REQ-DEEP2-006                                            | 46                       | —          |
| T-M4-004   | GREEN    | Implement `services/researcher/src/researcher/faithfulness_endpoint.py`   | REQ-DEEP2-006                                            | 46                       | T-M4-003   |
| T-M4-005   | RED      | Verifier agent PASS/FAIL gate + gate counter tests                        | REQ-DEEP2-006                                            | 42, 43, 44, 45, 48       | T-M4-002   |
| T-M4-006   | GREEN    | Implement Verifier in `internal/deepagent/agents.go`                      | REQ-DEEP2-006                                            | 42, 43, 44, 45, 48       | T-M4-005   |
| T-M4-007   | RED      | Writer retry loop + retries counter tests                                 | REQ-DEEP2-003                                            | 13, 14, 15, 16           | T-M2-008, T-M3-002, T-M4-006 |
| T-M4-008   | GREEN    | Implement orchestrator retry loop with `MaxRetries+1` bound               | REQ-DEEP2-003                                            | 13, 14, 15, 16           | T-M4-007   |
| T-M4-009   | RED      | Context cancellation between-agents test + at_agent semantics             | REQ-DEEP2-002                                            | 10                       | T-M4-008   |
| T-M4-010   | GREEN    | Implement `ctx.Err()` checks at agent boundaries + pipeline_cancelled wiring | REQ-DEEP2-002                                          | 10                       | T-M4-009   |
| T-M4-011   | RED      | Max-retry exhaustion (SSE + Buffered) + non-Verifier error abort tests    | REQ-DEEP2-009a-SSE, REQ-DEEP2-009a-Buffered, REQ-DEEP2-009b-SSE, REQ-DEEP2-009b-Buffered | 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30 | T-M4-008 |
| T-M4-012   | GREEN    | Implement error path: SSE terminal event vs HTTP 503 branching            | REQ-DEEP2-009a-SSE, REQ-DEEP2-009a-Buffered, REQ-DEEP2-009b-SSE, REQ-DEEP2-009b-Buffered | 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30 | T-M4-011 |
| T-M4-013   | REFACTOR | Consolidate error → SSE/HTTP-503 mapping helpers; verify outcome counter precision | REQ-DEEP2-009a-SSE, REQ-DEEP2-009a-Buffered, REQ-DEEP2-009b-SSE, REQ-DEEP2-009b-Buffered | 30 | T-M4-012 |
| T-M5-001   | RED      | Agent events payload schema_version + request_id JSON round-trip tests   | REQ-DEEP2-007                                            | 56                       | T-M2-008   |
| T-M5-002   | GREEN    | Implement `internal/streamsynth/agent_events.go` payload structs          | REQ-DEEP2-007                                            | 56                       | T-M5-001   |
| T-M5-003   | RED      | `LongFormSource` interface implemented by `deepreport.Report`             | NFR-DEEP2-003                                            | —                        | T-M3-004   |
| T-M5-004   | GREEN    | Make `Report` implement `LongFormSource`; add `StreamFinalReport` helper  | NFR-DEEP2-003, REQ-DEEP2-007                             | —                        | T-M5-003   |
| T-M5-005   | RED      | `EmitAgentEvent` SSE helper concurrent-write safety tests                 | REQ-DEEP2-007                                            | —                        | T-M5-002   |
| T-M5-006   | GREEN    | Implement `internal/deepagent/sse.go` event emission helper               | REQ-DEEP2-007                                            | —                        | T-M5-005   |
| T-M5-007   | RED      | SSE ordering: agent started/completed/verifier_result/retry_started sequence tests | REQ-DEEP2-007                                  | 49, 50, 51, 54, 55       | T-M5-006   |
| T-M5-008   | GREEN    | Wire SSE event emission into orchestrator phases (incl. heartbeat from handler entry) | REQ-DEEP2-007                              | 49, 50, 51, 54, 55       | T-M5-007   |
| T-M5-009   | RED      | Handler routing + mode dispatch + storm-default + no-shared-state tests  | REQ-DEEP2-001, REQ-DEEP2-011                             | 1, 2, 3, 6, 7, 8         | T-M2-008   |
| T-M5-010   | GREEN    | Implement `deep_agents_handler.go` + `synthesis.go` `?mode=` dispatch     | REQ-DEEP2-001, REQ-DEEP2-011                             | 1, 2, 3, 6, 7, 8         | T-M5-009   |
| T-M5-011   | RED      | Buffered fallback (`?stream=false`) tests: JSON shape + no SSE overhead   | REQ-DEEP2-010                                            | 4, 5                     | T-M5-010   |
| T-M5-012   | GREEN    | Implement buffered fallback path (no SSE writer/heartbeat instantiated)   | REQ-DEEP2-010                                            | 4, 5                     | T-M5-011   |
| T-M5-013   | RED      | Empty fanout SSE event sequence + JSON response shape tests              | REQ-DEEP2-012                                            | 32, 33, 34, 36           | T-M2-006, T-M5-010 |
| T-M5-014   | GREEN    | Wire empty fanout outcome in handler (SSE + buffered paths)               | REQ-DEEP2-012                                            | 32, 33, 34, 36           | T-M5-013   |
| T-M5-015   | RED      | Cancellation SSE terminal event test (`pipeline_cancelled`) + at_agent payload | REQ-DEEP2-002, REQ-DEEP2-007                        | 53                       | T-M4-010, T-M5-008 |
| T-M5-016   | GREEN    | Wire `pipeline_cancelled` terminal SSE emission in handler                | REQ-DEEP2-002, REQ-DEEP2-007                             | 53                       | T-M5-015   |
| T-M5-017   | RED      | `pipeline_failed` terminal SSE test on max-retry exhaustion (SSE path)    | REQ-DEEP2-007, REQ-DEEP2-009a-SSE, REQ-DEEP2-009b-SSE   | 52                       | T-M4-012, T-M5-008 |
| T-M5-018   | GREEN    | Wire `pipeline_failed` terminal SSE events for both 009a/009b SSE paths   | REQ-DEEP2-007, REQ-DEEP2-009a-SSE, REQ-DEEP2-009b-SSE   | 52                       | T-M5-017   |
| T-M5-019   | REFACTOR | Heartbeat goroutine timing cleanup + race-safety verification             | REQ-DEEP2-007                                            | 55                       | T-M5-008, T-M5-016, T-M5-018 |
| T-M6-001   | RED      | 3 new collector registration + label pre-declaration tests                | REQ-DEEP2-008, NFR-DEEP2-002                             | 57, 58, 59, 60           | T-M2-008   |
| T-M6-002   | GREEN    | Implement `internal/obs/metrics/deepagent.go` + `registerDeepAgent` + obs.go re-export | REQ-DEEP2-008, NFR-DEEP2-002                | 57, 58, 59, 60           | T-M6-001   |
| T-M6-003   | RED      | Cardinality guard `TestNoUnboundedLabels` regression test                 | NFR-DEEP2-002                                            | 61                       | T-M6-002   |
| T-M6-004   | GREEN    | Update metrics_test.go allowlist for `result` label name if required     | NFR-DEEP2-002                                            | 61                       | T-M6-003   |
| T-M6-005   | REFACTOR | Verify re-export through `obs.go` + grep no-user-input-in-WithLabelValues | REQ-DEEP2-008, NFR-DEEP2-002                             | 59                       | T-M6-002, T-M6-004 |
| T-M7-001   | RED      | E2E happy-path + retry-path tests via `httptest`                          | All Pipeline + Streaming REQs                            | 62, 63                   | T-M5-018, T-M6-002 |
| T-M7-002   | GREEN    | Wire E2E test fixtures; verify scenarios 1-2 from acceptance.md           | All Pipeline + Streaming REQs                            | 62, 63                   | T-M7-001   |
| T-M7-003   | RED      | NFR-DEEP2-001 budget (a): mocked orchestration p95 ≤ 1s (50 iters)       | NFR-DEEP2-001                                            | 64                       | T-M7-002   |
| T-M7-004   | GREEN    | Verify (or tune) orchestrator overhead to satisfy p95 ≤ 1s budget         | NFR-DEEP2-001                                            | 64                       | T-M7-003   |
| T-M7-005   | RED      | DEEP-001 regression + storm-mode schema-equivalence tests                 | REQ-DEEP2-011, NFR-DEEP2-003                             | 65, 66                   | T-M7-002   |
| T-M7-006   | GREEN    | Verify regression suite + documentation updates (.env.example, README)    | REQ-DEEP2-011, NFR-DEEP2-003, NFR-DEEP2-004              | 65, 66                   | T-M7-005   |

---

## Milestone M1 — Foundation: Types, Config, Prompts, LLM Routing

### T-M1-001 [RED]
- **Test**: `TestConfigLoadsAllFourModelAliasesFromEnv`, `TestConfigFallsBackToDefaultsWhenEnvAbsent`, `TestNoDirectOsGetenvInAgentsPackage`
- **REQ**: REQ-DEEP2-004
- **Scope**: Write failing tests in `internal/deepagent/config_test.go` covering 4 `DEEP_AGENT_*_MODEL` env-vars, default fallbacks, retry count parsing, faithfulness URL, and a grep-style enforcement that `os.Getenv` does not appear in any agents package file outside `config.go`.
- **Acceptance**: All three tests compile and fail with "Config undefined" / "function does not exist" — confirms tests bind to missing implementation.
- **Files touched**: `internal/deepagent/config_test.go`

### T-M1-002 [GREEN]
- **Test**: Same as T-M1-001 (now turns GREEN)
- **REQ**: REQ-DEEP2-004
- **Scope**: Implement `Config` struct loading all 8 env-vars per spec.md §6 with documented defaults; centralised loader function returning resolved `Config`.
- **Acceptance**: `go test ./internal/deepagent/config_test.go` passes; `Config` exposes `ResearcherModel/ReviewerModel/WriterModel/VerifierModel/MaxRetries/WriterRetryDelayMs/VerifierTimeoutMs/FaithfulnessURL`.
- **Files touched**: `internal/deepagent/config.go`

### T-M1-003 [RED]
- **Test**: `TestAgentTypeIsStringEnumBoundedTo4Values`, `TestPromptTemplatesHaveStableNames` (new tests added under §3.3 catalog spirit; align with NFR-DEEP2-002 bounded enum)
- **REQ**: REQ-DEEP2-004, NFR-DEEP2-002
- **Scope**: Write failing tests asserting `Agent` is a `type Agent string` enum with exactly 4 const values (`AgentResearcher`, `AgentReviewer`, `AgentWriter`, `AgentVerifier`) and that prompts package exposes stable template names for each role.
- **Acceptance**: Tests fail with "undefined: Agent" — confirms test binds to missing type.
- **Files touched**: `internal/deepagent/types_test.go`, `internal/deepagent/prompts_test.go`

### T-M1-004 [GREEN]
- **Test**: Same as T-M1-003 (now GREEN)
- **REQ**: REQ-DEEP2-004, NFR-DEEP2-002
- **Scope**: Implement `types.go` (`Agent` enum, `PipelineRequest`, `PipelineResult`, `AgentOutcome`, `AgentLogEntry`, `FaithfulnessResult`, `ResearcherOutput`, `ReviewerCritique`, `WriterDraft`, `VerifierResult`, `VerifierFeedback`) and `prompts.go` (system + user prompt templates per role).
- **Acceptance**: `go test ./internal/deepagent/...` passes for types/prompts subset; `go vet` clean.
- **Files touched**: `internal/deepagent/types.go`, `internal/deepagent/prompts.go`

### T-M1-005 [REFACTOR]
- **Test**: `TestNoDirectOsGetenvInAgentsPackage` (regression)
- **REQ**: REQ-DEEP2-004
- **Scope**: Consolidate env parsing into reusable helpers within `config.go`; run `grep -r 'os.Getenv' internal/deepagent/` returns hits only inside `config.go`.
- **Acceptance**: Grep verification passes; refactor leaves all M1 tests GREEN.
- **Files touched**: `internal/deepagent/config.go`

---

## Milestone M2 — Researcher + Reviewer Agents (Haiku Tier)

### T-M2-001 [RED]
- **Test**: `TestResearcherCallsFanoutDispatchExactlyOnce`, `TestResearcherUsesNoOtherRetrievalSource`, `TestResearcherDocsAreImmutableInDownstream`
- **REQ**: REQ-DEEP2-005
- **Scope**: Failing tests using mocked `fanout.Dispatch` spy + grep-style enforcement that no other retrieval imports (`http.Client`, `internal/adapters/*`, vector store packages) appear in `agents.go`.
- **Acceptance**: All three tests fail with "function does not exist" — confirms tests bind to missing `Researcher`.
- **Files touched**: `internal/deepagent/agents_test.go`

### T-M2-002 [GREEN]
- **Test**: Same as T-M2-001 (now GREEN)
- **REQ**: REQ-DEEP2-005
- **Scope**: Implement `Researcher(ctx, cfg, req, fanoutFn) (ResearcherOutput, error)` per plan.md §M2: 1 fanout call → claim extraction via Haiku LLM → `[]Claim + []NormalizedDocPayload`. Output slice MUST be copy-safe (no shared mutable backing).
- **Acceptance**: T-M2-001 tests GREEN.
- **Files touched**: `internal/deepagent/agents.go`

### T-M2-003 [RED]
- **Test**: `TestReviewerDoesNotCallFanout`
- **REQ**: REQ-DEEP2-002
- **Scope**: Failing test asserting Reviewer accepts only `ResearcherOutput` (no `fanoutFn` parameter); mock spy verifies fanout never invoked when Reviewer runs.
- **Acceptance**: Test fails with "Reviewer undefined".
- **Files touched**: `internal/deepagent/agents_test.go`

### T-M2-004 [GREEN]
- **Test**: Same as T-M2-003 (now GREEN)
- **REQ**: REQ-DEEP2-002
- **Scope**: Implement `Reviewer(ctx, cfg, llmClient, research ResearcherOutput) (ReviewerCritique, error)` — LLM-driven critique-only, fanout-free, output `[]{ClaimID, ConcernType, Severity}`.
- **Acceptance**: T-M2-003 tests GREEN; Reviewer body contains no `fanout` import.
- **Files touched**: `internal/deepagent/agents.go`

### T-M2-005 [RED]
- **Test**: `TestEmptyFanoutShortCircuitsPipeline`, `TestEmptyFanoutResearcherSkipsLLMInvocation`
- **REQ**: REQ-DEEP2-012
- **Scope**: Failing tests asserting that when `fanout.Dispatch` returns `Result.Docs == []`, (a) Researcher does NOT invoke the LLM mock (call count == 0), (b) returns `ResearcherOutput{IsEmpty: true}` immediately, (c) orchestrator short-circuits — Reviewer/Writer/Verifier never invoked.
- **Acceptance**: Tests fail (short-circuit logic missing).
- **Files touched**: `internal/deepagent/agents_test.go`, `internal/deepagent/orchestrator_test.go`

### T-M2-006 [GREEN]
- **Test**: Same as T-M2-005 (now GREEN)
- **REQ**: REQ-DEEP2-012
- **Scope**: Add empty-fanout short-circuit branch in Researcher (skip LLM) and orchestrator stub that returns early when `IsEmpty: true`. Histogram outcome label remains `success` (bounded enum compliance).
- **Acceptance**: T-M2-005 tests GREEN.
- **Files touched**: `internal/deepagent/agents.go`, `internal/deepagent/orchestrator.go`

### T-M2-007 [RED]
- **Test**: `TestOrchestratorRunsAgentsInOrder` (partial — Researcher → Reviewer subsection), `TestAllAgentsCallSingletonLLMClient` (partial — Researcher + Reviewer)
- **REQ**: REQ-DEEP2-002, REQ-DEEP2-004
- **Scope**: Failing tests using mock spy assertion that orchestrator invokes Researcher then Reviewer (in order) and that each agent dispatches through the singleton `llm.Client.Complete()` rather than instantiating its own.
- **Acceptance**: Tests fail (orchestrator does not yet exist).
- **Files touched**: `internal/deepagent/orchestrator_test.go`

### T-M2-008 [GREEN]
- **Test**: Same as T-M2-007 (now GREEN, Writer/Verifier stages may stub)
- **REQ**: REQ-DEEP2-002, REQ-DEEP2-004
- **Scope**: Implement orchestrator skeleton invoking Researcher → Reviewer (Writer/Verifier remain stubs returning sentinel errors so M3/M4 can plug in). All agents receive `cfg.<Role>Model` via context.
- **Acceptance**: M2-related orchestrator tests GREEN; M3/M4 tests intentionally RED.
- **Files touched**: `internal/deepagent/orchestrator.go`

### T-M2-009 [REFACTOR]
- **Test**: All M2 tests (regression)
- **REQ**: REQ-DEEP2-002
- **Scope**: Extract common pattern `invokeAgent(ctx, agent Agent, fn func() error)` wrapping LLM call + duration histogram observation + ctx propagation. Avoid premature abstraction — only extract if 3+ call sites share identical structure.
- **Acceptance**: All M1+M2 tests remain GREEN; line count of `agents.go` reduces by ≥ 20%.
- **Files touched**: `internal/deepagent/agents.go`, `internal/deepagent/orchestrator.go`

---

## Milestone M3 — Writer Agent (Sonnet Tier)

### T-M3-001 [RED]
- **Test**: `TestWriterDoesNotCallFanout`, `TestWriterAcceptsRetryHintAndPrependsToContext`, `TestWriterDraftSectionsHaveSentenceLevelCitations`
- **REQ**: REQ-DEEP2-002 (fanout-prohibition), implicit Writer contract
- **Scope**: Failing tests asserting Writer accepts `(ResearcherOutput, ReviewerCritique, *VerifierFeedback)` only (no fanout), retry hint prepended into LLM context when non-nil, and output draft contains sentence-level citation markers (1-indexed).
- **Acceptance**: Tests fail (Writer undefined).
- **Files touched**: `internal/deepagent/agents_test.go`

### T-M3-002 [GREEN]
- **Test**: Same as T-M3-001 (now GREEN)
- **REQ**: REQ-DEEP2-002
- **Scope**: Implement `Writer(ctx, cfg, llmClient, research, critique, retryHint) (WriterDraft, error)` — Sonnet LLM, single-shot completion, structured output parsing into `WriterDraft{Sections, Citations, CostUSD, Model, Provider}`.
- **Acceptance**: T-M3-001 tests GREEN; `agents.go` contains no `fanout` import in Writer code path (grep verified).
- **Files touched**: `internal/deepagent/agents.go`

### T-M3-003 [RED]
- **Test**: `TestWriterDraftImplementsLongFormSource` (new helper test asserting interface conformance via `var _ LongFormSource = (*WriterDraft)(nil)`)
- **REQ**: NFR-DEEP2-003 (decoupling streamsynth from concrete types)
- **Scope**: Failing test verifying `WriterDraft` satisfies the `LongFormSource` interface defined in `internal/streamsynth/longform_source.go` (interface itself does not yet exist).
- **Acceptance**: Compile error — interface undefined.
- **Files touched**: `internal/deepagent/types_test.go`

### T-M3-004 [GREEN]
- **Test**: Same as T-M3-003 (now GREEN)
- **REQ**: NFR-DEEP2-003
- **Scope**: Define `LongFormSource` interface (SectionCount, Section, Citations, Metadata) in new file `internal/streamsynth/longform_source.go`; implement interface methods on `WriterDraft` in `types.go`.
- **Acceptance**: Interface conformance check passes; `internal/deepagent` does not import `internal/deepreport`.
- **Files touched**: `internal/streamsynth/longform_source.go`, `internal/deepagent/types.go`

### T-M3-005 [REFACTOR]
- **Test**: All M3 tests (regression)
- **REQ**: NFR-DEEP2-003
- **Scope**: DRY Writer/Reviewer prompt template assembly if duplication exists; verify no shared mutable state between Writer state and other agents (race-detector compile pass).
- **Acceptance**: `go test -race ./internal/deepagent/...` GREEN; no global mutable variables in `agents.go`.
- **Files touched**: `internal/deepagent/agents.go`, `internal/deepagent/prompts.go`

---

## Milestone M4 — Verifier Agent + SYN-002 Wrapper + Retry Loop

### T-M4-001 [RED]
- **Test**: `TestCheckFaithfulnessWrapperHandlesSidecar5xx` + wrapper unit tests for success/timeout/transport-failure paths
- **REQ**: REQ-DEEP2-006
- **Scope**: Failing tests using `httptest.Server` simulating the researcher sidecar `POST /faithfulness_check` endpoint — verify wrapper maps 2xx → `FaithfulnessResult`, 5xx → `error` with `fail_error` gate-counter increment, ctx propagation honored.
- **Acceptance**: Tests fail (wrapper undefined).
- **Files touched**: `internal/synthesis/faithfulness_test.go`

### T-M4-002 [GREEN]
- **Test**: Same as T-M4-001 (now GREEN)
- **REQ**: REQ-DEEP2-006
- **Scope**: Implement `internal/synthesis/faithfulness.go:CheckFaithfulness(ctx, text, citations, docs) (FaithfulnessResult, error)`; HTTP client pattern mimicking `internal/synthesis/client.go`; uses `DEEP_AGENT_VERIFIER_TIMEOUT_MS` from Config; `DEEP_AGENT_FAITHFULNESS_URL` as endpoint.
- **Acceptance**: T-M4-001 tests GREEN.
- **Files touched**: `internal/synthesis/faithfulness.go`

### T-M4-003 [RED]
- **Test**: `TestFaithfulnessEndpointReusesExistingSYN002Logic` (Python pytest)
- **REQ**: REQ-DEEP2-006
- **Scope**: Failing pytest in `services/researcher/tests/test_faithfulness_endpoint.py` asserting (a) the new endpoint exists at `POST /faithfulness_check`, (b) request schema (Pydantic), (c) endpoint calls existing `enforce_faithfulness()` from `services/researcher/src/researcher/faithfulness.py` without modification (mock verified).
- **Acceptance**: Test fails — endpoint not registered.
- **Files touched**: `services/researcher/tests/test_faithfulness_endpoint.py`

### T-M4-004 [GREEN]
- **Test**: Same as T-M4-003 (now GREEN)
- **REQ**: REQ-DEEP2-006
- **Scope**: Implement `services/researcher/src/researcher/faithfulness_endpoint.py` — FastAPI route registration, Pydantic request/response models, delegate to existing `enforce_faithfulness()`.
- **Acceptance**: T-M4-003 pytest GREEN; existing SYN-002 logic unchanged (single source of truth preserved).
- **Files touched**: `services/researcher/src/researcher/faithfulness_endpoint.py`

### T-M4-005 [RED]
- **Test**: `TestVerifierCallsCheckFaithfulnessExactlyOnce`, `TestVerifierPassWhenUncitedCountZero`, `TestVerifierFailWhenUncitedCountPositive`, `TestVerifierDoesNotPerformAdditionalScoring`, `TestVerifierGateResultsCounterIncrementsOncePerInvocation`
- **REQ**: REQ-DEEP2-006
- **Scope**: Failing tests for Verifier agent: PASS when `UncitedSentencesCount == 0`, FAIL with feedback otherwise; counter `usearch_deep_agent_verifier_gate_results_total{result}` increments exactly once per invocation; no LLM call beyond the SYN-002 sidecar.
- **Acceptance**: Tests fail (Verifier undefined).
- **Files touched**: `internal/deepagent/agents_test.go`

### T-M4-006 [GREEN]
- **Test**: Same as T-M4-005 (now GREEN)
- **REQ**: REQ-DEEP2-006
- **Scope**: Implement `Verifier(ctx, cfg, draft, docs) (VerifierResult, error)` — binary gate on `UncitedSentencesCount == 0`; returns `VerifierResult{Pass, Feedback}`.
- **Acceptance**: T-M4-005 tests GREEN.
- **Files touched**: `internal/deepagent/agents.go`

### T-M4-007 [RED]
- **Test**: `TestOrchestratorRetriesWriterOnVerifierReject`, `TestOrchestratorEmitsRetryStartedBeforeRetryCall`, `TestRetriesCounterIncrementsExactlyOncePerRetry`, `TestNonVerifierErrorsDoNotTriggerRetry`
- **REQ**: REQ-DEEP2-003
- **Scope**: Failing tests asserting Verifier rejection triggers Writer retry up to `MaxRetries + 1` total Writer attempts; `retry_started` SSE event precedes retry call; retries counter `+= 1 per retry`; Researcher/Reviewer/Verifier errors do NOT trigger retry.
- **Acceptance**: Tests fail (retry loop not wired).
- **Files touched**: `internal/deepagent/orchestrator_test.go`

### T-M4-008 [GREEN]
- **Test**: Same as T-M4-007 (now GREEN)
- **REQ**: REQ-DEEP2-003
- **Scope**: Implement retry loop in `orchestrator.go`: initial Writer call + up to `MaxRetries` retries (default 2 = 3 total); only Verifier rejection triggers retry; emit `retry_started` before each retry; increment retries counter precisely once per retry.
- **Acceptance**: T-M4-007 tests GREEN.
- **Files touched**: `internal/deepagent/orchestrator.go`

### T-M4-009 [RED]
- **Test**: `TestOrchestratorHaltsOnContextCancelBetweenAgents`
- **REQ**: REQ-DEEP2-002
- **Scope**: Failing test simulating `ctx.Cancel()` between Reviewer and Writer — orchestrator must NOT invoke Writer/Verifier; `pipeline_cancelled` SSE event `at_agent` field populated per RDC-3 semantics (next-would-be agent OR in-progress agent on LLM mid-flight cancel).
- **Acceptance**: Test fails (ctx checks missing).
- **Files touched**: `internal/deepagent/orchestrator_test.go`

### T-M4-010 [GREEN]
- **Test**: Same as T-M4-009 (now GREEN)
- **REQ**: REQ-DEEP2-002
- **Scope**: Implement `ctx.Err()` check immediately before each agent invocation; on cancel, return sentinel `ErrPipelineCancelled{AtAgent}`. Wiring to SSE event emission deferred to M5.
- **Acceptance**: T-M4-009 GREEN; orphan LLM call detection (LLM client mock call count) verified.
- **Files touched**: `internal/deepagent/orchestrator.go`

### T-M4-011 [RED]
- **Test**: `TestSSEMaxRetryEmitsTerminalEvent`, `TestSSEMaxRetryHttpStatusStays200`, `TestBufferedMaxRetryReturns503`, `TestBufferedMaxRetryJsonBodyShape`, `TestSSEResearcherErrorEmitsTerminalEvent`, `TestSSEReviewerErrorEmitsTerminalEvent`, `TestSSEVerifierErrorEmitsTerminalEvent`, `TestBufferedResearcherErrorReturns503`, `TestBufferedReviewerErrorReturns503`, `TestBufferedVerifierErrorReturns503`, `TestErrorOutcomeCounterIncrementsExactlyOnce`
- **REQ**: REQ-DEEP2-009a-SSE, REQ-DEEP2-009a-Buffered, REQ-DEEP2-009b-SSE, REQ-DEEP2-009b-Buffered
- **Scope**: Failing tests for max-retry exhaustion (split SSE vs Buffered) and non-Verifier agent error abort (split SSE vs Buffered). HTTP 200 stays for SSE-active (headers flushed) — terminal event mechanism. HTTP 503 for buffered path. `usearch_deep_outcomes_total{outcome="error_pipeline_failed"}` increments exactly once per aborted pipeline.
- **Acceptance**: All 11 tests fail (error branching not wired).
- **Files touched**: `internal/deepagent/orchestrator_test.go`, `cmd/usearch-api/handlers/deep_agents_handler_test.go`

### T-M4-012 [GREEN]
- **Test**: Same as T-M4-011 (now GREEN)
- **REQ**: REQ-DEEP2-009a-SSE, REQ-DEEP2-009a-Buffered, REQ-DEEP2-009b-SSE, REQ-DEEP2-009b-Buffered
- **Scope**: Implement error-path branching in orchestrator + handler: detect stream state (SSE headers flushed vs not), emit terminal `pipeline_failed` SSE OR HTTP 503 JSON accordingly. Map error sources (Writer max-retry exhaustion / Researcher/Reviewer/Verifier infra) to `failed_agent` + `reason` payload fields.
- **Acceptance**: T-M4-011 tests GREEN.
- **Files touched**: `internal/deepagent/orchestrator.go`, `cmd/usearch-api/handlers/deep_agents_handler.go` (stub if M5 not yet started)

### T-M4-013 [REFACTOR]
- **Test**: All M4 tests (regression)
- **REQ**: REQ-DEEP2-009a-SSE, REQ-DEEP2-009a-Buffered, REQ-DEEP2-009b-SSE, REQ-DEEP2-009b-Buffered
- **Scope**: Consolidate error → SSE/HTTP-503 mapping into a single helper (`mapPipelineFailure(streamState, src) (sseEvent, httpStatus, jsonBody)`). Verify `usearch_deep_outcomes_total{outcome="error_pipeline_failed"}` increments exactly once across all 4 error-REQ paths.
- **Acceptance**: All M4 tests remain GREEN; helper unit-tested for all 4 REQ branches × 2 stream states.
- **Files touched**: `internal/deepagent/orchestrator.go`

---

## Milestone M5 — SSE Event Extension + Handler Integration

### T-M5-001 [RED]
- **Test**: `TestAllNewEventPayloadsCarrySchemaVersionAndRequestId` + JSON round-trip tests for each payload (AgentStarted, AgentCompleted, RetryStarted, VerifierResult, PipelineFailed, PipelineCancelled)
- **REQ**: REQ-DEEP2-007
- **Scope**: Failing tests asserting each new payload struct marshals/unmarshals JSON with `schema_version: 1` and `request_id` fields present.
- **Acceptance**: Tests fail (payload structs undefined).
- **Files touched**: `internal/streamsynth/agent_events_test.go`

### T-M5-002 [GREEN]
- **Test**: Same as T-M5-001 (now GREEN)
- **REQ**: REQ-DEEP2-007
- **Scope**: Implement payload structs in `internal/streamsynth/agent_events.go`: `AgentStartedPayload`, `AgentCompletedPayload` (with `outcome` JSON field accepting `empty_corpus`), `RetryStartedPayload`, `VerifierResultPayload`, `PipelineFailedPayload`, `PipelineCancelledPayload`. All include `schema_version: 1` and `request_id`.
- **Acceptance**: T-M5-001 tests GREEN.
- **Files touched**: `internal/streamsynth/agent_events.go`

### T-M5-003 [RED]
- **Test**: `TestReportImplementsLongFormSource` (Go interface assertion `var _ LongFormSource = (*deepreport.Report)(nil)`)
- **REQ**: NFR-DEEP2-003
- **Scope**: Failing test ensuring `deepreport.Report` implements the `LongFormSource` interface (so streamsynth no longer needs to reference concrete type).
- **Acceptance**: Compile error — Report missing interface methods.
- **Files touched**: `internal/deepreport/types_test.go`

### T-M5-004 [GREEN]
- **Test**: Same as T-M5-003 (now GREEN)
- **REQ**: NFR-DEEP2-003, REQ-DEEP2-007
- **Scope**: Add `SectionCount`, `Section`, `Citations`, `Metadata` methods to `Report` in `internal/deepreport/types.go`. Add new public helper `StreamFinalReport(ctx, w, source LongFormSource, agentLog []AgentLogEntry)` in `internal/streamsynth/longform.go` (Verifier-PASS post-processing entry point).
- **Acceptance**: T-M5-003 GREEN; DEEP-001 acceptance regression suite remains 100% green.
- **Files touched**: `internal/deepreport/types.go`, `internal/streamsynth/longform.go`

### T-M5-005 [RED]
- **Test**: `TestEmitAgentEventConcurrentWriteSafe` (race test with concurrent main pipeline + heartbeat goroutine writes)
- **REQ**: REQ-DEEP2-007
- **Scope**: Failing race-detector test verifying `EmitAgentEvent(w *sse.Writer, event AgentEvent)` is safe for concurrent invocation alongside heartbeat goroutine.
- **Acceptance**: Test fails (function undefined) or race detected.
- **Files touched**: `internal/deepagent/sse_test.go`

### T-M5-006 [GREEN]
- **Test**: Same as T-M5-005 (now GREEN)
- **REQ**: REQ-DEEP2-007
- **Scope**: Implement `internal/deepagent/sse.go:EmitAgentEvent` reusing `sse.Writer` from SYN-004 (existing mutex-protected primitive). Verify defer-close on ctx done.
- **Acceptance**: `go test -race ./internal/deepagent/sse_test.go` GREEN.
- **Files touched**: `internal/deepagent/sse.go`

### T-M5-007 [RED]
- **Test**: `TestSSEEmitsAgentStartedAndCompletedForEachAgent`, `TestSSEEmitsRetryStartedBeforeWriterRetry`, `TestSSEEmitsVerifierResultPerVerifierInvocation`, `TestSSESectionEventsOnlyAfterVerifierPass`, `TestSSEHeartbeatContinuesDuringAgentPhases`
- **REQ**: REQ-DEEP2-007
- **Scope**: Failing tests asserting (a) 8 minimum agent_started+agent_completed events per happy path, (b) retry_started precedes Writer retry, (c) verifier_result emitted per Verifier invocation, (d) section_start events ONLY after Verifier PASS, (e) heartbeat continues throughout agent phases.
- **Acceptance**: Tests fail (orchestrator does not yet emit SSE events).
- **Files touched**: `internal/deepagent/orchestrator_test.go`

### T-M5-008 [GREEN]
- **Test**: Same as T-M5-007 (now GREEN)
- **REQ**: REQ-DEEP2-007
- **Scope**: Wire `EmitAgentEvent` calls into orchestrator phases per RDC-6 (Writer/Verifier loop pre-buffered, section events only after PASS). Heartbeat goroutine started at handler entry (in M5-010 when handler exists; here orchestrator hooks the SSE writer through `PipelineRequest`).
- **Acceptance**: T-M5-007 tests GREEN.
- **Files touched**: `internal/deepagent/orchestrator.go`

### T-M5-009 [RED]
- **Test**: `TestDeepHandlerRoutesModeAgentsToOrchestrator`, `TestDeepHandlerDefaultsToModeStorm`, `TestDeepHandlerRequestSchemaMatchesContract`, `TestDeepHandlerStormModeUnchanged`, `TestDeepHandlerModeAbsentDefaultsToStorm`, `TestDeepHandlerNoSharedMutableStateBetweenModes`
- **REQ**: REQ-DEEP2-001, REQ-DEEP2-011
- **Scope**: Failing tests via `httptest`: `?mode=agents` routes to new handler, `?mode=` absent defaults to storm path, request schema parses `{request_id, query, lang}` ignoring `docs[]`, no shared mutable global state between modes (package-level var grep + concurrency race test).
- **Acceptance**: Tests fail (handler undefined or routing wrong).
- **Files touched**: `cmd/usearch-api/handlers/deep_agents_handler_test.go`

### T-M5-010 [GREEN]
- **Test**: Same as T-M5-009 (now GREEN)
- **REQ**: REQ-DEEP2-001, REQ-DEEP2-011
- **Scope**: Implement `cmd/usearch-api/handlers/deep_agents_handler.go` (Accept header content negotiation, SSE writer + heartbeat goroutine start at entry, orchestrator dispatch). Add `?mode=` parsing to `cmd/usearch-api/handlers/synthesis.go` (storm → DEEP-001 unchanged, agents → new handler).
- **Acceptance**: T-M5-009 tests GREEN; `internal/deepagent` does NOT import `internal/deepreport`.
- **Files touched**: `cmd/usearch-api/handlers/deep_agents_handler.go`, `cmd/usearch-api/handlers/synthesis.go`

### T-M5-011 [RED]
- **Test**: `TestDeepHandlerBufferedFallbackReturnsFinalReport`, `TestDeepHandlerBufferedFallbackHasNoSSEOverhead`
- **REQ**: REQ-DEEP2-010
- **Scope**: Failing tests asserting `?stream=false` (or non-SSE Accept) returns HTTP 200 JSON with same data fields as final agent_completed{verifier} + final Report. SSE writer constructor mock call count == 0.
- **Acceptance**: Tests fail (buffered path not yet implemented).
- **Files touched**: `cmd/usearch-api/handlers/deep_agents_handler_test.go`

### T-M5-012 [GREEN]
- **Test**: Same as T-M5-011 (now GREEN)
- **REQ**: REQ-DEEP2-010
- **Scope**: Implement buffered fallback branch in handler: no SSE writer, no heartbeat, collect agent_log + final report into single JSON payload.
- **Acceptance**: T-M5-011 tests GREEN.
- **Files touched**: `cmd/usearch-api/handlers/deep_agents_handler.go`

### T-M5-013 [RED]
- **Test**: `TestEmptyFanoutResponseShape`, `TestEmptyFanoutOutcomeCounterIncrements`, `TestEmptyFanoutSSEEventSequence`, `TestEmptyFanoutHistogramOutcomeIsSuccess`
- **REQ**: REQ-DEEP2-012
- **Scope**: Failing tests asserting empty-fanout SSE sequence (`agent_started{researcher}` → `agent_completed{researcher, outcome:"empty_corpus"}` → `done{total_sections:0}`) and corresponding JSON body shape for buffered path; outcome counter `empty_corpus` += 1; histogram label `success` (NOT `empty_corpus`) per P-M3 label-vs-field clarification.
- **Acceptance**: Tests fail (handler not yet emitting empty-fanout response).
- **Files touched**: `cmd/usearch-api/handlers/deep_agents_handler_test.go`

### T-M5-014 [GREEN]
- **Test**: Same as T-M5-013 (now GREEN)
- **REQ**: REQ-DEEP2-012
- **Scope**: Wire empty fanout outcome into handler: emit appropriate SSE sequence in stream path or JSON body in buffered path; increment `usearch_deep_outcomes_total{outcome="empty_corpus"}`.
- **Acceptance**: T-M5-013 tests GREEN.
- **Files touched**: `cmd/usearch-api/handlers/deep_agents_handler.go`, `internal/deepagent/orchestrator.go`

### T-M5-015 [RED]
- **Test**: `TestSSEEmitsPipelineCancelledOnContextCancel`
- **REQ**: REQ-DEEP2-002, REQ-DEEP2-007
- **Scope**: Failing test simulating `r.Context().Done()` fire after Reviewer completes — handler must emit terminal `pipeline_cancelled{at_agent}` SSE event with payload per P-M1 (next-would-be agent semantics).
- **Acceptance**: Test fails.
- **Files touched**: `cmd/usearch-api/handlers/deep_agents_handler_test.go`

### T-M5-016 [GREEN]
- **Test**: Same as T-M5-015 (now GREEN)
- **REQ**: REQ-DEEP2-002, REQ-DEEP2-007
- **Scope**: Wire ctx cancellation handling into handler: catch `ErrPipelineCancelled{AtAgent}` from orchestrator, emit terminal SSE `pipeline_cancelled` with `at_agent` field; stream closed cleanly. Goroutine leak free (goleak verified).
- **Acceptance**: T-M5-015 GREEN.
- **Files touched**: `cmd/usearch-api/handlers/deep_agents_handler.go`

### T-M5-017 [RED]
- **Test**: `TestSSEEmitsPipelineFailedOnExhaustion`
- **REQ**: REQ-DEEP2-007, REQ-DEEP2-009a-SSE, REQ-DEEP2-009b-SSE
- **Scope**: Failing test verifying both max-retry exhaustion AND non-Verifier agent errors result in terminal `pipeline_failed{failed_agent, reason}` SSE event when stream is active.
- **Acceptance**: Test fails (handler not yet wiring failure SSE).
- **Files touched**: `cmd/usearch-api/handlers/deep_agents_handler_test.go`

### T-M5-018 [GREEN]
- **Test**: Same as T-M5-017 (now GREEN)
- **REQ**: REQ-DEEP2-007, REQ-DEEP2-009a-SSE, REQ-DEEP2-009b-SSE
- **Scope**: Wire `pipeline_failed` terminal SSE emission using the consolidated mapping helper from T-M4-013; stream closed after event.
- **Acceptance**: T-M5-017 GREEN.
- **Files touched**: `cmd/usearch-api/handlers/deep_agents_handler.go`

### T-M5-019 [REFACTOR]
- **Test**: All M5 tests + race tests
- **REQ**: REQ-DEEP2-007
- **Scope**: Verify heartbeat goroutine timing (start at handler entry per RDC-10) is consistent; run `go test -race` on `httptest` integration tests; close stale goroutines on cancellation paths via `goleak`.
- **Acceptance**: `go test -race ./cmd/usearch-api/handlers/...` GREEN; `goleak` shows no leaked goroutines.
- **Files touched**: `cmd/usearch-api/handlers/deep_agents_handler.go`

---

## Milestone M6 — Prometheus Metrics + Cardinality Safety

### T-M6-001 [RED]
- **Test**: `TestThreeNewCollectorsRegisteredAtStartup`, `TestAllAgentLabelValuesPreDeclaredAtRegistration`, `TestNoLabelValueDerivedFromUserInput`, `TestDeepOutcomesExtendedWithEmptyCorpusAndPipelineFailed`
- **REQ**: REQ-DEEP2-008, NFR-DEEP2-002
- **Scope**: Failing tests asserting 3 new collectors registered via `registerDeepAgent(pr)` helper, all label values (`agent`, `outcome`, `result`) pre-declared via `.WithLabelValues(v).Add(0)`, no `WithLabelValues` call accepts user-derived strings (grep), existing `usearch_deep_outcomes_total` extended with `empty_corpus` + `error_pipeline_failed`.
- **Acceptance**: Tests fail (collectors do not yet exist).
- **Files touched**: `internal/obs/metrics/deepagent_test.go`

### T-M6-002 [GREEN]
- **Test**: Same as T-M6-001 (now GREEN)
- **REQ**: REQ-DEEP2-008, NFR-DEEP2-002
- **Scope**: Implement `internal/obs/metrics/deepagent.go` with 3 collectors (`DeepAgentDuration` HistogramVec{agent, outcome}, `DeepAgentRetries` CounterVec{agent="writer"}, `DeepAgentVerifierGateResults` CounterVec{result}). Pre-declare all label values per SYN-004 pattern. Add `registerDeepAgent(pr)` helper to `internal/obs/metrics/metrics.go`. Extend `usearch_deep_outcomes_total` pre-declaration with new values. Re-export via `internal/obs/obs.go`.
- **Acceptance**: T-M6-001 tests GREEN; `cmd/usearch-api/main.go` calls `deepagent.RegisterMetrics(pr)`.
- **Files touched**: `internal/obs/metrics/deepagent.go`, `internal/obs/metrics/metrics.go`, `internal/obs/obs.go`, `cmd/usearch-api/main.go`

### T-M6-003 [RED]
- **Test**: `TestCardinalityGuardRemainsGreen` (regression umbrella for `internal/obs/metrics/metrics_test.go::TestNoUnboundedLabels`)
- **REQ**: NFR-DEEP2-002
- **Scope**: Failing test ensuring cardinality guard remains green; if new label name `result` is not yet in allowlist, allowlist update required.
- **Acceptance**: Test fails (allowlist amendment needed OR new bounded set rejected).
- **Files touched**: `internal/obs/metrics/metrics_test.go`

### T-M6-004 [GREEN]
- **Test**: Same as T-M6-003 (now GREEN)
- **REQ**: NFR-DEEP2-002
- **Scope**: If required by existing guard schema, add `result` to allowlist; verify all label NAMES used by DEEP-002 are accepted by the bounded-set validator.
- **Acceptance**: T-M6-003 GREEN; cardinality guard test passes without label-set expansion beyond declared values.
- **Files touched**: `internal/obs/metrics/metrics_test.go` (allowlist amendment if applicable)

### T-M6-005 [REFACTOR]
- **Test**: All M6 tests (regression)
- **REQ**: REQ-DEEP2-008, NFR-DEEP2-002
- **Scope**: Verify re-export through `obs.go` cleanly exposes `obs.DeepAgentDuration`, `obs.DeepAgentRetries`, `obs.DeepAgentVerifierGateResults`; grep `WithLabelValues` ensures no user-input variables in label position.
- **Acceptance**: Manual grep verification clean; M6 tests remain GREEN.
- **Files touched**: `internal/obs/obs.go`

---

## Milestone M7 — End-to-End Integration + Documentation

### T-M7-001 [RED]
- **Test**: `TestE2EHappyPathReturns200WithCompleteStream`, `TestE2ERetryPathRecordsCorrectMetricsAndEvents`
- **REQ**: REQ-DEEP2-001 through REQ-DEEP2-008
- **Scope**: Failing E2E tests using `httptest.Server` for both happy path (Scenario 1) and retry path (Scenario 2); assert full SSE sequence, all 4 agents invoked in order, fanout called exactly once, metrics counters increment correctly.
- **Acceptance**: Tests fail until M1-M6 fully GREEN.
- **Files touched**: `cmd/usearch-api/handlers/deep_agents_handler_test.go`

### T-M7-002 [GREEN]
- **Test**: Same as T-M7-001 (now GREEN)
- **REQ**: REQ-DEEP2-001 through REQ-DEEP2-008
- **Scope**: Provide E2E fixtures (`fanout.Dispatch` mock returning 25 docs, `llm.Client.Complete` mocks for Haiku + Sonnet tiers, faithfulness sidecar mock); ensure scenarios 1-2 from acceptance.md pass.
- **Acceptance**: T-M7-001 GREEN; acceptance.md scenarios 1, 2 verified.
- **Files touched**: `cmd/usearch-api/handlers/deep_agents_handler_test.go` (fixture helpers)

### T-M7-003 [RED]
- **Test**: `TestE2ELatencyP95Under1SecondMocked` (50 iterations)
- **REQ**: NFR-DEEP2-001 budget (a)
- **Scope**: Failing statistical test asserting Go-side orchestration overhead p95 ≤ 1 s with LLM + faithfulness sidecar mocked (instant response). Verifies orchestrator/SSE/metrics overhead isolated from upstream latency.
- **Acceptance**: Test fails until performance budget verified.
- **Files touched**: `internal/deepagent/orchestrator_test.go`

### T-M7-004 [GREEN]
- **Test**: Same as T-M7-003 (now GREEN)
- **REQ**: NFR-DEEP2-001 budget (a)
- **Scope**: Verify orchestrator overhead satisfies p95 ≤ 1 s; tune if necessary (likely no tuning needed since mocked LLM is instant). Budget (b) prod p95 ≤ 60 s is operational gate verified during `/moai sync` smoke test — NOT a unit test.
- **Acceptance**: T-M7-003 GREEN; statistical p95 ≤ 1 s across 50 iterations.
- **Files touched**: `internal/deepagent/orchestrator.go` (only if tuning needed)

### T-M7-005 [RED]
- **Test**: `TestDeep001AcceptanceSuiteRemainsGreen` (regression umbrella), `TestStormModeResponseSchemaIdenticalPrePostDeep002`
- **REQ**: REQ-DEEP2-011, NFR-DEEP2-003
- **Scope**: Failing regression umbrella test invoking DEEP-001 acceptance suite (`services/storm/tests/`, `internal/deepreport/`, `internal/streamsynth/longform_test.go`); failing schema-equivalence test verifying `/deep?mode=storm` produces schema-identical AND semantically equivalent SSE/JSON pre/post-DEEP-002 (same event types in same order, same field names per event, same field types per field; non-deterministic field values may differ).
- **Acceptance**: Tests fail until DEEP-001 regression is verified end-to-end and schema equivalence is asserted.
- **Files touched**: `cmd/usearch-api/handlers/deep_agents_handler_test.go`, `internal/deepreport/types_test.go`

### T-M7-006 [GREEN]
- **Test**: Same as T-M7-005 (now GREEN)
- **REQ**: REQ-DEEP2-011, NFR-DEEP2-003, NFR-DEEP2-004
- **Scope**: Verify DEEP-001 acceptance suite passes 100%; update `.env.example` with 8 `DEEP_AGENT_*` env-var documentation; update `services/researcher/README.md` with new `POST /faithfulness_check` endpoint section. CHANGELOG entry: `feat(deep): SPEC-DEEP-002 multi-agent Researcher/Reviewer/Writer/Verifier pipeline`.
- **Acceptance**: All 66 tests from plan.md §3 catalog GREEN; coverage ≥ 85% on all new Go and Python files; TRUST 5 gates pass; LSP gates pass.
- **Files touched**: `.env.example`, `services/researcher/README.md`, `CHANGELOG.md`

---

## MX Tag Annotation Plan

Per plan.md §6, the following @MX tag annotations MUST be applied during GREEN-to-REFACTOR transitions. manager-tdd is responsible for adding/updating tags. All descriptions in English (per `code_comments: en` setting).

### @MX:ANCHOR (Invariant Contracts, fan_in ≥ 3)

| File | Symbol | Tag | Spec Refs |
|------|--------|-----|-----------|
| `internal/deepagent/orchestrator.go` | `RunPipeline` | `@MX:ANCHOR: Single entry for 4-agent pipeline orchestration` | SPEC-DEEP-002 REQ-DEEP2-002, REQ-DEEP2-003 |
| `internal/synthesis/faithfulness.go` | `CheckFaithfulness` | `@MX:ANCHOR: Single chokepoint for SYN-002 faithfulness gate invocation from Go side` | SPEC-SYN-002, SPEC-DEEP-002 REQ-DEEP2-006 |
| `cmd/usearch-api/handlers/deep_agents_handler.go` | `ServeHTTP` | `@MX:ANCHOR: HTTP boundary for /deep?mode=agents` | SPEC-DEEP-002 REQ-DEEP2-001, REQ-DEEP2-011 |

Applied in tasks: T-M4-008 (orchestrator anchor), T-M4-002 (faithfulness anchor), T-M5-010 (handler anchor).

### @MX:WARN (Danger Zones, requires @MX:REASON)

| File | Symbol | Tag | Spec Refs |
|------|--------|-----|-----------|
| `internal/deepagent/orchestrator.go` | retry loop block | `@MX:WARN: Bounded retry creates cost amplification surface` + `@MX:REASON: Each retry invokes Sonnet-tier Writer` | SPEC-DEEP-002 REQ-DEEP2-003, NFR-DEEP2-004 |
| `internal/deepagent/sse.go` | `EmitAgentEvent` | `@MX:WARN: Concurrent writes from main pipeline + heartbeat goroutine` + `@MX:REASON: sse.Writer mutex must be held` | SPEC-SYN-004, SPEC-DEEP-002 REQ-DEEP2-007 |
| `internal/deepagent/orchestrator.go` | ctx cancellation block | `@MX:WARN: Context cancellation must release in-flight LLM HTTP bodies` + `@MX:REASON: orphaned LLM calls burn budget` | SPEC-DEEP-002 REQ-DEEP2-002, RDC-3 |

Applied in tasks: T-M4-013 (retry loop), T-M5-019 (sse.go race-safety), T-M4-010 (ctx cancel).

### @MX:NOTE (Context Delivery)

| File | Symbol | Tag |
|------|--------|-----|
| `internal/deepagent/prompts.go` | each prompt template constant | `@MX:NOTE: Prompt template — semantic intent: <role>'s task is to <what>` (one per role) |
| `internal/deepagent/types.go` | `Agent` constants | `@MX:NOTE: Enum-like type; bounded label values for Prometheus cardinality safety` |
| `internal/deepagent/agents.go` | Verifier→Writer feedback adapter | `@MX:NOTE: Verifier feedback to Writer retry hint conversion` |
| `internal/deepagent/config.go` | env-var loading | `@MX:NOTE: All DEEP_AGENT_* env-vars loaded here; agents must not call os.Getenv` |

Applied in tasks: T-M1-004 (types/prompts notes), T-M1-002 (config note), T-M4-008 (Verifier feedback adapter note).

### @MX:TODO (Iteration Markers)

| Milestone | TODO Marker | Removal Point |
|-----------|-------------|---------------|
| M1 RED | `@MX:TODO: SPEC-DEEP-002 REQ-DEEP2-004 — implement Config struct` | T-M1-002 GREEN |
| M2 RED | `@MX:TODO: SPEC-DEEP-002 REQ-DEEP2-005 — implement Researcher` | T-M2-002 GREEN |
| M3 RED | `@MX:TODO: SPEC-DEEP-002 — implement Writer` | T-M3-002 GREEN |
| M4 RED | `@MX:TODO: SPEC-DEEP-002 REQ-DEEP2-006 — implement Verifier` | T-M4-006 GREEN |
| M5 RED | `@MX:TODO: SPEC-DEEP-002 REQ-DEEP2-001 — implement handler` | T-M5-010 GREEN |
| M6 RED | `@MX:TODO: SPEC-DEEP-002 REQ-DEEP2-008 — register collectors` | T-M6-002 GREEN |

All @MX:TODO markers MUST be removed in the corresponding GREEN task. manager-tdd produces an @MX Tag Report per plan.md §6.5 at the end of the Run phase.

---

## Verification Checklist

### EARS Requirement Coverage (15 REQs)

| REQ ID | Module | Test Coverage | Tasks |
|--------|--------|---------------|-------|
| REQ-DEEP2-001 | Endpoint | Tests 1, 2, 3 | T-M5-009, T-M5-010 |
| REQ-DEEP2-002 | Pipeline | Tests 9, 10, 11, 12 | T-M2-003, T-M2-004, T-M2-007, T-M2-008, T-M3-001, T-M3-002, T-M4-009, T-M4-010, T-M5-015, T-M5-016 |
| REQ-DEEP2-003 | Pipeline | Tests 13, 14, 15, 16 | T-M4-007, T-M4-008 |
| REQ-DEEP2-004 | LLM Routing | Tests 37, 38, 39, 40, 41 | T-M1-001, T-M1-002, T-M1-003, T-M1-004, T-M1-005, T-M2-007, T-M2-008 |
| REQ-DEEP2-005 | Pipeline | Tests 17, 18, 19 | T-M2-001, T-M2-002 |
| REQ-DEEP2-006 | Verifier Gate | Tests 42, 43, 44, 45, 46, 47, 48 | T-M4-001, T-M4-002, T-M4-003, T-M4-004, T-M4-005, T-M4-006 |
| REQ-DEEP2-007 | Streaming | Tests 49, 50, 51, 52, 53, 54, 55, 56 | T-M5-001, T-M5-002, T-M5-005, T-M5-006, T-M5-007, T-M5-008, T-M5-015 through T-M5-019 |
| REQ-DEEP2-008 | Observability | Tests 57, 58, 59, 60, 61 | T-M6-001, T-M6-002, T-M6-003, T-M6-004, T-M6-005 |
| REQ-DEEP2-009a-SSE | Pipeline | Tests 20, 21, 30 | T-M4-011, T-M4-012, T-M4-013, T-M5-017, T-M5-018 |
| REQ-DEEP2-009a-Buffered | Pipeline | Tests 22, 23, 30 | T-M4-011, T-M4-012, T-M4-013 |
| REQ-DEEP2-009b-SSE | Pipeline | Tests 24, 25, 26, 30 | T-M4-011, T-M4-012, T-M4-013, T-M5-017, T-M5-018 |
| REQ-DEEP2-009b-Buffered | Pipeline | Tests 27, 28, 29, 30 | T-M4-011, T-M4-012, T-M4-013 |
| REQ-DEEP2-010 | Endpoint | Tests 4, 5 | T-M5-011, T-M5-012 |
| REQ-DEEP2-011 | Endpoint | Tests 6, 7, 8, 66 | T-M5-009, T-M5-010, T-M7-005, T-M7-006 |
| REQ-DEEP2-012 | Pipeline | Tests 31, 32, 33, 34, 35, 36 | T-M2-005, T-M2-006, T-M5-013, T-M5-014 |

All 15 REQs have ≥ 1 task per RED-GREEN cycle. ✓

### NFR Coverage (4 NFRs)

| NFR ID | Coverage | Tasks |
|--------|----------|-------|
| NFR-DEEP2-001 | Test 64 (budget a); /moai sync staging smoke test (budget b) | T-M7-003, T-M7-004 + sync-phase operational gate |
| NFR-DEEP2-002 | Tests 58, 59, 60, 61 | T-M6-001, T-M6-002, T-M6-003, T-M6-004, T-M6-005 |
| NFR-DEEP2-003 | Tests 6, 65, 66 | T-M3-003, T-M3-004, T-M5-003, T-M5-004, T-M7-005, T-M7-006 |
| NFR-DEEP2-004 | Verified via metric labels + cost field in agent_completed payload | T-M6-002 (collector exposure), T-M7-006 (documentation) |

All 4 NFRs have explicit verification tasks. ✓

### Test Catalog Coverage (66 tests, plan.md §3)

| Test # Range | Section | Tasks |
|--------------|---------|-------|
| 1-8 | §3.1 Endpoint | T-M5-009, T-M5-010, T-M5-011, T-M5-012, T-M7-005 |
| 9-19 | §3.2 Pipeline (Researcher/Reviewer/Writer/no-fanout) | T-M2-001 through T-M2-008, T-M3-001, T-M3-002 |
| 20-30 | §3.2 Pipeline (errors, max-retry, retry counter) | T-M4-007, T-M4-008, T-M4-011, T-M4-012, T-M4-013 |
| 31-36 | §3.2 Pipeline (empty fanout) | T-M2-005, T-M2-006, T-M5-013, T-M5-014 |
| 37-41 | §3.3 LLM Routing | T-M1-001, T-M1-002, T-M1-005, T-M2-007, T-M2-008 |
| 42-48 | §3.4 Verifier Gate | T-M4-001, T-M4-002, T-M4-003, T-M4-004, T-M4-005, T-M4-006 |
| 49-56 | §3.5 Streaming | T-M5-001, T-M5-002, T-M5-007, T-M5-008, T-M5-015, T-M5-016, T-M5-017, T-M5-018 |
| 57-61 | §3.5 Observability | T-M6-001, T-M6-002, T-M6-003, T-M6-004, T-M6-005 |
| 62-66 | §3.6 E2E + Property | T-M7-001, T-M7-002, T-M7-003, T-M7-004, T-M7-005, T-M7-006 |

All 66 tests are mapped to at least one task. ✓

### Definition of Done (from acceptance.md §1)

- [ ] `internal/deepagent/` 7 Go files + 3 test files created → T-M1-004, T-M2-008, T-M3-002, T-M4-006, T-M5-006, T-M6-002
- [ ] `internal/synthesis/faithfulness.go` + test → T-M4-002
- [ ] `internal/streamsynth/agent_events.go` + test → T-M5-002
- [ ] `internal/streamsynth/longform_source.go` (LongFormSource interface) → T-M3-004
- [ ] `services/researcher/src/researcher/faithfulness_endpoint.py` + test → T-M4-004
- [ ] `cmd/usearch-api/handlers/deep_agents_handler.go` + test → T-M5-010
- [ ] `cmd/usearch-api/handlers/synthesis.go` `?mode=` dispatch → T-M5-010
- [ ] `internal/obs/metrics/deepagent.go` + test → T-M6-002
- [ ] `internal/obs/metrics/metrics.go` `registerDeepAgent` → T-M6-002
- [ ] `internal/obs/obs.go` re-export → T-M6-002, T-M6-005
- [ ] `.env.example` 8 env-var documentation → T-M7-006
- [ ] All 15 REQs covered by GREEN tests → verified above
- [ ] 4 NFRs verified → verified above
- [ ] `go test -race ./...` GREEN on touched packages → T-M5-019, T-M7-006
- [ ] Coverage ≥ 85% all new Go files → T-M7-006
- [ ] `pytest --cov=researcher` ≥ 85% on new Python files → T-M4-004
- [ ] SPEC-DEEP-001 acceptance suite 100% green (regression) → T-M7-005, T-M7-006
- [ ] SPEC-SYN-002, SPEC-SYN-004 acceptance suites 100% green → T-M4-004 (preserves SYN-002), T-M5-002 (extends SYN-004 compat)
- [ ] TRUST 5 gates passed → T-M7-006 (quality gate consolidation)
- [ ] LSP gates passed (zero errors/type/lint) → T-M7-006
- [ ] @MX tags applied per plan.md §6 → per-milestone REFACTOR tasks + manager-tdd MX report
- [ ] Conventional commit `feat(deep): SPEC-DEEP-002 multi-agent pipeline` → /moai sync
- [ ] Pre-submission self-review (no simpler approach) → /moai run final gate
- [ ] /moai sync staging smoke test verifies NFR-DEEP2-001 budget (b) prod p95 ≤ 60 s → operational gate

### Completion Criterion for /moai sync

`/moai sync SPEC-DEEP-002` is permitted ONLY when:

1. All 62 tasks above are marked complete (TaskList all `completed`)
2. All 66 RED-phase tests from plan.md §3 catalog are GREEN
3. Coverage thresholds met on `internal/deepagent/`, `internal/synthesis/`, `internal/streamsynth/`, `services/researcher/` new files (≥ 85%)
4. SPEC-DEEP-001 acceptance suite remains 100% green (regression)
5. Cardinality guard `TestNoUnboundedLabels` remains green
6. `go test -race ./...` GREEN on all touched packages
7. `goleak` shows no leaked goroutines on cancellation paths
8. @MX tag report generated by manager-tdd per plan.md §6.5

Upon all conditions met, /moai sync flips spec.md `status: planned` → `status: implemented` and creates the conventional commit referencing this SPEC.

---

*End of SPEC-DEEP-002 tasks v0.1.2.*
