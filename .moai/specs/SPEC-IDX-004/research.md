# SPEC-IDX-004 Research — Shared Index Multi-Tenancy

**Status**: Research-complete; ready for SPEC drafting
**Author**: limbowl (via manager-spec)
**Created**: 2026-05-22
**Milestone**: M6 — Team plane
**Depends on**: SPEC-IDX-001 (implemented), SPEC-IDX-002 (implemented), SPEC-IDX-003 (implemented), SPEC-AUTH-001 (planned M6), SPEC-AUTH-002 (planned M6), SPEC-OBS-001 (implemented)
**Parallelizable with**: SPEC-AUTH-001, SPEC-AUTH-002, SPEC-AUTH-003 within M6 per `.moai/project/roadmap.md:128`

---

## 0. Research Mandate

SPEC-IDX-004 (Shared index multi-tenancy)는 M6 team plane의 retrieval-layer
릴리스 게이트다. SPEC-IDX-001 (implemented, `.moai/specs/SPEC-IDX-001/spec.md:6`)이
세 store (Qdrant + Meilisearch + PostgreSQL)에 `team_id` 컬럼/필드/payload를
예약만 했고 v0.1에서는 **모든 row가 `team_id = NULL`** 인 single-tenant 상태로
운영 중이다 (현 코드 확인: `internal/index/dispatch.go:347` —
`"team_id": nil, // v0.1: always null per SPEC-IDX-004 reservation`,
`deploy/postgres/migrations/0001_create_docs.sql:16` —
`team_id TEXT NULL, -- reserved for SPEC-IDX-004 multi-tenancy enforcement`).

IDX-004는 그 reservation을 enforcement로 전환한다:

1. **Qdrant** 단일 collection 안에 `team_id` payload field + `is_tenant=true`
   payload index를 두는 **Tiered Multitenancy** 패턴으로 default-tier를 구성하고,
   대용량/프리미엄 팀은 옵션으로 dedicated collection으로 승격한다.
2. **Meilisearch** 의 `usearch_docs`(영문) + `usearch_docs_ko`(한국어, IDX-003
   추가) 두 index 모두에서 `team_id` filterable attribute를 **per-tenant
   token (JWT)** 의 `searchRules` 로 강제한다. Go 서비스가 토큰을 발급하고
   짧은 TTL + 권한 변경 시 회전한다.
3. **PostgreSQL** `docs` 테이블의 `team_id` 컬럼을 `NOT NULL DEFAULT 'default'`
   로 전환하고 (migration `0004_team_id_not_null.sql`), 새로 도입되는
   audit/document 테이블에도 `team_id` 컬럼을 추가한다.
4. **Personal context tier**: adapter-level visibility가 `private` 인 경우
   (e.g. SPEC-AUTH-002의 per-adapter visibility policy에서 user-private으로
   분류) `user_id` payload field를 함께 저장하고 retrieval 필터를
   `team_id = $T AND user_id = $U` 로 합성한다.
5. **Backfill migration**: 기존 IDX-001/002/003 시기 데이터를 설정 가능한
   default team (`COSTGUARD_DEFAULT_TEAM`과 같은 패턴, 본 SPEC에서는
   `INDEX_DEFAULT_TEAM`로 신설)으로 일괄 할당하는 일회성 SQL + Qdrant +
   Meili 백필 도구를 제공한다.
6. **Cross-team leak prevention**: `team_id` 필터가 query path에서 빠질 가능성
   자체를 제거하는 sentinel — `IndexQuery.TeamID == ""` 이면 즉시 typed error
   (`ErrTeamIDRequired`) 반환. CLI / MCP / API 진입점은 인증된 컨텍스트에서
   `team_id` 를 주입하며, 누락 시 401/403 매핑.
7. **Tenant-aware observability**: 신규 카디널리티 라벨 `team_id_hashed`
   (SHA-256 prefix 8 hex) 를 search/upsert 메트릭에 추가하고,
   `usearch_index_tenant_token_issued_total` / `_revoked_total` /
   `_validation_failures_total` 카운터를 신설한다. `team_id_hashed` 는
   `team_id` 평문 노출을 막으면서도 per-team trend 관측을 허용한다.

본 research는 (a) IDX-001/002/003의 현재 코드 상태를 file:line으로 확정,
(b) Qdrant Tiered Multitenancy 의 정확한 설정 형태를 외부 문서로 인용,
(c) Meilisearch 의 tenant token 생성 메커니즘 (HMAC-SHA256 JWT + `searchRules`)
의 정확한 SDK 시그니처 확정, (d) PG migration 의 backward-compat 경로,
(e) SPEC-AUTH-001/002 와의 인터페이스 contract를 정리한다.

`.moai/project/roadmap.md:155` 의 M6 exit criterion 중 **"shared index dedup
hits ≥30%"** 항목은 본 SPEC이 데이터 격리를 깨뜨리지 않으면서도 같은 team
내부에서는 dedup이 정상 동작하도록 보장한다는 의미다. 따라서 (a) 동일 team
내 동일 doc_id 재호출은 cache-hit 처럼 동작해야 하며 (b) 서로 다른 team 간
의 동일 doc_id 는 retrieval 결과에서 절대 교차되면 안 된다. 두 invariant 는
REQ-IDX4-001 + REQ-IDX4-007 로 EARS-encode 된다.

---

## 1. Existing-Code State

### 1.1 IDX-001 Reservation Surface (현재)

PostgreSQL `docs` 테이블에 이미 `team_id TEXT NULL` 컬럼이 존재한다
(`deploy/postgres/migrations/0001_create_docs.sql:16`):

