# SPEC-AUTH-003 Deep Research

Generated: 2026-05-22T00:00:00Z
Author: manager-spec (Phase 0.5 — context-derived)
Consumed by: manager-spec (Phase 1B), plan-auditor (Phase 2.3)

---

## 0. Scope of This Research

본 research.md는 M6 milestone의 Team-plane deliverable인 SPEC-AUTH-003
(`Audit log`)의 코드베이스 분석 + 아키텍처 결정 기록이다. roadmap.md §M6의 1줄 scope ("Postgres audit table, query replay, optional S3 export") 을
(a) immutable audit event store 스키마, (b) DEEP-004 decision event log 통합,
(c) Python-side LLM cost 회수(LiteLLM spend logs), (d) admin-only replay 엔드포인트,
(e) S3 export job, (f) PII masking + retention + tamper resistance 의 6개 축으로
분해해 각 축의 코드 진입점, 환경 변수, 메트릭, 스토리지, 보안 영향을 명세화한다.

본 SPEC은 M6의 Team-plane 세 SPEC(AUTH-001/002/003) 중 가장 downstream에 있는
"관찰 단" deliverable이다. AUTH-001(OIDC SSO)이 user identity를 채우고
AUTH-002(Team RBAC)가 decision을 만들며 AUTH-003은 그 두 단계의 결과 + 모든
data-plane 호출(query / deep / index write)을 단일 immutable trail에 기록한다.

또한 본 SPEC은 SPEC-DEEP-004(M5)가 명시적으로 deferred한 두 항목의 정착지다.

- DEEP-004 spec.md §6.3 (Forward-compatibility commitment with AUTH-001 and AUTH-003):
  "REQ-DEEP4-010의 decision event log JSON line schema 또한 SPEC-AUTH-003(M6) audit
  subsystem이 downstream consumer로 합류할 때 호환되도록 설계된다... schema는
  **additive**이며 SPEC-AUTH-003는 새 필드를 추가할 수 있으나 위 필드를 rename하거나
  remove할 수 SHALL NOT 한다. AUTH-003 진입 시 audit subsystem review checkpoint가
  본 commitment를 재검증한다."
- DEEP-004 spec.md §4 Exclusions 항목 8: "Python-side LiteLLM 호출의 ledger 적용
  없음... LiteLLM 프록시 자체의 spend logs로 후속 reconciliation(SPEC-AUTH-003 M6)."
- DEEP-004 plan.md §4 R5: "Python-side LLM cost 누락... SPEC-AUTH-003 (M6) audit
  log reconciliation으로 후속 처리."

본 SPEC은 위 두 약속을 동시에 정착시키되, DEEP-004의 기존 `cost_ledger` 스키마와
decision event log 필드를 변경하지 않는 **additive** 설계를 채택한다.

---

## 1. Audit Surface Analysis

### 1.1 What events does V1 need to audit

product.md "Team mode with 5 users: per-user quotas enforced, shared index dedup
hits ≥30%, audit log complete" 와 roadmap.md M6 exit criterion이 요구하는
"audit log 완전성"을 다음 event 분류로 분해한다.

| Category | Event types | Source | Frequency (per-team/day, 5-user) |
|----------|-------------|--------|----------------------------------|
| Identity | `auth.login`, `auth.logout`, `auth.fail`, `auth.token.refresh` | AUTH-001 JWT middleware | ~50 |
| Authorization | `rbac.allow`, `rbac.deny`, `rbac.policy_change` | AUTH-002 Casbin enforcer | ~500 (per query × adapters) |
| Data plane — query | `query.submit`, `query.complete`, `query.fail` | `cmd/usearch-api/handlers/synthesis.go` | ~200 |
| Data plane — /deep | `deep.start`, `deep.complete`, `deep.fail` | DEEP-002 4-agent pipeline | ~30 |
| Cost / quota | `cap.evaluation` (alias of DEEP-004 decision event log) | costguard middleware | ~30 + caps |
| Cost / quota | `cost.recorded` (mirrors `cost_ledger` insert) | costguard ledger.go | ~600 (per LLM call) |
| Cost / quota | `cost.reconciled` (Python-side LiteLLM spend reconciliation) | new `audit_reconcile` job | ~600 (1:1 with Python LLM calls) |
| Index | `index.write`, `index.delete`, `index.rebuild` | `internal/index/index.go` Upsert/Delete | ~5000 (ingest spikes) |
| Admin | `admin.config_change`, `admin.user_action`, `admin.replay` | new `/admin/*` handlers | ~10 |
| System | `audit.export`, `audit.chain_verify`, `audit.partition_drop` | new audit jobs | ~daily |

총 11 categories × multiple event_types = 약 **20개의 event_type**. label cardinality를
bounded enum으로 유지하기 위해 event_type은 startup time에 화이트리스트로 lock한다.

### 1.2 Volume estimate

5-user team, 100 queries/day, /deep 6 calls/day, ~600 LLM calls/day 가정:

- Identity events: 50 rows/day
- RBAC events: ~500 rows/day (per-query × per-adapter)
- Query events: 200 rows/day (submit + complete pair)
- /deep events: 30 rows/day
- Cap/cost events: ~1300 rows/day (DEEP-004 decision log + cost_ledger mirror + Python reconcile)
- Index events: ~5000 rows/day (적층 ingest 포함)
- Admin: ~10 rows/day

