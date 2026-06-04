# Codebase Structure

**Analysis Date:** 2026-06-04

## Directory Layout

```
universal-search/
├── cmd/                  # Main binaries (thin entry points)
│   ├── usearch/          # CLI: query, deep, config, history, repl, sources, login
│   ├── usearch-api/      # HTTP+SSE backend for the web frontend
│   ├── usearch-mcp/      # MCP server (stdio/http)
│   └── eval/             # Evaluation harness runner
├── internal/             # Private Go packages (module-scoped)
│   ├── adapters/         # Source plugins: reddit, hn, arxiv, github, youtube, searxng, naver, koreanews, social, noop
│   ├── router/           # Intent classification → routing decision
│   ├── fanout/           # Parallel adapter dispatch, dedup, sort
│   ├── synthesis/        # LLM synthesis + citations + faithfulness
│   ├── orchestrator/     # Reusable Search() pipeline (shared by API/MCP)
│   ├── deepagent/        # Researcher→Reviewer→Writer→Verifier multi-agent pipeline
│   ├── llm/              # LiteLLM client: provider, retry, cost, stream, config
│   ├── index/            # Hybrid index dispatch (meili/pg/qdrant/router/tenancy/tokenizer)
│   ├── idx5/             # Index v5 server: docid, lookup, staleness, writeback
│   ├── embedder/         # Embedding client
│   ├── synthcluster/     # Simhash/unionfind dedup clustering
│   ├── streamsynth/      # SSE streaming synthesis + agent events
│   ├── deepreport/       # Long-form report client
│   ├── sse/              # Server-sent-events transport
│   ├── api/admin/        # Admin HTTP handlers (adapters, audit, loopback mw)
│   ├── mcpserver/        # MCP server core + tools/
│   ├── auth/             # OIDC, cookies, middleware, rbac/, revocation
│   ├── access/           # Access control, cascade, dialer, escalation
│   ├── security/         # prompt, ratelimit, secrets, secretstore, ssrf, events
│   ├── audit/            # Tamper-evident audit chain + LiteLLM reconcile
│   ├── obs/              # Observability: log, metrics, trace, reqid
│   ├── usearch/          # CLI support: config, history
│   └── version/          # Build/version metadata
├── pkg/                  # Public, importable packages
│   ├── types/            # Adapter, Query, NormalizedDoc, Capabilities, errors
│   └── client/           # Thin programmatic client
├── proto/                # Protocol buffer definitions
├── services/             # Python sidecar services (Docker)
│   ├── embedder/         # Embedding service (pyproject.toml)
│   ├── researcher/       # Deep-research worker
│   ├── storm/            # STORM long-form generation
│   ├── tokenizer-ko/     # Korean tokenizer
│   └── koreanews/        # Korean news crawler service
├── web/                  # Next.js 16 / React 19 frontend
│   └── src/{app,components,lib}/
├── deploy/               # Docker compose + Dockerfiles + service configs
├── charts/               # Helm chart (universal-search)
├── tests/                # Cross-package Go tests (integration, eval)
├── scripts/              # CI/check shell scripts
├── tools/                # Code-gen + skill tooling
├── ops/                  # Operational config (security)
├── docs/                 # MDX documentation site
├── .moai/                # MoAI-ADK config, specs, plans, eval, state
├── .claude/              # Claude Code agents, commands, hooks, rules, skills
├── .planning/            # GSD planning artifacts (codebase maps live here)
└── go.mod                # Module: github.com/elymas/universal-search (Go 1.25.8)
```

## Directory Purposes

**`cmd/`:**
- Purpose: One subdirectory per compiled binary; each is a thin wiring layer.
- Contains: `main.go`, cobra commands, HTTP handlers, output formatters, tests.
- Key files: `cmd/usearch/main.go`, `cmd/usearch/root.go`, `cmd/usearch/query.go`, `cmd/usearch-api/main.go`, `cmd/usearch-mcp/main.go`.

**`internal/`:**
- Purpose: All private business logic; not importable outside this module.
- Contains: orchestration, adapters, cross-cutting services.
- Key files: `internal/adapters/registry.go`, `internal/router/router.go`, `internal/fanout/dispatch.go`, `internal/synthesis/client.go`, `internal/orchestrator/search.go`.

**`internal/adapters/`:**
- Purpose: One subpackage per search source, each implementing `pkg/types.Adapter`.
- Contains: source-specific HTTP/API clients returning `[]types.NormalizedDoc`.
- Key files: `adapters.go`, `registry.go`, `telemetry.go`, `visibility.go`, plus `arxiv/`, `github/`, `hn/`, `koreanews/`, `naver/`, `reddit/`, `searxng/`, `social/`, `youtube/`, `noop/`.

**`pkg/`:**
- Purpose: Public contracts safe for external import.
- Key files: `pkg/types/adapter.go` (the Adapter interface), `pkg/types/normalized_doc.go`, `pkg/types/query.go`, `pkg/types/errors.go`, `pkg/client/client.go`.

**`services/`:**
- Purpose: Python (pyproject.toml) sidecars reached over the network in the compose stack.
- Contains: `embedder/`, `researcher/`, `storm/`, `tokenizer-ko/`, `koreanews/` — each with `Dockerfile`, `src/`, `tests/`.

