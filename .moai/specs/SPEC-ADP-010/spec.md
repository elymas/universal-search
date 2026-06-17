---
id: SPEC-ADP-010
title: Facebook + Threads (Meta) Adapter — Feasibility & Integration Contract
version: 0.1.0
milestone: M3 — Fanout, adapters, index
status: implemented
priority: P3
owner: expert-backend
methodology: tdd
coverage_target: 85
created: 2026-06-04
updated: 2026-06-05
author: limbowl
issue_number: null
depends_on: [SPEC-ADP-006, SPEC-CORE-001, SPEC-IR-001]
labels: [adapter, social, meta, facebook, threads, M3]
---

# SPEC-ADP-010: Facebook + Threads (Meta) Adapter — Feasibility & Integration Contract

## HISTORY

- 2026-06-04 (initial draft v0.1, limbowl via manager-spec):
  GSD 감사 항목 **F-09** 대응 SPEC 초안. F-09 원문
  (`.planning/AUDIT-FINDINGS.md:23`): "Facebook and Threads have no
  adapter at all (0 code). Meta Graph API offers no public-feed keyword
  search → large new-adapter + auth design." 본 SPEC은 **공식 API로
  구현이 불가능할 수도 있는 기능**에 대한 것이므로, 모든 외부 주장을
  WebFetch로 검증하여 `.moai/specs/SPEC-ADP-010/research.md`에 인용했습니다.
  **존재하지 않는 엔드포인트/필드를 발명하지 않았습니다.**

  연구 핵심 결론 (research.md §3 per-platform verdict):

  - **Threads = ACHIEVABLE (조건부)**. Meta가 별도 Threads API에 전용
    `GET https://graph.threads.net/v1.0/keyword_search` 엔드포인트를
    제공함 (research.md §1.1, WebFetch 검증). 단, **`threads_keyword_search`
    권한이 Meta 앱 심사로 승인된 경우에만** 전체 공개 게시물을 검색하며,
    미승인 시 본인 게시물만 검색됨. → F-09의 "Meta에는 공개 키워드
    검색이 없다"는 단언은 Threads에 대해 **거짓**.

  - **Facebook = NOT ACHIEVABLE via 공식 API**. 공식 Graph API에 공개
    게시물/페이지 키워드 검색 엔드포인트가 없음 — 검증된 신호: Meta 정식
    search 레퍼런스 URL이 일관되게 HTTP 404 반환(동일 실행에서 Threads
    페이지는 200), 그리고 get-started/using-graph-api 문서에 공개 검색
    표면 부재 (research.md §2.1). (구체적 제거 버전/연도는 1차 출처를
    확보하지 못해 판정 근거에서 제외 — research.md §2.1 caveat 참조.)
    스크래핑은 ToS 리스크로 v0 배제(`tech.md:147` 준수). → F-09의 단언은
    Facebook에 대해 **참**.

  사용자-고정 결정:

  - **D1 v0 범위**: Threads INTEGRATED (단 OAuth 토큰 게이트 +
    `threads_keyword_search` 승인 전제) + Facebook RESERVED-but-DISABLED.
    Threads는 `THREADS_ACCESS_TOKEN`이 존재할 때만 등록됨(env-gated,
    GitHub 등록 패턴 `cmd/usearch/query.go:476-487` (YouTube는 `:488-494`)
    차용).
    Facebook은 SPEC-ADP-006의 X와 동일하게 constructor + Capabilities
    surface만 예약하고 Search는 영구 에러 반환.

  - **D2 신규 `internal/adapters/meta/` 패키지** (research.md §4.3 권고).
    social 패키지가 아닌 별도 패키지 — Meta의 OAuth 2.0 사용자 토큰 +
    60일 갱신 + 앱 심사라는 인증 축이 social의 익명/env-gate 모델과
    근본적으로 달라 응집도를 위해 격리. SPEC-ADP-006의
    2-서브소스-1-패키지 dispatch 패턴(`social.go:73-194`)은 차용:
    `meta.NewThreads(opts)` + `meta.NewFacebook(opts)`가 동일 `*Adapter`
    반환, `subSource`("threads"/"facebook")로 Search dispatch.

  - **D3 외부 차단 요인 명시**: Threads 공개 검색은 (1) Meta 앱 생성,
    (2) `threads_basic`+`threads_keyword_search` 앱 심사 승인,
    (3) OAuth long-lived 토큰 확보+60일 갱신을 모두 요구. 승인 전에는
    본인 게시물만 반환되어 무의미. 이 선행 조건은 어댑터 미등록으로
    fail-safe 처리 (토큰 부재 → registry에 추가 안 함).

  - **D4 Go 의존성 0개** (research.md §1.7). Threads keyword_search는
    단일 HTTP GET + JSON; `net/http`+`encoding/json` stdlib로 충분.
    Meta 공식 Go SDK 없음; 비공식 SDK 미채택. SPEC-ADP-006 D3
    (indigo 거부) 동일 posture.

  - **D5 점수 중립값**: keyword_search 응답 필드(research.md §1.3)에
    engagement 카운트(like/repost)가 없어 ADP-006식 Tanh 정규화 입력
    신호 부재 → v0는 `Score=0.5` 중립. Open Question §11.1.

  - **D6 시크릿 처리**: `THREADS_ACCESS_TOKEN`은 로그/메트릭 라벨에
    절대 미포함; env로만 주입, `Capabilities.AuthEnvVars`에 이름만 노출.

  - **D7 테스트 격리**: env 의존 테스트는 `Options.EnvLookup` 주입 사용;
    `t.Setenv` 금지(`-race` goroutine-unsafe, ADP-006 H1
    `spec.md:24-37`). httptest.Server + golden JSON 픽스처; CI 라이브
    네트워크 호출 없음.

  9 EARS REQs (8×P0 + 1×P1) — 5개 EARS 패턴 모두 사용
  (Ubiquitous: 001/005, Event-Driven: 002/003/004, Optional: 006,
  State-Driven: 007, Unwanted: 008 Facebook-not-supported + 009
  empty-query-rejection), 4 NFRs. 신규 Go 모듈 의존성 0개. M3에
  META-CATEGORY 어댑터로 삽입; P3(external-blocked, large design) —
  Threads 경로는 Meta 앱 심사 승인이라는 외부 차단 요인에 묶여 있고
  Facebook은 미구현 가능. Harness level: standard.

- 2026-06-04 (iteration 2 — plan-auditor cycle 1 PASS-WITH-FIXES 0.90,
  limbowl via manager-spec): 감사가 두 feasibility 판정(Threads
  ACHIEVABLE / Facebook NOT-ACHIEVABLE)과 9개 코드 인용을 curl로 독립
  검증 — fabrication 없음 확인. 6개 결함 수정: (D1 major) REQ-008
  acceptance + Facebook Capabilities.Notes + ErrFacebookNotSupported
  sentinel에서 출처 없는 `"removed in Graph v2.0"` 버전/연도 단언 제거 →
  검증 가능한 현재-상태 신호("no public-post keyword search endpoint";
  Graph search 레퍼런스 HTTP 404)로 약화. research.md §2.1은 버전/연도를
  미검증 배경 맥락으로 강등하고 1차 출처 부재를 명시. (D6) REQ-002에
  `TestSearchThreadsClampsLimitToMin1`(음수 limit → 1 clamp 하한) 테스트
  추가 (test 14a). (D4) §2.3에 0.0=unscored(타입 계약) vs 0.5=채점-중립
  차이를 명시; Open Question §11.1에 IDX-001 합의 항목 추가. (D3)
  GitHub/YouTube 등록 인용 라인 분리(GitHub `:476-487`, YouTube
  `:488-494`). (D2) REQ-002의 빈-질의 거부 Unwanted 절을 별도
  REQ-ADP10-009로 분리해 원자성 향상. (D5) `RateLimitPerMin=1`이
  per-minute가 아닌 per-user/24h 일일 예산임을 §6.3 Notes + Risks 행에
  강조. REQ 8→9개(8×P0+1×P1), NFR 4개 불변. plan-auditor cycle-2 재검토
  준비 완료.

---

## 1. Purpose

GSD 감사 항목 **F-09** (`.planning/AUDIT-FINDINGS.md:23`)는 Facebook과
Threads에 어댑터가 전혀 없음(0 code)을 지적하고, Meta Graph API에 공개
피드 키워드 검색이 없어 "large new-adapter + auth design"이 필요하다고
명시합니다. 본 SPEC은 이 공백을 해소하되, **현실을 정직하게 판정**하는
것을 1차 목표로 합니다.

연구(research.md)는 F-09의 핵심 단언이 **절반만 맞음**을 확인했습니다:

- **Facebook**: 참. 공식 API에 공개 게시물/페이지 키워드 검색 엔드포인트
  부재 (Graph API search 레퍼런스 HTTP 404로 검증; research.md §2.1).
- **Threads**: 거짓. Meta가 별도 Threads API에 전용 `keyword_search`
  엔드포인트를 추가함 (research.md §1.1, WebFetch 검증).

따라서 이 SPEC은 **플랫폼별로 다른 판정**을 내립니다 (§2.4 Feasibility
Verdict가 본 SPEC의 backbone):

1. **Threads는 INTEGRATED** — 단, 외부 차단 요인(Meta 앱 심사 승인 +
   OAuth 토큰) 전제. usearch의 어댑터 계약(질의 텍스트 → 랭크된 공개
   `NormalizedDoc` 결과)을 `keyword_search`로 충족 가능.
2. **Facebook은 RESERVED-but-DISABLED** — SPEC-ADP-006의 X 처리
   (`social.go:163-178`)와 동일. 공식 API로 어댑터 계약 충족 불가하며,
   스크래핑은 ToS 리스크로 v0 배제(`tech.md:147`).

이 어댑터는 fanout(SPEC-FAN-001), 재시도(SPEC-FAN-001), 캐싱
(SPEC-CACHE-001), 랭킹 융합(SPEC-IDX-001)을 하지 않으며, 메트릭/로그/
스팬을 직접 방출하지 않습니다(레지스트리 wrappedAdapter가 sole-emitter).
서브소스당 한 가지 일만 합니다: `types.Query`를 Threads keyword_search
HTTP 요청 → JSON 응답 → `[]NormalizedDoc`으로 변환하거나(Threads),
Facebook에 대해 외부 차단 요인을 명시하는 에러를 반환합니다.

