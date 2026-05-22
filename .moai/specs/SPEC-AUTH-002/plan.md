# SPEC-AUTH-002 Implementation Plan

Generated: 2026-05-22
Methodology: TDD (RED-GREEN-REFACTOR)
Coverage target: 85%
Harness: standard

---

## 1. Overview

본 plan.md는 SPEC-AUTH-002의 구현 단계별 task sequence를 정의한다. 11
EARS REQs + 8 NFRs를 6개 phase로 분해하며, 각 phase는 RED → GREEN →
REFACTOR 사이클을 따른다. plan-auditor 통과 + annotation cycle 완료 후
본 plan은 manager-tdd 에이전트에게 전달되어 phase-별로 진행한다.

본 SPEC의 sequencing principle: **(1) deny-by-default를 enforcer
초기화와 동시에 RED 테스트로 고정** (정책 누락 시 fail-closed가 핵심
security invariant), **(2) AUTH-001 forward-compat 컨텍스트 키
contract를 enforcer-level 통합 전에 wire** (downstream IDX-004/005 의존
성 가시화), **(3) IndexQuery team_id 강제는 모든 query handler 변경의
final wave** (단일 라인 변경이지만 IDX-001 reservation을 처음 활성화하는
의미).

---

## 2. Phase Breakdown

### Phase A — Casbin Library Pin + Model/Policy Embed

목표: Casbin v2 / casbin-pg-adapter direct require 추가 + RBAC-with-
domains model.conf 작성 + V1 default policy CSV embed.

**RED tests** (4):

1. `TestModelConfMatchesEmbed` — `//go:embed model.conf` 가 RBAC-with-
   domains 4-tuple 모델 (request/policy/role/effect/matchers 5 sections)
   을 정확히 담고 있음 (REQ-AUTH2-001).
2. `TestPolicyDefaultCSVMatchesEmbed` — `//go:embed policy_default.csv`
   가 V1 12 adapter + role 정책 rows (research §2.4 schema) 정확히
   담고 있음 (REQ-AUTH2-001, 007).
3. `TestCatchAllDenyRowPresent` — policy_default.csv의 마지막 row가
   `p, *, *, *, *, deny` 임 (REQ-AUTH2-002 deny-by-default 안전망).
4. `TestCasbinLibraryImportable` — `github.com/casbin/casbin/v2` 와
   `github.com/casbin/casbin-pg-adapter` import 가능 (compile gate).

**GREEN tasks**:

- `go.mod` 에 `github.com/casbin/casbin/v2 v2.103.x` 와 `github.com/
  casbin/casbin-pg-adapter v1.5.0` direct require add (정확한 minor
  버전은 `go get` 시점에 latest stable 확정).
- `internal/auth/rbac/model.conf` 작성 (research §2.3 verbatim).
- `internal/auth/rbac/policy_default.csv` 작성 (research §2.4 schema).
- `internal/auth/rbac/types.go` 작성: Role enum
  (`RoleObserver`, `RoleMember`, `RoleAdmin`), `Decision` struct,
  `AdapterVisibility` enum.

**REFACTOR**:

- model.conf 와 policy_default.csv 의 line-by-line 주석 추가 (운영자
  debugging 용이성).

---

### Phase B — Enforcer Core (init, LoadPolicy, Enforce, thread-safety)

목표: Casbin enforcer singleton + PG adapter 초기화 + deny-by-default
+ thread-safety 검증.

**RED tests** (7):

5. `TestEnforcerInitFromEmbeddedModel` — `rbac.NewEnforcer(adapter)` 호출
   시 embed된 model.conf로 enforcer 인스턴스 생성, error nil (REQ-AUTH2-001).
6. `TestEnforcerInitFatalExitOnFailure` — adapter init 실패 시
   `rbac.MustInit()`이 log.Fatalf 호출 (`auth.rbac.enabled: true` 일
   때만; `false`면 nil enforcer로 return) (REQ-AUTH2-001).
7. `TestEnforcerSingletonReused` — `rbac.GlobalEnforcer()` 두 번 호출 시
   동일 *casbin.Enforcer 포인터 반환 (REQ-AUTH2-001).
