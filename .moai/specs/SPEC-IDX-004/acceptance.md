# SPEC-IDX-004 Acceptance Scenarios

Generated: 2026-05-22
Format: Given / When / Then (Korean prose, English identifiers)

본 문서는 SPEC-IDX-004의 acceptance scenarios 8건 + boundary edge case 2건
을 정의한다. 각 시나리오는 spec.md §5에서 참조되며 plan-auditor가 SPEC PASS
평가의 근거로 사용한다. §5.6(M6 cross-team isolation gate)은 본 SPEC의
CRITICAL SECURITY INVARIANT이며 integration test로 실제 측정된다.

---

## §5.1 team T 사용자 enforced mode upsert + search round-trip → team_id 정확히 박힘

**Coverage**: REQ-IDX4-001, REQ-IDX4-002, REQ-IDX4-003, REQ-IDX4-004

### Given

- `INDEX_MULTI_TENANCY_MODE=enforced` (v1.0 default).
- AUTH-001의 JWT 미들웨어가 ctx에 `authctx.TeamIDKey="team-T"`,
  `authctx.UserIDKey="alice@example.com"`를 inject (또는 AUTH 미ship 환경
  에서는 `INDEX_DEFAULT_TEAM="team-T"` env var).
- IDX-004의 Qdrant `EnsureCollection`이 이미 booting 시점에 `team_id`
  payload index를 `is_tenant=true`로 생성한 상태.
- 신규 doc: `{doc_id: "doc-100", body: "샘플 문서", metadata: {}}`.

### When

caller가 `index.Upsert(ctx, []NormalizedDoc{doc})` 호출.

### Then

- PG `docs` 테이블에 row INSERT, `team_id="team-T"`, `user_id=NULL` (default
  visibility=team_shared).
- Qdrant point의 payload에 `{team_id: "team-T"}` 포함 (이전 v0.1의
  `nil` 박힘이 사라짐).
- Meili document의 `team_id` 필드에 `"team-T"` 박힘 (이전 v0.1의 `nil` 사라짐).
- 직후 `index.Search(ctx, IndexQuery{TeamID:"team-T", Query:"샘플"})` 호출
  → doc-100을 정확히 반환.
- Qdrant search filter expression에 `must: [{key: "team_id", match: {value:
  "team-T"}}]` 합성됨.
- Meili search 호출이 admin API key 대신 tenant token 사용 (REQ-008).
- caller가 `metadata: {team_id: "team-EVIL"}`를 미리 set한 경우에도 IDX-004
  가 silent overwrite → 최종 doc의 team_id는 `"team-T"` (ctx 값). WARN slog
  `event_type: "idx4.upsert.team_id_overridden"` emit.

---

## §5.2 Meili tenant token 발급 + cache 재사용 + Korean shard에도 적용

**Coverage**: REQ-IDX4-008, REQ-IDX4-009, NFR-IDX4-003

### Given

- §5.1과 동일 setup. token cache는 cold state.
- 같은 team_id="team-T", user_id="alice@example.com"으로 두 번 연속 Search
  호출.

### When

첫 번째 `index.Search(ctx, IndexQuery{TeamID:"team-T", Query:"문서"})`
→ token cache miss → `meilisearch-go.GenerateTenantToken(apiKeyUID,
searchRules, &TenantTokenOptions{ExpiresAt: now+15m})` 호출 → cache에
저장 → Meili 호출.

두 번째 호출 (같은 ctx) → token cache hit → 동일 token 재사용 → Meili 호출.

### Then

- 발급된 JWT의 `searchRules`:
  ```json
  {
    "usearch_docs": {"filter": "team_id = \"team-T\""},
    "usearch_docs_ko": {"filter": "team_id = \"team-T\""}
  }
  ```
- JWT 서명은 HMAC-SHA256, admin API key UID로 서명.
- Korean query (e.g. "한글 문서 검색")의 경우 `usearch_docs_ko` 인덱스에도
  같은 token으로 호출 → tenant boundary 정확히 유지.
