---
id: SPEC-ADP-001a
title: Reddit App-Only OAuth (Amendment to SPEC-ADP-001)
version: 0.1.0
milestone: M2 — First end-to-end slice (amendment)
status: implemented
priority: P1
owner: expert-backend
methodology: tdd
coverage_target: 85
created: 2026-06-04
updated: 2026-06-04
author: limbowl
issue_number: null
depends_on: [SPEC-ADP-001]
blocks: []
---

# SPEC-ADP-001a: Reddit App-Only OAuth (Amendment to SPEC-ADP-001)

## HISTORY

- 2026-06-04 (plan-audit fixes, limbowl via manager-spec): Applied the
  3 MAJOR + 1 MINOR findings from the plan-auditor APPROVE-WITH-FIXES
  verdict (`.moai/specs/SPEC-ADP-001a/audit.md`, 0 BLOCKER). (M-1) Split
  the overloaded Ubiquitous REQ-ADP-001a-006 into three atomic,
  correctly-patterned REQs: 006a (Ubiquitous — UA + Accept on every
  request), 006b (Event-Driven — `New` credential gate + graceful
  registration skip), 006c (Ubiquitous — Capabilities auth shape);
  re-mapped the §5 acceptance bullets to the split IDs. (M-2) Made the
  backward-compat sweep concrete in §8 + Milestone 7: enumerated the
  ~25+ `BaseURL`-stub construction sites across `reddit_test.go`,
  `search_test.go`, `client_test.go` that need `SkipAuthCheck:true`
  (the `HTTPClient==nil` escape does NOT spare them), plus the m-6
  default-host non-stub case (`TestHealthcheck`); added a DoD/acceptance
  gate that the full 59-function parent suite stays green. (M-3)
  Tightened the concurrency bound from "a bounded small number (≤ 5)"
  to "EXACTLY ONCE (`== 1`)" in REQ-ADP-001a-003, NFR-ADP-001a-001,
  §2.1 item (l), §5 acceptance, and the risk table — the mandated
  `sync.Mutex` + double-check guarantees a single token POST. (m-4)
  Parenthesized the `New` credential condition in §2.1 item (a) to
  `(ClientID=="" || ClientSecret=="") && !SkipAuthCheck &&
  HTTPClient==nil` (Go precedence). (m-5) Trimmed the parent-parity
  restatement in REQ-002 to a single reference. No BLOCKER existed; the
  approach and OAuth mechanics were validated unchanged by the audit
  (Dimension 4 PASS).

- 2026-06-04 (initial draft v0.1, limbowl via manager-spec):
  First EARS-formatted SPEC drafted after the research phase
  (`.moai/specs/SPEC-ADP-001a/research.md`, every claim file-cited or
  URL-cited). This SPEC is an **AMENDMENT to SPEC-ADP-001** (the
  Reddit reference adapter). SPEC-ADP-001 explicitly deferred OAuth:

  > **OAuth-authenticated variant** (`oauth.reddit.com` endpoint,
  > 60/min rate limit, per-team Reddit app credentials) → future
  > SPEC-ADP-001a if measured value warrants.
  > (`.moai/specs/SPEC-ADP-001/spec.md:913-915`)

  Measured value now warrants it: Reddit's public
  `https://www.reddit.com/search.json` endpoint returns **HTTP 403**
  for anonymous requests, and the existing adapter maps
  `403 → CategoryPermanent` (`internal/adapters/reddit/client.go:112-114`),
  so the fanout layer treats Reddit as permanently dead and never
  retries. Reddit search is non-functional in production
  (research.md §0).

  This SPEC fulfills the ADP-001a deferral by adding Reddit app-only
  (`client_credentials` / "userless") OAuth. It keeps the REQ-ID
  namespace in the ADP-001 family using a `-a` suffix
  (REQ-ADP-001a-001 … REQ-ADP-001a-007) so the amendment relationship
  is legible in traceability.

  User-locked decisions baked in (recorded as Decision Points in §2.5):
  - **D1 OAuth flow + endpoints**: app-only `client_credentials`
    grant. NO user login (no authorization_code flow). Token obtained
    by `POST https://www.reddit.com/api/v1/access_token` with HTTP
    Basic auth (`client_id:client_secret`) and body
    `grant_type=client_credentials`. Bearer token presented against
    `https://oauth.reddit.com/search` (authenticated host is
    `oauth.reddit.com`, NOT `www.reddit.com`). Custom User-Agent
    remains mandatory on the token POST and every search GET.
    (Research §4.)
  - **D2 Credentials + graceful skip**: credentials via env vars
    `REDDIT_CLIENT_ID` and `REDDIT_CLIENT_SECRET`. When EITHER is
    absent, the adapter is gracefully SKIPPED at registration
    (mirroring GitHub's conditional registration at
    `cmd/usearch/query.go:476-487`) — NOT register-and-fail.
    `Capabilities.RequiresAuth = true`,
    `AuthEnvVars = ["REDDIT_CLIENT_ID","REDDIT_CLIENT_SECRET"]`.
    (Research §2.3, §3.)
  - **D3 Token lifecycle**: app-only tokens expire (`expires_in` ≈ 1
    hour). Token is cached in the adapter and refreshed on expiry and
    on a 401 response. The token endpoint base is overridable for
    tests via env `REDDIT_OAUTH_URL` (mirroring the existing
    `REDDIT_BASE_URL` search-endpoint override). (Research §4.3.)
  - **D4 Status categorization**: once authenticated, 403 means
    genuinely forbidden (private/banned) → keep `CategoryPermanent`;
    401 means token expired/invalid → trigger one refresh+retry, then
    `CategoryUnavailable` if still failing; 429 keeps the existing
    `CategoryRateLimited` + Retry-After logic. Authenticated rate
    limit is 60 req/min (vs 10 anon) → `RateLimitPerMin` updated to
    60 in `Capabilities`. (Research §4.4, §5.)

  Scope held tight to Reddit OAuth. The parent SPEC's other deferrals
  (authorization_code user-login flow, subreddit-scoped search,
  time-range filtering, sort customization, comment retrieval,
  live-network CI tests) remain OUT OF SCOPE — see §7.

  Note: `cmd/usearch-api/main.go`'s `see SPEC-IR-001` reference
  (`cmd/usearch-api/main.go:50`) is UNRELATED to this SPEC; it concerns
  the Intent Router HTTP server, not Reddit OAuth (research.md §8).
  ADP-001a does NOT touch `cmd/usearch-api/`.

  9 EARS REQ rows covering all five EARS patterns (the original
  overloaded REQ-006 was split into 006a/006b/006c per plan-audit M-1)
  + 3 NFRs. Zero new Go
  module dependencies anticipated (token POST via stdlib `net/http`).
  Harness level: standard (single package, ≤10 source files;
  contains a `secret`/`credentials` keyword which warrants careful
  review of secret-hygiene NFR but does not escalate to thorough).
  Ready for plan-auditor review and the annotation cycle.

