---
id: SPEC-DEP-001
title: Dependency Pinning Policy and Audit CI
version: 0.1.0
milestone: M1 — Foundation
status: implemented
implemented_at: 2026-04-25
priority: P0
owner: expert-devops
methodology: tdd
coverage_target: 85
created: 2026-04-24
updated: 2026-05-04
depends_on: [SPEC-BOOT-001]
blocks: [SPEC-OBS-001, SPEC-LLM-001, SPEC-IR-001]
---

# SPEC-DEP-001: Dependency Pinning Policy and Audit CI

## 1. Purpose

SPEC-BOOT-001 established the initial dependency pinning baseline (compose image tags, `uv.lock`, `pnpm-lock.yaml`, `go.sum`, `LICENSE`/`NOTICE`). SPEC-DEP-001 layers the **policy, automation, and enforcement** on top: a Go dependency policy for future growth, scheduled audit and autoupdate CI, license compliance scanning, a dated SearXNG digest pin replacing `:latest`, Renovate-driven upgrade PRs, and an auto-generated `docs/dependencies.md` manifest. This SPEC hardens the supply chain without re-pinning what SPEC-BOOT-001 already pinned.

## 2. Scope

### 2.1 In-Scope

| Ref | Item |
|-----|------|
| a | Go dependency pinning policy + future-dep placeholder list |
| b | Pre-commit autoupdate CI job (weekly cron, opens PR) |
| c | Dependency audit CI (govulncheck, pip-audit, pnpm audit, hadolint) |
| d | License scan (go-licenses, pip-licenses, license-checker) + allowlist enforcement |
| e | SearXNG dated digest pin in `deploy/docker-compose.yml` |
| f | Renovate bot config (`renovate.json`) |
| g | `docs/dependencies.md` manifest + generation script |

### 2.2 Out-of-Scope

- Re-pinning what SPEC-BOOT-001 already pinned (compose images, `uv.lock`, `pnpm-lock.yaml`, `go.sum`, `LICENSE`)
- Full security audit and threat modeling (belongs to SPEC-SEC-001)
- Private registry / artifact proxy / SBOM signing (future SPEC)
- Runtime vulnerability scanning of deployed containers (belongs to SPEC-OBS-001 or SPEC-SEC-001)
- Auto-merging Renovate PRs (explicit open question; default = PR only)

## 3. EARS Requirements

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| REQ-DEP-001 | Ubiquitous | The repository SHALL document a Go dependency pinning policy covering direct pins (exact version), transitive pins (via `go.sum`), minor-range exceptions, and security-update handling. A placeholder list of expected future dependencies MUST map each to its planned consumer SPEC. | P0 | `docs/dependencies.md` contains a "Go Policy" section; the future-dep list includes chi→SPEC-IR-001, prometheus client_golang→SPEC-OBS-001, Asynq→SPEC-LLM-001 at minimum. |
| REQ-DEP-002 | Event-Driven | WHEN the weekly cron fires (Mondays 06:00 UTC), the pre-commit autoupdate CI job SHALL run `pre-commit autoupdate`, and IF any hook rev changes THEN it SHALL open a PR titled `chore(deps): pre-commit autoupdate` tagged `dependencies`. | P1 | `.github/workflows/pre-commit-autoupdate.yml` exists; syntax-valid; scheduled cron matches spec; PR flow verified by dry-run. |
| REQ-DEP-003 | Event-Driven | WHEN a PR is opened or updated, the dependency-audit CI SHALL run `govulncheck`, `pip-audit`, `pnpm audit`, and `hadolint`; findings SHALL be posted as PR comments; and IF any HIGH severity issue is detected THEN the PR status check SHALL fail. | P0 | `.github/workflows/deps-audit.yml` exists with all four tool invocations; blocking logic on HIGH severity; weekly cron also schedules a main-branch audit. |
| REQ-DEP-004 | Event-Driven / Unwanted | WHEN a PR is opened, the license scan CI SHALL generate `docs/licenses/{go,python,web}.txt` via go-licenses, pip-licenses, and license-checker; and the PR SHALL NOT pass status checks IF any direct dependency has a license outside the allowlist (MIT, Apache-2.0, BSD-*, ISC, PostgreSQL, MPL-2.0). SearXNG AGPL is pre-approved under the service-boundary exception. | P0 | Allowlist table present in SPEC; `scripts/check-license-allowlist.sh` exits non-zero on unapproved license; SearXNG exception documented. |
| REQ-DEP-005 | Unwanted | The `deploy/docker-compose.yml` file SHALL NOT reference `searxng/searxng:latest`; the SearXNG image SHALL be pinned to a dated tag or `@sha256:` digest recorded at SPEC merge time, with manual bumps gated by a future SPEC-DEP-NNN. | P0 | Regex `searxng/searxng(@sha256:\|:\d{4}\.\d{2}\.\d{2})` matches the compose file; `:latest` absent; digest-pinning policy documented in `docs/dependencies.md`. |
| REQ-DEP-006 | Event-Driven | WHEN Renovate bot scans the repository (scheduled off-hours), it SHALL group major-version upgrades into separate PRs, group minor/patch upgrades into a single weekly PR, cap concurrent open PRs at 5, exclude `.moai/` and docker image digests, and assign PRs to the current branch owner. | P1 | `renovate.json` validates against the Renovate schema; group rules and ignorePaths match the SPEC. |
| REQ-DEP-007 | Ubiquitous | The repository SHALL maintain `docs/dependencies.md` listing every direct dependency (Go, Python per-service, web, compose services) with name, pinned version, license, upstream repo URL, rationale linked to tech.md, and next-SPEC consumer. A script `scripts/gen-deps-manifest.sh` SHALL produce the machine-generated portion. | P0 | `docs/dependencies.md` exists and is non-empty; `scripts/gen-deps-manifest.sh` is idempotent and runs under 30 s locally; CI verifies the manifest is in sync with lock files. |