→ **~7000 rows/day/team** ≈ **210K rows/30d** ≈ **630K rows/90d hot retention**

row 평균 1-2 KB (payload JSONB 포함) → 90일 hot ≈ 1-2 GB per team. 10팀 deployments
≈ 10-20 GB. Monthly partition + index 합치면 약 30-60 GB. Postgres 16 + SSD 환경
에서 acceptable. partition drop으로 retention 종료 시 빠른 회수.

### 1.3 Volume risk: index events

`index.write` 단일 카테고리가 전체 audit volume의 ~70%를 차지한다. 옵션:

1. **All-in (v1)**: 모든 index write를 audit_events에 기록. 단순. partition drop으로
   회수.
2. **Sampling**: index.write는 10% sampling 또는 batch-summarize. complex.
3. **Skip**: index.write는 cost_ledger와 cost.recorded 이벤트로 갈음.

→ **결정 (D4 in §7)**: V1은 옵션 1(All-in) 채택. 운영자가
`audit.events.index_write_enabled: false`로 toggle 가능. roadmap M8 SPEC-EVAL-002에서
volume metric 기반 sampling 도입을 재평가.

### 1.4 Schema unification vs cross-reference (DEEP-004 cost_ledger와의 관계)

DEEP-004는 이미 `cost_ledger` 테이블을 운영한다(2026-05-22 implemented v0.1.1).
같은 이벤트(예: cost.recorded)를 두 곳에 기록할지가 핵심 설계 결정이다.

| Option | Pros | Cons |
|--------|------|------|
| **A. 통합** — `cost_ledger`를 audit_events 안에 흡수 (drop table + view) | single source of truth | DEEP-004 코드 변경, 마이그레이션 risk, downtime |
| **B. 이중 쓰기** — app-side가 cost_ledger insert + audit_events insert 둘 다 | both tables full | drift risk, write 비용 2x |
| **C. Trigger 자동 mirror** — Postgres trigger가 cost_ledger insert를 audit_events에 자동 복사 | DEEP-004 코드 변경 0, atomic | trigger 로직 hidden, debugability ↓ |
| **D. Cross-reference only** — audit_events는 cost_ledger를 가리키는 thin reference만 (event_type="cost.recorded", payload={cost_ledger_id: N}) | minimal schema overlap, DEEP-004 코드 변경 0 | join 필요, full audit dump이 multi-table query |

→ **결정 (D2 in §7)**: 옵션 D 채택. AUTH-003은 cost_ledger를 변경하지 SHALL NOT 하며,
audit_events row의 `payload`에 `cost_ledger_id` reference만 저장한다. cap.evaluation
같은 transient event(현재 stderr JSON line으로만 emit)는 audit_events에 직접 저장.
S3 export 시 cost_ledger join은 export job에서 수행하여 단일 JSONL 라인을 만든다.

이 결정은 DEEP-004 spec.md §6.3 forward-compat commitment(schema additive only)와
정합한다.

---

## 2. DEEP-004 Forward-Compatibility Audit

### 2.1 Decision event log 통합 경로

DEEP-004 REQ-DEEP4-010이 정의한 stderr JSON line schema:

```json
{
  "timestamp": "2026-05-21T13:45:00.123Z",
  "event_type": "cap.evaluation",
  "request_id": "req_abc123",
  "tenant_id": "default",
  "user_id": "anonymous",
  "decision": "allow|deny|degrade",
  "dimension": "calls|usd|none",
  "remaining": { "calls": 12, "usd": 4.23 },
  "screen_score": 7,
  "cache_hit": false
}
```

AUTH-003 통합 방식:

1. DEEP-004 emit point(`internal/deepagent/costguard/middleware.go::emitDecisionEvent`)
   는 그대로 유지. stderr JSON line이 primary delivery channel로 남는다.
2. AUTH-003은 **별도의 slog handler** (`internal/audit/decision_log_handler.go`)
   를 등록하여 같은 line을 audit_events에 mirror한다. 등록 시 stderr write는 보존
   되며 audit table write가 추가된다(tee 패턴).
3. JSON line의 모든 필드(timestamp, event_type, request_id, tenant_id, user_id,
   decision)는 audit_events row의 first-class column 또는 payload로 매핑:
   - `timestamp` → `audit_events.ts`
   - `event_type` → `audit_events.event_type` (값은 그대로 "cap.evaluation")
   - `request_id` → `audit_events.request_id`
   - `tenant_id` → `audit_events.tenant_id`
   - `user_id` → `audit_events.user_id`
   - `decision` → `audit_events.decision`
   - `dimension`, `remaining`, `screen_score`, `cache_hit` → `audit_events.payload` JSONB
4. DEEP-004는 schema를 변경하지 SHALL NOT 한다. AUTH-003은 새 필드를 payload에만
   추가할 수 있다(additive).

### 2.2 cost_ledger row mirror (cost.recorded events)

