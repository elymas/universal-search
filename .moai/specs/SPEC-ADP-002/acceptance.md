# SPEC-ADP-002 Acceptance Criteria (Post-Hoc)

**SPEC**: SPEC-ADP-002 — Hacker News Adapter
**Status**: implemented (2026-04-28)
**Format**: Given/When/Then per REQ + edge cases + Definition of Done

---

## 1. Acceptance Scenarios by REQ

### REQ-ADP2-001 — Adapter Interface Conformance

**AC-001: Compile-time interface assertion**
- Given the package `internal/adapters/hn` declares
  `var _ types.Adapter = (*Adapter)(nil)` at the bottom of `hn.go`,
- When `go build ./internal/adapters/hn/...` runs,
- Then the build succeeds without error.

**AC-002: Name returns "hackernews"**
- Given a constructed `*Adapter` from `New(Options{})`,
- When `(*Adapter).Name()` is called,
- Then the return value equals the literal string `"hackernews"`.

**AC-003: Capabilities are deterministic and shape-correct**
- Given a constructed `*Adapter`,
- When `Capabilities()` is called twice in succession,
- Then `reflect.DeepEqual(call1, call2) == true` AND the returned
  struct has `SourceID="hackernews"`, `DisplayName="Hacker News"`,
  `DocTypes=[DocTypePost]`, `RequiresAuth=false`, `AuthEnvVars=nil`,
  `SupportsSince=true`, `RateLimitPerMin=60`, `DefaultMaxResults=25`,
  and `Notes` contains all four documented substrings.

**AC-004: Healthcheck succeeds against reachable target**
- Given a TCP listener bound to `127.0.0.1:0` configured as the
  Healthcheck dial target via `Options.HealthcheckTarget`,
- When `Healthcheck(ctx)` is called with a non-cancelled ctx,
- Then it returns `nil`.

### REQ-ADP2-002 — Search Happy Path and URL Parameters

**AC-005: Happy path 25 results**
- Given a stub `httptest.Server` returning
  `testdata/search_response.json` (25-hit Algolia response) with HTTP
  200,
- When `Search(ctx, types.Query{Text: "rust", MaxResults: 25})` is
  called,
- Then the return is `(docs []NormalizedDoc, nil)` with `len(docs)
  == 25`. Every doc passes `Validate()` returning nil.

**AC-006: URL contains all required parameters**
- Given the stub captures the inbound request URL,
- When `Search(ctx, types.Query{Text: "rust", MaxResults: 50})` is
  called,
- Then the captured URL contains `query=rust`, `tags=story`,
  `hitsPerPage=50`.

**AC-007: hitsPerPage clamped to 100; defaults to 25**
- Given `Query.MaxResults = 500`,
- When `Search` is invoked,
- Then the request URL contains `hitsPerPage=100`.
- Given `Query.MaxResults = 0`, the request URL contains
  `hitsPerPage=25`.

**AC-008: page parameter only when cursor present**
- Given `Query.Cursor = "3"`, the request URL contains `page=3`.
- Given `Query.Cursor = ""`, the request URL does NOT contain a
  `page` parameter.

### REQ-ADP2-003 — HTTP 429 Rate-Limit Mapping

**AC-009: 429 with integer Retry-After**
- Given the stub returns HTTP 429 with `Retry-After: 30`,
- When `Search` is invoked,
- Then the return is `(nil, *types.SourceError{Adapter:"hackernews",
  Category: CategoryRateLimited, HTTPStatus: 429, RetryAfter: 30s,
  Cause: <non-nil>})`. `errors.Is(err, types.ErrRateLimited)`
  returns true.

**AC-010: 429 with HTTP-date Retry-After**
- Given the stub returns HTTP 429 with `Retry-After: <RFC1123 date
  30s in the future>`,
- When `Search` is invoked,
- Then the returned `*SourceError.RetryAfter` is in the range
  `(25s, 35s)`.

**AC-011: 429 without Retry-After defaults to 5s**
- Given the stub returns HTTP 429 with no `Retry-After` header,
- When `Search` is invoked,
- Then `*SourceError.RetryAfter == 5 * time.Second`.

**AC-012: Retry-After capped at 60s**
- Given the stub returns HTTP 429 with `Retry-After: 999`,
- When `Search` is invoked,
- Then `*SourceError.RetryAfter == 60 * time.Second`.

