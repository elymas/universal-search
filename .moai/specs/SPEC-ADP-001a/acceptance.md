# SPEC-ADP-001a Acceptance Criteria

Given-When-Then acceptance scenarios for Reddit app-only OAuth. All
scenarios use `httptest.Server` stubs for both the token endpoint
(`Options.OAuthURL`) and the search endpoint (`Options.BaseURL`). No
live network calls (NFR-ADP-001a-003).

---

## Scenario 1 — Token acquisition on first search (REQ-ADP-001a-001)

- **Given** a Reddit adapter constructed with `ClientID="cid"`,
  `ClientSecret="csecret"`, a stub token endpoint that returns
  `{"access_token":"tok123","token_type":"bearer","expires_in":3600}`,
  and a stub search endpoint that returns the 25-doc fixture,
- **When** `Search(ctx, q)` is called with no cached token,
- **Then** the adapter POSTs to the token endpoint with
  `Authorization: Basic base64("cid:csecret")`, body
  `grant_type=client_credentials`, the custom `usearch/...`
  User-Agent, and `Accept: application/json`,
- **And** the subsequent search GET carries `Authorization: bearer tok123`,
- **And** the call returns 25 valid `NormalizedDoc`s.

## Scenario 2 — Cached token reuse (REQ-ADP-001a-002)

- **Given** an adapter that has already acquired a non-expired token,
- **When** `Search` is called twice in sequence,
- **Then** the token endpoint is contacted exactly once (the second
  search reuses the cached token),
- **And** both searches target host `oauth.reddit.com` (default) and
  carry the bearer header.

## Scenario 3 — Expired token refresh (REQ-ADP-001a-003)

- **Given** an adapter whose cached token expiry is in the past,
- **When** `Search` is called,
- **Then** the adapter re-acquires a token from the token endpoint
  before issuing the search GET,
- **And** the search succeeds with the fresh token.

## Scenario 4 — 401 on search triggers single refresh + retry (REQ-ADP-001a-004)

- **Given** a stub search endpoint that returns HTTP 401 on the first
  GET and HTTP 200 (25 docs) on the second,
- **When** `Search` is called with a (stale) cached token,
- **Then** the adapter invalidates the token, re-acquires a fresh one,
  and retries the search exactly once,
- **And** the token endpoint is hit twice (initial + refresh), the
  search endpoint twice,
- **And** the call returns 25 docs.

## Scenario 5 — 401 twice exhausts to Unavailable (REQ-ADP-001a-004)

- **Given** a stub search endpoint that returns HTTP 401 on both the
  initial GET and the post-refresh retry,
- **When** `Search` is called,
- **Then** the adapter returns
  `*types.SourceError{Category: CategoryUnavailable, HTTPStatus: 401,
  Cause: ErrTokenRefreshExhausted}`,
- **And** `errors.Is(err, types.ErrSourceUnavailable)` is true,
- **And** exactly 2 search attempts were made (no third).

## Scenario 6 — 403 stays Permanent, no refresh (REQ-ADP-001a-005)

- **Given** a stub search endpoint that returns HTTP 403,
- **When** `Search` is called with a valid cached token,
- **Then** the adapter returns `CategoryPermanent` with `HTTPStatus=403`,
- **And** `errors.Is(err, types.ErrPermanent)` is true,
- **And** the token endpoint is NOT re-contacted (403 is not treated as
  a token problem).

## Scenario 7 — Bad credentials at token endpoint (REQ-ADP-001a-005)

- **Given** a stub token endpoint that returns HTTP 401 (or 403),
- **When** `Search` triggers token acquisition,
- **Then** the adapter returns `CategoryPermanent` with
  `errors.Is(err, ErrTokenAcquisitionFailed)`,
- **And** no search GET is issued.

## Scenario 8 — Graceful skip when credentials absent (REQ-ADP-001a-006b)

- **Given** `REDDIT_CLIENT_ID` or `REDDIT_CLIENT_SECRET` is unset,
- **When** `buildProductionRegistry()` runs,
- **Then** `New` returns `errors.Is(err, ErrMissingCredentials)` for
  the missing-credential case (with `SkipAuthCheck=false`,
  `HTTPClient=nil`),
- **And** the CLI registration block skips `Register`,
- **And** `reddit` is NOT present in `reg.List()`.

## Scenario 9 — Capabilities advertise auth (REQ-ADP-001a-006c)

