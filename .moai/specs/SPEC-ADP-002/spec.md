---
id: SPEC-ADP-002
title: Hacker News Adapter
version: 0.1.0
milestone: M2 — First end-to-end slice
status: implemented
implemented_at: 2026-04-28
priority: P0
owner: expert-backend
methodology: tdd
coverage_target: 85
created: 2026-04-28
updated: 2026-04-28
author: limbowl
issue_number: null
depends_on: [SPEC-CORE-001, SPEC-ADP-001]
blocks: [SPEC-FAN-001, SPEC-CLI-001, SPEC-SYN-001]
---

# SPEC-ADP-002: Hacker News Adapter

## HISTORY

- 2026-04-28 (initial draft v0.1, limbowl via manager-spec):
  Second-adapter SPEC drafted following the SPEC-ADP-001 reference
  shape verbatim. Scope and contracts derived from
  `.moai/specs/SPEC-ADP-002/research.md` (every external claim
  URL-cited; every internal claim file:line-cited). Built on
  SPEC-CORE-001 (`pkg/types.Adapter` interface, `pkg/types.Capabilities`
  descriptor, `pkg/types.Query`, `pkg/types.NormalizedDoc` 15-field
  struct, `*types.SourceError` taxonomy, registry wrappedAdapter
  sole-emitter pattern at `internal/adapters/registry.go:172-263`) and
  SPEC-ADP-001 (file layout + error mapping + MX tag plan + TDD harness
  + Tanh score normaliser, all reused as-is or as pattern). Soft dep on
  SPEC-IR-001 for `Capabilities` consumer contract.

  Key structural inheritance from ADP-001:
  - File layout (`reddit.go`/`search.go`/`client.go`/`parse.go`/
    `score.go`/`errors.go`/`bench_test.go` + testdata/) → identical
    in `internal/adapters/hn/`.
  - HTTP client construction (10s timeout, redirect allowlist,
    `reqid.NewTransport` wrapping) — host allowlist swapped to
    `{hn.algolia.com, news.ycombinator.com}`.
  - `categorizeStatus` rosetta — adopted as-is, only `Adapter` field
    swapped from `"reddit"` to `"hackernews"`.
  - `parseRetryAfter` helper — adopted verbatim (RFC 7231 §7.1.3
    parser, 5s default, 60s cap).
  - `normalizeScore` Tanh formula (divisor=100, center=0.5) — adopted
    verbatim. HN points distribution behaves similarly to Reddit
    upvotes for the typical `[0, ~5000]` range, so the formula's
    inflection point at 100 remains operationally meaningful.
  - Sole-emitter discipline (zero metrics/logs/spans emitted by the
    adapter; registry wrappedAdapter emits all observability).
  - `var _ types.Adapter = (*Adapter)(nil)` compile-time interface
    assertion.

  HN-specific deltas from ADP-001 documented inline in §6 and §7:
  - Endpoint: `https://hn.algolia.com/api/v1/search` (Algolia HN
    Search; no auth, public).
  - URL parameters: `query`, `tags=story` (hardcoded), `hitsPerPage`,
    `page`, optional `numericFilters` for `since` and `min_points`
    filters.
  - Response envelope: Algolia `{hits, nbHits, page, nbPages, ...}`
    instead of Reddit's `{data: {after, children: [...]}}`. New
    `parseHits` transform with HN-specific JSON struct tags.
  - Self-post handling: `url` field MAY be empty; canonical permalink
    `https://news.ycombinator.com/item?id=<objectID>` constructed as
    fallback.
  - HTML body: HN's `story_text` MAY contain `<p>`/`<a>`/`<i>` tags;
    `stripHTML` helper added (stdlib-only, conservative tag-strip
    + entity decode) to produce plain-text Body/Snippet for
    synthesis.
  - Defensive `_tags` filter: even though `tags=story` is requested,
    the parser skips hits whose `_tags` array does NOT include
    `"story"` (Algolia is known to have transient discrepancies).
  - Pagination: integer-string cursor (e.g., `"1"` for page 2)
    surfaced via `Metadata["next_cursor"]` on the LAST returned doc;
    parsed via `strconv.Atoi` on the way back in.

  10 EARS REQs (8 × P0 + 2 × P1) covering all five EARS patterns
  (Ubiquitous, Event-Driven, State-Driven via REQ-ADP2-010 concurrency
  contract, Optional, Unwanted), 3 NFRs, ~38 representative TDD tests,
  6 Open Questions carried forward from research.md §7. Zero new Go
  module dependencies — pure stdlib (`net/http`, `encoding/json`,
  `time`, `context`, `errors`, `strings`, `strconv`, `net/url`,
  `unicode/utf8`, `math`) plus existing `pkg/types` and
  `internal/obs/reqid` (nil-safe consumer). Inserted into M2 as the
  SECOND adapter consuming the SPEC-CORE-001 contract; the M2 exit
  criterion (`.moai/project/roadmap.md:149`) requires Reddit + HN
  results returned by `usearch query`. Harness level: standard (single
  domain, ≤10 source files, no security/payment keywords). Sprint
  Contract optional. Ready for plan-auditor review and annotation
  cycle.

---

## 1. Purpose

The M2 milestone exit criterion is unambiguous: `usearch query
"hello world"` returns Reddit + HN results with one synthesized
paragraph + citations (`.moai/project/roadmap.md:149`). SPEC-ADP-001
implemented the Reddit adapter as the reference shape. SPEC-ADP-002
implements the SECOND adapter — Hacker News via the public Algolia HN
Search API at `https://hn.algolia.com/api/v1/search`.

Hacker News is the natural choice for the second M2 adapter because:

1. **Public no-auth endpoint** (Algolia HN) eliminates secret-
   management complexity from the M2 thin slice, identical to ADP-001's
   posture. Zero ENV propagation, zero RegisterOptions complexity.
2. **Stable, well-documented API** (https://hn.algolia.com/api,
   referenced widely by SearXNG, gpt-researcher, and dozens of
   third-party HN tools). Low risk of surprise during run-phase
   implementation.
3. **Different response shape than Reddit** (Algolia's
   `{hits, nbHits, page, ...}` versus Reddit's
   `{data: {after, children: [...]}}`) exercises the JSON-mapping
   discipline established in ADP-001 §6.3 against a genuinely
   different envelope.
4. **Self-posts with HTML bodies** (Ask HN / Show HN / Tell HN)
   exercise body-text-cleaning logic that link-only Reddit posts
   did not require — the HTML-strip pass introduced here is the
   reference for any future adapter consuming HTML-encoded bodies
   (e.g., M3 Bluesky which encodes link cards).
5. **Two-branch URL construction** (link posts have `url`; self-posts
   construct `news.ycombinator.com/item?id=<id>` permalinks) is the
   reference for any future adapter where the canonical URL is
   conditional on item type.
6. **All four error categories reachable** in normal operation:
   Algolia returns 429 under sustained load (CategoryRateLimited),
   503 during cluster maintenance (CategoryUnavailable), 4xx for
   malformed queries (CategoryPermanent), and timeouts/network blips
   resolve to (CategoryTransient via `context.DeadlineExceeded`).
   The error-taxonomy contract gets a second real workout, validating
   that the ADP-001 mapping is generic.

Like ADP-001, SPEC-ADP-002 adapter does NOT do fanout (SPEC-FAN-001
owns goroutine dispatch), does NOT do retry (SPEC-FAN-001 owns
orchestration), does NOT do caching (SPEC-CACHE-001 owns 5-phase
fallback), does NOT do ranking fusion (SPEC-IDX-001 owns RRF), and
does NOT emit any metric/log/span itself (the registry wrappedAdapter
does, sole-emitter discipline). It DOES one job: turn a `types.Query`
into an Algolia HN HTTP request, parse the JSON `hits` envelope, and
return `[]types.NormalizedDoc` or `*types.SourceError`.

Completion unblocks the M2 thin-slice integration: SPEC-CLI-001 wires
both adapters into `cmd/usearch/main.go`, SPEC-SYN-001 consumes
`[]NormalizedDoc` from both for citation assembly, and SPEC-FAN-001
(M3) gains a second adapter to validate fanout/retry orchestration
against. The HN adapter also serves as the SECOND copy of the
reference pattern, validating that ADP-001's shape is portable
before the seven-way M3 ADP-* parallel implementation begins.

---

## 2. Scope

### 2.1 In-Scope

