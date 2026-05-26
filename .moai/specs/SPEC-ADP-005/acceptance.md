# SPEC-ADP-005 Acceptance Criteria (Post-Hoc)

**SPEC**: SPEC-ADP-005 — YouTube Adapter
**Status**: implemented (2026-05-07; 91.2% coverage)
**Format**: Given/When/Then per REQ + edge cases + Definition of Done

---

## 1. Acceptance Scenarios by REQ

### REQ-ADP5-001 — Adapter Interface + Healthcheck

**AC-001: Compile-time interface assertion**
- `var _ types.Adapter = (*Adapter)(nil)` present; build succeeds.

**AC-002: Name returns "youtube"**

**AC-003: Capabilities deterministic + shape-correct**
- Two calls return `reflect.DeepEqual`; `SourceID="youtube"`,
  `DisplayName="YouTube"`, `DocTypes=[DocTypeVideo]`,
  `SupportedLangs=nil`, `SupportsSince=true`, `RequiresAuth=false`,
  `RateLimitPerMin=30`, `DefaultMaxResults=25`, Notes contains all
  5 documented substrings.

**AC-004: Healthcheck success**
- Stub `/health` returns 200 with `{"status":"ok","ytdlp_version":
  "..."}` → nil error.

**AC-005: Healthcheck failure modes (4 tests)**
- 503 status → non-nil error.
- Malformed JSON body → non-nil error.
- `{"status":"degraded"}` → non-nil error.
- Sidecar unreachable → non-nil error.

### REQ-ADP5-002 — Search Happy Path

**AC-006: Happy path 25 videos**
- Stub returns `testdata/search_response.json` → 25 NormalizedDocs;
  each `Validate()` returns nil.

**AC-007: Request body fields always present**
- Decoded request body contains `query`, `max_results`,
  `cursor_offset` (always present, zero is no-cursor signal),
  `transcript_lang`, `include_transcripts=true`.

**AC-008: max_results clamp / default**
- `MaxResults=500` → `max_results=100`.
- `MaxResults=0` → `max_results=25`.

**AC-009: cursor_offset round-trip**
- `Cursor=""` → `cursor_offset=0` (field present and zero).
- `Cursor="25"` → `cursor_offset=25`.

**AC-010: Content-Type: application/json on POST**

### REQ-ADP5-003 — HTTP 429 Rate-Limit (30s default vs 5s)

