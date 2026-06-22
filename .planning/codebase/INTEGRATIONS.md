# External Integrations

**Analysis Date:** 2026-06-04

## APIs & External Services

**Search backends (source adapters in `internal/adapters/`):**

- SearXNG metasearch - Primary web search aggregator. Adapter `internal/adapters/searxng`. Reached at `SEARXNG_BASE_URL`; JSON response format enabled (see commit e0bedcc). Runs as a compose service.
- arXiv - Academic papers. Adapter `internal/adapters/arxiv`, OpenSearch/Atom feed at `http://arxiv.org/abs/`.
- GitHub - Repos/code. Adapter `internal/adapters/github` via `github.com/google/go-github/v73`, `https://api.github.com`. Auth: `GITHUB_TOKEN`.
- Hacker News - Adapter `internal/adapters/hn`.
- Reddit - Adapter `internal/adapters/reddit`. Auth: `REDDIT_CLIENT_SECRET` (+ client id).
- Naver - Korean blog/search/datalab. Adapter `internal/adapters/naver`, endpoints under `blog.naver.com`, `datalab.naver.com`. Auth: `NAVER_CLIENT_ID`, `NAVER_CLIENT_SECRET`.
- YouTube - Adapter `internal/adapters/youtube`.
- Social (Bluesky) - Adapter `internal/adapters/social`, `bsky.app` profile/post URLs.
- Korean News - Adapter `internal/adapters/koreanews`, backed by `services/koreanews` FastAPI sidecar.
- noop - Test/disabled placeholder adapter `internal/adapters/noop`.
- Adapter registry/visibility: `internal/adapters/registry.go`, `adapters.go`, `visibility.go`, `telemetry.go`. Crawl compliance via `github.com/temoto/robotstxt`.

**LLM gateway:**

- LiteLLM proxy (`ghcr.io/berriai/litellm:v1.83.7-stable.patch.1`) - Unified LLM gateway on port 4000 (`deploy/docker-compose.yml`). Go side uses `github.com/openai/openai-go` pointed at the proxy (`internal/llm/`).
- Upstream providers via LiteLLM: OpenAI (`OPENAI_API_KEY`), Anthropic (`ANTHROPIC_API_KEY`), Ollama (`OLLAMA_BASE_URL`, default `http://host.docker.internal:11434`).
- Models referenced: `claude-haiku-4-5`, `claude-sonnet-4-6`, `claude-opus-4-7` (deep-agent roles via `DEEP_AGENT_*_MODEL` env keys).

**Internal HTTP sidecars (Python FastAPI):**

- Researcher synthesis (`services/researcher`, port 8081) - LLM synthesis sidecar, calls LiteLLM. `RESEARCHER_BASE_URL`.
- Embedder (`services/embedder`, port 8082) - BGE-M3 dense embeddings via FlagEmbedding. `EMBEDDER_BASE_URL`. CPU default, GPU via overlay.
- Korean tokenizer (`services/tokenizer-ko`, port 8083) - mecab-ko morphology. `TOKENIZER_KO_BASE_URL`.
- Korean news (`services/koreanews`) - news fetch sidecar.
- STORM (`services/storm`) - long-form report generation; excluded from the uv workspace and not wired into the default compose stack.

## Data Storage

**Databases:**

- Qdrant 1.16.3 - Vector database for semantic search. Client `github.com/qdrant/go-client`. Ports 6333 (HTTP) / 6334 (gRPC). Env `QDRANT_HTTP_PORT`, `QDRANT_GRPC_PORT`.
- Meilisearch 1.42.1 - Keyword + hybrid index. Client `github.com/meilisearch/meilisearch-go`. Port 7700. Auth `MEILI_MASTER_KEY`, `MEILI_ENV`.
- PostgreSQL 16.13 - Metadata + audit log + Casbin policy store. Drivers `jackc/pgx/v5`, `go-pg/pg/v10`. `DATABASE_URL`, `POSTGRES_USER/PASSWORD/DB`. Migrations via `cmd/usearch-migrate` (`deploy/Dockerfile.usearch-migrate`).

**File Storage:**

- Named Docker volumes for stateful services (`qdrant_data`, `meili_data`, `pg_data`, `redis_data`, `prometheus_data`, `grafana_data`, `embedder_models`). No external object store detected.

**Caching:**

- Redis 7 - Session cache + Asynq-style task queue. Client `github.com/redis/go-redis/v9`. `REDIS_URL`, `REDIS_PORT`. Also backs SearXNG and LiteLLM.