8. `TestDenyByDefaultWhenNoAllowMatch` — empty policy set + Enforce
   호출 → false (REQ-AUTH2-002).
9. `TestExplicitDenyOverridesAllow` — `p, alice, t1, r1, read, allow` +
   `p, alice, t1, r1, read, deny` 둘 다 존재 → Enforce false (deny
   wins) (REQ-AUTH2-002).
10. `TestEnforcerThreadSafeUnderConcurrency` — 1000 goroutine 동시
    Enforce 호출, race detector 통과 (REQ-AUTH2-002, NFR-AUTH2-001).
11. `TestPGAdapterIsolatedConnection` — adapter init 시 `*pg.DB` 별도
    생성, 우리 hot-path pgxpool과 동일 instance 아님 (NFR-AUTH2-004).

**GREEN tasks**:

- `internal/auth/rbac/adapter.go` 작성 — `NewPGAdapter(dsn)` →
  `casbin-pg-adapter`의 `pgadapter.NewAdapter(dsn)` 호출, 별도
  `*pg.DB` 보유.
- `internal/auth/rbac/enforcer.go` 작성 — `NewEnforcer(adapter)`,
  `MustInit(cfg)`, `GlobalEnforcer()`, `Enforce(sub, dom, obj, act)`.
  내부적으로 `casbin.NewEnforcer(model.conf, adapter)` 호출 후
  `enforcer.EnableAutoSave(false)` (batch 작업 안전성).
- `deploy/postgres/migrations/0003_casbin_rules.sql` 작성 — research
  §4.1 schema (idempotent).
- bootstrap logic: `LoadPolicy()` 후 row count == 0 이면
  policy_default.csv를 `AddPolicies(...)` + `SavePolicy()` 일괄 load.

**REFACTOR**:

- enforcer init 의 error wrapping 을 fmt.Errorf "%w" 패턴으로 통일.
- adapter 의 *pg.DB lifecycle (shutdown 시 Close) 정리.

---

### Phase C — Identity Extraction + AUTH-001 Forward-Compat

목표: 본 SPEC의 두 컨텍스트 키 (`auth.TeamIDKey`, `auth.RolesKey`)
신설 + AUTH-001 컨텍스트 우선 + 헤더 fallback + empty team_id 처리.

**RED tests** (10):

12. `TestContextKeysExported` — `auth.TeamIDKey`, `auth.RolesKey` 상수
    가 `internal/auth/rbac/context.go`에 export됨, type 일치 (REQ-AUTH2-003).
13. `TestExtractsFromAUTH001JWTContext` — request context에
    `costguard.UserIDKey="alice"`, `auth.TeamIDKey="team-a"`,
    `auth.RolesKey=["member"]` set된 상태에서 TeamScopeMiddleware
    실행 → handler가 모두 read 가능 (REQ-AUTH2-003).
14. `TestFallsBackToHeadersWhenContextMissing` — AUTH-001 컨텍스트 없음
    + `X-User-Id: bob`, `X-Team-Id: team-b`, `X-Roles: admin,member`
    헤더 → handler에서 추출 가능 (REQ-AUTH2-003).
15. `TestFallsBackToAnonymousWhenAllMissing` — 컨텍스트도 헤더도
    없음 → user_id="anonymous", roles=[] (REQ-AUTH2-003).
16. `TestForwardCompatWithAUTH001Keys` — costguard.UserIDKey 의 source/
    의미가 본 SPEC 변경으로 영향받지 않음을 검증; AUTH-001의 기존
    테스트가 본 SPEC ship 후에도 PASS (NFR-AUTH2-005).
17. `TestEmptyTeamIDFallsBackToDefault` — config `default_team_id:
    "default"` + team_id empty → context team_id == "default"
    (REQ-AUTH2-004).
18. `TestEmptyTeamIDReturns400WhenDefaultBlank` — config `default_team_
    id: ""` + team_id empty → HTTP 400 + body `{"error":"team_id_
    required"}` (REQ-AUTH2-004).
19. `TestFallbackCounterIncrements` — empty team_id fallback 시
    `usearch_rbac_decisions_total{result="allow",reason_class="empty_
    team"}` +1 (REQ-AUTH2-004).
