---
id: SPEC-IDX-004
title: Shared index multi-tenancy
version: 0.1.0
milestone: M6 — Team plane
status: draft
priority: P0
owner: expert-backend
methodology: tdd
coverage_target: 85
created: 2026-05-22
updated: 2026-05-22
author: limbowl
issue_number: 0
depends_on: [SPEC-IDX-001, SPEC-IDX-002, SPEC-IDX-003, SPEC-AUTH-001, SPEC-AUTH-002, SPEC-OBS-001]
blocks: [SPEC-IDX-005]
---

# SPEC-IDX-004: Shared index multi-tenancy

## HISTORY

- 2026-05-22 (initial draft v0.1.0, limbowl via manager-spec):
  First EARS-formatted SPEC for the M6 (Team plane) retrieval-layer release
  gate. SPEC-IDX-001 (implemented)이 세 store(Qdrant + Meilisearch +
  PostgreSQL)에 `team_id` 컬럼/필드/payload를 reserve만 했고 v0.1에서는 모든
  row가 `team_id = NULL`인 single-tenant 상태로 운영 중이다
  (`internal/index/dispatch.go:347`, `deploy/postgres/migrations/0001_create_docs.sql:16`).
  IDX-004는 그 reservation을 enforcement로 전환하여 M6 team plane의 모든
  downstream SPEC(IDX-005, AUTH-002 visibility consumer)이 의존할 수 있는
  multi-tenancy invariant를 제공한다.

  본 SPEC은 SPEC-AUTH-001(JWT 미들웨어가 `authctx.TeamIDKey`로 team_id를
  context inject)과 SPEC-AUTH-002(per-adapter visibility 정책 hook 노출)의
  결과를 consume하는 retrieval-layer enforcement다. AUTH ship 이전에는
  `INDEX_DEFAULT_TEAM` env var로 동일 context key에 단일 team을 주입하여
  forward-compat을 유지한다(SPEC-DEEP-004 D1/D2와 동일 패턴).

  10개 결정사항은 research §11에서 context-derived로 확정되었고 §1.1에
  재명시되어 본 SPEC의 EARS 요구사항으로 번역된다.

  Pinned decisions:
  (D1) Qdrant Tiered Multitenancy: 단일 collection 기본 + `team_id` payload
       `keyword` index에 `is_tenant=true` flag. 대형 팀은
       `usearch_docs__team_<team_id_hashed>` dedicated collection으로
       config-driven 승격 (v1.0은 manual list만; doc-count auto-tier는
       SPEC-IDX-007 deferred).
  (D2) Meilisearch per-tenant tokens: `meilisearch-go.GenerateTenantToken`
       (HMAC-SHA256, admin API key UID 서명) + `searchRules`에
       `team_id = "<T>"` 필터. TTL default 15분 + 만료 60초 전 백그라운드
       refresh. `(team_id, user_id, api_key_uid)` 단위 in-process cache.
  (D3) PG enforcement: `docs.team_id` `NOT NULL DEFAULT 'default'` 전환
       (migration `0004_team_id_not_null.sql`) + `user_id TEXT NULL` 컬럼
       추가 (migration `0005_user_id_column.sql`) + composite indexes
       `(team_id, source_id)`, `(team_id, published_at DESC)`, partial
       `(team_id, user_id) WHERE user_id IS NOT NULL`.
  (D4) Personal context tier: AUTH-002의 `Adapter.Visibility()`가
       `user_private`을 반환하면 doc에 `user_id` payload를 함께 저장하고
       retrieval 필터를 `team_id = $T AND (user_id = $U OR user_id = "")`로
       합성. `team_shared`는 기본, `public`은 reserved sentinel
       `__public__` team_id.
  (D5) Tenancy mode 환경 변수 `INDEX_MULTI_TENANCY_MODE`:
       `enforced` (v1.0 default) / `permissive` (legacy NULL row 허용) /
       `legacy` (이전 v0.1 동작). `enforced` 모드에서 `TeamID == ""` 요청은
       `ErrTeamIDRequired` 즉시 반환.
  (D6) Backfill admin CLI: `usearch admin backfill-team --default-team <id>
       [--dry-run] [--batch-size 1000]`. PG batch UPDATE + Qdrant
       `set_payload` API + Meili `UpdateDocuments` patch. 진행 상황을
       `internal/index/backfill/state.json`에 기록하여 crash recovery.
  (D7) Cross-team leak prevention sentinel: `team_id`는 caller가
       `NormalizedDoc.Metadata`에 직접 셋하지 않는다 — IDX-004가 ctx에서
       추출하여 주입. adapter가 임의 team_id 주입을 시도하는 attack vector
       차단.
  (D8) Observability extensions: 신규 label `team_id_hashed`
       (`hex(sha256(team_id))[:8]` — 약 4×10^9 bound, 실제 production
       100~10000 팀 안전) + `visibility` (enum bounded 3 values).
       `__public__`은 평문 유지. 평문 `team_id`는 label/span attribute로
       절대 노출 금지. 신규 metric family
       `usearch_index_tenant_token_*` (issued/revoked/validation_failures).
  (D9) Token validation cache 분리 없음: Meili가 토큰 검증을 자체 수행.
       Go-side는 발급 캐시만 유지. Invalid 시 401 → 재발급 후 1회 재시도.
  (D10) Public tier 의 Qdrant 격리: v1.0은 default-tier 동일 collection
        + `team_id = $T OR team_id = "__public__"` payload filter. dedicated
        public collection은 post-V1 (public doc 비중 <5%로 성능 영향 미미).

  M6 release gate의 두 번째 SPEC(AUTH-001 다음)으로서 별도 GitHub Issue
  트랙되지 않으며 (`issue_number: 0`) plan-auditor 통과 후 status
  `draft → approved` 전이. M6 exit criterion("shared index dedup hits ≥30%",
  `.moai/project/roadmap.md:155`)을 IDX-005가 달성하기 위한 multi-tenancy
  invariant를 본 SPEC이 제공한다.

  Companion artifacts:
  - `.moai/specs/SPEC-IDX-004/research.md` — Phase 0.5 research
    (~840 lines, 12 sections — existing reservation surface, Qdrant Tiered
    Multitenancy, Meili tenant tokens, PG migration strategy, personal
    context tier, backfill, leak prevention, observability extensions,
    AUTH-002 integration, race/leak analysis, 10 pinned decisions, 10
    open questions)
  - `.moai/specs/SPEC-IDX-004/plan.md` — TDD 6-phase task sequence
  - `.moai/specs/SPEC-IDX-004/tasks.md` — task decomposition
  - `.moai/specs/SPEC-IDX-004/acceptance.md` — Given/When/Then 시나리오
    (8 main + 2 boundary edges, including M6 cross-team isolation gate)
  - `.moai/specs/SPEC-IDX-004/spec-compact.md` — compact view
  - `.moai/specs/SPEC-IDX-004/progress.md` — progress tracker

  11 EARS REQs (9 × P0 + 2 × P1), 8 NFRs, 5 modules
  (Tenancy Enforcement / Qdrant Tiered Multitenancy / Meili Tenant Tokens /
  PG Migration & Backfill / Observability & Adapter Visibility).
  Methodology: TDD, coverage target 85%, harness: standard. Owner:
  expert-backend.

