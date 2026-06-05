# Product — Universal Search Engine

## 1. Identity

**Universal Search** is a team-scale, source-grounded research meta-agent. It unifies web, social, academic, technical, and Korean-locale retrieval behind a single query plane, then routes results through hybrid LLM synthesis (basic summarization by default, `/deep` multi-agent deep-research on demand).

The system exposes multiple surfaces: a CLI (`usearch`), an HTTP API (`usearch-api`), an MCP server, and a Web UI. All surfaces share the same backend pipeline and team-scoped hybrid index.

It is a composition — not a fork — of three proven open-source lineages:

- **Access layer**: inspired by `fivetaku/insane-search` (5-phase adaptive scheduler, TLS fingerprint spoofing, Playwright fallback)
- **Research orchestration**: built on `assafelovic/gpt-researcher` (Apache-2.0 planner-executor-publisher workflow, citation tracking, MCP server `gptr-mcp`)
- **Breadth / real-time signal**: inspired by `mvanhorn/last30days-skill` (social-engagement-scored aggregation across Reddit / X / YouTube / HN / Polymarket / GitHub)
- **Citation report generation**: inspired by Stanford **STORM** (long-form knowledge curation with per-claim provenance)
- **Metasearch fanout**: **SearXNG** (70+ engine providers, privacy-preserving, self-hostable)

## 2. Problem Statement

Research work today is fragmented:

- Users context-switch across 5–10 tabs (Google, Reddit, arXiv, GitHub, Naver, YouTube, internal docs)
- Each search returns raw links — synthesis is manual and slow
- Existing "AI search" products (Perplexity, ChatGPT Search) are **single-tenant, closed, non-auditable**, and weak on Korean-locale sources and long-tail social platforms
- Teams have no shared, auditable research trail — the same questions get re-asked, answers drift, provenance disappears

Universal Search solves this by giving a team **one query → many sources → one cited synthesis → one shared, re-queryable memory**.

## 3. Primary Personas

| Persona | Need | Success Metric |
|---------|------|----------------|
| **Research-heavy engineer** | Technical deep-dives spanning GitHub, arXiv, docs, HN | Finds grounded answer in ≤60s that previously took ≥15min |
| **Product / strategy lead** | Market + social + news signal, Korean + global | Weekly report assembled in <10min with cited sources |
| **Korean analyst / journalist** | Naver news, 다음, Korean RSS, cross-referenced with global sources | Korean-first query returns Korean sources first, not translated summaries |
| **Team lead (shared memory)** | Team's prior queries become re-searchable asset | "Did anyone already research X?" answerable from shared index |

## 4. Scope — V1 (Full)

**In scope for V1** (per decisions 2026-04-24):

- 4 source categories: web+social, academic+technical, Korean-locale, personal-context (read-only, opt-in per user — scoped for V1.1 gate)
- Hybrid synthesis: default summarize-and-cite, `/deep` escalates to multi-agent pipeline (Researcher → Reviewer → Writer → Verifier)
- Team auth + shared hybrid index (Qdrant Tiered Multitenancy + Meilisearch + Postgres)
- Multiple surfaces: CLI (`usearch query`, `usearch deep`), HTTP API (`/search`, `/sources`, `/health`), MCP server, Claude Skill, Web UI
- Sources management: Registry-backed `sources list/status/show` commands with health checks and timeout support (SPEC-CLI-003)
- Mixed LLM routing via LiteLLM proxy (Claude primary, OpenAI-compatible, local Ollama/vLLM fallback)
- Observability: query log, citation-faithfulness eval, per-source success rate

**Out of scope for V1**:

- SaaS / public multi-tenant hosting (organization-scale self-hosted only)
- Write-back to external systems (purely read / retrieve / synthesize)
- Image / video generation
- Agentic task execution beyond retrieval (no "book flight", no "send email")

## 5. Non-Goals

- **Not** a Google replacement — no independent crawl / index of the open web
- **Not** a SaaS product (V1 is self-hosted for one team / org)
- **Not** a coding assistant (code-search MCPs like `zilliztech/claude-context` are complementary, not replaced)
- **Not** a personal productivity suite (Gmail / Drive / Calendar integration is explicitly deferred past V1 per user decision)

## 6. Success Metrics (V1 exit criteria)

| Metric | Target |
|--------|--------|
| Median query latency (basic synth) | ≤ 8s p50, ≤ 20s p95 |
| `/deep` latency | ≤ 5min p50 |
| Citation faithfulness (automated eval) | ≥ 0.85 |
| Source adapter success rate | ≥ 95% per adapter over 7-day window |
| Sources health check latency | ≤ 2s p50 |
| Duplicate-query dedup hit rate | ≥ 30% within team-shared index |
| LLM cost per basic query | ≤ $0.02 |
| LLM cost per `/deep` query | ≤ $0.50 |
| Korean-locale result ranking relevance | manual eval ≥ 4/5 on 50-query benchmark |

## 7. Differentiation vs Closest Competitors

| Competitor | What it does | What Universal Search adds |
|------------|--------------|----------------------------|
| Perplexity | Web search + citations | Shared team index, Korean-first, `/deep` multi-agent, self-hostable, MCP surface |
| GPT Researcher (alone) | Deep web research, single-user | Team auth, shared index, Korean sources, SearXNG fanout, social engagement signal |
| SearXNG (alone) | Privacy metasearch | LLM synthesis, citation, shared memory, social sources, STORM reports |
| last30days-skill | Social 30-day window | Full research pipeline, academic depth, deep-research mode, team scale |
| Danswer / Onyx | Enterprise internal search | External-source-first (web/social/academic) rather than internal-doc-first |
| Perplexica | OSS Perplexity clone | Korean sources, team multi-tenancy, MCP / Claude Skill surface, STORM-style reports |

The defensible wedge is the **composition**: no single existing OSS project covers (team-scale shared index) × (Korean-locale) × (social+academic+web) × (default+deep synthesis modes) × (multi-surface: CLI+MCP+Skill+UI).

## 8. Upstream Licenses (V1 dependency audit)

All upstream components are permissive and commercially usable:

- gpt-researcher — Apache-2.0
- SearXNG — AGPL-3.0 (self-hosted use is compliant; SaaS offering of modified version requires source release)
- STORM — MIT
- insane-search — MIT (pattern reference only; not bundled)
- last30days-skill — MIT (pattern reference only; not bundled)
- Qdrant — Apache-2.0
- Meilisearch — MIT (engine); commercial cloud requires license for some features
- LiteLLM — MIT
- naver-search-mcp (isnow890) — MIT (pattern reference; Naver API keys required)

Project license target: **Apache-2.0** (compatible with all, permits commercial redistribution).

**SearXNG AGPL caveat**: V1 runs SearXNG as a dependency service (separate process, not linked code). Internal team use is compliant. If Universal Search is ever offered as SaaS with modified SearXNG, source release is required.
