# SPEC-DOC-002 Plan — phased implementation

Status: draft companion to spec.md
Author: limbowl via manager-spec
Date: 2026-05-22
Methodology: **DDD** (ANALYZE-PRESERVE-IMPROVE per `.claude/rules/
moai/workflow/workflow-modes.md`). DDD-mode justification: 본 SPEC은
10개 production 어댑터의 **이미 작동하는 행위를 describe**하는
작업이 본질이다. `internal/adapters/{reddit,hn,arxiv,github,youtube,
social,searxng,naver,koreanews}/*.go` 는 모두 SPEC-ADP-001..009로
이미 ship되었고 acceptance test를 통과한다. 본 SPEC은 (a) ANALYZE
existing surface (research.md §1 inventory + Capabilities 추출 +
error envelope 매핑 + Korean-locale 특수 surface 식별), (b) PRESERVE
어댑터의 behavior를 **묘사 충실도**로 보장 (만약 페이지 초안이
operator에게 잘못된 mental model을 주면 페이지를 수정 — 코드 수정
없음; 코드 버그 발견 시 별도 SPEC), (c) IMPROVE는 새 MDX content
+ 드리프트 CI + 상태 배지 + Korean-locale operational notes의
도입만. 코드는 unchanged (REQ-DOC-002 7.3 invariant). 신규
narrative 콘텐츠 (각 어댑터의 Troubleshooting 5-field 항목,
Setup section의 Korean-locale 3-line summary) 는 TDD 하위 cycle이
부적합 — manager-docs + native reviewer 검수 + adapter-page-
completeness gate + lychee link-check + 드리프트 gate가 quality
보장. 새 Go 프로그램 (`tools/gen-adapter-ref/`) + 4개 shell scripts
+ 3개 React 컴포넌트는 TDD 하위 cycle로 실행 (RED test → GREEN
implementation → REFACTOR).

Coverage target: 85% (per spec.md frontmatter) applies to:
- `tools/gen-adapter-ref/` Go 프로그램 (AST 추출 로직)
- 4개 shell scripts (`gen-adapter-reference.sh`,
  `check-adapter-page-completeness.sh`, `check-doc-credentials.sh`,
  + the modified `check-bilingual-coverage.sh`)
- 3개 MDX components (`StatusBadge.tsx`, `CapabilitiesTable.tsx`,
  `AdapterCatalog.tsx`)

MDX 콘텐츠는 coverage 측정 대상이 아닌 "완성도 percentage" 게이트
(REQ-ADPDOC-002 10-section 강제 + NFR-ADPDOC-004 ≥ 50자 plain text
per section + REQ-ADPDOC-014 troubleshooting entry count + REQ-
ADPDOC-016 cross-link count).

Harness: **standard** (per `.moai/config/sections/harness.yaml`
auto-routing — P1 docs SPEC, no security domain involvement,
adapter behaviour는 unchanged; Sprint Contract RECOMMENDED but
NOT required per `.claude/rules/moai/design/constitution.md` §11).
17 EARS REQs + 6 NFRs + 12 EN MDX + 4 KO MDX + 1 Go program + 3
MDX components + 4 shell scripts + 2 CI workflow modifications +
1 modified bilingual-coverage script (DOC-001 coordination
required).

본 plan은 SPEC-DOC-002 구현을 priority-ordered phases로 sequence
한다. `.claude/rules/moai/core/agent-common-protocol.md` 시간 예측
금지 — phase는 priority + ordering만 사용.

---

## 1. Implementation principle

본 SPEC의 plan philosophy 6축:

1. **DOC-001 PASS first** — SPEC-DOC-002 run-phase는 SPEC-DOC-001
   run-phase가 완료된 후에만 시작 가능. DOC-001이 ship한 Nextra
   v4 app + `theme.config.tsx` + `lychee.toml` + `docs.yml`
   workflow + bilingual-coverage 스크립트 + `reference/adapters/
   index.mdx` placeholder가 모두 존재해야 본 SPEC의 file
   modification + new file 작성이 의미를 가짐. plan-auditor는
   DOC-001 PASS 확인 후 DOC-002 plan을 unlock.

2. **Drift CI infrastructure before content** — `tools/gen-
   adapter-ref/` Go 프로그램 + `scripts/gen-adapter-reference.sh`
   + `_generated/*.capabilities.json` 베이스라인 + CI job 활성화
   가 Phase 2에서 완료된 후, Phase 3-4의 MDX 콘텐츠가 `<Capabilities
   Table>` 컴포넌트로 JSON을 import 가능. 인프라가 콘텐츠 의존성
   순서.

3. **Status badge data feed parallel** — `<StatusBadge>` 컴포넌트
   + JSON Schema + EVAL-002 dashboard amendment 협의는 Phase 2와
   parallel. EVAL-002 export job이 아직 실장되지 않은 경우 (open
   question §8.4 → likely scenario) Phase 2 끝에 static 초기
   `adapter-status.json` 작성 → Phase 5 정도에서 EVAL-002 cron
   교체. blocking이 아닌 graceful degradation.

4. **EN-first content authoring, KO Tier-1 parallel** — Phase 3
   (auto-generated infra-light pages: reddit, hn, arxiv,
   youtube, searxng — no auth, no Korean-locale) 먼저 → Phase 4
   (auth-bearing: github + naver — Korean-locale specific
   prerequisites는 naver 부분에서만) → Phase 5 (Korean-locale
   heavy: koreanews + KO Tier-1 4-page translation). Bluesky + X
   는 social shared package 때문에 Phase 3-4 boundary에서 동시
   작성 (REQ-ADPDOC-009 shared-implementation callout 일관성).

5. **Credential placeholder lint baseline early** — `scripts/
   check-doc-credentials.sh` (REQ-ADPDOC-018) 가 Phase 2 끝에
   활성화되어 Phase 3-5의 모든 MDX 작성이 clean baseline 유지.
   PR마다 lint 통과 강제 (SPEC-SEC-001 D2 gitleaks pre-commit
   hook가 commit-time, DOC-002 lint가 PR-time — defense in
   depth).

