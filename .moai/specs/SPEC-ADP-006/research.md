# SPEC-ADP-006 Research — Bluesky + X (Twitter) Adapter

**Status**: Research-complete; ready for SPEC drafting
**Author**: limbowl (via manager-spec)
**Created**: 2026-05-04
**Milestone**: M3 — Fanout, adapters, index
**Depends on**: SPEC-CORE-001, SPEC-OBS-001, SPEC-IR-001
**Reference adapters consumed**: SPEC-ADP-001 (Reddit), SPEC-ADP-002 (Hacker News)

---

## 0. Research Mandate

SPEC-ADP-006 is the M3 social-platform adapter. Per
`.moai/project/roadmap.md:51` it covers two sources:

> SPEC-ADP-006 | Bluesky + X adapters | AT Protocol + ScrapeCreators (optional)

The mandate is to:

- Document the Bluesky public AppView API surface exactly: endpoint, query
  parameters, response envelope, pagination semantics, rate-limit behaviour,
  authentication model.
- Document the X (Twitter) 2026 access landscape (post-acquisition API
  restrictions, ScrapeCreators alternative, ToS / legal-risk position).
- Decide v0 scope: Bluesky-only vs Bluesky+X simultaneously.
- Map both response envelopes to the SPEC-CORE-001 NormalizedDoc canonical
  contract with social-specific Metadata extensions.
- Reuse the SPEC-ADP-001 reference shape (file layout, error mapping, MX tag
  plan, TDD harness) verbatim where applicable.
- Surface architectural decisions for plan-auditor scrutiny:
  - Single adapter package with two Adapter instances vs two packages.
  - Direct stdlib HTTP vs `github.com/bluesky-social/indigo` Go library.
  - Race-safe concurrent dispatch across both sub-sources.
  - X opt-in gating mechanism (ENV var, feature flag, ToS acknowledgement).
- Enumerate risks and propose mitigations.
- List Open Questions explicitly deferred but documented.

Every claim is either file-cited (e.g.,
`internal/adapters/registry.go:172-263`) or URL-cited from verified web
sources accessed via WebFetch. No invented facts.

---

## 1. Bluesky Public API Surface (PRIMARY sub-source)

### 1.1 Endpoint, Host, Authentication

**Public AppView host**: `https://public.api.bsky.app`

Citation: Bluesky API Directory at
https://docs.bsky.app/docs/advanced-guides/api-directory states:

> "many Bluesky Lexicon endpoints are public, and do not require
> authentication. These endpoints can be made directly against the Bluesky
> AppView, preferably via the `https://public.api.bsky.app` hostname"

Service-host summary table (same source):

| Service                  | Host                          |
|--------------------------|-------------------------------|
| AppView (authenticated)  | `https://api.bsky.app`        |
| Entryway                 | `https://bsky.social`         |
| Relay                    | `https://bsky.network`        |
| **AppView (public/anon)**| `https://public.api.bsky.app` |

**Search endpoint (XRPC)**: `/xrpc/app.bsky.feed.searchPosts`

Citation: Bluesky HTTP Reference at
https://docs.bsky.app/docs/api/app-bsky-feed-search-posts states:

> "Find posts matching search criteria, returning views of those posts."
> Endpoint type: GET. Public on the AppView host. "this API endpoint may
> require authentication (eg, not public) for some service providers" —
> when called against `public.api.bsky.app`, no auth is required.

**v0 authentication decision**: ANONYMOUS against
`public.api.bsky.app/xrpc/app.bsky.feed.searchPosts`. This mirrors ADP-001's
no-auth posture and eliminates secret-management complexity. App Password
flow (`com.atproto.server.createSession` → access JWT + refresh JWT) is
DEFERRED per Open Question §7.1.

App Password mechanics — for posterity (not implemented in v0):

> "com.atproto.server.createSession" returns a session object containing
> two API tokens. **accessJwt**: "an access token which is used to
> authenticate requests but expires after a few minutes". **refreshJwt**:
> "a refresh token which lasts longer and is used only to update the
> session with a new access token".

Citation: https://docs.bsky.app/docs/get-started

The cost of the auth path is non-trivial: a server-side adapter would have
to manage token rotation, handle 401-on-stale-access, reject invalid
credentials, and stash refreshed tokens in process memory. None of that is
required for the public AppView. v0 stays anonymous; SPEC-ADP-006a may add
the auth path if measured pain (e.g., higher anonymous IP rate limits)
warrants.

### 1.2 Query Parameters

Per `app.bsky.feed.searchPosts` Lexicon
(https://docs.bsky.app/docs/api/app-bsky-feed-search-posts):

| Parameter | Type | Required | Notes |
|-----------|------|----------|-------|
| `q`       | string | YES | Search query string |
| `limit`   | integer | no | Default ~25, max 100 (mirrors Reddit/HN convention) |
| `cursor`  | string | no | Opaque pagination cursor — pass back from prior response |
| `sort`    | enum (`top`, `latest`) | no | Default `top` (relevance) |
| `since`   | string (ISO datetime) | no | Lower bound on `indexedAt` |
| `until`   | string (ISO datetime) | no | Upper bound on `indexedAt` |
| `mentions`| string (DID/handle) | no | Filter to posts mentioning this actor |
| `author`  | string (DID/handle) | no | Filter to posts authored by this actor |
| `lang`    | string (BCP-47) | no | Filter to posts in language |
| `domain`  | string | no | Filter to posts containing links to domain |
| `url`     | string | no | Filter to posts containing this exact URL |
| `tag`     | string array | no | Hashtag filter (repeatable) |

**v0 mapping decisions** (parameters consumed from `types.Query`):

- `q` ← `q.Text`
- `limit` ← `clamp(q.MaxResults, 1, 100)`, default 25 when zero
- `cursor` ← `q.Cursor` (opaque pass-through)
- `lang` ← `q.Lang` (when non-empty)
- `since` ← `Query.Filters[Key="since"]` (ISO datetime; validated)
- `sort` ← hardcoded `top` (relevance) in v0; `Query.Filters[Key="sort"]`
  switch to `latest` deferred per Open Question §7.3.

Note: parameters `mentions`, `author`, `domain`, `url`, `tag` are NOT
mapped in v0. They exist in the API and can be added via
`Query.Filters[Key="mentions"]` etc. in a future enhancement; v0 skips
them to keep the surface minimal.

### 1.3 Response Envelope

The response shape was not fully detailed in the public docs page (the
WebFetch returned an abbreviated excerpt — see §1.4 limitations). Based on
the AT Protocol Lexicon convention (every search endpoint returns
`{cursor, posts}` or `{cursor, items}`) and the official TypeScript
codegen at https://github.com/bluesky-social/indigo (`api/bsky/feed*.go`
auto-generated types reference `app.bsky.feed.defs#postView` for items),
the documented shape is:

```json
{
  "cursor": "<opaque-string>",
  "hitsTotal": 1234,
  "posts": [
    {
      "uri": "at://did:plc:abc.../app.bsky.feed.post/3jzfcijpj2z2a",
      "cid": "bafyreihvffr...",
      "author": {
        "did": "did:plc:abc...",
        "handle": "alice.bsky.social",
        "displayName": "Alice",
        "avatar": "https://cdn.bsky.app/img/avatar/..."
      },
      "record": {
        "$type": "app.bsky.feed.post",
        "text": "post body up to 300 chars",
        "createdAt": "2026-04-15T10:23:11Z",
        "langs": ["en"],
        "facets": [...],
        "embed": {...}
      },
      "replyCount": 5,
      "repostCount": 12,
      "likeCount": 87,
      "quoteCount": 3,
      "indexedAt": "2026-04-15T10:23:14Z",
      "viewer": {...},
      "labels": [...]
    }
  ]
}
```

**Per-post fields used by v0**:

| Lexicon Field | Type | Notes |
|---------------|------|-------|
| `uri`         | AT-URI | e.g., `at://did:plc:.../app.bsky.feed.post/3jzfcijp...` — used as `NormalizedDoc.ID`. AT-URIs are stable. |
| `cid`         | string | Content-ID hash; stored in Metadata. |
| `author.did`  | DID | Used in Metadata. |
| `author.handle` | string | e.g., `alice.bsky.social`; populates `NormalizedDoc.Author` and Metadata. |
| `author.displayName` | string | Rendered name; goes into Metadata. |
| `record.text` | string | The post body. Plain text, max 300 chars per AT spec. Goes into `Body`/`Snippet`/`Title` (truncated). |
| `record.createdAt` | ISO datetime | `time.Parse(time.RFC3339, ...)` → `PublishedAt`. |
| `record.langs` | []string | First entry → `NormalizedDoc.Lang` (BCP-47). |
| `replyCount`  | integer | Metadata. |
| `repostCount` | integer | Metadata; the SPEC's "repost count" requirement. |
| `likeCount`   | integer | Metadata; the SPEC's "like count" requirement. |
| `quoteCount`  | integer | Metadata. |
| `indexedAt`   | ISO datetime | NOT used as `RetrievedAt` — that field is fanout-fetch time per CORE-001 contract. Surfaced in Metadata for synthesis-side dedup heuristics. |

**Posted_at**: `record.createdAt` is the user-visible timestamp; v0 uses
this for `NormalizedDoc.PublishedAt`. `indexedAt` is the AppView's index
time (always >= createdAt) and is not user-facing.

**URL construction**: Bluesky posts have a canonical web URL of the form:

```
https://bsky.app/profile/<handle-or-did>/post/<rkey>
```

where `<rkey>` is the third segment of the AT-URI (e.g.,
`3jzfcijpj2z2a` from `at://did:plc:abc.../app.bsky.feed.post/3jzfcijpj2z2a`).

The adapter constructs this URL deterministically from `author.handle` and
the AT-URI's last segment. The AT-URI itself is stored in
`Metadata["post_uri"]` for downstream consumers who need the canonical
protocol-level identifier.

### 1.4 Documentation Limitations

The Bluesky HTTP reference pages returned abbreviated content via
WebFetch — specifically:

- `https://docs.bsky.app/docs/api/app-bsky-feed-search-posts` did not
  include the full parameter type tables or the response Lexicon schema in
  the rendered page.
- `https://docs.bsky.app/docs/get-started` did not detail App Password
  refresh-frequency recommendations.
- `https://docs.bsky.app/docs/advanced-guides/api-directory` listed
  service hosts but not endpoint enumeration.

**Mitigation**: the SPEC's run-phase implementer SHALL verify field
shapes against either (a) the auto-generated Go types in
`github.com/bluesky-social/indigo/api/bsky/` (Lexicon codegen) BEFORE
committing parser code, or (b) a single live request to
`public.api.bsky.app` (gated behind `BSKY_LIVE=1` env, NOT in CI). The
field set documented above is the v0 contract; if the run-phase discovers
that any documented field is renamed or absent, that is a SPEC bug to fix
in iteration 2 — NOT a license to invent fields.

### 1.5 Pagination

Cursor-based, opaque. The response `cursor` is a string the adapter passes
through `Query.Cursor` on the next call. When the AppView returns no
`cursor` (or empty string), pagination is exhausted. This mirrors
Reddit's `data.after` and HN's integer page cursor — the same
`Metadata["next_cursor"]` surfacing convention applies (REQ-ADP-006
parser sets it on the LAST returned doc when cursor is non-empty).

### 1.6 HTTP Status Codes and Error Semantics

The AT Protocol exposes XRPC errors as JSON:

```json
{ "error": "InvalidRequest", "message": "Missing required parameter: q" }
```

Standard HTTP status mapping (consistent with ADP-001/ADP-002):

