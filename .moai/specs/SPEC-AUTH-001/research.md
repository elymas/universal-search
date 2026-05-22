# SPEC-AUTH-001 Deep Research

Generated: 2026-05-22T00:00:00Z
Author: manager-spec (Phase 0.5 — context-derived)
Consumed by: manager-spec (Phase 1B), plan-auditor (Phase 2.3)

---

## 0. Scope of This Research

본 research.md는 M6 milestone의 첫 deliverable이자 team plane의 release
gate인 SPEC-AUTH-001(OIDC SSO)에 대한 코드베이스 분석 + 외부 라이브러리
검증 + 아키텍처 결정 기록이다. roadmap.md §M6의 scope 한 줄
("Keycloak / Authentik integration, JWT validation middleware",
`.moai/project/roadmap.md` line 83)을 (a) OIDC discovery + JWKS rotation
인프라, (b) JWT validation 미들웨어, (c) 기존 X-User-Id 헤더 경로와의
forward-compat transition, (d) 익명 fallback과 dev 환경 stub의 4개 축으로
분해해 각 축의 라이브러리 선택·코드 진입점·테스트 전략·관측 surface를
명세화한다.

본 SPEC은 M6의 첫 SPEC이자 AUTH-002(Casbin RBAC)/AUTH-003(audit log)/
IDX-004(multi-tenant index)/IDX-005(team-shared answer reuse)의 **direct
blocker**다. 위 4개 SPEC 모두 "신뢰할 수 있는 user_id / tenant_id /
team_id 클레임"에 의존하기 때문에 AUTH-001 없이는 M6 어느 줄도 시작할 수
없다.

또한 본 SPEC은 M5에서 ship된 SPEC-DEEP-004의 forward-compat 약속을
실행에 옮기는 SPEC이다. DEEP-004 REQ-DEEP4-002와 §6.3은 "AUTH-001 ship 시
JWT 미들웨어가 같은 컨텍스트 키 `costguard.UserIDKey`에 user_id를 주입한다"
는 contract를 frozen 상태로 명시했다(`.moai/specs/SPEC-DEEP-004/spec.md`
line 181, lines 326-332). 본 SPEC은 이 contract를 schema 변경 없이
충족해야 한다.

---

## 1. Cost / Surface Analysis

### 1.1 영향받는 HTTP 진입점

AUTH-001이 ship되면 다음 진입점이 모두 JWT 미들웨어를 통과한다:

- `POST /query`, `POST /query/stream` (SPEC-SYN-001, SPEC-SYN-004)
- `POST /deep`, `POST /deep?mode=agents` (SPEC-DEEP-002, SPEC-DEEP-003)
- `GET /metrics`, `GET /healthz` — admin port (auth bypass 대상)
- `GET /v1/auth/callback`, `POST /v1/auth/logout` — auth endpoint (본 SPEC 신설)
- `GET /v1/auth/userinfo` — claim debug endpoint (본 SPEC 신설)

신설할 미들웨어 체인 (chi v5):

```
Request
  → request-id middleware (SPEC-OBS-001)
  → auth.JWTValidationMiddleware  ← 본 SPEC (NEW)
  → identity bridge: write `costguard.UserIDKey` from JWT sub  ← 본 SPEC (NEW)
  → costguard.IdentityMiddleware (SPEC-DEEP-004, 이미 ship)
  → costguard.CapCheckMiddleware (SPEC-DEEP-004, 이미 ship)
  → handler
```

근거: 기존 `internal/deepagent/costguard/middleware.go` line 38-66의
`IdentityMiddleware`는 X-User-Id 헤더를 1차 source로 읽는다. 본 SPEC은
그 앞에 JWT 미들웨어를 삽입하여 sub claim을 context에 미리 주입하고,
costguard의 IdentityMiddleware가 context를 우선 확인하도록 보강한다.
이렇게 하면 `cost_ledger.user_id` 컬럼 schema는 변경 없이 단순 source만
바뀐다 (REQ-DEEP4-002의 forward-compat 약속 그대로).

