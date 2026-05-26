# SPEC-ADP-007 Acceptance Criteria (Post-Hoc)

**SPEC**: SPEC-ADP-007 — SearXNG Bridge Adapter
**Status**: implemented (2026-05-07)
**Format**: Given/When/Then per REQ + edge cases + Definition of Done

---

## 1. Acceptance Scenarios by REQ

### REQ-ADP7-001 — Adapter Interface Conformance

**AC-001: Compile-time interface assertion**

**AC-002: Name returns "searxng"**

**AC-003: Capabilities deterministic + shape-correct**
- `SourceID="searxng"`, `DisplayName="SearXNG"`,
  `DocTypes=[DocTypeArticle]` (iteration-2 H1),
  `SupportedLangs=nil`, `SupportsSince=false`,
  `RequiresAuth=false`, `RateLimitPerMin=0` (iteration-2 H3),
  `DefaultMaxResults=25`. Notes documents the JSON-format
  precondition and engine list.

**AC-004: Healthcheck succeeds**
- Stub `httptest.Server` reachable → nil error.
- Healthcheck target defaults derived from `baseURL` host:port via
  `healthcheckHostFromBase` helper (iteration-2 H4).

### REQ-ADP7-002 — Search Happy Path

**AC-005: Happy path with results**
- Stub returns `testdata/search_response.json` → 25 NormalizedDocs;
  each `Validate()` returns nil.

**AC-006: URL parameters required**
- URL contains `q=<text>`, `format=json`.

**AC-007: pageno parameter (iteration-2 H5)**
- `Cursor=""` (default) → URL omits `pageno`.
- `Cursor="1"` → URL has explicit `pageno=1`.
- `Cursor="3"` → URL has `pageno=3`.

**AC-008: Empty query rejected with zero HTTP**

**AC-009: Invalid cursor rejected with zero HTTP**

### REQ-ADP7-003 — HTTP 429 Rate-Limit Mapping

**AC-010: 429 + Retry-After → RateLimited**
**AC-011: No Retry-After defaults to 5s**
**AC-012: Capped at 60s**
**AC-013: No internal retry**

### REQ-ADP7-004 — HTTP 4xx Permanent + 5xx Unavailable + Network

**AC-014: 4xx → Permanent** (table over 400/401/404)
**AC-015: 5xx → Unavailable** (table over 500/503)
**AC-016: Connection refused → Unavailable, HTTPStatus=0**

### REQ-ADP7-005 — NormalizedDoc Field Mapping

**AC-017: Field mapping table**
- Each NormalizedDoc has stable SHA256-derived `ID`,
  `SourceID="searxng"`, `URL`, `Title`, `Body=content`,
  `Snippet=truncate(content, 280)`,
  `Score=clamp(result.score, 0, 1)`, `Lang=""`,
  `DocType=DocTypeArticle`, `Author=""`,
  `PublishedAt = parsed RFC 3339 or zero`,
  `Citations=nil`, `Hash=""`.

**AC-018: Engine metadata required**
- Every doc Metadata has `{engine, engines, category}`.

**AC-019: Engines fallback (iteration-2 M4)**
- When `engines` field is null, missing, or empty array, the parser
  sets `Metadata["engines"] = []string{result.engine}`.

**AC-020: SHA256-derived ID determinism**
- Same fixture parsed twice → byte-equal IDs across all results.

**AC-021: Pagination cursor on last doc**
- Non-empty results → last doc `Metadata["next_cursor"] =
  strconv.Itoa(currentPage + 1)`.

**AC-022: No cursor on empty results (last-page inference)**
- Zero results → no doc has `next_cursor`; iteration termination
  signal (iteration-2 M3).

**AC-023: Hash always empty**

### REQ-ADP7-006 — User-Agent + Accept Headers

**AC-024: Custom UA**
**AC-025: `Accept: application/json`**
**AC-026: UA version configurable**

### REQ-ADP7-007 — Limiter 403/429 Dual Mapping (Optional)

**AC-027: 403 WITH Retry-After → RateLimited (promotion)**
- Stub returns 403 + `Retry-After: 10` →
  `errors.Is(err, types.ErrRateLimited)` AND `RetryAfter=10s`.

**AC-028: 403 WITHOUT Retry-After → Permanent**
- Stub returns 403, no `Retry-After` header →
  `errors.Is(err, types.ErrPermanent)`.

**AC-029: 429 always RateLimited (regardless of Retry-After)**
- Inherited from REQ-ADP7-003.

### REQ-ADP7-008 — Empty Query / Invalid Cursor Rejection (Unwanted)

**AC-030: Empty / whitespace Text rejected with zero HTTP**

**AC-031: Invalid cursor rejected with zero HTTP**
- `Cursor` in `["abc", "-1", "1.5"]` → ErrInvalidCursor; ZERO requests.

### REQ-ADP7-009 — Local Redirect Allowlist (Optional)

**AC-032: Allowlist hosts**
- `searxng`, `localhost`, `127.0.0.1` permitted; different-port
  redirects within allowlist permitted (iteration-2 M6).

**AC-033: Cross-host rejected → Permanent**

