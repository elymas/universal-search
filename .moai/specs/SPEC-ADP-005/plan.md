# SPEC-ADP-005 Implementation Plan (Post-Hoc)

**SPEC**: SPEC-ADP-005 — YouTube Adapter
**Status**: implemented (2026-05-07; 91.2% coverage)
**Methodology**: TDD (RED → GREEN → REFACTOR)
**Coverage Target**: 85% (achieved 91.2%)
**Owner**: expert-backend
**Priority**: P0

---

## 1. Overview

ADP-005 is the FIRST adapter targeting **video** content. It introduces
TWO architectural deltas versus previous adapters:

1. **Python sidecar at `services/youtube-extract/`** wrapping
   `yt-dlp` via FastAPI. The Go adapter is an HTTP client to the
   sidecar; yt-dlp subprocess invocation is fully contained in the
   Python service. Path B (YouTube Data API v3) was rejected: free
   tier allows only 100 searches/day at 100 quota units per
   `search.list`, and third-party transcript retrieval requires
   OAuth + video ownership.
2. **Tanh-of-log10 score formula** distinct from Reddit/HN's
   Tanh-of-(score/100). YouTube view counts span [0, ~10^10]; the
   log10 transform spreads the range across [0, 10] which Tanh then
   squishes into [0.5, 1.0] with meaningful gradient at every decade.

Score formula:
```
Score = clamp(0.5 + 0.5 * tanh(log10(viewCount + 1) / 5.0), 0.0, 1.0)
```

Korean-locale auto-detection (≥30% Hangul rune ratio) auto-sets
`transcript_lang="ko"` when no explicit `lang` filter is provided.

---

## 2. Architecture

### 2.1 Package Layout

```
internal/adapters/youtube/
├── youtube.go        — Adapter, Options, New, Name, Capabilities, Healthcheck (HTTP GET /health)
├── youtube_test.go   — interface conformance + Healthcheck failure modes
├── search.go         — (*Adapter).Search hot path + JSON request body construction
├── search_test.go    — E2E + happy path + error categorisation + filter + ctx + concurrent tests
├── client.go         — *http.Client construction (30s timeout, reqid Transport, NO redirect allowlist)
├── client_test.go    — categorizeStatus table + parseRetryAfter (30s default)
├── parse.go          — parseSearchResponse transform (sidecar JSON envelope)
├── parse_test.go     — field mapping + transcript snippet truncation + cursor + skip-on-item-error
├── lang.go           — detectKoreanQuery (≥30% Hangul) + selectTranscriptLang
├── lang_test.go      — Korean detection threshold + lang priority tests
├── score.go          — normalizeViewScore (Tanh-of-log10 formula)
├── score_test.go     — score table over 7 view-count values
├── errors.go         — ErrInvalidQuery + ErrInvalidCursor + ErrCursorOverCap + parseRetryAfter (30s default, 60s cap)
├── bench_test.go     — BenchmarkParseSearchResponse25Videos + TestMain goleak
└── testdata/         — 6 JSON fixtures (happy path, empty, pagination, korean, no_transcript, malformed)

services/youtube-extract/                      (Python sidecar — separate from this SPEC's Go scope)
├── Dockerfile + pyproject.toml + .env.example
└── src/youtube_extract/{app.py, ytdlp_runner.py, models.py}
```

### 2.2 Key Data Structures

**`Adapter` struct** (`youtube.go`): `httpClient *http.Client`,
`baseURL string` (sidecar URL, e.g. `http://localhost:8082`),
`userAgent string`, `healthcheckPath string` (default `/health`).
Immutable post-construction.

**`Options` struct**: `BaseURL`, `HTTPClient`, `UserAgentVersion`.
HTTP timeout is 30s (longer than ADP-001/002's 10s because yt-dlp
calls can legitimately take 5-15s with transcripts).

**Sidecar request body** (JSON): `{query, max_results, cursor_offset,
transcript_lang, include_transcripts: true, since: optional}`.

