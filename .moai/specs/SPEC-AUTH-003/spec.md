---
id: SPEC-AUTH-003
version: 0.1.0
status: draft
created: 2026-05-22
updated: 2026-05-22
author: limbowl
priority: P0
issue_number: 0
title: Audit log — immutable trail, query replay, S3 archive, LiteLLM cost reconciliation
milestone: M6 — Team plane
owner: expert-security
methodology: tdd
coverage_target: 85
depends_on: [SPEC-AUTH-001, SPEC-AUTH-002, SPEC-DEEP-004, SPEC-OBS-001]
blocks: []
---

# SPEC-AUTH-003: Audit log — immutable trail, query replay, S3 archive, LiteLLM cost reconciliation

## HISTORY

- 2026-05-22 (initial draft v0.1.0, limbowl via manager-spec):
  First EARS-formatted SPEC for the M6 Team-plane audit deliverable. `audit_events`는
  AUTH-001(OIDC SSO)이 발행한 identity, AUTH-002(Casbin RBAC)가 만든 decision,
  DEEP-004의 cap/cost event를 단일 immutable trail로 통합한다. 7개 pinned decision
  은 research §7에서 context-derived로 확정되었고 §1.1에 재명시한다.

  Pinned decisions:
  (D1) Event taxonomy: 11 categories × ~20 event_types (auth/rbac/query/deep/cost/index/admin/system).
       startup time enum lock으로 cardinality 폭증 방지. event_type 추가는 SPEC amendment 필수.
  (D2) cost_ledger와의 관계: cross-reference only(payload.cost_ledger_id). DEEP-004
       schema는 변경하지 SHALL NOT 한다. cost_ledger row INSERT는 Postgres AFTER INSERT
       trigger가 audit_events에 `event_type="cost.recorded"` mirror.
  (D3) Python-side LiteLLM cost 회수: 5-min Asynq scheduled job이 LiteLLM
       `GET /spend/logs?summarize=false`를 polling, request_id dedup, audit_events에만
       `event_type="cost.reconciled"`로 기록. cost_ledger는 변경 없음.
  (D4) Index event volume: V1은 모든 `index.write`를 기록. 운영자가
       `audit.events.index_write_enabled: false`로 toggle 가능. M8에서 sampling 재평가.
  (D5) cost_ledger mirror 메커니즘: Postgres AFTER INSERT trigger. DEEP-004 Go 코드 변경 0.
       trigger 실패는 cost_ledger INSERT를 abort하나, 운영자가
       `audit.cost_mirror_strict: false`로 fail-open 가능.
  (D6) Hash chain: optional, default OFF. 활성화 시 daily verify job이 모든 row 검증.
       app-side INSERT path는 advisory lock per-tenant로 race 방지.
  (D7) S3 export format: JSONL.gz weekly partition-level. opt-in via
       `audit.s3.enabled: true`. MinIO local 및 AWS S3 양쪽 지원.

  M6 Team-plane SPEC으로서 본 SPEC은 별도 GitHub Issue로 트랙되지 않으며
  (`issue_number: 0`) plan-auditor 통과 후 status `draft → approved` 전이.

  Companion artifacts:
  - `.moai/specs/SPEC-AUTH-003/research.md` — Phase 0.5 research (~750 lines,
    10 sections — audit surface analysis, DEEP-004 forward-compat audit, schema design,
    replay endpoint design, S3 export, PII/retention/tamper, 7 pinned decisions,
    observability, 12 risks)
  - `.moai/specs/SPEC-AUTH-003/plan.md` — TDD task sequence, 6-phase implementation,
    MX tag plan
  - `.moai/specs/SPEC-AUTH-003/acceptance.md` — Given/When/Then 시나리오 (8 main + 1 edge)
  - `.moai/specs/SPEC-AUTH-003/tasks.md` — Phase 단위 task decomposition
  - `.moai/specs/SPEC-AUTH-003/spec-compact.md` — compact view

  9 EARS REQs (8 × P0 + 1 × P1), 8 NFRs, 6 modules (Schema / Emission / Reconciliation /
  Replay / Export / Hardening). Methodology: TDD, coverage target 85%, harness: standard.
  Owner: expert-security.

---

## 1. Overview

본 SPEC은 M6 milestone Team-plane의 마지막 deliverable이자 V1 release gate의
audit completeness 요건(`.moai/project/product.md` "audit log complete")을 충족하는
immutable audit trail 계층을 정의한다. AUTH-001(OIDC SSO)이 user identity를 채우고
AUTH-002(Team RBAC)가 access decision을 만들며 DEEP-004가 cap/cost event를
emit하는 상태에서, AUTH-003는 이 세 출처의 event + 모든 data-plane 호출(query / deep /
index write)을 단일 Postgres 테이블(`audit_events`)에 immutable trail로 기록한다.

본 SPEC은 다음 6축을 단일 `internal/audit/` 패키지로 통합한다.

1. **Append-only Postgres audit table**: monthly partition, append-only triggers,
   bounded event_type enum.
2. **DEEP-004 decision event log 통합**: stderr JSON line을 audit_events에 mirror
   (DEEP-004 schema 변경 0).
3. **cost_ledger mirror**: Postgres trigger가 `cost_ledger` INSERT를 audit_events에
   `event_type="cost.recorded"`로 자동 복사 (cost_ledger 스키마 unchanged).
