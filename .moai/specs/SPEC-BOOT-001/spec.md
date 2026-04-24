---
id: SPEC-BOOT-001
title: Repo Scaffold and CI Bootstrap
milestone: M1 — Foundation
status: approved
priority: P0
owner: expert-devops
methodology: tdd
coverage_target: 85
created: 2026-04-24
updated: 2026-04-24
approved_by: limbowl
approved_at: 2026-04-24
depends_on: []
blocks: [SPEC-DEP-001, SPEC-OBS-001, SPEC-LLM-001, SPEC-IR-001, SPEC-ADP-001, SPEC-CLI-001, SPEC-IDX-001, SPEC-UI-001]
---

# SPEC-BOOT-001 — Repo Scaffold and CI Bootstrap

## 1. Purpose

This SPEC establishes the greenfield foundation for the Universal Search monorepo: a Go module for core orchestration, three Python services (gpt-researcher, STORM, embedder) managed as a `uv` workspace, a Next.js 16 web frontend, a locally runnable `docker-compose` stack covering all runtime dependencies (Qdrant, Meilisearch, PostgreSQL, SearXNG, LiteLLM, Redis), plus CI workflows, pre-commit hooks, and a consolidated `Makefile`. It is the first SPEC of Milestone M1 (Foundation) and its completion satisfies the M1 exit criterion defined in `.moai/project/roadmap.md` §5: `docker compose up` starts all dependencies, `usearch --version` runs, and CI is green. Every subsequent M1-M9 SPEC (SPEC-DEP-001, SPEC-OBS-001, SPEC-LLM-001, etc.) depends on this scaffold existing; without it, no downstream work is unblockable.

## 2. Scope

### In-Scope

- Go module `github.com/elymas/universal-search` with `cmd/`, `internal/`, `pkg/`, `proto/` roots
- Three empty-but-valid Python services under `services/{researcher,storm,embedder}/` managed as a uv workspace
- Next.js 16 App Router scaffold under `web/` with shadcn/ui, Tailwind, ESLint, Prettier
- `deploy/docker-compose.yml` with six services (Qdrant, Meili, PG, SearXNG, LiteLLM, Redis) all with healthchecks
- Root `.env.example` and per-service `.env.example` files
- GitHub Actions CI (Go, Python, Web, compose-check, pre-commit)
- Pre-commit framework configuration
- `.editorconfig`, `Makefile`, `LICENSE` (Apache-2.0), `NOTICE`, `README.md`
- `cmd/usearch/usearch --version` binary that prints a semver string

### Out-of-Scope (deferred to later SPECs)

- Search adapters (Google, Bing, Naver, Daum, DuckDuckGo, SearXNG integration) — SPEC-ADP-001+
- Authentication, RBAC, personal-context vault — later M2/M6 SPECs
- LLM invocation logic (LiteLLM compose entry is present; client code is not) — SPEC-LLM-001
- Vector/keyword index schemas (containers run; no data ingested) — SPEC-IDX-001
- Intent Router, MCP handlers, fan-out orchestration — SPEC-IR-001
- Observability wiring (internal/obs stub exists; no collectors wired) — SPEC-OBS-001
- Helm charts, Kubernetes manifests, production deployment — M8 onwards
- Playwright end-to-end test suite — M5 SPEC-E2E-001
- Any CLI subcommand beyond `--version` — SPEC-CLI-001
- Any UI pages beyond the default Next.js scaffold — SPEC-UI-001
- Evaluation harness, Ragas, KILT benchmarks — SPEC-EVAL-001
- SearXNG fork or patching; `searxng/searxng` is consumed as-is (service boundary, see NFR-BOOT-004)

## 3. EARS Requirements

