# SPEC-EVAL-003 Plan — phased implementation

Status: draft companion to spec.md (v0.2.0)
Author: limbowl via manager-spec
Date: 2026-05-22 (amended 2026-05-30 — B1 adapter-model fix + scope reductions)
Methodology: **TDD** (per `.moai/config/sections/quality.yaml` default
`development_mode: tdd` — 그리고 본 SPEC의 본질에 가장 잘 맞는다:
golden set 자체가 실패하는 테스트 코퍼스다. 각 query는 expected
ranking을 RED → GREEN으로 전환시키며 M3 어댑터 체인의 한국어 우선
랭킹을 강제한다. 채점기 / κ 계산기 / snapshot writer 모두 unit test
first 작성)
Coverage target: 85%
Harness: **standard** (per `.moai/config/sections/harness.yaml` 자동
라우팅 — 10 EARS REQs (3 × P0 + 4 × P1 + 1 × P2) + 5 NFRs + 1 protocol
doc + 1 golden set + 1 sheet template + 1 kappa 계산기 + 1 CI workflow.
복잡도가 thorough 임계치에 도달하지 않음. Sprint Contract는 optional
이지만 §7.4에서 권장한다)

본 plan은 SPEC-EVAL-003 구현을 priority-ordered phase로 나눈다.
`.claude/rules/moai/core/agent-common-protocol.md` 규칙에 따라 시간
추정은 **금지**되며 phase는 우선순위 + 순서로만 표현된다.

---

## 1. Implementation principle

평가 SPEC은 **관찰자(observer)**이며 **개입자(actor)**가 아니다.
plan은 다음 원칙을 우선한다:

1. **Golden set IS the test corpus** — TDD 모드 채택의 핵심 근거.
   `tests/eval/korean/golden-set.jsonl`의 각 query 객체는 expected
   ranking을 가진 test fixture로 취급된다. M3 어댑터 체인이 expected
   ranking을 만족하지 못하면 baseline run은 자연스럽게 RED. 어댑터·
   라우터·토크나이저가 정합되면 GREEN.

2. **Manual round는 자동화하지 않는다** — protocol doc + rubric +
   sheet 템플릿만 ship한다. 자동 실행 스크립트는 D7 비차단 정책과
   D1 manual-only 결정에 위배.

3. **κ 계산기는 결정론적** — 동일 sheet 입력에 동일 κ 출력. 부동
   소수점 비교는 1e-6 tolerance. `kappa.go` unit test가 12개
   fixture (identical / random / divergent / weighted / missing-row
   등)를 cover.

4. **Snapshot은 append-only** — 한 번 생성된 baseline snapshot은
   수정 금지. 새 round는 새 파일. NFR-EVAL-003에 따라 최신 4개
   유지, 5번째부터 archive.

5. **Protocol doc reproducibility 우선** — NFR-EVAL-001 dry-run
   테스트가 SPEC amendment 게이트. doc 작성은 외부 reader 관점에서
   self-contained해야 한다.

6. **No premature LLM-as-judge** — HISTORY D1을 어기는 코드는 작성
   하지 않는다. 향후 calibration용 fixture만 reserved (post-V1).

---

## 2. Phase ordering

### Phase 0 — Plan-auditor PASS (Priority High)

- Plan-auditor가 spec.md + research.md + plan.md + acceptance.md
  (Phase 0 산물)를 검토.
- MAJOR / MINOR / NIT 발견사항을 amendment commit으로 해결.
- **(v0.2.0) R1 HARD blocker 해소됨**: 게이트 메트릭(REQ-EVAL-005)
  + 골든셋 스키마(REQ-EVAL-001/008) + §1.3을 실제 어댑터 모델
  (단일 `naver` SourceID + `naver_vertical` filter, 단일 `koreanews`
  SourceID)로 재작성. tasks.md T03/T04/T08의 HARD BLOCKER 전제가
  충족되었다. recall은 `SourceID == "naver"` 기준으로 측정 가능.
- research §10 open question 중 plan-block 항목 해결:
  - Q4 code-mixed 한영 비율 — annotation에서 native 검토 → plan 확정.
  - Q6 κ 게이트 0.6 vs 0.7 — research §5.4 근거로 0.6 확정.
  - Q5 mecab-ko-dic 버전 회귀 정책 — D6 baseline 재계산 정책 명시.
- 상태 전환: `draft → approved`.
- Block: Phase 1 시작 전 Phase 0 완료 필수.

