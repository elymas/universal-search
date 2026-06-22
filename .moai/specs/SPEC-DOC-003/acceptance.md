# SPEC-DOC-003 — Acceptance Criteria

Companion to `spec.md` §5. Given-When-Then scenarios for the docs Next.js
prerender build fix (remove Pages-Router migration debris). Each scenario
maps to one or more EARS requirements in §2. The headline acceptance is the
`docs.yml` `build` job (`pnpm build`) passing on `main` with fresh
`out/en/index.html` and `out/ko/index.html` (REQ-DOCBLD-050).

All verification is clean-build-only: pre-existing `docs/out/` artifacts are
never accepted as evidence of a passing build (NFR-DOCBLD-003).

---

## §5.1 — Reproduce-first (capture the unminified failure)

Maps to REQ-DOCBLD-040.

- **Given** a clean checkout of `main` with the migration debris still
  present (`docs/pages/en/{index,introduction,getting-started}.mdx`, the
  `i18n` block at `docs/next.config.mjs:15`, and `docs/theme.config.tsx`),
  and `docs/node_modules` not yet built,
- **When** `cd docs && pnpm install --frozen-lockfile && pnpm build` is run
  BEFORE any fix is applied,
- **Then** the build fails with an unminified Server Components render error
  whose stack points at the `pages/en` route (the production digest
  `1872370934` `/en` failure), and that stack is captured as the
  reproduction evidence before any deletion or config edit.

## §5.2 — Routing debris removed

Maps to REQ-DOCBLD-010.

- **Given** the three orphaned Pages-Router files under `docs/pages/en/`,
- **When** `docs/pages/` is deleted entirely,
- **Then** `git ls-files docs/pages/` returns zero entries,
  `docs/content/en/index.mdx` and `docs/content/ko/index.mdx` remain present,
  and `/en` and `/ko` are served solely by the App Router catch-all
  `app/[lang]/[[...mdxPath]]/page.tsx`.

## §5.3 — Config debris removed

Maps to REQ-DOCBLD-020.

- **Given** `docs/next.config.mjs` defines the Pages-Router-only `i18n`
  block alongside `output:'export'` and the App Router,
- **When** only the `i18n:{ locales, defaultLocale }` block is removed,
- **Then** a grep for `i18n` in `docs/next.config.mjs` returns zero matches,
  and `output:'export'`, `distDir:'out'`, `images.unoptimized:true`, and
  `trailingSlash:true` remain unchanged.

## §5.4 — Dead theme config removed

Maps to REQ-DOCBLD-030.

- **Given** `docs/theme.config.tsx`, an unreferenced Nextra-v3 config
  incompatible with Nextra v4's strict `LayoutPropsSchema`,
- **When** the file is deleted,
- **Then** `docs/theme.config.tsx` is absent, a grep for `theme.config`
  across `docs/**/*.{ts,tsx,mjs,js}` returns zero matches (confirming
  nothing referenced it), and the build is unaffected by its absence.

## §5.5 — Post-fix clean build

Maps to REQ-DOCBLD-040 and REQ-DOCBLD-050.

- **Given** the three deletions and the one config edit are applied, and the
  stale `docs/out/` is deleted (or a clean checkout is used),
- **When** `cd docs && pnpm install --frozen-lockfile && pnpm build` is run
  AFTER the fix,
- **Then** the command exits 0 with no prerender or Server Components render
  error in the output, emits fresh `docs/out/en/index.html` and
  `docs/out/ko/index.html` (newer than the current `content/{en,ko}/index.mdx`),
  and the `docs.yml` `build` job (`pnpm build`) passes on `main`.

## §5.6 — Custom MDX component guard (EC-001 drill-down only)

Maps to REQ-DOCBLD-060.

- **Given** the primary deletions and config edit are applied AND the
  EC-001 secondary-cause drill-down has been triggered (the post-fix
  `pnpm build` still errors), so a custom MDX component
  (`AdapterCatalog`, `StatusBadge`, `CapabilitiesTable`) is implicated,
- **When** a static export exercises a page that mounts one of those
  components, including a case with a deliberately missing adapter key,
- **Then** the static export completes without a runtime error and the
  missing adapter key produces a safe fallback (no thrown exception).
- **And** under the primary smallest-surface path (EC-001 NOT triggered),
  this scenario imposes no code change — these components are not used by
  `/en` and read valid committed `_generated` JSON (NFR-DOCBLD-001).

---

## Edge cases

- **EC-001 — Secondary render cause after deletions.** If `pnpm build`
  still errors after removing `pages/`, the `i18n` key, and
  `theme.config.tsx`, drill into `content/en/index.mdx` and the custom MDX
  components (`AdapterCatalog`/`StatusBadge`/`CapabilitiesTable`) — though
  those are not used by `/en` and read valid committed JSON (§6). This is
  the only condition under which REQ-DOCBLD-060 mandates a code change.

- **EC-002 — Stale artifact masking.** Ensure `docs/out/` is deleted (or a
  clean checkout used) before the post-fix build so the May 31 artifact
  cannot masquerade as a passing build (false-positive, §1.3).

- **EC-003 — Patch preservation.** Confirm `pnpm install --frozen-lockfile`
  applies `docs/patches/nextra-theme-docs@4.6.1.patch`; a build failure due
  to a dropped patch is distinct from the migration-debris defect.

---

## Definition of Done

- [ ] Pre-fix reproduction captured the unminified `/en` failure stack
      (REQ-DOCBLD-040).
- [ ] `git ls-files docs/pages/` returns zero entries (REQ-DOCBLD-010).
- [ ] `docs/next.config.mjs` has no `i18n` key; `output:'export'`,
      `distDir:'out'`, `images.unoptimized`, `trailingSlash:true` preserved
      (REQ-DOCBLD-020).
- [ ] `docs/theme.config.tsx` deleted; no `theme.config` references remain
      (REQ-DOCBLD-030).
- [ ] Post-fix `pnpm build` exits 0 with fresh `out/{en,ko}/index.html`;
      `docs.yml` `build` job green on `main` (REQ-DOCBLD-050).
- [ ] REQ-DOCBLD-060 imposed no change under the primary path; addressed
      only if EC-001 was triggered.
- [ ] No dependency version, content, or App Router page changed
      (NFR-DOCBLD-001, NFR-DOCBLD-002).

---

*End of SPEC-DOC-003 acceptance criteria (draft).*
