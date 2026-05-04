---
id: SPEC-ADP-008
title: Naver Suite Adapter
version: 0.1.0
milestone: M3 — Fanout, adapters, index
status: draft
priority: P0
owner: expert-backend
methodology: tdd
coverage_target: 85
created: 2026-05-04
updated: 2026-05-04
author: limbowl
issue_number: null
depends_on: [SPEC-CORE-001, SPEC-OBS-001, SPEC-IR-001]
blocks: []
---

# SPEC-ADP-008: Naver Suite Adapter

## HISTORY

- 2026-05-04 (initial draft v0.1, limbowl via manager-spec):
  First EARS-formatted SPEC for the Naver Suite (web/news/blog/shopping +
  DataLab) M3 adapter. Built on SPEC-CORE-001 (`pkg/types.Adapter` 4-method
  contract at `pkg/types/adapter.go:28-45`, `pkg/types.NormalizedDoc`
  15-field canonical struct at `pkg/types/normalized_doc.go:40-56`,
  `*types.SourceError` taxonomy with four Categories at
  `pkg/types/errors.go:14-218`, `pkg/types.Capabilities` descriptor at
  `pkg/types/capabilities.go:38-62`, and `internal/adapters.Registry` with
  sole-emitter `wrappedAdapter` at `internal/adapters/registry.go:172-263`),
  SPEC-OBS-001 (`AdapterCalls{adapter,outcome}` + `AdapterCallDuration{adapter}`
  collectors with `adapter` and `outcome` already in the cardinality
  allowlist), SPEC-IR-001 (REQ-IR-008 selects `naver` for Korean queries via
  `SupportedLangs:["ko"]` + `DocTypes:[article,post]` per
  `.moai/specs/SPEC-IR-001/acceptance.md:177`), and the SPEC-ADP-001 +
  SPEC-ADP-002 reference shape (file layout, error mapping, MX tag plan,
  TDD harness, sole-emitter discipline).

  User-locked decisions baked in:

  - **D1 Path selection**: Direct REST in pure Go stdlib (Path B). MCP-wrap
    of `isnow890/naver-search-mcp` rejected because (a) MCP server is
    Node.js, breaking single-binary deployment per `tech.md:79-80`; (b) the
    MCP server is itself a thin wrapper over the same
    `https://openapi.naver.com/v1/search` endpoints (verified by reading
    `naver-api-core.client.ts` constants); (c) the M3 reference pattern from
    SPEC-ADP-001 + SPEC-ADP-002 is direct-REST-pure-stdlib and ADP-008
    preserves it. The "wrap `isnow890/naver-search-mcp`" wording in
    `.moai/project/tech.md:116` is interpreted as behavioural equivalence,
    not subprocess exec. Research §2.

  - **D2 Vertical selection**: Single `*Adapter` instance routes internally
    to one of five verticals (blog/news/web/shop/datalab) per
    `Query.Filters[Key="naver_vertical"]`. Default (filter absent) → `blog`
    (richest Korean user-generated content; lowest cross-source overlap with
    SearXNG/daum/RSS_korean). Unknown vertical value → `ErrInvalidVertical`
    wrapped in `*SourceError{CategoryPermanent}`. Research §3.1.

  - **D3 Authentication**: `RequiresAuth=true`,
    `AuthEnvVars=["NAVER_CLIENT_ID","NAVER_CLIENT_SECRET"]`. Headers
    `X-Naver-Client-Id` and `X-Naver-Client-Secret` set on every request.
    Registry validates env presence at `RegisterWithOptions` time per
    `internal/adapters/registry.go:122-129`. Tests use the
    `RegisterOptions{SkipAuthCheck: true}` opt-out from
    `internal/adapters/registry.go:49`. Research §1.2.

  - **D4 Per-vertical field mapping**: 4 distinct Naver JSON shapes →
    `NormalizedDoc` field mapping table per vertical (blog: bloggername +
    postdate; news: originallink + pubDate; web: bare title/link/description;
    shop: lprice/hprice/mallName/productId/category1-4 + image). Common
    fields (`SourceID="naver"`, `Lang="ko"`, `Score=0.5`,
    `RetrievedAt=now()`) cut across verticals. The vertical chosen for a
    given request lives in `NormalizedDoc.Metadata["naver_vertical"]` for
    downstream visibility but does NOT escape to Prometheus (sole-emitter
    discipline preserved; cardinality bounded). Research §3.

  - **D5 HTML entity + `<b>` highlight strip**: Naver wraps matched keywords
    in `<b>...</b>` tags inside `title` and `description` (Context7 explicit
    note), and HTML entities (`&amp;`, `&quot;`, `&lt;`, `&gt;`, `&#39;`)
    appear in user-generated content. The adapter strips both before
    populating `Title`, `Body`, `Snippet`. Helper duplicated from
    SPEC-ADP-002 `internal/adapters/hn/strip.go` (rule of three not yet
    reached; refactor SPEC after M3). Not a security boundary — output is
    plain text consumed by synthesis, never rendered as HTML. Research §1.7.

  - **D6 Score normalization**: Naver search response carries no engagement
    metric (no upvotes, no comment count, no view count for posts). All
    docs receive `Score=0.5` (neutral middle of `[0.0, 1.0]` codomain).
    SPEC-IDX-001 (M3) RRF re-ranks by rank not raw score, so the constant
    is harmless for fusion. Open Question §11.5 documents revisit triggers
    if RRF measurements indicate per-vertical calibration is needed.
    Research §3.2.

  - **D7 Pagination**: Naver uses 1-based `start` + `display` parameters
    (`start + display ≤ 1001` per Naver doc). Cursor encoding: integer-string
    of the next-page `start` value, surfaced via
    `Metadata["next_cursor"]` on the LAST returned doc. Round-trip:
    `Query.Cursor="26"` → `start=26` in next request. Cursor parse: invalid
    integer → `ErrInvalidCursor` wrapped in `*SourceError{CategoryPermanent}`.
    Research §1.3.

  - **D8 Sort**: Hardcoded `sort=sim` (relevance) in v0.1; opt-in to
    `sort=date` via `Query.Filters[Key="sort"][Value="date"]`. Research §1.3
    + Open Question §11.7. Other sort values are silently dropped (default to
    `sim`); no error returned (matches ADP-002 unknown-filter discipline at
    SPEC-ADP-002 REQ-ADP2-007).

  - **D9 DataLab opt-in**: DataLab POST is OFF the search hot path. Triggered
    only by `Query.Filters[Key="naver_vertical"][Value="datalab"]`. Request
    body is parsed from `Query.Text` (JSON-encoded payload). Returns one
    NormalizedDoc per `keywordGroups` row, with the time-series array
    serialised into `Metadata["datalab_data"]`. Quota note: shared 25k/day
    pool with search verticals (Open Question §11.4). Research §1.6 + §3.7.

  - **D10 Tests**: `net/http/httptest.Server` stub + golden JSON fixtures
    under `internal/adapters/naver/testdata/`. NO live network calls in CI.
    Optional env-gated integration test
    (`-tags=integration` + `NAVER_LIVE=1`) deferred to a follow-up SPEC if
    measured value warrants. Research §4.5.

  Resolved discrepancies:
  - `tech.md:116` says "wrap `isnow890/naver-search-mcp`" — D1 reinterprets
    this as behavioural-equivalence-via-direct-REST. The same endpoints are
    called; the same params; the same auth headers. The MCP runtime is
    omitted.
  - SPEC-IR-001 acceptance specifies `DocTypes:[article,post]` for naver
    (`.moai/specs/SPEC-IR-001/acceptance.md:177`). ADP-008 honours this:
    blog→`DocTypePost`, news→`DocTypeArticle`, web→`DocTypeArticle`,
    shop→`DocTypeOther` (no `DocTypeProduct` enum value yet — Open
    Question §11.6).

  14 EARS REQs (12 × P0 + 2 × P1) covering all five EARS patterns
  (Ubiquitous, Event-Driven, State-Driven via REQ-ADP8-012 concurrency-
  safety contract, Optional, Unwanted), 4 NFRs (parse-path performance,
  E2E p95 stub latency, race-clean concurrent invocation, zero goroutine
  leaks), 8 Open Questions carried forward from research.md §7. Zero new
  Go module dependencies — pure stdlib (`net/http`, `encoding/json`,
  `time`, `context`, `errors`, `strings`, `strconv`, `net/url`,
  `unicode/utf8`, `unicode`, `crypto/sha256`, `encoding/hex`, `os`,
  `fmt`, `io`) plus existing `pkg/types` and `internal/obs/reqid`
  (nil-safe consumer; the registry wraps observability, not the
  adapter). Inserted into M3 as one of seven parallel ADP-* SPECs gated
  on SPEC-FAN-001 per `.moai/project/roadmap.md:122-123`. Harness level:
  standard (single domain, ≤14 source files including DataLab path,
  no security/payment/PII keywords, no compose/env/config deltas).
  Sprint Contract optional. Ready for plan-auditor review and annotation
  cycle.

---

## 1. Purpose

