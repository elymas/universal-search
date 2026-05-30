---
id: SPEC-DOC-001
version: 0.2.0
status: approved
created: 2026-05-22
updated: 2026-05-30
author: limbowl
priority: P1
issue_number: 0
title: User guide — operator + end-user documentation site on Nextra, bilingual EN+KO, with link-check CI gate and gh-pages + container deployment
milestone: M9 — V1 release
owner: manager-docs
methodology: ddd
coverage_target: 85
depends_on: [SPEC-CLI-001, SPEC-CLI-002, SPEC-UI-001, SPEC-MCP-001, SPEC-SKILL-001, SPEC-AUTH-001, SPEC-AUTH-002, SPEC-AUTH-003, SPEC-OBS-001, SPEC-SEC-001]
blocks: [SPEC-REL-001]
related: [SPEC-DOC-002, SPEC-DEPLOY-001]
---

# SPEC-DOC-001: User guide — operator + end-user docs site on Nextra (bilingual EN+KO)

## HISTORY

- 2026-05-30 (amendment v0.2.0, limbowl via manager-spec):
  plan-auditor re-audit 대응 — 3개 stale infra/feature assumption
  수정 + manager-strategy proportionality 권고 적용:
  - **B1 (docs/ not greenfield)**: `docs/`는 빈 폴더가 아니라 STALE
    Nextra v3 / Next 14 build artifact (`.next/`, `out/`,
    `node_modules/`, v3-style `pages/` — 모두 gitignored)와
    git-tracked dependency 문서 (`dependencies.md`, `_deps-*.md`,
    `licenses/`)가 공존한다. REQ-DOC-001 + §1.1 + plan T2를
    수정: T2가 stale v3 artifact를 먼저 정리한 뒤 Nextra v4 /
    Next 16 (`^16.2.6`, `web/`와 일치)을 pin. Nextra v4는
    `content/{locale}/` 구조 사용 (v3 `pages/` 아님). git-tracked
    dependency 문서는 보존.
  - **B2 (SEC-001 forward-reference)**: `ops/security/*`는 main에
    존재하지 않는다 (SPEC-SEC-001 PR #42 미머지). REQ-DOC-005 +
    §1 inventory + §7 + plan T3.10을 수정: security operator
    페이지는 FORWARD-REFERENCE (placeholder + SPEC-SEC-001 링크)이며,
    SEC-001 머지 전까지 존재하지 않는 `ops/security/*` 파일 cross-
    link 금지. link-check CI는 이 forward-ref에서 fail하지 않음
    (lychee exclude/allowlist 처리).
  - **B3 (CLI surface underestimated)**: 실제 CLI는 7개 subcommand
    (`query`, `config`, `history`, `deep`, `sources`, `login`,
    `repl` — `cmd/usearch/root.go`). REQ-DOC-007의 "currently
    query"를 7개 subcommand 전체로 수정.
  - **Proportionality 축소** (manager-strategy 권고 자율 적용):
    (a) KO 번역 — V1은 Tier-1 (key operator + getting-started)
        KO만 ship; full reference-section KO는 V1.1로 연기. 90%
        KO-coverage gate는 Tier-1 한정으로 scope.
    (b) automated a11y CI (Pa11y/axe-core), automated Lighthouse
        CI, Playwright auto-screenshot은 V1.1로 연기 — V1은 manual
        audit + freshness gate만.
    (c) Docker container live-publish는 CI job은 작성하되 실제
        publish는 `<org>`/registry 경로 확정 (Open Question §3/§4)
        전까지 conditional/deferred — V1 gate 아님.
    (d) `check-doc-claims.sh` (P2)는 lowest priority / optional
        유지.
    V1-essential 유지: Nextra 사이트 build + gh-pages deploy,
    link-check CI (internal fail / external warn), Tier-1 EN+KO
    콘텐츠, CLI/MCP/config reference 페이지.

- 2026-05-22 (initial draft v0.1.0, limbowl via manager-spec):
  M9 (V1 release) 첫 번째 SPEC이자 V1.0.0 태깅의 release-blocking
  dependency. `.moai/project/roadmap.md:112` ("SPEC-DOC-001 | User
  guide | operator + end-user docs on Nextra | manager-docs")의
  full EARS 확장 + §5 M9 exit criterion ("docs site live")의
  단독 담보 SPEC. SPEC-DOC-001이 PASS하지 못하면 SPEC-REL-001의
  V1.0.0 tagging이 차단되며, 외부 사용자에게 ship되는 binary가
  사용 설명서 없이 출시되어 self-hosted 운영자 onboarding이
  불가능하다.

  본 SPEC은 **새로운 콘텐츠 시스템을 발명하지 않는다**. 11개
  완료/예정 SPEC의 README/runbook/configuration 자산을 **(a)
  consolidate, (b) gap-fill operator + end-user 내러티브 생성,
  (c) 단일 Nextra 사이트로 publish**하는 DDD-style consolidation
  SPEC이다. 따라서 methodology는 DDD (ANALYZE existing surface →
  PRESERVE working content with byte-fidelity link forwarding →
  IMPROVE with Nextra IA + bilingual coverage + CI gates).

  현재 코드베이스에 이미 배치된 docs 자산 (Pre-M9 inventory —
  research.md §1 참조):

  - `README.md` (87 lines, last touched 2026-04-28): repo-level
    quickstart (Docker / Go / make targets), prerequisites table,
    architecture diagram, license + SearXNG AGPL service-boundary
    note. Authoritative for "first 30 seconds" content but lacks
    end-user (query / CLI / Skill / MCP) narrative.
  - `CHANGELOG.md` (39k, KaC v1.1.0 format): per-SPEC commit log
    from M1 (2026-04-24) through M6 AUTH rollout (2026-05-22).
    SPEC reference index — must be linked from docs site footer.
  - `.moai/project/product.md` (172 lines): identity, problem,
    personas (Research-heavy engineer / Product lead / Korean
    analyst / Team lead), V1 scope, success metrics, differentiation
    matrix, upstream licenses. **Authoritative for "what is
    Universal Search" content** — docs Introduction page consumes
    this verbatim.
  - `.moai/project/tech.md` (~200 lines, frequently updated):
    architectural principles, language/runtime matrix, V1-locked
    tech stack per layer (retrieval / synthesis / surfaces).
    Authoritative for "architecture & rationale" content.
  - `.moai/project/structure.md`, `.moai/project/roadmap.md`:
    repo layout + milestone map. Internal references; not directly
    user-facing but inform IA decisions.
  - `docs/dependencies.md` + `docs/_deps-header.md` + `docs/_deps-
    compose-table.md` + `docs/licenses/` (git-tracked per
    `git ls-files docs/`): auto-generated dependency manifest from
    SPEC-DEP-001 (scripts/gen-deps-manifest.sh). Docs site links to
    this; does NOT re-render. **These git-tracked files MUST be
    preserved** — they are NOT removed during the v3-artifact cleanup
    below.
  - **STALE Nextra v3 / Next 14 build artifacts in `docs/`** (all
    gitignored, NOT tracked): `docs/.next/`, `docs/out/`,
    `docs/node_modules/`, `docs/pages/` (v3-convention page tree),
    `docs/content/` (partial v3-era scratch). These are leftovers
    from an abandoned Nextra v3 spike — `docs/` is therefore NOT a
    greenfield folder. They MUST be cleaned (plan T2) before the
    Nextra v4 app is bootstrapped, so v3 conventions (`pages/`) do
    not collide with v4 conventions (`content/{locale}/`).
  - `.moai/docs/MCP_OAUTH_SETUP.md`: M6 operator-facing OAuth/OIDC
    setup walk-through. Migration target — moves to `docs/operators/
    auth-setup.md` (or KO mirror).
  - `ops/security/runbook.md`, `ops/security/owasp-asvs-checklist
    .md`, `ops/security/threat-model.md` (SPEC-SEC-001 deliverables,
    M8 — **NOT on main; SPEC-SEC-001 PR #42 unmerged**): operator-
    facing security incident response + ASVS evidence + STRIDE model.
    **Forward-reference target** — until SEC-001 merges, the docs
    security pages are placeholders linking to SPEC-SEC-001 status;
    no cross-link to nonexistent `ops/security/*` files is emitted.
    On SEC-001 merge, these become cross-link targets in docs
    `Operators › Security` section.
  - `web/README.md` (Next.js UI scaffold notes): developer-facing,
    not migrated to public docs site.
  - Per-SPEC `acceptance.md` files: developer-facing test scenarios,
    not migrated. Public docs site cites SPEC IDs but does NOT
    surface internal SPEC bodies.

  본 SPEC이 신규로 도입하는 것:

  - `docs/` Nextra v4 application (separate Next.js sub-app, distinct
    from `web/` query UI). Path: `docs/` at repo root. **`docs/` is
    NOT greenfield**: it currently holds stale Nextra v3 / Next 14
    build artifacts (gitignored `.next/`, `out/`, `node_modules/`,
    `pages/`, scratch `content/`) plus git-tracked dependency docs
    (`dependencies.md`, `_deps-*.md`, `licenses/`). The v3 artifacts
    are cleaned first (plan T2); the git-tracked dependency docs are
    preserved and absorbed as link targets into Nextra `content/en/
    reference/dependencies.mdx` route + `content/en/legal/licenses
    .mdx`. The Nextra v4 app pins **Next.js `^16.2.6`** (matching
    `web/package.json`) and uses the v4 `content/{locale}/` content
    convention (NOT the v3 `pages/` convention).
  - Bilingual content tree: `content/en/` (authoritative for
    technical reference) + `content/ko/` (authoritative for
    Korean-specific operations — Naver setup, mecab-ko, Korean
    analyst persona). Parallel folder structure; Pagefind indexes
    both locales.
  - 7-section IA: Introduction / Getting Started / End Users /
    Operators / Reference (CLI / API / MCP / Skill / Adapters
    cross-link) / Troubleshooting / Legal. Hierarchy detailed in
    REQ-DOC-003.
  - `.github/workflows/docs.yml` CI workflow: (a) Nextra build,
    (b) `lychee` link-check (internal + external with rate-limit
    polite settings), (c) screenshot freshness (image age ≤ 90
    days for UI screenshots), (d) bilingual coverage gate (KO
    folder has ≥ 90% page parity with EN excluding `reference/cli`
    + `reference/api` which are EN-only by D3), (e) static export +
    gh-pages publish on main branch, (f) Docker image build
    targeting `ghcr.io/<org>/usearch-docs:<tag>` for air-gapped
    deployments.
  - `scripts/gen-cli-reference.sh`: auto-extracts `usearch --help`
    output from the built binary into `content/en/reference/cli/
    *.mdx`. Pattern mirrors `scripts/gen-deps-manifest.sh`
    convention. Re-runs on every CLI subcommand change.
  - `Dockerfile.docs`: multi-stage build (Node 22 + pnpm → Nextra
    static export → Caddy/nginx serve). Image targets self-hosted
    operators who cannot reach gh-pages.
  - `docs/content/en/CONTRIBUTING.md` (public-facing — distinct
    from any internal `CONTRIBUTING.md` at repo root): docs
    contribution workflow, content style guide (EN), translation
    workflow.
  - `docs/content/ko/CONTRIBUTING.md`: KO contribution workflow,
    mecab-ko-aware style notes, KO-specific terminology glossary.

  Pinned decisions (5개 scope pillar D1..D5 + 보조 D6..D8):

  (D1) **Site framework — Nextra v4.x on Next.js 16, standalone
       app under `docs/`**. roadmap.md:112 explicitly names
       "Nextra"; v4 (released 2026-Q1 per nextra.site) is current
       stable with Pagefind-based zero-config search + file-system
       i18n + RSC support — all features V1 needs without external
       dependencies. Standalone app (separate from `web/` query
       UI) because: (a) different release cadence (docs publish
       independently of UI), (b) different runtime (docs is
       static-exported; UI is dynamic with SSE), (c) different
       theme (docs uses Nextra Docs Theme; UI uses shadcn/ui per
       SPEC-UI-001). Anti-decision: Docusaurus (heavier deps,
       slower build, no first-class RSC), MkDocs (Python toolchain
       conflict with Go + Node stack), VitePress (Vue ecosystem
       drift from React + Next stack), hand-rolled (re-invention).

  (D2) **Content sourcing — three-pillar strategy**:
       - **PRESERVE**: existing assets imported via file copy +
         frontmatter wrap (no content rewriting in V1 phase 1).
         Sources: README quickstart → `getting-started/quickstart`,
         product.md → `introduction/what-is-usearch` +
         `introduction/personas`, tech.md → `reference/architecture`,
         dependencies.md → `reference/dependencies` (link to
         auto-gen), MCP_OAUTH_SETUP.md → `operators/auth-setup`,
         ops/security/* → `operators/security/*`.
       - **GENERATE**: from completed SPECs via scripts.
         `scripts/gen-cli-reference.sh` reads `usearch --help` per
         subcommand → MDX. CLI commands documented in `reference/
         cli/{query,config,history,deep,sources,login,repl}.mdx`
         (7 subcommands per `cmd/usearch/root.go`). MCP
         tool catalog generated from SPEC-MCP-001 OpenAPI/JSON
         schema if available (else hand-curated initial draft).
       - **HAND-WRITE**: net-new narrative content with no source.
         `getting-started/first-query.mdx`, `end-users/cli-tour.mdx`,
         `end-users/web-ui-tour.mdx`, `end-users/skill-claude.mdx`,
         `operators/deployment-helm.mdx` (cross-links to
         SPEC-DEPLOY-001), `troubleshooting/*.mdx`, `legal/security.
         mdx` (responsible disclosure — references SEC-001 V14
         SECURITY.md), all FAQ entries. Authors: manager-docs
         agent generates initial drafts; human approval gate per
         §3 annotation cycle.

  (D3) **Localization workflow — bilingual EN+KO, parallel
       content trees**:
       - Site is bilingual EN ↔ KO per product.md §3 Korean
         analyst persona + roadmap M3 exit criterion ("Korean
         query returns Naver results ranked first") — V1 docs
         site MUST be usable end-to-end in Korean for Korean
         operators.
       - File structure: Nextra's recommended `content/{locale}/`
         pattern. `content/en/` and `content/ko/` parallel trees;
         Nextra v4 file-system i18n auto-routes `/en/...` and
         `/ko/...`. Default locale: EN (covers majority of
         operator persona). Locale switcher in top nav.
       - Content authority: **EN is authoritative for technical
         reference** (`reference/cli`, `reference/api`,
         `reference/mcp`) — these auto-generate from English source
         (`--help` output, OpenAPI specs) and are NOT translated to
         KO in V1 (would create drift; future SPEC may add KO
         CLI help). **KO is authoritative for Korean-specific
         operational content** (`operators/korean-locale-setup`,
         mecab-ko troubleshooting, Naver API key acquisition) —
         these may not have an EN counterpart at V1.
       - Translation strategy: Phase 1 (V1.0.0) ships
         hand-translated KO for: Introduction (all pages), Getting
         Started (all pages), End Users (CLI/Web/Skill/MCP tour
         narratives), Operators core path (deployment, security
         basics, auth setup), Troubleshooting (top-10 issues).
         Tier 2 KO (deferred to V1.1 minor releases): full reference
         section translation, edge-case operator runbooks.
       - Coverage gate: REQ-DOC-016 enforces 90% page parity
         (KO pages exist for ≥ 90% of EN pages excluding the
         EN-only reference auto-gen subtree).
       - Anti-decision: machine translation (LLM-based) for V1.
         Korean is HARD requirement; MT quality is insufficient
         for the operator-trust contract. Translation is a
         human authoring task by manager-docs + native-speaker
         review.

  (D4) **Search — Nextra v4 built-in (Pagefind, static, zero-
       config)**. Pagefind indexes content at build time, produces
       static `_pagefind/` directory consumed by Nextra's search
       UI. No runtime dependency; works in air-gapped self-hosted
       deployment (REQ-DOC-011). Anti-decision: Algolia DocSearch
       (cloud dependency, conflicts with self-hosted ethos — same
       rationale as SPEC-SEC-001 D2 secret-scanner choice;
       Algolia would require committing API keys to public docs
       site). FlexSearch (Nextra v3 default) deprecated in favour
       of Pagefind in v4. Multi-locale search: Pagefind handles
       EN + KO indexes separately, switches with locale toggle.

  (D5) **CI/build gates + deployment target**:
       - Build gate: `next build` (static export) zero errors,
         zero new warnings vs main baseline.
       - Link-check: `lycheeverse/lychee-action@v2` with
         `lychee.toml` config — internal links MUST be 100%
         resolvable (broken internal link = CI fail); external
         links polled at most weekly (cached) with retry +
         403/429 exception list; broken external link = CI warn
         (not fail) to avoid third-party flakiness blocking
         merges. Allowlist for known-rate-limited domains.
       - Screenshot freshness: `scripts/check-screenshot-
         freshness.sh` scans `content/**/*.mdx` for `<img src=
         "/screenshots/..."` references, asserts file mtime ≤ 90
         days for UI screenshots tagged `screenshot:ui:*`.
         Documentation-only diagrams (`screenshot:diagram:*`) are
         exempt. Stale screenshot = CI warn + GitHub Issue auto-
         creation tagged `docs/stale-screenshot`.
       - Bilingual coverage gate per REQ-DOC-016.
       - Deployment dual:
         (a) **gh-pages**: GitHub Actions `actions/upload-pages-
             artifact@v3` + `actions/deploy-pages@v4` on main
             branch push. Static site at `https://<org>.github.io/
             universal-search/`. Public-facing canonical URL.
         (b) **Docker image**: `Dockerfile.docs` multi-stage
             (Node 22 build → Caddy serve) published to `ghcr.io/<
             org>/usearch-docs:<tag>` matching usearch binary
             release tag. Air-gapped self-hosted operators pull
             this image alongside usearch binary; same Helm chart
             (SPEC-DEPLOY-001) provisions both.
       - Anti-decisions: Vercel (vendor lock-in, no air-gapped
         option), Netlify (same), ReadTheDocs (Python toolchain,
         opinionated on Sphinx). gh-pages + Docker covers public
         + private operators with zero SaaS dependency.

  (D6) **Versioning policy — single live version per major,
       git-tag for historical**: V1.x ships single "latest" docs
       site. Docs follow usearch semver: minor version updates
       (V1.1, V1.2) update the same docs site; patch updates
       (V1.0.1, V1.0.2) typically do NOT modify docs except
       for known-bug entries. Historical versions accessible via
       `git checkout v0.x.y && cd docs && pnpm build` — no
       multi-version live site in V1. Nextra v4 has versioning
       support but adds complexity (multi-version manifests,
       cross-version links); deferred to V2 if user demand emerges.
       SPEC-REL-001 release notes reference the docs URL at the
       tag point.

  (D7) **Accessibility — WCAG 2.1 AA baseline**: docs site
       inherits the SPEC-UI-001 a11y commitment. Nextra v4 default
       theme is WCAG 2.1 AA compliant out-of-box per upstream
       claim (research.md §6); custom components (theme tweaks,
       admonitions) MUST preserve compliance. CI gate via Pa11y
       or axe-core on a sampled page set (deferred — V1 manual
       audit at the V1.0.0 freeze gate per REQ-DOC-017).

  (D8) **Content style + tone**: technical-precise, screenshot-
       supported where UI is involved. NO marketing copy without
       a source-grounded claim ("V1 supports Korean queries" OK
       because product.md §6 commits a Korean SLO; "the best
       search engine" NOT OK — no measurable claim). REQ-DOC-018
       (Unwanted pattern) enforces this as an Anti-pattern. Tone
       guide derived from `.moai/project/brand/brand-voice.md`
       if present (template currently `_TBD_` per SPEC-UI-001
       HISTORY); V1 ships with neutral-technical default tone
       and updates when brand-voice ships.

  Companion artifacts:
  - `.moai/specs/SPEC-DOC-001/research.md` — Phase 0.5 research
    (Nextra v4 architecture, alternative SSG comparison, i18n
    approach evaluation, search tooling, link-check tooling,
    deployment targets, comparable OSS doc sites audit).
  - `.moai/specs/SPEC-DOC-001/plan.md` — DDD phased plan
    (ANALYZE existing surface inventory → PRESERVE migration map
    → IMPROVE Nextra IA + bilingual + CI gates).

  18 EARS REQs (10 × P0 + 6 × P1 + 2 × P2) + 7 NFRs + 1 new Next.js
  sub-app + 1 new CI workflow + 2 new scripts + 1 new Dockerfile.
  Methodology: **DDD** (consolidation of existing assets — byte-
  fidelity preservation, Nextra-shaped IMPROVE). Coverage target
  85% applies to scripts + build pipeline (gen-cli-reference.sh
  + lychee config + bilingual gate script); MDX content is
  measured by completeness percentage not test coverage. Harness:
  **standard** (P1 docs SPEC, no security domain involvement —
  Sprint Contract RECOMMENDED but not required per `.claude/rules/
  moai/design/constitution.md` §11). Owner: manager-docs.

---

## 1. Overview

SPEC-DOC-001은 M9 (V1 release)의 첫 번째 SPEC이자 V1.0.0 tagging의
release-blocking dependency다. 본 SPEC은 **새로운 콘텐츠 시스템을
발명하지 않으며**, 11개 완료/예정 SPEC의 README, runbook, configuration
docs를 **(a) consolidation, (b) operator + end-user 내러티브 gap
closure, (c) Nextra 사이트로 publish**의 세 축으로 정리한다.

### 1.1 What ships

| Layer | Artifact | Purpose |
|-------|----------|---------|
| App | `docs/` Next.js + Nextra v4 standalone app (NEW) | bilingual EN+KO docs site, separate from `web/` query UI |
| Content | `docs/content/en/**/*.mdx` (NEW) | EN authoritative content (introduction, getting-started, end-users, operators, reference, troubleshooting, legal) |
| Content | `docs/content/ko/**/*.mdx` (NEW) | KO content (V1.0.0 coverage ≥ 90% of EN per D3) |
| Content | `docs/content/{en,ko}/reference/cli/*.mdx` (auto-generated) | CLI subcommand reference from `usearch --help` output |
| CI | `.github/workflows/docs.yml` (NEW) | build + link-check + screenshot-freshness + bilingual-coverage + dual deploy |
| Script | `scripts/gen-cli-reference.sh` (NEW) | extracts `usearch --help` per subcommand → MDX |
| Script | `scripts/check-screenshot-freshness.sh` (NEW) | mtime check on `screenshot:ui:*`-tagged images |
| Script | `scripts/check-bilingual-coverage.sh` (NEW) | EN ↔ KO parity validation per REQ-DOC-016 |
| Config | `docs/lychee.toml` (NEW) | link-check rules: internal strict, external retry+allowlist |
| Build | `Dockerfile.docs` (NEW) | multi-stage Node 22 → Caddy/nginx static serve |
| Deploy | gh-pages site at `https://<org>.github.io/universal-search/` | public canonical URL |
| Deploy | `ghcr.io/<org>/usearch-docs:<tag>` container image | air-gapped self-hosted operators |
| Docs | `docs/content/en/CONTRIBUTING.md`, `docs/content/ko/CONTRIBUTING.md` (NEW) | contribution + translation workflow |
| Migration | `README.md` (modified) | retain quickstart; add prominent docs site link at top |
| Migration | `.moai/docs/MCP_OAUTH_SETUP.md` (preserved + cross-linked) | move content to `docs/content/en/operators/auth-setup.mdx`; original retained as agent-facing memo |
| Forward-ref | `ops/security/runbook.md`, `owasp-asvs-checklist.md`, `threat-model.md` (NOT on main — SEC-001 PR #42 unmerged) | `docs/content/en/operators/security/*` are placeholders linking SPEC-SEC-001 status; cross-link to `ops/security/*` deferred until SEC-001 merges |

### 1.2 Motivation

V1 release ("usearch v1.0.0" tag in SPEC-REL-001) 직전 docs 부재는
**onboarding-blocking**이다. 한 번 release된 binary가 외부 사용자
환경에 배포되면:

- 자체 호스팅 operator가 `helm install usearch` (SPEC-DEPLOY-001)을
  실행할 때 secret 관리 (SPEC-SEC-001 D5), OIDC provider 연동
  (SPEC-AUTH-001), adapter API key acquisition (SPEC-DOC-002
  scope), Korean tokenizer setup (SPEC-IDX-003) 각 단계를
  문서 없이 시행착오로 학습해야 함.
- end-user가 `usearch query` (SPEC-CLI-001 implemented) 또는
  Web UI (SPEC-UI-001) 또는 Claude Skill (SPEC-SKILL-001) 또는
  MCP server (SPEC-MCP-001) 중 어떤 surface가 자신의 사용 사례에
  맞는지 비교 기준이 없음.
- Korean analyst persona (product.md §3)가 Korean-locale 기능
  (mecab-ko + Naver suite + Korean RSS — SPEC-IDX-003, ADP-008,
  ADP-009)이 존재한다는 사실을 발견할 채널이 없음.
- 보안 incident 발생 시 (SPEC-SEC-001 D2 committed-secret 시나리오)
  ops/security/runbook.md는 internal repo 경로에 묻혀 있어 외부
  사용자가 접근 불가.

본 SPEC이 **PASS**해야 하는 이유: M9 exit criterion "docs site
live" (`roadmap.md:157`)가 만족되지 않으면 SPEC-REL-001 V1.0.0
tagging 차단. SPEC-DOC-002 (adapter reference) 가 본 SPEC의
사이트 IA에 의존 (`reference/adapters/...`). SPEC-DEPLOY-001
Helm chart는 본 SPEC의 `operators/deployment-helm.mdx`로
사용자를 가이드.

### 1.3 Forward-compatibility commitments

본 SPEC은 다음 sibling/downstream SPEC과의 contract를 명시한다:

- **SPEC-DOC-002 (M9 sibling, not yet drafted)** — adapter
  reference (per-adapter keys, rate limits, Korean tokenizer
  setup, troubleshooting). 본 SPEC의 IA에 `reference/adapters/`
  슬롯을 reserve하며, DOC-002가 해당 슬롯의 콘텐츠를 채운다.
  본 SPEC은 DOC-002의 콘텐츠 형식 (per-adapter MDX template)을
  정의하지 않는다 — DOC-002 owner의 결정 사항.
- **SPEC-DEPLOY-001 (M9, not yet drafted)** — Helm chart for
  k8s deploy. 본 SPEC의 `operators/deployment-helm.mdx`가
  DEPLOY-001 Helm values reference + install walk-through의
  user-facing 표면. DEPLOY-001은 본 SPEC의 docs container image
  (`ghcr.io/<org>/usearch-docs`)를 Helm chart의 optional
  subchart로 포함하여 self-hosted operators가 동일 chart로
  binary + docs를 동시 배포할 수 있게 한다.
- **SPEC-REL-001 (M9, not yet drafted)** — V1.0.0 tag + release
  notes. 본 SPEC PASS는 REL-001의 "docs site live" exit gate.
  REL-001 release notes가 docs canonical URL을 cite.
- **SPEC-CLI-001 (implemented), SPEC-CLI-002 (drafted)** — CLI
  reference (`reference/cli/*.mdx`)가 `scripts/gen-cli-
  reference.sh`로 두 SPEC의 binary `--help` output에서 자동
  생성. CLI-002의 새 subcommand 추가는 docs 사이트 reference에
  자동 반영 (CLI-002 sync phase가 본 스크립트를 invoke).
- **SPEC-UI-001 (drafted)** — Web UI tour content (`end-users/
  web-ui-tour.mdx`)가 UI-001의 4개 route에 대한 스크린샷 + 사용
  narrative를 제공. 스크린샷은 `screenshot:ui:*` tag 적용 →
  90일 freshness gate (REQ-DOC-014).
- **SPEC-MCP-001 (drafted), SPEC-SKILL-001 (drafted)** — MCP
  tool 목록 + Skill installation은 두 SPEC의 implementation 완료
  후 V1 cycle 내에 docs에 추가됨. V1.0.0 docs ship에는 두 SPEC의
  최종 형태를 반영한 narrative + reference 포함.
- **SPEC-AUTH-001/002/003 (implemented)** — `.moai/docs/MCP_
  OAUTH_SETUP.md`가 `operators/auth-setup.mdx`로 마이그레이션;
  RBAC + audit log 운영 가이드가 `operators/team-rbac.mdx`로
  신규 작성 (AUTH-002/003 implementation을 reference).
- **SPEC-OBS-001 (implemented)** — Prometheus + slog + OTel
  운영 가이드가 `operators/observability.mdx`로 신규 작성.
  metric 카탈로그는 OBS-001의 cardinality allowlist에서 추출.
- **SPEC-SEC-001 (M8, drafted — PR #42 UNMERGED, NOT on main)** —
  `ops/security/runbook.md` + `owasp-asvs-checklist.md` +
  `threat-model.md`는 현재 main에 존재하지 않는다. 따라서 docs
  `operators/security/` 서브트리는 V1.0.0 ship 시점에
  **forward-reference**: placeholder + SPEC-SEC-001 상태 링크만
  surface하고, SEC-001 머지 전까지 존재하지 않는 `ops/security/*`
  파일로의 cross-link은 **금지** (link-check CI fail 방지 —
  REQ-DOC-005 + REQ-DOC-013 참조). SEC-001 머지 후 별도 PR에서
  placeholder를 MDX wrapper + locale context로 교체하여 canonical
  `ops/security/*` 파일을 cross-link한다.
- **SPEC-EVAL-001/002/003 (M8 sibling)** — eval methodology +
  Korean benchmark protocol을 `reference/evaluation.mdx`로 surface
  (선택적, V1.0.0 시점 우선순위 낮음).

### 1.4 Pinned architectural decisions

HISTORY의 D1..D8 8개 결정은 §2 requirements를 bind하는 constraint이다.
재논의 대상이 아니며, annotation cycle에서만 modification 가능.

---

## 2. EARS Requirements

### 2.1 Site Framework + Bootstrap (D1)

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-DOC-001** | Ubiquitous | The repository SHALL contain a standalone Next.js + Nextra v4 documentation application under `docs/` at the repo root, distinct from the `web/` query UI application. BECAUSE `docs/` already holds stale Nextra v3 / Next 14 build artifacts (gitignored `.next/`, `out/`, `node_modules/`, v3-convention `pages/`, scratch `content/`), the bootstrap SHALL FIRST remove these v3 artifacts AND SHALL preserve the git-tracked dependency docs (`docs/dependencies.md`, `docs/_deps-*.md`, `docs/licenses/`). The docs app SHALL pin Next.js `^16.2.6` (matching `web/package.json`), use the `nextra-theme-docs` theme, `pnpm` as the package manager (matching `web/`), Node.js 22 LTS, and TypeScript 5+ strict mode. The app SHALL use the Nextra v4 `content/{locale}/` content convention (NOT the v3 `pages/` convention). `docs/package.json` SHALL declare a `build`, `dev`, `lint`, and `start` script. The static export output SHALL be written to `docs/out/`. | P0 | Stale `docs/pages/` (v3) is absent after bootstrap; git-tracked `docs/dependencies.md` + `docs/licenses/` still present; `docs/package.json` exists with declared scripts and Next `^16.2.6`; `pnpm --dir docs install` succeeds; `pnpm --dir docs build` produces `docs/out/index.html`; the app boots at `pnpm --dir docs dev` and serves the introduction page. |
| **REQ-DOC-002** | Ubiquitous | The Nextra docs app SHALL be configured for static export (`output: 'export'` in `next.config.mjs`) AND file-system based i18n with locales `en` (default) and `ko`. The locale switcher SHALL appear in the top navigation. URL routing SHALL be `/en/<path>` and `/ko/<path>` with `/` redirecting to `/en/` by default. The Pagefind-based zero-config search SHALL be enabled and SHALL index both locale subtrees separately, switching index based on the active locale. | P0 | `docs/next.config.mjs` contains `output: 'export'` AND i18n config with `locales: ['en', 'ko']`; site navigation includes a working locale switcher; search bar functional on both `/en/` and `/ko/` paths; building produces separate `_pagefind/` indexes per locale. |

### 2.2 Information Architecture + Content Sections (D2)

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-DOC-003** | Ubiquitous | The docs site SHALL implement a 7-section top-level information architecture: (1) Introduction (`/introduction/`), (2) Getting Started (`/getting-started/`), (3) End Users (`/end-users/`), (4) Operators (`/operators/`), (5) Reference (`/reference/`), (6) Troubleshooting (`/troubleshooting/`), (7) Legal (`/legal/`). Each section SHALL have a landing page (`index.mdx`) plus the subpages enumerated in §7.1. The sidebar SHALL render this hierarchy with collapsible groups; the active page MUST be highlighted. | P0 | All 7 sections exist as folders under both `content/en/` and `content/ko/`; each has an `index.mdx`; sidebar navigation renders the hierarchy correctly; clicking any section reveals its subpages. |
| **REQ-DOC-004** | Event-Driven | WHEN a user (new operator persona per `.moai/project/product.md` §3) lands on the Getting Started landing page, the docs SHALL present a 30-minute path to first successful query covering: (a) prerequisites checklist (Docker / Go / Node / Python — sourced from `README.md` prerequisites table), (b) `make compose-up` + `make build` walkthrough, (c) first `usearch query "hello"` execution and expected output, (d) troubleshooting links for the three most common first-run failures (compose port conflict, missing env vars, LLM provider unconfigured). The path SHALL be navigable via "Next" buttons at the bottom of each page. | P0 | Getting Started landing page exists; the four sub-pages exist in order; "Next" buttons present and functional; manual trace test asserts a new user can reach a successful first query following only this section. |
| **REQ-DOC-005** | Ubiquitous | The Operators section SHALL contain at minimum the following sub-pages: `deployment-helm.mdx` (cross-links SPEC-DEPLOY-001), `auth-setup.mdx` (sourced from `.moai/docs/MCP_OAUTH_SETUP.md` + SPEC-AUTH-001), `team-rbac.mdx` (SPEC-AUTH-002), `audit-log.mdx` (SPEC-AUTH-003), `observability.mdx` (SPEC-OBS-001), `security/runbook.mdx`, `security/owasp-checklist.mdx`, `security/threat-model.mdx`. BECAUSE the `ops/security/*` canonical files do NOT yet exist on main (SPEC-SEC-001 PR #42 unmerged), the three `security/*` pages SHALL be FORWARD-REFERENCE placeholders at V1.0.0: each links to SPEC-SEC-001 status and SHALL NOT cross-link any `ops/security/*` path until SEC-001 merges. [HARD] No docs page SHALL emit a link to a nonexistent `ops/security/*` file — such links would break the link-check gate (REQ-DOC-013). On SEC-001 merge, a follow-up PR replaces the placeholders with MDX wrappers cross-linking the now-present canonical files. Each operator page SHALL include a "Last updated" date in frontmatter AND a "Related SPECs" footer linking the underlying SPEC IDs. | P0 | All listed operator pages exist in `content/en/operators/`; the three `security/*` pages are placeholders linking SPEC-SEC-001 (no `ops/security/*` link present); each page has frontmatter with `lastUpdated` date; each page footer renders "Related SPECs" with valid SPEC ID links; link-check passes (no broken `ops/security/*` reference). |
| **REQ-DOC-006** | Ubiquitous | The End Users section SHALL contain at minimum: `cli-tour.mdx` (SPEC-CLI-001 + SPEC-CLI-002 narrative), `web-ui-tour.mdx` (SPEC-UI-001 screenshots + narrative), `skill-claude.mdx` (SPEC-SKILL-001 install + usage), `mcp-integration.mdx` (SPEC-MCP-001 host configuration for Claude Code / Codex / Gemini CLI), and `surface-comparison.mdx` (when-to-use-which decision matrix). Each surface page SHALL include at least one screenshot or terminal cast tagged appropriately for screenshot-freshness gating per REQ-DOC-014. | P0 | All listed end-user pages exist in `content/en/end-users/`; each contains at least one tagged screenshot reference; manual review confirms screenshots match the current UI/CLI state. |

### 2.3 Reference + Auto-generation (D2)

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-DOC-007** | Ubiquitous | The repository SHALL provide `scripts/gen-cli-reference.sh` that builds the `usearch` binary (via `make build` if not present), invokes `usearch --help` AND `usearch <subcommand> --help` for each subcommand defined in SPEC-CLI-001 + SPEC-CLI-002 implemented set, captures the output, AND writes one MDX file per subcommand to `docs/content/en/reference/cli/{subcommand}.mdx`. The current implemented subcommand set (per `cmd/usearch/root.go`) is **7 subcommands: `query`, `config`, `history`, `deep`, `sources`, `login`, `repl`** — the script SHALL generate one MDX per subcommand AND auto-track additions/removals as the CLI surface evolves (no hardcoded subcommand list). Each MDX file SHALL include frontmatter `{title, generated: true, source: "usearch --help", lastGenerated: <ISO-8601 timestamp>}` AND the help text rendered inside an MDX `<CodeBlock>` component. The CI workflow SHALL invoke this script on every push to `main` AND assert the generated content matches the committed files (drift = CI fail). | P0 | `scripts/gen-cli-reference.sh` exists and is executable; running it against the current binary produces files for all 7 implemented subcommands (`query`, `config`, `history`, `deep`, `sources`, `login`, `repl`); CI workflow contains a `gen-cli-reference` job that fails when committed reference drifts from generated output. |
| **REQ-DOC-008** | Optional | WHERE SPEC-DOC-002 (adapter reference) has shipped, the docs site SHALL include a `reference/adapters/` subtree owned by SPEC-DOC-002 with cross-links from `end-users/surface-comparison.mdx` (intent router category routing) AND `operators/deployment-helm.mdx` (per-adapter env-var setup). The IA slot SHALL be reserved at SPEC-DOC-001 ship time with a placeholder landing page `reference/adapters/index.mdx` linking to SPEC-DOC-002 status. The placeholder SHALL be replaced by SPEC-DOC-002 content; SPEC-DOC-001 does NOT pre-author per-adapter content. | P1 | `reference/adapters/index.mdx` exists with placeholder + link to SPEC-DOC-002; cross-links present in surface-comparison and deployment-helm; merging SPEC-DOC-002 does not require modifying SPEC-DOC-001 IA. |
| **REQ-DOC-009** | Ubiquitous | The Troubleshooting section SHALL contain at minimum top-10 entries derived from: SPEC-CACHE-001 5-phase fallback failure modes, SPEC-AUTH-001 OIDC discovery failures, SPEC-SEC-001 SSRF block triage, SPEC-IDX-003 Korean tokenizer health-check failures, LLM provider connection errors (SPEC-LLM-001), Docker compose port conflicts (SPEC-BOOT-001), missing env vars (SPEC-BOOT-001), Qdrant/Meilisearch/Postgres connectivity (SPEC-IDX-001), adapter API key acquisition (placeholder linking SPEC-DOC-002), AND rate-limit-exceeded errors (SPEC-SEC-001 ratelimit). Each entry SHALL follow the format: Symptom → Likely Cause → Diagnostic Command → Resolution → Related SPEC IDs. | P1 | `troubleshooting/index.mdx` exists; at least 10 entries each following the 5-field format; each entry has at least one Related SPEC ID with valid link. |

### 2.4 Localization (D3)

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-DOC-010** | Ubiquitous | The docs site SHALL ship with hand-translated KO content for the following Tier-1 pages at V1.0.0: all Introduction subpages, all Getting Started subpages, all End Users subpages, Operators core path (deployment-helm, auth-setup, team-rbac, security/runbook), AND Troubleshooting top-10 entries. KO content SHALL be authored by manager-docs agent AND reviewed by a native-Korean-speaking reviewer (recorded in `docs/content/ko/CONTRIBUTING.md` reviewer log). The `content/ko/operators/korean-locale-setup.mdx` page (mecab-ko + Naver tokenizer specifics) SHALL be KO-authoritative — an EN counterpart MAY exist but the KO version is the source of truth for Korean-locale operational specifics. | P0 | Tier-1 KO pages exist in `content/ko/`; reviewer log in `content/ko/CONTRIBUTING.md` shows at least one named reviewer per Tier-1 page batch; `content/ko/operators/korean-locale-setup.mdx` marked authoritative. |
| **REQ-DOC-011** | Ubiquitous | The docs site SHALL provide built-in search via Nextra v4 Pagefind integration with NO external SaaS dependency (NO Algolia DocSearch). Search SHALL index all `*.mdx` content in the active locale, return results within 500ms p95 on a mid-tier dev laptop (`pnpm --dir docs dev` benchmark), AND support Korean text queries (Pagefind UTF-8 multi-byte support — verified against `content/ko/`). Search index files SHALL be served from the same origin as the docs site (no third-party requests at search time). | P0 | Search functional on dev server; search query for "Korean" returns relevant English results; search query "한국어" returns relevant Korean results; browser DevTools network tab confirms zero third-party requests during search interaction. |

### 2.5 CI Gates + Deployment (D5)

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-DOC-012** | Ubiquitous | The repository SHALL provide `.github/workflows/docs.yml` executing on every pull request affecting `docs/**`, `scripts/gen-cli-reference.sh`, `scripts/check-screenshot-freshness.sh`, OR `scripts/check-bilingual-coverage.sh`. The workflow SHALL contain jobs: (a) `build` — `pnpm --dir docs build` zero errors, (b) `link-check` — lychee link-check per REQ-DOC-013, (c) `gen-reference-drift` — gen-cli-reference drift check per REQ-DOC-007, (d) `screenshot-freshness` — per REQ-DOC-014, (e) `bilingual-coverage` — per REQ-DOC-016. The workflow SHALL complete within 5 minutes wall-clock on `ubuntu-24.04` hosted runner per NFR-DOC-001. | P0 | `.github/workflows/docs.yml` exists with all 5 jobs; CI run on a PR modifying `docs/content/en/introduction/index.mdx` executes all jobs; total runtime ≤ 5 min observed. |
| **REQ-DOC-013** | Event-Driven | WHEN the docs CI workflow runs, the `link-check` job SHALL execute `lycheeverse/lychee-action@v2` using `docs/lychee.toml` config. Internal links (relative MDX references, internal anchor links, intra-site routes) SHALL be 100% resolvable — ANY broken internal link SHALL fail the job. To keep the gate green while SPEC-SEC-001 is unmerged, the docs SHALL NOT emit any link to a nonexistent `ops/security/*` file (per REQ-DOC-005 forward-reference rule); the security placeholders link to in-site SPEC-SEC-001 status pages only. Should any deferred/forward-reference path be unavoidable, it SHALL be added to a `docs/lychee.toml` exclude allowlist with a comment naming the unblocking SPEC — the link-check job MUST NOT fail on these deferred references. External links (https://...) SHALL be polled with retry (3 attempts, exponential backoff) AND configurable allowlist for known-rate-limited domains (github.com API, twitter/x.com, anthropic.com); broken external link SHALL warn (not fail) the job AND post an annotation to the PR. The external-link allowlist SHALL be reviewed quarterly. | P0 | `docs/lychee.toml` exists with internal-strict + external-warn config + a deferred-reference exclude section (SEC-001 `ops/security/*` until merged); injected broken internal link `[bad](./does-not-exist)` fails CI; security placeholder pages do NOT produce a broken-link failure; injected unreachable external link `https://this-does-not-exist.invalid` produces PR annotation but does not fail; allowlisted domain (`https://api.github.com/`) returning 403 does NOT fail. |
| **REQ-DOC-014** | Event-Driven | WHEN the docs CI workflow runs, the `screenshot-freshness` job SHALL execute `scripts/check-screenshot-freshness.sh`. The script SHALL scan all MDX files for image references AND parse a frontmatter or sibling-metadata tag classifying each image as `screenshot:ui:*`, `screenshot:terminal:*`, `screenshot:diagram:*`, OR untagged. Images tagged `screenshot:ui:*` OR `screenshot:terminal:*` SHALL have file mtime ≤ 90 days OR a `lastVerified` date in MDX frontmatter ≤ 90 days. Stale images SHALL produce a CI warning AND auto-create a GitHub Issue tagged `docs/stale-screenshot` referencing the affected page. Diagrams (`screenshot:diagram:*`) and untagged images are exempt. | P1 | Script exists; UI screenshot with mtime > 90 days produces warning + Issue; freshly-updated `lastVerified` overrides mtime check; diagram tag exempts the image. |
| **REQ-DOC-015** | Ubiquitous | The docs CI workflow SHALL, on every push to `main`, deploy the **V1 gate target**: (a) gh-pages via `actions/upload-pages-artifact@v3` + `actions/deploy-pages@v4` to `https://<org>.github.io/universal-search/` (public canonical URL). The workflow SHALL ALSO contain a `build-and-push-container` job that builds the image from `Dockerfile.docs` (multi-stage Node 22 build → Caddy serve, image size ≤ 100 MB per NFR-DOC-004); HOWEVER the actual **publish** to `ghcr.io/<org>/usearch-docs:<sha>` / `:latest` (and `:v1.x.y` on tagged releases) is **DEFERRED and conditional** on resolution of the `<org>` / container-registry path (Open Questions §8.3, §8.4). Until resolved, the container job SHALL build-and-verify-only (no push) — image push is NOT a V1.0.0 ship gate. | P0 | On main push, gh-pages deploys (V1 gate). The `build-and-push-container` job builds `Dockerfile.docs` successfully and `docker run -p 8080:80 <local-image>` serves the docs index; the push step is gated behind a resolved-registry condition and does NOT block V1 when `<org>` is unresolved. |
| **REQ-DOC-016** | State-Driven | IF the `bilingual-coverage` CI job runs, THEN the script `scripts/check-bilingual-coverage.sh` SHALL enumerate the **Tier-1 EN page set** (all Introduction, all Getting Started, all End Users, Operators core path, Troubleshooting top-10 — per REQ-DOC-010) and assert `≥ 90%` of corresponding KO paths exist under `docs/content/ko/`. The entire `reference/` subtree is EXCLUDED from the V1 gate: `reference/cli/` + `reference/api/` are EN-only per D3, AND full reference-section KO translation is **DEFERRED to V1.1** (not part of the V1 Tier-1 coverage gate). The script SHALL produce a coverage report (`docs/coverage-report.md`) listing missing KO translations AND the V1.1-deferred reference subtree (informational, not counted). Tier-1 coverage `< 90%` SHALL fail the job. The 90% Tier-1 threshold SHALL NOT be lowered without explicit SPEC amendment. | P0 | Script exists; coverage report generated on every CI run; the gate counts only the Tier-1 set; deliberate removal of one KO page in the Tier-1 set drops coverage below threshold and fails CI; removal of a `reference/` KO page does NOT affect the gate; report lists the specific missing Tier-1 KO paths. |

### 2.6 Accessibility + Content Quality (D7, D8)

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-DOC-017** | Ubiquitous | The docs site SHALL maintain WCAG 2.1 AA accessibility baseline. Nextra v4 default theme provides this out-of-box; custom theme tweaks (color overrides, custom admonition components, code-block themes) SHALL preserve AA contrast ratios (≥ 4.5:1 for normal text, ≥ 3:1 for large text). A manual a11y audit SHALL be performed at the V1.0.0 freeze gate (pre-release) using axe-core browser extension OR equivalent; audit results SHALL be recorded in `docs/content/en/legal/accessibility.mdx`. Automated CI-time a11y testing (Pa11y / axe-core CLI) is deferred to V1.1. | P1 | At V1.0.0 freeze gate, accessibility audit report present in `legal/accessibility.mdx` with axe-core findings + remediation status; manual color-contrast check on default theme reveals zero violations. |
| **REQ-DOC-018** | Unwanted | The docs site SHALL NOT include marketing copy that makes claims not grounded in `.moai/project/product.md` SLOs OR cited external sources. Phrases such as "the best search engine", "industry-leading", "fastest", "most accurate" are PROHIBITED unless quantified by a product.md SLO (e.g., "≥ 0.85 citation faithfulness per `.moai/project/product.md` §6"). Comparison claims against competitors (Perplexity, GPT Researcher, SearXNG) SHALL cite the product.md §7 differentiation matrix verbatim. A lint script `scripts/check-doc-claims.sh` (advisory, V1) SHALL grep for the prohibited phrase list AND warn (not fail) on match — manual review required. | P2 | `scripts/check-doc-claims.sh` exists with prohibited phrase list; running it against current docs produces zero matches (clean baseline); deliberate insertion of "the fastest search engine" in a draft page triggers warning. |

---

## 3. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| **NFR-DOC-001** | Docs CI runtime budget | The `.github/workflows/docs.yml` workflow SHALL complete within 5 minutes wall-clock on `ubuntu-24.04` hosted runner for the median PR (under 50 MDX file changes), with all jobs executed in parallel where dependencies allow (build → link-check + gen-reference-drift + screenshot-freshness + bilingual-coverage run in parallel after build). |
| **NFR-DOC-002** | Static export bundle size | The Nextra static export output under `docs/out/` SHALL be ≤ 50 MB total (HTML + JS + CSS + assets, excluding `_pagefind/` search index AND embedded screenshots). The Pagefind search index SHALL be ≤ 20 MB per locale. Total docs site size including assets SHALL be ≤ 100 MB. Violations SHALL produce a CI warning with size delta breakdown. |
| **NFR-DOC-003** | Page load performance | Docs pages SHALL achieve Lighthouse Performance score ≥ 90 on a mobile-first audit (default Lighthouse mobile config) for the introduction landing page AND at least one randomly-sampled deep reference page. Lighthouse audit SHALL be run manually at V1.0.0 freeze gate; results recorded in `docs/content/en/legal/performance.mdx`. Automated Lighthouse CI is deferred to V1.1. |
| **NFR-DOC-004** | Docs container image size | The `ghcr.io/<org>/usearch-docs:<tag>` container image (multi-stage build, Caddy serve final layer) SHALL be ≤ 100 MB compressed. The image SHALL pass `trivy image --severity HIGH,CRITICAL` scan with zero findings (chained to SPEC-SEC-001 D1 Trivy policy). |
| **NFR-DOC-005** | Link-check freshness | The lychee external-link cache (`.lycheecache`) SHALL be persisted in CI cache (`actions/cache@v4`) with a max age of 7 days. Cache invalidation SHALL trigger full external-link re-check on schedule (weekly cron on Sunday 02:00 UTC). |
| **NFR-DOC-006** | KO translation latency commitment | When a Tier-1 EN page is added or substantively modified (≥ 30 lines diff), a corresponding KO update SHALL be staged within the same minor release window (V1.x). Patch releases (V1.0.z) MAY ship EN-only updates; the next minor release closes the gap. Tracked in `docs/content/ko/CONTRIBUTING.md` translation backlog table. |
| **NFR-DOC-007** | Docs site availability (gh-pages) | The gh-pages-hosted docs site SHALL be reachable at the canonical URL with 99% monthly uptime (GitHub Pages SLA). For air-gapped operators, the container image (`ghcr.io/<org>/usearch-docs:<tag>`) is the primary surface and is operator-availability-owned. |

---

## 4. Exclusions (What NOT to Build)

[HARD] 다음 항목은 본 SPEC 범위에서 명시적으로 제외된다. 각 항목은
known destination, rationale, 또는 follow-up이 기록되어 있다.

- **Per-adapter documentation pages**. → SPEC-DOC-002 (M9 sibling)
  소유. 본 SPEC은 `reference/adapters/index.mdx` 슬롯만 reserve.

- **Translated marketing site / landing page**. → 본 SPEC은 docs
  site only. 마케팅 페이지 (homepage, product positioning,
  competitor comparison)는 V1 scope 외. product.md §7 differentiation
  matrix는 `introduction/comparison.mdx`로 surface하되 marketing
  tone 금지 (REQ-DOC-018).

- **Video tutorials / screencasts**. → V1은 정적 텍스트 + 스크린샷.
  video는 hosting cost + freshness 부담 + accessibility (자막 작업
  부재) 우려로 V1 제외. V1.1 이후 screencast가 가치 입증되면 별도
  SPEC.

- **Interactive code playgrounds**. → V1은 정적 코드 블록 (구문
  강조만). MDX interactive components (live REPL, embedded usearch
  CLI in browser via WASM)는 V2 검토.

- **Multi-version docs site (V0.x, V1.x simultaneously live)**. →
  D6 결정. V1 단일 latest 사이트. 과거 버전은 git tag checkout으로
  접근. 다중 버전 docs는 Nextra v4 versioning feature 활용하여
  V2에서 검토.

- **Machine-translated KO content**. → D3 결정. Korean은 HARD
  requirement; LLM-기반 MT 품질은 operator-trust 계약에 불충분.
  인간 번역 (manager-docs + native reviewer)만.

- **Additional locales beyond EN + KO (JP, ZH, ES, FR, ...)**. →
  V1 scope 외. product.md §3 personas는 Korean-locale + global
  English만 명시. 추가 locale 요구가 user 데이터로 입증되면 별도
  SPEC.

- **Algolia DocSearch integration**. → D4 anti-decision. self-hosted
  ethos 위반 + secret 관리 부담 + Vercel/Algolia lock-in. Pagefind
  사용.

- **Vercel / Netlify / ReadTheDocs 배포**. → D5 anti-decision.
  gh-pages + Docker container 이중 배포가 V1 commitment. SaaS
  hosting은 V2 검토 (vendor evaluation 필요).

- **Real-time search relevance tuning UI**. → Pagefind 기본 가중치
  사용. custom search ranking은 V1 scope 외.

- **Comment system / community forum integration**. → docs site는
  static + read-only. GitHub Discussions가 community surface
  (separate URL); docs는 link만 제공.

- **Auto-generated OpenAPI client docs**. → SPEC-MCP-001이 OpenAPI
  spec을 ship할 경우 V1.1에서 별도 reference 페이지. V1.0.0은
  MCP tool 목록만 hand-curated.

- **Analytics integration (Google Analytics, Plausible, Umami)**. →
  V1은 zero-tracking docs. self-hosted operators가 자체 analytics
  추가 가능 (Helm values exposure는 SPEC-DEPLOY-001 결정 사항).

- **Search-engine optimization (SEO) metadata + sitemap.xml**. →
  basic OG tags + Nextra default `<title>` + `<meta>`만. 본격적
  SEO (structured data, hreflang full implementation, sitemap
  submission)은 V1.1.

- **Print-friendly stylesheet / PDF export**. → V1 web-only. PDF
  export는 V2 검토 (printed reference manual 요구 입증 시).

- **Custom 404 page with smart suggestions**. → Nextra 기본 404만.
  검색 기반 smart 404 (Algolia-style)는 V1 제외.

- **GitHub Issue tracking on this SPEC** (`issue_number: 0`). → M8/
  M9 polish SPEC 패턴.

- **Automated a11y CI (Pa11y / axe-core CLI)**. → V1.1로 연기 (D7).
  V1은 V1.0.0 freeze gate에서 manual axe-core 브라우저 확장 audit만
  (REQ-DOC-017). 자동화된 CI-time a11y 게이트는 가치 입증 후 V1.1.

- **Automated Lighthouse CI**. → V1.1로 연기 (NFR-DOC-003). V1은
  freeze gate에서 manual Lighthouse mobile audit만. CI 자동 perf
  회귀 게이트는 V1.1.

- **Full reference-section KO translation**. → V1.1로 연기 (D3).
  V1.0.0은 Tier-1 (introduction / getting-started / end-users /
  operators core / troubleshooting) KO만 ship. `reference/*` KO
  번역은 90% Tier-1 coverage gate (REQ-DOC-016) 범위 밖 — V1.1
  minor release에서 추가.

- **Docs container image live-publish to `ghcr.io/<org>/...`**. →
  V1.0.0 gate에서 제외 (REQ-DOC-015). `build-and-push-container`
  job은 작성하되 실제 push는 `<org>` / registry 경로 확정 (Open
  Question §8.3, §8.4) 전까지 conditional/deferred. V1 gate는
  gh-pages deploy + 로컬 image build-verify까지.

- **Automated screenshot generation via Playwright**. → V1은 수동
  스크린샷 + freshness gate (REQ-DOC-014). Playwright 기반 auto-
  capture는 SPEC-EVAL-002 dashboard infrastructure 재사용 가능
  하지만 V1 scope 외 — V1.1 검토.

- **In-page feedback widget ("Was this page helpful?")**. → V1
  no-tracking commitment과 충돌. V1.1 이후 self-hosted feedback
  (GitHub Issue link만)으로 검토.

- **Auth-gated documentation sections (enterprise-only docs)**. →
  V1 docs는 100% public. enterprise-only content는 V2 검토 시점에
  separate site (`enterprise.usearch.example.com`) 가능성.

---

## 5. Acceptance Criteria

per-REQ acceptance summary는 §2에 inline 문서화. 전체 Given-When-Then
scenarios는 `.moai/specs/SPEC-DOC-001/acceptance.md` (plan-auditor
cycle에서 작성). scenario index:

| Scenario | Description | Coverage |
|----------|-------------|----------|
| §5.1 | Nextra v4 app bootstrap: clone repo, `pnpm --dir docs install && pnpm --dir docs build` produces static export in `docs/out/`; serving with any static server displays introduction page. | REQ-DOC-001, REQ-DOC-002 |
| §5.2 | Locale switching: navigate `/en/getting-started/`, click locale switcher → KO page; Pagefind search query in KO returns KO results only. | REQ-DOC-002, REQ-DOC-011 |
| §5.3 | IA completeness: assert all 7 top-level sections exist with landing pages; sidebar renders hierarchy; each operator + end-user page has required cross-link footer. | REQ-DOC-003, REQ-DOC-005, REQ-DOC-006 |
| §5.4 | Getting Started flow: new-user manual trace — follow prerequisites → compose-up → build → first query; arrive at expected output without consulting external resources. | REQ-DOC-004 |
| §5.5 | CLI reference auto-generation: implement new subcommand `usearch test-cmd`; run `scripts/gen-cli-reference.sh`; assert `content/en/reference/cli/test-cmd.mdx` is created with current help text; CI drift job fails when committed reference becomes stale. | REQ-DOC-007 |
| §5.6 | Adapter reference slot reservation: `reference/adapters/index.mdx` exists with placeholder + SPEC-DOC-002 link; `end-users/surface-comparison.mdx` + `operators/deployment-helm.mdx` cross-link the slot. | REQ-DOC-008 |
| §5.7 | Troubleshooting completeness: navigate `troubleshooting/`, verify ≥ 10 entries each in 5-field format with valid SPEC ID links. | REQ-DOC-009 |
| §5.8 | KO Tier-1 coverage: enumerate Tier-1 EN pages, assert each has KO counterpart; reviewer log in `content/ko/CONTRIBUTING.md` non-empty. | REQ-DOC-010 |
| §5.9 | Search functionality: run search "한국어 토크나이저" on `/ko/` → returns mecab-ko setup page; run "MCP server" on `/en/` → returns MCP integration page; DevTools network confirms zero third-party requests. | REQ-DOC-011 |
| §5.10 | Link-check enforcement: open PR adding `[broken](./does-not-exist)` → CI link-check job fails; PR with allowlisted-rate-limited external link returning 429 → job warns but passes. | REQ-DOC-012, REQ-DOC-013 |
| §5.11 | Screenshot freshness: backdate one `screenshot:ui:*` image to 100 days ago → CI screenshot-freshness job produces warning + auto-creates GitHub Issue tagged `docs/stale-screenshot`. | REQ-DOC-014 |
| §5.12 | Deployment: push to `main` triggers gh-pages deploy (V1 gate); `build-and-push-container` job builds `Dockerfile.docs` and serves it locally (size ≤ 100 MB); actual `ghcr.io/<org>/usearch-docs:<sha>` push is conditional/deferred until `<org>`/registry resolved. | REQ-DOC-015, NFR-DOC-004 |
| §5.13 | Bilingual coverage gate: delete one Tier-1 KO page in a PR → coverage drops below 90% → CI bilingual-coverage job fails with report listing the missing path. | REQ-DOC-016 |
| §5.14 | Accessibility manual audit at V1.0.0 freeze: axe-core extension run on introduction + 3 random reference pages produces audit report in `legal/accessibility.mdx` with zero AA-level violations. | REQ-DOC-017 |
| §5.15 | Marketing-claim lint: insert "the fastest search" into draft `introduction/what-is-usearch.mdx` → `scripts/check-doc-claims.sh` warns; remove → clean. | REQ-DOC-018 |
| §5.16 | CI runtime budget: median PR docs.yml workflow run wall-clock ≤ 5 min observed across 10 recent runs. | NFR-DOC-001 |
| §5.17 | Bundle size: `pnpm --dir docs build && du -sh docs/out/` ≤ 50 MB excluding `_pagefind/`; Pagefind index ≤ 20 MB per locale. | NFR-DOC-002 |

---

## 6. Dependencies & Blocks

### 6.1 Upstream SPEC dependencies (depends_on)

- **SPEC-CLI-001 (implemented, M2)** — `usearch query` subcommand
  v0. Source for `reference/cli/query.mdx` auto-generation. Stable
  `--help` output contract MUST be preserved across V1.

- **SPEC-CLI-002 (drafted, M7)** — Full CLI v1 subcommand tree.
  Source for `reference/cli/{deep,sources,history,config,login}.mdx`
  + `end-users/cli-tour.mdx` narrative. V1.0.0 docs ship reflects
  CLI-002 final state.

- **SPEC-UI-001 (drafted, M7)** — Web UI v1. Source for `end-users/
  web-ui-tour.mdx` screenshots + narrative; 4 routes documented
  (home/query, source detail, history, layout chrome).

- **SPEC-MCP-001 (drafted, M7)** — MCP server. Source for
  `end-users/mcp-integration.mdx` (Claude Code / Codex / Gemini CLI
  host config) + `reference/mcp/` tool catalog.

- **SPEC-SKILL-001 (drafted, M7)** — Claude Skill marketplace
  package. Source for `end-users/skill-claude.mdx` (install +
  usage narrative).

- **SPEC-AUTH-001 (implemented, M6)** — OIDC JWT validation
  middleware. Source for `operators/auth-setup.mdx` (consolidating
  `.moai/docs/MCP_OAUTH_SETUP.md`).

- **SPEC-AUTH-002 (implemented, M6)** — Team RBAC via Casbin.
  Source for `operators/team-rbac.mdx`.

- **SPEC-AUTH-003 (implemented, M6)** — Audit log. Source for
  `operators/audit-log.mdx`.

- **SPEC-OBS-001 (implemented, M1)** — Observability baseline.
  Source for `operators/observability.mdx` (metric catalog from
  cardinality allowlist, slog conventions, OTel wiring).

- **SPEC-SEC-001 (drafted, M8 — PR #42 UNMERGED, not on main)** —
  Security hardening. `ops/security/*` canonical files do NOT yet
  exist on main. `operators/security/{runbook,owasp-checklist,
  threat-model}.mdx` are forward-reference placeholders linking
  SPEC-SEC-001 status until SEC-001 merges; cross-link to
  `ops/security/*` is added by a follow-up PR post-merge.

### 6.2 Related but soft (related)

- **SPEC-DOC-002 (M9 sibling, not yet drafted)** — adapter
  reference. 본 SPEC이 IA slot reserve; DOC-002가 콘텐츠를 채움.
  parallel 실행 가능; DOC-001 ship 후 DOC-002가 별도 PR로 콘텐츠
  추가.

- **SPEC-DEPLOY-001 (M9, not yet drafted)** — Helm chart for
  k8s deploy. 본 SPEC의 `operators/deployment-helm.mdx`가
  DEPLOY-001 Helm values + install walk-through의 user-facing
  surface. DEPLOY-001 Helm chart에 docs container를 optional
  subchart로 bundle.

- **SPEC-REL-001 (M9, not yet drafted)** — V1.0.0 tag + release
  notes. release notes가 docs canonical URL을 cite. 본 SPEC PASS는
  REL-001의 "docs site live" exit gate.

- **SPEC-EVAL-001/002/003 (M8 sibling)** — eval methodology +
  Korean benchmark protocol. 선택적으로 `reference/evaluation.mdx`
  로 surface; V1.0.0 시점 우선순위 낮음.

### 6.3 Downstream blocked SPECs (blocks)

- **SPEC-REL-001 (M9, not yet drafted)** — V1.0.0 tag + release
  notes. 본 SPEC PASS 없이는 "docs site live" exit criterion
  (`roadmap.md:157`) 미달성 → V1 태깅 차단. REL-001의 release
  notes 작성 자체가 본 SPEC `legal/changelog.mdx`를 cite.

### 6.4 External dependencies (run-phase pins)

| Dependency | Pinned version | Source | License |
|------------|---------------|--------|---------|
| Next.js | `^16.2.6` (matching `web/package.json`) | Vercel | MIT |
| React | 19.x | Meta | MIT |
| Nextra | 4.x (latest stable) | shuding/nextra | MIT |
| nextra-theme-docs | 4.x (matching Nextra) | shuding/nextra | MIT |
| Pagefind | embedded via Nextra v4 | CloudCannon/pagefind | MIT |
| TypeScript | 5.x strict | Microsoft | Apache-2.0 |
| pnpm | 9+ (matching repo) | pnpm | MIT |
| lychee-action | v2 | lycheeverse/lychee-action | Apache-2.0 |
| Caddy (docs container serve) | 2.8.x stable | caddyserver/caddy | Apache-2.0 |
| actions/upload-pages-artifact | v3 | GitHub | MIT |
| actions/deploy-pages | v4 | GitHub | MIT |
| actions/cache | v4 | GitHub | MIT |

신규 Node module direct deps: `nextra`, `nextra-theme-docs` (둘 다
`docs/package.json`만 추가; `web/package.json`은 미변경). SPEC-DEP-001
REQ-DEP-007 pin policy 준수.

---

## 7. Files to Create / Modify

### 7.1 Created (estimated; final list owned by run phase)

**Next.js + Nextra app**:

| Marker | Path | Purpose |
|--------|------|---------|
| [NEW] | `docs/package.json` | Nextra v4 + Next.js 16 + Pagefind deps |
| [NEW] | `docs/pnpm-lock.yaml` | locked dep tree |
| [NEW] | `docs/next.config.mjs` | static export + i18n (en, ko) + Nextra plugin |
| [NEW] | `docs/theme.config.tsx` | nextra-theme-docs configuration (locale switcher, logo, footer) |
| [NEW] | `docs/tsconfig.json` | TypeScript 5+ strict |
| [NEW] | `docs/.gitignore` | exclude `node_modules/`, `out/`, `.next/`, `_pagefind/` |
| [NEW] | `docs/README.md` | docs-app developer onboarding (separate from end-user docs) |

**EN content (Tier-1, V1.0.0 ship)**:

| Marker | Path | Purpose |
|--------|------|---------|
| [NEW] | `docs/content/en/index.mdx` | site landing → redirects to `introduction/` |
| [NEW] | `docs/content/en/introduction/index.mdx` | what is Universal Search (sourced from product.md §1) |
| [NEW] | `docs/content/en/introduction/personas.mdx` | 4 user personas (product.md §3) |
| [NEW] | `docs/content/en/introduction/comparison.mdx` | differentiation vs competitors (product.md §7, no marketing tone) |
| [NEW] | `docs/content/en/getting-started/index.mdx` | section landing + 30-min path |
| [NEW] | `docs/content/en/getting-started/prerequisites.mdx` | tools + versions (README) |
| [NEW] | `docs/content/en/getting-started/compose-setup.mdx` | `make compose-up` walkthrough |
| [NEW] | `docs/content/en/getting-started/build-binary.mdx` | `make build` + sanity check |
| [NEW] | `docs/content/en/getting-started/first-query.mdx` | `usearch query "hello"` |
| [NEW] | `docs/content/en/end-users/index.mdx` | section landing |
| [NEW] | `docs/content/en/end-users/surface-comparison.mdx` | CLI vs UI vs Skill vs MCP decision matrix |
| [NEW] | `docs/content/en/end-users/cli-tour.mdx` | CLI narrative (CLI-001 + CLI-002) |
| [NEW] | `docs/content/en/end-users/web-ui-tour.mdx` | Web UI narrative (UI-001) + screenshots |
| [NEW] | `docs/content/en/end-users/skill-claude.mdx` | Claude Skill install + usage (SKILL-001) |
| [NEW] | `docs/content/en/end-users/mcp-integration.mdx` | MCP host config (MCP-001) |
| [NEW] | `docs/content/en/operators/index.mdx` | section landing |
| [NEW] | `docs/content/en/operators/deployment-helm.mdx` | Helm install (cross-links DEPLOY-001) |
| [NEW] | `docs/content/en/operators/auth-setup.mdx` | OIDC setup (consolidating `.moai/docs/MCP_OAUTH_SETUP.md`) |
| [NEW] | `docs/content/en/operators/team-rbac.mdx` | Casbin RBAC ops (AUTH-002) |
| [NEW] | `docs/content/en/operators/audit-log.mdx` | audit log ops (AUTH-003) |
| [NEW] | `docs/content/en/operators/observability.mdx` | Prom + slog + OTel (OBS-001) |
| [NEW] | `docs/content/en/operators/security/index.mdx` | security ops landing |
| [NEW] | `docs/content/en/operators/security/runbook.mdx` | forward-ref placeholder → SPEC-SEC-001 (no `ops/security/*` link until SEC-001 merges) |
| [NEW] | `docs/content/en/operators/security/owasp-checklist.mdx` | forward-ref placeholder → SPEC-SEC-001 (deferred cross-link) |
| [NEW] | `docs/content/en/operators/security/threat-model.mdx` | forward-ref placeholder → SPEC-SEC-001 (deferred cross-link) |
| [NEW] | `docs/content/en/reference/index.mdx` | section landing |
| [NEW] | `docs/content/en/reference/architecture.mdx` | tech.md surface |
| [NEW] | `docs/content/en/reference/dependencies.mdx` | links to auto-generated docs/dependencies.md |
| [NEW] | `docs/content/en/reference/cli/index.mdx` | CLI subcommand landing (auto-gen rest) |
| [NEW] | `docs/content/en/reference/mcp/index.mdx` | MCP tool catalog (MCP-001) |
| [NEW] | `docs/content/en/reference/adapters/index.mdx` | placeholder + link to SPEC-DOC-002 |
| [NEW] | `docs/content/en/troubleshooting/index.mdx` | top-10 entries (REQ-DOC-009 format) |
| [NEW] | `docs/content/en/legal/index.mdx` | legal section landing |
| [NEW] | `docs/content/en/legal/licenses.mdx` | sourced from `docs/dependencies.md` + product.md §8 |
| [NEW] | `docs/content/en/legal/changelog.mdx` | links repo CHANGELOG.md |
| [NEW] | `docs/content/en/legal/security.mdx` | responsible disclosure (SEC-001 V14 SECURITY.md) |
| [NEW] | `docs/content/en/legal/accessibility.mdx` | V1.0.0 audit report |
| [NEW] | `docs/content/en/legal/performance.mdx` | V1.0.0 Lighthouse report |
| [NEW] | `docs/content/en/CONTRIBUTING.md` | docs contribution workflow (EN) |

**KO content (Tier-1 mirror, V1.0.0 ship, ≥ 90% parity)**:

| Marker | Path | Purpose |
|--------|------|---------|
| [NEW] | `docs/content/ko/index.mdx` | KO landing |
| [NEW] | `docs/content/ko/introduction/{index,personas,comparison}.mdx` | KO 번역 |
| [NEW] | `docs/content/ko/getting-started/{index,prerequisites,compose-setup,build-binary,first-query}.mdx` | KO 번역 |
| [NEW] | `docs/content/ko/end-users/{index,surface-comparison,cli-tour,web-ui-tour,skill-claude,mcp-integration}.mdx` | KO 번역 |
| [NEW] | `docs/content/ko/operators/{index,deployment-helm,auth-setup,team-rbac,security/runbook}.mdx` | Tier-1 KO 번역 |
| [NEW] | `docs/content/ko/operators/korean-locale-setup.mdx` | **KO-authoritative**: mecab-ko + Naver suite + Korean RSS setup |
| [NEW] | `docs/content/ko/troubleshooting/index.mdx` | top-10 entries KO 번역 |
| [NEW] | `docs/content/ko/legal/{index,licenses,changelog,security}.mdx` | KO 번역 |
| [NEW] | `docs/content/ko/CONTRIBUTING.md` | KO contribution + 번역 워크플로우 + reviewer log |

**Scripts**:

| Marker | Path | Purpose |
|--------|------|---------|
| [NEW] | `scripts/gen-cli-reference.sh` | `usearch --help` → MDX per REQ-DOC-007 |
| [NEW] | `scripts/check-screenshot-freshness.sh` | mtime + frontmatter check per REQ-DOC-014 |
| [NEW] | `scripts/check-bilingual-coverage.sh` | EN ↔ KO parity per REQ-DOC-016 |
| [NEW] | `scripts/check-doc-claims.sh` | marketing-claim lint per REQ-DOC-018 |

**CI + config + container**:

| Marker | Path | Purpose |
|--------|------|---------|
| [NEW] | `.github/workflows/docs.yml` | 5-job docs CI per REQ-DOC-012 |
| [NEW] | `docs/lychee.toml` | link-check rules per REQ-DOC-013 |
| [NEW] | `Dockerfile.docs` | multi-stage Node 22 → Caddy serve per REQ-DOC-015 |

**Static assets**:

| Marker | Path | Purpose |
|--------|------|---------|
| [NEW] | `docs/content/{en,ko}/_screenshots/` | UI / terminal / diagram images (tagged via sibling metadata or MDX frontmatter) |
| [NEW] | `docs/content/{en,ko}/_assets/logo.svg` | brand mark placeholder until `brand/visual-identity.md` ships |

### 7.2 Modified

| Path | Change |
|------|--------|
| `docs/` (stale v3 artifacts) | REMOVE gitignored Nextra v3 / Next 14 leftovers (`docs/.next/`, `docs/out/`, `docs/node_modules/`, `docs/pages/`, scratch `docs/content/`) before v4 bootstrap. PRESERVE git-tracked `docs/dependencies.md`, `docs/_deps-*.md`, `docs/licenses/` (REQ-DOC-001) |
| `README.md` | Add prominent docs site link at top ("Full documentation: https://<org>.github.io/universal-search/"); retain existing quickstart |
| `docs/dependencies.md` | NO content change; Nextra MDX wrapper at `docs/content/en/reference/dependencies.mdx` links here as canonical |
| `docs/_deps-header.md`, `docs/_deps-compose-table.md` | NO content change; same wrapping pattern |
| `.moai/docs/MCP_OAUTH_SETUP.md` | NO content change at V1.0.0 ship; cross-linked from `operators/auth-setup.mdx`; content also copied (Phase 2 may consolidate to single source) |
| `CHANGELOG.md` | Add `## [SPEC-DOC-001]` entry referencing this SPEC at run-phase completion |
| `.gitignore` (repo root) | Add `docs/node_modules/`, `docs/out/`, `docs/.next/`, `docs/_pagefind/` patterns |
| `.pre-commit-config.yaml` | Optional: add MDX format/lint hook (prettier for `docs/content/**`) |

### 7.3 Existing — Unchanged

- `web/` Next.js query UI — independent app, unchanged. `web/
  package.json` is NOT touched.
- `cmd/usearch/` Go binary — `--help` output is the source for CLI
  reference auto-generation; no behavior change.
- `internal/**` Go packages — domain code unchanged.
- `services/**` Python sidecars — unchanged.
- `ops/security/**` SEC-001 deliverables — NOT on main yet (SEC-001
  PR #42 unmerged). This SPEC does NOT create or modify them; docs
  security pages forward-reference SPEC-SEC-001 until it merges, at
  which point a follow-up PR adds the cross-links.
- `.moai/specs/SPEC-*/` — internal SPEC docs unchanged; docs site
  does NOT surface SPEC bodies (only SPEC IDs as anchors).

---

## 8. Open Questions

본 SPEC의 `_TBD_` markers + research.md §11는 canonical list. 요약:

1. **Brand identity timing** — `.moai/project/brand/visual-
   identity.md`는 SPEC-UI-001 시점에 `_TBD_`. docs site logo +
   color tokens는 V1.0.0 ship 시점까지 placeholder. brand ship
   시점 결정 + docs theme.config.tsx 업데이트 시점 결정 필요
   (plan-auditor + manager-docs 협의).

2. **Reviewer pool for KO Tier-1 translation** — REQ-DOC-010는
   "native-Korean-speaking reviewer"를 요구하지만 reviewer
   pool 구성 + 시간 commitment + compensation 모델 미결정. V1.0.0
   ship 시점에 reviewer 최소 1명 confirmed 필요 (run phase 시작
   전 user 결정 사항).

3. **gh-pages org/repo path** — `https://<org>.github.io/universal-
   search/` canonical URL. `<org>`는 GitHub 조직 또는 user account
   확정 필요 (SPEC-REL-001과 동시 결정; 현재 README §1은 "rename
   will happen at repository creation time" — SPEC-BOOT-001 Open
   Question §3 미해결).

4. **Container registry** — `ghcr.io/<org>/usearch-docs:<tag>`.
   GitHub Container Registry 사용 가정; alternative (Docker Hub,
   Quay) 검토 필요 여부.

5. **MCP tool catalog source** — SPEC-MCP-001이 OpenAPI/JSON
   schema를 ship하면 `reference/mcp/`를 그것에서 자동 생성 가능.
   미 ship이면 hand-curated. MCP-001 run-phase 산출물 확정 후
   결정.

6. **Auto-generated screenshot policy** — REQ-DOC-014는 수동
   스크린샷 + freshness gate. Playwright 기반 auto-capture (SPEC-
   EVAL-002 dashboard infra 재사용)는 V1.1 검토. V1.0.0은 수동
   screenshot count 추정 + 작업 부담 측정 필요.

7. **Search index size on full-content bilingual site** — NFR-DOC-
   002는 Pagefind index ≤ 20 MB per locale. KO + EN 둘 다 full
   Tier-1 콘텐츠 시 실측 필요. 초과 시 (a) section 단위 multi-
   index split, (b) low-priority page exclusion 옵션 평가.

8. **Lychee external-link allowlist scope** — REQ-DOC-013은 known-
   rate-limited domains allowlist. 최초 baseline allowlist
   (github.com API, anthropic.com, x.com, naver.com Naver developer
   docs) draft 후 quarterly review로 확장. baseline 작성 시점에
   user confirmation 필요.

이 항목들은 plan-auditor PASS를 차단하지 않는다 — known unresolved
scope edges로 rationale과 함께 tagged.

---

## 9. References

External (research.md §13 cited):

- Nextra v4 release: https://nextra.site/
- Nextra docs: https://nextra.site/docs
- Nextra i18n guide: https://nextra.site/docs/guide/i18n
- Pagefind: https://pagefind.app/
- Next.js 16 docs: https://nextjs.org/docs
- WCAG 2.1 AA: https://www.w3.org/TR/WCAG21/
- axe-core: https://github.com/dequelabs/axe-core
- lychee-action: https://github.com/lycheeverse/lychee-action
- lychee CLI: https://github.com/lycheeverse/lychee
- GitHub Pages deploy-pages action: https://github.com/actions/deploy-pages
- MDX: https://mdxjs.com/
- Caddy static serve: https://caddyserver.com/docs/quick-starts/static-files
- Docusaurus (rejected alternative): https://docusaurus.io/
- VitePress (rejected alternative): https://vitepress.dev/
- MkDocs (rejected alternative): https://www.mkdocs.org/
- Keep a Changelog v1.1.0: https://keepachangelog.com/en/1.1.0/
- Semantic Versioning 2.0.0: https://semver.org/spec/v2.0.0.html

Internal (project files):

- `.moai/project/product.md` §1 (identity), §3 (personas), §4
  (V1 scope), §6 (success metrics), §7 (differentiation),
  §8 (upstream licenses)
- `.moai/project/tech.md` (architectural principles, tech stack
  per layer — `reference/architecture.mdx` source)
- `.moai/project/structure.md` (repo layout)
- `.moai/project/roadmap.md` §M9 SPEC-DOC-001 row + §5 M9 exit
  criterion "docs site live"
- `.claude/rules/moai/core/moai-constitution.md` (TRUST 5
  framework — docs MUST be Tested/Readable/Unified/Trackable;
  Secured covered by SEC-001 already)
- `.claude/rules/moai/design/constitution.md` §11 (Sprint
  Contract recommended for standard harness)
- `README.md` (existing quickstart — `getting-started/` source)
- `CHANGELOG.md` (KaC v1.1.0 format — `legal/changelog.mdx` source)
- `docs/dependencies.md` (auto-generated dep manifest —
  `reference/dependencies.mdx` source)
- `.moai/docs/MCP_OAUTH_SETUP.md` (M6 operator guide —
  `operators/auth-setup.mdx` migration source)
- `ops/security/runbook.md`, `ops/security/owasp-asvs-
  checklist.md`, `ops/security/threat-model.md` (SPEC-SEC-001
  deliverables — `operators/security/*` cross-link target)
- `.moai/specs/SPEC-CLI-001/spec.md` (implemented; `usearch
  query` source)
- `.moai/specs/SPEC-CLI-002/spec.md` (drafted; full CLI v1)
- `.moai/specs/SPEC-UI-001/spec.md` (drafted; Web UI v1 — tour
  + screenshots source)
- `.moai/specs/SPEC-MCP-001/spec.md` (drafted; MCP server)
- `.moai/specs/SPEC-SKILL-001/spec.md` (drafted; Claude Skill)
- `.moai/specs/SPEC-AUTH-001/spec.md` (implemented; OIDC)
- `.moai/specs/SPEC-AUTH-002/spec.md` (implemented; RBAC)
- `.moai/specs/SPEC-AUTH-003/spec.md` (implemented; audit log)
- `.moai/specs/SPEC-OBS-001/spec.md` (implemented; observability)
- `.moai/specs/SPEC-SEC-001/spec.md` (drafted; security hardening)
- `.moai/specs/SPEC-DEP-001/spec.md` (implemented; dependency
  baseline — docs container Trivy gate inherits SEC-001 D1 policy)

---

*End of SPEC-DOC-001 v0.2.0 (draft).*
