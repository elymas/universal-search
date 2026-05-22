# SPEC-IDX-004 Implementation Plan

Generated: 2026-05-22
Methodology: TDD (RED-GREEN-REFACTOR)
Coverage target: 85%
Harness: standard

---

## 1. Overview

본 plan.md는 SPEC-IDX-004의 구현 단계별 task sequence를 정의한다. M6 retrieval
-layer enforcement gate로서 본 SPEC은 SPEC-IDX-001/002/003 (implemented) 위에
build되며, SPEC-AUTH-001/002의 ship 이전에는 `INDEX_DEFAULT_TEAM` env var
fallback (NFR-IDX4-008)으로 forward-compat을 유지한다. 11 EARS REQs + 8 NFRs
를 6개 phase로 분해하며 각 phase는 RED → GREEN → REFACTOR 사이클을 따른다.

본 SPEC은 **M6 exit criterion("shared index dedup hits ≥30%")의 enabling
invariant**다. plan-auditor 통과 후 manager-tdd가 phase별로 진행하며, Phase F
의 cross-team isolation integration test가 IDX-005 ship 전 필수 게이트다.

---

## 2. Phase Breakdown

### Phase A — Context & Tenancy Mode Foundation

목표: tenancy mode parse + context key 추출 + sentinel error를 단독 모듈로
완성한다. hot-path 코드의 모든 후속 단계가 이 모듈에 의존한다.
순서 근거: 가장 dependency 없는 기반 모듈을 먼저 RED 테스트로 고정.

**RED tests** (6):

1. `TestModeParsesEnforcedDefault` — `INDEX_MULTI_TENANCY_MODE` 미설정 →
   `enforced` (REQ-001).
2. `TestModeParsesPermissive` / `TestModeParsesLegacy` / `TestModeParsesInvalid`
   — env var 값 분기 (REQ-001).
3. `TestExtractTeamIDFromJWTContext` — `authctx.TeamIDKey` ctx에서 추출
   (REQ-003).
4. `TestExtractTeamIDFallsBackToEnvVar` — JWT 미부재 시 `INDEX_DEFAULT_TEAM`
   사용 (REQ-003).
5. `TestExtractTeamIDReturnsEmptyOnMissingBoth` — 둘 다 부재 → 빈 문자열
   (`enforced` 모드에서 후속 sentinel trigger) (REQ-003).
6. `TestContextKeyConventionMatchesDEEP004` — context key 명명이
   `costguard.UserIDKey` 패턴과 align (REQ-003).

**GREEN tasks**:

- `internal/index/tenancy/mode.go` 작성. `Mode` enum + `ParseMode(env)` +
  `ErrTeamIDRequired` sentinel.
- `internal/index/auth/context.go` 작성. `authctx.TeamIDKey`,
  `authctx.UserIDKey` 정의 + `extractTeamID(ctx)` / `extractUserID(ctx)`
  helper.
- `internal/index/auth/context_test.go` + `internal/index/tenancy/mode_test.go`
  작성.

**REFACTOR**:

- mode 분기 helper를 pure function으로 추출.
- env var 우선순위 logic을 별도 함수로 분리.

---

### Phase B — PG Migration Foundation

목표: PG `docs.team_id` `NOT NULL DEFAULT 'default'` 전환 + `user_id` 컬럼
+ composite indexes migration을 먼저 적용 가능 상태로 만든다.
순서 근거: 후속 dispatch.go enforcement가 PG schema에 의존.

**RED tests** (6):

7. `TestMigration0004Idempotent` — `0004_team_id_not_null.sql` 두 번 실행해도
   같은 schema 도달 (REQ-010).
8. `TestMigration0004BackfillsNullRows` — NULL team_id row가 `'default'`로
   채워짐 (REQ-010).
9. `TestMigration0004EnforcesNotNull` — Step 2 후 NULL INSERT 시도 시 PG
   constraint violation (REQ-010).
10. `TestMigration0004CreatesCompositeIndexes` — `idx_docs_team_id_source_id`
    + `idx_docs_team_published` 존재 (REQ-010).
