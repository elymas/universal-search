# SPEC-IDX-005 Implementation Plan

Generated: 2026-05-22
Methodology: TDD (RED-GREEN-REFACTOR)
Coverage target: 85%
Harness: standard

---

## 1. Overview

본 plan.md는 SPEC-IDX-005의 구현 단계별 task sequence를 정의한다. M6
release-driving deliverable로서 본 SPEC은 SPEC-IDX-001 / IDX-004 / AUTH-001
/ AUTH-002 가 모두 implemented 상태일 때 시작 가능하다. 10 EARS REQs +
7 NFRs를 5개 phase로 분해하며 각 phase는 RED → GREEN → REFACTOR 사이클을
따른다.

본 SPEC은 **M6 exit criterion의 PRIMARY DRIVER**다. plan-auditor 통과
후 manager-tdd가 phase별로 진행하며, Phase E의 dedup hit-rate ≥30%
integration test가 final M6 GA gate다.

---

## 2. Phase Breakdown

### Phase A — Storage Foundation + Schema

목표: durable storage 계층(PG `answer_cache` 테이블 + Qdrant payload
schema 확장)을 먼저 구축한다.
순서 근거: hot-path 코드가 이 계층에 의존하므로 가장 먼저 RED 테스트로
고정.

**RED tests** (5):

1. `TestMigration0003Idempotent` — `0003_answer_cache.sql` 두 번 실행해도
   같은 schema 도달 (REQ-006).
2. `TestAnswerCacheSchemaMatchesSpec` — pgx로 information_schema 쿼리해
   컬럼 집합·타입·제약 정확히 검증 (REQ-006).
3. `TestAnswerCacheIndexesExist` — `idx_answer_cache_team_created`,
   `idx_answer_cache_team_category`, `idx_answer_cache_force_stale` 인덱스
   존재 검증 (REQ-006).
4. `TestDocIDIncludesTeamId` — `docID("answer-cache", queryHash + ":" + team_id)`
   가 same query 다른 team에서 다른 doc_id 반환 (REQ-007).
5. `TestDocTypeCachedAnswerEnumAdded` — `pkg/types.DocType`에
   `DocTypeCachedAnswer = "cached_answer"` 추가됨 검증 (REQ-006).

**GREEN tasks**:

- `deploy/postgres/migrations/0003_answer_cache.sql` 작성 (idempotent
  `CREATE TABLE IF NOT EXISTS` + 3개 인덱스).
- `pkg/types/normalized_doc.go`에 `DocTypeCachedAnswer` enum 추가.
- `internal/idx5/types.go`에 `CachedAnswer` struct + `Staleness` enum +
  `LookupResult` 정의.
- doc_id 헬퍼 함수 `internal/idx5/docid.go` 작성 (IDX-001 `docID` 호출).

**REFACTOR**:

- enum 값 검증 helper 통합, types.go에 JSON marshal 헬퍼 추출.

---

### Phase B — Lookup Pipeline Core

목표: embedder + Qdrant similarity search + threshold/staleness evaluation을
단독 모듈로 완성한다.

**RED tests** (6):

6. `TestLookupCallsEmbedderOnce` — embedder.Embed가 정확히 1회 호출
   (REQ-001).
7. `TestLookupCallsIndexSearchWithTeamScopedFilter` — `IndexQuery.TeamID`,
   `DocTypes: [DocTypeCachedAnswer]`, `MaxResults: 1` 정확히 설정
   (REQ-001).
8. `TestLookupHitAboveThresholdServesCached` — score 0.95 + fresh →
   cached_answer 반환 (REQ-002).
9. `TestLookupMissBelowThresholdFallsThrough` — score 0.89 → MISS 반환
   (REQ-002).
10. `TestLookupPerTeamThresholdOverride` — deep.yaml team_overrides가
    적용됨 (REQ-002).
11. `TestStalenessFresh` / `TestStalenessSoftStale` / `TestStalenessHardStale`
    / `TestStalenessForceStaleOverride` / `TestStalenessPerCategoryTTL` —
    5개 분기 covered (REQ-003).

**GREEN tasks**:

- `internal/idx5/lookup.go` 작성. embedder + Qdrant search + threshold
  eval 합성.
- `internal/idx5/staleness.go` 작성. category → TTL lookup + age 평가
  + force_stale 처리.
- `internal/idx5/config.go` 일부 작성 (similarity_threshold, ttl map,
  team_overrides 로딩만).

**REFACTOR**:

- staleness eval 함수의 boundary condition test 추가 (age == TTL boundary).
- threshold override 로직을 pure function으로 추출하여 단위 테스트.

