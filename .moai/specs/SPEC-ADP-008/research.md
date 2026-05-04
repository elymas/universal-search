# SPEC-ADP-008 Research — Naver Suite Adapter

**Status**: Research-complete; ready for SPEC drafting
**Author**: limbowl (via manager-spec)
**Created**: 2026-05-04
**Milestone**: M3 — Fanout, adapters, index
**Depends on**: SPEC-CORE-001, SPEC-OBS-001, SPEC-IR-001
**Soft references**: SPEC-ADP-001 (Reddit reference shape), SPEC-ADP-002 (HN second-adapter symmetry)

---

## 0. Research Mandate

SPEC-ADP-008 (Naver Suite adapter) is the FIRST Korean-locale primary adapter
in Universal Search. Its job is to turn a `pkg/types.Query` into a request
against Naver's Open Search API — covering web, news, blog, and shopping
verticals plus DataLab trend lookups — and return `[]types.NormalizedDoc` or
`*types.SourceError`.

This document provides the SPEC author (manager-spec) with:

- The Naver Open API surface (endpoints, auth, parameters, response envelope,
  per-vertical item fields).
- The decision between MCP-wrap (Path A) and direct REST (Path B) with
  rationale.
- A field-mapping table per vertical from Naver JSON → `NormalizedDoc`.
- The reference patterns from SPEC-ADP-001 and SPEC-ADP-002 that ADP-008
  inherits verbatim (file layout, error mapping, sole-emitter discipline, MX
  tag plan, TDD harness).
- Risks unique to a Korean-locale + multi-vertical + auth-required adapter,
  with mitigations.
- Open Questions deferred but documented.

Every external claim is URL-cited and verified via Context7 lookups against
`/websites/developers_naver` (the indexed Naver Open API documentation
corpus). Every internal claim is file:line-cited.

---

## 1. Naver Open API Surface

### 1.1 Endpoints

Authoritative source: Context7 `/websites/developers_naver` corpus, confirmed
against the upstream `naver-search-mcp` client source code at
`https://github.com/isnow890/naver-search-mcp/blob/main/src/clients/naver-api-core.client.ts`
(extracted constants: `searchBaseUrl="https://openapi.naver.com/v1/search"`,
`datalabBaseUrl="https://openapi.naver.com/v1/datalab"`).

Search endpoints (HTTP GET):

| Vertical | Path | Notes |
|----------|------|-------|
| Web (webkr) | `/v1/search/webkr.json` | General Korean web pages |
| News | `/v1/search/news.json` | Naver-indexed news articles |
| Blog | `/v1/search/blog.json` | Naver Blog user-generated content |
| Shopping | `/v1/search/shop.json` | Naver Shopping product listings |

DataLab endpoint (HTTP POST):

| Tool | Path | Notes |
|------|------|-------|
| Search trends | `/v1/datalab/search` | Time-series trend data for keywords |

Both `searchBaseUrl` and `datalabBaseUrl` are pinned to
`https://openapi.naver.com` — single host, single TLS. The `.json` suffix
selects JSON response format (the alternative `.xml` is deprecated for our
use case; the Naver-recommended modern path is `.json`).

### 1.2 Authentication (HARD)

[HARD] Every request MUST include two HTTP headers:

```
X-Naver-Client-Id: $NAVER_CLIENT_ID
X-Naver-Client-Secret: $NAVER_CLIENT_SECRET
```

Source: Context7 `/websites/developers_naver` API reference + verified
against `naver-search-mcp` client source.

Failure modes:
- Missing header → 401 Unauthorized (Naver-side).
- Invalid credentials → 401.
- Disabled application → 403.

The adapter does NOT carry a default key. `Capabilities.RequiresAuth=true`
and `Capabilities.AuthEnvVars=["NAVER_CLIENT_ID", "NAVER_CLIENT_SECRET"]`. The
adapter registry's `RegisterWithOptions` validates env presence at
registration time per `internal/adapters/registry.go:122-129` (the
`SkipAuthCheck` opt-out exists for tests).

### 1.3 Common Search Query Parameters

Parameters apply uniformly across web/news/blog/shop verticals (Source:
Context7 `/websites/developers_naver`):

| Parameter | Type | Default | Range | Notes |
|-----------|------|---------|-------|-------|
| `query` | string | (required) | UTF-8 | URL-encoded search term |
| `display` | int | 10 | 1-100 | Results per page |
| `start` | int | 1 | 1-1000 | 1-based offset for pagination |
| `sort` | enum | `sim` | `sim`,`date` | `sim` = relevance, `date` = recency-first |

Important constraint: `start + display ≤ 1001`. Naver caps total reachable
results at 1000 for any single search even with pagination.

### 1.4 Search Response Envelope

All four search verticals return the same outer envelope (Source: Context7):

```json
{
  "lastBuildDate": "Mon, 04 May 2026 12:34:56 +0900",
  "total": 12345,
  "start": 1,
  "display": 25,
  "items": [ ... ]
}
```

- `total` — total hits available (capped at 1000 effective).
- `start` — echo of request `start`.
- `display` — echo of request `display`.
- `items` — array of per-vertical items (see §1.5).