4. **Python-side LiteLLM cost reconciliation**: 5-min Asynq scheduled job이
   `/spend/logs`를 polling하여 Python 사이드카 호출 비용을 audit_events에 기록.
5. **Admin-only query replay endpoint**: AUTH-002 RBAC + rate limit으로 보호된
   `/admin/audit/replay`.
6. **Optional S3 export job + retention**: weekly JSONL.gz export, 90일 hot retention,
   tamper-resistance(append-only triggers + optional hash chain).

### 1.1 Pinned Architectural Decisions

다음 7개 결정은 research §7에서 context-derived로 확정되었다. 본 SPEC은 이를 EARS
요구사항으로 번역할 뿐 재논의하지 않는다.

1. **Event taxonomy**: 11 categories × ~20 event_types. startup time enum lock.
   `event_type` 추가는 코드 변경(SPEC amendment).
2. **cost_ledger 관계**: cross-reference only (`payload.cost_ledger_id`). DEEP-004
   schema는 변경하지 SHALL NOT.
3. **Python-side cost 회수**: 5-min Asynq polling of LiteLLM `/spend/logs`; audit_events
   에만 기록(cost_ledger는 변경 없음). cap 평가에는 사용되지 SHALL NOT.
4. **Index event volume**: V1 all-in + `audit.events.index_write_enabled` toggle.
   sampling은 M8 후속.
5. **cost_ledger mirror**: Postgres AFTER INSERT trigger. 실패 시 cost_ledger INSERT
   abort default; 운영자가 `audit.cost_mirror_strict: false` 시 fail-open.
6. **Hash chain**: optional, default OFF. 활성화 시 advisory lock per-tenant로 INSERT
   race 방지 + daily verify job.
7. **S3 export**: weekly JSONL.gz partition-level. MinIO + AWS S3 호환.

### 1.2 Motivation

team plane의 audit 필요성은 다음 4축에서 발생한다.

- **Security incident response**: 어떤 사용자가 언제 어떤 query를 던졌고 어떤 RBAC
  decision이 났는가? AUTH-001/002 만으로는 이 trail이 없다.
- **Compliance**: 다중 사용자 환경에서 누가 무엇을 검색했는지의 immutable record는
  GDPR / SOC2 등 향후 audit 요건의 baseline.
- **Cost reconciliation**: DEEP-004는 Go-side cost만 추적. Python 사이드카(researcher,
  STORM)는 LiteLLM 직접 호출이라 cost가 cost_ledger에 누락. 본 SPEC이 audit_events에
  reconciled 행으로 회수.
- **Forensic replay**: 과거 query를 현재 어댑터로 재실행해 회귀 검증.

### 1.3 Relationship to DEEP-004 (Forward-Compatibility Honor)

DEEP-004 spec.md §6.3는 다음을 약속했다:

> "decision event log JSON line schema 또한 SPEC-AUTH-003(M6) audit subsystem이
> downstream consumer로 합류할 때 호환되도록 설계된다... schema는 **additive**이며
> SPEC-AUTH-003는 새 필드를 추가할 수 있으나 위 필드를 rename하거나 remove할 수
> SHALL NOT 한다."

본 SPEC은 위 약속을 다음과 같이 이행한다:

1. **schema unchanged**: DEEP-004의 stderr JSON line emission code(`internal/deepagent/
   costguard/middleware.go::emitDecisionEvent`)는 변경되지 SHALL NOT 한다. AUTH-003은
   별도 slog handler로 같은 line을 tee 받아 audit_events에 mirror한다.
2. **cost_ledger schema unchanged**: DEEP-004의 `cost_ledger` 테이블 컬럼은 변경되지
   SHALL NOT 한다. AUTH-003은 PG trigger로 cost_ledger INSERT를 audit_events에
   복사할 뿐이며, cost_ledger.user_id의 opaque TEXT semantics, outcome enum, deep_run_id
   nullable 등 모든 invariant가 보존된다.
3. **DEEP-004 6 mandatory fields(`timestamp`, `event_type`, `request_id`, `tenant_id`,
   `user_id`, `decision`)**가 audit_events.audit_events row에 1:1 매핑되며, 추가 필드는
   audit_events column 또는 payload JSONB key로만 표현된다(rename/remove 없음).

M6 진입 시 audit subsystem review checkpoint를 본 SPEC이 등록하며, plan-auditor가
DEEP-004 spec §6.3 commitment 준수 여부를 검증한다.

---

## 2. EARS Requirements

### 2.1 Audit Event Schema Module

| ID | Pattern | Requirement | Priority | Acceptance |
|----|---------|-------------|----------|------------|
| **REQ-AUTH3-001** | Ubiquitous | 본 SPEC은 Postgres 마이그레이션 `deploy/postgres/migrations/0003_audit_events.sql`을 신설하여 `audit_events` 테이블을 SHALL 생성한다. 컬럼: `(id BIGSERIAL, ts TIMESTAMPTZ NOT NULL DEFAULT NOW(), event_type TEXT NOT NULL, request_id TEXT, tenant_id TEXT NOT NULL DEFAULT 'default', user_id TEXT NOT NULL DEFAULT 'anonymous', team_id TEXT, resource TEXT, action TEXT, decision TEXT, deep_run_id TEXT, source TEXT NOT NULL DEFAULT 'go', ip INET, user_agent TEXT, payload JSONB, prev_hash TEXT, this_hash TEXT, schema_version INT NOT NULL DEFAULT 1)` with `PRIMARY KEY (id, ts)` and `PARTITION BY RANGE (ts)`. 인덱스: `(tenant_id, user_id, ts DESC)`, `(event_type, ts DESC)`, `(request_id) WHERE NOT NULL`, `(deep_run_id) WHERE NOT NULL`, `(team_id, ts DESC) WHERE NOT NULL`. `BEFORE UPDATE`와 `BEFORE DELETE` trigger가 `RAISE EXCEPTION`으로 row 수정/삭제를 SHALL 차단한다. partition drop 작업은 별도 DB role(`audit_admin`)만 수행 가능. (Acceptance §5.1, §5.6) | P0 | `TestMigration0003Idempotent`, `TestAuditEventsSchemaMatchesSpec`, `TestAuditEventsBlocksUpdate`, `TestAuditEventsBlocksDelete`, `TestAuditEventsPartitioned` |