11. `TestMigration0005AddsUserIDColumn` — `user_id TEXT NULL` 컬럼 추가됨
    (REQ-010).
12. `TestMigration0005CreatesPartialIndex` — `idx_docs_team_user` partial
    index `WHERE user_id IS NOT NULL` 존재 (REQ-010).

**GREEN tasks**:

- `deploy/postgres/migrations/0004_team_id_not_null.sql` 작성 (idempotent
  + single-transaction). `current_setting('app.default_team', true)` +
  `COALESCE` 패턴 사용.
- `deploy/postgres/migrations/0005_user_id_column.sql` 작성 (`ADD COLUMN
  IF NOT EXISTS` + partial index `CREATE INDEX IF NOT EXISTS ... WHERE`).
- `internal/index/pg/migrations_test.go` 작성 (testcontainers PG 사용).

**REFACTOR**:

- migration test helper 통합 (apply + verify + idempotency check).
- backfill batch size 검증 helper.

---

### Phase C — Dispatch Enforcement + Upsert Override

목표: dispatch.go의 `Search`/`Upsert` 진입점에 tenancy sentinel + caller-
provided team_id silent overwrite를 구현한다. v0.1의 `"team_id": nil` line
이 사라지고 실제 team_id가 박힌다.

**RED tests** (10):

13. `TestSearchRejectsEmptyTeamIDInEnforcedMode` — `q.TeamID == ""` →
    `ErrTeamIDRequired` 즉시 반환, embedder 미호출 (REQ-001).
14. `TestUpsertRejectsEmptyTeamIDInEnforcedMode` — ctx에 team_id 부재 →
    `ErrTeamIDRequired`, store fanout 미호출 (REQ-001).
15. `TestPermissiveModeAllowsNullTeamID` — `permissive` 모드에서 NULL
    team_id 허용 (REQ-001).
16. `TestLegacyModePreservesV01Behavior` — `legacy` 모드에서 team_id 무시
    (REQ-001).
17. `TestModeTransitionRequiresRestart` — hot-reload 미지원 검증 (REQ-001).
18. `TestUpsertIgnoresCallerProvidedTeamID` — `NormalizedDoc.Metadata["team_id"]`
    가 caller에 의해 set되었어도 IDX-004가 overwrite (REQ-002).
19. `TestUpsertLogsWarnOnOverride` — slog WARN 발생 검증 (`event_type:
    "idx4.upsert.team_id_overridden"`) (REQ-002).
20. `TestUpsertUsesContextTeamID` — ctx 의 team_id가 최종 doc에 박힘
    (REQ-002).
21. `TestUpsertFallsBackToDefaultTeamEnvVar` — JWT 미부재 + env var 있음
    → env var 값 사용 (REQ-002, REQ-003).
22. `TestUpsertSetsUserIDForPrivateVisibility` — visibility=user_private인
    경우 user_id payload 박힘 (REQ-002).

**GREEN tasks**:

- `internal/index/dispatch.go` 수정:
  - `Search` 진입점에 mode check + sentinel
  - `Upsert` 진입점에 ctx에서 team_id 추출 + silent overwrite + WARN slog
  - Meili document 생성에서 `"team_id": nil` 박힌 line 제거 (line 347 부근)
    → 실제 team_id set
  - Qdrant payload에 visibility=user_private인 경우 user_id 추가
- `internal/index/pg/client.go` 수정:
  - INSERT/SELECT 경로에서 team_id를 caller-context 기반 set
  - user_id 컬럼도 visibility=user_private인 경우 set

**REFACTOR**:

- enforcement 로직을 `internal/index/tenancy/enforce.go` 헬퍼로 추출
  (`func enforceTenantContext(ctx, mode) (teamID string, err error)`).
- `dispatch.go::Search` / `Upsert`가 헬퍼 단일 호출로 진입 검증.

---

### Phase D — Qdrant Tiered Multitenancy

목표: `EnsureCollection`에 `is_tenant=true` payload index 추가 + Search
filter builder의 visibility-aware 합성 + `__public__` sentinel 처리.

