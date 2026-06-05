# SPEC Review Report: SPEC-ADP-006-XENABLE

Iteration: 1/3
Verdict: PASS-WITH-FIXES
Overall Score: 0.83

프롬프트로 전달된 저자 측 추론 맥락은 M1 Context Isolation 원칙에 따라 검증 대상으로만
취급했다. 모든 인용은 `spec.md` / `research.md`와 **실제 소스 코드** 및 **라이브 외부
API 문서**(curl로 직접 fetch)를 대조하여 독립 확인했다. 결론은 "예상이 맞을 것"이 아니라
파일·HTTP 응답을 읽은 증거에 근거한다.

요약: 이 SPEC는 이 저장소의 과거 stale-citation 이력과 달리 **인용 정확도가 매우 높다**.
저자가 부모 SPEC 대비 주장한 두 가지 drift-correction(① 실제 `XOptions`에는 `EnvLookup`만
있고 `HealthcheckTarget`은 없음, ② X `Healthcheck`는 TCP probe가 아니라 `ErrXDisabled`를
직접 반환)은 **둘 다 실제 코드에서 사실로 확인**되었다(`social.go:61-66`, `social.go:208-209`).
세 가지 외부 API 주장도 라이브 소스로 모두 검증되었다. 결함은 발견됐지만 critical은 없고,
**의미적 주장 1건(D1)** 과 **frontmatter 컨벤션(D3)** 만 ready 전 보정이 필요하다.
BLOCK이 아니라 PASS-WITH-FIXES로 판정한다.

---

## Must-Pass Results

- [PASS] MP-1 REQ number consistency: `spec.md:L181-L189`. 기능 요구 `REQ-XEN-001..009`
  순차, 3자리 zero-pad 일관, 갭·중복 없음. NFR `NFR-XEN-001..004`(`spec.md:L197-L200`)
  순차. 확인 완료.
- [PASS] MP-2 EARS format compliance: `spec.md:L181-L189`. 9개 REQ 모두 패턴 라벨과
  실제 문장이 정합. REQ-XEN-001/006 Ubiquitous("The package/adapter SHALL …"),
  REQ-XEN-002/005/007 Event-Driven("WHEN … the adapter SHALL …"), REQ-XEN-003/009
  State-Driven("WHILE … SHALL …"), REQ-XEN-004 Unwanted("IF … THEN … SHALL …"),
  REQ-XEN-008 Optional("WHERE … the provider SHALL …"). 다섯 패턴 전부 사용.
  정규문(REQ/NFR 표 행)에 weasel word 없음(`appropriate/adequate/reasonable/proper`
  스캔 결과 0건; "best-effort"는 §6.4 매핑표·§10 리스크 행의 서술용이며 "zero on error"로
  결과가 이진 정의됨). 확인 완료.
- [FAIL] MP-3 YAML frontmatter validity: `spec.md:L1-L16`. 필수 키 `created_at` 대신
  `created`(L11) 사용 — auditor MP-3 firewall(required: id, version, status,
  **created_at**, priority, labels) 기준 hard FAIL. 단 `labels`(L15)·`priority`(L7)·
  `id`·`version`·`status`는 모두 존재하며, `created` vs `created_at` 명명은
  **이 프로젝트 전반의 frontmatter 관례**다(SPEC-CLI-003-review-1, SPEC-UI-002-review-1도
  동일 사유로 FAIL 기록 — 일관성 있음). 내용 품질과 무관한 스키마 컨벤션이며 fix 비용 최저(D3).
- [N/A] MP-4 Section 22 language neutrality: N/A — 단일 패키지 Go 어댑터
  (`internal/adapters/social/`) 대상. 16개 언어 툴링 열거 대상 아님. 자동 통과.

MP-3가 firewall 상 엄밀 FAIL 트리거이나, 프로젝트 관례임을 감안해 verdict는 내용 결함(D1)을
우선시한 PASS-WITH-FIXES로 기록하고 MP-3는 fix list 필수 항목으로 둔다.