6. **DOC-001 coordination late but explicit** — `scripts/check-
   bilingual-coverage.sh` 의 exclude pattern 확장 (REQ-ADPDOC-017)
   은 SPEC-DOC-001 owner 권한 영역. 본 plan의 Phase 6 (마지막)
   에서 DOC-001 owner 사인오프 + script modification + CI 재실행.
   너무 일찍 시작하면 DOC-001 run-phase와 conflict; 너무 늦으면
   KO Tier-1 검증 불가. Phase 6가 sweet spot.

---

## 2. Sprint Contract (RECOMMENDED per standard harness)

Sprint Contract는 builder (manager-ddd) ↔ evaluator-active 사이
협상 결과로 매 GAN Loop iteration 시작 전 작성. 본 SPEC의 V1 Sprint
Contract draft (run phase에서 evaluator-active와 finalize):

### Acceptance checklist (testable per iteration)

- [ ] `tools/gen-adapter-ref/` Go 프로그램 + 단위 테스트 (85%+
      coverage; 10개 어댑터에 대한 골든 테스트 모두 통과)
- [ ] `scripts/gen-adapter-reference.sh` 실행 시 10개 `_generated/
      {adapter}.capabilities.json` 생성, 각 JSON이 schema 준수
- [ ] CI `gen-adapter-ref-drift` job 활성화 + green baseline
- [ ] `<CapabilitiesTable>`, `<StatusBadge>`, `<AdapterCatalog>`
      React 컴포넌트 + 단위 테스트 (taxonomy boundary cases,
      filter logic, fallback rendering)
- [ ] 12개 EN MDX 페이지 작성 — 모두 10-section 강제 통과
- [ ] 4개 KO Tier-1 MDX 페이지 + native reviewer 검수 기록
      (`docs/content/ko/CONTRIBUTING.md`)
- [ ] CI `adapter-page-completeness` + `adapter-status-staleness`
      + `check-doc-credentials` job 활성화 + green baseline
- [ ] `docs/lychee.toml` 에 NFR-ADPDOC-005 provider URL allowlist
      추가, internal links 100% 해결
- [ ] `scripts/check-bilingual-coverage.sh` exclude pattern 확장
      (DOC-001 owner sign-off + 본 SPEC의 4 KO Tier-1 pages 포함
      + 8 Tier-2 EN-only pages 제외)
- [ ] `docs/content/en/end-users/surface-comparison.mdx` +
      `docs/content/en/operators/deployment-helm.mdx` 에 본 SPEC의
      페이지로 향하는 cross-link 추가 (DOC-001 페이지 modification,
      DOC-001 owner 협의)
- [ ] 어댑터 status badge data: EVAL-002 dashboard export job 실장
      OR static 초기 `adapter-status.json` 작성 (open question
      §8.4 resolution)

### Priority dimension

**Completeness** (evaluator-active 4-dimension scoring) — 본 SPEC의
quality는 16개 MDX 페이지 완비 + 드리프트 CI + 상태 배지 작동의
함수. Originality는 N/A (DOC-002는 industry-standard reference doc
pattern 적용, research §2). Functionality는 docs site의 navigation
+ 검색 + 페이지 렌더링 동작 + CI gate 작동. Craft (코드 품질)는
`tools/gen-adapter-ref/` Go 프로그램 + 4개 shell scripts + 3개
React 컴포넌트 한정 (콘텐츠는 craft 측정 대상 아님; completeness
가 cover).

### Test scenarios (integration)

- §5.1 EN page filename = SourceID (REQ-ADPDOC-001)
- §5.5 StatusBadge boundary cases (REQ-ADPDOC-005)
- §5.7 Drift CI gate trigger (REQ-ADPDOC-007)
- §5.10 Auth-required Setup section completeness (REQ-ADPDOC-010)
- §5.11 Korean-locale Setup section non-duplication (REQ-ADPDOC-011)
- §5.17 KO Tier-1 coverage gate (REQ-ADPDOC-017)
- §5.18 Credential placeholder lint (REQ-ADPDOC-018)

Sprint Contract iteration boundary: 각 Phase 끝에서 자동 평가;
incomplete 항목은 다음 iteration으로 carry forward (no regression
allowed per `constitution.md` §11.4 contract evolution rule).

---

## 3. Phased plan (priority-ordered, no time estimates)

### Phase 0 — Plan-auditor + DOC-001 PASS prerequisite gate

**Goal**: SPEC-DOC-002 run-phase 시작 전 필수 외부 조건 충족 확인.

**Activities**:

- plan-auditor가 본 plan + spec.md + research.md 를 read-only
  으로 검토하고 8개 open question에 대한 user 답변 collect
  (AskUserQuestion via MoAI orchestrator):
  - §8.1 Korean-tokenizer cross-link only (D6 confirmation)
  - §8.2 Status badge taxonomy ↔ EVAL-002 lifecycle field alignment
  - §8.3 Bluesky vs X page split (D3 confirmation)
  - §8.4 EVAL-002 dashboard export job timing
  - §8.5 `check-bilingual-coverage.sh` exclude pattern extension
        approval from DOC-001 owner
  - §8.6 `tools/gen-adapter-ref/` location convention
  - §8.7 Provider doc URL per-locale strategy
  - §8.8 Page completeness threshold (50 vs 100 vs 200 chars)
- SPEC-DOC-001 run-phase status 확인 — `git log -- .moai/specs/
  SPEC-DOC-001/` 가 implementation evidence (e.g.,
  `feat(docs): add SPEC-DOC-001 Nextra v4 docs site`)를 보여주는지
  확인. PASS되지 않은 경우 본 SPEC은 plan-only 상태 유지.
- Reviewer pool 확인 (NFR-ADPDOC-006): Korean Tier-1 4 페이지에
  대한 native-Korean reviewer 1명 confirmed (SPEC-DOC-001 §8.2 의
  reviewer pool 재사용 — 새 commitment 없음).

