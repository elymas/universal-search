# SPEC-AUTH-002 Acceptance Scenarios

Generated: 2026-05-22
Format: Given / When / Then (Korean prose, English identifiers)

본 문서는 SPEC-AUTH-002의 acceptance scenarios 9건 + boundary edge case
2건을 정의한다. 각 시나리오는 spec.md §5에서 참조되며 plan-auditor가 SPEC
PASS 평가의 근거로 사용한다.

---

## §5.1 enforcer init + valid policy → Enforce(member, team, query:basic, read) returns true

**Coverage**: REQ-AUTH2-001, REQ-AUTH2-002

### Given

- `auth.yaml`:
  ```yaml
  auth:
    rbac:
      enabled: true
      default_team_id: "default"
      pg_dsn: "postgres://test:test@localhost:5432/casbin_test?sslmode=disable"
      audit_to_stderr: true
  ```
- PostgreSQL `casbin_rule` table empty (fresh database).
- `policy_default.csv` 가 `//go:embed` 로 binary 에 포함되어 있고 다음
  핵심 row 보유:
  - `g, alice, role_member, engineering` (alice는 engineering team의
    member)
  - `p, role_member, *, query:basic, read, allow`
  - `p, *, *, *, *, deny` (catch-all)
- 서비스 startup 시 enforcer init → `LoadPolicy()` → row count == 0 →
  policy_default.csv 일괄 import → `SavePolicy()`.

### When

코드에서 다음 호출:
```go
decision, err := enforcer.Enforce("alice", "engineering", "query:basic", "read")
```

### Then

- `err == nil`, `decision == true`.
- enforcer singleton (`rbac.GlobalEnforcer()`) 두 번 호출 시 동일 포인터.
- `model.conf` 가 embed 된 그대로 enforcer 에 load됨 (`TestModelConfMatches
  Embed` 검증).
- PostgreSQL `casbin_rule` 테이블에 import 된 정책 row 들이 persist됨
  (다음 서비스 재시작 시 LoadPolicy 만으로 동일 상태 복원).
- Prometheus 카운터 `usearch_rbac_decisions_total{result="allow",reason_
  class="policy_matched"}` 1 증가.
- Histogram `usearch_rbac_eval_duration_seconds` 에 < 1ms 관측치 추가
  (NFR-AUTH2-001).

---

## §5.2 AUTH-001 JWT context 우선; header fallback도 동작

**Coverage**: REQ-AUTH2-003

### Given (case A — AUTH-001 컨텍스트 존재)

- AUTH-001 JWT 미들웨어가 valid JWT 검증 후 다음 컨텍스트 set:
  - `costguard.UserIDKey = "alice"` (AUTH-001 JWT sub claim)
  - `auth.TeamIDKey = "engineering"` (AUTH-001 JWT team_id claim)
  - `auth.RolesKey = []string{"member"}` (AUTH-001 JWT roles claim)
- 호출자가 동시에 `X-User-Id: malicious`, `X-Team-Id: marketing`,
  `X-Roles: admin` 헤더 부착 (악의적 spoofing 시도).

### When (case A)

호출자가 다음 요청 전송:
```
POST /query
Authorization: Bearer <valid_alice_jwt>
X-User-Id: malicious
X-Team-Id: marketing
X-Roles: admin
{"query": "test"}
```

### Then (case A)

- `TeamScopeMiddleware` 가 AUTH-001 컨텍스트를 우선 사용.
- `r.Context()` 에서:
  - `rbac.UserIDFromContext(ctx) == "alice"`
  - `rbac.TeamIDFromContext(ctx) == "engineering"`
  - `rbac.RolesFromContext(ctx) == []string{"member"}`
- 헤더의 `malicious` / `marketing` / `admin` 값은 **무시**됨.
- 후속 `EnforceMiddleware("query:basic","read")` 가 `Enforce("alice",
  "engineering", "query:basic", "read")` 호출 → true → HTTP 200.

### Given (case B — AUTH-001 컨텍스트 없음, 헤더 fallback)

- AUTH-001 비활성 환경 (e.g., `auth-001-ga=false` 또는 AUTH-001 미들웨어
  미설치 dev 환경).
- 호출자가 `X-User-Id: bob`, `X-Team-Id: research`, `X-Roles: member`
  헤더 부착.

### When (case B)

```
POST /query
X-User-Id: bob
X-Team-Id: research
X-Roles: member
{"query": "test"}
```

