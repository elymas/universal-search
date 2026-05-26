---
id: SPEC-DOC-001
version: 0.1.0
status: draft
created: 2026-05-26
author: limbowl (via manager-spec)
related_spec: SPEC-DOC-001 (spec.md, plan.md)
format: Given/When/Then
---

# SPEC-DOC-001 Acceptance Scenarios

## 0. Document Purpose

This document specifies acceptance criteria for SPEC-DOC-001 in Given/When/Then format, expanding the scenario index in spec.md §5 into externally-observable behaviors that the run phase MUST verify before declaring DOC-001 ship-ready.

Scope: 17 acceptance criteria (AC-001..AC-017) covering REQ-DOC-001 through REQ-DOC-018 + NFR-DOC-001 through NFR-DOC-007, plus 3 edge-case sections, plus a Definition of Done checklist.

Coverage policy: every REQ and every NFR in spec.md §2 / §3 has ≥1 matching AC below. See Coverage Matrix at end of file.

---

## 1. Acceptance Criteria (Given/When/Then)

### AC-001 — Nextra v4 app bootstrap produces static export

Covers: REQ-DOC-001

**Given** a fresh checkout of the repository with `docs/package.json`, `docs/next.config.mjs`, `docs/theme.config.tsx`, and `docs/content/en/introduction/index.mdx`.

**When** the contributor runs:
```
pnpm --dir docs install
pnpm --dir docs build
```

**Then**:
- `pnpm install` succeeds with no peer-dep errors (pnpm 9.x, Node 22 LTS).
- `pnpm --dir docs build` exits `0`.
- `docs/out/index.html` exists.
- `pnpm --dir docs dev` boots the dev server and serves the introduction page on http://localhost:3000.
- `docs/package.json` declares `build`, `dev`, `lint`, and `start` scripts.

Maps to scenario §5.1 in spec.md.

---

### AC-002 — Static export + i18n locale switcher work end-to-end

Covers: REQ-DOC-002

**Given** built static export with `docs/next.config.mjs` containing `output: 'export'` and `i18n: { locales: ['en', 'ko'], defaultLocale: 'en' }`.

**When** the contributor opens `docs/out/en/getting-started/index.html` in a static server and clicks the locale switcher.

**Then**:
- URL transitions from `/en/getting-started/` to `/ko/getting-started/`.
- KO content is rendered (matching `content/ko/getting-started/index.mdx`).
- `/` (root) redirects to `/en/` by default.
- `_pagefind/en/` and `_pagefind/ko/` directories exist as separate indexes.

Maps to scenarios §5.1, §5.2 in spec.md.

---

### AC-003 — Seven-section IA + sidebar hierarchy

Covers: REQ-DOC-003

**Given** built docs site.

**When** the contributor visits the docs root.

**Then**:
- All 7 top-level sections render in the sidebar: Introduction, Getting Started, End Users, Operators, Reference, Troubleshooting, Legal.
- Each section folder exists under both `content/en/` and `content/ko/` with an `index.mdx`.
- Clicking a section reveals its subpages with collapsible groups.
- The active page is visually highlighted in the sidebar.

Maps to scenario §5.3 in spec.md.

---

### AC-004 — Getting Started 30-minute path

Covers: REQ-DOC-004

**Given** the docs site and a new operator persona (per `.moai/project/product.md` §3) with no prior universal-search exposure.

**When** the operator follows the Getting Started landing page strictly (prerequisites → compose-up → build → first query) without consulting external resources.

**Then**:
- The four sub-pages exist in order with working "Next" buttons.
- Following the path produces a successful `usearch query "hello"` execution with the documented expected output.
- The page includes troubleshooting links for at least 3 most-common first-run failures (compose port conflict, missing env vars, LLM provider unconfigured).
- Total elapsed time is ≤ 30 minutes on a fresh laptop.

Maps to scenario §5.4 in spec.md.

---

### AC-005 — Operators + End Users content present with cross-link footers

Covers: REQ-DOC-005, REQ-DOC-006

**Given** built docs site.

**When** the contributor navigates the Operators and End Users sections.