---

## 1. Overview

본 SPEC은 M6 milestone의 **retrieval-layer 데이터 격리 게이트**다.
SPEC-IDX-001(implemented, `.moai/specs/SPEC-IDX-001/spec.md:6`)이 세 store
에 `team_id` 필드를 reserve만 한 상태에서, IDX-004는 그 reservation을
enforcement로 전환하여 cross-team 데이터 leak을 정확히 0으로 보장한다.

본 SPEC이 ship되기 전 retrieval 계층의 모든 row는 `team_id = NULL`이며
같은 cluster 내 모든 사용자가 동일 데이터를 본다. M6의 team plane이
의미를 가지려면 (a) 다른 팀의 데이터가 retrieval 결과에 절대 섞이지
않아야 하고, (b) 같은 팀 내부에서는 같은 doc_id의 dedup이 정상 동작해야
한다. 두 invariant는 REQ-IDX4-001 + REQ-IDX4-007로 EARS-encode된다.

### 1.1 Pinned Architectural Decisions

다음 10개 결정은 research §11에서 context-derived로 확정되었다. 본 SPEC은
이를 EARS 요구사항으로 번역할 뿐 재논의하지 않는다.

1. **Qdrant Tiered Multitenancy**: 단일 `usearch_docs` collection 기본 +
   `team_id` keyword payload index에 `is_tenant=true` flag. 대형 팀은
   `usearch_docs__team_<team_id_hashed>` dedicated collection으로
   config-driven 승격. v1.0은 manual list만 활성, doc-count auto-tier는
   SPEC-IDX-007 deferred. HNSW `payload_m`/`m=0` 재설정도 SPEC-IDX-007
   deferred.
2. **Meilisearch per-tenant tokens**: `meilisearch-go.GenerateTenantToken`
   (HMAC-SHA256, admin API key UID 서명) + `searchRules`에
   `team_id = "<T>"` 필터를 두 인덱스(`usearch_docs`, `usearch_docs_ko`)
   에 모두 적용. TTL default 15분 + 만료 60초 전 백그라운드 refresh.
   `(team_id, user_id, api_key_uid)` 단위 in-process cache.
3. **PG enforcement**: `docs.team_id` `NOT NULL DEFAULT 'default'` 전환
   (migration `0004_team_id_not_null.sql`) + `user_id TEXT NULL` 컬럼
   추가 (migration `0005_user_id_column.sql`). composite indexes:
   `(team_id, source_id)`, `(team_id, published_at DESC)`, partial
   `(team_id, user_id) WHERE user_id IS NOT NULL`.
4. **Personal context tier**: AUTH-002의 `Adapter.Visibility()`가
   `user_private`을 반환하면 doc에 `user_id` payload를 함께 저장하고
   retrieval 필터를 `team_id = $T AND (user_id = $U OR user_id = "")`로
   합성. `team_shared`는 기본, `public`은 reserved sentinel
   `__public__` team_id.
5. **Tenancy mode env var**: `INDEX_MULTI_TENANCY_MODE`:
   `enforced` (v1.0 default) / `permissive` / `legacy`. `enforced` 모드
   에서 `TeamID == ""` 요청은 `ErrTeamIDRequired` 즉시 반환. caller는
   embedder/store fanout 모두 skip.
6. **Backfill admin CLI**: `usearch admin backfill-team --default-team <id>
   [--dry-run] [--batch-size 1000]`. PG batch UPDATE + Qdrant
   `set_payload` API + Meili `UpdateDocuments` patch. 진행 상황을
   `internal/index/backfill/state.json`에 기록하여 crash recovery.
7. **Cross-team leak prevention sentinel**: `team_id`는 caller가
   `NormalizedDoc.Metadata`에 직접 셋하지 않는다 — IDX-004가 ctx에서
   추출하여 주입. adapter가 임의 team_id 주입을 시도하는 attack vector
   차단.
8. **Observability extensions**: 신규 label `team_id_hashed`
   (`hex(sha256(team_id))[:8]`) + `visibility` (bounded enum: `team_shared`,
   `user_private`, `public`). `__public__`은 평문 유지. 평문 `team_id`는
   label/span attribute로 절대 노출 금지. 신규 metric family
   `usearch_index_tenant_token_*`.
9. **Token validation cache 분리 없음**: Meili가 토큰 검증을 자체 수행.
   Go-side는 발급 캐시만 유지. Invalid 시 401 → 재발급 후 1회 재시도.
10. **Public tier Qdrant 격리**: v1.0은 default-tier 동일 collection +
    `team_id = $T OR team_id = "__public__"` payload filter. dedicated
    public collection은 post-V1.

### 1.2 Motivation

M6 team plane의 retrieval invariant는 다음과 같다:

1. **Cross-team isolation**: team A 사용자가 호출한 search/retrieval은
   team B의 doc을 절대 포함하지 않는다. 어느 store 한 곳에서 격리가
   실패하면 전체 chain이 깨진다.
2. **Same-team dedup**: 같은 team 내 동일 `doc_id`의 재호출은 정상적으로
   하나의 row를 반환한다. `team_id`가 dedup의 transparent partition
   key로 작동해야 한다.
3. **Personal context tier**: GitHub PAT로 fetch한 private repo, Gmail
   같은 인격적 adapter의 결과는 같은 team이라도 ingest한 user만 볼 수
   있어야 한다. visibility 정책이 doc 단위로 retrieval에 반영된다.
4. **Operational safety**: NULL team_id row가 v0.x 데이터로 존재하므로
   backfill 도구가 idempotent + resumable + dry-run 가능해야 한다.

본 SPEC이 ship된 후 IDX-005(team-shared answer reuse)가 이 invariant
위에 build되어 M6 exit criterion("shared index dedup hits ≥30%")이 측정
가능 상태에 진입한다.

### 1.3 Relation to IDX-005 (Downstream Consumer)

IDX-005는 fanout 이전에 동일 team의 사전 답변을 retrieval하여 재사용한다.
IDX-005의 모든 lookup은 `IndexQuery{TeamID: T, DocTypes: [DocTypeCachedAnswer]}`
형태로 IDX-001 surface를 호출한다. 이때 본 SPEC의 4-layer defense(Qdrant
payload filter + Meili tenant token + PG NOT NULL constraint + dispatch.go
sentinel)가 모두 작동해야 IDX-005의 NFR-IDX5-004(cross-tenant leak
probability == 0)가 보장된다. 즉 본 SPEC의 EARS REQ는 IDX-005의 보안
invariant의 load-bearing surface다.

### 1.4 Relation to DEEP-004 (cost_ledger)

