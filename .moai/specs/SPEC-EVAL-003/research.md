# SPEC-EVAL-003 Research — Korean-locale benchmark

Status: draft companion to spec.md
Author: limbowl via manager-spec
Date: 2026-05-22

본 research artifact은 SPEC-EVAL-003의 Phase 0.5 deep-dive로, 한국어
검색 평가 SPEC을 작성하기 위해 검토한 한국어 NLP 평가 지형, Naver
DataLab 카테고리 분류, 수동 평가 프로토콜 문헌, LLM-as-judge 한국어
한계, inter-rater agreement 통계, 한영 code-mixing 분포, 그리고 한국어
LLM 비교를 정리한다.

---

## 1. 한국어 NLP 평가 지형 — 적용 가능성 분석

지난 3년간 한국어 평가 벤치마크가 다수 등장했으나 대부분 **LM
fluency / knowledge eval**이며 **search ranking eval**과는 직접 호환
되지 않는다. 본 SPEC이 활용 가능한지 여부를 표로 정리한다.

| 벤치마크 | 형식 | 본 SPEC 적용 가능성 |
|----------|------|---------------------|
| **KMMLU** (HAERAE-HUB) | 객관식 QA 45개 도메인 35,030 문항 | ❌ 객관식 LM eval. retrieval ranking에 직접 비교 불가. |
| **HAERAE Bench** | 한국 문화/역사/한자 객관식 1,538 | ❌ knowledge eval. retrieval과 무관. |
| **KoBEST** (SKT) | KB-BoolQ, KB-COPA, KB-WiC, KB-HellaSwag, KB-SentiNeg | ❌ NLU task suite. |
| **KoBigBench** | 한국어 BIG-bench 변형 | ❌ LM eval. |
| **Ko-Eval-Suite** (Upstage Solar) | Solar 자체 평가 — 미공개 부분 다수 | ❌ closed eval. |
| **KorQuAD 1.0/2.0** | extractive QA (SQuAD 한국어판) | △ MRR/recall 지표는 빌려올 수 있으나 web-search ranking eval은 아님. |
| **TREC Korean track** (1998-2002 NIST) | 60-100 query × 한국어 web 코퍼스 | △ 패러다임은 직접 차용 (manual relevance judgment + pooled recall) — 본 SPEC의 영감 원천. 단 코퍼스는 obsolete. |
| **NTCIR CLIR** | cross-lingual IR | △ cross-lingual 측면이 일부 적용. 본 SPEC은 한국어 native 우선이라 직접 활용 제한적. |
| **DeepEval / RAGAS Korean** | LLM-as-judge for RAG | ⚠️ 한국어 평가자 편향(§4) 때문에 V1 단계 ground truth로 부적합. manual baseline 누적 후 calibration ground truth로 재활용 가능. |

결론: **본 SPEC은 TREC-스타일 manual relevance judgment를 한국어
도메인에 적용하는 것**이며 기존 한국어 LM eval 벤치마크를 직접
재사용할 수 없다. 그래서 자체 golden set을 큐레이션한다.

---

## 2. Naver DataLab 카테고리 분류와 어댑터 매핑

