# SPEC Review Report: SPEC-CLI-003

Iteration: 1/3
Verdict: PASS-WITH-FIXES
Overall Score: 0.74

프롬프트로 전달된 저자 측 추론 맥락(코드가 무엇을 말하는지에 대한 주장)은 M1 Context
Isolation 원칙에 따라 검증 대상으로만 취급했다. 모든 인용은 `spec.md` / `research.md`
와 **실제 소스 코드**를 직접 대조하여 독립 확인했다. 결론은 "예상 인용이 맞을 것"이
아니라 파일을 읽은 증거에 근거한다.

요약: 이 SPEC는 이 프로젝트의 과거 이력(존재하지 않는 경로/ID 인용)과 달리 **line-accurate
인용 정확도가 매우 높다**. 모든 `file:line` 인용을 재확인했고 행 번호·심볼 모두 일치했다.
그러나 인용된 프리미티브 위에 쌓아 올린 **의미적(semantic) 주장 두 가지가 실제 타입/시그니처와
모순**되며(아래 D1·D3), 이로 인해 핵심 수용 기준(출력 컬럼 형태, 4-상태 분류)이 인용된
프리미티브만으로는 구현 불가능하다. 그래서 BLOCK이 아닌 PASS-WITH-FIXES로 판정한다 —
F-06 진단과 방향은 견고하나, 출력 스키마·분류 메커니즘을 실제 코드에 맞춰 보정해야 한다.

---

## Must-Pass Results

- [PASS] MP-1 REQ number consistency: `spec.md:L212-L226`. 기능 요구 `REQ-CLI3-001..006`
  순차, NFR `NFR-CLI3-001..004` 순차. 갭·중복 없음, 3자리 zero-pad 일관. 확인 완료.
- [PASS] MP-2 EARS format compliance: `spec.md:L212-L217`. 6개 REQ 모두 명시적
  `SHALL` + 패턴 키워드 보유 — REQ-001 Ubiquitous, REQ-002/003 Event-Driven(`WHEN`),
  REQ-004/005 Ubiquitous, REQ-006 Unwanted(`IF ... THEN`). 정규문에 weasel word
  (should/reasonable/appropriate/adequate) 없음. REQ-001/002 가 다중 `SHALL` 복합문이라
  분해 권장이나 EARS 위반은 아님(D7 참조, minor).
- [FAIL] MP-3 YAML frontmatter validity: `spec.md:L1-L17`. 필수 키 `created_at` 대신
  `created`(L11) 사용, 필수 키 `labels` **완전 부재**. auditor 의 MP-3 firewall
  (required: id, version, status, created_at, priority, labels) 기준 hard FAIL.
  단, 이는 이 프로젝트 전반의 frontmatter 관례이며 SPEC-UI-002-review-1 도 동일 사유로
  FAIL 처리했다(일관성 있음). SPEC 의도/품질과 무관한 스키마 컨벤션 문제이므로 fix 비용은
  낮다(D8).
- [N/A] MP-4 Section 22 language neutrality: N/A — 단일 스택 Go CLI(`cmd/usearch`)
  대상. 16개 언어 툴링 열거 대상 아님. 자동 통과.

MP-3 가 must-pass FAIL 이므로 firewall 규칙상 전체는 엄밀히 FAIL 트리거다. 그러나
프로젝트 관례임을 감안해 verdict 는 내용적 결함(D1/D3)을 우선시한 PASS-WITH-FIXES 로
기록하되, MP-3 는 fix list 의 필수 항목으로 둔다.

