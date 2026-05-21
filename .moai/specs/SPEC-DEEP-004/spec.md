---
id: SPEC-DEEP-004
version: 0.1.1
status: draft
created: 2026-05-21
updated: 2026-05-21
author: limbowl
priority: P0
issue_number: 20
title: /deep quota and cost guard with Haiku pre-screen and prompt-cache reuse
milestone: M5 — /deep multi-agent
owner: expert-backend
methodology: tdd
coverage_target: 85
depends_on: [SPEC-DEEP-001, SPEC-DEEP-002, SPEC-DEEP-003, SPEC-LLM-001, SPEC-OBS-001, SPEC-IR-001, SPEC-CORE-001]
blocks: []
---

# SPEC-DEEP-004: /deep quota and cost guard with Haiku pre-screen and prompt-cache reuse

## HISTORY

- 2026-05-21 (v0.1.1 audit patches, limbowl via manager-spec):
  plan-auditor iter-1 PASS (overall 0.86) with 2 MAJOR + 8 MINOR + 1 NIT recommendations
  (`.moai/reports/plan-audit/SPEC-DEEP-004-review-1.md`). 본 amendment는 status 전이 전
  pre-approval 패치를 적용한다:
  - P-M1 (D1): REQ-DEEP4-010과 NFR-DEEP4-006 사이의 "audit log" vs "ledger row" 용어
    충돌을 해소. stderr JSON line은 "decision event log"로 통일하고, Postgres
    `cost_ledger` row의 durability는 "Ledger row durability"로 NFR-006을 rename.
    acceptance.md / spec-compact.md / plan.md의 stderr 용어도 같이 정합.
  - P-M2 (D2): §4 Exclusions에 10번째 항목 추가 — degraded-path cap-bypass loophole을
    의도된 design choice로 명시. `/basic` 평균 비용이 `/deep`의 ~1/30 수준이라는
    cost-asymmetry 근거 포함.
  - P-N1 (D3): REQ-DEEP4-003 EARS pattern label을 Ubiquitous → Event-Driven으로 정정하고
    lead-in을 "WHEN ..." 형태로 재작성.
  - P-N2 (D4): REQ-DEEP4-014에 Redis 복구 탐지 mechanism 명시 — health probe 주기
    5000ms (`costguard.redis.health_check_interval_ms`) + 연속 3회 성공 시 1회 트리거.
  - P-N3 (D5): NFR-DEEP4-002 단일 entry 내부에 성공 경로(p95 ≤ 50ms)와 실패-closed 경로
    (p99 wall-clock ≤ 2000ms) 분리 subclause 추가.
  - P-N4 (D6): REQ-DEEP4-012 cache_key salt를 messages 변경 없는 LiteLLM custom
    cache-key callback / wrapper layer 방식으로 명시. prompt contamination 위험 차단.
  - P-N5 (D7): NFR-DEEP4-009 metric 소유권 attribution 정정 — `usearch_deep_outcomes_total`
    의 owner는 SPEC-DEEP-001 (extended by SPEC-DEEP-002 REQ-DEEP2-008)로 수정.
  - P-N6 (D8): §6.1 SPEC-IR-001 status를 draft → implemented로 갱신 (IR-001 frontmatter
    검증 완료).
  - P-N7 (D10): §1 Overview에 Haiku pre-screen ↔ SPEC-IR-001 intent router의 orthogonal
    관계 명시 — 두 단계는 chained되지 않으며 IR-001의 category는 cache_key salt
    구성요소로만 소비된다.
  - P-N8 (D11): §6.3 forward-compatibility commitment를 확장 — REQ-DEEP4-010 decision
    event log JSON line schema가 SPEC-AUTH-003 (M6)에서 additive 확장 가능한 필드 집합
    (timestamp, event_type, request_id, tenant_id, user_id, decision)을 SHALL 포함한다고
    명시.
  - Deferred: D9 (DEEP-003 의존성 강도 수정), D12 (REQ-002/REQ-005 main GWT 추가) — 둘 다
    NIT / cosmetic으로 분류, manager-spec 판단에 따라 후속 amendment에서 다룰 수 있음.

  REQ/NFR 개수 변경 없음 (14 REQs + 10 NFRs 유지). NFR-DEEP4-002만 단일 entry 내부에서
  성공/실패 경로 subclause로 분리. Exclusion 항목은 9 → 10으로 1개 증가. spec-compact.md
  도 canonical 변경분을 미러링.

- 2026-05-21 (initial draft v0.1.0, limbowl via manager-spec):
  First EARS-formatted SPEC for the M5 release gate. `/deep` 멀티-에이전트
  서피스(DEEP-001/002/003)가 호출당 17~19건의 LLM 호출을 유발할 수 있어
  cost guard 없이는 GA 불가능하다. 본 SPEC은 4축(Haiku 사전 스크리닝,
  prompt-cache 재사용, per-tenant/per-user cap, durable cost ledger)을
  단일 미들웨어 계층으로 통합한다. 핵심 7개 결정사항은 research §7에서
  context-derived로 확정되었고 §1.1에 재명시한다.

  Pinned decisions:
  (D1) Identity는 HTTP 헤더 `X-User-Id` + `anonymous` fallback. SPEC-AUTH-001
       (M6)이 ship되면 JWT 미들웨어가 같은 헤더를 sub claim으로 채워
       schema 변경 없이 user-level cap이 활성화된다(forward-compat).
  (D2) Storage는 Postgres durable + Redis hot cache의 write-behind 패턴.
       Cap-check는 Redis Lua script로 원자적 실행, ledger 영구 저장은
       Asynq worker가 batch flush.
  (D3) Cap은 call-count와 $-amount 두 차원 모두 강제하며 먼저 도달한 축이
       기준이 된다. tenant 기본 한도는 20 calls/day OR $5/day.
  (D4) Haiku 점수 임계값: ≥6 proceed, 4-5 suggest /basic, <4 reject.
       임계값은 deep.yaml hot-reload로 운영 중 조정 가능.
  (D5) Cache 백엔드는 LiteLLM 내장 Redis cache 그대로. 별도 application
       cache layer 추가 없음.
  (D6) Cap 초과 시 기본 동작은 HTTP 429 + Retry-After. 호출자가
       `X-Allow-Degrade: 1` 헤더를 부착한 경우에 한해 `/basic` 모드로
       fallback.
  (D7) Audit 보존은 90일 hot in Postgres. archival은 M8 SPEC-AUDIT-002에
       위임.

  M5 release gate로서 본 SPEC은 별도 GitHub Issue로 트랙되지 않으며
  (`issue_number: 0`) plan-auditor 통과 후 status `draft → approved` 전이.

  Companion artifacts:
  - `.moai/specs/SPEC-DEEP-004/research.md` — Phase 0.5 research (~770 lines,
    10 sections — cost surface analysis, pre-AUTH identity strategy, Haiku
    screen pattern, LiteLLM cache reuse, storage architecture, 7 pinned
    decisions, observability surface, 10 risks)
  - `.moai/specs/SPEC-DEEP-004/plan.md` — TDD task sequence, 6-phase
    implementation, MX tag plan
  - `.moai/specs/SPEC-DEEP-004/acceptance.md` — Given/When/Then 시나리오
    (7 main + 1 boundary edge)
  - `.moai/specs/SPEC-DEEP-004/spec-compact.md` — compact view

  14 EARS REQs (12 × P0 + 2 × P1), 10 NFRs, 5 modules (Identity / Haiku Screen /
  Cost Ledger / Cap Enforcement / Cache & Observability). Methodology: TDD,
  coverage target 85%, harness: standard. Owner: expert-backend.

