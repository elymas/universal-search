# SPEC-ADP-001 Implementation Plan

**SPEC**: SPEC-ADP-001 — Reddit Adapter (Reference Implementation)
**Methodology**: TDD (RED → GREEN → REFACTOR per REQ)
**Coverage Target**: 85% (per `.moai/config/sections/quality.yaml`)
**Harness**: standard
**Owner**: expert-backend
**Priority**: P0
**Created**: 2026-04-26

---

## 1. Overview

This plan decomposes SPEC-ADP-001 into TDD-sequenced tasks, one per
EARS REQ plus integration and benchmark tasks. The Reddit adapter is
the FIRST real adapter consuming the SPEC-CORE-001 contract; the
shape laid down here propagates to ADP-002..009. Plan-phase
priorities: get the file layout right, get the error-mapping right,
get the field-mapping right, get the redirect-allowlist right.
Everything else flows from those four invariants.

---

## 2. Task Decomposition (TDD-Sequenced)

Tasks are ordered for RED→GREEN→REFACTOR cycles. Each task produces
a failing test first (RED), the minimum implementation to pass
(GREEN), then a tidy-up pass (REFACTOR). Files modified cite
expected line counts.

### Phase A — Skeleton and Interface (Priority High)

**Task A1: Bootstrap package and interface assertion**
- Create `internal/adapters/reddit/` directory.
- Create `reddit.go` with empty `Adapter` struct, `New`, `Name`,
  `Capabilities`, `Healthcheck` stubs returning zero values.
- Add compile-time assertion `var _ types.Adapter = (*Adapter)(nil)`
  at file end.
- RED: `TestAdapterImplementsInterface` (compile-time) + `TestAdapterName`.
- GREEN: implement minimal `Name() string { return "reddit" }`.
- REFACTOR: nothing yet.
- Files: `reddit.go` (~20 LoC), `reddit_test.go` (~30 LoC).
- REQ: REQ-ADP-001.

**Task A2: Capabilities descriptor**
- Implement `Capabilities()` returning the documented 10-field
  descriptor (DocTypes=[DocTypePost], RequiresAuth=false,
  RateLimitPerMin=10, etc.).
- RED: `TestCapabilitiesDeterministic`, `TestCapabilitiesShape`.
- GREEN: hard-code the value-returning function.
- REFACTOR: extract `defaultCapabilities` package-level variable if
  it improves readability.
- Files: `reddit.go` (+~30 LoC).
- REQ: REQ-ADP-001.

**Task A3: Healthcheck via TCP-connect (with HealthcheckTarget seam)**
- Add `HealthcheckTarget string` field to the `Options` struct
  (default constant `defaultHealthcheckTarget = "www.reddit.com:443"`).
- Add a matching unexported `healthcheckTarget string` field on
  the `Adapter` struct, populated by `New()` from the option (or
  the default when the option is empty).
- Implement TCP-connect via `net.Dialer.DialContext(ctx, "tcp",
  a.healthcheckTarget)` honouring ctx; close the connection on
  success.
- RED: `TestHealthcheckSucceeds` constructs the Adapter with
  `Options{HealthcheckTarget: <httptest server addr>}` so the dial
  hits a loopback listener, avoiding live network. This is the
  committed test seam (no package-level globals, no build tags).
- GREEN: minimal `net.Dialer.DialContext` + `conn.Close()`.
- REFACTOR: ensure `healthcheckTarget` is set exactly once at
  construction time and never mutated afterwards (immutable per
  REQ-ADP-011 concurrency safety).
- Files: `reddit.go` (+~20 LoC).
- REQ: REQ-ADP-001.

### Phase B — HTTP Client and Error Mapping (Priority High)

**Task B1: HTTP client with timeout, redirect allowlist, reqid transport**
- Create `client.go` with `newDefaultClient() *http.Client`
  (10s timeout, `redirectAllowlist` CheckRedirect, transport wrapped
  with `internal/obs/reqid.NewTransport(http.DefaultTransport)`).
- Define `allowedRedirectHosts` map and `redirectAllowlist`
  function (max 3 hops, host membership check).
