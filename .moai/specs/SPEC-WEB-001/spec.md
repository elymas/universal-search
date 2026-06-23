---
id: SPEC-WEB-001
version: 0.1.0
status: implemented
created: 2026-06-23
updated: 2026-06-24
author: limbowl
priority: P3
issue_number: 0
title: Web CI green — verify tsc devDeps + eslint config resolution (false-positive triage)
milestone: CI debt
owner: expert-frontend
methodology: ddd
coverage_target: 0
depends_on: []
blocks: []
related: [SPEC-SEC-001]
---

# SPEC-WEB-001: Web CI green — verify tsc devDeps + eslint config resolution

## HISTORY

- 2026-06-23 (initial draft v0.1.0, limbowl via manager-spec):
  CI-debt remediation SPEC for the reported Web CI failures: (1) `tsc`
  failing to find type declarations for `vitest` / `@testing-library/*` /
  `react/jsx-runtime`, and (2) `eslint` exiting 254 on config/plugin
  resolution. A focused investigation of the LIVE codebase found that
  **all five investigated loci are false-positives** — the reported
  signature does NOT reproduce against current `main`. This SPEC is
  therefore VERIFICATION-FIRST: its primary outcome is confirming the
  Web CI workflow is green on latest `main`, NOT a code change. No
  remediation code change is warranted on the basis of the investigation
  alone (see §4 root cause). Optional hardening (frozen-lockfile,
  tsconfig types allowlist) is captured as out-of-scope-by-default
  follow-up, gated on a future observed regression.

  Methodology: DDD (analyze existing CI surface, characterize the
  green/red state, change nothing unless a real failure reproduces).
  Coverage target N/A (no application code under test in this SPEC).
  Owner: expert-frontend.

---

## 1. Overview

SPEC-WEB-001 triages reported Web CI debt: `tsc --noEmit` (`pnpm
typecheck`) allegedly failing on missing test type declarations, and
`eslint` (`pnpm lint`) allegedly exiting 254 on config/plugin resolution.

