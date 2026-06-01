---
id: SPEC-REL-001
version: 0.2.0
status: draft
created: 2026-05-26
updated: 2026-05-31
author: limbowl (via manager-spec)
related_spec: SPEC-REL-001 (spec.md, plan.md)
format: Given/When/Then
---

# SPEC-REL-001 Acceptance Scenarios

## 0. Document Purpose

This document specifies acceptance criteria for SPEC-REL-001 in Given/When/Then format, complementing the EARS requirement table in spec.md §2 and the test-scenario list in spec.md §5 (S1..S12). The criteria below are the externally-observable behaviors that the run phase MUST verify before declaring REL-001 release-ready.

Scope: 13 acceptance criteria (AC-001..AC-013) covering REQ-REL-001 through REQ-REL-018 + NFR-REL-001 through NFR-REL-007, plus 3 edge-case sections, plus a Definition of Done checklist.

Coverage policy: every REQ and every NFR in spec.md §2 / §3 has ≥1 matching AC below. See Coverage Matrix at end of file.

---

## 1. Acceptance Criteria (Given/When/Then)

### AC-001 — `internal/version/` package consumes ldflags injection

Covers: REQ-REL-001, REQ-REL-002, NFR-REL-002

**Given** the `internal/version/version.go` package with default exports `Version = "0.1.0-dev"`, `Commit = "unknown"`, `BuildDate = "unknown"`.

**When** the build command is executed:
```
go build -ldflags "-X github.com/elymas/universal-search/internal/version.Version=1.0.0 -X github.com/elymas/universal-search/internal/version.Commit=abc123 -X github.com/elymas/universal-search/internal/version.BuildDate=2026-05-22T12:00:00Z" ./cmd/usearch
```

**Then**:
- The resulting binary's `--version` output contains `usearch v1.0.0`, matching the actual `semverPattern` `^usearch v\d+\.\d+\.\d+` (prefix-anchored, no trailing `$`, no prerelease group — per `cmd/usearch/main_test.go:12`).
- `internal/version.Commit` returns `"abc123"`.
- `internal/version.BuildDate` returns `"2026-05-22T12:00:00Z"`.
- Re-running the build with identical ldflags produces a binary that reports the same version values (NFR-REL-002 determinism).

Maps to scenarios S1, S2 in spec.md §5.

---

### AC-002 — Existing `TestVersionFlag` continues to PASS after refactor

Covers: REQ-REL-002

**Given** the refactored `cmd/usearch/main.go` consuming `internal/version.Version` instead of a local literal.

**When** the contributor runs:
```
go test -run TestVersionFlag ./cmd/usearch/...
```

**Then**:
- The test passes WITHOUT ldflags injection (default `0.1.0-dev`).
- The assertion `usearch --version` output matches the actual `semverPattern` `^usearch v\d+\.\d+\.\d+` (per `cmd/usearch/main_test.go:12` — prefix-anchored, no `$`, no prerelease group).
- The characterization test is preserved byte-for-byte: `main_test.go` is NOT modified by the refactor (HARD characterization requirement — the regex is preserved as-is, not tightened).

Maps to scenario S2 in spec.md §5.

---

### AC-003 — Three binaries (`usearch`, `usearch-api`, `usearch-mcp`) report identical version

Covers: REQ-REL-001

**Given** ldflags injection of `Version=1.0.0 Commit=abc123` applied to all three binaries in the same goreleaser run.

**When** the operator invokes:
```
./usearch --version
./usearch-api --version    # or equivalent
./usearch-mcp --version    # or equivalent
```

**Then**:
- All three outputs report `1.0.0` consistently (no drift).
- All three report commit `abc123`.
- The version values are sourced from the single `internal/version/` package (no duplicate literals in any cmd/).

Maps to scenario S3 in spec.md §5.

---

### AC-004 — CHANGELOG [1.0.0] section completeness

Covers: REQ-REL-003, NFR-REL-005

**Given** the released `CHANGELOG.md` with `[1.0.0] - YYYY-MM-DD` section.

