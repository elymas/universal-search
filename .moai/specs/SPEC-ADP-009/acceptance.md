# SPEC-ADP-009 Acceptance Criteria (Post-Hoc)

**SPEC**: SPEC-ADP-009 — KoreaNewsCrawler + Daum + Korean RSS Composite Adapter
**Status**: implemented (2026-05-04)
**Format**: Given/When/Then per REQ + edge cases + Definition of Done

---

## 1. Acceptance Scenarios by REQ

### REQ-ADP9-001 — Adapter Interface Conformance

**AC-001: Compile-time interface assertion**

**AC-002: Name returns "koreanews"**

**AC-003: Capabilities deterministic + shape-correct**
- `SourceID="koreanews"`,
  `DisplayName="Korean News (RSS + KoreaNewsCrawler + Daum)"`,
  `DocTypes=[DocTypeArticle]`, `SupportedLangs=["ko"]`,
  `SupportsSince=false`, `RequiresAuth=false`, `AuthEnvVars=nil`,
  `RateLimitPerMin=0`, `DefaultMaxResults=50`.
- `Notes` documents enabled sub-sources, RSS feed count, ToS
  posture for Daum, KNC sidecar status.

**AC-004: Healthcheck succeeds**

### REQ-ADP9-002 — RSS Sub-Source Search (Event-Driven)

**AC-005: Happy path with 3 feeds**
- 3 stub RSS servers returning small fixtures → merged
  NormalizedDocs returned; each `Validate()` returns nil; deduped.

**AC-006: gofeed parses RSS 2.0, Atom 1.0, JSON Feed 1.1**
- Table over 3 fixtures (one per format) → all yield NormalizedDocs.

**AC-007: Malformed XML → per-feed error isolated**
- One feed returns malformed XML; siblings succeed; the malformed
  feed's error appears in the per-feed-index error slice; the
  successful feeds' docs are returned.

**AC-008: Per-feed timeout enforced**
- `RSSPerFeedTimeout=200ms`; one feed sleeps 1s → that feed times
  out; sibling feeds return successfully.

**AC-009: Per-feed 4xx / 5xx → per-feed error**
- Stub returns 404 or 500 on one feed → that feed's error captured
  in per-feed-index error slice; siblings succeed.

### REQ-ADP9-003 — RSS Configuration Surface (Optional)

**AC-010: USEARCH_ADP009_RSS_FEEDS JSON array parsing**
- `USEARCH_ADP009_RSS_FEEDS='["http://a","http://b"]'` → Options
  has 2 feeds.

**AC-011: USEARCH_ADP009_RSS_FEEDS comma-list parsing**
- `USEARCH_ADP009_RSS_FEEDS=http://a,http://b` → Options has 2
  feeds.

**AC-012: 32-feed cap enforced**
- 40 feeds configured → first 32 kept; slog WARN emitted on
  truncation.

**AC-013: Empty list with RSS enabled → ErrEmptyRSSFeedList**
- `Options{RSSEnabled: true, RSSFeeds: nil}` →
  `*SourceError{Permanent, Cause: ErrEmptyRSSFeedList}`.

### REQ-ADP9-004 — Sub-Source Dispatch (Event-Driven)

**AC-014: RSS only**
- `Options{RSSEnabled: true, ...}` → only RSS worker invoked.

**AC-015: RSS + KNC**
- Both enabled → both workers invoked in parallel; results merged.

**AC-016: None enabled → empty result**
- All flags false → `(nil, nil)`.

### REQ-ADP9-005 — Empty Query Rejection (Unwanted)

**AC-017: Empty / whitespace Text rejected with zero HTTP**
- No HTTP requests to any sub-source.

### REQ-ADP9-006 — Daum Stub (Unwanted/Optional)

**AC-018: Daum disabled by default → no-op**
- `Options{DaumEnabled: false}` → Daum worker not invoked; `(nil, nil)`.

**AC-019: Daum enabled → ErrDaumDisabled regardless**
- `Options{DaumEnabled: true}` → `*SourceError{Permanent, Cause:
  ErrDaumDisabled, Notes: "subsource: daum"}`.
- Acceptance includes a table test over both `DaumEnabled` values
  to verify the stub deliberately ignores the flag.

**AC-020: Capabilities.Notes substring documents Daum status**

### REQ-ADP9-007 — KNC Sidecar HTTP Client (Optional)

**AC-021: KNC disabled by default → no-op**
- `Options{KNCEnabled: false}` → `(nil, nil)`.