### 1.2 Latency 영향

JWT 검증 path latency (local key cache hit):
- HMAC-SHA256 (대조군): ~10 µs / token
- RSA-SHA256 (Keycloak/Authentik 기본): ~150–300 µs / token (2048-bit key)
- ECDSA-SHA256: ~80–150 µs / token

JWKS cache miss → remote fetch:
- 첫 호출: ~50–200 ms (HTTP + JSON parse + key parse)
- keyfunc 라이브러리의 background refresh로 hot-path에서는 발생 X

→ **NFR target**: p95 ≤ 5 ms (cache hit path), p99 ≤ 300 ms (cache miss
forced refresh). NFR-AUTH1-001 참조.

### 1.3 비용 누락 risk (DEEP-004 forward-compat)

`cost_ledger.user_id` TEXT 컬럼은 opaque이므로 transition 시 schema
마이그레이션이 불필요하다. 본 SPEC ship 직후 ledger의 `user_id` 값 분포가
변경된다:

- AUTH-001 ship 전: 대부분 `"anonymous"` + 일부 X-User-Id 헤더 값
- AUTH-001 ship 후 (`auth-001-ga=true`): 대부분 OIDC `sub` claim 값
  (UUID 또는 email-like)

reconciliation: AUTH-003(M6) audit log에서 ledger user_id 분포 drift를
모니터링한다. 본 SPEC은 transition 자체의 책임만 진다.

---

## 2. OIDC Provider Strategy

### 2.1 Target Providers

tech.md §3 "Team plane" (`.moai/project/tech.md` line 69)이 명시한 대로
V1 default는 self-hosted Keycloak 또는 Authentik이다. 하지만 본 SPEC은
provider-specific 코드를 작성하지 않는다 — OIDC discovery + JWKS standard만
사용하면 모든 OIDC 1.0-compliant provider가 동작한다.

검증된 provider 매트릭스:

| Provider | Discovery URL pattern | JWKS pattern | Default claims |
|----------|----------------------|--------------|----------------|
| Keycloak | `/realms/{realm}/.well-known/openid-configuration` | `/realms/{realm}/protocol/openid-connect/certs` | `sub`, `preferred_username`, `email`, `realm_access` |
| Authentik | `/application/o/{slug}/.well-known/openid-configuration` | `/application/o/{slug}/jwks/` | `sub`, `preferred_username`, `email`, `groups` |
| Auth0 (hosted, optional) | `https://{tenant}.auth0.com/.well-known/openid-configuration` | exposed in discovery | `sub`, `email`, custom namespaced claims |
| Clerk (hosted, optional) | `https://{instance}.clerk.accounts.dev/.well-known/openid-configuration` | exposed in discovery | `sub`, `email`, `org_id` |

출처:
- Keycloak: https://www.keycloak.org/securing-apps/oidc-layers (WebFetch
  2026-05-22). well-known endpoint: `/realms/{realm-name}/.well-known/openid-configuration`.
  Certificate endpoint: `/realms/{realm-name}/protocol/openid-connect/certs`.
- Authentik: https://docs.goauthentik.io/docs/add-secure-apps/providers/oauth2/
  (WebFetch 2026-05-22). well-known: `/application/o/{slug}/.well-known/openid-configuration`.
  JWKS: `/application/o/{slug}/jwks/`.
- OIDC Discovery 1.0 spec: https://openid.net/specs/openid-connect-discovery-1_0.html
  (WebFetch 2026-05-22). Required fields: `issuer`, `authorization_endpoint`,
  `token_endpoint`, `jwks_uri`, `response_types_supported`,
  `subject_types_supported`, `id_token_signing_alg_values_supported`
  (must include RS256).

### 2.2 결정: provider-agnostic 구현

본 SPEC은 vendor-specific 클레임(예: Keycloak `realm_access.roles`)을
hot path에서 해석하지 않는다. AUTH-002 (Casbin RBAC)이 roles/groups
claim의 의미론적 매핑을 책임진다. 본 SPEC은 다음만 보장한다:

1. `iss` claim이 deploy 시점에 설정된 `auth.oidc.issuer`와 정확히 일치.
2. `aud` claim이 `auth.oidc.audience` 화이트리스트 중 하나와 일치.
3. `exp`, `nbf`가 현재 시각과 clock skew 범위 내에서 valid.
4. 서명이 JWKS의 `kid`-matching key로 검증 통과.
5. `sub` claim이 비어있지 않은 string.

기타 모든 claim은 raw map으로 context에 attach하고 downstream(특히
AUTH-002)이 해석한다.

### 2.3 결정 근거 (왜 single config로 multi-provider 지원)

- **Operational simplicity**: 운영자가 Keycloak에서 Authentik으로
  교체해도 코드 변경 없이 deep.yaml/auth.yaml의 `issuer`/`audience`
  + JWKS URL만 바꾼다.
- **CI test surface**: `services/auth-stub/` (본 SPEC 신설) 하나로 두
  provider를 모두 emulate할 수 있다 (둘 다 OIDC 1.0 표준).
- **Vendor risk**: tech.md §6 risks(line 144-153)에 명시된 "ToS / 라이선스
  보호" 원칙과 일관. provider-lock-in을 차단한다.

---

## 3. Library Selection — JWT + JWKS

### 3.1 후보 비교

| Library | Use case | Pros | Cons |
|---------|----------|------|------|
| `github.com/coreos/go-oidc/v3/oidc` | OIDC provider discovery + ID token verification | High-level API: `oidc.NewProvider(ctx, issuer)` → `provider.Verifier(cfg)`. Built-in JWKS rotation. coreos/RedHat-maintained. CoreOS/RedHat backing. Apache-2.0. | Slightly heavier API surface (oauth2 integration). Only verifies via JWKS — not direct access to keys. |
| `github.com/MicahParks/keyfunc/v3` | JWKS-driven keyfunc for `golang-jwt/jwt/v5` | Lightweight. Background refresh of JWKS. Unknown-kid auto-fetch. Pairs with golang-jwt's standard `Parse` API. Apache-2.0. | Lower-level — must wire claim validation manually (iss, aud, exp). |
| `github.com/golang-jwt/jwt/v5` | JWT parse + claim validation primitive | Already in `go.sum` (v5.3.1, see `go.mod` line 21). De-facto standard. Apache-2.0. | No JWKS support; needs a keyfunc supplier. |
| `github.com/luikyv/go-oidc` | Authorization Server impl (RP+OP) | Full OP/RP. | 본 SPEC은 RP만 필요. overkill. |
| `github.com/zitadel/oidc` | RP+OP SDK | Mature. | OP features 본 SPEC 범위 밖. |

출처:
- coreos/go-oidc v3.18.0 (April 2026): https://github.com/coreos/go-oidc
  (WebFetch 2026-05-22). Import path `github.com/coreos/go-oidc/v3/oidc`.
  Apache-2.0.
- MicahParks/keyfunc: https://github.com/MicahParks/keyfunc (WebFetch
  2026-05-22). v3 latest. Background JWKS refresh + unknown-kid re-fetch.
  Apache-2.0.
- golang-jwt/jwt/v5 already in go.sum: `go.mod` line 21.

### 3.2 결정: 하이브리드 — coreos/go-oidc primary, keyfunc fallback

**Primary path**: `github.com/coreos/go-oidc/v3/oidc`.

이유:
- `oidc.NewProvider(ctx, issuer)`가 discovery + JWKS endpoint 추출을
  한 줄로 처리하고, `provider.Verifier(&oidc.Config{ClientID: aud})`가
  `iss`/`aud`/`exp`/`nbf`/서명 검증을 한 번에 처리.
- background JWKS rotation을 라이브러리가 자체 관리(`*RemoteKeySet`).
- coreos/go-oidc의 v3 break(`NewRemoteKeySet` returns `*RemoteKeySet`
  instead of interface)는 본 SPEC의 사용 패턴에 영향 없음.

