# SPEC-AUTH-001 Acceptance Scenarios

Generated: 2026-05-22
Format: Given / When / Then (Korean prose, English identifiers)

본 문서는 SPEC-AUTH-001의 acceptance scenarios 9건 + boundary edge case
2건을 정의한다. 각 시나리오는 spec.md §5에서 참조되며 plan-auditor가 SPEC
PASS 평가의 근거로 사용한다.

---

## §5.1 valid JWT bearer → 200, sub이 costguard.UserIDKey로 주입

**Coverage**: REQ-AUTH1-001, REQ-AUTH1-003, REQ-AUTH1-006

### Given

- `auth.yaml`:
  ```yaml
  auth:
    mode: "permissive"
    oidc:
      issuer: "http://127.0.0.1:9001"        # stub server
      audience: ["usearch-api"]
      allow_private_issuer: true               # dev/CI
    clock_skew_seconds: 30
    tenant:
      mode: "static"
      default_tenant_id: "default"
  ```
- in-process OIDC stub이 startup 시점에 discovery + JWKS endpoint를
  serve.
- stub이 다음 claim으로 token 발급: `{"sub": "alice@example.com", "iss":
  "http://127.0.0.1:9001", "aud": "usearch-api", "exp": now+5min,
  "iat": now, "jti": "tok-001"}`
- 호출자 query: 정상 `/query` 요청.

### When

호출자가 다음 요청을 전송한다:
```
POST /query
Authorization: Bearer <signed_token>
Content-Type: application/json
{"query": "test"}
```

### Then

- HTTP 응답 status는 **200 OK**.
- request handler가 보는 `r.Context()`에서:
  - `costguard.UserIDFromContext(ctx) == "alice@example.com"`
  - `costguard.TenantIDFromContext(ctx) == "default"` (static mode)
  - `auth.ClaimsFromContext(ctx).Subject == "alice@example.com"`
  - `auth.ClaimsFromContext(ctx).Raw["jti"] == "tok-001"`
- Prometheus 카운터:
  - `usearch_auth_attempts_total{outcome="success"}` 1 증가
- Histogram `usearch_auth_validation_duration_seconds`에 < 10ms 관측치
  추가됨 (NFR-AUTH1-001).
- OTel span(`http.server`)에 attribute `auth.outcome="success"` +
  `auth.subject_hash=sha256("alice@example.com")` 부착.
- `cost_ledger`에 후속 LLM 호출 row가 추가되며 `user_id =
  "alice@example.com"` (DEEP-004 forward-compat 검증).
- decision event log line(stderr JSON) emit:
  ```json
  {"timestamp":"...","event_type":"auth.validation","request_id":"...","subject_hash":"sha256:...","outcome":"success","issuer":"http://127.0.0.1:9001","audience":"usearch-api","token_age_seconds":0.1}
  ```

---

## §5.2 permissive mode + 헤더 부재 → anonymous fallback, 200

**Coverage**: REQ-AUTH1-004

### Given

- `auth.yaml` `auth.mode: "permissive"` (V1 default).
- 호출자가 Authorization 헤더를 부착하지 않음.

### When

호출자가 다음 요청을 전송한다:
```
POST /query
Content-Type: application/json
{"query": "test"}
```

### Then

- HTTP 응답 status는 **200 OK** (anonymous 허용).
- handler context에서:
  - `costguard.UserIDFromContext(ctx) == "anonymous"`
  - `auth.ClaimsFromContext(ctx) == nil` (claim map 없음)
- 카운터 `usearch_auth_attempts_total{outcome="anonymous_fallback"}` 1
  증가.
- JWT 검증 자체가 실행되지 않으므로 `usearch_auth_validation_duration_seconds`
  Histogram은 변경 없음.
- decision event log line은 `outcome="anonymous_fallback"`.
- 후속 cost_ledger row의 `user_id = "anonymous"` — DEEP-004 V1 경로와
  완전 동일 (forward-compat).

---

## §5.3 JWKS rotation: provider key 회전 시 unknown-kid forced fetch 후 통과

**Coverage**: REQ-AUTH1-002, NFR-AUTH1-005

### Given