**Sidecar response envelope**: `{items: [<YTItem>...], has_more}` on
success; `{error: {category, message}}` on failure.

**Sentinels** (`errors.go`): `ErrInvalidQuery`, `ErrInvalidCursor`,
`ErrCursorOverCap` (message: `"youtube: max_results + cursor offset
> 100"`).

**Constants** (`score.go`): `log10Divisor = 5.0`, `scoreCenter = 0.5`
with `@MX:NOTE`.

### 2.3 Hot-Path Flow (REQ-ADP5-002)

1. Ctx pre-check (REQ-ADP5-009 precedence): if ctx already cancelled
   → return `*SourceError{Unavailable, Cause: ctx.Err()}`.
2. Validate `q.Text` (REQ-ADP5-008).
3. Parse `q.Cursor` as non-negative int via `strconv.Atoi`; reject
   negatives, floats, malformed.
4. Enforce cap: `clamp(q.MaxResults, 1, 100) + cursorOffset > 100` →
   `ErrCursorOverCap` (cap is INCLUSIVE of 100).
5. Apply `selectTranscriptLang(q.Text, q.Filters)` — priority:
   explicit filter > Korean auto-detect (≥30% Hangul) > "en".
6. Construct JSON request body and POST to `<baseURL>/search`.
7. Route by HTTP status: 200 → `parseSearchResponse()`; 429 → parse
   `Retry-After` (30s default vs Reddit/HN's 5s; 60s cap); 4xx →
   `CategoryPermanent`; 5xx + connection-refused →
   `CategoryUnavailable`; 503 with sidecar error envelope preserves
   the `reason` field via `errors.Unwrap` chain.
8. `parseSearchResponse()` skips items with per-item `error` field
   silently (sole-emitter discipline); transcript snippet truncated
   to ≤500 runes; `next_cursor = strconv.Itoa(currentOffset +
   len(items))` surfaced on last doc when `has_more=true` AND cap
   not hit.

### 2.4 Score Synthesis (REQ-ADP5-005 + §2.3)

Pure function `normalizeViewScore(int64) float64`:

| view_count       | log10(v+1) | tanh(.../5) | Score   |
|------------------|------------|-------------|---------|
| 0                | 0.000      | 0.000       | 0.5000  |
| 1                | 0.301      | 0.060       | 0.5301  |
| 100              | 2.004      | 0.380       | 0.6901  |
| 10,000           | 4.000      | 0.664       | 0.8319  |
| 1,000,000        | 6.000      | 0.834       | 0.9170  |
| 100,000,000      | 8.000      | 0.927       | 0.9637  |
| 10,000,000,000   | 10.000     | 0.964       | 0.9820  |

Null/missing view_count (livestream-archived edge case) → treated
as 0 → Score=0.5.

### 2.5 Korean-Locale Detection (REQ-ADP5-007)

`detectKoreanQuery(text)` counts runes in U+AC00..U+D7AF Hangul
block; returns true when ≥30% of total runes are Hangul.
`selectTranscriptLang(text, filters)` applies priority:
1. Explicit `Filters[Key="lang"]` non-empty 2-8 chars → use as-is.
2. Korean detection → `"ko"`.
3. Default → `"en"`.

### 2.6 Integration Points

- **Consumed by**: `internal/adapters/registry.go` (sole-emitter).
- **Consumes**: `pkg/types`, `internal/obs/reqid.NewTransport`.
- **External dep**: `services/youtube-extract/` Python sidecar (out
  of this SPEC's Go-side scope; contractually documented in spec.md
  §6.4).
- **Downstream**: SPEC-FAN-001, SPEC-IDX-001, SPEC-SYN-001 (transcript
  snippet consumed for citation assembly).

### 2.7 No Redirect Allowlist

Unlike ADP-001/002/003/004, the YouTube adapter does NOT enforce a
redirect allowlist. The sidecar URL is operator-configured and
trusted; redirect attacks come from external sources, not from the
operator's own sidecar. Stdlib default `http.Client.CheckRedirect`
behaviour applies.

---

## 3. Test Coverage Notes

- **Coverage**: 91.2% (exceeds 85% target).
- **All SPEC acceptance criteria met**.
- **golangci-lint clean**, race detector clean.
- **Score formula corrected per exact Go math** — spec.md §2.3 table
  had rounding in the tanh column; the formula `Score = clamp(0.5 +
  0.5*tanh(log10(v+1)/5.0))` is authoritative (per HISTORY
  2026-05-07).
- `BenchmarkParseSearchResponse25Videos` median ≤ 10ms (higher than
  ADP-001/002's 5ms target due to richer YouTube metadata —
  transcript snippet up to 500 runes, `available_transcript_langs`
  []string per item, etc.); `allocs/op ≤ 800` (≤32/video, higher
  than 20/doc floor due to transcript copy).
- `TestSearchConcurrentSafe` (50 goroutines) anchors REQ-ADP5-010
  AND NFR-ADP5-004 race-cleanness.

---

## 4. Technical Decisions (Locked During Implementation)

| Decision | Rationale |
|----------|-----------|
| Python sidecar wrapping yt-dlp via FastAPI | YouTube Data API v3 free tier allows 100 searches/day; transcripts require OAuth + ownership. yt-dlp wraps internal player API + caption endpoints with full functionality. Subprocess isolation in sidecar prevents GPL contagion. |
| 30s adapter HTTP timeout (vs 10s in ADP-001/002) | yt-dlp's typical full extraction with transcript takes 5-15s; tighter timeout would cause spurious failures. |
| 30s default Retry-After (vs 5s in Reddit/HN) | YouTube blocks tend to last longer per https://github.com/yt-dlp/yt-dlp/issues/10128. |
| `MaxResults + Cursor offset > 100` cap | yt-dlp's `ytsearchN:` becomes inefficient beyond N=100. |
| No redirect allowlist | Sidecar URL is operator-trusted; no external redirect surface. |
| Sole-emitter discipline preserved even for per-item parse errors | Items with per-item `error` field are skipped silently; operator-visible signal is the delta between request `max_results` and returned `len(docs)` surfaced by the wrappedAdapter's `result_count`. |
| Korean auto-detect threshold = 30% | Empirical; handles operator-curated mixed-locale queries. Future SPEC-IDX-003 may upgrade to real lang-detect. |

---

## 5. Risks Mitigated

- **YouTube IP-block challenge** → sidecar defaults
  `--sleep-requests 1.0 --sleep-interval 2 --max-sleep-interval 5`;
  optional cookie file; 30s Retry-After default.
- **Sidecar process unreachable** → 503/connection-refused →
  `CategoryUnavailable`; fanout proceeds with other adapters.
- **Subprocess zombie on caller cancel** → sidecar's responsibility
  (Python tests); Go side guarantees no goroutine leak via
  `goleak.VerifyNone` + `defer resp.Body.Close()`.
- **Transcript snippet >500 runes** → defensive truncation in
  parser; tested via `TestParseSearchResponseTruncatesOverlongTranscript`.
- **Race conditions** → 50-goroutine `TestSearchConcurrentSafe`
  under `-race`.

---

## 6. Out-of-Scope Reminders (from spec.md §7)

- YouTube Data API v3 OAuth integration → deferred to future
  SPEC-ADP-005a.
- Full transcript inclusion in /search response → snippet only in
  v0.1; full transcript via separate `/transcript` sidecar
  endpoint (Go-side binding deferred to SPEC-SYN-001).
- Music vs lecture vs short-form classification → SPEC-IR-001.
- Korean tokenization → SPEC-IDX-003.
- Live network integration tests in CI.
- Sidecar implementation details → tracked under Open Question §11.7.

---

*End of SPEC-ADP-005 plan.md (post-hoc, v1.0)*