| ID | Category | EARS Statement | Priority |
|----|----------|----------------|----------|
| REQ-BOOT-001 | Ubiquitous | The repository shall contain a valid Go module rooted at `go.mod` with module path `github.com/elymas/universal-search`, Go 1.23, and three package roots (`cmd/`, `internal/`, `pkg/`). | P0 |
| REQ-BOOT-002 | Ubiquitous | The repository shall contain `services/researcher/`, `services/storm/`, `services/embedder/`, each with `__init__.py`, `pyproject.toml`, `Dockerfile`, and `README.md`, each declaring `python = ">=3.11"`. | P0 |
| REQ-BOOT-003 | Ubiquitous | The repository shall contain a Next.js 16 App Router scaffold under `web/` configured with shadcn/ui, Tailwind CSS, ESLint, and Prettier. | P0 |
| REQ-BOOT-004 | Event-driven | When a developer runs `make compose-up`, the docker-compose stack shall start Qdrant v1.16.3, Meilisearch v1.42.1, PostgreSQL 16.13-alpine3.23, SearXNG (`searxng/searxng`), LiteLLM v1.83.7-stable.patch.1, and Redis 7-alpine, with every service reporting healthy within 60 seconds. | P0 |
| REQ-BOOT-005 | Ubiquitous | The `docker-compose.yml` shall use named volumes for stateful services, declare a healthcheck on every service, use `${VAR}` env interpolation for all configurable values, and contain no hardcoded credentials. | P0 |
| REQ-BOOT-006 | Ubiquitous | The repository shall contain `.env.example` at the root and `services/{researcher,storm,embedder}/.env.example` files documenting every environment variable consumed by compose or by service code. | P0 |
| REQ-BOOT-007 | Ubiquitous | GitHub Actions CI shall run a Go toolchain job (test + vet + golangci-lint), a Python toolchain job (pytest + ruff) per service, a Web toolchain job (typecheck + eslint + build), and a compose-up matrix check on every push and pull request targeting `main`, using Node 22 LTS. | P0 |
| REQ-BOOT-008 | Ubiquitous | A pre-commit framework shall configure hooks for `gofmt`, `goimports`, `ruff`, `prettier`, `eslint`, `trailing-whitespace`, `end-of-file-fixer`, `hadolint`, `shellcheck`, and `yamllint`. | P1 |
| REQ-BOOT-009 | Ubiquitous | The repository shall contain an `.editorconfig` with language-specific indent rules (Go: tabs, width 4; Python: spaces, width 4; TS/JS: spaces, width 2; YAML: spaces, width 2; Markdown: spaces, width 2). | P1 |
| REQ-BOOT-010 | Ubiquitous | The `Makefile` shall expose targets `dev`, `test`, `lint`, `build`, `clean`, `compose-up`, `compose-down`, `fmt`, `tidy`, and `install-py` for the uv workspace. | P0 |
| REQ-BOOT-011 | Ubiquitous | `README.md` shall provide a quickstart, list prerequisites (Docker, Go 1.23+, Python 3.11+, Node 22+, make), and link to `.moai/project/` documentation. | P1 |
| REQ-BOOT-012 | Event-driven | When a developer runs `./cmd/usearch/usearch --version` after `make build`, the binary shall print a semver string and exit with code 0. | P0 |

## 4. Non-Functional Requirements

| ID | Category | Statement |
|----|----------|-----------|
| NFR-BOOT-001 | Reproducibility | All runtime dependencies shall be pinned to exact versions; lockfiles (`go.sum`, `uv.lock`, `pnpm-lock.yaml`) shall be committed. |
| NFR-BOOT-002 | Local-first | `make dev` shall complete successfully on a freshly cloned repository without requiring any cloud credentials (API keys may remain blank in `.env`; services that require them are not exercised in `make dev`). |
| NFR-BOOT-003 | Airgap-friendly | The scaffold shall make no runtime egress calls beyond standard package registries (Docker Hub, GitHub Container Registry, PyPI, npm). |
| NFR-BOOT-004 | License compliance | `README.md` shall document the SearXNG AGPL-3.0 service-boundary relationship; the root `LICENSE` file shall be Apache-2.0; a `NOTICE` file shall enumerate Apache-2.0 attributions and the SearXNG service-boundary note. |

## 5. Acceptance Criteria

### M1 Exit Checkpoint (top-level)

- [ ] `make compose-up` starts all six compose services; `docker compose ps` reports every service as `healthy` within 60 seconds.
- [ ] `make build` produces `cmd/usearch/usearch`.
- [ ] `./cmd/usearch/usearch --version` exits 0 and prints a semver string matching the pattern `usearch v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.-]+)?`.
- [ ] All four CI workflows (Go, Python, Web, compose-check) pass on a fresh `main` push.

### REQ-BOOT-001 (Go module)

- [ ] `go mod verify` exits 0.
- [ ] `go.mod` declares `module github.com/elymas/universal-search` and `go 1.23`.
- [ ] `go.sum` is present and committed.
- [ ] `cmd/usearch/`, `internal/`, and `pkg/` each contain at least one `.go` file.
- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.

### REQ-BOOT-002 (Python services)

- [ ] `services/researcher/`, `services/storm/`, `services/embedder/` exist.
- [ ] Each contains `__init__.py`, `pyproject.toml`, `Dockerfile`, `README.md`.
- [ ] Each `pyproject.toml` declares `requires-python = ">=3.11"` (or equivalent in `[project]` block).
- [ ] `hadolint services/*/Dockerfile` exits 0.
- [ ] In each service venv, `python -c "import {service_name}"` succeeds.

### REQ-BOOT-003 (Next.js web)

- [ ] `web/` contains `app/`, `components/`, `lib/` directories.
- [ ] `npx next --version` reports a 16.x version from `web/`.
- [ ] `web/package.json` declares shadcn/ui (either as a dependency or via a `components.json` artifact).
- [ ] `pnpm -C web typecheck` exits 0.
- [ ] `pnpm -C web lint` exits 0.
- [ ] `pnpm -C web build` exits 0.

