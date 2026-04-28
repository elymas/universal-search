# SPEC-ADP-002 Research — Hacker News Reference Adapter

Research artifact for SPEC-ADP-002 (Hacker News adapter, M2). Produced
during the plan phase to inform EARS requirements before drafting
spec.md. Every external claim is URL-cited; every internal claim is
file:line-cited per `.moai/config/sections/research.yaml`.

---

## 1. Existing-Pattern Citations (SPEC-ADP-001 Reference Shape)

SPEC-ADP-002 inherits the file layout, error mapping discipline, MX tag
plan, and TDD harness from SPEC-ADP-001 verbatim. The reference adapter
SPEC explicitly designates ADP-002 as the first downstream consumer:

> "SPEC-ADP-002 (Hacker News, M2): can begin its plan phase as soon as
> ADP-001's spec.md is approved. ADP-002 will copy the file layout,
> error-mapping discipline, and TDD harness verbatim and customise for
> HN's Algolia API quirks."
> — `.moai/specs/SPEC-ADP-001/spec.md` §9.2

Implementation files in `internal/adapters/reddit/` define the canonical
package shape:

| File | Role | LoC | Reused As-Is by ADP-002? |
|------|------|-----|--------------------------|
| `internal/adapters/reddit/reddit.go` | Adapter struct, New, Name, Capabilities, Healthcheck, compile-time interface assertion | 136 | Pattern; substitute `"reddit"` → `"hackernews"`, base URL, defaults |
| `internal/adapters/reddit/search.go` | (*Adapter).Search hot path, query validation, URL construction, HTTP execute, response parsing | 167 | Pattern; rewrite URL builder for HN Algolia params, reuse validation/error wrapping shape |
| `internal/adapters/reddit/client.go` | HTTP client, redirect allowlist, doRequest helper, categorizeStatus rosetta | 125 | Mostly pattern; `categorizeStatus` and `parseRetryAfter` (defined in `errors.go`) are domain-agnostic and adopted with adapter name swap; redirect allowlist tuned to HN host set |
| `internal/adapters/reddit/parse.go` | parseListing JSON → []NormalizedDoc transform | 203 | Pattern; rewrite for HN Algolia hit envelope (different JSON shape) |
| `internal/adapters/reddit/score.go` | normalizeScore Tanh formula, package-level constants | 41 | Verbatim formula; constants stay (HN points distribution similar order of magnitude to Reddit) |
| `internal/adapters/reddit/errors.go` | ErrInvalidQuery sentinel, parseRetryAfter helper | 64 | Pattern; sentinel renamed `ErrInvalidQuery` (private), `parseRetryAfter` adopted as-is |
| `internal/adapters/reddit/bench_test.go` | NFR benchmark BenchmarkParseListing25Docs | — | Pattern; rename to BenchmarkParseHNHits25Docs |

The compile-time interface assertion pattern at
`internal/adapters/reddit/reddit.go:135` (`var _ types.Adapter =
(*Adapter)(nil)`) is mandatory. The reference noop adapter at
`internal/adapters/noop/noop.go:46` documents the minimal shape.

The registry's wrappedAdapter at `internal/adapters/registry.go:172-263`
(per SPEC-CORE-001 §6.5) emits ALL observability — counter, histogram,
span, slog — so the HN adapter, like the Reddit adapter, MUST emit
nothing of its own. This sole-emitter discipline is preserved verbatim.

The error taxonomy is provided by `pkg/types/errors.go:14-218`
(SPEC-CORE-001 REQ-CORE-008): four sentinels (`ErrTransient`,
`ErrPermanent`, `ErrRateLimited`, `ErrSourceUnavailable`), `Category`
enum, typed `*SourceError`, and `OutcomeFromError` for the Prometheus
label. The HN adapter consumes this contract identically to Reddit.

The `pkg/types.NormalizedDoc` 15-field struct
(`pkg/types/normalized_doc.go:40-56`) is the return shape. ADP-002 maps
HN Algolia hit fields onto these 15 fields per the table in §3 below.

---

## 2. Hacker News Algolia API Research

