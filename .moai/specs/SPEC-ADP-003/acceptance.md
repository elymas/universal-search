# SPEC-ADP-003 Acceptance Criteria (Post-Hoc)

**SPEC**: SPEC-ADP-003 — arXiv + Paper Search Adapter
**Status**: implemented (2026-05-07)
**Format**: Given/When/Then per REQ + edge cases + Definition of Done

---

## 1. Acceptance Scenarios by REQ

### REQ-ADP3-001 — Adapter Interface Conformance

**AC-001: Compile-time interface assertion**
- Given the package declares `var _ types.Adapter = (*Adapter)(nil)`,
- When `go build ./internal/adapters/arxiv/...` runs,
- Then the build succeeds.

**AC-002: Name returns "arxiv"**
- Given a constructed `*Adapter`,
- When `Name()` is called,
- Then the return value equals `"arxiv"`.

**AC-003: Capabilities deterministic + shape-correct**
- Given a constructed `*Adapter`,
- When `Capabilities()` is called twice,
- Then both calls return `reflect.DeepEqual` results AND the
  descriptor has `SourceID="arxiv"`, `DisplayName="arXiv"`,
  `DocTypes=[DocTypePaper]`, `SupportedLangs=nil`, `SupportsSince=true`,
  `RequiresAuth=false`, `RateLimitPerMin=20`, `DefaultMaxResults=25`,
  and `Notes` contains all 5 documented substrings.

**AC-004: Healthcheck succeeds**
- Given an httptest.Server bound to 127.0.0.1:0 via
  `Options.HealthcheckTarget`,
- When `Healthcheck(ctx)` is called,
- Then it returns nil.

**AC-005: New applies MinRequestInterval default**
- Given `New(Options{})`,
- When the constructor returns,
- Then `a.minInterval == 3 * time.Second`.

### REQ-ADP3-002 — Search Happy Path

**AC-006: Happy path 25 entries**
- Given a stub returning `testdata/search_response.xml`,
- When `Search(ctx, types.Query{Text: "transformer", MaxResults: 25})`
  is called,
- Then 25 NormalizedDocs are returned; each passes `Validate()`.

**AC-007: URL parameters required**
- Given the stub captures the request URL,
- When `Search` is invoked,
- Then the URL contains `search_query`, `max_results`,
  `sortBy=relevance`, `sortOrder=descending`.

**AC-008: max_results clamp and default**
- Given `MaxResults=500` → URL has `max_results=100`.
- Given `MaxResults=0` → URL has `max_results=25`.

**AC-009: start parameter cursor round-trip**
- Given `Cursor="50"` → URL has `start=50`.
- Given `Cursor=""` → URL has no `start` parameter or `start=0`.

**AC-010: Overshoot returns empty without error**
- Given a fixture where `start > totalResults`,
- When `Search` is invoked,
- Then `(nil, nil)` is returned.

### REQ-ADP3-003 — HTTP 429 Mapping

**AC-011: Integer Retry-After → 30s**
- Stub returns 429 + `Retry-After: 30` → `RetryAfter=30s`,
  `Category=CategoryRateLimited`.

**AC-012: HTTP-date Retry-After → 25-35s window**
- Stub returns 429 with HTTP-date 30s ahead → `RetryAfter ∈ (25s, 35s)`.

**AC-013: No header defaults to 5s**
- Stub returns 429 with no `Retry-After` → `RetryAfter=5s`.

**AC-014: Capped at 60s**
- Stub returns 429 with `Retry-After: 999` → `RetryAfter=60s`.

**AC-015: No internal retry**
- Stub counts inbound requests, returns 429 every time → exactly 1
  request observed.

### REQ-ADP3-004 — HTTP 4xx → Permanent

**AC-016: 400 validation error**
- Stub returns 400 + `testdata/search_response_400_error.xml` body →
  `errors.Is(err, types.ErrPermanent)` AND `HTTPStatus=400`.

