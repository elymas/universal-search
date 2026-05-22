# SPEC-AUTH-003 Implementation Plan

Generated: 2026-05-22
Methodology: TDD (RED-GREEN-REFACTOR)
Coverage target: 85%
Harness: standard

---

## 1. Overview

본 plan.md는 SPEC-AUTH-003의 구현 단계별 task sequence를 정의한다. 9 EARS REQs +
8 NFRs를 6개 phase로 분해하며, 각 phase는 RED → GREEN → REFACTOR 사이클을 따른다.
plan-auditor 통과 + annotation cycle 완료 후 본 plan은 manager-tdd 에이전트에게
전달되어 phase-별로 진행한다.

전체 design 원칙: **DEEP-004와 cost_ledger를 변경하지 않는 additive 통합**. AUTH-003
의 모든 변경은 audit_events 테이블, audit handler chain tee, PG trigger, Asynq job,
admin endpoint로 한정된다. DEEP-004 Go 코드는 single-line 변경 없이 본 SPEC을 마칠
수 있어야 한다.

---

## 2. Phase Breakdown

### Phase A — Schema + Role Separation Foundation

목표: append-only audit_events 테이블, monthly partition, role-based write 보호.
DEEP-004 cost_ledger와 cross-reference 가능한 schema를 먼저 고정한다.

순서 근거: 모든 emission/reconciliation/replay/export 코드가 이 schema에 의존.

**RED tests** (6):

1. `TestMigration0003Idempotent` — `0003_audit_events.sql` 두 번 실행 = 같은 schema
   (REQ-001).
2. `TestAuditEventsSchemaMatchesSpec` — pgx로 information_schema 쿼리, 컬럼/타입/
   제약 검증 (REQ-001).
3. `TestAuditEventsPartitioned` — `audit_events`가 `PARTITION BY RANGE (ts)`,
   현재 월 partition 자동 생성 (REQ-001).
4. `TestAuditEventsBlocksUpdate` — application connection으로 UPDATE 시 RAISE
   EXCEPTION (REQ-001, REQ-008).
5. `TestAuditEventsBlocksDelete` — application connection으로 DELETE 시 RAISE
   EXCEPTION (REQ-001, REQ-008).
6. `TestAuditAdminRoleSeparation` — `audit_writer` role은 SELECT/INSERT만,
   `audit_admin` role만 DROP PARTITION (REQ-008, NFR-001).

**GREEN tasks**:

- `deploy/postgres/migrations/0003_audit_events.sql` 작성:
  - `CREATE TABLE audit_events ... PARTITION BY RANGE (ts)`
  - `CREATE TABLE audit_events_y2026m05 PARTITION OF audit_events FOR VALUES FROM (...) TO (...)`
  - `CREATE TRIGGER audit_events_no_update/no_delete BEFORE UPDATE/DELETE ... RAISE EXCEPTION`
  - `CREATE ROLE audit_writer` + `GRANT SELECT, INSERT ON audit_events TO audit_writer`
  - `CREATE ROLE audit_admin` + `GRANT ALL ON audit_events TO audit_admin`
- `internal/audit/types.go` — `AuditEvent`, `EventType` enum, `Decision` enum.
- `internal/audit/partitions.go` — monthly partition lifecycle helpers.

**REFACTOR**:

- partition creation을 startup time에 자동 호출하는 helper(`EnsureCurrentPartition`).
- partition list 조회 함수(`ListPartitions(ctx)`).

---

### Phase B — Audit Emitter + DEEP-004 Tee Handler

목표: 단일 `EmitEvent` API와 DEEP-004 stderr JSON line tee handler. DEEP-004 코드를
변경하지 않는 additive 통합을 검증한다.

순서 근거: Phase A의 schema가 있어야 emit 가능. 다른 emit point들은 이 emitter를 재사용.

**RED tests** (7):

7. `TestEmitEventInsertsRow` — `EmitEvent(ctx, AuditEvent{...})` 호출 → audit_events
   에 1 row 추가, INSERT 컬럼 매핑 정확성 검증 (REQ-002).
8. `TestEmitEventNoPanicOnNilCtx` — nil context에서도 panic 없이 error 반환 (REQ-002).
9. `TestEmitEventAsyncEnqueuesAsynq` — `audit.async: true`(기본)에서 Asynq enqueue
   확인 + actual INSERT는 worker가 수행 (REQ-002, NFR-002).