**AC-013: No internal retry on 429**
- Given the stub counts inbound requests and returns HTTP 429,
- When `Search` is invoked once,
- Then exactly 1 request is observed.

### REQ-ADP2-004 — HTTP 4xx/5xx and Network Failure Mapping

**AC-014: 4xx → Permanent**
- Given the stub returns HTTP 401, 403, or 404,
- When `Search` is invoked,
- Then `errors.Is(err, types.ErrPermanent)` is true and
  `*SourceError.HTTPStatus` matches.

**AC-015: 5xx → Unavailable**
- Given the stub returns HTTP 500 or 503,
- When `Search` is invoked,
- Then `errors.Is(err, types.ErrSourceUnavailable)` is true.

**AC-016: Connection refused → Unavailable with HTTPStatus=0**
- Given the stub server is closed before `Search` is invoked,
- When `Search` is invoked,
- Then `errors.Is(err, types.ErrSourceUnavailable)` is true AND
  `*SourceError.HTTPStatus == 0`.

**AC-017: Underlying error preserved**
- Given a 5xx or network error path,
- When the returned `*SourceError` is unwrapped via `errors.Unwrap`,
- Then the inner error is non-nil and its `Error()` text contains
  evidence of the original failure.

### REQ-ADP2-005 — NormalizedDoc Field Mapping

**AC-018: Field mapping for typical story fixture**
- Given a hit with `objectID="39458123"`, `title="Hello"`, `url=
  "https://example.com/"`, `author="alice"`, `points=100`,
  `created_at_i=1714000000`, `num_comments=5`,
  `_tags=["story","front_page"]`,
- When `parseHits` is called,
- Then the returned NormalizedDoc has `ID="39458123"`,
  `SourceID="hackernews"`, `URL="https://example.com/"`,
  `Title="Hello"`, `PublishedAt=time.Unix(1714000000, 0).UTC()`,
  `Author="alice"`, `Score≈normalizeScore(100)≈0.881` (±0.001),
  `Lang=""`, `DocType=DocTypePost`, Metadata containing
  `num_comments: 5`, `points: 100`, `tags: ["story","front_page"]`,
  `external_url: "https://example.com/"`.

**AC-019: Self-post uses news.ycombinator.com permalink**
- Given a hit with `url=""` and `objectID="12345"`,
- When `parseHits` is called,
- Then the returned doc has
  `URL == "https://news.ycombinator.com/item?id=12345"`.

**AC-020: HTML body stripped via stripHTML**
- Given a hit with `story_text="<p>Hello <b>world</b></p>&amp;
  goodbye"`,
- When `parseHits` is called,
- Then the returned doc has `Body == "Hello world& goodbye"` (tags
  stripped, entities decoded).

**AC-021: Defensive _tags filter removes non-story hits**
- Given a Listing with `_tags` arrays of `["comment"]`,
  `["story","front_page"]`, `["poll"]`, `["story"]`,
- When `parseHits` is called,
- Then exactly 2 NormalizedDocs are returned (only the two
  `"story"`-tagged hits). Order preserved from the source array.

**AC-022: Pagination cursor on last doc**
- Given a response with `page=0, nbPages=5, len(hits)=25`,
- When `parseHits` is called,
- Then `len(docs) == 25` and `docs[24].Metadata["next_cursor"] ==
  "1"`. Earlier docs do NOT have the `next_cursor` key.

**AC-023: No cursor on last page**
- Given a response with `page=4, nbPages=5`,
- When `parseHits` is called,
- Then no returned doc has the `next_cursor` key.

**AC-024: Hash always empty**
- Given any successful parse,
- When iterating returned docs,
- Then every `doc.Hash == ""`.