### 2.2 Event Emission Module

| ID | Pattern | Requirement | Priority | Acceptance |
|----|---------|-------------|----------|------------|
| **REQ-AUTH3-002** | Ubiquitous | 본 SPEC은 단일 audit emitter(`internal/audit/store.go::EmitEvent(ctx, AuditEvent)`)를 도입하여 모든 audit event 발신점이 이 함수를 SHALL 경유한다. 발신점 통합 대상: (a) AUTH-001 JWT 미들웨어(login/logout/fail/token.refresh), (b) AUTH-002 Casbin enforcer(rbac.allow/deny, rbac.policy_change), (c) `cmd/usearch-api/handlers/synthesis.go`(query.submit/complete/fail, deep.start/complete/fail), (d) DEEP-004 decision event log(추가 emit point 없이 slog handler tee로 `cap.evaluation` 이벤트 수신), (e) DEEP-004 cost_ledger 트리거(`cost.recorded` 자동 발신, REQ-AUTH3-005 참조), (f) Python reconciliation job(`cost.reconciled`, REQ-AUTH3-003 참조), (g) `internal/index/index.go`(index.write/delete/rebuild, `audit.events.index_write_enabled: true`인 경우에 한함), (h) admin handler(admin.replay, admin.config_change). DEEP-004 emit code는 변경되지 SHALL NOT 한다. (Acceptance §5.1, §5.2, §5.7) | P0 | `TestEmitEventInsertsRow`, `TestEmitEventNoPanicOnNilCtx`, `TestDeep004DecisionLogMirroredToAudit`, `TestEmitEventDoesNotMutateDeep004Emit`, `TestIndexWriteToggleControlsEmission` |

### 2.3 Python-side LLM Cost Reconciliation Module

| ID | Pattern | Requirement | Priority | Acceptance |
|----|---------|-------------|----------|------------|
| **REQ-AUTH3-003** | Event-Driven | WHEN Asynq scheduled job `audit.litellm_reconcile`이 발화하면(기본 5분 주기, deep.yaml `audit.reconcile.interval_minutes`로 override 가능), 본 모듈은 LiteLLM `GET /spend/logs?summarize=false&start_date={last_run_ts - 1min}&end_date={now}` 엔드포인트를 SHALL 호출한다. 응답 row 중 (a) Go-side `cost_ledger`에 동일 `request_id`가 이미 존재하는 row, (b) 직전 reconciliation cycle에서 이미 audit_events에 `event_type="cost.reconciled"` AND `payload.litellm_request_id` 일치 row를 SHALL 제외(dedup). 남은 row 각각에 대해 audit_events에 `event_type="cost.reconciled"`, `source="python"`, `payload={litellm_request_id, model, prompt_tokens, completion_tokens, spend_usd, call_type, metadata.user_api_key_hash}` 형태로 INSERT SHALL 한다. 본 reconciliation job은 cost_ledger를 변경하지 SHALL NOT 한다(DEEP-004 forward-compat 보존). LiteLLM `/spend/logs` 호출 실패 시 `usearch_audit_reconcile_polls_total{outcome="error"}` 카운터 1 증가, 빈 응답 시 `outcome="empty"`, 성공 시 `outcome="success"`. (Acceptance §5.3) | P0 | `TestReconcileFetchesSpendLogs`, `TestReconcileDedupByRequestIdAgainstCostLedger`, `TestReconcileDedupByLitellmRequestIdAgainstAuditEvents`, `TestReconcileDoesNotMutateCostLedger`, `TestReconcileEmitsErrorCounterOnLitellmFailure` |

### 2.4 Query Replay Module

| ID | Pattern | Requirement | Priority | Acceptance |
|----|---------|-------------|----------|------------|
| **REQ-AUTH3-004** | Event-Driven | WHEN `POST /admin/audit/replay`가 호출되면, 본 SPEC은 다음 순서를 SHALL 실행한다: (1) AUTH-001 JWT 검증(미인증 → 401), (2) AUTH-002 RBAC `audit.replay` permission 검증(불충분 → 403), (3) Redis 기반 sliding window rate limit `audit.replay.max_per_minute`(기본 1) 평가(초과 → 429 + Retry-After), (4) body `{request_id: string}` 파싱(빈 값 → 400 + `unknown_request_id`), (5) `audit_events SELECT WHERE request_id=$1 AND event_type IN ('query.submit','deep.start')` 수행(없음 → 400 + `event_not_replayable`), (6) 새 request_id 생성 후 원본 query payload를 사용해 내부 핸들러 재실행(`/query` 또는 `/deep`), (7) 원본/replay 두 audit_events row 모두에 `payload.replayed_by={admin_user_id}` 와 `payload.replayed_from={original_request_id}` 기록, (8) `event_type="admin.replay"` 이벤트 발신. replay actor의 identity가 새 request의 user_id로 propagate되며 원본 user의 credential은 사용되지 SHALL NOT 한다. (Acceptance §5.4, §5.5) | P0 | `TestReplayRequiresAdminAuth`, `TestReplayRateLimitTriggers429`, `TestReplayUnknownRequestIdReturns400`, `TestReplayNonReplayableEventReturns400`, `TestReplayEmitsAdminEvent`, `TestReplayActorIdentityPropagation` |