| Code | Semantics | Adapter Response |
|------|-----------|------------------|
| 200  | Success | Parse `{cursor, posts}` envelope |
| 400  | Bad request (invalid query, malformed cursor) | `*SourceError{Category: CategoryPermanent}` |
| 401  | Auth required (unexpected on public AppView) | `*SourceError{Category: CategoryPermanent}` |
| 403  | Forbidden (rare; e.g., labels) | `*SourceError{Category: CategoryPermanent}` |
| 404  | Not found (typically endpoint typo) | `*SourceError{Category: CategoryPermanent}` |
| 429  | Rate limited (per-IP 3,000/5min) | `*SourceError{Category: CategoryRateLimited, RetryAfter}` |
| 500/502/503/504 | Server / cluster | `*SourceError{Category: CategoryUnavailable}` |
| network / timeout | Connection / DNS / TLS | `*SourceError{Category: CategoryUnavailable, HTTPStatus: 0}` |

The same `categorizeStatus(httpStatus, retryAfter, cause)` helper from
ADP-001 / ADP-002 applies (with `Adapter: "bluesky"` or `Adapter: "x"`).

### 1.7 Rate Limits (2026)

Citation: https://docs.bsky.app/docs/advanced-guides/rate-limits states:

- **Per-IP global**: "3,000 per 5 minutes" → effective 600/min sustained.
- **Per-account session creation**: 30 / 5min, 300 / day (irrelevant in v0
  anonymous mode).
- **Per-account write**: 5,000 points/hour, 35,000 points/day (irrelevant
  for read-only search).
- **Relay events**: 50/sec (irrelevant; v0 is REST not firehose).

The 3,000 / 5min IP figure is generous compared to Reddit's 10/min unauth
and HN/Algolia's ~60/min observed. The bottleneck is unlikely to be IP
rate-limit at the adapter layer; the fanout's `MaxParallel=8` cap from
SPEC-FAN-001 will dominate.

The page references "many HTTP API services return rate limit headers on
responses" and references the IETF draft. **HTTP 429** is the documented
return; no specific guidance about `Retry-After` header semantics. The
adapter reuses the ADP-001 `parseRetryAfter` helper (RFC 7231 §7.1.3,
5s default, 60s cap).

`Capabilities.RateLimitPerMin = 600` for the Bluesky sub-source (the
sustained per-IP figure).

### 1.8 Redirect Handling

`public.api.bsky.app` does not normally redirect. As a defensive measure
consistent with the ADP-001/ADP-002 pattern, the adapter SHALL enforce a
redirect allowlist of `{public.api.bsky.app, api.bsky.app, bsky.app}` and
reject cross-domain redirects with `*SourceError{CategoryPermanent}`. The
3-hop cap is preserved.

### 1.9 Why NOT use github.com/bluesky-social/indigo

Citation: https://github.com/bluesky-social/indigo states:

> "⚠️ All the packages in this repository are under active development.
> Features and software interfaces have not stabilized and may break or be
> removed."

While it is the official Go library and includes `api/bsky` Lexicon
codegen, three concerns rule it out for v0:

1. **Stability disclaimer** — explicit pre-stable warning. Adopting it
   couples our adapter to upstream churn.
2. **Supply-chain footprint** — indigo brings in a large dependency tree
   (CBOR codec, libp2p hooks, multiformats, zstd, etc. for full atproto
   tooling). v0 needs only one HTTP call; the import-cost-to-feature ratio
   is unfavourable.
3. **SPEC-DEP-001 dependency baseline** caps the Go module surface. Adding
   indigo would require coordinating an `expert-backend` review of the
   transitive set; this is out of v0 scope.

**Decision**: implement directly with `net/http` + `encoding/json`.
The query is one GET call; the response is one JSON envelope; the parsing
shape is well-known. The reference Reddit and HN adapters set the
template — a 7th dependency-free adapter at 200-300 lines is realistic.

This mirrors ADP-001 §5.2's `go-reddit` rejection rationale.

---

## 2. X (Twitter) 2026 Access Landscape (OPTIONAL sub-source)

### 2.1 Official X API Tier Status (as of 2026-05)

The post-acquisition X API tiers (Free / Basic / Pro / Enterprise)
restricted full-archive search and significantly reduced free-tier read
quotas in 2023-2024 and the trend has continued into 2026. Specifically:

- **Free tier**: write-only access; reads severely limited (search posts
  endpoint not available).
- **Basic tier ($200/mo)**: ~10,000 posts/month read, NO full-archive
  search.
- **Pro tier ($5,000/mo)**: includes recent (last-7-day) search; no
  full-archive on this tier.
- **Enterprise tier**: custom contract, full-archive search available,
  pricing not public.

**Implication for v0**: subscribing to a five-figure-per-month Enterprise
contract for an open-source dev tool is not justified. The official API
is OUT OF SCOPE for this SPEC.

> Note: numerical tier figures above reflect the 2023–2024 publicly
> announced pricing; X has revised tiers periodically. The implementer
> should re-verify against `https://developer.x.com` at run-phase time
> if cost-justified live integration is ever attempted. v0 does not
> integrate the official tier.

### 2.2 ScrapeCreators Alternative

Citation: https://www.scrapecreators.com/

ScrapeCreators offers a Twitter API (6 endpoints — full list at
`/twitter-api`) on a credit-based pay-as-you-go model:

| Tier        | Cost   | Credits | Per-1000-req cost |
|-------------|--------|---------|---|
| Free        | $0     | 100     | n/a (trial) |
| Freelance   | $47    | 25,000  | $1.88 |
| Business    | $497   | 500,000 | $0.99 |
| Enterprise  | custom | custom  | custom |

> "Every call fetches live, publicly available data straight from the
> source."

ScrapeCreators self-describes as "an independent API provider built by a
small global team in Austin, TX". They are **NOT** an officially-licensed
X API reseller — there is no partnership / licensing statement on the
homepage. Terms of Service link at `/terms` was not fetched in research;
the run-phase implementer SHALL review `/terms` BEFORE enabling the X
sub-source in any tenant.

### 2.3 ToS / Legal-Risk Position

