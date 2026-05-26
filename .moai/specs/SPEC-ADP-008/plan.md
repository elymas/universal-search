# SPEC-ADP-008 Implementation Plan (Post-Hoc)

**SPEC**: SPEC-ADP-008 — Naver Suite Adapter
**Status**: implemented (2026-05-07)
**Methodology**: TDD (RED → GREEN → REFACTOR)
**Coverage Target**: 85%
**Owner**: expert-backend
**Priority**: P0

---

## 1. Overview

ADP-008 is the Korean-locale PRIMARY adapter and the FIRST multi-vertical
adapter in MoAI Universal Search. A single `*Adapter` routes
internally to one of five verticals (blog / news / web / shop /
datalab) based on `Query.Filters[Key="naver_vertical"]`. Default
(filter absent) → `blog` (richest Korean user-generated content;
lowest cross-source overlap with SearXNG/daum/RSS_korean).

Key architectural deltas:

1. **First adapter with multi-endpoint dispatch** — single Search
   method dispatches to 5 distinct upstream endpoints
   (`https://openapi.naver.com/v1/search/{blog,news,webkr,shop}.json`
   + `https://openapi.naver.com/v1/datalab/search`).

2. **API-key authentication** — `RequiresAuth=true`,
   `AuthEnvVars=["NAVER_CLIENT_ID","NAVER_CLIENT_SECRET"]`. Both
   headers (`X-Naver-Client-Id`, `X-Naver-Client-Secret`) set on
   every request. Constructor validates env presence unless
   `Options.ClientID` injected (test path).

3. **HTML `<b>` highlight + entity strip** — Naver wraps matched
   keywords in `<b>...</b>` and user-generated content carries HTML
   entities (`&amp;`, `&quot;`, `&lt;`, `&gt;`, `&#39;`). The
   `stripHTML` helper (duplicated from SPEC-ADP-002) cleans
   `title`, `description`, `bloggername` before populating
   `Title`, `Body`, `Snippet`. NOT a security boundary.

4. **DataLab opt-in POST** — different endpoint, different request
   shape (`q.Text` JSON-encoded payload), different response shape
   (time-series instead of items). Returns one synthetic
   NormalizedDoc per `keywordGroups` row.

5. **`Score=0.5` constant** — Naver search response has no
   engagement metric; SPEC-IDX-001 RRF re-ranks by rank not score.

6. **Path B over Path A** — Direct REST in pure Go stdlib (not
   wrapping `isnow890/naver-search-mcp` MCP subprocess) because (a)
   single-binary deploy goal; (b) MCP is itself a thin REST wrapper
   over the same endpoints; (c) preserves the M3 reference pattern.

---

## 2. Architecture

### 2.1 Package Layout

```
internal/adapters/naver/
├── naver.go           — Adapter, Options, New (validates env), Name, Capabilities, Healthcheck
├── naver_test.go      — interface conformance + Capabilities + Healthcheck + auth validation
├── search.go          — (*Adapter).Search hot path + vertical dispatch + URL construction
├── search_test.go     — Per-vertical happy path + error categorisation + filter + ctx + concurrent
├── client.go          — *http.Client, doRequest (4 headers), categorizeStatus, redirectAllowlist
├── client_test.go     — categorizeStatus table + parseRetryAfter + 4-header presence
├── parse.go           — parseResponse + per-vertical parsers (parseBlogItem, parseNewsItem, parseWebItem, parseShopItem)
├── parse_test.go      — per-vertical field mapping + HTML strip + cursor + Hash empty
├── datalab.go         — searchDataLab POST path
├── datalab_test.go    — DataLab happy path + malformed q.Text rejection + empty result
├── strip.go           — stripHTML helper (verbatim from ADP-002)
├── strip_test.go      — table over 8+ inputs
├── score.go           — defaultScore() returns 0.5
├── score_test.go      — trivial deterministic test
├── errors.go          — 4 sentinels (ErrInvalidQuery, ErrInvalidVertical, ErrInvalidCursor, ErrAuthMissing) + parseRetryAfter
├── errors_test.go     — sentinel + parseRetryAfter table
├── bench_test.go      — BenchmarkParseBlogResponse25Items + TestMain goleak
└── testdata/          — 11 JSON fixtures (per-vertical + html entities + pagination + datalab)
```

