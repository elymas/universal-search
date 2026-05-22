---
id: SPEC-AUTH-001
version: 1.0.0
status: implemented
created: 2026-05-22
updated: 2026-05-22
author: limbowl
priority: P0
issue_number: 0
title: OIDC SSO with Keycloak/Authentik integration and JWT validation middleware
milestone: M6 — Team plane
owner: expert-security
methodology: tdd
coverage_target: 85
depends_on: [SPEC-BOOT-001, SPEC-OBS-001, SPEC-LLM-001]
blocks: [SPEC-AUTH-002, SPEC-AUTH-003, SPEC-IDX-004, SPEC-IDX-005]
---

# SPEC-AUTH-001: OIDC SSO with Keycloak/Authentik integration and JWT validation middleware

## HISTORY

- 2026-05-22 (v1.0.0 implemented, commit 35e49cd4745046a3d996afeb23ab32109efc8575): RED-GREEN-REFACTOR complete via TDD. internal/auth/ package with OIDC validator, JWT middleware, revocation checker, callback/logout flows. costguard joint REQ-AUTH1-006 + REQ-DEEP4-001 invariant implemented. Race-clean tests pass.

- 2026-05-22 (initial draft v0.1.0, limbowl via manager-spec):
  First EARS-formatted SPEC for the M6 release gate. AUTH-001은 M6 (Team
  plane)의 **첫 SPEC이자 direct blocker**다 — AUTH-002 (Casbin RBAC),
  AUTH-003 (audit log), IDX-004 (shared index multi-tenancy), IDX-005
  (team-shared answer reuse)가 모두 본 SPEC이 제공하는 신뢰 가능한
  user_id/tenant_id 클레임에 의존한다.

  본 SPEC은 또한 M5에서 ship된 SPEC-DEEP-004의 forward-compat 약속
  (REQ-DEEP4-002, §6.3)을 실행하는 SPEC이다. JWT 미들웨어가 같은 컨텍스트
  키 `costguard.UserIDKey`에 `sub` claim을 주입하여 `cost_ledger.user_id`
  TEXT 컬럼 스키마 변경 없이 user-level cap이 활성화된다.

  핵심 8개 결정사항은 research §8에서 context-derived로 확정되었고 §1.1에
  재명시한다.

  Pinned decisions:
  (D1) OIDC client library: `github.com/coreos/go-oidc/v3/oidc` primary.
       `MicahParks/keyfunc/v3` reserved fallback for advanced JWKS path.
       golang-jwt/jwt/v5는 이미 indirect 의존성으로 존재.
  (D2) Validation: `iss` exact match + `aud` whitelist match + `exp`/`nbf`
       with clock skew (default 30s) + 서명 검증. token introspection RPC
       사용 안 함.
  (D3) JWKS rotation: coreos/go-oidc의 `*RemoteKeySet` background refresh
       (cache TTL 4-6h) + unknown-kid on-demand re-fetch.
  (D4) Identity context key: DEEP-004 forward-compat 위해 `costguard.UserIDKey`
       재사용. 전체 claim map은 `auth.ClaimsKey`로 별도 주입.
  (D5) Anonymous fallback: 3-mode — `strict` / `permissive` / `disabled`.
       V1 default `permissive` (transition window). production 권장 전환:
       운영 안정 후 `strict`.
  (D6) Logout: OIDC RP-Initiated Logout + optional Redis revocation list
       (default disabled, opt-in only).
  (D7) Dev/CI stub: in-process Go OIDC stub (`internal/auth/testdata/oidc_stub/`).
       testcontainers Keycloak 사용 안 함.
  (D8) SSRF protection: discovery URL은 HTTPS-only + host allowlist +
       private IP 블록 (RFC 1918 / fc00::/7 / loopback) — `auth.oidc.
       allow_private_issuer: true`로만 우회 가능.

  M6 release gate의 첫 SPEC으로서 별도 GitHub Issue 트랙되지 않으며
  (`issue_number: 0`) plan-auditor 통과 후 status `draft → approved` 전이.

  Companion artifacts:
  - `.moai/specs/SPEC-AUTH-001/research.md` — Phase 0.5 research
    (~800 lines, 11 sections — cost surface, OIDC provider matrix, library
    selection, identity bridge, anonymous fallback, logout strategy, dev
    stub, 8 pinned decisions, observability surface, 10 risks)
  - `.moai/specs/SPEC-AUTH-001/plan.md` — TDD task sequence, 5-phase
    implementation, MX tag plan
  - `.moai/specs/SPEC-AUTH-001/acceptance.md` — Given/When/Then 시나리오
    (8 main + 2 boundary edges)
  - `.moai/specs/SPEC-AUTH-001/spec-compact.md` — compact view

  12 EARS REQs (10 × P0 + 2 × P1), 9 NFRs, 5 modules (Discovery / JWT
  Validation / Identity Bridge / Anonymous Fallback / Logout & Revocation).
  Methodology: TDD, coverage target 85%, harness: standard. Owner:
  expert-security.

---

## 1. Overview

본 SPEC은 M6 milestone(Team plane)의 첫 deliverable이자 release gate인
OIDC Single Sign-On 계층을 정의한다. SPEC-BOOT-001이 reserve한
`internal/auth/` stub (`internal/auth/auth.go` line 1-3 — single empty
package declaration)을 채우고, SPEC-DEEP-004가 약속한 forward-compat
contract(REQ-DEEP4-002, §6.3)를 실행한다.

본 SPEC은 다음 4축을 단일 chi v5 미들웨어 + 보조 핸들러 + config layer로
통합한다.

