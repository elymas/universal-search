# SPEC-ADP-007 Implementation Plan (Post-Hoc)

**SPEC**: SPEC-ADP-007 — SearXNG Bridge Adapter
**Status**: implemented (2026-05-07)
**Methodology**: TDD (RED → GREEN → REFACTOR)
**Coverage Target**: 85%
**Owner**: expert-backend
**Priority**: P0

---

## 1. Overview

ADP-007 is the GENERAL-WEB bridge adapter, talking to a local SearXNG
container deployed via SPEC-BOOT-001 at
`deploy/docker-compose.yml:106-130`. The adapter turns one
`types.Query` into a single
`GET ${USEARCH_SEARXNG_URL}/search?q=...&format=json[&pageno=N]` call
and surfaces results from 70+ aggregated upstream engines (google /
bing / duckduckgo / ...) as `[]NormalizedDoc`.

Key deltas versus other adapters:

1. **Local-only endpoint** — `Options.BaseURL` defaults to the
   `USEARCH_SEARXNG_URL` env var, defaulting to `http://searxng:8080`
   (compose-internal hostname). Plain HTTP inside the compose `app`
   network; no TLS.

2. **Engine-of-origin metadata** — every NormalizedDoc carries
   `Metadata["engine"]` (string, primary), `Metadata["engines"]`
   (`[]string`, all contributing engines), `Metadata["category"]`
   (single SearXNG-side category like `"general"` / `"news"`).
   First-class metadata for downstream RRF (SPEC-IDX-001) and dedup
   (SPEC-FAN-001 §2.4) consumers.

3. **403-with-Retry-After promoted to RateLimited** — SearXNG's
   limiter emits 403 OR 429 depending on version; the rosetta maps
   403 with `Retry-After` to `CategoryRateLimited` (otherwise 403
   falls through to `CategoryPermanent` per inherited rule).

4. **JSON-format precondition** — the deployed SearXNG instance MUST
   have `json` enabled in `search.formats` (settings.yml). The
   adapter hardcodes `format=json` in every URL; the precondition is
   documented in `Capabilities.Notes` and Open Question §11.1.

5. **SHA256-derived stable IDs** — SearXNG response lacks
   stable per-result IDs; the parser derives `NormalizedDoc.ID` via
   `sha256(URL + title)` hex-encoded for deterministic identity
   across requests (cross-page dedup safety).

---

## 2. Architecture

### 2.1 Package Layout

```
internal/adapters/searxng/
├── searxng.go            — Adapter, Options, New, Name, Capabilities, Healthcheck
├── searxng_test.go       — interface conformance + Capabilities + Healthcheck
├── search.go             — (*Adapter).Search hot path + URL construction
├── search_test.go        — E2E + happy path + error categorisation + 403-promotion + concurrent
├── client.go             — *http.Client construction (10s, reqid, local-only redirect allowlist)
├── client_test.go        — categorizeStatus (incl. 403-with-Retry-After) + parseRetryAfter
├── parse.go              — parseSearch transform + SHA256-derived ID + engine emission
├── parse_test.go         — field mapping + engines fallback + ID determinism + cursor
├── errors.go             — ErrInvalidQuery + ErrInvalidCursor + parseRetryAfter
├── export_test.go        — internal test helper exports
├── helpers_test.go       — shared test helpers
├── bench_test.go         — BenchmarkParseSearch25Results + TestMain goleak
└── testdata/             — 6 JSON fixtures (happy path, empty, pagination, multi_engine, no_published_date, malformed)
```

### 2.2 Key Data Structures

**`Adapter` struct** (`searxng.go`): `httpClient`, `baseURL`,
`userAgent`, `healthcheckTarget`. Immutable post-construction.

**`Options` struct**: `BaseURL` (defaults to `USEARCH_SEARXNG_URL`
env or `http://searxng:8080`), `HTTPClient`, `UserAgentVersion`,
`HealthcheckTarget` (default derived from `baseURL` host:port via
`healthcheckHostFromBase` helper).

**Response envelope** (`parse.go`): SearXNG `{query, number_of_results,
results[], suggestions, corrections, answers, infoboxes,
unresponsive_engines}`. Per-result fields: `url`, `title`, `content`,
`engine` (single), `engines` ([]string), `category`, `score`
(positive float, range engine-aggregation-dependent),
`publishedDate` (optional RFC 3339).

**Sentinels** (`errors.go`):
- `ErrInvalidQuery` — empty/whitespace.
- `ErrInvalidCursor` — non-numeric or negative.

### 2.3 Hot-Path Flow (REQ-ADP7-002)

1. Validate `q.Text` (REQ-ADP7-008).
2. Parse `q.Cursor` as integer; reject negative/non-numeric.
3. Build URL: `q`, `format=json` (hardcoded), `pageno=<N>` when
   `q.Cursor` parses to integer ≥ 1; cursor="1" produces explicit
   `pageno=1`; cursor="" omits the parameter entirely (server-side
   defaults to pageno=1).
4. `doRequest` sets UA + `Accept: application/json`.
5. Route by HTTP status via `categorizeStatus()`:
   - 200 → parseSearch.
   - 429 → `CategoryRateLimited` with parsed Retry-After.
   - 403 WITH `Retry-After` → `CategoryRateLimited` (promotion per D5).
   - 403 WITHOUT `Retry-After` → `CategoryPermanent`.
   - Other 4xx → `CategoryPermanent`.
   - 5xx + network → `CategoryUnavailable`.
