# SPEC-AUTH-003 Acceptance Scenarios

Generated: 2026-05-22
Format: Given / When / Then (Korean prose, English identifiers)

본 문서는 SPEC-AUTH-003의 acceptance scenarios 8건 + boundary edge case 1건을
정의한다. 각 시나리오는 spec.md §5에서 참조되며 plan-auditor가 SPEC PASS 평가의
근거로 사용한다.

---

## §5.1 user login → audit_events 행 1개 (auth.login, decision="allow")

**Coverage**: REQ-AUTH3-001, REQ-AUTH3-002, REQ-AUTH3-009

### Given

- AUTH-001 JWT 미들웨어가 활성화되어 `Authorization: Bearer <jwt>` 헤더를 처리.
- 호출자가 유효한 JWT(sub claim = "alice@example.com", team_id claim = "engineering")를
  제출.
- audit_events 테이블이 비어 있는 상태(테스트 fixture).
- `audit.async: true` (기본값).

### When

호출자가 `POST /auth/login` 요청을 보낸다(혹은 protected endpoint 첫 호출에서 JWT
가 검증된다).

### Then

- HTTP 응답 status는 **200 OK**.
- AUTH-001 핸들러가 `audit.EmitEvent(ctx, AuditEvent{event_type:"auth.login",
  decision:"allow", user_id:"alice@example.com", team_id:"engineering",
  payload:{method:"oidc", provider:"keycloak", session_id, jwt_jti}})`를 호출.
- 직후 Asynq queue에 INSERT 작업 enqueue.
- Asynq worker가 audit_events에 row 1개 추가:
  - `event_type = "auth.login"`
  - `decision = "allow"`
  - `user_id = "alice@example.com"`
  - `tenant_id = "default"` (기본값)
  - `team_id = "engineering"`
  - `source = "go"`
  - `ts ≈ NOW()` (이벤트 발신 시점 ± 200ms)
  - `payload.method = "oidc"`, `payload.provider = "keycloak"`
- 카운터 `usearch_audit_events_total{event_type="auth.login",decision="allow",source="go"}`
  가 1 증가.
- Histogram `usearch_audit_write_duration_seconds`가 1회 observation 추가.
- slog handler chain의 audit tee handler가 INFO 레벨 record 1개 emit.

---

## §5.2 RBAC deny → audit_events 행 1개 (rbac.deny, decision="deny")

**Coverage**: REQ-AUTH3-002

### Given

- AUTH-002 Casbin enforcer가 활성화되어 모든 핸들러 진입 전에 RBAC 평가.
- Policy: alice@example.com은 `engineering` team 멤버이지만 `legal` team의 query는
  접근 불가.
- 호출자가 `legal` team으로 scoped된 query를 시도.

### When

`POST /query?team=legal&q=...` 호출.

### Then

- HTTP 응답 status는 **403 Forbidden**.
- AUTH-002 enforcer가 `audit.EmitEvent(ctx, AuditEvent{event_type:"rbac.deny",
  decision:"deny", user_id:"alice@example.com", tenant_id:"default",
  team_id:"engineering", resource:"team:legal", action:"query.read",
  payload:{policy_id, subject:{user_id, team_id}, casbin_match}})`를 호출.
- audit_events에 row 1개 추가, `event_type="rbac.deny"`, `decision="deny"`.
- `resource = "team:legal"`, `action = "query.read"`.
- 카운터 `usearch_audit_events_total{event_type="rbac.deny",decision="deny",source="go"}`
  1 증가.
- `query.submit` 또는 `query.complete` 이벤트는 발신되지 SHALL NOT (RBAC가 핸들러
  실행 자체를 차단했으므로).

---

## §5.3 Python LiteLLM 호출 → reconciliation 후 cost.reconciled 행, cost_ledger 변경 없음

**Coverage**: REQ-AUTH3-003, NFR-AUTH3-003, NFR-AUTH3-005

### Given

- `services/researcher/` Python 사이드카가 `claude-haiku-4-5` 모델을 LiteLLM 프록시
  로 호출. 응답: 800 input tokens, 300 output tokens, $0.00184 spend.