**Gate**: All 8 open questions answered + DOC-001 PASS + Korean
reviewer confirmed → Phase 1 unlock. 미충족 항목은 plan-auditor
report에 blocker로 escalate.

**Deliverables**: plan-auditor PASS report at `.moai/reports/
plan-audit-SPEC-DOC-002-{DATE}/`.

---

### Phase 1 — DDD ANALYZE (codebase + DOC-001 surface inventory)

**Goal**: 10개 어댑터의 실제 surface와 DOC-001 site infrastructure 의
intersection 을 detailed map으로 inventory.

**Activities**:

- 10개 어댑터의 `Capabilities()` AST 정확한 line number 확인
  (`grep -n "Capabilities()" internal/adapters/*/[!_]*.go`).
- 10개 어댑터의 `client.go` `categorizeStatus`-style 함수의 정확한
  status code 매핑 enumerate.
- 10개 어댑터의 `internal/adapters/*/research.md` 확보 (이미
  존재; SPEC-ADP-* run-phase의 산출물) → Troubleshooting entries
  source 식별.
- DOC-001 ship 상태 inventory:
  - `docs/theme.config.tsx` (DOC-001) 의 MDX components 등록
    포인트 확인
  - `docs/lychee.toml` 의 현재 allowlist 확인 (NFR-ADPDOC-005 추가
    필요 entries 식별)
  - `docs/content/en/reference/adapters/index.mdx` placeholder 의
    내용 확인 (replace target)
  - `docs/content/en/end-users/surface-comparison.mdx` (DOC-001)
    의 cross-link insertion point 식별
  - `docs/content/en/operators/deployment-helm.mdx` (DOC-001) 의
    anchor target 식별
  - `scripts/check-bilingual-coverage.sh` (DOC-001) 의 현재
    exclude 패턴 logic 확인 → REQ-ADPDOC-017 extension scope
    계산
  - `docs/content/ko/CONTRIBUTING.md` (DOC-001) reviewer log
    엔트리 포맷 확인
- SPEC-EVAL-002 dashboard 의 `lifecycle` field 현재 schema 확인
  → DOC-002 D5 taxonomy 와 alignment delta 측정.

**Files read (read-only)**:

- `internal/adapters/{reddit,hn,arxiv,github,youtube,social,
  searxng,naver,koreanews,noop}/*.go` (production source)
- `internal/adapters/registry.go` (wrapper layer reference)
- `pkg/types/errors.go` (Category enum source)
- `.moai/specs/SPEC-ADP-001..009/{spec,research}.md` (per-adapter
  contracts + failure modes)
- `.moai/specs/SPEC-DOC-001/{spec,acceptance}.md` (site infra
  contract)
- `.moai/specs/SPEC-EVAL-002/spec.md` (dashboard contract)
- `docs/theme.config.tsx`, `docs/lychee.toml`,
  `docs/content/en/reference/adapters/index.mdx`,
  `docs/content/en/end-users/surface-comparison.mdx`,
  `docs/content/en/operators/deployment-helm.mdx`,
  `scripts/check-bilingual-coverage.sh`,
  `docs/content/ko/CONTRIBUTING.md` (all DOC-001 ship state)

**Deliverables**: `.moai/specs/SPEC-DOC-002/analyze-report.md`
documenting:

- Per-adapter exact Capabilities line numbers (10 entries)
- Per-adapter status code rosetta seed (REQ-ADPDOC-013 source)
- Per-adapter Troubleshooting seed entries (REQ-ADPDOC-014 source,
  ≥ 3 per adapter mined from research.md)
- DOC-001 surface intersection map (8 modification points)
- EVAL-002 dashboard schema delta (DOC-002 D5 alignment plan)

**Gate**: Analyze report reviewed → Phase 2 unlock.

---

### Phase 2 — Drift CI infrastructure (PRESERVE foundation)

**Goal**: 콘텐츠 작성 시작 전, 드리프트 게이트 인프라 + lint 인프라
+ 컴포넌트 인프라 완성.

**Activities** (TDD sub-cycle for code; RED → GREEN → REFACTOR):

#### 2.1 `tools/gen-adapter-ref/` Go program

- RED: `tools/gen-adapter-ref/extract_test.go` writes failing
  tests for AST extraction:
  - Golden test: parse `tools/gen-adapter-ref/testdata/
    fixture-adapter.go` → assert extracted JSON matches
    `testdata/fixture-adapter.expected.json`
  - Edge case: nil `AuthEnvVars` slice → JSON `"authEnvVars":
    null`
  - Edge case: non-literal `RateLimitPerMin` (e.g., `const`
    reference) → fail with diagnostic error pointing at line
  - Edge case: missing `Capabilities()` method → skip with
    informational log
- GREEN: minimal `tools/gen-adapter-ref/main.go` + `extract.go`
  implementing `go/parser` AST walk per REQ-ADPDOC-007 schema.
- REFACTOR: extract `parseStructLiteral`, `findCapabilitiesMethod`,
  `emitJSON` into separate testable functions; achieve 85%+
  coverage.

#### 2.2 `scripts/gen-adapter-reference.sh` shell wrapper

- RED: shellcheck + integration test (run script against
  fixture adapter dir → assert expected JSON output).
- GREEN: shell wrapper invoking `go run ./tools/gen-adapter-
  ref/` with adapter dir + output dir args. Mirrors SPEC-DOC-001
  `gen-cli-reference.sh` style.
- REFACTOR: add `--check` mode (no-write, diff-only) for CI use.

#### 2.3 Baseline `_generated/*.capabilities.json` (10 files)

- Run `scripts/gen-adapter-reference.sh` against current
  `internal/adapters/` → commit 10 JSON files to
  `docs/content/en/reference/adapters/_generated/`.
- Each JSON validated against schema (sourceID matches
  adapter package name, etc.).

#### 2.4 `scripts/check-doc-credentials.sh` lint script

- RED: test fixtures with both clean placeholders and
  realistic-shaped credentials → assert exit code 0 for clean,
  non-zero for tainted.
