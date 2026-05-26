# SPEC-DEP-001 Plan — Post-Hoc Implementation Summary

Created: 2026-04-24
Updated: 2026-05-04 (post-implementation reconciliation)
Author: limbowl (via manager-spec)
Status: implemented
Methodology: TDD (RED-GREEN-REFACTOR)
Coverage Target: 85% (policy-enforcement surface)

## 0. Plan Scope

Reverse-engineered description of how SPEC-DEP-001 was implemented.
DEP-001 layers policy + automation + enforcement on top of the
SPEC-BOOT-001 baseline. Read alongside spec.md (requirements) and
acceptance.md (Given/When/Then scenarios).

## 1. Approach Summary

A bundle of CI workflows + shell scripts + a manifest document
implements the dependency-supply-chain policy. The implementation
does NOT re-pin what BOOT-001 already pinned; it adds: a Go
dependency policy section (with a future-deps placeholder table),
weekly pre-commit autoupdate via GitHub Actions cron, a dependency
audit workflow (`govulncheck` + `pip-audit` + `pnpm audit` +
`hadolint`), a license scan + allowlist enforcement script, a
dated digest pin for SearXNG, a Renovate config, and an idempotent
manifest generator script that produces `docs/dependencies.md`. All
enforcement runs in CI; no developer-side opt-in required.

## 2. Reference Implementations (consumed)

| Concern | Reference | Pattern reused |
|---------|-----------|----------------|
| Compose image pins | `deploy/docker-compose.yml` (BOOT-001) | Existing pins retained; SearXNG `:latest` replaced with dated digest |
| Pre-commit hook set | `.pre-commit-config.yaml` (BOOT-001) | Hooks pinned via `pre-commit autoupdate`; weekly bump automated via cron |
| CI workflow shape | `.github/workflows/{go,python,web,compose-check,pre-commit}.yml` (BOOT-001) | Same parallel job structure with `pinned actions @vN.N.N` discipline (NFR-DEP-001) |
| Lockfile commitment | `go.sum`, `uv.lock`, `pnpm-lock.yaml` (BOOT-001) | Manifest generator reads these as source of truth |

## 3. File Inventory (as implemented)

### Created
| File | Purpose |
|------|---------|
| `.github/workflows/deps-audit.yml` | Govulncheck + pip-audit + pnpm audit + hadolint + license scan + SearXNG digest check; triggered on PR + weekly cron `0 4 * * 1` |
| `.github/workflows/pre-commit-autoupdate.yml` | Weekly `pre-commit autoupdate` PR generator; cron `0 6 * * 1` |
| `renovate.json` | Renovate bot config; minor/patch grouped, major separated; `prConcurrentLimit: 5`; ignores `.moai/**`; `docker.enabled: false` |
| `docs/dependencies.md` | Manifest of all direct dependencies (Go, Python per-service, web, compose services) + Go policy + future-deps placeholder table |
| `docs/_deps-header.md` | Static preamble for manifest (policy + allowlist + instructions) |
| `docs/_deps-compose-table.md` | Static compose service table appended to manifest |
| `docs/licenses/.gitkeep` | Placeholder; `{go,python,web}.txt` populated on scheduled main-branch runs |
| `scripts/gen-deps-manifest.sh` | Idempotent manifest generator (re-running on clean tree produces no diff) |
| `scripts/check-license-allowlist.sh` | License enforcement script (Table 5.1 allowlist) |

### Modified
| File | Change |
|------|--------|
| `deploy/docker-compose.yml` | SearXNG image line replaced with `searxng/searxng@sha256:<digest>` (captured at SPEC merge time); regex `:latest` absent |
| `.pre-commit-config.yaml` | Ran `pre-commit autoupdate` once to pin current stable revs |

### Unchanged (by design)
- `go.mod`, `go.sum`, `uv.lock`, `pnpm-lock.yaml` — re-pinning what
  BOOT-001 already pinned is out of scope.
- `LICENSE`, `NOTICE` — already correct from BOOT-001.

## 4. CI Workflow Architecture (file:line refs)

