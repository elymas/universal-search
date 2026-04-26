# SPEC-ADP-001 Acceptance Criteria

**SPEC**: SPEC-ADP-001 — Reddit Adapter (Reference Implementation)
**Format**: Given/When/Then scenarios per REQ + edge cases + quality gates
**Author**: limbowl (via manager-spec)
**Created**: 2026-04-26

---

## 1. Acceptance Scenarios by REQ

### REQ-ADP-001 — Adapter Interface Conformance

**Scenario S-001-A: Compile-time interface assertion**
- Given the package `internal/adapters/reddit` declares `var _
  types.Adapter = (*Adapter)(nil)` at the bottom of `reddit.go`,
- When `go build ./internal/adapters/reddit/...` runs,
- Then the build succeeds without error. Removing any of the four
  required methods causes a compile error referencing the assertion
  line.

**Scenario S-001-B: Name returns "reddit"**
- Given a constructed `*Adapter` from `New(Options{})`,
- When `(*Adapter).Name()` is called,
- Then the return value equals the literal string `"reddit"` and
  matches `(*Adapter).Capabilities().SourceID`.

**Scenario S-001-C: Capabilities are deterministic**
- Given a constructed `*Adapter`,
- When `Capabilities()` is called twice in succession,
- Then `reflect.DeepEqual(call1, call2) == true`. No call mutates
  internal state.

**Scenario S-001-D: Capabilities shape**
- Given a constructed `*Adapter`,
- When `Capabilities()` is called,
- Then the returned struct has `SourceID="reddit"`,
  `DisplayName="Reddit"`, `DocTypes=[DocTypePost]`,
  `RequiresAuth=false`, `AuthEnvVars=nil` or empty,
  `SupportsSince=false`, `RateLimitPerMin=10`,
  `DefaultMaxResults=25`, and `Notes` contains the substrings
  `"public no-auth"`, `"NSFW excluded by default"`, `"t=all"`,
  `"rate limit discrepancy"`.

**Scenario S-001-E: Healthcheck succeeds against reachable host**
- Given a TCP listener at `127.0.0.1:<port>` (test loopback)
  configured as the Healthcheck dial target,
- When `Healthcheck(ctx)` is called with a non-cancelled ctx,
- Then it returns `nil`. The opened connection is closed before
  return (no socket leak).

**Scenario S-001-F: Healthcheck honours ctx cancellation**
- Given a non-routable dial target and a ctx with 50ms deadline,
- When `Healthcheck(ctx)` is called,
- Then it returns a non-nil error within 100ms (not blocked on the
  default DNS/TCP timeout).

### REQ-ADP-002 — Search Happy Path and URL Parameters

**Scenario S-002-A: Happy path 25 results**
- Given a stub `httptest.Server` returning the contents of
  `testdata/search_response.json` (a 25-post Listing) with HTTP 200,
- When `Search(ctx, types.Query{Text: "go programming",
  MaxResults: 25})` is called,
- Then the return is `(docs []NormalizedDoc, nil)` with `len(docs)
  == 25`. Every doc passes `Validate()` returning nil. Each doc has
  `SourceID == "reddit"`, `DocType == DocTypePost`, non-empty `ID`
  (starting with `"t3_"`), non-empty `URL` (starting with
  `"https://www.reddit.com/"`), non-zero `RetrievedAt`, empty `Hash`,
  and Metadata with all 6 required keys.

**Scenario S-002-B: URL contains all required parameters**
- Given the stub server captures the inbound request URL,
- When `Search(ctx, types.Query{Text: "rust", MaxResults: 50})` is
  called,
- Then the captured URL contains `q=rust`, `sort=relevance`,
  `t=all`, `type=link`, `limit=50`, `include_over_18=false`. The
  URL host is `www.reddit.com` (or the test stub's host with
  rewritten BaseURL).