1. **OIDC discovery + JWKS rotation**: `coreos/go-oidc/v3`의 `oidc.
   NewProvider`로 발급자별 endpoint를 자동 추출. `*RemoteKeySet`의
   background refresh로 키 회전 무중단 처리.
2. **JWT validation middleware**: 서명 + `iss`/`aud`/`exp`/`nbf` claim
   검증을 chi 미들웨어로 노출. 검증 통과 시 `costguard.UserIDKey` +
   `auth.ClaimsKey` 컨텍스트 주입.
3. **Identity bridge**: DEEP-004의 기존 `costguard.IdentityMiddleware`를
   schema 변경 없이 확장 — context에 이미 `UserIDKey`가 있으면 그것을
   사용, 없으면 기존 X-User-Id 헤더 경로 fallback.
4. **Anonymous fallback + dev stub**: `auth.mode: permissive` (default)로
   JWT 미부착 시 anonymous로 통과시켜 단계적 rollout 가능. `internal/auth/
   testdata/oidc_stub/`로 testcontainers 없이 CI에서 real verification path
   검증.

### 1.1 Pinned Architectural Decisions

다음 8개 결정은 research §8에서 context-derived로 확정되었다. 본 SPEC은
이를 EARS 요구사항으로 번역할 뿐 재논의하지 않는다.

1. **OIDC client library**: `github.com/coreos/go-oidc/v3/oidc` (primary,
   2026-04 기준 v3.18.0). `MicahParks/keyfunc/v3`은 advanced path
   fallback으로 reserved. 추가 direct dependency 1개만 도입.
2. **Validation policy**: `iss` exact match + `aud` whitelist match +
   `exp`/`nbf` with configurable clock skew (default 30s) + signature
   verification. token introspection RPC는 V1 범위 밖 (latency penalty
   회피).
3. **JWKS rotation**: coreos/go-oidc의 `*RemoteKeySet`이 책임. background
   refresh + unknown-kid on-demand fetch. operator는 cache TTL 조정만
   가능.
4. **Identity context key**: `costguard.UserIDKey` (DEEP-004와 동일) 재사용.
   전체 claim map은 별도로 `auth.ClaimsKey`에 주입.
5. **Anonymous fallback**: `auth.mode` 3-mode (`strict` / `permissive` /
   `disabled`). V1 default `permissive`. production 권장 전환은 운영
   안정화 후 `strict`.
6. **Logout**: OIDC RP-Initiated Logout endpoint(`POST /v1/auth/logout`).
   Redis revocation list는 옵션 (`auth.revocation.enabled: false` default).
7. **Dev/CI stub**: in-process `internal/auth/testdata/oidc_stub/`.
   `httptest.NewServer(stub)`로 1줄 spawn. testcontainers Keycloak 미사용.
8. **SSRF protection**: discovery URL은 HTTPS scheme 강제 + host allowlist
   + private IP 블록. `auth.oidc.allow_private_issuer: true`인 dev/CI
   환경에서만 우회.

### 1.2 Motivation

M6은 "team plane"이다. M5까지 Universal Search는 single-tenant
self-host 가정이었다(`X-User-Id` 헤더 trust). M6은 **5명 이상의 팀**이
공유하는 환경에서 (a) 개별 사용자별 quota 강제, (b) RBAC, (c) 감사
가능한 audit trail을 제공해야 한다(`.moai/project/roadmap.md` line 155
M6 exit criterion). 본 SPEC은 이 세 가지 모두의 **기반** — 사용자 신원을
신뢰 가능한 형태로 제공한다.

또한 SPEC-DEEP-004는 M5 ship 시 "AUTH-001이 ship되면 schema 변경 없이
user-level cap이 활성화된다"는 contract를 frozen 상태로 명시했다
(REQ-DEEP4-002, line 181). 본 SPEC은 이 contract를 실행한다.

### 1.3 Forward-compat with SPEC-DEEP-004 (Why same context key)

DEEP-004 §6.3 (lines 326-332)은 다음을 frozen contract로 명시했다:

> REQ-DEEP4-002가 명시한 대로, SPEC-AUTH-001(M6)가 ship되어도 본 SPEC의
> schema·코드는 변경되지 SHALL NOT 한다. AUTH-001은 JWT 미들웨어가 같은
> context key(`costguard.UserIDKey`)에 user_id를 주입하도록 구현되며, 본
> SPEC의 `cost_ledger.user_id` 컬럼은 opaque TEXT를 유지하여 마이그레이션이
> 불필요하다.

본 SPEC은 이 contract를 다음 3가지로 충족한다:

- `internal/auth/middleware.go::JWTValidationMiddleware`는 검증 통과 시
  `context.WithValue(ctx, costguard.UserIDKey, claims.Sub)`를 호출한다
  (REQ-AUTH1-003).
- 기존 `costguard.IdentityMiddleware` (line 42-66)는 context에 이미
  `UserIDKey`가 있으면 그것을 우선 사용하도록 작은 patch가 추가된다
  (REQ-AUTH1-003 implementation).
- `cost_ledger.user_id` TEXT 컬럼은 schema 변경 없음 (REQ-AUTH1-003 +
  REQ-DEEP4-002 joint invariant).

---

## 2. EARS Requirements

### 2.1 Discovery Module

