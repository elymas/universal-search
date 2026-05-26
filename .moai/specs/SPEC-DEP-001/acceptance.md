# SPEC-DEP-001 Acceptance — Given/When/Then Scenarios

Created: 2026-04-24
Updated: 2026-05-04 (post-implementation reconciliation)
Author: limbowl (via manager-spec)
Status: implemented

## 0. Document Purpose

Given/When/Then acceptance scenarios for SPEC-DEP-001 — the dependency
pinning policy, audit CI, and license enforcement layer that sits on
top of SPEC-BOOT-001's baseline pinning.

## 1. Coverage Matrix

| AC | Scenario | REQs covered |
|----|----------|--------------|
| AC-001 | Go policy section + future-deps table present | REQ-DEP-001 |
| AC-002 | Weekly pre-commit autoupdate cron opens PR | REQ-DEP-002 |
| AC-003 | Audit CI runs govulncheck + pip-audit + pnpm audit + hadolint on PR | REQ-DEP-003 |
| AC-004 | License scan + allowlist enforcement | REQ-DEP-004 |
| AC-005 | SearXNG image dated-digest pin (no `:latest`) | REQ-DEP-005 |
| AC-006 | Renovate config valid + rules enforced | REQ-DEP-006 |
| AC-007 | `docs/dependencies.md` manifest + idempotent generator | REQ-DEP-007 |
| NFR-001 | Audit CI tool versions pinned (no `@main`/`@latest`) | NFR-DEP-001 |
| NFR-002 | Renovate weekly cadence + concurrent PR cap | NFR-DEP-002 |
| NFR-003 | License enforcement automated (no manual step) | NFR-DEP-003 |

## 2. Definition of Done

- [x] All 7 EARS REQs have green tests or CI gates.
- [x] All 3 NFRs validated.
- [x] `docs/dependencies.md` non-empty and contains 4 named sections.
- [x] `scripts/gen-deps-manifest.sh` idempotent (clean-tree diff empty).
- [x] `scripts/check-license-allowlist.sh` passes on current
      lockfile state.
- [x] `.github/workflows/deps-audit.yml` green on `main`.
- [x] `.github/workflows/pre-commit-autoupdate.yml` syntax-valid.
- [x] `renovate.json` validates against Renovate JSON schema.
- [x] `deploy/docker-compose.yml` SearXNG image is `searxng/searxng@sha256:...`
      (no `:latest`).
- [x] License compliance: only allowlisted licenses present in
      direct dependencies (SearXNG service-boundary exception
      documented in NOTICE).

## 3. Functional Scenarios

### AC-001 — Go policy + future-deps table

Maps to REQ-DEP-001.

- **Given** `docs/dependencies.md` exists.
- **When** an engineer opens the file and searches for `## Go Policy`
  or `### Go Dependency Pinning Policy`.
- **Then** the section is present and enumerates:
  - Direct pin strategy (exact minor version in `go.mod`).
  - Transitive pin strategy (locked via `go.sum`; `go mod tidy` pre-commit).
  - Minor-range exception (stdlib `golang.org/x/*` rolling).
  - Security update SLA: HIGH within 7 days, MEDIUM next weekly,
    LOW next cycle.
  - New-dep PR requirement: must reference a SPEC and add to the
    future-deps table.
- **And** the future-deps table contains at minimum:
  - `github.com/go-chi/chi/v5` → SPEC-IR-001
  - `github.com/prometheus/client_golang` → SPEC-OBS-001
  - `github.com/hibiken/asynq` → SPEC-LLM-001 (originally; in practice
    asynq is deferred to a later SPEC, but the placeholder remains)

### AC-002 — Weekly pre-commit autoupdate

Maps to REQ-DEP-002.

- **Given** `.github/workflows/pre-commit-autoupdate.yml` exists.
- **When** the workflow runs on cron `0 6 * * 1` (or
  `workflow_dispatch`).
- **Then** steps execute in order:
  1. `actions/checkout@v4`
  2. `actions/setup-python@v5` with `python-version: "3.12"`
  3. `pip install pre-commit==<pinned>`
  4. `pre-commit autoupdate`
