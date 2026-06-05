# SPEC-CLI-003 — Research

Companion artifact to `spec.md`. Deep codebase analysis grounding the
F-06 fix for `usearch sources`. All paths are project-root-relative.
Narrative in Korean per project `conversation_language`; code,
identifiers, and tables in English per house style.

> r1 (2026-06-04): plan-audit `SPEC-CLI-003-review-1.md` 반영. 의미적
> 주장 보정 — `Capabilities` 스칼라 category/lang 부재(D1), `disabled`
> 는 `SnapshotForAdmin` 경유(D2), `Resync` 미사용·`Healthcheck` 직접
> 호출(D3), `--format` 신설(D5), `schema_version` 문자열 `"1"`(D6),
> errgroup 비-취소 패턴(D12), panic/`--timeout<=0`(D13). 각 항목 소스
> 재확인 후 기록.

---

## 1. 배경: F-06 finding

GSD 코드베이스 감사에서 `usearch sources` 표면이 실제 레지스트리와
어긋난다는 finding **F-06** 이 보고되었다. 사용자 대면 증상은
`USAGE.md:~261` 의 "현재 상태" 노트에 그대로 기록되어 있다(`sources
status` 가 모든 소스를 `unknown` 으로 표시, 정적 `list` 가 등록된
`bluesky` 어댑터를 누락). F-06 은 감사가 이 갭에 부여한 finding ID 이며
`.planning/codebase/CONCERNS.md` 는 감사의 우려사항 레지스터다. 세 가지
구체적 결함이 있으며 모두 출하된 코드로 입증된다.

### 1.1 `sources status` 는 스텁이다

`cmd/usearch/sources_cmd.go:71-86` 의 `newSourcesStatusCmd` 는 항상
`"Source health check not yet implemented."` 를 출력하고 모든 어댑터를
리터럴 문자열 `unknown` 으로 나열한다. 어떤 헬스체크도 실행되지 않는다.

### 1.2 `list` / `show` 가 정적 슬라이스를 쓴다

`cmd/usearch/sources_cmd.go:16-25` 의 패키지 레벨 `knownAdapters` 슬라이스
(arxiv, github, hn, koreanews, naver, reddit, searxng, youtube)는 실제
레지스트리가 아니다. 결과적으로:

- `bluesky` 는 프로덕션에 등록되지만(`cmd/usearch/query.go:499-503`) 정적
  목록에는 **빠져 있다**.
- `github` / `youtube` / `naver` 는 env 게이팅으로 OFF 되어 등록되지 않은
  경우에도 목록에 **나타난다** (게이팅 조건은
  `cmd/usearch/query.go:458-514` `buildProductionRegistry` 참조:
  github=`USEARCH_GITHUB_TOKEN`/`GITHUB_TOKEN`, youtube=`YOUTUBE_BASE_URL`,
  naver=`NAVER_CLIENT_ID`+`NAVER_CLIENT_SECRET`).

### 1.3 `list` 출력 모양이 SPEC-CLI-002 REQ-CLI2-009 와 다르다

REQ-CLI2-009 는 `<name>\t<category>\t<lang>\t<auth_required:y/n>` 를
규정했으나, 출하된 구현(`sources_cmd.go:58`)은
`NAME\tCATEGORY\tDESCRIPTION` 를 낸다. 스펙-구현 불일치이며, SPEC-CLI-003
가 §2.4 에서 REQ-CLI2-009 모양으로 화해(reconcile)한다.

---

## 2. 핵심 설계 제약: `HealthSnapshot` 만으로는 부족하다

`internal/adapters/registry.go:363` 의 `Registry.HealthSnapshot()` 은
**프로세스 내 호출 텔레메트리**(success/fail 카운터)에서 상태를 도출한다.

```
classifyHealth (registry.go:340-353):
  success+fail == 0  → "healthy"   // 무호출 = 건강 (서버용 규칙)
  rate >= 0.95       → "healthy"
  rate >= 0.85       → "degraded"
  else               → "unhealthy"
```

단명(short-lived)하는 CLI 프로세스는 어댑터를 한 번도 호출하지 않으므로
모든 카운터가 비어 있고, 결과적으로 모든 어댑터가 `healthy` 로 보고된다.
즉 네트워크를 전혀 건드리지 않고 "전부 정상"이라고 거짓말한다. 장기 실행
서버에는 맞지만 CLI 일회성 호출에는 **무의미**하다.

→ 따라서 `sources status` 는 각 어댑터의 `Adapter.Healthcheck(ctx)`
(`pkg/types/adapter.go:38-40`)를 **직접** 호출한다.

