---
id: SPEC-ADP-001
title: Reddit Adapter (Reference Implementation)
version: 0.1.0
milestone: M2 — First end-to-end slice
status: draft
priority: P0
owner: expert-backend
methodology: tdd
coverage_target: 85
created: 2026-04-26
updated: 2026-04-26
author: limbowl
issue_number: null
depends_on: [SPEC-CORE-001, SPEC-OBS-001, SPEC-IR-001]
blocks: [SPEC-ADP-002, SPEC-FAN-001, SPEC-CLI-001, SPEC-SYN-001]
---

# SPEC-ADP-001: Reddit Adapter (Reference Implementation)

## HISTORY

- 2026-04-26 (initial draft v0.1, limbowl via manager-spec):
  First EARS-formatted SPEC drafted after research phase. Scope and
  contracts derived from `.moai/specs/SPEC-ADP-001/research.md` (791
  lines, every claim file-cited or URL-cited). Built on SPEC-CORE-001
  (`pkg/types.Adapter`, `pkg/types.Capabilities`, `pkg/types.Query`,
  `pkg/types.NormalizedDoc`, `*types.SourceError`, registry
  wrappedAdapter sole-emitter pattern at
  `internal/adapters/registry.go:172-263`) and SPEC-OBS-001
  (`AdapterCalls{adapter,outcome}` + `AdapterCallDuration{adapter}`
  collectors already in cardinality allowlist; no new metric families
  needed). Soft dep on SPEC-IR-001 for `Capabilities` consumer
  contract.

  User-locked decisions baked in:
  - **D1 Endpoint + Auth**: Public `https://www.reddit.com/search.json`
    no-auth path. `Capabilities.RequiresAuth=false`,
    `AuthEnvVars=[]`. OAuth (`oauth.reddit.com`) is OUT OF SCOPE;
    deferred to a future ADP-001a SPEC if measured value warrants.
    (Research §1.1, §7.1.)
  - **D2 NSFW filter**: User-controlled via `Query.Filters` with
    `Key="nsfw"`, `Value="true"|"false"`. Default (filter absent or
    `Value="false"`) sends `include_over_18=false` to Reddit; explicit
    `Value="true"` sends `include_over_18=true`. Documented in
    `Capabilities.Notes`. (Research §1.2, §7.4.)
  - **D3 Rate-limit policy**: On HTTP 429, parse `Retry-After`
    header (cap at 60s; default 5s when missing or malformed) and
    return `*types.SourceError{Adapter:"reddit",
    Category: types.CategoryRateLimited, HTTPStatus: 429,
    RetryAfter: <duration>, Cause: <inner>}`. Adapter does NOT retry
    internally; the fanout layer (SPEC-FAN-001, M3) owns retry
    orchestration. The adapter is stateless: it never tracks
    consecutive failures, never opens a circuit, never sleeps. After
    N consecutive 5xx the per-request response remains
    `Category: CategoryUnavailable`. This matches the
    division-of-labor documented in SPEC-CORE-001 §6.3 (registry
    wraps observability; adapter wraps source semantics; fanout
    wraps retry/orchestration). (Research §1.6, §1.7, §6 row 3.)
  - **D4 Tests**: `net/http/httptest.Server` stub + golden JSON
    fixtures under `internal/adapters/reddit/testdata/`. NO live
    network calls in CI. Optional env-gated integration test
    (`-tags=integration` + `REDDIT_LIVE=1`) is OUT OF SCOPE for v0.1;
    deferred to a follow-up SPEC if measured value warrants.
    (Research §4 testing approach.)

  Resolved discrepancy: `.moai/project/tech.md:106` claims `60/min`
  for the public JSON endpoint, while research.md §1.7 (citing
  painonsocial.com 2026 guidance) finds the documented unauthenticated
  ceiling is `10/min`. This SPEC adopts the conservative `10/min`
  figure for `Capabilities.RateLimitPerMin` (the value the Intent
  Router reads at startup). A follow-up sync task in `tech.md` to
  correct row "Reddit | … | 10/min unauth (60/min OAuth)" is
  recommended; see §11 Open Question 1. The `Capabilities.Notes`
  field documents the discrepancy verbatim so operators see it.

  11 EARS REQs (9 × P0 + 2 × P1) covering all five EARS
  patterns (Ubiquitous, Event-Driven, State-Driven via
  REQ-ADP-011 concurrency-safety contract, Optional, Unwanted),
  3 NFRs, ~46 representative TDD tests, 6 Open Questions
  carried forward from research.md §7 (the original §11.7 Metadata
  key API surface question was resolved inline in §6.3 with explicit
  REQUIRED vs OPTIONAL classification — see iteration 2 HISTORY
  entry below). Zero new Go module
  dependencies — pure stdlib (`net/http`, `encoding/json`, `time`,
  `context`, `errors`, `strings`, `strconv`, `net/url`, `unicode/utf8`,
  `math`) plus existing `pkg/types` and `internal/obs` (nil-safe
  consumer; the registry wraps observability, not the adapter).
  Inserted into M2 as the FIRST real adapter consuming the
  SPEC-CORE-001 contract; pattern that ADP-002..009 will copy.
  Harness level: standard (single domain, ≤10 source files, no
  security/payment keywords). Sprint Contract optional. Ready for
  plan-auditor review and annotation cycle.

- 2026-04-26 (iteration 3 — run-phase NFR amendment, limbowl via
  manager-tdd + MoAI orchestrator): NFR-ADP-001 alloc target revised
  from ≤ 250 to ≤ 500 (10/doc → 20/doc) after empirical baselining
  measured 460 allocs/op on the reference fixture. Floor analysis
  documented inline in NFR-ADP-001 row of §4 and §5 acceptance criteria
  block. Original target was set without empirical baseline; the
  `pkg/types.NormalizedDoc.Metadata = map[string]any` contract from
  SPEC-CORE-001 forces a structural floor of ~17 allocs/doc. All other
  acceptance criteria (55 tests pass, race clean, coverage 92.4%, parse
  p50 = 0.115ms, vet/gofmt clean, sole-emitter discipline preserved)
  unaffected. Implementation commit: 41372d4. The amendment baselines
  the alloc target for the ADP-001 reference shape; ADP-002..009 will
  inherit the same floor unless `pkg/types` Metadata contract is
  refactored in a future SPEC.

- 2026-04-26 (iteration 2 revisions, limbowl via manager-spec):
  D1 added REQ-ADP-011 (State-Driven concurrency safety) closing
  the missing fifth EARS pattern; D2/D3 corrected HISTORY counts
  (9 P0 + 2 P1, 46 tests); D4 added Options.HealthcheckTarget seam
  with default `"www.reddit.com:443"` resolving the §5 acceptance
  vs §6.4 sketch contradiction; D5 specified NFR-ADP-001 benchmark
  invocation `go test -bench=BenchmarkParseListing25Docs
  -benchtime=10x -count=5 ./internal/adapters/reddit/...` with
  median-of-5 assertion mechanism; D6 resolved Metadata key
  REQUIRED (6 keys) / OPTIONAL (7 keys) classification inline in
  §6.3 and removed Open Question §11.7 (now 6 open questions);
  D7 corrected noop.go line count (47 → 46); D9 dropped NBSP from
  REQ-ADP-008 acceptance fixture (NBSP not in `unicode.IsSpace`);
  D10 collapsed duplicate score.go MX target rows in §6.7. Audit
  report: .moai/reports/plan-audit/SPEC-ADP-001-review-1.md.

---

## 1. Purpose

SPEC-CORE-001 published the typed adapter contract (`pkg/types.Adapter`
4-method interface, `pkg/types.NormalizedDoc` 15-field canonical
struct, `*types.SourceError` taxonomy with four Categories,
`pkg/types.Capabilities` descriptor) and the `internal/adapters.Registry`
with its `wrappedAdapter` (`internal/adapters/registry.go:172-263`)
that emits one Prometheus counter + one histogram + one OTel span +
one slog record per `Search` call. SPEC-OBS-001 registered the
`AdapterCalls{adapter,outcome}` + `AdapterCallDuration{adapter}`
collectors with `adapter` and `outcome` already in the cardinality
allowlist. SPEC-IR-001 published the `RoutingDecision.AdapterSet`
shape — the Intent Router selects adapters by intersecting
`categoryEligibleDocTypes` with `Capabilities.SupportedLangs`
(IR-001 REQ-IR-008). The reference noop adapter
(`internal/adapters/noop/noop.go`, 46 lines) demonstrates the minimum
viable shape including the compile-time interface assertion
`var _ types.Adapter = (*Adapter)(nil)`.

SPEC-ADP-001 fills `internal/adapters/reddit/` with the **first real
adapter** consuming this contract end-to-end. The Reddit adapter is
chosen as the reference implementation because:

1. **Public no-auth endpoint** (`https://www.reddit.com/search.json`)
   eliminates secret-management complexity from the reference
   pattern, letting ADP-002..009 copy a clean shape.
2. **Cursor-based pagination** (`data.after` → `&after=t3_xxxxx`)
   exercises the full `Query.Cursor` round-trip that all 12+ adapters
   will need.
3. **Rich response envelope** (Listing → children[t3] → data.{15+
   fields}) exercises the full NormalizedDoc field-mapping discipline
   including `Metadata map[string]any` extension-bag conventions.
4. **All four error categories** are reachable in normal operation:
   429 (`CategoryRateLimited`, with Retry-After), 5xx
   (`CategoryUnavailable`), 4xx (`CategoryPermanent`), and timeout/
   network blip (`CategoryTransient` via `context.DeadlineExceeded`),
   so the error-taxonomy contract gets a real workout.
5. **Score normalization** is non-trivial (Reddit's score is
   unbounded `[-∞, +∞]` integer; NormalizedDoc.Score must be
   `[0.0, 1.0]`), forcing the SPEC to specify a deterministic
   formula that future adapters can adopt or adapt.

The adapter does NOT do fanout (SPEC-FAN-001 owns goroutine
dispatch), does NOT do retry (SPEC-FAN-001 owns orchestration), does
NOT do caching (SPEC-CACHE-001 owns 5-phase fallback), does NOT do
ranking fusion (SPEC-IDX-001 owns RRF), and does NOT emit any
metric/log/span itself (the registry wrappedAdapter does, sole-emitter
discipline). It DOES one job: turn a `types.Query` into a Reddit
HTTP request, parse the JSON Listing, and return
`[]types.NormalizedDoc` or `*types.SourceError`.