---

## 1. Overview

본 SPEC은 M5 milestone의 마지막 deliverable이자 `/deep` 멀티-에이전트
서피스의 release gate인 quota + cost guard 계층을 정의한다. SPEC-DEEP-001
(STORM 사이드카), SPEC-DEEP-002(4-agent pipeline), SPEC-DEEP-003(tree
exploration)이 호출당 17~19건의 LLM 호출 + $0.07~$0.19의 비용을 유발할 수
있어 안전망 없이 GA할 수 없다. 본 SPEC은 다음 4축을 단일 chi v5 미들웨어
계층으로 통합한다.

1. **Haiku 사전 스크리닝**: 진입 단계에서 query 적합성을 0-10 점수로 평가해
   `/basic`으로 충분한 질의를 차단한다.
2. **Prompt-cache 재사용**: LiteLLM 내장 Redis cache의 hit-rate를 24h 윈도우
   ≥30%로 운영 SLO화한다.
3. **Per-tenant/per-user cap**: 24h sliding window의 호출 수 + $-amount 두
   차원에 hard cap을 설정한다.
4. **Durable cost ledger**: 모든 LLM 호출의 토큰·비용·캐시-hit 여부를
   Postgres `cost_ledger`에 90일 hot retention으로 기록한다.

### 1.1 Pinned Architectural Decisions

다음 7개 결정은 research §7에서 context-derived로 확정되었다. 본 SPEC은
이를 EARS 요구사항으로 번역할 뿐 재논의하지 않는다.

1. **Identity 소스**: HTTP 헤더 `X-User-Id` + `anonymous` fallback.
   AUTH-001(M6) ship 시 JWT 미들웨어가 같은 헤더를 `sub` claim으로 채워
   schema 변경 없이 user-level cap 활성화. `cost_ledger.user_id` 컬럼은
   opaque TEXT NOT NULL DEFAULT 'anonymous'.
2. **Storage**: Postgres durable + Redis hot cache write-behind. Cap-check는
   Redis Lua script로 원자적, ledger 영구 저장은 Asynq worker가 batch flush.
3. **Cap 차원**: call-count AND $-amount 모두 강제, 먼저 도달한 축이 기준.
   tenant 기본 20 calls/day OR $5/day, user 기본 10 calls/day OR $2/day.
4. **Haiku 점수 임계값**: ≥6 proceed, 4-5 suggest /basic, <4 reject.
   deep.yaml hot-reload로 운영 중 조정 가능.
5. **Cache 백엔드**: LiteLLM 내장 Redis cache. 별도 application cache layer
   없음.
6. **Exceed 동작**: 기본 HTTP 429 + `Retry-After`. 호출자가
   `X-Allow-Degrade: 1` 헤더를 부착한 경우에 한해 `/basic` 모드 fallback.
7. **Audit 보존**: 90일 hot in Postgres. archival은 M8에 위임.

### 1.2 Motivation

`/deep` 서피스는 비싸고 느리다. 단일 호출이 평균 $0.07 / 30s, 최악 $0.19 /
60s를 소비한다. cost guard가 없으면 (a) 악의적 사용자가 quota 한도 없이
시스템을 고갈시킬 수 있고, (b) 일반 사용자가 `/basic`으로 충분한 질의에
`/deep`을 잘못 사용해 무의미한 비용·latency를 부담하며, (c) 운영자가 비용
현황을 가시화하지 못해 청구 사고에 노출된다. 본 SPEC은 이 세 가지를 단일
계층에서 해결한다.

### 1.3 Haiku Pre-Screen vs SPEC-IR-001 Intent Router (Orthogonality)

Haiku pre-screen(REQ-DEEP4-003/004/005)과 SPEC-IR-001 intent router는 직교
(orthogonal) gate로 운영된다. 두 단계는 chained되지 않으며 어느 한쪽의 결과가
상대편의 출력을 변경하지 않는다:

- SPEC-IR-001은 query를 intent category(예: `factual_short`, `research_long`)
  로 분류하여 REQ-DEEP4-012의 `cache_key` salt 구성요소로만 소비된다. IR-001은
  `/deep` vs `/basic` 라우팅 자체를 결정하지 않으며, 본 SPEC의 Haiku 분기 로직
  에도 입력되지 않는다.
- Haiku pre-screen은 query의 deep-warranted-ness(0-10 score)를 IR-001 category
  와 무관하게 독립적으로 판단한다. score 분기(≥6 proceed / 4-5 suggest-basic /
  <4 reject)는 IR-001 category를 참조하지 않는다.
- 두 단계의 실행 순서는 cap-check → Haiku pre-screen → (proceed 시) IR-001
  category 조회 → LLM 호출(cache_key에 category 반영)이며, IR-001은 LLM 호출
  레이어에서만 호출되어 cache_key 분리에 사용된다.

---

## 2. EARS Requirements

### 2.1 Identity Module