**Then**:
- Operators section contains at minimum: `deployment-helm.mdx`, `auth-setup.mdx`, `team-rbac.mdx`, `audit-log.mdx`, `observability.mdx`, `security/runbook.mdx`, `security/owasp-checklist.mdx`, `security/threat-model.mdx`.
- End Users section contains at minimum: `cli-tour.mdx`, `web-ui-tour.mdx`, `skill-claude.mdx`, `mcp-integration.mdx`, `surface-comparison.mdx`.
- Each operator page has frontmatter `lastUpdated` date AND a "Related SPECs" footer linking the underlying SPEC IDs.
- Each end-user page contains ≥ 1 tagged screenshot reference (`screenshot:ui:*` or `screenshot:terminal:*`).

Maps to scenario §5.3 in spec.md.

---

### AC-006 — CLI reference auto-generation with drift detection

Covers: REQ-DOC-007

**Given** the `usearch` binary at `bin/usearch` (or buildable via `make build`).

**When** the contributor runs:
```
./scripts/gen-cli-reference.sh
```

**Then**:
- The script builds the binary if missing.
- For each subcommand defined in SPEC-CLI-001/002 implemented set, an MDX file is written to `docs/content/en/reference/cli/{subcommand}.mdx`.
- Each MDX file has frontmatter `{title, generated: true, source: "usearch --help", lastGenerated: <ISO-8601>}`.
- The help text is rendered inside an MDX `<CodeBlock>` component.
- CI job `gen-reference-drift` fails when committed reference content drifts from re-generated output.
- Implementing a new subcommand `usearch test-cmd` and re-running the script produces a new file `content/en/reference/cli/test-cmd.mdx` with the current help text.

Maps to scenario §5.5 in spec.md.

---

### AC-007 — Adapter reference slot reservation (forward-compat)

Covers: REQ-DOC-008

**Given** built docs site without SPEC-DOC-002 (adapter reference) shipped yet.

**When** the contributor navigates to `/en/reference/adapters/`.

**Then**:
- `reference/adapters/index.mdx` exists with a placeholder + link to SPEC-DOC-002 status.
- `end-users/surface-comparison.mdx` cross-links the adapter slot.
- `operators/deployment-helm.mdx` cross-links the adapter slot.
- Merging SPEC-DOC-002 later does NOT require modifying the IA layout of SPEC-DOC-001.

Maps to scenario §5.6 in spec.md.

---

### AC-008 — Troubleshooting top-10 in standardized 5-field format

Covers: REQ-DOC-009

**Given** built docs site.

**When** the contributor visits `/en/troubleshooting/index.mdx`.

**Then**:
- ≥ 10 entries are listed, each following: Symptom → Likely Cause → Diagnostic Command → Resolution → Related SPEC IDs.
- Each entry has ≥ 1 Related SPEC ID with a valid intra-site link.
- The 10 entries are sourced from at least: SPEC-CACHE-001, SPEC-AUTH-001, SPEC-SEC-001, SPEC-IDX-003, SPEC-LLM-001, SPEC-BOOT-001, SPEC-IDX-001, SPEC-DOC-002 placeholder.

Maps to scenario §5.7 in spec.md.

---

### AC-009 — KO Tier-1 hand-translated coverage

Covers: REQ-DOC-010, NFR-DOC-006

**Given** built docs site at V1.0.0 freeze.

**When** the contributor enumerates Tier-1 EN pages and audits KO counterparts.

**Then**:
- Every Tier-1 EN page (all Introduction, all Getting Started, all End Users, Operators core path, Troubleshooting top-10) has a KO counterpart in `content/ko/`.
- `content/ko/CONTRIBUTING.md` reviewer log lists ≥ 1 named native-Korean-speaking reviewer per Tier-1 batch.
- `content/ko/operators/korean-locale-setup.mdx` is marked as KO-authoritative (source of truth).
- The translation backlog table in `content/ko/CONTRIBUTING.md` tracks any pending KO updates from recent EN modifications (≥ 30 lines diff) within the same minor release window.

Maps to scenarios §5.8, §5.13 in spec.md.

---

### AC-010 — Pagefind search functional with zero third-party requests

Covers: REQ-DOC-011