---

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.75 | 0.75 band — 대부분 단일 해석. 단 `<category>`/`<lang>` 컬럼의 출처가 모호(존재하지 않는 필드를 "Capabilities()에서"라고 명시) → 합리적 엔지니어가 서로 다르게 매핑할 여지 | `spec.md:L92,L129,L198`; `pkg/types/capabilities.go:38-62` |
| Completeness | 0.50 | 0.50 band — §4 가 약속한 `acceptance.md` 산출물이 디렉터리에 **부재**; panic 한 Healthcheck, `--timeout<=0` 등 일부 실패 모드 미정의; frontmatter `labels` 누락 | `spec.md:L232-L234`; dir listing(acceptance.md 없음) |
| Testability | 0.75 | 0.75 band — 거의 모든 AC 가 명명된 테스트로 이진 검증 가능. `not-configured`/`disabled` 상태는 프로덕션 경로에서 도달 불가(SkipAuthCheck/fake 필요)인데 요구로만 제시 → 테스트 전제 불명확 | `spec.md:L213,L216`; `internal/adapters/registry.go:151-152,394-432` |
| Traceability | 1.00 | 1.0 band — 6 REQ + 4 NFR 모두 §4 수용 게이트와 §5.4/research §5 테스트 매핑 보유. orphan/dangling 없음 | `spec.md:L236-L286`; `research.md:L120-L131` |

---

## Defects Found

