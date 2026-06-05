# SPEC-ADP-001a Research — Reddit App-Only OAuth

This research artifact records the codebase investigation and external
findings that ground SPEC-ADP-001a. Every claim below is either
file-cited (path:line) or URL-cited. The SPEC document treats the
LOCKED DECISIONS (D1..D4) as ground truth supplied by the user; this
file records the evidence that those decisions are consistent with the
existing codebase.

---

## 0. Problem Statement

The Reddit public JSON search endpoint
`https://www.reddit.com/search.json` now returns **HTTP 403** for
anonymous (unauthenticated) requests. The current Reddit adapter has
no authentication path, so every Reddit `Search` call fails. The
adapter's `categorizeStatus` maps `403 → CategoryPermanent`
(`internal/adapters/reddit/client.go:112-114`), so the fanout layer
treats Reddit as permanently dead and never retries. Net effect:
Reddit search is non-functional in production.

This SPEC fulfills the OAuth deferral explicitly recorded in the
parent SPEC-ADP-001:

> **D1 Endpoint + Auth**: Public `https://www.reddit.com/search.json`
> no-auth path. … OAuth (`oauth.reddit.com`) is OUT OF SCOPE;
> deferred to a future ADP-001a SPEC if measured value warrants.
> (`.moai/specs/SPEC-ADP-001/spec.md:39-43`)

and again in the parent Out-of-Scope and What-NOT-to-Build sections:

> **OAuth-authenticated variant** (`oauth.reddit.com` endpoint,
> 60/min rate limit, per-team Reddit app credentials) → future
> SPEC-ADP-001a if measured value warrants.
> (`.moai/specs/SPEC-ADP-001/spec.md:253-256` and `:913-915`)

Measured value now clearly warrants it: the anonymous path is dead, so
OAuth is the only way to keep Reddit search alive.

---

## 1. Current Reddit Adapter State (file-cited)

### 1.1 Options struct — NO Token / credential field

`internal/adapters/reddit/reddit.go:33-49`. `Options` has exactly four
fields: `BaseURL`, `HTTPClient`, `UserAgentVersion`,
`HealthcheckTarget`. There is no `ClientID`, `ClientSecret`, `Token`,
or `OAuthURL` field. The constructor (`reddit.go:63-91`) does NO auth
validation — the comment at `:60-62` notes `New` "currently always
nil; reserved for future validation."

### 1.2 Capabilities — RequiresAuth:false

`internal/adapters/reddit/reddit.go:99-118`. `Capabilities()` returns
`RequiresAuth: false`, `AuthEnvVars: nil`, `RateLimitPerMin: 10`. The
`Notes` field documents the public no-auth endpoint.

### 1.3 Endpoint + User-Agent

`defaultBaseURL = "https://www.reddit.com/search.json"`
(`reddit.go:17`). `defaultUserAgentTemplate =
"usearch/%s (+https://github.com/elymas/universal-search)"`
(`reddit.go:19-21`), `defaultUAVersion = "v0.1"` (`reddit.go:24`).
The User-Agent header is set on every outbound request at
`internal/adapters/reddit/client.go:72`. Reddit blocks the default Go
`net/http` UA, so the custom UA is a HARD precondition for any request
(parent REQ-ADP-009).

### 1.4 Status categorization — 403 → CategoryPermanent

`internal/adapters/reddit/client.go:102-124`. The `categorizeStatus`
switch maps `429 → CategoryRateLimited`, `4xx → CategoryPermanent`
(this catches 401, 403, 404), `5xx → CategoryUnavailable`,
`0 → CategoryUnavailable`. Crucially there is **no special-casing of
401**: an expired/invalid bearer token (401) would currently be
classified `CategoryPermanent` — which is wrong once OAuth lands
(401 is recoverable via token refresh).

### 1.5 Search hot path

`internal/adapters/reddit/search.go:43-110`. `Search` validates the
query (`isAllWhitespace`, REQ-ADP-008), builds the URL via
`buildSearchURL` (`search.go:113-125`, hard-codes
`sort=relevance&t=all&type=link`), issues one request via `doRequest`,
and maps non-200 responses through `categorizeStatus`. There is no
Authorization header, no token cache, no refresh logic.

### 1.6 BaseURL test override seam exists; no OAuth URL seam

`Options.BaseURL` (`reddit.go:34-36`) is the existing test override
for the search endpoint, consumed in `cmd/usearch/query.go:461-462`
via `os.Getenv("REDDIT_BASE_URL")`. There is no analogous
`REDDIT_OAUTH_URL` seam yet — D3 requires adding one so the token
endpoint can be stubbed in CI.

