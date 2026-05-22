# SPEC-AUTH-001 Implementation Plan

Generated: 2026-05-22
Methodology: TDD (RED-GREEN-REFACTOR)
Coverage target: 85%
Harness: standard

---

## 1. Overview

본 plan.md는 SPEC-AUTH-001의 구현 단계별 task sequence를 정의한다. 12 EARS
REQs + 9 NFRs를 5개 phase로 분해하며, 각 phase는 RED → GREEN → REFACTOR
사이클을 따른다. plan-auditor 통과 + annotation cycle 완료 후 본 plan은
manager-tdd 에이전트에게 전달되어 phase-별로 진행한다.

본 SPEC의 sequencing principle: **DEEP-004 forward-compat invariant를 가장
먼저 RED 테스트로 고정한다**. JWT 미들웨어가 ship되어도 DEEP-004의 기존
identity 경로가 깨지지 않음이 가장 critical한 invariant이기 때문이다.

---

## 2. Phase Breakdown

### Phase A — In-process OIDC Stub + Discovery Module

목표: testcontainers 없이 진짜 OIDC 검증 path를 unit-test 가능한 기반을
먼저 구축한다. discovery + SSRF protection은 stub이 있어야 검증 가능.

**RED tests** (6):

1. `TestOIDCStubServesDiscovery` — stub의 `/.well-known/openid-configuration`
   이 RFC-compliant JSON 반환.
2. `TestOIDCStubServesJWKS` — stub의 JWKS endpoint가 valid RSA-2048 JWK
   반환.
3. `TestOIDCStubIssueTokenHelper` — `stub.IssueToken(claims, ttl)`이
   stub의 keypair로 서명된 valid JWT 반환.
4. `TestDiscoveryFetchesProviderMetadata` — config issuer → provider 객체
   생성 + endpoint metadata in-memory 캐시 (REQ-AUTH1-001).
5. `TestDiscoveryFailsOnIssuerMismatch` — discovery 응답의 `issuer`가
   config와 불일치하면 startup 실패 (REQ-AUTH1-001).
6. `TestDiscoveryFatalExitOnFailure` — discovery 호출 실패 시 fatal exit
   (REQ-AUTH1-001).

**GREEN tasks**:

- `internal/auth/testdata/oidc_stub/oidc_stub.go` 작성 — net/http
  ServeMux 기반 in-process server. 자체 RSA-2048 keypair 생성, `/jwks` +
  `/.well-known/openid-configuration` + `IssueToken` helper export.
- `internal/auth/discovery.go` 작성 — `oidc.NewProvider` thin wrapper +
  issuer match 검증 + fatal-exit on error.
- `internal/auth/config.go` 작성 (초기 skeleton; 전체는 Phase D에서 완성).
- `internal/auth/types.go`에 `ClaimsKey`, `Claims` struct, error 타입.

**REFACTOR**:

- stub의 keypair caching (test당 매번 generate하지 않도록 once.Do
  pattern).
- discovery timeout config 노출 (NFR-AUTH1-003).

---

### Phase B — SSRF Protection + Issuer Validation

목표: discovery URL의 SSRF risk를 startup-time에 차단.

**RED tests** (5):

7. `TestHttpSchemeIssuerRejected` — `http://` scheme issuer는 startup
   거부 (REQ-AUTH1-011).
8. `TestNonAllowlistedHostRejected` — allowlist 외 host는 startup 거부
   (REQ-AUTH1-011).
9. `TestPrivateIPIssuerRejected` — DNS resolution이 RFC 1918 / loopback /
   link-local / IPv6 ULA 포함 시 startup 거부 (REQ-AUTH1-011).
10. `TestPrivateIPIssuerAllowedWhenDevFlagSet` — `auth.oidc.
    allow_private_issuer: true` 시 private IP block 우회 (REQ-AUTH1-011).
11. `TestStartupValidationFatalExit` — validation 실패 시 fatal exit 동작
    (REQ-AUTH1-011).

**GREEN tasks**:

- `internal/auth/discovery.go`에 `validateIssuerURL` 함수 추가:
  - scheme `https://` enforce
  - host allowlist (deploy-time list) match
  - net.Resolver로 DNS resolution, 결과를 net.IP 또는 netip.Addr로
    분류해 private range 확인 (`netip.Addr.IsPrivate()`, `.IsLoopback()`,
    `.IsLinkLocalUnicast()`)
