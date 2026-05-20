---
id: SPEC-ADP-006
title: Bluesky + X Adapter
version: 0.1.0
milestone: M3 — Fanout, adapters, index
status: implemented
priority: P0
owner: expert-backend
methodology: tdd
coverage_target: 85
created: 2026-05-04
updated: 2026-05-07
author: limbowl
issue_number: null
depends_on: [SPEC-CORE-001, SPEC-OBS-001, SPEC-IR-001]
blocks: []
---

# SPEC-ADP-006: Bluesky + X Adapter

## HISTORY

- 2026-05-04 (iteration 2 — plan-auditor cycle 1, limbowl via manager-spec):
  Audit identified 3 HIGH and 2 MEDIUM concerns; all addressed inline in
  this revision. HIGH fixes: (H1) REQ-ADP6-008 + REQ-ADP6-009 acceptance
  text now MANDATES `Options.EnvLookup` injection for env-dependent tests.
  Concurrent tests SHALL NOT use `t.Setenv` (goroutine-unsafe under
  `-race` per Go testing docs). The `Options.EnvLookup` callable is
  goroutine-safe by construction — each Adapter instance carries its
  own closure, no global mutation. (H2) REQ-ADP6-005 + §6.5 Bluesky
  field-mapping: `Title` is now set to `""` (empty) for Bluesky posts
  since the AT spec defines no headline field; `Body` carries
  `record.text` and `Snippet` carries truncated `record.text`. The
  prior text duplicated content between Title and Snippet; the cleaner
  shape preserves CORE-001:64 semantics (Title not required; empty
  permitted). NormalizedDoc.Validate() still passes (only ID, SourceID,
  URL, RetrievedAt are required). (H3) §6.4 X Capabilities Notes
  tightened: "Healthcheck verifies TCP reachability of x.com:443 only;
  Search remains disabled regardless of Healthcheck outcome. Operators
  monitoring uptime via Healthcheck SHALL NOT infer Search availability
  from a green Healthcheck on the X sub-source." MEDIUM fixes: (M1)
  HISTORY note added below — NFR-ADP6-001 alloc target ≤ 500 reserves
  the right to be raised in a future iteration if empirical baseline
  during run-phase exceeds 500/op (mirrors ADP-001's iteration-3
  precedent). (M2) §6.4 X Capabilities Notes — Healthcheck disclaimer
  (already covered by H3 fix). Total: 10 REQs (unchanged: H fixes
  tighten acceptance text, not add REQs). 4 NFRs unchanged.
  ~70 tests (unchanged count; tests reuse `Options.EnvLookup`).
  Status remains `draft` until cycle-2 audit confirms zero HIGH
  residual.

- 2026-05-04 (initial draft v0.1, limbowl via manager-spec):
  M3 social-platform adapter SPEC drafted following the SPEC-ADP-001 +
  SPEC-ADP-002 reference shapes verbatim. Scope and contracts derived from
  `.moai/specs/SPEC-ADP-006/research.md` (every external claim
  WebFetch-cited; every internal claim file:line-cited). Built on
  SPEC-CORE-001 (`pkg/types.Adapter` 4-method interface,
  `pkg/types.NormalizedDoc` 15-field canonical struct, `*types.SourceError`
  taxonomy with four Categories, `pkg/types.Capabilities` descriptor),
  SPEC-OBS-001 (`AdapterCalls{adapter,outcome}` +
  `AdapterCallDuration{adapter}` collectors, `adapter`/`outcome` already
  in cardinality allowlist; v0 emits TWO adapter label values:
  `"bluesky"` and `"x"`), and SPEC-IR-001 (`Capabilities` consumer
  contract, `CategorySocial` covers both sub-sources per
  `internal/router/category.go:14-15`). Reuses the SPEC-ADP-001 reference
  shape (file layout, error mapping, MX tag plan, TDD harness, Tanh score
  normaliser) and SPEC-ADP-002's secondary-adapter discipline.

  User-locked decisions baked in:

  - **D1 v0 scope**: Bluesky-only INTEGRATED + X RESERVED-but-DISABLED.
    Bluesky talks live to `https://public.api.bsky.app/xrpc/app.bsky.feed.searchPosts`
    (anonymous; no auth). X has its `Adapter` instance + `Capabilities`
    surface registered in the adapter registry but `Search` returns
    `*SourceError{Category: CategoryPermanent, Cause: ErrXDisabled}` until
    `USEARCH_X_ENABLED=true` is set, AND
    `*SourceError{Category: CategoryPermanent, Cause: ErrXProviderNotConfigured}`
    when the env gate is true but no provider is wired (the v0 default).
    Two-state error semantics distinguishes "operator did not opt in"
    from "operator opted in but no provider exists yet". Research §3.1
    documents the rationale; tech.md:147 mandates feature-flag opt-in
    for ToS-grey scraped sources. ScrapeCreators integration
    is DEFERRED to a future SPEC-ADP-006-XENABLE which will require
    explicit ToS acknowledgement at deployment time. Research §2.3.

  - **D2 Single package, two Adapter instances**: ONE Go package
    `internal/adapters/social/` with TWO constructors
    `social.NewBluesky(opts) (*Adapter, error)` and
    `social.NewX(opts) (*Adapter, error)`. Both return the same `*Adapter`
    struct type, distinguished by an unexported `subSource string` field
    set at construction time (`"bluesky"` or `"x"`). `(*Adapter).Name()`
    returns `subSource` so the registry's `wrappedAdapter`
    (`internal/adapters/registry.go:172-263`) emits two distinct
    `adapter` label values uniformly. `(*Adapter).Search` switches on
    `subSource` to dispatch to `searchBluesky` (live HTTP) or
    `searchXDisabled` (env-gated stub). Internally one struct, externally
    two registrations — honours both "single adapter, multi-source
    dispatch" and "two adapter labels emitted". Research §3.3.

  - **D3 No `github.com/bluesky-social/indigo` dependency**: indigo
    explicitly carries a "under active development; interfaces have not
    stabilized and may break or be removed" disclaimer at
    https://github.com/bluesky-social/indigo. Adoption couples the adapter
    to upstream churn and brings in transitive CBOR / libp2p / multiformats
    dependencies inappropriate for one HTTP call. Stdlib (`net/http`,
    `encoding/json`) suffices. Mirrors ADP-001 §5.2's go-reddit rejection.
    Research §1.9.

  - **D4 Bluesky URL construction**: post URLs are built deterministically
    from `author.handle` and the `rkey` (last segment of the AT-URI)
    as `"https://bsky.app/profile/" + handle + "/post/" + rkey`. The
    full AT-URI is preserved in `Metadata["post_uri"]` for downstream
    consumers needing the protocol-level identifier. Research §1.3.

  - **D5 Bluesky cursor strategy**: opaque pass-through. The response
    `cursor` (string) is surfaced via `Metadata["next_cursor"]` on the
    LAST returned doc when non-empty, identical to ADP-001 / ADP-002
    discipline. Empty cursor signals exhaustion. Research §1.5.

  - **D6 Score formula reuse**: `clamp(0.5 + 0.5 * tanh(x / 100.0), 0.0,
    1.0)` identical to SPEC-ADP-001 §2.3, with `x = likeCount +
    repostCount` for Bluesky. Reply / quote counts surface in Metadata
    only (not in the score calculation). The divisor=100 inflection point
    matches Reddit/HN posture; ADP-006 makes no per-source recalibration
    in v0 (Open Question §11.2). Research §4.3.

  - **D7 Defensive redirect allowlist**: `{public.api.bsky.app,
    api.bsky.app, bsky.app}` for Bluesky; `{}` (effectively reject all
    redirects) for X (since v0 does not make HTTP calls in the X path).
    3-hop cap preserved. Research §1.8.

  - **D8 Observability discipline**: ZERO new Prometheus metric families.
    Sole-emitter discipline preserved — the registry's wrappedAdapter
    emits all per-call metrics with `adapter` label set to `"bluesky"`
    or `"x"` (two distinct values, both already in the cardinality
    allowlist at `internal/obs/metrics/metrics.go`). The adapter itself
    emits no metrics, logs, or spans. Research §1.4.

  - **D9 Tests**: `httptest.Server` + golden JSON fixtures for Bluesky;
    no live network calls in CI. Optional env-gated live test
    (`-tags=integration` + `BSKY_LIVE=1`) deferred. The X disabled-path
    is tested with table-driven env scenarios.

  10 EARS REQs (8 × P0 + 2 × P1) covering all five EARS patterns
  (Ubiquitous via REQ-ADP6-001 / REQ-ADP6-005 / REQ-ADP6-006 / REQ-ADP6-010,
  Event-Driven via REQ-ADP6-002 / REQ-ADP6-003 / REQ-ADP6-004 / REQ-ADP6-008,
  State-Driven via REQ-ADP6-009 concurrent-safety contract, Optional via
  REQ-ADP6-007 lang/since filters, Unwanted not separately required —
  empty-query rejection is rolled into REQ-ADP6-002 acceptance per
  ADP-001's pattern reuse), 4 NFRs (NFR-ADP6-001 parse p50/allocs,
  NFR-ADP6-002 stub p95, NFR-ADP6-003 goroutine-leak, NFR-ADP6-004
  race-clean across both sub-sources), 9 Open Questions carried forward
  from research.md §7. Zero new Go module dependencies — pure stdlib
  (`net/http`, `encoding/json`, `time`, `context`, `errors`, `os`,
  `strings`, `strconv`, `net/url`, `unicode/utf8`, `math`) plus existing
  `pkg/types` and `internal/obs/reqid` (nil-safe consumer). Inserted into
  M3 as the SOCIAL-CATEGORY adapter; the seven-way M3 ADP-* parallel
  development includes SPEC-ADP-006 as one of the seven. Harness level:
  standard (single domain, ≤16 source files including testdata, no
  security/payment keywords, no compose/env/config deltas BEYOND a single
  ENV name documented in Capabilities.Notes). Sprint Contract optional.
  Ready for plan-auditor review and annotation cycle.

---

## 1. Purpose

The M3 milestone exit criterion is unambiguous:
`.moai/project/roadmap.md:150` — "All 12+ adapters pass contract tests;
`usearch query` returns fused results across ≥5 adapters; Korean query
returns Naver results ranked first." SPEC-FAN-001 (M3 gateway, status
approved) requires every adapter to honour the
`pkg/types.Adapter` 4-method contract and the
`registry.Get(name).Search(ctx, q)` invocation shape. SPEC-ADP-001
(Reddit) and SPEC-ADP-002 (Hacker News) implemented the M2 reference
shapes for general-web sources; SPEC-ADP-006 implements the M3
SOCIAL-PLATFORM adapter covering Bluesky and reserving X.

Per `.moai/project/roadmap.md:51` the SPEC scope is:

> SPEC-ADP-006 | Bluesky + X adapters | AT Protocol + ScrapeCreators
> (optional)

Bluesky is the natural primary sub-source because:

1. **Public AppView, no auth** (`https://public.api.bsky.app/xrpc/...`)
   eliminates secret-management complexity from the M3 adapter shape,
   identical to ADP-001 / ADP-002 posture. The 3,000-per-5min IP rate
   limit (research §1.7) is generous; the fanout's `MaxParallel=8` cap
   from SPEC-FAN-001 will dominate before this rate limit binds.