The investigation verified five candidate loci against the live codebase.
**Every one is a false-positive** — the proposed root cause ("CI install
omits devDependencies, or tsconfig pulls in test files without test type
packages") is FALSE for the present code, and running the exact CI
scripts locally against a full install yields `pnpm typecheck` exit 0 and
`pnpm lint` exit 0.

The SPEC's deliverable is **evidence**, not code: re-run the actual Web CI
workflow on latest `main` and record the conclusion. If green, close as
already-resolved.

### 1.1 Investigated loci and verdicts

| # | Locus | Hypothesis | Verdict |
|---|-------|------------|---------|
| 1 | `web/package.json:28-46` (devDependencies) | test type packages (vitest, @testing-library/*) missing → tsc cannot find declarations | **FALSE-POSITIVE** — all declared: `@testing-library/jest-dom ^6.9.1`, `@testing-library/react ^16.3.2`, `@testing-library/user-event ^14.6.1`, `vitest ^4.1.7`; `@types/react ^19` supplies `react/jsx-runtime` |
| 2 | `web/tsconfig.json:25-32` (include/exclude, no `types` array) | include globs pull in `src/__tests__` + admin `__tests__` without test type packages | **FALSE-POSITIVE** — no `types` array means tsc relies on installed `@types`, which ARE present; `tsc --noEmit` exits 0 on full install |
| 3 | `.github/workflows/web.yml:30-34` (Install + Type check) | CI install omits devDependencies → tsc cannot resolve declarations | **FALSE-POSITIVE** — install step is `pnpm --dir web install --no-frozen-lockfile`; no `--prod`/`--production`, no `NODE_ENV=production`, no `.npmrc` prune → pnpm installs devDependencies by default |
| 4 | `.github/workflows/web.yml:36-37` (Lint) + `web/eslint.config.mjs:1-10` | eslint exits 254 on config/plugin resolution failure | **FALSE-POSITIVE** — `eslint.config.mjs` imports `eslint-config-next/core-web-vitals` (valid subpath export, confirmed in `node_modules/eslint-config-next/package.json`) + `eslint-config-prettier`; `pnpm lint` exits 0 |
| 5 | `web/` vs `pnpm-workspace.yaml` + `web/pnpm-lock.yaml` | workspace/lockfile mismatch could prune web devDeps in CI | **FALSE-POSITIVE** — `pnpm-workspace.yaml packages: [docs]` only (web deliberately excluded, self-managed); `web/pnpm-lock.yaml` is a standalone single-importer (lockfileVersion 9.0); `pnpm install --frozen-lockfile --offline` succeeds (lockfile in sync) |

### 1.2 Classification flag

[HARD] **All five loci are FALSE-POSITIVE (non-reproducing).** This SPEC
does NOT prescribe a fix; it prescribes verification. The most likely
explanations for the original CI-debt signature are: (a) it was already
fixed by an earlier commit on the web/CI path (git history shows multiple
`fix(ci)`/`fix(lint)` web commits, e.g. `97c8766` unblock pre-commit
eslint wrapper, `3150722` pin pnpm v10, `f31f48c` Next bump) and the
snapshot predates those fixes, or (b) a transient CI-environment artifact
(e.g., an earlier missing `pnpm/action-setup` pin or a partial/cached
install) rather than a defect in the checked-in config.

This is recorded as a **needs-decision** item: after the live CI re-run,
classify the signature as already-fixed (close) vs flaky (open a
flakiness follow-up) vs (unexpectedly) reproducing (escalate to a real
fix). See §6.2.

---

## 2. EARS Requirements

EARS REQ ids use the form `REQ-WEB-NNN`, numbered 10/20/30…. Every
requirement is testable. The headline acceptance for this CI-debt SPEC is
"the named CI job (`web.yml`) passes on `main`".

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-WEB-010** | Event-Driven | WHEN the Web CI workflow (`.github/workflows/web.yml`) runs on the latest `main`, the system SHALL complete with conclusion `success`, with both the Type check step (`pnpm --dir web typecheck`) AND the Lint step (`pnpm --dir web lint`) exiting 0. | P3 | A triggered run of `web.yml` on `main` reports conclusion `success`; the typecheck and lint steps both show exit 0 in the run log. The run URL + conclusion are recorded. |
| **REQ-WEB-020** | State-Driven | WHILE the Web CI install step runs, the system SHALL install devDependencies (the install step SHALL remain free of `--prod`, `--production`, `NODE_ENV=production`, and any `.npmrc` prune setting) so that `tsc` and `eslint` can resolve test-only type packages. | P3 | Inspection of `.github/workflows/web.yml` confirms the install step contains none of `--prod`/`--production`/`NODE_ENV=production`/prune; the subsequent typecheck step finds all type declarations (exit 0). |
| **REQ-WEB-030** | Event-Driven | WHEN `pnpm --dir web typecheck` runs in CI against a full install, the system SHALL exit 0 with zero TypeScript type-declaration errors for `vitest`, `@testing-library/react`, `@testing-library/user-event`, `@testing-library/jest-dom`, and `react/jsx-runtime` across `src/__tests__` and `src/app/admin/_components/__tests__`. | P3 | The typecheck step exits 0 with zero `TS2307`/`Cannot find module`/`Cannot find type definition` errors for the five named packages across both test directories. |
| **REQ-WEB-040** | Event-Driven | WHEN `pnpm --dir web lint` runs in CI, the system SHALL exit 0 and SHALL resolve `eslint-config-next/core-web-vitals` and `eslint-config-prettier` without an exit-254 configuration-resolution error. | P3 | The lint step exits 0; the run log shows no exit code 254 and no "Cannot find config / failed to load plugin" error for the two named configs. |
| **REQ-WEB-050** | State-Driven | WHILE the Web CI install step runs, the system SHALL pin pnpm (via `pnpm/action-setup` or equivalent) to a major version compatible with the `web/pnpm-lock.yaml` `lockfileVersion` (9.0) before invoking install. | P3 | Inspection of `web.yml` confirms a pnpm version pin (major 10, compatible with lockfileVersion 9.0) precedes the install step; install succeeds against the existing lockfile. |
| **REQ-WEB-060** | Ubiquitous | The `web` package SHALL remain excluded from the root `pnpm-workspace.yaml` and SHALL be installed via its own standalone `web/pnpm-lock.yaml`, so that root-workspace resolution cannot prune its devDependencies. | P3 | `pnpm-workspace.yaml` `packages:` lists `docs` only (no `web`); `web/pnpm-lock.yaml` is a single-importer lockfile. The lockfile-in-sync claim — that `pnpm --dir web install --frozen-lockfile --offline` succeeds — is ASSERTED by the investigation but NOT yet reproduced; the run phase MUST execute this command and capture its exit code as evidence (do not treat the success claim as pre-verified). |
| **REQ-WEB-070** | Note (non-normative) | DEFERRED HARDENING (non-normative; not an EARS-binding requirement): WHERE the `web/pnpm-lock.yaml` is in sync with `web/package.json`, switching the Web CI install step to `--frozen-lockfile` (instead of `--no-frozen-lockfile`) would guarantee a reproducible, devDependency-complete install. This hardening is OUT OF SCOPE by default and is recorded only as a candidate follow-up to be applied IF a future CI-environment regression is observed; it carries no `SHALL`/`SHOULD` obligation while CI is green. | P3 | If applied: `web.yml` install step uses `--frozen-lockfile` and the run still completes `success`. If not applied (default): this note is recorded as deferred hardening with rationale (lockfile already in sync; current `--no-frozen-lockfile` install is devDependency-complete). |

---

## 3. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| **NFR-WEB-001** | No application code change | This SPEC SHALL NOT modify `web/package.json`, `web/tsconfig.json`, or any `web/src/**` source — the investigation confirmed devDependencies and tsconfig are correct. Any change is limited to `.github/workflows/web.yml` (optional hardening only) and SHALL be applied solely in response to an observed regression, not on the basis of this triage. |
| **NFR-WEB-002** | Evidence-first closure | Closure of this SPEC SHALL be backed by a recorded live Web CI run (run URL + conclusion) on `main`, not by local reproduction alone. |

---

## 4. Root Cause

The cited Web CI failures (tsc missing type declarations for
`vitest`/`@testing-library/*`/`react/jsx-runtime`; eslint exit 254) do
**NOT reproduce** against the current live codebase. All five
config/CI loci (§1.1) were verified and the proposed root cause ("CI
install omits devDependencies, or tsconfig includes test files without
test type packages") is FALSE for the present code:

1. Every cited type package is declared in `web/package.json`
   devDependencies and physically resolves in `node_modules`.
2. The CI install step uses `pnpm --dir web install --no-frozen-lockfile`
   with no `--prod`/`--production`, no `NODE_ENV=production`, and no
   `.npmrc` prune setting, so devDependencies are installed before the
   typecheck and lint steps.
3. Running the exact CI scripts locally against a full install yields
   `pnpm typecheck` exit 0 and `pnpm lint` exit 0.
4. The `web` package is intentionally excluded from the root pnpm
   workspace and carries a synced standalone lockfile (frozen + offline
   install succeeds).

Most likely explanation for the original signature: **(a) already fixed**
by an earlier `fix(ci)`/`fix(lint)` web commit (the snapshot predates
those fixes), or **(b) a transient CI-environment artifact** (earlier
missing `pnpm/action-setup` pin or a partial/cached install) rather than a
defect in the checked-in config. **No code change is warranted on the
basis of this finding alone.**

---

## 5. Acceptance Criteria

For this CI-debt SPEC, the headline acceptance is **"the named CI job
(`web.yml`) passes on `main`"** (REQ-WEB-010). Per-REQ acceptance is
inline in §2. Scenario index:

| Scenario | Description | Coverage |
|----------|-------------|----------|
| §5.1 | **Headline — live CI green**: trigger / inspect the latest `.github/workflows/web.yml` run on `main`; assert conclusion `success`; capture run URL + conclusion. | REQ-WEB-010 |
| §5.2 | **Install includes devDeps**: inspect `web.yml` install step; assert no `--prod`/`--production`/`NODE_ENV=production`/`.npmrc` prune; assert subsequent typecheck resolves all declarations. | REQ-WEB-020 |
| §5.3 | **Typecheck clean for named packages**: `pnpm --dir web typecheck` exits 0 with zero declaration errors for vitest, @testing-library/react, @testing-library/user-event, @testing-library/jest-dom, react/jsx-runtime across `src/__tests__` + `src/app/admin/_components/__tests__`. | REQ-WEB-030 |
| §5.4 | **Lint clean, no exit 254**: `pnpm --dir web lint` exits 0; `eslint-config-next/core-web-vitals` + `eslint-config-prettier` resolve without exit-254. | REQ-WEB-040 |
| §5.5 | **pnpm version compatible**: `web.yml` pins pnpm to a major compatible with lockfileVersion 9.0 before install; install succeeds. | REQ-WEB-050 |
| §5.6 | **Workspace isolation**: `pnpm-workspace.yaml packages:` is `[docs]` only; `web/pnpm-lock.yaml` is single-importer. The run phase MUST execute `pnpm --dir web install --frozen-lockfile --offline` and capture its exit code as evidence (asserted-not-reproduced). | REQ-WEB-060 |
| §5.7 | **Optional hardening (deferred)**: IF a future regression is observed, switch install to `--frozen-lockfile` and confirm run still `success`; ELSE record REQ-WEB-070 as deferred with rationale. | REQ-WEB-070 |

---

## 6. Dependencies, Blocks & Blockers

### 6.1 SPEC dependencies

- `depends_on`: none.
- `blocks`: none.
- `related`: **SPEC-SEC-001** — sibling CI-debt / CI-workflow context
  (security CI stage); no hard code dependency.

### 6.2 Blockers (investigation-recorded)

The investigation recorded two blockers that prevent a reproduction-first
fix and force this SPEC to be verification-first:

1. **Cannot reproduce the failing signature locally** — a full install
   yields `tsc` exit 0 and `eslint` exit 0, so there is no failing state
   to drive a reproduction-first fix. (Per CLAUDE.md Rule 4, a fix
   requires a reproducing test; none exists.)
2. **Cannot inspect the original failing CI run logs from a read-only
   session** — confirming whether the failure predates the existing
   web/CI fix commits or was a transient environment issue requires a
   **live re-run of the Web CI workflow** to decide already-fixed vs
   false-positive vs flaky. This is the needs-decision gate (§1.2).

### 6.3 External dependencies

None.

---

## 7. Exclusions (What NOT to Build)

[HARD] The following are explicitly OUT OF SCOPE. Each has a known
destination / rationale.

- **Any change to `web/package.json` devDependencies.** → NOT needed; the
  investigation confirmed all cited type packages (vitest,
  @testing-library/*, @types/react) are declared and resolve. Modifying
  them would be churn against a correct file (NFR-WEB-001).

- **Any change to `web/tsconfig.json` (e.g., adding a `types` allowlist
  or a dedicated `tsconfig.test.json`).** → OPTIONAL hardening only, and
  only if test-only types ever conflict with the app build — NOT needed
  today (`tsc --noEmit` exits 0). Deferred follow-up, not this SPEC.

- **Any change to `web/eslint.config.mjs` or `web/src/**` source.** →
  `pnpm lint` exits 0; the eslint config subpath imports resolve. No
  defect to remediate.

- **A reproduction-first code fix.** → BLOCKED: the signature does not
  reproduce locally (§6.2), so there is no failing test to write first.
  This SPEC ships verification evidence, not a fix.

- **Mandatory switch to `--frozen-lockfile`.** → OPTIONAL hardening
  (REQ-WEB-070), applied only on an observed future regression. The
  current `--no-frozen-lockfile` install is devDependency-complete and
  the lockfile is in sync.

- **Removing `web` from / adding `web` to the root pnpm workspace.** →
  `web` is deliberately excluded and self-manages a standalone lockfile
  (REQ-WEB-060). No workspace restructuring.

- **Broader Web CI debt** (Next.js build/test job hardening, web eslint/
  prettier pre-commit multi-linter debt). → Tracked separately in the
  deferred CI-debt backlog; out of scope for this focused
  typecheck/lint-resolution triage.

- **GitHub Issue tracking on this SPEC** (`issue_number: 0`). → CI-debt
  triage pattern; no issue opened.

---

## 8. References

Internal (project files cited by the investigation):

- `.github/workflows/web.yml` (Install / Type check / Lint steps — the
  CI surface under verification)
- `web/package.json` (devDependencies — confirmed complete)
- `web/tsconfig.json` (include/exclude, no `types` array — confirmed OK)
- `web/eslint.config.mjs` (eslint flat config — confirmed resolves)
- `web/pnpm-lock.yaml` (standalone single-importer lockfile, version 9.0)
- `pnpm-workspace.yaml` (`packages: [docs]` — web deliberately excluded)
- `node_modules/eslint-config-next/package.json` (confirms
  `core-web-vitals` subpath export)

Sibling SPEC:

- `.moai/specs/SPEC-SEC-001/spec.md` (frontmatter/section template;
  related CI-workflow context)

---

*End of SPEC-WEB-001 v0.1.0 (draft).*