**AC-022: KNC enabled, sidecar 503 → ErrKNCSidecarDown**
- Stub sidecar returns 503 → `*SourceError{Unavailable, Cause:
  ErrKNCSidecarDown}`.

**AC-023: KNC enabled, sidecar 200 → docs decoded**
- Stub returns 200 with KNC JSON fixture → NormalizedDocs returned.

**AC-024: 4xx → Permanent**
- Stub returns 400 → `*SourceError{Permanent}`.

**AC-025: 5xx → Unavailable**
- Stub returns 502 → `*SourceError{Unavailable}`.

**AC-026: Ctx cancel mid-flight**
- Stub delays 200ms; ctx cancelled at 50ms → wrapped
  `context.Canceled`.

### REQ-ADP9-008 — Korean Locale Detection (Ubiquitous)

**AC-027: Hangul ratio table**
- Pure Korean text → `Lang="ko"`.
- Pure English text → `Lang=""`.
- Mixed 50/50 Korean/English → `Lang="ko"` (≥0.30 threshold).
- Mixed 20/80 → `Lang=""` (<0.30).
- Empty string → `Lang=""`.
- Whitespace-only → `Lang=""`.

### REQ-ADP9-009 — Intra-Adapter Dedup (Optional)

**AC-028: URL canonicalization dedup**
- Two RSS items with same URL different content → first-occurrence
  wins; second dropped.

**AC-029: Tracking-param-stripped dedup**
- `?utm_source=...` variants of the same URL → deduped.

**AC-030: CanonicalHash fallback on unparseable URL**

**AC-031: Deterministic byte-equal output**
- Repeated dedup of same input → byte-equal slice.

### REQ-ADP9-010 — Composite Field Mapping (Ubiquitous)

**AC-032: All sub-sources merge into single result**
- Common fields: `SourceID="koreanews"`, `Score=0.5`,
  `DocType=DocTypeArticle`, `Hash=""`.
- Per-doc `Metadata["subsource"]` carries actual sub-source name
  (`rss`, `knc`, `daum`).
- Result sorted by `PublishedAt` descending then `SourceID`
  ascending.

### REQ-ADP9-011 — Concurrent Search Safety (State-Driven)

**AC-033: 50 goroutines race-clean**
- `concurrent_test.go::TestSearchConcurrentSafe` — 50 caller
  goroutines × 1 Search against stub feed servers; `-race` clean.

### REQ-ADP9-012 — User-Agent + Accept Headers (Event-Driven)

**AC-034: Custom UA on RSS + KNC requests**
- `User-Agent` starts with `"usearch/"` on every outbound request.

### REQ-ADP9-013 — Sub-Source Enable Flags via Env (Optional)

**AC-035: Env-var enable flags applied to Options**
- `USEARCH_ADP009_RSS_ENABLED=true`,
  `USEARCH_ADP009_DAUM_ENABLED=false`,
  `USEARCH_ADP009_KNC_ENABLED=false` → matches default Options.

---

## 2. NFR Acceptance

### NFR-ADP9-001 — Parse-Path Performance

**AC-N01: BenchmarkParseRSSFeed10Items within target**
- Median ≤ 5 ms; allocs/op ≤ 500.

### NFR-ADP9-002 — Race-Clean Concurrent Workload

**AC-N02: TestSearchConcurrentSafe under -race**

### NFR-ADP9-003 — No Goroutine Leak

**AC-N03: goleak.VerifyNone after mid-flight cancel + TestMain
VerifyTestMain**
- Including the errgroup workers — `eg.Wait()` ensures no goroutine
  outlives the Search call.

### NFR-ADP9-004 — Per-Feed Timeout Bound

**AC-N04: Slowest feed bounded by RSSPerFeedTimeout**
- 1 feed delayed 5s with `RSSPerFeedTimeout=200ms` → Search returns
  within 300ms; that feed's error captured; siblings succeed.

---

## 3. Edge Cases

**EC-001: All 3 sub-sources fail**
- RSS error + KNC sidecar 503 + Daum disabled → composite
  `*SourceError` returned (e.g., joining all sub-source errors).

**EC-002: One feed empty, others populated**
- Empty feed contributes 0 docs; total result is sum of non-empty
  feeds.

**EC-003: gofeed encoding warning**
- Feed declares EUC-KR encoding; after `utf8.ValidString` fails →
  `Metadata["encoding_warning"]` annotation; no conversion attempt.

**EC-004: Hangul threshold boundary**
- Exactly 30% Hangul → `Lang="ko"` (inclusive boundary).