### Then (case B)

- AUTH-001 컨텍스트 없음 → 헤더 fallback 활성화.
- `r.Context()` 에서:
  - `rbac.UserIDFromContext(ctx) == "bob"`
  - `rbac.TeamIDFromContext(ctx) == "research"`
  - `rbac.RolesFromContext(ctx) == []string{"member"}`
- enforce 통과 시 HTTP 200.

### Given (case C — 모두 부재)

- 컨텍스트도 헤더도 없음.

### When (case C)

```
POST /query
{"query": "test"}
```

### Then (case C)

- `rbac.UserIDFromContext(ctx) == "anonymous"`, roles == `[]`.
- team_id 는 empty → REQ-AUTH2-004 의 default_team_id fallback 적용
  (§5.3 참조).
- AUTH-001 의 기존 테스트 (e.g., DEEP-004 `TestIdentityMiddlewareReads
  XUserId`) 는 본 SPEC ship 후에도 unchanged 로 PASS (NFR-AUTH2-005).

---

## §5.3 empty team_id + default fallback → "default" scope; default blank → 400

**Coverage**: REQ-AUTH2-004

### Given (case A — default_team_id 기본값 사용)

- `auth.yaml` `auth.rbac.default_team_id: "default"` (V1 기본값).
- 호출자가 JWT 도 `X-Team-Id` 헤더도 없는 상태.
- 단, `X-User-Id: charlie` 와 `X-Roles: member` 는 있음.

### When (case A)

```
POST /query
X-User-Id: charlie
X-Roles: member
{"query": "test"}
```

### Then (case A)

- `TeamScopeMiddleware` 가 empty team_id 감지 → fallback `"default"`
  적용.
- `r.Context()` 에서 `rbac.TeamIDFromContext(ctx) == "default"`.
- Prometheus 카운터 `usearch_rbac_decisions_total{result="allow",reason_
  class="empty_team"}` 1 증가 (operator 가 fallback 사용량 모니터링
  가능).
- 후속 enforce 가 `Enforce("charlie", "default", "query:basic", "read")`
  호출. `policy_default.csv` 의 `p, role_member, *, query:basic, read,
  allow` 가 domain wildcard `*` 로 default team 도 매치 → true → HTTP 200.

### Given (case B — default_team_id empty, multi-team enforce 모드)

- `auth.yaml` `auth.rbac.default_team_id: ""` (operator 가 명시적으로
  enforce 모드 전환).
- 호출자가 동일 요청 (team_id 누락).

### When (case B)

```
POST /query
X-User-Id: charlie
X-Roles: member
{"query": "test"}
```

### Then (case B)

- HTTP 응답 status **400 Bad Request**.
- 응답 본문:
  ```json
  {"error": "team_id_required"}
  ```
- enforce 호출되지 않음 (middleware 단에서 단락).
- Prometheus 카운터: empty_team fallback counter 증가 없음 (다른 deny
  counter 도 증가 없음 — 400 은 enforce 이전 단계).

---

## §5.4 observer가 team_index write 시도 → 403 (deny-by-default + role 계층)

**Coverage**: REQ-AUTH2-002, REQ-AUTH2-005

### Given

- `policy_default.csv` 에 다음 명시적 정책:
  - `g, david, role_observer, engineering`
  - `p, role_member, *, team_index, write, allow`
  - `p, role_admin, *, team_index, write, allow`
  - (observer 에게는 team_index write 정책이 SHALL NOT 존재 — role 계층
    표현 D4 의 정책 row 복제 패턴에 따라 admin 이 member 권한도 명시
    보유하지만 observer 는 read-only)
- david 는 observer role 로 engineering team 소속.

### When

david 가 다음 요청 전송 (가상의 indexed 문서 직접 write 시도):
```
POST /admin/index/team_documents
Authorization: Bearer <david_jwt_observer>
{"document": "..."}
```

### Then

- AUTH-001 미들웨어 통과 → context 에 david/observer/engineering 주입.
- `TeamScopeMiddleware` 통과 → identity 추출.
- `EnforceMiddleware("team_index","write")` 가 `Enforce("david",
  "engineering", "team_index", "write")` 호출.
- 평가 결과:
  - matched policy: 없음 (observer 에게 team_index write 정책 부재).
  - catch-all `p, *, *, *, *, deny` 매치 또는 policy_effect 의
    `some(allow)` false → DENY.