**AC-017: 401, 403, 404 all map to Permanent**
- Table-driven over 401/403/404 → each asserts
  `errors.Is(err, types.ErrPermanent)` AND matching HTTPStatus.

**AC-018: No internal retry on 4xx**
- Exactly 1 request observed.

### REQ-ADP3-005 — HTTP 5xx and Network Failure

**AC-019: 500 and 503 → Unavailable**
- Stub returns 500 or 503 → `errors.Is(err,
  types.ErrSourceUnavailable)` AND matching HTTPStatus.

**AC-020: Connection refused → Unavailable with HTTPStatus=0**
- Stub closed before request →
  `errors.Is(err, types.ErrSourceUnavailable)` AND `HTTPStatus=0`.

**AC-021: Underlying error preserved**
- `errors.Unwrap(srcErr).Error()` contains inner cause text.

### REQ-ADP3-006 — NormalizedDoc Field Mapping

**AC-022: Field mapping over 5 fixtures**
- Table-driven (full entry with DOI + journal_ref, no DOI,
  multi-author, multi-version `<id>`, LaTeX title) — each asserts
  every documented field per §6.3 mapping table.

**AC-023: ID strip prefix**
- Entry `<id>http://arxiv.org/abs/2403.12345v2</id>` → `ID =
  "2403.12345v2"` (prefix stripped).

**AC-024: Multi-version ID preserves vN suffix**
- `<id>` ending in `v15` → `ID` ends in `v15`.

**AC-025: Whitespace collapse**
- 5-input table (newlines, multi-space, leading/trailing, control
  chars, Unicode) — `Title` matches `strings.Join(strings.Fields(s),
  " ")`.

**AC-026: Score constant = 0.5**
- Every returned doc has `Score == 0.5` exactly.

**AC-027: DOI in arxiv namespace populates Metadata**
- `<arxiv:doi>10.xyz/abc</arxiv:doi>` → `Metadata["doi"] == "10.xyz/abc"`.

**AC-028: No DOI omits key**
- Entry without `<arxiv:doi>` → `Metadata` does NOT contain `"doi"`
  key.

**AC-029: Authors list**
- Entry with 5 `<author><name>` → `Metadata["authors"]` is
  `[]string` length 5; `Author = Metadata["authors"][0]`.

**AC-030: Pagination cursor on last doc**
- `start=0, totalResults=100, len(entries)=25` → last doc
  `Metadata["next_cursor"] == "25"`.

**AC-031: No cursor on last page**
- `start=80, totalResults=100, len(entries)=20` → no doc has
  `next_cursor`.

**AC-032: Hash always empty**
- Every returned `doc.Hash == ""`.

**AC-033: All 6 required Metadata keys present**
- `{arxiv_id, authors, primary_category, categories, published_at,
  updated_at}` present in every doc.

**AC-034: Malformed XML → Permanent**
- Truncated XML → `*SourceError{Permanent}` with cause containing
  "xml" or "EOF".

### REQ-ADP3-007 — Category Filter (Optional)

**AC-035: Category filter prepended**
- `Filters=[{category, "cs.AI"}]`, `Text="transformer"` → URL has
  `search_query=cat%3Acs.AI+AND+transformer`.

**AC-036: Category absent or empty → verbatim text**
- `Filters=nil` OR `Filters=[{category, ""}]` → URL has
  `search_query=transformer`.

**AC-037: Unknown filter ignored**
- `Filters=[{nsfw, "true"}]` → URL has `search_query=transformer`.

### REQ-ADP3-008 — Empty Query / Invalid Start Rejection

**AC-038: Empty / whitespace Text rejected with zero HTTP**
- `Text` in `["", "   ", "\t\n  \r"]` → `*SourceError{Permanent,
  Cause: ErrInvalidQuery}`; ZERO requests; no rate-limit slot
  consumed (follow-up valid Search call does not wait).

