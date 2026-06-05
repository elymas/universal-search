# Structure — Universal Search Engine

## 1. Repository Layout

Monorepo, organized by bounded context. Go is primary for the orchestration plane (latency, concurrency, single-binary deploy). Python is used where upstream libraries (gpt-researcher, STORM, naver-search-mcp) and ML tooling dictate. TypeScript is confined to the Web UI.

```
univesal-search/
├── cmd/                          # Go binaries
│   ├── usearch/                  # main CLI
│   │   └── sources_cmd.go        # Registry-backed sources commands (SPEC-CLI-003)
│   ├── usearch-mcp/              # MCP server binary
│   └── usearch-api/              # HTTP API for Web UI
│       ├── api_handlers.go       # REST handlers /search, /sources, /health (SPEC-API-001)
│       └── api_handlers_test.go
│
├── internal/                     # Go private packages (orchestration plane)
│   ├── router/                   # Intent Router (SPEC-IR)
│   ├── fanout/                   # Multi-source dispatch (SPEC-FAN)
│   ├── adapters/                 # Per-source adapters (SPEC-ADP)
│   │   ├── reddit/
│   │   │   ├── oauth.go          # Reddit OAuth (client_credentials grant, SPEC-ADP-001a)
│   │   │   └── naver.go
│   │   ├── xtwitter/
│   │   ├── hackernews/
│   │   ├── youtube/
│   │   ├── bluesky/
│   │   ├── arxiv/
│   │   ├── github/
│   │   ├── naver/                # wraps naver-search-mcp
│   │   ├── daum/
│   │   ├── rss_korean/
│   │   ├── searxng/              # SearXNG fanout bridge
│   │   └── polymarket/
│   ├── pipeline/                  # Pipeline extraction for HTTP API (SPEC-API-001)
│   │   ├── pipeline.go           # Core pipeline interface
│   │   └── registry.go           # Adapter registry for status checks
│   ├── access/                   # 5-phase access fallback (SPEC-CACHE)
│   │   ├── phase0_index.go
│   │   ├── phase1_probe.go
│   │   ├── phase2_tls.go         # curl_cffi equivalent
│   │   └── phase3_browser.go     # Playwright bridge
│   ├── index/                    # Hybrid index client (SPEC-IDX)
│   │   ├── qdrant/
│   │   ├── meilisearch/
│   │   └── postgres/
│   ├── llm/                      # LiteLLM proxy client + router (SPEC-LLM)
│   ├── synthesis/                # Basic synthesis pipeline (SPEC-SYN)
│   ├── auth/                     # Team auth, RBAC, audit (SPEC-AUTH)
│   ├── obs/                      # Observability (SPEC-OBS)
│   ├── eval/                     # Citation faithfulness eval
│   └── xenable/                  # Adapter feature flag gating (SPEC-SEC-002, SPEC-ADP-006-XENABLE)
│
├── pkg/                          # Go public packages (SDK for external callers)
│   ├── client/                   # Go client library
│   └── types/                    # shared types (Query, Result, Citation, ...)
│
├── services/                     # Python services (gRPC / HTTP sidecars)
│   ├── researcher/               # gpt-researcher wrapper (SPEC-SYN, SPEC-DEEP)
│   ├── storm/                    # STORM wrapper for /deep reports
│   ├── embedder/                 # local embedding service (BGE-M3, fastembed-bm25)
│   └── youtube-extract/          # YouTube extraction sidecar (Python/FastAPI, SPEC-ADP-005a)
│
├── web/                          # Web UI (TypeScript, Next.js)
│   ├── app/
│   ├── components/
│   └── lib/
│
├── deploy/                       # Deployment artifacts
│   ├── docker-compose.yml        # dev stack (Qdrant, Meili, PG, SearXNG, LiteLLM)
│   ├── k8s/                      # Helm chart for team deployment
│   └── terraform/                # (future) cloud infra modules
│
├── skills/                       # Claude Skill package
│   └── universal-search/
│       ├── SKILL.md
│       └── scripts/
│
├── docs/                         # User & operator documentation
│   ├── adapters/                 # per-adapter: keys, rate limits, troubleshooting
│   ├── deployment/
│   └── evaluation/
│
├── .moai/                        # MoAI-ADK workspace
│   ├── project/                  # this folder (product.md, structure.md, tech.md, roadmap.md)
│   ├── specs/                    # SPEC-XXX documents
│   ├── design/
│   └── config/
│
├── .claude/                      # Claude Code rules, agents, skills, hooks
└── CLAUDE.md
```

## 2. Service Topology

