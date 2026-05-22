---
id: SPEC-IDX-005
version: 0.1.0
status: implemented
created: 2026-05-22
updated: 2026-05-22
author: limbowl
priority: P1
issue_number: null
title: Team-shared answer reuse via pre-fanout lookup with configurable staleness
milestone: M6 — Team plane
owner: expert-backend
methodology: tdd
coverage_target: 85
depends_on: [SPEC-IDX-001, SPEC-IDX-004, SPEC-AUTH-001, SPEC-AUTH-002, SPEC-CACHE-001, SPEC-SYN-001, SPEC-FAN-001]
blocks: []
---

# SPEC-IDX-005: Team-shared answer reuse via pre-fanout lookup with configurable staleness

## HISTORY

- 2026-05-22 (initial draft v0.1.0, limbowl via manager-spec):
  First EARS-formatted SPEC for the M6 release-driving deliverable. M6 exit
  criterion("shared index dedup hits ≥30%")의 PRIMARY DRIVER로서 IDX-005는
  AUTH-001 / AUTH-002 / IDX-004의 multi-tenancy 강제 위에 build되어 pre-fanout
  team-scoped answer cache lookup 계층을 추가한다.

  본 SPEC은 SPEC-IDX-001(implemented)이 정의한 `*index.Index.Search` /
  `Upsert` 공개 surface를 reuse하며, SPEC-FAN-001(implemented)의 `Dispatch`
  call site 직전에 chi v5 middleware로 inject된다. cache hit 시 fanout이
  완전히 bypass되어 synthesis까지 skip되며, SYN-001 response shape를 reuse한
  serve를 통해 downstream consumer는 차이를 인지하지 못한다.

  10 핵심 결정사항(research §7)은 §1.1에 재명시되어 본 SPEC의 EARS 요구사항
  으로 번역되며 재논의되지 않는다.

  Pinned decisions:
  (D1) Similarity threshold default 0.92. deep.yaml hot-reload로 운영 중
       조정 가능하며 per-team override 지원.
  (D2) Per-category staleness TTL via IR-001 category. web/social=짧음,
       academic=긺. mixed/unknown는 default 적용.
  (D3) Hard-stale은 lazy evict. 다음 write가 idempotent하게 overwrite.
       별도 evict job 없음.
  (D4) Citation re-validation default LAZY. eager_top_n / eager_all 모드는
       deploy 시점 opt-in.
  (D5) 사용자 thumbs-down 1회 → 즉시 stale. multi-user threshold tuning은
       SPEC-EVAL-003(M9) 후속.
  (D6) Force-refresh 우회는 `?force_refresh=true` query param OR
       `X-Force-Refresh: 1` header. metric으로 추적.
  (D7) Hit serve는 SYN-001 SynthesizeResponse shape 그대로. 추가 response
       header(X-Cache, X-Cache-Age-Seconds, X-Cache-Score)로만 차이 노출.
  (D8) cached answer의 Qdrant 저장은 기존 `usearch_docs` collection 재사용,
       `doc_type="cached_answer"` payload tag로 disambiguate.
  (D9) PG 저장은 새 `answer_cache` 테이블. migration
       `deploy/postgres/migrations/0003_answer_cache.sql`.
  (D10) V1은 Redis hot lookup 도입 NOT. Qdrant similarity search latency가
        충분하며 V2 측정 후 결정.

  M6의 마지막 P1 SPEC으로서 본 SPEC은 별도 GitHub Issue로 트랙되지 않으며
  (`issue_number: null`) plan-auditor 통과 후 status `draft → approved` 전이.

  Companion artifacts:
  - `.moai/specs/SPEC-IDX-005/research.md` — Phase 0.5 research
    (~520 lines, 9 sections — existing surface, lookup pipeline, threshold
    justification, TTL model, storage schema, observability, threat model,
    NFR targets, 10 pinned decisions, 8 open questions)
  - `.moai/specs/SPEC-IDX-005/plan.md` — TDD 5-phase task sequence
  - `.moai/specs/SPEC-IDX-005/tasks.md` — task decomposition
  - `.moai/specs/SPEC-IDX-005/acceptance.md` — Given/When/Then 시나리오
    (8 main + 2 boundary edge)
  - `.moai/specs/SPEC-IDX-005/spec-compact.md` — compact view
  - `.moai/specs/SPEC-IDX-005/progress.md` — progress tracker

  10 EARS REQs (8 × P0 + 2 × P1), 7 NFRs, 4 modules (Lookup Pipeline /
  Staleness & Eviction / Citation Re-Validation / Observability & Feedback).
  Methodology: TDD, coverage target 85%, harness: standard. Owner:
  expert-backend.

---

## 1. Overview

본 SPEC은 M6 milestone의 **release-driving deliverable**이자 `/query` 핫
경로의 **pre-fanout team-scoped lookup 계층**을 정의한다. SPEC-FAN-001의
multi-source fanout dispatch는 평균 1.5~3초의 wall-clock + N개 외부
adapter API 호출 비용을 소비한다. 같은 team의 다른 사용자가 사실상 동일한
query를 30분 이내에 다시 요청할 경우 fanout을 완전히 우회하고 저장된 합성
답변을 즉시 serve하여:

1. **운영 비용 절감**: adapter call 0회, LLM synthesis 0회 → $0.07 / 호출 → $0
2. **사용자 응답 latency 절감**: p95 ~5s → ~200ms
3. **외부 API rate-limit 보호**: 트래픽 폭주 시 adapter pool exhaustion 회피

이 효과는 M6 exit criterion("shared index dedup hits ≥30%")으로 SLO화되며
본 SPEC이 직접 측정·강제한다.

### 1.1 Pinned Architectural Decisions

