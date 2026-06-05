# SPEC-ADP-006-XENABLE Research — X (Twitter) Live Provider Enablement

**Status**: Research-complete; ready for SPEC drafting
**Author**: limbowl (via manager-spec)
**Created**: 2026-06-04
**Milestone**: M3 — Fanout, adapters, index
**Depends on**: SPEC-ADP-006, SPEC-CORE-001, SPEC-IR-001, SPEC-OBS-001
**Parent SPEC**: SPEC-ADP-006 (Bluesky + X Adapter) — this SPEC resolves its
deferred X provider decision (parent §11.4, §2.2, §6.4, §10 ToS row).
**Resolves audit finding**: F-08 (`.planning/AUDIT-FINDINGS.md:22`).

---

## 0. Research Mandate

SPEC-ADP-006 shipped the X (Twitter) sub-source **reserved-but-disabled** in
v0. Its `searchX` path (`internal/adapters/social/search_x.go:23-31`) returns
`ErrXDisabled` when `USEARCH_X_ENABLED != "true"` and
`ErrXProviderNotConfigured` when the env gate is on but no provider is wired.
The parent SPEC explicitly named a successor by ID:

> ScrapeCreators integration is DEFERRED to a future SPEC-ADP-006-XENABLE
> which will require explicit ToS acknowledgement at deployment time.
> (`.moai/specs/SPEC-ADP-006/spec.md:83-84`)

and assigned this SPEC's author the provider-choice open question:

> **X provider choice (when XENABLE happens)**. ScrapeCreators vs Nitter vs
> official tier. […] **Resolution owner**: future SPEC-ADP-006-XENABLE author.
> (`.moai/specs/SPEC-ADP-006/spec.md:1224-1230`)

This research artifact's mandate:

- Verify the current X stub state in `internal/adapters/social/` against the
  real merged code (NOT the parent SPEC's illustrative §6.6 sketch, which has
  drifted — see §1.3 below).
- Document the realistic X-search provider landscape in 2026 with current
  capabilities, pricing, and ToS posture — every external claim WebSearch /
  WebFetch-cited.
- Resolve the parent's provider open question by designing against a
  provider-abstraction interface so the concrete provider is pluggable, and
  by documenting the activation precondition as an external blocker.
- Map each provider's response envelope to the SPEC-CORE-001 NormalizedDoc
  contract, mirroring the Bluesky normalization already in
  `internal/adapters/social/parse.go`.
- Surface the test-isolation constraint inherited from the parent (NO
  `t.Setenv` under `-race`; `XOptions.EnvLookup` injection only — parent H1).

Every claim is either file-cited (e.g.,
`internal/adapters/social/social.go:164-178`) or URL-cited from verified web
sources. No invented endpoints, pricing, or API shapes.

---

## 1. Current X Stub State (verified against real source)

### 1.1 Constructor and Options

`internal/adapters/social/social.go:61-66` defines:

```go
type XOptions struct {
    EnvLookup func(string) string
}
```

[VERIFIED] `XOptions` carries ONLY `EnvLookup`. It has NO `HealthcheckTarget`,
no `BaseURL`, no `HTTPClient`, no `Token` field. The parent SPEC §6.6
illustrative sketch showed a `HealthcheckTarget` field on `XOptions` — that
field does **not** exist in the merged code. XENABLE must extend `XOptions`
with provider-config fields (see §4).

`NewX(opts XOptions)` (`social.go:110-124`) constructs:
- `httpClient: &http.Client{}` (non-nil, default zero-value client)
- `baseURL: ""`, `userAgent:` set via template, `healthcheckTarget: ""`
- `subSource: "x"`, `envLookup:` opts.EnvLookup or `os.Getenv`

### 1.2 Search and Healthcheck dispatch

- `Search` (`social.go:181-194`) switches on `subSource`; `"x"` → `searchX`.
- `searchX` (`search_x.go:23-31`): `val := a.envLookup("USEARCH_X_ENABLED")`;
  `val != "true"` → returns `ErrXDisabled`; otherwise →
  `ErrXProviderNotConfigured`. Const `xEnabledEnvVar = "USEARCH_X_ENABLED"`
  (`search_x.go:12`). Makes ZERO HTTP requests.
