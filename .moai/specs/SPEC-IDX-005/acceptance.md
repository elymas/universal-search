# SPEC-IDX-005 Acceptance Scenarios

Generated: 2026-05-22
Format: Given / When / Then (Korean prose, English identifiers)

본 문서는 SPEC-IDX-005의 acceptance scenarios 8건 + boundary edge case 2건
을 정의한다. 각 시나리오는 spec.md §5에서 참조되며 plan-auditor가 SPEC PASS
평가의 근거로 사용한다. §5.5(M6 exit gate)는 본 SPEC의 PRIMARY DRIVER이며
integration test로 실제 측정된다.

---

## §5.1 team T 사용자가 fresh cached answer hit → 200 + X-Cache: HIT

**Coverage**: REQ-IDX5-001, REQ-IDX5-002, REQ-IDX5-007

### Given

- deep.yaml `costguard.idx5.similarity_threshold: 0.92`,
  `staleness_ttl_by_category.web: 3600`.
- 30분 전에 team_id="team-T"의 다른 사용자(bob)가 query "양자 컴퓨팅 최신
  진전"을 fanout으로 호출하여 cached_answer record가 PG `answer_cache` +
  Qdrant `usearch_docs` 에 저장됨 (category="web", ttl_seconds=3600,
  force_stale=FALSE).
- 호출자 alice@example.com이 동일 team의 사용자, JWT 토큰이 정상 발급됨
  (AUTH-001 미들웨어가 team_id="team-T", user_id="alice@example.com"을
  context에 inject).
- 호출자의 query: "양자 컴퓨팅 최신 진척 상황" (bob의 query와 reformulation
  관계, 측정된 cosine similarity = 0.94).

### When