D1. `spec.md:L92, L129, L198`(및 §1.3 L50-L53) — **CRITICAL (code-citation / implementability).**
`list` 출력 형태 `<name>\t<category>\t<lang>\t<auth_required:y/n>` 의 `category` 와
`lang` 컬럼을 "live registry 의 `Capabilities()` (category, language, RequiresAuth)에서
가져온다"고 반복 명시한다(§2.1d, §2.1e, §2.4). 그러나 `pkg/types/capabilities.go:38-62`
의 `Capabilities` 구조체에는 **스칼라 `Category` 필드도 `Lang` 필드도 존재하지 않는다**.
존재하는 것은 `DocTypes []DocType`(슬라이스)와 `SupportedLangs []string`(슬라이스,
빈 값 = language-agnostic)뿐이다. 또한 어댑터 인터페이스(`pkg/types/adapter.go:28-45`)에도
`Category()` 접근자가 없다. 정적 슬라이스의 `category` 값(academic/code/social/…)은
`cmd/usearch/sources_cmd.go:17-24` 에 **손으로 적어 넣은 값**이지 레지스트리/Capabilities
파생값이 아니다. 결과적으로 (a) `category` 단일 문자열을 `DocTypes []DocType` 에서
도출하려면 SPEC 에 없는 매핑 규칙이 필요하고, (b) `lang` 을 `SupportedLangs []string`
(빈 슬라이스 포함)에서 단일 값으로 접는 규칙도 미정의다. 이는 핵심 AC(REQ-CLI3-001/004
출력 형태)를 인용된 프리미티브만으로 충족 불가하게 만들며, NFR-CLI3-004("어떤 어댑터
이름·**카테고리**·설명 병행 목록도 유지하지 않는다")와도 정면 충돌한다 — category 매핑을
다시 손으로 유지해야 하기 때문이다.
**Fix:** ① category/lang 의 정확한 출처를 명시하라. 선택지: `Capabilities.DocTypes` →
category 매핑 테이블을 SPEC 에 정의(그리고 NFR-CLI3-004 예외로 명문화), 또는
`Capabilities.DisplayName`/`Notes` 활용, 또는 `pkg/types` 에 `Category`/canonical-lang
필드 추가(이 경우 §2.2 "internal/adapters 무변경" 및 SDK 메이저 범프 영향 재평가).
② `lang` 의 `SupportedLangs` 접기 규칙(예: 첫 코드, 빈 슬라이스 → `*` 또는 `n/a`) 명시.

D2. `spec.md:L213,L300,L324`(§5.3 분류 step 1) — **MAJOR (code-citation / logic).**
분류 알고리즘 1단계가 "registry 가 disabled 로 보고하면 → `disabled`"이나, disabled
상태를 질의할 **public 프리미티브가 없다**. `r.disabled` 는
`internal/adapters/registry.go:112` 의 비공개 맵이며 getter 가 없다. disabled 신호는
오직 `Resync()`/`SnapshotForAdmin()` 의 반환 `Status` 문자열에만 접혀 노출된다. §2.2 는
"registry 무변경, 순수 consumer"를 [HARD] 로 못박았으므로 public getter 추가도 금지된다.
즉 disabled 를 알려면 반드시 `Resync`(→ Healthcheck 실행) 또는 `SnapshotForAdmin`
(텔레메트리, live 아님)을 경유해야 하는데, 이는 "Healthcheck **이전에** disabled 를
선판정한다"는 §5.3 순서와 모순된다.
**Fix:** disabled 취득 경로를 명시하라 — `SnapshotForAdmin()` 으로 disabled 집합을
선조회한 뒤 그 외 어댑터만 live probe 하거나, §2.2 의 "registry 무변경" 제약을 완화해
`IsDisabled(id) bool` 같은 read-only getter 추가를 허용(그러면 SPEC-UI-002/EVAL-002
오너십과 조율 필요).

D3. `spec.md:L213`(REQ-CLI3-002) — **MAJOR (code-citation / overstatement).**
"live probe 는 `Registry.Resync(ctx, id)` 를 재사용하여 connected/unhealthy/disabled/
not-configured 로 분류한다"고 하나, `registry.go:394-432` 의 `Resync` 는 (a) status 로
오직 `connected` 또는 `disabled` 만 설정하고, (b) Healthcheck 실패 시 view 가 아니라
`(nil, *UpstreamError)` 를 **조기 반환**한다(`:400-402`). 따라서 `Resync` 단독으로는
`unhealthy`/`not-configured` 를 결코 산출하지 못하며, 4-상태 분류는 호출부에서 Resync 의
**에러를 잡아 매핑**해야만 가능하다. 게다가 Resync 에러는 timeout/일반 실패를 구분하지
않는다. research.md §9 Q2 는 Resync 재사용을 "권장"으로 표시했으나, 실제로는 Resync 가
요구된 분류를 전달할 수 없으므로 권장 근거가 과장됐다.
**Fix:** REQ-CLI3-002 를 "어댑터 `Healthcheck(ctx)` 직접 호출(권장) + `Capabilities()`
기반 key_set + `SnapshotForAdmin` 기반 disabled" 로 메커니즘을 재서술하거나, Resync 재사용
시 "에러→unhealthy 매핑, disabled 는 별도 조회"라는 보정 단계를 명문화하라.

D4. `spec.md:L213,L325-L327`; `research.md:L141,L148-L149` — **MAJOR (testability).**
`not-configured` 는 프로덕션 경로에서 사실상 도달 불가능한 분기다.
`buildProductionRegistry`(`query.go:476-507`)는 auth-gated 어댑터를 env 존재 시에만
register 하고, `RegisterWithOptions`(`registry.go:151-152`)가 등록 시 `AuthEnvVars`
충족을 강제한다(`SkipAuthCheck` 없을 때). 따라서 등록된 RequiresAuth 어댑터는 거의 항상
키가 set 이다. SPEC 도 "보통 등록되지 않지만 방어적으로 surface"라고 인정하나, REQ-CLI3-002
는 `not-configured` 를 일급 필수 분류로 두고 `TestSourcesStatusClassifiesNotConfigured`
를 요구한다 — 이 테스트는 `SkipAuthCheck`/fake registry 로만 통과 가능하다.
**Fix:** REQ-CLI3-002 또는 §5.4 에 "`not-configured` 는 `SkipAuthCheck` 로 등록된 fake
어댑터 또는 등록 후 키 소실 시나리오에서만 재현된다"를 명시하고, 해당 테스트의 전제 환경을
적어라.

D5. `spec.md:L92, L132`(§2.1g) — **MAJOR (implementability understated).**
"CLI 의 `--format` 을 CLI-002 와 일관되게 honor 한다"가 비용을 과소평가한다. 실제로
`--format` 은 전역/persistent 플래그가 아니라 **명령별 로컬 플래그**다
(`cmd/usearch/query.go:305`, `cmd/usearch/root.go:181`). 현재 `sources` 하위 명령에는
`--format` 플래그가 전혀 없으므로 `list`/`status` 각각에 새로 등록해야 한다. 또한 공유
검증 헬퍼가 없고, 기존 에러 메시지(`query.go:132`: `usearch query: unsupported format
%q; valid: text, json, markdown`)는 REQ-CLI3-004 가 요구하는 정규 메시지
(`unsupported format '<value>'; valid: human, text, json, markdown, md`)와 다르다
— 즉 출하된 query 경로가 이미 자신의 CLI-002 REQ-CLI2-006 계약과 어긋나 있다.
**Fix:** §2.1g/REQ-CLI3-004 에 "포맷 검증 헬퍼를 중앙화/신설하고 정규 메시지로 통일"을
명시하라. "reuse"가 아니라 "create+centralize"임을 인정해야 한다.

D6. `spec.md:L132, L215`(REQ-CLI3-004 `schema_version`) — **MINOR (testability/ambiguity).**
`schema_version` 타입이 미고정. 기존 v0 query JSON(`cmd/usearch/output_json.go:23,37`)은
**문자열 `"1"`** 을 쓰는데, `cmd/usearch/repl.go:216` 은 **정수 `1`**, research.md §7
도 정수 `"schema_version":1` 로 예시한다. "v0/CLI-002 와 일관"하려면 문자열 `"1"` 로
핀해야 한다.
**Fix:** REQ-CLI3-004 에 `schema_version` 을 문자열 `"1"`(output_json.go 와 동일)로
명시.

D7. `spec.md:L212-L213`(REQ-CLI3-001/002) — **MINOR (clarity/EARS).**
REQ-001 과 REQ-002 가 다중 `SHALL` 을 한 요구에 적층한 복합문이다(REQ-001: registry
바인딩 + knownAdapters 제거 + List/Get 열거 + env-gating 가시성 4가지). EARS 위반은
아니나 검증 단위 분해가 바람직하다.
**Fix:** 선택적 — registry 바인딩(001a), `knownAdapters` 제거(001b)로 분리 고려.

D8. `spec.md:L11, L1-L17` — **MAJOR (MP-3 frontmatter).**
필수 키 `labels` 부재, `created_at` 대신 `created`. MP-3 must-pass 위반.
**Fix:** `labels: [cli, sources, health, M7]` 추가, `created` → `created_at` 키명 변경
(프로젝트 전반 컨벤션이라면 별도 결정으로 일괄 처리 가능).

D9. `spec.md`(depends_on `L15`) — **MAJOR (scope/dependency soundness).**
`depends_on: [SPEC-CLI-002, SPEC-EVAL-002]` 인데, 이 SPEC 의 **핵심 live 프리미티브
`Registry.Resync` 는 SPEC-UI-002(REQ-AS-002, `registry.go:393`) 소유**이고
SPEC-EVAL-002 의 `HealthSnapshot` 은 §1 에서 "CLI 에 무의미"하다며 명시적으로 사용을
거부한다. 즉 실제로 기능 의존하는 SPEC(UI-002)은 depends_on 에 없고, 사용하지 않는
프리미티브의 SPEC(EVAL-002)만 hard dep 로 선언돼 의존 가중이 어긋난다. §2.3 가 UI-002
출처를 각주로 언급은 하나 depends_on 에는 반영하지 않았다.
**Fix:** `depends_on` 에 `SPEC-UI-002` 추가(또는 EVAL-002 를 "parity reference(비-hard)"로
강등하고 UI-002 를 hard dep 로 승격). REQ-CLI2-009 _TBD_ 해소가 EVAL-002 엔드포인트가
아닌 UI-002 Resync 로 이뤄짐을 의존 그래프에 반영.

D10. `spec.md:L232-L234`(§4) — **MAJOR (completeness).**
§4 가 "상세 Given/When/Then 시나리오는 `.moai/specs/SPEC-CLI-003/acceptance.md` 에
있다"고 명시하나, 해당 디렉터리에는 `spec.md` 와 `research.md` 만 존재하고
**`acceptance.md` 가 없다**(dir listing 확인). AC 계약 산출물이 dangling 참조다.
**Fix:** `acceptance.md` 를 작성하거나, 미작성 상태라면 §4 의 "lives in acceptance.md"를
"to be authored in annotation cycle"로 수정.

D11. `spec.md:L213-L214, L223`(REQ-CLI3-002/003, NFR-CLI3-001) — **MINOR (assumption gap).**
NFR-CLI3-001 의 벽시계 상한(`--timeout + 500ms`)은 각 `Healthcheck` 가 ctx 취소를
존중한다는 전제에 의존한다. 그러나 어댑터 인터페이스 godoc(`pkg/types/adapter.go:38-40`)은
`Healthcheck` 에 대해 "Cheap"만 명시하고 ctx 준수를 **계약으로 보장하지 않는다**(Search
는 `:34-35` 에서 ctx 준수 MUST). ctx 를 무시하는 블로킹 Healthcheck 가 있으면 상한이
깨진다.
**Fix:** NFR-CLI3-001 에 "Healthcheck 가 ctx 취소를 존중한다는 전제"를 명문화하고,
fake 어댑터 테스트는 ctx 를 존중하도록 작성(실 어댑터 회귀는 별도 추적).

D12. `spec.md:L213`(REQ-CLI3-002 "errgroup") — **MINOR (design-hint risk).**
REQ-CLI3-002 가 `errgroup` 사용을 이름으로 못박는다. `errgroup.WithContext` 는 첫 에러에
형제 goroutine 을 취소하는데, REQ-CLI3-003 은 "느린 어댑터 하나가 전체를 실패시키지
않는다"를 요구한다. 구현자가 `g.Go` 에서 Healthcheck 에러를 반환하면 errgroup 이 나머지를
취소해 REQ-CLI3-003 을 위반한다. 올바른 패턴은 에러를 반환하지 않고 per-adapter 결과를
슬라이스에 수집(WithContext 미사용 또는 WaitGroup)하는 것.
**Fix:** REQ-CLI3-002 에 "probe goroutine 은 에러를 반환하지 않고 결과를 per-adapter 로
수집한다(첫 실패가 다른 probe 를 취소하지 않도록)"를 명시.

D13. `spec.md`(§3 전반) — **MINOR (completeness, failure mode).**
패닉하는 `Healthcheck` 에 대한 처리가 미정의. errgroup goroutine 내 panic 은 CLI 를
크래시시킨다. 어댑터 계약은 panic 을 금지하지 않는다. 또 `--timeout` 이 0/음수일 때의
의미(무한? 즉시?)가 REQ-CLI3-005 의 "invalid --timeout"과 경계가 불명확.
**Fix:** probe goroutine 의 `recover` 정책과 `--timeout<=0` 처리(거부 또는 default)를
명시.

---

## Chain-of-Verification Pass

2차 자기비판 패스 — 1차에서 놓친 결함을 찾기 위해 각 인용을 소스와 재대조했다.

검증한 인용(전수, 샘플링 아님) — 모두 line-accurate:
- `cmd/usearch/sources_cmd.go:16-25` knownAdapters(8개) ✓; `:58` `NAME\tCATEGORY\tDESCRIPTION` ✓; `:71-86` status 스텁 + "Source health check not yet implemented." + `unknown` ✓; `:88-114` show ✓.
- `cmd/usearch/query.go:458` buildProductionRegistry ✓(unexported, package main — §2.1f 직접 호출 주장 정확, 추출 불필요 = 숨은 리팩터 비용 **없음**); `:499-503` `social.NewBluesky` 등록 ✓; github `:476-487`/youtube `:488`/naver `:504-507` env-gating ✓.
- `internal/adapters/registry.go:340-353` classifyHealth(무호출→healthy) ✓; `:363` HealthSnapshot ✓; `:394` Resync ✓; `:234-236` SecretValue 항상 빈 문자열 ✓; `:175/185` Get/List ✓; `:306-329` AdapterHealth ✓; `:204-237` AdapterAdminView ✓; `:407-418` keyset 계산 ✓; `:327` CircuitState "closed" V1 ✓.
- errgroup 의존성: `go.mod:116` `golang.org/x/sync v0.20.0`, `internal/deepagent/tree.go:11` 사용 ✓.
- REQ-CLI2-009 인용문(`<name>\t<category>\t<lang>\t<auth_required:y/n>`): SPEC-CLI-002 `spec.md:203` 과 정확히 일치 ✓.
- `USAGE.md:~261` "현재 상태" 노트(status→모두 unknown, list→bluesky 누락): 확인 ✓.

2차에서 새로 표면화한 결함: D11(Healthcheck ctx 준수 비계약), D12(errgroup WithContext 취소
함정), D13(panic/음수 timeout 미정의). 결함 목록에 추가했다.

핵심 재확인: 인용 **행 번호**는 전부 맞다. 결함은 "인용이 가리키는 코드가 SPEC 의 의미적
주장을 뒷받침하지 못한다"는 D1(Capabilities 에 category/lang 없음)·D2(disabled getter
없음)·D3(Resync 가 4-상태 산출 불가)에 집중된다. 이 셋은 spot-check 가 아니라 타입
정의와 함수 본문을 직접 읽어 확정했다.