**AC-011: Integer Retry-After → 30s**
**AC-012: HTTP-date Retry-After → window (25s, 35s)**
**AC-013: No header defaults to 30s** (distinct from Reddit/HN's 5s)
**AC-014: Cap at 60s** (`Retry-After: 999` → 60s)
**AC-015: No internal retry** (1 request observed)

### REQ-ADP5-004 — HTTP 4xx / 5xx / Sidecar Failure

**AC-016: 4xx → Permanent**
- Table over 401/403/404 → `errors.Is(err, types.ErrPermanent)`.

**AC-017: 5xx → Unavailable**
- Table over 500/503/504.

**AC-018: Sidecar unreachable → Unavailable + HTTPStatus=0**

**AC-019: Sidecar yt-dlp signed-in challenge preserved**
- Stub returns 503 with `{"error":{"category":"unavailable",
  "reason":"yt-dlp signed-in challenge"}}` →
  `errors.Is(err, types.ErrSourceUnavailable)` AND
  `errors.Unwrap(srcErr).Error()` contains `"yt-dlp signed-in
  challenge"`.

**AC-020: Underlying error preserved**

### REQ-ADP5-005 — NormalizedDoc Field Mapping

**AC-021: Field mapping table (5 fixtures)**
- Link video with transcript, video without transcript,
  deleted-channel video, livestream-archived (null view_count),
  Korean video → every field per §6.3.

**AC-022: Korean video → Lang="ko"**

**AC-023: Transcript snippet ≤500 runes**
- `Metadata["transcript_snippet"]` length ≤ 500 runes.

**AC-024: Adapter truncates overlong transcript defensively**
- Fixture sends 1000-rune sidecar value → adapter truncates to 500.

**AC-025: Pagination cursor on last doc**
- `has_more=true, offset=0, 25 items` → last doc
  `Metadata["next_cursor"] == "25"`.

**AC-026: No cursor on last page**
- `has_more=false` → no `next_cursor`.

**AC-027: Hash always empty**

**AC-028: All 6 required Metadata keys present**
- `{channel_id, channel_url, duration_seconds, view_count,
  thumbnail_url, available_transcript_langs}`.

**AC-029: Items with per-item error skipped silently**
- Fixture with mixed successes + errors → only successes returned;
  NO log record emitted by the `youtube` package (verified by slog
  handler that fails the test on any record from package "youtube").

**AC-030: Livestream null view_count → Score=0.5**

**AC-031: Malformed JSON → Permanent**

### REQ-ADP5-006 — User-Agent / Accept / Content-Type Headers

**AC-032: Custom UA**
**AC-033: `Accept: application/json`**
**AC-034: `Content-Type: application/json` on POST**
**AC-035: UA version configurable**

### REQ-ADP5-007 — Lang and Since Filters with Korean Auto-Detect (Optional)

**AC-036: Explicit lang filter wins**
- `Filters=[{lang, "ja"}]` + Korean text → request
  `transcript_lang="ja"`; doc `Lang="ja"`.

**AC-037: Korean auto-detect (≥30% Hangul)**
- No filter, Korean text → `transcript_lang="ko"`.

**AC-038: English default for non-Korean text**

**AC-039: Threshold boundary 29% → English; 31% → Korean**

**AC-040: since filter added**
- `Filters=[{since, "1700000000"}]` → request `since=1700000000`.

**AC-041: Malformed / negative since dropped**

**AC-042: Unknown filter ignored**
- `Filters=[{nsfw,"true"}]` → no `nsfw` field; no error.

**AC-043: Empty lang value drops to default**
- `Filters=[{lang,""}]` → `transcript_lang="en"` (Korean detection
  runs; English default applies for non-Korean text).

**AC-044: Invalid lang format rejected**
- `Filters=[{lang,"verylongstring"}]` (length > 8) →
  `transcript_lang="en"`; adapter does not crash.

**AC-045: lang priority table tests (4 cases)** in `lang_test.go`

### REQ-ADP5-008 — Empty / Invalid Cursor / Cursor-over-Cap (Unwanted)

**AC-046: Empty/whitespace Text rejected, zero HTTP**

**AC-047: Invalid cursor rejected, zero HTTP**
- `["abc", "-1", "1.5", "1e3", " 25"]` → ErrInvalidCursor.

**AC-048: Cursor-over-cap rejected (>100 strict)**
- `MaxResults=50, Cursor="60"` → 110 > 100 → `ErrCursorOverCap`.
- `MaxResults=25, Cursor="75"` → 100 == 100 → request issued (cap
  is INCLUSIVE).
- `MaxResults=0 (defaults to 25), Cursor="76"` → 101 → ErrCursorOverCap.

### REQ-ADP5-009 — Context Cancellation Discipline

**AC-049: Ctx cancelled mid-flight**
- Stub delays 200ms; cancel ctx at 50ms → `errors.Is(err,
  types.ErrSourceUnavailable)` AND `errors.Is(err, context.Canceled)`.

**AC-050: Ctx already cancelled at entry → zero HTTP requests**
- `errors.Is(err, context.Canceled)` AND stub counter = 0.
- Precedence: REQ-ADP5-009 wins over REQ-ADP5-008 — empty query +
  pre-cancelled ctx returns `context.Canceled`, NOT
  `ErrInvalidQuery`.

**AC-051: Ctx deadline exceeded**
- 50ms deadline; stub delays 200ms → `errors.Is(err,
  context.DeadlineExceeded)`.

### REQ-ADP5-010 — Concurrent Search Safety

**AC-052: 50 goroutines race-clean**
- 50 goroutines × 1 Search; `-race` clean; 50 requests; 25 valid
  docs each.

---

## 2. NFR Acceptance

### NFR-ADP5-001 — Parse-Path Performance

**AC-N01: Benchmark within target**
- `BenchmarkParseSearchResponse25Videos` median ≤ 10 ms (higher
  than ADP-001/002 due to richer per-item Metadata);
  `allocs/op ≤ 800` (≤32/video).

### NFR-ADP5-002 — E2E p95 (Stub)

**AC-N02: p95 ≤ 200ms** over 100 invocations.

### NFR-ADP5-003 — No Goroutine Leak on Cancellation

**AC-N03: goleak.VerifyNone after mid-flight cancel**
- `TestMain` invokes `goleak.VerifyTestMain(m)`.

### NFR-ADP5-004 — Race-Clean Concurrent Workload

**AC-N04: `TestSearchConcurrentSafe` under -race**

---

## 3. Edge Cases

**EC-001: Empty items array**
- `parseSearchResponse` returns `(nil, "", nil)`.

**EC-002: Per-item parse error not logged**
- Sole-emitter discipline preserved; operator sees the result-count
  delta via wrappedAdapter's `result_count` attribute.

**EC-003: Transcript present but in different lang than requested**
- `available_transcript_langs` always populated in Metadata;
  consumer can re-request.

**EC-004: Score saturation at top-of-charts**
- Despacito (~8.5B views) maps to ≈0.98, not 1.0; leaves headroom.

**EC-005: NBSP not counted as whitespace**
- Same Go stdlib `unicode.IsSpace` semantics as ADP-001/002.

**EC-006: Cap is INCLUSIVE of 100**
- `MaxResults + Cursor offset == 100` succeeds; > 100 rejects.

---

## 4. Coverage Matrix

| REQ-ID | EARS Pattern | Acceptance Criteria | Implementation |
|--------|-------------|---------------------|----------------|
| REQ-ADP5-001 | Ubiquitous | AC-001..005 | `youtube.go`, `youtube_test.go` |
| REQ-ADP5-002 | Event-Driven | AC-006..010 | `search.go`, `search_test.go` |
| REQ-ADP5-003 | Event-Driven | AC-011..015 | `client.go::categorizeStatus`, `errors.go::parseRetryAfter` (30s default) |
| REQ-ADP5-004 | Event-Driven | AC-016..020 | `client.go::categorizeStatus`, sidecar error envelope parsing |
| REQ-ADP5-005 | Ubiquitous | AC-021..031 | `parse.go::parseSearchResponse`, `score.go::normalizeViewScore` |
| REQ-ADP5-006 | Ubiquitous | AC-032..035 | `client.go::doRequest` |
| REQ-ADP5-007 | Optional | AC-036..045 | `lang.go::detectKoreanQuery` + `selectTranscriptLang` |
| REQ-ADP5-008 | Unwanted | AC-046..048 | `search.go` (input validation), `errors.go::ErrCursorOverCap` |
| REQ-ADP5-009 | Event-Driven | AC-049..051 | `search.go` (ctx pre-check), stdlib ctx propagation |
| REQ-ADP5-010 | State-Driven | AC-052 | `search_test.go::TestSearchConcurrentSafe` |
| NFR-ADP5-001 | Performance | AC-N01 | `bench_test.go::BenchmarkParseSearchResponse25Videos` |
| NFR-ADP5-002 | Latency | AC-N02 | `search_test.go::TestSearchE2ELatencyStubP95` |
| NFR-ADP5-003 | Resource | AC-N03 | `search_test.go::TestSearchNoGoroutineLeakOnCancel`, `TestMain` goleak |
| NFR-ADP5-004 | Race-clean | AC-N04 | `search_test.go::TestSearchConcurrentSafe` |

---

## 5. Definition of Done

- [x] All 10 EARS REQs have passing tests.
- [x] All 4 NFRs have passing measurements.
- [x] `go test ./internal/adapters/youtube/...` exits 0.
- [x] `go test -race ./internal/adapters/youtube/...` exits 0.
- [x] `go test -cover` reports 91.2% (exceeds 85% target).
- [x] `go vet` and `golangci-lint run` clean.
- [x] `BenchmarkParseSearchResponse25Videos` median ≤ 10ms;
      allocs/op ≤ 800.
- [x] Korean auto-detection threshold (30%) tested at boundary.
- [x] Sole-emitter discipline verified — no slog records from
      "youtube" package even on per-item skip.
- [x] MX tags applied per spec.md §6.7 plan.
- [x] Capabilities.Notes contains all 5 documented substrings.
- [x] `var _ types.Adapter = (*Adapter)(nil)` present.
- [x] Sidecar contractually documented in spec.md §6.4; sidecar
      implementation tracked under Open Question §11.7.
- [x] SPEC status updated to `implemented` (2026-05-07).

---

*End of SPEC-ADP-005 acceptance.md (post-hoc, v1.0)*
