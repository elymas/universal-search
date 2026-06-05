---
id: SPEC-EVAL-003
version: 0.2.0
status: implemented
created: 2026-05-22
updated: 2026-05-30
author: limbowl
priority: P1
issue_number: 0
title: Korean-locale benchmark — 50-query manual scoring protocol for the V1 Korean-first ranking gate
milestone: M8 — Eval + polish
owner: expert-testing
methodology: tdd
coverage_target: 85
depends_on: [SPEC-IR-001, SPEC-ADP-008, SPEC-ADP-009, SPEC-IDX-003, SPEC-SYN-002]
blocks: [SPEC-REL-001]
related: [SPEC-EVAL-001, SPEC-EVAL-002]
---

# SPEC-EVAL-003: Korean-locale benchmark — 50-query manual scoring protocol

## HISTORY

- 2026-05-30 (amendment v0.2.0, limbowl via manager-spec):
  plan-auditor 재감사 통과를 위한 HARD blocker 수정 + 사용자
  승인 scope 축소. **상태는 draft 유지.**

  (B1 — HARD blocker 수정) REQ-EVAL-005의 V1 ship 게이트(top-3
  Naver recall ≥ 0.80)가 **존재하지 않는 어댑터 ID** 4개
  (`naver-news`/`naver-blog`/`naver-shopping`/`naver-academic`)
  위에 정의되어 측정 불가능했다. 라이브 코드(`internal/adapters/
  naver/naver.go:175,181`)는 단일 `SourceID: "naver"`이며 vertical
  은 `Filters[naver_vertical] ∈ {blog, news, web, shop, datalab}`
  + `DocType`로 표현된다. `naver-academic` vertical은 **존재하지
  않는다**. `internal/adapters/koreanews/koreanews.go:79,85`도 단일
  `SourceID: "koreanews"`이며 `daum-news`/`korea-news-crawler`
  개별 ID는 없다. 본 amendment는 게이트 메트릭 + 골든셋 스키마를
  실제 어댑터 모델로 재작성한다:
  - recall 메트릭은 실제 SourceID(`naver` 단일, `koreanews` 단일)
    를 키로 한다. Naver vertical 구분이 필요한 경우 실제 메커니즘
    (`naver_vertical` filter ∈ {blog, news, web, shop, datalab})
    으로 표현한다. `naver-academic`은 완전 삭제 — academic-tech
    버킷의 1차 target은 비-Naver 어댑터(arXiv/GitHub 등)로 정정.
  - 골든셋 `expected_sources`는 실제 SourceID(+ optional
    `expected_naver_vertical`)를 사용한다.
  - snapshot `adapter_versions` map은 실제 어댑터 ID(`naver`,
    `koreanews`, …)를 키로 한다.
  - §1.3 source list: `daum-news`/`korea-news-crawler` → `koreanews`,
    `naver-*` → `naver` + vertical filter로 정정.

  (B2 — scope 축소, 사용자 승인) V1 필수 항목만 게이트로 유지하고
  나머지는 명시적 deferral:
  - **Krippendorff α 보류**: V1 inter-rater 게이트는 Cohen/Light
    κ로 충분. α는 post-V1.
  - **REQ-EVAL-009 calibration 축소**: V1 path는 minimal
    (invalid round → re-round)만. 5-query 재채점 + FROZEN rubric
    amendment review + calibration-log 정교화는 post-V1.
  - **REQ-EVAL-006 per-category 0.10 flagged-warning**:
    observational — V1 게이트 항목 아님.
  - **EC-002(SHA drift) / EC-003(tokenizer version drift 주석)**:
    nice-to-have — deferred.
  - **V1 필수 유지**: golden-set loader, top-3 Naver recall(수정
    후), 3-rater Cohen κ ≥ 0.6 게이트, snapshot writer(append-only
    + SHA256), PII grep gate.

  (불변) D1 manual-only, D7 CI 비차단(artifact-only), EVAL-001과의
  자매·직교 관계는 그대로 유지된다. 본 amendment는 게이트의 측정
  가능성과 scope 경계만 수정하며 평가 철학은 변경하지 않는다.