**Scenario S-002-C: limit clamped to 100**
- Given `Query.MaxResults = 500`,
- When `Search` is invoked,
- Then the request URL contains `limit=100` (clamped per
  REQ-ADP-002).

**Scenario S-002-D: limit defaults to 25 when MaxResults=0**
- Given `Query.MaxResults = 0`,
- When `Search` is invoked,
- Then the request URL contains `limit=25` (default per
  Capabilities.DefaultMaxResults).

**Scenario S-002-E: Cursor honoured**
- Given `Query.Cursor = "t3_xyz123"`,
- When `Search` is invoked,
- Then the request URL contains `&after=t3_xyz123`.

**Scenario S-002-F: Empty cursor omits after parameter**
- Given `Query.Cursor = ""`,
- When `Search` is invoked,
- Then the request URL does NOT contain an `after` parameter.

### REQ-ADP-003 — HTTP 429 Rate-Limit Mapping

**Scenario S-003-A: 429 with integer Retry-After**
- Given the stub returns HTTP 429 with header `Retry-After: 30`,
- When `Search` is invoked,
- Then the return is `(nil, *types.SourceError{Adapter:"reddit",
  Category: CategoryRateLimited, HTTPStatus: 429, RetryAfter: 30s,
  Cause: <non-nil>})`. `errors.Is(err, types.ErrRateLimited)`
  returns true.

**Scenario S-003-B: 429 with HTTP-date Retry-After**
- Given the stub returns HTTP 429 with `Retry-After: <RFC1123 date 30
  seconds in the future>`,
- When `Search` is invoked,
- Then the returned `*SourceError.RetryAfter` is in the range
  `(25s, 35s)` (allowing 5s test-clock drift).

**Scenario S-003-C: 429 without Retry-After defaults to 5s**
- Given the stub returns HTTP 429 with no `Retry-After` header,
- When `Search` is invoked,
- Then `*SourceError.RetryAfter == 5*time.Second`.

**Scenario S-003-D: Retry-After capped at 60s**
- Given the stub returns HTTP 429 with `Retry-After: 999`,
- When `Search` is invoked,
- Then `*SourceError.RetryAfter == 60*time.Second` (capped per
  REQ-ADP-003).

**Scenario S-003-E: 429 negative or malformed Retry-After defaults to 5s**
- Given the stub returns HTTP 429 with `Retry-After: -10` or
  `Retry-After: not-a-number`,
- When `Search` is invoked,
- Then `*SourceError.RetryAfter == 5*time.Second`.

**Scenario S-003-F: No internal retry on 429**
- Given the stub server counts inbound requests and returns HTTP
  429 on every request,
- When `Search` is invoked once,
- Then exactly 1 request is observed by the stub. The adapter does
  not retry.

### REQ-ADP-004 — HTTP 4xx Permanent Mapping

**Scenario S-004-A: 401 → Permanent**
- Given the stub returns HTTP 401,
- When `Search` is invoked,
- Then `errors.Is(err, types.ErrPermanent)` is true and
  `*SourceError.HTTPStatus == 401`.

**Scenario S-004-B: 403 → Permanent**
- Given the stub returns HTTP 403,
- When `Search` is invoked,
- Then `errors.Is(err, types.ErrPermanent)` is true and
  `*SourceError.HTTPStatus == 403`.

**Scenario S-004-C: 404 → Permanent**
- Given the stub returns HTTP 404,
- When `Search` is invoked,
- Then `errors.Is(err, types.ErrPermanent)` is true and
  `*SourceError.HTTPStatus == 404`.

**Scenario S-004-D: No internal retry on 4xx**
- Given the stub returns HTTP 401 and counts inbound requests,
- When `Search` is invoked once,
- Then exactly 1 request is observed.

### REQ-ADP-005 — HTTP 5xx and Network Failure