- OIDC stub provider가 startup 시점에 keypair K1 (`kid: "k1"`)을 emit.
- 본 client가 startup 시 discovery + JWKS fetch → K1을 캐시.
- 30분 후 stub이 key를 K2 (`kid: "k2"`)로 회전 (K1은 deprecate되지만
  JWKS 응답에는 여전히 K1+K2 둘 다 포함, K2만 사용).
- stub이 K2로 서명된 새 token을 발급.

### When

호출자가 K2-서명 token으로 `POST /query` 호출.

### Then

- 본 client의 코드 path:
  1. RemoteKeySet 캐시에는 K1만 있음.
  2. JWT의 `kid: "k2"` header 발견 → 캐시 miss.
  3. coreos/go-oidc의 RemoteKeySet이 자동으로 jwks_uri를 1회 forced
     refetch (라이브러리 default behavior).
  4. 새 응답에 K2 포함 → 캐시 갱신.
  5. K2로 서명 검증 PASS.
- HTTP 응답 status는 **200 OK**.
- 카운터 `usearch_auth_jwks_refresh_total{outcome="unknown_kid_fetch"}`
  1 증가.
- Histogram `usearch_auth_jwks_refresh_duration_seconds{outcome=
  "unknown_kid_fetch"}`에 < 300ms (NFR-AUTH1-002) 관측치.

---

## §5.4 expired JWT → 401, reason="expired" (anonymous fallback 안 됨)

**Coverage**: REQ-AUTH1-005

### Given

- `auth.yaml` `auth.mode: "permissive"` (anonymous fallback 활성 상태).
- stub이 `exp: now - 1h` (만료된) token 발급.

### When

호출자가 expired token으로 `POST /query` 호출.

### Then

- HTTP 응답 status는 **401 Unauthorized** (anonymous fallback 적용 SHALL
  NOT — 검증 실패와 부재는 다르게 처리).
- 응답 본문:
  ```json
  {"error": "expired"}
  ```
- 카운터:
  - `usearch_auth_attempts_total{outcome="expired"}` 1 증가
  - `usearch_auth_failures_total{reason="expired"}` 1 증가
- OTel span attribute `auth.outcome="expired"`.
- request는 downstream(costguard.IdentityMiddleware 등)에 전혀 propagate
  되지 SHALL NOT 한다.
- decision event log line: `outcome="expired"`.

---

## §5.5 JWT의 sub이 costguard.IdentityMiddleware의 X-User-Id보다 우선

**Coverage**: REQ-AUTH1-003, REQ-AUTH1-006 (DEEP-004 forward-compat invariant)

### Given

- `auth.yaml` `auth.mode: "permissive"`.
- valid JWT의 `sub = "bob@example.com"`.
- 호출자가 동시에 `X-User-Id: malicious@attacker.com` 헤더 부착
  (악의적 헤더 spoofing 시도).

### When

호출자가 다음 요청 전송:
```
POST /deep
Authorization: Bearer <bob_token>
X-User-Id: malicious@attacker.com
{"query": "..."}
```

### Then

- JWT 미들웨어가 valid token 검증 통과 후 `context.WithValue(ctx,
  costguard.UserIDKey, "bob@example.com")` 호출.
- 그 다음 `costguard.IdentityMiddleware`가 실행되며:
  - context의 `UserIDKey` 값 `"bob@example.com"` 발견 → 그것 사용.
  - `X-User-Id: malicious@attacker.com` 헤더는 **무시**.
- handler에서 `costguard.UserIDFromContext(ctx) == "bob@example.com"`.
- `cost_ledger` row의 `user_id = "bob@example.com"` (NOT
  `"malicious@attacker.com"`).
- DEEP-004의 기존 테스트(`TestIdentityMiddlewareReadsXUserId`,
  `TestIdentityMiddlewareDefaultsAnonymous`)는 unchanged context (JWT
  미들웨어 없음) scenario이므로 모두 PASS.

### Why this matters

이것은 본 SPEC의 가장 critical한 invariant 중 하나다. JWT가 부착된 상태
에서도 X-User-Id 헤더 spoofing이 가능하다면 AUTH-001은 사실상 무효하다.
context-value-takes-precedence 패턴이 헤더 spoofing을 차단한다.