- LiteLLM proxy의 spend log에 다음 row가 기록됨:
  ```json
  {
    "request_id": "litellm-req-xyz999",
    "call_type": "acompletion",
    "model": "claude-haiku-4-5",
    "prompt_tokens": 800,
    "completion_tokens": 300,
    "spend": 0.00184,
    "metadata": {
      "user_api_key_hash": "abc123...",
      "spend_logs_metadata": {}
    }
  }
  ```
- 직전 reconciliation cycle은 T-5min에 종료, last_run_ts = T-5min.
- Go-side `cost_ledger`에는 동일 `request_id = "litellm-req-xyz999"`가 존재하지 않음.
- audit_events에도 `litellm_request_id = "litellm-req-xyz999"`가 존재하지 않음.
- 현재 시각 T에 Asynq scheduled job `audit.litellm_reconcile` 발화.

### When

Asynq job이 다음 작업을 수행:
1. LiteLLM `GET /spend/logs?summarize=false&start_date=T-6min&end_date=T` (1min overlap).
2. 응답 row 파싱.
3. dedup query: `SELECT 1 FROM cost_ledger WHERE request_id = $1` → 없음.
4. dedup query: `SELECT 1 FROM audit_events WHERE payload->>'litellm_request_id' = $1` → 없음.
5. INSERT into audit_events.

### Then

- audit_events에 row 1개 추가:
  - `event_type = "cost.reconciled"`
  - `source = "python"`
  - `user_id = "anonymous"` (LiteLLM이 user identity를 제공하지 않은 경우 fallback)
  - `payload = {"litellm_request_id":"litellm-req-xyz999", "model":"claude-haiku-4-5",
                "prompt_tokens":800, "completion_tokens":300, "spend_usd":0.00184,
                "call_type":"acompletion", "metadata":{...}}`
- **`cost_ledger` 테이블은 변경되지 않음** (row 추가 0, schema 변경 0).
- 카운터 `usearch_audit_reconcile_polls_total{outcome="success"}` 1 증가.
- 카운터 `usearch_audit_events_total{event_type="cost.reconciled",decision="none",source="python"}`
  1 증가.
- Gauge `usearch_audit_reconcile_lag_seconds` 갱신.

### Idempotency verification

- 다음 reconciliation cycle(T+5min)이 같은 spend log row를 받아도 dedup에 의해
  audit_events에 새 row가 추가되지 SHALL NOT 한다.
- cumulative drift 검증 job(weekly)이 직전 7일 동안 audit_events.cost.reconciled
  row 수와 LiteLLM `/spend/logs` 누적 row 수(dedup 후) 사이의 차이를 ≤ 0.5%로
  유지함을 확인.

---

## §5.4 admin 과거 query 재실행 → 새 query.submit + admin.replay 이벤트

**Coverage**: REQ-AUTH3-004, REQ-AUTH3-009

### Given

- 24시간 전 alice@example.com이 `POST /query` 호출, request_id="req_orig_001",
  query="quantum computing 2025 milestones".
- audit_events에 다음 row 존재:
  - `request_id="req_orig_001"`, `event_type="query.submit"`,
    `user_id="alice@example.com"`, `payload={query:{text:"quantum...", lang:"en"}}`
- admin user bob@example.com이 AUTH-002 policy에서 `audit.replay` permission 보유.
- bob의 직전 replay 호출은 5분 전(rate limit `1/min` 통과 가능).

### When

bob이 `POST /admin/audit/replay` 요청:
- Authorization: Bearer <bob's JWT>
- Body: `{"request_id":"req_orig_001"}`

### Then

- HTTP 응답 status는 **200 OK**.
- 응답 본문: `{"new_request_id":"req_replay_001","status":"submitted"}`.
- audit_events에 두 개 row 추가:
  1. `event_type="admin.replay"`, `decision="allow"`, `user_id="bob@example.com"`,
     `payload={original_request_id:"req_orig_001", new_request_id:"req_replay_001",
                replay_actor:"bob@example.com", query_sha256:"..."}`
  2. `event_type="query.submit"`, `user_id="bob@example.com"`,
     `request_id="req_replay_001"`, `payload={query:{text:"quantum...", lang:"en"},
     replayed_from:"req_orig_001", replayed_by:"bob@example.com"}`