### REQ-BOOT-004 (Compose stack starts)

- [ ] `make compose-up` returns within 60 seconds with all services reporting `healthy`.
- [ ] `curl -sf http://localhost:6333/readyz` returns 200 (Qdrant).
- [ ] `curl -sf http://localhost:7700/health` returns 200 (Meilisearch).
- [ ] `pg_isready -h localhost -p 5432` exits 0 (PostgreSQL).
- [ ] `curl -sf http://localhost:8080/` returns 200 (SearXNG).
- [ ] `curl -sf http://localhost:4000/health` returns 200 (LiteLLM).
- [ ] `redis-cli -h localhost ping` returns `PONG` (Redis).

### REQ-BOOT-005 (Compose structural quality)

- [ ] `docker compose config` exits 0 against `deploy/docker-compose.yml`.
- [ ] Qdrant, Meili, PG, Redis services each declare at least one named volume.
- [ ] Every service stanza contains a `healthcheck` block with `test`, `interval`, `timeout`, `retries`.
- [ ] `grep -E "(password|secret|token).*:.*['\"][^$]" deploy/docker-compose.yml` returns no hardcoded credentials.
- [ ] Every `${VAR}` referenced in compose has a matching entry in `.env.example` with a safe default.

### REQ-BOOT-006 (Env example documentation)

- [ ] `.env.example` exists at repo root.
- [ ] `services/researcher/.env.example`, `services/storm/.env.example`, `services/embedder/.env.example` exist.
- [ ] A diff-script check (`scripts/check-env-example.sh` or equivalent) confirms no `${VAR}` appears in any compose or service source file without a corresponding `.env.example` entry.
- [ ] `.env` is in `.gitignore`.
- [ ] `.env.example` is committed; no `.env` file is tracked.

### REQ-BOOT-007 (CI green)

- [ ] `.github/workflows/go.yml`, `python.yml`, `web.yml`, `compose-check.yml` exist.
- [ ] All four jobs green on a fresh checkout of `main`.
- [ ] `compose-check.yml` invokes `docker compose up --wait` and completes within 90 seconds.
- [ ] `web.yml` uses Node 22 LTS.
- [ ] `go.yml` uses Go 1.23.x.
- [ ] `python.yml` uses Python 3.11.

### REQ-BOOT-008 (Pre-commit)

- [ ] `.pre-commit-config.yaml` exists at repo root.
- [ ] `pre-commit run --all-files` exits 0 on the scaffold.
- [ ] Every listed hook (`gofmt`, `goimports`, `ruff`, `prettier`, `eslint`, `trailing-whitespace`, `end-of-file-fixer`, `hadolint`, `shellcheck`, `yamllint`) is declared.
- [ ] A CI job runs `pre-commit run --all-files`.

### REQ-BOOT-009 (editorconfig)

- [ ] `.editorconfig` parses cleanly under `editorconfig-checker`.
- [ ] Go section: `indent_style = tab`, `indent_size = 4`.
- [ ] Python section: `indent_style = space`, `indent_size = 4`.
- [ ] TS/JS section: `indent_style = space`, `indent_size = 2`.
- [ ] YAML section: `indent_style = space`, `indent_size = 2`.
- [ ] Markdown section: `indent_style = space`, `indent_size = 2`.

### REQ-BOOT-010 (Makefile)

- [ ] `make help` lists all required targets.
- [ ] `make tidy` invokes `go mod tidy`.
- [ ] `make fmt` invokes `gofmt -s -w .` and `ruff format services/`.
- [ ] `make compose-up` and `make compose-down` wrap `docker compose -f deploy/docker-compose.yml {up -d --wait,down}`.
- [ ] `make test` runs Go + Python + Node test suites.
- [ ] `make build` produces `cmd/usearch/usearch`.
- [ ] `make install-py` runs `uv sync` at the workspace root.
- [ ] Makefile operates on macOS and Linux without `bash`-only constructs in recipes.

### REQ-BOOT-011 (README)

- [ ] `README.md` is non-empty at the repo root.
- [ ] Contains a `## Quickstart` section with clone → `cp .env.example .env` → `make compose-up` → `make build` → `./cmd/usearch/usearch --version`.
- [ ] Prerequisites section lists Docker, Go 1.23+, Python 3.11+, Node 22+, make.
- [ ] Contains four relative links: `.moai/project/product.md`, `.moai/project/structure.md`, `.moai/project/tech.md`, `.moai/project/roadmap.md`.

### REQ-BOOT-012 (usearch --version)

- [ ] `make build` produces `cmd/usearch/usearch` (Unix) or `cmd/usearch/usearch.exe` (Windows, CI).
- [ ] `./cmd/usearch/usearch --version` exits 0.
- [ ] Stdout matches `^usearch v[0-9]+\.[0-9]+\.[0-9]+`.