**Given** built docs site on dev server.

**When** the contributor opens DevTools Network tab and runs the search queries: "Korean" on `/en/`, "한국어 토크나이저" on `/ko/`, "MCP server" on `/en/`.

**Then**:
- All queries return relevant results within 500ms p95 on a mid-tier laptop.
- "한국어 토크나이저" on `/ko/` returns the mecab-ko setup page.
- "MCP server" on `/en/` returns the MCP integration page.
- DevTools Network confirms ZERO third-party requests during search interaction (no algolia.com, no docsearch.com).
- `_pagefind/` indexes are served from the same origin as the docs site.

Maps to scenarios §5.2, §5.9 in spec.md.

---

### AC-011 — Docs CI workflow runs all 5 jobs within 5-minute budget

Covers: REQ-DOC-012, NFR-DOC-001

**Given** `.github/workflows/docs.yml` and a PR modifying `docs/content/en/introduction/index.mdx`.

**When** GitHub Actions runs the workflow on `ubuntu-24.04`.

**Then**:
- All 5 jobs execute: `build`, `link-check`, `gen-reference-drift`, `screenshot-freshness`, `bilingual-coverage`.
- Jobs after `build` run in parallel.
- Total wall-clock time ≤ 5 minutes for the median PR (< 50 MDX changes).
- A failure in any individual job causes the workflow status to be `failure`.

Maps to scenarios §5.10, §5.16 in spec.md.

---

### AC-012 — Lychee link-check: internal strict, external lenient

Covers: REQ-DOC-013

**Given** `docs/lychee.toml` configured for internal-strict + external-warn.

**When** the contributor opens a PR containing each of:
- A broken internal link: `[bad](./does-not-exist)`.
- An unreachable external link: `https://this-does-not-exist.invalid`.
- An allowlisted rate-limited external link: `https://api.github.com/` returning 403.

**Then**:
- `link-check` job FAILS due to the broken internal link.
- `link-check` job posts a PR annotation for the unreachable external link but does NOT fail on it.
- `link-check` job ignores the 403 from the allowlisted GitHub API link.
- External-link retries: 3 attempts with exponential backoff.
- `.lycheecache` is restored from CI cache (`actions/cache@v4`) and invalidated weekly.

Maps to scenario §5.10 in spec.md.

---

### AC-013 — Screenshot freshness gate auto-creates GitHub Issue for stale UI screenshots

Covers: REQ-DOC-014

**Given** a UI screenshot tagged `screenshot:ui:cli-tour-01.png` with mtime 100 days ago and no `lastVerified` override.

**When** `screenshot-freshness` job runs.

**Then**:
- The job emits a warning naming the file path and the affected page.
- A GitHub Issue is auto-created (or updated if existing) tagged `docs/stale-screenshot`, referencing the affected page.
- Updating `lastVerified: <today>` in the MDX frontmatter removes the warning on the next CI run.
- Diagrams (`screenshot:diagram:*`) and untagged images do NOT trigger the check.

Maps to scenario §5.11 in spec.md.

---

### AC-014 — Dual deployment: gh-pages + GHCR container

Covers: REQ-DOC-015, NFR-DOC-002, NFR-DOC-004, NFR-DOC-007

**Given** a push to `main`.

**When** the deploy job runs.

**Then**:
- gh-pages is deployed via `actions/deploy-pages@v4`; site is reachable at `https://<org>.github.io/universal-search/`.
- Container image is pushed to `ghcr.io/<org>/usearch-docs:<sha>` AND `ghcr.io/<org>/usearch-docs:latest`.
- `docker run -p 8080:80 ghcr.io/<org>/usearch-docs:latest` serves the docs index on http://localhost:8080.
- Compressed image size ≤ 100 MB.
- `trivy image --severity HIGH,CRITICAL ghcr.io/<org>/usearch-docs:latest` reports zero findings.
- `du -sh docs/out/` (excluding `_pagefind/`) ≤ 50 MB; `_pagefind/` per-locale ≤ 20 MB.
- On a tagged release (`v1.x.y`), additional tag `ghcr.io/<org>/usearch-docs:v1.x.y` is pushed.