- RED: `TestSearchFollowsAllowlistRedirect`,
  `TestSearchRejectsCrossDomainRedirect`,
  `TestSearchRejectsRedirectChainOver3` (these will fail because
  Search isn't implemented yet — acceptable; partial-RED is allowed
  during scaffolding).
- GREEN: implement `redirectAllowlist`.
- REFACTOR: ensure the allowlist map is package-level and immutable;
  add `@MX:NOTE` documenting the security boundary.
- Files: `client.go` (~80 LoC initially), `client_test.go` (~120 LoC).
- REQ: REQ-ADP-010 (redirect), supports REQ-ADP-009 (UA via doRequest).

**Task B2: HTTP status → SourceError categorisation**
- Implement `categorizeStatus(status int, retryAfter time.Duration,
  cause error) *types.SourceError` per the §6.4 sketch (429 →
  RateLimited, 4xx → Permanent, 5xx → Unavailable, 0 → Unavailable
  for network errors).
- RED: `TestCategorizeStatusTable` (truth table over 7 status codes).
- GREEN: switch statement.
- REFACTOR: ensure all paths populate `Adapter:"reddit"`,
  `HTTPStatus`, `Cause`.
- Files: `client.go` (+~40 LoC).
- REQ: REQ-ADP-003, REQ-ADP-004, REQ-ADP-005.

**Task B3: parseRetryAfter helper**
- Implement `parseRetryAfter(header string, now time.Time)
  time.Duration` per RFC 7231 §7.1.3 (try `strconv.Atoi` first; fall
  back to `http.ParseTime`; cap at 60s; default 5s on parse failure
  or empty header; reject negative values, defaulting to 5s).
- RED: `TestParseRetryAfterTable` (table over 6 inputs: integer,
  HTTP-date in future, missing, malformed, > 60s value, negative
  value).
- GREEN: minimal stdlib parsing.
- REFACTOR: extract constants `maxRetryAfter = 60*time.Second`,
  `defaultRetryAfter = 5*time.Second`.
- Files: `errors.go` (~50 LoC).
- REQ: REQ-ADP-003.

**Task B4: doRequest helper (UA header + Accept header)**
- Implement `doRequest(ctx, *http.Request) (*http.Response, error)`
  on `*Adapter` that sets `User-Agent: a.userAgent` and
  `Accept: application/json` headers and calls `a.httpClient.Do`.
- RED: `TestSearchSetsCustomUserAgent`, `TestSearchSetsAcceptJSON`,
  `TestSearchUserAgentVersionConfigurable` (these will pass partially
  once Search is implemented in Phase C).
- GREEN: header-set + Do.
- REFACTOR: ensure ctx is attached via `req.WithContext(ctx)` if not
  already.
- Files: `client.go` (+~20 LoC).
- REQ: REQ-ADP-009.

### Phase C — Parse Path (Priority High)

**Task C1: Score normalization**
- Implement `normalizeScore(score int) float64` per §2.3 Tanh
  formula. Pure function; no state.
- Define package-level constants `tanhDivisor = 100.0`,
  `scoreCenter = 0.5` with `@MX:NOTE` annotations.
- RED: `TestNormalizeScoreTable` (7 score values),
  `TestNormalizeScoreDeterministic`.
- GREEN: `0.5 + 0.5*math.Tanh(float64(score)/tanhDivisor)` with
  clamp to `[0, 1]`.
- REFACTOR: ensure clamp is explicit (`math.Max(0, math.Min(1, v))`)
  for floating-point edge case safety.
- Files: `score.go` (~25 LoC), `score_test.go` (~60 LoC).
- REQ: REQ-ADP-006 (score field of NormalizedDoc).

**Task C2: parseListing — happy path**
- Define internal struct types matching Reddit JSON envelope
  (`redditListing`, `redditChild`, `redditPostData`).
- Implement `parseListing(body []byte, retrievedAt time.Time)
  ([]types.NormalizedDoc, string, error)` that unmarshals,
  iterates `data.children`, filters non-`t3` kinds, transforms each
  per the §6.3 mapping table, returns docs + `data.after`.
- Surface `data.after` as `Metadata["next_cursor"]` on the LAST
  returned doc only (per REQ-ADP-006).
- RED: `TestParseListingFieldMapping`,
  `TestParseListingFiltersNonT3Kinds`,
  `TestParseListingPaginationCursor`,
  `TestParseListingNoCursorOnEmpty`, `TestParseListingHashEmpty`,
  `TestParseListingMetadataKeys`.