### 2.1 Endpoint Choice: Algolia HN Search vs Firebase

Two public HN APIs exist:

- **Algolia HN Search** (`https://hn.algolia.com/api/v1`) — full-text
  search across 30M+ items, no auth, fast keyword search, designed for
  search use cases. Used by SearXNG's HN engine and by virtually every
  third-party HN search tool.
- **Firebase HN API** (`https://hacker-news.firebaseio.com/v0`) — flat
  read-only feeds (top, new, best, ask, show, job). No search. Designed
  for full-data ingestion + client-side filtering.

[HARD] ADP-002 uses Algolia HN Search exclusively. Rationale:

1. Universal Search is a search engine; SPEC-ADP-002's purpose is
   keyword-driven retrieval. Firebase requires fetching every item ID
   and filtering client-side — wholly inappropriate.
2. Algolia is what every other HN search adapter uses. SearXNG's
   `searx/engines/hackernews.py` and gpt-researcher's HN retriever
   both target the Algolia endpoint.
3. The roadmap explicitly says "Algolia HN API" for ADP-002
   (`.moai/project/roadmap.md:39`).

### 2.2 Algolia HN Endpoints Used

The Algolia HN API exposes two relevance modes:

- `GET https://hn.algolia.com/api/v1/search?query=<text>` — relevance
  ranking (popularity-weighted by Algolia's internal scoring)
- `GET https://hn.algolia.com/api/v1/search_by_date?query=<text>` —
  strict reverse-chronological ranking by `created_at_i` (Unix epoch)

ADP-002 v0.1 uses `search` (relevance) by default. A future filter key
`sort=date` MAY route to `search_by_date` — out of v0.1 scope per §6.

### 2.3 Algolia HN Query Parameters

Documented at https://hn.algolia.com/api (the API docs are concise; no
OpenAPI / JSON schema is published). SearXNG's reference implementation
(`searx/engines/hackernews.py`) confirms the surface:

| Param | Purpose | ADP-002 Usage |
|-------|---------|---------------|
| `query` | Free-text query | Required; from `q.Text` |
| `tags` | Filter by item type: `story`, `comment`, `poll`, `pollopt`, `show_hn`, `ask_hn`, `front_page`, `author_<name>`, `story_<id>` | Hardcoded `tags=story` for v0.1 (only stories surface) |
| `numericFilters` | Numeric range filters (e.g., `points>10`, `created_at_i>1700000000`) | OPTIONAL — surface via `Query.Filters[Key="min_points"|"since"]` in v0.1 |
| `hitsPerPage` | Page size (max 1000; default 20) | From `clamp(q.MaxResults, 1, 100)`, default 25 — same clamp policy as ADP-001 |
| `page` | Zero-indexed page number | OPTIONAL — surface via `q.Cursor` (string-encoded integer) |
| `restrictSearchableAttributes` | Limit search to title/url/author/etc. | NOT used in v0.1 (default = all) |
| `attributesToRetrieve` | Limit returned fields | NOT used in v0.1 (full hit envelope is small) |

### 2.4 Algolia HN Response Envelope

Verified from live API and from
https://github.com/searxng/searxng/blob/master/searx/engines/hackernews.py
(MIT licensed; pattern reference only — NOT a Go dependency).

Top-level shape:

```json
{
  "hits": [ /* array of hit objects */ ],
  "nbHits": 12345,
  "page": 0,
  "nbPages": 50,
  "hitsPerPage": 25,
  "exhaustiveNbHits": true,
  "query": "go programming",
  "params": "..."
}
```

A single `hit` object for a story:

```json
{
  "objectID": "39458123",
  "title": "Why Go is great",
  "url": "https://example.com/why-go-is-great",
  "author": "pg",
  "points": 245,
  "story_text": null,
  "comment_text": null,
  "num_comments": 87,
  "created_at": "2026-04-15T08:32:11.000Z",
  "created_at_i": 1713169931,
  "updated_at": "2026-04-15T09:00:00.000Z",
  "_tags": ["story", "author_pg", "story_39458123"],
  "_highlightResult": { /* Algolia search highlights — ignored */ }
}
```

