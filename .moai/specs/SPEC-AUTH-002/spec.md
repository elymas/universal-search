---
id: SPEC-AUTH-002
title: Team RBAC
version: 1.0.0
milestone: M6 — Team plane
status: implemented
priority: P0
owner: expert-security
methodology: tdd
coverage_target: 85
created: 2026-05-22
updated: 2026-05-22
author: limbowl
issue_number: 0
depends_on: [SPEC-AUTH-001, SPEC-IDX-001, SPEC-OBS-001]
blocks: [SPEC-IDX-004, SPEC-IDX-005, SPEC-AUTH-003]
---

# SPEC-AUTH-002: Team RBAC (Casbin RBAC-with-domains, team-scoped queries, per-adapter visibility)

## HISTORY

- 2026-05-22 (v1.0.0 implemented, commit 932acd1047efa86a01b65ef4a915764e9104e2f5): RED-GREEN-REFACTOR complete via TDD. internal/auth/rbac/ package with Casbin model.conf + policy CSV, enforcer singleton, RBAC-with-domains pattern implementing team-scoped query enforcement and per-adapter visibility gates. Race-clean tests pass.

- 2026-05-22 (initial draft v0.1.0, limbowl via manager-spec):
  First EARS-formatted SPEC for the M6 release gate's **second deliverable**.
  AUTH-002는 M6 (Team plane)의 **direct downstream of AUTH-001**이자
  **direct blocker for IDX-004 / IDX-005**다. AUTH-001 (OIDC SSO + JWT
  validation middleware)이 ship한 `costguard.UserIDKey` + 신규 도입될
  `auth.TeamIDKey` / `auth.RolesKey` 컨텍스트 키를 read-only로 소비하여
  (a) Casbin RBAC-with-domains 평가, (b) team-scoped IndexQuery 강제,
  (c) per-adapter visibility 게이트 — 세 기능을 단일 chi 미들웨어
  체인으로 통합한다.

  본 SPEC은 또한 SPEC-IDX-001 v0.1이 reserve한 `team_id` 컬럼/payload/
  filterable attribute (`internal/index/dispatch.go:217,231-232,245`,
  `deploy/postgres/migrations/0001_create_docs.sql:17`)을 **처음으로
  실제 활성화**한다. IDX-001의 universally-NULL 상태는 본 SPEC을 통해
  per-request team scoping이 강제되는 상태로 전환되며, 이후 SPEC-IDX-004
  (M6, blocked by 본 SPEC)가 컬럼을 NOT NULL로 flip하고 row-level
  security를 추가한다.

  핵심 5개 결정사항은 research §3.8에서 context-derived로 확정되었고
  §1.1에 재명시한다.

  Pinned decisions:
  (D1) Casbin major version: `github.com/casbin/casbin/v2` 최신 stable.
       v3 snapshot은 운영 위험으로 reject. OPA / 직접구현은 fit-for-purpose
       기각.
  (D2) Policy persistence: `github.com/casbin/casbin-pg-adapter v1.5.0`
       + 별도 `*pg.DB` connection (hot-path pgxpool과 격리). gorm-adapter
       기각 (GORM 미사용). pckhoi/casbin-pgx-adapter는 maintenance 불확실
       으로 Open Question에 보류.
  (D3) Empty team_id 처리: `auth.yaml`의 `auth.rbac.default_team_id`
       (기본값 `"default"`) fallback. 운영자가 multi-team 환경 진입 시
       `default_team_id: ""` 로 설정해 HTTP 400 동작으로 전환.
  (D4) Role 계층 표현 (observer < member < admin): 정책 row 복제 —
       admin은 member superset row를 명시적으로 보유. custom matcher
       function / expansion-at-AddRole 기각.
  (D5) Policy hot reload: Admin `POST /admin/rbac/reload` endpoint
       only. fsnotify watcher / PG NOTIFY-LISTEN은 Open Question에
       보류.

  M6 release gate의 두 번째 SPEC으로서 별도 GitHub Issue 트랙되지
  않으며 (`issue_number: 0`) plan-auditor 통과 후 status `draft →
  approved` 전이.

  Companion artifacts:
  - `.moai/specs/SPEC-AUTH-002/research.md` — Phase 0.5 research
    (~870 lines, 11 sections — pre-SPEC context, Casbin library matrix,
    decision points, storage architecture, middleware chain, observability
    surface, 5 pinned decisions, 10 risks, file touch list, 6 open questions)
  - `.moai/specs/SPEC-AUTH-002/plan.md` — TDD task sequence, 6-phase
    implementation, MX tag plan
  - `.moai/specs/SPEC-AUTH-002/acceptance.md` — Given/When/Then 시나리오
    (8 main + 2 boundary edges)
  - `.moai/specs/SPEC-AUTH-002/spec-compact.md` — compact view

  11 EARS REQs (10 × P0 + 1 × P1), 8 NFRs, 6 modules (Enforcer Core /
  Identity Extraction / Middleware Chain / Per-Adapter Visibility /
  Admin Operations / Observability). Methodology: TDD, coverage target
  85%, harness: standard. Owner: expert-security.

---

## 1. Overview

본 SPEC은 M6 milestone(Team plane)의 두 번째 deliverable이자 IDX-004/
IDX-005의 direct blocker인 Team RBAC 계층을 정의한다. SPEC-AUTH-001이
주입하는 user_id/team_id/roles 컨텍스트를 소비하여 (a) Casbin 기반
정책 평가, (b) IndexQuery의 team scope 자동 강제, (c) 12개 V1 어댑터의
visibility 게이트를 단일 chi v5 미들웨어 + 보조 핸들러 + 정책 저장소
계층으로 통합한다.

본 SPEC은 다음 6축을 하나의 패키지 `internal/auth/rbac/`로 ship한다.

1. **Enforcer core**: `casbin.NewEnforcer(model.conf, pgadapter)` 기반
   in-memory 정책 평가. RBAC-with-domains 4-tuple 모델 (sub=user,
   dom=team, obj=resource, act=action). deny-by-default.
2. **Identity extraction**: AUTH-001 JWT 컨텍스트 또는 X-User-Id/
   X-Team-Id/X-Roles 헤더 fallback. empty team_id는 D3 fallback 정책
   적용.