### 1.5 Per-Vertical Item Field Schemas

Source: Context7 `/websites/developers_naver` API reference + cross-checked
against `naver-search-mcp` schemas.

#### 1.5.1 Web (webkr) item

| Field | Type | Notes |
|-------|------|-------|
| `title` | string | May contain `<b>` highlight tags + HTML entities |
| `link` | string | Canonical URL |
| `description` | string | Snippet; may contain `<b>` highlights + entities |

#### 1.5.2 News item

| Field | Type | Notes |
|-------|------|-------|
| `title` | string | `<b>`-marked highlight tags + entities |
| `originallink` | string | Original publisher URL (preferred for canonical) |
| `link` | string | Naver News URL (mirror) |
| `description` | string | Snippet; `<b>` highlights + entities |
| `pubDate` | string | RFC 1123 date, e.g., `Mon, 04 May 2026 09:00:00 +0900` |

Canonical-URL choice: prefer `originallink` when non-empty; fall back to
`link` (Naver mirror). Rationale: dedup across adapters benefits from the
publisher URL — multiple Naver mirrors of the same article would otherwise
appear as distinct docs.

#### 1.5.3 Blog item

| Field | Type | Notes |
|-------|------|-------|
| `title` | string | `<b>` highlights + entities |
| `link` | string | Blog post URL (Naver Blog or external) |
| `description` | string | Snippet; `<b>` + entities |
| `bloggername` | string | Blogger handle |
| `bloggerlink` | string | Blog homepage URL |
| `postdate` | string | `YYYYMMDD` (e.g., `20260504`) — NOT RFC 1123 |

`postdate` parse: 8-char numeric → `time.Parse("20060102", postdate).UTC()`.

#### 1.5.4 Shopping item

| Field | Type | Notes |
|-------|------|-------|
| `title` | string | Product title; `<b>` + entities |
| `link` | string | Product page URL |
| `image` | string | Thumbnail URL |
| `lprice` | string | Lowest price (numeric string, KRW) |
| `hprice` | string | Highest price (numeric string, KRW; may be `""`) |
| `mallName` | string | Seller/mall name |
| `productId` | string | Naver Shopping product ID |
| `productType` | string | Numeric code: `1`=Naver mall + general goods, `2`=Naver mall + brand, etc. |
| `brand` | string | Brand name (may be `""`) |
| `maker` | string | Manufacturer (may be `""`) |
| `category1`-`category4` | string | 4-level category hierarchy |

### 1.6 DataLab Search Trends (POST)

Endpoint: `POST https://openapi.naver.com/v1/datalab/search`

Request body (JSON; Source: Context7):

```json
{
  "startDate": "2026-04-01",
  "endDate":   "2026-05-04",
  "timeUnit":  "date",
  "keywordGroups": [
    { "groupName": "topic-A", "keywords": ["foo", "bar"] }
  ]
}
```

Optional fields (per Context7 schema): `device` (`pc`|`mobile`),
`ages` ([]string codes), `gender` (`m`|`f`).

Response shape: `{startDate, endDate, timeUnit, results: [{title, keywords,
data: [{period, ratio}]}]}` — time-series trend ratios (NOT absolute counts).

DataLab is NOT a search-result API. It returns trend metadata, not documents.
The adapter MAY surface DataLab output as `[]NormalizedDoc` with a synthetic
shape — one doc per keyword group, `Body` containing the trend table — but
this is a stretch fit. v0.1 treats DataLab as OPT-IN via
`Query.Filters[Key="naver_vertical"][Value="datalab"]`; default behaviour is
search verticals only.

### 1.7 HTML Entity and Highlight Encoding (HARD)

[HARD] Naver wraps matched keywords in `<b>...</b>` tags inside `title` and
`description` fields (Source: Context7 explicit note: "Matching keywords are
wrapped in `<b>` tags"). HTML entities (`&amp;`, `&quot;`, `&lt;`, `&gt;`,
`&#39;`) MAY appear in user-generated content (notably blog titles and
shopping product names).

The adapter MUST strip these tags + decode entities before populating
`NormalizedDoc.Title`, `Body`, `Snippet`. The `stripHTML` helper from
SPEC-ADP-002 §2.1 is the reference shape; ADP-008 duplicates it (same
duplication rationale as ADP-002 §6.2 — "rule of three" not yet reached).

NOT a security boundary: the stripped output is plain text consumed by
synthesis, never rendered as HTML. No XSS sanitisation required.

### 1.8 HTTP Status Codes and Error Semantics

| Code | Semantics | Adapter Response |
|------|-----------|------------------|
| 200 | Success | Parse and return docs |
| 400 | Malformed query (e.g., empty string passed through, invalid vertical) | `*SourceError{CategoryPermanent, HTTPStatus: 400}` |
| 401 | Missing or invalid auth headers | `*SourceError{CategoryPermanent, HTTPStatus: 401}` |
| 403 | Application disabled / quota exhausted at app-level | `*SourceError{CategoryPermanent, HTTPStatus: 403}` |
| 404 | Endpoint typo (should not occur in normal operation) | `*SourceError{CategoryPermanent, HTTPStatus: 404}` |
| 429 | Per-app daily quota exceeded | `*SourceError{CategoryRateLimited, HTTPStatus: 429, RetryAfter}` |
| 500/502/503/504 | Naver upstream incident | `*SourceError{CategoryUnavailable, HTTPStatus: code}` |
| Network error (DNS, dial timeout, TLS) | upstream offline | `*SourceError{CategoryUnavailable, HTTPStatus: 0}` |