### 1.7 Error sentinels — no ErrMissingToken

`internal/adapters/reddit/errors.go:1-63`. Defines `ErrInvalidQuery`
(`:16`) and the `parseRetryAfter` helper (`:33-63`, 60s cap, 5s
default). There is NO `ErrMissingCredentials` / `ErrMissingToken`
sentinel — D2 requires adding one (mirroring github's
`ErrMissingToken`).

### 1.8 Current production registration — UNCONDITIONAL

`cmd/usearch/query.go:461-465`. Reddit is registered unconditionally:

```go
if a, err := reddit.New(reddit.Options{
    BaseURL: os.Getenv("REDDIT_BASE_URL"),
}); err == nil {
    _ = reg.Register(a)
}
```

`New` never errors today, so Reddit is always registered. Under D2
this must become conditional on both credential env vars being present
(graceful skip otherwise) — mirroring the GitHub pattern.

---

## 2. Reference Pattern — GitHub Authenticated Adapter (file-cited)

The GitHub adapter is the canonical reference for an auth-gated
adapter and the user's REFERENCE PATTERN. ADP-001a mirrors its design.

### 2.1 Options.Token + constructor validation

`internal/adapters/github/github.go:46-61` adds `Token`,
`SkipAuthCheck` fields. `New` (`github.go:77-84`) returns
`*types.SourceError{Adapter:"github", Category: CategoryPermanent,
Cause: ErrMissingToken}` when `Token == "" && !SkipAuthCheck &&
HTTPClient == nil`. The `SkipAuthCheck` field (`github.go:58-60`) is
the test seam that lets tests inject a custom `HTTPClient` / stub
server without supplying a token. ADP-001a needs the analogous seam so
the OAuth token-endpoint stub can be wired in CI without real Reddit
credentials.

### 2.2 Capabilities — RequiresAuth:true + AuthEnvVars

`internal/adapters/github/github.go:137-161`. `RequiresAuth: true`,
`AuthEnvVars: []string{"USEARCH_GITHUB_TOKEN"}`. The registry validates
these at registration time (see §3).

### 2.3 Conditional registration (graceful skip)

`cmd/usearch/query.go:476-487`. The CLI reads the env var, falls back
to a secondary name, and registers ONLY if the token is present:

```go
token := os.Getenv("USEARCH_GITHUB_TOKEN")
if token == "" {
    token = os.Getenv("GITHUB_TOKEN")
}
if token != "" {
    if a, err := github.New(github.Options{
        BaseURL: os.Getenv("GITHUB_BASE_URL"),
        Token:   token,
    }); err == nil {
        _ = reg.Register(a)
    }
}
```

This is the exact shape ADP-001a's registration block must adopt
(reading `REDDIT_CLIENT_ID` + `REDDIT_CLIENT_SECRET`, registering only
when BOTH are non-empty).

### 2.4 ErrMissingToken sentinel

`internal/adapters/github/errors.go:13-14`:
`ErrMissingToken = errors.New("github: token required; set
USEARCH_GITHUB_TOKEN env var")`. ADP-001a adds an analogous
`ErrMissingCredentials` sentinel to `reddit/errors.go`.

### 2.5 A second auth-gated reference: Naver (dual-env)

`internal/adapters/naver/naver.go:118-132` shows a TWO-secret adapter:
`New` resolves `NAVER_CLIENT_ID` and `NAVER_CLIENT_SECRET`, returning
an error when either is absent; `Capabilities` advertises
`AuthEnvVars: []string{"NAVER_CLIENT_ID","NAVER_CLIENT_SECRET"}`
(`naver.go:196-197`). Reddit OAuth is structurally identical (two
secrets: client_id + client_secret). Note one divergence: Naver reads
env INSIDE `New` and registers via `naver.New(naver.Options{})` with
no args in `query.go:505`. The user's D2 explicitly chooses the
**GitHub** shape (read env in `query.go`, pass into Options, skip when
absent) over the Naver shape, for symmetry with the existing Reddit
registration block that already passes `BaseURL` from env.

---

## 3. Registry Auth Validation (file-cited)

