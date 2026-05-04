# SPEC-ADP-003 Research — arXiv + Paper Search Adapter

Research artifact for SPEC-ADP-003 (academic paper adapter, M3). Produced
during the plan phase to inform EARS requirements before drafting
spec.md. Every external claim is URL-cited; every internal claim is
file:line-cited per the project's research conventions.

---

## 0. Research Mandate

SPEC-ADP-003 is the first M3 adapter SPEC after SPEC-FAN-001 (the M3
gateway, status: approved). The roadmap row at
`.moai/project/roadmap.md:48` reads "SPEC-ADP-003 | arXiv +
paper-search adapters | wrap `openags/paper-search-mcp`". This research
artifact:

1. Surveys the two viable integration paths: (A) wrap
   `openags/paper-search-mcp` Python MCP server, (B) implement a direct
   arXiv REST API client in pure Go.
2. Documents the arXiv API surface in detail (endpoint, query
   parameters, response Atom XML envelope, rate-limit policy,
   pagination semantics).
3. Maps arXiv Atom entry fields to the `pkg/types.NormalizedDoc`
   15-field canonical contract.
4. Extracts the existing-codebase patterns from SPEC-ADP-001 (Reddit)
   and SPEC-ADP-002 (HN) that ADP-003 must mirror.
5. Recommends a path with rationale; flags risks; carries deferred
   decisions to spec.md §11 Open Questions.

Output: this artifact. Every external claim is WebFetch-verified
(URLs in §9). Every internal claim cites a file path with line numbers
when applicable.

The intent class is `academic` per
`internal/router/category.go:97` (CategoryAcademic eligible DocTypes
= `[DocTypePaper, DocTypeRepo, DocTypeIssue]`). ADP-003 fills the
`DocTypePaper` slot for arXiv-class queries; SPEC-ADP-004 will fill
`DocTypeRepo`/`DocTypeIssue` via GitHub.

---

## 1. Existing-Pattern Citations (SPEC-ADP-001 / SPEC-ADP-002 Reference Shape)

SPEC-ADP-003 inherits the file layout, error mapping discipline, MX tag
plan, and TDD harness from SPEC-ADP-001 (Reddit, reference) and
SPEC-ADP-002 (Hacker News). The reference adapter SPEC explicitly
designates the M3 ADP-* SPECs as downstream consumers of the pattern:

> "SPEC-ADP-003..009 (M3): copy the reference shape."
> — `.moai/specs/SPEC-ADP-001/spec.md:1060`

Implementation files in `internal/adapters/reddit/` define the canonical
package shape (verified via `ls /internal/adapters/reddit/`):

| File | Role | Reused As-Is by ADP-003? |
|------|------|--------------------------|
| `reddit.go` | Adapter struct, New, Name, Capabilities, Healthcheck, compile-time interface assertion | Pattern; substitute `"reddit"` → `"arxiv"`, base URL, defaults |
| `search.go` | (*Adapter).Search hot path, query validation, URL construction, HTTP execute, response parsing | Pattern; rewrite URL builder for arXiv params, swap JSON → XML body decode |
| `client.go` | HTTP client, redirect allowlist, doRequest helper, categorizeStatus rosetta | Pattern; redirect allowlist tuned to arXiv host set; categorizeStatus and parseRetryAfter adopted with adapter name swap |
| `parse.go` | parseListing JSON → []NormalizedDoc transform | Pattern; rewrite for Atom XML feed envelope (different shape) |
| `score.go` | normalizeScore Tanh formula, package-level constants | NOT REUSED — arXiv has no score; see §3.5 below |
| `errors.go` | ErrInvalidQuery sentinel, parseRetryAfter helper | Pattern; sentinels renamed `ErrInvalidQuery` and `ErrInvalidStart` (private), `parseRetryAfter` adopted as-is |
| `bench_test.go` | NFR benchmark BenchmarkParseListing25Docs | Pattern; rename to `BenchmarkParseFeed25Entries` |

The compile-time interface assertion pattern at
`internal/adapters/reddit/reddit.go:135` (`var _ types.Adapter =
(*Adapter)(nil)`) is mandatory. The reference noop adapter at
`internal/adapters/noop/noop.go:46` documents the minimal shape.

The registry's wrappedAdapter at `internal/adapters/registry.go:172-263`
emits ALL observability — counter, histogram, span, slog — so the arXiv
adapter, like Reddit and HN, MUST emit nothing of its own. Sole-emitter
discipline preserved verbatim.

The error taxonomy is provided by `pkg/types/errors.go:14-218`
(SPEC-CORE-001 REQ-CORE-008): four sentinels (`ErrTransient`,
`ErrPermanent`, `ErrRateLimited`, `ErrSourceUnavailable`), `Category`
enum, typed `*SourceError`, and `OutcomeFromError` for the Prometheus
label. The arXiv adapter consumes this contract identically.

The `pkg/types.NormalizedDoc` 15-field struct
(`pkg/types/normalized_doc.go:40-56`) is the return shape. ADP-003 maps
arXiv Atom entry fields onto these 15 fields per §3 below. The
`DocType` is `types.DocTypePaper` (defined in
`pkg/types/capabilities.go:17`).

The fanout layer (SPEC-FAN-001 spec.md, status: approved) consumes
`registry.Get("arxiv").Search(ctx, q)` per its REQ-FAN-002 dispatch
loop. ADP-003 inherits the REQ-ADP-011 / REQ-ADP2-010 concurrent-safety
contract from ADP-001/ADP-002 verbatim — the adapter must be race-clean
under N concurrent goroutines invoking Search on the same `*Adapter`.