본 SPEC은 SPEC-ADP-006의 social 어댑터 패턴을 차용하되, **익명 접근이
아닌 OAuth 2.0 사용자 토큰**이라는 새로운 인증 축을 도입하므로 별도
`internal/adapters/meta/` 패키지로 격리합니다 (research.md §4.3).

---

## 2. Scope

### 2.1 In-Scope

| Ref | Item |
|-----|------|
| a | `internal/adapters/meta/meta.go`: `Adapter` struct (httpClient + baseURL + accessToken + userAgent + healthcheckTarget + `subSource` dispatch field + `envLookup`), `NewThreads(opts ThreadsOptions) (*Adapter, error)` 생성자 (`subSource="threads"`, baseURL `https://graph.threads.net/v1.0/keyword_search`, accessToken은 `THREADS_ACCESS_TOKEN`에서), `NewFacebook(opts FacebookOptions) (*Adapter, error)` 생성자 (`subSource="facebook"`, baseURL 없음 — v0는 HTTP 호출 안 함), `(*Adapter).Name()` (subSource 반환), `(*Adapter).Capabilities()` (서브소스별 descriptor §6.3/§6.4), `(*Adapter).Healthcheck(ctx)` (Threads는 TCP-connect probe; Facebook은 `ErrFacebookDisabled` 반환, ADP-006 X Healthcheck `social.go:208-209` 패턴). 컴파일타임 단언 `var _ types.Adapter = (*Adapter)(nil)`. |
| b | `internal/adapters/meta/search.go`: `(*Adapter).Search(ctx, q)` — subSource dispatch. `"threads"` → `searchThreads`; `"facebook"` → `searchFacebookDisabled`. ctx 취소 honour. |
| c | `internal/adapters/meta/search_threads.go`: `(*Adapter).searchThreads(ctx, q)` — Threads 실경로. 질의 검증, `url.Values`로 URL 구성 (`q`, `limit` 1..100 clamp, `search_type=TOP` 하드코딩, `search_mode=KEYWORD` 하드코딩, `since`/`until` filter 존재 시), `Authorization: Bearer <accessToken>` 헤더 설정, `client.go::doRequest` 실행, `parse.go::parseKeywordSearch` 위임. |
| d | `internal/adapters/meta/search_facebook.go`: `(*Adapter).searchFacebookDisabled(ctx, q)` — 영구 비활성. HTTP 요청 0회. 항상 `(nil, &types.SourceError{Adapter:"facebook", Category: CategoryPermanent, Cause: ErrFacebookNotSupported})` 반환 (외부 차단 요인: 공식 API에 공개 키워드 검색 없음). env 게이트 없음 — X(ScrapeCreators 후보 존재)와 달리 Facebook은 공식 경로가 아예 없으므로 opt-in env조차 제공하지 않음. |
| e | `internal/adapters/meta/client.go`: HTTP client 구성 (timeout=10s, `CheckRedirect` per-subSource allowlist `{graph.threads.net}`, `Transport`는 `internal/obs/reqid.NewTransport` wrap), `doRequest(ctx, *http.Request)` (User-Agent + Accept + Authorization Bearer 헤더), `categorizeStatus(httpStatus, retryAfter, cause, adapterName) *types.SourceError`. 401/403 → CategoryPermanent (토큰 만료/권한 부족), 429 → CategoryRateLimited, 5xx/network → CategoryUnavailable. |
| f | `internal/adapters/meta/parse.go`: `parseKeywordSearch(body []byte, retrievedAt time.Time) ([]types.NormalizedDoc, error)` — `{data: [...]}` 응답 파싱, post당 §6.5 매핑. 빈 `data`는 `(nil, nil)` 반환 (민감 키워드 빈배열 정상 동작, research.md §1.5). Graph API 에러 envelope (`{error:{message, type, code}}`) → `*SourceError{CategoryPermanent}`. malformed JSON → `*SourceError{CategoryPermanent}`. |
| g | `internal/adapters/meta/score.go`: `neutralScore() float64` → 상수 0.5 반환 (D5; keyword_search 응답에 engagement 신호 없음). `@MX:NOTE`로 ADP-006 Tanh와의 차이 문서화. |
| h | `internal/adapters/meta/errors.go`: package-private sentinels `ErrInvalidQuery`, `ErrThreadsTokenMissing = errors.New("meta/threads: THREADS_ACCESS_TOKEN not set")` (생성자가 토큰 부재 시 반환 — registry가 등록 스킵), `ErrFacebookNotSupported = errors.New("meta/facebook: the official Facebook Graph API exposes no public-post keyword search endpoint; scraping excluded per tech.md:147")`, `ErrFacebookDisabled` (Healthcheck용). `parseRetryAfter(header, now) time.Duration` (ADP-006 차용). |
| i | `internal/adapters/meta/meta_test.go`: 두 인스턴스 인터페이스 적합성, `Name()` 라우팅, `Capabilities()` 결정성, Healthcheck. |
| j | `internal/adapters/meta/search_test.go`: Threads 실경로 — happy path, 빈 결과(민감 키워드), 429+Retry-After, 401(토큰 만료), 403(권한 부족), 5xx, redirect 허용/거부, since/until filter, ctx 취소, 빈 질의 거부, Bearer 헤더 존재, Graph 에러 envelope. Facebook 비활성 경로 — 항상 `ErrFacebookNotSupported`, HTTP 0회, env 무관 항상 동일. |
| k | `internal/adapters/meta/client_test.go`: `categorizeStatus` truth table, redirect allowlist, 헤더(UA + Accept + Authorization Bearer). |
| l | `internal/adapters/meta/parse_test.go`: §6.5 매핑 table-driven, Snippet 280-rune 절단, 빈 data, Graph 에러 envelope, malformed JSON, Hash 빈값, Metadata 키. |
| m | `internal/adapters/meta/bench_test.go`: `BenchmarkParseKeywordSearch25Docs` (NFR-ADP10-001). `TestMain` → `goleak.VerifyTestMain(m)` (NFR-ADP10-003). |
| n | `internal/adapters/meta/testdata/`: `threads_keyword_search_response.json` (happy path 25 posts), `_empty.json` (빈 data — 민감 키워드/무결과), `_with_media.json` (media_type 혼합), `_graph_error.json` (`{error:{...}}`), `_malformed.json` (truncated). |
| o | `cmd/usearch/query.go` `buildProductionRegistry` 수정: `THREADS_ACCESS_TOKEN` 존재 시에만 `meta.NewThreads`를 Register (GitHub `query.go:476-487` 패턴). Facebook은 등록하지 않음 (공식 경로 부재 — surface는 코드에 존재하나 production registry에 미배선; X와 달리 env opt-in도 없음). |

### 2.2 Out-of-Scope

[HARD] 본 SPEC은 다음을 명시적으로 제외합니다. 각 항목은 알려진 목적지
SPEC을 가지며, 이 목록은 ADP-010으로의 scope creep을 방지합니다.

- **Facebook 공개 콘텐츠 검색의 실제 구현** → 공식 API로 불가능
  (research.md §2). 스크래핑 기반 미래 시도는 SPEC-ADP-010-FBSCRAPE로
  deferred, ToS 승인 전제. v0는 Facebook DISABLED stub만.
- **Threads OAuth 토큰 자동 발급/갱신 플로우** (authorization window,
  code 교환, 60일 long-lived 갱신) → 미래 SPEC-AUTH-* 또는 운영자 책임.
  v0는 정적 `THREADS_ACCESS_TOKEN`을 env로 받고, 만료 시 401→Permanent.
  research.md §7.2.
- **Threads 게시물 작성/답글/insights** (`threads_content_publish` 등
  다른 스코프) → usearch는 read-only 검색 도구. 검색 외 기능 미구현.
- **Threads engagement 점수 정규화** (like/repost 기반 Tanh) →
  keyword_search 응답에 카운트 부재(research.md §1.3); `fields=` 확장
  가능 여부는 run phase 검증(Open Question §11.1). v0는 중립 0.5.
- **재시도 오케스트레이션** → SPEC-FAN-001 (M3, approved).
- **응답 캐싱** → SPEC-CACHE-001 (M3).
- **랭킹/중복제거/RRF 융합** → SPEC-IDX-001 (M3).
- **media (IMAGE/VIDEO) 리치 추출** → v0는 `media_type`을
  `Metadata["media_type"]`에만 surface; 미디어 URL/썸네일 추출 deferred.
- **한국어 토큰화/언어 추론** → SPEC-IDX-003 (M3). keyword_search 응답에
  lang 필드 없음(research.md §5) → `Lang=""`.
- **`pkg/llm` 통합** — 어댑터는 LLM 호출 안 함.
- **라이브 네트워크 통합 테스트 in CI** → httptest + golden 픽스처만.
  env-gated 라이브 테스트(`-tags=integration` + `THREADS_LIVE=1`)
  deferred.
- **per-adapter 커스텀 Prometheus 메트릭** → SPEC-OBS-001 allowlist
  수정 필요; out of scope. 공유 `AdapterCalls{adapter,outcome}`에 두
  라벨값 `"threads"`/`"facebook"` 추가로 충분.
- **`search_mode=TAG` 해시태그 검색 모드** → v0는 `KEYWORD` 하드코딩.
  미래 enhancement.
- **`search_type=RECENT` 시간순 모드** → v0는 `TOP` 하드코딩.
- **`author_username` 필터** → v0 미지원; usearch는 범용 키워드 검색.

### 2.3 Score Neutral Value (Departure from ADP-001/ADP-006)

