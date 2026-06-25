---
id: SPEC-ADP-001b
title: Reddit RSS Adapter (Credential-Free Fallback)
version: 0.1.0
milestone: M2 — Adapter coverage (fallback path)
status: implemented
priority: P1
owner: expert-backend
methodology: tdd
coverage_target: 85
created: 2026-06-25
updated: 2026-06-25
author: limbowl
issue_number: null
depends_on: [SPEC-CORE-001, SPEC-ADP-009, SPEC-CLI-003]
blocks: []
---

# SPEC-ADP-001b: Reddit RSS Adapter (Credential-Free Fallback)

## HISTORY

- 2026-06-25 (v0.1.0): Initial draft. Credential-free Reddit search adapter
  (`Name()="reddit-rss"`) using the public `www.reddit.com/search.rss` endpoint.
  Companion fallback to the OAuth `reddit` adapter (SPEC-ADP-001 / SPEC-ADP-001a),
  which became hard to provision after Reddit gated API-key creation behind the
  2026-06 "Responsible Builder Policy". This adapter is a SEPARATE, always-on,
  non-credentialed source. It does NOT modify or replace the OAuth adapter.

---

## Context

The existing OAuth Reddit adapter (`internal/adapters/reddit/`, `Name()="reddit"`)
requires `REDDIT_CLIENT_ID` / `REDDIT_CLIENT_SECRET` and queries
`oauth.reddit.com/search` via a `client_credentials` token. As of 2026-06 Reddit
gated new API-key creation behind an approval process, so OAuth credentials are
hard to obtain. This SPEC adds a credential-free fallback that needs no API key,
fetching the public global Reddit search RSS feed instead.

The two adapters are independent and coexist: when OAuth credentials are present
both `reddit` and `reddit-rss` may fan out simultaneously; when credentials are
absent only `reddit-rss` participates. The OAuth adapter is out of scope for any
modification here (see Exclusions).

### Design decisions (user-confirmed, pre-locked)

- D1: Separate independent adapter, `Name()="reddit-rss"`, always-on (registers
  unconditionally, no creds). Coexists with the OAuth `reddit` adapter.
- D2: Query mapping uses the global Reddit search RSS endpoint
  `https://www.reddit.com/search.rss?q=<query>&sort=relevance`. The usearch query
  text maps straight into `q`. RSS is parsed into `types.NormalizedDoc`. (v0.1
  emits no `t=` time-window parameter — see Exclusions.)

### Patterns the implementer MUST reuse

- Adapter contract: `pkg/types/adapter.go` — the 4-method interface
  `Name() / Search / Healthcheck / Capabilities`. `Search` and `Healthcheck`
  MUST honour `ctx` cancellation; raw errors MUST be wrapped in
  `*types.SourceError` with the correct `Category`.
- RSS parse pattern: `internal/adapters/koreanews/rss.go` and `search.go` already
  use `github.com/mmcdole/gofeed` with per-feed timeout, per-feed error
  isolation, and `feedItemsToDocs` mapping to `NormalizedDoc`. `reddit-rss` is a
  near-mirror but simpler: a SINGLE feed (the `search.rss` URL), so no
  multi-feed `errgroup` is required — reuse `gofeed` + `ParseURLWithContext` +
  per-call timeout + `SourceError` mapping.
- Registration: `internal/pipeline/pipeline.go`
  `BuildProductionRegistryWithResolverAndError` — non-credentialed adapters
  register via `reg.Register(a)` in the block around the `hn` / `arxiv` /
  `searxng` / `koreanews` entries. `reddit-rss` goes in this non-credentialed
  section and always registers.
- Source visibility: the adapter auto-appears in `usearch sources list/status`
  because that path (`cmd/usearch/sources_cmd.go`) wraps the SAME registry. No
  new wiring is needed for discoverability.

### Error taxonomy reference (exact, from `pkg/types/errors.go`)

The `Category` enum has exactly five values. There is no `CategoryTimeout` or
`CategoryNetwork`. Map as follows:

- HTTP 429 → `CategoryRateLimited` (set `RetryAfter` from the `Retry-After`
  header when present)
- HTTP 403 → `CategoryUnavailable` (retryable). Reddit returns 403 to
  anonymous/unidentified RSS traffic — the exact failure mode this adapter must
  tolerate — so an anon-block is treated as transient/recoverable, NOT a
  permanent failure. Mirrors the 401 carve-out style at
  `internal/adapters/reddit/client.go:118-124`.
- 4xx (other than 429 and 403) → `CategoryPermanent`
- 5xx → `CategoryUnavailable`
- network-layer error (DNS, dial, TLS, connection reset) → `CategoryUnavailable`
- `context.DeadlineExceeded` / timeout / transient parse-retry → `CategoryTransient`
- gofeed parse/format failure on a 2xx body → `CategoryTransient`
- invalid/empty query → `CategoryPermanent`

---

## Requirements (EARS)

### Construction and Options

- REQ-ADP1B-001 (Ubiquitous): The reddit-rss adapter **shall** expose a
  constructor `New(opts Options) (*Adapter, error)` that applies defaults to all
  zero-valued option fields and returns a value satisfying `types.Adapter`.