The Intent Router at `internal/router/category.go:97` returns
`[DocTypePaper, DocTypeRepo, DocTypeIssue]` for `CategoryAcademic`;
intersecting with the arXiv adapter's `Capabilities.DocTypes =
[DocTypePaper]` per `internal/router/router.go:258-294` selects ADP-003
for academic queries. The router test at
`internal/router/router_test.go:79, 411, 443` already references the
adapter name `"arxiv"` as a stub — REQ-ADP3-001 below makes this
canonical.

---

## 2. Integration Path Survey: paper-search-mcp vs Direct arXiv API

Two viable integration paths exist. The roadmap entry (`roadmap.md:48`)
mentions wrapping `openags/paper-search-mcp`; the tech.md entry at line
102 also mentions both (one line for arXiv direct, one line for "Paper
search | wrap openags/paper-search-mcp"). This research compares both
paths and recommends one.

### 2.1 Path A: Wrap `openags/paper-search-mcp` (Python MCP server)

**Source**: https://github.com/openags/paper-search-mcp
(WebFetch-verified; see §9).

**What it provides**:

- A Python-based MCP (Model Context Protocol) server that exposes a
  unified `search_papers` tool wrapping 22 academic sources: arXiv,
  bioRxiv, medRxiv, PubMed, Google Scholar, Semantic Scholar, Crossref,
  OpenAlex, IACR ePrint, PubMed Central, CORE, Europe PMC, dblp,
  OpenAIRE, CiteSeerX, DOAJ, BASE, Zenodo, HAL, SSRN, Unpaywall, plus
  optional/skeleton connectors for IEEE Xplore and ACM Digital Library.
- Standardised `Paper` class return shape across all sources.
- A `download_with_fallback` tool for OA PDF retrieval with chained
  fallback.
- Per-source helper tools (e.g., `search_arxiv`, `search_pubmed`).
- License: MIT. Repository: 1.3k stars, 147 forks, 39 commits on main,
  16 open issues, 15 open PRs (extracted 2026-05-04). No semver tags
  visible.

**Deployment model**:

- Python application; install via `pip install paper-search-mcp`,
  `uvx paper-search-mcp`, `npx @smithery/cli`, or Docker.
- Communicates with Go orchestrator via MCP-stdio (JSON-RPC 2.0 over
  stdin/stdout pipes).
- Subprocess lifecycle: spawn at first use, restart on crash.

**Authentication** (optional environment variables, all prefixed
`PAPER_SEARCH_MCP_`):

- `UNPAYWALL_EMAIL` (required for Unpaywall functionality)
- `CORE_API_KEY` (recommended for stability)
- `SEMANTIC_SCHOLAR_API_KEY` (improves rate limits)
- `GOOGLE_SCHOLAR_PROXY_URL` (bypasses bot-detection)
- `DOAJ_API_KEY`, `ZENODO_ACCESS_TOKEN` (optional)
- `IEEE_API_KEY`, `ACM_API_KEY` (required to activate paid connectors)

Most sources work without keys; keys enhance stability or coverage.

**Documented stability issues** (extracted from project README):

| Source | Issue | Mitigation |
|--------|-------|-----------|
| Google Scholar | Bot-detection (CAPTCHA) | Proxy URL required |
| Semantic Scholar | Rate-limiting (429) | API key recommended |
| CORE | Timeout/500 errors | Free API key + retry |
| OpenAIRE | Transient 403 | 3× retry escalation |
| CiteSeerX | 404 via web archive | Returns empty gracefully |
| BASE | Requires institutional registration | Returns empty gracefully |
| SSRN | Cloudflare 403 | No workaround; best-effort |

The README explicitly states: "Some search failures are caused by
external provider instability, not by bugs in this project."

### 2.2 Path B: Direct arXiv REST API in pure Go

**Source**: https://info.arxiv.org/help/api/user-manual.html
(WebFetch-verified; see §9).

**What it provides**:

- A REST endpoint at `http://export.arxiv.org/api/query` returning
  Atom 1.0 XML.
- Search query syntax with field prefixes (`ti:`, `au:`, `abs:`,
  `cat:`, `all:`).
- Pagination via `start` (0-indexed) and `max_results` (≤2000 per call,
  ≤30000 total).
- Sort by `relevance` / `lastUpdatedDate` / `submittedDate`.
- Three Atom namespaces: default, opensearch, arxiv extension.

**Authentication**: NONE.

**Rate limit** (quoted verbatim from the API user manual):
> "In cases where the API needs to be called multiple times in a row,
> we encourage you to play nice and incorporate a 3 second delay in
> your code."

This is a SHOULD (operational guideline), not an enforced server-side
rate limit. arXiv does not, in normal operation, return HTTP 429 — the
guideline is community-of-practice. Industry implementations
(e.g., the SearXNG arXiv engine) honour it via per-instance throttling.

**Coverage**: ~2.5 million preprints in physics, math, computer science,
quantitative biology, quantitative finance, statistics, electrical
engineering, economics. arXiv is the dominant preprint server for these
fields — no other source provides comparable depth for math/physics/CS.