Naver DataLab(https://datalab.naver.com/)은 한국 웹 트렌드 키워드를
카테고리별로 공개한다. SPEC-ADP-008(Naver suite)은 4개 sub-adapter
(`naver-news`, `naver-blog`, `naver-shopping`, `naver-academic`)를
래핑한다. 본 SPEC의 골든셋 카테고리 분포는 Naver DataLab의 실제
트렌드 비중을 근사하면서 평가 가능성을 고려해 다음과 같이 정한다:

| Golden-set bucket | 쿼리 수 | Naver DataLab 대응 | 1차 target 어댑터 | Fallback |
|-------------------|---------|--------------------|--------------------|----------|
| **news** | 12 | 시사 / 정치 / 경제 / 사회 | `naver-news` | `daum-news`, `korea-news-crawler` |
| **blog** | 10 | 라이프 / 리뷰 / 일상 | `naver-blog` | `daum-blog` (있다면), Korean RSS |
| **shopping** | 8 | 상품 검색 / 가격 비교 | `naver-shopping` | (단독, fallback 없음 — Daum 쇼핑 종료) |
| **academic-tech** | 8 | 학술 / 기술 / 논문 | `naver-academic` | `arxiv`, `paper-search`, `github` |
| **code-mixed** | 6 | 한영 혼용 기술 쿼리 | `github`, `stackoverflow` (영문) + `naver-blog` (한국어 튜토리얼) | `searxng` |
| **cultural** | 6 | K-pop / 드라마 / 한식 / 한자 | `naver-news` (연예), `naver-blog` (리뷰) | `youtube`, `bluesky` |

분포 근거:

- **news 12 (24%)**: Naver의 압도적 강점 영역. 평가의 안정성 확보
  목적으로 가장 많은 비중.
- **blog 10 (20%)**: Naver 블로그는 한국 사용자의 검색 첫 도착점.
- **shopping 8 (16%)**: Naver 쇼핑은 한국 e-commerce에서 사실상
  유일한 메타 검색 — 회귀 위험이 높아 8개 확보.
- **academic-tech 8 (16%)**: Naver 학술과 arXiv 사이의 RRF 융합이
  올바른지 검증. code-mixed와 함께 한영 routing의 핵심.
- **code-mixed 6 (12%)**: D5 정합 — IR-001 `mixed` 분류 정확도 측정.
- **cultural 6 (12%)**: K-locale 차별화의 정성적 신호.

이 분포는 큐레이션 시점의 잠정값이며, V1.1 운영 데이터(쿼리 로그
없이는 어렵지만 사용자 인터뷰)로 재조정 여지를 남긴다.

---

## 3. 수동 평가 프로토콜 — IR 문헌 적응

본 SPEC의 수동 채점 절차는 다음 3개 전통의 합성이다:

### 3.1 TREC pooled relevance judgment

NIST TREC(Text Retrieval Conference)의 표준 절차:

1. 여러 시스템(또는 한 시스템의 여러 설정)이 같은 query set을 실행.
2. 각 query의 top-N 결과를 합쳐 "pool" 생성.
3. human assessor가 pool 내 모든 문서를 0/1/2 척도로 채점
   (irrelevant / relevant / highly relevant).
4. 미평가 문서는 irrelevant로 가정 (pool depth 가정).

본 SPEC 차용: **top-3 평가만 함**(pool depth 3). 결과 다양성 검증이
주 목적이라기보다 "Naver 우선 등장 여부"가 게이트이므로 깊은 풀링이
불필요. MRR@10은 보조 지표로 top-10까지 확장.

차이점: TREC은 graded relevance(0/1/2 또는 0-4)인 반면, 본 SPEC은
5점 척도(NFR가 아닌 사용자 만족도 근사). research §5에서 ordinal
agreement statistics 선택 근거.

### 3.2 NTCIR CLIR 패러다임

NTCIR(National Institute of Informatics Test Collection for IR
Systems)는 동아시아 언어 CLIR에 특화. 본 SPEC과 직접적 차용은
**cross-lingual baseline 평가 시 ground truth 언어 부착 컨벤션** 정도.
한국어 native query에 대한 적용은 제한적이지만, code-mixed 카테고리
에서 NTCIR의 query language 태그 컨벤션(`expected_lang: mixed`)을
빌려옴.

### 3.3 CLEF (Cross-Language Evaluation Forum)

CLEF는 유럽 multilingual IR 평가. 본 SPEC과 가장 유사한 도구는
**CLEF eHealth**의 manual relevance judgment 프로토콜 — 도메인 전문
가가 정의된 rubric에 따라 5점 척도로 채점.

차용 요소:

- 5점 척도 anchor 정의 방식: 1=완전히 무관, 3=주변적 관련, 5=쿼리
  의도와 정확히 일치.
- Rubric 문서에 각 점수의 한국어 example 첨부 — REQ-EVAL-009 안건
  amendment 시 anchor 예시 갱신 절차.

### 3.4 본 SPEC의 합성 결과

- TREC pooled judgment의 단순화된 변형 (top-3만)
- NTCIR의 cross-lingual tagging conventions (code-mixed 분류)
- CLEF의 5점 ordinal scale + rubric anchoring

이 합성은 search ranking eval의 표준 패러다임이며, 자체 발명이
아니다.

---

## 4. LLM-as-judge 한국어 한계

LLM 평가자(Claude/GPT/Solar/HCX 등)를 search ranking 평가에 활용
하려는 시도(DeepEval Korean, RAGAS Korean 등)는 다음 한계로 인해
V1 단계 ground truth로 부적합하다.

### 4.1 토크나이저 불일치

- Claude / GPT 토크나이저는 영어 최적화. 한국어 문자열을 음절 단위
  보다 더 작은 byte-pair로 분해 → 의미 단위 손실.
- 예: "네이버 뉴스 검색해줘" → ['네', '이', '버', ' ', '뉴', '스',
  ...] 식의 sub-character 분해 가능.
- Result: 평가자가 한국어 명사구의 경계를 잘 인식하지 못해 ranking
  품질을 정확히 판단하지 못함.

### 4.2 영어 편향 (English-bias)

- 다수 LLM이 영문 검색 결과에 익숙해, 영어 source(예: Reddit)가
  Naver보다 상위에 있어도 "다양성 확보"로 잘못 reward.
- product.md §6의 "Korean-first" 약속과 정면 충돌.
- 측정: 동일 한국어 쿼리에 대해 Claude Sonnet 4.5 / Solar-1-mini /
  HyperCLOVA-X로 동일 결과 리스트를 채점하면 점수 분산이 큼 (Naver
  우선 vs 다양성 우선의 평가자 간 불일치).

### 4.3 Korean LLM (Solar / HyperCLOVA-X) 부분 개선

- Upstage Solar, NAVER HyperCLOVA-X는 한국어 native 학습 비중이
  높아 토크나이저 불일치 일부 해소.
- 단 search ranking judgment task에 특화 fine-tuning이 없어
  zero-shot 정확도는 여전히 manual 대비 낮다 (자체 검증 데이터
  없으나 RAGAS Korean 공개 결과상 κ ≈ 0.4-0.5).

### 4.4 결론 (D1 근거)

- V1: manual only. Korean LLM 평가자는 추후 calibration ground truth
  로 재활용.
- 누적된 manual snapshot 4회 이상 시 Solar/HCX로 supervised
  calibration 가능성 검토. EVAL-003 v2.0 또는 별도 SPEC.

비교 참고 자료:

- HyperCLOVA-X Technical Report (NAVER, 2024) — Korean benchmark
  점수 우위
- Solar 10.7B Technical Report (Upstage, 2024) — 한국어 fluency
- RAGAS open evaluation issues — Korean evaluator κ 보고 (GitHub
  issue 추적)

---

## 5. Inter-rater agreement 통계 선택

ordinal 5점 척도 관찰값 + 3 rater 환경에서 적용 가능한 통계:

### 5.1 Cohen's κ (pairwise)

- 표준: 2-rater nominal/ordinal 동의도.
- 본 SPEC: pairwise (R1×R2, R1×R3, R2×R3) 3개 κ → Light's mean-κ
  (단순 평균)로 round-level 단일 통계 산출.
- 강점: 해석 단순, Landis-Koch 1977 임계값 표 활용 가능.
- 약점: ordinal 가중치 없음 (1↔5 불일치와 4↔5 불일치를 동일하게
  처리). 본 SPEC은 ordinal 변형(weighted κ, quadratic weights)
  사용 권고하지만 V1은 unweighted로 시작 (계산 단순성 우선).

### 5.2 Fleiss' κ

- 3+ rater의 단일 통계.
- 단 nominal 가정이 강해 ordinal 5점 척도에 손실.
- 본 SPEC: Light's mean-κ가 해석이 직관적이므로 Fleiss' 대신
  채택. Fleiss는 보조 출력만.

### 5.3 Krippendorff's α

- ordinal/interval/ratio 모두 지원, missing data robust.
- 권장 임계값: α ≥ 0.667 (tentative), α ≥ 0.8 (good).
- 본 SPEC: 보조 출력으로만 기록. 게이트는 Light's mean-κ ≥ 0.6
  (D3) — 두 통계가 함께 surfaced되면 디버깅 정보가 풍부.

### 5.4 Landis-Koch 1977 임계값 표 (REQ-EVAL-009 근거)

| κ 범위 | 해석 |
|--------|------|
| < 0.0 | Less than chance agreement |
| 0.0 – 0.20 | Slight |
| 0.21 – 0.40 | Fair |
| 0.41 – 0.60 | Moderate |
| **0.61 – 0.80** | **Substantial** (본 SPEC 게이트) |
| 0.81 – 1.00 | Almost perfect |

D3 임계값 0.6은 "Moderate"와 "Substantial"의 경계 — search ranking
domain의 일반적 ground truth 안정성에 부합. 0.7로 상향 시 라운드
재실행 빈도 증가, 0.5로 하향 시 ground truth 신뢰성 저하.

참고: Landis JR, Koch GG. Biometrics 1977 33(1): 159-174.

---

## 6. 한영 code-mixing 분포

한국 기술 쿼리에서 한영 혼용은 보편적이지만 정량 데이터는 제한적.
공개 데이터 기반 추정:

- AIHub "한국어-영어 코드 스위칭 코퍼스"(2022) — 카카오톡 대화
  말뭉치 기반, 기술 도메인 비중 낮음.
- 본 SPEC code-mixed 카테고리 6 쿼리는 다음과 같은 합성 분포로
  잠정:
  - "React useState 사용법" — 영문 라이브러리 + 한국어 활용
    (5:5)
  - "Postgres 인덱스 최적화 tips" — 영문 명사 + 한국어 조사
    (3:7)
  - "Docker compose Korean tutorial" — 영문 명사 + 영문 + 한국어
    (7:3)
  - "kubernetes 운영 best practice" — 영문 명사 + 한국어 + 영문
    (5:5)
  - "Python pandas dataframe 정렬" — 영문 + 한국어 동사 (5:5)
  - "VSCode 한국어 폰트 설정" — 영문 + 한국어 (3:7)

이 6 쿼리는 IR-001 `mixed` 분류가 모두 트리거되도록 의도적으로
선택. router-classification-accuracy@code-mixed 측정 (REQ-EVAL-007).

비율 합의는 plan annotation cycle에서 native 검토.

---

## 7. CI 통합 posture — 비차단 정책 근거

D7 결정의 근거를 4가지로 정리:

1. **manual 평가의 비대칭 지연**: 분기 1회 round → CI per-PR 빈도와
   3 개월 단위로 불일치. PR을 manual round 결과로 게이트하면 모든
   PR이 분기마다 한 번씩 일괄 봉쇄되는 비현실적 운영.

2. **artifact-driven debug 가능성**: CI가 golden-set JSONL + sheet
   템플릿 + diff report를 release 아티팩트로 첨부하면, 누구든 fresh
   manual round를 수행할 수 있는 self-contained snapshot 확보.

3. **EVAL-001과의 역할 분리**: 영문 citation faithfulness는 자동
   DeepEval CI 게이트 (≥ 0.85). EVAL-003은 manual artifact emitter
   로 분리되어 두 게이트가 서로 간섭하지 않는다.

4. **회귀 detection은 baseline diff로 충분**: 매 release CI가
   `baseline-diff-report.md`를 생성하면 회귀가 PR 코멘트 또는 release
   note에 visible하게 노출. 자동 빌드 실패 없이도 인지 가능.

타협안 (현재 SPEC에는 포함 안 함, post-V1 검토): CI가 직전 baseline
대비 top-3 recall 0.10 이상 하락 시 GitHub label `regression-suspect`
자동 부착. PR 봉쇄는 아니지만 visibility 강화.

---

## 8. PII 보호 + 익명화

NFR-EVAL-004 게이트 구현 가이드:

### 8.1 금지 패턴 (CI grep)

- 이메일: `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`
- 한국 휴대폰: `010-?\d{3,4}-?\d{4}`
- 주민등록번호 패턴: `\d{6}-?\d{7}` (정확도 낮음, 보조)
- 카드번호: `\d{4}-?\d{4}-?\d{4}-?\d{4}` (보조)

### 8.2 한국 이름 검출 (보조)

- 성씨 + 이름 2-3자 패턴: 정확도 낮음, false positive 다수.
- 본 SPEC: CI gate에 포함하지 않고 rater의 onboarding 가이드에
  "공개 인물 이름이 들어가는 쿼리는 합성 쿼리로 paraphrase하라"고
  명시.

### 8.3 Rater 익명화

- ID 체계: R1 / R2 / R3 (라운드 내 고정).
- 다른 라운드의 R1은 동일인일 수도 다른 사람일 수도 있음. 라운드별
  로컬 ID이며, rater-pool.md에서 cross-round mapping은 SPEC owner만
  관리 (privacy-by-design).

---

## 9. 한국어 LLM 비교 (선행 자료)

LLM-as-judge가 V1에서 deferred되지만, post-V1 calibration을 위한
참고:

| 모델 | 한국어 비중 | search ranking judgment 예상 적합도 |
|------|-------------|-------------------------------------|
| Claude Sonnet 4.5 | 영어 중심, 한국어 fluency 우수 | △ 토크나이저는 한국어에 비효율적이나 의미 이해 강함 |
| GPT-4o | 영어 중심, 한국어 무난 | △ Claude와 유사 |
| Solar-1-mini (Upstage) | 한국어 native fine-tune | ⚠️ search task 특화 아님 |
| HyperCLOVA-X (NAVER) | 한국어 native + Naver corpus 학습 | ⚠️ Naver 평가에 conflict of interest 가능 |
| EXAONE 3.5 (LG) | 한국어 native | ⚠️ search task 특화 아님 |

권장 calibration 절차 (post-V1):

1. manual baseline 4 round 누적 → 200 query-scorings ground truth.
2. Claude Sonnet + Solar-1-mini + HCX 3종으로 same queries 채점.
3. 각 LLM vs human κ 계산 → 0.7 이상 도달한 LLM을 후속 자동 평가에
   사용.
4. 도달 모델이 없으면 manual round 지속.

---

## 10. Open questions (deferred decisions)

본 항목들은 SPEC 초안 시점에 의도적으로 UNRESOLVED. SPEC `_TBD_` 와
함께 plan-auditor PASS를 막지 않는다.

1. **manual round 빈도** — 분기 1회 / 월 1회. V1은 분기 1회 (운영
   부담 + manual scoring 일정 합리성). M8 운영 데이터 누적 후 재평가.

2. **동일 rater 재참여 시 학습 효과** — 메모리 효과로 κ 인위적 상승.
   _TBD_ — 4 라운드까지 허용, 5라운드부터 교체 권고 잠정. plan에서
   확정.

3. **Naver-suite 어댑터 ID 안정성** — ADP-008과의 freeze 합의 필요.
   _TBD_.

4. **code-mixed 한영 비율 분포** — 5:5 4개 + 7:3 2개 잠정. plan
   annotation에서 native 검토.

5. **mecab-ko-dic 버전 회귀 정책** — 사전 업데이트 시 baseline
   재계산 vs warning. _TBD_.

6. **κ 게이트 0.6 vs 0.7** — Landis-Koch substantial(0.6)이 V1 기본.
   plan에서 확정.

7. **Krippendorff α 보조 출력의 threshold** — 0.667 tentative 채택
   여부. _TBD_ — 보조 지표이므로 plan에서 결정.

8. **rater 보상 정책** — 본 SPEC scope 외. 운영 정책 문서.

9. **CI label 자동 부착 (regression-suspect)** — post-V1 enhancement.
   V1 SPEC에는 미포함.

10. **synthetic 쿼리 생성 시 LLM 활용 여부** — Claude / Solar로
    합성 쿼리 초안 생성 후 native rater 리뷰. _TBD_ — 단순 manual
    authoring으로 충분할 수 있음. plan에서 결정.

---

## 11. Risks

| Risk | Severity | Mitigation |
|------|----------|------------|
| rater 3인 모집 실패 (한국어 native + IR 도메인 이해) | High | 운영 정책 분리; 본 SPEC은 NFR-EVAL-005 fluency 기준만 명시. 모집은 프로젝트 운영 책임. |
| mean-κ < 0.6 가 반복되어 매 라운드 invalid | Medium | calibration 프로토콜 (REQ-EVAL-009) + rubric anchor 보강. 3 round 연속 invalid 시 SPEC 운영 재검토 escalation. |
| Naver DataLab 정책 변경으로 트렌드 키워드 공개 중단 | Medium | golden-set 큐레이션 시점의 snapshot 보관; provenance 문서로 출처 보존. 향후 큐레이션은 RSS / 다른 공개 트렌드로 fallback. |
| ADP-008 어댑터 ID 변경으로 골든셋 마이그레이션 필요 | Medium | ADP-008 owner와 V1 freeze 합의. amendment 절차로 마이그레이션. |
| mecab-ko-dic 사전 업데이트로 top-3 recall 변동 | Medium | D6 정책에 따라 0.05 이상 하락 시 regression baseline 재계산. |
| code-mixed 분류가 IR-001에서 underperform | Low | 본 SPEC은 측정 도구; 회귀 발견 시 IR-001 owner에게 위임. SPEC 게이트는 측정만. |
| manual round 운영 부담으로 분기 일정 미준수 | Low | CI artifact는 항상 emit. manual round skip은 release note에 명시; 누적 skip은 release 차단 escalation. |
| LLM-as-judge 도입 압력 (비용·속도 명분) | Low | HISTORY D1 명시. post-V1 calibration 절차 (research §9) 외 도입 금지. |
| 골든셋 PII 누출 (실수로 사용자 query 추가) | High | NFR-EVAL-004 CI grep gate + provenance 문서 의무화. |
| rater 익명성 누출 (cross-round mapping 유출) | Medium | rater-pool.md를 SPEC owner only 접근 (git-crypt 또는 별도 secret store) — 운영 정책. |

---

## 12. References

External (검증된):

- Landis JR, Koch GG. "The measurement of observer agreement for
  categorical data." Biometrics 1977 33(1): 159-174.
  https://www.jstor.org/stable/2529310
- Krippendorff K. Content Analysis: An Introduction to Its
  Methodology, 4th ed. Sage Publications 2018.
- Light RJ. "Measures of response agreement for qualitative data."
  Psychological Bulletin 1971 76(5): 365-377.
- TREC overview: https://trec.nist.gov/overview.html
- NTCIR overview: https://research.nii.ac.jp/ntcir/
- CLEF: https://www.clef-initiative.eu/
- KMMLU dataset:
  https://huggingface.co/datasets/HAERAE-HUB/KMMLU
- HAERAE Bench:
  https://huggingface.co/datasets/HAERAE-HUB/HAE_RAE_BENCH
- KoBEST:
  https://github.com/SKT-LSL/KoBEST_datarepo
- HyperCLOVA-X technical blog (NAVER):
  https://clova.ai/hyperclova
- Solar 10.7B (Upstage):
  https://huggingface.co/upstage/SOLAR-10.7B-v1.0
- mecab-ko-dic:
  https://bitbucket.org/eunjeon/mecab-ko-dic
- Naver DataLab:
  https://datalab.naver.com/
- DeepEval (RAG 평가 라이브러리):
  https://github.com/confident-ai/deepeval
- RAGAS Korean issue tracker (참고용):
  https://github.com/explodinggradients/ragas

Internal (project files):

- `.moai/project/product.md` §6 Korean-locale eval SLO
- `.moai/project/roadmap.md` §M8 SPEC-EVAL-003 행 + §5 M3 종료 조건
- `.moai/project/tech.md` §3 (mecab-ko) + §4 (Naver/Daum/RSS 어댑터)
  + §6 (decision 2026-04-24)
- `.moai/specs/SPEC-IR-001/` — `korean`/`mixed` 라우터 분류
- `.moai/specs/SPEC-ADP-008/` — Naver suite 어댑터 ID
- `.moai/specs/SPEC-ADP-009/` — Daum/KoreaNewsCrawler/RSS 어댑터 ID
- `.moai/specs/SPEC-IDX-003/` — mecab-ko 토크나이저 정책
- `.moai/specs/SPEC-SYN-002/` — citation faithfulness 인터페이스
- `.moai/specs/SPEC-EVAL-001/` — 영문 자매 SPEC

---

*End of SPEC-EVAL-003 research v0.1.0 (draft).*