- REQ-ADP1B-002 (Optional): **Where** `Options.BaseURL` is set (non-empty), the
  adapter **shall** issue search requests against that base instead of the
  production default `https://www.reddit.com/search.rss`, so tests may target a
  loopback `httptest` server.

- REQ-ADP1B-003 (Optional): **Where** `Options.UserAgent` is set (non-empty), the
  adapter **shall** send that value as the `User-Agent` request header; otherwise
  it **shall** send a descriptive default User-Agent
  (`usearch-reddit-rss/<version> (+https://github.com/elymas/universal-search)`),
  mirroring the OAuth adapter's UA template defined in
  `internal/adapters/reddit/reddit.go:23` (existing shape:
  `usearch/%s (+https://github.com/elymas/universal-search)`).

- REQ-ADP1B-004 (Optional): **Where** `Options.Timeout` is set (greater than
  zero), the adapter **shall** apply that value as the per-request timeout;
  otherwise it **shall** apply a default timeout (10 seconds).

### Search mapping

- REQ-ADP1B-005 (Event-Driven): **When** `Search(ctx, q)` is invoked with
  non-empty `q.Text`, the adapter **shall** construct the request URL by
  url-encoding `q.Text` into the `q` parameter of the configured `search.rss`
  base and appending `sort=relevance`, and **shall** emit no other query
  parameters (e.g. `https://www.reddit.com/search.rss?q=<encoded>&sort=relevance`).

- REQ-ADP1B-007 (Event-Driven): **When** a search response is received with a 2xx
  status, the adapter **shall** parse the body with `github.com/mmcdole/gofeed`
  and map each feed item to a `types.NormalizedDoc` with:
  `SourceID="reddit-rss"`, `Title` from the item title (HTML-stripped), `URL`
  from the item link, `Snippet` from the item description/content (HTML-stripped,
  truncated), `Body` from the richest available item text, `PublishedAt` from the
  item published/updated time (UTC, zero if absent), `RetrievedAt` from the
  adapter clock, `Author` from the item author when present, `DocType` =
  `types.DocTypePost`, and a populated `Hash` via `doc.CanonicalHash()`.

- REQ-ADP1B-008 (Ubiquitous): The adapter **shall** assign every emitted
  `NormalizedDoc` the neutral constant `Score` of 0.5 (mirroring the koreanews
  RSS default). v0.1 does not parse any score or engagement signal from the RSS
  body — see Exclusions.

- REQ-ADP1B-009 (Unwanted): **If** an individual feed item is missing a link
  (empty `URL`), **then** the adapter **shall** skip that item rather than emit
  an invalid `NormalizedDoc`.

### Context cancellation

- REQ-ADP1B-010 (Unwanted): **If** `ctx` is already cancelled or its deadline is
  exceeded before or during the search request, **then** the adapter **shall**
  abort the fetch and return a `*types.SourceError` (timeout →
  `CategoryTransient`; other cancellation → `CategoryUnavailable`) without
  blocking.

### Error category mapping

- REQ-ADP1B-011 (Unwanted): **If** the search response status is 429, **then** the
  adapter **shall** return a `*types.SourceError{Category: CategoryRateLimited}`
  populating `RetryAfter` from the `Retry-After` header when present.

- REQ-ADP1B-012 (Unwanted): **If** the search response status is 403, **then** the
  adapter **shall** return a
  `*types.SourceError{Category: CategoryUnavailable, HTTPStatus: 403}` (retryable
  by fanout), because Reddit serves 403 to anonymous/unidentified RSS traffic and
  an anon-block must remain recoverable rather than silently yielding zero
  results. This mirrors the 401 carve-out style at
  `internal/adapters/reddit/client.go:118-124`.

- REQ-ADP1B-012a (Unwanted): **If** the search response status is a 4xx other than
  429 and 403, **then** the adapter **shall** return a
  `*types.SourceError{Category: CategoryPermanent, HTTPStatus: status}`.

- REQ-ADP1B-013 (Unwanted): **If** the search response status is 5xx, **then** the
  adapter **shall** return a
  `*types.SourceError{Category: CategoryUnavailable, HTTPStatus: status}`.

- REQ-ADP1B-014 (Unwanted): **If** a network-layer error occurs (DNS, dial, TLS,
  reset), **then** the adapter **shall** return a
  `*types.SourceError{Category: CategoryUnavailable}` wrapping the cause.

- REQ-ADP1B-015 (Unwanted): **If** the response body is a malformed/unparseable
  feed on an otherwise-2xx response, **then** the adapter **shall** return a
  `*types.SourceError{Category: CategoryTransient}` wrapping the gofeed error.

- REQ-ADP1B-016 (Unwanted): **If** `q.Text` is empty or all-whitespace, **then**
  the adapter **shall** return a `*types.SourceError{Category: CategoryPermanent}`
  without issuing a network request.

### Host constraint (SSRF guard)