- GREEN: minimal struct definitions + `json.Unmarshal` + per-doc
  transform function.
- REFACTOR: extract `transformPost(data redditPostData,
  retrievedAt time.Time) types.NormalizedDoc` for testability and
  readability. Add `@MX:ANCHOR` to `parseListing`.
- Files: `parse.go` (~150 LoC), `parse_test.go` (~250 LoC),
  testdata fixtures (~6 files, ~15KB total).
- REQ: REQ-ADP-006.

**Task C3: parseListing — edge cases**
- Handle `[deleted]` author (returned as-is).
- Handle malformed JSON (`*SourceError{Permanent}`).
- Handle empty children array (return `nil, "", nil`).
- Handle missing optional fields (selftext, post_hint, link_flair_text)
  by leaving the corresponding NormalizedDoc/Metadata fields at zero
  values.
- RED: `TestParseListingDeletedAuthor`,
  `TestParseListingMalformedJSON`,
  `TestParseListingEmpty` (uses `search_response_empty.json`).
- GREEN: defensive nil/empty checks in `transformPost`.
- REFACTOR: ensure no field defaulting that contradicts research.md
  §7.5 (`[deleted]` returned literally).
- Files: `parse.go` (+~30 LoC), `parse_test.go` (+~80 LoC).
- REQ: REQ-ADP-006.

### Phase D — Search Hot Path (Priority High)

**Task D1: Search — empty query rejection**
- Implement `(*Adapter).Search(ctx, q types.Query)
  ([]types.NormalizedDoc, error)` with input validation: if
  `q.Text` is empty or all-whitespace (per `unicode.IsSpace`),
  return `*SourceError{Permanent, Cause: ErrInvalidQuery}` immediately.
- Define `ErrInvalidQuery` sentinel in `errors.go`.
- RED: `TestSearchEmptyQueryRejectedNoHTTP` (4-row table; verifies
  ZERO HTTP requests via instrumented stub).
- GREEN: `for _, r := range q.Text { if !unicode.IsSpace(r)
  { hasContent=true; break } }`.
- REFACTOR: extract `isAllWhitespace(s string) bool` helper if
  cleaner.
- Files: `search.go` (~30 LoC), `errors.go` (+~5 LoC),
  `search_test.go` (~80 LoC).
- REQ: REQ-ADP-008.

**Task D2: Search — URL construction**
- Build the request URL using `net/url.Values`: q (escaped), sort=
  relevance, t=all, type=link, limit (clamped 1..100, default 25),
  include_over_18 (per NSFW filter), after (if cursor non-empty).
- RED: `TestSearchURLParametersIncludeAllRequired`,
  `TestSearchClampsLimitTo100`, `TestSearchDefaultsLimitTo25`,
  `TestSearchHonoursCursorParameter`,
  `TestSearchNSFWFilterTrueIncludesOver18` and the 3 other NSFW
  variants.
- GREEN: `url.Values{}` + `Encode()`; helper `nsfwFilterValue(filters
  []types.Filter) string` returning `"true"` only when explicit
  match.
- REFACTOR: extract `buildSearchURL(baseURL string, q types.Query)
  string` for testability.
- Files: `search.go` (+~50 LoC), `search_test.go` (+~150 LoC).
- REQ: REQ-ADP-002, REQ-ADP-007.

**Task D3: Search — HTTP execute and response handling**
- Construct `http.Request` via `http.NewRequestWithContext`, call
  `a.doRequest`, handle the response:
  - 200 → read body, call `parseListing`, return docs + nil error.
  - 429 → parse `Retry-After`, return `categorizeStatus(429,
    retryAfter, cause)`.
  - 4xx (other) → `categorizeStatus(status, 0, cause)`.
  - 5xx → `categorizeStatus(status, 0, cause)`.
  - network error → `categorizeStatus(0, 0, err)`.
- Read response body with a hard cap (e.g., 5 MB via
  `io.LimitReader`) to prevent OOM from runaway responses.
