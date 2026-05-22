# SPEC-DOC-001 Plan — phased implementation

Status: draft companion to spec.md
Author: limbowl via manager-spec
Date: 2026-05-22
Methodology: **DDD** (ANALYZE-PRESERVE-IMPROVE per `.claude/rules/
moai/workflow/workflow-modes.md`). DDD-mode justification: 본 SPEC은
**기존 docs 자산을 consolidate + migrate**하는 작업이 본질이다.
README.md, CHANGELOG.md, `.moai/project/product.md` + `tech.md`,
`docs/dependencies.md`, `.moai/docs/MCP_OAUTH_SETUP.md`, `ops/security/*`
는 모두 현재 production-ready 콘텐츠다 — 본 SPEC은 (a) ANALYZE
existing surface (research.md §1 inventory), (b) PRESERVE 콘텐츠를
MDX wrapper로 byte-fidelity 마이그레이션 (변형 금지), (c) IMPROVE는
Nextra IA + bilingual 커버리지 + CI 게이트만 추가. 신규 narrative
콘텐츠 (`getting-started/first-query`, `end-users/cli-tour`, 등)는
TDD 하위 cycle이 부적합 (content authoring은 test-first 패러다임이
아님) — 대신 manager-docs + native reviewer 검수 + bilingual-coverage
+ link-check + screenshot-freshness 자동 게이트가 quality 보장. 스크립트
(`gen-cli-reference.sh`, `check-screenshot-freshness.sh`, `check-
bilingual-coverage.sh`, `check-doc-claims.sh`)는 TDD 하위 cycle로
실행 (RED test → GREEN script → REFACTOR).

Coverage target: 85% (per spec.md frontmatter) applies to script
logic (4 새 shell scripts). MDX 콘텐츠는 coverage 측정 대상이 아닌
"완성도 percentage" 게이트 (bilingual-coverage + REQ-DOC-003 IA
완비 + REQ-DOC-005/006/009 페이지 목록 완비).