- 원본 query.submit row의 `payload`에는 `replayed_by_actors:["bob@example.com"]`
  배열이 append되지 SHALL NOT (audit_events는 append-only이므로 원본 row는 변경 없음).
  대신 새 admin.replay 이벤트의 payload에 cross-reference 기록.
- 카운터 `usearch_audit_replay_requests_total{outcome="allowed"}` 1 증가.
- 새 query 실행의 user_id는 bob@example.com이며 alice의 credential은 사용되지
  SHALL NOT 한다. 새 query는 bob의 RBAC scope 안에서만 동작.

---

## §5.5 non-admin replay 시도 → 403, audit.replay decision="deny"

**Coverage**: REQ-AUTH3-004

### Given

- carol@example.com은 일반 team 멤버, `audit.replay` permission 미보유.
- 본인의 JWT로 audit replay endpoint 호출 시도.

### When

`POST /admin/audit/replay` with carol's JWT, body `{"request_id":"req_orig_001"}`.

### Then

- HTTP 응답 status는 **403 Forbidden**.
- 응답 본문: `{"error":"admin_role_required"}`.
- audit_events에 row 1개 추가:
  - `event_type="admin.replay"`
  - `decision="deny"`
  - `user_id="carol@example.com"`
  - `payload={original_request_id:"req_orig_001", denied_reason:"missing_permission_audit_replay"}`
- 카운터 `usearch_audit_replay_requests_total{outcome="denied"}` 1 증가.
- 원본 `req_orig_001` query는 재실행되지 SHALL NOT.

---

## §5.6 weekly S3 export + retention cleanup

**Coverage**: REQ-AUTH3-005, REQ-AUTH3-007, REQ-AUTH3-008, NFR-AUTH3-001, NFR-AUTH3-004

### Given

- `audit.s3.enabled: true`, `audit.s3.bucket = "test-audit"`, MinIO local endpoint.
- `audit.export.older_than_days: 7`.
- `audit.retention.hot_days: 90`, `audit.retention.require_s3_archive: true`.
- audit_events 테이블에 partition `audit_events_y2026m02`(2월) 존재, 약 180K rows.
- `audit_partitions.archived_at IS NULL` (아직 export 안 됨).
- 현재 시각 = 2026-05-22T02:00:00Z (일요일 02:00, cron `0 2 * * 0` 발화 시각).

### When

(a) `audit.export` job 발화 + (b) `audit.cleanup` job (다음 03:30) 발화.

### Then

**(a) export job**:

- Streaming SELECT에서 audit_events_y2026m02의 모든 row 추출.
- JSONL.gz로 직렬화, S3 PUT to
  `s3://test-audit/audit/default/year=2026/month=02/day=01/events-audit_events_y2026m02.jsonl.gz`.
- `audit_partitions` row 갱신: `archived_at = NOW()`.
- audit_events에 `event_type="audit.export"`, payload={s3_uri, partition_range:
  ["2026-02-01","2026-03-01"], row_count:180000, bytes_compressed:35MB} row 추가.
- 카운터 `usearch_audit_s3_export_rows_total += 180000`.
- 카운터 `usearch_audit_s3_export_bytes_total += 35000000`.
- Histogram `usearch_audit_s3_export_duration_seconds` p95 ≤ 300s (NFR-AUTH3-004).

**(b) cleanup job (90일 = 2026-02-21 이전 partition만 대상)**:

- `audit_events_y2026m02`는 `2026-02-01 → 2026-03-01` 범위. cutoff date(2026-02-21)
  이전 시작 → 대상.
- `audit_partitions.archived_at IS NOT NULL` 확인 → DROP 자격.
- `audit_admin` role connection으로 `ALTER TABLE audit_events DETACH PARTITION
  audit_events_y2026m02; DROP TABLE audit_events_y2026m02;` 실행.
- audit_events에 `event_type="audit.partition_drop"`, payload={partition_name:
  "audit_events_y2026m02", row_count:180000, range_start:"2026-02-01",
  range_end:"2026-03-01", archived: true} row 추가.
- 카운터 `usearch_audit_partition_drop_total` 1 증가.

### Failure scenarios