Completion unblocks four downstream SPECs in M2 and beyond:
SPEC-ADP-002 (Hacker News) and the seven M3 ADP-* SPECs will copy
the structure (file layout, error mapping, score-normalization
philosophy, fixture-based test discipline) verbatim. SPEC-FAN-001
(M3) consumes the adapter via `registry.Get("reddit").Search(ctx, q)`
and orchestrates retries on `errors.Is(err, types.ErrTransient)` /
`errors.Is(err, types.ErrRateLimited)`. SPEC-CLI-001 (M2) wires the
Reddit adapter into `usearch query "..."` for the M2 exit-criterion
demonstration (`.moai/project/roadmap.md:147`: "`usearch query
'hello world'` returns Reddit + HN results with one synthesized
paragraph + citations"). SPEC-SYN-001 consumes the
`[]types.NormalizedDoc` for citation assembly via the gpt-researcher
wrapper.

This is the **wedge SPEC** for the reference adapter pattern: the
shape laid down here propagates into every M3 ADP-* SPEC. Getting
the file layout, error mapping, NormalizedDoc field discipline, MX
tag plan, and TDD harness right here saves rework across 12
downstream adapter implementations.

---

## 2. Scope

### 2.1 In-Scope

| Ref | Item |
|-----|------|
| a | `internal/adapters/reddit/reddit.go`: `Adapter` struct (HTTP client + base URL + user-agent + default options), `New(opts Options) (*Adapter, error)` constructor, `Name() string` returning `"reddit"`, `Capabilities() types.Capabilities` returning a deterministic descriptor (RequiresAuth=false, AuthEnvVars=[], DocTypes=[DocTypePost], SupportedLangs=[] (language-agnostic), SupportsSince=false (V1 hardcodes t=all), RateLimitPerMin=10, DefaultMaxResults=25, DisplayName="Reddit", Notes documenting the rate-limit discrepancy + NSFW default + `t=all` limitation), and `Healthcheck(ctx) error` (TCP-connect probe to `www.reddit.com:443` with caller-supplied ctx). Compile-time interface assertion `var _ types.Adapter = (*Adapter)(nil)` at the bottom. |
| b | `internal/adapters/reddit/search.go`: `(*Adapter).Search(ctx, q types.Query) ([]types.NormalizedDoc, error)` — the hot path. Validates the query, builds the request URL via `url.Values`, delegates HTTP execution to `client.go`, delegates response parsing to `parse.go`, returns `[]NormalizedDoc` or `*SourceError`. Honours `ctx` cancellation throughout. |
| c | `internal/adapters/reddit/client.go`: HTTP client construction (timeout=10s default, `CheckRedirect` enforces a domain allowlist `{www.reddit.com, old.reddit.com, new.reddit.com, reddit.com}` with max 3 hops, `Transport` wrapped with `internal/obs/reqid.NewTransport(http.DefaultTransport)` for request-ID propagation), single `doRequest(ctx, *http.Request) (*http.Response, error)` helper that sets the User-Agent header and the `Accept: application/json` header, and `categorizeStatus(httpStatus int, retryAfter time.Duration, cause error) *types.SourceError` mapping HTTP status → Category per the table in §6. |
| d | `internal/adapters/reddit/parse.go`: `parseListing(body []byte, retrievedAt time.Time) ([]types.NormalizedDoc, string, error)` — parses the Reddit JSON envelope into `[]NormalizedDoc` and returns the `data.after` pagination cursor as the second value. Filters out children whose `kind != "t3"` (non-post results). Per-doc transform per the field-mapping table in §6.3. Empty-listing responses return `(nil, "", nil)`. Malformed JSON returns `*SourceError{Category: CategoryPermanent, Cause: <json error>}`. |
| e | `internal/adapters/reddit/score.go`: `normalizeScore(redditScore int) float64` — implements the Tanh formula specified in §2.3, deterministic, pure function. Package-level constants `tanhDivisor = 100.0` and `scoreCenter = 0.5` annotated with `@MX:NOTE` explaining the empirical choice. |
| f | `internal/adapters/reddit/errors.go`: package-private sentinel `ErrInvalidQuery = errors.New("reddit: query text empty or whitespace-only")` (wrapped in `*SourceError{Category: CategoryPermanent}` by Search). Helper `parseRetryAfter(header string, now time.Time) time.Duration` that handles both integer-seconds and HTTP-date forms per RFC 7231 §7.1.3, capped at 60s, defaulting to 5s on parse failure. |
| g | `internal/adapters/reddit/reddit_test.go`: tests for Adapter interface conformance (`var _ types.Adapter` assertion via `assertInterface`), `Name()` returns "reddit", `Capabilities()` returns deterministic value (called twice; equal), `Healthcheck()` succeeds against a stub `httptest.Server`, `New()` validates options. |
| h | `internal/adapters/reddit/search_test.go`: the largest test file. Drives `(*Adapter).Search` against `httptest.Server` with golden fixtures: happy path 25 results, empty result, 429 with Retry-After, 4xx, 5xx, redirect to allowed and disallowed hosts, NSFW filter on/off, pagination cursor round-trip, ctx cancellation mid-request. |
| i | `internal/adapters/reddit/client_test.go`: HTTP client unit tests — `categorizeStatus` truth table over 7 status codes, `parseRetryAfter` table over 6 input shapes, redirect allowlist enforcement, User-Agent header presence, `Accept: application/json` header presence. |
| j | `internal/adapters/reddit/parse_test.go`: field-mapping unit tests — table over 5 fixtures (full post, self-post with empty url, link post with external url, deleted-author post, NSFW post). Asserts each NormalizedDoc field per the §6.3 mapping table. Snippet truncation to 280 chars. Score normalization (4 example values). `data.after` cursor round-trip. Filter of non-`t3` children. Hash field is empty (REQ-ADP-006 / decision §2.4). |
| k | `internal/adapters/reddit/score_test.go`: `normalizeScore` table-driven test over 7 score values (`-1000, -10, 0, 10, 100, 1000, 10000`) with expected `[0.0, 1.0]` outputs (computed from the formula, asserted within `±0.001`). Determinism: same input → identical output across two calls. Boundary: very large positive/negative scores asymptote to 1.0 / 0.0 within the `±0.001` tolerance. |
| l | `internal/adapters/reddit/bench_test.go`: `BenchmarkParseListing25Docs` (NFR-ADP-001 — p50 ≤ 5 ms parse time on amd64 for a 25-doc Listing fixture; allocation ≤ 10 allocs per doc parsed). |
| m | `internal/adapters/reddit/testdata/`: golden JSON fixtures — `search_response.json` (25-post happy path, ~5KB), `search_response_empty.json` (empty children array, ~500B), `search_response_pagination.json` (page-2 fixture with `data.after` set), `search_response_with_nsfw.json` (mixed NSFW + safe posts, exercises filter logic), `search_response_deleted_post.json` (single post with `Author == "[deleted]"` and `Body == "[removed]"`), `search_response_malformed.json` (truncated JSON for parse-error path). |

### 2.2 Out-of-Scope

This SPEC explicitly excludes the following items. Each has a known
destination SPEC; this list prevents scope creep into ADP-001 (the
reference shape).

- **Per-source customisations specific to other sources** (Hacker
  News Algolia API quirks, arXiv OAI-PMH, GitHub PAT auth, YouTube
  yt-dlp metadata, Bluesky AT Protocol, Naver Korean-locale handling,
  Daum scraper-style handling, KoreaNewsCrawler RSS, SearXNG bridge,
  Polymarket public API) → SPEC-ADP-002 through SPEC-ADP-009.
- **Retry orchestration** (exponential backoff, circuit breaker,
  per-source health-state tracking, jitter, max-attempt counters) →
  SPEC-FAN-001 (M3). The adapter returns one categorised error per
  request and does not retry.
- **Response caching** (in-process LRU, Redis-backed, on-disk) →
  SPEC-CACHE-001 (M3). Each `Search` call is independent and
  idempotent at the adapter layer.
- **Result ranking, deduplication, RRF fusion across adapters** →
  SPEC-IDX-001 (M3). The adapter returns docs in the order Reddit
  returned them, with `Score` in `[0.0, 1.0]` per the formula in §2.3,
  but does not re-rank.
- **Tenant-scoped NSFW policy** (per-team override of the
  `include_over_18` default, audit-logged opt-in) → SPEC-AUTH-002
  (M6). v0.1 honours `Query.Filters` literally; SPEC-AUTH-002 will
  layer policy on top.
- **Adapter health-state machine** (auto-disable on N consecutive
  `CategoryUnavailable`, auto-re-enable on Healthcheck pass) →
  SPEC-EVAL-002 (M8). The adapter is stateless; FAN-001 and EVAL-002
  own state.
- **OAuth-authenticated variant** (`oauth.reddit.com` endpoint,
  60/min rate limit, per-team Reddit app credentials) → future
  SPEC-ADP-001a if measured value warrants. v0.1 is no-auth public
  endpoint only.
- **Subreddit-scoped search** (`/r/{sub}/search.json` path with
  `restrict_sr=on`) → out of v0.1 scope; future SPEC-ADP-001b.
- **Time-range filtering** (Reddit `t=hour|day|week|month|year`) →
  out of v0.1; hardcoded `t=all`. Documented in `Capabilities.Notes`
  and `SupportsSince=false`.
- **Result-sort customisation** (Reddit `sort=hot|new|top|comments`)
  → out of v0.1; hardcoded `sort=relevance`.
- **Comment retrieval** (Reddit comment threads via `t1_*` kinds) →
  out of scope; v0.1 returns posts (`t3` kind) only.
- **Live network integration tests in CI** → out of v0.1.
  `httptest.Server` + golden fixtures only. Optional env-gated live
  test (`-tags=integration` + `REDDIT_LIVE=1`) deferred to a future
  follow-up.
- **OpenAPI / proto schema for the adapter response** — the
  `[]types.NormalizedDoc` return type IS the schema; no separate IDL.
- **Korean tokenisation or language inference** for Reddit posts
  → SPEC-IDX-003 (M3). The adapter sets `NormalizedDoc.Lang = ""`
  (unknown).
- **`pkg/llm` integration** — the Reddit adapter does NOT call any
  LLM. Classification is the Intent Router's job (SPEC-IR-001).
- **Pre-flight Query validation beyond text-emptiness** (e.g.,
  rejecting queries longer than N chars) — Reddit accepts long
  queries; the adapter does not enforce additional length limits.
- **Per-adapter custom Prometheus metrics** (e.g.,
  `reddit_pagination_pages_total`) — would require amending
  SPEC-OBS-001's allowlist; out of scope. The shared
  `AdapterCalls{adapter="reddit",outcome}` family is sufficient for
  v0.1.

### 2.3 Score Normalization Formula (Architecture)

[HARD] The score normaliser in `score.go::normalizeScore(redditScore int)
float64` is a deterministic pure function so that golden tests can
compute expected `NormalizedDoc.Score` values from the input alone
and downstream ranking (SPEC-IDX-001 RRF) gets a stable input.

**Formula**:

```
Score = clamp(0.5 + 0.5 * tanh(score / 100.0), 0.0, 1.0)
```

where `tanh` is the standard hyperbolic tangent
(`math.Tanh` in Go's stdlib), `score / 100.0` is float division (the
integer Reddit score is converted to float64 first), and `clamp(x, lo,
hi) = max(lo, min(hi, x))`.

**Properties**:

- **Domain**: Reddit `score` ∈ `(-∞, +∞)` integer (in practice bounded
  by Reddit's signed-32-bit field; the adapter accepts the JSON
  `int64` value safely).
- **Codomain**: `Score` ∈ `[0.0, 1.0]` (mathematically `(0.0, 1.0)`
  from `tanh`; `clamp` enforces the closed interval against
  floating-point edge cases).
- **Symmetry**: `score = 0` → `Score = 0.5` (a "neutral" Reddit post
  with zero net upvotes maps to the middle of the `[0, 1]` range).
- **Inflection**: `score = ±100` → `Score ≈ 0.881 / 0.119`. Posts
  with 100+ net upvotes are "above the fold" intuitively.
- **Saturation**: `score = ±1000` → `Score ≈ 0.9999955 / 0.0000045`,
  effectively saturated. Posts with thousands of upvotes are
  indistinguishable in `Score` value (SPEC-IDX-001 RRF will
  re-distinguish via rank).
- **Determinism**: pure function, no state, no time, no I/O.

**Worked examples** (computed from the formula, asserted in
`score_test.go::TestNormalizeScoreTable` within `±0.001` tolerance):

| Reddit score | `score/100` | `tanh(score/100)` | `Score`        |
|--------------|-------------|--------------------|----------------|
| -1000        | -10.000     | -1.000000          | 0.000          |
| -10          | -0.100      | -0.099668          | 0.450166       |
| 0            | 0.000       | 0.000000           | 0.500000       |
| 10           | 0.100       | 0.099668           | 0.549834       |
| 100          | 1.000       | 0.761594           | 0.880797       |
| 1000         | 10.000      | 1.000000           | 1.000          |
| 10000        | 100.000     | 1.000000           | 1.000          |

**Tie-break behaviour**: Two posts with equal Reddit `score` produce
equal `NormalizedDoc.Score`. Order is preserved from Reddit's
response (Reddit's `sort=relevance` ranking determines order;
`Score` does not re-sort). SPEC-IDX-001 RRF uses rank not score for
fusion across adapters, so equal scores within Reddit do not cause
ranking instability.

**Rationale (why Tanh over Log1p or Percentile)** (research.md §2.2):
- Tanh handles negative scores gracefully (Log1p needs piecewise
  treatment); Reddit posts can be heavily downvoted.
- Tanh is stateless (Percentile would need a rolling baseline; the
  adapter is required to be stateless per D3).
- Inflection at 100 upvotes matches a common UX intuition for a
  "good" Reddit post.
- RRF in SPEC-IDX-001 will weight rank not raw score across adapters,
  so the precise score curve matters less than the bounded codomain
  and determinism.

The formula is intentionally locked in v0.1 — changing it later
requires a major-version bump of `Capabilities.Notes` and
coordination with SPEC-IDX-001's RRF tuning. Open Question §11.2
documents revisit triggers.

---

## 3. EARS Requirements

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| REQ-ADP-001 | Ubiquitous | The package `internal/adapters/reddit` SHALL expose an `Adapter` struct that implements `pkg/types.Adapter` exactly: `Name() string` returning `"reddit"`, `Search(ctx context.Context, q types.Query) ([]types.NormalizedDoc, error)`, `Healthcheck(ctx context.Context) error`, `Capabilities() types.Capabilities`. The package SHALL include a compile-time interface assertion `var _ types.Adapter = (*Adapter)(nil)`. `Capabilities()` SHALL be deterministic (two consecutive calls return equal values). | P0 | `TestAdapterName`, `TestAdapterImplementsInterface` (compile-time), `TestCapabilitiesDeterministic`, `TestCapabilitiesShape` (asserts SourceID="reddit", DocTypes=[DocTypePost], RequiresAuth=false, AuthEnvVars=[], SupportsSince=false, RateLimitPerMin=10, DefaultMaxResults=25 — all in `internal/adapters/reddit/reddit_test.go`). |
| REQ-ADP-002 | Event-Driven | WHEN `(*Adapter).Search(ctx, q)` is invoked with a non-empty `q.Text`, the adapter SHALL build an HTTP GET request to `https://www.reddit.com/search.json` with the following query parameters: `q=<url.QueryEscape(q.Text)>`, `sort=relevance`, `t=all`, `type=link`, `limit=clamp(q.MaxResults, 1, 100)` (defaulting to 25 when `q.MaxResults == 0`), `include_over_18=<NSFW filter per REQ-ADP-007>`, and `after=<q.Cursor>` if `q.Cursor != ""`. The adapter SHALL execute the request via the constructed `*http.Client`, parse the JSON Listing per REQ-ADP-006 mapping, and return `(docs, nil)` on HTTP 200 with `len(docs) ≤ 100`. | P0 | `TestSearchHappyPath25Docs` (httptest.Server returns `search_response.json`; assert 25 NormalizedDocs returned, each with all required fields populated and `Validate()` returning nil); `TestSearchURLParametersIncludeAllRequired` (inspect captured request URL; assert all 7 params present); `TestSearchClampsLimitTo100` (q.MaxResults=500 → URL has `limit=100`); `TestSearchDefaultsLimitTo25` (q.MaxResults=0 → URL has `limit=25`). All in `search_test.go`. |
| REQ-ADP-003 | Event-Driven | WHEN HTTP 429 is received from the Reddit endpoint, the adapter SHALL parse the `Retry-After` response header per RFC 7231 §7.1.3 (integer-seconds OR HTTP-date), cap the result at 60 seconds (any larger value is replaced with 60s), default to 5 seconds when the header is missing or malformed, and return `(nil, &types.SourceError{Adapter:"reddit", Category: types.CategoryRateLimited, HTTPStatus: 429, RetryAfter: <duration>, Cause: errors.New("reddit: rate limited")})`. The adapter SHALL NOT retry internally. | P0 | `TestSearchHTTP429WithIntegerRetryAfter` (`Retry-After: 30` → RetryAfter=30s); `TestSearchHTTP429WithHTTPDateRetryAfter` (`Retry-After: Wed, 21 Oct 2026 07:28:00 GMT` → RetryAfter computed from `time.Now()`; assert > 0); `TestSearchHTTP429NoRetryAfterDefaults5s`; `TestSearchHTTP429RetryAfterCapped60s` (`Retry-After: 999` → 60s); `TestSearchHTTP429NoInternalRetry` (assert exactly 1 outbound request). All in `search_test.go` + `client_test.go`. |
| REQ-ADP-004 | Event-Driven | WHEN HTTP 401, 403, or 404 is received from the Reddit endpoint, the adapter SHALL return `(nil, &types.SourceError{Adapter:"reddit", Category: types.CategoryPermanent, HTTPStatus: <code>, Cause: errors.New("reddit: permanent failure: <code>")})`. The adapter SHALL NOT retry. | P0 | `TestSearchHTTP401`, `TestSearchHTTP403`, `TestSearchHTTP404` — each asserts `errors.Is(err, types.ErrPermanent)` and the returned `*SourceError.HTTPStatus` matches. In `search_test.go`. |
| REQ-ADP-005 | Event-Driven | WHEN HTTP 500/502/503/504 is received OR a connection error occurs (DNS failure, dial timeout, read timeout, TLS handshake failure), the adapter SHALL return `(nil, &types.SourceError{Adapter:"reddit", Category: types.CategoryUnavailable, HTTPStatus: <code or 0>, Cause: <inner error>})`. Network-layer errors set `HTTPStatus=0`. The adapter SHALL NOT retry. | P0 | `TestSearchHTTP500`, `TestSearchHTTP503`, `TestSearchConnectionRefused` (httptest.Server closed before request), `TestSearchUnavailablePreservesUnderlyingError` (assert `errors.Unwrap(srcErr).Error()` contains the inner cause). In `search_test.go` + `client_test.go`. |
| REQ-ADP-006 | Ubiquitous | The adapter SHALL transform each Reddit JSON `children[i]` whose `kind == "t3"` into one `types.NormalizedDoc` using the field mapping in §6.3, MUST set `RetrievedAt = time.Now().UTC()` at the moment of parsing, MUST leave `Hash = ""` (consumers compute via `CanonicalHash()`), MUST populate `Metadata` with at minimum the keys `{subreddit, over_18, num_comments, upvote_ratio, external_url, kind}`, MUST set `DocType = types.DocTypePost`, MUST set `Lang = ""` (unknown). Children with `kind != "t3"` SHALL be skipped silently. The cursor `data.after` (if non-empty) SHALL be returned as the second return value of `parseListing` so `Search` can surface it via `Metadata["next_cursor"]` on the LAST returned NormalizedDoc — consumers can paginate by passing this value as `q.Cursor` on the next call. | P0 | `TestParseListingFieldMapping` (table-driven over 5 fixtures); `TestParseListingFiltersNonT3Kinds` (fixture with mixed t1/t3/t5 children → only t3 returned); `TestParseListingPaginationCursor` (fixture with `data.after = "t3_xyz"` → returned NormalizedDocs[len-1].Metadata["next_cursor"] == "t3_xyz"); `TestParseListingHashEmpty` (every returned doc has `Hash == ""`); `TestParseListingMetadataKeys` (all 6 required keys present in each returned doc). All in `parse_test.go`. |
| REQ-ADP-007 | Optional | WHERE `Query.Filters` contains an entry with `Key == "nsfw"` AND `Value == "true"`, the adapter SHALL set `include_over_18=true` in the request URL; WHERE the `nsfw` filter is absent OR `Value == "false"` OR `Value == ""`, the adapter SHALL set `include_over_18=false`. Any value other than `"true"` is treated as `false` (no error returned). The default behaviour (no filter supplied) is `include_over_18=false` (NSFW EXCLUDED). | P1 | `TestSearchNSFWFilterTrueIncludesOver18` (Filters=[{nsfw, true}] → URL has `include_over_18=true`); `TestSearchNSFWFilterFalseExcludes` (Filters=[{nsfw, false}] → URL has `include_over_18=false`); `TestSearchNSFWFilterAbsentDefaultsExclude` (Filters=nil → URL has `include_over_18=false`); `TestSearchNSFWFilterUnknownValueDefaultsExclude` (Filters=[{nsfw, maybe}] → URL has `include_over_18=false`). All in `search_test.go`. |
| REQ-ADP-008 | Unwanted | IF `Query.Text` is empty OR contains only Unicode whitespace runes (per `unicode.IsSpace` over every rune), THEN the adapter SHALL return `(nil, &types.SourceError{Adapter:"reddit", Category: types.CategoryPermanent, Cause: ErrInvalidQuery})` immediately and SHALL NOT issue any HTTP request. | P0 | `TestSearchEmptyQueryRejectedNoHTTP` (table-drives Text=`""`, Text=`"   "`, Text=`"\t\n  \r"`; for each asserts `errors.Is(err, types.ErrPermanent)` AND assert httptest.Server received zero requests). In `search_test.go`. |
| REQ-ADP-009 | Ubiquitous | The adapter SHALL set the `User-Agent` HTTP header on every outbound request to a non-default value of the form `usearch/<version> (+https://github.com/elymas/universal-search)` where `<version>` is supplied via `Options.UserAgentVersion` (default `"v0.1"`). The adapter SHALL set the `Accept` header to `application/json`. Reddit blocks default Go `net/http` User-Agent strings; setting a custom UA is a HARD precondition for any successful request (research.md §1.1). | P0 | `TestSearchSetsCustomUserAgent` (inspect captured `r.Header.Get("User-Agent")`; assert it starts with `"usearch/"` and contains `"(+https://github.com/elymas/universal-search)"`); `TestSearchSetsAcceptJSON` (assert `Accept: application/json`); `TestSearchUserAgentVersionConfigurable` (Options.UserAgentVersion="v0.2-rc1" → header contains `"usearch/v0.2-rc1"`). In `client_test.go`. |
| REQ-ADP-010 | Optional | WHERE the response is HTTP 301/302/303/307/308, the adapter's `*http.Client.CheckRedirect` SHALL follow up to 3 redirects WITHIN the allowlist `{www.reddit.com, old.reddit.com, new.reddit.com, reddit.com}`. Cross-domain redirects (any other host) SHALL be rejected by returning an error from `CheckRedirect` (Go stdlib then returns the redirect response's body unread; the adapter wraps this as `*SourceError{Adapter:"reddit", Category: CategoryPermanent, Cause: errors.New("reddit: cross-domain redirect rejected: <target host>")}` to prevent SSRF (research.md §1.8, §6 row 2). | P1 | `TestSearchFollowsAllowlistRedirect` (httptest.Server returns 301 → another httptest.Server with `www.reddit.com`-rewritten Host header; assert 200-path NormalizedDocs returned); `TestSearchRejectsCrossDomainRedirect` (httptest.Server returns 301 to `attacker.com`; assert `errors.Is(err, types.ErrPermanent)` AND error message contains "cross-domain redirect"); `TestSearchRejectsRedirectChainOver3` (httptest.Server bouncing within allowlist 4 times; assert error after 3 hops). In `client_test.go`. |
| REQ-ADP-011 | State-Driven | WHILE the same `*Adapter` instance is registered in the adapter registry and is being invoked concurrently from N goroutines (N ≥ 1), each `Search(ctx, q)` call SHALL execute independently with no shared mutable state across calls (the underlying `*http.Client` is goroutine-safe per Go stdlib; the adapter holds no per-call state); the cumulative effect SHALL be N independent HTTP round-trips with no race-detector alarms. | P0 | `TestSearchConcurrentSafe` (50 goroutines each issuing one Search against the same httptest.Server; assert (a) no race-detector alarm under `-race`, (b) total response count = 50 observed at the stub, (c) all 50 returned RoutingDecisions are well-formed `[]NormalizedDoc` slices with `Validate()` returning nil for every doc). In `search_test.go`. |

---

## 4. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| NFR-ADP-001 | Performance (parse path) | The parse path `parseListing(body []byte, retrievedAt time.Time) ([]NormalizedDoc, string, error)` SHALL execute with mean wall-clock duration per op ≤ 5 ms over `go test -bench=BenchmarkParseListing25Docs -benchtime=10x -count=5 ./internal/adapters/reddit/...` on amd64; the median of the 5 runs is the assertion value (passes when ≤ 5 ms). The fixture is the `search_response.json` golden (25-doc Listing, ~5KB). Allocation count ≤ 20 per doc parsed (i.e. ≤ 500 allocs total for 25 docs) per the same benchmark's `allocs/op` field. The original ≤ 10/doc target was empirically infeasible: `pkg/types.NormalizedDoc.Metadata = map[string]any` forces a floor of ~17 allocs/doc (1 map + 6 boxed primitives + 6-8 JSON-unmarshalled strings + URL concat + struct copy). Reaching ≤ 10/doc requires changing `pkg/types` (SPEC-CORE-001 contract — out of scope for ADP-001) or removing the Metadata mandate (REQ-ADP-006 violation). Measured via `BenchmarkParseListing25Docs` in `internal/adapters/reddit/bench_test.go`, run weekly in CI per the cadence established in SPEC-OBS-001 NFR-OBS-001. Benchmarks do not count toward coverage. |
| NFR-ADP-002 | End-to-end Latency | The end-to-end `Search` round-trip against the `httptest.Server` stub (no real network) SHALL complete with p95 ≤ 200 ms over 100 invocations, measured by `TestSearchE2ELatencyStubP95` in `search_test.go` (sort durations ascending, assert `durations[94] ≤ 200ms`). The harder live-Reddit p95 (≤ 5s) is documented as the operational target but is NOT enforced in CI (no live network). |
| NFR-ADP-003 | No goroutine leak on cancellation | The adapter SHALL NOT leak any goroutine when the caller's context is cancelled mid-`Search`. Verified by `TestSearchNoGoroutineLeakOnCancel` in `search_test.go`, which uses `go.uber.org/goleak.VerifyNone(t)` after a `Search` call whose ctx is cancelled mid-flight via a 50ms-delayed cancel; assert zero residual goroutines after the call returns. |

---

## 5. Acceptance Criteria

### REQ-ADP-001 — Adapter Interface Conformance

- File `internal/adapters/reddit/reddit.go` declares `Adapter` struct
  with the documented fields (`httpClient *http.Client`, `baseURL
  string`, `userAgent string`).
- The compile-time assertion `var _ types.Adapter = (*Adapter)(nil)`
  appears at the bottom of `reddit.go`. If the interface ever drifts,
  this assertion fails to compile.
- `(*Adapter).Name()` returns the literal string `"reddit"`.
- `(*Adapter).Capabilities()` returns a `types.Capabilities` with:
  - `SourceID = "reddit"` (matches `Name()`)
  - `DisplayName = "Reddit"`
  - `DocTypes = []types.DocType{types.DocTypePost}`
  - `SupportedLangs = nil` (language-agnostic; matches IR-001
    REQ-IR-008 fallback semantics)
  - `SupportsSince = false`
  - `RequiresAuth = false`
  - `AuthEnvVars = nil`
  - `RateLimitPerMin = 10`
  - `DefaultMaxResults = 25`
  - `Notes` contains the substrings `"public no-auth"`,
    `"NSFW excluded by default"`, `"t=all"`, and `"rate limit
    discrepancy"` (the documentation hook to surface the resolved
    10/min vs 60/min issue to operators).
- `(*Adapter).Healthcheck(ctx)` succeeds against an httptest.Server
  binding `127.0.0.1:0`. Tests construct the Adapter with
  `Options{HealthcheckTarget: <httptest.Server.Listener.Addr().String()>}`
  to redirect the dial target to a loopback test server; the
  production default is `"www.reddit.com:443"`.
- `TestAdapterName`, `TestAdapterImplementsInterface`,
  `TestCapabilitiesDeterministic`, `TestCapabilitiesShape`,
  `TestHealthcheckSucceeds` all pass.

### REQ-ADP-002 — Search Happy Path

- `TestSearchHappyPath25Docs` against `testdata/search_response.json`
  returns exactly 25 `NormalizedDoc` entries; each passes
  `Validate()` returning nil; the captured request URL contains all
  7 documented query parameters with the documented values.
- `TestSearchURLParametersIncludeAllRequired`,
  `TestSearchClampsLimitTo100`, `TestSearchDefaultsLimitTo25`,
  `TestSearchHonoursCursorParameter` (q.Cursor="t3_abc" → URL
  contains `&after=t3_abc`) all pass.

### REQ-ADP-003 — HTTP 429 Rate-Limit Mapping

- `TestSearchHTTP429WithIntegerRetryAfter` asserts returned err is
  `*types.SourceError` with `Category=CategoryRateLimited`,
  `HTTPStatus=429`, `RetryAfter=30s`.
- `TestSearchHTTP429WithHTTPDateRetryAfter` parses an HTTP-date 30s
  in the future; asserts `RetryAfter` is in `(25s, 35s)` (allowing
  test-clock drift).
- `TestSearchHTTP429NoRetryAfterDefaults5s` (no header) asserts
  `RetryAfter=5s`.
- `TestSearchHTTP429RetryAfterCapped60s` (`Retry-After: 999`) asserts
  `RetryAfter=60s`.
- `TestSearchHTTP429NoInternalRetry` instruments the httptest.Server
  with a request counter; asserts exactly 1 request observed.

### REQ-ADP-004 — HTTP 4xx Permanent Mapping

- `TestSearchHTTP401`, `TestSearchHTTP403`, `TestSearchHTTP404` each
  assert `errors.Is(err, types.ErrPermanent)` and the returned
  `*SourceError.HTTPStatus` matches the stub's status code.
- `TestSearchHTTP4xxNoInternalRetry` asserts exactly 1 request observed.

### REQ-ADP-005 — HTTP 5xx and Network Failure

- `TestSearchHTTP500`, `TestSearchHTTP503` each assert
  `errors.Is(err, types.ErrSourceUnavailable)` and `HTTPStatus=500/503`.
- `TestSearchConnectionRefused` (httptest.Server closed before
  request) asserts `errors.Is(err, types.ErrSourceUnavailable)` and
  `HTTPStatus=0`.
- `TestSearchUnavailablePreservesUnderlyingError`: assert
  `errors.Unwrap(srcErr) != nil` and the inner error message contains
  "connection refused" or equivalent.

### REQ-ADP-006 — NormalizedDoc Field Mapping

- `TestParseListingFieldMapping` table-drives 5 fixtures (full post,
  self-post empty url, link post external url, deleted-author post,
  NSFW post). For each, asserts every NormalizedDoc field per the
  §6.3 mapping table (ID, SourceID, URL, Title, Body, Snippet,
  PublishedAt, RetrievedAt non-zero, Author, Score within
  `[normalizeScore(rawScore) ± 0.001]`, Lang="", DocType=DocTypePost,
  Citations=nil, Metadata keys present).
- `TestParseListingFiltersNonT3Kinds`: fixture with kind values
  `["t1","t3","t5","t3"]` returns 2 docs (the two `t3` entries).
- `TestParseListingPaginationCursor`: fixture with
  `data.after = "t3_xyz"` returns docs whose
  `[len-1].Metadata["next_cursor"] == "t3_xyz"`. Earlier docs
  do NOT have the `next_cursor` key.
- `TestParseListingNoCursorOnEmpty`: fixture with `data.after = null`
  returns docs with no `next_cursor` key on any of them.
- `TestParseListingHashEmpty`: every returned `NormalizedDoc.Hash`
  equals `""`.
- `TestParseListingMetadataKeys`: each returned doc's Metadata has at
  least `{subreddit, over_18, num_comments, upvote_ratio,
  external_url, kind}`.

### REQ-ADP-007 — NSFW Filter

- `TestSearchNSFWFilterTrueIncludesOver18`,
  `TestSearchNSFWFilterFalseExcludes`,
  `TestSearchNSFWFilterAbsentDefaultsExclude`,
  `TestSearchNSFWFilterUnknownValueDefaultsExclude` each inspect the
  captured request URL's `include_over_18` parameter and assert the
  documented value.

### REQ-ADP-008 — Empty/Whitespace Query Rejection

- `TestSearchEmptyQueryRejectedNoHTTP` table-drives `Text` over
  `["", "   ", "\t\n  \r"]`; for each asserts
  `errors.Is(err, types.ErrPermanent)` AND the underlying cause via
  `errors.Is(err, ErrInvalidQuery)`. The httptest.Server is
  instrumented with a request counter; assert exactly 0 requests.

### REQ-ADP-009 — User-Agent and Accept Headers

- `TestSearchSetsCustomUserAgent`: captured request header
  `User-Agent` starts with `"usearch/"` and contains
  `"(+https://github.com/elymas/universal-search)"`.
- `TestSearchSetsAcceptJSON`: captured `Accept` header equals
  `"application/json"`.
- `TestSearchUserAgentVersionConfigurable`: `Options.UserAgentVersion
  = "v0.2-rc1"` → captured `User-Agent` contains `"usearch/v0.2-rc1"`.

### REQ-ADP-010 — Redirect Allowlist

- `TestSearchFollowsAllowlistRedirect`: server A returns 302 with
  Location header pointing to server B (Host header rewritten to
  `www.reddit.com`); the test installs server B as a custom
  `http.RoundTripper` resolver. Assert search succeeds and returns
  the body from server B.
- `TestSearchRejectsCrossDomainRedirect`: server A returns 302 with
  Location `https://attacker.com/x`. Assert
  `errors.Is(err, types.ErrPermanent)` and error message contains
  `"cross-domain redirect"`.
- `TestSearchRejectsRedirectChainOver3`: 4 servers chained within
  the allowlist; assert error returned after 3 hops with message
  containing `"too many redirects"`.

### REQ-ADP-011 — Concurrent Search Safety (State-Driven)

- `TestSearchConcurrentSafe`: a single `*Adapter` is constructed
  pointing at one `httptest.Server` (which records every inbound
  request). 50 goroutines are launched, each calling
  `(*Adapter).Search(ctx, q)` exactly once with the same query.
  All goroutines start via a `sync.WaitGroup` barrier so the
  invocations overlap.
- Assertions:
  1. The test executes successfully under `go test -race`; the
     race detector reports zero data-race alarms attributable to
     the adapter package.
  2. The stub server's request counter equals 50 (one HTTP
     round-trip per goroutine; no sharing, no caching, no retry).
  3. Every goroutine receives `(docs, nil)` with `len(docs) == 25`
     (matching the standard `search_response.json` fixture); each
     returned `[]types.NormalizedDoc` slice has every doc passing
     `Validate()` returning nil. No goroutine receives an error
     attributable to concurrent state corruption.