- **And** IF any hook rev changed, `peter-evans/create-pull-request@v6`
  opens a PR titled `"chore(deps): pre-commit autoupdate"` with
  labels `dependencies,automated`.

### AC-003 — Dependency audit CI

Maps to REQ-DEP-003.

- **Given** `.github/workflows/deps-audit.yml` exists.
- **When** the workflow runs on `pull_request` or weekly cron.
- **Then** four jobs execute in parallel:
  - `govulncheck-go`: `govulncheck ./...`
  - `pip-audit-python`: matrix per service runs `uv run pip-audit`
  - `pnpm-audit-web`: `pnpm --dir web audit --audit-level=high`
  - `hadolint-docker`: `hadolint/hadolint-action@v3.1.0` recursive
- **And** any HIGH severity finding fails the PR status check.
- **And** MEDIUM/LOW findings are posted as PR comments but do not
  block.

### AC-004 — License scan + allowlist

Maps to REQ-DEP-004.

- **Given** the `license-scan` job in `deps-audit.yml`.
- **When** the job runs:
  1. `go-licenses report ./... > docs/licenses/go.txt`
  2. `pip-licenses --format=plain > docs/licenses/python.txt`
  3. `license-checker --start web --out docs/licenses/web.txt`
  4. `bash scripts/check-license-allowlist.sh`
- **Then** the script exits 0 when every direct dep license is in the
  allowlist (Table 5.1 in plan.md / spec.md §5.1).
- **And** the script exits 1 when any direct dep license is outside
  the allowlist (excluding the SearXNG AGPL service-boundary
  exception).
- **And** `docs/licenses/{go,python,web}.txt` are regenerated on
  every scheduled main-branch run.

### AC-005 — SearXNG dated-digest pin

Maps to REQ-DEP-005.

- **Given** `deploy/docker-compose.yml`.
- **When** the regex
  `^\s*image:\s*searxng/searxng(@sha256:[a-f0-9]{64}|:[0-9]{4}\.[0-9]{2}\.[0-9]{2}-[a-f0-9]+)$`
  is applied.
- **Then** there is at least one match.
- **And** the literal substring `searxng/searxng:latest` does NOT appear.
- **And** `docs/dependencies.md` records the pinned digest with the
  date and source of discovery.

### AC-006 — Renovate config

Maps to REQ-DEP-006.

- **Given** `renovate.json` at repo root.
- **When** validated against the Renovate JSON schema.
- **Then** validation passes.
- **And** the following configuration is enforced:
  - `extends: ["config:recommended", ":dependencyDashboard"]`
  - `schedule: ["after 10pm every weekday", "before 5am every
    weekday", "every weekend"]`
  - `prConcurrentLimit: 5`
  - `separateMajorMinor: true`
  - `packageRules`: minor/patch → `groupName: "minor-and-patch-deps"`,
    `schedule: ["before 5am on monday"]`; major → separate PR with
    labels `["major-upgrade", "needs-review"]`
  - `ignorePaths: [".moai/**"]`
  - `docker.enabled: false` (image digests are manual per REQ-DEP-005)
  - `assignees: ["<branch-owner>"]`

### AC-007 — Manifest + idempotency

Maps to REQ-DEP-007.

- **Given** `scripts/gen-deps-manifest.sh` and the current lockfile
  state.
- **When** the engineer runs `bash scripts/gen-deps-manifest.sh`.
- **Then** `docs/dependencies.md` is regenerated with sections:
  - `## Go Dependencies` (output of `go list -m all`)
  - `## Python Dependencies (per service)` (per-service
    `uv pip freeze`)
  - `## Web Dependencies` (`pnpm --dir web list --depth=0`)
  - `## Compose Services` (static table)
- **And** running the script twice in succession on a clean tree
  produces no diff (`git diff --exit-code docs/dependencies.md`
  exits 0).
- **And** the script completes within 30 s locally on a warm cache.
- **And** CI verifies manifest drift on every PR touching dep
  manifests.

