# SPEC-DEP-001 Research Notes

## 1. Purpose of This SPEC

Narrow-scope hardening layered on top of SPEC-BOOT-001. BOOT-001 delivered the initial pinning baseline (compose tags, lockfiles, LICENSE/NOTICE). DEP-001 adds the surrounding **policy + automation + enforcement**: audit CI, license scanning, Renovate, a dated SearXNG digest replacing `:latest`, and a dependency manifest. No re-pinning of existing BOOT-001 artifacts.

## 2. Upstream Tool References

| Tool | Purpose | Upstream |
|------|---------|----------|
| `govulncheck` | Go vulnerability scanner with call-graph reachability | `golang.org/x/vuln/cmd/govulncheck` |
| `pip-audit` | Python vulnerability scan against PyPI advisories + OSV | `pypa/pip-audit` |
| `pnpm audit` | Node advisory scan via npm registry | `pnpm` built-in (`pnpm audit --audit-level=<level>`) |
| `hadolint` | Dockerfile linter (shell-check + best practices) | `hadolint/hadolint` + `hadolint/hadolint-action` |
| `go-licenses` | Go module license detection | `github.com/google/go-licenses` |
| `pip-licenses` | Python package license detection | `pip-licenses` on PyPI |
| `license-checker` | npm package license detection | `davglass/license-checker` |
| Renovate | Dependency upgrade PR automation | `renovatebot/renovate` (GitHub App) |
| `pre-commit autoupdate` | Hook rev bump in `.pre-commit-config.yaml` | `pre-commit/pre-commit` built-in |
| `peter-evans/create-pull-request` | PR-creating GitHub Action for autoupdate job | `peter-evans/create-pull-request@v6` |

## 3. SPEC-BOOT-001 Baseline (Already Delivered)

Delivered in PR #1 (pending merge to main):

- **Compose images (exact tags)**: Qdrant v1.16.3, Meilisearch v1.42.1, PostgreSQL 16.13-alpine3.23, SearXNG `searxng/searxng:latest` (⚠ DEP-001 replaces with digest), LiteLLM v1.83.7-stable.patch.1, Redis 7-alpine.
- **Python**: `uv.lock` with 240 packages resolved. Loose service ranges: gpt-researcher >=0.12,<1.0; knowledge-storm >=1.0,<2.0.
- **Web**: `pnpm-lock.yaml` with Next.js 16.2.4, shadcn 4.4.0, Tailwind v4, ESLint 9.
- **Go**: `go.mod` has single direct dep `gopkg.in/yaml.v3 v3.0.1` (used by `deploy/compose_test.go`); `go.sum` populated.
- **Pre-commit**: 10 hooks with approximate rev pins. `pre-commit autoupdate` not yet run — DEP-001 run phase runs it once to anchor current stable revs.
- **License**: Apache-2.0 `LICENSE` + `NOTICE` at repo root.

DEP-001 explicitly does **not** re-pin or modify any of the above except the SearXNG image (REQ-DEP-005) and `.pre-commit-config.yaml` (one-time autoupdate).

## 4. License Allowlist Rationale

Compatibility goal: permissive enough for redistribution under the project's Apache-2.0 license without triggering copyleft obligations.

| License | Why Approved |
|---------|--------------|
| MIT | Minimal restrictions; widely compatible |
| Apache-2.0 | Project license; self-compatible (patent grant aligned) |
| BSD-2/3-Clause | Permissive; long-standing Apache compatibility |
| ISC | MIT-equivalent; explicitly Apache-compatible |
| PostgreSQL | BSD-equivalent; permissive |
| MPL-2.0 | File-level weak copyleft only; acceptable when used as a library (not linked-modified) |

Blocked: GPL-*, AGPL, SSPL, proprietary-without-exception. These would force redistribution terms inconsistent with Apache-2.0 or create legal ambiguity.

**SearXNG AGPL exception**: SearXNG runs as a separate container process communicating over HTTP. No linking, no library inclusion. Service-boundary separation preserves Apache-2.0 compliance for our code. Exception documented in `scripts/check-license-allowlist.sh` and `docs/dependencies.md`.

## 5. SearXNG Digest Discovery

Two procedures for capturing a reproducible digest:

**Procedure A — Docker CLI (on a machine with current `:latest` pulled)**:
```
docker pull searxng/searxng:latest
docker inspect searxng/searxng:latest --format='{{index .RepoDigests 0}}'
# → searxng/searxng@sha256:<64-hex>
```

**Procedure B — Docker Hub API (no local pull required)**:
```
curl -s 'https://hub.docker.com/v2/repositories/searxng/searxng/tags/?page_size=25' \
  | jq -r '.results[] | select(.name | test("^[0-9]{4}\\.[0-9]{2}\\.[0-9]{2}")) | "\(.name) \(.digest)"' \
  | head -5
```

Pick the most recent dated tag with a stable digest. Record **both** the digest and the capture date in `docs/dependencies.md` §Compose Services for provenance. Future bumps require SPEC-DEP-NNN.

## 6. Approximate-Pin Context (from SPEC-BOOT-001)

`.pre-commit-config.yaml` currently uses approximate revs (nearest known stable, not necessarily latest). DEP-001 run phase:
1. Run `pre-commit autoupdate` once to pin to current latest stable revs.
2. Verify all hooks still pass.
3. Commit the updated `.pre-commit-config.yaml`.
4. From that point forward, the weekly CI job (REQ-DEP-002) keeps revs current via PRs.

This one-shot anchor is explicitly noted in `spec.md` §7 File Impact and §11 Open Questions (#3 auto-merge decision still open).

## 7. Out-of-Scope References

- **SPEC-SEC-001** (future): full threat model, SAST, secret scanning, SBOM signing.
- **SPEC-OBS-001** (blocked by this SPEC): runtime vulnerability scanning of running containers, Prometheus metrics client integration.
- **SPEC-LLM-001** (blocked by this SPEC): Asynq queue integration, which will trigger the first real Go dep addition under REQ-DEP-001 policy.

## 8. Key Design Decisions Locked

- **Renovate config location**: repository root (`renovate.json`). Rationale: Renovate best practice; easier discovery; no `.github/` conflation with workflows.
- **License scan output**: committed to `docs/licenses/` only on scheduled main-branch runs, not on every PR (avoids churn; PRs still run the scan for enforcement).
- **Renovate scope**: excludes `.moai/` (SPEC documents are human-curated) and docker image digests (manual per REQ-DEP-005).
- **Audit failure severity**: HIGH blocks merge; MEDIUM/LOW comment-only. Rationale: balance security with PR throughput; revisit after 4 weeks.