20. `TestDefaultTeamIDConfigurable` — `default_team_id: "engineering"`
    설정 시 empty team_id 가 "engineering"으로 채워짐 (REQ-AUTH2-004).
21. `TestTeamIDFromContextHelper` — `rbac.TeamIDFromContext(ctx)`
    helper가 모든 fallback 경로에서 일관된 string 반환 (REQ-AUTH2-006).

**GREEN tasks**:

- `internal/auth/rbac/context.go` 작성 — `TeamIDKey`, `RolesKey`
  타입 정의 + `TeamIDFromContext`, `RolesFromContext`, `UserIDFromContext`
  helper.
- `internal/auth/rbac/middleware.go` (1차 — TeamScopeMiddleware 만):
  AUTH-001 컨텍스트 → 헤더 → anonymous 우선순위, empty team_id 처리,
  카운터 emit.
- `.moai/config/sections/auth.yaml` 에 `auth.rbac.enabled`,
  `auth.rbac.default_team_id`, `auth.rbac.pg_dsn`, `auth.rbac.audit_to_
  stderr` 섹션 add.

**REFACTOR**:

- 컨텍스트 키 추출 로직을 단일 함수 `extractIdentity(ctx, r)` 로 분리
  (테스트 용이성).

---

### Phase D — EnforceMiddleware + Route Mapping + IndexQuery Wiring

목표: 본 SPEC의 핵심 — chi 미들웨어 통합 + route-resource 매핑 +
IndexQuery.TeamID 강제로 IDX-001 reservation 활성화.

**RED tests** (9):

22. `TestEnforceMiddlewareAllowsValidRequest` — context에 member role +
    valid policy → next.ServeHTTP 호출됨, status 200 (REQ-AUTH2-005).
23. `TestEnforceMiddlewareReturns403OnDeny` — context에 observer role +
    `team_index, write` 시도 → 403 + body `{"error":"forbidden",
    "resource":"team_index","action":"write"}` (REQ-AUTH2-005).
24. `TestRouteMappingTableConsistent` — `internal/auth/rbac/routes.go` 의
    table 이 spec §research 5.2의 11개 entry 정확히 보유 (REQ-AUTH2-005).
25. `TestPerAdapterCheckCalledFromFanout` — 가상의 fanout caller가
    `enforcer.Enforce(uid, tid, "adapter:reddit", "read")` 호출 →
    member role 통과, observer role도 통과 (read는 양쪽 모두 OK)
    (REQ-AUTH2-005, 007).
26. `TestQueryHandlerInjectsTeamIDIntoIndexQuery` — `POST /query` 호출
    시 query handler가 구성하는 IndexQuery 의 TeamID == context의
    team_id (REQ-AUTH2-006).
27. `TestQdrantFilterReceivesTeamID` — IndexQuery.TeamID = "team-a" 로
    dispatch.go 호출 → Qdrant client가 `Filter{TeamID:"team-a"}` 수신
    (REQ-AUTH2-006, IDX-001 호환).
28. `TestMeiliFilterReceivesTeamID` — IndexQuery.TeamID 가 Meili filter
    string `team_id = "team-a"` 로 포함됨 (REQ-AUTH2-006).
29. `TestPGFilterReceivesTeamID` — IndexQuery.TeamID 가 pg.Filters.
    TeamID 로 전파됨 (REQ-AUTH2-006).
30. `TestEnforceLatencyP95Under1ms` — 1000 회 Enforce + Histogram 측정,
    p95 < 1ms (NFR-AUTH2-001).

**GREEN tasks**:

- `internal/auth/rbac/routes.go` 작성 — route-resource table:
  `{Method:"POST", Path:"/query", Resource:"query:basic", Action:"read"}`
  외 10 entries.
- `internal/auth/rbac/middleware.go` 확장 — `EnforceMiddleware(resource,
  action) func(http.Handler) http.Handler`.
- `cmd/usearch-api/handlers/synthesis.go` 수정 — chi router 등록 시
  `rbac.EnforceMiddleware("query:basic", "read")` wrap + IndexQuery
  구성부에 `q.TeamID = rbac.TeamIDFromContext(ctx)` 한 줄 add.
