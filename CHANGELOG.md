# Changelog

All notable changes to Universal Search are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

Work in progress on Milestone M1 (Foundation). See `.moai/project/roadmap.md` for the full backlog.

## [0.0.1-dev] — 2026-04-24

First development snapshot of the Universal Search monorepo. Establishes the
greenfield foundation for the M1 Foundation milestone. Nothing in this release
is user-facing or production-ready — it is the scaffold upon which every
subsequent milestone builds.

### Added

- **Project documentation** under `.moai/project/`: product, structure, tech, roadmap, and competitive research. Locks V1 scope, persona targets, success metrics, 9-milestone roadmap, and 32+ SPEC backlog.
- **SPEC-BOOT-001** — first implemented SPEC. Delivers:
  - Go module `github.com/elymas/universal-search` (Go 1.23) with `cmd/`, `internal/`, `pkg/`, `proto/` roots. Ten `internal/` domain stubs (router, fanout, adapters, index, llm, synthesis, auth, obs, eval, meta), two `pkg/` stubs (client, types), and three `cmd/` entrypoints (`usearch`, `usearch-mcp`, `usearch-api`).
  - `cmd/usearch/usearch --version` binary printing `usearch v0.0.1-dev`.
  - Python `uv` workspace with three sidecar services (`researcher`, `storm`, `embedder`), each exposing a FastAPI `/health` endpoint. Python baseline 3.11+.
  - Next.js 16 App Router web scaffold under `web/` with shadcn/ui, Tailwind v4, ESLint 9, Prettier, and a node:test scaffold existence test.
  - `deploy/docker-compose.yml` — six-service dev stack (Qdrant v1.16.3, Meilisearch v1.42.1, PostgreSQL 16.13-alpine3.23, SearXNG, LiteLLM v1.83.7-stable.patch.1, Redis 7-alpine) with named volumes, healthchecks, and `${VAR}` interpolation.
  - Root `.env.example` documenting every compose variable.
  - GitHub Actions CI: `go.yml`, `python.yml`, `web.yml`, `compose-check.yml`, `pre-commit.yml`.
  - `.pre-commit-config.yaml` with 10 hooks (gofmt, goimports, ruff, prettier, eslint, trailing-whitespace, end-of-file-fixer, hadolint, shellcheck, yamllint).
  - `.editorconfig` with per-language indent rules.
  - `Makefile` with 15 targets (help, dev, compose-up, compose-down, compose-logs, build, test, test-go, test-go-integration, test-py, test-node, lint, fmt, clean, install-py, tidy).
  - `LICENSE` (Apache-2.0) and `NOTICE` documenting third-party dependencies and the SearXNG AGPL service-boundary relationship.
  - `scripts/check-env-example.sh` and `scripts/makefile_test.sh` verification scripts.
  - TDD RED tests under `cmd/usearch/main_test.go`, `internal/meta/module_test.go`, `deploy/compose_test.go` (integration-tagged), `services/*/tests/test_health.py`, `tests/scaffold/test_services.py`, and `web/tests/scaffold.test.ts`.

### Changed

- `.moai/project/tech.md` §2 Language Matrix: Python baseline refined from 3.12+ to 3.11+ (upstream constraint from gpt-researcher and knowledge-storm).
- `.moai/project/tech.md` §7 Decision Log: appended seven 2026-04-24 decisions from the SPEC-BOOT-001 annotation cycle covering module path, Python baseline, task queue backing, Node LTS, Python package manager, Node package manager, and service image pins.

### Notes

- The scaffold is intentionally minimal — `internal/` packages are stubs with zero runtime statements. Content lands in later SPECs (see `.moai/project/roadmap.md`). Tests today cover only the scaffold surface (`--version` flag, `/health` endpoints, config validity, existence assertions).
- Local CI mirror verified on macOS with Go 1.26.2 (forward-compatible with `go 1.23` directive): `go vet`, `go test -race`, `go build`, `uv run pytest` (9/9 pass), `pnpm typecheck/lint/test/build` (all pass), `docker compose config` (exit 0).
- GitHub Actions workflows are present but not yet validated against a live CI run. First push to `main` will exercise them.

[Unreleased]: https://github.com/elymas/universal-search/compare/v0.0.1-dev...HEAD
[0.0.1-dev]: https://github.com/elymas/universal-search/releases/tag/v0.0.1-dev
