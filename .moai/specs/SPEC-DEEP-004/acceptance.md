# SPEC-DEEP-004 Acceptance Scenarios

Generated: 2026-05-21
Format: Given / When / Then (Korean prose, English identifiers)

본 문서는 SPEC-DEEP-004의 acceptance scenarios 7건 + boundary edge case 1건
을 정의한다. 각 시나리오는 spec.md §5에서 참조되며 plan-auditor가 SPEC
PASS 평가의 근거로 사용한다.

---

## §5.1 anonymous 호출자의 daily call 한도 도달 → 429 + Retry-After

**Coverage**: REQ-DEEP4-001, REQ-DEEP4-009, REQ-DEEP4-010

### Given

- deep.yaml `costguard.tenant.max_calls_per_day: 20`, `max_usd_per_day: 5.00`
- deep.yaml `costguard.user.enabled: false` (V1 default)
- Redis hot cache에 `costguard:bucket:tenant:default:{현재시}`의 calls 값이 20
- 호출자가 `X-User-Id` 헤더를 부착하지 않음 → user_id = "anonymous"
- 호출자가 `X-Tenant-Id` 헤더를 부착하지 않음 → tenant_id = "default"
- 호출 query: "최근 양자 컴퓨팅 발전사 deep research"

### When

호출자가 `POST /deep?mode=agents` 요청을 보낸다.

### Then

- HTTP 응답 status는 **429 Too Many Requests**.
- 응답 헤더에 `Retry-After: {다음 1시간 버킷 만료까지의 초}`가 부착된다.
- 응답 본문은 JSON 형식이며 다음과 같다:
  ```json
  {
    "error": "cap_exceeded",
    "dimension": "calls",
    "remaining": {"calls": 0, "usd": 4.23},
    "reset_at": "2026-05-21T14:00:00Z"
  }
  ```
- Prometheus 카운터 `usearch_deep_calls_total{tenant="default",status="capped"}`
  가 1 증가한다.
- Decision event log line이 stderr로 출력된다 (REQ-DEEP4-010의 stderr JSON
  line — NFR-DEEP4-006의 Postgres ledger row와는 별개 아티팩트):
  ```json
  {"timestamp":"...","event_type":"cap.evaluation","request_id":"...","tenant_id":"default","user_id":"anonymous","decision":"deny","dimension":"calls","remaining":{"calls":0,"usd":4.23}}
  ```
- Haiku pre-screen은 **호출되지 않는다** (REQ-DEEP4-010의 마지막 문장).
- `/deep` 본 파이프라인(DEEP-002 4-agent)은 **호출되지 않는다**.
- `cost_ledger`에 새 row가 **기록되지 않는다** (LLM이 실행되지 않았으므로).

---

## §5.2 X-User-Id 호출자가 $-cap 도달 → 429 + 잔여=0 응답

**Coverage**: REQ-DEEP4-001, REQ-DEEP4-009, REQ-DEEP4-010

### Given

- deep.yaml `costguard.user.enabled: true` (V1.1 시나리오 가정)
- deep.yaml `costguard.user.max_usd_per_day: 2.00`
- Redis hot cache에 `costguard:bucket:user:alice@example.com:{현재시}`의
  usd_cost 값이 2.00
- 호출자가 `X-User-Id: alice@example.com` 헤더 부착
- 호출 query: "AI safety paper survey deep research"

### When

호출자가 `POST /deep?mode=agents` 요청을 보낸다.

### Then

- HTTP 응답 status는 **429 Too Many Requests**.
- 응답 헤더에 `Retry-After: {sliding window 만료까지 초}`.
- 응답 본문:
  ```json
  {
    "error": "cap_exceeded",
    "dimension": "usd",
    "remaining": {"calls": 8, "usd": 0.00},
    "reset_at": "2026-05-22T13:00:00Z"
  }
  ```
- `dimension` 필드가 `"usd"`로 표시된다 (이 사용자는 $-cap에 먼저 도달).
- 카운터 `usearch_deep_calls_total{tenant="default",status="capped"}` 1 증가.
- Decision event log line에 user_id = "alice@example.com"으로 기록.
- Haiku pre-screen 미호출. `/deep` 파이프라인 미호출.

