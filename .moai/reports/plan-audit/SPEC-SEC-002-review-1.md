# SPEC Review Report: SPEC-SEC-002

Iteration: 1/3
Verdict: PASS-WITH-FIXES
Overall Score: 0.62

프롬프트로 전달된 저자 측 추론 맥락(코드가 무엇을 말하는지에 대한 주장)은 **M1 Context
Isolation** 원칙에 따라 검증 대상으로만 취급했다. 모든 인용은 `spec.md` / `research.md`
와 **실제 소스 코드**를 직접 대조하여 독립 확인했다. 결론은 "예상 인용이 맞을 것"이
아니라 파일을 직접 읽은 증거에 근거한다.

요약: 이 SPEC는 이 저장소의 과거 이력(존재하지 않는 경로/ID/타입 인용)과 달리 **인용
정확도가 매우 높다**. naver / github / koreanews / secretstore / registry / security.yaml
의 모든 `file:line`을 재확인했고 행 번호·심볼·시그니처가 일치했다. F-07의 두 가지 과장
교정(키체인 백엔드 없음 / koreanews는 자격증명 없음)도 **둘 다 소스에서 실제로 참(true)**임을
확인했다. 그러나 **설계 건전성에 치명적 공백(D1)** 이 있다 — SEC-002가 자격증명 해석을
Resolver로 일원화한다고 선언하지만, 등록 게이트(`registry.go:151-153`)가 여전히
`os.LookupEnv`를 직접 읽으므로 **env가 아닌 백엔드(k8s/vault)에서는 어댑터가 Resolver로
키를 얻어도 등록 단계에서 거부**된다. 이는 F-07의 핵심 동기(k8s 백엔드를 어댑터에서 실제로
도달 가능하게 만드는 것)를 달성하지 못하게 만든다. 추가로 §6과 §2.2/OQ-4 사이에 llm/config
분류 모순(D2)이 있다. 따라서 BLOCK 직전의 PASS-WITH-FIXES로 판정한다.

---

## Must-Pass Results

- [PASS] MP-1 REQ number consistency: `spec.md:L241-L246`(REQ-SEC2-001..006 순차) +
  `spec.md:L252-L255`(NFR-SEC2-001..004 순차). 갭·중복 없음, 3자리 zero-pad 일관. 확인 완료.
- [PASS] MP-2 EARS format compliance: `spec.md:L241-L246`. 6개 REQ 모두 `SHALL` + 인식
  가능한 EARS 구조 보유 — REQ-001/005 Ubiquitous, REQ-002/003 Event-Driven(`WHEN`),
  REQ-004 Unwanted(`IF ... THEN`), REQ-006 부정형 Ubiquitous(`SHALL NEVER`). 정규문에
  weasel word(should/reasonable/appropriate) 없음. 모든 REQ가 다섯 패턴 중 하나에 매칭되어
  firewall 통과. (단 REQ-006의 라벨/HISTORY 패턴 집계는 부정확 — D3 참조, minor.)
- [FAIL] MP-3 YAML frontmatter validity: `spec.md:L1-L18`. 필수 키 `created_at` 대신
  `created`(L11) 사용. firewall 기준(required: id, version, status, created_at, priority,
  labels) hard FAIL. 단 `labels`(L15)는 존재하므로 결함은 `created_at` 단일 키. 이는
  이 프로젝트 전반의 frontmatter 관례이며 SPEC-CLI-003 / SPEC-UI-002 리뷰도 동일 사유로
  FAIL 처리했다(일관성 있음). fix 비용 낮음(D4).
- [N/A] MP-4 Section 22 language neutrality: N/A — 단일 스택 Go(`cmd/usearch` +
  `internal/...`) 대상. 16개 언어 툴링 열거 대상 아님. 자동 통과.

MP-3가 firewall 상 FAIL 트리거이나, 프로젝트 관례임을 감안해 verdict는 설계 결함(D1/D2)을
우선시한 PASS-WITH-FIXES로 기록하되 MP-3는 fix list 필수 항목으로 둔다.