[HARD] `score.go::neutralScore() float64`는 상수 `0.5`를 반환합니다.
SPEC-ADP-006의 Tanh 정규화(`spec.md:343-374`)와 **의도적으로 다릅니다**:
Threads keyword_search 응답 필드(research.md §1.3: `id`, `text`,
`media_type`, `permalink`, `timestamp`, `username`, `has_replies`,
`is_quote_post`, `is_reply`)에는 **like/repost 등 engagement 카운트가
없어** Tanh 입력 신호가 부재하기 때문입니다. 존재하지 않는 필드를
가정하지 않습니다. SPEC-IDX-001의 RRF 융합은 raw score가 아닌 rank를
가중하므로 중립 score는 어댑터 내 랭킹(keyword_search의 `TOP` 정렬 순서
보존)에 의존합니다. `fields=` 파라미터로 engagement 확장이 가능하다면
미래 iteration에서 Tanh를 채택할 수 있습니다 (Open Question §11.1).

[HARD — 타입 계약 차이 명시] `pkg/types.NormalizedDoc`의 타입 계약은
`Score == 0.0`을 "**unscored**(미채점)"로 정의합니다 — 0.0은 "낮은 점수"가
아니라 "점수 신호 없음"의 관용 sentinel입니다
(`normalized_doc.go:27,37`). 본 SPEC이 "engagement 신호 부재"를 0.0이
**아닌** 0.5로 표현하는 이유가 바로 이것입니다: 0.0을 쓰면 RRF
소비자(SPEC-IDX-001)가 이를 "미채점"으로 해석해 다르게 처리할 수 있고,
Threads 결과는 실제로 채점된 것(중립값으로)이므로 unscored sentinel과
구분되어야 합니다. 0.5는 명시적 "채점됨, 중립" 값입니다. IDX-001 RRF가
0.5 중립을 "중간 신뢰"로 오해하지 않도록, IDX-001 author와의 0.0 vs 0.5
합의는 Open Question §11.1에 추적합니다.

### 2.4 Feasibility Verdict (BACKBONE — per-platform)

[HARD] 본 SPEC의 핵심. research.md §3에 근거한 플랫폼별 판정:

| 플랫폼 | 공식 공개 키워드 검색 | 판정 | 외부 차단 요인 (precondition) |
|--------|----------------------|------|-------------------------------|
| **Threads** | `GET graph.threads.net/v1.0/keyword_search` 존재 | **ACHIEVABLE (조건부)** | (1) Meta 앱 생성, (2) `threads_keyword_search` 앱 심사 **승인**, (3) OAuth long-lived 토큰. 미충족 시 본인 게시물만 검색되어 무의미 → 어댑터 미등록(fail-safe). |
| **Facebook** | 없음 (Graph API search 레퍼런스 HTTP 404) | **NOT ACHIEVABLE via 공식 API** | 공식 엔드포인트 부재. 스크래핑만 가능하나 ToS 리스크로 v0 배제(`tech.md:147`). |

이 판정이 §3 EARS REQs의 구조를 결정합니다: Threads는 양성 동작 REQ
(REQ-ADP10-002..007), Facebook은 Unwanted-pattern REQ
(REQ-ADP10-008)로 영구 비활성 + 외부 차단 요인 명시.

---