10. `TestEmitEventSyncInsertsDirectly` — `audit.async: false`에서 동기 INSERT,
    p95 ≤ 30ms 검증 (REQ-002, NFR-002).
11. `TestDeep004DecisionLogMirroredToAudit` — DEEP-004 stderr JSON line emit →
    audit_events에 `event_type="cap.evaluation"` 1 row 추가, payload에 dimension/
    remaining/screen_score/cache_hit 포함 (REQ-002).
12. `TestEmitEventDoesNotMutateDeep004Emit` — DEEP-004 emit 호출 횟수와 audit
    handler invocation은 1:1, DEEP-004 emit code path는 변경되지 않음 (REQ-002,
    §1.3 forward-compat).
13. `TestEventTypeBoundedEnum` — 정의되지 않은 event_type emit 시도 → error 또는
    "unknown" collapse (NFR-008).

**GREEN tasks**:

- `internal/audit/store.go::EmitEvent(ctx, AuditEvent) error`.
  - `audit.async`이면 Asynq enqueue.
  - 아니면 pgx로 직접 INSERT (audit_writer role connection).
- `internal/audit/decision_log_handler.go` — slog handler. 기존 stderr handler
  chain에 tee 형태로 추가.
- `internal/obs/log/handler.go` 수정 — handler chain에 audit tee handler 추가.
  audit handler 실패는 stderr write를 abort하지 SHALL NOT (graceful degradation).

**REFACTOR**:

- AuditEvent 빌더 패턴(`NewAuditEvent(eventType).WithUser(...).WithDecision(...)`).
- emit point들의 boilerplate 축소.

---

### Phase C — cost_ledger Trigger + Index Write Hook

목표: DEEP-004 cost_ledger row 자동 mirror, index write event 발신.

**RED tests** (5):

14. `TestCostLedgerTriggerMirrorsToAudit` — cost_ledger INSERT → audit_events에
    `event_type="cost.recorded"` 1 row 추가, payload.cost_ledger_id가 NEW.id로
    설정 (REQ-002, §1.3).
15. `TestCostLedgerTriggerRollsBackOnAuditFailure` — `audit.cost_mirror_strict:
    true`(기본)에서 audit INSERT 실패 시 cost_ledger INSERT abort (REQ-002).
16. `TestCostLedgerTriggerFailOpenWhenStrictFalse` — `audit.cost_mirror_strict:
    false`에서 audit INSERT 실패 시 cost_ledger INSERT는 성공, audit emit error
    로깅 (REQ-002).
17. `TestIndexWriteEmitsAuditEvent` — `Index.Upsert` 호출 → audit_events에
    `event_type="index.write"` row 추가, payload에 store_outcomes 포함 (REQ-002).
18. `TestIndexWriteToggleControlsEmission` — `audit.events.index_write_enabled:
    false`에서 index.write event는 emit되지 않음 (REQ-002, D4).

**GREEN tasks**:

- `deploy/postgres/migrations/0004_audit_cost_ledger_trigger.sql` — `CREATE TRIGGER
  cost_ledger_to_audit AFTER INSERT ON cost_ledger ...`.
- `internal/index/index.go` 수정 — `Index.Upsert` 끝에 audit emit wrapper. IDX-001
  Surface 변경 없도록 internal helper 추가.
- `internal/audit/config.go` — `audit.events.index_write_enabled` toggle 로딩.

**REFACTOR**:

- trigger 동작을 unit test에서 검증하기 위한 PG testcontainer fixture 정리.

---

### Phase D — LiteLLM Reconciliation Job

목표: 5분 주기 Asynq job이 LiteLLM `/spend/logs`를 polling하여 Python-side cost
회수. cost_ledger 변경 없음.

**RED tests** (6):

19. `TestReconcileFetchesSpendLogs` — Asynq job 발화 → mock LiteLLM 서버에 `GET
    /spend/logs` 요청 도착, start_date/end_date 정확 (REQ-003).
20. `TestReconcileDedupByRequestIdAgainstCostLedger` — cost_ledger에 이미 동일
    request_id가 있는 spend log row는 skip (REQ-003).