SPEC-DEEP-004(implemented, M5)는 이미 `cost_ledger` 테이블에 `tenant_id`
+ `user_id` 컬럼을 박았다 (`deploy/postgres/migrations/0002_cost_ledger.sql`).
IDX-004는 동일한 의미의 `team_id` 컬럼을 `docs` 테이블에 enforce한다.
두 SPEC의 식별자(DEEP-004의 `tenant_id` ↔ IDX-004의 `team_id`)는 **같은
논리 entity**를 가리키며, AUTH-001의 JWT claim `sub.team_id`가 두 곳에
동일하게 주입된다. v1.0에서는 두 명칭을 그대로 유지하며, 향후 SPEC-AUTH-003
(audit log)에서 column rename이 검토될 수 있다.

---

## 2. EARS Requirements

### 2.1 Tenancy Enforcement Module

| ID | Pattern | Requirement | Priority | Acceptance |
|----|---------|-------------|----------|------------|
| **REQ-IDX4-001** | Event-Driven | WHEN `index.Index.Search(ctx, IndexQuery)` 또는 `index.Index.Upsert(ctx, docs)`가 호출되고 `INDEX_MULTI_TENANCY_MODE`가 `enforced`(v1.0 default)일 때, dispatch hot-path 첫 단계에서 `extractTeamID(ctx)` (Search) 또는 `q.TeamID` (Search query)를 검사 SHALL 한다. 빈 문자열이면 typed error `ErrTeamIDRequired` (sentinel `errors.New("index: team_id required")`)를 즉시 SHALL 반환하며 embedder/store fanout은 SHALL NOT 호출된다. `permissive` 모드에서는 NULL team_id 허용 (legacy data migration window 한정), `legacy` 모드에서는 v0.1 동작(team_id 무시) 그대로 SHALL 작동한다. mode 전환은 환경 변수 + 프로세스 재시작으로만 적용된다 (hot-reload 미지원). | P0 | `TestSearchRejectsEmptyTeamIDInEnforcedMode`, `TestUpsertRejectsEmptyTeamIDInEnforcedMode`, `TestPermissiveModeAllowsNullTeamID`, `TestLegacyModePreservesV01Behavior`, `TestModeTransitionRequiresRestart` |
| **REQ-IDX4-002** | Ubiquitous | `index.Index.Upsert` path는 caller가 `NormalizedDoc.Metadata["team_id"]`를 명시적으로 셋하는 것을 SHALL NOT 허용한다. caller가 metadata에 `team_id` 키를 미리 set한 경우 IDX-004는 silently overwrite SHALL 하며 (caller-provided 값 무시) WARN level slog로 (`event_type: "idx4.upsert.team_id_overridden"`) SHALL 기록한다. `team_id` 값은 `extractTeamID(ctx)` 결과(JWT context 또는 `INDEX_DEFAULT_TEAM` env var fallback)만 SHALL 사용된다. 이는 adapter가 임의 team_id를 주입하여 cross-team write를 시도하는 attack vector를 SHALL 차단한다. | P0 | `TestUpsertIgnoresCallerProvidedTeamID`, `TestUpsertLogsWarnOnOverride`, `TestUpsertUsesContextTeamID`, `TestUpsertFallsBackToDefaultTeamEnvVar` |
| **REQ-IDX4-003** | Event-Driven | WHEN AUTH-001의 JWT 미들웨어가 ship되어 있으면 `authctx.TeamIDKey` (canonical: `auth.contextKey("team_id")`) 와 `authctx.UserIDKey`가 ctx에 inject되어 있어야 SHALL 한다. AUTH-001 ship 이전 또는 미인증 경로(admin CLI, scheduled job)에서는 환경 변수 `INDEX_DEFAULT_TEAM` 값을 fallback으로 SHALL 주입한다. 환경 변수도 부재하면 `enforced` 모드에서 REQ-IDX4-001 sentinel이 trigger되어 호출이 거절된다. context key 명명은 SPEC-DEEP-004 D2 패턴(`costguard.UserIDKey`)과 동일 규약을 SHALL 따른다. | P0 | `TestExtractTeamIDFromJWTContext`, `TestExtractTeamIDFallsBackToEnvVar`, `TestExtractTeamIDReturnsEmptyOnMissingBoth`, `TestContextKeyConventionMatchesDEEP004` |

### 2.2 Qdrant Tiered Multitenancy Module

| ID | Pattern | Requirement | Priority | Acceptance |
|----|---------|-------------|----------|------------|
| **REQ-IDX4-004** | Ubiquitous | `internal/index/qdrant/client.go::EnsureCollection`은 collection bootstrap 시점에 `team_id` payload field에 대해 `{"field_name": "team_id", "field_schema": {"type": "keyword", "is_tenant": true}}` payload index를 SHALL 생성한다 (Qdrant `PUT /collections/{name}/index` API). 기존 collection에 대해 idempotent SHALL add (이미 존재 시 무해). HNSW `payload_m`/`m=0` 재설정은 v1.0 SHALL NOT 적용 (SPEC-IDX-007 deferred). `EnsureCollection`은 booting 시 한 번 호출되며 `team_id_hashed` 라벨이 부착된 결과 메트릭 `usearch_index_qdrant_payload_index_ensured_total{is_tenant="true"}` 1 SHALL 증가시킨다. | P0 | `TestEnsureCollectionAddsIsTenantPayloadIndex`, `TestEnsureCollectionIdempotentOnExisting`, `TestEnsureCollectionDoesNotResetHNSW`, `TestEnsureCollectionMetricEmitted` |
| **REQ-IDX4-005** | Optional | WHERE 운영자가 `.moai/config/sections/index.yaml` `qdrant.tiering.dedicated_teams: [<team_id>, ...]` 리스트에 team을 추가한 경우, admin CLI `usearch admin tier-promote --team <id>`가 (a) 새 collection `usearch_docs__team_<sha256(team_id)[:16]>` 생성 (vector_size + distance metric = default와 동일), (b) default-tier collection의 `team_id == <id>` point들을 batch scroll + dispatch.go의 모든 Upsert/Search가 해당 team 호출 시 dedicated collection으로 라우팅, (c) 기존 default-tier point streaming 이동 (Qdrant scroll API + 신규 collection upsert + 기존 delete), (d) 완료 후 검증 (`scroll(default, filter=team_id==X).count == 0`)을 SHALL 수행한다. 도구는 `--dry-run`을 SHALL 지원하여 영향 row 수만 출력한다. v1.0은 manual list만 활성; doc-count auto-tier는 SPEC-IDX-007 deferred. | P1 | `TestTierPromoteCreatesDedicatedCollection`, `TestTierPromoteRoutesUpsertToDedicated`, `TestTierPromoteStreamingMoveCompletes`, `TestTierPromoteDryRunOutputsCountsOnly`, `TestTierPromoteVerifiesEmpty` |
| **REQ-IDX4-006** | Ubiquitous | `internal/index/qdrant/client.go::Search`의 filter builder는 query의 effective filter를 다음과 같이 SHALL 합성한다: (a) `q.TeamID` 가 non-empty이면 `team_id = <q.TeamID>` payload condition 추가 (must clause). (b) `q.TeamID`의 visibility가 `user_private`을 포함하는 경우 `team_id = <q.TeamID> AND (user_id = <q.UserID> OR user_id = "")` 합성 (must clause). (c) `q.IncludePublic == true`인 경우 `team_id IN [<q.TeamID>, "__public__"]` (should clause). (d) `__public__`은 reserved sentinel team_id로 사용자 input team_id로 SHALL NOT 허용된다 (REQ-IDX4-007 검증). | P0 | `TestSearchFilterAddsTeamIDCondition`, `TestSearchFilterAddsUserIDForPrivateVisibility`, `TestSearchFilterAddsPublicOnIncludeFlag`, `TestSearchFilterRejectsPublicSentinelAsUserInput` |
| **REQ-IDX4-007** | Ubiquitous | `__public__` sentinel team_id는 reserved이며 다음 입력 경로에서 SHALL 거절된다: (a) JWT claim `sub.team_id == "__public__"` → 인증 미들웨어 (AUTH-001) reject. (b) `INDEX_DEFAULT_TEAM=__public__` env var → process start-up validation error. (c) admin CLI `usearch admin backfill-team --default-team __public__` → CLI rejects with explicit message. (d) `usearch admin tier-promote --team __public__` → CLI rejects. v1.0은 `__public__` doc을 default-tier collection 안에 `team_id = "__public__"` payload로 저장하고 retrieval 시 `should: [team_id == $T, team_id == "__public__"]` 합성 (REQ-IDX4-006c). dedicated public collection은 post-V1 deferred. | P0 | `TestPublicSentinelRejectedInJWTClaim`, `TestPublicSentinelRejectedInEnvVar`, `TestPublicSentinelRejectedInBackfillCLI`, `TestPublicSentinelRejectedInTierPromoteCLI`, `TestPublicSentinelAcceptedAsAdapterVisibility` |