---

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.85 | 0.75–1.0 — 거의 모든 REQ가 단일 해석. 단 §2.3 `normalizeScore`의 `max(...,0)` 주장이 실제 함수 동작과 불일치(D1)하여 합리적 엔지니어가 "재구현해야 하나"로 오해할 여지 | `spec.md:L168-172`; `internal/adapters/social/score.go:24-28` |
| Completeness | 0.85 | 0.75–1.0 — HISTORY/Purpose/Scope/REQ/AC/Technical/Exclusions/Deps/Risks/OpenQuestions 전부 존재; 실패 모드(429/4xx/5xx/network/ctx-cancel/blank-query/nil-provider/goroutine-leak/secret-leak) 망라; Exclusions 12개 구체 항목. created_at 누락(D3)만 감점 | `spec.md:L20-76,L136-164,L462-491` |
| Testability | 0.90 | 0.75–1.0 — REQ 9 + NFR 4 → §8 테스트 34개 모두 명명·이진 검증; `fakeProvider`/`httptest.Server`로 라이브 네트워크 0; goleak·race 게이트 명시. Title==Snippet(둘 다 280-rune) 같은 사소한 모호함(D5)만 잔존 | `spec.md:L181-189,L502-537` |
| Traceability | 1.00 | 1.0 — `REQ-XEN-001..009` + `NFR-XEN-001..004` 전부 §5 수용기준 + §8 테스트행 보유. orphan AC·dangling REQ 없음; 테스트가 실재 REQ만 참조 | `spec.md:L206-290,L504-537` |

---

## Defects Found

D1. `spec.md:L168-172` (§2.3) — **MAJOR (code-citation / invented behavior).**
SPEC는 "The X live path reuses `score.go::normalizeScore(likeCount, repostCount int)
float64` … with `x = max(LikeCount + RepostCount, 0)`"라고 [HARD]로 단언한다. 그러나
**실제** `internal/adapters/social/score.go:25`는 `x := float64(likeCount + repostCount)`로,
`max(...,0)` clamp이 **존재하지 않는다**. 재사용하겠다고 인용한 함수에 그 함수가 수행하지
않는 가드를 귀속시킨 것 — 정확히 이 저장소가 취약한 stale/invented-behavior 클래스다.
실무상 like/repost 카운트는 음수가 아니므로 런타임 영향은 없지만, [HARD] 주장이 인용 함수와
모순되므로 보정 필요.
**Fix**: `max(...,0)` 표현을 삭제하고 `x = LikeCount + RepostCount`로 정정하거나,
"카운트는 비음수로 가정(provider 보장)"임을 명시. 인용 함수를 바꾸지 말 것(재사용이 목적).

D2. `spec.md:L640` (§12 Internal 인용) 및 research `L454-460` — **MINOR (off-by-one 인용).**
`social.go:60-66`(XOptions)은 실제로 **L61-66**(L60은 주석), `163-178`(xCapabilities)은
**L164-178**(L163은 주석), `181-213`은 `Search`(L181-194)+`Healthcheck`(L199-213) 두 함수에
걸친 묶음 인용이다. 행 번호가 주석/선언 경계에서 ±1 어긋난다. 심볼·범위 의미는 모두 정확하며
구현에 영향 없음.
**Fix**: 선택적. `social.go:61-66,164-178,199-213`으로 미세 조정하면 line-accurate.

D3. `spec.md:L11` (frontmatter) — **MINOR (스키마 컨벤션, MP-3).**
필수 키 `created_at` 대신 `created` 사용. 프로젝트 전반 관례지만 auditor firewall상 FAIL.
**Fix**: `created: 2026-06-04` → `created_at: "2026-06-04"`로 변경(또는 프로젝트가
`created` 컨벤션을 공식화하면 firewall 예외 문서화).

D4. `spec.md:L186,L188,L404` (REQ-XEN-006 vs REQ-XEN-008 vs §6.4) — **MINOR (계층 간 미세 불일치).**
REQ-XEN-008(P2)은 official provider가 `public_metrics.quote_count`를 `XTweet.QuoteCount`로
**매핑하라**고 요구하지만, §6.4·REQ-XEN-006의 Metadata 필수 8키에는 `quote_count`가 빠지고
"OPTIONAL"로만 둔다. XTweet 필드(수집)와 Metadata 키(출력)는 다른 계층이라 모순은 아니나,
`quote_count`를 수집까지 의무화하면서 출력은 선택으로 두는 비대칭이 테스트 의도를 흐릴 수 있다.
**Fix**: §6.4 Metadata에 `quote_count`를 REQUIRED로 승격하거나, REQ-XEN-008의 quote_count
매핑을 "OPTIONAL(존재 시)"로 완화하여 한쪽으로 통일.

D5. `spec.md:L393,L395` (§6.4 매핑) — **MINOR (설계 모호).**
`Title` = first 280 runes of text, `Snippet` = first 280 runes of text → X 문서는 항상
`Title == Snippet`이 된다(트윗엔 별도 제목 없음). Bluesky 경로도 유사하므로 일관은 하나,
다운스트림(SPEC-IDX-001 RRF/표시)이 Title을 제목으로 취급하면 노이즈가 될 수 있다.
**Fix**: 선택적. `Title=""`(또는 short-form) 정책을 §6.4에 명시하거나 현 동작을 의도로 1줄 주석.

---

## External-API Claim Verification (라이브 소스 직접 fetch, 2026-06-04)

- **(a) ScrapeCreators X 키워드 검색 부재 → [검증됨/REJECTED 정당]**:
  `https://docs.scrapecreators.com/openapi.json` 파싱 결과 twitter 경로는
  `/v1/twitter/{profile, user-tweets, tweet, tweet/transcript, community, community/tweets}`
  뿐이며 **search 류 경로 0건**. SPEC의 "검색용 엔드포인트 없음 → 검색 용도 REJECT"
  (`spec.md:L145-146,L475-476`; research §2.2)는 정확. 엔드포인트 날조 아님.
