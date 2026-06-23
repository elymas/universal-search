---
id: SPEC-CICD-001
version: 0.1.0
status: implemented
created: 2026-06-23
updated: 2026-06-24
author: limbowl
priority: P2
issue_number: 0
title: CI infra hardening — Chart CI helm-repo registration for ct lint + hadolint Dockerfile lint compliance (DL3018/DL4006/SC2015)
milestone: operational — CI debt remediation
owner: expert-devops
methodology: ddd
coverage_target: 0
depends_on: []
blocks: []
related: [SPEC-DEPLOY-001, SPEC-DEP-001]
---

# SPEC-CICD-001: CI infra hardening — Chart CI helm-repo registration + hadolint Dockerfile lint

## HISTORY

- 2026-06-23 (initial draft v0.1.0, limbowl via manager-spec):
  Remediation SPEC for two independent, pre-existing CI-hardening gaps
  surfaced by investigation. This SPEC does NOT introduce new features — it
  closes existing CI debt so that the named CI jobs/checks pass on `main`.

  **Gap A — Chart CI `lint` job lacks `helm repo add` prelude.** The
  `.github/workflows/chart-ci.yml` `lint` job (L14–31) runs `ct lint` (and
  `helm lint --strict`) with NO `helm repo add` steps. `ct lint`
  (chart-testing-action) internally invokes `helm dependency build`, which
  validates `charts/universal-search/Chart.lock` against *registered* Helm
  repositories and aborts with "no repository definition" when
  `https://charts.bitnami.com/bitnami` (postgresql, redis) and
  `https://qdrant.github.io/qdrant-helm` (qdrant) are unregistered — even
  though the dependency `.tgz` files are vendored, `ct` does not trust them
  without repo definitions. Among the sibling jobs, `template` and
  `kubeconform` carry a repo-add block; `lint` AND `kind` do not. This SPEC
  fixes only `lint` — the `kind` gap is masked today by `continue-on-error:
  true` on its `helm dependency update` step (note as a known edge,
  optionally track in SPEC-DEPLOY-001).

  **Gap B — Dockerfiles predate the hadolint pre-commit hook.** The
  hadolint hook (`.pre-commit-config.yaml` L90–93, no config/ignore args)
  runs in CI via `pre-commit run --all-files` (`.github/workflows/
  pre-commit.yml` L48–59), making all hadolint findings CI-blocking. With
  NO `.hadolint.yaml` suppression file present (verified absent), every
  default rule fires. Confirmed findings:
  - `deploy/Dockerfile.usearch-api:11` — DL3018 (`apk add --no-cache git
    ca-certificates`, unpinned)
  - `deploy/Dockerfile.usearch-mcp:9` — DL3018 (same unpinned apk add)
  - `deploy/Dockerfile.usearch-migrate:11–12` — DL3018 (unpinned `apk add
    curl`) AND DL4006 (`curl ... | tar xvz` pipe without
    `SHELL [..., "-o", "pipefail", ...]`)
  - `Dockerfile.docs:38` — SC2015 (`A && B || C` build-verify construct
    where C may run even when A succeeds)

  **Verified path corrections (blockers carried forward):**
  - `Dockerfile.docs` is at the repository ROOT (`./Dockerfile.docs`), NOT
    under `deploy/`. The remediation MUST target the root path.
  - The `lint`-job fix MUST use the classic HTTPS bitnami index
    `https://charts.bitnami.com/bitnami` (matching `Chart.lock` L3/L6 +
    `Chart.yaml`), NOT the OCI URL `oci://registry-1.docker.io/
    bitnamicharts` that the existing `template` job (L49) uses, so
    `helm dependency build` does not invalidate the `Chart.lock` digest
    (`sha256:cfc60cd8...`).

  **Needs-decision item (flagged, not pre-resolved):** DL3018 admits two
  remediation styles — (a) hard version pins (`git=2.45.2-r0 ...`, brittle
  across Alpine base-image patch bumps, recurring maintenance) vs (b)
  inline `# hadolint ignore=DL3018` with written justification (recommended
  for `--no-cache` ephemeral builder/downloader stages). This SPEC requires
  the *outcome* (zero findings) and leaves the per-line style choice to the
  run phase, defaulting to (b) for ephemeral stages. See §4 and Open
  Questions.

  **Already-fixed / out-of-scope:** the pre-commit-autoupdate PR-permission
  was resolved by the user via a repo-level GitHub Actions setting (not an
  in-repo config); no requirement is needed.

  6 EARS REQs (all P2). Methodology: DDD (the surface is existing CI config
  + Dockerfiles; characterize current CI behavior, then improve). Coverage
  target N/A (CI-config + Dockerfile changes, no Go unit coverage). Owner:
  expert-devops.

