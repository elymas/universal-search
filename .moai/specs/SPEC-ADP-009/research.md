# SPEC-ADP-009 Research — KoreaNewsCrawler + Daum + Korean RSS Adapter

**Status**: Research-complete; ready for SPEC drafting
**Author**: limbowl (via manager-spec)
**Created**: 2026-05-04
**Milestone**: M3 — Fanout, adapters, index
**Depends on**: SPEC-CORE-001, SPEC-OBS-001, SPEC-IR-001
**Soft deps / siblings**: SPEC-FAN-001 (gateway), SPEC-ADP-001 (reference shape), SPEC-ADP-002 (second-shape duplication), SPEC-ADP-008 (Naver, primary Korean source — assumed parallel-planned)

---

## 0. Research Mandate

`.moai/project/roadmap.md:54` defines SPEC-ADP-009 as "KoreaNewsCrawler + 다음 + Korean RSS — wrap `lumyjuwon/KoreaNewsCrawler`, user-configurable RSS list". `.moai/project/tech.md:117` describes the adapter as "다음 / KoreaNewsCrawler — wrap lumyjuwon/KoreaNewsCrawler — none — scraper-style — fallback Korean news". `.moai/project/tech.md:118` adds "RSS (user-configured) — Tanuki / lightweight gofeed — none — 5min cache — for internal feeds".

This SPEC is the THIRD Korean-locale adapter in the M3 set:
- **SPEC-ADP-008 Naver** (planned in parallel) covers the PRIMARY Korean web/news/blog/shopping path via `isnow890/naver-search-mcp` with API-key auth and 25,000/day budget.
- **SPEC-ADP-009 (this SPEC)** covers the FALLBACK Korean breadth path: historical archive (KoreaNewsCrawler), supplementary search (Daum), and operator-configured RSS feeds (gofeed). Priority is **P1** (not strict P0) because Naver is already approved for the primary path; ADP-009's role is breadth, not single-point-of-failure coverage.

The mandate is to:

1. Document each of the THREE sub-source surfaces (KoreaNewsCrawler, Daum, RSS) with concrete API/protocol references, exactly as a single composite adapter exposing one `Adapter` contract from `pkg/types/adapter.go:28-45`.
2. Decide which sub-sources are **achievable in v0.1** versus **deferred** with explicit rationale and ToS / robots.txt evidence.
3. Map each viable sub-source's response shape to the `pkg/types.NormalizedDoc` 15-field canonical contract from SPEC-CORE-001.
4. Enumerate risks per sub-source (license, scraping legality, rate-limit, freshness) and propose mitigations.
5. Produce a per-sub-source feasibility matrix the SPEC author can use to lock the v0.1 scope deterministically.
6. List Open Questions deferred to SPEC drafting / run phase / future enhancement SPECs.

Every external claim is URL-cited; every internal claim is file:line-cited. No invented facts.

---

## 1. Adapter Composition Architecture

### 1.1 Single Adapter, Three Sub-Source Paths

[HARD] SPEC-ADP-009 is ONE adapter struct (`internal/adapters/koreanews/Adapter`) implementing the four-method `pkg/types.Adapter` contract, which internally dispatches to up to THREE sub-source paths gated by individual enable flags. This matches `.moai/project/structure.md:18-22` which reserves `internal/adapters/daum/` and `internal/adapters/rss_korean/` directory names; the SPEC consolidates them into a single `koreanews/` package because the three sub-sources share a single `RoutingDecision.AdapterSet` slot under the IR-001 `korean` Category and serving them as one composite adapter avoids cardinality bloat in observability labels (`adapter="koreanews"` vs three separate labels).

Rationale for composition over three separate adapters:
- **Cardinality discipline**: SPEC-OBS-001 caps the `adapter` label values; three Korean sub-sources at P1 priority do not justify three label slots when one composite suffices. Naver (ADP-008) gets its own slot because it is P0 primary.
- **Routing simplicity**: IR-001's `korean` Category routes to ALL Korean adapters by `Capabilities.SupportedLangs`. With one composite, the Category fans out to N=2 adapters (Naver + KoreaNews-composite); with three separate, N=4. The composite reduces fanout overhead and keeps the user-facing SourceID stable.
- **Operational clarity**: The three sub-sources share a single config block, single rate-limit budget, and single failure mode. Operators tune one adapter, not three.
- **Future portability**: If KoreaNewsCrawler's Python sidecar is later promoted to a separate SPEC (SPEC-ADP-009a), the composite can de-merge cleanly because the three internal paths are already separated by `subsource` enum.

Trade-off accepted: a single error from one sub-source (e.g., RSS feed unreachable) is reported under the composite's name. The `Metadata["subsource"]` field on each returned `NormalizedDoc` carries the actual sub-source value (`"rss"`, `"daum"`, `"knc"`) so consumers can attribute results.

### 1.2 Sub-Source Enable Flags

Each sub-source is independently enableable via env var:

| Env var | Default | Effect |
|---------|---------|--------|
| `USEARCH_ADP009_RSS_ENABLED` | `true` | Enables the user-configurable RSS feed list path |
| `USEARCH_ADP009_DAUM_ENABLED` | `false` | Enables the Daum search path (DEFAULT OFF in v0.1 — ToS / robots.txt block; see §3) |
| `USEARCH_ADP009_KNC_ENABLED` | `false` | Enables the KoreaNewsCrawler Python sidecar path (DEFAULT OFF in v0.1 — sidecar deferral; see §2) |

Rationale for default-off on KNC + Daum:
- **Daum default-off** is a hard requirement: `https://search.daum.net/robots.txt` (verified 2026-05-04 via WebFetch) returns the two-line file `User-agent: *\nDisallow: /` — every path is explicitly disallowed for all crawlers. Default-on would constitute a ToS violation. Operators who want to enable face the legal/ethical decision themselves.
- **KNC default-off** is a pragmatic deferral: the Python library `lumyjuwon/KoreaNewsCrawler` is unmaintained (last release 2022-03-27, ~3 years stale) and its scraper-style targeting of Naver portal is fragile against portal HTML changes. v0.1 ships the integration scaffold (sidecar plumbing under `services/koreanews/`) but defaults to OFF; operators who need historical Korean news scraping flip the flag knowing the freshness/legal trade-offs.
- **RSS default-on** is safe: gofeed parses operator-supplied feed URLs only. Operators control the source list explicitly via config; no scraping happens.

The Capabilities descriptor reflects the sub-source enablement at `Capabilities()` call time (deterministic per SPEC-CORE-001 contract): adapters fold the enable flags into a single `Notes` substring listing which sub-sources are active.

### 1.3 Combined Search Method Dispatch