DEEP-004 cost_ledger 행 생성 경로(`internal/deepagent/costguard/ledger.go::WriteLedgerEntry`)
에 audit hook을 추가한다. 옵션:

- **Postgres trigger**: `AFTER INSERT ON cost_ledger FOR EACH ROW EXECUTE FUNCTION
  mirror_to_audit_events()`. PG-side만 변경 → DEEP-004 Go 코드 변경 0.
- **App-side hook**: ledger.go에 audit_events INSERT 호출 추가. DEEP-004 코드 변경
  (1줄 추가 후 import).

→ **결정 (D5)**: Postgres trigger 채택. DEEP-004 코드 변경 0이 forward-compat
commitment를 가장 깔끔하게 보존한다. trigger는 audit_events에 `event_type="cost.recorded"`,
`payload={cost_ledger_id: NEW.id, model: NEW.model, usd_cost: NEW.usd_cost, ...}`
row를 추가한다. trigger 실패 시 cost_ledger INSERT를 RAISE EXCEPTION으로 abort
(append-only invariant 유지).

### 2.3 Python-side LiteLLM cost reconciliation

DEEP-004 Exclusion 8 + R5가 미해결로 남긴 Python-side 호출 비용을 회수한다.

**현재 상태**: `services/researcher/` (gpt-researcher 사이드카), `services/storm/`
(STORM 사이드카)는 직접 LiteLLM SDK 호출 → LiteLLM proxy에 spend log만 남기고
Go-side `cost_ledger`에는 기록되지 않는다.

**LiteLLM spend logs API** (verified 2026-05-22 via docs.litellm.ai):

```
GET /spend/logs?summarize=false&start_date=2026-05-21&end_date=2026-05-22
Response: [{request_id, call_type, model, prompt_tokens, completion_tokens,
            spend, metadata: {user_api_key, ...}}, ...]
```

추가 endpoints:
- `GET /global/spend/report?group_by=customer&api_key=sk-1234` — aggregated
- `GET /user/info?user_id=<id>` — per-user spend aggregation

**Reconciliation 메커니즘**:

옵션 A: webhook (LiteLLM이 push) → docs에 webhook 미지원 명시.
옵션 B: scheduled polling (Go Asynq job, 5min interval).
옵션 C: Postgres direct read (LiteLLM이 DB 사용 시 같은 PG에 접근) → vendor lock-in,
LiteLLM internal schema 변경 risk.

→ **결정 (D3)**: 옵션 B 채택. 새 Asynq scheduled job
`internal/audit/litellm_reconcile.go`가 5분마다 `/spend/logs` polling. 직전 polling
이후의 윈도우(start_date, end_date)를 계산하고, request_id 기준 dedup(Go-side
cost_ledger에 이미 기록된 행은 skip), 새 row를 audit_events에 `event_type="cost.reconciled"`,
`payload={source:"python", litellm_request_id, model, prompt_tokens, completion_tokens,
spend_usd}` 로 기록.

NOTE: Python-side 호출은 cost_ledger에 기록하지 SHALL NOT 한다. cost_ledger는
Go-side `llm.Client` 호출의 single source of truth로 유지(DEEP-004 약속). Python 비용은
audit_events에만 기록되며, cap 평가에는 사용되지 SHALL NOT 한다(DEEP-004 cap-check가
cost_ledger만 본다는 기존 설계 보존). cap 평가 통합은 후속 SPEC-AUTH-004(post-V1)이
다룰 문제.

### 2.4 Schema additive guarantee verification

DEEP-004 spec.md §6.3는 다음 필드의 rename/remove를 금지한다:
`timestamp`, `event_type`, `request_id`, `tenant_id`, `user_id`, `decision`.

AUTH-003 audit_events table은 위 6개 필드를 모두 dedicated column으로 가진다:

```sql
CREATE TABLE audit_events (
    ...
    ts          TIMESTAMPTZ  NOT NULL,        -- maps to "timestamp"
    event_type  TEXT         NOT NULL,
    request_id  TEXT,
    tenant_id   TEXT         NOT NULL DEFAULT 'default',
    user_id     TEXT         NOT NULL DEFAULT 'anonymous',
    decision    TEXT,                          -- nullable for non-decision events
    ...
);
```

AUTH-003이 추가하는 필드는 column이거나 payload JSONB의 key이며, 모두 DEEP-004
JSON line schema에 **새 key 추가**만 한다(기존 6 key는 1:1 매핑 보존). DEEP-004
emit code(stderr JSON line)는 변경 없음 — AUTH-003의 audit_events row는 stderr
line의 superset이다.

---

## 3. Audit Event Schema Design

### 3.1 Core table

`deploy/postgres/migrations/0003_audit_events.sql`:

```sql
CREATE TABLE IF NOT EXISTS audit_events (
    id              BIGSERIAL    NOT NULL,
    ts              TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    event_type      TEXT         NOT NULL,
    request_id      TEXT,
    tenant_id       TEXT         NOT NULL DEFAULT 'default',
    user_id         TEXT         NOT NULL DEFAULT 'anonymous',
    team_id         TEXT,                              -- from AUTH-002 (M6)
    resource        TEXT,                              -- e.g. "adapter:reddit", "doc:abc123"
    action          TEXT,                              -- verb: "read", "write", "delete", "submit"
    decision        TEXT,                              -- "allow" | "deny" | "degrade" | NULL
    deep_run_id     TEXT,                              -- correlates to /deep runs (nullable)
    source          TEXT         NOT NULL DEFAULT 'go', -- "go" | "python" | "system"
    ip              INET,                              -- requester IP (when available)
    user_agent      TEXT,                              -- HTTP UA
    payload         JSONB,                             -- event-specific data
    prev_hash       TEXT,                              -- previous row's this_hash (nullable when chain disabled)
    this_hash       TEXT,                              -- SHA256(prev_hash ‖ canonical_json(row_minus_hashes))
    schema_version  INT          NOT NULL DEFAULT 1,   -- additive evolution
    PRIMARY KEY (id, ts)
) PARTITION BY RANGE (ts);

-- Monthly partitions managed by audit.partitions job (Phase F)
-- Initial partition for the current month created at startup:
-- CREATE TABLE audit_events_y2026m05 PARTITION OF audit_events
--   FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');

CREATE INDEX IF NOT EXISTS idx_audit_events_tenant_user_ts
    ON audit_events (tenant_id, user_id, ts DESC);
CREATE INDEX IF NOT EXISTS idx_audit_events_type_ts
    ON audit_events (event_type, ts DESC);
CREATE INDEX IF NOT EXISTS idx_audit_events_request_id
    ON audit_events (request_id) WHERE request_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_audit_events_deep_run_id
    ON audit_events (deep_run_id) WHERE deep_run_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_audit_events_team_ts
    ON audit_events (team_id, ts DESC) WHERE team_id IS NOT NULL;
```

PRIMARY KEY가 `(id, ts)` composite인 이유: PARTITION BY RANGE (ts) 시 PK가 파티션
키를 포함해야 한다(Postgres 제약).

### 3.2 Append-only enforcement triggers

```sql
CREATE OR REPLACE FUNCTION audit_events_block_modify()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'audit_events is append-only; % blocked', TG_OP;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER audit_events_no_update
    BEFORE UPDATE ON audit_events
    FOR EACH ROW EXECUTE FUNCTION audit_events_block_modify();

CREATE TRIGGER audit_events_no_delete
    BEFORE DELETE ON audit_events
    FOR EACH ROW EXECUTE FUNCTION audit_events_block_modify();
```

NOTE: superuser 또는 `ALTER TABLE ... DISABLE TRIGGER`로 우회 가능하나 이는
운영자의 의도적 행위로 간주(예: partition drop은 trigger를 회피하기 위해 별도
DROP PARTITION 메커니즘 사용). 일반 application connection은 트리거 우회 권한이
없도록 별도 DB role(`audit_writer`)을 운영한다.

### 3.3 cost_ledger mirror trigger

```sql
CREATE OR REPLACE FUNCTION audit_mirror_cost_ledger()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO audit_events (
        ts, event_type, request_id, tenant_id, user_id,
        deep_run_id, source, action, decision, payload
    ) VALUES (
        NEW.ts,
        'cost.recorded',
        NEW.request_id,
        NEW.tenant_id,
        NEW.user_id,
        NEW.deep_run_id,
        'go',
        'llm.call',
        CASE WHEN NEW.outcome = 'success' THEN 'allow'
             WHEN NEW.outcome = 'capped'  THEN 'deny'
             ELSE NEW.outcome END,
        jsonb_build_object(
            'cost_ledger_id', NEW.id,
            'model', NEW.model,
            'prompt_tokens', NEW.prompt_tokens,
            'completion_tokens', NEW.completion_tokens,
            'usd_cost', NEW.usd_cost,
            'cache_hit', NEW.cache_hit,
            'intent_category', NEW.intent_category,
            'outcome', NEW.outcome
        )
    );
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER cost_ledger_to_audit
    AFTER INSERT ON cost_ledger
    FOR EACH ROW EXECUTE FUNCTION audit_mirror_cost_ledger();
```

trigger 실패는 cost_ledger INSERT를 abort. operationally tolerant하려면 trigger
내부에 `BEGIN ... EXCEPTION WHEN OTHERS THEN ...` block을 추가할 수 있으나, V1은
strict(write-or-fail) 모드를 default로 한다. 운영자가 `audit.cost_mirror_strict:
false`로 toggle 가능.

### 3.4 Hash chain (optional, default disabled)

```
this_hash = SHA256(prev_hash || canonical_json({
  id, ts, event_type, request_id, tenant_id, user_id,
  team_id, resource, action, decision, deep_run_id,
  source, ip, user_agent, payload, schema_version
}))
```

canonical_json은 key-sorted JSON encoding. `prev_hash`는 직전 row의 `this_hash`
(첫 row는 빈 문자열).

V1 default: `audit.hash_chain.enabled: false`. 활성화 시:

- Insert path는 app-side에서 prev_hash 조회 + 자체 SHA256 계산 후 INSERT.
- 동시 INSERT race를 막기 위해 `SELECT FOR UPDATE` 또는 advisory lock 필요.
  → 처리량 저하 trade-off. 따라서 default off.