- HTTP 응답 status **403 Forbidden**.
- 응답 본문:
  ```json
  {"error": "forbidden", "resource": "team_index", "action": "write"}
  ```
- Prometheus 카운터 `usearch_rbac_decisions_total{result="deny",reason_
  class="no_policy_matched"}` 1 증가.
- 동일 david 가 `POST /query` (resource=`query:basic`, action=`read`)
  를 호출하면 정책 `p, role_observer, *, query:basic, read, allow` 매치
  → HTTP 200 (read-only 권한은 유지).

---

## §5.5 EnforceMiddleware: `/admin/audit` 호출 시 observer 403, admin 200

**Coverage**: REQ-AUTH2-005

### Given

- 두 사용자:
  - eve: admin role (`g, eve, role_admin, engineering`)
  - david: observer role (`g, david, role_observer, engineering`)
- 정책 `p, role_admin, *, audit_log, read, allow` 존재.
- observer 에게 audit_log read 정책 부재.

### When (case A — observer 호출)

```
GET /admin/audit?limit=100
Authorization: Bearer <david_jwt_observer>
```

### Then (case A)

- `EnforceMiddleware("audit_log","read")` 가 `Enforce("david","engineering",
  "audit_log","read")` 호출.
- 정책 매치 없음 → DENY → HTTP 403 + body `{"error":"forbidden",
  "resource":"audit_log","action":"read"}`.

### When (case B — admin 호출)

```
GET /admin/audit?limit=100
Authorization: Bearer <eve_jwt_admin>
```

### Then (case B)

- `Enforce("eve","engineering","audit_log","read")` → `p, role_admin,*,
  audit_log,read,allow` 매치 → ALLOW → next.ServeHTTP 호출 → handler
  실행 → HTTP 200.

---

## §5.6 query handler가 IndexQuery.TeamID 강제 → 3-store filter 모두 활성화

**Coverage**: REQ-AUTH2-006

### Given

- alice (member of engineering team) 가 valid JWT 로 인증된 상태.
- `r.Context()` 에 `rbac.TeamIDFromContext(ctx) == "engineering"`.
- `internal/index/dispatch.go:217-245` 의 IndexQuery.TeamID 필드는 이미
  SPEC-IDX-001 v0.1 에서 reserve 되어 있고, Qdrant/Meili/PG 3 store 의
  filter 진입점은 universally NULL 상태 (v0.1 모든 INSERT 가 NULL).

### When

alice 가 다음 요청 전송:
```
POST /query
Authorization: Bearer <alice_jwt>
{"query": "casbin RBAC"}
```

### Then

- query handler (`cmd/usearch-api/handlers/synthesis.go`) 가 IndexQuery
  구성 시 다음 한 줄 추가:
  ```go
  q.TeamID = rbac.TeamIDFromContext(r.Context())
  ```
- 결과 IndexQuery `{TeamID:"engineering", ...}` 가 `dispatch.go` 의
  3-store 분기에 전달:
  - Qdrant: `client.go:242` 의 payload filter `Key:"team_id"` 가
    "engineering" 으로 set → search 결과 team_id="engineering" payload
    문서만 반환.
  - Meilisearch (`korean_shard.go:27`, `index.go:108` 의 filterable
    attribute): filter string `team_id = "engineering"` 적용 → 같은
    조건 매칭만 반환.
  - PostgreSQL (`dispatch.go:245` 의 `pg.Filters.TeamID`): WHERE 절에
    `team_id = 'engineering'` 추가 → 동일 매칭.
- v0.1 로 indexed 된 기존 row (team_id NULL) 는 어느 store 에서도 매치
  되지 않음 → empty result. (이는 SPEC-IDX-004 의 NOT NULL flip +
  backfill 까지 의도된 상태; 본 SPEC ship 직후 IDX-004 가 즉시 진행.)
- 다른 team 의 문서 (e.g., team_id="marketing") 는 search 결과에
  포함되지 SHALL NOT — cross-team data leak 차단.

### Why this matters

이는 본 SPEC 이 IDX-001 reservation 을 처음으로 실제 활성화하는 시점
이다. 단일 라인 변경 (per handler) 으로 3-store team scoping 이 모두
강제되는 이유는 SPEC-IDX-001 이 ship 시점에 이미 모든 filter 진입점
을 reserve 해 두었기 때문이다. 본 SPEC ship 직후 SPEC-IDX-004 가
컬럼 NOT NULL flip + RLS 추가로 DB-level 강제까지 완성한다.