호출자가 `POST /query` 요청 (Authorization: Bearer <JWT>, body: {"q": "양자
컴퓨팅 최신 진척 상황"}).

### Then

- HTTP 응답 status는 **200 OK**.
- 응답 헤더:
  - `X-Cache: HIT`
  - `X-Cache-Age-Seconds: 1800` (= 30분)
  - `X-Cache-Score: 0.94`
- 응답 body는 SYN-001 `SynthesizeResponse` JSON shape (text + citations +
  model + provider + cost_usd + tokens 등) — bob의 응답과 동일.
- IDX-001 `index.Search`는 **정확히 1회** 호출됨
  (`IndexQuery{TeamID:"team-T", DocTypes:[DocTypeCachedAnswer], MaxResults:1}`).
- `internal/fanout.Dispatch`는 **호출되지 않음**.
- `services/researcher` Python 사이드카는 **호출되지 않음**.
- PG `answer_cache.hit_count`는 1 증가 (async UPDATE, response latency에
  영향 없음).
- PG `answer_cache.last_served_at`는 NOW()로 업데이트.
- Prometheus 카운터 `usearch_idx5_lookups_total{outcome="hit"}` 1 증가.
- Histogram `usearch_idx5_reuse_latency_ms{outcome="hit"}`에 측정값 (~120ms)
  observation.
- Decision event log line이 stderr로 출력:
  ```json
  {"timestamp":"...","event_type":"idx5.lookup","request_id":"...","tenant_id":"team-T","user_id":"alice@example.com","outcome":"hit","similarity_score":0.94,"cached_doc_id":"abc123","category":"web","cached_age_seconds":1800,"ttl_remaining_seconds":1800,"latency_ms":120}
  ```

---

## §5.2 team T 사용자가 soft-stale hit → 200 serve + 비동기 refresh enqueue

**Coverage**: REQ-IDX5-001, REQ-IDX5-002, REQ-IDX5-003, REQ-IDX5-005

### Given

- §5.1과 동일 setup, 단 cached_answer는 50분 전 (3000초 전)에 작성됨.
  `category="web"`, `ttl_seconds=3600`. soft-stale boundary는 `0.5 * 3600 =
  1800초` 시점.
- age = 3000 > 1800, age < 3600 → soft-stale.
- 호출자 alice의 query similarity = 0.93.

### When

호출자가 `POST /query` 요청 (위와 동일).

### Then

- HTTP 응답 status는 **200 OK**.
- 응답 헤더:
  - `X-Cache: SOFT-HIT`
  - `X-Cache-Age-Seconds: 3000`
  - `X-Cache-Score: 0.93`
- 응답 body는 cached `SynthesizeResponse` (이미 PG에 저장된 값).
- Asynq queue `idx5-refresh`에 작업 enqueue:
  ```json
  {"team_id":"team-T","query_text":"양자 컴퓨팅 최신 진척 상황","original_doc_id":"abc123","category":"web"}
  ```
- 비동기 worker가 background에서 fanout + synthesis 실행하여 same doc_id로
  IDX-001 Upsert (idempotent overwrite). 이 작업은 **응답 latency에 영향
  없음**.
- 사용자는 30분 후 다시 호출하면 새로 refresh된 cached_answer를 받는다.
- Prometheus 카운터 `usearch_idx5_lookups_total{outcome="soft_hit"}` 1 증가.
- worker가 실패하면 다음 lookup에서 hard-stale로 평가되어 자연 fallback.

---

## §5.3 team T 사용자가 hard-stale → MISS + fanout fall-through + write

**Coverage**: REQ-IDX5-002, REQ-IDX5-003, REQ-IDX5-006

### Given

- §5.1과 동일 setup, 단 cached_answer는 90분 전 (5400초 전)에 작성.
  `category="web"`, `ttl_seconds=3600`. age = 5400 > 3600 → hard-stale.
- 호출자 alice의 query similarity = 0.94 (high score지만 staleness가
  더 strict).

### When

호출자가 `POST /query` 요청.

### Then

- HTTP 응답 status는 **200 OK** (단, fanout 경로의 normal latency ~3s).
- 응답 헤더:
  - `X-Cache: MISS`
- 응답 body는 새로 생성된 `SynthesizeResponse` (fanout + synthesis 실행).
- `internal/fanout.Dispatch` 호출됨.
- `services/researcher` Python 사이드카 호출됨.
- 응답 직후 비동기 write가 발생:
  - Qdrant `usearch_docs` collection에 point upsert (same doc_id로
    overwrite, force_stale=FALSE 리셋, created_at=NOW(), ttl_seconds=3600).
  - PG `answer_cache` INSERT ON CONFLICT (doc_id) DO UPDATE row.
- Prometheus 카운터 `usearch_idx5_lookups_total{outcome="hard_stale"}` 1 증가.
- Prometheus 카운터 `usearch_idx5_stale_evictions_total{category="web",
  mode="lazy"}` 1 증가 (다음 write가 overwrite로 evict).

---

## §5.4 sub-threshold similarity → MISS + fanout + write

**Coverage**: REQ-IDX5-001, REQ-IDX5-002

### Given

- §5.1과 동일 setup, 단 호출자의 query가 cached_answer와 의미적으로 다름.
  Qdrant similarity search top-1 score = 0.85 (threshold 0.92 미달).
- 호출자의 query: "양자 컴퓨팅 기초 원리 설명" (cached "양자 컴퓨팅 최신 진전"
  과 topic-related지만 다른 angle).

### When

호출자가 `POST /query` 요청.

### Then

- HTTP 응답 status는 **200 OK** (fanout normal latency).
- 응답 헤더:
  - `X-Cache: MISS`
- 응답 body는 새 fanout 결과의 `SynthesizeResponse`.
- `internal/fanout.Dispatch` 호출됨.
- 응답 직후 새 cached_answer record가 PG + Qdrant에 INSERT (다른 doc_id,
  같은 team_id).
- Prometheus 카운터 `usearch_idx5_lookups_total{outcome="miss"}` 1 증가.
- OTel span attribute `idx5.lookup.similarity_score = 0.85`로 기록.

---

## §5.5 M6 EXIT GATE: 100 query synthetic 트래픽 dedup hit-rate ≥30% 측정

**Coverage**: NFR-IDX5-001, REQ-IDX5-009 (PRIMARY M6 DRIVER)

### Given

- Integration test 환경: testcontainers Qdrant + PG + Redis + Asynq +
  stub LiteLLM + stub adapters.
- 100개 query 시퀀스 (synthetic, fixture file `internal/idx5/testdata/
  m6_synthetic_traffic.json`):
  - 35건은 single-team 내 5개 unique "base query" 각각에 대한 7개
    reformulation (예: "AI 안전 연구 최신" vs "AI safety 최근 연구" 등).
    각 reformulation pair는 측정된 cosine ≥ 0.92.
  - 35건은 같은 base query의 반복 호출 (cosine = 1.0).
  - 30건은 distinct query (cosine 모두 < 0.85).
- 첫 번째 base query 호출은 cold (MISS); 후속 reformulation/반복은 hit
  가능.
- TTL은 short window (300초) 설정하여 staleness가 발생하지 않도록 보장.
- `team_id="team-T"` 고정.

### When

테스트가 100개 query를 순차적으로 `POST /query`로 호출 (각 호출 후 1초
delay로 timestamp 분산). 첫 호출은 항상 MISS, 후속은 hit 가능 상태.

### Then

- 100개 호출이 모두 200 OK 응답.
- Prometheus snapshot 후 다음 계산:
  ```
  hits = usearch_idx5_lookups_total{outcome="hit"} + {outcome="soft_hit"}
  total = sum(usearch_idx5_lookups_total{outcome=~"hit|soft_hit|miss|hard_stale"})
  hit_rate = hits / total
  ```
- **assertion**: `hit_rate >= 0.30`.
  - 정확히는: 35 reformulation hit + 30 (35 반복 - 5 base) ≈ 65 hits
    중 5 base는 MISS, 30 distinct는 MISS → 100건 중 65 hit / 100 total
    = **0.65** (margin 0.35 over threshold 0.30).
- 만약 hit_rate < 0.30이면 본 SPEC의 implementation이 M6 GA gate를 통과
  하지 못하는 것이므로 PR 머지 거부.
- Prometheus 게이지 `usearch_idx5_dedup_hit_rate{team_id_hashed="<hash>"}`
  도 ≥ 0.30 emit.

### Why this matters

본 시나리오가 SPEC-IDX-005의 raison d'être다. 30% threshold 미달 시
M6 GA 조건 미충족. integration test 자체가 PR merge gate.

---

## §5.6 Cross-tenant probe: team U가 team T의 cached answer reuse 시도 → MISS 강제

**Coverage**: REQ-IDX5-007, NFR-IDX5-004 (CRITICAL SECURITY INVARIANT)

### Given

- team_id="team-T"의 사용자가 query "company internal roadmap 2026 Q3"
  를 호출하여 confidential cached_answer가 저장됨. team_id="team-T",
  doc_id 구성에 "team-T" 포함.
- team_id="team-U"의 다른 사용자(eve)가 같은 query 또는 reformulation을
  호출.

### When

team-U 사용자가 정상 JWT (team_id="team-U")로 `POST /query` 요청, body는
team-T cached answer와 cosine ≥ 0.92인 query.

### Then

- IDX-005 lookup은 `index.Search(ctx, IndexQuery{TeamID:"team-U",
  DocTypes:[DocTypeCachedAnswer]})` 호출.
- IDX-004 multi-tenancy 강제로 인해:
  - Qdrant payload filter `team_id == "team-U"`가 team-T row 제외.
  - PG row-level security가 team-T row 제외.
- lookup 결과는 **빈 슬라이스** (또는 미달 score의 team-U own record만).
- IDX-005 분기: MISS → fanout fall-through.
- HTTP 응답: **MISS 경로의 정상 fanout 응답** (eve의 자체 fanout 결과).
- eve는 team-T의 confidential 정보에 접근하지 **못함**.
- 응답 헤더: `X-Cache: MISS`.
- doc_id 검증: `docID("answer-cache", queryHash + ":team-U")`와
  `docID("answer-cache", queryHash + ":team-T")`는 항상 다름 (다른 suffix).

### Why this matters

NFR-IDX5-004의 invariant("정확히 0 cross-tenant leak")의 acceptance test.
4-layer defense의 정상 작동을 end-to-end로 검증.

---

## §5.7 feedback thumbs-down → 다음 lookup에서 hard-stale → fanout 재실행

**Coverage**: REQ-IDX5-005, REQ-IDX5-008

### Given

- team T에 fresh cached_answer 존재 (age=300초, TTL=3600초, force_stale=FALSE).
- 사용자 alice가 cached answer를 hit하여 응답을 받음 (§5.1 path).
- alice가 응답 품질에 불만, request_id를 가지고 `POST /feedback`
  endpoint 호출.

### When

```
POST /feedback
Authorization: Bearer <alice's JWT>
Body: {"request_id":"req-abc123","score":-1}
```

### Then

- IDX-005 feedback handler가 in-memory LRU에서 request_id → cached_doc_id
  매핑 조회 (LRU는 lookup serve 시점에 저장됨, TTL 24h).
- handler가 PG에 `UPDATE answer_cache SET force_stale = TRUE WHERE doc_id
  = 'abc123' AND team_id = 'team-T'` 실행 (team boundary 확인).
- HTTP 응답: 200 OK + body `{"status":"marked_stale"}`.
- Prometheus 카운터 `usearch_idx5_feedback_marks_total{score="thumbs_down"}`
  1 증가.
- alice가 즉시 같은 query를 다시 호출:
  - IDX-005 lookup이 cached_answer를 retrieve. similarity score = 1.0
    (identical query).
  - staleness evaluator: `force_stale = TRUE` → **hard-stale 강제**.
  - 분기: MISS → fanout fall-through (정상 응답).
  - 응답 헤더: `X-Cache: MISS`.
  - 새 cached_answer가 same doc_id로 overwrite (force_stale=FALSE 리셋).
- 만약 alice가 24h 내 동일 feedback을 중복 POST하면:
  - 핸들러는 idempotent UPDATE 실행 (이미 force_stale=TRUE).
  - 응답 200 OK.

---

## §5.8 citation re-validation eager_top_n 모드: 404 citation 응답에서 strip

**Coverage**: REQ-IDX5-004

### Given

- deep.yaml `costguard.idx5.citation_revalidation: "eager_top_n"`, default
  N=3.
- fresh cached_answer가 있고 citation 5개를 포함. 첫 번째 citation의 URL
  은 30일 전 작성된 블로그 글로 현재 410 Gone 반환. 나머지 4개는 200 OK.

### When

호출자가 hit 가능한 query를 `POST /query`로 호출.

### Then

- IDX-005 lookup이 cached_answer retrieve, threshold + staleness 통과.
- serve 직전 citation_revalidate가 top-3 citation에 parallel HEAD probe:
  - citation 1 URL → 410 → strip.
  - citation 2 URL → 200 → keep.
  - citation 3 URL → 200 → keep.
  - (citation 4, 5는 probe 안 함, 그대로 keep)
- 응답 body의 `citations` 배열은 4개 entry (citation 1만 제거).
- 응답 body의 `text` 필드의 marker `[1]`은 (citation 1 strip 후) 의미적으로
  떨어진 상태 — V1은 marker renumbering 없음(downstream consumer가 처리).
- 응답 헤더:
  - `X-Cache: HIT`
  - `X-Cache-Citation-Stale: 1`
- probe wall-clock latency ≈ 200ms p95 (parallel).
- 만약 citation 1이 timeout (5xx, network error)이면 strip하지 않음.

---

## Edge Case 1 — force_refresh=true 우회: hit 가능 상태에서도 fanout 직진

**Coverage**: REQ-IDX5-001 (boundary)

### Given

- §5.1과 동일 fresh cached_answer 상태.
- 호출자 alice가 force_refresh를 명시:
  - 옵션 A: query param `?force_refresh=true`
  - 옵션 B: header `X-Force-Refresh: 1`

### When

호출자가 `POST /query?force_refresh=true` 요청.

### Then

- IDX-005 middleware는 force_refresh 플래그를 확인하고 lookup을 **완전히
  skip**.
- embedder.Embed는 **호출되지 않음**.
- `index.Search`는 **호출되지 않음**.
- fanout이 직접 실행됨 (cache hit 가능한 상태에서도).
- HTTP 응답: fanout normal path 응답.
- 응답 헤더: `X-Cache: BYPASSED`.
- 응답 직후 새 cached_answer가 same doc_id로 overwrite (force_stale=FALSE,
  새 created_at).
- Prometheus 카운터 `usearch_idx5_lookups_total{outcome="bypassed"}` 1 증가.

### Why this matters

운영자나 사용자가 명시적으로 cache invalidation을 trigger할 수 있는 escape
hatch. 다른 force-refresh 경로(POST /feedback, scheduled refresh)와
독립적으로 작동.

---

## Edge Case 2 — TTL boundary: age = exactly TTL → hard-stale 분기

**Coverage**: REQ-IDX5-003 (boundary atomicity)

### Given

- cached_answer record: created_at = T, ttl_seconds = 3600.
- 현재 시점 NOW() = T + 3600.000s (정확히 boundary).

### When

호출자가 cached_answer hit 가능한 query를 호출.

### Then

- staleness evaluator: `age = 3600 >= ttl = 3600` → **hard-stale**.
- 분기: MISS → fanout fall-through (precision: ≥ 사용, > 아님).
- 만약 age = 3599.999s (정확히 1ms 부족)이었다면 soft-stale 분기.
- 만약 age = 3600.001s이었다면 hard-stale (위와 동일).

### Why this matters

이 boundary는 단순한 ≥/> 차이가 staleness 정확도에 미치는 영향을 검증.
spec의 "age >= ttl → hard-stale" 의미가 정확히 구현되어야 하며 ">"로
구현되면 정확히 1초 동안 stale data를 serve하는 leak이 발생.

---

## Acceptance Coverage Matrix

| Scenario | REQ-001 | REQ-002 | REQ-003 | REQ-004 | REQ-005 | REQ-006 | REQ-007 | REQ-008 | REQ-009 | REQ-010 |
|----------|---------|---------|---------|---------|---------|---------|---------|---------|---------|---------|
| §5.1 | ✓ | ✓ | | | | | ✓ | | ✓ | ✓ |
| §5.2 | ✓ | ✓ | ✓ | | ✓ | | | | ✓ | ✓ |
| §5.3 | | ✓ | ✓ | | | ✓ | | | ✓ | |
| §5.4 | ✓ | ✓ | | | | ✓ | | | ✓ | |
| §5.5 | | | | | | | | | ✓ | |
| §5.6 | | | | | | | ✓ | | ✓ | |
| §5.7 | | | ✓ | | ✓ | | | ✓ | ✓ | |
| §5.8 | | | | ✓ | | | | | ✓ | |
| Edge1 | ✓ | | | | | | | | ✓ | |
| Edge2 | | | ✓ | | | | | | | |

- §5.5 는 NFR-IDX5-001 (M6 exit gate)를 PRIMARY로 검증.
- §5.6 는 NFR-IDX5-004 (cross-tenant zero leak)를 PRIMARY로 검증.
- 모든 시나리오가 REQ-IDX5-009 (observability)를 indirectly 검증 (metric
  emission).

---

## M6 GA Criteria — Final Gate

본 acceptance.md의 모든 시나리오가 PASS 시 다음을 추가 확인 후 M6 GA 진입:

1. §5.5 PASS — dedup hit rate ≥30% on synthetic traffic
2. §5.6 PASS — cross-tenant leak = 0
3. Edge2 PASS — boundary atomicity
4. integration test가 -race 모드로 통과 (NFR-IDX5-006)
5. LSP gate: zero errors / zero type errors / zero lint errors
6. Coverage ≥85% on `internal/idx5/`
7. plan-auditor cycle 1+회 통과 (HIGH 0, MEDIUM ≤2, NIT 무한)

위 모두 PASS 시 M6 release candidate tag.

---

*End of SPEC-IDX-005 acceptance.md.*