### 2.5 S3 Export Module

| ID | Pattern | Requirement | Priority | Acceptance |
|----|---------|-------------|----------|------------|
| **REQ-AUTH3-005** | Optional | WHERE 본 SPEC은 `.moai/config/sections/audit.yaml`의 `audit.s3.enabled: true`로 명시 활성화된 경우, weekly Asynq scheduled job `audit.export`(기본 cron `0 2 * * 0`, UTC)가 다음을 SHALL 실행한다: (1) `audit.export.older_than_days`(기본 7) 이상 경과한 partition 식별, (2) 각 partition을 streaming SELECT으로 JSONL.gz로 변환, (3) S3 객체 path `s3://{bucket}/{prefix}{tenant_id}/year=YYYY/month=MM/day=DD/events-{partition_name}.jsonl.gz`로 PUT, (4) `audit_partitions` 메타 테이블에 `archived_at = NOW()` 기록, (5) `event_type="audit.export"`, payload={s3_uri, partition_range, row_count, bytes_compressed} 이벤트 발신. S3 업로드 실패 시 Asynq 재시도(3회 exponential backoff 1m/5m/15m); 모두 실패하면 archived_at은 NULL로 남아 다음 주기에 재시도 SHALL 한다. MinIO 로컬과 AWS S3 양쪽 모두 단일 aws-sdk-go-v2 client로 SHALL 지원한다. (Acceptance §5.6) | P1 | `TestS3ExportDisabledByDefault`, `TestS3ExportUploadsJSONLGz`, `TestS3ExportEmitsExportEvent`, `TestS3ExportRetriesOnFailure`, `TestS3ExportSkipsArchivedPartitions`, `TestS3ExportWorksWithMinIOEndpoint` |

### 2.6 PII Masking Module

| ID | Pattern | Requirement | Priority | Acceptance |
|----|---------|-------------|----------|------------|
| **REQ-AUTH3-006** | Optional | WHERE `audit.pii.mask_query_text: true`로 명시 활성화된 경우, audit emitter는 다음 event_type의 `payload.query.text` 필드를 SHALL `payload.query.text_sha256 = SHA256(original_text)`로 교체한다: `query.submit`, `query.complete`, `deep.start`, `deep.complete`. 다른 필드(`user_id`, `tenant_id`, `request_id`, `team_id`)는 masking 대상 SHALL NOT 한다. `audit.pii.mask_ip: true` 활성화 시 `audit_events.ip` 컬럼 INSERT 값을 `NULL`로 SHALL 강제한다. 기본값은 두 toggle 모두 `false`. masking이 적용된 query 이벤트는 replay 엔드포인트가 SHALL `400 Bad Request` + body `{"error":"query_text_masked","detail":"original query unavailable for replay"}`로 거부한다. (Acceptance §5.7) | P1 | `TestPIIMaskingDisabledByDefault`, `TestPIIMaskingReplacesQueryText`, `TestPIIMaskingPreservesIdentityFields`, `TestPIIMaskingPreventsReplay`, `TestIPMaskingNullifiesColumn` |

### 2.7 Retention Module

| ID | Pattern | Requirement | Priority | Acceptance |
|----|---------|-------------|----------|------------|
| **REQ-AUTH3-007** | Ubiquitous | 본 SPEC은 nightly Asynq scheduled job `audit.cleanup`(기본 cron `30 3 * * *`, UTC)을 SHALL 운영한다. 이 job은 `audit.retention.hot_days`(기본 90)일 이상 경과한 partition을 식별하고, `audit.retention.require_s3_archive: true`(기본값)인 경우 해당 partition의 `audit_partitions.archived_at IS NOT NULL` 인 partition만 SHALL DROP 한다. `require_s3_archive: false`인 경우 archived 여부에 무관하게 DROP. DROP 실행 시 `event_type="audit.partition_drop"`, payload={partition_name, row_count, range_start, range_end, archived: bool} 이벤트를 SHALL 발신한다. 모든 DROP은 `audit_admin` DB role을 통해 실행된다(일반 application role은 DROP 권한 없음). (Acceptance §5.6) | P0 | `TestCleanupSkipsRecentPartitions`, `TestCleanupSkipsUnarchivedWhenRequireS3IsTrue`, `TestCleanupDropsArchivedPartitions`, `TestCleanupEmitsPartitionDropEvent`, `TestCleanupRequiresAuditAdminRole` |

### 2.8 Tamper Resistance Module