## 6. Technical Approach

### 6.1 Directory Layout

```
universal-search/
├── cmd/
│   ├── usearch/            # main CLI entrypoint; main.go <50 lines, --version stub
│   │   ├── main.go
│   │   └── main_test.go
│   ├── usearch-mcp/        # MCP server entrypoint (stub in SPEC-BOOT-001)
│   │   └── main.go
│   └── usearch-api/        # REST/gRPC API entrypoint (stub in SPEC-BOOT-001)
│       └── main.go
├── internal/               # domain-organized packages (stubs; content lands in later SPECs)
│   ├── router/             # intent router (SPEC-IR-001)
│   ├── fanout/             # fan-out orchestrator (SPEC-IR-001)
│   ├── adapters/           # search adapters (SPEC-ADP-001+)
│   ├── index/              # vector/keyword indexes (SPEC-IDX-001)
│   ├── llm/                # LiteLLM client (SPEC-LLM-001)
│   ├── synthesis/          # STORM/gpt-researcher integration (M4)
│   ├── auth/               # auth + RBAC (M6)
│   ├── obs/                # observability (SPEC-OBS-001)
│   └── eval/               # evaluation harness (SPEC-EVAL-001)
├── pkg/                    # external consumers only
│   ├── client/             # public Go client
│   └── types/              # shared public types
├── proto/                  # gRPC contracts (one .proto per service, default; revisit in SPEC-IR-001)
├── services/               # Python sidecar services (uv workspace)
│   ├── researcher/         # gpt-researcher
│   │   ├── src/researcher/__init__.py
│   │   ├── src/researcher/main.py  # FastAPI + GET /health
│   │   ├── pyproject.toml
│   │   ├── Dockerfile
│   │   ├── README.md
│   │   └── .env.example
│   ├── storm/              # STORM (knowledge-storm)
│   │   └── ... (same layout)
│   └── embedder/           # embedding service
│       └── ... (same layout)
├── web/                    # Next.js 16 App Router
│   ├── app/
│   ├── components/
│   ├── lib/
│   ├── package.json
│   ├── tsconfig.json
│   ├── tailwind.config.ts
│   ├── next.config.ts
│   └── components.json     # shadcn/ui manifest
├── deploy/
│   ├── docker-compose.yml
│   ├── .env.example        # deploy-specific overrides (optional)
│   └── searxng/
│       └── settings.yml    # local SearXNG config (mounted at /etc/searxng)
├── .github/
│   └── workflows/
│       ├── go.yml
│       ├── python.yml
│       ├── web.yml
│       ├── compose-check.yml
│       └── pre-commit.yml
├── scripts/
│   └── check-env-example.sh
├── .moai/                  # (pre-existing; not touched by this SPEC)
├── .env.example            # root: covers compose + global API keys
├── .editorconfig
├── .gitignore
├── .pre-commit-config.yaml
├── go.mod                  # module github.com/elymas/universal-search
├── go.sum
├── pyproject.toml          # uv workspace root
├── uv.lock
├── pnpm-workspace.yaml     # includes web/
├── LICENSE                 # Apache-2.0
├── NOTICE
├── Makefile
└── README.md
```

### 6.2 docker-compose.yml Skeleton (deploy/docker-compose.yml)