- Rationale: this REQ crystallises the concurrency contract that
  the registry (`internal/adapters/registry.go:172-263`
  wrappedAdapter) and the future fanout layer (SPEC-FAN-001) rely
  on but the adapter SPEC did not previously state explicitly. The
  adapter holds no per-call state; `*http.Client` is documented as
  goroutine-safe in the Go stdlib; this test makes the contract
  testable.

### NFR-ADP-001 — Parse-Path Performance

- `BenchmarkParseListing25Docs` is invoked as
  `go test -bench=BenchmarkParseListing25Docs -benchtime=10x -count=5 ./internal/adapters/reddit/...`
  on amd64.
- Assertion mechanism: take the 5 reported per-op mean wall-clock
  durations (one per `-count` run); the MEDIAN of those 5 values
  SHALL be ≤ 5 ms. PASS/FAIL is decidable from the `go test -bench`
  output alone — no external CI script required.
- The bench reports `B/op` and `allocs/op`; `allocs/op` ≤ 500 (= 20 ×
  25 docs). See NFR-ADP-001 for the floor analysis explaining why the
  original ≤ 10/doc target was tightened to ≤ 20/doc during run-phase
  empirical measurement.

### NFR-ADP-002 — E2E p95 (Stub)

- `TestSearchE2ELatencyStubP95` runs 100 invocations against the
  stub `httptest.Server`, sorts elapsed durations, asserts
  `durations[94] ≤ 200ms`.