**RED tests** (9):

23. `TestEnsureCollectionAddsIsTenantPayloadIndex` — bootstrap 시
    `team_id` keyword + `is_tenant=true` index 생성 (REQ-004).
24. `TestEnsureCollectionIdempotentOnExisting` — 두 번 호출해도 same
    schema (REQ-004).
25. `TestEnsureCollectionDoesNotResetHNSW` — `payload_m`/`m=0` 미적용
    (REQ-004).
26. `TestEnsureCollectionMetricEmitted` — `usearch_index_qdrant_payload_
    index_ensured_total{is_tenant="true"}` 1 증가 (REQ-004).
27. `TestSearchFilterAddsTeamIDCondition` — `q.TeamID` non-empty → must
    clause 추가 (REQ-006).
28. `TestSearchFilterAddsUserIDForPrivateVisibility` — user_private →
    `team_id = $T AND (user_id = $U OR user_id = "")` 합성 (REQ-006).
29. `TestSearchFilterAddsPublicOnIncludeFlag` — `IncludePublic == true` →
    `should: [team_id == $T, team_id == "__public__"]` (REQ-006).
30. `TestSearchFilterRejectsPublicSentinelAsUserInput` — caller가
    `q.TeamID == "__public__"` 설정 시 거절 (REQ-007).
31. `TestPublicSentinelAcceptedAsAdapterVisibility` — adapter visibility로
    `public`인 doc은 `team_id == "__public__"` 박힘 + retrieval 정상
    (REQ-007).

**GREEN tasks**:

- `internal/index/qdrant/client.go::EnsureCollection` 수정. Qdrant `PUT
  /collections/{name}/index` API 호출. idempotent.
- `internal/index/qdrant/client.go::Search` filter builder 수정. must /
  should / must_not 분기에 따라 합성.
- `__public__` sentinel 처리: `IndexQuery.TeamID == "__public__"` 거절 +
  payload 값으로는 허용.

**REFACTOR**:

- filter builder를 pure function으로 추출. test는 filter expression
  output 검증.
- public sentinel 처리를 `internal/index/tenancy/public.go`로 분리.

---

### Phase E — Meili Tenant Tokens + Cache

목표: `meilisearch-go.GenerateTenantToken` 발급 + in-process cache +
refresh worker + revocation hook point + 두 인덱스 적용.

**RED tests** (11):

32. `TestSearchUsesTenantToken` — Meili Search 호출 시 admin key 대신
    tenant token 사용 (REQ-008).
33. `TestSearchTokenContainsCorrectSearchRules` — `searchRules`에 두
    인덱스 모두 + `team_id = "<T>"` filter 포함 (REQ-008).
34. `TestSearchTokenSignedWithHMAC256` — JWT 서명 검증 (REQ-008).
35. `TestSearchTokenAppliesToKoreanShard` — `usearch_docs_ko`에도 같은
    token 적용 (REQ-008).
36. `TestUpsertUsesAdminKeyNotTenantToken` — Upsert는 admin API key 사용
    유지 (REQ-008).
37. `TestTokenCacheReusesExistingEntry` — same `(team, user, key_uid)`
    triplet → cached token 재사용 (REQ-008).
38. `TestTokenCacheRefreshesBeforeExpiry` — 만료 60초 전 backgroun
    refresh worker 동작 (REQ-009).
39. `TestTokenCacheSyncOnceUnderConcurrency` — 50 goroutine × 100 호출
    조건에서 같은 (team, user)에 대해 발급은 한 번만 발생 (REQ-009,
    NFR-004).
40. `TestTokenCacheGracefulShutdown` — ctx cancellation honor +
    `goleak.VerifyNone` PASS (REQ-009, NFR-004).
41. `TestTokenCacheRevocationHookPointExists` — `team.member.removed`
    이벤트 hook point 존재 (REQ-009).
42. `TestTokenCacheMetricsEmitted` — `usearch_index_tenant_token_issued_
    total`, `_revoked_total`, `_validation_failures_total` 등록 + 1 증가
    검증 (REQ-009).

**GREEN tasks**:

- `internal/index/tenant/issuer.go` 작성. `meilisearch-go.GenerateTenantToken`
  wrapper.
- `internal/index/tenant/cache.go` 작성. `sync.Map` + per-entry expires_at
  + refresh worker goroutine + `sync.Once` per-cache-miss + revocation hook
  interface (DI seam).
- `internal/index/tenant/types.go` 작성. `TenantTokenEntry`, `TenancyMode`,
  `Visibility` enum.
- `internal/index/meili/client.go::Search` 수정. cache lookup → token 사용.
- `internal/index/meili/korean_shard.go` 수정. `user_id` filterable 추가
  (이미 `team_id`는 등록되어 있음).
- `internal/index/index.go:108` 수정. `FilterableAttributes`에 `user_id`
  추가.
- `internal/obs/metrics/tenant.go` 신설. 4 collector 정의 +
  `registerIDX4` helper.

**REFACTOR**:

- token issuance 캐시 hit/miss 분기를 단일 진입 함수로 통합.
- refresh worker lifecycle을 `Start(ctx) error` / `Shutdown(ctx) error`
  패턴으로 노출 (test에서 goleak 친화).
- revocation hook interface를 `pkg/types`로 격상하여 AUTH-002가 implement
  가능 surface로 만듦.

---

### Phase F — Backfill CLI + Tier Promote + Integration Test

목표: admin CLI 두 개 (`backfill-team`, `tier-promote`) + end-to-end
integration test (testcontainers Qdrant + Meili + PG + cross-team probe).

**RED tests** (16):

43. `TestBackfillCLIDryRunOutputsCounts` — `--dry-run`이 영향 row 수만
    출력 (REQ-011).
44. `TestBackfillCLIBatchUpdatesPostgres` — PG UPDATE batch (LIMIT
    subquery) 동작 (REQ-011).
45. `TestBackfillCLIPatchesQdrantPayload` — Qdrant `set_payload` API 사용
    (REQ-011).
46. `TestBackfillCLIPatchesMeiliDocuments` — Meili `UpdateDocuments`
    partial update (REQ-011).
47. `TestBackfillCLIResumesFromState` — crash 후 `state.json`에서 재개
    (REQ-011, NFR-005).
48. `TestBackfillCLIVerifiesCompletion` — 완료 후 `SELECT count(*) FROM
    docs WHERE team_id IS NULL` == 0 검증 (REQ-011).
49. `TestBackfillCLIMetricsEmitted` — `usearch_index_tenant_backfill_total`
    1 증가 (REQ-011).
50. `TestTierPromoteCreatesDedicatedCollection` — `usearch_docs__team_
    <hash>` 생성 (REQ-005).
51. `TestTierPromoteRoutesUpsertToDedicated` — promote 후 dispatch가
    dedicated collection으로 라우팅 (REQ-005).
52. `TestTierPromoteStreamingMoveCompletes` — default-tier에서 dedicated
    로 point 이동 완료 (REQ-005).
53. `TestTierPromoteDryRunOutputsCountsOnly` — `--dry-run` 동작 (REQ-005).
54. `TestTierPromoteVerifiesEmpty` — `scroll(default, filter=team_id==X)
    .count == 0` 검증 (REQ-005).
55. `TestPublicSentinelRejectedInJWTClaim` — `sub.team_id == "__public__"`
    인증 미들웨어 reject (REQ-007) — 본 SPEC은 hook point + interface만
    노출, AUTH-001이 enforce.
56. `TestPublicSentinelRejectedInEnvVar` — `INDEX_DEFAULT_TEAM=__public__`
    startup validation error (REQ-007).
57. `TestPublicSentinelRejectedInBackfillCLI` — CLI rejects (REQ-007).
58. `TestPublicSentinelRejectedInTierPromoteCLI` — CLI rejects (REQ-007).

**Integration test** (Phase F의 핵심):