- 별도 Asynq scheduled job `audit_chain_verify`(daily)가 모든 row를 순차 검증.
  위반 시 `audit_chain_violations_total` 카운터 증가 + 운영자 알람.

### 3.5 Payload schema by event_type (selected examples)

| event_type | payload structure |
|-----------|-------------------|
| `auth.login` | `{method:"oidc", provider:"keycloak", session_id, jwt_jti}` |
| `auth.fail` | `{method, reason:"invalid_signature"|"expired"|"unknown_user", provider}` |
| `rbac.deny` | `{policy_id, subject:{user_id, team_id}, action, resource, casbin_match:"..."}` |
| `query.submit` | `{query_text|sha256(query_text) if PII masked, mode:"basic"|"deep", lang}` |
| `query.complete` | `{result_count, latency_ms, sources:[..adapter ids]}` |
| `deep.start` | `{run_id, breadth, depth, query_sha256}` |
| `deep.complete` | `{run_id, total_nodes, total_cost_usd, status}` |
| `cap.evaluation` | DEEP-004 stderr JSON line의 superset (dimension, remaining, screen_score, cache_hit) |
| `cost.recorded` | trigger-populated (see §3.3) |
| `cost.reconciled` | `{source:"python", litellm_request_id, model, prompt_tokens, completion_tokens, spend_usd}` |
| `index.write` | `{doc_id, source_id, store_outcomes:{qdrant:"success", meili:"success", pg:"success"}}` |
| `admin.replay` | `{original_request_id, new_request_id, replay_actor, query_sha256}` |
| `audit.export` | `{s3_uri, partition_range:["2026-05-15","2026-05-22"], row_count, bytes_compressed}` |

---

## 4. Replay Endpoint Design

### 4.1 Purpose

운영자가 과거 쿼리를 현재 어댑터 상태에 대해 재실행하여 (a) 어댑터 회귀 검증,
(b) bug reproduction, (c) DeepEval 골든셋 큐레이션 등에 사용한다.

### 4.2 Endpoint contract

```
POST /admin/audit/replay
Authorization: Bearer <admin-token>
Body: {"request_id": "req_abc123"}
Response:
  200 OK: {"new_request_id":"req_xyz456","status":"submitted"}
  400 Bad Request: {"error":"unknown_request_id"|"event_not_replayable"}
  403 Forbidden: {"error":"admin_role_required"}
  429 Too Many Requests: {"error":"replay_rate_limit", "retry_after_seconds":60}
```

### 4.3 Security model

- **Auth**: AUTH-001 JWT 검증 + AUTH-002 RBAC `audit.replay` permission 필수.
- **Credential leak prevention**: audit_events는 사용자 credential을 저장 SHALL NOT
  한다. replay는 새 request로 실행되며, 새 request의 identity는 **replay actor**
  (admin)의 identity이지 original requester의 identity가 아니다. payload.metadata
  에 `original_request_id`, `replay_actor` 둘 다 기록.
- **Rate limit**: 1 replay/min per admin (Redis sliding window). cap-check 미들웨어
  재사용 가능.
- **Cost attribution**: replay 호출의 cost는 admin의 tenant에 산입(audit trail 명확성).
  운영자가 별도 tenant_id("system")로 분리하고 싶으면 deploy 시 config로 지정.

### 4.4 Replay-able events

- `query.submit` → 새 query.submit으로 재실행 가능.
- `deep.start` → 새 deep.start로 재실행 가능 (cap 평가 통과 시).
- 그 외 (auth.*, cap.evaluation, cost.*, admin.*) → not replayable, 400 응답.

### 4.5 Implementation

1. `POST /admin/audit/replay` 핸들러가 `audit_events` SELECT WHERE request_id=$1
   AND event_type IN ('query.submit', 'deep.start') 로 원본 이벤트 조회.
2. payload에서 query text / 모드 / lang 추출.
3. 새 request_id 생성 후 internal HTTP 또는 in-process call로 핸들러 재실행.
4. 원본/replay 두 이벤트 모두에 cross-reference 기록(`replayed_by`, `replayed_from`).

---

## 5. S3 Export Job

### 5.1 Trigger schedule

Weekly Asynq scheduled job `audit.export` (default Sunday 02:00 UTC):

1. 직전 N일(default 7d) 동안 archived가 아직 안된 partition을 식별.
2. 각 partition을 streaming SELECT으로 JSONL.gz로 변환.
3. S3 객체 path: `s3://{bucket}/audit/{tenant_id}/year=YYYY/month=MM/day=DD/events-{partition_name}.jsonl.gz`.
4. 업로드 성공 시 partition row에 `archived_at` 표시(별도 `audit_partitions` meta table).
5. emit audit_event(`event_type="audit.export"`, payload={s3_uri, partition_range, row_count, bytes}).

### 5.2 Format precedent

프로젝트 다른 곳의 데이터 export 패턴: SPEC-DEEP-004 cost_ledger는 export 정책이
없고, SPEC-IDX-001은 deferred. 명시적 precedent 없으므로 industry default
**JSONL.gz**(streaming, line-delimited JSON, gzip-compressed)를 채택. Parquet은
배제(쿼리 워크로드가 V1에 없고, 운영 부담 증가).