SPEC-CORE-001 published the typed adapter contract; SPEC-IR-001 published
the routing layer that selects `naver` for Korean queries (REQ-IR-008
intersects `SupportedLangs:["ko"]` with category-eligible DocTypes; per
`.moai/specs/SPEC-IR-001/acceptance.md:177` naver declares
`DocTypes:[article,post]`); SPEC-ADP-001 + SPEC-ADP-002 implemented two
reference adapters and locked the file-layout pattern. SPEC-FAN-001 (the
M3 gateway) consumes adapter outputs and is the wedge for 7-way M3
parallelization (`.moai/project/roadmap.md:122-123` — "All SPEC-ADP-*
(7-way), SPEC-IDX-* (3-way) — gated on SPEC-FAN-001").

SPEC-ADP-008 fills `internal/adapters/naver/` with the **Korean-locale
primary adapter**, the FIRST multi-vertical adapter in Universal Search.
The Naver adapter is the centerpiece for Korean-locale coverage:

1. **Korean-locale primary**: per `tech.md:116` "Korean-locale primary".
   The IR-001 acceptance test S-7
   (`.moai/specs/SPEC-IR-001/acceptance.md:173-198`) shows Naver is admitted
   when `Category=korean, Lang="ko"` and excluded from English-only
   AdapterSets.
2. **Multi-vertical**: single `*Adapter` routes to web/news/blog/shop based
   on `Query.Filters[Key="naver_vertical"]`. This is the FIRST adapter in
   the codebase with multi-endpoint dispatch — ADP-009 (KoreaNewsCrawler +
   다음 + Korean RSS) will copy the pattern.
3. **Authenticated endpoint**: first M3 adapter requiring API credentials
   (`X-Naver-Client-Id` + `X-Naver-Client-Secret`). Validates the
   `Capabilities.RequiresAuth=true` + `AuthEnvVars=[...]` path through the
   registry, which previous ADP-001 + ADP-002 (both no-auth) did not exercise.
4. **HTML markup in body**: Naver wraps matched keywords in `<b>` tags and
   user-generated content carries HTML entities. The `stripHTML` helper
   established by SPEC-ADP-002 §6.2 generalises here; downstream Korean
   adapters (ADP-009) will inherit the same shape.
5. **DataLab opt-in**: Naver DataLab trends API is a DIFFERENT response
   shape (time-series rather than items). v0.1 surfaces it as opt-in via
   filter; the contract demonstrates that the adapter framework can host
   non-search-shaped sources without contract amendments.
6. **Quota pressure**: 25,000/day per app — 4× lower than HN's effectively-
   unlimited Algolia and 2500× lower than Reddit's 10/min unauth path.
   Quota exhaustion (429) becomes a realistic operational signal. The
   adapter does NOT manage quota internally (per SPEC-FAN-001 D6); it
   surfaces 429 with `RetryAfter` and lets fanout / SPEC-EVAL-002 own
   health-state tracking.

The adapter does NOT do fanout (SPEC-FAN-001 owns goroutine dispatch),
does NOT do retry (SPEC-FAN-001 D6 says zero retry in v0.1), does NOT do
caching (SPEC-CACHE-001 owns 5-phase fallback), does NOT do ranking fusion
(SPEC-IDX-001 owns RRF), does NOT do Korean tokenization (SPEC-IDX-003
owns mecab-ko), does NOT emit any per-call metric/log/span itself (the
registry wrappedAdapter does, sole-emitter discipline). It DOES one job
in five clothing: turn a `types.Query` into a Naver Open API request,
parse the JSON envelope into per-vertical NormalizedDocs, and return
`[]types.NormalizedDoc` or `*types.SourceError`.

Completion is part of the M3 thin slice: combined with SPEC-FAN-001 and
the other six M3 ADP-* SPECs (ADP-003..009), `usearch query "한글 질문"`
returns Korean-ranked-first results per `.moai/project/roadmap.md:150` —
the M3 exit criterion. SPEC-IDX-003 (Korean tokenization) consumes
ADP-008's plain-text Body for mecab-ko tokenization. SPEC-CLI-001 wires
the adapter into the registry registration phase.

---

## 2. Scope

### 2.1 In-Scope

| Ref | Item |
|-----|------|
| a | `internal/adapters/naver/naver.go`: `Adapter` struct (HTTP client + base URL + user-agent + healthcheck target + clientID + clientSecret pulled from env at construction), `New(opts Options) (*Adapter, error)` constructor (validates env presence when `Options.ClientID == ""`, returns `ErrAuthMissing` when `NAVER_CLIENT_ID` / `NAVER_CLIENT_SECRET` are unset), `Name() string` returning `"naver"`, `Capabilities() types.Capabilities` returning a deterministic descriptor (RequiresAuth=true, AuthEnvVars=`["NAVER_CLIENT_ID","NAVER_CLIENT_SECRET"]`, DocTypes=`[DocTypeArticle, DocTypePost]`, SupportedLangs=`["ko"]`, SupportsSince=false (V1 hardcodes default sort), RateLimitPerMin=17, DefaultMaxResults=25, DisplayName="Naver", Notes documenting the supported verticals + default=blog + DataLab opt-in + 25k/day quota), and `Healthcheck(ctx) error` (TCP-connect probe to `openapi.naver.com:443` with caller-supplied ctx, target injectable via Options). Compile-time interface assertion `var _ types.Adapter = (*Adapter)(nil)` at the bottom. |
| b | `internal/adapters/naver/search.go`: `(*Adapter).Search(ctx, q types.Query) ([]types.NormalizedDoc, error)` — the hot path. Validates the query (REQ-ADP8-011), resolves the vertical (REQ-ADP8-004), dispatches to `searchVertical(ctx, vertical, q)` for blog/news/web/shop or `searchDataLab(ctx, q)` for datalab (REQ-ADP8-010), parses any `q.Cursor` as a positive integer `start` value (REQ-ADP8-005), builds the request URL via `url.Values` with `query`, `display` (clamped 1-100), `start` (1-1000), and `sort` (default `sim`, opt-in `date`), delegates HTTP execution to `client.go`, delegates response parsing to `parse.go` per vertical, returns `[]NormalizedDoc` or `*SourceError`. Honours `ctx` cancellation throughout. |
| c | `internal/adapters/naver/client.go`: HTTP client construction (timeout=10s default, `CheckRedirect` enforces a domain allowlist `{openapi.naver.com}` with max 3 hops, `Transport` wrapped with `internal/obs/reqid.NewTransport(http.DefaultTransport)` for request-ID propagation), `doRequest(req *http.Request) (*http.Response, error)` helper that sets the four headers (`User-Agent`, `Accept: application/json`, `X-Naver-Client-Id`, `X-Naver-Client-Secret`) on every request, and `categorizeStatus(httpStatus int, retryAfter time.Duration, cause error) *types.SourceError` mapping HTTP status → Category per the table in §6 (with 401/403 carrying an inline note that auth is the likely failure mode). |
| d | `internal/adapters/naver/parse.go`: `parseResponse(body []byte, vertical string, retrievedAt time.Time, currentStart int, requestedDisplay int) ([]types.NormalizedDoc, string, error)` — parses the search envelope into `[]NormalizedDoc` and returns the next-page cursor as the second return value. Dispatches to per-vertical sub-parsers (`parseBlogItem`, `parseNewsItem`, `parseWebItem`, `parseShopItem`) per the field-mapping tables in §6.3. Empty `items` array returns `(nil, "", nil)`. Malformed JSON returns `*SourceError{Category: CategoryPermanent, Cause: <json error>}`. |
| e | `internal/adapters/naver/datalab.go`: `searchDataLab(ctx, q)` — the opt-in POST path. Parses `q.Text` as JSON into a `dataLabRequest` struct, marshals back to bytes for the POST body, executes the request via `doRequest`, parses the response via `parseDataLab(body, retrievedAt)`, returns one synthetic `NormalizedDoc` per `keywordGroups` row. |
| f | `internal/adapters/naver/strip.go`: `stripHTML(s string) string` — conservative stdlib-only tag-strip + entity-decode helper. Mirrors `internal/adapters/hn/strip.go` shape. Handles `<b>`, `<i>`, `<a>`, `<br>`, `<p>`, `<code>`, `<pre>`, `&amp;`, `&lt;`, `&gt;`, `&quot;`, `&#39;`, `&nbsp;`, plus `<b>` highlight markers from Naver. NOT a security boundary. |
| g | `internal/adapters/naver/score.go`: `defaultScore() float64` — returns 0.5. Naver search response carries no engagement metric per Research §3.2; all docs receive the neutral middle. The function is intentionally simple (returns a constant) but lives in its own file to mirror ADP-001 / ADP-002 layout symmetry. |
| h | `internal/adapters/naver/errors.go`: package-private sentinels `ErrInvalidQuery = errors.New("naver: query text empty or whitespace-only")` (wrapped in `*SourceError{Category: CategoryPermanent}` by Search), `ErrInvalidVertical = errors.New("naver: invalid vertical")` (vertical filter has unknown value; wrapped in `CategoryPermanent`), `ErrInvalidCursor = errors.New("naver: cursor must be positive integer (1-1000)")` (cursor out of range; wrapped in `CategoryPermanent`), `ErrAuthMissing = errors.New("naver: NAVER_CLIENT_ID and NAVER_CLIENT_SECRET env vars required")` (returned by `New` when env not set and `Options.ClientID` not provided); `parseRetryAfter` helper duplicated from SPEC-ADP-001 §6.3. |
| i | `internal/adapters/naver/naver_test.go`: tests for Adapter interface conformance, `Name()`, `Capabilities()` determinism, `Healthcheck()`, `New()` validation (env presence required when Options.ClientID empty; `SkipAuthCheck`-equivalent via Options bypass for tests). |
| j | `internal/adapters/naver/search_test.go`: the largest test file. Drives `(*Adapter).Search` against `httptest.Server` per vertical: happy path 25 results blog, news, web, shop; vertical filter dispatch (5 valid + 1 invalid); empty result; 429 with Retry-After; 401 (auth-missing-on-server-side); 4xx; 5xx; redirect to allowed and disallowed hosts; pagination cursor round-trip; ctx cancellation mid-request; invalid cursor rejection; sort filter override (date vs sim); concurrent-safety. |
| k | `internal/adapters/naver/client_test.go`: HTTP client unit tests — `categorizeStatus` truth table over 7 status codes, `parseRetryAfter` table over 6 input shapes, redirect allowlist enforcement, all four headers (User-Agent, Accept, X-Naver-Client-Id, X-Naver-Client-Secret) presence on every request. |
| l | `internal/adapters/naver/parse_test.go`: field-mapping unit tests — table over per-vertical fixtures (blog with HTML entities; news with `originallink` empty; web bare; shop full; mixed `<b>` highlights). Asserts each NormalizedDoc field per the §6.3 mapping table. Snippet truncation to 280 runes. Pagination cursor round-trip. Hash field is empty. |
| m | `internal/adapters/naver/strip_test.go`: `stripHTML` table-driven test over 8+ inputs (empty, plain text, `<b>` highlight only, nested tags, malformed unclosed, all five canonical entities, mixed tags + entities, very long body). |
| n | `internal/adapters/naver/datalab_test.go`: DataLab POST path tests — happy path (3 keyword groups, 30 days timeUnit=date), malformed `q.Text` JSON rejection, response with empty results array. |
| o | `internal/adapters/naver/score_test.go`: trivial — assert `defaultScore() == 0.5` and determinism. |
| p | `internal/adapters/naver/bench_test.go`: `BenchmarkParseBlogResponse25Items` (NFR-ADP8-001 — p50 ≤ 5 ms parse time on amd64 for a 25-item Blog response fixture; allocation ≤ 20 allocs per item parsed = ≤ 500 allocs total). `TestMain` invokes `goleak.VerifyTestMain(m)` (NFR-ADP8-004). |
| q | `internal/adapters/naver/testdata/`: golden JSON fixtures — `search_response_blog.json` (25-item happy path, ~6KB), `search_response_news.json` (25-item with mix of populated and empty `originallink`, ~6KB), `search_response_web.json` (25-item bare title/link/description, ~5KB), `search_response_shop.json` (25-item with full shopping fields, ~8KB), `search_response_blog_empty.json` (zero items, ~150B), `search_response_blog_pagination.json` (page 1 of 5; total=125; start=26 cursor expected, ~6KB), `search_response_blog_html_entities.json` (HTML `<b>` highlights + 5 entities + nested tags, ~2KB), `search_response_news_no_originallink.json` (single news item with `originallink=""`, ~600B), `search_response_blog_malformed_postdate.json` (postdate=`""` and postdate=`"abc"` cases, ~600B), `search_response_blog_malformed.json` (truncated JSON for parse-error path, ~250B), `datalab_response.json` (3 keyword groups × 30 daily ratios, ~3KB). |

### 2.2 Out-of-Scope

[HARD] This SPEC explicitly excludes the following items. Each has a known
destination SPEC; this list prevents scope creep into ADP-008 and reaffirms
the multi-vertical reference shape.

- **MCP-server subprocess wrapping** of `isnow890/naver-search-mcp` →
  REJECTED per D1. Direct REST is the chosen path; the MCP repo is
  consulted as a tool taxonomy reference only.
- **Per-source customisations specific to other Korean sources** (다음 /
  Daum, KoreaNewsCrawler, RSS) → SPEC-ADP-009.
- **Per-source customisations for non-Korean adapters** (arXiv, GitHub,
  YouTube, Bluesky, X, SearXNG, Polymarket) → SPEC-ADP-003 through
  SPEC-ADP-007 + SPEC-ADP-006.
- **Retry orchestration** (exponential backoff, circuit breaker,
  per-source health-state tracking, jitter, max-attempt counters) →
  SPEC-FAN-001 D6. The adapter returns one categorised error per request
  and does not retry.
- **Response caching** (in-process LRU, Redis-backed, on-disk) →
  SPEC-CACHE-001 (M3). Each `Search` call is independent and idempotent
  at the adapter layer.
- **Result ranking, deduplication, RRF fusion across adapters** →
  SPEC-IDX-001 (M3). The adapter returns Naver-relevance order with
  `Score=0.5` (all docs equal), but does not re-rank.
- **Korean tokenization** (mecab-ko) → SPEC-IDX-003 (M3). The adapter
  emits plain stripped text in `Body`; tokenization is downstream's job.
- **Cross-vertical aggregation** (single Search call returning
  blog+news+web mixed) → out of v0.1. One Search call → one vertical.
  Future SPEC-ADP-008a may add a `naver_vertical=union` shortcut.
- **`DocTypeProduct` enum addition** to `pkg/types/capabilities.go` for
  shopping items → out of v0.1. Adding a `DocType` constant breaks the
  `pkg/types` SDK boundary semver per `structure.md:160-161`. Defer to
  coordinated SPEC-CORE-001a if SPEC-IDX-001 needs the distinction.
  Shopping items return `DocType=DocTypeOther`.
- **DataLab Shopping Insight tools** (`datalab_shopping_*` from the MCP
  taxonomy) → out of v0.1. The DataLab opt-in surface in v0.1 covers
  `/v1/datalab/search` (search trends) only; Shopping Insight has a
  different request shape and is deferred.
- **Tenant-scoped quota policy** (per-team quota of the 25k/day pool) →
  SPEC-AUTH-002 (M6). v0.1 honours the per-app pool literally.
- **Adapter health-state machine** (auto-disable on N consecutive
  `CategoryUnavailable`, auto-re-enable on Healthcheck pass) →
  SPEC-EVAL-002 (M8). The adapter is stateless.
- **OAuth-authenticated variant** (Naver Login OAuth instead of API key) →
  out of scope. The Open Search API uses static client-id / client-secret
  per Research §1.2.
- **Live network integration tests in CI** → out of v0.1.
  `httptest.Server` + golden fixtures only. Optional env-gated live test
  (`-tags=integration` + `NAVER_LIVE=1`) deferred to a future follow-up.
- **OpenAPI / proto schema for the adapter response** — the
  `[]types.NormalizedDoc` return type IS the schema; no separate IDL.
- **`pkg/llm` integration** — the Naver adapter does NOT call any LLM.
  Classification is the Intent Router's job (SPEC-IR-001).
- **Pre-flight Query validation beyond text-emptiness + vertical-validity +
  cursor-validity** — Naver accepts long queries; the adapter does not
  enforce additional length limits.
- **Per-adapter custom Prometheus metrics** (e.g.,
  `naver_vertical_calls_total{vertical}`) — would require amending
  SPEC-OBS-001's allowlist; out of scope. The shared
  `AdapterCalls{adapter="naver",outcome}` family is sufficient for v0.1.
  Per-vertical visibility lives in `Metadata["naver_vertical"]` for
  downstream correlation.
- **Cross-adapter helper extraction** (sharing `parseRetryAfter`,
  `categorizeStatus`, redirect allowlist, `stripHTML` between Reddit / HN /
  Naver) → out of v0.1. Three duplicated copies after ADP-008 lands; the
  rule of three is reached. SPEC-ADP-REFAC-001 (post-M3, after seven M3
  adapters) MAY consolidate. Per SPEC-ADP-002 §11.4 same disposition.

### 2.3 Vertical Selection Rules

[HARD] The `Search` method dispatches to one of FIVE verticals per the
`Query.Filters[Key="naver_vertical"]` lookup:

| Filter Value | Vertical | Endpoint | DocType |
|--------------|----------|----------|---------|
| `"blog"` | Blog | `/v1/search/blog.json` (GET) | `DocTypePost` |
| `"news"` | News | `/v1/search/news.json` (GET) | `DocTypeArticle` |
| `"web"` | Web (webkr) | `/v1/search/webkr.json` (GET) | `DocTypeArticle` |
| `"shop"` | Shopping | `/v1/search/shop.json` (GET) | `DocTypeOther` |
| `"datalab"` | DataLab trends | `/v1/datalab/search` (POST) | `DocTypeOther` |
| (absent or `""`) | Default = `blog` | `/v1/search/blog.json` (GET) | `DocTypePost` |
| any other | Reject | — | `*SourceError{CategoryPermanent}, Cause: ErrInvalidVertical` |

Lookup is case-sensitive (e.g., `"News"` is rejected). `Capabilities.Notes`
enumerates valid values verbatim.

The vertical chosen for a given request is stored in
`NormalizedDoc.Metadata["naver_vertical"]` for downstream visibility
(synthesis, dedup analysis, per-vertical evaluation in SPEC-EVAL-003) but
does NOT escape to the registry's Prometheus labels — `adapter="naver"`
remains the sole adapter-name label per SPEC-OBS-001 cardinality discipline.

### 2.4 Score Default (Architecture)

[HARD] All NormalizedDocs returned by ADP-008 carry `Score=0.5`. Naver's
search response envelope does NOT include a per-item engagement metric
(no upvotes, no view count, no comment count). All four search verticals
share this property. Setting all docs to the neutral midpoint of the
`[0.0, 1.0]` codomain is the simplest deterministic default.

Properties:

- **Determinism**: pure constant; no time, no I/O, no state.
- **No false ranking signal**: SPEC-IDX-001 (M3) RRF fuses by RANK across
  adapters, not by raw score. A constant per-adapter score does not
  destabilise RRF; it just means within-adapter rank from Naver's
  relevance order is the only intra-Naver signal.
- **Future-compatible**: a future SPEC-ADP-008a may derive a non-constant
  score from `pubDate` recency (news), `postdate` recency (blog), or
  `lprice` for shopping. Open Question §11.5 documents revisit triggers.

The default function is `score.go::defaultScore() float64 { return 0.5 }`.
Tests assert `defaultScore() == 0.5` exactly and determinism across two
calls.

---

## 3. EARS Requirements

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| REQ-ADP8-001 | Ubiquitous | The package `internal/adapters/naver` SHALL expose an `Adapter` struct that implements `pkg/types.Adapter` exactly: `Name() string` returning `"naver"`, `Search(ctx context.Context, q types.Query) ([]types.NormalizedDoc, error)`, `Healthcheck(ctx context.Context) error`, `Capabilities() types.Capabilities`. The package SHALL include a compile-time interface assertion `var _ types.Adapter = (*Adapter)(nil)`. `Capabilities()` SHALL be deterministic (two consecutive calls return equal values) with `SourceID="naver"`, `DisplayName="Naver"`, `DocTypes=[DocTypeArticle, DocTypePost]`, `SupportedLangs=["ko"]`, `SupportsSince=false`, `RequiresAuth=true`, `AuthEnvVars=["NAVER_CLIENT_ID", "NAVER_CLIENT_SECRET"]`, `RateLimitPerMin=17`, `DefaultMaxResults=25`, and `Notes` containing the substrings `"Naver Open API"`, `"verticals: blog (default), news, web, shop, datalab"`, `"Korean-locale primary"`, `"25000/day per app"`, and `"requires NAVER_CLIENT_ID + NAVER_CLIENT_SECRET"`. | P0 | `TestAdapterName`, `TestAdapterImplementsInterface` (compile-time), `TestCapabilitiesDeterministic`, `TestCapabilitiesShape` (asserts all 9 documented field values + Notes substring matches), `TestHealthcheckSucceeds` (stub `httptest.Server` injected via Options). All in `internal/adapters/naver/naver_test.go`. |
| REQ-ADP8-002 | Event-Driven | WHEN `(*Adapter).Search(ctx, q)` is invoked with a non-empty `q.Text` AND a valid (or absent) vertical filter resolving to one of the GET verticals (`blog`/`news`/`web`/`shop`), the adapter SHALL build an HTTP GET request to the matching endpoint (`https://openapi.naver.com/v1/search/<vertical>.json`) with the following query parameters: `query=<url.QueryEscape(q.Text)>`, `display=clamp(q.MaxResults, 1, 100)` (defaulting to 25 when `q.MaxResults == 0`), `start=<parsed cursor>` (defaulting to 1 when `q.Cursor == ""`), and `sort=<resolved sort>` (default `sim`, opt-in `date` per REQ-ADP8-008). The adapter SHALL execute the request via the constructed `*http.Client`, parse the JSON envelope per REQ-ADP8-006 mapping, and return `(docs, nil)` on HTTP 200 with `len(docs) ≤ 100`. | P0 | `TestSearchHappyPathBlog25Items` (httptest.Server returns `search_response_blog.json`; assert 25 NormalizedDocs returned, each with `Validate()` returning nil); same against news / web / shop fixtures; `TestSearchURLParametersIncludeAllRequired` (inspect captured request URL; assert `query`, `display`, `start`, `sort` always present); `TestSearchClampsDisplayTo100` (q.MaxResults=500 → URL has `display=100`); `TestSearchDefaultsDisplayTo25` (q.MaxResults=0 → URL has `display=25`); `TestSearchOmitsStartWhenCursorEmpty` (q.Cursor="" → URL has `start=1`); `TestSearchSetsStartWhenCursorPresent` (q.Cursor="26" → URL contains `start=26`). All in `search_test.go`. |
| REQ-ADP8-003 | Event-Driven | WHEN any outbound HTTP request is constructed by the Naver adapter, the adapter SHALL set the following four headers on the request: `User-Agent: usearch/<version> (+https://github.com/elymas/universal-search)` (custom UA per project convention), `Accept: application/json`, `X-Naver-Client-Id: <opts.ClientID || os.Getenv("NAVER_CLIENT_ID")>`, `X-Naver-Client-Secret: <opts.ClientSecret || os.Getenv("NAVER_CLIENT_SECRET")>`. WHEN `New(opts Options)` is called and BOTH `opts.ClientID == ""` AND `os.Getenv("NAVER_CLIENT_ID") == ""` (or symmetrically for ClientSecret), `New` SHALL return `(nil, ErrAuthMissing)`. The adapter SHALL NOT log the secret value at any level; the secret SHALL appear only in outbound request headers and SHALL NOT be embedded in `Capabilities.Notes` or any error message. | P0 | `TestNewRequiresClientIDEnv` (unset env, default Options → `errors.Is(err, ErrAuthMissing)`); `TestNewRequiresClientSecretEnv` (clientID set but secret unset → `ErrAuthMissing`); `TestNewAcceptsExplicitOptions` (Options.ClientID + Options.ClientSecret set, env unset → success); `TestSearchSendsAuthHeaders` (inspect captured request; assert both X-Naver-* headers populated with expected values); `TestSearchNeverLogsSecret` (capture slog JSON across happy + error paths; assert no record contains the literal secret value); `TestNotesDoesNotContainSecret` (`strings.Contains(caps.Notes, secretValue) == false`). |
| REQ-ADP8-004 | Optional | WHERE `Query.Filters` contains an entry with `Key == "naver_vertical"`, the adapter SHALL resolve `vertical = filters["naver_vertical"]`. The valid values are `"blog"`, `"news"`, `"web"`, `"shop"`, `"datalab"` (case-sensitive, exact match). WHERE the value is none of the five valid strings AND not the empty string, the adapter SHALL return `(nil, &types.SourceError{Adapter:"naver", Category: types.CategoryPermanent, Cause: ErrInvalidVertical})` immediately and SHALL NOT issue any HTTP request. WHERE the filter is absent OR has an empty value, the adapter SHALL default to `vertical = "blog"`. | P1 | `TestSearchVerticalBlog` (Filters=[{naver_vertical, blog}] → URL path = `/v1/search/blog.json`); same for news/web/shop; `TestSearchVerticalDataLab` (Filters=[{naver_vertical, datalab}] → POST to `/v1/datalab/search`); `TestSearchVerticalDefaultsToBlog` (Filters=nil → URL path = `/v1/search/blog.json`); `TestSearchVerticalEmptyValueDefaultsToBlog` (Filters=[{naver_vertical, ""}] → blog); `TestSearchVerticalInvalidRejected` (Filters=[{naver_vertical, "Image"}] → `errors.Is(err, types.ErrPermanent)`, AND `errors.Is(err, ErrInvalidVertical)`, AND zero HTTP requests observed at httptest.Server). All in `search_test.go`. |
| REQ-ADP8-005 | Event-Driven | WHEN `Query.Cursor` is non-empty, the adapter SHALL parse it via `strconv.Atoi`, accept positive integers in the range `[1, 1000]` (Naver's effective `start` ceiling), and use the parsed value as the `start` query parameter. WHEN parsing fails OR the integer is outside `[1, 1000]`, the adapter SHALL return `(nil, &types.SourceError{Adapter:"naver", Category: types.CategoryPermanent, Cause: ErrInvalidCursor})` immediately and SHALL NOT issue any HTTP request. WHEN `Query.Cursor` is empty, the adapter SHALL default `start=1`. The next-page cursor SHALL be surfaced via `Metadata["next_cursor"]` on the LAST returned NormalizedDoc, encoded as `strconv.Itoa(currentStart + len(returnedDocs))`, only WHEN `(currentStart + len(returnedDocs)) <= 1000` AND `(currentStart + len(returnedDocs)) <= total` (Naver's reported total in the envelope). Otherwise the `next_cursor` key SHALL be omitted (no further pages). | P0 | `TestSearchCursorRoundTrip` (q.Cursor="26", 25 items returned, total=125 → next_cursor="51" on last doc); `TestSearchCursorOmittedAtEnd` (start=976, 25 items, total=1000 → next doc would be 1001 > 1000 cap; assert no `next_cursor` key on any doc); `TestSearchCursorOmittedAtTotalEnd` (start=51, returns 10 items, total=60 → next would be 61 > 60; no cursor); `TestSearchInvalidCursorRejected` (table over `["abc", "0", "-1", "1.5", "1001", "9999"]` → ErrInvalidCursor + zero requests). All in `search_test.go`. |
| REQ-ADP8-006 | Ubiquitous | The adapter SHALL transform each Naver Open API search response item into one `types.NormalizedDoc` per the per-vertical field mappings in §6.3. Common invariants for all four search verticals: `SourceID = "naver"`; `Lang = "ko"`; `Score = 0.5` per §2.4; `RetrievedAt = time.Now().UTC()` set at parse time; `Hash = ""`; `Citations = nil`; `Title`, `Body`, `Snippet` SHALL be `stripHTML`-processed per REQ-ADP8-009; `Snippet` SHALL be the first 280 runes of stripped `description` (truncated with "…" suffix when longer), falling back to `stripHTML(title)` truncated to 280 runes when description is empty. `Metadata` SHALL contain at minimum the key `"naver_vertical"` with the resolved vertical name (`"blog"`, `"news"`, `"web"`, `"shop"`). Empty `items` arrays return `(nil, "", nil)`; malformed JSON returns `*SourceError{Category: CategoryPermanent}`. | P0 | `TestParseResponseBlogFieldMapping` (table over `search_response_blog.json` 25 items; assert every field per §6.3 row "Blog"); same for news / web / shop; `TestParseResponseEmpty` (empty `items` array → `(nil, "", nil)`); `TestParseResponseMalformed` (truncated JSON → `*SourceError{CategoryPermanent}`); `TestParseResponseHashEmpty` (every doc has `Hash == ""`); `TestParseResponseLangIsKo` (every doc has `Lang == "ko"`); `TestParseResponseScoreIsHalf` (every doc has `Score == 0.5`); `TestParseResponseSnippetTruncated` (description >280 runes → snippet ends with "…" and is exactly 280 runes long); `TestParseResponseSnippetFallsBackToTitle` (description="" → snippet derived from title). All in `parse_test.go`. |
| REQ-ADP8-007 | Event-Driven | WHEN HTTP 401 is received from any Naver endpoint, the adapter SHALL return `(nil, &types.SourceError{Adapter:"naver", Category: types.CategoryPermanent, HTTPStatus: 401, Cause: errors.New("naver: HTTP 401 (auth: check NAVER_CLIENT_ID / NAVER_CLIENT_SECRET)")})`. WHEN HTTP 403 is received (application disabled or per-app quota exhausted at the application level), the adapter SHALL return `(nil, &types.SourceError{Adapter:"naver", Category: types.CategoryPermanent, HTTPStatus: 403, Cause: errors.New("naver: HTTP 403 (application disabled or quota exhausted)")})`. WHEN HTTP 400 or 404 is received, the adapter SHALL return `*SourceError{CategoryPermanent}` with the corresponding HTTPStatus. WHEN HTTP 429 is received, the adapter SHALL parse `Retry-After` per RFC 7231 §7.1.3 (integer-seconds OR HTTP-date), default to 5 seconds when missing/malformed, cap at 60 seconds, and return `*SourceError{Category: CategoryRateLimited, HTTPStatus: 429, RetryAfter: <duration>}`. WHEN HTTP 500/502/503/504 is received OR a connection error occurs, the adapter SHALL return `*SourceError{Category: CategoryUnavailable, HTTPStatus: <code or 0>}`. The adapter SHALL NOT retry. | P0 | `TestSearchHTTP401AuthError` (assert error message contains "auth"); `TestSearchHTTP403QuotaExhausted` (assert message contains "quota"); `TestSearchHTTP400`, `TestSearchHTTP404` (each → ErrPermanent); `TestSearchHTTP429WithIntegerRetryAfter` (`Retry-After: 30` → RetryAfter=30s); `TestSearchHTTP429WithHTTPDateRetryAfter` (HTTP-date 30s ahead → RetryAfter ∈ (25s, 35s)); `TestSearchHTTP429NoRetryAfterDefaults5s`; `TestSearchHTTP429RetryAfterCapped60s`; `TestSearchHTTP429NoInternalRetry` (request count == 1); `TestSearchHTTP500` / `TestSearchHTTP503` (each → ErrSourceUnavailable + matching HTTPStatus); `TestSearchConnectionRefused` (server closed → HTTPStatus=0 + ErrSourceUnavailable); `TestSearchUnavailablePreservesUnderlyingError` (unwrap chain reveals inner cause). All in `search_test.go` + `client_test.go`. |
| REQ-ADP8-008 | Optional | WHERE `Query.Filters` contains an entry with `Key == "sort"` AND `Value == "date"`, the adapter SHALL set `sort=date` in the request URL (reverse-chronological order). WHERE the `sort` filter is absent OR has any other value (`""`, `"sim"`, `"relevance"`, etc.), the adapter SHALL set `sort=sim` (Naver's relevance default). The adapter SHALL NOT return an error for an unknown `sort` value (silently default to `sim`, matching the SPEC-ADP-002 REQ-ADP2-007 unknown-filter discipline). | P1 | `TestSearchSortDate` (Filters=[{sort, date}] → URL has `sort=date`); `TestSearchSortSim` (Filters=[{sort, sim}] → URL has `sort=sim`); `TestSearchSortAbsentDefaultsSim` (Filters=nil → `sort=sim`); `TestSearchSortUnknownDefaultsSim` (Filters=[{sort, "foo"}] → `sort=sim`, no error). All in `search_test.go`. |
| REQ-ADP8-009 | Ubiquitous | The adapter SHALL apply `stripHTML` to every text field copied from the Naver response into `NormalizedDoc.Title`, `Body`, and `Snippet`. The `stripHTML` function SHALL: (a) remove all `<tag>` and `</tag>` constructs (including but not limited to `<b>`, `<i>`, `<a>`, `<br>`, `<p>`, `<code>`, `<pre>`); (b) decode the five canonical HTML entities `&amp;`, `&lt;`, `&gt;`, `&quot;`, `&#39;` plus `&nbsp;` (collapsed to a single space); (c) leave malformed/unclosed tags pass through as plain text. The function is NOT a security boundary — the output is plain text consumed by synthesis, never rendered as HTML. | P0 | `TestStripHTMLTable` table-drives 8+ inputs: `("", "")`, `("plain", "plain")`, `("<b>x</b>", "x")`, `("<b>x</b> & <i>y</i>", "x & y")`, `("<b>한글</b>", "한글")`, `("&amp;hello&lt;b&gt;", "&hello<b>")`, `("&quot;quoted&quot;", "\"quoted\"")`, `("&nbsp;leading", " leading")`, `("<button>", "<button>"|"") (malformed unclosed; adapter chooses pass-through OR strip-balanced; document choice)`. In `strip_test.go`. Plus integration check in `parse_test.go::TestParseResponseStripsHTMLEntities` against `search_response_blog_html_entities.json` fixture. |
| REQ-ADP8-010 | Optional | WHERE `Query.Filters[Key="naver_vertical"][Value="datalab"]` is set, the adapter SHALL parse `Query.Text` as a JSON document conforming to the Naver DataLab search-trends request schema (`{startDate, endDate, timeUnit, keywordGroups: [{groupName, keywords: []}]}`), POST the marshalled JSON to `https://openapi.naver.com/v1/datalab/search` with `Content-Type: application/json` plus the four standard headers from REQ-ADP8-003, parse the response into one synthetic `NormalizedDoc` per `keywordGroups` row, and return `[]NormalizedDoc`. Each synthesized doc SHALL set: `SourceID="naver"`, `URL = "https://datalab.naver.com/keyword/trendResult.naver?hashKey=" + sha256(reqBody)[:16]`, `Title = "DataLab trend: " + groupName`, `Body` containing a human-readable summary of the time-series, `PublishedAt = parsed endDate`, `Author = "Naver DataLab"`, `Score = 0.5`, `Lang = "ko"`, `DocType = DocTypeOther`, `Metadata = {"naver_vertical":"datalab", "datalab_data": <serialised time-series>}`. WHEN `Query.Text` fails JSON parse OR the body fails Naver-side validation (HTTP 400), the adapter SHALL return `*SourceError{CategoryPermanent}`. | P1 | `TestSearchDataLabHappyPath` (3 keyword groups → 3 NormalizedDocs returned, each with the expected synthetic fields); `TestSearchDataLabMalformedQueryText` (q.Text="not JSON" → ErrPermanent + zero requests); `TestSearchDataLabUpstream400` (server returns 400 → ErrPermanent); `TestSearchDataLabPostsToCorrectEndpoint` (request URL matches `/v1/datalab/search`, method=POST). All in `datalab_test.go`. |
| REQ-ADP8-011 | Unwanted | IF `Query.Text` is empty OR contains only Unicode whitespace runes (per `unicode.IsSpace` over every rune), THEN the adapter SHALL return `(nil, &types.SourceError{Adapter:"naver", Category: types.CategoryPermanent, Cause: ErrInvalidQuery})` immediately and SHALL NOT issue any HTTP request. This includes the DataLab path (REQ-ADP8-010): an empty `Query.Text` is rejected before JSON parse is attempted. | P0 | `TestSearchEmptyQueryRejectedNoHTTP` (table over `["", "   ", "\t\n  \r"]`; for each asserts `errors.Is(err, types.ErrPermanent)` AND `errors.Is(err, ErrInvalidQuery)` AND zero HTTP requests at httptest.Server); same coverage extended for the datalab vertical. In `search_test.go`. |
| REQ-ADP8-012 | State-Driven | WHILE the same `*Adapter` instance is registered in the adapter registry and is being invoked concurrently from N goroutines (N ≥ 1), each `Search(ctx, q)` call SHALL execute independently with no shared mutable state across calls (the underlying `*http.Client` is goroutine-safe per Go stdlib; the adapter holds no per-call state; the `clientID` and `clientSecret` strings are immutable post-construction); the cumulative effect SHALL be N independent HTTP round-trips with no race-detector alarms. This requirement crystallises the concurrency contract that the registry (`internal/adapters/registry.go:172-263` wrappedAdapter) and the future fanout layer (SPEC-FAN-001) rely on. | P0 | `TestSearchConcurrentSafe` (50 goroutines each issuing one Search against the same httptest.Server; assert (a) no race-detector alarm under `-race`, (b) total response count = 50 observed at the stub, (c) all 50 returned slices are `[]types.NormalizedDoc` with `Validate()` returning nil for every doc, (d) the captured authorization headers are present and identical across all 50 requests). In `search_test.go`. |
| REQ-ADP8-013 | Optional | WHERE the response is HTTP 301/302/303/307/308, the adapter's `*http.Client.CheckRedirect` SHALL follow up to 3 redirects WITHIN the 1-host allowlist `{openapi.naver.com}`. Cross-domain redirects (any other host, including `naver.com` apex or `search.naver.com` HTML host) SHALL be rejected by returning an error from `CheckRedirect`; the adapter wraps this as `*SourceError{Adapter:"naver", Category: CategoryPermanent, Cause: errors.New("naver: cross-domain redirect rejected: <target host>")}` to prevent SSRF. Redirect chains exceeding 3 hops SHALL be rejected with a "too many redirects" message. While the Naver Open API does not redirect cross-domain in normal operation, the allowlist is defensive and consistent with SPEC-ADP-001 REQ-ADP-010 + SPEC-ADP-002 REQ-ADP2-009. | P1 | `TestSearchFollowsAllowlistRedirect` (httptest.Server returns 302 to a second httptest.Server with Host header rewritten to `openapi.naver.com`; assert NormalizedDocs returned); `TestSearchRejectsCrossDomainRedirect` (302 to `attacker.com` → ErrPermanent + message contains `"cross-domain redirect"`); `TestSearchRejectsRedirectChainOver3` (4-hop chain within allowlist → "too many redirects" rejection). All in `client_test.go`. |
| REQ-ADP8-014 | Ubiquitous | The Naver adapter SHALL emit ZERO Prometheus metric families, ZERO log records, and ZERO OTel spans of its own. ALL per-call observability SHALL come from the registry's `wrappedAdapter` at `internal/adapters/registry.go:195-219`. The adapter's `Search` SHALL return a correctly-categorised `*types.SourceError` so the wrappedAdapter's `OutcomeFromError(err)` (`pkg/types/errors.go:174-193`) computes the right `outcome` label. The adapter SHALL set `Lang="ko"` and `Metadata["naver_vertical"]=<resolved vertical>` on every emitted NormalizedDoc so downstream consumers (synthesis, evaluation, dedup) can correlate per-vertical signals. The `outcome` label SHALL remain bounded to the 5 SPEC-CORE-001 values (`success`, `failure`, `timeout`, `rate_limited`, `unavailable`) — no per-vertical sub-label is added. The `adapter_class` label space (used by `FanoutInflight` per SPEC-OBS-001) is bounded by the IR-001 Category enum and is NOT extended by ADP-008. | P0 | `TestNoNewMetricFamilies` (snapshot `prometheus.Registry.Gather()` before+after registration; assert family-count delta is zero); `TestNoSpansFromAdapter` (in-memory OTel exporter; call Search; assert the only `adapter.search` span is from the wrappedAdapter and has `adapter.name="naver"`); `TestNoLogsFromAdapter` (capture slog handler; the only "adapter call" record is from the wrappedAdapter); `TestMetadataNaverVerticalPresent` (every doc returned has `Metadata["naver_vertical"]` populated with one of the 4 search-vertical strings or `"datalab"`). |

---

## 4. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| NFR-ADP8-001 | Performance (parse path) | The parse path `parseResponse(body []byte, vertical string, retrievedAt time.Time, currentStart int, requestedDisplay int) ([]NormalizedDoc, string, error)` SHALL execute with mean wall-clock duration per op ≤ 5 ms over `go test -bench=BenchmarkParseBlogResponse25Items -benchtime=10x -count=5 ./internal/adapters/naver/...` on amd64; the median of the 5 runs is the assertion value (passes when ≤ 5 ms). The fixture is the `search_response_blog.json` golden (25-item Blog response, ~6KB). Allocation count ≤ 20 per item parsed (i.e. ≤ 500 allocs total for 25 items) per the same benchmark's `allocs/op` field — the same floor analysis from SPEC-ADP-001 NFR-ADP-001 applies (the `pkg/types.NormalizedDoc.Metadata = map[string]any` contract from SPEC-CORE-001 forces a structural floor of ~17 allocs/doc; Naver's `stripHTML` adds modest extra allocs on items with HTML markup but the standard 25-item fixture stays within 20/item average). Measured via `BenchmarkParseBlogResponse25Items` in `internal/adapters/naver/bench_test.go`, run weekly in CI per the cadence established in SPEC-OBS-001 NFR-OBS-001. Benchmarks do not count toward coverage. |
| NFR-ADP8-002 | End-to-end Latency | The end-to-end `Search` round-trip against the `httptest.Server` stub (no real network) SHALL complete with p95 ≤ 200 ms over 100 invocations, measured by `TestSearchE2ELatencyStubP95` in `search_test.go` (sort durations ascending, assert `durations[94] ≤ 200ms`). The harder live-Naver p95 (≤ 3s, between Algolia HN's ≤ 2s and Reddit's ≤ 5s — Naver Open API is reasonably fast inside Korea, slower outside) is documented as the operational target but is NOT enforced in CI (no live network). |
| NFR-ADP8-003 | Race-clean concurrent invocation | `internal/adapters/naver/search_test.go::TestSearchConcurrentSafe` SHALL execute successfully under `go test -race ./internal/adapters/naver/...` with the workload defined in REQ-ADP8-012: 50 goroutines × one Search per `*Adapter` against a single shared httptest.Server. Race-detector alarms attributable to the naver adapter package SHALL be zero. |
| NFR-ADP8-004 | Zero goroutine leaks | The adapter SHALL NOT leak any goroutine across any code path (success, partial-failure, ctx-cancellation mid-flight, per-adapter timeout, panic-recovery via standard ctx propagation). Verified by `internal/adapters/naver/bench_test.go::TestMain` invoking `goleak.VerifyTestMain(m)` (mirrors SPEC-ADP-001 + SPEC-ADP-002 pattern) and by `TestSearchNoGoroutineLeakOnCancel` in `search_test.go` using `goleak.VerifyNone(t)` after a `Search` call whose ctx is cancelled mid-flight via a 50ms-delayed cancel against a stub server delaying response by 200ms. |

---

## 5. Acceptance Criteria

### REQ-ADP8-001 — Adapter Interface Conformance

- File `internal/adapters/naver/naver.go` declares `Adapter` struct with the
  documented fields (`httpClient *http.Client`, `searchBaseURL string`,
  `datalabBaseURL string`, `userAgent string`, `clientID string`,
  `clientSecret string`, `healthcheckTarget string`).
- The compile-time assertion `var _ types.Adapter = (*Adapter)(nil)` appears
  at the bottom of `naver.go`.
- `(*Adapter).Name()` returns the literal string `"naver"`.
- `(*Adapter).Capabilities()` returns a `types.Capabilities` matching all
  9 documented field values exactly (`SourceID="naver"`,
  `DisplayName="Naver"`, `DocTypes=[DocTypeArticle, DocTypePost]`,
  `SupportedLangs=["ko"]`, `SupportsSince=false`, `RequiresAuth=true`,
  `AuthEnvVars=["NAVER_CLIENT_ID","NAVER_CLIENT_SECRET"]`,
  `RateLimitPerMin=17`, `DefaultMaxResults=25`).
- `Capabilities().Notes` contains all five required substrings: `"Naver
  Open API"`, `"verticals: blog (default), news, web, shop, datalab"`,
  `"Korean-locale primary"`, `"25000/day per app"`,
  `"requires NAVER_CLIENT_ID + NAVER_CLIENT_SECRET"`.
- `(*Adapter).Capabilities()` called twice returns `reflect.DeepEqual`
  results.
- `(*Adapter).Healthcheck(ctx)` succeeds against an httptest.Server bound
  to `127.0.0.1:0` via `Options.HealthcheckTarget`.

### REQ-ADP8-002 — Search Happy Path (per vertical)

- `TestSearchHappyPathBlog25Items` against
  `testdata/search_response_blog.json` returns 25 NormalizedDocs; each
  passes `Validate()`; URL contains `/v1/search/blog.json`.
- `TestSearchHappyPathNews25Items` against
  `testdata/search_response_news.json` returns 25 NormalizedDocs.
- `TestSearchHappyPathWeb25Items` against
  `testdata/search_response_web.json` returns 25 NormalizedDocs.
- `TestSearchHappyPathShop25Items` against
  `testdata/search_response_shop.json` returns 25 NormalizedDocs.
- `TestSearchURLParametersIncludeAllRequired`,
  `TestSearchClampsDisplayTo100`, `TestSearchDefaultsDisplayTo25`,
  `TestSearchOmitsStartWhenCursorEmpty` (defaults to `start=1`),
  `TestSearchSetsStartWhenCursorPresent` all pass.

### REQ-ADP8-003 — Authentication Headers and Env Resolution

- `TestNewRequiresClientIDEnv`: `os.Unsetenv("NAVER_CLIENT_ID")`; `New(Options{ClientSecret: "x"})` returns `(nil, ErrAuthMissing)`.
- `TestNewRequiresClientSecretEnv`: ClientID set, secret unset → ErrAuthMissing.
- `TestNewAcceptsExplicitOptions`: env unset; `Options{ClientID:"x", ClientSecret:"y"}` → success.
- `TestNewAcceptsEnvFallback`: `os.Setenv("NAVER_CLIENT_ID", "x"); os.Setenv("NAVER_CLIENT_SECRET", "y")`; `New(Options{})` → success.
- `TestSearchSendsAuthHeaders`: stub captures incoming request; assert `r.Header.Get("X-Naver-Client-Id") == "x"` and `r.Header.Get("X-Naver-Client-Secret") == "y"`.
- `TestSearchAlsoSendsUAAndAccept`: assert `User-Agent` starts with `"usearch/"` and `Accept == "application/json"`.
- `TestSearchNeverLogsSecret`: capture slog JSON across happy + 401 + 5xx paths; assert `strings.Contains(logBytes, "y") == false` (the secret value).
- `TestNotesDoesNotContainSecret`: `strings.Contains(caps.Notes, "y") == false`.

### REQ-ADP8-004 — Vertical Selection

- Five `TestSearchVertical*` tests assert each valid vertical routes to the
  expected endpoint path (`/v1/search/blog.json`, `/news.json`,
  `/webkr.json`, `/shop.json`, and `POST /v1/datalab/search` for datalab).
- `TestSearchVerticalDefaultsToBlog` (Filters=nil → blog).
- `TestSearchVerticalEmptyValueDefaultsToBlog` (Filters=[{naver_vertical, ""}] → blog).
- `TestSearchVerticalInvalidRejected` (Filters=[{naver_vertical, "Image"}] →
  ErrPermanent + ErrInvalidVertical + zero HTTP requests).

### REQ-ADP8-005 — Pagination Cursor

- `TestSearchCursorRoundTrip`: q.Cursor="26"; httptest stub fixture has
  `start=26`, `display=25`, `total=125`; assert next_cursor on last doc ==
  `"51"`.
- `TestSearchCursorOmittedAtEnd`: stub returns `start=976, display=25, total=1000`; assert no `next_cursor` on any doc (would be 1001 > 1000).
- `TestSearchCursorOmittedAtTotalEnd`: `start=51, display=25, total=60`,
  10 items returned; assert no `next_cursor` (would be 61 > 60).
- `TestSearchInvalidCursorRejected`: table over `["abc","0","-1","1.5","1001","9999"]` → ErrInvalidCursor + zero HTTP requests for each.

### REQ-ADP8-006 — Per-Vertical Field Mapping

- `TestParseResponseBlogFieldMapping` table-drives all 25 items in the blog
  fixture; for each, asserts every NormalizedDoc field per §6.3 row "Blog"
  (ID synthesized = `"naver:blog:" + sha256(link)[:8]`, SourceID="naver",
  URL=link, Title=stripHTML(title), Body=stripHTML(description), Snippet
  truncated, PublishedAt=parsed postdate (or zero on parse failure),
  RetrievedAt non-zero, Author=bloggername, Score=0.5, Lang="ko",
  DocType=DocTypePost, Metadata required keys present).
- `TestParseResponseNewsFieldMapping`: `URL=originallink` when non-empty;
  fallback to `link` when `originallink==""`. `Author` derived from
  `originallink` host. `PublishedAt=parsed pubDate`. `DocType=DocTypeArticle`.
- `TestParseResponseWebFieldMapping`: `URL=link`, `Author=""`,
  `PublishedAt=zero`, `DocType=DocTypeArticle`.
- `TestParseResponseShopFieldMapping`: `URL=link`, `Author=mallName`,
  `Body=stripHTML(title)` (no description in shop response — title is the
  product name), Metadata includes `lprice`, `hprice`, `mallName`,
  `productId`, `productType`, `image`, `category1`-`category4`, `brand`,
  `maker`. `DocType=DocTypeOther`.
- `TestParseResponseEmpty`: `(nil, "", nil)` on empty `items`.
- `TestParseResponseMalformed`: truncated JSON → `*SourceError{CategoryPermanent}`.
- `TestParseResponseHashEmpty`: every doc `Hash == ""`.
- `TestParseResponseLangIsKo`: every doc `Lang == "ko"`.
- `TestParseResponseScoreIsHalf`: every doc `Score == 0.5`.
- `TestParseResponseSnippetTruncated`: 281+ rune description → snippet ends
  with `"…"` and is exactly 280 runes total.
- `TestParseResponseSnippetFallsBackToTitle`: description="" → snippet
  derived from `stripHTML(title)`.

### REQ-ADP8-007 — HTTP Error Categorisation

- `TestSearchHTTP401AuthError`: 401 → ErrPermanent + HTTPStatus=401 + error message contains `"auth"`.
- `TestSearchHTTP403QuotaExhausted`: 403 → ErrPermanent + HTTPStatus=403 + error message contains `"quota"`.
- `TestSearchHTTP400`, `TestSearchHTTP404`: each → ErrPermanent + matching HTTPStatus.
- `TestSearchHTTP429*` four cases (integer Retry-After, HTTP-date, missing, capped): all assert RetryAfter values per §6.
- `TestSearchHTTP429NoInternalRetry`: server request count == 1.
- `TestSearchHTTP500`, `TestSearchHTTP503`: each → ErrSourceUnavailable + matching HTTPStatus.
- `TestSearchConnectionRefused`: server closed before request → HTTPStatus=0 + ErrSourceUnavailable.
- `TestSearchUnavailablePreservesUnderlyingError`: `errors.Unwrap(srcErr)` non-nil, inner cause text matches.

### REQ-ADP8-008 — Sort Selection

- `TestSearchSortDate`: Filters=[{sort, date}] → URL has `sort=date`.
- `TestSearchSortSim`, `TestSearchSortAbsentDefaultsSim`,
  `TestSearchSortUnknownDefaultsSim`: all assert URL has `sort=sim`, no
  error returned.

### REQ-ADP8-009 — HTML Strip and Entity Decode

- `TestStripHTMLTable` table-drives 8+ inputs covering empty, plain, single
  tag, nested tags, malformed, all five canonical entities, mixed,
  Korean-text-with-`<b>`, and `&nbsp;` collapse.
- `TestParseResponseStripsHTMLEntities` integration test against
  `search_response_blog_html_entities.json`: assert returned doc's Title
  has no `<b>` substring AND has decoded entities (`&amp;` → `&`,
  `&quot;` → `"`).

### REQ-ADP8-010 — DataLab Opt-In

- `TestSearchDataLabHappyPath`: `Query.Text` = JSON of 3 keyword groups +
  `startDate=2026-04-01, endDate=2026-05-01, timeUnit=date`; httptest stub
  returns 3 trend rows; assert 3 NormalizedDocs returned with
  `Metadata["naver_vertical"]=="datalab"` and `URL` matching the
  hash-derived synthetic URL pattern.
- `TestSearchDataLabMalformedQueryText`: q.Text=`"not JSON"` →
  ErrPermanent + zero HTTP requests.
- `TestSearchDataLabUpstream400`: stub returns HTTP 400 → ErrPermanent.
- `TestSearchDataLabPostsToCorrectEndpoint`: assert request URL matches
  `/v1/datalab/search`, `r.Method == http.MethodPost`, body is the
  marshalled JSON.

### REQ-ADP8-011 — Empty/Whitespace Query Rejection

- `TestSearchEmptyQueryRejectedNoHTTP` table-drives `Text` over
  `["", "   ", "\t\n  \r"]`; for each: `errors.Is(err, types.ErrPermanent)`
  AND `errors.Is(err, ErrInvalidQuery)` AND zero HTTP requests at the stub.
- Same coverage extended for the datalab vertical (empty `Query.Text`
  rejected before JSON parse).

### REQ-ADP8-012 — Concurrent Search Safety (State-Driven)

- `TestSearchConcurrentSafe`: a single `*Adapter` is constructed pointing
  at one `httptest.Server` (which records every inbound request). 50
  goroutines launched, each calling `(*Adapter).Search(ctx, q)` exactly
  once with identical queries. All goroutines start via a `sync.WaitGroup`
  barrier so the invocations overlap.
- Assertions:
  1. The test executes successfully under `go test -race`; race-detector
     reports zero data-race alarms attributable to the naver adapter
     package.
  2. The stub server's request counter equals 50.
  3. Every goroutine receives `(docs, nil)` with `len(docs) == 25` and
     each returned doc passes `Validate()`.
  4. The captured `X-Naver-Client-Id` and `X-Naver-Client-Secret` headers
     are present and identical across all 50 requests.

### REQ-ADP8-013 — Redirect Allowlist

- `TestSearchFollowsAllowlistRedirect`: 302 within allowlist followed.
- `TestSearchRejectsCrossDomainRedirect`: 302 to `attacker.com` →
  ErrPermanent + message contains `"cross-domain redirect"`.
- `TestSearchRejectsRedirectChainOver3`: 4-hop chain → "too many redirects".

### REQ-ADP8-014 — Sole-Emitter Discipline

- `TestNoNewMetricFamilies`: snapshot the Prometheus registry's `Gather()`
  output before constructing the Adapter; snapshot after one Search;
  assert the family-count delta is zero (no new metric families registered
  by ADP-008).
- `TestNoSpansFromAdapter`: in-memory OTel SpanRecorder; assert exactly one
  span `adapter.search` exists with `attribute.adapter.name == "naver"`,
  emitted by the wrappedAdapter (NOT by the naver package itself).
- `TestNoLogsFromAdapter`: capture slog handler; the only "adapter call"
  record is from the wrappedAdapter.
- `TestMetadataNaverVerticalPresent`: every NormalizedDoc returned has
  `Metadata["naver_vertical"]` populated with one of the five vertical
  strings.

### NFR-ADP8-001 — Parse-Path Performance

- `BenchmarkParseBlogResponse25Items` is invoked as
  `go test -bench=BenchmarkParseBlogResponse25Items -benchtime=10x -count=5 ./internal/adapters/naver/...` on amd64.
- Median of 5 reported per-op mean wall-clock durations SHALL be ≤ 5 ms.
- `allocs/op ≤ 500`.

### NFR-ADP8-002 — E2E p95 (Stub)

- `TestSearchE2ELatencyStubP95` runs 100 invocations against the stub
  `httptest.Server`, sorts elapsed durations, asserts `durations[94] ≤ 200ms`.

### NFR-ADP8-003 — Race-Clean Concurrent Workload

- `TestSearchConcurrentSafe` (REQ-ADP8-012 acceptance) executes under
  `go test -race`; race-detector alarms attributable to the naver package = 0.

### NFR-ADP8-004 — Zero Goroutine Leaks

- `TestMain` in `bench_test.go`:
  ```
  func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }
  ```
- `TestSearchNoGoroutineLeakOnCancel`: `goleak.VerifyNone(t)` after a
  `Search` call whose ctx was cancelled at 50ms while the stub server
  delays response by 200ms.

---

## 6. Technical Approach

### 6.1 Files to Modify

**Created (15 source files + 11 testdata)**:

- `internal/adapters/naver/naver.go` — Adapter struct, New, Name,
  Capabilities, Healthcheck, compile-time interface assertion, env
  resolution
- `internal/adapters/naver/naver_test.go` — interface conformance + env
  validation tests
- `internal/adapters/naver/search.go` — Search hot path, vertical dispatch,
  URL construction, sort + pagination param resolution
- `internal/adapters/naver/search_test.go` — main test file (largest)
- `internal/adapters/naver/client.go` — HTTP client construction,
  doRequest with auth headers, categorizeStatus, redirect allowlist
- `internal/adapters/naver/client_test.go` — error mapping + redirect tests
- `internal/adapters/naver/parse.go` — parseResponse envelope dispatcher +
  4 per-vertical sub-parsers (`parseBlogItem`, `parseNewsItem`,
  `parseWebItem`, `parseShopItem`)
- `internal/adapters/naver/parse_test.go` — field mapping tests per vertical
- `internal/adapters/naver/strip.go` — stripHTML helper
- `internal/adapters/naver/strip_test.go` — strip table tests
- `internal/adapters/naver/score.go` — defaultScore (returns 0.5)
- `internal/adapters/naver/score_test.go` — score tests
- `internal/adapters/naver/errors.go` — sentinels +
  parseRetryAfter helper
- `internal/adapters/naver/datalab.go` — DataLab POST builder + parser
- `internal/adapters/naver/datalab_test.go` — DataLab tests
- `internal/adapters/naver/bench_test.go` — `BenchmarkParseBlogResponse25Items`
  + `TestMain` with `goleak.VerifyTestMain`
- `internal/adapters/naver/testdata/search_response_blog.json` (~6KB)
- `internal/adapters/naver/testdata/search_response_news.json` (~6KB)
- `internal/adapters/naver/testdata/search_response_web.json` (~5KB)
- `internal/adapters/naver/testdata/search_response_shop.json` (~8KB)
- `internal/adapters/naver/testdata/search_response_blog_empty.json` (~150B)
- `internal/adapters/naver/testdata/search_response_blog_pagination.json` (~6KB)
- `internal/adapters/naver/testdata/search_response_blog_html_entities.json` (~2KB)
- `internal/adapters/naver/testdata/search_response_news_no_originallink.json` (~600B)
- `internal/adapters/naver/testdata/search_response_blog_malformed_postdate.json` (~600B)
- `internal/adapters/naver/testdata/search_response_blog_malformed.json` (~250B)
- `internal/adapters/naver/testdata/datalab_response.json` (~3KB)

**Modified**: none. The adapter self-contains. `pkg/types` already
publishes the contract; `internal/adapters/registry.go` already accepts
any `types.Adapter`; `internal/obs/metrics/metrics.go` already declares
the `AdapterCalls` and `AdapterCallDuration` collectors with `adapter`
and `outcome` in the cardinality allowlist (the `adapter="naver"` value
fits within the V1 14-adapter ceiling). Registration into the production
registry is owned by SPEC-CLI-001.

**Unchanged (by design)**:
- `pkg/types/*` — no contract change required.
- `internal/adapters/registry.go` — wrappedAdapter sole-emitter pattern
  preserved.
- `internal/obs/metrics/metrics.go` — no new metric family.
- `cmd/usearch/main.go` — registry construction owned by SPEC-CLI-001.
- `.moai/config/sections/*.yaml` — no config delta.

### 6.2 Package Layout

```
internal/adapters/naver/
├── naver.go                                # Adapter, New, Name, Capabilities, Healthcheck, interface assertion
├── naver_test.go                           # Interface conformance + Capabilities determinism + env validation
├── search.go                               # (*Adapter).Search hot path
├── search_test.go                          # Per-vertical happy + error categorisation tests
├── client.go                               # *http.Client, doRequest, categorizeStatus, redirect allowlist
├── client_test.go                          # categorizeStatus table + redirect allowlist + headers
├── parse.go                                # parseResponse envelope dispatcher + 4 per-vertical parsers
├── parse_test.go                           # Per-vertical field mapping tests
├── strip.go                                # stripHTML helper
├── strip_test.go                           # Tag-strip + entity-decode table
├── score.go                                # defaultScore (returns 0.5)
├── score_test.go
├── errors.go                               # ErrInvalidQuery / ErrInvalidVertical / ErrInvalidCursor / ErrAuthMissing + parseRetryAfter
├── datalab.go                              # DataLab POST builder + parser
├── datalab_test.go
├── bench_test.go                           # BenchmarkParseBlogResponse25Items + TestMain (goleak)
└── testdata/
    ├── search_response_blog.json           # 25-item blog happy path
    ├── search_response_news.json           # 25-item news (mix of populated + empty originallink)
    ├── search_response_web.json            # 25-item bare title/link/description
    ├── search_response_shop.json           # 25-item full shopping fields
    ├── search_response_blog_empty.json     # Zero items
    ├── search_response_blog_pagination.json # start=1, total=125 → next_cursor=26
    ├── search_response_blog_html_entities.json # <b> + 5 entities + nested
    ├── search_response_news_no_originallink.json # Single news with originallink=""
    ├── search_response_blog_malformed_postdate.json # postdate="" and postdate="abc"
    ├── search_response_blog_malformed.json # Truncated JSON
    └── datalab_response.json               # 3 keyword groups × 30 daily ratios
```

[NOTE on duplication vs sharing] `parseRetryAfter`, `categorizeStatus`,
the redirect-allowlist pattern, and `stripHTML` duplicate equivalents in
`internal/adapters/reddit/` (parseRetryAfter, categorizeStatus,
allowlist) and `internal/adapters/hn/` (stripHTML, parseRetryAfter,
categorizeStatus, allowlist). After ADP-008 lands, three duplicate copies
exist — the rule of three is now satisfied. SPEC-ADP-REFAC-001 (post-M3,
after the seven M3 ADP-* SPECs land) MAY consolidate. v0.1 ships
duplicates. See Open Question §11.3.

### 6.3 Naver Item → NormalizedDoc Field Mapping (per Vertical)

#### 6.3.1 Common fields (all four search verticals)

| NormalizedDoc field | Value | Notes |
|---------------------|-------|-------|
| `ID` | `"naver:" + vertical + ":" + sha256(link)[:8]` | Synthesized; Naver does NOT supply per-item ID |
| `SourceID` | `"naver"` | Matches `Name()` |
| `Lang` | `"ko"` | Static for the Naver Open API |
| `Score` | `0.5` | Per §2.4 — Naver supplies no engagement metric |
| `RetrievedAt` | `time.Now().UTC()` | Set by `parseResponse` caller |
| `Citations` | `nil` | Adapter returns posts/articles, not citation graphs |
| `Hash` | `""` | Consumer computes via `CanonicalHash()` |
| `Metadata["naver_vertical"]` | resolved vertical string | REQUIRED on every doc |

`Title`, `Body`, `Snippet` are vertical-specific (see below); all are
`stripHTML`-processed per REQ-ADP8-009. `Snippet` is truncated to 280
runes with "…" suffix when longer; falls back to truncated `Title` when
the description-equivalent field is empty.

#### 6.3.2 Blog vertical

| Naver JSON field | NormalizedDoc field | Transform |
|------------------|---------------------|-----------|
| `link` | `URL` | Use as-is |
| `title` | `Title` | `stripHTML(title)` |
| `description` | `Body` | `stripHTML(description)` |
| `description` (truncated 280r) | `Snippet` | `stripHTML(description)`, truncated; fallback to title |
| `postdate` | `PublishedAt` | `time.Parse("20060102", postdate).UTC()`; zero on parse failure |
| `bloggername` | `Author` | Use as-is |
| (constant) | `DocType` | `types.DocTypePost` |
| `bloggername`, `bloggerlink`, `postdate` | `Metadata` | REQUIRED keys: `naver_vertical="blog"`, `bloggername`, `bloggerlink`, `postdate` (raw `YYYYMMDD` string) |

#### 6.3.3 News vertical

| Naver JSON field | NormalizedDoc field | Transform |
|------------------|---------------------|-----------|
| `originallink` (or `link` fallback) | `URL` | Prefer `originallink` when non-empty; fall back to `link` |
| `title` | `Title` | `stripHTML(title)` |
| `description` | `Body` | `stripHTML(description)` |
| `description` (truncated 280r) | `Snippet` | Same as blog |
| `pubDate` | `PublishedAt` | `time.Parse(time.RFC1123Z, pubDate)`; fallback `time.RFC1123`; zero on failure |
| derived from `originallink` host | `Author` | Parse via `url.Parse(originallink).Host`; empty when `originallink==""` |
| (constant) | `DocType` | `types.DocTypeArticle` |
| `originallink`, `link` | `Metadata` | REQUIRED keys: `naver_vertical="news"`, `originallink`, `naver_link` (= `link`) |

#### 6.3.4 Web (webkr) vertical

| Naver JSON field | NormalizedDoc field | Transform |
|------------------|---------------------|-----------|
| `link` | `URL` | Use as-is |
| `title` | `Title` | `stripHTML(title)` |
| `description` | `Body` | `stripHTML(description)` |
| `description` (truncated 280r) | `Snippet` | Same as blog |
| (constant) | `PublishedAt` | `time.Time{}` (zero — Naver web search has no pubdate) |
| (constant) | `Author` | `""` |
| (constant) | `DocType` | `types.DocTypeArticle` |
| (none) | `Metadata` | REQUIRED keys: `naver_vertical="web"` only |

#### 6.3.5 Shopping vertical

| Naver JSON field | NormalizedDoc field | Transform |
|------------------|---------------------|-----------|
| `link` | `URL` | Use as-is |
| `title` | `Title` | `stripHTML(title)` (product name) |
| `title` | `Body` | Same as Title (no description in shop response) |
| `title` (truncated 280r) | `Snippet` | Same as Title truncated |
| (constant) | `PublishedAt` | `time.Time{}` (zero — no item-level date) |
| `mallName` | `Author` | Seller/mall name |
| (constant) | `DocType` | `types.DocTypeOther` (no `DocTypeProduct` enum yet — Open Question §11.6) |
| all shopping fields | `Metadata` | REQUIRED keys: `naver_vertical="shop"`, `mallName`, `productId`, `productType`, `lprice` (raw string), `hprice` (raw string), `image`, `category1`, `category2`, `category3`, `category4`, `brand`, `maker` |

#### 6.3.6 DataLab synthetic

| Source | NormalizedDoc field | Transform |
|--------|---------------------|-----------|
| (synthesized) | `ID` | `"naver:datalab:" + sha256(reqBody+groupName)[:8]` |
| (constant) | `SourceID` | `"naver"` |
| (synthesized) | `URL` | `"https://datalab.naver.com/keyword/trendResult.naver?hashKey=" + sha256(reqBody)[:16]` (deterministic but synthetic; consumers use it as a unique key only) |
| `groupName` | `Title` | `"DataLab trend: " + groupName` |
| `data` (top 3 dates) | `Body` | Human-readable summary lines, e.g., `"2026-04-15: ratio 87.3, 2026-04-16: ratio 83.1, 2026-04-17: ratio 79.5"` |
| `Body` (truncated 280r) | `Snippet` | Same |
| `endDate` | `PublishedAt` | `time.Parse("2006-01-02", endDate)` |
| (constant) | `Author` | `"Naver DataLab"` |
| (constant) | `Score` | `0.5` |
| (constant) | `Lang` | `"ko"` |
| (constant) | `DocType` | `types.DocTypeOther` |
| serialised time-series | `Metadata` | REQUIRED keys: `naver_vertical="datalab"`, `datalab_keyword_groups`, `datalab_time_unit`, `datalab_start_date`, `datalab_end_date`, `datalab_data` (the time-series array as a JSON string) |

### 6.4 Type Sketches (illustrative; final shapes in run phase)

```go
// internal/adapters/naver/naver.go
package naver

import (
    "context"
    "fmt"
    "net"
    "net/http"
    "os"

    "github.com/elymas/universal-search/pkg/types"
)

const (
    defaultSearchBaseURL  = "https://openapi.naver.com/v1/search"
    defaultDataLabBaseURL = "https://openapi.naver.com/v1/datalab"
    defaultUserAgentTemplate = "usearch/%s (+https://github.com/elymas/universal-search)"
    defaultUAVersion         = "v0.1"
    defaultHealthcheckTarget = "openapi.naver.com:443"

    envClientID     = "NAVER_CLIENT_ID"
    envClientSecret = "NAVER_CLIENT_SECRET"
)

type Options struct {
    SearchBaseURL    string        // default: defaultSearchBaseURL
    DataLabBaseURL   string        // default: defaultDataLabBaseURL
    HTTPClient       *http.Client  // default: 10s timeout, 1-host allowlist, reqid transport
    UserAgentVersion string
    HealthcheckTarget string
    ClientID         string        // optional: overrides env NAVER_CLIENT_ID
    ClientSecret     string        // optional: overrides env NAVER_CLIENT_SECRET
}

type Adapter struct {
    httpClient        *http.Client
    searchBaseURL     string
    datalabBaseURL    string
    userAgent         string
    clientID          string
    clientSecret      string
    healthcheckTarget string
}

func New(opts Options) (*Adapter, error) {
    cid := opts.ClientID
    if cid == "" { cid = os.Getenv(envClientID) }
    csec := opts.ClientSecret
    if csec == "" { csec = os.Getenv(envClientSecret) }
    if cid == "" || csec == "" {
        return nil, ErrAuthMissing
    }
    // ... (defaults, build *Adapter)
}

func (a *Adapter) Name() string { return "naver" }

func (a *Adapter) Capabilities() types.Capabilities {
    return types.Capabilities{
        SourceID:          "naver",
        DisplayName:       "Naver",
        DocTypes:          []types.DocType{types.DocTypeArticle, types.DocTypePost},
        SupportedLangs:    []string{"ko"},
        SupportsSince:     false,
        RequiresAuth:      true,
        AuthEnvVars:       []string{envClientID, envClientSecret},
        RateLimitPerMin:   17, // 25000/day / (24*60) ≈ 17/min
        DefaultMaxResults: 25,
        Notes: "Naver Open API (https://openapi.naver.com/v1/search/...). " +
            "verticals: blog (default), news, web, shop, datalab — " +
            "select via Query.Filters[Key=naver_vertical]. " +
            "Korean-locale primary; Lang=ko forced on all returned docs. " +
            "25000/day per app (shared across search + datalab). " +
            "requires NAVER_CLIENT_ID + NAVER_CLIENT_SECRET env. " +
            "sort defaults to sim (relevance); set Filters[Key=sort,Value=date] for reverse-chronological.",
    }
}

func (a *Adapter) Healthcheck(ctx context.Context) error {
    var d net.Dialer
    conn, err := d.DialContext(ctx, "tcp", a.healthcheckTarget)
    if err != nil { return err }
    return conn.Close()
}

var _ types.Adapter = (*Adapter)(nil)
```

```go
// internal/adapters/naver/client.go (excerpt)
var allowedRedirectHosts = map[string]struct{}{
    "openapi.naver.com": {},
}

func redirectAllowlist(req *http.Request, via []*http.Request) error {
    if len(via) >= 3 {
        return errors.New("naver: too many redirects (max 3)")
    }
    if _, ok := allowedRedirectHosts[req.URL.Hostname()]; !ok {
        return fmt.Errorf("naver: cross-domain redirect rejected: %s", req.URL.Hostname())
    }
    return nil
}

func (a *Adapter) doRequest(req *http.Request) (*http.Response, error) {
    req.Header.Set("User-Agent", a.userAgent)
    req.Header.Set("Accept", "application/json")
    req.Header.Set("X-Naver-Client-Id", a.clientID)
    req.Header.Set("X-Naver-Client-Secret", a.clientSecret)
    return a.httpClient.Do(req)
}

func categorizeStatus(status int, retryAfter time.Duration, cause error) *types.SourceError {
    se := &types.SourceError{Adapter: "naver", HTTPStatus: status, Cause: cause}
    switch {
    case status == 429:
        se.Category = types.CategoryRateLimited
        se.RetryAfter = retryAfter
    case status >= 400 && status < 500:
        se.Category = types.CategoryPermanent
    case status >= 500 && status < 600:
        se.Category = types.CategoryUnavailable
    case status == 0:
        se.Category = types.CategoryUnavailable
    default:
        se.Category = types.CategoryUnknown
    }
    return se
}
```

### 6.5 HTTP Client Construction Notes

Identical pattern to ADP-001 §6.5:

- **Timeout**: 10 seconds total request deadline (default). Caller's ctx
  deadline takes precedence.
- **Redirect policy**: `CheckRedirect` enforces the 1-host allowlist
  `{openapi.naver.com}` and caps at 3 hops.
- **Transport**: `reqid.NewTransport(http.DefaultTransport)` for
  request-ID propagation.
- **Headers per request**: four headers (`User-Agent`, `Accept: application/json`,
  `X-Naver-Client-Id`, `X-Naver-Client-Secret`). All four are mandatory per
  REQ-ADP8-003.

### 6.6 Observability Note

The Naver adapter, like Reddit and HN adapters, emits ZERO metrics, logs,
and spans of its own. ALL observability comes from the registry's
`wrappedAdapter` (`internal/adapters/registry.go:172-263`). This is the
sole-emitter discipline established in SPEC-CORE-001 §6.5. The adapter's
responsibility is to return a correctly-categorised `*types.SourceError`
so the wrappedAdapter computes the right `outcome` label via
`types.OutcomeFromError(err)`. The selected vertical lives in
`Metadata["naver_vertical"]` for downstream visibility but does NOT
escape to Prometheus — the `adapter="naver"` label remains a single
bounded value per SPEC-OBS-001 cardinality discipline.

### 6.7 MX Tag Plan

| File::Symbol | Tag | Reason |
|--------------|-----|--------|
| `naver.go::(*Adapter).Search` (defined in `search.go`) | `@MX:ANCHOR` | Sole entry point for all Naver fanout calls. fan_in ≥ 3 (registry wrappedAdapter, FAN-001 fanout, tests). `@MX:REASON: contract boundary; signature change ripples to FAN-001 + CLI-001 + SYN-001`. `@MX:SPEC: SPEC-ADP-008`. |
| `parse.go::parseResponse` | `@MX:ANCHOR` | Every Naver doc passes through this single transform dispatcher. fan_in = 1 (Search) but invariant-bearing — bug here corrupts every NormalizedDoc returned across all four verticals. `@MX:REASON: NormalizedDoc field-mapping integrity gate per-vertical`. |
| `client.go::doRequest` | `@MX:WARN` | Outbound network call. Redirect allowlist enforces SSRF safety boundary; auth headers (X-Naver-Client-Id / X-Naver-Client-Secret) are load-bearing for every request. `@MX:REASON: removing the CheckRedirect guard re-opens SSRF; missing auth headers cause silent 401 cascade`. |
| `client.go::categorizeStatus` | `@MX:NOTE` | The HTTP-status-to-Category rosetta with auth-specific 401/403 messages. Future contributors will look here first when adapting Naver-specific error semantics (e.g., quota messaging). |
| `client.go::allowedRedirectHosts` map | `@MX:NOTE` | The 1-entry redirect allowlist. Adding a host requires a security review (Naver's CDN architecture would change this). |
| `strip.go::stripHTML` | `@MX:NOTE` | Conservative stdlib-only HTML-strip + entity-decode. Mirrors `internal/adapters/hn/strip.go` shape. NOT a security boundary. Adding a sixth canonical entity or a seventh tag pattern requires updating the test fixture set first. |
| `naver.go::(*Adapter).Capabilities` | `@MX:NOTE` | The `Capabilities.Notes` text is consumer-facing; operator-readable. Changes propagate to operator dashboards (SPEC-EVAL-002 future). |

All tags `[AUTO]`-prefixed, include `@MX:SPEC: SPEC-ADP-008`, follow
`code_comments: en` per `.moai/config/sections/language.yaml`. Per-file
hard limit (3 ANCHOR + 5 WARN per `.moai/config/sections/mx.yaml`):
respected (max 1 ANCHOR + 1 WARN per file in this plan).

### 6.8 Harness Level

Per `.moai/config/sections/harness.yaml` auto-routing: 14 EARS REQs
(12 × P0 + 2 × P1) + 4 NFRs touching 1 package
(`internal/adapters/naver/`, ~16 source/test files including DataLab path)
+ zero cross-package edits + zero security/payment/PII keywords (auth
headers are env-driven; not classified as a security keyword by the
auto-detection rules) + zero compose/env/config deltas = **standard**
harness level. Sprint Contract is OPTIONAL but recommended. Evaluator
profile `default` applies.

---

## 7. What NOT to Build (Exclusions)

[HARD] This SPEC explicitly excludes the following items. Each has a known
destination SPEC; this list prevents scope creep into ADP-008 and reaffirms
the multi-vertical reference shape.

- **MCP-server subprocess wrapping** of `isnow890/naver-search-mcp` →
  REJECTED per D1. MCP repo is a tool taxonomy reference only.
- **Per-source customisations for non-Naver Korean adapters** (다음/Daum,
  KoreaNewsCrawler, RSS) → SPEC-ADP-009.
- **Per-source customisations for non-Korean adapters** (arXiv, GitHub,
  YouTube, Bluesky, X, SearXNG, Polymarket) → SPEC-ADP-003 through
  SPEC-ADP-007.
- **Retry orchestration** → SPEC-FAN-001 D6. Adapter is one-shot per call.
- **Response caching** → SPEC-CACHE-001 (M3). Adapter is stateless.
- **Result ranking, deduplication, RRF fusion across adapters** →
  SPEC-IDX-001 (M3). Adapter returns Naver-relevance order with
  `Score=0.5`.
- **Korean tokenization** (mecab-ko) → SPEC-IDX-003 (M3). Adapter emits
  plain stripped text in `Body`; tokenization is downstream.
- **Cross-vertical aggregation** (`naver_vertical=union` mixing
  blog+news+web in one Search call) → out of v0.1; future
  SPEC-ADP-008a.
- **`DocTypeProduct` enum addition** for shopping items → out of v0.1;
  defer to coordinated SPEC-CORE-001a if SPEC-IDX-001 needs it.
- **DataLab Shopping Insight tools** (`/v1/datalab/shopping/*` POST
  variants) → out of v0.1; only `/v1/datalab/search` (search trends) is
  exposed via the datalab vertical.
- **Tenant-scoped quota policy** (per-team quota of the 25k/day pool) →
  SPEC-AUTH-002 (M6).
- **Adapter health-state machine** (auto-disable on N consecutive
  failures) → SPEC-EVAL-002 (M8).
- **OAuth-authenticated variant** (Naver Login OAuth) → out of scope.
- **Subreddit-style scoping** (subforum / per-blogger restrict) → out of
  scope; Naver Open API does not surface this.
- **Sort modes beyond `sim`/`date`** → out of v0.1; Naver supports only
  these two.
- **Live network integration tests in CI** → out of v0.1; httptest +
  golden fixtures only.
- **Per-adapter custom Prometheus metrics** (e.g.,
  `naver_vertical_calls_total{vertical}`) → would require amending
  SPEC-OBS-001's allowlist. Out of v0.1.
- **`pkg/llm` integration** — adapter does NOT call LLMs.
- **Streaming Search results** → SPEC-SYN-004 (M4).
- **Cross-adapter helper extraction** — defer to SPEC-ADP-REFAC-001
  post-M3.
- **Image search vertical** (`/v1/search/image.json`) → out of v0.1.
  Image search returns thumbnails, not text — synthesis quality is
  poor without OCR.
- **KnowledgeiN, Encyclopedia, Cafe Article verticals** (the upstream
  MCP exposes them) → out of v0.1; the four verticals already in scope
  cover the M3 exit criterion (Korean ranked first).

---

## 8. TDD Plan

Development follows RED-GREEN-REFACTOR per
`quality.development_mode: tdd` (`.moai/config/sections/quality.yaml:17`).
Representative RED-phase tests, written before implementation, grouped by
REQ. Total: ~58 tests covering REQ-ADP8-001..014 + NFRs. Coverage target:
85% per `quality.test_coverage_target` (`.moai/config/sections/quality.yaml:76`).
Benchmarks do not count toward coverage.

| # | Test | Layer | REQ | Assertion |
|---|------|-------|-----|-----------|
| 1 | `TestAdapterName` | `naver_test.go` | REQ-ADP8-001 | `(*Adapter).Name() == "naver"` |
| 2 | `TestAdapterImplementsInterface` | `naver_test.go` | REQ-ADP8-001 | Compile-time `var _ types.Adapter = (*Adapter)(nil)` succeeds |
| 3 | `TestCapabilitiesDeterministic` | `naver_test.go` | REQ-ADP8-001 | Two consecutive `Capabilities()` calls return `reflect.DeepEqual` results |
| 4 | `TestCapabilitiesShape` | `naver_test.go` | REQ-ADP8-001 | All 9 documented field values + 5 Notes substring matches |
| 5 | `TestHealthcheckSucceeds` | `naver_test.go` | REQ-ADP8-001 | TCP dial against test loopback succeeds |
| 6 | `TestNewRequiresClientIDEnv` | `naver_test.go` | REQ-ADP8-003 | unset env, default Options → ErrAuthMissing |
| 7 | `TestNewRequiresClientSecretEnv` | `naver_test.go` | REQ-ADP8-003 | ID set, secret unset → ErrAuthMissing |
| 8 | `TestNewAcceptsExplicitOptions` | `naver_test.go` | REQ-ADP8-003 | Options.ClientID + Options.ClientSecret set → success |
| 9 | `TestNewAcceptsEnvFallback` | `naver_test.go` | REQ-ADP8-003 | Env set, Options empty → success |
| 10 | `TestSearchHappyPathBlog25Items` | `search_test.go` | REQ-ADP8-002, REQ-ADP8-006 | 25 NormalizedDocs, `Validate()` returns nil |
| 11 | `TestSearchHappyPathNews25Items` | `search_test.go` | REQ-ADP8-002, REQ-ADP8-006 | Same for news |
| 12 | `TestSearchHappyPathWeb25Items` | `search_test.go` | REQ-ADP8-002, REQ-ADP8-006 | Same for web |
| 13 | `TestSearchHappyPathShop25Items` | `search_test.go` | REQ-ADP8-002, REQ-ADP8-006 | Same for shop |
| 14 | `TestSearchURLParametersIncludeAllRequired` | `search_test.go` | REQ-ADP8-002 | Captured URL has `query`, `display`, `start`, `sort` |
| 15 | `TestSearchClampsDisplayTo100` | `search_test.go` | REQ-ADP8-002 | q.MaxResults=500 → URL has `display=100` |
| 16 | `TestSearchDefaultsDisplayTo25` | `search_test.go` | REQ-ADP8-002 | q.MaxResults=0 → URL has `display=25` |
| 17 | `TestSearchOmitsStartWhenCursorEmpty` | `search_test.go` | REQ-ADP8-002 | q.Cursor="" → URL has `start=1` |
| 18 | `TestSearchSetsStartWhenCursorPresent` | `search_test.go` | REQ-ADP8-002 | q.Cursor="26" → URL contains `start=26` |
| 19 | `TestSearchSendsAuthHeaders` | `client_test.go` | REQ-ADP8-003 | X-Naver-Client-Id + X-Naver-Client-Secret captured |
| 20 | `TestSearchAlsoSendsUAAndAccept` | `client_test.go` | REQ-ADP8-003 | UA + Accept headers captured |
| 21 | `TestSearchNeverLogsSecret` | `client_test.go` | REQ-ADP8-003 | slog records do not contain secret string |
| 22 | `TestNotesDoesNotContainSecret` | `naver_test.go` | REQ-ADP8-003 | `Capabilities.Notes` contains no secret value |
| 23 | `TestSearchVerticalBlog` | `search_test.go` | REQ-ADP8-004 | Filters[naver_vertical=blog] → URL path = `/v1/search/blog.json` |
| 24 | `TestSearchVerticalNews` | `search_test.go` | REQ-ADP8-004 | Same for news |
| 25 | `TestSearchVerticalWeb` | `search_test.go` | REQ-ADP8-004 | Same for web |
| 26 | `TestSearchVerticalShop` | `search_test.go` | REQ-ADP8-004 | Same for shop |
| 27 | `TestSearchVerticalDataLab` | `datalab_test.go` | REQ-ADP8-004, REQ-ADP8-010 | Filter datalab → POST to `/v1/datalab/search` |
| 28 | `TestSearchVerticalDefaultsToBlog` | `search_test.go` | REQ-ADP8-004 | Filters=nil → blog endpoint |
| 29 | `TestSearchVerticalEmptyValueDefaultsToBlog` | `search_test.go` | REQ-ADP8-004 | Filters[naver_vertical=""] → blog |
| 30 | `TestSearchVerticalInvalidRejected` | `search_test.go` | REQ-ADP8-004 | Filters[naver_vertical=Image] → ErrPermanent + ErrInvalidVertical + zero requests |
| 31 | `TestSearchCursorRoundTrip` | `search_test.go` | REQ-ADP8-005 | Cursor="26", total=125 → next_cursor="51" |
| 32 | `TestSearchCursorOmittedAtEnd` | `search_test.go` | REQ-ADP8-005 | start=976, total=1000 → no next_cursor |
| 33 | `TestSearchCursorOmittedAtTotalEnd` | `search_test.go` | REQ-ADP8-005 | start=51, total=60 → no next_cursor |
| 34 | `TestSearchInvalidCursorRejected` | `search_test.go` | REQ-ADP8-005 | Table over 6 invalid cursors → ErrInvalidCursor + zero requests |
| 35 | `TestParseResponseBlogFieldMapping` | `parse_test.go` | REQ-ADP8-006 | All 25 items mapped per §6.3 row "Blog" |
| 36 | `TestParseResponseNewsFieldMapping` | `parse_test.go` | REQ-ADP8-006 | originallink fallback to link tested |
| 37 | `TestParseResponseWebFieldMapping` | `parse_test.go` | REQ-ADP8-006 | PublishedAt zero, Author empty |
| 38 | `TestParseResponseShopFieldMapping` | `parse_test.go` | REQ-ADP8-006 | All shopping Metadata keys present |
| 39 | `TestParseResponseEmpty` | `parse_test.go` | REQ-ADP8-006 | Empty `items` → (nil, "", nil) |
| 40 | `TestParseResponseMalformed` | `parse_test.go` | REQ-ADP8-006 | Truncated JSON → ErrPermanent |
| 41 | `TestParseResponseHashEmpty` | `parse_test.go` | REQ-ADP8-006 | Every doc Hash == "" |
| 42 | `TestParseResponseLangIsKo` | `parse_test.go` | REQ-ADP8-006 | Every doc Lang == "ko" |
| 43 | `TestParseResponseScoreIsHalf` | `parse_test.go` | REQ-ADP8-006 | Every doc Score == 0.5 |
| 44 | `TestParseResponseSnippetTruncated` | `parse_test.go` | REQ-ADP8-006 | 281+ rune description → snippet 280 runes + "…" |
| 45 | `TestParseResponseSnippetFallsBackToTitle` | `parse_test.go` | REQ-ADP8-006 | description="" → snippet from title |
| 46 | `TestSearchHTTP401AuthError` | `search_test.go` | REQ-ADP8-007 | 401 → ErrPermanent + message contains "auth" |
| 47 | `TestSearchHTTP403QuotaExhausted` | `search_test.go` | REQ-ADP8-007 | 403 → ErrPermanent + message contains "quota" |
| 48 | `TestSearchHTTP400` | `search_test.go` | REQ-ADP8-007 | 400 → ErrPermanent |
| 49 | `TestSearchHTTP404` | `search_test.go` | REQ-ADP8-007 | 404 → ErrPermanent |
| 50 | `TestSearchHTTP429*` (4 cases) | `search_test.go` | REQ-ADP8-007 | RetryAfter parse + cap + default |
| 51 | `TestSearchHTTP429NoInternalRetry` | `search_test.go` | REQ-ADP8-007 | request count == 1 |
| 52 | `TestSearchHTTP500` / `TestSearchHTTP503` | `search_test.go` | REQ-ADP8-007 | ErrSourceUnavailable + matching HTTPStatus |
| 53 | `TestSearchConnectionRefused` | `search_test.go` | REQ-ADP8-007 | server closed → HTTPStatus=0 + Unavailable |
| 54 | `TestSearchUnavailablePreservesUnderlyingError` | `search_test.go` | REQ-ADP8-007 | Unwrap chain |
| 55 | `TestSearchSortDate` / `TestSearchSortSim` / `TestSearchSortAbsentDefaultsSim` / `TestSearchSortUnknownDefaultsSim` | `search_test.go` | REQ-ADP8-008 | Sort filter resolution |
| 56 | `TestStripHTMLTable` | `strip_test.go` | REQ-ADP8-009 | 8+ inputs covering all canonical entities + tags |
| 57 | `TestParseResponseStripsHTMLEntities` | `parse_test.go` | REQ-ADP8-009 | Integration against html_entities fixture |
| 58 | `TestSearchDataLabHappyPath` | `datalab_test.go` | REQ-ADP8-010 | 3 keyword groups → 3 docs returned |
| 59 | `TestSearchDataLabMalformedQueryText` | `datalab_test.go` | REQ-ADP8-010 | q.Text not JSON → ErrPermanent + zero requests |
| 60 | `TestSearchDataLabUpstream400` | `datalab_test.go` | REQ-ADP8-010 | server 400 → ErrPermanent |
| 61 | `TestSearchDataLabPostsToCorrectEndpoint` | `datalab_test.go` | REQ-ADP8-010 | URL match + method=POST |
| 62 | `TestSearchEmptyQueryRejectedNoHTTP` | `search_test.go` | REQ-ADP8-011 | Table over 3 empty/whitespace inputs → ErrInvalidQuery + zero requests; same coverage extended for datalab vertical |
| 63 | `TestSearchConcurrentSafe` | `search_test.go` | REQ-ADP8-012, NFR-ADP8-003 | 50 goroutines × Search; race-clean; identical headers across all |
| 64 | `TestSearchFollowsAllowlistRedirect` | `client_test.go` | REQ-ADP8-013 | 302 within allowlist followed |
| 65 | `TestSearchRejectsCrossDomainRedirect` | `client_test.go` | REQ-ADP8-013 | 302 to attacker.com → ErrPermanent + "cross-domain redirect" |
| 66 | `TestSearchRejectsRedirectChainOver3` | `client_test.go` | REQ-ADP8-013 | 4-hop chain → "too many redirects" |
| 67 | `TestNoNewMetricFamilies` | `naver_test.go` | REQ-ADP8-014 | Gather() before+after delta == 0 |
| 68 | `TestNoSpansFromAdapter` | `naver_test.go` | REQ-ADP8-014 | adapter.search span only from wrappedAdapter |
| 69 | `TestNoLogsFromAdapter` | `naver_test.go` | REQ-ADP8-014 | "adapter call" log only from wrappedAdapter |
| 70 | `TestMetadataNaverVerticalPresent` | `parse_test.go` | REQ-ADP8-014 | Every doc has Metadata["naver_vertical"] |
| 71 | `TestSearchE2ELatencyStubP95` | `search_test.go` | NFR-ADP8-002 | 100 invocations; p95 ≤ 200ms |
| 72 | `TestSearchNoGoroutineLeakOnCancel` | `search_test.go` | NFR-ADP8-004 | goleak.VerifyNone after mid-flight ctx cancel |
| 73 | `BenchmarkParseBlogResponse25Items` | `bench_test.go` | NFR-ADP8-001 | Median of 5 runs ≤ 5ms; allocs/op ≤ 500 |
| 74 | `TestMain` (goleak.VerifyTestMain) | `bench_test.go` | NFR-ADP8-004 | Package-level goroutine leak check |

RED-GREEN-REFACTOR per requirement:

1. RED: Write failing test for REQ-ADP8-N.
2. GREEN: Implement minimal code to pass.
3. REFACTOR: Tidy; extract shared helpers if they remove duplication
   WITHIN the package; keep file sizes manageable (target each `.go`
   file < 250 LoC excluding tests).

Greenfield note: `internal/adapters/naver/` does not exist. There is no
behaviour to preserve; no characterization tests needed.

---

## 9. Dependencies

### 9.1 Upstream SPEC Dependencies

- **SPEC-CORE-001 (implemented)**: provides `pkg/types.Adapter`,
  `pkg/types.Capabilities`, `pkg/types.Query`, `pkg/types.NormalizedDoc`,
  `*types.SourceError`, `types.OutcomeFromError`, `types.DocType` enum,
  `internal/adapters.Registry` with wrappedAdapter sole-emitter pattern,
  `internal/adapters/noop` reference shape. HARD dep.
- **SPEC-OBS-001 (implemented)**: provides `obs.Obs` bundle,
  `internal/obs/reqid.NewTransport` for request-ID propagation,
  `AdapterCalls{adapter,outcome}` and `AdapterCallDuration{adapter}`
  collectors. SOFT dep — adapter is nil-safe via the registry's nil-guards.
  The `adapter="naver"` cardinality value fits within the V1 14-adapter
  ceiling.
- **SPEC-IR-001 (implemented)**: documents the consumer contract for
  `Capabilities` (REQ-IR-008 selects AdapterSet by intersecting category-
  eligible DocTypes with `SupportedLangs`). ADP-008's `Capabilities()`
  shape (`DocTypes:[article,post]`, `SupportedLangs:["ko"]`) is the exact
  shape IR-001 acceptance test S-7
  (`.moai/specs/SPEC-IR-001/acceptance.md:177`) expects.
- **SPEC-ADP-001 (implemented)**: REQ-ADP-011 concurrent-safety contract is
  the precondition for NFR-ADP8-003. The reference adapter shape is
  copied verbatim (file layout, error mapping, MX tag plan, TDD harness).
- **SPEC-ADP-002 (implemented)**: HN adapter's `stripHTML` helper is
  duplicated; vertical-filter + multi-fixture testdata patterns are
  inherited.

### 9.2 Parallelizable

- **SPEC-ADP-003** (arXiv) / **SPEC-ADP-004** (GitHub) /
  **SPEC-ADP-005** (YouTube) / **SPEC-ADP-006** (Bluesky+X) /
  **SPEC-ADP-007** (SearXNG) / **SPEC-ADP-009** (Korean news) — all M3
  parallel ADP-* SPECs gated on SPEC-FAN-001 per
  `.moai/project/roadmap.md:122-123`. ADP-008 develops in parallel with
  these once FAN-001 is approved.
- **SPEC-IDX-003** (Korean tokenization, M3): can plan in parallel; consumes
  ADP-008's Body output downstream.
- **SPEC-CACHE-001** (M3): can plan in parallel; wraps fanout (and indirectly
  ADP-008 via fanout) in 5-phase access fallback.

### 9.3 Downstream Blocked SPECs

- **SPEC-FAN-001** (M3): consumes `(*naver.Adapter).Search` via
  `registry.Get("naver").Search(ctx, q)`.
- **SPEC-IDX-001** (M3): consumes `[]NormalizedDoc` for RRF fusion.
  ADP-008's `Score=0.5` constant means RRF fuses by RANK across adapters
  (no within-Naver score signal).
- **SPEC-IDX-003** (M3): consumes `Body` for mecab-ko tokenization.
- **SPEC-SYN-001** (basic synthesis): consumes `[]NormalizedDoc` for
  citation assembly via gpt-researcher Python sidecar.
- **SPEC-EVAL-003** (Korean-locale benchmark, M8): exercises ADP-008 against
  Korean golden queries.

### 9.4 External Dependencies (run-phase pins)

**Zero new Go module dependencies.** ADP-008 uses only:

- Go stdlib: `context`, `crypto/sha256`, `encoding/hex`, `encoding/json`,
  `errors`, `fmt`, `io`, `net`, `net/http`, `net/url`, `os`, `strconv`,
  `strings`, `time`, `unicode`, `unicode/utf8`
- `pkg/types` (already pinned via SPEC-CORE-001)
- `internal/obs/reqid` (already pinned via SPEC-OBS-001)
- Test-only: `go.uber.org/goleak` (already pinned by SPEC-ADP-001 +
  SPEC-ADP-002 run-phases)

---

## 10. Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Auth env vars missing at registration → adapter unusable | High | High | `Capabilities.RequiresAuth=true` + `AuthEnvVars=["NAVER_CLIENT_ID","NAVER_CLIENT_SECRET"]`. Registry's `RegisterWithOptions` rejects with `ErrMissingAuth` per `internal/adapters/registry.go:122-129`. `New` also returns `ErrAuthMissing` early. Tests use `RegisterOptions{SkipAuthCheck: true}`. |
| Auth secret value leaks into logs or error messages | High | High | REQ-ADP8-003 explicitly forbids logging the secret. `TestSearchNeverLogsSecret` + `TestNotesDoesNotContainSecret` enforce. Manual code review during run-phase ensures `fmt.Errorf` calls do not include the secret variable. |
| HTML entity / `<b>` tag leakage into Body / Title confuses synthesis | High | High | `stripHTML` applied to every text field per REQ-ADP8-009. Test fixture `search_response_blog_html_entities.json` covers `<b>`, `&amp;`, `&quot;`, `&#39;`, `&nbsp;`. |
| 25k/day quota exhaustion | High | Medium | `Capabilities.RateLimitPerMin=17` (conservative; daily/1440). Adapter does NOT manage quota — surfaces 429. SPEC-FAN-001 D6 says no retry. SPEC-EVAL-002 (M8) tracks per-adapter health. |
| DataLab opt-in misuse depletes shared 25k/day search budget | Medium | Medium | Default vertical is `blog`. DataLab requires explicit opt-in. `Capabilities.Notes` documents shared-quota. Open Question §11.4 tracks empirical verification. |
| News `originallink` empty surfaces Naver-mirror URL | Medium | Medium | Fallback to `link` documented in §6.3.3. NormalizedDoc.URL is canonical-best-effort; SPEC-SYN-003 (M4) handles deeper dedup. |
| Shopping `productType` enum drift (Naver adds new codes) | Low | Low | Stored as-is in `Metadata["productType"]`. Not used for routing/dedup. |
| `pubDate` parsing fails on edge-case date formats | Medium | Low | RFC 1123Z first, RFC 1123 fallback, then zero. `PublishedAt=zero` is valid per `pkg/types/normalized_doc.go:25`. |
| `postdate` (`YYYYMMDD`) malformed | Low | Low | Fall back to zero. Test fixture covers `postdate=""`, `postdate="abc"`. |
| Cross-domain redirect → SSRF surface | Low | Medium | 1-host allowlist `{openapi.naver.com}` rejects everything else. Test asserts via `TestSearchRejectsCrossDomainRedirect`. |
| `<b>` tag stripping false-positive eats user content | Low | Low | Test fixtures include `"<b>highlight</b> normal text"` and edge cases. Stdlib-only strip handles balanced tags; mismatched tags pass through. |
| Korean text encoding edge cases (NFC vs NFD) | Low | Low | Go strings are UTF-8 byte slices; `unicode/utf8` rune iteration handles all forms. No NFC/NFD coercion needed in v0.1. |
| Concurrent shared `*http.Client` | Low | High | `*http.Client` is goroutine-safe per Go stdlib. REQ-ADP8-012 asserts via 50-goroutine race-detector test. |
| Naver Open API contract drift (field added/removed/renamed) | Low | Medium | `encoding/json` ignores unknown fields. Test fixtures pinned. Naver Open API has been stable since 2014. |
| Vertical filter typo passed by caller | Low | Low | Reject case-sensitively with `ErrInvalidVertical`. `Capabilities.Notes` enumerates valid values. |
| `time.Now()` non-determinism in tests | Low | Low | `parseResponse` accepts `retrievedAt` argument; tests inject fixed time. |
| HTTP timeout (10s) too aggressive during incidents | Low | Low | Configurable via `Options.HTTPClient`. |
| Naver-side traffic shaping via UA fingerprinting (rejecting custom UA) | Low | Low | Custom UA convention is shared with Reddit (REQ-ADP-009) + HN (REQ-ADP2-006); no observed rejection in research. If issue arises, switch to a Naver-recommended UA per future amendment. |
| DataLab POST body size exceeds Naver limit (e.g., > 100 keyword groups) | Low | Low | Out of v0.1 scope; Naver's documented limit is loose. POST body is bounded by caller's `Query.Text`. |
| Hash collisions across Reddit/HN/Naver for same shared external URL | Low | Medium | `CanonicalHash` includes `SourceID` prefix per `pkg/types/normalized_doc.go:96-99` — never collide cross-source. |
| Score normalization formula impacts SPEC-IDX-001 RRF behaviour | Low | Low | Score=0.5 constant is uniform across ADP-008; RRF weights rank not raw score; constant is harmless. |
| Pagination cursor opacity confuses downstream consumers | Low | Low | REQ-ADP8-005 surfaces via `Metadata["next_cursor"]` on LAST doc. Cursor is integer-string; consumers MUST pass it back as `Query.Cursor` without re-parsing. |
| Stub `httptest.Server` race detector noise on 50-goroutine workload | Low | Low | ADP-001 + ADP-002 already validated this pattern. |
| Duplication of helpers with reddit + hn introduces drift over time | Medium | Low | Acknowledged. Three duplicate copies after ADP-008. SPEC-ADP-REFAC-001 post-M3 refactor SPEC. |

---

## 11. Open Questions

These are explicitly UNRESOLVED at SPEC-approval time. Each has a recommended
default and a one-line resolution owner. They do NOT block SPEC approval.

1. **Default vertical selection** (blog vs news vs web). **Recommended
   default**: `blog`. Richest Korean user-generated content; lowest
   cross-source overlap with SearXNG / daum / RSS_korean. Operators may
   override via filters. **Resolution owner**: SPEC-IR-001 author when
   intent sub-routing matures (post-M3).

2. **News canonical URL choice** (originallink vs link). **Recommended
   default**: originallink first, fall back to link. Dedup across adapters
   benefits from publisher URL. **Resolution owner**: SPEC-SYN-003 (M4
   dedup) author may revise if mirror-URL retention helps citation accuracy.

3. **Cross-adapter helper extraction** (sharing `parseRetryAfter`,
   `categorizeStatus`, redirect allowlist, `stripHTML` between Reddit / HN /
   Naver). **Recommended default**: defer until SPEC-ADP-REFAC-001 post-M3.
   Three duplicate copies after ADP-008 (rule of three reached). v0.1
   ships duplicates. **Resolution owner**: SPEC-ADP-REFAC-001 author (TBD
   post-M3).

4. **DataLab quota separation**. Is DataLab's quota independent of search
   verticals? **Recommended default**: assume SHARED 25k/day pool until
   empirically verified (safer for budget planning). **Resolution owner**:
   run-phase implementer during integration smoke test.

5. **Per-vertical score calibration**. Should `Score` derive from `pubDate`
   recency (news), `postdate` recency (blog), or `lprice` rank (shop)?
   **Recommended default**: keep `Score=0.5` constant in v0.1.
   Normalisation provides a `[0,1]` codomain regardless. Revisit after
   SPEC-IDX-001 RRF integration measurements. **Resolution owner**:
   SPEC-IDX-001 author.

6. **`DocTypeProduct` enum addition** to `pkg/types/capabilities.go` for
   shopping items. **Recommended default**: NO in v0.1. Adding a `DocType`
   constant breaks `pkg/types` SDK boundary semver. Defer to coordinated
   SPEC-CORE-001a if SPEC-IDX-001 needs the distinction. **Resolution owner**:
   SPEC-IDX-001 author.

7. **Sort selection (`sim` vs `date`)**. Hardcode `sim`, opt-in `date` via
   filter. **Recommended default**: hardcode `sim` in v0.1; add intent-derived
   sort via future IR-001 amendment. **Resolution owner**: SPEC-IR-001
   author.

8. **Korean tokenization at adapter layer**. Should the adapter pre-tokenize
   Korean text via mecab-ko before populating `Body`? **Recommended default**:
   NO. Tokenization is SPEC-IDX-003's job (M3, mecab-ko sidecar). Adapter
   returns plain stripped text; index layer tokenizes downstream.
   **Resolution owner**: SPEC-IDX-003 author.

---

## 12. References

### External (URL-cited; verified per research.md §8)

- Context7 corpus `/websites/developers_naver` — Naver Open API documentation
  reference for endpoints, parameters, response envelope, per-vertical
  schemas, HTML entity encoding.
- https://github.com/isnow890/naver-search-mcp — naver-search-mcp upstream
  repo (TypeScript, MIT). Tool taxonomy + endpoint constant reference.
- https://github.com/isnow890/naver-search-mcp/blob/main/src/clients/naver-api-core.client.ts
  — verified `searchBaseUrl="https://openapi.naver.com/v1/search"`,
  `datalabBaseUrl="https://openapi.naver.com/v1/datalab"` constants, plus
  30s timeout + 3 max-redirects defaults.
- https://github.com/searxng/searxng/blob/master/searx/engines/naver.py —
  SearXNG Naver engine (HTML scraping path, not adopted).
- RFC 7231 §7.1.3 Retry-After header semantics — basis for `parseRetryAfter`
  (inherited verbatim from SPEC-ADP-001 §6.3).

### Internal (file:line cited)

- `.moai/specs/SPEC-ADP-008/research.md` — full research artifact (this
  SPEC's research sibling).
- `.moai/specs/SPEC-CORE-001/spec.md` — Adapter / Capabilities / Query /
  NormalizedDoc / SourceError / Registry contract.
- `.moai/specs/SPEC-OBS-001/spec.md` — observability bundle, cardinality
  discipline.
- `.moai/specs/SPEC-IR-001/spec.md` — REQ-IR-008 AdapterSet selection.
- `.moai/specs/SPEC-IR-001/acceptance.md:177` — naver capabilities expected:
  `DocTypes:[article,post]`, `SupportedLangs:[ko]`.
- `.moai/specs/SPEC-ADP-001/spec.md` — reference adapter SPEC; structure
  inherited verbatim.
- `.moai/specs/SPEC-ADP-002/spec.md` — second-adapter SPEC; ADP-008 inherits
  `stripHTML`, vertical-filter pattern, multi-fixture testdata layout.
- `.moai/specs/SPEC-FAN-001/spec.md` — M3 fanout SPEC; consumes ADP-008
  via `registry.Get("naver").Search`.
- `pkg/types/adapter.go:28-45` — Adapter interface 4-method shape.
- `pkg/types/capabilities.go:38-62` — Capabilities struct + DocType enum.
- `pkg/types/query.go:18-44` — Query struct + Filter shape.
- `pkg/types/errors.go:14-218` — SourceError, Category, OutcomeFromError.
- `pkg/types/normalized_doc.go:40-106` — NormalizedDoc 15-field struct,
  Validate, CanonicalHash.
- `internal/adapters/registry.go:75-167` — Registry lifecycle, auth env
  validation at `:122-129`.
- `internal/adapters/registry.go:172-263` — wrappedAdapter sole-emitter
  pattern.
- `internal/adapters/noop/noop.go:1-46` — reference adapter shape.
- `internal/adapters/reddit/reddit.go:1-136` — Adapter struct pattern.
- `internal/adapters/reddit/search.go:1-167` — Search hot path pattern.
- `internal/adapters/reddit/parse.go:1-203` — parse pattern (ADP-008
  parseResponse follows the same shape, dispatched per-vertical).
- `internal/adapters/reddit/client.go:1-125` — HTTP client + redirect
  allowlist + categorizeStatus pattern.
- `internal/adapters/reddit/errors.go:1-64` — parseRetryAfter helper
  (duplicated verbatim).
- `internal/adapters/hn/strip.go` — stripHTML pattern (mirrored for
  ADP-008).
- `internal/llm/client.go:31-65` — HTTP client construction with timeout +
  reqid Transport.
- `.moai/project/roadmap.md:53` — M3 row for SPEC-ADP-008.
- `.moai/project/roadmap.md:122-123` — M3 7-way parallelization plan.
- `.moai/project/roadmap.md:150` — M3 exit criterion (Korean ranked first).
- `.moai/project/structure.md:25` — `internal/adapters/naver/` reservation.
- `.moai/project/tech.md:116` — adapter-strategy row (Naver: 25,000/day;
  Korean-locale primary).
- `.moai/config/sections/quality.yaml:17,76` — `development_mode: tdd`,
  `test_coverage_target: 85`.
- `.moai/config/sections/harness.yaml` — auto-routing rules.
- `.moai/config/sections/language.yaml` — `documentation: en`,
  `code_comments: en`.

---

*End of SPEC-ADP-008 v0.1 (DRAFT — pending plan-auditor review)*