**Fallback / advanced path**: `MicahParks/keyfunc/v3` + `golang-jwt/jwt/v5`.

이유: 특정 provider가 OIDC discovery 1.0을 제대로 구현하지 않거나
custom claim validation이 필요할 때 직접 keyfunc로 verify한다. 본 SPEC
V1은 기본 path만 ship한다.

### 3.3 의존성 추가

새로 추가될 직접 의존성: `github.com/coreos/go-oidc/v3`.

이미 transitively 또는 직접 존재:
- `github.com/golang-jwt/jwt/v5 v5.3.1` (indirect, `go.mod` line 21 → 본 SPEC이 direct로 승격)
- `github.com/go-jose/go-jose/v3 v3.0.4` (indirect, `go.mod` line 20 — coreos/go-oidc transitive)

→ direct require 추가: `coreos/go-oidc/v3`. `keyfunc/v3`은 v1 ship에는 미추가
(향후 advanced path 필요 시 추가).

### 3.4 버전 핀

- `github.com/coreos/go-oidc/v3 v3.18.0` (2026-04 최신).
- pin policy: `.claude/rules/moai/core/lsp-client.md`가 powernap에 적용한
  것과 동일하게, integration test (`internal/auth/oidc_integration_test.go`)
  통과 후에만 bump.

---

## 4. Identity Bridge: AUTH-001 ↔ DEEP-004 forward-compat

### 4.1 문제

DEEP-004는 V1에서 `costguard.UserIDKey` 컨텍스트 키에 `X-User-Id` 헤더
값(또는 `"anonymous"`)을 주입한다. AUTH-001이 ship되면 같은 컨텍스트 키에
JWT `sub` claim 값을 우선 주입해야 한다 (REQ-DEEP4-002).

### 4.2 결정: 같은 context key, switched source

`internal/auth/middleware.go`의 새 `JWTValidationMiddleware`는 검증 통과
시 다음을 수행한다:

```
ctx := r.Context()
ctx = context.WithValue(ctx, auth.ClaimsKey, claims) // 본 SPEC NEW
ctx = context.WithValue(ctx, costguard.UserIDKey, claims.Sub)  // DEEP-004 호환
ctx = context.WithValue(ctx, costguard.TenantIDKey, deriveTenant(claims))
```

`deriveTenant`는 deploy 시점 config로 결정:
- mode `"claim"`: claim의 `tenant_id` 또는 `org_id` 또는 `team_id` 값을 사용.
- mode `"header"`: `X-Tenant-Id` 헤더 값을 사용 (downstream proxy가 제공).
- mode `"static"`: deploy 시점에 고정 `default_tenant_id`.

기본값: `"static"` + `default`. AUTH-002에서 RBAC + tenant claim mapping이
ship되면 `"claim"`으로 전환된다.

### 4.3 환경 변수 게이트 `auth-001-ga`

DEEP-004 REQ-DEEP4-002가 명시한 환경 변수 `auth-001-ga`는 deploy-time
feature flag이다:

- `auth-001-ga=false` (or unset): 새 JWTValidationMiddleware는 등록되되
  **bypass mode**로 동작 (검증 skip, request 그대로 통과). 기존 X-User-Id
  헤더 경로가 active.
- `auth-001-ga=true`: 새 JWTValidationMiddleware가 active. JWT 부재 또는
  검증 실패 시 401. 단, `auth.allow_anonymous: true` (deploy 시점 config)
  로 명시 설정 시 fallback 동작 (§5 참조).

이 토글로 production에서 staged rollout 가능 (env 변경 → SIGHUP/restart로
즉시 적용).

### 4.4 costguard.IdentityMiddleware 호환 확장

기존 `internal/deepagent/costguard/middleware.go` line 42-66의
`IdentityMiddleware`는 X-User-Id 헤더만 본다. 본 SPEC은 이를 다음 우선
순위로 확장한다 (작은 패치):