| Ref | Item |
|-----|------|
| a | `internal/adapters/hn/hn.go`: `Adapter` struct (HTTP client + base URL + user-agent + healthcheck target), `New(opts Options) (*Adapter, error)` constructor, `Name() string` returning `"hackernews"`, `Capabilities() types.Capabilities` returning a deterministic descriptor (RequiresAuth=false, AuthEnvVars=nil, DocTypes=[DocTypePost], SupportedLangs=nil, SupportsSince=true (HN supports `numericFilters=created_at_i>...`), RateLimitPerMin=60, DefaultMaxResults=25, DisplayName="Hacker News", Notes documenting the public Algolia endpoint + self-post permalink convention + 5s default Retry-After + `tags=story` filter), and `Healthcheck(ctx) error` (TCP-connect probe to `hn.algolia.com:443` with caller-supplied ctx, target injectable via Options). Compile-time interface assertion `var _ types.Adapter = (*Adapter)(nil)` at the bottom. |
| b | `internal/adapters/hn/search.go`: `(*Adapter).Search(ctx, q types.Query) ([]types.NormalizedDoc, error)` — the hot path. Validates the query, parses any `q.Cursor` as a non-negative integer page, builds the request URL via `url.Values` with `query`, `tags=story`, `hitsPerPage` (clamped), `page` (when cursor present), `numericFilters` (when `since` or `min_points` filter present), delegates HTTP execution to `client.go`, delegates response parsing to `parse.go`, returns `[]NormalizedDoc` or `*SourceError`. Honours `ctx` cancellation throughout. |
| c | `internal/adapters/hn/client.go`: HTTP client construction (timeout=10s default, `CheckRedirect` enforces a domain allowlist `{hn.algolia.com, news.ycombinator.com}` with max 3 hops, `Transport` wrapped with `internal/obs/reqid.NewTransport(http.DefaultTransport)` for request-ID propagation), single `doRequest(req *http.Request) (*http.Response, error)` helper that sets the User-Agent header and the `Accept: application/json` header, reuse of the `categorizeStatus` rosetta from ADP-001 (renamed for the `"hackernews"` adapter field — see §6.2 for whether this is package-local helper or a shared `pkg/types/sourceerror` factory; default is package-local mirror to avoid premature abstraction). |
| d | `internal/adapters/hn/parse.go`: `parseHits(body []byte, retrievedAt time.Time, currentPage int, totalPages int) ([]types.NormalizedDoc, string, error)` — parses the Algolia HN response envelope `{hits, nbHits, page, nbPages, hitsPerPage}` into `[]NormalizedDoc` and returns the next-page cursor as the second return value. Filters out hits whose `_tags` array does NOT include `"story"` (defensive). Per-hit transform per the field-mapping table in §6.3. Empty `hits` returns `(nil, "", nil)`. Malformed JSON returns `*SourceError{Category: CategoryPermanent, Cause: <json error>}`. |
| e | `internal/adapters/hn/strip.go`: `stripHTML(s string) string` — conservative stdlib-only tag-strip + entity-decode helper for HN's `story_text` field. No external HTML parser; handles `<p>`, `<a>`, `<i>`, `<br>`, `<code>`, `<pre>`, `&amp;`, `&lt;`, `&gt;`, `&quot;`, `&#39;`, `&nbsp;` at minimum. NOT a security boundary (no XSS sanitization required — output is plain text consumed by synthesis, never rendered as HTML). |
| f | `internal/adapters/hn/score.go`: `normalizeScore(points int) float64` — Tanh formula identical to ADP-001 §2.3 (`clamp(0.5 + 0.5 * tanh(points / 100.0), 0.0, 1.0)`). Package-level constants `tanhDivisor = 100.0` and `scoreCenter = 0.5` annotated with `@MX:NOTE`. The formula is duplicated rather than imported because cross-adapter formula sharing creates premature coupling; if a third adapter wants the same normaliser, it goes into `pkg/types/score` or similar — out of scope here. |
| g | `internal/adapters/hn/errors.go`: package-private sentinel `ErrInvalidQuery = errors.New("hn: query text empty or whitespace-only")` (wrapped in `*SourceError{Category: CategoryPermanent}` by Search), `ErrInvalidCursor = errors.New("hn: cursor must be non-negative integer page")` (wrapped in `*SourceError{Category: CategoryPermanent}` by Search). The `parseRetryAfter` helper from ADP-001 is duplicated here (5s default, 60s cap, integer-seconds + HTTP-date parsing per RFC 7231 §7.1.3) — see §6.2 for shared-helper alternatives. |
| h | `internal/adapters/hn/hn_test.go`: tests for Adapter interface conformance (`var _ types.Adapter` assertion via `assertInterface`), `Name()` returns `"hackernews"`, `Capabilities()` returns deterministic value (called twice; equal), `Healthcheck()` succeeds against a stub `httptest.Server`, `New()` validates options. |
| i | `internal/adapters/hn/search_test.go`: the largest test file. Drives `(*Adapter).Search` against `httptest.Server` with golden fixtures: happy path 25 stories (mix of link + self-posts), empty result, 429 with Retry-After, 4xx, 5xx, redirect to allowed and disallowed hosts, `since` filter, `min_points` filter, pagination cursor round-trip, ctx cancellation mid-request, invalid cursor rejection, defensive `_tags` filter (mixed story + comment hits). |
| j | `internal/adapters/hn/client_test.go`: HTTP client unit tests — `categorizeStatus` truth table over 7 status codes, `parseRetryAfter` table over 6 input shapes, redirect allowlist enforcement (allowlist + cross-domain rejection + chain-over-3 rejection), User-Agent header presence, `Accept: application/json` header presence. |
| k | `internal/adapters/hn/parse_test.go`: field-mapping unit tests — table over 5 fixtures (link post with external url, Ask HN self-post with HTML body, deleted-author post, story with multiple `_tags`, mixed story+comment hits). Asserts each NormalizedDoc field per the §6.3 mapping table. Snippet truncation to 280 runes. Score normalization (4 example values). Pagination cursor round-trip. Filter of non-story `_tags`. Hash field is empty (REQ-ADP2-005). |
| l | `internal/adapters/hn/strip_test.go`: `stripHTML` table-driven test over 8 inputs (empty, plain text, single tag, nested tags, malformed unclosed tag, entity decoding `&amp;`/`&lt;`/`&gt;`/`&quot;`/`&#39;`, mixed tags + entities, very long body). |
| m | `internal/adapters/hn/score_test.go`: `normalizeScore` table-driven test over 7 point values (`-1000, -10, 0, 10, 100, 1000, 10000`) with expected `[0.0, 1.0]` outputs computed from the formula, asserted within `±0.001`. Determinism check. |
| n | `internal/adapters/hn/bench_test.go`: `BenchmarkParseHits25Hits` (NFR-ADP2-001 — p50 ≤ 5 ms parse time on amd64 for a 25-hit Algolia response fixture; allocation ≤ 20 allocs per hit parsed = ≤ 500 allocs total). |
| o | `internal/adapters/hn/testdata/`: golden JSON fixtures — `search_response.json` (25-story happy path, ~6KB), `search_response_empty.json` (zero hits, ~200B), `search_response_pagination.json` (page 0 with nbPages=5 for cursor round-trip, ~6KB), `search_response_self_post.json` (single Ask HN with HTML `story_text`, ~1KB), `search_response_deleted_author.json` (story with empty `author`, ~1KB), `search_response_with_comments.json` (mixed `_tags` containing story + comment for defensive filter validation, ~3KB), `search_response_malformed.json` (truncated JSON for parse-error path, ~200B). |

### 2.2 Out-of-Scope

This SPEC explicitly excludes the following items. Each has a known
destination SPEC; this list prevents scope creep into ADP-002 (the
second-adapter validation of the reference shape).

- **Per-source customisations specific to other sources** (arXiv
  OAI-PMH, GitHub PAT auth, YouTube yt-dlp metadata, Bluesky AT
  Protocol, Naver Korean-locale handling, Daum scraper-style handling,
  KoreaNewsCrawler RSS, SearXNG bridge, Polymarket public API) →
  SPEC-ADP-003 through SPEC-ADP-009 (M3).
- **Retry orchestration** (exponential backoff, circuit breaker,
  per-source health-state tracking, jitter, max-attempt counters) →
  SPEC-FAN-001 (M3). The adapter returns one categorised error per
  request and does not retry.
- **Response caching** → SPEC-CACHE-001 (M3). Each `Search` call is
  independent and idempotent at the adapter layer.
- **Result ranking, deduplication, RRF fusion across adapters** →
  SPEC-IDX-001 (M3). The adapter returns hits in Algolia's relevance
  order with `Score` in `[0.0, 1.0]` per the inherited Tanh formula,
  but does not re-rank.
- **HN comment retrieval** (`tags=comment` instead of `tags=story`)
  → out of scope; v0.1 hardcodes `tags=story` so only stories surface.
  Future SPEC-ADP-002a may add a `Query.Filters[Key="kind"]` switch.
- **HN poll / pollopt retrieval** — out of scope; same rationale as
  comments.
- **`search_by_date` mode** (strict reverse-chronological) — out of
  scope. v0.1 hardcodes the relevance-ranked `search` endpoint. A
  future `Query.Filters[Key="sort"]="date"` switch may route to
  `search_by_date`; deferred to a P2 enhancement post-M3.
- **Algolia's `restrictSearchableAttributes`, `attributesToRetrieve`,
  `typoTolerance`, `analytics` parameters** — out of v0.1; defaults
  apply.
- **Algolia's `_highlightResult` field** — IGNORED by the parser
  (search-highlight markup is noise for synthesis). Not requested via
  `attributesToRetrieve` in v0.1 for URL simplicity.
- **Live network integration tests in CI** → out of v0.1.
  `httptest.Server` + golden fixtures only. Optional env-gated live
  test (`-tags=integration` + `HN_LIVE=1`) deferred.
- **OpenAPI / proto schema for the adapter response** — the
  `[]types.NormalizedDoc` return type IS the schema; no separate IDL.
- **Korean tokenisation or language inference** for HN posts →
  SPEC-IDX-003 (M3). The adapter sets `NormalizedDoc.Lang = ""`
  (unknown).
- **`pkg/llm` integration** — the HN adapter does NOT call any LLM.
  Classification is the Intent Router's job (SPEC-IR-001).
- **Robust HTML parsing via `golang.org/x/net/html`** — out of v0.1.
  The stdlib-only `stripHTML` helper is sufficient for HN's shallow
  body markup. Open Question §11.1 documents revisit triggers.
- **Cross-adapter helper extraction** (sharing `parseRetryAfter`,
  `categorizeStatus`, `redirectAllowlist` between Reddit and HN) — out
  of v0.1. The two adapters MAY duplicate these helpers; extraction is
  premature. A future SPEC-ADP-REFAC-001 (post-M3, after seeing 4+
  adapters) MAY consolidate. See §11.4.
- **Per-adapter custom Prometheus metrics** (e.g.,
  `hn_pagination_pages_total`) — would require amending
  SPEC-OBS-001's allowlist; out of scope. The shared
  `AdapterCalls{adapter="hackernews",outcome}` family is sufficient.

### 2.3 Score Normalization (Inheritance from ADP-001)

[HARD] The score normaliser in `score.go::normalizeScore(points int)
float64` uses the same Tanh formula as SPEC-ADP-001 §2.3:

```
Score = clamp(0.5 + 0.5 * tanh(points / 100.0), 0.0, 1.0)
```

