---
id: SPEC-ADP-009
title: KoreaNewsCrawler + Daum + Korean RSS Adapter
version: 0.1.0
milestone: M3 — Fanout, adapters, index
status: draft
priority: P1
owner: expert-backend
methodology: tdd
coverage_target: 85
created: 2026-05-04
updated: 2026-05-04
author: limbowl
issue_number: null
depends_on: [SPEC-CORE-001, SPEC-OBS-001, SPEC-IR-001]
blocks: []
---

# SPEC-ADP-009: KoreaNewsCrawler + Daum + Korean RSS Adapter

## HISTORY

- 2026-05-04 (initial draft v0.1, limbowl via manager-spec):
  Composite Korean-news adapter SPEC drafted after deep research
  (`.moai/specs/SPEC-ADP-009/research.md`, every external claim
  WebFetch-verified or URL-cited; every internal claim file:line-cited).
  Built on SPEC-CORE-001 (`pkg/types.Adapter` 4-method interface,
  `pkg/types.Capabilities` descriptor, `pkg/types.Query`,
  `pkg/types.NormalizedDoc` 15-field struct, `*types.SourceError`
  taxonomy, registry wrappedAdapter sole-emitter pattern at
  `internal/adapters/registry.go:172-263`), SPEC-OBS-001
  (`AdapterCalls{adapter,outcome}` and `AdapterCallDuration{adapter}`
  collectors with `adapter` already in cardinality allowlist), and
  SPEC-IR-001 (`Capabilities.SupportedLangs=["ko"]` consumer
  contract — `korean` Category routing). Soft sibling: SPEC-FAN-001
  (M3 gateway; URL-canonicalization dedup rules at §2.4 reused for
  intra-adapter sub-source dedup). Pattern reference: SPEC-ADP-001
  (Reddit reference shape) + SPEC-ADP-002 (HN second-shape, HTML-strip
  helper precedent).

  User-locked architectural decisions baked in:

  - **D1 Composite over three separate adapters**: ONE adapter at
    `internal/adapters/koreanews/` exposing the four-method contract;
    internally dispatches to up to three sub-sources (RSS, KNC, Daum)
    gated by individual env flags. Cardinality discipline (one
    `adapter="koreanews"` Prometheus label vs three) and routing
    simplicity drive the decision. `Metadata["subsource"]` carries
    actual sub-source attribution per-doc. `.moai/project/structure.md:18-22`
    reservations of `daum/` and `rss_korean/` directories are
    consolidated into the composite. Research §1.1.

  - **D2 Sub-source defaults**: RSS DEFAULT-ON (full v0.1
    implementation via gofeed v1.3.0); KNC DEFAULT-OFF (Go HTTP
    client + Python sidecar SCAFFOLD at `services/koreanews/`;
    full sidecar deferred to SPEC-ADP-009-KNC); Daum DEFAULT-OFF
    (stub returning `ErrDaumDisabled` regardless of env flag; full
    activation requires SPEC-ADP-009-DAUM with explicit legal
    review). The Daum default-off is a HARD constraint:
    `https://search.daum.net/robots.txt` returns
    `User-agent: *\nDisallow: /` (verified 2026-05-04 via WebFetch),
    forbidding all crawlers. Research §1.2 + §3.

  - **D3 RSS feed list configuration surface**: env var
    `USEARCH_ADP009_RSS_FEEDS` accepts JSON array OR comma-list of
    feed URLs (≤32 entries hard-capped). Feeds beyond 32 truncate
    with slog WARN. Empty list when RSS enabled returns
    `*SourceError{CategoryPermanent, Cause: ErrEmptyRSSFeedList}`.
    YAML file path (`USEARCH_ADP009_RSS_FEEDS_FILE`) deferred to
    future SPEC-ADP-CFG-001 (horizontal config concern across
    adapters, OQ §11.4). Research §4.2.

  - **D4 Per-feed error isolation**: each feed fetch is independent
    via `errgroup.SetLimit(min(8, len(feeds)))` (mirroring SPEC-FAN-001
    §2.5 + §2.6 verbatim — pre-allocated per-index `[][]NormalizedDoc`
    + `[]error` slices, no shared map writes; supervisor builds
    merged result post-Wait). One feed's 4xx / 5xx / malformed XML /
    timeout SHALL NOT cancel sibling feed fetches. Per-feed timeout
    default 30s, capped by `min(perFeedTimeout, time-until-parent-deadline)`.
    Research §4.4.

  - **D5 Score normalization deferred**: composite ADP-009 uses
    constant `Score = 0.5` for all returned docs (mid-bucket
    placeholder). RSS items have no upvote signal; KNC items have
    no relevance signal; Daum is disabled. SPEC-IDX-001 RRF re-ranks
    by rank not raw score, so the bounded `[0,1]` codomain
    discipline is what matters. Open Question §11.7 documents
    revisit triggers. Research §6.7.

  - **D6 Korean locale detection heuristic**: Hangul rune ratio
    (`unicode.Is(unicode.Hangul, r)` count divided by
    `len(title+body) runes`) ≥ 0.30 → `Lang="ko"`; else `Lang=""`.
    Handles operator-configured English tech-blog feeds alongside
    Korean newspaper feeds. Future SPEC-IDX-003 (Korean tokenization)
    may upgrade to a real language-detect library. Research §6.1.

  - **D7 Sole-emitter discipline (verbatim from ADP-001/002)**: zero
    Prometheus metrics, zero OTel spans, zero slog records emitted
    by the adapter; ALL observability flows through the registry's
    wrappedAdapter. The composite returns a correctly-categorised
    `*types.SourceError` so `OutcomeFromError(err)` produces the
    right `outcome` label.

  Sub-source feasibility matrix LOCKED (research §7):

  | Sub-source | Library | Default | v0.1 Implementation |
  |------------|---------|---------|---------------------|
  | RSS | `github.com/mmcdole/gofeed` v1.3.0 (MIT) | ENABLED | Full: env feed list, parallel fetch, gofeed.Item → NormalizedDoc, per-feed isolation |
  | KNC | `lumyjuwon/KoreaNewsCrawler` v1.51 (MIT, 2022-stale) via Python sidecar | DISABLED | Go HTTP client + sidecar SCAFFOLD (stub returns 503) |
  | Daum | `https://search.daum.net/` (no public API) | DISABLED | Stub returns ErrDaumDisabled regardless of flag |

  13 EARS REQs (8 × P0 + 5 × P1) covering all five EARS patterns
  (Ubiquitous via REQ-ADP9-001/008/010, Event-Driven via
  REQ-ADP9-002/004/007/012, State-Driven via REQ-ADP9-011 concurrency
  contract, Optional via REQ-ADP9-003/006/009/013, Unwanted via
  REQ-ADP9-005/006), 4 NFRs, ~40 representative TDD tests, 8 Open
  Questions carried forward from research.md §6 for plan-auditor
  challenge. Three new package-level sentinels (`ErrInvalidQuery`,
  `ErrDaumDisabled`, `ErrKNCSidecarDown`, `ErrEmptyRSSFeedList`),
  one Hangul-ratio locale heuristic, one HTML-strip helper
  duplicated from ADP-002.

  ONE new Go module dependency: `github.com/mmcdole/gofeed` v1.3.0
  (MIT, well-maintained, ~5 transitive deps). Run-phase adds via
  `go get github.com/mmcdole/gofeed@v1.3.0`. Otherwise pure stdlib
  (`net/http`, `encoding/json`, `time`, `context`, `errors`,
  `strings`, `strconv`, `net/url`, `unicode`, `unicode/utf8`,
  `sync`, `os`, `os/exec` — no exec actually used) plus existing
  `pkg/types`, `internal/obs/reqid`, `golang.org/x/sync/errgroup`.

  Zero new Prometheus metric families (sole-emitter discipline).
  Zero security/payment/PII paths at the adapter layer beyond what
  RSS publishers already expose. Harness level: standard
  (single domain, ≤14 source/test files in `internal/adapters/koreanews/`,
  +1 Python services scaffold at `services/koreanews/`, no
  security/payment/PII keywords beyond ToS posture documentation,
  zero compose/env/config deltas beyond the four
  `USEARCH_ADP009_*` env vars and one optional
  `.moai/config/sections/koreanews.yaml`). Sprint Contract optional
  but recommended given the legal-posture component for Daum.
  Ready for plan-auditor review and annotation cycle.

---

## 1. Purpose

The M3 milestone (`.moai/project/roadmap.md:43-58`) ships the full
adapter suite. SPEC-ADP-008 (Naver, parallel-planned) covers the
PRIMARY Korean-locale path via `isnow890/naver-search-mcp` with
API-key auth and a 25,000/day budget — that is the P0 source for
Korean queries. SPEC-ADP-009 (this SPEC) covers the FALLBACK
breadth path: historical archive (KoreaNewsCrawler), supplementary
search (Daum), and operator-configured RSS feeds (gofeed).

The composite adapter at `internal/adapters/koreanews/` is the
THIRD Korean-locale entry point in the M3 set:

1. **RSS path** (default-on): operators configure a list of Korean
   newspaper / blog RSS URLs via env. The adapter parses each feed
   in parallel (errgroup-bounded), maps `gofeed.Item` to
   `pkg/types.NormalizedDoc`, and merges results. This covers
   small-press Korean publishers, internal corp feeds, and any
   operator-curated Korean source not on Naver portal.
2. **KoreaNewsCrawler path** (default-off; SCAFFOLD only): a future
   SPEC-ADP-009-KNC may complete the Python sidecar at
   `services/koreanews/` exposing `POST /search` over the
   `lumyjuwon/KoreaNewsCrawler` v1.51 library. v0.1 ships the Go
   HTTP client + sidecar Dockerfile + stub FastAPI handler returning
   503; operators who need historical archive scraping implement
   the handler against the documented contract.
3. **Daum path** (default-off; STUB only): legal posture forbids
   v0.1 from shipping a working scraper. `https://search.daum.net/robots.txt`
   returns `User-agent: *\nDisallow: /` (verified 2026-05-04 via
   WebFetch). The stub returns `ErrDaumDisabled` regardless of the
   env flag; future SPEC-ADP-009-DAUM may unlock the path with
   explicit Kakao authorisation OR operator-attested compliance
   review.

Why a single composite over three separate adapters:

1. **Cardinality discipline**: SPEC-OBS-001 caps the `adapter` label
   values; three Korean sub-sources at P1 priority do not justify
   three label slots when one composite suffices. Naver (ADP-008)
   gets its own slot because it is the P0 primary.
2. **Routing simplicity**: IR-001's `korean` Category routes to
   ALL Korean adapters by `Capabilities.SupportedLangs`. With one
   composite, the Category fans out to N=2 adapters (Naver +
   koreanews-composite); with three separate, N=4. The composite
   reduces fanout overhead and keeps the user-facing SourceID
   stable.
3. **Operational clarity**: the three sub-sources share a single
   config block, single failure mode, single Healthcheck endpoint.
   Operators tune one adapter, not three.
4. **Future portability**: if KNC's sidecar is later promoted to
   a separate SPEC, the composite de-merges cleanly because the
   three internal paths are already separated by `subsource` enum.

The adapter, like ADP-001/002, does NOT do fanout (SPEC-FAN-001 owns
goroutine dispatch across adapters), does NOT do retry (SPEC-FAN-001
owns retry orchestration), does NOT do response caching
(SPEC-CACHE-001 owns the 5-phase fallback for blocked sources),
does NOT do ranking fusion (SPEC-IDX-001 owns RRF), and does NOT
emit any metric/log/span itself (the registry wrappedAdapter does,
sole-emitter discipline preserved). It DOES one job: turn a
`types.Query` into a fan-out across the enabled sub-sources, parse
each response, merge / dedup / sort the docs, and return
`[]types.NormalizedDoc` or `*types.SourceError`.

