---
id: SPEC-DOC-002
version: 0.2.0
status: draft
created: 2026-05-26
updated: 2026-05-31
author: limbowl (via manager-spec)
related_spec: SPEC-DOC-002 (spec.md, plan.md)
format: Given/When/Then
---

> v0.2.0 (2026-05-31): reconciled with spec.md HISTORY v0.2.0 code-spec
> corrections — (A1) HN page slug `hackernews`; (A2) social AST helper
> funcs; (A3) `x` disabled v0 stub; (A4) static `adapter-status.json`,
> staleness/schema-validation gates deferred.

# SPEC-DOC-002 Acceptance Scenarios

## 0. Document Purpose

This document specifies acceptance criteria for SPEC-DOC-002 in Given/When/Then format, expanding the scenario index in spec.md §5 (§5.1..§5.21) into externally-observable behaviors the run phase MUST verify before declaring DOC-002 ship-ready.

Scope: 18 acceptance criteria (AC-001..AC-018) covering REQ-ADPDOC-001 through REQ-ADPDOC-018 + NFR-ADPDOC-001 through NFR-ADPDOC-006, plus 3 edge-case sections, plus a Definition of Done checklist.

Coverage policy: every REQ and every NFR in spec.md §2 / §3 has ≥1 matching AC below. See Coverage Matrix at end of file.

---

## 1. Acceptance Criteria (Given/When/Then)

### AC-001 — Ten EN per-adapter pages with filename matching SourceID

Covers: REQ-ADPDOC-001

**Given** the built docs site with `docs/content/en/reference/adapters/` populated.

**When** the contributor runs `ls docs/content/en/reference/adapters/*.mdx`.

**Then**:
- Exactly 10 per-adapter MDX files exist, named by SourceID: `reddit.mdx`, `hackernews.mdx`, `arxiv.mdx`, `github.mdx`, `youtube.mdx`, `bluesky.mdx`, `x.mdx`, `searxng.mdx`, `naver.mdx`, `koreanews.mdx`. (The HN page is `hackernews.mdx` — `hn.go:101` `SourceID: "hackernews"` — NOT `hn.mdx`; the Go package dir `hn/` is not the SourceID.)
- Plus `index.mdx` and `errors.mdx` (12 MDX files total).
- `noop.mdx` / `reference.mdx` does NOT exist (test-only adapter, SourceID `reference`).
- For each file, the basename matches the `Capabilities().SourceID` value, resolved via the SourceID-keyed registry (handles `hn/`→`hackernews` and the two `social.go` helper funcs → `bluesky`/`x`), NOT the package directory name.
- `scripts/check-adapter-page-completeness.sh` exits `0`.

Maps to scenario §5.1 in spec.md.

---

### AC-002 — Each page has 10 H2 sections in the prescribed order

Covers: REQ-ADPDOC-002

**Given** each of the 10 per-adapter MDX files.

**When** the page is parsed to AST.

**Then**:
- Exactly 10 H2 (`## ...`) headings appear in this order:
  1. `## Status & Compatibility`
  2. `## Overview`
  3. `## Setup`
  4. `## Capabilities`
  5. `## Query syntax`
  6. `## Rate limits`
  7. `## Error reference`
  8. `## Troubleshooting`
  9. `## Version compatibility`
  10. `## Related`
- Each section heading appears exactly once per page.
- For non-auth adapters, the `## Setup` section contains the text "Authentication: not required — public endpoint" (it is NOT omitted).
- CI job `adapter-page-completeness` FAILS when any heading is missing or out of order.

Maps to scenario §5.2 in spec.md.

---

### AC-003 — Adapter catalog index with sortable table + filter

Covers: REQ-ADPDOC-003

**Given** built docs site at `/en/reference/adapters/`.

**When** the contributor visits the page and uses the category filter.

**Then**:
- The index page renders a `<AdapterCatalog>` table with 10 rows.
- Columns are: `Adapter`, `Status` (via `<StatusBadge>`), `Category`, `Auth required`, `Korean-locale optimized`, `Detail page`.
- Categories cover: `search-engine`, `social`, `academic`, `news`, `korean-locale`.
- Clicking "Category: news" narrows the table to `koreanews` and `naver` (the news + Korean-locale overlap).
- A "Common error categories" footnote links to `errors.mdx`.