Naver does NOT consistently return a `Retry-After` header on 429; default to
5 seconds and cap at 60 seconds — same default policy as SPEC-ADP-001
REQ-ADP-003 and SPEC-ADP-002 REQ-ADP2-003.

### 1.9 Rate Limit Reality (2026)

Source: `.moai/project/tech.md:116` — "wrap `isnow890/naver-search-mcp`
(web/news/blog/shopping + DataLab) | Naver API key | 25000/day | Korean-locale
primary".

The 25,000-calls/day figure is **per registered application** (per
`X-Naver-Client-Id`), counted across ALL search verticals AND DataLab.
DataLab does NOT have an independent quota — it shares the same 25k pool
(Open Question §7.5).

`Capabilities.RateLimitPerMin` is a per-minute approximation: `25000 /
(24*60) ≈ 17/min`. The adapter sets `RateLimitPerMin=17` (conservative;
realistic burst capacity is higher but the adapter does not orchestrate
quota — that is fanout/orchestrator territory per SPEC-FAN-001 D6).

### 1.10 Redirect Behaviour

Naver Open API responds with 200 directly for all valid requests; cross-
domain redirects are not part of the documented behaviour. The adapter's
`CheckRedirect` enforces a 1-host allowlist `{openapi.naver.com}` plus a
3-hop cap as a defensive SSRF guard (mirrors SPEC-ADP-002 §6.5 pattern).

---

## 2. Path Decision: MCP-Wrap vs Direct REST

### 2.1 Path A — MCP-Wrap (REJECTED)

Approach: Spawn the upstream `@isnow890/naver-search-mcp` Node.js binary as
a child process and communicate via JSON-RPC over stdio (or HTTP if exposed).

Pros:
- Reuses upstream tool definitions and parameter validation.
- Future MCP feature additions (e.g., new verticals) propagate "for free".

Cons (decisive):
- **Runtime baggage**: deployment requires Node.js 18+ and npm. Universal
  Search ships as a single Go binary; bundling Node breaks the deployment
  story (`.moai/project/tech.md:79-80` — "single binary").
- **Process management**: subprocess lifecycle (start, restart on crash,
  zombie reaping, stdin/stdout pipe handling) is non-trivial Go code.
- **Indirection cost**: the MCP server is itself a thin wrapper over
  `https://openapi.naver.com/v1/search` REST endpoints (verified in §1.1
  by reading `naver-api-core.client.ts`). Adding JSON-RPC stdio between Go
  and HTTP doubles the serialization cost and adds a second failure surface.
- **Goes against M3 reference shape**: SPEC-ADP-001 and SPEC-ADP-002
  established a direct-REST pattern that the seven M3 adapter SPECs copy
  verbatim. Diverging here would force ADP-009 (Korean news) into a
  different mould or invent a per-adapter MCP shim.
- **Authentication in subprocess**: env vars must be propagated to the
  child Node process. Either inherited (leaks all parent env) or
  selectively forwarded (extra plumbing).

### 2.2 Path B — Direct REST (SELECTED)

Approach: pure Go stdlib + `pkg/types` + `internal/obs/reqid`. Mirrors
SPEC-ADP-001 and SPEC-ADP-002 file layout verbatim.

Pros:
- Zero runtime dependencies beyond Go stdlib (per SPEC-ADP-001 §9.4 + ADP-002
  §9.4 precedent).
- Same shape as ADP-001 / ADP-002 — pattern continuity.
- Single binary deploy preserved.
- Auth: read env vars at construction time, set headers per request — no
  process plumbing.

Cons:
- Per-vertical field-mapping logic lives in our codebase (vs. delegated to
  the MCP server). This is small (4 verticals × ~5 fields each = 20 mapping
  rows) and unlikely to drift (Naver Open API is stable per §1.4 envelope).
- DataLab opt-in adds a POST path that the search verticals do not exercise
  — a small additional code surface (~50 LoC).

Decision: **Path B (Direct REST)**. The `naver-search-mcp` upstream is
referenced for its tool taxonomy and parameter schemas (which we mirror
faithfully), but execution is Go-native. The "wrap" in
`.moai/project/tech.md:116` is interpreted as "behavioural equivalence with
the MCP server" rather than "subprocess exec".

---

## 3. NormalizedDoc Field Mapping (per Vertical)

### 3.1 Vertical Selection

Vertical selection happens in `Search` based on `Query.Filters`. The lookup
key is `naver_vertical` (consistent with ADP-002's `since`/`min_points`
filter convention).