- config loader에서 startup-time invocation. hot-reload 시점에도 호출.

**REFACTOR**:

- private IP range 판별 helper(`internal/auth/private_ip.go`)로 분리
  + 단위 테스트 추가.

---

### Phase C — JWT Validation + Forward-Compat with DEEP-004

목표: 본 SPEC의 핵심 — JWT validation 미들웨어를 작성하고, DEEP-004의
기존 identity 경로가 본 변경으로 깨지지 않음을 RED 테스트로 먼저 고정한다.

**RED tests** (12):

12. `TestValidTokenInjectsSubIntoUserIDKey` — valid JWT bearer로 호출 시
    `costguard.UserIDKey` context value가 `claims.Sub`로 채워짐
    (REQ-AUTH1-003).
13. `TestValidTokenInjectsClaimsIntoClaimsKey` — full claim map이
    `auth.ClaimsKey`로 주입됨 (REQ-AUTH1-003).
14. `TestInvalidSignatureRejected` — 잘못된 서명 → 401, reason="invalid_signature"
    (REQ-AUTH1-003, 005).
15. `TestInvalidIssuerRejected` — `iss` 불일치 → 401, reason="invalid_iss"
    (REQ-AUTH1-003, 005).
16. `TestInvalidAudienceRejected` — `aud` whitelist 미일치 → 401,
    reason="invalid_aud" (REQ-AUTH1-003, 005).
17. `TestExpiredTokenRejected` — `exp` 만료 → 401, reason="expired"
    (REQ-AUTH1-003, 005).
18. `TestNbfNotYetValidRejected` — `nbf` 미래 → 401, reason="invalid_nbf"
    (REQ-AUTH1-003).
19. `TestMissingSubRejected` — empty `sub` → 401, reason="malformed"
    (REQ-AUTH1-003).
20. `TestClockSkewToleranceApplied` — skew 25s 미래 PASS, 35s FAIL
    (NFR-AUTH1-004).
21. **[CRITICAL]** `TestForwardCompatWithDeep004IdentityMiddleware` —
    JWT 미들웨어가 context에 sub 주입 후 costguard.IdentityMiddleware가
    그 값을 우선 사용. DEEP-004 cost_ledger 등 downstream에 영향 없음
    (REQ-AUTH1-003, 006).
22. `TestIdentityBridgeContextKeyTakesPrecedenceOverHeader` — context에
    UserIDKey가 있으면 X-User-Id 헤더 무시 (REQ-AUTH1-006).
23. `TestIdentityBridgeFallsBackToHeader` — context에 UserIDKey 없으면
    X-User-Id 헤더 사용 (DEEP-004 V1 경로 호환, REQ-AUTH1-006).

**GREEN tasks**:

- `internal/auth/validator.go` 작성 — `provider.Verifier(cfg).Verify(ctx,
  rawToken)` 호출 + claim 추출 + clock skew enforcement.
- `internal/auth/middleware.go` 작성 — `JWTValidationMiddleware`. valid
  통과 시 context에 ClaimsKey, UserIDKey, TenantIDKey 주입.
- `internal/deepagent/costguard/middleware.go::IdentityMiddleware` (line
  42-66)에 source-priority 분기 추가 (strictly additive patch).

**REFACTOR**:

- 검증 결과 → metric/span 매핑을 별도 helper로 추출
  (`emitAuthOutcome(span, metric, reason)`).
- failure reason enum을 types.go에 centralize.

---

### Phase D — Mode + Tenant + JWKS Rotation

목표: anonymous fallback의 3-mode, tenant 결정, JWKS rotation resilience.

**RED tests** (10):

24. `TestPermissiveModeMissingTokenInjectsAnonymous` — permissive +
    Authorization 헤더 부재 → anonymous, 200 (REQ-AUTH1-004).
25. `TestStrictModeMissingTokenReturns401` — strict + 헤더 부재 → 401
    (REQ-AUTH1-004).
26. `TestDisabledModeBypassesEntirely` — disabled mode → 검증 skip
    (REQ-AUTH1-004).
27. `TestAuthModeGaugeEmittedAtStartup` — `usearch_auth_mode{mode}` 게이지
    startup 시점 emit (REQ-AUTH1-004, NFR-AUTH1-009).