- GREEN: regex set covering REQ-ADPDOC-018 pattern list
  (intentionally aligned with `.gitleaks.toml` per D8 — copy
  the SEC-001 D2 rules + add docs-specific MDX patterns).
- REFACTOR: extract regex set to companion `.docs-credentials
  -patterns.toml` for shared maintenance with SEC-001 D2.

#### 2.5 MDX components

- RED: `docs/components/StatusBadge.test.tsx` writes failing
  tests:
  - Renders `stable` for `successRate7d=0.950, lifecycle="stable"`
  - Renders `beta` for `successRate7d=0.949, lifecycle="beta"`
  - Renders fallback for missing required field
- GREEN: `StatusBadge.tsx` minimal implementation reading
  build-time JSON import.
- REFACTOR: extract color mapping + accessibility attributes
  (ARIA labels per SPEC-DOC-001 REQ-DOC-017 WCAG 2.1 AA).
- Repeat RED/GREEN/REFACTOR for `CapabilitiesTable.tsx` (renders
  5 fields + source footer) and `AdapterCatalog.tsx`
  (filterable + sortable table with category filter).

#### 2.6 CI workflow additions

- Modify `.github/workflows/docs.yml`:
  - Add `gen-adapter-ref-drift` job — `scripts/gen-adapter-
    reference.sh --check` against committed `_generated/`
  - Add `adapter-page-completeness` job — `scripts/check-
    adapter-page-completeness.sh` (Phase 3 deliverable; can
    no-op pass until adapter pages exist)
  - Add `adapter-status-staleness` job — mtime check on
    `_generated/adapter-status.json`
  - Add `check-doc-credentials` job
- Initial CI run validates all jobs green on baseline.

**Files created**:

- `tools/gen-adapter-ref/main.go`, `extract.go`, `extract_test.go`,
  `main_test.go`
- `tools/gen-adapter-ref/testdata/fixture-adapter.go`,
  `fixture-adapter.expected.json`, additional edge case fixtures
- `scripts/gen-adapter-reference.sh`
- `scripts/check-doc-credentials.sh`
- `.docs-credentials-patterns.toml` (shared with SEC-001 D2)
- `scripts/check-adapter-page-completeness.sh` (initial scaffold
  no-op; full implementation Phase 3)
- `docs/components/StatusBadge.tsx`, `StatusBadge.test.tsx`
- `docs/components/CapabilitiesTable.tsx`,
  `CapabilitiesTable.test.tsx`
- `docs/components/AdapterCatalog.tsx`, `AdapterCatalog.test.tsx`
- 10× `docs/content/en/reference/adapters/_generated/{adapter}.
  capabilities.json` (baseline)
- `docs/content/en/reference/adapters/_generated/adapter-status.
  schema.json` (JSON Schema definition)

**Files modified**:

- `.github/workflows/docs.yml` (5 new jobs added or scaffolded)
- `docs/theme.config.tsx` (DOC-001 ownership — register 3 MDX
  components; coordinate with DOC-001 owner)
- `docs/lychee.toml` (NFR-ADPDOC-005 allowlist entries added)

**Gate**: `tools/gen-adapter-ref/` 85%+ coverage; all 3 React
components unit tests pass; CI `gen-adapter-ref-drift` +
`check-doc-credentials` jobs green; baseline `_generated/` JSON
files committed → Phase 3 unlock.

---

### Phase 3 — EN content authoring: no-auth, non-Korean adapters (5 pages)

**Goal**: Lowest-risk page batch (reddit, hn, arxiv, youtube,
searxng) 작성으로 template pattern stabilization.

**Activities** (manager-docs agent authoring each page via
file-by-file iteration):

#### 3.1 Per-page authoring (5 EN pages)

For each of `reddit.mdx`, `hn.mdx`, `arxiv.mdx`, `youtube.mdx`,
`searxng.mdx`:

- Frontmatter: `lastVerified: 2026-05-22`, `category`, `lifecycle`
  (from EVAL-002 dashboard read OR static fallback).
- Section 1 (Status & Compatibility): `<StatusBadge>` + SPEC-ADP-*
  cite + source path link + frontmatter lastVerified surface.
- Section 2 (Overview): 1-paragraph upstream provider + use case
  (sourced from SPEC-ADP-* §1.1 / §1.2 narratives).
- Section 3 (Setup): "Authentication: not required — public
  endpoint" formulation + 1-sentence access tier explanation
  (e.g., reddit unauth tier, hn Algolia free tier, arxiv public
  API, youtube cookie-scrape mode, searxng self-hosted).
- Section 4 (Capabilities): `<CapabilitiesTable src="_generated/
  {adapter}.capabilities.json" />`.
- Section 5 (Query syntax): adapter-specific query translation
  (e.g., reddit `/search.json?q=`, arxiv API syntax, hn algolia
  query DSL).
- Section 6 (Rate limits): 4-element format per REQ-ADPDOC-012.
- Section 7 (Error reference): cross-link `errors.mdx` + adapter-
  specific status rosetta from Phase 1 analyze-report.md.
- Section 8 (Troubleshooting): ≥ 3 entries from Phase 1 source
  inventory.
- Section 9 (Version compatibility): table with at least 1 row
  dated within 90 days of 2026-05-22.
- Section 10 (Related): ≥ 4 cross-links per REQ-ADPDOC-016.

#### 3.2 `errors.mdx` shared reference page

- 5 H3 subsections (one per Category) per REQ-ADPDOC-004 contract.
- Each subsection: HTTP status codes + fanout retry semantics
  (FAN-001 cross-link) + RetryAfter handling + real example
  message.

#### 3.3 `_meta.json` sidebar ordering

- `docs/content/en/reference/adapters/_meta.json` with key order:
  `index, reddit, hn, arxiv, github, youtube, bluesky, x,
  searxng, naver, koreanews, errors`.

#### 3.4 `scripts/check-adapter-page-completeness.sh` full impl