| `Query.Filters[Key="naver_vertical"].Value` | Vertical | Endpoint |
|---------------------------------------------|----------|----------|
| `"blog"` | Blog | `/v1/search/blog.json` |
| `"news"` | News | `/v1/search/news.json` |
| `"web"` | Web (webkr) | `/v1/search/webkr.json` |
| `"shop"` | Shopping | `/v1/search/shop.json` |
| `"datalab"` | DataLab trends | `POST /v1/datalab/search` |
| (absent or `""`) | Default = `blog` | `/v1/search/blog.json` |
| any other value | Reject with `ErrInvalidVertical` (CategoryPermanent) |

Default = `blog` rationale: blog content is the richest user-generated
Korean text source; news has tighter overlap with `daum`/`rss_korean`
(SPEC-ADP-009); web (webkr) overlaps with SearXNG (SPEC-ADP-007). When the
IR-001 router dispatches a `CategoryKorean` query without explicit filter,
Naver blog gives the best signal.

### 3.2 Common Fields (All Search Verticals)

| Naver field | NormalizedDoc field | Transform |
|-------------|---------------------|-----------|
| (constructed: `"naver:" + vertical + ":" + sha256(link)[:8]`) | `ID` | Naver does NOT supply a stable per-item ID across verticals; we synthesize one from the canonical URL hash. Used as adapter-internal ID; NormalizedDoc uniqueness is downstream's concern. |
| (constant) | `SourceID` | `"naver"` |
| `link` (or `originallink` for news) | `URL` | `originallink` preferred for news; `link` for others |
| `stripHTML(title)` | `Title` | Strip `<b>` tags + decode HTML entities |
| `stripHTML(description)` | `Body` | Same as title |
| `stripHTML(description)` truncated to 280 runes | `Snippet` | Falls back to truncated `Title` when description empty |
| (per §3.3-3.6) | `PublishedAt` | Vertical-specific date parse; zero when not provided (web has no pubdate) |
| `time.Now().UTC()` (parse-time) | `RetrievedAt` | Set by `parseResponse` caller for determinism |
| (per §3.3-3.6) | `Author` | `bloggername` (blog), `mallName` (shop), publisher derived from `originallink` host (news), `""` (web) |
| `0.5` (default) | `Score` | Naver does not surface engagement scores in the search response; assign neutral middle. SPEC-IDX-001 RRF re-ranks by rank not score. |
| `"ko"` | `Lang` | Naver Open API serves Korean content; static label |
| (per vertical) | `DocType` | blog→`DocTypePost`, news→`DocTypeArticle`, web→`DocTypeArticle`, shop→`DocTypeOther` (no shopping enum), datalab→`DocTypeOther` |
| `nil` | `Citations` | Adapter returns posts/articles, not citation graphs |
| (per §3.3-3.6) | `Metadata` | Vertical-specific extras (REQUIRED keys per vertical) |
| `""` | `Hash` | Consumers compute via `CanonicalHash()` |

### 3.3 Blog vertical specifics

Required Metadata keys: `naver_vertical=blog`, `bloggername`, `bloggerlink`,
`postdate` (raw `YYYYMMDD` string for downstream-readability).

`PublishedAt`: parse `postdate` via `time.Parse("20060102", postdate).UTC()`;
zero on parse failure.

`Author`: copy from `bloggername`.

DocType: `DocTypePost`.

### 3.4 News vertical specifics

Required Metadata keys: `naver_vertical=news`, `originallink`, `naver_link`
(= the `link` field; renamed to disambiguate from canonical URL).

`PublishedAt`: parse `pubDate` via `time.Parse(time.RFC1123Z, pubDate).UTC()`
with fallback to `time.RFC1123`; zero on parse failure.

`URL`: prefer `originallink` when non-empty; fall back to `link`.

`Author`: derive from `originallink` host (e.g.,
`originallink="https://www.example.com/article"` → `Author="example.com"`).
Naver does not supply a per-article author field. Empty `originallink`
results in `Author=""`.

DocType: `DocTypeArticle`.

### 3.5 Web (webkr) vertical specifics

Required Metadata keys: `naver_vertical=web`.

`PublishedAt`: zero (Naver web search does NOT supply a pubdate).

`Author`: empty.

DocType: `DocTypeArticle`.

### 3.6 Shopping vertical specifics

Required Metadata keys: `naver_vertical=shop`, `mallName`, `productId`,
`productType`, `lprice`, `hprice` (raw strings; KRW), `image`,
`category1`-`category4`.

`PublishedAt`: zero (no item-level date).

`Author`: copy from `mallName`.

DocType: `DocTypeOther` (Naver Shopping doesn't fit the existing enum;
SPEC-IDX-001 may add `DocTypeProduct` in a future amendment — Open Question
§7.6).

### 3.7 DataLab (POST) specifics

Required Metadata keys: `naver_vertical=datalab`,
`datalab_keyword_groups`, `datalab_time_unit`, `datalab_start_date`,
`datalab_end_date`, `datalab_data` (the time-series array marshalled as
JSON string for downstream-readability).

`URL`: synthesized — `https://datalab.naver.com/keyword/trendResult.naver?
hashKey=<sha256(req-body)[:16]>` (deterministic but synthetic; consumers use
it as a unique key, NOT a clickable URL).

