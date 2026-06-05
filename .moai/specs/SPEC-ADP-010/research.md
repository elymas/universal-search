# SPEC-ADP-010 Research — Facebook + Threads (Meta) Adapter Feasibility

연구 일자: 2026-06-04 · 작성자: limbowl (manager-spec) · GSD 감사 항목: **F-09**

이 문서는 SPEC-ADP-010의 사실 근거(backbone)입니다. 모든 외부 주장은
WebFetch 출처를 인용하고, 모든 내부 주장은 `file:line`으로 인용합니다.
**이 SPEC은 공식 API로 구현이 불가능할 수도 있는 기능에 대한 것입니다.**
따라서 연구의 1차 목표는 "행복 경로(happy path)를 발명하지 않는 것"이며,
플랫폼별로 현실을 정직하게 판정하는 것입니다.

---

## 0. F-09 원문과 연구 질문

`.planning/AUDIT-FINDINGS.md:23` (severity medium, class manual/feature):

> "Facebook and Threads have no adapter at all (0 code). Meta Graph API
> offers no public-feed keyword search → large new-adapter + auth design."

F-09의 핵심 단언("Meta Graph API에는 공개 피드 키워드 검색이 없다")은
**절반만 맞습니다.** 연구 결과 Facebook(파란 앱)에 대해서는 참이지만,
Threads에 대해서는 거짓입니다 — Meta가 별도의 Threads API를 출시하면서
전용 `keyword_search` 엔드포인트를 추가했기 때문입니다. 따라서 판정은
반드시 **플랫폼별로** 내려야 합니다.

연구 질문:
1. **Threads API** — 공식 API 표면은 무엇인가? 공개 게시물의 키워드/주제
   검색이 가능한가, 아니면 본인 계정 읽기/쓰기 전용인가? OAuth 모델은?
2. **Facebook Graph API** — 공개 콘텐츠 검색의 현재 상태는? 무엇이 키워드로
   검색 가능한가? 앱 심사/권한 요건은?
3. usearch 스타일 키워드 검색 어댑터(질의 텍스트 → 랭크된 공개
   `types.Document` 결과)가 각 플랫폼에서 달성 가능한가?

---

## 1. Threads API — 사실 조사 (ACHIEVABLE, 단 조건부)

### 1.1 키워드 검색 엔드포인트는 실재한다 (검증됨)

WebFetch로 `https://developers.facebook.com/docs/threads/keyword-search`를
조회한 결과, 다음이 **명시적으로 문서화**되어 있습니다:

- **엔드포인트**: `GET https://graph.threads.net/v1.0/keyword_search`
- **검색 범위 (CRITICAL)**: `threads_keyword_search` 승인이 **없으면**
  "search is performed only on posts owned by the authenticated user"
  (본인 게시물만). 승인이 **있으면** "search the full public posts"
  (전체 공개 게시물). → **usearch가 원하는 "공개 게시물 키워드 검색"은
  `threads_keyword_search` 권한이 승인된 경우에만 가능하다.**

### 1.2 파라미터 (검증됨)

| 이름 | 타입 | 비고 |
|------|------|------|
| `q` | string | **필수**; 검색 키워드 |
| `search_type` | string | 선택; `TOP`(기본) 또는 `RECENT` |
| `search_mode` | string | 선택; `KEYWORD`(기본) 또는 `TAG` |
| `media_type` | string | 선택; `TEXT`, `IMAGE`, `VIDEO` |
| `since` | timestamp | 선택; Unix 타임스탬프 또는 파싱 가능한 날짜 |
| `until` | timestamp | 선택; 동일 |
| `limit` | integer | 선택; 기본 25, 최대 100 |
| `author_username` | string | 선택; 특정 사용자명으로 필터 |

### 1.3 응답 구조 (검증됨)

`data` 배열에 media 객체들이 담겨 반환되며, 각 객체는 다음 필드를
포함합니다: `id`, `text`, `media_type`, `permalink`, `timestamp`,
`username`, `has_replies`, `is_quote_post`, `is_reply`.