`internal/adapters/registry.go:147-166` (`RegisterWithOptions`). When
`caps.RequiresAuth` is true and `SkipAuthCheck` is false, the registry
iterates `caps.AuthEnvVars` and returns
`&RegistryError{Op:"register", Name:name, Cause: ErrMissingAuth}` if
any listed env var is unset (`registry.go:151-157`). This is a SECOND
safety net beyond the constructor check: even if the CLI forgot the
conditional, the registry would reject registration when the env vars
are absent. ADP-001a's `AuthEnvVars =
["REDDIT_CLIENT_ID","REDDIT_CLIENT_SECRET"]` therefore gets validated
both at the CLI call site (D2 graceful skip) and at `Register`.

---

## 4. Reddit App-Only OAuth Mechanics (URL-cited; D1/D3 basis)

### 4.1 Token acquisition (client_credentials / "userless")

Reddit's app-only OAuth uses the OAuth2 `client_credentials` grant
(Reddit calls it the "application-only / userless" flow). The client
POSTs to `https://www.reddit.com/api/v1/access_token` with HTTP Basic
auth (`client_id:client_secret` base64-encoded in the `Authorization`
header) and a form body `grant_type=client_credentials`. The response
is JSON `{ "access_token": "...", "token_type": "bearer",
"expires_in": 3600, "scope": "*" }`.

Sources:
- https://github.com/reddit-archive/reddit/wiki/OAuth2 — Reddit OAuth2
  overview (authorization endpoints, grant types).
- https://github.com/reddit-archive/reddit/wiki/OAuth2-Quick-Start-Example
  — quick-start showing the Basic-auth POST to
  `/api/v1/access_token`.
- https://www.reddit.com/dev/api — authenticated API host is
  `https://oauth.reddit.com`; the bearer token is presented as
  `Authorization: bearer <token>`.

### 4.2 Authenticated search host

Authenticated requests go to `https://oauth.reddit.com/search` (NOT
`www.reddit.com`). The query parameters are otherwise identical to the
public `search.json` path (`q`, `sort`, `t`, `type`, `limit`,
`include_over_18`, `after`). The custom User-Agent remains mandatory
on every request (both the token POST and the search GET).

### 4.3 Token lifecycle

App-only tokens expire (`expires_in` is typically 3600 seconds = 1
hour). The adapter must cache the token and its expiry, refresh
proactively when expired (with a small safety margin), and refresh
reactively when a search returns 401 (token revoked/invalid before
its nominal expiry). D3 mandates a test-override env
(`REDDIT_OAUTH_URL`) for the token endpoint, mirroring the existing
`REDDIT_BASE_URL` override for the search endpoint.

### 4.4 Authenticated rate limit