- RED: `TestSearchHappyPath25Docs`,
  `TestSearchHTTP429WithIntegerRetryAfter` and 4 other 429 variants,
  `TestSearchHTTP401/403/404`, `TestSearchHTTP500/503`,
  `TestSearchConnectionRefused`,
  `TestSearchUnavailablePreservesUnderlyingError`.
- GREEN: switch on `resp.StatusCode`.
- REFACTOR: extract `(a *Adapter) executeSearch(ctx, urlStr string)
  ([]NormalizedDoc, error)` to keep `Search` orchestration short.
  Add `@MX:ANCHOR` to `Search`.
- Files: `search.go` (+~80 LoC), `search_test.go` (+~250 LoC).
- REQ: REQ-ADP-002, REQ-ADP-003, REQ-ADP-004, REQ-ADP-005.

### Phase E — NFRs and Quality Gates (Priority Medium)

**Task E1: Goroutine leak verification**
- Add `TestSearchNoGoroutineLeakOnCancel` using `goleak.VerifyNone(t)`
  after a Search call whose ctx is cancelled mid-flight (50ms cancel
  vs 200ms server delay).
- Add `go.uber.org/goleak` to `go.mod` if missing.
- GREEN: ensure body is fully drained or response is closed even on
  ctx cancellation; defer `resp.Body.Close()` is mandatory.
- Files: `search_test.go` (+~30 LoC), `go.mod` (1-line).
- NFR: NFR-ADP-003.

**Task E2: E2E p95 latency assertion**
- Add `TestSearchE2ELatencyStubP95` that runs 100 Search invocations
  against the stub; sort durations; assert `durations[94] ≤ 200ms`.
- Files: `search_test.go` (+~30 LoC).
- NFR: NFR-ADP-002.

**Task E3: Parse-path benchmark (with NFR-ADP-001 invocation contract)**
- Add `BenchmarkParseListing25Docs` reading the 25-doc fixture and
  calling `parseListing` in a tight loop. Report `B/op`,
  `allocs/op`, and timing.
- Invocation contract per NFR-ADP-001: the assertion is run as
  `go test -bench=BenchmarkParseListing25Docs -benchtime=10x
  -count=5 ./internal/adapters/reddit/...` on amd64. Take the 5
  per-op mean wall-clock durations from the `-count` runs; the
  MEDIAN of those 5 values must be ≤ 5 ms. Pass/fail is decidable
  from `go test -bench` output alone — no external CI script.
- Files: `bench_test.go` (~40 LoC).
- NFR: NFR-ADP-001.

**Task E4: Coverage verification**
- Run `go test -cover ./internal/adapters/reddit/...`; assert
  reported coverage ≥ 85%.
- If under target, add tests for uncovered branches (typically: rare
  parse-error paths, edge cases in score clamp).
- Files: none new; possibly +tests in existing `*_test.go`.
- Quality gate: `coverage_target: 85`.

### Phase F — REFACTOR and MX Tag Audit (Priority Medium)

**Task F1: MX tag application**
- Apply tags per the §6.7 plan:
  - `@MX:ANCHOR` on `(*Adapter).Search`, `parseListing`.
  - `@MX:NOTE` on `normalizeScore`, `categorizeStatus`,
    `tanhDivisor`/`scoreCenter` constants, `allowedRedirectHosts`.
  - `@MX:WARN` on `doRequest` (network call + redirect safety
    boundary).
- Each tag includes `[AUTO]` prefix, `@MX:SPEC: SPEC-ADP-001`, and
  `@MX:REASON` for ANCHOR/WARN.
- Files: all `*.go` files.

**Task F2: File-size and readability sweep**
- Verify each `.go` file is under ~200 LoC (excluding tests).
- Extract helpers if any file exceeds the soft cap.
- Run `gofmt`, `go vet`, `golangci-lint run ./internal/adapters/reddit/...`.
- Files: as needed.

**Task F3: Capabilities.Notes verification**
- Confirm the Notes string contains all 4 documented substrings
  (`"public no-auth"`, `"NSFW excluded by default"`, `"t=all"`,
  `"rate limit discrepancy"`) per acceptance criteria.
- Files: `reddit.go`.

---

## 3. Technology Stack

**Pure Go stdlib + existing internal packages**:
- `context` — ctx propagation through Search and Healthcheck.
- `encoding/json` — Listing envelope unmarshal in `parseListing`.
- `errors` — sentinel definitions (`ErrInvalidQuery`), `errors.Is`
  for tests.