Domain (HN-specific): HN `points` ∈ `[0, ~10000]` integer (in practice
the all-time top stories are around 6000 points; new stories start at
1 — submitter's auto-vote). HN does not have negative scores in the
public response (privately-flagged scores aren't surfaced via Algolia).

The formula's `[0, 1]` codomain, `score=0 → Score=0.5` semantic, and
saturation at `±1000+` properties are unchanged. ADP-002 keeps the
same divisor=100 and center=0.5 because the HN points distribution is
similar enough to Reddit upvotes that a separate calibration would be
premature optimization. SPEC-IDX-001 (M3) RRF fusion uses rank not raw
score across adapters, so the precise score curve matters less than
the bounded codomain and determinism shared between adapters.

Open Question §11.5 tracks revisit triggers if measured ranking quality
indicates HN-specific calibration is needed.

---

## 3. EARS Requirements

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| REQ-ADP2-001 | Ubiquitous | The package `internal/adapters/hn` SHALL expose an `Adapter` struct that implements `pkg/types.Adapter` exactly: `Name() string` returning `"hackernews"`, `Search(ctx context.Context, q types.Query) ([]types.NormalizedDoc, error)`, `Healthcheck(ctx context.Context) error`, `Capabilities() types.Capabilities`. The package SHALL include a compile-time interface assertion `var _ types.Adapter = (*Adapter)(nil)`. `Capabilities()` SHALL be deterministic (two consecutive calls return equal values) with `SourceID="hackernews"`, `DisplayName="Hacker News"`, `DocTypes=[DocTypePost]`, `SupportedLangs=nil`, `SupportsSince=true`, `RequiresAuth=false`, `AuthEnvVars=nil`, `RateLimitPerMin=60`, `DefaultMaxResults=25`, and `Notes` containing the substrings `"Algolia HN Search"`, `"public no-auth"`, `"tags=story"`, and `"self-posts use news.ycombinator.com permalink"`. | P0 | `TestAdapterName`, `TestAdapterImplementsInterface` (compile-time), `TestCapabilitiesDeterministic`, `TestCapabilitiesShape` (asserts all 9 documented field values + Notes substring matches), `TestHealthcheckSucceeds` (stub `httptest.Server` injected via Options). All in `internal/adapters/hn/hn_test.go`. |
| REQ-ADP2-002 | Event-Driven | WHEN `(*Adapter).Search(ctx, q)` is invoked with a non-empty `q.Text`, the adapter SHALL build an HTTP GET request to `https://hn.algolia.com/api/v1/search` with the following query parameters: `query=<url.QueryEscape(q.Text)>`, `tags=story`, `hitsPerPage=clamp(q.MaxResults, 1, 100)` (defaulting to 25 when `q.MaxResults == 0`), `page=<parsed cursor>` (only when `q.Cursor != ""`), and optionally `numericFilters=<comma-joined expressions>` when REQ-ADP2-007 filters are present. The adapter SHALL execute the request via the constructed `*http.Client`, parse the JSON `hits` envelope per REQ-ADP2-005 mapping, and return `(docs, nil)` on HTTP 200 with `len(docs) ≤ 100`. | P0 | `TestSearchHappyPath25Stories` (httptest.Server returns `search_response.json`; assert 25 NormalizedDocs returned, each with all required fields populated and `Validate()` returning nil); `TestSearchURLParametersIncludeAllRequired` (inspect captured request URL; assert `query`, `tags=story`, `hitsPerPage` always present); `TestSearchClampsHitsPerPageTo100` (q.MaxResults=500 → URL has `hitsPerPage=100`); `TestSearchDefaultsHitsPerPageTo25` (q.MaxResults=0 → URL has `hitsPerPage=25`); `TestSearchOmitsPageWhenCursorEmpty` (q.Cursor="" → URL has no `page` param); `TestSearchSetsPageWhenCursorPresent` (q.Cursor="3" → URL has `page=3`). All in `search_test.go`. |
| REQ-ADP2-003 | Event-Driven | WHEN HTTP 429 is received from the Algolia HN endpoint, the adapter SHALL parse the `Retry-After` response header per RFC 7231 §7.1.3 (integer-seconds OR HTTP-date), cap the result at 60 seconds (any larger value is replaced with 60s), default to 5 seconds when the header is missing or malformed (Algolia commonly omits `Retry-After` on 429), and return `(nil, &types.SourceError{Adapter:"hackernews", Category: types.CategoryRateLimited, HTTPStatus: 429, RetryAfter: <duration>, Cause: errors.New("hn: rate limited")})`. The adapter SHALL NOT retry internally. | P0 | `TestSearchHTTP429WithIntegerRetryAfter` (`Retry-After: 30` → RetryAfter=30s); `TestSearchHTTP429WithHTTPDateRetryAfter` (HTTP-date 30s in future → RetryAfter ∈ (25s, 35s)); `TestSearchHTTP429NoRetryAfterDefaults5s` (no header → RetryAfter=5s; matches Algolia's typical behaviour); `TestSearchHTTP429RetryAfterCapped60s` (`Retry-After: 999` → 60s); `TestSearchHTTP429NoInternalRetry` (assert exactly 1 outbound request observed). All in `search_test.go` + `client_test.go`. |
| REQ-ADP2-004 | Event-Driven | WHEN HTTP 401, 403, 404, or any 4xx other than 429 is received, the adapter SHALL return `(nil, &types.SourceError{Adapter:"hackernews", Category: types.CategoryPermanent, HTTPStatus: <code>, Cause: errors.New("hn: permanent failure: <code>")})`. WHEN HTTP 500/502/503/504 is received OR a connection error occurs (DNS failure, dial timeout, read timeout, TLS handshake failure), the adapter SHALL return `(nil, &types.SourceError{Adapter:"hackernews", Category: types.CategoryUnavailable, HTTPStatus: <code or 0>, Cause: <inner error>})`. Network-layer errors set `HTTPStatus=0`. The adapter SHALL NOT retry. | P0 | `TestSearchHTTP4xx` (table over 401/403/404 → ErrPermanent + matching HTTPStatus); `TestSearchHTTP5xx` (table over 500/503 → ErrSourceUnavailable + matching HTTPStatus); `TestSearchConnectionRefused` (httptest.Server closed before request; HTTPStatus=0); `TestSearchUnavailablePreservesUnderlyingError` (assert `errors.Unwrap(srcErr).Error()` contains the inner cause). All in `search_test.go` + `client_test.go`. |
| REQ-ADP2-005 | Ubiquitous | The adapter SHALL transform each Algolia hit whose `_tags` array CONTAINS `"story"` into one `types.NormalizedDoc` using the field mapping in §6.3, MUST set `RetrievedAt = time.Now().UTC()` at the moment of parsing, MUST leave `Hash = ""` (consumers compute via `CanonicalHash()`), MUST populate `Metadata` with at minimum the keys `{num_comments, points, tags, external_url}`, MUST set `DocType = types.DocTypePost`, MUST set `Lang = ""` (unknown). When `url` is empty (self-post), the adapter SHALL construct the canonical permalink as `"https://news.ycombinator.com/item?id=" + objectID` and use that as `NormalizedDoc.URL`. When `story_text` is non-empty, the adapter SHALL apply `stripHTML` before assigning to `Body` and `Snippet`. Hits whose `_tags` array does NOT contain `"story"` SHALL be skipped silently (defensive against Algolia returning mixed item types despite the `tags=story` request parameter). The next-page cursor (when `currentPage + 1 < nbPages`) SHALL be returned as the second return value of `parseHits` so `Search` can surface it via `Metadata["next_cursor"]` on the LAST returned NormalizedDoc — encoded as `strconv.Itoa(currentPage + 1)`. | P0 | `TestParseHitsFieldMapping` (table-driven over 4 fixtures); `TestParseHitsFiltersNonStoryTags` (fixture with mixed story/comment `_tags` → only story hits returned); `TestParseHitsSelfPostUsesPermalink` (fixture with `url=""` and `objectID="12345"` → returned doc has `URL == "https://news.ycombinator.com/item?id=12345"`); `TestParseHitsHTMLBodyStripped` (fixture with `story_text="<p>Hello <b>world</b></p>"` → returned doc has `Body == "Hello world"`); `TestParseHitsPaginationCursor` (fixture with `page=0, nbPages=5` → returned NormalizedDocs[len-1].Metadata["next_cursor"] == "1"); `TestParseHitsNoCursorOnLastPage` (fixture with `page=4, nbPages=5` → no doc has `next_cursor` key); `TestParseHitsHashEmpty` (every returned doc has `Hash == ""`); `TestParseHitsMetadataKeys` (all 4 required keys present in each returned doc). All in `parse_test.go`. |
| REQ-ADP2-006 | Ubiquitous | The adapter SHALL set the `User-Agent` HTTP header on every outbound request to a non-default value of the form `usearch/<version> (+https://github.com/elymas/universal-search)` where `<version>` is supplied via `Options.UserAgentVersion` (default `"v0.1"`). The adapter SHALL set the `Accept` header to `application/json`. While Algolia HN is more permissive than Reddit about default Go User-Agents, setting a custom UA preserves the project-wide convention from ADP-001 REQ-ADP-009 and identifies traffic for operational debugging at Algolia's side. | P0 | `TestSearchSetsCustomUserAgent` (inspect captured `r.Header.Get("User-Agent")`; assert it starts with `"usearch/"` and contains `"(+https://github.com/elymas/universal-search)"`); `TestSearchSetsAcceptJSON` (assert `Accept: application/json`); `TestSearchUserAgentVersionConfigurable` (Options.UserAgentVersion="v0.2-rc1" → header contains `"usearch/v0.2-rc1"`). All in `client_test.go`. |
| REQ-ADP2-007 | Optional | WHERE `Query.Filters` contains an entry with `Key == "since"` AND `Value` parses as a positive Unix-seconds integer, the adapter SHALL include the expression `created_at_i>=<value>` in the `numericFilters` query parameter. WHERE `Query.Filters` contains an entry with `Key == "min_points"` AND `Value` parses as a positive integer, the adapter SHALL include the expression `points>=<value>` in the `numericFilters` query parameter. When both filters are present, expressions SHALL be comma-joined (e.g., `numericFilters=created_at_i>=1700000000,points>=10`). Filter keys other than these two SHALL be silently ignored (no error returned). Malformed filter values (non-numeric, negative) SHALL be silently dropped (no error, no expression added). The default behaviour (no filter supplied) is to omit the `numericFilters` parameter entirely. | P1 | `TestSearchSinceFilterAdded` (Filters=[{since, "1700000000"}] → URL has `numericFilters=created_at_i>=1700000000`); `TestSearchMinPointsFilterAdded` (Filters=[{min_points, "10"}] → URL has `numericFilters=points>=10`); `TestSearchBothFiltersJoined` (Filters=[{since, "1700000000"}, {min_points, "10"}] → URL has `numericFilters=created_at_i>=1700000000,points>=10`); `TestSearchUnknownFilterIgnored` (Filters=[{nsfw, "true"}] → URL has no `numericFilters`); `TestSearchMalformedFilterDropped` (Filters=[{since, "abc"}] → URL has no `numericFilters`); `TestSearchNegativeFilterDropped` (Filters=[{min_points, "-5"}] → URL has no `numericFilters`); `TestSearchNoFilterOmitsParameter` (Filters=nil → URL has no `numericFilters`). All in `search_test.go`. |
| REQ-ADP2-008 | Unwanted | IF `Query.Text` is empty OR contains only Unicode whitespace runes (per `unicode.IsSpace` over every rune), THEN the adapter SHALL return `(nil, &types.SourceError{Adapter:"hackernews", Category: types.CategoryPermanent, Cause: ErrInvalidQuery})` immediately and SHALL NOT issue any HTTP request. IF `Query.Cursor` is non-empty AND does NOT parse as a non-negative integer via `strconv.Atoi` (negative integers also rejected), THEN the adapter SHALL return `(nil, &types.SourceError{Adapter:"hackernews", Category: types.CategoryPermanent, Cause: ErrInvalidCursor})` immediately and SHALL NOT issue any HTTP request. | P0 | `TestSearchEmptyQueryRejectedNoHTTP` (table over `["", "   ", "\t\n  \r"]` for `q.Text`; for each asserts `errors.Is(err, types.ErrPermanent)` AND assert httptest.Server received zero requests); `TestSearchInvalidCursorRejectedNoHTTP` (table over `["abc", "-1", "1.5", "1e3"]` for `q.Cursor`; for each asserts `errors.Is(err, types.ErrPermanent)` AND zero requests). All in `search_test.go`. |
| REQ-ADP2-009 | Optional | WHERE the response is HTTP 301/302/303/307/308, the adapter's `*http.Client.CheckRedirect` SHALL follow up to 3 redirects WITHIN the allowlist `{hn.algolia.com, news.ycombinator.com}`. Cross-domain redirects (any other host) SHALL be rejected by returning an error from `CheckRedirect`; the adapter wraps this as `*SourceError{Adapter:"hackernews", Category: CategoryPermanent, Cause: errors.New("hn: cross-domain redirect rejected: <target host>")}` to prevent SSRF. Redirect chains exceeding 3 hops SHALL be rejected with a "too many redirects" message. While Algolia HN does not redirect cross-domain in normal operation, the allowlist is defensive and consistent with the pattern established in SPEC-ADP-001 REQ-ADP-010. | P1 | `TestSearchFollowsAllowlistRedirect` (httptest.Server returns 302 to a second httptest.Server with Host header rewritten to `hn.algolia.com`; assert 200-path NormalizedDocs returned); `TestSearchRejectsCrossDomainRedirect` (httptest.Server returns 301 to `attacker.com`; assert `errors.Is(err, types.ErrPermanent)` AND error message contains `"cross-domain redirect"`); `TestSearchRejectsRedirectChainOver3` (httptest.Server bouncing within allowlist 4 times; assert error after 3 hops). All in `client_test.go`. |
| REQ-ADP2-010 | State-Driven | WHILE the same `*Adapter` instance is registered in the adapter registry and is being invoked concurrently from N goroutines (N ≥ 1), each `Search(ctx, q)` call SHALL execute independently with no shared mutable state across calls (the underlying `*http.Client` is goroutine-safe per Go stdlib; the adapter holds no per-call state); the cumulative effect SHALL be N independent HTTP round-trips with no race-detector alarms. This requirement crystallises the concurrency contract that the registry (`internal/adapters/registry.go:172-263` wrappedAdapter) and the future fanout layer (SPEC-FAN-001) rely on. | P0 | `TestSearchConcurrentSafe` (50 goroutines each issuing one Search against the same httptest.Server; assert (a) no race-detector alarm under `-race`, (b) total response count = 50 observed at the stub, (c) all 50 returned slices are `[]types.NormalizedDoc` with `Validate()` returning nil for every doc). In `search_test.go`. |

---

## 4. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| NFR-ADP2-001 | Performance (parse path) | The parse path `parseHits(body []byte, retrievedAt time.Time, currentPage int, totalPages int) ([]NormalizedDoc, string, error)` SHALL execute with mean wall-clock duration per op ≤ 5 ms over `go test -bench=BenchmarkParseHits25Hits -benchtime=10x -count=5 ./internal/adapters/hn/...` on amd64; the median of the 5 runs is the assertion value (passes when ≤ 5 ms). The fixture is the `search_response.json` golden (25-hit Algolia response, ~6KB). Allocation count ≤ 20 per hit parsed (i.e. ≤ 500 allocs total for 25 hits) per the same benchmark's `allocs/op` field — the same floor analysis from SPEC-ADP-001 NFR-ADP-001 applies (the `pkg/types.NormalizedDoc.Metadata = map[string]any` contract from SPEC-CORE-001 forces a structural floor of ~17 allocs/doc; HN's `stripHTML` adds a few additional allocations per self-post but the average across mostly-link-post fixtures stays within 20/hit). Measured via `BenchmarkParseHits25Hits` in `internal/adapters/hn/bench_test.go`, run weekly in CI per the cadence established in SPEC-OBS-001 NFR-OBS-001. Benchmarks do not count toward coverage. |
| NFR-ADP2-002 | End-to-end Latency | The end-to-end `Search` round-trip against the `httptest.Server` stub (no real network) SHALL complete with p95 ≤ 200 ms over 100 invocations, measured by `TestSearchE2ELatencyStubP95` in `search_test.go` (sort durations ascending, assert `durations[94] ≤ 200ms`). The harder live-Algolia p95 (≤ 2s, tighter than Reddit's ≤ 5s because Algolia's CDN is faster) is documented as the operational target but is NOT enforced in CI (no live network). |
| NFR-ADP2-003 | No goroutine leak on cancellation | The adapter SHALL NOT leak any goroutine when the caller's context is cancelled mid-`Search`. Verified by `TestSearchNoGoroutineLeakOnCancel` in `search_test.go`, which uses `go.uber.org/goleak.VerifyNone(t)` after a `Search` call whose ctx is cancelled mid-flight via a 50ms-delayed cancel; assert zero residual goroutines after the call returns. |

---

## 5. Acceptance Criteria

### REQ-ADP2-001 — Adapter Interface Conformance

- File `internal/adapters/hn/hn.go` declares `Adapter` struct with the
  documented fields (`httpClient *http.Client`, `baseURL string`,
  `userAgent string`, `healthcheckTarget string`).
- The compile-time assertion `var _ types.Adapter = (*Adapter)(nil)`
  appears at the bottom of `hn.go`. If the interface ever drifts,
  this assertion fails to compile.
- `(*Adapter).Name()` returns the literal string `"hackernews"`.
- `(*Adapter).Capabilities()` returns a `types.Capabilities` with:
  - `SourceID = "hackernews"`
  - `DisplayName = "Hacker News"`
  - `DocTypes = []types.DocType{types.DocTypePost}`
  - `SupportedLangs = nil` (language-agnostic)
  - `SupportsSince = true` (HN supports `created_at_i>` filters)
  - `RequiresAuth = false`
  - `AuthEnvVars = nil`
  - `RateLimitPerMin = 60`
  - `DefaultMaxResults = 25`
  - `Notes` contains the substrings `"Algolia HN Search"`,
    `"public no-auth"`, `"tags=story"`, and
    `"self-posts use news.ycombinator.com permalink"`.
- `(*Adapter).Healthcheck(ctx)` succeeds against an httptest.Server
  bound to `127.0.0.1:0`. Tests construct the Adapter with
  `Options{HealthcheckTarget: <httptest.Server.Listener.Addr().String()>}`
  to redirect the dial target; the production default is
  `"hn.algolia.com:443"`.
- `TestAdapterName`, `TestAdapterImplementsInterface`,
  `TestCapabilitiesDeterministic`, `TestCapabilitiesShape`,
  `TestHealthcheckSucceeds` all pass.

### REQ-ADP2-002 — Search Happy Path

- `TestSearchHappyPath25Stories` against
  `testdata/search_response.json` returns exactly 25 `NormalizedDoc`
  entries (mix of link + self-posts); each passes `Validate()`
  returning nil; the captured request URL contains all 3 mandatory
  query parameters (`query`, `tags=story`, `hitsPerPage=25`).
- `TestSearchURLParametersIncludeAllRequired`,
  `TestSearchClampsHitsPerPageTo100`,
  `TestSearchDefaultsHitsPerPageTo25`,
  `TestSearchOmitsPageWhenCursorEmpty`,
  `TestSearchSetsPageWhenCursorPresent` (q.Cursor="3" → URL contains
  `&page=3`) all pass.

### REQ-ADP2-003 — HTTP 429 Rate-Limit Mapping

- `TestSearchHTTP429WithIntegerRetryAfter` asserts returned err is
  `*types.SourceError` with `Category=CategoryRateLimited`,
  `HTTPStatus=429`, `RetryAfter=30s`.
- `TestSearchHTTP429WithHTTPDateRetryAfter` parses an HTTP-date 30s
  in the future; asserts `RetryAfter` is in `(25s, 35s)`.
- `TestSearchHTTP429NoRetryAfterDefaults5s` (no header — Algolia's
  typical behaviour) asserts `RetryAfter=5s`.
- `TestSearchHTTP429RetryAfterCapped60s` (`Retry-After: 999`) asserts
  `RetryAfter=60s`.
- `TestSearchHTTP429NoInternalRetry` asserts exactly 1 request observed.

### REQ-ADP2-004 — HTTP 4xx/5xx and Network Failure Mapping

- `TestSearchHTTP4xx` table-drives 401, 403, 404; each asserts
  `errors.Is(err, types.ErrPermanent)` and matching HTTPStatus.
- `TestSearchHTTP5xx` table-drives 500, 503; each asserts
  `errors.Is(err, types.ErrSourceUnavailable)` and matching HTTPStatus.
- `TestSearchConnectionRefused` (httptest.Server closed before
  request) asserts `errors.Is(err, types.ErrSourceUnavailable)` and
  `HTTPStatus=0`.
- `TestSearchUnavailablePreservesUnderlyingError`: assert
  `errors.Unwrap(srcErr) != nil` and the inner error message contains
  "connection refused" or equivalent.

### REQ-ADP2-005 — NormalizedDoc Field Mapping

- `TestParseHitsFieldMapping` table-drives 4 fixtures (link post with
  external url, Ask HN self-post with HTML body, deleted-author post,
  story with multiple `_tags`). For each, asserts every NormalizedDoc
  field per the §6.3 mapping table (ID, SourceID, URL, Title, Body,
  Snippet, PublishedAt, RetrievedAt non-zero, Author, Score within
  `[normalizeScore(rawPoints) ± 0.001]`, Lang="", DocType=DocTypePost,
  Citations=nil, Metadata keys present).
- `TestParseHitsFiltersNonStoryTags`: fixture from
  `search_response_with_comments.json` with mixed `_tags` arrays
  containing both `"story"` and `"comment"` entries returns only the
  story hits (defensive client-side filter).
- `TestParseHitsSelfPostUsesPermalink`: fixture with `url=""` and
  `objectID="12345"` returns a doc with
  `URL == "https://news.ycombinator.com/item?id=12345"`.
- `TestParseHitsHTMLBodyStripped`: fixture with
  `story_text="<p>Hello <b>world</b></p>&amp; goodbye"` returns a doc
  with `Body == "Hello world& goodbye"` (tags stripped, entities
  decoded).
- `TestParseHitsPaginationCursor`: fixture with `page=0, nbPages=5`
  returns docs whose `[len-1].Metadata["next_cursor"] == "1"`. Earlier
  docs do NOT have the `next_cursor` key.
- `TestParseHitsNoCursorOnLastPage`: fixture with `page=4, nbPages=5`
  returns docs with no `next_cursor` key on any of them.
- `TestParseHitsHashEmpty`: every returned `NormalizedDoc.Hash`
  equals `""`.
- `TestParseHitsMetadataKeys`: each returned doc's Metadata has at
  least `{num_comments, points, tags, external_url}`.

### REQ-ADP2-006 — User-Agent and Accept Headers

- `TestSearchSetsCustomUserAgent`: captured request header
  `User-Agent` starts with `"usearch/"` and contains
  `"(+https://github.com/elymas/universal-search)"`.
- `TestSearchSetsAcceptJSON`: captured `Accept` header equals
  `"application/json"`.
- `TestSearchUserAgentVersionConfigurable`: `Options.UserAgentVersion
  = "v0.2-rc1"` → captured `User-Agent` contains `"usearch/v0.2-rc1"`.

### REQ-ADP2-007 — Numeric Filters

- `TestSearchSinceFilterAdded`: `Filters=[{since, "1700000000"}]` →
  URL has `numericFilters=created_at_i>=1700000000`.
- `TestSearchMinPointsFilterAdded`: `Filters=[{min_points, "10"}]` →
  URL has `numericFilters=points>=10`.
- `TestSearchBothFiltersJoined`:
  `Filters=[{since, "1700000000"}, {min_points, "10"}]` → URL has
  `numericFilters=created_at_i>=1700000000,points>=10`.
- `TestSearchUnknownFilterIgnored`: `Filters=[{nsfw, "true"}]` → URL
  has no `numericFilters` parameter.
- `TestSearchMalformedFilterDropped`: `Filters=[{since, "abc"}]` →
  URL has no `numericFilters` parameter; no error returned.
- `TestSearchNegativeFilterDropped`:
  `Filters=[{min_points, "-5"}]` → URL has no `numericFilters`
  parameter; no error returned.
- `TestSearchNoFilterOmitsParameter`: `Filters=nil` → URL has no
  `numericFilters` parameter.

### REQ-ADP2-008 — Empty Query and Invalid Cursor Rejection

- `TestSearchEmptyQueryRejectedNoHTTP` table-drives `q.Text` over
  `["", "   ", "\t\n  \r"]`; for each asserts
  `errors.Is(err, types.ErrPermanent)` AND
  `errors.Is(err, ErrInvalidQuery)`. The httptest.Server is
  instrumented with a request counter; assert exactly 0 requests.
- `TestSearchInvalidCursorRejectedNoHTTP` table-drives `q.Cursor`
  over `["abc", "-1", "1.5", "1e3"]`; for each asserts
  `errors.Is(err, types.ErrPermanent)` AND
  `errors.Is(err, ErrInvalidCursor)`; assert zero requests.

### REQ-ADP2-009 — Redirect Allowlist

- `TestSearchFollowsAllowlistRedirect`: server A returns 302 with
  Location header pointing to server B (Host header rewritten to
  `hn.algolia.com`); the test installs server B as a custom
  `http.RoundTripper` resolver. Assert search succeeds and returns
  the body from server B.
- `TestSearchRejectsCrossDomainRedirect`: server A returns 302 with
  Location `https://attacker.com/x`. Assert
  `errors.Is(err, types.ErrPermanent)` and error message contains
  `"cross-domain redirect"`.
- `TestSearchRejectsRedirectChainOver3`: 4 servers chained within
  the allowlist; assert error returned after 3 hops with message
  containing `"too many redirects"`.

### REQ-ADP2-010 — Concurrent Search Safety (State-Driven)

- `TestSearchConcurrentSafe`: a single `*Adapter` is constructed
  pointing at one `httptest.Server` (which records every inbound
  request). 50 goroutines are launched, each calling
  `(*Adapter).Search(ctx, q)` exactly once with the same query.
  All goroutines start via a `sync.WaitGroup` barrier so the
  invocations overlap.
- Assertions:
  1. The test executes successfully under `go test -race`; the
     race detector reports zero data-race alarms attributable to
     the adapter package.
  2. The stub server's request counter equals 50.
  3. Every goroutine receives `(docs, nil)` with `len(docs) == 25`
     (matching the standard `search_response.json` fixture); each
     returned `[]types.NormalizedDoc` slice has every doc passing
     `Validate()` returning nil.

### NFR-ADP2-001 — Parse-Path Performance

- `BenchmarkParseHits25Hits` is invoked as
  `go test -bench=BenchmarkParseHits25Hits -benchtime=10x -count=5 ./internal/adapters/hn/...`
  on amd64.
- Assertion mechanism: take the 5 reported per-op mean wall-clock
  durations (one per `-count` run); the MEDIAN of those 5 values
  SHALL be ≤ 5 ms. PASS/FAIL is decidable from the `go test -bench`
  output alone — no external CI script required.
- The bench reports `B/op` and `allocs/op`; `allocs/op` ≤ 500 (= 20 ×
  25 hits). Floor analysis: same `pkg/types.NormalizedDoc.Metadata =
  map[string]any` constraint as ADP-001 NFR-ADP-001 (see iteration 3
  HISTORY entry there); HN's `stripHTML` adds modest extra allocs on
  self-posts but the standard 25-hit fixture is mostly link posts.

### NFR-ADP2-002 — E2E p95 (Stub)

- `TestSearchE2ELatencyStubP95` runs 100 invocations against the
  stub `httptest.Server`, sorts elapsed durations, asserts
  `durations[94] ≤ 200ms`.

### NFR-ADP2-003 — Goroutine Leak Check

- `TestSearchNoGoroutineLeakOnCancel`: `goleak.VerifyNone(t)`
  succeeds after a `Search` call whose ctx was cancelled at 50ms
  while the stub server delays response by 200ms.

### Integration Checkpoint (M2 Exit Criterion)

When SPEC-CLI-001 lands and registers both the Reddit and HN adapters
into the registry, and SPEC-SYN-001 wires synthesis on top, the M2
exit criterion `usearch query "hello world"` returns Reddit + HN
results with one synthesized paragraph + citations
(`.moai/project/roadmap.md:149`) becomes achievable. ADP-002's
acceptance includes a smoke check (manual or scripted) that
`registry.Get("hackernews").Search(ctx, q)` against a stub returns
parseable `[]NormalizedDoc` interleaving with Reddit results from
ADP-001. This integration assertion lives in CLI-001's acceptance
criteria, not here, but is documented for traceability.

---

## 6. Technical Approach

### 6.1 Files to Modify

**Created (15 files)**:
- `internal/adapters/hn/hn.go` — Adapter struct, New, Name,
  Capabilities, Healthcheck, compile-time interface assertion
- `internal/adapters/hn/hn_test.go` — interface conformance tests
- `internal/adapters/hn/search.go` — Search method (the hot path),
  URL construction, filter expression building
- `internal/adapters/hn/search_test.go` — main test file (largest)
- `internal/adapters/hn/client.go` — HTTP client construction,
  doRequest, categorizeStatus
- `internal/adapters/hn/client_test.go` — error mapping + redirect
  tests
- `internal/adapters/hn/parse.go` — parseHits transform
- `internal/adapters/hn/parse_test.go` — field mapping tests
- `internal/adapters/hn/strip.go` — stripHTML helper
- `internal/adapters/hn/strip_test.go` — stripHTML table tests
- `internal/adapters/hn/score.go` — normalizeScore Tanh formula
- `internal/adapters/hn/score_test.go` — score normalization tests
- `internal/adapters/hn/errors.go` — ErrInvalidQuery /
  ErrInvalidCursor sentinels + parseRetryAfter helper
- `internal/adapters/hn/bench_test.go` — NFR-ADP2-001 benchmark
- `internal/adapters/hn/testdata/search_response.json` (~6KB)
- `internal/adapters/hn/testdata/search_response_empty.json` (~200B)
- `internal/adapters/hn/testdata/search_response_pagination.json`
  (~6KB)
- `internal/adapters/hn/testdata/search_response_self_post.json`
  (~1KB)
- `internal/adapters/hn/testdata/search_response_deleted_author.json`
  (~1KB)
- `internal/adapters/hn/testdata/search_response_with_comments.json`
  (~3KB)
- `internal/adapters/hn/testdata/search_response_malformed.json`
  (~200B)

**Modified**: none. The adapter self-contains. `pkg/types` already
publishes the contract, `internal/adapters/registry.go` already
accepts any `types.Adapter`, `internal/obs/metrics/metrics.go` already
declares `AdapterCalls` and `AdapterCallDuration` collectors with
`adapter` and `outcome` in the cardinality allowlist (the
`adapter="hackernews"` value is bounded by the V1 14-adapter ceiling
per SPEC-OBS-001 NFR-OBS-002 and ADP-001 NFR-ADP-001's analysis).

**Unchanged (by design)**: same as ADP-001 §6.1 — `registry.go`,
`pkg/types/*`, `internal/obs/metrics/metrics.go`, and
`cmd/usearch/main.go` (registry construction owned by SPEC-CLI-001).

### 6.2 Package Layout

```
internal/adapters/hn/
├── hn.go                                 # Adapter, New, Name, Capabilities, Healthcheck, interface assertion
├── hn_test.go                            # Interface conformance + Capabilities determinism
├── search.go                             # (*Adapter).Search hot path
├── search_test.go                        # E2E + happy path + error categorisation tests
├── client.go                             # *http.Client, doRequest, categorizeStatus
├── client_test.go                        # categorizeStatus table + redirect allowlist
├── parse.go                              # parseHits transform (HN Algolia envelope)
├── parse_test.go                         # Field mapping table tests
├── strip.go                              # stripHTML helper for HN story_text
├── strip_test.go                         # Tag-strip + entity-decode table tests
├── score.go                              # normalizeScore (Tanh formula, identical to ADP-001)
├── score_test.go                         # Score normalization table
├── errors.go                             # ErrInvalidQuery + ErrInvalidCursor sentinels + parseRetryAfter helper
├── bench_test.go                         # BenchmarkParseHits25Hits
└── testdata/
    ├── search_response.json              # Happy path 25 stories (mixed link/self)
    ├── search_response_empty.json        # Zero hits
    ├── search_response_pagination.json   # page=0, nbPages=5
    ├── search_response_self_post.json    # Single Ask HN with HTML story_text
    ├── search_response_deleted_author.json # Empty author
    ├── search_response_with_comments.json # Mixed _tags story+comment
    └── search_response_malformed.json    # Truncated JSON
```

[NOTE on duplication vs sharing] `parseRetryAfter`, `categorizeStatus`,
and the redirect-allowlist pattern duplicate the equivalents in
`internal/adapters/reddit/`. This duplication is INTENTIONAL in v0.1:

- The two helpers are short (≤ 30 LoC each) and the duplication cost
  is small relative to the cost of a premature shared package.
- Sharing requires choosing a home (`internal/adapters/common/`?
  `pkg/types/sourceerror/`?) and a shape (function or type-with-method?
  what about adapter-name parameter?).
- ADP-001 was the FIRST adapter; ADP-002 is the SECOND. "Rule of
  three" applies: a shared package becomes worthwhile after 3+
  consumers, not 2.
- The seven-way M3 ADP-* parallelization will produce 9 adapter
  implementations within M3. After M3 lands, a refactor SPEC
  (SPEC-ADP-REFAC-001) MAY consolidate — see Open Question §11.4.

### 6.3 HN Algolia Hit → NormalizedDoc Field Mapping

| Algolia HN Field | NormalizedDoc Field | Transform |
|------------------|---------------------|-----------|
| `objectID` | `ID` | Use as-is (string of integer, e.g., `"39458123"`) |
| (constant) | `SourceID` | `"hackernews"` (matches `Name()`) |
| `url` (when non-empty) | `URL` | Use as-is |
| (when `url == ""`) | `URL` | `"https://news.ycombinator.com/item?id=" + objectID` (canonical permalink) |
| `title` | `Title` | Use as-is (HN-style prefixes like `"Show HN: "` preserved) |
| `stripHTML(story_text)` (empty for link posts) | `Body` | Apply `stripHTML` to decode entities and remove tags |
| `stripHTML(story_text)` truncated to 280 runes; falls back to `truncateRunes(title, 280)` when body empty | `Snippet` | Same truncation discipline as ADP-001 |
| `time.Unix(int64(created_at_i), 0).UTC()` | `PublishedAt` | Algolia provides `created_at_i` as Unix-seconds int |
| (parse time) | `RetrievedAt` | `time.Now().UTC()` (set by `parseHits` caller) |
| `author` | `Author` | Use as-is (may be `""` for deleted users) |
| `normalizeScore(int(points))` per §2.3 | `Score` | Tanh formula identical to ADP-001 |
| (constant) | `Lang` | `""` (HN has no per-item language field) |
| (constant) | `DocType` | `types.DocTypePost` |
| (nil) | `Citations` | `nil` |
| (constructed) | `Metadata` | Map containing two key tiers. **REQUIRED keys**: `num_comments` (int), `points` (int — surfaced for consumers wanting pre-normalisation), `tags` ([]string filtered to non-internal entries — e.g., `["story", "front_page"]`), `external_url` (= `url`; empty when self-post). REQ-ADP2-005 enforces these 4 as the contractual minimum. **OPTIONAL keys**: `comment_text` (only when hit is a comment, which v0.1 filters out — documented for forward-compat), `parent_id` (story this hit belongs to, when applicable). The LAST returned doc additionally gets `next_cursor` (REQUIRED on the last doc only, when `currentPage + 1 < nbPages`) encoded as `strconv.Itoa(currentPage + 1)`. |
| (constant) | `Hash` | `""` (consumers compute via `CanonicalHash()`) |

### 6.4 Type Sketches (illustrative; final shapes in run phase)

```go
// internal/adapters/hn/hn.go
package hn

import (
    "context"
    "fmt"
    "net"
    "net/http"

    "github.com/elymas/universal-search/pkg/types"
)

const (
    defaultBaseURL           = "https://hn.algolia.com/api/v1/search"
    defaultUserAgentTemplate = "usearch/%s (+https://github.com/elymas/universal-search)"
    defaultUAVersion         = "v0.1"
    defaultHealthcheckTarget = "hn.algolia.com:443"
)

type Options struct {
    BaseURL           string        // default: defaultBaseURL (test override)
    HTTPClient        *http.Client  // default: 10s timeout, allowlist redirect, reqid transport
    UserAgentVersion  string        // default: "v0.1"
    HealthcheckTarget string        // default: "hn.algolia.com:443"
}

type Adapter struct {
    httpClient        *http.Client
    baseURL           string
    userAgent         string
    healthcheckTarget string
}

func New(opts Options) (*Adapter, error) {
    base := opts.BaseURL
    if base == "" {
        base = defaultBaseURL
    }
    version := opts.UserAgentVersion
    if version == "" {
        version = defaultUAVersion
    }
    ua := fmt.Sprintf(defaultUserAgentTemplate, version)
    client := opts.HTTPClient
    if client == nil {
        client = newDefaultClient()
    }
    target := opts.HealthcheckTarget
    if target == "" {
        target = defaultHealthcheckTarget
    }
    return &Adapter{
        httpClient:        client,
        baseURL:           base,
        userAgent:         ua,
        healthcheckTarget: target,
    }, nil
}

func (a *Adapter) Name() string { return "hackernews" }

func (a *Adapter) Capabilities() types.Capabilities {
    return types.Capabilities{
        SourceID:          "hackernews",
        DisplayName:       "Hacker News",
        DocTypes:          []types.DocType{types.DocTypePost},
        SupportedLangs:    nil,
        SupportsSince:     true, // numericFilters=created_at_i>=...
        RequiresAuth:      false,
        AuthEnvVars:       nil,
        RateLimitPerMin:   60,
        DefaultMaxResults: 25,
        Notes: "Algolia HN Search public no-auth endpoint " +
            "(https://hn.algolia.com/api). Stories only (tags=story " +
            "hardcoded; comments / polls deferred). Self-posts use " +
            "news.ycombinator.com permalink as URL. Body and Snippet " +
            "are HTML-stripped from story_text. Filter keys: 'since' " +
            "(Unix seconds, maps to created_at_i>=) and 'min_points' " +
            "(integer, maps to points>=).",
    }
}

func (a *Adapter) Healthcheck(ctx context.Context) error {
    var d net.Dialer
    conn, err := d.DialContext(ctx, "tcp", a.healthcheckTarget)
    if err != nil {
        return err
    }
    return conn.Close()
}

var _ types.Adapter = (*Adapter)(nil)
```

```go
// internal/adapters/hn/client.go (excerpt)
var allowedRedirectHosts = map[string]struct{}{
    "hn.algolia.com":      {},
    "news.ycombinator.com": {},
}

func redirectAllowlist(req *http.Request, via []*http.Request) error {
    if len(via) >= 3 {
        return errors.New("hn: too many redirects (max 3)")
    }
    host := req.URL.Hostname()
    if _, ok := allowedRedirectHosts[host]; !ok {
        return fmt.Errorf("hn: cross-domain redirect rejected: %s", host)
    }
    return nil
}

func categorizeStatus(status int, retryAfter time.Duration, cause error) *types.SourceError {
    se := &types.SourceError{Adapter: "hackernews", HTTPStatus: status, Cause: cause}
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

```go
// internal/adapters/hn/parse.go (excerpt)
type algoliaResponse struct {
    Hits        []algoliaHit `json:"hits"`
    NbHits      int          `json:"nbHits"`
    Page        int          `json:"page"`
    NbPages     int          `json:"nbPages"`
    HitsPerPage int          `json:"hitsPerPage"`
}

type algoliaHit struct {
    ObjectID    string   `json:"objectID"`
    Title       string   `json:"title"`
    URL         string   `json:"url"`
    Author      string   `json:"author"`
    Points      int      `json:"points"`
    StoryText   string   `json:"story_text"`
    NumComments int      `json:"num_comments"`
    CreatedAtI  int64    `json:"created_at_i"`
    Tags        []string `json:"_tags"`
    // _highlightResult intentionally omitted — IGNORED.
}

// parseHits returns (docs, nextCursor, error). nextCursor is "" when no
// further pages exist; otherwise strconv.Itoa(currentPage + 1).
func parseHits(body []byte, retrievedAt time.Time) ([]types.NormalizedDoc, string, error) {
    var resp algoliaResponse
    if err := json.Unmarshal(body, &resp); err != nil {
        return nil, "", &types.SourceError{
            Adapter:  "hackernews",
            Category: types.CategoryPermanent,
            Cause:    fmt.Errorf("hn: malformed JSON response: %w", err),
        }
    }
    // ... filter non-story, transform, set next_cursor on last doc
}
```

### 6.5 HTTP Client Construction Notes

Identical to ADP-001 §6.5:

- **Timeout**: 10 seconds total request deadline (default). Caller's
  ctx deadline takes precedence.
- **Redirect policy**: `CheckRedirect` enforces the allowlist
  `{hn.algolia.com, news.ycombinator.com}` and caps at 3 hops.
- **Transport**: `reqid.NewTransport(http.DefaultTransport)` for
  request-ID propagation.
- **Headers per request**: `User-Agent: usearch/<version>
  (+https://github.com/elymas/universal-search)` and
  `Accept: application/json`. NO authentication header.

### 6.6 Observability Note

The HN adapter, like the Reddit adapter, emits ZERO metrics, logs,
and spans of its own. ALL observability comes from the registry's
`wrappedAdapter` (`internal/adapters/registry.go:172-263`). This is
the sole-emitter discipline established in SPEC-CORE-001 §6.5 and
preserved verbatim by SPEC-ADP-001. The adapter's responsibility is
to return a correctly-categorised `*types.SourceError` so the
wrappedAdapter computes the right `outcome` label via
`types.OutcomeFromError(err)`.

### 6.7 MX Tag Plan

| File::Symbol | Tag | Reason |
|--------------|-----|--------|
| `hn.go::(*Adapter).Search` (defined in `search.go`) | `@MX:ANCHOR` | Sole entry point for all HN fanout calls. fan_in ≥ 3 (registry wrappedAdapter, FAN-001 fanout, tests). `@MX:REASON: contract boundary; signature change ripples to FAN-001 + CLI-001 + SYN-001`. |
| `parse.go::parseHits` | `@MX:ANCHOR` | Every HN hit passes through this single transform. fan_in = 1 (Search) but invariant-bearing — bug here corrupts every NormalizedDoc returned. `@MX:REASON: NormalizedDoc field-mapping integrity gate`. |
| `score.go::normalizeScore` (function) and constants `tanhDivisor=100.0, scoreCenter=0.5` | `@MX:NOTE` | Documents the Tanh formula choice and tie-in to SPEC-IDX-001 RRF. Same shape as ADP-001 score.go. |
| `client.go::categorizeStatus` | `@MX:NOTE` | The HTTP-status-to-Category rosetta. Future contributors will look here first when a new HTTP code needs handling. |
| `client.go::doRequest` | `@MX:WARN` | Outbound network call. Redirect allowlist enforces SSRF safety boundary. `@MX:REASON: removing the CheckRedirect guard re-opens SSRF`. |
| `client.go::allowedRedirectHosts` map | `@MX:NOTE` | The 2-entry redirect allowlist. Adding a host requires a security review. |
| `strip.go::stripHTML` | `@MX:NOTE` | Conservative stdlib-only HTML-strip. NOT a security boundary (output never rendered as HTML). Adding a third HN body markup pattern requires updating the test fixture set first. |

All tags are `[AUTO]`-prefixed (agent-generated), include
`@MX:SPEC: SPEC-ADP-002`, and follow `code_comments: en` per
`.moai/config/sections/language.yaml`.

### 6.8 Harness Level

Per `.moai/config/sections/harness.yaml` auto-routing: 10 EARS REQs
(8 × P0 + 2 × P1) + 3 NFRs touching 1 package (8 source files +
7 testdata fixtures) + zero cross-package edits + zero security/
payment/PII keywords + zero compose/env/config deltas =
**standard** harness level. Sprint Contract is OPTIONAL but
recommended. Evaluator profile `default` applies.

---

## 7. What NOT to Build (Exclusions)

[HARD] This SPEC explicitly excludes the following items. Each has a
known destination SPEC; this list prevents scope creep into ADP-002
and reaffirms the second-adapter discipline.

- **Per-source customisations for arXiv, GitHub, YouTube, Bluesky, X,
  SearXNG, Naver, Daum, KoreaNewsCrawler, RSS, Polymarket** →
  SPEC-ADP-003..009 (M3).
- **Retry orchestration** (exponential backoff, circuit breaker,
  per-source health-state tracking, jitter) → SPEC-FAN-001 (M3).
  Adapter is one-shot per call.
- **Response caching** (in-process LRU, Redis, on-disk fixture cache)
  → SPEC-CACHE-001 (M3). Adapter is stateless.
- **Result ranking, deduplication, RRF fusion across adapters** →
  SPEC-IDX-001 (M3). Adapter returns Algolia-relevance order with the
  Tanh-normalised Score; cross-adapter ranking is fusion's job.
- **HN comment, poll, pollopt retrieval** → out of v0.1 scope; v0.1
  hardcodes `tags=story`. A future SPEC-ADP-002a may add a
  `Query.Filters[Key="kind"]` switch.
- **`search_by_date` mode** (strict reverse-chronological) → out of
  v0.1; default relevance only. Future P2 enhancement post-M3.
- **Robust HTML parsing via `golang.org/x/net/html`** → out of v0.1.
  The stdlib-only `stripHTML` helper is sufficient for HN's shallow
  body markup.
- **Cross-adapter helper extraction** (sharing `parseRetryAfter`,
  `categorizeStatus`, redirect allowlist between Reddit and HN
  packages) → out of v0.1. See §11.4. Refactor SPEC after M3.
- **Sort customisation** (Algolia's built-in sort options) → out of
  v0.1; hardcoded relevance.
- **Live network integration tests in CI** → out of v0.1; httptest
  + golden fixtures only.
- **Per-adapter custom Prometheus metrics** (e.g.,
  `hn_pagination_pages_total`) → would require amending
  SPEC-OBS-001's allowlist. Out of v0.1.
- **Korean-locale handling for HN** → SPEC-IDX-003 (M3); HN returns
  Lang="" (unknown).
- **Streaming Search results** (channel-based incremental delivery)
  → SPEC-SYN-004 (M4) if measured value.
- **HN highlight-result rendering** (`_highlightResult` Algolia
  field with `<em>` markup) → IGNORED by parser; never surfaces to
  consumers.
- **Author-specific filtering** (Algolia's `tags=author_<name>`
  filter) → out of v0.1; v0.1 filters key is `since` or `min_points`
  only.

---

## 8. TDD Plan

Development follows RED-GREEN-REFACTOR per
`quality.development_mode: tdd` (`.moai/config/sections/quality.yaml`).
Representative RED-phase tests, written before implementation,
grouped by REQ. Total: ~38 tests covering REQ-ADP2-001..010 + NFRs.
Coverage target: 85% per `quality.test_coverage_target`. Benchmarks
do not count toward coverage.

| # | Test | Layer | REQ | Assertion |
|---|------|-------|-----|-----------|
| 1 | `TestAdapterName` | `hn_test.go` | REQ-ADP2-001 | `(*Adapter).Name() == "hackernews"` |
| 2 | `TestAdapterImplementsInterface` | `hn_test.go` | REQ-ADP2-001 | Compile-time `var _ types.Adapter = (*Adapter)(nil)` succeeds |
| 3 | `TestCapabilitiesDeterministic` | `hn_test.go` | REQ-ADP2-001 | Two consecutive `Capabilities()` calls return `reflect.DeepEqual` results |
| 4 | `TestCapabilitiesShape` | `hn_test.go` | REQ-ADP2-001 | All 9 documented field values match (SourceID, DisplayName, DocTypes, RequiresAuth, AuthEnvVars, SupportsSince=true, RateLimitPerMin=60, DefaultMaxResults=25, plus Notes substring contains) |
| 5 | `TestHealthcheckSucceeds` | `hn_test.go` | REQ-ADP2-001 | TCP dial against test loopback succeeds |
| 6 | `TestSearchHappyPath25Stories` | `search_test.go` | REQ-ADP2-002, REQ-ADP2-005 | 25 NormalizedDocs returned; each `Validate()` returns nil |
| 7 | `TestSearchURLParametersIncludeAllRequired` | `search_test.go` | REQ-ADP2-002 | Captured request URL has `query`, `tags=story`, `hitsPerPage` |
| 8 | `TestSearchClampsHitsPerPageTo100` | `search_test.go` | REQ-ADP2-002 | q.MaxResults=500 → URL has `hitsPerPage=100` |
| 9 | `TestSearchDefaultsHitsPerPageTo25` | `search_test.go` | REQ-ADP2-002 | q.MaxResults=0 → URL has `hitsPerPage=25` |
| 10 | `TestSearchOmitsPageWhenCursorEmpty` | `search_test.go` | REQ-ADP2-002 | q.Cursor="" → URL has no `page` param |
| 11 | `TestSearchSetsPageWhenCursorPresent` | `search_test.go` | REQ-ADP2-002 | q.Cursor="3" → URL contains `&page=3` |
| 12 | `TestSearchHTTP429WithIntegerRetryAfter` | `search_test.go` | REQ-ADP2-003 | `Retry-After: 30` → SourceError.RetryAfter==30s |
| 13 | `TestSearchHTTP429WithHTTPDateRetryAfter` | `search_test.go` | REQ-ADP2-003 | HTTP-date 30s ahead → RetryAfter ∈ (25s, 35s) |
| 14 | `TestSearchHTTP429NoRetryAfterDefaults5s` | `search_test.go` | REQ-ADP2-003 | No header → RetryAfter==5s (Algolia's typical case) |
| 15 | `TestSearchHTTP429RetryAfterCapped60s` | `search_test.go` | REQ-ADP2-003 | `Retry-After: 999` → RetryAfter==60s |
| 16 | `TestSearchHTTP429NoInternalRetry` | `search_test.go` | REQ-ADP2-003 | Server request count == 1 |
| 17 | `TestSearchHTTP4xx` | `search_test.go` | REQ-ADP2-004 | Table over 401/403/404 → ErrPermanent + matching HTTPStatus |
| 18 | `TestSearchHTTP5xx` | `search_test.go` | REQ-ADP2-004 | Table over 500/503 → ErrSourceUnavailable + matching HTTPStatus |
| 19 | `TestSearchConnectionRefused` | `search_test.go` | REQ-ADP2-004 | `errors.Is(err, types.ErrSourceUnavailable)`; HTTPStatus==0 |
| 20 | `TestSearchUnavailablePreservesUnderlyingError` | `search_test.go` | REQ-ADP2-004 | `errors.Unwrap(srcErr).Error()` contains inner cause text |
| 21 | `TestParseHitsFieldMapping` | `parse_test.go` | REQ-ADP2-005 | Table over 4 fixtures; every documented field maps correctly |
| 22 | `TestParseHitsFiltersNonStoryTags` | `parse_test.go` | REQ-ADP2-005 | Mixed _tags story+comment → only story hits returned |
| 23 | `TestParseHitsSelfPostUsesPermalink` | `parse_test.go` | REQ-ADP2-005 | url="" + objectID="12345" → URL = "https://news.ycombinator.com/item?id=12345" |
| 24 | `TestParseHitsHTMLBodyStripped` | `parse_test.go` | REQ-ADP2-005 | story_text with HTML → Body has tags stripped, entities decoded |
| 25 | `TestParseHitsPaginationCursor` | `parse_test.go` | REQ-ADP2-005 | page=0, nbPages=5 → last doc Metadata["next_cursor"]=="1" |
| 26 | `TestParseHitsNoCursorOnLastPage` | `parse_test.go` | REQ-ADP2-005 | page=4, nbPages=5 → no doc has `next_cursor` key |
| 27 | `TestParseHitsHashEmpty` | `parse_test.go` | REQ-ADP2-005 | Every NormalizedDoc.Hash == "" |
| 28 | `TestParseHitsMetadataKeys` | `parse_test.go` | REQ-ADP2-005 | All 4 required Metadata keys present (num_comments, points, tags, external_url) |
| 29 | `TestParseHitsDeletedAuthor` | `parse_test.go` | REQ-ADP2-005 | author="" returned as-is; Validate() still passes |
| 30 | `TestParseHitsMalformedJSON` | `parse_test.go` | REQ-ADP2-005 | Truncated JSON → `*SourceError{Category: CategoryPermanent}` |
| 31 | `TestStripHTMLTable` | `strip_test.go` | REQ-ADP2-005 | Table over 8 inputs (empty, plain, single tag, nested tags, malformed, entities, mixed, very long) |
| 32 | `TestSearchSetsCustomUserAgent` | `client_test.go` | REQ-ADP2-006 | UA starts with "usearch/" + contains URL |
| 33 | `TestSearchSetsAcceptJSON` | `client_test.go` | REQ-ADP2-006 | `Accept: application/json` header present |
| 34 | `TestSearchUserAgentVersionConfigurable` | `client_test.go` | REQ-ADP2-006 | Options override propagates to UA header |
| 35 | `TestSearchSinceFilterAdded` | `search_test.go` | REQ-ADP2-007 | Filters=[{since,"1700000000"}] → URL has `numericFilters=created_at_i>=1700000000` |
| 36 | `TestSearchMinPointsFilterAdded` | `search_test.go` | REQ-ADP2-007 | Filters=[{min_points,"10"}] → URL has `numericFilters=points>=10` |
| 37 | `TestSearchBothFiltersJoined` | `search_test.go` | REQ-ADP2-007 | Two filters → comma-joined |
| 38 | `TestSearchUnknownFilterIgnored` | `search_test.go` | REQ-ADP2-007 | Unknown key → no numericFilters param |
| 39 | `TestSearchMalformedFilterDropped` | `search_test.go` | REQ-ADP2-007 | Non-numeric value → no numericFilters param |
| 40 | `TestSearchNegativeFilterDropped` | `search_test.go` | REQ-ADP2-007 | Negative value → no numericFilters param |
| 41 | `TestSearchNoFilterOmitsParameter` | `search_test.go` | REQ-ADP2-007 | Filters=nil → no numericFilters param |
| 42 | `TestSearchEmptyQueryRejectedNoHTTP` | `search_test.go` | REQ-ADP2-008 | Table over empty/whitespace q.Text → ErrPermanent + zero requests |
| 43 | `TestSearchInvalidCursorRejectedNoHTTP` | `search_test.go` | REQ-ADP2-008 | Table over invalid cursors → ErrPermanent + zero requests |
| 44 | `TestSearchFollowsAllowlistRedirect` | `client_test.go` | REQ-ADP2-009 | 302 within allowlist followed |
| 45 | `TestSearchRejectsCrossDomainRedirect` | `client_test.go` | REQ-ADP2-009 | 302 to attacker.com → ErrPermanent + "cross-domain redirect" message |
| 46 | `TestSearchRejectsRedirectChainOver3` | `client_test.go` | REQ-ADP2-009 | 4-hop chain rejected with "too many redirects" |
| 47 | `TestNormalizeScoreTable` | `score_test.go` | REQ-ADP2-005 | 7 score values → expected `[0,1]` outputs within ±0.001 |
| 48 | `TestNormalizeScoreDeterministic` | `score_test.go` | REQ-ADP2-005 | Two calls on same input return byte-equal output |
| 49 | `TestParseRetryAfterTable` | `client_test.go` | REQ-ADP2-003 | Table over 6 inputs (int, HTTP-date, missing, malformed, > 60, negative) |
| 50 | `TestCategorizeStatusTable` | `client_test.go` | REQ-ADP2-003/004 | Truth table over 7 status codes (200/401/403/404/429/500/503/0) → expected Category |
| 51 | `TestSearchE2ELatencyStubP95` | `search_test.go` | NFR-ADP2-002 | 100 invocations against stub; p95 ≤ 200ms |
| 52 | `TestSearchNoGoroutineLeakOnCancel` | `search_test.go` | NFR-ADP2-003 | `goleak.VerifyNone(t)` after mid-flight ctx cancel |
| 53 | `BenchmarkParseHits25Hits` | `bench_test.go` | NFR-ADP2-001 | Median of 5 `-count` runs at `-benchtime=10x` is ≤ 5ms per op; allocs/op ≤ 500 |
| 54 | `TestSearchConcurrentSafe` | `search_test.go` | REQ-ADP2-010 | 50 goroutines call Search on shared `*Adapter` against one stub; race-detector clean (`-race`); stub observes 50 requests; every goroutine receives 25 valid `NormalizedDoc`s |

RED-GREEN-REFACTOR per requirement:

1. RED: Write failing test for REQ-ADP2-N.
2. GREEN: Implement minimal code to pass.
3. REFACTOR: Tidy; extract shared helpers if they remove duplication
   WITHIN the package; keep file sizes manageable (target each `.go`
   file < 200 LoC excluding tests).

Greenfield note: `internal/adapters/hn/` does not exist. There is no
behaviour to preserve; no characterization tests needed.

---

## 9. Dependencies

### 9.1 Upstream SPEC Dependencies

- **SPEC-CORE-001 (implemented; merged commit f728aa2)**: provides
  `pkg/types.Adapter`, `pkg/types.Capabilities`, `pkg/types.Query`,
  `pkg/types.NormalizedDoc`, `*types.SourceError`,
  `types.OutcomeFromError`, `types.DocType` enum,
  `internal/adapters.Registry` with wrappedAdapter sole-emitter
  pattern, `internal/adapters/noop` reference shape. HARD dep.
- **SPEC-ADP-001 (implemented; merged commit 41372d4 + e3d1f7d +
  b2c2c53)**: provides the reference adapter shape that ADP-002
  copies — file layout, error mapping discipline, MX tag plan, TDD
  harness, Tanh score normaliser, `parseRetryAfter` pattern,
  `categorizeStatus` pattern, redirect allowlist pattern. HARD dep
  on the file-layout convention, NOT on the package contents (HN's
  package is independent).
- **SPEC-OBS-001 (implemented)**: provides `obs.Obs` bundle,
  `internal/obs/reqid.NewTransport` for request-ID propagation,
  `AdapterCalls{adapter,outcome}` and `AdapterCallDuration{adapter}`
  collectors. SOFT dep — adapter is nil-safe via the registry's
  nil-guards. The `adapter="hackernews"` cardinality value fits
  within the V1 14-adapter ceiling.
- **SPEC-IR-001 (implemented; merged commit 8a20b68)**: documents
  the consumer contract for `Capabilities` (REQ-IR-008 selects
  AdapterSet by intersecting `categoryEligibleDocTypes` with
  `SupportedLangs`). ADP-002's `Capabilities()` shape (DocTypes,
  SupportedLangs=nil) determines which routing categories the HN
  adapter will be selected for. SOFT dep.

### 9.2 Parallelizable

- **SPEC-CLI-001 (M2)**: can plan in parallel; CLI-001 wires both
  Reddit and HN adapters into `cmd/usearch/main.go` registry
  construction. Depends on ADP-002's `New(opts) (*Adapter, error)`
  constructor signature being approved.
- **SPEC-SYN-001 (M2)**: can plan in parallel; synthesis consumes
  `[]types.NormalizedDoc` shape (already locked in CORE-001), so
  ADP-002 doesn't add new constraints.

### 9.3 Downstream Blocked SPECs

- **SPEC-FAN-001** (M3): consumes `(*hn.Adapter).Search` via
  `registry.Get("hackernews").Search(ctx, q)` and orchestrates retry
  on `errors.Is(err, types.ErrTransient)` /
  `errors.Is(err, types.ErrRateLimited)`. With two adapters available,
  fanout testing gets a real workout.
- **SPEC-CLI-001** (M2): wires the adapter into the M2 exit-criterion
  demonstration.
- **SPEC-SYN-001** (M2): consumes `[]NormalizedDoc` for citation
  assembly via the gpt-researcher Python sidecar.
- **SPEC-IDX-001** (M3): consumes `NormalizedDoc.Score` (Tanh-normalised
  in ADP-002) as one input to RRF fusion across adapters.

### 9.4 External Dependencies (run-phase pins)

**Zero new Go module dependencies.** ADP-002 uses only:
- Go stdlib: `context`, `encoding/json`, `errors`, `fmt`, `io`,
  `math`, `net`, `net/http`, `net/url`, `strconv`, `strings`, `time`,
  `unicode`, `unicode/utf8`
- `pkg/types` (already pinned via SPEC-CORE-001)
- `internal/obs/reqid` (already pinned via SPEC-OBS-001)
- Test-only: `go.uber.org/goleak` (for NFR-ADP2-003) — already added
  by SPEC-ADP-001 run-phase.

---

## 10. Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Algolia API contract drift (new field added, existing field renamed) | Low | Medium | `encoding/json` tolerates unknown fields; test fixtures pinned to documented shape. SearXNG's HN engine has been stable against this API for 5+ years. |
| `stripHTML` helper breaks on adversarial input (script tags, malformed tags, nested entities) | Medium | Low | Strip is conservative (replaces ALL tag-bracket pairs with empty); does NOT execute as HTML; passes raw-text safety test. Tested via `TestStripHTMLTable` over 8 input shapes. NOT a security boundary — output never rendered as HTML. |
| Two-branch URL construction (link vs self-post) creates inconsistent canonical URLs that downstream dedup confuses | Low | Medium | `CanonicalHash` includes `SourceID` prefix per `pkg/types/normalized_doc.go:96-99` — Reddit and HN cannot cross-collide. Within HN, a self-post and a link post have DIFFERENT URLs by construction (permalink vs external). Documented in §6.3 mapping table. |
| HN's "Show HN" / "Ask HN" titles include category prefix (e.g., `"Show HN: My new tool"`) | Medium | Low | Prefix preserved as-is in `Title`. Consumers (synthesis layer) decide whether to strip — out of adapter scope. |
| HN self-posts have rich body but link posts have empty body — synthesis quality varies | Medium | Low | `Body` reflects what HN provides; downstream synthesis (SPEC-SYN-001) may fall back to `Title + Snippet` when `Body == ""`. Same risk as ADP-001 link posts. |
| Cursor parsing fails on adversarial input (huge integers, negative, non-numeric) | Low | Low | `strconv.Atoi` catches non-numeric; explicit guard against negative; REQ-ADP2-008 rejects with ErrInvalidCursor. Empty cursor is valid (page 0). Excessive page (e.g., 10000) is sent to Algolia, which gracefully returns empty hits. |
| Hash collisions across Reddit and HN for same shared external URL (e.g., a Reddit submission linking to the same article HN linked to) | Low | Medium | `CanonicalHash` includes `SourceID` prefix — Reddit and HN cannot collide. Tested in `pkg/types/normalized_doc_test.go::TestCanonicalHashIncludesSourceID`. |
| `_tags` filter is case-sensitive; an Algolia drift to `"Story"` (capital S) silently filters all results | Low | Medium | Filter literal match for `"story"` (lowercase). If Algolia drifts, the parser returns empty docs (not erroneous docs). Open Question §11.6 documents revisit. |
| Score normalization formula impacts SPEC-IDX-001 RRF behaviour | Medium | Medium | Formula identical to ADP-001 §2.3 — same risk, same mitigation. RRF in SPEC-IDX-001 weights rank not raw score. |
| Pagination cursor opacity confuses downstream consumers | Low | Low | REQ-ADP2-005 surfaces the cursor via `Metadata["next_cursor"]` on the LAST doc. Cursor is conceptually opaque (string); consumers MUST pass it back as `Query.Cursor` without parsing. Documented in `Capabilities.Notes`. |
| `time.Now()` in `RetrievedAt` non-deterministic in tests | Low | Low | `parseHits` accepts `retrievedAt time.Time` parameter; tests inject a fixed time. Search wraps with `time.Now().UTC()` in production. |
| Algolia returns extra fields not in v0.1 schema, causing test fixtures to drift | Low | Low | Test fixtures are static. The JSON parser ignores unknown fields. |
| HTTP timeout (10s) too aggressive for Algolia HN during incidents | Low | Low | Configurable via `Options.HTTPClient`; default 10s aligns with NFR-ADP2-002 stub p95 200ms × 50× safety margin. |
| Duplication of helpers with reddit package introduces drift over time | Medium | Low | Acknowledged. See §11.4 — refactor SPEC after M3 lands the next 3+ adapters. v0.1 acceptance includes a manual diff check that `parseRetryAfter` and `categorizeStatus` shapes match across the two packages. |

---

## 11. Open Questions

These are explicitly unresolved at SPEC-approval time. Each has a
recommended default and a one-line resolution owner. They do NOT
block SPEC approval.

1. **HTML-strip implementation depth** (stdlib-only tag-strip vs
   adopting `golang.org/x/net/html`). **Recommended default**:
   stdlib-only; HN body markup is shallow (`<p>`, `<a>`, `<i>`,
   `<br>`, `<code>`, `<pre>` typically). Revisit if real-world HN
   bodies produce stripping bugs.
   **Resolution owner**: run-phase implementer; SPEC-SYN-001 may
   request a richer text pipeline if needed.

2. **Sort modes** (hardcode `search` (relevance) or expose a
   `Query.Filters[Key="sort"]` switch routing to `search_by_date`).
   **Recommended default**: hardcode relevance in v0.1; add
   `sort=date` filter as a P2 enhancement when SPEC-IDX-001 RRF
   integration measures the value.
   **Resolution owner**: SPEC-IDX-001 author.

3. **`hitsPerPage` clamp ceiling** (Algolia allows up to 1000;
   ADP-001 clamps at 100). Should ADP-002 also clamp at 100 or
   honour Algolia's 1000 ceiling?
   **Recommended default**: clamp at 100 to match ADP-001 (uniform
   adapter behaviour); the per-page bandwidth saving is small.
   **Resolution owner**: SPEC-FAN-001 author may revisit.

4. **Cross-adapter helper extraction** (sharing `parseRetryAfter`,
   `categorizeStatus`, redirect allowlist between Reddit and HN
   packages). **Recommended default**: defer until M3 lands 3+
   more adapters (rule of three). At that point, a SPEC-ADP-REFAC-001
   may consolidate. v0.1 ships duplicate helpers in each adapter
   package.
   **Resolution owner**: SPEC-ADP-REFAC-001 author (TBD post-M3).

5. **HN-specific score calibration** (Tanh divisor=100, center=0.5
   inherited from ADP-001). Should HN tune the divisor differently
   given that HN's `points` distribution top-end is ~6000 vs
   Reddit's effectively unbounded `score`?
   **Recommended default**: keep identical formula in v0.1;
   normalisation provides a `[0,1]` codomain regardless. Revisit
   after SPEC-IDX-001 RRF integration measurements.
   **Resolution owner**: SPEC-IDX-001 author.

6. **`_tags` filter case-sensitivity** (literal `"story"` lowercase
   match). If Algolia ever drifts the tag string casing, the parser
   silently returns empty results.
   **Recommended default**: lowercase literal match; document in
   `Capabilities.Notes`. If observed, switch to case-insensitive
   `strings.EqualFold` per element.
   **Resolution owner**: run-phase implementer; revisit if
   SPEC-EVAL-002 reliability dashboard shows HN result count
   anomalies.

---

## 12. References

### External (URL-cited; verified per research.md §8)

- https://hn.algolia.com/api — Algolia HN Search API documentation
  (canonical reference; the docs are concise and no OpenAPI schema
  is published).
- https://github.com/searxng/searxng/blob/master/searx/engines/hackernews.py
  — SearXNG HN engine implementation; pattern reference only (NOT a
  Go dependency).
- https://news.ycombinator.com/item?id=<id> — HN canonical permalink
  format used for self-post `URL` construction.
- RFC 7231 §7.1.3 Retry-After header semantics — basis for
  REQ-ADP2-003 parser (inherited from ADP-001).

### Internal (file:line cited)

- `.moai/specs/SPEC-ADP-002/research.md` — full research artifact
  for this SPEC.
- `.moai/specs/SPEC-ADP-001/spec.md` — reference adapter SPEC; this
  SPEC inherits structure verbatim.
- `.moai/specs/SPEC-CORE-001/spec.md` — Adapter / Capabilities /
  Query / NormalizedDoc / SourceError contract.
- `.moai/specs/SPEC-OBS-001/spec.md` — observability bundle and
  cardinality discipline.
- `.moai/specs/SPEC-IR-001/spec.md` — `Capabilities` consumer
  contract (REQ-IR-008).
- `pkg/types/adapter.go` — Adapter interface 4-method shape.
- `pkg/types/capabilities.go:38-62` — Capabilities struct + DocType
  enum.
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
- `internal/adapters/reddit/reddit.go:1-136` — Adapter struct
  pattern (mirrored by ADP-002 hn.go).
- `internal/adapters/reddit/search.go:1-167` — Search hot path
  pattern (mirrored by ADP-002 search.go).
- `internal/adapters/reddit/parse.go:1-203` — parseListing pattern
  (HN parseHits is the equivalent).
- `internal/adapters/reddit/client.go:1-125` — HTTP client +
  redirect allowlist pattern.
- `internal/adapters/reddit/score.go:1-41` — Tanh score formula
  (duplicated verbatim in ADP-002 score.go).
- `internal/adapters/reddit/errors.go:1-64` — parseRetryAfter helper
  pattern (duplicated verbatim).
- `internal/llm/client.go:31-65` — HTTP client construction pattern
  with timeout + reqid Transport wrapping.
- `.moai/project/roadmap.md:39, 122, 149` — M2 row, parallelization
  plan, exit criterion.
- `.moai/project/structure.md:18-22` — `internal/adapters/hn/`
  reservation.
- `.moai/project/tech.md:108` — Hacker News row
  ("Algolia HN API ... generous ... stable, no-auth").
- `.moai/config/sections/quality.yaml` — `development_mode: tdd`,
  `test_coverage_target: 85`.
- `.moai/config/sections/harness.yaml` — auto-routing to standard
  level.
- `.moai/config/sections/language.yaml` — `documentation: en`,
  `code_comments: en`.

---

*End of SPEC-ADP-002 v0.1 (DRAFT)*