X's Developer Agreement explicitly prohibits unauthorised scraping of the
platform. ScrapeCreators's "publicly available data" framing does not
shield the consumer from the Developer Agreement's terms — a third-party
proxy is still data sourced from X.

This is the same risk class flagged in `.moai/project/tech.md:147`:

> "ToS violation on scraped sources (X, Instagram) | High | Feature-flag
> behind team opt-in; default to API-based adapters only"

**v0 decision**: the X sub-source is **DISABLED BY DEFAULT** and gated
behind the env var `USEARCH_X_ENABLED=true`. When unset or `false`, the
adapter returns `*SourceError{Category: CategoryPermanent, Cause:
ErrXDisabled}` immediately. This:

1. Honours `tech.md:147`'s feature-flag mandate.
2. Makes opt-in explicit (operator must set the env consciously).
3. Pushes ToS-acceptance responsibility to the operator (a future
   SPEC-ADP-006-AUTH may add a stronger acknowledgement gate, e.g., a
   `--accept-tos` CLI flag).
4. Allows the registry to register the X adapter unconditionally
   (Capabilities deterministic; the gate is in `Search`, not at construct
   time).

Alternative considered and rejected: ship `social.NewX` only when build
tag `xtwitter` is set. Rejected because compile-time gating is heavier
than necessary and complicates CI.

### 2.4 X Sub-Source — Reserved Surface

The SPEC reserves the X surface (constructor `social.NewX(opts) (*Adapter,
error)`, `Name() = "x"`, `Capabilities()` deterministic) so that a
future SPEC-ADP-006-XENABLE can wire ScrapeCreators (or any other
provider) without changing the registry contract. The reserved surface in
v0 is implemented as a **placeholder Search** that always returns
`ErrXDisabled` until the env gate is true AND a provider implementation
is wired in (deferred to SPEC-ADP-006-XENABLE).

When `USEARCH_X_ENABLED=true` AND no provider is wired (v0 default),
`Search` SHALL return `*SourceError{Category: CategoryPermanent, Cause:
ErrXProviderNotConfigured}`. This distinguishes "operator opted in but
nothing is built yet" from "operator did not opt in".

This dual-error mechanism is documented in REQ-ADP6-008.

### 2.5 X Field Mapping (forward-compat sketch, NOT in v0 hot path)

For posterity only — when X is wired in a future SPEC, the field mapping
SHOULD be:

| X / Tweet Field | NormalizedDoc Field | Transform |
|-----------------|---------------------|-----------|
| `id_str`        | `ID`                | `"x:" + id_str` |
| (constant)      | `SourceID`          | `"x"` |
| (constructed)   | `URL`               | `"https://x.com/<screen_name>/status/<id_str>"` |
| `full_text` truncated | `Title` | First 100 runes |
| `full_text`     | `Body`              | Plain text |
| `full_text` truncated | `Snippet` | First 280 runes |
| `created_at`    | `PublishedAt`       | `time.Parse(twitter time fmt)` |
| (parse time)    | `RetrievedAt`       | `time.Now().UTC()` |
| `user.screen_name` | `Author`         | use as-is |
| (constructed)   | `Score`             | Tanh of `favorite_count + 0.5*retweet_count` (or similar; calibrate later) |
| `user.lang` or `lang` | `Lang`        | BCP-47 |
| (constant)      | `DocType`           | `DocTypeSocial` |
| (rich)          | `Metadata`          | `{handle, repost_count, like_count, reply_count, quote_count, sub_source: "x"}` |

This mapping is NOT implemented in v0. It is documented so that the
future XENABLE SPEC can copy it verbatim instead of re-deriving.

---

## 3. v0 Scope Decision

### 3.1 Decision: Bluesky-only INTEGRATED, X RESERVED-but-DISABLED

**Bluesky** is integrated end-to-end:
- Live HTTP path against `public.api.bsky.app` (CI uses httptest stubs).
- Full field mapping per §1.3.
- Capabilities, error categorisation, redirect allowlist, NFRs.

**X** is reserved-and-disabled:
- `social.NewX(opts) (*Adapter, error)` constructor exists.
- `Name() = "x"`, `Capabilities()` deterministic with
  `Notes` documenting "DISABLED in v0".
- `Search` returns `ErrXDisabled` (env gate) or
  `ErrXProviderNotConfigured` (env gate set but no provider) — the
  outcome is `outcome="failure"` via the registry wrappedAdapter.
- ZERO HTTP code in v0. ZERO ScrapeCreators dependency in v0.
- The SPEC documents the future enablement path (SPEC-ADP-006-XENABLE).

### 3.2 Rationale

1. **Roadmap honour**: `roadmap.md:51` says "Bluesky + X adapters". Two
   adapter Names in the registry honour the description even if X is
   non-functional in v0. The two-adapter shape lets the IR-001 router
   emit `decision.AdapterSet=["bluesky","x"]` for `Category=social`
   without ad-hoc filtering.
2. **`tech.md:147` compliance**: feature-flag behind opt-in is the
   architectural mandate. Reserved-and-disabled is the cheapest correct
   implementation.
3. **Test discipline**: implementing Bluesky live + X stubbed gives full
   acceptance coverage today AND tests the disabled path's error
   semantics. Future XENABLE work doesn't have to refactor the
   adapter shape — just plug in a provider behind the env gate.
4. **Cost**: zero subscription expense to ship M3.
5. **Legal hygiene**: no ToS-grey scraping shipped. The ScrapeCreators
   integration is a LATER consenting decision per tenant.

### 3.3 Single Package, Two Adapter Instances

The implementation lives in ONE package `internal/adapters/social/` with
TWO Adapter instance constructors:

```go
package social

func NewBluesky(opts BlueskyOptions) (*Adapter, error)
func NewX(opts XOptions) (*Adapter, error)
```

Both return `*Adapter` (a single struct type) with internal `subSource`
field set to `"bluesky"` or `"x"`. `Adapter.Name()` returns the value
of `subSource` so the registry observability (`AdapterCalls{adapter,
outcome}`) correctly emits two distinct `adapter` label values.