---

## §5.6 tenant.mode="claim" + JWT의 `org_id` claim → costguard.TenantIDKey

**Coverage**: REQ-AUTH1-007

### Given

- `auth.yaml`:
  ```yaml
  auth:
    tenant:
      mode: "claim"
      claim_path: "org_id"
      default_tenant_id: "default"
  ```
- stub이 다음 claim으로 token 발급: `{"sub": "carol", "org_id":
  "team-alpha", ...}`.

### When

호출자가 위 token으로 `POST /deep` 호출.

### Then

- handler context에서:
  - `costguard.UserIDFromContext(ctx) == "carol"`
  - `costguard.TenantIDFromContext(ctx) == "team-alpha"`
- 만약 token이 `org_id` claim을 포함하지 않으면 `TenantIDKey == "default"`
  (fallback).
- 후속 `cost_ledger` row의 `tenant_id = "team-alpha"`.
- DEEP-004의 cap 평가는 tenant `"team-alpha"` 버킷에서 수행.

---

## §5.7 strict mode + 헤더 부재 → 401; disabled mode → bypass

**Coverage**: REQ-AUTH1-004, REQ-AUTH1-008

### Given (case A — strict)

- `auth.yaml` `auth.mode: "strict"`.
- 호출자가 Authorization 헤더 부재.

### When (case A)

호출자가 `POST /query` 호출 (헤더 없이).

### Then (case A)

- HTTP 응답 status **401 Unauthorized**.
- 응답 본문: `{"error": "missing_token"}`.
- 카운터 `usearch_auth_failures_total{reason="missing_token"}` 1 증가.

### Given (case B — disabled)

- env `auth-001-ga=false` 또는 `auth.mode: "disabled"`.
- 호출자가 Authorization 헤더 부재.
- `X-User-Id: dave@example.com` 헤더 부착.

### When (case B)

호출자가 `POST /query` 호출.

### Then (case B)

- HTTP 응답 status **200 OK**.
- JWT 미들웨어는 검증을 전혀 수행하지 SHALL NOT 한다.
- `costguard.UserIDFromContext(ctx) == "dave@example.com"` (DEEP-004 V1
  경로 그대로 동작).
- 카운터 `usearch_auth_attempts_total` 변경 없음 (JWT 미들웨어가
  skip되므로).
- `usearch_auth_mode{mode="disabled"}` 게이지는 startup 시점에 emit.

### Given (case C — allowlisted endpoint, any mode)

- `auth.yaml` `auth.mode: "strict"`.
- 호출자가 Authorization 헤더 부재.

### When (case C)

호출자가 `GET /healthz` 호출.

### Then (case C)

- HTTP 응답 status **200 OK** (allowlist 우회).
- JWT 미들웨어는 strict mode 임에도 본 endpoint에서 skip.
- `usearch_auth_attempts_total` 변경 없음.

---

## §5.8 logout endpoint → 302 + end_session_endpoint URL; revocation enabled 시 Redis set

**Coverage**: REQ-AUTH1-009, REQ-AUTH1-010

### Given (case A — revocation disabled)

- `auth.yaml` `auth.revocation.enabled: false`.
- OIDC stub의 discovery 응답에 `end_session_endpoint:
  "http://127.0.0.1:9001/logout"` 포함.
- 호출자가 valid JWT `eve_token` (claim `jti: "tok-002"`)로 인증된 상태.

### When (case A)

호출자가 다음 요청 전송:
```
POST /v1/auth/logout
Authorization: Bearer eve_token
```

### Then (case A)

- HTTP 응답 status **302 Found**.
- `Location` 헤더:
  `http://127.0.0.1:9001/logout?id_token_hint=eve_token&post_logout_redirect_uri=https%3A%2F%2Fusearch.example.com%2F`
- Redis 호출 없음 (revocation disabled).
- 카운터 `usearch_auth_token_revoked_total{trigger="explicit_logout"}`
  1 증가.

### Given (case B — revocation enabled)

- `auth.yaml` `auth.revocation.enabled: true`.
- 나머지 동일.

### When (case B)

위와 동일 요청 전송.

### Then (case B)