**AC-039: Invalid start cursor rejected with zero HTTP**
- `Cursor` in `["abc", "-1", "1.5", "1e3", "  "]` →
  `*SourceError{Permanent, Cause: ErrInvalidStart}`; ZERO requests.

### REQ-ADP3-009 — User-Agent + Accept Headers

**AC-040: Default UA and Accept**
- Captured `User-Agent` starts with `"usearch/"` and contains
  `"(+https://github.com/elymas/universal-search)"` AND `Accept ==
  "application/atom+xml"`.

**AC-041: Custom UA version propagates**
- `Options.UserAgentVersion = "v0.2-rc1"` → UA contains
  `"usearch/v0.2-rc1"`.

### REQ-ADP3-010 — Redirect Allowlist (Optional)

**AC-042: Allowlist redirect followed**
- Server A 302 → server B (Host rewritten to `export.arxiv.org`) →
  docs from server B returned.

**AC-043: Cross-domain redirect rejected**
- Server A 302 to `attacker.com` → `errors.Is(err,
  types.ErrPermanent)` AND error contains `"cross-domain redirect"`.

**AC-044: Chain over 3 hops rejected**
- 4-hop chain → error contains `"too many redirects"`.

### REQ-ADP3-011 — Concurrent Search Safety

**AC-045: 50 goroutines race-clean**
- 50 goroutines × 1 Search against shared `*Adapter` constructed
  with `Options{MinRequestInterval: 0}` against one stub → no race
  alarms under `-race`; 50 requests observed; every goroutine
  receives `(docs, nil)` with `len(docs) == 25`.

### REQ-ADP3-012 — Rate-Limit Serialisation (State-Driven)

**AC-046: 3 sequential calls with 10ms interval**
- `Options{MinRequestInterval: 10ms}`; 3 sequential `Search` calls;
- Total elapsed ∈ `[20ms, 50ms]`; exactly 3 outbound requests.

**AC-047: Ctx cancel during wait returns within 5ms**
- `Options{MinRequestInterval: 10s}`; first call succeeds;
  second call has pre-cancelled ctx;
- Second call returns within 5ms with err satisfying
  `errors.Is(err, context.Canceled)`.

**AC-048: Per-instance state (no cross-instance serialisation)**
- Two separate `*Adapter` instances each with `MinRequestInterval=10s`
  call Search at same wall-clock time;
- Both return within 100ms (no serialisation across instances).

---

## 2. NFR Acceptance

### NFR-ADP3-001 — Parse-Path Performance