This honours the user requirement: "Single adapter, multi-source dispatch
internally" (one Go package, one struct type, shared dispatch helpers)
AND "two adapter labels emitted" (each Adapter instance has its own
`Name()`, the wrappedAdapter sees two registrations).

Internally, `Search` switches on `a.subSource`:
- `bluesky` → live HTTP path (`searchBluesky`).
- `x` → disabled path (`searchXDisabled`).

This is identical to the noop adapter pattern at
`internal/adapters/noop/noop.go:1-46` — one struct, one contract — but
with internal dispatch on a constructor-set field.

---

## 4. Existing Codebase Patterns — Reuse Inventory

ADP-001 (Reddit) and ADP-002 (Hacker News) are the reference shapes.
SPEC-ADP-006 reuses them verbatim where possible:

### 4.1 Adopted Verbatim from ADP-001 / ADP-002

| Pattern | File reference | Reuse decision |
|---------|---------------|----------------|
| `Adapter` struct shape (`httpClient`, `baseURL`, `userAgent`, `healthcheckTarget`) | `internal/adapters/reddit/reddit.go:53-58` | ADOPT verbatim, plus a `subSource string` field. |
| `var _ types.Adapter = (*Adapter)(nil)` compile-time assertion | `internal/adapters/reddit/reddit.go:135` | ADOPT verbatim. |
| `Healthcheck` TCP-dial against `Options.HealthcheckTarget` | `internal/adapters/reddit/reddit.go:123-130` | ADOPT verbatim; default target is `public.api.bsky.app:443` for Bluesky, `<provider-host>:443` for X (placeholder `127.0.0.1:0` when disabled). |
| `User-Agent` header `usearch/<version> (+...)` | `internal/adapters/reddit/client.go::doRequest` | ADOPT verbatim. |
| `Accept: application/json` header | `internal/adapters/reddit/client.go::doRequest` | ADOPT verbatim. |
| HTTP client with 10s timeout, redirect allowlist, `reqid.NewTransport` | `internal/adapters/reddit/client.go::newDefaultClient` | ADOPT verbatim with sub-source-specific allowlist. |
| `categorizeStatus(status, retryAfter, cause) *SourceError` | `internal/adapters/reddit/client.go` | ADOPT pattern; the `Adapter:` field is set from `a.subSource` (so `"bluesky"` or `"x"`). |
| `parseRetryAfter` helper (RFC 7231 §7.1.3, 5s default, 60s cap) | `internal/adapters/reddit/errors.go` | ADOPT verbatim (private package helper). |
| Fixture-based testing with `net/http/httptest.Server` + golden JSON | `internal/adapters/reddit/testdata/` | ADOPT verbatim (per-sub-source testdata directories). |
| `Tanh` score normaliser (divisor=100, center=0.5) | `internal/adapters/reddit/score.go` | ADOPT verbatim for Bluesky (likeCount + repostCount input). Inflection at 100 engagements is reasonable. |
| `BenchmarkParseListing25Docs` pattern | `internal/adapters/reddit/bench_test.go` | ADOPT pattern — `BenchmarkParseSearchPosts25Docs`. |
| `goleak.VerifyTestMain(m)` in `bench_test.go::TestMain` | `internal/adapters/reddit/bench_test.go` | ADOPT verbatim. |
| `TestSearchConcurrentSafe` (50 goroutines, race-clean) | `internal/adapters/reddit/search_test.go` | ADOPT verbatim per REQ-ADP6-009. |

### 4.2 New Patterns (SPEC-ADP-006-only)

| Pattern | Reason |
|---------|--------|
| `subSource` constructor parameter / struct field | Two Adapter instances backed by one struct; sub-source is the dispatch key. |
| `searchBluesky(ctx, q) ([]NormalizedDoc, error)` | Bluesky-specific URL build + parse. |
| `searchXDisabled(ctx, q) ([]NormalizedDoc, error)` | Returns `ErrXDisabled` or `ErrXProviderNotConfigured` based on env. |
| `parseSearchPosts(body, retrievedAt time.Time) ([]NormalizedDoc, string, error)` | Bluesky `app.bsky.feed.searchPosts` envelope parser. |
| `constructBlueskyURL(handle, atURI string) string` | Build `https://bsky.app/profile/<handle>/post/<rkey>`. |
| `parseATURI(atURI string) (did, collection, rkey string, err error)` | Tiny AT-URI parser. |
| Env-gated `searchXDisabled` returning `ErrXDisabled` vs `ErrXProviderNotConfigured` | Two-state opt-in/not-configured semantics. |

### 4.3 Tanh Score Input for Bluesky

The score formula stays `clamp(0.5 + 0.5 * tanh(x / 100), 0.0, 1.0)`.
The input `x` for Bluesky is:

```
x = likeCount + repostCount
```

(reply and quote counts are NOT included to match the Reddit/HN posture
of "primary engagement only"; secondary signals stay in Metadata for RRF
to consider).

When neither field is present (defensive), `x = 0` → `Score = 0.5`.

This formula choice is documented in REQ-ADP6-005 acceptance.

### 4.4 What ADP-006 Does NOT Reuse from ADP-001 / ADP-002