### 5.3 Configurability

`.moai/config/sections/audit.yaml`:

```yaml
audit:
  s3:
    enabled: false                      # opt-in
    bucket: "${AUDIT_S3_BUCKET}"
    prefix: "audit/"
    region: "${AWS_REGION:-us-east-1}"
    endpoint: "${AUDIT_S3_ENDPOINT}"     # for MinIO local override
    access_key_id: "${AUDIT_S3_ACCESS_KEY}"
    secret_access_key: "${AUDIT_S3_SECRET_KEY}"
  export:
    schedule_cron: "0 2 * * 0"           # weekly Sunday 02:00 UTC
    older_than_days: 7                   # archive partitions older than 7d
    delete_after_export: false           # if true, drop partition after S3 confirms
```

### 5.4 Failure handling

S3 upload 실패 시:

- Asynq retry policy: 3회 exponential backoff (1m, 5m, 15m).
- 모두 실패하면 partition은 `archived_at = NULL`로 남아 다음 주기에 재시도.
- emit audit_event `event_type="audit.export"` with `decision="deny"`, payload에 error.
- partition drop은 archived 확인 전까지 SHALL NOT 발생.

### 5.5 MinIO local-vs-AWS S3

V1은 두 환경 모두 지원:

- Local dev: docker-compose `minio` 서비스(예: `minio/minio:RELEASE.2024-12-13T22-19-12Z`),
  endpoint `http://minio:9000`, bucket auto-create on startup.
- AWS S3: standard AWS SDK. region + IAM 권한 사용.

코드 레벨에서는 `github.com/aws/aws-sdk-go-v2/service/s3` 단일 client로 양쪽 모두
다룸(MinIO도 S3 API 호환).

---

## 6. PII Masking + Retention + Tamper Resistance

### 6.1 PII masking

Default OFF. 운영자가 `audit.pii.mask_query_text: true`로 활성화하면:

- `query.submit`, `query.complete`, `deep.start`, `deep.complete` 이벤트의
  `payload.query.text`(있는 경우)가 `payload.query.text_sha256 = SHA256(text)`로 교체.
- 다른 필드(user_id, tenant_id, request_id)는 masking 대상 SHALL NOT.
- replay 엔드포인트는 sha256만 있는 경우 replay 불가(원본 텍스트 부재).

추가 toggle: `audit.pii.mask_ip: false` (default off; team-internal IP는 audit
가치가 PII 가치보다 크다고 판단).

### 6.2 Retention

기본 hot retention: 90일 in Postgres (DEEP-004 §1.1 D7와 동일).

```yaml
audit:
  retention:
    hot_days: 90                  # rows older are eligible for partition drop
    require_s3_archive: true       # do NOT drop until S3 export confirmed
```

- Nightly Asynq job `audit.cleanup`이 partition 단위로 drop.
- `require_s3_archive: true`이고 S3 export 미완료 partition은 SKIP.
- emit `event_type="audit.partition_drop"` event.

### 6.3 Tamper resistance

3-layer 방어:

1. **App-level role separation**: application connection은 SELECT/INSERT만 가능.
   UPDATE/DELETE 권한 없음. GRANT 정책으로 enforced.
2. **DB trigger** (§3.2): superuser가 아닌 한 UPDATE/DELETE 시 RAISE EXCEPTION.
3. **Hash chain** (§3.4, optional): cryptographic 무결성. tamper 발생 시 daily
   verify job이 검출 + 알람.

### 6.4 Backup considerations

audit_events는 daily logical backup(`pg_dump`) 대상. WAL replication 또는 PITR은
deploy SPEC(M9)에서 다룸. 본 SPEC은 backup 정책을 정의하지 SHALL NOT 하며,
S3 export가 사실상 secondary durability layer 역할을 한다.

---

## 7. Pinned Decisions (No User Re-prompt)

다음 7개 결정은 본 research 단계에서 context-derived로 확정한다. 이후 SPEC
주 본문(§1.1)에서 같은 번호로 재참조된다.

| ID | Decision | Recommendation | Alternatives Considered |
|----|----------|----------------|------------------------|
| **D1** | Event taxonomy | 20 event_types across 11 categories (§1.1) | open enum (cardinality risk), 단일 "event" type (semantic loss) |
| **D2** | cost_ledger 관계 | Option D — cross-reference only via payload.cost_ledger_id; cost_ledger schema unchanged | 흡수/이중쓰기/trigger-mirror (D5에서 trigger-mirror를 별도로 채택하여 자동화) |
| **D3** | Python-side LiteLLM cost 회수 | 5-min Asynq polling of `GET /spend/logs`; insert to audit_events only (NOT cost_ledger) | webhook (LiteLLM 미지원), PG direct read (vendor lock-in) |
| **D4** | Index event volume | All-in V1 + toggle `audit.events.index_write_enabled`; sampling은 M8 후속 | always-sample (audit 손실), always-skip (compliance 불충분) |
| **D5** | cost_ledger → audit_events mirror | Postgres AFTER INSERT trigger (§3.3) | App-side hook in DEEP-004 (코드 변경 발생) |
| **D6** | Hash chain | Optional, default OFF; daily verify job when enabled | Always-on (write throughput penalty), never (low tamper detection) |
| **D7** | S3 export format | JSONL.gz weekly | Parquet (운영 부담), CSV (스키마 손실), single-blob daily (대용량 객체) |