---

## §5.7 reddit (team_shared) member 접근; gmail (personal V1.1) deny

**Coverage**: REQ-AUTH2-007, REQ-AUTH2-008

### Given (case A — team_shared adapter)

- 12 V1 adapter 중 reddit 이 `internal/adapters/registry.go` 에
  `Visibility: VisibilityTeamShared` 로 등록됨.
- 정책 `p, role_member, *, adapter:reddit, read, allow` 존재.
- alice (member of engineering) 가 query 발행.

### When (case A)

```
POST /query
Authorization: Bearer <alice_jwt>
{"query": "rust async"}
```

query handler 의 fanout 단계에서 reddit adapter 사용 직전 본 SPEC 이
add 한 per-adapter enforce 호출:
```go
if !enforcer.Enforce(uid, tid, "adapter:reddit", "read") {
    // skip reddit; continue with other adapters
    return
}
```

### Then (case A)

- `Enforce("alice","engineering","adapter:reddit","read")` → true.
- reddit adapter 정상 호출, search 결과가 다른 adapter (hn, arxiv 등)
  결과와 함께 합성.
- HTTP 응답 status **200 OK**, body 에 reddit 결과 포함.
- Prometheus 카운터 `usearch_rbac_decisions_total{result="allow",reason_
  class="policy_matched"}` 1 증가 (per-adapter check 마다).

### Given (case B — personal adapter V1.1, V1 에서는 deny)

- 가상 미래의 gmail adapter 가 `VisibilityPersonal` 로 등록됨 (V1.1
  ship 가정). V1 에서는 등록조차 되지 않으나 본 시나리오는 enum/정책
  shape 검증용.
- V1 ship 시점에는 personal 정책 row 가 SHALL NOT emit (research §3.4).

### When (case B)

(가상의 V1.1 시나리오) alice 가 gmail 데이터 query 시도:
```
POST /query
Authorization: Bearer <alice_jwt>
{"query": "today's gmail", "adapters": ["gmail"]}
```

### Then (case B in V1)

- V1 ship 에서는 gmail adapter 자체가 registry 에 없음 → 요청은 단순히
  "unknown adapter" 로 무시 또는 400 반환 (별도 SPEC 처리).
- V1 의 enforcer 평가 시점에 `adapter:gmail:alice` 같은 객체에 대한
  allow 정책 부재 → deny-by-default.
- `AdapterVisibility.Personal` enum 은 정의되어 있어 V1.1 SPEC-AUTH-005
  가 활성화 시 즉시 사용 가능.
- helper `MakePersonalPolicyRow("alice","engineering","gmail")` 호출 시
  반환되는 정책 row: `p, alice, engineering, adapter:gmail:alice, read,
  allow` shape 검증 (REQ-AUTH2-008).

---

## §5.8 admin reload + member add/remove/list endpoints

**Coverage**: REQ-AUTH2-009, REQ-AUTH2-010

### Given

- eve: admin role of engineering.
- enforcer 가 startup 시점에 100 개 정책 row 보유.

### When (case A — reload)

eve 가 PG `casbin_rule` 테이블에 직접 SQL 로 5 row add 후:
```
POST /admin/rbac/reload
Authorization: Bearer <eve_jwt_admin>
```

### Then (case A)

- handler 가 `enforcer.LoadPolicy()` 호출.
- HTTP 응답 status **200 OK**.
- 응답 본문:
  ```json
  {"status": "reloaded", "policy_count": 105}
  ```
- 직후 새 정책 row 의 권한이 즉시 enforce 에 반영.
- Prometheus 카운터 `usearch_rbac_policy_reload_total{outcome="success"}`
  1 증가.

### Given (case A boundary — observer 호출)

david (observer) 가 동일 호출:
```
POST /admin/rbac/reload
Authorization: Bearer <david_jwt_observer>
```

### Then (case A boundary)

- `EnforceMiddleware("rbac_policy","*")` 가 deny → HTTP 403.

### When (case B — add member)

eve 가 호출:
```
POST /admin/members
Authorization: Bearer <eve_jwt_admin>
{"user_id":"frank","team_id":"engineering","role":"member"}
```

### Then (case B)

- handler 가 `enforcer.AddRoleForUserInDomain("frank","role_member",
  "engineering")` + `SavePolicy()` 호출.