### 2.2 Key Data Structures

**`Adapter` struct** (`naver.go`): `httpClient`, `baseURL`,
`userAgent`, `healthcheckTarget`, `clientID`, `clientSecret`
(pulled from env at construction; constructor returns
`ErrAuthMissing` when both env and Options are empty).

**`Options` struct**: `BaseURL`, `HTTPClient`, `UserAgentVersion`,
`HealthcheckTarget`, `ClientID`, `ClientSecret`, `SkipAuthCheck`
(test-only).

**Per-vertical response struct types** (`parse.go`):
- Blog/News/Web all share `{lastBuildDate, total, start, display,
  items[]}` envelope with item-shape differences.
- Shop: same envelope + extra item fields (lprice, hprice, mallName,
  productId, category1-4, image).
- DataLab: distinct envelope `{startDate, endDate, timeUnit,
  results: [{title, keywords[], data: [{period, ratio}]}]}`.

**Sentinels** (`errors.go`):
- `ErrInvalidQuery` — empty/whitespace.
- `ErrInvalidVertical` — `naver_vertical` filter has unknown value.
- `ErrInvalidCursor` — cursor out of `[1, 1000]` range.
- `ErrAuthMissing` — `New` env validation failure.

### 2.3 Hot-Path Flow (REQ-ADP8-002 ff)

1. Validate `q.Text`.
2. Resolve vertical from `Filters[Key="naver_vertical"]`; default
   `blog`. Unknown → `ErrInvalidVertical`.
3. Parse `q.Cursor` as positive int in `[1, 1000]`; reject otherwise.
4. Dispatch:
   - `blog`/`news`/`web`/`shop` → `searchVertical(ctx, vertical, q)`:
     build URL with `query`, `display` (clamped 1-100), `start` (1-1000),
     `sort` (default `sim`, opt-in `date`); `doRequest` sets four
     headers; route by HTTP status; call per-vertical parser.
   - `datalab` → `searchDataLab(ctx, q)`: parses `q.Text` as JSON,
     POSTs to `/v1/datalab/search`, returns synthetic NormalizedDocs.
5. Per-vertical parser applies `stripHTML` to `title`,
   `description`, `bloggername`; sets `SourceID="naver"`,
   `Lang="ko"`, `Score=0.5`, `DocType` per vertical (blog →
   DocTypePost; news → DocTypeArticle; web → DocTypeArticle;
   shop → DocTypeOther).
6. Surface `Metadata["next_cursor"] = strconv.Itoa(currentStart +
   requestedDisplay)` on last doc when more pages available.

### 2.4 Per-Vertical Field Mapping (REQ-ADP8-005)

See spec.md §6.3. Highlights:
- **blog**: bloggername → `Author`; postdate (YYYYMMDD) →
  `PublishedAt`; `DocType=DocTypePost`.
- **news**: originallink (when non-empty) preferred as URL over
  `link`; pubDate (RFC 1123) → `PublishedAt`;
  `DocType=DocTypeArticle`.
- **web**: bare title/link/description; `Author=""`;
  `PublishedAt=zero`; `DocType=DocTypeArticle`.
- **shop**: lprice/hprice/mallName/productId/category1-4 + image
  all in Metadata; `DocType=DocTypeOther` (no `DocTypeProduct`
  enum value yet — Open Question §11.6).

Common across all verticals: `Metadata["naver_vertical"]` for
downstream visibility (does NOT escape to Prometheus).

### 2.5 Authentication Flow (REQ-ADP8-003)

- Constructor validates: when `Options.ClientID == "" &&
  Options.ClientSecret == ""`, reads `os.Getenv("NAVER_CLIENT_ID")`
  + `os.Getenv("NAVER_CLIENT_SECRET")`. If both still empty →
  `ErrAuthMissing`.