---

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.75 | 0.75 band — 대부분 단일 해석. 단 §6(`spec.md:L418`)이 llm/config를 "existing caller now using Resolver"로 세는 반면 §2.2(`L199-L203`)·OQ-4(`L515-L521`)는 "future caller"로 배제 → fan_in 의미가 모순 | `spec.md:L418, L199-L203, L515-L521`; `internal/llm/config/config.go:45,58` |
| Completeness | 0.50 | 0.50 band — 등록 게이트(`registry.go:151-153`)가 Resolver를 우회하는 점을 §2.2가 "preserved"로 명시했으나 그 결과 k8s/vault 경로가 깨진다는 점은 미해결. 핵심 목표(백엔드 선택이 어댑터에 실효)가 env 외 백엔드에서 충족되지 않음 | `spec.md:L204-L209`; `internal/adapters/registry.go:151-153,266-274` |
| Testability | 0.75 | 0.75 band — 모든 REQ/NFR이 §3 표의 명명된 테스트 + §4 게이트로 이진 검증 가능. 단 §3·§4의 테스트는 모두 env/fake-Resolver 전제라 k8s 백엔드의 등록 성공을 검증하는 테스트가 없음(D1과 연동) | `spec.md:L241-L246, L266-L320` |
| Traceability | 1.00 | 1.0 band — REQ-SEC2-001..006 + NFR-SEC2-001..004 전부 §4 게이트 + §3 명명 테스트 매핑. orphan/dangling 없음 | `spec.md:L239-L255, L266-L320` |

---

## Code-Citation Verification (STRICT — 저장소의 stale-citation 이력 대응)

직접 Read/grep로 대조한 결과 (각 항목 = SPEC 주장 → 실측):

- `internal/security/secretstore/resolver.go` — `Resolver` 인터페이스 + `@MX:ANCHOR`
  (L17-L22), no-leak godoc(L13-L15), `ErrNotImplemented`(L11). **일치.**
- `factory.go:22-36` — `NewResolver(backend, mountPath)`, `BackendEnv/K8s/Vault`(L6-L10),
  vault→`NewVaultResolver`, unknown→config error(L33-L34), `DefaultK8sMountPath =
  "/var/run/secrets"`(L13). **일치.**
- `vault.go:14-17` — `Get` always `ErrNotImplemented`. **일치.**
- `env.go:16,21-27` — `NewEnvResolver`, `os.LookupEnv` 시맨틱(unset→empty+error). **일치.**
- `k8s.go:21-49` — `NewK8sResolver`, 마운트 파일 per-key. **일치.**
- `internal/adapters/naver/naver.go:23` — `var secretEnv secretstore.Resolver =
  secretstore.NewEnvResolver()`. **일치.** `naver.go:118-133` — `New`이 `secretEnv.Get`로
  `NAVER_CLIENT_ID/SECRET` 해석, `Options.ClientID/Secret` 오버라이드. **일치.**
  `naver.go:196-197` — `RequiresAuth:true`, `AuthEnvVars:[NAVER_CLIENT_ID,
  NAVER_CLIENT_SECRET]`. **일치.**
- `internal/adapters/github/github.go:46-48` — `Options.Token` 주입점. **일치.**
  `github.go:146-147` — `RequiresAuth:true`, `AuthEnvVars:[USEARCH_GITHUB_TOKEN]`. **일치.**
- `cmd/usearch/query.go:476-487` — raw `os.Getenv("USEARCH_GITHUB_TOKEN")` →
  `GITHUB_TOKEN` fallback → `github.New(Options{Token})`. **일치.** `query.go:505` —
  `naver.New(naver.Options{})` (전역 resolver 의존). **일치.** `buildProductionRegistry()`는
  인자 없음(L458). `cmd/usearch/`에 `secretstore`/`NewResolver` grep = **0건(확인)**.
- `internal/adapters/koreanews/koreanews.go:91` — `AuthEnvVars: nil`,
  `RequiresAuth:false`. **일치.** `options.go:124-157` — `OptionsFromEnv`가
  `USEARCH_ADP009_*` 플래그/URL만 읽음(RSS_ENABLED/FEEDS/DAUM_ENABLED/KNC_ENABLED/
  KNC_BASE_URL). **일치.**
- `internal/adapters/registry.go:151-152` — `AuthEnvVars` 등록 강제(`os.LookupEnv`).
  **일치.** `registry.go:234-236` — `SecretValue` ALWAYS empty. **일치.**
- `.moai/config/sections/security.yaml:17-25` — `secrets.backend: env`(env|k8s|vault),
  `k8s_mount_path: /var/run/secrets`. **일치.** (주석 L14가 패키지를
  `internal/security/{secrets,...}`로 표기 — 실제 어댑터 경로는 `secretstore`; SPEC가
  L568에서 이 불일치를 정확히 지적함.)