**`Registry.Resync` 를 쓰지 않는 이유 (plan-audit D3).** r1 이전 초안은
`Registry.Resync(ctx,id)` (`registry.go:394`) 재사용을 "권장"했으나, 실제
시그니처를 다시 읽어보니 부적합하다: (1) 반환 `AdapterAdminView.Status` 는
오직 `connected`/`disabled` 만 설정하고(`registry.go:420-423`)
`unhealthy`/`not-configured` 를 결코 내지 못한다; (2) Healthcheck 실패 시
view 가 아니라 `(nil, *UpstreamError)` 를 **조기 반환**한다
(`registry.go:400-402`). 따라서 4-상태 분류를 `Resync` 단독으로는 만들 수
없다. CLI 가 `Healthcheck` 를 직접 부르고 결과를 매핑한다(nil→connected,
error/deadline/panic→unhealthy). `disabled` 는 별도로
`SnapshotForAdmin()` 으로 선조회한다(§3, D2).

---

## 3. 재사용할 기존 인프라 (재발명 금지)

| Symbol | File:Line | 용도 (USED / NOT used) |
|--------|-----------|------|
| `Registry.SnapshotForAdmin()` | `registry.go:251` | **USED.** `disabled` 집합의 유일 공개 read 경로(`Status=="disabled"`, `:279-282`) + key-set(`:266-277`). 텔레메트리 기반·network-free. probe 전 1회 호출. |
| `Adapter.Healthcheck(ctx) error` | `pkg/types/adapter.go:38-40` | **USED.** 실제 liveness 프로브, **직접** 호출(Resync 경유 아님). godoc 은 "Cheap"만 명시, ctx 준수 비계약. |
| `Adapter.Capabilities()` → `DocTypes`, `SupportedLangs`, `RequiresAuth`, `AuthEnvVars`, `DisplayName`, `RateLimitPerMin` | `pkg/types/capabilities.go:38-62` | **USED.** 컬럼 파생(§3.3) + key-set. **스칼라 `Category`/`Lang` 없음**(D1). |
| `Registry.List()` / `Registry.Get(id)` | `registry.go:185 / 175` | **USED.** 등록 어댑터 열거 / 조회. |
| `buildProductionRegistry()` | `cmd/usearch/query.go:458` | **USED.** `sources` 가 `query` 와 동일 레지스트리를 직접 호출(둘 다 package main, 추출 불필요). |
| `Registry.HealthSnapshot()` | `registry.go:363` | NOT used. 텔레메트리, CLI 에선 빈 값. 패리티 맥락만. |
| `Registry.Resync(ctx, id)` | `registry.go:394` | NOT used. status 가 connected/disabled 만, 실패 시 에러 조기 반환(D3). |

### 3.1 D2 — `disabled` 취득 경로 (registry 무변경)

`r.disabled` 는 비공개 맵이며 public getter 가 없고 §2.2 가 getter 추가를
금지한다. 유일 공개 경로는 `Registry.SnapshotForAdmin()`(`registry.go:251`)이
반환하는 `[]AdapterAdminView` 의 `Status` 필드(`disabled` 어댑터는
`"disabled"`, `:279-282`)다. probe 전 1회 호출해 disabled 집합을 만들고,
분류 시 참조한다. disabled 판정이 Healthcheck 결과에 의존하지 않으므로
D2/D3 의 순서 모순이 해소된다. (테스트는 기존 `ToggleEnabled` 로 disabled
를 만든다.)

### 3.2 D1 — category/lang 파생 (스칼라 필드 없음)

`Capabilities` 에는 `DocTypes []DocType`(`:43-44`)와 `SupportedLangs
[]string`(`:45-47`)뿐, 스칼라 `Category`/`Lang` 이 없다. 어댑터 인터페이스
(`adapter.go:28-45`)에도 `Category()` 접근자가 없다. 정적 슬라이스의
category 값(academic/code/…)은 손으로 적은 값이었다. 따라서 컬럼을 파생해야
한다(spec.md §2.5 정규 규칙):
- category: `DocTypes` → 우선순위 매핑(paper→academic, repo/issue→code,
  video→video, post/social→social, article→news, 그 외/빈→other). 이
  매핑 테이블이 NFR-CLI3-004 의 유일 허용 예외(레지스트리 파생 순수함수라
  drift 불가).
- lang: `SupportedLangs` 접기 — 빈 슬라이스→`*`, 단일→그 코드, 복수→`첫+`.

### 3.3 보안 불변식

`AdapterAdminView.SecretValue` 는 항상 빈 문자열이다(`registry.go:234-236`).
`sources` 출력도 env-var **이름**과 boolean key-set 만 노출, 값은 절대 금지
(REQ-CLI3-006).

### 3.4 동시성 프리미티브 (D12)

