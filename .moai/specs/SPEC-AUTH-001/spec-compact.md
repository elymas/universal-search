# SPEC-AUTH-001 (Compact)

id: SPEC-AUTH-001 | v0.1.0 | draft | owner: expert-security | TDD | coverage 85% | priority P0
depends_on: BOOT-001, OBS-001, LLM-001
blocks: AUTH-002, AUTH-003, IDX-004, IDX-005
title: OIDC SSO with Keycloak/Authentik integration and JWT validation middleware

## Pinned Decisions

- D1 OIDC client lib: `github.com/coreos/go-oidc/v3/oidc` primary (v3.18.0). `MicahParks/keyfunc/v3` reserved advanced fallback.
- D2 Validation: `iss` exact + `aud` whitelist + `exp`/`nbf` w/ clock skew (default 30s) + signature. No introspection RPC.
- D3 JWKS rotation: coreos/go-oidc `*RemoteKeySet` background refresh (4-6h) + unknown-kid forced fetch.
- D4 Identity context key: reuse `costguard.UserIDKey` (DEEP-004 forward-compat). Add `auth.ClaimsKey` for full claim map.
- D5 Anonymous fallback: 3-mode (strict / permissive / disabled). V1 default `permissive`. Promote to strict after stabilization.
- D6 Logout: OIDC RP-Initiated Logout endpoint. Optional Redis revocation list (default disabled).
- D7 Dev/CI stub: in-process Go OIDC stub at `internal/auth/testdata/oidc_stub/`. No testcontainers Keycloak.
- D8 SSRF protection: HTTPS-only + host allowlist + private IP block (RFC 1918 / fc00::/7 / loopback / link-local). Override only via `auth.oidc.allow_private_issuer: true`.

## EARS Requirements

### Discovery Module

- REQ-AUTH1-001 (Ubiquitous, P0): startup-time `oidc.NewProvider(issuer)` → cache endpoint metadata. Issuer mismatch or discovery failure → fatal exit.
- REQ-AUTH1-002 (Ubiquitous, P0): JWKS rotation via coreos/go-oidc `*RemoteKeySet` (background refresh + unknown-kid forced fetch). `usearch_auth_jwks_refresh_total{outcome}` counter for scheduled / unknown_kid_fetch / parse_error / network_error.

### JWT Validation Module

- REQ-AUTH1-003 (Event-Driven, P0): WHEN Authorization: Bearer present, verify (signature, iss exact, aud whitelist, exp+skew, nbf+skew, sub non-empty). On pass, inject `auth.ClaimsKey` + `costguard.UserIDKey` (sub) + `costguard.TenantIDKey` into context (per tenant.mode).
- REQ-AUTH1-004 (Optional, P1): WHERE mode=permissive AND missing header → inject "anonymous". WHERE mode=strict AND missing → 401. WHERE mode=disabled → bypass entirely (env `auth-001-ga` falsy). `usearch_auth_mode{mode}` gauge at startup.
- REQ-AUTH1-005 (Unwanted, P0): IF header present AND validation fails → 401 + reason ∈ {expired, invalid_signature, invalid_aud, invalid_iss, invalid_nbf, malformed, revoked}. NEVER fall back to anonymous on validation failure. `usearch_auth_failures_total{reason}`.

### Identity Bridge Module

- REQ-AUTH1-006 (Ubiquitous, P0): extend `costguard.IdentityMiddleware` with source-priority: (a) context UserIDKey (JWT path) > (b) X-User-Id header (DEEP-004 V1 path) > (c) "anonymous". Schema unchanged. DEEP-004 existing tests MUST pass unchanged.
- REQ-AUTH1-007 (Optional, P1): WHERE tenant.mode=claim, extract from `claim_path` (e.g., "org_id") → TenantIDKey; fallback to default. WHERE tenant.mode=header → X-Tenant-Id. WHERE tenant.mode=static → default only.

### Anonymous Fallback / Mode Module

- REQ-AUTH1-008 (Ubiquitous, P0): hardcoded allowlist `/healthz`, `/metrics`, `/v1/auth/callback`, `/v1/auth/login`, GET `/v1/auth/logout` bypass auth middleware regardless of mode. Admin port mux has no auth chain attached. Allowlist logged at startup.

### Logout & Revocation Module

- REQ-AUTH1-009 (Event-Driven, P0): WHEN POST /v1/auth/logout with valid JWT → 302 + Location=end_session_endpoint (from discovery). WHERE revocation.enabled → SADD `auth:revoked:{jti}` + EXPIRE (exp-now). `usearch_auth_token_revoked_total{trigger}` counter.
- REQ-AUTH1-010 (Optional, P1): WHERE revocation.enabled, EXISTS check on `auth:revoked:{jti}` after validation. Revoked → 401 reason=revoked. Redis failure: failure_mode=fail-open (default) skip check; fail-closed → 401.

### SSRF & Security Module

- REQ-AUTH1-011 (Unwanted, P0): IF issuer URL not HTTPS OR host not in allowlist OR DNS resolves to private IP (RFC 1918 / fc00::/7 / loopback / link-local) → startup fatal exit. `auth.oidc.allow_private_issuer: true` bypasses private IP block only (HTTPS + allowlist still enforced).
- REQ-AUTH1-012 (Ubiquitous, P0): `/v1/auth/callback` registered with rate-limit (`auth.callback.rate_limit_per_minute`, default 60 per source IP, Redis sliding window with in-memory fallback). V1 returns 501 Not Implemented; full handler ships with SPEC-UI-001 (M7).