### NFR-ADP-003 — Goroutine Leak Check

- `TestSearchNoGoroutineLeakOnCancel`: `goleak.VerifyNone(t)`
  succeeds after a `Search` call whose ctx was cancelled at 50ms
  while the stub server delays response by 200ms.

---

## 6. Technical Approach

### 6.1 Files to Modify

**Created (12 files)**:
- `internal/adapters/reddit/reddit.go` — Adapter struct, New, Name, Capabilities, Healthcheck, compile-time interface assertion
- `internal/adapters/reddit/reddit_test.go` — interface conformance tests
- `internal/adapters/reddit/search.go` — Search method (the hot path)
- `internal/adapters/reddit/search_test.go` — main test file (largest)
- `internal/adapters/reddit/client.go` — HTTP client construction, doRequest, categorizeStatus
- `internal/adapters/reddit/client_test.go` — error mapping + redirect tests
- `internal/adapters/reddit/parse.go` — parseListing transform
- `internal/adapters/reddit/parse_test.go` — field mapping tests
- `internal/adapters/reddit/score.go` — normalizeScore Tanh formula
- `internal/adapters/reddit/score_test.go` — score normalization tests
- `internal/adapters/reddit/errors.go` — ErrInvalidQuery sentinel + parseRetryAfter helper
- `internal/adapters/reddit/bench_test.go` — NFR-ADP-001 benchmark
- `internal/adapters/reddit/testdata/search_response.json` (~5KB)
- `internal/adapters/reddit/testdata/search_response_empty.json` (~500B)
- `internal/adapters/reddit/testdata/search_response_pagination.json` (~5KB)
- `internal/adapters/reddit/testdata/search_response_with_nsfw.json` (~5KB)
- `internal/adapters/reddit/testdata/search_response_deleted_post.json` (~2KB)
- `internal/adapters/reddit/testdata/search_response_malformed.json` (~200B)

