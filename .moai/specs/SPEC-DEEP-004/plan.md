# SPEC-DEEP-004 Implementation Plan

Generated: 2026-05-21
Methodology: TDD (RED-GREEN-REFACTOR)
Coverage target: 85%
Harness: standard

---

## 1. Overview

본 plan.md는 SPEC-DEEP-004의 구현 단계별 task sequence를 정의한다. 14 EARS
REQs + 10 NFRs를 6개 phase로 분해하며, 각 phase는 RED → GREEN → REFACTOR
사이클을 따른다. plan-auditor 통과 + annotation cycle 완료 후 본 plan은
manager-tdd 에이전트에게 전달되어 phase-별로 진행한다.

---

## 2. Phase Breakdown

### Phase A — Postgres Schema + Asynq Reconcile Foundation

목표: durable storage 계층과 reconciliation 인프라를 먼저 구축한다.
순서 근거: hot-path 코드가 이 계층에 의존하므로 가장 먼저 RED 테스트로 고정.

**RED tests** (4):

1. `TestMigration0002Idempotent` — `0002_cost_ledger.sql` 두 번 실행해도
   같은 schema 도달 (REQ-006).
2. `TestLedgerSchemaMatchesSpec` — pgx로 information_schema 쿼리해 컬럼
   집합·타입·제약 정확히 검증 (REQ-006).
3. `TestReconcileSchedulerRunsEvery5Min` — Asynq mock으로 schedule 등록
   확인 (REQ-008).
4. `TestReconcileDriftExceedingThresholdAlertsAndCorrects` — Redis 누적값
   $5.00, Postgres SUM $4.99 → 0.2% drift > 0.1% → alarm + Redis reset
   (REQ-008, NFR-005).

**GREEN tasks**:

- `deploy/postgres/migrations/0002_cost_ledger.sql` 작성 (idempotent
  `CREATE TABLE IF NOT EXISTS` + 3개 인덱스).
- `internal/deepagent/costguard/reconcile_job.go` Asynq scheduled task 등록
  + drift 측정 + Redis truth-reset.
- `internal/deepagent/costguard/ledger.go`의 schema 검증 helper.

**REFACTOR**:

- migration 번호 충돌 검사 helper(`scripts/check-migration-numbers.sh`)와
  공유 가능한 pgx connection pool 패턴 추출.

---

### Phase B — Redis Cap-Check Primitive + Lua Script

목표: cap 강제의 핵심인 atomic cap-check를 단독 모듈로 완성한다.

**RED tests** (5):

5. `TestCapCheckTenantCalls` — 20 calls 도달 시 cap 초과 반환 (REQ-009).
6. `TestCapCheckTenantUSD` — $5.00 도달 시 cap 초과 반환 (REQ-009).
7. `TestCapCheckUserCapWhenEnabled` — deep.yaml `user.enabled: true`일 때만
   user-level cap 강제 (REQ-009).
8. `TestCapCheckLuaScriptAtomic` — Redis Lua script 단일 호출로 두 차원
   평가 + counter 증가 + TTL refresh 모두 수행 (REQ-009, NFR-004).
9. `TestCapCheckConcurrent100RequestsNoRace` — goroutine 100개 동시 호출,
   cap이 20일 때 정확히 20만 통과, 80은 거부 (NFR-004).

**GREEN tasks**:

- `internal/deepagent/costguard/cap_check.go` 작성. Lua script 파일은
  `internal/deepagent/costguard/lua/cap_check.lua`로 embed.
- `internal/deepagent/costguard/types.go`에 `CapResult`, `CapDimension`
  enum 정의.

**REFACTOR**:

- Lua script 재사용성 개선 (per-tenant / per-user 분기 통합).
- error 메시지 사용자 친화 개선 (`remaining_calls`, `reset_at` 필드).

---

### Phase C — Haiku Pre-Screen Client + Circuit Breaker

목표: 진입 단계 게이트를 독립 모듈로 완성한다. LLM-001의 client 패턴을
재사용.

**RED tests** (5):

10. `TestHaikuScreenCallsLLMClientWithHaikuModel` — `llm.Client.Complete`
    호출 시 model alias가 deep.yaml 값으로 (REQ-003).
11. `TestHaikuScreenParsesScoreFromJSON` — 정상 JSON 응답에서 score, rationale,
    suggested_mode 추출 (REQ-003).
12. `TestHaikuScreenParseFailureIncrementsCounterAndFailsOpen` — 잘못된
    JSON → counter 1 증가 + fail-open (REQ-003).
