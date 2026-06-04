# Technology Stack

**Analysis Date:** 2026-06-04

## Languages

**Primary:**
- Go 1.25.8 - Core engine, CLI, admin API, MCP server. Module `github.com/elymas/universal-search` (`go.mod`). All `cmd/` entry points and `internal/` packages.
- TypeScript 5.x - Next.js web frontend in `web/src/` (`web/tsconfig.json`).
- Python >=3.11 - Sidecar microservices in `services/` (embedder, researcher, tokenizer-ko, koreanews). `services/storm` targets >=3.10.

**Secondary:**
- SQL - PostgreSQL schema/migrations consumed via `cmd/usearch-migrate` and `internal/audit`/`internal/access` (Casbin pg-adapter).
- Bash/Dockerfile - Build and deploy tooling under `deploy/`, `services/*/Dockerfile`, `scripts/`.

## Runtime

**Environment:**
- Go 1.25.8 toolchain (`go.mod` line 3). Note: Docker builder images pin `golang:1.24-alpine` (`deploy/Dockerfile.usearch-api`) — a version skew worth tracking.
- Node.js (Next.js 16) for `web/`. React 19 runtime.
- Python 3.11+ (FastAPI/uvicorn) for sidecars; managed via `uv` workspace (`pyproject.toml`).

**Package Manager:**
- Go modules - Lockfile `go.sum` present.
- pnpm (web) - Lockfile `web/pnpm-lock.yaml` present. `onlyBuiltDependencies` restricts native builds to `sharp`, `unrs-resolver` (`web/package.json`).
- uv (Python) - Workspace defined in root `pyproject.toml` `[tool.uv.workspace]`; members are the four production sidecars, `services/storm` excluded.

## Frameworks

**Core (Go):**
- `github.com/spf13/cobra` v1.10.2 - CLI command tree (`cmd/usearch/root.go`).
- `github.com/knadh/koanf/v2` v2.3.4 - Layered configuration loader (`internal/usearch/config/config.go`, `internal/mcpserver/config.go`). Providers: env, file, structs; TOML parser.
- `github.com/modelcontextprotocol/go-sdk` v1.6.1 - MCP server + tools (`internal/mcpserver/`, `cmd/usearch-mcp/main.go`).
- `github.com/playwright-community/playwright-go` v0.5700.1 - Headless browser fetch for deep research adapters.

**Web:**
- Next.js 16.2.6 (App Router) - `web/src/app/`. Config `web/next.config.mjs`.
- React 19 / react-dom 19.
- Tailwind CSS 3.4 (`web/tailwind.config.ts`, `web/postcss.config.mjs`) + Radix UI primitives (`@radix-ui/react-dialog`, `-tooltip`, `-separator`, `-slot`), `class-variance-authority`, `tailwind-merge`, `lucide-react`. shadcn-style components in `web/src/components/ui/`.

**Python sidecars:**
- FastAPI >=0.115 + uvicorn[standard] - all of `services/*/pyproject.toml`.
- `FlagEmbedding>=1.3.0` (BGE-M3 embeddings) - `services/embedder`.
- `mecab-ko>=1.0,<2.0` (Korean morphology) - `services/tokenizer-ko`.

**Testing:**
- Go: `github.com/stretchr/testify` v1.11.1, `go.uber.org/goleak` v1.3.0, `github.com/DATA-DOG/go-sqlmock` v1.5.2, `github.com/alicebob/miniredis/v2` v2.38.0.
- Web: Vitest 4 (`web/vitest.config.ts`), `@testing-library/react` 16, jsdom.
- Python: pytest 8 + pytest-asyncio + pytest-cov (`[dependency-groups]` in `services/*/pyproject.toml`).

**Build/Dev:**
- `Makefile` - Aggregate targets: `test` (test-go/test-py/test-node), `lint`, `build`, `compose-up/down`, `fmt`, `tidy`, `install-py`.
- GoReleaser v2 (`.goreleaser.yml`) - 3 binaries × {linux,darwin} × {amd64,arm64}; `CGO_ENABLED=0`, stripped (`-s -w`), version injected via ldflags into `internal/version`.
- ESLint 9 flat config (`web/eslint.config.mjs`) + Prettier 3.
- golangci-lint, ruff, hadolint (invoked via `make lint`).

