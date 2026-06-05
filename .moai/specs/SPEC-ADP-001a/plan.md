# SPEC-ADP-001a Implementation Plan

Amendment to SPEC-ADP-001: add Reddit app-only OAuth
(`client_credentials` grant) to restore Reddit search after the public
anonymous endpoint started returning HTTP 403.

Methodology: TDD (RED-GREEN-REFACTOR), brownfield enhancement of the
existing `internal/adapters/reddit` package.

---

## Technical Approach

Add an authentication layer in front of the existing Reddit Search hot
path without rewriting the parsing, scoring, NSFW-filter, or
URL-building logic the parent SPEC shipped. The new surface is:

1. A token cache (`sync.Mutex` + double-checked expiry) holding the
   bearer token and its expiry.
2. A token acquisition function that POSTs to Reddit's
   `/api/v1/access_token` with HTTP Basic auth and
   `grant_type=client_credentials`.
3. An auth preamble in `Search` that fetches/refreshes the token, sets
   the `Authorization: bearer <token>` header, targets
   `oauth.reddit.com`, and handles a 401 with one refresh+retry.
4. Credential-gated construction + conditional CLI registration
   mirroring the GitHub adapter.

The design mirrors two existing auth-gated adapters already in the
codebase (GitHub at `internal/adapters/github/`, Naver at
`internal/adapters/naver/`), so there is strong precedent for every
piece.

---

## Milestones (priority-ordered, no time estimates)

### Milestone 1 — Credential plumbing + Capabilities (Priority High)

- Extend `reddit.Options` with `ClientID`, `ClientSecret`, `OAuthURL`,
  `SkipAuthCheck`.
- Add credential validation to `New` returning `ErrMissingCredentials`
  (covers REQ-ADP-001a-006b).
- Add the three error sentinels to `errors.go`.
- Update `Capabilities()`: `RequiresAuth=true`,
  `AuthEnvVars=["REDDIT_CLIENT_ID","REDDIT_CLIENT_SECRET"]`,
  `RateLimitPerMin=60`, rewritten `Notes`.
- RED tests: `TestNewMissingClientIDReturnsErr`,
  `TestNewMissingClientSecretReturnsErr`, `TestNewSkipAuthCheckSucceeds`,
  `TestCapabilitiesAuthShape`.
- Gate: existing parent `reddit_test.go` assertions updated for the new
  Capabilities shape; all other parent tests still green.

### Milestone 2 — Token acquisition (Priority High)

- New `oauth.go`: `acquireToken(ctx)` (Basic auth, form body, custom
  UA, JSON parse, expiry computation; bad-cred 401/403 → Permanent /
  `ErrTokenAcquisitionFailed`; 5xx/network → Unavailable).
- New `oauth_test.go`: token-endpoint stub tests
  (REQ-ADP-001a-001, REQ-ADP-001a-005 token-side, REQ-ADP-001a-007
  override).
- Depends on Milestone 1 (Options fields, sentinels).

### Milestone 3 — Token cache + concurrency safety (Priority High)

- `tokenCache` type with `get(ctx, refreshFn)` / `invalidate()`,
  guarded by `sync.Mutex` + double-checked expiry (or singleflight).
- Wire the cache into the `Adapter` struct.
- RED/concurrency test: extend `TestSearchConcurrentSafe` to assert the
  token endpoint is hit EXACTLY ONCE (`== 1`) under 50 concurrent
  first-time searches, race-clean (NFR-ADP-001a-001,
  REQ-ADP-001a-003). The `== 1` bound is guaranteed by the
  double-checked-locking refresh; a regression to 2+ POSTs fails the
  test.
- Depends on Milestone 2.

### Milestone 4 — Bearer-authenticated search + 401 handling (Priority High)

- `search.go` auth preamble: obtain token, set `Authorization: bearer`,
  target `oauth.reddit.com` (default search base URL change).
- `client.go`: `doRequest` sets the Authorization header; add
  `oauth.reddit.com` to `allowedRedirectHosts`; `categorizeStatus`
  401 carve-out.
- 401 refresh+retry-once loop in `Search`; exhaustion →
  `CategoryUnavailable` / `ErrTokenRefreshExhausted`.
- 403 stays `CategoryPermanent` (no refresh).
- RED tests: `TestSearchUsesBearerToken`, `TestSearchReusesCachedToken`,
  `TestSearchHostIsOAuthReddit`, `TestSearch401TriggersSingleRefreshRetry`,
  `TestSearch401TwiceExhausts`, `TestSearch403StaysPermanent`
  (REQ-ADP-001a-002/004/005).
- Depends on Milestones 2 and 3.