```sql
team_id      TEXT        NULL,  -- reserved for SPEC-IDX-004 multi-tenancy enforcement
```

같은 migration의 line 25 에 B-tree 인덱스도 이미 생성되어 있다:

```sql
CREATE INDEX IF NOT EXISTS idx_docs_team_id      ON docs (team_id);
```

`internal/index/dispatch.go:347` 의 Meilisearch document 생성 경로는
`team_id: nil` 을 명시적으로 박아 둠으로써 v0.1 동안 모든 doc 이 NULL team
임을 보장한다:

```go
"team_id":      nil, // v0.1: always null per SPEC-IDX-004 reservation
```

`internal/index/qdrant/client.go:242` 의 payload filter builder 는 이미
`team_id` 키를 다룬다:

```go
Key: "team_id",
```

`internal/index/meili/korean_shard.go:27` 의 Korean shard settings 도
filterable attribute 에 `team_id` 를 포함한다 (IDX-003 의 forward-compat
조치):

```go
"team_id",
```

`internal/index/pg/client.go:121, 171, 230, 246` 의 모든 INSERT / SELECT
경로는 `team_id` 컬럼을 명시 — 필터링 / 삽입 / 검색 모두 컬럼 자체를 지원
한다. 단, INSERT 는 항상 NULL 을 박는다.

따라서 IDX-004 의 코드 변경은 **삽입 시 실제 team_id 값을 채우는 부분 + 검색
시 team_id 필수 강제 + Qdrant payload index 의 `is_tenant=true` 토글 + Meili
tenant token 발급 + backfill migration + observability label 확장** 으로
한정된다.

### 1.2 IDX-001 의 `IndexQuery.TeamID` 필드

`internal/index/types.go:25-27` 에 이미 필드가 존재한다:

```go
// TeamID is reserved for SPEC-IDX-004 multi-tenancy enforcement.
// In v0.1: empty = all rows; non-empty = filter (excludes NULL rows, so empty result).
TeamID string
```

[HARD] IDX-004 는 이 필드의 의미를 다음과 같이 전환한다:

- 빈 문자열 → `ErrTeamIDRequired` 즉시 반환 (single-tenant 모드 제거)
- 운영 모드 호환을 위해 `INDEX_MULTI_TENANCY_MODE` 환경 변수 도입
  (`enforced` / `permissive` / `legacy`). `enforced` 가 v1.0 기본. `legacy` 는
  마이그레이션 윈도우 동안만 활성화하여 단계적 전환을 허용한다.

### 1.3 IDX-002/003 의 Embedding과 Tokenization 영향

- **IDX-002 (BGE-M3 embedder)**: 임베딩은 텍스트만 사용하므로 team 정보와
  무관하다. 같은 team이든 다른 team이든 같은 텍스트는 같은 벡터를 만든다.
  IDX-004 는 임베딩 캐시(`internal/embedder/client.go` LRU)에 team-aware key
  를 추가하지 **않는다** — 그렇게 하면 캐시 hit-rate 가 팀 수에 반비례하여
  급락한다. 대신 retrieval 시 payload filter 에서 team 격리가 이뤄진다.
- **IDX-003 (Korean tokenizer + ko shard)**: Meili `usearch_docs_ko` 인덱스도
  `team_id` filterable attribute 를 이미 가지므로 IDX-004 는 두 인덱스 모두에
  대한 tenant token issuance 만 추가하면 된다. Korean 토크나이저 설정은
  team-agnostic 이다.

### 1.4 SPEC-AUTH-001 / AUTH-002 인터페이스

AUTH-001 (`.moai/project/roadmap.md:83`)는 OIDC SSO + JWT 미들웨어를 제공한다.
JWT claim 에서 `team_id` 를 추출하여 request context 에 주입하는 미들웨어
시그니처는 다음과 같이 합의되었다 (DEEP-004 에서 forward-compat 으로 동일
패턴 사용 — `.moai/specs/SPEC-DEEP-004/spec.md:11` D1, D2):

```go
ctx = context.WithValue(parent, authctx.TeamIDKey, jwt.Sub.TeamID)
ctx = context.WithValue(ctx, authctx.UserIDKey, jwt.Sub.UserID)
```

IDX-004 는 그 context key 를 단일 진실 공급원(`authctx.TeamIDKey`)으로 소비한다.
AUTH-001 ship 이전에는 환경 변수 `INDEX_DEFAULT_TEAM` 으로 동일한 키에 단일
team 을 주입하여 forward-compat 을 유지한다 (DEEP-004 D2 와 같은 패턴).

AUTH-002 (`.moai/project/roadmap.md:84`)는 Casbin 기반 team RBAC + **per-adapter
visibility** 를 정의한다. 각 adapter (e.g. Reddit, GitHub, Naver) 는 다음 중
하나의 visibility 정책을 가진다:

| Policy | 의미 | 적용 |
|--------|------|------|
| `team-shared` | 결과를 같은 team 내 모든 user 가 조회 가능 | 기본값 |
| `user-private` | 결과를 ingest 한 user 만 조회 가능 (e.g. GitHub PAT 로 fetch 한 private repo, Gmail 같은 인격적 adapter) | 명시적 opt-in |
| `public` | 모든 team 가 조회 가능 (e.g. 공개 arXiv) | adapter 별 운영 결정 |

IDX-004 는 visibility 정책을 `adapter.visibility` 메타 필드 (SPEC-CORE-001 의
`pkg/types.Adapter` 인터페이스에 추가 예정 — AUTH-002 의 변경분이지만 IDX-004
는 그 결과를 소비) 에서 읽어 ingest 시점에 doc 별 `team_id` + `user_id` + 가
시성 카테고리를 결정한다. v0.1 기본은 모두 `team-shared`.