- 2026-05-22 (initial draft v0.1.0, limbowl via manager-spec):
  M8 폴리시 단계의 한국어 평가 SPEC 초안. `.moai/project/roadmap.md`
  §M8 행("SPEC-EVAL-003 | Korean-locale benchmark | 50-query
  Korean-first eval, manual scoring protocol | expert-testing")과
  §5 M3 종료 조건("Korean query returns Naver results ranked
  first"), 그리고 `.moai/project/product.md` §6 V1 종료 지표
  ("Korean-locale result ranking relevance | manual eval ≥ 4/5
  on 50-query benchmark")의 회귀 가드로 설계되었다.

  본 SPEC은 SPEC-EVAL-001(영문 citation faithfulness)과 자매
  관계이며 SPEC-EVAL-002(어댑터 가용성)와는 직교한다. 영문 평가가
  자동 DeepEval CI 게이트인 반면, 한국어 평가는 LLM 평가자의
  한국어 편향(§research §4) 때문에 V1 단계에서는 의도적으로
  **수동(human-in-the-loop) 평가**만 채택한다. CI는 골든셋
  아티팩트만 산출하고 PR을 막지 않는다.

  Pinned decisions:
  (D1) **Scoring protocol = manual human-in-the-loop**, NOT
       LLM-as-judge. 근거: product.md §6 SLO 텍스트가 "manual
       eval"을 명시하고, 한국어 LLM 평가자(Claude/Solar/HCX)는
       토크나이저 불일치와 영어 편향으로 인해 ordinal relevance
       판단에서 인간과의 편차가 크다(research §4). LLM-as-judge는
       post-V1 enhancement로 명시 보류.
  (D2) **Golden set 구성 = 50 queries × Naver DataLab 카테고리
       분포**(news 12 / blog 10 / shopping 8 / academic-tech 8 /
       code 한영 혼용 6 / cultural 6). SPEC-ADP-008 Naver suite의
       category 인벤토리와 1:1로 정렬되어 어댑터 회귀를 항목별로
       관찰 가능. research §2.
  (D3) **Evaluator pool = 최소 3 raters, Cohen's κ ≥ 0.6 게이트**.
       Landis-Koch 1977 기준의 "substantial agreement"를 채택.
       κ < 0.6이면 라운드는 무효 처리되고 캘리브레이션 세션이
       선행되어야 한다. research §5.
  (D4) **Naver-first ranking metric = top-3 recall@k for the
       `naver` adapter** (단일 SourceID, vertical은
       `naver_vertical` filter로 구분). M3 종료 조건("Naver
       results ranked first")의 측정 가능한 구현체. 합격선
       0.80(`expected_naver_relevant == true`인 쿼리의 80%
       이상에서 `naver` SourceID 결과 ≥1건이 상위 3위 내).
       MRR@10도 보조 지표로 기록하지만 게이트는 top-3 recall.
       (v0.2.0 정정: 이전 초안의 `naver-news`/`naver-blog`/
       `naver-shopping` 분리 어댑터 ID는 라이브 코드에 존재하지
       않으므로 단일 `naver` SourceID + vertical filter로 재정의.)
  (D5) **Code-mixing 처리 = SPEC-IR-001 `mixed` 분류**와 정합.
       6개 한영 혼용 쿼리는 별도 sub-bucket로 분리되어 mecab-ko
       세그멘트 정확도와 라우터 분류 일치도를 동시에 측정. research
       §6.
  (D6) **Tokenization regression baseline = mecab-ko**(tech.md
       2026-04-24 decision). konlpy/khaiii는 fallback으로 문서화
       되지만 baseline run에 포함되지 않는다. 토크나이저 교체로
       top-3 recall이 0.05 이상 하락하면 regression 발견.
  (D7) **CI integration posture = 비차단(artifact-only)**. 한국어
       eval은 manual·offline. CI는 매 릴리스마다 (a) 골든셋
       JSONL, (b) scoring sheet CSV 템플릿, (c) 직전 라운드의
       baseline 스냅샷을 아티팩트로 첨부한다. 분기별 1회 manual
       round로 회귀 점검. research §7.
  (D8) **Rater anonymization** — 골든셋 쿼리는 Naver DataLab의
       공개 트렌드 키워드 또는 합성 쿼리만 사용(사용자 PII 없음).
       Rater는 R1/R2/R3 익명 ID. research §8.

  Companion artifacts:
  - `.moai/specs/SPEC-EVAL-003/research.md` — Phase 0.5 research
    (한국어 NLP 평가 지형, Naver DataLab 분류, 수동 평가 프로토콜
    문헌, LLM-as-judge 한국어 한계, inter-rater agreement 통계,
    code-mixing 분포, Korean LLM 비교).
  - `.moai/specs/SPEC-EVAL-003/plan.md` — TDD 모드 phased plan.

  10 EARS REQs + 5 NFRs + 1 protocol 문서 + 1 golden set + 1
  scoring sheet 템플릿 + 1 kappa 계산기. Methodology: **TDD**
  (golden set 자체가 실패하는 테스트 코퍼스 — 각 쿼리는
  RED→GREEN 전환을 통해 M3 어댑터 체인의 한국어 우선 랭킹을
  강제한다). Coverage target 85%. Harness: standard. Owner:
  expert-testing.

---

## 1. Overview

SPEC-EVAL-003은 Universal Search V1의 **한국어 우선 정체성**을
회귀로부터 보호하는 평가 SPEC이다. M3 단계에서 SPEC-ADP-008(Naver
suite)·SPEC-ADP-009(KoreaNewsCrawler + Daum + Korean RSS)·
SPEC-IDX-003(mecab-ko 토크나이저)·SPEC-IR-001(korean 분류기)이 모두
구현되면 사용자가 한국어 쿼리를 입력했을 때 Naver suite 결과가
상위에 표시되어야 한다. 이 약속이 시간이 지나며 깨지지 않도록
50-query 골든셋과 분기 manual scoring round를 운영한다.

### 1.1 What ships

`.moai/specs/SPEC-EVAL-003/` 외에 다음 산출물이 추가된다:

```
internal/eval/korean/
├── loader.go               [골든셋 JSONL → in-memory struct 로더]
├── scoring.go              [top-3 recall, MRR@10, κ 계산기]
├── kappa.go                [Cohen's κ + Light's mean-κ (3-rater);
│                            Krippendorff α는 post-V1 보류]
├── snapshot.go             [release-tagged baseline JSON 직렬화]
└── *_test.go               [TDD: 골든셋 각 쿼리 = test case]

docs/eval/ko/
├── protocol.md             [수동 평가 프로토콜 (rater handbook)]
├── rubric.md               [5점 척도 anchor + Korean examples]
├── onboarding.md           [rater fluency 기준, 익명화 규칙]
└── kappa-interpretation.md [Landis-Koch 가이드 + threshold 정책]

tests/eval/korean/
├── golden-set.jsonl        [50 queries × 6 categories]
├── scoring-sheet-template.csv  [rater 입력 양식]
└── baseline-snapshots/
    └── v1.0.0.json         [최초 baseline]
```

### 1.2 Motivation

평가 SPEC이 없을 때의 위험:

- M3에서 한국어 우선 랭킹이 우연히 달성되더라도, 어댑터 교체나
  IR-001 임계값 변경 한 번으로 회귀가 발생해도 누구도 알지 못한다.
- product.md §6 SLO("manual eval ≥ 4/5")가 측정 가능한 구현체
  없이 약속에 머문다 — V1 ship 시 SLO 검증 불가.
- Korean LLM-as-judge로의 안일한 전환이 영어 편향 점수를 무비판
  적으로 통과시켜 한국어 사용자 경험을 침묵 속에서 악화시킨다.

평가 SPEC이 있을 때:

- 매 릴리스마다 골든셋 JSONL이 아티팩트로 첨부되어 회귀 시 진단
  자료가 즉시 확보된다.
- 분기별 3-rater round로 SLO를 정량적으로 검증한다 (top-3 recall
  ≥ 0.80, mean ranking score ≥ 4/5).
- LLM-as-judge는 future enhancement로 명시 보류되어, manual
  baseline이 자동화의 calibration 기준이 된다.

### 1.3 Forward-compatibility commitments

- **SPEC-IR-001**: 본 SPEC은 IR-001의 `korean`/`mixed` 분류를
  **소비**한다. 새 분류 카테고리가 추가되면 골든셋 분포도
  amendment로 갱신.
- **SPEC-ADP-008/009**: 실제 어댑터 SourceID를 정답
  `expected_sources` 필드에 사용한다 — Naver suite는 단일
  `naver`(vertical은 optional `expected_naver_vertical` ∈
  {blog, news, web, shop, datalab}로 표현), Korean news suite는
  단일 `koreanews`(라이브 코드 `naver.go:181` / `koreanews.go:85`).
  `naver-news`/`naver-blog`/`naver-shopping`/`naver-academic` 및
  `daum-news`/`korea-news-crawler` 같은 분리 ID는 코드에 존재하지
  않는다. 어댑터 SourceID 변경 시 골든셋 마이그레이션 필요.
- **SPEC-IDX-003**: mecab-ko 결과가 토큰 단위로 골든셋 정답
  doc_id에 영향을 미친다. IDX-003 토크나이저 정책이 바뀌면 D6
  per regression baseline.
- **SPEC-SYN-002**: synthesis citation faithfulness는 EVAL-001
  영역. 본 SPEC은 ranking·retrieval 단계만 평가.
- **SPEC-EVAL-001**(영문 자매): 동일한 golden-set JSONL 포맷을
  사용하지만 카테고리 분포·평가 프로토콜·CI 게이트가 다르다.
- **SPEC-EVAL-002**(어댑터 가용성): 별개 trace. EVAL-002는 어댑터
  성공률, EVAL-003은 결과 순위 품질.

### 1.4 Pinned architectural decisions

HISTORY의 8개 pinned decision은 §2 REQ를 구속하는 제약이다.
재논의되지 않는다.

---

## 2. EARS Requirements

### 2.1 Golden Set Module

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-EVAL-001** | Ubiquitous | The Korean golden-set SHALL be stored as a single JSONL file at `tests/eval/korean/golden-set.jsonl` containing exactly 50 query objects. Each object SHALL include the fields: `query_id` (`KR-{NNN}` zero-padded), `query_text` (Korean, optionally mixed with English for code bucket), `category` (one of `news`, `blog`, `shopping`, `academic-tech`, `code-mixed`, `cultural`), `expected_lang` (`ko` or `mixed`), `expected_router_class` (matches SPEC-IR-001 categories — `korean` or `mixed`), `expected_naver_relevant` (boolean — true when a `naver` SourceID result is expected in top-3), `expected_naver_vertical` (optional string — one of `blog`, `news`, `web`, `shop`, `datalab` per the live `naver_vertical` filter values; present only when `expected_naver_relevant == true` and the query targets a specific vertical), `expected_sources` (array of **real adapter SourceID** strings whose results SHOULD appear in top-10 — valid values are exactly the registered SourceIDs, e.g. `naver`, `koreanews`, `arxiv`, `github`; the legacy `naver-news`/`naver-blog`/`naver-shopping`/`naver-academic`/`daum-news`/`korea-news-crawler` strings are PROHIBITED), `notes` (optional rater hint, ≤ 200 chars). The category distribution SHALL be exactly: 12 news / 10 blog / 8 shopping / 8 academic-tech / 6 code-mixed / 6 cultural (per HISTORY D2). NOTE: the `academic-tech` bucket does NOT use a Naver vertical (Naver has no `academic` vertical in the live code); academic-tech queries target non-Naver SourceIDs (`arxiv`, `github`, …) and SHOULD set `expected_naver_relevant: false` unless a `naver` `blog`/`news` result is independently expected. | P0 | Schema validation test asserts file parses as 50 objects, category counts match exactly, all required fields populated, every `expected_sources` entry is a registered SourceID (zero phantom IDs), `expected_naver_vertical` (when present) ∈ {blog, news, web, shop, datalab}, no PII (per NFR-EVAL-004). |
| **REQ-EVAL-002** | Ubiquitous | The golden-set queries SHALL be sourced from Naver DataLab public trending keywords (last 90 days at curation time) OR synthetic queries authored by a native Korean speaker. The set SHALL NOT include any user-submitted query, log-extracted query, or query containing personally identifiable information (names, emails, phone numbers, account identifiers). Each query's provenance SHALL be recorded in `docs/eval/ko/golden-set-provenance.md` (DataLab URL + capture date OR "synthetic, authored 2026-MM-DD"). | P0 | Provenance document audit + grep for PII patterns (email regex, phone regex, common Korean name+rank patterns) returns zero matches. |

### 2.2 Scoring Protocol Module

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-EVAL-003** | Ubiquitous | The manual scoring sheet template SHALL be a single CSV file at `tests/eval/korean/scoring-sheet-template.csv` with the header row: `query_id, rater_id, ranking_score, source_relevance, code_switching_handling, tokenization_quality, top3_naver_hit, mrr_top10, notes`. `ranking_score`, `source_relevance`, `code_switching_handling`, and `tokenization_quality` SHALL each be integer 1–5 (5 = best); `top3_naver_hit` SHALL be boolean (`true`/`false`); `mrr_top10` SHALL be a float in `[0.0, 1.0]`. The rater workflow SHALL be documented in `docs/eval/ko/protocol.md` such that any qualified rater can complete a full 50-query round from the document alone (per NFR-EVAL-001). | P0 | CSV header lint test; protocol document inspection test asserts all required sections (rater handbook, scoring rubric pointer, kappa interpretation pointer, anonymization rules) are present. |
| **REQ-EVAL-004** | Event-Driven | WHEN a scoring round produces three or more independent rater sheets covering the same golden-set version, the system SHALL compute Cohen's κ for the `ranking_score` field pairwise across all rater pairs AND aggregate via Light's mean-κ for a single round-level statistic. The round SHALL be marked `valid` IF AND ONLY IF the mean-κ is ≥ 0.6 (substantial agreement per Landis-Koch 1977 per HISTORY D3). Rounds with mean-κ < 0.6 SHALL be marked `invalid` and trigger the rater calibration protocol per REQ-EVAL-009 before any baseline snapshot is taken. | P0 | `kappa.go` unit tests cover (a) three identical rater sheets → κ = 1.0, (b) random sheets → κ near 0.0, (c) realistic divergent sheets → κ in 0.3–0.5 range marked invalid; integration test asserts an `invalid` round does not produce a snapshot. |

### 2.3 Ranking Metrics Module

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-EVAL-005** | Ubiquitous | The Naver-first ranking metric SHALL be defined as **top-3 recall@k for the `naver` SourceID** (single registered adapter ID; verticals are NOT separate adapters — see live code `internal/adapters/naver/naver.go:181`). A golden-set query is *Naver-hit at k=3* when at least one of its top-3 results has `SourceID == "naver"` AND, when the query specifies `expected_naver_vertical`, that result's vertical matches (vertical is derived from the result `DocType` per the live mapping `DocTypePost→blog`, `DocTypeArticle→news`, `DocTypeOther→{web,shop,datalab}`; when `DocType` alone is ambiguous the metric falls back to SourceID-only match and records the vertical as `unverified`). The recall is computed over the subset of golden-set queries where `expected_naver_relevant == true` (per HISTORY D4): `recall@3 = (count of Naver-hit queries) / (count of expected_naver_relevant queries)`. The pass threshold for the M3-exit regression gate SHALL be **0.80** (at least 80% of Naver-relevant queries return a `naver` SourceID result in the top-3). MRR@10 SHALL be computed and recorded as a supplementary metric but SHALL NOT gate. | P0 | `scoring.go` unit test asserts top-3 recall computation correctness against a known fixture keyed on `SourceID == "naver"` (+ optional vertical-via-DocType match); baseline-run integration test asserts the M3 stack achieves recall ≥ 0.80 on the curated golden set. |
| **REQ-EVAL-006** | Ubiquitous | Top-k recall metrics SHALL be computed and reported per category bucket (news / blog / shopping / academic-tech / code-mixed / cultural), keyed on the `naver` SourceID per REQ-EVAL-005. The per-category recall SHALL be surfaced in the baseline snapshot JSON so that regressions are localizable to a single category. The aggregate (all-categories) top-3 Naver recall is the **only V1 gate** per REQ-EVAL-005; per-category recalls are **observational and NOT a V1 gate item**. (v0.2.0 scope reduction) The per-category 0.10 flagged-warning in the snapshot diff report is **observational only** — it surfaces a hint in the diff report but does NOT gate any build, PR, or release. Elaborate per-category drift alerting is deferred to post-V1. | P1 | Snapshot JSON schema test asserts a `per_category` map with all six buckets is populated. The 0.10 flagged-warning, if implemented, is purely informational; absence of the flagged-warning logic does NOT fail any V1 acceptance criterion. |
| **REQ-EVAL-007** | State-Driven | IF a golden-set query has `category == "code-mixed"`, THEN the scoring sheet `code_switching_handling` field SHALL be a required input (no nulls). The round-level summary SHALL report the mean code-switching score AND the percentage of code-mixed queries where SPEC-IR-001 correctly classified the query as `mixed` (router-classification-accuracy@code-mixed). The protocol document SHALL describe how raters judge code-switching handling (mecab-ko tokenization on Korean segments, English passthrough on English segments, no over-segmentation of common code identifiers per HISTORY D5). | P1 | Sheet validation test asserts code-mixed rows have non-null `code_switching_handling`; aggregator emits `router_class_accuracy_mixed` to the snapshot. |

### 2.4 Regression & Reproducibility Module

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-EVAL-008** | Ubiquitous | Each completed valid scoring round SHALL produce a baseline snapshot JSON at `tests/eval/korean/baseline-snapshots/{release-tag}.json` containing: `release_tag`, `round_date`, `rater_ids` (anonymous IDs only per HISTORY D8), `mean_kappa`, `top3_naver_recall` (aggregate + per-category), `mrr_top10`, `mean_ranking_score`, `router_class_accuracy_mixed`, `tokenizer_version` (mecab-ko version per HISTORY D6), `adapter_versions` (map of **real adapter SourceID → version pin**, keyed on the registered SourceIDs such as `naver`, `koreanews`, … from SPEC-ADP-008/009 — NOT the phantom `naver-news`/`daum-news`/`korea-news-crawler` strings), and `golden_set_sha256`. Snapshots SHALL be append-only — never modified after creation. The latest four release snapshots SHALL be retained in the repository (per NFR-EVAL-003). | P1 | Snapshot writer unit tests assert append-only + SHA256 determinism + every `adapter_versions` key is a registered SourceID; CI workflow asserts directory invariant (≤ 4 most-recent files retained; older auto-archived to `baseline-snapshots/archive/`). |
| **REQ-EVAL-009** | Event-Driven | (v0.2.0 scope reduction — minimal V1 path) WHEN a scoring round is marked `invalid` (mean-κ < 0.6 per REQ-EVAL-004), the **V1 calibration path SHALL be: discard the invalid round and re-run a fresh round from scratch on a new sheet** (invalid round → re-round). No baseline snapshot is produced from an invalid round; a snapshot is produced only once a re-round reaches mean-κ ≥ 0.6. The protocol document `docs/eval/ko/protocol.md` SHALL state this minimal re-round rule plainly. **DEFERRED to post-V1**: the elaborate calibration ceremony — (a) joint re-scoring of the 5 lowest-agreement queries, (b) rubric-anchor divergence discussion, (c) FROZEN rubric amendment review by SPEC owner, (d) a structured `docs/eval/ko/calibration-log.md` ledger — is NOT a V1 deliverable and SHALL NOT gate any V1 acceptance criterion. | P1 | Protocol document audit asserts the minimal "invalid round → re-round" rule is stated; integration test asserts an `invalid` round produces no snapshot and a re-round with mean-κ ≥ 0.6 does. The deferred calibration-log ceremony is out of V1 scope. |
| **REQ-EVAL-010** | Optional | WHERE Continuous Integration runs on a release tag commit, the CI workflow SHALL emit three artifacts: (a) the current `golden-set.jsonl` (immutable evidence of what was tested), (b) a fresh `scoring-sheet-template.csv` (rater-ready), (c) a `baseline-diff-report.md` comparing the most recent valid baseline snapshot against the previous one (top-3 recall delta, per-category recall delta, mean ranking score delta, flagged regressions). The workflow SHALL NOT fail the build on any metric delta — the Korean benchmark is non-blocking per HISTORY D7. | P2 | CI workflow file exists; smoke test asserts artifacts are uploaded on a release tag; no failure path is wired. |

---

## 3. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| **NFR-EVAL-001** | Protocol reproducibility | A qualified native Korean rater (NFR-EVAL-005 fluency criterion) SHALL be able to complete a full 50-query scoring round from `docs/eval/ko/protocol.md` + `docs/eval/ko/rubric.md` + the golden-set JSONL + the scoring-sheet CSV template **without consulting the SPEC author or any prior rater**. Reproducibility is gated by a dry-run test at every SPEC amendment: an independent reader of the protocol drafts a procedural summary; SPEC owner verifies the summary matches the intended workflow. |
| **NFR-EVAL-002** | Rater throughput estimate | The protocol document SHALL state an expected throughput estimate (50 queries per rater per round). This is a planning aid and recruitment baseline, **not** a gate — raters are not penalized for slower scoring. The estimate informs scheduling: a 3-rater round nominally consumes 150 query-scorings of effort. |
| **NFR-EVAL-003** | Artifact retention | The four most recent valid baseline snapshots SHALL be retained in `tests/eval/korean/baseline-snapshots/`. Older snapshots SHALL be moved to `baseline-snapshots/archive/` but never deleted. The golden-set JSONL itself is versioned via the file SHA256 in each snapshot — modifications to the golden set produce a new SHA256 and require a fresh full round. |
| **NFR-EVAL-004** | Query anonymization | The golden-set JSONL SHALL NOT contain any user PII. Sources are limited to (a) public Naver DataLab trending keywords, (b) public Korean RSS feed titles, (c) synthetic queries authored explicitly for this benchmark. CI runs a PII grep gate (email regex, Korean phone format `010-XXXX-XXXX`, common 3-character Korean surname+given-name patterns from a name allow/deny list) — zero matches required. |
| **NFR-EVAL-005** | Korean rater fluency | Raters SHALL be native or near-native Korean speakers with sufficient familiarity with Korean web culture to judge Naver vs Daum vs international source relevance. The onboarding document SHALL state this criterion plainly. Rater recruitment evidence SHALL be retained in `docs/eval/ko/rater-pool.md` (anonymous IDs + fluency self-attestation date only — no PII per NFR-EVAL-004). |

---

## 4. Exclusions (What NOT to Build)

[HARD] This SPEC explicitly excludes the following items. Each has a
known destination SPEC, rationale, or follow-up; this list prevents
scope creep into EVAL-003.

- **영문 citation faithfulness 평가** (DeepEval 자동 채점, 50-query
  영문 golden-set, ≥0.85 CI gate). → SPEC-EVAL-001 영역. EVAL-003은
  한국어 ranking·retrieval만 다루며 synthesis faithfulness는
  언어 무관하게 EVAL-001이 책임진다.

- **어댑터 가용성 / 7일 롤링 success rate 대시보드**. → SPEC-EVAL-002
  영역. EVAL-003 baseline snapshot에 `adapter_versions` 필드는
  포함되지만 가용성 메트릭 자체는 다루지 않는다.

- **자동화된 LLM-as-judge Korean 평가**. → Post-V1로 의도적 deferral
  (HISTORY D1). Korean LLM 평가자의 영어 편향과 토크나이저 불일치
  (research §4) 문제로 V1 단계에서는 manual baseline 확보가 우선.
  manual round가 4회 누적되면 향후 LLM-as-judge 캘리브레이션의
  ground truth로 재활용 가능.

- **synthesis 단계 한국어 출력 품질 평가**(번역 자연성, 문체, 존댓말
  일관성 등). → EVAL-001 citation faithfulness가 언어 무관 검사를
  수행. 별도의 generation-quality eval SPEC은 post-V1.

- **실시간 평가 대시보드 / Grafana 패널**. → 평가는 분기 manual
  round; 실시간 대시보드는 SPEC-EVAL-002의 어댑터 가용성 영역.
  EVAL-003 baseline snapshot은 정적 JSON 아티팩트로 충분.

- **사용자 쿼리 로그에서의 골든셋 추출**. → NFR-EVAL-004 PII
  금지 조항과 충돌. 골든셋은 공개 DataLab 키워드 + 합성 쿼리만
  사용 (REQ-EVAL-002).

- **MRR@10 게이트 운영**. → 보조 지표로 기록되지만 게이트는 top-3
  Naver recall 단일 지표 (REQ-EVAL-005). MRR을 게이트로 추가하면
  관측 가능한 회귀가 두 지표 사이에서 상쇄되어 진단이 어려워진다.

- **konlpy / khaiii 토크나이저 비교 baseline**. → 운영 baseline은
  mecab-ko 단일 (HISTORY D6). 다른 토크나이저는 docs에 fallback으
  로 문서화되지만 baseline run에 포함되지 않는다. 향후 토크나이저
  교체 시 별도 비교 SPEC.

- **번역된 영문 쿼리 평가** (예: 영문 "Naver news AI" → 한국어
  결과 기대). → router-classification@code-mixed에서 일부 다루지만
  본격적인 cross-lingual retrieval eval은 별도 SPEC. EVAL-003은
  한국어 native 또는 한영 mixed 쿼리만 다룬다.

- **GitHub Issue 추적** (`issue_number: 0`). → 프로젝트 M8 패턴
  유지.

- **Rater 보상 / 인센티브 정책**. → 운영 정책 문서이지 SPEC 영역
  아님. `docs/eval/ko/rater-pool.md`에 익명 ID와 fluency 자기
  확인만 기록.

- **Brand-voice 평가** (Korean 톤·매너). → `.moai/project/brand/`
  영역. EVAL-003은 retrieval/ranking 품질만 다룬다.

- **CI에서 manual eval round 강제 실행**. → manual은 분기 1회.
  CI는 artifact 산출 + diff report만 (REQ-EVAL-010, HISTORY D7).
  PR을 막지 않는다.

- **단일 rater round 허용** (κ 게이트 우회). → REQ-EVAL-004 최소
  3 raters를 우회하지 않는다. 캘리브레이션 후 재라운드.

- **SPEC-EVAL-001과의 golden-set JSONL 스키마 통합 작업**. →
  EVAL-003 V1은 자체 스키마를 사용하고, EVAL-001과의 통합은 별도
  refactor SPEC. V1 단계에서 두 스키마는 의도적으로 분리된다 —
  카테고리 분포·필수 필드·게이트 정책이 다르기 때문.

- **Krippendorff α 통계 산출** (v0.2.0 deferral). → V1 inter-rater
  게이트는 Cohen's κ + Light's mean-κ ≥ 0.6로 충분. ordinal α는
  보조 통계이며 post-V1. `kappa.go`는 V1에서 α를 구현하지 않는다.

- **정교한 calibration 의식** (v0.2.0 deferral, REQ-EVAL-009). →
  V1은 minimal path만(invalid round → re-round). 5-query joint
  re-score + rubric-anchor divergence 토의 + FROZEN rubric
  amendment review + `calibration-log.md` 구조화 ledger는 post-V1.

- **per-category 0.10 flagged-warning 게이트** (v0.2.0 deferral,
  REQ-EVAL-006). → per-category recall은 snapshot에 기록되지만
  observational. 0.10 drift 경고는 diff report의 informational
  hint일 뿐 어떤 build/PR/release도 막지 않는다. 정교한 drift
  alerting은 post-V1.

- **golden-set SHA drift 경고** (v0.2.0 deferral, EC-002) +
  **tokenizer version drift diff 주석** (v0.2.0 deferral, EC-003).
  → nice-to-have. V1 acceptance를 막지 않으며 post-V1 enhancement.

---

## 5. Acceptance Criteria

REQ별 acceptance 요약은 §2에 명시. 전체 Given-When-Then 시나리오는
별도 `acceptance.md`(plan 단계에서 작성)에서 다룬다. 시나리오 인덱
스:

| Scenario | Description | Coverage |
|----------|-------------|----------|
| §5.1 | Golden-set schema validation: `tests/eval/korean/golden-set.jsonl` 파일이 정확히 50개 객체를 가지며 카테고리 분포 (12/10/8/8/6/6)와 필수 필드를 모두 만족. PII grep 0 매치. | REQ-EVAL-001, REQ-EVAL-002, NFR-EVAL-004 |
| §5.2 | Manual scoring round 실행: 3 raters가 50 queries × 8 fields를 채운 sheet 3장을 생성. Light's mean-κ 계산 후 ≥ 0.6이면 valid 마킹, 아니면 invalid → re-round (V1 minimal path). | REQ-EVAL-003, REQ-EVAL-004 |
| §5.3 | M3 exit gate 통과: M3 implemented 시점의 baseline run이 top-3 Naver recall ≥ 0.80을 달성. 미달 시 ADP-008/IDX-003/IR-001 회귀 진단 트리거. | REQ-EVAL-005 |
| §5.4 | 카테고리별 회귀 감지: news 버킷 recall이 직전 snapshot 대비 0.10 이상 하락 → diff report에 flagged-warning 표시. | REQ-EVAL-006 |
| §5.5 | Code-mixed 쿼리 처리: 6개 한영 혼용 쿼리에 대해 IR-001이 `mixed` 분류 + mean code-switching-handling ≥ 4/5 + mecab-ko 한국어 segment 처리 평가. | REQ-EVAL-007 |
| §5.6 | Baseline snapshot 생성 + 보존: valid round 완료 시 `baseline-snapshots/{tag}.json` 생성. 최신 4개 보존, 5번째부터 archive 이동. | REQ-EVAL-008, NFR-EVAL-003 |
| §5.7 | Invalid round → re-round (V1 minimal path): 가짜 invalid round 시뮬레이션 → 라운드 폐기 → 새 sheet 발급 → re-round valid(κ ≥ 0.6) 시 snapshot. (정교한 calibration-log ceremony는 post-V1.) | REQ-EVAL-009 |
| §5.8 | CI artifact 산출: release tag commit → CI workflow가 golden-set + sheet template + diff-report 3종 업로드, 빌드 실패 없음. | REQ-EVAL-010, HISTORY D7 |
| §5.9 | Reproducibility dry-run: 신규 rater가 protocol·rubric·golden-set·template 4종 문서만으로 1 query를 스코어링하고 SPEC owner가 절차 일치 확인. | NFR-EVAL-001 |
| §5.10 | PII 회귀: 골든셋 PR에 PII 패턴을 의도적으로 추가 → CI PII grep gate가 실패 처리. | NFR-EVAL-004 |

---

## 6. Dependencies & Blocks

### 6.1 Upstream SPEC dependencies (depends_on)

- **SPEC-IR-001** (implemented) — `korean`/`mixed` 라우터 분류
  카테고리를 골든셋 `expected_router_class` 필드에서 소비. 라우터
  카테고리 추가/변경 시 골든셋 amendment 필요.

- **SPEC-ADP-008** (draft/implemented) — 단일 `naver` SourceID를
  `expected_sources` 및 top-3 Naver recall 계산에서 사용. vertical
  은 `naver_vertical` filter ∈ {blog, news, web, shop, datalab}로
  표현되며 별개 어댑터가 아니다(라이브 코드 `naver.go:181`).
  `naver-news`/`naver-blog`/`naver-shopping`/`naver-academic`
  분리 ID는 존재하지 않는다. SourceID 변경 시 골든셋 마이그레이션.

- **SPEC-ADP-009** (draft/implemented) — 단일 `koreanews` SourceID
  (RSS + KoreaNewsCrawler 합성 어댑터, 라이브 코드 `koreanews.go:85`)
  를 fallback 정답 소스로 사용. `daum-news`/`korea-news-crawler`
  분리 ID는 존재하지 않는다(Daum은 v0.1에서 stub/disabled).
  ADP-009 미구현 단계의 baseline run은 `naver` 단독 recall만 측정.

- **SPEC-IDX-003** (draft/implemented) — mecab-ko 토크나이저
  버전을 baseline snapshot `tokenizer_version` 필드에 기록. 토크
  나이저 정책 변경 시 D6 regression baseline 정책 적용.

- **SPEC-SYN-002** (implemented) — citation faithfulness는 EVAL-001
  영역이지만, retrieval 단계에서 누락된 doc_id가 synthesis로
  전파되는 회귀를 감지하기 위해 baseline snapshot에서 `doc_id`
  trace 가능 여부를 사이드 체크. SYN-002 인터페이스 의존.

### 6.2 Related but soft (related)

- **SPEC-EVAL-001** — 영문 자매. 동일 JSONL 포맷 컨벤션을 미래에
  통합할 수 있도록 필드 명명을 일관되게 유지(예: `query_id`,
  `expected_sources`). V1에서는 의도적으로 분리.

- **SPEC-EVAL-002** — 어댑터 가용성. baseline snapshot의
  `adapter_versions` 필드는 EVAL-002 데이터와 교차 참조 가능.

### 6.3 Downstream blocked SPECs (blocks)

- **SPEC-REL-001** (M9 V1 release) — V1 ship 전 최소 1회 valid
  baseline round 통과 (top-3 Naver recall ≥ 0.80) 필요. product.md
  §6 SLO 보증의 measurable evidence.

### 6.4 External dependencies (run-phase pins)

- mecab-ko 사전 (≥ mecab-ko-dic 2.1.1) — sidecar 서비스에 설치.
- Naver DataLab 공개 트렌드 페이지 — 골든셋 큐레이션 시점의 90일
  창 스냅샷 (provenance 문서에 capture date 기록).
- 3 native Korean raters (NFR-EVAL-005 fluency 기준 충족).

신규 Go 의존성: 없음 (stdlib `encoding/json`, `encoding/csv`,
`crypto/sha256`만 사용).

---

## 7. Files to Create / Modify

### 7.1 Created (final list owned by run phase)

| Marker | Path | Purpose |
|--------|------|---------|
| [NEW] | `internal/eval/korean/loader.go` | golden-set JSONL → struct 로더. |
| [NEW] | `internal/eval/korean/scoring.go` | top-3 recall + MRR@10 계산. |
| [NEW] | `internal/eval/korean/kappa.go` | Cohen's κ + Light's mean-κ. (v0.2.0: Krippendorff α 보조 출력은 post-V1로 보류 — V1 inter-rater 게이트는 Cohen/Light κ로 충분.) |
| [NEW] | `internal/eval/korean/snapshot.go` | baseline snapshot JSON 직렬화 + 보존 정책. |
| [NEW] | `internal/eval/korean/*_test.go` | TDD: 골든셋 쿼리당 test case + fixture rounds. |
| [NEW] | `docs/eval/ko/protocol.md` | rater handbook (REQ-EVAL-003 NFR-EVAL-001 핵심 문서). |
| [NEW] | `docs/eval/ko/rubric.md` | 5점 척도 anchor + Korean examples (REQ-EVAL-009 amendment 대상). |
| [NEW] | `docs/eval/ko/onboarding.md` | NFR-EVAL-005 fluency 기준 + 익명화 규칙. |
| [NEW] | `docs/eval/ko/kappa-interpretation.md` | Landis-Koch 표 + threshold 정책. |
| [NEW] | `docs/eval/ko/golden-set-provenance.md` | 쿼리별 출처 + capture date. |
| [DEFERRED post-V1] | `docs/eval/ko/calibration-log.md` | invalid round 처리 이력 ledger. v0.2.0: V1은 protocol.md의 minimal "invalid round → re-round" 규칙으로 충분; 구조화 ledger는 post-V1. |
| [NEW] | `docs/eval/ko/rater-pool.md` | rater 익명 ID + fluency self-attestation. |
| [NEW] | `tests/eval/korean/golden-set.jsonl` | 50 queries × 6 categories (REQ-EVAL-001). |
| [NEW] | `tests/eval/korean/scoring-sheet-template.csv` | rater 입력 양식 (REQ-EVAL-003). |
| [NEW] | `tests/eval/korean/baseline-snapshots/v1.0.0.json` | 최초 baseline (V1 release 시점). |
| [NEW] | `.github/workflows/korean-eval.yml` | CI artifact 산출 워크플로 (REQ-EVAL-010). |

### 7.2 Modified (parent repo)

| Path | Change |
|------|--------|
| `.moai/specs/SPEC-EVAL-001/spec.md` | (필요 시) cross-reference 추가 — "EVAL-003은 한국어 ranking, 본 SPEC은 영문 citation faithfulness". Amendment OK, 게이트 변경 금지. |
| `README.md` | 평가 섹션에 "Korean-locale benchmark — 분기 manual round" 한 문단 추가. |

### 7.3 Existing — Unchanged

- `internal/router/`, `internal/adapters/`, `internal/index/` — 모두
  read-only 소비. 평가는 외부 관찰자 입장.
- `cmd/usearch*` — touch 없음.

---

## 8. Open Questions

본 SPEC의 `_TBD_`는 research §10에 모인다. plan-auditor 편의를 위해
재정리:

1. **분기 1회 vs 월 1회 manual round 빈도** — V1 초안은 분기 1회로
   고정. M8 운영 데이터 누적 후 재평가. **plan-auditor PASS 무관.**

2. **rater 1인이 여러 라운드 참여 시 학습 효과 통제** — 동일 rater
   가 같은 골든셋을 반복 스코어링할 때 메모리 효과가 κ를 인위적
   으로 올릴 가능성. _TBD_ — 4 라운드까지는 동일 rater 허용, 5라운
   드부터는 rater 교체 권고. plan에서 정책 확정.

3. **Naver-suite 어댑터 ID 안정성** — ADP-008이 어댑터 ID를 V1.x
   동안 변경할 가능성. _TBD_ — ADP-008 SPEC owner와 V1 freeze 합의
   필요.

4. **code-mixed 6 쿼리의 한영 비율** — 7:3 / 5:5 / 3:7 중 어느 분포
   가 실사용을 대표하는지 데이터 없음. _TBD_ — research §6에서
   discussion하고 plan annotation에서 확정. 초안은 5:5 4개 + 7:3
   2개로 잠정.

5. **mecab-ko-dic 버전 회귀 정책** — 사전 업데이트로 top-3 recall이
   0.05 이상 변동하면 baseline 재계산 vs warning. _TBD_ — D6
   regression baseline 정책의 상세화. plan 단계.

6. **κ 게이트 0.6 vs 0.7** — Landis-Koch substantial(0.6)이 V1 기본.
   research에서 검색 relevance 영역의 일반 임계값을 검토. _TBD_ —
   research 결론 후 plan에서 확정.

이 항목들은 plan-auditor PASS를 막지 않는 known unresolved scope
edges.

---

## 9. References

External (research.md §12에서 인용):

- Landis JR, Koch GG. "The measurement of observer agreement for
  categorical data." Biometrics 1977
  (https://www.jstor.org/stable/2529310)
- Krippendorff K. "Content Analysis: An Introduction to Its
  Methodology" 4th ed. — α statistic for ordinal data
- TREC Korean track historical reports (NIST TREC archive)
- KMMLU benchmark — Korean LM eval suite
  (https://huggingface.co/datasets/HAERAE-HUB/KMMLU)
- HAERAE Bench — Korean cultural knowledge
  (https://huggingface.co/datasets/HAERAE-HUB/HAE_RAE_BENCH)
- KoBEST — Korean balanced eval suite
  (https://github.com/SKT-LSL/KoBEST_datarepo)
- Naver DataLab — 공개 트렌드 키워드
  (https://datalab.naver.com/)
- mecab-ko-dic upstream — Korean morphological analyzer dictionary
  (https://bitbucket.org/eunjeon/mecab-ko-dic)

Internal (project files):

- `.moai/project/product.md` §6 ("Korean-locale result ranking
  relevance | manual eval ≥ 4/5 on 50-query benchmark")
- `.moai/project/roadmap.md` §M8 (SPEC-EVAL-003 행) + §5 (M3 종료
  조건 "Naver results ranked first") + §3 (M8 parallelization)
- `.moai/project/tech.md` §3 (Korean tokenizer mecab-ko) + §4
  (Naver / Daum / KoreaNewsCrawler 어댑터 인벤토리)
- `.moai/specs/SPEC-IR-001/spec.md` — `korean`/`mixed` 라우터
  카테고리 정의
- `.moai/specs/SPEC-ADP-008/spec.md` — Naver suite 어댑터 ID
  카탈로그
- `.moai/specs/SPEC-ADP-009/spec.md` — Daum / KoreaNewsCrawler /
  RSS 어댑터 카탈로그
- `.moai/specs/SPEC-IDX-003/spec.md` — mecab-ko 토크나이저 정책
- `.moai/specs/SPEC-SYN-002/spec.md` — citation faithfulness 인터
  페이스 (사이드 체크 의존)
- `.moai/specs/SPEC-EVAL-001/spec.md` — 영문 자매 SPEC (JSONL 포맷
  conventions 일관성)

---

*End of SPEC-EVAL-003 v0.2.0 (draft).*