이 형상은 SPEC-CORE-001의 `pkg/types.NormalizedDoc` 계약에 직접 매핑
가능합니다 (§5 매핑 표 참조). `permalink`이 URL, `text`가 Body/Snippet,
`username`이 Author, `timestamp`이 PublishedAt이 됩니다.

### 1.4 인증 모델 (검증됨)

WebFetch `https://developers.facebook.com/docs/threads/get-started`:

- **OAuth 2.0** 사용자 액세스 토큰 기반.
- **Short-lived 토큰**: 1시간 유효 → long-lived로 교환 가능.
- **Long-lived 토큰**: 60일 유효; 만료 전 `GET /refresh_access_token`로
  갱신 가능.
- 인가 흐름: authorization window → authorization code(1시간 유효) →
  short-lived token → long-lived token.
- **앱 심사 필수**: 역할이 할당되지 않은 앱은 공개 출시 전 각 권한에 대해
  "app review" 승인을 받아야 비-tester 사용자에게 접근 권한이 부여됨.
- 스코프: `threads_basic`(모든 호출 필수), `threads_content_publish`,
  `threads_manage_replies`, `threads_read_replies`,
  `threads_manage_insights`. **`threads_keyword_search`는 별도 권한으로
  keyword-search 페이지(§1.1)에 문서화되어 있음** — get-started 페이지의
  기본 스코프 목록에는 나오지 않음.

### 1.5 레이트 리밋 (검증됨)

keyword-search 페이지 명시:
- "Maximum 2,200 queries within any continuous 24-hour period"
- "Queries returning no results are not deducted from the limit"
- **사용자(user)당** 제한이며 앱당이 아님 — 한 사용자의 모든 앱에 걸쳐 합산.
- "Requests containing keywords Meta considers sensitive return empty
  arrays" → 민감 키워드는 에러가 아니라 **빈 배열**로 반환됨.

24h/2,200 = 약 91/시간 ≈ **분당 1.5회**. SPEC-FAN-001의 `MaxParallel`
캡과 결합하면 매우 빠듯한 예산이며, 다른 어댑터와 달리 익명 호출이 아니라
**사용자 토큰당** 한도라는 점이 운영상 핵심 제약입니다.

### 1.6 외부 차단 요인 (BLOCKER PRECONDITION)

Threads 경로가 **공개 게시물**을 반환하려면 다음 선행 조건이 모두
충족되어야 합니다 (§1.1 + §1.4):
1. Meta 개발자 앱 생성 + Threads 유스케이스 구성.
2. `threads_basic` + `threads_keyword_search` 권한에 대한 **Meta 앱 심사
   승인** (외부 차단 요인 — usearch가 통제할 수 없음).
3. 유효한 OAuth 2.0 long-lived 사용자 액세스 토큰 확보 및 60일 갱신
   운영.

승인 전에는 엔드포인트가 본인 게시물만 검색하므로 usearch 용도로는
무의미합니다. **이 선행 조건은 SPEC에서 명시적으로 기술하며,
어댑터는 토큰 부재 시 등록되지 않습니다(env-gated registration).**

### 1.7 Go 라이브러리 의존성 — 불필요 (판정)

Threads keyword_search는 단일 HTTP GET + JSON 응답입니다. SPEC-ADP-006의
D3 결정(`internal/adapters/social/social.go:1` — indigo 거부, stdlib만 사용)과
동일하게 **별도 Go 모듈 의존성 없이** `net/http` + `encoding/json`으로
충분합니다. Meta용 공식 Go SDK는 존재하지 않으며, 비공식 SDK는 채택하지
않습니다.

---

## 2. Facebook (파란 앱) Graph API — 사실 조사 (NOT ACHIEVABLE)

### 2.1 공개 게시물/페이지 키워드 검색은 공식 API로 불가능 (판정)

