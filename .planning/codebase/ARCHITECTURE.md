<!-- refreshed: 2026-06-04 -->

# Architecture

**Analysis Date:** 2026-06-04

## System Overview

```text
┌─────────────────────────────────────────────────────────────────────────┐
│                          ENTRY POINTS (cmd/)                              │
├──────────────────┬──────────────────┬──────────────────┬─────────────────┤
│  usearch (CLI)   │  usearch-api     │  usearch-mcp     │  eval           │
│ `cmd/usearch`    │ `cmd/usearch-api`│ `cmd/usearch-mcp`│ `cmd/eval`      │
│ cobra query/deep │ HTTP+SSE backend │ MCP stdio/http   │ harness runner  │
└────────┬─────────┴────────┬─────────┴────────┬─────────┴─────────────────┘
         │                  │                  │
         ▼                  ▼                  ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                    ORCHESTRATION LAYER (internal/)                        │
│  router → fanout → synthesis  (the "query" pipeline)                      │
│  deepagent.orchestrator       (the "deep" multi-agent pipeline)          │
│  `internal/router` `internal/fanout` `internal/synthesis`                │
│  `internal/orchestrator` `internal/deepagent`                            │
└────────┬──────────────────────────────────────────────┬──────────────────┘
         │                                               │
         ▼                                               ▼
┌──────────────────────────────────┐   ┌────────────────────────────────────┐
│   SOURCE ADAPTERS                 │   │   CROSS-CUTTING SERVICES            │
│   `internal/adapters/*`           │   │   `internal/llm`  (LiteLLM client)  │
│   reddit, hn, arxiv, github,      │   │   `internal/obs`  (otel/log/metric) │
│   youtube, searxng, naver,        │   │   `internal/security` `internal/auth`│
│   koreanews, social, noop         │   │   `internal/access` `internal/audit`│
└────────┬──────────────────────────┘   └────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────────────────┐
│              EXTERNAL SERVICES (deploy/docker-compose.yml)                │
│  qdrant · meilisearch · postgres · redis · searxng · litellm             │
│  researcher · embedder · tokenizer-ko · prometheus · grafana             │
│  (Python sidecars under `services/`)                                     │
└─────────────────────────────────────────────────────────────────────────┘
```

## Component Responsibilities

| Component          | Responsibility                                                   | File                                 |
| ------------------ | ---------------------------------------------------------------- | ------------------------------------ |
| CLI root           | Cobra command tree, global flags, obs/LLM init                   | `cmd/usearch/root.go`                |
| Query orchestrator | Drives classify → fanout → synthesize for `usearch query`        | `cmd/usearch/query.go`               |
| Deep command       | Stub wiring for multi-agent deep research (not yet wired)        | `cmd/usearch/deep_cmd.go`            |
| Adapter contract   | 4-method interface every source implements                       | `pkg/types/adapter.go`               |
| Adapter registry   | Register/Get/List, auth env validation, per-call obs wrapping    | `internal/adapters/registry.go`      |
| Intent router      | Classifies query → category, lang, adapter set                   | `internal/router/router.go`          |
| Fanout dispatcher  | Parallel adapter dispatch, dedup, sort, partial-failure handling | `internal/fanout/dispatch.go`        |
| Synthesis client   | LLM synthesis of docs into cited answer                          | `internal/synthesis/client.go`       |
| Orchestrator (lib) | Reusable `Search()` pipeline shared by API/MCP                   | `internal/orchestrator/search.go`    |
| Deep agent         | Researcher→Reviewer→Writer→Verifier pipeline                     | `internal/deepagent/orchestrator.go` |
| LLM client         | LiteLLM-backed provider, retry, cost, streaming                  | `internal/llm/client.go`             |
| Observability      | OTel traces, slog logs, Prometheus metrics, reqid                | `internal/obs/obs.go`                |

## Pattern Overview

**Overall:** Layered pipeline architecture with a plugin (adapter) registry, behind multiple thin entry-point binaries that share a common orchestration core.

**Key Characteristics:**

- Standard Go project layout: `cmd/` thin binaries, `internal/` private packages, `pkg/` public contracts.
- Plugin pattern — all sources implement the `types.Adapter` interface and self-register in a `Registry`; orchestration code never imports a concrete adapter except in the CLI's `buildProductionRegistry`.
- Interface-driven seams (`synthClientIface`, `adapters.Registry`, `router.Router`) enable test injection via functional options.
- Cross-cutting concerns (obs, auth, audit, security) are isolated packages composed at the entry-point layer.
- Polyglot deployment: Go core plus Python sidecar services (`services/`) reached over HTTP/network in the compose stack.

## Layers

**Entry-point layer (`cmd/`):**

- Purpose: Process boot, flag parsing, dependency wiring, exit-code mapping.
- Location: `cmd/usearch`, `cmd/usearch-api`, `cmd/usearch-mcp`, `cmd/eval`.
- Contains: cobra commands, HTTP handlers, MCP tool wiring, output formatters.
- Depends on: orchestration + cross-cutting packages.
- Used by: end users (CLI), web frontend (API), MCP clients.

**Orchestration layer (`internal/router`, `internal/fanout`, `internal/synthesis`, `internal/orchestrator`, `internal/deepagent`):**

- Purpose: Turn a query into ranked, synthesized results.
- Depends on: adapter registry, LLM client, obs.
- Used by: all entry points.

**Source layer (`internal/adapters/*`):**

- Purpose: Per-source HTTP/API integration returning `[]types.NormalizedDoc`.
- Depends on: `pkg/types` contracts only.
- Used by: fanout via the registry.

**Indexing/retrieval layer (`internal/index`, `internal/idx5`, `internal/embedder`, `internal/synthcluster`):**

- Purpose: Hybrid retrieval (meili/qdrant/pg), embeddings, dedup clustering.
- Used by: deep pipeline and index-backed adapters.

**Contract layer (`pkg/types`, `pkg/client`):**

- Purpose: Public, importable types (`Adapter`, `Query`, `NormalizedDoc`, `Capabilities`, errors) and a thin client.
- Depends on: stdlib only.

## Data Flow

### Primary Request Path — `usearch query "<prompt>"`

1. `main()` builds the cobra root and runs it (`cmd/usearch/main.go:12`).
2. Query subcommand `RunE` inits obs + optional LLM client, then calls `Execute` (`cmd/usearch/root.go:110`).
3. `Execute` parses flags, attaches a request ID, applies the pipeline deadline, opens root span `usearch.cli.query` (`cmd/usearch/query.go:111`).
4. Production adapter registry is built (`buildProductionRegistry`) registering reddit/hn/arxiv/github/youtube/searxng/naver/koreanews/social (`cmd/usearch/query.go:165`).
5. Router classifies the prompt → category, lang, adapter set (`internal/router/router.go:151`, called at `cmd/usearch/query.go:186`).
6. Source filter is intersected with the routing decision (`cmd/usearch/query.go:195`).
7. Fanout dispatcher fans out to the effective adapter set in parallel, dedups, sorts (`internal/fanout/fanout.go:63`, called at `cmd/usearch/query.go:228`).
8. Partial failures recorded per adapter; all-fail or timeout maps to a system exit code (`cmd/usearch/query.go:238`).
9. Synthesis client produces a cited answer from the docs (`internal/synthesis/client.go`, called at `cmd/usearch/query.go:258`).
10. Response formatted to stdout as text/json/markdown; exit code derived from outcome (`cmd/usearch/query.go:275`).

### Deep Research Path — `usearch deep "<prompt>"`

1. Deep subcommand prints staged progress (Researcher → Reviewer → Writer → Verifier) (`cmd/usearch/deep_cmd.go:29`).
2. CLI wiring is a stub — `@MX:TODO` to call `deepagent.RunPipeline` once an LLM client is available (`cmd/usearch/deep_cmd.go:32`); it returns `ExitSystemError`.
3. The implemented pipeline lives in `internal/deepagent/orchestrator.go` and is exercised today via `cmd/usearch-api/handlers/deep.go` over SSE, not the CLI.

### API / SSE Path — `cmd/usearch-api`

1. `usearch-api` serves HTTP handlers in `cmd/usearch-api/handlers/` (`deep.go`, `synthesis.go`).
2. Synthesis/deep results stream to the web frontend over SSE via `internal/sse` and `internal/streamsynth`.
3. The shared `internal/orchestrator.Search` (`internal/orchestrator/search.go:68`) is the reusable pipeline for non-CLI callers.

**State Management:**

- Request-scoped: request ID + deadline carried on `context.Context` (`internal/obs/reqid`).
- Persistent: history in `internal/usearch/history`; audit chains in `internal/audit`; vector/keyword indexes in external qdrant/meilisearch/postgres.

## Key Abstractions

**Adapter:**

- Purpose: Uniform contract for every search source (`Name`, `Search`, `Healthcheck`, `Capabilities`).
- Examples: `internal/adapters/reddit`, `internal/adapters/arxiv`, `internal/adapters/searxng`.
- Pattern: Interface in `pkg/types/adapter.go` (`@MX:ANCHOR`, fan_in ≥ 12); registry wraps each impl for observability.

**Registry:**

- Purpose: Thread-safe adapter directory with auth-env validation and per-call telemetry wrapping.
- Examples: `internal/adapters/registry.go`.
- Pattern: `sync.RWMutex`, sorted `List()`, duplicate detection.

**RoutingDecision:**

- Purpose: Output of classification (category, lang, adapter set) consumed by fanout.
- Examples: `internal/router/routing_decision.go`.

**NormalizedDoc:**

- Purpose: Canonical cross-source document shape feeding dedup and synthesis.
- Examples: `pkg/types/normalized_doc.go`.

## Entry Points

**`cmd/usearch` (CLI):**

- Location: `cmd/usearch/main.go` → `root.go`.
- Triggers: shell invocation `usearch query|deep|config|history|repl|sources|login`.
- Responsibilities: flag parsing, obs/LLM init, pipeline orchestration, exit codes.

**`cmd/usearch-api` (HTTP backend):**

- Location: `cmd/usearch-api/main.go` + `handlers/`.
- Triggers: HTTP requests from `web/` frontend.
- Responsibilities: deep + synthesis endpoints, SSE streaming.

**`cmd/usearch-mcp` (MCP server):**

- Location: `cmd/usearch-mcp/main.go`; tools in `internal/mcpserver/tools/`.
- Triggers: MCP clients over stdio/HTTP.
- Responsibilities: `search`, `deep_research`, `get_citation`, `list_sources` tools.

**`cmd/eval` (evaluation harness):**

- Location: `cmd/eval/main.go`.
- Triggers: eval runs against `.moai/eval` / `tests/eval`.

## Architectural Constraints

- **Threading:** Fanout uses bounded parallel goroutines (errgroup-style) over the adapter set; all blocking calls take `context.Context` first. Deadline propagates from `--timeout` (default 30s, max 5m).
- **Global state:** `cmd/usearch/root.go` holds a package-level `rootCmd`; tests use `newRootCmd` to get an isolated cobra tree. Avoid additional package-level mutable state per the Go rules.
- **Adapter boundary:** Orchestration code (router/fanout/synthesis) MUST depend only on `pkg/types` + the registry, never on a concrete adapter package. Concrete adapters are imported only in `cmd/usearch/query.go` (`buildProductionRegistry`).
- **Internal isolation:** `internal/` is import-private to this module; public contracts must live in `pkg/`.

## Anti-Patterns

### Importing a concrete adapter into orchestration

**What happens:** A package under `internal/router`, `internal/fanout`, or `internal/synthesis` directly imports e.g. `internal/adapters/reddit`.
**Why it's wrong:** Breaks the plugin seam — orchestration becomes source-coupled and untestable without live sources.
**Do this instead:** Depend on `pkg/types.Adapter` and resolve via `adapters.Registry.Get` (see `cmd/usearch/query.go:171`).

### CLI stub returning success while unwired

**What happens:** `deep_cmd.go` describes the pipeline but does not run it.
**Why it's wrong:** It correctly returns `ExitSystemError`, but the gap (`@MX:TODO` at `cmd/usearch/deep_cmd.go:32`) means CLI `deep` is non-functional while the API path is.
**Do this instead:** Wire the CLI to `internal/deepagent.orchestrator` once the LLM client is constructed, mirroring `cmd/usearch-api/handlers/deep.go`.

## Error Handling

**Strategy:** Wrapped sentinel errors with categories; exit codes mapped at the CLI boundary.

**Patterns:**

- Sources wrap raw errors in `*types.SourceError` with a `Category` (`pkg/types/errors.go`); fanout aggregates them into `AdapterErrors` for partial-failure reporting.
- Registry returns `*RegistryError` (`ErrDuplicateAdapter`, `ErrMissingAuth`, `ErrAdapterNotFound`) — classify via `errors.Is` / `errors.As`.
- CLI maps outcomes to exit codes 0/1/2/3 via `exitError` (`cmd/usearch/exitcode.go`, `root.go:191`).

## Cross-Cutting Concerns

**Logging:** `internal/obs/log` (slog); payload to stdout, progress/errors to stderr only (REQ-CLI-006).
**Tracing/Metrics:** `internal/obs` — OTel spans (`usearch.cli.query`) + Prometheus counters/histograms; registry auto-emits per-Search telemetry.
**Validation:** Flag/format/timeout validation at the CLI boundary; SSRF/private-IP guards in `internal/security/ssrf` and `internal/auth/private_ip.go`.
**Authentication:** OIDC + RBAC in `internal/auth` (validator, middleware, `rbac/`); access control in `internal/access`; secrets in `internal/security/secrets` / `secretstore`.
**Audit:** Tamper-evident chains and LiteLLM reconciliation in `internal/audit`.

---

_Architecture analysis: 2026-06-04_