1. context에 이미 `UserIDKey`가 있으면 그것을 사용 (JWT 미들웨어가 주입).
2. 없으면 `X-User-Id` 헤더 (V1 경로).
3. 없으면 `"anonymous"`.

이 변경은 schema 영향 없음. 단순히 context value 확인 분기 추가.

### 4.5 cost_ledger.user_id 영향

DEEP-004 REQ-DEEP4-002의 frozen contract: schema 변경 없음. opaque TEXT
유지. 본 SPEC이 ledger row의 user_id 값 분포만 점진적으로 바꾼다 (anonymous
→ OIDC sub).

---

## 5. Anonymous Fallback Strategy

### 5.1 동기

`/healthz`, `/metrics` 등 unauthenticated endpoint와 dev/CI 환경의 stub
경로는 JWT 없이도 동작해야 한다. 또한 self-host 시 일부 운영자는 V1.1
초기에 anonymous를 그대로 두고 후속에 JWT를 강제하는 single-tenant 모드를
원할 수 있다.

### 5.2 결정: 3-mode fallback

`auth.yaml`의 `auth.mode`:

| mode | JWT 미부착 시 | JWT 검증 실패 시 |
|------|--------------|-----------------|
| `"strict"` | 401 Unauthorized | 401 Unauthorized |
| `"permissive"` | anonymous + 200 | 401 Unauthorized |
| `"disabled"` | bypass (auth-001-ga=false 동치) | bypass |

기본값: `auth.mode: "permissive"` (V1.1 transition window). production
권장: 운영 안정 후 `"strict"`로 전환.

### 5.3 보호 엔드포인트 분리

`/healthz`, `/metrics`, `/v1/auth/callback`은 무조건 bypass (config와
무관). admin port (`USEARCH_ADMIN_PORT`)는 본 SPEC의 미들웨어 체인에
포함되지 않는다.

### 5.4 anonymous user_id 값

permissive mode + JWT 미부착 시:
- `costguard.UserIDKey`에 `"anonymous"` 주입 (DEEP-004 fallback과 동일).
- `cost_ledger`에 `outcome="success"`로 정상 기록 (cap 평가는 anonymous
  공유 버킷 사용).

---

## 6. Logout & Token Revocation Strategy

### 6.1 OIDC RP-Initiated Logout

OIDC 1.0 §RP-Initiated Logout 사양에 따라 본 SPEC은:

- `POST /v1/auth/logout` endpoint 신설.
- 응답: 302 Redirect to `{end_session_endpoint}?id_token_hint={...}&post_logout_redirect_uri={...}`.
  `end_session_endpoint`는 discovery document에서 자동 추출.
- session 자체는 stateless (JWT만 사용). 따라서 client-side에서 토큰을
  버리면 충분. 추가로 본 SPEC은 단기 revocation list (Redis SET, TTL =
  longest token exp = 1h)를 옵션으로 제공.

### 6.2 Revocation list 설계 (optional)

`auth.revocation.enabled: true`인 경우:
- `POST /v1/auth/revoke` endpoint가 JWT의 `jti` claim을 Redis SET
  `auth:revoked:{jti}`에 EXPIRE `exp - now`로 추가.
- JWTValidationMiddleware가 검증 시 Redis EXISTS 1회 추가 (RTT < 1ms).
- Redis 단절 시: fail-open (default) — revocation list가 잠시 무효해질
  뿐 정상 인증은 계속 통과.

### 6.3 결정: V1은 RP-Initiated Logout만 ship

revocation list는 옵션 기능이며 V1 default는 `enabled: false`. token
revocation을 강하게 요구하는 환경(예: 보안 감사 대응)에서만 켜도록.
이유: 모든 요청에 Redis RTT 1회 추가하는 비용 vs JWT 만료 TTL의 짧음
(보통 5–15분)의 trade-off.

---

## 7. Dev / CI Stub Strategy

### 7.1 문제

local dev + CI에서 Keycloak/Authentik instance를 띄우는 것은 무겁다
(memory ~500MB+, startup ~30s). 또한 integration test는 reproducible JWT
가 필요하다.