28. `TestTenantClaimModeExtractsFromClaim` — JWT의 `org_id` (또는 설정
    path)가 TenantIDKey로 주입 (REQ-AUTH1-007).
29. `TestTenantClaimModeFallsBackToDefault` — claim 부재 시 default tenant
    (REQ-AUTH1-007).
30. `TestTenantHeaderModeReadsHeader` — header mode → X-Tenant-Id 사용
    (REQ-AUTH1-007).
31. `TestTenantStaticModeUsesDefault` — static mode → default만 사용
    (REQ-AUTH1-007).
32. `TestJWKSScheduledRefreshIncrementsCounter` — background refresh가
    counter 증가 (REQ-AUTH1-002).
33. `TestUnknownKidTriggersForcedFetch` — unknown kid → forced fetch +
    counter 증가 (REQ-AUTH1-002).
    Additional: `TestJWKSRotationResilience` — stub의 key를 회전하면
    본 client가 새 키로 검증 통과 (NFR-AUTH1-005).

**GREEN tasks**:

- `internal/auth/config.go` 완성 — koanf 기반 layered config + fsnotify
  hot-reload.
- `internal/auth/tenant.go` 작성 — 3-mode 분기.
- `internal/auth/middleware.go`의 mode 분기 로직 완성.
- `internal/auth/metrics.go` 작성 — Prometheus collector 등록.
- `internal/obs/metrics/metrics.go`의 `registerAuth(r)` 호출 추가.

**REFACTOR**:

- mode resolution을 단일 함수로 추출 (`resolveAuthMode(cfg, env)`).
- tenant.mode 별 strategy를 interface로 분리.

---

### Phase E — Logout + Revocation + Callback + chi Wiring

목표: 보조 핸들러 ship + main.go wiring + end-to-end integration test.

**RED tests** (12):

34. `TestLogoutReturns302WithEndSessionURL` — `POST /v1/auth/logout` →
    302 + Location 헤더 (REQ-AUTH1-009).
35. `TestLogoutWithoutEndSessionEndpointReturns204` — provider가
    end_session_endpoint emit하지 않으면 204 (REQ-AUTH1-009).
36. `TestLogoutWithRevocationEnabledAddsToRedisSet` — revocation enabled
    시 Redis SADD + EXPIRE (REQ-AUTH1-009).
37. `TestLogoutWithRevocationDisabledSkipsRedis` — disabled 시 Redis 호출
    없음 (REQ-AUTH1-009).
38. `TestRevokedTokenReturns401` — revoked token → 401, reason="revoked"
    (REQ-AUTH1-010).
39. `TestNonRevokedTokenPasses` — non-revoked → 정상 통과 (REQ-AUTH1-010).
40. `TestRevocationCheckRedisFailureFailsOpenByDefault` — Redis 단절 +
    fail-open default → 통과 (REQ-AUTH1-010).
41. `TestRevocationCheckRedisFailureFailsClosedWhenConfigured` —
    fail-closed override → 401 (REQ-AUTH1-010).
42. `TestAuthCallbackEndpointRegistered` — `/v1/auth/callback` 등록됨
    (REQ-AUTH1-012).
43. `TestAuthCallbackRateLimit60PerMinute` — 60 req/min 초과 → 429
    (REQ-AUTH1-012).
44. `TestAuthCallbackReturns501InV1` — 정상 호출도 v1에서는 501
    (REQ-AUTH1-012).
45. `TestHealthzBypassesAuth` / `TestMetricsBypassesAuth` /
    `TestAdminPortHasNoAuthMiddleware` — endpoint allowlist (REQ-AUTH1-008).
    End-to-end integration: `TestEndToEndJWTToCostGuard` — chi 체인
    request-id → auth → costguard.Identity → costguard.CapCheck → handler.
    full path를 in-process OIDC stub + miniredis로 구동.

**GREEN tasks**:

- `internal/auth/logout.go`, `internal/auth/revocation.go`,
  `internal/auth/callback.go` 작성.
- `cmd/usearch-api/handlers/auth.go` 신설.
- `cmd/usearch-api/main.go` 수정 — chi router 본격 구성, auth.Init 호출,
  미들웨어 체인 wire. admin port는 별도 mux (auth 없이).
- in-memory rate-limit fallback 구현 (Redis 미사용 시).

**REFACTOR**:

- middleware chain 순서 검증 helper (lint check or test).
- panic 안전성 확보 (defer recover in JWTValidationMiddleware).

---

### Phase F — Production Hardening + Observability

목표: NFR-AUTH1-009의 production warning, OTel span 부착, hot-reload
검증. (Phase E 종료 시점에 충분히 진행되었을 수 있는 추가 hardening.)

**RED tests** (4):

46. `TestProductionModePermissiveWarnsAtStartup` — `USEARCH_ENV=production`
    + `auth.mode=permissive` → WARN log + gauge emit (NFR-AUTH1-009).
47. `TestOTelSpanAuthAttributes` — `auth.outcome`, `auth.subject_hash`,
    `auth.token_age_seconds`, `auth.jwks_cache_hit` span attribute 부착
    검증 (research §9.2).
48. `TestConfigHotReloadOnSIGHUP` — auth.yaml 수정 후 SIGHUP → mode 변경,
    issuer 변경은 warning log + 무시 (NFR-AUTH1-007).
49. `TestNoUnboundedLabelsAuthAdded` — `outcome`, `reason`, `mode`,
    `trigger` 라벨이 화이트리스트에 추가 (NFR-AUTH1-006, SPEC-OBS-001
    호환).

**GREEN tasks**:

- production env 감지 + warning log emit.
- OTel span attribute 부착 (`span.SetAttributes(...)`).
- fsnotify watcher의 `auth.mode` etc. live-update; issuer change는 ignored
  + warning.
- `internal/obs/metrics/metrics_test.go::TestNoUnboundedLabels` 화이트리스트
  업데이트.

**REFACTOR**:

- audit log line emitter를 별도 struct에 centralize (AUTH-003 호환).
- additive-only schema 규칙(research §9.3)을 코드 주석에 명시.

---

## 3. Test Catalog Summary

| Phase | Tests Added | REQs Covered | NFRs Covered |
|-------|-------------|--------------|--------------|
| A | 6 | 001 | 003 |
| B | 5 | 011 | — |
| C | 12 | 003, 005, 006 | 001, 004 |
| D | 10+ | 002, 004, 007 | 005, 007, 008 |
| E | 12+ | 008, 009, 010, 012 | 002 |
| F | 4 | (cross-cutting) | 006, 008, 009 |
| **Total** | **49+** | **12 / 12** | **9 / 9** |

REQ-AUTH1-007 (tenant mode)는 Phase D의 4개 tenant test로 cover. REQ-AUTH1-008
(endpoint allowlist)는 Phase E의 bypass test 3개로 cover.

---

## 4. Risk Mitigation Table

| Risk | Phase | Mitigation Strategy |
|------|-------|---------------------|
| **R1** SSRF on discovery URL | Phase B | startup-time validation + private IP block. `TestPrivateIPIssuerRejected` |
| **R2** JWKS rotation mid-deploy 401 spike | Phase D | coreos/go-oidc의 auto-refresh + unknown-kid forced fetch. `TestJWKSRotationResilience` |
| **R3** Clock skew false rejection | Phase C | `auth.clock_skew_seconds` config + `TestClockSkewToleranceApplied` |
| **R4** AUTH-001 ship이 DEEP-004 cost_ledger forward-compat 깨트림 | Phase C | RED 테스트로 가장 먼저 invariant 고정 — `TestForwardCompatWithDeep004IdentityMiddleware`. cost_ledger 마이그레이션 추가 금지. |
| **R5** /metrics, /healthz가 JWT 요구 | Phase E | hardcoded allowlist + `TestHealthzBypassesAuth` / `TestAdminPortHasNoAuthMiddleware` |
| **R6** Token replay after logout | Phase E | optional revocation list. default `enabled: false` — operator opt-in. `TestRevokedTokenReturns401` |
| **R7** Permissive mode left in production | Phase F | startup warning + gauge emit. `TestProductionModePermissiveWarnsAtStartup` |
| **R8** JWKS endpoint outage | Phase D | background refresh keeps cache warm. fail-closed default. `TestJWKSRefreshNetworkErrorCounterIncrements` |
| **R9** Discovery rate-limit by provider | Phase A | discovery는 startup 1회 + 의도된 hot-reload 시. negligible volume. |
| **R10** Custom claim conflict (multi-provider) | (out of scope V1) | V1은 single issuer/audience만. SPEC-AUTH-002가 claim 매핑 책임. |

