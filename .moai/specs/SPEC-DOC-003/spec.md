---
id: SPEC-DOC-003
version: 0.1.0
status: draft
created: 2026-06-23
updated: 2026-06-23
author: limbowl
priority: P1
issue_number: 0
title: Docs Next.js prerender build fix — remove Pages-Router migration debris (orphaned pages/, i18n config key, dead theme.config) blocking the /en static export
milestone: post-V1 CI/docs debt
owner: expert-frontend
methodology: ddd
coverage_target: 0
depends_on: []
blocks: []
related: []
---

# SPEC-DOC-003: Docs Next.js prerender build fix — remove Pages-Router migration debris

## HISTORY

- 2026-06-23 (plan-auditor fixes, limbowl via manager-spec):
  Three targeted corrections, no new requirements. (1) Authored the
  companion `acceptance.md` so the §5 Given-When-Then pointer resolves.
  (2) Gated REQ-DOCBLD-060 behind an explicit precondition ("ONLY WHEN the
  EC-001 secondary-cause drill-down is triggered") so the runtime
  safe-fallback no longer silently mandates a code change under the primary
  smallest-surface path (NFR-DOCBLD-001). (3) Qualified the HISTORY lockfile
  reference as `docs/pnpm-lock.yaml` (the operative lockfile; the root
  `pnpm-lock.yaml` pins next 16.2.7 / react 19.2.7 and does not govern the
  docs build). Renumbered all requirement IDs from the `REQ-DOC` prefix to
  `REQ-DOCBLD` (docs-build) because `REQ-DOC` collides with SPEC-DOC-001
  (which owns REQ-DOC-001..018, including a direct REQ-DOC-010 clash).
  IDs updated across the §2 tables, the §5.x finding map, the §1/§6/§7/§8
  references, and every `acceptance.md` "maps to REQ-..." line.

- 2026-06-23 (initial draft v0.1.0, limbowl via manager-spec):
  Remediation of EXISTING docs-build debt (NOT a new feature). The docs
  site was migrated from Nextra v3 (Pages Router) to Nextra v4 (App
  Router: `app/[lang]/[[...mdxPath]]/page.tsx` + `content/`), but three
  classes of legacy Pages-Router artifacts were left behind, causing the
  `/en` static-export prerender to fail with a production-masked Server
  Components render error (digest `1872370934`). The defect is
  pre-existing — present since the Phase-1 docs infra commit (#38) — and
  is NOT a regression.

  Toolchain baseline (confirmed via `docs/pnpm-lock.yaml` — the operative
  lockfile; the root `pnpm-lock.yaml` pins next 16.2.7 / react 19.2.7 and
  is NOT the one that governs the docs build): Next.js 16.2.6 +
  Nextra 4.6.1 + nextra-theme-docs 4.6.1 (consumed with a local patch) +
  React 19.2.6, App Router content model
  (`content/` + `app/[lang]/[[...mdxPath]]/page.tsx:4`
  `generateStaticParamsFor('mdxPath','lang')`).

  Root cause (three left-behind artifacts):
  1. `docs/pages/en/{index,introduction,getting-started}.mdx` — 3 tracked
     orphaned Pages-Router MDX files. `pages/en/index.mdx` route-collides
     with the App Router catch-all on `/en` and has no `_app.tsx` / MDX
     provider wiring, so Next 16 throws a Server Components render error
     when prerendering it.
  2. `docs/next.config.mjs:15` — the `i18n:{ locales, defaultLocale }`
     key, which is Pages-Router-only and invalid alongside
     `output:'export'` (line 9) + App Router in Next 16. App Router locale
     routing is handled by the Nextra `[lang]` segment, not next.config.
  3. `docs/theme.config.tsx` — unreferenced Nextra-v3 theme config leftover
     (zero imports anywhere); uses keys incompatible with Nextra v4's
     strict `LayoutPropsSchema`. Dead file — migration debris.

  Fix surface: deletions + one config edit (smallest-surface fix).
  DELETE `docs/pages/` entirely; REMOVE the `i18n` block from
  `next.config.mjs` (keep `output:'export'`, `distDir:'out'`,
  `images.unoptimized`, `trailingSlash:true`); DELETE
  `docs/theme.config.tsx`; then run a local
  `cd docs && pnpm install --frozen-lockfile && pnpm build` to
  reproduce-then-confirm the `/en` prerender succeeds.

  Methodology: DDD (characterize existing build behavior via local repro
  → preserve content under `content/{en,ko}` → improve by removing debris).
  Harness: standard (P1 CI/docs debt, no production runtime change).
  Owner: expert-frontend.

  Flagged finding (status: false-positive — must NOT be trusted as a
  green signal): the stale local artifact `docs/out/en/index.html` (May 31
  10:42) is gitignored and predates `content/en/index.mdx` (Jun 1 09:28)
  and the App Router migration. It does NOT prove the current tree builds.
  A clean rebuild is the ONLY valid verification (see REQ-DOCBLD-040 / EC-002).

  Flagged blocker (status: needs-local-repro): full root-cause
  confirmation requires a local `cd docs && pnpm install && pnpm build`.
  It could not be run in the grounding pass (read-only, `node_modules`
  not built). The production build masks the Server Components error behind
  digest `1872370934`; the local repro is needed to read the unminified
  stack and confirm `pages/en` is the failing route rather than a secondary
  cause. The SPEC therefore makes the repro the FIRST acceptance step
  (REQ-DOCBLD-040).

---

## 1. Overview

SPEC-DOC-003 fixes the failing docs static-export build (`/en` prerender)
by removing three classes of Nextra-v3 (Pages Router) migration debris
that survived the migration to Nextra v4 (App Router). This is **debt
remediation, not a new feature**: all documentation content already lives
under `docs/content/{en,ko}/` and is served by the App Router catch-all
`app/[lang]/[[...mdxPath]]/page.tsx`. The fix is three deletions plus one
config edit.

### 1.1 What ships

| Layer | Artifact | Change |
|-------|----------|--------|
| Docs routing | `docs/pages/` (3 orphaned MDX files) | DELETE entirely |
| Docs config | `docs/next.config.mjs:15` (`i18n` block) | REMOVE the key only |
| Docs theme | `docs/theme.config.tsx` | DELETE (unreferenced dead file) |
| Verification | `docs/out/en/index.html`, `docs/out/ko/index.html` | regenerated fresh by a clean build |
| CI | `docs.yml` `build` job (`pnpm build`) | turns green |

### 1.2 Root cause (confirmed)

The migration from Nextra v3 (Pages Router) to Nextra v4 (App Router)
left three artifacts in the tree:

1. **Orphaned Pages-Router MDX** — `docs/pages/en/{index,introduction,getting-started}.mdx`
   (3 tracked files; their links use root-relative `/getting-started`
   with no `/en` prefix, confirming pre-i18n-migration Pages Router
   origin). `pages/en/index.mdx` maps to `/en` and collides with the App
   Router catch-all serving `content/en/index.mdx`. With no `_app.tsx` /
   MDX provider, prerendering throws the digest-masked Server Components
   render error on `/en`.

2. **Invalid `i18n` config key** — `docs/next.config.mjs:15`
   `i18n:{ locales:['en','ko'], defaultLocale:'en' }` is set alongside
   `output:'export'` (line 9) and the App Router. The Next.js `i18n`
   option is Pages-Router-only and unsupported with `output:export` and
   App Router; in Next 16.2.6 it is invalid config. App Router i18n is
   provided by Nextra's `[lang]` segment, not next.config.

3. **Dead theme config** — `docs/theme.config.tsx` is never imported
   (zero `theme.config` matches across `*.ts/*.tsx/*.mjs/*.js`); it uses
   keys (`darkMode`, `nextThemes`, `useNextSeoProps`, `search.debounce`)
   not part of Nextra v4's strict `LayoutPropsSchema`
   (`z.strictObject`). It does not itself break the build but is debris
   that should be deleted.

### 1.3 Flagged findings

- **false-positive** — `docs/out/en/index.html` (stale build artifact,
  May 31 10:42): gitignored, predates `content/en/index.mdx` (Jun 1
  09:28) and the App Router migration. It is NOT evidence that the
  current tree builds. Relying on it would be a false green signal. A
  clean rebuild is the only valid verification (REQ-DOCBLD-040, EC-002).

- **needs-local-repro (blocker, not a defect)** — full root-cause
  confirmation requires a local `cd docs && pnpm install && pnpm build`,
  not runnable in the read-only grounding pass. The production build
  masks the Server Components error behind digest `1872370934`; the local
  repro is required to read the unminified stack and confirm `pages/en`
  is the failing route. Hence the repro is the FIRST acceptance step
  (REQ-DOCBLD-040).

### 1.4 Decision needed (suppress-with-justification vs fix)

All three artifacts are resolved by **fix (delete/remove)**, not by
suppression. There is no justification for retaining orphaned routes,
invalid config, or a dead theme file. No `suppress-with-justification`
path applies. The only open decision is captured in §6 (whether a
secondary render cause surfaces after the primary deletions, requiring a
drill-down into `content/en/index.mdx` and the custom MDX components).

---

## 2. EARS Requirements

### 2.1 Routing debris removal

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-DOCBLD-010** | Ubiquitous (SHALL NOT) | The docs site SHALL NOT contain a Pages-Router routing directory (`docs/pages/`); all documentation content SHALL be authored under `docs/content/{en,ko}/` and served via the App Router catch-all `app/[lang]/[[...mdxPath]]/page.tsx`. The three orphaned files `docs/pages/en/{index,introduction,getting-started}.mdx` SHALL be removed. | P1 | `git ls-files docs/pages/` returns zero entries after the fix; `docs/content/en/index.mdx` and `docs/content/ko/index.mdx` remain present; `/en` and `/ko` are served solely by the App Router. |

### 2.2 Config debris removal

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-DOCBLD-020** | Conditional (IF-THEN) | IF `docs/next.config.mjs` sets `output:'export'` with the App Router, THEN the configuration SHALL NOT define the Pages-Router-only `i18n` key; locale routing SHALL be provided solely by the `[lang]` App Router segment and the `content/{en,ko}` directories. The edit SHALL remove only the `i18n:{...}` block and SHALL preserve `output:'export'`, `distDir:'out'`, `images.unoptimized`, and `trailingSlash:true`. | P1 | `next.config.mjs` contains no `i18n` key after the fix; `output:'export'`, `distDir:'out'`, `images.unoptimized:true`, and `trailingSlash:true` are unchanged; a grep for `i18n` in `next.config.mjs` returns zero matches. |
| **REQ-DOCBLD-030** | Ubiquitous (SHALL NOT) | The docs site SHALL NOT retain unreferenced Nextra v3 (Pages Router) configuration files incompatible with the Nextra v4 strict `LayoutPropsSchema`; specifically `docs/theme.config.tsx` SHALL be removed. The removal SHALL be safe because the file has zero imports anywhere in the app. | P1 | `docs/theme.config.tsx` is absent after the fix; a grep for `theme.config` across `docs/**/*.{ts,tsx,mjs,js}` returns zero matches (confirming nothing referenced it); the build is unaffected by its absence. |

### 2.3 Build verification

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-DOCBLD-040** | Event-Driven | WHEN the SPEC is executed, the FIRST acceptance step SHALL be a local `cd docs && pnpm install --frozen-lockfile && pnpm build` reproduction that captures the unminified render error (the digest `1872370934` Server Components failure on `/en`) BEFORE any fix is applied; the SAME command SHALL pass AFTER the fix. The stale `docs/out/` artifact (May 31) SHALL NOT be treated as a green signal — only a clean rebuild verifies the fix. | P1 | Pre-fix: `cd docs && pnpm install --frozen-lockfile && pnpm build` fails with a captured unminified stack pointing at the `pages/en` route; post-fix: the same command exits 0. The pre-fix `docs/out/` is deleted/ignored before the post-fix build so the result is fresh. |
| **REQ-DOCBLD-050** | Ubiquitous | The docs build (`cd docs && pnpm install --frozen-lockfile && pnpm build`) SHALL complete without any Next.js prerender or Server Components render error, AND SHALL emit `out/en/index.html` and `out/ko/index.html`. | P1 | `pnpm build` exits 0 with no prerender/Server-Components error in output; `docs/out/en/index.html` and `docs/out/ko/index.html` exist and are newer than the current `content/{en,ko}/index.mdx`; the `docs.yml` `build` job (`pnpm build`) passes on `main`. |

### 2.4 Custom MDX component resilience

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-DOCBLD-060** | Conditional (IF-THEN) | ONLY WHEN the EC-001 secondary-cause drill-down is triggered (i.e. `pnpm build` still errors after the primary deletions of `pages/`, the `i18n` key, and `theme.config.tsx`), IF a custom MDX component (`AdapterCatalog`, `StatusBadge`, `CapabilitiesTable`) is then found to be rendered during static export, THEN the system SHALL render it without runtime errors against the committed `_generated` JSON, returning a safe fallback when an adapter key is missing. This REQ SHALL NOT mandate any code change under the primary smallest-surface path (NFR-DOCBLD-001), where these components are not implicated. | P1 | Under the primary path this REQ imposes no code change. Only if EC-001 is triggered: a static-export build exercising any page that mounts these components completes without a runtime error, and a deliberately missing adapter key produces a safe fallback (no thrown exception). NOTE: these components are NOT used by `/en` and read valid committed JSON; this REQ is a guard for the secondary-cause drill-down only (§6). |

---

## 3. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| **NFR-DOCBLD-001** | Smallest-surface change | The fix SHALL be limited to three deletions (`docs/pages/`, `docs/theme.config.tsx`) and one config edit (`i18n` removal in `docs/next.config.mjs`). No content under `docs/content/{en,ko}/`, no `app/[lang]/[[...mdxPath]]/page.tsx`, and no dependency versions SHALL be changed as part of this remediation. |
| **NFR-DOCBLD-002** | Preserve the Nextra theme patch | The local patch `docs/patches/nextra-theme-docs@4.6.1.patch` (makes `LayoutProps.children` optional) SHALL be preserved. Nextra SHALL NOT be bumped as part of this fix; any version change would require re-validating the patch and is out of scope. |
| **NFR-DOCBLD-003** | Clean-build-only verification | Verification SHALL rely exclusively on a fresh `pnpm build` output. Pre-existing `docs/out/` artifacts SHALL NOT be used as evidence of a passing build (the May 31 artifact is stale and gitignored). |

---

## 4. Exclusions (What NOT to Build)

[HARD] The following are explicitly out of scope. Each has a known
destination or rationale.

- **Trusting the stale `docs/out/` artifact**. → false-positive
  (§1.3). The May 31 `out/en/index.html` predates the current content and
  the App Router migration. It is deleted/regenerated by a clean build,
  never used as a green signal.

- **Nextra / Next.js / React version bumps**. → Out of scope. The fix is
  migration-debris removal, not a dependency upgrade. The
  `nextra-theme-docs@4.6.1` local patch must be preserved; bumping would
  require re-validating the patch (NFR-DOCBLD-002).

- **Rewriting `content/en/index.mdx` or the App Router page**. → Out of
  scope unless a secondary render cause surfaces after the primary
  deletions (§6 drill-down). The primary fix touches neither.

- **Refactoring the custom MDX components** (`AdapterCatalog`,
  `StatusBadge`, `CapabilitiesTable`). → Out of scope. They are not used
  by `/en` and read valid committed `_generated` JSON. REQ-DOCBLD-060 only
  asserts a safe-fallback guard, not a rewrite.

- **Adding new docs content, pages, or locales**. → Out of scope. This is
  a build fix; content authoring is unrelated.

- **CI workflow changes to `docs.yml`**. → Out of scope. The existing
  `build` job (`pnpm build`) is the verifier; it turns green once the
  debris is removed. No workflow edit is required.

- **GitHub Issue tracking on this SPEC** (`issue_number: 0`). → CI/docs
  debt remediation pattern; no issue.

---

## 5. Acceptance Criteria

Headline acceptance (CI-debt SPEC): **the `docs.yml` `build` job
(`pnpm build`) passes on `main`** with `out/en/index.html` and
`out/ko/index.html` regenerated fresh (REQ-DOCBLD-050).

per-REQ acceptance summary is inline in §2. The full Given-When-Then
scenarios and edge cases are authored in the companion file
`.moai/specs/SPEC-DOC-003/acceptance.md` (one GWT scenario per row below).
Scenario index:

| Scenario | Description | Coverage |
|----------|-------------|----------|
| §5.1 | Reproduce-first: pre-fix local `cd docs && pnpm install --frozen-lockfile && pnpm build` fails with an unminified stack pointing at the `pages/en` route (the digest `1872370934` `/en` Server Components error). | REQ-DOCBLD-040 |
| §5.2 | Routing debris removed: `git ls-files docs/pages/` returns zero entries; `content/{en,ko}/index.mdx` remain; `/en` served only by the App Router catch-all. | REQ-DOCBLD-010 |
| §5.3 | Config debris removed: `next.config.mjs` has no `i18n` key; `output:'export'`, `distDir:'out'`, `images.unoptimized`, `trailingSlash:true` preserved. | REQ-DOCBLD-020 |
| §5.4 | Dead theme removed: `docs/theme.config.tsx` absent; grep for `theme.config` returns zero matches confirming nothing referenced it. | REQ-DOCBLD-030 |
| §5.5 | Post-fix clean build: `pnpm build` exits 0, no prerender/Server-Components error, emits fresh `out/en/index.html` + `out/ko/index.html`; `docs.yml` `build` job green on `main`. | REQ-DOCBLD-040, REQ-DOCBLD-050 |
| §5.6 | Custom MDX component guard: static export exercising `AdapterCatalog`/`StatusBadge`/`CapabilitiesTable` completes without runtime error; missing adapter key → safe fallback. | REQ-DOCBLD-060 |

### Edge cases

- **EC-001** — Secondary render cause after deletions: if `pnpm build`
  still errors after removing `pages/`, the `i18n` key, and
  `theme.config.tsx`, drill into `content/en/index.mdx` and the custom MDX
  components (`AdapterCatalog`/`StatusBadge`/`CapabilitiesTable`) — though
  those are not used by `/en` and read valid committed JSON (§6).

- **EC-002** — Stale artifact masking: ensure `docs/out/` is deleted (or a
  clean checkout used) before the post-fix build so the May 31 artifact
  cannot masquerade as a passing build (false-positive, §1.3).

- **EC-003** — Patch preservation: confirm `pnpm install --frozen-lockfile`
  applies `docs/patches/nextra-theme-docs@4.6.1.patch`; a build failure
  due to a dropped patch is distinct from the migration-debris defect.

---

## 6. Dependencies & Blocks

### 6.1 Upstream SPEC dependencies (depends_on)

None. This is a self-contained docs-build remediation; no SPEC must land
first.

### 6.2 Downstream blocked SPECs (blocks)

None recorded. The failing `docs.yml` `build` job is CI debt; no other
SPEC is gated on this fix.

### 6.3 Blockers (operational)

- **needs-local-repro** — Full root-cause confirmation requires a local
  `cd docs && pnpm install && pnpm build`. It was NOT runnable in the
  read-only grounding pass (`node_modules` not built, no build executed).
  The production build masks the Server Components error behind digest
  `1872370934`; the local repro reads the unminified stack and confirms
  `pages/en` is the failing route. This is why REQ-DOCBLD-040 makes the repro
  the first acceptance step.

- **stale-artifact trust** — `docs/out/` (May 31) must NOT be trusted as a
  green signal; a clean build is the only valid verification (NFR-DOCBLD-003,
  EC-002).

### 6.4 External dependencies (run-phase pins)

| Dependency | Version | Source | Note |
|------------|---------|--------|------|
| next | ^16.2.6 (lockfile 16.2.6) | `docs/package.json` / `docs/pnpm-lock.yaml` | App Router + `output:'export'` |
| nextra | ^4.0.0 (lockfile 4.6.1) | `docs/package.json` / `docs/pnpm-lock.yaml` | App Router content model |
| nextra-theme-docs | 4.6.1 (patched) | `docs/pnpm-lock.yaml` + `docs/patches/nextra-theme-docs@4.6.1.patch` | patch makes `LayoutProps.children` optional — MUST preserve (NFR-DOCBLD-002) |
| react | 19.2.6 | `docs/pnpm-lock.yaml` | — |

No upstream library fix is pending; the failure is local migration debris,
not an unreleased dependency bug. Do NOT bump Nextra without re-validating
the local patch.

---

## 7. Files to Create / Modify

### 7.1 Deleted

| Path | Reason |
|------|--------|
| `docs/pages/en/index.mdx` | orphaned Pages-Router route, collides with App Router `/en` (REQ-DOCBLD-010) |
| `docs/pages/en/introduction.mdx` | orphaned Pages-Router MDX (content lives in `content/{en,ko}`) (REQ-DOCBLD-010) |
| `docs/pages/en/getting-started.mdx` | orphaned Pages-Router MDX (REQ-DOCBLD-010) |
| `docs/pages/` (directory) | remove the entire Pages-Router routing directory (REQ-DOCBLD-010) |
| `docs/theme.config.tsx` | unreferenced Nextra-v3 dead config, incompatible with Nextra v4 `LayoutPropsSchema` (REQ-DOCBLD-030) |

### 7.2 Modified

| Path | Change |
|------|--------|
| `docs/next.config.mjs` | remove the `i18n:{ locales, defaultLocale }` block (line 15); preserve `output:'export'` (line 9), `distDir:'out'`, `images.unoptimized`, `trailingSlash:true` (REQ-DOCBLD-020) |

### 7.3 Regenerated (build output, gitignored)

| Path | Change |
|------|--------|
| `docs/out/en/index.html` | regenerated fresh by clean `pnpm build` (replaces stale May 31 artifact) (REQ-DOCBLD-050) |
| `docs/out/ko/index.html` | emitted by clean `pnpm build` (REQ-DOCBLD-050) |

### 7.4 Existing — Unchanged

- `docs/content/{en,ko}/` — all documentation content already lives here;
  no change (NFR-DOCBLD-001).
- `docs/app/[lang]/[[...mdxPath]]/page.tsx` — App Router catch-all; no
  change.
- `docs/patches/nextra-theme-docs@4.6.1.patch` — preserved as-is
  (NFR-DOCBLD-002).
- `docs/package.json`, `docs/pnpm-lock.yaml` — no dependency version change
  (NFR-DOCBLD-001, NFR-DOCBLD-002).
- Custom MDX components (`AdapterCatalog`, `StatusBadge`,
  `CapabilitiesTable`) — unchanged unless EC-001 secondary-cause drill-down
  requires it.

---

## 8. Open Questions

1. **Secondary render cause** — If the build still fails after the three
   primary removals, does the residual error originate in
   `content/en/index.mdx` or in a custom MDX component? Resolved by the
   REQ-DOCBLD-040 local repro reading the unminified stack (run phase).

2. **`docs.yml` cache invalidation** — Whether the CI `build` job needs a
   cache bust so the stale `out/` is not restored. Confirm at run phase
   when the green build is verified.

These do not block planning — they are known scope edges resolved by the
first acceptance step (REQ-DOCBLD-040).

---

## 9. References

Internal (project files):

- `docs/pages/en/{index,introduction,getting-started}.mdx` — orphaned
  Pages-Router MDX (deletion target)
- `docs/next.config.mjs:9` (`output:'export'`), `:15` (`i18n` block —
  removal target)
- `docs/theme.config.tsx` — dead Nextra-v3 config (deletion target)
- `docs/content/{en,ko}/index.mdx` — canonical App Router content
- `docs/app/[lang]/[[...mdxPath]]/page.tsx:4` —
  `generateStaticParamsFor('mdxPath','lang')`
- `docs/patches/nextra-theme-docs@4.6.1.patch` — local theme patch
  (preserve)
- `docs/package.json` (next ^16.2.6, nextra ^4.0.0), `docs/pnpm-lock.yaml`
  (next@16.2.6, nextra@4.6.1, nextra-theme-docs@4.6.1, react@19.2.6)
- `docs/out/en/index.html` — stale gitignored artifact (false-positive;
  do not trust)
- `docs/.gitignore` — lists `out`

---

*End of SPEC-DOC-003 v0.1.0 (draft).*