```
(*Adapter).Search(ctx, q):
  ├── if USEARCH_ADP009_RSS_ENABLED → searchRSS(ctx, q)        // gofeed parse of N pre-configured feed URLs
  ├── if USEARCH_ADP009_DAUM_ENABLED → searchDaum(ctx, q)      // best-effort HTML parse OR explicit error
  ├── if USEARCH_ADP009_KNC_ENABLED → searchKNC(ctx, q)        // HTTP call to Python sidecar at services/koreanews/
  └── merge + dedup-by-URL + sort by recency, return []NormalizedDoc
```

Each sub-source returns a partial `[]NormalizedDoc` slice; the composite merges them (URL-canonicalization-based dedup mirroring SPEC-FAN-001 §2.3 algorithm — same first-occurrence-wins discipline, deterministic), tags each doc with `Metadata["subsource"] = "<name>"`, and returns the merged slice. The composite NEVER returns a SUCCESS error if at least one sub-source returned at least one doc — sub-source-level errors collapse into `Metadata` annotations (per-doc when applicable) and Notes-level summaries; only when ALL sub-sources fail does the composite return a `*SourceError`.

**Distinction from SPEC-FAN-001 fanout**: The fanout layer (SPEC-FAN-001) operates at the ADAPTER level (`registry.Get("koreanews").Search(...)`); the SPEC-ADP-009 internal sub-source dispatch operates at the SUB-ADAPTER level inside the composite. Both use the same dedup/sort discipline, but they live at different scopes — the fanout dedups across `koreanews` + `naver` + `reddit` etc.; the composite dedups within the three Korean sub-sources only. Consumers never see the internal dispatch.

---

## 2. KoreaNewsCrawler (Python Sidecar Path)

### 2.1 Library Surface

**Source**: `https://github.com/lumyjuwon/KoreaNewsCrawler` (verified via WebFetch 2026-05-04)

**License**: MIT (verified)
**Last release**: v1.51 on 2022-03-27 (~3 years stale at SPEC creation date 2026-05-04)
**Activity**: 186 commits on master, 10 releases, considered effectively unmaintained
**Install**: `pip install KoreaNewsCrawler`

**Public API** (from README extract):

```python
from korea_news_crawler.articlecrawler import ArticleCrawler

Crawler = ArticleCrawler()
Crawler.set_category("politics", "IT_science", "economy")
Crawler.set_date_range("2017-01", "2018-04-20")
Crawler.start()
```

**Supported categories** (`ArticleCrawler.set_category`):
- `politics`, `economy`, `society`, `living_culture`, `IT_science`, `world`, `opinion`

**Supported sports categories** (`SportCrawler.set_category`): Korea baseball, Korea soccer, world baseball, world soccer, basketball, volleyball, golf, general sports, e-sports.

**Output schema** (CSV columns A-F):
- A: Article date & time
- B: Category (Korean string from set above)
- C: Media Company (e.g., "조선일보", "동아일보", "한겨레", etc. — the actual list is whatever appears on Naver portal)
- D: Article title
- E: Article body
- F: Article URL