---

## 1. Purpose

SPEC-ADP-001 published the Reddit reference adapter against the public
no-auth `search.json` endpoint with `Capabilities.RequiresAuth=false`.
That endpoint now returns HTTP 403 to anonymous callers
(research.md §0, §1.4), and the adapter's `categorizeStatus`
(`internal/adapters/reddit/client.go:102-124`) maps the generic
`4xx → CategoryPermanent`, so the fanout layer (SPEC-FAN-001) never
retries. Reddit search is dead.

SPEC-ADP-001a restores Reddit search by adding **app-only OAuth**
(`client_credentials` grant, also called "userless" or
"application-only"). The adapter authenticates with a per-deployment
Reddit app's `client_id` + `client_secret`, obtains a bearer token,
caches it, refreshes it on expiry and on 401, and issues
bearer-authenticated search requests against `oauth.reddit.com`.

This is an **amendment**, not a rewrite. It preserves the parent
SPEC's URL-parameter discipline, NSFW filter (REQ-ADP-007), JSON
parsing + field mapping (REQ-ADP-006), Tanh score normalization
(§2.3 of parent), Retry-After parsing
(`internal/adapters/reddit/errors.go:33-63`), redirect allowlist
(REQ-ADP-010), and query validation (REQ-ADP-008). It adds an auth
layer in front of the existing hot path and adjusts status
categorization for the authenticated regime (D4).

The Reddit adapter follows the same auth-gated shape already proven by
the GitHub adapter (`internal/adapters/github/github.go:46-84,137-161`)
and the Naver adapter (`internal/adapters/naver/naver.go:118-132`):
`RequiresAuth=true`, credential-bearing `Options`, constructor
validation with a `SkipAuthCheck` test seam, registry-level
`AuthEnvVars` validation (`internal/adapters/registry.go:147-166`),
and conditional registration that gracefully skips when credentials
are absent (`cmd/usearch/query.go:476-487`).

---

## 2. Scope

### 2.1 In-Scope