- `Healthcheck` (`social.go:199-213`): for `"x"` returns `ErrXDisabled`
  directly (L208-209). [VERIFIED] X Healthcheck does NOT TCP-probe an
  endpoint — the parent §6.6 sketch showed a TCP dial for both sub-sources;
  the real code only dials for Bluesky. XENABLE must change this branch to
  probe the configured provider when enabled.

### 1.3 Sentinels

`internal/adapters/social/errors.go:19-46`:
- `ErrXDisabled` and `ErrXProviderNotConfigured` are package-level
  `*types.SourceError` vars (both `Category: types.CategoryPermanent`),
  whose `.Cause` is a private `xSentinelError` string type. [VERIFIED] They
  are NOT bare `errors.New(...)` values — the parent §2.1(i) prose described
  them as `errors.New(...)`; the merged code wraps them as `*SourceError`.
- `parseRetryAfter(header string, now time.Time) time.Duration`
  (`errors.go:61-91`) caps at 60s, defaults to 5s. Reusable by the X live
  path verbatim.

### 1.4 Capabilities (disabled descriptor)

`xCapabilities()` (`social.go:164-178`): `SourceID:"x"`,
`DisplayName:"X (Twitter)"`, `DocTypes:[DocTypePost]`, `SupportedLangs:nil`,
`SupportsSince:false`, `RequiresAuth:false`, `AuthEnvVars:nil`,
`RateLimitPerMin:0`, `DefaultMaxResults:0`, `Notes` contains
`"DISABLED in v0"`, `"USEARCH_X_ENABLED=true"`,
`"SPEC-ADP-006-XENABLE pending"`, `"social"`. XENABLE flips this to a live
descriptor when the env gate + provider config are present (see §5).

### 1.5 Production registry wiring

`cmd/usearch/query.go:498-503` (`buildProductionRegistry`):

```go
// Bluesky live (X is stub-only — disabled until provider configured).
if a, err := social.NewBluesky(social.BlueskyOptions{
    BaseURL: os.Getenv("BLUESKY_BASE_URL"),
}); err == nil {
    _ = reg.Register(a)
}
```

[VERIFIED] `social.NewX` is NOT called in `buildProductionRegistry`. XENABLE
must add a gated `NewX` registration here, behind `USEARCH_X_ENABLED=true`
AND provider credentials present (see §6). The registration pattern mirrors
the existing env-gated adapters: GitHub (`query.go:476-487`, gated on token),
YouTube (`query.go:488-494`, gated on `YOUTUBE_BASE_URL`).

### 1.6 Live-path template: Bluesky

`internal/adapters/social/search_bluesky.go:36-122` is the structural
template the X live path must mirror:
1. Validate query (`isBlankQuery`) → `ErrInvalidQuery` wrapped in
   `*SourceError{CategoryPermanent}`.
2. Build params via `url.Values`; clamp limit (1..100, default 25).
3. `http.NewRequestWithContext` + `a.doRequest` (sets UA + Accept, enforces
   CheckRedirect allowlist).
4. Non-200 → `categorizeStatus(adapterName, status, retryAfter, nil)` (429
   parses `Retry-After`).
5. `io.ReadAll` → `parseSearchPosts(body, retrievedAt)` → `[]NormalizedDoc`.

`categorizeStatus` (`client.go:91-111`) already accepts `adapterName="x"`.
`doRequest` (`client.go:62-66`) is sub-source-agnostic.

---

## 2. X (Twitter) Search Provider Landscape (2026)

The parent SPEC assumed **ScrapeCreators** as the v1 candidate
(`spec.md:1226-1229`). This research **contradicts** that assumption for the
keyword-search use-case (§2.2). Three realistic options were evaluated.

### 2.1 Option A — X Official API v2

