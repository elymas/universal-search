# SPEC-AUTH-003 (Compact)

id: SPEC-AUTH-003 | v0.1.0 | draft | owner: expert-security | TDD | coverage 85% | priority P0
depends_on: AUTH-001, AUTH-002, DEEP-004, OBS-001
title: Audit log — immutable trail, query replay, S3 archive, LiteLLM cost reconciliation

## Pinned Decisions

- D1 Event taxonomy: 11 categories × ~20 event_types (auth/rbac/query/deep/cost/index/admin/system). startup enum lock.
- D2 cost_ledger relationship: cross-reference only via `payload.cost_ledger_id`. DEEP-004 schema unchanged.
- D3 Python LiteLLM cost: 5-min Asynq polling of `/spend/logs`. audit_events only. NOT cost_ledger. NOT cap-relevant.
- D4 Index events: V1 all-in + `audit.events.index_write_enabled` toggle. Sampling deferred to M8.
- D5 cost_ledger mirror: Postgres AFTER INSERT trigger. DEEP-004 Go code unchanged.
- D6 Hash chain: optional, default OFF. Advisory lock per-tenant when enabled. Daily verify job.
- D7 S3 export: weekly JSONL.gz partition-level. MinIO + AWS S3 compatible.

## EARS Requirements

### Schema Module

- REQ-AUTH3-001 (Ubiquitous, P0): `deploy/postgres/migrations/0003_audit_events.sql` creates `audit_events` table with monthly partitioning (PARTITION BY RANGE ts), BEFORE UPDATE/DELETE triggers raise EXCEPTION, role separation (audit_writer SELECT/INSERT only; audit_admin for DROP PARTITION).

### Emission Module

- REQ-AUTH3-002 (Ubiquitous, P0): single `EmitEvent(ctx, AuditEvent)` emitter funnels all audit writes. Integration points: AUTH-001 (login/logout/fail/token), AUTH-002 (rbac.allow/deny/policy_change), synthesis handler (query.submit/complete/fail, deep.start/complete/fail), DEEP-004 decision log (slog tee, no DEEP-004 code change), cost_ledger trigger (cost.recorded auto-mirror), Python reconcile (cost.reconciled), index hook (index.write/delete), admin handler (admin.replay/config_change). DEEP-004 emit code SHALL NOT change.

### Python LiteLLM Reconciliation Module

- REQ-AUTH3-003 (Event-Driven, P0): WHEN Asynq job `audit.litellm_reconcile` fires (default 5min), poll LiteLLM `GET /spend/logs?summarize=false`; dedup against (a) cost_ledger.request_id, (b) audit_events.payload.litellm_request_id; INSERT remaining as `event_type="cost.reconciled"`, `source="python"`. cost_ledger SHALL NOT be mutated.

### Query Replay Module

- REQ-AUTH3-004 (Event-Driven, P0): WHEN `POST /admin/audit/replay` invoked: JWT (401), `audit.replay` RBAC (403), rate limit 1/min (429), fetch event (400 if unknown or non-replayable), re-execute with admin actor identity (NOT original user). Emit `admin.replay` event with cross-reference.

### S3 Export Module

- REQ-AUTH3-005 (Optional, P1): WHERE `audit.s3.enabled: true`, weekly Asynq job `audit.export` (cron `0 2 * * 0`) streams partitions older than 7d to S3 as JSONL.gz. MinIO and AWS S3 supported via aws-sdk-go-v2 single client. Retry 3x exponential backoff on failure. Emit `audit.export` event.

### PII Masking Module

- REQ-AUTH3-006 (Optional, P1): WHERE `audit.pii.mask_query_text: true`, replace `payload.query.text` with `payload.query.text_sha256` for query.submit/complete/deep.start/deep.complete events. Identity fields (user_id, tenant_id, request_id) NEVER masked. Masked events reject replay with 400 `query_text_masked`. `audit.pii.mask_ip: true` nullifies `ip` column.

### Retention Module

- REQ-AUTH3-007 (Ubiquitous, P0): nightly Asynq job `audit.cleanup` (cron `30 3 * * *`) drops partitions older than `audit.retention.hot_days` (default 90). `require_s3_archive: true` (default) requires `archived_at IS NOT NULL`. DROP via `audit_admin` role. Emit `audit.partition_drop` event.

### Tamper Resistance Module

- REQ-AUTH3-008 (Unwanted, P0): IF application connection attempts UPDATE/DELETE/TRUNCATE → Postgres triggers RAISE EXCEPTION + DB role `audit_writer` has only SELECT/INSERT grants. WHERE `audit.hash_chain.enabled: true`, INSERT acquires `pg_advisory_xact_lock(hashtext(tenant_id))`, computes `this_hash = SHA256(prev_hash || canonical_json(row_minus_hashes))`. Daily `audit.chain_verify` job detects violations.

### Observability Module

- REQ-AUTH3-009 (Ubiquitous, P0): new Prometheus metrics (all `usearch_audit_*`): events_total{event_type, decision, source}, write_duration_seconds, lag_seconds{source}, s3_export_duration_seconds, s3_export_rows_total, s3_export_bytes_total, chain_violations_total, reconcile_polls_total{outcome}, reconcile_lag_seconds, partition_drop_total, replay_requests_total{outcome}. SPEC-OBS-001 `TestNoUnboundedLabels` whitelist extended with event_type, decision, source, outcome labels.

## Non-Functional Requirements

