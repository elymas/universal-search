# SPEC-DEEP-004 Deep Research

Generated: 2026-05-21T00:00:00Z
Author: manager-spec (Phase 0.5 — context-derived)
Consumed by: manager-spec (Phase 1B), plan-auditor (Phase 2.3)

---

## 0. Scope of This Research

본 research.md는 M5 milestone의 마지막 deliverable인 SPEC-DEEP-004
(`/deep` quota + cost guard)에 대한 코드베이스 분석 + 아키텍처 결정 기록이다.
roadmap.md §M5의 scope 한 줄("per-user per-day cap, Haiku pre-screen,
prompt-cache reuse")을 (a) 모든 LLM 호출 비용의 측정·추적, (b) Haiku 사전
스크리닝 게이트, (c) LiteLLM 프롬프트 캐시 hit-rate 향상, (d) 사용자/테넌트
단위 cap 강제의 4개 축으로 분해해 각 축의 코드 진입점, 환경 변수, 메트릭,
스토리지, 보안 영향을 명세화한다.

본 SPEC은 M5의 release gate다. SPEC-DEEP-001(STORM 장문 사이드카),
SPEC-DEEP-002(4-agent 파이프라인), SPEC-DEEP-003(tree exploration)이 모두
LLM 호출량을 비선형으로 증폭시키기 때문에 cost guard 없이 `/deep` 서피스를
GA할 수 없다. 따라서 본 SPEC은 위 3개 SPEC의 비용 발생 지점을 모두
관찰해야 하며, 단일한 ledger + cap-check 계층으로 통합한다.

---

## 1. Cost Surface Analysis

### 1.1 `/deep` 파이프라인의 LLM 호출 지점

DEEP-002의 4-agent sequential pipeline + DEEP-003의 tree exploration (breadth
≤ 4, depth ≤ 3)을 결합한 최악-경우 시나리오에서 단일 `/deep` 호출 1회가
유발하는 LLM 호출 횟수를 추산한다.

| 단계 | 모델 티어 | 호출 횟수(최악) | 평균 입력 토큰 | 평균 출력 토큰 |
|------|----------|----------------|----------------|----------------|
| Haiku pre-screen (본 SPEC 추가) | claude-haiku-4-5 | 1 | ~600 (query + few-shot) | ~50 (score JSON) |
| Researcher × tree node (DEEP-003 breadth=4, depth=3) | claude-haiku-4-5 | 12 | ~1,500 (query + fanout 컨텍스트) | ~600 (claim 추출) |
| Reviewer | claude-haiku-4-5 | 1 | ~3,000 (모든 researcher claims) | ~400 (critique notes) |
| Writer (초기 + 최대 2회 재시도) | claude-sonnet-4-6 | 1~3 | ~4,000 (evidence + critique) | ~1,500 (section text) |
| Verifier (Writer 호출당 1회) | claude-sonnet-4-6 | 1~3 | ~3,000 (Writer 출력 + docs) | ~200 (faithfulness result) |
| **합계 (최악)** | — | **17~19** | — | — |

### 1.2 단가 추정 (2026-05-21 시점 공개 가격)

LiteLLM 프록시를 통해 라우팅되는 4개 모델 별칭의 표준 단가:

| 모델 별칭 | 입력 단가 / 1M tok | 출력 단가 / 1M tok |
|-----------|---------------------|---------------------|
| claude-haiku-4-5-20251001 | $0.80 | $4.00 |
| claude-sonnet-4-6-20251022 | $3.00 | $15.00 |

최악-경우 1회 `/deep` 비용 (모두 cache miss 가정):

- Haiku 호출 14건(pre-screen 1 + Researcher 12 + Reviewer 1):
  - 입력 ~21,600 tok × $0.80/M = $0.01728
  - 출력 ~7,650 tok × $4.00/M = $0.0306
- Sonnet 호출 6건(Writer 3 + Verifier 3, 최대 재시도):
  - 입력 ~21,000 tok × $3.00/M = $0.063
  - 출력 ~5,100 tok × $15.00/M = $0.0765

→ **합계 ≈ $0.187 / 1회 (cache miss, 최악-경우)**

평균-경우(Writer 1회 + Verifier 1회 PASS, tree depth=2 breadth=3):
- Haiku 호출 9건: ~$0.025
- Sonnet 호출 2건: ~$0.044
- → **합계 ≈ $0.069 / 1회 (평균)**

### 1.3 product.md 성공 지표와의 일치

`.moai/project/product.md` 성공 지표는 "≤ $0.50 per /deep, dedup ≥ 30%
on repeated queries"이다. 위 1.2의 평균-경우($0.07)와 최악-경우($0.19) 모두
$0.50 cap 안에 들어온다. 따라서 본 SPEC의 cost guard는 (a) 정상-경우 거의
침범하지 않는 안전망이자, (b) 비정상 재시도 폭주·prompt injection 의심
시나리오·악성 사용자 시도에 대한 hard limit으로 동작한다.

### 1.4 비용 누락 위험 지점

본 SPEC이 모든 LLM 호출을 ledger에 기록하기 위해서는 다음 진입점을 모두
가로채야 한다.

- `internal/llm.Client.Complete()` — DEEP-001/002/003 + SYN-001의 단일 choke point
- `services/researcher` Python 사이드카의 LiteLLM SDK 호출 — Python-side
  faithfulness check + STORM 사이드카는 자체 Python SDK로 LiteLLM에 직접
  접근한다. 본 SPEC은 Go-side `llm.Client` 호출에만 ledger를 적용하며,
  Python-side 호출의 cost 회수는 **LiteLLM 프록시 자체의 spend logs**를
  관측하는 reconciliation job(별도 SPEC-DEEP-004 M5 후속 작업)에 위임한다.

→ **결정**: 본 SPEC v1은 Go-side `llm.Client`만 hook한다. Python-side 비용은
research §9 risk에 명시하고 SPEC-AUTH-003 audit log에서 cross-process
reconciliation을 후속 처리한다.

---

## 2. Identity Strategy Pre-AUTH-001

### 2.1 문제 정의

SPEC-AUTH-001(M6 — JWT 기반 사용자 인증)이 아직 구현되지 않은 상태에서
"per-user per-day cap"을 V1 deliverable로 요구한다. 본 SPEC은 AUTH-001에
종속되지 않는 forward-compatible identity 메커니즘을 도입해야 한다.

### 2.2 결정: HTTP 헤더 `X-User-Id` + `anonymous` fallback

cost_ledger의 `user_id` 컬럼은 opaque TEXT 컬럼으로 정의하며, 다음 두 가지
입력 소스를 순서대로 시도한다.

1. **V1 (M5)**: HTTP 요청 헤더 `X-User-Id`의 값. 상위 프록시(예: nginx
   `proxy_set_header X-User-Id ${request.user}`) 또는 CLI/MCP 클라이언트가
   직접 부착한다. 헤더 부재 시 `anonymous`라는 단일 공유 버킷으로 분류한다.
2. **V1.1 (M6 AUTH-001 도입 후)**: JWT 미들웨어가 `sub` claim을 추출해
   `r = r.WithContext(setUserID(r.Context(), claim.Sub))` 형태로 주입한다.
   `setUserID` 헬퍼는 컨텍스트에 ID를 박고, 동시에 downstream 응답에서
   `X-User-Id`를 reset해 fan-in 서비스가 일관된 값을 보도록 한다.

### 2.3 Forward-compatibility 보증

- `cost_ledger.user_id TEXT NOT NULL DEFAULT 'anonymous'` — schema는 변경 없음
- M6 AUTH-001 도입 시 추가 마이그레이션 불필요(opaque string 그대로)
- 본 SPEC의 cap 강제 로직은 `user_id` 값 자체에 의존하지 않으며,
  `(tenant_id, user_id)` tuple을 키로 사용한다.

### 2.4 Per-tenant vs per-user cap의 단계적 도입

- **V1 (M5 default)**: 단일-팀 self-host 환경 가정. tenant cap만 강제.
  `tenant_id`는 deploy 시점에 deep.yaml `default_tenant_id`로 설정되며
  요청 헤더 `X-Tenant-Id`로 override 가능.
- **V1.1 (M6 AUTH-001)**: per-user cap이 추가로 활성화. JWT의 `sub` claim
  + `team_id` claim으로 `(tenant_id, user_id)` 튜플 캡 강제.
- **Transition gate**: AUTH-001이 `auth-001-ga` 환경 변수로 활성화되면 본
  SPEC의 미들웨어는 user-level cap을 자동으로 enable한다.

### 2.5 결정 근거 (왜 X-User-Id 헤더인가)

- **Forward-compat**: 다른 옵션(예: TLS 클라이언트 인증서 hash, IP-based
  identity)은 AUTH-001 도입 시 모두 deprecate되어 cost_ledger의 옛 데이터를
  마이그레이션해야 한다. `X-User-Id`는 AUTH-001이 같은 헤더를 채우도록
  지정하면 transition cost가 0이다.
- **Self-hostable**: enterprise self-host 사용자가 자체 SSO/proxy 계층으로
  user identity를 부착하는 패턴을 가장 자주 본다(grafana, kibana 등 모두
  같은 패턴).
- **CLI 대응**: `usearch deep` CLI는 OS user 이름을 자동으로 X-User-Id로
  부착할 수 있다(향후 SPEC-CLI-002 M7에서 wiring).

---

## 3. Haiku Pre-Screen Pattern

### 3.1 동기

`/deep` 호출은 비싸고 느리다(NFR-DEEP2-001 = p95 ≤ 60s). 사용자가
경량 질의(예: "Python 3.13 release date")를 `/deep`으로 보낼 때 시스템은
17건 이상의 LLM 호출을 유발해 평균 $0.07 + 30~60초의 latency를 소비하지만,
같은 답을 `/basic` (단일 LLM 합성)으로 0.5초 + $0.002에 얻을 수 있다.
Haiku pre-screen은 이런 misuse를 진입 단계에서 차단한다.

### 3.2 메커니즘

`/deep` 요청이 인증 + cap-check를 통과한 직후, 본 SPEC이 추가하는 새 미들웨어
`costguard.HaikuScreen`이 다음 단계를 실행한다.

1. Haiku-tier 모델(`claude-haiku-4-5-20251001`)을 LiteLLM 프록시로 호출.
2. system prompt는 고정 few-shot("아래 질의가 multi-step research를 요구하는지
   0-10 점수로 평가하라; 5점 이하면 /basic으로 충분; <4점은 거부")
3. 사용자 query만 user message로 전달.
4. JSON 응답 `{"score": int, "rationale": str, "suggested_mode": "deep"|"basic"|"reject"}`
   을 파싱.

### 3.3 점수 임계값과 분기

| Haiku 점수 | 동작 | HTTP 응답 |
|------------|------|----------|
| ≥ 6 | `/deep` 파이프라인 진행 | 200 (SSE 시작) |
| 4 ~ 5 | `/basic` 모드 사용을 제안하고 거부 | 400 + body `{"error":"deep_not_warranted", "suggested_mode":"basic", "screen_score":N}` |
| < 4 | 거부 (저품질·노이즈·정답 불가 질의) | 400 + body `{"error":"query_rejected_by_screen", "rationale":"..."}` |

거부/제안 응답에서도 **Haiku 호출 비용은 ledger에 기록**된다(`screen_cost_usd`).
이는 호출자가 비용을 인지하도록 하고, 임계값 튜닝을 위한 raw data를 보존하기
위함이다.

### 3.4 Latency budget

Haiku pre-screen은 `/deep` 진입 경로의 latency를 직접적으로 증가시킨다.
다음 가드를 둔다.

- Per-call timeout: 200ms p95(NFR-DEEP4-001)
- LiteLLM 프록시 로컬 캐시(같은 query hash + same model)가 활성화되어
  cache hit 시 < 10ms에 응답.
- Circuit breaker: Haiku 호출이 연속 5회 timeout 또는 5xx → fail-open으로
  전환(스크리닝 건너뜀, 본 파이프라인 진행). `usearch_deep_haiku_screen_breaker_state`
  메트릭으로 가시화.

### 3.5 LLM-as-judge 안티 패턴 회피

Haiku pre-screen은 본질적으로 LLM-as-judge 패턴이지만, SPEC-DEEP-002의
Verifier가 명시적으로 거부한 multi-dimensional scoring과 다음과 같이 다르다.

- DEEP-002 Verifier는 **출력 품질** 평가 → 다차원 점수화는 inflation 위험
- DEEP-004 pre-screen은 **입력 분기** 결정 → 단일 정수 점수면 충분
- 임계값은 deep.yaml hot-reload로 운영 중 조정 가능

---

## 4. Prompt-Cache Reuse via LiteLLM

### 4.1 LiteLLM 내장 캐시 활용

SPEC-LLM-001 §2.2가 명시하듯 "Prompt caching orchestration"은 LiteLLM
proxy의 transparent passthrough에 위임된다. 본 SPEC은 LiteLLM Redis-backed
cache 기능을 활성화하고, Go-side에서는 다음 두 가지만 보장한다.

1. **cache_key 결정성**: 같은 (model, system_prompt, user_prompt) → 같은
   캐시 키. LiteLLM 기본 동작은 SHA256(model + messages_json)이며 본 SPEC은
   이 기본 동작을 사용한다.
2. **캐시 hit 기록**: LiteLLM 응답 헤더 `x-litellm-cache-hit: true/false`
   를 `internal/llm.Client`의 cost middleware에서 추출해 cost_ledger의
   `cache_hit` 컬럼에 기록.

### 4.2 캐시 TTL 정책

- Researcher 결과(fanout docs 기반 claim 추출): 24h TTL.
  - 24h 내 같은 query + 같은 fanout 결과 hash → cache hit.
  - fanout 결과 변동(새 doc 추가) 시 hash 변경 → 자동 miss.
- Reviewer/Writer/Verifier: per-intent class TTL.
  - SPEC-IR-001의 intent class hash + query hash 조합으로 캐시 키.
  - intent 변동 시 자동 invalidate.
- Haiku pre-screen: 1h TTL.
  - 짧은 TTL로 임계값 튜닝 반영 보장.

### 4.3 캐시 무효화 트리거

1. **Model version change**: LiteLLM `model_list`의 모델 별칭이 새 ID로
   교체되면 cache_key의 model 부분이 변경되어 자동 invalidate.
2. **System prompt change**: prompt 파일 변경 시 cache_key의 prompt hash가
   변경되어 자동 invalidate.
3. **Intent classification drift**: SPEC-IR-001의 intent hash가 변경되면
   다운스트림 prompt 컨텍스트가 달라져 자동 invalidate.

### 4.4 캐시 hit-rate 목표

`.moai/project/product.md`의 "dedup ≥ 30% on repeated queries" 지표를 캐시
hit-rate ≥ 30% (단일 팀 24h 윈도우)로 운영 SLO화한다. Prometheus 메트릭
`usearch_deep_cache_hits_total{tier}` / `usearch_deep_cache_attempts_total{tier}`
의 비율로 24h rolling 측정.

### 4.5 Anthropic prompt caching과의 차이

Anthropic은 자체 prompt cache(`cache_control: ephemeral`) 기능을 가진다.
이는 LiteLLM cache와 별개이며 본 SPEC은 후자만 사용한다. Anthropic-native
cache의 활용은 향후 SPEC-COST-OPT-001(M8)에서 다룬다.

---

## 5. Quota Storage Architecture

### 5.1 Dual-tier 저장소

| 계층 | 저장소 | TTL | 역할 |
|------|--------|-----|------|
| Hot | Redis | 24h sliding window | Fast cap-check, atomic INCR |
| Warm | Postgres `cost_ledger` table | 90일 hot retention | Durable audit, reconciliation source |
| Cold | (M8 SPEC-AUDIT-002로 위임) | 영구 archival | Compliance, ML-based threshold tuning |

### 5.2 Postgres `cost_ledger` 스키마

```sql
CREATE TABLE IF NOT EXISTS cost_ledger (
    id              BIGSERIAL    PRIMARY KEY,
    user_id         TEXT         NOT NULL DEFAULT 'anonymous',
    tenant_id       TEXT         NOT NULL DEFAULT 'default',
    request_id      TEXT         NOT NULL,
    deep_run_id     TEXT,                       -- nullable: pre-screen-only calls
    model           TEXT         NOT NULL,
    prompt_tokens   INT          NOT NULL DEFAULT 0,
    completion_tokens INT        NOT NULL DEFAULT 0,
    usd_cost        NUMERIC(10,6) NOT NULL DEFAULT 0,
    cache_hit       BOOLEAN      NOT NULL DEFAULT FALSE,
    intent_category TEXT,                       -- from SPEC-IR-001
    outcome         TEXT         NOT NULL,      -- {success, error, capped, degraded}
    ts              TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_cost_ledger_user_ts ON cost_ledger(user_id, ts DESC);
CREATE INDEX idx_cost_ledger_tenant_ts ON cost_ledger(tenant_id, ts DESC);
CREATE INDEX idx_cost_ledger_deep_run ON cost_ledger(deep_run_id) WHERE deep_run_id IS NOT NULL;
```

마이그레이션 파일 경로: `deploy/postgres/migrations/0002_create_cost_ledger.sql`
(현존하는 `0001_create_docs.sql` 다음 번호).

### 5.3 Redis 키 설계

```
costguard:window:tenant:{tenant_id}    → usd_cost (FLOAT, EXPIRE 24h sliding)
costguard:window:user:{user_id}        → usd_cost
costguard:calls:tenant:{tenant_id}     → call_count (INT)
costguard:calls:user:{user_id}         → call_count
```

각 키는 24h sliding window를 표현한다. Redis EXPIRE는 정확한 sliding이
어렵기 때문에 hourly bucket을 사용한다.

```
costguard:bucket:tenant:{tenant_id}:2026-05-21T13   → usd_cost
costguard:bucket:user:{user_id}:2026-05-21T13       → usd_cost
```

cap-check 시 최근 24개 시간 버킷을 SUM한다 (Lua script로 atomic 실행).

### 5.4 Write-behind 패턴

비용 발생 시점의 latency를 최소화하기 위해 Write-behind 패턴을 채택한다.

1. Cost guard 미들웨어가 LLM 응답 수신 직후 Redis INCR(원자적).
2. 동시에 Asynq queue에 `cost_ledger_write` 작업 enqueue.
3. Asynq worker가 batch(예: 100 row 또는 5초 timeout)로 Postgres flush.

이 패턴은 Redis 가용성에 의존하지만, Redis는 본 SPEC의 hot-path 종속성이며
이미 LiteLLM 캐시·Asynq queue·SPEC-IDX-001 캐시 등에서 광범위하게 사용 중이다.

### 5.5 Redis 단절 시 복구

Redis가 다운되면:

1. Cost guard는 fail-closed로 동작(429 응답)을 기본으로 한다.
2. 운영자가 deep.yaml의 `costguard.redis_failure_mode: fail-closed | fail-open`
   를 설정 가능(기본 fail-closed).
3. Redis 복구 시 Postgres에서 24h window를 재구성하는
   `costguard.RehydrateWindow(ctx)` 함수가 자동 실행된다.

### 5.6 Reconciliation Job

Asynq scheduled job `cost_ledger_reconcile` (5분 주기):

1. Postgres에서 최근 5분 row의 SUM(usd_cost)을 (tenant, user)별로 집계.
2. Redis 누적치와 비교.
3. drift > 0.1% (NFR-DEEP4-005) 시 알람 발행 + Redis 값을 Postgres truth로
   재설정.

---

## 6. Cap Enforcement Behavior

### 6.1 Default 한도값 (deep.yaml)

```yaml
costguard:
  enabled: true
  default_tenant_id: "default"
  redis_failure_mode: "fail-closed"
  tenant:
    max_calls_per_day: 20
    max_usd_per_day: 5.00
  user:
    enabled: false        # V1.1에서 AUTH-001 활성화 시 true
    max_calls_per_day: 10
    max_usd_per_day: 2.00
  haiku_screen:
    enabled: true
    model: "claude-haiku-4-5-20251001"
    threshold_proceed: 6
    threshold_suggest: 4
    timeout_ms: 200
    fail_open_on_timeout: true
  cache:
    hit_rate_target_pct: 30
    haiku_ttl_seconds: 3600
    researcher_ttl_seconds: 86400
```

### 6.2 Hard reject vs Degraded path

두 종류의 cap-exceed 응답을 지원한다.

- **Hard reject (default)**: HTTP 429 + `Retry-After` 헤더(다음 sliding
  window 만료까지의 초). 응답 본문은
  `{"error":"cap_exceeded", "dimension":"calls"|"usd", "remaining":{"calls":N,"usd":F}, "reset_at":"ISO-8601"}`.
- **Degraded path (opt-in via `X-Allow-Degrade: 1`)**: cap 초과 시 `/deep`
  대신 `/basic` 모드로 fallback하고 응답 헤더 `X-Deep-Degraded: cap-exceeded`
  를 부착. HTTP 200. 호출자는 응답에 한해 `/basic` 비용을 부담한다.

### 6.3 Cap 차원 결정 (call-count + $-amount, lower wins)

cap을 두 차원으로 정의한다:

- **call-count**: 24h sliding window 내 `/deep` 호출 횟수.
- **$-amount**: 24h sliding window 내 누적 USD 비용.

먼저 도달한 차원이 거부 기준이 된다. 응답 본문의 `dimension` 필드가 어떤
축이 도달했는지 알려준다.

### 6.4 Retroactive refund 미지원

cap이 초과된 호출은 ledger에 `outcome="capped"`로 기록되지만 비용은 적립
되지 않는다(이미 호출이 거부되어 LLM 비용이 발생하지 않았으므로). 단,
Haiku pre-screen 비용은 cap 초과 호출에도 기록된다(pre-screen이 cap-check
전에 실행되기 때문). 운영자의 수동 조정은 직접 Postgres `cost_ledger`에
보정 row를 추가하는 방식으로만 지원한다.

### 6.5 Cap 강제 시점 (request lifecycle 상의 위치)

chi v5 미들웨어 체인:

```
Request
  → request-id middleware (SPEC-OBS-001)
  → identity middleware (X-User-Id 추출)
  → costguard.CapCheck middleware  ← 본 SPEC
  → costguard.HaikuScreen middleware ← 본 SPEC
  → /deep handler (DEEP-002 or DEEP-003)
  → costguard.LedgerWrite middleware (response phase) ← 본 SPEC
Response
```

순서 근거:
- CapCheck를 먼저 두어 cap 초과 사용자의 Haiku 비용 발생 자체를 막는다.
- HaikuScreen은 cap을 통과한 호출에만 실행된다.
- LedgerWrite는 응답 단계에서 LLM 비용을 집계해 기록한다.

---

## 7. Pinned Decisions (No User Re-prompt)

다음 7개 결정은 본 research 단계에서 context-derived로 확정한다. 이후 SPEC
주 본문(§1.1)에서 같은 번호로 재참조된다.

| ID | Decision | Recommendation | Alternatives Considered |
|----|----------|----------------|------------------------|
| **D1** | Identity 소스 | HTTP 헤더 `X-User-Id` + `anonymous` fallback | TLS cert hash (마이그레이션 cost), IP-based (NAT 문제), session cookie (CLI에서 사용 불가) |
| **D2** | Storage 아키텍처 | Postgres durable + Redis hot cache write-behind | Redis-only (durability 부족), Postgres-only (latency), Kafka-based event log (운영 부담) |
| **D3** | Cap 차원 | call-count AND $-amount 모두 강제 (lower wins) | call-count only (Sonnet vs Haiku 비용 차이 무시), $-only (악성 단순 query 폭주 미차단) |
| **D4** | Haiku 점수 임계값 | ≥6 proceed / 4-5 suggest /basic / <4 reject | ≥7 (too strict, false-positive 증가), ≥5 (false-negative, cap 초과 위험) |
| **D5** | Cache 백엔드 | LiteLLM 내장 Redis-backed cache | 별도 application-level cache layer (DRY 위반), Anthropic-native cache only (다른 provider 미커버) |
| **D6** | Exceed 동작 | Hard 429 + `Retry-After`, degraded path는 opt-in `X-Allow-Degrade: 1` | Always degraded (silent quality 저하), Always 429 (UX 저하), Email notification (latency) |
| **D7** | Audit 보존 | 90일 hot retention in Postgres, archival은 M8에 위임 | 영구 hot (Postgres bloat), 30일 hot (compliance 부족), Time-series DB (의존성 추가) |

---

## 8. Observability Surface

### 8.1 Prometheus 메트릭

SPEC-OBS-001 NFR-OBS-002의 cardinality safety 규칙을 따른다. 모든 label
값은 bounded enumerable set이며 startup time에 pre-declare된다.

| Metric | Type | Labels | Cardinality |
|--------|------|--------|-------------|
| `usearch_deep_calls_total` | CounterVec | `{tenant, status}` | tenant ≤ 100 × status ∈ {allowed, capped, degraded, rejected_by_screen, suggested_basic, error} = 600 |
| `usearch_deep_cost_usd_total` | CounterVec | `{tenant, model}` | tenant ≤ 100 × model ≤ 15 = 1500 |
| `usearch_deep_cache_hits_total` | CounterVec | `{tier}` | tier ∈ {haiku_screen, researcher, reviewer, writer, verifier} = 5 |
| `usearch_deep_cache_attempts_total` | CounterVec | `{tier}` | 5 |
| `usearch_deep_haiku_screen_score` | Histogram | (no labels) | buckets [0,2,4,6,8,10] |
| `usearch_deep_haiku_screen_breaker_state` | GaugeVec | `{state}` | state ∈ {closed, half_open, open} = 3 |
| `usearch_deep_cap_check_duration_seconds` | Histogram | (no labels) | buckets [0.001, 0.005, 0.01, 0.05, 0.1] |
| `usearch_deep_ledger_write_duration_seconds` | Histogram | (no labels) | buckets [0.005, 0.01, 0.05, 0.1, 0.5, 1] |

**중요**: `tenant` label 값은 deploy 시점에 deep.yaml `costguard.allowed_tenants`
로 화이트리스트되며, 화이트리스트 외 값은 `tenant=unknown`으로 collapse되어
cardinality 폭증을 방지한다(NFR-DEEP4-007).

### 8.2 OTel span 속성

`/deep` 요청에 대응하는 OTel span(`deep.request`)에 다음 속성 추가:

- `deep.cap.tenant_remaining_usd` (float)
- `deep.cap.tenant_remaining_calls` (int)
- `deep.cap.user_remaining_usd` (float, V1.1)
- `deep.cap.user_remaining_calls` (int, V1.1)
- `deep.cache.hit_ratio` (float, 0.0-1.0)
- `deep.screen.score` (int 0-10)
- `deep.screen.outcome` (string in {proceed, suggest_basic, reject, fail_open})

label cardinality와 무관하므로 user_id, request_id 등의 high-cardinality
값도 OTel span 속성으로는 안전하게 부착할 수 있다(NFR-OBS-002).

### 8.3 Audit log (SPEC-AUTH-003 호환)

JSON line per cap event(allow/deny/degrade)를 stderr로 출력. SPEC-AUTH-003
(M6)이 동일 스키마로 통합된 audit log를 정의하므로 본 SPEC은 다음 필드를
미리 채택한다.

```json
{
  "timestamp": "2026-05-21T13:45:00.123Z",
  "event_type": "cap.evaluation",
  "request_id": "req_abc123",
  "tenant_id": "default",
  "user_id": "anonymous",
  "decision": "allow|deny|degrade",
  "dimension": "calls|usd|none",
  "remaining": { "calls": 12, "usd": 4.23 },
  "screen_score": 7,
  "cache_hit": false
}
```

---

## 9. Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|-----------|
| **R1** Concurrent /deep requests race past cap (TOCTOU) | High | High (cap이 사실상 무효) | Redis INCR + Lua script로 cap-check + counter-update를 원자적으로 실행. NFR-DEEP4-004 검증. |
| **R2** Ledger drift between Redis hot cache and Postgres | Medium | Medium (cap 강제 정확도 저하) | 5분 주기 Asynq reconciliation job. drift > 0.1% (NFR-DEEP4-005) 시 알람 + 자동 보정. |
| **R3** Haiku pre-screen latency stalls /deep entrypoint | Medium | High (UX 저하) | 200ms hard timeout(NFR-DEEP4-001) + circuit breaker fail-open. |
| **R4** Forward-compat break when AUTH-001 lands | Low | High (스키마 마이그레이션 필요) | user_id를 opaque TEXT 컬럼으로 유지. AUTH-001은 X-User-Id 헤더만 채우면 됨. M6 진입 시 schema review checkpoint. |
| **R5** Python-side LiteLLM 호출 비용이 ledger에서 누락 | High | Medium ($-cap 정확도 저하) | M5 v1: Go-side만 hook. Python-side는 LiteLLM 자체 spend logs로 후속 reconciliation(SPEC-AUTH-003 audit log). research §1.4에 명시. |
| **R6** Cache key collision across tenant/intent boundaries | Low | High (cross-tenant data leak) | LiteLLM cache key에 `tenant_id`와 `intent_category` 명시적 포함. NFR-DEEP4-007이 PII 미포함을 보장. |
| **R7** Haiku model API drift between versions (response schema 변경) | Medium | Medium (screen score 파싱 실패) | JSON schema 검증 + fallback to fail-open. `usearch_deep_haiku_screen_parse_errors_total` 메트릭으로 추적. |
| **R8** Redis 단절 시 cost guard 완전 정지 | Low | High (서비스 영향) | deep.yaml `redis_failure_mode: fail-closed` 기본; 운영자가 fail-open 명시적 선택 가능. Postgres에서 24h window 재구성. |
| **R9** Cap configuration drift between deploys | Medium | Low | deep.yaml hot-reload + Prometheus alerting on `costguard.config_version` mismatch. |
| **R10** Compute-vs-storage trade in 90일 retention | Low | Low | `cost_ledger` partition by month (M8 후속 task). 단일 테이블 90일은 ~수십 GB로 v1 acceptable. |

---

## 10. References

### 10.1 Internal SPEC documents
- `.moai/specs/SPEC-DEEP-001/spec.md` — STORM long-form 사이드카, 비용 발생 지점
- `.moai/specs/SPEC-DEEP-002/spec.md` — 4-agent pipeline, REQ-DEEP2-008(`usearch_deep_outcomes_total`)
- `.moai/specs/SPEC-DEEP-003/spec.md` — tree exploration, breadth/depth 한도
- `.moai/specs/SPEC-LLM-001/spec.md` — `llm.Client` 단일 choke point, cost middleware
- `.moai/specs/SPEC-OBS-001/spec.md` — NFR-OBS-002 cardinality safety
- `.moai/specs/SPEC-IR-001/spec.md` — intent classification, `/deep` 라우팅
- `.moai/specs/SPEC-CORE-001/spec.md` — `NormalizedDoc`, tenant scoping
- `.moai/specs/SPEC-AUTH-001/spec.md` — (M6) JWT 인증, X-User-Id transition gate
- `.moai/specs/SPEC-AUTH-003/spec.md` — (M6) audit log schema 호환

### 10.2 Implementation references (reuse map)

| New file | Closest analog | Reference |
|----------|---------------|-----------|
| `internal/deepagent/costguard/middleware.go` | `cmd/usearch-api/handlers/synthesis.go` | chi v5 middleware pattern |
| `internal/deepagent/costguard/ledger.go` | `internal/index/pg/` | pgx-based row insert with batching |
| `internal/deepagent/costguard/haiku_screen.go` | `internal/llm/client.go` | LLM call + JSON parse + circuit breaker |
| `internal/deepagent/costguard/cache_key.go` | `internal/llm/cost.go` middleware | response header extraction |
| `internal/deepagent/costguard/reconcile_job.go` | `internal/llm/router.go` ring buffer | 5-min scheduled job pattern |
| `deploy/postgres/migrations/0002_cost_ledger.sql` | `deploy/postgres/migrations/0001_create_docs.sql` | migration numbering convention |

### 10.3 External references
- LiteLLM caching docs: https://docs.litellm.ai/docs/caching
- Anthropic prompt caching (out of scope for v1): https://docs.anthropic.com/claude/docs/prompt-caching
- Redis INCR atomicity: https://redis.io/commands/incr
- Asynq scheduled tasks: https://github.com/hibiken/asynq/wiki/Periodic-Tasks

---

**End of Research Document**