59. **`TestCrossTeamIsolationEndToEnd`** — **M6 CROSS-TEAM ISOLATION GATE**
    (NFR-IDX4-001): testcontainers (Qdrant + Meili + PG)에서 team T가
    upsert한 100 doc + team U가 upsert한 100 doc 환경. team T의
    `Search(q={TeamID: "team-T"})`가 team U doc을 0회 반환. team U의
    `Search(q={TeamID: "team-U"})`가 team T doc을 0회 반환. 100회 반복
    + concurrent (50 goroutine).

60. `TestRetrievalLatencyDegradationWithinBudget` — NFR-IDX4-003 (p95
    overhead ≤ 10ms) + NFR-IDX4-006 (p95 degradation ≤ 10%) 측정.
    baseline은 IDX-001 dispatch.go without IDX-004 (`legacy` mode).

**GREEN tasks**:

- `internal/index/backfill/cli.go` + `internal/index/backfill/state.go`
  작성.
- `internal/index/backfill/cli_test.go` 작성.
- `cmd/usearch/admin/backfill.go` 작성. CLI 진입점 + flag parsing.
- `cmd/usearch/admin/tier_promote.go` 작성. CLI 진입점.
- `cmd/usearch/admin/tier_promote_test.go` 작성.
- `cmd/usearch/main.go` 수정. admin sub-command 등록 + startup
  validation.
- `internal/index/tenant_integration_test.go` 작성. M6 cross-team probe
  + latency budget verification.
- `internal/obs/metrics/metrics.go` 수정. `registerIDX4(r)` 호출 추가.
- `internal/obs/metrics/metrics.go:171-176` 수정. cardinality allowlist
  확장 (`team_id_hashed`, `visibility`, `tier`).
- `internal/obs/metrics/metrics_test.go` 수정. `TestNoUnboundedLabels`
  화이트리스트 확장 (NFR-IDX4-007).
- `internal/obs/obs.go` 수정. collector re-export.
- `pkg/types/adapter.go` 수정. `Adapter.Visibility()` 인터페이스 신설.
- `.moai/config/sections/index.yaml` 신설.
- `.env.example` 수정. 신규 env-var 문서화.

**REFACTOR**:

- admin CLI 공통 코드 (flag parsing, state file IO)를 helper로 추출.
- cross-team probe assertion helper.
- tier-promote streaming move의 panic recovery.

---

## 3. Test Catalog Summary

| Phase | Tests Added | REQs Covered | NFRs Covered |
|-------|-------------|--------------|--------------|
| A | 6 | 001, 003 | — |
| B | 6 | 010 | 005 |
| C | 10 | 001, 002, 003 | 008 |
| D | 9 | 004, 006, 007 | 003, 006 |
| E | 11 | 008, 009 | 003, 004 |
| F | 18 | 005, 007, 011 | 001, 002, 003, 005, 006, 007 |
| **Total** | **60** | **11 / 11** | **8 / 8** |

`TestCrossTeamIsolationEndToEnd` (NFR-IDX4-001)는 Phase F의 critical M6
exit-contribution gate다.

---

## 4. Risk Mitigation Table

