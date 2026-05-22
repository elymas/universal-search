## Task Decomposition
SPEC: SPEC-AUTH-003

| Task ID | Description | Requirement | Dependencies | Planned Files | Status |
|---------|-------------|-------------|--------------|---------------|--------|
| T-001 | Phase A: audit_events schema + partition + append-only triggers + role separation | REQ-001, REQ-008, NFR-001 | - | deploy/postgres/migrations/0003_audit_events.sql, internal/audit/types.go, internal/audit/partitions.go | pending |
| T-002 | Phase B: EmitEvent emitter + DEEP-004 decision log tee handler | REQ-002, REQ-009, NFR-002, NFR-008 | T-001 | internal/audit/store.go, internal/audit/decision_log_handler.go, internal/obs/log/handler.go, internal/audit/metrics.go | pending |
| T-003 | Phase C: cost_ledger AFTER INSERT trigger + index.write audit hook | REQ-002 | T-001, T-002 | deploy/postgres/migrations/0004_audit_cost_ledger_trigger.sql, internal/index/index.go, internal/audit/config.go | pending |
| T-004 | Phase D: LiteLLM /spend/logs reconciliation job + dedup + metrics | REQ-003, REQ-009, NFR-003, NFR-005 | T-002 | internal/audit/litellm_reconcile.go, internal/audit/litellm_client.go | pending |
| T-005 | Phase E: /admin/audit/replay endpoint + RBAC + rate limit + actor propagation | REQ-004, REQ-006, REQ-009 | T-002 | internal/audit/replay.go, cmd/usearch-api/handlers/admin_audit.go, cmd/usearch-api/main.go | pending |
| T-006 | Phase F: S3 export + retention cleanup + PII masking + optional hash chain | REQ-005, REQ-006, REQ-007, REQ-008, NFR-004, NFR-007 | T-002, T-005 | internal/audit/export.go, internal/audit/cleanup.go, internal/audit/chain.go, .moai/config/sections/audit.yaml, deploy/docker-compose.yml (MinIO) | pending |