| ID | Pattern | Requirement | Priority | Acceptance |
|----|---------|-------------|----------|------------|
| **REQ-AUTH1-001** | Ubiquitous | OIDC discovery layer는 startup 시점에 `auth.oidc.issuer` config(예: `https://keycloak.example.com/realms/team`)로부터 `oidc.NewProvider(ctx, issuer)`를 호출 SHALL 하고 응답으로 얻은 provider의 endpoint metadata(`token_endpoint`, `jwks_uri`, `end_session_endpoint`, `userinfo_endpoint`)를 in-memory에 캐시 SHALL 한다. discovery 호출은 startup-blocking이며 실패 시(`auth.mode != "disabled"`인 한) 프로세스는 fatal exit SHALL 한다. discovery 응답의 `issuer` field는 config의 `issuer`와 정확히 일치 SHALL 하며 불일치 시 startup이 SHALL 실패한다. provider 객체는 이후 모든 JWT verification의 단일 source of truth로 SHALL 사용된다. (Acceptance §5.1) | P0 | `TestDiscoveryFetchesProviderMetadata`, `TestDiscoveryFailsOnIssuerMismatch`, `TestDiscoveryFatalExitOnFailure`, `TestProviderSingletonReused` |
| **REQ-AUTH1-002** | Ubiquitous | JWKS rotation은 coreos/go-oidc의 `*RemoteKeySet` (provider.Verifier 내부 wrapped)가 SHALL 책임진다. background refresh interval은 라이브러리 default(4-6h)를 사용하며 본 SPEC은 별도 cron이나 manual refresh 로직을 SHALL NOT 추가한다. WHEN incoming JWT의 `kid` header가 캐시된 키 set에 부재하면, RemoteKeySet은 즉시 jwks_uri를 1회 refetch SHALL 하고(라이브러리 default 동작), refetch 후에도 kid가 없으면 검증 실패 SHALL 한다. `usearch_auth_jwks_refresh_total{outcome}` 카운터는 (a) scheduled refresh, (b) unknown-kid forced fetch, (c) parse error, (d) network error 4 outcome에 대해 1씩 증가 SHALL 한다. (Acceptance §5.3) | P0 | `TestJWKSScheduledRefreshIncrementsCounter`, `TestUnknownKidTriggersForcedFetch`, `TestUnknownKidAfterForcedFetchFails`, `TestJWKSRefreshNetworkErrorCounterIncrements` |

### 2.2 JWT Validation Module

| ID | Pattern | Requirement | Priority | Acceptance |
|----|---------|-------------|----------|------------|
| **REQ-AUTH1-003** | Event-Driven | WHEN incoming HTTP request가 `Authorization: Bearer <jwt>` 헤더를 포함하면, `auth.JWTValidationMiddleware`는 provider.Verifier로 token을 검증 SHALL 한다. 검증은 다음 6개 조건을 모두 만족해야 한다: (a) 서명이 JWKS의 `kid`-matching public key로 valid, (b) `iss` claim이 config `auth.oidc.issuer`와 정확히 일치, (c) `aud` claim 값(또는 array) 중 하나가 config `auth.oidc.audience` 화이트리스트와 일치, (d) `exp` claim이 `now() - auth.clock_skew_seconds` 보다 미래, (e) `nbf` claim(있는 경우)이 `now() + auth.clock_skew_seconds` 보다 과거, (f) `sub` claim이 non-empty string. 검증 통과 시 미들웨어는 `context.WithValue(ctx, auth.ClaimsKey, parsedClaims)`와 `context.WithValue(ctx, costguard.UserIDKey, claims.Sub)`를 SHALL 호출하여 downstream `costguard.IdentityMiddleware`가 동일한 context key를 관찰하도록 보장한다. tenant 결정은 `auth.tenant.mode` config(`"claim"`/`"header"`/`"static"`)에 따라 SHALL 수행되며 결과는 `costguard.TenantIDKey`에 SHALL 주입된다. (Acceptance §5.1, §5.5) | P0 | `TestValidTokenInjectsSubIntoUserIDKey`, `TestValidTokenInjectsClaimsIntoClaimsKey`, `TestInvalidSignatureRejected`, `TestInvalidIssuerRejected`, `TestInvalidAudienceRejected`, `TestExpiredTokenRejected`, `TestNbfNotYetValidRejected`, `TestMissingSubRejected`, `TestClockSkewToleranceApplied`, `TestTenantClaimModeWritesContextKey`, `TestForwardCompatWithDeep004IdentityMiddleware` |
| **REQ-AUTH1-004** | Optional | WHERE `auth.mode == "permissive"` (V1 default) AND `Authorization` 헤더가 부재한 경우, 미들웨어는 검증을 SHALL 생략하고 `context.WithValue(ctx, costguard.UserIDKey, "anonymous")`를 SHALL 주입하여 downstream 미들웨어가 DEEP-004 V1 경로(anonymous 공유 버킷)와 동일하게 동작하도록 한다. WHERE `auth.mode == "strict"` AND 헤더 부재면, 미들웨어는 HTTP 401 + body `{"error":"missing_token"}`을 SHALL 반환한다. WHERE `auth.mode == "disabled"` (i.e., env `auth-001-ga` falsy)면, 미들웨어는 어떤 검증도 수행하지 SHALL NOT 하며 request를 그대로 통과시킨다 (`costguard.UserIDKey`는 기존 X-User-Id 경로로 결정). `auth.mode` 값은 startup 시점에 `usearch_auth_mode{mode}` 게이지로 emit SHALL 된다. (Acceptance §5.2, §5.7) | P1 | `TestPermissiveModeMissingTokenInjectsAnonymous`, `TestStrictModeMissingTokenReturns401`, `TestDisabledModeBypassesEntirely`, `TestAuthModeGaugeEmittedAtStartup` |
| **REQ-AUTH1-005** | Unwanted | IF `auth.mode == "permissive"` OR `"strict"` AND `Authorization` 헤더는 있으나 JWT가 검증 실패(`exp` 만료, 서명 불일치, `iss`/`aud` 불일치, malformed)면, 미들웨어는 HTTP 401 + body `{"error":"<reason>"}`을 SHALL 반환한다. reason enum: `expired`, `invalid_signature`, `invalid_aud`, `invalid_iss`, `invalid_nbf`, `malformed`, `revoked`. `usearch_auth_failures_total{reason}` 카운터를 1 증가 SHALL 시키고, OTel span attribute `auth.outcome`을 동일 reason으로 SHALL 설정한다. **검증 실패 시 anonymous fallback으로 통과시키지 SHALL NOT 한다** — 즉 부재(REQ-AUTH1-004)와 검증실패는 다르게 처리된다. (Acceptance §5.4) | P0 | `TestExpiredTokenReturns401NotAnonymous`, `TestInvalidSignatureReturns401`, `TestMalformedTokenReturns401`, `TestFailureReasonCounterIncremented`, `TestFailureSetsOTelSpanAttribute` |