모순/일관성 점검: §2.4 의 출력-형태 화해는 정직하고 CLI-002 와 모순되지 않는다(REQ-CLI2-009
는 status 를 "deferred"로만 두었지 메커니즘을 EVAL-002 로 고정하지 않았으므로 UI-002 Resync
피벗은 허용된다). 단 그 화해가 요구하는 category/lang 이 D1 로 인해 내부적으로 무너진다.

---

## Regression Check (Iteration 2+ only)

해당 없음 — iteration 1.

---

## Recommendation

진단(F-06)과 설계 방향(single source of truth, HealthSnapshot 부적합 논증, live Resync
피벗)은 견고하며 인용 정확도는 모범적이다. 그러나 **인용된 프리미티브가 핵심 출력/분류
요구를 떠받치지 못하는** 의미적 결함이 있어 ready 가 아니다. ready 도달을 위한 최소 변경:

1. **(D1, CRITICAL)** `<category>`·`<lang>` 컬럼의 실제 출처를 확정하라. `Capabilities`
   에는 스칼라 `Category`/`Lang` 이 없다(`pkg/types/capabilities.go:38-62`). `DocTypes`→
   category 매핑 규칙과 `SupportedLangs`→단일 lang 접기 규칙을 SPEC 에 정의하고, 이 매핑이
   NFR-CLI3-004("병행 카테고리 목록 금지")의 명시적 예외임을 적거나, `pkg/types` 필드 추가
   경로(§2.2 무변경 제약·SDK 범프 영향 재평가)를 택하라.