13. `TestScreenScore7Proceeds` / `TestScreenScore5SuggestsBasic` /
    `TestScreenScore3Rejects` — 3-way 임계값 분기 (REQ-004).
14. `TestHaikuScreenBreakerOpensAfter5ConsecutiveFailures` — 5회 연속 5xx
    시 breaker state = open + 다음 30s 호출 건너뜀 (REQ-005).

**GREEN tasks**:

- `internal/deepagent/costguard/haiku_screen.go` 작성.
- circuit breaker는 LLM-001의 in-package ring buffer 패턴 재사용
  (research §10.2 reuse map).

**REFACTOR**:

- Haiku JSON 응답 schema를 별도 file로 분리 (`testdata/haiku_screen_schema.json`).

---

### Phase D — Cache Key Strategy + LiteLLM Integration

목표: LiteLLM 내장 캐시 활성화 + cache_key prefix로 cross-tenant 격리.

**RED tests** (4):

15. `TestLiteLLMCacheEnabledViaConfigYaml` — `deploy/litellm/config.yaml`에
    `cache: true` + `cache_params.type: redis` 검증 (REQ-012).
16. `TestCacheKeyPrefixIncludesTenantAndIntent` — LiteLLM custom cache-key
    callback(또는 cost guard wrapper layer)이 `SHA256(tenant_id ‖ intent_category
    ‖ model ‖ messages_json)` 형태의 salt를 적용하며 `messages` payload는
    변경되지 않음을 검증. 동일 query를 다른 tenant_id로 두 번 호출 시 cache hit
    이 발생하지 SHALL NOT 한다(REQ-012).
17. `TestCacheHitRecordedInLedger` — `x-litellm-cache-hit: true` 응답 →
    ledger row의 cache_hit = TRUE (REQ-013).
18. `TestCacheHitCounterIncrements` / `TestCacheAttemptCounterIncrementsOnEveryCall`
    — Prometheus 카운터 정확성 (REQ-013).

**GREEN tasks**:

- `internal/deepagent/costguard/cache_key.go` 작성.
- `deploy/litellm/config.yaml` 수정 (cache enable + TTL per tier).
- `internal/deepagent/costguard/metrics.go`에 hit/attempt counter 정의.

**REFACTOR**:

- cache key prefix 함수의 가독성 개선, intent category enum 검증.

---

### Phase E — Middleware + chi Wiring

목표: 4개 미들웨어를 단일 체인으로 결합하고 `/deep` 라우트에 wire.

**RED tests** (6):

19. `TestIdentityMiddlewareReadsXUserId` / `TestIdentityMiddlewareDefaultsAnonymous`
    / `TestIdentityMiddlewareDefaultTenantFromConfig` (REQ-001).
20. `TestCapExceededReturns429` / `TestCapExceededRetryAfterHeader` /
    `TestCapExceededResponseBodyShape` (REQ-010).
21. `TestDegradeHeaderFallsBackToBasic` (REQ-011).
22. `TestRedisOutageFailsClosed` / `TestRedisOutageFailOpenOverride` (REQ-014).
23. `TestLedgerWritesRowPerLLMCall` — middleware 종료 시 ledger row 1개
    추가 (REQ-006).
24. End-to-end integration via `httptest.NewServer`: 진짜 chi mux + 진짜
    Redis(testcontainers) + 진짜 Postgres(testcontainers).

**GREEN tasks**:

- `internal/deepagent/costguard/middleware.go` 작성. 미들웨어 체인 순서:
  Identity → CapCheck → HaikuScreen → (handler) → LedgerWrite.
- `cmd/usearch-api/handlers/synthesis.go` 수정해 `/deep` 라우트 앞에 chain
  부착.
- `cmd/usearch-api/main.go` 수정해 costguard.New 초기화 + reconcile job
  등록.

**REFACTOR**:

- middleware ordering 검증 helper, panic 안전성 확보.

---

### Phase F — Decision Event Log + Observability + Hot-Reload

목표: SPEC-AUTH-003 호환 decision event log(REQ-DEEP4-010 stderr JSON line),
OTel span, deep.yaml hot-reload.

**RED tests** (4):

25. `TestCapExceededIncrementsCounterAndDecisionLog` — JSON line per cap event
    stderr 출력. line schema는 §6.3 forward-compat 필드(`timestamp`, `event_type`,
    `request_id`, `tenant_id`, `user_id`, `decision`)를 모두 포함 (REQ-010).