---

### Phase C — Hit Serve + Cache Write-Back

목표: cached answer을 SYN-001 response shape로 reconstruct하여 serve하고,
fanout MISS 경로에서 async write-back을 실행한다.

**RED tests** (7):

12. `TestServeReconstructsSynthesizeResponse` — cached_answer record →
    SYN-001 SynthesizeResponse JSON (REQ-002).
13. `TestLookupResponseHeadersOnHit` — `X-Cache: HIT`, `X-Cache-Age-Seconds`,
    `X-Cache-Score` headers 부착 (REQ-002).
14. `TestServeSoftStaleSetsSoftHitHeader` — soft-stale은 `X-Cache: SOFT-HIT`
    (REQ-002).
15. `TestCacheWriteOnMissFireAndForget` — fanout MISS 후 Qdrant + PG에
    write가 async로 발생 (REQ-006).
16. `TestCacheWriteFailureDoesNotImpactResponse` — write 실패해도 response
    latency 영향 없음 (REQ-006).
17. `TestCacheWriteUpsertOverwritesIdempotent` — same doc_id 재 write는
    overwrite (REQ-006).
18. `TestHitCountIncrementsOnServe` / `TestLastServedAtUpdatesOnServe` —
    PG hit_count UPDATE async (REQ-005).

**GREEN tasks**:

- `internal/idx5/serve.go` 작성. cached_answer → SynthesizeResponse JSON
  reconstruct + response header 부착.
- `internal/idx5/writeback.go` 작성. fire-and-forget goroutine으로
  Qdrant upsert + PG insert.
- `internal/idx5/refresh_job.go` 작성 (soft-stale enqueue 부분만; worker는
  Phase E).

**REFACTOR**:

- writeback goroutine의 panic recovery + slog WARN 통합.
- response header 생성을 helper로 추출.

---

### Phase D — Citation Re-Validation + Feedback Handler

목표: eager citation re-validation 모드 (CACHE-001 Phase 2 재사용) +
feedback thumbs-down handler를 구현한다.

**RED tests** (6):

19. `TestCitationRevalidationLazyDefault` — default 모드는 citation 그대로
    serve (REQ-004).
20. `TestCitationRevalidationEagerTopNStrips404` — 404 citation strip +
    `X-Cache-Citation-Stale: 1` header (REQ-004).
21. `TestCitationRevalidationEagerTopNKeepsTimeout` — timeout citation은
    유지 (REQ-004).
22. `TestCitationRevalidationReusesPhase2HeadProbe` — CACHE-001
    `internal/access/phase2_probe.go::headProbe` 재사용 (REQ-004).
23. `TestFeedbackMarksForceStale` — `POST /feedback {score: -1}` →
    `answer_cache.force_stale = TRUE` UPDATE (REQ-008).
24. `TestFeedbackIdempotentOnDuplicate` — 같은 (team, user, doc_id) 24h
    내 중복 호출 무해 (REQ-008).
25. `TestFeedbackUnmappedIncrementsCounter` — 24h LRU 만료된 request_id →
    counter 1 증가 (REQ-008).
26. `TestFeedbackRespectsTenantBoundary` — team U가 team T의 doc_id로
    feedback POST 시도 → 무시 (REQ-008, NFR-004).

**GREEN tasks**:

- `internal/idx5/citation_revalidate.go` 작성. CACHE-001 phase2 헤더
  추출 → internal API로 expose.
- `internal/idx5/feedback.go` 작성. `POST /feedback` handler + LRU
  + force_stale UPDATE.

**REFACTOR**:

- LRU 구현은 stdlib `container/list` + sync.Mutex 또는 external lib?
  decision: stdlib 충분 (24h TTL, 추정 100k entries × ~100 bytes = ~10MB
  in-process).
- citation strip 로직을 pure function으로 추출 (citations slice in →
  filtered slice out).

---

### Phase E — Middleware Wiring + Observability + M6 Exit Test

목표: 모든 모듈을 chi v5 middleware로 결합하고, OTel + slog + Prometheus
observability를 부착하며, M6 exit criterion synthetic test를 실행한다.

**RED tests** (9):

27. `TestLookupSkipsOnForceRefreshQueryParam` — `?force_refresh=true` →
    fanout 직진 (REQ-001).
28. `TestLookupSkipsOnForceRefreshHeader` — `X-Force-Refresh: 1` → fanout
    직진 (REQ-001).
29. `TestSoftStaleEnqueuesRefreshJob` — soft-stale serve 후 Asynq job
    enqueued (REQ-005).