**AC-025: Metadata has all 4 required keys**
- Given any successful parse,
- When iterating returned docs,
- Then each doc's Metadata has at minimum `{num_comments, points,
  tags, external_url}`.

### REQ-ADP2-006 — User-Agent and Accept Headers

**AC-026: Default User-Agent and Accept set**
- Given `Options.UserAgentVersion` not specified,
- When `Search` is invoked,
- Then the captured `User-Agent` starts with `"usearch/"` and
  contains `"(+https://github.com/elymas/universal-search)"` AND
  the captured `Accept` equals `"application/json"`.

**AC-027: Custom UA version propagates**
- Given `Options.UserAgentVersion = "v0.2-rc1"`,
- When `Search` is invoked,
- Then the captured `User-Agent` contains `"usearch/v0.2-rc1"`.

### REQ-ADP2-007 — Numeric Filters (Optional)

**AC-028: since filter added**
- Given `Query.Filters = [{since, "1700000000"}]`,
- When `Search` is invoked,
- Then the request URL contains
  `numericFilters=created_at_i>=1700000000`.

**AC-029: min_points filter added**
- Given `Query.Filters = [{min_points, "10"}]`,
- When `Search` is invoked,
- Then the request URL contains `numericFilters=points>=10`.

**AC-030: Both filters joined**
- Given both filters,
- Then the request URL contains
  `numericFilters=created_at_i>=1700000000,points>=10`.

**AC-031: Unknown / malformed / negative filters dropped silently**
- Given `Filters=[{nsfw,"true"}]` OR `[{since,"abc"}]` OR
  `[{min_points,"-5"}]`,
- When `Search` is invoked,
- Then the request URL has no `numericFilters` parameter AND no
  error is returned.

### REQ-ADP2-008 — Empty/Whitespace Query and Invalid Cursor Rejection

**AC-032: Empty / whitespace Text rejected with zero HTTP requests**
- Given `Query.Text` is one of `["", "   ", "\t\n  \r"]`,
- When `Search` is invoked against an instrumented stub,
- Then the return is `(nil, *SourceError{Permanent, Cause:
  ErrInvalidQuery})` AND the stub observes ZERO requests.

**AC-033: Invalid cursor rejected with zero HTTP requests**
- Given `Query.Cursor` is one of `["abc", "-1", "1.5", "1e3"]`,
- When `Search` is invoked,
- Then the return is `(nil, *SourceError{Permanent, Cause:
  ErrInvalidCursor})` AND the stub observes ZERO requests.

### REQ-ADP2-009 — Redirect Allowlist (Optional)

**AC-034: Allowlist redirect followed**
- Given server A returns HTTP 302 with Location pointing to server
  B (Host rewritten to `hn.algolia.com`),
- When `Search` is invoked,
- Then the returned docs come from server B.

**AC-035: Cross-domain redirect rejected**
- Given server A returns HTTP 302 with Location
  `https://attacker.com/x`,
- When `Search` is invoked,
- Then the return is `(nil, *SourceError{Permanent, Cause: <error
  containing "cross-domain redirect">})`.

**AC-036: Chain over 3 hops rejected**
- Given 4 servers each returning 302 within the allowlist,
- When `Search` is invoked,
- Then the return is an error whose message contains `"too many
  redirects"`.

### REQ-ADP2-010 — Concurrent Search Safety (State-Driven)

**AC-037: 50 concurrent goroutines, race-clean**
- Given a single `*Adapter` constructed with one stub server,
- And 50 goroutines synchronised via `sync.WaitGroup` barrier,
- When all 50 issue `Search` simultaneously,
- Then `go test -race` reports zero data-race alarms AND the stub's
  request counter equals 50 AND every goroutine receives `(docs,
  nil)` with `len(docs) == 25`.

---

## 2. NFR Acceptance

### NFR-ADP2-001 — Parse-Path Performance

**AC-N01: Benchmark within target**
- Given `BenchmarkParseHits25Hits` reading the 25-hit fixture,
- When invoked as `go test -bench=BenchmarkParseHits25Hits
  -benchtime=10x -count=5 ./internal/adapters/hn/...`,
- Then the median of the 5 per-op mean durations is ≤ 5 ms AND
  `allocs/op` is ≤ 500.

### NFR-ADP2-002 — Stub E2E p95 Latency

**AC-N02: p95 ≤ 200ms over 100 invocations**
- Given a stub `httptest.Server` returning the 25-hit fixture,
- When `Search` is invoked 100 times sequentially,
- Then `durations[94] ≤ 200ms`.

### NFR-ADP2-003 — No Goroutine Leak on Cancellation

**AC-N03: goleak verifies clean shutdown**
- Given a stub delaying 200ms and a ctx cancelled at 50ms,
- When `Search` is invoked and returns,
- Then `goleak.VerifyNone(t)` reports zero residual goroutines.

---

## 3. Edge Cases