26. `TestNoUnboundedLabels` (SPEC-OBS-001 기존 테스트 확장) — `tenant`,
    `status`, `tier`, `state` 라벨이 화이트리스트에 추가됨 확인 (NFR-007).
27. `TestOTelSpanAttributes` — `deep.cap.tenant_remaining_usd`, `deep.cache.
    hit_ratio`, `deep.screen.score` 등 span attribute 검증 (NFR-010).
28. `TestConfigHotReloadOnSIGHUP` — deep.yaml 수정 후 SIGHUP → 다음 요청
    부터 새 값 적용 (NFR-008).

**GREEN tasks**:

- `internal/deepagent/costguard/decision_log.go` (or inline in middleware.go)에
  decision event log JSON line emitter 작성.
- `internal/deepagent/costguard/config.go`에 fsnotify-based watcher 작성.
- OTel span attribute 부착 코드 추가.

**REFACTOR**:

- decision event log schema를 SPEC-AUTH-003 (M6) audit subsystem와 align하도록
  별도 struct에 centralize. additive-only schema 규칙(§6.3)을 코드 주석에 명시.

---

## 3. Test Catalog Summary

| Phase | Tests Added | REQs Covered | NFRs Covered |
|-------|-------------|--------------|--------------|
| A | 4 | 006, 008 | 005, 006 |
| B | 5 | 009 | 004 |
| C | 5 | 003, 004, 005 | 001 |
| D | 4 | 012, 013 | 003 |
| E | 6 | 001, 010, 011, 014, 006 | 002 |
| F | 4 | 010 | 007, 008, 009, 010 |
| **Total** | **28** | **14 / 14** | **10 / 10** |

추가로 REQ-DEEP4-002 (forward-compat with AUTH-001)는 Phase A의 schema
test와 Phase E의 context propagation test로 cover된다.

---

## 4. Risk Mitigation Table

| Risk | Phase | Mitigation Strategy |
|------|-------|---------------------|
| **R1** TOCTOU race on cap check | Phase B | Lua script atomic execution + `TestCapCheckConcurrent100RequestsNoRace` 검증 |
| **R2** Redis ↔ Postgres drift | Phase A | 5분 reconciliation job + drift > 0.1% 시 자동 보정 + `TestReconcileDriftExceedingThresholdAlertsAndCorrects` |
| **R3** Haiku circuit breaker fail-open behavior | Phase C | 5회 연속 실패 → 30s open 윈도우 + fail-open default. `TestHaikuScreenBreakerOpensAfter5ConsecutiveFailures` 검증 |
| **R4** Forward-compat break with AUTH-001 | Phase A + E | `cost_ledger.user_id` opaque TEXT 유지. Context key는 `costguard.UserIDKey` 단일 source. M6 schema review checkpoint 등록 |
| **R5** Python-side LLM cost 누락 | (out of scope) | research §1.4에 명시. SPEC-AUTH-003 (M6) audit log reconciliation으로 후속 처리 |
| **R6** Cross-tenant cache key collision | Phase D | cache key prefix에 `tenant_id` + `intent_category` 명시. `TestCacheKeyPrefixIncludesTenantAndIntent` |
| **R7** Haiku API drift (JSON schema 변경) | Phase C | JSON schema 검증 + parse 실패 시 fail-open + `usearch_deep_haiku_screen_parse_errors_total` 추적 |
| **R8** Redis 단절 전면 정지 | Phase E | `costguard.redis_failure_mode` 기본 fail-closed; 운영자가 fail-open 명시 가능. RehydrateWindow 자동 |

---

## 5. MX Tag Plan

본 SPEC의 구현은 다음 @MX 태그를 생성한다.

### 5.1 @MX:ANCHOR (high fan_in, invariant contract)

- `internal/deepagent/costguard/middleware.go::CapCheckMiddleware`
  — fan_in ≥ 3 (synthesis.go, /deep route, integration tests). cap-check
  미들웨어의 동작은 모든 `/deep` 호출의 invariant.
- `internal/deepagent/costguard/ledger.go::WriteLedgerEntry`
  — fan_in ≥ 3 (middleware, reconcile job, audit hook). 모든 LLM 호출의
  비용이 이 함수를 거친다.
- `internal/deepagent/costguard/cap_check.go::EvaluateAtomic`
  — fan_in ≥ 3. atomic 평가 함수의 의미를 다른 개발자에게 명시.

### 5.2 @MX:WARN (danger zone, requires @MX:REASON)