Maps to scenarios §5.12, §5.17 in spec.md.

---

### AC-015 — Bilingual coverage gate enforces ≥ 90% KO parity

Covers: REQ-DOC-016

**Given** `scripts/check-bilingual-coverage.sh` and current Tier-1 EN pages.

**When** the contributor opens a PR deleting one Tier-1 KO page (e.g., `content/ko/getting-started/index.mdx`).

**Then**:
- Coverage drops below 90%.
- `bilingual-coverage` job FAILS.
- `docs/coverage-report.md` is generated listing the specific missing KO paths.
- The 90% threshold can NOT be lowered without explicit SPEC amendment (script literal threshold).

Maps to scenario §5.13 in spec.md.

---

### AC-016 — WCAG 2.1 AA accessibility audit at V1.0.0 freeze

Covers: REQ-DOC-017, NFR-DOC-003

**Given** the V1.0.0 freeze build of the docs site.

**When** an axe-core browser extension audit is run on the introduction page + 3 randomly-sampled deep reference pages.

**Then**:
- Audit results are recorded in `docs/content/en/legal/accessibility.mdx`.
- Zero AA-level violations on the audited pages.
- Custom theme overrides (color, admonition, code-block) preserve contrast ratios ≥ 4.5:1 for normal text and ≥ 3:1 for large text.
- A manual Lighthouse mobile audit on the introduction page + a random deep page achieves Performance score ≥ 90.
- Lighthouse results are recorded in `docs/content/en/legal/performance.mdx`.

Maps to scenario §5.14 in spec.md.

---

### AC-017 — Marketing-claim lint blocks unsubstantiated superlatives

Covers: REQ-DOC-018, NFR-DOC-005

**Given** `scripts/check-doc-claims.sh` with prohibited-phrase allowlist + the current clean docs baseline.

**When** the contributor inserts "the fastest search engine" into a draft `introduction/what-is-usearch.mdx` and runs the script.

**Then**:
- The script WARNS (advisory, not failing in V1) and lists the offending file + line.
- Removing the phrase produces a clean run with zero warnings.
- A baseline `scripts/check-doc-claims.sh` run against current docs produces zero matches.
- Lychee external-link cache (NFR-DOC-005): `.lycheecache` is persisted with max age 7 days and refreshed on the Sunday 02:00 UTC weekly cron.

Maps to scenario §5.15 in spec.md.

---

## 2. Edge Cases

### EC-001 — Locale switcher fallback when KO page is missing

**Given** an EN page exists at `/en/operators/some-page` but the corresponding KO translation does NOT exist (mid-development).

**When** a user on `/en/operators/some-page` clicks the locale switcher.

**Then**:
- The switcher does NOT result in a 404.
- The user is either redirected to `/ko/` landing with a banner indicating "translation pending", OR the active page is replaced by the locale's nearest available page.
- The behavior is documented in `content/ko/CONTRIBUTING.md` translation backlog table.

### EC-002 — CLI reference drift CI failure on stale committed reference

**Given** the contributor modifies `usearch query --help` output (e.g., adds a new flag) but does NOT re-run `scripts/gen-cli-reference.sh` before committing.

**When** the CI `gen-reference-drift` job runs.

**Then**:
- The job FAILS with a diff showing the drift.
- The PR is blocked.
- The author is instructed to run `scripts/gen-cli-reference.sh` and commit the regenerated reference.

### EC-003 — Pagefind search index inflates beyond 20 MB per locale

**Given** the docs site has grown to ~3,000 MDX pages per locale and the Pagefind index exceeds 20 MB per locale.

**When** `pnpm --dir docs build` runs.

**Then**:
- CI produces a WARNING with the size delta breakdown (per-locale index size + total).
- The build still SUCCEEDS (NFR-DOC-002 is a soft warning, not a hard fail).
- A follow-up Issue is created to investigate Pagefind tuning options (excluding low-value pages, reducing index granularity).

---

## 3. Definition of Done Checklist