- Phase 2 scaffold replaced with full implementation:
  - Parse each `*.mdx` file (excluding `_generated/`, `_meta.json`,
    `index.mdx`, `errors.mdx`)
  - Assert exactly 10 H2 headings in prescribed order
  - Assert ≥ 50 chars plain text per section (NFR-ADPDOC-004
    threshold; finalized in Phase 0 §8.8)
  - Assert filename = SourceID from `_generated/{adapter}.
    capabilities.json`
- CI `adapter-page-completeness` job activates and passes against
  the 5 Phase 3 pages + Phase 2 baseline.

**Files created**:

- `docs/content/en/reference/adapters/reddit.mdx`
- `docs/content/en/reference/adapters/hn.mdx`
- `docs/content/en/reference/adapters/arxiv.mdx`
- `docs/content/en/reference/adapters/youtube.mdx`
- `docs/content/en/reference/adapters/searxng.mdx`
- `docs/content/en/reference/adapters/errors.mdx`
- `docs/content/en/reference/adapters/_meta.json`

**Gate**: 6 EN pages (5 adapter + 1 errors) green against
completeness CI; credential lint clean; lychee internal-strict
100% pass; lychee external for new provider URLs allowlisted →
Phase 4 unlock.

---

### Phase 4 — EN content authoring: auth-bearing + social (5 pages)

**Goal**: Higher-complexity pages — github + naver (auth-required;
naver also Korean-locale heavy) + bluesky + x (social shared
package) + still missing koreanews EN.

**Activities**:

#### 4.1 `github.mdx`

- All 10 sections per Phase 3 template, PLUS:
- Section 3 (Setup) per REQ-ADPDOC-010: GitHub PAT URL link +
  `USEARCH_GITHUB_TOKEN` env var + recommended scopes (e.g.,
  `repo:public_repo` for public-only) + verification command
  (`usearch query "test" --source github` non-error assertion) +
  SPEC-DEPLOY-001 Helm values cross-link.
- All credential examples MUST be placeholders (REQ-ADPDOC-018);
  `check-doc-credentials.sh` validates.

#### 4.2 `naver.mdx`

- All 10 sections, PLUS:
- Section 3 (Setup) per REQ-ADPDOC-010: Naver Developer Console
  URL + 2 env vars (`NAVER_CLIENT_ID`, `NAVER_CLIENT_SECRET`) +
  recommended Application category + verification command +
  DEPLOY-001 cross-link.
- Section 3 (Setup) per REQ-ADPDOC-011: 3-line Korean-locale
  summary + cross-link to DOC-001 `operators/korean-locale-
  setup.mdx`. Topics: Service URL registration in Naver
  Developer Console + UTF-8 query passthrough + DataLab endpoint
  distinction.
- Section 6 (Rate limits): note the `openapi.naver.com` redirect
  allowlist constraint (research §1.6) — operators behind
  corporate proxies must not redirect through CDN.
- Section 7 (Error reference): Naver 401 row with operator action
  "check `NAVER_CLIENT_ID` env var + Service URL registered in
  Developer Console".

#### 4.3 `bluesky.mdx`

- All 10 sections per template.
- Section 1 (Status & Compatibility): badge likely `beta`
  (research §1.7) based on Bluesky AppView recent rate-limit
  changes.
- "Shared implementation notes" callout linking to `x.mdx`
  (REQ-ADPDOC-009).
- Section 6 (Rate limits): 600/min (advertised) — Bluesky AppView
  public; mechanism is HTTP 429 response handling.

#### 4.4 `x.mdx`

- All 10 sections per template.
- Section 1 (Status & Compatibility): badge likely `alpha` per
  research §1.7 (degraded syndication).
- Section 2 (Overview): explicit "degraded mode — syndication
  endpoint health is opaque" framing.
- Section 6 (Rate limits): "none advertised — degraded mode"
  formulation per REQ-ADPDOC-012 enumeration.
- "Shared implementation notes" callout linking to `bluesky.mdx`.

#### 4.5 `koreanews.mdx` EN

- All 10 sections.
- Section 3 (Setup) per REQ-ADPDOC-011: 3-line Korean-locale
  summary covering EUC-KR legacy feed handling (`locale.go`),
  mecab-ko-aware dedup (`dedup.go`), KNC sidecar (`knc.go` +
  `USEARCH_KNC_ENDPOINT` env var + `services/storm/
  koreanewscrawler/`).
- Section 8 (Troubleshooting): ≥ 5 entries (REQ-ADPDOC-014
  koreanews-specific minimum due to multi-source complexity).

**Files created**:

- `docs/content/en/reference/adapters/github.mdx`
- `docs/content/en/reference/adapters/naver.mdx`
- `docs/content/en/reference/adapters/bluesky.mdx`
- `docs/content/en/reference/adapters/x.mdx`
- `docs/content/en/reference/adapters/koreanews.mdx`

**Files modified**:

- `docs/content/en/operators/deployment-helm.mdx` (DOC-001) —
  add anchored subsections `#github-pat`, `#naver-credentials`,
  `#knc-endpoint` per REQ-ADPDOC-010(e). DOC-001 owner protocol:
  PR review by DOC-001 owner before merge.

**Gate**: All 11 EN adapter pages + errors.mdx pass completeness
CI; credential lint clean; lychee internal 100% pass; cross-links
to `deployment-helm.mdx` anchors resolve → Phase 5 unlock.

---

### Phase 5 — `index.mdx` catalog + KO Tier-1 translation (5 pages)

**Goal**: Complete EN site with catalog + KO Tier-1 batch with
native reviewer signoff.

**Activities**:

#### 5.1 `index.mdx` EN catalog

- Replace SPEC-DOC-001 placeholder with `<AdapterCatalog>`
  component rendering 10-row table per REQ-ADPDOC-003.
- Each row reads frontmatter from `{adapter}.mdx` for category +
  auth + Korean-locale fields.
- "Common error categories" footnote linking `errors.mdx`.
- `_meta.json` updated to include `errors` in sidebar.

#### 5.2 KO Tier-1 4 pages

- `docs/content/ko/reference/adapters/naver.mdx` — KO 번역
  (manager-docs draft + native reviewer signoff).