- `internal/deepagent/costguard/cap_check.go::loadLuaScript`
  — `@MX:WARN`: Lua script의 atomic 의미가 깨지면 cap이 사실상 무효해진다.
  `@MX:REASON`: Redis pipeline 또는 multi-key transaction을 단일 Lua
  실행으로 결합하기 때문에 코드 수정 시 atomic 의미 보존 필수.
- `internal/deepagent/costguard/haiku_screen.go::circuitBreakerOnFailure`
  — `@MX:WARN`: circuit breaker 실패가 누적되면 모든 호출이 fail-open으로
  통과해 cap 효용이 저하될 수 있다. `@MX:REASON`: open 윈도우 30s 동안
  실제 cost guard는 cap-check만 동작.
- `internal/deepagent/costguard/reconcile_job.go::flushRedisFromPostgres`
  — `@MX:WARN`: Redis truth-reset이 잘못 trigger되면 정상 누적값을 손실
  한다. `@MX:REASON`: drift > 0.1% 한도 조정 시 NFR-005 재검토 필요.

### 5.3 @MX:NOTE (context & intent delivery)

- `internal/deepagent/costguard/middleware.go` 파일 상단에 미들웨어 체인
  순서의 design rationale 기록.
- `internal/deepagent/costguard/cache_key.go::PrefixCacheKey`에 tenant +
  intent prefix의 보안적 의미 기록.

### 5.4 @MX:TODO (incomplete work — resolved in GREEN phase)

- RED phase에서 placeholder 함수에 `@MX:TODO`를 부착하고 GREEN phase
  종료 시 모두 제거.

---

## 6. File Touch Order (recommended TDD progression)

1. **Phase A start**: `deploy/postgres/migrations/0002_cost_ledger.sql` →
   `internal/deepagent/costguard/types.go` (schema struct) → `ledger.go`
   (write helpers) → `reconcile_job.go` → 4 tests.
2. **Phase B**: `internal/deepagent/costguard/lua/cap_check.lua` →
   `cap_check.go` → 5 tests.
3. **Phase C**: `internal/deepagent/costguard/haiku_screen.go` (with
   embedded circuit breaker) → 5 tests.
4. **Phase D**: `deploy/litellm/config.yaml` (cache enable) →
   `cache_key.go` → 4 tests.
5. **Phase E**: `middleware.go` → `cmd/usearch-api/handlers/synthesis.go`
   수정 → `cmd/usearch-api/main.go` 수정 → 6 tests (이 중 1개는 
   testcontainers 기반 integration).
6. **Phase F**: `audit.go` (or middleware-inline) → `config.go` hot-reload
   → OTel span 부착 → 4 tests.

---

## 7. Coverage and Quality Gates

- Coverage 목표: 85% (per `quality.yaml`).
- 새 package `internal/deepagent/costguard/`만 측정.
- TRUST 5 gates: 모든 phase 종료 시점에 `go vet` + `golangci-lint` +
  `go test -race` 통과.
- Cardinality test: `internal/obs/metrics/metrics_test.go::TestNoUnboundedLabels`
  화이트리스트에 신규 label 추가 후 통과.
- LSP gate: zero errors / zero type errors / zero lint errors.

---

## 8. Pre-submission Self-Review

전체 changeset이 완성된 시점에 다음을 확인한다:

- middleware 체인 순서가 §5.2의 design rationale을 반영하는가?
- Lua script의 atomic 의미가 단일 redis call로 보존되는가?
- circuit breaker의 state가 fail-open default를 유지하는가?
- `cost_ledger.user_id` 컬럼이 opaque TEXT인가? (AUTH-001 forward-compat)
- Prometheus 메트릭이 SPEC-OBS-001 명명 규칙(`usearch_deep_*`)을 따르는가?
- `tenant` label이 화이트리스트로 bounded되어 `unknown` collapse가 작동하는가?

---

## 9. Implementation Sequencing Across Sessions

본 SPEC의 6개 phase는 sequential 의존성을 가진다 (Phase B는 Phase A의
ledger row 형식을 참조 등). 단일 manager-tdd 세션으로 완주가 어려운 경우
다음 세션 분할이 권장된다:

- **Session 1**: Phase A + B (storage foundation + cap-check core)
- **Session 2**: Phase C + D (Haiku screen + cache integration)
- **Session 3**: Phase E + F (middleware wiring + observability + hot-reload)

각 세션 시작 시 `/clear` 후 본 plan.md만 재로드하여 컨텍스트를 보존한다.

---

*End of SPEC-DEEP-004 plan.*