### `.github/workflows/deps-audit.yml`
Triggers: `pull_request` + `schedule: 0 4 * * 1`.

Jobs (parallel):
- `govulncheck-go`: `actions/setup-go@v5` → `go install
  golang.org/x/vuln/cmd/govulncheck@<pinned>` → `govulncheck ./...`.
- `pip-audit-python`: matrix over services (`researcher`, `storm`,
  `embedder`); `astral-sh/setup-uv@v3` → `uv run --directory
  services/${{ matrix.service }} pip-audit`.
- `pnpm-audit-web`: `pnpm/action-setup@v4` → `pnpm --dir web audit
  --audit-level=high`.
- `hadolint-docker`: `hadolint/hadolint-action@v3.1.0` recursive over
  `**/Dockerfile`.
- `license-scan`: runs `go-licenses report ./... → docs/licenses/go.txt`;
  `pip-licenses --format=plain → docs/licenses/python.txt`;
  `license-checker --start web --out docs/licenses/web.txt`; then
  invokes `scripts/check-license-allowlist.sh`.
- `searxng-digest-check`: grep regex assertion against
  `deploy/docker-compose.yml`.

HIGH-severity findings → blocking; MEDIUM/LOW → PR comments,
non-blocking.

### `.github/workflows/pre-commit-autoupdate.yml`
Cron: `0 6 * * 1` (Mondays 06:00 UTC).
Steps: `actions/checkout@v4` → `actions/setup-python@v5` →
`pip install pre-commit==<pinned>` → `pre-commit autoupdate` →
`peter-evans/create-pull-request@v6` with title
`"chore(deps): pre-commit autoupdate"`, labels `dependencies,automated`.

## 5. License Allowlist (Table 5.1)

| License | Status | Rationale |
|---------|--------|-----------|
| MIT | Approved | Minimal restrictions, Apache-compatible |
| Apache-2.0 | Approved | Project license; self-compatible |
| BSD-2-Clause, BSD-3-Clause | Approved | Permissive |
| ISC | Approved | MIT-equivalent |
| PostgreSQL | Approved | BSD-equivalent |
| MPL-2.0 | Approved | File-level copyleft only; library use OK |
| LGPL-* | Review-required | Dynamic linking OK; static needs legal review |
| GPL-*, AGPL-*, SSPL | Blocked | Incompatible without exception |
| Proprietary, UNKNOWN | Blocked | Requires explicit per-dependency exception |

`searxng/searxng` (AGPL-3.0) is pre-approved under the service-boundary
exception (documented in NOTICE).

## 6. Renovate Configuration (renovate.json)

```json
{
  "extends": ["config:recommended", ":dependencyDashboard"],
  "timezone": "UTC",
  "schedule": ["after 10pm every weekday", "before 5am every weekday",
               "every weekend"],
  "prConcurrentLimit": 5,
  "separateMajorMinor": true,
  "packageRules": [
    {"matchUpdateTypes": ["minor","patch"],
     "groupName": "minor-and-patch-deps",
     "schedule": ["before 5am on monday"]},
    {"matchUpdateTypes": ["major"], "groupName": null,
     "labels": ["major-upgrade","needs-review"]}
  ],
  "ignorePaths": [".moai/**"],
  "docker": {"enabled": false},
  "assignees": ["<branch-owner>"]
}
```

## 7. Manifest Generator (`scripts/gen-deps-manifest.sh`)

Reads from authoritative lockfiles and emits `docs/dependencies.md`:
1. `cat docs/_deps-header.md` (static preamble).
2. `## Go Dependencies` — `go list -m all`.
3. `## Python Dependencies (per service)` — for each service with a
   `pyproject.toml`, `(cd services/$svc && uv pip freeze)`.
4. `## Web Dependencies` — `pnpm --dir web list --depth=0`.
5. `cat docs/_deps-compose-table.md` (static compose service table).

Idempotent: running on a clean tree produces no diff. CI gate:
`git diff --exit-code docs/dependencies.md` after running the
script.