21. `TestReconcileDedupByLitellmRequestIdAgainstAuditEvents` — audit_events에
    이미 동일 litellm_request_id row가 있으면 skip (idempotent) (REQ-003).
22. `TestReconcileDoesNotMutateCostLedger` — reconciliation 종료 후 cost_ledger
    row 수 변동 0 (REQ-003, §1.3 forward-compat).
23. `TestReconcileEmitsErrorCounterOnLitellmFailure` — LiteLLM 응답 5xx →
    `usearch_audit_reconcile_polls_total{outcome="error"}` 1 증가 (REQ-003, REQ-009).
24. `TestReconcileLagGaugeEmitted` — job 종료 시 `usearch_audit_reconcile_lag_seconds`
    Gauge 갱신 (NFR-003).

**GREEN tasks**:

- `internal/audit/litellm_reconcile.go`:
  - Asynq scheduled job 등록(default `*/5 * * * *`).
  - HTTP client → LiteLLM `/spend/logs` (configurable endpoint).
  - dedup: `SELECT 1 FROM cost_ledger WHERE request_id = $1 LIMIT 1` +
    `SELECT 1 FROM audit_events WHERE payload->>'litellm_request_id' = $1 LIMIT 1`.
  - INSERT audit_events: `event_type="cost.reconciled"`, `source="python"`,
    payload에 spend log 필드 통째 저장.
- `internal/audit/metrics.go` — reconcile 카운터/게이지 등록.

**REFACTOR**:

- LiteLLM HTTP client를 별도 module(`litellm_client.go`)로 추출하여 향후 mock 용이.
- dedup query를 single UNION SELECT으로 최적화.

---

### Phase E — Replay Endpoint + Admin Wiring

목표: `/admin/audit/replay`, RBAC + rate limit + admin event emission.

**RED tests** (7):

25. `TestReplayRequiresAdminAuth` — Authorization 누락 → 401, AUTH-002 permission
    `audit.replay` 없음 → 403 (REQ-004).
26. `TestReplayRateLimitTriggers429` — 1 replay/min cap 초과 → 429 + Retry-After
    (REQ-004).
27. `TestReplayUnknownRequestIdReturns400` — audit_events에 없는 request_id →
    400 + `unknown_request_id` (REQ-004).
28. `TestReplayNonReplayableEventReturns400` — event_type이 `query.submit` /
    `deep.start` 아닌 경우 400 + `event_not_replayable` (REQ-004).
29. `TestReplayActorIdentityPropagation` — replay 호출 시 새 request의 user_id는
    admin actor, 원본 user_id는 payload.replayed_from에만 기록 (REQ-004).
30. `TestReplayEmitsAdminEvent` — replay 성공 시 audit_events에 `event_type=
    "admin.replay"` event 발신 (REQ-004).
31. `TestReplayWithPIIMaskedQueryReturns400` — query.submit row가 PII masked 상태
    (text_sha256만 존재) → 400 + `query_text_masked` (REQ-004, REQ-006).

**GREEN tasks**:

- `internal/audit/replay.go` — 핸들러 로직 (auth → rbac → rate limit → fetch →
  validate → re-execute → emit).
- `cmd/usearch-api/handlers/admin_audit.go` — chi 라우터 `r.Route("/admin/audit",
  func(r chi.Router) { r.Post("/replay", h.Replay); ... })`.
- `cmd/usearch-api/main.go` 수정 — admin routes wiring.

**REFACTOR**:

- replay rate limit을 costguard CapChecker 재사용으로 통합(DRY).
- replayable event_type 화이트리스트를 config로 분리(`audit.replay.allowed_event_types`).

---

### Phase F — S3 Export + Retention + PII Masking + Hash Chain

목표: optional features(S3 export, hash chain, PII masking) + nightly retention cleanup.

**RED tests** (10):

32. `TestS3ExportDisabledByDefault` — `audit.s3.enabled: false`이면 export job은
    schedule되지만 실행 시 즉시 return (REQ-005).
33. `TestS3ExportUploadsJSONLGz` — MinIO testcontainer → JSONL.gz 객체 PUT 검증,
    내용은 partition row를 newline-separated JSON으로 (REQ-005).
34. `TestS3ExportEmitsExportEvent` — 성공 시 audit_events에 `event_type=
    "audit.export"` row 추가 (REQ-005, REQ-009).
