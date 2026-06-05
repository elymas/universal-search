---
id: SPEC-ADP-006-XENABLE
title: X (Twitter) Adapter — Live Provider Enablement
version: 0.1.0
milestone: M3 — Fanout, adapters, index
status: draft
priority: P2
owner: expert-backend
methodology: tdd
coverage_target: 85
created: 2026-06-04
updated: 2026-06-04
author: limbowl
depends_on: [SPEC-ADP-006, SPEC-CORE-001, SPEC-IR-001, SPEC-OBS-001]
labels: [adapter, social, x-twitter, M3]
---

# SPEC-ADP-006-XENABLE: X (Twitter) Adapter — Live Provider Enablement

## HISTORY

- 2026-06-04 (iteration 2 — plan-auditor cycle 1, limbowl via manager-spec):
  Audit returned PASS-WITH-FIXES (0.83); both author-claimed drift-corrections
  and all three external-API claims were independently VERIFIED true via curl.
  Fixes applied inline: (D1, MAJOR) §2.3 score-formula prose corrected — the
  reused `score.go::normalizeScore` (`internal/adapters/social/score.go:24-28`)
  has NO input clamp `max(LikeCount+RepostCount, 0)`; the real function is
  `x := float64(likeCount + repostCount)` with the clamp applied to the OUTPUT
  value only. The invented input clamp is removed; the reused function is
  unchanged. (D4) `quote_count` collect/output asymmetry reconciled — promoted
  to a REQUIRED Metadata key in §6.4 and REQ-XEN-006 so collection (REQ-XEN-008)
  and output agree. (D2) internal citation line ranges tightened to
  `social.go:61-66,164-178,199-213` (±1 comment/declaration boundary). (D5)
  §6.4 `Title` policy made explicit (Title intentionally equals Snippet for X
  since tweets carry no separate headline). (D3, MP-3) frontmatter retains
  `created`/`updated` (NOT `created_at`) — this is the established house
  convention across all SPECs in this repo (e.g. SPEC-CLI-003, SPEC-UI-002
  carry the same accepted MP-3 deviation); making this lone SPEC use
  `created_at` would break repo-wide consistency. Keys unchanged by decision.

