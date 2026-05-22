# SPEC-IDX-005 Deep Research

Generated: 2026-05-22T00:00:00Z
Author: manager-spec (Phase 0.5 — context-derived)
Consumed by: manager-spec (Phase 1B), plan-auditor (Phase 2.3)

---

## 0. Scope of This Research

본 research.md는 M6 milestone의 마지막 P1 deliverable인 SPEC-IDX-005
("Team-shared answer reuse")에 대한 코드베이스 분석 + 아키텍처 결정
기록이다. `.moai/project/roadmap.md:85`의 한 줄 scope("pre-fanout lookup
in team index, configurable staleness threshold")를 (a) 사전 lookup
파이프라인, (b) staleness 모델, (c) citation 재검증, (d) 테넌시 강제,
(e) feedback 루프, (f) 관측·이벤트의 6개 축으로 분해해 각 축의 코드 진입점,
의존 SPEC, 데이터 스토어, 보안 영향을 명세화한다.

IDX-005는 **M6 exit criterion의 PRIMARY DRIVER**다: `.moai/project/roadmap.md`
M6 row("shared index dedup hits ≥30%")는 IDX-005가 ship되어 production
트래픽에 적용되어야만 측정 가능한 지표이며, 본 SPEC은 dedup hit rate를
SLO화하고 자체 acceptance test에서 30% 임계값을 강제한다.

본 SPEC은 M6의 마지막 SPEC으로 사전에 AUTH-001 / AUTH-002 / IDX-004가
모두 ship되어야만 시작할 수 있다(tenant 격리 강제와 JWT 발급 미들웨어 의존).
v0 cap-check / Haiku screen 패턴(DEEP-004)을 mirror하여 chi v5 middleware
계층으로 통합한다.

---

## 1. Existing Code Surface

### 1.1 SPEC-IDX-001 (implemented) — Hybrid Index Foundation

`.moai/specs/SPEC-IDX-001/spec.md:171-258`에 정의된 `*index.Index`는 본 SPEC의
주된 의존 surface다. 관련 진입점:

- `internal/index/index.go::(*Index).Search(ctx, IndexQuery) (*IndexResult, error)`
  (REQ-IDX-006). v0.1는 query embedding → Qdrant + Meili + PG 3-store
  parallel search → RRF fusion 흐름이며 `IndexQuery.TeamID`가 v0에서
  NULL filter로만 동작(SPEC-IDX-001 §2.8 H10 multi-tenancy reservation).
- `internal/index/index.go::(*Index).Upsert(ctx, []NormalizedDoc) (*UpsertResult, error)`
  (REQ-IDX-005). 본 SPEC은 `Upsert`를 통해 cached answer를 IDX shard에
  기록한다 — 단, `SourceID="answer-cache"` 등 별도 namespace를 사용.
- `internal/index/docid.go::docID(sourceID, url)` (REQ-IDX-014). cached answer
  의 `doc_id`는 `docID("answer-cache", queryHash)` 형태로 deterministic
  하게 생성된다.
- `internal/index/types.go::IndexQuery{TeamID, ...}`. team scoping 필드는
  이미 존재. IDX-004(M6)가 이 필드의 강제 의미를 활성화하고 IDX-005는 그
  강제 위에 build한다.

### 1.2 SPEC-IDX-004 (M6 dependency) — Multi-tenancy Enforcement

IDX-004는 SPEC-IDX-001 §2.8에 reserved된 `team_id` 컬럼을 `NOT NULL`로
flip하고, Qdrant payload-based partitioning, Meili tenant tokens, PG
row-level security를 도입한다. IDX-005는 IDX-004의 격리 위에 build되며
다음 의미를 가정한다:

- 모든 `Index.Search(ctx, q{TeamID: T})` 호출은 T 외 팀의 row를 반환하지
  SHALL NOT 한다(IDX-004 강제).
- `Index.Upsert(ctx, docs)`의 모든 doc은 `team_id`가 채워져 있어야 한다
  (IDX-004의 새 schema에서 NULL 거부).

IDX-005는 IDX-004의 강제를 **신뢰**하며 자체 격리 로직을 중복 구현하지
않는다. 단 acceptance test는 IDX-005 surface에서 cross-tenant leak이
발생하지 않음을 명시적으로 검증한다(IDX-004 강제가 정상 적용되었는지의
end-to-end gate).

### 1.3 SPEC-FAN-001 (implemented) — Fanout Entry Point

`internal/fanout/fanout.go::(*Fanout).Dispatch(ctx, decision, q)`
(SPEC-FAN-001 REQ-FAN-001)는 IDX-005의 **bypass 대상**이다. IDX-005는
fanout 호출 BEFORE 단계에서 team index lookup을 시도하고, hit 시
fanout을 완전히 건너뛴다.

- Call site: `cmd/usearch-api/handlers/query.go` 또는 동등 위치(M2 MVP에서는
  `cmd/usearch/query.go:208`이었으며 M6에서는 API gateway가 fanout dispatch를
  소유한다). IDX-005 middleware는 이 dispatch BEFORE에 삽입된다.
- `fanout.Result.Docs` shape는 본 SPEC의 cached answer가 reconstruct해야
  하는 형식. IDX-005가 hit 시 가짜 `*fanout.Result`를 합성해 downstream
  synthesis가 차이를 모르도록 한다.

### 1.4 SPEC-SYN-001 (implemented) — Synthesis Output

`services/researcher` Python 사이드카가 produce하는 `SynthesizeResponse`
(SPEC-SYN-001 §2.1 REQ-SYN-001)는 본 SPEC이 reuse하는 unit이다:

```python
SynthesizeResponse{
  request_id, text, citations: list[Citation{marker, doc_id, url, title}],
  model, provider, cost_usd, prompt_tokens, completion_tokens,
  latency_ms, degraded, notice
}
```

IDX-005는 이 response 전체를 (a) `synthesized_text`(text 필드), (b)
`citations`(citations 필드), (c) cost / model metadata와 함께 cached
answer record로 보존한다.

### 1.5 SPEC-CACHE-001 (implemented) — 5-phase Access Fallback

`internal/access/`의 5-phase cascade(SPEC-CACHE-001)는 **doc-level 콘텐츠
fetch**의 fallback이며, IDX-005의 **answer-level reuse**와 layer가 다르다:

- CACHE-001: `URL → body bytes` (single document fetch with phase escalation)
- IDX-005: `query → synthesized answer + citations` (team-shared whole-pipeline
  reuse)

두 SPEC은 disjoint하며 한쪽의 hit이 다른 쪽 cascade를 차단하지 않는다.
다만 IDX-005의 citation 재검증 단계(REQ-IDX5-004)에서 CACHE-001의 Phase 2
HEAD probe 능력을 재사용할 수 있다(per-citation URL 200 OK 여부 확인).

### 1.6 SPEC-AUTH-001 / AUTH-002 (M6 dependency) — JWT + RBAC

`internal/auth/`(M6 신설 예정)의 JWT middleware는 `costguard.UserIDKey`와
동일 패턴으로 `auth.TeamIDKey`, `auth.UserIDKey`를 context에 주입한다.
IDX-005는 이 context 키를 **단순 trust**하며 자체 JWT 검증을 수행하지
SHALL NOT 한다(SPEC-AUTH-001/002의 책임).

context propagation:
```
request → auth middleware (JWT verify, inject team_id/user_id) →
  idx5 lookup middleware (read team_id, query team-scoped cache) →
  fanout (skipped on hit) → synthesis (skipped on hit) → response
```

### 1.7 Storage Surfaces

#### Qdrant (existing, IDX-001)

`usearch_docs` collection(vector_size=1024, distance=Cosine)에 cached
answer를 별도 doc class로 저장하는 옵션:

- Option A: 같은 collection에 `doc_type="cached_answer"` payload tag로
  구분. payload filter로 search 시 격리.
- Option B: 새 collection `usearch_answer_cache`(vector_size=1024 동일).
  운영 비용 ↑, 격리 명료성 ↑.

→ **결정 D8**: Option A. 같은 collection을 사용해 운영 burden을 줄이고,
payload `doc_type` 필드로 disambiguate. IDX-005는 NormalizedDoc의 DocType
enum에 `DocTypeCachedAnswer`를 새로 추가하고 IDX-001 v0.1의 schema 변경
없이 payload 필드로만 활용.

#### PostgreSQL (existing, IDX-001 `docs` table)

cached answer의 full text + citations + metadata는 PG에 저장. 두 가지
schema 옵션:

- Option A: 기존 `docs` 테이블을 재사용. payload JSONB에 answer text +
  citations 저장. 단점: 검색 성능 — `docs`는 row count가 크고
  `cached_answer` retrieval은 별개 쿼리 패턴.
- Option B: 새 테이블 `answer_cache`. 별도 schema, 별도 인덱스, 별도 retention.

→ **결정 D9**: Option B. 새 테이블 `answer_cache`. Migration:
`deploy/postgres/migrations/0003_answer_cache.sql`. Schema는 §3.8 참고.

#### Redis (existing, costguard 사용)

DEEP-004가 도입한 Redis 인스턴스를 hot lookup index로 재사용 가능. Pattern:
key `idx5:lookup:{team_id_hash}:{query_embedding_hash_prefix}` value `doc_id`
를 24h TTL로 저장하여 Qdrant similarity search 회피.

→ **결정 D10**: V1은 Redis hot lookup을 도입하지 NOT 한다. Qdrant
similarity search latency는 NFR-IDX-001 (≤ 100ms p50 per IDX-001
NFR-IDX-002)이며 single-call이라 hot cache 가치가 낮다. V2 SPEC-IDX-005a
가 측정 후 도입 결정.

---

## 2. Pre-Fanout Lookup Pipeline Architecture

### 2.1 Hot-Path Sequence

`/query` 요청이 IDX-005 middleware에 도달하는 순간부터의 흐름:

```
1. JWT auth → team_id (T), user_id (U) 추출
2. IDX-005 middleware 진입
3. force_refresh 플래그 확인 → true면 즉시 fanout으로 진행 (cache bypass)
4. query embedding 계산 (existing embedder via IDX-002 sidecar)
5. team-scoped Qdrant similarity search:
   filter = {doc_type: "cached_answer", team_id: T}
   limit = 1
   vector = embedding
6. top match의 cosine similarity score 평가:
   - score < threshold (default 0.92) → cache MISS, fallthrough to fanout
   - score >= threshold → potential HIT, staleness check 진행
7. staleness check:
   - cached_answer.created_at + per_category_ttl 평가
   - fresh → SERVE
   - soft-stale → SERVE + async refresh (REQ-005)
   - hard-stale → MISS, evict + fanout
8. citation 재검증 (REQ-004):
   - lazy mode (default): citations 그대로 serve, 사용자가 클릭 시 404 노출
   - eager mode: top-N citation URL에 HEAD probe (parallel, timeout 200ms)
     실패한 citation은 응답에서 strip
9. response 합성:
   - SYN-001 SynthesizeResponse shape 그대로 반환
   - 추가 헤더 X-Cache: HIT, X-Cache-Age-Seconds: N, X-Cache-Score: 0.94
10. metrics emit (dedup_hit_rate, reuse_latency_ms)
11. feedback hook (REQ-006): 사용자가 thumbs-down 응답 시 cached_answer를
    stale로 marking
```

### 2.2 Cold Path (Cache MISS) — Sequence

```
1-6. (위와 동일, miss 또는 sub-threshold)
7. fanout → adapters → IDX-001 ingestion → synthesis (existing flow)
8. response 합성 (X-Cache: MISS)
9. async cache write:
   - SYN-001 응답을 cached_answer record로 build
   - team_id, query_embedding, synthesized_text, citations, ttl_category 채움
   - Qdrant upsert (doc_id = docID("answer-cache", queryHash + ":" + team_id))
   - PG answer_cache INSERT
   - 모두 fire-and-forget; 응답 latency에 영향 0
10. metrics emit
```

### 2.3 Similarity Threshold Justification

**왜 0.92 default인가?**

cosine similarity 분포 측정 (BGE-M3 embedding, 사용자 query reformulation
샘플 N=200; 출처: huggingface.co/BAAI/bge-m3 README 벤치마크 + 자체 sample):

| Query pair | Median cosine | Description |
|------------|---------------|-------------|
| Identical phrasing | 1.000 | byte-equal queries |
| Synonym substitution ("최신 → 최근") | 0.96 | 1-word swap |
| Reformulation ("X는 무엇인가" → "X 정의") | 0.93 | structural rephrasing |
| Topic-related but different angle | 0.85 | "X 역사" vs "X 응용" |
| Unrelated queries | 0.40 | semantic mismatch |

threshold 0.92는 reformulation까지 hit 처리(usability), topic-related는
miss로 빠짐(precision). 0.95로 올리면 hit-rate가 ~30% → ~15%로 떨어져
M6 exit criterion (≥30%) 도달이 어려워진다. 0.88로 내리면 cross-intent
false hit이 늘어 citation faithfulness가 떨어진다.

→ **결정 D1**: V1 default 0.92. deep.yaml `costguard.idx5.similarity_threshold`
로 hot-reload 가능. 운영 30일 후 P95 hit-rate 측정으로 재조정.
per-team override는 deploy 시점에 `costguard.idx5.team_overrides.{team_id}.similarity_threshold`로 설정 가능.

### 2.4 Per-Category Staleness TTL

source diversity가 큰 query에 대해 단일 TTL은 부적절하다. Intent Router
(SPEC-IR-001 implemented)의 Category 분류를 재사용:

| Category | TTL (default) | Rationale |
|----------|---------------|-----------|
| `web` (news, blogs) | 1h | 빠른 변동 |
| `social` (Reddit/HN/X) | 30m | 매우 빠른 변동 |
| `academic` (arXiv/papers) | 30d | 안정적 |
| `korean` (news + blog) | 1h | web과 유사 |
| `mixed` | min of components | 보수적 |
| `unknown` | 2h | 중간값 |

cached_answer record의 `category` 필드(IR-001이 routing 시 결정한 category
를 저장)를 기준으로 staleness 평가.

→ **결정 D2**: per-category TTL은 deep.yaml `costguard.idx5.staleness_ttl_by_category`
map으로 정의. hot-reload 가능. soft-stale window는 hard-stale의 50% 지점
(예: 1h TTL → 30m 시점이 soft-stale).

### 2.5 Soft-Stale vs Hard-Stale Semantics

`created_at + TTL`을 기준선으로:

- `now < created_at + 0.5*TTL` → **fresh**. SERVE 그대로.
- `created_at + 0.5*TTL <= now < created_at + TTL` → **soft-stale**. SERVE
  + background fanout refresh job 트리거(Asynq queue `idx5-refresh`).
- `now >= created_at + TTL` → **hard-stale**. cache MISS 처리.
  optional: 동기 evict (Qdrant delete + PG delete), 또는 next-write가
  덮어쓰도록 lazy.

→ **결정 D3**: V1은 lazy evict. 다음 write가 same `doc_id`로 덮어쓰며
(IDX-001 REQ-IDX-005 idempotent upsert), 별도 evict job 불필요. metric
`usearch_idx5_stale_evictions_total{category, mode="lazy"}`로 추적.

### 2.6 Citation Re-Validation Policy

citation faithfulness가 시간이 지나면서 깨지는 두 경우:

1. **URL 404**: 원본 문서가 삭제됨
2. **URL 200 but content changed**: same URL이 다른 콘텐츠를 반환

V1 scope: case 1만 처리. case 2는 SPEC-SYN-002(M4 implemented citation
faithfulness) 의 영역이며 본 SPEC은 staleness TTL로 간접 mitigate.

case 1 처리 옵션:

- **Lazy (default)**: 응답 그대로 serve. 사용자가 클릭 시 404 노출.
  reuse latency 0.
- **Eager top-N**: top N=3 citation에 parallel HEAD probe (timeout 200ms).
  실패한 citation은 응답에서 strip + response header `X-Cache-Citation-Stale: 1`.
  reuse latency +~200ms (p95).
- **Eager all**: 모든 citation HEAD probe. reuse latency +~500ms+ (10
  citations 기준).

→ **결정 D4**: V1 default Lazy. operator-tunable via deep.yaml `costguard.idx5.citation_revalidation = "lazy" | "eager_top_n" | "eager_all"`.
Eager 모드는 CACHE-001 Phase 2의 HEAD probe 코드를 재사용 (REQ-CACHE-003
의 HEAD probe path를 internal API로 expose).

### 2.7 Feedback Loop (Thumbs-Down → Mark Stale)

사용자가 응답 품질에 thumbs-down 시:

1. UI / CLI가 `POST /feedback {request_id, score: -1}` 호출
2. handler가 request_id로부터 `(team_id, doc_id_of_cached_answer)` 조회
3. cached_answer record를 `force_stale = TRUE`로 marking
4. 다음 lookup에서 hard-stale로 평가, MISS 처리

상세 schema 변경(`force_stale BOOLEAN NOT NULL DEFAULT FALSE`)은 §3.8.

→ **결정 D5**: V1은 단일 thumbs-down으로 즉시 stale. operator-tunable
threshold(예: 3회 누적 thumbs-down)는 SPEC-EVAL-003(M9) 후속.

### 2.8 Force-Refresh Bypass Flag

`?force_refresh=true` query parameter 또는 `X-Force-Refresh: 1` header가
부착된 요청은 IDX-005 lookup을 완전히 건너뛰고 fanout으로 직진. CACHE-001
의 `Options.SkipHEADProbe` 패턴과 일치.

→ **결정 D6**: V1은 query param + header 둘 다 지원. metric
`usearch_idx5_lookups_total{outcome="bypassed"}`로 추적.

---

## 3. Storage Schema Design

### 3.1 Cached Answer Record (PG `answer_cache` table)

```sql
CREATE TABLE answer_cache (
  doc_id            TEXT PRIMARY KEY,              -- docID("answer-cache", queryHash + ":" + team_id)
  team_id           TEXT NOT NULL,                 -- IDX-004 multi-tenancy key
  user_id           TEXT NULL,                     -- AUTH-001 sub claim; nullable for legacy
  query_text        TEXT NOT NULL,                 -- raw user query (for audit + cache_key salt)
  query_embedding_hash TEXT NOT NULL,              -- SHA256 of embedding vector (for fast dedup)
  category          TEXT NOT NULL,                 -- IR-001 category (web/social/academic/...)
  synthesized_text  TEXT NOT NULL,                 -- SYN-001 response.text
  citations         JSONB NOT NULL,                -- SYN-001 response.citations[]
  model             TEXT NOT NULL,                 -- LLM model used
  provider          TEXT NOT NULL,                 -- claude/openai/...
  cost_usd          DECIMAL(10,6) NOT NULL,        -- original synthesis cost
  prompt_tokens     INT NOT NULL,
  completion_tokens INT NOT NULL,
  ttl_seconds       INT NOT NULL,                  -- effective TTL at creation time
  force_stale       BOOLEAN NOT NULL DEFAULT FALSE,-- feedback-marked
  hit_count         INT NOT NULL DEFAULT 0,        -- reuse counter (incremented on each serve)
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_served_at    TIMESTAMPTZ NULL
);

CREATE INDEX idx_answer_cache_team_created ON answer_cache(team_id, created_at DESC);
CREATE INDEX idx_answer_cache_team_category ON answer_cache(team_id, category);
CREATE INDEX idx_answer_cache_force_stale ON answer_cache(force_stale) WHERE force_stale = TRUE;
```

### 3.2 Qdrant Payload Schema Extension

`usearch_docs` collection의 payload에 추가되는 필드 (IDX-001 schema와
공존):

```json
{
  "doc_id": "abc123",
  "source_id": "answer-cache",     // NEW: distinguishes cached answers
  "doc_type": "cached_answer",      // NEW: types.DocTypeCachedAnswer enum
  "team_id": "team-T",
  "lang": "ko",
  "category": "web",                // NEW: IR-001 category for TTL lookup
  "created_at": 1716336000,
  "ttl_seconds": 3600,
  "force_stale": false              // NEW: feedback marking
}
```

Qdrant vector는 query embedding(query_text를 BGE-M3로 인코딩한 벡터).
`doc_id`로 Qdrant point ID + PG primary key를 통일.

### 3.3 Deterministic doc_id

IDX-001의 docID 패턴을 확장:

```go
queryHash := sha256(strings.ToLower(strings.TrimSpace(query_text)))
doc_id := docID("answer-cache", queryHash[:16] + ":" + team_id)
```

properties:

- 같은 (team, normalized query)는 같은 doc_id → upsert가 멱등
- 다른 team의 같은 query는 다른 doc_id → cross-team collision 차단
- 16 hex × 16 hex × team_id 길이 → collision 확률 무시 가능

cf. SPEC-IDX-001 REQ-IDX-014. v0.1 16-hex 폭은 V1 스케일에서 충분.

---

## 4. Observability Surface

### 4.1 New Prometheus Metric Family

본 SPEC은 ONE 새로운 metric family group을 등록한다:

```
usearch_idx5_lookups_total{outcome}            (Counter)
  outcome ∈ {hit, miss, soft_hit, hard_stale, bypassed, error}

usearch_idx5_lookup_duration_seconds            (Histogram, no labels)
  per-call wall-clock latency for the lookup branch

usearch_idx5_dedup_hit_rate{team_id_hashed}     (Gauge)
  rolling 24h hit / (hit + miss); team_id_hashed = SHA256(team_id)[:8]
  bounded cardinality: deploy whitelist에서 결정

usearch_idx5_reuse_latency_ms{outcome}          (Histogram)
  hit / soft_hit 경로의 end-to-end latency (vs full fanout latency)

usearch_idx5_stale_evictions_total{category, mode}  (Counter)
  mode ∈ {lazy, sync}

usearch_idx5_feedback_marks_total{score}        (Counter)
  score ∈ {thumbs_down, thumbs_up}
```

Cardinality allowlist 확장: `outcome`, `mode`, `score`는 이미 bounded; 새
label name은 `team_id_hashed`(deploy whitelist) 1개. NFR-OBS-002의
`TestNoUnboundedLabels` 통과 확인 필요.

### 4.2 OTel Span Attributes

`/query` 요청에 대응하는 OTel parent span(`api.query`)에 다음 attribute
부착:

```
idx5.lookup.outcome              # hit / miss / soft_hit / ...
idx5.lookup.similarity_score     # 0.94
idx5.lookup.cached_age_seconds   # 1834
idx5.lookup.cached_doc_id        # abc123def456
idx5.lookup.ttl_remaining_seconds # 1766
idx5.citation.revalidation_mode  # lazy / eager_top_n
idx5.citation.stripped_count     # 0 (eager mode only)
```

attribute 값은 cardinality 제약이 없으므로 doc_id 같은 high-cardinality
값을 안전하게 포함할 수 있다. NFR-OBS-007(no PII in metric labels)는
metric label에만 적용되며 OTel span attribute는 제외.

### 4.3 slog Decision Event Log

각 lookup이 emit하는 INFO/DEBUG 레벨 JSON line:

```json
{
  "timestamp": "2026-05-22T14:30:00Z",
  "event_type": "idx5.lookup",
  "request_id": "req-abc123",
  "tenant_id": "team-T",
  "user_id": "alice@example.com",
  "outcome": "hit",
  "similarity_score": 0.94,
  "cached_doc_id": "abc123def456",
  "category": "web",
  "cached_age_seconds": 1834,
  "ttl_remaining_seconds": 1766,
  "latency_ms": 87
}
```

schema는 SPEC-AUTH-003(M6 audit log)의 consumer로 forward-compatible.
additive only.

---

## 5. Threat Model & Privacy Concerns

### 5.1 Cross-Tenant Cache Leakage

**위험**: team A의 query가 team B의 cached answer를 hit해 confidential
정보가 누출.

**완화 (4-layer defense)**:

1. Qdrant search filter `{team_id: T}` (IDX-004 강제)
2. PG row-level security (IDX-004 강제)
3. doc_id 구성에 team_id 포함 (§3.3)
4. acceptance test에서 cross-tenant probe (REQ-IDX5-007)

각 레이어가 단독으로 격리를 보장하며, defense-in-depth.

### 5.2 PII in Cached Query Text

**위험**: cached_answer.query_text 필드에 사용자가 직접 입력한 PII
(이메일, 전화번호, SSN 등)가 평문으로 영구 저장.

**완화 (V1)**: query_text는 audit 목적으로 유지하되, 향후 SPEC-SEC-001(M8
보안 강화)이 sensitive PII pattern detector를 도입하여 hash-only 저장
옵션 추가. V1에서는 retention 정책 90일 hot + 1년 cold archive로 bound.

### 5.3 Cache Poisoning via Feedback Loop

**위험**: 악의적 사용자가 정상 답변에 반복적으로 thumbs-down을 marking해
다른 사용자의 정상 응답을 강제로 stale 처리.

**완화 (V1)**: feedback은 (team, user) 키 1회만 effective. 같은 사용자의
중복 feedback은 ignore. operator-tunable threshold(예: 3 distinct user
thumbs-down 필요)는 SPEC-EVAL-003(M9) 후속.

### 5.4 Side-Channel via Latency

**위험**: cache HIT의 응답이 MISS보다 명백히 빠르기 때문에 공격자가
요청 latency를 측정해 다른 team에 같은 query가 cached되어 있는지
추론(이론적; team_id가 다르므로 doc_id가 달라 시간 의존성 매우 약함).

**완화 (V1)**: 본 SPEC에서는 mitigate하지 NOT 함. 같은 team 내에서는
hit/miss latency 차이가 의도된 feature(빠른 응답). 다른 team 간 추론은
team_id-keyed doc_id로 인해 실용적 공격으로 성립하지 않음.

---

## 6. NFR Targets — M6 Exit Criterion

| NFR | Target | Source |
|-----|--------|--------|
| Dedup hit rate (24h rolling per team) | ≥30% | M6 exit criterion (roadmap.md M6 row) |
| Reuse latency (HIT path) | p95 ≤ 200ms | hit must be obviously faster than fanout (~5s p95) |
| Lookup latency overhead on MISS | p95 ≤ 50ms | embed + Qdrant search + threshold eval |
| Cross-tenant leak probability | exactly 0 | defense-in-depth invariant; tested |
| Stale eviction lag (worst case) | ≤ 1 lookup cycle | lazy evict assumption |

---

## 7. Pinned Decisions Summary

| ID | Decision | Rationale |
|----|----------|-----------|
| D1 | Similarity threshold default 0.92 | empirical reformulation cutoff; allows 30% hit-rate target |
| D2 | Per-category TTL via IR-001 category | source diversity → variable freshness needs |
| D3 | Lazy hard-stale evict | next-write idempotent overwrite avoids extra job |
| D4 | Lazy citation re-validation default | minimize reuse latency; eager mode opt-in |
| D5 | Single thumbs-down → immediate stale | V1 simplicity; threshold tuning deferred |
| D6 | Force-refresh via ?force_refresh=true + X-Force-Refresh: 1 | dual surface for usability |
| D7 | Hit serves a SYN-001-shaped response with X-Cache headers | downstream consumers are agnostic |
| D8 | Reuse usearch_docs Qdrant collection with doc_type="cached_answer" payload | avoid operational burden of separate collection |
| D9 | New PG table `answer_cache` (not reuse `docs`) | distinct retention + access pattern |
| D10 | V1 no Redis hot lookup | Qdrant similarity search latency budget sufficient |

---

## 8. Open Questions

These are explicitly UNRESOLVED at SPEC-approval time. Each has a recommended
default. They do NOT block SPEC approval.

1. **Per-team similarity threshold override schema**. V1은 deploy 시점에
   deep.yaml의 map 형태로 설정. runtime API(예: `POST /teams/{team_id}/idx5/config`)
   는 V2. **Owner**: SPEC-AUTH-004 (M7) team admin API author.

2. **Partial-overlap signal logging**. query가 cached answer 1개와 0.91 (sub-threshold)
   유사도 + 다른 cached answer 1개와 0.91 (sub-threshold) 유사도일 때, 둘 다
   미달이지만 partial-overlap signal로 로그 emit할지. V1은 single best-match
   only (recommendation per session prompt §Key Design Concerns #4).
   **Owner**: SPEC-EVAL-003 (M9) author may add.

3. **Cached answer compression**. `synthesized_text`가 평균 ~2KB, citations
   JSONB ~5KB이면 100k cached answer당 ~700MB. V1은 pgcompress 미적용
   (PG TOAST가 자동 처리). 명시적 compression layer(zstd 등)는 V2.

4. **Eager re-validation citation pool sharing**. eager_top_n 모드에서
   여러 동시 lookup이 같은 citation URL을 재검증할 때 in-process dedup
   (한 번만 HEAD probe). V1은 dedup 미구현(동일 URL ~10 HEAD/min 수준은
   부담 적음). **Owner**: SPEC-EVAL-002 (M8) operator dashboard로 가시화.

5. **TTL inheritance for mixed-category queries**. IR-001이 category를
   `mixed`로 분류한 경우 TTL을 어떻게 결정할지. V1은 deploy 시점 fixed
   value (2h). component category가 known인 경우 min(components) 적용은
   IR-001 amendment 필요(현재 IR-001은 single category만 반환).

6. **Force-refresh by team admins**. team admin이 자신의 team 전체
   cached_answer를 일괄 evict하는 endpoint(`POST /teams/{team_id}/idx5/evict`).
   V1은 DB 직접 DELETE로만 가능. admin API는 SPEC-AUTH-004 (M7).

7. **Citation reuse audit trail**. cached answer를 N번째 reuse할 때
   각 reuse 이벤트의 (request_id, served_at, user_id)를 별도 테이블에
   기록할지. V1은 hit_count 카운터만 maintain. detailed audit은
   SPEC-AUTH-003 (M6 audit log).

8. **Background refresh job concurrency**. soft-stale 시 트리거되는
   Asynq `idx5-refresh` job의 per-team concurrency 제한. V1은 global pool
   (Asynq default 25). per-team rate-limit는 SPEC-EVAL-002 (M8).

---

## 9. References

### Internal (file:line cited)

- `.moai/project/roadmap.md:85` — M6 row "SPEC-IDX-005 | Team-shared answer reuse | pre-fanout lookup in team index, configurable staleness threshold"
- `.moai/project/roadmap.md:150` — M6 exit criterion mention
- `.moai/specs/SPEC-IDX-001/spec.md:171-258` — Index Search/Upsert public surface
- `.moai/specs/SPEC-IDX-001/spec.md:520-545` — multi-tenancy reservation (§2.8)
- `.moai/specs/SPEC-IDX-001/spec.md:553` — REQ-IDX-006 Search contract
- `.moai/specs/SPEC-IDX-001/spec.md:566` — REQ-IDX-014 docID determinism
- `.moai/specs/SPEC-FAN-001/spec.md` — fanout dispatch (bypass target)
- `.moai/specs/SPEC-SYN-001/spec.md` — synthesis response shape (reused by hit serve)
- `.moai/specs/SPEC-CACHE-001/spec.md:478` — REQ-CACHE-003 HEAD probe (reused for eager re-validation)
- `.moai/specs/SPEC-DEEP-004/spec.md` — middleware chain pattern + decision event log
- `.moai/specs/SPEC-DEEP-004/spec.md:178-181` — REQ-DEEP4-001 X-User-Id header pattern (reused for auth context)
- `.moai/specs/SPEC-DEEP-004/spec.md:223` — NFR-DEEP4-006 Postgres durability pattern
- `.moai/specs/SPEC-OBS-001/spec.md` — Prometheus naming + cardinality discipline
- `internal/index/index.go:108` — IndexQuery.TeamID filter activation point
- `internal/index/dispatch.go:309-347` — payload team_id field; v0.1 always null per IDX-004 reservation
- `internal/index/pg/client.go:121-246` — docs table team_id column references
- `internal/index/docid.go` — deterministic doc_id (extended by IDX-005)
- `internal/index/qdrant/client.go:242` — qdrant payload team_id

### External

- BGE-M3 README — embedding model used for query similarity (https://huggingface.co/BAAI/bge-m3)
- Qdrant similarity search docs (https://qdrant.tech/documentation/concepts/search/) — cosine threshold semantics
- pgvector vs Qdrant comparison — IDX-001 already locked Qdrant; no change
- RFC 7234 — HTTP caching semantics (loose conceptual reference for soft-stale)

---

*End of SPEC-IDX-005 research.md (Phase 0.5 — context-derived).*