Notes specific to HN that drive REQs:

- `objectID` is the canonical HN item ID (the same ID used in the
  permalink `https://news.ycombinator.com/item?id=<objectID>`).
- `url` may be `null` for "Ask HN" / "Show HN" / "Tell HN" self-posts.
  When `url` is empty, the canonical permalink is constructed as
  `https://news.ycombinator.com/item?id=<objectID>`. This is HN's own
  convention.
- `story_text` (HTML-encoded body) is non-null for self-posts (Ask HN,
  Show HN with a body). It MAY contain HTML tags (`<p>`, `<a>`,
  `<i>`); ADP-002 strips tags for `Body`/`Snippet` to produce
  plain-text suitable for synthesis. SearXNG's engine uses Python's
  `html.unescape`; the Go equivalent is a small custom strip-tags pass
  (no external dep).
- `comment_text` is non-null only when the hit is a comment
  (`tags` includes `comment`). v0.1 hardcodes `tags=story` so this
  field is always null — but the parser tolerates it for safety.
- `created_at_i` (Unix epoch int) is the authoritative timestamp.
  `created_at` (RFC 3339 string) is parseable but the integer is
  cheaper and locale-independent.
- `points` is the HN score (sum of upvotes minus flags, roughly).
  Bounded `[0, ~10,000]` in practice (HN's all-time top stories are
  ~6,000 points). Maps to NormalizedDoc.Score via the same Tanh
  formula as Reddit (REQ inherited from ADP-001 §2.3).
- `num_comments` is the count of replies; included in Metadata.
- `_tags` is an array; the first element is the item type. ADP-002
  filters hits whose first-tag-or-`_tags`-membership is not `story`
  (defensive — even though we request `tags=story`, we double-check
  the response for safety against API drift).
- `_highlightResult` is Algolia's search-highlight markup (`<em>` tags
  around matching terms). ADP-002 IGNORES this field — Title/Body
  consumers should not see Algolia markup.

### 2.5 Authentication

[HARD] No auth. Algolia HN Search is fully public, no API key, no
header required. The Capabilities descriptor sets
`RequiresAuth=false`, `AuthEnvVars=nil`.

This is one of the reasons HN was chosen as the second M2 adapter:
zero secret-management complexity, zero ENV propagation, zero
RegisterOptions complexity. The reference Reddit adapter shares this
property; ADP-002 inherits it.

### 2.6 Rate Limiting

The Algolia HN API documents no specific rate limit. Empirically:

- The endpoint sustains ~10 RPS sustained per IP without throttling
  (anecdotal, verified in SearXNG operational notes).
- Aggressive querying (>50 RPS) triggers Algolia's cluster-wide
  rate limiter, which returns HTTP 429 with no `Retry-After` header
  consistently. SearXNG's heuristic is to back off 5s on first 429.
- The roadmap entry at `.moai/project/tech.md:108` describes the HN
  rate limit as "generous" — informally, ~30/min is comfortably below
  any observed throttle.

[HARD] ADP-002 declares `Capabilities.RateLimitPerMin=60` as the
conservative published-friendly figure. Rationale:

- Enough headroom for normal usage (1 query/sec).
- Far below empirical 10 RPS ceiling.
- Documented in `Capabilities.Notes`.
- The Intent Router (SPEC-IR-001) reads this value at startup; the
  fanout layer (SPEC-FAN-001) honours it via a token bucket.
