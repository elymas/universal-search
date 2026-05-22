# SPEC-AUTH-002 (Compact)

id: SPEC-AUTH-002 | v0.1.0 | draft | owner: expert-security | TDD | coverage 85% | priority P0
depends_on: AUTH-001, IDX-001, OBS-001
blocks: IDX-004, IDX-005, AUTH-003
title: Team RBAC (Casbin RBAC-with-domains, team-scoped queries, per-adapter visibility)

## Pinned Decisions

- D1 Casbin major version: `github.com/casbin/casbin/v2` latest stable (v2.103.x). v3 snapshot rejected (operational risk). OPA / custom impl rejected (fit-for-purpose).
- D2 Policy persistence: `github.com/casbin/casbin-pg-adapter v1.5.0` + isolated `*pg.DB` connection (separated from hot-path pgxpool). gorm-adapter rejected (no GORM in stack).
- D3 Empty team_id handling: `auth.rbac.default_team_id` (default `"default"`) fallback. Operator switches to multi-team enforce via `default_team_id: ""` → HTTP 400.
- D4 Role hierarchy (observer < member < admin): policy row duplication — admin holds explicit superset of member rows. Custom matcher / AddRole expansion rejected.
- D5 Policy hot reload: Admin `POST /admin/rbac/reload` endpoint only. fsnotify / PG NOTIFY-LISTEN deferred to Open Questions.

## EARS Requirements

### Enforcer Core Module

- REQ-AUTH2-001 (Ubiquitous, P0): `casbin.NewEnforcer(model.conf, pgadapter)` at startup. RBAC-with-domains 4-tuple model (`r = sub, dom, obj, act`; `g = _, _, _`). `//go:embed model.conf`. Init failure → fatal exit (unless `auth.rbac.enabled: false`). Singleton enforcer shared by all middleware/admin handlers.
- REQ-AUTH2-002 (Ubiquitous, P0): deny-by-default. `policy_effect = some(allow) && !some(deny)`. catch-all `p, *, *, *, *, deny` row in policy_default.csv. Thread-safe via casbin v2 internal RWMutex.

### Identity Extraction Module

- REQ-AUTH2-003 (Event-Driven, P0): WHEN request enters protected route, `rbac.TeamScopeMiddleware` extracts (user_id, team_id, roles) priority: (1) AUTH-001 JWT context (`costguard.UserIDKey` + `auth.TeamIDKey` + `auth.RolesKey`); (2) header fallback (`X-User-Id` / `X-Team-Id` / `X-Roles` comma-separated); (3) anonymous fallback.
- REQ-AUTH2-004 (Optional, P1): WHERE team_id empty AND `default_team_id` non-empty → use default. WHERE both empty → HTTP 400 `{"error":"team_id_required"}`. Counter `usearch_rbac_decisions_total{result="allow",reason_class="empty_team"}` on fallback.

### Middleware Chain Module

- REQ-AUTH2-005 (Event-Driven, P0): `rbac.EnforceMiddleware(resource, action)` reads extracted identity, calls `enforcer.Enforce(user, team, resource, action)`, allow → next.ServeHTTP, deny → HTTP 403 `{"error":"forbidden","resource":"...","action":"..."}`. Route-resource mapping in `internal/auth/rbac/routes.go` (table-driven, 11 entries — query:basic, query:deep, audit_log, rbac_policy, member, api_key, adapter_config, adapter:NAME).
- REQ-AUTH2-006 (Ubiquitous, P0): query handlers (`synthesis.go`, `deep.go`) inject `q.TeamID = rbac.TeamIDFromContext(ctx)` into IndexQuery, activating IDX-001 reserved `team_id` filter across all 3 stores (Qdrant payload, Meili `team_id = "..."`, PG WHERE). Single-line change per handler; IDX-004 follows up with NOT NULL flip + RLS.

### Per-Adapter Visibility Module