**Modified**: none. The adapter self-contains. No cross-package
changes are required: `pkg/types` already publishes the contract,
`internal/adapters/registry.go` already accepts any `types.Adapter`,
`internal/obs/metrics/metrics.go` already declares `AdapterCalls` and
`AdapterCallDuration` collectors with `adapter` and `outcome` in the
cardinality allowlist.

**Unchanged (by design)**:
- `internal/adapters/registry.go` (lines 172-263) — wrappedAdapter
  emits ALL observability for ADP-001's `Search` calls. The adapter
  itself emits nothing.
- `pkg/types/{adapter.go, capabilities.go, query.go,
  normalized_doc.go, errors.go}` — no contract change required;
  ADP-001 consumes the existing API.
- `internal/obs/metrics/metrics.go` — no new metric family.
- `cmd/usearch/main.go` — registry construction and adapter
  registration is owned by SPEC-CLI-001 (M2). ADP-001 does not modify
  cmd code.

### 6.2 Package Layout

```
internal/adapters/reddit/
├── reddit.go                                # Adapter, New, Name, Capabilities, Healthcheck, interface assertion
├── reddit_test.go                           # Interface conformance + Capabilities determinism
├── search.go                                # (*Adapter).Search hot path
├── search_test.go                           # E2E + happy path + error categorisation tests
├── client.go                                # *http.Client, doRequest, categorizeStatus
├── client_test.go                           # categorizeStatus table + redirect allowlist
├── parse.go                                 # parseListing transform
├── parse_test.go                            # Field mapping table tests
├── score.go                                 # normalizeScore (Tanh formula)
├── score_test.go                            # Score normalization table
├── errors.go                                # ErrInvalidQuery sentinel + parseRetryAfter helper
├── bench_test.go                            # BenchmarkParseListing25Docs
└── testdata/
    ├── search_response.json                 # Happy path 25 posts
    ├── search_response_empty.json           # Zero children
    ├── search_response_pagination.json      # data.after set
    ├── search_response_with_nsfw.json       # Mixed NSFW + safe
    ├── search_response_deleted_post.json    # [deleted] author
    └── search_response_malformed.json       # Truncated JSON
```

### 6.3 Reddit JSON → NormalizedDoc Field Mapping

| Reddit JSON Field | NormalizedDoc Field | Transform |
|-------------------|---------------------|-----------|
| `data.name` | `ID` | Use as-is (e.g., `t3_abc123xyz`) |
| (constant) | `SourceID` | `"reddit"` (matches `Name()`) |
| `data.permalink` | `URL` | `"https://www.reddit.com" + permalink` |
| `data.title` | `Title` | Use as-is |
| `data.selftext` | `Body` | Use as-is (empty for link posts) |
| `data.selftext` (truncated) | `Snippet` | First 280 runes; if longer, append "..."; if empty, derive from `data.title` truncated similarly |
| `data.created_utc` | `PublishedAt` | `time.Unix(int64(v), 0).UTC()` |
| (parse time) | `RetrievedAt` | `time.Now().UTC()` (set by `parseListing` caller) |
| `data.author` | `Author` | Use as-is (may be `"[deleted]"`) |
| `data.score` | `Score` | `normalizeScore(int(score))` per §2.3 |
| (constant) | `Lang` | `""` (Reddit has no per-post language field) |
| (constant) | `DocType` | `types.DocTypePost` |
| (nil) | `Citations` | `nil` |
| (constructed) | `Metadata` | Map containing two key tiers (resolves Open Question §11.7 inline). **REQUIRED keys** (consumers MAY rely on presence and stable shape; changes require a major-version bump of `Capabilities.Notes` and downstream coordination): `subreddit`, `over_18`, `num_comments`, `upvote_ratio`, `external_url` (= `data.url`), `kind` (= `"t3"`). REQ-ADP-006 enforces these 6 as the contractual minimum. **OPTIONAL keys** (MAY be present; consumers SHALL NOT assume presence; subject to change without major-version bump): `subreddit_name_prefixed`, `ups`, `spoiler`, `locked`, `stickied`, `link_flair_text`, `post_hint`. The LAST returned doc additionally gets `next_cursor` (REQUIRED on the last doc only) if `data.after != ""`. |
| (constant) | `Hash` | `""` (consumers compute via `CanonicalHash()`) |

### 6.4 Type Sketches (illustrative; final shapes in run phase)