Harness: **standard** (per `.moai/config/sections/harness.yaml`
auto-routing — P1 docs SPEC, no security domain, 18 EARS REQs + 7
NFRs + 1 new Next.js sub-app + 1 new CI workflow + 4 new scripts +
1 new Dockerfile; Sprint Contract RECOMMENDED but NOT required
per `.claude/rules/moai/design/constitution.md` §11 "Sprint Contracts
are optional but recommended for standard harness level").

본 plan은 SPEC-DOC-001 구현을 priority-ordered phases로 sequence
한다. `.claude/rules/moai/core/agent-common-protocol.md` 시간 예측
금지 — phase는 priority + ordering만 사용.

---

## 1. Implementation principle

본 SPEC의 plan philosophy 5축:

1. **Content-first ordering** — Nextra 앱 부트스트랩은 작고 빠른
   작업이다. 본 plan의 80% 작업량은 콘텐츠 작성 (EN + KO 약 47개
   MDX 페이지). 따라서 phase ordering은 인프라 → 콘텐츠 → CI 게이트
   순서.
2. **PRESERVE before IMPROVE** — 기존 자산 (README, product.md,
   tech.md, dependencies.md, MCP_OAUTH_SETUP.md, ops/security/*)
   부터 MDX wrapper로 마이그레이션. 콘텐츠 변형 금지 — wrapper +
   frontmatter + 크로스링크만 추가. 그 후 hand-write 신규 narrative.
3. **EN authoritative for technical reference** — `reference/cli`,
   `reference/api`, `reference/mcp`는 EN-only (D3). KO는 narrative-
   heavy 페이지에 집중 (introduction, getting-started, end-users,
   operators core).
4. **CI 게이트는 작업 안정화 후 활성화** — Phase 6에서 게이트 활성화
   하기 전, Phase 4에서 baseline 콘텐츠가 게이트 통과 가능한 상태
   확인. 게이트가 초기에 fail하면 작업 시작이 차단됨.
5. **dual-deploy 마지막** — gh-pages + Docker container 배포는 Phase
   7. 콘텐츠 + CI 게이트가 모두 stable한 후 publish. release-blocking
   요소이므로 V1.0.0 freeze 전에 publish 검증.

---

## 2. Sprint Contract (RECOMMENDED per standard harness)

Sprint Contract는 builder (manager-ddd) ↔ evaluator-active 사이
협상 결과로 매 GAN Loop iteration 시작 전 작성. 본 SPEC의 V1 Sprint
Contract draft (run phase에서 evaluator-active와 finalize):

### Acceptance checklist (testable per iteration)

- [ ] Nextra v4 standalone app boots (`pnpm --dir docs dev` +
      `pnpm --dir docs build` 둘 다 zero error)
- [ ] 7-section IA가 EN + KO 둘 다 존재 (사이드바 + 랜딩 페이지)
- [ ] Tier-1 EN 페이지 (≈ 25개) 전부 작성 — 빈 placeholder 금지
- [ ] Tier-1 KO 페이지 (≈ 22개, EN의 ≥ 90%) 전부 작성 + native
      reviewer 검수 기록
- [ ] `scripts/gen-cli-reference.sh` 동작 + 현재 binary에 대해
      CLI reference MDX 생성 + drift check 통과
- [ ] `docs.yml` CI workflow 5개 job 모두 활성화 + green baseline
- [ ] 내부 링크 100% 해결 (lychee internal-strict pass)
- [ ] 90% bilingual coverage 게이트 pass
- [ ] gh-pages 배포 succeeds + `https://<org>.github.io/universal-
      search/` 접근 가능
- [ ] Docker container image (`ghcr.io/<org>/usearch-docs:<sha>`)
      build + pull + serve 검증

### Priority dimension

**Completeness** (evaluator-active 4-dimension scoring) — 본 SPEC의
quality는 콘텐츠 완비 + CI 게이트 작동의 함수. Originality는 N/A
(docs site는 industry-standard pattern 적용). Functionality는 사이트
build + deploy + 사용자 navigation 동작. Craft (코드 품질)는 script
4개 + Dockerfile 한정 (콘텐츠는 craft 측정 대상 아님).

### Test scenarios (integration)

- §5.1 Nextra v4 bootstrap: build succeeds + static export 동작
- §5.4 Getting Started flow: 새 사용자가 4-step 경로 따라 첫 쿼리
  성공
- §5.5 CLI reference auto-generation: 신규 subcommand 추가 시 drift
  detection
- §5.10 link-check enforcement: 깨진 internal link가 CI fail
- §5.12 dual deployment: main push 시 gh-pages + ghcr.io 동시 publish
- §5.13 bilingual coverage gate: KO 페이지 누락이 CI fail

### Pass conditions (minimum score per criterion)

- Completeness: ≥ 0.85 (must-pass; 90% bilingual + 모든 Tier-1
  페이지 존재가 정량 측정)
- Functionality: ≥ 0.80 (build + deploy + 사용자 navigation)
- Craft: ≥ 0.70 (script 코드 품질; shell + bash 기본 lint)
- Consistency: ≥ 0.80 (기존 SPEC pattern 준수 — DOC-001 IA가
  DOC-002, DEPLOY-001, REL-001과 horizontal 호환)

---

## 3. Phase ordering

본 SPEC은 **Phase 1 → 7**의 7개 phase로 sequence. 각 phase는 entry
condition + exit gate가 명시되어 있다.

### Phase 1 — ANALYZE: Existing docs surface inventory + IA design

**Priority**: High (foundation for all subsequent phases)

**Entry conditions**:
- spec.md draft v0.1.0 PASS annotation cycle.
- research.md §1 inventory complete.

**Goals**:
- 모든 기존 docs 자산의 file-by-file inventory 확정 (research.md
  §1 검증).
- 7-section IA + sub-page 목록 확정 (REQ-DOC-003 + spec.md §7.1).
- 각 sub-page의 sourcing 결정 (PRESERVE migration vs GENERATE
  auto-gen vs HAND-WRITE) — D2 strategy 적용.

**Tasks**:
- T1.1: Verify all 8 source assets in research.md §1.1-1.2 exist
  and have expected content.
- T1.2: Build a migration map matrix (Page → Source → Strategy)
  for each Tier-1 page. Format: `| Page | Source File | Strategy
  (PRESERVE/GENERATE/HAND-WRITE) | Owner |`.
- T1.3: Draft sidebar `_meta.json` (Nextra v4 navigation manifest)
  for each section.
- T1.4: Identify all `screenshot:ui:*` placeholders needed (web-ui-
  tour.mdx, skill-claude.mdx) — list to be filled in Phase 4.

**Exit gate**:
- Migration map committed to `docs/MIGRATION_MAP.md` (developer-
  facing, NOT user-surfaced).
- IA navigation manifest drafted in `_meta.json` skeletons under
  `docs/content/en/` and `docs/content/ko/`.
- plan-auditor reviews migration map for completeness.

---

### Phase 2 — IMPROVE infra: Nextra v4 app bootstrap

**Priority**: High (prerequisite for content authoring)

**Entry conditions**: Phase 1 exit gate PASS.

**Goals**:
- Standalone Nextra v4 app under `docs/` operational.
- i18n routing (en, ko) functional.
- Pagefind search functional on empty + sample content.
- Static export produces `docs/out/`.

**Tasks**:
- T2.1: `docs/package.json` + `pnpm-lock.yaml` (deps: nextra@4.x,
  nextra-theme-docs@4.x, next@16.x, react@19.x, typescript@5.x).
  Pin exact minor versions per SPEC-DEP-001 REQ-DEP-007.
- T2.2: `docs/next.config.mjs` with `output: 'export'`, i18n config
  (`locales: ['en', 'ko'], defaultLocale: 'en'`), Nextra plugin
  wiring.
- T2.3: `docs/theme.config.tsx` — site title ("Universal Search
  Docs"), locale switcher enabled, placeholder logo, footer with
  CHANGELOG link.
- T2.4: `docs/tsconfig.json` (TS 5 strict, matches `web/tsconfig.
  json` conventions).
- T2.5: `docs/.gitignore` + repo-root `.gitignore` amendments
  (`docs/node_modules/`, `docs/out/`, `docs/.next/`).
- T2.6: Sample content for smoke test: `content/en/index.mdx`,
  `content/ko/index.mdx`, `content/en/introduction/index.mdx` (1
  paragraph each).
- T2.7: Verify `pnpm --dir docs install && pnpm --dir docs build`
  zero errors; serve `docs/out/` via `npx serve docs/out/`; navigate
  `/en/` and `/ko/`.
- T2.8: Verify Pagefind search indexes both locales.

**Exit gate**:
- All REQ-DOC-001 + REQ-DOC-002 acceptance criteria met on smoke-
  test content.
- `docs/README.md` (docs-app developer onboarding) drafted.
- No CI workflow yet (Phase 6).

---

### Phase 3 — PRESERVE migration: Existing assets → MDX wrappers

**Priority**: High (most user-facing value, no new content authoring)

**Entry conditions**: Phase 2 exit gate PASS.

**Goals**:
- Migrate all 8 existing source assets (research.md §1.1-1.3) to
  MDX wrappers under `content/en/`.
- Byte-fidelity preservation: source content unchanged; MDX wrapper
  adds frontmatter + cross-links + locale context.
- Tier-1 KO mirrors with placeholder content (Phase 4 fills KO).

**Tasks**:
- T3.1: `content/en/introduction/index.mdx` — wrap product.md §1
  (identity).
- T3.2: `content/en/introduction/personas.mdx` — wrap product.md §3.
- T3.3: `content/en/introduction/comparison.mdx` — wrap product.md
  §7 (no marketing tone — REQ-DOC-018 lint pass).
- T3.4: `content/en/getting-started/prerequisites.mdx` — wrap README
  Prerequisites table.
- T3.5: `content/en/getting-started/compose-setup.mdx` — wrap README
  Quickstart steps 1-3.
- T3.6: `content/en/getting-started/build-binary.mdx` — wrap README
  Quickstart steps 4-5.
- T3.7: `content/en/reference/architecture.mdx` — wrap tech.md
  Architectural Principles + Tech Stack tables.
- T3.8: `content/en/reference/dependencies.mdx` — link to canonical
  `docs/dependencies.md` (NOT re-render).
- T3.9: `content/en/operators/auth-setup.mdx` — wrap `.moai/docs/
  MCP_OAUTH_SETUP.md` content.
- T3.10: `content/en/operators/security/runbook.mdx`, `owasp-
  checklist.mdx`, `threat-model.mdx` — cross-link `ops/security/*`
  canonical files.
- T3.11: `content/en/legal/licenses.mdx` — wrap product.md §8
  upstream licenses + LICENSE + NOTICE references.
- T3.12: `content/en/legal/changelog.mdx` — embed/link CHANGELOG.md.
- T3.13: Tier-1 KO placeholder pages — `content/ko/<path>/index.mdx`
  with "이 페이지는 한국어 번역이 준비 중입니다 / English: <link>"
  placeholder + Phase 4 will fill.

**Exit gate**:
- All migration tasks T3.1-T3.13 complete.
- `pnpm --dir docs build` zero errors with migrated content.
- Manual smoke test: navigate to each migrated page in browser,
  verify content renders correctly.
- Internal link integrity: every cross-link resolves (Phase 6 lychee
  CI gate will enforce; here manual spot-check).

---

### Phase 4 — HAND-WRITE: Net-new narrative content (EN + KO)

**Priority**: High (largest work block)

**Entry conditions**: Phase 3 exit gate PASS.

**Goals**:
- Author all net-new EN narrative content (≈ 13 new MDX pages).
- Native-Korean-speaking reviewer assigned (per Open Question
  §11.2).
- Author Tier-1 KO mirrors (replace Phase 3 placeholders).

**Tasks**:
- T4.1: `content/en/getting-started/index.mdx` — section landing
  + 30-min path overview.
- T4.2: `content/en/getting-started/first-query.mdx` — `usearch
  query "hello"` + expected output narrative.
- T4.3: `content/en/end-users/index.mdx` — section landing.
- T4.4: `content/en/end-users/surface-comparison.mdx` — CLI vs UI
  vs Skill vs MCP decision matrix (cross-linked from `introduction/
  index.mdx`).
- T4.5: `content/en/end-users/cli-tour.mdx` — depends on CLI-002
  implementation status. V1.0.0 ship 시점에 implementation 완료
  가정; pre-impl 단계는 stub.
- T4.6: `content/en/end-users/web-ui-tour.mdx` — UI-001 4 routes
  narrative + screenshot placeholders (`screenshot:ui:home`,
  `screenshot:ui:source-detail`, `screenshot:ui:history`).
  실 스크린샷은 UI-001 implementation 완료 후 capture.
- T4.7: `content/en/end-users/skill-claude.mdx` — Claude Skill
  install + usage. SKILL-001 implementation 완료 후 finalize.
- T4.8: `content/en/end-users/mcp-integration.mdx` — MCP-001 host
  config for Claude Code, Codex, Gemini CLI. MCP-001 implementation
  완료 후 finalize.
- T4.9: `content/en/operators/index.mdx` — section landing.
- T4.10: `content/en/operators/deployment-helm.mdx` — Helm install
  walk-through. DEPLOY-001 미 ship 상태에서 V1.0.0 placeholder +
  V1.0.x patch에서 finalize 가능.
- T4.11: `content/en/operators/team-rbac.mdx` — Casbin RBAC ops
  guide (AUTH-002 implementation 기반).
- T4.12: `content/en/operators/audit-log.mdx` — Audit log ops
  guide (AUTH-003 implementation 기반).
- T4.13: `content/en/operators/observability.mdx` — Prom + slog +
  OTel ops guide (OBS-001 implementation 기반 + cardinality
  allowlist 카탈로그).
- T4.14: `content/en/operators/security/index.mdx` — security ops
  landing.
- T4.15: `content/en/reference/mcp/index.mdx` — MCP tool catalog
  (hand-curated; MCP-001 OpenAPI ship 시 V1.1에서 자동 생성으로
  전환).
- T4.16: `content/en/reference/adapters/index.mdx` — placeholder
  + SPEC-DOC-002 link (REQ-DOC-008 IA slot reservation).
- T4.17: `content/en/troubleshooting/index.mdx` — top-10 entries
  (5-field format: Symptom → Cause → Diagnostic → Resolution →
  SPEC). 10 entries drawn from CACHE-001 5-phase failure modes,
  AUTH-001 OIDC discovery failures, SEC-001 SSRF block triage,
  IDX-003 mecab-ko issues, LLM-001 provider errors, BOOT-001
  compose port conflicts, BOOT-001 missing env vars, IDX-001
  store connectivity, ADAPTER API key acquisition (placeholder
  → DOC-002), SEC-001 ratelimit.
- T4.18: `content/en/legal/index.mdx` — legal section landing.
- T4.19: `content/en/legal/security.mdx` — responsible disclosure
  (SEC-001 V14 SECURITY.md cross-link).
- T4.20: `content/en/legal/accessibility.mdx` — V1.0.0 freeze gate
  audit (PLACEHOLDER at Phase 4; populated by Phase 6 manual audit).
- T4.21: `content/en/legal/performance.mdx` — V1.0.0 freeze gate
  Lighthouse audit (PLACEHOLDER at Phase 4; populated by Phase 6).
- T4.22: `content/en/CONTRIBUTING.md` — docs contribution workflow.
- T4.23 ~ T4.44: KO mirrors of all T4.1-T4.22 — Tier-1 ≥ 90%
  coverage. `content/ko/operators/korean-locale-setup.mdx`
  KO-authoritative.
- T4.45: `content/ko/CONTRIBUTING.md` + reviewer log table.

**Exit gate**:
- All EN Tier-1 pages exist with substantive content (no empty
  placeholder).
- KO Tier-1 coverage ≥ 90% via `scripts/check-bilingual-coverage.sh`
  (manual run at exit; CI activates Phase 6).
- Native reviewer log entry in `content/ko/CONTRIBUTING.md` for at
  least the first KO page batch.

---

### Phase 5 — IMPROVE scripts: 4 new CI helper scripts (TDD)

**Priority**: Medium (CI gate enablement prerequisite)

**Entry conditions**: Phase 4 exit gate PASS (at least Tier-1 EN
content available for scripts to scan).

**Goals**:
- 4 new scripts under `scripts/` operational.
- Each script has RED → GREEN → REFACTOR cycle.

**Tasks**:
- T5.1: `scripts/gen-cli-reference.sh` (REQ-DOC-007).
  RED: test that asserts the script exits non-zero when binary not
  built. GREEN: implement `make build` + `usearch --help` parsing
  + MDX writeout. REFACTOR: extract subcommand enumeration.
- T5.2: `scripts/check-screenshot-freshness.sh` (REQ-DOC-014).
  RED: test asserts script fails on file with mtime > 90 days
  AND `screenshot:ui:*` tag. GREEN: implement scan + tag parsing.
  REFACTOR: separate concerns (scan / parse / compare / report).
- T5.3: `scripts/check-bilingual-coverage.sh` (REQ-DOC-016).
  RED: test asserts script fails when KO coverage < 90% of EN
  excluding `reference/cli/` + `reference/api/`. GREEN: implement
  EN enumeration + KO existence check + report. REFACTOR: extract
  exclusion list to config.
- T5.4: `scripts/check-doc-claims.sh` (REQ-DOC-018).
  RED: test asserts script warns on prohibited phrase. GREEN:
  implement grep + warning output. REFACTOR: externalize prohibited
  phrase list.

**Exit gate**:
- All 4 scripts pass their RED test (initial failure) → GREEN test
  (post-implementation pass).
- Coverage of script logic ≥ 85% (per spec.md target).
- Scripts run successfully against Phase 4 content (baseline pass).

---

### Phase 6 — IMPROVE CI: `.github/workflows/docs.yml` activation

**Priority**: Medium (enables ongoing quality enforcement)

**Entry conditions**: Phase 5 exit gate PASS.

**Goals**:
- 5-job CI workflow operational on every PR + main push.
- Baseline green: current main branch passes all 5 jobs.
- Stale-screenshot Issue automation working.

**Tasks**:
- T6.1: `.github/workflows/docs.yml` 5-job workflow per REQ-DOC-012.
  Triggers: `pull_request` + `push: main` on paths `docs/**`,
  `scripts/gen-cli-reference.sh`, `scripts/check-screenshot-
  freshness.sh`, `scripts/check-bilingual-coverage.sh`, `scripts/
  check-doc-claims.sh`, `.github/workflows/docs.yml`.
- T6.2: `docs/lychee.toml` per REQ-DOC-013 — internal-strict,
  external-warn, allowlist baseline (github.com API, anthropic.com,
  x.com, naver.com developer).
- T6.3: `actions/cache@v4` for `.lycheecache` per NFR-DOC-005.
- T6.4: Auto-Issue creation logic (`peter-evans/create-issue-from-
  file@v5` 또는 GitHub CLI) for stale screenshot warnings per
  REQ-DOC-014.
- T6.5: Verify baseline green: trigger workflow on PR with no
  changes, confirm all 5 jobs pass.
- T6.6: Synthetic regression tests: temporarily inject broken
  internal link + stale screenshot + missing KO page in a test
  branch; verify each triggers the correct job failure.

**Exit gate**:
- CI workflow active on main branch with green baseline.
- Synthetic regression tests confirm each gate fires correctly.
- NFR-DOC-001 runtime budget (≤ 5 min) met (initial measurement).

---

### Phase 7 — IMPROVE deployment: Dual gh-pages + container

**Priority**: Medium (release-blocking — V1.0.0 ship 시점에 publish)

**Entry conditions**: Phase 6 exit gate PASS.

**Goals**:
- gh-pages deploy operational on every `main` push.
- Container image build + push to `ghcr.io/<org>/usearch-docs`
  operational.
- Both deploys verified end-to-end.

**Tasks**:
- T7.1: Add `deploy-pages` job to `docs.yml` (on `push: main` only):
  `actions/upload-pages-artifact@v3` + `actions/deploy-pages@v4`.
- T7.2: `Dockerfile.docs` multi-stage build per REQ-DOC-015.
  Stage 1: `node:22-alpine` + `pnpm install` + `pnpm build`.
  Stage 2: `caddy:2.8-alpine` + `COPY --from=builder /app/out
  /srv` + Caddyfile.
- T7.3: Add `build-and-push-container` job to `docs.yml`:
  `docker/build-push-action@v6` to `ghcr.io/<org>/usearch-docs:
  <sha>` + `:latest` on main; tagged release (`v*.*.*`) → `:v*.
  *.*` per REQ-DOC-015.
- T7.4: Container image Trivy scan per NFR-DOC-004 (chains to
  SPEC-SEC-001 D1 Trivy policy).
- T7.5: Verify gh-pages deploy: push test commit to main; confirm
  `https://<org>.github.io/universal-search/` reflects change within
  5 minutes.
- T7.6: Verify container image: `docker pull ghcr.io/<org>/usearch-
  docs:latest && docker run -p 8080:80 ...` serves docs index.
- T7.7: NFR-DOC-002 bundle-size measurement + NFR-DOC-004 image-
  size measurement; if violations, identify reduction opportunities.

**Exit gate**:
- gh-pages reachable at canonical URL.
- Container image pullable from `ghcr.io/<org>/usearch-docs:latest`.
- All NFR-DOC-001 through NFR-DOC-007 SLAs verified or documented
  baseline.
- V1.0.0 freeze gate manual audits (REQ-DOC-017 accessibility +
  Lighthouse) recorded in `legal/accessibility.mdx` + `legal/
  performance.mdx`.
- Drift guard check (per `workflow-modes.md`): planned files
  (this plan §3 tasks) vs actual modifications — drift ≤ 30%.

---

## 4. Files to Create / Modify (consolidated, per phase)

spec.md §7.1과 §7.2의 file 목록을 phase별로 grouping. 동일 file은
중복하지 않고 최초 등장 phase에 listed.

| Phase | Path | Strategy | Owner |
|-------|------|----------|-------|
| 1 | `docs/MIGRATION_MAP.md` (developer-facing) | HAND-WRITE | manager-docs |
| 2 | `docs/package.json`, `pnpm-lock.yaml`, `next.config.mjs`, `theme.config.tsx`, `tsconfig.json`, `.gitignore`, `README.md` | NEW | manager-docs |
| 2 | `docs/content/en/index.mdx`, `content/ko/index.mdx` (smoke-test) | NEW (minimal) | manager-docs |
| 3 | `docs/content/en/introduction/{index,personas,comparison}.mdx` | PRESERVE (wrap product.md) | manager-docs |
| 3 | `docs/content/en/getting-started/{prerequisites,compose-setup,build-binary}.mdx` | PRESERVE (wrap README) | manager-docs |
| 3 | `docs/content/en/reference/{architecture,dependencies}.mdx` | PRESERVE (wrap tech.md + docs/dependencies.md) | manager-docs |
| 3 | `docs/content/en/operators/auth-setup.mdx` | PRESERVE (wrap MCP_OAUTH_SETUP.md) | manager-docs |
| 3 | `docs/content/en/operators/security/{runbook,owasp-checklist,threat-model}.mdx` | PRESERVE (cross-link ops/security/*) | manager-docs |
| 3 | `docs/content/en/legal/{licenses,changelog}.mdx` | PRESERVE (wrap product.md §8 + CHANGELOG.md) | manager-docs |
| 3 | `docs/content/ko/**` Tier-1 placeholder MDX | NEW (Phase 4 fills) | manager-docs |
| 4 | `docs/content/en/getting-started/{index,first-query}.mdx` | HAND-WRITE | manager-docs |
| 4 | `docs/content/en/end-users/{index,surface-comparison,cli-tour,web-ui-tour,skill-claude,mcp-integration}.mdx` | HAND-WRITE | manager-docs |
| 4 | `docs/content/en/operators/{index,deployment-helm,team-rbac,audit-log,observability}.mdx` | HAND-WRITE | manager-docs |
| 4 | `docs/content/en/operators/security/index.mdx` | HAND-WRITE | manager-docs |
| 4 | `docs/content/en/reference/{mcp,adapters,cli}/index.mdx` | HAND-WRITE | manager-docs |
| 4 | `docs/content/en/troubleshooting/index.mdx` | HAND-WRITE | manager-docs |
| 4 | `docs/content/en/legal/{index,security,accessibility,performance}.mdx` | HAND-WRITE (accessibility/performance: Phase 6 audit) | manager-docs |
| 4 | `docs/content/en/CONTRIBUTING.md` | HAND-WRITE | manager-docs |
| 4 | `docs/content/ko/**` Tier-1 KO 번역 (≥ 90% coverage) | HAND-WRITE + reviewer | manager-docs + reviewer |
| 4 | `docs/content/ko/operators/korean-locale-setup.mdx` (KO-authoritative) | HAND-WRITE | manager-docs |
| 4 | `docs/content/ko/CONTRIBUTING.md` + reviewer log | HAND-WRITE | manager-docs + reviewer |
| 5 | `scripts/gen-cli-reference.sh` | NEW (TDD) | manager-docs |
| 5 | `scripts/check-screenshot-freshness.sh` | NEW (TDD) | manager-docs |
| 5 | `scripts/check-bilingual-coverage.sh` | NEW (TDD) | manager-docs |
| 5 | `scripts/check-doc-claims.sh` | NEW (TDD) | manager-docs |
| 6 | `.github/workflows/docs.yml` | NEW | manager-docs |
| 6 | `docs/lychee.toml` | NEW | manager-docs |
| 7 | `Dockerfile.docs` | NEW | manager-docs + expert-devops review |
| (multi) | `README.md` (root) — add docs link | MODIFIED | manager-docs |
| (multi) | `.gitignore` (root) — add docs paths | MODIFIED | manager-docs |
| (multi) | `CHANGELOG.md` — add SPEC-DOC-001 entry | MODIFIED at run completion | manager-docs |

---

## 5. Risk + drift management

- **Drift guard**: 매 phase exit에 `workflow-modes.md` drift guard
  실행. planned (이 plan §3 tasks) vs actual diff < 30%. 초과 시
  Re-planning Gate 활성화.
- **Re-planning trigger**: 3-iteration stagnation (acceptance criteria
  완료율 zero for 3 consecutive iterations). `progress.md`에 iteration
  end마다 acceptance count + error delta 기록.
- **CI 게이트 회피 금지**: Phase 6 활성화 후 `--no-verify` git
  commit 금지. CI가 fail하면 SPEC amendment 또는 acceptance criteria
  완화 (manager-docs + plan-auditor 합의).
- **KO reviewer 부재 시**: REQ-DOC-010 hard requirement. reviewer
  assignment 지연 시 V1.0.0 ship 차단 — manager-docs가 user 알림.
- **External link rot**: NFR-DOC-005 weekly cron으로 점진 발견. 점차
  깨지면 quarterly triage 운영.

---

## 6. Coordination with downstream SPECs

- **SPEC-DOC-002 (parallel)**: 본 SPEC의 `reference/adapters/index.
  mdx` slot이 reserved. DOC-002 작업 시 DOC-001 작업에 영향 없음 —
  DOC-002 PR가 별도로 `reference/adapters/*.mdx` 추가.
- **SPEC-DEPLOY-001 (downstream)**: DEPLOY-001 Helm chart가 본 SPEC
  container image (`ghcr.io/<org>/usearch-docs:<tag>`)를 optional
  subchart로 bundle. DEPLOY-001 작성 시 본 SPEC의 image naming +
  Helm values schema 협의.
- **SPEC-REL-001 (downstream)**: REL-001 release notes가 docs canonical
  URL을 cite. REL-001 작성 시 본 SPEC의 deploy 상태 확인.

---

## 7. Open questions resolution path

spec.md §8 + research.md §11의 10개 Open Question은 다음 시점에
resolution:

| # | Question | Resolution timing | Owner |
|---|----------|------------------|-------|
| 11.1 | Brand identity timing | Phase 2 (placeholder OK) → V1.0.x patch | manager-docs + brand owner |
| 11.2 | KO reviewer pool | Phase 4 entry (BLOCKING) | user + manager-docs |
| 11.3 | gh-pages canonical URL `<org>` | Phase 7 entry (BLOCKING) | user + SPEC-REL-001 |
| 11.4 | Container registry choice | Phase 7 entry | user + expert-devops |
| 11.5 | MCP tool catalog source | Phase 4 (hand-curated baseline) | manager-docs + MCP-001 owner |
| 11.6 | Auto-screenshot policy | V1.1 (deferred) | post-V1 |
| 11.7 | Search index size | Phase 6 (measured) | manager-docs |
| 11.8 | Lychee external allowlist | Phase 6 (baseline) | manager-docs |
| 11.9 | Docs container base (Caddy vs nginx) | Phase 7 (recommend Caddy) | manager-docs |
| 11.10 | Auto-translated KO drafts | V1.0.0 (rejected) → V1.1 re-eval | post-V1 |

BLOCKING items (11.2, 11.3, 11.4)은 해당 phase 진입 전에 user
resolution 필수. 미해결 시 phase entry 차단.

---

## 8. Completion criteria (V1.0.0 ship gate)

본 SPEC의 V1.0.0 ship 시점 모든 다음 조건 충족:

- [ ] 7-section IA EN + KO 둘 다 완비 (REQ-DOC-003)
- [ ] Tier-1 EN 페이지 약 25개 작성 (Phase 4 task list)
- [ ] Tier-1 KO 페이지 ≥ 90% coverage (REQ-DOC-016)
- [ ] CLI reference auto-generation 동작 (REQ-DOC-007)
- [ ] 4 scripts 모두 RED → GREEN → REFACTOR 완료 + 85% coverage
      (Phase 5)
- [ ] `docs.yml` 5-job CI 전부 green baseline (Phase 6)
- [ ] gh-pages publish + container image push 둘 다 동작 (Phase 7)
- [ ] V1.0.0 freeze gate manual audit (accessibility + performance)
      결과 recorded in `legal/accessibility.mdx` + `legal/performance
      .mdx`
- [ ] CHANGELOG.md에 SPEC-DOC-001 entry 추가
- [ ] SPEC-REL-001 release notes가 docs canonical URL cite 가능

본 plan v0.1.0 (draft) — plan-auditor cycle pending. annotation
cycle 결과 + Open Question resolution 후 finalize.

---

*End of SPEC-DOC-001 plan v0.1.0 (draft).*
