# SPEC Review Report: SPEC-ADP-010

Iteration: 1/3
Verdict: **PASS-WITH-FIXES**
Overall Score: 0.90

> Reasoning context ignored per M1 Context Isolation. This audit reads only
> `spec.md`, the companion `research.md`, the cited source files, and live
> Meta developer documentation independently fetched during the audit.

대상: `.moai/specs/SPEC-ADP-010/spec.md` (Facebook + Threads (Meta) 어댑터,
GSD 감사 항목 F-09 대응). 본 SPEC의 backbone은 외부 API에 대한 **feasibility
판정**이므로, 모든 외부 주장을 실시간 검증(curl + HTML 파싱)으로 독립
확인했습니다. 결론부터: **핵심 feasibility 판정 2건은 모두 참이며, 발명된
엔드포인트/필드/권한명은 발견되지 않았습니다.**

---

## Must-Pass Results

- **[PASS] MP-1 REQ number consistency** — REQ-ADP10-001 ~ 008, 8개 연속,
  일관된 3자리 zero-pad, gap/중복 없음 (`spec.md:226-233`). NFR-ADP10-001~004
  도 연속 (`spec.md:241-244`).

- **[PASS] MP-2 EARS format compliance** — 5개 패턴 모두 사용:
  Ubiquitous(001,005), Event-Driven(002,003,004), Optional(006),
  State-Driven(007), Unwanted(008). Facebook 영구-미구현 요구사항이 Unwanted
  패턴("The adapter SHALL NOT issue ANY HTTP request ... WHEN ... SHALL
  return ErrFacebookNotSupported", `spec.md:233`)으로 올바르게 표현됨.
  경미한 흠은 D2 참조(복합 패턴).

- **[PASS] MP-3 YAML frontmatter validity** — id/version/status/priority/
  labels 모두 존재, 타입 정상 (`spec.md:2-17`). 날짜 키는 프로젝트 관행에
  따라 `created`/`updated` (다른 SPEC들과 동일; `created_at` 변형 아님 —
  프로젝트 일관성 OK).

- **[N/A] MP-4 Section 22 language neutrality** — N/A: 본 SPEC은 16개
  프로그래밍 언어 도구 체인이 아니라 단일 도메인(Meta 소셜 API) 어댑터
  SPEC. 자동 통과.

---

## External-API Feasibility Verification (HIGHEST PRIORITY — independently verified)

도구: `curl` 직접 호출(2026-06-04 실행) + HTML 본문 파싱. WebSearch 미사용
환경이지만 1차 출처(Meta 공식 문서)를 직접 fetch 하여 검증함.

| 주장 (spec.md / research.md) | 독립 검증 결과 | 판정 |
|---|---|---|
| Threads 엔드포인트 `GET graph.threads.net/v1.0/keyword_search` 존재 (`spec.md:36,227`) | keyword-search 문서 페이지 **HTTP 200**; 본문에 `graph.threads.net/v1.0/keyword_search` 정확히 등장 | **TRUE — 발명 아님** |
| 권한명 `threads_keyword_search` (`spec.md:37,213`) | 본문에 `threads_keyword_search` 정확히 등장 | **TRUE** |
| 미승인 시 본인 게시물만 / 승인 시 전체 공개 게시물 (`spec.md:38, research §1.1`) | 페이지 본문 그대로: "앱이 `threads_keyword_search` 권한에 대해 승인되지 않은 경우, 검색은 인증된 사용자가 소유한 게시물에 대해서만 수행됩니다. 승인을 받은 후에는 전체 공개 게시물을 검색할 수 있게 됩니다." | **TRUE — 정확** |
| OAuth short-lived 1h → long-lived 60d, `GET /refresh_access_token` 갱신 (`research §1.4`) | long-lived-tokens 페이지 본문: "단기 실행되고 1시간 동안 유효", "장기 실행 토큰은 60일 동안 유효", "`GET /refresh_access_token` 엔드포인트" | **TRUE — 정확** |
| 레이트리밋 2,200/24h, 사용자당, 민감 키워드 → 빈 배열 (`spec.md:78,444; research §1.5`) | 본문: "2,200개의 쿼리를 전송", "민감한 ... 빈 배열을 반환" | **TRUE** |
| 파라미터 `q/search_type(TOP·RECENT)/search_mode(KEYWORD·TAG)/media_type/since/until/limit/author_username` (`research §1.2`) | 본문에서 `search_type`, `search_mode`, `media_type`, `author_username` + 값 TOP/RECENT/KEYWORD/TAG 모두 확인 | **TRUE** |
| 응답 필드 `permalink/username/has_replies/is_quote_post/is_reply/media_type` (`research §1.3`) | 본문에서 `permalink`, `has_replies`, `is_quote_post`, `media_type` 확인 | **TRUE** |
| 응답에 engagement 카운트·lang 없음 → `Score=0.5`, `Lang=""` (`spec.md:200-204, 502-504`) | 본문에 like_count/repost_count 류 카운트 필드 부재 확인 (`repost`는 1회, `is_quote_post` 맥락) | **TRUE — 정직한 처리** |
| Facebook 공개 검색 레퍼런스 부재 (`spec.md:41-46,214; research §2.1`) | `/docs/graph-api/reference/v22.0/search` **HTTP 404**, `/docs/graph-api/reference/search` **HTTP 404** (둘 다) — Threads 페이지는 200인데 Facebook search 레퍼런스만 404 | **TRUE — NOT ACHIEVABLE 판정 뒷받침** |

**결론: SPEC의 두 backbone 판정(Threads ACHIEVABLE-조건부 / Facebook NOT
ACHIEVABLE)은 Meta 1차 문서로 독립 검증됨. 엔드포인트 경로, 권한명, OAuth
토큰 수명(1h/60d), 레이트리밋, 게이팅 동작 — 모두 정확하며 fabrication
없음.** 이것이 본 감사의 가장 중요한 결과입니다.

단, Facebook NOT-ACHIEVABLE 판정의 **시점 주장**("removed in Graph v2.0
(2015)", "2018–2020 제거")은 404 신호(부재의 증거)에 기반할 뿐 1차 출처가
없음 → D1 참조.

---

## Code-Citation Accuracy (STRICT — all verified against working tree)

| SPEC 인용 | 실제 | 판정 |
|---|---|---|
| `pkg/types/adapter.go:28-45` Adapter 4-메서드 | interface `28-45` 정확; Name/Search/Healthcheck/Capabilities | **정확** |
| `pkg/types/capabilities.go:38-62` Capabilities struct | struct `38-62` 정확 | **정확** |
| Capabilities 필드셋 `DocTypes/SupportedLangs/RequiresAuth/AuthEnvVars/DisplayName/RateLimitPerMin` + scalar Category/Lang 없음 | 실제 필드: SourceID, DisplayName, DocTypes, SupportedLangs, SupportsSince, RequiresAuth, AuthEnvVars, RateLimitPerMin, DefaultMaxResults, Notes. **Category/Lang scalar 없음** | **정확 — 발명 필드 없음** |
| `internal/adapters/social/social.go:33-40` Adapter struct | struct `33-39` (httpClient/baseURL/userAgent/healthcheckTarget/subSource/envLookup) | **정확** |
| `social.go:73-124` NewBluesky/NewX 2-서브소스 dispatch | NewBluesky@73, NewX@110, Name@128, Capabilities switch@133 | **정확** |
| `social.go:163-178` xCapabilities DISABLED | `163-178` xCapabilities() | **정확** |
| `social.go:208-209` X Healthcheck DISABLED | `ErrXDisabled` @209 | **정확** |
| `cmd/usearch/query.go:458-514` buildProductionRegistry | func@458 … `return reg`@513, `}`@514 | **정확** |
| GitHub env-gated 등록 패턴 + `THREADS_ACCESS_TOKEN` 차용 | `USEARCH_GITHUB_TOKEN`@476 (GITHUB_TOKEN fallback@478), Register@485 — SPEC가 인용한 env 이름 일치 | **정확** |

부가 검증: `pkg/types/normalized_doc.go`에 §6.5 매핑 대상 필드(ID/SourceID/
URL/Title/Body/Snippet/PublishedAt/RetrievedAt/Author/Score/Lang/Citations/
Metadata/Hash) 전부 존재. `pkg/types/errors.go`의 `SourceError.Is()`가
`CategoryPermanent → ErrPermanent` 매핑(`errors.go:105-113`)하므로 REQ-008·
REQ-002 의 `errors.Is(err, types.ErrPermanent)` acceptance 달성 가능 —
reddit 어댑터(`internal/adapters/reddit/search.go:49`)가 동일 패턴 입증.

**메모리 경고(드래프트 SPEC는 종종 존재하지 않는 경로/ID를 인용)와 달리,
이 SPEC의 코드 인용은 전부 정확함.** 발명된 필드/경로 없음.

---

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.90 | 0.75–1.0 | 거의 모든 요구사항 단일 해석 가능; per-platform verdict 표(`spec.md:211-214`)가 backbone을 명확히 함. 감점: REQ-001/002 복합 절(D2). |
| Completeness | 0.95 | 1.0 | HISTORY/Purpose/Scope/REQ/AC/Exclusions(§7)/Risks/Open Q 전부 존재; Exclusions 14개 구체 항목(`spec.md:565-591`); frontmatter 완비. |
| Testability | 0.90 | 0.75–1.0 | AC 전부 이진(`Score==0.5`, `errors.Is`, "zero requests", HTTP 코드별 매핑); weasel word 없음. 감점: limit 음수→1 clamp 미검증(D7). |
| Traceability | 1.0 | 1.0 | 모든 REQ→§5 AC→§8 TDD 표(테스트 1-56) 매핑; orphan AC 없음, uncovered REQ 없음. |

---

## Defects Found

**D1. `spec.md:233` (REQ-ADP10-008 acceptance) + `research.md:135-146` — Severity: major**
REQ-008 acceptance(`TestFacebookNotSupportedMessageDocumentsBlocker`)는 에러
문자열이 `"removed in Graph v2.0"`를 **반드시 포함**하도록 요구하고,
Capabilities.Notes(`spec.md:471`)도 "removed in the Graph API in v2.0"을
단언함. 그러나 research.md §2.1의 시점 주장(v2.0/2015, 2018–2020)은 **1차
출처 인용이 없음** — research §8이 "WebSearch 비활성 → 제3자 교차검증
못함"을 인정하고, 판정을 404 신호(부재의 증거)에 의존함. 본 감사는 404를
독립 확인했으나(NOT-ACHIEVABLE 결론 자체는 견고), **특정 버전·연도를 테스트
통과 조건으로 하드코딩**하는 것은 출처 없는 역사적 단언에 테스트를 묶는
것임.
→ Fix: (a) 테스트/Notes 문자열을 검증 가능한 형태로 약화 —
"no official Graph API endpoint for public-post keyword search" 같이 현재-
상태 단언만 요구하고 버전/연도는 선택적으로; 또는 (b) run phase 진입 전
v2.0 deprecation 1차 출처(Meta changelog/공지 URL)를 research.md §8에
추가하고 인용. 현재 404 검증만으로 연도까지 단언하지 말 것.

**D2. `spec.md:226-227` (REQ-ADP10-001, REQ-ADP10-002) — Severity: minor**
두 요구사항이 한 행에 복합 패턴을 포함. REQ-001(Ubiquitous)은 토큰 부재 시
`ErrThreadsTokenMissing` 반환이라는 조건절(implicit If)을 내포; REQ-002
(Event-Driven)은 빈 질의 거부 "IF q.Text is empty ... THEN ..."라는 Unwanted
절을 내포. 원자성이 떨어져 추적·검증 시 모호.
→ Fix: 빈-질의 거부를 별도 Unwanted REQ로 분리(예: REQ-ADP10-009), 토큰-
부재 생성자 동작을 REQ-001에서 별도 절로 분리하거나 별 REQ화. (선택적 —
패턴 라벨 자체는 유효.)

**D3. `spec.md:53, 156, 391` — Severity: minor**
GitHub env-gated 등록 패턴을 HISTORY는 `query.go:476-494`로, §2.1(o)·§6.1은
`:476-487`로 인용. 실제로 GitHub 블록은 476-485, YouTube 블록이 488-492.
`476-494` 범위는 두 어댑터를 뭉뚱그림.
→ Fix: GitHub은 `:476-487`(또는 476-485), YouTube는 `:488-492`로 분리 표기.

**D4. `spec.md:196-205, 495` vs `pkg/types/normalized_doc.go:27,37` — Severity: minor**
SPEC는 `Score=0.5` 중립을 채택하나, NormalizedDoc 타입 문서는
"Score == 0.0 means unscored, NOT zero engagement"를 명시(즉 '신호 없음'의
관용 값은 0.0). 0.5 선택은 방어 가능하나(0.0은 '미채점'으로 오해됨), SPEC가
타입 계약의 0.0=unscored 관용과의 **차이를 명시**하지 않음 — RRF 소비자
(SPEC-IDX-001)가 0.5를 '중간 신뢰'로 오해할 여지.
→ Fix: §2.3에 "0.0은 타입 계약상 'unscored'를 의미하므로, '신호 부재'를
0.0이 아닌 0.5 중립으로 표현함"을 한 줄 명시. (또는 IDX-001 author와 0.0 vs
0.5 합의를 Open Question §11.1에 추가.)

**D5. `spec.md:444` (RateLimitPerMin=1) — Severity: minor**
`RateLimitPerMin: 1`을 "2200/24h ≈ 1.5/min floored to 1"로 도출. 그러나
Threads 한도는 **사용자(토큰)당** per-24h이지 per-minute·per-app이 아님
(`research §1.5`). `RateLimitPerMin` 필드 의미(calls/min, 0=unknown)와 느슨한
적합. Notes가 "2200/24h per USER"를 문서화하므로 치명적이지 않으나, 라우터가
이를 분당 1회 글로벌 한도로 오해하면 과소 호출.
→ Fix: 현행 유지 가능하되 Notes에 "per-user 24h budget, not a per-minute
cap"를 더 강조하거나, FAN-001이 분당 캡이 아닌 일일 토큰 예산으로 다루도록
Risks 행(`spec.md:731`)에 명시. (현재 어느 정도 명시됨 — 강화 권고.)

**D6. `spec.md:227` (REQ-ADP10-002 acceptance) — Severity: minor (테스트 누락)**
REQ-002 본문은 `limit=clamp(q.MaxResults,1,100)`로 하한 1을 규정하나,
acceptance/TDD 표에는 clamp-to-100(`test 14`)과 default-25(`test 15`)만 있고
**음수/<1 입력 → 1 clamp** 검증 케이스가 없음.
→ Fix: `TestSearchThreadsClampsLimitToMin1`(MaxResults=-5 또는 0 미만 →
`limit=1` 또는 default 처리) 추가. (q.MaxResults=0은 default 25로 처리되므로
음수 경계만 보강.)

---

## Honest-Scope Assessment (감사 항목 4)

- **Facebook 불가능 = 진짜 외부 차단 요인으로 프레이밍됨 (정직)**: REQ-008이
  HTTP 0회·스크래핑 0회·결과 fabrication 금지를 Unwanted로 명시하고
  (`spec.md:233`), opt-in env조차 제공 안 함(ADP-006의 X와 의도적 차별화),
  `ErrFacebookNotSupported` 메시지가 영구 한계임을 문서화. 가짜 happy path
  아님. **검증된 404 신호와 일관.**
- **정규화 갭 정직 처리**: `Score=0.5`, `Lang=""`를 `[HONESTY]` 주석
  (`spec.md:502-504`)으로 명시하고, "존재하지 않는 필드를 가정하지 않는다"고
  선언. keyword_search 응답에 engagement/lang 부재를 본 감사가 독립 확인 →
  주장 일관. (단 D4의 0.0 vs 0.5 계약 차이만 보완 필요.)
- 프로젝트 스코어링 모델과의 정합: NormalizedDoc 타입이 Score를 [0,1]로
  정의(`normalized_doc.go:27`)하고 IDX-001 RRF가 raw score가 아닌 rank를
  가중한다는 SPEC 주장은 합리적; 중립 0.5가 랭킹을 왜곡하지 않음.

## Package-Structure Judgment (감사 항목 5)

신규 `internal/adapters/meta/` (vs `social/` 확장) 권고는 **타당**:
(1) social.go는 이미 Bluesky(익명)+X(stub) 2-서브소스로 포화하고 인증축이
익명/env-gate임 — 검증됨(`social.go:33-40, 73-124`). (2) Meta는 OAuth 2.0
사용자 토큰 + 60일 갱신 + 앱 심사라는 **근본적으로 다른 인증축**(본 감사가
1h/60d/refresh_access_token로 독립 확인) → 응집도 격리 근거 충분. (3)
ADP-006이 fediverse 분리를 시사(`spec.md:794` 인용)하여 social의 무한 확장
의도 부재. 2-서브소스-1-패키지 dispatch 패턴 차용도 검증된 social 패턴과
일치. 판단 건전.

## Testability & Completeness 추가 메모

- OAuth 토큰 부재 fail-safe(어댑터 미등록)는 REQ-001 +
  `TestNewThreadsMissingTokenReturnsError` + `TestFacebookNotRegisteredIn
  ProductionRegistry`로 커버 — 견고.
- 앱-심사-미승인 시나리오: 토큰은 있으나 권한 미승인 → 401/403으로 귀결
  (REQ-003)되어 CategoryPermanent. 다만 "권한 미승인 시 본인 게시물만
  반환(200 OK, 비어있지 않은 data)" 상태는 어댑터가 구분 불가(정상 200으로
  보임). SPEC은 이를 운영자 책임/Capabilities.Notes로 위임(`spec.md:727`) —
  honest하나 런타임 탐지 불가는 알려진 한계로 명시됨. 추가 테스트 불요.