**EC-005: Daum env flag plumbed but unused**
- `USEARCH_ADP009_DAUM_ENABLED=true` → Daum worker invoked →
  returns `ErrDaumDisabled` regardless. The flag is for future
  consumption only.

**EC-006: KNC sidecar HTTP contract**
- POST to `${KNCBaseURL}/search` with `{query, max_results}`.
- Expected response: `{articles: [{title, url, content, ...}, ...]}`
  OR `{error, message}` for failures.

---

## 4. Coverage Matrix

| REQ-ID | EARS Pattern | Acceptance Criteria | Implementation |
|--------|-------------|---------------------|----------------|
| REQ-ADP9-001 | Ubiquitous | AC-001..004 | `koreanews.go`, `koreanews_test.go` |
| REQ-ADP9-002 | Event-Driven | AC-005..009 | `rss.go::searchRSS`, `rss_test.go` |
| REQ-ADP9-003 | Optional | AC-010..013 | `options.go` env parsing, `options_test.go`, `errors.go::ErrEmptyRSSFeedList` |
| REQ-ADP9-004 | Event-Driven | AC-014..016 | `search.go` (composite dispatch), `search_test.go` |
| REQ-ADP9-005 | Unwanted | AC-017 | `search.go` (input validation) |
| REQ-ADP9-006 | Unwanted/Optional | AC-018..020 | `daum.go::searchDaum`, `daum_test.go`, `errors.go::ErrDaumDisabled` |
| REQ-ADP9-007 | Optional | AC-021..026 | `knc.go::searchKNC`, `knc_test.go`, `errors.go::ErrKNCSidecarDown` |
| REQ-ADP9-008 | Ubiquitous | AC-027 | `locale.go::detectKorean`, `locale_test.go` |
| REQ-ADP9-009 | Optional | AC-028..031 | `dedup.go::dedupDocs`, `dedup_test.go` |
| REQ-ADP9-010 | Ubiquitous | AC-032 | `search.go` (merge + sort), `parse.go` (per-doc Metadata) |
| REQ-ADP9-011 | State-Driven | AC-033 | `concurrent_test.go::TestSearchConcurrentSafe` |
| REQ-ADP9-012 | Event-Driven | AC-034 | `knc.go` (HTTP client), `rss.go` (gofeed UA — set via custom transport) |
| REQ-ADP9-013 | Optional | AC-035 | `options.go` env loader |
| NFR-ADP9-001 | Performance | AC-N01 | `bench_test.go::BenchmarkParseRSSFeed10Items` |
| NFR-ADP9-002 | Race-clean | AC-N02 | `concurrent_test.go::TestSearchConcurrentSafe` |
| NFR-ADP9-003 | Resource | AC-N03 | `bench_test.go::TestMain` goleak, `rss_test.go` cancellation tests |
| NFR-ADP9-004 | Timeout | AC-N04 | `rss_test.go::TestRSSPerFeedTimeoutBound` |

---

## 5. Definition of Done

- [x] All 13 EARS REQs have passing tests.
- [x] All 4 NFRs have passing measurements.
- [x] `go test ./internal/adapters/koreanews/...` exits 0.
- [x] `go test -race ./internal/adapters/koreanews/...` exits 0.
- [x] `go test -cover` reports ≥ 85%.
- [x] `go vet` and `golangci-lint run` clean.
- [x] `BenchmarkParseRSSFeed10Items` median ≤ 5ms; allocs/op ≤ 500.
- [x] RSS tested against ≥ 3 feed formats (RSS 2.0, Atom 1.0, JSON Feed 1.1).
- [x] Per-feed isolation (one feed's failure does not cancel siblings).
- [x] Daum stub returns ErrDaumDisabled regardless of flag.
- [x] KNC sidecar HTTP client tested against 503 default + 200 with
      JSON + 4xx + 5xx + ctx cancel.
- [x] Hangul ratio threshold tested at boundary.
- [x] Intra-adapter dedup tested with URL canonicalization + hash
      fallback.
- [x] MX tags applied.
- [x] Capabilities.Notes documents enabled sub-sources, feed count,
      ToS posture, sidecar status.
- [x] `var _ types.Adapter = (*Adapter)(nil)` present.
- [x] `go.mod` updated with `github.com/mmcdole/gofeed v1.3.0` +
      transitive deps.
- [x] `services/koreanews/` scaffold present (Dockerfile +
      pyproject.toml + stub `app.py` returning 503).
- [x] SPEC status updated to `implemented` (2026-05-04).

---

*End of SPEC-ADP-009 acceptance.md (post-hoc, v1.0)*
