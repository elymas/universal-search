# Tech — Universal Search Engine

## 1. Architectural Principles

1. **Composition over reinvention** — three existing MIT/Apache repos (gpt-researcher, STORM, SearXNG) own the hard parts (planner, report generation, metasearch fanout). We own the plane that makes them a **team product**: auth, shared index, Korean-locale coverage, multi-surface client.
2. **Go for the plane, Python for the depth** — orchestration, adapters, index access, auth, CLI, MCP, HTTP API are Go (single binary, low latency, concurrency). Planner / deep-research / STORM / embedding are Python sidecars.
3. **Hybrid retrieval always** — BM25 + dense vector + optional sparse (BGE-M3) fused via Reciprocal Rank Fusion. Never one-or-the-other.
4. **Shared index is a first-class product** — team queries populate it; future queries hit it before external fanout. Dedup + answer reuse is the long-term defensible advantage.
5. **Every claim cites a source** — no synthesis output is emitted without `doc_id` traceability. This is enforced at the synthesis layer, not as a downstream lint.
6. **LLM is a replaceable dependency** — LiteLLM proxy in front. Switching Claude ↔ Opus ↔ local model must be a config change, not a code change.
7. **Provider-neutral via MCP** — the MCP server is the canonical programmatic surface. CLI / Skill / Web UI are thin wrappers over it.

## 2. Language / Runtime Matrix

| Language | Version | Where | Why |
|----------|---------|-------|-----|
| Go | 1.23+ | `cmd/`, `internal/`, `pkg/` | Single-binary deploy, stdlib HTTP, goroutines for fanout, project's native stack |
| Python | 3.12+ | `services/*` | gpt-researcher, STORM, embedding models |
| TypeScript | 5.4+ | `web/` | Next.js 16 App Router for Web UI |
| SQL (PostgreSQL) | 16+ | schema under `internal/index/postgres/migrations/` | metadata + audit |

## 3. Tech Stack (V1 locked)

### Orchestration plane (Go)

| Concern | Choice | Rationale |
|---------|--------|-----------|
| HTTP router | chi v5 | minimal, stdlib-compatible, middleware chain |
| gRPC | connect-go | browser-compatible, simpler than raw grpc-go |
| Config | koanf | layered TOML / env / flag |
| Logging | slog (stdlib) | structured JSON, zero external dep |
| Metrics | prometheus client_golang | industry standard |
| Tracing | OpenTelemetry | multi-backend |
| Task queue | Asynq (Redis-backed) | for async deep research |
| LSP support | powernap (already pinned per `.claude/rules/moai/core/lsp-client.md`) | existing MoAI convention |

### Retrieval layer

| Concern | Choice | Rationale |
|---------|--------|-----------|
| Metasearch fanout | **SearXNG** (self-hosted) | 70+ engines, privacy, zero API keys needed |
| Vector DB | **Qdrant v1.16+** | Tiered Multitenancy (team isolation), RRF support |
| Keyword + hybrid | **Meilisearch v1.10+** | BM25 + vectors, multi-tenant tokens, low-ops |
| Metadata / audit | **PostgreSQL 16** | ACID for audit, pgvector as fallback |
| Embeddings (dense) | **BGE-M3** via local service | multilingual incl. Korean, SOTA OSS |
| Embeddings (sparse) | **FastEmbed BM25** (IDF-weighted) | hybrid with dense via RRF |
| Cross-encoder rerank | **BGE-reranker-v2-m3** | optional, for top-50 → top-10 |
| Fulltext extraction | **Jina Reader** (fallback), `go-readability` (primary) | primary OSS, Jina as escape hatch |
| Browser fallback | **Playwright MCP** | per insane-search pattern |
| Korean keyword tokenizer | **mecab-ko / khaiii** (served via Python sidecar) | Meilisearch default tokenizer is weak for Korean |

### Synthesis layer

| Concern | Choice | Rationale |
|---------|--------|-----------|
| Planner / executor (basic) | **gpt-researcher** (Apache-2.0) | proven planner-executor-publisher, gptr-mcp exists |
| Deep report generation | **STORM** (MIT) | long-form with per-claim citations |
| Multi-agent deep mode | custom over STORM + gpt-researcher agents | Researcher → Reviewer → Writer → Verifier |
| LLM router | **LiteLLM proxy** | single endpoint, per-key provider routing, cost tracking |
| Primary LLM | **Claude Sonnet 4.6 / Opus 4.7** via Anthropic API | team default |
| Cost-saver LLM | **Haiku 4.5** or **Llama-3.3-70B** (vLLM / Ollama) | summarization, dedup |
| Local fallback | **Ollama** or **vLLM** | airgap / cost floor |
| Prompt cache | Anthropic prompt caching | 75% cost savings on repeated planner prompts |

### Team plane

| Concern | Choice | Rationale |
|---------|--------|-----------|
| Auth | OIDC (Keycloak / Authentik self-hosted, or hosted Auth0 / Clerk) | team SSO |
| RBAC | Casbin | declarative policy |
| Audit log | Postgres table + optional S3 archive | immutable trail |
| Rate limit | Redis + sliding window | per-user / per-team quotas |
| Secrets | age-encrypted secrets file, or Vault | per-deployment choice |

### Surfaces