`Title`: `"DataLab trend: " + groupName` per result row.

`Body`: human-readable summary of the time-series (top 3 dates with highest
ratios).

`PublishedAt`: parse `endDate` from request body.

`Author`: `"Naver DataLab"`.

DocType: `DocTypeOther`.

DataLab opt-in only. The adapter requires that `Query.Text` is the JSON-
encoded request body when vertical=datalab. Open Question §7.4 documents the
ergonomics revisit.

---

## 4. Existing Codebase Patterns to Follow

### 4.1 Adapter Interface Conformance

`pkg/types/adapter.go:28-45` declares the 4-method interface. ADP-008
implements:
- `Name() string` → `"naver"` (lowercase, stable)
- `Search(ctx context.Context, q types.Query) ([]types.NormalizedDoc, error)`
- `Healthcheck(ctx context.Context) error` (TCP-connect to
  `openapi.naver.com:443`)
- `Capabilities() types.Capabilities` (deterministic)

Compile-time assertion at the bottom of `naver.go`:
`var _ types.Adapter = (*Adapter)(nil)`.

### 4.2 Error Taxonomy and SourceError Construction

`pkg/types/errors.go:14-218` publishes the four Categories. ADP-008's
`categorizeStatus` mirrors ADP-001's `internal/adapters/reddit/client.go:102-124`
shape verbatim, with `Adapter: "naver"`. Mappings:
- 401/403 → `CategoryPermanent` (with note "auth failed; check env vars")
- 429 → `CategoryRateLimited` with parsed `Retry-After` (default 5s, cap 60s)
- 4xx other → `CategoryPermanent`
- 5xx → `CategoryUnavailable`
- network error → `CategoryUnavailable, HTTPStatus=0`

### 4.3 Registry Pattern and Sole-Emitter Discipline

`internal/adapters/registry.go:172-263` wraps every Adapter with
`wrappedAdapter`, which emits all observability:
- 1× OTel span `adapter.search`
- 1× Prometheus counter `AdapterCalls{adapter,outcome}`
- 1× Prometheus histogram `AdapterCallDuration{adapter}`
- 1× slog INFO/WARN

The Naver adapter emits ZERO metrics/logs/spans of its own. The
`adapter="naver"` label is bounded by the V1 14-adapter ceiling per
SPEC-OBS-001. Per-vertical sub-labelling is OUT OF SCOPE — the chosen
vertical lives in `NormalizedDoc.Metadata["naver_vertical"]` for downstream
visibility but does not escape to Prometheus (`outcome` cardinality stays
bounded to the 5 SPEC-CORE-001 values).

### 4.4 HTTP Client Construction Pattern

`internal/llm/client.go:31-65` and
`internal/adapters/reddit/client.go:50-56` show the canonical shape:

```go
&http.Client{
    Timeout:       10 * time.Second,
    Transport:     reqid.NewTransport(http.DefaultTransport),
    CheckRedirect: redirectAllowlist,  // 1-host allowlist for Naver
}
```

ADP-008 reuses this verbatim. The `reqid.NewTransport` wrap propagates
request IDs into outbound headers for observability correlation.

### 4.5 Reference Adapter (Reddit / HN) File Layout

`internal/adapters/reddit/` ships 13 files (10 source + 3 testdata-related).
`internal/adapters/hn/` ships 14 (similar shape + `strip.go` + per-vertical
fixtures). ADP-008 inherits this shape with extensions for multi-vertical:

```
internal/adapters/naver/
├── naver.go                                # Adapter struct + interface assertion
├── naver_test.go
├── search.go                               # Hot path: vertical dispatch + URL construction
├── search_test.go
├── client.go                               # HTTP client, doRequest, categorizeStatus, allowlist
├── client_test.go
├── parse.go                                # parseResponse (envelope) + 4 per-vertical parsers
├── parse_test.go
├── strip.go                                # stripHTML (mirrors hn/strip.go)
├── strip_test.go
├── score.go                                # normalizeScore (constant 0.5 default + score_test.go)
├── score_test.go
├── errors.go                               # ErrInvalidQuery, ErrInvalidVertical, parseRetryAfter
├── datalab.go                              # DataLab POST builder + parser (opt-in path)
├── datalab_test.go
├── bench_test.go                           # BenchmarkParseBlogResponse25Items + goleak.VerifyTestMain
└── testdata/
    ├── search_response_blog.json           # 25-item happy path
    ├── search_response_news.json
    ├── search_response_web.json
    ├── search_response_shop.json
    ├── search_response_blog_empty.json
    ├── search_response_blog_pagination.json
    ├── search_response_blog_html_entities.json
    ├── search_response_news_no_originallink.json
    ├── search_response_blog_malformed_postdate.json
    ├── search_response_blog_malformed.json
    └── datalab_response.json
```

---

## 5. Reference Implementations (External)

### 5.1 isnow890/naver-search-mcp (TypeScript)

Source: https://github.com/isnow890/naver-search-mcp

License: MIT.