- 2026-06-04 (initial draft v0.1, limbowl via manager-spec):
  Successor SPEC reserved by name in the parent SPEC-ADP-006
  (`.moai/specs/SPEC-ADP-006/spec.md:83-84` "DEFERRED to a future
  SPEC-ADP-006-XENABLE", and §11.4 `spec.md:1224-1230` assigning the X
  provider open question to "future SPEC-ADP-006-XENABLE author"). Resolves
  GSD audit finding F-08 (`.planning/AUDIT-FINDINGS.md:22` — "X (Twitter)
  adapter is stub-only … `NewX` not registered in production registry.
  SPEC-ADP-006-XENABLE pending (social.go:176); needs provider + paid API").
  Scope and contracts derived from `.moai/specs/SPEC-ADP-006-XENABLE/research.md`
  (every external claim WebFetch-cited; every internal claim file:line-cited).
  Built on the parent's reserved X surface in `internal/adapters/social/`
  (`XOptions`/`NewX`/`searchX`/`xCapabilities`/`ErrXDisabled`/
  `ErrXProviderNotConfigured`), SPEC-CORE-001 (`pkg/types.Adapter`,
  `NormalizedDoc`, `*SourceError`, `Capabilities`), SPEC-IR-001
  (`Capabilities` consumer contract; `CategorySocial` covers X), and
  SPEC-OBS-001 (`AdapterCalls{adapter,outcome}` already carries the `"x"`
  label value; no allowlist amendment).

  Parent decisions this SPEC RESOLVES:

  - **Provider choice (parent Open Question §11.4)**: The parent assumed
    ScrapeCreators. Research §2.2 [VERIFIED] that ScrapeCreators has NO
    keyword-search endpoint for X — the assumption is incorrect for search.
    This SPEC does NOT hard-pick a single provider; it designs against an
    `XProvider` interface (research §4) so the concrete provider is
    pluggable. Two realistic concrete providers: (A) X official API v2
    `GET /2/tweets/search/recent` (pay-per-usage, ~$0.005/Post read,
    lowest ToS risk); (C) twitterapi.io `GET /twitter/tweet/advanced_search`
    (~$0.15/1000 tweets, higher ToS-risk class). Selection is a
    business / credentials / ToS decision documented as an EXTERNAL BLOCKER
    (§7, research §7).

  - **ToS gate (parent ToS risk row `spec.md:1190`, tech.md:147)**: When the
    selected provider is a third-party aggregator, an explicit
    ToS-acknowledgement gate at deployment time is REQUIRED. Option A
    (first-party official API) satisfies tech.md:147's "API-based adapters
    only" default.

  - **Env gate `USEARCH_X_ENABLED` (parent `spec.md:427,703,967`)**: retained
    verbatim. This SPEC adds the env-on + provider-configured → LIVE
    transition while preserving the parent's two-state disabled semantics
    (`ErrXDisabled` when env not "true"; `ErrXProviderNotConfigured` when env
    "true" but no provider).

  9 EARS REQs (5 × P0 + 4 × P2) covering all five EARS patterns (Ubiquitous,
  Event-Driven, State-Driven, Optional, Unwanted), 4 NFRs. Zero new Go module
  dependencies (stdlib + existing `pkg/types` + `internal/obs/reqid`; the
  concrete provider's HTTP client uses stdlib). [HARD] Test isolation inherits
  the parent's H1 mandate: NO `t.Setenv` under `-race` — env-dependent tests
  use `XOptions.EnvLookup` injection; provider behavior is exercised via an
  injected `fakeProvider` (no live network, no real credentials). Harness
  level: standard (single package `internal/adapters/social/` + one gated
  registration edit in `cmd/usearch/query.go`; the ToS-grey provider posture
  is contained behind the env + ToS-ack gates per tech.md:147).
  Ready for plan-auditor review and annotation cycle.

---

## 1. Purpose

SPEC-ADP-006 shipped the X (Twitter) sub-source **reserved-but-disabled**.
Its `searchX` path (`internal/adapters/social/search_x.go:23-31`) returns
`ErrXDisabled` when `USEARCH_X_ENABLED != "true"` and
`ErrXProviderNotConfigured` when the env gate is on but no provider is wired,
and `NewX` is NOT registered in production (`cmd/usearch/query.go:498-503`
registers Bluesky only, with the comment "X is stub-only — disabled until
provider configured"). Audit finding F-08 (`.planning/AUDIT-FINDINGS.md:22`)
records this gap and names this SPEC as the resolver.

This SPEC enables a LIVE X search path behind two gates:

1. The env gate `USEARCH_X_ENABLED=true` (the parent's opt-in mechanism).
2. A configured concrete provider (the parent's deferred decision).

It designs against an `XProvider` interface (research §4) so the concrete
backend is pluggable. The two realistic concrete providers researched
(research §2) are the X official API v2 and twitterapi.io; ScrapeCreators is
rejected for search (research §2.2). The live Search path turns a
`types.Query` into a provider HTTP call and normalizes the provider envelope
to `[]types.NormalizedDoc`, mirroring the Bluesky normalization already in
`internal/adapters/social/parse.go`.

The adapter does NOT do fanout (SPEC-FAN-001), retry (SPEC-FAN-001), caching
(SPEC-CACHE-001), ranking fusion (SPEC-IDX-001), or emit any metric/log/span
itself (the registry `wrappedAdapter` does, sole-emitter discipline,
`internal/adapters/registry.go:478-482`). It DOES one job: dispatch the X
sub-source through a configured provider when enabled, and preserve the
existing disabled semantics when not.

[HARD] **External-blocker precondition**: production *activation* of the live
path requires paid provider credentials AND a ToS-acknowledgement decision
(research §7). This SPEC specifies and tests the integration contract; it
cannot and does not activate the live path, because activation depends on
those external preconditions. The SPEC is fully implementable and testable
without credentials (via an injected `fakeProvider`).

---

## 2. Scope

### 2.1 In-Scope

| Ref | Item |
|-----|------|
| a | `internal/adapters/social/x_provider.go`: `XProvider` interface (`Name() string`, `SearchTweets(ctx, types.Query) (results []XTweet, nextCursor string, err error)`), the provider-neutral `XTweet` intermediate struct (research §4), and the extension of `XOptions` with a `Provider XProvider` field (the existing `EnvLookup` field is retained unchanged). |
| b | `internal/adapters/social/search_x.go`: extend `searchX` so that — WHEN `envLookup("USEARCH_X_ENABLED") != "true"` → `ErrXDisabled` (unchanged); WHEN env == `"true"` AND `a.xProvider == nil` → `ErrXProviderNotConfigured` (unchanged); WHEN env == `"true"` AND `a.xProvider != nil` → live path: call `a.xProvider.SearchTweets(ctx, q)`, normalize results to `[]NormalizedDoc` via `normalizeXTweets`, surface `nextCursor` on the last doc's `Metadata["next_cursor"]`, honour `ctx` cancellation. Empty query text (`isBlankQuery`) → `*SourceError{Adapter:"x", Category: CategoryPermanent, Cause: ErrInvalidQuery}` BEFORE invoking the provider. |
| c | `internal/adapters/social/social.go`: extend the `Adapter` struct with an unexported `xProvider XProvider` field; extend `NewX(opts XOptions)` to set `xProvider = opts.Provider` (nil when absent). `Name()` continues to return `"x"`. `Capabilities()` returns the LIVE descriptor (§6.3) when `a.xProvider != nil`, else the existing disabled descriptor (`xCapabilities()`). |
| d | `internal/adapters/social/social.go` `Healthcheck`: extend the `"x"` branch so that WHEN `a.xProvider == nil` → return `ErrXDisabled` (unchanged); WHEN `a.xProvider != nil` → probe the provider via a lightweight provider call (or a provider-exposed `Healthcheck`), mapping failure to a `*SourceError` per §6.4. |
| e | `internal/adapters/social/x_normalize.go`: `normalizeXTweets(tweets []XTweet, nextCursor string, retrievedAt time.Time) ([]types.NormalizedDoc, error)` — per-tweet transform per §6.5 field mapping. Sets `Hash=""`, `DocType=DocTypePost`, `SourceID="x"`, `RetrievedAt`, reuses `score.go::normalizeScore(likeCount, repostCount)` (parent Tanh formula, parent §2.3). Surfaces `next_cursor` on the LAST doc only when non-empty. |
| f | `internal/adapters/social/x_official.go` (Option A reference provider, OPTIONAL — at least one concrete provider OR a documented `fakeProvider`-only test path): `xOfficialProvider` implementing `XProvider` against `GET /2/tweets/search/recent` with Bearer Token auth, `tweet.fields=public_metrics,created_at`, `next_token` pagination. Constructed from `XOptions`/env credentials. Uses the existing `doRequest` + `categorizeStatus` helpers. |
| g | `cmd/usearch/query.go`: gated X registration in `buildProductionRegistry` (after the Bluesky block at `query.go:498-503`). WHEN `os.Getenv("USEARCH_X_ENABLED") == "true"` AND a provider is buildable from env credentials (AND, for a third-party provider, the ToS-ack gate is satisfied), construct `social.NewX(social.XOptions{Provider: prov})` and `reg.Register(a)`. WHEN env unset → no registration (status-quo, no behavior change). |
| h | `internal/adapters/social/search_x_test.go` (or extension of `search_test.go`): table-driven tests for the env-gate × provider-presence matrix, the live path against a `fakeProvider`, normalization, healthcheck, rate-limit handling, and backward-compat. All env-dependent tests inject `XOptions.EnvLookup` (NO `t.Setenv`). |
| i | `internal/adapters/social/x_normalize_test.go`: field-mapping unit tests over ≥4 `XTweet` fixtures (typical, empty author/URL-fallback, high engagement, zero engagement). Asserts each NormalizedDoc field per §6.5; `next_cursor` round-trip; `Hash==""`. |

### 2.2 Out-of-Scope

[HARD] This SPEC explicitly excludes the following. Each has a known
destination; this list prevents scope creep into XENABLE.

- **Production activation of the live path** (acquiring paid credentials,
  making the operator's provider+ToS decision) → EXTERNAL BLOCKER (§7). This
  SPEC ships the contract + tests; it does not turn on live X in any
  deployment.
- **ScrapeCreators provider** → REJECTED for search (research §2.2; no
  keyword-search endpoint).
- **Nitter provider** → operationally fragile (parent §11.4); not designed.
- **Full-archive search** (`GET /2/tweets/search/all`, advanced access) →
  v0 of XENABLE uses recent search only; full-archive deferred (research §9).
- **X App-only vs user-context OAuth flow management** (token rotation,
  refresh) → Option A reference provider uses a static Bearer Token from env;
  rotation is operator-side.
- **Lang inference for X posts** → `Lang=""` in v0; SPEC-IDX-003.
- **Retry orchestration / circuit breaking** → SPEC-FAN-001.
- **Response caching** → SPEC-CACHE-001.
- **Ranking / dedup / RRF fusion** → SPEC-IDX-001.
- **Per-adapter custom Prometheus metrics or new label values** → the `"x"`
  `adapter` label value already exists (parent REQ-ADP6-010); no allowlist
  amendment.
- **`outcome="disabled"` Prometheus label value** → disabled errors continue
  to map to `outcome="failure"` (parent §6.8).
- **Changes to the Bluesky sub-source** → untouched.
- **Live network integration tests in CI** → `fakeProvider` + fixtures only;
  optional env-gated live test (`-tags=integration` + real creds) deferred.

### 2.3 Score Normalisation (Inheritance from ADP-001 via parent)

[HARD] The X live path reuses `score.go::normalizeScore(likeCount,
repostCount int) float64` (`internal/adapters/social/score.go:24-28`)
verbatim — no modification to the reused function. The real function computes
`x := float64(likeCount + repostCount)` (NO input clamp on `x`) and returns
`math.Max(0.0, math.Min(1.0, scoreCenter + scoreCenter*math.Tanh(x/100.0)))`
— i.e. the clamp is applied to the OUTPUT value, not to `x`. Like/repost
counts are non-negative by provider contract, so the absence of an input
clamp has no runtime effect. Reply/quote counts surface in Metadata only. No
per-source recalibration in v0 (RRF in SPEC-IDX-001 weights rank, not raw
score).

---

## 3. EARS Requirements

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| REQ-XEN-001 | Ubiquitous | The package `internal/adapters/social` SHALL define an `XProvider` interface (`Name() string`, `SearchTweets(ctx, types.Query) ([]XTweet, string, error)`) and a provider-neutral `XTweet` struct. `XOptions` SHALL gain a `Provider XProvider` field; its existing `EnvLookup func(string) string` field SHALL be retained unchanged. `NewX(opts XOptions)` SHALL set the adapter's unexported `xProvider` to `opts.Provider` (nil when absent) and SHALL continue to return an `*Adapter` whose `Name()` equals `"x"` and which satisfies the compile-time assertion `var _ types.Adapter = (*Adapter)(nil)`. Construction SHALL NOT issue any HTTP request and SHALL NOT panic on a nil `Provider`. | P0 | `TestXProviderInterfaceShape` (compile-time `var _ XProvider = (*fakeProvider)(nil)`); `TestNewXAcceptsProvider` (`NewX(XOptions{Provider: fp})` → `Name()=="x"`, internal `xProvider` non-nil); `TestNewXNilProviderOK` (`NewX(XOptions{})` constructs without panic, `xProvider==nil`); `TestXStillImplementsAdapter` (compile-time `var _ types.Adapter`). In `search_x_test.go`. |
| REQ-XEN-002 | Event-Driven | WHEN `(*Adapter).Search(ctx, q)` is invoked on the X instance AND `envLookup("USEARCH_X_ENABLED") == "true"` AND `a.xProvider != nil` AND `q.Text` is non-empty, the adapter SHALL call `a.xProvider.SearchTweets(ctx, q)`, normalize the returned `[]XTweet` into `[]types.NormalizedDoc` via `normalizeXTweets` per §6.5, and return `(docs, nil)`. Each returned doc SHALL satisfy `NormalizedDoc.Validate() == nil` (ID, SourceID=`"x"`, URL, RetrievedAt populated). The provider's non-empty `nextCursor` SHALL be surfaced on the LAST returned doc's `Metadata["next_cursor"]`. | P0 | `TestSearchXLiveHappyPath` (`fakeProvider` returns 5 tweets; assert 5 valid NormalizedDocs, each `Validate()==nil`, SourceID=="x"); `TestSearchXLivePassesQueryToProvider` (assert `fakeProvider` received the same `q.Text`/`q.MaxResults`); `TestSearchXLiveSurfacesCursor` (`fakeProvider` nextCursor="c2" → last doc `Metadata["next_cursor"]=="c2"`, earlier docs lack the key). In `search_x_test.go`. |
| REQ-XEN-003 | State-Driven | WHILE `envLookup("USEARCH_X_ENABLED") != "true"`, the adapter SHALL return `(nil, ErrXDisabled)` for every `Search` call regardless of whether a `Provider` is configured, and SHALL NOT invoke `a.xProvider.SearchTweets`. WHILE `envLookup("USEARCH_X_ENABLED") == "true"` AND `a.xProvider == nil`, the adapter SHALL return `(nil, ErrXProviderNotConfigured)` and SHALL NOT issue any HTTP request. Both `ErrXDisabled` and `ErrXProviderNotConfigured` SHALL continue to satisfy `errors.Is(err, types.ErrPermanent)`. [HARD] All env-dependent acceptance tests SHALL drive env state via `XOptions.EnvLookup` injection and SHALL NOT use `os.Setenv`/`t.Setenv` (goroutine-unsafe under `-race`; parent H1). | P0 | `TestSearchXDisabledEvenWithProvider` (EnvLookup→`""`, Provider=fp → `errors.Is(err, ErrXDisabled)`; fp.SearchTweets call count == 0); `TestSearchXEnabledNilProvider` (EnvLookup→`"true"`, Provider=nil → `errors.Is(err, ErrXProviderNotConfigured)`; zero HTTP); `TestSearchXErrorsArePermanent` (both errors satisfy `errors.Is(err, types.ErrPermanent)`). In `search_x_test.go`. |
| REQ-XEN-004 | Unwanted | IF `q.Text` is empty OR contains only Unicode whitespace (per `unicode.IsSpace` over every rune), THEN the adapter SHALL return `(nil, &types.SourceError{Adapter:"x", Category: types.CategoryPermanent, Cause: ErrInvalidQuery})` BEFORE invoking `a.xProvider.SearchTweets`, even when the env gate is on and a provider is configured, and SHALL NOT issue any HTTP request. | P0 | `TestSearchXLiveEmptyQueryRejected` (EnvLookup→`"true"`, Provider=fp, table over `["", "   ", "\t\n  "]` for `q.Text`; for each: `errors.Is(err, types.ErrPermanent)` AND `errors.Is(err, ErrInvalidQuery)` AND fp.SearchTweets call count == 0). In `search_x_test.go`. |
| REQ-XEN-005 | Event-Driven | WHEN `a.xProvider.SearchTweets` returns a non-nil error, the adapter SHALL return that error unchanged when it is already a `*types.SourceError`, otherwise SHALL wrap it as `&types.SourceError{Adapter:"x", Category: <classified>, Cause: <err>}`. WHEN the provider signals a rate-limit (HTTP 429 with `Retry-After`), the concrete provider SHALL produce `*SourceError{Category: CategoryRateLimited, HTTPStatus:429, RetryAfter: parseRetryAfter(...)}` (reusing `errors.go::parseRetryAfter`). WHEN the provider signals auth failure (401/403), the result SHALL be `CategoryPermanent`. WHEN the provider signals 5xx / network failure, the result SHALL be `CategoryUnavailable` (`categorizeStatus(adapterName="x", ...)`). The adapter SHALL NOT retry internally. | P0 | `TestSearchXProviderErrorPropagated` (`fakeProvider` returns a `*SourceError{CategoryUnavailable}` → same error returned); `TestSearchXProviderRawErrorWrapped` (`fakeProvider` returns `errors.New("boom")` → result is `*SourceError{Adapter:"x"}` wrapping it); `TestXOfficialRateLimit` (Option A provider via `httptest.Server` 429 + `Retry-After: 30` → `CategoryRateLimited`, RetryAfter==30s); `TestXOfficialAuthFailure` (401 → `CategoryPermanent`); `TestXOfficialUnavailable` (503 → `CategoryUnavailable`). In `search_x_test.go` + provider test. |
| REQ-XEN-006 | Ubiquitous | The adapter SHALL transform each `XTweet` into one `types.NormalizedDoc` per the §6.5 mapping: `ID="x:"+tweet.ID`, `SourceID="x"`, `URL` from provider URL or constructed `"https://x.com/"+handle+"/status/"+id` (fallback `"https://x.com/i/status/"+id` when handle empty), `Title`/`Snippet`=first 280 runes of text, `Body`=full text, `Author`=handle, `Score=normalizeScore(LikeCount, RepostCount)`, `Lang=""`, `DocType=DocTypePost`, `Hash=""`, `RetrievedAt=time.Now().UTC()`. `Metadata` SHALL contain at minimum `{handle, tweet_id, like_count, repost_count, reply_count, quote_count, posted_at, sub_source(="x"), provider}` (the same `quote_count` that REQ-XEN-008 mandates the provider collect). | P0 | `TestNormalizeXTweetsFieldMapping` (table over 4 fixtures; assert every documented field); `TestNormalizeXTweetsURLFallback` (empty handle → URL is `https://x.com/i/status/<id>`); `TestNormalizeXTweetsHashEmpty` (every doc `Hash==""`); `TestNormalizeXTweetsMetadataKeys` (all 9 required keys present, `sub_source=="x"`); `TestNormalizeXTweetsScore` (Tanh values within ±0.001). In `x_normalize_test.go`. |
| REQ-XEN-007 | Event-Driven | WHEN `(*Adapter).Healthcheck(ctx)` is invoked on the X instance AND `a.xProvider == nil`, the adapter SHALL return `ErrXDisabled` (unchanged from parent). WHEN `a.xProvider != nil`, the adapter SHALL probe provider reachability and return nil on success or a `*types.SourceError` on failure; it SHALL honour `ctx` cancellation. | P2 | `TestXHealthcheckNilProvider` (Provider=nil → `errors.Is(err, ErrXDisabled)`); `TestXHealthcheckLive` (Provider=fp with healthy probe → nil); `TestXHealthcheckLiveFailure` (fp probe fails → non-nil `*SourceError`); `TestXHealthcheckCtxCancel` (cancelled ctx → error). In `search_x_test.go`. |
| REQ-XEN-008 | Optional | WHERE the selected concrete provider is the X official API (Option A), the provider SHALL include `tweet.fields=public_metrics,created_at` and SHALL map `public_metrics.like_count`/`retweet_count`/`reply_count`/`quote_count` into the corresponding `XTweet` fields. WHERE `q.MaxResults > 0`, the provider SHALL pass it as the upstream result cap (clamped to the provider's documented maximum); WHERE `q.MaxResults == 0`, the provider SHALL use the adapter default. WHERE `q.Cursor != ""`, the provider SHALL pass it as the upstream pagination token (`next_token` for Option A, `cursor` for Option C). | P2 | `TestXOfficialRequestFields` (inspect captured request URL via `httptest.Server`; assert `tweet.fields` contains `public_metrics` and `created_at`); `TestXOfficialPassesCursor` (q.Cursor="t1" → request has `next_token=t1`); `TestXOfficialMapsPublicMetrics` (fixture with public_metrics → XTweet LikeCount/RepostCount populated). In provider test. |
| REQ-XEN-009 | State-Driven | WHILE both the Bluesky and X (live, provider-configured) `*Adapter` instances are registered in the same registry and invoked concurrently from M caller goroutines, each `Search` call SHALL execute with no shared mutable state across the two instances (each holds its own `subSource`, `envLookup`, `xProvider`); the cumulative effect SHALL be race-clean under `go test -race`. The X live path SHALL hold no per-call mutable adapter state (the `XProvider` and its HTTP client are goroutine-safe by construction). [HARD] X-instance tests for this REQ SHALL inject env via `XOptions.EnvLookup` (NOT `t.Setenv`). | P0 | `TestSearchXLiveConcurrentSafe` (50 goroutines × 1 Search on a shared X instance with `EnvLookup→"true"` + a goroutine-safe `fakeProvider`; race-clean; each receives valid docs); `TestSearchBothSubSourcesLiveConcurrent` (one Bluesky instance + one live X instance in same registry; 50 caller goroutines invoke both; race-clean; no cross-pollination). In `search_x_test.go`. |

---

## 4. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| NFR-XEN-001 | Error-category parity | The X live path SHALL map provider failures to the same `*types.SourceError` Category taxonomy used by the Bluesky path: 429→`CategoryRateLimited` (with `RetryAfter`), 4xx→`CategoryPermanent`, 5xx/network→`CategoryUnavailable`, `ctx` deadline→`CategoryUnavailable`/transient per existing classification. Verified by `TestSearchXProviderErrorCategoryParity` (table over status codes through the Option A provider against `httptest.Server`, asserting identical Category outcomes to the Bluesky `categorizeStatus` truth table). No new `outcome` or `adapter` Prometheus label value is introduced (the `"x"` value already exists per parent REQ-ADP6-010). |
| NFR-XEN-002 | Secret & ToS handling | Provider credentials (Bearer Token / API key) SHALL be sourced from environment variables and SHALL NOT be logged, embedded in code, or written to any artifact. The live path SHALL activate ONLY when `USEARCH_X_ENABLED=true` AND a provider is configured; for a third-party provider, a ToS-acknowledgement gate SHALL additionally be required (research §7, tech.md:147). Verified by `TestXNoSecretInError` (a forced provider error message contains no token substring) and `TestXRegistrationGatedByEnv` (a unit covering `buildProductionRegistry` registers X only when the env gate is on). |
| NFR-XEN-003 | Test isolation (race-safe env) | [HARD] No test in the X path SHALL call `os.Setenv` or `t.Setenv`. All env state SHALL be driven via `XOptions.EnvLookup` injection; all provider behavior SHALL be driven via an injected `fakeProvider` or `httptest.Server`. Verified by `go test -race ./internal/adapters/social/...` passing with zero race-detector alarms attributable to the package, and by a grep-style assertion in review that the X test files contain no `t.Setenv`/`os.Setenv`. |
| NFR-XEN-004 | No goroutine leak on cancellation | The X live path SHALL NOT leak any goroutine when the caller's context is cancelled mid-`Search`. Verified by `TestSearchXLiveNoGoroutineLeakOnCancel` using `go.uber.org/goleak.VerifyNone(t)` after a `Search` whose ctx is cancelled mid-flight (provider HTTP call against an `httptest.Server` with a delayed response). The package `TestMain` (parent `bench_test.go`) already invokes `goleak.VerifyTestMain(m)`. |

---

## 5. Acceptance Criteria

### REQ-XEN-001 — Provider Interface + Extended Options

- `internal/adapters/social/x_provider.go` declares `XProvider`, `XTweet`,
  and the `Provider` field added to `XOptions`.
- The `Adapter` struct carries an unexported `xProvider XProvider` field.
- `NewX(XOptions{Provider: fp})` sets `xProvider` non-nil; `NewX(XOptions{})`
  leaves it nil without panic.
- Compile-time assertions `var _ types.Adapter = (*Adapter)(nil)` and
  `var _ XProvider = (*fakeProvider)(nil)` hold.
- `Name()` still returns `"x"`.

### REQ-XEN-002 — Live Search Happy Path

- `fakeProvider` returning N tweets → N `NormalizedDoc`s, each
  `Validate()==nil`, `SourceID=="x"`.
- The provider receives the unmodified `q.Text` / `q.MaxResults`.
- Non-empty `nextCursor` appears on the LAST doc's `Metadata["next_cursor"]`
  only.

### REQ-XEN-003 — Env-Gate × Provider Matrix (backward compat)

- env != "true" (even with a Provider) → `ErrXDisabled`; provider NOT called.
- env == "true" + nil provider → `ErrXProviderNotConfigured`; zero HTTP.
- Both errors satisfy `errors.Is(err, types.ErrPermanent)`.
- All driven via `XOptions.EnvLookup` (no `t.Setenv`).

### REQ-XEN-004 — Empty-Query Rejection (Unwanted)

- env on + provider configured + empty/whitespace `q.Text` →
  `ErrPermanent` wrapping `ErrInvalidQuery`; provider NOT called; zero HTTP.

### REQ-XEN-005 — Provider Error Mapping

- Provider `*SourceError` returned unchanged.
- Provider raw error wrapped in `*SourceError{Adapter:"x"}`.
- Option A provider: 429→RateLimited+RetryAfter, 401/403→Permanent,
  5xx/network→Unavailable. No internal retry.

### REQ-XEN-006 — XTweet → NormalizedDoc Mapping

- 4-fixture table asserts every documented field.
- URL fallback to `https://x.com/i/status/<id>` when handle empty.
- `Hash==""` on every doc.
- Required Metadata keys present (`handle`, `tweet_id`, `like_count`,
  `repost_count`, `reply_count`, `quote_count`, `posted_at`,
  `sub_source="x"`, `provider`).
- Score Tanh values within ±0.001.

### REQ-XEN-007 — Healthcheck

- nil provider → `ErrXDisabled`.
- live provider healthy → nil; unhealthy → `*SourceError`; cancelled ctx →
  error.

### REQ-XEN-008 — Option A Provider Request Shape

- Request URL includes `tweet.fields=public_metrics,created_at`.
- `q.Cursor` passes through as `next_token`.
- `public_metrics` maps into XTweet engagement fields.

### REQ-XEN-009 — Concurrent Safety

- `TestSearchXLiveConcurrentSafe`: 50 goroutines, race-clean, valid docs.
- `TestSearchBothSubSourcesLiveConcurrent`: Bluesky + live X in one registry,
  50 caller goroutines, race-clean, no cross-pollination.

### NFR-XEN-001 — Error-Category Parity

- Provider failures map to the same Category taxonomy as Bluesky; no new
  Prometheus label values.

### NFR-XEN-002 — Secret & ToS Handling

- Credentials from env only; never logged/embedded.
- Live activation requires env gate + provider (+ ToS-ack for third-party).
- `TestXNoSecretInError`, `TestXRegistrationGatedByEnv` pass.

### NFR-XEN-003 — Race-Safe Test Isolation

- `go test -race ./internal/adapters/social/...` clean.
- No `t.Setenv`/`os.Setenv` in X test files.

### NFR-XEN-004 — Goroutine Leak Check

- `TestSearchXLiveNoGoroutineLeakOnCancel` (`goleak.VerifyNone`) passes after
  mid-flight ctx cancel.

---

## 6. Technical Approach

### 6.1 Files to Modify / Create

**Created**:
- `internal/adapters/social/x_provider.go` — `XProvider`, `XTweet`,
  `XOptions.Provider`.
- `internal/adapters/social/x_normalize.go` — `normalizeXTweets`.
- `internal/adapters/social/x_official.go` — Option A reference provider
  (OPTIONAL if shipping `fakeProvider`-only; at least the interface + one
  concrete or a documented stub must exist for the live path to be testable).
- `internal/adapters/social/x_normalize_test.go` — mapping tests.
- `internal/adapters/social/search_x_test.go` (or extend `search_test.go`) —
  env×provider matrix, live path, concurrency, healthcheck, error mapping.

**Modified**:
- `internal/adapters/social/social.go` — add `xProvider` field; extend
  `NewX`, `Capabilities`, `Healthcheck` for the live branch.
- `internal/adapters/social/search_x.go` — extend `searchX` with the live
  branch (env on + provider present).
- `cmd/usearch/query.go` — gated X registration in
  `buildProductionRegistry` (after `query.go:498-503`).

**Unchanged (by design)**:
- `internal/adapters/social/search_bluesky.go`, `parse.go`, `url.go` —
  Bluesky path untouched.
- `internal/adapters/social/client.go` — `doRequest` / `categorizeStatus`
  reused as-is (both already accept `adapterName="x"`).
- `internal/adapters/social/errors.go` — `ErrXDisabled` /
  `ErrXProviderNotConfigured` / `parseRetryAfter` reused unchanged.
- `internal/adapters/social/score.go` — `normalizeScore` reused unchanged.
- `pkg/types/*` — no contract change.
- `internal/obs/metrics/metrics.go` — `"x"` label value already exists; no
  new family.

### 6.2 searchX Live Branch (illustrative; final shape in run phase)

```go
// search_x.go (extended)
func (a *Adapter) searchX(ctx context.Context, q types.Query) ([]types.NormalizedDoc, error) {
    if a.envLookup(xEnabledEnvVar) != "true" {
        return nil, ErrXDisabled
    }
    if a.xProvider == nil {
        return nil, ErrXProviderNotConfigured
    }
    if isBlankQuery(q.Text) {
        return nil, &types.SourceError{
            Adapter:  "x",
            Category: types.CategoryPermanent,
            Cause:    ErrInvalidQuery,
        }
    }
    tweets, nextCursor, err := a.xProvider.SearchTweets(ctx, q)
    if err != nil {
        var se *types.SourceError
        if errors.As(err, &se) {
            return nil, se
        }
        return nil, &types.SourceError{
            Adapter:  "x",
            Category: types.CategoryUnavailable,
            Cause:    err,
        }
    }
    return normalizeXTweets(tweets, nextCursor, time.Now().UTC())
}
```

### 6.3 X LIVE Capabilities Descriptor (when provider configured)

```go
types.Capabilities{
    SourceID:          "x",
    DisplayName:       "X (Twitter)",
    DocTypes:          []types.DocType{types.DocTypePost},
    SupportedLangs:    nil,
    SupportsSince:     false, // recent-search window in v0; since deferred
    RequiresAuth:      true,  // provider credentials required when live
    AuthEnvVars:       []string{/* provider-specific token env var */},
    RateLimitPerMin:   0,     // provider-dependent; 0 = unknown
    DefaultMaxResults: 25,
    Notes: "X (Twitter) social LIVE via configured XProvider. Enabled by " +
        "USEARCH_X_ENABLED=true + provider creds. Provider is pluggable " +
        "(X official API or twitterapi.io). ToS-grey third-party providers " +
        "require explicit ToS acknowledgement at deployment (tech.md:147).",
}
```

When `a.xProvider == nil`, `Capabilities()` returns the existing disabled
descriptor (`xCapabilities()`, parent `social.go:164-178`) unchanged.

### 6.4 X Tweet → NormalizedDoc Field Mapping

[NOTE on Title==Snippet] Tweets carry no separate headline field, so `Title`
and `Snippet` are INTENTIONALLY identical (both the first 280 runes of the
tweet text). This mirrors the Bluesky path (parent §6.5) where posts likewise
have no headline. Downstream consumers (SPEC-IDX-001 RRF / display) SHOULD
treat the X `Title` as a short-form preview, not a curated headline. A future
enhancement MAY set `Title=""` for X if display noise is observed; v0 keeps
the parent's symmetric posture for cross-adapter consistency.

| XTweet field | NormalizedDoc field | Transform |
|--------------|---------------------|-----------|
| `ID` | `ID` | `"x:" + ID` |
| (constant) | `SourceID` | `"x"` |
| `URL` / (constructed) | `URL` | provider URL, else `"https://x.com/"+handle+"/status/"+ID`, else `"https://x.com/i/status/"+ID` |
| `truncateRunes(Text,280)` | `Title` | first 280 runes (intentionally == Snippet; see note above) |
| `Text` | `Body` | as-is |
| `truncateRunes(Text,280)` | `Snippet` | first 280 runes |
| `CreatedAt` (parsed) | `PublishedAt` | best-effort parse → UTC; zero on error |
| `CreatedAt` | `Metadata["posted_at"]` | original string |
| (parse time) | `RetrievedAt` | `time.Now().UTC()` |
| `AuthorHandle` | `Author` | as-is |
| `normalizeScore(LikeCount, RepostCount)` | `Score` | Tanh formula |
| (constant) | `Lang` | `""` in v0 |
| (constant) | `DocType` | `types.DocTypePost` |
| (constant) | `Hash` | `""` |
| (constructed) | `Metadata` | REQUIRED: `handle`, `tweet_id`, `like_count`, `repost_count`, `reply_count`, `quote_count`, `posted_at`, `sub_source`(=`"x"`), `provider`. LAST doc adds `next_cursor` when non-empty. (`quote_count` is REQUIRED here to match REQ-XEN-008's collection mandate — collection and output agree.) |

### 6.5 Registration Gating (cmd/usearch/query.go)

Inserted after the Bluesky block (`query.go:498-503`), mirroring the
GitHub/YouTube env-gated patterns (`query.go:476-494`):

```go
// X live (gated): USEARCH_X_ENABLED=true AND a buildable provider.
if os.Getenv("USEARCH_X_ENABLED") == "true" {
    if prov, ok := buildXProvider(); ok {
        if a, err := social.NewX(social.XOptions{Provider: prov}); err == nil {
            _ = reg.Register(a)
        }
    }
}
```

`buildXProvider()` reads provider-specific credential env vars (and, for a
third-party provider, the ToS-ack gate) and returns `(XProvider, true)` only
when fully configured. When env unset → block is skipped entirely (status
quo; no behavior change).

### 6.6 Observability Note

The live X path emits ZERO metrics/logs/spans itself. All observability comes
from the registry `wrappedAdapter` (`internal/adapters/registry.go:478-482`),
which emits under `adapter="x"` (already in SPEC-OBS-001's allowlist per
parent REQ-ADP6-010). `outcome` values: `"success"` (live 200),
`"rate_limited"` (429), `"unavailable"` (5xx/network), `"failure"` (4xx,
disabled errors). No new label value introduced.

### 6.7 MX Tag Plan

| File::Symbol | Tag | Reason |
|--------------|-----|--------|
| `search_x.go::(*Adapter).searchX` | `@MX:ANCHOR` | Sole entry for the X sub-source; live + disabled dispatch; fan_in ≥ 3 (registry wrappedAdapter, FAN-001, tests). `@MX:REASON: env+provider gating; changing the branch order changes disabled/live semantics`. `@MX:SPEC: SPEC-ADP-006-XENABLE`. |
| `x_provider.go::XProvider` | `@MX:ANCHOR` | Public seam for pluggable provider backends. `@MX:REASON: interface contract; every concrete provider + the adapter normalization depend on its shape`. `@MX:SPEC: SPEC-ADP-006-XENABLE`. |
| `x_normalize.go::normalizeXTweets` | `@MX:NOTE` | Every X doc passes through this single transform; NormalizedDoc field-mapping integrity gate for X. |
| `x_official.go::(*xOfficialProvider).SearchTweets` | `@MX:WARN` | Outbound network call carrying a Bearer Token. `@MX:REASON: credential handling + metered cost ($0.005/Post); do not log the token`. `@MX:SPEC: SPEC-ADP-006-XENABLE`. |
| `social.go::Adapter.xProvider` field | `@MX:NOTE` | The live/disabled discriminator for X. Nil = disabled; non-nil = live. |
| `cmd/usearch/query.go::buildProductionRegistry` (X block) | `@MX:WARN` | ToS + secret gate for a ToS-grey source. `@MX:REASON: registering X without the env + ToS-ack gates violates tech.md:147`. `@MX:SPEC: SPEC-ADP-006-XENABLE`. |

All tags `[AUTO]`-prefixed, `code_comments: en`, within per-file limits
(`.moai/config/sections/mx.yaml`: 3 ANCHOR + 5 WARN).

### 6.8 Harness Level

Per `.moai/config/sections/harness.yaml` auto-routing: 9 EARS REQs
(5 × P0 + 4 × P2) + 4 NFRs touching ONE package
(`internal/adapters/social/`) plus ONE gated registration edit in
`cmd/usearch/query.go` + reused env var (no new config file). The ToS-grey
provider posture is contained behind the env + ToS-ack gates (tech.md:147
honoured; no provider activated in CI) = **standard** harness level. Sprint
Contract OPTIONAL. Evaluator profile `default`.

---

## 7. Exclusions (What NOT to Build)

[HARD] This SPEC explicitly excludes the following. Each has a known
destination; this list prevents scope creep.

- **EXTERNAL BLOCKER — production activation**: acquiring paid provider
  credentials and making the operator's provider + ToS-acknowledgement
  decision are OUTSIDE this SPEC. The SPEC ships the integration contract and
  tests (driven by `fakeProvider`/`httptest.Server`); it does NOT turn on
  live X in any deployment. Activation requires (research §7): (1) paid X
  Developer credits (Option A) OR a funded twitterapi.io key (Option C) —
  neither has a usable free tier for search; and (2) a ToS-acknowledgement
  decision per tech.md:147 (mandatory when the provider is third-party).
- **ScrapeCreators provider** → REJECTED for search (research §2.2; no
  keyword-search endpoint for X).
- **Nitter provider** → operationally fragile; not designed.
- **X full-archive search** (`GET /2/tweets/search/all`, advanced access) →
  v0 uses recent search only.
- **OAuth token rotation / refresh management** → static env Bearer Token;
  rotation is operator-side.
- **Lang inference for X posts** → `Lang=""`; SPEC-IDX-003.
- **Retry orchestration / circuit breaking** → SPEC-FAN-001.
- **Response caching** → SPEC-CACHE-001.
- **Ranking / dedup / RRF fusion** → SPEC-IDX-001.
- **Changes to the Bluesky sub-source** → untouched.
- **New Prometheus metric families or label values** → `adapter="x"` already
  exists (parent REQ-ADP6-010).
- **`outcome="disabled"` label value** → disabled errors stay
  `outcome="failure"`.
- **Live network integration tests in CI** → `fakeProvider` + fixtures only.

---

## 8. TDD Plan

Development follows RED-GREEN-REFACTOR per `quality.development_mode: tdd`.
Representative RED-phase tests, written before implementation, grouped by REQ.
Coverage target 85% per `quality.test_coverage_target`. Benchmarks do not
count toward coverage.

| # | Test | Layer | REQ | Assertion |
|---|------|-------|-----|-----------|
| 1 | `TestXProviderInterfaceShape` | `search_x_test.go` | REQ-XEN-001 | compile-time `var _ XProvider = (*fakeProvider)(nil)` |
| 2 | `TestNewXAcceptsProvider` | `search_x_test.go` | REQ-XEN-001 | `NewX(XOptions{Provider: fp})` → `Name()=="x"`, `xProvider` non-nil |
| 3 | `TestNewXNilProviderOK` | `search_x_test.go` | REQ-XEN-001 | `NewX(XOptions{})` no panic; `xProvider==nil` |
| 4 | `TestXStillImplementsAdapter` | `search_x_test.go` | REQ-XEN-001 | compile-time `var _ types.Adapter = (*Adapter)(nil)` |
| 5 | `TestSearchXLiveHappyPath` | `search_x_test.go` | REQ-XEN-002 | 5 tweets → 5 valid docs, SourceID=="x" |
| 6 | `TestSearchXLivePassesQueryToProvider` | `search_x_test.go` | REQ-XEN-002 | provider received same `q.Text`/`q.MaxResults` |
| 7 | `TestSearchXLiveSurfacesCursor` | `search_x_test.go` | REQ-XEN-002 | last doc `next_cursor`; earlier docs lack key |
| 8 | `TestSearchXDisabledEvenWithProvider` | `search_x_test.go` | REQ-XEN-003 | EnvLookup→`""` + provider → `ErrXDisabled`; provider call count 0 |
| 9 | `TestSearchXEnabledNilProvider` | `search_x_test.go` | REQ-XEN-003 | EnvLookup→`"true"` + nil → `ErrXProviderNotConfigured`; zero HTTP |
| 10 | `TestSearchXErrorsArePermanent` | `search_x_test.go` | REQ-XEN-003 | both errors `errors.Is(err, types.ErrPermanent)` |
| 11 | `TestSearchXLiveEmptyQueryRejected` | `search_x_test.go` | REQ-XEN-004 | table empty/whitespace → `ErrInvalidQuery`; provider call count 0 |
| 12 | `TestSearchXProviderErrorPropagated` | `search_x_test.go` | REQ-XEN-005 | provider `*SourceError` returned unchanged |
| 13 | `TestSearchXProviderRawErrorWrapped` | `search_x_test.go` | REQ-XEN-005 | raw error → `*SourceError{Adapter:"x"}` |
| 14 | `TestXOfficialRateLimit` | provider test | REQ-XEN-005 | 429+`Retry-After:30` → RateLimited, RetryAfter==30s |
| 15 | `TestXOfficialAuthFailure` | provider test | REQ-XEN-005 | 401 → CategoryPermanent |
| 16 | `TestXOfficialUnavailable` | provider test | REQ-XEN-005 | 503 → CategoryUnavailable |
| 17 | `TestNormalizeXTweetsFieldMapping` | `x_normalize_test.go` | REQ-XEN-006 | 4-fixture table; every documented field |
| 18 | `TestNormalizeXTweetsURLFallback` | `x_normalize_test.go` | REQ-XEN-006 | empty handle → `x.com/i/status/<id>` |
| 19 | `TestNormalizeXTweetsHashEmpty` | `x_normalize_test.go` | REQ-XEN-006 | every doc `Hash==""` |
| 20 | `TestNormalizeXTweetsMetadataKeys` | `x_normalize_test.go` | REQ-XEN-006 | 9 required keys; `sub_source=="x"` |
| 21 | `TestNormalizeXTweetsScore` | `x_normalize_test.go` | REQ-XEN-006 | Tanh within ±0.001 |
| 22 | `TestXHealthcheckNilProvider` | `search_x_test.go` | REQ-XEN-007 | nil provider → `ErrXDisabled` |
| 23 | `TestXHealthcheckLive` | `search_x_test.go` | REQ-XEN-007 | healthy provider → nil |
| 24 | `TestXHealthcheckLiveFailure` | `search_x_test.go` | REQ-XEN-007 | failing probe → `*SourceError` |
| 25 | `TestXHealthcheckCtxCancel` | `search_x_test.go` | REQ-XEN-007 | cancelled ctx → error |
| 26 | `TestXOfficialRequestFields` | provider test | REQ-XEN-008 | URL has `tweet.fields=public_metrics,created_at` |
| 27 | `TestXOfficialPassesCursor` | provider test | REQ-XEN-008 | q.Cursor="t1" → `next_token=t1` |
| 28 | `TestXOfficialMapsPublicMetrics` | provider test | REQ-XEN-008 | public_metrics → XTweet counts |
| 29 | `TestSearchXLiveConcurrentSafe` | `search_x_test.go` | REQ-XEN-009, NFR-XEN-003 | 50 goroutines; race-clean; valid docs |
| 30 | `TestSearchBothSubSourcesLiveConcurrent` | `search_x_test.go` | REQ-XEN-009, NFR-XEN-003 | Bluesky + live X; 50 goroutines; race-clean |
| 31 | `TestSearchXProviderErrorCategoryParity` | provider test | NFR-XEN-001 | status-code table → same Category as Bluesky |
| 32 | `TestXNoSecretInError` | provider test | NFR-XEN-002 | forced error message has no token substring |
| 33 | `TestXRegistrationGatedByEnv` | `cmd/usearch` test | NFR-XEN-002 | X registered only when env gate on |
| 34 | `TestSearchXLiveNoGoroutineLeakOnCancel` | `search_x_test.go` | NFR-XEN-004 | `goleak.VerifyNone` after mid-flight cancel |

RED-GREEN-REFACTOR per requirement. Brownfield note: the X stub already
exists (`search_x.go`, `social.go`); the live branch is added test-first
while preserving the disabled-path behavior (characterized by existing parent
tests, which must continue to pass).

---

## 9. Dependencies

### 9.1 Upstream

- **SPEC-ADP-006 (implemented)**: provides the reserved X surface
  (`XOptions`/`NewX`/`searchX`/`xCapabilities`/`ErrXDisabled`/
  `ErrXProviderNotConfigured`), the Bluesky live-path template, and the
  shared `doRequest`/`categorizeStatus`/`parseRetryAfter`/`normalizeScore`
  helpers. HARD dep.
- **SPEC-CORE-001 (implemented)**: `pkg/types.Adapter`, `Capabilities`,
  `Query`, `NormalizedDoc`, `*SourceError`, `DocType`. HARD dep.
- **SPEC-IR-001 (implemented)**: `Capabilities` consumer contract;
  `CategorySocial` covers X. SOFT dep.
- **SPEC-OBS-001 (implemented)**: `AdapterCalls{adapter,outcome}`; `"x"`
  label value already present. SOFT dep.

### 9.2 Downstream

- **SPEC-FAN-001 (approved)**: consumes `registry.Get("x").Search`; once a
  live provider is configured, `Result.AdapterErrors["x"]` may carry live
  failures instead of the disabled sentinel.
- **SPEC-IDX-001 (M3)**: consumes `NormalizedDoc.Score` (Tanh) for RRF.

### 9.3 External (run-phase pins)

**Zero new Go module dependencies.** Stdlib (`context`, `encoding/json`,
`errors`, `fmt`, `io`, `net/http`, `net/url`, `os`, `strconv`, `strings`,
`time`, `unicode`, `unicode/utf8`, `math`) + `pkg/types` + `internal/obs/reqid`
+ test-only `go.uber.org/goleak` (already pinned). The concrete provider's
HTTP client uses stdlib only.

**External services (activation-time, NOT module deps)**: X official API v2
(research §2.1) OR twitterapi.io (research §2.3) — both paid; both are the
EXTERNAL BLOCKER of §7.

---

## 10. Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Provider choice changes post-approval | Medium | Low | `XProvider` interface makes the concrete backend pluggable; swap is a new file, not a contract change. |
| Official API metered-cost blowout | Medium | High | `q.MaxResults` clamp + `DefaultMaxResults` bound per-call reads; budget is an operator decision. |
| Third-party (twitterapi.io) ToS-risk class | High | High | ToS-ack env gate required for third-party providers (NFR-XEN-002, tech.md:147); default deployment ships no X provider. |
| Backward-compat regression (env-unset behavior changes) | Low | High | REQ-XEN-003 asserts env-unset still returns `ErrXDisabled`; registration only added inside the `=="true"` branch; existing parent tests must stay green. |
| `t.Setenv` race under `-race` | Medium | Medium | NFR-XEN-003 mandates `XOptions.EnvLookup` injection; no `t.Setenv`/`os.Setenv` in X tests. |
| Provider timestamp-format variance | High | Low | Best-effort parse; `PublishedAt` zero on error (`Validate` does not require it). |
| Author handle absent (official API needs expansions join) | Medium | Low | URL falls back to `x.com/i/status/<id>`; `Author=""` permitted. |
| Secret leakage in logs/errors | Low | High | NFR-XEN-002 + `TestXNoSecretInError`; adapter emits nothing itself (sole-emitter). |
| Goroutine leak on ctx cancel mid-provider-call | Low | High | NFR-XEN-004 + existing `goleak.VerifyTestMain`. |

---

## 11. Open Questions

These are UNRESOLVED at SPEC-approval time; each has a recommended default and
does NOT block approval (full discussion in research §9).

1. **Default concrete provider when multiple creds present**. Recommended:
   prefer Option A (official, lowest ToS risk) when its creds exist; Option C
   only behind explicit ToS-ack. Owner: run-phase implementer + operator.
2. **ToS-ack mechanism (env var name)**. Recommended: `USEARCH_X_TOS_ACK=true`,
   required only for third-party providers. Owner: this SPEC's run phase.
3. **Recent vs full-archive search (Option A)**. Recommended: recent search
   (7-day window) in v0; full-archive deferred. Owner: run-phase implementer.
4. **Lang inference for X posts**. Recommended: `Lang=""` in v0. Owner:
   SPEC-IDX-003 author.

---

## 12. References

### External (URL-cited; WebFetch-verified per research.md §10)

- https://docs.x.com/x-api/posts/recent-search — `GET /2/tweets/search/recent`.
- https://docs.x.com/x-api/posts/search/integrate/build-a-query — query
  operators; recent vs full-archive limits.
- https://docs.x.com/x-api/fundamentals/metrics — `public_metrics` fields.
- https://docs.x.com/x-api/getting-started/pricing — pay-per-usage credit
  model ($0.005/Post read).
- https://docs.scrapecreators.com/v1/twitter — no X keyword-search endpoint
  (rejected).
- https://docs.twitterapi.io/api-reference/endpoint/tweet_advanced_search —
  `GET /twitter/tweet/advanced_search`.
- https://twitterapi.io/pricing — 15 credits/tweet = $0.15/1000 tweets.

### Internal (file:line cited)

- `.moai/specs/SPEC-ADP-006-XENABLE/research.md` — full research artifact.
- `.moai/specs/SPEC-ADP-006/spec.md:83-84` — parent deferral by name.
- `.moai/specs/SPEC-ADP-006/spec.md:1224-1230` — parent provider open
  question (this SPEC's author assigned).
- `.moai/specs/SPEC-ADP-006/spec.md:1190` — parent ToS risk row.
- `.moai/specs/SPEC-ADP-006/spec.md:264` — sentinel definitions.
- `internal/adapters/social/social.go:61-66,110-124,164-178,181-194,199-213`
  — `XOptions`, `NewX`, `xCapabilities`, `Search`, `Healthcheck`.
- `internal/adapters/social/search_x.go:12,23-31` — `searchX`,
  `xEnabledEnvVar`.
- `internal/adapters/social/search_bluesky.go:36-122` — live-path template.
- `internal/adapters/social/client.go:62-66,91-111` — `doRequest`,
  `categorizeStatus`.
- `internal/adapters/social/errors.go:19-46,61-91` — sentinels,
  `parseRetryAfter`.
- `internal/adapters/social/score.go` — `normalizeScore`.
- `internal/adapters/social/parse.go` — Bluesky normalization template.
- `cmd/usearch/query.go:476-503` — `buildProductionRegistry` env-gated
  registration patterns.
- `pkg/types/capabilities.go:38-62` — `Capabilities`.
- `pkg/types/errors.go:19-120` — sentinels, `Category`, `*SourceError`.
- `pkg/types/query.go` — `Query`, `Filter`.
- `pkg/types/normalized_doc.go:40-78` — `NormalizedDoc`, `Validate`.
- `internal/adapters/registry.go:136,478-482` — `Register`, `wrappedAdapter`.
- `.moai/project/tech.md:107,147` — X row, ToS feature-flag mandate.
- `.planning/AUDIT-FINDINGS.md:22` — F-08.
- `.moai/config/sections/quality.yaml` — `development_mode: tdd`,
  `test_coverage_target: 85`.
- `.moai/config/sections/language.yaml` — `documentation: en`,
  `code_comments: en`.

---

*End of SPEC-ADP-006-XENABLE v0.1 (DRAFT)*