### Milestone 5 — Secret hygiene (Priority High)

- `TestErrorsDoNotLeakSecrets`: drive every error path with sentinel
  credentials; assert no error string contains them
  (NFR-ADP-001a-002).
- Audit all `*SourceError.Cause` constructions for credential
  interpolation; confirm sentinel-only causes.

### Milestone 6 — Conditional CLI registration (Priority Medium)

- `cmd/usearch/query.go` (~461-465): replace the unconditional Reddit
  block with a conditional one reading `REDDIT_CLIENT_ID` +
  `REDDIT_CLIENT_SECRET`, passing `OAuthURL` from `REDDIT_OAUTH_URL`,
  registering only when both creds are present. Mirror the GitHub
  block (`query.go:476-487`).
- Test (in `cmd/usearch`): with both vars unset, `reddit` is absent
  from `buildProductionRegistry().List()`.
- Depends on Milestones 1-4 (the constructor signature must be stable).

### Milestone 7 — Backward-compat sweep + REFACTOR (Priority Medium)

This is a concrete ~25-edit sweep, NOT an incidental cleanup
(plan-audit M-2). The parent suite is 59 `Test`/`Benchmark` functions
across 7 files; ~25+ construct the adapter via `New(Options{BaseURL:
ts.URL})` or `New(Options{})` (a `BaseURL` stub, NOT an `HTTPClient`),
so the `HTTPClient == nil` escape in the new credential gate
(REQ-ADP-001a-006b) does NOT spare them — each MUST gain
`SkipAuthCheck: true`.

- Add `SkipAuthCheck: true` to every `BaseURL`-stub construction site:
  - `reddit_test.go`: lines 14, 28, 37, 50, 118 (118 = `TestHealthcheck`,
    a default-host case per m-6 — no `BaseURL`, sets only
    `HealthcheckTarget`, still hits the gate).
  - `search_test.go`: lines 48, 81, 124, 150, 176, 199, 238, 267, 294,
    322, 361, 391, 424, 452, 479, 528, 557, 599, 667, 713.
  - `client_test.go`: lines 126, 155, 181, 234, 260, 317.
- Also add `SkipAuthCheck: true` to any test relying on the changed
  DEFAULT base URL (`www.reddit.com/search.json` →
  `oauth.reddit.com/search`) with no `BaseURL` override (m-6 subset).
- Run the full parent suite (59 `Test`/`Benchmark` funcs + the new
  OAuth tests) green under `-race`.
- REFACTOR: keep `oauth.go` < 200 LoC; extract shared header-setting
  if it reduces duplication; verify `go vet` + `gofmt` clean.
- Update MX tags (oauth.go WARN/ANCHOR, search.go/client.go ANCHOR/NOTE
  updates) per spec §6.3.
- Drift-guard note: this sweep touches ~25 lines across 3 existing test
  files; it is PLANNED scope, not drift — the run phase must not flag
  it.

---

## File Ownership (single-domain; sub-agent mode)

All work is in one package (`internal/adapters/reddit`) plus one
`cmd/usearch` edit — no parallel file-conflict risk. Sub-agent mode is
appropriate (not team mode).

Created: `oauth.go`, `oauth_test.go`.
Modified: `reddit.go`, `search.go`, `client.go`, `errors.go`,
`reddit_test.go`, `search_test.go`, `cmd/usearch/query.go`.
Untouched: `parse.go`, `score.go`, `testdata/*`, `cmd/usearch-api/*`,
`pkg/types/*`, `internal/adapters/registry.go`.

---

## Risks (see spec §10 for full table)

Top three:
1. Token-POST stampede under concurrency → double-checked locking /
   singleflight (NFR-ADP-001a-001).
2. Secret leakage into errors/logs → sentinel-only causes
   (NFR-ADP-001a-002).
3. Parent tests breaking on the credential check → `SkipAuthCheck`
   seam.

---

## Validation Strategy

- `go test -race ./internal/adapters/reddit/...` (concurrency + all
  unit tests, including parent suite).
- `go test ./cmd/usearch/...` (conditional registration).
- `go vet ./...`, `gofmt -l`, `golangci-lint run` per `go.md` MUST
  rules.
- Coverage ≥ 85% for `internal/adapters/reddit`
  (`quality.test_coverage_target`).
- No live network: confirmed by stub-only `OAuthURL` + `BaseURL` in
  every OAuth test (NFR-ADP-001a-003).

---

## Delegation

Single-domain backend adapter work → `expert-backend` subagent (per
spec `owner: expert-backend`). The run phase is invoked via
`/moai run SPEC-ADP-001a` after the annotation cycle approves this
plan.