```go
// internal/adapters/reddit/reddit.go
package reddit

import (
    "context"
    "net/http"

    "github.com/elymas/universal-search/internal/obs/reqid"
    "github.com/elymas/universal-search/pkg/types"
)

const (
    defaultBaseURL           = "https://www.reddit.com/search.json"
    defaultUserAgentTemplate = "usearch/%s (+https://github.com/elymas/universal-search)"
    defaultUAVersion         = "v0.1"
    defaultHealthcheckTarget = "www.reddit.com:443"
)

type Options struct {
    BaseURL           string        // default: defaultBaseURL (test override)
    HTTPClient        *http.Client  // default: 10s timeout, allowlist redirect, reqid transport
    UserAgentVersion  string        // default: "v0.1"
    HealthcheckTarget string        // default: "www.reddit.com:443"; tests substitute a loopback httptest.Server address
}

type Adapter struct {
    httpClient        *http.Client
    baseURL           string
    userAgent         string
    healthcheckTarget string // injectable test seam; defaults to "www.reddit.com:443"
}

func New(opts Options) (*Adapter, error) {
    base := opts.BaseURL
    if base == "" {
        base = defaultBaseURL
    }
    ua := fmt.Sprintf(defaultUserAgentTemplate, firstNonEmpty(opts.UserAgentVersion, defaultUAVersion))
    client := opts.HTTPClient
    if client == nil {
        client = newDefaultClient()
    }
    target := opts.HealthcheckTarget
    if target == "" {
        target = defaultHealthcheckTarget // "www.reddit.com:443"
    }
    return &Adapter{
        httpClient:        client,
        baseURL:           base,
        userAgent:         ua,
        healthcheckTarget: target,
    }, nil
}

func (a *Adapter) Name() string { return "reddit" }

func (a *Adapter) Capabilities() types.Capabilities {
    return types.Capabilities{
        SourceID:          "reddit",
        DisplayName:       "Reddit",
        DocTypes:          []types.DocType{types.DocTypePost},
        SupportedLangs:    nil, // language-agnostic
        SupportsSince:     false,
        RequiresAuth:      false,
        AuthEnvVars:       nil,
        RateLimitPerMin:   10, // research.md §1.7 conservative public-endpoint figure
        DefaultMaxResults: 25,
        Notes: "Reddit public no-auth search.json endpoint. NSFW excluded by default; " +
            "set Query.Filters[{nsfw, true}] to include. t=all hardcoded (time-range " +
            "filter deferred). Rate limit discrepancy: 10/min unauth (this SPEC) vs " +
            "60/min in tech.md (sync follow-up needed).",
    }
}

func (a *Adapter) Healthcheck(ctx context.Context) error {
    // TCP-connect probe. Cheap; low-value but matches research.md §7.1 default.
    // a.healthcheckTarget is set from Options.HealthcheckTarget at construction
    // time (default "www.reddit.com:443"); tests inject a loopback address.
    var d net.Dialer
    conn, err := d.DialContext(ctx, "tcp", a.healthcheckTarget)
    if err != nil {
        return err
    }
    return conn.Close()
}

// Compile-time interface assertion.
var _ types.Adapter = (*Adapter)(nil)
```

```go
// internal/adapters/reddit/client.go
func newDefaultClient() *http.Client {
    return &http.Client{
        Timeout:       10 * time.Second,
        Transport:     reqid.NewTransport(http.DefaultTransport),
        CheckRedirect: redirectAllowlist,
    }
}

var allowedRedirectHosts = map[string]struct{}{
    "www.reddit.com": {},
    "old.reddit.com": {},
    "new.reddit.com": {},
    "reddit.com":     {},
}

func redirectAllowlist(req *http.Request, via []*http.Request) error {
    if len(via) >= 3 {
        return errors.New("reddit: too many redirects (max 3)")
    }
    if _, ok := allowedRedirectHosts[req.URL.Host]; !ok {
        return fmt.Errorf("reddit: cross-domain redirect rejected: %s", req.URL.Host)
    }
    return nil
}

// categorizeStatus maps an HTTP status (and optional underlying error) into
// a *types.SourceError. retryAfter is honoured only for 429 responses.
func categorizeStatus(status int, retryAfter time.Duration, cause error) *types.SourceError {
    se := &types.SourceError{Adapter: "reddit", HTTPStatus: status, Cause: cause}
    switch {
    case status == 429:
        se.Category = types.CategoryRateLimited
        se.RetryAfter = retryAfter
    case status >= 400 && status < 500:
        se.Category = types.CategoryPermanent
    case status >= 500 && status < 600:
        se.Category = types.CategoryUnavailable
    case status == 0: // network-layer
        se.Category = types.CategoryUnavailable
    default:
        // 1xx/2xx/3xx unexpected here (Search consumes 2xx body before calling)
        se.Category = types.CategoryUnknown
    }
    return se
}
```

### 6.5 HTTP Client Construction Notes

- **Timeout**: 10 seconds total request deadline (default). Caller's
  ctx deadline takes precedence when shorter.
- **Redirect policy**: `CheckRedirect` enforces the allowlist
  `{www.reddit.com, old.reddit.com, new.reddit.com, reddit.com}` and
  caps at 3 hops. Cross-domain redirects return an error from
  `CheckRedirect` (Go stdlib then wraps in `*url.Error` which the
  adapter unwraps + re-wraps as `*SourceError{CategoryPermanent}`).
- **Transport**: `reqid.NewTransport(http.DefaultTransport)` (mirrors
  `internal/llm/client.go:51-54`) propagates the request ID from
  context to outbound headers. Required for observability correlation.
- **Headers per request**: `User-Agent: usearch/<version>
  (+https://github.com/elymas/universal-search)` and `Accept:
  application/json`. NO authentication header (public endpoint).

### 6.6 Observability Note

The Reddit adapter emits ZERO metrics, logs, and spans of its own.
ALL observability comes from the registry's `wrappedAdapter`
(`internal/adapters/registry.go:195-252`), which on every `Search`
call emits:
- one OTel span `adapter.search` with attributes `adapter.name`,
  `adapter.outcome`, `adapter.result_count`
- one Prometheus counter increment on
  `AdapterCalls{adapter="reddit",outcome=<...>}`
- one Prometheus histogram observation on
  `AdapterCallDuration{adapter="reddit"}`
- one slog record at INFO (success) or WARN (non-success)

The adapter's responsibility is to return a correctly-categorised
`*types.SourceError` so the wrappedAdapter computes the right
`outcome` label via `types.OutcomeFromError(err)` (see
`pkg/types/errors.go:174-193`):
- `nil` → `"success"`
- `context.DeadlineExceeded` → `"timeout"`
- `CategoryRateLimited` → `"rate_limited"`
- `CategoryUnavailable` → `"unavailable"`
- `CategoryTransient` → `"transient"`
- `CategoryPermanent` / unknown → `"failure"`

This sole-emitter discipline is the same pattern that all other M3
adapters will follow.

### 6.7 MX Tag Plan

| File::Symbol | Tag | Reason |
|--------------|-----|--------|
| `reddit.go::(*Adapter).Search` | `@MX:ANCHOR` | Sole entry point for all Reddit fanout calls. fan_in ≥ 3 (registry wrappedAdapter, FAN-001 fanout, tests). `@MX:REASON: contract boundary; signature change ripples to FAN-001 + CLI-001 + SYN-001`. |
| `parse.go::parseListing` | `@MX:ANCHOR` | Every Reddit doc passes through this single transform. fan_in = 1 (Search) but invariant-bearing — bug here corrupts every NormalizedDoc returned. `@MX:REASON: NormalizedDoc field-mapping integrity gate`. |
| `score.go::normalizeScore` (function) and constants `tanhDivisor=100.0, scoreCenter=0.5` (declared in same file) | `@MX:NOTE` | Documents the Tanh formula choice and tie-in to SPEC-IDX-001 RRF. The function gets a doc-comment `@MX:NOTE` explaining the empirical inflection-point at score=100; the two constants get inline `@MX:NOTE` annotations citing Open Question §11.2 revisit triggers. Single logical concern, two adjacent annotation sites. |
| `errors.go::categorizeStatus` (helper, defined in client.go) | `@MX:NOTE` | The HTTP-status-to-Category rosetta stone. Future contributors will look here first when a new HTTP code needs handling. |
| `client.go::doRequest` | `@MX:WARN` | Outbound network call. Redirect allowlist enforces SSRF safety boundary. `@MX:REASON: removing the CheckRedirect guard re-opens SSRF`. |
| `client.go::allowedRedirectHosts` map | `@MX:NOTE` | The 4-entry redirect allowlist. Adding a host requires a security review. |

All tags are `[AUTO]`-prefixed (agent-generated), include
`@MX:SPEC: SPEC-ADP-001`, and follow `code_comments: en` per
`.moai/config/sections/language.yaml`.

### 6.8 Harness Level

Per `.moai/config/sections/harness.yaml` auto-routing: 11 EARS REQs
(9 × P0 + 2 × P1) + 3 NFRs touching 1 package (10 source files +
6 testdata fixtures) + zero cross-package edits + zero security/
payment/PII keywords + zero compose/env/config deltas =
**standard** harness level. Sprint Contract is OPTIONAL but
recommended. Evaluator profile `default` applies.

---

## 7. What NOT to Build (Exclusions)

[HARD] This SPEC explicitly excludes the following items. Each has a
known destination SPEC; this list prevents scope creep into ADP-001
and reaffirms the reference-shape discipline.

- **Per-source customisations for HN, arXiv, GitHub, YouTube,
  Bluesky, X, SearXNG, Naver, Daum, KoreaNewsCrawler, RSS,
  Polymarket** → SPEC-ADP-002 (M2), SPEC-ADP-003..009 (M3). Each
  source has its own auth, rate-limit, pagination, and field-mapping
  quirks; ADP-001 is the reference shape that all 12 will copy
  structurally.
- **Retry orchestration** (exponential backoff, circuit breaker,
  per-source health-state tracking, jitter) → SPEC-FAN-001 (M3).
  Adapter is one-shot per call.
- **Response caching** (in-process LRU, Redis, on-disk fixture cache)
  → SPEC-CACHE-001 (M3). Adapter is stateless.
- **Result ranking, deduplication, RRF fusion across adapters** →
  SPEC-IDX-001 (M3). Adapter returns Reddit-relevance order with the
  Tanh-normalised Score; cross-adapter ranking is fusion's job.
- **Tenant-scoped NSFW policy** (per-team override of the
  `include_over_18` default, audit-logged opt-in, age verification
  hooks) → SPEC-AUTH-002 (M6). v0.1 honours `Query.Filters` literally.
- **Adapter health-state machine** (auto-disable on N consecutive
  failures, auto-re-enable on Healthcheck pass, weighted reliability
  score) → SPEC-EVAL-002 (M8).
- **OAuth-authenticated variant** (`oauth.reddit.com`, 60/min limit,
  per-team Reddit app credentials) → future SPEC-ADP-001a if measured
  value warrants.
- **Subreddit-scoped search** (`/r/{sub}/search.json` +
  `restrict_sr=on`) → out of v0.1; future enhancement SPEC.
- **Time-range filtering** (Reddit `t=hour|day|week|month|year`) →
  out of v0.1.
- **Sort customisation** (Reddit `sort=hot|new|top|comments`) → out
  of v0.1; hardcoded `relevance`.
- **Comment retrieval** (`t1_*` kinds) → out of scope.
- **Live network integration tests in CI** → out of v0.1; httptest
  + golden fixtures only.
- **Per-adapter custom Prometheus metrics** (e.g.,
  `reddit_pagination_pages_total`) → would require amending
  SPEC-OBS-001's allowlist. Out of v0.1; the shared
  `AdapterCalls{adapter,outcome}` family is sufficient.
- **Korean-locale handling for Reddit** → SPEC-IDX-003 (M3) +
  SPEC-ADP-008/009 own Korean-source adapters; Reddit returns
  Lang="" (unknown).
- **Streaming Search results** (channel-based incremental delivery)
  → SPEC-SYN-004 (M4) if measured value.

---

## 8. TDD Plan