### 2.3 Identity Bridge Module

| ID | Pattern | Requirement | Priority | Acceptance |
|----|---------|-------------|----------|------------|
| **REQ-AUTH1-006** | Ubiquitous | 기존 `internal/deepagent/costguard/middleware.go::IdentityMiddleware` (line 42-66)는 source-priority 분기를 추가하도록 SHALL 확장된다: (a) context에 이미 `costguard.UserIDKey` 값이 set되어 있고 non-empty string이면 그것을 사용한다(JWT 미들웨어 주입 경로), (b) 그렇지 않으면 `X-User-Id` 헤더를 읽는다(DEEP-004 V1 경로), (c) 그것도 없으면 `"anonymous"`로 fallback한다. 본 확장은 `cost_ledger.user_id` 컬럼 schema를 SHALL NOT 변경하며 DEEP-004의 모든 기존 테스트(`TestIdentityMiddlewareReadsXUserId`, `TestIdentityMiddlewareDefaultsAnonymous`)는 SHALL 통과한다 — 즉 본 확장은 strictly additive다. (Acceptance §5.5) | P0 | `TestIdentityBridgeContextKeyTakesPrecedenceOverHeader`, `TestIdentityBridgeFallsBackToHeader`, `TestIdentityBridgeFallsBackToAnonymous`, `TestDeep004ExistingTestsPassUnchanged` |
| **REQ-AUTH1-007** | Optional | WHERE `auth.tenant.mode == "claim"`인 경우, JWT 미들웨어는 deploy 시점에 설정된 `auth.tenant.claim_path`(예: `"tenant_id"`, `"org_id"`, `"team_id"` 또는 `"realm_access.tenant"` dot-path)에 해당하는 claim 값을 추출하여 `costguard.TenantIDKey`에 SHALL 주입한다. 추출 실패 또는 빈 값인 경우 `auth.tenant.default_tenant_id`(예: `"default"`)를 SHALL 사용한다. WHERE `auth.tenant.mode == "header"`면 `X-Tenant-Id` 헤더 값을 사용한다(DEEP-004 V1 경로 호환). WHERE `auth.tenant.mode == "static"`면 `auth.tenant.default_tenant_id`만 사용한다. (Acceptance §5.6) | P1 | `TestTenantClaimModeExtractsFromClaim`, `TestTenantClaimModeFallsBackToDefault`, `TestTenantHeaderModeReadsHeader`, `TestTenantStaticModeUsesDefault` |

### 2.4 Anonymous Fallback / Mode Module

| ID | Pattern | Requirement | Priority | Acceptance |
|----|---------|-------------|----------|------------|
| **REQ-AUTH1-008** | Ubiquitous | 본 SPEC의 미들웨어 체인은 다음 endpoint를 SHALL 우회한다(`auth.mode`와 무관, 하드코딩된 allowlist): `/healthz`, `/metrics`, `/v1/auth/callback`, `/v1/auth/login`, `/v1/auth/logout` (logout은 GET endpoint logout-initiation의 경우만, POST logout-revoke는 인증 필요). admin port(`USEARCH_ADMIN_PORT`)에 binding된 mux는 본 미들웨어 체인이 attach되지 SHALL NOT 한다. allowlist는 startup 시점에 log에 SHALL 출력되어 운영자가 검증할 수 있도록 한다. (Acceptance §5.7) | P0 | `TestHealthzBypassesAuth`, `TestMetricsBypassesAuth`, `TestAuthCallbackBypassesAuth`, `TestAdminPortHasNoAuthMiddleware`, `TestAllowlistLoggedAtStartup` |

### 2.5 Logout & Revocation Module

| ID | Pattern | Requirement | Priority | Acceptance |
|----|---------|-------------|----------|------------|
| **REQ-AUTH1-009** | Event-Driven | WHEN client가 `POST /v1/auth/logout`를 valid JWT와 함께 호출하면, 핸들러는 (a) HTTP 302 응답 + Location 헤더 = `{end_session_endpoint}?id_token_hint={token}&post_logout_redirect_uri={config_value}` (provider discovery에서 추출된 `end_session_endpoint`), AND (b) WHERE `auth.revocation.enabled == true`인 경우 JWT의 `jti` claim(있으면)을 Redis key `auth:revoked:{jti}` SET에 EXPIRE `exp - now` 초로 SHALL 추가한다. `usearch_auth_token_revoked_total{trigger="explicit_logout"}` 카운터 1 증가. WHERE `auth.revocation.enabled == false`인 경우 Redis 호출은 SHALL 생략된다. (Acceptance §5.8) | P0 | `TestLogoutReturns302WithEndSessionURL`, `TestLogoutWithoutEndSessionEndpointReturns204`, `TestLogoutWithRevocationEnabledAddsToRedisSet`, `TestLogoutWithRevocationDisabledSkipsRedis`, `TestLogoutCounterIncrements` |
| **REQ-AUTH1-010** | Optional | WHERE `auth.revocation.enabled == true` AND JWT validation이 성공한 직후, 미들웨어는 token의 `jti` claim(있는 경우)에 대해 Redis `EXISTS auth:revoked:{jti}` 1회를 SHALL 호출한다. 결과가 `1`이면 token은 revoked로 간주되어 HTTP 401 + body `{"error":"revoked"}`로 거부 SHALL 된다. Redis 호출 실패 시(connection refused, timeout)는 `auth.revocation.failure_mode`(default `"fail-open"`)에 따라 SHALL 처리한다: `"fail-open"` → revocation check skip 후 정상 통과; `"fail-closed"` → 401 + `{"error":"revocation_check_unavailable"}`. (Acceptance §5.8) | P1 | `TestRevokedTokenReturns401`, `TestNonRevokedTokenPasses`, `TestRevocationCheckRedisFailureFailsOpenByDefault`, `TestRevocationCheckRedisFailureFailsClosedWhenConfigured` |