### 2.3 Meili Tenant Tokens Module

| ID | Pattern | Requirement | Priority | Acceptance |
|----|---------|-------------|----------|------------|
| **REQ-IDX4-008** | Event-Driven | WHEN `internal/index/meili/client.go::Search`가 호출되고 `enforced` mode인 경우, 미들웨어는 (a) `(team_id, user_id, admin_api_key_uid)` triplet으로 `internal/index/tenant/cache.go`에서 cached tenant token을 SHALL lookup; (b) cache miss 시 `meilisearch-go.GenerateTenantToken(apiKeyUID, searchRules, &TenantTokenOptions{ExpiresAt: now+15m})` 호출하여 신규 발급; (c) HMAC-SHA256 서명된 JWT의 `searchRules`에 두 인덱스 (`usearch_docs`, `usearch_docs_ko`) 각각에 `{"filter": "team_id = \"<team_id>\""}` 포함; (d) 발급된 토큰을 Meili search API의 `Authorization: Bearer <token>` 헤더로 SHALL 사용 (admin API key 대신). Upsert / EnsureIndex / Settings 등 admin operation은 admin API key를 SHALL 직접 사용 (tenant token은 search endpoint에만 적용됨). | P0 | `TestSearchUsesTenantToken`, `TestSearchTokenContainsCorrectSearchRules`, `TestSearchTokenSignedWithHMAC256`, `TestSearchTokenAppliesToKoreanShard`, `TestUpsertUsesAdminKeyNotTenantToken`, `TestTokenCacheReusesExistingEntry` |
| **REQ-IDX4-009** | Ubiquitous | `internal/index/tenant/cache.go`의 token cache는 다음 동작을 SHALL 보장한다: (a) `sync.Map` + TTL-based eviction (per-entry expires_at 추적); (b) 만료 60초 전부터 backgroun refresh worker가 새 토큰을 발급하여 stale-while-revalidate; (c) 동시 cache-miss는 `sync.Once` 보호로 같은 (team, user) 조합에 대해 단 한 번만 발급; (d) refresh worker는 ctx cancellation을 honor하여 graceful shutdown (`goleak.VerifyNone` PASS); (e) AUTH-002의 `team.member.removed` SSE/Redis pub-sub 이벤트(future hook point)를 구독하여 즉시 invalidate. v1.0은 hook point + DI seam만 정의하고 구체 wiring은 AUTH-002 ship 후 추가. cache는 메트릭 `usearch_index_tenant_token_issued_total{tier,outcome}`, `usearch_index_tenant_token_revoked_total{tier}`, `usearch_index_tenant_token_validation_failures_total{tier,outcome}`을 SHALL emit한다. | P0 | `TestTokenCacheRefreshesBeforeExpiry`, `TestTokenCacheSyncOnceUnderConcurrency`, `TestTokenCacheGracefulShutdown`, `TestTokenCacheRevocationHookPointExists`, `TestTokenCacheMetricsEmitted` |

### 2.4 PG Migration & Backfill Module

| ID | Pattern | Requirement | Priority | Acceptance |
|----|---------|-------------|----------|------------|
| **REQ-IDX4-010** | Ubiquitous | 본 SPEC은 다음 PG migration 파일을 SHALL 신설한다: (a) `deploy/postgres/migrations/0004_team_id_not_null.sql` — `docs.team_id`를 `NOT NULL DEFAULT 'default'`로 전환. single-transaction 내에서 Step 1 NULL row backfill (`UPDATE docs SET team_id = COALESCE(team_id, current_setting('app.default_team', true), 'default') WHERE team_id IS NULL`) + Step 2 `ALTER COLUMN team_id SET DEFAULT 'default'` + Step 3 `ALTER COLUMN team_id SET NOT NULL` + Step 4 composite indexes (`idx_docs_team_id_source_id ON docs (team_id, source_id)`, `idx_docs_team_published ON docs (team_id, published_at DESC)`). migration은 idempotent SHALL 한다 (`CREATE INDEX IF NOT EXISTS`, `ALTER TABLE ... IF EXISTS` 패턴). (b) `deploy/postgres/migrations/0005_user_id_column.sql` — `ALTER TABLE docs ADD COLUMN IF NOT EXISTS user_id TEXT NULL` + partial index `CREATE INDEX IF NOT EXISTS idx_docs_team_user ON docs (team_id, user_id) WHERE user_id IS NOT NULL`. | P0 | `TestMigration0004Idempotent`, `TestMigration0004BackfillsNullRows`, `TestMigration0004EnforcesNotNull`, `TestMigration0004CreatesCompositeIndexes`, `TestMigration0005AddsUserIDColumn`, `TestMigration0005CreatesPartialIndex` |
| **REQ-IDX4-011** | Optional | 본 SPEC은 admin CLI `usearch admin backfill-team --default-team <id> [--dry-run] [--batch-size 1000]`을 SHALL 제공한다 (`cmd/usearch/admin/backfill.go` 신설). 동작: (a) PG: `UPDATE docs SET team_id = $1 WHERE team_id IS NULL` batch 처리 (`WHERE doc_id IN (SELECT doc_id FROM docs WHERE team_id IS NULL LIMIT $batch_size)` 패턴으로 lock 시간 단축). (b) Qdrant: 각 point의 payload에 `set_payload` API로 `team_id` 추가, batch (default 1000 points/batch). (c) Meili: `meilisearch-go.UpdateDocuments`로 `team_id` 필드 patch (partial update). (d) 진행 상황을 `internal/index/backfill/state.json`에 기록 (last_processed_doc_id per store) 하여 crash 시 재개. (e) 완료 후 검증: `SELECT count(*) FROM docs WHERE team_id IS NULL` == 0, Qdrant scroll로 payload 검증, Meili count 검증. `--dry-run`은 변경 없이 PG 영향 row 수 + Qdrant 영향 point 수 + Meili 영향 document 수를 출력한다. metric `usearch_index_tenant_backfill_total{store,outcome}`을 SHALL emit한다. | P1 | `TestBackfillCLIDryRunOutputsCounts`, `TestBackfillCLIBatchUpdatesPostgres`, `TestBackfillCLIPatchesQdrantPayload`, `TestBackfillCLIPatchesMeiliDocuments`, `TestBackfillCLIResumesFromState`, `TestBackfillCLIVerifiesCompletion`, `TestBackfillCLIMetricsEmitted` |