## 4. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| NFR-DEP-001 | Reproducibility | The audit CI MUST be deterministic: pinned tool versions (govulncheck, pip-audit, hadolint, go-licenses, etc.) via GitHub Actions `uses:` with `@vN.N.N` refs; no floating `@main` or `@latest`. Local reproduction via `act` SHOULD produce identical output. |
| NFR-DEP-002 | Upgrade Velocity | Renovate cadence SHALL be weekly for minor/patch (grouped, off-hours) and on-demand for major (separate PRs). Pre-commit autoupdate SHALL run weekly. Maximum 5 open Renovate PRs at any time. |
| NFR-DEP-003 | License Compliance | License enforcement MUST be automated (no manual review step); allowlist violations MUST block merge; license data MUST be regenerated on every PR touching dependency manifests. |

## 5. Acceptance Criteria

### REQ-DEP-001 — Go Policy
- `docs/dependencies.md` §"Go Policy" enumerates: direct pin strategy, transitive pin strategy, minor-range criteria, security update SLA (HIGH: 7 days, MEDIUM: 30 days, LOW: next release cycle).
- Future-dep placeholder list contains at minimum: `github.com/go-chi/chi/v5` (SPEC-IR-001), `github.com/prometheus/client_golang` (SPEC-OBS-001), `github.com/hibiken/asynq` (SPEC-LLM-001).
- A test verifies the section is non-empty and includes the three named packages.

### REQ-DEP-002 — Pre-commit Autoupdate
- `.github/workflows/pre-commit-autoupdate.yml` exists, passes `actions/workflow-syntax-check`.
- Cron: `0 6 * * 1` (Mon 06:00 UTC).
- Steps: checkout → setup-python → install pre-commit (pinned) → `pre-commit autoupdate` → detect diff → create PR via `peter-evans/create-pull-request@vN` (pinned).
- PR labels include `dependencies`, `automated`.

### REQ-DEP-003 — Dependency Audit
- `.github/workflows/deps-audit.yml` exists, passes syntax validation.
- Triggers: `pull_request` + `schedule: 0 4 * * 1`.
- Jobs:
  - `govulncheck-go`: `go run golang.org/x/vuln/cmd/govulncheck@vN ./...`
  - `pip-audit-python`: per-service `uv run pip-audit` (services with `pyproject.toml`)
  - `pnpm-audit-web`: `pnpm audit --audit-level=high` in `apps/web/`
  - `hadolint-docker`: `hadolint/hadolint-action@vN` on all Dockerfiles under `services/` and `deploy/`
- HIGH severity in any job fails the check; MEDIUM/LOW are posted as PR comments but non-blocking.