다음 10개 결정은 research §7에서 context-derived로 확정되었다. 본 SPEC은
이를 EARS 요구사항으로 번역할 뿐 재논의하지 않는다.

1. **Similarity threshold**: cosine 0.92 default. 측정된 BGE-M3 reformulation
   cutoff. deep.yaml `costguard.idx5.similarity_threshold`로 hot-reload
   가능. per-team override 지원.
2. **Per-category staleness TTL**: IR-001 category를 키로 web=1h, social=30m,
   academic=30d, korean=1h, mixed=2h, unknown=2h. deep.yaml
   `costguard.idx5.staleness_ttl_by_category` map.
3. **Soft-stale vs hard-stale**: `created_at + 0.5*TTL` 시점부터 soft-stale,
   `created_at + TTL` 시점부터 hard-stale. soft는 SERVE + 비동기 refresh,
   hard는 MISS 처리. hard-stale evict는 lazy(다음 write가 overwrite).
4. **Citation re-validation policy**: default LAZY (재검증 없이 serve).
   `eager_top_n`, `eager_all` 모드는 deploy 시점 opt-in. eager는 CACHE-001
   Phase 2 HEAD probe 재사용.
5. **Feedback loop**: 사용자 thumbs-down 1회 → `force_stale=TRUE` marking →
   다음 lookup에서 hard-stale로 평가. multi-user threshold는 후속 SPEC.
6. **Force-refresh bypass**: `?force_refresh=true` query param 또는
   `X-Force-Refresh: 1` header. lookup 전체 skip, fanout 직진. metric으로
   추적.
7. **Hit serve shape**: SYN-001 `SynthesizeResponse` JSON shape 그대로.
   추가 response header(`X-Cache: HIT|MISS|BYPASSED`, `X-Cache-Age-Seconds`,
   `X-Cache-Score`)로만 차이 노출. downstream consumer는 코드 변경 없이
   동작.
8. **Qdrant storage**: 기존 `usearch_docs` collection 재사용. payload
   `doc_type = "cached_answer"`로 disambiguate. 별도 collection 미생성.
9. **PG storage**: 새 `answer_cache` 테이블. migration `0003_answer_cache.sql`.
   `docs` 테이블 schema는 변경 없음.
10. **Redis hot lookup**: V1 미도입. Qdrant similarity search latency
    (≤100ms p50, NFR-IDX-002)가 충분. V2 측정 후 결정.

### 1.2 Motivation

`/query` fanout 경로는 평균-경우 fanout 1.5s + IDX-001 retrieval 100ms
+ synthesis 1.5s = ~3.1s, 최악-경우 ~8s까지 latency를 소비한다. 사용자
질의에는 **반복성 패턴**이 강하다(자체 telemetry 가정: 동일 team 내 24시간
window에 30% 이상의 query가 유사 reformulation). 본 SPEC이 ship되기 전에는
이 반복 트래픽 전체가 fanout pipeline을 통과한다.

또한 외부 API 비용 + rate-limit이 운영 burden이다. Reddit / HN /
SearXNG / Naver 등 adapter는 per-key 분당 호출 한도가 있어 트래픽 폭주
시 일부 호출이 RATE_LIMITED로 빠진다. 본 SPEC이 30% 트래픽을 흡수하면
adapter pool 부하가 directly 30% 감소한다.