- S3 PUT 실패 시: Asynq 재시도 3회(1m/5m/15m). 모두 실패하면 `archived_at`은 NULL
  로 남음. cleanup job은 archived가 NULL인 partition을 SKIP (`require_s3_archive:
  true`이므로).
- `audit_admin` role 부재 시 cleanup job은 fatal error 발생, partition DROP되지
  SHALL NOT (NFR-001 append-only invariant 보호).

---

## §5.7 PII masking ON → query.text → sha256, replay 400

**Coverage**: REQ-AUTH3-006

### Given

- `audit.pii.mask_query_text: true` (운영자 명시 활성화).
- alice@example.com이 query 호출: text="aspirin overdose treatment", lang="en".

### When

`POST /query?q=aspirin+overdose+treatment&lang=en` 호출.

### Then

(a) query.submit 이벤트 발신:

- audit_events에 row 1개 추가:
  - `event_type="query.submit"`
  - `user_id="alice@example.com"` (masking 적용 SHALL NOT — identity field)
  - `payload={query:{text_sha256:"<sha256('aspirin overdose treatment')>","lang":"en"}}`
  - 원본 `payload.query.text`는 존재 SHALL NOT.

(b) 24시간 후 admin bob이 replay 시도:

- `POST /admin/audit/replay` body `{"request_id":"req_xxxx"}`.
- 핸들러가 audit_events 조회 → payload.query.text 부재 확인.
- HTTP 응답 status **400 Bad Request**, body `{"error":"query_text_masked",
  "detail":"original query unavailable for replay"}`.
- audit_events에 `event_type="admin.replay"`, `decision="deny"`,
  `payload.denied_reason="query_text_masked"` 1 row 추가.
- 카운터 `usearch_audit_replay_requests_total{outcome="error"}` 1 증가.

### IP masking sub-scenario

- `audit.pii.mask_ip: true` 추가 설정 시 모든 audit_events row의 `ip` 컬럼은
  NULL로 강제 INSERT.

---

## §5.8 DEEP-004 cap.evaluation stderr line이 audit_events에 mirror, DEEP-004 코드 변경 없음

**Coverage**: REQ-AUTH3-002, spec.md §1.3 forward-compat

### Given

- DEEP-004(SPEC-DEEP-004) implemented & deployed. `/deep` 호출이 cap 초과되어
  decision event log JSON line이 stderr로 emit됨.
- AUTH-003 audit tee handler가 slog handler chain에 등록됨.
- DEEP-004 emit code(`internal/deepagent/costguard/middleware.go::emitDecisionEvent`)
  는 SINGLE LINE 변경 없음.

### When

호출자가 cap 초과된 `/deep` 호출.

DEEP-004는 다음 stderr line emit:

```json
{
  "timestamp":"2026-05-22T13:45:00.123Z",
  "event_type":"cap.evaluation",
  "request_id":"req_capped_001",
  "tenant_id":"default",
  "user_id":"alice@example.com",
  "decision":"deny",
  "dimension":"calls",
  "remaining":{"calls":0,"usd":4.23},
  "screen_score":7,
  "cache_hit":false
}
```

### Then

- stderr에 JSON line이 정상 출력됨(DEEP-004 기존 동작 보존).
- audit tee handler가 같은 line을 parse하여 audit_events에 row 1개 추가:
  - `event_type="cap.evaluation"`
  - `request_id="req_capped_001"`
  - `tenant_id="default"`
  - `user_id="alice@example.com"`
  - `decision="deny"`
  - `source="go"`
  - `ts = "2026-05-22T13:45:00.123Z"` (DEEP-004 timestamp 그대로)
  - `payload = {"dimension":"calls", "remaining":{"calls":0,"usd":4.23},
                "screen_score":7, "cache_hit":false}`
- DEEP-004의 6 mandatory 필드(`timestamp`, `event_type`, `request_id`, `tenant_id`,
  `user_id`, `decision`)가 audit_events row의 column으로 1:1 매핑됨.
- `internal/deepagent/costguard/` 모든 파일의 변경 없음 검증 (forward-compat 약속).
- `cost_ledger` schema 변경 없음.

### cost.recorded mirror (companion test)