---

## 3. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| **NFR-IDX4-001** | Cross-team leak probability | 본 SPEC의 모든 retrieval/upsert 경로에서 cross-team data leak 확률은 정확히 0 SHALL 한다. 4-layer defense (Qdrant payload filter via `is_tenant=true` index + Meili tenant token search rules + PG NOT NULL + dispatch.go sentinel `ErrTeamIDRequired`)가 모두 작동해야 한다. acceptance §5.6에서 cross-team probe로 검증되며, IDX-005의 NFR-IDX5-004 (정확히 0 leak)의 load-bearing surface다. |
| **NFR-IDX4-002** | Same-team dedup correctness | 같은 team 내 동일 `doc_id`의 재호출(upsert)은 PG `ON CONFLICT (doc_id) DO UPDATE` 패턴으로 idempotent SHALL 동작한다 (IDX-001 REQ-IDX-005 재사용). retrieval에서 동일 doc_id가 두 번 반환되지 SHALL NOT 한다. 본 invariant는 IDX-005의 M6 exit criterion("shared index dedup hits ≥30%") 측정의 전제다. |
| **NFR-IDX4-003** | Lookup overhead in enforced mode | `enforced` mode에서 team_id sentinel check + payload filter 추가로 인한 retrieval latency overhead는 p95 ≤ 10ms SHALL 한다. Qdrant `is_tenant=true` 의 co-location 효과로 pre-filter overhead는 < 5% (Qdrant 공식 벤치마크). Meili tenant token cache hit-rate는 정상 운영 시 ≥ 99% SHALL 유지된다. token cache miss 시 HMAC-SHA256 서명 ~30µs + cache 조회 ~1µs로 cold first call만 latency 추가. |
| **NFR-IDX4-004** | Token cache concurrency safety | 50 goroutine × 100 동시 Search 호출, 같은 team_id 5개 분포 조건에서 token cache는 race 없이 동작 SHALL 한다. `sync.Once` 보호로 같은 (team, user) 조합에 대해 토큰 발급은 단 한 번만 발생한다. refresh worker 1개는 graceful shutdown 가능 (`goleak.VerifyNone` PASS). |
| **NFR-IDX4-005** | Backfill atomicity & resumability | 백필 admin CLI는 (a) batch 단위 진행 시 PG lock 시간을 최소화 SHALL 한다 (sub-query LIMIT 패턴). (b) crash 시 `state.json`의 `last_processed_doc_id`에서 재개 SHALL 가능. (c) 진행 도중 새 ingest가 들어와도 NOT NULL constraint가 새 row의 team_id를 강제하므로 race 없음. (d) `--dry-run`은 변경 없이 영향 row 수를 정확히 출력 SHALL. |
| **NFR-IDX4-006** | Qdrant Tiered Multitenancy regression budget | `is_tenant=true` payload index 추가로 인한 retrieval p95 latency degradation은 ≤ 10% SHALL 한다. baseline은 IDX-001의 SLA(p50 ≤ 100ms, p95 ≤ 250ms — `internal/index/qdrant/client.go` SLA). 측정은 acceptance §5.5 integration test에서 100-query mixed-team 트래픽으로 수행. |
| **NFR-IDX4-007** | Observability cardinality bounded | 신규 label `team_id_hashed`는 `hex(sha256(team_id))[:8]` (4×10^9 가능값) — 실제 production 100~10000 팀 안전. `__public__`은 평문 유지. `visibility` enum 3 values (`team_shared`, `user_private`, `public`) bounded. 평문 `team_id`는 label/span attribute로 SHALL NOT 노출. `internal/obs/metrics/metrics_test.go::TestNoUnboundedLabels` allowlist 확장 필요. |
| **NFR-IDX4-008** | AUTH ship-before forward-compat | AUTH-001/002 ship 이전에 본 SPEC이 production 환경에서 안전 동작해야 SHALL 한다. `INDEX_DEFAULT_TEAM` env var fallback이 `extractTeamID(ctx)` 의 ctx 미부재 경로를 cover한다. visibility 정책 hook 미구현 adapter는 `team_shared`로 default 처리하여 모든 doc이 same-team 가시성. cross-team leak 위험은 없으나 public 데이터의 cross-team 재사용 효율은 v0.x 동안 손실됨. |

---

## 4. Exclusions (What NOT to Build)

[HARD] 본 SPEC은 다음 항목을 명시적으로 제외한다. 각 항목은 후속 SPEC 또는
별도 트랙의 책임이다.

- **Qdrant HNSW `payload_m`/`m=0` 재설정** — Qdrant 공식 문서가 권장하는
  per-tenant HNSW 옵션은 v1.0 SHALL NOT 적용. NFR-IDX4-006의 회귀
  benchmark 결과로 SPEC-IDX-007 (post-V1)에서 결정.
- **Doc-count 기반 auto-tier promotion** — v1.0은 manual list
  (`qdrant.tiering.dedicated_teams`)만 활성. 누적 doc count threshold
  기반 auto-tier 승격은 SPEC-IDX-007 deferred.
- **Dedicated public collection** — `__public__` doc은 v1.0에서
  default-tier collection 안에 payload `team_id = "__public__"`로 저장
  + `should: [team_id == $T, team_id == "__public__"]` 합성으로 처리.
  dedicated public collection은 post-V1 (public doc 비중 <5%로 성능 영향
  미미).
- **Token validation cache 분리** — Meili가 토큰 검증을 자체 수행하므로
  Go-side는 발급 캐시만 유지. validation cache 분리는 운영 복잡도만 증가.
  Invalid 시 401 → 재발급 후 1회 재시도.
- **`team_id` column rename in cost_ledger** — DEEP-004의 `cost_ledger.
  tenant_id`와 IDX-004의 `docs.team_id`는 의미적으로 같지만 v1.0에서는
  두 명칭을 그대로 유지. AUTH-003 audit log SPEC에서 unification 검토.