```yaml
services:
  qdrant:
    image: qdrant/qdrant:v1.16.3
    ports:
      - "${QDRANT_HTTP_PORT:-6333}:6333"
      - "${QDRANT_GRPC_PORT:-6334}:6334"
    volumes:
      - qdrant_data:/qdrant/storage
    healthcheck:
      test: ["CMD-SHELL", "wget -qO- http://localhost:6333/readyz || exit 1"]
      interval: 10s
      timeout: 3s
      retries: 5
    restart: unless-stopped

  meilisearch:
    image: getmeili/meilisearch:v1.42.1
    ports:
      - "${MEILI_PORT:-7700}:7700"
    environment:
      MEILI_MASTER_KEY: ${MEILI_MASTER_KEY}
      MEILI_ENV: ${MEILI_ENV:-development}
    volumes:
      - meili_data:/meili_data
    healthcheck:
      test: ["CMD-SHELL", "wget -qO- http://localhost:7700/health || exit 1"]
      interval: 10s
      timeout: 3s
      retries: 5
    restart: unless-stopped

  postgres:
    image: postgres:16.13-alpine3.23
    ports:
      - "${POSTGRES_PORT:-5432}:5432"
    environment:
      POSTGRES_USER: ${POSTGRES_USER:-usearch}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
      POSTGRES_DB: ${POSTGRES_DB:-usearch}
    volumes:
      - pg_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER:-usearch} -d ${POSTGRES_DB:-usearch}"]
      interval: 10s
      timeout: 3s
      retries: 5
    restart: unless-stopped

  redis:
    image: redis:7-alpine
    ports:
      - "${REDIS_PORT:-6379}:6379"
    volumes:
      - redis_data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 3s
      retries: 5
    restart: unless-stopped

  searxng:
    image: searxng/searxng:latest
    ports:
      - "${SEARXNG_PORT:-8080}:8080"
    volumes:
      - ./searxng/settings.yml:/etc/searxng/settings.yml:ro
    environment:
      SEARXNG_BASE_URL: ${SEARXNG_BASE_URL:-http://localhost:8080/}
      SEARXNG_SECRET: ${SEARXNG_SECRET}
    depends_on:
      redis:
        condition: service_healthy
    healthcheck:
      test: ["CMD-SHELL", "wget -qO- http://localhost:8080/ || exit 1"]
      interval: 15s
      timeout: 5s
      retries: 5
    restart: unless-stopped

  litellm:
    image: ghcr.io/berriai/litellm:v1.83.7-stable.patch.1
    ports:
      - "${LITELLM_PORT:-4000}:4000"
    environment:
      LITELLM_MASTER_KEY: ${LITELLM_MASTER_KEY}
      OPENAI_API_KEY: ${OPENAI_API_KEY:-}
      ANTHROPIC_API_KEY: ${ANTHROPIC_API_KEY:-}
    depends_on:
      redis:
        condition: service_healthy
    healthcheck:
      test: ["CMD-SHELL", "wget -qO- http://localhost:4000/health || exit 1"]
      interval: 15s
      timeout: 5s
      retries: 5
    restart: unless-stopped

volumes:
  qdrant_data:
  meili_data:
  pg_data:
  redis_data:
```

### 6.3 CI Workflow Skeleton

Four workflow files under `.github/workflows/`:

- **go.yml** — triggers on push/PR to `main`; matrix single Go 1.23.x; steps: `actions/checkout@v4` → `actions/setup-go@v5` (with built-in cache) → `go mod download` → `go vet ./...` → `golangci-lint run` → `go test -race -cover ./...`.
- **python.yml** — matrix strategy over the three services (`researcher`, `storm`, `embedder`); Python 3.11; `actions/setup-python@v5` → `pip install uv` → `uv sync --frozen --package {service}` → `ruff check services/{service}` → `uv run --package {service} pytest`.
- **web.yml** — Node 22 LTS; `actions/setup-node@v4` with pnpm cache → `pnpm -C web install --frozen-lockfile` → `pnpm -C web typecheck` → `pnpm -C web lint` → `pnpm -C web build`.
- **compose-check.yml** — ubuntu-latest; `docker compose -f deploy/docker-compose.yml config` → `docker compose up --wait --quiet-pull` (timeout 90s) → `docker compose ps` → `docker compose down -v`.
- **pre-commit.yml** — runs `pre-commit run --all-files` on every push.

All jobs run in parallel; required checks on `main` branch protection rule.

### 6.4 Pre-commit Hook Order

Declared in `.pre-commit-config.yaml` in this order (earlier hooks fail fast on mechanical issues):

1. `pre-commit/pre-commit-hooks`: `trailing-whitespace`, `end-of-file-fixer`, `check-yaml`, `check-added-large-files` (`maxkb=1000`)
2. `dnephin/pre-commit-golang`: `go-fmt`, `go-vet`
3. `astral-sh/ruff-pre-commit`: `ruff`, `ruff-format`
4. `pre-commit/mirrors-prettier` (web + yaml/md)
5. `hadolint/hadolint` pre-commit wrapper (Dockerfile linting)
6. `shellcheck-py/shellcheck-py`
7. `adrienverge/yamllint`

Rev pins will be populated by running `pre-commit autoupdate` once on first commit (noted in Open Questions).

### 6.5 Makefile Target DAG

```
help           : prints target list (default target)
install-py     : uv sync (workspace root)
tidy           : go mod tidy
fmt            : gofmt -s -w . ; ruff format services/ ; pnpm -C web prettier --write .
lint           : go vet ./... ; golangci-lint run ; ruff check services/ ; pnpm -C web lint
build          : go build -o cmd/usearch/usearch ./cmd/usearch
test-go        : go test -race -cover ./...
test-go-integration : go test -race -tags=integration ./...
test-py        : uv run pytest services/
test-node      : pnpm -C web test
test           : test-go + test-py + test-node  (sequential locally; parallel in CI via -j3)
compose-up     : docker compose -f deploy/docker-compose.yml up -d --wait
compose-down   : docker compose -f deploy/docker-compose.yml down
compose-logs   : docker compose -f deploy/docker-compose.yml logs -f
dev            : compose-up ; go run ./cmd/usearch
clean          : rm -rf cmd/usearch/usearch dist/ node_modules/ .venv/
```

Dependencies: `dev` requires `compose-up`; `test` fan-outs to `test-go`, `test-py`, `test-node`; `build` is standalone.