**Endpoints** (https://docs.x.com/x-api/posts/recent-search,
https://docs.x.com/x-api/posts/search/integrate/build-a-query):
- `GET /2/tweets/search/recent` — Posts from the last 7 days matching a query.
- `GET /2/tweets/search/all` — full-archive search (advanced access level).

**Query**: advanced operator syntax (core operators available to any project;
advanced operators reserved for higher access). Recent-search query string
char limit is 512 (self-serve) / 4096 (enterprise); full-archive 1024 /
4096 (https://docs.x.com/x-api/posts/search/integrate/build-a-query).

**Request params**: `query`, `start_time`, `end_time`, `sort_order`
(recency/relevancy), `max_results`, `next_token` (pagination),
`tweet.fields`, `expansions`, `user.fields`.

**Response fields**: with `tweet.fields=public_metrics,created_at`, each Post
carries `id`, `text`, `created_at`, `author_id`, and `public_metrics`
containing `like_count`, `retweet_count`, `reply_count`, `quote_count`,
`impression_count`, `bookmark_count`
(https://docs.x.com/x-api/fundamentals/metrics). Author handle/displayName
require an `expansions=author_id` + `user.fields` join.

**Auth**: Bearer Token (OAuth2 app-only) or OAuth2 user token with
`tweet.read` + `users.read` scopes
(https://docs.x.com/x-api/posts/recent-search).

**Pricing**: As of 2026 the official API uses a **pay-per-usage credit
model** — credits purchased upfront, "$0.005 per Post read", per-endpoint
rates in the Developer Console
(https://docs.x.com/x-api/getting-started/pricing). The previously-reported
Free / Basic ($100/mo) / Pro ($5,000/mo) / Enterprise monthly tiers are
superseded by this consumption model. [NOTE] The parent SPEC §11.4 cited the
old tiered model; this research supersedes it.

**ToS posture**: Official, first-party. No scraping. Lowest legal risk.
Cost is the binding constraint (every Post read is metered).

### 2.2 Option B — ScrapeCreators (parent's assumed provider) — REJECTED for search

ScrapeCreators' documented Twitter endpoints are: GET Profile, GET User
Tweets, GET Tweet Details, GET Transcript, GET Community, GET Community
Tweets (https://docs.scrapecreators.com/v1/twitter). [VERIFIED] There is
**no keyword-search endpoint** for Twitter/X — search exists for other
platforms (TikTok, Instagram, YouTube) but not X. Auth is `x-api-key`
header; "no API rate limits" (keep below 500 concurrent).

**Conclusion**: ScrapeCreators cannot satisfy the X keyword-search contract
the adapter requires. The parent SPEC's ScrapeCreators-for-search assumption
is incorrect. ScrapeCreators is suited to profile / user-timeline pulls, not
the `types.Query.Text` → matched-Posts shape this adapter needs.

### 2.3 Option C — twitterapi.io (third-party search aggregator)

**Endpoint** (https://docs.twitterapi.io/api-reference/endpoint/tweet_advanced_search):
`GET /twitter/tweet/advanced_search` on base `https://api.twitterapi.io`.

**Request params**: `query` (required; advanced operator syntax incl.
`from:`, `since_time:`, `until_time:` timestamp operators), `queryType`
(required; `"Latest"` or `"Top"`), `cursor` (optional; empty string for first
page).

**Response fields**: `text`, `author` (UserInfo object), `likeCount`,
`retweetCount`, `replyCount`, `createdAt` (e.g.
`"Tue Dec 10 07:00:30 +0000 2024"`), `url`, plus `has_next_page` and
`next_cursor` pagination controls.

**Auth**: `X-API-Key` header.

**Pricing**: 15 credits per tweet returned = **$0.15 per 1,000 tweets**; no
free tier; pay-as-you-go; 1 USD = 100,000 credits; minimum $0.00015 (15
credits) per call (https://twitterapi.io/pricing).

**ToS posture**: Third-party aggregator. Higher legal-risk class than the
official API (falls under tech.md:147's "scraped sources" feature-flag
mandate). Cost is ~30× cheaper per result than the official $0.005/Post.

### 2.4 Provider comparison

| Dimension | A: X Official API | B: ScrapeCreators | C: twitterapi.io |
|-----------|-------------------|-------------------|------------------|
| Keyword search endpoint | Yes (recent + full-archive) | **No** (rejected) | Yes (advanced_search) |
| Query syntax | Advanced operators | n/a | Advanced operators |
| Pagination | `next_token` | n/a | `cursor` / `next_cursor` |
| Engagement metrics | public_metrics (6) | n/a | like/retweet/reply counts |
| Author handle | via expansions join | n/a | inline `author` object |
| Auth | Bearer Token | x-api-key | X-API-Key |
| Cost / result | ~$0.005 / Post | n/a | ~$0.00015 / tweet |
| ToS risk class | Low (first-party) | n/a | Higher (aggregator) |

**Resolution of parent §11.4**: Do NOT hard-pick a single provider in the
SPEC. Design against an `XProvider` interface (§4) so the concrete provider
is pluggable; A (official) and C (twitterapi.io) are the two realistic
concrete implementations. B (ScrapeCreators) is removed from consideration
for search. **Provider selection is a business / credentials / ToS decision
that is an EXTERNAL BLOCKER** — see §7.

---

## 3. Why a provider-abstraction interface (not a hard provider pick)

1. **No locked-in default**: A is lowest-risk but most expensive; C is
   cheapest but higher ToS-risk class. The choice depends on operator budget
   and legal posture — both outside this SPEC's authority.
2. **External blocker decoupling**: The SPEC can fully specify the
   integration *contract* (interface, normalization, error mapping,
   registration gating) without the paid credentials existing. Activation
   requires creds + ToS acknowledgement, which the SPEC documents as a
   precondition rather than a deliverable.
3. **Testability**: A provider interface is trivially fake-able in tests
   (a `fakeProvider` returning canned results), so the live Search path is
   fully TDD-covered with ZERO live network calls and ZERO real credentials,
   exactly as the Bluesky path uses `httptest.Server` + fixtures.
4. **Mirrors existing discipline**: SPEC-ADP-006 already split the adapter
   from its transport concerns; an `XProvider` is the natural seam for the X
   sub-source's pluggable backend.

---

## 4. Provider Interface Sketch (illustrative; final shape in run phase)

```go
// internal/adapters/social/x_provider.go
package social

import (
    "context"

    "github.com/elymas/universal-search/pkg/types"
)

// XProvider abstracts a concrete X (Twitter) search backend. Implementations
// translate a normalized Query into provider-specific HTTP calls and return
// raw provider results. The adapter owns normalization to NormalizedDoc.
//
// Implementations MUST NOT panic; transport/quota/auth failures map to a
// *types.SourceError with the appropriate Category.
type XProvider interface {
    // Name identifies the concrete provider (e.g. "x-official", "twitterapi-io").
    Name() string
    // SearchTweets executes one search page and returns raw results plus an
    // opaque next-page cursor (empty when exhausted).
    SearchTweets(ctx context.Context, q types.Query) (results []XTweet, nextCursor string, err error)
}

// XTweet is the provider-neutral intermediate shape the adapter normalizes
// into NormalizedDoc. Concrete providers populate it from their own envelopes.
type XTweet struct {
    ID           string
    Text         string
    AuthorHandle string
    URL          string
    CreatedAt    string // provider-native timestamp string
    LikeCount    int
    RepostCount  int
    ReplyCount   int
    QuoteCount   int
}

// Extended XOptions (run phase appends provider config to the existing struct).
//   Provider XProvider          // injected concrete provider; nil => not configured
//   EnvLookup func(string) string  // existing field, retained
```

The adapter's existing `score.go::normalizeScore(likeCount, repostCount)`
(Tanh formula, parent §2.3) is reused verbatim for X with
`x = LikeCount + RepostCount`.

---

## 5. X Tweet → NormalizedDoc Field Mapping (provider-neutral)

Mirrors the Bluesky mapping in `internal/adapters/social/parse.go`.

| XTweet field | NormalizedDoc field | Transform |
|--------------|---------------------|-----------|
| `ID` | `ID` | `"x:" + ID` |
| (constant) | `SourceID` | `"x"` (matches `Name()`) |
| `URL` | `URL` | provider URL, or constructed `"https://x.com/" + handle + "/status/" + ID` when absent |
| `truncateRunes(Text, 280)` | `Title` | first 280 runes |
| `Text` | `Body` | as-is (plain text) |
| `truncateRunes(Text, 280)` | `Snippet` | first 280 runes |
| `CreatedAt` (parsed) | `PublishedAt` | best-effort parse to UTC; zero on parse error |
| (parse time) | `RetrievedAt` | `time.Now().UTC()` |
| `AuthorHandle` | `Author` | as-is |
| `normalizeScore(LikeCount, RepostCount)` | `Score` | Tanh formula (parent §2.3) |
| (empty unless detectable) | `Lang` | `""` in v0 (provider lang inference deferred) |
| (constant) | `DocType` | `types.DocTypePost` |
| (constant) | `Hash` | `""` (consumers compute `CanonicalHash()`) |
| (constructed) | `Metadata` | REQUIRED keys: `handle`, `tweet_id`, `like_count`, `repost_count`, `reply_count`, `posted_at`, `sub_source` (=`"x"`), `provider` (provider Name). LAST doc gets `next_cursor` when non-empty. |

`Validate()` requires only ID, SourceID, URL, RetrievedAt
(`pkg/types/normalized_doc.go:63-78`) — all populated above.

---

## 6. Registration Gating (cmd/usearch/query.go)

XENABLE adds to `buildProductionRegistry` (after the Bluesky block at
`query.go:498-503`):

```go
// X live (gated): requires USEARCH_X_ENABLED=true AND provider creds present.
if os.Getenv("USEARCH_X_ENABLED") == "true" {
    if prov, ok := buildXProvider(); ok { // reads provider-specific creds env
        if a, err := social.NewX(social.XOptions{Provider: prov}); err == nil {
            _ = reg.Register(a)
        }
    }
    // env on but no creds → NewX still constructs; Search returns
    // ErrXProviderNotConfigured (unchanged backward-compat behavior).
}
```

[NOTE] When `USEARCH_X_ENABLED` is unset (the default), `NewX` is NOT
registered at all — identical to the v0 status quo, so no behavior change
for existing deployments. The two-error semantics
(`ErrXDisabled` / `ErrXProviderNotConfigured`) is preserved for callers that
construct `NewX` directly.

---

## 7. External Blocker Precondition (explicit)

[HARD] X live activation requires, OUTSIDE this SPEC's code deliverables:

1. **Paid provider credentials**: an X Developer account with metered credits
   (Option A) OR a twitterapi.io API key with funded credits (Option C).
   Both are paid; neither has a usable free tier for search.
2. **ToS acknowledgement at deployment time**: per `tech.md:147` ("ToS
   violation on scraped sources (X, Instagram) … Feature-flag behind team
   opt-in; default to API-based adapters only"). If the chosen provider is a
   third-party aggregator (Option C), an explicit ToS-acknowledgement gate
   (env var) is REQUIRED before the live path activates. Option A (official
   API) is first-party and satisfies the "API-based adapters only" default.

This SPEC designs the integration contract. It does NOT — and cannot —
activate the live path, because activation depends on the paid credentials
and ToS decision above. The SPEC is implementable and fully testable (via
`fakeProvider`) without those preconditions; only production *activation* is
blocked.

---

## 8. Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Provider choice changes after SPEC approval | Medium | Low | `XProvider` interface makes the concrete provider pluggable; swapping A↔C is a new file, not a contract change. |
| Official API cost blowout (metered per Post) | Medium | High | `DefaultMaxResults` cap + `Query.MaxResults` clamp bound per-call reads; operator budget is an external decision. |
| twitterapi.io ToS-risk class | High | High | Gate behind explicit ToS-ack env var when a third-party provider is selected; default deployment uses no X provider (status quo). |
| Backward-compat regression: env-unset behavior changes | Low | High | Tests assert env-unset still returns `ErrXDisabled` unchanged; registration only added inside the `== "true"` branch. |
| `t.Setenv` race under `-race` | Medium | Medium | Parent H1 mandate: all env-dependent tests use `XOptions.EnvLookup` injection, never `t.Setenv`/`os.Setenv`. |
| Provider timestamp formats differ (RFC3339 vs Twitter format) | High | Low | Normalization parses best-effort; `PublishedAt` zero on parse error (Validate does not require it). |
| Author handle absent (official API needs expansions join) | Medium | Low | URL falls back to `tweet_id`-only `x.com/i/status/<id>` form; `Author=""` permitted. |

---

## 9. Open Questions

1. **Default concrete provider when both creds present** — recommended
   default: prefer Option A (official, lowest ToS risk) when its creds exist;
   fall back to Option C only behind explicit ToS-ack. Resolution owner:
   run-phase implementer + operator.
2. **ToS-ack mechanism** (env var name vs config flag) — recommended:
   `USEARCH_X_TOS_ACK=true` env var, required only when the selected provider
   is third-party. Resolution owner: this SPEC's run phase.
3. **Lang inference for X posts** — recommended: `Lang=""` in v0; defer to
   SPEC-IDX-003. Resolution owner: SPEC-IDX-003 author.
4. **Full-archive vs recent search (Option A)** — recommended: recent search
   (`/2/tweets/search/recent`, 7-day window) in v0; full-archive needs
   advanced access. Resolution owner: run-phase implementer.

---

## 10. References

### External (URL-cited; WebFetch-verified 2026-06-04)

- https://docs.x.com/x-api/posts/recent-search — `GET /2/tweets/search/recent`
  (last-7-days), auth (Bearer / OAuth2 scopes), request params.
- https://docs.x.com/x-api/posts/search/integrate/build-a-query — query
  operators (core vs advanced), recent vs full-archive char limits.
- https://docs.x.com/x-api/fundamentals/metrics — `public_metrics` fields
  (like_count, retweet_count, reply_count, quote_count, impression_count,
  bookmark_count).
- https://docs.x.com/x-api/getting-started/pricing — pay-per-usage credit
  model, "$0.005 per Post read".
- https://docs.scrapecreators.com/v1/twitter — ScrapeCreators Twitter
  endpoints (profile/user-tweets/tweet-details/community); NO keyword search.
- https://docs.twitterapi.io/api-reference/endpoint/tweet_advanced_search —
  `GET /twitter/tweet/advanced_search`, params (query, queryType, cursor),
  response fields, `X-API-Key` auth.
- https://twitterapi.io/pricing — 15 credits/tweet = $0.15/1000 tweets, no
  free tier.

### Internal (file:line cited)

- `internal/adapters/social/social.go:61-66` — `XOptions{EnvLookup}` (no
  HealthcheckTarget).
- `internal/adapters/social/social.go:110-124` — `NewX` constructor.
- `internal/adapters/social/social.go:164-178` — `xCapabilities()`.
- `internal/adapters/social/social.go:181-194` — `Search` dispatch.
- `internal/adapters/social/social.go:199-213` — `Healthcheck` (X returns
  `ErrXDisabled`).
- `internal/adapters/social/search_x.go:12,23-31` — `searchX`,
  `xEnabledEnvVar`.
- `internal/adapters/social/search_bluesky.go:36-122` — live-path template.
- `internal/adapters/social/client.go:62-66,91-111` — `doRequest`,
  `categorizeStatus` (accepts `"x"`).
- `internal/adapters/social/errors.go:19-46,61-91` — `ErrXDisabled`,
  `ErrXProviderNotConfigured` (`*SourceError` vars), `parseRetryAfter`.
- `internal/adapters/social/score.go` — `normalizeScore` Tanh formula.
- `internal/adapters/social/parse.go` — Bluesky normalization template.
- `cmd/usearch/query.go:476-503` — `buildProductionRegistry` env-gated
  registration patterns (GitHub/YouTube/Bluesky).
- `pkg/types/capabilities.go:38-62` — `Capabilities` struct.
- `pkg/types/errors.go:19-120` — sentinels, `Category`, `*SourceError`.
- `pkg/types/query.go` — `Query`, `Filter`.
- `pkg/types/normalized_doc.go:40-78` — `NormalizedDoc`, `Validate`.
- `internal/adapters/registry.go:136,478-482` — `Register`, `wrappedAdapter`
  sole-emitter.
- `.moai/specs/SPEC-ADP-006/spec.md:83-84,1224-1230,1190` — parent deferral,
  provider open question, ToS risk row.
- `.moai/project/tech.md:107,147` — X row, ToS feature-flag mandate.
- `.planning/AUDIT-FINDINGS.md:22` — F-08.

---

*End of SPEC-ADP-006-XENABLE research (2026-06-04)*
