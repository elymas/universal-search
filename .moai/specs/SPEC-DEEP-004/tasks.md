## Task Decomposition
SPEC: SPEC-DEEP-004

| Task ID | Description | Requirement | Dependencies | Planned Files | Status |
|---------|-------------|-------------|--------------|---------------|--------|
| T-001 | Phase A: Postgres schema + Asynq reconcile | REQ-006, REQ-008, NFR-005, NFR-006 | - | deploy/postgres/migrations/0002_cost_ledger.sql, internal/deepagent/costguard/types.go, internal/deepagent/costguard/ledger.go, internal/deepagent/costguard/reconcile_job.go | pending |
| T-002 | Phase B: Redis cap-check Lua + atomic eval | REQ-009, NFR-004 | T-001 | internal/deepagent/costguard/cap_check.go, internal/deepagent/costguard/lua/cap_check.lua | pending |
| T-003 | Phase C: Haiku pre-screen + circuit breaker | REQ-003, REQ-004, REQ-005 | T-001 | internal/deepagent/costguard/haiku_screen.go | pending |
| T-004 | Phase D: LiteLLM cache key + metrics | REQ-012, REQ-013, NFR-003 | T-001 | internal/deepagent/costguard/cache_key.go, internal/deepagent/costguard/metrics.go, deploy/litellm/config.yaml | pending |
| T-005 | Phase E: Middleware chain + chi wiring | REQ-001, REQ-010, REQ-011, REQ-014 | T-001,T-002,T-003,T-004 | internal/deepagent/costguard/middleware.go, cmd/usearch-api/handlers/synthesis.go, cmd/usearch-api/main.go | pending |
| T-006 | Phase F: Decision log + OTel + hot-reload | REQ-010, NFR-007, NFR-008, NFR-010 | T-005 | internal/deepagent/costguard/config.go, internal/obs/metrics/metrics.go, internal/obs/obs.go | pending |