### 2.6 SSRF & Security Module

| ID | Pattern | Requirement | Priority | Acceptance |
|----|---------|-------------|----------|------------|
| **REQ-AUTH1-011** | Unwanted | IF `auth.oidc.issuer` 값이 (a) `https://` scheme이 아니거나, (b) `auth.oidc.allowed_issuer_hosts` 화이트리스트(deploy 시점 list)에 포함되지 않는 host를 가리키거나, (c) DNS resolution 결과 private IP range(RFC 1918 IPv4 `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`; loopback `127.0.0.0/8` / `::1`; link-local `169.254.0.0/16` / `fe80::/10`; IPv6 ULA `fc00::/7`)를 포함하면, startup-time validation은 SHALL 실패하고 프로세스는 fatal exit SHALL 한다. WHERE `auth.oidc.allow_private_issuer == true`(dev/CI 환경에서만)인 경우, private IP 블록은 SHALL 우회된다(allowlist + HTTPS는 여전히 enforce). 본 검증은 startup-time + config hot-reload 시점에 SHALL 수행된다. (Acceptance §5.9) | P0 | `TestHttpSchemeIssuerRejected`, `TestNonAllowlistedHostRejected`, `TestPrivateIPIssuerRejected`, `TestPrivateIPIssuerAllowedWhenDevFlagSet`, `TestStartupValidationFatalExit` |
| **REQ-AUTH1-012** | Ubiquitous | `POST /v1/auth/callback` endpoint(OIDC authorization code flow의 redirect URI)는 본 SPEC v1에서 SHALL 등록되되 rate-limit 미들웨어(`auth.callback.rate_limit_per_minute`, default 60 per source IP)로 SHALL 보호된다. rate limit은 Redis sliding window로 구현되며 Redis 미사용 시 in-memory bucket fallback. rate limit 초과 시 HTTP 429 + Retry-After 응답. v1은 callback의 실제 token exchange 로직을 SHALL 구현하지 않는다(M7 SPEC-UI-001이 frontend authorization code flow를 ship할 때 핸들러 본체가 채워진다); v1은 endpoint 등록 + rate limit + 501 Not Implemented 응답만 ship한다. (Acceptance §5.9 boundary) | P0 | `TestAuthCallbackEndpointRegistered`, `TestAuthCallbackRateLimit60PerMinute`, `TestAuthCallbackReturns501InV1`, `TestRateLimitRedisFailureFallsBackToInMemory` |

---

## 3. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| **NFR-AUTH1-001** | JWT validation latency (cache hit) | JWT validation 호출의 wall-clock latency는 JWKS cache hit 경로에서 p95 ≤ 5 ms, p99 ≤ 10 ms SHALL 한다. RSA-2048 서명 검증 비용이 dominate함. 측정: `usearch_auth_validation_duration_seconds` Histogram의 p95 / p99 quantile. |
| **NFR-AUTH1-002** | JWKS refresh latency (forced fetch on unknown kid) | unknown-kid에 의한 강제 JWKS refetch는 wall-clock p99 ≤ 300 ms SHALL 한다. 측정: `usearch_auth_jwks_refresh_duration_seconds{outcome="unknown_kid_fetch"}` Histogram의 p99. |
| **NFR-AUTH1-003** | Startup validation fatal exit budget | OIDC discovery + issuer validation 실패 시 startup-blocking 호출의 timeout은 30s SHALL 한다 (`auth.discovery.timeout_seconds` config). timeout 초과 시 프로세스 fatal exit. dev/CI에서 OIDC stub은 < 100ms 응답. |
| **NFR-AUTH1-004** | Clock skew tolerance | `exp`/`nbf` 평가는 `auth.clock_skew_seconds`(default 30s) 범위 내에서 SHALL 통과한다. NTP drift가 < 30s인 환경 가정. 테스트: `TestClockSkewToleranceApplied`가 skew 25s 미래 토큰을 PASS, 35s 미래 토큰을 FAIL로 검증. |
| **NFR-AUTH1-005** | JWKS rotation resilience | provider가 key 회전 후 30분 이내(라이브러리 default cache TTL의 절반)에 본 SPEC의 client는 새 키로 token을 검증 SHALL 한다 — 즉 unknown-kid 강제 fetch가 정상 동작. integration test `TestJWKSRotationResilience`(stub provider가 key를 회전)에서 검증. |
| **NFR-AUTH1-006** | No PII in metric labels | 본 SPEC이 신설하는 모든 Prometheus 메트릭 label 값은 bounded enumerable set이며 PII(user email, sub claim raw value, IP)나 high-cardinality 값을 SHALL NOT 포함한다. `outcome`, `reason`, `mode`, `trigger` 등 enum label만 사용. 검증: SPEC-OBS-001의 `TestNoUnboundedLabels`에 신규 메트릭 추가 후 통과. |
| **NFR-AUTH1-007** | Configurability via auth.yaml hot-reload | `auth.yaml`의 `auth.mode`, `auth.clock_skew_seconds`, `auth.revocation.*`, `auth.tenant.*` 설정은 서비스 재시작 없이 SIGHUP 또는 fsnotify로 hot-reload 가능 SHALL 한다. 단 `auth.oidc.issuer`/`audience`/`allowed_issuer_hosts` 변경은 startup-only(provider singleton recreate가 필요하기 때문) — hot-reload 시 무시되며 warning log. |
| **NFR-AUTH1-008** | Prometheus metric naming convention | 본 SPEC이 신설하는 모든 메트릭은 SPEC-OBS-001의 명명 규칙(`usearch_<domain>_<noun>_<unit>` 패턴)을 SHALL 준수한다. 신설 메트릭 namespace는 `usearch_auth_*`. label 화이트리스트는 NFR-AUTH1-006에 명시됨. |
| **NFR-AUTH1-009** | Production-mode startup warning | WHERE deploy env 변수 `USEARCH_ENV == "production"` AND `auth.mode != "strict"`인 경우, startup 시 WARN 레벨로 명확한 메시지("auth mode 'permissive' in production environment; expect untrusted requests")를 SHALL 출력한다. `usearch_auth_mode{mode}` 게이지 emit도 함께. 운영자 misconfiguration 조기 감지 목적. |