3. **Middleware chain**: chi v5 미들웨어 두 개 — `TeamScopeMiddleware`
   (team_id 추출 및 context 주입), `EnforceMiddleware(resource, action)`
   (route-specific authz 게이트).
4. **Per-adapter visibility**: `internal/adapters/visibility.go`의 새
   `AdapterVisibility` enum (team_shared / personal / admin_only). V1
   ship되는 12개 adapter는 모두 team_shared. 정책 모델은 V1.1 personal
   adapter (gmail, drive 등)도 표현 가능하도록 reserve.
5. **Admin operations**: `POST /admin/rbac/reload` (hot reload),
   `POST/GET/DELETE /admin/members` (team membership), `POST /admin/api-
   keys/rotate` (forward-compat with SPEC-AUTH-004). admin role만
   접근 가능.
6. **Observability surface**: Prometheus metrics
   (`usearch_rbac_decisions_total{result,reason_class}`,
   `usearch_rbac_eval_duration_seconds`,
   `usearch_rbac_policy_reload_total`), OTel span `rbac.evaluate`, stderr
   JSON line audit (SPEC-AUTH-003 forward-compat schema).

### 1.1 Pinned Architectural Decisions

다음 5개 결정은 research §3.8에서 context-derived로 확정되었다. 본 SPEC은
이를 EARS 요구사항으로 번역할 뿐 재논의하지 않는다.

1. **Casbin major version**: `github.com/casbin/casbin/v2` 최신 stable
   라인 (v2.103.x 기준; run phase에서 minor pin 확정). v3는 2026-05-06
   기준 `v3.11.0-snapshot.3` snapshot 상태로 운영 위험. OPA는 사이드카
   운영 부담, 직접 구현은 표현력/문서화/검증 모두 손해로 기각. v3 GA
   발표 후 별도 SPEC-AUTH-002-UPGRADE-001에서 v2 → v3 마이그레이션.
2. **Policy persistence**: `github.com/casbin/casbin-pg-adapter v1.5.0`
   + 별도 `*pg.DB` connection. casbin-pg-adapter가 go-pg 기반이지만
   우리 hot-path pgxpool과는 connection이 격리되므로 cap-exhausted 시
   정책 reload가 막히지 않는다. gorm-adapter는 우리 stack에 GORM 부재로
   기각.
3. **Empty team_id 처리**: V1 default는 `auth.yaml`의 `auth.rbac.default_
   team_id` (기본값 `"default"`) fallback. 운영자가 multi-team 환경으로
   전환 시 `default_team_id: ""` 설정으로 HTTP 400 동작 전환. enforce
   skip은 insecure로 기각, anonymous-team은 의미 모호로 기각.
4. **Role 계층 표현**: observer < member < admin 계층은 정책 row
   복제로 표현 — admin은 member 정책 row의 superset을 명시 보유. 운영자가
   `policy_default.csv`를 읽으면 각 역할의 권한이 즉시 보임 (debugging
   용이성). custom matcher function (maintenance 부담), expansion-at-
   AddRole (정책 row 부풀음)은 기각. V1 role count ≤ 10 가정.
5. **Policy hot reload**: V1은 admin `POST /admin/rbac/reload`
   endpoint만 채택. enforcer.LoadPolicy() 호출. fsnotify watcher (효용
   작음), PG NOTIFY/LISTEN (복잡도) 는 Open Question에 보류.

### 1.2 Motivation

M6은 "team plane"이다. M5까지 Universal Search는 single-tenant
self-host 가정 + 헤더 trust였다. M6은 **5명 이상의 팀**이 공유하는
환경에서 (a) 개별 사용자 quota 강제 [SPEC-AUTH-001 ship으로 완료],
(b) **per-team data isolation** [본 SPEC], (c) audit trail [SPEC-AUTH-003]
세 가지를 모두 제공해야 한다 (`.moai/project/roadmap.md` M6 exit
criterion).

본 SPEC ship 시점에 다음 contract가 활성화된다:

- 한 사용자가 발행하는 모든 query (basic, deep)는 그 사용자의 active
  team scope 안에서만 실행되며, 다른 team의 indexed document에 접근할
  수 없다.
- adapter 사용은 visibility 정책에 따라 게이트된다 — V1의 12개 어댑터는
  team-shared (모든 member 접근 가능), V1.1+의 personal adapter는 owner
  user만 접근 가능.
- admin operations (member 추가/제거, 정책 reload, audit log 조회, API
  key rotation)는 admin role 사용자만 가능하며, 모든 RBAC 결정은
  audit-log forward-compat schema의 stderr JSON line으로 emit된다.

### 1.3 Forward-compat with SPEC-AUTH-001 (Why additive context keys)

SPEC-AUTH-001은 `costguard.UserIDKey` (DEEP-004와 공유)와 `auth.ClaimsKey`
(전체 claim map)를 ship한다. 본 SPEC은 다음 **additive** 컨텍스트 키
2개를 신설한다:

- `auth.TeamIDKey` (NEW) — request의 active team domain. AUTH-001이
  JWT `team_id` claim을 추출해 주입하거나, 부재 시 본 SPEC의
  `TeamScopeMiddleware`가 `X-Team-Id` 헤더 또는 `default_team_id`
  fallback으로 주입.
- `auth.RolesKey` (NEW) — request의 team-scoped roles. AUTH-001이 JWT
  `roles` claim을 추출해 주입하거나 본 SPEC이 헤더 fallback으로 주입.

[HARD] 본 SPEC은 `costguard.UserIDKey`, `costguard.TenantIDKey`,
`auth.ClaimsKey`의 의미를 SHALL NOT 변경한다. costguard의 `TenantIDKey`
(배포 단위 tenant — 예: `acme-corp` self-host)와 본 SPEC의 `TeamIDKey`
(tenant 내 team — 예: `acme-corp` 안의 `engineering-team`)는 별개의
스코프이며, M6 후속 PR에서 costguard가 필요 시 `auth.UserIDKey` re-export를
참조하도록 점진 정리한다 (본 SPEC 범위 밖).

### 1.4 Forward-compat with SPEC-IDX-001 / SPEC-IDX-004