## Authentication & Identity

**Auth Provider:**

- OIDC - `github.com/coreos/go-oidc/v3` for token verification (`internal/auth/`, `internal/auth/config.go`).
- JWT - `github.com/golang-jwt/jwt/v5` for session/bearer tokens.
- Authorization (RBAC) - Casbin (`github.com/casbin/casbin/v2`) with PostgreSQL policy adapter (`github.com/casbin/casbin-pg-adapter`) and file-adapter fallback (`internal/access/`).
- Admin UI is localhost-gated (`web/src/app/admin/_components/localhost-gate.tsx`).

## Monitoring & Observability

**Error Tracking / Tracing:**

- OpenTelemetry (`go.opentelemetry.io/otel` + OTLP gRPC trace exporter). Env `OTLP_ENDPOINT`, `OTLP_SAMPLE_RATIO`. Implementation in `internal/obs/`.

**Metrics:**

- Prometheus 2.54.1 - Scrapes the usearch admin server `/metrics` (`github.com/prometheus/client_golang`). Config `deploy/prometheus/prometheus.yml`, recording rules + alerts (`recording-rules.yml`, `alerts.yml`). 30-day retention. Port 9090 (host 9091).
- Grafana 11.3.0 - Adapter reliability dashboards, auto-provisioned from `deploy/grafana/`. Localhost-bound, anonymous viewer.
- Alertmanager 0.27.0 - Optional (compose profile `alerts`), null receiver by default (`deploy/alertmanager/alertmanager.yml`).

**Logs:**

- Structured logging with `LOG_LEVEL`. Optional Loki shipping via `LOKI_ENDPOINT`.

## CI/CD & Deployment

**Hosting:**

- Container images published to `ghcr.io/elymas/universal-search/*` (usearch-api, usearch-mcp, usearch-migrate). Distroless non-root, multi-arch (linux/amd64+arm64).
- Helm chart `charts/universal-search/` with Bitnami PostgreSQL (16.4.5) / Redis (20.6.2) and Qdrant (1.16.3) subcharts (`Chart.yaml`, `Chart.lock`).

**Release Pipeline:**

- GoReleaser v2 (`.goreleaser.yml`) builds 12 archives (3 binaries × 2 OS × 2 arch). SBOM via syft, coverage gates noted in release history.
- `Dockerfile.docs` builds the documentation image.

## Environment Configuration

**Required env vars (secrets — names only):**

- LLM: `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `LITELLM_MASTER_KEY`, `OLLAMA_BASE_URL`.
- Search/index: `MEILI_MASTER_KEY`, `SEARXNG_SECRET`, `SEARXNG_BASE_URL`.
- Data: `DATABASE_URL`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB`, `REDIS_URL`.
- Source adapters: `GITHUB_TOKEN`, `NAVER_CLIENT_ID`, `NAVER_CLIENT_SECRET`, `REDDIT_CLIENT_SECRET`.

**Secrets location:**

- `.env` (gitignored) at repo root; template in `.env.example`. Compose interpolates `${VAR}` — no hardcoded credentials (enforced policy in `deploy/docker-compose.yml`). `.gitleaks.toml` guards against committed secrets.

## Webhooks & Callbacks

**Incoming:**

- HTTP/SSE API served by `cmd/usearch-api` and consumed by the web client: `/api/query`, `/api/query/stream` (SSE), `/api/sources`, `/api/history`, `/api/admin/adapters`, `/api/admin/audit/queries` (`web/src/lib/api.ts`, `web/src/lib/sse-client.ts`). Note: the web backend is currently a partial stub — the CLI (`usearch query` / `deep`) is the fully working search path.
- MCP server (`cmd/usearch-mcp`, `internal/mcpserver/`) exposes tools `search`, `list_sources`, `get_citation`, `deep_research` over the Model Context Protocol via `github.com/modelcontextprotocol/go-sdk`.

**Outgoing:**

- No registered outbound webhooks. Alertmanager can route alerts once an operator wires a receiver (default null).

## LSP Integration

- LSP client tooling is referenced in project rules (`.claude/rules/moai/core/lsp-client.md`, powernap v0.1.4) for the MoAI dev harness, but is NOT a dependency of the `universal-search` application module (`go.mod` does not require powernap). It belongs to the surrounding MoAI-ADK toolchain, not the search product.

---

_Integration audit: 2026-06-04_