**Scenario S-005-A: 500 → Unavailable**
- Given the stub returns HTTP 500,
- When `Search` is invoked,
- Then `errors.Is(err, types.ErrSourceUnavailable)` is true and
  `*SourceError.HTTPStatus == 500`.

**Scenario S-005-B: 503 → Unavailable**
- Given the stub returns HTTP 503,
- When `Search` is invoked,
- Then `errors.Is(err, types.ErrSourceUnavailable)` is true and
  `*SourceError.HTTPStatus == 503`.

**Scenario S-005-C: Connection refused → Unavailable**
- Given the stub server is closed before `Search` is invoked
  (resulting in TCP connection refused),
- When `Search` is invoked,
- Then `errors.Is(err, types.ErrSourceUnavailable)` is true and
  `*SourceError.HTTPStatus == 0`.

**Scenario S-005-D: Underlying error preserved**
- Given a 5xx or network error path,
- When the returned `*SourceError` is unwrapped via `errors.Unwrap`,
- Then the inner error is non-nil and its `Error()` text contains
  evidence of the original failure (e.g., `"connection refused"`,
  `"500 Internal Server Error"`, or the HTTP status text).

**Scenario S-005-E: ctx.Done before request → wrapped error**
- Given a ctx already cancelled before `Search` is called,
- When `Search` is invoked,
- Then the return is a non-nil error. `errors.Is(err,
  context.Canceled)` is true OR the error is a `*SourceError`
  wrapping `context.Canceled`. (Either is acceptable; the goal is
  that ctx cancellation is honoured.)

### REQ-ADP-006 — NormalizedDoc Field Mapping

**Scenario S-006-A: Field mapping for full post fixture**
- Given `testdata/search_response.json` with a single post having
  ID `"t3_abc"`, permalink `/r/golang/comments/abc/`, title
  `"Hello"`, selftext `"World"`, score 100, created_utc
  1714000000.0, author "alice", subreddit "golang", num_comments 5,
  upvote_ratio 0.95, over_18 false,
- When `parseListing` is called,
- Then the returned NormalizedDoc has `ID="t3_abc"`,
  `SourceID="reddit"`,
  `URL="https://www.reddit.com/r/golang/comments/abc/"`,
  `Title="Hello"`, `Body="World"`, `Snippet="World"`,
  `PublishedAt=time.Unix(1714000000, 0).UTC()`, `RetrievedAt`
  non-zero, `Author="alice"`, `Score≈normalizeScore(100)≈0.881`
  (within ±0.001), `Lang=""`, `DocType=DocTypePost`,
  `Citations=nil`, `Hash=""`, and Metadata containing `subreddit:
  "golang"`, `over_18: false`, `num_comments: 5`, `upvote_ratio:
  0.95`, `external_url: <data.url>`, `kind: "t3"`.

**Scenario S-006-B: Snippet truncates at 280 chars**
- Given a post with `selftext` of length 500 chars,
- When `parseListing` is called,
- Then the returned doc's `Snippet` has rune-length ≤ 283 (280 +
  `"..."` suffix).

**Scenario S-006-C: Self-post with empty selftext derives Snippet from Title**
- Given a post with `selftext == ""` and `title == "ChatGPT
  weighs in on the debate"`,
- When `parseListing` is called,
- Then `Body == ""` and `Snippet` is derived from `title` (truncated
  if necessary).

**Scenario S-006-D: Non-t3 children filtered**
- Given a Listing with `children[]` containing kinds
  `["t1", "t3", "t5", "t3"]` (a comment, two posts, a subreddit),
- When `parseListing` is called,
- Then exactly 2 NormalizedDocs are returned (the two `t3` posts
  only). Order is preserved from the source array.

**Scenario S-006-E: Pagination cursor on last doc**
- Given `testdata/search_response_pagination.json` with `data.after
  == "t3_xyz"` and 25 children,
- When `parseListing` is called,
- Then `len(docs) == 25` and `docs[24].Metadata["next_cursor"] ==
  "t3_xyz"`. `docs[0..23]` do NOT have a `next_cursor` key.