---

## 4. Exclusions (What NOT to Build)

[HARD] 본 SPEC은 다음 항목을 명시적으로 제외한다. 각 항목은 후속 SPEC 또는
별도 트랙의 책임이다.

- **RBAC / 정책 평가 없음** — JWT 검증 후의 권한 결정(어떤 user가 어떤
  endpoint에 접근 가능한지)은 SPEC-AUTH-002(Casbin)의 책임. 본 SPEC은
  user_id + tenant_id + raw claim map을 제공할 뿐이다.
- **Audit log subsystem 미구현** — auth.validation 이벤트 stderr JSON
  line만 emit한다. Postgres `audit_log` 테이블, replay 기능, S3 archive는
  SPEC-AUTH-003의 책임. 본 SPEC은 AUTH-003이 consume할 line schema의
  forward-compat만 보장한다.
- **OAuth2 authorization code flow 미구현(v1)** — `POST /v1/auth/callback`
  endpoint는 v1에서 등록되되 501 Not Implemented 응답. token exchange의
  실제 구현은 frontend OAuth2 flow가 필요한 SPEC-UI-001(M7)에 위임.
  본 SPEC v1은 **bearer token validation only** — JWT는 외부 도구(예:
  `kc-auth` CLI, frontend OAuth2 lib)가 제공한다고 가정한다.
- **User self-service signup 미구현** — 사용자 생성·삭제·비밀번호 재설정은
  Keycloak/Authentik admin console 또는 provider-native API의 책임이며
  본 SPEC 범위 밖이다.
- **MFA / WebAuthn 직접 구현 안 함** — 모든 MFA는 OIDC provider 단에서
  처리되며 JWT의 `amr` claim으로만 본 SPEC에 expose된다(향후 RBAC에서 활용).
- **Multi-org / B2B SSO 구성 미지원** — V1은 single issuer + single
  audience list 가정. 여러 OIDC provider를 동시에 trust해야 하는 SaaS
  scenario는 V2(M9+)에 위임.
- **API key 인증 없음** — bearer token만 지원. machine-to-machine 호출은
  OIDC client credentials grant(provider가 발급한 JWT)를 사용해야 하며
  별도 long-lived API key 발급 mechanism은 V2에 위임.
- **Token introspection RPC 미사용** — RFC 7662 introspection은 provider
  round-trip latency를 매 request마다 추가하므로 사용하지 않는다. 모든
  검증은 local JWKS 기반 stateless verification.
- **Anthropic-style PAT / PKCE 흐름 미구현** — RP-Initiated Logout만
  필요하다(D6). PKCE는 frontend OAuth2 flow의 책임(SPEC-UI-001).
- **Token refresh handling 미구현** — refresh token은 client-side
  (frontend / CLI)에서 관리한다. 본 SPEC은 access/id token만 검증한다.
- **DEEP-004 cost_ledger schema 변경 없음** — `cost_ledger.user_id` TEXT
  컬럼은 그대로 유지된다(REQ-DEEP4-002와 joint invariant). 본 SPEC은
  ledger row의 `user_id` 값 분포만 점진적으로 변경한다(`"anonymous"` →
  OIDC `sub`).

---

## 5. Acceptance Scenarios

상세 Given/When/Then 시나리오는 `.moai/specs/SPEC-AUTH-001/acceptance.md`에
정의되어 있다. 본 절은 인덱스를 제공한다.

| Scenario | 설명 | Coverage |
|----------|------|----------|
| §5.1 | valid JWT bearer → 200, sub이 costguard.UserIDKey로 주입 | REQ-001, 003, 006 |
| §5.2 | permissive mode + 헤더 부재 → anonymous fallback, 200 | REQ-004 |
| §5.3 | JWKS rotation: provider가 key 회전 → 본 client가 unknown-kid 강제 fetch 후 통과 | REQ-002, NFR-005 |
| §5.4 | expired JWT → 401 + reason="expired" (anonymous fallback 안 됨) | REQ-005 |
| §5.5 | JWT의 sub이 costguard.IdentityMiddleware의 X-User-Id보다 우선 | REQ-003, 006 |
| §5.6 | tenant.mode="claim" + JWT의 `org_id` claim → costguard.TenantIDKey | REQ-007 |
| §5.7 | strict mode + 헤더 부재 → 401; disabled mode → bypass | REQ-004, 008 |
| §5.8 | logout endpoint → 302 + end_session_endpoint URL; revocation enabled 시 Redis set | REQ-009, 010 |
| §5.9 | SSRF block: http:// scheme 또는 private IP issuer → startup fatal exit; `/v1/auth/callback` rate-limit 60/min | REQ-011, 012 |
| Edge1 | clock skew 25s 미래 토큰 PASS, 35s FAIL | REQ-003, NFR-004 |
| Edge2 | revocation enabled + Redis 단절 → fail-open default; fail-closed override 시 401 | REQ-010 |