- **Given** any constructed Reddit adapter,
- **When** `Capabilities()` is called,
- **Then** it returns `RequiresAuth=true`,
  `AuthEnvVars=["REDDIT_CLIENT_ID","REDDIT_CLIENT_SECRET"]`,
  `RateLimitPerMin=60`,
- **And** `Notes` contains `"oauth.reddit.com"`,
  `"client_credentials"`, `"60/min"`, and both env-var names,
- **And** two consecutive calls return equal values (determinism
  preserved).

## Scenario 10 — Token endpoint override for CI (REQ-ADP-001a-007)

- **Given** `Options.OAuthURL` set to a stub URL (or `REDDIT_OAUTH_URL`
  env set),
- **When** token acquisition runs,
- **Then** the POST targets the stub, not the default
  `https://www.reddit.com/api/v1/access_token`,
- **And** with `OAuthURL` empty the adapter targets the default
  endpoint (verified via injected RoundTripper capture, no live call).

## Scenario 11 — Existing parent suite stays green (backward compat, M-2)

- **Given** the parent SPEC-ADP-001 Reddit test suite (59
  `Test`/`Benchmark` functions across `reddit_test.go`,
  `search_test.go`, `client_test.go`, `parse_test.go`, `score_test.go`,
  `bench_test.go`),
- **When** the ~25+ `BaseURL`-stub construction sites (enumerated in
  spec.md §8 / plan.md Milestone 7) have `SkipAuthCheck: true` added
  and the new Capabilities-shape assertions are updated,
- **Then** `go test -race ./internal/adapters/reddit/...` passes with
  ZERO failures across both the parent suite and the new OAuth tests,
- **And** no parent test is deleted or skipped to achieve green (only
  `SkipAuthCheck` and the updated Capabilities assertions change).

---

## Edge Cases

- **Empty/whitespace query**: REQ-ADP-008 (parent) still applies —
  rejected before any token acquisition or HTTP request.
- **Concurrent first-time searches (50 goroutines)**: token endpoint
  contacted EXACTLY ONCE (`== 1`), never 50 times and never 2+ (a
  check-then-act race would emit 2+ and FAIL); race detector clean
  (NFR-ADP-001a-001).
- **Custom User-Agent on token POST**: mandatory; a missing UA would be
  rejected by Reddit just like the search path (REQ-ADP-001a-006a).
- **Token expiring mid-flight between cache check and search**: covered
  by the 401 refresh+retry path (Scenario 4), independent of cached
  expiry.
- **NSFW filter, pagination cursor, score normalization**: unchanged
  from parent; verified against the authenticated path.

---

## Quality Gate Criteria

- `go test -race ./internal/adapters/reddit/...` passes (new OAuth
  tests + all 59 parent `Test`/`Benchmark` funcs).
- `go test ./cmd/usearch/...` passes (conditional registration).
- `go vet ./...` clean; `gofmt -l` reports no files; `golangci-lint
  run` clean.
- Coverage ≥ 85% for `internal/adapters/reddit`.
- No live network dial during tests (stub-only token + search
  endpoints).
- No secret material in any error/log string (`TestErrorsDoNotLeakSecrets`).
- MX tags updated per spec §6.3 with `@MX:SPEC: SPEC-ADP-001a`.

---

## Definition of Done

- [ ] All 9 EARS REQ rows (REQ-ADP-001a-001 … 005, 006a, 006b, 006c,
      007) implemented and tested.
- [ ] All 3 NFRs (NFR-ADP-001a-001 … 003) verified by passing tests.
- [ ] All 11 acceptance scenarios pass.
- [ ] Parent SPEC-ADP-001 test suite (59 funcs) remains green —
      `SkipAuthCheck:true` added to the ~25+ enumerated `BaseURL`-stub
      sites; no parent test deleted or skipped (Scenario 11, M-2).
- [ ] Conditional CLI registration mirrors the GitHub pattern; Reddit
      gracefully skipped when credentials absent.
- [ ] 401 vs 403 categorization correct (refresh-recoverable vs
      permanent).
- [ ] Token cache concurrency-safe; token endpoint hit EXACTLY ONCE
      (`== 1`) under 50 concurrent first-time searches, `-race` clean.
- [ ] No secret leakage; sole-emitter observability discipline
      preserved.
- [ ] `cmd/usearch-api/main.go` untouched (unrelated SPEC-IR-001
      reference).
- [ ] TRUST 5 quality gates passed; coverage ≥ 85%.