- `docs/content/ko/reference/adapters/koreanews.mdx` — KO 번역.
- `docs/content/ko/reference/adapters/errors.mdx` — KO 번역
  (Category enum 자체는 코드 식별자라 unchanged; prose만 번역).
- `docs/content/ko/reference/adapters/index.mdx` — KO 카탈로그.
- `docs/content/ko/reference/adapters/_meta.json` — KO sidebar
  (4 entries).
- Each KO page: native reviewer signoff entry appended to
  `docs/content/ko/CONTRIBUTING.md` (DOC-001 ship state)
  reviewer log per REQ-ADPDOC-017 + NFR-ADPDOC-006.

#### 5.3 EVAL-002 status feed reconciliation

- If EVAL-002 dashboard export job (open question §8.4) is now
  shipping `adapter-status.json` daily → switch from static
  initial JSON to live feed; verify CI `adapter-status-staleness`
  passes.
- If EVAL-002 export job still pending → file follow-up SPEC-
  EVAL-002 amendment ticket; static initial JSON remains in
  place until export job lands.

**Files created**:

- `docs/content/en/reference/adapters/index.mdx` (replaces
  DOC-001 placeholder)
- `docs/content/ko/reference/adapters/index.mdx`
- `docs/content/ko/reference/adapters/naver.mdx`
- `docs/content/ko/reference/adapters/koreanews.mdx`
- `docs/content/ko/reference/adapters/errors.mdx`
- `docs/content/ko/reference/adapters/_meta.json`

**Files modified**:

- `docs/content/en/reference/adapters/_meta.json` (add `errors`)
- `docs/content/ko/CONTRIBUTING.md` (DOC-001) — append KO Tier-1
  reviewer log entries; DOC-001 owner review

**Gate**: All 16 MDX pages (12 EN + 4 KO Tier-1) pass completeness
CI; credential lint clean; lychee 100% internal pass; KO reviewer
log shows 4 entries → Phase 6 unlock.

---

### Phase 6 — DOC-001 coordination, surface-comparison cross-links, bilingual coverage gate amendment

**Goal**: Final integration with SPEC-DOC-001 site infrastructure.

**Activities**:

#### 6.1 `surface-comparison.mdx` cross-links