30. `TestRefreshJobUpsertsOverSameDocId` — worker 실행 시 same doc_id로
    overwrite (REQ-005, NFR-006).
31. `TestRefreshJobFailureDoesNotImpactResponse` — worker 실패해도 다음
    lookup에서 hard-stale로 처리 (REQ-005).
32. `TestCrossTenantLookupReturnsZeroResults` — team U가 team T cached
    answer를 reuse 시도 → MISS 강제 (REQ-007, NFR-004).
33. `TestMetricsRegisteredWithCorrectNames` — 7 collectors 정확한 namespace
    (REQ-009).
34. `TestDedupHitRateGaugeBoundedByWhitelist` — `team_id_hashed` 화이트리스트
    외 값은 `unknown`으로 collapse (REQ-009).
35. `TestObservabilitySafeOnNilObs` — nil obs로 미들웨어 호출 시 panic
    NO (REQ-009).
36. `TestOTelSpanAttributesEmitted` / `TestSlogJSONLineSchemaComplete` /
    `TestSlogLineRequestIdPropagated` (REQ-010).
37. `TestConfigHotReloadOnSIGHUP` — deep.yaml 수정 후 SIGHUP → 다음 요청
    부터 새 값 적용 (NFR-007).
38. **`TestDedupHitRateAt30PctOnSyntheticTraffic`** — **M6 EXIT GATE**:
    synthetic 트래픽 100 query (그 중 35건은 같은 사용자 reformulation)을
    integration test (httptest + testcontainers Qdrant + PG + Redis)로
    실행하여 `usearch_idx5_lookups_total{outcome="hit"} + {outcome="soft_hit"}` /
    `total ≥ 0.30` 검증 (NFR-001).

**GREEN tasks**:

- `internal/idx5/middleware.go` 작성. lookup orchestration + force-refresh
  + serve / fall-through 분기.
- `internal/obs/metrics/idx5.go` 작성. 7개 collector + registerIDX5
  helper.
- `internal/obs/metrics/metrics.go` 수정. registerIDX5 호출 추가.
- `internal/obs/obs.go` 수정. collector re-export.
- `internal/obs/metrics/metrics_test.go` 수정. `TestNoUnboundedLabels`
  화이트리스트에 `team_id_hashed`, 새 outcome enum values 추가.
- `cmd/usearch-api/handlers/query.go` (or 동등 핸들러) 수정. middleware
  chain에 wire.
- `cmd/usearch-api/main.go` 수정. `idx5.New(...)` 초기화 + refresh worker
  + scheduled hooks.
- `internal/idx5/refresh_job.go` worker 완성.
- `internal/idx5/integration_test.go` 작성. M6 exit gate synthetic test.

**REFACTOR**:

- middleware 체인 순서 검증 helper (Identity → AuthZ → IDX-005 lookup
  → fanout).
- OTel span attribute 부착 helper.
- decision event log JSON line schema struct에 centralize (additive-only
  규칙을 코드 주석에 명시).

---

## 3. Test Catalog Summary

| Phase | Tests Added | REQs Covered | NFRs Covered |
|-------|-------------|--------------|--------------|
| A | 5 | 006, 007 | — |
| B | 6 | 001, 002, 003 | 002 |
| C | 7 | 002, 005, 006 | 003 |
| D | 8 | 004, 008 | 004 |
| E | 12 | 001, 005, 007, 009, 010 | 001, 004, 005, 006, 007 |
| **Total** | **38** | **10 / 10** | **7 / 7** |

추가로 REQ-IDX5-009의 cardinality allowlist 확장은 Phase E의 metrics_test.go
수정으로 cover된다.

---

## 4. Risk Mitigation Table