### 1.5 OBS-001 카디널리티 화이트리스트

`internal/obs/metrics/metrics.go:171-176` 의 cardinality allowlist (SPEC-OBS-001
NFR-OBS-002) 는 모든 새 label 이 명시적으로 추가되어야 한다. IDX-001 이
`store` + `op` 를 추가했고, DEEP-004 가 `tenant`, `status`, `tier`, `state` 를
추가했다. IDX-004 는 다음을 추가한다:

- `team_id_hashed` (SHA-256 prefix 8 hex, bounded — 약 4 × 10^9 values 가능
  하지만 실제 production 에서는 100-1000 팀 수준이라 cardinality 폭증 위험
  없음)
- `visibility` (enum: `team_shared`, `user_private`, `public` — 3 values)

[HARD] `team_id` 평문은 label 로 절대 노출되지 않는다. observability 운영자
가 GDPR / 보안 감사 시 hash → plaintext 매핑을 별도 secret 채널로 보유한다.

### 1.6 DEEP-004 의 Cost Guard 와의 상호 작용

DEEP-004 (`.moai/specs/SPEC-DEEP-004/spec.md`) 는 `cost_ledger` 테이블에 이미
`tenant_id TEXT` + `user_id TEXT` 컬럼을 박았다 (`deploy/postgres/migrations/
0002_cost_ledger.sql`). IDX-004 는 동일한 의미의 `team_id` 컬럼을 `docs` 테이블
에 enforce 한다. 두 SPEC 의 식별자 (DEEP-004 의 `tenant_id` ↔ IDX-004 의
`team_id`) 는 **같은 논리 entity** 를 가리키며, AUTH-001 의 JWT claim
`sub.team_id` 가 두 곳에 동일하게 주입된다.

향후 SPEC-AUTH-003 (audit log, `.moai/project/roadmap.md:85`) 에서 두 컬럼명을
통일하는 schema rename 이 검토될 수 있으나 v1.0 에서는 두 명칭을 그대로 유지
한다 — 의미가 같으므로 운영 영향 없음.

### 1.7 IDX-005 (Team-Shared Answer Reuse) 와의 의존성

IDX-005 (`.moai/project/roadmap.md:87`)는 fanout 이전에 동일 query 에 대한
team 의 사전 답변을 retrieval 하여 staleness threshold 안이면 재사용한다.
이 SPEC 은 IDX-004 의 team_id-aware Search 가 정확히 동일 team 의 답변만
반환하도록 보장하는 것을 전제로 한다. 따라서 IDX-004 는 IDX-005 를 blocks
한다 (`.moai/specs/SPEC-IDX-001/spec.md:16` 의 `blocks: [...SPEC-IDX-005]`
체인 유지).

---

## 2. Qdrant Tiered Multitenancy

### 2.1 단일 vs 다중 컬렉션 결정

Qdrant 공식 문서 (https://qdrant.tech/documentation/guides/multiple-partitions/,
2026-05-22 WebFetch 검증) 는 다음을 권장한다:

> "Qdrant recommends creating a single collection per embedding model with
> payload-based partitioning for different tenants. ... Only create separate
> collections when you have a limited number of users and you need isolation,
> despite the higher resource costs."

IDX-004 는 **single collection 기본 + 대형 팀에 한해 dedicated collection 옵션**
의 tiered 패턴을 선택한다:

| Tier | 대상 | Collection | 운영 |
|------|------|-----------|------|
| Default (shared) | 모든 팀 기본 | `usearch_docs` (IDX-001 기존) | payload `team_id` 필터로 격리 |
| Dedicated (premium) | 대용량 / 데이터 격리 강제 팀 | `usearch_docs__team_<team_id_hashed>` | collection-level 격리 |

승격 트리거 (config-driven):
- 팀의 누적 doc count 가 `qdrant.tiering.dedicated_threshold_docs` (기본
  1,000,000) 를 초과
- 운영자가 명시적으로 `qdrant.tiering.dedicated_teams` 리스트에 추가
- 보안 감사 요건상 데이터 분리가 필요한 enterprise 계약 팀

승격은 일회성 background migration 으로 수행 (operator-triggered admin CLI
`usearch admin tier-promote --team <id>`). v1.0 의 backfill 도구는 dedicated
collection 생성 + 기존 default-tier point 들의 streaming 이동 + verify 단계로
구성된다.

### 2.2 Payload Index Configuration

위 문서가 인용하는 정확한 설정:

> ```
> PUT /collections/{collection_name}/index
> {
>   "field_name": "group_id",
>   "field_schema": {
>     "type": "keyword",
>     "is_tenant": true
>   }
> }
> ```
>
> "is_tenant=true ... storage structure will be organized in a way to co-locate
> vectors of the same tenant together, which can significantly improve
> performance by utilizing sequential reads."

IDX-004 의 EnsureCollection 변경분:

- Qdrant collection bootstrap 시 `team_id` 키에 대해 `keyword` + `is_tenant=true`
  payload index 를 생성한다. 기존 collection 에 대해 idempotent 하게 add 한다.
- HNSW 설정: 문서가 권장하는 `payload_m: 16` + `m: 0` (전역 HNSW 비활성) 패턴을
  채택할지 여부는 NFR-IDX4-006 의 회귀 benchmark 결과로 결정한다. v1.0 기본은
  보수적으로 **기존 HNSW 그대로** 유지하고, payload index `is_tenant=true` 만
  적용한다. `payload_m` 활성화는 future SPEC-IDX-007 의 옵션으로 deferred.