- REQ-AUTH2-007 (Ubiquitous, P0): `internal/adapters/registry.go` adds `Visibility AdapterVisibility` field, enum (`team_shared`, `personal`, `admin_only`). All 12 V1 adapters (reddit, hn, arxiv, github, youtube, bluesky, x, naver, daum, koreannewscrawler, searxng, rss) = `team_shared`. Fanout calls `enforce(uid, tid, "adapter:NAME", "read")` per adapter; deny skips adapter (not entire request).
- REQ-AUTH2-008 (Optional, P1): V1.1 `AdapterVisibility.Personal` adapters use policy shape `p, <owner>, <team>, adapter:<name>:<owner>, read, allow`. V1 emits no personal rows (deny-by-default). Enum + helper reserved; activation deferred to SPEC-AUTH-005.

### Admin Operations Module

- REQ-AUTH2-009 (Event-Driven, P0): WHEN admin calls `POST /admin/rbac/reload` → `enforcer.LoadPolicy()` → HTTP 200 + `{"status":"reloaded","policy_count":N}`. Counter `usearch_rbac_policy_reload_total{outcome=success|failure}`. Failure preserves existing in-memory enforcer (atomic replace).
- REQ-AUTH2-010 (Event-Driven, P0): WHEN admin calls member endpoints: (a) `POST /admin/members` body `{user_id, team_id, role}` → AddRoleForUserInDomain + SavePolicy → 201; (b) `DELETE /admin/members?...` → DeleteRoleForUserInDomain → 204; (c) `GET /admin/members?team_id=...` → list. Invalid role (not observer/member/admin) → 400.

### Observability Module

- REQ-AUTH2-011 (Ubiquitous, P0): Every Enforce decision emits 3 surfaces: (a) `usearch_rbac_decisions_total{result,reason_class}` counter (result ∈ {allow,deny}, reason_class ∈ {policy_matched, no_policy_matched, explicit_deny, empty_team}); (b) OTel span `rbac.evaluate` with attributes (user_id, team_id, resource, action, decision, eval_duration_ms); (c) stderr JSON line audit event (schema forward-compat with SPEC-AUTH-003).

## Non-Functional Requirements

- NFR-AUTH2-001 Enforce eval latency p95 ≤ 1ms, p99 ≤ 3ms (in-memory, role_count ≤ 10)
- NFR-AUTH2-002 LoadPolicy p99 ≤ 200ms (< 100 policy rows)
- NFR-AUTH2-003 No PII/unbounded values in metric labels (result/reason_class/outcome enum only); high-cardinality goes to span attrs + stderr audit
- NFR-AUTH2-004 Policy storage connection isolated from hot-path pgxpool (separate `*pg.DB`)
- NFR-AUTH2-005 Forward-compat with AUTH-001 context keys (additive only; costguard.UserIDKey/TenantIDKey semantics unchanged)
- NFR-AUTH2-006 auth.yaml hot-reload via SIGHUP/fsnotify (default_team_id, audit_to_stderr, enabled); pg_dsn is startup-only with warning
- NFR-AUTH2-007 Prometheus naming follows OBS-001 convention (`usearch_rbac_*`)
- NFR-AUTH2-008 stderr/stdout separation for audit log (DEEP-004 REQ-DEEP4-010 pattern); slog stdout, audit stderr

## Exclusions

- No JWT validation / OIDC discovery (delegated to SPEC-AUTH-001; AUTH-002 only consumes injected context keys)
- No audit log persistence (delegated to SPEC-AUTH-003; only stderr JSON line + forward-compat schema commitment)
- No API key rotation implementation (v1: authz gate + 501; full impl ships with SPEC-AUTH-004 M7+)
- No adapter enablement toggle implementation (v1: authz + 501; full impl ships with SPEC-IDX-004 M6)
- No personal adapter activation (V1.1: SPEC-AUTH-005 with OAuth + per-user policy generation)
- No PG row-level security (delegated to SPEC-IDX-004)
- No Python sidecar RBAC (Go-side ingress only; internal RPC trust for V1)
- No multi-team active context (one team_id per request; multi-team UI deferred to SPEC-UI-001 V1.1)
- No Casbin v3 migration (v3 snapshot risk; v2 line pinned; SPEC-AUTH-002-UPGRADE-001 follow-up)
- No self-service team management (admin role only; B2B SSO scenario deferred to V2)
- No role hierarchy auto-expansion (static policy row duplication for V1; revisit if role count > 5)
- No cost_ledger schema change (DEEP-004 cost guard = user-level, AUTH-002 RBAC = team-level, kept orthogonal)