**Scenario S-006-F: No cursor when data.after is null/empty**
- Given a Listing with `data.after == null` or `data.after == ""`,
- When `parseListing` is called,
- Then no returned doc has a `next_cursor` Metadata key.

**Scenario S-006-G: Hash always empty**
- Given any successful parse,
- When iterating returned docs,
- Then every `doc.Hash == ""`. Consumers compute hash via
  `doc.CanonicalHash()`.

**Scenario S-006-H: Metadata has all 6 required keys**
- Given any successful parse,
- When iterating returned docs,
- Then each doc's Metadata has at minimum
  `{subreddit, over_18, num_comments, upvote_ratio, external_url,
  kind}`.

**Scenario S-006-I: Deleted-author post returned as-is**
- Given `testdata/search_response_deleted_post.json` with
  `data.author == "[deleted]"` and `data.selftext == "[removed]"`,
- When `parseListing` is called,
- Then the returned doc has `Author == "[deleted]"`, `Body ==
  "[removed]"`, and `Validate()` returns nil (because URL, ID,
  SourceID, RetrievedAt are all populated).

**Scenario S-006-J: Malformed JSON → Permanent error**
- Given `testdata/search_response_malformed.json` containing
  truncated JSON,
- When `parseListing` is called,
- Then the returned error is `*types.SourceError{Adapter:"reddit",
  Category: CategoryPermanent, Cause: <json error>}`.

**Scenario S-006-K: Empty children array**
- Given `testdata/search_response_empty.json` with `data.children
  == []`,
- When `parseListing` is called,
- Then it returns `(nil, "", nil)` (zero docs, no cursor, no error).

### REQ-ADP-007 — NSFW Filter

**Scenario S-007-A: nsfw=true → include_over_18=true**
- Given `Query.Filters = [{Key:"nsfw", Value:"true"}]`,
- When `Search` is invoked,
- Then the request URL contains `include_over_18=true`.

**Scenario S-007-B: nsfw=false → include_over_18=false**
- Given `Query.Filters = [{Key:"nsfw", Value:"false"}]`,
- When `Search` is invoked,
- Then the request URL contains `include_over_18=false`.

**Scenario S-007-C: nsfw filter absent → default exclude**
- Given `Query.Filters = nil` or `Query.Filters = []`,
- When `Search` is invoked,
- Then the request URL contains `include_over_18=false`.

**Scenario S-007-D: nsfw with unknown value → default exclude**
- Given `Query.Filters = [{Key:"nsfw", Value:"maybe"}]`,
- When `Search` is invoked,
- Then the request URL contains `include_over_18=false` (any value
  other than literal `"true"` is treated as false).

**Scenario S-007-E: Mixed NSFW + safe results returned as Reddit returns them**
- Given `testdata/search_response_with_nsfw.json` with 25 posts of
  which 3 have `over_18: true`,
- When `Search` is invoked with `Query.Filters =
  [{nsfw, true}]`,
- Then all 25 posts are returned (the adapter does not post-filter;
  Reddit's server-side filter behaviour is trusted per Open Question
  §11.4). Each over-18 post has `Metadata["over_18"] == true`.

### REQ-ADP-008 — Empty/Whitespace Query Rejection

**Scenario S-008-A: Empty Text rejected**
- Given `Query.Text == ""` and an instrumented stub that counts
  requests,
- When `Search` is invoked,
- Then the return is `(nil, *SourceError{Permanent, Cause:
  ErrInvalidQuery})`. The stub observes ZERO requests.

**Scenario S-008-B: Whitespace-only Text rejected**
- Given `Query.Text` is one of `"   "` (ASCII spaces) or
  `"\t\n  \r"` (mixed tab/newline/carriage-return + ASCII spaces) —
  both inputs match the breaking-whitespace set Go's
  `unicode.IsSpace` identifies as space,
