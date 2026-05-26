# SPEC-ADP-002 Implementation Plan (Post-Hoc)

**SPEC**: SPEC-ADP-002 — Hacker News Adapter
**Status**: implemented (2026-04-28)
**Methodology**: TDD (RED → GREEN → REFACTOR)
**Coverage Target**: 85% (per `.moai/config/sections/quality.yaml`)
**Owner**: expert-backend
**Priority**: P0

---

## 1. Overview

ADP-002 is the SECOND real adapter in MoAI Universal Search, consuming
SPEC-CORE-001's `pkg/types.Adapter` contract via Algolia HN Search at
`https://hn.algolia.com/api/v1/search`. The adapter mirrors the file
layout and discipline established by SPEC-ADP-001 (Reddit) verbatim,
with HN-specific deltas: Algolia `{hits, nbHits, page, nbPages}` JSON
envelope (versus Reddit's `{data: {after, children}}`), self-post
permalink construction (`https://news.ycombinator.com/item?id=<id>`
when `url` is empty), HTML body stripping via a `stripHTML` helper
(HN's `story_text` carries `<p>/<a>/<i>` markup), and a defensive
`_tags` filter (parser skips hits whose `_tags` array does not
include `"story"` even though `tags=story` was requested).

The adapter is one-shot per call: no fanout, no retry, no caching, no
ranking fusion, zero observability emission (sole-emitter discipline
preserved — the registry's `wrappedAdapter` at
`internal/adapters/registry.go:172-263` emits all per-call metrics
and slog records). Score normalization is the SPEC-ADP-001 §2.3
Tanh-of-(points/100) formula duplicated verbatim.

---

## 2. Architecture

### 2.1 Package Layout

```
internal/adapters/hn/
├── hn.go              (138 LoC) — Adapter, Options, New, Name, Capabilities, Healthcheck
├── hn_test.go         (149 LoC) — interface conformance + Capabilities determinism
├── search.go          (203 LoC) — (*Adapter).Search hot path + URL construction + filter expressions
├── search_test.go     (767 LoC) — E2E + happy path + error categorisation + concurrent safety
├── client.go          (119 LoC) — *http.Client, doRequest, categorizeStatus, redirectAllowlist
├── client_test.go     (289 LoC) — categorizeStatus table + redirect allowlist enforcement
├── parse.go           (201 LoC) — parseHits transform (Algolia HN envelope)
├── parse_test.go      (352 LoC) — field-mapping table + defensive _tags filter + pagination cursor
├── strip.go           ( 57 LoC) — stripHTML helper (stdlib-only tag-strip + entity decode)
├── strip_test.go      ( 86 LoC) — table-driven over 8 input shapes
├── score.go           ( 41 LoC) — normalizeScore Tanh formula (verbatim from ADP-001)
├── score_test.go      ( 57 LoC) — score normalization table
├── errors.go          ( 69 LoC) — ErrInvalidQuery + ErrInvalidCursor sentinels + parseRetryAfter helper
├── bench_test.go      ( 31 LoC) — BenchmarkParseHits25Hits + TestMain goleak
└── testdata/          (7 fixtures) — search_response*.json
```

Total: ~2,759 LoC across 14 Go files + 7 JSON fixtures.

### 2.2 Key Data Structures

**`Adapter` struct** (`hn.go:36-45`): immutable post-construction;
holds `httpClient *http.Client`, `baseURL string`, `userAgent string`,
`healthcheckTarget string`. Goroutine-safe by virtue of immutability
plus stdlib `*http.Client` thread-safety.

**`Options` struct** (`hn.go:18-34`): `BaseURL`, `HTTPClient`,
`UserAgentVersion`, `HealthcheckTarget`. All fields have documented
zero-value defaults applied in `New()`.

**`algoliaResponse` / `algoliaHit` struct types** (`parse.go:11-37`):
JSON-tagged structs matching the Algolia HN envelope. The `_tags`
array is decoded as `[]string` for the defensive story-only filter
in `parseHits`.

**Package-level sentinels** (`errors.go`):
`ErrInvalidQuery`, `ErrInvalidCursor`. Both are wrapped in
`*types.SourceError{Category: CategoryPermanent}` by `Search()`.

**Constants** (`score.go`): `tanhDivisor = 100.0`,
`scoreCenter = 0.5`. Annotated with `@MX:NOTE` documenting the
Tanh formula choice and tie-in to SPEC-IDX-001 RRF.

### 2.3 Hot-Path Flow (REQ-ADP2-002)

1. `(*Adapter).Search(ctx, q)` validates `q.Text` (REQ-ADP2-008) —
   rejects empty/whitespace via `unicode.IsSpace` rune scan.
2. Validates `q.Cursor` via `strconv.Atoi` (REQ-ADP2-008) — rejects
   negative integers and non-numeric values.
3. Builds the request URL via `net/url.Values` with `query`,
   `tags=story` (hardcoded), `hitsPerPage` (clamped 1–100, default
   25), `page` (only when cursor present), optional `numericFilters`
   from `Query.Filters[since|min_points]`.
4. Constructs `http.Request` via `http.NewRequestWithContext`.
5. `doRequest()` sets `User-Agent`, `Accept: application/json`
   headers and invokes `httpClient.Do`.
6. Routes by HTTP status: 200 → `parseHits()`; 429 → parses
   `Retry-After` header, wraps in `*SourceError{CategoryRateLimited}`;
   other 4xx → `CategoryPermanent`; 5xx + network errors →
   `CategoryUnavailable`. All via `categorizeStatus()`.
7. `parseHits()` decodes the Algolia envelope, applies the defensive
   `_tags` story filter, transforms each hit per the §6.3 field
   mapping table, applies `stripHTML` to `story_text`, surfaces the
   next-page cursor (`strconv.Itoa(currentPage + 1)`) via
   `Metadata["next_cursor"]` on the LAST returned doc.

### 2.4 Integration Points

- **Consumed by**: `internal/adapters/registry.go` — the
  `wrappedAdapter` wrap pattern at lines 172-263 emits all
  observability (sole-emitter discipline).
- **Consumes**: `pkg/types` (Adapter, Capabilities, Query,
  NormalizedDoc, SourceError, DocType enum),
  `internal/obs/reqid.NewTransport` for request-ID propagation.
- **Downstream**: SPEC-FAN-001 fanout dispatches to
  `registry.Get("hackernews").Search`; SPEC-IDX-001 RRF consumes
  the Tanh-normalised `Score`; SPEC-SYN-001 consumes
  `[]NormalizedDoc` for citation assembly.

### 2.5 HTTP Client Construction

`newDefaultClient()` (`client.go`):
- `Timeout: 10 * time.Second` (caller's ctx deadline takes precedence
  when shorter).
- `CheckRedirect: redirectAllowlist` enforcing
  `{hn.algolia.com, news.ycombinator.com}` with a 3-hop cap.
- `Transport: reqid.NewTransport(http.DefaultTransport)` for
  request-ID propagation.

---

## 3. Test Coverage Notes

- **Coverage**: meets 85% target per `.moai/config/sections/quality.yaml`.
- **Race-clean**: `go test -race ./internal/adapters/hn/...` is part
  of the CI pipeline; `TestSearchConcurrentSafe` (50 goroutines, one
  shared `*Adapter`, one stub server) anchors REQ-ADP2-010.
- **Goroutine leak**: `bench_test.go::TestMain` invokes
  `goleak.VerifyTestMain(m)` (NFR-ADP2-003).
- **Benchmark**: `BenchmarkParseHits25Hits` invoked as
  `go test -bench=BenchmarkParseHits25Hits -benchtime=10x -count=5
  ./internal/adapters/hn/...` (NFR-ADP2-001) — median of 5 ≤ 5ms,
  allocs/op ≤ 500.
- **Stub p95**: `TestSearchE2ELatencyStubP95` (NFR-ADP2-002) — 100
  invocations against `httptest.Server`; `durations[94] ≤ 200ms`.

---

## 4. Technical Decisions (Locked During Implementation)

| Decision | Rationale |
|----------|-----------|
| Duplicate `parseRetryAfter`, `categorizeStatus`, `redirectAllowlist` from Reddit | Rule-of-three not yet reached at ADP-002 time; extraction would couple parallel SPECs to a moving shape. Refactor deferred to SPEC-ADP-REFAC-001 post-M3. |
| Stdlib-only `stripHTML` | HN body markup is shallow (`<p>`, `<a>`, `<i>`, `<br>`, `<code>`, `<pre>` + 5 entities). `golang.org/x/net/html` rejected as premature dep. |
| Hardcode `tags=story` | v0.1 ships stories only; comments/polls deferred to future SPEC-ADP-002a. |
| Defensive `_tags` filter | Algolia is known to have transient discrepancies; the parser skips non-story hits silently rather than trusting the request parameter. |
| Self-post URL = `news.ycombinator.com/item?id=<objectID>` | Algolia returns empty `url` for self-posts; HN canonical permalink is the operational URL consumers expect. |

---

## 5. Risks Mitigated

- **Algolia contract drift** → `encoding/json` tolerates unknown
  fields; fixtures pinned to documented shape.
- **Hash collisions across Reddit/HN** → `CanonicalHash` includes
  `SourceID` prefix per `pkg/types/normalized_doc.go:96-99`.
- **Score formula coupling with SPEC-IDX-001** → Identical formula
  to ADP-001 §2.3; RRF weights rank not raw score.
- **Goroutine leak on ctx cancel** → `defer resp.Body.Close()` +
  `goleak.VerifyNone(t)` after every cancellation test.

---

## 6. Out-of-Scope Reminders (from spec.md §7)

The implementer did NOT add (per HARD exclusions):
- Retry orchestration (SPEC-FAN-001 owns).
- Response caching (SPEC-CACHE-001 owns).
- Cross-adapter helper extraction (deferred to SPEC-ADP-REFAC-001).
- HN comments / polls (deferred to SPEC-ADP-002a).
- `search_by_date` mode (P2 enhancement post-M3).
- Live network integration tests in CI.
- Per-adapter custom Prometheus metrics.

---

*End of SPEC-ADP-002 plan.md (post-hoc, v1.0)*
