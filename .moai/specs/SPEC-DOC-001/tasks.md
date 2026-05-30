# SPEC-DOC-001 Tasks — atomic decomposition

Generated: 2026-05-30 (manager-strategy, Phase 1 analysis)
Methodology: DDD (ANALYZE-PRESERVE-IMPROVE). Harness: standard.
Source: spec.md (18 EARS + 7 NFR), plan.md (7 phases).

This file decomposes the plan's 7 phases into <=10 atomic tasks, each
completable in one DDD cycle. Scripts (T7) follow TDD sub-cycle
(quality.yaml development_mode=tdd; coverage_target=85 applies to the 4
shell scripts only — MDX content is measured by completeness/coverage
gates, not test coverage).

IMPORTANT pre-run amendment (see BLOCKER B1): existing `docs/` already
contains stale Nextra **v3.1.1 / Next 14** build artifacts
(docs/.next/, docs/out/, docs/node_modules/ — all gitignored). SPEC
demands Nextra **v4 / Next 16**. Bootstrap MUST clean these and pin v4.
SPEC §0 inventory wording ("introduces docs/ Nextra v4 application")
implies greenfield, which is stale — flag, do not blindly trust.

---

## Tasks

### T1 — ANALYZE: source inventory + migration map (plan Phase 1)
- Requirements: REQ-DOC-003 (IA), D2 sourcing strategy
- Deps: none
- Acceptance:
  - All source assets verified to exist with expected content:
    README.md, CHANGELOG.md, .moai/project/{product,tech,structure,roadmap}.md,
    docs/dependencies.md (+ _deps-*.md), .moai/docs/MCP_OAUTH_SETUP.md.
  - **ops/security/* NOT present on main** (lives on SPEC-SEC-001 branch
    — see B2). Migration map marks security pages as FORWARD-REF.
  - `docs/MIGRATION_MAP.md` written (Page→Source→Strategy→Owner).
  - `_meta` navigation skeletons drafted for en + ko.

### T2 — IMPROVE infra: clean stale scaffold + Nextra v4 bootstrap (plan Phase 2)
- Requirements: REQ-DOC-001, REQ-DOC-002, NFR-DOC-002
- Deps: T1
- Acceptance:
  - Remove stale `docs/.next/`, `docs/out/`, `docs/node_modules/`
    (Nextra v3/Next 14 artifacts). Confirm gitignore covers them.
  - `docs/package.json` pins nextra@4.x, nextra-theme-docs@4.x,
    next@16.x (match web/ at ^16.2.6), react@19.x, typescript@5 strict.
  - `docs/next.config.mjs`: `output: 'export'` + i18n locales [en,ko]
    defaultLocale en. `docs/theme.config.tsx`, `docs/tsconfig.json`.
  - `pnpm --dir docs install && pnpm --dir docs build` zero errors;
    `/en/` and `/ko/` route; Pagefind indexes both locales (REQ-DOC-011).
  - NOTE Nextra v4 i18n moved to `content/{locale}/` app-router model;
    the stale scaffold used `pages/` (v3) — verify v4 content dir layout
    via Context7/nextra docs before committing structure.

### T3 — PRESERVE migration: existing assets → MDX wrappers (plan Phase 3)
- Requirements: REQ-DOC-005 (operators), REQ-DOC-003, D2 PRESERVE
- Deps: T2
- Acceptance:
  - product.md → introduction/{what-is,personas,comparison};
    tech.md → reference/architecture; dependencies.md →
    reference/dependencies (link, no re-render); MCP_OAUTH_SETUP.md →
    operators/auth-setup; README quickstart → getting-started/*.
  - Byte-fidelity: content unchanged, only frontmatter + cross-link
    wrappers added.
  - Each operators page: `lastUpdated` frontmatter + "Related SPECs" footer.
  - Security pages (runbook/owasp/threat-model) created as
    placeholder-with-forward-ref to SPEC-SEC-001 (B2), NOT cross-linking
    nonexistent ops/security/* files.

### T4 — HAND-WRITE EN narrative: getting-started + end-users + troubleshooting (plan Phase 4)
- Requirements: REQ-DOC-004, REQ-DOC-006, REQ-DOC-009
- Deps: T3
- Acceptance:
  - Getting Started 4-page path (prerequisites→compose→build→first-query)
    with Next buttons; manual trace reaches successful `usearch query`.
  - End Users: surface-comparison, cli-tour, web-ui-tour (screenshots
    tagged screenshot:ui:*), skill-claude, mcp-integration.
  - Troubleshooting: >=10 entries in 5-field format with valid SPEC IDs.
  - operators/observability + team-rbac + audit-log narratives written.

### T5 — GENERATE: scripts/gen-cli-reference.sh + CLI reference MDX (plan Phase 5)
- Requirements: REQ-DOC-007 (TDD sub-cycle)
- Deps: T2 (app), binary buildable via `make build`
- Acceptance:
  - Script extracts `usearch --help` + per-subcommand help → MDX.
  - **CLI surface is query, config, history, deep, sources, login, repl**
    (7 top-level + nested) — NOT just `query` as SPEC REQ-DOC-007 ln438
    claims ("currently query"). Amend SPEC drift expectation (B3); generate
    reference for the full implemented set.
  - frontmatter {title, generated:true, source, lastGenerated};
    drift check fails when committed reference is stale.
  - reference/adapters/index.mdx placeholder + SPEC-DOC-002 link (REQ-DOC-008).

### T6 — KO Tier-1 translation + bilingual coverage (plan Phase 4/6)
- Requirements: REQ-DOC-010, REQ-DOC-016, NFR-DOC-006
- Deps: T3, T4 (EN content must exist first)
- Acceptance:
  - Hand-translated KO for Tier-1 set (introduction, getting-started,
    end-users, operators core, troubleshooting top-10).
  - korean-locale-setup.mdx KO-authoritative (mecab-ko + Naver).
  - native reviewer log in ko/CONTRIBUTING.md (>=1 reviewer — see B4
    Open Question: reviewer pool unconfirmed).
  - >=90% page parity (excl reference/cli + reference/api EN-only).
  - DEFERRABLE: full KO for non-Tier-1 reference → V1.1 (D3 Tier-2).

### T7 — CI gate scripts: screenshot-freshness + bilingual-coverage + doc-claims (plan Phase 6, TDD)
- Requirements: REQ-DOC-013, REQ-DOC-014, REQ-DOC-016, REQ-DOC-018
- Deps: T5, T6
- Acceptance:
  - scripts/check-screenshot-freshness.sh, check-bilingual-coverage.sh,
    check-doc-claims.sh + docs/lychee.toml. Each with TDD RED→GREEN.
  - lychee internal-strict (broken internal = fail), external-warn +
    allowlist (github/anthropic/x/naver).
  - Pattern mirrors existing scripts/gen-deps-manifest.sh + check-*.sh.

### T8 — CI workflow: .github/workflows/docs.yml (plan Phase 6)
- Requirements: REQ-DOC-012, NFR-DOC-001
- Deps: T7
- Acceptance:
  - 5 jobs: build, link-check, gen-reference-drift, screenshot-freshness,
    bilingual-coverage. Parallel after build. <=5 min budget.
  - Green baseline on a trivial content PR.

### T9 — Deploy: Dockerfile.docs + gh-pages + ghcr container (plan Phase 7)
- Requirements: REQ-DOC-015, NFR-DOC-004
- Deps: T8
- Acceptance:
  - Dockerfile.docs multi-stage Node 22 → Caddy, image <=100MB,
    trivy HIGH/CRITICAL clean.
  - gh-pages deploy on main; ghcr.io/<org>/usearch-docs:<sha>+latest.
  - **BLOCKED on <org> resolution** (B4 Open Q3: gh-pages org/repo path
    + container registry unconfirmed; SPEC-BOOT-001 Open Q3). Deploy job
    can be authored but live publish needs <org>.

### T10 — V1.0.0 freeze-gate audits + README link + CHANGELOG (plan Phase 7)
- Requirements: REQ-DOC-017 (a11y), NFR-DOC-003 (Lighthouse), REQ-DOC-018
- Deps: T9
- Acceptance:
  - Manual axe-core a11y audit → legal/accessibility.mdx.
  - Manual Lighthouse >=90 → legal/performance.mdx.
  - README.md docs-site link added; CHANGELOG.md [SPEC-DOC-001] entry.

---

## Deferral candidates (move out of V1.0.0 if scope pressure)
- Full KO translation of reference section (D3 Tier-2 → V1.1).
- Automated a11y CI (Pa11y/axe-core CLI) — D7 defers to V1.1 (manual at freeze).
- Automated Lighthouse CI — NFR-DOC-003 defers to V1.1.
- Playwright auto-screenshot capture (V1 manual + freshness gate).
- check-doc-claims.sh is P2/advisory — lowest priority, can slip.
- Docker container live publish if <org> unresolved (author job, defer publish).

## Blockers (resolve before/early in run)
- B1: stale Nextra v3/Next14 artifacts in docs/ — clean in T2.
- B2: ops/security/* absent on main (SPEC-SEC-001 PR#42 unmerged) —
  security operator pages are FORWARD-REF, not cross-links.
- B3: SPEC REQ-DOC-007 understates CLI surface ("currently query");
  real surface is 7 subcommands — amend, generate full set.
- B4: Open Questions (reviewer pool, <org>/registry path) — soft, do
  not block plan-auditor but gate T6 reviewer-log + T9 live deploy.