This SPEC is the THIRD in the seven-way M3 ADP-* parallelization
gated on SPEC-FAN-001 (`.moai/project/roadmap.md:122-123`).
Completion contributes to the M3 exit criterion
(`.moai/project/roadmap.md:150` — "Korean query returns Naver
results ranked first; ADP-009 supplements breadth").

---

## 2. Scope

### 2.1 In-Scope

| Ref | Item |
|-----|------|
| a | `internal/adapters/koreanews/koreanews.go`: `Adapter` struct (HTTP clients per sub-source + base URLs + user-agent + sub-source enable flags + RSS feed list + healthcheck target), `New(opts Options) (*Adapter, error)` constructor, `Name() string` returning `"koreanews"`, `Capabilities() types.Capabilities` returning a deterministic descriptor (RequiresAuth=false, AuthEnvVars=nil, DocTypes=[DocTypeArticle], SupportedLangs=["ko"], SupportsSince=false, RateLimitPerMin=0, DefaultMaxResults=50, DisplayName="Korean News (RSS + KoreaNewsCrawler + Daum)", Notes documenting enabled sub-sources, RSS feed count, ToS posture for Daum, KNC sidecar status), and `Healthcheck(ctx) error`. Compile-time interface assertion `var _ types.Adapter = (*Adapter)(nil)` at the bottom. |
| b | `internal/adapters/koreanews/options.go`: `Options{RSSEnabled bool, RSSFeeds []string, RSSPerFeedTimeout time.Duration, DaumEnabled bool, KNCEnabled bool, KNCBaseURL string, MaxParallelFeeds int, HTTPClient *http.Client, UserAgentVersion string, HealthcheckTarget string}` with documented zero-value defaults (`RSSEnabled=true`, `DaumEnabled=false`, `KNCEnabled=false`, `MaxParallelFeeds=8`, `RSSPerFeedTimeout=30s`, `KNCBaseURL="http://localhost:8002"`) and validation in `New`. Env-var loader helpers parse `USEARCH_ADP009_RSS_ENABLED`, `USEARCH_ADP009_RSS_FEEDS` (JSON array OR comma-list, ≤32 entries enforced), `USEARCH_ADP009_DAUM_ENABLED`, `USEARCH_ADP009_KNC_ENABLED`, `USEARCH_ADP009_KNC_BASE_URL`. |
| c | `internal/adapters/koreanews/search.go`: `(*Adapter).Search(ctx, q types.Query) ([]types.NormalizedDoc, error)` — the composite hot path. Validates the query (rejects empty/whitespace per REQ-ADP9-005), dispatches to enabled sub-sources via internal errgroup (one worker per enabled sub-source), merges per-sub-source `[]NormalizedDoc` slices, deduplicates by canonical URL (mirroring SPEC-FAN-001 §2.4 8 rules), sorts by `PublishedAt` descending then `SourceID` ascending, returns `[]NormalizedDoc` or composite `*SourceError`. Honours `ctx` cancellation throughout. |
| d | `internal/adapters/koreanews/rss.go`: RSS sub-source. `searchRSS(ctx, q types.Query) ([]types.NormalizedDoc, []error)` — fans the configured RSS feed URLs across `errgroup.SetLimit(opts.MaxParallelFeeds)`. Each per-feed worker derives a per-feed ctx (`min(opts.RSSPerFeedTimeout, time-until-parent-deadline)`), invokes `gofeed.NewParser().ParseURLWithContext(perFeedCtx, feedURL)`, transforms `gofeed.Item` per §6.3 mapping table, returns docs + per-feed error. Per-feed errors are isolated (one feed's failure does not cancel siblings). After `eg.Wait()`, the RSS sub-source returns the merged docs and a per-feed-index error slice. |
| e | `internal/adapters/koreanews/daum.go`: Daum sub-source STUB. `searchDaum(ctx, q types.Query) ([]types.NormalizedDoc, error)` — when `opts.DaumEnabled == true`, returns `(nil, &types.SourceError{Adapter:"koreanews", Category: types.CategoryPermanent, Cause: ErrDaumDisabled, Notes: "subsource: daum"})` immediately; when `opts.DaumEnabled == false`, returns `(nil, nil)` (no-op). The stub deliberately ignores the env flag's true state at the implementation level — the flag is plumbed for future SPEC-ADP-009-DAUM consumption only. |
| f | `internal/adapters/koreanews/knc.go`: KoreaNewsCrawler sub-source. `searchKNC(ctx, q types.Query) ([]types.NormalizedDoc, error)` — when `opts.KNCEnabled == true`, issues HTTP POST to `${opts.KNCBaseURL}/search` with JSON body `{query, max_results}`. On HTTP 503 (sidecar stub default), returns `*SourceError{CategoryUnavailable, Cause: ErrKNCSidecarDown}`. On HTTP 200, decodes the JSON response and maps each article per §6.3 KNC mapping. When `opts.KNCEnabled == false`, returns `(nil, nil)`. |
| g | `internal/adapters/koreanews/locale.go`: `detectKorean(text string) string` — Hangul ratio heuristic. Counts runes where `unicode.Is(unicode.Hangul, r)` is true, divided by total non-whitespace runes; if ratio ≥ 0.30, returns `"ko"`; else returns `""`. Pure function. |
| h | `internal/adapters/koreanews/strip.go`: `stripHTML(s string) string` — duplicated verbatim from SPEC-ADP-002 §6.4 helper. Conservative stdlib-only tag-strip + entity-decode. NOT a security boundary (output is plain text consumed by synthesis, never rendered as HTML). The duplication is intentional; consolidation deferred per SPEC-ADP-002 §11.4 rule-of-three guidance. |
| i | `internal/adapters/koreanews/dedup.go`: `dedupDocs(docs []types.NormalizedDoc) ([]types.NormalizedDoc, int)` — internal sub-source dedup. Implements URL canonicalization (mirror SPEC-FAN-001 §2.4 8 rules verbatim) + `CanonicalHash` fallback for unparseable URLs. First-occurrence-wins. Returns deduped slice + drop count. The intra-adapter dedup operates BEFORE the FAN-001 cross-adapter dedup; FAN-001 then de-dupes across `koreanews` + `naver` + `reddit` etc. |
| j | `internal/adapters/koreanews/errors.go`: package-private sentinels: `ErrInvalidQuery = errors.New("koreanews: query text empty or whitespace-only")`, `ErrDaumDisabled = errors.New("koreanews: daum subsource is disabled in v0.1 per robots.txt; enable via future SPEC-ADP-009-DAUM with legal review")`, `ErrKNCSidecarDown = errors.New("koreanews: knc sidecar unreachable (default port 8002)")`, `ErrEmptyRSSFeedList = errors.New("koreanews: rss enabled but no feed URLs configured (set USEARCH_ADP009_RSS_FEEDS)")`. |
| k | `internal/adapters/koreanews/koreanews_test.go`: tests for Adapter interface conformance (`var _ types.Adapter` compile-time assertion), `Name()` returns `"koreanews"`, `Capabilities()` returns deterministic value (called twice; equal), `Healthcheck()` succeeds against a stub `httptest.Server`, `New()` validates options. |
| l | `internal/adapters/koreanews/search_test.go`: composite Search dispatch tests. Sub-source enable flag combinations (RSS only, RSS+KNC, all three, none enabled), happy path (3 RSS feeds → 25 merged docs), empty Query.Text rejection (no HTTP requests issued), empty RSS feed list when RSS enabled rejection, ctx cancellation mid-fanout. |
| m | `internal/adapters/koreanews/rss_test.go`: RSS sub-source unit + integration tests. `gofeed` parse over 5 fixtures (RSS 2.0 Korean publisher style, Atom 1.0, JSON Feed 1.1, malformed XML, empty feed). Per-feed parallel fetch via errgroup, per-feed timeout (`RSSPerFeedTimeout=200ms`; one feed sleeps 1s → that feed times out, siblings succeed). Per-feed error isolation (4xx, 5xx, malformed). Hangul-ratio locale detection on representative items. Pagination NOT supported in v0.1 (RSS feeds don't have query-time pagination). |
| n | `internal/adapters/koreanews/daum_test.go`: Daum stub returns `ErrDaumDisabled` regardless of `Options.DaumEnabled` value (table test over both). Capabilities `Notes` substring assertion. |
| o | `internal/adapters/koreanews/knc_test.go`: KNC sidecar HTTP client. Stub `httptest.Server` returning 503 → `ErrKNCSidecarDown`. Stub returning 200 with JSON → decoded NormalizedDocs. Stub returning 4xx → `*SourceError{CategoryPermanent}`. Stub returning 5xx → `*SourceError{CategoryUnavailable}`. ctx cancellation mid-flight. |
| p | `internal/adapters/koreanews/locale_test.go`: `detectKorean` table over 6 inputs (pure Korean ≥ 0.30 → "ko"; pure English → ""; mixed 50/50 Korean/English → "ko"; mixed 20/80 → ""; empty string → ""; whitespace-only → ""). |
| q | `internal/adapters/koreanews/strip_test.go`: `stripHTML` table over 8 inputs (duplicated from ADP-002 strip_test.go). |
| r | `internal/adapters/koreanews/dedup_test.go`: dedup table over 5 fixtures (same-URL different-content, tracking-param-stripped, hash fallback on unparseable URL, mixed valid/invalid URL key spaces, deterministic byte-equal output). |
| s | `internal/adapters/koreanews/concurrent_test.go`: NFR-ADP9-002 race-clean workload. 50 goroutines × 1 Search per `*Adapter` against stub feed servers. |
| t | `internal/adapters/koreanews/bench_test.go`: `BenchmarkParseRSSFeed10Items` (NFR-ADP9-001). `TestMain` calls `goleak.VerifyTestMain(m)` (NFR-ADP9-003). |
| u | `internal/adapters/koreanews/testdata/`: 5 RSS/Atom/JSON Feed golden fixtures + 1 KNC sidecar JSON response fixture. |
| v | `services/koreanews/` SCAFFOLD (Python sidecar; STUB only in v0.1): `Dockerfile` (python:3.12-slim base), `pyproject.toml` (pins KoreaNewsCrawler==1.51, fastapi, uvicorn, pydantic v2), `src/main.py` (FastAPI stub returning 503 with `{detail: "knc sidecar not yet implemented (see SPEC-ADP-009-KNC)"}`), `tests/test_stub.py` (asserts stub returns 503), `README.md` (documents scaffold status + future SPEC ref + HTTP contract). |

### 2.2 Out-of-Scope

[HARD] This SPEC explicitly excludes the following items. Each has a
known destination SPEC; this list prevents scope creep into ADP-009
(the composite Korean-fallback adapter).

- **Per-source customisations specific to other sources** (Reddit
  internals, HN internals, arXiv, GitHub, YouTube, Bluesky, X,
  SearXNG, Naver, Polymarket) → SPEC-ADP-001/002/003/004/005/006/
  007/008.
- **Naver (primary Korean source)** → SPEC-ADP-008. ADP-009 is the
  fallback breadth path; ADP-008 is the P0 primary.
- **Retry orchestration** (exponential backoff, circuit breaker,
  per-feed retry budget) → SPEC-FAN-001 (M3) — owns retry. v0.1
  returns one categorised error per request and does not retry.
- **Response caching** (in-process LRU, Redis-backed, on-disk
  fixture cache) → SPEC-CACHE-001 (M3). Each `Search` is
  independent and idempotent.
- **Result ranking fusion across adapters** (Reciprocal Rank
  Fusion) → SPEC-IDX-001 (M3). The composite returns docs ordered
  by `PublishedAt` descending; cross-adapter ranking is fusion's
  job.
- **KoreaNewsCrawler full sidecar implementation** (Python FastAPI
  handler that actually wraps `ArticleCrawler.start()` and
  intercepts CSV output) → future SPEC-ADP-009-KNC after legal
  review of Naver-portal scraping. v0.1 ships the Go HTTP client
  + Python sidecar SCAFFOLD only (Dockerfile + pyproject.toml +
  stub returning 503).
- **Daum search activation** (any working scraping path against
  `search.daum.net`) → future SPEC-ADP-009-DAUM with explicit
  Kakao authorisation OR operator-attested compliance review.
  Daum's robots.txt explicitly forbids all crawlers (verified
  2026-05-04 via WebFetch). The flag is plumbed for future
  consumption only.
- **YAML file path for RSS feed list** (`USEARCH_ADP009_RSS_FEEDS_FILE`
  pointing to a YAML config) → future SPEC-ADP-CFG-001 (horizontal
  config concern across adapters). v0.1 supports only env-var
  JSON / comma-list.
- **EUC-KR or other non-UTF-8 encoding conversion** for legacy
  Korean RSS feeds → future SPEC. v0.1 honours the XML
  `encoding=` declaration via `gofeed`'s underlying `goxpp`
  parser; non-UTF-8 byte sequences after `unicode/utf8.ValidString`
  fail get a `Metadata["encoding_warning"]` annotation but no
  conversion attempt.
- **Per-adapter custom Prometheus metrics** (e.g.,
  `koreanews_feeds_fetched_total`) → would require amending
  SPEC-OBS-001's allowlist. Out of v0.1; the shared
  `AdapterCalls{adapter="koreanews",outcome}` family is sufficient.
- **Real language-detect library** (instead of Hangul-ratio
  heuristic) → SPEC-IDX-003 (Korean tokenization, M3) may upgrade
  if heuristic produces false-positives.
- **Pagination support for RSS feeds** — RSS feeds do not have
  query-time pagination; the entire feed is returned per request.
  `Query.Cursor` is ignored by the RSS sub-source. KNC sidecar
  may support pagination in a future SPEC.
- **Per-feed health monitoring** (granular per-feed availability
  dashboard) → SPEC-EVAL-002 (M8). v0.1 Healthcheck probes ONE
  representative endpoint per enabled sub-source.
- **Live network integration tests in CI** → out of v0.1.
  `httptest.Server` + golden fixtures only. Optional env-gated
  live test (`-tags=integration` + `KOREANEWS_LIVE=1`) deferred.
- **Streaming/incremental result delivery from inside Search** →
  SPEC-SYN-004 (M4).
- **Cardinality allowlist amendment** — ZERO new label names; the
  shared `adapter` and `outcome` labels suffice.
- **Cross-adapter helper extraction** (sharing `stripHTML` between
  HN and ADP-009) — out of v0.1. Per SPEC-ADP-002 §11.4 "rule of
  three" guidance, consolidation waits until 3+ adapters use the
  helper.

### 2.3 Composite Dispatch Architecture

[HARD] `(*Adapter).Search(ctx, q)` is the single composite dispatch
function. The dispatch is deterministic and pure (input → output;
no global state mutation; no time except `time.Now()` for
`RetrievedAt`).

**Dispatch algorithm**:

1. Validate `q.Text`: empty or whitespace-only → return
   `*SourceError{CategoryPermanent, Cause: ErrInvalidQuery}` immediately
   with NO HTTP requests issued. (REQ-ADP9-005)
2. If `opts.RSSEnabled` and `len(opts.RSSFeeds) == 0`: return
   `*SourceError{CategoryPermanent, Cause: ErrEmptyRSSFeedList}`
   immediately with NO HTTP requests. (REQ-ADP9-005)
3. Build a list of enabled sub-source workers:
   - If `opts.RSSEnabled && len(opts.RSSFeeds) > 0`: append `searchRSS`.
   - If `opts.DaumEnabled`: append `searchDaum` (returns
     `ErrDaumDisabled` synchronously per stub).
   - If `opts.KNCEnabled`: append `searchKNC`.
4. If the enabled list is empty (all flags off): return
   `(nil, nil)` (success with zero docs; no error).
5. Spawn one goroutine per enabled sub-source via
   `errgroup.SetLimit(len(enabled))` (no shared state; each worker
   writes to its own pre-allocated index in `[][]NormalizedDoc` and
   `[]error` slices, mirroring SPEC-FAN-001 §2.6 verbatim).
6. After `eg.Wait()` returns: merge per-sub-source docs into a single
   slice; tag each doc's `Metadata["subsource"]` with the originating
   sub-source name (`"rss"`, `"daum"`, `"knc"`).
7. Apply intra-adapter dedup (`dedupDocs` per §6.3). Increment
   internal counter for the dropped count (exposed via slog at
   call boundary, NOT as a NormalizedDoc field — same pattern as
   SPEC-FAN-001 `Stats.DedupDropped`).
8. Sort by `PublishedAt` descending (newer first); tie-break by
   `SourceID` ascending (stable per `sort.SliceStable`); secondary
   tie-break by `RetrievedAt` descending. Mirror SPEC-FAN-001 §2.5
   tie-break philosophy.
9. If `len(docs) == 0` AND ALL sub-sources returned errors:
   collapse the per-sub-source errors into a single composite
   `*SourceError{Category: CategoryUnavailable, Cause: <first non-nil>}`
   and return `(nil, err)`. If at least one doc was returned (or
   no sub-source errored), return `(docs, nil)` with sub-source
   errors collapsed silently into slog WARN annotations.

**Note on FAN-001 vs ADP-009 dispatch scope**: The fanout layer
(SPEC-FAN-001) operates at the ADAPTER level
(`registry.Get("koreanews").Search(...)`). The intra-ADP-009
sub-source dispatch operates at the SUB-ADAPTER level inside the
composite. Both use the same errgroup + per-index-slice +
URL-canonicalization-dedup discipline; they live at different
scopes. Consumers never see the internal dispatch.

### 2.4 RSS Per-Feed Timeout Derivation

[HARD] The per-feed ctx is derived as:

```
feedDeadline = min(
    opts.RSSPerFeedTimeout,            // default 30s
    timeUntil(parentCtx.Deadline())    // remaining caller budget; ∞ if no parent deadline
)
feedCtx, cancel = context.WithTimeout(parentCtx, feedDeadline)
defer cancel()
```

Properties (mirror SPEC-FAN-001 §2.5 exactly):

- The PARENT ctx propagation is preserved: cancelling the parent
  cancels every per-feed ctx via Go context inheritance.
- The per-feed timeout NEVER exceeds the caller's budget.
- A caller with NO deadline gets the 30s floor.
- The `cancel` function MUST be called (via `defer cancel()`) to
  release timer resources.

Pre-launch ctx guard (mirror SPEC-FAN-001 §2.5 H18 fix): BEFORE
every `eg.Go` call, the supervisor SHALL check `ctx.Err()`. If the
parent ctx is already cancelled, the supervisor SHALL skip the
launch and pre-populate the per-index error slot with
`*SourceError{Adapter:"koreanews", Category: CategoryUnavailable,
Cause: ctx.Err(), Notes: "subsource: rss; pre-launch skip"}`.
This prevents the `errgroup.SetLimit` deadlock case.

---

## 3. EARS Requirements

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| REQ-ADP9-001 | Ubiquitous | The package `internal/adapters/koreanews` SHALL expose an `Adapter` struct that implements `pkg/types.Adapter` exactly: `Name() string` returning `"koreanews"`, `Search(ctx context.Context, q types.Query) ([]types.NormalizedDoc, error)`, `Healthcheck(ctx context.Context) error`, `Capabilities() types.Capabilities`. The package SHALL include a compile-time interface assertion `var _ types.Adapter = (*Adapter)(nil)`. `Capabilities()` SHALL be deterministic (two consecutive calls return equal values) with `SourceID="koreanews"`, `DisplayName="Korean News (RSS + KoreaNewsCrawler + Daum)"`, `DocTypes=[DocTypeArticle]`, `SupportedLangs=["ko"]`, `SupportsSince=false`, `RequiresAuth=false`, `AuthEnvVars=nil`, `RateLimitPerMin=0`, `DefaultMaxResults=50`, AND `Notes` containing the substrings `"composite adapter"`, `"rss"`, `"daum: disabled"`, and `"knc: disabled"` (when sub-source defaults apply). | P0 | `TestAdapterName`, `TestAdapterImplementsInterface` (compile-time), `TestCapabilitiesDeterministic`, `TestCapabilitiesShape` (asserts all 9 documented field values + Notes substring matches), `TestHealthcheckSucceeds` (stub `httptest.Server` injected via Options.HealthcheckTarget). All in `internal/adapters/koreanews/koreanews_test.go`. |
| REQ-ADP9-002 | Event-Driven | WHEN `(*Adapter).Search(ctx, q)` is invoked with a non-empty `q.Text`, the adapter SHALL build a list of enabled sub-source workers (RSS if `opts.RSSEnabled && len(opts.RSSFeeds) > 0`; Daum if `opts.DaumEnabled`; KNC if `opts.KNCEnabled`), spawn one goroutine per enabled sub-source via `errgroup.SetLimit(len(enabled))`, collect per-sub-source results, merge / dedup / sort docs per §2.3 dispatch algorithm, and return `(docs, nil)` when at least one doc is returned. Each returned doc SHALL have `Metadata["subsource"]` set to one of `"rss"` / `"daum"` / `"knc"`. The composite SHALL NOT cancel sibling sub-sources because of one sub-source's error (suppress-error idiom: workers return `nil` to errgroup even on internal error). | P0 | `TestSearchAllSubSourcesEnabled` (stub RSS + KNC sidecar + Daum disabled-by-default; assert `len(docs) > 0`, all docs have `Metadata["subsource"]` populated); `TestSearchOnlyRSSEnabled` (only RSSEnabled=true, KNCEnabled=DaumEnabled=false; assert all docs have `Metadata["subsource"]=="rss"`); `TestSearchAllSubSourcesDisabled` (all flags false; assert `(nil, nil)` returned with no errors); `TestSearchOneSubSourceFailsOthersSucceed` (RSS feed 4xx + KNC sidecar 200; assert RSS errors logged but composite returns KNC docs successfully). All in `search_test.go`. |
| REQ-ADP9-003 | Optional | WHERE `opts.RSSEnabled == true` AND `len(opts.RSSFeeds) > 0`, the RSS sub-source SHALL fan the configured feed URLs across `errgroup.SetLimit(opts.MaxParallelFeeds)` (default 8, capped by `len(opts.RSSFeeds)`), derive a per-feed ctx via `min(opts.RSSPerFeedTimeout, time-until-parent-deadline)`, invoke `gofeed.NewParser().ParseURLWithContext(perFeedCtx, feedURL)` for each, transform `gofeed.Item` per §6.3 RSS mapping, AND assemble the merged docs after `eg.Wait()`. Per-feed errors SHALL be isolated: one feed's 4xx / 5xx / timeout / malformed XML SHALL NOT cancel sibling feed fetches. Each per-feed worker SHALL write to its own pre-allocated index in `[][]NormalizedDoc` and `[]error` slices (NEVER directly to a shared map; per SPEC-FAN-001 §2.6). | P1 | `TestSearchRSSHappyPath3Feeds` (3 stub feed servers each returning 5 items; assert `len(docs) == 15`, all `Metadata["subsource"]=="rss"`); `TestSearchRSSPerFeedTimeoutIndependent` (3 feeds: feed1 returns 5 items at 100ms, feed2 sleeps 1s and exceeds `RSSPerFeedTimeout=200ms`, feed3 returns 5 items at 150ms; assert `len(docs) == 10`, total elapsed ≈ 200ms); `TestSearchRSSPerFeedErrorIsolation` (3 feeds: feed1 returns 200 with valid XML, feed2 returns 503, feed3 returns 200 with malformed XML; assert `len(docs) == 5` from feed1 only, two errors logged); `TestSearchRSSHonoursMaxParallelFeeds` (16 feeds, `MaxParallelFeeds=4`; instrument feed handlers with atomic inflight counter; assert `max(inflight) == 4`); `TestSearchRSSCanceledMidQueue` (5 feeds with `MaxParallelFeeds=2`; cancel parent ctx at 50ms; assert no deadlock and elapsed ≤ 100ms). All in `rss_test.go`. |
| REQ-ADP9-004 | Event-Driven | WHEN an RSS feed fetch returns HTTP 4xx (other than 429), the per-feed worker SHALL record `*SourceError{Adapter:"koreanews", Category: types.CategoryPermanent, HTTPStatus: <code>, Cause: <inner>, Notes: "subsource: rss; feed: <url>"}` for that feed only. WHEN HTTP 5xx OR network error (DNS, dial timeout, TLS handshake, read timeout) OR `context.DeadlineExceeded` from per-feed ctx, the per-feed worker SHALL record `*SourceError{Adapter:"koreanews", Category: types.CategoryUnavailable, HTTPStatus: <code or 0>, Cause: <inner>, Notes: "subsource: rss; feed: <url>"}`. WHEN `gofeed.NewParser().ParseURLWithContext` returns a non-`http.RoundTripError` (malformed XML, invalid feed format), the per-feed worker SHALL record `*SourceError{Adapter:"koreanews", Category: types.CategoryPermanent, HTTPStatus: 0, Cause: <inner>, Notes: "subsource: rss; feed: <url>; reason: malformed"}`. NO per-feed worker SHALL retry; SHALL NOT propagate errors that cancel sibling workers. | P0 | `TestSearchRSSHTTP4xx` (table over 401/403/404 stub responses; assert per-feed error is `*SourceError{CategoryPermanent}` with matching HTTPStatus); `TestSearchRSSHTTP5xx` (table over 500/502/503; assert `CategoryUnavailable`); `TestSearchRSSConnectionRefused` (feed server pre-closed; assert `CategoryUnavailable` with HTTPStatus=0); `TestSearchRSSMalformedXML` (stub returns `<bad>not <closed>`; assert `CategoryPermanent` with reason="malformed"); `TestSearchRSSPerFeedNoInternalRetry` (instrument feed server; assert exactly 1 request per feed). All in `rss_test.go`. |
| REQ-ADP9-005 | Unwanted | IF `q.Text` is empty OR contains only Unicode whitespace runes (per `unicode.IsSpace` over every rune), THEN the adapter SHALL return `(nil, &types.SourceError{Adapter:"koreanews", Category: types.CategoryPermanent, Cause: ErrInvalidQuery})` immediately AND SHALL NOT issue any HTTP request. IF `opts.RSSEnabled == true` AND `len(opts.RSSFeeds) == 0`, THEN the adapter SHALL return `(nil, &types.SourceError{Adapter:"koreanews", Category: types.CategoryPermanent, Cause: ErrEmptyRSSFeedList})` immediately AND SHALL NOT issue any HTTP request. The empty-query check happens BEFORE the empty-feed-list check (so an empty query is rejected even when feeds are misconfigured). | P0 | `TestSearchEmptyQueryRejectedNoHTTP` (table over `["", "   ", "\t\n  \r"]` for `q.Text`; for each asserts `errors.Is(err, types.ErrPermanent)` AND `errors.Is(err, ErrInvalidQuery)` AND assert all stub feed servers received zero requests); `TestSearchEmptyRSSFeedListRejected` (RSSEnabled=true, RSSFeeds=nil; assert `errors.Is(err, ErrEmptyRSSFeedList)` AND zero requests); `TestSearchEmptyQueryTakesPrecedence` (RSSFeeds=nil AND q.Text=""; assert error is ErrInvalidQuery, NOT ErrEmptyRSSFeedList). All in `search_test.go`. |
| REQ-ADP9-006 | Unwanted | IF `opts.DaumEnabled == true`, THEN the Daum sub-source SHALL return `(nil, &types.SourceError{Adapter:"koreanews", Category: types.CategoryPermanent, Cause: ErrDaumDisabled, Notes: "subsource: daum"})` regardless of any other state. The composite SHALL NOT issue any HTTP request to `search.daum.net`. The `Capabilities().Notes` field SHALL contain the substring `"daum: disabled"` (or `"daum: stub"`) AND a reference to the robots.txt evidence. The flag MAY be plumbed end-to-end through `Options` so future SPEC-ADP-009-DAUM can replace the stub function cleanly without an Options struct change; the env var `USEARCH_ADP009_DAUM_ENABLED` parses but its `true` value still routes to the stub in v0.1. | P0 | `TestSearchDaumStubReturnsErrDisabled` (Options.DaumEnabled=true; assert returned err is `*SourceError{CategoryPermanent}` with `errors.Is(err, ErrDaumDisabled)` AND no outbound HTTP request to any host containing `"daum"` (use `http.DefaultTransport` interceptor)); `TestSearchDaumDisabledNoOp` (Options.DaumEnabled=false; assert no error and no outbound HTTP request); `TestCapabilitiesNotesDocumentDaumStatus` (assert `Notes` substring contains `"daum: disabled"` or `"daum: stub"` AND `"robots.txt"`). In `daum_test.go` and `koreanews_test.go`. |
| REQ-ADP9-007 | Optional | WHERE `opts.KNCEnabled == true`, the KNC sub-source SHALL issue an HTTP POST to `${opts.KNCBaseURL}/search` (default `http://localhost:8002/search`) with JSON body `{query: q.Text, max_results: clamp(q.MaxResults, 1, 100)}` and `Content-Type: application/json` header, derive a request ctx via `min(30s, time-until-parent-deadline)`, decode the JSON response, and map each `articles[]` element per §6.3 KNC mapping table. WHEN the sidecar returns HTTP 503 (default stub state) OR connection refused, the KNC sub-source SHALL return `*SourceError{Adapter:"koreanews", Category: types.CategoryUnavailable, HTTPStatus: <code or 0>, Cause: ErrKNCSidecarDown, Notes: "subsource: knc; sidecar: <url>"}`. WHEN HTTP 4xx, return `*SourceError{CategoryPermanent}`. WHEN HTTP 5xx (other than 503), return `*SourceError{CategoryUnavailable}`. WHEN HTTP 200 with malformed JSON, return `*SourceError{CategoryPermanent}` with reason="malformed". | P1 | `TestSearchKNCSidecarStub503` (stub httptest.Server returns 503; assert `errors.Is(err, ErrKNCSidecarDown)` AND `errors.Is(err, types.ErrSourceUnavailable)`); `TestSearchKNCSidecarHappyPath` (stub returns 200 with `{"articles":[{...},{...}]}`; assert decoded NormalizedDocs returned with `Metadata["subsource"]=="knc"`, `Lang="ko"`, `DocType=DocTypeArticle`); `TestSearchKNCSidecarHTTP4xx` (table over 400/404; assert `CategoryPermanent`); `TestSearchKNCSidecarHTTP5xx` (502/504; assert `CategoryUnavailable`); `TestSearchKNCSidecarMalformedJSON` (stub returns 200 with `{`; assert `CategoryPermanent`); `TestSearchKNCDisabledNoOp` (Options.KNCEnabled=false; assert no outbound request). All in `knc_test.go`. |
| REQ-ADP9-008 | Ubiquitous | The adapter SHALL transform each RSS `gofeed.Item` per §6.3 RSS mapping table AND each KNC sidecar `articles[]` entry per §6.3 KNC mapping table. Every returned `NormalizedDoc` SHALL have: `SourceID="koreanews"`, `DocType=types.DocTypeArticle`, `RetrievedAt=time.Now().UTC()` at parse time, `Hash=""` (consumers compute via `CanonicalHash()`), `Citations=nil`, AND `Metadata["subsource"]` set to `"rss"` / `"knc"` (NEVER `"daum"` in v0.1). RSS items SHALL additionally have `Metadata["feed_url"]` and `Metadata["feed_title"]`; KNC items SHALL additionally have `Metadata["category"]` and `Metadata["data_source"]="naver_portal"`. The `Score` field SHALL be set to `0.5` for ALL returned docs (mid-bucket placeholder; SPEC-IDX-001 RRF re-ranks by rank not raw score). The `Lang` field SHALL be set per REQ-ADP9-013 Korean locale heuristic. | P0 | `TestParseRSSFieldMapping` (table over 4 fixtures: RSS 2.0 Korean publisher, Atom 1.0, JSON Feed, RSS with empty author; assert every NormalizedDoc field per §6.3); `TestParseKNCFieldMapping` (KNC sidecar JSON fixture; assert every field); `TestAllReturnedDocsHaveSubsourceMetadata` (mixed RSS + KNC stub responses; assert every returned doc has non-empty `Metadata["subsource"]`); `TestAllReturnedDocsHaveScoreHalf` (assert every doc.Score == 0.5); `TestAllReturnedDocsHaveDocTypeArticle`; `TestAllReturnedDocsHashEmpty`. All in `rss_test.go` + `knc_test.go`. |
| REQ-ADP9-009 | Optional | WHERE `opts.DaumEnabled == true` is set at adapter construction time (via `New(opts)` OR via `USEARCH_ADP009_DAUM_ENABLED=true` env var), the constructor SHALL emit ONE slog WARN record at construction time (NOT per Search call) with attributes `{adapter:"koreanews", subsource:"daum", warning:"daum scraping is disabled by default per robots.txt; enabling without future SPEC-ADP-009-DAUM legal review violates ToS", robots_txt_evidence:"User-agent: * Disallow: /"}`. The warning SHALL fire EXACTLY ONCE per `*Adapter` instance lifetime, NOT per call. Additionally, the `Capabilities.Notes` substring SHALL include the robots.txt evidence URL `https://search.daum.net/robots.txt` so operators see the legal posture in observability dumps. | P1 | `TestNewLogsDaumWarningWhenEnabled` (capture slog JSON via custom handler; construct `New(Options{DaumEnabled: true, ...})`; assert exactly 1 WARN record with the documented attributes); `TestNewNoDaumWarningWhenDisabled` (Options.DaumEnabled=false; assert zero WARN records); `TestCapabilitiesNotesIncludesRobotsTxtURL` (assert `Notes` contains `"search.daum.net/robots.txt"`). In `koreanews_test.go`. |
| REQ-ADP9-010 | Ubiquitous | The adapter SHALL set the `User-Agent` HTTP header on every outbound RSS feed request AND every KNC sidecar request to a non-default value of the form `usearch/<version> (+https://github.com/elymas/universal-search)` where `<version>` is supplied via `Options.UserAgentVersion` (default `"v0.1"`). The adapter SHALL set the `Accept` header to `application/rss+xml, application/atom+xml, application/feed+json, application/json;q=0.9, */*;q=0.5` for RSS requests (covers RSS, Atom, JSON Feed, and graceful fallback) AND `application/json` for KNC sidecar requests. The custom UA preserves the project-wide convention from ADP-001 REQ-ADP-009 and identifies traffic for operational debugging at upstream feed servers. | P0 | `TestRSSFetchSetsCustomUserAgent` (stub feed server captures `r.Header.Get("User-Agent")`; assert it starts with `"usearch/"` and contains `"(+https://github.com/elymas/universal-search)"`); `TestRSSFetchSetsAcceptFeedTypes` (assert `Accept` header contains `"application/rss+xml"` AND `"application/atom+xml"` AND `"application/feed+json"`); `TestKNCSetsAcceptJSON` (assert KNC request `Accept: application/json`); `TestUserAgentVersionConfigurable` (Options.UserAgentVersion="v0.2-rc1" → headers contain `"usearch/v0.2-rc1"`). In `rss_test.go` + `knc_test.go`. |
| REQ-ADP9-011 | State-Driven | WHILE the same `*Adapter` instance is registered in the adapter registry and is being invoked concurrently from N goroutines (N ≥ 1), each `Search(ctx, q)` call SHALL execute independently with no shared mutable state across calls (the underlying `*http.Client`s for RSS and KNC are goroutine-safe per Go stdlib; the gofeed.Parser is immutable post-construction; the adapter holds no per-call state); the cumulative effect SHALL be N independent composite dispatches (each spawning M sub-source fan-outs internally) with no race-detector alarms. This requirement crystallises the concurrency contract that the registry (`internal/adapters/registry.go:172-263` wrappedAdapter) and the future fanout layer (SPEC-FAN-001) rely on. | P0 | `TestSearchConcurrentSafe` (50 caller goroutines × 1 Search per `*Adapter` against 3-feed stub server pool; assert (a) no race-detector alarm under `-race`, (b) every goroutine receives `(docs, nil)` with `len(docs) >= 1`, (c) every returned `[]types.NormalizedDoc` has every doc passing `Validate()` returning nil). In `concurrent_test.go`. |
| REQ-ADP9-012 | Event-Driven | WHEN the parent ctx is cancelled mid-Search (caller's deadline expires), the adapter SHALL collect partial results from any sub-source that completed prior to cancellation, SHALL record `context.Canceled` (or `context.DeadlineExceeded`) for sub-sources that did NOT complete, SHALL release all per-feed and per-sub-source goroutines via `defer cancel()` discipline, SHALL NOT leak any goroutine (verified by `goleak.VerifyNone(t)` per NFR-ADP9-003), AND SHALL return `(partial-docs, nil)` on partial success OR `(nil, *SourceError{CategoryUnavailable, Cause: ctx.Err()})` when no sub-source completed. The adapter SHALL honour `ctx` cancellation throughout: `gofeed.ParseURLWithContext` propagates ctx; the KNC HTTP client uses `http.NewRequestWithContext`; the daum stub is synchronous and immediate (no ctx required). | P0 | `TestSearchPartialResultsOnParentTimeout` (5 feeds: feed1+feed2 return 100ms, feed3+feed4+feed5 sleep 5s; parent ctx 500ms; assert `len(docs) == 10`, total elapsed ∈ [500ms, 800ms]); `TestSearchAlreadyCancelledCtx` (parent ctx pre-cancelled; assert `(nil, *SourceError{CategoryUnavailable})` AND zero outbound HTTP requests AND no goroutines spawned (NumGoroutine before/after delta within tolerance)); `TestSearchNoGoroutineLeakOnCancel` (`goleak.VerifyNone(t)` after a Search whose ctx is cancelled at 50ms while feeds delay 200ms). All in `search_test.go`. |
| REQ-ADP9-013 | Optional | WHERE the parsed `Title + Body` of an RSS / KNC item contains Korean Hangul runes such that the ratio (Hangul rune count / non-whitespace rune count) is ≥ 0.30, the adapter SHALL set `NormalizedDoc.Lang = "ko"`. WHERE the ratio is < 0.30 OR the combined text is empty/whitespace-only, the adapter SHALL set `NormalizedDoc.Lang = ""` (unknown). The detection is a stateless pure function (`detectKorean(text string) string`) implemented via `unicode.Is(unicode.Hangul, r)` over the rune sequence. KNC items unconditionally have `Lang="ko"` because the upstream library exclusively sources Korean newspapers; the heuristic is applied only as a defensive fallback. | P1 | `TestDetectKoreanTable` in `locale_test.go` (table over 6 inputs: pure Korean text 100% Hangul → "ko"; pure English → ""; 50/50 mixed → "ko"; 20/80 Korean/English → ""; empty string → ""; whitespace-only → ""); `TestRSSItemKoreanDetected` (gofeed.Item with Korean title+body → returned doc has `Lang="ko"`); `TestRSSItemEnglishNotDetected` (English-only item → `Lang=""`); `TestKNCItemAlwaysKorean` (KNC stub response with mixed-language item → `Lang="ko"` regardless of ratio, because KNC unconditionally tags). In `locale_test.go` + `rss_test.go` + `knc_test.go`. |

---

## 4. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| NFR-ADP9-001 | Performance (RSS parse path) | The RSS parse path `parseFeed(body []byte, retrievedAt time.Time, feedURL string) ([]NormalizedDoc, error)` SHALL execute with mean wall-clock duration per op ≤ 10 ms over `go test -bench=BenchmarkParseRSSFeed10Items -benchtime=10x -count=5 ./internal/adapters/koreanews/...` on amd64; the median of the 5 runs is the assertion value (passes when ≤ 10 ms). The fixture is the `rss_2_0.xml` golden (10-item RSS 2.0 Korean publisher style, ~8KB). The 10ms target (vs ADP-001's 5ms for Reddit JSON) accommodates gofeed's wider XML parsing surface (namespace handling, multi-format dispatch). Allocation count ≤ 800 per op (vs ADP-001's 500 ceiling for 25 docs; gofeed produces more strings than `json.Unmarshal` due to XML namespace expansion). The benchmark is run weekly in CI per the cadence established in SPEC-OBS-001 NFR-OBS-001. Benchmarks do not count toward coverage. |
| NFR-ADP9-002 | Race-clean concurrent invocation | `internal/adapters/koreanews/concurrent_test.go::TestSearchConcurrentSafe` SHALL execute successfully under `go test -race ./internal/adapters/koreanews/...` with 50 caller goroutines × 1 `Search` call each against a 3-feed stub server pool. Total: 150 RSS feed fetches + 0 KNC calls (default-off) + 0 Daum calls (always-off). Race-detector alarms attributable to the koreanews package SHALL be zero. |
| NFR-ADP9-003 | Zero goroutine leaks | The adapter SHALL NOT leak any goroutine when the caller's context is cancelled mid-`Search` OR when any sub-source returns. Verified by `TestSearchNoGoroutineLeakOnCancel` in `search_test.go` (using `go.uber.org/goleak.VerifyNone(t)` after a `Search` call whose ctx is cancelled mid-flight via a 50ms-delayed cancel; assert zero residual goroutines after the call returns). Additionally, `internal/adapters/koreanews/bench_test.go::TestMain` SHALL invoke `goleak.VerifyTestMain(m)` (mirrors `internal/adapters/reddit/bench_test.go` pattern). The adapter itself spawns only the bounded errgroup workers (capped at `len(enabled-sub-sources)` for the outer dispatch and `MaxParallelFeeds` for the inner RSS fan-out); no detached background goroutines are permitted. |
| NFR-ADP9-004 | End-to-end Latency (Stub) | The end-to-end `Search` round-trip against the `httptest.Server` stub pool (no real network) with RSS-only configuration (3 stub feeds returning 5 items each at 50ms latency) SHALL complete with p95 ≤ 200 ms over 100 invocations, measured by `TestSearchE2ELatencyStubP95` in `search_test.go` (sort durations ascending, assert `durations[94] ≤ 200ms`). The harder live-feed p95 is documented as the operational target (≤ 5s for combined RSS + KNC sidecar) but is NOT enforced in CI (no live network). |

---

## 5. Acceptance Criteria

### REQ-ADP9-001 — Adapter Interface Conformance

- File `internal/adapters/koreanews/koreanews.go` declares `Adapter` struct with the documented fields (`rssClient *http.Client`, `kncClient *http.Client`, `gofeedParser *gofeed.Parser`, `opts Options`, `userAgent string`, `healthcheckTarget string`).
- The compile-time assertion `var _ types.Adapter = (*Adapter)(nil)` appears at the bottom of `koreanews.go`. If the interface ever drifts, this assertion fails to compile.
- `(*Adapter).Name()` returns the literal string `"koreanews"`.
- `(*Adapter).Capabilities()` returns a `types.Capabilities` with all 9 documented field values matching exactly:
  - `SourceID = "koreanews"`
  - `DisplayName = "Korean News (RSS + KoreaNewsCrawler + Daum)"`
  - `DocTypes = []types.DocType{types.DocTypeArticle}`
  - `SupportedLangs = []string{"ko"}`
  - `SupportsSince = false`
  - `RequiresAuth = false`
  - `AuthEnvVars = nil`
  - `RateLimitPerMin = 0`
  - `DefaultMaxResults = 50`
  - `Notes` contains the substrings `"composite adapter"`, `"rss"`, `"daum: disabled"` (or `"daum: stub"`), `"knc: disabled"` (or `"knc: scaffold"`), AND `"search.daum.net/robots.txt"`.
- `(*Adapter).Healthcheck(ctx)` succeeds against an `httptest.Server` bound to `127.0.0.1:0`. Tests construct the Adapter with `Options{HealthcheckTarget: <httptest.Server URL>}`.
- `TestAdapterName`, `TestAdapterImplementsInterface`, `TestCapabilitiesDeterministic`, `TestCapabilitiesShape`, `TestHealthcheckSucceeds` all pass.

### REQ-ADP9-002 — Composite Search Dispatch

- `TestSearchAllSubSourcesEnabled`: 3 stub RSS feeds + KNC sidecar stub returning 200 with 2 articles + Daum disabled (always); assert `len(docs) >= 1`, every doc has non-empty `Metadata["subsource"]` value in `{"rss", "knc"}` (never `"daum"` in v0.1).
- `TestSearchOnlyRSSEnabled`: only `Options.RSSEnabled=true`, 3 stub feeds; assert `len(docs) == 15` (3 × 5), every doc has `Metadata["subsource"]=="rss"`.
- `TestSearchAllSubSourcesDisabled`: all flags false; assert `(nil, nil)` returned with `err == nil`.
- `TestSearchOneSubSourceFailsOthersSucceed`: RSS feed1 returns 4xx, RSS feed2 returns 200 with 5 docs, KNC sidecar returns 200 with 3 docs; assert `len(docs) == 8`, slog WARN logged for feed1, no error returned at composite level.

### REQ-ADP9-003 — RSS Sub-Source Fan-Out

- `TestSearchRSSHappyPath3Feeds`: 3 stub feed servers each returning 5 items (15 total unique URLs); assert `len(docs) == 15`, all `Metadata["subsource"]=="rss"`, all `DocType==DocTypeArticle`.
- `TestSearchRSSPerFeedTimeoutIndependent`: 3 feeds, `RSSPerFeedTimeout=200ms`; feed1 returns 5 items at 100ms, feed2 sleeps 1s (will time out at 200ms), feed3 returns 5 items at 150ms; assert `len(docs) == 10` (feed1+feed3), feed2 error is `*SourceError{CategoryUnavailable, Cause: <wraps context.DeadlineExceeded>}`, total elapsed ≈ 200ms.
- `TestSearchRSSPerFeedErrorIsolation`: 3 feeds: feed1 returns 200/valid, feed2 returns 503, feed3 returns 200/malformed XML; assert `len(docs) == 5` (feed1 only), exactly 2 per-feed errors logged via slog.
- `TestSearchRSSHonoursMaxParallelFeeds`: 16 stub feeds, `MaxParallelFeeds=4`; instrument feed handlers with `atomic.Int32` inflight counter; assert `max(inflight) == 4` exactly.
- `TestSearchRSSCanceledMidQueue`: 5 feeds with `MaxParallelFeeds=2`; cancel parent ctx at 50ms; assert no deadlock, total elapsed ≤ 100ms, `goleak.VerifyNone(t)` clean.

### REQ-ADP9-004 — Per-Feed HTTP Error Categorisation

- `TestSearchRSSHTTP4xx`: table over 401, 403, 404 stub responses; for each, the per-feed error is `*SourceError{CategoryPermanent}` with matching HTTPStatus and `Notes` substring containing `"subsource: rss"` AND the feed URL.
- `TestSearchRSSHTTP5xx`: table over 500, 502, 503; assert `*SourceError{CategoryUnavailable}` with matching HTTPStatus.
- `TestSearchRSSConnectionRefused`: feed server pre-closed before request; assert `*SourceError{CategoryUnavailable, HTTPStatus: 0}`.
- `TestSearchRSSMalformedXML`: stub returns 200 with truncated XML `<rss><chann`; assert `*SourceError{CategoryPermanent}` with reason substring `"malformed"`.
- `TestSearchRSSPerFeedNoInternalRetry`: instrument feed server with request counter; assert exactly 1 request observed per feed.

### REQ-ADP9-005 — Empty Query and Empty Feed List Rejection

- `TestSearchEmptyQueryRejectedNoHTTP`: table over `q.Text` in `["", "   ", "\t\n  \r"]`; for each: `errors.Is(err, types.ErrPermanent)` AND `errors.Is(err, ErrInvalidQuery)`; instrument all stub servers with request counters, assert all zero.
- `TestSearchEmptyRSSFeedListRejected`: `Options.RSSEnabled=true, Options.RSSFeeds=nil`, valid `q.Text="hello"`; assert `errors.Is(err, ErrEmptyRSSFeedList)` AND `errors.Is(err, types.ErrPermanent)`; assert zero outbound HTTP requests.
- `TestSearchEmptyQueryTakesPrecedence`: `q.Text="" AND Options.RSSFeeds=nil`; assert error is `ErrInvalidQuery`, NOT `ErrEmptyRSSFeedList`.
- `TestSearchEmptyFeedListAcceptableWhenRSSDisabled`: `Options.RSSEnabled=false, RSSFeeds=nil`, KNC enabled with stub returning 2 docs; assert `(docs, nil)` with `len(docs) == 2` (RSS empty list is fine when RSS is disabled).

### REQ-ADP9-006 — Daum Stub

- `TestSearchDaumStubReturnsErrDisabled`: `Options.DaumEnabled=true`, stub other sub-sources to return zero docs; assert returned err is `*SourceError{CategoryPermanent}` with `errors.Is(err, ErrDaumDisabled)`. Use a custom `http.RoundTripper` interceptor on the adapter's `httpClient` and assert NO outbound request to any host containing `"daum"`.
- `TestSearchDaumDisabledNoOp`: `Options.DaumEnabled=false`, RSS returns 5 docs; assert `(docs, nil)` with `len(docs) == 5` and no Daum-related error in slog WARN.
- `TestCapabilitiesNotesDocumentDaumStatus`: assert `Capabilities().Notes` contains the substrings `"daum: disabled"` (or `"daum: stub"`) AND `"robots.txt"` AND `"User-agent: *"` AND `"Disallow: /"` — exposing the legal posture verbatim.

### REQ-ADP9-007 — KNC Sidecar Client

- `TestSearchKNCSidecarStub503`: stub `httptest.Server` returns 503 with body `{"detail": "knc sidecar not yet implemented"}`; `Options.KNCEnabled=true, Options.KNCBaseURL=<stub URL>`; assert `errors.Is(err, ErrKNCSidecarDown)` AND `errors.Is(err, types.ErrSourceUnavailable)` AND `*SourceError.HTTPStatus == 503`.
- `TestSearchKNCSidecarHappyPath`: stub returns 200 with `{"articles":[{"url":"https://news.example.kr/1","title":"제목 1","body":"본문 1","date":"2026-05-01T10:00:00","author":"매체A","category":"politics"}]}`; assert exactly 1 NormalizedDoc returned with `Metadata["subsource"]=="knc"`, `Lang="ko"`, `DocType=DocTypeArticle`, `Author="매체A"`, `Metadata["category"]=="politics"`, `Metadata["data_source"]=="naver_portal"`.
- `TestSearchKNCSidecarHTTP4xx`: table over 400, 404; assert `*SourceError{CategoryPermanent, HTTPStatus: <code>}`.
- `TestSearchKNCSidecarHTTP5xx`: 502, 504; assert `*SourceError{CategoryUnavailable}`.
- `TestSearchKNCSidecarMalformedJSON`: stub returns 200 with truncated `{"articles":[`; assert `*SourceError{CategoryPermanent}` with reason substring `"malformed"`.
- `TestSearchKNCDisabledNoOp`: `Options.KNCEnabled=false`; assert no outbound HTTP request to KNC sidecar URL (intercept via `http.RoundTripper`).
- `TestSearchKNCRequestShape`: capture request; assert method=POST, path=`/search`, header `Content-Type: application/json`, header `Accept: application/json`, body matches `{"query":"<q.Text>","max_results":<clamped>}`.

### REQ-ADP9-008 — NormalizedDoc Field Mapping

- `TestParseRSSFieldMapping`: table over 4 fixtures (RSS 2.0 Korean publisher, Atom 1.0, JSON Feed, RSS with empty author); for each, every NormalizedDoc field per the §6.3 RSS mapping table.
- `TestParseKNCFieldMapping`: KNC stub fixture with mixed article shapes; every NormalizedDoc field per the §6.3 KNC mapping table.
- `TestAllReturnedDocsHaveSubsourceMetadata`: composite Search across mixed RSS + KNC stubs; assert every returned doc has `Metadata["subsource"] in {"rss", "knc"}` (NOT "daum"; NOT empty).
- `TestAllReturnedDocsHaveScoreHalf`: assert every `doc.Score == 0.5` exactly.
- `TestAllReturnedDocsHaveDocTypeArticle`: assert every `doc.DocType == types.DocTypeArticle`.
- `TestAllReturnedDocsHashEmpty`: assert every `doc.Hash == ""`.
- `TestAllReturnedDocsCitationsNil`: assert every `doc.Citations == nil`.
- `TestRSSDocsMetadataKeys`: assert every RSS-sub-source doc has Metadata keys `{subsource, feed_url, feed_title}` at minimum.
- `TestKNCDocsMetadataKeys`: assert every KNC-sub-source doc has Metadata keys `{subsource, category, data_source}` at minimum.

### REQ-ADP9-009 — Daum Warning at Construction

- `TestNewLogsDaumWarningWhenEnabled`: capture slog JSON via custom handler; construct `New(Options{DaumEnabled: true, RSSEnabled: false})`; assert exactly 1 WARN record with attributes `{adapter:"koreanews", subsource:"daum", warning:<contains "robots.txt">, robots_txt_evidence:<contains "Disallow: /">}`. Verify that subsequent `Search` calls do NOT emit additional WARN records (one-per-instance discipline).
- `TestNewNoDaumWarningWhenDisabled`: `Options.DaumEnabled=false`; assert zero WARN records during `New(...)` and during subsequent `Search` calls.
- `TestCapabilitiesNotesIncludesRobotsTxtURL`: assert `Capabilities().Notes` contains the literal substring `"https://search.daum.net/robots.txt"`.

### REQ-ADP9-010 — User-Agent and Accept Headers

- `TestRSSFetchSetsCustomUserAgent`: stub feed server captures `r.Header.Get("User-Agent")`; assert it starts with `"usearch/"` and contains `"(+https://github.com/elymas/universal-search)"`.
- `TestRSSFetchSetsAcceptFeedTypes`: assert `Accept` header contains all of `"application/rss+xml"`, `"application/atom+xml"`, `"application/feed+json"`.
- `TestKNCSetsAcceptJSON`: assert KNC request `Accept` header equals `"application/json"`.
- `TestKNCSetsContentTypeJSON`: assert KNC request `Content-Type` header equals `"application/json"`.
- `TestUserAgentVersionConfigurable`: `Options.UserAgentVersion="v0.2-rc1"`; assert both RSS and KNC request UA headers contain `"usearch/v0.2-rc1"`.

### REQ-ADP9-011 — Concurrent Search Safety (State-Driven)

- `TestSearchConcurrentSafe` in `concurrent_test.go`:
  - Construct one `*Adapter` with 3 stub feed servers + KNC stub returning 200/2-articles + Daum disabled.
  - Spawn 50 caller goroutines via `sync.WaitGroup` barrier.
  - Each goroutine performs 1 `Search(ctx, q)` call.
  - Total: 50 composite invocations × (3 RSS feeds + 1 KNC) = 200 outbound HTTP round-trips.
- Assertions:
  1. Test executes successfully under `go test -race ./internal/adapters/koreanews/...`; race-detector alarms attributable to the koreanews package = 0.
  2. Every goroutine receives `(docs, nil)` with `len(docs) >= 17` (3×5 RSS + 2 KNC, modulo intra-adapter dedup).
  3. Every returned `[]NormalizedDoc` has every doc passing `Validate()` returning nil.
  4. `goleak.VerifyNone(t)` at test end confirms zero residual goroutines.

### REQ-ADP9-012 — Partial Results on Cancellation

- `TestSearchPartialResultsOnParentTimeout`: 5 RSS feeds; feed1+feed2 return 100ms, feed3+feed4+feed5 sleep 5s; parent ctx 500ms; assert `(docs, nil)` with `len(docs) == 10`, total elapsed ∈ [500ms, 800ms]. Per-feed errors for feed3/4/5 logged via slog with `Cause: context.DeadlineExceeded`.
- `TestSearchAlreadyCancelledCtx`: pre-cancelled ctx; assert `(nil, *SourceError{CategoryUnavailable, Cause: context.Canceled})`. Use `runtime.NumGoroutine()` snapshots before/after; delta within race-detector tolerance (≤ 2). Zero outbound HTTP requests via interceptor check.
- `TestSearchNoGoroutineLeakOnCancel`: `goleak.VerifyNone(t)` after a Search whose ctx is cancelled at 50ms while feeds delay 200ms.

### REQ-ADP9-013 — Korean Locale Heuristic

- `TestDetectKoreanTable` in `locale_test.go`:
  - Pure Korean text "안녕하세요 한국어 텍스트 입니다" → `"ko"` (ratio = 1.0)
  - Pure English text "Hello world this is English" → `""` (ratio = 0.0)
  - 50/50 mixed "Hello 안녕 World 세상" → `"ko"` (ratio ≈ 0.50 ≥ 0.30)
  - Mostly-English with Korean accent "Big news 한국 today" → calculate: 2 Hangul / 11 non-whitespace ≈ 0.18 → `""`
  - Empty string `""` → `""`
  - Whitespace-only `"   "` → `""`
  - Korean-with-numbers "한국 2026 뉴스" → ratio = 4/8 = 0.50 → `"ko"`
- `TestRSSItemKoreanDetected`: gofeed.Item with title="한국 뉴스 헤드라인" body="본문 내용은 한국어"; assert returned doc has `Lang="ko"`.
- `TestRSSItemEnglishNotDetected`: gofeed.Item with title="English Tech News" body="Content in English"; assert `Lang=""`.
- `TestKNCItemAlwaysKorean`: KNC stub fixture with mixed-language item (title in English, body in Korean); assert returned doc has `Lang="ko"` regardless of ratio (KNC items unconditionally tag because the upstream library is Korean-exclusive).

### NFR-ADP9-001 — RSS Parse-Path Performance

- `BenchmarkParseRSSFeed10Items` is invoked as `go test -bench=BenchmarkParseRSSFeed10Items -benchtime=10x -count=5 ./internal/adapters/koreanews/...` on amd64.
- The fixture is `testdata/rss_2_0.xml` (10-item RSS 2.0 Korean publisher style, ~8KB).
- Assertion: median of 5 reported per-op mean wall-clock durations SHALL be ≤ 10 ms.
- The bench reports `B/op` and `allocs/op`; `allocs/op ≤ 800` per op (10 items per fixture × ~80 allocs/item floor including gofeed's namespace-aware unmarshal + NormalizedDoc Metadata map).

### NFR-ADP9-002 — Race-Clean Concurrent Workload

- `TestSearchConcurrentSafe` (REQ-ADP9-011 acceptance) executes under `go test -race ./internal/adapters/koreanews/...`; race-detector alarms attributable to the koreanews package = 0.

### NFR-ADP9-003 — Zero Goroutine Leaks

- `TestMain` in `bench_test.go`:
  ```go
  func TestMain(m *testing.M) {
      goleak.VerifyTestMain(m)
  }
  ```
  Mirrors `internal/adapters/reddit/bench_test.go`.
- Every Search-invoking test SHALL pass `goleak.VerifyNone(t)` at its end.

### NFR-ADP9-004 — E2E p95 Stub

- `TestSearchE2ELatencyStubP95` runs 100 invocations against the 3-stub-feed pool with each feed returning at 50ms; sorts elapsed durations; asserts `durations[94] ≤ 200ms`.

### Integration Checkpoint (M3 Exit Criterion)

When SPEC-CLI-001's CLI integration registers the `koreanews` adapter alongside Reddit, HN, and Naver (ADP-008), and SPEC-FAN-001 fans out across them, the M3 exit criterion (`.moai/project/roadmap.md:150` — "Korean query returns Naver results ranked first; ADP-009 supplements breadth") becomes achievable. ADP-009's acceptance includes a smoke check (manual or scripted) that `registry.Get("koreanews").Search(ctx, q)` against a 3-feed stub returns at least 1 NormalizedDoc with `Lang="ko"` and `SourceID="koreanews"`. This integration assertion lives in CLI-001's acceptance criteria, not here, but is documented for traceability.

---

## 6. Technical Approach

### 6.1 Files to Modify

**Created (16 Go files + 4 Python scaffold files + 7 testdata fixtures)**:

- `internal/adapters/koreanews/koreanews.go` — Adapter struct, New, Name, Capabilities, Healthcheck, interface assertion
- `internal/adapters/koreanews/koreanews_test.go` — interface conformance + Capabilities determinism + Daum warning emission
- `internal/adapters/koreanews/options.go` — Options + sub-source enable flags + RSS feed list parsing + env loaders
- `internal/adapters/koreanews/options_test.go` — env var parsing + validation
- `internal/adapters/koreanews/search.go` — composite (*Adapter).Search dispatch
- `internal/adapters/koreanews/search_test.go` — composite happy path + error propagation
- `internal/adapters/koreanews/rss.go` — RSS sub-source: gofeed parse, per-feed errgroup, NormalizedDoc transform
- `internal/adapters/koreanews/rss_test.go` — RSS fan-out, per-feed isolation, error categorisation
- `internal/adapters/koreanews/daum.go` — Daum stub: returns ErrDaumDisabled
- `internal/adapters/koreanews/daum_test.go` — stub behaviour verification
- `internal/adapters/koreanews/knc.go` — KoreaNewsCrawler HTTP sidecar client
- `internal/adapters/koreanews/knc_test.go` — sidecar client happy path + 503 handling
- `internal/adapters/koreanews/locale.go` — Hangul-ratio language detection
- `internal/adapters/koreanews/locale_test.go`
- `internal/adapters/koreanews/strip.go` — HTML strip helper (duplicated from ADP-002)
- `internal/adapters/koreanews/strip_test.go`
- `internal/adapters/koreanews/dedup.go` — intra-adapter URL canonicalization + dedup
- `internal/adapters/koreanews/dedup_test.go`
- `internal/adapters/koreanews/errors.go` — sentinels (ErrInvalidQuery, ErrDaumDisabled, ErrKNCSidecarDown, ErrEmptyRSSFeedList)
- `internal/adapters/koreanews/concurrent_test.go` — NFR-ADP9-002 race workload
- `internal/adapters/koreanews/bench_test.go` — BenchmarkParseRSSFeed10Items + TestMain (goleak)
- `internal/adapters/koreanews/testdata/rss_2_0.xml` (~8KB Korean RSS 2.0)
- `internal/adapters/koreanews/testdata/atom_1_0.xml` (~6KB)
- `internal/adapters/koreanews/testdata/json_feed_1_1.json` (~3KB)
- `internal/adapters/koreanews/testdata/rss_with_korean_text.xml` (~4KB Hangul-heavy)
- `internal/adapters/koreanews/testdata/rss_malformed.xml` (~200B truncated)
- `internal/adapters/koreanews/testdata/knc_response.json` (~2KB happy path)
- `internal/adapters/koreanews/testdata/knc_response_503.json` (~200B sidecar unavailable)

Python sidecar SCAFFOLD:
- `services/koreanews/Dockerfile` (python:3.12-slim base; ~30 lines)
- `services/koreanews/pyproject.toml` (pins `KoreaNewsCrawler==1.51`, `fastapi>=0.115`, `uvicorn>=0.32`, `pydantic>=2.9`; ~25 lines)
- `services/koreanews/src/main.py` (FastAPI stub returning 503; ~50 lines)
- `services/koreanews/tests/test_stub.py` (asserts 503 response shape; ~30 lines)
- `services/koreanews/README.md` (documents scaffold status + future SPEC-ADP-009-KNC ref + HTTP contract; ~80 lines)

**Modified (2 files)**:

- `go.mod` / `go.sum` — `go get github.com/mmcdole/gofeed@v1.3.0` adds the RSS parser dependency (MIT licensed, v1.3.0 stable; ~5 transitive deps).
- (Optional) `.moai/config/sections/koreanews.yaml` — operator config for adapter defaults; deferred to a future SPEC-ADP-CFG-001 unless run-phase finds it justified.

**Unchanged (by design)**:

- `pkg/types/*` — no contract change required. ADP-009 consumes existing API.
- `internal/adapters/registry.go` — wrappedAdapter sole-emitter pattern preserved; ADP-009 emits ZERO new metric/log/span families.
- `internal/obs/metrics/metrics.go` — no new metric family. The `adapter="koreanews"` cardinality value fits within the V1 14-adapter ceiling per SPEC-OBS-001 NFR-OBS-002.
- `cmd/usearch/main.go` — registry construction owned by SPEC-CLI-001. ADP-009 does not modify cmd code; CLI-001 will register `koreanews` alongside other adapters in a future commit.

### 6.2 Package Layout

```
internal/adapters/koreanews/
├── koreanews.go                        # Adapter, New, Name, Capabilities, Healthcheck, interface assertion
├── koreanews_test.go                   # Interface conformance + Capabilities determinism
├── options.go                          # Options + env var loaders + validation
├── options_test.go
├── search.go                           # (*Adapter).Search composite dispatch
├── search_test.go                      # Composite tests
├── rss.go                              # RSS sub-source: gofeed parse + per-feed errgroup
├── rss_test.go                         # RSS fan-out + isolation tests
├── daum.go                             # Daum stub returning ErrDaumDisabled
├── daum_test.go
├── knc.go                              # KNC sidecar HTTP client
├── knc_test.go
├── locale.go                           # Hangul-ratio detector
├── locale_test.go
├── strip.go                            # HTML-strip helper (duplicated from ADP-002)
├── strip_test.go
├── dedup.go                            # Intra-adapter URL canonicalization + dedup
├── dedup_test.go
├── errors.go                           # Sentinels
├── concurrent_test.go                  # NFR-ADP9-002 race workload
├── bench_test.go                       # BenchmarkParseRSSFeed10Items + TestMain (goleak)
└── testdata/
    ├── rss_2_0.xml                     # 10-item Korean RSS 2.0 happy path
    ├── atom_1_0.xml                    # Atom 1.0
    ├── json_feed_1_1.json              # JSON Feed 1.1
    ├── rss_with_korean_text.xml        # Hangul-heavy locale-detection fixture
    ├── rss_malformed.xml               # Truncated XML for parse-error path
    ├── knc_response.json               # Sidecar happy-path response
    └── knc_response_503.json           # Sidecar unavailable response
```

```
services/koreanews/                     # SCAFFOLD only in v0.1
├── Dockerfile
├── pyproject.toml
├── README.md
├── src/
│   └── main.py                         # FastAPI stub returning 503
└── tests/
    └── test_stub.py
```

### 6.3 Field Mapping Tables

#### RSS sub-source (`gofeed.Item` → NormalizedDoc)

| gofeed.Item field | NormalizedDoc field | Transform |
|-------------------|---------------------|-----------|
| `GUID` (or `Link` if GUID empty) | `ID` | `"rss:" + canonicalURL(GUID OR Link)` (deterministic per-item) |
| (constant) | `SourceID` | `"koreanews"` (composite Adapter name) |
| `Link` | `URL` | Use as-is; canonicalised at dedup time per SPEC-FAN-001 §2.4 8 rules |
| `Title` | `Title` | Trim whitespace |
| `Content` (if non-empty) else `Description` | `Body` | Apply `stripHTML` (§strip.go); decode entities |
| First 280 runes of stripped Body | `Snippet` | Truncate; append `"..."` if longer; if Body empty, fall back to truncated Title |
| `PublishedParsed` (if non-nil) else `time.Unix(0, 0)` | `PublishedAt` | UTC normalise |
| (parse time) | `RetrievedAt` | `time.Now().UTC()` (set by `parseFeed` caller) |
| `Author.Name` (if non-nil) else `""` | `Author` | RSS `<author>` is often empty; missing is acceptable |
| (constant) | `Score` | `0.5` (mid-bucket placeholder per D5) |
| `detectKorean(Title + " " + Body)` | `Lang` | `"ko"` if Hangul ratio ≥ 0.30; else `""` |
| (constant) | `DocType` | `types.DocTypeArticle` |
| (nil) | `Citations` | `nil` |
| (constructed) | `Metadata` | Map containing two key tiers. **REQUIRED keys** (consumers MAY rely on presence): `subsource` (= `"rss"`), `feed_url` (the operator-supplied URL), `feed_title` (= `feed.Title` if non-empty, else feed_url). REQ-ADP9-008 enforces these 3. **OPTIONAL keys**: `categories` ([]string from gofeed.Item.Categories), `enclosure_count` (int), `encoding_warning` ("non-utf-8 byte sequence detected" when applicable). |
| (constant) | `Hash` | `""` (consumers compute via `CanonicalHash()`) |

#### KNC sub-source (sidecar JSON → NormalizedDoc)

Sidecar response shape (JSON):

```json
{
  "articles": [
    {
      "url": "https://news.example.kr/article/12345",
      "title": "기사 제목",
      "body": "기사 본문 내용...",
      "date": "2026-05-01T10:00:00",
      "author": "조선일보",
      "category": "politics"
    }
  ],
  "errors": []
}
```

| Sidecar JSON field | NormalizedDoc field | Transform |
|--------------------|---------------------|-----------|
| `url` | `ID` | `"knc:" + canonicalURL(url)` (deterministic per-article) |
| (constant) | `SourceID` | `"koreanews"` |
| `url` | `URL` | Use as-is (already canonical Naver portal URL) |
| `title` | `Title` | Use as-is (Korean) |
| `body` | `Body` | Use as-is; not HTML-stripped (KNC sidecar is expected to return plain text) |
| First 280 runes of `body` | `Snippet` | Truncate; if Body empty, fall back to truncated Title |
| `date` | `PublishedAt` | Parse RFC 3339 / Korean `"YYYY-MM-DD HH:MM:SS"`; UTC normalise. If parse fails, set to `time.Time{}` (zero). |
| (parse time) | `RetrievedAt` | `time.Now().UTC()` |
| `author` | `Author` | Use as-is (e.g., `"조선일보"` — newspaper name as Author per Korean news convention) |
| (constant) | `Score` | `0.5` |
| (constant) | `Lang` | `"ko"` (KNC unconditionally Korean per REQ-ADP9-013 KNC-specific clause) |
| (constant) | `DocType` | `types.DocTypeArticle` |
| (nil) | `Citations` | `nil` |
| (constructed) | `Metadata` | **REQUIRED keys**: `subsource` (= `"knc"`), `category` (= JSON `category` field), `data_source` (= `"naver_portal"` constant — KNC scrapes Naver, not direct newspapers). REQ-ADP9-008 enforces these 3. |
| (constant) | `Hash` | `""` |

### 6.4 HTTP Client Construction

The composite adapter constructs ONE `*http.Client` for RSS feed fetches and ONE `*http.Client` for KNC sidecar calls. Both clients share the same construction pattern as ADP-001/002:

- **Timeout**: 30 seconds total request deadline (default; configurable via `Options.HTTPClient`). Caller's ctx deadline takes precedence.
- **Redirect policy** (RSS only): `CheckRedirect` follows up to 3 redirects within ANY host (no per-feed allowlist; operator-supplied feed URLs are inherently trusted by the operator). KNC sidecar redirects beyond `KNCBaseURL` are rejected via a 1-host allowlist.
- **Transport**: `reqid.NewTransport(http.DefaultTransport)` for request-ID propagation (mirrors `internal/llm/client.go:51-54`).
- **Headers per RSS request**: `User-Agent: usearch/<version> (+https://github.com/elymas/universal-search)` AND `Accept: application/rss+xml, application/atom+xml, application/feed+json, application/json;q=0.9, */*;q=0.5`.
- **Headers per KNC request**: same UA AND `Accept: application/json` AND `Content-Type: application/json`.
- NO authentication header on either client (RSS feeds are public; KNC sidecar is internal localhost service).

### 6.5 Observability Note

The koreanews adapter, like Reddit and HN, emits ZERO Prometheus metric families, ZERO OTel spans, and ZERO slog records OF ITS OWN beyond the Daum-warning-at-construction (REQ-ADP9-009). ALL per-call observability comes from the registry's `wrappedAdapter`
(`internal/adapters/registry.go:172-263`). The composite returns a correctly-categorised `*types.SourceError` so `OutcomeFromError(err)` produces the right `outcome` label.

The Daum-warning-at-construction is the ONE exception: it fires from `New(...)` (NOT from `Search`), exactly once per `*Adapter` instance, and exists solely to surface the legal posture to operators in observability dumps. It is NOT a per-call emission and does not violate sole-emitter discipline (sole-emitter discipline applies to per-call observability, not lifecycle events).

Per-feed RSS errors and per-call KNC errors are stored in slog WARN records (NOT new metric families). The shared `AdapterCalls{adapter="koreanews",outcome}` counter family covers per-call success/failure aggregation; per-sub-source errors are accessible via the slog stream (with `subsource` attribute).

### 6.6 MX Tag Plan

| File::Symbol | Tag | Reason |
|--------------|-----|--------|
| `koreanews.go::(*Adapter).Search` (defined in `search.go`) | `@MX:ANCHOR` | Sole entry point for all koreanews fanout calls. fan_in ≥ 3 (registry wrappedAdapter, FAN-001 fanout, tests). `@MX:REASON: contract boundary; signature change ripples to FAN-001 + CLI-001 + IDX-001`. `@MX:SPEC: SPEC-ADP-009`. |
| `rss.go::parseFeed` | `@MX:ANCHOR` | Every RSS doc passes through this single transform. fan_in = 1 (Search-internal) but invariant-bearing — bug here corrupts every RSS-sub-source NormalizedDoc returned. `@MX:REASON: NormalizedDoc field-mapping integrity gate for RSS sub-source`. |
| `rss.go::fetchAllFeeds` | `@MX:WARN` | Outbound fan-out spawns N goroutines (one per configured feed). `@MX:REASON: errgroup pattern + per-feed error isolation must not regress; goroutine leak risk on ctx cancel; pre-launch ctx guard prevents SetLimit deadlock per SPEC-FAN-001 §2.5 H18`. |
| `daum.go::(*Adapter).searchDaum` | `@MX:WARN` | Stub returning ErrDaumDisabled. `@MX:REASON: enabling without future SPEC + Kakao authorisation OR operator-attested compliance review violates robots.txt explicitly forbidding all crawlers (User-agent: * Disallow: /, verified 2026-05-04)`. |
| `knc.go::callSidecar` | `@MX:NOTE` | HTTP POST to Python sidecar; sidecar URL configurable via `Options.KNCBaseURL`. Future SPEC-ADP-009-KNC replaces stub with real KoreaNewsCrawler integration. |
| `locale.go::detectKorean` | `@MX:NOTE` | Heuristic: Hangul rune ratio threshold 0.30. Open Question §11.2 documents revisit triggers (false-positive surveillance under SPEC-IDX-003 ownership). |
| `dedup.go::dedupDocs` | `@MX:NOTE` | Intra-adapter URL canonicalization mirroring SPEC-FAN-001 §2.4. Operates BEFORE FAN-001 cross-adapter dedup. |
| `errors.go::ErrDaumDisabled` | `@MX:NOTE` | Sentinel documents the legal posture. Required reading for any future SPEC author touching the daum sub-source. |

All tags `[AUTO]`-prefixed, include `@MX:SPEC: SPEC-ADP-009`, follow `code_comments: en` per `.moai/config/sections/language.yaml`. Per-file hard limit (3 ANCHOR + 5 WARN per `.moai/config/sections/mx.yaml`): respected (max in this set is 1 ANCHOR + 1 WARN per file; total package: 2 ANCHOR + 2 WARN + 4 NOTE = within limits).

### 6.7 Configuration (Optional)

Operators may tune defaults via `.moai/config/sections/koreanews.yaml`:

```yaml
# .moai/config/sections/koreanews.yaml (NEW; OPTIONAL; deferred to future SPEC-ADP-CFG-001 unless v0.1 finds it justified)
koreanews:
  rss:
    enabled: true
    feeds:
      - url: https://news.example.kr/rss
      - url: https://blog.example.kr/feed.xml
    per_feed_timeout_ms: 30000
    max_parallel_feeds: 8
  daum:
    enabled: false       # cannot be true in v0.1; flag plumbed for future SPEC
  knc:
    enabled: false       # default-off; future SPEC-ADP-009-KNC unlocks
    base_url: http://localhost:8002
```

The CLI's `cmd/usearch/main.go` (or the `executeConfig`) consumes this file and constructs `Options` accordingly. v0.1 ships with all fields defaulted via env vars; YAML config is OPTIONAL.

### 6.8 Harness Level

Per `.moai/config/sections/harness.yaml` auto-routing: 13 EARS REQs (8 × P0 + 5 × P1) + 4 NFRs touching 1 package (`internal/adapters/koreanews/`, ~14 source/test files including testdata) + 1 services scaffold (`services/koreanews/`, ~5 files) + zero cross-package edits + zero security/payment/PII keywords beyond the documented Daum legal posture (which is documentation, not implementation) + zero compose/env/config deltas beyond the four `USEARCH_ADP009_*` env vars and one optional yaml = **standard** harness level. Sprint Contract is OPTIONAL but RECOMMENDED given the Daum legal-posture component (operators reading `Capabilities.Notes` need to understand the robots.txt evidence). Evaluator profile `default` applies.

---

## 7. What NOT to Build (Exclusions)

[HARD] This SPEC explicitly excludes the following items. Each has a known destination SPEC; this list prevents scope creep into ADP-009 (the composite Korean-fallback adapter).

- **Naver primary path** (`isnow890/naver-search-mcp` integration with API-key auth and 25,000/day budget) → SPEC-ADP-008. ADP-009 covers the BREADTH fallback; ADP-008 is the P0 primary.
- **KoreaNewsCrawler full sidecar implementation** (Python FastAPI handler that actually wraps `ArticleCrawler.start()` and intercepts CSV output) → future SPEC-ADP-009-KNC after legal review of Naver-portal scraping (the upstream library targets Naver portal, which has an OFFICIAL API at `openapi.naver.com` that ADP-008 uses; KNC bypasses the official path). v0.1 ships only the Go HTTP client + Python sidecar SCAFFOLD (Dockerfile + pyproject.toml + stub returning 503).
- **Daum search activation** (any working scraping path against `search.daum.net`) → future SPEC-ADP-009-DAUM with explicit Kakao authorisation OR operator-attested compliance review. Daum's robots.txt explicitly forbids all crawlers (`User-agent: *\nDisallow: /` verified 2026-05-04 via WebFetch). The flag is plumbed for future consumption only.
- **YAML file path for RSS feed list** (`USEARCH_ADP009_RSS_FEEDS_FILE` pointing to a YAML config file with per-feed labels and per-feed timeout overrides) → future SPEC-ADP-CFG-001 (horizontal config concern across adapters).
- **EUC-KR or other non-UTF-8 encoding conversion** for legacy Korean RSS feeds → future SPEC. v0.1 emits a `Metadata["encoding_warning"]` annotation when `unicode/utf8.ValidString(title || body)` fails but does NOT attempt conversion via `golang.org/x/text/encoding/korean`.
- **Real language-detect library** (instead of Hangul-ratio heuristic) → SPEC-IDX-003 (Korean tokenization, M3) may upgrade if heuristic produces false-positives.
- **Pagination support for RSS feeds** — RSS does not have query-time pagination; the entire feed is returned per fetch. `Query.Cursor` is ignored by the RSS sub-source. KNC sidecar may support pagination in a future SPEC.
- **Per-feed health monitoring** (granular per-feed availability dashboard) → SPEC-EVAL-002 (M8). v0.1 Healthcheck probes ONE representative endpoint per enabled sub-source.
- **Retry orchestration** (exponential backoff, circuit breaker, per-feed retry budget keyed on `Capabilities.RateLimitPerMin`) → SPEC-FAN-001 (M3). v0.1 ships zero-retry per sub-source.
- **Response caching** (in-process LRU on success path) → out of v0.1; SPEC-CACHE-001 owns the BLOCKED-source 5-phase fallback only.
- **Result ranking fusion across adapters** (Reciprocal Rank Fusion) → SPEC-IDX-001 (M3). Composite output is RRF input.
- **Streaming/incremental result delivery from inside Search** → SPEC-SYN-004 (M4).
- **Per-tenant adapter visibility / RBAC** (some teams may not want certain feeds enabled) → SPEC-AUTH-002 (M6).
- **Per-adapter custom Prometheus metrics** (e.g., `koreanews_feeds_fetched_total`) → would require amending SPEC-OBS-001 allowlist. Out of v0.1.
- **HTTP / gRPC server exposure of the adapter** → SPEC-MCP-001 (M7) and future SPEC-API-001. ADP-009 is a Go library only in v0.1.
- **Cross-adapter helper extraction** (`stripHTML` shared between HN and ADP-009; URL canonicalization shared between FAN-001 and ADP-009 dedup) → out of v0.1. Per SPEC-ADP-002 §11.4 "rule of three" guidance, consolidation waits until 3+ adapters use the helper. ADP-009 is the second consumer; consolidation triggers when the next adapter (ADP-006 Bluesky?) also needs HTML-strip.
- **Cardinality allowlist amendment** — ZERO new label names.
- **Live network integration tests in CI** → out of v0.1; httptest + golden fixtures only. Optional env-gated live test (`-tags=integration` + `KOREANEWS_LIVE=1`) deferred.
- **OpenAPI / proto schema for the adapter response** — the `[]types.NormalizedDoc` return type IS the schema; no separate IDL.
- **`pkg/llm` integration** — the koreanews adapter does NOT call any LLM. Classification is the Intent Router's job (SPEC-IR-001).
- **MX tag scoring weights or RRF input parameters** (Score=0.5 placeholder is intentional; tuning per-adapter Score curves is out of v0.1).

---

## 8. TDD Plan

Development follows RED-GREEN-REFACTOR per `quality.development_mode: tdd` (`.moai/config/sections/quality.yaml`). Representative RED-phase tests, written before implementation, grouped by REQ. Total: ~40 tests covering REQ-ADP9-001..013 + NFRs. Coverage target: 85% per `quality.test_coverage_target`. Benchmarks do not count toward coverage.

| # | Test | Layer | REQ | Assertion |
|---|------|-------|-----|-----------|
| 1 | `TestAdapterName` | `koreanews_test.go` | REQ-ADP9-001 | `(*Adapter).Name() == "koreanews"` |
| 2 | `TestAdapterImplementsInterface` | `koreanews_test.go` | REQ-ADP9-001 | Compile-time `var _ types.Adapter = (*Adapter)(nil)` succeeds |
| 3 | `TestCapabilitiesDeterministic` | `koreanews_test.go` | REQ-ADP9-001 | Two consecutive `Capabilities()` calls return `reflect.DeepEqual` results |
| 4 | `TestCapabilitiesShape` | `koreanews_test.go` | REQ-ADP9-001 | All 9 documented fields match (SourceID, DisplayName, DocTypes=[Article], SupportedLangs=["ko"], SupportsSince=false, RequiresAuth=false, AuthEnvVars=nil, RateLimitPerMin=0, DefaultMaxResults=50) + Notes substring matches |
| 5 | `TestHealthcheckSucceeds` | `koreanews_test.go` | REQ-ADP9-001 | TCP/HTTP probe against test loopback succeeds |
| 6 | `TestSearchAllSubSourcesEnabled` | `search_test.go` | REQ-ADP9-002 | RSS+KNC enabled; mixed docs returned; all `Metadata["subsource"]` populated |
| 7 | `TestSearchOnlyRSSEnabled` | `search_test.go` | REQ-ADP9-002 | RSS only; all docs have `subsource=="rss"` |
| 8 | `TestSearchAllSubSourcesDisabled` | `search_test.go` | REQ-ADP9-002 | All flags false; `(nil, nil)` returned |
| 9 | `TestSearchOneSubSourceFailsOthersSucceed` | `search_test.go` | REQ-ADP9-002 | RSS feed1 4xx + RSS feed2 200 + KNC 200; composite succeeds with merged docs |
| 10 | `TestSearchRSSHappyPath3Feeds` | `rss_test.go` | REQ-ADP9-003 | 3×5 = 15 docs; all `subsource=="rss"` |
| 11 | `TestSearchRSSPerFeedTimeoutIndependent` | `rss_test.go` | REQ-ADP9-003 | 1 slow feed times out; siblings succeed; total elapsed ≈ per-feed timeout |
| 12 | `TestSearchRSSPerFeedErrorIsolation` | `rss_test.go` | REQ-ADP9-003 | 4xx + malformed XML feeds isolated; only good feed contributes |
| 13 | `TestSearchRSSHonoursMaxParallelFeeds` | `rss_test.go` | REQ-ADP9-003 | 16 feeds, max=4; observed `max(inflight)==4` |
| 14 | `TestSearchRSSCanceledMidQueue` | `rss_test.go` | REQ-ADP9-003 | Pre-launch ctx guard prevents SetLimit deadlock |
| 15 | `TestSearchRSSHTTP4xx` | `rss_test.go` | REQ-ADP9-004 | Table 401/403/404 → CategoryPermanent + matching HTTPStatus |
| 16 | `TestSearchRSSHTTP5xx` | `rss_test.go` | REQ-ADP9-004 | Table 500/502/503 → CategoryUnavailable |
| 17 | `TestSearchRSSConnectionRefused` | `rss_test.go` | REQ-ADP9-004 | Pre-closed server → CategoryUnavailable + HTTPStatus=0 |
| 18 | `TestSearchRSSMalformedXML` | `rss_test.go` | REQ-ADP9-004 | Truncated XML → CategoryPermanent + reason="malformed" |
| 19 | `TestSearchRSSPerFeedNoInternalRetry` | `rss_test.go` | REQ-ADP9-004 | Server request count == 1 per feed |
| 20 | `TestSearchEmptyQueryRejectedNoHTTP` | `search_test.go` | REQ-ADP9-005 | Table empty/whitespace q.Text → ErrInvalidQuery + zero requests |
| 21 | `TestSearchEmptyRSSFeedListRejected` | `search_test.go` | REQ-ADP9-005 | RSSEnabled + nil feeds → ErrEmptyRSSFeedList + zero requests |
| 22 | `TestSearchEmptyQueryTakesPrecedence` | `search_test.go` | REQ-ADP9-005 | Empty query + nil feeds → ErrInvalidQuery (not ErrEmptyRSSFeedList) |
| 23 | `TestSearchEmptyFeedListAcceptableWhenRSSDisabled` | `search_test.go` | REQ-ADP9-005 | RSS off + KNC on with stub → success |
| 24 | `TestSearchDaumStubReturnsErrDisabled` | `daum_test.go` | REQ-ADP9-006 | DaumEnabled=true → ErrDaumDisabled + zero outbound to *.daum.net |
| 25 | `TestSearchDaumDisabledNoOp` | `daum_test.go` | REQ-ADP9-006 | DaumEnabled=false → no error, RSS docs returned |
| 26 | `TestCapabilitiesNotesDocumentDaumStatus` | `koreanews_test.go` | REQ-ADP9-006 | Notes contains "daum: disabled" + "robots.txt" + "Disallow: /" |
| 27 | `TestSearchKNCSidecarStub503` | `knc_test.go` | REQ-ADP9-007 | Sidecar 503 → ErrKNCSidecarDown + CategoryUnavailable |
| 28 | `TestSearchKNCSidecarHappyPath` | `knc_test.go` | REQ-ADP9-007 | Sidecar 200/JSON → decoded NormalizedDocs with subsource="knc", Lang="ko" |
| 29 | `TestSearchKNCSidecarHTTP4xx` | `knc_test.go` | REQ-ADP9-007 | 400/404 → CategoryPermanent |
| 30 | `TestSearchKNCSidecarHTTP5xx` | `knc_test.go` | REQ-ADP9-007 | 502/504 → CategoryUnavailable |
| 31 | `TestSearchKNCSidecarMalformedJSON` | `knc_test.go` | REQ-ADP9-007 | Truncated JSON → CategoryPermanent + reason="malformed" |
| 32 | `TestSearchKNCDisabledNoOp` | `knc_test.go` | REQ-ADP9-007 | KNCEnabled=false → no outbound to KNC URL |
| 33 | `TestSearchKNCRequestShape` | `knc_test.go` | REQ-ADP9-007 | POST + path + headers + JSON body match |
| 34 | `TestParseRSSFieldMapping` | `rss_test.go` | REQ-ADP9-008 | Table 4 fixtures; every NormalizedDoc field per §6.3 RSS table |
| 35 | `TestParseKNCFieldMapping` | `knc_test.go` | REQ-ADP9-008 | KNC fixture; every field per §6.3 KNC table |
| 36 | `TestAllReturnedDocsHaveSubsourceMetadata` | `search_test.go` | REQ-ADP9-008 | Every doc has `Metadata["subsource"] in {"rss","knc"}` |
| 37 | `TestAllReturnedDocsHaveScoreHalf` | `search_test.go` | REQ-ADP9-008 | Every doc.Score == 0.5 |
| 38 | `TestAllReturnedDocsHaveDocTypeArticle` | `search_test.go` | REQ-ADP9-008 | Every doc.DocType == DocTypeArticle |
| 39 | `TestAllReturnedDocsHashEmpty` | `search_test.go` | REQ-ADP9-008 | Every doc.Hash == "" |
| 40 | `TestAllReturnedDocsCitationsNil` | `search_test.go` | REQ-ADP9-008 | Every doc.Citations == nil |
| 41 | `TestRSSDocsMetadataKeys` | `rss_test.go` | REQ-ADP9-008 | RSS docs have {subsource, feed_url, feed_title} |
| 42 | `TestKNCDocsMetadataKeys` | `knc_test.go` | REQ-ADP9-008 | KNC docs have {subsource, category, data_source} |
| 43 | `TestNewLogsDaumWarningWhenEnabled` | `koreanews_test.go` | REQ-ADP9-009 | DaumEnabled=true → 1 slog WARN at construction |
| 44 | `TestNewNoDaumWarningWhenDisabled` | `koreanews_test.go` | REQ-ADP9-009 | DaumEnabled=false → 0 WARN records |
| 45 | `TestCapabilitiesNotesIncludesRobotsTxtURL` | `koreanews_test.go` | REQ-ADP9-009 | Notes contains literal "https://search.daum.net/robots.txt" |
| 46 | `TestRSSFetchSetsCustomUserAgent` | `rss_test.go` | REQ-ADP9-010 | UA starts with "usearch/" + URL |
| 47 | `TestRSSFetchSetsAcceptFeedTypes` | `rss_test.go` | REQ-ADP9-010 | Accept contains rss+xml, atom+xml, feed+json |
| 48 | `TestKNCSetsAcceptJSON` | `knc_test.go` | REQ-ADP9-010 | KNC Accept = application/json |
| 49 | `TestKNCSetsContentTypeJSON` | `knc_test.go` | REQ-ADP9-010 | KNC Content-Type = application/json |
| 50 | `TestUserAgentVersionConfigurable` | `rss_test.go` + `knc_test.go` | REQ-ADP9-010 | Options.UserAgentVersion override propagates |
| 51 | `TestSearchConcurrentSafe` | `concurrent_test.go` | REQ-ADP9-011, NFR-ADP9-002 | 50 goroutines × 1 Search race-clean |
| 52 | `TestSearchPartialResultsOnParentTimeout` | `search_test.go` | REQ-ADP9-012 | 5 feeds, 500ms parent ctx → partial 10 docs returned |
| 53 | `TestSearchAlreadyCancelledCtx` | `search_test.go` | REQ-ADP9-012 | Pre-cancelled ctx → CategoryUnavailable + zero requests |
| 54 | `TestSearchNoGoroutineLeakOnCancel` | `search_test.go` | REQ-ADP9-012, NFR-ADP9-003 | goleak.VerifyNone after mid-flight cancel |
| 55 | `TestDetectKoreanTable` | `locale_test.go` | REQ-ADP9-013 | 6+ inputs (pure ko, pure en, mixed, empty, whitespace, ko+nums) → expected output |
| 56 | `TestRSSItemKoreanDetected` | `rss_test.go` | REQ-ADP9-013 | Korean title+body → Lang="ko" |
| 57 | `TestRSSItemEnglishNotDetected` | `rss_test.go` | REQ-ADP9-013 | English-only → Lang="" |
| 58 | `TestKNCItemAlwaysKorean` | `knc_test.go` | REQ-ADP9-013 | KNC item → Lang="ko" regardless of ratio |
| 59 | `TestSearchE2ELatencyStubP95` | `search_test.go` | NFR-ADP9-004 | 100 invocations against 3-feed stub; p95 ≤ 200ms |
| 60 | `BenchmarkParseRSSFeed10Items` | `bench_test.go` | NFR-ADP9-001 | 10-item RSS 2.0 fixture; median p50 ≤ 10ms; allocs/op ≤ 800 |
| 61 | `TestMain` (goleak.VerifyTestMain) | `bench_test.go` | NFR-ADP9-003 | Package-level goroutine leak check |
| 62 | `TestEnvVarRSSFeedsJSON` | `options_test.go` | REQ-ADP9-002/003 | USEARCH_ADP009_RSS_FEEDS as JSON array → parsed correctly |
| 63 | `TestEnvVarRSSFeedsCommaList` | `options_test.go` | REQ-ADP9-002/003 | USEARCH_ADP009_RSS_FEEDS as comma-list → parsed correctly |
| 64 | `TestEnvVarRSSFeedsCappedAt32` | `options_test.go` | REQ-ADP9-002 | 50 feeds in env → truncated to 32 with WARN |
| 65 | `TestDedupSameURLFirstWins` | `dedup_test.go` | REQ-ADP9-002 | Intra-adapter dedup table |
| 66 | `TestStripHTMLTable` | `strip_test.go` | REQ-ADP9-008 | Duplicated from ADP-002 (8 inputs) |

RED-GREEN-REFACTOR per requirement:
1. RED: Write failing test for REQ-ADP9-N.
2. GREEN: Implement minimal code to pass.
3. REFACTOR: Tidy; extract shared helpers if they remove duplication WITHIN the package; keep file sizes manageable (target each `.go` file < 250 LoC excluding tests).

Greenfield note: `internal/adapters/koreanews/` does not exist. There is no behaviour to preserve; no characterization tests needed.

---

## 9. Dependencies

### 9.1 Upstream SPEC Dependencies

- **SPEC-CORE-001 (implemented)**: provides `pkg/types.Adapter`, `pkg/types.Capabilities`, `pkg/types.Query`, `pkg/types.NormalizedDoc`, `*types.SourceError`, `types.OutcomeFromError`, `types.DocType` enum, `internal/adapters.Registry` with wrappedAdapter sole-emitter pattern, `internal/adapters/noop` reference shape. HARD dep.
- **SPEC-OBS-001 (implemented)**: provides `obs.Obs` bundle, `internal/obs/reqid.NewTransport` for request-ID propagation, `AdapterCalls{adapter,outcome}` and `AdapterCallDuration{adapter}` collectors. SOFT dep — adapter is nil-safe via the registry's nil-guards. The `adapter="koreanews"` cardinality value fits within the V1 14-adapter ceiling per SPEC-OBS-001 NFR-OBS-002.
- **SPEC-IR-001 (implemented)**: documents the consumer contract for `Capabilities` (REQ-IR-008 selects AdapterSet by intersecting `categoryEligibleDocTypes` with `SupportedLangs`). ADP-009's `Capabilities()` shape (DocTypes=[Article], SupportedLangs=["ko"]) determines which routing categories the koreanews adapter will be selected for. SOFT dep — IR-001 lookups happen at startup; ADP-009 just declares its capability.

### 9.2 Pattern References (Not Hard Deps)

- **SPEC-ADP-001 (implemented)**: reference adapter shape (Reddit) — file layout, MX tag plan, TDD harness, error mapping discipline, sole-emitter discipline, REQ-ADP-011 concurrent-safety contract. ADP-009 inherits structure verbatim where applicable.
- **SPEC-ADP-002 (implemented)**: second-shape duplication (Hacker News) — `stripHTML` helper duplicated verbatim into ADP-009/strip.go (per §11.4 rule-of-three guidance).
- **SPEC-FAN-001 (approved)**: URL canonicalization rules at §2.4 reused for ADP-009's intra-adapter dedup (`dedup.go::dedupDocs`); per-feed errgroup pattern at §2.5/§2.6 reused for `rss.go::fetchAllFeeds`.

### 9.3 Parallelizable

- **SPEC-CLI-001 (M2; implemented)**: a follow-up commit registers `koreanews` adapter alongside Reddit/HN in `cmd/usearch/main.go`. Depends on ADP-009's `New(opts) (*Adapter, error)` constructor signature being approved.
- **SPEC-FAN-001 (M3; approved)**: consumes `(*koreanews.Adapter).Search` via `registry.Get("koreanews").Search(ctx, q)`. With ADP-009 + FAN-001 + Naver-ADP-008, fanout testing exercises 3+ adapters including the Korean primary and breadth paths.
- **SPEC-ADP-003 / 004 / 005 / 006 / 007 / 008 (all M3)**: parallel adapter SPECs gated on FAN-001. ADP-009 is the THIRD in the seven-way M3 ADP-* parallelization (per `.moai/project/roadmap.md:122-123`).

### 9.4 Downstream Blocked SPECs

- **SPEC-IDX-001** (M3): consumes `[]NormalizedDoc` from ADP-009 (Lang="ko" docs flow into Korean shard per SPEC-IDX-003).
- **SPEC-IDX-003** (M3): may upgrade `locale.go::detectKorean` heuristic to a real language-detect library.
- **SPEC-EVAL-002** (M8): may upgrade `Healthcheck` to per-feed granular health.
- **SPEC-ADP-009-KNC** (future post-M3): full KoreaNewsCrawler Python sidecar implementation, replacing the v0.1 stub.
- **SPEC-ADP-009-DAUM** (future, conditional on legal review): unlock the Daum sub-source.
- **SPEC-ADP-CFG-001** (future): horizontal config concern across adapters; may add YAML file path for RSS feed list.

### 9.5 External Dependencies (run-phase pins)

**ONE new Go module dependency**:
- `github.com/mmcdole/gofeed` v1.3.0 (MIT, released 2024-03-01, actively maintained). Transitive deps: `github.com/mmcdole/goxpp`, `github.com/PuerkitoBio/goquery`, `github.com/json-iterator/go` (all MIT or Apache-2.0). Run-phase adds via `go get github.com/mmcdole/gofeed@v1.3.0`.

Otherwise pure stdlib:
- Go stdlib: `context`, `encoding/json`, `errors`, `fmt`, `io`, `net`, `net/http`, `net/url`, `os`, `strconv`, `strings`, `sync`, `sync/atomic`, `time`, `unicode`, `unicode/utf8`
- `pkg/types` (already pinned via SPEC-CORE-001)
- `internal/obs/reqid` (already pinned via SPEC-OBS-001)
- `golang.org/x/sync/errgroup` (already pinned via `go.mod:33` `golang.org/x/sync v0.20.0`)
- Test-only: `go.uber.org/goleak` (already pinned indirect via `go.mod:30`; reddit/HN adapters already use it)

Python sidecar SCAFFOLD dependencies (deferred to SPEC-ADP-009-KNC implementation):
- `KoreaNewsCrawler==1.51` (MIT, 2022-stale)
- `fastapi>=0.115` (MIT)
- `uvicorn>=0.32` (BSD-3-Clause)
- `pydantic>=2.9` (MIT)

---

## 10. Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Daum scraping enabled accidentally violates robots.txt | Low | High | REQ-ADP9-006 makes the Daum stub return ErrDaumDisabled REGARDLESS of the env flag's value. The flag is plumbed for future SPEC consumption only. The Daum-warning-at-construction (REQ-ADP9-009) emits a slog WARN to alert operators who flip the flag of the legal posture. |
| RSS feed publisher's robots.txt forbids automated fetching | Medium | Medium | The adapter does NOT validate per-feed robots.txt — operators choose feeds explicitly via env var, accepting responsibility. `Capabilities.Notes` documents that operators must review each feed's ToS before adding to the list. |
| KoreaNewsCrawler sidecar enabled without sidecar running | Medium | Low | REQ-ADP9-007 maps connection-refused / 503 to ErrKNCSidecarDown with `CategoryUnavailable`. The composite returns success with zero KNC docs (RSS still contributes). Operators see the WARN slog and stand up the sidecar at their leisure. |
| gofeed library fails on EUC-KR encoded Korean RSS feeds | Medium | Medium | gofeed's underlying `goxpp` parser respects the XML `encoding=` declaration; feeds with correct declaration parse fine. Feeds without declarations produce garbled output, which the adapter SHOULD annotate via `Metadata["encoding_warning"]`. Open Question §11.5 documents revisit triggers. |
| Hangul-ratio heuristic produces false-positives on emoji-heavy English text | Low | Low | Heuristic uses `unicode.Is(unicode.Hangul, r)` which strictly matches Hangul block — emoji do NOT trigger. Pure English with Korean accents (e.g., "한국 today") falls below 0.30 ratio. SPEC-IDX-003 may upgrade if real-world feeds produce surprises. |
| RSS feed list grows beyond 32 entries (operator misconfig) | Medium | Low | REQ-ADP9-002 hard-caps at 32; excess entries truncated with slog WARN. Operators see the warning and either prune or escalate to a future SPEC raising the cap. |
| Per-feed error message contains feed URL leaking sensitive data | Low | Low | Operator-supplied feed URLs are inherently visible in config. Error messages include feed URL for diagnostic purposes. Operators with sensitive internal-feed URLs control the slog routing (Loki / S3 / etc.). |
| RSS feed returns extremely large payload (multi-MB feed) | Low | Medium | gofeed's pull-based XML parser is memory-efficient for streaming; per-feed timeout (30s) bounds parse time. NFR-ADP9-001 alloc ceiling (≤800/op) keeps per-doc memory bounded. |
| KNC sidecar URL points to attacker-controlled host (operator misconfig) | Low | High | The KNC sidecar URL is operator-controlled via `USEARCH_ADP009_KNC_BASE_URL`. Defaults to localhost. The 1-host redirect allowlist on the KNC client rejects redirects beyond the configured base URL. SSRF surface is the operator's responsibility (consistent with MoAI's existing self-hosted deployment posture). |
| Score=0.5 placeholder breaks RRF tie-breaking | Low | Low | SPEC-IDX-001 RRF uses RANK not raw score across adapters; ties within ADP-009 break by `RetrievedAt`. Open Question §11.7 documents the revisit trigger if measured ranking quality suffers. |
| Goroutine leak under panic in gofeed parser | Low | High | `defer recover()` per per-feed worker (mirroring SPEC-FAN-001 §2.6 pattern); panic converted to `*SourceError{CategoryUnknown}`. NFR-ADP9-003 + goleak verify in CI. |
| Composite SPEC duplicates SPEC-FAN-001 errgroup machinery | Medium | Low | Acknowledged duplication; SPEC-FAN-001 §11.4 "rule of three" guidance applies. ADP-009's intra-adapter fan-out is the SECOND consumer of the per-index-slice + errgroup + pre-launch-ctx-guard pattern. Consolidation triggers at the third. |
| Operator accidentally enables KNC without legal review of Naver-portal scraping | Low | Medium | KNC scaffold's stub returns 503 by default. Operators completing the sidecar must read `services/koreanews/README.md` which documents the future SPEC-ADP-009-KNC requirement and the legal posture (KNC bypasses Naver's official API used by SPEC-ADP-008). |
| RSS feed with relative URLs in `<link>` produces invalid NormalizedDoc.URL | Low | Low | gofeed handles relative URL resolution against the feed's `xml:base` attribute. The adapter's URL canonicalization step handles edge cases. Tests cover relative-URL fixture. |
| `time.Now()` in `RetrievedAt` non-deterministic in tests | Low | Low | Test stubs accept injectable time via `Options.NowFunc func() time.Time` (default `time.Now`); tests inject a fixed time. |
| HTTP timeout (30s) too aggressive for slow Korean feeds | Low | Low | Configurable via `Options.RSSPerFeedTimeout`. Default 30s aligns with the per-feed scope (feed parsing is bounded by feed size, not HTTP latency). |

---

## 11. Open Questions

These are explicitly UNRESOLVED at SPEC-approval time. Each has a recommended default and a one-line resolution owner. They do NOT block SPEC approval.

1. **KNC sidecar implementation timeline**. Recommended default: SCAFFOLD only in v0.1; full sidecar in future SPEC-ADP-009-KNC after legal review of Naver-portal scraping. Resolution owner: future SPEC-ADP-009-KNC author after M3 completes.

2. **Daum re-enablement path**. Recommended default: NEVER without legal review. Future SPEC-ADP-009-DAUM may unlock the path with explicit Kakao authorisation OR operator-attested compliance review. Resolution owner: future SPEC-ADP-009-DAUM author + legal review.

3. **YAML file path for RSS feed list**. Recommended default: env var only in v0.1; YAML path deferred to future SPEC-ADP-CFG-001 (horizontal config concern across adapters). Resolution owner: future SPEC-ADP-CFG-001 author.

4. **Cross-adapter helper extraction** (sharing `stripHTML`, URL canonicalization, errgroup pattern). Recommended default: defer until 3+ consumers (rule of three per SPEC-ADP-002 §11.4). v0.1 ships duplicate helpers in each adapter package and intra-adapter fanout. Resolution owner: SPEC-ADP-REFAC-001 author (TBD post-M3).

5. **Feed encoding detection (UTF-8 vs EUC-KR)**. Recommended default: NO conversion in v0.1; rely on XML `encoding=` declaration; emit `Metadata["encoding_warning"]` on `unicode/utf8.ValidString` failure. Future SPEC may add `golang.org/x/text/encoding/korean` conversion. Resolution owner: SPEC-IDX-003 (Korean tokenization) author may upgrade if it becomes painful at indexing time.

6. **Per-feed Healthcheck granularity**. Recommended default: probe ONE representative endpoint per enabled sub-source. Future SPEC-EVAL-002 (M8) may upgrade per-feed health surfacing. Resolution owner: SPEC-EVAL-002 author.

7. **Score normalization**. Recommended default: constant `Score = 0.5` for all ADP-009 docs in v0.1 (mid-bucket placeholder). Future SPEC-IDX-001 RRF integration may reveal need for per-sub-source calibration. Resolution owner: SPEC-IDX-001 author.

8. **Korean locale heuristic upgrade**. Recommended default: Hangul-ratio threshold 0.30 in v0.1. Future SPEC-IDX-003 may upgrade to a real language-detect library. Resolution owner: SPEC-IDX-003 author.

---

## 12. References

### External (URL-cited; verified per research.md §10)

- `https://github.com/lumyjuwon/KoreaNewsCrawler` — KoreaNewsCrawler library (MIT, v1.51 released 2022-03-27, unmaintained, scrapes Naver portal).
- `https://github.com/lumyjuwon/KoreaNewsCrawler/blob/master/README.md` — README extract: ArticleCrawler API, supported categories (politics/economy/society/living_culture/IT_science/world/opinion), output CSV schema (date/category/media/title/body/url).
- `https://github.com/mmcdole/gofeed` — gofeed Go library (MIT, v1.3.0 released 2024-03-01, actively maintained, supports RSS 0.90-2.0 / Atom 0.3-1.0 / JSON Feed 1.0-1.1).
- `https://search.daum.net/robots.txt` — Daum robots.txt: `User-agent: *\nDisallow: /` (verified via WebFetch 2026-05-04; full-domain disallow for all crawlers).

### Internal (file:line cited)

- `.moai/specs/SPEC-ADP-009/research.md` — full research artifact (this SPEC's research sibling).
- `.moai/specs/SPEC-CORE-001/spec.md` — Adapter / Capabilities / Query / NormalizedDoc / SourceError / Registry contract.
- `.moai/specs/SPEC-OBS-001/spec.md` — observability bundle and cardinality discipline.
- `.moai/specs/SPEC-IR-001/spec.md` — `Capabilities` consumer contract (REQ-IR-008 SupportedLangs intersection).
- `.moai/specs/SPEC-ADP-001/spec.md` — Reddit reference adapter shape.
- `.moai/specs/SPEC-ADP-002/spec.md` — Hacker News second-shape; `stripHTML` precedent at §6.4.
- `.moai/specs/SPEC-FAN-001/spec.md` — Multi-source fanout; URL canonicalization rules at §2.4 (8 rules); per-index-slice + errgroup pattern at §2.5/§2.6.
- `pkg/types/adapter.go:28-45` — Adapter interface 4-method shape.
- `pkg/types/capabilities.go:38-62` — Capabilities struct + DocType enum.
- `pkg/types/query.go:18-44` — Query struct + Filter shape.
- `pkg/types/errors.go:14-218` — SourceError, Category enum, OutcomeFromError.
- `pkg/types/normalized_doc.go:40-106` — NormalizedDoc 15-field struct, Validate, CanonicalHash.
- `internal/adapters/registry.go:75-167` — Registry lifecycle.
- `internal/adapters/registry.go:172-263` — wrappedAdapter sole-emitter pattern.
- `internal/adapters/noop/noop.go:1-46` — reference adapter shape + compile-time interface assertion.
- `internal/adapters/reddit/reddit.go:1-136` — Adapter struct pattern (mirrored by ADP-009 koreanews.go).
- `internal/adapters/reddit/search.go:1-167` — Search hot path pattern.
- `internal/adapters/reddit/parse.go:1-203` — parseListing pattern (RSS parseFeed equivalent).
- `internal/adapters/reddit/client.go:1-125` — HTTP client + redirect allowlist pattern.
- `internal/adapters/reddit/score.go:1-41` — score normalization pattern (NOT used by ADP-009; constant 0.5 instead).
- `internal/adapters/reddit/errors.go:1-64` — parseRetryAfter helper pattern (NOT used by ADP-009 directly; sentinel pattern reused).
- `internal/adapters/hn/strip.go` — `stripHTML` helper duplicated verbatim into ADP-009/strip.go.
- `internal/adapters/registry.go:172-263` — wrappedAdapter sole-emitter pattern.
- `services/researcher/` — Python sidecar precedent (gpt-researcher wrapper, FastAPI + Pydantic v2; SPEC-SYN-001 implementation 2026-05-04).
- `services/embedder/` — second Python sidecar precedent.
- `internal/llm/client.go:31-65` — HTTP client construction pattern with timeout + reqid Transport wrapping.
- `.moai/project/roadmap.md:54` — M3 row for SPEC-ADP-009.
- `.moai/project/roadmap.md:122-123` — M3 parallelization gate on SPEC-FAN-001.
- `.moai/project/roadmap.md:150` — M3 exit criterion.
- `.moai/project/structure.md:18-22` — `internal/adapters/daum/`, `rss_korean/` reservations consolidated into `koreanews/`.
- `.moai/project/tech.md:117-118` — Per-source adapter strategy: Daum/KNC scraper-style, RSS gofeed.
- `.moai/config/sections/quality.yaml` — `development_mode: tdd`, `test_coverage_target: 85`.
- `.moai/config/sections/harness.yaml` — auto-routing to standard level.
- `.moai/config/sections/language.yaml` — `documentation: en`, `code_comments: en`.
- `go.mod` — Go 1.25.8; existing dependencies do not yet include `github.com/mmcdole/gofeed`. Run-phase will add via `go get github.com/mmcdole/gofeed@v1.3.0`.

---

*End of SPEC-ADP-009 v0.1 (status: draft, awaiting plan-auditor cycle 1)*