- `cmd/usearch-api/handlers/deep.go` 수정 — 동일 패턴, resource=`query:
  deep`.
- `internal/auth/rbac/metrics.go` 작성 — Prometheus collector 등록
  (`usearch_rbac_decisions_total`, `usearch_rbac_eval_duration_seconds`
  Histogram).

**REFACTOR**:

- Enforce 호출 + metric emit + span attach를 단일 helper
  `evaluateAndEmit(ctx, sub, dom, obj, act)` 로 분리.
- 403 response body 생성을 별도 함수 `denyResponse(w, resource,
  action)`로 추출.

---

### Phase E — Per-Adapter Visibility + Admin Operations

목표: AdapterVisibility enum + registry 통합 + per-adapter authz +
admin endpoints (reload, member CRUD).

**RED tests** (12):

31. `TestAdapterRegistryHasVisibilityField` — `adapters.Adapter` 인터페이스
    또는 등록 함수에 Visibility 필드/메서드 존재 (REQ-AUTH2-007).
32. `TestDefaultAdapterVisibilityIsTeamShared` — 12 V1 adapter 모두
    `VisibilityTeamShared` (REQ-AUTH2-007).
33. `TestAdapterDenySkipsAdapterNotEntireRequest` — 가상의 query
    fanout 에서 adapter A는 deny, B는 allow → B의 결과만 합성, HTTP
    200 (NOT 403) (REQ-AUTH2-007).
34. `TestAdapterDenyIncrementsCounter` — adapter deny 시 카운터 +1
    (REQ-AUTH2-007).
35. `TestPersonalAdapterPolicyShape` — `AdapterVisibility.Personal` 등록
    시 정책 row가 `p, <owner>, <team>, adapter:<name>:<owner>, read,
    allow` shape 임을 helper 가 SHALL 생성 (V1.1 reserve; v1에서는
    helper 만 존재) (REQ-AUTH2-008).
36. `TestPersonalAdapterDeniedInV1` — V1에서 personal adapter (가상의
    `adapter:gmail:alice`) 정책 row가 emit 되지 않음 → enforce false
    (REQ-AUTH2-008).
37. `TestPersonalAdapterEnumPresent` — `AdapterVisibility.Personal`,
    `.AdminOnly`, `.TeamShared` 3 enum 모두 정의 (REQ-AUTH2-008).
38. `TestReloadEndpointAdminOnly` — `POST /admin/rbac/reload`를
    member role로 호출 → 403; admin로 호출 → 200 (REQ-AUTH2-009).
39. `TestReloadEndpointReturnsPolicyCount` — 200 응답 body에 `policy_
    count` 필드, reload 후 총 policy row 수와 일치 (REQ-AUTH2-009).
40. `TestReloadFailureKeepsExistingPolicy` — PG connection 차단 후
    reload → HTTP 500, 그러나 기존 enforcer 메모리 정책은 unchanged
    (다음 Enforce 호출 정상 동작) (REQ-AUTH2-009).
41. `TestReloadCounterIncrements` — success / failure 카운터 `usearch_
    rbac_policy_reload_total{outcome}` 정상 +1 (REQ-AUTH2-009).
42. `TestAddMemberEndpoint` + `TestRemoveMemberEndpoint` + `TestList
    MembersEndpoint` + `TestInvalidRoleReturns400` — REQ-AUTH2-010
    4개 시나리오.

**GREEN tasks**:

- `internal/adapters/visibility.go` 작성 — `AdapterVisibility` enum
  (`team_shared` / `personal` / `admin_only`) + `MakePersonalPolicyRow
  (owner, team, name)` helper.
- `internal/adapters/registry.go` 수정 — 등록 시 Visibility default
  `team_shared` 적용. 12개 adapter 명시 등록 (compile-time list).
- query fanout 코드 (위치는 `internal/sources/` 또는 `cmd/usearch-api/
  handlers/synthesis.go` 내부) 에 per-adapter enforce 호출 add. deny
  시 adapter skip + 카운터 emit + log.
- `internal/auth/rbac/admin.go` 작성 — 4개 endpoint handler. `enforcer.
  LoadPolicy()` atomic 보장은 casbin v2 의 RWMutex 의존.