- **AUTH-002 visibility hook 의 wire-up 구체화** — v1.0은 interface +
  DI seam만 정의 (`Adapter.Visibility()` 가 미구현된 adapter는 `team_shared`
  로 default). 구체 wiring은 AUTH-002 ship 후 추가.
- **`team.member.removed` event 의 wire-up 구체화** — token revocation
  hook point + DI seam만 정의. SSE/Redis pub-sub 구독은 AUTH-002 ship 후
  추가.
- **AUTH-001/002 의 자체 구현** — 본 SPEC은 두 SPEC이 정의하는 context
  key (`authctx.TeamIDKey`, `authctx.UserIDKey`)를 consume할 뿐, 두 SPEC
  의 JWT 검증/RBAC 자체는 SHALL NOT 구현한다. AUTH ship 이전에는
  `INDEX_DEFAULT_TEAM` env var fallback으로 forward-compat.
- **Cross-region tenancy** — 본 SPEC은 single-cluster 가정. multi-region
  데이터 격리(GDPR data residency 등)는 post-V1 + 별도 SPEC.
- **Migration rollback automation** — `0004` migration은 NOT NULL 전환만
  되돌리고 backfill된 `default` 팀 row를 NULL로 되돌리지 않는다. 운영자가
  명시적으로 `UPDATE docs SET team_id = NULL WHERE team_id = 'default'`
  실행 필요 (사실상 IDX-004 자체 rollback 의미).
- **GitHub Issue tracking on this SPEC** (`issue_number: 0` per session
  pattern — orchestrator handles).

---

## 5. Acceptance Scenarios

상세 Given/When/Then 시나리오는 `.moai/specs/SPEC-IDX-004/acceptance.md`에
정의되어 있다. 본 절은 인덱스를 제공한다.

| Scenario | 설명 | Coverage |
|----------|------|----------|
| §5.1 | team T 사용자 enforced mode upsert + search round-trip → team_id 정확히 박힘 | REQ-001, 002, 003, 004 |
| §5.2 | Meili tenant token 발급 + cache 재사용 + Korean shard에도 적용 | REQ-008, 009 |
| §5.3 | personal context tier: user_private adapter doc이 ingestor만 보임 | REQ-002, 006 |
| §5.4 | __public__ sentinel doc이 모든 team retrieval에 합성됨 | REQ-006, 007 |
| §5.5 | NFR regression: 100-query mixed-team 트래픽 p95 ≤ 10% degradation | NFR-003, 006 |
| §5.6 | **M6 EXIT CONTRIBUTION**: cross-team probe — team U가 team T doc 접근 시도 → 0 leak | NFR-001, REQ-001, 006, 007, 008 |
| §5.7 | backfill admin CLI dry-run + 실제 실행 + resume from crash | REQ-011, NFR-005 |
| §5.8 | Qdrant tier-promote admin CLI: 대형 팀의 dedicated collection 승격 | REQ-005 |
| Edge1 | tenancy mode 전환 (`permissive` → `enforced`) 시 NULL row 거절 동작 | REQ-001 |
| Edge2 | token cache concurrency: 50 goroutine × 100 호출 race-free | NFR-004 |

---

## 6. Dependencies & Blocks

### 6.1 Upstream SPEC dependencies (depends_on)

- **SPEC-IDX-001** (implemented) — `*index.Index.Search` / `Upsert` 공개
  surface + dispatch.go 의 payload 생성 경로 + Qdrant `EnsureCollection`
  + Meili `EnsureIndex` 모두 직접 확장. `team_id` reservation surface
  (`internal/index/types.go:25-27`, `internal/index/dispatch.go:347`,
  `internal/index/qdrant/client.go:242`)를 enforcement로 전환. **HARD dep**.
- **SPEC-IDX-002** (implemented) — BGE-M3 embedder. team-agnostic이므로
  본 SPEC은 변경 없음. 다만 embedder cache 키에 team_id를 추가하지
  **않는다** (cache hit-rate가 팀 수에 반비례 폭락 방지). **SOFT dep**.
- **SPEC-IDX-003** (implemented) — Korean tokenizer + ko shard. 두
  인덱스 (`usearch_docs`, `usearch_docs_ko`) 모두에 tenant token 적용
  필요. **HARD dep**.
- **SPEC-AUTH-001** (M6, draft) — JWT 미들웨어가 `authctx.TeamIDKey`,
  `authctx.UserIDKey`를 ctx에 inject. 본 SPEC은 이 context key를 단일
  진실 공급원으로 consume. AUTH-001 ship 이전에는 `INDEX_DEFAULT_TEAM`
  env var fallback (NFR-008). **HARD dep** (forward-compat with fallback).
- **SPEC-AUTH-002** (M6, draft) — `Adapter.Visibility()` 메타 함수가
  `team_shared` / `user_private` / `public` 반환. 본 SPEC은 visibility
  hook 구조만 정의하고 구체 wiring은 AUTH-002 ship 후 추가. **HARD dep**
  (forward-compat with default).
- **SPEC-OBS-001** (implemented) — Prometheus naming + cardinality 화이트
  리스트 메커니즘 (`internal/obs/metrics/metrics.go:171-176`). 본 SPEC
  은 신규 label `team_id_hashed`, `visibility`, `tier` 추가. **HARD dep**.

### 6.2 Downstream blocked SPECs (blocks)

- **SPEC-IDX-005** (M6, draft) — team-shared answer reuse. 본 SPEC의
  team_id-aware Search가 정확히 동일 team의 답변만 반환하도록 보장하는
  것을 전제로 build됨. NFR-IDX5-004 (cross-tenant leak == 0)는 본 SPEC
  의 NFR-IDX4-001의 load-bearing surface.

### 6.3 Forward-compatibility commitment with SPEC-AUTH-003 audit log

본 SPEC의 metric family `usearch_index_tenant_token_*` + slog event
`event_type: "idx4.upsert.team_id_overridden"`은 AUTH-003 audit log
subsystem이 downstream consumer로 합류할 때 호환되도록 설계된다. slog
line schema는 additive only이며 AUTH-003가 새 필드를 추가할 수 있으나
필드를 rename하거나 remove할 수 SHALL NOT.

### 6.4 Forward-compatibility commitment with SPEC-IDX-007 (post-V1)

본 SPEC의 Tiered Multitenancy는 v1.0에서 single-collection 기본 + manual
dedicated list로만 활성화된다. SPEC-IDX-007 (post-V1)이 (a) doc-count
auto-tier, (b) HNSW `payload_m`/`m=0` 재설정, (c) dedicated public
collection을 추가할 때 본 SPEC의 `qdrant.tiering.*` config namespace
와 admin CLI surface(`usearch admin tier-promote`)를 재사용한다.

---

## 7. Files to Create / Modify

### 7.1 Created