`golang.org/x/sync/errgroup` 는 이미 의존성이다
(`internal/deepagent/tree.go:11`). 단 `errgroup.WithContext` 는 첫 에러에
형제를 취소하므로 REQ-CLI3-003("느린 하나가 전체를 죽이지 않음")을 위반한다.
따라서 probe 는 에러를 그룹에 반환하지 않고 index-disjoint 슬롯에 per-adapter
결과를 수집한다(WithContext 미사용 plain `errgroup.Group` 또는
`sync.WaitGroup`). 각 probe 는 자기 `context.WithTimeout` + `defer cancel()`
+ `recover()`(panic→unhealthy, D13)를 둔다.

---

## 4. 확정된 설계 결정 (사용자 사전 승인 — _TBD_ 아님)

1. **범위 = status + list 화해.** (a) 실제 `sources status` 헬스 구현,
   (b) `list`/`show` 를 실제 레지스트리 기반으로 전환. `show <name>` 도 동일
   레지스트리 소스로 일관 유지.
2. **헬스 실행 모델 = 라이브 Healthcheck 직접 호출 + 타임아웃.** 프로덕션
   레지스트리 빌드 → disabled 집합 선조회(`SnapshotForAdmin`) → 비-disabled
   어댑터별 `Adapter.Healthcheck(ctx)` 를 §3.4 비-취소 fan-out 으로 동시 실행
   (`errgroup.WithContext` 금지) → per-adapter 타임아웃(기본 `3s`,
   `--timeout` override; `<=0` 거부). 분류: `connected` /
   `unhealthy`(error/timeout/panic) / `disabled` / `not-configured`. 추가로
   `Capabilities().AuthEnvVars` 기반 `key_set` 노출. `--format` 은 신설
   플래그(reuse 아님), `schema_version` 는 문자열 `"1"`.
3. **독립 SPEC-CLI-003** (CLI-002 개정 아님).

---

## 5. EARS / NFR 커버리지 매핑

| 요구 | 결함/결정 | 검증 테스트(제안) |
|------|-----------|-------------------|
| REQ-CLI3-001 | §1.2 정적 슬라이스 → 레지스트리 단일 소스 | `TestSourcesListReflectsRegistry`, `TestSourcesListOmitsUnregisteredGatedAdapter`, `TestSourcesListIncludesBluesky`, `TestSourcesNoStaticKnownAdapters` |
| REQ-CLI3-002 | §1.1 스텁 → 라이브 동시 헬스 + 분류(직접 Healthcheck, disabled=SnapshotForAdmin) | `TestSourcesStatusLiveHealthcheck`, `TestSourcesStatusConcurrentProbe`, `TestSourcesStatusClassifiesDisabled`, `TestSourcesStatusClassifiesNotConfigured`(SkipAuthCheck fake), `TestSourcesStatusReportsKeySet`, `TestSourcesStatusPanicClassifiedUnhealthy` |
| REQ-CLI3-003 | §3.4 per-adapter timeout, one-slow 격리(비-취소) | `TestSourcesStatusTimeoutClassifiesUnhealthy`, `TestSourcesStatusOneSlowDoesNotBlockOthers`, `TestSourcesStatusTotalLatencyBounded` |
| REQ-CLI3-004 | §3.2 `--format` 신설 + 파생 컬럼 + schema_version 문자열 | `TestSourcesStatusJSONSchema`, `TestSourcesStatusMarkdownTable`, `TestSourcesStatusHumanColumns`, `TestSourcesListJSONFormat`, `TestSourcesListMarkdownFormat`, `TestSourcesListCategoryLangDerivation`, `TestSourcesFormatInvalidRejected`, `TestSourcesFormatHelperShared` |
| REQ-CLI3-005 | 결정: status=report not gate | `TestSourcesStatusExitsZeroWithUnhealthy`, `TestSourcesStatusExitsZeroWithNotConfigured`, `TestSourcesStatusBadTimeoutExitsUserError`, `TestSourcesShowUnregisteredExitsUserError` |
| REQ-CLI3-006 | edge: empty registry, no secret leak | `TestSourcesListEmptyRegistry`, `TestSourcesStatusEmptyRegistry`, `TestSourcesNoSecretValueLeak` |
| NFR-CLI3-001 | bounded latency (errgroup) | `TestSourcesStatusTotalLatencyBounded` |
| NFR-CLI3-002 | list network-free | `TestSourcesListIssuesNoProbes` |
| NFR-CLI3-003 | goroutine hygiene | `goleak.VerifyTestMain` |
| NFR-CLI3-004 | no registry drift | `TestSourcesListMatchesBuildProductionRegistry` |

---