- **(b) X 공식 API v2 ~$0.005/Post read → [검증됨, pricing은 drift 가능]**:
  `https://docs.x.com/x-api/getting-started/pricing`에서 `0.005` 39회, `pay-per`/`credit`
  모델 다수 확인. recent-search 엔드포인트 `GET /2/tweets/search/recent`·`tweet.fields`·
  `next_token`(research §2.1)는 공식 문서 기반으로 구현 가능. 가격은 변동 가능하나 날조 아님.
- **(c) twitterapi.io ~$0.15/1k tweets → [검증됨]**:
  `https://twitterapi.io/pricing`에서 `$0.15`, `15 credits`, `100,000 credits/$1` 확인.
  `advanced_search` 엔드포인트의 `queryType`/`cursor`/`next_cursor`/`X-API-Key`도 라이브
  문서에서 확인 → **`XProvider` 인터페이스는 최소 1개(실은 2개) 실 provider로 구현 가능**.

세 주장 모두 invented endpoint/capability 없음. 가격은 minor drift 리스크만 존재(현재 일치).

---

## Consistency with Parent SPEC-ADP-006

- **2-state disabled semantics 보존 → [정확]**: REQ-XEN-003(`spec.md:L183`)이
  env≠"true"→`ErrXDisabled`, env=="true" & provider==nil→`ErrXProviderNotConfigured`를
  명시. 실제 `search_x.go:23-31`·`errors.go:19-34`의 두 sentinel(`*types.SourceError`,
  `CategoryPermanent`)과 정합. 두 sentinel이 `errors.Is(err, types.ErrPermanent)` 만족
  주장도 `errors.go`의 `CategoryPermanent` 래핑과 부합.
- **default(env unset) 하위호환 → [정확]**: §6.5 등록 게이팅(`spec.md:L413`)이
  `os.Getenv("USEARCH_X_ENABLED")=="true"` 분기 **내부**에만 `NewX` 등록을 추가 →
  env unset 시 등록 자체가 일어나지 않아 현 status quo(`query.go:498-503`, Bluesky만 등록)
  와 동일. 직접 `NewX` 생성 시에도 `searchX`가 `ErrXDisabled` 그대로 반환 → 무변경.
- **H1 mandate(no `t.Setenv` under `-race`) → [정확·강제]**: NFR-XEN-003(`spec.md:L199`)·
  REQ-XEN-003/009가 모든 env-의존 테스트를 `XOptions.EnvLookup` 주입으로 강제하고
  `t.Setenv`/`os.Setenv` 금지를 [HARD]로 명시. 부모의 `XOptions.EnvLookup`(`social.go:61-66`)
  설계와 일치. `goleak.VerifyTestMain`(`bench_test.go:14`) 재사용 주장도 실파일로 확인.