---

## 1. Overview

SPEC-CICD-001 remediates two independent, pre-existing CI-hardening gaps.
It is a **CI-debt cleanup**, not a new capability: no application code
changes, no new dependency. The headline acceptance is that the named CI
jobs/checks pass on `main`.

### 1.1 What ships

| Layer | Artifact | Change |
|-------|----------|--------|
| CI | `.github/workflows/chart-ci.yml` (modify) | Add `helm repo add bitnami https://charts.bitnami.com/bitnami` + `helm repo add qdrant https://qdrant.github.io/qdrant-helm` to the `lint` job before the `ct lint` step (L27) |
| Dockerfile | `deploy/Dockerfile.usearch-api` (modify) | Resolve DL3018 on L11 (`apk add` git/ca-certificates) |
| Dockerfile | `deploy/Dockerfile.usearch-mcp` (modify) | Resolve DL3018 on L9 (`apk add` git/ca-certificates) |
| Dockerfile | `deploy/Dockerfile.usearch-migrate` (modify) | Resolve DL3018 (L11 `apk add curl`) AND DL4006 (L12 `curl \| tar` pipe) |
| Dockerfile | `Dockerfile.docs` (modify, repo ROOT) | Resolve SC2015 on L38 (build-verify `A && B \|\| C`) |
| Config (optional) | `.hadolint.yaml` (new, optional) | Centralize any ignores if the run phase prefers a single config over inline comments |

### 1.2 Motivation

Both gaps are silent CI breakers/blockers that should be closed before the
chart and container images are relied upon in a release flow:

- The Chart CI `lint` job cannot resolve `Chart.lock` dependencies, so it
  fails (or is effectively non-validating) for any PR touching the chart —
  blocking confident chart changes.
- Every PR touching the listed Dockerfiles triggers blocking hadolint
  findings (DL3018/DL4006/SC2015) via `pre-commit run --all-files`,
  producing red checks that mask genuine review signal.

### 1.3 Scope boundary (what this is NOT)

This SPEC does not redesign the CI pipeline, add new scanners, change
Dockerfile base images, or alter chart dependencies. It registers the
missing Helm repos in one job and makes four Dockerfiles hadolint-clean.

---

## 2. Scope

### 2.1 In scope

- `.github/workflows/chart-ci.yml` `lint` job: register Bitnami + Qdrant
  Helm repos before `ct lint`, using the Chart.lock-consistent HTTPS URLs.
- DL3018 remediation on `deploy/Dockerfile.usearch-api:11`,
  `deploy/Dockerfile.usearch-mcp:9`, `deploy/Dockerfile.usearch-migrate:11`.
- DL4006 remediation on `deploy/Dockerfile.usearch-migrate:12` (piped
  `curl | tar`).
- SC2015 remediation on `Dockerfile.docs:38` (repo root).
- Optional `.hadolint.yaml` to centralize justified ignores.

### 2.2 Out of scope

- pre-commit-autoupdate PR-permission (already resolved by the user via a
  repo-level GitHub Actions setting; not an in-repo config).
- The `template`/`kubeconform` jobs' existing repo-add blocks (`lint` and
  `kind` lack one; this SPEC only adds the missing one to `lint`). The
  `kind` gap is masked today by `continue-on-error: true` on its `helm
  dependency update` step — a known edge, optionally tracked in
  SPEC-DEPLOY-001, not fixed here. The OCI vs HTTPS URL inconsistency
  between the repo-add-carrying jobs and Chart.lock is noted as a blocker
  but full URL alignment across all jobs is NOT required — only the `lint`
  job must use the Chart.lock-consistent URL (see Open Questions Q2).
- Any other hadolint findings beyond DL3018/DL4006/SC2015 on files other
  than the four named Dockerfiles.
- Dockerfile base-image upgrades, multi-stage restructuring, or content
  changes unrelated to the cited lint rules.
- Application/Go code, chart templates, and chart dependency versions.

---

