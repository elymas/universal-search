## Task Decomposition
SPEC: SPEC-AUTH-002

| Task ID | Description | Requirement | Dependencies | Planned Files | Status |
|---------|-------------|-------------|--------------|---------------|--------|
| T-001 | Phase A: Casbin library pin + model/policy embed | REQ-001 (partial), REQ-002 (partial) | - | go.mod, internal/auth/rbac/types.go, internal/auth/rbac/model.conf, internal/auth/rbac/policy_default.csv | pending |
| T-002 | Phase B: Enforcer core (init, LoadPolicy, Enforce, thread-safety) | REQ-001, REQ-002, NFR-001, NFR-004 | T-001 | internal/auth/rbac/adapter.go, internal/auth/rbac/enforcer.go, deploy/postgres/migrations/0003_casbin_rules.sql | pending |
| T-003 | Phase C: Identity extraction + AUTH-001 forward-compat + empty team_id handling | REQ-003, REQ-004, REQ-006 (helper), NFR-005, NFR-006 | T-001 | internal/auth/rbac/context.go, internal/auth/rbac/middleware.go (TeamScopeMiddleware), .moai/config/sections/auth.yaml (rbac section) | pending |
| T-004 | Phase D: EnforceMiddleware + route mapping + IndexQuery wiring | REQ-005, REQ-006, NFR-001 | T-002, T-003 | internal/auth/rbac/routes.go, internal/auth/rbac/middleware.go (EnforceMiddleware), internal/auth/rbac/metrics.go (initial), cmd/usearch-api/handlers/synthesis.go (patch), cmd/usearch-api/handlers/deep.go (patch) | pending |
| T-005 | Phase E: Per-adapter visibility + admin operations | REQ-007, REQ-008, REQ-009, REQ-010 | T-002, T-004 | internal/adapters/visibility.go, internal/adapters/registry.go (patch), internal/auth/rbac/admin.go, cmd/usearch-api/main.go (admin route wiring) | pending |
| T-006 | Phase F: Observability + audit log + production hardening | REQ-011, NFR-003, NFR-006, NFR-007, NFR-008 | T-005 | internal/auth/rbac/audit.go, internal/auth/rbac/metrics.go (OTel span attach), internal/obs/metrics/metrics.go (registerRBAC), internal/obs/obs.go (re-export), internal/obs/metrics/metrics_test.go (whitelist), .env.example | pending |

## Dependencies Graph

```
T-001 ──┬──→ T-002 ──┐
        │           │
        └──→ T-003 ──┼──→ T-004 ──→ T-005 ──→ T-006
                    │
                    └─(T-004 also depends on T-002 for enforcer)
                       (T-005 also depends on T-002 for adapter registry path)
```

## Acceptance Criteria Mapping

| Task | Acceptance Scenarios |
|------|---------------------|
| T-001 | §5.1 (partial: model embed validation) |
| T-002 | §5.1 (full: enforcer init + Enforce), §5.4 (deny-by-default + role hierarchy), Edge1 (concurrency) |
| T-003 | §5.2 (AUTH-001 JWT context + header fallback), §5.3 (empty team_id default fallback + 400) |
| T-004 | §5.5 (EnforceMiddleware 403 on deny), §5.6 (IndexQuery TeamID wiring activates IDX-001 reservation) |
| T-005 | §5.7 (per-adapter visibility: team_shared / personal V1.1 reserve), §5.8 (admin reload + member CRUD), Edge2 (LoadPolicy atomic replace) |
| T-006 | §5.9 (decision audit 3-surface emit: Prometheus + OTel + stderr JSON), (cross-cutting: cardinality whitelist, hot-reload) |