### REQ-DEP-004 — License Scan
- License scan job in `.github/workflows/deps-audit.yml` generates and commits (on main-branch scheduled runs) `docs/licenses/{go,python,web}.txt`.
- `scripts/check-license-allowlist.sh` reads generated `.txt` files, fails on any license not in the allowlist (Table 5.1 below).
- SearXNG AGPL is excluded via service-boundary comment in the script (SearXNG runs as a separate container, not linked).

#### Table 5.1 — License Allowlist

| License | Status | Rationale |
|---------|--------|-----------|
| MIT | Approved | Minimal restrictions, compatible with Apache-2.0 redistribution |
| Apache-2.0 | Approved | Project license; self-compatible |
| BSD-2-Clause, BSD-3-Clause | Approved | Permissive, compatible |
| ISC | Approved | Permissive, MIT-equivalent |
| PostgreSQL | Approved | Permissive, BSD-equivalent |
| MPL-2.0 | Approved | File-level copyleft only; acceptable for library use |
| LGPL-* | Review-required | Dynamic linking OK; static linking requires legal review |
| GPL-*, AGPL-*, SSPL | Blocked | Incompatible with Apache-2.0 redistribution without exception |
| Proprietary, UNKNOWN | Blocked | Requires explicit per-dependency exception |

### REQ-DEP-005 — SearXNG Digest Pin
- `deploy/docker-compose.yml` SearXNG service image line matches `^\s*image:\s*searxng/searxng(@sha256:[a-f0-9]{64}|:\d{4}\.\d{2}\.\d{2}-[a-f0-9]+)$`.
- `docs/dependencies.md` records the pinned digest and the date/source of discovery.
- A CI test (regex check) in `deps-audit.yml` fails if `:latest` is reintroduced.

### REQ-DEP-006 — Renovate Config
- `renovate.json` at repo root validates against the Renovate JSON schema.
- Configuration enforces:
  - `extends: ["config:recommended"]`
  - `schedule: ["after 10pm every weekday", "before 5am every weekday", "every weekend"]`
  - `prConcurrentLimit: 5`
  - `packageRules`: major → separate PR with `separateMajorMinor: true`; minor/patch → `groupName: "minor-and-patch-deps"`, `schedule: ["before 5am on monday"]`
  - `ignorePaths: [".moai/**"]`
  - `docker.enabled: false` (image digests are manual per REQ-DEP-005)
  - `assignees: ["<branch-owner-github-handle>"]` (placeholder resolved at merge time)

### REQ-DEP-007 — docs/dependencies.md Manifest
- Manifest contains:
  - **Go** section: output of `go list -m all` with columns: module, version, license, repo URL, rationale.
  - **Python** section: per-service output of `uv pip freeze` (services/orchestrator, services/research-agent, etc.).
  - **Web** section: `pnpm list --depth=0` from `apps/web/`.
  - **Compose Services** section: hardcoded table (Qdrant, Meilisearch, PostgreSQL, SearXNG, LiteLLM, Redis) with pinned tags from SPEC-BOOT-001 and digest for SearXNG from REQ-DEP-005.
- `scripts/gen-deps-manifest.sh` is idempotent (re-running produces no diff on clean tree).
- CI job verifies `git diff --exit-code docs/dependencies.md` after running the script (manifest drift detection).

## 6. Technical Approach

### 6.1 Go Dependency Policy (REQ-DEP-001)

Policy statement in `docs/dependencies.md`:

> **Go Dependency Pinning Policy**
>
> - **Direct dependencies**: Pinned to exact minor version in `go.mod` (e.g., `v5.1.0` not `v5`). Upgrades via Renovate PR.
> - **Transitive dependencies**: Locked via `go.sum` checksums. `go mod tidy` enforced in pre-commit.
> - **Minor-range exception**: Standard library modules (`golang.org/x/*`) MAY use latest patch within the minor line; Renovate weekly bump.
> - **Security updates**: HIGH severity → patch within 7 days (emergency SPEC or hotfix PR). MEDIUM → next weekly Renovate run. LOW → next scheduled dependency review.
> - **New dependencies**: Require SPEC reference in the PR description and an entry in the Future-Dependencies Placeholder table below.

Future-Dependencies Placeholder table:

| Package | Planned Consumer SPEC | Purpose |
|---------|----------------------|---------|
| `github.com/go-chi/chi/v5` | SPEC-IR-001 | HTTP router for Information Retrieval service |
| `github.com/prometheus/client_golang` | SPEC-OBS-001 | Prometheus metrics instrumentation |
| `github.com/hibiken/asynq` | SPEC-LLM-001 | Redis-backed task queue for LLM orchestration |
| `github.com/jackc/pgx/v5` | SPEC-DB-001 (tentative) | PostgreSQL driver |
| `github.com/qdrant/go-client` | SPEC-VECTOR-001 (tentative) | Qdrant client |

### 6.2 Pre-commit Autoupdate CI Skeleton (REQ-DEP-002)

```yaml
# .github/workflows/pre-commit-autoupdate.yml
name: pre-commit autoupdate
on:
  schedule:
    - cron: "0 6 * * 1"   # Monday 06:00 UTC
  workflow_dispatch:
jobs:
  autoupdate:
    runs-on: ubuntu-24.04
    permissions:
      contents: write
      pull-requests: write
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-python@v5
        with: { python-version: "3.12" }
      - run: pip install pre-commit==<pinned>
      - run: pre-commit autoupdate
      - uses: peter-evans/create-pull-request@v6
        with:
          title: "chore(deps): pre-commit autoupdate"
          branch: automated/pre-commit-autoupdate
          labels: dependencies,automated
          body: "Automated weekly pre-commit autoupdate."
```

### 6.3 Dependency Audit CI Skeleton (REQ-DEP-003, REQ-DEP-004)

```yaml
# .github/workflows/deps-audit.yml
name: deps-audit
on:
  pull_request:
  schedule:
    - cron: "0 4 * * 1"   # Monday 04:00 UTC
jobs:
  govulncheck:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version-file: "go.mod" }
      - run: go install golang.org/x/vuln/cmd/govulncheck@<pinned>
      - run: govulncheck ./...

  pip-audit:
    runs-on: ubuntu-24.04
    strategy:
      matrix:
        service: [orchestrator, research-agent, storm-writer]  # enumerated
    steps:
      - uses: actions/checkout@v4
      - uses: astral-sh/setup-uv@v3
      - run: uv run --directory services/${{ matrix.service }} pip-audit

  pnpm-audit:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v4
      - uses: pnpm/action-setup@v4
      - run: pnpm --dir apps/web audit --audit-level=high

  hadolint:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v4
      - uses: hadolint/hadolint-action@v3.1.0
        with:
          recursive: true
          dockerfile: "**/Dockerfile"

  license-scan:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v4
      - name: Go licenses
        run: |
          go install github.com/google/go-licenses@<pinned>
          go-licenses report ./... > docs/licenses/go.txt
      - name: Python licenses
        run: |
          pip install pip-licenses==<pinned>
          pip-licenses --format=plain > docs/licenses/python.txt
      - name: Web licenses
        run: |
          npm install -g license-checker@<pinned>
          license-checker --start apps/web --out docs/licenses/web.txt
      - run: bash scripts/check-license-allowlist.sh

  searxng-digest-check:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v4
      - run: |
          grep -E '^\s*image:\s*searxng/searxng' deploy/docker-compose.yml \
            | grep -Eq '(@sha256:[a-f0-9]{64}|:[0-9]{4}\.[0-9]{2}\.[0-9]{2})' \
            || { echo "SearXNG must be digest-pinned"; exit 1; }
```

### 6.4 License Allowlist Script Outline (REQ-DEP-004)

```bash
# scripts/check-license-allowlist.sh
#!/usr/bin/env bash
set -euo pipefail
ALLOWLIST="MIT|Apache-2.0|BSD-2-Clause|BSD-3-Clause|ISC|PostgreSQL|MPL-2.0"
EXCEPTIONS="searxng/searxng"   # service-boundary pre-approved AGPL

for f in docs/licenses/go.txt docs/licenses/python.txt docs/licenses/web.txt; do
  awk -v allow="$ALLOWLIST" -v except="$EXCEPTIONS" '
    { if ($0 ~ except) next
      if ($0 !~ allow) { print "DISALLOWED: " $0; bad=1 } }
    END { exit bad }
  ' "$f"
done
```

### 6.5 SearXNG Dated Digest Pin (REQ-DEP-005)

Digest discovery procedure (recorded in `docs/dependencies.md`):