### 6.6 License and NOTICE Content Outline

**LICENSE** — verbatim Apache-2.0 license text.

**NOTICE** — structured as:

```
Universal Search
Copyright (c) 2026 elymas

This product includes software developed by the Universal Search contributors
under the Apache License, Version 2.0.

---

Third-Party Services (run as external processes; not bundled or statically linked):

- SearXNG (AGPL-3.0): consumed as a Docker image via docker-compose. Universal
  Search does not modify, fork, or redistribute SearXNG source code. Network
  interaction across a service boundary does not constitute a derivative work
  under AGPL-3.0.
- gpt-researcher (Apache-2.0): imported as a Python dependency in
  services/researcher.
- knowledge-storm (MIT): imported as a Python dependency in services/storm.
- Qdrant (Apache-2.0), Meilisearch (MIT), PostgreSQL (PostgreSQL License),
  Redis (BSD-3-Clause / SSPL — see redis:7-alpine image labels),
  LiteLLM (MIT): all consumed as Docker images.

Full third-party attribution files will be collected under docs/licenses/ as
SPECs add direct dependencies.
```

## 7. File Impact

| Path | Purpose |
|------|---------|
| `go.mod` | Go module declaration (`module github.com/elymas/universal-search`, `go 1.23`) |
| `go.sum` | Go checksum lockfile |
| `cmd/usearch/main.go` | CLI entrypoint; parses `--version`; <50 LOC |
| `cmd/usearch/main_test.go` | `TestVersionFlag` — REQ-BOOT-012 RED test |
| `cmd/usearch-mcp/main.go` | MCP server stub (exits 0; logs "not implemented") |
| `cmd/usearch-api/main.go` | API server stub (exits 0; logs "not implemented") |
| `internal/router/router.go` | Stub package (`package router`; single unexported constant) |
| `internal/fanout/fanout.go` | Stub package |
| `internal/adapters/adapters.go` | Stub package |
| `internal/index/index.go` | Stub package |
| `internal/llm/llm.go` | Stub package |
| `internal/synthesis/synthesis.go` | Stub package |
| `internal/auth/auth.go` | Stub package |
| `internal/obs/obs.go` | Stub package |
| `internal/eval/eval.go` | Stub package |
| `pkg/client/client.go` | Public client stub |
| `pkg/types/types.go` | Public types stub |
| `proto/.gitkeep` | Reserve proto/ root; actual .proto files land in later SPECs |
| `services/researcher/pyproject.toml` | uv workspace member; `requires-python = ">=3.11"`; depends on `gpt-researcher` |
| `services/researcher/src/researcher/__init__.py` | Package init |
| `services/researcher/src/researcher/main.py` | FastAPI app with `GET /health` returning `{"status":"ok","service":"researcher"}` |
| `services/researcher/tests/test_health.py` | Health endpoint test |
| `services/researcher/Dockerfile` | `FROM python:3.11-slim`; installs uv; runs FastAPI |
| `services/researcher/README.md` | Service overview + run instructions |
| `services/researcher/.env.example` | `OPENAI_API_KEY=`, `TAVILY_API_KEY=`, `OPENAI_BASE_URL=` |
| `services/storm/pyproject.toml` | uv member; depends on `knowledge-storm` |
| `services/storm/src/storm/__init__.py` | Package init |
| `services/storm/src/storm/main.py` | FastAPI `/health` |
| `services/storm/tests/test_health.py` | Health test |
| `services/storm/Dockerfile` | Python 3.11-slim |
| `services/storm/README.md` | Service overview |
| `services/storm/.env.example` | STORM-specific vars |
| `services/embedder/pyproject.toml` | uv member; depends on `sentence-transformers` or similar (placeholder) |
| `services/embedder/src/embedder/__init__.py` | Package init |
| `services/embedder/src/embedder/main.py` | FastAPI `/health` |
| `services/embedder/tests/test_health.py` | Health test |
| `services/embedder/Dockerfile` | Python 3.11-slim |
| `services/embedder/README.md` | Service overview |
| `services/embedder/.env.example` | Embedder-specific vars |
| `pyproject.toml` | uv workspace root: `[tool.uv.workspace] members = ["services/*"]` |
| `uv.lock` | uv workspace lockfile |
| `web/package.json` | Next.js 16, shadcn/ui, Tailwind, ESLint, Prettier |
| `web/pnpm-lock.yaml` | pnpm lockfile |
| `web/tsconfig.json` | TypeScript config (strict, `@/*` alias) |
| `web/next.config.ts` | Next.js 16 config |
| `web/tailwind.config.ts` | Tailwind config |
| `web/components.json` | shadcn/ui manifest |
| `web/app/layout.tsx` | Root layout (default scaffold) |
| `web/app/page.tsx` | Home page (default scaffold) |
| `web/app/globals.css` | Tailwind directives |
| `web/.eslintrc.json` | ESLint config |
| `web/.prettierrc` | Prettier config |
| `pnpm-workspace.yaml` | `packages: ["web"]` |
| `deploy/docker-compose.yml` | Six-service stack (see §6.2) |
| `deploy/searxng/settings.yml` | Minimal SearXNG configuration (secret_key sourced from env, local-only engines) |
| `.env.example` | Root env template (all compose vars + global API keys) |
| `.editorconfig` | Language-indent rules (REQ-BOOT-009) |
| `.gitignore` | Go, Python, Node, env, IDE artifacts |
| `.pre-commit-config.yaml` | Hook order per §6.4 |
| `.github/workflows/go.yml` | Go CI |
| `.github/workflows/python.yml` | Python CI (matrix over services) |
| `.github/workflows/web.yml` | Web CI (Node 22) |
| `.github/workflows/compose-check.yml` | `docker compose up --wait` gate |
| `.github/workflows/pre-commit.yml` | `pre-commit run --all-files` gate |
| `scripts/check-env-example.sh` | Greps for `${VAR}` in compose/source and verifies `.env.example` entries |
| `Makefile` | Targets per §6.5 |
| `LICENSE` | Apache-2.0 verbatim |
| `NOTICE` | Copyright + SearXNG service-boundary + future attributions |
| `README.md` | Quickstart + prereqs + links to `.moai/project/` |