- DEEP-004 cost_ledger row INSERT 시 Postgres trigger `cost_ledger_to_audit`이
  발화하여 audit_events에 `event_type="cost.recorded"`, payload.cost_ledger_id=
  NEW.id, payload에 model/tokens/usd_cost/cache_hit 복사 row 추가.
- cost_ledger row 자체는 변경되지 SHALL NOT (mirror only).

---

## Edge Case — hash chain enabled, 동시 INSERT 100건 → prev_hash 정확 연결

**Coverage**: REQ-AUTH3-008, NFR-AUTH3-007

### Given

- `audit.hash_chain.enabled: true` (운영자 명시 활성화).
- tenant_id="default"의 audit_events에 5개 row 존재. 마지막 row의 `this_hash`는
  `"h_05"`(예시).
- 100개 goroutine이 동시에 `EmitEvent(ctx, AuditEvent{tenant_id:"default",
  event_type:"query.submit", ...})` 호출.

### When

`go test -race`로 100 goroutine 병렬 INSERT 실행.

### Then

- 모든 INSERT는 `pg_advisory_xact_lock(hashtext("default"))` advisory lock으로
  serialize됨.
- 100개 row가 순차적으로 INSERT되며 각 row의 `prev_hash`는 직전 row의 `this_hash`
  로 정확히 연결:
  - row 6: prev_hash = "h_05", this_hash = SHA256("h_05" || canonical_json(row_6))
  - row 7: prev_hash = "h_06" (= row 6's this_hash), this_hash = SHA256(...)
  - ...
  - row 105: prev_hash = "h_104", this_hash = SHA256(...)
- `go test -race`는 race condition 검출 0건.
- daily verify job `audit.chain_verify` 실행 시 모든 row의 hash가 valid로 판정 →
  `usearch_audit_chain_violations_total` 0.
- 90일 hot retention 전체(약 600K-2M rows) verify가 ≤ 30분 내 완료(NFR-007).

### canonical_json stability

- canonical_json은 key-sorted JSON encoding. 같은 row를 두 번 serialize해도
  byte-equal output 보장 (golden test로 검증).
- map iteration의 비결정성을 막기 위해 key를 명시적 sorted slice로 처리.

### Tamper detection

- 운영자가 superuser 권한으로 `UPDATE audit_events SET payload = ... WHERE id = 50;`
  실행 (trigger 우회 가정).
- 다음 daily verify 실행 시 row 50의 `this_hash`가 재계산된 SHA256과 불일치 →
  `usearch_audit_chain_violations_total` 1 증가, WARN slog record emit.

---

## Acceptance Coverage Matrix

| Scenario | REQ-001 | REQ-002 | REQ-003 | REQ-004 | REQ-005 | REQ-006 | REQ-007 | REQ-008 | REQ-009 |
|----------|---------|---------|---------|---------|---------|---------|---------|---------|---------|
| §5.1 | ✓ | ✓ |   |   |   |   |   |   | ✓ |
| §5.2 |   | ✓ |   |   |   |   |   |   |   |
| §5.3 |   |   | ✓ |   |   |   |   |   | ✓ |
| §5.4 |   |   |   | ✓ |   |   |   |   | ✓ |
| §5.5 |   |   |   | ✓ |   |   |   |   |   |
| §5.6 |   |   |   |   | ✓ |   | ✓ | ✓ | ✓ |
| §5.7 |   |   |   |   |   | ✓ |   |   |   |
| §5.8 |   | ✓ |   |   |   |   |   |   |   |
| Edge |   |   |   |   |   |   |   | ✓ |   |

NFR coverage:

- NFR-001: §5.1 (append-only invariant via trigger + role)
- NFR-002: §5.1 (write latency budget)
- NFR-003: §5.3 (reconciliation freshness)
- NFR-004: §5.6 (S3 export throughput)
- NFR-005: §5.3 (reconciliation drift)
- NFR-006: §5.1, §5.2 (no PII in metric labels — verified by TestNoUnboundedLabelsAuditExtended)
- NFR-007: Edge (hash chain verification budget)
- NFR-008: §5.1 (cardinality safety — event_type enum)

---

*End of SPEC-AUTH-003 acceptance.md.*