**`web/`:**
- Purpose: Next.js App Router frontend.
- Contains: `src/app/` (pages: `page.tsx`, `admin/`, `history/`, `sources/`), `src/components/` (incl. `ui/`), `src/lib/` (`api.ts`, `sse-client.ts`, `utils.ts`).

**`deploy/`:**
- Purpose: Container deployment.
- Key files: `deploy/docker-compose.yml`, `deploy/docker-compose.gpu.yml`, `Dockerfile.usearch-api`, `Dockerfile.usearch-mcp`, `Dockerfile.usearch-migrate`, plus per-service config dirs (`prometheus/`, `grafana/`, `litellm/`, `searxng/`, `postgres/`, `alertmanager/`).

**`.moai/` and `.claude/`:**
- `.moai/`: MoAI-ADK workspace — `specs/`, `plans/`, `config/`, `eval/`, `reports/`, `state/`, `manifest.json`.
- `.claude/`: Claude Code config — `agents/`, `commands/`, `hooks/`, `rules/`, `skills/`, `settings.json`.

## Key File Locations

**Entry Points:**
- `cmd/usearch/main.go`: CLI process entry.
- `cmd/usearch-api/main.go`: HTTP/SSE backend.
- `cmd/usearch-mcp/main.go`: MCP server.
- `cmd/eval/main.go`: eval harness.

**Configuration:**
- `go.mod` / `go.sum`: Go module + dependencies.
- `web/package.json`: frontend deps + scripts.
- `deploy/docker-compose.yml`: 13-service runtime stack.
- `.moai/config/`: MoAI-ADK settings.

**Core Logic:**
- `internal/orchestrator/search.go`: reusable search pipeline.
- `internal/router/router.go`: classification.
- `internal/fanout/dispatch.go`: parallel dispatch.
- `internal/synthesis/client.go`: answer synthesis.
- `internal/deepagent/orchestrator.go`: deep research pipeline.

**Testing:**
- Co-located `*_test.go` throughout `cmd/`, `internal/`, `pkg/`.
- `tests/integration/deep_tree_test.go`, `tests/eval/korean/`.

## Naming Conventions

**Files:**
- Snake_case Go files: `query.go`, `deep_cmd.go`, `routing_decision.go`, `cache_writethrough.go`.
- Tests: `*_test.go` co-located; internal tests `*_internal_test.go`; coverage helpers `coverage_*_test.go`.
- Command files in CLI: `<noun>_cmd.go` (`config_cmd.go`, `history_cmd.go`, `sources_cmd.go`, `login_cmd.go`).

**Directories:**
- Lowercase, single-word package dirs matching the Go package name (`fanout`, `synthcluster`, `streamsynth`).
- Adapter subpackages named exactly by source ID (`reddit`, `hackernews` → `hn`, `arxiv`).

**Symbols:**
- Exported: PascalCase (`Execute`, `Registry`, `RoutingDecision`).
- SPEC traceability in doc comments (`SPEC-CLI-002`, `REQ-CLI-006`) and `@MX:` annotation tags.

## Where to Add New Code

**New search source/adapter:**
- Implementation: `internal/adapters/<source>/<source>.go` implementing `pkg/types.Adapter`.
- Register: add to `buildProductionRegistry` in `cmd/usearch/query.go`.
- Tests: `internal/adapters/<source>/<source>_test.go`.

**New CLI subcommand:**
- Command file: `cmd/usearch/<name>_cmd.go` with a `new<Name>Cmd() *cobra.Command`.
- Register: add `root.AddCommand(new<Name>Cmd())` in `registerSubcommands` (`cmd/usearch/root.go:62`).

**New API endpoint:**
- Handler: `cmd/usearch-api/handlers/<name>.go`; wire into `cmd/usearch-api/main.go`.

**New MCP tool:**
- Tool: `internal/mcpserver/tools/<name>.go`; register in the tool definitions.

**New orchestration stage:**
- Library code: `internal/<stage>/`; depend only on `pkg/types` + registry — never a concrete adapter.

**Utilities / shared types:**
- Public contracts: `pkg/types/`.
- Observability helpers: `internal/obs/` (log, metrics, trace, reqid).

**Frontend feature:**
- Page: `web/src/app/<route>/page.tsx`; shared component: `web/src/components/`; API/SSE client: `web/src/lib/`.

## Special Directories

**`.moai/`:**
- Purpose: MoAI-ADK SPEC/plan/eval workspace.
- Generated: Partially (reports, state, logs).
- Committed: Yes (specs, config, plans).

**`.planning/`:**
- Purpose: GSD planning artifacts; codebase maps written to `.planning/codebase/`.
- Generated: Yes.
- Committed: Yes.

**`.moai-backups/`, `.venv/`, `.ruff_cache/`, `.pytest_cache/`, `node_modules/`, `web/.next/`, `bin/`:**
- Purpose: Backups, virtualenv, tool caches, build artifacts.
- Generated: Yes.
- Committed: No (gitignored).

**`charts/`:**
- Purpose: Helm chart `universal-search` (`templates/`, `charts/`, `ci/`) for Kubernetes deployment.
- Committed: Yes.

---

*Structure analysis: 2026-06-04*