**판정의 근거 = 검증된 현재-상태 신호 (1차).** 본 연구에서 Meta의 정식
search 레퍼런스 URL(`/docs/graph-api/reference/v22.0/search`,
`/docs/graph-api/reference/search`)을 WebFetch로 조회한 결과 **둘 다
일관되게 HTTP 404**가 반환되었습니다. 동일 실행에서 Threads
keyword-search 페이지는 HTTP 200으로 정상 조회되었으므로, 404는 단순
네트워크 오류가 아니라 **해당 레퍼런스(공개 search 엔드포인트)가 더 이상
존재하지 않음**을 가리킵니다. 추가로 Meta get-started 및 using-graph-api
문서 어디에도 공개 게시물/페이지 키워드 검색 표면이 등장하지 않습니다.
→ **현재 공식 채널로 본인이 소유하지 않은 Facebook 공개 게시물/페이지를
키워드로 검색할 방법은 없다**는 결론은 이 검증된 신호만으로 성립합니다.

[HONESTY — 미검증 맥락, 판정 근거 아님] 일반적으로 알려진 바로는
`type=post` 공개 게시물 검색이 Graph API v2.0(2015) 무렵, 나머지 공개
검색 타입(`page`/`place`/`event`/`user`)이 2018–2020에 걸쳐 제거된 것으로
이해됩니다. **그러나 본 연구는 이 구체적 버전/연도에 대한 Meta 1차 출처
(공식 changelog/공지 URL)를 확보하지 못했습니다** (WebSearch 도구가 본
실행 환경에서 비활성화). 따라서 이 버전/연도는 배경 맥락일 뿐이며,
**NOT-ACHIEVABLE 판정도 SPEC의 어떤 acceptance/Notes 문자열도 이 미검증
날짜에 의존하지 않습니다** (plan-audit D1). 판정은 오직 위의 404 신호 +
공개 검색 표면 부재에 근거합니다. run phase 진입 전, 버전/연도를 SPEC에
다시 넣으려면 Meta 1차 changelog URL을 본 §8에 인용해야 합니다.

[HONESTY] 본 SPEC은 Facebook 공개 키워드 검색용으로 **존재하지 않는
엔드포인트를 발명하지 않습니다.**

### 2.2 본인이 소유한 페이지만 접근 가능 (usearch 용도에 부적합)

Graph API로 접근 가능한 것은 **앱이 권한을 가진(본인 소유 또는 명시적
승인된)** Page/게시물뿐입니다. usearch의 어댑터 계약은 "임의의 질의
텍스트 → 임의의 공개 게시물 랭킹 결과"이므로, 소유 페이지 한정 접근은
어댑터 계약을 충족하지 못합니다.

### 2.3 스크래핑 = ToS 리스크, v0에서 배제 (판정)

비공식 경로(HTML 스크래핑, mbasic 파싱 등)는 Facebook ToS 위반 위험이
있습니다. 이는 SPEC-ADP-006이 X(트위터)에 대해 취한 입장과 동일합니다:
`tech.md:147`의 "ToS-grey 스크래핑 소스는 feature-flag opt-in"
의무를 준수하여 **v0에서는 어떤 스크래핑도 출하하지 않습니다.**
SPEC-ADP-006의 X 처리 패턴(`internal/adapters/social/social.go:163-178`
xCapabilities — DISABLED 상태로 surface만 예약)을 그대로 차용합니다.

---

## 3. 어댑터 달성 가능성 종합 판정 (per-platform verdict)

| 플랫폼 | 공식 공개 키워드 검색 | 판정 | 근거 |
|--------|----------------------|------|------|
| **Threads** | `graph.threads.net/v1.0/keyword_search` (단, `threads_keyword_search` 앱 심사 승인 필요) | **ACHIEVABLE (조건부)** | §1.1–1.6 |
| **Facebook** | 없음 (공식 search 레퍼런스 HTTP 404; 스크래핑만 가능, ToS 리스크) | **NOT ACHIEVABLE via 공식 API** | §2.1–2.3 |