- `.planning/AUDIT-FINDINGS.md:30` — F-07 문구가 research.md 인용과 **verbatim 일치**;
  `:15` "F-07만 SPEC 미작성" 확인.

**발명된 타입/필드/메서드 없음.** 인용 정확도는 이 저장소 표준 대비 최상위.

### Finding-correction 검증 (audit dimension 2)

- 교정(a) "키체인 백엔드 없음": `factory.go:6-10`에 `BackendEnv/K8s/Vault`만 정의,
  `keychain.go` 부재(`git ls-files internal/security/secretstore/` = doc/env/factory/
  k8s/resolver/resolver_test/vault). **참 — 교정 유효.**
- 교정(b) "koreanews는 자격증명 없음": `koreanews.go:91` `AuthEnvVars: nil` +
  `options.go:124-157` 플래그 전용. **참 — 교정 유효.**

두 교정 모두 코드와 일치하므로 major defect 아님(오히려 SPEC의 강점).

### OQ-1 (`secrets/` vs `secretstore/` 중복) 검증 (audit dimension 3)

- `internal/security/secrets/resolver.go`는 deny rule로 Read 불가 확인(권한 거부 재현됨) —
  SPEC의 "read-restricted, 내용 미확인" 주장은 정직하고 정확.
- grep로 importer 조사: `secrets`(secretstore 아님) 패키지를 import하는 곳은
  **`internal/security/secrets/resolver_test.go` (자기 자신의 테스트) 단 한 곳**. 프로덕션
  importer 0건. `secretstore`의 실제 importer는 `internal/llm/config/config.go`와
  `internal/adapters/naver/naver.go`.
- **평가:** `secrets/`를 OQ로 남기는 것은 **수용 가능**. naver/llm/config 모두
  `secretstore`를 import하고, `secrets/`는 자기 테스트 외 live importer가 없어 stale
  leftover일 개연성이 매우 높다. SEC-002가 `secretstore`만 소비하므로 잘못된 패키지를
  배선할 위험(R7)은 grep 증거로 사실상 해소됨. **설계를 BLOCK하지 않음.** (단, 권장 default를
  "별도 cleanup SPEC"로 둔 것은 적절.)

---

## Defects Found

D1. `spec.md:L204-L209`(§2.2 "NOT changing adapter registration semantics") +
`spec.md:L241`(REQ-SEC2-001 "Resolver SHALL be the SOLE source") +
`spec.md:L255`(NFR-SEC2-004) — **CRITICAL (design soundness / completeness).**
SEC-002는 어댑터 자격증명을 Resolver로 일원화한다고 선언하나, **등록 게이트
`internal/adapters/registry.go:151-153`이 여전히 `os.LookupEnv(AuthEnvVars)`를 직접
읽는다**(추가로 admin-view status도 `registry.go:266-274`에서 동일하게 `os.LookupEnv` 사용).
SPEC §2.2는 이 게이트를 "preserved"로 명시한다. 결과:
- `backend: env` → `os.LookupEnv`가 프로세스 env에서 키를 찾으므로 정상(backward-compat OK).
- **`backend: k8s` / `vault` → 자격증명은 마운트 파일/Vault에 있고 프로세스 env에는 없으므로,
  naver/github가 Resolver로 키를 성공적으로 해석해도 `RegisterWithOptions`가 `ErrMissingAuth`로
  등록을 거부한다.** 즉 SEC-002의 핵심 목표(operator가 `backend: k8s`를 선택하면 어댑터가
  투명하게 키를 얻는다, `spec.md:L99-L103`)가 **유일하게 의미 있는 비-env 백엔드에서 달성되지
  않는다.** 이는 F-07이 지목한 "백엔드가 어댑터에서 도달 불가" 결함을 등록 게이트 위치로
  옮겨놓은 것과 같다 — Resolver는 "SOLE source"가 아니며(`registry.go:151-153`이 두 번째
  env-전용 소스), NFR-SEC2-004 "single source of truth"가 위반된다.
