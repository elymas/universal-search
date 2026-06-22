# Unimplemented SPEC — Implementation Requirements

Reviewed 2026-06-05. Covers the two remaining audit gaps: F-08 (X) and F-09 (Facebook+Threads).
Both SPECs are `draft`, plan-audited **PASS-WITH-FIXES** (X 0.83, Meta 0.90), with all external-API
claims independently verified live (curl) by plan-auditor. Code citations all accurate.

## Key framing — two layers per SPEC

1. **CODE layer** — interface, normalization, registry gating, tests. NO external blocker. Pure Go,
   **0 new dependencies**, fully testable now with `fakeProvider` / `httptest.Server` / golden fixtures.
   → `/moai run` ready.
2. **ACTIVATION layer** — turning the live path ON. Blocked by money / OAuth / Meta app review / ToS-ack.
   Cannot be satisfied by code alone; requires human decisions + external accounts.

---

## F-08 — SPEC-ADP-006-XENABLE (X / Twitter) [P2]

### Code (run-ready, no blocker)

- New `internal/adapters/social/x_provider.go`: `XProvider` interface (`Name()`, `SearchTweets(ctx, Query) ([]XTweet, nextCursor, err)`) + provider-neutral `XTweet` struct; extend `XOptions` with `Provider XProvider` (keep `EnvLookup`).
- Extend `search_x.go` (live path + 2-state disabled preserved), `social.go` (`xProvider` field, `NewX`, live Capabilities, Healthcheck), `cmd/usearch/query.go` (gated registration after Bluesky block).
- `normalizeXTweets` → `[]NormalizedDoc`, `Metadata["next_cursor"]`, error-category parity with Bluesky (`categorizeStatus`).
- Optional reference provider `x_official.go` (X API v2) OR documented `fakeProvider`-only test path.
- 34 tests, EnvLookup injection (**no `t.Setenv`/`os.Setenv` under `-race`**), goleak + race, 0 live network.
- 0 new Go deps.

### Activation (EXTERNAL BLOCKER — money + decision)

Two gates: `USEARCH_X_ENABLED=true` **AND** a configured provider. Pick ONE provider:

| Provider                            | Endpoint                             | Auth         | Cost (verified live)           | ToS                                      |
| ----------------------------------- | ------------------------------------ | ------------ | ------------------------------ | ---------------------------------------- |
| (A) X official API v2 (recommended) | `GET /2/tweets/search/recent`        | Bearer token | ~$0.005 / post read, paid tier | lowest risk                              |
| (C) twitterapi.io                   | `GET /twitter/tweet/advanced_search` | `X-API-Key`  | ~$0.15 / 1k tweets             | 3rd-party → **ToS-ack gate required**    |
| ScrapeCreators                      | —                                    | —            | —                              | **REJECTED** — no search endpoint exists |

- Credentials via env (Bearer / API key); never logged, never in artifacts.
- 3rd-party provider additionally requires a ToS-acknowledgement gate before registration.
- Pre-run fixes D1 (score-formula prose) + D3 (frontmatter) — **already applied inline** (HISTORY iteration 2).

### Net

Code+tests = implement now. Live = needs a paid X data plan + provider choice (+ ToS-ack if 3rd-party).

---

## F-09 — SPEC-ADP-010 (Facebook + Threads / Meta) [P3]

New isolated package `internal/adapters/meta/` (NOT `social/` — Meta uses OAuth 2.0 user token + 60-day
refresh + app review = a different auth axis). 0 new Go deps.

### Threads = ACHIEVABLE (conditional)

**Code (run-ready, no blocker):**

- `meta.go` (`NewThreads`/`NewFacebook`, Name, Capabilities, Healthcheck), `search.go` (subSource dispatch), `search_threads.go` (live path: query validate, `url.Values`, `Bearer` header, `doRequest`, `parseKeywordSearch`), `client.go` (10s timeout, redirect allowlist `{graph.threads.net}`, reqid transport, `categorizeStatus`), `parse.go`, `score.go` (`neutralScore()=0.5`), `errors.go`.
- Endpoint verified live: `GET https://graph.threads.net/v1.0/keyword_search` (HTTP 200).
- Env gate: registered only when `THREADS_ACCESS_TOKEN` is set (missing → `ErrThreadsTokenMissing`, registry skips).
- v0 hardcodes `search_type=TOP`, `search_mode=KEYWORD`; `since/until` optional; `limit` clamp 1..100.
- Response has NO engagement/lang → `Score=0.5`, `Lang=""` (honest, documented).
- 56 tests, httptest + golden fixtures, 0 live CI network. Live tests deferred behind `-tags=integration` + `THREADS_LIVE=1`.

**Activation (EXTERNAL BLOCKER — Meta app + approval):**

1. Create a Meta developer app.
2. Get **`threads_basic` + `threads_keyword_search`** permissions approved via **Meta App Review**.
   - Without `threads_keyword_search` approval → searches ONLY the authed user's own posts, not public.
3. Obtain OAuth long-lived token (short 1h → long 60d) + implement **60-day refresh** via `GET /refresh_access_token`.

- Rate limit: **2,200 queries / 24h per user** (not per-minute). Sensitive keywords → empty array (normal).

### Facebook = NOT ACHIEVABLE (permanent)

- No code path. `NewFacebook` returns `ErrFacebookNotSupported`, issues 0 HTTP, NO env opt-in (permanent, unlike X).
- Official Graph API exposes no public-post keyword search (search reference URLs return HTTP 404 — verified; Threads page returns 200 in the same run). Scraping excluded per ToS (`tech.md:147`).
- Honest, documented dead-end. Stays RESERVED-but-DISABLED.

### Pre-run fixes — ALL RESOLVED (HISTORY iteration 2, plan-auditor cycle 1)

All 6 plan-audit defects were applied inline after review-1:

- D1 (major): removed the unsourced `"removed in Graph v2.0"` assertion from REQ-008 + Capabilities.Notes; now a current-state assertion only (`"no public-post keyword search"`), test 50 asserts no version/year string.
- D6 (minor): added `TestSearchThreadsClampsLimitToMin1` (test 14a, MaxResults=-5 → limit=1).
- D2 (empty-query split to REQ-ADP10-009), D3 (citation line split), D4 (0.0-vs-0.5 note + Open Q), D5 (rate-limit Notes) all applied.
- No pending pre-run fixes. SPEC run-ready (status: draft).

### Net

Threads code+tests = implement now; live = needs a Meta app + `threads_keyword_search` approval + OAuth token & refresh. Facebook = impossible via official API, stays disabled.

---

## Summary

| Gap           | Code now?                         | New Go deps | Live activation blocker                                        |
| ------------- | --------------------------------- | ----------- | -------------------------------------------------------------- |
| F-08 X        | yes (interface + fakeProvider)    | 0           | paid X data plan + provider choice (+ToS-ack if 3rd-party)     |
| F-09 Threads  | yes (full path + golden fixtures) | 0           | Meta app + `threads_keyword_search` review + OAuth 60d refresh |
| F-09 Facebook | n/a (permanent stub)              | 0           | none possible — official API has no public keyword search      |

Both code layers are unblocked and `/moai run`-ready. The blockers are operational (money / Meta approval / OAuth),
not engineering. Recommended order if pursuing: implement both CODE layers now (testable, mergeable), defer ACTIVATION
until the external accounts/credits/approvals are obtained.