---

## 6. Dependencies & Blocks

### 6.1 Upstream SPEC dependencies (depends_on)

- **SPEC-BOOT-001** (implemented) — `internal/auth/` stub 패키지를 reserve
  (`internal/auth/auth.go` line 1-3, single empty package decl). 본 SPEC이
  reservation을 fulfill한다.
- **SPEC-OBS-001** (implemented) — 신설 Prometheus 메트릭이 OBS-001
  registry에 등록되며 NFR-OBS-002 cardinality safety를 SHALL 준수한다.
  request-id propagation 패턴과 OTel span attribute convention 재사용.
- **SPEC-LLM-001** (implemented) — middleware-based response observability
  emission 패턴(`internal/llm/client.go::cost middleware`)을 참조한다.
  JWT 미들웨어의 metric emission 구조가 LLM-001의 cost middleware와
  parallel하다.

### 6.2 Downstream blocked SPECs (blocks)

- **SPEC-AUTH-002** (planned, M6) — Casbin RBAC. AUTH-001의 raw claim
  map(`auth.ClaimsKey`)을 consume하여 roles/groups → permission 매핑.
- **SPEC-AUTH-003** (planned, M6) — audit log. AUTH-001의 stderr JSON
  line schema(§9.3 in research)를 inherit하고 Postgres `audit_log`
  테이블에 persist.
- **SPEC-IDX-004** (planned, M6) — shared index multi-tenancy. Qdrant
  Tiered Multitenancy / Meili per-tenant token이 AUTH-001의
  `costguard.TenantIDKey` context value를 사용.
- **SPEC-IDX-005** (planned, M6) — team-shared answer reuse. 팀 인덱스에서
  사전 lookup 시 `tenant_id` scoping에 AUTH-001 claim 사용.

### 6.3 Joint invariant with SPEC-DEEP-004 (forward-compat)

DEEP-004 REQ-DEEP4-002 + §6.3 (lines 326-332)이 명시한 frozen contract를
본 SPEC이 fulfill한다. 본 SPEC ship 시점에 다음이 SHALL 보장된다:

- `cost_ledger.user_id` TEXT 컬럼 schema 변경 없음.
- `cost_ledger` 마이그레이션 추가 없음 (`deploy/postgres/migrations/`에
  본 SPEC은 SQL 파일을 추가하지 SHALL NOT 한다).
- `costguard.UserIDKey` context key 값은 (`auth-001-ga=true` AND JWT
  valid AND `auth.mode != "disabled"`) 일 때만 JWT `sub`로 채워지며, 그
  외에는 기존 X-User-Id 경로 그대로 (REQ-AUTH1-006).
- DEEP-004의 모든 기존 테스트(특히 `TestIdentityMiddlewareReadsXUserId`,
  `TestIdentityMiddlewareDefaultsAnonymous`)는 본 SPEC ship 후에도 SHALL
  통과한다 (REQ-AUTH1-006 acceptance).

본 commitment는 M6 release gate에서 schema review checkpoint로 재검증.

---

## 7. Files to Create / Modify

### 7.1 Created

| Marker | Path | Purpose |
|--------|------|---------|
| [NEW] | `internal/auth/middleware.go` | chi v5 미들웨어 — JWTValidationMiddleware + bypass allowlist orchestration |
| [NEW] | `internal/auth/validator.go` | provider.Verifier wrapper + claim extraction + clock skew handling |
| [NEW] | `internal/auth/discovery.go` | `oidc.NewProvider` wrapper + SSRF validation + startup-time fatal exit |
| [NEW] | `internal/auth/config.go` | auth.yaml 로더 + koanf hot-reload watcher + mode/issuer/audience validation |
| [NEW] | `internal/auth/tenant.go` | tenant.mode 분기 (claim / header / static) → TenantIDKey 주입 |
| [NEW] | `internal/auth/revocation.go` | optional Redis revocation list helpers (EXISTS / SADD) |
| [NEW] | `internal/auth/logout.go` | RP-Initiated Logout 핸들러 + revocation list integration |
| [NEW] | `internal/auth/callback.go` | `/v1/auth/callback` rate-limit + 501 stub (v1 scope) |
| [NEW] | `internal/auth/metrics.go` | Prometheus collector 등록 헬퍼 |
| [NEW] | `internal/auth/types.go` | `ClaimsKey` constant, `Claims` struct, error types |
| [NEW] | `internal/auth/middleware_test.go` | full middleware test suite (incl. DEEP-004 forward-compat) |
| [NEW] | `internal/auth/validator_test.go` | JWT validation test matrix (signature, iss, aud, exp, nbf, sub) |
| [NEW] | `internal/auth/discovery_test.go` | SSRF block test cases (HTTPS, private IP, allowlist) |
| [NEW] | `internal/auth/tenant_test.go` | tenant mode 3 branches |
| [NEW] | `internal/auth/revocation_test.go` | Redis SET / EXISTS / fail-open/closed |
| [NEW] | `internal/auth/logout_test.go` | 302 + end_session + revocation integration |
| [NEW] | `internal/auth/callback_test.go` | rate-limit + 501 |
| [NEW] | `internal/auth/oidc_integration_test.go` | integration test using in-process OIDC stub + real coreos/go-oidc verifier |
| [NEW] | `internal/auth/testdata/oidc_stub/oidc_stub.go` | in-process OIDC stub: discovery + JWKS + token issuance helper |
| [NEW] | `internal/auth/testdata/oidc_stub/oidc_stub_test.go` | stub self-test |
| [NEW] | `.moai/config/sections/auth.yaml` | `auth.*` 신설 config |
| [NEW] | `cmd/usearch-api/handlers/auth.go` | logout + callback handler registration |