IDX-001 (`internal/index/dispatch.go:217,231-232,245`)는 `IndexQuery.
TeamID` 필드와 Qdrant/Meili/PG 3-store filter 진입점을 이미 reserve한
상태 (v0.1은 universally NULL). 본 SPEC은 query handler가 IndexQuery
구성 시 `q.TeamID = rbac.TeamIDFromContext(ctx)` 호출을 SHALL 강제
하여 IDX-001 reservation을 처음으로 활성화한다. 단일 라인 추가로
3-store team scoping이 모두 강제된다.

본 SPEC ship 후 SPEC-IDX-004 (M6, blocked by 본 SPEC)가 컬럼을 NOT
NULL로 flip하고 PG row-level security + Qdrant payload-based partitioning
을 추가한다. IDX-004의 입력 source가 본 SPEC이다.

---

## 2. EARS Requirements

### 2.1 Enforcer Core Module

| ID | Pattern | Requirement | Priority | Acceptance |
|----|---------|-------------|----------|------------|
| **REQ-AUTH2-001** | Ubiquitous | RBAC enforcer는 startup 시점에 `casbin.NewEnforcer(modelPath, adapter)`로 초기화 SHALL 한다. model 파일은 `internal/auth/rbac/model.conf` (RBAC-with-domains 4-tuple: `r = sub, dom, obj, act`; `p = sub, dom, obj, act, eft`; `g = _, _, _`; `policy_effect = some(where (p.eft == allow)) && !some(where (p.eft == deny))`; `matchers = g(r.sub, p.sub, r.dom) && r.dom == p.dom && keyMatch(r.obj, p.obj) && (r.act == p.act || p.act == "*")`)이며 build 시점에 `//go:embed model.conf`로 embed SHALL 한다. enforcer 초기화 실패 시 프로세스는 fatal exit SHALL 한다 (`auth.rbac.enabled: false`로 명시 비활성화한 경우 제외). enforcer 인스턴스는 singleton으로 모든 middleware/admin handler가 공유 SHALL 한다. (Acceptance §5.1) | P0 | `TestEnforcerInitFromEmbeddedModel`, `TestEnforcerInitFatalExitOnFailure`, `TestEnforcerSingletonReused`, `TestModelConfMatchesEmbed` |
| **REQ-AUTH2-002** | Ubiquitous | enforcer.Enforce(sub, dom, obj, act) 호출의 평가 결과는 deny-by-default SHALL 한다 — `policy_effect = some(where (p.eft == allow)) && !some(where (p.eft == deny))` 의미로, (a) `allow` 정책이 1개 이상 매치 AND (b) `deny` 정책이 0개 매치인 경우에만 PASS. allow 0개 매치 또는 deny 1개 이상 매치 시 모두 DENY. `policy_default.csv`의 catch-all row (`p, *, *, *, *, deny`)는 deny-by-default 안전망으로 SHALL 존재한다. enforcer는 thread-safe SHALL 하며 (casbin v2 내부 RWMutex로 보장) 동시 다중 request 평가에 안전 SHALL 하다. (Acceptance §5.1, §5.4) | P0 | `TestDenyByDefaultWhenNoAllowMatch`, `TestExplicitDenyOverridesAllow`, `TestCatchAllDenyRowPresent`, `TestEnforcerThreadSafeUnderConcurrency` |

### 2.2 Identity Extraction Module

| ID | Pattern | Requirement | Priority | Acceptance |
|----|---------|-------------|----------|------------|
| **REQ-AUTH2-003** | Event-Driven | WHEN incoming HTTP request가 protected route group에 진입하면, `rbac.TeamScopeMiddleware`는 다음 우선순위로 (user_id, team_id, roles) 3-tuple을 추출 SHALL 한다: (1) AUTH-001 JWT 주입 컨텍스트 — `costguard.UserIDKey` / `auth.TeamIDKey` / `auth.RolesKey` — 가 모두 존재하면 그것을 사용; (2) pre-AUTH-001 또는 `auth-001-ga` 미설정 시 헤더 fallback — `X-User-Id` / `X-Team-Id` / `X-Roles` (comma-separated); (3) default fallback — user_id=`"anonymous"`, roles=`[]`. team_id가 empty인 경우의 처리는 REQ-AUTH2-004에 위임한다. 추출된 3-tuple은 후속 미들웨어가 관찰할 수 있도록 `auth.TeamIDKey`/`auth.RolesKey` (그리고 헤더 fallback 시 `costguard.UserIDKey`)에 SHALL 주입된다. (Acceptance §5.2) | P0 | `TestExtractsFromAUTH001JWTContext`, `TestFallsBackToHeadersWhenContextMissing`, `TestFallsBackToAnonymousWhenAllMissing`, `TestForwardCompatWithAUTH001Keys` |
| **REQ-AUTH2-004** | Optional | WHERE 추출 결과 team_id가 empty string AND `auth.rbac.default_team_id`가 non-empty string인 경우, `TeamScopeMiddleware`는 team_id를 `default_team_id` 값(V1 기본값 `"default"`)으로 SHALL 채운다. WHERE team_id가 empty AND `default_team_id`도 empty (operator가 명시적으로 multi-team enforce 모드로 전환)인 경우, 미들웨어는 HTTP 400 + body `{"error":"team_id_required"}` 를 SHALL 반환한다. fallback 시점에 카운터 `usearch_rbac_decisions_total{result="allow",reason_class="empty_team"}` 1 증가 (operator가 fallback 사용량 모니터링 가능). (Acceptance §5.3) | P1 | `TestEmptyTeamIDFallsBackToDefault`, `TestEmptyTeamIDReturns400WhenDefaultBlank`, `TestFallbackCounterIncrements`, `TestDefaultTeamIDConfigurable` |

### 2.3 Middleware Chain Module