- Graph 에러 envelope, 빈-data=빈결과(에러 아님), 429 Retry-After cap,
  redirect allowlist(SSRF), ctx 취소 goroutine leak, race — 전부 REQ+테스트
  보유. 누락 실패 모드는 D6(음수 limit) 정도로 경미.

---

## Chain-of-Verification Pass

2차 정독으로 재확인한 항목:
- **REQ 시퀀스 end-to-end**: 001~008 전 행을 직접 카운트 — gap/중복 없음
  확인(spot-check 아님).
- **Traceability 전수**: §3 REQ ↔ §5 AC ↔ §8 TDD 표(56개 테스트) 교차 —
  모든 REQ/NFR에 최소 1개 테스트 매핑 확인(REQ-008→test 48-51,
  NFR→52-56).
- **코드 인용 전수**: 인용된 9개 file:line을 working tree에서 직접 grep —
  전부 일치(특히 capabilities 필드셋에 Category/Lang scalar 부재 확인,
  SourceError.Is→ErrPermanent 배선 확인).
- **외부 주장 전수**: Threads 엔드포인트/권한/토큰수명/레이트리밋/게이팅과
  Facebook 404를 curl로 직접 검증 — 표 참조. fabrication 없음.
- **Exclusions 구체성**: §2.2 + §7의 항목들이 각각 목적지 SPEC을 가짐
  (FBSCRAPE/AUTH-*/IDX-001/FAN-001/CACHE-001) — vague 항목 없음.