Maps to scenario §5.3 in spec.md.

---

### AC-004 — Shared errors.mdx documents all 5 SourceError categories

Covers: REQ-ADPDOC-004

**Given** built docs site.

**When** the contributor visits `/en/reference/adapters/errors.mdx`.

**Then**:
- The page contains 5 H3 subsections, one per `*types.SourceError.Category` value: `CategoryPermanent`, `CategoryRateLimited`, `CategoryUnavailable`, `CategoryTransient`, `CategoryUnknown`.
- Each subsection contains 4 documented fields: triggering HTTP status codes, fanout dispatcher behaviour (with SPEC-FAN-001 cross-link), `RetryAfter` handling, one real adapter example.
- Each per-adapter page's `## Error reference` section links to this page; lychee resolves all 10 links.

Maps to scenario §5.4 in spec.md.

---

### AC-005 — StatusBadge renders correct lifecycle from JSON with boundary cases

Covers: REQ-ADPDOC-005

**Given** `_generated/adapter-status.json` with entry `{"bluesky": {"lifecycle": "stable", "successRate7d": 0.97, "verifiedAt": "2026-05-20T10:00:00Z"}}`.

**When** the contributor builds the docs and renders `/en/reference/adapters/bluesky`.

**Then**:
- A green `<StatusBadge>` is rendered with lifecycle "stable".
- The rendered output includes the 7-day success rate (0.97) and the `verifiedAt` timestamp.
- Unit test boundary: successRate = 0.949 renders `beta`, successRate = 0.950 renders `stable`.
- Lifecycle mapping rule: `stable` requires `status: implemented` AND rate ≥ 0.95; `beta` requires rate ∈ [0.80, 0.95); `disabled` covers flag-gated stubs with no live V1 path (e.g. `x`, `lifecycle: disabled`, grey badge); `deprecated` is reserved for post-V1. (There is NO `alpha` tier — A3.)
- `<StatusBadge>` reads the STATIC, DOC-002-owned `adapter-status.json` (A4 — EVAL-002 ships no such export).

Maps to scenario §5.5 in spec.md.

---

### AC-006 — Static adapter-status.json drives badges with fallback rendering

Covers: REQ-ADPDOC-006

**Given** the STATIC, hand-curated `_generated/adapter-status.json` (DOC-002-owned; A4 — EVAL-002 ships no export and no `lifecycle` field), and an entry deliberately missing the `lifecycle` field.

**When** the docs build runs.

**Then**:
- Every key in the file is a real adapter `SourceID`; `x` carries `lifecycle: disabled`.
- The malformed entry (missing `lifecycle`) causes `<StatusBadge>` to render the fallback "Status unknown" badge for that adapter; the build does NOT fail.
- Unknown adapter keys in the JSON are silently ignored.
- NOTE (A4): the live cron-published feed + a build-time JSON-Schema validation step (`adapter-status.schema.json`) + staleness automation are DEFERRED to a post-V1 EVAL-002 amendment; they are NOT verified at V1.

Maps to scenario §5.6 in spec.md.

---

### AC-007 — Drift CI gate fails on adapter Go source modification without JSON regeneration

Covers: REQ-ADPDOC-007, NFR-ADPDOC-001

**Given** `tools/gen-adapter-ref/main.go` + `scripts/gen-adapter-reference.sh` + CI workflow `docs.yml` containing the `gen-adapter-ref-drift` job.

**Given** the generator is driven by a SourceID-keyed registry (A2) that handles: standard `{pkg}.go` adapters, the `hn/`→`hackernews` slug (A1), and the `social.go` switch-dispatch over `blueskyCapabilities()` (`social.go:144`) / `xCapabilities()` (`social.go:164`) — emitting `bluesky` + `x` JSONs (no `bluesky.go`/`x.go`).

**When** the contributor:
- Case A: manually edits `_generated/reddit.capabilities.json` (artificial drift).
- Case B: modifies a real adapter's `RateLimitPerMin` (e.g., `blueskyCapabilities()` `RateLimitPerMin: 600` → `500` in `social.go`) without re-running the generator.
- Case C: clean state, no source changes.