6. `parseSearch()` decodes results, derives stable ID via SHA256,
   surfaces engine metadata per §6.3, sets
   `Metadata["next_cursor"] = strconv.Itoa(currentPage + 1)` on last
   doc when results are non-empty.

### 2.4 Field Mapping (REQ-ADP7-005)

Per result:
- `ID = sha256(result.url + result.title)` hex-encoded (32 chars).
- `SourceID = "searxng"` (unconditional; engine kept in Metadata).
- `URL = result.url`.
- `Title = result.title`.
- `Body = result.content`.
- `Snippet = truncate(result.content, 280)`.
- `Score = clamp(result.score, 0.0, 1.0)` (the SearXNG `score`
  field is engine-aggregation-dependent; the clamp guarantees the
  `[0.0, 1.0]` codomain regardless of upstream).
- `Lang = ""` (SearXNG does not surface per-result language in the
  default `general` category).
- `DocType = DocTypeArticle` (Capabilities advertises `[Article]`).
- `Author = ""` (default `general` category does not surface
  per-result author; per-engine extraction deferred per iteration-2
  H6).
- `PublishedAt = parsed RFC 3339` when `result.publishedDate`
  present; zero value otherwise.
- `Metadata`: REQUIRED `{engine, engines, category}`; OPTIONAL
  `{score_raw, published_date}`.

Engines fallback (iteration-2 M4): when `engines` field is null,
missing, or empty array → `Metadata["engines"] = []string{result.engine}`
(single-engine list).

### 2.5 Integration Points

- **Consumed by**: `internal/adapters/registry.go::wrappedAdapter`.
- **Consumes**: `pkg/types`, `internal/obs/reqid.NewTransport`,
  the deployed SearXNG container.
- **Downstream**: SPEC-FAN-001 cross-adapter dedup may reuse the
  engine metadata; SPEC-IDX-001 RRF consumes `Score`.

### 2.6 Local-Only Redirect Allowlist

`redirectAllowlist` accepts only:
- `searxng` (compose hostname)
- `localhost`
- `127.0.0.1`

with any port (different-port redirects within allowlist permitted
per iteration-2 M6). 3-hop cap. Any other host →
`CategoryPermanent`. SearXNG normally does not redirect.

---

## 3. Test Coverage Notes

- Coverage meets 85% target.
- 53 representative tests (iteration-2 corrected from "~38").
- `TestParseSearchIDDeterminism` (iteration-2 M1) — SHA256-derived
  ID is byte-equal across repeated parses of the same fixture.
- `TestParseSearchEnginesFallback` (iteration-2 M4) — null/missing
  `engines` field → `Metadata["engines"] = []string{result.engine}`.
- `Test403WithRetryAfterPromotedToRateLimited` — 403 + `Retry-After:
  10` → `CategoryRateLimited`; 403 without → `CategoryPermanent`.
- `BenchmarkParseSearch25Results` median ≤ 5ms; `allocs/op ≤ 50`
  per result (higher than ADP-001 ≤20/doc due to `engines []string`
  slice copy + SHA256 hash derivation; spec.md NFR-ADP7-001
  iteration-2 M5 justification).

---

## 4. Technical Decisions (Locked During Implementation)

| Decision | Rationale |
|----------|-----------|
| Hardcode `format=json` | The only JSON-emitting format SearXNG supports; the deployed instance must have it enabled in settings.yml (Open Question §11.1). |
| Local-only redirect allowlist | SearXNG never redirects cross-host in normal operation; the allowlist is defensive within trusted compose network. |
| 403-with-Retry-After → RateLimited | SearXNG's limiter emits 403 OR 429 depending on version. Promotion preserves retry semantics. |
| SHA256-derived stable IDs | SearXNG response has no stable per-result ID; SHA256 over `url + title` is collision-resistant and deterministic. |
| `Capabilities.DocTypes=[DocTypeArticle]` (iteration-2 H1) | IR-001 selectAdapterSet still selects SearXNG for any CategoryWeb query because `[Article]` intersects with the eligible set `{Article, Post, Other}`; the prior 3-element advertisement was over-promising. |
| `RateLimitPerMin=0` (iteration-2 H3) | No external rate limit; the local instance is operator-controlled via `server.limiter` in settings.yml. The prior 60/min figure was fabricated. |
| Decimal-string page-number cursor | Reused verbatim from SPEC-ADP-002 HN pattern. |
| Author hardcoded empty (iteration-2 H6) | The default `general` category does not surface per-result author; per-engine extraction is a follow-up if measured value warrants. |

---

## 5. Risks Mitigated

- **JSON format disabled in settings.yml** → 503 / 404 → operator
  sees `CategoryUnavailable` and can correct via the one-line
  settings change documented in Capabilities.Notes.
- **403 limiter behaviour across SearXNG versions** →
  `categorizeStatus` promotes 403-with-Retry-After to RateLimited.
- **Per-result Score not in `[0,1]`** → `clamp` ensures bounded
  codomain regardless of upstream.
- **Engines field absent on some responses** → fallback to single-
  engine list per iteration-2 M4 fix.

---

## 6. Out-of-Scope Reminders (from spec.md §7)

- `engines` URL parameter (operator-side engine restriction) → Open
  Question §11.2.
- `language` URL parameter → Open Question §11.3.
- `time_range` / `safesearch` URL parameters → Open Question §11.4.
- Per-engine author extraction → future SPEC.
- Live integration test (`-tags=integration` + `SEARXNG_LIVE=1`) →
  out of v0.1.

---

*End of SPEC-ADP-007 plan.md (post-hoc, v1.0)*