Authenticated Reddit OAuth clients get **60 requests/minute** (vs ~10
for anonymous). Source confirmed in the parent SPEC research notes and
the parent SPEC HISTORY discrepancy entry
(`.moai/specs/SPEC-ADP-001/spec.md:69-77`, citing
painonsocial.com 2026 guidance: "10/min unauthenticated, 60/min
authenticated"). ADP-001a updates `Capabilities.RateLimitPerMin` from
10 to 60.

---

## 5. Status Code Semantics Once Authenticated (D4 basis)

Once the adapter sends a valid bearer token, the meaning of HTTP
status codes shifts:

| Status | Anonymous meaning (today) | Authenticated meaning (D4) | Adapter action |
|--------|---------------------------|----------------------------|----------------|
| 401 | (rare; → Permanent) | token expired/invalid | refresh token once + retry; if still 401 → CategoryUnavailable |
| 403 | blocked anonymous (the bug) | genuinely forbidden (private/banned/quarantined sub) | CategoryPermanent (keep) |
| 429 | rate limited | rate limited (60/min ceiling) | CategoryRateLimited, parse Retry-After (existing logic) |
| 5xx | unavailable | unavailable | CategoryUnavailable (keep) |

The key behavioural change: 401 must be peeled out of the generic
`4xx → CategoryPermanent` branch in `categorizeStatus` and handled as
a recoverable refresh trigger. 403 stays Permanent. This is the
single most important categorization change in the SPEC.

---

## 6. Concurrency / NFR Notes

The adapter is invoked concurrently from N goroutines (parent
REQ-ADP-011, `internal/adapters/reddit/search.go:36`). The token cache
is NEW shared mutable state — the FIRST mutable state the Reddit
adapter has ever held. It MUST be guarded (e.g., `sync.Mutex` or
`sync.RWMutex` with a singleflight-style refresh to avoid a thundering
herd of concurrent token POSTs when the token expires). The existing
`TestSearchConcurrentSafe` (50 goroutines, `-race`) must continue to
pass with the token cache in place. This is the principal new NFR.

Secret hygiene: `client_id`, `client_secret`, and the bearer
`access_token` MUST NOT appear in any log, error message, or
`*types.SourceError.Cause`. The existing adapter emits no logs itself
(sole-emitter discipline — the registry wrappedAdapter logs), which
helps; the SPEC must ensure error `Cause` strings never embed the
secret or token.

---

## 7. Backward Compatibility

The parent SPEC-ADP-001 shipped with 55 tests + 1 benchmark, all green
(`.moai/specs/SPEC-ADP-001/spec.md:23`). The user constraint requires
the existing 10+ Reddit tests to keep passing. Strategy:
- Keep the existing public no-auth code paths intact where they do not
  conflict (URL parameter building, NSFW filter, parse, score,
  Retry-After, redirect allowlist, query validation).
- New OAuth tests use an `httptest.Server` token-endpoint stub + an
  `httptest.Server` search stub. NO live network in CI (mirrors parent
  D4).
- Existing tests that construct the adapter via `Options{BaseURL:...}`
  with a stub will need the auth-skip seam (`SkipAuthCheck` analogue)
  so they do not start failing on a missing-credential check. This is
  why the GitHub `SkipAuthCheck` seam (§2.1) is part of the design.

---

## 8. Unrelated Reference Note (explicit, per user instruction)

`cmd/usearch-api/main.go` contains the comment
`"usearch-api: not implemented (see SPEC-IR-001)"`
(`cmd/usearch-api/main.go:50`, with related references at `:2`, `:5`,
`:39`, `:45`, `:48`). **This `see SPEC-IR-001` reference is UNRELATED
to SPEC-ADP-001a.** It points at the Intent Router / HTTP API server
work, not Reddit OAuth. ADP-001a does NOT touch `cmd/usearch-api/`.
The only `cmd/` file ADP-001a touches is
`cmd/usearch/query.go` (the Reddit registration block). Recording this
here so a future reader does not conflate the two.

---

## 9. Scope Boundary (kept tight)

ADP-001a is strictly Reddit app-only OAuth: token acquisition,
caching, refresh, bearer-authenticated search against
`oauth.reddit.com`, conditional registration, 401/403/429
categorization, and concurrency-safe token storage. Everything else
the parent SPEC deferred (user-login authorization_code flow,
subreddit-scoped search, time-range filtering, sort customization,
comment retrieval, live-network CI tests) remains OUT OF SCOPE. The
SPEC's §7 Exclusions enumerates these.

---

## 10. References

### External (URL-cited)

- https://github.com/reddit-archive/reddit/wiki/OAuth2 — Reddit OAuth2
  flows (client_credentials / userless application-only grant).
- https://github.com/reddit-archive/reddit/wiki/OAuth2-Quick-Start-Example
  — Basic-auth POST to `/api/v1/access_token`.
- https://www.reddit.com/dev/api — authenticated host
  `https://oauth.reddit.com`, `Authorization: bearer <token>`.
- RFC 6749 §4.4 — OAuth 2.0 Client Credentials Grant.
- RFC 7231 §7.1.3 — Retry-After header (already implemented in
  `reddit/errors.go:33-63`).

### Internal (file:line cited)

- `.moai/specs/SPEC-ADP-001/spec.md:39-43, 69-77, 253-256, 913-915` —
  parent SPEC OAuth deferral + rate-limit discrepancy.
- `internal/adapters/reddit/reddit.go:17,19-24,33-49,63-91,99-118` —
  current Options / constructor / Capabilities / endpoint / UA.
- `internal/adapters/reddit/client.go:71-75,102-124` — doRequest UA
  header, categorizeStatus (403→Permanent).
- `internal/adapters/reddit/search.go:43-110,113-125` — Search hot
  path + buildSearchURL.
- `internal/adapters/reddit/errors.go:16,33-63` — ErrInvalidQuery,
  parseRetryAfter.
- `internal/adapters/github/github.go:46-61,77-84,137-161` — Token
  field, constructor validation, RequiresAuth Capabilities.
- `internal/adapters/github/errors.go:13-14` — ErrMissingToken
  sentinel.
- `internal/adapters/naver/naver.go:118-132,196-197` — dual-secret
  reference.
- `cmd/usearch/query.go:461-465,476-487` — Reddit (unconditional) +
  GitHub (conditional) registration.
- `internal/adapters/registry.go:147-166` — registry AuthEnvVars
  validation.
- `pkg/types/adapter.go:28-45` — Adapter interface.
- `pkg/types/capabilities.go:38-61` — Capabilities struct fields.
- `pkg/types/query.go:18-44` — Query + Filter shape.
- `cmd/usearch-api/main.go:2,50` — UNRELATED SPEC-IR-001 reference.

---

*End of SPEC-ADP-001a research.md*