**Then**:
- Case A: CI FAILS with diff showing the artificial modification.
- Case B: CI FAILS with diff showing the regenerated `bluesky.capabilities.json` differs from the committed JSON.
- Case C: CI PASSES; a clean run emits exactly 10 JSONs incl. `hackernews.capabilities.json` (from `hn/`), `bluesky.capabilities.json` (rate 600), and `x.capabilities.json` (rate 0).
- Drift gate runtime ≤ 60 seconds wall-clock on `ubuntu-24.04` (NFR-ADPDOC-001).
- Generator processes all 10 adapters in ≤ 5 seconds.

Maps to scenarios §5.7, §5.19 in spec.md.

---

### AC-008 — CapabilitiesTable renders auto-extracted fields with source path footer

Covers: REQ-ADPDOC-008

**Given** built docs site with `<CapabilitiesTable>` MDX component.

**When** the contributor renders each per-adapter page and inspects the table.

**Then**:
- Each page uses `<CapabilitiesTable src="_generated/{adapter}.capabilities.json" />`.
- The table displays 5 fields: `sourceID`, `requiresAuth`, `authEnvVars`, `rateLimitPerMin`, `defaultMaxResults`.
- The footer reads: "Extracted from `internal/adapters/{name}/{name}.go:NNN`" with the correct line number for each adapter.
- Grep verification: `grep -E "RateLimitPerMin|RequiresAuth" docs/content/en/reference/adapters/*.mdx` returns ZERO hits (no hardcoded values in MDX).

Maps to scenario §5.8 in spec.md.

---

### AC-009 — Bluesky + X documented as separate pages with shared callout

Covers: REQ-ADPDOC-009

**Given** built docs site.

**When** the contributor visits `/en/reference/adapters/bluesky` and `/en/reference/adapters/x`.

**Then**:
- Both pages exist as separate MDX files.
- `bluesky.mdx` shows `RateLimitPerMin: 600` (from the auto-extracted Capabilities).
- `x.mdx` shows `RateLimitPerMin: 0` and is framed as a DISABLED v0 stub (A3): `Status & Compatibility` badge is `disabled`; the Overview states "not available in V1 — no live path; requires `USEARCH_X_ENABLED=true`". It is NOT framed as "alpha"/"degraded".
- Each page contains a "Shared implementation notes" callout linking to the other.
- The `index.mdx` catalog renders 2 separate rows with `Category: social`.
- Shared notes (URL extraction, parse rules, scoring) are NOT duplicated — only cross-linked.

Maps to scenario §5.9 in spec.md.

---

### AC-010 — Auth-required adapter Setup sections contain all 5 fields

Covers: REQ-ADPDOC-010

**Given** built docs site.

**When** the contributor inspects `github.mdx`, `naver.mdx`, and `reddit.mdx` Setup sections.

**Then**:
- `github.mdx` Setup contains: (a) GitHub PAT registration URL (lychee-resolved), (b) env var names from `AuthEnvVars`, (c) recommended PAT scopes, (d) verification command (`usearch query "test" --source github`), (e) cross-link to SPEC-DEPLOY-001 Helm values key.
- `naver.mdx` Setup contains the same 5 fields with Naver-specific content.
- `reddit.mdx` (no-auth) Setup contains the text "Authentication: not required — public endpoint" with a 1-sentence explanation.
- The same pattern holds for the other 7 no-auth adapters.

Maps to scenario §5.10 in spec.md.

---

### AC-011 — Korean-locale adapter Setup includes 3-line summary + cross-link (no duplication)

Covers: REQ-ADPDOC-011

**Given** built docs site.

**When** the contributor inspects `naver.mdx` and `koreanews.mdx` Setup sections.

**Then**:
- `naver.mdx` Setup contains: Naver Developer Console "Service URL" registration note + UTF-8 verbatim Korean query note + DataLab endpoint distinction note + prominent cross-link to SPEC-DOC-001 `operators/korean-locale-setup.mdx` (KO-authoritative).
- `koreanews.mdx` Setup contains: EUC-KR legacy feed note + mecab-ko dedup note + KNC sidecar requirement note + cross-link.
- Neither page contains a full mecab-ko setup walkthrough (which lives in DOC-001).

Maps to scenario §5.11 in spec.md.

---

### AC-012 — Rate limits section contains all 4 elements per adapter

Covers: REQ-ADPDOC-012

**Given** built docs site.