| Risk | Phase | Mitigation Strategy |
|------|-------|---------------------|
| **R1** Cross-tenant cache leak (catastrophic privacy violation) | Phase E | 4-layer defense (IDX-004 Qdrant filter + IDX-004 PG RLS + doc_id team_id 포함 + acceptance test). `TestCrossTenantLookupReturnsZeroResults` + integration test에서 cross-tenant probe. NFR-IDX5-004 강제. |
| **R2** Similarity threshold mismatch (false hits) | Phase B | 0.92 default empirical. per-team override. hit rate가 30% 미달 시 알람. 운영 30일 후 P95 측정으로 재조정. |
| **R3** Soft-stale refresh job thundering herd | Phase E | 같은 (team, query)에 대한 동시 refresh는 IDX-001 ON CONFLICT (doc_id) DO UPDATE로 idempotent. NFR-006 race-safety 검증. |
| **R4** Citation 404 misleading user (lazy mode) | Phase D | document staleness는 TTL로 간접 mitigate. eager mode opt-in 제공. Future SPEC-SYN-002 amendment로 mid-content invalidation 가능. |
| **R5** Feedback loop poisoning (malicious thumbs-down) | Phase D | V1은 (team, user, doc_id) idempotent. multi-user threshold tuning은 후속 SPEC. cross-tenant probe로 boundary 강제. |
| **R6** Embedder latency degradation cascades to /query path | Phase B | NFR-002 (overhead p95 ≤ 50ms) 강제. embedder timeout 200ms로 budget cap. miss는 fanout으로 fall-through (lookup latency 추가만 발생, hit가 안 되는 정상 path). |
| **R7** Asynq refresh worker outage (soft-stale forever) | Phase E | worker 실패 시 다음 lookup에서 hard-stale로 평가 → 자동 fallback. Asynq worker는 별도 health-check + replica로 운영(M6 deployment SPEC). |
| **R8** PG `answer_cache` 테이블 크기 폭증 | Phase A | per-team retention 정책: 90일 hot in PG. archival은 M8 SPEC-AUDIT-002. V1은 NFR 외부. 90일 자동 DELETE는 Asynq scheduled job (deferred SPEC). |
| **R9** Hot-reload config inconsistency mid-request | Phase E | hot-reload는 다음 request부터 적용. 진행 중인 request는 이전 값 사용. SIGHUP 처리는 DEEP-004 패턴 재사용. |
| **R10** M6 exit gate (≥30% dedup hit rate) 미달 | Phase E | `TestDedupHitRateAt30PctOnSyntheticTraffic` 통과 필수. 실패 시 threshold 또는 TTL 조정 + 재시도. integration test 자체가 GA gate. |

---

## 5. MX Tag Plan

본 SPEC의 구현은 다음 @MX 태그를 생성한다.

### 5.1 @MX:ANCHOR (high fan_in, invariant contract)

- `internal/idx5/middleware.go::LookupMiddleware`
  — fan_in ≥ 3 (query.go, MCP handler, integration tests). lookup
  middleware의 동작은 모든 `/query` 호출의 invariant.
- `internal/idx5/lookup.go::Lookup`
  — fan_in ≥ 3 (middleware, refresh job worker, integration tests).
  similarity threshold + staleness 평가의 의미가 본 함수에서 정의된다.
- `internal/idx5/writeback.go::WriteCachedAnswer`
  — fan_in ≥ 3 (MISS 경로 handler, refresh worker). 모든 cached answer
  write가 이 함수를 거친다.
- `internal/idx5/feedback.go::HandleFeedback`
  — fan_in ≥ 3 (UI feedback endpoint, CLI feedback, future MCP feedback).
  feedback의 의미 invariant.

### 5.2 @MX:WARN (danger zone, requires @MX:REASON)

- `internal/idx5/middleware.go::handleHit`
  — `@MX:WARN`: hit serve 시 fanout을 완전히 skip하기 때문에 cross-tenant
  격리가 깨지면 즉시 데이터 유출. `@MX:REASON`: NFR-IDX5-004 invariant
  (정확히 0 leak)의 load-bearing path.
- `internal/idx5/writeback.go::fireAndForget`
  — `@MX:WARN`: 비동기 goroutine 스폰. 잘못된 lifecycle은 goroutine leak
  발생. `@MX:REASON`: 응답 latency 영향 없음을 보장하면서 panic-safe해야
  함.
- `internal/idx5/citation_revalidate.go::eagerProbeTopN`
  — `@MX:WARN`: parallel HEAD probe spawn. timeout/cancel 없으면 reuse
  latency 폭증. `@MX:REASON`: per-probe ctx timeout + 부모 ctx 전파가
  필수.
- `internal/idx5/refresh_job.go::Worker`
  — `@MX:WARN`: 같은 (team, query) 조합에 대한 동시 refresh가 발생할 수
  있다. `@MX:REASON`: IDX-001 ON CONFLICT (doc_id) DO UPDATE의 race-safety
  를 trust.

### 5.3 @MX:NOTE (context & intent delivery)

- `internal/idx5/middleware.go` 파일 상단에 lookup → fanout 분기 design
  rationale 기록.
- `internal/idx5/staleness.go::Evaluate`에 per-category TTL의 freshness
  trade-off 기록.
- `internal/idx5/lookup.go::computeDocID`에 doc_id 구성에 team_id 포함
  결정의 보안적 의미 기록.