| ID | Pattern | Requirement | Priority | Acceptance |
|----|---------|-------------|----------|------------|
| **REQ-DEEP4-001** | Ubiquitous | cost guard 미들웨어는 모든 `/deep` 요청에서 HTTP 헤더 `X-User-Id`를 읽어 `user_id`를 결정 SHALL 한다. 헤더가 부재하거나 빈 문자열인 경우 `user_id = "anonymous"` 단일 공유 버킷으로 분류 SHALL 한다. 마찬가지로 `X-Tenant-Id` 헤더를 읽어 `tenant_id`를 결정하며 부재 시 deep.yaml `costguard.default_tenant_id` 값(기본 `"default"`)을 사용 SHALL 한다. 결정된 (`user_id`, `tenant_id`) tuple은 request context에 주입되어 downstream 미들웨어와 핸들러가 동일한 값을 관찰 SHALL 한다. (Acceptance §5.1, §5.2) | P0 | `TestIdentityMiddlewareReadsXUserId`, `TestIdentityMiddlewareDefaultsAnonymous`, `TestIdentityMiddlewareDefaultTenantFromConfig`, `TestIdentityContextPropagatesToHandler` |
| **REQ-DEEP4-002** | Optional | WHERE SPEC-AUTH-001(M6)이 ship되어 `auth-001-ga` 환경 변수가 truthy로 설정된 경우, identity middleware는 JWT의 `sub` claim을 우선 소스로 사용 SHALL 하고 `X-User-Id` 헤더는 보조 fallback으로만 사용 SHALL 한다. JWT 미디어웨어가 같은 컨텍스트 키(`costguard.UserIDKey`)에 user_id를 주입하기 때문에 본 SPEC의 schema나 downstream 코드는 변경되지 SHALL NOT 한다(forward-compatibility 보장). `cost_ledger.user_id` TEXT 컬럼은 V1 → V1.1 전환 시 마이그레이션을 SHALL NOT 요구한다. (Acceptance §6) | P1 | `TestIdentityForwardCompatWithAuth001`, `TestLedgerSchemaUnchangedAcrossV1ToV1_1Transition` |

### 2.2 Haiku Pre-Screen Module

| ID | Pattern | Requirement | Priority | Acceptance |
|----|---------|-------------|----------|------------|
| **REQ-DEEP4-003** | Event-Driven | WHEN `/deep` 요청이 cap-check를 통과한 직후, cost guard는 Haiku-tier 모델(`claude-haiku-4-5-20251001`을 deep.yaml `costguard.haiku_screen.model`로 override 가능)을 호출해 query의 multi-step research 필요성을 0-10 정수 점수로 평가 SHALL 한다. Haiku 호출은 singleton `llm.Client.Complete()`를 경유 SHALL 하며 LLM 응답은 JSON 객체 `{"score": int, "rationale": string, "suggested_mode": "deep"\|"basic"\|"reject"}` 스키마로 파싱 SHALL 된다. 파싱 실패 시 `usearch_deep_haiku_screen_parse_errors_total` 카운터를 1 증가시키고 fail-open(proceed) 처리 SHALL 한다. (Acceptance §5.3) | P0 | `TestHaikuScreenCallsLLMClientWithHaikuModel`, `TestHaikuScreenParsesScoreFromJSON`, `TestHaikuScreenParseFailureIncrementsCounterAndFailsOpen` |
| **REQ-DEEP4-004** | Event-Driven | WHEN Haiku pre-screen이 반환한 score가 deep.yaml `costguard.haiku_screen.threshold_proceed`(기본 6) 이상이면, cost guard는 본 `/deep` 파이프라인 진행 SHALL 한다. WHEN score가 `threshold_suggest`(기본 4) 이상이고 `threshold_proceed` 미만이면, cost guard는 HTTP 400 + body `{"error":"deep_not_warranted","suggested_mode":"basic","screen_score":N,"rationale":"..."}` 응답 SHALL 한다. WHEN score가 `threshold_suggest` 미만이면, cost guard는 HTTP 400 + body `{"error":"query_rejected_by_screen","screen_score":N,"rationale":"..."}` 응답 SHALL 한다. 세 분기 모두 Haiku 호출 자체의 비용은 `cost_ledger`에 `outcome="screen_only"`로 기록 SHALL 된다. (Acceptance §5.3) | P0 | `TestScreenScore7Proceeds`, `TestScreenScore5SuggestsBasic`, `TestScreenScore3Rejects`, `TestScreenCostRecordedRegardlessOfOutcome` |
| **REQ-DEEP4-005** | Unwanted | IF Haiku pre-screen 호출이 deep.yaml `costguard.haiku_screen.timeout_ms`(기본 200ms) 이내에 응답을 반환하지 못하거나 비복구성 LLM 에러를 반환하면, cost guard는 score 평가를 SHALL 생략하고 본 `/deep` 파이프라인을 fail-open으로 진행 SHALL 한다(단, deep.yaml `costguard.haiku_screen.fail_open_on_timeout: false`로 명시 비활성화된 경우 HTTP 503 + body `{"error":"screen_unavailable"}`로 거부). 연속 5회 timeout 또는 5xx 발생 시 circuit breaker가 open으로 전환되어 다음 30초 동안 모든 호출에서 스크리닝을 건너뛰며, `usearch_deep_haiku_screen_breaker_state{state}` 게이지가 `"open"`으로 1, 다른 상태는 0이 SHALL 된다. (Acceptance §5.3) | P0 | `TestHaikuScreenTimeoutFailsOpen`, `TestHaikuScreenBreakerOpensAfter5ConsecutiveFailures`, `TestHaikuScreenBreakerStateGaugeEmitted` |

### 2.3 Cost Ledger Module

