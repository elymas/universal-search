# Dependency Manifest

Auto-generated dependency manifest for the Universal Search project.
See [SPEC-DEP-001](/.moai/specs/SPEC-DEP-001/spec.md) and
[docs/tech.md](tech.md) for architectural context.

The machine-generated sections (Go, Python, Web) are produced by
`scripts/gen-deps-manifest.sh`. Rerun after any dependency changes.

---

## Go Dependency Pinning Policy

> **Direct dependencies**: Pinned to exact minor version in `go.mod` (e.g., `v5.1.0` not `v5`).
> Upgrades via Renovate PR.
>
> **Transitive dependencies**: Locked via `go.sum` checksums. `go mod tidy` enforced in pre-commit.
>
> **Minor-range exception**: Standard library modules (`golang.org/x/*`) MAY use latest patch
> within the minor line; Renovate weekly bump.
>
> **Security updates**:
>
> - HIGH severity → patch within 7 days (emergency SPEC or hotfix PR)
> - MEDIUM → next weekly Renovate run
> - LOW → next scheduled dependency review
>
> **New dependencies**: Require SPEC reference in the PR description and an entry in the
> Future-Dependencies Placeholder table below.

## Future-Dependencies Placeholder

Planned Go dependencies not yet added. Each must be introduced via a PR referencing the
corresponding consumer SPEC.

| Package                               | Planned Consumer SPEC       | Purpose                                       |
| ------------------------------------- | --------------------------- | --------------------------------------------- |
| `github.com/go-chi/chi/v5`            | SPEC-IR-001                 | HTTP router for Information Retrieval service |
| `github.com/prometheus/client_golang` | SPEC-OBS-001                | Prometheus metrics instrumentation            |
| `github.com/hibiken/asynq`            | SPEC-LLM-001                | Redis-backed task queue for LLM orchestration |
| `github.com/jackc/pgx/v5`             | SPEC-DB-001 (tentative)     | PostgreSQL driver                             |
| `github.com/qdrant/go-client`         | SPEC-VECTOR-001 (tentative) | Qdrant client                                 |

---

## License Allowlist

License compliance is automated via `scripts/check-license-allowlist.sh`.
See [SPEC-DEP-001 §5.1](/.moai/specs/SPEC-DEP-001/spec.md) for the allowlist table.

Approved licenses: MIT, Apache-2.0, BSD-2-Clause, BSD-3-Clause, ISC, PostgreSQL, MPL-2.0

Pre-approved exceptions:

- `searxng/searxng` (AGPL-3.0): consumed as an external Docker service, not linked.
  Service boundary means AGPL copyleft does not apply. See NOTICE.

---

## SearXNG Digest Pin Procedure

Per REQ-DEP-005, the SearXNG image must be pinned to a dated tag or sha256 digest.

**Current pin** (captured 2026-04-24 via Docker Hub API):

- Tag: `searxng/searxng:2026.04.22-74f1ca203`
- Digest: `sha256:37c616a774b90fb5df9239eb143f1b11866ddf7b830cd1ebcca6ba11b38cc2bf`

**Procedure to update the pin**:

1. Query Docker Hub API: `curl "https://hub.docker.com/v2/repositories/searxng/searxng/tags/?page_size=20&ordering=last_updated"`
2. Identify the most recent stable dated tag (format `YYYY.MM.DD-<hash>`, not `latest` or `edge`).
3. Or pull and capture digest: `docker pull searxng/searxng:latest && docker inspect searxng/searxng:latest --format='{{index .RepoDigests 0}}'`
4. Update `deploy/docker-compose.yml` image line to `searxng/searxng@sha256:<64-hex>` or `searxng/searxng:YYYY.MM.DD-<hash>`.
5. Update this document with the new pin date and digest value.
6. Manual bumps require a new SPEC-DEP-NNN with justification.

---

## Setup Requirements

**Renovate GitHub App** must be installed on the repository for automated dependency PRs (REQ-DEP-006).
This is a one-time user action and is not automated by SPEC-DEP-001.

Installation: https://github.com/apps/renovate

After installation, Renovate will create a Dependency Dashboard issue and begin opening
PRs according to the schedule in `renovate.json`.

---