35. `TestS3ExportRetriesOnFailure` — S3 PUT 실패 → Asynq 재시도 3회 (REQ-005).
36. `TestCleanupSkipsRecentPartitions` — hot_days=90일 이내 partition은 DROP되지
    않음 (REQ-007).
37. `TestCleanupSkipsUnarchivedWhenRequireS3IsTrue` — `require_s3_archive: true`
    + archived_at NULL → DROP되지 않음 (REQ-007).
38. `TestCleanupEmitsPartitionDropEvent` — DROP 시 audit_events에
    `event_type="audit.partition_drop"` event (REQ-007).
39. `TestPIIMaskingReplacesQueryText` — `audit.pii.mask_query_text: true`에서
    query.submit emit 시 query.text → text_sha256 (REQ-006).
40. `TestPIIMaskingPreservesIdentityFields` — masking 활성화 상태에서도 user_id,
    tenant_id, request_id는 그대로 (REQ-006).
41. `TestHashChainAdvisoryLockPreventsRace` — `audit.hash_chain.enabled: true`에서
    동시 INSERT 100건 → prev_hash 정확히 연결, daily verify 통과 (REQ-008,
    NFR-007).

**GREEN tasks**:

- `internal/audit/export.go` — S3 export Asynq job. aws-sdk-go-v2 single client
  for MinIO + AWS. streaming SELECT → gzip writer → S3 PUT.
- `internal/audit/cleanup.go` — nightly Asynq job, `DROP PARTITION` via
  `audit_admin` connection.
- `internal/audit/chain.go` — advisory lock + SHA256 hash + canonical_json.
- `internal/audit/store.go` 수정 — masking + hash chain branches.
- `.moai/config/sections/audit.yaml` — 전체 config sections.
- `deploy/docker-compose.yml` — MinIO 서비스 추가.

**REFACTOR**:

- canonical_json을 별도 helper로 추출, golden test로 stability 검증.
- chain verify job을 별도 file로 분리.

---

## 3. Test Catalog Summary

| Phase | Tests Added | REQs Covered | NFRs Covered |
|-------|-------------|--------------|--------------|
| A | 6 | 001, 008 | 001 |
| B | 7 | 002 (DEEP-004 forward-compat), 009 | 002, 008 |
| C | 5 | 002 (cost.recorded, index.write) | — |
| D | 6 | 003, 009 | 003 |
| E | 7 | 004, 006 | — |
| F | 10 | 005, 006, 007, 008 | 004, 005, 007 |
| **Total** | **41** | **9 / 9** | **8 / 8** |

---

## 4. Risk Mitigation Table

| Risk | Phase | Mitigation Strategy |
|------|-------|---------------------|
| **R1** Audit write가 hot path latency를 증가 | Phase B | `audit.async: true` 기본 + Asynq queue. NFR-002로 50ms p95 강제. `TestEmitEventAsyncEnqueuesAsynq` |
| **R2** cost_ledger trigger 실패가 cost write를 망친다 | Phase C | `audit.cost_mirror_strict: false` toggle. `TestCostLedgerTriggerFailOpenWhenStrictFalse` |
| **R3** Audit volume Postgres 디스크 폭증 | Phase F | Monthly partition + 90d retention. `index.write` toggle(D4). 추가로 S3 archive 후 DROP |
| **R4** Hash chain race under concurrent INSERT | Phase F | App-side advisory lock per-tenant(`pg_advisory_xact_lock`). `TestHashChainAdvisoryLockPreventsRace` |
| **R5** Python reconciliation drift(중복/누락) | Phase D | `litellm_request_id` UNIQUE dedup. window overlap 1분. NFR-005로 drift ≤ 0.5% 강제 |
| **R6** S3 outage이 retention cleanup 차단 | Phase F | `require_s3_archive: false` toggle. operator escape hatch |
| **R7** Replay endpoint이 PII 유출 | Phase E | AUTH-002 `audit.replay` permission + admin actor identity propagation. `TestReplayActorIdentityPropagation` |
| **R8** event_type cardinality explosion | Phase B | startup enum lock. `TestEventTypeBoundedEnum` + NFR-008 |
| **R9** PG trigger overhead under high ingest | Phase C | `audit.events.index_write_enabled: false` toggle. M8 sampling 재평가 (D4) |
| **R10** LiteLLM `/spend/logs` schema 변경 | Phase D | 단일 fetcher isolated. parse 실패 시 `reconcile_polls_total{outcome="error"}` |
| **R11** Audit-as-PII | Phase F | retention + PII masking + AUTH-002 `audit.read` permission |
| **R12** Raw SQL이 trigger 우회 | Phase A | DB role separation. `audit_writer`는 UPDATE/DELETE 권한 없음. `TestAuditAdminRoleSeparation` |