### Phase 1 — Protocol & rubric authoring (Priority High)

Goal: 외부 rater가 본 doc만으로 50-query round를 수행할 수 있는
self-contained protocol 확보 (NFR-EVAL-001).

Tasks:

1. `docs/eval/ko/protocol.md` 작성 (rater handbook):
   - 라운드 전체 워크플로 (golden set 다운로드 → 시스템 실행 →
     query별 top-10 결과 캡처 → sheet 작성 → 제출).
   - 5점 척도 정의 (1=완전히 무관, 3=주변적 관련, 5=정확히 일치).
   - code-mixed 처리 가이드 (한국어 segment 평가 ≠ 영어 segment).
   - 토크나이저 품질 판단 방법 (mecab-ko 출력을 사용자가 어떻게
     관찰할지 — IDX-003 디버그 출력 또는 결과의 자연성).
   - 익명화 규칙 (R1/R2/R3 ID; rater-pool.md 위치).

2. `docs/eval/ko/rubric.md` 작성:
   - 각 점수의 한국어 example 5-10개 (concrete anchoring).
   - 카테고리별 특수 사례 (예: shopping bucket에서 가격 비교 결과
     누락 시 감점).
   - (v0.2.0: V1 invalid round 처리는 minimal re-round만. 정교한
     rubric amendment 절차 + calibration log + SPEC owner review는
     post-V1 — REQ-EVAL-009 deferral.)

3. `docs/eval/ko/onboarding.md` 작성:
   - NFR-EVAL-005 fluency 기준 (native or near-native).
   - 추천 background (IR 도메인 이해 + Korean web 문화 친숙).
   - 익명 ID 발급 + 첫 round 안내.

4. `docs/eval/ko/kappa-interpretation.md` 작성:
   - Landis-Koch 표 (research §5.4).
   - 0.6 게이트 근거 + invalid round 시 minimal re-round 포인터.
   - (v0.2.0: Krippendorff α 해석은 post-V1로 보류 — V1은 Cohen/
     Light κ만 다룬다.)

5. (v0.2.0: `docs/eval/ko/calibration-log.md` 구조화 ledger는
   post-V1로 보류 — REQ-EVAL-009 deferral. V1은 protocol.md에
   "invalid round → re-round" 규칙만 명시한다.)

6. `docs/eval/ko/rater-pool.md` 템플릿 작성:
   - 빈 양식 + 익명 ID 발급 절차.

7. **Reproducibility dry-run test** (NFR-EVAL-001):
   - 본 plan 외부의 검토자(예: 다른 SPEC owner)가 protocol을 읽고
     1-query 시뮬레이션을 수행 후 절차 요약 작성.
   - SPEC owner가 요약과 의도된 워크플로 일치 확인.
   - 불일치 발생 시 protocol amendment.

Exit criterion:
- 6개 docs 모두 published.
- Reproducibility dry-run PASS.
- Frontmatter / 형식 lint 통과.

### Phase 2 — Golden set curation (Priority High)

Goal: 50 queries × 6 buckets, PII 0건, provenance 100% 문서화.

Tasks:

1. **News bucket 12 queries** 큐레이션:
   - Naver DataLab 최근 90일 시사 트렌드에서 4 query (capture date
     기록).
   - 합성 query 8개 (정치 / 경제 / 사회 분야 균등).
   - 각 query의 `expected_sources`에 `naver` SourceID +
     `expected_naver_vertical: news`를 기록(단일 어댑터 + vertical
     filter 모델, `naver.go:181`). 분리 `naver-news` ID는 사용하지
     않는다.

2. **Blog bucket 10 queries** 큐레이션:
   - Naver DataLab 라이프 / 리뷰 트렌드 4 + 합성 6.
   - `expected_sources: [naver]` + `expected_naver_vertical: blog`.

3. **Shopping bucket 8 queries**: 카테고리 균등 (가전 / 패션 /
   식품 / 생활용품 각 2).
   - `expected_sources: [naver]` + `expected_naver_vertical: shop`.

4. **Academic-tech bucket 8 queries**: 한국어 학술 키워드 + 한영
   기술 용어 mix (router는 `korean` 또는 `mixed` 양쪽 가능).
   - **주의**: Naver에는 `academic` vertical이 없다(라이브 코드).
     1차 target은 비-Naver SourceID(`arxiv`, `github`, …)이며
     `expected_naver_relevant: false`가 기본. `naver` blog/news
     결과가 독립적으로 기대되는 경우에만 `naver`를 추가한다.