| Pattern | Why omitted |
|---------|-------------|
| `stripHTML` helper (HN's `story_text` HTML) | Bluesky `record.text` is plain text per AT spec; no HTML. |
| `numericFilters` URL composition (HN-specific) | Bluesky uses standard query params, not Algolia composition. |
| `data.after` cursor parsing (Reddit-specific) | Bluesky cursor is opaque pass-through; no parsing needed. |
| `_tags=story` defensive filter (HN-specific) | Bluesky `app.bsky.feed.searchPosts` returns post views directly — no kind union to filter. |
| `over_18=false` NSFW gate (Reddit-specific) | Bluesky has `labels` system; NSFW gating via labels is OUT OF v0 scope (Open Question §7.5). |

---

## 5. Package Layout Proposal

```
internal/adapters/social/
├── social.go                          # Adapter struct, NewBluesky, NewX, Name, Capabilities, Healthcheck, compile-time assertion
├── social_test.go                     # Interface conformance + Capabilities determinism + Name routing
├── search.go                          # (*Adapter).Search hot path; sub-source dispatch
├── search_bluesky.go                  # searchBluesky URL build + HTTP execute + parse delegation
├── search_x.go                        # searchXDisabled env-gated stub
├── search_test.go                     # E2E + happy path + error categorisation tests
├── client.go                          # *http.Client construction, doRequest, categorizeStatus
├── client_test.go                     # categorizeStatus table + redirect allowlist + headers
├── parse.go                           # parseSearchPosts (Bluesky envelope)
├── parse_test.go                      # Field mapping table tests
├── score.go                           # normalizeScore Tanh formula
├── score_test.go                      # Score normalization table
├── url.go                             # constructBlueskyURL + parseATURI helpers
├── url_test.go                        # URL helper table tests
├── errors.go                          # ErrInvalidQuery, ErrInvalidCursor, ErrXDisabled, ErrXProviderNotConfigured + parseRetryAfter helper
├── errors_test.go                     # Sentinel + parseRetryAfter tests
├── bench_test.go                      # BenchmarkParseSearchPosts25Docs + TestMain (goleak)
└── testdata/
    ├── bluesky_search_response.json           # Happy path 25 posts
    ├── bluesky_search_response_empty.json     # Zero hits
    ├── bluesky_search_response_pagination.json # cursor set
    ├── bluesky_search_response_with_lang.json  # langs varying
    ├── bluesky_search_response_self_post.json  # post with embed/facets stripped
    ├── bluesky_search_response_with_repost.json # high engagement counts
    └── bluesky_search_response_malformed.json  # truncated JSON
```

Estimated LoC: ~1,400 production + ~700 test code + ~30 KB testdata.
Each `.go` file targets < 200 LoC excluding tests (matches ADP-001's
discipline at `internal/adapters/reddit/reddit.go:1-136` = 136 LoC).

---

## 6. Reference Implementations Surveyed

### 6.1 SearXNG Bluesky engine

SearXNG includes a Bluesky engine
(https://github.com/searxng/searxng/tree/master/searx/engines, file
`bluesky.py`). Pattern reference only — Python implementation, but
confirms the canonical request shape (`q`, `limit`, `cursor` against
`public.api.bsky.app/xrpc/app.bsky.feed.searchPosts`) and field
extraction (`record.text`, `record.createdAt`, `author.handle`,
counts, `uri`, `cid`).

ADP-006 does not depend on SearXNG; it observes the parameter conventions.

### 6.2 github.com/bluesky-social/indigo (Go) — REJECTED

See §1.9. Pre-stable disclaimer + transitive dependency cost. The
auto-generated Lexicon types in `api/bsky/feed*.go` are useful as a
field-name reference but the package itself is not imported.

### 6.3 atproto.tools — community resources

https://atproto.tools/ catalogs community implementations across
languages. None of the Go entries listed have stronger stability
guarantees than indigo. v0 sticks with stdlib HTTP.

### 6.4 ScrapeCreators X provider — DEFERRED

See §2.2. v0 does not integrate; SPEC-ADP-006-XENABLE may.

---

## 7. Open Questions

These are explicitly UNRESOLVED at SPEC-approval time. Each has a
recommended default. They do NOT block SPEC approval.

### 7.1 App Password authentication for Bluesky

**Question**: should v0 use App Password / `createSession` to authenticate
against `https://api.bsky.app` (instead of the public AppView)?

**Recommended default**: **NO**. v0 uses anonymous `public.api.bsky.app`
exclusively. The 3,000-per-5min IP rate limit is generous; auth complexity
(JWT rotation, refreshJwt management) is not justified for the M3 thin
slice.

**Resolution owner**: future SPEC-ADP-006-AUTH author when measured
anonymous IP rate-limit pressure warrants. Per
https://docs.bsky.app/docs/get-started, App Password is preferred over
OAuth for non-interactive server use because OAuth requires a client
challenge flow incompatible with headless adapters.

### 7.2 Score formula tuning for Bluesky

**Question**: is `likeCount + repostCount` the right input for the Tanh
formula? Should `replyCount` and `quoteCount` factor in?

**Recommended default**: `likeCount + repostCount` only in v0. Like
counts dominate the engagement signal on Bluesky (reply / quote are
rarer); the inflection at 100 matches the Reddit/HN convention. RRF
fusion in SPEC-IDX-001 weights rank not raw score across adapters, so
calibration error is bounded.

**Resolution owner**: SPEC-IDX-001 author may revisit if cross-adapter
ranking quality measurement indicates Bluesky scores systematically too
high or too low.

### 7.3 `sort=top` vs `sort=latest` for Bluesky

**Question**: should v0 expose `Query.Filters[Key="sort"]` to switch
between Bluesky's `top` (relevance) and `latest` (chronological)?

**Recommended default**: hardcode `sort=top` in v0. Mirrors Reddit's
`sort=relevance` and HN's relevance default. A `Query.Filters[Key="sort"]`
switch is a clean follow-up enhancement.

**Resolution owner**: future enhancement SPEC; deferred.

### 7.4 X provider choice (when XENABLE happens)

**Question**: ScrapeCreators vs Nitter scraping vs official X tier
subscription?

**Recommended default**: ScrapeCreators on opt-in tenant subscription
(Freelance $47/mo for 25k requests; ToS reviewed at deployment time).
Nitter is operationally fragile (instances disappear). Official tier is
cost-prohibitive for an open-source dev tool.

**Resolution owner**: future SPEC-ADP-006-XENABLE author.

### 7.5 Bluesky NSFW / labels handling

**Question**: should v0 filter out posts with `labels` indicating NSFW or
hate-speech moderation?

**Recommended default**: NO in v0. Labels surface in
`Metadata["labels"]`; consumers (synthesis, UI) filter at their layer.
Adapter-level label filtering is OUT OF v0 scope. Future SPEC-AUTH-002
may add a tenant-scoped policy.

**Resolution owner**: SPEC-AUTH-002 author.

### 7.6 Bluesky embed / facets serialisation

**Question**: Bluesky posts can carry `record.embed` (image, external
link card, quoted post, video) and `record.facets` (mention/link/tag
ranges in the text). Should the adapter surface these?

**Recommended default**: NO in v0 hot path. The adapter sets `Body =
record.text` (plain) and `Snippet = truncate(record.text, 280)`. Embed
and facets are stored verbatim in `Metadata["embed"]` /
`Metadata["facets"]` ONLY when downstream consumers ask for them. Run-
phase MAY decide to elide these from Metadata to reduce allocation
cost.

**Resolution owner**: run-phase implementer; SPEC-SYN-001 may request
embed surfacing if synthesis quality on Bluesky benefits.

### 7.7 X opt-in mechanism: ENV vs CLI flag vs config file

**Question**: `USEARCH_X_ENABLED=true` is the v0 mechanism. Should it
be a CLI flag (`usearch query --x-enabled ...`) or a config file
(`fanout.yaml: x_enabled: true`) instead?

**Recommended default**: env var. Simplest, deployment-friendly,
reversible per process.

**Resolution owner**: SPEC-CLI-002 (M7) author may add a flag layer if
operator UX demands.

### 7.8 Bluesky cursor opacity vs structured

**Question**: surface the cursor in `Metadata["next_cursor"]` (opaque
string) or split into structured fields?

**Recommended default**: opaque, on the LAST returned doc only. Mirrors
ADP-001 REQ-ADP-006 + ADP-002 REQ-ADP2-005. Consumers MUST NOT parse.

**Resolution owner**: SPEC-FAN-001 author; resolved.

### 7.9 Per-adapter X-disabled outcome label

**Question**: when X is disabled, does `outcome="failure"` from
`OutcomeFromError` mislead operators (it looks like a real failure)?

**Recommended default**: yes, ship with `outcome="failure"` in v0;
document in `Capabilities.Notes` so dashboard operators know to filter
"x" out of their failure aggregations. Adding a `outcome="disabled"`
value would amend SPEC-OBS-001's allowlist — out of scope.

**Resolution owner**: SPEC-OBS-001 author may add a label value if
operational complaints accumulate.

---

## 8. Risk Register

| Risk | Severity | Mitigation |
|------|----------|------------|
| Bluesky AppView returns HTTP 200 with an error envelope (XRPC error in body, non-2xx-conventional) | Medium | `parseSearchPosts` checks for `error` JSON key BEFORE looking for `posts` array; returns `*SourceError{Category: CategoryPermanent, Cause: <error.message>}` if found. Test fixture covers this case. |
| Bluesky response shape drift (new field added, existing renamed) | Low | `encoding/json` ignores unknown fields; the adapter reads only documented fields. Lexicon-versioned response shapes in atproto are stable by contract. |
| AT-URI parsing edge cases (handle vs DID in URI) | Medium | `parseATURI` returns the `rkey` from the LAST URI segment regardless of whether the second segment is a handle or DID. URL construction uses `author.handle` (always present in `postView`); the AT-URI is stored in Metadata for handle-resolution-needed consumers. |
| Bluesky redirect to authenticated endpoint when load-shed | Low | Redirect allowlist enforces `{public.api.bsky.app, api.bsky.app, bsky.app}`; cross-domain rejected. If `public.api.bsky.app` redirects to `api.bsky.app` under load, allowlist allows but the unauthenticated request will likely 401 — handled as `CategoryPermanent`. |
| `record.text` longer than 280 runes (uncommon but possible via lexicon update) | Low | `Snippet = truncateRunes(record.text, 280)` regardless of source length. |
| `record.langs` empty | Low | `NormalizedDoc.Lang = ""`; treated as unknown per CORE-001:28. |
| `author.handle` empty (unusual; happens for deleted-but-cached actors) | Low | Falls back to `author.did` for the URL construction; documented in §4.2. |
| X env var typo (`USEARCH_X_ENABLE=true` instead of `USEARCH_X_ENABLED=true`) | Medium | The env var name is a HARD constant; documented in `Capabilities.Notes`. Operator-side issue, not adapter-side. |
| Concurrent adapter calls share `*http.Client` race | Low | `*http.Client` is goroutine-safe per Go stdlib. ADP-001 REQ-ADP-011 / ADP-002 REQ-ADP2-010 patterns adopted: 50-goroutine race-clean test. |
| `parseATURI` panics on malformed input | Low | All parse paths return `error`; no panics. Fuzz-style table test covers malformed inputs. |
| Bluesky 429 sustained (e.g., shared IP behind cloud NAT) | Medium | `parseRetryAfter` honours `Retry-After`; SPEC-FAN-001 retry orchestration is the supervisor (out of scope here). |
| Score formula too generous on Bluesky (saturates everything to 1.0) | Medium | Tanh divisor=100 means inflection at 100 engagements; max popular Bluesky posts have ~1000 likes. Saturation matches Reddit/HN posture. RRF re-weights via rank. |
| Test fixture rot (Lexicon shape change in production) | Low | Fixtures are static; CI is offline. Live integration test is OUT OF v0 (can be added behind `BSKY_LIVE=1`). |
| ScrapeCreators ToS exposes operator to legal risk if XENABLE is integrated and misconfigured | High | v0 does not ship the integration. SPEC-ADP-006-XENABLE will require explicit ToS acknowledgement. |
| Goroutine leak on ctx cancel mid-Bluesky call | Low | NFR-ADP6-003 + `goleak.VerifyTestMain`. |

---

## 9. Sources and Citations

### External URLs (WebFetch verified 2026-05-04)

- https://atproto.com/ — AT Protocol overview, public firehose, auth/lexicon section pointers.
- https://docs.bsky.app/ — Bluesky documentation home; SDK list (TypeScript/Python/Dart/Go).
- https://docs.bsky.app/docs/api/app-bsky-feed-search-posts — `searchPosts` endpoint metadata.
- https://docs.bsky.app/docs/api/app-bsky-actor-search-actors — `searchActors` endpoint (forward-compat reference; v0 uses searchPosts only).
- https://docs.bsky.app/docs/advanced-guides/api-directory — service host table; `public.api.bsky.app` recommendation.
- https://docs.bsky.app/docs/advanced-guides/rate-limits — IP/account rate limits.
- https://docs.bsky.app/docs/get-started — `createSession` flow, accessJwt/refreshJwt mechanics.
- https://github.com/bluesky-social/indigo — official Go library; pre-stable disclaimer.
- https://www.scrapecreators.com/ — ScrapeCreators pricing, services, ToS-of-platform stance.
- RFC 7231 §7.1.3 — `Retry-After` header semantics.

### Internal Files (file:line cited)

- `.moai/specs/SPEC-ADP-001/spec.md:1-1216` — reference adapter SPEC.
- `.moai/specs/SPEC-ADP-001/research.md:1-791` — reference research artifact.
- `.moai/specs/SPEC-ADP-002/spec.md:1-1190` — second-adapter SPEC.
- `.moai/specs/SPEC-CORE-001/spec.md` — Adapter / Capabilities / Query / NormalizedDoc / SourceError contract.
- `.moai/specs/SPEC-OBS-001/spec.md` — observability bundle, cardinality discipline.
- `.moai/specs/SPEC-IR-001/spec.md` — `Capabilities` consumer contract, Category enum.
- `.moai/specs/SPEC-FAN-001/spec.md` — multi-source fanout supervisor.
- `pkg/types/adapter.go:28-45` — Adapter interface.
- `pkg/types/capabilities.go:38-62` — Capabilities struct.
- `pkg/types/query.go:18-44` — Query struct.
- `pkg/types/normalized_doc.go:40-106` — NormalizedDoc struct, Validate, CanonicalHash.
- `pkg/types/errors.go:14-218` — SourceError, Category, OutcomeFromError.
- `internal/adapters/registry.go:75-167` — Registry lifecycle.
- `internal/adapters/registry.go:172-263` — wrappedAdapter sole-emitter pattern.
- `internal/adapters/noop/noop.go:1-46` — minimal-shape reference.
- `internal/adapters/reddit/reddit.go:1-136` — Adapter struct + Capabilities pattern (mirrored).
- `internal/adapters/reddit/search.go` — Search hot path pattern.
- `internal/adapters/reddit/parse.go` — parseListing pattern.
- `internal/adapters/reddit/client.go` — HTTP client + redirect allowlist + categorizeStatus.
- `internal/adapters/reddit/score.go` — Tanh formula.
- `internal/adapters/reddit/errors.go` — sentinel sentinels + parseRetryAfter.
- `internal/adapters/hn/hn.go:1-138` — second-adapter Adapter struct.
- `internal/adapters/hn/strip.go` — stripHTML helper (NOT reused — Bluesky text is plain).
- `internal/router/category.go:14-15` — `CategorySocial` definition; covers Bluesky + X.
- `.moai/project/roadmap.md:51` — SPEC-ADP-006 row.
- `.moai/project/roadmap.md:122-123` — M3 7-way parallelization gating on FAN-001.
- `.moai/project/structure.md:18-22` — `internal/adapters/{xtwitter,bluesky}/` reservation. NOTE: structure.md uses two separate package names; the SPEC consolidates into ONE `social/` package per §3.3 — a structure.md follow-up sync task is recommended (Open Question §11.1).
- `.moai/project/tech.md:107` — X / Twitter row (ScrapeCreators or Nitter).
- `.moai/project/tech.md:110` — Bluesky row (AT Protocol public feed, anonymous, generous).
- `.moai/project/tech.md:147` — ToS feature-flag mandate.
- `.moai/config/sections/quality.yaml` — `development_mode: tdd`, `test_coverage_target: 85`.
- `.moai/config/sections/harness.yaml` — auto-routing.
- `.moai/config/sections/language.yaml` — `documentation: en`, `code_comments: en`.
- `go.mod` — current module surface; verified no atproto deps; `go.uber.org/goleak` already pinned via ADP-001 run-phase.

---

End of Research Document.

**Summary for SPEC Author**: Bluesky-only v0 integration via the public
AppView (`https://public.api.bsky.app/xrpc/app.bsky.feed.searchPosts`),
no auth, ~600/min IP rate limit, stdlib HTTP only (no
`bluesky-social/indigo` dependency). Field mapping per §1.3 with social
extras (`handle`, `post_uri`, `repost_count`, `like_count`,
`posted_at`) in Metadata. X sub-source RESERVED but disabled via
`USEARCH_X_ENABLED=true` env gate; default returns
`*SourceError{CategoryPermanent, ErrXDisabled}`. Single
`internal/adapters/social/` package, two `Adapter` instances
(`NewBluesky`, `NewX`) sharing one struct with `subSource` dispatch
field — the registry observability emits two adapter labels (`bluesky`,
`x`). ToS / legal risk surfaced; ScrapeCreators integration deferred
to a future SPEC-ADP-006-XENABLE behind explicit operator
acknowledgement. SPEC should span 10-12 EARS REQs covering interface
conformance, search method, capabilities (intent: social, sub-sources:
[bluesky, x_optional]), Bluesky URL construction, opaque cursor,
Bluesky field mapping (incl. social extras), authentication-less
posture, X opt-in gating, error categorisation, race-safe across both
sub-sources, observability with two adapter labels, rate limits per
sub-source, resource cleanup. NFRs: parse p50 ≤ 5ms, allocs/op ≤ 500,
race-clean (50 goroutines), goleak-clean. Harness level: standard.
Sprint Contract optional. Coverage target 85%. Zero new Go module
dependencies.
