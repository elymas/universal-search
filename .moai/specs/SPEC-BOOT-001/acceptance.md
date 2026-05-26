# SPEC-BOOT-001 Acceptance — Given/When/Then Scenarios

Created: 2026-04-24
Updated: 2026-04-28 (post-implementation reconciliation)
Author: limbowl (via manager-spec)
Status: implemented

## 0. Document Purpose

This document specifies the acceptance criteria for SPEC-BOOT-001 in
Given/When/Then format, complementing the requirement-level acceptance
table in spec.md §5. Each scenario is mapped to one or more EARS
requirements. The scenarios below describe externally observable
behaviours that must hold for the M1 Foundation milestone to be
considered exited.

## 1. Coverage Matrix

| AC | Scenario | REQs covered |
|----|----------|--------------|
| AC-001 | Go module compiles and verifies | REQ-BOOT-001 |
| AC-002 | Three Python sidecar services importable + uv-managed | REQ-BOOT-002 |
| AC-003 | Next.js 16 web scaffold typechecks, lints, builds | REQ-BOOT-003 |
| AC-004 | `make compose-up` brings the 6-service stack healthy | REQ-BOOT-004, REQ-BOOT-005 |
| AC-005 | Compose file declares no hardcoded credentials | REQ-BOOT-005 |
| AC-006 | `.env.example` documents every interpolated `${VAR}` | REQ-BOOT-006 |
| AC-007 | All four CI workflows pass on a fresh `main` push | REQ-BOOT-007 |
| AC-008 | Pre-commit framework passes on the scaffold | REQ-BOOT-008 |
| AC-009 | `.editorconfig` declares per-language indent rules | REQ-BOOT-009 |
| AC-010 | `Makefile` exposes the 10 required targets | REQ-BOOT-010 |
| AC-011 | `README.md` quickstart works end-to-end | REQ-BOOT-011 |
| AC-012 | `usearch --version` prints semver and exits 0 | REQ-BOOT-012 |
| AC-013 | Reproducibility — lockfiles committed | NFR-BOOT-001 |
| AC-014 | Local-first — `make dev` succeeds without cloud keys | NFR-BOOT-002 |
| AC-015 | Airgap-friendly — no runtime egress | NFR-BOOT-003 |
| AC-016 | License compliance — Apache-2.0 + SearXNG service-boundary | NFR-BOOT-004 |

## 2. Definition of Done

- [x] `make compose-up` starts all 6 compose services healthy within 60s.
- [x] `make build` produces `cmd/usearch/usearch`.
- [x] `./cmd/usearch/usearch --version` exits 0 with semver output.
- [x] All four CI workflows green on `main`.
- [x] `pre-commit run --all-files` exits 0 on the scaffold.
- [x] `docs/` quickstart links resolve to populated documents.
- [x] All four lockfiles (`go.sum`, `uv.lock`, `pnpm-lock.yaml`, plus any
      compose digest pin once SPEC-DEP-001 lands) committed.
- [x] `LICENSE` is Apache-2.0; `NOTICE` enumerates SearXNG AGPL
      service-boundary clause.
- [x] No `.env` file tracked in git; `.env.example` is the only template.

## 3. Functional Scenarios

### AC-001 — Go module compiles and verifies

Maps to REQ-BOOT-001.

**Given** the repository at a clean checkout.

**When** the engineer runs `go mod verify && go build ./... && go vet ./...`.

**Then** each command exits 0; `go.mod` declares
`module github.com/elymas/universal-search` and `go 1.25`; `go.sum` is
present; `cmd/`, `internal/`, and `pkg/` each contain at least one
`.go` file.

**Verification**: existing `cmd/usearch/main_test.go` + `go vet` CI step
in `.github/workflows/go.yml`.

### AC-002 — Python sidecar services

Maps to REQ-BOOT-002.

**Given** the `services/` directory exists.

**When** the engineer runs `uv sync --frozen --package researcher` (and
similarly for `storm`, `embedder`).

**Then** each service venv resolves; `python -c "import researcher"` (and
the others) succeeds; each `pyproject.toml` declares
`requires-python = ">=3.11"`; each service's `Dockerfile` passes
`hadolint`.

### AC-003 — Next.js web scaffold

Maps to REQ-BOOT-003.

**Given** the `web/` directory.

**When** the engineer runs
`pnpm -C web install --frozen-lockfile && pnpm -C web typecheck && pnpm -C web lint && pnpm -C web build`.

**Then** each step exits 0; `web/package.json` declares Next.js 16.x and
a shadcn/ui marker (`components.json`); `web/app/`, `components/`, `lib/`
directories exist.

### AC-004 — Compose stack boots healthy

Maps to REQ-BOOT-004 + REQ-BOOT-005.

**Given** a fresh `.env` copied from `.env.example` with placeholder
values (any non-empty `MEILI_MASTER_KEY`, `POSTGRES_PASSWORD`,
`SEARXNG_SECRET`, `LITELLM_MASTER_KEY`).