| ID | Pattern | Requirement | Priority | Acceptance |
|----|---------|-------------|----------|------------|
| **REQ-DEEP4-006** | Ubiquitous | 모든 Go-side `llm.Client` LLM 호출의 결과는 `cost_ledger` 테이블에 row를 SHALL 기록한다. row 컬럼: `(id, user_id, tenant_id, request_id, deep_run_id, model, prompt_tokens, completion_tokens, usd_cost, cache_hit, intent_category, outcome, ts)`. `outcome`은 enum {`success`, `error`, `capped`, `degraded`, `screen_only`} 중 하나이며 cap 초과로 거부된 호출은 LLM이 실행되지 않으므로 본 row를 기록하지 SHALL NOT 한다(단, cap 초과 이전에 실행된 Haiku pre-screen 호출은 `outcome="screen_only"`로 기록). `deep_run_id`는 단일 `/deep` 요청의 모든 LLM 호출을 묶는 UUID로 nullable이며 pre-screen-only 호출에서는 NULL이 허용된다. 마이그레이션 파일은 `deploy/postgres/migrations/0002_cost_ledger.sql`로 신설 SHALL 된다. (Acceptance §5.4, §5.7) | P0 | `TestLedgerSchemaMatchesSpec`, `TestLedgerWritesRowPerLLMCall`, `TestLedgerOutcomeEnumEnforced`, `TestLedgerDeepRunIdNullableForScreenOnly`, `TestMigration0002Idempotent` |
| **REQ-DEEP4-007** | Ubiquitous | cost guard는 LLM 응답 수신 직후 Redis hot cache의 24h sliding window 누적값을 INCR로 SHALL 갱신한다. Redis 키 패턴: `costguard:bucket:tenant:{tenant_id}:{YYYY-MM-DDTHH}` (usd_cost), `costguard:bucket:user:{user_id}:{YYYY-MM-DDTHH}` (usd_cost), 그리고 `:calls:` 접두사로 호출 횟수. 동시에 Asynq queue `cost-ledger-write`에 row 작업을 enqueue SHALL 한다. Asynq worker는 batch(최대 100 row 또는 5초 timeout)로 Postgres에 INSERT SHALL 한다. Redis INCR 실패 시 동기적으로 retry(최대 3회 exponential backoff)하며, 모두 실패하면 fail-closed(429 응답)로 SHALL 처리한다. (Acceptance §5.7) | P0 | `TestLedgerRedisIncrOnLLMResponse`, `TestLedgerAsynqEnqueueAfterRedisIncr`, `TestLedgerBatchFlushTo100Rows`, `TestLedgerRedisFailureFailsClosed` |
| **REQ-DEEP4-008** | Ubiquitous | 5분 주기 Asynq scheduled job `cost-ledger-reconcile`는 Postgres `cost_ledger`에서 직전 5분 row의 SUM(usd_cost)을 (tenant_id, user_id)별로 집계 SHALL 하고 Redis hot cache의 같은 윈도우 누적값과 비교 SHALL 한다. drift가 NFR-DEEP4-005의 0.1% 한도를 초과하면 `usearch_deep_ledger_drift_alerts_total` 카운터를 1 증가시키고 Redis 값을 Postgres truth로 자동 재설정 SHALL 한다. Redis가 unreachable인 경우 reconciliation job은 SHALL NOT 실패하며 다음 주기에 재시도 SHALL 한다. (Acceptance §5.7) | P0 | `TestReconcileSchedulerRunsEvery5Min`, `TestReconcileDriftExceedingThresholdAlertsAndCorrects`, `TestReconcileSurvivesRedisOutage` |

### 2.4 Cap Enforcement Module

| ID | Pattern | Requirement | Priority | Acceptance |
|----|---------|-------------|----------|------------|
| **REQ-DEEP4-009** | Ubiquitous | cost guard 미들웨어는 `/deep` 요청 진입 시점에 Redis Lua script를 통해 atomic하게 다음 두 차원을 SHALL 평가한다: (a) `(tenant_id)`의 24h sliding window 누적 호출 횟수와 deep.yaml `costguard.tenant.max_calls_per_day`(기본 20) 비교, (b) `(tenant_id)`의 24h sliding window 누적 USD 비용과 `costguard.tenant.max_usd_per_day`(기본 5.00) 비교. WHERE deep.yaml `costguard.user.enabled: true`인 경우 동일 두 차원을 user-level cap에도 적용 SHALL 한다. Lua script는 두 차원 평가 + counter increment + TTL refresh를 단일 Redis call로 SHALL 실행하여 동시 요청의 race를 차단한다. (Acceptance §5.1, §5.2) | P0 | `TestCapCheckTenantCalls`, `TestCapCheckTenantUSD`, `TestCapCheckUserCapWhenEnabled`, `TestCapCheckLuaScriptAtomic`, `TestCapCheckConcurrent100RequestsNoRace` |
| **REQ-DEEP4-010** | Event-Driven | WHEN tenant의 24h 누적 호출 수 또는 USD 비용이 cap을 초과하면, cost guard는 HTTP 429 응답을 SHALL 반환한다. 응답 헤더에 `Retry-After: {다음 sliding window 만료까지 초}`를 SHALL 부착 하고, 응답 본문은 `{"error":"cap_exceeded","dimension":"calls"\|"usd","remaining":{"calls":N,"usd":F},"reset_at":"YYYY-MM-DDTHH:MM:SSZ"}` 형식으로 SHALL emit한다. 카운터 `usearch_deep_calls_total{tenant,status="capped"}`를 1 증가 SHALL 시키고 decision event log(stderr-emitted JSON line, NFR-DEEP4-006의 Postgres `cost_ledger` ledger row와는 별개 아티팩트)에 `decision="deny"` event를 SHALL 출력한다. cap 초과로 거부된 호출은 Haiku 호출이나 본 `/deep` 파이프라인 호출을 SHALL NOT 트리거한다. (Acceptance §5.1, §5.2) | P0 | `TestCapExceededReturns429`, `TestCapExceededRetryAfterHeader`, `TestCapExceededResponseBodyShape`, `TestCapExceededIncrementsCounterAndDecisionLog`, `TestCapExceededSkipsHaikuAndPipeline` |
| **REQ-DEEP4-011** | Optional | WHERE 요청 헤더 `X-Allow-Degrade: 1`이 부착된 상태에서 cap이 초과되면, cost guard는 HTTP 429 대신 SPEC-SYN-001 `/basic` 모드로 SHALL fallback한다. 응답은 HTTP 200이며 응답 헤더에 `X-Deep-Degraded: cap-exceeded`를 SHALL 부착한다. `/basic` 호출의 비용 자체는 `cost_ledger`에 `outcome="degraded"`로 별도 row로 SHALL 기록되지만, cap 평가에는 산입 SHALL NOT 된다(이미 cap을 초과한 호출자의 fallback이므로 추가 cap 평가는 무의미). 카운터 `usearch_deep_calls_total{tenant,status="degraded"}`를 1 증가 SHALL 시킨다. (Acceptance §5.6) | P1 | `TestDegradeHeaderFallsBackToBasic`, `TestDegradedResponseHeaderEmitted`, `TestDegradedLedgerRowOutcomeDegraded`, `TestDegradedCounterIncrements` |