- NFR-AUTH3-001 Append-only invariant: app connection UPDATE/DELETE/TRUNCATE blocked by trigger + GRANT
- NFR-AUTH3-002 Audit write latency: async p95 ≤ 50ms (Asynq enqueue), sync p95 ≤ 30ms
- NFR-AUTH3-003 Reconciliation freshness: lag ≤ 6min average (5min poll + 1min processing)
- NFR-AUTH3-004 S3 export throughput: single partition (200K rows / 300MB) ≤ 5min
- NFR-AUTH3-005 Reconciliation drift: cumulative ≤ 0.5% over 7d
- NFR-AUTH3-006 No PII in metric labels; bounded enums only
- NFR-AUTH3-007 Hash chain verify: 90d retention (600K-2M rows) ≤ 30min daily
- NFR-AUTH3-008 Cardinality safety: event_type/decision/source/outcome are startup-locked enums

## Exclusions

- No JWT issuance / OIDC SSO implementation (delegated to SPEC-AUTH-001 M6)
- No RBAC policy definition (delegated to SPEC-AUTH-002 M6)
- No real-time SIEM integration (Splunk/ELK/Datadog) — S3 JSONL.gz is the batch interop path
- No cost-aware billing integration (post-V1 SPEC-BILLING-001)
- No Python-side cost in cost_ledger or cap eval (DEEP-004 forward-compat; SPEC-AUTH-004 post-V1 may revisit)
- No cost_ledger schema modification or migration (DEEP-004 forward-compat hard rule)
- No original user identity reuse in replay (admin actor identity only)
- No multi-region S3 replication (deploy-config concern)
- No ML-based audit analytics (post-V1 SPEC-EVAL-AUDIT-001 M9)
- No GDPR right-to-be-forgotten automation (post-V1 SPEC-COMPLIANCE-001)
- No DLP for audit data egress (operational control)

## Acceptance Scenarios

- §5.1 user login → audit_events row (auth.login, decision=allow) (REQ-001, 002, 009)
- §5.2 RBAC deny → audit_events row (rbac.deny, decision=deny) (REQ-002)
- §5.3 Python LiteLLM call → 5-min reconciliation → cost.reconciled in audit_events only; cost_ledger unchanged (REQ-003, NFR-003, NFR-005)
- §5.4 admin replays past query → new query.submit + admin.replay events, actor identity propagation (REQ-004, 009)
- §5.5 non-admin replay attempt → 403, admin.replay decision=deny (REQ-004)
- §5.6 weekly S3 export → JSONL.gz upload, archived_at set; cleanup drops only archived partitions (REQ-005, 007, 008, NFR-001, NFR-004)
- §5.7 PII masking on → query.text → text_sha256, replay returns 400 query_text_masked (REQ-006)
- §5.8 DEEP-004 cap.evaluation stderr mirrored to audit_events; DEEP-004 code/cost_ledger schema unchanged (REQ-002, §1.3 forward-compat)
- Edge hash_chain enabled, 100 concurrent INSERTs → advisory lock serializes, prev_hash correct, verify passes (REQ-008, NFR-007)

## Files to Create

- internal/audit/store.go
- internal/audit/types.go
- internal/audit/decision_log_handler.go
- internal/audit/litellm_reconcile.go
- internal/audit/litellm_client.go
- internal/audit/replay.go
- internal/audit/export.go
- internal/audit/cleanup.go
- internal/audit/chain.go
- internal/audit/partitions.go
- internal/audit/config.go
- internal/audit/metrics.go
- internal/audit/{*_test.go}
- cmd/usearch-api/handlers/admin_audit.go
- deploy/postgres/migrations/0003_audit_events.sql
- deploy/postgres/migrations/0004_audit_cost_ledger_trigger.sql
- .moai/config/sections/audit.yaml

## Files to Modify

- cmd/usearch-api/handlers/synthesis.go (query/deep event emission hooks)
- cmd/usearch-api/main.go (audit init + 4 scheduled jobs + admin routes)
- internal/obs/metrics/metrics.go (registerAudit)
- internal/obs/obs.go (re-export collectors)
- internal/obs/metrics/metrics_test.go (extend TestNoUnboundedLabels with event_type, decision, source, outcome)
- internal/obs/log/handler.go (audit tee handler in slog chain)
- internal/index/index.go (Upsert/Delete audit hook wrapper, IDX-001 surface unchanged)
- .env.example (AUDIT_S3_*, AUDIT_HASH_CHAIN_ENABLED, etc.)
- deploy/docker-compose.yml (add MinIO service for local S3-compatible storage)

## DEEP-004 Forward-Compat Honor (unchanged)

- `internal/deepagent/costguard/middleware.go::emitDecisionEvent` — SINGLE LINE unchanged
- `internal/deepagent/costguard/ledger.go::WriteLedgerEntry` — SINGLE LINE unchanged
- `internal/deepagent/costguard/` package — all files unchanged
- `deploy/postgres/migrations/0002_cost_ledger.sql` — schema unchanged
- DEEP-004 6 mandatory JSON line fields (timestamp/event_type/request_id/tenant_id/user_id/decision) → 1:1 mapped to audit_events columns

## AUTH-002 Permission Additions (cross-spec dependency)

Required before AUTH-003 ships (AUTH-002 amendment):

- `audit.read.team` — team owner reads own team's events
- `audit.read.global` — admin reads all events
- `audit.replay` — admin re-executes past queries
- `audit.export.trigger` — admin triggers S3 export
- `audit.partition.drop` — superadmin drops partitions

---

*End of compact spec.*