**AC-034: Chain over 3 hops rejected**

### REQ-ADP7-010 — Concurrent Search Safety (State-Driven)

**AC-035: 50 goroutines race-clean**

### REQ-ADP7-011 — JSON-Format Precondition (Ubiquitous)

**AC-036: URL hardcodes format=json**
- Every outbound URL contains `format=json`; deployed SearXNG must
  have JSON enabled in `search.formats` (settings.yml).

---

## 2. NFR Acceptance

### NFR-ADP7-001 — Parse-Path Performance

**AC-N01: Benchmark within target**
- `BenchmarkParseSearch25Results` median ≤ 5 ms;
  `allocs/op ≤ 50` per result (higher than ADP-001 ≤20/doc due to
  engines slice copy + SHA256 ID derivation; iteration-2 M5).

### NFR-ADP7-002 — E2E p95 (Stub)

**AC-N02: p95 ≤ 200ms** over 100 invocations.

### NFR-ADP7-003 — No Goroutine Leak on Cancellation

**AC-N03: goleak.VerifyNone after mid-flight cancel + TestMain goleak**

### NFR-ADP7-004 — Race-Clean Concurrent Workload

**AC-N04: `TestSearchConcurrentSafe` under -race**

---

## 3. Edge Cases

**EC-001: Empty results array → no next_cursor**
- Iteration termination signal; caller stops paginating.

**EC-002: PublishedDate parse failure → zero PublishedAt**
- Defensive: malformed `publishedDate` does not fail the parse;
  doc still returned with zero `PublishedAt`.

**EC-003: Score outside [0,1]**
- Upstream engine score > 1.0 → clamped to 1.0; negative → clamped to 0.

**EC-004: Different-port redirect within allowlist permitted**
- `searxng:8080` → `searxng:8081` redirect allowed (iteration-2 M6).

**EC-005: parseRetryAfter for 403 promotion**
- Same parser as 429; uses RFC 7231 §7.1.3 (integer + HTTP-date,
  60s cap, 5s default).

---

## 4. Coverage Matrix

| REQ-ID | EARS Pattern | Acceptance Criteria | Implementation |
|--------|-------------|---------------------|----------------|
| REQ-ADP7-001 | Ubiquitous | AC-001..004 | `searxng.go`, `searxng_test.go` |
| REQ-ADP7-002 | Event-Driven | AC-005..009 | `search.go`, `search_test.go` |
| REQ-ADP7-003 | Event-Driven | AC-010..013 | `client.go::categorizeStatus`, `errors.go::parseRetryAfter` |
| REQ-ADP7-004 | Event-Driven | AC-014..016 | `client.go::categorizeStatus` |
| REQ-ADP7-005 | Ubiquitous | AC-017..023 | `parse.go::parseSearch` |
| REQ-ADP7-006 | Ubiquitous | AC-024..026 | `client.go::doRequest` |
| REQ-ADP7-007 | Optional | AC-027..029 | `client.go::categorizeStatus` (403 promotion path) |
| REQ-ADP7-008 | Unwanted | AC-030..031 | `search.go` (input validation) |
| REQ-ADP7-009 | Optional | AC-032..034 | `client.go::redirectAllowlist` |
| REQ-ADP7-010 | State-Driven | AC-035 | `search_test.go::TestSearchConcurrentSafe` |
| REQ-ADP7-011 | Ubiquitous | AC-036 | `search.go` (URL builder) |
| NFR-ADP7-001 | Performance | AC-N01 | `bench_test.go::BenchmarkParseSearch25Results` |
| NFR-ADP7-002 | Latency | AC-N02 | `search_test.go::TestSearchE2ELatencyStubP95` |
| NFR-ADP7-003 | Resource | AC-N03 | `search_test.go::TestSearchNoGoroutineLeakOnCancel`, `bench_test.go::TestMain` |
| NFR-ADP7-004 | Race-clean | AC-N04 | `search_test.go::TestSearchConcurrentSafe` |

---

## 5. Definition of Done

- [x] All 11 EARS REQs have passing tests.
- [x] All 4 NFRs have passing measurements.
- [x] `go test ./internal/adapters/searxng/...` exits 0.
- [x] `go test -race ./internal/adapters/searxng/...` exits 0.
- [x] `go test -cover` reports ≥ 85%.
- [x] `go vet` and `golangci-lint run` clean.
- [x] `BenchmarkParseSearch25Results` median ≤ 5ms;
      allocs/op ≤ 50 per result.
- [x] 403-with-Retry-After promotion verified.
- [x] SHA256 ID determinism verified.
- [x] Engines fallback verified.
- [x] MX tags applied per spec.md §6.7 (6 entries post iteration-2 M7).
- [x] Capabilities.Notes documents JSON-format precondition + engine
      list.
- [x] `var _ types.Adapter = (*Adapter)(nil)` present.
- [x] No drive-by changes outside `internal/adapters/searxng/`.
- [x] SPEC status updated to `implemented` (2026-05-07).

---

*End of SPEC-ADP-007 acceptance.md (post-hoc, v1.0)*