| Surface | Stack | Rationale |
|---------|-------|-----------|
| CLI | Go + cobra + bubbletea (optional TUI) | single binary |
| MCP server | Go + official MCP Go SDK | canonical programmatic surface |
| Claude Skill | SKILL.md + bash/python scripts → MCP server | marketplace distribution |
| Web UI | Next.js 16 + shadcn/ui + Tailwind | team dashboard |

### Observability

| Concern | Choice |
|---------|--------|
| Logs | slog → Loki (optional) |
| Metrics | Prometheus → Grafana |
| Tracing | OpenTelemetry → Tempo or Jaeger |
| Eval harness | DeepEval + custom citation-faithfulness scorer |

### Deployment (V1 target: self-hosted team, docker-compose or k8s)

| Concern | Choice |
|---------|--------|
| Dev stack | docker-compose.yml (Qdrant, Meilisearch, PG, SearXNG, LiteLLM, Redis) |
| Team deploy | Helm chart under `deploy/k8s/` |
| Container registry | ghcr.io |
| Update channel | semantic version tags on `main` |

## 4. Per-Source Adapter Strategy

| Source | Approach | Auth | Rate limit | Notes |
|--------|----------|------|------------|-------|
| Reddit | public JSON API | anonymous UA | 60/min | fallback to Pushshift mirror if needed |
| X / Twitter | ScrapeCreators API (last30days pattern) or Nitter | API key | per-plan | no official API for deep search in 2026 tier |
| Hacker News | Algolia HN API | none | generous | stable, no-auth |
| YouTube | yt-dlp (metadata + transcript) | none | self-throttle | transcript via `--write-auto-subs` |
| Bluesky | AT Protocol public feed | anonymous | generous | |
| TikTok / Instagram / Threads | ScrapeCreators API | API key | per-plan | optional, keyed feature flag |
| arXiv | OAI-PMH + arXiv Search API | none | 3s between req | |
| GitHub | GitHub REST + Search API | PAT per team | 5000/hr with auth | wrap via `github/github-mcp-server` |
| Paper search | wrap `openags/paper-search-mcp` | per-source | varies | Crossref / OpenAlex / Semantic Scholar |
| Polymarket | public API | none | generous | |
| Naver (web/news/blog/shopping) | wrap `isnow890/naver-search-mcp` | Naver API key | 25000/day | Korean-locale primary |
| 다음 / KoreaNewsCrawler | wrap `lumyjuwon/KoreaNewsCrawler` | none | scraper-style | fallback Korean news |
| RSS (user-configured) | Tanuki / lightweight gofeed | none | 5min cache | for internal feeds |
| General web | SearXNG fanout | none | SearXNG handles | primary web source |

## 5. Hybrid Ranking

```
    Query
      │
      ▼
 ┌─────────────┐   ┌──────────────┐   ┌─────────────┐
 │ Meili BM25  │   │ Qdrant dense │   │ FastEmbed   │
 │   (lexical) │   │   (semantic) │   │   sparse    │
 └──────┬──────┘   └───────┬──────┘   └──────┬──────┘
        │                  │                 │
        └────────┬─────────┴─────────────────┘
                 ▼
        Reciprocal Rank Fusion (RRF)
                 ▼
        [optional] BGE-reranker-v2-m3 cross-encoder
                 ▼
           Top-K candidates → Synthesis
```

RRF formula: `score(d) = Σ 1 / (k + rank_i(d))` with `k=60` by convention.

## 6. Risks & Mitigations

| Risk | Severity | Mitigation |
|------|----------|------------|
| ToS violation on scraped sources (X, Instagram) | High | Feature-flag behind team opt-in; default to API-based adapters only |
| SearXNG AGPL contagion if ever offered as SaaS | Medium | V1 is self-hosted only; SaaS path re-evaluates licensing |
| LLM cost blowout on `/deep` | Medium | Per-user `/deep` quota; Haiku pre-screen; prompt caching |
| Korean tokenizer mismatch (Meili default weak for ko) | Medium | mecab-ko sidecar; separate Korean index shard |
| Qdrant + Meili + PG triple-ops burden for small teams | Medium | `deploy/docker-compose.yml` one-liner; optional "lite" profile with LanceDB + SQLite |
| Citation faithfulness drift with new LLM versions | Medium | DeepEval gate in CI against 50-query golden set |
| 5-phase access fallback brittleness | Low | Phase 3 (Playwright) is bounded behind feature flag and per-query budget |
| Upstream (gpt-researcher / STORM) breaking changes | Low | Pin exact versions; upgrade under phase-specific SPEC |

## 7. Decision Log (initial)

| Date | Decision | Rationale |
|------|----------|-----------|
| 2026-04-24 | Framework base = SearXNG + gpt-researcher + STORM | User selected over scratch-build / Perplexica fork |
| 2026-04-24 | LLM policy = mixed routing via LiteLLM | cost + flexibility + privacy tiers |
| 2026-04-24 | Index = Qdrant + Meilisearch + PostgreSQL | proven multi-tenant, hybrid retrieval |
| 2026-04-24 | V1 scope = full (deep mode + team + shared index) | user explicitly selected over phased MVP |
| 2026-04-24 | License target = Apache-2.0 | compatible with all upstream, permits commercial |
| 2026-04-24 | Primary orchestration language = Go | existing MoAI stack convention, powernap LSP |
| 2026-04-24 | SearXNG as service, not fork | AGPL boundary preserved |
| 2026-04-24 | Korean tokenization = mecab-ko sidecar | Meili default tokenizer is weak for Korean |

Subsequent decisions append to this table, never overwrite.