| Risk | Phase | Mitigation Strategy |
|------|-------|---------------------|
| **R1** Cross-team data leak (catastrophic privacy violation) | Phase F | 4-layer defense (Qdrant payload filter via `is_tenant=true` + Meili tenant token + PG NOT NULL + dispatch.go sentinel `ErrTeamIDRequired`). `TestCrossTeamIsolationEndToEnd` integration test에서 100 doc × 2 team × 50 goroutine 동시 probe. NFR-IDX4-001 강제. IDX-005 ship 전 필수 게이트. |
| **R2** Migration 0004 NOT NULL 전환 도중 in-flight INSERT race | Phase B | single-transaction + `LOCK TABLE docs IN EXCLUSIVE MODE`. 운영자에게 ingest 일시 정지 권장 (operational runbook). 새 INSERT는 backfill 이후 항상 NOT NULL constraint 적용되므로 race 없음. |
| **R3** Meili tenant token 캐시 concurrency 폭주 | Phase E | `sync.Once` per (team, user, key_uid) → 같은 조합 동시 발급은 정확히 한 번. `goleak.VerifyNone` + `-race` 모드 검증. refresh worker는 별도 ctx로 graceful shutdown. NFR-IDX4-004 강제. |
| **R4** AUTH-001/002 ship 지연으로 본 SPEC 단독 운영 시 안전성 | Phase A | `INDEX_DEFAULT_TEAM` env var fallback. visibility 미구현 adapter는 `team_shared` default. cross-team leak 위험은 없음. public 데이터의 cross-team 재사용 효율은 v0.x 손실 (acceptable trade-off). NFR-IDX4-008. |
| **R5** Qdrant `is_tenant=true` payload index가 기존 collection에서 reindex 트리거 | Phase D | Qdrant docs에 따르면 idempotent add. 기존 데이터는 background reindex (수 분 ~ 수 시간 depending on size). 운영 환경에서 maintenance window 권장. `TestEnsureCollectionIdempotentOnExisting`. |
| **R6** backfill CLI 도중 crash → 일부 row만 처리 | Phase F | `state.json`에 `last_processed_doc_id` per-store 기록. 재실행 시 그 지점부터 재개. NFR-IDX4-005 강제. `TestBackfillCLIResumesFromState`. |
| **R7** `__public__` sentinel을 caller가 임의로 사용 | Phase D | 4-entry-point reject (JWT claim / env var / backfill CLI / tier-promote CLI). adapter visibility로만 합법적 사용. `TestPublicSentinelRejected*` 4건. |
| **R8** Tenant token TTL 만료 직전 trafic spike | Phase E | 만료 60초 전 background refresh worker가 새 토큰 미리 발급 → cache hit-rate ≥ 99% 유지 (NFR-IDX4-003). cold-start 첫 호출만 latency 추가 (~30µs HMAC). |
| **R9** AUTH-002 visibility hook 미정의 adapter | Phase E | default `team_shared`. 모든 doc이 same-team 가시성 → cross-team leak 위험 없음. public/user-private의 cross-team 재사용 효율은 AUTH-002 ship 후 활성화. NFR-IDX4-008. |
| **R10** observability 신규 label `team_id_hashed` cardinality 폭증 | Phase F | SHA-256 prefix 8 hex = 4×10^9 가능값. 실제 production 100~10000 팀 안전. `__public__`은 평문 유지. `TestNoUnboundedLabels` 화이트리스트 확장. NFR-IDX4-007. |

---

## 5. MX Tag Plan

본 SPEC의 구현은 다음 @MX 태그를 생성한다.

### 5.1 @MX:ANCHOR (high fan_in, invariant contract)

- `internal/index/tenancy/mode.go::ParseMode`
  — fan_in ≥ 3 (dispatch.go Search, dispatch.go Upsert, cmd/usearch/main.go
  startup validation). tenancy mode 의미가 본 함수에서 정의된다.
- `internal/index/auth/context.go::ExtractTeamID`
  — fan_in ≥ 3 (dispatch.go Search, dispatch.go Upsert, backfill CLI,
  integration tests). context key + env var fallback의 단일 진실
  공급원.
- `internal/index/tenant/cache.go::GetOrIssue`
  — fan_in ≥ 3 (meili/client.go Search, integration tests). token cache
  의 핵심 invariant (sync.Once per triplet).
- `internal/index/dispatch.go::Search`
  — fan_in ≥ 3 (idx5/lookup.go, API handler, MCP handler). multi-tenancy
  enforcement의 hot-path invariant.

### 5.2 @MX:WARN (danger zone, requires @MX:REASON)

- `internal/index/dispatch.go::Upsert::injectTeamIDFromContext`
  — `@MX:WARN`: caller-provided team_id를 silent overwrite. 잘못 구현 시
  cross-team write attack vector. `@MX:REASON`: REQ-IDX4-002 invariant
  (caller가 임의 team_id 주입 차단)의 load-bearing path. WARN slog로
  관측.
- `internal/index/tenant/cache.go::refreshWorker`
  — `@MX:WARN`: background goroutine 스폰. lifecycle 잘못 관리 시 leak.
  `@MX:REASON`: graceful shutdown 필수 (`goleak.VerifyNone`), ctx
  cancellation honor.
