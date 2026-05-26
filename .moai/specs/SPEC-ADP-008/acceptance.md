# SPEC-ADP-008 Acceptance Criteria (Post-Hoc)

**SPEC**: SPEC-ADP-008 — Naver Suite Adapter
**Status**: implemented (2026-05-07)
**Format**: Given/When/Then per REQ + edge cases + Definition of Done

---

## 1. Acceptance Scenarios by REQ

### REQ-ADP8-001 — Adapter Interface Conformance

**AC-001: Compile-time interface assertion**

**AC-002: Name returns "naver"**

**AC-003: Capabilities deterministic + shape-correct**
- `SourceID="naver"`, `DisplayName="Naver"`,
  `DocTypes=[DocTypeArticle, DocTypePost]`,
  `SupportedLangs=["ko"]`, `SupportsSince=false`,
  `RequiresAuth=true`,
  `AuthEnvVars=["NAVER_CLIENT_ID","NAVER_CLIENT_SECRET"]`,
  `RateLimitPerMin=17`, `DefaultMaxResults=25`. Notes documents
  the supported verticals + default=blog + DataLab opt-in + 25k/day
  quota.

**AC-004: Healthcheck succeeds**

### REQ-ADP8-002 — Search Happy Path (Per Vertical)

**AC-005: Blog happy path**
- Stub returns `testdata/search_response_blog.json`,
  `Filters=nil` (default blog) → 25 NormalizedDocs; each
  `Validate()` returns nil.

**AC-006: News happy path**
- `Filters=[{naver_vertical, "news"}]` →
  `testdata/search_response_news.json` → 25 NormalizedDocs.

**AC-007: Web happy path**
- `naver_vertical=web` → `testdata/search_response_web.json` → 25 NormalizedDocs.

**AC-008: Shop happy path**
- `naver_vertical=shop` → `testdata/search_response_shop.json` → 25 NormalizedDocs.

**AC-009: URL parameters**
- URL contains `query`, `display`, `start`, `sort=sim` (default).

**AC-010: display clamp / default**
- `MaxResults=500` → `display=100`.
- `MaxResults=0` → `display=25`.

### REQ-ADP8-003 — Authentication (Headers + Constructor)

**AC-011: 4 headers on every request**
- `User-Agent`, `Accept: application/json`,
  `X-Naver-Client-Id: <id>`, `X-Naver-Client-Secret: <secret>`.

**AC-012: Constructor rejects missing env**
- `New(Options{ClientID:"", ClientSecret:""})` with env unset →
  `ErrAuthMissing`.

**AC-013: SkipAuthCheck-equivalent via Options bypass**
- `New(Options{ClientID:"test-id", ClientSecret:"test-secret"})` →
  no error.

### REQ-ADP8-004 — Vertical Routing

**AC-014: Default vertical = blog**
- `Filters=nil` → URL path is `/v1/search/blog.json`.

**AC-015: Vertical filter dispatches to correct endpoint**
- 5 valid verticals → 5 distinct URL paths.

**AC-016: Unknown vertical → ErrInvalidVertical**
- `Filters=[{naver_vertical, "unknown"}]` →
  `*SourceError{Permanent, Cause: ErrInvalidVertical}`; ZERO HTTP.

### REQ-ADP8-005 — Per-Vertical NormalizedDoc Mapping

**AC-017: Blog field mapping**
- Each doc: `Author=stripHTML(bloggername)`, `PublishedAt =
  time.Parse("20060102", postdate)`, `DocType=DocTypePost`.

**AC-018: News field mapping**
- Each doc: `URL = originallink || link` (originallink preferred
  when non-empty), `PublishedAt = time.Parse(time.RFC1123, pubDate)`,
  `DocType=DocTypeArticle`.

**AC-019: Web field mapping**
- Each doc: bare title/link/description; `Author=""`;
  `PublishedAt=zero`; `DocType=DocTypeArticle`.