- 302 응답 + Location 헤더 (case A와 동일).
- 추가로 Redis 호출:
  - `SADD auth:revoked:tok-002 1`
  - `EXPIRE auth:revoked:tok-002 <exp-now in seconds>`
- 직후 같은 token으로 다음 요청 시도:
  ```
  POST /query
  Authorization: Bearer eve_token
  ```
- 응답 status **401 Unauthorized**, body `{"error": "revoked"}`.
- 카운터 `usearch_auth_failures_total{reason="revoked"}` 1 증가.

### Given (case C — revocation enabled + provider has no end_session_endpoint)

- discovery 응답에 `end_session_endpoint` 없음 (hosted provider 일부 케이스).

### When (case C)

호출자가 `POST /v1/auth/logout` (valid JWT 부착).

### Then (case C)

- HTTP 응답 status **204 No Content** (서버측 revocation만 수행, 외부
  redirect 없음).
- Redis 호출은 revocation enabled 여부에 따라 동일.

---

## §5.9 SSRF block: http:// 또는 private IP issuer → startup fatal exit; /v1/auth/callback rate-limit 60/min

**Coverage**: REQ-AUTH1-011, REQ-AUTH1-012

### Given (case A — http:// scheme rejected)

- `auth.yaml` `auth.oidc.issuer: "http://example.com/realms/team"`
  (HTTPS 아님).

### When (case A)

프로세스 startup.

### Then (case A)

- startup-time validation 실패.
- fatal log: `"auth: issuer must use https scheme"`
- 프로세스 fatal exit (exit code 1).
- `auth.oidc.allow_private_issuer: true` 이어도 HTTPS 강제는 우회되지
  SHALL NOT 한다.

### Given (case B — private IP rejected without allow_private_issuer)

- `auth.yaml` `auth.oidc.issuer: "https://192.168.1.100/realms/team"`.
- `auth.oidc.allow_private_issuer: false` (default).
- DNS resolution 결과 `192.168.1.100` (RFC 1918 private).

### When (case B)

프로세스 startup.

### Then (case B)

- startup-time validation 실패.
- fatal log: `"auth: issuer host resolves to private IP range; set auth.oidc.allow_private_issuer=true for dev/CI"`
- 프로세스 fatal exit.

### Given (case C — allowlisted private IP for dev)

- 위와 동일하되 `auth.oidc.allow_private_issuer: true`.

### When (case C)

프로세스 startup.

### Then (case C)

- startup-time validation 통과 (HTTPS check OK, allowlist OK, private IP
  is bypassed by flag).
- 정상 startup.

### Given (case D — /v1/auth/callback rate limit)

- `auth.yaml` `auth.callback.rate_limit_per_minute: 60`.
- 동일 source IP에서 1분 내에 61회 호출 시도.

### When (case D)

61번째 호출.

### Then (case D)

- 처음 60회: HTTP **501 Not Implemented** (v1 stub 응답, REQ-AUTH1-012).
- 61번째: HTTP **429 Too Many Requests** + Retry-After 헤더.
- 카운터 (rate-limit middleware 자체의) increment.

---

## Edge Case 1 — clock skew 25s 미래 PASS, 35s FAIL

**Coverage**: REQ-AUTH1-003, NFR-AUTH1-004

### Given

- `auth.yaml` `auth.clock_skew_seconds: 30`.
- stub server clock과 본 client clock이 정확히 동기화된 상태.

### When

- (a) `iat = now + 25s`, `exp = now + 5m + 25s` token으로 호출.
- (b) `iat = now + 35s`, `exp = now + 5m + 35s` token으로 호출.

### Then

(a) — skew 25s 미래 (tolerance 내):
- 검증 PASS (nbf 평가 시 `now + clock_skew_seconds = now + 30s ≥ 25s`).
- HTTP 200.

(b) — skew 35s 미래 (tolerance 초과):
- 검증 FAIL with reason="invalid_nbf" (또는 `nbf` claim 부재 시
  "malformed").
- HTTP 401.
- 카운터 `usearch_auth_failures_total{reason="invalid_nbf"}` 1 증가.

### Why this matters