- `internal/index/qdrant/client.go::Search::buildFilter`
  — `@MX:WARN`: filter expression 누락 시 cross-team data 노출. `@MX:REASON`:
  NFR-IDX4-001 (0 leak) load-bearing — 모든 query path에서 team_id must
  clause 확정.
- `internal/index/backfill/cli.go::run`
  — `@MX:WARN`: PG `UPDATE` batch 처리 도중 lock 시간 길어지면 in-flight
  ingest blocked. `@MX:REASON`: NFR-IDX4-005 (sub-query LIMIT 패턴 +
  resumable state) 으로 mitigated. operator runbook에 ingest 일시 정지
  권장.

### 5.3 @MX:NOTE (context & intent delivery)

- `internal/index/tenancy/mode.go` 파일 상단에 3-mode 의미 + 전환 정책
  (restart 필수) 기록.
- `internal/index/auth/context.go::ExtractTeamID` 에 DEEP-004 convention
  align rationale 기록.
- `internal/index/tenant/issuer.go::GenerateToken` 에 Meili admin key UID
  vs admin API key 구분 기록 (혼동 방지).
- `internal/index/qdrant/client.go::EnsureCollection` 에 `is_tenant=true`
  payload index의 co-location 효과 + HNSW `payload_m` 미적용 결정 기록.

### 5.4 @MX:TODO (incomplete work — resolved in GREEN phase)

- RED phase에서 placeholder 함수에 `@MX:TODO`를 부착하고 GREEN phase
  종료 시 모두 제거.
- AUTH-002 visibility hook은 v1.0 default `team_shared`이므로 wire-up
  추가 시점에 `@MX:TODO` 제거 (post-IDX-004 ship).

---

## 6. File Touch Order (recommended TDD progression)

1. **Phase A start**: `internal/index/tenancy/mode.go` →
   `internal/index/auth/context.go` → tests → REFACTOR.
2. **Phase B**: `deploy/postgres/migrations/0004_team_id_not_null.sql` +
   `0005_user_id_column.sql` → `internal/index/pg/migrations_test.go`
   (testcontainers) → 6 tests.
3. **Phase C**: `internal/index/dispatch.go` (Search/Upsert 진입점) →
   `internal/index/pg/client.go` (INSERT/SELECT 경로) → 10 tests.
4. **Phase D**: `internal/index/qdrant/client.go::EnsureCollection` →
   `::Search::buildFilter` → 9 tests.
5. **Phase E**: `internal/index/tenant/types.go` →
   `internal/index/tenant/issuer.go` → `internal/index/tenant/cache.go`
   → `internal/index/meili/client.go::Search` →
   `internal/index/meili/korean_shard.go` (user_id 추가) →
   `internal/index/index.go:108` (FilterableAttributes 확장) →
   `internal/obs/metrics/tenant.go` → 11 tests.
6. **Phase F**: `internal/index/backfill/state.go` →
   `internal/index/backfill/cli.go` →
   `cmd/usearch/admin/backfill.go` →
   `cmd/usearch/admin/tier_promote.go` →
   `cmd/usearch/main.go` (sub-command 등록) →
   `internal/obs/metrics/metrics.go` (registerIDX4 + allowlist) →
   `internal/obs/obs.go` (re-export) →
   `internal/obs/metrics/metrics_test.go` (allowlist 확장) →
   `pkg/types/adapter.go` (Visibility 인터페이스) →
   `.moai/config/sections/index.yaml` →
   `.env.example` →
   `internal/index/tenant_integration_test.go` (M6 cross-team probe +
   latency budget) → 18 tests.

---

## 7. Coverage and Quality Gates

- Coverage 목표: 85% (per `quality.yaml`).
- 새 packages 측정: `internal/index/tenancy/`, `internal/index/auth/`,
  `internal/index/tenant/`, `internal/index/backfill/`,
  `cmd/usearch/admin/`. 기존 `internal/index/` 의 수정 부분도 회귀
  coverage 유지.