**AC-020: Shop field mapping**
- Each doc: Metadata has `{lprice, hprice, mallName, productId,
  category1, category2, category3, category4, image}`;
  `DocType=DocTypeOther`.

**AC-021: Common fields**
- All docs: `SourceID="naver"`, `Lang="ko"`, `Score=0.5`,
  `RetrievedAt=now()`, `Hash=""`.

**AC-022: HTML strip applied**
- Title/Body/Snippet have `<b>` tags removed and `&amp; &lt; &gt;
  &quot; &#39;` entities decoded.

**AC-023: Pagination cursor**
- Last doc `Metadata["next_cursor"] = strconv.Itoa(currentStart +
  requestedDisplay)`.

### REQ-ADP8-006 — User-Agent + Accept Headers

**AC-024: Custom UA + Accept**
**AC-025: UA version configurable**

### REQ-ADP8-007 — Sort Filter (Optional)

**AC-026: Default sort=sim**
**AC-027: Opt-in sort=date**
- `Filters=[{sort, "date"}]` → URL has `sort=date`.

**AC-028: Other sort values dropped silently → sim**
- `Filters=[{sort, "random"}]` → URL has `sort=sim`.

### REQ-ADP8-008 — Empty Query / Invalid Cursor / Invalid Vertical Rejection

**AC-029: Empty / whitespace Text rejected with zero HTTP**

**AC-030: Invalid cursor rejected with zero HTTP**
- `Cursor` in `["abc", "0", "-1", "1001"]` → ErrInvalidCursor
  (range is `[1, 1000]`).

**AC-031: Invalid vertical rejected with zero HTTP** (per AC-016)

### REQ-ADP8-009 — HTTP 429 / 4xx / 5xx Mapping

**AC-032: 429 → RateLimited with Retry-After**
**AC-033: 4xx → Permanent** (table over 400/401/403/404)
**AC-034: 5xx → Unavailable**
**AC-035: Connection refused → Unavailable + HTTPStatus=0**

### REQ-ADP8-010 — DataLab Opt-In (Optional)

**AC-036: DataLab POST happy path**
- `Filters=[{naver_vertical, "datalab"}]` + `q.Text` = JSON
  payload → POST to `/v1/datalab/search` with body parsed from
  `q.Text`; returns one NormalizedDoc per `keywordGroups` row;
  `Metadata["datalab_data"]` contains time-series array.

**AC-037: Malformed q.Text rejected**
- `q.Text` not valid JSON → `*SourceError{Permanent}`.

**AC-038: Empty results array → (nil, nil)**

### REQ-ADP8-011 — Redirect Allowlist

**AC-039: Allowlist `{openapi.naver.com}` enforced**

**AC-040: Cross-domain rejected**

**AC-041: Chain over 3 hops rejected**

### REQ-ADP8-012 — Concurrent Search Safety (State-Driven)

**AC-042: 50 goroutines race-clean**

---

## 2. NFR Acceptance

### NFR-ADP8-001 — Parse-Path Performance

**AC-N01: Benchmark within target**
- `BenchmarkParseBlogResponse25Items` median ≤ 5 ms; allocs/op ≤ 500.

### NFR-ADP8-002 — E2E p95 (Stub)

**AC-N02: p95 ≤ 200ms** over 100 invocations.

### NFR-ADP8-003 — Race-Clean Concurrent Workload

**AC-N03: TestSearchConcurrentSafe under -race**

### NFR-ADP8-004 — No Goroutine Leak

**AC-N04: goleak.VerifyNone + TestMain VerifyTestMain**

---

## 3. Edge Cases

**EC-001: News with empty originallink**
- `originallink=""` → URL = `link`; doc still validates.

**EC-002: Blog with malformed postdate**
- `postdate=""` or `"abc"` → `PublishedAt = zero`; no error.

**EC-003: HTML entities + nested `<b>` tags**
- `stripHTML` handles `<b>foo<b>bar</b></b>` → `"foobar"`.