NTP drift가 < 30s인 환경 가정 하에서 false positive(정상 token 거부)와
false negative(미래 token 통과)의 boundary를 정확히 검증. clock_skew_seconds
config가 의미를 가지려면 boundary가 정확해야 함.

---

## Edge Case 2 — revocation enabled + Redis 단절 → fail-open default; fail-closed override 시 401

**Coverage**: REQ-AUTH1-010

### Given

- `auth.yaml` `auth.revocation.enabled: true`.
- valid JWT (revoked SET에 등록되지 않은 정상 token).
- Redis 인스턴스가 시점 T에 crash.

### When (case A — default fail-open)

- `auth.revocation.failure_mode: "fail-open"` (default).
- 호출자가 T 시점에 정상 token으로 `POST /query` 호출.

### Then (case A)

- 본 client의 코드 path:
  1. JWT 서명/iss/aud/exp 검증 통과.
  2. Redis `EXISTS auth:revoked:{jti}` 호출 → connection refused.
  3. fail-open default: revocation check skip.
  4. request 정상 통과.
- HTTP 응답 status **200 OK**.
- 카운터 `usearch_auth_attempts_total{outcome="success"}` 1 증가 (단,
  warning log: "revocation check unavailable, fail-open").

### When (case B — fail-closed override)

- `auth.revocation.failure_mode: "fail-closed"`.
- 동일 호출.

### Then (case B)

- HTTP 응답 status **401 Unauthorized**.
- 응답 본문: `{"error": "revocation_check_unavailable"}`.
- 카운터 `usearch_auth_failures_total{reason="revocation_check_unavailable"}`
  1 증가.

### Why this matters

revocation은 security-critical layer지만, 잘못된 failure_mode 설정이
정상 요청을 모두 차단하는 사고를 일으킬 수 있다. default fail-open은
"revocation은 best-effort, JWT TTL이 진짜 security boundary"라는 의미를
가진다. 짧은 JWT TTL(5-15분)이 mitigation strategy의 핵심.

---

## Acceptance Coverage Matrix

| Scenario | REQ-001 | REQ-002 | REQ-003 | REQ-004 | REQ-005 | REQ-006 | REQ-007 | REQ-008 | REQ-009 | REQ-010 | REQ-011 | REQ-012 |
|----------|---------|---------|---------|---------|---------|---------|---------|---------|---------|---------|---------|---------|
| §5.1 | ✓ |   | ✓ |   |   | ✓ |   |   |   |   |   |   |
| §5.2 |   |   |   | ✓ |   |   |   |   |   |   |   |   |
| §5.3 |   | ✓ |   |   |   |   |   |   |   |   |   |   |
| §5.4 |   |   |   |   | ✓ |   |   |   |   |   |   |   |
| §5.5 |   |   | ✓ |   |   | ✓ |   |   |   |   |   |   |
| §5.6 |   |   |   |   |   |   | ✓ |   |   |   |   |   |
| §5.7 |   |   |   | ✓ |   |   |   | ✓ |   |   |   |   |
| §5.8 |   |   |   |   |   |   |   |   | ✓ | ✓ |   |   |
| §5.9 |   |   |   |   |   |   |   |   |   |   | ✓ | ✓ |
| Edge1 |   |   | ✓ |   |   |   |   |   |   |   |   |   |
| Edge2 |   |   |   |   |   |   |   |   |   | ✓ |   |   |

NFR coverage:
- NFR-AUTH1-001 (latency p95 ≤ 5ms): §5.1 측정 + benchmark.
- NFR-AUTH1-002 (forced fetch p99 ≤ 300ms): §5.3 측정.
- NFR-AUTH1-003 (startup timeout): unit test direct.
- NFR-AUTH1-004 (clock skew): Edge1.
- NFR-AUTH1-005 (JWKS rotation resilience): §5.3.
- NFR-AUTH1-006 (no PII labels): metrics_test.go::TestNoUnboundedLabels 확장.
- NFR-AUTH1-007 (hot-reload): Phase F unit test.
- NFR-AUTH1-008 (metric naming): metrics_test.go 확장.
- NFR-AUTH1-009 (production warning): Phase F unit test.

---

*End of SPEC-AUTH-001 acceptance.md.*