## Acceptance Scenarios

- §5.1 enforcer init + valid policy → member can read query:basic (REQ-001, 002)
- §5.2 AUTH-001 JWT context precedes header; header fallback works when context absent (REQ-003)
- §5.3 empty team_id + default fallback → "default" scope; default blank → 400 (REQ-004)
- §5.4 observer attempts team_index write → 403 (deny-by-default + role hierarchy) (REQ-002, 005)
- §5.5 EnforceMiddleware: observer → 403 on /admin/audit; admin → 200 (REQ-005)
- §5.6 query handler injects IndexQuery.TeamID → IDX-001 3-store filter activates (REQ-006)
- §5.7 reddit (team_shared) member accessible; gmail (personal V1.1) denied in V1 (REQ-007, 008)
- §5.8 admin reload + member add/remove/list endpoints (REQ-009, 010)
- §5.9 every decision emits Prometheus + OTel + stderr JSON line (REQ-011)
- Edge1 1000 concurrent Enforce → race-free, p95 < 1ms (REQ-002, NFR-001)
- Edge2 LoadPolicy failure → existing in-memory enforcer preserved (REQ-009)

## Files to Create

- internal/auth/rbac/enforcer.go
- internal/auth/rbac/adapter.go
- internal/auth/rbac/middleware.go
- internal/auth/rbac/context.go
- internal/auth/rbac/routes.go
- internal/auth/rbac/admin.go
- internal/auth/rbac/audit.go
- internal/auth/rbac/metrics.go
- internal/auth/rbac/types.go
- internal/auth/rbac/model.conf
- internal/auth/rbac/policy_default.csv
- internal/auth/rbac/{*_test.go}
- internal/auth/rbac/integration_test.go
- internal/adapters/visibility.go
- internal/adapters/visibility_test.go
- deploy/postgres/migrations/0003_casbin_rules.sql
- .moai/config/sections/auth.yaml (rbac section)

## Files to Modify

- cmd/usearch-api/main.go (enforcer init + admin route wiring + middleware chain wire)
- cmd/usearch-api/handlers/synthesis.go (EnforceMiddleware wrap + IndexQuery.TeamID inject)
- cmd/usearch-api/handlers/deep.go (same pattern; resource=query:deep)
- internal/index/dispatch.go (comment-only: team_id source note)
- internal/adapters/registry.go (Visibility field default = team_shared)
- internal/obs/metrics/metrics.go (registerRBAC)
- internal/obs/obs.go (re-export new collectors)
- internal/obs/metrics/metrics_test.go (extend TestNoUnboundedLabels allowlist with result, reason_class, outcome)
- .env.example (AUTH_RBAC_* env-vars)
- go.mod / go.sum (direct require casbin/v2 v2.103.x + casbin-pg-adapter v1.5.0)

## Forward-Compat Joint Invariants

- AUTH-001 NFR-AUTH2-005: costguard.UserIDKey + costguard.TenantIDKey + auth.ClaimsKey semantics UNCHANGED; AUTH-002 adds only auth.TeamIDKey + auth.RolesKey (additive)
- IDX-001 schema unchanged: team_id TEXT NULL column + Qdrant payload + Meili filterable attribute reservation activated, no migration added by AUTH-002 (separate 0003_casbin_rules.sql is for policy storage, unrelated to IDX-001 schema)
- DEEP-004 cost_ledger schema unchanged: user-level cost guard orthogonal to team-level RBAC
- AUTH-003 forward-compat: stderr JSON line schema (timestamp, event_type=rbac.decision, request_id, tenant_id, team_id, user_id, decision, resource, action, reason) is additive-forward-compatible

---

*End of compact spec.*