| ID | Pattern | Requirement | Priority | Acceptance |
|----|---------|-------------|----------|------------|
| **REQ-AUTH3-008** | Unwanted | IF application connection이 `audit_events` 테이블에 대해 `UPDATE`, `DELETE` 또는 `TRUNCATE`를 시도하면, Postgres는 REQ-AUTH3-001의 `BEFORE UPDATE`/`BEFORE DELETE` trigger를 통해 RAISE EXCEPTION을 SHALL 발생시킨다. 추가 방어로, application connection은 DB role `audit_writer`를 사용하며 이 role은 `audit_events`에 대해 `SELECT, INSERT` 권한만 SHALL 가진다(`GRANT` 정책으로 강제). `audit.hash_chain.enabled: true`(기본 `false`)인 경우, audit emitter는 INSERT 직전에 advisory lock per-tenant(`pg_advisory_xact_lock(hashtext(tenant_id))`)을 획득하고, `prev_hash`는 같은 `tenant_id` + `event_type` 조합의 직전 row의 `this_hash`로 SHALL 설정, `this_hash = SHA256(prev_hash || canonical_json(row_minus_hashes))`로 SHALL 계산한다. daily Asynq job `audit.chain_verify`가 모든 row를 순차 검증하며 위반 발견 시 `usearch_audit_chain_violations_total` 카운터 1 증가 + WARN 레벨 slog 출력 SHALL 한다. (Acceptance §5.6) | P0 | `TestAuditAdminRoleSeparation`, `TestAppConnectionDeniedUpdate`, `TestAppConnectionDeniedDelete`, `TestHashChainDisabledByDefault`, `TestHashChainAdvisoryLockPreventsRace`, `TestHashChainVerifyDetectsViolation`, `TestHashChainCanonicalJsonStable` |

### 2.9 Observability Module

| ID | Pattern | Requirement | Priority | Acceptance |
|----|---------|-------------|----------|------------|
| **REQ-AUTH3-009** | Ubiquitous | 본 SPEC은 다음 Prometheus 메트릭을 SHALL 신설한다(모두 namespace `usearch_audit_*`): (a) `usearch_audit_events_total{event_type, decision, source}` CounterVec, (b) `usearch_audit_write_duration_seconds` Histogram, (c) `usearch_audit_lag_seconds{source}` Gauge, (d) `usearch_audit_s3_export_duration_seconds` Histogram, (e) `usearch_audit_s3_export_rows_total` Counter, (f) `usearch_audit_s3_export_bytes_total` Counter, (g) `usearch_audit_chain_violations_total` Counter, (h) `usearch_audit_reconcile_polls_total{outcome}` CounterVec, (i) `usearch_audit_reconcile_lag_seconds` Gauge, (j) `usearch_audit_partition_drop_total` Counter, (k) `usearch_audit_replay_requests_total{outcome}` CounterVec. 모든 label은 bounded enum이며 SPEC-OBS-001 `TestNoUnboundedLabels` 화이트리스트에 `event_type`, `decision`, `source`, `outcome` 라벨 추가가 SHALL 필요하다. OTel span(`audit.replay`, `audit.export`)에 high-cardinality span attribute(actor_user_id, partition, s3_uri) 부착은 NFR-OBS-002 적용 범위 밖이므로 허용된다. (Acceptance §5.1, §5.4, §5.6) | P0 | `TestAuditMetricsRegistered`, `TestEventTotalCounterIncrements`, `TestWriteDurationHistogramObserved`, `TestReconcileCounterByOutcome`, `TestReplayCounterByOutcome`, `TestNoUnboundedLabelsAuditExtended`, `TestOTelSpanAttributesAuditReplay` |

---

## 3. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| **NFR-AUTH3-001** | Append-only invariant | application connection으로 실행되는 어떤 SQL도 audit_events row를 modify할 SHALL NOT. 구체적으로: `UPDATE`/`DELETE`/`TRUNCATE` 시 Postgres가 trigger 또는 `GRANT` 정책에 의해 거부한다. 측정: `TestAuditAdminRoleSeparation` + `TestAppConnectionDeniedUpdate` + `TestAppConnectionDeniedDelete` 통과. |
| **NFR-AUTH3-002** | Audit write latency | audit emitter `EmitEvent`의 wall-clock latency는 (a) 정상 경로 p95 ≤ 50ms (Asynq enqueue 포함), (b) Asynq queue 직접 INSERT 모드(`audit.async: false`) p95 ≤ 30ms. hot-path(/query, /deep) 핸들러는 `audit.async: true`로 동작하여 audit write가 응답 latency budget을 침범하지 SHALL NOT. 측정: `usearch_audit_write_duration_seconds` Histogram p95. |
| **NFR-AUTH3-003** | Reconciliation freshness | LiteLLM `/spend/logs` polling의 lag(가장 최근 polling 시점과 현재 시각의 차이)는 평균 ≤ 6분 SHALL 한다(5-min 주기 + 1분 처리 여유). 측정: `usearch_audit_reconcile_lag_seconds` Gauge. |
| **NFR-AUTH3-004** | S3 export throughput | weekly export job은 단일 partition(평균 200K rows, ~300MB uncompressed)을 ≤ 5분 내에 완료 SHALL 한다. 측정: `usearch_audit_s3_export_duration_seconds` Histogram p95. |
| **NFR-AUTH3-005** | Reconciliation drift | reconciliation job 종료 후 audit_events `event_type="cost.reconciled"` row 수와 LiteLLM `/spend/logs` 응답 row 수(dedup 후) 사이의 누적 drift는 ≤ 0.5% SHALL 한다. 측정: weekly verification job이 직전 7일 비교. |
| **NFR-AUTH3-006** | No PII in metric labels | 본 SPEC이 신설하는 모든 Prometheus 메트릭 label 값은 bounded enum이며 user_id, tenant_id, request_id, IP, query text 같은 PII / high-cardinality 값을 SHALL NOT 포함한다. SPEC-OBS-001 NFR-OBS-002 준수. 검증: `TestNoUnboundedLabelsAuditExtended` 통과. |
| **NFR-AUTH3-007** | Hash chain verification budget | `audit.hash_chain.enabled: true`인 경우 daily `audit.chain_verify` job은 90일 hot retention 전체(약 600K-2M rows)를 ≤ 30분 내에 검증 SHALL 한다. 측정: job 종료 시 emit되는 `usearch_audit_chain_verify_duration_seconds` Histogram. |
| **NFR-AUTH3-008** | Cardinality safety | event_type, decision, source, outcome 라벨 값은 startup time enum lock으로 bounded. 정의되지 않은 event_type을 emit하려는 시도는 runtime error 또는 `event_type="unknown"`으로 collapse SHALL 한다(deploy 시 config로 선택). 새 event_type 추가는 SPEC amendment + 코드 변경 둘 다 SHALL 필요하다. |