**Fix (택1):**
(1) 권장 — `RegisterWithOptions`의 auth 검사를 Resolver-aware로 만든다: `os.LookupEnv` 대신
주입된 Resolver로 `AuthEnvVars` 존재를 확인하거나, credentialed 어댑터는 CLI에서 Resolver로
키를 이미 해석했으므로 `RegisterWithOptions(a, RegisterOptions{SkipAuthCheck: true})`로
등록하고 등록 전 CLI가 직접 skip/loud-fail을 판정한다. 이 경우 §2.2의 "registry 등록
시맨틱 불변" 문장을 수정하고 REQ를 1개 추가(예: REQ-SEC2-007 "credentialed 어댑터 등록은
Resolver 해석 결과에 따른다; registry는 env를 직접 읽지 않는다").
(2) 대안 — SEC-002 범위를 **env 백엔드 한정**으로 명시적으로 축소하고, "k8s/vault 어댑터
등록은 `registry.go` 게이트 리팩터가 선행돼야 하는 알려진 공백"임을 §2.2/OQ에 명문화한다.
어느 쪽이든 미해결 시 재심에서 BLOCK로 격상해야 한다.

D2. `spec.md:L199-L203`(§2.2 "NOT changing the llm/config secret site … future caller") +
`spec.md:L418`(§6 ANCHOR row "the existing llm/config caller now use the Resolver") +
`spec.md:L515-L521`(OQ-4 "future caller") — **MAJOR (내부 모순 / citation 정확도).**
세 곳이 llm/config를 각각 "미수정 future caller"(§2.2), "이 SPEC로 Resolver를 쓰게 되는
existing caller"(§6 fan_in 집계), "future caller"(OQ-4)로 **상이하게** 규정한다. 실측:
`internal/llm/config/config.go:14`가 이미 `secretstore`를 import하고, `:45`
`var secretEnv = secretstore.NewEnvResolver()`, `:58` `secretEnv.Get(ctx,
"LITELLM_MASTER_KEY")`로 **이미 Resolver의 live caller**다(naver와 동일한 env-전용 하드코딩
안티패턴 포함). 따라서:
- §6의 "predicted fan_in >= 3 … from predicted to actual" 서술은 부정확하다 — naver +
  llm/config 두 caller가 **SEC-002 이전에 이미** `Get`을 호출 중이므로 fan_in 산정 자체가
  흐트러져 있다.
- llm/config를 "future"로 부르는 것은 코드와 모순(이미 present). 정직한 표현은 "llm/config는
  이미 secretstore를 쓰지만 naver와 동일한 env-전용 결함이 남아 있으며, 그 교정은 별도
  SPEC"이다.
**Fix:** §6 ANCHOR row에서 llm/config를 "SEC-002로 새로 Resolver를 쓰게 되는 caller"
집계에서 제외하고, §2.2/OQ-4를 "llm/config는 현재 `secretstore.NewEnvResolver()`로 이미
연결돼 있으나 동일한 하드코딩 결함이 있어 별도 리팩터 대상"으로 정정. fan_in 서술은 "이미
naver+llm/config 2 caller; SEC-002가 github CLI-site를 추가"로 수정.

D3. `spec.md:L87-L88`(HISTORY) + `spec.md:L246`(REQ-SEC2-006 Pattern 열) —
**MINOR (EARS 라벨/집계 정확도).**
HISTORY는 "Four EARS patterns used (Ubiquitous + Event-Driven + State-Driven + Unwanted)"라
주장하나 **State-Driven(`WHILE [state]`) 패턴을 쓰는 REQ가 없다**(REQ-005의 내장 `WHEN`은
Event 성격이지 State가 아님). 또한 REQ-SEC2-006은 Pattern 열에 "Unwanted"로 표기됐으나
실제 문장은 `IF…THEN` 구조가 없는 `SHALL NEVER`(부정형 Ubiquitous)다 — audit dimension 4가
요구한 "no-leak이 Unwanted를 올바르게 사용"은 충족되지 않는다(REQ-SEC2-004 vault는 올바른
Unwanted). 요구 자체는 이진 검증 가능하므로 MP-2는 통과하나 라벨은 부정확.
**Fix:** HISTORY의 패턴 집계를 "Ubiquitous + Event-Driven + Unwanted"로 정정(State-Driven
삭제). REQ-SEC2-006의 Pattern을 "Ubiquitous"로 바꾸거나, no-leak을 진짜 Unwanted로 재서술
(예: `IF a resolved secret value would be written to a log/metric/span/error, THEN the
system SHALL redact it to the env-var name`).