**When** the verification command is executed:
```
SPEC_COUNT=$(awk '/^## \[1\.0\.0\]/,/^## \[/' CHANGELOG.md | grep -c "SPEC-")
IMPL_COUNT=$(grep -l "status: implemented" .moai/specs/SPEC-*/spec.md | wc -l)
```

**Then**:
- `SPEC_COUNT` ≥ `IMPL_COUNT` (every implemented SPEC has at least one mention in the 1.0.0 section).
- The 1.0.0 section follows the Keep-a-Changelog format with subsections (Added, Changed, Deprecated, Removed, Fixed, Security).
- Every SPEC ID referenced is a valid SPEC directory under `.moai/specs/`.

Maps to scenario S4 in spec.md §5.

---

### AC-005 — MIGRATION.md exactly 12 sections in prescribed order

Covers: REQ-REL-004

**Given** the released `MIGRATION.md`.

**When** the structural validation script extracts all `^## ` headers.

**Then**:
- Exactly 12 sections appear in the canonical order defined by spec.md HISTORY D4 / REQ-REL-004: §1 Overview, §2 CLI breaking changes, §3 Config schema breaking changes, §4 Env var renames, §5 MCP protocol surface, §6 Adapter plugin contract, §7 MoAI Skill manifest, §8 REST/GraphQL endpoint schema, §9 Database schema migration policy, §10 Adapter status taxonomy reference, §11 Upgrade procedure, §12 Rollback procedure.
- Each section is non-empty (≥ 30 plain-text characters after stripping code blocks); sections with no current known breaking change explicitly state "v1.0.0 — no breaking changes in this category".
- §11 Upgrade procedure commands (helm / go install / Skill reinstall) are runnable (lint-checked for shell syntax).

Maps to scenario S5 in spec.md §5.

---

### AC-006 — Goreleaser produces exactly 12 archives + checksums + SBOM

Covers: REQ-REL-006

**Given** `.goreleaser.yml` configured per REQ-REL-006 and a `v1.0.0` tag.

**When** the maintainer runs:
```
goreleaser release --clean --skip=publish
```

**Then**:
- The `dist/` directory contains exactly 12 archive files matching the pattern `usearch_1.0.0_{linux,darwin}_{amd64,arm64}.tar.gz` (cross-product: 3 binaries × 2 OS × 2 arch).
- `SHA256SUMS` is generated covering all 12 archives.
- One SPDX SBOM file is generated per archive (or one aggregate SBOM, per goreleaser config).
- `goreleaser check` exits `0` (config validates).

Maps to scenario S6 in spec.md §5.

---

### AC-007 — Pre-tag verification matrix gate failure aborts release

Covers: REQ-REL-007, REQ-REL-009

**Given** the `release.yml` workflow triggered by a `v1.0.0` tag push, with G6 (EVAL trio gate) failing (e.g., EVAL-001 faithfulness score 0.82, below the 0.85 threshold).

**When** the workflow executes.

**Then**:
- The `pre-tag-verify` job FAILS at G6.
- Subsequent jobs (goreleaser build, SLSA, cosign, GitHub Release publish) do NOT execute.
- No GitHub Release is created.
- The workflow status is `failure`.
- The maintainer is notified of the specific failing gate.

Maps to scenario S7 in spec.md §5.

---

### AC-008 — Successful release publishes GitHub Release with all artifacts

Covers: REQ-REL-007, REQ-REL-008, REQ-REL-010, REQ-REL-011, REQ-REL-012, REQ-REL-013, REQ-REL-014, REQ-REL-016, REQ-REL-017

Verification timing: this AC is a Sprint-2 / release-ceremony acceptance (requires a real signed tag push + merged dependency workflows). It is NOT verified at implementation time; the implementation-time acceptance is the dry-run/snapshot/lint coverage in AC-006, AC-012, and gates A1..A13.

**Given** all G1..G12 gates PASS and `goreleaser release` succeeds.