Purpose validated: tool taxonomy (search_blog / search_news / search_webkr /
search_shop / datalab_search) maps 1:1 to ADP-008's vertical filter values.
Confirmed endpoint URLs by reading
`https://github.com/isnow890/naver-search-mcp/blob/main/src/clients/naver-api-core.client.ts`:
`searchBaseUrl="https://openapi.naver.com/v1/search"`,
`datalabBaseUrl="https://openapi.naver.com/v1/datalab"`. Headers:
`X-Naver-Client-Id`, `X-Naver-Client-Secret`, `Content-Type: application/json`.
Timeout: 30s. Max redirects: 3.

NOT used as a Go dependency or subprocess. Used purely as a structural
reference.

### 5.2 SearXNG `naver.py` engine

Source: https://github.com/searxng/searxng/blob/master/searx/engines/naver.py

The SearXNG engine scrapes `https://search.naver.com/search.naver` (HTML)
rather than calling the Open API — that approach avoids API key registration
but is fragile to UI changes. ADP-008 uses the OpenAPI path (auth-based,
stable JSON envelope) instead. SearXNG's parameter list (`where`, `start`,
`nso`) is informational only; the JSON API uses different parameter names
(`display`, `start`, `sort`).

---

## 6. Risk Register

| Risk | Severity | Mitigation |
|------|----------|-----------|
| Auth env vars missing at registration → adapter unusable | High | `Capabilities.RequiresAuth=true` + `AuthEnvVars=["NAVER_CLIENT_ID","NAVER_CLIENT_SECRET"]`; the registry's `RegisterWithOptions` rejects with `ErrMissingAuth` per `internal/adapters/registry.go:122-129`. Tests use `SkipAuthCheck` per `internal/adapters/registry.go:49`. |
| HTML entity / `<b>` tag leakage into `Body` / `Title` confuses synthesis | High | `stripHTML` helper applied to every text field per REQ-ADP8-009. Test fixture `search_response_blog_html_entities.json` covers `<b>`, `&amp;`, `&quot;`, `&#39;`, `&nbsp;`. Output is plain text; not a security boundary. |
| 25k/day quota exhaustion | High | `Capabilities.RateLimitPerMin=17` (conservative). Adapter does NOT manage quota — it returns 429 on quota exhaustion. SPEC-FAN-001 (M3) D6 says fanout doesn't retry; SPEC-EVAL-002 (M8) tracks per-adapter health. |
| DataLab shares the same 25k quota → opt-in misuse depletes search budget | Medium | Default vertical is `blog`. DataLab requires explicit opt-in via `Query.Filters[Key="naver_vertical"][Value="datalab"]`. `Capabilities.Notes` documents the shared quota. |
| News `originallink` empty surfaces Naver-mirror URL instead of publisher URL | Medium | Fallback to `link` documented in §3.4. NormalizedDoc.URL is canonical-best-effort; SPEC-SYN-003 (M4) handles dedup deeper. |
| Shopping `productType` enum drift (Naver adds new product type codes) | Low | Stored as-is in `Metadata["productType"]`. Not used for routing/dedup. |
| `pubDate` parsing fails on locale-specific date formats | Medium | Try `time.RFC1123Z` first, then `time.RFC1123`, fall back to zero. `PublishedAt=zero` is valid per `pkg/types/normalized_doc.go:25` ("zero when source provides no date"). |
| `postdate` (`YYYYMMDD`) parses incorrectly on malformed strings | Low | Fall back to zero. Test fixture covers `postdate=""`, `postdate="invalid"`. |
| Cross-domain redirect (e.g., Naver CDN routing) → SSRF surface | Low | Allowlist `{openapi.naver.com}` rejects everything else. Test asserts via `TestSearchRejectsCrossDomainRedirect`. |
| `<b>` tag stripping false-positive eats user content (e.g., legitimate text containing `<b`) | Low | Test fixtures include `"<b>highlight</b> normal text"` and `"<button>"` strings. Conservative stdlib-only strip handles balanced tags; mismatched tags pass through (acceptable in plain text). |
| Korean text encoding edge cases (Unicode normalization, NFC vs NFD) | Low | Go strings are UTF-8 byte slices; `unicode/utf8` rune iteration handles all forms. No NFC/NFD coercion needed in v0.1. |
| Concurrent shared `*http.Client` across goroutines | Low | `*http.Client` is goroutine-safe per Go stdlib. REQ-ADP8-012 asserts via 50-goroutine race-detector test. |
| Naver Open API contract drift (field added/removed) | Low | `encoding/json` ignores unknown fields. Test fixtures pinned. SearXNG engine has been stable against the same shape for 5+ years. |
| Vertical filter typo passed by caller (e.g., `"News"` capitalized) | Low | Reject case-sensitively with `ErrInvalidVertical`. `Capabilities.Notes` enumerates valid values. |
| `time.Now()` non-determinism in tests | Low | `parseResponse` accepts `retrievedAt` argument; tests inject fixed time. |
| HTTP timeout (10s) too aggressive during incidents | Low | Configurable via `Options.HTTPClient`. Default aligns with NFR-ADP8-002 stub p95 baseline. |
| Stub `httptest.Server` race detector noise on 50-goroutine workload | Low | Server itself is goroutine-safe per Go stdlib. ADP-001 / ADP-002 already validated this pattern. |