## 8. TDD Plan

Development follows RED-GREEN-REFACTOR per `development_mode: tdd` in `.moai/config/sections/quality.yaml`. Below are representative RED-phase test slices; the full set is derived by the run-phase implementer from every REQ.

1. **REQ-BOOT-012 — `TestVersionFlag`** (`cmd/usearch/main_test.go`)
   - Invokes `main` with argv `["usearch", "--version"]`.
   - Asserts stdout matches regex `^usearch v\d+\.\d+\.\d+`.
   - Asserts exit code 0.
   - Fails until `cmd/usearch/main.go` implements `--version` handling.

2. **REQ-BOOT-001 — `TestModulePath`** (`internal/meta/module_test.go`)
   - Reads `go.mod` at repo root.
   - Asserts first `module` directive equals `github.com/elymas/universal-search`.
   - Asserts `go` directive equals `1.23`.
   - Fails until `go.mod` is created with the correct values.

3. **REQ-BOOT-002 — `test_services_importable`** (`tests/scaffold/test_services.py` at repo root using pytest)
   - For each service (`researcher`, `storm`, `embedder`): `importlib.import_module(svc)` must not raise.
   - Fails until each service `__init__.py` exists and is syntactically valid.

4. **REQ-BOOT-004 + REQ-BOOT-005 — `TestComposeConfigValid`** (`deploy/compose_test.go` with `//go:build integration`)
   - Invokes `exec.Command("docker", "compose", "-f", "deploy/docker-compose.yml", "config")`.
   - Asserts exit code 0.
   - Parses the rendered YAML and asserts every service has a `healthcheck` stanza.
   - Fails until `deploy/docker-compose.yml` is written with healthchecks.

5. **REQ-BOOT-006 — `TestEnvExampleCompleteness`** (`scripts/check_env_test.go` or shell test)
   - Scans `deploy/docker-compose.yml` for `${VAR}` occurrences.
   - For each, asserts presence in `.env.example`.
   - Fails until every interpolated variable has a documented default.

6. **REQ-BOOT-003 — `test_web_scaffold` (pnpm task)** (`web/tests/scaffold.test.ts`)
   - Asserts `web/app/layout.tsx` and `web/app/page.tsx` exist.
   - Asserts `package.json` lists Next.js 16.x and a shadcn/ui marker (either dependency or `components.json`).
   - Runs `pnpm build` via Node test runner and asserts exit code 0.
   - Fails until `web/` scaffold is produced by `create-next-app@16` + `shadcn init`.

7. **REQ-BOOT-010 — `TestMakefileTargets`** (`scripts/makefile_test.sh`)
   - `make -n help | grep -E "^  (dev|test|lint|build|clean|compose-up|compose-down|fmt|tidy|install-py):"` must return ten matches.
   - Fails until `Makefile` is written with the required targets.

8. **REQ-BOOT-008 — `test_precommit_passes`** (CI job alias; also a local test)
   - Runs `pre-commit run --all-files` on the committed scaffold.
   - Asserts exit code 0.
   - Fails until all hooks pass on the scaffold (may require autoupdate + fixup commits).

Remaining REQs (BOOT-007, BOOT-009, BOOT-011) follow the same pattern: one failing test per acceptance bullet, implementation minimal enough to pass the test, REFACTOR only when green.

## 9. Dependencies

This SPEC has no upstream SPEC dependencies (it is the first).

It **blocks** the following SPECs, which extend or consume its scaffold:

| Blocked SPEC | Consumption Point |
|--------------|-------------------|
| SPEC-DEP-001 | Tightens version pins; converts `searxng:latest` to a dated digest |
| SPEC-OBS-001 | Fills `internal/obs/` with OpenTelemetry wiring |
| SPEC-LLM-001 | Adds LiteLLM client code in `internal/llm/`; extends the LiteLLM compose entry with a config mount |
| SPEC-IR-001 | Fills `internal/router/` and `internal/fanout/`; adds `.proto` files under `proto/` |
| SPEC-ADP-001 | Adds first search adapter in `internal/adapters/` |
| SPEC-CLI-001 | Extends `cmd/usearch/` with real subcommands beyond `--version` |
| SPEC-IDX-001 | Fills `internal/index/` with Qdrant + Meili clients and schemas |
| SPEC-UI-001 | Builds first real pages and components under `web/` |

## 10. Risks

| Severity | Risk | Mitigation |
|----------|------|------------|
| High | SearXNG `:latest` image drifts between CI runs, producing non-reproducible builds. | Document in Open Questions; pin to a dated digest in SPEC-DEP-001. Until then, the `compose-check.yml` job always pulls fresh, accepting drift as a known limitation. |
| High | `uv` workspace compatibility with three heterogeneous services (gpt-researcher's LangChain deps, STORM's litellm deps, embedder's ML/BLAS deps) may produce resolver conflicts. | Run `uv sync` on each service individually first; if a joint resolve fails, fall back to per-service `pyproject.toml` + `.venv` (no workspace) as a contingency, documented as an Open Question. |
| Medium | CI `compose-up --wait` on GitHub-hosted runners may exceed the 90-second budget under load (Docker-in-Docker slow pulls, image warm-up). | Layer-cache Docker pulls via `actions/cache` keyed on the compose image list; relax the 60s-local / 90s-CI requirement to 90s/120s if CI runner P95 exceeds the budget. |
| Medium | Directory typo: the working directory is `univesal-search` while the GitHub repo will be `universal-search`. A filesystem rename is cosmetic but can confuse new contributors; the Go module path `github.com/elymas/universal-search` is independent of directory name. | Document in Open Questions + README; defer rename to repo creation. Module imports are unaffected. |
| Medium | Pre-commit hook rev pins drift between environments if developers forget to `pre-commit autoupdate`. | First commit runs `pre-commit autoupdate`; CI fails if `.pre-commit-config.yaml` contains unresolved `rev: ""` placeholders. |
| Low | LiteLLM image `v1.83.7-stable.patch.1` is a patched stable tag that may be retracted upstream. | Monitor release notes; SPEC-DEP-001 revisits pin. |
| Low | `searxng/searxng-docker` repo archival (2026-03-28) could signal upstream maintenance slowdown. | Consume `searxng/searxng` image directly; if upstream image becomes stale, plan fork in SPEC-ADP-001. |

## 11. Open Questions

The following are explicitly unresolved in this SPEC and must be answered by the run-phase implementer (or deferred to the blocked SPECs above). They do not prevent SPEC-BOOT-001 from being approved.

1. **SearXNG image pinning strategy (Q6 from architect's D12).** Should `deploy/docker-compose.yml` pin `searxng/searxng` by SHA256 digest, dated tag (e.g., `2026.04.01-abc1234`), or leave `:latest`? Default for M1: `:latest`. Resolution deferred to SPEC-DEP-001 (package pinning sweep).
2. **`proto/` granularity (Q7).** Is one `.proto` file per internal service the correct default, or should shared message types live in a `proto/common.proto`? Default for M1: create `proto/.gitkeep` only; first real `.proto` file lands in SPEC-IR-001.
3. **Directory rename (`univesal-search` → `universal-search`).** Cosmetic filesystem rename to match the canonical GitHub repo name. Independent of the Go module path. Default: do not rename in this SPEC; document in README. Owner: project maintainer, at repo-creation time.

## 12. tech.md Refinement Required

Upon approval of SPEC-BOOT-001, `.moai/project/tech.md` requires the following edits (to be performed in the run phase, not in this SPEC):

- **§2 Language Matrix**: update Python baseline from `3.12+` to `3.11+`. Rationale: `gpt-researcher` and `knowledge-storm` upstreams target Python 3.11; forcing 3.12 would block M1. Confirmed with user on 2026-04-24.
- **§7 Decision Log**: append a new entry:
  > `2026-04-24: Python baseline refined to 3.11+ per gpt-researcher / knowledge-storm upstream constraints (SPEC-BOOT-001 annotation). Previously 3.12+; no existing code affected (greenfield scaffold). Owner: limbowl.`

This is an artifact note only. Do not edit `tech.md` from within SPEC-BOOT-001; the edit is a run-phase deliverable tracked against REQ-BOOT-002 acceptance.

---

*End of SPEC-BOOT-001*