**When** the GitHub Release publish step executes.

**Then**:
- A GitHub Release with title `v1.0.0` is created.
- The Release body contains the verbatim CHANGELOG `[1.0.0]` section content.
- All 12 archive files are attached.
- `SHA256SUMS` is attached.
- `*.intoto.jsonl` SLSA provenance is attached (REQ-REL-011).
- `*.sig` + `*.crt` cosign signature artifacts are attached (REQ-REL-012).
- SPDX SBOM files are attached.
- The tag is GPG-signed (per REQ-REL-008 release ceremony — OPERATIONAL/post-merge; not required at implementation time).
- `.moai/project/roadmap.md` PR is auto-created updating M9 status (REQ-REL-014, REQ-REL-016).
- Optional release-notification webhook fires (REQ-REL-013).

Maps to scenarios S8, S11 in spec.md §5.

---

### AC-009 — Cosign verify-blob succeeds for released archives

Covers: REQ-REL-012, NFR-REL-004

**Given** the published GitHub Release with `usearch_1.0.0_linux_amd64.tar.gz` + `.sig` + `.crt`.

**When** a fresh operator runs:
```
cosign verify-blob \
  --certificate usearch_1.0.0_linux_amd64.tar.gz.crt \
  --signature usearch_1.0.0_linux_amd64.tar.gz.sig \
  --certificate-identity-regexp "https://github.com/elymas/universal-search/.github/workflows/release.yml@.*" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  usearch_1.0.0_linux_amd64.tar.gz
```

**Then**:
- The verification succeeds (exit code `0`).
- The same command, copied verbatim from RELEASE.md or the README, succeeds for ANY of the 12 archives (reproducibility per NFR-REL-004).

Maps to scenario S9 in spec.md §5.

---

### AC-010 — SLSA verifier confirms provenance

Covers: REQ-REL-011, NFR-REL-004

**Given** the published GitHub Release with `multiple.intoto.jsonl` provenance.

**When** a fresh operator runs:
```
slsa-verifier verify-artifact \
  --provenance-path multiple.intoto.jsonl \
  --source-uri github.com/elymas/universal-search \
  --source-tag v1.0.0 \
  usearch_1.0.0_linux_amd64.tar.gz
```

**Then**:
- The verification succeeds (exit code `0`).
- SLSA L2 builder identity is confirmed.
- The provenance binds the artifact to the GitHub Actions workflow run + source repo + tag.

Maps to scenario S10 in spec.md §5.

---

### AC-011 — Cross-SPEC verification at G7 (DEPLOY-001 + chart parity)

Covers: REQ-REL-017

**Given** SPEC-DEPLOY-001 `build-images.yml` has published the real app images `ghcr.io/elymas/usearch-api:1.0.0` and `ghcr.io/elymas/usearch-mcp:1.0.0` (plus `ghcr.io/elymas/usearch-migrate:1.0.0`) with valid cosign signatures, AND `chart-release.yml` has published `oci://ghcr.io/elymas/charts/universal-search:1.0.0` with `appVersion: 1.0.0`. (Note: `universal-search` is the Helm **chart** name only — no `universal-search` app image is built, so G7 must not verify one.)

**When** the `release.yml` G7 step executes verification.

**Then**:
- G7 verification SUCCEEDS.
- If a required app image (`usearch-api` or `usearch-mcp`) is missing → G7 FAILS, release aborts.
- If the chart `appVersion` ≠ git tag → G7 FAILS, release aborts.
- G7 does NOT reference a `ghcr.io/elymas/universal-search` app image (a nonexistent artifact would otherwise hard-fail the gate).
- DEPLOY-001 and chart-release.yml's publish workflows must complete BEFORE REL-001's release.yml is triggered (dependency ordering enforced).

Maps to scenario S11 in spec.md §5.

---

### AC-012 — Dry-run mode does not publish

Covers: REQ-REL-015, NFR-REL-006

**Given** `release.yml` invoked via `workflow_dispatch` with input `dry_run: true`.