1. Pull the current `:latest` tag: `docker pull searxng/searxng:latest`
2. Capture the digest: `docker inspect searxng/searxng:latest --format='{{index .RepoDigests 0}}'`
3. Alternative: query Docker Hub API `https://hub.docker.com/v2/repositories/searxng/searxng/tags/?page_size=10` for dated tags.
4. Record result as `searxng/searxng@sha256:<64-hex>` in `deploy/docker-compose.yml`.
5. Document the date of capture alongside the digest in `docs/dependencies.md`.
6. Manual bumps require a new SPEC-DEP-NNN with justification.

### 6.6 Renovate Config (REQ-DEP-006)

```json
{
  "$schema": "https://docs.renovatebot.com/renovate-schema.json",
  "extends": ["config:recommended", ":dependencyDashboard"],
  "timezone": "UTC",
  "schedule": ["after 10pm every weekday", "before 5am every weekday", "every weekend"],
  "prConcurrentLimit": 5,
  "separateMajorMinor": true,
  "packageRules": [
    {
      "matchUpdateTypes": ["minor", "patch"],
      "groupName": "minor-and-patch-deps",
      "schedule": ["before 5am on monday"]
    },
    {
      "matchUpdateTypes": ["major"],
      "groupName": null,
      "labels": ["major-upgrade", "needs-review"]
    }
  ],
  "ignorePaths": [".moai/**"],
  "docker": { "enabled": false },
  "assignees": ["<branch-owner>"]
}
```

### 6.7 `scripts/gen-deps-manifest.sh` Outline (REQ-DEP-007)