---

## 5. MX Tag Plan

본 SPEC의 구현은 다음 @MX 태그를 생성한다.

### 5.1 @MX:ANCHOR (high fan_in, invariant contract)

- `internal/auth/middleware.go::JWTValidationMiddleware`
  — fan_in ≥ 3 (main.go, integration tests, future endpoint additions).
  모든 인증 필요 endpoint의 단일 진입점.
  `@MX:REASON`: 본 미들웨어는 cost_ledger.user_id의 값 source가 된다.
  변경 시 DEEP-004 forward-compat invariant를 깨뜨릴 수 있음.
- `internal/auth/validator.go::Verify`
  — fan_in ≥ 3 (middleware, callback handler, revocation check).
  `@MX:REASON`: provider.Verifier에 위임하지만 clock skew enforcement는
  본 SPEC이 직접 구현. NFR-AUTH1-004 invariant.
- `internal/deepagent/costguard/middleware.go::IdentityMiddleware` (modified)
  — 본 SPEC의 REQ-AUTH1-006 + DEEP-004 REQ-DEEP4-001의 joint invariant.
  `@MX:REASON`: 두 SPEC의 forward-compat contract가 본 함수의 source-priority
  분기에 의존.

### 5.2 @MX:WARN (danger zone, requires @MX:REASON)

- `internal/auth/discovery.go::validateIssuerURL`
  — `@MX:WARN`: SSRF protection은 misconfiguration 시 회피될 수 있음.
  `@MX:REASON`: private IP block의 DNS resolution이 변조될 수 있음 (DNS
  rebinding). production deploy 시 별도 layer-7 proxy egress 권장.
- `internal/auth/revocation.go::IsRevoked`
  — `@MX:WARN`: Redis 단절 시 fail-open default는 revocation을 사실상
  무효화. `@MX:REASON`: revocation은 옵션이며 critical workflow는 짧은
  JWT TTL로 mitigate해야 함.
- `internal/auth/middleware.go::JWTValidationMiddleware`의 disabled mode
  분기 — `@MX:WARN`: env `auth-001-ga=false` 시 모든 검증이 bypass된다.
  `@MX:REASON`: M5→M6 transition window를 위한 의도된 동작. production
  ship 시 `auth-001-ga=true`를 enforce.

### 5.3 @MX:NOTE (context & intent delivery)

- `internal/auth/middleware.go` 파일 상단에 JWT 미들웨어 체인 순서 +
  DEEP-004 forward-compat rationale 기록.
- `internal/auth/discovery.go`에 SSRF protection 분류 알고리즘 (RFC 1918
  / fc00::/7 / loopback / link-local)의 의미 기록.
- `internal/auth/types.go::ClaimsKey`에 "auth.ClaimsKey 는 본 SPEC 전용,
  user_id만 필요한 downstream은 `costguard.UserIDKey` 사용" 노트.

### 5.4 @MX:TODO (incomplete work — resolved in GREEN phase)

- RED phase에서 placeholder 함수에 `@MX:TODO`를 부착하고 GREEN phase
  종료 시 모두 제거.
- `cmd/usearch-api/handlers/auth.go::CallbackHandler`에 `@MX:TODO: v1은
  501. M7 SPEC-UI-001이 frontend OAuth2 flow ship 시 본체 구현`을 유지
  (v1 ship 후에도 의도적으로 유지).

---

## 6. File Touch Order (recommended TDD progression)

1. **Phase A start**: `internal/auth/testdata/oidc_stub/oidc_stub.go` →
   `internal/auth/types.go` → `internal/auth/discovery.go` → 6 tests.
2. **Phase B**: `internal/auth/discovery.go`의 `validateIssuerURL` 확장 +
   `internal/auth/private_ip.go` → 5 tests.
3. **Phase C**: `internal/auth/validator.go` → `internal/auth/middleware.go`
   초기 버전 → `internal/deepagent/costguard/middleware.go` patch → 12 tests
   (특히 `TestForwardCompatWithDeep004IdentityMiddleware`).
4. **Phase D**: `internal/auth/config.go` 완성 → `internal/auth/tenant.go`
   → `internal/auth/middleware.go`의 mode 분기 → `internal/auth/metrics.go`
   → `internal/obs/metrics/metrics.go` 수정 → 10+ tests.