→ SPEC-ADP-010의 v0 범위:
- **Threads**: OAuth 토큰 게이트 + `threads_keyword_search` 승인 전제로
  실제 통합. 토큰 부재 시 어댑터 미등록(env-gated). SPEC-ADP-006의
  Bluesky 실경로 형상을 차용하되, 익명 대신 **OAuth Bearer 토큰** 사용.
- **Facebook**: SPEC-ADP-006의 X와 동일하게 **RESERVED-but-DISABLED**.
  구조(constructor + Capabilities surface)만 예약하고 Search는 영구
  에러(외부 차단 요인 문서화). 스크래핑 출하 금지.

---

## 4. 코드 템플릿 검증 (file:line 인용)

본 SPEC이 따를 구조 템플릿이 실재하는지 확인했습니다.

### 4.1 어댑터 계약 (검증됨)

- `pkg/types/adapter.go:28-45` — `Adapter` 인터페이스 4-메서드:
  `Name() string`, `Search(ctx, Query) ([]NormalizedDoc, error)`,
  `Healthcheck(ctx) error`, `Capabilities() Capabilities`.
- `pkg/types/capabilities.go:38-62` — `Capabilities` struct 정확한 필드:
  `SourceID`, `DisplayName`, `DocTypes []DocType`,
  `SupportedLangs []string`, `SupportsSince bool`, `RequiresAuth bool`,
  `AuthEnvVars []string`, `RateLimitPerMin int`, `DefaultMaxResults int`,
  `Notes string`. (작업 지시서의 `DocTypes`/`SupportedLangs`/
  `RequiresAuth`/`AuthEnvVars`/`DisplayName`/`RateLimitPerMin` 필드명
  모두 확인 — 추가로 `SupportsSince`/`DefaultMaxResults`/`Notes`/
  `SourceID` 존재.)
- `pkg/types/capabilities.go:14-22` — `DocType` 열거: `DocTypePost`,
  `DocTypeSocial` 등 존재. Threads/Facebook 게시물은 `DocTypePost`.

### 4.2 구조 템플릿 = social 패키지 (검증됨)

- `internal/adapters/social/social.go:33-40` — `Adapter` struct
  (httpClient + baseURL + userAgent + healthcheckTarget + subSource +
  envLookup).
- `internal/adapters/social/social.go:73-124` — `NewBluesky`/`NewX`
  생성자. **두 생성자가 동일한 `*Adapter`를 반환**하고 `subSource`로
  분기. `Search`는 `social.go:181-194`에서 `subSource` switch로 dispatch
  (`searchBluesky` / `searchX`).
- `internal/adapters/social/social.go:163-178` — `xCapabilities()` =
  DISABLED 상태 어댑터의 Capabilities 형상 레퍼런스. Facebook의 DISABLED
  surface가 이를 직접 미러링.
- `internal/adapters/social/social.go:199-213` — Healthcheck: Bluesky는
  TCP dial, X는 `ErrXDisabled` 반환. Facebook도 DISABLED 반환 패턴.
- (search_bluesky.go) — HTTP search → normalize → Document 실경로 형상.
  Threads의 실경로가 이를 미러링하되 OAuth Bearer 헤더를 추가.

### 4.3 패키지 위치 결정 (권고)

작업 지시서는 두 후보를 제시합니다: 기존 `internal/adapters/social/`에
서브소스를 추가 vs 신규 `internal/adapters/meta/`. **권고: 신규
`internal/adapters/meta/` 패키지.** 근거:
- social 패키지는 이미 Bluesky(익명 AppView) + X(stub) 두 서브소스로
  포화. Meta는 **OAuth 2.0 사용자 토큰 + 60일 갱신 + 앱 심사**라는
  근본적으로 다른 인증 축을 가져, social의 익명/env-gate 모델과 섞이면
  단일 패키지의 응집도가 무너짐.
- SPEC-ADP-006 §1(`spec.md:243-246`)은 "Threads + Mastodon under one
  `fediverse/` package" 같은 미래 분리를 명시적으로 시사 — 즉 social
  패키지는 무한 확장 의도가 아님.