**When** the engineer runs `make compose-up`.

**Then** within 60 s, `docker compose ps` reports every service as
`healthy`; the following probes succeed:
- `curl -sf http://localhost:6333/readyz` (Qdrant) returns 200
- `curl -sf http://localhost:7700/health` (Meilisearch) returns 200
- `pg_isready -h localhost -p 5432` (PostgreSQL) exits 0
- `curl -sf http://localhost:8080/` (SearXNG) returns 200
- `curl -sf http://localhost:4000/health` (LiteLLM) returns 200
- `redis-cli -h localhost ping` returns `PONG`

**Verification**: `.github/workflows/compose-check.yml`.

### AC-005 — No hardcoded credentials in compose

Maps to REQ-BOOT-005.

**Given** the compose file at `deploy/docker-compose.yml`.

**When** `grep -E "(password|secret|token).*:.*['\"][^$]"
deploy/docker-compose.yml` runs.

**Then** zero matches are returned (every credential is sourced from
`${VAR}` env interpolation).

**And** every service stanza contains a `healthcheck` block with
`test`, `interval`, `timeout`, `retries`.

### AC-006 — `.env.example` completeness

Maps to REQ-BOOT-006.

**Given** the compose file and per-service source code.

**When** `scripts/check-env-example.sh` runs.

**Then** every `${VAR}` referenced in compose has a matching key in
`.env.example`; every env var consumed by service source code has the
corresponding `services/{name}/.env.example` entry.

**And** `.env` is in `.gitignore`; no `.env` file is committed.

### AC-007 — CI green on fresh main push

Maps to REQ-BOOT-007.

**Given** a fresh push or PR targeting `main`.

**When** GitHub Actions schedules the workflow set.

**Then** all four workflows complete with green status:
- `go.yml` — Go 1.25.x; `go vet → golangci-lint → go test`
- `python.yml` — Python 3.11; per-service ruff + pytest
- `web.yml` — Node 22 LTS; typecheck + lint + build
- `compose-check.yml` — `compose up --wait` within 90 s

**And** all jobs run in parallel; required checks block merge until
green.

### AC-008 — Pre-commit framework

Maps to REQ-BOOT-008.

**Given** the scaffold at HEAD.

**When** the engineer runs `pre-commit run --all-files`.

**Then** every declared hook (`gofmt`, `goimports`, `ruff`, `prettier`,
`eslint`, `trailing-whitespace`, `end-of-file-fixer`, `hadolint`,
`shellcheck`, `yamllint`) executes and exits 0.

**And** `.github/workflows/pre-commit.yml` runs the same command in CI.

### AC-009 — `.editorconfig` per-language indent

Maps to REQ-BOOT-009.

**Given** the `.editorconfig` at repo root.

**When** parsed by `editorconfig-checker`.

**Then** the following sections each declare the required indent rules:
- `[*.go]` → `indent_style = tab`, `indent_size = 4`
- `[*.py]` → `indent_style = space`, `indent_size = 4`
- `[*.{ts,tsx,js,jsx}]` → `indent_style = space`, `indent_size = 2`
- `[*.{yml,yaml}]` → `indent_style = space`, `indent_size = 2`
- `[*.md]` → `indent_style = space`, `indent_size = 2`

### AC-010 — Makefile targets

Maps to REQ-BOOT-010.

**Given** the `Makefile` at repo root.

**When** the engineer runs `make help`.

**Then** the help output lists all ten required targets:
`dev`, `test`, `lint`, `build`, `clean`, `compose-up`, `compose-down`,
`fmt`, `tidy`, `install-py`.

**And** each target is implementable on both macOS and Linux without
bash-only constructs in recipes.

### AC-011 — README quickstart

Maps to REQ-BOOT-011.

**Given** `README.md` at repo root.

**When** a new contributor follows the Quickstart section:
1. clone the repo
2. `cp .env.example .env`
3. `make compose-up`
4. `make build`
5. `./cmd/usearch/usearch --version`

**Then** each step succeeds in order; the final command prints a semver
string and exits 0.

**And** README contains a Prerequisites section listing Docker, Go 1.25+,
Python 3.11+, Node 22+, make; and four relative links to
`.moai/project/{product,structure,tech,roadmap}.md`.

### AC-012 — `usearch --version`

Maps to REQ-BOOT-012.

**Given** the binary at `cmd/usearch/usearch` after `make build`.

**When** the engineer runs `./cmd/usearch/usearch --version`.

**Then** stdout matches the regex `^usearch v[0-9]+\.[0-9]+\.[0-9]+`
(currently `usearch v0.1.0-dev\n`); the exit code is 0; nothing is
written to stderr.

**Also**: `./cmd/usearch/usearch -v` produces identical output (short
form).

**Verification test**: `cmd/usearch/main_test.go::TestVersionFlag` and
`TestVersionShortFlag`.