| Marker | Path | Purpose |
|--------|------|---------|
| [NEW] | `internal/index/tenant/cache.go` | Meili tenant token in-process cache + refresh worker + revocation hook point |
| [NEW] | `internal/index/tenant/issuer.go` | `meilisearch-go.GenerateTenantToken` wrapper + HMAC-SHA256 서명 검증 helper |
| [NEW] | `internal/index/tenant/types.go` | `TenantTokenEntry`, `TenancyMode` enum, `Visibility` enum 등 |
| [NEW] | `internal/index/tenant/cache_test.go` | concurrency safety + ttl eviction + sync.Once + goleak |
| [NEW] | `internal/index/tenant/issuer_test.go` | token 발급 + searchRules 검증 + Korean shard 적용 |
| [NEW] | `internal/index/backfill/cli.go` | admin CLI `usearch admin backfill-team` 본체 |
| [NEW] | `internal/index/backfill/state.go` | state.json 파싱/저장 + resume logic |
| [NEW] | `internal/index/backfill/cli_test.go` | dry-run, batch, resume, verify 테스트 |
| [NEW] | `internal/index/backfill/state.json` | (runtime artifact, .gitignore 대상; 코드는 schema만 정의) |
| [NEW] | `cmd/usearch/admin/backfill.go` | CLI 진입점 + flag parsing |
| [NEW] | `cmd/usearch/admin/tier_promote.go` | CLI 진입점 `usearch admin tier-promote` (REQ-005) |
| [NEW] | `cmd/usearch/admin/tier_promote_test.go` | dry-run + streaming move + verify |
| [NEW] | `internal/index/auth/context.go` | `authctx.TeamIDKey`, `authctx.UserIDKey` 정의 + `extractTeamID(ctx)` / `extractUserID(ctx)` helper + env var fallback |
| [NEW] | `internal/index/auth/context_test.go` | context key extract + fallback + DEEP-004 convention 검증 |
| [NEW] | `internal/index/tenancy/mode.go` | `INDEX_MULTI_TENANCY_MODE` parse + `ErrTeamIDRequired` sentinel error |
| [NEW] | `internal/index/tenancy/mode_test.go` | mode 분기 + sentinel error 동작 |
| [NEW] | `deploy/postgres/migrations/0004_team_id_not_null.sql` | docs.team_id NOT NULL + composite indexes migration |
| [NEW] | `deploy/postgres/migrations/0005_user_id_column.sql` | docs.user_id + partial index migration |
| [NEW] | `internal/index/pg/migrations_test.go` | 두 migration idempotency + backfill + index 존재 검증 |
| [NEW] | `internal/index/tenant_integration_test.go` | end-to-end (Qdrant + Meili + PG testcontainers) tenancy enforcement |
| [NEW] | `internal/obs/metrics/tenant.go` | 4 신규 collector 정의 + registerIDX4 helper |

### 7.2 Modified

| Path | Change |
|------|--------|
| `internal/index/dispatch.go` | (a) `Search`/`Upsert` 진입 시점에 `tenancy.Mode()` + `extractTeamID(ctx)` sentinel 추가 (REQ-001). (b) Upsert path가 `NormalizedDoc.Metadata["team_id"]`를 silent overwrite (REQ-002). (c) Meili document 생성에서 `"team_id": nil` 박힌 line 제거 — 실제 team_id를 set (line 347 부근). (d) Qdrant payload 생성에서 `user_id` 추가 (visibility==user_private인 경우). |
| `internal/index/qdrant/client.go` | (a) `EnsureCollection`이 `team_id` payload index를 `is_tenant=true`로 SHALL 생성 (REQ-004). (b) `Search` filter builder가 visibility-aware 합성 (REQ-006). (c) public sentinel 처리 (REQ-007). |
| `internal/index/meili/client.go` | (a) `Search` path가 tenant token 사용 (REQ-008). (b) `Upsert` / `EnsureIndex` / `Settings`는 admin API key 직접 사용 유지. (c) `usearch_docs_ko` shard에도 동일 token 적용 (REQ-008). (d) Meili `FilterableAttributes`에 `user_id` 추가 (REQ-006). |
| `internal/index/meili/korean_shard.go` | 이미 `team_id` filterable 등록되어 있음 (IDX-003); `user_id` 필드 추가 |
| `internal/index/index.go:108` | `FilterableAttributes`에 `user_id` 추가 |
| `internal/index/types.go:25-27` | `IndexQuery.TeamID` 필드 주석 update — v1.0 enforcement 의미로 전환. `UserID`, `IncludePublic` 필드 신설. |
| `internal/index/pg/client.go` | INSERT/SELECT 경로에서 `team_id`를 caller-context 기반으로 set (NULL 대신). `user_id` 컬럼도 visibility=user_private인 경우 set. |
| `cmd/usearch/main.go` | admin sub-command 등록 (`backfill-team`, `tier-promote`) + tenancy mode 환경 변수 startup validation |
| `internal/obs/metrics/metrics.go` | `registerIDX4(r)` helper 호출 추가 |
| `internal/obs/metrics/metrics.go:171-176` | cardinality allowlist에 `team_id_hashed`, `visibility`, `tier` 추가 |
| `internal/obs/obs.go` | `obs.IDX4TokenIssued`, `obs.IDX4TokenRevoked`, `obs.IDX4TokenValidation`, `obs.IDX4Backfill` collector re-export |
| `internal/obs/metrics/metrics_test.go` | `TestNoUnboundedLabels` 화이트리스트 확장 (NFR-IDX4-007) |
| `pkg/types/adapter.go` | (forward-compat with AUTH-002) `Adapter.Visibility()` 인터페이스 method 신설. 미구현 adapter는 `team_shared` default (NFR-IDX4-008). |
| `.moai/config/sections/index.yaml` | 신설: `qdrant.tiering.dedicated_teams: []`, `meili.tenant_token.ttl_minutes: 15`, `default_team` 등 |
| `.env.example` | `INDEX_MULTI_TENANCY_MODE`, `INDEX_DEFAULT_TEAM`, `MEILI_TENANT_TOKEN_TTL_MINUTES` 등 신규 env-var 문서화 |

### 7.3 Existing — Unchanged

- `internal/embedder/` (IDX-002) — embedder cache는 team-agnostic 유지
  (cache hit-rate 보호).
- `services/researcher/` (SYN-001) — Python 사이드카는 본 SPEC 변경 없음.
- `internal/access/` (CACHE-001) — 변경 없음.
- `internal/fanout/` (FAN-001) — 변경 없음 (호출자가 ctx에 team_id 주입).
- `deploy/postgres/migrations/0001_create_docs.sql`, `0002_cost_ledger.sql`,
  `0003_answer_cache.sql` (IDX-005) — 변경 없음 (IDX-004는 0004, 0005만
  신설).

---

## 8. Open Questions

본 SPEC은 §1.1의 10개 pinned decision으로 대부분의 ambiguity를 해소했다.
다음 항목은 plan-auditor와의 협의 또는 첫 운영 데이터 기반 튜닝이 필요한
경계 사례다.