| ID | Pattern | Requirement | Priority | Acceptance |
|----|---------|-------------|----------|------------|
| **REQ-AUTH2-005** | Event-Driven | WHEN protected route handler 등록 시점에 `rbac.EnforceMiddleware(resource, action)` 가 wrap되면, 매 request마다 (a) `TeamScopeMiddleware`가 주입한 (user_id, team_id, roles)를 SHALL 읽어, (b) `enforcer.Enforce(user_id, team_id, resource, action)` 평가를 SHALL 수행하고, (c) decision == true → next.ServeHTTP, decision == false → HTTP 403 + body `{"error":"forbidden","resource":"<resource>","action":"<action>"}` 반환 SHALL 한다. route-resource 매핑은 `internal/auth/rbac/routes.go`의 table-driven 정의 (예: `POST /query` → (`query:basic`, `read`); `POST /deep` → (`query:deep`, `read`); `GET /admin/audit` → (`audit_log`, `read`); 등 §research 5.2 11개 매핑)를 SHALL 따른다. per-adapter usage check는 query fanout 핸들러 내부에서 호출되며 middleware 위치가 아닌 application 레벨로 위임한다. (Acceptance §5.4, §5.5) | P0 | `TestEnforceMiddlewareAllowsValidRequest`, `TestEnforceMiddlewareReturns403OnDeny`, `TestRouteMappingTableConsistent`, `TestPerAdapterCheckCalledFromFanout` |
| **REQ-AUTH2-006** | Ubiquitous | query handler (`cmd/usearch-api/handlers/synthesis.go`, `handlers/deep.go`)는 IndexQuery 구성 시 `q.TeamID = rbac.TeamIDFromContext(r.Context())` 호출을 SHALL 포함하여, SPEC-IDX-001이 reserve한 `internal/index/dispatch.go:217,231-232,245`의 3-store team_id filter (Qdrant payload, Meilisearch `team_id = "..."` filter, PostgreSQL WHERE 절)가 처음으로 실제 활성화 SHALL 되도록 한다. team_id가 empty string인 경우에도 (D3 fallback이 적용되었으므로) `default_team_id` 값이 전파되며, IDX-001 v0.1의 universally-NULL row와는 매칭되지 않는다 — 즉 v0.1로 indexed된 기존 row는 SPEC-IDX-004 마이그레이션 (NOT NULL flip) 까지는 별도 backfill SHALL NOT 한다. SPEC-IDX-004는 본 SPEC ship 후 즉시 진행된다. (Acceptance §5.6) | P0 | `TestQueryHandlerInjectsTeamIDIntoIndexQuery`, `TestQdrantFilterReceivesTeamID`, `TestMeiliFilterReceivesTeamID`, `TestPGFilterReceivesTeamID` |

### 2.4 Per-Adapter Visibility Module

| ID | Pattern | Requirement | Priority | Acceptance |
|----|---------|-------------|----------|------------|
| **REQ-AUTH2-007** | Ubiquitous | adapter registry (`internal/adapters/registry.go`)는 adapter 등록 시점에 `Visibility AdapterVisibility` 필드 (enum: `team_shared`, `personal`, `admin_only`)를 SHALL 보유한다. default 값은 `team_shared` SHALL 이며 V1 ship되는 12개 adapter (reddit, hn, arxiv, github, youtube, bluesky, x, naver, daum, koreannewscrawler, searxng, rss)는 모두 `team_shared`로 등록된다. query fanout 시점에 각 adapter 사용 전 `enforcer.Enforce(user_id, team_id, "adapter:"+adapter.Name(), "read")` 평가 SHALL 수행하며, deny 시 해당 adapter는 skip되고 다른 adapter의 결과만 합성된다 (HTTP 403 returned 하지 SHALL NOT — partial failure 모드). skip 시점에 카운터 `usearch_rbac_decisions_total{result="deny",reason_class="policy_matched"}` 1 증가. (Acceptance §5.7) | P0 | `TestAdapterRegistryHasVisibilityField`, `TestDefaultAdapterVisibilityIsTeamShared`, `TestAdapterDenySkipsAdapterNotEntireRequest`, `TestAdapterDenyIncrementsCounter` |
| **REQ-AUTH2-008** | Optional | WHERE V1.1 이후 personal adapter (gmail, drive 등)가 ship되면 `AdapterVisibility.Personal`로 등록 SHALL 되며, 정책 row 형태는 `p, <owner_user_id>, <team_id>, adapter:<name>:<owner_user_id>, read, allow`를 사용 SHALL 한다. enforcer matcher의 `keyMatch` 함수가 `adapter:gmail:*` wildcard와 `adapter:gmail:alice` exact match를 모두 표현하므로 owner_user_id별 정책을 분리 emit한다. V1 ship 시점에는 personal 정책 row가 SHALL NOT emit되며 모든 personal adapter 요청은 deny-by-default로 차단된다. 본 요구사항은 design space만 reserve하며 구현은 SPEC-AUTH-005 (V1.1)에 위임한다. (Acceptance §5.7 boundary) | P1 | `TestPersonalAdapterPolicyShape`, `TestPersonalAdapterDeniedInV1`, `TestPersonalAdapterEnumPresent` |

### 2.5 Admin Operations Module

| ID | Pattern | Requirement | Priority | Acceptance |
|----|---------|-------------|----------|------------|
| **REQ-AUTH2-009** | Event-Driven | WHEN admin role 사용자가 `POST /admin/rbac/reload`를 호출하면, 핸들러는 `enforcer.LoadPolicy()`를 SHALL 호출하여 PostgreSQL `casbin_rule` 테이블의 최신 정책을 메모리 enforcer로 reload SHALL 한다. reload 성공 시 HTTP 200 + body `{"status":"reloaded","policy_count":N}` (N은 reload 후 총 정책 row 수) 반환 SHALL 하며, 카운터 `usearch_rbac_policy_reload_total{outcome="success"}` 1 증가 SHALL 한다. reload 실패 시 (PG connection failure 등) HTTP 500 + body `{"error":"reload_failed","reason":"..."}` 반환 SHALL 하며 `usearch_rbac_policy_reload_total{outcome="failure"}` 1 증가 SHALL 한다. reload 실패 시에도 기존 메모리 enforcer는 unchanged SHALL 유지된다 (atomic replace — LoadPolicy 실패가 기존 정책을 손상시키지 SHALL NOT). admin이 아닌 role 호출 시 `EnforceMiddleware`가 403으로 차단. (Acceptance §5.8) | P0 | `TestReloadEndpointAdminOnly`, `TestReloadEndpointReturnsPolicyCount`, `TestReloadFailureKeepsExistingPolicy`, `TestReloadCounterIncrements` |
| **REQ-AUTH2-010** | Event-Driven | WHEN admin role 사용자가 member management endpoint를 호출하면, 다음 SHALL 동작한다: (a) `POST /admin/members` body `{"user_id":"alice","team_id":"engineering","role":"member"}` → `enforcer.AddRoleForUserInDomain("alice","role_member","engineering")` + `SavePolicy()` → HTTP 201; (b) `DELETE /admin/members?user_id=alice&team_id=engineering&role=member` → `enforcer.DeleteRoleForUserInDomain(...)` + `SavePolicy()` → HTTP 204; (c) `GET /admin/members?team_id=engineering` → 해당 team의 모든 (user_id, role) pair 반환. all 3 endpoint는 admin role 사용자만 접근 가능 (`EnforceMiddleware("member","*")` 적용). 잘못된 role 값 (observer/member/admin 외) 입력 시 HTTP 400 + body `{"error":"invalid_role"}`. (Acceptance §5.8) | P0 | `TestAddMemberEndpoint`, `TestRemoveMemberEndpoint`, `TestListMembersEndpoint`, `TestInvalidRoleReturns400` |

