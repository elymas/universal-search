---
id: SPEC-ADP-003
title: arXiv + Paper Search Adapter
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

# SPEC-ADP-003: arXiv + Paper Search Adapter

## HISTORY

- 2026-05-04 (initial draft v0.1, limbowl via manager-spec):
  First M3 adapter SPEC drafted following the SPEC-ADP-001 / SPEC-ADP-002
  reference shape verbatim. Scope and contracts derived from
  `.moai/specs/SPEC-ADP-003/research.md` (every external claim
  WebFetch-verified at §9; every internal claim file:line-cited). Built
  on SPEC-CORE-001 (`pkg/types.Adapter` 4-method interface,
  `pkg/types.Capabilities` descriptor, `pkg/types.Query`,
  `pkg/types.NormalizedDoc` 15-field struct including `DocTypePaper` at
  `pkg/types/capabilities.go:17`, `*types.SourceError` taxonomy, registry
  wrappedAdapter sole-emitter pattern at
  `internal/adapters/registry.go:172-263`), SPEC-OBS-001
  (`AdapterCalls{adapter,outcome}` + `AdapterCallDuration{adapter}`
  collectors with `adapter` and `outcome` already in the cardinality
  allowlist), SPEC-IR-001 (`CategoryAcademic` selects adapters whose
  `Capabilities.DocTypes` intersects `[DocTypePaper, DocTypeRepo,
  DocTypeIssue]` per `internal/router/category.go:97`), SPEC-ADP-001
  (Reddit reference shape — file layout, error mapping discipline,
  `parseRetryAfter` helper pattern, `categorizeStatus` rosetta, redirect
  allowlist, MX tag plan, TDD harness — all reused as pattern), and
  SPEC-ADP-002 (HN reference for second-adapter shape inheritance,
  cursor-via-Metadata convention, multi-source comparison hardening).
  SPEC-FAN-001 status: approved (the M3 fanout gateway), so all M3 ADP-*
  SPECs are unblocked per `.moai/project/roadmap.md:122-123`.

  Key structural inheritance from ADP-001/002:
  - File layout (`{adapter}.go`/`search.go`/`client.go`/`parse.go`/
    `errors.go`/`bench_test.go` + testdata/) → identical in
    `internal/adapters/arxiv/`. The `score.go` file from ADP-001/002 is
    NOT reused — arXiv has no per-paper relevance score (see §2.3
    Score Population Strategy).
  - HTTP client construction (10s timeout, redirect allowlist,
    `reqid.NewTransport` wrapping) — host allowlist swapped to
    `{export.arxiv.org, arxiv.org}`.
  - `categorizeStatus` rosetta — adopted as-is, only `Adapter` field
    swapped from `"reddit"`/`"hackernews"` to `"arxiv"`. See §6.2 for
    duplication-vs-share posture (defer extraction; rule of three not
    yet hit).
  - `parseRetryAfter` helper — adopted verbatim (RFC 7231 §7.1.3
    parser, 5s default, 60s cap). arXiv rarely returns 429 in practice
    (the 3-second guideline is operational courtesy, not enforced
    server-side), but the helper matters for completeness.
  - Sole-emitter discipline (zero metrics/logs/spans emitted by the
    adapter; registry wrappedAdapter at
    `internal/adapters/registry.go:172-263` emits all observability).
  - `var _ types.Adapter = (*Adapter)(nil)` compile-time interface
    assertion.

  arXiv-specific deltas from ADP-001/002 (documented inline in §6 and
  §7):
  - Endpoint: `https://export.arxiv.org/api/query` (Cornell-hosted
    public API; no auth).
  - Response format: Atom 1.0 XML with three namespaces (default
    `http://www.w3.org/2005/Atom`, opensearch
    `http://a9.com/-/spec/opensearch/1.1/`, arxiv extension
    `http://arxiv.org/schemas/atom`). `encoding/xml` (stdlib) handles
    namespace-prefixed elements via `xml.Name{Space, Local}`.
  - URL parameters: `search_query`, `start` (0-indexed), `max_results`
    (≤100 clamp v0.1; arXiv's higher 2000 ceiling under-clamped for
    uniform adapter behaviour — see Open Question §11.3),
    `sortBy=relevance`, `sortOrder=descending` (both hardcoded v0.1).
  - Per-paper fields: `<id>` (canonical URL), `<published>` (RFC 3339
    submission date), `<updated>` (RFC 3339 last-version date), `<title>`,
    `<summary>` (abstract), `<author><name>` (≥1 per entry),
    `<arxiv:primary_category term>`, `<category term>` (≥1),
    `<arxiv:doi>` (optional; ~30% of papers), `<arxiv:journal_ref>`
    (optional), `<arxiv:comment>` (optional), `<link rel="alternate">`,
    `<link title="pdf">` (optional).
  - Score field absent — `NormalizedDoc.Score` set to constant `0.5`
    (neutral) per §2.3 rationale. RRF in SPEC-IDX-001 weights rank not
    score, so the constant is harmless. Open Question §11.5 documents
    revisit triggers.
  - Pagination: integer-string cursor (e.g., `"25"` for page 2 with
    page-size 25) surfaced via `Metadata["next_cursor"]` on the LAST
    returned doc; parsed via `strconv.Atoi` on the way back in.
  - Rate-limit discipline: arXiv published guideline is "play nice and
    incorporate a 3 second delay" (research §3.8, quoted verbatim from
    https://info.arxiv.org/help/api/user-manual.html). The adapter
    enforces this via a per-instance minimum-interval gate
    (`Options.MinRequestInterval`, default `3 * time.Second`; tests
    inject smaller values to keep concurrent-safety test runtimes
    reasonable). The mutex-guarded `nextRequest` time.Time is the ONLY
    shared mutable state in the adapter; the actual wait happens
    OUTSIDE the lock with ctx-cancellation respected via `select`. This
    is the only divergence from ADP-001 D3's "never sleeps" discipline,
    documented in `Capabilities.Notes`.
  - Title and summary whitespace collapse: arXiv pretty-prints XML
    with arbitrary newlines and indentation in `<title>` and
    `<summary>`. The adapter applies `strings.Join(strings.Fields(s),
    " ")` before assigning to NormalizedDoc fields. LaTeX strings (e.g.,
    `$E=mc^2$`) are preserved as plain text — synthesis decides whether
    to render or strip.

  Decision: Direct arXiv REST API (path B) chosen over wrapping
  `openags/paper-search-mcp` (path A). Rationale documented in
  research.md §2.4 (six factors: reference-shape discipline, risk
  discipline, deployment simplicity, scope discipline, future
  composability, zero new module dependencies). The roadmap entry at
  `.moai/project/roadmap.md:48` ("wrap `openags/paper-search-mcp`") is
  reinterpreted: ADP-003 v0.1 ships arXiv direct; the multi-source MCP
  wrap is deferred to a follow-up `SPEC-ADP-003-MCP` if measured demand
  for non-arXiv academic sources warrants the Python sidecar overhead
  (Open Question §11.1).

  12 EARS REQs (10 × P0 + 2 × P1) covering all five EARS patterns
  (Ubiquitous, Event-Driven, State-Driven via REQ-ADP3-011 concurrency
  contract AND REQ-ADP3-012 rate-limit serialization, Optional,
  Unwanted), 4 NFRs, ~40 representative TDD tests, 8 Open Questions
  carried forward from research.md §7. Zero new Go module dependencies
  — pure stdlib (`net/http`, `encoding/xml`, `time`, `context`,
  `errors`, `strings`, `strconv`, `net/url`, `sync`, `unicode`,
  `unicode/utf8`) plus existing `pkg/types` and `internal/obs/reqid`
  (nil-safe consumer). Inserted into M3 as the FIRST adapter SPEC after
  the FAN-001 gateway; the remaining six M3 ADP-* SPECs (ADP-004
  through ADP-009) parallelize. Harness level: standard (single domain,
  ≤10 source files, no security/payment keywords). Sprint Contract
  optional. Ready for plan-auditor review and annotation cycle.

---

## 1. Purpose

The M3 milestone exit criterion is unambiguous: `usearch query` returns
fused results across ≥5 adapters; Korean query returns Naver results
ranked first (`.moai/project/roadmap.md:150`). SPEC-ADP-001 implemented
the Reddit adapter as the reference shape; SPEC-ADP-002 implemented HN
as the second-adapter validation; SPEC-FAN-001 (status: approved) is
the M3 fanout gateway that dispatches to all registered adapters.
SPEC-ADP-003 implements the THIRD adapter — and the FIRST academic
adapter — via a direct client of arXiv's public REST API at
`https://export.arxiv.org/api/query`.

arXiv is the natural choice for the first M3 adapter because:

1. **Public no-auth endpoint** (Cornell-hosted, online since 1991)
   eliminates secret-management complexity from the M3 thin slice,
   identical to ADP-001/002's posture. Zero ENV propagation, zero
   RegisterOptions complexity. The rate-limit guideline ("play nice and
   incorporate a 3 second delay") is operational courtesy, not enforced
   server-side, and is honoured by the adapter via a per-instance
   minimum-interval gate (REQ-ADP3-012).
2. **Stable, well-documented API** (https://info.arxiv.org/help/api/
   user-manual.html). The endpoint has been stable since at least 2008
   per Wikipedia (https://en.wikipedia.org/wiki/ArXiv). Low risk of
   surprise during run-phase implementation.
3. **Different response shape than Reddit/HN** (Atom 1.0 XML with
   three namespaces vs JSON). Exercises the contract-mapping discipline
   established in ADP-001 §6.3 and ADP-002 §6.3 against a genuinely
   different envelope. The `encoding/xml` (stdlib) parsing exercise is
   the reference for any future XML-bearing adapter (RSS-based Korean
   adapters in SPEC-ADP-009 may follow).
4. **DocTypePaper coverage**. arXiv fills the
   `Capabilities.DocTypes = [DocTypePaper]` slot for the
   `CategoryAcademic` intent class (`internal/router/category.go:97`
   maps `academic` to `[DocTypePaper, DocTypeRepo, DocTypeIssue]`).
   SPEC-ADP-004 will fill `DocTypeRepo`/`DocTypeIssue` via GitHub.
5. **All four error categories reachable** in normal operation:
   arXiv returns 400 for malformed `start`/`max_results`
   (CategoryPermanent), 5xx during cluster maintenance
   (CategoryUnavailable), 429 rare but possible during sustained load
   (CategoryRateLimited), and timeouts/network blips resolve to
   (CategoryTransient via `context.DeadlineExceeded`). Validates the
   error-taxonomy contract for academic-source adapters.
6. **No score field — Score=0.5 constant**. arXiv's Atom feed does
   not surface a per-entry relevance score (unlike Reddit's `score`
   field or HN's `points` field). The adapter sets
   `NormalizedDoc.Score = 0.5` (neutral) per §2.3 rationale. SPEC-IDX-001
   RRF (M3) uses RANK not SCORE for fusion across adapters, so the
   constant is harmless; ranking quality is preserved through
   adapter-internal output order.
7. **Rate-limit serialisation discipline**. arXiv's 3-second-between-
   requests guideline forces the adapter to maintain ONE small piece of
   shared state (the `nextRequest` time.Time) — the only such shared
   state across all adapter SPECs to date. REQ-ADP3-012 makes the
   contract testable. The mutex-guarded gate is the reference for any
   future adapter with a published rate-limit guideline (e.g., GitHub's
   5000/hr unauth, NCBI's 3/sec, NASA ADS).

Like ADP-001/002, the arXiv adapter does NOT do fanout (SPEC-FAN-001
owns goroutine dispatch), does NOT do retry (SPEC-FAN-001 owns
orchestration; v0.1 ships zero-retry per FAN-001 D6), does NOT do
caching (SPEC-CACHE-001 owns 5-phase fallback), does NOT do ranking
fusion (SPEC-IDX-001 owns RRF), and does NOT emit any metric/log/span
itself (the registry wrappedAdapter does, sole-emitter discipline). It
DOES one job: turn a `types.Query` into an arXiv HTTP request, parse
the Atom XML feed, and return `[]types.NormalizedDoc` or
`*types.SourceError`.

Completion adds a third concrete adapter to the registry, validating
the FAN-001 dispatch contract against three sources (Reddit, HN, arXiv)
across two intent classes (social, academic). The arXiv adapter also
serves as the THIRD copy of the reference pattern, validating that
ADP-001's shape is portable to a fundamentally different
response-format domain (XML vs JSON) before the remaining six-way M3
ADP-* parallel implementation begins. The SPEC unblocks no downstream
SPECs explicitly — its `blocks` list is empty — but completion
contributes to the M3 exit criterion (`.moai/project/roadmap.md:150`,
"≥5 adapters fused").

---

## 2. Scope

### 2.1 In-Scope

| Ref | Item |
|-----|------|
| a | `internal/adapters/arxiv/arxiv.go`: `Adapter` struct (HTTP client + base URL + user-agent + healthcheck target + rate-limit state per §2.4), `New(opts Options) (*Adapter, error)` constructor, `Name() string` returning `"arxiv"`, `Capabilities() types.Capabilities` returning a deterministic descriptor (RequiresAuth=false, AuthEnvVars=nil, DocTypes=[DocTypePaper], SupportedLangs=nil (language-agnostic; arXiv is English-dominant but does not surface a per-paper language field), SupportsSince=true (arXiv supports `submittedDate:[…]` ranges in `search_query` syntax — Optional REQ-ADP3-007), RateLimitPerMin=20 (informational; the 3s/req guideline yields ~20/min), DefaultMaxResults=25, DisplayName="arXiv", Notes documenting the public no-auth endpoint + 3-second courtesy interval + `sortBy=relevance` discipline + Score=0.5 constant + LaTeX-pass-through behaviour), and `Healthcheck(ctx) error` (TCP-connect probe to `export.arxiv.org:443` with caller-supplied ctx, target injectable via Options). Compile-time interface assertion `var _ types.Adapter = (*Adapter)(nil)` at the bottom. |
| b | `internal/adapters/arxiv/search.go`: `(*Adapter).Search(ctx, q types.Query) ([]types.NormalizedDoc, error)` — the hot path. Validates the query (REQ-ADP3-008), parses any `q.Cursor` as a non-negative integer start (REQ-ADP3-008), waits for the per-instance rate-limit slot via `waitForRateSlot(ctx)` (REQ-ADP3-012), builds the request URL via `url.Values` with `search_query`, `start`, `max_results` (clamped), `sortBy=relevance`, `sortOrder=descending`, and optionally a category-filter prepended `cat:<value> AND ` to `search_query` when REQ-ADP3-007 filter is present, delegates HTTP execution to `client.go`, delegates response parsing to `parse.go`, returns `[]NormalizedDoc` or `*SourceError`. Honours `ctx` cancellation throughout. |
| c | `internal/adapters/arxiv/client.go`: HTTP client construction (timeout=10s default, `CheckRedirect` enforces a domain allowlist `{export.arxiv.org, arxiv.org}` with max 3 hops, `Transport` wrapped with `internal/obs/reqid.NewTransport(http.DefaultTransport)` for request-ID propagation), single `doRequest(req *http.Request) (*http.Response, error)` helper that sets the User-Agent header and the `Accept: application/atom+xml` header, and `categorizeStatus(httpStatus int, retryAfter time.Duration, cause error) *types.SourceError` mapping HTTP status → Category per the table in §6 — adopted verbatim from ADP-001/002 with the `Adapter` field swapped to `"arxiv"`. |
| d | `internal/adapters/arxiv/parse.go`: `parseFeed(body []byte, retrievedAt time.Time, currentStart int) ([]types.NormalizedDoc, string, error)` — parses the Atom XML envelope `{feed > entry+, opensearch:totalResults, opensearch:startIndex, opensearch:itemsPerPage}` into `[]NormalizedDoc` and returns the next-page cursor as the second return value. Per-entry transform per the field-mapping table in §6.3. Whitespace-collapse helper `collapseWS(s string) string` applied to title and summary (`strings.Join(strings.Fields(s), " ")`). Empty `<feed>` (zero `<entry>` elements) returns `(nil, "", nil)`. Malformed XML returns `*SourceError{Category: CategoryPermanent, Cause: <xml error>}`. |
| e | `internal/adapters/arxiv/errors.go`: package-private sentinel `ErrInvalidQuery = errors.New("arxiv: query text empty or whitespace-only")` (wrapped in `*SourceError{Category: CategoryPermanent}` by Search), `ErrInvalidStart = errors.New("arxiv: start cursor must be non-negative integer")` (wrapped in `*SourceError{Category: CategoryPermanent}` by Search). The `parseRetryAfter(header string, now time.Time) time.Duration` helper from ADP-001 is duplicated here (5s default, 60s cap, integer-seconds + HTTP-date parsing per RFC 7231 §7.1.3) — see §6.2 for shared-helper alternatives. |
| f | `internal/adapters/arxiv/arxiv_test.go`: tests for Adapter interface conformance (`var _ types.Adapter` assertion via `assertInterface`), `Name()` returns `"arxiv"`, `Capabilities()` returns deterministic value (called twice; equal), `Healthcheck()` succeeds against a stub `httptest.Server`, `New()` validates options and applies defaults including `MinRequestInterval = 3 * time.Second` when zero. |
| g | `internal/adapters/arxiv/search_test.go`: the largest test file. Drives `(*Adapter).Search` against `httptest.Server` with golden Atom XML fixtures: happy path 25 entries (mix of with/without DOI, single/multi author), empty result, 429 with Retry-After, 4xx including 400 (malformed start), 5xx, redirect to allowed and disallowed hosts, category filter, pagination cursor round-trip, ctx cancellation mid-request, invalid cursor rejection, overshoot (start > totalResults). |
| h | `internal/adapters/arxiv/client_test.go`: HTTP client unit tests — `categorizeStatus` truth table over 7 status codes, `parseRetryAfter` table over 6 input shapes, redirect allowlist enforcement (allowlist + cross-domain rejection + chain-over-3 rejection), User-Agent header presence, `Accept: application/atom+xml` header presence. |
| i | `internal/adapters/arxiv/parse_test.go`: field-mapping unit tests — table over 7 fixtures (full entry with DOI + journal_ref + comment, entry without DOI, multi-author entry with 5 authors, multi-version `<id>` ending in `v15`, entry with LaTeX in title, overshoot empty feed, malformed XML for parse-error path). Asserts each NormalizedDoc field per the §6.3 mapping table. Whitespace collapse over 5 input shapes (deeply nested newlines, multiple consecutive spaces, leading/trailing whitespace, control chars passthrough, Unicode preservation). Pagination cursor round-trip (start=0, totalResults=100 → next_cursor="25"). Hash field is empty (REQ-ADP3-006). |
| j | `internal/adapters/arxiv/rate_test.go`: rate-limit serialisation tests for REQ-ADP3-012 — 3 successive Search calls on the same `*Adapter` with `MinRequestInterval=10ms` total elapsed ∈ [20ms, 50ms]; ctx cancellation during a wait returns `context.DeadlineExceeded` immediately rather than waiting full interval; rate state is per-instance (two separate `*Adapter` instances do not share state). |
| k | `internal/adapters/arxiv/bench_test.go`: `BenchmarkParseFeed25Entries` (NFR-ADP3-001 — p50 ≤ 5 ms parse time on amd64 for a 25-entry Atom XML fixture; allocation ≤ 28 allocs per entry parsed = ≤ 700 allocs total; XML floor is higher than JSON's ≤ 20/doc due to `encoding/xml` constant overhead — see §4 NFR-ADP3-001 floor analysis). `TestMain` calls `goleak.VerifyTestMain(m)` (NFR-ADP3-004). |
| l | `internal/adapters/arxiv/testdata/`: golden Atom XML fixtures — `search_response.xml` (25-entry happy path mixing with/without DOI, single/multi author, ~12KB), `search_response_empty.xml` (zero-entry feed, ~500B), `search_response_pagination.xml` (start=0, totalResults=100; surfaces next_cursor="25", ~12KB), `search_response_with_doi.xml` (single entry with DOI populated, ~2KB), `search_response_no_doi.xml` (single entry without DOI, ~2KB), `search_response_multi_author.xml` (entry with 5 authors, ~3KB), `search_response_multi_version.xml` (entry with `<id>` ending in `v15`, ~2KB), `search_response_overshoot.xml` (start > totalResults; empty feed, ~500B), `search_response_400_error.xml` (Atom feed with single error `<entry>` for HTTP 400 path, ~500B), `search_response_latex_title.xml` (title containing `$E=mc^2$`, ~2KB), `search_response_malformed.xml` (truncated XML for parse-error path, ~200B). |

### 2.2 Out-of-Scope

This SPEC explicitly excludes the following items. Each has a known
destination SPEC; this list prevents scope creep into ADP-003 (the
first M3 adapter validating XML response handling).

- **Wrapping `openags/paper-search-mcp` for non-arXiv academic
  sources** (PubMed, bioRxiv, medRxiv, Google Scholar, Semantic
  Scholar, Crossref, OpenAlex, etc.) → future `SPEC-ADP-003-MCP` if
  measured demand warrants the Python sidecar overhead. Rationale in
  research.md §2.4 (six factors). Open Question §11.1 documents
  revisit triggers.
- **Per-source customisations specific to other M3 sources** (GitHub
  PAT auth, YouTube yt-dlp metadata, Bluesky AT Protocol, Naver
  Korean-locale handling, Daum scraper-style handling, KoreaNewsCrawler
  RSS, SearXNG bridge, Polymarket public API) → SPEC-ADP-004 through
  SPEC-ADP-009 (M3, parallelizable per
  `.moai/project/roadmap.md:122-123`).
- **Retry orchestration** (exponential backoff, circuit breaker,
  per-source health-state tracking, jitter, max-attempt counters) →
  SPEC-FAN-001 v0.1 ships zero-retry per its D6 decision; future
  SPEC-FAN-001-RETRY may add. The adapter returns one categorised error
  per request and does not retry.
- **Response caching** → SPEC-CACHE-001 (M3). Each `Search` call is
  independent and idempotent at the adapter layer (with the noted
  exception of the per-instance rate-limit state, which is correctness
  state not caching state).
- **Result ranking, deduplication, RRF fusion across adapters** →
  SPEC-IDX-001 (M3). The adapter returns entries in arXiv's relevance
  order with `Score=0.5` constant; cross-adapter ranking is fusion's
  job. SPEC-FAN-001 also performs URL-canonicalization-based dedup
  before passing to RRF.
- **arXiv `id_list` lookup mode** (direct retrieval by arXiv ID) →
  out of v0.1 scope; v0.1 hardcodes the `search_query` mode. A future
  P2 enhancement may add a `Query.Filters[Key="id_list"]` switch.
- **arXiv `search_by_date` semantics** — arXiv supports
  `sortBy=submittedDate` and `sortBy=lastUpdatedDate` for chronological
  sort; v0.1 hardcodes `sortBy=relevance` to mirror ADP-001's
  `sort=relevance` discipline. A future filter `Query.Filters[Key="sort"]`
  may surface this; deferred to P2.
- **Date-range filtering via `submittedDate:[...]` search-query
  syntax** — `Capabilities.SupportsSince=true` is declared (since
  arXiv's `search_query` syntax DOES support date-range expressions),
  but v0.1 does NOT implement the translation from `Query.Filters`
  date-range to `submittedDate:[…]`. The capability declaration is
  forward-compatible with REQ-IR-008 but the implementation is deferred
  to a future P2 enhancement when consumer needs are clearer. Open
  Question §11.8 documents.
- **Citation count integration** (Semantic Scholar API enrichment
  step to populate `NormalizedDoc.Score` with citation-derived signal)
  → out of v0.1; Score=0.5 constant. Citation enrichment requires a
  separate adapter or post-processing step, deferred to a future
  SPEC-ENRICH-001 if measured demand warrants. Open Question §11.5.
- **Author affiliation extraction** (`<arxiv:affiliation>` child of
  `<author>`) → out of v0.1; affiliations are ignored. Future patch
  SPEC may add as Metadata key `affiliations`. Open Question §11.7.
- **PDF download integration** (`<link title="pdf">` URL is exposed
  in Metadata for forward-compat; the adapter does NOT fetch PDFs in
  v0.1) → SPEC-CACHE-001 (M3) owns 5-phase access fallback that may
  consume the PDF URL.
- **OAI-PMH bulk-export protocol** (arXiv also exposes an OAI-PMH
  endpoint at `http://export.arxiv.org/oai2`) → out of v0.1 scope.
  OAI-PMH is for bulk metadata harvesting (full-archive ingestion),
  not search. The user manual explicitly distinguishes the two
  endpoints. tech.md row "OAI-PMH + arXiv Search API" may suggest both
  are used; v0.1 uses only the Search API.
- **Live network integration tests in CI** → out of v0.1.
  `httptest.Server` + golden Atom XML fixtures only. Optional env-gated
  live test (`-tags=integration` + `ARXIV_LIVE=1`) deferred to a future
  follow-up.
- **OpenAPI / proto schema for the adapter response** — the
  `[]types.NormalizedDoc` return type IS the schema; no separate IDL.
- **Korean tokenisation or language inference** for arXiv papers →
  SPEC-IDX-003 (M3). The adapter sets `NormalizedDoc.Lang = ""`
  (unknown). arXiv abstracts are English-dominant.
- **`pkg/llm` integration** — the arXiv adapter does NOT call any
  LLM. Classification is the Intent Router's job (SPEC-IR-001).
- **LaTeX rendering / stripping in `<title>` / `<summary>`** —
  arXiv abstracts contain LaTeX strings (`$E=mc^2$`, `$\alpha$`, etc.)
  as plain text. The adapter passes them through unmodified — synthesis
  (SPEC-SYN-001) decides whether to render or strip. Documented in
  `Capabilities.Notes`.
- **Cross-adapter helper extraction** (sharing `parseRetryAfter`,
  `categorizeStatus`, `redirectAllowlist` between Reddit, HN, and
  arXiv packages) — out of v0.1. The three adapters MAY duplicate
  these helpers; extraction is premature. ADP-002 §11.4 already noted
  this; a future SPEC-ADP-REFAC-001 (post-M3, after seeing 4+
  adapters) MAY consolidate. See §11.6.
- **Per-adapter custom Prometheus metrics** (e.g.,
  `arxiv_pagination_pages_total`) — would require amending
  SPEC-OBS-001's allowlist; out of scope. The shared
  `AdapterCalls{adapter="arxiv",outcome}` family is sufficient.

### 2.3 Score Population Strategy

[HARD] arXiv's Atom API does NOT surface a per-entry relevance score.
The `sortBy=relevance` parameter affects OUTPUT ORDER but no per-entry
numeric score is published. The adapter populates `NormalizedDoc.Score`
with the constant value `0.5` (mid-range neutral) on every returned
doc.

**Rationale** (verbatim from research.md §3.5):

- arXiv's relevance ranking already determines OUTPUT ORDER. Encoding
  the same signal as Score is redundant and may bias RRF.
- RRF in SPEC-IDX-001 uses RANK not SCORE for fusion across adapters
  (per FAN-001 §6.1). Position-derived score adds zero useful signal
  beyond rank.
- Tanh-normalised position would still produce a one-to-one mapping to
  position; setting `Score=0.5` and letting RRF use position achieves
  the same end with one fewer transformation.
- A future SPEC (post-V1) MAY integrate Semantic Scholar citation
  counts via a separate adapter or enrichment step.

**Constants in `arxiv.go`** (annotated `@MX:NOTE`):

```
const constantScore = 0.5 // arXiv has no per-paper score; see SPEC-ADP-003 §2.3
```

The `score.go` file from ADP-001/002 is NOT reused in `internal/adapters/arxiv/`
because there is no normalisation function — every doc gets the same
constant. The `@MX:NOTE` annotation on `constantScore` documents the
choice and tie-in to SPEC-IDX-001 RRF; Open Question §11.5 documents
revisit triggers.

**Tie-break behaviour**: All 25 arXiv entries from one Search call have
equal `Score`. Order is preserved from arXiv's response (arXiv's
`sortBy=relevance` ranking determines order; `Score` does not re-sort).
SPEC-IDX-001 RRF uses rank not score for fusion across adapters, so
equal scores within arXiv do not cause ranking instability.

### 2.4 Rate-Limit Serialisation State

[HARD] arXiv's published guideline:

> "we encourage you to play nice and incorporate a 3 second delay in
> your code."
> — https://info.arxiv.org/help/api/user-manual.html

This is operational courtesy, not an enforced server-side limit. The
adapter implements per-instance serialisation:

```go
// internal/adapters/arxiv/arxiv.go (sketch; final shape in run phase)
type Adapter struct {
    httpClient        *http.Client
    baseURL           string
    userAgent         string
    healthcheckTarget string

    // Rate-limit state (REQ-ADP3-012). Per-instance; mutex-guarded.
    rateMu      sync.Mutex
    nextRequest time.Time     // earliest moment the next outbound HTTP call may start
    minInterval time.Duration // from Options.MinRequestInterval; default 3s
}
```

**Algorithm** (`waitForRateSlot(ctx context.Context) error` in
`search.go`):

1. Lock `rateMu`. Compute `wait := time.Until(a.nextRequest)`.
2. Update `a.nextRequest = time.Now().Add(a.minInterval)`. Unlock.
3. If `wait <= 0`, return `nil` immediately.
4. Otherwise:
   ```
   select {
   case <-time.After(wait):
       return nil
   case <-ctx.Done():
       return ctx.Err()
   }
   ```

**Properties**:

- The mutex is held only briefly (compute + update; ~10ns). The actual
  wait happens OUTSIDE the lock. Concurrent goroutines do NOT serialise
  on the lock itself; they serialise on the `nextRequest` time.
- Ctx cancellation is respected: a caller with a 200ms deadline against
  an adapter that would otherwise wait 3s sees `context.DeadlineExceeded`
  immediately, not a 3-second hang.
- The `nextRequest` update happens BEFORE the wait. This means goroutine
  N+1 sees the slot reserved by goroutine N as soon as N enters the
  function, even if N is still waiting. Cumulative effect: 3 calls
  separated by 0 wall-clock time produce sleeps of (0, 3s, 6s) relative
  to the first call's start.
- Two separate `*Adapter` instances do NOT share state — the rate-limit
  is per-instance. The registry creates ONE `*Adapter` per registered
  source, so in practice "per-instance" = "per-source".

**Configurability**:

- `Options.MinRequestInterval` defaults to `3 * time.Second` matching
  arXiv's guideline.
- Tests inject `0` (or `1*time.Millisecond`) to keep the
  concurrent-safety test (REQ-ADP3-011, 50 goroutines) under unit-test
  runtime ceiling. The default is restored in production via a
  zero-value substitution in `New(opts)`.

**This is the only divergence from ADP-001 D3 ("never sleeps")
discipline**. It is documented in `Capabilities.Notes` so operators
understand the adapter holds shared mutable state.

REQ-ADP3-012 makes the gate testable. NFR-ADP3-003 (race-clean) verifies
the mutex is correct under 50-goroutine load.

---

## 3. EARS Requirements

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| REQ-ADP3-001 | Ubiquitous | The package `internal/adapters/arxiv` SHALL expose an `Adapter` struct that implements `pkg/types.Adapter` exactly: `Name() string` returning `"arxiv"`, `Search(ctx context.Context, q types.Query) ([]types.NormalizedDoc, error)`, `Healthcheck(ctx context.Context) error`, `Capabilities() types.Capabilities`. The package SHALL include a compile-time interface assertion `var _ types.Adapter = (*Adapter)(nil)`. `Capabilities()` SHALL be deterministic (two consecutive calls return equal values) with `SourceID="arxiv"`, `DisplayName="arXiv"`, `DocTypes=[DocTypePaper]`, `SupportedLangs=nil`, `SupportsSince=true`, `RequiresAuth=false`, `AuthEnvVars=nil`, `RateLimitPerMin=20`, `DefaultMaxResults=25`, and `Notes` containing the substrings `"public no-auth"`, `"3-second courtesy interval"`, `"sortBy=relevance"`, `"Score=0.5 constant"`, and `"LaTeX pass-through"`. | P0 | `TestAdapterName` (`(*Adapter).Name() == "arxiv"`); `TestAdapterImplementsInterface` (compile-time `var _ types.Adapter = (*Adapter)(nil)` succeeds); `TestCapabilitiesDeterministic` (two consecutive `Capabilities()` calls return `reflect.DeepEqual` results); `TestCapabilitiesShape` (asserts all 9 documented field values + Notes substring matches); `TestHealthcheckSucceeds` (TCP dial against test loopback succeeds via `Options.HealthcheckTarget` injection). All in `internal/adapters/arxiv/arxiv_test.go`. |
| REQ-ADP3-002 | Event-Driven | WHEN `(*Adapter).Search(ctx, q)` is invoked with a non-empty `q.Text`, the adapter SHALL build an HTTP GET request to `https://export.arxiv.org/api/query` with the following query parameters: `search_query=<url.QueryEscape(buildSearchQuery(q))>` where `buildSearchQuery` prepends `cat:<value> AND ` to `q.Text` when REQ-ADP3-007 category filter is present and otherwise returns `q.Text` verbatim, `start=<parsed cursor or 0>` (only present when `q.Cursor != ""` and parses non-negative; otherwise omitted), `max_results=clamp(q.MaxResults, 1, 100)` (defaulting to 25 when `q.MaxResults == 0`), `sortBy=relevance`, `sortOrder=descending`. The adapter SHALL execute the request via the constructed `*http.Client`, parse the Atom XML feed per REQ-ADP3-006 mapping, and return `(docs, nil)` on HTTP 200 with `len(docs) ≤ 100`. | P0 | `TestSearchHappyPath25Entries` (httptest.Server returns `search_response.xml`; assert 25 NormalizedDocs returned, each with all required fields populated and `Validate()` returning nil); `TestSearchURLParametersIncludeAllRequired` (inspect captured request URL; assert `search_query`, `max_results`, `sortBy=relevance`, `sortOrder=descending` always present); `TestSearchClampsMaxResultsTo100` (q.MaxResults=500 → URL has `max_results=100`); `TestSearchDefaultsMaxResultsTo25` (q.MaxResults=0 → URL has `max_results=25`); `TestSearchOmitsStartWhenCursorEmpty` (q.Cursor="" → URL has no `start` param OR `start=0` per implementation choice; assert one of these); `TestSearchSetsStartWhenCursorPresent` (q.Cursor="50" → URL contains `&start=50`); `TestSearchOvershoot` (`search_response_overshoot.xml`; start > totalResults → returns empty docs, no error). All in `search_test.go`. |
| REQ-ADP3-003 | Event-Driven | WHEN HTTP 429 is received from the arXiv endpoint, the adapter SHALL parse the `Retry-After` response header per RFC 7231 §7.1.3 (integer-seconds OR HTTP-date), cap the result at 60 seconds (any larger value is replaced with 60s), default to 5 seconds when the header is missing or malformed (arXiv typically omits `Retry-After` on 429 since 429 is rare in normal operation), and return `(nil, &types.SourceError{Adapter:"arxiv", Category: types.CategoryRateLimited, HTTPStatus: 429, RetryAfter: <duration>, Cause: errors.New("arxiv: rate limited")})`. The adapter SHALL NOT retry internally. | P0 | `TestSearchHTTP429WithIntegerRetryAfter` (`Retry-After: 30` → RetryAfter=30s); `TestSearchHTTP429WithHTTPDateRetryAfter` (HTTP-date 30s in future → RetryAfter ∈ (25s, 35s)); `TestSearchHTTP429NoRetryAfterDefaults5s` (no header → RetryAfter=5s); `TestSearchHTTP429RetryAfterCapped60s` (`Retry-After: 999` → 60s); `TestSearchHTTP429NoInternalRetry` (assert exactly 1 outbound request observed). All in `search_test.go` + `client_test.go`. |
| REQ-ADP3-004 | Event-Driven | WHEN HTTP 400, 401, 403, 404, or any 4xx other than 429 is received, the adapter SHALL return `(nil, &types.SourceError{Adapter:"arxiv", Category: types.CategoryPermanent, HTTPStatus: <code>, Cause: errors.New("arxiv: permanent failure: <code>")})`. HTTP 400 specifically covers arXiv-side validation rejections (e.g., `max_results > 30000`, non-integer `start`, malformed `id_list`); the adapter does NOT parse the Atom error-feed body in v0.1 — the HTTP status alone drives categorisation. The adapter SHALL NOT retry. | P0 | `TestSearchHTTP400ValidationError` (server returns 400 + `search_response_400_error.xml` body; assert `errors.Is(err, types.ErrPermanent)`, `HTTPStatus=400`); `TestSearchHTTP401`, `TestSearchHTTP403`, `TestSearchHTTP404` — each asserts `errors.Is(err, types.ErrPermanent)` and the returned `*SourceError.HTTPStatus` matches; `TestSearchHTTP4xxNoInternalRetry` (assert exactly 1 request observed). All in `search_test.go`. |
| REQ-ADP3-005 | Event-Driven | WHEN HTTP 500/502/503/504 is received OR a connection error occurs (DNS failure, dial timeout, read timeout, TLS handshake failure), the adapter SHALL return `(nil, &types.SourceError{Adapter:"arxiv", Category: types.CategoryUnavailable, HTTPStatus: <code or 0>, Cause: <inner error>})`. Network-layer errors set `HTTPStatus=0`. The adapter SHALL NOT retry. | P0 | `TestSearchHTTP500`, `TestSearchHTTP503` each assert `errors.Is(err, types.ErrSourceUnavailable)` and `HTTPStatus=500/503`; `TestSearchConnectionRefused` (httptest.Server closed before request) asserts `errors.Is(err, types.ErrSourceUnavailable)` and `HTTPStatus=0`; `TestSearchUnavailablePreservesUnderlyingError` (assert `errors.Unwrap(srcErr).Error()` contains the inner cause). In `search_test.go` + `client_test.go`. |
| REQ-ADP3-006 | Ubiquitous | The adapter SHALL transform each Atom `<entry>` element (excluding feed-level error entries) into one `types.NormalizedDoc` using the field mapping in §6.3, MUST set `RetrievedAt = time.Now().UTC()` at the moment of parsing, MUST leave `Hash = ""` (consumers compute via `CanonicalHash()`), MUST populate `Metadata` with at minimum the keys `{arxiv_id, authors, primary_category, categories, published_at, updated_at}` (six required keys), MUST set `DocType = types.DocTypePaper`, MUST set `Lang = ""` (unknown), MUST set `Score = 0.5` (constant per §2.3). The adapter SHALL apply `collapseWS(s) = strings.Join(strings.Fields(s), " ")` to `<title>` and `<summary>` before assigning to `Title` and `Body` — arXiv pretty-prints XML with arbitrary newlines and indentation in these fields. The arXiv ID exposed in `NormalizedDoc.ID` SHALL be the bare identifier (e.g., `"2403.12345v2"`) obtained by stripping the prefix `http://arxiv.org/abs/` from the `<id>` element value. The next-page cursor (when `currentStart + len(entries) < totalResults`) SHALL be returned as the second return value of `parseFeed` so `Search` can surface it via `Metadata["next_cursor"]` on the LAST returned NormalizedDoc — encoded as `strconv.Itoa(currentStart + len(entries))`. | P0 | `TestParseFeedFieldMapping` (table-driven over 5 fixtures: full entry with DOI + journal_ref, entry without DOI, multi-author, multi-version `<id>`, LaTeX title); `TestParseFeedIDStripPrefix` (entry `<id>http://arxiv.org/abs/2403.12345v2</id>` → `NormalizedDoc.ID == "2403.12345v2"`); `TestParseFeedWhitespaceCollapse` (table over 5 input shapes — deeply nested newlines, multiple consecutive spaces, leading/trailing whitespace, control chars passthrough, Unicode preservation); `TestParseFeedScoreConstant` (every doc has `Score == 0.5` exactly); `TestParseFeedDOIInArxivNamespace` (entry with `<arxiv:doi>10.xyz/abc</arxiv:doi>` → `Metadata["doi"] == "10.xyz/abc"`); `TestParseFeedNoDOIOmitsKey` (entry without `<arxiv:doi>` → `Metadata` does NOT contain key `"doi"`); `TestParseFeedAuthorsList` (entry with 5 `<author><name>` → `Metadata["authors"]` is `[]string` of length 5; `Author` is the first); `TestParseFeedPaginationCursor` (fixture `start=0, totalResults=100, len(entries)=25` → returned NormalizedDocs[len-1].Metadata["next_cursor"] == "25"); `TestParseFeedNoCursorOnLastPage` (fixture `start=80, totalResults=100, len(entries)=20` → no doc has `next_cursor` key); `TestParseFeedHashEmpty` (every returned doc has `Hash == ""`); `TestParseFeedMetadataKeys` (all 6 required keys present in each returned doc); `TestParseFeedMultiVersionID` (`<id>` ending in `v15` → ID preserves the version suffix). All in `parse_test.go`. |
| REQ-ADP3-007 | Optional | WHERE `Query.Filters` contains an entry with `Key == "category"` AND `Value` is a non-empty arXiv-classification string (e.g., `"cs.AI"`, `"math.GT"`, `"physics.gen-ph"`), the adapter SHALL prepend `cat:<value> AND ` to the user's `q.Text` before url-encoding into the `search_query` parameter. WHERE the `category` filter is absent OR `Value == ""`, the adapter SHALL send `q.Text` verbatim as `search_query`. Filter keys other than `category` SHALL be silently ignored in v0.1 (no error returned). Malformed category values containing arXiv-syntax-significant characters (`(`, `)`, `:`, ` AND `, ` OR `, ` ANDNOT `) are NOT escaped — the adapter passes through. Future stricter validation may reject; Open Question §11.7. | P1 | `TestSearchCategoryFilterAdded` (Filters=[{category, "cs.AI"}], q.Text="transformer" → URL has `search_query=cat:cs.AI%20AND%20transformer`); `TestSearchCategoryFilterAbsent` (Filters=nil → URL has `search_query=transformer` verbatim); `TestSearchCategoryFilterEmpty` (Filters=[{category, ""}] → URL has `search_query=transformer` verbatim, no prepend); `TestSearchUnknownFilterIgnored` (Filters=[{nsfw, "true"}] → URL has `search_query=transformer` verbatim). All in `search_test.go`. |
| REQ-ADP3-008 | Unwanted | IF `Query.Text` is empty OR contains only Unicode whitespace runes (per `unicode.IsSpace` over every rune), THEN the adapter SHALL return `(nil, &types.SourceError{Adapter:"arxiv", Category: types.CategoryPermanent, Cause: ErrInvalidQuery})` immediately and SHALL NOT issue any HTTP request. IF `Query.Cursor` is non-empty AND does NOT parse as a non-negative integer via `strconv.Atoi` (negative integers also rejected; floats and exponential notation also rejected), THEN the adapter SHALL return `(nil, &types.SourceError{Adapter:"arxiv", Category: types.CategoryPermanent, Cause: ErrInvalidStart})` immediately and SHALL NOT issue any HTTP request. | P0 | `TestSearchEmptyQueryRejectedNoHTTP` (table over `["", "   ", "\t\n  \r"]` for `q.Text`; for each asserts `errors.Is(err, types.ErrPermanent)` AND `errors.Is(err, ErrInvalidQuery)` AND assert httptest.Server received zero requests AND assert no rate-limit slot was consumed (i.e., a follow-up Search call with valid query starts immediately, not 3s later)); `TestSearchInvalidStartRejectedNoHTTP` (table over `["abc", "-1", "1.5", "1e3", "  "]` for `q.Cursor`; for each asserts `errors.Is(err, types.ErrPermanent)` AND `errors.Is(err, ErrInvalidStart)` AND zero requests). All in `search_test.go`. |
| REQ-ADP3-009 | Ubiquitous | The adapter SHALL set the `User-Agent` HTTP header on every outbound request to a non-default value of the form `usearch/<version> (+https://github.com/elymas/universal-search)` where `<version>` is supplied via `Options.UserAgentVersion` (default `"v0.1"`). The adapter SHALL set the `Accept` header to `application/atom+xml`. While arXiv does not explicitly require a custom UA in its published guidelines, setting one preserves the project-wide convention from ADP-001 REQ-ADP-009 / ADP-002 REQ-ADP2-006 and identifies traffic for operational debugging at arXiv's side. | P0 | `TestSearchSetsCustomUserAgent` (inspect captured `r.Header.Get("User-Agent")`; assert it starts with `"usearch/"` and contains `"(+https://github.com/elymas/universal-search)"`); `TestSearchSetsAcceptAtomXML` (assert `Accept: application/atom+xml`); `TestSearchUserAgentVersionConfigurable` (Options.UserAgentVersion="v0.2-rc1" → header contains `"usearch/v0.2-rc1"`). All in `client_test.go`. |
| REQ-ADP3-010 | Optional | WHERE the response is HTTP 301/302/303/307/308, the adapter's `*http.Client.CheckRedirect` SHALL follow up to 3 redirects WITHIN the allowlist `{export.arxiv.org, arxiv.org}`. Cross-domain redirects (any other host) SHALL be rejected by returning an error from `CheckRedirect`; the adapter wraps this as `*SourceError{Adapter:"arxiv", Category: CategoryPermanent, Cause: errors.New("arxiv: cross-domain redirect rejected: <target host>")}` to prevent SSRF. Redirect chains exceeding 3 hops SHALL be rejected with a "too many redirects" message. While arXiv does not redirect cross-domain in normal operation, the allowlist is defensive and consistent with the pattern established in SPEC-ADP-001 REQ-ADP-010 and SPEC-ADP-002 REQ-ADP2-009. | P1 | `TestSearchFollowsAllowlistRedirect` (httptest.Server returns 302 to a second httptest.Server with Host header rewritten to `export.arxiv.org`; assert 200-path NormalizedDocs returned); `TestSearchRejectsCrossDomainRedirect` (httptest.Server returns 301 to `attacker.com`; assert `errors.Is(err, types.ErrPermanent)` AND error message contains `"cross-domain redirect"`); `TestSearchRejectsRedirectChainOver3` (httptest.Server bouncing within allowlist 4 times; assert error after 3 hops). All in `client_test.go`. |
| REQ-ADP3-011 | State-Driven | WHILE the same `*Adapter` instance is registered in the adapter registry and is being invoked concurrently from N goroutines (N ≥ 1), each `Search(ctx, q)` call SHALL execute independently with no race-detector alarms, the cumulative effect SHALL be N independent HTTP round-trips (modulo serialisation per REQ-ADP3-012). The adapter's only shared mutable state is the rate-limit gate (`rateMu`, `nextRequest`, per §2.4); access SHALL be mutex-guarded with the actual wait happening OUTSIDE the lock to prevent contention. The HTTP client (`*http.Client`) is goroutine-safe per Go stdlib documentation. | P0 | `TestSearchConcurrentSafe` in `search_test.go`: 50 goroutines each issuing one Search against the same httptest.Server, against an `*Adapter` constructed with `Options{MinRequestInterval: 0}` to disable rate-limit serialisation for this test. Assertions: (a) no race-detector alarm under `-race`; (b) total response count = 50 observed at the stub; (c) all 50 returned slices are `[]types.NormalizedDoc` with `Validate()` returning nil for every doc. NFR-ADP3-003 anchors. |
| REQ-ADP3-012 | State-Driven | WHILE the adapter's `Options.MinRequestInterval > 0`, consecutive `Search(ctx, q)` calls on the same `*Adapter` instance SHALL be serialised such that consecutive HTTP round-trips against the arXiv endpoint are separated by at least `MinRequestInterval`. The waiting goroutine SHALL respect ctx cancellation: a caller with a sub-`MinRequestInterval` deadline SHALL receive `context.DeadlineExceeded` (wrapped as `*SourceError{Category: CategoryUnavailable, Cause: ctx.Err()}`) immediately rather than waiting the full interval. The serialisation state is per-adapter-instance; two separate `*Adapter` instances do NOT share state. The default `MinRequestInterval` is `3 * time.Second` matching arXiv's published "play nice" guideline (https://info.arxiv.org/help/api/user-manual.html). | P0 | `TestSearchRateLimitInterval` in `rate_test.go` (3 successive Search calls on the same `*Adapter` constructed with `Options{MinRequestInterval: 10*time.Millisecond}`; assert total elapsed ∈ [20ms, 50ms]; assert 3 outbound requests observed at the stub); `TestSearchRateLimitCtxCancel` (1 Search succeeds; concurrent 2nd Search with ctx cancelled at 1ms while `MinRequestInterval=10s` returns within 5ms with `errors.Is(err, context.Canceled)` or wrapped equivalent); `TestSearchRateLimitPerInstance` (two separate `*Adapter` instances each call Search once at the same wall-clock moment with `MinRequestInterval=10s`; assert both return within 100ms — they do NOT serialise across instances). All in `rate_test.go`. |

---

## 4. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| NFR-ADP3-001 | Performance (parse path) | The parse path `parseFeed(body []byte, retrievedAt time.Time, currentStart int) ([]NormalizedDoc, string, error)` SHALL execute with mean wall-clock duration per op ≤ 5 ms over `go test -bench=BenchmarkParseFeed25Entries -benchtime=10x -count=5 ./internal/adapters/arxiv/...` on amd64; the median of the 5 runs is the assertion value (passes when ≤ 5 ms). The fixture is the `search_response.xml` golden (25-entry Atom feed, ~12KB). Allocation count ≤ 28 per entry parsed (i.e. ≤ 700 allocs total for 25 entries) per the same benchmark's `allocs/op` field. The 700 floor (vs ADP-001 NFR-ADP-001's amended 500) reflects the higher constant overhead of `encoding/xml` compared to `encoding/json`: each entry incurs ~6 namespace-aware element allocations (default + opensearch + arxiv tags) plus the same `pkg/types.NormalizedDoc.Metadata = map[string]any` floor (~17 allocs/doc) inherited from SPEC-CORE-001's contract. The original 500-floor target from ADP-001 is empirically infeasible for XML; the floor is documented here so future contributors do not chase a moving target. The 700 floor is itself a STARTING target — run-phase iteration may revise downward (per the iteration-3 amendment pattern from ADP-001's HISTORY 2026-04-26) once empirical baseline is established. Measured via `BenchmarkParseFeed25Entries` in `internal/adapters/arxiv/bench_test.go`, run weekly in CI per the cadence established in SPEC-OBS-001 NFR-OBS-001. Benchmarks do not count toward coverage. |
| NFR-ADP3-002 | End-to-end Latency | The end-to-end `Search` round-trip against the `httptest.Server` stub (no real network, with `Options{MinRequestInterval: 0}` to disable rate-limit serialisation for this test) SHALL complete with p95 ≤ 200 ms over 100 invocations, measured by `TestSearchE2ELatencyStubP95` in `search_test.go` (sort durations ascending, assert `durations[94] ≤ 200ms`). The harder live-arXiv p95 is documented as the operational target (≤ 5s, similar to Reddit) but is NOT enforced in CI (no live network). Note: this NFR explicitly excludes the rate-limit interval from the measurement — production callers will see end-to-end latency dominated by the 3-second courtesy interval when issuing back-to-back Search calls. The NFR measures parse + HTTP overhead only. |
| NFR-ADP3-003 | Race-clean concurrent invocation | `internal/adapters/arxiv/search_test.go::TestSearchConcurrentSafe` (REQ-ADP3-011 acceptance) SHALL execute successfully under `go test -race ./internal/adapters/arxiv/...` with 50 goroutines × 1 stub server, against an `*Adapter` constructed with `Options{MinRequestInterval: 0}`. Race-detector alarms attributable to the arxiv package SHALL be zero. The mutex-guarded rate-limit state (`rateMu`, `nextRequest`) is exercised even when `MinRequestInterval=0` because the lock-and-update sequence still runs; the test verifies the lock is correct under contention. |
| NFR-ADP3-004 | No goroutine leak on cancellation | The adapter SHALL NOT leak any goroutine when the caller's context is cancelled mid-`Search`. Verified by `TestSearchNoGoroutineLeakOnCancel` in `search_test.go`, which uses `go.uber.org/goleak.VerifyNone(t)` after a `Search` call whose ctx is cancelled mid-flight via a 50ms-delayed cancel; assert zero residual goroutines after the call returns. `TestMain` in `bench_test.go` invokes `goleak.VerifyTestMain(m)` to enforce the property package-wide (mirrors the ADP-001 / ADP-002 `bench_test.go` pattern). The rate-limit waiter (REQ-ADP3-012) is the only place the adapter sleeps; ctx-cancel-while-waiting is exercised in `TestSearchRateLimitCtxCancel` and verified leak-free by goleak. |

---

## 5. Acceptance Criteria

### REQ-ADP3-001 — Adapter Interface Conformance

- File `internal/adapters/arxiv/arxiv.go` declares `Adapter` struct
  with the documented fields (`httpClient *http.Client`, `baseURL
  string`, `userAgent string`, `healthcheckTarget string`, `rateMu
  sync.Mutex`, `nextRequest time.Time`, `minInterval time.Duration`).
- The compile-time assertion `var _ types.Adapter = (*Adapter)(nil)`
  appears at the bottom of `arxiv.go`. If the interface ever drifts,
  this assertion fails to compile.
- `(*Adapter).Name()` returns the literal string `"arxiv"`.
- `(*Adapter).Capabilities()` returns a `types.Capabilities` with:
  - `SourceID = "arxiv"` (matches `Name()`)
  - `DisplayName = "arXiv"`
  - `DocTypes = []types.DocType{types.DocTypePaper}`
  - `SupportedLangs = nil` (language-agnostic)
  - `SupportsSince = true` (forward-compat for date-range filter; v0.1
    does not implement date-range translation — Open Question §11.8)
  - `RequiresAuth = false`
  - `AuthEnvVars = nil`
  - `RateLimitPerMin = 20` (3-second courtesy interval ≈ 20/min)
  - `DefaultMaxResults = 25`
  - `Notes` contains the substrings `"public no-auth"`,
    `"3-second courtesy interval"`, `"sortBy=relevance"`,
    `"Score=0.5 constant"`, and `"LaTeX pass-through"`.
- `(*Adapter).Healthcheck(ctx)` succeeds against an httptest.Server
  bound to `127.0.0.1:0`. Tests construct the Adapter with
  `Options{HealthcheckTarget: <httptest.Server.Listener.Addr().String()>}`
  to redirect the dial target; the production default is
  `"export.arxiv.org:443"`.
- `New(opts)` substitutes `MinRequestInterval = 3 * time.Second` when
  `opts.MinRequestInterval == 0`.
- `TestAdapterName`, `TestAdapterImplementsInterface`,
  `TestCapabilitiesDeterministic`, `TestCapabilitiesShape`,
  `TestHealthcheckSucceeds`, `TestNewAppliesMinRequestIntervalDefault`
  all pass.

### REQ-ADP3-002 — Search Happy Path

- `TestSearchHappyPath25Entries` against
  `testdata/search_response.xml` returns exactly 25 `NormalizedDoc`
  entries (mix of with/without DOI, single/multi author); each passes
  `Validate()` returning nil; the captured request URL contains the 4
  mandatory query parameters (`search_query`, `max_results`,
  `sortBy=relevance`, `sortOrder=descending`).
- `TestSearchURLParametersIncludeAllRequired`,
  `TestSearchClampsMaxResultsTo100`,
  `TestSearchDefaultsMaxResultsTo25`,
  `TestSearchOmitsStartWhenCursorEmpty`,
  `TestSearchSetsStartWhenCursorPresent` (q.Cursor="50" → URL
  contains `&start=50`), `TestSearchOvershoot` (start > totalResults
  → empty docs, no error) all pass.

### REQ-ADP3-003 — HTTP 429 Rate-Limit Mapping

- `TestSearchHTTP429WithIntegerRetryAfter` asserts returned err is
  `*types.SourceError` with `Category=CategoryRateLimited`,
  `HTTPStatus=429`, `RetryAfter=30s`.
- `TestSearchHTTP429WithHTTPDateRetryAfter` parses an HTTP-date 30s
  in the future; asserts `RetryAfter` is in `(25s, 35s)`.
- `TestSearchHTTP429NoRetryAfterDefaults5s` (no header — arXiv's
  typical case) asserts `RetryAfter=5s`.
- `TestSearchHTTP429RetryAfterCapped60s` (`Retry-After: 999`) asserts
  `RetryAfter=60s`.
- `TestSearchHTTP429NoInternalRetry` asserts exactly 1 request observed.

### REQ-ADP3-004 — HTTP 4xx Permanent Mapping

- `TestSearchHTTP400ValidationError`: server returns 400 with
  `search_response_400_error.xml` body; assert
  `errors.Is(err, types.ErrPermanent)` and `HTTPStatus=400`. The
  adapter does NOT attempt to parse the Atom error-feed body.
- `TestSearchHTTP401`, `TestSearchHTTP403`, `TestSearchHTTP404` each
  assert `errors.Is(err, types.ErrPermanent)` and the returned
  `*SourceError.HTTPStatus` matches the stub's status code.
- `TestSearchHTTP4xxNoInternalRetry` asserts exactly 1 request observed.

### REQ-ADP3-005 — HTTP 5xx and Network Failure

- `TestSearchHTTP500`, `TestSearchHTTP503` each assert
  `errors.Is(err, types.ErrSourceUnavailable)` and `HTTPStatus=500/503`.
- `TestSearchConnectionRefused` (httptest.Server closed before
  request) asserts `errors.Is(err, types.ErrSourceUnavailable)` and
  `HTTPStatus=0`.
- `TestSearchUnavailablePreservesUnderlyingError`: assert
  `errors.Unwrap(srcErr) != nil` and the inner error message contains
  "connection refused" or equivalent.

### REQ-ADP3-006 — NormalizedDoc Field Mapping

- `TestParseFeedFieldMapping` table-drives 5 fixtures (full entry with
  DOI + journal_ref, entry without DOI, multi-author, multi-version
  `<id>`, LaTeX title). For each, asserts every NormalizedDoc field per
  the §6.3 mapping table (ID with bare arXiv ID, SourceID="arxiv", URL
  = `<id>` value, Title whitespace-collapsed, Body whitespace-collapsed,
  Snippet 280-rune-truncated, PublishedAt parsed RFC 3339, RetrievedAt
  non-zero, Author = first author name, Score==0.5, Lang="",
  DocType=DocTypePaper, Citations=nil, Metadata keys present per §6.3).
- `TestParseFeedIDStripPrefix`: entry
  `<id>http://arxiv.org/abs/2403.12345v2</id>` → `NormalizedDoc.ID ==
  "2403.12345v2"`. Edge case: `TestParseFeedMultiVersionID` with
  `v15` suffix still produces `ID == "2403.12345v15"`.
- `TestParseFeedWhitespaceCollapse`: table over 5 input shapes (deeply
  nested newlines, multiple consecutive spaces, leading/trailing
  whitespace, control chars passthrough, Unicode preservation including
  mathematical symbols). Each input is wrapped in a `<title>` element
  in a fixture; output `Title` matches `strings.Join(strings.Fields(s), " ")`.
- `TestParseFeedScoreConstant`: every doc has `Score == 0.5` exactly
  (byte-equal float64 comparison via `==` since 0.5 is representable
  exactly).
- `TestParseFeedDOIInArxivNamespace`: entry with
  `<arxiv:doi>10.xyz/abc</arxiv:doi>` → `Metadata["doi"] == "10.xyz/abc"`.
  The XML decoder must honour the `arxiv:` namespace prefix.
- `TestParseFeedNoDOIOmitsKey`: entry without `<arxiv:doi>` →
  `Metadata` does NOT contain key `"doi"` (key absence is the signal,
  not empty string).
- `TestParseFeedAuthorsList`: entry with 5 `<author><name>` elements
  → `Metadata["authors"]` is `[]string` of length 5 in submission
  order; `NormalizedDoc.Author` is `Metadata["authors"][0]`.
- `TestParseFeedPaginationCursor`: fixture with `start=0,
  totalResults=100, len(entries)=25` → returned NormalizedDocs[len-1]
  `.Metadata["next_cursor"] == "25"`. Earlier docs do NOT have the
  `next_cursor` key.
- `TestParseFeedNoCursorOnLastPage`: fixture with `start=80,
  totalResults=100, len(entries)=20` → no doc has `next_cursor` key
  (since 80 + 20 == totalResults).
- `TestParseFeedHashEmpty`: every returned `NormalizedDoc.Hash`
  equals `""`.
- `TestParseFeedMetadataKeys`: each returned doc's Metadata has at
  least the 6 required keys `{arxiv_id, authors, primary_category,
  categories, published_at, updated_at}`.
- `TestParseFeedMalformedXML`: truncated XML body → `*SourceError{
  Category: CategoryPermanent}` with cause containing "xml" or
  "EOF".

### REQ-ADP3-007 — Category Filter

- `TestSearchCategoryFilterAdded`: `Filters=[{category, "cs.AI"}]`,
  `q.Text="transformer"` → captured URL has
  `search_query=cat%3Acs.AI+AND+transformer` (URL-encoded form of
  `cat:cs.AI AND transformer`).
- `TestSearchCategoryFilterAbsent`: `Filters=nil`, `q.Text="transformer"`
  → URL has `search_query=transformer` verbatim (URL-encoded only as
  needed).
- `TestSearchCategoryFilterEmpty`: `Filters=[{category, ""}]` →
  treated as absent; URL has `search_query=transformer` verbatim.
- `TestSearchUnknownFilterIgnored`: `Filters=[{nsfw, "true"}]` → URL
  has `search_query=transformer` verbatim (unknown key silently
  ignored).

### REQ-ADP3-008 — Empty Query and Invalid Cursor Rejection

- `TestSearchEmptyQueryRejectedNoHTTP` table-drives `q.Text` over
  `["", "   ", "\t\n  \r"]`; for each asserts
  `errors.Is(err, types.ErrPermanent)` AND
  `errors.Is(err, ErrInvalidQuery)`. The httptest.Server is
  instrumented with a request counter; assert exactly 0 requests.
  Additional assertion: a follow-up valid-query Search call does NOT
  wait `MinRequestInterval` — the rate-limit slot was not consumed by
  the rejected query.
- `TestSearchInvalidStartRejectedNoHTTP` table-drives `q.Cursor`
  over `["abc", "-1", "1.5", "1e3", "  "]`; for each asserts
  `errors.Is(err, types.ErrPermanent)` AND
  `errors.Is(err, ErrInvalidStart)`; assert zero requests.

### REQ-ADP3-009 — User-Agent and Accept Headers

- `TestSearchSetsCustomUserAgent`: captured request header
  `User-Agent` starts with `"usearch/"` and contains
  `"(+https://github.com/elymas/universal-search)"`.
- `TestSearchSetsAcceptAtomXML`: captured `Accept` header equals
  `"application/atom+xml"`.
- `TestSearchUserAgentVersionConfigurable`: `Options.UserAgentVersion
  = "v0.2-rc1"` → captured `User-Agent` contains `"usearch/v0.2-rc1"`.

### REQ-ADP3-010 — Redirect Allowlist

- `TestSearchFollowsAllowlistRedirect`: server A returns 302 with
  Location header pointing to server B (Host header rewritten to
  `export.arxiv.org`); the test installs server B as a custom
  `http.RoundTripper` resolver. Assert search succeeds and returns
  the body from server B.
- `TestSearchRejectsCrossDomainRedirect`: server A returns 302 with
  Location `https://attacker.com/x`. Assert
  `errors.Is(err, types.ErrPermanent)` and error message contains
  `"cross-domain redirect"`.
- `TestSearchRejectsRedirectChainOver3`: 4 servers chained within
  the allowlist; assert error returned after 3 hops with message
  containing `"too many redirects"`.

### REQ-ADP3-011 — Concurrent Search Safety (State-Driven)

- `TestSearchConcurrentSafe`: a single `*Adapter` is constructed with
  `Options{MinRequestInterval: 0, BaseURL: stubServer.URL}` so the
  rate-limit gate runs (lock + update + zero wait) but does not delay
  the 50 goroutines. The stub server records every inbound request.
  50 goroutines are launched, each calling
  `(*Adapter).Search(ctx, q)` exactly once with the same query.
  All goroutines start via a `sync.WaitGroup` barrier so the
  invocations overlap.
- Assertions:
  1. The test executes successfully under `go test -race`; the race
     detector reports zero data-race alarms attributable to the arxiv
     package. The `rateMu` mutex protects `nextRequest` correctly.
  2. The stub server's request counter equals 50 (one HTTP round-trip
     per goroutine).
  3. Every goroutine receives `(docs, nil)` with `len(docs) == 25`
     (matching the standard `search_response.xml` fixture); each
     returned `[]types.NormalizedDoc` slice has every doc passing
     `Validate()` returning nil. No goroutine receives an error
     attributable to concurrent state corruption.

### REQ-ADP3-012 — Rate-Limit Serialisation (State-Driven)

- `TestSearchRateLimitInterval`: construct `*Adapter` with
  `Options{MinRequestInterval: 10*time.Millisecond}`. Issue 3
  successive Search calls SEQUENTIALLY (not concurrent — the gate's
  serialisation is what we test). Measure total elapsed; assert ∈
  [20ms, 50ms]. Assert the stub server received exactly 3 requests.
  The lower bound (20ms) reflects the two waits between calls 1→2 and
  2→3; the upper bound (50ms) is generous for test-machine jitter.
- `TestSearchRateLimitCtxCancel`: construct `*Adapter` with
  `Options{MinRequestInterval: 10*time.Second}`. Issue Search call 1
  (succeeds immediately, consumes a rate-limit slot). Spawn a
  goroutine that issues Search call 2 with `ctx, cancel :=
  context.WithCancel(parent); cancel()` so ctx is already cancelled;
  measure elapsed time. Assert call 2 returns within 10ms (NOT 10
  seconds) with err such that
  `errors.Is(err, context.Canceled)` OR
  `errors.Is(err, types.ErrSourceUnavailable)` (depending on whether
  the adapter wraps the cancel; either is acceptable). The key
  invariant: ctx cancellation breaks the wait immediately.
- `TestSearchRateLimitPerInstance`: construct two separate
  `*Adapter` instances with `Options{MinRequestInterval:
  10*time.Second}` each. Issue Search on instance A and instance B at
  the same wall-clock moment. Assert both return within 100ms — they
  do NOT serialise across instances. (This validates that the
  rate-limit state is per-instance, not global.)

### NFR-ADP3-001 — Parse-Path Performance

- `BenchmarkParseFeed25Entries` is invoked as
  `go test -bench=BenchmarkParseFeed25Entries -benchtime=10x -count=5 ./internal/adapters/arxiv/...`
  on amd64.
- Assertion mechanism: take the 5 reported per-op mean wall-clock
  durations (one per `-count` run); the MEDIAN of those 5 values
  SHALL be ≤ 5 ms. PASS/FAIL is decidable from the `go test -bench`
  output alone.
- The bench reports `B/op` and `allocs/op`; `allocs/op` ≤ 700 (= 28 ×
  25 entries). The 700 floor is XML-amended from ADP-001's 500 — see
  NFR-ADP3-001 for the floor analysis. Run-phase iteration may revise
  downward (per ADP-001 iteration-3 amendment pattern) once empirical
  baseline is established.

### NFR-ADP3-002 — E2E p95 (Stub)

- `TestSearchE2ELatencyStubP95` runs 100 invocations against the
  stub `httptest.Server` with `Options{MinRequestInterval: 0}` (rate
  gate disabled), sorts elapsed durations, asserts
  `durations[94] ≤ 200ms`.

### NFR-ADP3-003 — Race-Clean Concurrent Workload

- `TestSearchConcurrentSafe` (REQ-ADP3-011 acceptance) executes under
  `go test -race`; race-detector alarms attributable to the arxiv
  package = 0. The test validates that the `rateMu` mutex protects
  `nextRequest` correctly under 50-goroutine contention.

### NFR-ADP3-004 — Goroutine Leak Check

- `TestSearchNoGoroutineLeakOnCancel`: `goleak.VerifyNone(t)`
  succeeds after a `Search` call whose ctx was cancelled at 50ms
  while the stub server delays response by 200ms.
- `TestMain` in `bench_test.go` invokes `goleak.VerifyTestMain(m)` to
  enforce the property package-wide. The rate-limit waiter
  (`waitForRateSlot`) is the only place the adapter creates a
  potentially-suspended goroutine via `time.After`; ctx-cancel-while-
  waiting is exercised in `TestSearchRateLimitCtxCancel` and verified
  leak-free by goleak.

---

## 6. Technical Approach

### 6.1 Files to Modify

**Created (15 files)**:

- `internal/adapters/arxiv/arxiv.go` — Adapter struct, New, Name,
  Capabilities, Healthcheck, compile-time interface assertion,
  rate-limit state declaration
- `internal/adapters/arxiv/arxiv_test.go` — interface conformance
  tests, defaults application
- `internal/adapters/arxiv/search.go` — Search method (the hot path),
  URL construction, category-filter prepend, rate-limit waiter
- `internal/adapters/arxiv/search_test.go` — main test file (largest)
- `internal/adapters/arxiv/client.go` — HTTP client construction,
  doRequest, categorizeStatus
- `internal/adapters/arxiv/client_test.go` — error mapping + redirect
  tests
- `internal/adapters/arxiv/parse.go` — parseFeed transform with Atom
  XML namespace handling
- `internal/adapters/arxiv/parse_test.go` — field mapping tests,
  whitespace-collapse table, namespace tests
- `internal/adapters/arxiv/errors.go` — ErrInvalidQuery /
  ErrInvalidStart sentinels + parseRetryAfter helper
- `internal/adapters/arxiv/rate_test.go` — rate-limit serialisation
  tests (REQ-ADP3-012)
- `internal/adapters/arxiv/bench_test.go` — NFR-ADP3-001 benchmark +
  TestMain with goleak.VerifyTestMain
- `internal/adapters/arxiv/testdata/search_response.xml` (~12KB)
- `internal/adapters/arxiv/testdata/search_response_empty.xml` (~500B)
- `internal/adapters/arxiv/testdata/search_response_pagination.xml`
  (~12KB)
- `internal/adapters/arxiv/testdata/search_response_with_doi.xml`
  (~2KB)
- `internal/adapters/arxiv/testdata/search_response_no_doi.xml`
  (~2KB)
- `internal/adapters/arxiv/testdata/search_response_multi_author.xml`
  (~3KB)
- `internal/adapters/arxiv/testdata/search_response_multi_version.xml`
  (~2KB)
- `internal/adapters/arxiv/testdata/search_response_overshoot.xml`
  (~500B)
- `internal/adapters/arxiv/testdata/search_response_400_error.xml`
  (~500B)
- `internal/adapters/arxiv/testdata/search_response_latex_title.xml`
  (~2KB)
- `internal/adapters/arxiv/testdata/search_response_malformed.xml`
  (~200B)

**Modified**: none. The adapter self-contains. `pkg/types` already
publishes the contract (including `DocTypePaper` at
`pkg/types/capabilities.go:17`), `internal/adapters/registry.go`
already accepts any `types.Adapter`,
`internal/obs/metrics/metrics.go` already declares `AdapterCalls` and
`AdapterCallDuration` collectors with `adapter` and `outcome` in the
cardinality allowlist (the `adapter="arxiv"` value is bounded by the
V1 14-adapter ceiling per SPEC-OBS-001 NFR-OBS-002).

**Unchanged (by design)**: same as ADP-001 §6.1 / ADP-002 §6.1 —
`registry.go`, `pkg/types/*`, `internal/obs/metrics/metrics.go`, and
`cmd/usearch/main.go` (registry construction owned by SPEC-CLI-001).
The `internal/router/category.go` already maps `CategoryAcademic` to
`[DocTypePaper, DocTypeRepo, DocTypeIssue]`; no router changes
required.

### 6.2 Package Layout

```
internal/adapters/arxiv/
├── arxiv.go                              # Adapter, New, Name, Capabilities, Healthcheck, interface assertion, rate-limit state
├── arxiv_test.go                         # Interface conformance + Capabilities determinism + New defaults
├── search.go                             # (*Adapter).Search hot path + waitForRateSlot
├── search_test.go                        # E2E + happy path + error categorisation tests
├── client.go                             # *http.Client, doRequest, categorizeStatus
├── client_test.go                        # categorizeStatus table + redirect allowlist
├── parse.go                              # parseFeed transform (Atom XML envelope, namespace-aware)
├── parse_test.go                         # Field mapping table tests + whitespace-collapse + namespace
├── errors.go                             # ErrInvalidQuery + ErrInvalidStart + parseRetryAfter helper
├── rate_test.go                          # REQ-ADP3-012 rate-limit serialisation tests
├── bench_test.go                         # BenchmarkParseFeed25Entries + TestMain (goleak)
└── testdata/
    ├── search_response.xml               # Happy path 25 entries
    ├── search_response_empty.xml         # Zero entries
    ├── search_response_pagination.xml    # start=0, totalResults=100
    ├── search_response_with_doi.xml      # Single entry with arxiv:doi
    ├── search_response_no_doi.xml        # Single entry without arxiv:doi
    ├── search_response_multi_author.xml  # 5 authors
    ├── search_response_multi_version.xml # <id> ending in v15
    ├── search_response_overshoot.xml     # start > totalResults; empty
    ├── search_response_400_error.xml     # Atom feed with error entry
    ├── search_response_latex_title.xml   # Title containing $E=mc^2$
    └── search_response_malformed.xml     # Truncated XML
```

[NOTE on duplication vs sharing] `parseRetryAfter`, `categorizeStatus`,
and the redirect-allowlist pattern duplicate the equivalents in
`internal/adapters/reddit/` and `internal/adapters/hn/`. This
duplication is INTENTIONAL in v0.1:

- The three helpers are short (≤ 30 LoC each) and the duplication cost
  is small relative to the cost of a premature shared package.
- ADP-001 was the FIRST adapter; ADP-002 was the SECOND; ADP-003 is
  the THIRD. "Rule of three" applies: with three implementations the
  shared shape is now visible, but the seven-way M3 ADP-* parallel
  development has not yet started — extracting now risks coupling the
  parallel SPECs to a shared package whose shape is still in flux.
- The seven-way M3 ADP-* parallelization will produce 9 adapter
  implementations within M3. After M3 lands, a refactor SPEC
  (SPEC-ADP-REFAC-001) MAY consolidate — see Open Question §11.6.
- The v0.1 acceptance includes a manual diff check that
  `parseRetryAfter` and `categorizeStatus` shapes match across the
  three packages (Reddit, HN, arXiv).

### 6.3 arXiv Atom Entry → NormalizedDoc Field Mapping

| Atom XML Field | NormalizedDoc Field | Transform |
|----------------|---------------------|-----------|
| `<id>` (e.g., `http://arxiv.org/abs/2403.12345v2`) minus prefix | `ID` | `strings.TrimPrefix(id, "http://arxiv.org/abs/")` → `"2403.12345v2"` |
| (constant) | `SourceID` | `"arxiv"` (matches `Name()`) |
| `<id>` (full URL) | `URL` | Use as-is (the canonical abstract page URL) |
| `<title>` | `Title` | `collapseWS(<title>)` = `strings.Join(strings.Fields(s), " ")` — collapses all-whitespace runs to single space |
| `<summary>` | `Body` | `collapseWS(<summary>)` |
| `truncateRunes(collapseWS(<summary>), 280)` | `Snippet` | First 280 runes; if longer, append "..."; if empty, derive from `collapseWS(<title>)` truncated similarly |
| `time.Parse(time.RFC3339, <published>)` | `PublishedAt` | The first-version submission date |
| (parse time) | `RetrievedAt` | `time.Now().UTC()` (set by `parseFeed` caller) |
| First `<author><name>` | `Author` | First author only; full list goes to Metadata |
| (constant `0.5`) | `Score` | Per §2.3 — arXiv has no per-paper score |
| (constant `""`) | `Lang` | arXiv has no per-paper language field |
| (constant `DocTypePaper`) | `DocType` | `pkg/types.DocTypePaper` per `pkg/types/capabilities.go:17` |
| (nil) | `Citations` | nil |
| (constructed) | `Metadata` | Map containing two key tiers. **REQUIRED keys** (consumers MAY rely on presence and stable shape; changes require a major-version bump of `Capabilities.Notes` and downstream coordination): `arxiv_id` (string, bare ID; duplicates `NormalizedDoc.ID`), `authors` ([]string of author names in submission order), `primary_category` (string from `<arxiv:primary_category term>`), `categories` ([]string from all `<category term>`), `published_at` (string, RFC 3339), `updated_at` (string, RFC 3339, from `<updated>`). REQ-ADP3-006 enforces these 6 as the contractual minimum. **OPTIONAL keys** (MAY be present; consumers SHALL NOT assume presence): `doi` (string from `<arxiv:doi>` when present), `journal_ref` (string from `<arxiv:journal_ref>`), `comment` (string from `<arxiv:comment>`), `pdf_url` (string from `<link title="pdf">`), `total_results` (int from feed-level `<opensearch:totalResults>` — surfaced ONLY on the FIRST returned doc per call). The LAST returned doc additionally gets `next_cursor` (REQUIRED on the last doc only, when `currentStart + len(entries) < totalResults`) encoded as `strconv.Itoa(currentStart + len(entries))`. |
| (constant) | `Hash` | `""` (consumers compute via `CanonicalHash()`) |

### 6.4 Type Sketches (illustrative; final shapes in run phase)

```go
// internal/adapters/arxiv/arxiv.go
package arxiv

import (
    "context"
    "fmt"
    "net"
    "net/http"
    "sync"
    "time"

    "github.com/elymas/universal-search/pkg/types"
)

const (
    defaultBaseURL           = "https://export.arxiv.org/api/query"
    defaultUserAgentTemplate = "usearch/%s (+https://github.com/elymas/universal-search)"
    defaultUAVersion         = "v0.1"
    defaultHealthcheckTarget = "export.arxiv.org:443"
    defaultMinInterval       = 3 * time.Second
    constantScore            = 0.5
)

type Options struct {
    BaseURL            string
    HTTPClient         *http.Client
    UserAgentVersion   string
    HealthcheckTarget  string
    MinRequestInterval time.Duration // default 3s; tests inject 0 or 1ms
}

type Adapter struct {
    httpClient        *http.Client
    baseURL           string
    userAgent         string
    healthcheckTarget string

    // Rate-limit state (REQ-ADP3-012). Per-instance; mutex-guarded.
    rateMu      sync.Mutex
    nextRequest time.Time
    minInterval time.Duration
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
    interval := opts.MinRequestInterval
    if interval == 0 {
        interval = defaultMinInterval
    }
    return &Adapter{
        httpClient:        client,
        baseURL:           base,
        userAgent:         ua,
        healthcheckTarget: target,
        minInterval:       interval,
    }, nil
}

func (a *Adapter) Name() string { return "arxiv" }

func (a *Adapter) Capabilities() types.Capabilities {
    return types.Capabilities{
        SourceID:          "arxiv",
        DisplayName:       "arXiv",
        DocTypes:          []types.DocType{types.DocTypePaper},
        SupportedLangs:    nil,
        SupportsSince:     true,
        RequiresAuth:      false,
        AuthEnvVars:       nil,
        RateLimitPerMin:   20,
        DefaultMaxResults: 25,
        Notes: "arXiv public no-auth API endpoint " +
            "(https://export.arxiv.org/api/query). 3-second courtesy " +
            "interval enforced per arXiv guideline (configurable via " +
            "Options.MinRequestInterval). sortBy=relevance hardcoded; " +
            "Score=0.5 constant (arXiv has no per-paper relevance score). " +
            "LaTeX pass-through (mathematical notation in titles and " +
            "abstracts is preserved as plain text; synthesis decides " +
            "whether to render). Filter keys: 'category' (e.g., " +
            "'cs.AI', 'math.GT'). SupportsSince=true is forward-compat " +
            "for date-range translation (deferred to P2 enhancement).",
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
// internal/adapters/arxiv/search.go (excerpt)

// waitForRateSlot blocks until the next outbound HTTP slot is available
// or ctx is cancelled. Returns nil on success; ctx.Err() on cancellation.
// Per §2.4 algorithm.
func (a *Adapter) waitForRateSlot(ctx context.Context) error {
    a.rateMu.Lock()
    wait := time.Until(a.nextRequest)
    a.nextRequest = time.Now().Add(a.minInterval)
    a.rateMu.Unlock()
    if wait <= 0 {
        return nil
    }
    t := time.NewTimer(wait)
    defer t.Stop()
    select {
    case <-t.C:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}

// buildSearchQuery prepends "cat:<value> AND " to q.Text when a category
// filter is present; otherwise returns q.Text verbatim.
func buildSearchQuery(q types.Query) string {
    for _, f := range q.Filters {
        if f.Key == "category" && f.Value != "" {
            return fmt.Sprintf("cat:%s AND %s", f.Value, q.Text)
        }
    }
    return q.Text
}
```

```go
// internal/adapters/arxiv/client.go (excerpt)
var allowedRedirectHosts = map[string]struct{}{
    "export.arxiv.org": {},
    "arxiv.org":        {},
}

func redirectAllowlist(req *http.Request, via []*http.Request) error {
    if len(via) >= 3 {
        return errors.New("arxiv: too many redirects (max 3)")
    }
    host := req.URL.Hostname()
    if _, ok := allowedRedirectHosts[host]; !ok {
        return fmt.Errorf("arxiv: cross-domain redirect rejected: %s", host)
    }
    return nil
}

func categorizeStatus(status int, retryAfter time.Duration, cause error) *types.SourceError {
    se := &types.SourceError{Adapter: "arxiv", HTTPStatus: status, Cause: cause}
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
// internal/adapters/arxiv/parse.go (excerpt)

// xml.Name with namespace URIs (NOT prefixes — Go's encoding/xml
// uses the namespace URI as the canonical match key).
var (
    arxivNamespaceURI      = "http://arxiv.org/schemas/atom"
    opensearchNamespaceURI = "http://a9.com/-/spec/opensearch/1.1/"
)

type atomFeed struct {
    XMLName       xml.Name    `xml:"http://www.w3.org/2005/Atom feed"`
    TotalResults  int         `xml:"http://a9.com/-/spec/opensearch/1.1/ totalResults"`
    StartIndex    int         `xml:"http://a9.com/-/spec/opensearch/1.1/ startIndex"`
    ItemsPerPage  int         `xml:"http://a9.com/-/spec/opensearch/1.1/ itemsPerPage"`
    Entries       []atomEntry `xml:"http://www.w3.org/2005/Atom entry"`
}

type atomEntry struct {
    XMLName         xml.Name        `xml:"http://www.w3.org/2005/Atom entry"`
    ID              string          `xml:"http://www.w3.org/2005/Atom id"`
    Published       string          `xml:"http://www.w3.org/2005/Atom published"`
    Updated         string          `xml:"http://www.w3.org/2005/Atom updated"`
    Title           string          `xml:"http://www.w3.org/2005/Atom title"`
    Summary         string          `xml:"http://www.w3.org/2005/Atom summary"`
    Authors         []atomAuthor    `xml:"http://www.w3.org/2005/Atom author"`
    PrimaryCategory categoryAttr    `xml:"http://arxiv.org/schemas/atom primary_category"`
    Categories      []categoryAttr  `xml:"http://www.w3.org/2005/Atom category"`
    DOI             string          `xml:"http://arxiv.org/schemas/atom doi"`
    JournalRef      string          `xml:"http://arxiv.org/schemas/atom journal_ref"`
    Comment         string          `xml:"http://arxiv.org/schemas/atom comment"`
    Links           []linkAttr      `xml:"http://www.w3.org/2005/Atom link"`
}

type atomAuthor struct {
    Name string `xml:"http://www.w3.org/2005/Atom name"`
}

type categoryAttr struct {
    Term string `xml:"term,attr"`
}

type linkAttr struct {
    Href  string `xml:"href,attr"`
    Rel   string `xml:"rel,attr"`
    Title string `xml:"title,attr"`
}

// parseFeed parses the Atom XML body and returns the docs and the next
// cursor (empty string when no further pages).
func parseFeed(body []byte, retrievedAt time.Time, currentStart int) ([]types.NormalizedDoc, string, error) {
    var feed atomFeed
    if err := xml.Unmarshal(body, &feed); err != nil {
        return nil, "", &types.SourceError{
            Adapter:  "arxiv",
            Category: types.CategoryPermanent,
            Cause:    fmt.Errorf("arxiv: malformed XML response: %w", err),
        }
    }
    docs := make([]types.NormalizedDoc, 0, len(feed.Entries))
    // ... per-entry transform per §6.3 mapping table
    // ... set next_cursor on last doc when currentStart + len(docs) < feed.TotalResults
    nextCursor := ""
    if currentStart+len(docs) < feed.TotalResults {
        nextCursor = strconv.Itoa(currentStart + len(docs))
    }
    return docs, nextCursor, nil
}
```

### 6.5 HTTP Client Construction Notes

Identical to ADP-001 §6.5 / ADP-002 §6.5:

- **Timeout**: 10 seconds total request deadline (default). Caller's
  ctx deadline takes precedence.
- **Redirect policy**: `CheckRedirect` enforces the allowlist
  `{export.arxiv.org, arxiv.org}` and caps at 3 hops.
- **Transport**: `reqid.NewTransport(http.DefaultTransport)` for
  request-ID propagation.
- **Headers per request**: `User-Agent: usearch/<version>
  (+https://github.com/elymas/universal-search)` and
  `Accept: application/atom+xml`. NO authentication header.

### 6.6 Observability Note

The arXiv adapter, like Reddit and HN, emits ZERO metrics, logs, and
spans of its own. ALL observability comes from the registry's
`wrappedAdapter` (`internal/adapters/registry.go:172-263`). This is
the sole-emitter discipline established in SPEC-CORE-001 §6.5 and
preserved verbatim by SPEC-ADP-001 / SPEC-ADP-002. The adapter's
responsibility is to return a correctly-categorised
`*types.SourceError` so the wrappedAdapter computes the right
`outcome` label via `types.OutcomeFromError(err)` (see
`pkg/types/errors.go:174-193`):

- `nil` → `"success"`
- `context.DeadlineExceeded` → `"timeout"`
- `CategoryRateLimited` → `"rate_limited"`
- `CategoryUnavailable` → `"unavailable"`
- `CategoryTransient` → `"transient"`
- `CategoryPermanent` / unknown → `"failure"`

### 6.7 MX Tag Plan

| File::Symbol | Tag | Reason |
|--------------|-----|--------|
| `arxiv.go::(*Adapter).Search` (defined in `search.go`) | `@MX:ANCHOR` | Sole entry point for all arXiv fanout calls. fan_in ≥ 3 (registry wrappedAdapter, FAN-001 fanout, tests). `@MX:REASON: contract boundary; signature change ripples to FAN-001 + CLI-001 + SYN-001`. `@MX:SPEC: SPEC-ADP-003`. |
| `parse.go::parseFeed` | `@MX:ANCHOR` | Every arXiv doc passes through this single transform. fan_in = 1 (Search) but invariant-bearing — bug here corrupts every NormalizedDoc returned. `@MX:REASON: NormalizedDoc field-mapping integrity gate; arXiv namespace handling correctness lives here`. |
| `arxiv.go::(*Adapter).waitForRateSlot` (defined in `search.go`) | `@MX:ANCHOR` | The rate-limit gate is invariant-bearing — it serialises ALL arXiv-bound HTTP requests and is the ONLY place the adapter holds shared mutable state. `@MX:REASON: removing the gate violates the arXiv 3-second guideline AND breaks REQ-ADP3-012 acceptance`. |
| `arxiv.go::constantScore` | `@MX:NOTE` | Documents the `Score=0.5` choice and tie-in to SPEC-IDX-001 RRF. The constant gets a doc-comment `@MX:NOTE` explaining §2.3 rationale and Open Question §11.5 revisit triggers. |
| `client.go::categorizeStatus` | `@MX:NOTE` | The HTTP-status-to-Category rosetta. Future contributors will look here first when a new HTTP code needs handling. |
| `client.go::doRequest` | `@MX:WARN` | Outbound network call. Redirect allowlist enforces SSRF safety boundary. `@MX:REASON: removing the CheckRedirect guard re-opens SSRF`. |
| `client.go::allowedRedirectHosts` map | `@MX:NOTE` | The 2-entry redirect allowlist. Adding a host requires a security review. |
| `parse.go::collapseWS` (helper) | `@MX:NOTE` | The whitespace-collapse pass over `<title>` and `<summary>` is a load-bearing invariant for Body/Title cleanliness. Future contributors who change to `strings.TrimSpace` (insufficient — leaves multi-space runs) or to `regexp.MustCompile(\\s+)` (slower, unnecessary) will degrade quality. |

All tags are `[AUTO]`-prefixed (agent-generated), include
`@MX:SPEC: SPEC-ADP-003`, and follow `code_comments: en` per
`.moai/config/sections/language.yaml`. Per-file hard limit
(3 ANCHOR + 5 WARN per `.moai/config/sections/mx.yaml`): respected.

### 6.8 Harness Level

Per `.moai/config/sections/harness.yaml` auto-routing: 12 EARS REQs
(10 × P0 + 2 × P1) + 4 NFRs touching 1 package (8 source files +
11 testdata fixtures) + zero cross-package edits + zero security/
payment/PII keywords + zero compose/env/config deltas =
**standard** harness level. Sprint Contract is OPTIONAL but
recommended. Evaluator profile `default` applies.

---

## 7. What NOT to Build (Exclusions)

[HARD] This SPEC explicitly excludes the following items. Each has a
known destination SPEC; this list prevents scope creep into ADP-003 and
reaffirms the third-adapter discipline.

- **Wrapping `openags/paper-search-mcp` for non-arXiv academic
  sources** → future `SPEC-ADP-003-MCP` (Open Question §11.1).
  Rationale: research.md §2.4 (path-A vs path-B comparison favours
  direct API for v0.1 due to reference-shape discipline, risk
  discipline, deployment simplicity, scope discipline).
- **Per-source customisations for GitHub, YouTube, Bluesky, X,
  SearXNG, Naver, Daum, KoreaNewsCrawler, RSS, Polymarket** →
  SPEC-ADP-004..009 (M3, parallelizable per
  `.moai/project/roadmap.md:122-123`).
- **Retry orchestration** (exponential backoff, circuit breaker,
  per-source health-state tracking, jitter) → SPEC-FAN-001 v0.1 ships
  zero-retry per its D6; future SPEC-FAN-001-RETRY may add. Adapter
  is one-shot per call.
- **Response caching** (in-process LRU, Redis, on-disk fixture cache)
  → SPEC-CACHE-001 (M3). Adapter is stateless except for the
  rate-limit gate (which is correctness state, not caching state).
- **Result ranking, deduplication, RRF fusion across adapters** →
  SPEC-IDX-001 (M3). SPEC-FAN-001 also performs URL-canonicalization-
  based dedup. Adapter returns arXiv-relevance order with `Score=0.5`
  constant; cross-adapter ranking is fusion's job.
- **arXiv `id_list` lookup mode** → out of v0.1 scope; future P2
  enhancement.
- **arXiv `search_by_date` / chronological sort** → out of v0.1;
  hardcoded `sortBy=relevance`. Future filter `Query.Filters[Key="sort"]`
  may surface this.
- **Date-range filtering via `submittedDate:[…]` search-query syntax**
  → out of v0.1; `Capabilities.SupportsSince=true` is declared
  (forward-compat) but the implementation is deferred. Open Question
  §11.8.
- **Citation count integration** (Semantic Scholar API enrichment) →
  out of v0.1; Score=0.5 constant. Open Question §11.5.
- **Author affiliation extraction** (`<arxiv:affiliation>`) → out of
  v0.1; future patch SPEC. Open Question §11.7.
- **PDF download integration** → SPEC-CACHE-001 (M3) owns 5-phase
  access fallback that may consume the PDF URL exposed in
  `Metadata["pdf_url"]`.
- **OAI-PMH bulk-export protocol** → out of v0.1; OAI-PMH is for
  full-archive metadata harvesting, not search.
- **Live network integration tests in CI** → out of v0.1; httptest +
  golden fixtures only.
- **Per-adapter custom Prometheus metrics** → would require amending
  SPEC-OBS-001's allowlist. Out of v0.1.
- **Korean-locale handling for arXiv** → SPEC-IDX-003 (M3); arXiv
  abstracts are English-dominant; adapter sets `Lang=""`.
- **LaTeX rendering / stripping** → out of v0.1; pass-through. Synthesis
  (SPEC-SYN-001) decides.
- **Cross-adapter helper extraction** (sharing `parseRetryAfter`,
  `categorizeStatus`, `redirectAllowlist` between Reddit, HN, arXiv
  packages) → out of v0.1. Open Question §11.6. Refactor SPEC after M3
  parallelisation lands.
- **Streaming Search results** (channel-based incremental delivery) →
  SPEC-SYN-004 (M4) if measured value.
- **Author-specific filtering** (arXiv's `au:` search-query prefix
  exposed via structured filter key) → out of v0.1; users can write
  `au:vaswani` in `q.Text` themselves.

---

## 8. TDD Plan

Development follows RED-GREEN-REFACTOR per
`quality.development_mode: tdd` (`.moai/config/sections/quality.yaml`).
Representative RED-phase tests, written before implementation, grouped
by REQ. Total: ~40 tests covering REQ-ADP3-001..012 + NFRs. Coverage
target: 85% per `quality.test_coverage_target`. Benchmarks do not count
toward coverage.

| # | Test | Layer | REQ | Assertion |
|---|------|-------|-----|-----------|
| 1 | `TestAdapterName` | `arxiv_test.go` | REQ-ADP3-001 | `(*Adapter).Name() == "arxiv"` |
| 2 | `TestAdapterImplementsInterface` | `arxiv_test.go` | REQ-ADP3-001 | Compile-time `var _ types.Adapter = (*Adapter)(nil)` succeeds |
| 3 | `TestCapabilitiesDeterministic` | `arxiv_test.go` | REQ-ADP3-001 | Two consecutive `Capabilities()` calls return `reflect.DeepEqual` results |
| 4 | `TestCapabilitiesShape` | `arxiv_test.go` | REQ-ADP3-001 | All 9 documented field values match (SourceID, DisplayName, DocTypes, RequiresAuth, AuthEnvVars, SupportsSince=true, RateLimitPerMin=20, DefaultMaxResults=25, plus Notes substring contains) |
| 5 | `TestHealthcheckSucceeds` | `arxiv_test.go` | REQ-ADP3-001 | TCP dial against test loopback succeeds |
| 6 | `TestNewAppliesMinRequestIntervalDefault` | `arxiv_test.go` | REQ-ADP3-001 | `New(Options{})` substitutes `minInterval = 3*time.Second` |
| 7 | `TestSearchHappyPath25Entries` | `search_test.go` | REQ-ADP3-002, REQ-ADP3-006 | 25 NormalizedDocs returned; each `Validate()` returns nil |
| 8 | `TestSearchURLParametersIncludeAllRequired` | `search_test.go` | REQ-ADP3-002 | Captured URL has `search_query`, `max_results`, `sortBy=relevance`, `sortOrder=descending` |
| 9 | `TestSearchClampsMaxResultsTo100` | `search_test.go` | REQ-ADP3-002 | q.MaxResults=500 → URL has `max_results=100` |
| 10 | `TestSearchDefaultsMaxResultsTo25` | `search_test.go` | REQ-ADP3-002 | q.MaxResults=0 → URL has `max_results=25` |
| 11 | `TestSearchOmitsStartWhenCursorEmpty` | `search_test.go` | REQ-ADP3-002 | q.Cursor="" → URL has no `start` param (or `start=0`) |
| 12 | `TestSearchSetsStartWhenCursorPresent` | `search_test.go` | REQ-ADP3-002 | q.Cursor="50" → URL contains `&start=50` |
| 13 | `TestSearchOvershoot` | `search_test.go` | REQ-ADP3-002 | start > totalResults → empty docs, no error |
| 14 | `TestSearchHTTP429WithIntegerRetryAfter` | `search_test.go` | REQ-ADP3-003 | `Retry-After: 30` → SourceError.RetryAfter==30s |
| 15 | `TestSearchHTTP429WithHTTPDateRetryAfter` | `search_test.go` | REQ-ADP3-003 | HTTP-date 30s ahead → RetryAfter ∈ (25s, 35s) |
| 16 | `TestSearchHTTP429NoRetryAfterDefaults5s` | `search_test.go` | REQ-ADP3-003 | No header → RetryAfter==5s |
| 17 | `TestSearchHTTP429RetryAfterCapped60s` | `search_test.go` | REQ-ADP3-003 | `Retry-After: 999` → RetryAfter==60s |
| 18 | `TestSearchHTTP429NoInternalRetry` | `search_test.go` | REQ-ADP3-003 | Server request count == 1 |
| 19 | `TestSearchHTTP400ValidationError` | `search_test.go` | REQ-ADP3-004 | 400 + Atom error feed body → ErrPermanent + HTTPStatus=400 |
| 20 | `TestSearchHTTP4xx` | `search_test.go` | REQ-ADP3-004 | Table over 401/403/404 → ErrPermanent + matching HTTPStatus |
| 21 | `TestSearchHTTP5xx` | `search_test.go` | REQ-ADP3-005 | Table over 500/503 → ErrSourceUnavailable + matching HTTPStatus |
| 22 | `TestSearchConnectionRefused` | `search_test.go` | REQ-ADP3-005 | `errors.Is(err, types.ErrSourceUnavailable)`; HTTPStatus==0 |
| 23 | `TestSearchUnavailablePreservesUnderlyingError` | `search_test.go` | REQ-ADP3-005 | `errors.Unwrap(srcErr).Error()` contains inner cause text |
| 24 | `TestParseFeedFieldMapping` | `parse_test.go` | REQ-ADP3-006 | Table over 5 fixtures; every documented field maps correctly |
| 25 | `TestParseFeedIDStripPrefix` | `parse_test.go` | REQ-ADP3-006 | `<id>http://arxiv.org/abs/2403.12345v2</id>` → ID="2403.12345v2" |
| 26 | `TestParseFeedMultiVersionID` | `parse_test.go` | REQ-ADP3-006 | `<id>` ending in `v15` → ID preserves `v15` suffix |
| 27 | `TestParseFeedWhitespaceCollapse` | `parse_test.go` | REQ-ADP3-006 | Table over 5 input shapes (newlines, multi-space, leading/trailing, control chars, Unicode) |
| 28 | `TestParseFeedScoreConstant` | `parse_test.go` | REQ-ADP3-006 | Every doc has Score == 0.5 |
| 29 | `TestParseFeedDOIInArxivNamespace` | `parse_test.go` | REQ-ADP3-006 | `<arxiv:doi>` value populates `Metadata["doi"]` |
| 30 | `TestParseFeedNoDOIOmitsKey` | `parse_test.go` | REQ-ADP3-006 | Entry without DOI → no `Metadata["doi"]` key |
| 31 | `TestParseFeedAuthorsList` | `parse_test.go` | REQ-ADP3-006 | 5 authors → `Metadata["authors"]` = []string of 5; Author = first |
| 32 | `TestParseFeedPaginationCursor` | `parse_test.go` | REQ-ADP3-006 | start=0, totalResults=100, len=25 → next_cursor="25" on last doc |
| 33 | `TestParseFeedNoCursorOnLastPage` | `parse_test.go` | REQ-ADP3-006 | start=80, totalResults=100, len=20 → no next_cursor anywhere |
| 34 | `TestParseFeedHashEmpty` | `parse_test.go` | REQ-ADP3-006 | Every NormalizedDoc.Hash == "" |
| 35 | `TestParseFeedMetadataKeys` | `parse_test.go` | REQ-ADP3-006 | All 6 required Metadata keys present |
| 36 | `TestParseFeedMalformedXML` | `parse_test.go` | REQ-ADP3-006 | Truncated XML → `*SourceError{Category: CategoryPermanent}` |
| 37 | `TestSearchCategoryFilterAdded` | `search_test.go` | REQ-ADP3-007 | Filters=[{category, "cs.AI"}] → search_query=cat:cs.AI%20AND%20transformer |
| 38 | `TestSearchCategoryFilterAbsent` | `search_test.go` | REQ-ADP3-007 | Filters=nil → search_query=transformer verbatim |
| 39 | `TestSearchCategoryFilterEmpty` | `search_test.go` | REQ-ADP3-007 | Filters=[{category, ""}] → no prepend |
| 40 | `TestSearchUnknownFilterIgnored` | `search_test.go` | REQ-ADP3-007 | Filters=[{nsfw, "true"}] → no prepend |
| 41 | `TestSearchEmptyQueryRejectedNoHTTP` | `search_test.go` | REQ-ADP3-008 | Table over empty/whitespace q.Text → ErrPermanent + zero requests + zero rate-slot consumed |
| 42 | `TestSearchInvalidStartRejectedNoHTTP` | `search_test.go` | REQ-ADP3-008 | Table over invalid cursors → ErrPermanent + zero requests |
| 43 | `TestSearchSetsCustomUserAgent` | `client_test.go` | REQ-ADP3-009 | UA starts with "usearch/" + contains URL |
| 44 | `TestSearchSetsAcceptAtomXML` | `client_test.go` | REQ-ADP3-009 | `Accept: application/atom+xml` header present |
| 45 | `TestSearchUserAgentVersionConfigurable` | `client_test.go` | REQ-ADP3-009 | Options override propagates to UA header |
| 46 | `TestSearchFollowsAllowlistRedirect` | `client_test.go` | REQ-ADP3-010 | 302 within allowlist followed |
| 47 | `TestSearchRejectsCrossDomainRedirect` | `client_test.go` | REQ-ADP3-010 | 302 to attacker.com → ErrPermanent + "cross-domain redirect" message |
| 48 | `TestSearchRejectsRedirectChainOver3` | `client_test.go` | REQ-ADP3-010 | 4-hop chain rejected with "too many redirects" |
| 49 | `TestSearchConcurrentSafe` | `search_test.go` | REQ-ADP3-011, NFR-ADP3-003 | 50 goroutines × shared `*Adapter` × 1 stub server, race-clean (`-race`); MinRequestInterval=0 |
| 50 | `TestSearchRateLimitInterval` | `rate_test.go` | REQ-ADP3-012 | 3 sequential calls, MinRequestInterval=10ms; total elapsed ∈ [20ms, 50ms]; 3 requests observed |
| 51 | `TestSearchRateLimitCtxCancel` | `rate_test.go` | REQ-ADP3-012 | Cancelled ctx during wait → returns within 5ms with errors.Is(err, context.Canceled) |
| 52 | `TestSearchRateLimitPerInstance` | `rate_test.go` | REQ-ADP3-012 | Two `*Adapter` instances do not serialise across each other |
| 53 | `TestParseRetryAfterTable` | `client_test.go` | REQ-ADP3-003 | Table over 6 inputs (int, HTTP-date, missing, malformed, > 60, negative) |
| 54 | `TestCategorizeStatusTable` | `client_test.go` | REQ-ADP3-003/004/005 | Truth table over 7 status codes (200/400/401/404/429/500/503/0) → expected Category |
| 55 | `TestSearchE2ELatencyStubP95` | `search_test.go` | NFR-ADP3-002 | 100 invocations against stub (MinRequestInterval=0); p95 ≤ 200ms |
| 56 | `TestSearchNoGoroutineLeakOnCancel` | `search_test.go` | NFR-ADP3-004 | `goleak.VerifyNone(t)` after mid-flight ctx cancel |
| 57 | `BenchmarkParseFeed25Entries` | `bench_test.go` | NFR-ADP3-001 | Median of 5 `-count` runs at `-benchtime=10x` is ≤ 5ms per op; allocs/op ≤ 700 |
| 58 | `TestMain` (goleak.VerifyTestMain) | `bench_test.go` | NFR-ADP3-004 | Package-level goroutine leak check |

RED-GREEN-REFACTOR per requirement:

1. RED: Write failing test for REQ-ADP3-N.
2. GREEN: Implement minimal code to pass.
3. REFACTOR: Tidy; extract shared helpers if they remove duplication
   WITHIN the package; keep file sizes manageable (target each `.go`
   file < 200 LoC excluding tests).

Greenfield note: `internal/adapters/arxiv/` does not exist. There is no
behaviour to preserve; no characterization tests needed.

---

## 9. Dependencies

### 9.1 Upstream SPEC Dependencies

- **SPEC-CORE-001 (implemented; merged commit f728aa2)**: provides
  `pkg/types.Adapter`, `pkg/types.Capabilities` (including
  `DocTypePaper` constant at `pkg/types/capabilities.go:17`),
  `pkg/types.Query`, `pkg/types.NormalizedDoc`, `*types.SourceError`,
  `types.OutcomeFromError`, `types.DocType` enum,
  `internal/adapters.Registry` with wrappedAdapter sole-emitter
  pattern, `internal/adapters/noop` reference shape. HARD dep.
- **SPEC-OBS-001 (implemented)**: provides `obs.Obs` bundle,
  `internal/obs/reqid.NewTransport` for request-ID propagation,
  `AdapterCalls{adapter,outcome}` and `AdapterCallDuration{adapter}`
  collectors with `adapter` and `outcome` already in cardinality
  allowlist. SOFT dep — adapter is nil-safe via the registry's
  nil-guards. The `adapter="arxiv"` cardinality value fits within the
  V1 14-adapter ceiling per SPEC-OBS-001 NFR-OBS-002.
- **SPEC-IR-001 (implemented; merged commit 8a20b68)**: documents the
  consumer contract for `Capabilities` (REQ-IR-008 selects AdapterSet
  by intersecting `categoryEligibleDocTypes` with `SupportedLangs`).
  ADP-003's `Capabilities()` shape (DocTypes=[DocTypePaper],
  SupportedLangs=nil) determines which routing categories the arXiv
  adapter will be selected for — `CategoryAcademic` per
  `internal/router/category.go:97`. SOFT dep — IR-001 lookups happen
  at startup; ADP-003 just declares its capability. Note: the router
  test at `internal/router/router_test.go:79, 411, 443` already uses
  the adapter name `"arxiv"` as a stub — this SPEC makes the name
  canonical with a real implementation.

### 9.2 Parallelizable

- **SPEC-ADP-004 through SPEC-ADP-009 (M3)**: all six remaining M3
  adapter SPECs can develop in parallel with ADP-003, gated only on
  the now-approved SPEC-FAN-001. Each will copy the ADP-003 shape (or
  ADP-001/002 shape) and customise for its own source.
- **SPEC-IDX-001 (M3)**: can plan in parallel; IDX-001 consumes
  `[]NormalizedDoc` from FAN-001's Result. ADP-003 doesn't add new
  constraints beyond Score=0.5 (which is already a documented FAN-001
  input contract).
- **SPEC-CACHE-001 (M3)**: can plan in parallel; CACHE-001 wraps
  fanout in a 5-phase access fallback harness. ADP-003's
  `Metadata["pdf_url"]` may be a Phase 0 input.

### 9.3 Downstream Blocked SPECs

ADP-003 explicitly blocks zero SPECs (`blocks: []` in frontmatter). Its
completion contributes to the M3 exit criterion
(`.moai/project/roadmap.md:150`, "≥5 adapters fused") but no
single SPEC is gated on ADP-003 specifically.

### 9.4 External Dependencies (run-phase pins)

**Zero new Go module dependencies.** ADP-003 uses only:

- Go stdlib: `context`, `encoding/xml`, `errors`, `fmt`, `math`, `net`,
  `net/http`, `net/url`, `strconv`, `strings`, `sync`, `time`,
  `unicode`, `unicode/utf8`
- `pkg/types` (already pinned via SPEC-CORE-001)
- `internal/obs/reqid` (already pinned via SPEC-OBS-001)
- Test-only: `go.uber.org/goleak` (for NFR-ADP3-004) — already pinned
  indirect via `go.mod:30` (added by SPEC-ADP-001 run-phase).

Note: `encoding/xml` is in the Go stdlib; no third-party XML library
added. The Atom 1.0 format is well-handled by the stdlib's namespace-
aware XML decoder.

---

## 10. Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| arXiv API contract drift (new namespace, new element added) | Low | Medium | `encoding/xml` tolerates unknown elements; test fixtures pinned to documented shape. arXiv API has been stable since at least 2008 per Wikipedia. |
| `encoding/xml` heavier than `encoding/json` (parse alloc/op floor higher than ADP-001/002) | Medium | Low | NFR-ADP3-001 declares `allocs/op ≤ 700` as a starting target (vs ≤500 in ADP-001 amended); empirical baseline established in run-phase iteration. The pkg/types contract floor (~17 allocs/doc for Metadata map) dominates either way. |
| Rate-limit state mutex contention under heavy concurrent load | Low | Low | Mutex held only for ~10ns (compute+update); actual wait is outside lock. 50-goroutine race test (NFR-ADP3-003) verifies. |
| Title/summary whitespace handling breaks on adversarial input (deeply nested newlines, control chars) | Medium | Low | `strings.Fields` collapses any whitespace run; control chars pass through (Go strings are byte-indexed; UTF-8 preserved). Tested via `TestParseFeedWhitespaceCollapse` over 5 input shapes. |
| Author parsing breaks when `<author>` has no `<name>` child or has multiple `<name>` children (malformed) | Medium | Low | Defensive: skip `<author>` without `<name>`; take first `<name>` if multiple. Test fixture covers. |
| DOI absent from most papers (only ~30% have author-supplied DOI) | High (but expected) | Low | DOI is OPTIONAL Metadata key; consumers cannot rely on its presence. Documented in §6.3 mapping table and REQ-ADP3-006. |
| `<published>` and `<updated>` differ for multi-version papers — choosing wrong one as PublishedAt | Low | Low | DESIGN DECISION: PublishedAt = first-version submission date (`<published>`). The `<updated>` value goes to Metadata `updated_at` for consumers who care. |
| Cursor parsing fails on adversarial input (huge integers, negative, non-numeric) | Low | Low | `strconv.Atoi` catches non-numeric; explicit guard against negative; REQ-ADP3-008 rejects with ErrInvalidStart. |
| Score=0.5 (constant) breaks SPEC-IDX-001 RRF expectations | Low | Medium | RRF uses RANK not SCORE per FAN-001 §6.1. Equal scores don't break RRF; tie-breaking happens via rank within source. Open Question §11.5 documents revisit triggers. |
| Hash collisions across arxiv and other sources for shared external URL (e.g., ResearchGate copy of an arXiv paper) | Low | Medium | `CanonicalHash` includes `SourceID` prefix per `pkg/types/normalized_doc.go:96-99` — never collides cross-adapter. |
| Atom XML namespace handling complexity | Medium | Medium | `encoding/xml` honors `xmlns` declarations natively via `xml.Name{Space, Local}`. Test fixtures include all three namespaces; parsing tested via `TestParseFeedDOIInArxivNamespace` and the field-mapping table over 5 fixtures. The Go stdlib's namespace-URI-as-key convention is well-documented. |
| `time.Parse(time.RFC3339, "...-04:00")` timezone handling | Low | Low | RFC 3339 parser handles timezone offsets natively. All times converted to UTC via `.UTC()` after parse. Tested via `TestParseFeedFieldMapping` with multiple timezone fixtures. |
| arXiv returns Atom feed with zero entries on overshoot (`start` > totalResults) | Low | Low | Adapter returns `(nil, nil)` (no error). Caller sees pagination terminated. Tested via `TestSearchOvershoot`. |
| Default Go `net/http` User-Agent borderline-rejected (no documented evidence, but defensive) | Low | Low | REQ-ADP3-009 makes custom UA mandatory (mirrors ADP-001/002 convention). |
| HTTP timeout (10s) too aggressive for arXiv during incidents | Low | Low | Configurable via `Options.HTTPClient`; default 10s aligns with NFR-ADP3-002 stub p95 200ms × 50× safety margin. |
| LaTeX strings in `<summary>` confuse downstream synthesis | Medium | Low | Pass-through; documented in `Capabilities.Notes`. SPEC-SYN-001 decides whether to render or strip. NOT an adapter concern. |
| Multi-rune special characters in arxiv_id (versioning: v1, v2, ..., v15+) | Low | Low | The `vN` suffix is plain ASCII; `strings.TrimPrefix` handles any version count. Tested via 3 fixtures (no version, v1, v15). |
| Rate-limit interval too long for interactive UX (3 seconds is annoying) | Medium | Medium | Configurable via `Options.MinRequestInterval`. Production deployments may tune lower if they accept the operational courtesy risk. The default matches arXiv's published guideline. Open Question §11.4. |
| Two adapter instances spawned mistakenly (e.g., test harness anti-pattern) leading to state divergence | Low | Low | Per-instance state by design (REQ-ADP3-012 acceptance includes `TestSearchRateLimitPerInstance`). The registry creates ONE instance per source, so production has no multi-instance concern. |
| Duplication of helpers with reddit and hn packages introduces drift over time | Medium | Low | Acknowledged. See §11.6 — refactor SPEC after M3 lands the next 4+ adapters. v0.1 acceptance includes a manual diff check that `parseRetryAfter` and `categorizeStatus` shapes match across the three packages. |

---

## 11. Open Questions

These are explicitly UNRESOLVED at SPEC-approval time. Each has a
recommended default and a one-line resolution owner. They do NOT block
SPEC approval.

1. **paper-search-mcp wrapping for non-arXiv academic sources**
   (PubMed, bioRxiv, medRxiv, Google Scholar, Semantic Scholar,
   Crossref, OpenAlex). **Recommended default**: defer to future
   `SPEC-ADP-003-MCP` if measured demand for non-arXiv academic sources
   warrants the Python sidecar overhead. Rationale in research.md
   §2.4 (six factors: reference-shape discipline, risk discipline,
   deployment simplicity, scope discipline, future composability, zero
   new module dependencies). **Resolution owner**: SPEC-ADP-003-MCP
   author (TBD post-V1 if traffic warrants).

2. **Score population strategy**. Constant `0.5` (chosen in §2.3)
   versus position-derived `1.0 - rank/maxResults` versus
   citation-count integration. **Recommended default**: constant `0.5`
   for v0.1. Revisit after SPEC-IDX-001 RRF integration measurements
   indicate position-derived signal adds value. **Resolution owner**:
   SPEC-IDX-001 author.

3. **`max_results` clamp ceiling**. arXiv allows up to 2000 per call;
   ADP-003 clamps at 100 (matching ADP-001/002 uniform ceiling).
   **Recommended default**: clamp at 100 (uniform adapter behaviour);
   the per-page bandwidth saving is significant only for batch-export
   use cases not in V1 scope. **Resolution owner**: SPEC-FAN-001 author
   may revisit if pagination pressure becomes a measured concern.

4. **MinRequestInterval default value**. arXiv guideline is 3 seconds;
   the adapter declares this as the default. Should production
   actually sleep 3s between every Search invocation, or trust the
   infrequent nature of search workloads to stay under the limit
   organically? **Recommended default**: enforce 3s in production;
   tests inject `0` or `1ms` for fast concurrent-safety verification.
   **Resolution owner**: SPEC-FAN-001a author may add a per-source
   token-bucket layer that subsumes the per-adapter wait. Operators
   may tune via config.

5. **Citation count integration**. Should ADP-003 enrich each doc with
   citation counts via Semantic Scholar API? **Recommended default**:
   NO in v0.1 — adds an external dep, another rate-limit surface, and
   another auth path. Score=0.5 constant is sufficient for RRF. A
   future SPEC-ENRICH-001 may add post-processing enrichment.
   **Resolution owner**: SPEC-IDX-001 author may request if RRF
   measurements indicate citation signal helps ranking quality.

6. **Cross-adapter helper extraction** (sharing `parseRetryAfter`,
   `categorizeStatus`, `redirectAllowlist` between Reddit, HN, arXiv,
   and the future six M3 adapters). **Recommended default**: defer
   until M3 lands the remaining four adapters (ADP-004..009 minus the
   already-built ADP-007 placeholder). At that point, with 7-9 adapter
   implementations visible, `SPEC-ADP-REFAC-001` may consolidate. v0.1
   ships duplicate helpers in each adapter package. **Resolution
   owner**: SPEC-ADP-REFAC-001 author (TBD post-M3).

7. **Author affiliation extraction** (`<arxiv:affiliation>`). Some
   downstream synthesis (e.g., organisation-keyword search) may want
   it. **Recommended default**: ignore in v0.1; add as Metadata key
   `affiliations` in a future patch SPEC. **Resolution owner**:
   SPEC-SYN-001 author.

8. **`SupportsSince=true` translation to `submittedDate:[…]`
   search-query syntax**. The capability declaration is
   forward-compat, but v0.1 does NOT translate `Query.Filters`
   date-range to arXiv search-query syntax. **Recommended default**:
   defer to a P2 enhancement once consumer needs are clearer; document
   the gap in `Capabilities.Notes`. **Resolution owner**: run-phase
   implementer; SPEC-IR-001a author may surface a structured date-
   range filter that ADP-003 consumes.

---

## 12. References

### External (URL-cited; verified per research.md §9)

- https://github.com/openags/paper-search-mcp — paper-search-mcp MCP
  server (rejected as v0.1 dependency per research.md §2.4 rationale;
  pattern reference only). WebFetch-verified 2026-05-04.
- https://info.arxiv.org/help/api/index.html — arXiv API documentation
  index.
- https://info.arxiv.org/help/api/user-manual.html — arXiv API user
  manual; canonical reference for endpoint, parameters, response
  envelope, rate-limit guideline. WebFetch-verified 2026-05-04.
- https://en.wikipedia.org/wiki/ArXiv — arXiv background
  (Cornell-hosted, 2.5M+ preprints, online since 1991, API stable
  since 2008).
- https://www.w3.org/2005/Atom — Atom 1.0 specification (referenced
  via Go's `encoding/xml` namespace handling).
- RFC 7231 §7.1.3 Retry-After header semantics — basis for
  REQ-ADP3-003 parser (inherited from ADP-001/002).

### Internal (file:line cited)

- `.moai/specs/SPEC-ADP-003/research.md` — full research artifact for
  this SPEC.
- `.moai/specs/SPEC-ADP-001/spec.md` — reference adapter SPEC; this
  SPEC inherits structure verbatim (1215 lines).
- `.moai/specs/SPEC-ADP-002/spec.md` — second-adapter SPEC reference
  (1189 lines); HN's HTML-strip pattern is a reference for any
  rich-text body adapter (ADP-003 uses whitespace-collapse instead of
  HTML-strip since arXiv doesn't have HTML in `<title>`/`<summary>`).
- `.moai/specs/SPEC-FAN-001/spec.md` — M3 fanout gateway (status:
  approved); REQ-FAN-002 dispatch loop consumes this adapter.
- `.moai/specs/SPEC-CORE-001/spec.md` — Adapter / Capabilities /
  Query / NormalizedDoc / SourceError contract.
- `.moai/specs/SPEC-OBS-001/spec.md` — observability bundle and
  cardinality discipline.
- `.moai/specs/SPEC-IR-001/spec.md` — `Capabilities` consumer
  contract (REQ-IR-008).
- `pkg/types/adapter.go:28-45` — Adapter interface 4-method shape.
- `pkg/types/capabilities.go:17` — `DocTypePaper` constant (the slot
  this adapter fills).
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
- `internal/adapters/reddit/search.go` — Search hot path pattern
  (mirrored by ADP-003 search.go).
- `internal/adapters/reddit/parse.go` — JSON → NormalizedDoc transform
  pattern (XML equivalent in ADP-003 parse.go).
- `internal/adapters/reddit/client.go` — HTTP client + redirect
  allowlist pattern.
- `internal/adapters/reddit/score.go` — Tanh score formula (NOT
  reused; ADP-003 uses Score=0.5 constant per §2.3).
- `internal/adapters/reddit/errors.go` — `parseRetryAfter` helper
  pattern (reused as-is).
- `internal/adapters/reddit/bench_test.go` — `goleak.VerifyTestMain`
  pattern (mirrored by ADP-003 bench_test.go).
- `internal/adapters/hn/parse.go` — HN parseHits pattern (XML decoder
  is the equivalent transform layer for ADP-003).
- `internal/router/category.go:16-17` — `CategoryAcademic` constant.
- `internal/router/category.go:97` — `CategoryAcademic` eligible
  DocTypes = `[DocTypePaper, DocTypeRepo, DocTypeIssue]` — ADP-003
  fills the `DocTypePaper` slot.
- `internal/router/router.go:258-294` — `selectAdapterSet` algorithm
  (consumes `Capabilities.DocTypes`).
- `internal/router/router_test.go:79, 411, 443` — `arxiv` adapter name
  already used as a stub; ADP-003 makes it canonical.
- `internal/llm/client.go:31-65` — HTTP client + reqid Transport
  pattern (inherited from ADP-001 reference shape).
- `.moai/project/roadmap.md:48` — M3 row "SPEC-ADP-003 | arXiv +
  paper-search adapters | wrap `openags/paper-search-mcp`" (this SPEC
  reinterprets the wrap mention; see research.md §2.4).
- `.moai/project/roadmap.md:122-123` — M3 parallelization plan (M3
  ADP-* SPECs gated on SPEC-FAN-001 — SATISFIED).
- `.moai/project/roadmap.md:150` — M3 exit criterion ("`usearch
  query` returns fused results across ≥5 adapters").
- `.moai/project/structure.md:18-22` — `internal/adapters/arxiv/`
  reservation.
- `.moai/project/tech.md:102` — Adapter strategy row "arXiv |
  OAI-PMH + arXiv Search API | none | 3s between req".
- `.moai/project/tech.md:103` — "Paper search | wrap
  openags/paper-search-mcp | per-source | varies | Crossref / OpenAlex
  / Semantic Scholar" (deferred per research.md §2.4).
- `go.mod:30` — `go.uber.org/goleak v1.3.0` (NFR-ADP3-004 requirement).
- `.moai/config/sections/quality.yaml` — `development_mode: tdd`,
  `test_coverage_target: 85`.
- `.moai/config/sections/harness.yaml` — auto-routing to standard
  level.
- `.moai/config/sections/language.yaml` — `documentation: en`,
  `code_comments: en`.

---

*End of SPEC-ADP-003 v0.1 (DRAFT)*