1. **Default team identifier**: 기본 team의 식별자. v1.0은 env var
   `INDEX_DEFAULT_TEAM` 우선, fallback `"default"` (DEEP-004의 default
   tenant와 동일 명명). 운영자 변경 가능. **Owner**: 운영팀 deploy
   decision.

2. **Token TTL**: 15분 default. 운영자가 `.moai/config/sections/index.yaml`
   로 5분~1시간 사이 조정 가능. 5분은 회전 비용 ↑ + 권한 변경 반영 빠름,
   1시간은 회전 비용 ↓ + AUTH-002 revocation 반영 지연. **Owner**:
   first-30-day 운영 데이터로 튜닝.

3. **team_id format validation**: v1.0은 정규식 `^[a-zA-Z0-9_-]{1,64}$` +
   reserved sentinel(`__public__`, `system`) 거절. UUID 강제는 변환 비용
   ↑로 미적용. **Owner**: AUTH-001의 JWT claim format과 align 필요.

4. **Dedicated tier promotion trigger**: v1.0은 manual list만. doc-count
   auto-tier는 SPEC-IDX-007 deferred. **Owner**: post-V1 SPEC author.

5. **user_id ingest 경로 (stateful vs stateless adapter)**: stateful
   adapter는 자체 OAuth context로 user_id 제공, stateless adapter는
   IDX-004가 ctx에서 추출. 두 경로 모두 지원. **Owner**: AUTH-002 +
   adapter 별 구현 결정.

5개는 plan-auditor가 SPEC을 PASS로 평가하기 위해 필수적인 결정이 아니다.
모두 first-30-day 운영 데이터 또는 후속 SPEC로 튜닝 가능한 항목이다.

---

## 9. References

### External (URL-cited; verified per research §12)

- https://qdrant.tech/documentation/guides/multiple-partitions/ — Qdrant
  Tiered Multitenancy 공식 가이드. `group_id` payload field, `is_tenant=true`
  payload index, single vs dedicated collection 결정 기준, HNSW pre-filter
  의 co-location 효과. REQ-IDX4-004 / REQ-IDX4-006 근거.
- https://www.meilisearch.com/docs/learn/security/multi_tenancy — Meilisearch
  tenant token (JWT) 공식 가이드. `searchRules`, `exp`, admin vs search
  key 구분, XSS 주의. REQ-IDX4-008 / REQ-IDX4-009 근거.
- https://github.com/meilisearch/meilisearch-go — meilisearch-go SDK
  v0.36.2. `GenerateTenantToken(apiKeyUID, searchRules, options) (string,
  error)` 함수 시그니처 + HMAC-SHA256 서명. REQ-IDX4-008 근거.
- https://qdrant.tech/documentation/concepts/payload/ — Qdrant payload
  schema, keyword vs numeric index 구분.
- https://qdrant.tech/documentation/concepts/filtering/ — Qdrant filter
  expression, must / should / must_not.

### Internal (file:line cited)

- `.moai/specs/SPEC-IDX-004/research.md` — 본 SPEC의 research artifact
  (~840 lines, 12 sections).
- `.moai/specs/SPEC-IDX-001/spec.md:6` — IDX-001 status: implemented.
- `.moai/specs/SPEC-IDX-001/spec.md:16` — IDX-001 blocks: [...SPEC-IDX-004...].
- `.moai/specs/SPEC-IDX-001/spec.md:100-106` — D10 multi-tenancy reservation.
- `.moai/specs/SPEC-IDX-001/spec.md:226-258` — IDX-001 §2.8 multi-tenancy
  forward-compat surface.
- `.moai/specs/SPEC-IDX-002/spec.md` — implemented BGE-M3 embedder.
- `.moai/specs/SPEC-IDX-003/spec.md` — implemented Korean tokenizer + ko shard.
- `.moai/specs/SPEC-AUTH-001/spec.md` — M6 OIDC SSO + JWT 미들웨어, context
  key 패턴 (`authctx.TeamIDKey`, `authctx.UserIDKey`) 공급원.
- `.moai/specs/SPEC-AUTH-002/spec.md` — M6 Casbin RBAC + per-adapter
  visibility, `Adapter.Visibility()` hook 공급원.
- `.moai/specs/SPEC-IDX-005/spec.md` — M6 team-shared answer reuse, 본
  SPEC의 immediate downstream consumer.
- `.moai/specs/SPEC-DEEP-004/spec.md:11` — DEEP-004 D1 X-User-Id
  forward-compat 패턴, context key 명명 convention 소스.
- `.moai/specs/SPEC-DEEP-004/spec.md` — DEEP-004 cost_ledger schema with
  tenant_id + user_id columns (v1.0 column name 유지 정책 근거).
- `.moai/specs/SPEC-OBS-001/spec.md` — observability baseline + cardinality
  allowlist mechanism.
- `internal/index/index.go:108` — Meili FilterableAttributes 현 상태
  (team_id 포함).
- `internal/index/dispatch.go:232` — Meili filter expression builder team_id.
- `internal/index/dispatch.go:309-319` — Qdrant payload team_id from
  doc.Metadata.
- `internal/index/dispatch.go:347` — Meili document `team_id: nil`
  (v0.1 reservation; REQ-IDX4-002에서 실제 set으로 전환).
- `internal/index/meili/korean_shard.go:27` — Korean shard team_id
  filterable (IDX-003 forward-compat).
- `internal/index/qdrant/client.go:242` — Qdrant filter team_id key.
- `internal/index/pg/client.go:121, 171, 230, 246` — PG client team_id
  컬럼 지원 (NULL 박힘).
- `internal/index/types.go:25-27` — IndexQuery.TeamID 필드 (v0.1 의미
  → v1.0 enforcement 의미로 전환).
- `deploy/postgres/migrations/0001_create_docs.sql:16` — docs.team_id
  컬럼 reservation.
- `deploy/postgres/migrations/0001_create_docs.sql:25` — team_id B-tree
  인덱스.
- `internal/obs/metrics/metrics.go:171-176` — cardinality allowlist
  (NFR-OBS-002).
- `.moai/project/roadmap.md:80-87` — M6 SPEC backlog (AUTH + IDX-004/005).
- `.moai/project/roadmap.md:128` — M6 parallelization plan
  (AUTH-001/002/003 + IDX-004/005 동시 가능).
- `.moai/project/roadmap.md:155` — M6 exit criterion ("shared index
  dedup hits ≥30%", IDX-005가 직접 측정; 본 SPEC이 multi-tenancy
  invariant 제공).
- `.moai/project/tech.md:39-50` — retrieval layer choices (Qdrant Tiered
  Multitenancy, Meili per-tenant tokens 명시).
- `.moai/project/tech.md:65-72` — team plane stack.
- `.moai/config/sections/quality.yaml` — `development_mode: tdd`,
  `test_coverage_target: 85`.
- `.moai/config/sections/language.yaml` — `documentation: en`,
  `code_comments: en`, `conversation_language: ko`.

---

*End of SPEC-IDX-004 v0.1.0 (draft; pending plan-auditor cycle).*