Development follows RED-GREEN-REFACTOR per
`quality.development_mode: tdd` (`.moai/config/sections/quality.yaml`).
Representative RED-phase tests, written before implementation,
grouped by REQ. Total: 46 tests (45 covering REQ-ADP-001..010 +
NFRs, plus 1 added in iteration 2 for REQ-ADP-011 concurrency
safety). Coverage target: 85% per
`quality.test_coverage_target`. Benchmarks do not count toward
coverage.

| # | Test | Layer | REQ | Assertion |
|---|------|-------|-----|-----------|
| 1 | `TestAdapterName` | `reddit_test.go` | REQ-ADP-001 | `(*Adapter).Name() == "reddit"` |
| 2 | `TestAdapterImplementsInterface` | `reddit_test.go` | REQ-ADP-001 | Compile-time `var _ types.Adapter = (*Adapter)(nil)` succeeds |
| 3 | `TestCapabilitiesDeterministic` | `reddit_test.go` | REQ-ADP-001 | Two consecutive `Capabilities()` calls return `reflect.DeepEqual` results |
| 4 | `TestCapabilitiesShape` | `reddit_test.go` | REQ-ADP-001 | All 10 documented field values match (SourceID, DisplayName, DocTypes, RequiresAuth, AuthEnvVars, SupportsSince, RateLimitPerMin, DefaultMaxResults, plus Notes substring contains) |
| 5 | `TestHealthcheckSucceeds` | `reddit_test.go` | REQ-ADP-001 | TCP dial against test loopback succeeds |
| 6 | `TestSearchHappyPath25Docs` | `search_test.go` | REQ-ADP-002, REQ-ADP-006 | 25 NormalizedDocs returned; each `Validate()` returns nil |
| 7 | `TestSearchURLParametersIncludeAllRequired` | `search_test.go` | REQ-ADP-002 | Captured request URL has `q`, `sort`, `t`, `type`, `limit`, `include_over_18` (and optionally `after`) params |
| 8 | `TestSearchClampsLimitTo100` | `search_test.go` | REQ-ADP-002 | q.MaxResults=500 → URL has `limit=100` |
| 9 | `TestSearchDefaultsLimitTo25` | `search_test.go` | REQ-ADP-002 | q.MaxResults=0 → URL has `limit=25` |
| 10 | `TestSearchHonoursCursorParameter` | `search_test.go` | REQ-ADP-002 | q.Cursor="t3_abc" → URL contains `&after=t3_abc` |
| 11 | `TestSearchHTTP429WithIntegerRetryAfter` | `search_test.go` | REQ-ADP-003 | `Retry-After: 30` → SourceError.RetryAfter==30s |
| 12 | `TestSearchHTTP429WithHTTPDateRetryAfter` | `search_test.go` | REQ-ADP-003 | HTTP-date 30s ahead → RetryAfter ∈ (25s, 35s) |
| 13 | `TestSearchHTTP429NoRetryAfterDefaults5s` | `search_test.go` | REQ-ADP-003 | No header → RetryAfter==5s |
| 14 | `TestSearchHTTP429RetryAfterCapped60s` | `search_test.go` | REQ-ADP-003 | `Retry-After: 999` → RetryAfter==60s |
| 15 | `TestSearchHTTP429NoInternalRetry` | `search_test.go` | REQ-ADP-003 | Server request count == 1 |
| 16 | `TestSearchHTTP401` / `403` / `404` | `search_test.go` | REQ-ADP-004 | `errors.Is(err, types.ErrPermanent)`; HTTPStatus matches |
| 17 | `TestSearchHTTP500` / `503` | `search_test.go` | REQ-ADP-005 | `errors.Is(err, types.ErrSourceUnavailable)`; HTTPStatus matches |
| 18 | `TestSearchConnectionRefused` | `search_test.go` | REQ-ADP-005 | `errors.Is(err, types.ErrSourceUnavailable)`; HTTPStatus==0 |
| 19 | `TestSearchUnavailablePreservesUnderlyingError` | `search_test.go` | REQ-ADP-005 | `errors.Unwrap(srcErr).Error()` contains inner cause text |
| 20 | `TestParseListingFieldMapping` | `parse_test.go` | REQ-ADP-006 | Table over 5 fixtures; every documented field maps correctly |
| 21 | `TestParseListingFiltersNonT3Kinds` | `parse_test.go` | REQ-ADP-006 | Mixed kinds → only t3 docs returned |
| 22 | `TestParseListingPaginationCursor` | `parse_test.go` | REQ-ADP-006 | `data.after` set → last doc Metadata["next_cursor"] populated |
| 23 | `TestParseListingNoCursorOnEmpty` | `parse_test.go` | REQ-ADP-006 | `data.after = null` → no doc has `next_cursor` key |
| 24 | `TestParseListingHashEmpty` | `parse_test.go` | REQ-ADP-006 | Every NormalizedDoc.Hash == "" |
| 25 | `TestParseListingMetadataKeys` | `parse_test.go` | REQ-ADP-006 | All 6 required Metadata keys present |
| 26 | `TestParseListingDeletedAuthor` | `parse_test.go` | REQ-ADP-006 | Author="[deleted]" returned as-is; Validate() still passes |
| 27 | `TestParseListingMalformedJSON` | `parse_test.go` | REQ-ADP-006 | Truncated JSON → `*SourceError{Category: CategoryPermanent}` |
| 28 | `TestSearchNSFWFilterTrueIncludesOver18` | `search_test.go` | REQ-ADP-007 | Filters=[{nsfw, true}] → URL has `include_over_18=true` |
| 29 | `TestSearchNSFWFilterFalseExcludes` | `search_test.go` | REQ-ADP-007 | Filters=[{nsfw, false}] → URL has `include_over_18=false` |
| 30 | `TestSearchNSFWFilterAbsentDefaultsExclude` | `search_test.go` | REQ-ADP-007 | Filters=nil → URL has `include_over_18=false` |
| 31 | `TestSearchNSFWFilterUnknownValueDefaultsExclude` | `search_test.go` | REQ-ADP-007 | Filters=[{nsfw, maybe}] → URL has `include_over_18=false` |
| 32 | `TestSearchEmptyQueryRejectedNoHTTP` | `search_test.go` | REQ-ADP-008 | Table over 4 empty/whitespace inputs; assert ErrPermanent + zero requests |
| 33 | `TestSearchSetsCustomUserAgent` | `client_test.go` | REQ-ADP-009 | UA starts with "usearch/" + contains URL |
| 34 | `TestSearchSetsAcceptJSON` | `client_test.go` | REQ-ADP-009 | `Accept: application/json` header present |
| 35 | `TestSearchUserAgentVersionConfigurable` | `client_test.go` | REQ-ADP-009 | Options override propagates to UA header |
| 36 | `TestSearchFollowsAllowlistRedirect` | `client_test.go` | REQ-ADP-010 | 302 within allowlist followed |
| 37 | `TestSearchRejectsCrossDomainRedirect` | `client_test.go` | REQ-ADP-010 | 302 to `attacker.com` → ErrPermanent |
| 38 | `TestSearchRejectsRedirectChainOver3` | `client_test.go` | REQ-ADP-010 | 4-hop chain rejected |
| 39 | `TestNormalizeScoreTable` | `score_test.go` | REQ-ADP-006 | 7 score values → expected `[0,1]` outputs within ±0.001 |
| 40 | `TestNormalizeScoreDeterministic` | `score_test.go` | REQ-ADP-006 | Two calls on same input return byte-equal output |
| 41 | `TestParseRetryAfterTable` | `client_test.go` | REQ-ADP-003 | Table over 6 inputs (int, HTTP-date, missing, malformed, > 60, negative) |
| 42 | `TestCategorizeStatusTable` | `client_test.go` | REQ-ADP-003/004/005 | Truth table over 7 status codes (200/401/403/404/429/500/503/0) → expected Category |
| 43 | `TestSearchE2ELatencyStubP95` | `search_test.go` | NFR-ADP-002 | 100 invocations against stub; p95 ≤ 200ms |
| 44 | `TestSearchNoGoroutineLeakOnCancel` | `search_test.go` | NFR-ADP-003 | `goleak.VerifyNone(t)` after mid-flight ctx cancel |
| 45 | `BenchmarkParseListing25Docs` | `bench_test.go` | NFR-ADP-001 | Median of 5 `-count` runs at `-benchtime=10x` is ≤ 5ms per op; allocs/op ≤ 500 (revised from ≤ 250 during run phase per HISTORY iteration 3) |
| 46 | `TestSearchConcurrentSafe` | `search_test.go` | REQ-ADP-011 | 50 goroutines call Search on shared `*Adapter` against one stub; race-detector clean (`-race`); stub observes 50 requests; every goroutine receives 25 valid `NormalizedDoc`s |

RED-GREEN-REFACTOR per requirement:
1. RED: Write failing test for REQ-ADP-N.
2. GREEN: Implement minimal code to pass.
3. REFACTOR: Tidy; extract shared helpers if they remove duplication;
   keep file sizes manageable (target each `.go` file < 200 LoC
   excluding tests).

Greenfield note: `internal/adapters/reddit/` does not exist. There
is no behaviour to preserve; no characterization tests needed.

---

## 9. Dependencies

### 9.1 Upstream SPEC Dependencies

- **SPEC-CORE-001 (implemented; merged commit f728aa2)**: provides
  `pkg/types.Adapter`, `pkg/types.Capabilities`, `pkg/types.Query`,
  `pkg/types.NormalizedDoc`, `*types.SourceError`,
  `types.OutcomeFromError`, `types.DocType` enum,
  `internal/adapters.Registry` with wrappedAdapter sole-emitter
  pattern, `internal/adapters/noop` reference shape. HARD dep.
- **SPEC-OBS-001 (implemented)**: provides `obs.Obs` bundle,
  `internal/obs/reqid.NewTransport` for request-ID propagation,
  `AdapterCalls{adapter,outcome}` and `AdapterCallDuration{adapter}`
  collectors with `adapter` and `outcome` already in cardinality
  allowlist. ADP-001 consumes `reqid.NewTransport` directly; the
  registry wrappedAdapter consumes the rest. SOFT dep — adapter is
  nil-safe via the registry's nil-guards.
- **SPEC-IR-001 (implemented; merged commit 8a20b68)**: documents
  the consumer contract for `Capabilities` (REQ-IR-008 selects
  AdapterSet by intersecting `categoryEligibleDocTypes` with
  `SupportedLangs`). ADP-001's `Capabilities()` shape (DocTypes,
  SupportedLangs) determines which routing categories the Reddit
  adapter will be selected for. SOFT dep — IR-001 lookups happen at
  startup; ADP-001 just declares its capability.

### 9.2 Parallelizable

- **SPEC-ADP-002 (Hacker News, M2)**: can begin its plan phase as
  soon as ADP-001's spec.md is approved. ADP-002 will copy the file
  layout, error-mapping discipline, and TDD harness verbatim and
  customise for HN's Algolia API quirks.
- **SPEC-CLI-001 (M2)**: can plan in parallel; CLI-001 wires the
  adapter into `cmd/usearch/main.go` registry construction.
  Depends on ADP-001's `New(opts) (*Adapter, error)` constructor
  signature being approved.