**When** the workflow executes.

**Then**:
- All G1..G12 gates run.
- `goreleaser build` runs and `dist/` archives are generated as workflow artifacts.
- The following steps are SKIPPED:
  - `cosign sign`.
  - SLSA provenance generation.
  - GitHub Release publish.
  - `.moai/project/roadmap.md` PR creation.
  - Notification webhook fires.
- The workflow status is `success` with a clear "dry-run mode" banner in the summary.

Maps to scenario S12 in spec.md §5.

---

### AC-013 — Pre-release tag (e.g., v1.0.0-rc1) handling

Covers: REQ-REL-018, NFR-REL-001, NFR-REL-003, NFR-REL-007

**Given** a git tag matching the pre-release pattern (e.g., `v1.0.0-rc1`) is pushed.

**When** `release.yml` evaluates the tag.

**Then**:
- The workflow runs but marks the GitHub Release as `prerelease: true`.
- Subsequent stable tag (`v1.0.0`) re-runs the full ceremony WITHOUT requiring tag rotation.
- The full ceremony completes within the runtime budget (NFR-REL-001).
- The tag itself is immutable once pushed (NFR-REL-003 — verified by `git verify-tag` against the GPG signature and by the absence of `git push --force` in the workflow).
- Maintainer time burden from tag push to released artifacts is ≤ the documented bound in NFR-REL-007.

---

## 2. Edge Cases

### EC-001 — ldflags injection silently fails; binary reports `0.1.0-dev`

**Given** a build pipeline where ldflags injection is misconfigured (e.g., wrong package path).

**When** the binary is built and reports `usearch v0.1.0-dev` (the default value).

**Then**:
- G12 verification step in `release.yml` automatically greps `usearch --version` output against `$GITHUB_REF_NAME` (the tag).
- The mismatch FAILS the release.
- The workflow aborts before any artifact is published.
- The maintainer is notified with the specific diff.

### EC-002 — CHANGELOG missing an implemented SPEC ID

**Given** a SPEC `SPEC-FOO-001` exists with `status: implemented` but is NOT cited in CHANGELOG `[1.0.0]`.

**When** the verification step counts SPEC IDs.

**Then**:
- The NFR-REL-005 automatic count check FAILS.
- The G1 gate FAILS.
- The maintainer is instructed to either add the SPEC ID to CHANGELOG or downgrade the SPEC status (with reasoning).

### EC-003 — Sigstore Rekor unavailable mid-release

**Given** Rekor transparency log is unreachable during the cosign sign step.

**When** `cosign sign-blob` is invoked.

**Then**:
- `cosign sign-blob` retries up to 3 times (release.yml `retry: 3`).
- If still failing after 3 retries, the workflow PAUSES and notifies the maintainer.
- The maintainer follows the RELEASE.md procedure: wait for Rekor recovery OR delay the release.
- No partial Release is published (atomicity preserved).

---

## 3. Definition of Done Checklist

- [ ] All 13 AC scenarios pass on CI.
- [ ] All 12 test scenarios from spec.md §5 (S1..S12) are implemented as automated tests.
- [ ] All 13 acceptance gates A1..A13 in spec.md §6 PASS.
- [ ] `internal/version/` package exports `Version`, `Commit`, `BuildDate`; consumed by all 3 binaries.
- [ ] `cmd/usearch/main.go` no longer contains a literal version string.
- [ ] CHANGELOG `[1.0.0]` section complete, cites every implemented SPEC.
- [ ] MIGRATION.md has exactly 12 sections per REQ-REL-004.
- [ ] RELEASE.md has all 5 sections (A..E) per REQ-REL-005.
- [ ] `.goreleaser.yml` validates with `goreleaser check`.
- [ ] `.github/workflows/release.yml` validates with `actionlint` (zero errors).
- [ ] `release.yml` declares all 12 pre-tag verification gates G1..G12.
- [ ] `release.yml` G7/G8/G9 reference DEPLOY-001/DOC-001/DOC-002 workflow run URLs.
- [ ] TRUST 5 — Tested: ≥ 85% coverage on `internal/version/` + release.yml shell scripts.
- [ ] TRUST 5 — Secured: gitleaks + Trivy on workflow YAML report zero findings.
- [ ] TRUST 5 — Trackable: every PR cites `SPEC-REL-001`.
- [ ] Cross-SPEC dependency verification: DEPLOY-001 + DOC-001 + DOC-002 + SEC-001 + EVAL trio all PASS at G6/G7/G8/G9.
- [ ] Dry-run mode tested by maintainer pre-tag.
- [ ] Tag immutability: `v1.0.0` is GPG-signed and protected from force-push.
- [ ] Open Questions OQ1..OQN in spec.md §8 are resolved or explicitly deferred with mitigation.

