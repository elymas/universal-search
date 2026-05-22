## Task Decomposition
SPEC: SPEC-AUTH-001

| Task ID | Description | Requirement | Dependencies | Planned Files | Status |
|---------|-------------|-------------|--------------|---------------|--------|
| T-001 | Phase A: In-process OIDC stub + Discovery module | REQ-001, NFR-003 | - | internal/auth/testdata/oidc_stub/oidc_stub.go, internal/auth/types.go, internal/auth/discovery.go, internal/auth/config.go (skeleton) | pending |
| T-002 | Phase B: SSRF protection + Issuer validation | REQ-011 | T-001 | internal/auth/discovery.go (extend), internal/auth/private_ip.go | pending |
| T-003 | Phase C: JWT validation core + Forward-compat with DEEP-004 | REQ-003, REQ-005, REQ-006, NFR-001, NFR-004 | T-001 | internal/auth/validator.go, internal/auth/middleware.go (initial), internal/deepagent/costguard/middleware.go (additive patch) | pending |
| T-004 | Phase D: Mode + Tenant + JWKS rotation | REQ-002, REQ-004, REQ-007, NFR-005, NFR-007, NFR-008 | T-003 | internal/auth/config.go (complete), internal/auth/tenant.go, internal/auth/middleware.go (mode branches), internal/auth/metrics.go, internal/obs/metrics/metrics.go (registerAuth) | pending |
| T-005 | Phase E: Logout + Revocation + Callback + chi wiring | REQ-008, REQ-009, REQ-010, REQ-012, NFR-002 | T-001, T-003, T-004 | internal/auth/logout.go, internal/auth/revocation.go, internal/auth/callback.go, cmd/usearch-api/handlers/auth.go, cmd/usearch-api/main.go | pending |
| T-006 | Phase F: Production hardening + Observability | NFR-006, NFR-008, NFR-009 | T-005 | internal/auth/middleware.go (OTel attrs), internal/auth/config.go (hot-reload), internal/obs/metrics/metrics_test.go (allowlist), .moai/config/sections/auth.yaml, .env.example | pending |

## Dependencies Graph

```
T-001 ──┬──→ T-002 ──┐
        │           │
        └──→ T-003 ──┼──→ T-004 ──→ T-005 ──→ T-006
                    │
                    └─(T-005 also depends on T-001 for stub)
```

## Acceptance Criteria Mapping

| Task | Acceptance Scenarios |
|------|---------------------|
| T-001 | §5.1 (partial: discovery setup) |
| T-002 | §5.9 SSRF block |
| T-003 | §5.1 (full: valid JWT path), §5.4 (expired), §5.5 (forward-compat with DEEP-004), Edge1 (clock skew) |
| T-004 | §5.2 (permissive anonymous), §5.3 (JWKS rotation), §5.6 (tenant claim mode), §5.7 (mode branches) |
| T-005 | §5.7 (allowlist bypass), §5.8 (logout + revocation), §5.9 (callback rate-limit) |
| T-006 | (cross-cutting: production warning, OTel, hot-reload, metric whitelist) |