- `cmd/usearch-api/main.go` 에서 admin route 등록:
  `r.Route("/admin", ...)` 그룹에 위 endpoint wire + `EnforceMiddleware
  ("rbac_policy","*")` 등 적용.

**REFACTOR**:

- per-adapter enforce 의 caller pattern 을 docs 에 명시 (fanout 코드
  reviewer 가 일관 적용 가능).
- admin handler 의 입력 validation 을 별도 helper 로 추출.

---

### Phase F — Observability + Audit Log + Production Hardening

목표: decision audit 3-surface 동시 emit + cardinality 화이트리스트 +
metrics_test 확장 + hot-reload.

**RED tests** (6):

43. `TestDecisionEmitsPrometheusCounter` — 모든 Enforce 결과 (allow/
    deny) 가 `usearch_rbac_decisions_total{result,reason_class}` 카운터
    1 증가 (REQ-AUTH2-011).
44. `TestDecisionEmitsOTelSpan` — `rbac.evaluate` span 생성, 6 attribute
    (user_id, team_id, resource, action, decision, eval_duration_ms)
    부착 (REQ-AUTH2-011).
45. `TestDecisionEmitsStderrJSONLine` — stderr 에 JSON line emit, schema
    (timestamp, event_type=`rbac.decision`, request_id, tenant_id,
    team_id, user_id, decision, resource, action, reason) 정확
    (REQ-AUTH2-011).
46. `TestAuditSchemaCompatWithAUTH003` — schema 가 SPEC-AUTH-003 audit
    log forward-compat (필수 필드 superset 검증) (REQ-AUTH2-011,
    NFR-AUTH2-008).
47. `TestNoUnboundedLabelsRBACAdded` — `internal/obs/metrics/metrics_
    test.go::TestNoUnboundedLabels` 화이트리스트에 `result`, `reason_
    class`, `outcome` 라벨 추가 후 통과 (NFR-AUTH2-003).
48. `TestConfigHotReloadOnSIGHUP` — auth.yaml 수정 후 SIGHUP → `default_
    team_id`, `audit_to_stderr` 변경, `pg_dsn`은 warning log + 무시
    (NFR-AUTH2-006).

**GREEN tasks**:

- `internal/auth/rbac/audit.go` 작성 — stderr JSON line emitter
  (encoding/json 으로 직접 write to os.Stderr; slog 의 stdout 과 분리).
  schema 는 AUTH-003 forward-compat 호환.
- `internal/auth/rbac/metrics.go` 의 OTel span attach 로직 완성
  (`otel.Tracer("rbac").Start(ctx, "rbac.evaluate")`).
- `internal/obs/metrics/metrics_test.go::TestNoUnboundedLabels` 화이트
  리스트에 신규 라벨 add.
- `internal/auth/rbac/config.go` (또는 main.go 내부) hot-reload 로직 —
  fsnotify watcher 또는 SIGHUP handler 로 `default_team_id`/`audit_to_
  stderr` live-update.

**REFACTOR**:

- audit emitter 를 별도 struct (`AuditEmitter`) 로 추출 — AUTH-003 이
  PostgreSQL sink 를 add 할 때 interface 통한 dual-write 가능하도록.
- 6개 OTel attribute key 를 상수로 centralize.

---

## 3. Test Catalog Summary

| Phase | Tests Added | REQs Covered | NFRs Covered |
|-------|-------------|--------------|--------------|
| A | 4 | 001 (partial), 002 (partial) | — |
| B | 7 | 001, 002 | 001, 004 |
| C | 10 | 003, 004, 006 | 005, 006 |
| D | 9 | 005, 006 | 001 |
| E | 12 | 007, 008, 009, 010 | — |
| F | 6 | 011 | 003, 006, 007, 008 |
| **Total** | **48** | **11 / 11** | **8 / 8** |

REQ-AUTH2-001/002는 Phase A (embed gate) + Phase B (런타임 검증)에
나누어 cover. REQ-AUTH2-003/004는 Phase C에 집중.

---

## 4. Risk Mitigation Table