---

## 5. MX Tag Plan

본 SPEC의 구현은 다음 @MX 태그를 생성한다.

### 5.1 @MX:ANCHOR (high fan_in, invariant contract)

- `internal/audit/store.go::EmitEvent`
  — fan_in ≥ 7 (AUTH-001 middleware, AUTH-002 enforcer, synthesis handler, deep
  handler, DEEP-004 decision log tee, reconcile job, index hook, admin handler).
  모든 audit event 발신점의 단일 funnel. `@MX:REASON: 발신점 boilerplate를
  바꾸려면 모든 caller를 동시 수정해야 한다`. `@MX:SPEC: SPEC-AUTH-003`.
- `internal/audit/decision_log_handler.go::Handle`
  — DEEP-004 forward-compat의 single point of integration. `@MX:REASON: DEEP-004
  schema 6 mandatory 필드(timestamp, event_type, request_id, tenant_id, user_id,
  decision)의 1:1 매핑이 invariant`. `@MX:SPEC: SPEC-AUTH-003 §1.3`.
- `internal/audit/chain.go::ComputeThisHash`
  — hash chain enabled 상태에서 모든 INSERT가 거치는 함수. canonical_json의 정의가
  바뀌면 기존 chain 전체가 invalidate. `@MX:REASON: hash chain invariant`.

### 5.2 @MX:WARN (danger zone, requires @MX:REASON)

- `deploy/postgres/migrations/0004_audit_cost_ledger_trigger.sql::audit_mirror_cost_ledger`
  — trigger 실패가 cost_ledger INSERT를 abort. `@MX:WARN`: `@MX:REASON: DEEP-004
  cost write path를 implicit하게 의존. trigger 수정 시 DEEP-004 운영 영향 평가 필수`.
- `internal/audit/replay.go::HandleReplay`
  — admin이 임의 query를 재실행 가능. `@MX:WARN`: `@MX:REASON: AUTH-002 permission
  검증이 무효화되면 audit_events의 모든 query를 admin이 재실행 가능. policy 변경
  시 본 함수의 RBAC 검증 순서 확인 필수`.
- `internal/audit/chain.go::AcquireAdvisoryLock`
  — Postgres advisory lock의 timeout 미설정 시 deadlock 가능. `@MX:WARN`:
  `@MX:REASON: hash chain enabled 상태에서 lock 보유 중 트랜잭션 crash → 다른 INSERT
  무한 대기`.
- `internal/audit/cleanup.go::DropPartition`
  — `audit_admin` role로 DROP PARTITION 실행. `@MX:WARN`: `@MX:REASON: DROP은
  revertible하지 않다. require_s3_archive 검증이 빠지면 데이터 영구 손실`.

### 5.3 @MX:NOTE (context & intent delivery)

- `internal/audit/store.go` 파일 상단 — DEEP-004 forward-compat 약속 인용 (spec.md §1.3).
- `internal/audit/litellm_reconcile.go::performReconciliation` — DEEP-004 §1.4
  Python-side 위임 인용.
- `internal/audit/types.go::EventType` — enum 추가는 SPEC amendment + 코드 변경
  필수임을 명시.

### 5.4 @MX:TODO (incomplete work — resolved in GREEN phase)

- RED phase에서 placeholder 함수에 `@MX:TODO`를 부착하고 GREEN phase 종료 시 모두
  제거.

---

## 6. File Touch Order (recommended TDD progression)

1. **Phase A start**: `deploy/postgres/migrations/0003_audit_events.sql` →
   `internal/audit/types.go` → `internal/audit/partitions.go` → 6 tests.
2. **Phase B**: `internal/audit/store.go` → `internal/audit/decision_log_handler.go`
   → `internal/obs/log/handler.go` 수정 → 7 tests.