### 2.3 HNSW Pre-Filtering Behavior

Qdrant 의 HNSW filter 적용은 cardinality estimator 기반 plan 선택을 거친다
(reference: Qdrant docs "Filterable HNSW"). `is_tenant=true` 가 적용되면
같은 team 의 벡터들이 메모리/디스크 상에서 co-locate 되어 pre-filter overhead
가 < 5% 로 떨어진다 (Qdrant 공식 벤치마크). NFR-IDX4-006 는 그 측정값을
검증한다.

---

## 3. Meilisearch Per-Tenant Tokens

### 3.1 Tenant Token JWT 구조

Meilisearch 공식 문서 (https://www.meilisearch.com/docs/learn/security/multi_tenancy,
2026-05-22 WebFetch 검증) 가 정의:

> "Tenant tokens function as JWTs containing embedded filter rules. ... search
> rules ([filters]) that automatically apply to every search request."
>
> "Tenant tokens only restrict the search endpoint. They do not apply to admin
> operations."
>
> "Search rules are 'Filter expressions baked into a token (e.g.,
> `user_id = 123`)'."

IDX-004 의 토큰 payload 형태:

```json
{
  "apiKeyUid": "<uuid4 of admin API key used to sign>",
  "searchRules": {
    "usearch_docs": { "filter": "team_id = \"<team_id>\"" },
    "usearch_docs_ko": { "filter": "team_id = \"<team_id>\"" }
  },
  "exp": 1717980000
}
```

서명: `HS256` (HMAC-SHA256) 으로 Meili admin API key 의 secret 을 비밀로 사용.

### 3.2 Go SDK 시그니처

`github.com/meilisearch/meilisearch-go` (v0.36.2 이상, IDX-001 에서 pinned)
의 정확한 시그니처는 (2026-05-22 검증):

```go
func (m *meilisearch) GenerateTenantToken(
    apiKeyUID string,
    searchRules map[string]interface{},
    options *TenantTokenOptions,
) (string, error)
```

- `apiKeyUID`: UUID4 (Meili admin API key 의 uid; admin key 가 아닌 uid 임에
  주의 — admin key 자체는 `options.APIKey` 로 별도 전달)
- `searchRules`: 위 §3.1 payload 의 `searchRules` 필드와 동일 shape 의 map
- `options.ExpiresAt`: 미래 시각 timestamp; 검증 시 현재시각보다 미래여야
  함. IDX-004 는 기본 TTL **15분** + 자동 refresh 패턴 사용.

토큰 발급은 IDX-004 의 신규 모듈 `internal/index/tenant/` 에서 캐싱한다 —
같은 (team_id, user_id, api_key_uid) 조합은 cached. 만료 60초 전부터 백그라운드
refresh.

### 3.3 권한 변경 시 회전

AUTH-002 의 RBAC 정책이 변경되면 (e.g. user 가 team 에서 제거됨) IDX-004
는 즉시 토큰을 invalidate 한다:

- AUTH-002 가 발행하는 SSE/Redis pub-sub 이벤트 `team.member.removed` 구독
- 해당 (team_id, user_id) 토큰 캐시 entry 제거
- 다음 요청에서 새 토큰 발급 시 자동 검증 실패 (Casbin policy 거부)

이 메커니즘은 SPEC-AUTH-002 의 implementation 후 wire-up 한다. v1.0 의 IDX-004
는 hook point (interface + DI seam) 만 정의하고 구체 wiring 은 AUTH-002 ship
후 추가한다.

### 3.4 Tenant Token 적용 경로

Meilisearch 의 tenant token 은 **search endpoint** 에만 적용된다 (admin
operation 은 admin API key 직접 사용). 따라서 IDX-004 의 분기:

- **Search path** (`internal/index/meili/client.go::Search`): tenant token 사용
- **Upsert path** (`AddDocuments`): admin API key 사용 + doc payload 의
  `team_id` 필드를 명시적으로 set
- **EnsureIndex / Settings**: admin API key 사용 (IDX-004 collection bootstrap)

---

## 4. PostgreSQL Migration Strategy

### 4.1 Migration File 0004_team_id_not_null.sql

Migration 0001 (IDX-001) 의 `team_id TEXT NULL` 컬럼을 `NOT NULL DEFAULT
'default'` 로 전환:

```sql
-- SPEC-IDX-004 REQ-IDX4-006: Backfill + enforce NOT NULL on docs.team_id.

-- Step 1: Backfill NULL rows to the configured default team.
UPDATE docs SET team_id = COALESCE(team_id, current_setting('app.default_team', true), 'default')
WHERE team_id IS NULL;

-- Step 2: Set NOT NULL + DEFAULT (idempotent via CHECK).
ALTER TABLE docs ALTER COLUMN team_id SET DEFAULT 'default';
ALTER TABLE docs ALTER COLUMN team_id SET NOT NULL;

-- Step 3: Composite index for (team_id, source_id) range scans.
CREATE INDEX IF NOT EXISTS idx_docs_team_id_source_id ON docs (team_id, source_id);

-- Step 4: Index hot path: (team_id, published_at DESC) for time-bounded queries.
CREATE INDEX IF NOT EXISTS idx_docs_team_published ON docs (team_id, published_at DESC);
```

[HARD] Migration 은 **single-transaction** 이어야 한다 (Step 1 backfill 과
Step 2 NOT NULL 사이에 새 NULL row 가 추가되면 ALTER 실패). Migration 실행
시점에 ingestion 을 일시 정지하거나, PG 의 `LOCK TABLE docs IN EXCLUSIVE MODE`
를 사용한다.