| Risk | Phase | Mitigation Strategy |
|------|-------|---------------------|
| **R1** Casbin v2 → v3 migration 부담 | (out of v1 scope) | 별도 SPEC-AUTH-002-UPGRADE-001. v2 라인 고정 (research §2.1 D1). |
| **R2** Empty team_id 환경에서 cross-team data leak | Phase C | §1.1 D3: default_team_id fallback; operator 가 multi-team enforce 시 `default_team_id: ""` → HTTP 400. `TestEmptyTeamIDReturns400WhenDefaultBlank`. |
| **R3** Casbin enforcer hot-path 성능 저하 | Phase B + D | in-memory eval, NFR-AUTH2-001 (p95 < 1ms). `TestEnforceLatencyP95Under1ms`. role count ≤ 10 가정. |
| **R4** PG `casbin_rule` schema drift | Phase B | 명시 마이그레이션 `0003_casbin_rules.sql` ship. adapter version pin 변경 시 schema review checkpoint. |
| **R5** AUTH-001 forward-compat 깨짐 (컨텍스트 키 충돌) | Phase C | 신규 키 `auth.TeamIDKey`/`RolesKey` 만 additive 추가. costguard.UserIDKey/TenantIDKey 의미 unchanged. `TestForwardCompatWithAUTH001Keys`. |
| **R6** LoadPolicy 가 in-flight request 차단 | Phase E | casbin v2 RWMutex 로 짧게 (수십 ms) 차단. V1 policy 수 < 100. `TestEnforceLatencyP95Under1ms` 와 함께 검증. |
| **R7** Personal adapter 누설 (V1.1+) | Phase E | V1 에서 emit 안 됨. V1.1 enable 시 `keyMatch2` matcher + integration test 필수. 본 SPEC v1 은 design space (`AdapterVisibility.Personal` enum) 만 reserve. |
| **R8** policy 저장 connection 이 hot-path pgxpool 과 미격리 | Phase B | §1.1 D2: 별도 `*pg.DB`. `TestPGAdapterIsolatedConnection`. |
| **R9** stderr JSON line 의 stdout 오염 | Phase F | DEEP-004 REQ-DEEP4-010 패턴 차용. slog 는 stdout, audit 는 stderr 전용. |
| **R10** catch-all deny 가 admin 작업 차단 | Phase A + B | policy_effect 의 `!some(deny)` 의미상 allow 가 매치되면 catch-all 발동 안 함. `TestCatchAllDenyDoesNotBlockAdmin` 검증. |

---

## 5. MX Tag Plan

본 SPEC의 구현은 다음 @MX 태그를 생성한다.

### 5.1 @MX:ANCHOR (high fan_in, invariant contract)

- `internal/auth/rbac/enforcer.go::Enforce`
  — fan_in ≥ 3 (모든 EnforceMiddleware, per-adapter check, admin
  handler). 단일 정책 평가 진입점.
  `@MX:REASON`: deny-by-default invariant + policy_effect 의 의미가
  본 함수에 집중. 변경 시 모든 RBAC 결정 영향.
- `internal/auth/rbac/middleware.go::EnforceMiddleware`
  — fan_in ≥ 3 (synthesis, deep, admin handlers).
  `@MX:REASON`: route-resource 매핑 + 403 응답 형태 + audit emit의
  단일 진입점.
- `internal/auth/rbac/middleware.go::TeamScopeMiddleware`
  — fan_in ≥ 3 (모든 protected route).
  `@MX:REASON`: AUTH-001 forward-compat contract + empty team_id 처리
  + 헤더 fallback 의 source-priority 분기가 본 함수에 의존.

### 5.2 @MX:WARN (danger zone, requires @MX:REASON)

- `internal/auth/rbac/admin.go::ReloadHandler`
  — `@MX:WARN`: LoadPolicy 실패 시 기존 정책 atomic 보존이 핵심
  invariant. `@MX:REASON`: 실패 시 enforcer 가 손상되면 production
  outage. casbin v2 가 atomic replace 보장하지만 wrapper 코드에서
  실수로 partial state 노출 위험.