### 5.4 @MX:TODO (incomplete work — resolved in GREEN phase)

- RED phase에서 placeholder 함수에 `@MX:TODO`를 부착하고 GREEN phase
  종료 시 모두 제거.

---

## 6. File Touch Order (recommended TDD progression)

1. **Phase A start**: `deploy/postgres/migrations/0003_answer_cache.sql` →
   `pkg/types/normalized_doc.go` (DocTypeCachedAnswer 추가) →
   `internal/idx5/types.go` → 5 tests.
2. **Phase B**: `internal/idx5/lookup.go` → `internal/idx5/staleness.go`
   → `internal/idx5/config.go` (일부) → 6 tests.
3. **Phase C**: `internal/idx5/serve.go` → `internal/idx5/writeback.go`
   → 7 tests.
4. **Phase D**: `internal/idx5/citation_revalidate.go` →
   `internal/idx5/feedback.go` → 6 tests.
5. **Phase E**: `internal/idx5/middleware.go` →
   `internal/idx5/refresh_job.go` worker 완성 →
   `internal/obs/metrics/idx5.go` → metrics.go / obs.go / metrics_test.go
   수정 → `cmd/usearch-api/handlers/query.go` 수정 →
   `cmd/usearch-api/main.go` 수정 → `internal/idx5/integration_test.go`
   작성 (M6 exit gate test) → 12 tests.

---

## 7. Coverage and Quality Gates

- Coverage 목표: 85% (per `quality.yaml`).
- 새 package `internal/idx5/`만 측정.
- TRUST 5 gates: 모든 phase 종료 시점에 `go vet` + `golangci-lint` +
  `go test -race` 통과.
- Cardinality test: `internal/obs/metrics/metrics_test.go::TestNoUnboundedLabels`
  화이트리스트에 신규 label 추가 후 통과.
- LSP gate: zero errors / zero type errors / zero lint errors.
- **M6 GA gate**: `TestDedupHitRateAt30PctOnSyntheticTraffic` PASS
  (Phase E의 마지막 RED → GREEN 사이클).

---

## 8. Pre-submission Self-Review

전체 changeset이 완성된 시점에 다음을 확인한다:

- middleware 체인 순서가 §5.1의 design rationale을 반영하는가?
  (Identity → AuthZ → IDX-005 lookup → fanout)
- doc_id 구성에 team_id가 정확히 포함되어 cross-tenant collision이
  차단되는가?
- soft-stale serve가 동기 응답 latency를 증가시키지 않는가?
- citation re-validation eager 모드가 CACHE-001 phase2 코드를 재사용하는가
  (코드 중복 없음)?
- `answer_cache.force_stale` flag가 lookup의 hard-stale 분기에 반영되는가?
- Prometheus 메트릭이 SPEC-OBS-001 명명 규칙(`usearch_idx5_*`)을 따르는가?
- `team_id_hashed` label이 화이트리스트로 bounded되어 `unknown` collapse가
  작동하는가?
- M6 exit gate test가 30% threshold를 정확히 측정하는가?

---

## 9. Implementation Sequencing Across Sessions

본 SPEC의 5개 phase는 sequential 의존성을 가진다 (Phase B는 Phase A의
schema를 참조 등). 단일 manager-tdd 세션으로 완주가 어려운 경우 다음 세션
분할이 권장된다:

- **Session 1**: Phase A + B (storage foundation + lookup core)
- **Session 2**: Phase C + D (hit serve + write-back + citation + feedback)
- **Session 3**: Phase E (middleware wiring + observability + M6 exit
  gate test)

각 세션 시작 시 `/clear` 후 본 plan.md + spec-compact.md만 재로드하여
컨텍스트를 보존한다.

---

## 10. M6 Release Gate Checklist

Phase E 완료 직후 다음을 차례로 확인한다:

- [ ] `TestDedupHitRateAt30PctOnSyntheticTraffic` PASS
- [ ] cross-tenant leak probe (acceptance §5.6) PASS
- [ ] IDX-004 multi-tenancy 강제가 정상 작동 (4-layer defense 모두 active)
- [ ] AUTH-001 JWT context propagation이 미들웨어에서 read 가능
- [ ] AUTH-002 RBAC가 lookup endpoint 진입 시 enforce됨
- [ ] Prometheus dashboard에 `usearch_idx5_dedup_hit_rate` 가시화 가능
- [ ] config hot-reload가 SIGHUP에 정상 반응
- [ ] M6 release candidate tag 준비

위 모두 PASS 시 M6 GA 진입 가능.

---

*End of SPEC-IDX-005 plan.*