## 3. EARS Requirements

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-CICD-010** | Optional-feature (WHERE) | WHERE the Chart CI `lint` job runs `ct lint` (`.github/workflows/chart-ci.yml`), the workflow SHALL register the Bitnami (`https://charts.bitnami.com/bitnami`) and Qdrant (`https://qdrant.github.io/qdrant-helm`) Helm repositories via `helm repo add` steps placed before the `ct lint` step (current L27), so that the `helm dependency build` invoked internally by `ct lint` can resolve all three `Chart.lock` dependencies (postgresql, redis, qdrant). | P2 | `chart-ci.yml` `lint` job contains `helm repo add bitnami https://charts.bitnami.com/bitnami` and `helm repo add qdrant https://qdrant.github.io/qdrant-helm` before `ct lint`; the Chart CI `lint` job passes on `main` with no "no repository definition" error. |
| **REQ-CICD-020** | Ubiquitous | The Chart CI workflow SHALL, in the `lint` job, use Helm repository URLs that are consistent with the URLs pinned in `charts/universal-search/Chart.lock` (Bitnami via `https://charts.bitnami.com/bitnami`, Qdrant via `https://qdrant.github.io/qdrant-helm`) so that `helm dependency build` does not invalidate the `Chart.lock` digest (`sha256:cfc60cd8...`). The `lint` job SHALL NOT introduce the OCI Bitnami URL (`oci://registry-1.docker.io/bitnamicharts`) for dependency resolution against this lock file. | P2 | Grep of the `lint` job confirms the HTTPS bitnami URL and absence of the OCI URL; `ct lint` completes without a digest-mismatch / lock-out-of-date error. |
| **REQ-CICD-030** | Optional-feature (WHERE) | WHERE a Dockerfile RUN instruction installs packages via `apk add` (`deploy/Dockerfile.usearch-api:11`, `deploy/Dockerfile.usearch-mcp:9`, `deploy/Dockerfile.usearch-migrate:11`), the system SHALL resolve hadolint DL3018 by EITHER pinning each package to an explicit version (`pkg=version`) OR carrying an inline `# hadolint ignore=DL3018` comment with a one-line written justification immediately above the RUN instruction. | P2 | hadolint reports zero DL3018 findings for the three named files when run via `pre-commit run --all-files`; each remediated line is either version-pinned or accompanied by a justified ignore comment. |
| **REQ-CICD-040** | Optional-feature (WHERE) | WHERE a Dockerfile RUN instruction contains a piped command (`\|`) (`deploy/Dockerfile.usearch-migrate:12`, `curl ... \| tar xvz`), the system SHALL set a `SHELL` instruction with `-o pipefail` (e.g. `SHELL ["/bin/ash", "-o", "pipefail", "-c"]` for the Alpine downloader stage) before that RUN instruction, satisfying hadolint DL4006. | P2 | hadolint reports zero DL4006 findings for `deploy/Dockerfile.usearch-migrate`; the downloader stage declares a `SHELL` with `-o pipefail` before the piped `curl \| tar` RUN. |
| **REQ-CICD-050** | Optional-feature (WHERE) | WHERE the `Dockerfile.docs` build-verification step (repository ROOT path `./Dockerfile.docs:38`) checks for the static export output, the system SHALL express its success/failure branches WITHOUT the `A && B \|\| C` shorthand (hadolint SC2015), using either an explicit `RUN if test -f ./out/index.html; then ...; else ...; exit 1; fi` conditional OR an inline `# hadolint ignore=SC2015` with a one-line justification. | P2 | hadolint reports zero SC2015 findings for `./Dockerfile.docs`; the build-verify line is rewritten as an explicit conditional or carries a justified ignore. |
| **REQ-CICD-060** | Event-Driven (WHEN) | WHEN `pre-commit run --all-files` executes the hadolint hook in CI (`.github/workflows/pre-commit.yml`), the run SHALL produce zero DL3018, DL4006, and SC2015 findings across `deploy/Dockerfile.usearch-api`, `deploy/Dockerfile.usearch-mcp`, `deploy/Dockerfile.usearch-migrate`, and `Dockerfile.docs`. | P2 | The pre-commit CI check passes on `main`; a local `pre-commit run hadolint --all-files` (or hadolint over the four files) reports zero DL3018/DL4006/SC2015 findings. |

---

## 4. Findings Disposition (false-positive / needs-decision flags)

Per investigation, each confirmed finding is real and CI-blocking; none are
false positives. One item is **already-fixed** and one carries a
**needs-decision** flag for the run phase:

| Finding | Status | Disposition |
|---------|--------|-------------|
| chart-ci `lint` missing `helm repo add` | confirmed | Fix — REQ-CICD-010/020 |
| DL3018 (api:11, mcp:9, migrate:11) | confirmed | **NEEDS-DECISION**: pin versions (brittle across Alpine bumps) vs justified `# hadolint ignore=DL3018` (recommended for `--no-cache` ephemeral stages). REQ-CICD-030 permits either; outcome = zero findings. |
| DL4006 (migrate:12 piped curl) | confirmed | Fix via `SHELL [..., "-o", "pipefail", ...]` — REQ-CICD-040 |
| SC2015 (Dockerfile.docs:38) | confirmed | Fix via explicit `if` block or justified ignore — REQ-CICD-050 |
| hadolint runs in CI, no `.hadolint.yaml` | confirmed | Context: makes findings CI-blocking; optional centralized config in §1.1 |
| pre-commit-autoupdate PR-permission | **already-fixed** | OUT OF SCOPE — resolved via repo-level GitHub Actions setting; no requirement. |

No finding is a false positive. The only path correction is that
`Dockerfile.docs` lives at the repository ROOT, not under `deploy/`.

---

## 5. Acceptance Criteria

The headline acceptance for this CI-debt SPEC is: **the named CI jobs/checks
pass on `main`.**

| Scenario | Given-When-Then | Coverage |
|----------|-----------------|----------|
| AC-1 | GIVEN a PR touching `charts/universal-search/**` WHEN the Chart CI `lint` job runs `ct lint` THEN the job completes with no "no repository definition" error and passes on `main`. | REQ-CICD-010 |
| AC-2 | GIVEN the `lint` job's repo-add steps WHEN `helm dependency build` resolves `Chart.lock` THEN the digest is NOT invalidated (no lock-out-of-date error) because the HTTPS bitnami URL matches Chart.lock; the OCI URL is absent from the `lint` job. | REQ-CICD-020 |
| AC-3 | GIVEN `deploy/Dockerfile.usearch-api`, `deploy/Dockerfile.usearch-mcp`, `deploy/Dockerfile.usearch-migrate` WHEN hadolint runs THEN it reports zero DL3018 findings (each `apk add` line pinned or carries a justified ignore). | REQ-CICD-030 |
| AC-4 | GIVEN `deploy/Dockerfile.usearch-migrate` WHEN hadolint runs THEN it reports zero DL4006 findings (downloader stage declares `SHELL` with `-o pipefail` before the piped `curl \| tar` RUN). | REQ-CICD-040 |
| AC-5 | GIVEN `./Dockerfile.docs` (repo root) WHEN hadolint runs THEN it reports zero SC2015 findings (build-verify rewritten as explicit conditional or justified ignore). | REQ-CICD-050 |
| AC-6 | GIVEN all four named Dockerfiles WHEN `pre-commit run --all-files` executes the hadolint hook in CI THEN the pre-commit check passes on `main` with zero DL3018/DL4006/SC2015 findings. | REQ-CICD-060 |

### 5.1 Definition of Done

- Chart CI `lint` job passes on `main` (AC-1, AC-2).
- pre-commit (hadolint) check passes on `main` (AC-3–AC-6).
- No new hadolint findings introduced on files outside the four named
  Dockerfiles.
- The needs-decision DL3018 style choice is recorded (per-line) in the run
  phase.

---

## 6. Dependencies & Blocks

### 6.1 Upstream dependencies (depends_on)

None. This SPEC operates on already-present CI config and Dockerfiles.

### 6.2 Related (soft)

- **SPEC-DEPLOY-001** — owns the chart + the three Go Dockerfiles
  (usearch-api/mcp/migrate). This SPEC hardens their CI/lint posture
  without changing their build content. Related, not a hard dependency.
- **SPEC-DEP-001** — owns the dependency-audit / pre-commit CI baseline
  (the hadolint hook lives in that lineage). This SPEC makes existing files
  conform to that baseline. Related, not a hard dependency.

### 6.3 Downstream blocked (blocks)

None.

### 6.4 Blockers / cautions carried into run phase

1. **Path correction (HARD):** `Dockerfile.docs` is at the repository ROOT
   (`./Dockerfile.docs`), NOT under `deploy/`. Targeting `deploy/
   Dockerfile.docs` would edit a nonexistent file.
2. **URL consistency (HARD):** the `lint`-job repo-add MUST use
   `https://charts.bitnami.com/bitnami` (Chart.lock-consistent), NOT the
   OCI URL the `template` job uses, or `helm dependency build` invalidates
   the Chart.lock digest.
3. **DL3018 style (needs-decision):** hard version pins break on Alpine
   patch bumps; justified `# hadolint ignore=DL3018` is recommended for
   `--no-cache` ephemeral builder/downloader stages. Decision is per-line,
   made in the run phase.

### 6.5 External dependencies