- `meta/` 패키지는 social의 2-서브소스-1-패키지 dispatch 패턴
  (Threads 실경로 + Facebook DISABLED stub)을 그대로 차용하되 OAuth
  관심사를 격리.

### 4.4 레지스트리 배선 지점 (검증됨)

- `cmd/usearch/query.go:458-514` `buildProductionRegistry` — 신규
  어댑터가 여기서 등록됨. auth-gated 어댑터는 env 확인 후 Register
  (예: `query.go:476-487` GitHub은 토큰 존재 시에만,
  `query.go:488-494` YouTube는 base URL 존재 시에만 등록).
  → Threads 어댑터는 `THREADS_ACCESS_TOKEN` 존재 시에만 등록하는
  동일 패턴.

---

## 5. Threads post → NormalizedDoc 필드 매핑 (제안; 최종형은 run phase)

| keyword_search media 필드 | NormalizedDoc 필드 | 변환 |
|---------------------------|--------------------|------|
| `id` | `ID` | `"threads:" + id` |
| (상수) | `SourceID` | `"threads"` (`Name()`과 일치) |
| `permalink` | `URL` | 그대로 사용 |
| `username` | `Author`, `Metadata["username"]` | 그대로 |
| `text` | `Body` | 그대로 (plain text) |
| `truncateRunes(text, 280)` | `Snippet`, `Title` | 처음 280 runes |
| `timestamp` (RFC 3339/Unix) | `PublishedAt`, `Metadata["posted_at"]` | UTC 파싱; 실패 시 zero |
| (파싱 시각) | `RetrievedAt` | `time.Now().UTC()` |
| `media_type` | `Metadata["media_type"]` | TEXT/IMAGE/VIDEO |
| `is_reply`/`is_quote_post`/`has_replies` | `Metadata[...]` | bool 그대로 |
| (상수) | `DocType` | `types.DocTypePost` |
| (상수) | `Score` | v0: 중립 0.5 (keyword_search는 like/repost 카운트를 기본 필드로 노출하지 않음 — §1.3 검증; ADP-006의 Tanh 입력 신호가 없으므로 중립값. Open Question §11.2) |
| (상수) | `Hash` | `""` (소비자가 `CanonicalHash()` 계산) |
| (상수) | `Lang` | `""` (keyword_search 응답에 lang 필드 없음 — 검증됨) |

[HONESTY] keyword_search 응답 필드 목록(§1.3)에는 **engagement 카운트
(like/repost)와 language 코드가 포함되지 않습니다.** 따라서 ADP-006식
Tanh 점수 정규화의 입력 신호가 부재하며, v0는 `Score=0.5` 중립값을
사용합니다. 추가 필드를 `fields=` 파라미터로 확장 요청 가능한지는 run
phase에서 라이브 검증이 필요(Open Question).

---

## 6. 관측성 / 보안 / 테스트 규율 (SPEC-ADP-006 차용)

- **관측성**: 신규 Prometheus 메트릭 패밀리 0개. 레지스트리
  `wrappedAdapter`가 `adapter="threads"`/`adapter="facebook"` 두 라벨로
  모든 per-call 메트릭 방출 (ADP-006 `spec.md:391` REQ-ADP6-010 패턴).
- **시크릿 처리**: `THREADS_ACCESS_TOKEN`은 절대 로깅/메트릭 라벨에
  포함하지 않음. env로만 주입, Capabilities.AuthEnvVars에 이름만 노출.
- **테스트 격리**: env 의존 테스트는 `Options.EnvLookup` 주입 사용 —
  `t.Setenv` 금지(`-race`에서 goroutine-unsafe, ADP-006 H1 결정
  `spec.md:24-37` 참조).

---

## 7. Open Questions (미해결, SPEC 승인 차단 아님)

1. Threads keyword_search 응답에 `fields=` 파라미터로 like_count 등
   engagement를 확장 요청할 수 있는가? → run phase 라이브 검증.
   기본 default: 불가 가정, `Score=0.5` 중립.