---

## 4. Exclusions (What NOT to Build)

[HARD] 본 SPEC은 다음 항목을 명시적으로 제외한다. 각 항목은 후속 SPEC 또는
별도 트랙의 책임이다.

- **JWT 발급/OIDC SSO 구현 자체 없음** — 본 SPEC은 AUTH-001이 발행한 JWT에서
  identity를 소비할 뿐이며, JWT 서명 검증·OIDC discovery·token endpoint 등의
  발급 로직은 SPEC-AUTH-001(M6)의 책임.
- **RBAC policy 정의 자체 없음** — 본 SPEC은 AUTH-002의 Casbin enforcer가 만든
  decision을 소비할 뿐. policy 모델·permission set·team scoping은 SPEC-AUTH-002(M6)의
  책임. `audit.replay`, `audit.read` 등 새 permission은 AUTH-002 amendment로 도입.
- **Real-time SIEM 통합 없음** — Splunk / ELK / Datadog 등 외부 SIEM으로의
  streaming은 본 SPEC 범위 밖. JSONL.gz S3 export가 batch interop 경로. real-time
  streaming은 별도 M9+ SPEC.
- **cost-aware billing 통합 없음** — `cost.reconciled` 이벤트는 audit purpose만이며
  billing system으로 push되지 SHALL NOT. Stripe·invoice·결제 통합은 SPEC-BILLING-001
  (post-V1)의 책임.
- **Python-side cost의 cost_ledger 통합 없음** — Python LLM 호출 비용은 audit_events
  의 `cost.reconciled` 이벤트로만 가시화되며, DEEP-004 cap 평가에 산입되지 SHALL NOT
  한다. cap 통합은 SPEC-AUTH-004(post-V1)가 다룬다.
- **cost_ledger 통합/마이그레이션 없음** — DEEP-004 forward-compat 보장. cost_ledger
  schema는 변경되지 SHALL NOT 하며, audit_events는 cost_ledger를 cross-reference만 한다.
- **Audit replay에서 원본 user identity 재사용 없음** — replay는 admin actor identity로
  실행되며 원본 user의 credential 또는 session은 절대 복원되지 SHALL NOT. 원본 user
  identity는 payload metadata로만 보존된다.
- **다중 region S3 replication 없음** — 단일 region S3 bucket export만 지원. multi-
  region replication은 AWS S3 native 기능에 위임(deploy config).
- **Real-time anomaly detection / ML-based audit analytics 없음** — ML 기반 이상 탐지
  (예: 비정상 RBAC deny 패턴)는 본 SPEC 범위 밖이며 M9 SPEC-EVAL-AUDIT-001로 위임.
- **GDPR right-to-be-forgotten 자동화 없음** — audit_events는 immutable이므로 특정
  user 삭제 요청 시 운영자가 별도 SQL DELETE를 superuser로 실행해야 한다. 자동화된
  GDPR delete API는 본 SPEC 범위 밖이며 SPEC-COMPLIANCE-001(post-V1)에 위임.
- **Audit data egress 통제(DLP) 없음** — admin이 audit data를 export한 후 외부로
  반출하는 행위는 운영적 통제(SOC 등)에 위임. 본 SPEC은 S3 export 시 audit event
  로만 trail을 남긴다.
- **GitHub Issue tracking on this SPEC** (skipped per session pattern; `issue_number: 0`).

---

## 5. Acceptance Scenarios

상세 Given/When/Then 시나리오는 `.moai/specs/SPEC-AUTH-003/acceptance.md`에
정의되어 있다. 본 절은 인덱스를 제공한다.

| Scenario | 설명 | Coverage |
|----------|------|----------|
| §5.1 | user login → audit_events 행 1개 (event_type="auth.login", decision="allow") | REQ-001, 002, 009 |
| §5.2 | RBAC deny → audit_events 행 1개 (event_type="rbac.deny", decision="deny") | REQ-002 |
| §5.3 | Python LiteLLM 호출 → 5분 후 reconciliation job 발화 → audit_events에 `cost.reconciled` 1행 추가, cost_ledger는 변경 없음 | REQ-003, NFR-003 |
| §5.4 | admin이 과거 query 재실행 → 새 query.submit 이벤트 + admin.replay 이벤트 emit, replay actor identity 사용 | REQ-004, 009 |
| §5.5 | non-admin user가 replay 시도 → 403 Forbidden, audit_events에 `admin.replay decision="deny"` 기록 | REQ-004 |
| §5.6 | weekly S3 export job 발화 → 7일 이상 partition JSONL.gz로 S3 업로드, archived_at 표시, retention cleanup이 archived partition만 drop | REQ-005, 007, 008, NFR-001, NFR-004 |
| §5.7 | PII masking on → query.submit 이벤트의 query.text가 sha256으로 교체, replay는 400 | REQ-006 |
| §5.8 | DEEP-004 cap.evaluation stderr line이 audit_events에도 mirror, DEEP-004 emit code 변경 없음, cost_ledger 변경 없음 (forward-compat) | REQ-002, §1.3 |
| Edge | hash_chain enabled 상태에서 두 concurrent INSERT → advisory lock으로 순차 직렬화 후 prev_hash 정확히 연결, daily verify 통과 | REQ-008, NFR-007 |