- [ ] All 17 AC scenarios pass on a fresh CI run.
- [ ] All 17 scenario index entries (§5.1..§5.17) in spec.md are implemented as automated tests or manual audit reports.
- [ ] `docs/package.json` declares `build`, `dev`, `lint`, `start` scripts.
- [ ] `docs/next.config.mjs` contains `output: 'export'` + i18n `locales: ['en', 'ko']`.
- [ ] All 7 top-level IA sections exist with landing pages in both locales.
- [ ] `scripts/gen-cli-reference.sh`, `scripts/check-screenshot-freshness.sh`, `scripts/check-bilingual-coverage.sh`, `scripts/check-doc-claims.sh` are committed and executable.
- [ ] `.github/workflows/docs.yml` contains all 5 jobs and completes within 5 minutes for the median PR.
- [ ] `docs/lychee.toml` with internal-strict + external-warn config.
- [ ] gh-pages deploys at `https://<org>.github.io/universal-search/`.
- [ ] `ghcr.io/<org>/usearch-docs:latest` reachable; container size ≤ 100 MB; zero Trivy HIGH/CRITICAL findings.
- [ ] KO Tier-1 coverage ≥ 90% (bilingual-coverage gate green).
- [ ] V1.0.0 freeze: axe-core audit + Lighthouse mobile audit recorded in `legal/`.
- [ ] Open Questions in spec.md §8 are resolved or explicitly deferred with mitigation.

---

## 4. Coverage Matrix (REQ → AC)

| REQ / NFR | AC-001 | AC-002 | AC-003 | AC-004 | AC-005 | AC-006 | AC-007 | AC-008 | AC-009 | AC-010 | AC-011 | AC-012 | AC-013 | AC-014 | AC-015 | AC-016 | AC-017 |
|-----------|--------|--------|--------|--------|--------|--------|--------|--------|--------|--------|--------|--------|--------|--------|--------|--------|--------|
| REQ-DOC-001 | ✓ |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   |
| REQ-DOC-002 |   | ✓ |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   |
| REQ-DOC-003 |   |   | ✓ |   |   |   |   |   |   |   |   |   |   |   |   |   |   |
| REQ-DOC-004 |   |   |   | ✓ |   |   |   |   |   |   |   |   |   |   |   |   |   |
| REQ-DOC-005 |   |   |   |   | ✓ |   |   |   |   |   |   |   |   |   |   |   |   |
| REQ-DOC-006 |   |   |   |   | ✓ |   |   |   |   |   |   |   |   |   |   |   |   |
| REQ-DOC-007 |   |   |   |   |   | ✓ |   |   |   |   |   |   |   |   |   |   |   |
| REQ-DOC-008 |   |   |   |   |   |   | ✓ |   |   |   |   |   |   |   |   |   |   |
| REQ-DOC-009 |   |   |   |   |   |   |   | ✓ |   |   |   |   |   |   |   |   |   |
| REQ-DOC-010 |   |   |   |   |   |   |   |   | ✓ |   |   |   |   |   |   |   |   |
| REQ-DOC-011 |   |   |   |   |   |   |   |   |   | ✓ |   |   |   |   |   |   |   |
| REQ-DOC-012 |   |   |   |   |   |   |   |   |   |   | ✓ |   |   |   |   |   |   |
| REQ-DOC-013 |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |   |   |   |   |
| REQ-DOC-014 |   |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |   |   |   |
| REQ-DOC-015 |   |   |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |   |   |
| REQ-DOC-016 |   |   |   |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |   |
| REQ-DOC-017 |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |
| REQ-DOC-018 |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   | ✓ |
| NFR-DOC-001 |   |   |   |   |   |   |   |   |   |   | ✓ |   |   |   |   |   |   |
| NFR-DOC-002 |   |   |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |   |   |
| NFR-DOC-003 |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |
| NFR-DOC-004 |   |   |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |   |   |
| NFR-DOC-005 |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   | ✓ |
| NFR-DOC-006 |   |   |   |   |   |   |   |   | ✓ |   |   |   |   |   |   |   |   |
| NFR-DOC-007 |   |   |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |   |   |

Every REQ and NFR has ≥ 1 AC; edge cases supplement EC-001..EC-003.

---

*End of SPEC-DOC-001 acceptance.md.*