### 2.5 Cache Reuse & Observability Module

| ID | Pattern | Requirement | Priority | Acceptance |
|----|---------|-------------|----------|------------|
| **REQ-DEEP4-012** | Ubiquitous | LiteLLM 내장 Redis-backed prompt cache는 deploy 시점에 활성화 SHALL 되며 (`deploy/litellm/config.yaml`의 `litellm_settings.cache: true`, `cache_params.type: redis`). 본 SPEC은 별도의 application-level cache layer를 추가하지 SHALL NOT 한다. cache TTL은 모델 티어별로 다음과 같이 SHALL 설정한다: Haiku pre-screen 1h, Researcher 24h, Reviewer/Writer/Verifier 24h. cache_key는 LiteLLM 기본 동작(SHA256(model + messages_json)) 위에 `tenant_id` + `intent_category`를 cache-key salt로 SHALL 적용한다. salt는 LiteLLM의 custom cache-key callback(또는 cost guard wrapper layer)에서 `SHA256(tenant_id ‖ intent_category ‖ model ‖ messages_json)` 형태로 합성되며 `messages` 페이로드 자체를 변경하지 SHALL NOT 한다 — 즉 LLM은 salt 값을 인지하지 못하고 prompt completion이 오염되지 않는다. cross-tenant cache key collision은 이 salt 차원에서 차단된다. (Acceptance §5.4, §5.5) | P0 | `TestLiteLLMCacheEnabledViaConfigYaml`, `TestNoApplicationLevelCacheLayerAdded`, `TestCacheTTLPerTier`, `TestCacheKeyPrefixIncludesTenantAndIntent` |
| **REQ-DEEP4-013** | Event-Driven | WHEN LiteLLM 응답에 `x-litellm-cache-hit: true` 헤더가 포함되면, cost guard는 해당 LLM 호출에 대응하는 `cost_ledger` row의 `cache_hit` 컬럼을 SHALL `TRUE`로 기록 SHALL 하고 `usd_cost`는 LiteLLM이 보고하는 캐시-감면된 값(보통 0)을 사용 SHALL 한다. 카운터 `usearch_deep_cache_hits_total{tier}`를 1 증가시키며, `usearch_deep_cache_attempts_total{tier}` 카운터는 hit/miss 무관 모든 LLM 호출마다 1 증가 SHALL 한다. 두 카운터의 24h 비율이 deep.yaml `costguard.cache.hit_rate_target_pct`(기본 30) 미만이면 `usearch_deep_cache_hit_rate_below_target` 게이지를 1로 SHALL 설정한다. (Acceptance §5.4, §5.5) | P0 | `TestCacheHitRecordedInLedger`, `TestCacheHitCounterIncrements`, `TestCacheAttemptCounterIncrementsOnEveryCall`, `TestCacheHitRateBelowTargetGaugeEmitted` |
| **REQ-DEEP4-014** | Unwanted | IF Redis hot cache가 unreachable이고 deep.yaml `costguard.redis_failure_mode: "fail-closed"`(기본값)인 경우, cost guard는 모든 `/deep` 요청을 HTTP 503 + body `{"error":"costguard_unavailable","detail":"redis unreachable"}`로 SHALL 거부한다. WHERE `redis_failure_mode: "fail-open"`로 명시 override된 경우에 한해 cap 평가를 skip하고 본 파이프라인을 진행 SHALL 한다. Redis 복구는 `costguard.redis.health_check_interval_ms`(기본 5000ms) 주기 health probe(`PING` 또는 트리비얼 `EXISTS` 호출)로 감지하며, 연속 3회 health check 성공 후 Asynq job `costguard.RehydrateWindow`가 1회 트리거되어 Postgres `cost_ledger`에서 24h window를 자동 재구성 SHALL 한다. 어떤 failure mode에서도 Haiku pre-screen은 정상 실행되며 본 파이프라인이 차단되는 경우에도 `cost_ledger`에 `outcome="error"` row가 기록 SHALL 된다. (Acceptance §5.7) | P0 | `TestRedisOutageFailsClosed`, `TestRedisOutageFailOpenOverride`, `TestRedisRehydrateFromPostgresOnRecovery`, `TestRedisOutageStillRecordsErrorRow` |

---