### 7.2 Modified

| Path | Change |
|------|--------|
| `cmd/usearch-api/main.go` | auth.Init 호출 + JWT 미들웨어를 chi 체인 맨 앞(request-id middleware 다음)에 wire + admin port는 그대로 인증 없이 노출 |
| `cmd/usearch-api/handlers/synthesis.go`, `cmd/usearch-api/handlers/deep.go` | 변경 없음 — JWT 미들웨어는 mux 등록 시점에 wrap |
| `internal/deepagent/costguard/middleware.go` | `IdentityMiddleware` (line 42-66)에 source-priority 분기 추가 (context value > X-User-Id header > "anonymous"). REQ-AUTH1-006 구현. strictly additive — 기존 테스트 모두 PASS. |
| `internal/obs/metrics/metrics.go` | `registerAuth(r)` 헬퍼 호출 추가 |
| `internal/obs/obs.go` | `obs.AuthAttempts`, `obs.AuthFailures`, `obs.JWKSRefresh` collector re-export |
| `internal/obs/metrics/metrics_test.go` | `TestNoUnboundedLabels` 화이트리스트에 `outcome`, `reason`, `mode`, `trigger` 라벨 추가 (단 `mode`, `trigger`는 이미 다른 SPEC에서 추가됐을 수 있음 — diff 시 확인) |
| `.env.example` | `AUTH_OIDC_ISSUER`, `AUTH_OIDC_AUDIENCE`, `AUTH_MODE`, `AUTH_001_GA` 등 신규 env-var 문서화 |
| `go.mod` / `go.sum` | direct require `github.com/coreos/go-oidc/v3 v3.18.0` 추가. `github.com/golang-jwt/jwt/v5`는 indirect → direct로 승격 |
| `deploy/docker-compose.dev.yml` | (optional) OIDC stub 서비스 추가 — local dev에서 keycloak 대체용 |

### 7.3 Existing — Unchanged (verified invariants)

- `deploy/postgres/migrations/0002_cost_ledger.sql` (DEEP-004) — schema 변경
  없음. 본 SPEC은 cost_ledger 마이그레이션을 추가하지 SHALL NOT 한다.
- `internal/deepagent/costguard/ledger.go`, `cap_check.go`, `haiku_screen.go`
  — read-only consumer. `costguard.UserIDKey` context value의 source가
  바뀌어도 코드 변경 없음.
- `internal/llm/client.go` (LLM-001) — read-only consumer.
- `internal/obs/reqid/` (OBS-001) — read-only consumer.
- `services/researcher/`, `services/storm/` Python 사이드카 — JWT 검증은
  Go-side ingress에서만 수행. Python sidecar는 internal RPC trust로 유지
  (M6 후속 IDX-004/IDX-005에서 internal mTLS 또는 service token으로
  보강 가능).

---

## 8. Open Questions

본 SPEC은 §1.1의 8개 pinned decision으로 대부분의 ambiguity를 해소했다.
다음 항목은 plan-auditor와의 협의 또는 첫 운영 데이터 기반 튜닝이 필요한
경계 사례다.

1. **`auth.mode` production default**: V1 default는 `permissive`. 단,
   `USEARCH_ENV == "production"`에서 강제로 `strict`로 promote할지
   여부. **권장**: V1 ship 시점에는 WARN log만 emit하고(NFR-AUTH1-009)
   M6 ship 후 30일 운영 안정화 보면 `strict` promotion(별도 minor SPEC).

2. **`auth.clock_skew_seconds` default**: 30s가 적절한가? AWS/GCP 환경
   NTP drift는 < 100ms이지만 self-host bare-metal 환경에서 NTP가 잘못
   설정된 사례가 종종 보고됨. **권장**: 30s 유지, NFR-AUTH1-004 검증.
   운영자가 NTP 모니터링하면 1-5s로 tighten 가능.

3. **`auth.tenant.mode` default**: V1 default `static`(`default` 단일
   tenant). 단일-팀 self-host에 적합. M6에서 multi-team이 본격화되면
   `claim`으로 promote. **권장**: V1 ship 시 `static` 유지, AUTH-002가
   `claim`으로 promote할 때 path 검증 추가.

4. **JWT `jti` claim의 존재 가정**: revocation list가 동작하려면 provider가
   `jti` claim을 emit해야 한다. Keycloak/Authentik은 default `jti` emit
   하지만, 일부 hosted provider(Auth0 일부 tier)는 emit 안 함. **권장**:
   revocation 활성화 시 `jti` claim 부재 token을 warning log + revocation
   skip + counter `usearch_auth_token_revoked_total{trigger="missing_jti"}`
   로 추적.

위 4개는 plan-auditor가 SPEC을 PASS로 평가하기 위해 필수적인 결정이 아니다.
모두 first-30-day 운영 데이터 또는 후속 minor SPEC으로 튜닝 가능한 항목이다.

---

*End of SPEC-AUTH-001 v0.1.0 (draft).*