### 7.2 결정: in-process OIDC stub server

본 SPEC은 `internal/auth/testdata/oidc_stub/`에 다음을 ship한다:

- `oidc_stub.go`: net/http 기반 minimal OIDC RP server.
  - `/.well-known/openid-configuration` 엔드포인트
  - `/jwks` 엔드포인트 (자체 RSA-2048 keypair 또는 ECDSA-P256)
  - `/token` 엔드포인트 (test 시 사전 정의된 JWT 발급)
- `IssueToken(claims map[string]any, ttl time.Duration) string` helper
  (test 코드에서 호출)
- testcontainers 불필요. unit test에서 `httptest.NewServer(stub)` 1줄로
  띄움.

### 7.3 사용 시나리오

```
provider := oidc_stub.New(t)
defer provider.Close()
token := provider.IssueToken(map[string]any{"sub": "alice@example.com"}, 5*time.Minute)
req.Header.Set("Authorization", "Bearer "+token)
```

CI에서 같은 stub을 dev docker-compose 서비스로도 노출 가능 (옵션 — `deploy/docker-compose.dev.yml`에 추가).

---

## 8. Pinned Decisions (No User Re-prompt)

다음 8개 결정은 본 research 단계에서 context-derived로 확정한다. 이후 SPEC
주 본문(§1.1)에서 같은 번호로 재참조된다.

| ID | Decision | Recommendation | Alternatives Considered |
|----|----------|----------------|------------------------|
| **D1** | OIDC client library | `github.com/coreos/go-oidc/v3` primary; `MicahParks/keyfunc/v3` reserved for advanced path | luikyv/go-oidc (OP feature overkill), zitadel/oidc (mature but heavier), manual jwt+JWKS (Reinvent wheel) |
| **D2** | Issuer/audience validation | `iss` exact match + `aud` whitelist match; no token introspection RPC | RPC introspection (latency penalty per request), accept any-aud (security risk) |
| **D3** | JWKS rotation | rely on coreos/go-oidc's `*RemoteKeySet` background refresh (default 4–6h cache) with on-demand re-fetch on unknown kid | manual JWKS cron job (operational burden), no caching (latency penalty) |
| **D4** | Identity context key | reuse `costguard.UserIDKey` (per DEEP-004 forward-compat contract); add `auth.ClaimsKey` for full claim map | new auth-specific key (breaks DEEP-004 contract), proto annotation (out of scope) |
| **D5** | Anonymous fallback | 3-mode (`strict` / `permissive` / `disabled`); default `permissive` for V1 transition window | always-strict (breaks dev/CI), always-permissive (security risk in prod) |
| **D6** | Logout strategy | OIDC RP-Initiated Logout + optional revocation list (Redis SET, TTL = max token exp, fail-open) | revocation list always-on (latency + Redis dependency), no logout (poor UX) |
| **D7** | Dev/CI stub | in-process Go OIDC stub (`internal/auth/testdata/oidc_stub/`); no Keycloak in CI | testcontainers Keycloak (slow startup), mock-only (no real verification path) |
| **D8** | SSRF protection on discovery | enforce HTTPS scheme + host allowlist + DNS resolution against private IPv4/IPv6 (RFC 1918 / fc00::/7 / ::ffff:127.0.0.1) blocked unless `auth.oidc.allow_private_issuer: true` | trust user config (SSRF risk), proxy egress (operational complexity) |

---

## 9. Observability Surface

### 9.1 Prometheus 메트릭

SPEC-OBS-001 NFR-OBS-002의 cardinality safety 규칙을 따른다.