## 3. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| **NFR-DEEP4-001** | Haiku pre-screen latency | Haiku pre-screen 호출의 wall-clock latency는 p95 ≤ 200ms SHALL 한다. 200ms를 초과하면 REQ-DEEP4-005의 fail-open으로 전환된다. 측정 방법: `internal/deepagent/costguard/haiku_screen_test.go`의 50-iteration 통계, 그리고 production에서는 `usearch_deep_haiku_screen_duration_seconds` Histogram의 p95 quantile. |
| **NFR-DEEP4-002** | Ledger write latency | 두 경로에 대해 별도 budget을 명시 SHALL 한다. (a) **성공 경로**(Redis INCR + Asynq enqueue 모두 정상): 합산 wall-clock latency p95 ≤ 50ms. (b) **실패-closed 경로**(REQ-DEEP4-007의 3회 exponential backoff retry — 250ms+500ms+1000ms — 후 429 응답): 합산 wall-clock p99 ≤ 2000ms. Postgres 영구 저장은 write-behind이므로 두 경로 모두 본 NFR의 budget 밖이다. 측정: 성공 경로는 `usearch_deep_ledger_write_duration_seconds` Histogram, 실패 경로는 `usearch_deep_ledger_failclose_duration_seconds` Histogram. |
| **NFR-DEEP4-003** | Cache hit-rate target | 단일 팀(`tenant_id` fixed) 24h rolling window에서 `usearch_deep_cache_hits_total` / `usearch_deep_cache_attempts_total` 비율은 ≥ 30% SHALL 유지된다(`.moai/project/product.md`의 "dedup ≥ 30%" 지표). 미달 시 `usearch_deep_cache_hit_rate_below_target` 게이지가 1로 emit되며 운영자 알람 트리거. |
| **NFR-DEEP4-004** | Cap check atomicity | Redis Lua script로 실행되는 cap-check + counter-update의 wall-clock latency는 p95 ≤ 10ms SHALL 한다. 동시 100건 요청 시나리오(`TestCapCheckConcurrent100RequestsNoRace`)에서 cap 한도 초과 호출 수가 정확히 0이어야 한다(TOCTOU race 미발생). 측정: `usearch_deep_cap_check_duration_seconds` Histogram. |
| **NFR-DEEP4-005** | Ledger reconciliation drift | 5분 주기 reconciliation job이 측정한 Redis hot cache와 Postgres `cost_ledger` 간 누적 USD drift는 ≤ 0.1% SHALL 한다(즉, $5/day cap 기준 $0.005 이내). 초과 시 자동 보정 + 알람. 측정: `usearch_deep_ledger_drift_ratio` Gauge. |
| **NFR-DEEP4-006** | Ledger row durability | Postgres `cost_ledger` 테이블에 기록되는 모든 ledger row(REQ-DEEP4-006 스키마)는 synchronous commit(`synchronous_commit = on`)으로 fsync 되어 단일 노드 crash 시에도 손실되지 SHALL NOT 한다. 본 NFR은 ledger row(Postgres 영구 저장 아티팩트)에만 적용되며, REQ-DEEP4-010이 emit하는 decision event log(stderr JSON line, 별개 아티팩트)의 durability는 본 SPEC의 범위 밖이다 — decision event log는 stderr 캡처/로그 수집 파이프라인의 보장에 위임한다. WAL replication 또는 backup 정책은 본 SPEC의 범위 밖이며 별도 deploy SPEC이 다룬다. |
| **NFR-DEEP4-007** | No PII in metric labels | 본 SPEC이 신설하는 모든 Prometheus 메트릭 label 값은 bounded enumerable set이며 (a) user_id, (b) query 본문, (c) request_id, (d) rationale 텍스트, (e) IP 주소 등 PII나 high-cardinality 값을 SHALL NOT 포함한다. `tenant` label은 deploy 시점에 `costguard.allowed_tenants` 화이트리스트로 bounded되며, 화이트리스트 외 값은 `tenant=unknown`으로 collapse 된다. 검증: SPEC-OBS-001의 `TestNoUnboundedLabels` 통과. |
| **NFR-DEEP4-008** | Configurability via deep.yaml hot-reload | deep.yaml의 `costguard.*` 모든 설정은 서비스 재시작 없이 SIGHUP 신호 또는 file watcher로 hot-reload 가능 SHALL 한다. hot-reload 후 다음 요청부터 새 값이 적용되며, 진행 중인 요청은 reload 이전 값으로 완료된다. config 변경 시점은 `usearch_deep_costguard_config_version` 게이지가 새 hash로 emit된다. |
| **NFR-DEEP4-009** | Prometheus metric naming convention | 본 SPEC이 신설하는 모든 메트릭은 SPEC-OBS-001의 명명 규칙(`usearch_<domain>_<noun>_<unit>` 패턴)을 SHALL 준수한다. 신설 메트릭 namespace는 `usearch_deep_*`이며 일부 메트릭(`usearch_deep_calls_total`)은 SPEC-DEEP-001이 소유하고 SPEC-DEEP-002 REQ-DEEP2-008이 outcome label을 확장한 기존 `usearch_deep_outcomes_total`과 별개로 운영된다(전자는 cap-status 분류, 후자는 pipeline outcome). |
| **NFR-DEEP4-010** | OTel span attribute convention | `/deep` 요청에 대응하는 OTel span(`deep.request`)에 `deep.cap.tenant_remaining_usd`, `deep.cap.tenant_remaining_calls`, `deep.cache.hit_ratio`, `deep.screen.score`, `deep.screen.outcome` 등 본 SPEC이 정의하는 span 속성을 SHALL 부착한다. span attribute는 SPEC-OBS-001 §6의 명명 규칙(`<domain>.<subdomain>.<attribute>` snake_case)을 따른다. attribute 값은 cardinality 제약이 없으므로 user_id, request_id 같은 high-cardinality 값을 안전하게 포함할 수 있다(NFR-DEEP4-007 적용 범위 밖). |

---

## 4. Exclusions (What NOT to Build)

[HARD] 본 SPEC은 다음 항목을 명시적으로 제외한다. 각 항목은 후속 SPEC 또는
별도 트랙의 책임이다.

- **결제·빌링 통합 없음** — 본 SPEC은 비용을 측정하고 cap만 강제한다.
  Stripe, billing portal, invoice 생성 등의 결제 시스템 통합은 본 SPEC
  범위 밖이며 별도 M9+ commercial SPEC이 다룬다.
- **글로벌 비용 대시보드 없음** — Prometheus 메트릭만 emit하며, Grafana
  대시보드 JSON 정의·자동 deploy는 본 SPEC 범위 밖. M8 SPEC-EVAL-002가
  통합 대시보드를 ship한다.
- **소급(retroactive) quota 환불 없음** — cap에 의해 거부된 호출이 사후에
  정당하다고 판명되어도 `cost_ledger`에서 row를 삭제하거나 quota를
  복원하지 않는다. 운영자의 수동 보정은 직접 SQL INSERT로만 지원한다.
- **JWT 기반 per-user 인증 직접 구현 안 함** — `X-User-Id` 헤더의 부착자
  검증, JWT 서명 검증, claim 추출은 SPEC-AUTH-001(M6)의 책임. 본 SPEC은
  헤더를 단순 trust한다(self-host 환경에서 상위 프록시가 검증한 것으로
  간주).
- **다중 조직(multi-org) cap 계층 없음** — V1은 (tenant, user) 2-tier만
  지원한다. 조직 → 팀 → 사용자 3-tier hierarchy는 SPEC-AUTH-004(M7)에
  위임. tenant_id는 deploy 시점에 단일 화이트리스트로 관리된다.
- **실시간 cost websocket push 없음** — 호출자가 응답 본문의 `remaining`
  필드를 통해 잔여 quota를 인지하는 pull 모델만 지원. server-side push로
  실시간 비용 변동을 알리는 기능은 별도 M8 frontend SPEC이 다룬다.
- **ML 기반 Haiku 점수 자동 튜닝 없음** — 임계값 6/4는 수동 설정이며
  deep.yaml hot-reload로 조정한다. 사용자 피드백 기반 자동 임계값 학습은
  SPEC-EVAL-003(M9)에 위임.