- When `Search` is invoked,
- Then the return is `(nil, *SourceError{Permanent, Cause:
  ErrInvalidQuery})`. The stub observes ZERO requests.
- Note: NBSP U+00A0 is intentionally NOT in this fixture because
  `unicode.IsSpace` returns false for NBSP; the implementation
  rule and the test fixture are aligned on the breaking-whitespace
  set only. Whether the policy should expand to cover NBSP /
  `unicode.White_Space` is documented as a future-iteration
  consideration in the §3 mapping discussion (not an open question
  blocking v0.1).

**Scenario S-008-C: Single-character non-whitespace Text accepted**
- Given `Query.Text == "x"`,
- When `Search` is invoked against a stub returning empty results,
- Then the request IS made (Reddit may return zero results, but
  the adapter doesn't reject on length).

### REQ-ADP-009 — User-Agent and Accept Headers

**Scenario S-009-A: Default User-Agent set**
- Given `Options.UserAgentVersion` not specified (default `"v0.1"`),
- When `Search` is invoked,
- Then the captured request `User-Agent` header equals
  `"usearch/v0.1 (+https://github.com/elymas/universal-search)"`.

**Scenario S-009-B: Custom UA version propagates**
- Given `Options.UserAgentVersion = "v0.2-rc1"`,
- When `Search` is invoked,
- Then the captured request `User-Agent` header equals
  `"usearch/v0.2-rc1 (+https://github.com/elymas/universal-search)"`.

**Scenario S-009-C: Accept header set**
- Given any `Search` invocation,
- When the request reaches the stub,
- Then the captured `Accept` header equals `"application/json"`.

**Scenario S-009-D: No default Go UA leaked**
- Given any `Search` invocation,
- When the captured `User-Agent` header is inspected,
- Then it does NOT contain the substring `"Go-http-client"` (the
  default Go UA, which Reddit blocks).

### REQ-ADP-010 — Redirect Allowlist

**Scenario S-010-A: Allowlist redirect followed**
- Given server A returns HTTP 302 with `Location:
  https://www.reddit.com/<path>` and a custom `http.RoundTripper`
  routes `www.reddit.com` requests to server B (which returns the
  25-doc Listing),
- When `Search` is invoked,
- Then the return is the 25-doc result (redirect followed
  successfully).

**Scenario S-010-B: Cross-domain redirect rejected**
- Given server A returns HTTP 302 with `Location:
  https://attacker.com/x`,
- When `Search` is invoked,
- Then the return is `(nil, *SourceError{Permanent, Cause: <error
  containing "cross-domain redirect rejected: attacker.com">})`.
  No request is made to `attacker.com`.

**Scenario S-010-C: Redirect chain over 3 hops rejected**
- Given 4 servers each returning 302 to the next within the
  allowlist,
- When `Search` is invoked,
- Then the return is an error whose message contains `"too many
  redirects"`. The 4th server is NOT reached.

**Scenario S-010-D: 1- and 2-hop redirects within allowlist succeed**
- Given a 2-hop chain (`www.reddit.com` → `old.reddit.com` →
  200 response),
- When `Search` is invoked,
- Then the return is the docs from the final response (not an
  error).

### REQ-ADP-011 — Concurrent Search Safety (State-Driven)

**Scenario S-011-A: 50 concurrent goroutines share one Adapter**
- Given a single `*Adapter` constructed with `Options{BaseURL:
  <stub server URL>}` and a stub `httptest.Server` that records
  every inbound request and returns
  `testdata/search_response.json` (25-doc Listing) on each call,
- And given a `sync.WaitGroup` barrier so 50 goroutines start
  their `Search` invocations as close to simultaneously as the
  scheduler allows,
- When all 50 goroutines complete and the test gathers their
  return values,
- Then:
  1. `go test -race` reports zero data-race alarms attributable
     to the adapter package.
  2. The stub's request counter equals exactly 50 (one HTTP
     round-trip per goroutine; no hidden caching, deduplication,
     or retry).
  3. Every goroutine receives `(docs, nil)` with `len(docs) ==
     25`. Each `[]types.NormalizedDoc` slice has every doc
     passing `Validate()` returning nil.
  4. No goroutine returns an error attributable to concurrent
     state corruption (e.g., torn reads, panics).