- 두 번째 호출의 Meili Authorization 헤더는 첫 번째와 동일한 토큰 (cache
  hit).
- 발급은 정확히 1회 (`sync.Once` 보호).
- Prometheus 카운터 `usearch_index_tenant_token_issued_total{tier="meili",
  outcome="success"}` 1 증가 (첫 호출만).
- token TTL = 900초 (15분). 만료 60초 전 background refresh worker가
  새 토큰을 미리 발급 → cache hit-rate ≥ 99% 유지 (NFR-IDX4-003).
- Upsert 경로는 admin API key 직접 사용 (tenant token 미사용; REQ-008).

---

## §5.3 personal context tier: user_private adapter doc이 ingestor만 보임

**Coverage**: REQ-IDX4-002, REQ-IDX4-006

### Given

- AUTH-002 visibility hook 가정: GitHub PAT adapter가 `Visibility()` →
  `user_private` 반환.
- user_id="alice@example.com"이 GitHub private repo doc-200을 ingest
  → IDX-004가 doc payload에 `team_id="team-T", user_id="alice@example.com"`
  set.
- 같은 team의 다른 user_id="bob@example.com"이 같은 team_id로 search 호출.

### When

bob의 `index.Search(ctx={team_id:"team-T", user_id:"bob@example.com"},
IndexQuery{TeamID:"team-T", UserID:"bob@example.com", IncludePrivate:true})`.

### Then

- Qdrant filter expression:
  ```
  must: [
    {key: "team_id", match: {value: "team-T"}},
    {should: [
      {key: "user_id", match: {value: "bob@example.com"}},
      {key: "user_id", match: {value: ""}}
    ]}
  ]
  ```
- doc-200은 결과에서 제외됨 (`user_id="alice@example.com"` ≠ `"bob@example.com"`
  AND ≠ `""`).
- alice의 동일 호출은 doc-200을 정상 반환.
- PG `docs` 테이블도 partial index `idx_docs_team_user WHERE user_id IS
  NOT NULL` 이용하여 efficient lookup.

---

## §5.4 __public__ sentinel doc이 모든 team retrieval에 합성됨

**Coverage**: REQ-IDX4-006, REQ-IDX4-007

### Given

- arXiv adapter가 `Visibility() → public` 반환.
- IDX-004가 arXiv doc doc-300의 payload에 `team_id="__public__"` set.
- 두 다른 team_id="team-T", team_id="team-U" 사용자가 `IncludePublic:true`
  옵션으로 search 호출.

### When

team-T 사용자의 `index.Search(ctx, IndexQuery{TeamID:"team-T",
IncludePublic:true, Query:"arxiv 양자 컴퓨팅"})`.
team-U 사용자의 `index.Search(ctx, IndexQuery{TeamID:"team-U",
IncludePublic:true, Query:"arxiv 양자 컴퓨팅"})`.

### Then

- 두 호출 모두 doc-300을 반환.
- Qdrant filter expression:
  ```
  must: [{should: [
    {key: "team_id", match: {value: "team-T"}},  // 또는 team-U
    {key: "team_id", match: {value: "__public__"}}
  ]}]
  ```
- team-T 사용자가 caller로서 `q.TeamID = "__public__"`를 직접 설정한 경우
  REQ-IDX4-007에 의해 즉시 거절 (filter builder validation).
- `INDEX_DEFAULT_TEAM=__public__` env var 설정 시 process startup
  validation error로 거절.

---

## §5.5 NFR regression: 100-query mixed-team 트래픽 p95 ≤ 10% degradation

**Coverage**: NFR-IDX4-003, NFR-IDX4-006

### Given

- Integration test 환경: testcontainers Qdrant + Meili + PG.
- 5 team (team-A, B, C, D, E), 각 team 200 doc 사전 ingest (총 1000 doc).
- baseline 측정 단계: `INDEX_MULTI_TENANCY_MODE=legacy` 모드에서 100
  query 실행 → IDX-001 dispatch.go without enforcement → p50/p95 latency
  기록.