## Non-Functional Requirements

- NFR-AUTH1-001 JWT validation p95 ≤ 5ms (cache hit), p99 ≤ 10ms
- NFR-AUTH1-002 JWKS unknown-kid forced fetch p99 ≤ 300ms
- NFR-AUTH1-003 Startup discovery timeout 30s; fatal exit on timeout
- NFR-AUTH1-004 Clock skew tolerance default 30s for exp/nbf
- NFR-AUTH1-005 JWKS rotation resilience: client picks up new keys within library default cache window
- NFR-AUTH1-006 No PII/unbounded values in metric labels (outcome/reason/mode/trigger enum only)
- NFR-AUTH1-007 auth.yaml hot-reload via SIGHUP/fsnotify (mode, clock_skew, revocation.*, tenant.*); issuer/audience changes are startup-only with warning log
- NFR-AUTH1-008 Prometheus naming follows OBS-001 convention (`usearch_auth_*`)
- NFR-AUTH1-009 Production env (`USEARCH_ENV=production`) + mode != strict → startup WARN log + gauge emit

## Exclusions

- No RBAC / policy evaluation (delegated to SPEC-AUTH-002)
- No audit log subsystem (delegated to SPEC-AUTH-003; only stderr JSON line + forward-compat schema commitment)
- No OAuth2 authorization code flow handler (v1: 501; full impl ships with SPEC-UI-001 M7)
- No user signup / management (provider-native responsibility)
- No MFA / WebAuthn direct (provider responsibility, exposed via amr claim)
- No multi-org / B2B SSO (V1: single issuer + single audience list)
- No API key auth (bearer-only)
- No token introspection RPC (latency penalty avoided)
- No PKCE flow (frontend responsibility)
- No refresh token handling (client-side)
- No cost_ledger schema change (DEEP-004 forward-compat joint invariant)

## Acceptance Scenarios

- §5.1 valid JWT → 200, sub injected into costguard.UserIDKey (REQ-001, 003, 006)
- §5.2 permissive mode + missing header → anonymous, 200 (REQ-004)
- §5.3 JWKS rotation: new kid → forced fetch → pass (REQ-002, NFR-005)
- §5.4 expired JWT → 401 reason=expired; NEVER anonymous fallback on failure (REQ-005)
- §5.5 JWT sub takes precedence over X-User-Id header (anti-spoofing; REQ-003, 006)
- §5.6 tenant.mode=claim + JWT org_id → costguard.TenantIDKey (REQ-007)
- §5.7 strict + missing → 401; disabled → bypass; allowlist endpoint → bypass regardless (REQ-004, 008)
- §5.8 logout → 302 + end_session_endpoint URL; revocation enabled → Redis SADD; provider w/o end_session → 204 (REQ-009, 010)
- §5.9 SSRF block: http:// or private IP issuer → startup fatal; callback rate-limit 60/min (REQ-011, 012)
- Edge1 clock skew 25s future PASS; 35s future FAIL (REQ-003, NFR-004)
- Edge2 revocation enabled + Redis outage → fail-open default; fail-closed override → 401 (REQ-010)

## Files to Create

- internal/auth/middleware.go
- internal/auth/validator.go
- internal/auth/discovery.go
- internal/auth/config.go
- internal/auth/tenant.go
- internal/auth/revocation.go
- internal/auth/logout.go
- internal/auth/callback.go
- internal/auth/metrics.go
- internal/auth/types.go
- internal/auth/private_ip.go
- internal/auth/{*_test.go}
- internal/auth/oidc_integration_test.go
- internal/auth/testdata/oidc_stub/oidc_stub.go
- cmd/usearch-api/handlers/auth.go
- .moai/config/sections/auth.yaml

## Files to Modify

- cmd/usearch-api/main.go (chi router build-out + auth middleware wiring)
- internal/deepagent/costguard/middleware.go (additive patch: source-priority for UserIDKey)
- internal/obs/metrics/metrics.go (registerAuth)
- internal/obs/obs.go (re-export new collectors)
- internal/obs/metrics/metrics_test.go (extend TestNoUnboundedLabels allowlist with outcome, reason, mode, trigger)
- .env.example (AUTH_* env-vars)
- go.mod / go.sum (direct require coreos/go-oidc/v3 v3.18.0; promote golang-jwt/jwt/v5 to direct)

## Forward-Compat Joint Invariants

- DEEP-004 REQ-DEEP4-002: cost_ledger.user_id TEXT column schema UNCHANGED on V1→V1.1 transition
- DEEP-004 §6.3: AUTH-001 ship MUST inject sub into same context key `costguard.UserIDKey`
- DEEP-004 existing tests (TestIdentityMiddlewareReadsXUserId, TestIdentityMiddlewareDefaultsAnonymous) MUST pass unchanged
- AUTH-003 (planned M6): stderr JSON line schema (timestamp, event_type, request_id, subject_hash, outcome, issuer, audience, token_age_seconds) is additive-forward-compatible

---

*End of compact spec.*