- `fmt` — error message construction, UA template formatting.
- `math` — `math.Tanh`, `math.Max`, `math.Min` in `normalizeScore`.
- `net` — `net.Dialer.DialContext` for Healthcheck.
- `net/http` — Client, Request, Response, CheckRedirect.
- `net/url` — `url.Values` for query parameter construction,
  `url.QueryEscape`.
- `strconv` — `strconv.Atoi` in `parseRetryAfter`.
- `strings` — header inspection, snippet construction.
- `time` — Time arithmetic (PublishedAt, RetrievedAt,
  parseRetryAfter).
- `unicode` / `unicode/utf8` — `unicode.IsSpace` for empty-query
  detection, rune counting for Snippet truncation.
- `pkg/types` — Adapter interface, NormalizedDoc, Query, SourceError,
  Capabilities, DocType (already pinned via SPEC-CORE-001).
- `internal/obs/reqid` — `reqid.NewTransport` for request-ID
  propagation in HTTP transport (already pinned via SPEC-OBS-001).
- Test-only: `net/http/httptest` (stdlib), `testing` (stdlib), and
  `go.uber.org/goleak` (NEW — to be added under SPEC-DEP-001 policy
  if absent).

**No third-party Reddit client library** (research.md §5.2): we
implement directly to avoid go-reddit's transitive dependency
surface and to retain control over `*SourceError` categorisation.

---

## 4. HTTP Client Construction (Reference: `internal/llm/client.go`)

The Reddit adapter's HTTP client construction mirrors the pattern
established in `internal/llm/client.go:48-55`:

```go
oc := openai.NewClient(
    option.WithBaseURL(cfg.BaseURL+"/v1"),
    option.WithAPIKey(cfg.MasterKey),
    option.WithHTTPClient(&http.Client{
        Timeout:   time.Duration(cfg.TimeoutSeconds) * time.Second,
        Transport: reqid.NewTransport(http.DefaultTransport),
    }),
)
```

