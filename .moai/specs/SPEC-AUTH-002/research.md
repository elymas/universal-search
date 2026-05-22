# SPEC-AUTH-002 Deep Research

Generated: 2026-05-22T00:00:00Z
Author: manager-spec (Phase 0.5 — context-derived)
Consumed by: manager-spec (Phase 1B), plan-auditor (Phase 2.3)

---

## 0. Scope of This Research

본 research.md는 M6 milestone의 두 번째 deliverable인 SPEC-AUTH-002
(Team RBAC) 에 대한 코드베이스 분석 + 외부 라이브러리 조사 + 아키텍처
결정 기록이다. roadmap.md §M6 의 scope 한 줄("Casbin policy, team-scoped
queries, per-adapter visibility")을 (a) 정책 모델/저장소, (b) 미들웨어/
컨텍스트 전파, (c) 어댑터 가시성, (d) 역할 계층, (e) 정책 hot-reload,
(f) 관측·감사 surface 의 6개 축으로 분해해 각 축의 코드 진입점, 환경
변수, 메트릭, 마이그레이션, 보안 영향을 명세화한다.

본 SPEC 은 M6 의 "shared team plane" 단계를 여는 첫 RBAC 레이어다.
SPEC-AUTH-001(OIDC SSO + JWT 미들웨어)이 user identity 를 context 에
주입하고, SPEC-IDX-001 의 hybrid index 가 `team_id` 컬럼을 reserve 한
상태(v0.1 universally NULL)이므로 본 SPEC 은 이 두 토대 위에 (a) team
membership 추출, (b) Casbin 기반 인가, (c) team-scoped query filter
강제 라는 세 기능을 단일 미들웨어 chain 으로 통합한다. AUTH-002 가
ship 되어야 SPEC-IDX-004(shared index multi-tenancy 강제)와
SPEC-IDX-005(team-shared answer reuse)가 의미를 갖는다.

---

## 1. Architectural Context Pre-AUTH-002

### 1.1 AUTH-001 의 산출물 (M6 선행)

SPEC-AUTH-001(OIDC SSO)은 본 SPEC 의 직접 선행 의존성이다. 본 research 는
AUTH-001 이 다음 contract 를 ship 한다고 가정한다(AUTH-001 spec.md
검토 결과 — 본 research 와 동시에 작성 중인 상태이므로 contract 는
양방향 합의가 필요):

- JWT 검증 미들웨어가 `Authorization: Bearer <jwt>` 헤더를 파싱하여
  서명을 verify 한 뒤, 다음 claims 를 request context 에 주입한다:
  - `sub` (string) → user_id
  - `team_id` (string, optional) → 사용자가 현재 선택한 active team
  - `roles` ([]string, optional) → 해당 team 내 역할 (예:
    `["member"]`, `["admin", "audit_viewer"]`)
  - `tenant_id` (string, optional) → 배포 단위 tenant (multi-tenant
    deploy 시)
- 컨텍스트 키 이름: AUTH-001 은 새 패키지 `internal/auth/jwt` 또는
  `internal/auth` 에 `UserIDKey`, `TeamIDKey`, `RolesKey`, `TenantIDKey`
  4개 키를 export 한다. 본 SPEC AUTH-002 는 이들 키를 read-only 로
  소비한다.
- 미들웨어 chain 위치: AUTH-001 미들웨어는 chi v5 router 의 protected
  route group 진입 가장 앞단에 위치한다. JWT 검증 실패 시 HTTP 401 로
  단락 종료.
- Fallback (forward-compat 와 pre-AUTH-001 운영): JWT 가 없거나
  `auth-001-ga` env var 가 truthy 가 아닌 경우, AUTH-001 미들웨어는
  request 를 통과시키되 user_id = "anonymous", team_id = "" (empty
  string) 으로 컨텍스트를 설정한다. 본 SPEC 의 enforcer 는 empty
  team_id 를 "default team" 또는 "no team selected" 중 하나로
  처리하며 이는 §3.5 에서 결정한다.

### 1.2 SPEC-IDX-001 의 tenancy reservation (M3 implemented)

`.moai/specs/SPEC-IDX-001/spec.md:101-110` (REQ-IDX-010) 와
`pkg/types/normalized_doc.go` 의 `team_id` 필드, `deploy/postgres/
migrations/0001_create_docs.sql:17` 의 `team_id TEXT NULL` 컬럼, 그리고
`internal/index/dispatch.go:217,231-232,245` 의 IndexQuery.TeamID
filter 코드는 모두 SPEC-IDX-001 v0.1 이 reserve 한 단일-tenant
default 상태다.

- PG 컬럼: `team_id TEXT NULL` — v0.1 모든 INSERT 는 NULL.
- Qdrant payload: `internal/index/qdrant/client.go:242` 에서
  `Key: "team_id"` 로 payload filter 지원 진입점이 이미 마련됨.
- Meilisearch filterable attributes: `internal/index/index.go:108`
  에 `"team_id"` 가 등록됨.
- 한국어 shard: `internal/index/meili/korean_shard.go:27` 도
  `team_id` 가 filterable 로 등록됨.

SPEC-IDX-004(M6, 본 SPEC 의 downstream) 는 위 컬럼을 NOT NULL 로
flip 하고 row-level security 와 Qdrant payload-based partitioning 을
추가한다. 본 AUTH-002 는 IDX-004 의 입력(`team_id` source) 을 제공
한다.

### 1.3 SPEC-DEEP-004 의 costguard.TenantIDKey 와의 구분

`internal/deepagent/costguard/middleware.go:16` 의 `TenantIDKey` 는
이미 존재하며 의미는 "deployment 단위 tenant" 이다 (예: `acme-corp`
self-host 배포). 본 SPEC 이 도입하는 `team_id` 는 "한 tenant 안의
특정 team" (예: `acme-corp` 안의 `engineering-team`, `research-team`)
이라는 더 fine-grained scope 다.

| 개념 | 키 (소유자) | 출처 | 의미 |
|------|-------------|------|------|
| tenant_id | `costguard.TenantIDKey` (DEEP-004) | `X-Tenant-Id` 헤더 또는 AUTH-001 JWT `tenant_id` claim | 배포 단위 격리 (single-team self-host 에서는 = `"default"`) |
| user_id | `costguard.UserIDKey` (DEEP-004) → `auth.UserIDKey` (AUTH-001 wire) | JWT `sub` claim 또는 `X-User-Id` 헤더 | 개별 사용자 |
| team_id | `auth.TeamIDKey` (NEW, 본 SPEC) | JWT `team_id` claim 또는 `X-Team-Id` 헤더 | tenant 내 team 단위 격리 — RBAC domain |
| roles | `auth.RolesKey` (NEW, 본 SPEC) | JWT `roles` claim (team-scoped) | observer/member/admin 등 |

본 SPEC 은 costguard 의 TenantIDKey 의미를 변경하지 SHALL NOT 한다
(forward-compat). 단, 새 패키지 `internal/auth/rbac` 에 `TeamIDKey` 와
`RolesKey` 를 신설하고, costguard 가 필요 시 `auth.UserIDKey` 를 읽도록
M6 후속 리팩터링은 별도 PR 로 분리한다. v1 에서는 단순히 두 키를
독립적으로 운영한다.

### 1.4 V1 의 adapter visibility 분류 (product.md §4)

`.moai/project/product.md:34-43` 의 V1 scope:

> "4 source categories: web+social, academic+technical, Korean-locale,
>  personal-context (read-only, opt-in per user — scoped for V1.1 gate)"

본 research 는 다음과 같이 분류한다:

| Adapter 그룹 | 가시성 | V1 ship? | RBAC 정책 |
|--------------|--------|----------|-----------|
| **team_shared**: reddit, hn, arxiv, github, youtube, bluesky, x, naver, daum, koreannewscrawler, searxng, rss | team 전체 공유 | YES (M2-M3) | `p, ROLE_member, TEAM, adapter:NAME, read, allow` |
| **personal**: gmail, calendar, drive, obsidian, slack-private | 사용자 본인만 | NO (V1.1 deferred) | `p, USER, TEAM, adapter:NAME:USER, read, allow` — owner_user_id 매칭 |
| **admin_only**: (V1 에는 없음, 미래용) | admin 만 | NO | `p, ROLE_admin, TEAM, adapter:NAME, read, allow` |

V1 에서 personal 카테고리는 ship 되지 SHALL NOT 한다(product.md §4
명시). 그러나 본 SPEC 의 정책 모델은 personal 케이스를 표현 가능해야
한다 — V1.1 진입 시 정책만 추가하면 enforcer 가 즉시 적용한다.

### 1.5 Admin operations 의 범위 (M6 surface)

본 SPEC 의 admin 역할이 수행 가능한 operations:

- **member management**: 팀에 사용자 추가/제거, 역할 변경
- **policy reload**: Casbin policy hot reload 트리거
- **audit log view**: SPEC-AUTH-003(audit log) 의 query/export endpoint 접근
- **API key rotation**: external adapter API key (Naver, GitHub PAT 등)
  의 rotation. 실제 rotation 로직은 별도 SPEC-AUTH-004(M7+)에서 다루며,
  본 SPEC 은 endpoint 의 authz 만 정의한다.
- **adapter enablement toggle**: 팀별로 사용 가능한 adapter set 의 enable/
  disable. enablement 자체는 SPEC-IDX-004(M6) 가 enforce 한다.

본 SPEC 의 admin role 정의는 위 5개 operations 에 대한 authz 게이트만
포함한다. operation 자체의 구현은 각각의 owning SPEC 에 위임한다.

---

## 2. Casbin Library Choice

### 2.1 검토 대상

| Option | Module path | Latest | License | Notes |
|--------|-------------|--------|---------|-------|
| **A. casbin/v2** | `github.com/casbin/casbin/v2` | v2.103.x (stable line) | Apache-2.0 | 가장 많이 문서화·운영 사례, FilteredAdapter 호환. **권장**. |
| B. casbin/v3 (snapshot) | `github.com/casbin/casbin/v3` | v3.11.0-snapshot.3 (2026-05-06) | Apache-2.0 | 아직 snapshot. 운영 위험. |
| C. 직접 구현 (custom) | n/a | n/a | n/a | 표현력·검증·문서화 모두 손해. 기각. |
| D. OPA (open-policy-agent) | `github.com/open-policy-agent/opa` | v0.x stable | Apache-2.0 | Rego DSL — 학습 곡선 + 사이드카 운영 부담. 기각. |
| E. AWS Cedar (Go port) | n/a | preview | n/a | 미성숙. 기각. |

**결정 (D1)**: `github.com/casbin/casbin/v2` 의 최신 stable 라인 (run
phase 에서 정확한 minor pin 확정). 근거:

1. roadmap.md `.moai/project/roadmap.md:80` 와 tech.md `.moai/project/
   tech.md:70` 둘 다 "Casbin" 으로 명시.
2. 표준 RBAC-with-domains 모델 (request_definition `sub, dom, obj, act`)
   이 본 SPEC 의 4축 (user, team, resource, action) 과 1:1 매핑.
3. v2 는 production 사례 풍부 (CNCF 프로젝트, Apache Top-Level), v3 는
   2026-05-06 시점 snapshot 으로 GA 가 아직 불확실.
4. casbin-pg-adapter v1.5.0 (Apache, 2025-11-22) 가 v2 와 호환.

**Upgrade Policy**: v3 GA(non-snapshot stable tag) 가 발표되면 별도
SPEC-AUTH-002-UPGRADE-001 에서 v2→v3 마이그레이션을 다룬다. 본 SPEC
v1 은 v2 라인을 고정한다.

### 2.2 PostgreSQL 영속 어댑터

| Option | Module path | License | 호환성 |
|--------|-------------|---------|--------|
| **A. casbin-pg-adapter** | `github.com/casbin/casbin-pg-adapter` v1.5.0 | Apache-2.0 | FilteredAdapter 지원, go-pg 기반 자체 connection. **권장**. |
| B. casbin/gorm-adapter | `github.com/casbin/gorm-adapter/v3` | Apache-2.0 | GORM 의존, 우리 stack 은 pgx-only — 기각. |
| C. pckhoi/casbin-pgx-adapter (3rd party) | `github.com/pckhoi/casbin-pgx-adapter` | MIT | pgx 네이티브, 그러나 maintenance 정도 unclear. Open Question §11 에서 재검토. |

**결정 (D2)**: `github.com/casbin/casbin-pg-adapter v1.5.0` 을 V1 에
채택한다. 우리는 이미 `github.com/jackc/pgx/v5` (IDX-001 픽스) 를
사용하지만 정책 저장소는 **별도 pgxpool 또는 별도 *pg.DB** 로 격리한다.
이는 다음 이점이 있다:

- 정책 저장 connection 이 hot-path query connection 과 격리 → cap-
  exhausted 시 정책 reload 가 막히지 않는다.
- casbin-pg-adapter 가 go-pg 를 require 하지만 우리는 그것의 connection
  만 사용하므로 SQL 표현은 단순한 `casbin_rule` 테이블 하나뿐이다 —
  schema 단순.

**Open Question §11.4** 로 남김: M6+ 운영 데이터 수집 후 pckhoi/casbin-
pgx-adapter 로 단일-driver 통합을 재검토할지.

### 2.3 Casbin model file: RBAC with domains

`.moai/specs/SPEC-AUTH-002/research.md` 가 채택할 모델 (verbatim,
casbin docs https://casbin.apache.org/docs/rbac-with-domains 검증됨):

```ini
# internal/auth/rbac/model.conf
[request_definition]
r = sub, dom, obj, act

[policy_definition]
p = sub, dom, obj, act, eft

[role_definition]
g = _, _, _

[policy_effect]
e = some(where (p.eft == allow)) && !some(where (p.eft == deny))

[matchers]
m = g(r.sub, p.sub, r.dom) && r.dom == p.dom && keyMatch(r.obj, p.obj) && (r.act == p.act || p.act == "*")
```

설명:

- `r = sub, dom, obj, act` — request 4-tuple: (user_id, team_id,
  resource, action).
- `p = sub, dom, obj, act, eft` — policy 5-tuple: subject (user 또는
  role), domain (team), object, action, effect. effect=`allow|deny`.
- `g = _, _, _` — 3-인자 role mapping: (user, role, team). 사용자가
  특정 team 안에서 특정 role 을 가진다.
- `policy_effect` — `allow` 가 1개 이상 매치되고 `deny` 가 하나도 매치
  되지 않으면 통과. deny 가 매치되면 즉시 거부. allow 가 없으면 거부
  (deny-by-default).
- `matchers` — `g(user, subject_pattern, domain)` 으로 role mapping
  확인, domain 일치, object 는 `keyMatch` (RESTful path 매칭, e.g.,
  `adapter:*` 가 `adapter:reddit` 매칭) 으로 와일드카드 허용, action 은
  exact-match 또는 정책의 `*` 가 모든 action 매칭.

근거 (REQ-AUTH2-001 acceptance):

1. casbin docs RBAC-with-domains 표준 4-tuple 모델 — 외부 검증 완료.
2. `eft` 컬럼 명시는 deny-by-default 정책을 명확히 표현 가능.
3. `keyMatch` 는 `casbin/v2` builtin function — adapter 와 같은 계층적
   resource 표현에 유용 (`adapter:*` → 모든 adapter, `adapter:gmail:*` →
   private 적용 가능).

### 2.4 정책 예시 (V1 default policies)

`internal/auth/rbac/policy_default.csv` (적용 시 PG `casbin_rule` 테이블로
import; bootstrap 단계에서만 사용):

```csv
# Roles (g = role mapping: user, role, team)
g, role_observer, role_member, *
g, role_member, role_admin, *

# Note: 위 두 줄은 role 계층(observer < member < admin)을 표현. 단,
# Casbin g 는 user-role 매핑용이므로 role-role 계층은 별도 mechanism 으로
# 표현해야 한다. §3.3 에서 결정.

# Team-shared adapters: members can read
p, role_member, *, adapter:reddit, read, allow
p, role_member, *, adapter:hn, read, allow
p, role_member, *, adapter:arxiv, read, allow
p, role_member, *, adapter:github, read, allow
p, role_member, *, adapter:youtube, read, allow
p, role_member, *, adapter:bluesky, read, allow
p, role_member, *, adapter:x, read, allow
p, role_member, *, adapter:naver, read, allow
p, role_member, *, adapter:daum, read, allow
p, role_member, *, adapter:koreannewscrawler, read, allow
p, role_member, *, adapter:searxng, read, allow
p, role_member, *, adapter:rss, read, allow

# Query operations: members can issue queries
p, role_member, *, query:basic, read, allow
p, role_member, *, query:deep, read, allow

# Index writes: members can write to team-shared index
p, role_member, *, team_index, write, allow

# Observer can only read query results, not write
p, role_observer, *, query:basic, read, allow
p, role_observer, *, query:deep, read, allow
# (no team_index write)

# Admin operations: only admin role
p, role_admin, *, audit_log, read, allow
p, role_admin, *, member, *, allow
p, role_admin, *, rbac_policy, *, allow
p, role_admin, *, adapter_config, write, allow
p, role_admin, *, api_key, rotate, allow

# Personal-context (deferred V1.1): owner-only — example shape
# p, alice, team-A, adapter:gmail:alice, read, allow
# (정책은 V1 에서 emit 되지 SHALL NOT 함; V1.1 enable 시 owner_user_id
#  매칭 로직과 함께 활성화)

# Catch-all deny (deny-by-default 강화)
p, *, *, *, *, deny
```

**Notes**:

1. domain 위치에 `*` 사용 — `keyMatch` 또는 정책 매칭에서 domain
   wildcard. v1 default 는 모든 team 에 동일하게 적용. team-specific
   override 는 추가 row 로 표현.
2. `g, role_observer, role_member, *` 의 의미는 "role_observer 의 모든
   사용자는 role_member 의 권한도 가진다" 가 아니다 — Casbin g 는
   user-to-role 매핑이므로 role-to-role 계층은 직접 정책 row 로 표현
   해야 한다. §3.3 에서 재결정.
3. catch-all `p, *, *, *, *, deny` 는 policy_effect 의 `!some(deny)`
   조건에 의해 deny-by-default 의 백업 안전망. 모든 다른 allow 가 매치
   되지 않을 때 발동.

### 2.5 Casbin v2 Go API surface (본 SPEC 사용 메서드)

`pkg.go.dev/github.com/casbin/casbin/v2` 에 정의된 핵심 API:

- `casbin.NewEnforcer(model, adapter) (*Enforcer, error)` — model.conf
  과 adapter 로 enforcer 인스턴스화.
- `enforcer.Enforce(sub, dom, obj, act) (bool, error)` — 4-tuple
  policy decision. true=allow, false=deny.
- `enforcer.LoadPolicy() error` — adapter 에서 policy 재로드 (hot
  reload).
- `enforcer.SavePolicy() error` — 메모리 policy 를 adapter 에 저장.
- `enforcer.AddPolicy(sub, dom, obj, act, eft) (bool, error)` —
  단일 policy 추가 (member management 시 사용).
- `enforcer.RemovePolicy(sub, dom, obj, act, eft) (bool, error)` —
  단일 policy 제거.
- `enforcer.AddRoleForUserInDomain(user, role, dom) (bool, error)` —
  사용자에게 특정 team 안에서 role 부여.
- `enforcer.GetRolesForUserInDomain(user, dom) []string` — 사용자의
  특정 team 안에서의 role 목록.
- `enforcer.EnableAutoSave(false)` — AddPolicy 가 즉시 DB 저장하지
  않도록 함 (batch 작업 시).

호환성 검증: casbin-pg-adapter v1.5.0 의 `NewAdapter(connString)` 또는
`NewAdapterByDB(*pg.DB)` 가 위 인터페이스를 만족.

---

## 3. Decision Points

### 3.1 Identity Source Priority

본 SPEC 의 enforcer 는 다음 우선순위로 (user_id, team_id, roles) 를
추출한다:

1. **AUTH-001 JWT context** (최우선): `r.Context().Value(auth.UserIDKey)`,
   `auth.TeamIDKey`, `auth.RolesKey` 를 읽는다.
2. **HTTP header fallback** (pre-AUTH-001 또는 `auth-001-ga` 미설정):
   `X-User-Id`, `X-Team-Id`, `X-Roles` (comma-separated).
3. **Default fallback**: user_id="anonymous", team_id=`""`, roles=[].

team_id 가 empty 또는 missing 인 경우의 처리는 §3.5.

### 3.2 Empty team_id 의 처리

가능한 4가지 정책:

- (a) **HTTP 400** with `{"error":"team_id_required"}` — 가장 엄격.
- (b) **default team 자동 할당**: `costguard.DefaultTenantID` 와 동일한
  값(`"default"`) 으로 team_id 설정 — 단일-team self-host 환경에 편리.
- (c) **enforce skip**: team_id 가 비면 RBAC 평가 자체를 건너뛴다 —
  insecure, 기각.
- (d) **anonymous team**: team_id=`"anonymous"` 라는 special domain —
  V1 안전망.

**결정 (D3)**: V1 default 는 (b) — `.moai/config/sections/auth.yaml`
의 `auth.rbac.default_team_id`(기본값 `"default"`) 로 fallback. 운영자가
multi-team 환경으로 전환하면 `default_team_id: ""` 로 설정해 (a) 동작
으로 전환 가능. AUTH-001 JWT 미들웨어가 ship 되면 일반적으로 JWT 가
team_id 를 채워주므로 본 fallback 은 pre-AUTH-001 안전망 역할.

### 3.3 Role hierarchy (observer < member < admin)

Casbin g (3-arg) 는 user-role 매핑이며 role-role 계층 표현은 두 방식:

- **A. 정책 복제**: admin 정책 row 를 member 정책 row 의 superset 으로
  복제. 정책 row 수가 증가하나 의미 명확.
- **B. role hierarchy 전용 별도 mechanism**: enforcer 에 custom function
  추가, matcher 에서 `roleAtLeast(r.sub, p.required_role)` 같은 호출.
  Casbin v2 가 custom function 지원하나 maintenance 부담.
- **C. expansion at policy reload time**: admin 이 추가될 때마다 enforcer
  가 member 정책도 자동 add. AddRoleForUserInDomain 의 부수 효과로
  구현 가능. trade-off: 정책 row 가 자동 부풀어 오름.

**결정 (D4)**: **A (정책 복제)** 를 채택. 단순성 우선. role 계층은 3
레벨뿐이며, 각 역할의 정책 row 수도 ~20개 이하로 작다. 운영자가
`policy_default.csv` 를 읽으면 "admin 이 어떤 권한을 가지는지" 가
즉시 보인다 — debugging 용이성. V1.1 이상에서 role 수가 늘면 (C) 로
전환 검토.

**역할별 권한 매트릭스** (V1):

| Resource | observer | member | admin |
|----------|----------|--------|-------|
| `query:basic` read | ✓ | ✓ | ✓ |
| `query:deep` read | ✓ | ✓ | ✓ |
| `team_index` write | — | ✓ | ✓ |
| `adapter:reddit` (외 team_shared) read | ✓ | ✓ | ✓ |
| `adapter:gmail:OWNER` (personal, V1.1) read | — | owner only | owner only |
| `audit_log` read | — | — | ✓ |
| `member` add/remove/update | — | — | ✓ |
| `rbac_policy` add/remove/update | — | — | ✓ |
| `adapter_config` write | — | — | ✓ |
| `api_key` rotate | — | — | ✓ |

### 3.4 Per-Adapter Visibility

본 SPEC 의 정책 모델은 두 visibility class 를 지원한다:

- **team_shared**: object=`adapter:{name}`, all team members read.
- **personal**: object=`adapter:{name}:{owner_user_id}`, 단 owner 만
  read. enforcer matcher 에서 `keyMatch2` 로 owner 추출 후 r.sub 와
  비교.

V1 ship: team_shared 만. personal 정책은 정책 row 가 emit 되지 않으므로
enforcer 결정에서 자연스럽게 deny-by-default 됨. V1.1 enable 시
`policy_default.csv` 에 `p, alice, team-A, adapter:gmail:alice, read,
allow` 같은 row 가 추가되며 enforcer 가 즉시 작동.

**registry 에 visibility 메타데이터**: `internal/adapters.Registry`
의 adapter 등록 함수에 `Visibility AdapterVisibility` 필드 추가. enum:

```go
// internal/adapters/visibility.go (NEW, by AUTH-002)
type AdapterVisibility string

const (
    VisibilityTeamShared AdapterVisibility = "team_shared"
    VisibilityPersonal   AdapterVisibility = "personal"   // V1.1+
    VisibilityAdminOnly  AdapterVisibility = "admin_only" // future
)
```

V1 ship 되는 모든 12개 adapter (reddit, hn, arxiv, github, youtube,
bluesky, x, naver, daum, koreannewscrawler, searxng, rss) 는 모두
`VisibilityTeamShared`. registry 등록 시점에 default 가 team_shared
이므로 별도 코드 변경 없음.

### 3.5 Policy hot reload mechanism

세 가지 옵션:

- **A. Admin HTTP endpoint** `POST /admin/rbac/reload`: admin role 만
  접근 가능. enforcer.LoadPolicy() 호출.
- **B. fsnotify watcher on model.conf**: 모델 변경 시 자동 reload.
  model.conf 는 거의 변경되지 않으므로 효용 작음.
- **C. PG NOTIFY/LISTEN 기반 push reload**: casbin_rule 테이블 변경 시
  PG NOTIFY 발생 → 서비스가 LISTEN → 자동 reload. 가장 우아하나 추가
  코드 복잡도.

**결정 (D5)**: **A** 만 V1 에 채택. B 와 C 는 Open Question §11.3 에
남김. admin endpoint 는 chi route 로 `/admin/rbac/reload` 에 wire 하며
RBAC middleware 가 admin role 만 통과시킨다. 정책 변경 패턴:

1. 운영자가 PG `casbin_rule` 테이블 직접 변경 (또는 별도 admin UI 가
   AddPolicy/RemovePolicy API 호출).
2. `POST /admin/rbac/reload` 호출.
3. enforcer.LoadPolicy() 실행, 다음 요청부터 새 정책 적용.

### 3.6 Decision audit (forward-compat with SPEC-AUTH-003)

모든 RBAC 결정(allow + deny 양쪽)을 stderr JSON line 으로 emit. schema
는 SPEC-AUTH-003 의 audit log 와 호환되도록 다음 필드를 추가:

```json
{
  "timestamp": "2026-05-22T10:00:00.123Z",
  "event_type": "rbac.decision",
  "request_id": "req_abc123",
  "tenant_id": "default",
  "team_id": "engineering",
  "user_id": "alice@example.com",
  "decision": "allow|deny",
  "resource": "adapter:reddit",
  "action": "read",
  "reason": "matched policy p:role_member,*,adapter:reddit,read,allow"
}
```

deny 의 경우 `reason` 에는 매칭된 deny 정책 또는 "no allow policy
matched (default deny)" 가 들어간다. allow 의 경우 매칭된 allow 정책의
인덱스/요약.

SPEC-AUTH-003(audit log) 가 ship 되면 같은 schema 를 Postgres `audit_log`
테이블로 persist 한다. 본 SPEC v1 은 stderr 출력만 보장하며 영구
저장은 AUTH-003 의 책임.

### 3.7 Policy storage 경계 (Casbin vs application config)

**Casbin 에 저장**: subjects, objects, actions, role assignments.
**Casbin 에 저장 안 함**:
- adapter 의 visibility 메타데이터 (registry 의 in-memory enum)
- API key 자체 (별도 secret store, SPEC-SEC-001 M8)
- adapter 의 enablement (별도 team_adapter_enablement 테이블, SPEC-IDX-004 M6)

근거: Casbin 은 정책 평가에 최적화된 단순 KV. 메타데이터는 각각의
owning subsystem 에서 관리하고 enforcer 는 정책 row 만 본다.

### 3.8 Pinned Decisions (No User Re-prompt)

다음 5개 결정을 본 research 단계에서 context-derived 로 확정한다.
spec.md §1.1 에서 같은 번호로 재참조한다.

| ID | Decision | Recommendation | Alternatives Considered |
|----|----------|----------------|------------------------|
| **D1** | Casbin major version | `github.com/casbin/casbin/v2` 최신 stable | v3 snapshot (운영 위험), OPA (사이드카 부담), 직접 구현 (DRY 위반) |
| **D2** | Policy persistence | `github.com/casbin/casbin-pg-adapter v1.5.0` + 별도 connection | gorm-adapter (stack mismatch), pgx-native 3rd party (maintenance unclear), file-only (운영 부담) |
| **D3** | Empty team_id 처리 | deep.yaml `auth.rbac.default_team_id` (기본 `"default"`) fallback | HTTP 400 (UX 저하), enforce skip (insecure), anonymous-team (의미 모호) |
| **D4** | Role 계층 표현 | 정책 row 복제 (admin = member superset row) | custom matcher function (maintenance), expansion-at-AddRole (정책 부풀음) |
| **D5** | Policy hot reload | Admin `POST /admin/rbac/reload` endpoint only | fsnotify watcher (효용 작음), PG NOTIFY/LISTEN (복잡도) |

---

## 4. Storage Architecture

### 4.1 Postgres `casbin_rule` 테이블

`casbin-pg-adapter` 가 auto-create 하지만 본 SPEC 은 명시적
마이그레이션 파일 `deploy/postgres/migrations/0003_casbin_rules.sql` 을
ship 한다(idempotent). schema:

```sql
-- SPEC-AUTH-002 REQ-AUTH2-005: Casbin policy persistence
-- casbin-pg-adapter v1.5.0 호환 schema
CREATE TABLE IF NOT EXISTS casbin_rule (
    id          BIGSERIAL PRIMARY KEY,
    ptype       VARCHAR(100) NOT NULL,
    v0          VARCHAR(100),
    v1          VARCHAR(100),
    v2          VARCHAR(100),
    v3          VARCHAR(100),
    v4          VARCHAR(100),
    v5          VARCHAR(100)
);
CREATE INDEX IF NOT EXISTS idx_casbin_rule_ptype ON casbin_rule(ptype);
CREATE INDEX IF NOT EXISTS idx_casbin_rule_v0 ON casbin_rule(v0);
CREATE INDEX IF NOT EXISTS idx_casbin_rule_v1 ON casbin_rule(v1);
```

각 row 의 의미:

- `ptype = 'p'` → policy row, v0=sub, v1=dom, v2=obj, v3=act, v4=eft.
- `ptype = 'g'` → grouping row (user-role), v0=user, v1=role, v2=team.

마이그레이션은 IDX-001 의 `0001_create_docs.sql` + DEEP-004 의
`0002_cost_ledger.sql` 다음 번호.

### 4.2 Bootstrap policy import

서비스 startup 시:

1. enforcer 생성 (`casbin.NewEnforcer(model.conf, pgadapter)`).
2. `enforcer.LoadPolicy()` 실행 → PG 에서 기존 policy 로드.
3. 만약 PG 가 비어 있으면 (count=0), `internal/auth/rbac/policy_default.
   csv` 의 정책을 `enforcer.AddPolicies(...)` 로 일괄 add 후
   `SavePolicy()` 호출.
4. count > 0 이면 기존 정책 그대로 사용.

policy_default.csv 는 build 시점 embed (`//go:embed policy_default.
csv`). 운영자가 PG 의 정책을 직접 수정해도 다음 startup 에서
overwrite 되지 SHALL NOT 한다.

### 4.3 Connection pool 격리

- 정책 저장 connection: 별도 `*pg.DB` (go-pg 기반, casbin-pg-adapter
  가 require). 최대 2 connection (정책 read/write 는 빈도 매우 낮음).
- 정책 read 빈도: 매 요청 0회 (enforcer 가 메모리에 캐시한 policy 로
  결정). 단, `LoadPolicy()` 시점에 1회 SELECT *.
- 정책 write 빈도: admin endpoint 호출 시점에만 (시간당 ~0회 예상).

→ 별도 pool 의 부담은 무시할 수 있다.

---

## 5. Middleware Chain Position

### 5.1 chi v5 middleware order (V1.1 with AUTH-001)

```
Request
  → request-id middleware (SPEC-OBS-001)
  → CORS / rate-limit (existing)
  → AUTH-001 JWT validation middleware  ← AUTH-001 owns
        - JWT verify, claims → context (user_id, team_id, roles, tenant_id)
        - 401 if missing/invalid
  → costguard.IdentityMiddleware (DEEP-004)  ← reads ctx, fallback to X-* headers
  → costguard.CapCheckMiddleware (DEEP-004)
  → costguard.HaikuScreenMiddleware (DEEP-004)
  → AUTH-002 TeamScopeMiddleware  ← 본 SPEC, NEW
        - 만약 ctx 에 team_id 없으면 X-Team-Id 또는 default fallback
        - team_id 를 IndexQuery.TeamID 로 자동 attach 가능하도록 ctx 에 setter
  → AUTH-002 EnforceMiddleware  ← 본 SPEC, NEW
        - enforcer.Enforce(user_id, team_id, resource, action) 호출
        - resource 는 route 마다 다름 (e.g., `query:basic`, `query:deep`,
          `audit_log`, `member`, etc.)
        - allow → next.ServeHTTP, deny → 403 + decision event log
  → handler (synthesis.go, deep.go, etc.)
Response
  → costguard.LedgerWrite (response phase)
```

### 5.2 Route-to-resource mapping

`internal/auth/rbac/routes.go` 가 chi route 와 (resource, action) 매핑:

| Route | Resource | Action |
|-------|----------|--------|
| `POST /query` | `query:basic` | `read` |
| `POST /deep` | `query:deep` | `read` |
| `POST /admin/rbac/reload` | `rbac_policy` | `*` |
| `GET /admin/audit` | `audit_log` | `read` |
| `POST /admin/members` | `member` | `*` |
| `GET /admin/members` | `member` | `read` |
| `DELETE /admin/members/:id` | `member` | `*` |
| `POST /admin/api-keys/rotate` | `api_key` | `rotate` |
| `POST /admin/adapters/:name/enable` | `adapter_config` | `write` |
| (per-adapter usage during query fanout) | `adapter:{name}` | `read` |

per-adapter usage check 는 query handler 내부에서 호출 (각 fanout
worker 가 검사) — middleware 위치가 아닌 application 레벨.

### 5.3 Team-scoped query integration

`internal/index/dispatch.go` 의 IndexQuery 구조체는 이미 `TeamID
string` 필드를 가진다 (SPEC-IDX-001 REQ-IDX-010). 본 SPEC 은 query
handler 에서 IndexQuery 구성 시 `q.TeamID = auth.TeamIDFromContext(ctx)`
호출을 SHALL 강제한다.

dispatch.go:217-232 의 Qdrant filter, dispatch.go:231-232 의 Meili
filter, dispatch.go:245 의 PG filter 가 이미 TeamID 필드를 소비하므로,
IndexQuery 구성 단 한 줄만 추가하면 3-store 모두 team scope 가
강제된다.

SPEC-IDX-004(M6, blocked by 본 SPEC)는 v0.1 의 universally NULL 인
team_id 컬럼을 NOT NULL 로 flip 하고 row-level security 를 추가한다.
본 SPEC 은 IDX-004 의 입력(`team_id` ctx source) 을 제공하는 역할.

---

## 6. Observability Surface

### 6.1 Prometheus 메트릭

SPEC-OBS-001 NFR-OBS-002 의 cardinality safety 를 준수. 모든 label 값
은 bounded enum 또는 화이트리스트.

| Metric | Type | Labels | Cardinality |
|--------|------|--------|-------------|
| `usearch_rbac_decisions_total` | CounterVec | `{result, reason_class}` | 2 × 4 = 8 |
| `usearch_rbac_eval_duration_seconds` | Histogram | (no labels) | buckets [0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05] |
| `usearch_rbac_policy_reload_total` | CounterVec | `{outcome}` | 2 (success/failure) |
| `usearch_rbac_policy_count` | Gauge | (no labels) | 단일 값 |

- `result ∈ {allow, deny}`.
- `reason_class ∈ {policy_matched, no_policy_matched, explicit_deny,
  empty_team}`.

**중요**: `user_id`, `team_id`, `resource`, `action` 은 label 로 사용
하지 SHALL NOT 한다(unbounded). 이들 값은 OTel span attribute 와 audit
log 에 기록.

### 6.2 OTel span attributes

`rbac.evaluate` span:

- `rbac.user_id` (string, high-cardinality OK)
- `rbac.team_id` (string)
- `rbac.resource` (string)
- `rbac.action` (string)
- `rbac.decision` (string: "allow"/"deny")
- `rbac.eval_duration_ms` (float)

### 6.3 Decision event log (forward-compat with AUTH-003)

§3.6 schema. stderr JSON line. SPEC-AUTH-003 가 ship 되면 같은 schema
의 Postgres `audit_log` 테이블로 persist.

### 6.4 slog 통합

매 decision 마다 slog.LogAttrs(ctx, slog.LevelInfo, "rbac decision",
slog.String("decision", ...), ...) 호출. 응답 latency 에 영향
없어야 한다 — slog 의 비동기 sink 사용.

---

## 7. Pinned Decisions (No User Re-prompt) — re-stated

§3.8 의 5개 결정을 spec.md §1.1 에서 재명시한다.

---

## 8. Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|-----------|
| **R1** Casbin v2 → v3 migration 부담 (v3 GA 후) | Medium | Medium | 별도 SPEC-AUTH-002-UPGRADE-001 에서 v3 마이그레이션. 본 SPEC v1 은 v2 라인 고정. casbin v2 의 API 안정성은 5년+ 입증. |
| **R2** Empty team_id 환경에서 cross-team data leak | Medium | High | §3.2 D3: empty team_id 는 deep.yaml `default_team_id` (`"default"`) 로 fallback. 운영자가 multi-team 환경으로 전환 시 `default_team_id: ""` 로 명시 → 모든 empty 요청이 HTTP 400. |
| **R3** Casbin enforcer 가 hot-path 에서 느림 | Low | Medium | enforcer 는 in-memory policy 평가, ~100µs/eval (NFR-AUTH2-001 = p95 ≤ 1ms). casbin v2 의 RBAC-with-domains 모델은 O(role_count) 평가. role_count ≤ 10 가 V1 기준. |
| **R4** PG `casbin_rule` 테이블 schema drift (adapter version 변경) | Low | High | 명시적 `0003_casbin_rules.sql` 마이그레이션 ship. adapter version pin 변경 시 schema review checkpoint. |
| **R5** Forward-compat break with AUTH-001 (JWT claim name 충돌) | Medium | High | §1.1 의 4개 context key (`UserIDKey`, `TeamIDKey`, `RolesKey`, `TenantIDKey`) 를 AUTH-001 spec.md drafting 단계에서 양방향 검토. 본 SPEC 에 명시. |
| **R6** Policy reload 가 in-flight request 차단 | Low | Medium | casbin v2 enforcer 의 `LoadPolicy()` 는 RWMutex 의 write lock 으로 짧게 (수십 ms) 차단. 대량 정책(1000+ row) 인 경우 reload 가 길어질 수 있으나 V1 기준 < 100 row 이므로 무시할 수준. |
| **R7** personal adapter 정책 누설(V1.1+) — owner_user_id 가 잘못 추출 | Low (V1.1+) | High | V1 에서 emit 되지 않음. V1.1 enable 시 `keyMatch2` matcher 의 caller 검증 + integration test 필수. 본 SPEC v1 의 범위 밖이나 design space 만 reserve. |
| **R8** Casbin go-pg connection 이 우리 pgx pool 과 격리되지 않으면 hot-path 영향 | Low | Medium | §2.2 D2: 정책 저장은 별도 connection. pgxpool 과 분리. |
| **R9** stderr JSON line 의 stdout 오염 (slog 와 충돌) | Low | Low | SPEC-OBS-001 의 slog 가 stdout 사용. 본 SPEC 의 decision event log 는 stderr 전용 (REQ-DEEP4-010 패턴 그대로 차용). |
| **R10** policy_default.csv 의 catch-all deny 가 admin 작업 차단 | Medium | High | catch-all `p, *, *, *, *, deny` 의 우선순위는 policy_effect 의 `!some(deny)` 에 의해 결정. allow policy 가 매치되면 catch-all 은 발동하지 않음. `TestCatchAllDenyDoesNotBlockAdmin` 으로 검증. |

---

## 9. Files to Create / Modify

### 9.1 Created

| Marker | Path | Purpose |
|--------|------|---------|
| [NEW] | `internal/auth/rbac/enforcer.go` | Casbin enforcer wrapper, init, LoadPolicy, Enforce, AddRoleForUserInDomain helpers |
| [NEW] | `internal/auth/rbac/adapter.go` | casbin-pg-adapter 초기화 + 별도 pg connection 관리 |
| [NEW] | `internal/auth/rbac/middleware.go` | chi v5 미들웨어 — TeamScopeMiddleware, EnforceMiddleware |
| [NEW] | `internal/auth/rbac/context.go` | TeamIDKey, RolesKey, UserIDKey re-export + helpers (TeamIDFromContext, RolesFromContext, UserIDFromContext) |
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
| [NEW] | `internal/adapters/visibility.go` | AdapterVisibility enum + 등록 시 default = team_shared |
| [NEW] | `deploy/postgres/migrations/0003_casbin_rules.sql` | casbin_rule 테이블 schema |
| [NEW] | `.moai/config/sections/auth.yaml` | `auth.rbac.*` 설정 신설 |

### 9.2 Modified

| Path | Change |
|------|--------|
| `cmd/usearch-api/main.go` | enforcer 초기화 (casbin.NewEnforcer + adapter), default policy bootstrap, admin route wiring |
| `cmd/usearch-api/handlers/synthesis.go` | chi 미들웨어 체인에 `rbac.TeamScopeMiddleware` → `rbac.EnforceMiddleware("query:basic", "read")` 추가 |
| `cmd/usearch-api/handlers/deep.go` | 동일 패턴, resource="query:deep" |
| `internal/index/dispatch.go` | IndexQuery 구성부에 `q.TeamID = rbac.TeamIDFromContext(ctx)` 호출 추가 (이미 TeamID 필드 존재; 입력만 wire) |
| `internal/adapters/registry.go` | adapter 등록 시 Visibility 필드 (default team_shared) |
| `internal/obs/metrics/metrics.go` | `registerRBAC(r)` 헬퍼 호출 추가 |
| `internal/obs/obs.go` | `obs.RBACDecisions`, `obs.RBACEvalDuration` 등 신설 collector re-export |
| `internal/obs/metrics/metrics_test.go` | `TestNoUnboundedLabels` 화이트리스트에 `result`, `reason_class`, `outcome` 라벨 추가 |
| `.env.example` | `AUTH_RBAC_PG_DSN`, `AUTH_RBAC_DEFAULT_TEAM_ID` 등 신규 env-var 문서화 |
| `go.mod` | `github.com/casbin/casbin/v2`, `github.com/casbin/casbin-pg-adapter` 추가 |

### 9.3 Existing — Unchanged

- `internal/deepagent/costguard/` — DEEP-004 의 미들웨어 chain. AUTH-002
  는 그 뒤에 추가될 뿐 costguard 코드 자체는 unchanged.
- `internal/index/qdrant/`, `meili/`, `pg/` — IDX-001 의 team_id
  필터링 진입점 이미 존재. AUTH-002 는 IndexQuery 구성부만 변경.
- `services/researcher/` Python 사이드카 — Go-side RBAC 만 v1 적용.
  Python-side 호출의 authz 는 V1.1 별도 SPEC.

---

## 10. References

### 10.1 Internal SPEC documents

- `.moai/specs/SPEC-AUTH-001/spec.md` — (M6, parallel) OIDC SSO + JWT 검증.
  본 SPEC 의 user_id/team_id/roles context source.
- `.moai/specs/SPEC-AUTH-003/spec.md` — (M6, parallel) audit log. 본
  SPEC 의 decision event log schema 와 호환.
- `.moai/specs/SPEC-IDX-001/spec.md:101-110` — REQ-IDX-010 team_id 컬럼
  reservation. 본 SPEC 의 team-scoped query 강제 대상.
- `.moai/specs/SPEC-IDX-001/spec.md:540-560` — RBAC 통합 표면 (Qdrant
  payload + Meili filter + PG filter 의 team_id 진입점).
- `.moai/specs/SPEC-IDX-004/spec.md` — (M6, blocked by 본 SPEC) shared
  index multi-tenancy 강제. 본 SPEC 의 team_id source 를 소비.
- `.moai/specs/SPEC-IDX-005/spec.md` — (M6, blocked by 본 SPEC) team-shared
  answer reuse.
- `.moai/specs/SPEC-DEEP-004/spec.md:130-145` — costguard.TenantIDKey 와의
  의미 구분 근거.
- `.moai/specs/SPEC-OBS-001/spec.md` — NFR-OBS-002 cardinality safety;
  본 SPEC 신설 메트릭이 따른다.
- `.moai/specs/SPEC-LLM-001/spec.md` — `llm.Client` 패턴 (단일 choke
  point + middleware). 본 SPEC 의 enforcer 패턴이 동일 모티프 따른다.

### 10.2 External references (verified)

- Casbin RBAC with domains documentation:
  https://casbin.apache.org/docs/rbac-with-domains
  (모델 파일 syntax, tenant separation 예시, Enforce signature 검증)
- Casbin Go SDK:
  https://github.com/casbin/casbin (v2.103.x stable, Apache-2.0)
- Casbin PG adapter:
  https://github.com/casbin/casbin-pg-adapter (v1.5.0 2025-11-22,
  Apache-2.0, FilteredAdapter 지원)
- Casbin model overview:
  https://casbin.org/docs/syntax-for-models

### 10.3 Internal codebase references (file:line)

- `internal/deepagent/costguard/middleware.go:14-18` — 기존 context key
  패턴 (`UserIDKey`, `TenantIDKey`, `RequestIDKey`).
- `internal/deepagent/costguard/middleware.go:41-65` — IdentityMiddleware
  패턴. 본 SPEC 의 TeamScopeMiddleware 가 유사 구조.
- `internal/deepagent/costguard/middleware.go:73-126` — CapCheckMiddleware
  의 deny 응답 패턴 (HTTP 4xx + JSON body + decision event log). 본 SPEC
  의 EnforceMiddleware 가 동일 패턴 차용.
- `internal/index/dispatch.go:217,231-232,245` — IndexQuery.TeamID 의
  3-store filter 진입점 (현재 universally NULL, 본 SPEC 으로 활성화).
- `internal/index/qdrant/client.go:242` — Qdrant payload filter `team_id`
  진입점.
- `internal/index/meili/korean_shard.go:27` — Meilisearch 한국어 shard
  의 team_id filterable.
- `internal/index/index.go:108` — Meilisearch default index 의 team_id
  filterable.
- `deploy/postgres/migrations/0001_create_docs.sql:17` — `team_id TEXT
  NULL` 컬럼 reservation.
- `deploy/postgres/migrations/0002_cost_ledger.sql` — DEEP-004 의
  마이그레이션. 본 SPEC 의 `0003_casbin_rules.sql` 가 다음 번호.
- `cmd/usearch-api/main.go` — stub server. 본 SPEC 이 enforcer 초기화
  + admin route wiring 추가.
- `cmd/usearch-api/handlers/synthesis.go`, `deep.go` — chi 핸들러.
  미들웨어 chain 진입점.
- `internal/adapters/registry.go` — adapter 등록 진입점. 본 SPEC 이
  Visibility 필드 추가.
- `.moai/project/roadmap.md:78-86` — M6 row "SPEC-AUTH-002 | Team RBAC |
  Casbin policy, team-scoped queries, per-adapter visibility | expert-
  security".
- `.moai/project/tech.md:70` — "RBAC | Casbin | declarative policy".
- `.moai/project/product.md:34-43` — V1 adapter 카테고리 (4 source
  categories + personal-context V1.1 deferred).

---

## 11. Open Questions

본 research 의 5개 pinned decision 으로 대부분의 ambiguity 가 해소되었다.
다음 항목은 plan-auditor 와의 협의 또는 첫 운영 데이터 기반 튜닝이 필요한
경계 사례다.

1. **AUTH-001 JWT claim 이름 최종 확정**. 본 research §1.1 은 `sub`,
   `team_id`, `roles`, `tenant_id` 4개 claim 을 가정한다. AUTH-001 spec.md
   가 ship 되면 실제 claim 이름이 확정된다. **권장**: 본 SPEC 의 context
   key 명 (`UserIDKey`, `TeamIDKey`, `RolesKey`) 와 AUTH-001 의 wiring
   을 양방향 cross-reference 로 검토. **Resolution owner**: AUTH-001
   author + 본 SPEC author 의 joint review.

2. **role 계층 표현 전환 기준**. §3.3 D4 는 정책 row 복제 (option A) 를
   채택했으나 role 수가 5+ 로 늘면 expansion-at-AddRole (option C) 로
   전환 권장. **권장**: V1 ship 후 role count 가 5 를 넘으면 SPEC-AUTH-
   002-AMEND-001 에서 다룸. **Resolution owner**: 운영 데이터 기반.

3. **Policy hot reload 자동화 (fsnotify or PG NOTIFY)**. §3.5 D5 는
   admin endpoint 만 V1 채택. **권장**: M7 이후 운영 빈도가 시간당 1회
   이상이면 PG NOTIFY/LISTEN 도입 검토. **Resolution owner**:
   SPEC-AUTH-004(M7+) author.

4. **casbin-pg-adapter 의 go-pg 의존성 → pgx-native 전환**. §2.2 D2 는
   casbin-pg-adapter (go-pg 기반) 채택. **권장**: pgx-native 3rd party
   adapter 의 maintenance 상태가 안정되면 (또는 우리가 fork 운영) 단일
   driver 통합. **Resolution owner**: M8 SPEC-DEP-002 dependency audit.

5. **personal adapter (gmail, drive 등) V1.1 활성화 시점**. product.md
   §4 가 deferred 로 명시. **권장**: V1 ship 직후가 아닌 V1.1 별도 SPEC
   에서 OAuth + per-user policy generation 통합. 본 SPEC 은 design space
   만 reserve (`AdapterVisibility.Personal` enum 정의).
   **Resolution owner**: SPEC-AUTH-005(V1.1) author.

6. **multi-team active 컨텍스트 (한 user 가 동시 여러 team)**. 본 SPEC
   v1 은 요청당 정확히 하나의 active team_id 만 처리한다 (JWT `team_id`
   claim 또는 `X-Team-Id` 헤더). 사용자가 두 team 에 속해도 한 요청은
   한 team scope. **권장**: V1.1 에서 multi-team UI 추가 시 클라이언트
   가 team_id 를 명시적으로 선택해 전송하는 패턴 유지. **Resolution
   owner**: SPEC-UI-001 author.

위 6개는 plan-auditor 가 SPEC 을 PASS 로 평가하기 위해 필수적인 결정이
아니다. 모두 first-30-day 운영 데이터로 튜닝 가능한 항목이다.

---

*End of SPEC-AUTH-002 research document.*