3. **Phase C**: `deploy/postgres/migrations/0004_audit_cost_ledger_trigger.sql` →
   `internal/index/index.go` audit hook → 5 tests.
4. **Phase D**: `internal/audit/litellm_reconcile.go` →
   `internal/audit/metrics.go` (reconcile collectors) → 6 tests.
5. **Phase E**: `internal/audit/replay.go` → `cmd/usearch-api/handlers/admin_audit.go`
   → `cmd/usearch-api/main.go` wiring → 7 tests.
6. **Phase F**: `internal/audit/export.go` → `internal/audit/cleanup.go` →
   `internal/audit/chain.go` → `internal/audit/store.go` masking/chain branches →
   `.moai/config/sections/audit.yaml` → `deploy/docker-compose.yml` (MinIO) → 10 tests.

---

## 7. Coverage and Quality Gates

- Coverage 목표: 85% (per `quality.yaml`).
- 새 package `internal/audit/`만 측정.
- TRUST 5 gates: 모든 phase 종료 시점에 `go vet` + `golangci-lint` + `go test -race`
  통과.
- Cardinality test: `internal/obs/metrics/metrics_test.go::TestNoUnboundedLabels`
  화이트리스트에 `event_type`, `decision`, `source`, `outcome` 라벨 추가 후 통과.
- LSP gate: zero errors / zero type errors / zero lint errors.
- Migration validation: `0003_*.sql`, `0004_*.sql` 두 번 실행해도 idempotent 검증.

---

## 8. Pre-submission Self-Review

전체 changeset이 완성된 시점에 다음을 확인한다:

- DEEP-004 `internal/deepagent/costguard/` 내부 파일이 SINGLE LINE 변경 없는가? (forward-compat)
- `cost_ledger` 스키마(`deploy/postgres/migrations/0002_cost_ledger.sql`)가 변경 없는가?
- DEEP-004 6 mandatory 필드(timestamp/event_type/request_id/tenant_id/user_id/decision)
  가 audit_events row에 1:1 매핑되는가?
- audit handler chain의 audit tee handler 실패가 stderr write를 abort하지 않는가?
- application connection role이 audit_events에 대해 UPDATE/DELETE 권한이 없는가?
- replay 호출이 원본 user identity를 재사용하지 않고 admin actor identity로 실행되는가?
- LiteLLM reconciliation이 cost_ledger를 변경하지 않는가?
- Prometheus 메트릭 cardinality가 NFR-OBS-002를 준수하는가?
- 모든 새 event_type이 startup enum에 등록되었는가?

---

## 9. Implementation Sequencing Across Sessions

본 SPEC의 6개 phase는 sequential 의존성을 가진다. 단일 manager-tdd 세션으로 완주가
어려운 경우 다음 세션 분할이 권장된다:

- **Session 1**: Phase A + B (schema foundation + emitter + DEEP-004 tee)
- **Session 2**: Phase C + D (cost_ledger trigger + LiteLLM reconciliation)
- **Session 3**: Phase E + F (replay endpoint + S3/retention/hash chain)

각 세션 시작 시 `/clear` 후 본 plan.md만 재로드하여 컨텍스트를 보존한다.

---

## 10. AUTH-002 Permission Additions (cross-spec dependency)

본 SPEC이 ship되기 전에 AUTH-002의 Casbin policy에 다음 permission이 추가되어야
한다(AUTH-002 amendment 필요):

| Permission | Subject scope | Resource scope | Action | Default grantees |
|------------|--------------|----------------|--------|------------------|
| `audit.read.team` | team member | own team's audit_events | read | team_owner role |
| `audit.read.global` | admin | all audit_events | read | admin role |
| `audit.replay` | admin | replayable events only | replay | admin role |
| `audit.export.trigger` | admin | S3 export job | trigger | admin role |
| `audit.partition.drop` | admin | partition DROP | execute | superadmin role |

본 SPEC plan-auditor cycle 중 AUTH-002 spec.md에 위 5개 permission 추가 PR을 요청한다.
AUTH-002 미배포 환경에서도 본 SPEC은 동작 가능(safety default: 모든 admin endpoint
가 403 반환).

---

*End of SPEC-AUTH-003 plan.*