**EC-004: Quota exhaustion semantics**
- 429 surfaces `RetryAfter`; fanout owns retry decisions; adapter
  is stateless.

**EC-005: Auth-related 401 distinct from query 4xx**
- categorizeStatus carries an inline note that 401/403 likely
  indicate auth failure; both map to `CategoryPermanent` (consumer
  cannot fix by retry).

**EC-006: DocTypeProduct unavailable**
- Shop maps to `DocTypeOther`; consumers check
  `Metadata["naver_vertical"] == "shop"` to distinguish.

---

## 4. Coverage Matrix

| REQ-ID | EARS Pattern | Acceptance Criteria | Implementation |
|--------|-------------|---------------------|----------------|
| REQ-ADP8-001 | Ubiquitous | AC-001..004 | `naver.go`, `naver_test.go` |
| REQ-ADP8-002 | Event-Driven | AC-005..010 | `search.go`, `search_test.go` |
| REQ-ADP8-003 | Ubiquitous | AC-011..013 | `naver.go::New` validation, `client.go::doRequest` |
| REQ-ADP8-004 | Event-Driven | AC-014..016 | `search.go` (vertical dispatch), `errors.go::ErrInvalidVertical` |
| REQ-ADP8-005 | Ubiquitous | AC-017..023 | `parse.go::parseBlogItem` / `parseNewsItem` / `parseWebItem` / `parseShopItem`, `strip.go::stripHTML` |
| REQ-ADP8-006 | Ubiquitous | AC-024..025 | `client.go::doRequest` |
| REQ-ADP8-007 | Optional | AC-026..028 | `search.go` (sort handling) |
| REQ-ADP8-008 | Unwanted | AC-029..031 | `search.go` (input validation) |
| REQ-ADP8-009 | Event-Driven | AC-032..035 | `client.go::categorizeStatus`, `errors.go::parseRetryAfter` |
| REQ-ADP8-010 | Optional | AC-036..038 | `datalab.go::searchDataLab` |
| REQ-ADP8-011 | Optional | AC-039..041 | `client.go::redirectAllowlist` |
| REQ-ADP8-012 | State-Driven | AC-042 | `search_test.go::TestSearchConcurrentSafe` |
| NFR-ADP8-001 | Performance | AC-N01 | `bench_test.go::BenchmarkParseBlogResponse25Items` |
| NFR-ADP8-002 | Latency | AC-N02 | `search_test.go::TestSearchE2ELatencyStubP95` |
| NFR-ADP8-003 | Race-clean | AC-N03 | `search_test.go::TestSearchConcurrentSafe` |
| NFR-ADP8-004 | Resource | AC-N04 | `search_test.go::TestSearchNoGoroutineLeakOnCancel`, `bench_test.go::TestMain` |

---

## 5. Definition of Done

- [x] All 14 EARS REQs have passing tests.
- [x] All 4 NFRs have passing measurements.
- [x] `go test ./internal/adapters/naver/...` exits 0.
- [x] `go test -race ./internal/adapters/naver/...` exits 0.
- [x] `go test -cover` reports ≥ 85%.
- [x] `go vet` and `golangci-lint run` clean.
- [x] `BenchmarkParseBlogResponse25Items` median ≤ 5ms;
      allocs/op ≤ 500.
- [x] All 5 verticals (blog/news/web/shop/datalab) have happy-path
      tests.
- [x] HTML strip table has ≥ 8 inputs.
- [x] DataLab POST path test exists.
- [x] Client secret never appears in error messages or slog records.
- [x] MX tags applied per spec.md plan.
- [x] Capabilities.Notes documents verticals + default + DataLab +
      25k/day quota.
- [x] `var _ types.Adapter = (*Adapter)(nil)` present.
- [x] No drive-by changes outside `internal/adapters/naver/`.
- [x] SPEC status updated to `implemented` (2026-05-07).

---

*End of SPEC-ADP-008 acceptance.md (post-hoc, v1.0)*