## 6. 분류 알고리즘 (정규 — spec.md §5.3)

probe 전 1회: `disabledSet = {id : Status=="disabled" in SnapshotForAdmin()}`.

`Registry.List()` 의 각 어댑터 id:

```
id ∈ disabledSet                             → "disabled"      (probe 안 함)
RequiresAuth && any(AuthEnvVars) unset       → "not-configured" (probe 안 함)
else Healthcheck(ctx, --timeout) [recover]:
    nil                                      → "connected"
    error || deadline || panic               → "unhealthy"
key_set = !(RequiresAuth && any(AuthEnvVars) unset)   // 모든 어댑터
```

disabled·not-configured 둘 다 pre-probe 라서 어떤 분류도 Healthcheck 결과/
에러에 의존하지 않는다(D3 요구 충족).

**D4 — `not-configured` 도달성.** `buildProductionRegistry` 경로에서는
도달 불가다: `RegisterWithOptions`(`registry.go:151-152`)가 등록 시
`AuthEnvVars` 충족을 강제하므로 키 없는 auth 어댑터는 애초에 등록 안 된다.
이 분기는 `RegisterOptions{SkipAuthCheck:true}` + 키 미설정 fake, 또는
등록 후 키 소실 시나리오에서만 재현된다. 테스트 전제로 명시.

---

## 7. 출력 모양 (제안)

`sources list` (REQ-CLI2-009 화해 모양):
```
NAME       CATEGORY  LANG  AUTH
arxiv      academic  en    n
reddit     social    en    y
...
```

`sources status`:
```
NAME       STATUS         KEYS
arxiv      connected      n
github     not-configured n
reddit     unhealthy      y
...
```

JSON 은 `{"schema_version":"1","sources":[{"name","status","key_set","error?"}]}`
형태로 안정 스키마 유지(D6: `schema_version` 는 **문자열 `"1"`**, `output_json.go:23,37`
와 동일 — `repl.go:216` 의 정수 `1` 아님). markdown 은 동일 컬럼의 테이블.
category 는 `*` 가 아니라 §3.2 매핑값, lang 은 §3.2 접기값.

---

## 8. 리스크 (spec.md §8 요약)

- R1 라이브 Healthcheck 행(hang) → per-adapter timeout + errgroup + 벽시계
  상한 CI 검증.
- R2 테스트/CI 가 실제 네트워크 호출 → fake 어댑터(scripted Healthcheck);
  list/show 는 network-free.
- R3 `knownAdapters` 제거가 기존 CLI-002 sources 테스트 깨뜨림 → 테스트를
  레지스트리 기반 + REQ-CLI2-009 컬럼으로 갱신(§2.4 화해).
- R4 secret 누출 → audit 테스트, 값 금지.
- R5 env-게이팅으로 출력이 환경 의존 → no-drift 테스트는 하드코딩 기대값이
  아니라 `buildProductionRegistry().List()` 에 핀.
- R6 goroutine 누수 → goleak + await + deferred cancel.
- R7 (D11) Healthcheck 가 ctx 무시 → 어댑터 계약 비보장(`adapter.go:38-40`);
  WithTimeout 은 WAIT 만 bound, ctx 무시 probe 는 goroutine 누수 가능;
  테스트는 ctx-honoring fake; 실어댑터 준수는 별도 추적.
- R8 (D13) panic 하는 Healthcheck → per-probe `recover()`, unhealthy 분류.
- R9 (D12) `errgroup.WithContext` 가 형제 취소 → WithContext 금지, 비-에러
  수집 패턴.

---

## 9. Open questions (annotation cycle 에서 해소)

1. `--timeout` 기본값 `3s` 적정? (어댑터 헬스체크 SLA 기준 재확인.) `<=0` 은
   거부(infinite 아님, D13/REQ-CLI3-005).
2. [RESOLVED — D3] 라이브 프로브는 `Adapter.Healthcheck` **직접 호출**로
   확정. `Registry.Resync` 는 4-상태를 못 내고 실패 시 에러 조기 반환이라
   부적합. disabled 는 `SnapshotForAdmin` 으로 별도 조회.
3. `list` 의 `DESCRIPTION` 컬럼을 `show` 로 이전하는 화해를 annotation 에서
   정식 승인.
4. [RESOLVED] `buildProductionRegistry` 는 둘 다 package main 이라 직접 호출,
   추출 불필요(auditor 확인).
5. [NEW — D1] §3.2 의 `DocTypes`→category 매핑 테이블 + `SupportedLangs`
   접기 규칙을 annotation 에서 확정(특히 복수 DocType 우선순위, 복수 lang
   표기 `첫+`).

---

*End of SPEC-CLI-003 research.md (draft).*