---

## 6. Dependencies & Blocks

### 6.1 Upstream SPEC dependencies (depends_on)

- **SPEC-AUTH-001** (draft, M6) — JWT 미들웨어가 같은 context key
  (`costguard.UserIDKey` 또는 새로운 `auth.UserIDKey`)에 user_id를 주입하며, AUTH-001
  발신 이벤트(login/logout/fail)가 audit emitter를 SHALL 호출한다. AUTH-001 미배포
  시에도 본 SPEC은 anonymous fallback으로 동작 가능(REQ-AUTH3-001 default).
- **SPEC-AUTH-002** (draft, M6) — Casbin enforcer가 decision 결과를 audit emitter로
  발신, `audit.replay` / `audit.read` permission이 정의된다. AUTH-002 미배포 시
  replay endpoint는 모든 호출자를 403으로 거부(safety default).
- **SPEC-DEEP-004** (implemented, M5) — decision event log JSON line schema(REQ-DEEP4-010)와
  `cost_ledger` 테이블(REQ-DEEP4-006)을 forward-compat 약속에 따라 흡수 없이 mirror
  한다. AUTH-003은 DEEP-004 schema·코드를 변경하지 SHALL NOT.
- **SPEC-OBS-001** (implemented, M1) — Prometheus 메트릭 등록 인프라, slog handler chain,
  cardinality allowlist(`TestNoUnboundedLabels`)를 본 SPEC이 확장.

### 6.2 Downstream blocked SPECs (blocks)

본 SPEC은 직접 블로킹하는 M6 SPEC이 없다. M6 Team-plane release gate로서 본 SPEC이
ship되면 V1 audit completeness 요건이 충족되며 M9 release가 audit-complete 상태로
가능해진다.

### 6.3 Forward-compatibility commitments

- **with SPEC-DEEP-004**: cost_ledger 스키마, decision event log JSON line의 6개
  mandatory 필드, cost_ledger.user_id의 opaque TEXT semantics는 본 SPEC 진입 시에도
  변경되지 SHALL NOT. 본 SPEC은 trigger와 slog handler tee로만 통합한다. M6 진입
  시 audit subsystem review checkpoint를 본 SPEC이 등록하며 plan-auditor가 본
  commitment 준수를 검증한다.
- **with SPEC-AUTH-001**: AUTH-001이 후속 amendment로 새 identity 필드(예: `team_id`)를
  추가하면 audit_events.team_id 컬럼이 자동으로 수용한다.
- **with SPEC-AUTH-002**: AUTH-002가 새 permission(`audit.replay`, `audit.read`,
  `audit.export.trigger`)을 추가해야 본 SPEC의 endpoint들이 동작한다. 새 permission
  목록은 본 SPEC plan.md §10에 기록되며 AUTH-002 amendment로 추가된다.
- **with SPEC-IDX-001**: `index.write` 이벤트는 IDX-001의 `Index.Upsert` 호출에
  audit hook을 부착하여 발신한다. IDX-001 surface는 변경되지 SHALL NOT — audit hook은
  `Index.Upsert` 외부 wrapper로 적용된다.

---

## 7. Files to Create / Modify

### 7.1 Created