- Modify `docs/content/en/end-users/surface-comparison.mdx`
  (DOC-001 ownership): add cross-links from each row of the
  CLI/UI/Skill/MCP decision matrix to per-adapter reference
  pages where the row mentions a specific adapter (e.g., "Use
  CLI for batch Naver queries" → link to `reference/adapters/
  naver.mdx`).
- DOC-001 owner review required; coordinate via plan-auditor or
  direct PR review.

#### 6.2 `check-bilingual-coverage.sh` exclude pattern extension

- Modify `scripts/check-bilingual-coverage.sh` (DOC-001
  ownership) per REQ-ADPDOC-017:
  - Extend EXCLUDE pattern to skip `docs/content/en/reference/
    adapters/{reddit,hn,arxiv,github,youtube,bluesky,x,searxng}
    .mdx` (Tier-2 EN-only).
  - Extend REQUIRED pattern to include `docs/content/ko/
    reference/adapters/{index,naver,koreanews,errors}.mdx`
    (Tier-1 KO required).
- Test fixtures: deleting one KO Tier-1 page drops coverage
  below 90% threshold and fails CI; deleting one EN Tier-2 page
  does NOT fail.
- DOC-001 owner sign-off required (open question §8.5).

#### 6.3 CI integration verification

- Run full `docs.yml` workflow end-to-end:
  - `build` job (Nextra v4 build) — DOC-001 ownership; verify
    no regressions.
  - `link-check` (lychee) — verify 100% internal pass;
    NFR-ADPDOC-005 external allowlist working.
  - `gen-reference-drift` (DOC-001 REQ-DOC-007) — unchanged.
  - `gen-adapter-ref-drift` (NEW, DOC-002 REQ-ADPDOC-007).
  - `screenshot-freshness` (DOC-001 REQ-DOC-014) — unchanged
    (DOC-002 adds no screenshots in V1.0.0).
  - `bilingual-coverage` (DOC-001 REQ-DOC-016 + DOC-002
    REQ-ADPDOC-017 extension).
  - `adapter-page-completeness` (NEW, DOC-002 REQ-ADPDOC-002 +
    NFR-ADPDOC-004).
  - `adapter-status-staleness` (NEW, DOC-002 NFR-ADPDOC-003).
  - `check-doc-credentials` (NEW, DOC-002 REQ-ADPDOC-018).
- Total wall-clock measured; assert NFR-ADPDOC-001 (≤ 60s
  drift gate) + NFR-ADPDOC-002 (≤ 30s completeness gate) +
  combined ≤ 6 min per NFR-ADPDOC-001 ceiling.

#### 6.4 Pre-submission self-review per workflow-modes.md §"Pre-submission Self-Review"

- Review full diff of Phases 2-6 against SPEC acceptance
  criteria.
- Ask: simpler approach? — examine if any of the 3 React
  components could be merged (e.g., does `StatusBadge` justify
  separate component vs inline render?). Likely NO — separate
  testability + ARIA accessibility per WCAG 2.1 AA argues for
  keeping them separate.
- Ask: would removing any change still satisfy SPEC? — examine
  if `scripts/check-doc-credentials.sh` is redundant with
  SPEC-SEC-001 D2 gitleaks. NO — different gate timing
  (commit-time vs PR-time) + different pattern coverage
  (shape-resembling placeholders vs raw secrets), so both are
  load-bearing.
- If any simplification found, apply + re-run all CI; if
  no simplification, proceed to Phase 7.

**Gate**: All 9 CI jobs green; manual deploy to gh-pages staging
shows all 16 pages render correctly + status badges + Capabilities
tables + AdapterCatalog filter + lychee resolves all links →
Phase 7 unlock.

---

### Phase 7 — Sync (PR + docs publish + SPEC status update)

**Goal**: Ship SPEC-DOC-002 to V1.0.0 ship-ready state.

**Activities**:

- `/moai sync SPEC-DOC-002` invocation triggers manager-docs
  sync workflow.
- PR creation by manager-git agent with the diff from Phases
  2-6.
- gh-pages deploy on PR merge (DOC-001 REQ-DOC-015 workflow).
- Container image push to `ghcr.io/<org>/usearch-docs:<sha>`
  (DOC-001 REQ-DOC-015) — includes DOC-002 content.
- `.moai/specs/SPEC-DOC-002/spec.md` frontmatter updated:
  `status: implemented`, `updated: <ship date>`.
- HISTORY entry appended documenting V1.0.0 ship.
- CHANGELOG.md entry per KaC v1.1.0 format.
- Cross-SPEC notification:
  - SPEC-REL-001 owner notified — DOC-002 ship satisfies
    "complete adapter reference" exit criterion.
  - SPEC-DEPLOY-001 owner notified — Helm values cross-links
    in `deployment-helm.mdx` ready for DEPLOY-001 to consume.
  - SPEC-EVAL-002 owner notified — if export job still pending,
    follow-up amendment SPEC ticket created.

**Deliverables**: PR merged; docs site live with `reference/
adapters/` populated; SPEC status `implemented`; CHANGELOG
entry.

**Gate**: PR merged + gh-pages reachable at canonical URL +
container image pulled successfully + SPEC-REL-001 exit criterion
re-evaluated PASS → SPEC-DOC-002 V1.0.0 ship complete.

---

## 4. File-by-file work breakdown

Summary count per Phase (priority-ordered):

| Phase | EN MDX | KO MDX | Generated JSON | Components | Tools | Scripts | CI mod | Notes |
|-------|--------|--------|----------------|------------|-------|---------|--------|-------|
| 0 | — | — | — | — | — | — | — | plan-auditor + DOC-001 PASS gate |
| 1 | — | — | — | — | — | — | — | ANALYZE inventory; read-only |
| 2 | — | — | 11 + schema | 3 | 1 | 3 | 5 jobs added | drift + lint + components |
| 3 | 6 (5 adp + errors) + _meta | — | — | — | — | 1 (completeness full impl) | — | no-auth EN |
| 4 | 5 (github, naver, bluesky, x, koreanews) | — | — | — | — | — | — | auth + Korean EN; deployment-helm modify |
| 5 | 1 (index replaces placeholder) | 4 + _meta | adapter-status feed | — | — | — | — | catalog + KO Tier-1; CONTRIBUTING modify |
| 6 | — | — | — | — | — | 1 (bilingual modify) | full E2E | DOC-001 coordination; surface-comparison modify |
| 7 | — | — | — | — | — | — | — | sync + PR + ship |

Cumulative at Phase 7 ship:

- EN MDX: 12 (10 adapter + index + errors)
- KO MDX: 4 (naver + koreanews + errors + index)
- Generated JSON: 12 (10 Capabilities + 1 status + 1 schema)
- React components: 3 (StatusBadge, CapabilitiesTable,
  AdapterCatalog) + 3 test files
- Go tools: 1 (`tools/gen-adapter-ref/`)
- Shell scripts: 4 new (`gen-adapter-reference.sh`,
  `check-adapter-page-completeness.sh`, `check-doc-credentials.sh`,
  `.docs-credentials-patterns.toml`) + 1 modified
  (`check-bilingual-coverage.sh`)
- CI jobs added to docs.yml: 4 new
  (`gen-adapter-ref-drift`, `adapter-page-completeness`,
  `adapter-status-staleness`, `check-doc-credentials`) +
  1 modified (`bilingual-coverage` exclude pattern)
- DOC-001 ship state modifications: 4 files
  (`theme.config.tsx`, `lychee.toml`, `surface-comparison.mdx`,
  `deployment-helm.mdx`, `CONTRIBUTING.md` — all require DOC-001
  owner review)

---

## 5. Risk-managed sequencing

Priority order of phases is determined by **risk minimization**:

- Phase 0-1 (gates + analyze): zero risk, all read-only.
- Phase 2 (CI infrastructure): risk = breaking DOC-001 docs.yml.
  Mitigation = all new jobs are additive (no existing job
  modified); `theme.config.tsx` change is additive component
  registration only.
- Phase 3 (no-auth EN content): lowest content risk (no auth
  setup complexity, no Korean-locale subtleties). Establishes
  template stability before harder pages.
- Phase 4 (auth + social EN): higher content risk (REQ-ADPDOC-
  010 5-field auth setup, REQ-ADPDOC-018 credential lint stress
  test). Phase 3 template carries forward.
- Phase 5 (catalog + KO Tier-1): Native reviewer dependency
  introduced; SLO measured (NFR-ADPDOC-006 ≤ 5 days per KO
  page). EVAL-002 reconciliation may slip — graceful
  degradation already planned.
- Phase 6 (DOC-001 coordination): highest cross-SPEC risk. DOC-
  001 owner sign-off required for 3 files. Late phase placement
  means all DOC-002 work is verified ship-ready before
  coordination ask.
- Phase 7 (sync): mechanical.

If any phase fails its gate, **drift guard** (workflow-modes.md
§"Drift Guard") activates: cumulative drift > 30% triggers
re-planning gate; user notified via AskUserQuestion (orchestrator).

---

## 6. Coverage + completeness measurement

### 6.1 Code coverage targets (85%+ per SPEC frontmatter)

- `tools/gen-adapter-ref/`: AST extraction + JSON emit logic.
  Target 90% (golden tests + edge cases).
- `scripts/*.sh`: shell scripts measured via `bashcov` or
  manual exit-code matrix. Target 85%+.
- `docs/components/*.tsx`: React unit tests via Vitest or
  Jest. Target 85%+ (taxonomy + filter + fallback).

### 6.2 Content completeness gates (REQ-ADPDOC-* not coverage)

- 10-section heading order: REQ-ADPDOC-002 strict.
- ≥ 50 chars plain text per section: NFR-ADPDOC-004 (final
  threshold per Phase 0 §8.8 resolution).
- ≥ 3 troubleshooting entries (≥ 5 for koreanews):
  REQ-ADPDOC-014.
- ≥ 4 cross-links in Related: REQ-ADPDOC-016.
- 12 EN pages required; 4 KO Tier-1 pages required:
  REQ-ADPDOC-001, REQ-ADPDOC-017.
- Capabilities table auto-extracted (no hand override): REQ-
  ADPDOC-008 grep assertion.

### 6.3 Drift gates

- `gen-adapter-ref-drift`: REQ-ADPDOC-007 — committed JSON
  matches freshly-generated output.
- `adapter-status-staleness`: NFR-ADPDOC-003 — `adapter-status.
  json` mtime ≤ 7 days (warn, not fail).
- `lastVerified` per-page: REQ-ADPDOC-015 — ≥ 1 row within 90
  days; warn at > 180 days.

### 6.4 Anti-pattern gates

- `check-doc-credentials`: REQ-ADPDOC-018 — placeholder-only
  policy, hard fail.
- Bilingual coverage extension: REQ-ADPDOC-017 — 4 KO Tier-1
  required.

---

## 7. Backout plan

If SPEC-DOC-002 PR encounters production blocker post-Phase 7:

- **Backout option 1**: revert Phase 7 PR; SPEC-DOC-001 placeholder
  `index.mdx` is restored; site continues to function (DOC-001
  PASS unaffected). Pages added by DOC-002 simply do not exist;
  internal cross-links from DOC-001 ship state (`surface-
  comparison.mdx`, `deployment-helm.mdx`) become broken
  internal links — lychee CI fails. **Coordination required**:
  DOC-001 owner reverts their corresponding modifications.
- **Backout option 2 (granular)**: revert specific page batches
  (Phase 3 5 pages, Phase 4 5 pages, Phase 5 5 pages). The
  drift gate naturally degrades because `_generated/*.
  capabilities.json` files have no consumer if the corresponding
  page is removed — orphaned JSON file is acceptable (CI does
  not fail on orphaned JSON; only on JSON drift vs Go source).
- **Backout option 3**: hotfix via DOC-001 ship state — revert
  ONLY the DOC-001 modifications (Phase 6 § 6.1, 6.2) while
  keeping DOC-002 pages. Pages still exist but cross-links from
  DOC-001 pages are absent. Acceptable degraded state for an
  emergency rollback.

Backout decision made by SPEC-DOC-002 owner + SPEC-DOC-001 owner
+ MoAI orchestrator (AskUserQuestion).

---

## 8. Open Questions resolved within plan

- §8.1 (Korean-tokenizer cross-link only): committed by D6 +
  REQ-ADPDOC-011. Phase 4-5 implementation enforces.
- §8.2 (badge taxonomy ↔ EVAL-002 lifecycle alignment): Phase
  0 plan-auditor activity. If misalignment found, EVAL-002
  amendment scheduled in parallel (SPEC-EVAL-002 owner ticket).
- §8.3 (Bluesky vs X split): committed by D3 + REQ-ADPDOC-009.
  Phase 4 implementation enforces.
- §8.4 (EVAL-002 export job timing): graceful degradation in
  Phase 2 (static initial JSON) + Phase 5 reconciliation.
- §8.5 (`check-bilingual-coverage.sh` exclude pattern): Phase
  6 § 6.2 coordination.
- §8.6 (`tools/gen-adapter-ref/` location): Phase 2.1 confirms
  `tools/` directory creation (precedent set by this SPEC).
- §8.7 (provider doc URL per locale): Phase 3-5 per-page authoring
  decision; EN page → EN provider doc; KO page → KO provider
  doc where available.
- §8.8 (page completeness threshold): Phase 0 plan-auditor +
  baseline draft sample (Phase 3 first page draft) determines
  final value; default 50 chars.

---

## 9. Sync Phase delegation (post-Phase 7)

Per `.claude/rules/moai/workflow/spec-workflow.md` Phase Transitions:

- `/moai sync SPEC-DOC-002` invokes manager-docs sync workflow.
- manager-git creates PR with conventional commit message:
  `docs(spec): implement SPEC-DOC-002 — adapter reference pages
  with drift CI`.
- CHANGELOG.md entry per KaC v1.1.0 (DOC-001 REQ-DOC-007
  precedent inherited).
- API documentation: not applicable (DOC-002 is documentation
  SPEC, not API code).
- README.md: updated with link to `reference/adapters/` (DOC-001
  REQ-DOC-001 already established docs site link in README).

---

## 10. References (cross-references this plan relies on)

- `.moai/specs/SPEC-DOC-002/spec.md` — SPEC contract
- `.moai/specs/SPEC-DOC-002/research.md` — codebase analysis
- `.moai/specs/SPEC-DOC-001/{spec,plan}.md` — site infrastructure
  contract + DOC-001 phasing (DOC-002 Phase 0 prerequisite gate
  references DOC-001 PASS evidence)
- `.moai/specs/SPEC-EVAL-002/{spec,plan}.md` — dashboard schema +
  potential export job amendment
- `.moai/specs/SPEC-SEC-001/spec.md` D2 — gitleaks gate
  (DOC-002 §2.4 `check-doc-credentials.sh` aligns with this)
- `.claude/rules/moai/workflow/workflow-modes.md` — DDD cycle +
  Sprint Contract + Drift Guard + Pre-submission Self-Review
- `.claude/rules/moai/design/constitution.md` §11 — Sprint
  Contract harness routing
- `.claude/rules/moai/core/agent-common-protocol.md` — time
  estimation prohibition (this plan complies)
- `.claude/rules/moai/workflow/mx-tag-protocol.md` — MX tag
  integration for `tools/gen-adapter-ref/` (new Go program will
  carry `@MX:ANCHOR` on exported AST extraction functions per
  fan_in ≥ 3 heuristic if relevant)

---

*End of SPEC-DOC-002 plan.md v0.1.0 (draft).*