D4. `spec.md:L11` — **MINOR (frontmatter 컨벤션).**
필수 키 `created_at` 대신 `created` 사용. firewall MP-3 FAIL. `labels`(L15)는 존재.
**Fix:** `created:` → `created_at:` (이 저장소 전반 컨벤션 정렬; SPEC-CLI-003/UI-002와 동일
조치).

---

## Chain-of-Verification Pass

1차 검토 후 빠르게 지나친 영역을 재독했다:

- **모든 REQ-SEC2-NNN을 끝까지 정독**(스킵 없음): 001~006 + NFR 001~004 전부 EARS 구조·
  테스트 매핑 개별 확인. → D3(State-Driven 미사용, REQ-006 라벨)는 이 재독에서 포착.
- **REQ 번호 end-to-end 시퀀싱**: 표(L241-L246)와 §4 게이트(L266-L304) 양쪽에서 번호 대조 —
  갭/중복 없음 재확인.
- **Traceability 전수 확인**(샘플링 아님): 6 REQ + 4 NFR 각각 §3 명명 테스트 + §4 게이트
  보유. orphan AC / dangling REQ 없음.
- **Exclusions(§2.2) 구체성 점검**: 8개 항목 모두 구체적(키체인/vault-impl/koreanews/
  secretstore-시그니처/secrets-통합/koanf/llm-config/registration). 단 "registration
  semantics preserved" 항목이 D1의 근본 원인임을 재독 중 포착 — 단순 presence 확인을 넘어
  내용을 코드와 대조한 결과 치명 공백 발견.
- **요구 간 모순 탐색**(단일 요구 내부가 아니라 문서 전체): §6 vs §2.2 vs OQ-4의 llm/config
  규정 충돌(D2)을 이 교차 점검에서 발견. §2.2의 registration-preserved vs REQ-001의
  SOLE-source(D1)도 교차 모순.
- **신규 발견**: registry admin-view(`registry.go:266-274`)도 `os.LookupEnv`를 사용 →
  D1의 영향 범위가 등록뿐 아니라 status 보고까지 확장됨을 추가 확인(D1 증거 보강).

1차에서 누락 후 2차에서 포착한 결함: D1의 registry status 경로(266-274), D3의 State-Driven
과장. 나머지 결함(D1 등록 게이트, D2)은 1차에서 식별됨.

---

## Recommendation

판정: **PASS-WITH-FIXES (Overall 0.62)**. F-07 진단 방향과 인용 정확도는 이 저장소 기준
최상위이며, 두 finding 교정(키체인 없음 / koreanews 무자격증명)은 코드로 참임을 확인했다.
그러나 **D1이 SEC-002의 핵심 목표를 비-env 백엔드에서 무력화**하므로 ready 전 반드시 해소해야
한다(미해소 시 재심 BLOCK).

ready 도달을 위한 최소 변경 목록:

1. **[필수·D1]** 등록 게이트(`registry.go:151-153`)와 Resolver의 이중 소스 문제를 해소.
   택1: (a) credentialed 어댑터를 `SkipAuthCheck:true`로 등록하고 CLI가 Resolver 해석
   결과로 skip/loud-fail을 판정하도록 §5.3·§2.2를 수정 + REQ 추가, 또는 (b) SEC-002 범위를
   env 백엔드 한정으로 명시 축소하고 k8s/vault 등록 공백을 §2.2/OQ에 명문화. 어느 경로든
   k8s 백엔드에서 naver/github가 **등록에 성공**함을 검증하는 테스트를 §3·§4에 추가.
2. **[필수·D2]** §6 ANCHOR row와 §2.2/OQ-4의 llm/config 규정을 코드 사실(이미 live
   secretstore caller)에 맞춰 통일. fan_in 서술을 "이미 naver+llm/config 2 caller"로 정정.
3. **[필수·D4]** frontmatter `created:` → `created_at:`.
4. **[권장·D3]** HISTORY 패턴 집계에서 State-Driven 삭제; REQ-SEC2-006 Pattern을
   "Ubiquitous"로 정정하거나 no-leak을 진짜 `IF…THEN` Unwanted로 재서술.

위 1~3을 반영하면 0.80+ 및 PASS 도달 가능. D1이 단일 최대 리스크다.

---

*End of SPEC-SEC-002 review-1.*