2. **Stable, well-documented Lexicon** (https://docs.bsky.app/docs/api/app-bsky-feed-search-posts)
   with cursor-based pagination and an `app.bsky.feed.defs#postView`
   response shape that maps directly to the SPEC-CORE-001 NormalizedDoc
   contract.
3. **Genuine social engagement signal** (likeCount + repostCount on
   `postView`) exercises the same Tanh score normaliser introduced in
   ADP-001 §2.3 against a different platform, validating the formula's
   portability.
4. **All four error categories reachable** in normal operation: 400 for
   malformed cursor (CategoryPermanent), 429 for IP rate-limit
   (CategoryRateLimited), 503 for cluster maintenance
   (CategoryUnavailable), timeouts (CategoryTransient via
   `context.DeadlineExceeded`).

X (Twitter) is a separately-registered Adapter instance whose v0
behaviour is OPTIONAL / DISABLED:

1. The X official API tier 2026 status (research §2.1) makes
   non-Enterprise integration infeasible; full-archive search is gated
   behind a custom-priced contract. Free / Basic / Pro tiers do not
   surface a usable search endpoint.
2. ScrapeCreators (research §2.2) is the documented v1 alternative but
   carries ToS-grey legal risk per `tech.md:147`. v0 reserves the
   surface (constructor, Capabilities, Search method) but ships
   non-functional. Operators must explicitly opt in by setting
   `USEARCH_X_ENABLED=true`; even then, v0 returns
   `ErrXProviderNotConfigured` because no provider is wired. A future
   SPEC-ADP-006-XENABLE will integrate ScrapeCreators (or alternative)
   behind explicit ToS acknowledgement.
3. The two-Adapter shape lets the IR-001 router produce
   `decision.AdapterSet=["bluesky","x"]` for `Category=social` queries
   without the router knowing about the env gate; the registry's
   wrappedAdapter emits the X disabled outcome as `outcome="failure"`
   uniformly.

The adapter does NOT do fanout (SPEC-FAN-001 owns goroutine dispatch),
does NOT do retry (SPEC-FAN-001 owns orchestration), does NOT do
caching (SPEC-CACHE-001 owns the 5-phase fallback), does NOT do ranking
fusion (SPEC-IDX-001 owns RRF), and does NOT emit any metric/log/span
itself (the registry wrappedAdapter does, sole-emitter discipline). It
DOES one job per sub-source: turn a `types.Query` into either a
Bluesky AppView HTTP request and JSON response → `[]NormalizedDoc`,
OR an env-gated rejection for X.

Completion is required by the M3 exit criterion (≥5 adapters fused).
Bluesky contributes one adapter to the social fan-in; X contributes a
registered-but-disabled adapter that surfaces a clean error to the
fanout layer (which records it in `Result.AdapterErrors["x"]` and
contributes zero docs).

This SPEC is the THIRD copy of the reference adapter pattern (after
ADP-001 / ADP-002), validating that the shape is portable from
text-only sources to a structured-Lexicon source AND demonstrating the
single-package multi-source dispatch pattern that future similarly-
shaped adapters (e.g., Threads + Mastodon under one
`fediverse/` package) may adopt.

---

## 2. Scope

### 2.1 In-Scope

| Ref | Item |
|-----|------|
| a | `internal/adapters/social/social.go`: `Adapter` struct (HTTP client + base URL + user-agent + healthcheck target + `subSource` dispatch field), `NewBluesky(opts BlueskyOptions) (*Adapter, error)` constructor (sets `subSource="bluesky"`, baseURL `https://public.api.bsky.app/xrpc/app.bsky.feed.searchPosts`), `NewX(opts XOptions) (*Adapter, error)` constructor (sets `subSource="x"`, no baseURL since v0 does not make HTTP calls), `(*Adapter).Name() string` returning the `subSource` value, `(*Adapter).Capabilities() types.Capabilities` returning a deterministic descriptor whose contents differ by sub-source (see §6.3 / §6.4), `(*Adapter).Healthcheck(ctx) error` (TCP-connect probe to `Options.HealthcheckTarget`, default `public.api.bsky.app:443` for Bluesky and a configurable target for X — when X is disabled the Healthcheck succeeds against the configured target if reachable, mirrors noop adapter discipline). Compile-time interface assertion `var _ types.Adapter = (*Adapter)(nil)`. |
| b | `internal/adapters/social/search.go`: `(*Adapter).Search(ctx, q types.Query) ([]types.NormalizedDoc, error)` — sub-source dispatch hot path. Switches on `a.subSource`: `"bluesky"` → `a.searchBluesky(ctx, q)`; `"x"` → `a.searchXDisabled(ctx, q)`. Honours `ctx` cancellation throughout. |
| c | `internal/adapters/social/search_bluesky.go`: `(*Adapter).searchBluesky(ctx, q types.Query) ([]types.NormalizedDoc, error)` — Bluesky live path. Validates query, builds URL via `url.Values` with `q`, `limit` (clamped 1..100), `cursor` (when non-empty), `lang` (when non-empty), `since` (when filter present and parseable as ISO datetime), `sort=top` (hardcoded in v0), executes via `client.go::doRequest`, delegates parsing to `parse.go::parseSearchPosts`. |
| d | `internal/adapters/social/search_x.go`: `(*Adapter).searchXDisabled(ctx, q types.Query) ([]types.NormalizedDoc, error)` — env-gated stub. Reads `os.Getenv("USEARCH_X_ENABLED")`. When the value is NOT exactly `"true"` (case-sensitive) → returns `(nil, &types.SourceError{Adapter: "x", Category: CategoryPermanent, Cause: ErrXDisabled})`. When the value IS exactly `"true"` → returns `(nil, &types.SourceError{Adapter: "x", Category: CategoryPermanent, Cause: ErrXProviderNotConfigured})` (because v0 ships no provider). The two-error semantics is testable without env mutation in a test by injecting `Options.EnvLookup func(string) string` (default `os.Getenv`). The function makes ZERO HTTP requests in v0 regardless of env state. |
| e | `internal/adapters/social/client.go`: HTTP client construction (timeout=10s default, `CheckRedirect` enforces a per-sub-source allowlist, `Transport` wrapped with `internal/obs/reqid.NewTransport(http.DefaultTransport)` for request-ID propagation), single `doRequest(ctx, *http.Request) (*http.Response, error)` helper that sets the User-Agent header and the `Accept: application/json` header, and `categorizeStatus(httpStatus int, retryAfter time.Duration, cause error, adapterName string) *types.SourceError` mapping HTTP status → Category — `adapterName` parameter accepts `"bluesky"` or `"x"` so one helper serves both sub-sources. |
| f | `internal/adapters/social/parse.go`: `parseSearchPosts(body []byte, retrievedAt time.Time) ([]types.NormalizedDoc, string, error)` — parses the Bluesky AppView response envelope `{cursor, posts}` (the documented `app.bsky.feed.searchPosts` shape) into `[]NormalizedDoc` and returns the next-page cursor as the second return value. Per-post transform per the field-mapping table in §6.5. Empty `posts` returns `(nil, "", nil)`. Bluesky XRPC error envelope (response with `error` and `message` JSON keys) returns `*SourceError{Category: CategoryPermanent, Cause: errors.New("bluesky: " + body.error + ": " + body.message)}`. Malformed JSON returns `*SourceError{Category: CategoryPermanent, Cause: <json error>}`. |
| g | `internal/adapters/social/url.go`: `constructBlueskyURL(handle, atURI string) (string, error)` builds `"https://bsky.app/profile/" + handle + "/post/" + rkey` from the AT-URI's last segment. Falls back to `author.did` when `handle` is empty. `parseATURI(atURI string) (did, collection, rkey string, err error)` is a tiny parser — splits `at://<did-or-handle>/<collection>/<rkey>` and returns the three segments. Both functions are pure; URL construction is deterministic. |
| h | `internal/adapters/social/score.go`: `normalizeScore(likeCount, repostCount int) float64` — implements the Tanh formula identical to SPEC-ADP-001 §2.3 with `x = likeCount + repostCount` clamped to non-negative. Pure function. Package-level constants `tanhDivisor = 100.0` and `scoreCenter = 0.5` annotated with `@MX:NOTE`. |
| i | `internal/adapters/social/errors.go`: package-private sentinels `ErrInvalidQuery = errors.New("social: query text empty or whitespace-only")` (wrapped in `*SourceError{Category: CategoryPermanent}` by Search), `ErrInvalidCursor = errors.New("social: cursor must be non-empty if provided")` (the Bluesky cursor is opaque so the only invalid case is empty-but-passed-as-non-empty — an empty `q.Cursor` is the start-of-search signal and is valid), `ErrXDisabled = errors.New("social/x: USEARCH_X_ENABLED env var not set to true; X sub-source disabled")` (returned when env gate is false; CategoryPermanent), `ErrXProviderNotConfigured = errors.New("social/x: no X provider is wired in v0; SPEC-ADP-006-XENABLE pending")` (returned when env gate is true but no provider exists; CategoryPermanent). Helper `parseRetryAfter(header string, now time.Time) time.Duration` adopted verbatim from ADP-001. |
| j | `internal/adapters/social/social_test.go`: tests for both Adapter instances' interface conformance (`var _ types.Adapter` assertion via package-level), `Name()` returns `"bluesky"` for `NewBluesky` and `"x"` for `NewX`, `Capabilities()` deterministic for both, `Healthcheck()` succeeds for both against stub. |
| k | `internal/adapters/social/search_test.go`: Bluesky live path tests — happy path 25 results, empty result, 429 with Retry-After, 4xx, 5xx, redirect to allowed and disallowed hosts, `lang` filter, `since` filter, pagination cursor round-trip, ctx cancellation mid-request, empty/whitespace query rejection, XRPC error-envelope parsing. X disabled path tests — env unset → ErrXDisabled, env=`"true"` → ErrXProviderNotConfigured, env=`"yes"` / `"1"` / `"TRUE"` (capitalised) → ErrXDisabled (case-sensitive `"true"` only), zero HTTP requests under any env state. |
| l | `internal/adapters/social/client_test.go`: HTTP client unit tests — `categorizeStatus` truth table over 7 status codes for both `"bluesky"` and `"x"` adapter names, `parseRetryAfter` table, redirect allowlist enforcement, headers (User-Agent + Accept). |
| m | `internal/adapters/social/parse_test.go`: field-mapping unit tests — table over 5 fixtures (typical post with non-empty langs, post with empty langs / Lang="", post with empty handle / DID-fallback URL, post with high engagement counts, post with empty repostCount/likeCount). Asserts each NormalizedDoc field per §6.5 mapping table. Snippet truncation to 280 runes. Score normalization (4 example values). Pagination cursor round-trip. Hash field is empty (REQ-ADP6-005). XRPC error-envelope path. |
| n | `internal/adapters/social/url_test.go`: `parseATURI` table over 6 inputs (typical, missing scheme, missing collection, missing rkey, empty, malformed). `constructBlueskyURL` table over 4 inputs (with handle, empty handle DID-fallback, empty rkey, malformed AT-URI). |
| o | `internal/adapters/social/score_test.go`: `normalizeScore` table-driven test over 7 (like, repost) tuples — `(0, 0)` → 0.5, `(10, 0)` → 0.549..., `(0, 10)` → 0.549..., `(50, 50)` → 0.761..., `(500, 500)` → 1.0, `(-1, 0)` (negative — defensively coerced to 0) → 0.5, `(1000, 5000)` → 1.0. Assertions within ±0.001. Determinism check. |
| p | `internal/adapters/social/errors_test.go`: Sentinel comparison via `errors.Is`; `parseRetryAfter` table. |
| q | `internal/adapters/social/bench_test.go`: `BenchmarkParseSearchPosts25Docs` (NFR-ADP6-001 — p50 ≤ 5 ms parse time on amd64 for a 25-post AppView fixture; allocs/op ≤ 500). `TestMain` calls `goleak.VerifyTestMain(m)` (NFR-ADP6-003). |
| r | `internal/adapters/social/testdata/`: golden JSON fixtures (Bluesky-only — X has no HTTP path in v0). `bluesky_search_response.json` (~6KB happy path, 25 posts mix of plain + with embeds), `bluesky_search_response_empty.json` (~200B, empty `posts` array), `bluesky_search_response_pagination.json` (~6KB with `cursor` set), `bluesky_search_response_with_lang.json` (~3KB, `langs=["ko"]` and `langs=["en"]` mixed), `bluesky_search_response_high_engagement.json` (~2KB, single post with likeCount=1000, repostCount=500), `bluesky_search_response_xrpc_error.json` (~150B, `{error: "InvalidRequest", message: "..."}`), `bluesky_search_response_malformed.json` (~200B, truncated JSON). |

### 2.2 Out-of-Scope

[HARD] This SPEC explicitly excludes the following items. Each has a known
destination SPEC; this list prevents scope creep into ADP-006.

- **Per-source customisations specific to other M3 adapters** (arXiv,
  GitHub, YouTube, SearXNG, Naver, Daum, KoreaNewsCrawler, RSS,
  Polymarket) → SPEC-ADP-003, SPEC-ADP-004, SPEC-ADP-005, SPEC-ADP-007,
  SPEC-ADP-008, SPEC-ADP-009.
- **Retry orchestration** (exponential backoff, circuit breaker,
  per-source health-state tracking, jitter, max-attempt counters) →
  SPEC-FAN-001 (M3, approved). The adapter returns one categorised error
  per request and does not retry.
- **Response caching** → SPEC-CACHE-001 (M3). Each Search call is
  independent and idempotent at the adapter layer.
- **Result ranking, deduplication, RRF fusion across adapters** →
  SPEC-IDX-001 (M3). The adapter returns posts in Bluesky's relevance
  order with `Score` in `[0.0, 1.0]` per the inherited Tanh formula,
  but does not re-rank.
- **App Password / `com.atproto.server.createSession` authenticated
  variant** → future SPEC-ADP-006-AUTH if measured anonymous IP rate-
  limit pressure warrants. v0 uses `public.api.bsky.app` exclusively.
  Research §1.1 + Open Question §11.1.
- **X provider integration** (ScrapeCreators, Nitter, official tier) →
  future SPEC-ADP-006-XENABLE behind explicit ToS acknowledgement.
  v0 ships X reserved-and-disabled. Research §2.3 + Open Question §11.4.
- **`record.embed` / `record.facets` rich extraction** (image cards,
  external link previews, quoted-post unwrapping, mention/link/tag range
  metadata) → out of v0 hot path. Embed and facets stored as opaque
  values in `Metadata["embed"]` / `Metadata["facets"]` ONLY when present;
  run-phase MAY decide to elide for allocation savings (Open Question
  §11.6). The plain-text `record.text` is the v0 Body.
- **Bluesky labels system / NSFW gating** → out of v0; surfaces in
  `Metadata["labels"]` only. SPEC-AUTH-002 (M6) may layer tenant policy.
  Research §7.5 + Open Question §11.5.
- **`sort=latest` Bluesky chronological mode** → out of v0; hardcode
  `sort=top`. Future enhancement; Open Question §11.3.
- **`searchActors` (`/xrpc/app.bsky.actor.searchActors`)** → out of
  scope; v0 indexes posts only.
- **Bluesky firehose / jetstream subscription** → not a search adapter
  shape; out of v0.
- **Live network integration tests in CI** → out of v0; httptest +
  golden fixtures only. Optional env-gated live test
  (`-tags=integration` + `BSKY_LIVE=1`) deferred.
- **OpenAPI / proto schema for the adapter response** — the
  `[]types.NormalizedDoc` return type IS the schema; no separate IDL.
- **Korean tokenisation or language inference** for Bluesky posts →
  SPEC-IDX-003 (M3). The adapter sets `NormalizedDoc.Lang = ""` when
  `record.langs` is empty, otherwise `record.langs[0]`.
- **`pkg/llm` integration** — the adapter does NOT call any LLM.
  Classification is the Intent Router's job (SPEC-IR-001).
- **Cross-adapter helper extraction** (sharing `parseRetryAfter`,
  `categorizeStatus`, redirect allowlist with reddit/hn packages) → out
  of v0. Refactor SPEC after M3 lands the seventh adapter.
- **Per-adapter custom Prometheus metrics** → would require amending
  SPEC-OBS-001's allowlist. Out of v0; the shared
  `AdapterCalls{adapter,outcome}` family with two new label values
  (`"bluesky"`, `"x"`) is sufficient.
- **`outcome="disabled"` Prometheus label value for X disabled state**
  → would require amending SPEC-OBS-001's outcome allowlist; out of v0.
  X disabled errors emit `outcome="failure"` per
  `pkg/types.OutcomeFromError(*SourceError{CategoryPermanent})`.
- **Auto-disable Bluesky on N consecutive Unavailable** → SPEC-EVAL-002
  (M8). The adapter is stateless.
- **Pagination beyond Bluesky's documented cursor range** → adapter
  honours whatever cursor the AppView returned; depth limits are an
  AppView-side concern.

### 2.3 Score Normalisation (Inheritance from ADP-001)

[HARD] The score normaliser in `score.go::normalizeScore(likeCount,
repostCount int) float64` uses the same Tanh formula as SPEC-ADP-001
§2.3:

```
x     = max(likeCount + repostCount, 0)
Score = clamp(0.5 + 0.5 * tanh(x / 100.0), 0.0, 1.0)
```

Domain (Bluesky-specific): `likeCount` and `repostCount` ∈ `[0, ~10000]`
integer in practice. `x` is the SUM of the two primary engagement
signals (reply / quote counts NOT included — they surface in Metadata for
RRF tuning by SPEC-IDX-001 if needed). The `max(*, 0)` clamp defends
against negative values from response-envelope corruption (impossible in
practice but cheap to enforce).

The codomain `[0.0, 1.0]`, `x=0 → Score=0.5` neutral semantic, and
saturation behaviour are unchanged. The divisor=100 inflection point
matches ADP-001 / ADP-002 — Bluesky's most popular posts (~5000 likes)
saturate to ~1.0; typical posts (10-50 engagements) cluster around
0.55-0.65; posts with zero engagement show 0.5 (neutral). RRF fusion in
SPEC-IDX-001 weights rank not raw score across adapters, so cross-source
calibration error is bounded.

Open Question §11.2 documents revisit triggers if measured Bluesky
ranking quality indicates calibration is needed.

For the X sub-source, `normalizeScore` is NOT invoked in v0 (no parse
path). When SPEC-ADP-006-XENABLE wires a provider, it MAY adopt the same
formula or define its own.

---

## 3. EARS Requirements

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| REQ-ADP6-001 | Ubiquitous | The package `internal/adapters/social` SHALL expose constructors `NewBluesky(opts BlueskyOptions) (*Adapter, error)` and `NewX(opts XOptions) (*Adapter, error)`, both returning `*Adapter` instances that implement `pkg/types.Adapter` exactly: `Name() string`, `Search(ctx, types.Query) ([]types.NormalizedDoc, error)`, `Healthcheck(ctx) error`, `Capabilities() types.Capabilities`. The package SHALL include a compile-time interface assertion `var _ types.Adapter = (*Adapter)(nil)`. `(*Adapter).Name()` returned by the Bluesky instance SHALL equal `"bluesky"`; `(*Adapter).Name()` returned by the X instance SHALL equal `"x"`. `Capabilities()` SHALL be deterministic (two consecutive calls return equal values) for both instances and SHALL set `SourceID` to match `Name()`. The Bluesky instance's Capabilities SHALL declare `DocTypes=[DocTypePost]`, `SupportedLangs=nil`, `SupportsSince=true`, `RequiresAuth=false`, `AuthEnvVars=nil`, `RateLimitPerMin=600`, `DefaultMaxResults=25`, `DisplayName="Bluesky"`, and `Notes` containing the substrings `"public AppView"`, `"app.bsky.feed.searchPosts"`, `"sort=top"`, and `"social"`. The X instance's Capabilities SHALL declare `DocTypes=[DocTypePost]`, `SupportedLangs=nil`, `SupportsSince=false`, `RequiresAuth=false`, `AuthEnvVars=nil`, `RateLimitPerMin=0`, `DefaultMaxResults=0`, `DisplayName="X (Twitter)"`, and `Notes` containing the substrings `"DISABLED in v0"`, `"USEARCH_X_ENABLED=true"`, `"SPEC-ADP-006-XENABLE pending"`, and `"social"`. | P0 | `TestBlueskyName` / `TestXName` (assert literals); `TestBlueskyImplementsInterface` / `TestXImplementsInterface` (compile-time `var _ types.Adapter`); `TestBlueskyCapabilitiesDeterministic` / `TestXCapabilitiesDeterministic`; `TestBlueskyCapabilitiesShape` (asserts all 9 documented field values + Notes substring matches); `TestXCapabilitiesShape` (asserts all 9 documented field values + Notes "DISABLED in v0" substring); `TestBlueskyHealthcheckSucceeds` / `TestXHealthcheckSucceeds` (each against a stub `httptest.Server` injected via Options.HealthcheckTarget). All in `social_test.go`. |
| REQ-ADP6-002 | Event-Driven | WHEN `(*Adapter).Search(ctx, q)` is invoked on the Bluesky instance with a non-empty `q.Text`, the adapter SHALL build an HTTP GET request to `https://public.api.bsky.app/xrpc/app.bsky.feed.searchPosts` with the following query parameters: `q=<url.QueryEscape(q.Text)>`, `limit=clamp(q.MaxResults, 1, 100)` (defaulting to 25 when `q.MaxResults == 0`), `sort=top` (hardcoded), `cursor=<q.Cursor>` (only when `q.Cursor != ""`), `lang=<q.Lang>` (only when `q.Lang != ""`), `since=<filter value>` (only when `Query.Filters[Key="since"]` is present and parses as RFC 3339 datetime). The adapter SHALL execute the request via the constructed `*http.Client`, parse the JSON envelope per REQ-ADP6-005 mapping, and return `(docs, nil)` on HTTP 200 with `len(docs) ≤ 100`. IF `q.Text` is empty OR contains only Unicode whitespace (per `unicode.IsSpace` over every rune), THEN the adapter SHALL return `(nil, &types.SourceError{Adapter:"bluesky", Category: types.CategoryPermanent, Cause: ErrInvalidQuery})` immediately and SHALL NOT issue any HTTP request. | P0 | `TestSearchBlueskyHappyPath25Posts` (httptest.Server returns `bluesky_search_response.json`; assert 25 NormalizedDocs returned, each with all required fields populated and `Validate()` returning nil); `TestSearchBlueskyURLParametersIncludeAllRequired` (inspect captured request URL; assert `q`, `limit`, `sort=top` always present); `TestSearchBlueskyClampsLimitTo100` (q.MaxResults=500 → URL has `limit=100`); `TestSearchBlueskyDefaultsLimitTo25` (q.MaxResults=0 → URL has `limit=25`); `TestSearchBlueskyOmitsCursorWhenEmpty` (q.Cursor="" → URL has no `cursor`); `TestSearchBlueskySetsCursorWhenPresent` (q.Cursor="abc123" → URL has `cursor=abc123`); `TestSearchBlueskyOmitsLangWhenEmpty` / `TestSearchBlueskySetsLangWhenPresent` (q.Lang="ko" → URL has `lang=ko`); `TestSearchBlueskyEmptyQueryRejectedNoHTTP` (table over `["", "   ", "\t\n  \r"]` for `q.Text`; for each asserts `errors.Is(err, types.ErrPermanent)` AND assert httptest.Server received zero requests). All in `search_test.go`. |
| REQ-ADP6-003 | Event-Driven | WHEN HTTP 429 is received from the Bluesky AppView, the adapter SHALL parse the `Retry-After` response header per RFC 7231 §7.1.3 (integer-seconds OR HTTP-date), cap the result at 60 seconds, default to 5 seconds when the header is missing or malformed, and return `(nil, &types.SourceError{Adapter:"bluesky", Category: types.CategoryRateLimited, HTTPStatus: 429, RetryAfter: <duration>, Cause: errors.New("bluesky: rate limited")})`. WHEN HTTP 400, 401, 403, or 404 is received, the adapter SHALL return `(nil, &types.SourceError{Adapter:"bluesky", Category: types.CategoryPermanent, HTTPStatus: <code>, Cause: errors.New("bluesky: permanent failure: <code>")})`. WHEN HTTP 500/502/503/504 is received OR a connection error occurs (DNS failure, dial timeout, read timeout, TLS handshake failure), the adapter SHALL return `(nil, &types.SourceError{Adapter:"bluesky", Category: types.CategoryUnavailable, HTTPStatus: <code or 0>, Cause: <inner error>})`. Network-layer errors set `HTTPStatus=0`. The adapter SHALL NOT retry internally. | P0 | `TestSearchBlueskyHTTP429WithIntegerRetryAfter` (`Retry-After: 30` → RetryAfter=30s); `TestSearchBlueskyHTTP429WithHTTPDateRetryAfter` (HTTP-date 30s in future → RetryAfter ∈ (25s, 35s)); `TestSearchBlueskyHTTP429NoRetryAfterDefaults5s`; `TestSearchBlueskyHTTP429RetryAfterCapped60s`; `TestSearchBlueskyHTTP429NoInternalRetry` (assert exactly 1 outbound request); `TestSearchBlueskyHTTP4xx` (table over 400/401/403/404 → ErrPermanent + matching HTTPStatus); `TestSearchBlueskyHTTP5xx` (table over 500/503 → ErrSourceUnavailable + matching HTTPStatus); `TestSearchBlueskyConnectionRefused` (httptest.Server closed before request; HTTPStatus=0); `TestSearchBlueskyUnavailablePreservesUnderlyingError`. All in `search_test.go` + `client_test.go`. |
| REQ-ADP6-004 | Event-Driven | WHEN the Bluesky AppView returns HTTP 200 with a JSON body whose top-level structure contains both `error` and `message` string keys (the AT Protocol XRPC error envelope shape), the adapter SHALL return `(nil, &types.SourceError{Adapter:"bluesky", Category: types.CategoryPermanent, Cause: fmt.Errorf("bluesky: %s: %s", body.error, body.message)})`. The parser SHALL detect the XRPC error envelope BEFORE attempting to read the `posts` array — a body with `{"error": "InvalidRequest", "message": "..."}` is NOT a partial-result success even though the HTTP status is 200. | P0 | `TestSearchBlueskyXRPCErrorEnvelope` (httptest.Server returns 200 with `bluesky_search_response_xrpc_error.json` body `{"error":"InvalidRequest","message":"Cursor format is invalid"}`; assert `errors.Is(err, types.ErrPermanent)` AND error message contains `"InvalidRequest"` AND `"Cursor format is invalid"`); `TestParseSearchPostsXRPCErrorBeforePosts` (parser given a body with both `error` and `posts` keys; assert error is returned BEFORE any post is emitted). In `search_test.go` + `parse_test.go`. |
| REQ-ADP6-005 | Ubiquitous | The adapter SHALL transform each `posts[i]` entry from the Bluesky AppView response into one `types.NormalizedDoc` using the field mapping in §6.5, MUST set `RetrievedAt = time.Now().UTC()` at the moment of parsing, MUST leave `Hash = ""` (consumers compute via `CanonicalHash()`), MUST populate `Metadata` with at minimum the keys `{handle, post_uri, repost_count, like_count, posted_at, sub_source}`, MUST set `DocType = types.DocTypePost`, MUST set `Lang = record.langs[0]` when `len(record.langs) > 0` else `Lang = ""`, MUST construct `URL` deterministically as `"https://bsky.app/profile/" + handle + "/post/" + rkey` (or `did + "/post/" + rkey` when handle is empty). The cursor (when the response `cursor` field is non-empty) SHALL be returned as the second return value of `parseSearchPosts` so `Search` can surface it via `Metadata["next_cursor"]` on the LAST returned NormalizedDoc — consumers paginate by passing this opaque value as `q.Cursor` on the next call. | P0 | `TestParseSearchPostsFieldMapping` (table-driven over 5 fixtures); `TestParseSearchPostsConstructsBlueskyURL` (fixture with handle="alice.bsky.social" and AT-URI ending in "3jzfcijpj2z2a" → URL == "https://bsky.app/profile/alice.bsky.social/post/3jzfcijpj2z2a"); `TestParseSearchPostsFallsBackToDIDWhenHandleEmpty` (fixture with handle="" and DID="did:plc:abc..." → URL == "https://bsky.app/profile/did:plc:abc.../post/<rkey>"); `TestParseSearchPostsLangFromLangs` (fixture with langs=["ko"] → Lang=="ko"; fixture with langs=[] → Lang==""); `TestParseSearchPostsPaginationCursor` (fixture with `cursor=="abc123"` → returned NormalizedDocs[len-1].Metadata["next_cursor"] == "abc123"; earlier docs do NOT have the key); `TestParseSearchPostsNoCursorOnEmpty` (fixture with `cursor=""` → no doc has `next_cursor`); `TestParseSearchPostsHashEmpty` (every returned doc has `Hash == ""`); `TestParseSearchPostsMetadataKeys` (all 6 required keys present including `sub_source=="bluesky"`). All in `parse_test.go`. |
| REQ-ADP6-006 | Ubiquitous | The adapter SHALL set the `User-Agent` HTTP header on every outbound request to a non-default value of the form `usearch/<version> (+https://github.com/elymas/universal-search)` where `<version>` is supplied via `Options.UserAgentVersion` (default `"v0.1"`). The adapter SHALL set the `Accept` header to `application/json`. WHILE `public.api.bsky.app` is generally permissive about default Go User-Agents, setting a custom UA preserves the project-wide convention from ADP-001 REQ-ADP-009 and identifies traffic for operational debugging at the AppView's side. The X disabled adapter SHALL NOT issue any HTTP requests in v0; this REQ does NOT apply to the X sub-source's `Search` path. | P0 | `TestSearchBlueskySetsCustomUserAgent` (inspect captured `r.Header.Get("User-Agent")`; assert it starts with `"usearch/"` and contains `"(+https://github.com/elymas/universal-search)"`); `TestSearchBlueskySetsAcceptJSON` (assert `Accept: application/json`); `TestSearchBlueskyUserAgentVersionConfigurable` (Options.UserAgentVersion="v0.2-rc1" → header contains `"usearch/v0.2-rc1"`); `TestSearchXMakesNoHTTPRequest` (instrument a stub that fails the test if any request arrives; invoke X-disabled Search; assert no request observed). All in `client_test.go` + `search_test.go`. |
| REQ-ADP6-007 | Optional | WHERE `q.Lang` is non-empty, the adapter SHALL include `lang=<q.Lang>` in the request URL. WHERE `Query.Filters` contains an entry with `Key == "since"` AND `Value` parses as an RFC 3339 datetime (per `time.Parse(time.RFC3339, value)`), the adapter SHALL include `since=<value>` in the request URL. Filter keys other than `since` SHALL be silently ignored (no error returned). Malformed `since` values SHALL be silently dropped (no error, no `since` parameter added). The default behaviour (no filter, empty Lang) is to omit BOTH parameters entirely. This REQ applies to the Bluesky sub-source only. | P1 | `TestSearchBlueskyLangAdded` (q.Lang="en" → URL has `lang=en`); `TestSearchBlueskyLangOmittedWhenEmpty`; `TestSearchBlueskySinceFilterAdded` (Filters=[{since,"2026-04-01T00:00:00Z"}] → URL has `since=2026-04-01T00:00:00Z`); `TestSearchBlueskySinceFilterDroppedWhenMalformed` (Filters=[{since,"yesterday"}] → URL has no `since`); `TestSearchBlueskyUnknownFilterIgnored` (Filters=[{tag,"bluesky"}] → URL has no `tag` parameter). All in `search_test.go`. |
| REQ-ADP6-008 | Event-Driven | WHEN `(*Adapter).Search(ctx, q)` is invoked on the X instance, the adapter SHALL evaluate `os.Getenv("USEARCH_X_ENABLED")` (or the Options-injected `EnvLookup` callable when set, for testability). IF the value is NOT exactly the string `"true"` (case-sensitive), THEN the adapter SHALL return `(nil, &types.SourceError{Adapter:"x", Category: types.CategoryPermanent, Cause: ErrXDisabled})`. IF the value IS exactly `"true"`, THEN the adapter SHALL return `(nil, &types.SourceError{Adapter:"x", Category: types.CategoryPermanent, Cause: ErrXProviderNotConfigured})` (because no provider is wired in v0; SPEC-ADP-006-XENABLE will replace this branch with live behaviour). In BOTH branches, the adapter SHALL NOT issue any HTTP request. The adapter SHALL NOT panic regardless of env state. [HARD] All env-dependent acceptance tests for this REQ SHALL drive env state via `XOptions.EnvLookup` injection — they SHALL NOT mutate process env via `os.Setenv` or `t.Setenv` because `t.Setenv` is goroutine-unsafe under `-race` when sibling tests run in parallel (per Go testing docs at https://pkg.go.dev/testing#T.Setenv: "Setenv cannot be used in parallel tests"). | P0 | `TestSearchXDisabledByDefault` (Options.EnvLookup returns `""` → `errors.Is(err, ErrXDisabled)` AND `errors.Is(err, types.ErrPermanent)`); `TestSearchXDisabledNonTrueValues` (Options.EnvLookup table-driven over `["", "false", "yes", "1", "TRUE", "True"]`; for each → `errors.Is(err, ErrXDisabled)`); `TestSearchXEnabledNoProvider` (Options.EnvLookup returns `"true"` → `errors.Is(err, ErrXProviderNotConfigured)` AND `errors.Is(err, types.ErrPermanent)`); `TestSearchXNoHTTPRegardlessOfEnv` (request counter on a stub server is 0 across all simulated env states; uses Options.EnvLookup to drive states); `TestSearchXEnvLookupInjection` (Options.EnvLookup=func(_ string) string {return "true"} → ErrXProviderNotConfigured returned WITHOUT touching real os.Getenv — verified by NOT setting `USEARCH_X_ENABLED` in the test process and asserting the result is still ErrXProviderNotConfigured). All in `search_test.go`. |
| REQ-ADP6-009 | State-Driven | WHILE the same `*Adapter` instance (Bluesky OR X) is registered in the adapter registry and is being invoked concurrently from N goroutines (N ≥ 1), each `Search(ctx, q)` call SHALL execute independently with no shared mutable state across calls (the underlying `*http.Client` is goroutine-safe per Go stdlib; the adapter holds no per-call state; the env lookup for X is a stateless `os.Getenv` OR a pure `EnvLookup` closure captured at construction time); the cumulative effect SHALL be N independent dispatches with no race-detector alarms. WHEN both Bluesky and X Adapter instances are registered in the same registry and invoked concurrently from M caller goroutines (each goroutine driving both adapters via the registry), there SHALL be no shared mutable state between the two `*Adapter` instances; each instance holds its own `*http.Client`, its own `subSource` field, its own `envLookup` closure, and the cumulative effect across `2 * M` Search invocations SHALL be race-clean. [HARD] The X-instance acceptance tests for this REQ SHALL inject env state via `XOptions.EnvLookup` (NOT `t.Setenv` — see REQ-ADP6-008 rationale). | P0 | `TestSearchBlueskyConcurrentSafe` (50 goroutines × 1 Search against shared *Adapter pointing at one stub; race-detector clean under `-race`; stub observes 50 requests; every goroutine receives 25 valid `NormalizedDoc`s); `TestSearchXConcurrentSafe` (50 goroutines × 1 Search on shared *Adapter X instance constructed with `XOptions.EnvLookup=func(_ string) string {return ""}`; all 50 receive ErrXDisabled; zero HTTP requests; race clean); `TestSearchBothSubSourcesConcurrent` (one Bluesky instance + one X instance constructed with `EnvLookup` injection against same registry; 50 caller goroutines each invoke both adapters via registry.Get; race clean; Bluesky returns 25 docs each; X returns ErrXDisabled each; zero cross-pollination of `subSource` field). All in `search_test.go`. |
| REQ-ADP6-010 | Ubiquitous | The package SHALL emit ZERO new Prometheus metric families. ALL per-adapter observability comes from the registry's `wrappedAdapter` (`internal/adapters/registry.go:172-263`) which on every `Search` call emits one OTel `adapter.search` span, one Prometheus counter increment on `AdapterCalls{adapter,outcome}`, one Prometheus histogram observation on `AdapterCallDuration{adapter}`, and one slog record. The `adapter` label value SHALL be `"bluesky"` for the Bluesky instance and `"x"` for the X instance — TWO distinct label values consuming the existing `adapter` label cardinality budget per SPEC-OBS-001's allowlist (no allowlist amendment required). Both label values are bounded by the V1 14-adapter ceiling at `internal/obs/metrics/metrics.go`. The `outcome` label values consumed in v0 are `"success"` (Bluesky 200), `"rate_limited"` (Bluesky 429), `"unavailable"` (Bluesky 5xx / network), `"timeout"` (ctx deadline), and `"failure"` (Bluesky 4xx, XRPC error envelope, both X disabled errors) — all already in SPEC-OBS-001's outcome allowlist. No new label value is introduced. | P0 | `TestNoNewMetricFamilies` (snapshot `prometheus.Registry.Gather()` before constructing both adapters; snapshot after one Search invocation each; assert family-count delta is zero); `TestAdapterLabelValues` (after invoking Search on Bluesky and X, assert `AdapterCalls` collector reports observations under `adapter="bluesky"` AND `adapter="x"` only — no other adapter label values introduced by this package); `TestXFailureOutcomeMapping` (invoke X disabled Search; assert the registry-emitted Prometheus counter sees `outcome="failure"`, NOT a fabricated `"disabled"` value). All in `search_test.go` plus a metrics integration test. |

---

## 4. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| NFR-ADP6-001 | Performance (parse path) | The parse path `parseSearchPosts(body []byte, retrievedAt time.Time) ([]NormalizedDoc, string, error)` SHALL execute with mean wall-clock duration per op ≤ 5 ms over `go test -bench=BenchmarkParseSearchPosts25Docs -benchtime=10x -count=5 ./internal/adapters/social/...` on amd64; the median of the 5 runs is the assertion value (passes when ≤ 5 ms). The fixture is `bluesky_search_response.json` (~6KB, 25-post AppView envelope). Allocation count ≤ 20 per post parsed (≤ 500 allocs total for 25 posts) per the same benchmark's `allocs/op` field. The same floor analysis from SPEC-ADP-001 NFR-ADP-001 applies: the `pkg/types.NormalizedDoc.Metadata = map[string]any` contract from SPEC-CORE-001 forces a structural floor of ~17 allocs/doc; Bluesky's slightly richer Metadata (6 required keys vs Reddit's 6 vs HN's 4) keeps the budget within 20/post. Measured via `BenchmarkParseSearchPosts25Docs` in `internal/adapters/social/bench_test.go`. Benchmarks do not count toward coverage. |
| NFR-ADP6-002 | End-to-end Latency | The end-to-end `(*Adapter).searchBluesky` round-trip against the `httptest.Server` stub (no real network) SHALL complete with p95 ≤ 200 ms over 100 invocations, measured by `TestSearchBlueskyE2ELatencyStubP95` in `search_test.go` (sort durations ascending, assert `durations[94] ≤ 200ms`). The harder live-Bluesky p95 (≤ 2s; AppView is fast) is documented as the operational target but is NOT enforced in CI (no live network). The X disabled path is bounded by NOT-issuing-HTTP discipline; its p95 is measured by `TestSearchXDisabledLatencyP95` and SHALL be ≤ 1ms over 100 invocations (effectively just an env lookup + struct construction). |
| NFR-ADP6-003 | No goroutine leak on cancellation | The adapter SHALL NOT leak any goroutine when the caller's context is cancelled mid-`Search`. Verified by `TestSearchBlueskyNoGoroutineLeakOnCancel` in `search_test.go`, which uses `go.uber.org/goleak.VerifyNone(t)` after a `Search` call whose ctx is cancelled mid-flight via a 50ms-delayed cancel; assert zero residual goroutines after the call returns. `internal/adapters/social/bench_test.go::TestMain` SHALL invoke `goleak.VerifyTestMain(m)` (mirrors ADP-001 / ADP-002 pattern) covering both Bluesky and X-disabled paths. |
| NFR-ADP6-004 | Race-clean across sub-sources | `internal/adapters/social/search_test.go::TestSearchBothSubSourcesConcurrent` SHALL execute successfully under `go test -race ./internal/adapters/social/...` with the workload defined in REQ-ADP6-009: one Bluesky instance plus one X instance registered in the same registry, 50 caller goroutines each invoking BOTH adapters via the registry. Race-detector alarms attributable to the `internal/adapters/social` package SHALL be zero. Cumulative call count: 100 adapter Search invocations (50 Bluesky + 50 X). |

---

## 5. Acceptance Criteria

### REQ-ADP6-001 — Adapter Interface Conformance (Both Sub-Sources)

- File `internal/adapters/social/social.go` declares `Adapter` struct
  with the documented fields (`httpClient *http.Client`, `baseURL
  string`, `userAgent string`, `healthcheckTarget string`, `subSource
  string`, `envLookup func(string) string`).
- The compile-time assertion `var _ types.Adapter = (*Adapter)(nil)`
  appears at the bottom of `social.go`. If the interface ever drifts,
  this assertion fails to compile.
- `social.NewBluesky(BlueskyOptions{})` returns an `*Adapter` with
  `subSource="bluesky"` and the documented Bluesky defaults.
- `social.NewX(XOptions{})` returns an `*Adapter` with `subSource="x"`,
  `httpClient=nil` (no HTTP path in v0), `envLookup=os.Getenv` by default.
- `(*Adapter).Name()` returns `"bluesky"` for the Bluesky instance and
  `"x"` for the X instance.
- Bluesky `(*Adapter).Capabilities()` returns the documented descriptor
  (REQ-ADP6-001 acceptance summary).
- X `(*Adapter).Capabilities()` returns the documented descriptor
  including `Notes` containing `"DISABLED in v0"`,
  `"USEARCH_X_ENABLED=true"`, `"SPEC-ADP-006-XENABLE pending"`.
- Capabilities determinism: two consecutive calls return
  `reflect.DeepEqual` results for both instances.
- `(*Adapter).Healthcheck(ctx)` succeeds against an httptest.Server
  bound to `127.0.0.1:0` for both instances when
  `Options.HealthcheckTarget` is set to that loopback address.
- All REQ-ADP6-001 tests pass.

### REQ-ADP6-002 — Bluesky Search Happy Path + Empty-Query Rejection

- `TestSearchBlueskyHappyPath25Posts` against
  `testdata/bluesky_search_response.json` returns exactly 25
  `NormalizedDoc` entries; each passes `Validate()` returning nil; the
  captured request URL contains all 3 mandatory query parameters
  (`q`, `limit`, `sort=top`).
- URL composition tests pass (clamp limit, default limit, cursor
  inclusion, lang inclusion).
- `TestSearchBlueskyEmptyQueryRejectedNoHTTP` asserts zero HTTP
  requests under empty/whitespace `q.Text`, returns `ErrPermanent`
  wrapping `ErrInvalidQuery`.

### REQ-ADP6-003 — HTTP Error Mapping (Bluesky)

- 429 with integer Retry-After: `RetryAfter=30s`, Category=RateLimited,
  HTTPStatus=429.
- 429 with HTTP-date Retry-After: `RetryAfter ∈ (25s, 35s)` for a
  30s-future date.
- 429 without header: defaults to 5s.
- 429 with overlong header (`Retry-After: 999`): capped at 60s.
- 429 with NO internal retry: server request count == 1.
- 4xx (400/401/403/404): ErrPermanent + matching HTTPStatus.
- 5xx (500/503): ErrSourceUnavailable + matching HTTPStatus.
- Connection refused: ErrSourceUnavailable, HTTPStatus=0.
- Underlying error preserved through `errors.Unwrap`.

### REQ-ADP6-004 — XRPC Error Envelope Detected on HTTP 200

- `TestSearchBlueskyXRPCErrorEnvelope`: HTTP 200 with body
  `{"error":"InvalidRequest","message":"Cursor format is invalid"}`
  yields `ErrPermanent` and the error string contains
  `"InvalidRequest"` AND `"Cursor format is invalid"`.
- `TestParseSearchPostsXRPCErrorBeforePosts`: parser detects the
  error envelope BEFORE attempting to read `posts`. A body that
  contains both `error` and `posts` MUST emit the error and zero
  docs.

### REQ-ADP6-005 — NormalizedDoc Field Mapping (Bluesky)

- `TestParseSearchPostsFieldMapping` table-drives 5 fixtures (typical
  post, langs=[], handle="" with DID fallback, high engagement,
  zero engagement). For each, asserts every NormalizedDoc field per
  §6.5 mapping table.
- `TestParseSearchPostsConstructsBlueskyURL`: handle + rkey →
  `"https://bsky.app/profile/" + handle + "/post/" + rkey`.
- `TestParseSearchPostsFallsBackToDIDWhenHandleEmpty`: empty handle
  uses DID in URL.
- Lang mapping: first element of `record.langs` or `""`.
- Pagination cursor on last doc only when `cursor != ""`.
- Hash empty on every returned doc.
- Required Metadata keys present: `handle`, `post_uri`,
  `repost_count`, `like_count`, `posted_at`, `sub_source` (=`bluesky`).

### REQ-ADP6-006 — User-Agent and Accept Headers (Bluesky); No HTTP for X

- `TestSearchBlueskySetsCustomUserAgent`: UA starts with `"usearch/"` +
  contains `"(+https://github.com/elymas/universal-search)"`.
- `TestSearchBlueskySetsAcceptJSON`: `Accept: application/json`.
- `TestSearchBlueskyUserAgentVersionConfigurable`: Options override
  propagates.
- `TestSearchXMakesNoHTTPRequest`: across all env states, a stub
  server that fails on any request observation reports zero requests.

### REQ-ADP6-007 — Optional Filters (Bluesky)

- `lang` filter inclusion / omission.
- `since` filter parsing (RFC 3339 valid → URL inclusion; malformed →
  silent drop).
- Unknown filter keys ignored.

### REQ-ADP6-008 — X Env-Gate Two-State Semantics

- Env unset → ErrXDisabled.
- Env set to non-`"true"` value (table over 6 case variations) →
  ErrXDisabled.
- Env set to exactly `"true"` → ErrXProviderNotConfigured.
- Both errors satisfy `errors.Is(err, types.ErrPermanent)`.
- `Options.EnvLookup` injection works for testability without env
  mutation.
- Zero HTTP requests under any env state.

### REQ-ADP6-009 — Concurrent Search Safety (State-Driven)

- `TestSearchBlueskyConcurrentSafe`: 50 goroutines, race-clean,
  stub observes 50 requests, every goroutine receives 25 valid
  NormalizedDocs.
- `TestSearchXConcurrentSafe`: 50 goroutines, race-clean, all 50
  receive ErrXDisabled, zero HTTP requests.
- `TestSearchBothSubSourcesConcurrent`: 50 caller goroutines each
  invoking both Bluesky and X via registry; race-clean; no shared
  mutable state across instances; cumulative 100 adapter invocations.

### REQ-ADP6-010 — Observability Discipline

- `TestNoNewMetricFamilies`: registry `Gather()` snapshot delta is
  zero across construct + Search invocation.
- `TestAdapterLabelValues`: `AdapterCalls` collector observed under
  `adapter="bluesky"` and `adapter="x"` only — no other label values
  fabricated by this package.
- `TestXFailureOutcomeMapping`: X disabled errors emit
  `outcome="failure"` (NOT a fabricated `"disabled"` value).

### NFR-ADP6-001 — Parse-Path Performance

- `BenchmarkParseSearchPosts25Docs` invoked as
  `go test -bench=BenchmarkParseSearchPosts25Docs -benchtime=10x -count=5 ./internal/adapters/social/...`
  on amd64.
- Median of 5 runs: ≤ 5 ms per op.
- `allocs/op ≤ 500` (≤ 20/post × 25 posts).

### NFR-ADP6-002 — E2E p95 (Stub) for Both Sub-Sources

- `TestSearchBlueskyE2ELatencyStubP95`: 100 invocations against stub;
  `durations[94] ≤ 200ms`.
- `TestSearchXDisabledLatencyP95`: 100 invocations of X-disabled path;
  `durations[94] ≤ 1ms`.

### NFR-ADP6-003 — Goroutine Leak Check

- `TestSearchBlueskyNoGoroutineLeakOnCancel`: `goleak.VerifyNone(t)`
  succeeds after mid-flight ctx cancel.
- `TestMain` in `bench_test.go` invokes `goleak.VerifyTestMain(m)`.

### NFR-ADP6-004 — Race-Clean Across Sub-Sources

- `TestSearchBothSubSourcesConcurrent` (REQ-ADP6-009 third assertion)
  executes under `go test -race`; race-detector alarms attributable
  to `internal/adapters/social` = 0.

---

## 6. Technical Approach

### 6.1 Files to Modify

**Created (16 files + 7 testdata fixtures)**:
- `internal/adapters/social/social.go` — Adapter struct, NewBluesky,
  NewX, Name, Capabilities, Healthcheck, compile-time assertion
- `internal/adapters/social/social_test.go` — interface conformance
- `internal/adapters/social/search.go` — sub-source dispatch
- `internal/adapters/social/search_bluesky.go` — Bluesky live path
- `internal/adapters/social/search_x.go` — X env-gated stub
- `internal/adapters/social/search_test.go` — main test file
- `internal/adapters/social/client.go` — HTTP client + categorizeStatus
- `internal/adapters/social/client_test.go` — HTTP error mapping
- `internal/adapters/social/parse.go` — parseSearchPosts
- `internal/adapters/social/parse_test.go` — field mapping
- `internal/adapters/social/url.go` — constructBlueskyURL, parseATURI
- `internal/adapters/social/url_test.go` — URL helper tests
- `internal/adapters/social/score.go` — Tanh formula
- `internal/adapters/social/score_test.go` — score normalization
- `internal/adapters/social/errors.go` — sentinels + parseRetryAfter
- `internal/adapters/social/errors_test.go` — sentinel tests
- `internal/adapters/social/bench_test.go` — benchmark + TestMain
- `internal/adapters/social/testdata/bluesky_search_response.json`
- `internal/adapters/social/testdata/bluesky_search_response_empty.json`
- `internal/adapters/social/testdata/bluesky_search_response_pagination.json`
- `internal/adapters/social/testdata/bluesky_search_response_with_lang.json`
- `internal/adapters/social/testdata/bluesky_search_response_high_engagement.json`
- `internal/adapters/social/testdata/bluesky_search_response_xrpc_error.json`
- `internal/adapters/social/testdata/bluesky_search_response_malformed.json`

**Modified**: none. The adapter self-contains. No cross-package
changes are required: `pkg/types` already publishes the contract,
`internal/adapters/registry.go` already accepts any `types.Adapter`,
`internal/obs/metrics/metrics.go` already declares `AdapterCalls` and
`AdapterCallDuration` collectors with `adapter` and `outcome` in the
cardinality allowlist (the two new label values `"bluesky"` and `"x"`
fit within the V1 14-adapter ceiling).

**Unchanged (by design)**:
- `internal/adapters/registry.go:172-263` — wrappedAdapter
  emits ALL observability for SPEC-ADP-006's `Search` calls. The
  adapter itself emits nothing.
- `pkg/types/{adapter.go, capabilities.go, query.go,
  normalized_doc.go, errors.go}` — no contract change.
- `internal/obs/metrics/metrics.go` — no new metric family.
- `cmd/usearch/main.go` — registry construction and adapter
  registration is owned by SPEC-CLI-001 (M2). ADP-006 does not modify
  cmd code; the adapter is consumed by future SPEC-CLI work for M3
  fanout integration.
- `.moai/project/structure.md` — current text reserves
  `internal/adapters/{xtwitter,bluesky}/`. ADP-006 consolidates into
  a single `internal/adapters/social/` package; a follow-up sync task
  in structure.md is recommended (Open Question §11.7). The
  consolidation is a net simplification (one package vs two for a
  conceptually-related set of social platforms).

### 6.2 Package Layout

```
internal/adapters/social/
├── social.go                                 # Adapter, NewBluesky, NewX, Name, Capabilities, Healthcheck, interface assertion
├── social_test.go                            # Interface conformance + Capabilities determinism + Name routing
├── search.go                                 # (*Adapter).Search hot path; sub-source dispatch
├── search_bluesky.go                         # searchBluesky URL build + HTTP execute + parse delegation
├── search_x.go                               # searchXDisabled env-gated stub
├── search_test.go                            # E2E + happy path + error categorisation tests + concurrent safety
├── client.go                                 # *http.Client, doRequest, categorizeStatus
├── client_test.go                            # categorizeStatus table + redirect allowlist + headers
├── parse.go                                  # parseSearchPosts (Bluesky AppView envelope)
├── parse_test.go                             # Field mapping table tests
├── url.go                                    # constructBlueskyURL + parseATURI helpers
├── url_test.go                               # URL helper table tests
├── score.go                                  # normalizeScore (Tanh formula, identical to ADP-001)
├── score_test.go                             # Score normalization table
├── errors.go                                 # ErrInvalidQuery + ErrInvalidCursor + ErrXDisabled + ErrXProviderNotConfigured + parseRetryAfter helper
├── errors_test.go                            # Sentinel + parseRetryAfter tests
├── bench_test.go                             # BenchmarkParseSearchPosts25Docs + TestMain (goleak)
└── testdata/
    ├── bluesky_search_response.json          # Happy path 25 posts
    ├── bluesky_search_response_empty.json    # Zero posts
    ├── bluesky_search_response_pagination.json # cursor set
    ├── bluesky_search_response_with_lang.json  # Mixed langs
    ├── bluesky_search_response_high_engagement.json # Score saturation case
    ├── bluesky_search_response_xrpc_error.json # XRPC error envelope
    └── bluesky_search_response_malformed.json # Truncated JSON
```

[NOTE on duplication vs sharing] `parseRetryAfter`, `categorizeStatus`,
and the redirect-allowlist pattern duplicate the equivalents in
`internal/adapters/reddit/` and `internal/adapters/hn/`. This duplication
is INTENTIONAL in v0:
- The two helpers are short (≤ 30 LoC each); duplication cost is small
  vs. premature shared package.
- ADP-001 was first, ADP-002 second, ADP-006 third. "Rule of three"
  arguably triggers now — but a refactor SPEC requires choosing a home
  (`internal/adapters/common/`?) and a shape that survives the
  remaining four M3 adapters. Defer to SPEC-ADP-REFAC-001 post-M3.

### 6.3 Bluesky Capabilities Descriptor (Detailed)

```go
types.Capabilities{
    SourceID:          "bluesky",
    DisplayName:       "Bluesky",
    DocTypes:          []types.DocType{types.DocTypePost},
    SupportedLangs:    nil,         // multi-lingual; Intent Router treats nil as "matches any language"
    SupportsSince:     true,        // since (RFC 3339) maps to AppView's `since` parameter
    RequiresAuth:      false,       // public AppView; anonymous
    AuthEnvVars:       nil,
    RateLimitPerMin:   600,         // 3000/5min IP limit per docs.bsky.app rate-limits page
    DefaultMaxResults: 25,
    Notes: "Bluesky public AppView (https://public.api.bsky.app) " +
        "via app.bsky.feed.searchPosts. social. sort=top hardcoded " +
        "(latest deferred). Lang filter from Query.Lang; since " +
        "filter (RFC 3339) from Query.Filters[since]. Cursor is " +
        "opaque; consumers MUST pass back as Query.Cursor unchanged. " +
        "No App Password (createSession deferred to SPEC-ADP-006-AUTH). " +
        "Rate limit: 3000/5min per IP (research §1.7).",
}
```

### 6.4 X Capabilities Descriptor (Disabled-State Detailed)

```go
types.Capabilities{
    SourceID:          "x",
    DisplayName:       "X (Twitter)",
    DocTypes:          []types.DocType{types.DocTypePost},
    SupportedLangs:    nil,
    SupportsSince:     false,
    RequiresAuth:      false,        // env-gate, not registry-time auth
    AuthEnvVars:       nil,
    RateLimitPerMin:   0,            // unknown / disabled
    DefaultMaxResults: 0,            // disabled
    Notes: "X (Twitter) social. DISABLED in v0. To enable, set " +
        "USEARCH_X_ENABLED=true env var AND wait for SPEC-ADP-006-XENABLE " +
        "to wire a provider. Until then, all Search calls return " +
        "ErrXDisabled (env unset/not-true) or ErrXProviderNotConfigured " +
        "(env=true). Both errors map to outcome=failure in the registry's " +
        "wrappedAdapter (no new label value). ScrapeCreators is the v1 " +
        "candidate provider; ToS review and explicit operator opt-in " +
        "required at deployment time.",
}
```

### 6.5 Bluesky Post → NormalizedDoc Field Mapping

| AppView postView Field | NormalizedDoc Field | Transform |
|------------------------|---------------------|-----------|
| `uri` | (used to derive `rkey`) | `parseATURI(uri)` returns `(did, "app.bsky.feed.post", rkey)` |
| `uri` | `Metadata["post_uri"]` | Use as-is |
| `cid` | `Metadata["cid"]` | Use as-is |
| `author.did` | `Metadata["did"]` | Use as-is |
| `author.handle` | `Author` | Use as-is |
| `author.handle` | `Metadata["handle"]` | Use as-is |
| `author.displayName` | `Metadata["display_name"]` | Use as-is (when present) |
| (constructed) | `ID` | `"bluesky:" + rkey` |
| (constant) | `SourceID` | `"bluesky"` (matches `Name()`) |
| (constructed) | `URL` | `"https://bsky.app/profile/" + (handle if non-empty else did) + "/post/" + rkey` |
| `truncateRunes(record.text, 280)` | `Title` | First 280 runes (Bluesky posts are at most 300 chars per AT spec, so this is essentially identity except for very long posts) |
| `record.text` | `Body` | Use as-is (plain text per AT spec; no HTML stripping) |
| `truncateRunes(record.text, 280)` | `Snippet` | First 280 runes |
| `record.createdAt` (parsed RFC 3339) | `PublishedAt` | `time.Parse(time.RFC3339, value).UTC()`; zero on parse error |
| `record.createdAt` | `Metadata["posted_at"]` | Original ISO string preserved |
| (parse time) | `RetrievedAt` | `time.Now().UTC()` (set by `parseSearchPosts` caller) |
| `normalizeScore(likeCount, repostCount)` | `Score` | Tanh formula identical to ADP-001 |
| `record.langs[0]` if present else `""` | `Lang` | First lang only; rest discarded in v0 |
| (constant) | `DocType` | `types.DocTypePost` |
| (nil) | `Citations` | `nil` |
| (constructed) | `Metadata` | Map containing two key tiers. **REQUIRED keys** (consumers MAY rely on presence): `handle`, `post_uri` (the AT-URI), `repost_count` (int), `like_count` (int), `posted_at` (ISO string), `sub_source` (= `"bluesky"`). REQ-ADP6-005 enforces these 6. **OPTIONAL keys** (MAY be present; consumers SHALL NOT assume): `cid`, `did`, `display_name`, `reply_count`, `quote_count`, `indexed_at`, `langs` (full slice), `labels` (slice if non-empty). The LAST returned doc additionally gets `next_cursor` (REQUIRED on the last doc only) when the response `cursor` field is non-empty. |
| (constant) | `Hash` | `""` (consumers compute via `CanonicalHash()`) |

### 6.6 Type Sketches (illustrative; final shapes in run phase)

```go
// internal/adapters/social/social.go
package social

import (
    "context"
    "fmt"
    "net"
    "net/http"
    "os"

    "github.com/elymas/universal-search/pkg/types"
)

const (
    defaultBaseURL           = "https://public.api.bsky.app/xrpc/app.bsky.feed.searchPosts"
    defaultUserAgentTemplate = "usearch/%s (+https://github.com/elymas/universal-search)"
    defaultUAVersion         = "v0.1"
    defaultBlueskyHealthcheckTarget = "public.api.bsky.app:443"
    defaultXHealthcheckTarget       = "x.com:443" // best-effort; X path is non-functional in v0
)

// BlueskyOptions configures the Bluesky sub-source.
type BlueskyOptions struct {
    BaseURL           string
    HTTPClient        *http.Client
    UserAgentVersion  string
    HealthcheckTarget string
}

// XOptions configures the X sub-source. v0 has no HTTP path; the
// EnvLookup hook is provided for testability of the env gate.
type XOptions struct {
    HealthcheckTarget string
    // EnvLookup replaces os.Getenv during testing. Default: os.Getenv.
    EnvLookup func(key string) string
}

type Adapter struct {
    httpClient        *http.Client
    baseURL           string
    userAgent         string
    healthcheckTarget string
    subSource         string                  // "bluesky" or "x"
    envLookup         func(string) string     // X-only; nil for bluesky
}

func NewBluesky(opts BlueskyOptions) (*Adapter, error) {
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
        client = newDefaultClient("bluesky")
    }
    target := opts.HealthcheckTarget
    if target == "" {
        target = defaultBlueskyHealthcheckTarget
    }
    return &Adapter{
        httpClient:        client,
        baseURL:           base,
        userAgent:         ua,
        healthcheckTarget: target,
        subSource:         "bluesky",
    }, nil
}

func NewX(opts XOptions) (*Adapter, error) {
    target := opts.HealthcheckTarget
    if target == "" {
        target = defaultXHealthcheckTarget
    }
    lookup := opts.EnvLookup
    if lookup == nil {
        lookup = os.Getenv
    }
    return &Adapter{
        httpClient:        nil, // no HTTP path in v0
        baseURL:           "",
        userAgent:         "",
        healthcheckTarget: target,
        subSource:         "x",
        envLookup:         lookup,
    }, nil
}

func (a *Adapter) Name() string { return a.subSource }

func (a *Adapter) Capabilities() types.Capabilities {
    if a.subSource == "x" {
        return blueprintCapabilitiesX()
    }
    return blueprintCapabilitiesBluesky()
}

func (a *Adapter) Healthcheck(ctx context.Context) error {
    var d net.Dialer
    conn, err := d.DialContext(ctx, "tcp", a.healthcheckTarget)
    if err != nil {
        return err
    }
    return conn.Close()
}

// search.go
func (a *Adapter) Search(ctx context.Context, q types.Query) ([]types.NormalizedDoc, error) {
    switch a.subSource {
    case "bluesky":
        return a.searchBluesky(ctx, q)
    case "x":
        return a.searchXDisabled(ctx, q)
    default:
        // Should never happen; constructors enforce.
        return nil, &types.SourceError{
            Adapter:  a.subSource,
            Category: types.CategoryUnknown,
            Cause:    fmt.Errorf("social: unknown sub-source %q", a.subSource),
        }
    }
}

// search_x.go
func (a *Adapter) searchXDisabled(ctx context.Context, _ types.Query) ([]types.NormalizedDoc, error) {
    if err := ctx.Err(); err != nil {
        return nil, &types.SourceError{Adapter: "x", Category: types.CategoryUnavailable, Cause: err}
    }
    if a.envLookup("USEARCH_X_ENABLED") != "true" {
        return nil, &types.SourceError{
            Adapter:  "x",
            Category: types.CategoryPermanent,
            Cause:    ErrXDisabled,
        }
    }
    return nil, &types.SourceError{
        Adapter:  "x",
        Category: types.CategoryPermanent,
        Cause:    ErrXProviderNotConfigured,
    }
}

var _ types.Adapter = (*Adapter)(nil)
```

### 6.7 HTTP Client Construction Notes

- **Timeout**: 10 seconds total request deadline (default). Caller's
  ctx deadline takes precedence when shorter.
- **Redirect policy**: `CheckRedirect` enforces the per-sub-source
  allowlist. For Bluesky: `{public.api.bsky.app, api.bsky.app,
  bsky.app}`, max 3 hops. For X: not applicable in v0 (no HTTP path).
  Cross-domain redirects rejected with
  `*SourceError{CategoryPermanent}`.
- **Transport**: `reqid.NewTransport(http.DefaultTransport)` (mirrors
  ADP-001 §6.5). Required for observability correlation.
- **Headers per request**: `User-Agent: usearch/<version>
  (+https://github.com/elymas/universal-search)` and `Accept:
  application/json`. NO authentication header (public AppView).

### 6.8 Observability Note

Both Bluesky and X Adapter instances emit ZERO metrics, logs, and
spans of their own. ALL observability comes from the registry's
`wrappedAdapter` (`internal/adapters/registry.go:172-263`). The two
distinct `Name()` values produce two distinct `adapter` label values
(`"bluesky"`, `"x"`), each consuming the existing `adapter` label
allowlist budget. The `outcome` label values consumed are:

- `"success"` — Bluesky 200 happy path.
- `"rate_limited"` — Bluesky 429.
- `"unavailable"` — Bluesky 5xx, network errors.
- `"timeout"` — ctx.DeadlineExceeded.
- `"failure"` — Bluesky 4xx (incl. XRPC error envelope), X disabled
  (both ErrXDisabled and ErrXProviderNotConfigured).

ZERO new label values are introduced. SPEC-OBS-001's allowlist is
preserved.

### 6.9 MX Tag Plan

| File::Symbol | Tag | Reason |
|--------------|-----|--------|
| `social.go::(*Adapter).Search` (defined in `search.go`) | `@MX:ANCHOR` | Sole entry point for all social fanout calls (Bluesky + X). fan_in ≥ 3 (registry wrappedAdapter, FAN-001 fanout, tests). `@MX:REASON: contract boundary; sub-source dispatch logic; signature change ripples to FAN-001 + CLI-001 + SYN-001`. `@MX:SPEC: SPEC-ADP-006`. |
| `parse.go::parseSearchPosts` | `@MX:ANCHOR` | Every Bluesky doc passes through this single transform. fan_in = 1 (searchBluesky) but invariant-bearing. `@MX:REASON: NormalizedDoc field-mapping integrity gate for Bluesky social posts`. `@MX:SPEC: SPEC-ADP-006`. |
| `score.go::normalizeScore` and constants `tanhDivisor=100.0, scoreCenter=0.5` | `@MX:NOTE` | Documents the Tanh formula choice and tie-in to SPEC-IDX-001 RRF. Same formula as ADP-001 / ADP-002. |
| `client.go::categorizeStatus` | `@MX:NOTE` | The HTTP-status-to-Category rosetta. Future contributors look here when a new HTTP code needs handling for Bluesky. |
| `client.go::doRequest` | `@MX:WARN` | Outbound network call. Redirect allowlist enforces SSRF safety boundary. `@MX:REASON: removing the CheckRedirect guard re-opens SSRF`. `@MX:SPEC: SPEC-ADP-006`. |
| `client.go::allowedRedirectHosts` map | `@MX:NOTE` | The 3-entry redirect allowlist for Bluesky. Adding a host requires a security review. |
| `search_x.go::searchXDisabled` | `@MX:WARN` | Env-gated security boundary for ToS-grey integration. `@MX:REASON: bypassing the USEARCH_X_ENABLED gate or wiring a provider here without ToS review violates tech.md:147 mandate`. `@MX:SPEC: SPEC-ADP-006`. |
| `social.go::Adapter.subSource` field | `@MX:NOTE` | The dispatch key. Adding a third sub-source value requires a SPEC amendment. |

All tags are `[AUTO]`-prefixed (agent-generated), include
`@MX:SPEC: SPEC-ADP-006`, and follow `code_comments: en` per
`.moai/config/sections/language.yaml`. Per-file hard limit (3 ANCHOR +
5 WARN per `.moai/config/sections/mx.yaml`): respected.

### 6.10 Harness Level

Per `.moai/config/sections/harness.yaml` auto-routing: 10 EARS REQs
(8 × P0 + 2 × P1) + 4 NFRs touching 1 package
(`internal/adapters/social/`, ~17 source/test files + 7 testdata
fixtures) + zero cross-package edits + ONE new env var name
documented in Capabilities.Notes (no new config file) + zero
security/payment/PII keywords (the X disabled-by-default posture
HONOURS tech.md:147's ToS feature-flag mandate; no scraping shipped) =
**standard** harness level. Sprint Contract is OPTIONAL but
recommended. Evaluator profile `default` applies.

---

## 7. What NOT to Build (Exclusions)

[HARD] This SPEC explicitly excludes the following items. Each has a
known destination SPEC; this list prevents scope creep into ADP-006.

- **App Password / `com.atproto.server.createSession` authentication
  flow** for higher-rate-limit Bluesky access → future
  SPEC-ADP-006-AUTH. v0 uses `public.api.bsky.app` anonymously.
- **X (Twitter) provider integration** (ScrapeCreators, Nitter,
  official tier subscription) → future SPEC-ADP-006-XENABLE behind
  explicit ToS acknowledgement. v0 ships X reserved-and-disabled.
- **Per-source customisations specific to other M3 adapters** (arXiv,
  GitHub, YouTube, SearXNG, Naver, Daum, KoreaNewsCrawler, RSS,
  Polymarket) → SPEC-ADP-003 through SPEC-ADP-009.
- **Retry orchestration** → SPEC-FAN-001 (M3, approved).
- **Response caching** → SPEC-CACHE-001 (M3).
- **Result ranking, deduplication, RRF fusion across adapters** →
  SPEC-IDX-001 (M3).
- **Bluesky `record.embed` rich extraction** (image cards, external
  link previews, quoted-post unwrapping) → out of v0 hot path. Embeds
  surface in `Metadata["embed"]` only when present.
- **Bluesky `record.facets`** (mention/link/tag ranges in text) →
  surfaced opaque in `Metadata["facets"]`; structured extraction
  deferred.
- **Bluesky labels system / NSFW gating** → SPEC-AUTH-002 (M6) may
  layer tenant policy. Surface in `Metadata["labels"]` only.
- **`sort=latest` Bluesky chronological mode** → out of v0; hardcode
  `sort=top`.
- **`searchActors` (`/xrpc/app.bsky.actor.searchActors`)** → out of
  scope.
- **Bluesky firehose / jetstream subscription** → not a search adapter.
- **Live network integration tests in CI** → out of v0;
  `httptest.Server` + golden fixtures only. Optional env-gated live
  test (`-tags=integration` + `BSKY_LIVE=1`) deferred.
- **Korean tokenisation or language inference** → SPEC-IDX-003 (M3).
- **`pkg/llm` integration** — adapter does NOT call any LLM.
- **Cross-adapter helper extraction** (sharing helpers with reddit/hn
  packages) → out of v0; refactor SPEC after M3.
- **Per-adapter custom Prometheus metrics** → SPEC-OBS-001 allowlist
  amendment required; out of scope.
- **`outcome="disabled"` Prometheus label value** for X disabled state
  → would amend SPEC-OBS-001 outcome allowlist; out of scope.
- **Auto-disable adapter on N consecutive Unavailable** →
  SPEC-EVAL-002 (M8). Adapter is stateless.
- **Streaming Search results** → SPEC-SYN-004 (M4) if measured value.
- **HTTP / gRPC server exposure of social adapter** → SPEC-MCP-001
  (M7) and future SPEC-API-001.

---

## 8. TDD Plan

Development follows RED-GREEN-REFACTOR per
`quality.development_mode: tdd` (`.moai/config/sections/quality.yaml`).
Representative RED-phase tests, written before implementation,
grouped by REQ. Total: ~52 tests covering REQ-ADP6-001..010 + NFRs.
Coverage target: 85% per `quality.test_coverage_target`. Benchmarks
do not count toward coverage.

| # | Test | Layer | REQ | Assertion |
|---|------|-------|-----|-----------|
| 1 | `TestBlueskyName` | `social_test.go` | REQ-ADP6-001 | `(*Adapter).Name() == "bluesky"` |
| 2 | `TestXName` | `social_test.go` | REQ-ADP6-001 | `(*Adapter).Name() == "x"` |
| 3 | `TestBlueskyImplementsInterface` | `social_test.go` | REQ-ADP6-001 | Compile-time `var _ types.Adapter = (*Adapter)(nil)` |
| 4 | `TestXImplementsInterface` | `social_test.go` | REQ-ADP6-001 | Same — both share *Adapter |
| 5 | `TestBlueskyCapabilitiesDeterministic` | `social_test.go` | REQ-ADP6-001 | Two calls return DeepEqual |
| 6 | `TestXCapabilitiesDeterministic` | `social_test.go` | REQ-ADP6-001 | Two calls return DeepEqual |
| 7 | `TestBlueskyCapabilitiesShape` | `social_test.go` | REQ-ADP6-001 | All 9 documented field values + Notes substrings |
| 8 | `TestXCapabilitiesShape` | `social_test.go` | REQ-ADP6-001 | All 9 documented field values + Notes "DISABLED in v0" / "USEARCH_X_ENABLED=true" / "SPEC-ADP-006-XENABLE pending" |
| 9 | `TestBlueskyHealthcheckSucceeds` | `social_test.go` | REQ-ADP6-001 | TCP dial against test loopback succeeds |
| 10 | `TestXHealthcheckSucceeds` | `social_test.go` | REQ-ADP6-001 | Same |
| 11 | `TestSearchBlueskyHappyPath25Posts` | `search_test.go` | REQ-ADP6-002, REQ-ADP6-005 | 25 NormalizedDocs returned; each `Validate()` returns nil |
| 12 | `TestSearchBlueskyURLParametersIncludeAllRequired` | `search_test.go` | REQ-ADP6-002 | `q`, `limit`, `sort=top` always present |
| 13 | `TestSearchBlueskyClampsLimitTo100` | `search_test.go` | REQ-ADP6-002 | q.MaxResults=500 → URL has `limit=100` |
| 14 | `TestSearchBlueskyDefaultsLimitTo25` | `search_test.go` | REQ-ADP6-002 | q.MaxResults=0 → URL has `limit=25` |
| 15 | `TestSearchBlueskyOmitsCursorWhenEmpty` | `search_test.go` | REQ-ADP6-002 | q.Cursor="" → URL has no `cursor` |
| 16 | `TestSearchBlueskySetsCursorWhenPresent` | `search_test.go` | REQ-ADP6-002 | q.Cursor="abc123" → URL contains `&cursor=abc123` |
| 17 | `TestSearchBlueskyEmptyQueryRejectedNoHTTP` | `search_test.go` | REQ-ADP6-002 | Table over empty/whitespace q.Text → ErrPermanent + zero requests |
| 18 | `TestSearchBlueskyHTTP429WithIntegerRetryAfter` | `search_test.go` | REQ-ADP6-003 | `Retry-After: 30` → SourceError.RetryAfter==30s |
| 19 | `TestSearchBlueskyHTTP429WithHTTPDateRetryAfter` | `search_test.go` | REQ-ADP6-003 | HTTP-date 30s ahead → RetryAfter ∈ (25s, 35s) |
| 20 | `TestSearchBlueskyHTTP429NoRetryAfterDefaults5s` | `search_test.go` | REQ-ADP6-003 | No header → RetryAfter==5s |
| 21 | `TestSearchBlueskyHTTP429RetryAfterCapped60s` | `search_test.go` | REQ-ADP6-003 | `Retry-After: 999` → RetryAfter==60s |
| 22 | `TestSearchBlueskyHTTP429NoInternalRetry` | `search_test.go` | REQ-ADP6-003 | Server request count == 1 |
| 23 | `TestSearchBlueskyHTTP4xx` | `search_test.go` | REQ-ADP6-003 | Table over 400/401/403/404 → ErrPermanent + matching HTTPStatus |
| 24 | `TestSearchBlueskyHTTP5xx` | `search_test.go` | REQ-ADP6-003 | Table over 500/503 → ErrSourceUnavailable + matching HTTPStatus |
| 25 | `TestSearchBlueskyConnectionRefused` | `search_test.go` | REQ-ADP6-003 | `errors.Is(err, types.ErrSourceUnavailable)`; HTTPStatus==0 |
| 26 | `TestSearchBlueskyUnavailablePreservesUnderlyingError` | `search_test.go` | REQ-ADP6-003 | `errors.Unwrap(srcErr).Error()` contains inner cause |
| 27 | `TestSearchBlueskyXRPCErrorEnvelope` | `search_test.go` | REQ-ADP6-004 | HTTP 200 with XRPC error body → ErrPermanent + error string contains `"InvalidRequest"` |
| 28 | `TestParseSearchPostsXRPCErrorBeforePosts` | `parse_test.go` | REQ-ADP6-004 | Parser detects error envelope before reading posts |
| 29 | `TestParseSearchPostsFieldMapping` | `parse_test.go` | REQ-ADP6-005 | Table over 5 fixtures; every documented field maps correctly |
| 30 | `TestParseSearchPostsConstructsBlueskyURL` | `parse_test.go` | REQ-ADP6-005 | handle + rkey → `https://bsky.app/profile/<handle>/post/<rkey>` |
| 31 | `TestParseSearchPostsFallsBackToDIDWhenHandleEmpty` | `parse_test.go` | REQ-ADP6-005 | Empty handle uses DID in URL |
| 32 | `TestParseSearchPostsLangFromLangs` | `parse_test.go` | REQ-ADP6-005 | langs[0] or "" |
| 33 | `TestParseSearchPostsPaginationCursor` | `parse_test.go` | REQ-ADP6-005 | last doc's Metadata has `next_cursor` when response cursor non-empty |
| 34 | `TestParseSearchPostsNoCursorOnEmpty` | `parse_test.go` | REQ-ADP6-005 | Empty cursor → no doc has key |
| 35 | `TestParseSearchPostsHashEmpty` | `parse_test.go` | REQ-ADP6-005 | Every NormalizedDoc.Hash == "" |
| 36 | `TestParseSearchPostsMetadataKeys` | `parse_test.go` | REQ-ADP6-005 | All 6 required Metadata keys present (handle, post_uri, repost_count, like_count, posted_at, sub_source=="bluesky") |
| 37 | `TestParseSearchPostsMalformedJSON` | `parse_test.go` | REQ-ADP6-005 | Truncated JSON → `*SourceError{Category: CategoryPermanent}` |
| 38 | `TestSearchBlueskySetsCustomUserAgent` | `client_test.go` | REQ-ADP6-006 | UA starts with "usearch/" + contains URL |
| 39 | `TestSearchBlueskySetsAcceptJSON` | `client_test.go` | REQ-ADP6-006 | `Accept: application/json` header present |
| 40 | `TestSearchBlueskyUserAgentVersionConfigurable` | `client_test.go` | REQ-ADP6-006 | Options override propagates |
| 41 | `TestSearchXMakesNoHTTPRequest` | `search_test.go` | REQ-ADP6-006 | Across all env states, stub server observes zero requests |
| 42 | `TestSearchBlueskyLangAdded` / `TestSearchBlueskyLangOmittedWhenEmpty` | `search_test.go` | REQ-ADP6-007 | URL `lang` parameter inclusion logic |
| 43 | `TestSearchBlueskySinceFilterAdded` | `search_test.go` | REQ-ADP6-007 | RFC 3339 valid → URL includes `since` |
| 44 | `TestSearchBlueskySinceFilterDroppedWhenMalformed` | `search_test.go` | REQ-ADP6-007 | Malformed → silent drop |
| 45 | `TestSearchBlueskyUnknownFilterIgnored` | `search_test.go` | REQ-ADP6-007 | Unknown key → no URL change |
| 46 | `TestSearchXDisabledByDefault` | `search_test.go` | REQ-ADP6-008 | env unset → ErrXDisabled |
| 47 | `TestSearchXDisabledNonTrueValues` | `search_test.go` | REQ-ADP6-008 | Table over `["", "false", "yes", "1", "TRUE", "True"]` → ErrXDisabled |
| 48 | `TestSearchXEnabledNoProvider` | `search_test.go` | REQ-ADP6-008 | env="true" → ErrXProviderNotConfigured |
| 49 | `TestSearchXNoHTTPRegardlessOfEnv` | `search_test.go` | REQ-ADP6-008 | Zero HTTP requests under any env state |
| 50 | `TestSearchXEnvLookupInjection` | `search_test.go` | REQ-ADP6-008 | Options.EnvLookup callable for testability without env mutation |
| 51 | `TestSearchBlueskyConcurrentSafe` | `search_test.go` | REQ-ADP6-009, NFR-ADP6-004 | 50 goroutines × 1 Search; race-clean; 50 stub requests; 25 valid docs each |
| 52 | `TestSearchXConcurrentSafe` | `search_test.go` | REQ-ADP6-009, NFR-ADP6-004 | 50 goroutines × 1 Search; race-clean; all receive ErrXDisabled; zero HTTP |
| 53 | `TestSearchBothSubSourcesConcurrent` | `search_test.go` | REQ-ADP6-009, NFR-ADP6-004 | 50 caller goroutines invoking BOTH adapters via registry; race-clean; 100 cumulative invocations |
| 54 | `TestNoNewMetricFamilies` | `search_test.go` (or new observability_test.go) | REQ-ADP6-010 | Gather() before+after delta == 0 |
| 55 | `TestAdapterLabelValues` | `search_test.go` | REQ-ADP6-010 | `AdapterCalls` collector observed under `adapter="bluesky"` AND `adapter="x"` only |
| 56 | `TestXFailureOutcomeMapping` | `search_test.go` | REQ-ADP6-010 | X disabled → registry counter sees `outcome="failure"` |
| 57 | `TestNormalizeScoreTable` | `score_test.go` | REQ-ADP6-005 | 7 (like, repost) tuples → expected `[0,1]` outputs within ±0.001 |
| 58 | `TestNormalizeScoreDeterministic` | `score_test.go` | REQ-ADP6-005 | Two calls on same input return byte-equal output |
| 59 | `TestParseATURITable` | `url_test.go` | REQ-ADP6-005 | Table over 6 inputs (typical, missing scheme, missing collection, missing rkey, empty, malformed) |
| 60 | `TestConstructBlueskyURLTable` | `url_test.go` | REQ-ADP6-005 | Table over 4 inputs (with handle, DID-fallback, empty rkey, malformed AT-URI) |
| 61 | `TestParseRetryAfterTable` | `errors_test.go` | REQ-ADP6-003 | Table over 6 inputs |
| 62 | `TestCategorizeStatusTable` | `client_test.go` | REQ-ADP6-003 | Truth table over 7 status codes for both adapter names |
| 63 | `TestSearchBlueskyFollowsAllowlistRedirect` | `client_test.go` | REQ-ADP6-003 | 302 within allowlist followed |
| 64 | `TestSearchBlueskyRejectsCrossDomainRedirect` | `client_test.go` | REQ-ADP6-003 | 302 to attacker.com → ErrPermanent + "cross-domain redirect" |
| 65 | `TestSearchBlueskyRejectsRedirectChainOver3` | `client_test.go` | REQ-ADP6-003 | 4-hop chain rejected |
| 66 | `TestSearchBlueskyE2ELatencyStubP95` | `search_test.go` | NFR-ADP6-002 | 100 invocations against stub; p95 ≤ 200ms |
| 67 | `TestSearchXDisabledLatencyP95` | `search_test.go` | NFR-ADP6-002 | 100 invocations of X-disabled path; p95 ≤ 1ms |
| 68 | `TestSearchBlueskyNoGoroutineLeakOnCancel` | `search_test.go` | NFR-ADP6-003 | `goleak.VerifyNone(t)` after mid-flight ctx cancel |
| 69 | `BenchmarkParseSearchPosts25Docs` | `bench_test.go` | NFR-ADP6-001 | Median of 5 runs ≤ 5ms; allocs/op ≤ 500 |
| 70 | `TestMain` (goleak.VerifyTestMain) | `bench_test.go` | NFR-ADP6-003 | Package-level goroutine leak check |

RED-GREEN-REFACTOR per requirement:
1. RED: Write failing test for REQ-ADP6-N.
2. GREEN: Implement minimal code to pass.
3. REFACTOR: Tidy; extract shared helpers if they remove duplication
   WITHIN the package; keep file sizes manageable (target each `.go`
   file < 200 LoC excluding tests).

Greenfield note: `internal/adapters/social/` does not exist. There is
no behaviour to preserve; no characterization tests needed.

---

## 9. Dependencies

### 9.1 Upstream SPEC Dependencies

- **SPEC-CORE-001 (implemented)**: provides `pkg/types.Adapter`,
  `pkg/types.Capabilities`, `pkg/types.Query`,
  `pkg/types.NormalizedDoc`, `*types.SourceError`,
  `types.OutcomeFromError`, `types.DocType` enum,
  `internal/adapters.Registry` with wrappedAdapter sole-emitter
  pattern, `internal/adapters/noop` reference shape. HARD dep.
- **SPEC-OBS-001 (implemented)**: provides `obs.Obs` bundle,
  `internal/obs/reqid.NewTransport` for request-ID propagation,
  `AdapterCalls{adapter,outcome}` and `AdapterCallDuration{adapter}`
  collectors. SOFT dep — adapter is nil-safe via the registry's
  nil-guards. The two new `adapter` label values (`"bluesky"`, `"x"`)
  fit within the V1 14-adapter ceiling.
- **SPEC-IR-001 (implemented)**: documents the consumer contract for
  `Capabilities` (REQ-IR-008 selects AdapterSet by intersecting
  `categoryEligibleDocTypes` with `SupportedLangs`). ADP-006's
  `Capabilities()` shape (DocTypes=[DocTypePost], SupportedLangs=nil)
  determines that BOTH Adapter instances are selected for
  `Category=social` queries. SOFT dep.

### 9.2 Parallelizable

- **SPEC-ADP-003 / 004 / 005 / 007 / 008 / 009 (all M3)**: develop
  in parallel per `roadmap.md:122-123` (gated on SPEC-FAN-001, which
  is already approved at status: approved). ADP-006 and the other six
  ADP-* SPECs share the reference shape from ADP-001/002 and do not
  step on each other's package directories.
- **SPEC-IDX-001 (M3)**: can plan in parallel; consumes
  `[]NormalizedDoc` shape (already locked in CORE-001).

### 9.3 Downstream Blocked SPECs

- **SPEC-CLI-001** (M2, follow-up integration): registers the social
  adapters into `cmd/usearch/main.go`. The two `Adapter` instances
  register independently.
- **SPEC-FAN-001** (M3, approved): consumes
  `(*social.Adapter).Search` for both sub-sources via
  `registry.Get("bluesky").Search(ctx, q)` and
  `registry.Get("x").Search(ctx, q)`. With both adapters registered,
  the `Result.AdapterErrors["x"]` will always be non-nil in v0
  (containing `ErrXDisabled` or `ErrXProviderNotConfigured`); FAN-001's
  partial-result assembly handles this uniformly per REQ-FAN-003.
- **SPEC-IDX-001** (M3): consumes `NormalizedDoc.Score`
  (Tanh-normalised in ADP-006) as one input to RRF fusion across
  adapters.
- **SPEC-ADP-006-AUTH** (deferred): future App Password integration.
- **SPEC-ADP-006-XENABLE** (deferred): future X provider integration.

### 9.4 External Dependencies (run-phase pins)

**Zero new Go module dependencies.** ADP-006 uses only:
- Go stdlib: `context`, `encoding/json`, `errors`, `fmt`, `io`,
  `math`, `net`, `net/http`, `net/url`, `os`, `strconv`, `strings`,
  `time`, `unicode`, `unicode/utf8`
- `pkg/types` (already pinned via SPEC-CORE-001)
- `internal/obs/reqid` (already pinned via SPEC-OBS-001)
- Test-only: `go.uber.org/goleak` (already added by SPEC-ADP-001
  run-phase; reused).

The `github.com/bluesky-social/indigo` Go library is explicitly NOT
adopted (research §1.9). ScrapeCreators integration is NOT shipped
in v0 (research §2.3).

---

## 10. Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Bluesky XRPC error envelope on HTTP 200 misinterpreted as partial-result success | Medium | Medium | REQ-ADP6-004 + `parseSearchPosts` checks for `error` JSON key BEFORE looking for `posts` array. Tested in `TestSearchBlueskyXRPCErrorEnvelope` and `TestParseSearchPostsXRPCErrorBeforePosts`. |
| Bluesky Lexicon shape drift (new field, renamed field) | Low | Medium | `encoding/json` ignores unknown fields; the adapter reads only documented fields. Lexicon-versioned shapes in atproto are stable by contract. The fixtures are static; if the AppView ever drifts the documented shape, it's a v2 SPEC concern. |
| AT-URI parsing fails on edge cases (handle vs DID in URI, missing collection segment) | Medium | Low | `parseATURI` returns the rkey from the LAST URI segment regardless. URL construction prefers `author.handle` (always present in `postView` per AT spec); falls back to DID. `TestParseATURITable` covers 6 input shapes. |
| Cross-domain redirect (open SSRF) on Bluesky | Low | High | `redirectAllowlist` enforces 3-host allowlist; cross-domain rejected. `TestSearchBlueskyRejectsCrossDomainRedirect` verifies. |
| `record.text` longer than 280 runes (uncommon but possible if Lexicon update relaxes limits) | Low | Low | `Snippet = truncateRunes(record.text, 280)`. |
| `record.langs` empty | Low | Low | `Lang = ""`; `Validate()` does not require Lang. |
| `author.handle` empty for deleted-but-cached actors | Medium | Low | Falls back to `author.did` for URL construction; documented in §6.5. |
| X env var typo (`USEARCH_X_ENABLE` vs `USEARCH_X_ENABLED`) | Medium | Low | Env var name is HARD constant in code; documented in `Capabilities.Notes`. Operator-side issue. The Adapter does not warn on near-miss env names in v0 (would require fuzzy logic; out of scope). |
| Operator sets `USEARCH_X_ENABLED=true` expecting v0 to work | High | Low | `ErrXProviderNotConfigured` message explicitly references "SPEC-ADP-006-XENABLE pending". `Capabilities.Notes` documents v0 disabled state. Tests assert exact error message substrings. |
| Concurrent calls on shared `*http.Client` race | Low | High | `*http.Client` goroutine-safe per Go stdlib. `TestSearchBlueskyConcurrentSafe` runs under `go test -race`. NFR-ADP6-004 covers cross-sub-source race. |
| `parseATURI` panics on malformed input | Low | Medium | All paths return `error`; no panics. Fuzz-style table test covers malformed inputs. |
| Bluesky 429 sustained (e.g., shared CGNAT IP) | Medium | Medium | `parseRetryAfter` honours `Retry-After`; SPEC-FAN-001 retry orchestration is the supervisor. |
| Score formula too aggressive (saturates everything) on Bluesky | Medium | Low | Tanh divisor=100 means inflection at 100 engagements; max popular Bluesky posts ~5000 likes → ~1.0 saturation. Matches Reddit/HN posture. RRF re-weights via rank. |
| Test fixture rot (Lexicon shape change in production) | Low | Low | Fixtures are static; CI is offline. Live integration test deferred to env-gated `BSKY_LIVE=1`. |
| The dual-error-state for X (Disabled vs ProviderNotConfigured) creates operator confusion | Medium | Low | Both errors satisfy `errors.Is(err, types.ErrPermanent)` so monitoring treats them uniformly as `outcome="failure"`. The exact sentinel only matters for human debugging; the messages are explicit. |
| Goroutine leak on ctx cancel mid-Bluesky call | Low | High | NFR-ADP6-003 + `goleak.VerifyTestMain`. |
| ScrapeCreators ToS exposure if XENABLE is integrated and misconfigured in a future SPEC | High | High | v0 ships zero scraping. SPEC-ADP-006-XENABLE will require explicit ToS acknowledgement at deployment time. tech.md:147 mandate honoured. |
| `Metadata["next_cursor"]` opacity confuses downstream consumers | Low | Low | Documented in `Capabilities.Notes` and §6.5. Consumers MUST pass back as `Query.Cursor` without parsing — same convention as ADP-001/ADP-002. |
| Consolidation into `social/` vs structure.md's `xtwitter/` + `bluesky/` reservation | Medium | Low | Open Question §11.7 tracks the structure.md follow-up sync. The single-package consolidation is a net simplification; tech.md:107 (X) and tech.md:110 (Bluesky) rows remain accurate. |
| `time.Now()` in `RetrievedAt` non-deterministic in tests | Low | Low | `parseSearchPosts` accepts `retrievedAt time.Time` parameter; tests inject a fixed time. |
| Bluesky auth path silently activates if BaseURL changes to `api.bsky.app` | Low | High | `BaseURL` is HARD-pinned to `public.api.bsky.app` in `defaultBaseURL`; tests inject httptest stubs only. The redirect allowlist would catch a runtime drift to `api.bsky.app`. |

---

## 11. Open Questions

These are explicitly UNRESOLVED at SPEC-approval time. Each has a
recommended default. They do NOT block SPEC approval.

1. **App Password / `createSession` authentication for Bluesky**.
   **Recommended default**: NO in v0. Anonymous `public.api.bsky.app`
   exclusively. The 3,000-per-5min IP rate limit is generous; auth
   complexity (JWT rotation, refreshJwt management) is not justified.
   App Password is preferred over OAuth for non-interactive server use
   per `https://docs.bsky.app/docs/get-started`.
   **Resolution owner**: future SPEC-ADP-006-AUTH author.

2. **Score formula tuning for Bluesky**. `likeCount + repostCount` vs
   inclusion of `replyCount` / `quoteCount`.
   **Recommended default**: `likeCount + repostCount` only in v0. Like
   counts dominate engagement; the inflection at 100 matches
   Reddit/HN. RRF weights rank not score across adapters.
   **Resolution owner**: SPEC-IDX-001 author.

3. **`sort=top` vs `sort=latest` for Bluesky**.
   **Recommended default**: hardcode `sort=top` in v0. Mirrors Reddit
   `sort=relevance` and HN's relevance default. A
   `Query.Filters[Key="sort"]` switch is a clean follow-up.
   **Resolution owner**: future enhancement SPEC.

4. **X provider choice (when XENABLE happens)**. ScrapeCreators vs
   Nitter vs official tier.
   **Recommended default**: ScrapeCreators on opt-in tenant
   subscription (Freelance $47/mo for 25k requests; ToS reviewed at
   deployment time). Nitter is operationally fragile. Official
   Enterprise tier is cost-prohibitive.
   **Resolution owner**: future SPEC-ADP-006-XENABLE author.

5. **Bluesky NSFW / labels handling**.
   **Recommended default**: NO in v0. Labels surface in
   `Metadata["labels"]`; consumer-side filtering. Future SPEC-AUTH-002
   may add tenant policy.
   **Resolution owner**: SPEC-AUTH-002 author.

6. **Bluesky `record.embed` / `record.facets` rich extraction**.
   **Recommended default**: opaque pass-through to Metadata only;
   no structured extraction in v0. Run-phase MAY elide for allocation
   savings.
   **Resolution owner**: run-phase implementer; SPEC-SYN-001 may
   request embed surfacing if synthesis benefits.

7. **`internal/adapters/social/` consolidation vs structure.md's
   `xtwitter/` + `bluesky/` reservation**. The current SPEC consolidates
   into ONE package; structure.md:18-22 reserves two separate package
   names.
   **Recommended default**: consolidate into `social/`; sync
   structure.md to reflect the change in the next `/moai sync` after
   ADP-006 implementation lands.
   **Resolution owner**: docs-sync agent in next sync pass.

8. **X opt-in mechanism: ENV vs CLI flag vs config file**.
   `USEARCH_X_ENABLED=true` is the v0 mechanism.
   **Recommended default**: env var. Simplest, deployment-friendly,
   reversible per process. CLI flag layer can be added later.
   **Resolution owner**: SPEC-CLI-002 (M7) author may add a flag layer.

9. **`outcome="disabled"` Prometheus label value for X disabled state**.
   The current design emits `outcome="failure"` which may confuse
   dashboard operators expecting "x" failures to be real failures.
   **Recommended default**: ship with `outcome="failure"` in v0;
   document in `Capabilities.Notes`. Adding a `disabled` label value
   would amend SPEC-OBS-001 allowlist — out of scope.
   **Resolution owner**: SPEC-OBS-001 author may add a label value if
   operational complaints accumulate.

---

## 12. References

### External (URL-cited; verified per research.md §9)

- https://atproto.com/ — AT Protocol overview, public firehose,
  auth/lexicon section pointers.
- https://docs.bsky.app/ — Bluesky documentation home; SDK list
  (TypeScript / Python / Dart / Go).
- https://docs.bsky.app/docs/api/app-bsky-feed-search-posts — primary
  search endpoint metadata.
- https://docs.bsky.app/docs/api/app-bsky-actor-search-actors — actors
  search endpoint (forward-compat reference; v0 uses searchPosts only).
- https://docs.bsky.app/docs/advanced-guides/api-directory — service
  host table; `public.api.bsky.app` recommendation.
- https://docs.bsky.app/docs/advanced-guides/rate-limits — IP/account
  rate limits.
- https://docs.bsky.app/docs/get-started — `createSession` flow,
  accessJwt/refreshJwt mechanics.
- https://github.com/bluesky-social/indigo — official Go library;
  pre-stable disclaimer (REJECTED as dependency).
- https://www.scrapecreators.com/ — ScrapeCreators pricing, services,
  ToS-of-platform stance (DEFERRED to future SPEC).
- RFC 7231 §7.1.3 — `Retry-After` header semantics.

### Internal (file:line cited)

- `.moai/specs/SPEC-ADP-006/research.md` — full research artifact for
  this SPEC.
- `.moai/specs/SPEC-ADP-001/spec.md` — first reference adapter SPEC;
  this SPEC inherits structure verbatim.
- `.moai/specs/SPEC-ADP-002/spec.md` — second-adapter SPEC.
- `.moai/specs/SPEC-CORE-001/spec.md` — Adapter / Capabilities /
  Query / NormalizedDoc / SourceError contract.
- `.moai/specs/SPEC-OBS-001/spec.md` — observability bundle and
  cardinality discipline.
- `.moai/specs/SPEC-IR-001/spec.md` — `Capabilities` consumer
  contract (REQ-IR-008).
- `.moai/specs/SPEC-FAN-001/spec.md` — multi-source fanout (M3
  gateway, approved status).
- `pkg/types/adapter.go:28-45` — Adapter interface 4-method shape.
- `pkg/types/capabilities.go:38-62` — Capabilities struct + DocType
  enum.
- `pkg/types/query.go:18-44` — Query struct + Filter shape.
- `pkg/types/errors.go:14-218` — SourceError, Category, OutcomeFromError.
- `pkg/types/normalized_doc.go:40-106` — NormalizedDoc, Validate,
  CanonicalHash.
- `internal/adapters/registry.go:75-167` — Registry lifecycle.
- `internal/adapters/registry.go:172-263` — wrappedAdapter
  sole-emitter pattern.
- `internal/adapters/noop/noop.go:1-46` — reference adapter shape +
  compile-time interface assertion.
- `internal/adapters/reddit/reddit.go:1-136` — Adapter struct pattern
  (mirrored).
- `internal/adapters/reddit/search.go` — Search hot path pattern.
- `internal/adapters/reddit/parse.go` — parseListing pattern.
- `internal/adapters/reddit/client.go` — HTTP client + redirect
  allowlist + categorizeStatus pattern.
- `internal/adapters/reddit/score.go` — Tanh score formula
  (duplicated verbatim).
- `internal/adapters/reddit/errors.go` — parseRetryAfter helper
  (duplicated verbatim).
- `internal/adapters/hn/hn.go:1-138` — second Adapter struct pattern.
- `internal/adapters/hn/strip.go` — stripHTML helper (NOT reused —
  Bluesky text is plain).
- `internal/router/category.go:14-15` — `CategorySocial` definition;
  covers Bluesky + X.
- `internal/llm/client.go:31-65` — HTTP client construction pattern
  with timeout + reqid Transport wrapping.
- `internal/obs/metrics/metrics.go` — `AdapterCalls` and
  `AdapterCallDuration` collectors; `adapter`/`outcome` cardinality
  allowlist.
- `internal/obs/reqid` — request-ID propagation transport.
- `.moai/project/roadmap.md:51` — SPEC-ADP-006 row.
- `.moai/project/roadmap.md:122-123` — M3 7-way parallelization.
- `.moai/project/roadmap.md:150` — M3 exit criterion.
- `.moai/project/structure.md:18-22` — `xtwitter`/`bluesky` package
  reservation (consolidation noted in Open Question §11.7).
- `.moai/project/tech.md:107` — X / Twitter row (ScrapeCreators or
  Nitter, per-plan, no official deep search 2026).
- `.moai/project/tech.md:110` — Bluesky row (AT Protocol public feed,
  anonymous, generous).
- `.moai/project/tech.md:147` — ToS feature-flag mandate.
- `.moai/config/sections/quality.yaml` — `development_mode: tdd`,
  `test_coverage_target: 85`.
- `.moai/config/sections/harness.yaml` — auto-routing.
- `.moai/config/sections/language.yaml` — `documentation: en`,
  `code_comments: en`.
- `go.mod` — current module surface; verified no atproto deps;
  `go.uber.org/goleak` already pinned via ADP-001 run-phase.

---

*End of SPEC-ADP-006 v0.1 (DRAFT)*