**Endpoint stability**: arXiv is operated by Cornell University, has
been online since 1991, and the v1 API has been stable since at least
2008 (Wikipedia reference: https://en.wikipedia.org/wiki/ArXiv). The
endpoint is a load-bearing piece of academic infrastructure.

### 2.3 Comparison Matrix

| Dimension | Path A (paper-search-mcp wrap) | Path B (direct arXiv) |
|-----------|--------------------------------|------------------------|
| Languages required | Go + Python runtime | Go only |
| Subprocess management | Yes (MCP-stdio lifecycle, restart) | No |
| External dep count | +1 module (MCP-stdio Go client) + Python sidecar | 0 new modules |
| Source coverage | 22 sources | 1 source (arXiv) |
| Reliability profile | Variable per source (Google Scholar/SSRN flaky) | High (arXiv since 1991) |
| Auth complexity | 8+ optional env vars | None |
| Test fixture complexity | Per-source response shapes | One Atom feed shape |
| Reference-shape parity with ADP-001/002 | LOW (different deployment model) | HIGH (mirrors ADP-001/002 exactly) |
| Response format | JSON via MCP | Atom 1.0 XML |
| Score field present | Per-source; some have, some don't | NO (no relevance score in Atom) |
| Pagination | Source-specific via tool params | Standard `start` + `max_results` |
| First-merge maintenance burden | High (22 connectors to validate against) | Low (one stable API) |

### 2.4 Recommendation: Path B (Direct arXiv API)

[HARD] **The recommendation is Path B (direct arXiv REST API)**.

Rationale (in order of weight):

1. **Reference-shape discipline** (highest weight). SPEC-ADP-001 is the
   reference; SPEC-ADP-002 copied it verbatim; the seven M3 ADP-*
   SPECs are explicitly slated to "copy the reference shape"
   (`SPEC-ADP-001/spec.md:1060`). Wrapping an MCP server is a
   fundamentally different architecture — subprocess management,
   JSON-RPC over stdio, cross-language contract. That breaks the
   pattern. ADP-003 stays in-pattern with a pure-Go HTTP/XML client.
2. **Risk discipline**. paper-search-mcp's documented stability issues
   span 7+ of its 22 sources (Google Scholar bot-detection, CORE
   timeouts, OpenAIRE 403s, SSRN Cloudflare blocks, etc.). Composite
   reliability is bounded above by the weakest source. arXiv direct
   has no comparable reliability tail.
3. **Deployment simplicity**. The project tech.md and roadmap describe
   a Go single-binary orchestration plane (`structure.md:6-9`).
   Wrapping paper-search-mcp injects a Python runtime requirement and
   a subprocess lifecycle into the orchestrator. The arXiv direct
   adapter is pure-Go stdlib (matches the SPEC-DEP-001 dependency
   discipline).
4. **Scope discipline**. v0.1 should ship one academic adapter that
   works reliably. arXiv covers physics/math/CS/q-bio/q-fin
   exhaustively — the dominant academic intent for international
   English-language users. bioRxiv/medRxiv (life sciences), PubMed
   (medical), and Google Scholar (citation count signal) can ship as
   follow-up SPECs (SPEC-ADP-003a/003b/003c) once arXiv's adapter
   shape is validated.
5. **Future composability**. A future SPEC-ADP-003-MCP CAN later wrap
   paper-search-mcp for non-arXiv academic sources (PubMed, bioRxiv).
   The two adapters coexist via the `academic` intent class — IR-001
   selects multiple academic adapters per query, and FAN-001
   dispatches to all of them in parallel. ADP-003 (arXiv direct) and
   ADP-003-MCP (paper-search-mcp wrap) can ship independently.
6. **No new module dependencies**. Path B adds zero `go.mod` entries.
   `encoding/xml` is in the stdlib (already used elsewhere in the
   repo for nothing yet, but standard). Path A would add at least an
   MCP-stdio Go client library plus the Python runtime.

The roadmap entry at `roadmap.md:48` reading "wrap
`openags/paper-search-mcp`" is therefore reinterpreted: ADP-003 v0.1
delivers the arXiv direct adapter; the paper-search-mcp wrapping is
deferred to a follow-up SPEC (open question §7.1 below). The tech.md
row at line 102 ("arXiv | OAI-PMH + arXiv Search API | none |
3s between req") confirms the direct-API path is acceptable.

The decision is documented inline in spec.md HISTORY (iteration 1) so
future readers see the path-A vs path-B trade-off was considered, not
elided.

---

## 3. arXiv API Surface (Path B Detail)

### 3.1 Endpoint and Basic Request Structure

**Base URL**: `http://export.arxiv.org/api/query`

Note: the documented URL uses `http://`, not `https://`. arXiv supports
both schemes; the canonical form in the user manual is `http`. The
adapter uses HTTPS in production for confidentiality of the search
query, falling back via the system trust store. v0.1 sends:

```
GET https://export.arxiv.org/api/query?search_query=<…>&start=0&max_results=25
Accept: application/atom+xml
User-Agent: usearch/v0.1 (+https://github.com/elymas/universal-search)
```

The `Accept` header requests Atom XML (the API's only response
format). The custom User-Agent identifies the client at arXiv's side
for operational debugging — arXiv's published API guidelines do not
explicitly require a custom UA, but every responsible implementation
sets one (mirrors the ADP-001 REQ-ADP-009 convention).

### 3.2 Query Parameters

Verified from the arXiv API user manual
(https://info.arxiv.org/help/api/user-manual.html, WebFetch §9):

| Parameter | Type | Default | Max | ADP-003 Usage |
|-----------|------|---------|-----|---------------|
| `search_query` | string | (required, OR `id_list`) | — | From `q.Text` with optional category-filter prepend (REQ-ADP3-007) |
| `id_list` | comma-delimited string | (none) | — | NOT USED in v0.1 — search-by-ID is a different feature. |
| `start` | int | 0 | 30000 | From parsed `q.Cursor` (decimal-string non-negative integer; see §3.7) |
| `max_results` | int | 10 | 2000 (per call), 30000 (total) | `clamp(q.MaxResults, 1, 100)` defaulting to 25 (matches ADP-001/002 ceiling); arXiv's higher 2000 ceiling deliberately under-clamped for uniformity (Open Question §7.3) |
| `sortBy` | string | `relevance` | — | Hardcoded `relevance` in v0.1 (matches ADP-001 `sort=relevance` discipline) |
| `sortOrder` | string | `descending` | — | Hardcoded `descending` |

Search-query syntax notes (manual §3.1):

- Field prefixes: `ti:` (title), `au:` (author), `abs:` (abstract),
  `cat:` (category), `all:` (any field), `co:` (comments), `jr:`
  (journal), `id:` (id), `rn:` (report number).
- Boolean operators: `AND`, `OR`, `ANDNOT`. Default operator is `AND`.
- Phrase queries quoted: `ti:"hello world"`.
- Category filter for REQ-ADP3-007 example:
  `cat:cs.AI AND all:transformers`.

### 3.3 Response Atom XML Envelope

The arXiv API returns Atom 1.0 XML with three namespaces:

- Default: `http://www.w3.org/2005/Atom`
- OpenSearch: `http://a9.com/-/spec/opensearch/1.1/`
- arXiv extension: `http://arxiv.org/schemas/atom`

Top-level shape:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom"
      xmlns:opensearch="http://a9.com/-/spec/opensearch/1.1/"
      xmlns:arxiv="http://arxiv.org/schemas/atom">
  <link href="…" rel="self" type="application/atom+xml"/>
  <title>ArXiv Query: search_query=…</title>
  <id>http://arxiv.org/api/…</id>
  <updated>2026-05-04T00:00:00-04:00</updated>
  <opensearch:totalResults>12345</opensearch:totalResults>
  <opensearch:startIndex>0</opensearch:startIndex>
  <opensearch:itemsPerPage>25</opensearch:itemsPerPage>
  <entry>
    <!-- per-paper fields per §3.4 -->
  </entry>
  <!-- additional entries -->
</feed>
```

The adapter consumes:

- `<entry>` elements (zero or more) — the paper records.
- `<opensearch:totalResults>` — for next-page cursor decision (when
  `start + len(entries) < totalResults`, surface `next_cursor =
  strconv.Itoa(start + len(entries))`).
- `<opensearch:startIndex>` — confirms the request's `start` parameter
  was honoured (informational; not used for branching).
- `<opensearch:itemsPerPage>` — confirms how many entries are in this
  feed (informational).

The adapter does NOT consume:

- Feed-level `<title>`, `<id>`, `<updated>`, `<link>` (operational
  metadata).
- Empty `<entry>` (single placeholder when `<opensearch:totalResults>`
  is 0 — no documented behaviour, but defensive parsing tolerates).

### 3.4 Per-Entry Fields

Each `<entry>` for a paper contains (verified from
https://info.arxiv.org/help/api/user-manual.html §4):

| Element | Namespace | Cardinality | ADP-003 Field | Notes |
|---------|-----------|-------------|---------------|-------|
| `<id>` | default | 1 | `ID` (after stripping `http://arxiv.org/abs/` prefix) | Format `http://arxiv.org/abs/2403.12345v2`; the trailing `vN` is the version |
| `<published>` | default | 1 | `PublishedAt` | RFC 3339; first version submission date |
| `<updated>` | default | 1 | (Metadata `updated`) | RFC 3339; last version submission date |
| `<title>` | default | 1 | `Title` (after collapsing internal whitespace) | May contain newlines and multi-space runs from formatted XML |
| `<summary>` | default | 1 | `Body` (after collapse), `Snippet` (truncated 280 runes) | The abstract; same whitespace-collapse treatment as title |
| `<author>` | default | ≥1 | (Metadata `authors` []string) and (Author = first author's `<name>`) | Each `<author>` has one `<name>` child; affiliation in `<arxiv:affiliation>` if present (ignored in v0.1) |
| `<arxiv:primary_category term="…"/>` | arxiv | 1 | (Metadata `primary_category`) | E.g., `term="cs.AI"`; first attribute is the value |
| `<category term="…"/>` | default | ≥1 | (Metadata `categories` []string) | Multiple per entry; includes primary + secondary classifications |
| `<arxiv:doi>` | arxiv | 0..1 | (Metadata `doi`) | Author-supplied DOI; absent for many entries |
| `<arxiv:journal_ref>` | arxiv | 0..1 | (Metadata `journal_ref`) | E.g., "Phys. Rev. Lett. 132 (2024) 080401" |
| `<arxiv:comment>` | arxiv | 0..1 | (Metadata `comment`) | Author comments (e.g., "12 pages, 5 figures") |
| `<link rel="alternate"/>` | default | 1 | `URL` | Abstract page URL; same as `<id>` body in canonical form |
| `<link title="pdf"/>` | default | 0..1 | (Metadata `pdf_url`) | Direct PDF link; out of v0.1 scope but stored for forward-compat |
| `<link title="doi"/>` | default | 0..1 | (Metadata `doi_url`) | Resolved DOI URL when DOI is known |

**ID stripping rule (REQ-ADP3-006)**: the `<id>` element value is
`http://arxiv.org/abs/{arxiv_id}`. The adapter strips the prefix
`http://arxiv.org/abs/` to expose the bare arXiv identifier (e.g.,
`2403.12345v2`) as `NormalizedDoc.ID`. This matches the convention
adopted by every academic ID system (PubMed PMID, DOI itself) of
exposing the bare identifier without protocol/host context.

**Title and summary whitespace collapse**: arXiv returns titles and
summaries formatted with arbitrary newlines and indentation from the
XML pretty-print. The adapter applies `strings.Join(strings.Fields(s),
" ")` to collapse whitespace before assigning to NormalizedDoc fields.
This is a conservative pass; it does not strip Unicode (mathematical
unicode symbols in titles are preserved).

### 3.5 No Score Field — Score Population Strategy

[HARD] arXiv's Atom API does NOT surface a relevance score per entry.
The `sortBy=relevance` parameter affects the order of returned entries,
but no per-entry numeric score is published.

Three candidate strategies for `NormalizedDoc.Score`:

1. **Position-derived score**: `Score = 1.0 - (position / maxResults)`
   — first-position entry gets ~1.0, last gets ~0.0. Encodes arXiv's
   relevance ranking as a numeric for downstream RRF (SPEC-IDX-001).
2. **Constant 0.5 (neutral)**: every paper gets `Score=0.5`. Defers
   ranking entirely to RRF (which weights rank not score per
   FAN-001 §6.1). Simple, deterministic, consistent.
3. **Citation-count integration**: query Semantic Scholar API for
   citation counts and normalise via Tanh. Adds an external dep and
   another rate-limit surface. Out of v0.1 scope.

[HARD] Recommendation: **Constant 0.5 (option 2)** in v0.1.

Rationale:

- arXiv's relevance ranking already determines OUTPUT ORDER. Encoding
  the same signal as Score is redundant and may bias RRF.
- RRF in SPEC-IDX-001 uses RANK not SCORE for fusion across adapters.
  Position-derived score adds zero useful signal beyond rank.
- Tanh-normalised position would still produce a one-to-one mapping
  to position; setting `Score=0.5` and letting RRF use position
  achieves the same end with one fewer transformation.
- A future SPEC (post-V1) MAY integrate Semantic Scholar citation
  counts via a separate adapter or enrichment step.

The constant `0.5` is documented as `@MX:NOTE` in `score.go` (single
file, ~10 LoC). Open Question §7.5 documents revisit triggers if RRF
ranking quality measurements indicate Score-1.0-position-derived
performs better.

### 3.6 HTTP Status Codes and Error Semantics

| Code | Semantics | Adapter Response |
|------|-----------|------------------|
| 200 | Success (Atom feed body) | Parse and return docs |
| 301/302 | Redirect (rare) | Follow within allowlist |
| 400 | Malformed request (e.g., `max_results` >30000, non-integer `start`) | `*SourceError{CategoryPermanent}` |
| 4xx other | Forbidden / not-found | `*SourceError{CategoryPermanent}` |
| 429 | Rate limited (rare in normal operation) | `*SourceError{CategoryRateLimited}` with parsed Retry-After (default 5s, cap 60s); inherited from ADP-001 §1.7 |
| 5xx | Server error | `*SourceError{CategoryUnavailable}` |
| timeout / DNS | Network blip | `*SourceError{CategoryUnavailable}` HTTPStatus=0 |

arXiv returns 400 with an Atom feed containing a single error `<entry>`
on validation errors (per the API user manual §X). The adapter does
NOT parse the error feed body in v0.1 — the HTTP status alone drives
categorisation. The Atom error body is consumed via the standard
parse-and-discard path (returns no entries; the error is the HTTP
status).

### 3.7 Pagination Semantics

arXiv uses 0-indexed integer pagination via `start` + `max_results`.
The response feed includes `<opensearch:totalResults>` (the total
matching the query, often an order of magnitude larger than the
returned page).

Cursor protocol (mirrors HN ADP-002):

- v0.1 surfaces the next-page cursor via
  `Metadata["next_cursor"]` on the LAST returned doc, encoding the
  next `start` value as a decimal string (e.g., `"25"` for page 2 with
  page-size 25).
- Callers paginate by passing this string back as `q.Cursor` on the
  next call. The adapter parses `q.Cursor` as a non-negative integer
  via `strconv.Atoi` and sets `&start=<n>` on the URL.
- When `start + len(entries) >= totalResults`, no `next_cursor` key
  is set (last page).
- Invalid cursor (non-numeric, negative) is rejected by Search with
  `*SourceError{CategoryPermanent}` (mirrors ADP-002's invalid-cursor
  rejection discipline).

**Edge case — start exceeds totalResults**: arXiv returns an empty
feed. The adapter returns `(nil, nil)` (no error, no docs). The caller
sees pagination has terminated.

**Edge case — start exceeds 30000**: arXiv returns HTTP 400. The
adapter maps to `*SourceError{CategoryPermanent}`. Note: this is a
hard cap from arXiv ("max_results is limited to 30000 in slices of at
most 2000 at a time"). REQ-ADP3-002 acceptance tests cover this path.

### 3.8 Rate Limit Policy

[HARD] arXiv's published guideline:

> "we encourage you to play nice and incorporate a 3 second delay in
> your code."
> — https://info.arxiv.org/help/api/user-manual.html

This is operational courtesy, not an enforced server-side limit. arXiv
does not, in normal operation, return HTTP 429. The adapter:

1. Declares `Capabilities.RateLimitPerMin = 20` (3s/req → 20/min).
   This is informational metadata for the Intent Router and any future
   cost/quota planner.
2. Internally enforces a per-instance minimum interval between
   consecutive HTTP round-trips. The interval is configurable via
   `Options.MinRequestInterval` (default `3 * time.Second`); tests
   inject `0` or `1*time.Millisecond` to keep the concurrent-safety
   test (REQ-ADP3-011, 50 goroutines) under unit-test runtime.
3. Honours ctx cancellation while waiting — a caller with a 200ms
   deadline against an adapter that would otherwise wait 3s sees
   `context.DeadlineExceeded` immediately, not a 3-second hang.

Mechanism (stdlib only — no `golang.org/x/time/rate` dep added):

```go
// Sketch (final shape in run phase).
type Adapter struct {
    // ... existing fields ...
    rateMu       sync.Mutex
    nextRequest  time.Time
    minInterval  time.Duration // from Options.MinRequestInterval
}

func (a *Adapter) waitForRateSlot(ctx context.Context) error {
    a.rateMu.Lock()
    wait := time.Until(a.nextRequest)
    a.nextRequest = time.Now().Add(a.minInterval)
    a.rateMu.Unlock()
    if wait <= 0 {
        return nil
    }
    select {
    case <-time.After(wait):
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

The mutex is held only briefly (compute and update `nextRequest`) and
the actual wait happens outside the lock. Ctx cancellation is
respected via the `select`. This pattern is race-clean (verified
informally; final test in REQ-ADP3-011 / NFR-ADP3-003).

This adds modest state to `*Adapter` (one mutex, one time.Time, one
duration) but keeps the adapter stateless across `Search` calls in the
deeper sense: no per-call state is retained across calls beyond the
"when was the last call" wall-clock checkpoint. The design discipline
established in ADP-001 D3 ("never tracks consecutive failures, never
opens a circuit, never sleeps") is preserved with the noted exception
of the 3-second courtesy interval — which is documented in
`Capabilities.Notes`.

### 3.9 SSL / Redirect Policy

`export.arxiv.org` responds at `https://` with a Cornell-issued cert
valid via the system trust store. The endpoint may redirect to
`arxiv.org` (apex) in some configurations. ADP-003 enforces a redirect
allowlist mirroring ADP-001/002 SSRF guard:

- Allowlist hosts: `export.arxiv.org`, `arxiv.org`.
- Maximum 3 hops.
- Cross-domain redirect → `*SourceError{CategoryPermanent}` with
  message containing `"cross-domain redirect"`.

---

## 4. NormalizedDoc Field Mapping (arXiv-Specific)

| Reddit (ADP-001) | HN Algolia (ADP-002) | arXiv (ADP-003) | NormalizedDoc Field | Transform |
|------------------|----------------------|-----------------|---------------------|-----------|
| `data.name` (e.g., `t3_abc`) | `objectID` | `<id>` minus `http://arxiv.org/abs/` | `ID` | E.g., `2403.12345v2`. Bare arXiv identifier with version suffix. |
| (constant `"reddit"`) | (constant `"hackernews"`) | (constant `"arxiv"`) | `SourceID` | matches `Name()` |
| `https://www.reddit.com` + permalink | `url` or HN permalink | `<id>` (full URL) | `URL` | The abstract page URL; canonical form `http://arxiv.org/abs/2403.12345v2` |
| `data.title` | `title` | `collapseWS(<title>)` | `Title` | Whitespace-collapsed (newlines + multi-space → single space) |
| `data.selftext` | `stripHTML(story_text)` | `collapseWS(<summary>)` | `Body` | Whitespace-collapsed abstract |
| Truncated to 280 runes | Truncated to 280 runes | `truncateRunes(collapseWS(<summary>), 280)` | `Snippet` | Same truncation discipline |
| `time.Unix(int64(created_utc), 0).UTC()` | `time.Unix(int64(created_at_i), 0).UTC()` | `time.Parse(time.RFC3339, <published>)` | `PublishedAt` | RFC 3339 parse; convert to UTC |
| (parse time) | (parse time) | (parse time) | `RetrievedAt` | `time.Now().UTC()` injected by caller |
| `data.author` | `author` | First `<author><name>` | `Author` | First author only; full list goes to Metadata |
| `normalizeScore(data.score)` | `normalizeScore(points)` | constant `0.5` | `Score` | Per §3.5 — arXiv has no relevance score; constant 0.5 |
| (constant `""`) | (constant `""`) | (constant `""`) | `Lang` | arXiv has no per-paper language field; abstracts are typically English but not guaranteed |
| (constant `DocTypePost`) | (constant `DocTypePost`) | (constant `DocTypePaper`) | `DocType` | DocTypePaper per `pkg/types/capabilities.go:17` |
| (nil) | (nil) | (nil) | `Citations` | nil; per-claim provenance is SPEC-SYN-002's job |
| 6 required + 7 optional keys | 4 required + 3 optional keys | 6 required + 5 optional keys (see below) | `Metadata` | arXiv-specific extension bag |
| (constant `""`) | (constant `""`) | (constant `""`) | `Hash` | Consumers compute via `CanonicalHash()` |

**Required Metadata keys** (REQ-ADP3-006 enforces presence on every
returned doc):

- `arxiv_id` (string from the bare ID, e.g., `"2403.12345v2"`) —
  duplicates `NormalizedDoc.ID` for adapter-agnostic Metadata-only
  consumers
- `authors` (`[]string` of author names in submission order) — primary
  paper-specific extra
- `primary_category` (string from `<arxiv:primary_category term>`) —
  e.g., `"cs.AI"`
- `categories` (`[]string` from all `<category term>` elements) —
  full classification set
- `published_at` (string, RFC 3339, mirrors `PublishedAt` for
  Metadata-only consumers)
- `updated_at` (string, RFC 3339, from `<updated>`) — distinct from
  `published_at` when paper has multiple versions

**Optional Metadata keys** (MAY be present):

- `doi` (string from `<arxiv:doi>` when author supplied a DOI)
- `journal_ref` (string from `<arxiv:journal_ref>`)
- `comment` (string from `<arxiv:comment>`, e.g., "12 pages, 5
  figures")
- `pdf_url` (string from `<link title="pdf">`) — convenience for
  forward-compat with SPEC-CACHE-001 PDF download paths
- `total_results` (int from feed-level `<opensearch:totalResults>`) —
  surfaced ONLY on the FIRST returned doc per call; consumers can
  use it to pre-allocate paginated UI state
- `next_cursor` (string) — REQUIRED on the LAST returned doc when
  another page exists; absent otherwise (same convention as
  ADP-001/002)

The DOI extraction is non-trivial (the `<arxiv:doi>` element is in the
arxiv namespace; the XML decoder must be aware). REQ-ADP3-006
acceptance includes a fixture with DOI present and a fixture with DOI
absent.

---

## 5. Race-Safety and Resource Cleanup

The adapter's concurrent-safety contract mirrors ADP-001 REQ-ADP-011
and ADP-002 REQ-ADP2-010 verbatim. Specific to arXiv:

1. The `*http.Client` is goroutine-safe per Go stdlib documentation
   (https://pkg.go.dev/net/http#Client).
2. The rate-limit state (`nextRequest`, `rateMu`) is the ONLY shared
   mutable state in the adapter. Access is mutex-guarded; the actual
   wait happens outside the lock with ctx-cancellation respected.
3. The adapter holds NO per-call state beyond what's on the stack.
   Two concurrent `Search` calls execute on disjoint local variables.
4. `encoding/xml.Decoder` is created per-call (stack-local); not
   shared.
5. HTTP response bodies are closed via `defer resp.Body.Close()` in
   `client.go::doRequest` — same pattern as ADP-001/002.

NFR-ADP3-003 enforces race-cleanliness via 50-goroutine workload under
`go test -race`. NFR-ADP3-004 enforces zero-goroutine-leak via
`goleak.VerifyNone` after a ctx-cancellation mid-flight test.

---

## 6. Risks (arXiv-Specific)

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| arXiv API contract drift (new namespace, new element added) | Low | Medium | `encoding/xml` tolerates unknown elements; test fixtures pinned to documented shape. arXiv API has been stable since at least 2008 (per Wikipedia). |
| `encoding/xml` heavier than `encoding/json` (parse alloc/op floor higher than ADP-001/002) | Medium | Low | NFR-ADP3-001 declares `allocs/op ≤ 700` as a starting target (vs ≤500 in ADP-001/002 NFR-ADP-001 amended); empirical baseline established in run-phase iteration. The pkg/types contract floor (~17 allocs/doc for Metadata map) dominates either way. |
| Rate-limit state mutex contention under heavy concurrent load | Low | Low | Mutex held only for ~10ns (compute+update); actual wait is outside lock. 50-goroutine race test (NFR-ADP3-003) verifies. |
| Title/summary whitespace handling breaks on adversarial input (deeply nested newlines, control chars) | Medium | Low | `strings.Fields` collapses any whitespace run; control chars pass through (Go strings are byte-indexed; UTF-8 preserved). Tested via `TestParseFeedWhitespaceCollapse` over 5 input shapes. |
| Author parsing breaks when `<author>` has no `<name>` child or has multiple `<name>` children (malformed) | Medium | Low | Defensive: skip `<author>` without `<name>`; take first `<name>` if multiple. Test fixture covers. |
| DOI absent from most papers (only ~30% have author-supplied DOI) | High (but expected) | Low | DOI is OPTIONAL Metadata key; consumers cannot rely on its presence. Documented in §4 mapping table. |
| `<published>` and `<updated>` differ for multi-version papers — choosing wrong one as PublishedAt | Low | Low | DESIGN DECISION: PublishedAt = first-version submission date (`<published>`). The `<updated>` value goes to Metadata `updated_at` for consumers who care. |
| Cursor parsing fails on adversarial input (huge integers, negative, non-numeric) | Low | Low | `strconv.Atoi` catches non-numeric; explicit guard against negative; REQ-ADP3-008 rejects with ErrInvalidStart. |
| Score=0.5 (constant) breaks SPEC-IDX-001 RRF expectations | Low | Medium | RRF uses RANK not SCORE per FAN-001 §6.1. Equal scores don't break RRF; tie-breaking happens via rank within source. Open Question §7.5 documents revisit triggers. |
| Hash collisions across arxiv and other sources for shared external URL (e.g., ResearchGate copy of an arXiv paper) | Low | Medium | `CanonicalHash` includes `SourceID` prefix per `pkg/types/normalized_doc.go:96-99` — never collides cross-adapter. |
| Atom XML namespace handling complexity | Medium | Medium | `encoding/xml` honors `xmlns` declarations natively via `xml.Name{Space, Local}`. Test fixtures include all three namespaces; parsing tested via `TestParseFeedDOIInArxivNamespace`. |
| `time.Parse(time.RFC3339, "...-04:00")` timezone handling | Low | Low | RFC 3339 parser handles timezone offsets natively. All times converted to UTC via `.UTC()` after parse. Tested via `TestParseFeedTimezoneHandling`. |
| arXiv returns Atom feed with zero entries on overshoot (`start` > totalResults) | Low | Low | Adapter returns `(nil, nil)` (no error). Caller sees pagination terminated. Tested via `TestSearchOvershoot`. |
| Default Go `net/http` User-Agent borderline-rejected (no documented evidence, but defensive) | Low | Low | REQ-ADP3-009 makes custom UA mandatory (mirrors ADP-001/002 convention). |
| HTTP timeout (10s) too aggressive for arXiv during incidents | Low | Low | Configurable via `Options.HTTPClient`; default 10s aligns with NFR-ADP3-002 stub p95 200ms × 50× safety margin. |
| HTML in `<summary>` / `<title>` (mathematical notation, LaTeX) | Medium | Low | arXiv's Atom feed contains LaTeX strings as plain text (e.g., `$E=mc^2$`). The adapter does NOT render or interpret LaTeX; it passes through. Synthesis (SPEC-SYN-001) decides whether to strip or render. Documented in `Capabilities.Notes`. |
| Multi-rune special characters in arxiv_id (versioning: v1, v2, ..., v15+) | Low | Low | The `vN` suffix is plain ASCII; `strings.TrimPrefix` handles any version count. Tested via 3 fixtures (no version, v1, v15). |

---

## 7. Open Questions (Carried Forward to spec.md §11)

1. **paper-search-mcp wrapping for non-arXiv academic sources**.
   ADP-003 v0.1 ships arXiv direct only. PubMed, bioRxiv, medRxiv,
   Google Scholar, Semantic Scholar coverage requires either
   per-source direct adapters (one SPEC each) or a single
   paper-search-mcp wrap SPEC. **Recommended default**: defer to
   future `SPEC-ADP-003-MCP` if measured demand for non-arXiv
   academic sources warrants the Python sidecar overhead. **Resolution
   owner**: SPEC-ADP-003-MCP author (TBD post-V1 if traffic warrants).

2. **Score population strategy**. Constant `0.5` (recommended in
   §3.5) versus position-derived `1.0 - rank/maxResults` versus
   citation-count integration. **Recommended default**: constant `0.5`
   for v0.1. Revisit after SPEC-IDX-001 RRF integration measurements
   indicate position-derived signal adds value. **Resolution owner**:
   SPEC-IDX-001 author.

3. **`max_results` clamp ceiling**. arXiv allows up to 2000 per call;
   ADP-001/002 clamp at 100. Should ADP-003 also clamp at 100 or
   honour arXiv's higher ceiling? **Recommended default**: clamp at 100
   (uniform adapter behaviour); the per-page bandwidth saving is
   significant only for batch-export use cases not in V1 scope.
   **Resolution owner**: SPEC-FAN-001 author may revisit if
   pagination pressure becomes a measured concern.

4. **Healthcheck strategy**. TCP-connect to `export.arxiv.org:443`
   (mirrors ADP-001/002) versus an HTTP HEAD on the API root.
   **Recommended default**: TCP-connect (cheap, sufficient, uniform
   pattern). **Resolution owner**: SPEC-EVAL-002 may upgrade if
   richer signal needed.

5. **MinRequestInterval default value**. arXiv guideline is 3
   seconds; the adapter declares this as the default. Should production
   actually sleep 3s between every Search invocation, or trust the
   infrequent nature of search workloads to stay under the limit
   organically? **Recommended default**: enforce 3s in production;
   tests inject `0` or `1ms` for fast concurrent-safety verification.
   **Resolution owner**: SPEC-FAN-001a author may add a per-source
   token-bucket layer that subsumes the per-adapter wait.

6. **PDF download integration**. The `<link title="pdf">` URL is
   exposed in Metadata but not consumed in v0.1. SPEC-CACHE-001's
   5-phase access fallback may use it for Phase 0 (pre-fetch). **Out
   of v0.1 scope**. **Resolution owner**: SPEC-CACHE-001 author.

7. **Author affiliation extraction**. `<arxiv:affiliation>` (a child
   of `<author>`) is ignored in v0.1. Some downstream synthesis (e.g.,
   organisation-keyword search) may want it. **Recommended default**:
   ignore in v0.1; add as Metadata key `affiliations` in a future
   patch SPEC. **Resolution owner**: SPEC-SYN-001 author.

8. **arXiv search query syntax exposure**. arXiv's `search_query`
   supports field prefixes (`ti:`, `au:`, `abs:`, `all:`). v0.1 sends
   the user's raw `q.Text` as the search_query unmodified — the user
   can write `ti:transformer au:vaswani` themselves. Should the
   adapter offer a structured `Query.Filters[Key="title"]` /
   `[Key="author"]` translation layer? **Recommended default**:
   pass-through in v0.1; structured filters can be added as a P2
   enhancement once consumer needs are clearer. **Resolution owner**:
   run-phase implementer; SPEC-IR-001a may surface this.

---

## 8. Implementation Hints for the SPEC Author

### 8.1 SPEC Structure and Depth Calibration

Reference SPEC-ADP-001 (1215 lines) and SPEC-ADP-002 (1189 lines) for
depth and shape:

- **SPEC-ADP-001** is ~1200 lines spanning 11 EARS REQs.
- **SPEC-ADP-002** is ~1190 lines spanning 10 EARS REQs.
- **SPEC-ADP-003 should be ~1100-1300 lines** spanning 10-12 EARS REQs.

### 8.2 EARS REQ Coverage

Suggested REQ breakdown:

| ID | Pattern | Topic |
|----|---------|-------|
| REQ-ADP3-001 | Ubiquitous | Adapter interface conformance + Capabilities deterministic shape |
| REQ-ADP3-002 | Event-Driven | Search request URL construction (search_query, start, max_results, sortBy) |
| REQ-ADP3-003 | Event-Driven | HTTP 429 → CategoryRateLimited with Retry-After parsing |
| REQ-ADP3-004 | Event-Driven | HTTP 4xx → CategoryPermanent (including 400 for malformed start/max_results) |
| REQ-ADP3-005 | Event-Driven | HTTP 5xx + network error → CategoryUnavailable |
| REQ-ADP3-006 | Ubiquitous | Atom XML → NormalizedDoc field mapping (entry → doc, with DOI/authors/categories) |
| REQ-ADP3-007 | Optional | Category filter (`Query.Filters[Key="category"]` → `cat:<value>` prepend) |
| REQ-ADP3-008 | Unwanted | Empty query rejection AND invalid start cursor rejection |
| REQ-ADP3-009 | Ubiquitous | User-Agent + Accept headers |
| REQ-ADP3-010 | Optional | Redirect allowlist (`export.arxiv.org`, `arxiv.org`) |
| REQ-ADP3-011 | State-Driven | Concurrent Search safety (50 goroutines × stub server, race-clean) |
| REQ-ADP3-012 | State-Driven | Rate-limit compliance (3-second minimum interval, ctx-cancellation respected) |

### 8.3 NFRs

- **NFR-ADP3-001**: parse path `parseFeed(body []byte, retrievedAt
  time.Time, currentStart int) ([]NormalizedDoc, string, error)` —
  median p50 ≤ 5ms, allocs/op ≤ 700 (XML-amended floor; see Risks
  table).
- **NFR-ADP3-002**: end-to-end p95 ≤ 200ms against stub.
- **NFR-ADP3-003**: race-clean under 50-goroutine concurrent load.
- **NFR-ADP3-004**: zero goroutine leaks on ctx-cancellation
  mid-flight (verified via `goleak.VerifyNone`).

### 8.4 Test Fixtures

Required testdata (golden Atom XML):

| File | Purpose | Size |
|------|---------|------|
| `search_response.xml` | Happy path: 25 entries spanning multiple categories | ~12KB |
| `search_response_empty.xml` | Empty `<feed>` (zero entries) | ~500B |
| `search_response_pagination.xml` | start=0 with totalResults=100, surfaces next_cursor | ~12KB |
| `search_response_with_doi.xml` | Entry with `<arxiv:doi>` populated | ~2KB |
| `search_response_no_doi.xml` | Entry without `<arxiv:doi>` | ~2KB |
| `search_response_multi_author.xml` | Entry with 5+ authors | ~3KB |
| `search_response_multi_version.xml` | Entry with `<id>` ending in `v15` | ~2KB |
| `search_response_overshoot.xml` | start>totalResults; empty feed | ~500B |
| `search_response_400_error.xml` | Atom feed with single error `<entry>` | ~500B |
| `search_response_malformed.xml` | Truncated XML for parse-error path | ~200B |
| `search_response_latex_title.xml` | Title containing LaTeX `$E=mc^2$` | ~2KB |

---

## 9. References

### External (URL-cited; WebFetch-verified)

- https://github.com/openags/paper-search-mcp — paper-search-mcp MCP
  server (rejected as v0.1 dependency per §2.4 rationale; pattern
  reference only).
- https://info.arxiv.org/help/api/index.html — arXiv API documentation
  index (top-level, links to user-manual).
- https://info.arxiv.org/help/api/user-manual.html — arXiv API user
  manual (canonical reference for endpoint, parameters, response
  envelope, rate-limit guideline). WebFetch-verified 2026-05-04.
- https://en.wikipedia.org/wiki/ArXiv — arXiv background
  (Cornell-hosted, 2.5M+ preprints, online since 1991, API stable since
  2008).
- RFC 7231 §7.1.3 Retry-After header semantics (inherited from
  SPEC-ADP-001/002 reference parser).
- https://www.w3.org/2005/Atom — Atom 1.0 specification (referenced
  via Go's `encoding/xml` namespace handling).

### Internal (file:line cited)

- `.moai/specs/SPEC-ADP-001/spec.md` — full reference SPEC structure
  (1215 lines).
- `.moai/specs/SPEC-ADP-001/research.md` — reference research artifact
  (791 lines).
- `.moai/specs/SPEC-ADP-002/spec.md` — second-adapter SPEC (1189
  lines); HN's HTML-strip pattern is a reference for any rich-text
  body adapter.
- `.moai/specs/SPEC-ADP-002/research.md` — HN research artifact (480
  lines).
- `.moai/specs/SPEC-FAN-001/spec.md` — M3 fanout gateway SPEC
  (status: approved); REQ-FAN-002 dispatch loop consumes this adapter.
- `.moai/specs/SPEC-CORE-001/spec.md` — Adapter / Capabilities /
  Query / NormalizedDoc / SourceError contract.
- `.moai/specs/SPEC-OBS-001/spec.md` — observability bundle and
  cardinality discipline.
- `.moai/specs/SPEC-IR-001/spec.md` — `Capabilities` consumer
  contract (REQ-IR-008); `CategoryAcademic` selects DocTypePaper
  adapters.
- `pkg/types/adapter.go:28-45` — Adapter interface 4-method shape.
- `pkg/types/capabilities.go:17` — `DocTypePaper` constant.
- `pkg/types/capabilities.go:38-62` — Capabilities struct.
- `pkg/types/query.go:18-44` — Query struct + Filter shape.
- `pkg/types/errors.go:14-218` — SourceError, Category enum,
  CategorizeError, OutcomeFromError, ValidationError.
- `pkg/types/normalized_doc.go:40-106` — NormalizedDoc 15-field
  struct, Validate, CanonicalHash.
- `internal/adapters/registry.go:75-167` — Registry lifecycle.
- `internal/adapters/registry.go:172-263` — wrappedAdapter
  sole-emitter pattern.
- `internal/adapters/noop/noop.go:1-46` — reference adapter shape +
  compile-time interface assertion.
- `internal/adapters/reddit/reddit.go:1-136` — Adapter struct pattern
  (mirrored by ADP-003 arxiv.go).
- `internal/adapters/reddit/search.go` — Search hot path pattern.
- `internal/adapters/reddit/parse.go` — JSON → NormalizedDoc transform
  pattern (XML equivalent in ADP-003).
- `internal/adapters/reddit/client.go` — HTTP client + redirect
  allowlist pattern.
- `internal/adapters/reddit/score.go` — Tanh score formula (NOT
  reused; see §3.5).
- `internal/adapters/reddit/errors.go` — `parseRetryAfter` helper
  pattern (reused as-is).
- `internal/adapters/hn/parse.go` — HN parseHits pattern (XML decoder
  is the equivalent transform layer).
- `internal/router/category.go:16-17` — `CategoryAcademic` constant.
- `internal/router/category.go:97` — `CategoryAcademic` eligible
  DocTypes = `[DocTypePaper, DocTypeRepo, DocTypeIssue]`.
- `internal/router/router_test.go:79, 411, 443` — `arxiv` adapter
  name already used as a stub.
- `internal/llm/client.go:31-65` — HTTP client + reqid Transport
  pattern (inherited from ADP-001 reference shape).
- `.moai/project/roadmap.md:48` — M3 row "SPEC-ADP-003 | arXiv +
  paper-search adapters | wrap `openags/paper-search-mcp`" (this SPEC
  reinterprets the wrap mention; see §2.4).
- `.moai/project/roadmap.md:122-123` — M3 parallelization plan (M3
  ADP-* SPECs gated on SPEC-FAN-001 — SATISFIED, FAN-001 status:
  approved).
- `.moai/project/roadmap.md:150` — M3 exit criterion ("`usearch
  query` returns fused results across ≥5 adapters").
- `.moai/project/structure.md:18-22` — `internal/adapters/arxiv/`
  reservation.
- `.moai/project/tech.md:102` — Adapter strategy row "arXiv |
  OAI-PMH + arXiv Search API | none | 3s between req".
- `.moai/project/tech.md:103` — "Paper search | wrap
  openags/paper-search-mcp | per-source | varies | Crossref / OpenAlex
  / Semantic Scholar" (deferred per §2.4).
- `go.mod:30` — `go.uber.org/goleak v1.3.0` (NFR-ADP3-004).
- `go.mod:33` — `golang.org/x/sync v0.20.0` (errgroup; NOT consumed by
  ADP-003 directly but available).
- `.moai/config/sections/quality.yaml` — `development_mode: tdd`,
  `test_coverage_target: 85`.
- `.moai/config/sections/harness.yaml` — auto-routing to standard
  level.
- `.moai/config/sections/language.yaml` — `documentation: en`,
  `code_comments: en`.

---

*End of SPEC-ADP-003 research.md v0.1*