- `internal/auth/rbac/middleware.go::TeamScopeMiddleware` 의 헤더
  fallback 분기 — `@MX:WARN`: AUTH-001 비활성 환경에서 `X-Team-Id`
  헤더 spoofing 가능. `@MX:REASON`: AUTH-001 ship 후 production에서는
  헤더 fallback 사용을 금지하는 별도 가드 (e.g., `auth-001-ga=true` 에서
  헤더 무시) 필요. v1 은 transition 위해 유지.
- `internal/auth/rbac/enforcer.go::LoadPolicy` — `@MX:WARN`: RWMutex
  write lock 으로 in-flight request 차단. `@MX:REASON`: policy row
  1000+ 환경에서 lock duration 이 SLO 영향 가능. NFR-AUTH2-002 모니터링
  + 별도 SPEC 으로 incremental reload 검토.

### 5.3 @MX:NOTE (context & intent delivery)

- `internal/auth/rbac/model.conf` 파일 상단에 RBAC-with-domains 모델
  의미 + 4-tuple convention + casbin docs 링크.
- `internal/auth/rbac/policy_default.csv` 파일 상단에 role 계층 표현
  (D4) 의 정책 row 복제 패턴 설명 + V1.1 personal adapter shape 예시
  (주석으로).
- `internal/auth/rbac/context.go::TeamIDKey` 에 "auth.TeamIDKey 는
  본 SPEC 전용, deployment 단위 tenant 는 costguard.TenantIDKey 사용"
  노트.

### 5.4 @MX:TODO (incomplete work — resolved in GREEN phase)

- RED phase에서 placeholder 함수 (e.g., `MakePersonalPolicyRow`) 에
  `@MX:TODO` 부착하고 GREEN phase 종료 시 모두 제거.
- `internal/adapters/visibility.go::MakePersonalPolicyRow` 는 V1 에서
  caller가 없으므로 `@MX:TODO: V1.1 SPEC-AUTH-005 가 OAuth + per-user
  policy generation 시 호출 시작` 을 유지 (의도적 V1.1 reserve marker).

---

## 6. File Touch Order (recommended TDD progression)

1. **Phase A start**: `go.mod` direct require → `internal/auth/rbac/
   types.go` → `internal/auth/rbac/model.conf` → `internal/auth/rbac/
   policy_default.csv` → 4 tests.
2. **Phase B**: `internal/auth/rbac/adapter.go` → `internal/auth/rbac/
   enforcer.go` → `deploy/postgres/migrations/0003_casbin_rules.sql` →
   7 tests (`TestEnforcerThreadSafeUnderConcurrency` 는 race detector
   on).
3. **Phase C**: `internal/auth/rbac/context.go` → `internal/auth/rbac/
   middleware.go` (TeamScopeMiddleware 만) → `.moai/config/sections/auth.
   yaml` rbac 섹션 add → 10 tests.
4. **Phase D**: `internal/auth/rbac/routes.go` → `internal/auth/rbac/
   middleware.go` (EnforceMiddleware add) → `internal/auth/rbac/metrics.
   go` → `cmd/usearch-api/handlers/synthesis.go` patch → `handlers/deep.
   go` patch → 9 tests.
5. **Phase E**: `internal/adapters/visibility.go` → `internal/adapters/
   registry.go` patch → fanout per-adapter enforce 호출 add →
   `internal/auth/rbac/admin.go` → `cmd/usearch-api/main.go` admin
   route wiring → 12 tests.
6. **Phase F**: `internal/auth/rbac/audit.go` → OTel span attach +
   `internal/auth/rbac/metrics.go` 확장 → `internal/obs/metrics/metrics_
   test.go::TestNoUnboundedLabels` whitelist 확장 → hot-reload 로직
   add → 6 tests.

---

## 7. Coverage and Quality Gates

- Coverage 목표: 85% (per `quality.yaml`).
- 새 패키지 `internal/auth/rbac/` + `internal/adapters/visibility.go` 측정.
- TRUST 5 gates: 모든 phase 종료 시점에 `go vet` + `golangci-lint` +
  `go test -race` 통과.
- Cardinality test: `internal/obs/metrics/metrics_test.go::TestNoUnbounded
  Labels` 화이트리스트에 신규 라벨 (`result`, `reason_class`, `outcome`)
  추가 후 통과.