**When** the contributor inspects the `## Rate limits` section on each of the 10 per-adapter pages.

**Then** each section contains all 4 elements:
- (a) Advertised `RateLimitPerMin` value (auto-imported via `<CapabilitiesTable>`).
- (b) Enforcement mechanism — one of: `in-process interval guard` (arxiv), `HTTP 429 response handling` (reddit/hackernews/github/naver/youtube/bluesky), `operator-configured per-feed` (koreanews), `none — self-hosted` (searxng), `none — disabled v0 stub` (x: no live path at V1). Each text matches research §1.4 verbatim.
- (c) Link to upstream provider's published quota documentation (lychee-resolved against the NFR-ADPDOC-005 allowlist).
- (d) Exhaustion behaviour: returns `CategoryRateLimited` with `RetryAfter` from upstream; SPEC-FAN-001 cross-link.

Maps to scenario §5.12 in spec.md.

---

### AC-013 — Error reference rosetta + shared link, with adapter-specific quirks

Covers: REQ-ADPDOC-013

**Given** built docs site.

**When** the contributor inspects the `## Error reference` section on each page.

**Then**:
- Each page links to `errors.mdx`.
- Each page contains a rosetta table with columns: `HTTP status` / `Category` / `Cause` / `Operator action`.
- `github.mdx` rosetta lists a row for HTTP 422 ("Validation failed" → `CategoryPermanent`).
- `naver.mdx` rosetta lists HTTP 401 ("Invalid client id" → `CategoryPermanent`, operator action mentions `NAVER_CLIENT_ID` env var).
- Each page lists the status codes handled by the adapter's `categorizeStatus`-style function.

Maps to scenario §5.13 in spec.md.

---

### AC-014 — Troubleshooting sections meet entry counts and 5-field format

Covers: REQ-ADPDOC-014

**Given** built docs site.

**When** the contributor inspects each page's `## Troubleshooting` section.

**Then**:
- Each page has ≥ 3 entries in the 5-field format: `Symptom` / `Likely cause` / `Diagnostic command` / `Resolution` / `Related SPECs`.
- `koreanews.mdx` has ≥ 5 entries (due to multi-source aggregation complexity).
- Every "Related SPECs" field contains valid SPEC ID links resolved by lychee internal-strict.

Maps to scenario §5.14 in spec.md.

---

### AC-015 — Version compatibility tables with lastVerified frontmatter + staleness warn

Covers: REQ-ADPDOC-015

**Given** built docs site at V1.0.0 ship time (2026-05-22).

**When** the contributor inspects each page's `## Version compatibility` section and frontmatter.

**Then**:
- Each page has a non-empty Version compatibility table with at least 1 verified-against row.
- Each row contains: `usearch version`, `upstream provider API version`, `last verified date`, `verifier` (name from CONTRIBUTING.md log).
- Page frontmatter contains `lastVerified: YYYY-MM-DD` within 90 days of release (≥ 2026-02-21).
- Backdating one page's `lastVerified` to 200 days produces a CI WARNING (not failure).

Maps to scenario §5.15 in spec.md.

---

### AC-016 — Related sections contain ≥ 4 cross-links each, 100% resolved

Covers: REQ-ADPDOC-016

**Given** built docs site.

**When** lychee internal-strict link-check runs on all 10 per-adapter pages.

**Then**:
- Each page's `## Related` section contains ≥ 4 cross-links: (a) SPEC-ADP-XXX document, (b) SPEC-DOC-001 `end-users/surface-comparison.mdx`, (c) SPEC-DEPLOY-001 `operators/deployment-helm.mdx` with anchor, (d) any body-referenced SPECs (FAN-001, CACHE-001, IDX-003, EVAL-002).
- Total internal links across the 10 pages ≥ 40.
- lychee resolves 100% of internal links (zero broken).

Maps to scenario §5.16 in spec.md.

---

### AC-017 — KO Tier-1: 4 hand-translated pages with reviewer signoff, coverage gate

Covers: REQ-ADPDOC-017, NFR-ADPDOC-006

**Given** built docs site at V1.0.0.

**When** the contributor enumerates `docs/content/ko/reference/adapters/` and runs `scripts/check-bilingual-coverage.sh`.