```bash
#!/usr/bin/env bash
# scripts/gen-deps-manifest.sh
set -euo pipefail
OUT="docs/dependencies.md"

{
  cat docs/_deps-header.md   # static preamble (policy, allowlist reference)
  echo "## Go Dependencies"
  echo '```'
  go list -m all
  echo '```'

  echo "## Python Dependencies (per service)"
  for svc in services/*/; do
    [ -f "$svc/pyproject.toml" ] || continue
    echo "### $svc"
    echo '```'
    (cd "$svc" && uv pip freeze)
    echo '```'
  done

  echo "## Web Dependencies"
  echo '```'
  pnpm --dir apps/web list --depth=0
  echo '```'

  cat docs/_deps-compose-table.md   # hardcoded compose service table
} > "$OUT"
```

## 7. File Impact

### Created

| File | Purpose |
|------|---------|
| `.github/workflows/deps-audit.yml` | Audit CI (govulncheck, pip-audit, pnpm audit, hadolint, license scan, SearXNG digest check) |
| `.github/workflows/pre-commit-autoupdate.yml` | Weekly pre-commit autoupdate PR |
| `renovate.json` | Renovate bot configuration (root location per Renovate best practice) |
| `docs/licenses/.gitkeep` | Placeholder; `go.txt`/`python.txt`/`web.txt` populated by CI on main-branch scheduled run |
| `docs/dependencies.md` | Manifest of all direct dependencies (Go, Python per-service, web, compose) |
| `docs/_deps-header.md` | Static preamble for manifest (policy, allowlist, instructions) |
| `docs/_deps-compose-table.md` | Static compose service table for manifest |
| `scripts/gen-deps-manifest.sh` | Idempotent manifest generator |
| `scripts/check-license-allowlist.sh` | License enforcement script |

### Modified

| File | Change |
|------|--------|
| `deploy/docker-compose.yml` | Replace `searxng/searxng:latest` with dated digest (captured at SPEC merge time) |
| `.pre-commit-config.yaml` | Run `pre-commit autoupdate` once during SPEC-DEP-001 run phase to pin current stable revs |

## 8. TDD Plan

Representative RED tests (written before implementation):

| Test | Layer | Assertion |
|------|-------|-----------|
| `TestGovulncheckInvocation` | CI workflow YAML | `.github/workflows/deps-audit.yml` contains a job step invoking `govulncheck ./...` |
| `TestPipAuditInvocation` | CI workflow YAML | `deps-audit.yml` contains `pip-audit` step per service matrix |
| `TestPnpmAuditInvocation` | CI workflow YAML | `deps-audit.yml` contains `pnpm audit --audit-level=high` in `apps/web` |
| `TestHadolintInvocation` | CI workflow YAML | `deps-audit.yml` uses `hadolint/hadolint-action@vN` with `recursive: true` |
| `TestLicenseAllowlist` | shell script | `scripts/check-license-allowlist.sh` exits 0 on synthetic allowed-only fixture, exits 1 on synthetic disallowed fixture |
| `TestRenovateConfigValid` | JSON schema | `renovate.json` validates against Renovate schema (via `ajv` or Renovate's own `--dry-run`) |
| `TestRenovateConfigRules` | JSON assertion | `renovate.json` has `prConcurrentLimit: 5`, ignores `.moai/**`, disables docker digests |
| `TestSearXNGDigestPinned` | regex on compose file | `deploy/docker-compose.yml` SearXNG image matches digest/dated regex, `:latest` absent |
| `TestDepsManifestGeneration` | shell script | `scripts/gen-deps-manifest.sh` runs successfully, `docs/dependencies.md` non-empty, contains `Go Dependencies`, `Python Dependencies`, `Web Dependencies`, `Compose Services` headings |
| `TestDepsManifestIdempotent` | shell script | Running `gen-deps-manifest.sh` twice produces identical output (diff empty) |
| `TestPreCommitAutoupdateWorkflow` | CI workflow YAML | `.github/workflows/pre-commit-autoupdate.yml` contains `pre-commit autoupdate` step and `peter-evans/create-pull-request` step |
| `TestGoFutureDepsListed` | content | `docs/dependencies.md` Go policy section includes `chi`, `client_golang`, `asynq` mapped to their SPEC IDs |

Coverage target: 85% of the policy-enforcement surface (scripts + workflow assertions). Workflow YAML coverage measured via file-presence + content-match assertions; script coverage via `bats` or direct invocation tests.

## 9. Dependencies

- **Prerequisite**: SPEC-BOOT-001 MUST be merged to `main` before SPEC-DEP-001 enters run phase (stacked PR pattern; `depends_on` in front-matter enforces).
- **External (user action)**: Renovate GitHub App installed on the repository. Not automated by this SPEC; documented in `docs/dependencies.md` as a one-time setup step.
- **Tooling versions**: govulncheck, pip-audit, hadolint, go-licenses, pip-licenses, license-checker versions pinned in CI workflows (exact versions captured at SPEC run time).

## 10. Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Renovate opens too many PRs, overwhelming reviewers | Medium | Medium | `prConcurrentLimit: 5` + minor/patch grouping into a single weekly PR |
| License allowlist false positive blocks legitimate dep (e.g., dual-licensed) | Medium | High (blocks merge) | Override mechanism via per-dependency exception line in `scripts/check-license-allowlist.sh` (documented); manual exception requires SPEC annotation |
| `govulncheck` reports noisy transitive vulnerabilities irrelevant to our call paths | High | Low | Use `govulncheck`'s built-in call-graph filter (reports only reachable vulns by default); manual suppression via `//go:linkname`-style ignore file if needed |
| Weekly cron drift or missed runs | Low | Low | Explicit cron expression `0 6 * * 1`; `workflow_dispatch` trigger available for manual runs; dependency-dashboard issue from Renovate provides visibility |
| SearXNG digest pin goes stale (security patch missed) | Medium | Medium | `docs/dependencies.md` records pin date; quarterly review reminder in dependency-dashboard issue; explicit SPEC-DEP-NNN for each bump |

## 11. Open Questions

1. **Renovate auto-merge for patch-only**: Should Renovate auto-merge patch-level updates that pass all CI checks, or always require human review? Default for this SPEC: **PR only, no auto-merge**. Revisit after 4 weeks of operation.
2. **License allowlist override mechanism**: PR-comment-based (`/license-override <package>`) vs a static `scripts/license-exceptions.txt` file? Default: **static exceptions file**, one line per exception with justification comment.
3. **Pre-commit autoupdate PR auto-merge**: If all hooks still pass after autoupdate, should the PR auto-merge? Default: **manual review**; reconsider once the pre-commit set has stabilized over 4 weekly cycles.

## 12. HISTORY

- 2026-04-24: Initial draft (v0.1) — limbowl, manager-spec. Scope derived from SPEC-DEP-001 user-specified items (a–g) in the plan-phase team-lead prompt. Built on SPEC-BOOT-001 baseline (PR #1, pending merge).