| Metric | Type | Labels | Cardinality |
|--------|------|--------|-------------|
| `usearch_auth_attempts_total` | CounterVec | `{outcome}` | outcome ∈ {success, expired, invalid_signature, invalid_aud, invalid_iss, missing_token, jwks_unreachable, anonymous_fallback} = 8 |
| `usearch_auth_failures_total` | CounterVec | `{reason}` | reason ∈ {expired, invalid_signature, invalid_aud, invalid_iss, invalid_nbf, malformed, jwks_unreachable, revoked} = 8 |
| `usearch_auth_jwks_refresh_total` | CounterVec | `{outcome}` | outcome ∈ {success, network_error, parse_error, unknown_kid_fetch} = 4 |
| `usearch_auth_jwks_refresh_duration_seconds` | Histogram | (no labels) | buckets [0.001, 0.01, 0.05, 0.1, 0.5, 1, 5] |
| `usearch_auth_validation_duration_seconds` | Histogram | (no labels) | buckets [0.0001, 0.001, 0.005, 0.01, 0.05, 0.1] |
| `usearch_auth_token_revoked_total` | CounterVec | `{trigger}` | trigger ∈ {explicit_logout, admin} = 2 (only when revocation enabled) |
| `usearch_auth_mode` | Gauge | `{mode}` | mode ∈ {strict, permissive, disabled} = 3 (config snapshot) |

### 9.2 OTel span 속성

`/v1/*` 요청 span에 다음 속성 추가:
- `auth.outcome` (string, enum from §9.1)
- `auth.subject_hash` (sha256 of sub, low-cardinality-safe in OTel)
- `auth.token_age_seconds` (float, derived from iat)
- `auth.jwks_cache_hit` (bool)

### 9.3 Audit log

AUTH-003(M6) audit subsystem이 정의할 통합 audit log line schema와 호환되는
preliminary line을 본 SPEC도 emit한다:

```json
{
  "timestamp": "2026-05-22T14:00:00.123Z",
  "event_type": "auth.validation",
  "request_id": "req_abc123",
  "subject_hash": "sha256:...",
  "outcome": "success",
  "issuer": "https://keycloak.example/realms/team",
  "audience": "usearch-api",
  "token_age_seconds": 42.1
}
```

AUTH-003 ship 시 schema 확장은 additive 보장 — 위 6개 field는 rename/remove
SHALL NOT.

---

## 10. Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|-----------|
| **R1** SSRF on OIDC discovery URL (operator misconfigures issuer to localhost or AWS metadata service) | Medium | Critical | D8: HTTPS-only + host allowlist + private IP block. Config-time validation + run-time DNS resolution check. |
| **R2** JWKS rotation mid-deploy causing 401 spike | Medium | High | coreos/go-oidc's auto-refresh + on-demand fetch on unknown-kid handles this. NFR-AUTH1-005 enforces test coverage. |
| **R3** Clock skew between auth server and Universal Search node | Medium | Medium | `auth.clock_skew_seconds` config (default 30s). Add to `exp`/`nbf` evaluation. NFR-AUTH1-006 enforces test for skew. |
| **R4** AUTH-001 ship breaks DEEP-004 cost_ledger forward-compat | Low | Critical | DEEP-004 REQ-DEEP4-002 contract explicitly preserved (§4 in this research). `cost_ledger.user_id` schema unchanged. Phase E integration test (`TestForwardCompatWithDeep004`) re-verifies. |
| **R5** /metrics or /healthz accidentally requires JWT | Low | High | Endpoint allowlist hardcoded in middleware chain. Test `TestAdminEndpointsBypassAuth` in Phase E. |
| **R6** Token replay after logout (no server-side session) | Medium | Medium | Optional revocation list (D6). Default `enabled: false` — operator opt-in. AUTH-003 audit log captures revocation events. |
| **R7** Permissive mode left in production by mistake | Medium | High | Startup log warning when `auth.mode != "strict"` in production-marked env. `usearch_auth_mode` gauge for monitoring. NFR-AUTH1-009 enforces startup-time check. |
| **R8** Vendor JWKS endpoint goes down → all requests fail | Medium | Critical | Background refresh keeps cache warm. On JWKS unreachable + cache stale → fail-closed by default (`auth.jwks_failure_mode: "fail-closed"`). Operator can opt to fail-open for non-critical workloads. |
| **R9** Discovery endpoint rate-limiting by provider | Low | Medium | discovery is fetched once at startup + on issuer change. JWKS refresh is cached. negligible volume. |
| **R10** Custom claim mapping conflicts (e.g., two providers use different "email" claim names) | Low | Low | V1 only mandates standard OIDC core claims (`sub`, `iss`, `aud`, `exp`, `nbf`). Custom claim mapping deferred to AUTH-002. |