| Marker | Path | Purpose |
|--------|------|---------|
| [NEW] | `internal/audit/store.go` | `EmitEvent(ctx, AuditEvent)` 단일 emitter, Asynq enqueue 또는 sync INSERT 선택 |
| [NEW] | `internal/audit/types.go` | `AuditEvent`, `EventType` enum, `Decision` enum, `AuditSource` enum |
| [NEW] | `internal/audit/decision_log_handler.go` | DEEP-004 stderr JSON line을 audit_events에 tee하는 slog handler |
| [NEW] | `internal/audit/litellm_reconcile.go` | LiteLLM `/spend/logs` polling Asynq job, dedup, `cost.reconciled` INSERT |
| [NEW] | `internal/audit/replay.go` | `/admin/audit/replay` 핸들러 로직, RBAC + rate limit |
| [NEW] | `internal/audit/export.go` | S3 export Asynq job, JSONL.gz streaming, MinIO+AWS 호환 |
| [NEW] | `internal/audit/cleanup.go` | nightly partition drop Asynq job, retention 검증 |
| [NEW] | `internal/audit/chain.go` | hash chain primitives, advisory lock, daily verify job |
| [NEW] | `internal/audit/partitions.go` | monthly partition lifecycle(create next, list, drop) |
| [NEW] | `internal/audit/config.go` | `audit.yaml` 로더 + hot-reload watcher |
| [NEW] | `internal/audit/metrics.go` | Prometheus collector 등록 헬퍼 |
| [NEW] | `internal/audit/store_test.go` | EmitEvent / Asynq enqueue / dedup 테스트 |
| [NEW] | `internal/audit/decision_log_handler_test.go` | DEEP-004 tee 미러링 검증 |
| [NEW] | `internal/audit/litellm_reconcile_test.go` | mock LiteLLM `/spend/logs` + dedup 검증 |
| [NEW] | `internal/audit/replay_test.go` | RBAC / rate-limit / 400/403/429 시나리오 |
| [NEW] | `internal/audit/export_test.go` | MinIO testcontainer 기반 JSONL.gz 업로드 검증 |
| [NEW] | `internal/audit/cleanup_test.go` | partition drop + archive 조건 검증 |
| [NEW] | `internal/audit/chain_test.go` | hash chain 정확성 + advisory lock race 검증 |
| [NEW] | `internal/audit/config_test.go` | audit.yaml 파싱, hot-reload, 기본값 fallback |
| [NEW] | `cmd/usearch-api/handlers/admin_audit.go` | chi 라우터 wiring (`/admin/audit/replay`, `/admin/audit/export`) |
| [NEW] | `deploy/postgres/migrations/0003_audit_events.sql` | audit_events 테이블 + 트리거 + role 정의 |
| [NEW] | `deploy/postgres/migrations/0004_audit_cost_ledger_trigger.sql` | cost_ledger AFTER INSERT trigger (mirror) |
| [NEW] | `.moai/config/sections/audit.yaml` | `audit.*` 설정 신설 |
| [NEW] | `deploy/docker-compose.yml` 항목 | MinIO 서비스 추가(local S3 호환) |

### 7.2 Modified

| Path | Change |
|------|--------|
| `cmd/usearch-api/handlers/synthesis.go` | query.submit/complete/fail + deep.start/complete/fail 이벤트 발신 hook 추가 |
| `cmd/usearch-api/main.go` | `audit.New(cfg, obs, pgPool, asynqClient, s3Client, redisClient)` 초기화 + 4개 scheduled job 등록(`audit.litellm_reconcile`, `audit.export`, `audit.cleanup`, `audit.chain_verify`) + admin routes wiring |
| `internal/obs/metrics/metrics.go` | `registerAudit(r)` 헬퍼 호출 추가 |
| `internal/obs/obs.go` | audit collector re-export |
| `internal/obs/metrics/metrics_test.go` | `TestNoUnboundedLabels` 화이트리스트에 `event_type`, `decision`, `source`, `outcome` 라벨 추가 |
| `internal/obs/log/handler.go` | DEEP-004 decision log handler chain에 audit tee handler 추가 |
| `internal/index/index.go` | `Index.Upsert` / `Index.Delete` 결과를 audit emitter로 발신(wrapper 패턴, IDX-001 surface 변경 없음) |
| `.env.example` | `AUDIT_S3_*`, `AUDIT_HASH_CHAIN_ENABLED` 등 신규 env-var 문서화 |

### 7.3 Existing — Unchanged (forward-compat)

- `internal/deepagent/costguard/middleware.go::emitDecisionEvent` — DEEP-004
  forward-compat: 본 SPEC은 새 emit point를 추가하지 않고 slog handler tee로만 통합.
- `internal/deepagent/costguard/ledger.go::WriteLedgerEntry` — DEEP-004 forward-compat:
  cost.recorded mirror는 PG trigger로 자동화, Go 코드 변경 0.
- `internal/deepagent/costguard/` 모든 파일 — DEEP-004 코드는 변경되지 SHALL NOT.
- `deploy/postgres/migrations/0002_cost_ledger.sql` — cost_ledger schema 변경 없음.
- `pkg/types/*` — NormalizedDoc 등 contract 변경 없음.

---

## 8. Open Questions

본 SPEC은 §1.1의 7개 pinned decision으로 대부분의 ambiguity를 해소했다. 다음
항목은 plan-auditor와의 협의 또는 첫 운영 데이터 기반 튜닝이 필요한 경계 사례다.

1. **Reconciliation 주기**: 5분이 충분한지, LiteLLM `/spend/logs` 호출 빈도와
   API rate limit의 trade-off. **권장**: V1 5분 유지, 첫 30일 운영 후 lag 측정 결과로
   3-15분 범위에서 조정.
2. **Hash chain default**: V1 OFF가 보수적인지, ON이 운영 부담인지. **권장**: OFF
   유지(advisory lock contention risk). 컴플라이언스 요건이 있는 deploy만 ON.
3. **S3 export default**: V1 opt-in이 맞는지. **권장**: opt-in 유지. S3 credential
   설정이 없는 self-host 환경의 first-run UX 보호.
4. **`audit.read` permission scope**: admin만 vs. team owner도 자기 team audit 조회
   가능. **권장**: 두 permission으로 분리(`audit.read.team`, `audit.read.global`).
   AUTH-002 amendment에서 결정.
5. **Cost reconciled의 cap 평가 산입**: V1은 No (cap 평가는 cost_ledger만). 운영자가
   Python 호출도 cap에 산입을 원하면? **권장**: SPEC-AUTH-004(post-V1)로 위임.

위 5개는 plan-auditor가 SPEC을 PASS로 평가하기 위해 필수적인 결정이 아니다. 모두
first-30-day 운영 데이터로 튜닝 가능한 항목이다.

---

*End of SPEC-AUTH-003 v0.1.0 (status: draft; pending plan-auditor cycle).*