### 2.6 Observability Surface Module

| ID | Pattern | Requirement | Priority | Acceptance |
|----|---------|-------------|----------|------------|
| **REQ-AUTH2-011** | Ubiquitous | 본 SPEC이 신설하는 모든 RBAC 결정 (allow + deny 양쪽)은 다음 세 surface에 동시 emit SHALL 한다: (a) Prometheus 카운터 `usearch_rbac_decisions_total{result, reason_class}` 1 증가 — `result ∈ {allow, deny}`, `reason_class ∈ {policy_matched, no_policy_matched, explicit_deny, empty_team}`; (b) OTel span `rbac.evaluate` 생성, attribute `rbac.user_id`, `rbac.team_id`, `rbac.resource`, `rbac.action`, `rbac.decision`, `rbac.eval_duration_ms` 부착 (span attribute는 high-cardinality 허용); (c) stderr JSON line audit event — schema는 SPEC-AUTH-003의 audit log forward-compat 호환 (timestamp, event_type=`rbac.decision`, request_id, tenant_id, team_id, user_id, decision, resource, action, reason). 본 SPEC v1은 stderr 출력만 보장하며 PostgreSQL `audit_log` 테이블 persist는 SPEC-AUTH-003 책임. (Acceptance §5.9) | P0 | `TestDecisionEmitsPrometheusCounter`, `TestDecisionEmitsOTelSpan`, `TestDecisionEmitsStderrJSONLine`, `TestAuditSchemaCompatWithAUTH003` |

---

## 3. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| **NFR-AUTH2-001** | Enforcer evaluation latency | `enforcer.Enforce(sub, dom, obj, act)` 호출의 wall-clock latency는 in-memory policy 평가 경로에서 p95 ≤ 1 ms, p99 ≤ 3 ms SHALL 한다. casbin v2의 RBAC-with-domains 모델은 O(role_count) 평가이며 V1 role_count ≤ 10 가정. 측정: `usearch_rbac_eval_duration_seconds` Histogram의 p95 / p99 quantile. |
| **NFR-AUTH2-002** | Policy reload latency | `enforcer.LoadPolicy()` 호출의 wall-clock duration은 < 100 정책 row 환경에서 p99 ≤ 200 ms SHALL 한다. casbin v2의 LoadPolicy는 RWMutex write lock으로 in-flight request를 짧게 차단하므로 운영 영향 최소화. policy row 수가 1000+ 로 늘면 별도 SPEC에서 incremental reload 검토. |
| **NFR-AUTH2-003** | Cardinality safety | 본 SPEC이 신설하는 모든 Prometheus 메트릭 label 값은 bounded enumerable set이며 PII (user_id, team_id 원본 값)나 unbounded 값을 SHALL NOT 포함한다. `result` / `reason_class` / `outcome` enum label만 사용. high-cardinality 값 (user_id, team_id, resource, action)은 OTel span attribute 와 stderr audit log에만 기록. 검증: `internal/obs/metrics/metrics_test.go::TestNoUnboundedLabels` 화이트리스트에 `result`, `reason_class`, `outcome` 추가 후 통과. |
| **NFR-AUTH2-004** | Policy storage connection isolation | Casbin policy 저장 connection (`*pg.DB` for casbin-pg-adapter)은 우리 hot-path pgxpool과 SHALL 격리된다. 최대 2 connection만 사용 (정책 read 빈도 = startup + admin reload만; write 빈도 = admin endpoint만). hot-path pgxpool exhausted 상황에서도 정책 reload가 막히지 SHALL NOT 한다. |
| **NFR-AUTH2-005** | Forward-compat with SPEC-AUTH-001 context keys | 본 SPEC이 신설하는 `auth.TeamIDKey`, `auth.RolesKey`는 AUTH-001의 `costguard.UserIDKey`, `auth.ClaimsKey`와 동일한 패키지 `internal/auth/`에 export SHALL 된다. 단, costguard의 `TenantIDKey` 의미 (deployment 단위 tenant)는 SHALL NOT 변경된다 — 본 SPEC의 TeamIDKey는 tenant 내 team 단위 격리로 별개 의미. AUTH-001의 모든 기존 테스트는 본 SPEC ship 후에도 SHALL 통과한다. |
| **NFR-AUTH2-006** | Configurability via auth.yaml hot-reload | `auth.yaml`의 `auth.rbac.default_team_id`, `auth.rbac.enabled`, `auth.rbac.audit_to_stderr` 설정은 서비스 재시작 없이 SIGHUP 또는 fsnotify로 hot-reload 가능 SHALL 한다. 단 `auth.rbac.pg_dsn` (정책 저장소 connection string) 변경은 startup-only — hot-reload 시 무시되며 warning log. |
| **NFR-AUTH2-007** | Prometheus metric naming convention | 본 SPEC이 신설하는 모든 메트릭은 SPEC-OBS-001의 명명 규칙 (`usearch_<domain>_<noun>_<unit>` 패턴)을 SHALL 준수한다. 신설 메트릭 namespace는 `usearch_rbac_*`. label 화이트리스트는 NFR-AUTH2-003에 명시됨. |
| **NFR-AUTH2-008** | stderr/stdout separation for audit log | RBAC decision audit log JSON line은 stderr 전용 SHALL 이며, SPEC-OBS-001의 slog stdout 출력과 충돌하지 SHALL NOT 한다 (DEEP-004 REQ-DEEP4-010 패턴 그대로 차용). SPEC-AUTH-003 ship 시 stderr 출력은 그대로 유지되며 추가로 PostgreSQL `audit_log` 테이블에 persist된다 (additive). |