- LSP gate: zero errors / zero type errors / zero lint errors.
- AUTH-001 regression gate: `internal/auth/...`의 모든 기존 테스트가
  본 SPEC ship 후에도 통과 — 별도 CI gate으로 enforce.
- IDX-001 regression gate: `internal/index/...`의 모든 기존 테스트가
  본 SPEC ship 후에도 통과 (IndexQuery.TeamID 필드는 이미 존재; 본
  SPEC 은 source 만 wire).

---

## 8. Pre-submission Self-Review

전체 changeset이 완성된 시점에 다음을 확인한다:

- middleware chain 순서가 §research §5.1 을 반영하는가? (request-id →
  CORS/rate-limit → AUTH-001 JWT → costguard → AUTH-002 TeamScope →
  AUTH-002 Enforce → handler)
- `auth.TeamIDKey`/`auth.RolesKey` 가 strictly additive 인가? (AUTH-001
  의 기존 키 의미 unchanged)
- IndexQuery.TeamID 강제가 query handler 단 한 줄 추가로 3-store 모두
  활성화되는가?
- deny-by-default 가 모든 경로에서 보장되는가? (policy 누락, enforcer
  init 실패, empty team_id 모두 fail-closed)
- Prometheus 메트릭이 SPEC-OBS-001 명명 규칙 (`usearch_rbac_*`) 을
  따르는가?
- 모든 enum label 이 NFR-AUTH2-003 화이트리스트에 추가되었는가?
- admin endpoint 가 admin role 외에는 모두 차단되는가?
- 추가된 직접 의존성 (`casbin/v2`, `casbin-pg-adapter`) 이 go.mod 에
  deterministic version 으로 핀되어 있는가?

---

## 9. Implementation Sequencing Across Sessions

본 SPEC의 6개 phase 는 sequential 의존성을 가진다 (Phase C 의
TeamScopeMiddleware 는 Phase A 의 types/model 을 참조 등). 단일
manager-tdd 세션으로 완주가 어려운 경우 다음 세션 분할이 권장된다:

- **Session 1**: Phase A + B (Casbin pin + model/policy embed + enforcer
  core + PG adapter + thread-safety).
- **Session 2**: Phase C + D (identity extraction + AUTH-001 forward-
  compat + EnforceMiddleware + route mapping + IndexQuery wiring).
- **Session 3**: Phase E + F (per-adapter visibility + admin operations +
  observability + audit log + production hardening).

각 세션 시작 시 `/clear` 후 본 plan.md만 재로드하여 컨텍스트를 보존한다.

---

## 10. Decisions Log

본 SPEC 구현 중 다음 결정을 기록한다 (research §3.8 의 D1-D5 외
implementation-time decisions):

| ID | Decision | Phase | Rationale |
|----|----------|-------|-----------|
| **DI-1** | direct require add: `casbin/v2 v2.103.x` + `casbin-pg-adapter v1.5.0` | A | research §2.1 D1, §2.2 D2. minor 버전은 `go get` 시점 latest stable 확정. |
| **DI-2** | model.conf + policy_default.csv 는 `//go:embed` | A | runtime asset loading 회피 + binary self-contained. 운영자가 PG 정책 수정해도 default csv 는 bootstrap 시점에만 사용. |
| **DI-3** | bootstrap policy import 는 LoadPolicy 후 row count == 0 일 때만 | B | 운영자가 PG 정책 직접 수정 후 다음 startup 에서 덮어쓰지 않도록 보호. |
| **DI-4** | per-adapter check 는 middleware 가 아닌 application-level (fanout caller) | D + E | research §5.2 마지막 단락. middleware 는 route-resource 매핑만, adapter usage 는 query 실행 시점에 동적 결정. |
| **DI-5** | admin route 는 chi `r.Route("/admin", ...)` 그룹으로 분리 + `EnforceMiddleware("rbac_policy","*")` 등 적용 | E | route table 일관성 + admin 전용 가시화. |
| **DI-6** | audit emitter 는 별도 struct (`AuditEmitter`) 로 추출 | F | AUTH-003 ship 시 PostgreSQL sink dual-write 가능하도록 interface 통한 확장 여지. |

---

*End of SPEC-AUTH-002 plan.*