- HTTP 응답 status **201 Created**.
- PG `casbin_rule` 에 row `g | frank | role_member | engineering` 추가됨.
- 직후 frank 가 같은 권한으로 enforce 평가됨 (별도 reload 불필요 —
  AddRoleForUserInDomain 자체가 in-memory + persist).

### When (case C — list members)

```
GET /admin/members?team_id=engineering
Authorization: Bearer <eve_jwt_admin>
```

### Then (case C)

- 응답 본문 예시:
  ```json
  {
    "team_id": "engineering",
    "members": [
      {"user_id":"alice","role":"member"},
      {"user_id":"david","role":"observer"},
      {"user_id":"eve","role":"admin"},
      {"user_id":"frank","role":"member"}
    ]
  }
  ```

### When (case D — remove member)

```
DELETE /admin/members?user_id=frank&team_id=engineering&role=member
Authorization: Bearer <eve_jwt_admin>
```

### Then (case D)

- handler 가 `enforcer.DeleteRoleForUserInDomain("frank","role_member",
  "engineering")` + `SavePolicy()` 호출.
- HTTP 응답 status **204 No Content**.

### When (case E — invalid role)

```
POST /admin/members
Authorization: Bearer <eve_jwt_admin>
{"user_id":"grace","team_id":"engineering","role":"superuser"}
```

### Then (case E)

- handler 가 input validation 단계에서 거부 (observer/member/admin 외
  값).
- HTTP 응답 status **400 Bad Request** + body `{"error":"invalid_role"}`.

---

## §5.9 모든 decision이 Prometheus + OTel span + stderr JSON line 동시 emit

**Coverage**: REQ-AUTH2-011

### Given

- 임의의 RBAC 평가 호출 (e.g., alice/engineering/query:basic/read).
- OTel exporter 가 testcollector 로 set, stderr 캡처 active.

### When

`enforcer.Enforce("alice","engineering","query:basic","read")` 호출.

### Then

세 surface 모두 동시 emit:

1. **Prometheus 카운터** `usearch_rbac_decisions_total{result="allow",
   reason_class="policy_matched"}` 1 증가.

2. **OTel span** `rbac.evaluate` 생성, 다음 attribute 부착:
   - `rbac.user_id = "alice"` (string, high-cardinality OK)
   - `rbac.team_id = "engineering"`
   - `rbac.resource = "query:basic"`
   - `rbac.action = "read"`
   - `rbac.decision = "allow"`
   - `rbac.eval_duration_ms = <float, < 1.0>`

3. **stderr JSON line audit event**:
   ```json
   {
     "timestamp": "2026-05-22T10:00:00.123Z",
     "event_type": "rbac.decision",
     "request_id": "req_abc123",
     "tenant_id": "default",
     "team_id": "engineering",
     "user_id": "alice",
     "decision": "allow",
     "resource": "query:basic",
     "action": "read",
     "reason": "matched policy p:role_member,*,query:basic,read,allow"
   }
   ```

검증:
- 세 surface 의 timestamp 가 같은 request 의 동일 평가에 대해 일관됨
  (~ms 단위 격차 허용).
- stderr JSON schema 가 SPEC-AUTH-003 의 audit log forward-compat 호환
  (필수 필드 superset 검증) — AUTH-003 가 ship 되면 같은 schema 가
  PostgreSQL `audit_log` 테이블에 persist 됨 (additive, 본 SPEC v1
  의 stderr 출력은 유지).
- slog 의 stdout 출력과 충돌 없음 (NFR-AUTH2-008 — stderr/stdout 분리).
- deny 결과의 경우 `decision: "deny"`, `reason` 에 매칭 deny 정책 또는
  `"no allow policy matched (default deny)"` 가 들어감.

---

## Edge Case 1 — Casbin enforcer concurrency: 동시 1000 request 평가 race 없음, NFR latency 충족

**Coverage**: REQ-AUTH2-002, NFR-AUTH2-001

### Given

- 100 정책 row + 10 role 의 enforcer (V1 가정 시나리오).
- Go test `-race` 활성.

### When

- 1000 goroutine 동시 호출:
  ```go
  for i := 0; i < 1000; i++ {
      go func(i int) {
          enforcer.Enforce(fmt.Sprintf("user%d", i%50), "team-a",
              "query:basic", "read")
      }(i)
  }
  ```

### Then

- race detector 가 race 감지 SHALL NOT (casbin v2 내부 RWMutex 로 thread-
  safe 보장).