부모 §11.4의 provider 미결정(부모는 ScrapeCreators를 default로 추천)을 이 SPEC가
"검색 불가 → REJECT, `XProvider` 추상화로 대체"로 정정한 것은 근거(라이브 OpenAPI)가 있는
타당한 supersede다.

---

## External-Blocker Honesty

[정직함 — 강점]. §1 L111-116, §2.2, §7(`spec.md:L467-474`)이 production *activation*은
유료 크레딧 + ToS-ack 결정에 의존하며 **이 SPEC가 라이브를 켤 수 없음**을 명시적으로 반복
선언. "구현은 막혀 있지 않다"는 식의 위장 없음. 동시에 contract+test는 `fakeProvider`로
완전 구현·검증 가능함을 분리해 명확히 함(`spec.md:L114-116`). 결함 없음.

---

## Chain-of-Verification Pass

Second-look 재독으로 다음을 전수 확인했다:
- REQ-XEN-001..009 9개 항목을 모두 1행씩 정독(첫 몇 개만 스캔하지 않음) → 패턴·SHALL 정합.
- REQ 번호 시퀀싱을 end-to-end 확인(001→009 연속, NFR 001→004 연속) — 갭/중복 없음.
- Traceability를 표본이 아니라 전수로 확인: §3 표 REQ ↔ §5 AC ↔ §8 테스트 34행 모두 대응.
- Exclusions(§2.2, §7) 항목별 구체성 확인 — 각 항목이 목적지(SPEC-FAN/CACHE/IDX/IDX-003)
  또는 명시 사유(REJECT/EXTERNAL BLOCKER)를 가짐. vague 항목 없음.
- 요구 간 모순 점검: 1차에서 놓쳤던 **D4(quote_count 계층 비대칭)** 와 **D5(Title==Snippet)**
  를 2차에서 신규 포착해 결함 목록에 추가했다.
- 코드 인용 전수 재대조 결과 **D1(score.go max clamp 부재)** 가 가장 실질적 결함으로 확정.
  나머지 인용(`search_bluesky.go:36-122`, `client.go:62-66,91-111`, `registry.go:478-482`,
  `normalized_doc.go:63-77`, `query.go:498-503`, `truncateRunes@parse.go:223`,
  parent `spec.md:83-84,1190,1224-1230`, `AUDIT-FINDINGS:22`, `tech.md:107,147`)은
  심볼·범위 모두 일치(행 번호 ±1 cosmetic은 D2).

---

## Recommendation

Verdict: **PASS-WITH-FIXES**. F-08 진단과 방향(provider 추상화 + 외부 블로커 분리 + 2-state
disabled 보존 + H1 race-safe 테스트)은 견고하고, 인용·외부 API 검증 정확도가 이 저장소
기준 상위권이다. ready 도달을 위한 **최소 변경 목록**:

1. **D1 (MAJOR, 필수)** — `spec.md:L168-172`: `x = max(LikeCount + RepostCount, 0)` 표현을
   실제 `score.go:25`(`x := float64(likeCount + repostCount)`)에 맞춰 정정. clamp이 없음을
   반영하거나 "카운트 비음수 가정"을 명시. 인용 함수는 변경 금지.
2. **D3 (MP-3, 필수)** — `spec.md:L11`: `created` → `created_at`로 키 변경(또는 프로젝트
   `created` 컨벤션을 firewall 예외로 공식 문서화).
3. **D4 (MINOR, 권장)** — `quote_count`를 §6.4 Metadata REQUIRED로 승격하거나 REQ-XEN-008
   매핑을 OPTIONAL로 완화하여 수집/출력 계층 정합.
4. **D2 (MINOR, 선택)** — §12·research 행 번호를 `61-66,164-178,199-213`으로 미세 정정.
5. **D5 (MINOR, 선택)** — §6.4 `Title` 정책(Title==Snippet 의도/대안)을 1줄 명시.

D1·D3 보정 후 즉시 run-ready. 외부 활성화는 SPEC 설계대로 유료 크레딧+ToS-ack의 EXTERNAL
BLOCKER로 남으며, 이는 결함이 아니라 정직하게 명시된 전제다.