- Headers on every outbound request: `User-Agent`, `Accept:
  application/json`, `X-Naver-Client-Id`, `X-Naver-Client-Secret`.
- Registry validates `AuthEnvVars` at `RegisterWithOptions` time
  per `internal/adapters/registry.go:122-129`; tests use
  `RegisterOptions{SkipAuthCheck: true}`.

### 2.6 Integration Points

- **Consumed by**: `internal/adapters/registry.go::wrappedAdapter`.
- **Consumes**: `pkg/types`, `internal/obs/reqid.NewTransport`.
- **Downstream**: SPEC-IR-001 (`korean` category routing per
  `acceptance.md:177`); SPEC-IDX-003 (Korean tokenization consumes
  stripped Body).

### 2.7 Redirect Allowlist

`{openapi.naver.com}` with 3-hop cap.

---

## 3. Test Coverage Notes

- Coverage meets 85% target.
- Per-vertical happy paths against 4 separate fixtures
  (`search_response_blog.json`, `search_response_news.json`,
  `search_response_web.json`, `search_response_shop.json`).
- HTML strip table over 8+ inputs (5 entities + `<b>` highlight +
  nested + malformed + very long).
- DataLab POST path with 3 keyword groups × 30 daily ratios fixture.
- Token-leak prevention: client secret never appears in error
  messages or slog records.
- `BenchmarkParseBlogResponse25Items` median ≤ 5ms; allocs/op ≤ 500.

---

## 4. Technical Decisions (Locked During Implementation)

| Decision | Rationale |
|----------|-----------|
| Path B (direct REST) over Path A (MCP wrap) | Single-binary deploy goal; MCP is itself a thin wrapper; preserves M3 reference pattern. |
| Single Adapter, multi-vertical dispatch | One `naver` Prometheus label; cardinality discipline. Per-vertical visibility lives in Metadata. |
| Default vertical = `blog` | Richest Korean UGC; lowest cross-source overlap. |
| `Score=0.5` constant | Naver search has no engagement metric; RRF re-ranks by rank. |
| Shop → `DocTypeOther` (not `DocTypeProduct`) | Adding enum value breaks `pkg/types` SDK boundary semver per `structure.md:160-161`. Defer to SPEC-CORE-001a. |
| Hardcoded `sort=sim` (opt-in `sort=date`) | Default to relevance; date sort is a power-user feature. |
| stripHTML duplicated from ADP-002 | Rule of three barely reached; refactor SPEC after M3. |
| DataLab opt-in | Different request/response shape; surfaces opt-in via filter; demonstrates the framework hosts non-search-shaped sources. |

---

## 5. Risks Mitigated

- **Quota exhaustion (25k/day per app)** → 429 → `CategoryRateLimited`
  surfaced with `RetryAfter`; fanout owns retry decisions.
- **Auth missing at runtime** → registry validates `AuthEnvVars` at
  startup; constructor validates `Options.ClientID`.
- **HTML markup in body breaks synthesis** → `stripHTML` cleans
  before populating `Body`; tested over 8+ inputs.
- **`originallink` empty on some news items** → fallback to `link`;
  fixture covers (`search_response_news_no_originallink.json`).
- **Cursor parse on adversarial input** → `strconv.Atoi` +
  explicit range guard `[1, 1000]`.

---

## 6. Out-of-Scope Reminders (from spec.md §7)

- MCP-server subprocess wrapping → REJECTED per D1.
- Cross-vertical aggregation (single Search returning multiple
  verticals mixed) → future SPEC-ADP-008a.
- `DocTypeProduct` enum addition → SPEC-CORE-001a.
- DataLab Shopping Insight tools → out of v0.1.
- Tenant-scoped quota policy → SPEC-AUTH-002 (M6).
- OAuth-authenticated variant → out of scope.
- Korean tokenization → SPEC-IDX-003.

---

*End of SPEC-ADP-008 plan.md (post-hoc, v1.0)*