---

## 7. Open Questions

These are intentionally deferred; they do not block SPEC approval. Each
has a recommended default and a one-line resolution owner.

### 7.1 Vertical default selection

Should the default vertical (when `Query.Filters[Key="naver_vertical"]` is
absent) be `blog`, `news`, or `web`? **Recommended default**: `blog`. Blog
has the richest Korean user-generated text content and lowest cross-source
overlap (web overlaps with SearXNG, news overlaps with daum). Operators may
override via filters. **Resolution owner**: SPEC-IR-001 author when intent
sub-routing matures (post-M3).

### 7.2 News canonical URL choice

Use `originallink` (publisher URL) or `link` (Naver mirror) for
`NormalizedDoc.URL`? **Recommended default**: `originallink` first, fall back
to `link`. Rationale: dedup across adapters benefits from publisher URL.
**Resolution owner**: SPEC-SYN-003 (M4 dedup) author may revise if mirror-
URL retention helps citation accuracy.

### 7.3 Shopping `DocType`

`DocTypeOther` is awkward for shopping items. Should `pkg/types/capabilities.go`
gain a `DocTypeProduct` constant? **Recommended default**: NO in v0.1. Adding
a `DocType` constant breaks the `pkg/types` SDK boundary semver per
`structure.md:160-161`. Defer to a coordinated SPEC-CORE-001a if/when SPEC-IDX-001
needs the distinction. **Resolution owner**: SPEC-IDX-001 author.

### 7.4 DataLab ergonomics

DataLab's POST body is heterogeneous (keywordGroups, timeUnit, demographic
filters); shoving it through `Query.Text` as a JSON string is awkward.
**Recommended default**: keep the JSON-string-in-Query.Text shape for v0.1
(opt-in only; not on the hot path). A future SPEC-ADP-008-DL may add a
typed `DataLabQuery` shape under `pkg/types`. **Resolution owner**:
SPEC-IDX-002 (embedding service) author may need DataLab as a feature input
and would drive the typed-shape SPEC.

### 7.5 DataLab quota separation

Is DataLab's 25k/day quota independent of search vertical quota?
**Recommended default**: assume SHARED 25k pool until empirically verified
(safer for budget planning). **Resolution owner**: run-phase implementer
during integration smoke test.

### 7.6 Korean tokenization at adapter layer

Should the adapter pre-tokenize Korean text via mecab-ko before populating
`Body`? **Recommended default**: NO. Tokenization is SPEC-IDX-003's job (M3,
mecab-ko sidecar). The adapter returns plain stripped text; the index layer
tokenizes downstream. **Resolution owner**: SPEC-IDX-003 author.

### 7.7 `productType` enum stability

Naver's shopping `productType` codes (`1`-`8` ish) lack a published canonical
list. **Recommended default**: store as-is in `Metadata["productType"]`,
treat as opaque numeric string. **Resolution owner**: not assigned —
operational concern, not a blocker.

### 7.8 Sort selection (`sim` vs `date`)

Naver's `sort=date` returns reverse-chronological results; `sort=sim` returns
relevance-ranked. **Recommended default**: hardcode `sim` in v0.1; optional
opt-in via `Query.Filters[Key="sort"][Value="date"]`. **Resolution owner**:
SPEC-IR-001 author may add an intent-derived sort directive.

---

## 8. Sources and Citations

### External URLs (verified via Context7 + GitHub source inspection)

- `/websites/developers_naver` (Context7 indexed corpus) — Naver Open API
  documentation. Source for §1.1 endpoints, §1.3 query parameters, §1.4
  response envelope, §1.5 per-vertical item schemas, §1.7 HTML entity rule,
  §1.6 DataLab POST body shape.
- https://github.com/isnow890/naver-search-mcp — naver-search-mcp upstream
  repo (TypeScript, MIT). Tool taxonomy reference for §3.1 vertical mapping;
  rejected as runtime dep per §2.
- https://github.com/isnow890/naver-search-mcp/blob/main/src/clients/naver-api-core.client.ts
  — verified `searchBaseUrl` / `datalabBaseUrl` constants and 30s timeout +
  3 max-redirects defaults. Confirms §1.1 endpoint pinning.
- https://github.com/searxng/searxng/blob/master/searx/engines/naver.py —
  SearXNG Naver engine (HTML scraping path). Used as informational comparison
  in §5.2; not adopted.
- RFC 7231 §7.1.3 Retry-After header — basis for `parseRetryAfter` (inherited
  from SPEC-ADP-001 §6.3).

### Internal Files (file:line cited)

- `/Users/masterp/Projects/superwork/univesal-search/.moai/specs/SPEC-CORE-001/spec.md`
  — adapter contract.
- `/Users/masterp/Projects/superwork/univesal-search/.moai/specs/SPEC-OBS-001/spec.md`
  — observability + cardinality allowlist.
- `/Users/masterp/Projects/superwork/univesal-search/.moai/specs/SPEC-IR-001/spec.md`
  — REQ-IR-008 AdapterSet selection (Korean Lang admits naver).
