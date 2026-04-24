# Research — Similar Repos & Competitive Landscape

Research performed 2026-04-24 before project scoping, to determine whether any existing project already covers the Universal Search target, and to source battle-tested components for reuse.

## 1. Three user-provided seeds

| Repo | Lineage | Role in Universal Search |
|------|---------|--------------------------|
| **fivetaku/insane-search** | Python access layer (MIT) | **Pattern reference only** — adopt the 5-phase access fallback idea (index → probe → TLS → browser) for `internal/access/`. Not bundled. |
| **getcompanion-ai/feynman** | TypeScript research CLI (MIT) | **Pattern reference only** — adopt multi-agent roles (Researcher / Reviewer / Writer / Verifier) for `/deep` mode. Built on proprietary Pi runtime; not reusable as-is. |
| **mvanhorn/last30days-skill** | Python/Node Claude Skill (MIT) | **Pattern reference only** — adopt engagement-based ranking for social sources (Reddit / X / YT / HN / Polymarket). ScrapeCreators API integration informs our optional-adapter strategy. |

Conclusion: **none of the three is a drop-in base.** Each solves one slice. We lift patterns, not code.

## 2. Stronger base candidates discovered

| Repo | License | Why it's a stronger base than the three seeds |
|------|---------|-----------------------------------------------|
| **assafelovic/gpt-researcher** | Apache-2.0 | Full planner-executor-publisher, `gptr-mcp` MCP server, citation tracking, OpenAI-compatible endpoint support (local models), doc/CSV/MD/PPTX ingestion. **Adopted as the synthesis base.** |
| **searxng/searxng** | AGPL-3.0 | 70+ search engine providers, privacy-first, self-hostable, no API keys needed. **Adopted as the metasearch fanout layer** (run as a service, not linked — preserves AGPL boundary). |
| **stanford-oval/storm** | MIT | Long-form knowledge curation with per-claim citations. **Adopted as the `/deep` report generator.** |
| **ItzCrazyKns/Perplexica (now Vane)** | MIT | OSS Perplexity clone — SearxNG + embeddings + reranking. Considered as full base; rejected because TypeScript stack conflicts with MoAI's Go/Python convention and Perplexica has limited team/multi-tenant features. |
| **Open Deep Search (arxiv 2503.20201)** | research paper | "Open Search Tool + Open Reasoning Agent" pattern informs our Intent Router. Not a deployable repo. |

## 3. Domain-specific components adopted

### Academic / paper search

- **openags/paper-search-mcp** (MCP) — arXiv / PubMed / bioRxiv / Crossref / OpenAlex / Semantic Scholar. **Adopted as SPEC-ADP-003 backend.**
- **github/github-mcp-server** (official) — GitHub repos / issues / PRs / code. **Adopted as SPEC-ADP-004 backend.**
- **zilliztech/claude-context** — semantic code search MCP. **Not adopted for V1** (complementary, not competitor; users can run both).

### Korean locale

- **isnow890/naver-search-mcp** (MIT, MCP) — Naver web/news/blog/shopping + DataLab trends. **Adopted as SPEC-ADP-008 backend.**
- **lumyjuwon/KoreaNewsCrawler** (Python) — Naver News category crawler across 1000+ Korean outlets. **Adopted as SPEC-ADP-009 supplement.**
- **Naver official Search API + searchad-apidoc** — primary official channel, keyed.
- SerpApi / SearchAPI / Apify — third-party SERP services. **Not adopted** (avoid vendor lock-in and recurring cost for scrape-equivalent work).

### Real-time / social (last30days lineage)

- **ScrapeCreators API** — TikTok / Instagram / Threads (used by last30days). **Optional adapter, feature-flagged**, due to ToS considerations.
- Reddit public JSON API, Hacker News Algolia API, Bluesky AT Protocol, yt-dlp — no-key or low-friction, adopted directly.
- Polymarket public API — adopted as signal source (prediction odds as engagement proxy).

### Retrieval infrastructure

- **Qdrant v1.16+** (Apache-2.0) — Tiered Multitenancy released March 2026, ideal for team-shared index with per-team isolation. **Adopted.**
- **Meilisearch v1.10+** (MIT) — BM25 + vector hybrid, multi-tenant tokens. **Adopted for lexical side of hybrid retrieval.**
- **OpenSearch** — considered as single-stack alternative; rejected because Qdrant's multitenancy + Meili's developer ergonomics win for this use case.
- **pgvector** — kept as emergency fallback inside Postgres, not primary.
- **LanceDB + SQLite** — kept as "lite profile" option for small teams in docs (not default).
- **Pinecone / Typesense Cloud** — SaaS; rejected (self-hosted-first for V1).

