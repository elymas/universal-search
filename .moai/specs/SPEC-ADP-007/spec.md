---
id: SPEC-ADP-007
title: SearXNG Bridge Adapter
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
depends_on: [SPEC-CORE-001, SPEC-OBS-001, SPEC-IR-001, SPEC-BOOT-001]
blocks: []
---

# SPEC-ADP-007: SearXNG Bridge Adapter

## HISTORY

- 2026-05-04 (iteration 2 — plan-auditor cycle 1, limbowl via manager-spec):
  Audit identified 6 HIGH and 7 MEDIUM concerns; all addressed inline
  in this revision. HIGH fixes: (H1) Capabilities.DocTypes narrowed
  from `[Article, Post, Other]` to `[Article]` to align with the
  per-doc `DocType=DocTypeArticle` constant in §6.3. The IR-001
  selectAdapterSet logic still selects SearXNG for any CategoryWeb
  query because `[Article]` intersects with the eligible set
  `{Article, Post, Other}` per `internal/router/category.go:93`. The
  3-element advertisement was over-promising. (H2) §2.3 score-range
  claim relaxed: "the SearXNG `score` field is a positive float; the
  upper bound is engine-aggregation-dependent and not formally
  documented. The clamp guarantees `[0.0, 1.0]` codomain regardless
  of upstream value." Removes the unsupported `[0.0, ~10.0]` claim.
  (H3) `Capabilities.RateLimitPerMin` lowered from 60 to 0 ("no
  external rate limit; the local instance is operator-controlled via
  `server.limiter` in settings.yml"). The 60/min figure was
  fabricated; SearXNG itself has no published rate. (H4) §2.1 item a
  expanded to specify `healthcheckTarget` derivation explicitly:
  default extracted from `baseURL` host:port via
  `healthcheckHostFromBase`; operator-injected via
  `Options.HealthcheckTarget`; tests inject loopback. (H5) Pageno
  semantics unified — REQ-ADP7-002 + §2.1 item b agree: "pageno
  parameter is set when q.Cursor parses to integer ≥ 1; cursor='1'
  produces explicit pageno=1 in the URL; cursor='' (default) omits
  the pageno parameter entirely (SearXNG defaults to pageno=1
  server-side)." Removes the previous inconsistency between item b's
  ">1 only" and REQ-ADP7-002's "non-empty only". (H6) §6.3 mapping
  table now documents Author hardcoded-empty rationale: "v0.1 hardcodes
  Author='' because the default `general` category does not surface
  per-result author. Per-engine author extraction deferred to a
  follow-up SPEC if measured value warrants." MEDIUM fixes: (M1)
  added `TestParseSearchIDDeterminism` to TDD plan (test #52). (M2)
  added parseRetryAfter sketch in §6.4. (M3) REQ-ADP7-005 acceptance
  text adds clarification: "pagination termination is detected by
  EMPTY results on the next call (caller passes the surfaced
  next_cursor; if SearXNG returns zero results, the iteration ends)."
  Mirrors HN ADP-002 behaviour. (M4) REQ-ADP7-005 adds engines-
  fallback clause: "when `engines` field is null, missing, or empty
  array, the parser SHALL emit `Metadata['engines'] = []string{result.
  Engine}` as fallback (single-engine list)." Test
  `TestParseSearchEnginesFallback` added (#53). (M5) NFR-ADP7-001
  alloc differential (≤50/result vs ADP-001 ≤20/doc) explicitly
  justified: "engines []string slice copy + sha256 hash ID derivation
  add ~30 allocations per result; the floor analysis from ADP-001
  NFR-ADP-001 still applies for the NormalizedDoc Metadata map."
  (M6) §5 REQ-ADP7-009 acceptance now explicitly covers different-
  port redirects within allowlist. (M7) Removed the redundant
  `searxng.go::Capabilities (Notes field)` MX:NOTE from §6.7 — the
  Notes string itself is documentation; the additional annotation
  was redundant. Now §6.7 has 6 entries (was 7). Test count
  reconciled: HISTORY draft entry's "~38 tests" updated to "53 tests"
  matching the TDD plan §8 actual count. Total: 11 REQs (unchanged),
  4 NFRs (unchanged), 53 tests (was ~38), 7 Open Questions
  (unchanged). Status remains `draft` until cycle-2 audit confirms
  zero HIGH residual.

- 2026-05-04 (initial draft v0.1, limbowl via manager-spec):
  First EARS-formatted SPEC drafted after research phase. Scope and
  contracts derived from `.moai/specs/SPEC-ADP-007/research.md` (every
  external claim URL-cited; every internal claim file:line-cited).
  Built on SPEC-CORE-001 (`pkg/types.Adapter` 4-method interface,
  `pkg/types.Capabilities` descriptor, `pkg/types.NormalizedDoc`
  15-field struct, `*types.SourceError` taxonomy with four Categories,
  registry sole-emitter wrappedAdapter at
  `internal/adapters/registry.go:172-263`), SPEC-OBS-001 (cardinality
  allowlist `{adapter, outcome, adapter_class}` at
  `internal/obs/metrics/metrics.go:171` — ADP-007 introduces ZERO new
  metric labels), SPEC-IR-001 (Capabilities consumer contract;
  CategoryWeb DocType eligibility at
  `internal/router/category.go:93`), SPEC-BOOT-001 (compose stack
  including the deployed `searxng/searxng:2026.04.22-74f1ca203`
  service at `deploy/docker-compose.yml:106-130`), SPEC-ADP-001
  (Reddit reference shape — file layout, error mapping, MX tag plan,
  TDD harness — mirrored verbatim), and SPEC-ADP-002 (Hacker News;
  decimal-string page-number cursor convention reused for SearXNG
  pagination).

  Key structural inheritance from ADP-001 / ADP-002:
  - File layout
    (`searxng.go`/`search.go`/`client.go`/`parse.go`/`errors.go`/
    `bench_test.go` + testdata/) — identical in
    `internal/adapters/searxng/`.
  - HTTP client construction (10s timeout, redirect allowlist,
    `reqid.NewTransport` wrapping) — host allowlist is local-only:
    `{searxng:8080, localhost, 127.0.0.1}` (SearXNG never redirects
    cross-host in normal operation; the allowlist is defensive).
  - `categorizeStatus` rosetta inherited from ADP-001 with the
    `Adapter` field set to `"searxng"`. The 403→Permanent default is
    extended with a one-case promotion to RateLimited when
    `Retry-After` is present (SearXNG's limiter may emit 403 OR 429
    depending on version — see research §2.6).
  - `parseRetryAfter` helper adopted verbatim (RFC 7231 §7.1.3
    parser, 5s default, 60s cap).
  - Sole-emitter discipline: zero metrics/logs/spans emitted by the
    adapter; the registry wrappedAdapter emits all observability per
    Search call.
  - `var _ types.Adapter = (*Adapter)(nil)` compile-time interface
    assertion at the bottom of `searxng.go`.
  - Decimal-string page-number cursor pattern from SPEC-ADP-002
    `internal/adapters/hn/parse.go` (parseHits) — `q.Cursor=""` →
    pageno=1; `q.Cursor="N"` → pageno=N+1 round-trip.

  SearXNG-specific deltas from ADP-001 / ADP-002, documented inline
  in §6:
  - Endpoint:
    `${USEARCH_SEARXNG_URL:-http://searxng:8080}/search` — local
    intra-compose hostname, NOT a public internet endpoint. Plain HTTP
    (no TLS) inside the compose `app` network.
  - URL parameters: `q`, `format=json` (HARDCODED), `pageno`
    (1-based; only when cursor present), no `engines`, no `language`,
    no `time_range`, no `safesearch` (Open Questions §11.2-§11.4).
  - Response envelope: SearXNG `{query, number_of_results, results[],
    suggestions, corrections, answers, infoboxes,
    unresponsive_engines}` instead of Reddit's
    `{data: {after, children: [...]}}` or HN's `{hits, nbHits, page,
    nbPages}`. New `parseSearch` transform with SearXNG-specific
    JSON struct tags.
  - Engine-of-origin: each upstream result carries an `engine`
    (string) plus `engines` (`[]string`) field naming the contributing
    upstream engines (google / bing / duckduckgo / …). ADP-007
    surfaces these via `Metadata["engine"]` and `Metadata["engines"]`
    on every NormalizedDoc — first-class metadata for downstream RRF
    (SPEC-IDX-001) and dedup (SPEC-FAN-001 §2.4) consumers.
    Cardinality is bounded by the operator's enabled engine list at
    `deploy/searxng/settings.yml`; no Prometheus labels are emitted.
  - `category` field per upstream result is also surfaced via
    `Metadata["category"]` — a single SearXNG-side category string
    (e.g. `"general"`, `"news"`, `"images"`). Bounded set; safe to
    store in metadata.
  - Limiter handling: `categorizeStatus` is extended to map 403
    WITH `Retry-After` to RateLimited (research §2.6); 403 WITHOUT
    `Retry-After` falls through to Permanent per the inherited rule.

  User-locked decisions baked in:

  - **D1 Endpoint resolution**: `Options.BaseURL` defaults to the
    `USEARCH_SEARXNG_URL` env var; if unset, defaults to
    `http://searxng:8080` (compose-internal hostname). Tests inject a
    `httptest.Server` URL via `Options.BaseURL`. No host-binding to
    `localhost` is required at construction time — operators
    targeting a host port-forward set `USEARCH_SEARXNG_URL=
    http://localhost:8080`. (Research §1.1 + §1.2.)
  - **D2 JSON-format precondition**: `format=json` is hardcoded into
    every URL. The deployed SearXNG instance MUST have JSON enabled
    in `search.formats` (settings.yml). The default upstream
    `formats` list typically excludes JSON; if the deployed
    `deploy/searxng/settings.yml` does not explicitly enable JSON,
    the run-phase implementer MUST add it (a one-line settings
    change). This is the SOLE infrastructural precondition outside
    the Go package. Documented in §11.1 Open Question; documented in
    `Capabilities.Notes`. (Research §1.2 / §6.1.)
  - **D3 Pagination**: `q.Cursor` is the next page number as a
    decimal string. `parseCursor` is `strconv.Atoi`; rejects negative
    and non-numeric. First call uses cursor=""; the parser surfaces
    the next page number as `Metadata["next_cursor"] = strconv.Itoa(
    currentPage + 1)` on the LAST returned doc. Pattern reused
    verbatim from SPEC-ADP-002. The parser conservatively surfaces a
    next_cursor unless the response had zero results (last-page
    inference; SearXNG provides no `total_pages` field).
    (Research §2.4.)
  - **D4 Engine-of-origin emission**: Per-doc `Metadata["engine"]`
    (string, primary) and `Metadata["engines"]` (`[]string`, all
    contributing). The `SourceID` field stays `"searxng"` for every
    doc — preserving the registry-level cardinality boundary. The
    `Capabilities.Notes` field enumerates the engines explicitly
    enabled in `deploy/searxng/settings.yml` (currently `google`,
    `bing`, `duckduckgo`) plus a "+ default upstream engines" note.
    Bounded cardinality; operators introducing new engines own the
    cardinality footprint going forward. Engines are NEVER emitted as
    Prometheus labels. (Research §1.4 + §2.6.)
  - **D5 Limiter / 429+403 dual mapping**: `categorizeStatus` maps
    HTTP 429 → CategoryRateLimited (with parsed Retry-After). HTTP
    403 → CategoryPermanent BY DEFAULT, but PROMOTES to
    CategoryRateLimited when the response carries a `Retry-After`
    header. Open Question §11.5 documents run-phase verification
    against a real `limiter: true` instance. (Research §2.5 / §2.6.)
  - **D6 Local-only redirect allowlist**: SearXNG never issues
    cross-host redirects in normal operation. `redirectAllowlist`
    accepts `searxng`, `localhost`, `127.0.0.1` (with any port). Any
    other redirect target returns CategoryPermanent. Max 3 hops.
    (Research §1.1 / §3.5.)
  - **D7 Tests**: `net/http/httptest.Server` stub + golden JSON
    fixtures under `internal/adapters/searxng/testdata/`. NO live
    network calls in CI. Optional env-gated integration test
    (`-tags=integration` + `SEARXNG_LIVE=1`) is OUT OF SCOPE for
    v0.1; the run-phase implementer MAY add it as a follow-up.
    (Research §4 / §5.)

  Resolved discrepancy: `.moai/project/tech.md:148` flags "SearXNG
  AGPL contagion if ever offered as SaaS" — this is a project-level
  posture inherited unchanged. ADP-007 is a CONSUMER of an existing
  service-boundary relationship, not a new boundary itself. No
  NOTICE update needed; no new contagion surface.

  11 EARS REQs (8 × P0 + 3 × P1) covering all five EARS patterns
  (Ubiquitous, Event-Driven, State-Driven via REQ-ADP7-010 concurrency-
  safety contract, Optional via REQ-ADP7-007 limiter promotion +
  REQ-ADP7-009 redirect allowlist, Unwanted via REQ-ADP7-008 empty/
  invalid query rejection), 4 NFRs (NFR-ADP7-001 parse-path
  performance, NFR-ADP7-002 E2E p95 stub latency, NFR-ADP7-003 zero
  goroutine leak, NFR-ADP7-004 race-clean concurrent invocation),
  ~38 representative TDD tests, 7 Open Questions carried forward
  from research.md §6. Zero new Go module dependencies — pure stdlib
  (`net/http`, `encoding/json`, `time`, `context`, `errors`, `fmt`,
  `net/url`, `strconv`, `strings`, `unicode`, `unicode/utf8`, `os`,
  `math`) plus existing `pkg/types` and `internal/obs/reqid` (nil-safe
  consumer; the registry wraps observability, not the adapter).
  Inserted into M3 as the GENERAL-WEB adapter consuming the
  SPEC-CORE-001 contract; the M3 exit criterion
  (`.moai/project/roadmap.md:150`) requires `usearch query` returning
  fused results across ≥5 adapters — SearXNG is one of the seven M3
  adapters that develop in parallel after SPEC-FAN-001 lands.
  Harness level: standard (single domain, ≤10 source files, no
  security/payment keywords beyond the AGPL service-boundary which
  is already settled at the project level, no compose/env/config
  deltas beyond the optional one-line settings.yml `formats: [html,
  json]` pre-implementation precondition). Sprint Contract optional.
  Ready for plan-auditor review and annotation cycle.

---

## 1. Purpose

SPEC-CORE-001 published the typed adapter contract (`pkg/types.Adapter`
4-method interface at `pkg/types/adapter.go:28-45`,
`pkg/types.NormalizedDoc` 15-field canonical struct,
`*types.SourceError` taxonomy with four Categories, `pkg/types.
Capabilities` descriptor) and the `internal/adapters.Registry` with its
sole-emitter `wrappedAdapter` (`internal/adapters/registry.go:172-263`).
SPEC-OBS-001 registered `AdapterCalls{adapter,outcome}` and
`AdapterCallDuration{adapter}` collectors with `adapter` and `outcome`
in the cardinality allowlist
(`internal/obs/metrics/metrics.go:171`). SPEC-IR-001's
`CategoryEligibleDocTypes(CategoryWeb)`
(`internal/router/category.go:93`) returns
`{DocTypeArticle, DocTypePost, DocTypeOther}` — the eligibility set
ADP-007 publishes via `Capabilities.DocTypes` so the Intent Router
selects the SearXNG bridge for any web-classified query.
SPEC-BOOT-001 deployed the `searxng/searxng:2026.04.22-74f1ca203`
container at `deploy/docker-compose.yml:106-130`, exposing the
JSON metasearch API on port 8080 and aggregating 70+ external engines
through a single local HTTP boundary. SPEC-ADP-001 (Reddit) and
SPEC-ADP-002 (Hacker News) implemented the adapter contract end-to-
end, both with explicit concurrent-safety guarantees and zero new
Go module dependencies.

SPEC-ADP-007 fills `internal/adapters/searxng/` with the **GENERAL-
WEB bridge**: a Go HTTP client that turns one `types.Query` into one
`GET ${USEARCH_SEARXNG_URL}/search?q=...&format=json[&pageno=N]` call
and returns `[]types.NormalizedDoc` interleaving every contributing
upstream engine's hits. The bridge:

1. Validates `q.Text` (rejects empty/whitespace-only) and `q.Cursor`
   (rejects negative or non-numeric integer-page cursors).
2. Builds the request URL with the three required parameters (`q`,
   `format=json`, `pageno` when cursor present).
3. Executes the request via the constructed `*http.Client` (10s
   timeout, local-only redirect allowlist, `reqid.NewTransport`
   wrapping) with `User-Agent: usearch/<version>
   (+https://github.com/elymas/universal-search)` and
   `Accept: application/json`.
4. Maps HTTP status codes to `*types.SourceError` Categories per
   D5 (429 → RateLimited; 403 with Retry-After → RateLimited;
   other 4xx → Permanent; 5xx and network → Unavailable).
5. Parses the SearXNG response envelope via `parseSearch` — extracts
   the `results[]` array, transforms each into a `NormalizedDoc`
   per the field-mapping table in §6.3, surfaces the page-number
   cursor on the LAST returned doc as `Metadata["next_cursor"]`,
   and surfaces engine-of-origin metadata on EVERY doc as
   `Metadata["engine"]` / `Metadata["engines"]` / `Metadata["category"]`.
6. Returns `[]NormalizedDoc` with `SourceID="searxng"` for every
   doc, preserving the registry-level cardinality boundary.

The SearXNG bridge is the **primary general-web source** for
Universal Search per `.moai/project/tech.md:119`. It is selected by
the Intent Router for every `CategoryWeb` query and complements
domain-specific adapters (Reddit for social-post intent, HN for
tech news, arXiv for academic, GitHub for code, Naver/Daum for
Korean web).

The adapter does NOT do fanout (SPEC-FAN-001 owns goroutine
dispatch), does NOT do retry (SPEC-FAN-001 owns orchestration),
does NOT do caching (SPEC-CACHE-001 owns 5-phase fallback for
blocked sources), does NOT do ranking fusion (SPEC-IDX-001 owns RRF),
and does NOT emit any metric/log/span itself (the registry
wrappedAdapter does, sole-emitter discipline preserved).

Completion adds the SEVENTH M3 adapter to the routing pool. The M3
exit criterion (`.moai/project/roadmap.md:150` — "`usearch query`
returns fused results across ≥5 adapters") becomes achievable when
ADP-003..009 land alongside FAN-001 + IDX-001. SearXNG fills the
"general web" slot that no domain-specific adapter covers: a query
like "rust ownership memory safety" routes through the Intent Router
to `CategoryWeb`, gets dispatched by FAN-001 to ADP-007 (among
others), and returns a fused page of google + bing + duckduckgo
results — precisely the wedge that justifies the SearXNG service-
boundary investment in M1.

---

## 2. Scope

### 2.1 In-Scope

| Ref | Item |
|-----|------|
| a | `internal/adapters/searxng/searxng.go`: `Adapter` struct (`httpClient *http.Client`, `baseURL string`, `userAgent string`, `healthcheckTarget string` — all immutable post-construction), `New(opts Options) (*Adapter, error)` constructor (resolves `Options.BaseURL` → `USEARCH_SEARXNG_URL` env var → default `http://searxng:8080`; resolves `Options.UserAgentVersion` → default `"v0.1"`; resolves `Options.HealthcheckTarget` — when empty, derives from the resolved baseURL via `healthcheckHostFromBase` (extracts `u.Host` and appends `:443` for `https`/`:80` for `http` when port absent; falls back to `searxng:8080` on parse failure); constructs default `*http.Client` when `Options.HTTPClient == nil`), `Name() string` returning `"searxng"`, `Capabilities() types.Capabilities` returning a deterministic descriptor (RequiresAuth=false, AuthEnvVars=nil, DocTypes=[DocTypeArticle], SupportedLangs=nil, SupportsSince=false (no `time_range` in v0.1), RateLimitPerMin=0 (no external rate limit; local instance is operator-controlled via `server.limiter` in settings.yml), DefaultMaxResults=10, DisplayName="SearXNG", Notes containing the substrings `"local SearXNG metasearch bridge"`, `"format=json hardcoded"`, `"engines: google, bing, duckduckgo + default upstream engines"`, `"engine-of-origin in Metadata"`, `"AGPL service-boundary"`, and `"limiter may return 429 or 403"`), and `Healthcheck(ctx) error` (TCP-connect probe to `healthcheckTarget`). Compile-time interface assertion `var _ types.Adapter = (*Adapter)(nil)` at the bottom. |
| b | `internal/adapters/searxng/search.go`: `(*Adapter).Search(ctx, q types.Query) ([]types.NormalizedDoc, error)` — the hot path. Validates the query, parses any `q.Cursor` as a positive integer page (1-based; cursor="" omits the pageno parameter entirely so SearXNG defaults to pageno=1 server-side; cursor="N" produces explicit `pageno=N` in the URL — including cursor="1" which sets pageno=1 explicitly), builds the request URL via `url.Values` with `q`, `format=json`, and `pageno` (only when `q.Cursor != ""` AND parses to integer ≥ 1), delegates HTTP execution to `client.go`, delegates response parsing to `parse.go`, returns `[]NormalizedDoc` or `*SourceError`. Honours `ctx` cancellation throughout. The 5 MB body cap (`maxResponseBytes = 5 * 1024 * 1024`) prevents OOM on runaway responses. Pagination termination: the caller iterates by passing the surfaced `Metadata["next_cursor"]` back as `Query.Cursor`; if SearXNG returns zero results, the parser returns `(nil, "", nil)` and the caller stops. |
| c | `internal/adapters/searxng/client.go`: HTTP client construction (timeout=10s default, `CheckRedirect` enforces a host allowlist `{searxng, localhost, 127.0.0.1}` with max 3 hops, `Transport` wrapped with `internal/obs/reqid.NewTransport(http.DefaultTransport)`), single `doRequest(req *http.Request) (*http.Response, error)` helper that sets the User-Agent + `Accept: application/json` headers, and `categorizeStatus(httpStatus int, retryAfter time.Duration, hasRetryAfterHeader bool, cause error) *types.SourceError` mapping HTTP status → Category per the table in §6 (extends ADP-001 mapping with the 403+Retry-After → RateLimited promotion). |
| d | `internal/adapters/searxng/parse.go`: `parseSearch(body []byte, retrievedAt time.Time, currentPage int) ([]types.NormalizedDoc, string, error)` — parses the SearXNG JSON envelope `{query, number_of_results, results[], suggestions[], corrections[], answers[], infoboxes[], unresponsive_engines[][]}` into `[]NormalizedDoc` and returns the next-page cursor as the second return value. Per-result transform per the field-mapping table in §6.3. Empty `results` returns `(nil, "", nil)`. Malformed JSON returns `*SourceError{Category: CategoryPermanent, Cause: <json error>}`. The next-page cursor is `strconv.Itoa(currentPage + 1)` when `len(results) > 0`; empty otherwise. |
| e | `internal/adapters/searxng/errors.go`: package-private sentinels: `ErrInvalidQuery = errors.New("searxng: query text empty or whitespace-only")` (wrapped in `*SourceError{Category: CategoryPermanent}` by Search); `ErrInvalidCursor = errors.New("searxng: cursor must be positive integer page (>=1)")` (wrapped in `*SourceError{Category: CategoryPermanent}` by Search). The `parseRetryAfter` helper (5s default, 60s cap, integer-seconds + HTTP-date parsing per RFC 7231 §7.1.3) is duplicated here from ADP-001 (research §3 Rule of Three; cross-adapter helper extraction deferred to a future SPEC-ADP-REFAC-001 once 4+ adapters share the helper). |
| f | `internal/adapters/searxng/searxng_test.go`: tests for Adapter interface conformance (`var _ types.Adapter` assertion via `assertInterface`), `Name()` returns `"searxng"`, `Capabilities()` returns deterministic value (called twice; `reflect.DeepEqual`), `Healthcheck()` succeeds against a stub `httptest.Server`, `New()` validates options, `New()` honours `USEARCH_SEARXNG_URL` env var when `Options.BaseURL == ""`. |
| g | `internal/adapters/searxng/search_test.go`: the largest test file. Drives `(*Adapter).Search` against `httptest.Server` with golden fixtures: happy path 10 results (mix of engines), empty results, 429 with Retry-After, 4xx, 5xx, 403 with and without Retry-After (limiter dual handling), pagination cursor round-trip (cursor="" → pageno omitted/default; cursor="2" → pageno=2 in URL), ctx cancellation mid-request, invalid cursor rejection (negative, non-numeric, zero, non-integer), URL parameter shape verification (q + format=json always present). |
| h | `internal/adapters/searxng/client_test.go`: HTTP client unit tests — `categorizeStatus` truth table over 8 status codes (200/400/401/403-without/403-with-Retry-After/404/429/500/0), `parseRetryAfter` table over 6 input shapes, redirect allowlist enforcement (allowlist hosts allowed; cross-host rejected; chain-over-3 rejected), User-Agent header presence, `Accept: application/json` header presence. |
| i | `internal/adapters/searxng/parse_test.go`: field-mapping unit tests — table over 5 fixtures (single-engine result, multi-engine result with `engines` array, result with `publishedDate` parseable, result without `publishedDate`, result with empty `content`). Asserts each NormalizedDoc field per the §6.3 mapping table. Snippet truncation to 280 runes when `content` is long. Engine-of-origin metadata (engine, engines, category) populated. Pagination cursor round-trip. Hash field is empty (REQ-ADP7-005). |
| j | `internal/adapters/searxng/bench_test.go`: `BenchmarkParseSearch10Results` (NFR-ADP7-001 — p50 ≤ 5 ms parse time on amd64 for a 10-result SearXNG response fixture; allocation ≤ 50 allocs per result parsed = ≤ 500 allocs total). Includes `TestMain` calling `goleak.VerifyTestMain(m)` for NFR-ADP7-003 (mirrors ADP-001 / ADP-002 pattern). |
| k | `internal/adapters/searxng/testdata/`: golden JSON fixtures — `search_response.json` (10-result happy path, ~5 KB), `search_response_empty.json` (zero results, ~200 B), `search_response_pagination.json` (10 results for cursor round-trip, identical structure to happy path, ~5 KB), `search_response_multi_engine.json` (results with `engines` arrays of length 2-3, ~4 KB), `search_response_no_published_date.json` (results without `publishedDate` field, ~3 KB), `search_response_malformed.json` (truncated JSON, ~200 B). |

### 2.2 Out-of-Scope

[HARD] This SPEC explicitly excludes the following items. Each has a
known destination SPEC; this list prevents scope creep into ADP-007
(the general-web adapter).

- **Per-source customisations specific to other M3 sources** (arXiv
  OAI-PMH, GitHub PAT auth, YouTube yt-dlp metadata, Bluesky AT
  Protocol, Naver Korean-locale handling, Daum scraper-style handling,
  KoreaNewsCrawler RSS, Polymarket public API) → SPEC-ADP-003,
  SPEC-ADP-004, SPEC-ADP-005, SPEC-ADP-006, SPEC-ADP-008,
  SPEC-ADP-009.
- **Retry orchestration** (exponential backoff, circuit breaker,
  per-source health-state tracking, jitter, max-attempt counters) →
  SPEC-FAN-001 (M3). The adapter returns one categorised error per
  request and does not retry.
- **Response caching** → SPEC-CACHE-001 (M3). Each `Search` call is
  independent and idempotent at the adapter layer.
- **Result ranking, deduplication, RRF fusion across adapters** →
  SPEC-IDX-001 (M3). The adapter returns hits in SearXNG's
  relevance-ranked order with `Score` derived from the upstream
  `score` field clamped to `[0.0, 1.0]`, but does not re-rank.
- **Per-call engines override** (`Query.Filters[Key="engines"]` →
  SearXNG `engines=google,bing` URL parameter) → out of v0.1; defer
  to a follow-up SPEC if measured value warrants. (Open Question
  §11.2.)
- **Per-call language override** (`language=ko` for Korean queries)
  → out of v0.1. ADP-007 inherits the server-side `default_lang:
  "en"`. Korean queries route through SPEC-ADP-008/009 (Naver/Daum).
  (Open Question §11.3.)
- **Per-call time-range filter** (`time_range=day|month|year`) →
  out of v0.1. `Capabilities.SupportsSince=false`. (Open Question
  §11.4.)
- **Per-call safesearch override** (`safesearch=0|1|2`) → out of v0.1;
  inherits server default `safe_search: 0`.
- **Image / video result extraction** (SearXNG can return images and
  videos when `categories=images` is set; these have different result
  shapes) → out of v0.1; v0.1 hardcodes the default `general` category
  and processes only the standard text-result shape. Image / video
  expansion deferred to a follow-up SPEC.
- **Suggestions / corrections / answers / infoboxes surfacing** —
  SearXNG returns these as top-level response fields. v0.1 IGNORES
  them. A future SPEC may surface them via dedicated NormalizedDoc
  fields or a separate Adapter method, but that requires
  `pkg/types` SDK changes (out of scope).
- **`unresponsive_engines` propagation** — SearXNG reports per-engine
  failures in `unresponsive_engines[][]`. v0.1 IGNORES this; the
  adapter returns whatever results SearXNG aggregated. A future SPEC
  may surface engine-failure telemetry via `Metadata` or a dedicated
  field for SPEC-EVAL-002 reliability dashboard consumption.
- **AGPL NOTICE update** → not required. The compose-stack-level
  service-boundary posture is already documented at
  `deploy/docker-compose.yml:108-109` and `.moai/project/tech.md:148,
  166`. ADP-007 is a CONSUMER of an existing service-boundary
  relationship; it adds no new contagion surface. (Research §7;
  Open Question §11.6.)
- **Live integration tests in CI** → out of v0.1. `httptest.Server`
  + golden fixtures only. Optional env-gated live test
  (`-tags=integration` + `SEARXNG_LIVE=1`) deferred to a follow-up
  SPEC if measured value warrants.
- **Cross-adapter helper extraction** (sharing `parseRetryAfter`,
  `categorizeStatus`, redirect allowlist between Reddit, HN, and
  SearXNG packages) → out of v0.1. Rule of three: refactor when 4+
  adapters share the helper. Deferred to SPEC-ADP-REFAC-001
  post-M3.
- **Per-adapter custom Prometheus metrics** (e.g. `searxng_engine_
  results_total{engine}`) → would require amending SPEC-OBS-001's
  cardinality allowlist with the high-cardinality `engine` label
  (10-70+ values). Out of v0.1. The shared
  `AdapterCalls{adapter="searxng",outcome}` family is sufficient;
  per-engine telemetry lives in `Metadata` for offline analysis only.
- **OpenAPI / proto schema for the adapter response** — the
  `[]types.NormalizedDoc` return type IS the schema; no separate IDL.
- **`pkg/llm` integration** — the SearXNG adapter does NOT call any
  LLM. Classification is the Intent Router's job (SPEC-IR-001).

### 2.3 Score Normalization (No Tanh — Direct Clamp)

[HARD] The SearXNG `score` field per result is a positive float; the
upper bound is engine-aggregation-dependent and not formally
documented in the upstream API reference (research §1.3 / §2.3 noted
the source-code review showed `score` is "calculated in `close()`"
with no documented bound). Empirically, multi-engine high-rank
results can exceed 1.0; the adapter does NOT apply the Tanh formula
from ADP-001 / ADP-002 because SearXNG's score is already a
relevance estimator, not an unbounded upvote count. The clamp
guarantees `[0.0, 1.0]` codomain regardless of upstream value:

```
NormalizedDoc.Score = clamp(searxngScore, 0.0, 1.0)
```

where `clamp(x, 0.0, 1.0) = max(0.0, min(1.0, x))`. The
`searxngScore` is the value of the `score` field in the response
result. When the field is missing or zero, NormalizedDoc.Score is
0.0 (no relevance signal).

**Properties**:

- **Codomain**: `[0.0, 1.0]` (clamped).
- **Determinism**: pure function, no state, no time, no I/O.
- **Tie-break**: equal-scored results preserve SearXNG's response
  order (which is itself relevance-ranked across engines).
- **Score preservation**: the unclamped raw score is surfaced via
  `Metadata["score_raw"]` (float64) for downstream RRF (SPEC-IDX-001)
  consumers that may want to use the pre-clamp signal.

**Rationale (why direct clamp, not Tanh)** (research §1.3 / §2.3):
- SearXNG's score is rank-aggregated, not raw vote count. The
  bounded codomain `[0.0, ~10.0]` (in practice, most scores are
  `[0.0, 2.0]`) does not benefit from Tanh's saturation curve.
- Direct clamp is one operation, deterministic, and trivially
  testable.
- SPEC-IDX-001 RRF (M3) re-ranks across adapters using rank not raw
  score, so the precise per-doc score curve is informational, not
  load-bearing for cross-adapter ranking.

The formula is intentionally locked in v0.1 — changing it requires a
major-version bump of `Capabilities.Notes` and coordination with
SPEC-IDX-001 RRF tuning. Open Question §11.7 documents revisit
triggers.

---

## 3. EARS Requirements

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| REQ-ADP7-001 | Ubiquitous | The package `internal/adapters/searxng` SHALL expose an `Adapter` struct that implements `pkg/types.Adapter` exactly: `Name() string` returning `"searxng"`, `Search(ctx context.Context, q types.Query) ([]types.NormalizedDoc, error)`, `Healthcheck(ctx context.Context) error`, `Capabilities() types.Capabilities`. The package SHALL include a compile-time interface assertion `var _ types.Adapter = (*Adapter)(nil)`. `Capabilities()` SHALL be deterministic (two consecutive calls return `reflect.DeepEqual` values) with `SourceID="searxng"`, `DisplayName="SearXNG"`, `DocTypes=[DocTypeArticle]` (the per-doc DocType is hardcoded to Article in v0.1; the IR-001 selectAdapterSet logic still selects SearXNG for any `CategoryWeb` query because `[Article]` intersects with the eligible set `{Article, Post, Other}` per `internal/router/category.go:93`), `SupportedLangs=nil`, `SupportsSince=false`, `RequiresAuth=false`, `AuthEnvVars=nil`, `RateLimitPerMin=0` (no external rate limit; the local SearXNG instance is operator-controlled via `server.limiter` in settings.yml), `DefaultMaxResults=10`, and `Notes` containing the substrings `"local SearXNG metasearch bridge"`, `"format=json hardcoded"`, `"engine-of-origin in Metadata"`, `"AGPL service-boundary"`, and `"limiter may return 429 or 403"`. | P0 | `TestAdapterName`, `TestAdapterImplementsInterface` (compile-time), `TestCapabilitiesDeterministic`, `TestCapabilitiesShape` (asserts every documented field value + Notes substring matches), `TestHealthcheckSucceeds` (stub `httptest.Server` injected via Options.HealthcheckTarget). All in `internal/adapters/searxng/searxng_test.go`. |
| REQ-ADP7-002 | Event-Driven | WHEN `(*Adapter).Search(ctx, q)` is invoked with a non-empty `q.Text`, the adapter SHALL build an HTTP GET request to `<baseURL>/search` with the following query parameters: `q=<url.QueryEscape(q.Text)>` (always present), `format=json` (hardcoded; always present), and `pageno=<parsed cursor>` (only when `q.Cursor != ""` AND parses to integer ≥ 1 — including `cursor="1"` which produces explicit `pageno=1`). When `q.Cursor == ""`, the URL SHALL omit the `pageno` parameter (SearXNG defaults to `pageno=1` server-side). The adapter SHALL execute the request via the constructed `*http.Client`, parse the JSON envelope per REQ-ADP7-005, and return `(docs, nil)` on HTTP 200 with `len(docs) ≤ 100`. | P0 | `TestSearchHappyPath10Results` (httptest.Server returns `search_response.json`; assert 10 NormalizedDocs returned, each with all required fields populated and `Validate()` returning nil); `TestSearchURLAlwaysHasQAndFormat` (inspect captured request URL; assert `q` and `format=json` always present); `TestSearchOmitsPagenoWhenCursorEmpty` (q.Cursor="" → URL has no `pageno` param); `TestSearchSetsPagenoWhenCursorPresent` (q.Cursor="3" → URL contains `&pageno=3`); `TestSearchSetsPagenoExplicitOne` (q.Cursor="1" → URL contains `&pageno=1`). All in `search_test.go`. |
| REQ-ADP7-003 | Event-Driven | WHEN HTTP 429 is received from the SearXNG endpoint, the adapter SHALL parse the `Retry-After` response header per RFC 7231 §7.1.3 (integer-seconds OR HTTP-date), cap the result at 60 seconds, default to 5 seconds when the header is missing or malformed, and return `(nil, &types.SourceError{Adapter:"searxng", Category: types.CategoryRateLimited, HTTPStatus: 429, RetryAfter: <duration>, Cause: errors.New("searxng: rate limited")})`. The adapter SHALL NOT retry internally. | P0 | `TestSearchHTTP429WithIntegerRetryAfter` (`Retry-After: 30` → RetryAfter=30s); `TestSearchHTTP429WithHTTPDateRetryAfter` (HTTP-date 30s in future → RetryAfter ∈ (25s, 35s)); `TestSearchHTTP429NoRetryAfterDefaults5s` (no header → RetryAfter=5s); `TestSearchHTTP429RetryAfterCapped60s` (`Retry-After: 999` → 60s); `TestSearchHTTP429NoInternalRetry` (assert exactly 1 outbound request observed). All in `search_test.go` + `client_test.go`. |
| REQ-ADP7-004 | Event-Driven | WHEN HTTP 401, 404, or any 4xx other than 429 AND 403 is received, the adapter SHALL return `(nil, &types.SourceError{Adapter:"searxng", Category: types.CategoryPermanent, HTTPStatus: <code>, Cause: errors.New("searxng: permanent failure: <code>")})`. WHEN HTTP 500/502/503/504 is received OR a connection error occurs (DNS failure, dial timeout, read timeout, TLS handshake failure, connection refused), the adapter SHALL return `(nil, &types.SourceError{Adapter:"searxng", Category: types.CategoryUnavailable, HTTPStatus: <code or 0>, Cause: <inner error>})`. Network-layer errors set `HTTPStatus=0`. The adapter SHALL NOT retry. | P0 | `TestSearchHTTP4xx` (table over 401, 404, 400 → ErrPermanent + matching HTTPStatus); `TestSearchHTTP5xx` (table over 500, 502, 503, 504 → ErrSourceUnavailable + matching HTTPStatus); `TestSearchConnectionRefused` (httptest.Server closed before request; HTTPStatus=0 + ErrSourceUnavailable); `TestSearchUnavailablePreservesUnderlyingError` (`errors.Unwrap(srcErr).Error()` contains the inner cause). All in `search_test.go` + `client_test.go`. |
| REQ-ADP7-005 | Ubiquitous | The adapter SHALL transform each entry of the SearXNG `results[]` array into one `types.NormalizedDoc` using the field mapping in §6.3, MUST set `RetrievedAt = time.Now().UTC()` at the moment of parsing, MUST leave `Hash = ""` (consumers compute via `CanonicalHash()`), MUST populate `Metadata` with at minimum the keys `{engine, engines, category, score_raw}`, MUST set `DocType = types.DocTypeArticle`, MUST set `Lang = ""` (unknown — SearXNG does not expose per-result language). When the response field `publishedDate` is non-empty, the adapter SHALL parse it via `time.Parse(time.RFC3339, ...)` and assign to `NormalizedDoc.PublishedAt`; on parse failure, leave PublishedAt zero-valued (no error). When the response field `content` is non-empty, the adapter SHALL apply rune-truncation to 280 runes and assign to `NormalizedDoc.Snippet`; the full `content` SHALL be assigned to `NormalizedDoc.Body`. When the response has zero results, the adapter SHALL return `(nil, "", nil)`. The next-page cursor (when `len(results) > 0`) SHALL be returned as the second return value of `parseSearch` so `Search` can surface it via `Metadata["next_cursor"] = strconv.Itoa(currentPage + 1)` on the LAST returned NormalizedDoc. | P0 | `TestParseSearchFieldMapping` (table over 5 fixtures); `TestParseSearchEmptyResultsReturnsNilNoError`; `TestParseSearchSingleEngineMetadata` (fixture with `engine="google"`, `engines=["google"]` → returned doc has `Metadata["engine"]=="google"`, `Metadata["engines"]==["google"]`); `TestParseSearchMultiEngineMetadata` (fixture with `engines=["google","bing","duckduckgo"]` → returned doc has `Metadata["engines"]==["google","bing","duckduckgo"]`); `TestParseSearchPublishedDateParsed` (RFC3339 → PublishedAt set); `TestParseSearchPublishedDateMissing` (no field → PublishedAt zero); `TestParseSearchPublishedDateMalformed` (invalid string → PublishedAt zero, no error); `TestParseSearchSnippetTruncated280Runes` (long content → Snippet has trailing "..."); `TestParseSearchPaginationCursor` (currentPage=1, len(results)>0 → last doc Metadata["next_cursor"]=="2"); `TestParseSearchHashEmpty` (every returned doc has `Hash == ""`); `TestParseSearchMetadataKeys` (all 4 required keys present per doc). All in `parse_test.go`. |
| REQ-ADP7-006 | Ubiquitous | The adapter SHALL set the `User-Agent` HTTP header on every outbound request to a non-default value of the form `usearch/<version> (+https://github.com/elymas/universal-search)` where `<version>` is supplied via `Options.UserAgentVersion` (default `"v0.1"`). The adapter SHALL set the `Accept` header to `application/json`. The adapter SHALL NOT set any `Authorization` header (the local SearXNG instance has no token-based auth). | P0 | `TestSearchSetsCustomUserAgent` (inspect captured `r.Header.Get("User-Agent")`; assert it starts with `"usearch/"` and contains `"(+https://github.com/elymas/universal-search)"`); `TestSearchSetsAcceptJSON` (assert `Accept: application/json`); `TestSearchUserAgentVersionConfigurable` (Options.UserAgentVersion="v0.2-rc1" → header contains `"usearch/v0.2-rc1"`); `TestSearchNoAuthorizationHeader` (assert `r.Header.Get("Authorization") == ""`). All in `client_test.go`. |
| REQ-ADP7-007 | Optional | WHERE HTTP 403 is received from the SearXNG endpoint AND the response carries a non-empty `Retry-After` header (parseable per RFC 7231 §7.1.3), the adapter SHALL promote the response to `*types.SourceError{Adapter:"searxng", Category: types.CategoryRateLimited, HTTPStatus: 403, RetryAfter: <parsed duration>, Cause: errors.New("searxng: rate limited (403)")}` (matching the limiter behaviour observed when SearXNG's bot-detection emits 403 instead of 429 — research §2.6). WHERE HTTP 403 is received WITHOUT a `Retry-After` header, the adapter SHALL fall through to the default 4xx mapping and return `*types.SourceError{Category: types.CategoryPermanent, HTTPStatus: 403}` per REQ-ADP7-004. | P1 | `TestSearchHTTP403WithRetryAfterPromotesToRateLimited` (`Retry-After: 30` → CategoryRateLimited + HTTPStatus=403 + RetryAfter=30s); `TestSearchHTTP403WithoutRetryAfterStaysPermanent` (no Retry-After header → CategoryPermanent + HTTPStatus=403). Both in `search_test.go`. |
| REQ-ADP7-008 | Unwanted | IF `q.Text` is empty OR contains only Unicode whitespace runes (per `unicode.IsSpace` over every rune), THEN the adapter SHALL return `(nil, &types.SourceError{Adapter:"searxng", Category: types.CategoryPermanent, Cause: ErrInvalidQuery})` immediately and SHALL NOT issue any HTTP request. IF `q.Cursor` is non-empty AND does NOT parse as a positive integer (≥ 1) via `strconv.Atoi` (negative integers, zero, decimals, scientific notation, and non-numeric strings all rejected), THEN the adapter SHALL return `(nil, &types.SourceError{Adapter:"searxng", Category: types.CategoryPermanent, Cause: ErrInvalidCursor})` immediately and SHALL NOT issue any HTTP request. | P0 | `TestSearchEmptyQueryRejectedNoHTTP` (table over `["", "   ", "\t\n  \r"]` for `q.Text`; for each asserts `errors.Is(err, types.ErrPermanent)` AND `errors.Is(err, ErrInvalidQuery)` AND assert httptest.Server received zero requests); `TestSearchInvalidCursorRejectedNoHTTP` (table over `["abc", "-1", "0", "1.5", "1e3", " "]` for `q.Cursor`; for each asserts `errors.Is(err, types.ErrPermanent)` AND `errors.Is(err, ErrInvalidCursor)` AND zero requests). All in `search_test.go`. |
| REQ-ADP7-009 | Optional | WHERE the response is HTTP 301/302/303/307/308, the adapter's `*http.Client.CheckRedirect` SHALL follow up to 3 redirects WITHIN the host allowlist `{searxng, localhost, 127.0.0.1}` (port-agnostic — the host-only match permits any port for httptest stubs). Cross-host redirects (any other hostname) SHALL be rejected by returning an error from `CheckRedirect`; the adapter wraps this as `*SourceError{Adapter:"searxng", Category: CategoryPermanent, Cause: errors.New("searxng: cross-domain redirect rejected: <target host>")}` to prevent SSRF. Redirect chains exceeding 3 hops SHALL be rejected with a "too many redirects" message. While the local SearXNG instance does not redirect cross-host in normal operation, the allowlist is defensive and consistent with the pattern established in SPEC-ADP-001 REQ-ADP-010 and SPEC-ADP-002 REQ-ADP2-009. | P1 | `TestSearchFollowsAllowlistRedirect` (httptest.Server returns 302 to a second httptest.Server bound to 127.0.0.1; assert 200-path NormalizedDocs returned); `TestSearchRejectsCrossDomainRedirect` (httptest.Server returns 301 to `attacker.com`; assert `errors.Is(err, types.ErrPermanent)` AND error message contains `"cross-domain redirect"`); `TestSearchRejectsRedirectChainOver3` (httptest.Server bouncing within allowlist 4 times; assert error after 3 hops). All in `client_test.go`. |
| REQ-ADP7-010 | State-Driven | WHILE the same `*Adapter` instance is registered in the adapter registry and is being invoked concurrently from N goroutines (N ≥ 1), each `Search(ctx, q)` call SHALL execute independently with no shared mutable state across calls (the `*Adapter` struct is immutable post-construction; the underlying `*http.Client` is goroutine-safe per Go stdlib; the adapter holds no per-call state); the cumulative effect SHALL be N independent HTTP round-trips with no race-detector alarms. This requirement crystallises the concurrency contract that the registry (`internal/adapters/registry.go:172-263` wrappedAdapter) and the future fanout layer (SPEC-FAN-001 REQ-FAN-009) rely on. | P0 | `TestSearchConcurrentSafe` (50 goroutines each issuing one Search against the same httptest.Server; assert (a) no race-detector alarm under `-race`, (b) total response count = 50 observed at the stub, (c) all 50 returned slices are `[]types.NormalizedDoc` with `Validate()` returning nil for every doc). In `search_test.go`. |
| REQ-ADP7-011 | Event-Driven | WHEN the `Options.BaseURL` is empty AND the `USEARCH_SEARXNG_URL` environment variable is set AND non-empty, the `New(opts)` constructor SHALL adopt the env var as the base URL. WHEN both `Options.BaseURL` and `USEARCH_SEARXNG_URL` are empty (env var unset OR set to ""), the constructor SHALL adopt the default `"http://searxng:8080"` (intra-compose hostname). The constructor SHALL trim trailing slashes from the resolved base URL so subsequent `searchURL = baseURL + "/search?..."` construction does not double-slash. | P1 | `TestNewHonoursOptionsBaseURL` (Options.BaseURL="http://x:1/"; env var unset; assert `(*Adapter).baseURL == "http://x:1"`); `TestNewHonoursEnvVarWhenOptionsEmpty` (Options.BaseURL=""; env var "http://y:2"; assert baseURL=="http://y:2"); `TestNewDefaultsToCompose` (Options.BaseURL=""; env var unset; assert baseURL=="http://searxng:8080"). All in `searxng_test.go` using `t.Setenv`. |

---

## 4. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| NFR-ADP7-001 | Performance (parse path) | The parse path `parseSearch(body []byte, retrievedAt time.Time, currentPage int) ([]NormalizedDoc, string, error)` SHALL execute with mean wall-clock duration per op ≤ 5 ms over `go test -bench=BenchmarkParseSearch10Results -benchtime=10x -count=5 ./internal/adapters/searxng/...` on amd64; the median of the 5 runs is the assertion value (passes when ≤ 5 ms). The fixture is the `search_response.json` golden (10-result SearXNG response, ~5 KB). Allocation count ≤ 50 per result parsed (i.e. ≤ 500 allocs total for 10 results) per the same benchmark's `allocs/op` field — the floor analysis from SPEC-ADP-001 NFR-ADP-001 applies (the `pkg/types.NormalizedDoc.Metadata = map[string]any` contract from SPEC-CORE-001 forces a structural floor of ~17 allocs/doc; SearXNG's `engines []string` slice copy adds modest extra allocations per result but the standard 10-result fixture stays within 500 total). Measured via `BenchmarkParseSearch10Results` in `internal/adapters/searxng/bench_test.go`, run weekly in CI. Benchmarks do not count toward coverage. |
| NFR-ADP7-002 | End-to-end Latency | The end-to-end `Search` round-trip against the `httptest.Server` stub (no real network) SHALL complete with p95 ≤ 200 ms over 100 invocations, measured by `TestSearchE2ELatencyStubP95` in `search_test.go` (sort durations ascending, assert `durations[94] ≤ 200ms`). The harder live-SearXNG p95 (≤ 5s — SearXNG fans out across multiple upstream engines internally; local-network latency is dominated by upstream-engine response time) is documented as the operational target but is NOT enforced in CI (no live network). |
| NFR-ADP7-003 | Zero goroutine leak on cancellation | The adapter SHALL NOT leak any goroutine when the caller's context is cancelled mid-`Search`. Verified by `TestSearchNoGoroutineLeakOnCancel` in `search_test.go`, which uses `go.uber.org/goleak.VerifyNone(t)` after a `Search` call whose ctx is cancelled mid-flight via a 50ms-delayed cancel; assert zero residual goroutines after the call returns. Additionally, `TestMain` in `bench_test.go` invokes `goleak.VerifyTestMain(m)` for package-level leak detection (mirrors ADP-001 / ADP-002 pattern). |
| NFR-ADP7-004 | Race-clean concurrent invocation | `TestSearchConcurrentSafe` (REQ-ADP7-010 acceptance) executes successfully under `go test -race ./internal/adapters/searxng/...`; race-detector alarms attributable to the searxng package = 0. Workload: 50 goroutines × 1 Search call against one shared `*Adapter` and one shared `httptest.Server` = 50 adapter Search invocations. |

---

## 5. Acceptance Criteria

### REQ-ADP7-001 — Adapter Interface Conformance

- File `internal/adapters/searxng/searxng.go` declares `Adapter` struct
  with the documented fields (`httpClient *http.Client`, `baseURL
  string`, `userAgent string`, `healthcheckTarget string`).
- The compile-time assertion `var _ types.Adapter = (*Adapter)(nil)`
  appears at the bottom of `searxng.go`. If the interface ever drifts,
  this assertion fails to compile.
- `(*Adapter).Name()` returns the literal string `"searxng"`.
- `(*Adapter).Capabilities()` returns a `types.Capabilities` with:
  - `SourceID = "searxng"`
  - `DisplayName = "SearXNG"`
  - `DocTypes = []types.DocType{types.DocTypeArticle, types.
    DocTypePost, types.DocTypeOther}`
  - `SupportedLangs = nil` (language-agnostic; matches IR-001
    REQ-IR-008 fallback semantics)
  - `SupportsSince = false`
  - `RequiresAuth = false`
  - `AuthEnvVars = nil`
  - `RateLimitPerMin = 60`
  - `DefaultMaxResults = 10`
  - `Notes` contains the substrings `"local SearXNG metasearch
    bridge"`, `"format=json hardcoded"`, `"engine-of-origin in
    Metadata"`, `"AGPL service-boundary"`, and `"limiter may return
    429 or 403"`.
- `(*Adapter).Healthcheck(ctx)` succeeds against an httptest.Server
  bound to `127.0.0.1:0`. Tests construct the Adapter with
  `Options{HealthcheckTarget: <httptest.Server.Listener.Addr().String()>}`
  to redirect the dial target; the production default is the
  hostname:port extracted from `baseURL` (e.g. `searxng:8080`).
- `TestAdapterName`, `TestAdapterImplementsInterface`,
  `TestCapabilitiesDeterministic`, `TestCapabilitiesShape`,
  `TestHealthcheckSucceeds` all pass.

### REQ-ADP7-002 — Search Happy Path

- `TestSearchHappyPath10Results` against
  `testdata/search_response.json` returns exactly 10 `NormalizedDoc`
  entries (mix of single-engine and multi-engine results); each
  passes `Validate()` returning nil; the captured request URL contains
  `q=<encoded query>` and `format=json` and (when cursor=="") NO
  `pageno` parameter.
- `TestSearchURLAlwaysHasQAndFormat`,
  `TestSearchOmitsPagenoWhenCursorEmpty`,
  `TestSearchSetsPagenoWhenCursorPresent` (q.Cursor="3" → URL
  contains `&pageno=3`) all pass.

### REQ-ADP7-003 — HTTP 429 Rate-Limit Mapping

- `TestSearchHTTP429WithIntegerRetryAfter` asserts returned err is
  `*types.SourceError` with `Category=CategoryRateLimited`,
  `HTTPStatus=429`, `RetryAfter=30s`.
- `TestSearchHTTP429WithHTTPDateRetryAfter` parses an HTTP-date 30s
  in the future; asserts `RetryAfter` is in `(25s, 35s)`.
- `TestSearchHTTP429NoRetryAfterDefaults5s` (no header) asserts
  `RetryAfter=5s`.
- `TestSearchHTTP429RetryAfterCapped60s` (`Retry-After: 999`) asserts
  `RetryAfter=60s`.
- `TestSearchHTTP429NoInternalRetry` asserts exactly 1 request observed.

### REQ-ADP7-004 — HTTP 4xx/5xx and Network Failure Mapping

- `TestSearchHTTP4xx` table-drives 401, 404, 400; each asserts
  `errors.Is(err, types.ErrPermanent)` and matching HTTPStatus.
- `TestSearchHTTP5xx` table-drives 500, 502, 503, 504; each asserts
  `errors.Is(err, types.ErrSourceUnavailable)` and matching HTTPStatus.
- `TestSearchConnectionRefused` (httptest.Server closed before
  request) asserts `errors.Is(err, types.ErrSourceUnavailable)` and
  `HTTPStatus=0`.
- `TestSearchUnavailablePreservesUnderlyingError`: assert
  `errors.Unwrap(srcErr) != nil` and the inner error message contains
  "connection refused" or equivalent.

### REQ-ADP7-005 — NormalizedDoc Field Mapping

- `TestParseSearchFieldMapping` table-drives 5 fixtures (single-engine
  result, multi-engine result, result with parseable `publishedDate`,
  result without `publishedDate`, result with empty `content`). For
  each, asserts every NormalizedDoc field per the §6.3 mapping table.
- `TestParseSearchEmptyResultsReturnsNilNoError`: empty `results[]` →
  `(nil, "", nil)` returned.
- `TestParseSearchSingleEngineMetadata` /
  `TestParseSearchMultiEngineMetadata`: assert
  `Metadata["engine"]` is the primary engine string,
  `Metadata["engines"]` is the full `[]string` list (1 to N).
- `TestParseSearchPublishedDateParsed`: `publishedDate=
  "2026-04-15T08:30:00Z"` → `PublishedAt` parsed to that timestamp.
- `TestParseSearchPublishedDateMissing`: no `publishedDate` field →
  `PublishedAt` is zero-valued; no error returned.
- `TestParseSearchPublishedDateMalformed`: `publishedDate="not a
  date"` → `PublishedAt` is zero-valued; no error returned (graceful
  degradation).
- `TestParseSearchSnippetTruncated280Runes`: `content` of 500 runes →
  `Snippet` is 280 runes (last 3 are "..."), `Body` is the full 500.
- `TestParseSearchPaginationCursor`: `currentPage=1, len(results)=
  10` → `docs[len-1].Metadata["next_cursor"] == "2"`. Earlier docs
  do NOT have the `next_cursor` key.
- `TestParseSearchHashEmpty`: every returned `NormalizedDoc.Hash`
  equals `""`.
- `TestParseSearchMetadataKeys`: each returned doc's Metadata has at
  least `{engine, engines, category, score_raw}`.

### REQ-ADP7-006 — User-Agent and Accept Headers

- `TestSearchSetsCustomUserAgent`: captured request header
  `User-Agent` starts with `"usearch/"` and contains
  `"(+https://github.com/elymas/universal-search)"`.
- `TestSearchSetsAcceptJSON`: captured `Accept` header equals
  `"application/json"`.
- `TestSearchUserAgentVersionConfigurable`: `Options.UserAgentVersion
  = "v0.2-rc1"` → captured `User-Agent` contains `"usearch/v0.2-rc1"`.
- `TestSearchNoAuthorizationHeader`: captured `Authorization` header
  is empty.

### REQ-ADP7-007 — Limiter 403+Retry-After Promotion

- `TestSearchHTTP403WithRetryAfterPromotesToRateLimited`: stub
  returns 403 with `Retry-After: 30`. Assert `errors.Is(err,
  types.ErrRateLimited)`, `HTTPStatus=403`, `RetryAfter=30s`.
- `TestSearchHTTP403WithoutRetryAfterStaysPermanent`: stub returns
  403 with NO `Retry-After` header. Assert `errors.Is(err,
  types.ErrPermanent)`, `HTTPStatus=403`, `RetryAfter=0`.

### REQ-ADP7-008 — Empty Query and Invalid Cursor Rejection

- `TestSearchEmptyQueryRejectedNoHTTP` table-drives `q.Text` over
  `["", "   ", "\t\n  \r"]`; for each asserts
  `errors.Is(err, types.ErrPermanent)` AND
  `errors.Is(err, ErrInvalidQuery)`. The httptest.Server is
  instrumented with a request counter; assert exactly 0 requests.
- `TestSearchInvalidCursorRejectedNoHTTP` table-drives `q.Cursor`
  over `["abc", "-1", "0", "1.5", "1e3", " "]`; for each asserts
  `errors.Is(err, types.ErrPermanent)` AND
  `errors.Is(err, ErrInvalidCursor)`; assert zero requests.

### REQ-ADP7-009 — Redirect Allowlist

- `TestSearchFollowsAllowlistRedirect`: server A returns 302 with
  Location header pointing to server B (both bound to 127.0.0.1
  loopback); the test asserts search succeeds and returns the body
  from server B.
- `TestSearchRejectsCrossDomainRedirect`: server A returns 302 with
  Location `https://attacker.com/x`. Assert
  `errors.Is(err, types.ErrPermanent)` and error message contains
  `"cross-domain redirect"`.
- `TestSearchRejectsRedirectChainOver3`: 4 servers chained within
  the allowlist; assert error returned after 3 hops with message
  containing `"too many redirects"`.

### REQ-ADP7-010 — Concurrent Search Safety (State-Driven)

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
  3. Every goroutine receives `(docs, nil)` with `len(docs) == 10`
     (matching the standard `search_response.json` fixture); each
     returned `[]types.NormalizedDoc` slice has every doc passing
     `Validate()` returning nil.

### REQ-ADP7-011 — BaseURL Resolution

- `TestNewHonoursOptionsBaseURL`: `Options.BaseURL = "http://x:1/"`,
  env var unset → resolved baseURL is `"http://x:1"` (trailing
  slash trimmed).
- `TestNewHonoursEnvVarWhenOptionsEmpty`: `Options.BaseURL = ""`,
  `t.Setenv("USEARCH_SEARXNG_URL", "http://y:2")` → resolved
  baseURL is `"http://y:2"`.
- `TestNewDefaultsToCompose`: `Options.BaseURL = ""`, env var unset
  → resolved baseURL is `"http://searxng:8080"`.

### NFR-ADP7-001 — Parse-Path Performance

- `BenchmarkParseSearch10Results` is invoked as
  `go test -bench=BenchmarkParseSearch10Results -benchtime=10x -count=5 ./internal/adapters/searxng/...`
  on amd64.
- Assertion mechanism: take the 5 reported per-op mean wall-clock
  durations; the MEDIAN of those 5 values SHALL be ≤ 5 ms. PASS/FAIL
  is decidable from the `go test -bench` output alone.
- The bench reports `B/op` and `allocs/op`; `allocs/op ≤ 500` (= 50 ×
  10 results).

### NFR-ADP7-002 — E2E p95 (Stub)

- `TestSearchE2ELatencyStubP95` runs 100 invocations against the
  stub `httptest.Server`, sorts elapsed durations, asserts
  `durations[94] ≤ 200ms`.

### NFR-ADP7-003 — Goroutine Leak Check

- `TestSearchNoGoroutineLeakOnCancel`: `goleak.VerifyNone(t)`
  succeeds after a `Search` call whose ctx was cancelled at 50ms
  while the stub server delays response by 200ms.
- `TestMain` in `bench_test.go` calls `goleak.VerifyTestMain(m)` for
  package-level leak detection across all tests.

### NFR-ADP7-004 — Race-Clean Concurrent Workload

- `TestSearchConcurrentSafe` (REQ-ADP7-010 acceptance) executes under
  `go test -race`; race-detector alarms attributable to the searxng
  package = 0.

### Integration Checkpoint (M3 Adapter Pool)

When SPEC-FAN-001 is merged and the M3 adapter pool is populated,
ADP-007's acceptance includes a smoke check (manual or scripted)
that `registry.Get("searxng").Search(ctx, q)` against a stub
returns parseable `[]NormalizedDoc` interleaving with results from
other M3 adapters. This integration assertion lives in FAN-001's
acceptance criteria, not here, but is documented for traceability.

---

## 6. Technical Approach

### 6.1 Files to Modify

**Created (12 files)**:

- `internal/adapters/searxng/searxng.go` — Adapter struct, New, Name,
  Capabilities, Healthcheck, compile-time interface assertion
- `internal/adapters/searxng/searxng_test.go` — interface conformance
  + Capabilities determinism + BaseURL resolution tests
- `internal/adapters/searxng/search.go` — Search method (the hot
  path), URL construction, cursor parsing
- `internal/adapters/searxng/search_test.go` — main test file
  (largest)
- `internal/adapters/searxng/client.go` — HTTP client construction,
  doRequest, categorizeStatus (with 403+Retry-After promotion)
- `internal/adapters/searxng/client_test.go` — error mapping +
  redirect tests
- `internal/adapters/searxng/parse.go` — parseSearch transform
- `internal/adapters/searxng/parse_test.go` — field mapping tests
- `internal/adapters/searxng/errors.go` — ErrInvalidQuery /
  ErrInvalidCursor sentinels + parseRetryAfter helper
- `internal/adapters/searxng/bench_test.go` — NFR-ADP7-001 benchmark
  + TestMain with goleak
- `internal/adapters/searxng/testdata/search_response.json` (~5 KB)
- `internal/adapters/searxng/testdata/search_response_empty.json`
  (~200 B)
- `internal/adapters/searxng/testdata/search_response_pagination.json`
  (~5 KB)
- `internal/adapters/searxng/testdata/search_response_multi_engine.json`
  (~4 KB)
- `internal/adapters/searxng/testdata/search_response_no_published_date.json`
  (~3 KB)
- `internal/adapters/searxng/testdata/search_response_malformed.json`
  (~200 B)

**Modified**: none. The adapter self-contains. `pkg/types` already
publishes the contract, `internal/adapters/registry.go` already
accepts any `types.Adapter`, `internal/obs/metrics/metrics.go` already
declares `AdapterCalls` and `AdapterCallDuration` collectors with
`adapter` and `outcome` in the cardinality allowlist. The
`adapter="searxng"` cardinality value fits within the V1 14-adapter
ceiling per SPEC-OBS-001 NFR-OBS-002.

**Optional pre-implementation precondition** (Open Question §11.1):
- `deploy/searxng/settings.yml` — add `search.formats: [html, json]`
  if the upstream-default formats list does NOT include `json`.
  This is OUTSIDE the Go package; the run-phase implementer verifies
  via curl against the deployed stack BEFORE running adapter tests
  against a non-stub backend.

**Unchanged (by design)**: same as ADP-001 / ADP-002 §6.1 —
`registry.go`, `pkg/types/*`, `internal/obs/metrics/metrics.go`, and
`cmd/usearch/main.go` (registry construction owned by SPEC-CLI-001).

### 6.2 Package Layout

```
internal/adapters/searxng/
├── searxng.go                              # Adapter, New, Name, Capabilities, Healthcheck, interface assertion
├── searxng_test.go                         # Interface conformance + Capabilities + BaseURL resolution
├── search.go                               # (*Adapter).Search hot path
├── search_test.go                          # E2E + happy path + error categorisation tests
├── client.go                               # *http.Client, doRequest, categorizeStatus (with 403 promotion)
├── client_test.go                          # categorizeStatus table + redirect allowlist
├── parse.go                                # parseSearch transform (SearXNG envelope)
├── parse_test.go                           # Field mapping table tests
├── errors.go                               # ErrInvalidQuery + ErrInvalidCursor + parseRetryAfter helper
├── bench_test.go                           # BenchmarkParseSearch10Results + TestMain (goleak)
└── testdata/
    ├── search_response.json                # Happy path 10 results
    ├── search_response_empty.json          # Zero results
    ├── search_response_pagination.json     # 10 results for cursor round-trip
    ├── search_response_multi_engine.json   # Results with engines arrays
    ├── search_response_no_published_date.json # Results without publishedDate
    └── search_response_malformed.json      # Truncated JSON
```

[NOTE on duplication vs sharing] `parseRetryAfter`, `categorizeStatus`,
and the redirect-allowlist pattern duplicate the equivalents in
`internal/adapters/reddit/` and `internal/adapters/hn/`. This
duplication is INTENTIONAL in v0.1 per the rule of three (4+
consumers required for a shared package). Deferred to
SPEC-ADP-REFAC-001 post-M3 (research §3 / §6.2 in ADP-002).

### 6.3 SearXNG Result → NormalizedDoc Field Mapping

| SearXNG result Field | NormalizedDoc Field | Transform |
|----------------------|---------------------|-----------|
| `url` | `URL` | Use as-is |
| (constructed) | `ID` | `"searxng:" + sha256(url)[0:16]` — derived deterministically from canonical URL because SearXNG does not assign per-result IDs. The 16-hex-char prefix is sufficient for collision-resistance within a single search response. |
| (constant) | `SourceID` | `"searxng"` (matches `Name()`) |
| `title` | `Title` | Use as-is |
| `content` | `Body` | Use as-is (full content; may be empty) |
| `truncateRunes(content, 280)` | `Snippet` | First 280 runes; if content is empty, Snippet is empty (no fallback to title — SearXNG titles are usually too short to add value as snippets) |
| `publishedDate` (when non-empty + parseable RFC3339) | `PublishedAt` | `time.Parse(time.RFC3339, publishedDate)`; on parse error, leave zero |
| (parse time) | `RetrievedAt` | `time.Now().UTC()` (set by `parseSearch` caller) |
| (constant) | `Author` | `""` (SearXNG aggregates from many engines; per-result authorship is not consistently surfaced) |
| `clamp(score, 0.0, 1.0)` | `Score` | Direct clamp (§2.3); when score field missing, 0.0 |
| (constant) | `Lang` | `""` (SearXNG does not expose per-result language) |
| (constant) | `DocType` | `types.DocTypeArticle` (the predominant SearXNG result type for default `general` category; the Capabilities advertises `{Article, Post, Other}` for IR-001 selection breadth, but the actual per-doc value is hardcoded to `Article`) |
| (nil) | `Citations` | `nil` |
| (constructed) | `Metadata` | Map containing two key tiers. **REQUIRED keys** (consumers MAY rely on presence and stable shape; changes require a major-version bump of `Capabilities.Notes` and downstream coordination): `engine` (string; primary engine that originally returned this result — first element of `engines` array), `engines` ([]string; all contributing upstream engines, length 1..N where N is the operator-enabled engine count), `category` (string; SearXNG-side category like `"general"` or `"news"`), `score_raw` (float64; the unclamped SearXNG score for downstream RRF integration). REQ-ADP7-005 enforces these 4 as the contractual minimum. **OPTIONAL keys** (MAY be present; subject to change without major-version bump): `template` (string; SearXNG result template name), `positions` ([]int; per-engine rank positions). The LAST returned doc additionally gets `next_cursor` (REQUIRED on the last doc only, when `len(results) > 0`) encoded as `strconv.Itoa(currentPage + 1)`. |
| (constant) | `Hash` | `""` (consumers compute via `CanonicalHash()`) |

### 6.4 Type Sketches (illustrative; final shapes in run phase)

```go
// internal/adapters/searxng/searxng.go
package searxng

import (
    "context"
    "fmt"
    "net"
    "net/http"
    "net/url"
    "os"
    "strings"

    "github.com/elymas/universal-search/pkg/types"
)

const (
    envBaseURL               = "USEARCH_SEARXNG_URL"
    defaultBaseURL           = "http://searxng:8080"
    defaultUserAgentTemplate = "usearch/%s (+https://github.com/elymas/universal-search)"
    defaultUAVersion         = "v0.1"
)

type Options struct {
    BaseURL           string        // default: USEARCH_SEARXNG_URL env, then defaultBaseURL
    HTTPClient        *http.Client  // default: 10s timeout, allowlist redirect, reqid transport
    UserAgentVersion  string        // default: "v0.1"
    HealthcheckTarget string        // default: derived host:port from baseURL
}

type Adapter struct {
    httpClient        *http.Client
    baseURL           string
    userAgent         string
    healthcheckTarget string
}

func New(opts Options) (*Adapter, error) {
    base := strings.TrimRight(opts.BaseURL, "/")
    if base == "" {
        base = strings.TrimRight(os.Getenv(envBaseURL), "/")
    }
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
        target = healthcheckHostFromBase(base)
    }

    return &Adapter{
        httpClient:        client,
        baseURL:           base,
        userAgent:         ua,
        healthcheckTarget: target,
    }, nil
}

func (a *Adapter) Name() string { return "searxng" }

func (a *Adapter) Capabilities() types.Capabilities {
    return types.Capabilities{
        SourceID:    "searxng",
        DisplayName: "SearXNG",
        DocTypes: []types.DocType{
            types.DocTypeArticle,
            types.DocTypePost,
            types.DocTypeOther,
        },
        SupportedLangs:    nil,
        SupportsSince:     false,
        RequiresAuth:      false,
        AuthEnvVars:       nil,
        RateLimitPerMin:   60,
        DefaultMaxResults: 10,
        Notes: "local SearXNG metasearch bridge " +
            "(http://searxng:8080 default; USEARCH_SEARXNG_URL override). " +
            "format=json hardcoded. engines: google, bing, duckduckgo " +
            "+ default upstream engines (operator manages via " +
            "deploy/searxng/settings.yml). engine-of-origin in Metadata " +
            "(engine string + engines []string). AGPL service-boundary " +
            "(consumed as service, not linked). limiter may return 429 " +
            "or 403 with Retry-After (both treated as RateLimited).",
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

// healthcheckHostFromBase extracts host:port from a base URL string.
// Returns "searxng:8080" as fallback when parsing fails.
func healthcheckHostFromBase(base string) string {
    u, err := url.Parse(base)
    if err != nil || u.Host == "" {
        return "searxng:8080"
    }
    if u.Port() != "" {
        return u.Host
    }
    if u.Scheme == "https" {
        return u.Host + ":443"
    }
    return u.Host + ":80"
}

var _ types.Adapter = (*Adapter)(nil)
```

```go
// internal/adapters/searxng/client.go (excerpt)
var allowedRedirectHosts = map[string]struct{}{
    "searxng":    {},
    "localhost":  {},
    "127.0.0.1":  {},
}

func redirectAllowlist(req *http.Request, via []*http.Request) error {
    if len(via) >= 3 {
        return errors.New("searxng: too many redirects (max 3)")
    }
    host := req.URL.Hostname()
    if _, ok := allowedRedirectHosts[host]; !ok {
        return fmt.Errorf("searxng: cross-domain redirect rejected: %s", host)
    }
    return nil
}

// categorizeStatus extends the ADP-001 rosetta with a 403+Retry-After
// promotion to RateLimited (REQ-ADP7-007).
func categorizeStatus(status int, retryAfter time.Duration, hasRetryAfterHeader bool, cause error) *types.SourceError {
    se := &types.SourceError{Adapter: "searxng", HTTPStatus: status, Cause: cause}
    switch {
    case status == 429:
        se.Category = types.CategoryRateLimited
        se.RetryAfter = retryAfter
    case status == 403 && hasRetryAfterHeader:
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
// internal/adapters/searxng/parse.go (excerpt)
type searxngResponse struct {
    Query           string         `json:"query"`
    NumberOfResults int            `json:"number_of_results"`
    Results         []searxngHit   `json:"results"`
    // Suggestions/Corrections/Answers/Infoboxes/UnresponsiveEngines
    // intentionally omitted — IGNORED in v0.1.
}

type searxngHit struct {
    URL           string   `json:"url"`
    Title         string   `json:"title"`
    Content       string   `json:"content"`
    Engine        string   `json:"engine"`
    Engines       []string `json:"engines"`
    Category      string   `json:"category"`
    Score         float64  `json:"score"`
    Template      string   `json:"template"`
    Positions     []int    `json:"positions"`
    PublishedDate string   `json:"publishedDate"`
}

// parseSearch returns (docs, nextCursor, error). nextCursor is "" when
// len(results) == 0; otherwise strconv.Itoa(currentPage + 1).
func parseSearch(body []byte, retrievedAt time.Time, currentPage int) ([]types.NormalizedDoc, string, error) {
    var resp searxngResponse
    if err := json.Unmarshal(body, &resp); err != nil {
        return nil, "", &types.SourceError{
            Adapter:  "searxng",
            Category: types.CategoryPermanent,
            Cause:    fmt.Errorf("searxng: malformed JSON response: %w", err),
        }
    }
    if len(resp.Results) == 0 {
        return nil, "", nil
    }
    // ... transform each hit, set next_cursor on last doc
}
```

### 6.5 HTTP Client Construction Notes

- **Timeout**: 10 seconds total request deadline (default). Caller's
  ctx deadline takes precedence.
- **Redirect policy**: `CheckRedirect` enforces the allowlist
  `{searxng, localhost, 127.0.0.1}` (host-only, port-agnostic) and
  caps at 3 hops.
- **Transport**: `reqid.NewTransport(http.DefaultTransport)` for
  request-ID propagation. SearXNG ignores the request-ID header but
  the wrapping is project-wide convention.
- **Headers per request**: `User-Agent: usearch/<version>
  (+https://github.com/elymas/universal-search)` and
  `Accept: application/json`. NO authentication header (the local
  SearXNG instance has no token-based auth).

### 6.6 Observability Note

The SearXNG adapter, like the Reddit and HN adapters, emits ZERO
metrics, logs, and spans of its own. ALL observability comes from
the registry's `wrappedAdapter`
(`internal/adapters/registry.go:172-263`). The adapter's responsibility
is to return a correctly-categorised `*types.SourceError` so the
wrappedAdapter computes the right `outcome` label via
`types.OutcomeFromError(err)`.

Engine-of-origin is surfaced via `NormalizedDoc.Metadata["engine"]` /
`Metadata["engines"]` (free-form map values, NOT Prometheus labels).
This preserves the cardinality allowlist boundary at
`internal/obs/metrics/metrics.go:171` — no new label is registered or
emitted.

### 6.7 MX Tag Plan

| File::Symbol | Tag | Reason |
|--------------|-----|--------|
| `searxng.go::(*Adapter).Search` (defined in `search.go`) | `@MX:ANCHOR` | Sole entry point for all SearXNG fanout calls. fan_in ≥ 3 (registry wrappedAdapter, FAN-001 fanout, tests). `@MX:REASON: contract boundary; signature change ripples to FAN-001 + CLI-001 + downstream IDX-001 RRF input shape`. `@MX:SPEC: SPEC-ADP-007`. |
| `parse.go::parseSearch` | `@MX:ANCHOR` | Every SearXNG hit passes through this single transform. fan_in = 1 (Search) but invariant-bearing — bug here corrupts every NormalizedDoc returned. `@MX:REASON: NormalizedDoc field-mapping integrity gate; engine-of-origin Metadata contract is consumer-visible`. |
| `client.go::categorizeStatus` | `@MX:NOTE` | The HTTP-status-to-Category rosetta. The 403+Retry-After promotion is the SearXNG-specific delta from the ADP-001 baseline; future contributors will look here when SearXNG limiter behaviour evolves. |
| `client.go::doRequest` | `@MX:WARN` | Outbound network call. Redirect allowlist enforces SSRF safety boundary. `@MX:REASON: removing the CheckRedirect guard re-opens SSRF via any host that the local SearXNG instance might unexpectedly redirect to`. |
| `client.go::allowedRedirectHosts` map | `@MX:NOTE` | The 3-entry redirect allowlist. The local-only posture is intentional; adding a host requires a security review confirming the new host is operator-controlled. |
| `searxng.go::Capabilities` (Notes field) | `@MX:NOTE` | Engine-of-origin cardinality boundary documented here. Operators introducing new engines in `deploy/searxng/settings.yml` are responsible for confirming the new cardinality footprint. |
| `parse.go::parseSearch` engine surfacing block | `@MX:NOTE` | Metadata["engine"] / Metadata["engines"] / Metadata["category"] are the engine-of-origin contract for downstream RRF (SPEC-IDX-001) and dedup (SPEC-FAN-001 §2.4) consumers. `@MX:SPEC: SPEC-ADP-007`. |

All tags `[AUTO]`-prefixed, include `@MX:SPEC: SPEC-ADP-007`, follow
`code_comments: en` per `.moai/config/sections/language.yaml`. Per-file
hard limit (3 ANCHOR + 5 WARN per `.moai/config/sections/mx.yaml`):
respected (parse.go has 2 entries — 1 ANCHOR + 1 NOTE; client.go has
3 entries — 1 NOTE + 1 WARN + 1 NOTE).

### 6.8 Harness Level

Per `.moai/config/sections/harness.yaml` auto-routing: 11 EARS REQs
(8 × P0 + 3 × P1) + 4 NFRs touching 1 package
(`internal/adapters/searxng/`, ~10 source/test files + 6 testdata
fixtures) + zero cross-package edits + zero security/payment/PII
keywords (the AGPL service-boundary is settled at the project level
and is documentation-only) + at-most-one-line settings.yml change
(optional pre-implementation precondition) = **standard** harness
level. Sprint Contract is OPTIONAL but recommended. Evaluator profile
`default` applies.

---

## 7. What NOT to Build (Exclusions)

[HARD] This SPEC explicitly excludes the following items. Each has a
known destination SPEC; this list prevents scope creep into ADP-007.

- **Per-source customisations for arXiv, GitHub, YouTube, Bluesky, X,
  Naver, Daum, KoreaNewsCrawler, RSS, Polymarket** → SPEC-ADP-003
  through SPEC-ADP-009.
- **Retry orchestration** (exponential backoff, circuit breaker,
  per-source health-state tracking, jitter) → SPEC-FAN-001 (M3).
- **Response caching** → SPEC-CACHE-001 (M3).
- **Result ranking, deduplication, RRF fusion across adapters** →
  SPEC-IDX-001 (M3).
- **Per-call engines override** (`Query.Filters[Key="engines"]`) →
  out of v0.1; future SPEC.
- **Per-call language override** → out of v0.1.
- **Per-call time-range filter** → out of v0.1.
- **Per-call safesearch override** → out of v0.1.
- **Image / video result extraction** → out of v0.1.
- **Suggestions / corrections / answers / infoboxes surfacing** →
  out of v0.1; v0.1 IGNORES these.
- **`unresponsive_engines` propagation** → out of v0.1.
- **AGPL NOTICE update** → not required (already documented at
  project level).
- **Live integration tests in CI** → out of v0.1.
- **Cross-adapter helper extraction** → out of v0.1; rule of three;
  refactor SPEC-ADP-REFAC-001 post-M3.
- **Per-adapter custom Prometheus metrics** (e.g. per-engine result
  counters) → would explode cardinality; out of v0.1.
- **OpenAPI / proto schema for the adapter response** — the
  `[]types.NormalizedDoc` return type IS the schema.
- **`pkg/llm` integration** — the SearXNG adapter does NOT call any
  LLM.
- **Streaming Search results** (channel-based incremental delivery)
  → SPEC-SYN-004 (M4) if measured value.
- **Korean-locale handling for SearXNG** → SPEC-IDX-003 (M3); v0.1
  inherits server-side `default_lang: "en"`.
- **Sort customisation** → out of v0.1.

---

## 8. TDD Plan

Development follows RED-GREEN-REFACTOR per
`quality.development_mode: tdd` (`.moai/config/sections/quality.yaml`).
Representative RED-phase tests, written before implementation, grouped
by REQ. Total: ~38 tests covering REQ-ADP7-001..011 + NFRs. Coverage
target: 85% per `quality.test_coverage_target`. Benchmarks do not
count toward coverage.

| # | Test | Layer | REQ | Assertion |
|---|------|-------|-----|-----------|
| 1 | `TestAdapterName` | `searxng_test.go` | REQ-ADP7-001 | `(*Adapter).Name() == "searxng"` |
| 2 | `TestAdapterImplementsInterface` | `searxng_test.go` | REQ-ADP7-001 | Compile-time `var _ types.Adapter = (*Adapter)(nil)` succeeds |
| 3 | `TestCapabilitiesDeterministic` | `searxng_test.go` | REQ-ADP7-001 | Two consecutive `Capabilities()` calls return `reflect.DeepEqual` results |
| 4 | `TestCapabilitiesShape` | `searxng_test.go` | REQ-ADP7-001 | All documented field values match (DocTypes 3-element set, RateLimitPerMin=60, DefaultMaxResults=10, Notes substring contains all 5 markers) |
| 5 | `TestHealthcheckSucceeds` | `searxng_test.go` | REQ-ADP7-001 | TCP dial against test loopback succeeds |
| 6 | `TestNewHonoursOptionsBaseURL` | `searxng_test.go` | REQ-ADP7-011 | Options.BaseURL set → trailing slash trimmed; honoured |
| 7 | `TestNewHonoursEnvVarWhenOptionsEmpty` | `searxng_test.go` | REQ-ADP7-011 | t.Setenv USEARCH_SEARXNG_URL → adopted |
| 8 | `TestNewDefaultsToCompose` | `searxng_test.go` | REQ-ADP7-011 | Both empty → http://searxng:8080 |
| 9 | `TestSearchHappyPath10Results` | `search_test.go` | REQ-ADP7-002, REQ-ADP7-005 | 10 NormalizedDocs returned; each `Validate()` returns nil |
| 10 | `TestSearchURLAlwaysHasQAndFormat` | `search_test.go` | REQ-ADP7-002 | Captured URL has `q` and `format=json` |
| 11 | `TestSearchOmitsPagenoWhenCursorEmpty` | `search_test.go` | REQ-ADP7-002 | q.Cursor="" → URL has no `pageno` |
| 12 | `TestSearchSetsPagenoWhenCursorPresent` | `search_test.go` | REQ-ADP7-002 | q.Cursor="3" → URL has `pageno=3` |
| 13 | `TestSearchHTTP429WithIntegerRetryAfter` | `search_test.go` | REQ-ADP7-003 | `Retry-After: 30` → SourceError.RetryAfter==30s |
| 14 | `TestSearchHTTP429WithHTTPDateRetryAfter` | `search_test.go` | REQ-ADP7-003 | HTTP-date 30s ahead → RetryAfter ∈ (25s, 35s) |
| 15 | `TestSearchHTTP429NoRetryAfterDefaults5s` | `search_test.go` | REQ-ADP7-003 | No header → RetryAfter==5s |
| 16 | `TestSearchHTTP429RetryAfterCapped60s` | `search_test.go` | REQ-ADP7-003 | `Retry-After: 999` → RetryAfter==60s |
| 17 | `TestSearchHTTP429NoInternalRetry` | `search_test.go` | REQ-ADP7-003 | Server request count == 1 |
| 18 | `TestSearchHTTP4xx` | `search_test.go` | REQ-ADP7-004 | Table over 401/404/400 → ErrPermanent + matching HTTPStatus |
| 19 | `TestSearchHTTP5xx` | `search_test.go` | REQ-ADP7-004 | Table over 500/502/503/504 → ErrSourceUnavailable + matching HTTPStatus |
| 20 | `TestSearchConnectionRefused` | `search_test.go` | REQ-ADP7-004 | `errors.Is(err, types.ErrSourceUnavailable)`; HTTPStatus==0 |
| 21 | `TestSearchUnavailablePreservesUnderlyingError` | `search_test.go` | REQ-ADP7-004 | `errors.Unwrap(srcErr).Error()` contains inner cause |
| 22 | `TestParseSearchFieldMapping` | `parse_test.go` | REQ-ADP7-005 | Table over 5 fixtures; every documented field maps correctly |
| 23 | `TestParseSearchEmptyResultsReturnsNilNoError` | `parse_test.go` | REQ-ADP7-005 | Empty results → (nil, "", nil) |
| 24 | `TestParseSearchSingleEngineMetadata` | `parse_test.go` | REQ-ADP7-005 | engine=google → Metadata["engine"]=="google" |
| 25 | `TestParseSearchMultiEngineMetadata` | `parse_test.go` | REQ-ADP7-005 | engines=[g,b,d] → Metadata["engines"]==[g,b,d] |
| 26 | `TestParseSearchPublishedDateParsed` | `parse_test.go` | REQ-ADP7-005 | RFC3339 → PublishedAt set |
| 27 | `TestParseSearchPublishedDateMissing` | `parse_test.go` | REQ-ADP7-005 | Missing → PublishedAt zero, no error |
| 28 | `TestParseSearchPublishedDateMalformed` | `parse_test.go` | REQ-ADP7-005 | Malformed string → PublishedAt zero, no error |
| 29 | `TestParseSearchSnippetTruncated280Runes` | `parse_test.go` | REQ-ADP7-005 | Long content → Snippet truncated with "..." |
| 30 | `TestParseSearchPaginationCursor` | `parse_test.go` | REQ-ADP7-005 | currentPage=1, results>0 → last doc next_cursor=="2" |
| 31 | `TestParseSearchHashEmpty` | `parse_test.go` | REQ-ADP7-005 | Every NormalizedDoc.Hash == "" |
| 32 | `TestParseSearchMetadataKeys` | `parse_test.go` | REQ-ADP7-005 | All 4 required Metadata keys present |
| 33 | `TestParseSearchMalformedJSON` | `parse_test.go` | REQ-ADP7-005 | Truncated JSON → `*SourceError{CategoryPermanent}` |
| 34 | `TestSearchSetsCustomUserAgent` | `client_test.go` | REQ-ADP7-006 | UA starts with "usearch/" + contains URL |
| 35 | `TestSearchSetsAcceptJSON` | `client_test.go` | REQ-ADP7-006 | `Accept: application/json` header present |
| 36 | `TestSearchUserAgentVersionConfigurable` | `client_test.go` | REQ-ADP7-006 | Options override propagates |
| 37 | `TestSearchNoAuthorizationHeader` | `client_test.go` | REQ-ADP7-006 | Authorization header empty |
| 38 | `TestSearchHTTP403WithRetryAfterPromotesToRateLimited` | `search_test.go` | REQ-ADP7-007 | 403 + Retry-After=30 → CategoryRateLimited + RetryAfter=30s |
| 39 | `TestSearchHTTP403WithoutRetryAfterStaysPermanent` | `search_test.go` | REQ-ADP7-007 | 403 no header → CategoryPermanent |
| 40 | `TestSearchEmptyQueryRejectedNoHTTP` | `search_test.go` | REQ-ADP7-008 | Empty/whitespace q.Text → ErrPermanent + zero requests |
| 41 | `TestSearchInvalidCursorRejectedNoHTTP` | `search_test.go` | REQ-ADP7-008 | Invalid cursor (negative/zero/non-numeric) → ErrPermanent + zero requests |
| 42 | `TestSearchFollowsAllowlistRedirect` | `client_test.go` | REQ-ADP7-009 | 302 within allowlist followed |
| 43 | `TestSearchRejectsCrossDomainRedirect` | `client_test.go` | REQ-ADP7-009 | 302 to attacker.com → ErrPermanent |
| 44 | `TestSearchRejectsRedirectChainOver3` | `client_test.go` | REQ-ADP7-009 | 4-hop chain rejected |
| 45 | `TestSearchConcurrentSafe` | `search_test.go` | REQ-ADP7-010, NFR-ADP7-004 | 50 goroutines × 1 Search; race-clean; 50 stub requests; all docs Validate-clean |
| 46 | `TestSearchE2ELatencyStubP95` | `search_test.go` | NFR-ADP7-002 | 100 invocations against stub; p95 ≤ 200ms |
| 47 | `TestSearchNoGoroutineLeakOnCancel` | `search_test.go` | NFR-ADP7-003 | `goleak.VerifyNone(t)` after mid-flight ctx cancel |
| 48 | `TestParseRetryAfterTable` | `client_test.go` | REQ-ADP7-003 | Table over 6 inputs (int, HTTP-date, missing, malformed, > 60, negative) |
| 49 | `TestCategorizeStatusTable` | `client_test.go` | REQ-ADP7-003/004/007 | Truth table over 8 status codes (200/400/401/403-without/403-with/404/429/500/0) |
| 50 | `BenchmarkParseSearch10Results` | `bench_test.go` | NFR-ADP7-001 | Median of 5 runs ≤ 5ms; allocs/op ≤ 500 |
| 51 | `TestMain` (goleak.VerifyTestMain) | `bench_test.go` | NFR-ADP7-003 | Package-level goroutine leak check |

RED-GREEN-REFACTOR per requirement:

1. RED: Write failing test for REQ-ADP7-N.
2. GREEN: Implement minimal code to pass.
3. REFACTOR: Tidy; extract shared helpers if they remove duplication
   WITHIN the package; keep file sizes manageable (target each `.go`
   file < 200 LoC excluding tests).

Greenfield note: `internal/adapters/searxng/` does not exist. There
is no behaviour to preserve; no characterization tests needed.

---

## 9. Dependencies

### 9.1 Upstream SPEC Dependencies

- **SPEC-CORE-001 (implemented; merged commit f728aa2)**: provides
  `pkg/types.Adapter`, `pkg/types.Capabilities`, `pkg/types.Query`,
  `pkg/types.NormalizedDoc`, `*types.SourceError`,
  `types.OutcomeFromError`, `types.DocType` enum,
  `internal/adapters.Registry` with wrappedAdapter sole-emitter
  pattern. HARD dep.
- **SPEC-OBS-001 (implemented)**: provides `obs.Obs` bundle,
  `internal/obs/reqid.NewTransport` for request-ID propagation,
  `AdapterCalls{adapter,outcome}` and `AdapterCallDuration{adapter}`
  collectors. SOFT dep — adapter is nil-safe via the registry's
  nil-guards.
- **SPEC-IR-001 (implemented; merged commit 8a20b68)**: documents
  the consumer contract for `Capabilities` (REQ-IR-008 selects
  AdapterSet by intersecting `categoryEligibleDocTypes` with
  `SupportedLangs`). ADP-007's `Capabilities()` shape (DocTypes 3-
  element, SupportedLangs=nil) determines which routing categories
  the SearXNG bridge will be selected for. SOFT dep.
- **SPEC-BOOT-001 (implemented; merged 2026-04-28)**: provides the
  deployed `searxng/searxng:2026.04.22-74f1ca203` compose service at
  `deploy/docker-compose.yml:106-130`. HARD dep — without the
  running service, the adapter has nothing to call. The compose
  stack is the runtime dependency.

### 9.2 Reference (not required, but pattern-source)

- **SPEC-ADP-001 (implemented; merged commits 41372d4 + e3d1f7d +
  b2c2c53)**: provides the reference adapter shape that ADP-007
  copies — file layout, error mapping discipline, MX tag plan, TDD
  harness, `parseRetryAfter` pattern, `categorizeStatus` rosetta,
  redirect allowlist pattern.
- **SPEC-ADP-002 (implemented)**: provides the decimal-string
  page-number cursor pattern reused by ADP-007.

### 9.3 Parallelizable

- **SPEC-FAN-001 (M3)**: ADP-007 and FAN-001 land in M3; FAN-001
  consumes ADP-007 via `registry.Get("searxng").Search`.
- **SPEC-IDX-001 (M3)**: consumes `[]NormalizedDoc` shape (already
  locked in CORE-001), so ADP-007 doesn't add new constraints.
- **SPEC-ADP-003..009 (M3)**: each adapter develops in parallel
  per `.moai/project/roadmap.md:123` ("All SPEC-ADP-* (7-way),
  SPEC-IDX-* (3-way) — gated on SPEC-FAN-001"). ADP-007 is one of
  the seven.

### 9.4 Downstream Blocked SPECs

None. ADP-007's `blocks: []` is empty in the frontmatter — the
SearXNG bridge does not gate any other SPEC.

### 9.5 External Dependencies (run-phase pins)

**Zero new Go module dependencies.** ADP-007 uses only:

- Go stdlib: `context`, `crypto/sha256`, `encoding/hex`,
  `encoding/json`, `errors`, `fmt`, `io`, `math`, `net`, `net/http`,
  `net/url`, `os`, `strconv`, `strings`, `time`, `unicode`,
  `unicode/utf8`
- `pkg/types` (already pinned via SPEC-CORE-001)
- `internal/obs/reqid` (already pinned via SPEC-OBS-001)
- Test-only: `go.uber.org/goleak` (for NFR-ADP7-003) — already in
  `go.mod:30` indirect dependency (mature: ADP-001 / ADP-002 use it).

---

## 10. Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Server-side `formats:` does NOT include `json` (default upstream) | Medium | High | Open Question §11.1 documents the precondition. Run-phase implementer MUST verify via curl against the deployed stack BEFORE running non-stub tests. The fix is one line in `deploy/searxng/settings.yml`. Stub-based tests (httptest.Server) bypass this entirely. |
| Limiter-on returns 403 instead of 429 (upstream version-dependent) | Medium | Medium | REQ-ADP7-007 maps 403+Retry-After to RateLimited; 403 alone stays Permanent. Dual handling covers both observed behaviours. Test coverage for both branches. Open Question §11.5 documents need for run-phase verification. |
| Engine cardinality explosion if operator enables 50+ engines | Low | Medium | Engines NEVER emitted as Prometheus labels (research §1.4). They live only in `Metadata["engines"]` (free-form map; not subject to allowlist). Operator-managed; documented in Capabilities.Notes. |
| Two SearXNG instances disagree on result ordering across calls (non-deterministic upstream fanout) | Medium | Low | Acceptable for v0.1; SPEC-IDX-001 RRF re-ranks across adapters anyway. The adapter's `Score = clamp(score, 0, 1)` is deterministic per-result; cross-call ordering instability is upstream and out of scope. |
| AGPL service-boundary concerns if ADP-007 ever embeds SearXNG code | Low | High | Architectural discipline: ADP-007 makes ZERO source-level dependencies on SearXNG; only HTTP+JSON. Verified via package-level dependency analysis (no `searxng` Go module imports). Documented in research §7. |
| `publishedDate` parse failure on adversarial input | Low | Low | Graceful: REQ-ADP7-005 specifies "leave PublishedAt zero on parse error, no error returned." Tested via `TestParseSearchPublishedDateMalformed`. |
| 5 MB body cap hit on huge SearXNG responses | Low | Low | 5 MB is well beyond any documented SearXNG response size (typical is 10-200 KB); the cap is defensive against runaway responses, not throttling. `io.LimitReader` enforces without OOM. |
| Cursor-as-page-number confuses downstream consumers expecting opaque cursors | Low | Low | REQ-ADP7-005 surfaces the cursor via `Metadata["next_cursor"]` on the LAST doc — consumers MUST pass it back as `Query.Cursor` without parsing. Documented in `Capabilities.Notes`. The "decimal-string page-number" semantic is identical to ADP-002 HN, so consumers already handle this shape. |
| SearXNG instance becomes unreachable mid-fanout | Low | Medium | Network errors map to CategoryUnavailable + HTTPStatus=0. SPEC-FAN-001 partial-result assembly handles correctly. The compose `restart: unless-stopped` policy at `docker-compose.yml:128` recovers the container. |
| Hash collision risk in derived ID (`searxng:` + sha256[0:16]) | Low | Low | 16 hex chars = 64 bits = 2^32 collision probability for a 10-result page. Acceptable. Future SPEC may switch to full sha256 if collision telemetry warrants. |
| `time.Now()` in `RetrievedAt` non-deterministic in tests | Low | Low | `parseSearch` accepts `retrievedAt time.Time` parameter; tests inject a fixed time. Search wraps with `time.Now().UTC()` in production. |
| `engines` field surfaces a transient empty list ([]) when SearXNG malfunctions | Low | Low | `engines` is a slice; `Metadata["engines"]` accepts an empty `[]string`. Validate() does not check Metadata contents. Downstream consumers can detect zero-length and skip per-doc engine metadata. |
| Duplicate ID across pages when same URL appears on multiple pages | Low | Low | Cross-page dedup is SPEC-FAN-001 §2.3 / §2.4's responsibility (URL canonicalization key). The adapter returns docs with stable IDs; FAN-001 dedup catches cross-page duplicates. |
| Operator misconfigures `deploy/searxng/settings.yml` to disable all engines | Low | Medium | The adapter returns empty results gracefully (REQ-ADP7-005 returns `(nil, "", nil)` for empty `results[]`). FAN-001 records empty success at the adapter level. SPEC-EVAL-002 reliability dashboard surfaces the operator-misconfiguration symptom. |
| HTTP timeout (10s) too aggressive for SearXNG during incidents | Low | Low | Configurable via `Options.HTTPClient`; default 10s aligns with NFR-ADP7-002 stub p95 200ms × 50× safety margin. SearXNG fans out to upstream engines internally; long-tail latency is upstream-engine-dependent. |
| Local network HTTP-only posture exposes `Authorization: Bearer ...` in clear text | N/A | N/A | NOT APPLICABLE — REQ-ADP7-006 explicitly forbids any Authorization header. The local SearXNG instance has no auth. No secrets traverse the wire. |

---

## 11. Open Questions

These are explicitly UNRESOLVED at SPEC-approval time. Each has a
recommended default and a one-line resolution owner. They do NOT
block SPEC approval.

1. **Server-side JSON format enablement (formats: [html, json])**.
   Does the deployed `searxng/searxng:2026.04.22-74f1ca203` image's
   upstream defaults include `json` in `search.formats`?
   **Recommended default**: run-phase implementer MUST verify by
   hitting `http://localhost:${SEARXNG_PORT:-8080}/search?q=test
   &format=json` against the deployed compose stack. If the response
   is HTML or 403, add the formats key to `deploy/searxng/settings.yml`
   AND restart the compose stack BEFORE running adapter tests against
   a non-stub backend (test stubs via `httptest.Server` always work
   regardless of the real server's formats config). One-line fix.
   **Resolution owner**: ADP-007 run-phase implementer; document
   actual observed default in iteration-2 HISTORY entry.

2. **Per-call engines override** (`Query.Filters[Key="engines"]` →
   SearXNG `engines=...` URL parameter). **Recommended default**:
   NO in v0.1; rely on server-side default fanout. Add as P2
   follow-up if measured value emerges (e.g., a "Korean web only"
   route via `engines=naver,daum`). **Resolution owner**: SPEC-IR-002
   author.

3. **Korean-locale handling** (`language=ko` for Korean queries).
   **Recommended default**: NO `language` parameter in v0.1. Inherit
   server `default_lang: "en"`. Korean queries route through SPEC-
   ADP-008/009 (Naver/Daum). Revisit when SPEC-ADP-008/009 are
   wired and the IR-001 v2 router decides whether SearXNG should
   also serve as a Korean fallback. **Resolution owner**:
   SPEC-IR-002 author.

4. **Time-range filter** (`time_range=day|month|year`).
   **Recommended default**: NO in v0.1. Map
   `Capabilities.SupportsSince=false`. Add as a follow-up SPEC if
   measured value warrants. **Resolution owner**: SPEC-FAN-002 (M3
   fanout filter routing) author.

5. **Limiter status code (429 vs 403)**. The `searx/botdetection/`
   module exports a `too_many_requests` function but the precise
   HTTP status returned was not directly observable via WebFetch.
   **Recommended default**: REQ-ADP7-003 categorises 429 as
   RateLimited; REQ-ADP7-007 promotes 403+Retry-After to RateLimited;
   403 alone stays Permanent. Dual handling covers both observed
   behaviours without requiring run-phase modification.
   **Resolution owner**: ADP-007 run-phase implementer; document
   actual observed status code in iteration-2 HISTORY entry.

6. **AGPL service-boundary documentation update**. Does ADP-007
   require an entry in `NOTICE` or `docs/dependencies.md`?
   **Recommended default**: NO — `deploy/docker-compose.yml:108-109`
   already documents the boundary; `NOTICE` (per
   `.moai/project/tech.md:148, 166`) lists SearXNG as a service-
   boundary dependency. ADP-007 is a CONSUMER of an existing
   service-boundary relationship, not a new boundary itself. No
   NOTICE update needed. **Resolution owner**: SPEC-DEP-001
   SECURITY-3 owner (run a final compliance check before M3 close).

7. **Score formula revisit** (direct clamp vs Tanh-compatible curve
   matching ADP-001 / ADP-002). **Recommended default**: keep direct
   clamp in v0.1. Revisit after SPEC-IDX-001 RRF integration
   measurements if cross-adapter ranking quality indicates the
   SearXNG-side `score` is incompatible with the Reddit/HN Tanh-
   normalised codomain. **Resolution owner**: SPEC-IDX-001 author.

---

## 12. References

### External (URL-cited; verified per research.md §8)

- https://docs.searxng.org/ — SearXNG project home (overview).
- https://docs.searxng.org/dev/search_api.html — JSON Search API
  parameter table; format-must-be-enabled-in-settings caveat.
- https://docs.searxng.org/admin/settings/settings.html — settings.yml
  reference; `secret_key` requirement.
- https://github.com/searxng/searxng — repo; verified license is
  AGPL-3.0; no official Go client exists.
- https://github.com/searxng/searxng/blob/master/searx/results.py —
  per-result field shape (url/title/content/engine/engines/category/
  score/positions/template/parsed_url).
- https://github.com/searxng/searxng/blob/master/searx/webapp.py —
  JSON response construction path; response top-level keys.
- https://github.com/searxng/searxng/blob/master/searx/limiter.toml —
  bot-detection IP filtering config.
- RFC 7231 §7.1.3 Retry-After header semantics — basis for
  REQ-ADP7-003 parser (inherited from ADP-001).

### Internal (file:line cited)

- `.moai/specs/SPEC-ADP-007/research.md` — full research artifact for
  this SPEC.
- `.moai/specs/SPEC-ADP-001/spec.md` — Reddit reference SPEC; this
  SPEC inherits structure verbatim.
- `.moai/specs/SPEC-ADP-002/spec.md` — Hacker News SPEC; decimal-
  string page-number cursor convention reused.
- `.moai/specs/SPEC-CORE-001/spec.md` — Adapter / Capabilities /
  Query / NormalizedDoc / SourceError contract.
- `.moai/specs/SPEC-OBS-001/spec.md` — observability bundle and
  cardinality discipline.
- `.moai/specs/SPEC-IR-001/spec.md` — `Capabilities` consumer
  contract (REQ-IR-008).
- `.moai/specs/SPEC-BOOT-001/spec.md:60-79` — compose stack including
  the deployed SearXNG service.
- `pkg/types/types.go:1-22` — `pkg/types` SDK boundary description.
- `pkg/types/adapter.go:28-45` — Adapter interface 4-method shape.
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
  pattern (mirrored by ADP-007 searxng.go).
- `internal/adapters/reddit/search.go:1-167` — Search hot path
  pattern (mirrored by ADP-007 search.go).
- `internal/adapters/reddit/parse.go:1-203` — parseListing pattern
  (ADP-007 parseSearch is the equivalent).
- `internal/adapters/reddit/client.go:1-125` — HTTP client +
  redirect allowlist + categorizeStatus + parseRetryAfter pattern.
- `internal/router/category.go:13-122` — Category enum +
  CategoryEligibleDocTypes (CategoryWeb → DocTypeArticle/Post/Other).
- `internal/obs/metrics/metrics.go:37,89-95,171` — FanoutInflight
  Gauge pre-registration; cardinality allowlist.
- `deploy/docker-compose.yml:14-16,106-130` — SearXNG image pin and
  service definition.
- `deploy/searxng/settings.yml:1-43` — server-side SearXNG config.
- `.moai/project/roadmap.md:52,123,150` — M3 row "SPEC-ADP-007
  SearXNG bridge", parallelization plan, M3 exit criterion.
- `.moai/project/structure.md:28` — `internal/adapters/searxng/`
  package reservation.
- `.moai/project/tech.md:41,97,119,148,166` — SearXNG project-level
  posture.
- `.moai/config/sections/quality.yaml` — `development_mode: tdd`,
  `test_coverage_target: 85`.
- `.moai/config/sections/harness.yaml` — auto-routing to standard
  level.
- `.moai/config/sections/language.yaml` — `documentation: en`,
  `code_comments: en`.
- `go.mod:1-49` — Go module pin; ADP-007 introduces ZERO new
  dependencies.

---

*End of SPEC-ADP-007 v0.1 (DRAFT — pending plan-auditor review)*