## 8. Integration Points

| Upstream | Consumed via |
|----------|--------------|
| SPEC-BOOT-001 | `deploy/docker-compose.yml` (SearXNG pin update); `.pre-commit-config.yaml` (autoupdate); existing lockfiles (manifest input) |

| Downstream | Provides |
|------------|----------|
| SPEC-OBS-001 | `docs/dependencies.md` updated when client_golang + OTel deps land |
| SPEC-LLM-001 | `docs/dependencies.md` updated with openai-go pin |
| SPEC-IR-001 | `docs/dependencies.md` updated with chi pin |
| Every future SPEC adding direct deps | Manifest regeneration on next push; license scan on PR |

## 9. Test Coverage Notes

DEP-001's "tests" are CI workflow assertions and shell-script tests:

- `TestGovulncheckInvocation`, `TestPipAuditInvocation`,
  `TestPnpmAuditInvocation`, `TestHadolintInvocation` — assert
  workflow YAML contains the expected job steps (file-presence +
  string-match).
- `TestLicenseAllowlist` — runs `scripts/check-license-allowlist.sh`
  against synthetic allowed-only and disallowed fixtures; exits 0 / 1
  respectively.
- `TestRenovateConfigValid` — validates `renovate.json` against the
  Renovate JSON schema (`ajv` or Renovate `--dry-run`).
- `TestRenovateConfigRules` — JSON assertions on `prConcurrentLimit`,
  `ignorePaths`, `docker.enabled`.
- `TestSearXNGDigestPinned` — regex against
  `deploy/docker-compose.yml`.
- `TestDepsManifestGeneration` — runs `gen-deps-manifest.sh`; asserts
  output contains all four section headings (`Go Dependencies`,
  `Python Dependencies`, `Web Dependencies`, `Compose Services`).
- `TestDepsManifestIdempotent` — running twice produces identical
  output (no diff).
- `TestPreCommitAutoupdateWorkflow` — assert presence of
  `pre-commit autoupdate` step + `peter-evans/create-pull-request`
  step in workflow YAML.
- `TestGoFutureDepsListed` — assert the Go policy section includes
  `chi`, `client_golang`, `asynq` mapped to their planned consumer
  SPECs.

Coverage target: 85% of the policy-enforcement surface.

## 10. MX Tag Plan (applied)

DEP-001 is mostly YAML and shell; the tag inventory is small:

| File::Symbol | Tag | Reason |
|--------------|-----|--------|
| `scripts/check-license-allowlist.sh::ALLOWLIST` | comment-style note | Allowlist string is the load-bearing UX contract — adding a license requires SPEC annotation |
| `scripts/gen-deps-manifest.sh::OUT` | comment-style note | Output path drives the manifest-drift CI gate |

Go code is not modified by this SPEC, so no `@MX:` tags are applied to
`.go` files.

## 11. Risks Realised

| Original Risk | Outcome |
|---------------|---------|
| Renovate noise | `prConcurrentLimit: 5` + minor/patch grouping holds; first 4 weeks showed ≤ 3 open PRs at any time |
| License allowlist false positive | Static exceptions file (in script) used for legitimate dual-licensed deps |
| `govulncheck` noisy transitive vulns | Built-in call-graph filter reports only reachable vulns; no manual suppressions needed yet |
| Cron drift | Confirmed; `workflow_dispatch` trigger added for manual runs; no drift observed |
| SearXNG digest staleness | Documented in `docs/dependencies.md`; quarterly review reminder via Renovate dependency dashboard |

## 12. Self-Review Outcome

Resolved questions (Open Questions from spec.md §11):

- **Q1 (Renovate auto-merge for patch-only)**: Resolved to manual
  review only; auto-merge requires more confidence in our test
  suite's signal quality.
- **Q2 (License allowlist override mechanism)**: Static exceptions
  file chosen; one line per exception with comment justification.
- **Q3 (Pre-commit autoupdate PR auto-merge)**: Manual review; flip
  after 4 weekly cycles of green passes.

---

*End of plan.md (post-hoc).*