- **Python-side LiteLLM 호출의 ledger 적용 없음** — research §1.4에 명시한
  대로 본 SPEC v1은 Go-side `llm.Client` 호출만 hook한다. `services/researcher`,
  `services/storm` 등 Python 사이드카가 직접 LiteLLM SDK로 호출하는 비용은
  LiteLLM 프록시 자체의 spend logs로 후속 reconciliation(SPEC-AUTH-003 M6).
- **Anthropic-native prompt caching(`cache_control: ephemeral`) 활용 없음** —
  본 SPEC v1은 LiteLLM 내장 Redis cache만 사용한다. Anthropic-native cache는
  vendor-specific이며 cross-provider portability를 해친다. SPEC-COST-OPT-001
  (M8)이 다룬다.
- **Degraded-path 비용 제한 부재** — REQ-DEEP4-011에 따라 `X-Allow-Degrade: 1`
  헤더로 cap 초과 후 `/basic` 모드로 fallback된 호출은 `outcome="degraded"`로
  ledger에 기록되지만 cost guard cap 평가에서는 SHALL NOT 산입된다. 이는 의도된
  design choice이며 abuse vector가 아니다: `/basic` 평균 비용은 호출당 약 $0.002
  (단일 LLM 합성, prompt+completion ~1.5K tok @ Sonnet)로 `/deep` 평균 $0.05~$0.20
  대비 약 1/30 수준이라 cap 초과자가 `X-Allow-Degrade`로 무한 호출하더라도 비용
  damage는 자체적으로 bound된다. 추후 `/basic` 자체에 별도 cap(예: `costguard.
  degrade.max_per_day`)을 도입할지 여부는 M8 또는 별도 후속 SPEC에서 운영 데이터
  기반으로 재평가한다. cf. REQ-DEEP4-011.

---

## 5. Acceptance Scenarios

상세 Given/When/Then 시나리오는 `.moai/specs/SPEC-DEEP-004/acceptance.md`에
정의되어 있다. 본 절은 인덱스를 제공한다.

| Scenario | 설명 | Coverage |
|----------|------|----------|
| §5.1 | anonymous 호출자가 daily call 한도 도달 → 429 + Retry-After | REQ-001, 009, 010 |
| §5.2 | X-User-Id 헤더 호출자가 $-cap 도달 → 429 + 잔여=0 응답 | REQ-001, 009, 010 |
| §5.3 | Haiku 점수 3 → 즉시 거부 응답 (이유 포함, 비용은 ledger 기록) | REQ-003, 004, 006 |
| §5.4 | 동일 query 24h 내 재호출 → cache hit, ledger에 cache_hit=true | REQ-006, 012, 013 |
| §5.5 | LiteLLM 캐시 hit-rate가 24h window에서 30% 이상 측정됨 | REQ-013, NFR-003 |
| §5.6 | X-Allow-Degrade: 1 + cap 초과 → /basic synth로 fallback, 200 | REQ-011 |
| §5.7 | Redis 단절 시 Postgres에서 window 재구성하여 cap check 정상 수행 | REQ-007, 008, 014 |
| Edge | cap 잔여 1 → 그 호출 성공, 다음 호출 429 (경계 케이스) | REQ-009, 010 |

---

## 6. Dependencies & Blocks

### 6.1 Upstream SPEC dependencies (depends_on)

- **SPEC-DEEP-001** (implemented) — STORM 사이드카의 LLM 호출 진입점을
  본 SPEC이 hook하여 ledger를 기록.
- **SPEC-DEEP-002** (draft) — 4-agent pipeline의 17~19건 호출이 본 SPEC의
  cap 강제 대상.
- **SPEC-DEEP-003** (draft) — tree exploration의 breadth/depth가 cap 차원
  설계에 직접 반영(최악-경우 호출 횟수 추산).
- **SPEC-LLM-001** (implemented) — `llm.Client`가 모든 LLM 호출의 단일
  choke point. 본 SPEC의 ledger write는 LLM-001의 cost middleware 패턴을
  확장한다.
- **SPEC-OBS-001** (implemented) — 신설 Prometheus 메트릭이 OBS-001
  registry에 등록되며 NFR-OBS-002 cardinality safety를 SHALL 준수한다.
- **SPEC-IR-001** (implemented) — IR-001의 intent category는 본 SPEC의
  cache_key salt(REQ-DEEP4-012) 구성요소로만 소비된다. Haiku pre-screen과
  IR-001은 §1.3에 명시한 대로 orthogonal gate이며 chained되지 않는다. cap이
  통과한 후 `/deep` 라우팅은 IR-001의 기존 로직 그대로 동작.
- **SPEC-CORE-001** (implemented) — `tenant_id` 컬럼 정의 및 NormalizedDoc
  tenant scoping의 일관성 유지.

### 6.2 Downstream blocked SPECs (blocks)

본 SPEC은 직접 블로킹하는 M5 SPEC이 없다. M5 release gate로서 본 SPEC이
ship되면 `/deep` 전체 서피스가 GA 가능 상태에 진입한다.

### 6.3 Forward-compatibility commitment with AUTH-001 and AUTH-003

REQ-DEEP4-002가 명시한 대로, SPEC-AUTH-001(M6)가 ship되어도 본 SPEC의
schema·코드는 변경되지 SHALL NOT 한다. AUTH-001은 JWT 미들웨어가 같은
context key(`costguard.UserIDKey`)에 user_id를 주입하도록 구현되며, 본 SPEC의
`cost_ledger.user_id` 컬럼은 opaque TEXT를 유지하여 마이그레이션이 불필요
하다. M6 진입 시 schema review checkpoint가 본 commitment를 재검증한다.

REQ-DEEP4-010의 decision event log JSON line schema 또한 SPEC-AUTH-003(M6)
audit subsystem이 downstream consumer로 합류할 때 호환되도록 설계된다.
decision event log line은 다음 필드를 SHALL 포함한다: `timestamp`(ISO-8601),
`event_type`(예: `cap.evaluation`), `request_id`, `tenant_id`, `user_id`,
`decision`(예: `"allow"`, `"deny"`, `"degrade"`). schema는 **additive**이며
SPEC-AUTH-003는 새 필드를 추가할 수 있으나 위 필드를 rename하거나 remove할
수 SHALL NOT 한다. AUTH-003 진입 시 audit subsystem review checkpoint가 본
commitment를 재검증한다.