---

## 4. Exclusions (What NOT to Build)

[HARD] 본 SPEC은 다음 항목을 명시적으로 제외한다. 각 항목은 후속 SPEC 또는
별도 트랙의 책임이다.

- **JWT 검증 / OIDC discovery 없음** — JWT 서명 검증, JWKS rotation,
  iss/aud claim 검증은 SPEC-AUTH-001의 책임. 본 SPEC은 AUTH-001이 주입한
  컨텍스트 키를 read-only로 소비할 뿐이다.
- **Audit log persistence 미구현** — RBAC decision은 stderr JSON line만
  emit한다. PostgreSQL `audit_log` 테이블, replay 기능, S3 archive는
  SPEC-AUTH-003의 책임. 본 SPEC은 AUTH-003이 consume할 line schema의
  forward-compat만 보장한다.
- **API key rotation 실제 구현 없음** — `POST /admin/api-keys/rotate`
  endpoint는 본 SPEC에서 authz 게이트만 정의하며 (admin role만 접근),
  실제 rotation 로직은 SPEC-AUTH-004 (M7+)에 위임. v1은 endpoint 등록 +
  authz + 501 Not Implemented 응답만 ship한다.
- **Adapter enablement toggle 실제 구현 없음** — `POST /admin/adapters/:
  name/enable` endpoint는 본 SPEC에서 authz 게이트만 정의 (admin만 접근),
  실제 enablement 저장/조회는 SPEC-IDX-004 (M6, downstream)의 책임. v1은
  authz + 501 응답.
- **Personal adapter (gmail, drive 등) 활성화 미구현** — `AdapterVisibility.
  Personal` enum과 정책 row shape는 본 SPEC v1에서 reserve되며 V1.1
  ship 시 SPEC-AUTH-005에서 OAuth + per-user policy generation과 함께
  활성화. v1에서는 personal 정책 row가 emit되지 SHALL NOT 한다.
- **PG row-level security 없음** — IndexQuery에 team_id를 강제하지만
  DB-level RLS (PostgreSQL `CREATE POLICY ... USING (team_id = current_
  setting(...))`)는 SPEC-IDX-004의 책임. 본 SPEC은 application-layer
  enforcement만 제공.
- **Python sidecar RBAC 적용 없음** — `services/researcher/`, `services/
  storm/` Python sidecar 내부의 LLM 호출 / sub-query 생성은 Go-side
  ingress에서 인증된 후 internal RPC trust로 처리. Python-side authz는
  V1.1+ 별도 SPEC.
- **Multi-team active context 미지원 (v1)** — 한 request는 정확히 하나의
  active team_id만 처리한다 (JWT `team_id` claim 또는 `X-Team-Id` 헤더).
  사용자가 두 team에 속해도 한 요청은 한 team scope. multi-team 동시
  쿼리는 V1.1 별도 SPEC (SPEC-UI-001 의 client-side team selector).
- **Casbin v3 migration 없음** — v3는 2026-05-06 시점 snapshot 상태로
  운영 위험. v3 GA 발표 후 별도 SPEC-AUTH-002-UPGRADE-001에서 마이그레이션.
- **Self-service team management 없음** — 사용자가 자기 자신을 team에
  추가/제거하는 endpoint는 ship되지 SHALL NOT 한다. member management는
  admin role만 가능 (REQ-AUTH2-010). 셀프 서비스는 V2+ B2B SSO scenario에서.
- **Role hierarchy auto-expansion 없음** — observer < member < admin
  계층은 정책 row 복제로 정적 표현 (D4). AddRole 시점에 하위 role의
  정책을 자동 add하는 expansion은 V1.1+ role count > 5 일 때 별도 SPEC.
- **`cost_ledger` schema 변경 없음** — DEEP-004 cost_ledger와 본 SPEC
  은 직교한다. cost_ledger에 team_id 컬럼을 추가하지 SHALL NOT 한다
  (cost guard는 user 단위, RBAC은 team 단위로 분리 운영). M7+ team-
  level quota 가 본격화되면 별도 SPEC.

---

## 5. Acceptance Scenarios

상세 Given/When/Then 시나리오는 `.moai/specs/SPEC-AUTH-002/acceptance.md`에
정의되어 있다. 본 절은 인덱스를 제공한다.

| Scenario | 설명 | Coverage |
|----------|------|----------|
| §5.1 | enforcer init + valid policy → Enforce(member, team, query:basic, read) returns true | REQ-001, 002 |
| §5.2 | AUTH-001 JWT context 우선, 헤더 fallback도 동작 | REQ-003 |
| §5.3 | empty team_id + default fallback → "default" team scope; default 비어있을 때 400 | REQ-004 |
| §5.4 | role_observer가 team_index write 시도 → 403 (deny-by-default) | REQ-002, 005 |
| §5.5 | EnforceMiddleware: `/admin/audit` 호출 시 observer는 403, admin은 200 | REQ-005 |
| §5.6 | query handler가 IndexQuery.TeamID 강제 → 3-store filter 모두 활성화 | REQ-006 |
| §5.7 | reddit adapter (team_shared)는 member 접근 가능; gmail (personal V1.1)은 deny | REQ-007, 008 |
| §5.8 | admin reload endpoint + member add/remove/list | REQ-009, 010 |
| §5.9 | 모든 decision이 Prometheus + OTel span + stderr JSON line 동시 emit | REQ-011 |
| Edge1 | Casbin enforcer concurrency: 동시 1000 request 평가 시 race 없음, NFR latency 충족 | REQ-002, NFR-001 |
| Edge2 | LoadPolicy 실패 시 기존 메모리 enforcer 보존 (atomic replace) | REQ-009 |

---

## 6. Dependencies & Blocks

### 6.1 Upstream SPEC dependencies (depends_on)

- **SPEC-AUTH-001** (planned, M6 parallel) — OIDC SSO + JWT validation
  middleware. 본 SPEC의 user_id/team_id/roles context source. AUTH-001
  미들웨어가 본 SPEC의 `TeamScopeMiddleware`보다 앞에 chi 체인에 위치
  하여 JWT 검증 통과 시 컨텍스트를 주입한다.