5. **Code-mixed bucket 6 queries**:
   - research §6의 6 query 시안을 native rater 1인 리뷰 → 확정.
   - 한영 비율 5:5 4개 + 7:3 2개 잠정 (Phase 0에서 확정).

6. **Cultural bucket 6 queries**: K-pop / 드라마 / 한식 / 한자
   각 2 (한자는 한국어 한자어 검색 — 일본/중국 구별).

7. `tests/eval/korean/golden-set.jsonl` 작성 — 50 객체 JSONL.

8. `docs/eval/ko/golden-set-provenance.md` 작성 — query_id 별
   출처 (DataLab URL + capture date OR "synthetic, 2026-MM-DD").

9. **PII grep CI** 추가 (`.github/workflows/korean-eval.yml`의
   pre-check 단계):
   - email regex
   - 010-XXXX-XXXX 휴대폰
   - 주민번호 패턴 (보조)
   - 신용카드 패턴 (보조)
   - zero match 시 통과.

Exit criterion:
- 50 queries × 6 buckets 분포 정확.
- provenance 문서화 100%.
- PII grep 0 matches.

### Phase 3 — Scoring sheet template + κ 계산기 (Priority High, TDD)

Goal: `kappa.go` + sheet template + loader/scoring 단위 테스트 모두
GREEN.

Tasks (TDD RED → GREEN → REFACTOR):

1. **RED**: `internal/eval/korean/loader_test.go` 작성:
   - `TestLoadGoldenSet_50Objects` — 정확히 50개 객체.
   - `TestLoadGoldenSet_CategoryDistribution` — 12/10/8/8/6/6.
   - `TestLoadGoldenSet_AllRequiredFields` — 필수 필드 누락 검출.
   - `TestLoadGoldenSet_InvalidJSON_ReturnsError`.
   - 실행 → 모두 실패 (loader.go 없음).

2. **GREEN**: `internal/eval/korean/loader.go` 최소 구현:
   - `LoadGoldenSet(path string) ([]GoldenQuery, error)`.
   - 카테고리 카운트 검증.
   - 필수 필드 reflection 검증.
   - 테스트 통과.

3. **RED**: `internal/eval/korean/scoring_test.go`:
   - `TestTopKRecall_PerfectMatch` → recall = 1.0.
   - `TestTopKRecall_NoMatch` → 0.0.
   - `TestTopKRecall_PartialMatch` → 정확한 분수.
   - `TestTopKRecall_NaverSubset` — Naver-relevant subset만 계산.
   - `TestMRRAt10_FixedFixture` → 알려진 값.
   - `TestPerCategoryRecall` — 6 카테고리 dict 반환.
   - 실행 → 모두 실패.

4. **GREEN**: `internal/eval/korean/scoring.go` 구현:
   - `Top3NaverRecall(round Round, gold []GoldenQuery) float64`.
   - `MRRAt10(round Round, gold []GoldenQuery) float64`.
   - `PerCategoryRecall(round Round, gold []GoldenQuery) map[string]float64`.

5. **RED**: `internal/eval/korean/kappa_test.go`:
   - `TestCohenKappa_IdenticalRaters` → 1.0.
   - `TestCohenKappa_RandomRaters` → 0.0 ± 0.05.
   - `TestCohenKappa_DivergentRaters` → 0.3-0.5 범위.
   - `TestLightMeanKappa_ThreeRaters` — pairwise 3개 평균.
   - `TestKappa_MissingRow_ReturnsError` — sheet 정합성 검증.
   - (v0.2.0: `TestKrippendorffAlpha_OrdinalAuxiliary`는 post-V1로
     보류 — V1 kappa.go는 α를 구현하지 않는다.)

6. **GREEN**: `internal/eval/korean/kappa.go` 구현:
   - `CohenKappa(r1, r2 []int) (float64, error)`.
   - `LightMeanKappa(sheets [][]int) (float64, error)`.
   - (v0.2.0: `KrippendorffAlphaOrdinal`는 post-V1 보류.)

7. `tests/eval/korean/scoring-sheet-template.csv` 작성 — CSV header
   per REQ-EVAL-003.

8. **REFACTOR**: 중복 제거 (각 metric 함수의 GoldenQuery 매칭 로직
   공통화 등). 테스트 GREEN 유지.

Exit criterion:
- 모든 unit test PASS.
- `go test ./internal/eval/korean/...` race-clean.
- Coverage ≥ 85% (per 본 SPEC coverage_target).