---

## 2. NFR Acceptance

### NFR-ADP-001 — Parse-Path Performance

**Scenario N-001: Benchmark within target**
- Given `BenchmarkParseListing25Docs` reading
  `testdata/search_response.json` (25-doc Listing, ~5KB),
- When the benchmark is invoked on amd64 as
  `go test -bench=BenchmarkParseListing25Docs -benchtime=10x
  -count=5 ./internal/adapters/reddit/...`,
- Then the median of the 5 reported per-op mean wall-clock
  durations is ≤ 5ms and `allocs/op` is ≤ 250. Pass/fail is
  decidable from the `go test -bench` output alone (no external
  CI script required).

### NFR-ADP-002 — Stub E2E p95 Latency

**Scenario N-002: p95 ≤ 200ms over 100 invocations**
- Given a stub `httptest.Server` returning the 25-doc fixture with
  zero artificial delay,
- When `Search` is invoked 100 times sequentially,
- Then the 95th percentile elapsed time (sorted durations index 94)
  is ≤ 200ms.

### NFR-ADP-003 — No Goroutine Leak on Cancellation

**Scenario N-003: goleak verifies clean shutdown**
- Given a stub server that delays response by 200ms and a ctx
  cancelled at 50ms via `context.WithTimeout`,
- When `Search` is invoked and returns (with an error wrapping
  `context.DeadlineExceeded`),
- Then `goleak.VerifyNone(t)` reports zero residual goroutines.

---

## 3. Edge-Case Scenarios

**Scenario E-001: HTTP 200 with empty children**
- Given the stub returns HTTP 200 with body
  `{"kind":"Listing","data":{"children":[],"after":null}}`,
- When `Search` is invoked,
- Then the return is `(nil, nil)` (no docs, no error). The
  registry's wrappedAdapter records this as outcome="success".

**Scenario E-002: HTTP 200 with malformed JSON**
- Given the stub returns HTTP 200 with body `{"kind":"Listing"`
  (truncated),
- When `Search` is invoked,
- Then the return is `(nil, *SourceError{Permanent, Cause: <json
  error>})`.

**Scenario E-003: 25-doc fixture with 3 over_18 + nsfw filter false**
- Given `testdata/search_response_with_nsfw.json` (25 posts, 3 over
  18) and `Query.Filters = [{nsfw, false}]`,