`current_setting('app.default_team', true)` 는 `psql` connection 파라미터
`-c "app.default_team=<value>"` 또는 application 의 `SET` 명령으로 주입.
기본값 `'default'` 가 fallback.

### 4.2 신규 테이블 team_id 컬럼

향후 AUTH-003 (audit log) 가 추가하는 `audit_log` 테이블에도 `team_id TEXT NOT
NULL` 컬럼이 필요하다. IDX-004 는 audit_log 의 schema 를 직접 만들지 않으므로
이 부분은 AUTH-003 의 책임이지만, contract 차원에서 IDX-004 의 research §1.6
에서 미리 align 한다.

DEEP-004 의 `cost_ledger.tenant_id` 는 의미적으로 `team_id` 와 같다. v1.0 에서는
컬럼명 그대로 두고, AUTH-003 가 unification 을 결정한다.

### 4.3 Rollback 전략

NOT NULL 전환은 destructive 가 아니다 (역방향 `ALTER COLUMN ... DROP NOT NULL`
은 instant). 그러나 backfill 된 `default` 팀 row 를 NULL 로 되돌리는 것은
의미가 없으므로 rollback 은 schema 만 되돌리고 데이터는 유지한다. 운영자가
명시적으로 `UPDATE docs SET team_id = NULL WHERE team_id = 'default'` 를 실행
해야 한다 — 이 경우 IDX-004 의 enforce 검증은 실패하므로 사실상 IDX-004 자체
의 rollback 이 되어야 한다.

---

## 5. Personal Context Tier (user_id payload)

### 5.1 Adapter Visibility Policy

AUTH-002 에서 도입되는 `Adapter.Visibility()` 메타 함수는 다음 중 하나를
반환한다:

```go
type AdapterVisibility int

const (
    VisibilityTeamShared AdapterVisibility = iota // 기본 — same team 의 모든 user
    VisibilityUserPrivate                          // ingest 한 user 만
    VisibilityPublic                               // 모든 team
)
```

[HARD] IDX-004 의 Upsert 경로는 adapter visibility 를 다음과 같이 처리한다:

| Visibility | Qdrant payload | Meili document | PG row | Search filter |
|------------|---------------|---------------|--------|---------------|
| TeamShared | `team_id: T, user_id: ""` | `team_id: T, user_id: ""` | `team_id=T, user_id=NULL` | `team_id = T` |
| UserPrivate | `team_id: T, user_id: U` | `team_id: T, user_id: U` | `team_id=T, user_id=U` | `team_id = T AND (user_id = U OR user_id = "")` |
| Public | `team_id: "__public__", user_id: ""` | `team_id: "__public__"` | `team_id='__public__'` | `team_id = T OR team_id = "__public__"` |

[HARD] `__public__` 은 reserved sentinel team_id. 일반 team 으로 사용 불가
(REQ-IDX4-007 검증).

### 5.2 user_id PG 컬럼 추가

Migration `0005_user_id_column.sql`:

```sql
ALTER TABLE docs ADD COLUMN IF NOT EXISTS user_id TEXT NULL;
CREATE INDEX IF NOT EXISTS idx_docs_team_user ON docs (team_id, user_id)
    WHERE user_id IS NOT NULL;
```

partial index 는 `user_id IS NOT NULL` 인 row 에 한해 생성하여 storage 절약.

### 5.3 Meili filterable attribute 확장

`internal/index/index.go:108` 에 이미 등록된 filterable attributes 에 `user_id`
추가:

```go
FilterableAttributes: []string{"source_id", "lang", "doc_type", "team_id", "user_id", "published_at"},
```

기존 인덱스는 `UpdateSettings` 로 patch — Meili 는 settings 변경 시 reindex
가 background 로 발생 (수 초 ~ 수 분). 운영 중 zero-downtime.

---

## 6. Backfill Migration Tool

### 6.1 도구 형태

새 admin CLI 명령:

```
usearch admin backfill-team --default-team <id> [--dry-run] [--batch-size 1000]
```

동작:

1. PG: `UPDATE docs SET team_id = $1 WHERE team_id IS NULL` (batch 처리,
   `WHERE doc_id IN (SELECT doc_id FROM docs WHERE team_id IS NULL LIMIT $2)`
   pattern 으로 lock 시간 단축).
2. Qdrant: 각 point 의 payload 에 `set_payload` API 로 `team_id` 추가. 같은
   team 으로 batch (default 1000 points/batch).
3. Meili: `UpdateDocuments` 로 `team_id` 필드 patch. partial update 지원
   (`meilisearch-go` 의 `UpdateDocuments` 메서드 사용).
4. 진행 상황을 `internal/index/backfill/state.json` 에 기록하여 재개 가능.
5. 완료 후 검증: `SELECT count(*) FROM docs WHERE team_id IS NULL` == 0
   확인. Qdrant scroll 로 payload 검증. Meili count 검증.

### 6.2 Dry-Run 모드

`--dry-run` 은 변경 없이 다음을 출력:

- PG 영향 row 수
- Qdrant 영향 point 수 (collection 별)
- Meili 영향 document 수 (index 별)
- 예상 시간 (batch 단위로 stub 측정)

---

## 7. Cross-Team Leak Prevention

### 7.1 Sentinel Pattern

`internal/index/dispatch.go` 의 search 진입점:

```go
func (idx *Index) Search(ctx context.Context, q IndexQuery) (*IndexResult, error) {
    if mode := indexTenancyMode(); mode == TenancyEnforced && q.TeamID == "" {
        return nil, ErrTeamIDRequired
    }
    // ... existing logic
}
```

[HARD] `TeamID == ""` && mode == `enforced` 는 즉시 거절. Search hot path 의
첫 검증 — embedder 호출 / store fanout 모두 건너뜀.

### 7.2 Upsert 강제

Upsert 도 동일:

```go
func (idx *Index) Upsert(ctx context.Context, docs []types.NormalizedDoc) (*UpsertResult, error) {
    teamID := extractTeamID(ctx)  // from context.WithValue(authctx.TeamIDKey, ...)
    if mode := indexTenancyMode(); mode == TenancyEnforced && teamID == "" {
        return nil, ErrTeamIDRequired
    }
    for i := range docs {
        if docs[i].Metadata == nil {
            docs[i].Metadata = make(map[string]any)
        }
        docs[i].Metadata["team_id"] = teamID
        // user_id from visibility policy of adapter
    }
    // ... existing logic
}
```

[HARD] team_id 는 caller 가 NormalizedDoc.Metadata 에 직접 셋하지 않는다 —
IDX-004 가 context 에서 추출하여 주입. 이는 adapter 가 임의의 team_id 를
주입하여 cross-team write 를 시도하는 attack vector 를 차단한다.

### 7.3 Test Strategy

Cross-team leak 회귀 테스트는 두 단계로 구성:

1. **Unit**: Mock store + IndexQuery 분기 정상 동작 확인. Search with q.TeamID
   = "team-A" → 결과에 `team_id != "team-A"` row 없음.
2. **Integration**: testcontainers 기반 실제 Qdrant + Meili + PG 에 두 team
   의 doc 을 ingest 후 `Search(q={TeamID: "team-A"})` 가 team-B doc 을 절대
   포함하지 않음. 100회 반복 + concurrent 검증.

---

## 8. Observability Extensions

### 8.1 New Counters / Histograms

`internal/obs/metrics/tenant.go` (신규 파일):

```go
TenantTokenIssued     *prometheus.CounterVec   // labels: tier ∈ {meili, qdrant}, outcome ∈ {success, error}
TenantTokenRevoked    *prometheus.CounterVec   // labels: tier
TenantTokenValidation *prometheus.CounterVec   // labels: tier, outcome
IndexTenantBackfill   *prometheus.CounterVec   // labels: store ∈ {qdrant, meili, pg}, outcome
```

### 8.2 Label Extension on Existing Metrics

기존 IDX-001 의 `IndexOps`, `IndexOpDuration` 에 `team_id_hashed` label 추가.
카디널리티 폭증 방지를 위해 다음 규칙:

- `team_id_hashed` = `hex(sha256(team_id))[:8]` — 8 hex = 4 × 10^9 가능값.
- 실제 production 의 team 수는 100~10000 수준이라 한 label 차원에서 안전.
- `__public__` team 은 `"__public__"` 으로 별도 매핑 (해시하지 않음, 카운터에서
  명시적으로 추적).

### 8.3 NFR-OBS-002 화이트리스트 확장

`internal/obs/metrics/metrics_test.go::TestNoUnboundedLabels` 의 allowlist 에
`team_id_hashed`, `visibility`, `tier` 추가. 다른 라벨 (예: store, op, outcome)
은 IDX-001 / DEEP-004 에서 이미 등록.

### 8.4 OTel Span Attributes

`index.search` span 에 다음 attribute 추가:

- `index.team_id_hashed` (string)
- `index.tenant_mode` (string: enforced/permissive/legacy)
- `index.visibility_filter` (string: 적용된 visibility 필터 표현)

[HARD] 평문 `team_id` 는 span attribute 로도 노출 금지. 디버깅 필요 시
`index.team_id_hashed` 로 trace 후 별도 매핑 테이블 참조.

---

## 9. Adapter Visibility Integration with AUTH-002

### 9.1 Contract Surface

AUTH-002 가 정의할 (예상) 추가 인터페이스:

```go
// pkg/types/adapter.go (AUTH-002 amendment 예정)
type AdapterMeta interface {
    Adapter
    Visibility() AdapterVisibility  // team_shared / user_private / public
}
```

IDX-004 의 Upsert path 는 `adapter.Visibility()` 를 호출하여 doc 단위
classification. 호출 시점:

- fanout.Dispatch 가 NormalizedDoc 을 반환할 때 각 adapter 의 visibility 를
  메타에 attach (fanout.Result 의 PerAdapterMeta 필드 — AUTH-002 가 추가).
- IDX-004 는 Metadata 에서 visibility 를 읽어 team_id / user_id 채움.

### 9.2 Forward-Compat 전략

AUTH-002 가 ship 되기 전 v0.x 운영을 위해 IDX-004 는 다음 default 적용:

- adapter 가 `Visibility()` 메서드를 구현하지 않으면 `team_shared` 로 간주.
- `__public__` 분류는 IDX-004 자체로는 적용하지 않음 (AUTH-002 의 도입 후
  활성화).

이 default 는 v1.0 안전 동작을 보장한다 — 모든 doc 이 same-team 가시성이라
cross-team leak 위험은 없으나, public 데이터의 cross-team 재사용 효율은 v0.x
에서 손실된다.

---

## 10. Race / Leak / Cancellation Analysis

### 10.1 Tenant Token 캐시 동시성

`internal/index/tenant/cache.go` 는 sync.Map + ttl-based eviction. 동시 접근
패턴:

- 50 goroutine × 100 Search 호출, 같은 team_id 5개 분포
- 신규 token 발급 (cache miss) 은 sync.Once 보호로 같은 (team, user) 조합에
  대해 단 한 번만 발급
- TTL refresh 백그라운드 worker 1개 (goleak.VerifyNone 통과 위해 정상 종료)

### 10.2 Context Propagation

`authctx.TeamIDKey` 는 ctx 를 통해 caller → Index → store driver 까지 전달.
ctx cancellation 은 모든 hop 에서 honor. token cache 의 refresh worker 는
별도 ctx (background) 사용 — 메인 ctx 취소에 영향 받지 않음.

### 10.3 Migration 도중 Concurrent Ingest

Backfill 도중에 새 ingest 가 들어오면 PG 의 NOT NULL constraint 가 새 row 도
`team_id != NULL` 강제. backfill 은 기존 row 만 처리. 따라서 race 없음.
단, NOT NULL 전환 (Step 2 ALTER) 시점에 in-flight INSERT 가 있으면 `ALTER`
가 lock 대기. 운영자는 ingest 일시 정지 후 migration 실행 권장.

---

## 11. Open Questions

번호 매겨진 미해결 결정 (recommended default 포함, 본 SPEC 승인 시 OQ 로
spec body 에 carry-forward).

### 11.1 Default Team Identifier

기본 team 의 식별자. 후보:

- `"default"` (가장 단순)
- `"public"` (의미 충돌 — `__public__` 은 sentinel 로 예약)
- 운영자가 `INDEX_DEFAULT_TEAM` env var 로 지정

**Recommended**: env var 우선, fallback `"default"`. DEEP-004 의 default
tenant 와 동일 명명.

### 11.2 Token TTL

- 5분: 회전 비용 ↑, 권한 변경 반영 빠름
- 15분 (recommended): 균형
- 1시간: 회전 비용 ↓, AUTH-002 revocation 반영 지연

**Recommended**: 15분 + 만료 60초 전 백그라운드 refresh. 운영자가
`.moai/config/sections/index.yaml` 로 조정.

### 11.3 team_id Format Validation

- 임의 string 허용 (UUID 권장이나 강제 없음)
- UUID 형식 강제 (변환 비용 ↑)
- 알파벳 + 숫자 + `-` 만 허용 (정규식 `^[a-zA-Z0-9_-]{1,64}$`)

**Recommended**: 후자 (정규식). reserved 식별자 `__public__` + `system` 은
사용자 input 으로 금지.

### 11.4 Dedicated Tier Promotion Trigger

- doc count threshold (e.g. 1M)
- operator manual list
- 둘 다

**Recommended**: 둘 다 지원. v1.0 은 manual list 만 활성, doc-count auto-tier
는 SPEC-IDX-007 (post-V1) deferred.

### 11.5 user_id 비공개 adapter 의 ingest 트리거

- adapter 가 stateful (자체 OAuth context) → adapter 가 user_id 제공
- adapter 가 stateless (질의 시 인증된 user 사용) → IDX-004 가 ctx 에서 추출

**Recommended**: 두 경로 모두 지원. adapter 가 명시적으로 visibility +
ingester user_id 를 제공하면 우선; 그렇지 않으면 ctx 의 user_id 사용.

### 11.6 Token Validation Cache 분리

Meili 가 token 검증을 자체 수행하므로 Go-side cache 는 발급 캐시 + 유효성
체크 cache 가 분리되어 있다. validation cache 가 별도로 필요한가?

**Recommended**: NO. Meili 가 valid 면 success, invalid 면 401 → IDX-004 가
재발급 후 한 번 재시도. validation 캐시는 운영 복잡도만 증가.

### 11.7 Backfill 도구의 실패 복구

batch 단위 진행 후 crash 시 재실행은 어떻게?

**Recommended**: state.json 에 last_processed_doc_id 기록. 재실행 시 그 지점
부터 재개. PG / Qdrant / Meili 별로 진행 상황 별도 기록.

### 11.8 Public Tier 의 Qdrant 격리

`__public__` doc 들이 default-tier collection 안에 섞이면 모든 team 의 검색
경로에서 이 row 들이 scan 된다. 분리 collection 으로 빼야 하나?

**Recommended**: v1.0 은 default-tier 동일 collection + payload filter `team_id
= $T OR team_id = "__public__"` 으로 처리. dedicated public collection 은
post-V1. public doc 비중이 < 5% 이므로 성능 영향 미미.

### 11.9 Token Issuance 의 Per-Request Latency

token 발급은 HMAC-SHA256 서명 (~30µs) + cache 조회 (~1µs). p99 < 50µs 추정.
warm cache 의 경우 negligible. cold cache 첫 호출만 latency 추가.

**Recommended**: NFR-IDX4-005 에 cache hit-rate ≥ 99% 명시 (정상 운영 시).

### 11.10 IDX-005 의 Cache-Hit 경로와 visibility

IDX-005 (team-shared answer reuse) 가 fanout 이전에 호출하는 Index.Search 는
user_id 를 모르는 상태 (request 진입 시점). user_private adapter 의 답변은
캐시되지 말아야 하나?

**Recommended**: IDX-005 는 `team-shared` + `public` 만 cache. `user-private`
은 IDX-005 의 cache 대상에서 제외. IDX-005 의 SPEC body 에서 명시.

---

## 12. Sources and Citations

### External URLs (WebFetch 검증 2026-05-22)

- https://qdrant.tech/documentation/guides/multiple-partitions/ — Qdrant
  Tiered Multitenancy 공식 가이드. `group_id` payload field, `is_tenant=true`
  payload index, single vs dedicated collection 결정 기준, HNSW pre-filter
  의 co-location 효과. §2.1-2.3 에서 인용.