- `/Users/masterp/Projects/superwork/univesal-search/.moai/specs/SPEC-IR-001/acceptance.md:177`
  — naver capabilities expected: `DocTypes:[article,post]`,
  `SupportedLangs:[ko]`.
- `/Users/masterp/Projects/superwork/univesal-search/.moai/specs/SPEC-ADP-001/spec.md`
  — reference adapter SPEC; SPEC-ADP-008 inherits structure verbatim.
- `/Users/masterp/Projects/superwork/univesal-search/.moai/specs/SPEC-ADP-002/spec.md`
  — second-adapter SPEC; ADP-008 inherits `stripHTML`, vertical-filter
  pattern, multi-fixture testdata layout.
- `/Users/masterp/Projects/superwork/univesal-search/.moai/specs/SPEC-FAN-001/spec.md`
  — M3 fanout SPEC; consumes ADP-008 via `registry.Get("naver").Search`.
- `/Users/masterp/Projects/superwork/univesal-search/pkg/types/adapter.go:28-45`
  — Adapter interface.
- `/Users/masterp/Projects/superwork/univesal-search/pkg/types/capabilities.go:38-62`
  — Capabilities + DocType enum.
- `/Users/masterp/Projects/superwork/univesal-search/pkg/types/query.go:18-44`
  — Query + Filter shape.
- `/Users/masterp/Projects/superwork/univesal-search/pkg/types/errors.go:14-218`
  — SourceError, Category enum, OutcomeFromError.
- `/Users/masterp/Projects/superwork/univesal-search/pkg/types/normalized_doc.go:40-106`
  — NormalizedDoc 15-field struct, Validate, CanonicalHash.
- `/Users/masterp/Projects/superwork/univesal-search/internal/adapters/registry.go:75-167`
  — Registry lifecycle + auth env validation.
- `/Users/masterp/Projects/superwork/univesal-search/internal/adapters/registry.go:172-263`
  — wrappedAdapter sole-emitter pattern.
- `/Users/masterp/Projects/superwork/univesal-search/internal/adapters/reddit/reddit.go:1-136`
  — Adapter struct shape mirrored by ADP-008 `naver.go`.
- `/Users/masterp/Projects/superwork/univesal-search/internal/adapters/reddit/search.go:1-167`
  — Search hot-path pattern.
- `/Users/masterp/Projects/superwork/univesal-search/internal/adapters/reddit/parse.go:1-203`
  — parse pattern (ADP-008's parseResponse follows the same shape, dispatched
  to per-vertical sub-parsers).
- `/Users/masterp/Projects/superwork/univesal-search/internal/adapters/reddit/client.go:1-125`
  — HTTP client + redirect allowlist + categorizeStatus pattern.
- `/Users/masterp/Projects/superwork/univesal-search/internal/adapters/reddit/errors.go:1-64`
  — parseRetryAfter helper (duplicated verbatim per ADP-002 §6.2 rule of
  three).
- `/Users/masterp/Projects/superwork/univesal-search/internal/llm/client.go:31-65`
  — HTTP client construction with timeout + reqid Transport.
- `/Users/masterp/Projects/superwork/univesal-search/.moai/project/roadmap.md:53`
  — M3 row for SPEC-ADP-008.
- `/Users/masterp/Projects/superwork/univesal-search/.moai/project/roadmap.md:122-123`
  — M3 7-way parallelization plan; ADP-008 gated on FAN-001.
- `/Users/masterp/Projects/superwork/univesal-search/.moai/project/structure.md:25`
  — `internal/adapters/naver/` reservation.
- `/Users/masterp/Projects/superwork/univesal-search/.moai/project/tech.md:116`
  — adapter-strategy row (Naver: 25,000/day; Korean-locale primary).
- `/Users/masterp/Projects/superwork/univesal-search/.moai/config/sections/quality.yaml:17,76`
  — `development_mode: tdd`, `test_coverage_target: 85`.
- `/Users/masterp/Projects/superwork/univesal-search/.moai/config/sections/harness.yaml:8-17`
  — auto-routing rules (standard level for ADP-008).
- `/Users/masterp/Projects/superwork/univesal-search/.moai/config/sections/language.yaml`
  — `documentation: en`, `code_comments: en`.

---

End of Research Document.

**Summary for SPEC author**: The Naver Open API surface is well-documented
through Context7 and the upstream MCP reference. Direct REST is the right
path (Path B); MCP-wrap is rejected for runtime-baggage and reference-shape
reasons. The adapter spans 4 search verticals (blog default, news, web,
shop) plus DataLab opt-in, all behind a single `Search` entry point that
dispatches by `Query.Filters[Key="naver_vertical"]`. Authentication is
header-based (`X-Naver-Client-Id` + `X-Naver-Client-Secret`) and validated
at registration time. The HTML entity/`<b>` tag strip is the only non-trivial
text transform. Reference shape inheritance from ADP-001 + ADP-002 is
verbatim. 14 EARS REQs anticipated; 4 NFRs; 8 Open Questions deferred.