### Retrieval technique

- **Reciprocal Rank Fusion (RRF)** — adopted as the hybrid fusion method (k=60).
- **BGE-M3** dense embeddings + **FastEmbed BM25** sparse + optional **BGE-reranker-v2-m3** cross-encoder. This follows the `CortexReach/memory-lancedb-pro` and Qdrant+RRF patterns documented in 2026.

### LLM routing

- **LiteLLM proxy** (MIT) — single endpoint, per-key provider routing, cost tracking. **Adopted as SPEC-LLM-001 backbone.**
- **Ollama** / **vLLM** — local fallback, feature-flagged.
- **Anthropic Claude API (primary)**, **OpenAI-compatible APIs (secondary)** — routed via LiteLLM.

### Agent orchestration patterns

- **STORM + gpt-researcher combined 4-agent** (Researcher / Reviewer / Writer / Verifier) — adopted for `/deep` (SPEC-DEEP-002).
- **wshobson/agents** (Claude Code multi-agent orchestration) — informs our CLI agent wiring; not bundled.
- **Orchestra-Research/AI-Research-SKILLs** — skill library for AI agents; informs our Claude Skill packaging.

### Access / anti-block

- insane-search 5-phase pattern — reimplemented in Go (`internal/access/`) with:
  - Phase 0: index lookup (shared index hit)
  - Phase 1: probe (WebFetch, Jina Reader, UA variants)
  - Phase 2: TLS impersonation (Go equivalent of curl_cffi — `utls` library)
  - Phase 3: **Playwright MCP** bridge (direct reuse, not reimplementation)
- **Jina Reader** — adopted as fallback readability service.
- **Wayback Machine + archive.today** — adopted as historical-snapshot fallbacks.

## 4. Gap confirmation

After the above survey, the **gap** Universal Search fills remains:

- No existing project couples (team-scale shared index) × (Korean-locale first-class) × (social+academic+web breadth) × (basic+`/deep` synthesis) × (CLI+MCP+Skill+UI surfaces).
- Closest composite would be "deploy Perplexica + add Korean adapters + graft STORM + build team auth" — which amounts to roughly 60% of V1's scope reassembled from scratch anyway. Our composition is cleaner.

## 5. Sources

- [fivetaku/insane-search](https://github.com/fivetaku/insane-search)
- [getcompanion-ai/feynman](https://github.com/getcompanion-ai/feynman)
- [mvanhorn/last30days-skill](https://github.com/mvanhorn/last30days-skill)
- [assafelovic/gpt-researcher](https://github.com/assafelovic/gpt-researcher)
- [searxng/searxng](https://github.com/searxng/searxng)
- [openags/paper-search-mcp](https://github.com/openags/paper-search-mcp)
- [github/github-mcp-server](https://github.com/github/github-mcp-server)
- [zilliztech/claude-context](https://github.com/zilliztech/claude-context)
- [isnow890/naver-search-mcp](https://github.com/isnow890/naver-search-mcp)
- [lumyjuwon/KoreaNewsCrawler](https://github.com/lumyjuwon/KoreaNewsCrawler)
- [ItzCrazyKns/Perplexica / Vane](https://github.com/ItzCrazyKns/Perplexica)
- [Open Deep Search paper (arxiv 2503.20201)](https://arxiv.org/html/2503.20201v1)
- [Qdrant multitenancy 2026](https://kulekci.medium.com/multi-tenant-vector-search-in-practice-building-a-shared-knowledge-base-with-qdrant-7b7928ba00fe)
- [Meilisearch hybrid + multi-tenancy](https://www.meilisearch.com/blog/how-do-you-search-in-a-database-with-llm)
- [CortexReach/memory-lancedb-pro (hybrid Vec+BM25 pattern reference)](https://github.com/CortexReach/memory-lancedb-pro)
- [Hybrid Search BM25 + dense (LanceDB blog)](https://www.lancedb.com/blog/hybrid-search-combining-bm25-and-semantic-search-for-better-results-with-lan-1358038fe7e6)
- [Claude Deep Research MCP](https://github.com/mcherukara/Claude-Deep-Research)
- [STORM (Stanford)](https://github.com/stanford-oval/storm)