- **모순 점검**: REQ 간 모순 없음; Exclusions가 포함 요구사항과 충돌 없음
  (Facebook은 REQ-008과 Exclusion 양쪽에서 일관되게 DISABLED).

신규 발견: 1차에서 놓쳤던 D6(음수 limit clamp 테스트 누락)을 2차에서 §8 TDD
표 정독 중 발견하여 추가. 그 외 1차 결론 유지.

---

## Recommendation

**Verdict: PASS-WITH-FIXES.** 본 SPEC의 핵심 위험(외부 API feasibility)은
독립 검증 결과 **전부 참**이며, 코드 인용도 전부 정확하고 발명된 엔드포인트/
필드/권한명이 없음. 발견된 결함은 1건의 major(D1, 출처 없는 버전/연도를 테스트
통과 조건에 하드코딩)와 5건의 minor로, 어느 것도 backbone 판정을 무효화하지
않음.

ready 도달을 위한 최소 변경 목록:

1. **(D1, major)** REQ-008 acceptance와 Facebook Capabilities.Notes의
   `"removed in Graph v2.0"` 문자열을 (a) 검증 가능한 현재-상태 단언으로
   약화하거나, (b) run phase 전 Meta v2.0 deprecation 1차 출처를 research.md
   §8에 인용. 404 검증만으로 특정 연도를 단언하지 말 것.
2. **(D6, minor)** `limit` 음수/하한 clamp 테스트(`TestSearchThreadsClamps
   LimitToMin1`) 1건 추가하여 REQ-002 본문과 acceptance 정합.
3. **(D4, minor)** §2.3에 "0.0=unscored(타입 계약) vs 0.5=신호부재 중립"
   차이를 1줄 명시하거나 Open Question §11.1에 IDX-001 합의 항목 추가.
4. **(D3, minor)** GitHub/YouTube 등록 인용 라인을 분리(`476-487` /
   `488-492`).
5. **(D2, minor, 선택)** REQ-001/002의 복합 절을 별 REQ로 분리해 원자성 향상.
6. **(D5, minor, 선택)** `RateLimitPerMin=1`이 per-minute가 아닌 per-user/24h
   예산임을 FAN-001 소비자에게 더 강하게 신호(Notes/Risks 강화).

1번을 해소하면 PASS로 승격 권고. 2~6은 run phase 중 병행 가능.