**Then**:
- Exactly 4 KO MDX files exist: `index.mdx`, `naver.mdx`, `koreanews.mdx`, `errors.mdx`.
- `docs/content/ko/CONTRIBUTING.md` reviewer log contains ≥ 1 named native-Korean reviewer per Tier-1 page.
- Bilingual coverage gate PASSES at V1.0.0 (the 8 Tier-2 EN-only pages are explicitly excluded from the 90% gate by the amended script).
- Deleting `content/ko/reference/adapters/naver.mdx` drops Tier-1 coverage below threshold and FAILS CI.
- Deleting any of the 8 Tier-2 EN-only pages does NOT fail (still EN-authoritative).
- Average Tier-1 page review turnaround ≤ 5 calendar days over the 4-page batch (tracked in CONTRIBUTING.md).

Maps to scenario §5.17 in spec.md.

---

### AC-018 — Credential placeholder lint catches realistic-shaped patterns

Covers: REQ-ADPDOC-018

**Given** `scripts/check-doc-credentials.sh` with regex set aligned to SPEC-SEC-001 D2 `.gitleaks.toml`.

**When** the contributor:
- Case A: injects a realistic-shaped GitHub PAT pattern (e.g., `ghp_abc123...40chars`) into `github.mdx`.
- Case B: injects `<YOUR_GITHUB_TOKEN>` into `github.mdx`.
- Case C: runs the script against the clean V1.0.0 baseline.

**Then**:
- Case A: CI FAILS, the script reports the file + line + matched pattern.
- Case B: CI PASSES (placeholder format is allowed).
- Case C: zero matches across all 12 pages (EN + KO Tier-1).
- The regex set covers at minimum: AWS access key prefixes, GitHub PAT prefixes, 40-char hex tokens outside fenced code blocks, JWT-shaped strings, Naver client secret format.

Maps to scenario §5.18 in spec.md.

---

## 2. Edge Cases

### EC-001 — adapter-status.json staleness gate (DEFERRED post-V1)

Covers: NFR-ADPDOC-003 (deferred)

**Given** the V1 `_generated/adapter-status.json` is a STATIC, hand-curated file (A4) — an mtime-based staleness gate is meaningless for a manually-edited file.

**When** the docs build runs at V1.

**Then**:
- There is NO `adapter-status-staleness` CI job and NO `docs/stale-adapter-status` GitHub-Issue automation at V1 — both are DEFERRED to the post-V1 EVAL-002 live-export amendment.
- V1 freshness is instead asserted by the per-page `lastVerified` frontmatter staleness warn (REQ-ADPDOC-015, part of `adapter-page-completeness`).
- When the live EVAL-002 export lands post-V1, this mtime gate is re-activated and this edge case becomes testable as originally written.

Maps to scenario §5.20 in spec.md (deferred).

### EC-002 — Page falls below 50-character per-section threshold

Covers: NFR-ADPDOC-004

**Given** `reddit.mdx` with the Troubleshooting section deliberately blanked (< 50 chars plain text after stripping code blocks + frontmatter).

**When** the `adapter-page-completeness` CI job runs.

**Then**:
- The job FAILS naming the page + section.
- Restoring the section content (≥ 50 plain-text chars) returns the job to PASS.
- The threshold value is implementation-defined per Open Question §8.8 in spec.md.

Maps to scenario §5.21 in spec.md.

### EC-003 — New adapter shipped without matching MDX page

**Given** a contributor adds a new adapter `internal/adapters/newsource/newsource.go` with `SourceID: "newsource"` but does NOT add `docs/content/en/reference/adapters/newsource.mdx`.

**When** CI runs `scripts/check-adapter-page-completeness.sh`.

**Then**:
- The script FAILS, listing the new SourceID as missing a page.
- The contributor is instructed to either add the MDX file or remove the adapter from the production set.
- The drift gate (gen-adapter-ref-drift) ALSO fails because the `_generated/newsource.capabilities.json` is missing.

---

## 3. Definition of Done Checklist