**Data source**: The library "crawls news articles from media organizations posted on NAVER portal". This means KNC scrapes Naver's news aggregator, NOT individual newspaper sites. Implications:
- Naver's portal HTML schema changes break KNC silently (no upstream maintenance to track).
- ToS exposure is to Naver's portal, not the originating newspaper. Naver's portal terms govern scraping; this is the same surface area covered (officially) by SPEC-ADP-008 Naver-MCP, which is API-key authenticated and respects 25,000/day.
- KNC essentially does what ADP-008 already does, but unauthenticated and via HTML scraping. The value-add for ADP-009 is **historical archive coverage** (date_range queries beyond ADP-008's "recent items" window) and **fallback when Naver API is unavailable**.

**Documented disclaimers / ToS warnings**: NONE. The README does not warn about scraping legality.

### 2.2 Python Sidecar Pattern (Existing Convention)

`.moai/project/structure.md` (line not specified but `services/researcher/` and `services/embedder/` exist) establishes the pattern: Python libraries that don't have native Go ports run as HTTP/gRPC sidecars under `services/`. Existing pattern at `services/researcher/` (gpt-researcher wrapper, FastAPI + Pydantic v2 per SPEC-SYN-001 implementation 2026-05-04):
- `services/researcher/Dockerfile`
- `services/researcher/pyproject.toml`
- `services/researcher/src/` (handler code)
- `services/researcher/tests/`

**Proposed pattern for KNC**: `services/koreanews/`:
- `services/koreanews/pyproject.toml` — pins `KoreaNewsCrawler==1.51` + FastAPI + uvicorn
- `services/koreanews/src/main.py` — FastAPI endpoint `POST /search` accepting `{categories: [...], date_from: "YYYY-MM", date_to: "YYYY-MM", query: "<text>", max_results: int}` returning `{articles: [{date, category, media, title, body, url}], errors: [...]}`
- `services/koreanews/src/handler.py` — wraps `ArticleCrawler.set_category(...)`, `set_date_range(...)`, intercepts CSV output via in-memory IO (avoid disk writes), filters articles by `query` substring or future Korean tokenizer (out of v0.1 scope).
- `services/koreanews/Dockerfile` — builds slim Python 3.12 image
- `services/koreanews/tests/` — pytest with mocked `ArticleCrawler`

**Go-side client** (`internal/adapters/koreanews/knc_client.go`):
- HTTP POST to `${USEARCH_ADP009_KNC_BASE_URL}/search` (default `http://localhost:8002`)
- Honours ctx; 30s default timeout (Korean news scraping is slow).
- Decodes JSON response; one error per sub-source error returned.

**Why default-off (v0.1)**:
- Sidecar deferral: building, dockerizing, and integrating the Python sidecar is non-trivial scaffolding work. v0.1 ships the GO-side stub (HTTP client + config flag) and DOCUMENTS the sidecar requirement but does NOT block on Python service implementation. A future SPEC-ADP-009-KNC may complete the sidecar.
- Maintenance freshness: 2022-stale Python lib's CSS selector targeting will likely have drifted; debugging Korean portal HTML in v0.1 distracts from the M3 milestone goal.
- Operator opt-in: Korean teams that need historical archive scraping can stand up the sidecar themselves with the documented HTTP contract.

### 2.3 KNC NormalizedDoc Field Mapping

When the sidecar returns articles, the Go client maps per-article CSV fields to `NormalizedDoc`:

| Sidecar JSON field | NormalizedDoc field | Transform |
|--------------------|---------------------|-----------|
| `url` | `ID` | `"knc:" + url-canonical-bytes` (deterministic per-article) |
| (constant) | `SourceID` | `"koreanews"` (matches `Adapter.Name()`) |
| `url` | `URL` | Use as-is (already canonical Naver portal URL) |
| `title` | `Title` | Use as-is (Korean) |
| `body` | `Body` | Use as-is (Korean, may be long) |
| First 280 runes of `body` | `Snippet` | Truncate; if longer, append "..."; if `body` empty, derive from `title` |
| `date` | `PublishedAt` | Parse Korean date string (`"YYYY-MM-DD HH:MM"`); UTC normalise. If parse fails, set to zero. |
| (parse time) | `RetrievedAt` | `time.Now().UTC()` |
| `media` | `Author` | Use as-is (e.g., "조선일보" — newspaper name as Author per Korean news convention) |
| (constant) | `Score` | `0.5` (no upvote/relevance signal; mid-bucket placeholder for RRF input) |
| (constant) | `Lang` | `"ko"` (KNC sources are exclusively Korean) |
| (constant) | `DocType` | `types.DocTypeArticle` |
| (nil) | `Citations` | `nil` |
| `category` | `Metadata["category"]` | Use as-is (Korean category string) |
| (constant) | `Metadata["subsource"]` | `"knc"` |
| (constant) | `Metadata["data_source"]` | `"naver_portal"` (since KNC scrapes Naver, not direct) |
| (constant) | `Hash` | `""` (consumers compute via `CanonicalHash()`) |

---

## 3. Daum Search (Best-Effort, Default-Off)

### 3.1 Daum Surface

**Source**: `https://search.daum.net/` (Daum is the search engine subsidiary of Kakao Corp., one of two major Korean search portals alongside Naver)

**Public API**: NONE. Kakao does not publish a documented Daum Search API in 2026. Historical "Daum API" deprecated years ago when Daum merged into Kakao; current Kakao Developers offerings (`https://developers.kakao.com/`) cover KakaoMap, KakaoTalk, Kakao Login, but NOT a generic Daum web/news search endpoint comparable to Naver's `openapi.naver.com/v1/search/news.json`.

**Robots.txt verdict** (verified via WebFetch 2026-05-04 against `https://search.daum.net/robots.txt`):

```
User-agent: *
Disallow: /
```

This is **two lines**: a blanket disallow for all crawlers across the entire `search.daum.net` domain. There is no `Sitemap:` directive. No path-specific allowlist. Daum's robots.txt explicitly forbids automated access.

### 3.2 Implementation Options Considered

| Option | Description | Verdict |
|--------|-------------|---------|
| Path A — HTML scraping with custom UA | Issue HTTP GET to `https://search.daum.net/search?q=<query>&w=news` and parse HTML | REJECTED — direct violation of robots.txt; cannot ship default-on |
| Path B — Reverse-engineered XHR endpoints | Inspect Daum's mobile/desktop XHR calls; replay them | REJECTED — same robots.txt violation; additionally fragile |
| Path C — Third-party Daum search proxies | Use a hosted scraper (e.g., ScraperAPI, ScrapingBee) that abstracts ToS exposure | REJECTED — adds external paid dependency; v0.1 ships pure stdlib |
| Path D — Naver as Daum surrogate (since both are Korean portals) | Defer Daum coverage to SPEC-ADP-008 Naver | PARTIAL — Naver covers most of what Daum does, but specific Daum-exclusive results (Kakao-curated content) are lost |
| Path E — Default-OFF stub implementation | Wire the sub-source path with `enabled=false` default; if operator opts in, return a clear `*SourceError{Category: CategoryPermanent, Cause: "daum: scraping disabled by default per robots.txt; opt in via USEARCH_ADP009_DAUM_ENABLED at your own risk"}` and document the legal posture in `Capabilities.Notes` | **ACCEPTED** for v0.1 |

### 3.3 Default-Off Stub Decision (v0.1)

[HARD] In v0.1, if `USEARCH_ADP009_DAUM_ENABLED=true`, the adapter SHALL still return an explicit `*SourceError{Category: CategoryPermanent, Cause: ErrDaumDisabled}` (a package-level sentinel) UNTIL a future SPEC-ADP-009-DAUM is approved with explicit legal review and an operator-supplied compliance assertion. The flag itself is plumbed (so the stub can be replaced cleanly later) but its activation does NOT change the response shape — the SPEC author and run-phase implementer never write a scraping path in v0.1.

**Why this approach over removing Daum entirely**:
- The roadmap (`.moai/project/roadmap.md:54`) and tech matrix (`.moai/project/tech.md:117`) reference Daum explicitly as a target; removing the slot now creates churn when a future SPEC needs it.
- Plumbing the flag keeps the stable Adapter shape; future SPEC-ADP-009-DAUM lands a single function impl, not a refactor.
- The `Notes` field warns operators of the legal posture, providing transparency.

### 3.4 Daum Mapping (DEFERRED)

When (and only when) a future SPEC-ADP-009-DAUM authorises Daum scraping, the mapping table will live in that SPEC's research.md. v0.1 publishes no Daum-specific mapping.

---

## 4. RSS Feeds (gofeed; Default-On)

### 4.1 gofeed Library

**Source**: `https://github.com/mmcdole/gofeed` (verified via WebFetch 2026-05-04)

**License**: MIT (verified)
**Latest stable version**: v1.3.0 (released 2024-03-01)
**Maintenance**: Active

**API**:

```go
import "github.com/mmcdole/gofeed"

fp := gofeed.NewParser()

// from URL with context
ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
defer cancel()
feed, err := fp.ParseURLWithContext("http://feeds.example.com/feed.xml", ctx)

// from bytes / string / io.Reader
feed, err := fp.ParseString(rawXMLString)
feed, err := fp.Parse(reader)
```

**Output struct** (`gofeed.Feed`):
- `Title string`, `Description string`, `Link string`
- `Published string` (raw string), `PublishedParsed *time.Time` (parsed)
- `Author *gofeed.Person`
- `Items []*gofeed.Item`

**Per-item struct** (`gofeed.Item`):
- `Title`, `Description`, `Content`, `Link`, `Links []string`
- `Published string`, `PublishedParsed *time.Time`
- `Author *gofeed.Person` (single author)
- `GUID string`, `Categories []string`
- `Image *gofeed.Image`, `Enclosures []*gofeed.Enclosure`
- `DublinCoreExt`, `ITunesExt` (built-in extension support)
- `Custom map[string]string` (unstructured extras)

**Supported feed formats**: RSS 0.90 / 0.91 / 0.92 / 0.93 / 0.94 / 1.0 / 2.0; Atom 0.3 / 1.0; JSON Feed 1.0 / 1.1.

**Malformed-feed handling**: "Best-effort approach" — handles unescaped markup, undeclared namespace prefixes, missing/illegal tags, incorrect date formats. Korean publishers' feeds (often EUC-KR encoded) are NOT explicitly listed as supported, but gofeed's underlying `goxpp` parser respects the XML encoding declaration (`<?xml version="1.0" encoding="EUC-KR"?>`) when present. Fallback: feeds without correct encoding declaration may produce garbled `Title`/`Description` strings; the adapter SHOULD detect non-UTF-8 byte sequences and emit a `Metadata["encoding_warning"]` annotation.

**Dependencies pulled by gofeed v1.3.0**:
- `github.com/mmcdole/goxpp` (XML pull parser; small)
- `github.com/PuerkitoBio/goquery` (jQuery-like HTML; for content extraction from RSS `description` HTML)
- `github.com/json-iterator/go` (JSON iterator for JSON Feed)
- `github.com/stretchr/testify` (test-only)

`go.mod` impact: ~5 new direct dependencies. Acceptable per SPEC-DEP-001 conventions (dependencies are MIT, well-maintained, stable releases).

### 4.2 RSS Feed List Configuration

The adapter MUST accept an operator-configured list of feed URLs. v0.1 design:

**Source**: env var `USEARCH_ADP009_RSS_FEEDS` containing either:
- Path A — JSON array of strings: `["https://news.example.kr/rss", "https://blog.example.kr/feed.xml"]`
- Path B — Comma-separated: `https://a.kr/rss,https://b.kr/feed`

Path A is recommended; the adapter parses with `json.Unmarshal` first, falls back to comma-split if JSON parse fails.

**Alternative path C** — path to YAML file via `USEARCH_ADP009_RSS_FEEDS_FILE`:
```yaml
# /etc/usearch/rss-feeds.yaml
feeds:
  - url: https://news.example.kr/rss
    label: "Example News"          # optional display name
    timeout_seconds: 30            # optional per-feed override
  - url: https://blog.example.kr/feed.xml
    label: "Example Blog"
```

For v0.1, env var (path A or B) is the canonical mechanism; YAML file (path C) is OPTIONAL — implemented if a low-effort koanf integration is justified, OTHERWISE deferred to a future config SPEC. The Open Question §6.4 tracks this decision.

**Empty list handling**: if the env var is unset OR parses to an empty list, RSS sub-source returns `(nil, nil)` with NO error and NO HTTP requests. The composite handles this gracefully.

**Maximum feed count**: hardcoded ceiling of 32 feeds in v0.1 (prevents runaway config-induced fanout). Excess entries beyond 32 are silently truncated with a slog WARN.

### 4.3 RSS Field Mapping

| gofeed.Item field | NormalizedDoc field | Transform |
|-------------------|---------------------|-----------|
| `GUID` (or `Link` if GUID empty) | `ID` | `"rss:" + url-canonical-bytes(GUID || Link)` |
| (constant) | `SourceID` | `"koreanews"` (composite Adapter name) |
| `Link` | `URL` | Use as-is; canonicalise per dedup rules from SPEC-FAN-001 §2.4 |
| `Title` | `Title` | Use as-is; trim whitespace |
| `Content` (if non-empty) else `Description` | `Body` | HTML-strip via `stripHTML` (mirror SPEC-ADP-002 §6.4 helper); decode entities |
| First 280 runes of stripped Body | `Snippet` | Truncate; if longer append "..." |
| `PublishedParsed` (if non-nil) else `time.Unix(0, 0)` | `PublishedAt` | UTC normalise |
| (parse time) | `RetrievedAt` | `time.Now().UTC()` |
| `Author.Name` (if non-nil) else `""` | `Author` | RSS `<author>` is often empty; missing is acceptable |
| (constant) | `Score` | `0.5` (no relevance signal from RSS; placeholder for RRF) |
| Detected language (Korean if `unicode/utf8` valid + Hangul rune presence ≥ 30% of title+body) | `Lang` | Heuristic detection; `"ko"` or `""` (unknown). Future enhancement: language-detect library. |
| (constant) | `DocType` | `types.DocTypeArticle` |
| (nil) | `Citations` | `nil` |
| `feed-source-url` (operator config) | `Metadata["feed_url"]` | The feed URL this item came from |
| `feed.Title` (if non-empty) | `Metadata["feed_title"]` | The feed's own title |
| `Categories []` (if non-empty) | `Metadata["categories"]` | gofeed Item.Categories slice as-is |
| (constant) | `Metadata["subsource"]` | `"rss"` |
| (constant) | `Hash` | `""` (consumers compute) |

### 4.4 Per-Feed Error Handling

[HARD] Each feed fetch is INDEPENDENT — a single feed timeout / 4xx / malformed XML SHALL NOT cancel sibling feed fetches. Implementation: parallelise the N feed fetches with `errgroup.SetLimit(min(8, len(feeds)))` (mirroring SPEC-FAN-001 §2.5 pattern verbatim). Each per-feed worker:

1. Per-feed ctx derivation: `min(perFeedTimeout=30s, time-until-parent-deadline)`.
2. `gofeed.NewParser().ParseURLWithContext(ctx, feedURL)`.
3. On error: record `feed_error="<msg>"` in a per-feed-index error slot; return zero docs.
4. On success: transform items per §4.3; return docs.

After all per-feed workers complete, the composite assembles the merged RSS doc list, sets `Metadata["feed_errors_count"]` on the FIRST returned doc to expose the per-feed failure count to consumers.

Error categorisation for RSS sub-source:
- HTTP 4xx (404 feed gone) → `*SourceError{Category: CategoryPermanent}` for that feed only; sibling feeds unaffected.
- HTTP 5xx / timeout → `*SourceError{Category: CategoryUnavailable}` for that feed only.
- Malformed XML → `*SourceError{Category: CategoryPermanent}` for that feed only.
- Empty feed (zero items, valid XML) → no error; returns `(nil, nil)` from that feed.

If ALL feeds fail AND no other sub-source returned docs, the composite Search returns `*SourceError{Category: CategoryUnavailable}` to signal complete failure.

---

## 5. Existing Codebase Patterns to Follow

### 5.1 Adapter Interface (mirror ADP-001/002)

`pkg/types/adapter.go:28-45` — the four-method `Adapter` interface. Composite adapter implements:
- `Name() string` returns `"koreanews"` (single value despite three sub-sources).
- `Search(ctx, q) ([]NormalizedDoc, error)` dispatches per §1.3.
- `Healthcheck(ctx) error` — TCP-connect probe to a representative endpoint per enabled sub-source. v0.1 simplifies: probes ONLY the RSS path (since it is default-on); for KNC/Daum (default-off), Healthcheck succeeds vacuously when sub-source is disabled. Test stubs inject a `HealthcheckTarget` Options field.
- `Capabilities() types.Capabilities` returns deterministic descriptor.

### 5.2 Capabilities Shape

Required values (consumers per SPEC-IR-001 REQ-IR-008):
- `SourceID = "koreanews"`
- `DisplayName = "Korean News (RSS + KoreaNewsCrawler + Daum)"`
- `DocTypes = []types.DocType{types.DocTypeArticle}`
- `SupportedLangs = []string{"ko"}` — explicit Korean (UNLIKE Reddit/HN which set nil for language-agnostic)
- `SupportsSince = false` — RSS feeds don't accept date-range queries; KNC sidecar's date_range is opaque to ADP-009 v0.1
- `RequiresAuth = false` — RSS is public; KNC sidecar is internal localhost; Daum is disabled
- `AuthEnvVars = nil`
- `RateLimitPerMin = 0` (unknown / variable per feed)
- `DefaultMaxResults = 50` — rolls up across sub-sources; per-feed cap is much smaller
- `Notes` — composite string listing enabled sub-sources, RSS feed count, ToS posture

### 5.3 SourceError Construction (mirror ADP-001 §6.5 verbatim)

The composite uses the same `*types.SourceError` shape as Reddit/HN:
- All four Categories reachable: Permanent (Daum disabled, 4xx feeds, malformed XML), Unavailable (5xx feeds, KNC sidecar down, all-sub-sources-failed), RateLimited (none in v0.1; deferred to FAN-001 retry SPEC), Transient (network blip on RSS fetch, ctx cancel mid-flight).
- The composite delegates to per-sub-source `categorizeStatus`-shape helpers; no new error sentinels beyond `ErrInvalidQuery` (empty query), `ErrDaumDisabled` (ToS gate), `ErrKNCSidecarDown` (Python sidecar unreachable), `ErrEmptyRSSFeedList` (config gate when RSS sub-source enabled but feed list empty — distinct from "feeds returned empty results" which is silent).

### 5.4 Sole-Emitter Discipline

Mirroring SPEC-ADP-001 §6.4 verbatim: the adapter emits ZERO Prometheus metrics, ZERO OTel spans, and ZERO slog records of its own. ALL observability flows through the registry's `wrappedAdapter` at `internal/adapters/registry.go:172-263`. The composite returns the correctly-categorised `*SourceError` so `OutcomeFromError(err)` (`pkg/types/errors.go:174-193`) produces the right `outcome` label.

### 5.5 Concurrent-Safety Contract (mirror ADP-001 REQ-ADP-011, ADP-002 REQ-ADP2-010)

[HARD] Multiple goroutines invoking `(*koreanews.Adapter).Search` concurrently SHALL be race-clean. The adapter holds:
- One `*http.Client` for RSS gofeed Parser (goroutine-safe per Go stdlib).
- One `*http.Client` for KNC sidecar (goroutine-safe per Go stdlib; nil when KNC disabled).
- Immutable config (sub-source flags, RSS feed list, sidecar base URL) loaded once in `New`.

The internal sub-source dispatch is the same `errgroup` pattern as SPEC-FAN-001 — pre-allocated per-index slices, no shared map writes, supervisor builds the merged result.

Test pattern: `TestSearchConcurrentSafe` 50 goroutines × 1 Search per `*Adapter` race-clean (mirrors ADP-001/002 NFR test).

---

## 6. Open Questions

These are intentionally deferred; they do not block SPEC approval. Each documents the recommended default and the resolution owner.

### 6.1 Korean Locale Detection on RSS Items

**Question**: Should RSS items be auto-tagged `Lang="ko"`, or should the adapter detect Korean content and tag accordingly?

**Recommended default**: Heuristic detection in v0.1 — count Hangul Unicode runes (`unicode.Is(unicode.Hangul, r)`) in `title + body`; if ratio ≥ 0.30, set `Lang="ko"`, else leave `Lang=""`. This handles the case where an operator configures an English-language tech blog feed alongside Korean newspaper feeds. SPEC-IR-001's routing then differentiates.

**Resolution owner**: SPEC-IDX-003 (Korean tokenization) author may upgrade detection to a real language-detect library at M3 if heuristic produces false-positives.

### 6.2 KNC Sidecar Implementation Timeline

**Question**: Should v0.1 include the Python sidecar build, or only the Go HTTP client?

**Recommended default**: Go HTTP client + sidecar SCAFFOLD only. The `services/koreanews/` directory ships with `pyproject.toml`, `Dockerfile`, and a stub FastAPI handler returning `503 Service Unavailable` ("KNC sidecar not yet implemented"). The full KNC integration ships in a future SPEC-ADP-009-KNC. Operators who need KNC immediately can implement the handler against the documented contract.

**Resolution owner**: future SPEC-ADP-009-KNC author after M3 completes; deferral is consistent with the v0.1 "narrow scope" discipline shared with ADP-001 (which deferred OAuth) and ADP-002 (which deferred comment retrieval).

### 6.3 Daum Re-Enablement Path

**Question**: Under what conditions can Daum scraping be enabled?

**Recommended default**: NEVER without legal review. v0.1 ships Daum as a stub returning `ErrDaumDisabled` regardless of the env flag (the flag is plumbed for future SPEC consumption only). A future SPEC-ADP-009-DAUM may unlock the path, but ONLY with: (a) explicit Kakao authorisation OR (b) operator-attested compliance with their own jurisdiction's web-scraping law (e.g., DMCA §1201 review in US, GDPR / Korean PIPA review in Korea). v0.1 documents the legal posture in `Notes`.

**Resolution owner**: future SPEC-ADP-009-DAUM author + legal review.

### 6.4 RSS Config Source (env var vs YAML file vs koanf)

**Question**: Should v0.1 implement the YAML file config path, or env var only?

**Recommended default**: env var only in v0.1 (`USEARCH_ADP009_RSS_FEEDS` JSON or comma-list; `USEARCH_ADP009_RSS_FEEDS_FILE` deferred to a future config SPEC if needed). Env var keeps the surface narrow; koanf-based YAML loading is a horizontal concern across all adapters that should be SPEC'd once globally rather than per-adapter.

**Resolution owner**: future SPEC-ADP-CFG-001 author (covers config patterns across adapters).

### 6.5 Feed Encoding Detection (UTF-8 vs EUC-KR)

**Question**: Should the adapter detect and convert non-UTF-8 feed encodings?

**Recommended default**: NO conversion in v0.1; `gofeed`'s underlying `goxpp` parser respects the XML `encoding=` declaration when present. Feeds without correct declarations produce garbled output, which the adapter SHOULD annotate with `Metadata["encoding_warning"]="non-utf-8 byte sequence detected"` after a `unicode/utf8.ValidString(title || body)` check fails. Full EUC-KR conversion via `golang.org/x/text/encoding/korean` is deferred to a future SPEC.

**Resolution owner**: SPEC-IDX-003 author (Korean tokenization) likely owns this if it becomes painful at indexing time.

### 6.6 Per-Feed Healthcheck Granularity

**Question**: Should `Healthcheck(ctx)` probe every configured feed, or just one?

**Recommended default**: probe ONE representative endpoint per sub-source. For RSS, probe the FIRST configured feed URL (or `https://www.example.com:443` TCP fallback when feed list is empty). For KNC, probe `${USEARCH_ADP009_KNC_BASE_URL}/health` if KNC is enabled (else vacuous success). For Daum, vacuous success (always disabled). Aggregating per-feed health into a single boolean is sufficient for SPEC-EVAL-002 dashboard purposes.

**Resolution owner**: SPEC-EVAL-002 author (M8) may upgrade per-feed health surfacing if needed.

### 6.7 Score Normalization

**Question**: Should the composite use the Tanh formula from ADP-001/002?

**Recommended default**: NO — RSS items have no upvote/relevance signal. v0.1 sets `Score = 0.5` for ALL ADP-009 docs (mid-bucket placeholder). KNC items also lack signal. Daum is disabled. SPEC-IDX-001 RRF re-ranks by rank not raw score, so the precise Score value matters less than the bounded codomain `[0,1]`.

**Resolution owner**: SPEC-IDX-001 author may revisit if measured ranking quality suffers.

### 6.8 De-Duplication Across Sub-Sources

**Question**: Should the composite dedup an article that appears via both RSS AND KNC?

**Recommended default**: YES — dedup by canonical URL (mirror SPEC-FAN-001 §2.3 first-occurrence-wins). The composite's internal merge runs URL canonicalization across all three sub-source results before returning. This handles the case where a Korean newspaper publishes via RSS and that same article is later picked up by KNC scraping Naver portal.

**Resolution owner**: SPEC-FAN-001 already owns the cross-adapter dedup; ADP-009's intra-adapter dedup is a strict subset of the same algorithm. No separate decision needed.

---

## 7. Per-Sub-Source Feasibility Matrix

This matrix is the SPEC author's locked v0.1 scope:

| Sub-source | Library / Surface | License | Default | v0.1 Implementation | Future SPEC |
|------------|-------------------|---------|---------|---------------------|-------------|
| RSS feeds | `github.com/mmcdole/gofeed` v1.3.0 | MIT | **ENABLED** | Full implementation: env-supplied feed list (≤32 entries), per-feed parallel fetch via errgroup, 30s per-feed timeout, gofeed.Item → NormalizedDoc mapping per §4.3, per-feed error isolation | None (v0.1 complete) |
| KoreaNewsCrawler | `lumyjuwon/KoreaNewsCrawler` v1.51 (MIT) via Python sidecar at `services/koreanews/` | MIT | **DISABLED** | Go-side HTTP client to sidecar (`POST /search`); sidecar SCAFFOLD only (`pyproject.toml` + `Dockerfile` + stub FastAPI returning 503); Capabilities Notes warns scaffold-only | SPEC-ADP-009-KNC (post-M3) for full sidecar |
| Daum | `https://search.daum.net/` (no public API; robots.txt blocks all) | N/A (no public API) | **DISABLED** | Stub returns `ErrDaumDisabled` regardless of flag; flag plumbed for future SPEC; Capabilities Notes documents legal posture | SPEC-ADP-009-DAUM (legal review required) |

---

## 8. ToS / robots.txt Compliance Table

| Source | robots.txt URL | robots.txt verdict | ToS verdict | v0.1 Action |
|--------|---------------|---------------------|--------------|-------------|
| Daum | `https://search.daum.net/robots.txt` | `User-agent: * Disallow: /` (verified 2026-05-04) | Kakao does not publish a documented Search API; scraping requires explicit authorisation | DISABLED — stub returns ErrDaumDisabled |
| Naver portal (via KNC) | `https://news.naver.com/robots.txt` | Not verified for this SPEC; ADP-008 owns | Naver provides an official API at `openapi.naver.com` — KNC's portal-scraping bypasses the official path | DISABLED in v0.1; future SPEC-ADP-009-KNC requires legal review of portal-scraping when an official API exists |
| Operator-supplied RSS feeds | Each feed publisher's robots.txt | Not the adapter's responsibility (operator chooses feeds) | Each feed publisher's ToS — operator MUST own this decision | ENABLED; adapter Notes warn operator to review each feed's ToS before adding to the list |
| Reddit | (verified in SPEC-ADP-001 research) | (out of scope — not part of ADP-009) | (out of scope) | N/A |

---

## 9. Implementation Hints for the SPEC Author

### 9.1 SPEC Structure and Depth Calibration

Reference SPEC-ADP-001 (~1200 lines, 11 REQs) and SPEC-ADP-002 (~1190 lines, 10 REQs) for depth calibration. SPEC-ADP-009 should span **~700-900 lines** with **10-13 EARS REQs** because:
- Most of the per-sub-source machinery (HTTP client, redirect allowlist, parseRetryAfter, score normalizer) does NOT apply (RSS uses gofeed; KNC and Daum are disabled).
- The composite adapter pattern and sub-source dispatch are the novel content.
- Three new package-level sentinels (`ErrInvalidQuery`, `ErrDaumDisabled`, `ErrKNCSidecarDown`, `ErrEmptyRSSFeedList`).
- Configuration surface (3 enable flags + 1 feed list + 1 KNC base URL) is bigger than ADP-001/002.

### 9.2 EARS REQ Breakdown (Suggested for SPEC Author)

| REQ ID | Pattern | Topic |
|--------|---------|-------|
| REQ-ADP9-001 | Ubiquitous | Composite Adapter interface conformance + Capabilities determinism (SourceID="koreanews", DocTypes=[Article], SupportedLangs=["ko"]) |
| REQ-ADP9-002 | Event-Driven | Search dispatch fans to enabled sub-sources only; merges results; tags Metadata["subsource"] |
| REQ-ADP9-003 | Optional | RSS sub-source: parse env-supplied feed list, parallel fetch via errgroup, gofeed.Item → NormalizedDoc per §4.3, per-feed error isolation |
| REQ-ADP9-004 | Event-Driven | Per-feed HTTP error categorisation: 4xx → CategoryPermanent that-feed-only, 5xx/timeout → CategoryUnavailable that-feed-only, malformed XML → CategoryPermanent that-feed-only |
| REQ-ADP9-005 | Unwanted | Empty RSS feed list when RSS enabled OR empty Query.Text → return *SourceError{CategoryPermanent} immediately, zero HTTP requests |
| REQ-ADP9-006 | Unwanted | Daum sub-source: when USEARCH_ADP009_DAUM_ENABLED=true, return *SourceError{CategoryPermanent, Cause: ErrDaumDisabled}; document robots.txt evidence in Capabilities.Notes |
| REQ-ADP9-007 | Optional | KNC sub-source: HTTP POST to sidecar at USEARCH_ADP009_KNC_BASE_URL/search; on 503 stub response, return *SourceError{CategoryUnavailable, Cause: ErrKNCSidecarDown}; honour ctx |
| REQ-ADP9-008 | Ubiquitous | NormalizedDoc field mapping per §4.3 (RSS) and §2.3 (KNC, when sidecar present); Hash="", Metadata["subsource"] always set |
| REQ-ADP9-009 | Optional | Robots.txt + ToS compliance gate: when DAUM enabled, log a slog WARN at adapter init time; document in Capabilities.Notes |
| REQ-ADP9-010 | Ubiquitous | User-Agent + Accept headers on RSS HTTP requests (mirror ADP-001 REQ-ADP-009 — gofeed's transport accepts custom Client) |
| REQ-ADP9-011 | State-Driven | Concurrent-safety: 50 goroutines × 1 Search → race-clean (mirror ADP-001 REQ-ADP-011) |
| REQ-ADP9-012 | Event-Driven | Resource cleanup on ctx cancel: gofeed Parser honours ctx.Done; KNC sidecar HTTP client honours ctx.Done; no goroutine leaks (NFR-ADP9-003) |
| REQ-ADP9-013 | Optional | Korean locale heuristic: Hangul rune ratio ≥ 0.30 → Lang="ko"; else Lang="" (per OQ §6.1) |

This list is a recommendation; the SPEC author may collapse REQs if any feel redundant. Total of 12-13 REQs is appropriate.

### 9.3 NFRs

- **NFR-ADP9-001**: Performance — RSS parse path ≤ 10 ms p50 per feed (gofeed XML parsing is wider than Reddit JSON, so 10 ms vs ADP-001's 5 ms). Allocation budget ≤ 800 allocs/op (gofeed produces more strings than Reddit's `json.Unmarshal` due to XML namespace handling).
- **NFR-ADP9-002**: Race-clean concurrent invocation (50 goroutines × 1 Search).
- **NFR-ADP9-003**: Zero goroutine leaks on ctx cancel; `goleak.VerifyNone(t)` at end of every Search-invoking test.
- **NFR-ADP9-004**: End-to-end p95 ≤ 200 ms against `httptest.Server` stub (mirror ADP-001 NFR-ADP-002).

### 9.4 Test Plan

Total: ~40 tests covering REQ-ADP9-001..013 + NFRs. Layout:

- `internal/adapters/koreanews/koreanews_test.go` — Capabilities determinism, Adapter interface conformance, Healthcheck (5 tests)
- `internal/adapters/koreanews/search_test.go` — composite Search dispatch happy path, sub-source enable flag combinations, empty query rejection (8 tests)
- `internal/adapters/koreanews/rss_test.go` — gofeed parse table over fixtures (RSS 2.0, Atom, JSON Feed); per-feed parallel fetch; per-feed error isolation; canonical URL dedup; 30s per-feed timeout (15 tests)
- `internal/adapters/koreanews/daum_test.go` — Daum stub returns ErrDaumDisabled regardless of flag; ToS posture documented in Notes (3 tests)
- `internal/adapters/koreanews/knc_test.go` — KNC sidecar HTTP client; 503 → ErrKNCSidecarDown; sidecar happy path (mock httptest server) (5 tests)
- `internal/adapters/koreanews/locale_test.go` — Hangul ratio heuristic table (3 tests)
- `internal/adapters/koreanews/concurrent_test.go` — 50 goroutines race-clean (NFR-ADP9-002)
- `internal/adapters/koreanews/bench_test.go` — BenchmarkParseRSSFeed10Items + TestMain with goleak.VerifyTestMain (NFR-ADP9-001 + 003)

### 9.5 Package Layout

```
internal/adapters/koreanews/
├── koreanews.go                      # Adapter, New, Name, Capabilities, Healthcheck, interface assertion
├── koreanews_test.go                 # Interface conformance + Capabilities determinism
├── options.go                        # Options + sub-source enable flags + RSS feed list parsing
├── search.go                         # (*Adapter).Search composite dispatch
├── search_test.go                    # E2E + happy path tests
├── rss.go                            # RSS sub-source: gofeed parse, per-feed errgroup, NormalizedDoc transform
├── rss_test.go                       # gofeed parse table + per-feed isolation tests
├── daum.go                           # Daum stub: returns ErrDaumDisabled
├── daum_test.go                      # ToS posture verification
├── knc.go                            # KoreaNewsCrawler HTTP sidecar client
├── knc_test.go                       # KNC client happy path + 503 handling
├── locale.go                         # Hangul-ratio language detection
├── locale_test.go
├── strip.go                          # HTML strip helper for RSS Description bodies (mirror ADP-002)
├── strip_test.go
├── errors.go                         # ErrInvalidQuery, ErrDaumDisabled, ErrKNCSidecarDown, ErrEmptyRSSFeedList
├── concurrent_test.go                # NFR-ADP9-002 race workload
├── bench_test.go                     # BenchmarkParseRSSFeed10Items + TestMain (goleak)
└── testdata/
    ├── rss_2_0.xml                   # RSS 2.0 fixture (Korean publisher style)
    ├── atom_1_0.xml                  # Atom 1.0 fixture
    ├── json_feed_1.json              # JSON Feed fixture
    ├── rss_with_korean_text.xml      # Korean Hangul-heavy fixture for locale heuristic
    ├── rss_malformed.xml             # Truncated XML for parse-error path
    ├── knc_response.json             # Sidecar happy-path response
    └── knc_response_503.json         # Sidecar unavailable response
```

Plus `services/koreanews/` SCAFFOLD (Python sidecar; stub-only in v0.1):

```
services/koreanews/
├── Dockerfile                        # python:3.12-slim base
├── pyproject.toml                    # KoreaNewsCrawler==1.51, fastapi, uvicorn, pydantic v2
├── README.md                         # documents stub status + future SPEC ref
├── src/
│   └── main.py                       # FastAPI stub endpoint returning 503
└── tests/
    └── test_stub.py                  # asserts stub returns 503
```

### 9.6 MX Tag Plan

| File::Symbol | Tag | Reason |
|--------------|-----|--------|
| `koreanews.go::(*Adapter).Search` (defined in `search.go`) | `@MX:ANCHOR` | Sole entry point; fan_in ≥ 3 (registry, FAN-001 fanout, tests). `@MX:REASON: contract boundary; signature change ripples to FAN-001 + CLI-001`. |
| `rss.go::parseFeed` | `@MX:ANCHOR` | Every RSS doc passes through this transform. fan_in = 1 (Search-internal) but invariant-bearing. `@MX:REASON: NormalizedDoc field-mapping integrity gate for RSS sub-source`. |
| `rss.go::fetchAllFeeds` | `@MX:WARN` | Outbound fan-out spawns N goroutines (one per feed). `@MX:REASON: errgroup pattern + per-feed error isolation must not regress; goroutine leak risk on ctx cancel`. |
| `daum.go::(*Adapter).searchDaum` | `@MX:WARN` | Stub returning ErrDaumDisabled. `@MX:REASON: enabling without future SPEC + legal review violates robots.txt`. |
| `knc.go::callSidecar` | `@MX:NOTE` | HTTP POST to Python sidecar; sidecar URL configurable. Future SPEC-ADP-009-KNC replaces stub. |
| `errors.go::ErrDaumDisabled` | `@MX:NOTE` | Sentinel documents the legal posture. |
| `locale.go::detectKorean` | `@MX:NOTE` | Heuristic: Hangul rune ratio threshold 0.30. Open Question §6.1 documents revisit triggers. |

All tags `[AUTO]`-prefixed, `@MX:SPEC: SPEC-ADP-009`, `code_comments: en` per `.moai/config/sections/language.yaml`. Within per-file ANCHOR/WARN limits.

### 9.7 Harness Level

11-13 EARS REQs (mostly P0/P1) + 4 NFRs touching 1 package (≤14 source files including testdata) + 1 services scaffold + zero security-critical paths (no payments, no PII at the adapter layer beyond what RSS publishers already expose) = **standard** harness level. Sprint Contract optional.

---

## 10. Sources and Citations

### External URLs (WebFetch verified 2026-05-04)

- `https://github.com/lumyjuwon/KoreaNewsCrawler` — KoreaNewsCrawler library (MIT, v1.51 released 2022-03-27, unmaintained, scrapes Naver portal).
- `https://github.com/lumyjuwon/KoreaNewsCrawler/blob/master/README.md` — README extract: ArticleCrawler API, supported categories, output CSV schema.
- `https://github.com/mmcdole/gofeed` — gofeed Go library (MIT, v1.3.0 released 2024-03-01, supports RSS 0.90-2.0 / Atom 0.3-1.0 / JSON Feed 1.0-1.1).
- `https://search.daum.net/robots.txt` — Daum robots.txt: `User-agent: *\nDisallow: /` (full-domain disallow).

### External URLs (referenced but not WebFetch'd in this session)

- `https://www.rss-specifications.com/rss-specifications.htm` — RSS 2.0 specification reference (canonical authority on the spec; gofeed's behaviour is consistent with it per gofeed's own docs).
- `https://hn.algolia.com/api` — referenced in SPEC-ADP-002 §12; not fetched here as out of scope.

### Internal Files (file:line cited)

- `.moai/specs/SPEC-CORE-001/spec.md` — Adapter / Capabilities / Query / NormalizedDoc / SourceError contract.
- `.moai/specs/SPEC-OBS-001/spec.md` — observability bundle and cardinality discipline.
- `.moai/specs/SPEC-IR-001/spec.md` — `Capabilities` consumer contract (REQ-IR-008).
- `.moai/specs/SPEC-ADP-001/spec.md` — Reference adapter shape (Reddit) — file layout, MX tag plan, TDD harness, error mapping discipline.
- `.moai/specs/SPEC-ADP-001/research.md` — Research depth calibration reference.
- `.moai/specs/SPEC-ADP-002/spec.md` — Second-shape duplication (Hacker News). HTML-strip helper precedent.
- `.moai/specs/SPEC-FAN-001/spec.md` — Multi-source fanout. URL canonicalization rules at §2.4 reused for intra-adapter dedup.
- `pkg/types/adapter.go:28-45` — Adapter interface contract.
- `pkg/types/capabilities.go:38-62` — Capabilities struct + DocType enum.
- `pkg/types/query.go:18-44` — Query struct + Filter shape.
- `pkg/types/errors.go:14-218` — SourceError, Category enum, OutcomeFromError.
- `pkg/types/normalized_doc.go:40-106` — NormalizedDoc 15-field struct, Validate, CanonicalHash.
- `internal/adapters/registry.go:75-167` — Registry lifecycle.
- `internal/adapters/registry.go:172-263` — wrappedAdapter sole-emitter pattern.
- `internal/adapters/noop/noop.go:1-46` — reference noop shape + compile-time interface assertion.
- `internal/adapters/reddit/reddit.go:1-136` — Adapter struct pattern (mirrored by ADP-009 koreanews.go).
- `internal/adapters/hn/` — second-shape duplication reference.
- `services/researcher/` — Python sidecar precedent (gpt-researcher wrapper, FastAPI + Pydantic v2).
- `services/embedder/` — second Python sidecar precedent.
- `.moai/project/roadmap.md:54` — M3 row for SPEC-ADP-009.
- `.moai/project/roadmap.md:122-123` — M3 parallelization gate on SPEC-FAN-001.
- `.moai/project/roadmap.md:150` — M3 exit criterion ("Korean query returns Naver results ranked first" — primary covered by ADP-008; ADP-009 supplements breadth).
- `.moai/project/structure.md:18-22` — `internal/adapters/daum/`, `rss_korean/` reservations consolidated into `koreanews/`.
- `.moai/project/tech.md:117-118` — Per-source adapter strategy: Daum/KNC scraper-style; RSS gofeed.
- `.moai/config/sections/quality.yaml` — `development_mode: tdd`, `test_coverage_target: 85`.
- `.moai/config/sections/harness.yaml` — auto-routing to standard level.
- `.moai/config/sections/language.yaml` — `documentation: en`, `code_comments: en`.
- `go.mod` — Go 1.25.8; existing dependencies do not yet include `github.com/mmcdole/gofeed`. Run-phase will add via `go get github.com/mmcdole/gofeed/v2@v1.3.0`.

---

End of Research Document.

**Summary for SPEC Author**: SPEC-ADP-009 is a COMPOSITE adapter at `internal/adapters/koreanews/` exposing one `Adapter` contract that internally dispatches to up to three sub-sources (RSS via gofeed, KoreaNewsCrawler via Python sidecar, Daum via best-effort) gated by individual `USEARCH_ADP009_*_ENABLED` env flags. v0.1 ships RSS as DEFAULT-ON (full implementation), KNC as DEFAULT-OFF (Go HTTP client + sidecar SCAFFOLD only; full sidecar in future SPEC-ADP-009-KNC), and Daum as DEFAULT-OFF (stub returning ErrDaumDisabled regardless of flag; legal review required for any future SPEC-ADP-009-DAUM). Daum's robots.txt explicitly forbids all crawlers (`User-agent: *\nDisallow: /` verified 2026-05-04). KoreaNewsCrawler is MIT v1.51 unmaintained-since-2022-03-27; gofeed is MIT v1.3.0 actively maintained. The composite adapter follows the SPEC-ADP-001/002 reference shape verbatim — sole-emitter discipline, race-clean concurrent invocation, four error categories, errgroup-based per-feed parallelism with per-index slice state (mirroring SPEC-FAN-001 §2.6). Capabilities = `{SourceID: "koreanews", DocTypes: [Article], SupportedLangs: ["ko"], SupportsSince: false, RequiresAuth: false, RateLimitPerMin: 0, DefaultMaxResults: 50}`. Priority P1 (Korean breadth source; Naver-ADP-008 is the P0 primary). The SPEC should span 700-900 lines covering 12-13 EARS REQ-* items with ~40 tests targeting 85% coverage. Three new package-level sentinels, one Hangul-ratio locale heuristic, one HTML-strip helper duplicated from ADP-002. Zero new Prometheus metric families (sole-emitter discipline). Zero security/payment/PII paths at the adapter layer. Harness level: standard. Sprint Contract optional. Six Open Questions deferred to plan-auditor review and future SPECs.