None.

---

## 7. Files to Create / Modify

### 7.1 Modified

| Path | Change |
|------|--------|
| `.github/workflows/chart-ci.yml` | Add `helm repo add bitnami https://charts.bitnami.com/bitnami` + `helm repo add qdrant https://qdrant.github.io/qdrant-helm` step to the `lint` job before `ct lint` (L27) — REQ-CICD-010/020 |
| `deploy/Dockerfile.usearch-api` | L11 `apk add` DL3018 remediation — REQ-CICD-030 |
| `deploy/Dockerfile.usearch-mcp` | L9 `apk add` DL3018 remediation — REQ-CICD-030 |
| `deploy/Dockerfile.usearch-migrate` | L11 `apk add curl` DL3018 + L12 piped `curl \| tar` DL4006 (`SHELL ... -o pipefail`) — REQ-CICD-030/040 |
| `Dockerfile.docs` (repo ROOT) | L38 build-verify SC2015 remediation — REQ-CICD-050 |

### 7.2 Created (optional)

| Marker | Path | Purpose |
|--------|------|---------|
| [NEW, optional] | `.hadolint.yaml` | Centralize justified ignores if the run phase prefers a single config over inline comments (none exists today) |

### 7.3 Existing — Unchanged

- `.github/workflows/chart-ci.yml` `template`/`kubeconform` jobs —
  their existing repo-add blocks are left as-is. `lint` and `kind` both
  lack one; this SPEC adds it only to `lint`. The `kind` gap is masked
  today by `continue-on-error: true` on its `helm dependency update` step
  (a known edge, optionally tracked in SPEC-DEPLOY-001; not fixed here).
  Full OCI-vs-HTTPS URL alignment across jobs is out of scope.
- `.pre-commit-config.yaml` hadolint hook (L90–93) — unchanged; this SPEC
  makes files conform to it, not the other way around.
- `.github/workflows/pre-commit.yml` — unchanged; it already runs
  `pre-commit run --all-files`.
- All application/Go code, chart templates, and chart dependency versions.

---

## 8. Open Questions

1. **DL3018 remediation style (needs-decision)** — per-line choice between
   hard version pins vs justified `# hadolint ignore=DL3018` for the three
   `apk add` lines. Default recommendation: ignore-with-justification for
   `--no-cache` ephemeral builder/downloader stages. Resolved in run phase.

2. **chart-ci URL inconsistency** — the `template` job uses the OCI Bitnami
   URL while Chart.lock/Chart.yaml use HTTPS. This SPEC only requires the
   `lint` job to use the Chart.lock-consistent HTTPS URL. Whether to also
   align the `template`/`kubeconform`/`kind` jobs to HTTPS for consistency
   is deferred (out of scope here; track in SPEC-DEPLOY-001 if desired).

3. **`.hadolint.yaml` vs inline comments** — whether to centralize ignores
   in a new `.hadolint.yaml` or keep them inline. Either satisfies the
   "zero findings" outcome; run-phase preference.

These do not block plan-auditor PASS — they are known scope edges tagged
with rationale.

---

## 9. References

Internal (verified during planning):

- `.github/workflows/chart-ci.yml` (L14–31 `lint` job; L47–51 `template`
  job repo-add block using OCI bitnami URL)
- `charts/universal-search/Chart.lock` (L3/L6 bitnami HTTPS, L9 qdrant
  HTTPS, digest `sha256:cfc60cd8...`)
- `charts/universal-search/Chart.yaml` (dependency declarations)
- `deploy/Dockerfile.usearch-api` (L11 `apk add` DL3018)
- `deploy/Dockerfile.usearch-mcp` (L9 `apk add` DL3018)
- `deploy/Dockerfile.usearch-migrate` (L11 `apk add curl` DL3018; L12
  `curl \| tar` DL4006)
- `Dockerfile.docs` (repo ROOT; L38 build-verify SC2015)
- `.pre-commit-config.yaml` (L90–93 hadolint hook, no args / no config)
- `.github/workflows/pre-commit.yml` (L48–59, `pre-commit run --all-files`)

External:

- hadolint rules: https://github.com/hadolint/hadolint#rules
- DL3018 (pin apk versions), DL4006 (pipefail on piped RUN), SC2015
  (ShellCheck: `A && B || C` is not if-then-else)
- chart-testing (`ct`): https://github.com/helm/chart-testing
- Helm dependency build: https://helm.sh/docs/helm/helm_dependency_build/

---

*End of SPEC-CICD-001 v0.1.0 (draft).*