- 모든 goroutine 정상 return.
- `usearch_rbac_eval_duration_seconds` Histogram 의 p95 < 1ms (NFR-
  AUTH2-001).
- 동시 호출 중 `enforcer.LoadPolicy()` 가 별도 goroutine 에서 호출되어도
  enforce 결과는 atomic (LoadPolicy 직전 / 직후 상태만 보임).

---

## Edge Case 2 — LoadPolicy 실패 시 기존 메모리 enforcer 보존 (atomic replace)

**Coverage**: REQ-AUTH2-009

### Given

- enforcer 가 정상 100 정책 row 로 동작 중.
- eve (admin) 가 `POST /admin/rbac/reload` 호출 직전, 별도 작업자가
  PG connection 을 차단 (or `pg_dsn` 일시 변경).

### When

```
POST /admin/rbac/reload
Authorization: Bearer <eve_jwt_admin>
```

핸들러가 `enforcer.LoadPolicy()` 호출 → casbin-pg-adapter 가 PG SELECT
실패 → error return.

### Then

- HTTP 응답 status **500 Internal Server Error**.
- 응답 본문:
  ```json
  {"error": "reload_failed", "reason": "connection refused"}
  ```
- Prometheus 카운터 `usearch_rbac_policy_reload_total{outcome="failure"}`
  1 증가.
- **기존 메모리 enforcer 는 unchanged** — 직후 같은 enforcer 로 enforce
  호출 시 정상 100 정책 평가 가능 (atomic replace; casbin v2 의
  LoadPolicy 가 실패 시 in-memory state 를 손상시키지 SHALL NOT).
- alice 의 `POST /query` 같은 normal request 는 영향 없이 계속 200
  응답 (downtime 없음).

### Why this matters

policy reload 가 production outage 의 trigger 가 되지 않도록 atomic
replace 가 enforced 되어야 한다. casbin v2 가 내부적으로 보장하지만
본 SPEC 의 wrapper 코드 (`enforcer.go` 의 LoadPolicy 호출 시 추가 처리
가 있는 경우) 가 실수로 partial state 를 노출하지 않도록 별도 테스트로
가드한다.

---

## Acceptance Coverage Matrix

| Scenario | REQ-001 | REQ-002 | REQ-003 | REQ-004 | REQ-005 | REQ-006 | REQ-007 | REQ-008 | REQ-009 | REQ-010 | REQ-011 |
|----------|---------|---------|---------|---------|---------|---------|---------|---------|---------|---------|---------|
| §5.1 | ✓ | ✓ |   |   |   |   |   |   |   |   |   |
| §5.2 |   |   | ✓ |   |   |   |   |   |   |   |   |
| §5.3 |   |   |   | ✓ |   |   |   |   |   |   |   |
| §5.4 |   | ✓ |   |   | ✓ |   |   |   |   |   |   |
| §5.5 |   |   |   |   | ✓ |   |   |   |   |   |   |
| §5.6 |   |   |   |   |   | ✓ |   |   |   |   |   |
| §5.7 |   |   |   |   |   |   | ✓ | ✓ |   |   |   |
| §5.8 |   |   |   |   |   |   |   |   | ✓ | ✓ |   |
| §5.9 |   |   |   |   |   |   |   |   |   |   | ✓ |
| Edge1 |   | ✓ |   |   |   |   |   |   |   |   |   |
| Edge2 |   |   |   |   |   |   |   |   | ✓ |   |   |

NFR coverage:
- NFR-AUTH2-001 (enforce p95 ≤ 1ms): §5.1 측정 + Edge1.
- NFR-AUTH2-002 (LoadPolicy p99 ≤ 200ms): §5.8 case A 측정.
- NFR-AUTH2-003 (cardinality safety): metrics_test.go::TestNoUnboundedLabels 확장.
- NFR-AUTH2-004 (PG connection isolation): TestPGAdapterIsolatedConnection (Phase B).
- NFR-AUTH2-005 (AUTH-001 forward-compat): §5.2 case A + TestForwardCompatWithAUTH001Keys.
- NFR-AUTH2-006 (hot-reload): Phase F unit test.
- NFR-AUTH2-007 (metric naming): metrics_test.go 확장.
- NFR-AUTH2-008 (stderr/stdout 분리): §5.9 + TestDecisionEmitsStderrJSONLine.

---

*End of SPEC-AUTH-002 acceptance.md.*