```
┌─────────────────────────────────────────────────────────────────┐
│ Client surfaces                                                 │
│  CLI (usearch)   MCP (usearch-mcp)   Claude Skill   Web UI      │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│ usearch-api (Go) — auth, rate-limit, request routing            │
└──────────────────────────┬──────────────────────────────────────┘
                           │
           ┌───────────────┼───────────────┐
           ▼               ▼               ▼
    ┌──────────┐    ┌────────────┐   ┌────────────┐
    │  Router  │───▶│   Fanout   │──▶│ Synthesis  │
    │  (Go)    │    │   (Go)     │   │ (Go+Py)    │
    └──────────┘    └─────┬──────┘   └─────┬──────┘
                          │                │
                          ▼                ▼
                 ┌────────────────┐  ┌──────────────┐
                 │ Source Adapter │  │  LLM Router  │
                 │   pool (Go)    │  │ (LiteLLM)    │
                 └────────┬───────┘  └──────┬───────┘
                          │                 │
                          ▼                 ▼
                ┌──────────────────────────────────┐
                │  Shared Hybrid Index             │
                │  Qdrant (vectors, multi-tenant)  │
                │  Meilisearch (BM25 + vectors)    │
                │  Postgres (metadata, audit)      │
                └──────────────────────────────────┘

                  External services (sidecars):
                  SearXNG  ·  Playwright MCP  ·  LiteLLM
                  researcher.py (gpt-researcher)
                  storm.py (STORM)
                  embedder.py (BGE-M3 / fastembed)
```

## 3. Bounded Contexts & Ownership

| Context | Primary language | Key responsibility | Owner agent (MoAI) |
|---------|------------------|--------------------|--------------------|
| Router | Go | Classify query intent → dispatch plan | expert-backend |
| Fanout | Go | Parallel dispatch to N adapters, timeout, partial-result assembly | expert-backend |
| Adapters | Go | Per-source auth, rate-limit, pagination, normalization | expert-backend |
| Access fallback | Go + Python | 5-phase escalation for blocked sources | expert-backend |
| Index | Go | Qdrant + Meilisearch + PG client; hybrid rank fusion (RRF) | expert-backend |
| Synthesis (basic) | Go + Python | gpt-researcher planner-executor call, citation assembly | expert-backend / python services |
| Synthesis (deep) | Python | STORM + multi-agent (Researcher/Reviewer/Writer/Verifier) | expert-backend |
| LLM routing | Go + LiteLLM | Provider selection by task class | expert-backend |
| Auth / RBAC / audit | Go | SSO, team scope, per-query audit log | expert-security |
| Observability | Go | Query log, latency histograms, citation eval | expert-performance |
| Web UI | TypeScript / Next.js | Query input, streaming results, citation UI | expert-frontend |
| CLI | Go | `usearch query "..."`, `usearch deep "..."` | expert-backend |
| MCP server | Go | JSON-RPC endpoint, tool schema | expert-backend |
| Claude Skill | Markdown + scripts | SKILL.md wrapping MCP client | builder-skill |

## 4. Data Model (high-level)

```
Query          { id, user_id, team_id, text, intent[], created_at }
DispatchPlan   { query_id, adapters[], deadline, max_parallel }
RawResult      { query_id, adapter, source_url, title, snippet, raw_payload, fetched_at }
NormalizedDoc  { id, query_id, source_type, url, title, content, authors, published_at, engagement_metrics, lang }
Embedding      { doc_id, model, vector, created_at }
Citation       { claim_text, doc_ids[], confidence }
SynthReport    { query_id, mode(basic|deep), summary, citations[], model_used, tokens_used, cost }
TeamIndex      { team_id, doc_ids[], visibility(team|user|public) }
AuditEvent     { user_id, team_id, action, query_id, timestamp, ip, user_agent }
```

## 5. Interface Stability Contract

- **`pkg/types`** is the public SDK boundary. Breaking changes require major version bump.
- **MCP tool schema** (`usearch-mcp`) follows MCP spec versioning; additions are minor, removals are major.
- **CLI flags** follow semver on the binary.
- **Internal packages (`internal/*`)** have no stability guarantee — free to refactor.
- **Python services** expose gRPC contracts (`.proto` files under `proto/`); these follow semver.

## 6. Test Topology

| Layer | Test type | Framework |
|-------|-----------|-----------|
| Adapters | Contract tests against recorded fixtures | go test + testify + go-vcr |
| Router | Unit tests (intent classification) | go test |
| Fanout | Integration tests with fake adapters | go test -tags=integration |
| Index | Integration with dockerized Qdrant / Meili / PG | go test -tags=integration + testcontainers |
| Synthesis | Golden-file tests with stubbed LLM | go test + pytest |
| Citation eval | Automated faithfulness score on benchmark set | pytest + deepeval |
| E2E | Playwright against Web UI + live local stack | Playwright MCP |
| Load | k6 on `usearch-api` | k6 |

Coverage targets (TRUST-5 gates): Go ≥ 85%, Python services ≥ 80%, Web UI ≥ 70%.