**EC-001: HTTP 200 with empty hits**
- Stub returns `{"hits":[],"nbHits":0,"page":0,"nbPages":0}` → `(nil, nil)`.

**EC-002: HTTP 200 with malformed JSON**
- Truncated JSON body → `*SourceError{Permanent, Cause: <json error>}`.

**EC-003: Deleted-author hit**
- Hit with `author=""` returned as-is; `Validate()` still passes
  (URL, ID, SourceID, RetrievedAt all populated).

**EC-004: Pagination round-trip**
- First call returns `next_cursor="1"`; second call with
  `Cursor="1"` sends `page=1` in URL.

**EC-005: ctx cancellation mid-flight**
- Stub delays 300ms; ctx cancelled at 100ms → non-nil error
  wrapping `context.Canceled` OR `context.DeadlineExceeded`; total
  elapsed ≤ 150ms.

**EC-006: Score saturation at extremes**
- `points = -10000, -1000, 0, 1000, 10000` → Scores within ±0.001
  of `0.0, 0.0, 0.5, 1.0, 1.0`.

**EC-007: NBSP not whitespace**
- `Query.Text = " "` (NBSP-only) is NOT rejected by
  REQ-ADP2-008 because `unicode.IsSpace` returns false for NBSP.
  The whitespace-rejection contract follows Go stdlib's
  breaking-whitespace set.

---

## 4. Coverage Matrix

| REQ-ID | EARS Pattern | Acceptance Criteria | Implementation |
|--------|-------------|---------------------|----------------|
| REQ-ADP2-001 | Ubiquitous | AC-001..004 | `hn.go`, `hn_test.go` |
| REQ-ADP2-002 | Event-Driven | AC-005..008 | `search.go`, `search_test.go` |
| REQ-ADP2-003 | Event-Driven | AC-009..013 | `client.go::categorizeStatus`, `errors.go::parseRetryAfter` |
| REQ-ADP2-004 | Event-Driven | AC-014..017 | `client.go::categorizeStatus` |
| REQ-ADP2-005 | Ubiquitous | AC-018..025 | `parse.go::parseHits`, `strip.go::stripHTML`, `score.go::normalizeScore` |
| REQ-ADP2-006 | Ubiquitous | AC-026..027 | `client.go::doRequest` |
| REQ-ADP2-007 | Optional | AC-028..031 | `search.go::buildSearchURL` |
| REQ-ADP2-008 | Unwanted | AC-032..033 | `search.go` (input validation) |
| REQ-ADP2-009 | Optional | AC-034..036 | `client.go::redirectAllowlist` |
| REQ-ADP2-010 | State-Driven | AC-037 | `search_test.go::TestSearchConcurrentSafe` |
| NFR-ADP2-001 | Performance | AC-N01 | `bench_test.go::BenchmarkParseHits25Hits` |
| NFR-ADP2-002 | Latency | AC-N02 | `search_test.go::TestSearchE2ELatencyStubP95` |
| NFR-ADP2-003 | Resource | AC-N03 | `search_test.go::TestSearchNoGoroutineLeakOnCancel` |

---

## 5. Definition of Done

- [x] All 10 EARS REQs in spec.md §3 have at least one passing test
      referencing the REQ ID.
- [x] All 3 NFRs have at least one passing measurement.
- [x] `go test ./internal/adapters/hn/...` exits 0.
- [x] `go test -race ./internal/adapters/hn/...` exits 0.
- [x] `go test -cover ./internal/adapters/hn/...` reports ≥ 85%.
- [x] `go vet ./internal/adapters/hn/...` clean.
- [x] `golangci-lint run ./internal/adapters/hn/...` clean.
- [x] `BenchmarkParseHits25Hits` median ≤ 5ms; allocs/op ≤ 500.
- [x] `TestSearchE2ELatencyStubP95` passes.
- [x] `TestSearchNoGoroutineLeakOnCancel` passes.
- [x] MX tags applied per spec.md §6.7 plan.
- [x] Capabilities.Notes contains all 4 documented substrings.
- [x] Compile-time `var _ types.Adapter = (*Adapter)(nil)` present
      in `hn.go`.
- [x] No drive-by changes outside `internal/adapters/hn/`.
- [x] SPEC status updated to `implemented` (2026-04-28).

---

*End of SPEC-ADP-002 acceptance.md (post-hoc, v1.0)*