## 4. Non-Functional Acceptance

### NFR-BOOT-001 — Reproducibility

- All lockfiles committed: `go.sum`, `uv.lock`, `pnpm-lock.yaml`.
- Compose image tags pinned to exact versions (SearXNG digest deferred
  to SPEC-DEP-001).
- A fresh clone followed by `make compose-up && make build` produces a
  byte-identical binary on the same OS+arch.

### NFR-BOOT-002 — Local-first

- `make dev` runs on a freshly cloned repo with blank `.env` keys.
- Services that require API keys (LiteLLM external providers, gpt-
  researcher's TAVILY/OpenAI) start healthy with empty keys; they are
  not exercised by `make dev`.

### NFR-BOOT-003 — Airgap-friendly

- No runtime egress beyond standard package registries (Docker Hub,
  GHCR, PyPI, npm).
- No telemetry phone-home in any service.

### NFR-BOOT-004 — License compliance

- Root `LICENSE` is verbatim Apache-2.0.
- Root `NOTICE` enumerates Apache-2.0 attributions and the SearXNG
  AGPL-3.0 service-boundary note.
- `README.md` documents the SearXNG service-boundary relationship.

## 5. Edge Cases

### EC-001 — Missing `.env` file

- **When** the engineer runs `make compose-up` without `.env`.
- **Then** the command fails fast with a clear error pointing at
  `.env.example`; no service is started.

### EC-002 — Compose port collision

- **When** another process already binds `:6333` (Qdrant default port).
- **Then** `docker compose up` reports the bind failure for Qdrant; the
  service becomes unhealthy; `make compose-up --wait` exits non-zero
  after the timeout.

### EC-003 — Outdated lockfile

- **When** a developer modifies `go.mod` without running `go mod tidy`.
- **Then** the pre-commit `go-mod-tidy` hook fails the commit.

### EC-004 — SearXNG `:latest` drift

- **Documented behaviour** until SPEC-DEP-001: `:latest` may drift
  between `compose pull` cycles; this is the cost paid to defer the
  digest pinning decision; reproducibility is recoverable by re-running
  `compose pull` from a known-good machine.

### EC-005 — Pre-commit rev pins unresolved

- **When** the engineer runs `pre-commit run --all-files` immediately
  after a fresh clone without first running `pre-commit autoupdate`.
- **Then** the hooks run with the committed rev pins; no `rev: ""`
  placeholders remain (the first commit of BOOT-001 ran autoupdate to
  resolve them).

### EC-006 — macOS vs Linux Makefile

- **Given** a contributor switching between macOS and Ubuntu.
- **When** running any `make` target.
- **Then** the target succeeds without modification on either platform
  (recipes use POSIX-portable constructs).

### EC-007 — Directory typo (`univesal-search`)

- **Documented limitation**: the working directory may be named with a
  typo; the Go module path `github.com/elymas/universal-search` is
  independent of the filesystem directory name; `go build` works
  unaffected. Rename is deferred to the repo-creation owner.

## 6. Quality Gate Criteria

| Criterion | Threshold | Source |
|-----------|-----------|--------|
| `go vet ./...` | clean | REQ-BOOT-001 |
| `go build ./...` | clean | REQ-BOOT-001 |
| `golangci-lint run` | clean | quality.yaml |
| Compose `up --wait` | success within 90 s (CI) | REQ-BOOT-007 |
| `pre-commit run --all-files` | clean | REQ-BOOT-008 |
| `pnpm -C web typecheck/lint/build` | clean | REQ-BOOT-003 |
| `hadolint services/*/Dockerfile` | clean | REQ-BOOT-002 |
| `cmd/usearch/usearch --version` exit code | 0 | REQ-BOOT-012 |
| `cmd/usearch/usearch --version` stdout | matches `^usearch v\d+\.\d+\.\d+` | REQ-BOOT-012 |

## 7. Out-of-Scope Confirmations

The following are NOT acceptance criteria for SPEC-BOOT-001 (restated
from spec.md §2.2 for the post-implementation reviewer):

- Search adapters → SPEC-ADP-001+
- Authentication, RBAC, vault → M2/M6 SPECs
- LLM client implementation → SPEC-LLM-001
- Index schemas + ingestion → SPEC-IDX-001+
- Intent Router, MCP, fanout → SPEC-IR-001, SPEC-MCP-001, SPEC-FAN-001
- Observability wiring → SPEC-OBS-001
- Helm/Kubernetes manifests → M8+
- Playwright e2e suite → SPEC-E2E-001 (M5)
- CLI subcommands beyond `--version` → SPEC-CLI-001
- UI pages beyond default Next.js scaffold → SPEC-UI-001
- Evaluation harness → SPEC-EVAL-001
- SearXNG fork or patching → out of scope permanently (consumed as-is)

---

*End of acceptance.md (post-hoc).*