---

## §5.3 Haiku 점수 3 → 즉시 거부, 비용은 ledger 기록

**Coverage**: REQ-DEEP4-003, REQ-DEEP4-004, REQ-DEEP4-006

### Given

- 호출자의 24h cap 여유 충분 (calls 5/20, usd $0.50/$5.00).
- 호출자의 query: "오늘 날씨"
- Haiku pre-screen 모델이 다음 JSON을 반환:
  ```json
  {
    "score": 3,
    "rationale": "단순 사실 조회 질의로 multi-step research 불필요",
    "suggested_mode": "reject"
  }
  ```
- Haiku 호출의 비용 = $0.00012 (prompt 600 tok + completion 50 tok).

### When

호출자가 `POST /deep?mode=agents`로 위 query 요청.

### Then

- HTTP 응답 status는 **400 Bad Request**.
- 응답 본문:
  ```json
  {
    "error": "query_rejected_by_screen",
    "screen_score": 3,
    "rationale": "단순 사실 조회 질의로 multi-step research 불필요"
  }
  ```
- `cost_ledger`에 1개 row가 추가된다:
  - `model = "claude-haiku-4-5-20251001"`
  - `prompt_tokens = 600`
  - `completion_tokens = 50`
  - `usd_cost = 0.00012`
  - `outcome = "screen_only"`
  - `deep_run_id = NULL` (본 파이프라인 미실행)
- Histogram `usearch_deep_haiku_screen_score_bucket{le="4"}` 1 증가.
- 카운터 `usearch_deep_calls_total{tenant="default",status="rejected_by_screen"}`
  1 증가.
- `/deep` 본 파이프라인은 호출되지 않는다.

---

## §5.4 동일 query 24h 내 재호출 → cache hit, cache_hit=true 기록

**Coverage**: REQ-DEEP4-006, REQ-DEEP4-012, REQ-DEEP4-013

### Given

- 호출자 alice@example.com이 30분 전에 같은 query
  "quantum supremacy 2025 milestones deep research"를 호출하여 성공 응답을
  받음. 이때 Haiku pre-screen → Researcher × 9건 → Reviewer → Writer
  → Verifier 전체 LLM 호출이 LiteLLM Redis cache에 저장됨 (TTL 24h).
- 두 번째 호출은 같은 query, 같은 tenant_id="default", 같은 intent_category
  ("research_long").
- 호출자의 cap 여유 충분.

### When

호출자가 30분 후 같은 query로 `POST /deep?mode=agents` 재호출.

### Then

- Haiku pre-screen 응답에 `x-litellm-cache-hit: true` 헤더 포함, 본
  파이프라인 진행 (score ≥ 6).
- Researcher / Reviewer / Writer / Verifier 호출 모두 cache hit (LiteLLM
  Redis가 같은 cache_key로 즉시 응답 반환).
- HTTP 응답 status **200 OK** (정상 응답).
- 응답 본문은 첫 호출과 동일한 final report.
- `cost_ledger`에 LLM 호출 1건당 1개 row 추가 (총 약 12~13 row), 각각:
  - `cache_hit = TRUE`
  - `usd_cost`는 LiteLLM이 보고하는 캐시-감면된 값 (보통 0 또는 약간의
    소비량)
- 각 row의 `deep_run_id`는 동일한 UUID로 묶임.
- Prometheus 카운터:
  - `usearch_deep_cache_hits_total{tier="researcher"}` 9 증가
  - `usearch_deep_cache_hits_total{tier="haiku_screen"}` 1 증가
  - `usearch_deep_cache_attempts_total{tier="researcher"}` 9 증가
  - `usearch_deep_cache_attempts_total{tier="haiku_screen"}` 1 증가
- End-to-end latency: 첫 호출이 ~45초였다면 본 cache-hit 호출은 ~3초.

---

## §5.5 24h 윈도우에서 캐시 hit-rate ≥ 30% 측정