- enforcement 측정 단계: `INDEX_MULTI_TENANCY_MODE=enforced` 모드로 전환
  후 동일 100 query 재실행.

### When

100 query를 round-robin으로 5 team에 분산 호출. 각 호출은 `IndexQuery{TeamID:
"team-X"}` 형태.

### Then

- baseline p50 latency ≤ 100ms (IDX-001 SLA).
- baseline p95 latency ≤ 250ms (IDX-001 SLA).
- enforcement p50 latency degradation ≤ 5% (Qdrant `is_tenant=true`의
  co-location 효과).
- **assertion**: enforcement p95 latency degradation ≤ 10% (NFR-IDX4-006).
- token cache hit-rate ≥ 99% (NFR-IDX4-003).
- 만약 p95 degradation > 10%이면 본 SPEC의 implementation이 NFR 회귀
  budget을 초과한 것이므로 PR 머지 거부.

---

## §5.6 **M6 EXIT CONTRIBUTION**: cross-team probe → 0 leak

**Coverage**: NFR-IDX4-001, REQ-IDX4-001, REQ-IDX4-006, REQ-IDX4-007,
REQ-IDX4-008 (CRITICAL SECURITY INVARIANT)

### Given

- Integration test 환경: testcontainers (Qdrant + Meili + PG).
- team-T가 100 confidential doc ingest (e.g. "company internal roadmap
  2026 Q3"). 각 doc의 `team_id="team-T"`.
- team-U가 100 distinct doc ingest, `team_id="team-U"`.
- 모든 doc은 동일 collection (`usearch_docs`) + 동일 Meili 인덱스
  (`usearch_docs`).

### When

team-U 사용자가 정상 JWT (team_id="team-U")로 다음 시도를 100회 반복 +
50 goroutine concurrent:

1. team-T doc의 body 키워드를 query로 `index.Search(ctx={team_id:"team-U"},
   IndexQuery{TeamID:"team-U"})` 호출.
2. team-T의 doc_id를 직접 query로 (full-text or exact match) `index.Search`
   호출.
3. PG 직접 connection으로 `SELECT * FROM docs WHERE team_id = 'team-T'`
   시도 (단, normal application code path는 RLS or 조건 박힘으로 거절).
4. Qdrant 직접 API로 `team_id="team-T"` filter 없이 scroll 시도 (단,
   normal application path는 항상 team_id filter 합성).
5. Meili 직접 API로 admin key 없이 search 시도 (단, tenant token이 team-U
   로 발급되어 team-T row 자동 제외).

### Then

- 시도 (1), (2)의 결과: lookup 응답이 **빈 슬라이스** 또는 team-U own
  records만 (NEVER team-T row).
- 시도 (3), (4), (5)는 normal application code path에서는 발생할 수 없으며
  (외부에서 raw DB connection으로만 가능), 본 acceptance test는 normal
  path에서만 검증.
- 100 회 시도 × 50 goroutine = 5000회 모두 cross-team leak 0회 (정확히 0).
- 4-layer defense 모두 작동:
  - Qdrant payload filter `team_id == "team-U"` (REQ-IDX4-006)
  - Meili tenant token의 `searchRules: team_id = "team-U"` (REQ-IDX4-008)
  - PG INSERT/SELECT 경로에서 ctx의 team_id 기반 row 격리 (REQ-IDX4-002)
  - dispatch.go sentinel `ErrTeamIDRequired` (REQ-IDX4-001)
- Prometheus 카운터 `usearch_index_tenant_token_validation_failures_total
  {outcome="rejected"}` 0 (정상 enforcement; validation failure는 어떤
  team-U 호출에서도 발생 안 함).

### Why this matters

NFR-IDX4-001 ("cross-team leak probability 정확히 0")의 acceptance test.
본 시나리오는 IDX-005의 NFR-IDX5-004 ("정확히 0 cross-tenant cached
answer leak")의 prerequisite invariant다. 본 시나리오가 실패하면 IDX-005
도 자동으로 실패하므로 M6 전체 GA가 차단된다.

---

## §5.7 backfill admin CLI dry-run + 실제 실행 + resume from crash

**Coverage**: REQ-IDX4-011, NFR-IDX4-005

### Given

- v0.1 데이터: PG `docs` 테이블에 `team_id=NULL` row 5000개 + Qdrant
  + Meili 동일.
- Operator가 `INDEX_DEFAULT_TEAM="default"` 설정 후 마이그레이션 0004
  실행 직전 단계.

### When (Phase 1: dry-run)

```
usearch admin backfill-team --default-team default --dry-run --batch-size 1000
```

### Then (Phase 1)

- CLI는 stderr/stdout에 다음 출력:
  - "PG: 5000 rows to update (5 batches of 1000)"
  - "Qdrant: ~5000 points to patch (5 batches)"
  - "Meili (usearch_docs): ~5000 documents to patch"
  - "Meili (usearch_docs_ko): ~N documents to patch (sub-count)"
- **PG / Qdrant / Meili에 어떤 변경도 발생하지 않음** (verify: `SELECT
  count(*) FROM docs WHERE team_id IS NULL` = 5000 그대로).
- `state.json` 미생성.

### When (Phase 2: 실제 실행 + 도중 crash 시뮬레이션)

```
usearch admin backfill-team --default-team default --batch-size 1000
```

3번째 batch 처리 후 강제 종료 (SIGKILL).

### Then (Phase 2)

- 3 batches × 1000 rows = 3000 rows가 `team_id='default'`로 update됨.
- `state.json` 에 `{"pg": {"last_processed_doc_id": "..."}, "qdrant":
  {...}, "meili": {...}}` 기록.
- 남은 2000 rows는 NULL 상태 유지.
- migration 0004는 아직 실행 안 함 (NOT NULL 강제 안 됨).

### When (Phase 3: resume)

```
usearch admin backfill-team --default-team default --batch-size 1000
```

### Then (Phase 3)

- CLI는 `state.json` 읽고 last_processed_doc_id 이후부터 재개.
- 추가 2 batches × 1000 rows = 2000 rows update.
- 완료 후 verify: `SELECT count(*) FROM docs WHERE team_id IS NULL` = 0.
- Qdrant scroll: `team_id IS NULL` payload point 0개.
- Meili search: `team_id IS NULL` document 0개.
- Prometheus 카운터 `usearch_index_tenant_backfill_total{store="pg",
  outcome="success"}` += 5000 (cumulative across resume).
- `state.json` 삭제 (정상 완료).
- 이제 migration 0004 안전 실행 가능 (NULL row 0).

---

## §5.8 Qdrant tier-promote admin CLI: 대형 팀의 dedicated collection 승격

**Coverage**: REQ-IDX4-005

### Given

- 운영 환경에서 team-MEGA의 doc 수가 1,000,000 초과.
- 운영자가 `.moai/config/sections/index.yaml`에 `qdrant.tiering.dedicated_teams:
  ["team-MEGA"]` 추가.

### When (Phase 1: dry-run)

```
usearch admin tier-promote --team team-MEGA --dry-run
```

### Then (Phase 1)

- CLI 출력:
  - "Would create collection: usearch_docs__team_<sha256('team-MEGA')[:16]>"
  - "Would move ~1,000,000 points from usearch_docs to dedicated"
  - "Default-tier impact: filter team_id == 'team-MEGA' would return 0 points
    post-promote"
- Qdrant에 변경 없음.

### When (Phase 2: 실제 실행)

```
usearch admin tier-promote --team team-MEGA
```

### Then (Phase 2)

- Qdrant: 새 collection `usearch_docs__team_<hash>` 생성 (default와 동일
  vector_size + distance metric).
- streaming 이동: default-tier scroll → batch 1000 points → 신규
  collection upsert → 기존 delete.
- 완료 후 검증:
  - `scroll(default, filter=team_id=="team-MEGA").count == 0` (NEVER fail
    on this).
  - `scroll(dedicated, no filter).count >= 1_000_000` (모두 이동됨).
- dispatch.go의 routing logic이 즉시 team-MEGA upsert/search를 dedicated
  collection으로 라우팅.
- 기존 team들 (team-A, B, ...) 의 retrieval은 default-tier 동일하게 작동.
- v1.0은 manual list만 활성; doc-count auto-tier는 SPEC-IDX-007 deferred.

---

## Edge Case 1 — tenancy mode 전환 (`permissive` → `enforced`) 시 NULL row 거절 동작

**Coverage**: REQ-IDX4-001 (boundary)

### Given

- 기존 데이터에 일부 NULL team_id row가 남아있음 (backfill 완료 직후 NULL
  0이지만, 실험적으로 NULL row를 직접 INSERT한 상태로 가정).
- 운영자가 `INDEX_MULTI_TENANCY_MODE=permissive`로 운영 중 → mode를
  `enforced`로 전환 + 프로세스 재시작.

### When

caller가 `index.Search(ctx, IndexQuery{TeamID:"team-T"})` 호출 (정상
team_id 제공).

### Then

- `enforced` 모드에서 `q.TeamID != ""`이므로 sentinel 통과.
- Qdrant filter는 `team_id == "team-T"` must clause 적용 → NULL team_id row
  는 결과에서 자동 제외 (filter 조건 미부합).
- PG SELECT도 `team_id = 'team-T'` 조건이 NULL row 제외.

### When (boundary)

caller가 `index.Search(ctx, IndexQuery{TeamID:""})` 호출 (실수로 빈 team_id).

### Then

- `enforced` 모드: 즉시 `ErrTeamIDRequired` 반환. embedder 미호출. store
  fanout 미호출.
- 이전 `permissive` 모드였다면 NULL team_id row 포함하여 응답 (호환성
  유지).
- 이전 `legacy` 모드였다면 team_id 완전 무시 → 모든 row 반환 (v0.1 동작).

### Why this matters

mode 전환의 의미를 명확히 검증. `enforced` 모드의 sentinel이 첫 단계에서
정확히 동작함을 확인. NULL row를 영구히 격리하려면 backfill 완료 + migration
0004 적용 후 mode 전환해야 함을 operational runbook에 명시.

---

## Edge Case 2 — token cache concurrency: 50 goroutine × 100 호출 race-free

**Coverage**: NFR-IDX4-004

### Given

- token cache는 cold state.
- 50 goroutine이 같은 (team_id="team-T", user_id="alice")에 대해 동시
  `index.Search` 호출 시작.
- 각 goroutine은 100 호출.

### When

테스트가 50 goroutine을 `sync.WaitGroup`으로 launch → 각 goroutine에서
100 회 Search 호출 → 총 5000 호출.

### Then

- `meilisearch-go.GenerateTenantToken`은 정확히 **1회** 호출됨 (sync.Once
  per (team, user, key_uid) triplet).
- 5000 회 호출 모두 정상 응답 (cache hit-rate ≥ 99.98% — 첫 호출만 miss).
- `goleak.VerifyNone(t)` PASS (refresh worker만 active, 그것도 ctx cancel
  로 graceful shutdown).
- `-race` 모드에서 실행 시 data race detector trigger 없음.
- Prometheus 카운터 `usearch_index_tenant_token_issued_total{tier="meili",
  outcome="success"}` 정확히 1 증가.

### Why this matters

`sync.Once`가 high-concurrency 조건에서 정확히 동작함을 검증. token 발급
이 5000회 중복 호출되면 Meili admin key 부하 5000배 + HMAC-SHA256 서명
비용 5000배 → 운영 비용 폭증 + 잠재적 Meili rate-limit 트리거.

---

## Acceptance Coverage Matrix

| Scenario | REQ-001 | REQ-002 | REQ-003 | REQ-004 | REQ-005 | REQ-006 | REQ-007 | REQ-008 | REQ-009 | REQ-010 | REQ-011 |
|----------|---------|---------|---------|---------|---------|---------|---------|---------|---------|---------|---------|
| §5.1 | ✓ | ✓ | ✓ | ✓ | | | | | | | |
| §5.2 | | | | | | | | ✓ | ✓ | | |
| §5.3 | | ✓ | | | | ✓ | | | | | |
| §5.4 | | | | | | ✓ | ✓ | | | | |
| §5.5 | | | | | | | | | | | |
| §5.6 | ✓ | | | | | ✓ | ✓ | ✓ | | | |
| §5.7 | | | | | | | | | | | ✓ |
| §5.8 | | | | | ✓ | | | | | | |
| Edge1 | ✓ | | | | | | | | | | |
| Edge2 | | | | | | | | | ✓ | | |
| PG migrations | | | | | | | | | | ✓ | |

- §5.5 는 NFR-IDX4-003 + NFR-IDX4-006 (latency budget)를 PRIMARY로 검증.
- §5.6 는 NFR-IDX4-001 (cross-team zero leak)을 PRIMARY로 검증 — M6
  enabling invariant.
- §5.7 는 NFR-IDX4-005 (backfill atomicity & resumability)를 PRIMARY로
  검증.
- Edge2 는 NFR-IDX4-004 (token cache concurrency safety)를 PRIMARY로 검증.
- PG migrations (`TestMigration0004*`, `TestMigration0005*`)는 별도
  unit test로 REQ-IDX4-010 cover.
- 모든 시나리오가 NFR-IDX4-007 (observability cardinality bounded)을
  indirectly 검증 (메트릭 emission + 화이트리스트).
- NFR-IDX4-002 (same-team dedup correctness)는 IDX-001 ON CONFLICT (doc_id)
  DO UPDATE 패턴 재사용으로 자동 보장.
- NFR-IDX4-008 (AUTH ship-before forward-compat)은 `INDEX_DEFAULT_TEAM`
  env var fallback (REQ-IDX4-003) 동작 검증으로 cover.

---

## M6 IDX-004 Release Gate — Final Checklist

본 acceptance.md의 모든 시나리오가 PASS 시 다음을 추가 확인 후 IDX-005
implementation 진입 가능:

1. §5.1 PASS — enforced mode upsert/search round-trip 정상
2. §5.2 PASS — Meili tenant token 두 인덱스 적용 + cache 재사용
3. §5.3 PASS — personal context tier visibility 정확
4. §5.4 PASS — `__public__` sentinel 합성 정확
5. §5.5 PASS — latency degradation ≤ 10%
6. **§5.6 PASS — cross-team leak 정확히 0 (M6 ENABLING INVARIANT)**
7. §5.7 PASS — backfill CLI dry-run + 실제 + resume
8. §5.8 PASS — tier-promote CLI 정상
9. Edge1, Edge2 PASS — mode 전환 boundary + token cache concurrency
10. PG migrations idempotent + backfill verified
11. integration test가 `-race` 모드로 통과 (NFR-IDX4-004)
12. LSP gate: zero errors / zero type errors / zero lint errors
13. Coverage ≥85% on `internal/index/tenancy/`, `internal/index/auth/`,
    `internal/index/tenant/`, `internal/index/backfill/`,
    `cmd/usearch/admin/`
14. plan-auditor cycle 1+회 통과 (HIGH 0, MEDIUM ≤2, NIT 무한)

위 모두 PASS 시 IDX-004 ship → IDX-005 implementation 진입 가능 → IDX-005
의 `TestDedupHitRateAt30PctOnSyntheticTraffic` (M6 PRIMARY GATE)가 측정
가능 상태에 진입.

---

*End of SPEC-IDX-004 acceptance.md.*