- **SPEC-SYN-001 (M2)**: can plan in parallel; synthesis consumes
  `[]types.NormalizedDoc` shape (already locked in CORE-001), so
  ADP-001 doesn't add new constraints.

### 9.3 Downstream Blocked SPECs

- **SPEC-ADP-002** (M2): copies ADP-001's file layout and TDD
  harness as the starting point.
- **SPEC-FAN-001** (M3): consumes `(*reddit.Adapter).Search` via
  `registry.Get("reddit").Search(ctx, q)` and orchestrates retry
  on `errors.Is(err, types.ErrTransient)` /
  `errors.Is(err, types.ErrRateLimited)`.
- **SPEC-CLI-001** (M2): wires the adapter into the M2 exit-criterion
  demonstration.
- **SPEC-SYN-001** (M2): consumes `[]NormalizedDoc` for citation
  assembly via the gpt-researcher Python sidecar.
- **SPEC-ADP-003..009** (M3): copy the reference shape.
- **SPEC-IDX-001** (M3): consumes `NormalizedDoc.Score` (Tanh-normalised
  in ADP-001) as one input to RRF fusion across adapters.

### 9.4 External Dependencies (run-phase pins)

**Zero new Go module dependencies.** ADP-001 uses only:
- Go stdlib: `context`, `encoding/json`, `errors`, `fmt`, `math`,
  `net`, `net/http`, `net/url`, `strconv`, `strings`, `time`,
  `unicode`, `unicode/utf8`
- `pkg/types` (already pinned via SPEC-CORE-001)
- `internal/obs/reqid` (already pinned via SPEC-OBS-001)
- Test-only: `go.uber.org/goleak` (for NFR-ADP-003) — verify
  presence in `go.mod`; if absent, the run-phase implementer adds it
  under SPEC-DEP-001's existing dependency-management policy.

---

## 10. Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Default Go `net/http` User-Agent rejected by Reddit (immediate 429) | High | High | REQ-ADP-009 makes custom UA a HARD requirement; integration test (deferred but stubbed in v0.1 acceptance criteria) verifies. UA construction in `New()` is non-bypassable. |
| Rate-limit doc discrepancy (research.md says 10/min unauth; tech.md:106 says 60/min) misleads operators | High | Medium | This SPEC adopts the conservative 10/min figure in `Capabilities.RateLimitPerMin` and documents the discrepancy in `Capabilities.Notes`. Open Question §11.1 tracks the tech.md sync follow-up. |
| Cross-domain redirect (open SSRF) | Medium | High | REQ-ADP-010 + `redirectAllowlist` `CheckRedirect` enforces the 4-host allowlist; cross-domain redirect rejected with `*SourceError{Permanent}`. Test `TestSearchRejectsCrossDomainRedirect` verifies. |
| NSFW filter unreliable (Reddit may return over-18 posts despite `include_over_18=false`) | Medium | Medium | v0.1 trusts Reddit's filter (no client-side post-filter). Document in `Capabilities.Notes`. Future SPEC-AUTH-002 may add tenant-scoped post-filter for safety. Open Question §11.4 tracks empirical verification. |
| `[deleted]` author / `[removed]` body posts surface to consumers | Medium | Low | REQ-ADP-006 returns deleted posts as-is (Author="[deleted]"). Validate() still passes (URL is the permalink, not the deleted content). Consumers can post-filter on Author. Documented in research.md §7.5. |
| Score normalization formula impacts SPEC-IDX-001 RRF behaviour | Medium | Medium | §2.3 locks the Tanh formula precisely with worked examples. Open Question §11.2 documents revisit triggers (post-M3 evaluation). RRF in SPEC-IDX-001 weights rank not raw score, so impact is bounded. |
| Pagination cursor opacity confuses downstream consumers | Low | Low | REQ-ADP-006 surfaces the cursor via `Metadata["next_cursor"]` on the LAST doc. Cursor is opaque; consumers MUST pass it back as `Query.Cursor` without parsing. Documented in `Capabilities.Notes`. |
| `time.Now()` in `RetrievedAt` non-deterministic in tests | Low | Low | `parseListing` accepts `retrievedAt time.Time` parameter; tests inject a fixed time. Search wraps with `time.Now().UTC()` in production. |
| Reddit response envelope changes (Reddit silently adds/removes JSON fields) | Low | Medium | Adapter consumes only the documented field set (research.md §1.4). Unknown fields ignored by `encoding/json`. New fields can be opted-in via Metadata keys without breaking consumers. |
| HTTP timeout (10s) too aggressive for Reddit during incidents | Low | Low | Configurable via `Options.HTTPClient`; default 10s aligns with NFR-ADP-002 stub p95 200ms × 50× safety margin. Caller's ctx deadline takes precedence. |
| `goleak` not present in go.mod | Low | Low | Run-phase implementer adds via `go get go.uber.org/goleak` per SPEC-DEP-001 policy. |
| Reddit returns `kind="t3"` posts mixed with crossposts (`crosspost_parent_list`) | Low | Low | v0.1 ignores crosspost metadata; the post itself is still a `t3` and renders as a normal NormalizedDoc. Crossposts are documented as out-of-scope future enhancement. |

---

## 11. Open Questions

These are explicitly unresolved at SPEC-approval time. Each has a
recommended default and a one-line resolution owner. They do NOT
block SPEC approval.

1. **Rate-limit documentation discrepancy** (10/min in research.md
   §1.7 vs 60/min in tech.md:106). **Recommended default**: adopt
   10/min (conservative, matches 2026 painonsocial.com guidance for
   unauthenticated public access). Update tech.md row "Reddit | …"
   to read "10/min unauth (60/min via OAuth — out of v0.1 scope)".
   **Resolution owner**: docs-sync agent in the next `/moai sync`
   pass after ADP-001 implementation lands.

2. **Score normalization formula revisit** (Tanh divisor 100 vs
   alternative tuning). **Recommended default**: keep Tanh divisor
   100 in v0.1. Revisit after SPEC-IDX-001 RRF integration (M3) if
   ranking quality measurements indicate the inflection point is
   wrong. **Resolution owner**: SPEC-IDX-001 author.

3. **Healthcheck implementation depth** (TCP-connect vs real GET vs
   noop). **Recommended default**: TCP-connect (research.md §7.1).
   Cheap (~100ms), sufficient for SPEC-EVAL-002 reachability check.
   **Resolution owner**: SPEC-EVAL-002 author may upgrade if richer
   signal needed.

4. **NSFW filter empirical verification** (does Reddit honour
   `include_over_18=false` or do NSFW posts leak through?).
   **Recommended default**: trust Reddit in v0.1; no client-side
   post-filter. **Resolution owner**: SPEC-AUTH-002 author when
   implementing tenant-scoped policy in M6 — should add post-filter
   if empirical test reveals leakage.

5. **`[deleted]` post handling policy** (return as-is vs filter out
   vs Author=""). **Recommended default**: return as-is (research.md
   §7.5). Consumers can post-filter on `Author == "[deleted]"`.
   **Resolution owner**: SPEC-SYN-001 author may filter at synthesis
   time if `[deleted]` posts pollute citation output.

6. **Cursor surfacing mechanism** (Metadata["next_cursor"] on last
   doc vs separate response wrapper). **Recommended default**:
   Metadata["next_cursor"] on the last doc (REQ-ADP-006). Backward-
   compatible with `[]NormalizedDoc` return type. **Resolution
   owner**: SPEC-FAN-001 author may request a wrapper if
   multi-adapter pagination becomes complex.

[§11.7 RESOLVED in iteration 2]: The original Metadata key API
surface question has been resolved inline in §6.3 of the mapping
table, which now classifies the 6 REQUIRED keys (`subreddit`,
`over_18`, `num_comments`, `upvote_ratio`, `external_url`, `kind`)
versus the 7 OPTIONAL keys (`subreddit_name_prefixed`, `ups`,
`spoiler`, `locked`, `stickied`, `link_flair_text`, `post_hint`).
`next_cursor` is REQUIRED on the last doc only. The active open
question count is now 6 (questions 1–6 above).

---

## 12. References

### External (URL-cited; verified per research.md §9)

- https://til.simonwillison.net/reddit/scraping-reddit-json — Simon
  Willison's TIL on Reddit JSON API querying and User-Agent
  requirements.
- https://github.com/searxng/searxng/blob/master/searx/engines/reddit.py
  — SearXNG Reddit engine implementation; canonical request/response
  structure reference.
- https://github.com/vartanbeno/go-reddit — go-reddit MIT-licensed
  Go Reddit client library (REJECTED as dependency per research.md
  §5.2; pattern reference only).
- https://painonsocial.com/blog/reddit-api-rate-limits-guide —
  Reddit API rate limits (10/min unauthenticated, 60/min
  authenticated) — basis for §11.1 discrepancy resolution.
- https://painonsocial.com/blog/reddit-api-rate-limits-workaround —
  Strategies for working within Reddit rate limits.
- RFC 7231 §7.1.3 Retry-After header semantics — basis for
  REQ-ADP-003 parser.

### Internal (file:line cited)

- `.moai/specs/SPEC-ADP-001/research.md` — full research artifact
  (791 lines).
- `.moai/specs/SPEC-CORE-001/spec.md` — Adapter / Capabilities /
  Query / NormalizedDoc / SourceError contract.
- `.moai/specs/SPEC-OBS-001/spec.md` — observability bundle and
  cardinality discipline.
- `.moai/specs/SPEC-IR-001/spec.md` — `Capabilities` consumer
  contract (REQ-IR-008).
- `pkg/types/adapter.go:28-45` — Adapter interface.
- `pkg/types/capabilities.go:38-62` — Capabilities struct + DocType
  enum.
- `pkg/types/query.go:18-44` — Query struct + Filter shape.
- `pkg/types/errors.go:14-218` — SourceError, Category enum,
  CategorizeError, OutcomeFromError, ValidationError.
- `pkg/types/normalized_doc.go:40-106` — NormalizedDoc 15-field
  struct, Validate, CanonicalHash.
- `internal/adapters/registry.go:75-167` — Registry lifecycle.
- `internal/adapters/registry.go:172-263` — wrappedAdapter
  sole-emitter pattern.
- `internal/adapters/noop/noop.go:1-46` — reference adapter shape +
  compile-time interface assertion.
- `internal/llm/client.go:31-65` — HTTP client construction pattern
  with timeout + reqid Transport wrapping.
- `.moai/project/roadmap.md:36-39, 117-122, 147` — M2 milestone,
  parallelization, exit criterion.
- `.moai/project/structure.md:18-22` — `internal/adapters/reddit/`
  reservation.
- `.moai/project/tech.md:102-107` — per-source adapter strategy
  (Reddit row; rate-limit discrepancy source).
- `.moai/config/sections/quality.yaml` — `development_mode: tdd`,
  `test_coverage_target: 85`.
- `.moai/config/sections/harness.yaml` — auto-routing to standard
  level.
- `.moai/config/sections/language.yaml` — `documentation: en`,
  `code_comments: en`.

---

*End of SPEC-ADP-001 v0.1*