**Coverage**: REQ-DEEP4-013, NFR-DEEP4-003

### Given

- 단일 팀(tenant_id="default") 환경에서 지난 24시간 동안 100건의 `/deep`
  호출이 발생.
- 그 중 35건은 같은 사용자가 비슷한 query를 반복하여 LiteLLM cache가 hit됨.
- 65건은 첫 호출이거나 다른 query 패턴으로 cache miss.

### When

운영자가 Prometheus 쿼리:
```promql
sum(rate(usearch_deep_cache_hits_total[24h]))
/
sum(rate(usearch_deep_cache_attempts_total[24h]))
```
를 실행한다.

### Then

- 결과 값은 0.30 이상 (정확히는 35/100 = 0.35).
- 게이지 `usearch_deep_cache_hit_rate_below_target`는 `0`을 emit.
- Grafana 대시보드 (M8 후속)의 cache hit-rate 패널은 녹색 상태.
- 만약 hit-rate가 0.30 미만이었다면 게이지는 `1`을 emit하고 운영자 알람이
  트리거됨.

---

## §5.6 X-Allow-Degrade: 1 + cap 초과 → /basic fallback, HTTP 200

**Coverage**: REQ-DEEP4-011

### Given

- 호출자 alice@example.com의 user-level cap이 모두 도달 (calls 10/10).
- 호출자가 `X-Allow-Degrade: 1` 헤더 부착.
- 호출 query: "최신 ML 논문 요약 부탁해"

### When

호출자가 `POST /deep?mode=agents` 요청 (with `X-Allow-Degrade: 1`).

### Then

- HTTP 응답 status는 **200 OK** (429 아님).
- 응답 헤더에 `X-Deep-Degraded: cap-exceeded` 부착.
- `/basic` 모드(SPEC-SYN-001)로 처리되어 단일 LLM 합성 응답 반환.
- 응답 본문은 `/basic` 형식의 결과 (단순 합성 결과, sources 포함).
- `cost_ledger`에 `/basic` 호출의 LLM 비용이 `outcome="degraded"`로 기록.
- 이 호출의 비용은 cap 평가에 산입되지 **않음** (이미 cap 초과 상태의
  fallback이므로).
- 카운터 `usearch_deep_calls_total{tenant="default",status="degraded"}` 1 증가.
- Decision event log:
  ```json
  {"event_type":"cap.evaluation","decision":"degrade",...}
  ```

---

## §5.7 Redis 단절 시 Postgres에서 window 재구성하여 cap 정상 동작

**Coverage**: REQ-DEEP4-007, REQ-DEEP4-008, REQ-DEEP4-014

### Given

- deep.yaml `costguard.redis_failure_mode: "fail-closed"` (기본값).
- Redis 인스턴스가 시점 T에 crash, 30초 후 T+30에 복구.
- T 시점 직전까지 Postgres `cost_ledger`에 24h 누적 18 calls / $3.50 기록.
- 호출자가 T 시점에 `/deep` 호출을 시도.

### When

- (a) T 시점에 호출 시도 → 응답 확인
- (b) T+30 시점(Redis 복구 직후)에 동일 호출 → 응답 확인

### Then

(a) **T 시점 (Redis 단절)**:

- HTTP 응답 status **503 Service Unavailable**.
- 응답 본문:
  ```json
  {"error":"costguard_unavailable","detail":"redis unreachable"}
  ```
- `cost_ledger`에 `outcome="error"` row 1개 추가 (LLM 호출은 발생하지 않음).
- 카운터 `usearch_deep_calls_total{tenant="default",status="error"}` 1 증가.

(b) **T+30 시점 (Redis 복구 직후)**:

- Asynq job `costguard.RehydrateWindow`가 자동 실행됨.
- Postgres `cost_ledger`에서 직전 24h SUM(usd_cost) = $3.50, COUNT(*) = 18
  계산.
- Redis hot cache의 `costguard:bucket:tenant:default:*` 키 24개를 갱신.
- 호출자의 다음 `/deep` 호출은 정상 cap 평가를 통과(remaining: 2 calls,
  $1.50).