- **SPEC-IDX-001** (implemented) — Hybrid index core. `IndexQuery.
  TeamID` 필드와 Qdrant/Meilisearch/PostgreSQL 3-store filter 진입점이
  이미 reserve된 상태 (`internal/index/dispatch.go:217,231-232,245`).
  본 SPEC이 query handler에서 IndexQuery 구성 시 TeamID를 강제 주입하여
  reservation을 활성화한다.
- **SPEC-OBS-001** (implemented) — Observability core. 신설 Prometheus
  메트릭이 OBS-001 registry에 등록되며 NFR-OBS-002 cardinality safety를
  SHALL 준수한다. request-id propagation 패턴과 OTel span attribute
  convention 재사용.

### 6.2 Downstream blocked SPECs (blocks)

- **SPEC-IDX-004** (planned, M6) — Shared index multi-tenancy enforcement.
  본 SPEC ship 후 IDX-001 v0.1의 universally-NULL `team_id` 컬럼을 NOT
  NULL로 flip하고 PG row-level security + Qdrant payload-based partitioning
  추가. IDX-004의 입력 source가 본 SPEC.
- **SPEC-IDX-005** (planned, M6) — Team-shared answer reuse. 팀 인덱스에서
  사전 lookup 시 `team_id` scoping에 본 SPEC의 컨텍스트 사용. AUTH-002
  ship 없이는 cross-team leak 위험.
- **SPEC-AUTH-003** (planned, M6) — Audit log subsystem. 본 SPEC의
  stderr JSON line schema를 inherit하고 PostgreSQL `audit_log` 테이블에
  persist. AUTH-003 ship 시 본 SPEC의 stderr 출력은 그대로 유지 (additive).

### 6.3 Cross-SPEC Joint Invariants

본 SPEC ship 시점에 다음이 SHALL 보장된다 (forward-compat):

- **AUTH-001 컨텍스트 키 unchanged**: `costguard.UserIDKey`,
  `costguard.TenantIDKey`, `auth.ClaimsKey`의 의미와 source 우선순위가
  본 SPEC 변경으로 영향받지 SHALL NOT 한다. 본 SPEC은 신규 키 (`auth.
  TeamIDKey`, `auth.RolesKey`) 만 additive로 추가한다.
- **IDX-001 schema unchanged**: `team_id TEXT NULL` 컬럼, Qdrant payload
  filter, Meilisearch filterable attribute는 본 SPEC 변경 없이 그대로
  활성화된다. 본 SPEC이 SQL migration을 추가하지 SHALL NOT 한다 (단,
  `0003_casbin_rules.sql`는 신규 정책 저장소이므로 IDX-001 schema와는
  무관).
- **DEEP-004 cost_ledger 의미 unchanged**: cost_ledger.user_id /
  tenant_id 컬럼의 의미와 source는 본 SPEC 변경 영향 없음. cost guard는
  user 단위 quota, RBAC은 team 단위 인가로 직교 운영.

본 commitment는 M6 release gate에서 schema review checkpoint로 재검증.

---

## 7. Files to Create / Modify

### 7.1 Created

| Marker | Path | Purpose |
|--------|------|---------|
| [NEW] | `internal/auth/rbac/enforcer.go` | Casbin enforcer wrapper, init, LoadPolicy, Enforce, AddRoleForUserInDomain helpers |
| [NEW] | `internal/auth/rbac/adapter.go` | casbin-pg-adapter 초기화 + 별도 pg connection 관리 |
| [NEW] | `internal/auth/rbac/middleware.go` | chi v5 미들웨어 — TeamScopeMiddleware, EnforceMiddleware |
| [NEW] | `internal/auth/rbac/context.go` | TeamIDKey, RolesKey export + helpers (TeamIDFromContext, RolesFromContext) |
| [NEW] | `internal/auth/rbac/routes.go` | Route → (resource, action) 매핑 table + helper |
| [NEW] | `internal/auth/rbac/admin.go` | admin endpoint handlers — POST /admin/rbac/reload, /admin/members |
| [NEW] | `internal/auth/rbac/audit.go` | decision event log stderr JSON line emitter |
| [NEW] | `internal/auth/rbac/metrics.go` | Prometheus collector 등록 헬퍼 |
| [NEW] | `internal/auth/rbac/types.go` | Role enum, Decision struct, AdapterVisibility 등 |
| [NEW] | `internal/auth/rbac/model.conf` | Casbin RBAC-with-domains 모델 (embed via go:embed) |
| [NEW] | `internal/auth/rbac/policy_default.csv` | V1 default policies (embed) |
| [NEW] | `internal/auth/rbac/enforcer_test.go` | enforcer init, LoadPolicy, role hierarchy, deny-by-default |
| [NEW] | `internal/auth/rbac/middleware_test.go` | 미들웨어 chain, team_id 추출, 403 on deny |
| [NEW] | `internal/auth/rbac/admin_test.go` | hot reload endpoint, admin-only 강제 |
| [NEW] | `internal/auth/rbac/audit_test.go` | decision event log schema, stderr 출력 검증 |
| [NEW] | `internal/auth/rbac/metrics_test.go` | label whitelist 준수, cardinality bounded |
| [NEW] | `internal/auth/rbac/routes_test.go` | route mapping table 검증 |
| [NEW] | `internal/auth/rbac/integration_test.go` | E2E: chi 체인 request-id → AUTH-001 → costguard → AUTH-002 → handler |
| [NEW] | `internal/adapters/visibility.go` | AdapterVisibility enum + 등록 시 default = team_shared |
| [NEW] | `internal/adapters/visibility_test.go` | enum 정의, default 적용 검증 |
| [NEW] | `deploy/postgres/migrations/0003_casbin_rules.sql` | casbin_rule 테이블 schema (idempotent) |
| [NEW] | `.moai/config/sections/auth.yaml` 추가 섹션 (`auth.rbac.*`) | rbac.enabled / default_team_id / pg_dsn / audit_to_stderr 등 신설 config |

### 7.2 Modified