---

## 11. References

### 11.1 Internal SPEC documents

- `.moai/specs/SPEC-BOOT-001/spec.md` — repo scaffold, `internal/auth/` stub created (line 38)
- `.moai/specs/SPEC-OBS-001/spec.md` — observability baseline, request-id propagation pattern
- `.moai/specs/SPEC-LLM-001/spec.md` — request lifecycle middleware pattern (`internal/llm/client.go`)
- `.moai/specs/SPEC-DEEP-004/spec.md` — REQ-DEEP4-002 forward-compat contract (line 181), §6.3 commitment (lines 326-341)
- `.moai/specs/SPEC-DEEP-004/research.md` — §2 identity strategy pre-AUTH-001 (lines 95-145)
- `.moai/project/roadmap.md` — M6 row (line 83), exit criteria (line 155)
- `.moai/project/tech.md` — Team plane Auth row (line 69), security-related risks (lines 144-153)

### 11.2 Existing code touched

- `internal/auth/auth.go` — stub package, single empty file (verified 2026-05-22)
- `internal/deepagent/costguard/middleware.go` line 11-18: contextKey definitions (UserIDKey, TenantIDKey, RequestIDKey)
- `internal/deepagent/costguard/middleware.go` line 42-66: `IdentityMiddleware` to be extended (§4.4)
- `cmd/usearch-api/main.go` line 39-42: current stub mux (will gain auth middleware)
- `cmd/usearch-api/handlers/synthesis.go`, `cmd/usearch-api/handlers/deep.go`: target endpoints

### 11.3 External references

- coreos/go-oidc v3.18.0 (April 2026): https://github.com/coreos/go-oidc
  (WebFetch 2026-05-22). Import: `github.com/coreos/go-oidc/v3/oidc`.
- MicahParks/keyfunc v3: https://github.com/MicahParks/keyfunc (WebFetch 2026-05-22).
- golang-jwt/jwt v5.3.1: already in `go.sum`.
- Keycloak OIDC: https://www.keycloak.org/securing-apps/oidc-layers (WebFetch 2026-05-22).
- Authentik OAuth2 / OIDC provider: https://docs.goauthentik.io/docs/add-secure-apps/providers/oauth2/
  (WebFetch 2026-05-22).
- OIDC Discovery 1.0: https://openid.net/specs/openid-connect-discovery-1_0.html
  (WebFetch 2026-05-22). Required fields enumerated in §2.1.
- RFC 7519 (JWT): https://datatracker.ietf.org/doc/html/rfc7519
- RFC 8414 (OAuth 2.0 Authorization Server Metadata): https://datatracker.ietf.org/doc/html/rfc8414

### 11.4 Implementation references (reuse map)

| New file | Closest analog | Reference |
|----------|---------------|-----------|
| `internal/auth/middleware.go` | `internal/deepagent/costguard/middleware.go` | chi v5 middleware pattern, contextKey discipline |
| `internal/auth/validator.go` | `internal/llm/client.go` cost middleware | response-time validation + observability emit pattern |
| `internal/auth/discovery.go` | (new) | thin wrapper over `oidc.NewProvider` |
| `internal/auth/config.go` | `internal/deepagent/costguard/config.go` | koanf-based hot-reloadable config |
| `internal/auth/metrics.go` | `internal/deepagent/costguard/metrics.go` | Prometheus collector registration pattern |
| `internal/auth/testdata/oidc_stub/oidc_stub.go` | (new) | httptest.NewServer pattern; no equivalent in repo today |
| `cmd/usearch-api/handlers/auth.go` | `cmd/usearch-api/handlers/deep.go` | http.Handler implementation pattern |

---

**End of Research Document**