마지막으로 **M6 GA 조건**으로 product.md 성공 지표("dedup ≥ 30% on
repeated queries")가 명시되어 있으며, 본 SPEC이 이 지표의 sole driver다.

### 1.3 Orthogonality with SPEC-CACHE-001

SPEC-CACHE-001(implemented)의 5-phase access fallback은 **doc-level**
콘텐츠 fetch 영역(URL → body bytes)을 담당하며, IDX-005는 **answer-level**
재사용(query → synthesized answer)을 담당한다. 두 SPEC은 layer가 다르고
disjoint하다. 한쪽의 hit이 다른 쪽 cascade를 차단하지 않는다.

다만 IDX-005의 citation 재검증(REQ-IDX5-004) eager 모드는 CACHE-001
Phase 2의 HEAD probe + robots.txt 검증 코드를 internal API로 재사용한다
— code reuse는 있지만 contractual dependency는 없다.

---

## 2. EARS Requirements

### 2.1 Lookup Pipeline Module

| ID | Pattern | Requirement | Priority | Acceptance |
|----|---------|-------------|----------|------------|
| **REQ-IDX5-001** | Event-Driven | WHEN `/query` 요청이 IDX-005 middleware에 도달하고 `force_refresh` 플래그(query param 또는 header)가 FALSE 또는 부재인 경우, middleware는 (a) JWT context로부터 `team_id`를 추출 SHALL 하고(SPEC-AUTH-001 강제), (b) embedder를 통해 query embedding을 계산 SHALL 하고(SPEC-IDX-002 sidecar 재사용), (c) `index.Search(ctx, IndexQuery{TeamID: T, DocTypes: [DocTypeCachedAnswer], MaxResults: 1, ...})`를 호출하여 team-scoped top-1 similarity match를 retrieve SHALL 한다. fanout 호출은 lookup 결과를 평가한 후 분기 SHALL 한다 (REQ-IDX5-002에 의해 결정). | P0 | `TestLookupSkipsOnForceRefreshQueryParam`, `TestLookupSkipsOnForceRefreshHeader`, `TestLookupCallsEmbedderOnce`, `TestLookupCallsIndexSearchWithTeamScopedFilter` |
| **REQ-IDX5-002** | Event-Driven | WHEN REQ-IDX5-001의 lookup이 반환한 top-1 match의 cosine similarity score가 deep.yaml `costguard.idx5.similarity_threshold`(default 0.92) 이상이고, 매치 record의 staleness 평가(REQ-IDX5-003)가 `fresh` 또는 `soft-stale`인 경우, middleware는 fanout 호출을 SHALL skip하고 cached `SynthesizeResponse` JSON을 immediate response로 SHALL serve한다. response는 HTTP 200 + body는 SYN-001 schema 그대로, 추가 response header `X-Cache: HIT` (또는 `SOFT-HIT` if soft-stale), `X-Cache-Age-Seconds: N`, `X-Cache-Score: F.FF`를 SHALL 부착한다. WHEN score < threshold OR staleness == `hard-stale` OR lookup returned empty, middleware는 fanout pipeline으로 SHALL fall-through. WHERE per-team threshold override(deep.yaml `costguard.idx5.team_overrides.{team_id}.similarity_threshold`)가 설정된 경우 global default 대신 override 값을 SHALL 사용한다. | P0 | `TestLookupHitAboveThresholdServesCached`, `TestLookupMissBelowThresholdFallsThrough`, `TestLookupHardStaleFallsThrough`, `TestLookupPerTeamThresholdOverride`, `TestLookupResponseHeadersOnHit` |
| **REQ-IDX5-003** | Event-Driven | WHEN cached_answer record가 lookup에서 반환되면, middleware는 staleness를 다음 규칙으로 SHALL 평가한다: cached_answer record의 `category` 필드(IR-001 category)를 키로 deep.yaml `costguard.idx5.staleness_ttl_by_category`에서 `ttl_seconds`(default: web=3600, social=1800, academic=2592000, korean=3600, mixed=7200, unknown=7200)를 lookup. `age = now - cached_answer.created_at`. `age < 0.5 * ttl` → `fresh`. `0.5 * ttl <= age < ttl` → `soft-stale`. `age >= ttl` → `hard-stale`. 추가로 cached_answer.force_stale가 TRUE이면 staleness는 무조건 `hard-stale`이 SHALL 된다. | P0 | `TestStalenessFresh`, `TestStalenessSoftStale`, `TestStalenessHardStale`, `TestStalenessForceStaleOverride`, `TestStalenessPerCategoryTTL` |
| **REQ-IDX5-004** | Optional | WHERE deep.yaml `costguard.idx5.citation_revalidation`이 `"eager_top_n"`(default N=3)으로 설정된 경우, hit serve 직전 middleware는 cached_answer.citations slice의 top-N citation URL에 parallel HTTP HEAD probe를 SHALL 발사한다(timeout 200ms per probe via `context.WithTimeout`). 4xx(404, 410)을 반환한 citation은 응답 body에서 SHALL strip되고 응답 header `X-Cache-Citation-Stale: 1`이 SHALL 부착된다. timeout 또는 5xx은 무시(citation 유지) SHALL 한다. WHERE `citation_revalidation: "lazy"`(default)인 경우 본 단계는 SHALL skip된다. WHERE `citation_revalidation: "eager_all"`인 경우 모든 citation에 동일 probe를 SHALL 적용한다. probe 구현은 SPEC-CACHE-001의 Phase 2 HEAD probe path를 internal API로 SHALL 재사용한다. | P1 | `TestCitationRevalidationLazyDefault`, `TestCitationRevalidationEagerTopNStrips404`, `TestCitationRevalidationEagerTopNKeepsTimeout`, `TestCitationRevalidationReusesPhase2HeadProbe` |
| **REQ-IDX5-005** | Event-Driven | WHEN REQ-IDX5-003의 staleness 평가가 `soft-stale`이고 hit이 serve되면, middleware는 비동기 Asynq job `idx5-refresh`를 enqueue SHALL 한다. payload: `{team_id, query_text, original_doc_id, category}`. job worker는 standard fanout + synthesis pipeline을 실행 SHALL 하며 결과를 same `doc_id`로 IDX-001 Upsert SHALL 한다(idempotent overwrite). refresh job 실패는 response latency에 영향을 SHALL NOT 미치며 다음 lookup 시점에 hard-stale로 evict처리 SHALL 된다. WHEN cached_answer의 hit이 일어날 때마다 `hit_count`와 `last_served_at`이 PG에서 SHALL update된다(async, non-blocking). | P0 | `TestSoftStaleEnqueuesRefreshJob`, `TestRefreshJobUpsertsOverSameDocId`, `TestRefreshJobFailureDoesNotImpactResponse`, `TestHitCountIncrementsOnServe`, `TestLastServedAtUpdatesOnServe` |

### 2.2 Storage & Cache Write Module

| ID | Pattern | Requirement | Priority | Acceptance |
|----|---------|-------------|----------|------------|
| **REQ-IDX5-006** | Ubiquitous | `cmd/usearch-api/handlers/query.go`(또는 동등 핸들러)는 fanout MISS 경로(IDX-005 lookup이 miss하거나 force-refresh로 bypass된 경우)의 synthesis 응답을 받은 직후, 다음을 비동기로 fire-and-forget SHALL 한다: (a) Qdrant `usearch_docs` collection에 point upsert (point_id = `docID("answer-cache", queryHash + ":" + team_id)`, vector = query embedding, payload = {doc_type: "cached_answer", team_id, category, created_at, ttl_seconds, force_stale: false}); (b) PG `answer_cache` 테이블에 INSERT ON CONFLICT (doc_id) DO UPDATE row (synthesized_text, citations, model, provider, cost_usd, prompt_tokens, completion_tokens, ttl_seconds, query_text, query_embedding_hash, created_at = NOW(), force_stale = FALSE, hit_count = 0). fire-and-forget 실패는 `/query` 응답 latency에 영향을 SHALL NOT 미치며 WARN level slog로 SHALL 기록된다. PG migration 파일은 `deploy/postgres/migrations/0003_answer_cache.sql`로 신설 SHALL 된다. | P0 | `TestCacheWriteOnMissFireAndForget`, `TestCacheWriteFailureDoesNotImpactResponse`, `TestCacheWriteUpsertOverwritesIdempotent`, `TestMigration0003Idempotent`, `TestAnswerCacheSchemaMatchesSpec` |
| **REQ-IDX5-007** | Ubiquitous | 모든 IDX-005 lookup 및 write 작업은 `team_id`를 격리 키로 SHALL 사용한다. (a) lookup의 `index.Search(ctx, IndexQuery)`는 `q.TeamID`를 반드시 채워야 SHALL 하며 SPEC-IDX-004의 multi-tenancy 강제(Qdrant payload filter, Meili tenant token, PG row-level security)를 trust SHALL 한다. (b) write의 doc_id 구성에 team_id를 SHALL 포함한다(`docID("answer-cache", queryHash + ":" + team_id)`). (c) team T의 cached_answer가 team U에 의해 reuse되는 가능성은 zero SHALL 한다. (d) `team_id`가 context에 부재(JWT 인증 실패 또는 미인증 요청)할 경우 middleware는 lookup을 SHALL skip하고 fanout 직진 SHALL 한다(에러는 발생시키지 SHALL NOT, AUTH-001 미들웨어가 reject 처리). cross-tenant leak가 acceptance test로 검증 SHALL 된다. | P0 | `TestLookupRequiresTeamIDInContext`, `TestLookupSkipsWhenTeamIDMissing`, `TestDocIDIncludesTeamId`, `TestCrossTenantLookupReturnsZeroResults`, `TestCrossTenantWriteDoesNotPollute` |

### 2.3 Feedback & Observability Module

| ID | Pattern | Requirement | Priority | Acceptance |
|----|---------|-------------|----------|------------|
| **REQ-IDX5-008** | Event-Driven | WHEN 사용자가 `POST /feedback {request_id, score: -1}` 엔드포인트를 호출(SPEC-EVAL-* M8 또는 UI surface에서 trigger), 본 SPEC은 다음 핸들러를 SHALL 노출한다: (a) `request_id`로부터 (`team_id`, `cached_doc_id`) tuple을 복구 SHALL 한다 (lookup serve 시점에 request_id → doc_id mapping을 in-memory TTL=24h LRU에 저장); (b) `answer_cache.force_stale = TRUE` UPDATE WHERE doc_id = cached_doc_id AND team_id = team_id SHALL 실행한다; (c) 같은 (team_id, user_id, doc_id) 조합의 feedback이 24h 내 중복 호출되는 경우 idempotent하게 처리 SHALL 한다(중복 UPDATE 무해). cached_doc_id를 복구할 수 없는 경우(예: 24h LRU 만료, fanout MISS 경로 응답에 대한 feedback) 핸들러는 SHALL 200을 반환하되 internal counter `usearch_idx5_feedback_unmapped_total`을 1 증가 SHALL 시킨다. | P1 | `TestFeedbackMarksForceStale`, `TestFeedbackIdempotentOnDuplicate`, `TestFeedbackUnmappedIncrementsCounter`, `TestFeedbackRespectsTenantBoundary` |
| **REQ-IDX5-009** | Ubiquitous | 본 SPEC은 다음 Prometheus metric family를 SHALL 등록한다 (`internal/obs/metrics/idx5.go` 신설): (a) `usearch_idx5_lookups_total{outcome}` Counter, outcome ∈ {`hit`, `miss`, `soft_hit`, `hard_stale`, `bypassed`, `error`}. (b) `usearch_idx5_lookup_duration_seconds` Histogram (no labels). (c) `usearch_idx5_dedup_hit_rate{team_id_hashed}` Gauge, 24h rolling hit / (hit + miss); `team_id_hashed = SHA256(team_id)[:8]`. team_id_hashed values는 deploy whitelist `costguard.idx5.allowed_team_hashes`에 bounded SHALL 되며 화이트리스트 외 값은 `team_id_hashed=unknown`으로 collapse. (d) `usearch_idx5_reuse_latency_ms{outcome}` Histogram, outcome ∈ {`hit`, `soft_hit`}. (e) `usearch_idx5_stale_evictions_total{category, mode}` Counter, mode ∈ {`lazy`, `sync`}. (f) `usearch_idx5_feedback_marks_total{score}` Counter, score ∈ {`thumbs_down`, `thumbs_up`}. (g) `usearch_idx5_feedback_unmapped_total` Counter (no labels). Cardinality allowlist 확장: 새 label name `team_id_hashed` 1개, 새 outcome values는 모두 bounded enum. NFR-OBS-002 `TestNoUnboundedLabels` 화이트리스트 update가 필요 SHALL 하다. middleware는 `obs.Obs`, `obs.Metrics`, individual collector, `obs.Logger`가 nil인 경우에도 panic하지 SHALL NOT 한다. | P0 | `TestMetricsRegisteredWithCorrectNames`, `TestLookupOutcomeLabels`, `TestDedupHitRateGaugeBoundedByWhitelist`, `TestReuseLatencyHistogramEmittedOnHit`, `TestObservabilitySafeOnNilObs`, `TestCardinalityAllowlistUpdated` |
| **REQ-IDX5-010** | Ubiquitous | 본 SPEC은 lookup 호출당 한 개의 OTel parent span attribute set을 부모 `api.query` span에 SHALL 부착한다: `idx5.lookup.outcome` ∈ {hit, miss, soft_hit, hard_stale, bypassed, error}, `idx5.lookup.similarity_score` (float64), `idx5.lookup.cached_age_seconds` (int64), `idx5.lookup.cached_doc_id` (string), `idx5.lookup.ttl_remaining_seconds` (int64), `idx5.citation.revalidation_mode` ∈ {lazy, eager_top_n, eager_all}, `idx5.citation.stripped_count` (int64; eager 모드에서만 의미). 본 SPEC은 lookup 결과의 한 개 INFO-level slog JSON line(`event_type: "idx5.lookup"`)을 SHALL emit한다. line schema는 §6.3 forward-compat 필드(`timestamp`, `event_type`, `request_id`, `tenant_id`, `user_id`, `outcome`, `similarity_score`, `cached_doc_id`, `category`, `cached_age_seconds`, `ttl_remaining_seconds`, `latency_ms`)를 모두 포함 SHALL 한다. SPEC-AUTH-003(M6) audit log subsystem이 downstream consumer로 합류할 때 호환되도록 schema는 additive only SHALL 한다. | P0 | `TestOTelSpanAttributesEmitted`, `TestSlogJSONLineSchemaComplete`, `TestSlogLineRequestIdPropagated`, `TestSlogLineAddtiveOnlySchema` |

---

## 3. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| **NFR-IDX5-001** | M6 dedup hit-rate target | 단일 팀(`team_id` fixed) 24h rolling window에서 `usearch_idx5_lookups_total{outcome="hit"}` + `{outcome="soft_hit"}` / total lookups 비율은 production 트래픽에 대해 ≥ 30% SHALL 유지된다(`.moai/project/roadmap.md` M6 exit criterion). 미달 시 `usearch_idx5_dedup_hit_rate{team_id_hashed}` 게이지가 0.30 미만을 emit하며 운영자 알람 트리거. acceptance test에서는 synthetic 트래픽으로 검증(§5.5 참고). |
| **NFR-IDX5-002** | Lookup overhead on MISS path | IDX-005 lookup이 cache MISS로 빠지는 경우 추가되는 latency overhead는 p95 ≤ 50ms SHALL 한다. 구성: embedder call(~10ms) + Qdrant similarity search(~30ms, NFR-IDX-002 budget 100ms의 30%) + threshold/staleness eval(~5ms) + Postgres lookup confirm(~5ms). 측정: `usearch_idx5_lookup_duration_seconds` Histogram, outcome=miss filter. |
| **NFR-IDX5-003** | Reuse latency on HIT path | cache HIT 경로의 end-to-end response latency(`/query` request 진입 → response body 전송 완료)는 p95 ≤ 200ms SHALL 한다. 구성: lookup(~50ms) + cached_answer read(~30ms) + (optional) citation re-validation(0~200ms depending on mode) + JSON serialize(~5ms) + network send(~10ms). 측정: `usearch_idx5_reuse_latency_ms{outcome="hit"}` Histogram. |
| **NFR-IDX5-004** | Cross-tenant leak probability | IDX-005를 통한 cross-tenant cached answer leak 확률은 정확히 0 SHALL 한다 (acceptance test §5.6 참고). 4-layer defense(IDX-004 Qdrant filter + IDX-004 PG RLS + doc_id team_id 포함 + acceptance test cross-tenant probe)가 모두 작동해야 한다. |
| **NFR-IDX5-005** | Stale eviction lag | hard-stale로 판정된 cached_answer record는 다음 lookup cycle(=다음 동일 query 호출) 시점에 evict되거나 새 write로 overwrite SHALL 된다. 즉 evict lag는 정확히 1 cycle. lazy evict의 staleness 누적은 발생하지 SHALL NOT 한다. |
| **NFR-IDX5-006** | Refresh job concurrency safety | 동일 (team_id, query_text) 조합에 대해 동시에 두 개의 `idx5-refresh` Asynq job이 launch되는 경우(timing window), 두 job이 same doc_id로 IDX-001 Upsert를 동시 호출해도 race 없이 idempotent SHALL 처리된다(IDX-001 REQ-IDX-005 ON CONFLICT (doc_id) DO UPDATE 보장). `goleak.VerifyNone`이 통과 SHALL 한다. |
| **NFR-IDX5-007** | Configurability via deep.yaml hot-reload | `costguard.idx5.*` 모든 설정(similarity_threshold, staleness_ttl_by_category, citation_revalidation, team_overrides, allowed_team_hashes)은 서비스 재시작 없이 SIGHUP 신호 또는 fsnotify file watcher로 hot-reload 가능 SHALL 한다. hot-reload 후 다음 lookup부터 새 값이 적용되며 진행 중인 lookup은 reload 이전 값으로 완료된다. config 변경 시점은 `usearch_idx5_config_version` 게이지가 새 hash로 emit된다. SPEC-DEEP-004 NFR-DEEP4-008의 hot-reload 패턴을 SHALL 재사용한다. |

---

## 4. Exclusions (What NOT to Build)

[HARD] 본 SPEC은 다음 항목을 명시적으로 제외한다. 각 항목은 후속 SPEC 또는
별도 트랙의 책임이다.

- **다중-user threshold tuning for feedback marking** — V1은 단일
  thumbs-down으로 즉시 stale. operator-tunable(예: 3 distinct user
  thumbs-down 필요)은 SPEC-EVAL-003(M9)에 위임.
- **Redis hot lookup cache layer** — V1은 Qdrant similarity search만 사용.
  Qdrant latency가 충분(≤100ms p50)하여 추가 cache layer 도입 NOT.
  V2 SPEC-IDX-005a가 측정 후 결정.
- **Partial-overlap signal logging** — query가 cached answer 여러 개와
  sub-threshold 유사도일 때 partial-overlap signal로 로그 emit하지 NOT.
  V1은 single best-match only. SPEC-EVAL-003(M9)가 add 가능.
- **Cached answer compression** — `synthesized_text` + `citations` JSONB의
  명시적 zstd 압축은 V1 미적용. PG TOAST가 자동 처리. V2 측정 후 결정.
- **Eager re-validation citation pool dedup** — 여러 동시 lookup이 같은
  citation URL을 재검증할 때 in-process dedup 없음. SPEC-EVAL-002(M8)
  operator dashboard로 가시화하여 후속 판단.
- **TTL inheritance for mixed-category queries** — IR-001이 `mixed` 분류
  반환 시 component categories의 min을 inherit하는 logic은 IR-001
  amendment 필요(현재 IR-001은 single category만 반환). V1은 deploy 시점
  fixed value(2h).
- **Team admin bulk evict endpoint** — `POST /teams/{team_id}/idx5/evict`
  같은 admin API는 SPEC-AUTH-004(M7)에 위임. V1은 DB 직접 DELETE로만
  가능.
- **Citation reuse audit trail** — 각 reuse 이벤트(request_id, served_at,
  user_id)의 상세 audit row 저장은 SPEC-AUTH-003(M6 audit log)에 위임.
  V1은 hit_count 카운터만 maintain.
- **Background refresh job per-team rate-limiting** — Asynq `idx5-refresh`
  job의 per-team concurrency 제한은 V1 미구현(global pool). SPEC-EVAL-002
  (M8) 후속.
- **Mid-content cache invalidation** — cached citation URL의 콘텐츠가
  변경되었지만 URL은 200 OK인 경우(SPEC-SYN-002 citation faithfulness
  영역)는 본 SPEC 범위 밖. TTL 기반 간접 mitigation만 제공.
- **Anthropic prompt cache replay** — LiteLLM의 vendor-native prompt cache
  reuse(DEEP-004와 별도 layer)는 본 SPEC 미적용. SPEC-COST-OPT-001(M8)에
  위임.
- **GitHub Issue tracking on this SPEC** (`issue_number: null` per session
  pattern — orchestrator handles).

---

## 5. Acceptance Scenarios

상세 Given/When/Then 시나리오는 `.moai/specs/SPEC-IDX-005/acceptance.md`에
정의되어 있다. 본 절은 인덱스를 제공한다.

| Scenario | 설명 | Coverage |
|----------|------|----------|
| §5.1 | team T 사용자가 fresh cached answer hit → 200 + X-Cache: HIT | REQ-001, 002, 007 |
| §5.2 | team T 사용자가 soft-stale hit → 200 serve + 비동기 refresh enqueue | REQ-001, 002, 003, 005 |
| §5.3 | team T 사용자가 hard-stale → MISS + fanout fall-through + write | REQ-002, 003, 006 |
| §5.4 | sub-threshold similarity → MISS + fanout + write | REQ-001, 002 |
| §5.5 | M6 exit gate: 100 query synthetic 트래픽 dedup hit-rate ≥30% 측정 | NFR-001, REQ-009 |
| §5.6 | cross-tenant probe: team U가 team T의 cached answer를 reuse 시도 → MISS 강제 | REQ-007, NFR-004 |
| §5.7 | feedback thumbs-down → 다음 lookup에서 hard-stale → fanout 재실행 | REQ-005, 008 |
| §5.8 | citation re-validation eager_top_n 모드: 404 citation 응답에서 strip | REQ-004 |
| Edge1 | force_refresh=true 우회: hit 가능 상태에서도 fanout 직진 | REQ-001 |
| Edge2 | TTL boundary: age = exactly TTL → hard-stale 분기 (boundary atomicity) | REQ-003 |

---

## 6. Dependencies & Blocks

### 6.1 Upstream SPEC dependencies (depends_on)

- **SPEC-IDX-001** (implemented) — `*index.Index.Search` / `Upsert` 공개
  surface 직접 consume. doc_id 결정 로직 확장. `usearch_docs` Qdrant
  collection 재사용. **HARD dep**.
- **SPEC-IDX-004** (M6 depends_on) — multi-tenancy 강제 위에 build. Qdrant
  payload filter + Meili tenant token + PG row-level security를 trust.
  **HARD dep**.
- **SPEC-AUTH-001** (M6 depends_on) — JWT context로부터 `team_id` 추출.
  context key 패턴은 DEEP-004의 `costguard.UserIDKey`와 일치. **HARD dep**.
- **SPEC-AUTH-002** (M6 depends_on) — Casbin policy로 user의 team 소속
  검증. IDX-005 middleware는 이를 trust. **HARD dep**.
- **SPEC-CACHE-001** (implemented) — Phase 2 HEAD probe path를 internal
  API로 재사용(eager citation re-validation 모드). **SOFT dep** — 코드
  reuse만, contract 의존성 없음.
- **SPEC-SYN-001** (implemented) — `SynthesizeResponse` JSON shape를 hit
  serve의 wire format으로 재사용. **HARD dep**.
- **SPEC-FAN-001** (implemented) — `Dispatch` call site 직전에 middleware
  주입. hit 시 fanout 호출을 skip하는 분기 로직. **HARD dep**.

### 6.2 Downstream blocked SPECs (blocks)

본 SPEC은 직접 블로킹하는 후속 SPEC이 없다. M6 release gate의 PRIMARY
DRIVER로서 본 SPEC이 ship되면 M6 GA 조건의 dedup hit rate ≥30%가 측정
가능 상태에 진입한다.

### 6.3 Forward-compatibility commitment with SPEC-AUTH-003 audit log

REQ-IDX5-010의 decision event log JSON line schema는 SPEC-AUTH-003(M6)
audit subsystem이 downstream consumer로 합류할 때 호환되도록 설계된다.
slog line은 다음 필드를 SHALL 포함한다: `timestamp`(ISO-8601), `event_type`
(="idx5.lookup"), `request_id`, `tenant_id`, `user_id`, `outcome`,
`similarity_score`, `cached_doc_id`, `category`, `cached_age_seconds`,
`ttl_remaining_seconds`, `latency_ms`. schema는 **additive**이며
SPEC-AUTH-003는 새 필드를 추가할 수 있으나 위 필드를 rename하거나 remove할
수 SHALL NOT 한다. AUTH-003 진입 시 audit subsystem review checkpoint가
본 commitment를 재검증한다.

DEEP-004 REQ-DEEP4-010의 decision event log와 disjoint하며 (event_type이
다름), AUTH-003가 두 source를 모두 consume하도록 설계된다.

---

## 7. Files to Create / Modify

### 7.1 Created

| Marker | Path | Purpose |
|--------|------|---------|
| [NEW] | `internal/idx5/middleware.go` | chi v5 미들웨어 — lookup orchestration + force-refresh handling |
| [NEW] | `internal/idx5/lookup.go` | embedder call + Qdrant similarity search + threshold evaluation |
| [NEW] | `internal/idx5/staleness.go` | per-category TTL evaluation + force_stale 처리 |
| [NEW] | `internal/idx5/serve.go` | cached answer을 SYN-001 SynthesizeResponse JSON으로 reconstruct + response header 부착 |
| [NEW] | `internal/idx5/writeback.go` | fanout MISS 경로의 async cache write (Qdrant + PG) |
| [NEW] | `internal/idx5/citation_revalidate.go` | eager mode citation HEAD probe (CACHE-001 Phase 2 path 재사용) |
| [NEW] | `internal/idx5/feedback.go` | thumbs-down handler → force_stale UPDATE |
| [NEW] | `internal/idx5/refresh_job.go` | Asynq `idx5-refresh` job worker (soft-stale background refresh) |
| [NEW] | `internal/idx5/config.go` | deep.yaml `costguard.idx5.*` 로더 + fsnotify hot-reload watcher |
| [NEW] | `internal/idx5/metrics.go` | Prometheus collector 등록 헬퍼 |
| [NEW] | `internal/idx5/types.go` | `LookupResult`, `Staleness` enum, `CachedAnswer` struct 등 |
| [NEW] | `internal/idx5/middleware_test.go` | Lookup orchestration + force-refresh + cross-tenant 통합 테스트 |
| [NEW] | `internal/idx5/lookup_test.go` | similarity threshold + Qdrant filter + per-team override |
| [NEW] | `internal/idx5/staleness_test.go` | TTL 평가 + boundary + force_stale |
| [NEW] | `internal/idx5/writeback_test.go` | async write fire-and-forget + idempotent upsert |
| [NEW] | `internal/idx5/citation_revalidate_test.go` | lazy/eager_top_n/eager_all 모드 분기 + 404 strip |
| [NEW] | `internal/idx5/feedback_test.go` | feedback marks force_stale + idempotent + cross-tenant |
| [NEW] | `internal/idx5/refresh_job_test.go` | soft-stale enqueue + worker upsert + race safety |
| [NEW] | `internal/idx5/config_test.go` | deep.yaml 파싱 + hot-reload + per-team override |
| [NEW] | `internal/idx5/integration_test.go` | M6 exit criterion synthetic traffic dedup hit-rate ≥30% |
| [NEW] | `internal/obs/metrics/idx5.go` | 7 Prometheus collector 정의 + registerIDX5 헬퍼 |
| [NEW] | `deploy/postgres/migrations/0003_answer_cache.sql` | `answer_cache` 테이블 + 3 indexes 마이그레이션 |

### 7.2 Modified

| Path | Change |
|------|--------|
| `cmd/usearch-api/handlers/query.go` (or 동등 핸들러) | chi v5 미들웨어 체인에 `idx5.LookupMiddleware`를 fanout dispatch 직전에 SHALL wire. fanout MISS 경로 종료 직후 `idx5.WriteBack` 비동기 호출 SHALL 추가 |
| `cmd/usearch-api/main.go` | `idx5.New(cfg, obs, embedder, indexClient, pgPool, asynqClient)` 초기화 + scheduled health-check hooks SHALL 등록 |
| `internal/obs/metrics/metrics.go` | `registerIDX5(r)` 헬퍼 호출 추가 |
| `internal/obs/obs.go` | `obs.IDX5Lookups`, `obs.IDX5LookupDuration`, `obs.IDX5DedupHitRate`, `obs.IDX5ReuseLatency`, `obs.IDX5StaleEvictions`, `obs.IDX5FeedbackMarks`, `obs.IDX5FeedbackUnmapped` 신설 collector re-export |
| `internal/obs/metrics/metrics_test.go` | `TestNoUnboundedLabels` 화이트리스트에 `team_id_hashed`, 새 outcome enum values 추가 |
| `pkg/types/normalized_doc.go` (or 동등) | `DocType` enum에 `DocTypeCachedAnswer = "cached_answer"` SHALL 추가 |
| `internal/index/index.go` | `IndexQuery.DocTypes` 필터링이 `DocTypeCachedAnswer`를 honor SHALL 한다 (기존 enum 처리 path에 자동 포함; 변경 없으면 trivial) |
| `.moai/config/sections/deep.yaml` | `costguard.idx5.*` 섹션 신설 (similarity_threshold, staleness_ttl_by_category, citation_revalidation, team_overrides, allowed_team_hashes, redis 등) |
| `.env.example` | `IDX5_SIMILARITY_THRESHOLD`, `IDX5_CITATION_REVALIDATION_MODE`, `IDX5_DEFAULT_TENANT_ID` 등 신규 env-var 문서화 |

### 7.3 Existing — Unchanged

- `internal/index/` (IDX-001) — `*index.Index.Search` / `Upsert` public
  surface는 IDX-005가 그대로 호출. 코드 변경 없음(`DocType` enum 추가는
  `pkg/types`에서 자동 작동).
- `internal/fanout/` (FAN-001) — `Dispatch` 자체는 변경 없음. 호출자가
  분기 결정.
- `services/researcher/` (SYN-001) — Python 사이드카는 변경 없음.
- `internal/access/` (CACHE-001) — Phase 2 HEAD probe path는 read-only로
  re-import.
- `internal/auth/` (AUTH-001/002, M6 신설) — IDX-005가 context key를
  trust하며 자체 검증 없음.

---

## 8. Open Questions

본 SPEC은 §1.1의 10개 pinned decision으로 대부분의 ambiguity를 해소했다.
다음 항목은 plan-auditor와의 협의 또는 첫 운영 데이터 기반 튜닝이 필요한
경계 사례다.

1. **per-team similarity threshold runtime API**: V1은 deploy 시점 deep.yaml
   map. runtime modification API(예: `POST /teams/{team_id}/idx5/config`)는
   V2. **Owner**: SPEC-AUTH-004 (M7) team admin API author.

2. **Partial-overlap signal**: query가 cached answer 여러 개와 sub-threshold
   유사도일 때 partial-overlap을 metric으로 emit할지. V1은 single best-match
   only. **Owner**: SPEC-EVAL-003 (M9) author may add.

3. **Cached answer storage compression**: 명시적 zstd compression layer가
   V1에 필요한가. PG TOAST가 자동 처리하므로 V1은 미적용. **Owner**: V2
   measurement decision after 30-day production data.

4. **Eager re-validation citation pool sharing**: V1은 in-process dedup
   미구현. **Owner**: SPEC-EVAL-002 (M8) dashboard로 가시화 후 결정.

5. **Mixed-category TTL inheritance**: V1은 fixed 2h. min(components)
   inherit logic은 IR-001 amendment 필요. **Owner**: future SPEC-IR-001a.

위 5개는 plan-auditor가 SPEC을 PASS로 평가하기 위해 필수적인 결정이 아니다.
모두 first-30-day 운영 데이터로 튜닝 가능한 항목이다.

---

## 9. References

### External (URL-cited; verified per research §9)

- https://huggingface.co/BAAI/bge-m3 — BGE-M3 embedding model used for
  query similarity. Reformulation cutoff 측정 근거.
- https://qdrant.tech/documentation/concepts/search/ — Qdrant cosine
  similarity score 정의 (1.0 = identical, 0.0 = orthogonal). vector_size
  + distance metric IDX-001과 동일.
- https://www.postgresql.org/docs/current/storage-toast.html — PG TOAST
  자동 압축 (research §9 cited).
- RFC 7234 §4.2.4 — Cache-Control stale-while-revalidate semantics (loose
  conceptual reference for soft-stale).

### Internal (file:line cited)

- `.moai/specs/SPEC-IDX-005/research.md` — 본 SPEC의 research artifact.
- `.moai/project/roadmap.md:85` — M6 row "SPEC-IDX-005 | Team-shared
  answer reuse | pre-fanout lookup in team index, configurable staleness
  threshold".
- `.moai/project/roadmap.md:150` — M6 exit criterion mention("shared index
  dedup hits ≥30%").
- `.moai/project/product.md` 성공 지표 — "dedup ≥ 30% on repeated queries".
- `.moai/specs/SPEC-IDX-001/spec.md:171-258` — Index public surface
  contract.
- `.moai/specs/SPEC-IDX-001/spec.md:520-545` — multi-tenancy reservation
  (§2.8); IDX-005가 trust하는 IDX-004의 강제 위에 build.
- `.moai/specs/SPEC-IDX-001/spec.md:553-566` — REQ-IDX-006 (Search) +
  REQ-IDX-014 (docID determinism).
- `.moai/specs/SPEC-FAN-001/spec.md:6` — FAN-001 status: implemented.
- `.moai/specs/SPEC-FAN-001/spec.md:296` — REQ-FAN-001 Dispatch contract
  (bypass target).
- `.moai/specs/SPEC-SYN-001/spec.md:188-194` — SynthesizeResponse JSON
  shape (hit serve format reused).
- `.moai/specs/SPEC-CACHE-001/spec.md:478` — REQ-CACHE-003 HEAD probe path
  (eager mode 재사용).
- `.moai/specs/SPEC-CACHE-001/spec.md:260` — Phase 2 implementation in
  `internal/access/phase2_probe.go`.
- `.moai/specs/SPEC-DEEP-004/spec.md` — middleware chain pattern, deep.yaml
  hot-reload pattern, decision event log forward-compat pattern.
- `.moai/specs/SPEC-DEEP-004/spec.md:178-181` — REQ-DEEP4-001 X-User-Id /
  X-Tenant-Id context propagation pattern (mirror).
- `.moai/specs/SPEC-OBS-001/spec.md` — Prometheus naming + cardinality
  discipline.
- `internal/index/index.go:108` — `usearch_docs` Meili filterable
  attributes including team_id (IDX-005가 정렬).
- `internal/index/dispatch.go:232` — IndexQuery.TeamID filter activation.
- `internal/index/dispatch.go:309-347` — payload team_id field (IDX-004가
  활성화, IDX-005가 trust).
- `internal/index/pg/client.go:121-246` — `docs` 테이블 team_id 참조
  (변경 없음).
- `internal/index/docid.go` — deterministic doc_id (IDX-005가
  `docID("answer-cache", ...)` 패턴으로 확장).
- `internal/index/qdrant/client.go:242` — Qdrant payload team_id field
  (IDX-005가 같은 payload 형식으로 cached_answer 저장).

---

*End of SPEC-IDX-005 v0.1.0 (draft; pending plan-auditor cycle).*
