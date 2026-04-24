# Changelog

All notable changes to this project are documented in this file.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
versioning follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **SPEC-BOOT-001** — M1 Foundation repo scaffold and CI bootstrap
  - Go module `github.com/elymas/universal-search` with `cmd/usearch` CLI (prints semver via `--version`), `internal/` domain stubs, `pkg/` public interfaces
  - Python `uv` workspace with three services (`researcher`, `storm`, `embedder`), each with `pyproject.toml`, `Dockerfile`, test skeleton
  - Next.js 16 web scaffold under `web/` with Tailwind, shadcn/ui config, ESLint + Prettier
  - `deploy/docker-compose.yml` with six pinned services (Qdrant v1.16.3, Meilisearch v1.42.1, PostgreSQL 16.13-alpine3.23, SearXNG, LiteLLM v1.83.7-stable.patch.1, Redis 7-alpine), all healthchecked, `${VAR}` env interpolation, named volumes
  - GitHub Actions CI matrix (`go-ci`, `python-ci`, `web-ci`, `compose-check`, `pre-commit`) on Node 22 LTS with all actions pinned
  - `.pre-commit-config.yaml` (gofmt, goimports, ruff, prettier, eslint, trailing-whitespace, end-of-file-fixer, hadolint, shellcheck, yamllint)
  - `Makefile` (dev, test, lint, build, clean, compose-up/down, fmt, tidy, install-py), `.editorconfig`, `LICENSE` (Apache-2.0), `NOTICE`, `README.md`
- **SPEC-DEP-001** — Dependency pinning policy and audit CI
  - `docs/dependencies.md` manifest with Go pinning policy, future-dependencies placeholder table (chi → SPEC-IR-001, client_golang → SPEC-OBS-001, asynq → SPEC-LLM-001, pgx → SPEC-DB-001, qdrant/go-client → SPEC-VECTOR-001), compose service table, license allowlist
  - `.github/workflows/deps-audit.yml` running `govulncheck`, `pip-audit` (per-service matrix), `pnpm audit`, `hadolint`, license scan with allowlist enforcement, and SearXNG digest regression check on every PR and weekly cron
  - `.github/workflows/pre-commit-autoupdate.yml` weekly cron (Monday 06:00 UTC) opening automated PR
  - `renovate.json` with `prConcurrentLimit: 5`, minor/patch grouping, `.moai/**` ignored, docker digest updates disabled (manual SPEC-gated)
  - `scripts/gen-deps-manifest.sh` idempotent manifest generator
  - `scripts/check-license-allowlist.sh` enforcing MIT / Apache-2.0 / BSD-* / ISC / PostgreSQL / MPL-2.0 with SearXNG AGPL service-boundary exception, supporting `$LICENSE_DIR` override for tests
  - `tests/spec_dep_001_test.go` — 11 TDD acceptance tests covering REQ-DEP-001..007

### Changed
- **SearXNG image** pinned from `searxng/searxng:latest` to `searxng/searxng:2026.04.22-74f1ca203` (digest `sha256:37c616a774b90fb5df9239eb143f1b11866ddf7b830cd1ebcca6ba11b38cc2bf`, captured 2026-04-24 via Docker Hub API) per REQ-DEP-005
- **NOTICE** updated to point at `docs/dependencies.md` as the authoritative manifest

[Unreleased]: https://github.com/elymas/universal-search/commits/main