### Phase 4 — Snapshot writer + 보존 정책 (Priority High, TDD)

Goal: baseline snapshot JSON 직렬화 + append-only + 보존 4개.

Tasks:

1. **RED**: `internal/eval/korean/snapshot_test.go`:
   - `TestSnapshot_ValidRound_WritesFile`.
   - `TestSnapshot_InvalidRound_DoesNotWrite` (κ < 0.6 시 거부).
   - `TestSnapshot_AppendOnly_RejectsOverwrite`.
   - `TestSnapshot_RetentionPolicy_KeepsLatestFour` — 5번째 추가
     시 가장 오래된 것이 archive로 이동.
   - `TestSnapshot_JSONSchema_AllFields` — REQ-EVAL-008 필드 검증.
   - `TestSnapshot_GoldenSetSHA256` — 결정론.

2. **GREEN**: `internal/eval/korean/snapshot.go` 구현:
   - `WriteSnapshot(round Round, tag string, dir string) error`.
   - 유효성 검증 → 거부 vs 작성.
   - 보존 정책 (archive/ 디렉토리).

3. `tests/eval/korean/baseline-snapshots/.gitkeep` 빈 디렉토리 보존.

4. **REFACTOR**: 파일 IO 추상화 (test에서 in-memory FS 사용 가능
   하도록 io/fs.FS interface 채택 검토).

Exit criterion:
- snapshot test 모두 PASS.
- Manual smoke: dummy 3-rater round → snapshot file 생성 → 보존
  policy 동작.

### Phase 5 — CI workflow (Priority Medium)

Goal: release tag commit 시 artifact 3종 emit, build 실패 없음.

Tasks:

1. `.github/workflows/korean-eval.yml` 작성:
   - `on: { release: { types: [published] } }` 또는 tag push.
   - Steps:
     a. Checkout.
     b. PII grep gate (Phase 2 정책).
     c. `go test ./internal/eval/korean/...` race-clean.
     d. golden-set.jsonl + scoring-sheet-template.csv를 artifact
        로 업로드.
     e. 직전 valid snapshot vs 현재 snapshot 비교 → `baseline-diff
        -report.md` 생성 → artifact 업로드.
     f. `continue-on-error: true` 모든 metric step (HISTORY D7
        비차단 정책).

2. **Smoke test**: 가짜 release tag로 workflow trigger → 3 artifact
   업로드 확인 → 빌드 status `success`.

3. README badge 추가 (선택):
   ```
   [![Korean Eval Artifacts](https://github.com/.../actions/
   workflows/korean-eval.yml/badge.svg)](https://github.com/.../
   actions/workflows/korean-eval.yml)
   ```

Exit criterion:
- Smoke test PASS.
- 임의 release tag로 trigger 성공.

### Phase 6 — Baseline run on M3 stack (Priority Medium)

Goal: M3 implemented 시점의 baseline round 1회 완료 → V1 release
시점의 `v1.0.0.json` snapshot 확보.

Tasks:

1. M3 의존 SPEC 모두 implemented 상태인지 확인:
   - SPEC-IR-001 (이미 implemented).
   - SPEC-ADP-008 (M3 행 — status 확인).
   - SPEC-ADP-009 (M3 행 — status 확인).
   - SPEC-IDX-003 (M3 행 — status 확인).
   - 누락 시 본 phase 차단; 해당 SPEC owner에게 escalation.

2. 3 rater 모집 (NFR-EVAL-005 fluency 기준).
3. Rater에게 protocol + rubric + golden-set + sheet template 배포.
4. Round 실행 (각 rater 50 queries 채점).
5. 3 sheets 수집 → loader → κ 계산 → ≥ 0.6 확인 → valid 마킹.
6. M3 exit gate 검증: top-3 Naver recall ≥ 0.80?
   - 통과 시 snapshot 작성, SPEC-REL-001에 evidence 전달.
   - 미달 시 회귀 진단 (IR-001 / ADP-008 / IDX-003 owner에게
     escalation), 본 phase는 진단 결과 별도 처리.
7. invalid round (κ < 0.6) 시 V1 minimal path: 라운드 폐기 → 새
   sheet로 re-round → κ ≥ 0.6 도달 시 snapshot (REQ-EVAL-009
   v0.2.0). 정교한 calibration ceremony는 post-V1.