- TRUST 5 gates: 모든 phase 종료 시점에 `go vet` + `golangci-lint` +
  `go test -race` 통과.
- Cardinality test: `internal/obs/metrics/metrics_test.go::TestNoUnboundedLabels`
  화이트리스트에 신규 label (`team_id_hashed`, `visibility`, `tier`) 추가
  후 통과.
- LSP gate: zero errors / zero type errors / zero lint errors.
- **M6 cross-team isolation gate**: `TestCrossTeamIsolationEndToEnd`
  PASS (Phase F의 critical integration test).

---

## 8. Pre-submission Self-Review

전체 changeset이 완성된 시점에 다음을 확인한다:

- `dispatch.go::Search`/`Upsert` 진입 첫 단계가 tenancy sentinel인가?
  (embedder/store fanout 모두 그 이후에 호출되는가?)
- caller-provided `Metadata["team_id"]`가 정말 silent overwrite되며 WARN
  slog가 emit되는가?
- v0.1의 `"team_id": nil` 박힌 line이 완전히 제거되었는가?
- Meili tenant token이 두 인덱스 (`usearch_docs`, `usearch_docs_ko`) 모두
  에 적용되는가?
- token cache가 같은 (team, user, key_uid) triplet에 대해 정확히 한 번
  발급하는가? (`sync.Once` 검증)
- `__public__` sentinel이 4개 입력 경로에서 모두 거절되는가?
- PG migration 0004 + 0005가 idempotent하게 두 번 적용 가능한가?
- backfill CLI가 `--dry-run` + 실제 실행 + `state.json` resume 모두
  정상 동작하는가?
- cardinality 화이트리스트 확장이 `TestNoUnboundedLabels`에서 통과하는가?
- cross-team isolation integration test가 100 doc × 2 team × 50 goroutine
  concurrent 조건에서 0 leak 검증하는가?
- AUTH-001/002 ship 이전 운영 (env var fallback + default visibility)
  에서 안전한가?

---

## 9. Implementation Sequencing Across Sessions

본 SPEC의 6개 phase는 sequential 의존성을 가진다 (Phase C는 Phase A/B의
모듈을 참조 등). 단일 manager-tdd 세션으로 완주가 어려운 경우 다음 세션
분할이 권장된다:

- **Session 1**: Phase A + B (context/mode foundation + PG migration)
- **Session 2**: Phase C + D (dispatch enforcement + Qdrant Tiered
  Multitenancy)
- **Session 3**: Phase E + F (Meili tenant tokens + admin CLI + integration
  test)

각 세션 시작 시 `/clear` 후 본 plan.md + spec-compact.md만 재로드하여
컨텍스트를 보존한다.

---

## 10. M6 Release Gate Checklist (IDX-004's contribution)

Phase F 완료 직후 다음을 차례로 확인한다:

- [ ] `TestCrossTeamIsolationEndToEnd` PASS (NFR-IDX4-001, M6 enabling
  invariant)
- [ ] `TestRetrievalLatencyDegradationWithinBudget` PASS (NFR-IDX4-003 +
  NFR-IDX4-006)
- [ ] cardinality allowlist `TestNoUnboundedLabels` PASS (NFR-IDX4-007)
- [ ] PG migration 0004 + 0005 idempotent + backfill 정확 (REQ-010)
- [ ] backfill CLI dry-run + 실제 실행 + resume 정상 (REQ-011)
- [ ] Meili tenant token 두 인덱스 적용 + cache concurrency safe
  (REQ-008/009, NFR-004)
- [ ] `__public__` sentinel 4-entry-point reject (REQ-007)
- [ ] AUTH-001/002 미ship 환경에서 env var fallback 동작 (NFR-008)
- [ ] M6 IDX-005 (downstream consumer)가 본 SPEC의 multi-tenancy invariant
  를 trust 가능한 상태

위 모두 PASS 시 IDX-005 implementation 진입 가능 → IDX-005의 `TestDedupHitRate
At30PctOnSyntheticTraffic` (M6 PRIMARY GATE)가 측정 가능 상태에 진입.

---

*End of SPEC-IDX-004 plan.*
