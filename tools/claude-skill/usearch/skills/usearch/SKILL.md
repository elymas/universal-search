---
description: "Universal Search -- team-scale research agent. Use when the user asks a research-style question needing citations, multi-source synthesis across web/social/academic/Korean sources, a long-form deep-research report, citation drill-down on a specific source, or wants to reuse prior team queries. Invokes the Universal Search MCP server for sourced answers."
---

# Universal Search

Universal Search is a research meta-agent that fans out across web, social, academic, and Korean-locale sources, synthesizes results with inline citations, and supports deep-research report generation. It connects to your team's Universal Search MCP server.

## Tool Selection Guide

When the user's query matches one of the patterns below, invoke the corresponding MCP tool. All tools are discovered automatically via the MCP `tools/list` handshake when the `usearch-mcp` server is connected.

### `search` -- Basic research synthesis

Use when the user asks a factual question that benefits from multiple sources, citations, or cross-referencing. Examples:

- "What are the latest developments in quantum error correction?"
- "Compare React vs Vue performance benchmarks"
- "Summarize recent AI regulation news with sources"

The `search` tool returns a synthesized answer with numbered citations linking back to source documents.

### `deep_research` -- Long-form report generation

Use when the user explicitly asks for a comprehensive, thorough, or detailed report -- or mentions "for the team", "deep dive", or uses the `/deep` keyword. Examples:

- "Give me a comprehensive report on quantum computing for the team"
- "Write a deep analysis of semiconductor supply chain risks"
- "/deep investigate the current state of CRISPR gene therapy"

The `deep_research` tool produces a STORM-style multi-section report with full source attribution. This is significantly more expensive than `search` -- reserve for queries that genuinely need long-form output.

### `list_sources` -- Adapter discovery

Use when the user asks what sources or adapters are available. Examples:

- "What sources can you search?"
- "Which academic databases do you have access to?"
- "Show me available Korean news sources"

The `list_sources` tool returns the configured adapter registry, grouped by category (web, social, academic, Korean, etc.).

### `get_citation` -- Citation drill-down

Use when the user references a specific source from a prior answer and wants more detail. Examples:

- "Tell me more about source [3] from your last answer"
- "Show me the full text of citation 2"
- "Get the details on that Naver article you cited"

The `get_citation` tool takes a `doc_id` from a previous response and returns the full NormalizedDoc with metadata, full text, and provenance.

### Korean-language queries

When the user's query contains Korean characters or explicitly references Korean sources (e.g., "Naver news", Korean-language topics), the server auto-routes to Korean-locale adapters (Naver, Daum, KoreaNewsCrawler, Korean RSS) via the intent router. No special tool flag or argument is needed -- just call `search` or `deep_research` as you normally would. Examples:

- "AI 뉴스 검색해줘" (searches Korean sources automatically)
- "Find Korean perspectives on AI regulation"
- "Naver에서 최근 부동산 뉴스 찾아줘"

## Error Handling

When a tool call returns an error from the Universal Search error namespace, surface it to the user with an actionable next step.

### `-32002 usearch.unauthorized`

The server rejected the request because authentication is not configured.

**Tell the user:** "Universal Search is not configured yet. Run `usearch config init` to set up your endpoint and authentication, then retry."

### `-32000 usearch.cap_exceeded`

The daily quota for the requested operation has been reached.

**Tell the user:** "Daily limit reached for this operation. The limit resets at the time shown in the error. Try again after the reset, or ask your team admin about increasing the quota."

### `-32007 usearch.citation_not_found`

The requested citation `doc_id` does not exist or has expired from the cache.

**Tell the user:** "That citation is no longer available (it may have expired from the session cache). Try running the search again to get fresh results with new citations."

## Configuration

Universal Search reads its configuration from `~/.config/usearch/config.toml`. Before using this plugin:

1. Install the `usearch-mcp` binary (see the plugin README for install methods).
2. Run `usearch config init` to configure endpoints and authentication.
3. Verify with `usearch-mcp --version`.

The plugin launches `usearch-mcp` from your PATH via stdio transport. No credentials are embedded in the plugin -- all auth flows through the server's own config.