## 3. EARS Requirements

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| REQ-ADP10-001 | Ubiquitous | The package `internal/adapters/meta` SHALL expose constructors `NewThreads(opts ThreadsOptions) (*Adapter, error)` and `NewFacebook(opts FacebookOptions) (*Adapter, error)`, both returning `*Adapter` implementing `pkg/types.Adapter` exactly (`Name()`, `Search()`, `Healthcheck()`, `Capabilities()`), with compile-time assertion `var _ types.Adapter = (*Adapter)(nil)`. `NewThreads` SHALL return `(nil, ErrThreadsTokenMissing)` when neither `opts.AccessToken` nor `envLookup("THREADS_ACCESS_TOKEN")` yields a non-empty token (so the registry skips registration). `(*Adapter).Name()` SHALL equal `"threads"` for the Threads instance and `"facebook"` for the Facebook instance. `Capabilities()` SHALL be deterministic and set `SourceID` to match `Name()`. Threads Capabilities SHALL declare `DocTypes=[DocTypePost]`, `SupportedLangs=nil`, `SupportsSince=true`, `RequiresAuth=true`, `AuthEnvVars=["THREADS_ACCESS_TOKEN"]`, `RateLimitPerMin=1` (2200/24h ≈ 1.5/min, floored to 1), `DefaultMaxResults=25`, `DisplayName="Threads"`, `Notes` containing `"graph.threads.net"`, `"keyword_search"`, `"threads_keyword_search permission required"`, and `"meta"`. Facebook Capabilities SHALL declare `DocTypes=[DocTypePost]`, `SupportedLangs=nil`, `SupportsSince=false`, `RequiresAuth=false`, `AuthEnvVars=nil`, `RateLimitPerMin=0`, `DefaultMaxResults=0`, `DisplayName="Facebook"`, `Notes` containing `"NOT SUPPORTED"`, `"no public-post keyword search"`, and `"meta"` (the Notes SHALL NOT assert an unsourced removal version/year — see D1). | P0 | `TestThreadsName`/`TestFacebookName`; `TestThreadsImplementsInterface`/`TestFacebookImplementsInterface`; `TestNewThreadsMissingTokenReturnsError` (no token → `errors.Is(err, ErrThreadsTokenMissing)`, nil adapter); `TestThreadsCapabilitiesDeterministic`/`TestFacebookCapabilitiesDeterministic`; `TestThreadsCapabilitiesShape` (all field values + Notes substrings incl. `"threads_keyword_search permission required"`); `TestFacebookCapabilitiesShape` (Notes contains `"NOT SUPPORTED"` and `"no public-post keyword search"`; SHALL NOT assert any version/year string); `TestThreadsHealthcheckSucceeds` (stub loopback); `TestFacebookHealthcheckReturnsDisabled` (`errors.Is(err, ErrFacebookDisabled)`). All in `meta_test.go`. |
| REQ-ADP10-002 | Event-Driven | WHEN `(*Adapter).Search(ctx, q)` is invoked on the Threads instance with non-empty `q.Text`, the adapter SHALL build an HTTP GET to `https://graph.threads.net/v1.0/keyword_search` with query params `q=<url.QueryEscape(q.Text)>`, `limit=clamp(q.MaxResults,1,100)` (default 25 when 0), `search_type=TOP` (hardcoded), `search_mode=KEYWORD` (hardcoded), `since=<value>`/`until=<value>` (only when present per REQ-ADP10-006), set header `Authorization: Bearer <accessToken>`, execute via the `*http.Client`, parse per REQ-ADP10-005, and return `(docs, nil)` on HTTP 200 with `len(docs) ≤ 100`. (Empty/whitespace-query rejection is specified separately as REQ-ADP10-009.) | P0 | `TestSearchThreadsHappyPath25Posts` (httptest stub returns `threads_keyword_search_response.json`; 25 docs, each `Validate()` nil); `TestSearchThreadsURLParametersRequired` (`q`, `limit`, `search_type=TOP`, `search_mode=KEYWORD` always present); `TestSearchThreadsClampsLimitTo100`; `TestSearchThreadsClampsLimitToMin1` (q.MaxResults=-5 → URL has `limit=1`; lower bound of `clamp(...,1,100)` exercised); `TestSearchThreadsDefaultsLimitTo25`; `TestSearchThreadsSetsBearerToken` (captured `Authorization` header == `"Bearer <token>"`); `TestSearchThreadsEmptyQueryRejectedNoHTTP` (table over `["","   ","\t\n"]` → ErrPermanent + zero requests). In `search_test.go`. |
| REQ-ADP10-003 | Event-Driven | WHEN HTTP 401 or 403 is received from graph.threads.net (token expired, revoked, or `threads_keyword_search` permission not granted), the adapter SHALL return `(nil, &types.SourceError{Adapter:"threads", Category: CategoryPermanent, HTTPStatus:<code>, Cause: errors.New("threads: auth/permission failure: <code>")})`. WHEN HTTP 429 is received, the adapter SHALL parse `Retry-After` (cap 60s, default 5s) and return `CategoryRateLimited`. WHEN HTTP 400/404 is received, the adapter SHALL return `CategoryPermanent`. WHEN HTTP 5xx OR a connection error occurs, the adapter SHALL return `CategoryUnavailable` (HTTPStatus=0 for network errors). The adapter SHALL NOT retry internally. | P0 | `TestSearchThreadsHTTP401` / `TestSearchThreadsHTTP403` (→ ErrPermanent + matching HTTPStatus; error mentions auth/permission); `TestSearchThreadsHTTP429WithRetryAfter` (`Retry-After: 30` → RetryAfter=30s, CategoryRateLimited); `TestSearchThreadsHTTP429Defaults5s`; `TestSearchThreadsHTTP429Capped60s`; `TestSearchThreadsHTTP4xx` (400/404 → ErrPermanent); `TestSearchThreadsHTTP5xx` (500/503 → ErrSourceUnavailable); `TestSearchThreadsConnectionRefused` (HTTPStatus=0); `TestSearchThreadsNoInternalRetry` (request count == 1). In `search_test.go` + `client_test.go`. |
| REQ-ADP10-004 | Event-Driven | WHEN graph.threads.net returns HTTP 200 with a JSON body containing a top-level `error` object (the Graph API error envelope `{"error":{"message":..,"type":..,"code":..}}`), the adapter SHALL return `(nil, &types.SourceError{Adapter:"threads", Category: CategoryPermanent, Cause: fmt.Errorf("threads: %s (code %d)", error.message, error.code)})`. The parser SHALL detect the error envelope BEFORE reading the `data` array. WHEN the body contains a `data` array that is empty (a valid response for a no-result or Meta-sensitive-keyword query per research.md §1.5), the adapter SHALL return `(nil, nil)` — an empty result, NOT an error. | P0 | `TestSearchThreadsGraphErrorEnvelope` (200 + `threads_keyword_search_response_graph_error.json` → ErrPermanent, message + code in error string); `TestParseKeywordSearchErrorBeforeData` (body with both `error` and `data` → error returned, zero docs); `TestSearchThreadsEmptyDataIsEmptyResult` (`_empty.json` → `(nil, nil)`, no error — sensitive-keyword/no-result path). In `search_test.go` + `parse_test.go`. |
| REQ-ADP10-005 | Ubiquitous | The adapter SHALL transform each `data[i]` media object into one `types.NormalizedDoc` per the §6.5 mapping: `RetrievedAt=time.Now().UTC()`, `Hash=""`, `DocType=types.DocTypePost`, `SourceID="threads"`, `ID="threads:"+id`, `URL=permalink`, `Author=username`, `Body=text`, `Snippet=truncateRunes(text,280)`, `Title=truncateRunes(text,280)`, `PublishedAt=parse(timestamp).UTC()` (zero on parse error), `Score=neutralScore()` (=0.5, §2.3), `Lang=""` (no lang field in response). `Metadata` SHALL contain at minimum `{username, permalink, media_type, posted_at, sub_source}` (`sub_source=="threads"`). | P0 | `TestParseKeywordSearchFieldMapping` (table over 3 fixtures: text post, media post, missing-optional-fields post; assert every field per §6.5); `TestParseKeywordSearchScoreNeutral` (every doc `Score==0.5`); `TestParseKeywordSearchLangEmpty` (every doc `Lang==""`); `TestParseKeywordSearchHashEmpty`; `TestParseKeywordSearchMetadataKeys` (5 required keys incl. `sub_source=="threads"`); `TestParseKeywordSearchSnippetTruncation` (>280-rune text → 280-rune Snippet). In `parse_test.go`. |
| REQ-ADP10-006 | Optional | WHERE `Query.Filters` contains an entry with `Key=="since"` AND `Value` parses as a Unix timestamp or RFC 3339 datetime, the adapter SHALL include `since=<value>` in the request URL. Same for `Key=="until"` → `until=<value>`. Filter keys other than `since`/`until` SHALL be silently ignored. Malformed `since`/`until` values SHALL be silently dropped (no error, no param). Default (no filters) omits both. This REQ applies to the Threads sub-source only. | P1 | `TestSearchThreadsSinceFilterAdded` (RFC 3339 valid → URL has `since`); `TestSearchThreadsUntilFilterAdded`; `TestSearchThreadsSinceFilterDroppedWhenMalformed` (`"yesterday"` → no `since`); `TestSearchThreadsUnknownFilterIgnored` (`{tag,"x"}` → no `tag` param). In `search_test.go`. |
| REQ-ADP10-007 | State-Driven | WHILE the same Threads `*Adapter` instance is invoked concurrently from N goroutines (N≥1), each `Search(ctx,q)` SHALL execute independently with no shared mutable state (the `*http.Client` is goroutine-safe; the adapter holds no per-call state; `accessToken` is immutable after construction); the cumulative effect SHALL be N independent dispatches with zero race-detector alarms. WHEN both Threads and Facebook instances are invoked concurrently, there SHALL be no shared mutable state between the two `*Adapter` instances. [HARD] env-dependent acceptance tests SHALL inject env via `Options.EnvLookup` (NOT `t.Setenv` — goroutine-unsafe under `-race`, per ADP-006 H1 `spec.md:24-37`). | P0 | `TestSearchThreadsConcurrentSafe` (50 goroutines × 1 Search on shared instance; `-race` clean; stub observes 50 requests; each receives 25 docs); `TestSearchBothSubSourcesConcurrent` (Threads + Facebook instances, 50 caller goroutines invoking both; `-race` clean; Threads → 25 docs, Facebook → ErrFacebookNotSupported each; no cross-pollination of `subSource`). In `search_test.go`. |
| REQ-ADP10-008 | Unwanted | The adapter SHALL NOT issue ANY HTTP request, perform ANY scraping, or fabricate ANY result for the Facebook sub-source. WHEN `(*Adapter).Search(ctx,q)` is invoked on the Facebook instance, the adapter SHALL return `(nil, &types.SourceError{Adapter:"facebook", Category: CategoryPermanent, Cause: ErrFacebookNotSupported})` for EVERY invocation regardless of query, env, or configuration, because the official Facebook Graph API exposes no public-post keyword search endpoint (verified: the canonical Graph API search reference returns HTTP 404; research.md §2.1) and scraping is excluded per `tech.md:147`. The `ErrFacebookNotSupported` message SHALL document the external blocker (no official endpoint for public-post keyword search) so operators understand this is a permanent platform limitation, not a transient failure or missing config; the message SHALL NOT assert an unsourced removal version or year (see D1 / research.md §2.1 — the verifiable signal is the absent endpoint, not a deprecation date). The adapter SHALL NOT provide a `USEARCH_FACEBOOK_ENABLED`-style opt-in env (unlike ADP-006's X, no viable provider exists to enable). | P0 | `TestSearchFacebookAlwaysNotSupported` (table over varied queries/contexts → always `errors.Is(err, ErrFacebookNotSupported)` AND `errors.Is(err, types.ErrPermanent)`); `TestSearchFacebookMakesNoHTTPRequest` (stub that fails on any request observation → zero requests across all invocations); `TestFacebookNotSupportedMessageDocumentsBlocker` (error string contains `"Facebook Graph API"` AND `"no public-post keyword search"`; the test SHALL NOT assert any version/year string); `TestFacebookNotRegisteredInProductionRegistry` (`buildProductionRegistry()` does not contain an adapter named `"facebook"`). In `search_test.go` + a `cmd/usearch` registry test. |
| REQ-ADP10-009 | Unwanted | IF `(*Adapter).Search(ctx,q)` is invoked on the Threads instance with a `q.Text` that is empty OR contains only Unicode whitespace (per `unicode.IsSpace` over every rune), THEN the adapter SHALL return `(nil, &types.SourceError{Adapter:"threads", Category: CategoryPermanent, Cause: ErrInvalidQuery})` immediately and SHALL NOT issue any HTTP request. | P0 | `TestSearchThreadsEmptyQueryRejectedNoHTTP` (table over `["", "   ", "\t\n"]` for `q.Text`; each → `errors.Is(err, ErrInvalidQuery)` AND `errors.Is(err, types.ErrPermanent)` AND the httptest stub observes zero requests). In `search_test.go`. |

---

## 4. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| NFR-ADP10-001 | Performance (parse path) | `parseKeywordSearch(body, retrievedAt)` SHALL execute with median wall-clock ≤ 5 ms over `go test -bench=BenchmarkParseKeywordSearch25Docs -benchtime=10x -count=5 ./internal/adapters/meta/...` on amd64, against `threads_keyword_search_response.json` (~5KB, 25 posts). Allocation count ≤ 20 per post parsed (≤ 500 total) per `allocs/op`. Same `NormalizedDoc.Metadata=map[string]any` structural floor as ADP-006 NFR-ADP6-001 (`spec.md:399`). Measured via `BenchmarkParseKeywordSearch25Docs` in `bench_test.go`. Benchmarks do not count toward coverage. |
| NFR-ADP10-002 | Secret handling | `THREADS_ACCESS_TOKEN` SHALL NEVER appear in any log record, metric label, span attribute, error message, or `Capabilities` field other than the AuthEnvVars *name* `"THREADS_ACCESS_TOKEN"`. The token SHALL be transmitted ONLY in the `Authorization: Bearer` request header. Verified by `TestThreadsTokenNotInErrorMessages` (force 401/403/5xx and assert the returned `*SourceError.Error()` does NOT contain the token value) and `TestThreadsTokenNotInCapabilities` (assert `Capabilities().Notes` and all string fields do not contain a sample token value). In `meta_test.go` + `search_test.go`. |
| NFR-ADP10-003 | No goroutine leak on cancellation | The adapter SHALL NOT leak any goroutine when the caller's ctx is cancelled mid-`Search`. Verified by `TestSearchThreadsNoGoroutineLeakOnCancel` using `go.uber.org/goleak.VerifyNone(t)` after a mid-flight ctx cancel. `bench_test.go::TestMain` SHALL invoke `goleak.VerifyTestMain(m)` covering both Threads and Facebook-disabled paths (ADP-006 NFR-ADP6-003 `spec.md:401` pattern). |
| NFR-ADP10-004 | Race-clean across sub-sources | `search_test.go::TestSearchBothSubSourcesConcurrent` SHALL execute successfully under `go test -race ./internal/adapters/meta/...` with the REQ-ADP10-007 workload (Threads + Facebook instances, 50 caller goroutines each invoking BOTH). Race-detector alarms attributable to `internal/adapters/meta` SHALL be zero. Env-dependent state driven via `Options.EnvLookup` injection (NOT `t.Setenv`). |

---

## 5. Acceptance Criteria

### REQ-ADP10-001 — Adapter Interface Conformance (Both Sub-Sources)

- `internal/adapters/meta/meta.go` declares `Adapter` struct with the
  documented fields (`httpClient`, `baseURL`, `accessToken`,
  `userAgent`, `healthcheckTarget`, `subSource`, `envLookup`).
- `var _ types.Adapter = (*Adapter)(nil)` appears at the bottom of
  `meta.go`.
- `meta.NewThreads(ThreadsOptions{AccessToken:"t"})` returns a Threads
  `*Adapter`; `meta.NewThreads(ThreadsOptions{})` with no env token
  returns `(nil, ErrThreadsTokenMissing)`.
- `meta.NewFacebook(FacebookOptions{})` returns a Facebook `*Adapter`
  with `subSource="facebook"`.
- `Name()` returns `"threads"` / `"facebook"`.
- Threads `Capabilities()` includes `RequiresAuth=true`,
  `AuthEnvVars=["THREADS_ACCESS_TOKEN"]`, and Notes with
  `"threads_keyword_search permission required"`.
- Facebook `Capabilities().Notes` contains `"NOT SUPPORTED"` and
  `"no public-post keyword search"` (and asserts NO version/year string).
- Capabilities determinism: two consecutive calls `reflect.DeepEqual`.
- Threads `Healthcheck` succeeds against loopback stub; Facebook
  `Healthcheck` returns `ErrFacebookDisabled`.
- All REQ-ADP10-001 tests pass.

### REQ-ADP10-002 — Threads Search Happy Path + Empty-Query Rejection

- `TestSearchThreadsHappyPath25Posts` returns exactly 25 `NormalizedDoc`,
  each `Validate()` nil; URL contains `q`, `limit`, `search_type=TOP`,
  `search_mode=KEYWORD`.
- `Authorization: Bearer <token>` header set on the request.
- Limit bounds tests pass: clamp-to-100 (MaxResults=500 → `limit=100`),
  clamp-to-min-1 (MaxResults=-5 → `limit=1`), and default
  (MaxResults=0 → `limit=25`). All three `clamp(...,1,100)` paths
  exercised.
- `TestSearchThreadsEmptyQueryRejectedNoHTTP`: zero HTTP requests under
  empty/whitespace `q.Text`, returns `ErrPermanent` wrapping
  `ErrInvalidQuery`.

### REQ-ADP10-003 — HTTP Error Mapping (Threads)

- 401 / 403 → ErrPermanent + matching HTTPStatus; error mentions
  auth/permission.
- 429 with integer Retry-After → RetryAfter honoured (cap 60s,
  default 5s), CategoryRateLimited.
- 400 / 404 → ErrPermanent.
- 500 / 503 → ErrSourceUnavailable.
- Connection refused → ErrSourceUnavailable, HTTPStatus=0.
- No internal retry: request count == 1.

### REQ-ADP10-004 — Graph Error Envelope + Empty Data Semantics

- HTTP 200 with `{"error":{...}}` → ErrPermanent; message + code in
  error string; detected BEFORE reading `data`.
- HTTP 200 with empty `data` array → `(nil, nil)` (empty result, NOT
  error — sensitive-keyword / no-result path per research.md §1.5).

### REQ-ADP10-005 — NormalizedDoc Field Mapping (Threads)

- `TestParseKeywordSearchFieldMapping` table over 3 fixtures asserts
  every field per §6.5.
- Score is neutral 0.5 on every doc.
- Lang is "" on every doc.
- Hash is "" on every doc.
- Required Metadata keys present: `username`, `permalink`,
  `media_type`, `posted_at`, `sub_source` (=`threads`).
- Snippet truncated to 280 runes.

### REQ-ADP10-006 — Optional Filters (Threads)

- `since` / `until` filter parsing (valid → URL inclusion; malformed →
  silent drop).
- Unknown filter keys ignored.

### REQ-ADP10-007 — Concurrent Search Safety (State-Driven)

- `TestSearchThreadsConcurrentSafe`: 50 goroutines, `-race` clean, stub
  observes 50 requests, every goroutine receives 25 docs.
- `TestSearchBothSubSourcesConcurrent`: 50 caller goroutines invoking
  both Threads and Facebook; `-race` clean; Threads → 25 docs each,
  Facebook → ErrFacebookNotSupported each; no shared mutable state.

### REQ-ADP10-008 — Facebook Not-Supported (Unwanted / External Blocker)

- `TestSearchFacebookAlwaysNotSupported`: every invocation →
  `ErrFacebookNotSupported` satisfying `errors.Is(err, types.ErrPermanent)`.
- `TestSearchFacebookMakesNoHTTPRequest`: zero requests across all
  invocations (no HTTP, no scraping).
- `TestFacebookNotSupportedMessageDocumentsBlocker`: error string
  contains `"Facebook Graph API"` AND `"no public-post keyword search"`;
  the test asserts NO version/year string (D1).
- `TestFacebookNotRegisteredInProductionRegistry`: production registry
  has no `"facebook"` adapter.

### REQ-ADP10-009 — Empty/Whitespace-Query Rejection (Unwanted)

- `TestSearchThreadsEmptyQueryRejectedNoHTTP`: table over `["", "   ",
  "\t\n"]` for `q.Text`; each returns `ErrPermanent` wrapping
  `ErrInvalidQuery`, and the httptest stub observes zero requests.

### NFR-ADP10-001 — Parse-Path Performance

- `BenchmarkParseKeywordSearch25Docs` median of 5 runs ≤ 5 ms/op;
  `allocs/op ≤ 500`.

### NFR-ADP10-002 — Secret Handling

- `TestThreadsTokenNotInErrorMessages`: token value absent from all
  `*SourceError.Error()` outputs across 401/403/5xx.
- `TestThreadsTokenNotInCapabilities`: token value absent from all
  Capabilities string fields.

### NFR-ADP10-003 — Goroutine Leak Check

- `TestSearchThreadsNoGoroutineLeakOnCancel`: `goleak.VerifyNone(t)`
  succeeds after mid-flight ctx cancel.
- `TestMain` invokes `goleak.VerifyTestMain(m)`.

### NFR-ADP10-004 — Race-Clean Across Sub-Sources

- `TestSearchBothSubSourcesConcurrent` runs under `go test -race`;
  alarms attributable to `internal/adapters/meta` = 0.

---

## 6. Technical Approach

### 6.1 Files to Modify

**Created (13 files + 5 testdata fixtures)**:
- `internal/adapters/meta/meta.go` — Adapter struct, NewThreads,
  NewFacebook, Name, Capabilities, Healthcheck, interface assertion
- `internal/adapters/meta/meta_test.go` — interface conformance
- `internal/adapters/meta/search.go` — sub-source dispatch
- `internal/adapters/meta/search_threads.go` — Threads live path
- `internal/adapters/meta/search_facebook.go` — Facebook not-supported stub
- `internal/adapters/meta/search_test.go` — main test file
- `internal/adapters/meta/client.go` — HTTP client + categorizeStatus
- `internal/adapters/meta/client_test.go` — HTTP error mapping + headers
- `internal/adapters/meta/parse.go` — parseKeywordSearch
- `internal/adapters/meta/parse_test.go` — field mapping
- `internal/adapters/meta/score.go` — neutralScore (=0.5, §2.3)
- `internal/adapters/meta/errors.go` — sentinels + parseRetryAfter
- `internal/adapters/meta/bench_test.go` — benchmark + TestMain (goleak)
- `internal/adapters/meta/testdata/threads_keyword_search_response.json`
- `internal/adapters/meta/testdata/threads_keyword_search_response_empty.json`
- `internal/adapters/meta/testdata/threads_keyword_search_response_with_media.json`
- `internal/adapters/meta/testdata/threads_keyword_search_response_graph_error.json`
- `internal/adapters/meta/testdata/threads_keyword_search_response_malformed.json`

**Modified**:
- `cmd/usearch/query.go` `buildProductionRegistry` (`:458-514`) — add
  `THREADS_ACCESS_TOKEN`-gated `meta.NewThreads` registration following
  the GitHub-token pattern (`:476-487`). Facebook is NOT registered.

**Unchanged (by design)**:
- `internal/adapters/registry.go:172-263` — wrappedAdapter emits ALL
  observability for ADP-010's Search calls. The adapter emits nothing.
- `pkg/types/{adapter.go, capabilities.go, query.go, normalized_doc.go,
  errors.go}` — no contract change.
- `internal/obs/metrics/metrics.go` — no new metric family; two new
  `adapter` label values `"threads"`/`"facebook"` fit the existing
  cardinality budget.

### 6.2 Package Layout

```
internal/adapters/meta/
├── meta.go                # Adapter, NewThreads, NewFacebook, Name, Capabilities, Healthcheck, interface assertion
├── meta_test.go           # Interface conformance + Capabilities determinism + secret-handling
├── search.go              # (*Adapter).Search dispatch
├── search_threads.go      # searchThreads URL build + Bearer header + HTTP + parse delegation
├── search_facebook.go     # searchFacebookDisabled — always ErrFacebookNotSupported, zero HTTP
├── search_test.go         # E2E + happy path + error mapping + concurrent safety
├── client.go              # *http.Client, doRequest, categorizeStatus
├── client_test.go         # categorizeStatus + redirect allowlist + headers (incl. Bearer)
├── parse.go               # parseKeywordSearch (Threads keyword_search envelope)
├── parse_test.go          # Field mapping table tests
├── score.go               # neutralScore (=0.5; §2.3 departure from ADP-006 Tanh)
├── errors.go              # ErrInvalidQuery + ErrThreadsTokenMissing + ErrFacebookNotSupported + ErrFacebookDisabled + parseRetryAfter
├── bench_test.go          # BenchmarkParseKeywordSearch25Docs + TestMain (goleak)
└── testdata/
    ├── threads_keyword_search_response.json        # Happy path 25 posts
    ├── threads_keyword_search_response_empty.json  # Empty data (sensitive/no-result)
    ├── threads_keyword_search_response_with_media.json
    ├── threads_keyword_search_response_graph_error.json
    └── threads_keyword_search_response_malformed.json
```

[NOTE on duplication vs sharing] `parseRetryAfter`, `categorizeStatus`,
and the redirect-allowlist pattern duplicate equivalents in
`internal/adapters/{reddit,hn,social}/`. This is INTENTIONAL in v0 (ADP-006
§6.2 same posture). A cross-adapter `common/` extraction is deferred to
SPEC-ADP-REFAC-001 post-M3.

### 6.3 Threads Capabilities Descriptor (Detailed)

```go
types.Capabilities{
    SourceID:          "threads",
    DisplayName:       "Threads",
    DocTypes:          []types.DocType{types.DocTypePost},
    SupportedLangs:    nil,
    SupportsSince:     true,
    RequiresAuth:      true,
    AuthEnvVars:       []string{"THREADS_ACCESS_TOKEN"},
    RateLimitPerMin:   1,   // coarse floor signal only; TRUE limit is a per-user/24h budget, not per-minute (research §1.5; see Notes)
    DefaultMaxResults: 25,
    Notes: "Threads (Meta) via graph.threads.net keyword_search. " +
        "meta. OAuth 2.0 Bearer token (THREADS_ACCESS_TOKEN). " +
        "threads_keyword_search permission required for full public-post " +
        "search; without it only the authed user's own posts are returned " +
        "(research §1.1). search_type=TOP, search_mode=KEYWORD hardcoded. " +
        "since/until filters from Query.Filters. RATE LIMIT: 2200 queries " +
        "per 24h PER USER (per token), across all apps — this is a daily " +
        "budget, NOT a per-minute cap. RateLimitPerMin=1 is a coarse floor " +
        "only; consumers (SPEC-FAN-001) MUST treat this as a daily token " +
        "budget, not a 1-call/min global limit. No engagement counts in " +
        "response → Score=0.5 neutral (research §1.3).",
}
```

### 6.4 Facebook Capabilities Descriptor (Not-Supported Detailed)

```go
types.Capabilities{
    SourceID:          "facebook",
    DisplayName:       "Facebook",
    DocTypes:          []types.DocType{types.DocTypePost},
    SupportedLangs:    nil,
    SupportsSince:     false,
    RequiresAuth:      false,
    AuthEnvVars:       nil,
    RateLimitPerMin:   0,
    DefaultMaxResults: 0,
    Notes: "Facebook (Meta) meta. NOT SUPPORTED. The official Facebook " +
        "Graph API exposes no public-post keyword search endpoint " +
        "(verified: canonical Graph API search reference returns HTTP 404; " +
        "research §2.1). No official endpoint exists to keyword-search " +
        "public posts/Pages you do not own. Scraping is excluded per " +
        "tech.md:147 (ToS risk). All Search calls return " +
        "ErrFacebookNotSupported (permanent). Surface reserved for future " +
        "SPEC-ADP-010-FBSCRAPE only if a ToS-compliant path emerges; no " +
        "opt-in env provided in v0.",
}
```

### 6.5 Threads keyword_search media → NormalizedDoc Field Mapping

| keyword_search field | NormalizedDoc field | Transform |
|----------------------|---------------------|-----------|
| `id` | `ID` | `"threads:" + id` |
| (constant) | `SourceID` | `"threads"` (matches `Name()`) |
| `permalink` | `URL`, `Metadata["permalink"]` | use as-is |
| `username` | `Author`, `Metadata["username"]` | use as-is |
| `text` | `Body` | use as-is (plain text) |
| `truncateRunes(text, 280)` | `Title`, `Snippet` | first 280 runes |
| `timestamp` (RFC 3339 or Unix) | `PublishedAt` | UTC parse; zero on error |
| `timestamp` | `Metadata["posted_at"]` | original string |
| (parse time) | `RetrievedAt` | `time.Now().UTC()` |
| `media_type` | `Metadata["media_type"]` | TEXT/IMAGE/VIDEO |
| `has_replies`/`is_reply`/`is_quote_post` | `Metadata[...]` | bool as-is (optional keys) |
| `neutralScore()` | `Score` | constant 0.5 (§2.3; no engagement signal) |
| (constant) | `Lang` | `""` (no lang field in response) |
| (constant) | `DocType` | `types.DocTypePost` |
| (nil) | `Citations` | `nil` |
| (constructed) | `Metadata` | REQUIRED keys: `username`, `permalink`, `media_type`, `posted_at`, `sub_source`(=`"threads"`). OPTIONAL: `has_replies`, `is_reply`, `is_quote_post`. |
| (constant) | `Hash` | `""` (consumers compute via `CanonicalHash()`) |

[HONESTY] keyword_search response (research.md §1.3) exposes NO engagement
counts and NO language code. The mapping above does not invent them.
`Score` is neutral and `Lang` is empty by design, not by omission.

### 6.6 HTTP Client Construction Notes

- **Timeout**: 10s total (caller ctx deadline takes precedence when
  shorter).
- **Redirect policy**: `CheckRedirect` allowlist `{graph.threads.net}`,
  max 3 hops; cross-domain rejected with `CategoryPermanent`.
- **Transport**: `reqid.NewTransport(http.DefaultTransport)` for
  observability correlation.
- **Headers per request**: `User-Agent: usearch/<version>
  (+https://github.com/elymas/universal-search)`, `Accept:
  application/json`, and `Authorization: Bearer <accessToken>`.
  [HARD] The token appears ONLY here (NFR-ADP10-002).

### 6.7 Observability Note

Both Threads and Facebook Adapter instances emit ZERO metrics, logs, and
spans of their own. ALL observability comes from the registry's
`wrappedAdapter` (`internal/adapters/registry.go:172-263`). The two
distinct `Name()` values produce two `adapter` label values
(`"threads"`, `"facebook"`). The `outcome` values consumed: `"success"`
(Threads 200), `"rate_limited"` (Threads 429), `"unavailable"` (Threads
5xx/network), `"timeout"` (ctx deadline), `"failure"` (Threads 4xx, Graph
error envelope, Facebook not-supported). NO new label value introduced;
SPEC-OBS-001 allowlist preserved.

Note: since Facebook is not registered in production
(`buildProductionRegistry`), the `adapter="facebook"` label is emitted
only in tests that construct the Facebook instance directly.

### 6.8 MX Tag Plan

| File::Symbol | Tag | Reason |
|--------------|-----|--------|
| `meta.go::(*Adapter).Search` (in `search.go`) | `@MX:ANCHOR` | Sole entry for Meta fanout (Threads + Facebook). fan_in ≥ 3 (registry wrappedAdapter, FAN-001, tests). `@MX:REASON: contract boundary; sub-source dispatch`. `@MX:SPEC: SPEC-ADP-010`. |
| `parse.go::parseKeywordSearch` | `@MX:ANCHOR` | Every Threads doc passes through this transform. `@MX:REASON: NormalizedDoc field-mapping integrity gate`. `@MX:SPEC: SPEC-ADP-010`. |
| `client.go::doRequest` | `@MX:WARN` | Outbound network call carrying the Bearer token. `@MX:REASON: removing CheckRedirect re-opens SSRF; token must stay in Authorization header only (NFR-ADP10-002)`. `@MX:SPEC: SPEC-ADP-010`. |
| `search_facebook.go::searchFacebookDisabled` | `@MX:WARN` | External-blocker boundary. `@MX:REASON: wiring any Facebook scraping here violates tech.md:147 ToS mandate; no official API path exists`. `@MX:SPEC: SPEC-ADP-010`. |
| `score.go::neutralScore` | `@MX:NOTE` | Documents the 0.5 neutral departure from ADP-006 Tanh — keyword_search has no engagement signal. |
| `meta.go::Adapter.accessToken` field | `@MX:WARN` | Secret. `@MX:REASON: must never be logged/metric-labelled; Authorization header only (NFR-ADP10-002)`. `@MX:SPEC: SPEC-ADP-010`. |

All tags `[AUTO]`-prefixed, include `@MX:SPEC: SPEC-ADP-010`, follow
`code_comments: en`. Per-file limits (3 ANCHOR + 5 WARN per
`.moai/config/sections/mx.yaml`): respected.

### 6.9 Harness Level

13 EARS REQs-equivalent surface (9 REQs + 4 NFRs) touching 1 new package
(`internal/adapters/meta/`, ~13 source/test files + 5 fixtures) + ONE
cross-package edit (`cmd/usearch/query.go` registry wiring) + ONE new
secret env var (`THREADS_ACCESS_TOKEN`, handled via AuthEnvVars +
NFR-ADP10-002 secret discipline) = **standard** harness level. The
secret-handling NFR raises scrutiny but stays within standard (no
payment/PII, no new config file). Sprint Contract OPTIONAL. Evaluator
profile `default`.

---

## 7. What NOT to Build (Exclusions)

[HARD] 본 SPEC은 다음을 명시적으로 제외합니다. 각 항목은 알려진 목적지를
가지며, ADP-010으로의 scope creep을 방지합니다.

- **Facebook 공개 콘텐츠 검색의 실제 동작** → 공식 API 부재
  (research.md §2). 미래 스크래핑 시도는 SPEC-ADP-010-FBSCRAPE,
  ToS 승인 전제. v0는 DISABLED stub만 (REQ-ADP10-008).
- **Threads OAuth 토큰 발급/60일 갱신 자동화** → 미래 SPEC-AUTH-* 또는
  운영자 책임. v0는 정적 `THREADS_ACCESS_TOKEN` env, 만료 시 401.
- **Threads engagement 기반 점수 정규화** → 응답에 카운트 부재; v0 중립
  0.5. `fields=` 확장 가능성은 run phase 검증 (Open Question §11.1).
- **Threads 게시물 작성/답글/insights** → usearch는 read-only 검색.
- **`search_type=RECENT` / `search_mode=TAG` / `author_username`** →
  v0는 TOP/KEYWORD 하드코딩, author 필터 미지원.
- **media (IMAGE/VIDEO) 리치 추출** → `media_type`만 surface.
- **재시도 오케스트레이션** → SPEC-FAN-001 (M3, approved).
- **응답 캐싱** → SPEC-CACHE-001 (M3).
- **랭킹/중복제거/RRF 융합** → SPEC-IDX-001 (M3).
- **한국어 토큰화/언어 추론** → SPEC-IDX-003 (M3). `Lang=""`.
- **`pkg/llm` 통합** → 어댑터는 LLM 호출 안 함.
- **라이브 네트워크 통합 테스트 in CI** → httptest + golden 픽스처만.
  env-gated 라이브(`-tags=integration` + `THREADS_LIVE=1`) deferred.
- **per-adapter 커스텀 Prometheus 메트릭** → SPEC-OBS-001 allowlist
  수정 필요; out of scope.
- **`USEARCH_FACEBOOK_ENABLED` opt-in env** → ADP-006의 X와 달리
  활성화할 viable provider가 없으므로 제공하지 않음 (REQ-ADP10-008).
- **cross-adapter helper 추출** (reddit/hn/social과 공유) → out of v0;
  SPEC-ADP-REFAC-001 post-M3.

---

## 8. TDD Plan

Development follows RED-GREEN-REFACTOR per `quality.development_mode: tdd`
(`.moai/config/sections/quality.yaml`). Representative RED-phase tests,
written before implementation, grouped by REQ. Coverage target: 85% per
`quality.test_coverage_target`. Benchmarks do not count toward coverage.

| # | Test | Layer | REQ | Assertion |
|---|------|-------|-----|-----------|
| 1 | `TestThreadsName` | `meta_test.go` | REQ-ADP10-001 | `Name() == "threads"` |
| 2 | `TestFacebookName` | `meta_test.go` | REQ-ADP10-001 | `Name() == "facebook"` |
| 3 | `TestThreadsImplementsInterface` | `meta_test.go` | REQ-ADP10-001 | Compile-time `var _ types.Adapter` |
| 4 | `TestFacebookImplementsInterface` | `meta_test.go` | REQ-ADP10-001 | Same — both share *Adapter |
| 5 | `TestNewThreadsMissingTokenReturnsError` | `meta_test.go` | REQ-ADP10-001 | No token → `ErrThreadsTokenMissing`, nil adapter |
| 6 | `TestThreadsCapabilitiesDeterministic` | `meta_test.go` | REQ-ADP10-001 | Two calls DeepEqual |
| 7 | `TestFacebookCapabilitiesDeterministic` | `meta_test.go` | REQ-ADP10-001 | Two calls DeepEqual |
| 8 | `TestThreadsCapabilitiesShape` | `meta_test.go` | REQ-ADP10-001 | All field values + Notes incl. `"threads_keyword_search permission required"` |
| 9 | `TestFacebookCapabilitiesShape` | `meta_test.go` | REQ-ADP10-001 | Notes contains `"NOT SUPPORTED"` / `"no public-post keyword search"` (no version/year asserted) |
| 10 | `TestThreadsHealthcheckSucceeds` | `meta_test.go` | REQ-ADP10-001 | TCP dial loopback succeeds |
| 11 | `TestFacebookHealthcheckReturnsDisabled` | `meta_test.go` | REQ-ADP10-001 | `errors.Is(err, ErrFacebookDisabled)` |
| 12 | `TestSearchThreadsHappyPath25Posts` | `search_test.go` | REQ-ADP10-002, 005 | 25 docs; each `Validate()` nil |
| 13 | `TestSearchThreadsURLParametersRequired` | `search_test.go` | REQ-ADP10-002 | `q`, `limit`, `search_type=TOP`, `search_mode=KEYWORD` present |
| 14 | `TestSearchThreadsClampsLimitTo100` | `search_test.go` | REQ-ADP10-002 | MaxResults=500 → `limit=100` |
| 14a | `TestSearchThreadsClampsLimitToMin1` | `search_test.go` | REQ-ADP10-002 | MaxResults=-5 → `limit=1` (lower bound of clamp) |
| 15 | `TestSearchThreadsDefaultsLimitTo25` | `search_test.go` | REQ-ADP10-002 | MaxResults=0 → `limit=25` |
| 16 | `TestSearchThreadsSetsBearerToken` | `client_test.go` | REQ-ADP10-002 | `Authorization: Bearer <token>` |
| 17 | `TestSearchThreadsEmptyQueryRejectedNoHTTP` | `search_test.go` | REQ-ADP10-002 | Empty/whitespace → ErrPermanent + zero requests |
| 18 | `TestSearchThreadsHTTP401` | `search_test.go` | REQ-ADP10-003 | 401 → ErrPermanent, HTTPStatus=401, mentions auth |
| 19 | `TestSearchThreadsHTTP403` | `search_test.go` | REQ-ADP10-003 | 403 → ErrPermanent (permission not granted) |
| 20 | `TestSearchThreadsHTTP429WithRetryAfter` | `search_test.go` | REQ-ADP10-003 | `Retry-After: 30` → RetryAfter=30s, RateLimited |
| 21 | `TestSearchThreadsHTTP429Defaults5s` | `search_test.go` | REQ-ADP10-003 | No header → 5s |
| 22 | `TestSearchThreadsHTTP429Capped60s` | `search_test.go` | REQ-ADP10-003 | `Retry-After: 999` → 60s |
| 23 | `TestSearchThreadsHTTP4xx` | `search_test.go` | REQ-ADP10-003 | 400/404 → ErrPermanent |
| 24 | `TestSearchThreadsHTTP5xx` | `search_test.go` | REQ-ADP10-003 | 500/503 → ErrSourceUnavailable |
| 25 | `TestSearchThreadsConnectionRefused` | `search_test.go` | REQ-ADP10-003 | HTTPStatus=0, ErrSourceUnavailable |
| 26 | `TestSearchThreadsNoInternalRetry` | `search_test.go` | REQ-ADP10-003 | Request count == 1 |
| 27 | `TestSearchThreadsGraphErrorEnvelope` | `search_test.go` | REQ-ADP10-004 | 200 + `{error:{...}}` → ErrPermanent, message+code |
| 28 | `TestParseKeywordSearchErrorBeforeData` | `parse_test.go` | REQ-ADP10-004 | error detected before reading data |
| 29 | `TestSearchThreadsEmptyDataIsEmptyResult` | `search_test.go` | REQ-ADP10-004 | empty data → `(nil, nil)`, no error |
| 30 | `TestParseKeywordSearchFieldMapping` | `parse_test.go` | REQ-ADP10-005 | Table over 3 fixtures; every field per §6.5 |
| 31 | `TestParseKeywordSearchScoreNeutral` | `parse_test.go` | REQ-ADP10-005 | Every doc `Score==0.5` |
| 32 | `TestParseKeywordSearchLangEmpty` | `parse_test.go` | REQ-ADP10-005 | Every doc `Lang==""` |
| 33 | `TestParseKeywordSearchHashEmpty` | `parse_test.go` | REQ-ADP10-005 | Every doc `Hash==""` |
| 34 | `TestParseKeywordSearchMetadataKeys` | `parse_test.go` | REQ-ADP10-005 | 5 required keys incl. `sub_source=="threads"` |
| 35 | `TestParseKeywordSearchSnippetTruncation` | `parse_test.go` | REQ-ADP10-005 | >280-rune text → 280-rune Snippet |
| 36 | `TestParseKeywordSearchMalformedJSON` | `parse_test.go` | REQ-ADP10-005 | Truncated → `*SourceError{CategoryPermanent}` |
| 37 | `TestSearchThreadsSinceFilterAdded` | `search_test.go` | REQ-ADP10-006 | RFC 3339 valid → URL `since` |
| 38 | `TestSearchThreadsUntilFilterAdded` | `search_test.go` | REQ-ADP10-006 | valid → URL `until` |
| 39 | `TestSearchThreadsSinceFilterDroppedWhenMalformed` | `search_test.go` | REQ-ADP10-006 | malformed → no `since` |
| 40 | `TestSearchThreadsUnknownFilterIgnored` | `search_test.go` | REQ-ADP10-006 | unknown key → no param |
| 41 | `TestSearchThreadsSetsCustomUserAgent` | `client_test.go` | REQ-ADP10-002 | UA starts `"usearch/"` + contains repo URL |
| 42 | `TestSearchThreadsSetsAcceptJSON` | `client_test.go` | REQ-ADP10-002 | `Accept: application/json` |
| 43 | `TestCategorizeStatusTable` | `client_test.go` | REQ-ADP10-003 | Truth table over status codes |
| 44 | `TestSearchThreadsRejectsCrossDomainRedirect` | `client_test.go` | REQ-ADP10-003 | 302 to attacker.com → ErrPermanent |
| 45 | `TestParseRetryAfterTable` | `client_test.go` | REQ-ADP10-003 | Table over inputs |
| 46 | `TestSearchThreadsConcurrentSafe` | `search_test.go` | REQ-ADP10-007, NFR-ADP10-004 | 50 goroutines; `-race` clean; 50 requests; 25 docs each |
| 47 | `TestSearchBothSubSourcesConcurrent` | `search_test.go` | REQ-ADP10-007, NFR-ADP10-004 | 50 callers × both adapters; race-clean; Threads 25 / Facebook ErrFacebookNotSupported |
| 48 | `TestSearchFacebookAlwaysNotSupported` | `search_test.go` | REQ-ADP10-008 | Every invocation → `ErrFacebookNotSupported` + `ErrPermanent` |
| 49 | `TestSearchFacebookMakesNoHTTPRequest` | `search_test.go` | REQ-ADP10-008 | Zero requests across all invocations |
| 50 | `TestFacebookNotSupportedMessageDocumentsBlocker` | `search_test.go` | REQ-ADP10-008 | Error contains `"Facebook Graph API"` + `"no public-post keyword search"` (no version/year asserted) |
| 51 | `TestFacebookNotRegisteredInProductionRegistry` | `cmd/usearch` registry test | REQ-ADP10-008 | No `"facebook"` adapter in `buildProductionRegistry()` |
| 52 | `TestThreadsTokenNotInErrorMessages` | `search_test.go` | NFR-ADP10-002 | Token absent from all `*SourceError.Error()` (401/403/5xx) |
| 53 | `TestThreadsTokenNotInCapabilities` | `meta_test.go` | NFR-ADP10-002 | Token absent from Capabilities string fields |
| 54 | `TestSearchThreadsNoGoroutineLeakOnCancel` | `search_test.go` | NFR-ADP10-003 | `goleak.VerifyNone(t)` after mid-flight cancel |
| 55 | `BenchmarkParseKeywordSearch25Docs` | `bench_test.go` | NFR-ADP10-001 | Median of 5 ≤ 5ms; allocs/op ≤ 500 |
| 56 | `TestMain` (goleak.VerifyTestMain) | `bench_test.go` | NFR-ADP10-003 | Package-level goroutine leak check |

RED-GREEN-REFACTOR per requirement:
1. RED: Write failing test for REQ-ADP10-N.
2. GREEN: Implement minimal code to pass.
3. REFACTOR: Tidy; keep each `.go` file < 200 LoC (excl. tests).

Greenfield note: `internal/adapters/meta/` does not exist. No behaviour
to preserve; no characterization tests needed.

---

## 9. Dependencies

### 9.1 Upstream SPEC Dependencies

- **SPEC-ADP-006 (implemented)**: provides the social adapter reference
  shape — 2-서브소스-1-패키지 dispatch, DISABLED-sub-source Capabilities
  pattern (X → Facebook), `Options.EnvLookup` test-isolation discipline,
  redirect allowlist + `categorizeStatus` + `parseRetryAfter` patterns.
  HARD dep (structural template).
- **SPEC-CORE-001 (implemented)**: `pkg/types.Adapter`,
  `pkg/types.Capabilities`, `pkg/types.Query`,
  `pkg/types.NormalizedDoc`, `*types.SourceError`,
  `types.OutcomeFromError`, `types.DocType`, `internal/adapters.Registry`
  with wrappedAdapter sole-emitter. HARD dep.
- **SPEC-IR-001 (implemented)**: `Capabilities` consumer contract.
  ADP-010's `DocTypes=[DocTypePost]` + `SupportedLangs=nil` determines
  Threads selection for `Category=social/meta` queries. SOFT dep.

### 9.2 Parallelizable

- **SPEC-ADP-003..009 (M3)**: independent package directories; no
  conflict.
- **SPEC-IDX-001 (M3)**: consumes `[]NormalizedDoc` shape (locked in
  CORE-001).

### 9.3 Downstream Blocked SPECs

- **SPEC-FAN-001 (M3, approved)**: consumes `(*meta.Adapter).Search`
  for Threads via `registry.Get("threads").Search(ctx, q)`. Facebook is
  not registered, so it does not appear in fanout.
- **SPEC-IDX-001 (M3)**: consumes `NormalizedDoc.Score` (neutral 0.5
  for Threads) as one RRF input (rank-weighted).
- **SPEC-ADP-010-FBSCRAPE (deferred)**: future Facebook scraping behind
  ToS acknowledgement.
- **SPEC-AUTH-* (deferred)**: future Threads OAuth token lifecycle
  management.

### 9.4 External Dependencies (run-phase pins)

**Zero new Go module dependencies.** ADP-010 uses only:
- Go stdlib: `context`, `encoding/json`, `errors`, `fmt`, `io`, `net`,
  `net/http`, `net/url`, `os`, `strconv`, `strings`, `time`, `unicode`,
  `unicode/utf8`
- `pkg/types` (SPEC-CORE-001), `internal/obs/reqid` (SPEC-OBS-001)
- Test-only: `go.uber.org/goleak` (already pinned via ADP-001)

No Meta official Go SDK exists; no unofficial SDK adopted (research.md
§1.7). The `THREADS_ACCESS_TOKEN` is the only runtime external input.

---

## 10. Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Meta revises/removes the Threads keyword_search endpoint (young API) | Medium | High | research.md §8 notes Threads API is new and changing; run phase MUST re-verify docs before implementation. Adapter reads only documented fields; `encoding/json` ignores unknown ones. |
| `threads_keyword_search` app review NOT granted → only own-posts returned | High | High | External blocker (research.md §1.6). Adapter registers only when token present; operator must complete app review. Capabilities.Notes documents the precondition. This is the dominant feasibility risk → P3 priority. |
| Token leaks into logs/metrics/errors | Low | High | NFR-ADP10-002 + `TestThreadsTokenNotInErrorMessages` + `@MX:WARN` on accessToken field. Token in Authorization header only. |
| Operator assumes Facebook will work | High | Low | REQ-ADP10-008 `ErrFacebookNotSupported` message documents the permanent blocker; Capabilities.Notes says "NOT SUPPORTED". Facebook not registered in production. |
| keyword_search has no engagement signal → poor ranking | Medium | Medium | v0 neutral Score=0.5; relies on keyword_search `TOP` ordering. RRF (SPEC-IDX-001) weights rank. `fields=` expansion explored in run phase (Open Question §11.1). |
| 2200/24h-per-user rate limit binds quickly under load | Medium | Medium | `RateLimitPerMin=1` in Capabilities is a coarse floor signal; [HARD] the REAL limit is a **per-user (per-token) 2200-query/24h budget, NOT a per-minute cap and NOT per-app**. SPEC-FAN-001 SHALL treat Threads as a daily token-budget source, not a 1-call/min source — a per-minute interpretation would under-call by ~91x. Capabilities.Notes (§6.3) documents this explicitly so the router does not misread the field. Single shared token is a v0 limitation. |
| Empty `data` (sensitive keyword) misread as error | Medium | Low | REQ-ADP10-004 explicitly maps empty `data` → `(nil, nil)`, not error. `TestSearchThreadsEmptyDataIsEmptyResult` verifies. |
| timestamp format ambiguity (Unix vs RFC 3339) | Medium | Low | Parser tries RFC 3339 then Unix; zero on failure; original preserved in `Metadata["posted_at"]`. |
| Token expiry mid-operation (60-day long-lived) | Medium | Medium | 401 → CategoryPermanent; operator must refresh. Auto-refresh deferred (Out-of-Scope). |
| Cross-domain redirect (SSRF) on Threads | Low | High | `redirectAllowlist={graph.threads.net}`; cross-domain rejected. `TestSearchThreadsRejectsCrossDomainRedirect`. |
| Concurrent calls on shared `*http.Client` race | Low | High | `*http.Client` goroutine-safe; `TestSearchThreadsConcurrentSafe` under `-race`. NFR-ADP10-004. |
| Goroutine leak on ctx cancel | Low | High | NFR-ADP10-003 + `goleak.VerifyTestMain`. |
| Facebook surface tempts a future contributor to wire scraping | Medium | High | `@MX:WARN` on `searchFacebookDisabled` referencing tech.md:147; REQ-ADP10-008 forbids HTTP/scraping; no opt-in env. |

---

## 11. Open Questions

미해결 사항. 각각 권고 기본값을 가지며 SPEC 승인을 차단하지 않습니다.

1. **keyword_search engagement 필드 확장 + Score 0.0 vs 0.5 계약 합의**.
   (a) `fields=` 파라미터로 like_count 등을 요청할 수 있는가?
   (b) `NormalizedDoc.Score` 타입 계약상 `0.0`은 "unscored"
   (`normalized_doc.go:27`)인데, Threads의 "engagement 신호 부재"를 0.0이
   아닌 0.5 중립으로 표현하는 것을 SPEC-IDX-001 RRF 소비자가 그대로
   수용하는가, 아니면 다른 sentinel(예: 별도 unscored 플래그)을 원하는가?
   **권고 기본값**: (a) v0는 불가 가정, `Score=0.5` 중립; run phase 라이브
   검증 후 가능하면 ADP-006식 Tanh 채택. (b) 0.5 중립 유지 — 0.0은 IDX-001
   에서 unscored로 오해될 수 있으므로 명시적 채점-중립값 0.5가 더 안전
   (§2.3 참조). IDX-001 author 확인 필요.
   **Resolution owner**: run-phase 구현자 + SPEC-IDX-001 author.

2. **Threads OAuth 토큰 60일 갱신 주체**. 어댑터 vs 별도 토큰 관리
   컴포넌트.
   **권고 기본값**: v0는 정적 env 토큰, 갱신은 운영자 책임 (만료 시
   401→Permanent). 미래 SPEC-AUTH-*가 자동화.
   **Resolution owner**: 미래 SPEC-AUTH-* author.

3. **Facebook 영구 배제 vs 미래 스크래핑 SPEC**.
   **권고 기본값**: v0 영구 DISABLED. ToS-compliant 경로가 등장하면
   SPEC-ADP-010-FBSCRAPE로 재검토.
   **Resolution owner**: 미래 SPEC author.

4. **패키지 위치 `meta/` vs `social/` 통합**.
   **권고 기본값**: 신규 `meta/` (research.md §4.3 — OAuth 인증 축 격리).
   **Resolution owner**: 본 SPEC D2에서 확정; structure.md sync는 다음
   `/moai sync`에서.

5. **`media_type=IMAGE/VIDEO` 결과 포함 여부**.
   **권고 기본값**: 전부 포함, DocType=post 일괄; media URL 추출은
   deferred.
   **Resolution owner**: run-phase 구현자.

---

## 12. References

### External (URL-cited; research.md §8에서 WebFetch 검증)

- https://developers.facebook.com/docs/threads/keyword-search — Threads
  keyword_search 엔드포인트/파라미터/응답/권한/레이트리밋 (1차 출처).
- https://developers.facebook.com/docs/threads/get-started — OAuth 2.0
  토큰 모델, 앱 심사, 스코프.
- https://developers.facebook.com/docs/threads — Threads API 개요.
- https://developers.facebook.com/docs/graph-api/reference/v22.0/search
  — HTTP 404 (공개 search 레퍼런스 부재 신호).
- https://developers.facebook.com/docs/graph-api/using-graph-api/ —
  공개 게시물 검색 표면 부재 확인.
- RFC 7231 §7.1.3 — `Retry-After` 헤더 의미론.

### Internal (file:line cited)

- `.moai/specs/SPEC-ADP-010/research.md` — full research artifact.
- `.planning/AUDIT-FINDINGS.md:23` — F-09 원문.
- `.moai/specs/SPEC-ADP-006/spec.md` — social 어댑터 reference shape
  (구조 템플릿); `:24-37` t.Setenv 금지, `:163-178` X DISABLED
  Capabilities, `:243-246` fediverse 분리 시사, `:391` 관측성 패턴.
- `.moai/specs/SPEC-CORE-001/spec.md` — Adapter / Capabilities / Query /
  NormalizedDoc / SourceError 계약.
- `.moai/specs/SPEC-IR-001/spec.md` — Capabilities 소비자 계약.
- `.moai/specs/SPEC-FAN-001/spec.md` — multi-source fanout.
- `pkg/types/adapter.go:28-45` — Adapter 4-메서드.
- `pkg/types/capabilities.go:14-22` — DocType 열거.
- `pkg/types/capabilities.go:38-62` — Capabilities struct.
- `internal/adapters/social/social.go:33-40` — Adapter struct 형상.
- `internal/adapters/social/social.go:73-124` — NewBluesky/NewX 2-서브소스
  생성자 패턴.
- `internal/adapters/social/social.go:163-178` — xCapabilities DISABLED
  레퍼런스.
- `internal/adapters/social/social.go:181-213` — Search dispatch +
  Healthcheck DISABLED.
- `cmd/usearch/query.go:458-514` — buildProductionRegistry; auth-gated
  등록 패턴 (GitHub `:476-487`, YouTube `:488-494`).
- `internal/adapters/registry.go:172-263` — wrappedAdapter sole-emitter.
- `internal/obs/metrics/metrics.go` — AdapterCalls/AdapterCallDuration
  collectors; adapter/outcome 카디널리티 allowlist.
- `internal/obs/reqid` — request-ID 전파 transport.
- `.moai/project/tech.md:147` — ToS feature-flag 의무 (Facebook 스크래핑
  배제 근거).
- `.moai/config/sections/quality.yaml` — `development_mode: tdd`,
  `test_coverage_target: 85`.
- `.moai/config/sections/language.yaml` — `documentation: en`,
  `code_comments: en`.
- `go.mod` — `go.uber.org/goleak` 기존 pin; Meta atproto/SDK deps 없음.

---

*End of SPEC-ADP-010 v0.1 (DRAFT)*