- 카운터 `usearch_deep_calls_total{tenant="default",status="allowed"}` 1 증가.

운영자가 `redis_failure_mode: "fail-open"`로 명시 override한 경우:

- (a) T 시점 호출은 cap 평가를 skip하고 본 파이프라인을 진행 (해당 호출의
  cost는 Postgres에 직접 기록되며 Redis 복구 후 RehydrateWindow가 누락분을
  반영).

---

## Edge Case — cap 잔여 1 → 그 호출 성공, 다음 호출 429

**Coverage**: REQ-DEEP4-009, REQ-DEEP4-010 (boundary condition)

### Given

- deep.yaml `costguard.tenant.max_calls_per_day: 20`.
- Redis hot cache에 `costguard:bucket:tenant:default:*` 합계 19 calls
  (정확히 1회 잔여).
- $-cap 여유 충분 (usd $4.80 / $5.00, 호출당 평균 $0.07이므로 1회 여유).
- 동일 호출자가 짧은 시간 간격으로 두 번 호출.

### When

- 호출 #1: T 시점에 `/deep` 호출.
- 호출 #2: T+1ms 시점에 같은 호출자가 `/deep` 호출.

### Then

호출 #1:

- Lua script 평가: calls = 19, +1 → 20 (정확히 cap 도달, 통과).
- HTTP 응답 200 OK. 정상 `/deep` 파이프라인 실행.
- `cost_ledger`에 row(들) 추가, `outcome="success"`.
- 카운터 `usearch_deep_calls_total{tenant="default",status="allowed"}` 1 증가.
- Redis bucket의 calls 값 20.

호출 #2:

- Lua script 평가: calls = 20, +1 → 21 (cap 초과).
- HTTP 응답 **429 Too Many Requests**.
- 응답 본문 `dimension: "calls"`, `remaining: {calls: 0, usd: ...}`.
- 호출 #1과 호출 #2의 atomic 평가가 race 없이 정확히 1건만 통과시킨다
  (NFR-DEEP4-004 검증).

### Why this matters

이 boundary는 단순한 ≤/< 차이가 cap 정확도에 미치는 영향을 검증한다.
spec의 "calls < max_calls_per_day → 통과" 의미가 정확히 구현되어야 하며,
"calls <= max_calls_per_day → 통과"로 구현되면 정확히 max_calls_per_day + 1
번 통과해 cap이 한 칸씩 누수된다.

---

## Acceptance Coverage Matrix

| Scenario | REQ-001 | REQ-002 | REQ-003 | REQ-004 | REQ-005 | REQ-006 | REQ-007 | REQ-008 | REQ-009 | REQ-010 | REQ-011 | REQ-012 | REQ-013 | REQ-014 |
|----------|---------|---------|---------|---------|---------|---------|---------|---------|---------|---------|---------|---------|---------|---------|
| §5.1 | ✓ | | | | | | | | ✓ | ✓ | | | | |
| §5.2 | ✓ | | | | | | | | ✓ | ✓ | | | | |
| §5.3 | | | ✓ | ✓ | | ✓ | | | | | | | | |
| §5.4 | | | | | | ✓ | | | | | | ✓ | ✓ | |
| §5.5 | | | | | | | | | | | | | ✓ | |
| §5.6 | | | | | | | | | | | ✓ | | | |
| §5.7 | | | | | | | ✓ | ✓ | | | | | | ✓ |
| Edge | | | | | | | | | ✓ | ✓ | | | | |

- REQ-DEEP4-002 (AUTH-001 forward-compat)은 schema-level 검증으로 cover
  되며 unit test (Phase A의 `TestIdentityForwardCompatWithAuth001`)에서
  직접 검증.
- REQ-DEEP4-005 (Haiku breaker)는 unit test (Phase C의
  `TestHaikuScreenBreakerOpensAfter5ConsecutiveFailures`)에서 cover.

---

*End of SPEC-DEEP-004 acceptance.md.*