---

## 7. Files to Create / Modify

### 7.1 Created

| Marker | Path | Purpose |
|--------|------|---------|
| [NEW] | `internal/deepagent/costguard/middleware.go` | chi v5 미들웨어 — identity / cap-check / Haiku screen / ledger write 4-step orchestration |
| [NEW] | `internal/deepagent/costguard/ledger.go` | `cost_ledger` row 작성, Redis INCR, Asynq enqueue |
| [NEW] | `internal/deepagent/costguard/haiku_screen.go` | Haiku LLM 호출, JSON 파싱, circuit breaker |
| [NEW] | `internal/deepagent/costguard/cache_key.go` | LiteLLM cache key prefix 구성 (tenant + intent) |
| [NEW] | `internal/deepagent/costguard/cap_check.go` | Redis Lua script 로딩 + cap evaluation 함수 |
| [NEW] | `internal/deepagent/costguard/reconcile_job.go` | Asynq scheduled job — Redis ↔ Postgres drift 보정 |
| [NEW] | `internal/deepagent/costguard/config.go` | deep.yaml `costguard.*` 로더 + hot-reload watcher |
| [NEW] | `internal/deepagent/costguard/metrics.go` | Prometheus collector 등록 헬퍼 |
| [NEW] | `internal/deepagent/costguard/types.go` | `CapResult`, `LedgerEntry`, `HaikuScreenResult`, `CapDimension` enum 등 |
| [NEW] | `internal/deepagent/costguard/middleware_test.go` | Identity, cap-check, screen orchestration 통합 테스트 |
| [NEW] | `internal/deepagent/costguard/ledger_test.go` | Redis INCR + Asynq enqueue + Postgres flush 테스트 |
| [NEW] | `internal/deepagent/costguard/haiku_screen_test.go` | Haiku 호출 timeout, breaker, JSON 파싱 실패 fail-open |
| [NEW] | `internal/deepagent/costguard/cap_check_test.go` | Lua script 원자성, 동시 100 요청 race 차단 |
| [NEW] | `internal/deepagent/costguard/reconcile_job_test.go` | drift 검출, 자동 보정, Redis outage 복구 |
| [NEW] | `internal/deepagent/costguard/config_test.go` | deep.yaml 파싱, hot-reload, 기본값 fallback |
| [NEW] | `deploy/postgres/migrations/0002_cost_ledger.sql` | `cost_ledger` 테이블 생성 마이그레이션 |
| [NEW] | `.moai/config/sections/deep.yaml` | `costguard.*` 설정 신설 (없으면 신설, 있으면 §1.1 D4 정책으로 확장) |

### 7.2 Modified

| Path | Change |
|------|--------|
| `cmd/usearch-api/handlers/synthesis.go` | chi v5 미들웨어 체인에 `costguard.IdentityMiddleware` → `costguard.CapCheckMiddleware` → `costguard.HaikuScreenMiddleware`를 `/deep` 라우트 앞에 wire |
| `cmd/usearch-api/main.go` | costguard 초기화: `costguard.New(cfg, obs, redisClient, asynqClient, pgPool)` + scheduled reconcile job 등록 |
| `internal/obs/metrics/metrics.go` | `registerCostGuard(r)` 헬퍼 호출 추가 |
| `internal/obs/obs.go` | `obs.DeepCapChecks`, `obs.DeepCostUSD`, `obs.DeepCacheHits` 등 신설 collector re-export |
| `internal/obs/metrics/metrics_test.go` | `TestNoUnboundedLabels` 화이트리스트에 `tenant`, `status`, `tier`, `state` 라벨 추가 |
| `deploy/litellm/config.yaml` | `litellm_settings.cache: true` + `cache_params.type: redis` 활성화. 모델별 TTL 설정 추가 |
| `.env.example` | `COSTGUARD_REDIS_FAILURE_MODE`, `COSTGUARD_DEFAULT_TENANT_ID` 등 신규 env-var 문서화 |

### 7.3 Existing — Unchanged

- `internal/llm/client.go` (LLM-001) — read-only consumer. cost middleware는
  이미 cost_usd, cache hit 헤더를 추출한다. costguard는 이 결과를 ledger에
  기록할 뿐이다.
- `internal/deepreport/` (DEEP-001) — read-only consumer.
- `internal/deepagent/` (DEEP-002) — read-only consumer.
- `services/researcher/` Python 사이드카 — 본 SPEC v1 범위 밖.

---

## 8. Open Questions

본 SPEC은 §1.1의 7개 pinned decision으로 대부분의 ambiguity를 해소했다.
다음 항목은 plan-auditor와의 협의 또는 첫 운영 데이터 기반 튜닝이 필요한
경계 사례다.

1. **Default tenant cap 정확한 dollar 수치**: research §1.2의 평균-경우
   $0.07 × 20 calls = $1.40, 최악-경우 $0.19 × 20 calls = $3.80이므로 $5/day
   tenant cap은 ~26회 최악-경우를 수용한다. 단일-팀 self-host 환경에서 $5는
   적절하나, 다중-팀 SaaS 환경에서는 부족할 수 있다. **권장**: V1 ship 시
   $5 유지, 첫 30일 운영 후 P95 daily usage 측정 결과로 재조정.
2. **User cap V1.1 활성화 정확한 시점**: AUTH-001(M6) ship 직후 즉시 활성화
   할지, 운영 데이터 수집 후 추가 1주 grace period를 둘지. **권장**: AUTH-001
   ship + 1 sprint 운영 안정화 후 활성화.
3. **Cap 차원 우선순위 표시(call vs $)**: 응답 본문의 `dimension` 필드가
   먼저 도달한 차원을 보여주지만, 두 차원 모두 동시에 도달하면 무엇을 우선
   표시할지. **권장**: alphabetical 정렬(`calls` 우선). 운영자 피드백으로
   재조정 가능.

위 3개는 plan-auditor가 SPEC을 PASS로 평가하기 위해 필수적인 결정이 아니다.
모두 first-30-day 운영 데이터로 튜닝 가능한 항목이다.

---

*End of SPEC-DEEP-004 v0.1.1 (draft).*