2. Threads OAuth long-lived 토큰 60일 자동 갱신을 어댑터가 직접 할지,
   별도 토큰 관리 컴포넌트(SPEC-AUTH-*)가 할지. → default: v0는 정적
   토큰을 env로 받고, 갱신은 운영자 책임(어댑터는 만료 시 401→Permanent).
3. Facebook을 영구 배제할지, 미래 SPEC-ADP-010-FBSCRAPE로 ToS 승인
   하에 남길지. → default: 미래 SPEC으로 deferred, v0는 DISABLED stub.
4. 패키지 위치: `meta/`(권고) vs `social/` 통합. → §4.3 default: `meta/`.
5. `media_type=IMAGE/VIDEO` 결과를 v0에서 포함할지 텍스트만 필터할지.
   → default: 전부 포함, DocType=post 일괄.

---

## 8. References

### External (WebFetch 검증; 2026-06-04)

- https://developers.facebook.com/docs/threads/keyword-search — Threads
  keyword_search 엔드포인트, 파라미터, 응답, 권한, 레이트 리밋, 민감
  키워드 빈배열 동작. (1차 출처, 조회 성공)
- https://developers.facebook.com/docs/threads/get-started — OAuth 2.0
  토큰 모델(short/long-lived, refresh), 앱 심사 요건, 스코프 목록.
  (조회 성공)
- https://developers.facebook.com/docs/threads — Threads API 개요,
  Keyword Search 기능 존재 확인. (조회 성공)
- https://developers.facebook.com/docs/graph-api/reference/v22.0/search
  — **HTTP 404** (공개 search 레퍼런스 부재 신호). (조회 실패=신호)
- https://developers.facebook.com/docs/graph-api/reference/search —
  **HTTP 404** (동일 신호). (조회 실패=신호)
- https://developers.facebook.com/docs/graph-api/using-graph-api/ —
  search/공개 게시물 검색 언급 없음 확인. (조회 성공)

검증 한계: WebSearch 도구가 본 실행 환경에서 비활성화되어 Facebook
공개 검색 제거 시점에 대한 제3자 교차검증은 수행하지 못함. Facebook
NOT-ACHIEVABLE 판정은 (a) Meta 공식 search 레퍼런스의 일관된 404,
(b) get-started/using-graph-api 문서에 공개 게시물 검색 표면 부재,
(c) Threads에만 전용 keyword_search가 별도로 추가되었다는 사실에
근거함. run phase 진입 전 Meta 문서 최신성 재확인 권장.

### Internal (file:line 인용)

- `.planning/AUDIT-FINDINGS.md:23` — F-09 원문.
- `pkg/types/adapter.go:28-45` — Adapter 4-메서드 인터페이스.
- `pkg/types/capabilities.go:14-22` — DocType 열거.
- `pkg/types/capabilities.go:38-62` — Capabilities struct 필드.
- `internal/adapters/social/social.go:33-40` — Adapter struct 형상.
- `internal/adapters/social/social.go:73-124` — NewBluesky/NewX 생성자
  (2-서브소스-1-패키지 패턴).
- `internal/adapters/social/social.go:163-178` — xCapabilities DISABLED
  레퍼런스.
- `internal/adapters/social/social.go:181-213` — Search dispatch +
  Healthcheck DISABLED 패턴.
- `cmd/usearch/query.go:458-514` — buildProductionRegistry 배선 지점;
  auth-gated 등록 패턴(GitHub `:476-487`, YouTube `:488-494`).
- `.moai/specs/SPEC-ADP-006/spec.md:24-37` — t.Setenv 금지 / EnvLookup
  주입 결정(H1).
- `.moai/specs/SPEC-ADP-006/spec.md:243-246` — fediverse 미래 분리 시사.
- `.moai/specs/SPEC-ADP-006/spec.md:391` — REQ-ADP6-010 관측성 패턴.
- `.moai/project/tech.md:147` — ToS-grey 소스 feature-flag opt-in 의무
  (SPEC-ADP-006 인용, ADP-010이 Facebook에 동일 적용).

---

*End of SPEC-ADP-010 research (2026-06-04)*