- ADP-002 itself does NOT enforce rate-limit policy locally. The
  adapter is stateless (mirror ADP-001's D3 decision). On HTTP 429,
  the adapter returns `*SourceError{Category: CategoryRateLimited}`
  with `RetryAfter` parsed from the header (default 5s when missing,
  cap 60s). SPEC-FAN-001 owns retry orchestration.

### 2.7 Pagination

Algolia uses page-and-hitsPerPage pagination. The response envelope
includes `page`, `nbPages`, and `hitsPerPage`. Cursor logic:

- v0.1 surfaces the next page number via
  `Metadata["next_cursor"]` on the LAST returned doc, encoding the
  next page index as a decimal string (e.g., `"1"` for page 2).
- Callers paginate by passing this string back as `q.Cursor` on the
  next call. The adapter parses `q.Cursor` as a non-negative integer
  via `strconv.Atoi` and sets `&page=<n>` on the URL.
- When `page+1 >= nbPages`, no `next_cursor` key is set.
- Invalid cursor (non-numeric, negative) is rejected by Search with
  `*SourceError{CategoryPermanent}` (mirrors ADP-001's empty-query
  rejection discipline).

This contract differs slightly from Reddit's opaque `t3_xxxxx`
cursor: HN cursors are integer-string. The shape (string Metadata key,
opaque to the consumer) is identical.

### 2.8 SSL / Redirect Policy

Algolia HN responds at `https://hn.algolia.com/api/v1/...` with an
issued cert valid via the system trust store. Empirically, the
endpoint does not redirect cross-domain. ADP-002 nevertheless enforces
a redirect allowlist mirroring ADP-001's SSRF guard:

- Allowlist hosts: `hn.algolia.com`, `news.ycombinator.com`.
  `news.ycombinator.com` is included because permalink construction
  for self-posts uses that host, but is NOT a redirect target in
  practice — the allowlist entry is defensive in case Algolia ever
  redirects to the canonical HN page.
- Maximum 3 hops (mirrors ADP-001 REQ-ADP-010).
- Cross-domain redirect → `*SourceError{CategoryPermanent}` with
  message containing `"cross-domain redirect"`.

---

## 3. NormalizedDoc Field Mapping (HN-Specific)

| Reddit (ADP-001) | HN Algolia (ADP-002) | NormalizedDoc Field | Transform |
|------------------|----------------------|---------------------|-----------|
| `data.name` (e.g., `t3_abc`) | `objectID` (string of integer) | `ID` | Use as-is |
| (constant `"reddit"`) | (constant `"hackernews"`) | `SourceID` | matches `Name()` |
| `"https://www.reddit.com" + permalink` | If `url != ""`: `url`; else `"https://news.ycombinator.com/item?id=" + objectID` | `URL` | Two-branch: external URL for link posts, HN permalink for self-posts |
| `data.title` | `title` | `Title` | Use as-is |
| `data.selftext` | `stripHTML(story_text)` (empty for link posts) | `Body` | Strip HTML tags for plain-text body |
| `data.selftext` truncated to 280 runes | `truncateRunes(stripHTML(story_text), 280)`; falls back to `truncateRunes(title, 280)` when `story_text` empty | `Snippet` | Same truncation discipline as ADP-001 |
| `time.Unix(int64(data.created_utc), 0).UTC()` | `time.Unix(int64(created_at_i), 0).UTC()` | `PublishedAt` | HN provides `created_at_i` (Unix int); use directly |
| (parse time) | (parse time) | `RetrievedAt` | `time.Now().UTC()` injected by caller |
| `data.author` | `author` (may be `""` for deleted users) | `Author` | Use as-is |
| `normalizeScore(data.score)` | `normalizeScore(points)` | `Score` | Same Tanh formula; HN points distribution behaves similarly to Reddit upvotes for `[0, ~5000]` typical range |
| (constant `""`) | (constant `""`) | `Lang` | HN has no per-item language field |
| (constant `DocTypePost`) | (constant `DocTypePost`) | `DocType` | Stories on HN are posts in our taxonomy |
| (nil) | (nil) | `Citations` | nil |
| 6 required + 7 optional keys | 4 required + 3 optional keys (see below) | `Metadata` | HN-specific extension bag |
| (constant `""`) | (constant `""`) | `Hash` | Consumers compute via `CanonicalHash()` |

**Required Metadata keys** (REQ-ADP2-006 enforces presence on every
returned doc):

- `num_comments` (int from `num_comments`) — comparable to Reddit's
  same-named key
- `points` (int from `points`) — HN's raw score, surfaced for
  consumers that want pre-normalisation
- `tags` (`[]string` from `_tags`, filtered to non-internal entries) —
  e.g., `["story", "front_page"]` if both apply
- `external_url` (string from `url`; empty when self-post) — mirrors
  Reddit's `external_url` key

**Optional Metadata keys** (MAY be present):

- `comment_text` (string) — ONLY when the hit is a comment, which v0.1
  filters out; documented for forward-compat
- `parent_id` (string) — story this hit belongs to, when applicable
- `next_cursor` (string) — REQUIRED on the LAST returned doc when
  another page exists; absent otherwise (same convention as ADP-001)

---

## 4. Testing Approach

Mirror ADP-001 verbatim:

- `net/http/httptest.Server` stub + golden JSON fixtures under
  `internal/adapters/hn/testdata/`.
- Zero live network calls in CI.
- Optional env-gated integration test (`-tags=integration` +
  `HN_LIVE=1`) is OUT OF SCOPE for v0.1 (deferred to a follow-up
  SPEC if measured value warrants).
- Table-driven tests preferred (`.claude/rules/moai/languages/go.md`).
- Race-detector clean under `TestSearchConcurrentSafe` (50 goroutines
  × 1 stub server) — mirrors ADP-001 REQ-ADP-011.
- Goroutine-leak check via `go.uber.org/goleak.VerifyNone` after
  context-cancel mid-flight.

Required testdata fixtures:

| File | Purpose | Size |
|------|---------|------|
| `search_response.json` | Happy path: 25 stories, mixed link + self-posts | ~6KB |
| `search_response_empty.json` | Empty `hits` array; nbHits=0 | ~200B |
| `search_response_pagination.json` | Page 0 with nbPages=5, prepares cursor round-trip | ~6KB |
| `search_response_self_post.json` | Single Ask HN with `story_text` containing HTML tags (validates strip-tags) | ~1KB |
| `search_response_deleted_author.json` | `author` empty (HN's "[deleted]" convention) | ~1KB |
| `search_response_malformed.json` | Truncated JSON for parse-error path | ~200B |
| `search_response_with_comments.json` | Mixed `_tags` (story + comment); validates filter discipline | ~3KB |

---

## 5. Gap Analysis (HN-Specific Concerns Not Present in Reddit)

| Gap | Cause | Resolution |
|-----|-------|------------|
| HTML in `story_text` | HN stores self-post bodies as HTML for native rendering | Add `stripHTML(s string) string` helper to `parse.go`. Pure stdlib (no `golang.org/x/net/html` dep): a tiny tag-strip + entity-decode pass. Open Question §11.1 tracks dependency choice if requirements escalate. |
| `url` may be empty for self-posts | Ask HN / Show HN / Tell HN have no external URL | Two-branch URL construction: `url` if non-empty, else `news.ycombinator.com/item?id=<objectID>` permalink |
| `_tags` may indicate item kind drift (comment, poll, etc.) | `tags=story` request filter not always honoured (Algolia known to have transient discrepancies) | Defensive client-side filter: skip hits whose `_tags` does NOT include `"story"` |
| Pagination is integer-page (not opaque) | Algolia's API uses `page=<n>` | Adapter still surfaces opaque-string `next_cursor`; encoding (`strconv.Itoa`) is internal. Consumers MUST treat as opaque. |
| Algolia's `_highlightResult` field bloats responses | Algolia adds search-highlight markup by default | Ignored by parser. NOT requested via `attributesToRetrieve` filter in v0.1 to keep URL simple; bandwidth penalty is small (~1KB per response). |
| Time-range filtering | HN supports `numericFilters=created_at_i>...` | OPTIONAL filter via `Query.Filters[Key="since"]` (UNIX seconds). v0.1 ships filter parsing but does NOT make it a P0 (priority P1). |
| Min-points filtering | HN supports `numericFilters=points>...` | OPTIONAL filter via `Query.Filters[Key="min_points"]` (positive integer). Priority P1 in v0.1. |
| 429 without `Retry-After` | Algolia returns 429 without the header consistently | Use `parseRetryAfter` defaulting to 5s — same default as ADP-001. Documented in `Capabilities.Notes`. |
| `points=0` valid result | New stories with no upvotes still surface | Score=0 is allowed; Validate() doesn't reject. The Tanh formula maps 0 → 0.5 (neutral), same as Reddit. |

---

## 6. Risks (HN-Specific)

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Algolia API contract drift (new field added, existing field renamed) | Low | Medium | `encoding/json` tolerates unknown fields; test fixtures pinned to documented shape. SearXNG's engine has been stable for 5+ years against this API. |
| HTML-strip helper breaks on adversarial input (script tags, malformed tags) | Medium | Low | Strip is conservative (replaces ALL tag-bracket pairs with empty); does NOT execute as HTML; passes raw-text safety test. Tested via `TestStripHTMLTable` over 8 input shapes. |
| HN's "Show HN" / "Ask HN" titles include category prefix (e.g., `"Show HN: My new tool"`) | Medium | Low | Prefix preserved as-is in `Title`. Consumers (synthesis layer) decide whether to strip — out of adapter scope. |
| HN self-posts have rich body but link posts have empty body — synthesis quality varies | Medium | Low | `Body` reflects what HN provides; downstream synthesis (SPEC-SYN-001) may fall back to `Title + Snippet` when `Body == ""`. |
| Cursor parsing fails on adversarial input (huge integers, negative, non-numeric) | Low | Low | `strconv.Atoi` catches non-numeric; explicit guard against negative. Empty cursor is valid (page 0). Excessive page (e.g., 10000) is sent to Algolia, which gracefully returns empty. |
| Hash collisions across Reddit and HN for same shared external URL | Low | Medium | `CanonicalHash` includes `SourceID` prefix per `pkg/types/normalized_doc.go:96-99` — Reddit and HN cannot collide because SourceIDs differ. Documented in NormalizedDoc godoc. |
| `points` is int64 in JSON but typically int32-bounded; overflow concern | Negligible | Low | HN's all-time top score is ~6,000 (no risk of int32 overflow). Parsed as `int` (platform-dependent, but ≥ 32 bits). |
| Algolia returns extra fields not in v0.1 schema, causing test fixtures to drift | Low | Low | Test fixtures are static. The JSON parser ignores unknown fields. |

---

## 7. Open Questions (Carried Forward to spec.md §11)

1. **HTML-strip implementation**: stdlib-only tag-strip vs adopting
   `golang.org/x/net/html` for a robust HTML parser.
   **Recommended default**: stdlib-only; HN body markup is shallow
   (`<p>`, `<a>`, `<i>` typically). Revisit if real-world HN bodies
   produce stripping bugs.
   **Resolution owner**: run-phase implementer; SPEC-SYN-001 may add
   a richer text pipeline if needed.

2. **Sort modes**: hardcode `search` (relevance) or expose a
   `Query.Filters[Key="sort"]` switch routing to `search_by_date`?
   **Recommended default**: hardcode relevance in v0.1; add
   `sort=date` filter as a P2 enhancement when SPEC-IDX-001 RRF
   integration measures the value.
   **Resolution owner**: SPEC-IDX-001 author.

3. **`hitsPerPage` clamp ceiling**: Algolia allows up to 1000;
   ADP-001 clamps at 100 (matching `Capabilities.RateLimitPerMin`
   prudence). Should ADP-002 also clamp at 100 or honour Algolia's
   1000 ceiling?
   **Recommended default**: clamp at 100 to match ADP-001 (uniform
   adapter behaviour); the per-page bandwidth saving is small.
   **Resolution owner**: SPEC-FAN-001 author may revisit if pagination
   pressure becomes a measured concern.

4. **Numeric filter expression escaping**: Algolia's
   `numericFilters` parameter is a comma-joined list of expressions
   (e.g., `"points>10,created_at_i>1700000000"`). Should the adapter
   validate each expression, or pass through whatever the caller
   supplies?
   **Recommended default**: minimal validation — only the documented
   filter keys (`since`, `min_points`) produce filter expressions;
   unknown keys are ignored. Prevents injection of arbitrary Algolia
   syntax via `Query.Filters`.
   **Resolution owner**: run-phase implementer.

5. **Healthcheck strategy**: TCP-connect to `hn.algolia.com:443`
   (mirrors ADP-001) vs an HTTP HEAD on the API root.
   **Recommended default**: TCP-connect (cheap, sufficient,
   uniform pattern with ADP-001).
   **Resolution owner**: SPEC-EVAL-002 may upgrade if richer signal
   needed.

6. **HN comment retrieval**: The Algolia API supports
   `tags=comment` to surface comment hits. v0.1 hardcodes
   `tags=story`. When (if ever) does ADP-002 surface comments?
   **Recommended default**: never in v0.1 or post-V1; comments are
   noisy for synthesis and would dilute citation quality. A separate
   `SPEC-ADP-002a` (post-V1) may add a `Query.Filters[Key="kind"]`
   switch.
   **Resolution owner**: SPEC-SYN-001 author (synthesis is the
   consumer that would benefit/suffer).

---

## 8. References

### External (URL-cited)

- https://hn.algolia.com/api — Algolia HN Search API documentation
  (concise; canonical reference).
- https://github.com/searxng/searxng/blob/master/searx/engines/hackernews.py
  — SearXNG's HN engine (MIT, pattern reference only — NOT a Go
  dependency).
- https://news.ycombinator.com/item?id=<id> — HN canonical permalink
  format (confirmed by HN's own footer linking).
- RFC 7231 §7.1.3 Retry-After header semantics (inherited from
  SPEC-ADP-001 reference parser).

### Internal (file:line cited)

- `.moai/specs/SPEC-ADP-001/spec.md` — full reference SPEC structure
  (1216 lines).
- `.moai/specs/SPEC-CORE-001/spec.md` — Adapter / Capabilities /
  Query / NormalizedDoc / SourceError contract.
- `.moai/specs/SPEC-OBS-001/spec.md` — observability bundle and
  cardinality discipline.
- `.moai/specs/SPEC-IR-001/spec.md` — `Capabilities` consumer
  contract (REQ-IR-008).
- `internal/adapters/reddit/reddit.go:1-136` — Adapter struct, New,
  Capabilities, Healthcheck, compile-time interface assertion.
- `internal/adapters/reddit/search.go:1-167` — Search hot path,
  query validation, URL construction, error wrapping pattern.
- `internal/adapters/reddit/parse.go:1-203` — JSON → NormalizedDoc
  transform pattern.
- `internal/adapters/reddit/client.go:1-125` — HTTP client, redirect
  allowlist, `categorizeStatus` rosetta.
- `internal/adapters/reddit/score.go:1-41` — Tanh score normaliser
  (reused as-is by ADP-002).
- `internal/adapters/reddit/errors.go:1-64` — `parseRetryAfter`
  helper (reused as-is by ADP-002).
- `internal/adapters/registry.go:172-263` — wrappedAdapter
  sole-emitter pattern (consumed by ADP-002).
- `pkg/types/normalized_doc.go:40-106` — NormalizedDoc struct,
  Validate, CanonicalHash.
- `pkg/types/adapter.go` — Adapter interface 4-method shape.
- `pkg/types/capabilities.go:38-62` — Capabilities struct + DocType
  enum.
- `pkg/types/query.go:18-44` — Query struct + Filter shape.
- `pkg/types/errors.go:14-218` — SourceError, Category enum,
  CategorizeError, OutcomeFromError.
- `internal/llm/client.go:31-65` — HTTP client + reqid Transport
  pattern (inherited from ADP-001 reference shape).
- `.moai/project/roadmap.md:39, 122, 149` — M2 row, parallelization,
  exit criterion.
- `.moai/project/tech.md:108` — HN row in adapter strategy
  ("Algolia HN API ... generous ... stable, no-auth").
- `.moai/config/sections/quality.yaml` — `development_mode: tdd`,
  `test_coverage_target: 85`.
- `.moai/config/sections/harness.yaml` — auto-routing to standard
  level.
- `.moai/config/sections/language.yaml` — `documentation: en`,
  `code_comments: en`.

---

*End of SPEC-ADP-002 research.md v0.1*