---

## 8. Observability Surface

### 8.1 Prometheus metrics

SPEC-OBS-001 NFR-OBS-002의 cardinality safety 규칙을 따른다. 모든 label 값은
bounded enumerable set이며 startup time에 pre-declare된다.

| Metric | Type | Labels | Cardinality |
|--------|------|--------|-------------|
| `usearch_audit_events_total` | CounterVec | `{event_type, decision, source}` | event_type 20 × decision 4 (`allow`/`deny`/`degrade`/`none`) × source 3 (`go`/`python`/`system`) = 240 |
| `usearch_audit_write_duration_seconds` | Histogram | (no labels) | buckets [0.001, 0.005, 0.01, 0.05, 0.1] |
| `usearch_audit_lag_seconds` | Gauge | `{source}` | source 3 |
| `usearch_audit_s3_export_duration_seconds` | Histogram | (no labels) | buckets [1, 10, 60, 300, 600] |
| `usearch_audit_s3_export_rows_total` | Counter | (no labels) | — |
| `usearch_audit_s3_export_bytes_total` | Counter | (no labels) | — |
| `usearch_audit_chain_violations_total` | Counter | (no labels) | — |
| `usearch_audit_reconcile_polls_total` | CounterVec | `{outcome}` | outcome ∈ {success, error, empty} = 3 |
| `usearch_audit_reconcile_lag_seconds` | Gauge | (no labels) | — |
| `usearch_audit_partition_drop_total` | Counter | (no labels) | — |
| `usearch_audit_replay_requests_total` | CounterVec | `{outcome}` | outcome ∈ {allowed, denied, rate_limited, error} = 4 |

**중요**: `event_type`, `decision`, `source`, `outcome` 모두 enum으로 bounded.
SPEC-OBS-001 `TestNoUnboundedLabels` 화이트리스트에 추가 필요.

### 8.2 OTel span attributes

`/admin/audit/replay` 핸들러의 OTel span(`audit.replay`)에 다음 속성 추가:

- `audit.replay.original_request_id` (string)
- `audit.replay.new_request_id` (string)
- `audit.replay.event_type` (string)
- `audit.replay.actor_user_id` (string)

`audit.export` 스케줄 job의 span(`audit.export`)에 다음 속성:

- `audit.export.partition` (string)
- `audit.export.row_count` (int)
- `audit.export.bytes_compressed` (int)
- `audit.export.s3_uri` (string)

NFR-OBS-002 적용 범위 밖(span attribute는 high-cardinality 허용).

### 8.3 slog records

각 audit_events INSERT 시점에 INFO 레벨 slog record 1개 emit:

```json
{
  "level": "INFO",
  "msg": "audit_event_recorded",
  "request_id": "req_abc123",
  "tenant_id": "default",
  "user_id": "...",
  "event_type": "cap.evaluation",
  "decision": "deny"
}
```

DEEP-004 decision event log(stderr JSON line)와 별개. log 수집 파이프라인이 둘 다
수집해도 dedup은 audit_events.id 기준으로 가능.

---

## 9. Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|-----------|
| **R1** Audit write가 hot path latency를 증가시킨다 | Medium | Medium | Async write via Asynq queue(cap-evaluation은 fire-and-forget으로 emit, audit handler가 enqueue). NFR-AUTH3-002로 50ms p95 budget 강제. |
| **R2** cost_ledger trigger 실패가 cost write를 망친다 | Low | High | trigger 내부에 try/catch + 운영자 toggle `audit.cost_mirror_strict: false`로 fail-open 가능 |
| **R3** Audit volume이 Postgres 디스크 폭증 | Medium | Medium | Monthly partition + 90d retention + S3 archive. `index.write` 토글로 70% 즉시 절감 가능 |
| **R4** Hash chain race under concurrent INSERT | Medium (when enabled) | High (chain breaks) | App-side serialization via advisory lock per-tenant. 또는 일괄 검증 job만 hash 책임지고 INSERT는 prev_hash NULL → backfill |
| **R5** Python-side reconciliation drift(중복/누락) | Medium | Medium | dedup by `litellm_request_id` UNIQUE. 5-min poll window overlap 1분(idempotent UPSERT) |
| **R6** S3 outage causes retention cleanup deadlock | Low | Medium | `require_s3_archive: false` toggle (운영자가 S3 없이도 운영 가능) |
| **R7** Replay endpoint leaks PII via re-execution | Low | High | replay actor must have `audit.replay` permission (AUTH-002). 원본 user identity는 새 request에 propagate되지 않으며, replay actor identity로 재실행 |
| **R8** event_type cardinality explosion | Low | Medium | startup time enum lock. 새 event_type 추가는 코드 변경(SPEC amendment) 필요 |
| **R9** Postgres trigger overhead under high-throughput ingest | Medium | Medium | `audit.events.index_write_enabled: false` toggle. trigger는 per-row이므로 batch INSERT 시 N배. M8에서 partition-aware async COPY로 전환 검토 |
| **R10** LiteLLM `/spend/logs` schema 변경 | Medium | Low | 단일 fetcher module isolated. schema 검증 + parse 실패 시 `usearch_audit_reconcile_polls_total{outcome="error"}` 기록 + 운영자 알람 |
| **R11** Audit-as-PII risk (audit log itself contains personal data) | Low | Medium | retention + PII masking + access control. AUTH-002 RBAC `audit.read` permission 필수 |
| **R12** Trigger를 우회한 app-side raw SQL이 audit를 회피 | Low | High | DB role `audit_writer`는 SELECT/INSERT만, `audit_admin`만 partition drop 권한. code review + lint(`grep -r "DELETE FROM audit_events"`) |