Differences for Reddit (no openai-go SDK; raw HTTP):
- No API key (public endpoint).
- `CheckRedirect` field set to `redirectAllowlist` (LLM client uses
  default redirect handling because it's calling LiteLLM proxy
  which doesn't redirect cross-domain).
- Per-request UA + Accept headers set in `doRequest` (LLM client
  delegates to openai-go).

---

## 5. Reference Citations for Implementer

| File | Lines | Purpose |
|------|-------|---------|
| `internal/llm/client.go` | 31-65 | HTTP client construction pattern with timeout + transport wrapping |
| `internal/llm/client.go` | 51-54 | Specifically the `&http.Client{Timeout, Transport: reqid.NewTransport(...)}` shape |
| `internal/adapters/noop/noop.go` | 14-46 | Minimum viable adapter shape (struct, New, Name, Healthcheck, Search, Capabilities, compile-time assertion) |
| `internal/adapters/registry.go` | 195-219 | wrappedAdapter Search method — the observability emitter that ADP-001's Search feeds into |
| `internal/adapters/registry.go` | 220-252 | wrappedAdapter emit helper — confirms which fields feed which metric labels |
| `pkg/types/errors.go` | 71-120 | SourceError struct + Is/Unwrap — the wrapping target for all ADP-001 errors |
| `pkg/types/errors.go` | 174-193 | OutcomeFromError — confirms which Category maps to which Prometheus outcome label |
| `pkg/types/normalized_doc.go` | 40-77 | NormalizedDoc struct + Validate — required-field discipline |
| `pkg/types/capabilities.go` | 38-62 | Capabilities struct shape + DocType constants |
| `pkg/types/query.go` | 18-44 | Query + Filter shape — the Search input contract |

---

## 6. Risk Analysis (Implementation-Focused)

| Risk | Likelihood | Impact | TDD Mitigation |
|------|-----------|--------|----------------|
| Default Go UA causes immediate 429 | High | High | REQ-ADP-009 tests inspect captured request header; failing UA construction fails the test before any live request would |
| Cross-domain redirect open SSRF | Medium | High | `TestSearchRejectsCrossDomainRedirect` writes the failing test FIRST; allowlist is the minimum implementation to pass |
| Score formula off-by-one or sign error | Medium | Medium | `TestNormalizeScoreTable` covers 7 hand-computed values; any divergence > 0.001 fails |
| `[deleted]` post breaks Validate() | Low | Medium | `TestParseListingDeletedAuthor` confirms Validate passes (URL is permalink, not deleted body) |
| Pagination cursor lost across calls | Low | Medium | `TestParseListingPaginationCursor` + `TestSearchHonoursCursorParameter` close the round-trip loop |
| Rate-limit doc discrepancy misleads operators | High | Medium | `TestCapabilitiesShape` asserts `Notes` substring `"rate limit discrepancy"` — operators see the disclosure |
| Goroutine leak on ctx cancel | Medium | Medium | NFR-ADP-003 + `goleak` test catches at CI time |
| Parse-path slow / allocates excessively | Low | Low | NFR-ADP-001 benchmark + 250-allocs ceiling catches at CI weekly |
| HTTP timeout too short for slow Reddit | Low | Low | 10s default is configurable via `Options.HTTPClient`; caller's ctx wins when shorter |
| `Capabilities` accidentally returns mutable slice (caller mutates) | Low | Low | `TestCapabilitiesDeterministic` + `reflect.DeepEqual` catches mutation across calls |

---

## 7. Sequencing and Parallelisation

Tasks A1 → A2 → A3 are sequential (build on each other).
Tasks B1, B2, B3 can be done in any order (independent helpers).
Task B4 depends on A1.
Tasks C1, C2 can be done in parallel after A1.
Task C3 depends on C2.
Task D1 depends on A1 + errors.go from B3.
Tasks D2, D3 depend on D1, B1, B2, B3, B4, C2, C3.
Phase E (NFRs) runs after Phase D.
Phase F (REFACTOR) runs last.

In team mode, Phase A and Phase B and Phase C can be assigned to
three implementers in parallel; Phase D and onward serialise.

---

## 8. Quality Gates

Before SPEC is marked `implemented`:

- [ ] All 46 tests in §8 of spec.md pass (`go test ./internal/adapters/reddit/...`) — includes `TestSearchConcurrentSafe` for REQ-ADP-011 added in iteration 2.
- [ ] `go test -race ./internal/adapters/reddit/...` passes (REQ-ADP-011 concurrency contract requires race-detector clean).
- [ ] Coverage ≥ 85% (`go test -cover`).
- [ ] `go vet ./internal/adapters/reddit/...` clean.
- [ ] `golangci-lint run ./internal/adapters/reddit/...` clean.
- [ ] `BenchmarkParseListing25Docs` invoked as `go test -bench=BenchmarkParseListing25Docs -benchtime=10x -count=5 ./internal/adapters/reddit/...` on amd64; the median of the 5 reported per-op mean durations is ≤ 5ms; allocs/op ≤ 250.
- [ ] All MX tags applied per §6.7 plan; `@MX:REASON` present on
      ANCHOR + WARN.
- [ ] Capabilities.Notes contains all 4 documented substrings.
- [ ] No goroutine leak per `TestSearchNoGoroutineLeakOnCancel`.
- [ ] Stub-server p95 latency ≤ 200ms per `TestSearchE2ELatencyStubP95`.
- [ ] LSP gate (per `.moai/config/sections/quality.yaml` run-phase
      thresholds): zero errors, zero type errors, zero lint errors.

---

## 9. Out-of-Scope Reminders (from spec.md §7)

The implementer MUST NOT add (per HARD exclusions in spec.md):
- Retry orchestration (FAN-001 owns it).
- Caching (CACHE-001 owns it).
- Per-adapter custom Prometheus metrics (would amend OBS-001
  allowlist).
- Cross-cutting observability calls in the adapter (registry
  wrappedAdapter handles it).
- OAuth path (future ADP-001a).
- Subreddit-scoped or time-range filtering (future enhancement).
- Live network tests in CI (out of v0.1).

Drive-by refactors of `pkg/types`, `internal/adapters/registry.go`,
or `internal/obs/*` are FORBIDDEN — those are owned by SPEC-CORE-001
and SPEC-OBS-001 respectively.

---

*End of SPEC-ADP-001 plan.md v0.1*