5. **Phase E**: `internal/auth/logout.go` → `internal/auth/revocation.go` →
   `internal/auth/callback.go` → `cmd/usearch-api/handlers/auth.go` →
   `cmd/usearch-api/main.go` 수정 → 12+ tests (이 중 1개는 miniredis
   기반 integration).
6. **Phase F**: OTel span 부착, production warning log, hot-reload 검증,
   metrics 화이트리스트 업데이트 → 4 tests.

---

## 7. Coverage and Quality Gates

- Coverage 목표: 85% (per `quality.yaml`).
- 새 package `internal/auth/`만 측정.
- `internal/auth/testdata/oidc_stub/` 패키지는 측정 대상에서 제외 (test
  helper).
- TRUST 5 gates: 모든 phase 종료 시점에 `go vet` + `golangci-lint` +
  `go test -race` 통과.
- Cardinality test: `internal/obs/metrics/metrics_test.go::TestNoUnboundedLabels`
  화이트리스트에 신규 label 추가 후 통과.
- LSP gate: zero errors / zero type errors / zero lint errors.
- DEEP-004 regression gate: `internal/deepagent/costguard/...`의 모든
  기존 테스트가 본 SPEC ship 후에도 통과 — 별도 CI gate으로 enforce.

---

## 8. Pre-submission Self-Review

전체 changeset이 완성된 시점에 다음을 확인한다:

- middleware chain 순서가 §5.1 ANCHOR rationale을 반영하는가?
- `costguard.UserIDKey` context key가 본 SPEC 변경으로 schema 영향 없이
  source만 바뀌는가? (REQ-AUTH1-006 + REQ-DEEP4-002 joint invariant)
- SSRF protection이 production-deployable 수준인가? (private IP block +
  HTTPS + allowlist 모두 active)
- `auth.mode=permissive` default가 production WARN을 emit하는가?
- Prometheus 메트릭이 SPEC-OBS-001 명명 규칙(`usearch_auth_*`)을 따르는가?
- 모든 enum label이 NFR-AUTH1-006 화이트리스트에 추가되었는가?
- `/healthz`, `/metrics`, `/v1/auth/callback` (auth 미들웨어 우회) 동작
  검증 테스트가 있는가?
- 추가된 직접 의존성(`coreos/go-oidc/v3`)이 go.mod에 deterministic
  version으로 핀되어 있는가?

---

## 9. Implementation Sequencing Across Sessions

본 SPEC의 6개 phase는 sequential 의존성을 가진다(Phase C의 JWT 검증은
Phase A의 stub provider를 참조 등). 단일 manager-tdd 세션으로 완주가
어려운 경우 다음 세션 분할이 권장된다:

- **Session 1**: Phase A + B (stub provider + discovery + SSRF guard)
- **Session 2**: Phase C + D (JWT validation core + mode/tenant/JWKS rotation)
- **Session 3**: Phase E + F (logout/revocation/callback + chi wiring +
  production hardening)

각 세션 시작 시 `/clear` 후 본 plan.md만 재로드하여 컨텍스트를 보존한다.

---

## 10. Decisions Log

본 SPEC 구현 중 다음 결정을 기록한다 (research §8의 D1-D8 외 implementation-time
decisions):

| ID | Decision | Phase | Rationale |
|----|----------|-------|-----------|
| **DI-1** | direct require add: `coreos/go-oidc/v3 v3.18.0` | A | research §3.2 D1. golang-jwt/jwt/v5는 indirect → direct로 승격. |
| **DI-2** | OIDC stub 패키지 위치 `internal/auth/testdata/oidc_stub/` | A | testdata 하위는 go build 시 자동 제외. `internal/`은 외부 import 차단. |
| **DI-3** | DEEP-004 IdentityMiddleware patch는 strictly additive | C | REQ-AUTH1-006 invariant. DEEP-004 기존 테스트가 unchanged로 통과해야 함. |
| **DI-4** | rate-limit fallback 구현은 `golang.org/x/time/rate` token bucket | E | new direct dep 없이 stdlib + x/time 만으로 구현. |
| **DI-5** | `cmd/usearch-api/main.go`의 chi router 본격 구성 | E | 본 SPEC ship 전까지 main.go가 stub 상태(`mux := http.NewServeMux()` line 39). chi v5 wiring을 본 SPEC이 최초로 ship. |

---

*End of SPEC-AUTH-001 plan.*