Exit criterion:
- valid round 1회 완료.
- `baseline-snapshots/v1.0.0.json` 생성.
- top-3 Naver recall ≥ 0.80 (M3 exit 게이트 충족).

### Phase 7 — Sync phase (Priority Low)

Goal: 문서 + PR.

Tasks:

1. `manager-docs`:
   - 부모 README.md에 "Evaluation — Korean-locale benchmark" 섹션
     추가 (분기 manual round + artifact location).
   - SPEC-DOC-001 사이트 (M9 publish 시점) 평가 페이지에 cross-link.
2. CHANGELOG entry:
   - V1.0.0 entry에 "Korean-locale 50-query manual eval baseline
     established" 항목 추가.
3. `manager-git`:
   - PR title: `feat(eval): implement SPEC-EVAL-003 — Korean-locale
     50-query manual benchmark (M8)`.
   - PR body: spec.md / research.md / plan.md / acceptance.md
     links + REQ acceptance checklist + baseline snapshot URL.
4. 상태 전환: `approved → implemented` (merge + baseline round
   완료).

---

## 3. Test inventory (TDD checkpoints)

Phase별 TDD 체크포인트:

- Phase 3 (loader): `TestLoadGoldenSet_50Objects`,
  `TestLoadGoldenSet_CategoryDistribution`,
  `TestLoadGoldenSet_AllRequiredFields`,
  `TestLoadGoldenSet_InvalidJSON_ReturnsError`.

- Phase 3 (scoring): `TestTopKRecall_PerfectMatch`,
  `TestTopKRecall_NoMatch`, `TestTopKRecall_PartialMatch`,
  `TestTopKRecall_NaverSubset`, `TestMRRAt10_FixedFixture`,
  `TestPerCategoryRecall`.

- Phase 3 (kappa): `TestCohenKappa_IdenticalRaters`,
  `TestCohenKappa_RandomRaters`,
  `TestCohenKappa_DivergentRaters`,
  `TestLightMeanKappa_ThreeRaters`,
  `TestKappa_MissingRow_ReturnsError`.
  (v0.2.0: `TestKrippendorffAlphaOrdinal`는 post-V1 보류.)

- Phase 4 (snapshot): `TestSnapshot_ValidRound_WritesFile`,
  `TestSnapshot_InvalidRound_DoesNotWrite`,
  `TestSnapshot_AppendOnly_RejectsOverwrite`,
  `TestSnapshot_RetentionPolicy_KeepsLatestFour`,
  `TestSnapshot_JSONSchema_AllFields`,
  `TestSnapshot_GoldenSetSHA256`.

- Phase 1 (reproducibility): manual `ProtocolReproducibilityDryRun`
  (외부 검토자 1인 + SPEC owner sign-off).

- Phase 2 (PII): `TestGoldenSet_PIIGrep_ZeroMatches`.

- Phase 5 (CI): manual `WorkflowSmokeTestArtifacts3Uploaded`.

- Phase 6 (baseline): manual `BaselineRound_KappaGe06_RecallGe080`.

Manual 항목은 plan-auditor에서 evidence 형태(스크린샷, log, sign-off
docstring)로 검증.

---

## 4. MX tag plan

본 SPEC은 새 코드를 작성하지만 외부 caller가 거의 없는 평가 도구
영역이라 `@MX:ANCHOR` 적용 대상은 제한적이다.

| File | Tag | Reason |
|------|-----|--------|
| `internal/eval/korean/scoring.go::Top3NaverRecall` | `@MX:ANCHOR` | M3 exit gate evidence를 생성하는 함수. SPEC-REL-001이 consumer. |
| `internal/eval/korean/kappa.go::LightMeanKappa` | `@MX:ANCHOR` | round validity 결정 함수. REQ-EVAL-004 / REQ-EVAL-009 모두 consumer. |
| `internal/eval/korean/snapshot.go::WriteSnapshot` | `@MX:WARN` + `@MX:REASON` | append-only 정책 위반 시 데이터 손실 위험. `@MX:REASON: snapshots are immutable evidence; overwrite is rejected to prevent baseline tampering`. |
| `internal/eval/korean/loader.go::LoadGoldenSet` | `@MX:NOTE` | golden set 스키마 변경 시 amendment 절차 필요. |

`@MX:TODO`는 RED phase에서 자동 부착 (각 unit test의 빈 구현체).
GREEN phase에서 제거.

---

## 5. Risk-driven sequencing notes

research §11 risk와 phase mapping:

- rater 3인 모집 실패 → Phase 6 차단. 운영 정책 분리이며 SPEC
  scope 외이나, Phase 0에서 모집 계획 확인 권고.
- κ < 0.6 반복 → Phase 6 V1 minimal re-round (REQ-EVAL-009 v0.2.0).
  정교한 calibration ceremony + rubric anchor 보강은 post-V1.
- Naver DataLab 정책 변경 → Phase 2 provenance 보관 + fallback
  큐레이션 방안 (RSS 등).
- ADP-008 어댑터 ID 변경 → Phase 6 baseline run 실행 전 ADP-008
  owner와 freeze 합의.
- mecab-ko-dic 버전 변동 → Phase 6 snapshot의 `tokenizer_version`
  필드로 추적, D6 정책 적용.
- code-mixed 분류 underperform → 본 SPEC은 측정만; 회귀 발견 시
  IR-001 owner에게 위임 (Phase 6에서 escalation).
- manual round 운영 부담 → CI artifact는 자동 (Phase 5), manual
  skip은 release note 명시.
- LLM-as-judge 도입 압력 → HISTORY D1 + research §4·§9 인용으로
  거절.
- 골든셋 PII 누출 → Phase 2 CI gate (REQ-EVAL-004 NFR).

---

## 6. Sync-phase deliverables (Phase 7)

- 부모 README.md: "Evaluation" 섹션 + Korean-locale 안내.
- CHANGELOG: V1.0.0 entry에 baseline 추가 항목.
- PR title: `feat(eval): implement SPEC-EVAL-003 — Korean-locale
  50-query manual benchmark (M8)`.
- PR body:
  - spec.md / research.md / plan.md / acceptance.md links.
  - REQ acceptance checklist.
  - baseline `v1.0.0.json` snapshot link.
  - CI workflow first-run link (artifact 3종 업로드 evidence).
- 상태: `approved → implemented` on merge + baseline 완료.
- Notify: SPEC-REL-001 owner (V1 release evidence 전달); SPEC-EVAL-001
  owner (cross-reference 안내).

---

## 7. Open factoring decisions deferred to run phase

본 항목들은 plan 시점에 의도적으로 미결정 — implementation detail
이며 run-phase agent 또는 annotation cycle에서 확정.

1. **golden set 큐레이션 분담** — SPEC owner 단독 vs rater 협업.
   plan은 SPEC owner 단독 큐레이션 권장 (consistency 확보) 후
   Phase 2 검토 단계에서 1인의 native rater 리뷰. annotation에서
   확정.

2. **code-mixed 한영 비율 확정** — research §6의 5:5 4개 + 7:3 2개
   잠정안을 plan annotation에서 native 검토 후 확정. Phase 2 진입
   전 필수.

3. **kappa 가중 (weighted vs unweighted)** — V1은 unweighted Cohen
   κ (research §5.1). 만약 4개 round 누적 후 ordinal 가중치
   필요성 surfacing되면 EVAL-003 v1.1에서 weighted 도입.

4. **rater-pool.md 비밀 관리** — git-crypt vs separate secret store
   vs SPEC owner local. run-phase에서 결정. plan은 "SPEC owner only
   접근" 정책만 명시.

5. **CI workflow trigger 조건** — release tag (published) vs tag
   push vs nightly cron. plan 권장: release published. annotation
   에서 확정.

6. **snapshot version pinning convention** — `v1.0.0.json` vs
   `2026-05-22-v1.0.0.json` (date prefix). plan 권장: 단순 tag (날짜
   는 snapshot 내부 `round_date` 필드).

7. **baseline-diff-report.md 형식** — Markdown table only vs +
   Mermaid graph. plan 권장: Markdown table only (CI artifact
   consumer가 GitHub UI에서 직접 읽음). 향후 dashboard 도입 시
   재검토.

8. **Sprint Contract 채택 여부** — harness standard이므로 optional.
   plan 권장: Phase 3 (scoring 계산기) + Phase 4 (snapshot) 두
   technical phase에서 Sprint Contract 채택. evaluator-active는
   acceptance criterion 기반 채점 (top-3 recall 계산 정확도,
   κ 결정론, snapshot 보존 정책 준수). Phase 1·2·5·6·7은 contract
   없이도 acceptance가 명확.

이들은 scope-bounded — SPEC 계약은 변경하지 않는 mechanical
implementation choices.

---

*End of SPEC-EVAL-003 plan v0.2.0 (draft).*