- REQ-ADP1B-017 (Ubiquitous): The adapter **shall** constrain the production
  request host to `www.reddit.com` and **shall not** follow redirects to hosts
  outside an allowlist of `{www.reddit.com}`, mirroring the redirect-allowlist
  discipline in `internal/adapters/reddit/client.go` (which constrains to a
  5-host Reddit allowlist). The test `BaseURL` override (REQ-ADP1B-002) is
  exempt so loopback `httptest` hosts can be used.

### Capabilities and Healthcheck

- REQ-ADP1B-018 (Ubiquitous): The adapter **shall** return deterministic
  `Capabilities` with `SourceID="reddit-rss"`, `DisplayName="Reddit (RSS)"`,
  `DocTypes=[DocTypePost]`, `RequiresAuth=false`, `AuthEnvVars=nil`, and Notes
  describing it as the credential-free public-RSS fallback to the OAuth `reddit`
  adapter. `SourceID` MUST equal `Name()`.

- REQ-ADP1B-019 (Event-Driven): **When** `Healthcheck(ctx)` is invoked, the
  adapter **shall** probe a lightweight target on the configured base host (e.g.
  a HEAD/GET against `search.rss` with a trivial query) and return `nil` on
  HTTP 2xx/3xx, or a `*types.SourceError{Category: CategoryUnavailable}` on
  transport failure or 5xx.

### Registry wiring

- REQ-ADP1B-020 (Ubiquitous): The pipeline **shall** register the reddit-rss
  adapter unconditionally in the non-credentialed block of
  `BuildProductionRegistryWithResolverAndError` via `reg.Register(a)`, honouring a
  `REDDIT_RSS_BASE_URL` env override for the base URL (parallel to `HN_BASE_URL`
  / `ARXIV_BASE_URL`), so that `reddit-rss` is always present in the registry
  regardless of credential state.

- REQ-ADP1B-021 (Ubiquitous): The adapter **shall** emit no Prometheus metrics,
  OTel spans, or slog records from within its own package (sole-emitter
  discipline); observability is provided by the registry's `wrappedAdapter`
  layer, consistent with all existing adapters.

---

## Category metadata

- Category: **social** (same as the OAuth `reddit` adapter). `DocTypes` =
  `[types.DocTypePost]`.

---

## Risks (documented)

- R1 — Unidentified RSS traffic may be rate-limited or blocked. Reddit throttles
  anonymous traffic aggressively and serves HTTP 403 to unidentified clients.
  Mitigation: a descriptive custom `User-Agent` is REQUIRED (REQ-ADP1B-003),
  mirroring the OAuth adapter's UA template at
  `internal/adapters/reddit/reddit.go:23`. HTTP 429 maps to `CategoryRateLimited`
  with `RetryAfter` (REQ-ADP1B-011); HTTP 403 (anon-block) maps to
  `CategoryUnavailable` so it stays retryable rather than yielding silent zero
  results (REQ-ADP1B-012).
- R2 — `search.rss` is lower-fidelity than the OAuth Data API: result count is
  capped (~25 items, no pagination) and lacks rich engagement metadata (exact
  upvotes, subreddit facets, NSFW flags). Documented as a known limitation; this
  adapter is a fallback, not a replacement.
- R3 — Host allowlist: production requests and redirects are constrained to
  `www.reddit.com` (REQ-ADP1B-017) to preserve the SSRF guard posture of the
  existing reddit client. The `BaseURL` override is test-only.

---

## Exclusions (What NOT to Build)

- Do **not** modify, replace, deprecate, or share code with the existing OAuth
  `reddit` adapter (`internal/adapters/reddit/`). It keeps `Name()="reddit"` and
  its OAuth flow unchanged. `reddit-rss` is a separate package.
- Do **not** add credential handling, OAuth, `secretstore.Resolver` usage, or any
  `REDDIT_CLIENT_*` env lookups. This adapter is unconditionally credential-free.
- Do **not** implement per-subreddit RSS routing, subreddit listing feeds,
  comment-thread RSS, pagination, or `after`/`before` cursor traversal. Only the
  global `search.rss?q=` endpoint is in scope.
- v0.1 excludes time-window filtering: the adapter emits no `t=` parameter,
  because `pkg/types/query.go` has no `Since` field (only `Filters []Filter` and
  `Deadline`). Revisit if a `Filter.Key` convention for Reddit time windows is
  later standardized.
- v0.1 excludes score/engagement extraction: the adapter does NOT parse upvotes,
  comment counts, or feed extensions from the RSS body. Every doc gets the
  neutral constant `Score` 0.5 (REQ-ADP1B-008). Revisit if `search.rss` is later
  found to carry a reliable engagement signal.
- Do **not** add a multi-feed `errgroup` fan-out — there is exactly one feed URL
  per search.
- Do **not** introduce a new HTTP client abstraction or a new RSS library; reuse
  `net/http` plus `github.com/mmcdole/gofeed` already vendored for koreanews.
- Do **not** add ranking, dedup-across-adapters, or fanout changes — those live
  in SPEC-FAN-001 / the synthesis layer, not in this adapter.