- When `Search` is invoked (the stub does not actually filter
  server-side, since it's a stub),
- Then 25 docs are returned (the adapter trusts Reddit's filter).
  The test fixture documents the limitation; future SPEC-AUTH-002
  may add post-filtering.

**Scenario E-004: Pagination round-trip**
- Given `Search(ctx, Query{Text: "go", Cursor: ""})` returns 25 docs
  whose last has `Metadata["next_cursor"] == "t3_xyz"`,
- When the caller invokes `Search(ctx, Query{Text: "go", Cursor:
  "t3_xyz"})`,
- Then the second request URL contains `&after=t3_xyz` (cursor
  round-trip closed).

**Scenario E-005: Cross-domain redirect SSRF prevention**
- Given the stub returns 302 to `https://internal-service:9999/x`
  (a hypothetical SSRF target),
- When `Search` is invoked,
- Then the cross-domain redirect is rejected; no request to
  `internal-service` is observed; the returned error indicates
  `"cross-domain redirect rejected"`.

**Scenario E-006: ctx cancellation mid-flight**
- Given a stub server that delays response by 300ms and a ctx
  cancelled at 100ms,
- When `Search` is invoked,
- Then the return is a non-nil error wrapping
  `context.DeadlineExceeded` (or `context.Canceled`, depending on
  cancel cause). The total elapsed time is ≤ 150ms (cancellation
  honoured promptly, not waiting for the 300ms server delay).

**Scenario E-007: Very long query text**
- Given `Query.Text` is 4096 characters of valid UTF-8,
- When `Search` is invoked against a stub returning empty,
- Then the request URL contains the full URL-escaped text in the
  `q` parameter; no truncation is applied by the adapter (Reddit
  may truncate server-side; the adapter does not enforce additional
  length limits).

**Scenario E-008: Score normalization at extremes**
- Given a parse with posts of `score = -10000, -1000, 0, 1000,
  10000`,
- When the resulting NormalizedDocs are inspected,
- Then their Scores are within `±0.001` of `0.0, 0.0, 0.5, 1.0,
  1.0` respectively (saturation at extremes).

---

## 4. Quality Gate Criteria

Before SPEC is marked `implemented`, ALL of the following MUST be
true:

- [ ] **Test pass**: `go test ./internal/adapters/reddit/...` exits 0.
- [ ] **Race-clean**: `go test -race ./internal/adapters/reddit/...`
      exits 0.
- [ ] **Coverage**: `go test -cover ./internal/adapters/reddit/...`
      reports ≥ 85% (per `quality.test_coverage_target`).
- [ ] **vet clean**: `go vet ./internal/adapters/reddit/...` has no
      output.
- [ ] **lint clean**: `golangci-lint run
      ./internal/adapters/reddit/...` has no findings.
- [ ] **Bench**: `BenchmarkParseListing25Docs` invoked as
      `go test -bench=BenchmarkParseListing25Docs -benchtime=10x
      -count=5 ./internal/adapters/reddit/...` on amd64; median of
      the 5 reported per-op mean durations is ≤ 5ms; allocs/op ≤ 250.
- [ ] **Stub p95**: `TestSearchE2ELatencyStubP95` passes.
- [ ] **Goroutine clean**: `TestSearchNoGoroutineLeakOnCancel`
      passes.
- [ ] **MX tags**: All tags from spec.md §6.7 applied with `[AUTO]`
      prefix and `@MX:SPEC: SPEC-ADP-001`.
- [ ] **Capabilities.Notes**: Contains the 4 documented substrings
      (`"public no-auth"`, `"NSFW excluded by default"`, `"t=all"`,
      `"rate limit discrepancy"`).
- [ ] **Compile-time interface assertion**: `var _ types.Adapter =
      (*Adapter)(nil)` present in `reddit.go`.
- [ ] **No drive-by changes**: `git diff` outside
      `internal/adapters/reddit/` is empty (or limited to
      `go.mod`/`go.sum` for the optional `goleak` add).

---

## 5. Definition of Done

The SPEC moves from `draft` → `in-progress` → `implemented` when:

1. All 11 EARS REQs in spec.md §3 have at least one passing test
   referencing the REQ ID in test name or comment (REQ-ADP-001
   through REQ-ADP-011, including the iteration-2-added
   REQ-ADP-011 concurrency-safety contract).
2. All 3 NFRs in spec.md §4 have at least one passing measurement.
3. All quality gate criteria in §4 above are checked off.
4. `internal/adapters/reddit/` package compiles, tests, lints, and
   benchmarks without error.
5. The HISTORY block in spec.md is appended with the implementation
   commit hash and a one-line summary (per the SPEC-CORE-001 +
   SPEC-IR-001 convention).
6. `manager-docs` has run `/moai sync SPEC-ADP-001` and updated
   CHANGELOG.md, optionally `docs/adapters/reddit.md`, and any
   downstream SPEC dependency-status notes.

---

*End of SPEC-ADP-001 acceptance.md v0.1*