| Path | Change |
|------|--------|
| `cmd/usearch-api/main.go` | enforcer 초기화 (casbin.NewEnforcer + adapter) + default policy bootstrap + admin route wiring (`/admin/rbac/reload`, `/admin/members`). 미들웨어 체인에 `rbac.TeamScopeMiddleware` 를 AUTH-001 JWT 미들웨어 직후, costguard 직후 위치에 add. |
| `cmd/usearch-api/handlers/synthesis.go` | chi 미들웨어 체인에 `rbac.EnforceMiddleware("query:basic", "read")` 추가. IndexQuery 구성부에 `q.TeamID = rbac.TeamIDFromContext(ctx)` 한 줄 추가. |
| `cmd/usearch-api/handlers/deep.go` | 동일 패턴, resource=`query:deep`. |
| `internal/index/dispatch.go` | (line 217 주변) IndexQuery 구성부 검토 — 본 SPEC은 query handler 측에서 team_id를 set하므로 dispatch.go 자체는 변경 없음을 검증. 단 코멘트에 "team_id source: SPEC-AUTH-002 (M6)" 1줄 추가. |
| `internal/adapters/registry.go` | adapter 등록 함수에 Visibility 필드 default `team_shared` 적용. 12개 V1 adapter는 모두 team_shared. |
| `internal/obs/metrics/metrics.go` | `registerRBAC(r)` 헬퍼 호출 추가. |
| `internal/obs/obs.go` | `obs.RBACDecisions`, `obs.RBACEvalDuration`, `obs.RBACPolicyReload`, `obs.RBACPolicyCount` collector re-export. |
| `internal/obs/metrics/metrics_test.go` | `TestNoUnboundedLabels` 화이트리스트에 `result`, `reason_class`, `outcome` 라벨 추가. |
| `.env.example` | `AUTH_RBAC_PG_DSN`, `AUTH_RBAC_DEFAULT_TEAM_ID`, `AUTH_RBAC_ENABLED`, `AUTH_RBAC_AUDIT_TO_STDERR` 등 신규 env-var 문서화. |
| `go.mod` / `go.sum` | direct require `github.com/casbin/casbin/v2 v2.103.x`, `github.com/casbin/casbin-pg-adapter v1.5.0` 추가. (run phase에서 minor pin 확정.) |

### 7.3 Existing — Unchanged (verified invariants)

- `internal/deepagent/costguard/` — DEEP-004 미들웨어. 본 SPEC은 후속에
  추가될 뿐 costguard 코드 자체는 unchanged. costguard.UserIDKey/
  TenantIDKey 의미도 unchanged.
- `internal/auth/` (SPEC-AUTH-001 산출물) — 본 SPEC은 AUTH-001이 export한
  `costguard.UserIDKey`, `auth.ClaimsKey`를 read-only로 consume. AUTH-001
  코드는 unchanged.
- `internal/index/qdrant/`, `meili/`, `pg/` — IDX-001의 team_id 필터링
  진입점은 이미 존재 (`internal/index/dispatch.go:217,231-232,245`,
  `internal/index/qdrant/client.go:242`, `internal/index/meili/korean_
  shard.go:27`, `internal/index/index.go:108`). 본 SPEC은 IndexQuery
  구성부만 변경하며 3-store 코드 자체는 변경 없음.
- `deploy/postgres/migrations/0001_create_docs.sql` (IDX-001) — schema
  unchanged.
- `deploy/postgres/migrations/0002_cost_ledger.sql` (DEEP-004) — schema
  unchanged.
- `services/researcher/`, `services/storm/` Python sidecar — Go-side
  RBAC만 v1 적용. Python-side authz는 V1.1+ 별도 SPEC.

---

## 8. Open Questions

본 SPEC은 §1.1의 5개 pinned decision으로 대부분의 ambiguity를 해소했다.
다음 항목은 plan-auditor와의 협의 또는 첫 운영 데이터 기반 튜닝이 필요한
경계 사례다.

1. **AUTH-001 JWT claim 이름 최종 확정**: 본 SPEC의 `TeamIDKey`/
   `RolesKey`가 소비하는 JWT claim 이름 (`team_id`, `roles`)을 AUTH-001
   ship 시점에 양방향 cross-reference로 확정. **권장**: 본 SPEC의 컨텍스트
   키 명 (`auth.TeamIDKey`, `auth.RolesKey`)와 AUTH-001의 wiring을
   joint review. **Resolution owner**: AUTH-001 author + 본 SPEC author.

2. **Role 계층 표현 전환 기준**: §1.1 D4는 정책 row 복제 (option A)
   채택. role 수가 5+ 로 늘면 expansion-at-AddRole로 전환 권장.
   **권장**: V1 ship 후 role count가 5를 넘으면 SPEC-AUTH-002-AMEND-001
   에서 다룸. **Resolution owner**: 운영 데이터 기반.

3. **Policy hot reload 자동화 (fsnotify or PG NOTIFY)**: §1.1 D5는 admin
   endpoint만 V1 채택. **권장**: M7 이후 운영 빈도가 시간당 1회 이상이면
   PG NOTIFY/LISTEN 도입 검토. **Resolution owner**: SPEC-AUTH-004 (M7+)
   author.

4. **casbin-pg-adapter의 go-pg 의존성 → pgx-native 전환**: §1.1 D2는
   casbin-pg-adapter (go-pg) 채택. **권장**: pgx-native 3rd party adapter
   의 maintenance 상태가 안정되면 단일 driver 통합. **Resolution owner**:
   M8 SPEC-DEP-002 dependency audit.

5. **Personal adapter V1.1 활성화 시점**: product.md §4가 deferred로
   명시. **권장**: V1 ship 직후가 아닌 V1.1 별도 SPEC에서 OAuth +
   per-user policy generation 통합. 본 SPEC v1은 design space (`Adapter
   Visibility.Personal` enum, 정책 row shape) 만 reserve. **Resolution
   owner**: SPEC-AUTH-005 (V1.1) author.

6. **Multi-team active 컨텍스트**: 본 SPEC v1은 요청당 정확히 하나의
   active team_id만 처리. 사용자가 두 team에 속해도 한 요청은 한 team
   scope. **권장**: V1.1에서 multi-team UI 추가 시 client가 team_id를
   명시적으로 선택해 전송하는 패턴 유지. **Resolution owner**:
   SPEC-UI-001 author.

위 6개는 plan-auditor가 SPEC을 PASS로 평가하기 위해 필수적인 결정이 아니다.
모두 first-30-day 운영 데이터 또는 후속 minor SPEC으로 튜닝 가능한 항목이다.

---

*End of SPEC-AUTH-002 v0.1.0 (draft).*