- [ ] All 18 AC scenarios pass on a fresh CI run.
- [ ] All 21 scenario index entries (§5.1..§5.21) in spec.md are implemented.
- [ ] 12 EN MDX files exist (10 adapters + `index.mdx` + `errors.mdx`).
- [ ] 4 KO Tier-1 MDX files exist with reviewer signoff.
- [ ] `tools/gen-adapter-ref/main.go` + `scripts/gen-adapter-reference.sh` + `scripts/check-adapter-page-completeness.sh` + `scripts/check-doc-credentials.sh` are committed and executable.
- [ ] `.github/workflows/docs.yml` extended with `gen-adapter-ref-drift`, `adapter-page-completeness`, `check-doc-credentials` jobs.
- [ ] SPEC-DOC-001 `scripts/check-bilingual-coverage.sh` amended to recognize the `reference/adapters/` Tier-1 set.
- [ ] SPEC-DOC-001 `docs/lychee.toml` extended with the provider-doc allowlist (NFR-ADPDOC-005).
- [ ] STATIC `_generated/adapter-status.json` is committed; every key is a real SourceID; `x` carries `lifecycle: disabled`. (A4: `adapter-status.schema.json` build-time validation is DEFERRED to a post-V1 EVAL-002 amendment, NOT a V1 DoD item.)
- [ ] Drift CI runtime ≤ 60 seconds; page-completeness CI runtime ≤ 30 seconds; combined SPEC-DOC-001 + DOC-002 docs CI ≤ 6 minutes.
- [ ] Clean credential-lint baseline across all 12 pages.
- [ ] Open Questions in spec.md §8 are resolved or explicitly deferred with mitigation.

---

## 4. Coverage Matrix (REQ → AC)

| REQ / NFR | AC-001 | AC-002 | AC-003 | AC-004 | AC-005 | AC-006 | AC-007 | AC-008 | AC-009 | AC-010 | AC-011 | AC-012 | AC-013 | AC-014 | AC-015 | AC-016 | AC-017 | AC-018 | EC |
|-----------|--------|--------|--------|--------|--------|--------|--------|--------|--------|--------|--------|--------|--------|--------|--------|--------|--------|--------|----|
| REQ-ADPDOC-001 | ✓ |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   | EC-003 |
| REQ-ADPDOC-002 |   | ✓ |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   |
| REQ-ADPDOC-003 |   |   | ✓ |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   |
| REQ-ADPDOC-004 |   |   |   | ✓ |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   |
| REQ-ADPDOC-005 |   |   |   |   | ✓ |   |   |   |   |   |   |   |   |   |   |   |   |   |   |
| REQ-ADPDOC-006 |   |   |   |   |   | ✓ |   |   |   |   |   |   |   |   |   |   |   |   |   |
| REQ-ADPDOC-007 |   |   |   |   |   |   | ✓ |   |   |   |   |   |   |   |   |   |   |   |   |
| REQ-ADPDOC-008 |   |   |   |   |   |   |   | ✓ |   |   |   |   |   |   |   |   |   |   |   |
| REQ-ADPDOC-009 |   |   |   |   |   |   |   |   | ✓ |   |   |   |   |   |   |   |   |   |   |
| REQ-ADPDOC-010 |   |   |   |   |   |   |   |   |   | ✓ |   |   |   |   |   |   |   |   |   |
| REQ-ADPDOC-011 |   |   |   |   |   |   |   |   |   |   | ✓ |   |   |   |   |   |   |   |   |
| REQ-ADPDOC-012 |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |   |   |   |   |   |   |
| REQ-ADPDOC-013 |   |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |   |   |   |   |   |
| REQ-ADPDOC-014 |   |   |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |   |   |   |   |
| REQ-ADPDOC-015 |   |   |   |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |   |   |   |
| REQ-ADPDOC-016 |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |   |   |
| REQ-ADPDOC-017 |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |   |
| REQ-ADPDOC-018 |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |
| NFR-ADPDOC-001 |   |   |   |   |   |   | ✓ |   |   |   |   |   |   |   |   |   |   |   |   |
| NFR-ADPDOC-002 |   | ✓ |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   |
| NFR-ADPDOC-003 |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   | EC-001 |
| NFR-ADPDOC-004 |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   | EC-002 |
| NFR-ADPDOC-005 |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |   |   | ✓ |   |   |   |
| NFR-ADPDOC-006 |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |   |

Every REQ and NFR has ≥ 1 AC or EC; edge cases EC-001..EC-003 supplement NFRs and the file-completeness invariant.

---

*End of SPEC-DOC-002 acceptance.md.*