---

## 4. Coverage Matrix (REQ → AC)

| REQ / NFR | AC-001 | AC-002 | AC-003 | AC-004 | AC-005 | AC-006 | AC-007 | AC-008 | AC-009 | AC-010 | AC-011 | AC-012 | AC-013 | EC |
|-----------|--------|--------|--------|--------|--------|--------|--------|--------|--------|--------|--------|--------|--------|----|
| REQ-REL-001 | ✓ |   | ✓ |   |   |   |   |   |   |   |   |   |   |   |
| REQ-REL-002 | ✓ | ✓ |   |   |   |   |   |   |   |   |   |   |   |   |
| REQ-REL-003 |   |   |   | ✓ |   |   |   |   |   |   |   |   |   | EC-002 |
| REQ-REL-004 |   |   |   |   | ✓ |   |   |   |   |   |   |   |   |   |
| REQ-REL-005 |   |   |   |   |   |   |   |   |   |   |   |   |   |   |
| REQ-REL-006 |   |   |   |   |   | ✓ |   |   |   |   |   |   |   |   |
| REQ-REL-007 |   |   |   |   |   |   | ✓ | ✓ |   |   |   |   |   |   |
| REQ-REL-008 |   |   |   |   |   |   |   | ✓ |   |   |   |   |   |   |
| REQ-REL-009 |   |   |   |   |   |   | ✓ |   |   |   |   |   |   |   |
| REQ-REL-010 |   |   |   |   |   |   |   | ✓ |   |   |   |   |   |   |
| REQ-REL-011 |   |   |   |   |   |   |   | ✓ |   | ✓ |   |   |   |   |
| REQ-REL-012 |   |   |   |   |   |   |   | ✓ | ✓ |   |   |   |   | EC-003 |
| REQ-REL-013 |   |   |   |   |   |   |   | ✓ |   |   |   |   |   |   |
| REQ-REL-014 |   |   |   |   |   |   |   | ✓ |   |   |   |   |   |   |
| REQ-REL-015 |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |   |
| REQ-REL-016 |   |   |   |   |   |   |   | ✓ |   |   |   |   |   |   |
| REQ-REL-017 |   |   |   |   |   |   |   | ✓ |   |   | ✓ |   |   |   |
| REQ-REL-018 |   |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |
| NFR-REL-001 |   |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |
| NFR-REL-002 | ✓ |   |   |   |   |   |   |   |   |   |   |   |   |   |
| NFR-REL-003 |   |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |
| NFR-REL-004 |   |   |   |   |   |   |   |   | ✓ | ✓ |   |   |   |   |
| NFR-REL-005 |   |   |   | ✓ |   |   |   |   |   |   |   |   |   | EC-002 |
| NFR-REL-006 |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |   |
| NFR-REL-007 |   |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |

Note: REQ-REL-005 (RELEASE.md ceremony documentation) is verified by gate A8 (manual review) in the Definition of Done checklist rather than an isolated AC, since it is a documentation deliverable rather than a behavior. EC-001..EC-003 supplement ldflags failure detection, CHANGELOG drift, and Rekor outage.

---

*End of SPEC-REL-001 acceptance.md.*