## Key Dependencies

**Critical (Go):**
- `github.com/qdrant/go-client` v1.17.0 - Vector search client (Qdrant).
- `github.com/meilisearch/meilisearch-go` v0.36.2 - Keyword/hybrid index client.
- `github.com/redis/go-redis/v9` v9.19.0 - Cache + task queue.
- `github.com/jackc/pgx/v5` v5.9.2 + `github.com/go-pg/pg/v10` v10.15.0 - PostgreSQL drivers/ORM.
- `github.com/openai/openai-go` v1.12.0 - LLM client (routed through LiteLLM proxy).
- `github.com/coreos/go-oidc/v3` v3.18.0 + `github.com/golang-jwt/jwt/v5` v5.3.1 - OIDC auth + JWT (`internal/auth/`).
- `github.com/casbin/casbin/v2` v2.135.0 + `github.com/casbin/casbin-pg-adapter` v1.5.0 - RBAC authorization (`internal/access/`).
- `github.com/google/go-github/v73` v73.0.0 - GitHub source adapter (`internal/adapters/github`).
- `github.com/mmcdole/gofeed` v1.3.0 - RSS/Atom feed parsing (news/blog adapters).
- `github.com/temoto/robotstxt` v1.1.2 - robots.txt compliance for crawling.
- `github.com/oklog/ulid/v2` v2.1.1 - Sortable IDs (direct require).

**Infrastructure (Go):**
- OpenTelemetry suite `go.opentelemetry.io/otel` v1.43.0 (+ sdk, trace, otlptrace, otlptracegrpc) - Distributed tracing/metrics export.
- `github.com/prometheus/client_golang` v1.23.2 - Metrics endpoint (`internal/obs/`).
- `golang.org/x/sync` v0.20.0 (errgroup), `golang.org/x/time` v0.15.0 (rate limiting).

## Configuration

**Environment:**
- `.env` (present, gitignored) + `.env.example` (template, 60+ keys).
- Loaded via koanf env provider in `internal/usearch/config/config.go`.
- Key groups: service ports (`QDRANT_*`, `MEILI_*`, `POSTGRES_*`, `REDIS_*`, `SEARXNG_*`, `LITELLM_*`, `EMBEDDER_*`, `TOKENIZER_KO_*`, `RESEARCHER_*`), secrets (`OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `MEILI_MASTER_KEY`, `LITELLM_MASTER_KEY`, `SEARXNG_SECRET`, `NAVER_CLIENT_ID/SECRET`, `GITHUB_TOKEN`, `REDDIT_CLIENT_SECRET`), deep-research tuning (`DEEP_AGENT_*`, `DEEP_TREE_*`), observability (`OTLP_ENDPOINT`, `OTLP_SAMPLE_RATIO`, `LOKI_ENDPOINT`, `LOG_LEVEL`).
- `NEXT_PUBLIC_API_URL` configures the web client backend base (`web/src/lib/api.ts`).

**Build:**
- `.goreleaser.yml` - Release binaries.
- `Makefile` - Local build/test/lint orchestration.
- `pyproject.toml` (root + per-service) - Python deps via uv; hatchling build backend.
- `.gitleaks.toml` - Secret-scanning rules.

## Platform Requirements

**Development:**
- Go 1.25.8, Node.js (pnpm), Python 3.11+ with uv, Docker + docker compose.
- `make dev` brings up the full 11-service compose stack with healthchecks (`deploy/docker-compose.yml`).

**Production:**
- Multi-arch container images (linux/amd64, linux/arm64) built distroless non-root (`deploy/Dockerfile.usearch-api`, `-mcp`, `-migrate`).
- Helm chart `charts/universal-search/` (Chart v0.1.0, appVersion 0.1.0) deploys `usearch-api`, `usearch-mcp`, Documentation, with Bitnami PostgreSQL 16.4.5 / Redis 20.6.2 and Qdrant 1.16.3 subcharts.
- Optional GPU overlay for the embedder (`deploy/docker-compose.gpu.yml`, NVIDIA CUDA 12.x + nvidia-container-toolkit).

---

*Stack analysis: 2026-06-04*