---

## 10. References

### 10.1 Internal SPEC documents
- `.moai/specs/SPEC-DEEP-004/spec.md` — decision event log JSON line schema (REQ-DEEP4-010),
  cost_ledger schema (REQ-DEEP4-006), forward-compat commitment (§6.3)
- `.moai/specs/SPEC-DEEP-004/research.md` §1.4 — Python-side cost 누락 risk
- `.moai/specs/SPEC-DEEP-004/research.md` §8.3 — audit log JSON 필드 사전 채택
- `.moai/specs/SPEC-DEEP-004/research.md` R5 — Python reconciliation 위임
- `.moai/specs/SPEC-AUTH-001/spec.md` (M6 draft) — OIDC SSO, JWT middleware의
  user_id propagation pattern
- `.moai/specs/SPEC-AUTH-002/spec.md` (M6 draft) — Casbin RBAC, `audit.replay` /
  `audit.read` permission 추가 필요
- `.moai/specs/SPEC-OBS-001/spec.md` — slog handler 추가 패턴, Prometheus naming
  convention(`usearch_<domain>_*`), NFR-OBS-002 cardinality safety
- `.moai/specs/SPEC-IDX-001/spec.md` — `internal/index.Upsert`의 audit hook 부착점

### 10.2 Implementation references (reuse map)

| New file | Closest analog | Reference |
|----------|---------------|-----------|
| `internal/audit/store.go` | `internal/deepagent/costguard/ledger.go` | pgx-based row insert pattern |
| `internal/audit/handler.go` (slog handler) | `internal/obs/log/handler.go` | structured log handler chain |
| `internal/audit/decision_log_handler.go` | `internal/deepagent/costguard/middleware.go::emitDecisionEvent` | mirror DEEP-004 stderr line → audit_events |
| `internal/audit/litellm_reconcile.go` | `internal/deepagent/costguard/reconcile_job.go` | 5-min scheduled Asynq pattern |
| `internal/audit/export.go` | (new) | streaming SELECT → S3 PutObject via aws-sdk-go-v2 |
| `internal/audit/replay.go` | `cmd/usearch-api/handlers/synthesis.go` | chi handler with admin RBAC |
| `internal/audit/chain.go` | (new) | SHA256 hash-chain primitives |
| `deploy/postgres/migrations/0003_audit_events.sql` | `deploy/postgres/migrations/0002_cost_ledger.sql` | migration numbering + idempotent CREATE |
| `cmd/usearch-api/handlers/admin_audit.go` | `cmd/usearch-api/handlers/synthesis.go` | chi router pattern |

### 10.3 External references

- LiteLLM Spend Logs API: https://docs.litellm.ai/docs/proxy/cost_tracking
  (verified 2026-05-22: `GET /spend/logs?summarize=false&start_date=&end_date=`)
- LiteLLM Global Spend Report: https://docs.litellm.ai/docs/proxy/cost_tracking
  (`GET /global/spend/report?group_by=customer&api_key=...`)
- AWS S3 SDK Go v2: https://aws.github.io/aws-sdk-go-v2/docs/sdk-utilities/s3/
- MinIO S3 compatibility: https://min.io/docs/minio/linux/developers/go/API.html
- Postgres partition pruning: https://www.postgresql.org/docs/16/ddl-partitioning.html
- Postgres trigger documentation: https://www.postgresql.org/docs/16/sql-createtrigger.html
- Asynq scheduled tasks: https://github.com/hibiken/asynq/wiki/Periodic-Tasks

### 10.4 File-line internal citations

- `.moai/project/roadmap.md:85` — M6 row "SPEC-AUTH-003 | Audit log | Postgres
  audit table, query replay, optional S3 export"
- `.moai/project/tech.md:70-74` — Team plane stack (Auth/RBAC/Audit/Rate-limit/Secrets)
- `.moai/project/product.md` — "audit log complete" 지표
- `deploy/postgres/migrations/0001_create_docs.sql` — migration numbering precedent
- `deploy/postgres/migrations/0002_cost_ledger.sql` — cost_ledger schema (target of trigger)
- `internal/deepagent/costguard/middleware.go::emitDecisionEvent` — decision event log emit point
- `internal/deepagent/costguard/ledger.go::WriteLedgerEntry` — cost_ledger writer to be triggered
- `internal/index/pg/client.go:90` — EnsureSchema pattern for migration application

---

**End of Research Document**