**AC-N01: Benchmark within target**
- `BenchmarkParseFeed25Entries` invoked as `go test -bench=...
  -benchtime=10x -count=5` → median ≤ 5 ms; `allocs/op ≤ 700`
  (XML floor amended from JSON's ≤ 500).

### NFR-ADP3-002 — E2E p95 (Stub)

**AC-N02: p95 ≤ 200ms**
- 100 invocations against stub with `MinRequestInterval=0` →
  `durations[94] ≤ 200ms`.

### NFR-ADP3-003 — Race-Clean Concurrent Workload

**AC-N03: TestSearchConcurrentSafe under -race**
- `go test -race` reports zero data-race alarms attributable to the
  arxiv package.

### NFR-ADP3-004 — No Goroutine Leak on Cancellation

**AC-N04: goleak verifies clean shutdown**
- `goleak.VerifyNone(t)` after a Search whose ctx was cancelled at
  50ms with stub delay 200ms.
- `TestMain` invokes `goleak.VerifyTestMain(m)`.

---

## 3. Edge Cases

**EC-001: Empty `<feed>` (zero entries)**
- `parseFeed` returns `(nil, "", nil)`.

**EC-002: Multi-version `<id>` with v15 suffix**
- ID preserves the version suffix.

**EC-003: Whitespace collapse with control chars**
- Control chars pass through; only whitespace runs collapsed.

**EC-004: LaTeX in title preserved verbatim**
- `$E=mc^2$` survives the parse pipeline; synthesis decides
  rendering.

**EC-005: Pre-cancelled ctx at function entry**
- Returns wrapped `context.Canceled` (NOT
  `ErrInvalidQuery`/`ErrInvalidStart`) — REQ-ADP5-009-style
  precedence applies in spirit although not explicitly REQ-ed here.

**EC-006: rate-limit slot NOT consumed by validation rejection**
- A rejected empty-query call does NOT advance `nextRequest`; a
  following valid query starts immediately.

---

## 4. Coverage Matrix

| REQ-ID | EARS Pattern | Acceptance Criteria | Implementation |
|--------|-------------|---------------------|----------------|
| REQ-ADP3-001 | Ubiquitous | AC-001..005 | `arxiv.go`, `arxiv_test.go` |
| REQ-ADP3-002 | Event-Driven | AC-006..010 | `search.go`, `search_test.go` |
| REQ-ADP3-003 | Event-Driven | AC-011..015 | `client.go::categorizeStatus`, `errors.go::parseRetryAfter` |
| REQ-ADP3-004 | Event-Driven | AC-016..018 | `client.go::categorizeStatus` |
| REQ-ADP3-005 | Event-Driven | AC-019..021 | `client.go::categorizeStatus` |
| REQ-ADP3-006 | Ubiquitous | AC-022..034 | `parse.go::parseFeed`, `parse.go::collapseWS` |
| REQ-ADP3-007 | Optional | AC-035..037 | `search.go::buildSearchQuery` |
| REQ-ADP3-008 | Unwanted | AC-038..039 | `search.go` (input validation) |
| REQ-ADP3-009 | Ubiquitous | AC-040..041 | `client.go::doRequest` |
| REQ-ADP3-010 | Optional | AC-042..044 | `client.go::redirectAllowlist` |
| REQ-ADP3-011 | State-Driven | AC-045 | `search_test.go::TestSearchConcurrentSafe` |
| REQ-ADP3-012 | State-Driven | AC-046..048 | `search.go::waitForRateSlot`, `rate_test.go` |
| NFR-ADP3-001 | Performance | AC-N01 | `bench_test.go::BenchmarkParseFeed25Entries` |
| NFR-ADP3-002 | Latency | AC-N02 | `search_test.go::TestSearchE2ELatencyStubP95` |
| NFR-ADP3-003 | Race-clean | AC-N03 | `search_test.go::TestSearchConcurrentSafe` |
| NFR-ADP3-004 | Resource | AC-N04 | `bench_test.go::TestMain`, `TestSearchNoGoroutineLeakOnCancel` |

---

## 5. Definition of Done

- [x] All 12 EARS REQs in spec.md §3 have at least one passing test.
- [x] All 4 NFRs have at least one passing measurement.
- [x] `go test ./internal/adapters/arxiv/...` exits 0.
- [x] `go test -race ./internal/adapters/arxiv/...` exits 0.
- [x] `go test -cover ./internal/adapters/arxiv/...` reports ≥ 85%.
- [x] `go vet ./internal/adapters/arxiv/...` clean.
- [x] `golangci-lint run ./internal/adapters/arxiv/...` clean.
- [x] `BenchmarkParseFeed25Entries` median ≤ 5ms; allocs/op ≤ 700.
- [x] `TestSearchE2ELatencyStubP95` passes.
- [x] `TestSearchNoGoroutineLeakOnCancel` passes.
- [x] Rate-limit tests `TestSearchRateLimitInterval`,
      `TestSearchRateLimitCtxCancel`, `TestSearchRateLimitPerInstance`
      pass.
- [x] MX tags applied per spec.md §6.7 plan.
- [x] Capabilities.Notes contains all 5 documented substrings.
- [x] Compile-time `var _ types.Adapter = (*Adapter)(nil)` present.
- [x] No drive-by changes outside `internal/adapters/arxiv/`.
- [x] SPEC status updated to `implemented` (2026-05-07).

---

*End of SPEC-ADP-003 acceptance.md (post-hoc, v1.0)*