| Ref | Item |
|-----|------|
| a | `internal/adapters/reddit/reddit.go`: extend `Options` with `ClientID string`, `ClientSecret string`, `OAuthURL string` (test override for the token endpoint, default `https://www.reddit.com/api/v1/access_token`), and `SkipAuthCheck bool` (test seam mirroring github). Extend `New(opts Options) (*Adapter, error)` to return `*types.SourceError{Adapter:"reddit", Category: CategoryPermanent, Cause: ErrMissingCredentials}` when `(ClientID == "" || ClientSecret == "") && !SkipAuthCheck && HTTPClient == nil` (note the parentheses: the OR binds before the ANDs, mirroring the GitHub gate at `github.go:78` — an empty credential must NOT error when `SkipAuthCheck=true`). Update `Capabilities()`: `RequiresAuth=true`, `AuthEnvVars=["REDDIT_CLIENT_ID","REDDIT_CLIENT_SECRET"]`, `RateLimitPerMin=60`, and `Notes` rewritten to describe the OAuth endpoint, the 60/min authenticated limit, and the env vars. The `Adapter` struct gains an injected token-cache field (see item d). |
| b | `internal/adapters/reddit/oauth.go` (new file): app-only token acquisition. `acquireToken(ctx) (token string, expiry time.Time, err error)` POSTs to `a.oauthURL` with HTTP Basic auth (`client_id:client_secret`), form body `grant_type=client_credentials`, the custom User-Agent header, and `Accept: application/json`; parses the JSON `{access_token, token_type, expires_in}` response; computes `expiry = time.Now().Add(time.Duration(expires_in) * time.Second - safetyMargin)`. Maps token-endpoint failures: 401/403 on the token POST → `*SourceError{Category: CategoryPermanent, Cause: ErrTokenAcquisitionFailed}` (bad credentials are not retryable); 5xx/network → `CategoryUnavailable`. Secret material (client_id/secret/token) MUST NOT appear in any error or log (NFR-ADP-001a-002). |
| c | `internal/adapters/reddit/oauth.go`: a `tokenCache` type holding `mu sync.Mutex`, `token string`, `expiry time.Time`. Method `get(ctx, refreshFn) (string, error)` returns the cached token when unexpired, else calls `refreshFn` under the lock (or via a singleflight pattern to avoid a thundering herd of concurrent token POSTs — see NFR-ADP-001a-001). Method `invalidate()` clears the cached token (called on a 401 to force refresh on the next `get`). |
| d | `internal/adapters/reddit/search.go`: `Search` gains an auth preamble — obtain a valid bearer token from the cache (refreshing if needed), set `Authorization: bearer <token>`, and issue the request against `a.searchBaseURL` (now `https://oauth.reddit.com/search`, overridable via existing `Options.BaseURL`). On a 401 response, invalidate the token, refresh once, retry exactly once; if the retry still returns 401 → `*SourceError{Category: CategoryUnavailable, HTTPStatus: 401, Cause: ErrTokenRefreshExhausted}`. All other status handling reuses the existing `categorizeStatus` path (with the 401 carve-out from item f). |
| e | `internal/adapters/reddit/client.go`: `doRequest` (or a sibling helper) sets the `Authorization: bearer <token>` header in addition to the existing `User-Agent` and `Accept` headers. The custom User-Agent stays mandatory on every request (REQ-ADP-001a-006a). The redirect allowlist gains `oauth.reddit.com` (the new authenticated host) alongside the existing four reddit hosts. |
| f | `internal/adapters/reddit/client.go`: `categorizeStatus` carve-out — 401 is no longer swept into the generic `4xx → CategoryPermanent` branch. The Search layer intercepts 401 before categorization (item d). If a 401 ever reaches `categorizeStatus` after the refresh+retry is exhausted, it maps to `CategoryUnavailable`. 403 explicitly remains `CategoryPermanent`. |
| g | `internal/adapters/reddit/errors.go`: new sentinels `ErrMissingCredentials = errors.New("reddit: client credentials required; set REDDIT_CLIENT_ID and REDDIT_CLIENT_SECRET env vars")`, `ErrTokenAcquisitionFailed = errors.New("reddit: oauth token acquisition failed")`, `ErrTokenRefreshExhausted = errors.New("reddit: token refresh exhausted after 401 retry")`. Existing `ErrInvalidQuery` + `parseRetryAfter` preserved unchanged. |
| h | `internal/adapters/reddit/oauth_test.go` (new): token acquisition against an `httptest.Server` token-endpoint stub — happy path (parses access_token + expires_in), 401 token POST (bad creds → CategoryPermanent / ErrTokenAcquisitionFailed), 5xx token POST (CategoryUnavailable), Basic-auth header correctness (decode and assert `client_id:client_secret`), form body `grant_type=client_credentials`, custom User-Agent present on the token POST, secret-not-leaked assertion (error string contains no credential substrings). |
| i | `internal/adapters/reddit/search_test.go` (extend): authenticated happy path (token acquired then 25-doc search), 401-on-search triggers exactly one refresh + one retry (instrument both stubs with request counters), 401-twice exhausts to CategoryUnavailable / ErrTokenRefreshExhausted, 403 stays CategoryPermanent, `Authorization: bearer <token>` header present on the search GET, token reused across two sequential searches (token endpoint hit once), expired token triggers a single refresh. |
| j | `internal/adapters/reddit/reddit_test.go` (extend): `Capabilities()` now asserts `RequiresAuth=true`, `AuthEnvVars=["REDDIT_CLIENT_ID","REDDIT_CLIENT_SECRET"]`, `RateLimitPerMin=60`; `New` returns `ErrMissingCredentials` when either credential is empty and `SkipAuthCheck=false` and `HTTPClient=nil`; `New` succeeds with `SkipAuthCheck=true` (the seam existing stub tests use). |
| k | `cmd/usearch/query.go` (modify, ~lines 461-465): replace the unconditional Reddit registration block with a conditional one that reads `REDDIT_CLIENT_ID` + `REDDIT_CLIENT_SECRET`, registers ONLY when BOTH are non-empty (graceful skip otherwise), and passes them into `reddit.Options` along with the existing `BaseURL` and the new `OAuthURL` env override. Mirror the GitHub block's shape (`query.go:476-487`). |
| l | `internal/adapters/reddit/search_test.go` (extend): concurrency — `TestSearchConcurrentSafe` (the parent's 50-goroutine `-race` test) MUST continue to pass with the token cache in place; the token endpoint stub MUST be hit EXACTLY ONCE (`== 1`) across all 50 concurrent first-time searches, never 50 times (NFR-ADP-001a-001 / REQ-ADP-001a-003). |

### 2.2 Out-of-Scope

This SPEC explicitly excludes the following. Each has a known
destination; this list prevents scope creep beyond Reddit app-only
OAuth.

- **User-login OAuth (authorization_code grant)** — per-user Reddit
  accounts, scopes beyond app-only, refresh-token rotation, redirect
  URIs. ADP-001a is `client_credentials` only. A future SPEC may add
  user-login if a use case appears.
- **All non-Reddit adapters** (HN, arXiv, GitHub, YouTube, Bluesky,
  Naver, Daum, KNC, RSS, SearXNG, Polymarket) → their own ADP-* SPECs.
- **Retry orchestration** (exponential backoff, circuit breaker) →
  SPEC-FAN-001 (M3). The 401 refresh+retry in this SPEC is a single,
  in-adapter recovery for token expiry — NOT general retry.
- **Response caching** of search results → SPEC-CACHE-001 (M3). Only
  the OAuth TOKEN is cached here, not search responses.
- **Result ranking / RRF fusion** → SPEC-IDX-001 (M3).
- **Subreddit-scoped search, time-range filtering, sort
  customization, comment retrieval** → still deferred from the parent
  SPEC; out of this amendment's scope.
- **Live network integration tests in CI** → `httptest.Server` stubs
  for both the token endpoint and the search endpoint only. Optional
  env-gated live test deferred (parent SPEC D4).
- **Tenant-scoped / per-team Reddit app credentials** (multiple app
  registrations selected per request) → out of scope; a single
  process-wide `client_id`/`client_secret` pair from env.
- **Secret rotation at runtime** (re-reading env after process start)
  → out of scope; credentials are read once at registration.
- **Changes to `cmd/usearch-api/main.go`** — its `see SPEC-IR-001`
  reference is unrelated (research.md §8); ADP-001a touches only
  `cmd/usearch/query.go` in `cmd/`.

### 2.3 Relationship to Parent SPEC-ADP-001

ADP-001a is additive and amends specific behaviours:

| Parent behaviour | ADP-001a change |
|------------------|-----------------|
| `Capabilities.RequiresAuth=false`, `AuthEnvVars=nil` | → `true`, `["REDDIT_CLIENT_ID","REDDIT_CLIENT_SECRET"]` |
| `Capabilities.RateLimitPerMin=10` | → `60` (authenticated ceiling) |
| Search host `www.reddit.com/search.json` | → `oauth.reddit.com/search` (default; `Options.BaseURL` override preserved) |
| No Authorization header | → `Authorization: bearer <token>` on every search GET |
| 401 → CategoryPermanent (via generic 4xx) | → refresh+retry once, then CategoryUnavailable |
| 403 → CategoryPermanent | → unchanged (genuinely forbidden) |
| 429 → CategoryRateLimited + Retry-After | → unchanged |
| Unconditional registration (`query.go:461-465`) | → conditional, graceful skip when creds absent |
| No token state | → guarded token cache (`sync.Mutex`/singleflight) |

Preserved unchanged: REQ-ADP-002 URL params, REQ-ADP-006 field
mapping, REQ-ADP-007 NSFW filter, REQ-ADP-008 query validation,
REQ-ADP-009 custom User-Agent (now ALSO on the token POST),
REQ-ADP-010 redirect allowlist (extended with `oauth.reddit.com`),
REQ-ADP-011 concurrency safety (now also covering the token cache),
the Tanh score formula, and the parent's golden fixtures.

### 2.4 Decision Points (locked; do not re-litigate)

- **D1 — OAuth flow + endpoints**: app-only `client_credentials`
  grant; token POST to `https://www.reddit.com/api/v1/access_token`
  (HTTP Basic auth + `grant_type=client_credentials` body); bearer
  search against `https://oauth.reddit.com/search`; custom UA
  mandatory throughout. NO authorization_code user-login flow.
- **D2 — Credentials + graceful skip**: env vars `REDDIT_CLIENT_ID`
  and `REDDIT_CLIENT_SECRET`; skip registration gracefully when either
  is absent (mirror GitHub); `RequiresAuth=true`,
  `AuthEnvVars=["REDDIT_CLIENT_ID","REDDIT_CLIENT_SECRET"]`.
- **D3 — Token lifecycle**: cache token + expiry; refresh on expiry
  and on 401; token endpoint base overridable via `REDDIT_OAUTH_URL`
  (mirrors `REDDIT_BASE_URL`).
- **D4 — Status categorization**: authenticated 403 → CategoryPermanent
  (keep); 401 → one refresh+retry then CategoryUnavailable; 429 →
  CategoryRateLimited (keep); `RateLimitPerMin=60`.

---

## 3. EARS Requirements

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| REQ-ADP-001a-001 | Event-Driven | WHEN `(*Adapter).Search` is invoked and no valid cached bearer token exists, the adapter SHALL acquire one by issuing `POST <oauthURL>` (default `https://www.reddit.com/api/v1/access_token`) with HTTP Basic auth header derived from `client_id:client_secret`, a form body `grant_type=client_credentials`, the custom User-Agent header, and `Accept: application/json`; on HTTP 200 it SHALL parse `access_token` and `expires_in` and cache the token with its computed expiry; then it SHALL proceed with the bearer-authenticated search. | P0 | `TestAcquireTokenHappyPath` (stub returns `{access_token, expires_in:3600}`; assert cached token used); `TestAcquireTokenBasicAuthHeader` (decode `Authorization` → `client_id:client_secret`); `TestAcquireTokenFormBody` (assert `grant_type=client_credentials`); `TestAcquireTokenSetsUserAgent`. In `oauth_test.go`. |
| REQ-ADP-001a-002 | Event-Driven | WHEN a valid cached bearer token exists, the adapter SHALL issue the search request to `https://oauth.reddit.com/search` (overridable via `Options.BaseURL`) with the header `Authorization: bearer <token>`, and SHALL NOT acquire a new token. Query-parameter construction and response parsing remain identical to the parent SPEC (REQ-ADP-002, REQ-ADP-006). | P0 | `TestSearchUsesBearerToken` (assert `Authorization: bearer <tok>` on the search GET); `TestSearchReusesCachedToken` (two sequential searches → token endpoint hit exactly once); `TestSearchHostIsOAuthReddit` (default search host is `oauth.reddit.com`). In `search_test.go`. |
| REQ-ADP-001a-003 | State-Driven | WHILE the cached token is expired (`time.Now() ≥ cached expiry`) at the start of a `Search` call, the adapter SHALL refresh the token via REQ-ADP-001a-001 before issuing the search request. The refresh SHALL be concurrency-safe under the locked-refresh mechanism (`sync.Mutex` + double-checked expiry, or singleflight): under N concurrent first-time-or-expired Search calls sharing one `*Adapter`, the token endpoint SHALL be contacted EXACTLY ONCE (the first caller to win the lock acquires; all others observe the freshly-cached token after the lock releases — never one POST per goroutine). | P0 | `TestSearchRefreshesExpiredToken` (set cached expiry in the past → token endpoint hit again before search); `TestSearchConcurrentSafe` (parent's 50-goroutine `-race` test; assert token endpoint hit count `== 1`, search stub observes 50 requests, no race alarm). In `search_test.go`. |
| REQ-ADP-001a-004 | Unwanted | IF a search request returns HTTP 401 (token expired/revoked before nominal expiry), THEN the adapter SHALL invalidate the cached token, acquire a fresh token, and retry the search exactly ONCE; IF the retry also returns 401, THEN the adapter SHALL return `(nil, &types.SourceError{Adapter:"reddit", Category: types.CategoryUnavailable, HTTPStatus: 401, Cause: ErrTokenRefreshExhausted})` and SHALL NOT retry further. | P0 | `TestSearch401TriggersSingleRefreshRetry` (search stub returns 401 then 200; assert token endpoint hit twice, search stub hit twice, final result is 25 docs); `TestSearch401TwiceExhausts` (search stub returns 401 twice; assert `errors.Is(err, types.ErrSourceUnavailable)`, HTTPStatus=401, exactly 2 search attempts). In `search_test.go`. |
| REQ-ADP-001a-005 | Unwanted | IF a search request returns HTTP 403, THEN the adapter SHALL classify it as `CategoryPermanent` (genuinely forbidden: private, banned, or quarantined resource) and SHALL NOT refresh the token or retry. IF the token-acquisition POST itself returns HTTP 401 or 403 (invalid `client_id`/`client_secret`), THEN the adapter SHALL return `*SourceError{Category: CategoryPermanent, Cause: ErrTokenAcquisitionFailed}` and SHALL NOT retry. | P0 | `TestSearch403StaysPermanent` (search stub returns 403; assert `errors.Is(err, types.ErrPermanent)`, no token re-acquisition); `TestAcquireToken401BadCreds` (token stub returns 401; assert CategoryPermanent + `errors.Is(err, ErrTokenAcquisitionFailed)`); `TestAcquireToken403BadCreds`. In `search_test.go` + `oauth_test.go`. |
| REQ-ADP-001a-006a | Ubiquitous | The adapter SHALL set the custom User-Agent header `usearch/<version> (+https://github.com/elymas/universal-search)` AND the `Accept: application/json` header on EVERY outbound request — both the token-acquisition POST and every search GET. (Reddit rejects the default Go `net/http` User-Agent; the custom UA is a HARD precondition on both endpoints.) | P0 | `TestTokenPostSetsUserAgent` (token POST carries `usearch/...` UA + `Accept: application/json`); `TestSearchSetsCustomUserAgent` (search GET carries the same; preserved from parent). In `oauth_test.go` + `client_test.go`. |
| REQ-ADP-001a-006b | Event-Driven | WHEN `New(opts)` is invoked with `(opts.ClientID == "" \|\| opts.ClientSecret == "") && !opts.SkipAuthCheck && opts.HTTPClient == nil`, THEN it SHALL return `*types.SourceError{Adapter:"reddit", Category: CategoryPermanent, Cause: ErrMissingCredentials}` and SHALL NOT construct a usable adapter; AND WHEN the CLI registration site (`buildProductionRegistry`) receives that error, it SHALL skip `Register` so the Reddit adapter is gracefully absent from the registry (mirroring the GitHub conditional at `query.go:476-487`). | P0 | `TestNewMissingClientIDReturnsErr`, `TestNewMissingClientSecretReturnsErr` (assert `errors.Is(err, ErrMissingCredentials)`); `TestNewSkipAuthCheckSucceeds` (SkipAuthCheck=true → usable adapter, nil error); CLI test: both env vars unset → `reddit` absent from `buildProductionRegistry().List()`. In `reddit_test.go` + `cmd/usearch` test. |
| REQ-ADP-001a-006c | Ubiquitous | The adapter's `Capabilities()` SHALL report `RequiresAuth=true`, `AuthEnvVars=["REDDIT_CLIENT_ID","REDDIT_CLIENT_SECRET"]`, and `RateLimitPerMin=60`, and SHALL remain deterministic (two consecutive calls return equal values). The `Notes` field SHALL document the OAuth endpoint, the 60/min authenticated limit, and the two env-var names. | P0 | `TestCapabilitiesAuthShape` (RequiresAuth/AuthEnvVars/RateLimitPerMin); `TestCapabilitiesDeterministic` (preserved); `TestCapabilitiesNotesSubstrings` (`oauth.reddit.com`, `client_credentials`, `60/min`, both env names). In `reddit_test.go`. |
| REQ-ADP-001a-007 | Optional | WHERE the environment variable `REDDIT_OAUTH_URL` is set, the adapter SHALL use its value as the token-acquisition endpoint instead of the default `https://www.reddit.com/api/v1/access_token`; WHERE it is unset, the adapter SHALL use the default. This mirrors the existing `REDDIT_BASE_URL` override for the search endpoint and exists so CI can stub the token endpoint without live network. | P1 | `TestOAuthURLOverrideUsesEnvValue` (construct with `Options{OAuthURL: stub.URL}` → token POST hits the stub); `TestOAuthURLDefaultWhenUnset` (Options.OAuthURL empty → default endpoint chosen, verified via injected RoundTripper capture). In `oauth_test.go`. |

---

## 4. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| NFR-ADP-001a-001 | Token cache thread-safety | The token cache is the FIRST mutable state the Reddit adapter holds. It SHALL be guarded by a locked-refresh mechanism (`sync.Mutex` + double-checked expiry, or singleflight) such that, under N concurrent first-time-or-expired `Search` calls on a shared `*Adapter`, the token endpoint is contacted EXACTLY ONCE and all callers observe the same freshly-cached token — never one POST per goroutine, and never a check-then-act race that emits 2+ POSTs. Verified under `go test -race` by `TestSearchConcurrentSafe` (50 goroutines on a shared `*Adapter` and a single token-endpoint stub) with the token endpoint's observed request count asserted `== 1`. |
| NFR-ADP-001a-002 | No secret leakage | The `client_id`, `client_secret`, and bearer `access_token` SHALL NOT appear in any `*types.SourceError.Cause` string, any log line, or any panic message. Error causes use the fixed sentinels (`ErrMissingCredentials`, `ErrTokenAcquisitionFailed`, `ErrTokenRefreshExhausted`) without interpolating credential material. Verified by `TestErrorsDoNotLeakSecrets` (drive each error path with sentinel credentials like `client_id="SECRET_ID"`/`client_secret="SECRET_KEY"` and assert no error string contains those substrings). The adapter emits no logs itself (sole-emitter discipline preserved — the registry wrappedAdapter logs). |
| NFR-ADP-001a-003 | CI uses stubbed token endpoint | All OAuth tests SHALL run against `httptest.Server` stubs for BOTH the token endpoint (`Options.OAuthURL`) and the search endpoint (`Options.BaseURL`). NO live network call to `reddit.com` or `oauth.reddit.com` SHALL occur during `go test`. Verified by the absence of any non-loopback dial in the test suite and by all OAuth tests constructing the adapter with `SkipAuthCheck=true` + stub URLs. |

---

## 5. Acceptance Criteria

### REQ-ADP-001a-001 — Token Acquisition

- `TestAcquireTokenHappyPath`: token-endpoint stub returns
  `{"access_token":"tok123","token_type":"bearer","expires_in":3600}`;
  the subsequent search GET carries `Authorization: bearer tok123`.
- `TestAcquireTokenBasicAuthHeader`: the token POST's `Authorization`
  header decodes (base64, `Basic ` prefix) to `client_id:client_secret`.
- `TestAcquireTokenFormBody`: the token POST body contains
  `grant_type=client_credentials` (form-encoded).
- `TestAcquireTokenSetsUserAgent`: the token POST carries the custom
  `usearch/...` User-Agent and `Accept: application/json`.

### REQ-ADP-001a-002 — Bearer-Authenticated Search

- `TestSearchUsesBearerToken`: captured search GET has
  `Authorization: bearer <token>`.
- `TestSearchReusesCachedToken`: two sequential `Search` calls with a
  non-expired token → token endpoint observed exactly 1 request; both
  searches return 25 docs.
- `TestSearchHostIsOAuthReddit`: with default Options (no BaseURL
  override) the search target host is `oauth.reddit.com` (verified via
  injected RoundTripper capture).
- All preserved parent assertions (URL params, NSFW, parse, score)
  still pass against the authenticated path.

### REQ-ADP-001a-003 — Expired Token Refresh + Concurrency

- `TestSearchRefreshesExpiredToken`: with the cached expiry forced into
  the past, the next `Search` re-hits the token endpoint before the
  search GET.
- `TestSearchConcurrentSafe` (extended parent test): 50 goroutines
  call `Search` on one shared `*Adapter` against one token-endpoint
  stub + one search stub; under `-race` there are zero race alarms;
  the token endpoint is observed EXACTLY ONCE (assert `== 1` — the
  locked-refresh mechanism guarantees a single POST; any 2+ POSTs
  indicate a check-then-act race and FAIL the test); the search stub
  observes 50 requests; every goroutine receives 25 valid
  `NormalizedDoc`s.

### REQ-ADP-001a-004 — 401 Refresh + Single Retry

- `TestSearch401TriggersSingleRefreshRetry`: search stub returns 401
  on the first GET, 200 (25 docs) on the second; assert the token
  endpoint is hit twice (initial + refresh) and the search stub twice,
  and the final result is 25 docs.
- `TestSearch401TwiceExhausts`: search stub returns 401 on both
  attempts; assert `errors.Is(err, types.ErrSourceUnavailable)`,
  `HTTPStatus == 401`, `errors.Is(err, ErrTokenRefreshExhausted)`, and
  exactly 2 search attempts (no third).

### REQ-ADP-001a-005 — 403 Permanent + Bad-Credential Token Failure

- `TestSearch403StaysPermanent`: search stub returns 403; assert
  `errors.Is(err, types.ErrPermanent)`, `HTTPStatus == 403`, and the
  token endpoint is NOT re-hit (no refresh on 403).
- `TestAcquireToken401BadCreds`: token stub returns 401; assert
  `errors.Is(err, types.ErrPermanent)` and
  `errors.Is(err, ErrTokenAcquisitionFailed)`.
- `TestAcquireToken403BadCreds`: token stub returns 403; same
  assertions as the 401 bad-creds case.

### REQ-ADP-001a-006a — User-Agent + Accept on every request

- `TestTokenPostSetsUserAgent` + `TestSearchSetsCustomUserAgent`: both
  outbound requests (token POST and search GET) carry the custom
  `usearch/...` User-Agent and `Accept: application/json`.

### REQ-ADP-001a-006b — Credential gate + graceful registration skip

- `TestNewMissingClientIDReturnsErr` (ClientID="", ClientSecret set,
  SkipAuthCheck=false, HTTPClient=nil) → `errors.Is(err,
  ErrMissingCredentials)`; `TestNewMissingClientSecretReturnsErr`
  symmetric.
- `TestNewSkipAuthCheckSucceeds`: `New(Options{SkipAuthCheck:true,
  BaseURL: stub.URL})` returns a usable adapter with nil error.
- CLI registration: when either env var is unset, the Reddit adapter
  is absent from `buildProductionRegistry()`'s registry (verified by a
  `cmd/usearch` test that unsets both vars and asserts `reddit` is not
  in `reg.List()`).

### REQ-ADP-001a-006c — Capabilities auth shape

- `TestCapabilitiesAuthShape`: `Capabilities()` returns
  `RequiresAuth=true`, `AuthEnvVars=["REDDIT_CLIENT_ID",
  "REDDIT_CLIENT_SECRET"]`, `RateLimitPerMin=60`.
- `TestCapabilitiesNotesSubstrings`: `Notes` contains the substrings
  `"oauth.reddit.com"`, `"client_credentials"`, `"60/min"`, and the
  two env-var names.
- `TestCapabilitiesDeterministic` (preserved): two consecutive calls
  return equal values.

### REQ-ADP-001a-007 — Token Endpoint Override

- `TestOAuthURLOverrideUsesEnvValue`: `Options{OAuthURL: stub.URL,
  SkipAuthCheck:true}` → the token POST hits the stub.
- `TestOAuthURLDefaultWhenUnset`: empty `OAuthURL` → the adapter
  targets `https://www.reddit.com/api/v1/access_token` (verified via
  injected RoundTripper capture, no live call).

### NFR-ADP-001a-001 — Token Cache Thread-Safety

- `TestSearchConcurrentSafe` passes under `-race`; token endpoint
  request count is asserted `== 1` (exactly one POST under 50
  concurrent first-time callers).

### NFR-ADP-001a-002 — No Secret Leakage

- `TestErrorsDoNotLeakSecrets`: drive `ErrMissingCredentials`,
  `ErrTokenAcquisitionFailed`, `ErrTokenRefreshExhausted` paths with
  recognizable secret sentinels; assert no error string (recursively
  unwrapped) contains `"SECRET_ID"` or `"SECRET_KEY"` or the bearer
  token value.

### NFR-ADP-001a-003 — Stubbed Token Endpoint in CI

- Every OAuth test constructs the adapter with `SkipAuthCheck=true` +
  stub `OAuthURL` + stub `BaseURL`. No test performs a live dial to
  `reddit.com`/`oauth.reddit.com`. The existing parent tests continue
  to pass.

---

## 6. Technical Approach

### 6.1 Files to Modify

**Created**:
- `internal/adapters/reddit/oauth.go` — `acquireToken`, `tokenCache`
  type with `sync.Mutex` (or singleflight), `get`/`invalidate`.
- `internal/adapters/reddit/oauth_test.go` — token-endpoint stub
  tests.

**Modified**:
- `internal/adapters/reddit/reddit.go` — `Options` gains `ClientID`,
  `ClientSecret`, `OAuthURL`, `SkipAuthCheck`; `New` adds credential
  validation; `Adapter` struct gains `clientID`, `clientSecret`,
  `oauthURL`, `tokens *tokenCache` fields; `Capabilities` updated
  (RequiresAuth, AuthEnvVars, RateLimitPerMin, Notes); default search
  base URL becomes `https://oauth.reddit.com/search`.
- `internal/adapters/reddit/search.go` — auth preamble (obtain token),
  `Authorization` header, 401 refresh+retry-once loop.
- `internal/adapters/reddit/client.go` — `doRequest` sets
  `Authorization: bearer`; redirect allowlist adds `oauth.reddit.com`;
  `categorizeStatus` 401 carve-out.
- `internal/adapters/reddit/errors.go` — three new sentinels.
- `internal/adapters/reddit/reddit_test.go`,
  `internal/adapters/reddit/search_test.go` — extended tests.
- `cmd/usearch/query.go` (~461-465) — conditional Reddit
  registration mirroring GitHub (`query.go:476-487`).

**Unchanged (by design)**:
- `pkg/types/*` — no contract change; `Capabilities` already has
  `RequiresAuth`/`AuthEnvVars`/`RateLimitPerMin`
  (`pkg/types/capabilities.go:50-57`).
- `internal/adapters/registry.go` — already validates `AuthEnvVars`
  (`registry.go:147-166`); ADP-001a's new `AuthEnvVars` value is
  validated automatically.
- `internal/adapters/reddit/parse.go`, `score.go` — parsing + score
  normalization unchanged.
- `cmd/usearch-api/main.go` — UNRELATED (its `see SPEC-IR-001` note is
  the Intent Router server, research.md §8).

### 6.2 OAuth Flow Sketch (illustrative; final shapes in run phase)

```
Search(ctx, q):
  token, err := a.tokens.get(ctx, a.acquireToken)   // refresh if expired
  if err != nil { return nil, err }                  // token POST failed
  resp := a.doSearch(ctx, q, token)                  // Authorization: bearer
  if resp == 401:
      a.tokens.invalidate()
      token, err = a.tokens.get(ctx, a.acquireToken) // forced refresh
      if err != nil { return nil, err }
      resp = a.doSearch(ctx, q, token)               // single retry
      if resp == 401:
          return nil, SourceError{Unavailable, 401, ErrTokenRefreshExhausted}
  // 403 -> Permanent; 429 -> RateLimited; 5xx -> Unavailable; 200 -> parse
```

The `tokenCache.get` performs a double-checked-locking read (or
singleflight) so that 50 concurrent first-time callers do not issue 50
token POSTs (NFR-ADP-001a-001).

### 6.3 MX Tag Plan

| File::Symbol | Tag | Reason |
|--------------|-----|--------|
| `oauth.go::acquireToken` | `@MX:WARN` | Outbound network call carrying secret credentials. `@MX:REASON: leaking client_secret or token into error/log strings is a security incident; keep causes sentinel-only`. |
| `oauth.go::tokenCache.get` | `@MX:ANCHOR` | Shared mutable state guard; concurrency-correctness contract for all Search calls. `@MX:REASON: a broken lock here causes token-POST stampede and/or data races across all fanout goroutines`. |
| `client.go::categorizeStatus` | `@MX:NOTE` (update existing) | Document the new 401 carve-out (401 is refresh-recoverable, not Permanent). |
| `search.go::Search` | `@MX:ANCHOR` (update existing) | Note the auth preamble + 401 refresh-retry semantics added by ADP-001a. |

All tags `[AUTO]`-prefixed, include `@MX:SPEC: SPEC-ADP-001a`, in
English per `language.yaml` `code_comments: en`.

### 6.4 Harness Level

Single package (`internal/adapters/reddit`), ≤10 source files, one
`cmd/` edit. Contains `secret`/`credentials`/`auth` keywords, so the
secret-hygiene NFR (NFR-ADP-001a-002) warrants careful review, but the
change is local and well-precedented (GitHub/Naver adapters). Routes
to **standard** harness level; Sprint Contract optional. Evaluator
profile `default`.

---

## 7. What NOT to Build (Exclusions)

[HARD] This SPEC explicitly excludes the following. This list prevents
scope creep beyond Reddit app-only OAuth.

- **User-login OAuth (authorization_code grant)**, per-user accounts,
  refresh-token rotation, redirect URIs → out of scope; future SPEC if
  a use case appears. ADP-001a is `client_credentials` only.
- **All non-Reddit adapters** → their own ADP-* SPECs.
- **Retry orchestration** (exponential backoff, circuit breaker,
  health-state) → SPEC-FAN-001 (M3). The 401 refresh+retry here is a
  single in-adapter token-expiry recovery, not general retry.
- **Search-response caching** → SPEC-CACHE-001 (M3). Only the OAuth
  token is cached.
- **Result ranking / RRF fusion** → SPEC-IDX-001 (M3).
- **Subreddit-scoped search, time-range filtering, sort customization,
  comment retrieval** → still deferred from the parent SPEC.
- **Live network integration tests in CI** → stubs only.
- **Tenant-scoped / per-team Reddit app credentials** → out of scope;
  single process-wide credential pair from env.
- **Runtime secret rotation** (re-reading env after start) → out of
  scope; credentials read once at registration.
- **Changes to `cmd/usearch-api/main.go`** — its `see SPEC-IR-001`
  reference is unrelated (Intent Router server, research.md §8).
  ADP-001a touches only `cmd/usearch/query.go` under `cmd/`.

---

## 8. TDD Plan

Development follows RED-GREEN-REFACTOR per
`quality.development_mode: tdd`. This is a brownfield enhancement of an
existing package (parent shipped 55 tests + 1 benchmark, all green),
so each RED test is written informed by the existing behaviour
(workflow-modes.md "Brownfield Enhancement").

RED-GREEN-REFACTOR per requirement:
1. RED: write the failing OAuth test (token stub + search stub).
2. GREEN: implement minimal token acquisition/cache/refresh.
3. REFACTOR: extract the token-cache helper; keep `oauth.go` focused;
   keep each `.go` file < 200 LoC excluding tests.

Backward-compat gate (concrete, NOT incidental — plan-audit M-2): the
parent's existing Reddit suite is 59 `Test`/`Benchmark` functions
across 7 files. ~25+ of them construct the adapter via
`New(Options{BaseURL: ts.URL})` or `New(Options{})` — i.e. they use a
`BaseURL` stub, NOT an `HTTPClient`. Because the new credential gate
(REQ-ADP-001a-006b) only escapes when `HTTPClient != nil`, the
`HTTPClient == nil` escape hatch does NOT spare these tests: every one
of them will start failing the credential check and MUST gain
`SkipAuthCheck: true`. This is a deliberate, planned ~25-edit sweep —
the run phase must budget for it and the drift guard must not flag it
as unplanned scope. Affected construction sites (verified by the
audit):

- `reddit_test.go`: lines 14, 28, 37, 50, 118.
- `search_test.go`: lines 48, 81, 124, 150, 176, 199, 238, 267, 294,
  322, 361, 391, 424, 452, 479, 528, 557, 599, 667, 713.
- `client_test.go`: lines 126, 155, 181, 234, 260, 317.

Additionally (plan-audit m-6): changing the default search base URL
from `www.reddit.com/search.json` to `oauth.reddit.com/search` means
tests that rely on the DEFAULT base URL (no `BaseURL`, no
`HTTPClient`) — notably `TestHealthcheck` at `reddit_test.go:118`,
which sets only `HealthcheckTarget` — ALSO hit the credential gate and
need `SkipAuthCheck: true`. These default-host cases are a distinct
subset of the sweep, called out so they are not overlooked.

Representative new tests (grouped by REQ): token acquisition (happy /
Basic-auth / form-body / UA — REQ-001a-001, 006a), bearer search (uses
token / reuses cached / oauth host — REQ-001a-002), expired-token
refresh + concurrency exactly-once (`TestSearchConcurrentSafe`
extended — REQ-001a-003 / NFR-001a-001), 401 refresh+retry-once +
exhaustion (REQ-001a-004), 403 permanent + bad-credential token
failure (REQ-001a-005), credential gate + graceful registration skip
(REQ-001a-006b), Capabilities auth shape (REQ-001a-006c),
`REDDIT_OAUTH_URL` override (REQ-001a-007), secret-leak guard
(NFR-001a-002). Coverage target 85% per
`quality.test_coverage_target`.

---

## 9. Dependencies

### 9.1 Upstream

- **SPEC-ADP-001 (implemented; the parent)**: provides the entire
  Reddit adapter this SPEC amends. HARD dep.
- **SPEC-CORE-001 (implemented)**: `pkg/types.Adapter`,
  `Capabilities` (RequiresAuth/AuthEnvVars/RateLimitPerMin),
  `*types.SourceError` + Categories, registry `AuthEnvVars`
  validation. HARD dep (already satisfied).
- **SPEC-OBS-001 (implemented)**: `reqid.NewTransport` (already used
  by the Reddit client). SOFT dep.

### 9.2 Downstream

- **SPEC-FAN-001 (M3)**: continues to consume
  `registry.Get("reddit").Search`; the 401 refresh-retry is internal
  and invisible to fanout. No new constraint.

### 9.3 External (run-phase pins)

**Zero new Go module dependencies anticipated.** The token POST uses
stdlib `net/http` + `net/url` + `encoding/json` + `encoding/base64`
(or `http.Request.SetBasicAuth`). `sync` for the token-cache mutex.
The existing `go.uber.org/goleak` (test-only) is reused for the
cancellation NFR if needed. If the run-phase implementer prefers
`golang.org/x/sync/singleflight` for the refresh stampede guard, it is
already an indirect dependency in most Go projects — verify in `go.mod`
and add under SPEC-DEP-001 policy only if absent; a plain
`sync.Mutex` + double-checked expiry satisfies NFR-ADP-001a-001
without any new dependency.

---

## 10. Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Token POST stampede under concurrent first-time Search (50 goroutines → 50 token POSTs) | High | Medium | NFR-ADP-001a-001 mandates double-checked locking or singleflight; `TestSearchConcurrentSafe` asserts token endpoint hit `== 1`. |
| Secret (client_secret / token) leaks into an error or log | Medium | High | NFR-ADP-001a-002 + `TestErrorsDoNotLeakSecrets`; error causes use fixed sentinels, never interpolate credentials; adapter emits no logs itself. |
| Existing parent tests break on the new credential check | High | Medium | `SkipAuthCheck` seam (mirrors github); stub-based tests opt out of the check. Backward-compat gate in §8. |
| 401-vs-403 confusion regresses (treating expired token as Permanent, or forbidden as refreshable) | Medium | High | REQ-ADP-001a-004/005 + `categorizeStatus` 401 carve-out; explicit tests for both paths. |
| Reddit rotates/invalidates the app token faster than `expires_in` claims | Low | Medium | 401-on-search refresh path (REQ-ADP-001a-004) handles early invalidation regardless of cached expiry. |
| `oauth.reddit.com` redirect not in allowlist → SSRF guard rejects legitimate redirect | Low | Low | Add `oauth.reddit.com` to `allowedRedirectHosts` (`client.go:24-29`). |
| Token-endpoint base hard-coded → CI cannot stub | High (if unaddressed) | High | REQ-ADP-001a-007 + `Options.OAuthURL` / `REDDIT_OAUTH_URL` override; all tests use the stub. |

---

## 11. Open Questions

These do NOT block SPEC approval; each has a recommended default.

1. **Refresh stampede mechanism** (plain `sync.Mutex` +
   double-checked expiry vs `golang.org/x/sync/singleflight`).
   **Recommended default**: `sync.Mutex` + double-checked expiry —
   zero new dependency, satisfies NFR-ADP-001a-001 for the modest
   fanout sizes (≤ ~50 goroutines). **Resolution owner**: run-phase
   implementer.
2. **Token expiry safety margin** (refresh how early before nominal
   `expires_in`). **Recommended default**: 60s margin (refresh when
   `time.Now() ≥ expiry - 60s`) to avoid a search racing token
   expiry. **Resolution owner**: run-phase implementer.
3. **Behaviour when only the CLI is misconfigured but registry check
   passes** (e.g., env set but empty string). **Recommended default**:
   treat empty-string credential as absent (skip), consistent with
   `os.Getenv` returning "" for unset. **Resolution owner**:
   run-phase implementer.
4. **`tech.md` rate-limit row update** (parent SPEC's Open Question 1
   recommended correcting the Reddit row to "10/min unauth, 60/min
   OAuth"). Now that OAuth is the live path, the row should read
   "60/min OAuth (app-only)". **Resolution owner**: docs-sync agent in
   the next `/moai sync` after ADP-001a lands.

---

## 12. References

### External (URL-cited; verified per research.md §10)

- https://github.com/reddit-archive/reddit/wiki/OAuth2 — Reddit OAuth2
  flows (client_credentials / userless grant).
- https://github.com/reddit-archive/reddit/wiki/OAuth2-Quick-Start-Example
  — Basic-auth POST to `/api/v1/access_token`.
- https://www.reddit.com/dev/api — authenticated host
  `https://oauth.reddit.com`, `Authorization: bearer <token>`.
- RFC 6749 §4.4 — OAuth 2.0 Client Credentials Grant.
- RFC 7231 §7.1.3 — Retry-After (existing in `reddit/errors.go`).

### Internal (file:line cited)

- `.moai/specs/SPEC-ADP-001/spec.md` — parent SPEC (esp. `:39-43`,
  `:69-77`, `:253-256`, `:913-915` OAuth deferral + rate discrepancy).
- `.moai/specs/SPEC-ADP-001a/research.md` — full research artifact.
- `internal/adapters/reddit/reddit.go:17,33-49,63-91,99-118` — Options,
  New, Capabilities, endpoint, UA.
- `internal/adapters/reddit/client.go:24-29,71-75,102-124` — redirect
  allowlist, doRequest headers, categorizeStatus.
- `internal/adapters/reddit/search.go:43-110,113-125` — Search hot
  path + buildSearchURL.
- `internal/adapters/reddit/errors.go:16,33-63` — ErrInvalidQuery,
  parseRetryAfter.
- `internal/adapters/github/github.go:46-84,137-161` — auth-gated
  reference (Token, SkipAuthCheck, constructor validation,
  RequiresAuth Capabilities).
- `internal/adapters/github/errors.go:13-14` — ErrMissingToken
  sentinel.
- `internal/adapters/naver/naver.go:118-132,196-197` — dual-secret
  reference.
- `cmd/usearch/query.go:461-465,476-487` — Reddit (to be made
  conditional) + GitHub (reference) registration.
- `internal/adapters/registry.go:147-166` — registry AuthEnvVars
  validation.
- `pkg/types/capabilities.go:38-61` — Capabilities struct.
- `pkg/types/query.go:18-44` — Query + Filter.
- `cmd/usearch-api/main.go:2,50` — UNRELATED SPEC-IR-001 reference.
- `.moai/config/sections/quality.yaml` — `development_mode: tdd`,
  `test_coverage_target: 85`.
- `.moai/config/sections/language.yaml` — `code_comments: en`.

---

*End of SPEC-ADP-001a v0.1*