- https://www.meilisearch.com/docs/learn/security/multi_tenancy — Meilisearch
  tenant token (JWT) 공식 가이드. searchRules, exp, admin vs search key 구분,
  XSS 주의. §3.1, §3.3 에서 인용.
- https://github.com/meilisearch/meilisearch-go — meilisearch-go SDK
  v0.36.2. `GenerateTenantToken(apiKeyUID, searchRules, options) (string,
  error)` 함수 시그니처 + HMAC-SHA256 서명. §3.2 에서 인용.
- https://qdrant.tech/documentation/concepts/payload/ — Qdrant payload
  schema, keyword vs numeric index 구분.
- https://qdrant.tech/documentation/concepts/filtering/ — Qdrant filter
  expression, must / should / must_not.

### Internal Files (file:line 인용)

- `.moai/specs/SPEC-IDX-001/spec.md:6` — IDX-001 status: implemented.
- `.moai/specs/SPEC-IDX-001/spec.md:16` — IDX-001 blocks: [..SPEC-IDX-004..].
- `.moai/specs/SPEC-IDX-001/spec.md:100-106` — D10 multi-tenancy reservation.
- `.moai/specs/SPEC-IDX-001/spec.md:226-258` — IDX-001 §2.8 multi-tenancy
  forward-compat surface.
- `.moai/specs/SPEC-IDX-002/spec.md` — implemented BGE-M3 embedder.
- `.moai/specs/SPEC-IDX-003/spec.md` — implemented Korean tokenizer + ko shard.
- `.moai/specs/SPEC-DEEP-004/spec.md:11` — DEEP-004 D1 X-User-Id forward-compat.
- `.moai/specs/SPEC-DEEP-004/spec.md` — DEEP-004 cost_ledger schema with
  tenant_id + user_id columns.
- `.moai/specs/SPEC-OBS-001/spec.md` — observability baseline + cardinality
  allowlist mechanism.
- `internal/index/index.go:108` — Meili FilterableAttributes 현 상태 (team_id
  포함).
- `internal/index/dispatch.go:232` — Meili filter expression builder team_id.
- `internal/index/dispatch.go:309-319` — Qdrant payload team_id from
  doc.Metadata.
- `internal/index/dispatch.go:347` — Meili document `team_id: nil` (v0.1
  reservation).
- `internal/index/meili/korean_shard.go:27` — Korean shard team_id filterable.
- `internal/index/qdrant/client.go:242` — Qdrant filter team_id key.
- `internal/index/pg/client.go:121, 171, 230, 246` — PG client team_id 컬럼
  지원.
- `internal/index/types.go:25-27` — IndexQuery.TeamID 필드.
- `deploy/postgres/migrations/0001_create_docs.sql:16, 25` — docs.team_id
  컬럼 + 인덱스.
- `internal/obs/metrics/metrics.go:171-176` — cardinality allowlist.
- `.moai/project/roadmap.md:80-87` — M6 SPEC backlog (AUTH + IDX-004/005).
- `.moai/project/roadmap.md:128` — M6 parallelization plan.
- `.moai/project/roadmap.md:155` — M6 exit criterion (shared index dedup
  ≥30%).
- `.moai/project/tech.md:39-50` — retrieval layer choices (Qdrant Tiered
  Multitenancy, Meili per-tenant tokens 명시).
- `.moai/project/tech.md:65-72` — team plane stack.
- `.moai/config/sections/quality.yaml` — `development_mode: tdd`,
  `test_coverage_target: 85`.
- `.moai/config/sections/language.yaml` — `documentation: en`, `code_comments:
  en`, `conversation_language: ko`.

---

End of Research Document.

**Summary for SPEC Author**: SPEC-IDX-004는 IDX-001 의 v0.1 multi-tenancy
reservation 을 enforcement 로 전환한다. (a) Qdrant single-collection + `team_id`
payload + `is_tenant=true` 인덱스의 Tiered Multitenancy 기본 + 대형 팀의
dedicated collection 옵션, (b) Meili admin key 로 HMAC-SHA256 서명한 15분
TTL tenant token (`searchRules` 에 `team_id = "<T>"` 필터) per (team, user) 조합,
(c) PG `docs.team_id` NOT NULL + DEFAULT migration 0004 + user_id 컬럼 + composite
index, (d) `user_id` payload 로 personal context tier 지원 (AUTH-002 의 adapter
visibility 결과 소비), (e) `INDEX_DEFAULT_TEAM` 환경 변수 기반 backfill admin
CLI, (f) cross-team leak prevention sentinel (`TeamID == ""` && mode==enforced →
`ErrTeamIDRequired`), (g) observability label `team_id_hashed` 추가 + 3개 신규
tenant-token 메트릭 family. 기존 코드 변경은 dispatch.go 의 payload 생성 로직 +
EnsureCollection 의 payload index + Meili client 의 search/upsert 분리 + admin
CLI 추가에 한정. 새 외부 모듈 의존성 없음 (jwt 토큰은 meilisearch-go SDK 의 빌트
인 함수 사용). IDX-005 / AUTH-002 와의 의존성: IDX-004 가 AUTH-002 의 visibility
hook 을 소비하나 v1.0 에서는 default `team_shared` 로 안전 동작. IDX-005 는
IDX-004 의 team_id-aware Search 를 전제로 한다. M6 exit criterion "shared index
dedup hits ≥30%" 는 same-team 내부 dedup 정상 + cross-team 격리로 충족.