2. **(D2, MAJOR)** disabled 상태 취득 경로를 명시하라(`SnapshotForAdmin()` 선조회 권장).
   public getter 가 없으며 §2.2 가 registry 변경을 금지함을 반영.
3. **(D3, MAJOR)** REQ-CLI3-002 의 분류 메커니즘을 재서술하라 — `Resync` 는 connected/
   disabled 만 내고 실패 시 에러 조기 반환(`registry.go:400-402`)이므로, 어댑터
   `Healthcheck` 직접 호출 + 에러→unhealthy 매핑 + 별도 disabled 조회로 명문화(또는 Resync
   재사용 시 보정 단계 추가).
4. **(D5, MAJOR)** `--format` 을 "reuse"가 아닌 "각 sources 하위 명령에 신설 + 검증 헬퍼
   중앙화 + 정규 메시지 통일"로 §2.1g/REQ-CLI3-004 보정(`query.go:305`, `root.go:181`,
   `query.go:132` 현 상태 반영).
5. **(D9, MAJOR)** `depends_on` 에 `SPEC-UI-002` 추가(실제 사용 프리미티브 Resync 의 오너).
6. **(D10, MAJOR)** `acceptance.md` 작성 또는 §4 의 "lives in acceptance.md" 참조 수정.
7. **(D8, MAJOR/MP-3)** frontmatter 에 `labels` 추가, `created`→`created_at`.
8. **(D4·D6·D11·D12·D13, MINOR)** `not-configured` 도달 전제 명시, `schema_version` 타입
   문자열 `"1"` 핀, Healthcheck ctx-준수 전제 명문화, errgroup 비-취소 수집 패턴 명시,
   panic/`--timeout<=0` 처리 정의.

verdict: **PASS-WITH-FIXES** — BLOCK 이 아닌 이유는 인용 정확도와 진단이 견고하고 builder
재사용(D 없음)이 검증되어 구조적으로 구현 가능하기 때문이며, PASS 가 아닌 이유는 D1/D2/D3
가 핵심 AC 를 인용 프리미티브만으로 충족 불가하게 만들고 MP-3·acceptance.md 가 미충족이기
때문이다. 위 1-7 을 반영하면 ready.