## 4. Non-Functional Acceptance

### NFR-DEP-001 — Reproducibility

- Every GitHub Actions `uses:` reference in `.github/workflows/deps-audit.yml`
  and `.github/workflows/pre-commit-autoupdate.yml` uses
  `@vN.N.N` (no `@main`, no `@latest`).
- Local reproduction via `act` produces identical command output to
  CI.

### NFR-DEP-002 — Upgrade velocity

- Renovate cadence: weekly for minor/patch (grouped, off-hours);
  on-demand for major (separate PRs).
- Pre-commit autoupdate cadence: weekly (`0 6 * * 1`).
- Maximum 5 open Renovate PRs at any time
  (`prConcurrentLimit: 5`).

### NFR-DEP-003 — License compliance

- License enforcement runs automatically on every PR — no manual
  review step.
- Allowlist violations block merge.
- License data is regenerated on every PR touching `go.mod`, any
  `services/*/pyproject.toml`, or `web/package.json`.

## 5. Edge Cases

### EC-001 — Renovate weekly PR contains breaking change

- Minor/patch group includes a backward-incompatible patch (rare).
- Audit CI on the PR catches via `govulncheck` or `pnpm audit` if it
  introduces a vulnerability; the test suite on the PR catches
  behavioural regressions; human reviewer makes the merge decision.

### EC-002 — License scan reports a new "UNKNOWN" license

- The dep is blocked by `scripts/check-license-allowlist.sh`.
- Resolution: the SPEC introducing the dep adds an exception line to
  the script with comment justification AND updates Table 5.1 if the
  license is now broadly approved.

### EC-003 — SearXNG digest 404s upstream

- The compose pull fails; `compose-check.yml` CI job fails.
- Resolution: a new SPEC-DEP-NNN bumps the pin with the next stable
  digest.

### EC-004 — `govulncheck` reports a vuln in a transitive dep not
exercised by our call paths

- govulncheck's call-graph filter normally suppresses these.
- If a false-positive HIGH leaks through, suppress via go-vuln's
  ignore mechanism in a follow-up SPEC.

### EC-005 — Manifest drift on docker compose service version bump

- `gen-deps-manifest.sh` regenerates the Compose Services table from
  `docs/_deps-compose-table.md` (which is static); a compose image
  bump requires editing that table by hand.
- CI catches manifest drift via `git diff --exit-code`.

### EC-006 — Pre-commit autoupdate PR fails CI

- The autoupdate PR contains hook rev bumps that fail the
  `pre-commit run --all-files` step in `.github/workflows/pre-commit.yml`.
- Resolution: human investigates; manually pins the failing hook to
  the previous rev; commits onto the autoupdate branch.

## 6. Quality Gate Criteria

| Criterion | Threshold | Source |
|-----------|-----------|--------|
| Audit CI green on main | yes | REQ-DEP-003 |
| All `uses:` actions pinned `@vN.N.N` | yes | NFR-DEP-001 |
| `scripts/check-license-allowlist.sh` exit | 0 | REQ-DEP-004 |
| `scripts/gen-deps-manifest.sh` idempotency | clean diff | REQ-DEP-007 |
| Renovate JSON schema validation | pass | REQ-DEP-006 |
| SearXNG image regex match | pass (no `:latest`) | REQ-DEP-005 |
| `docs/dependencies.md` section coverage | 4 sections present | REQ-DEP-007 |
| Pre-commit autoupdate cron pinned | `0 6 * * 1` | REQ-DEP-002 |

## 7. Out-of-Scope Confirmations

Restated from spec.md §2.2:

- Re-pinning what BOOT-001 already pinned (compose images, lockfiles)
- Full security audit and threat modeling → SPEC-SEC-001 (M8)
- Private registry / artifact proxy / SBOM signing → future SPEC
- Runtime vulnerability scanning of deployed containers →
  SPEC-OBS-001 or SPEC-SEC-001
- Auto-merging Renovate PRs → out of scope (Open Question deferred)

---

*End of acceptance.md (post-hoc).*
