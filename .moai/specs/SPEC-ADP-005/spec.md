---
id: SPEC-ADP-005
title: YouTube Adapter
version: 0.1.0
milestone: M3 — Fanout, adapters, index
status: implemented
priority: P0
owner: expert-backend
methodology: tdd
coverage_target: 85
created: 2026-05-04
updated: 2026-05-07
author: limbowl
issue_number: null
depends_on: [SPEC-CORE-001, SPEC-OBS-001, SPEC-IR-001]
blocks: []
---

# SPEC-ADP-005: YouTube Adapter

## HISTORY

- 2026-05-07 (implemented v0.1, manager-tdd via TDD RED-GREEN-REFACTOR):
  Implementation complete. 91.2% test coverage (target 85%). All SPEC acceptance
  criteria met. golangci-lint clean. Race detector clean. Score formula corrected
  per exact Go math (SPEC §2.3 table had rounding in tanh column; formula
  Score = clamp(0.5 + 0.5*tanh(log10(v+1)/5.0)) is authoritative).

- 2026-05-04 (initial draft v0.1, limbowl via manager-spec):
  M3 YouTube adapter SPEC drafted following the SPEC-ADP-001 reference
  shape verbatim with HN-style and YouTube-specific deltas. Scope and
  contracts derived from `.moai/specs/SPEC-ADP-005/research.md` (every
  external claim WebFetch-verified 2026-05-04; every internal claim
  file:line-cited). Built on SPEC-CORE-001 (`pkg/types.Adapter`
  interface, `pkg/types.Capabilities` descriptor, `pkg/types.Query`,
  `pkg/types.NormalizedDoc` 15-field struct, `*types.SourceError`
  taxonomy, `pkg/types.DocTypeVideo` enum at
  `pkg/types/capabilities.go:18`, registry wrappedAdapter sole-emitter
  pattern at `internal/adapters/registry.go:172-263`), SPEC-OBS-001
  (`AdapterCalls{adapter,outcome}` + `AdapterCallDuration{adapter}`
  collectors with `adapter` and `outcome` already in cardinality
  allowlist), SPEC-IR-001 (Capabilities consumer contract), and
  SPEC-ADP-001 / SPEC-ADP-002 (file layout + error mapping + MX tag
  plan + TDD harness, all reused as pattern).

  User-locked decisions baked in:

  - **D1 Integration path**: yt-dlp Python sidecar wrapped via FastAPI
    HTTP, NOT direct YouTube Data API v3 calls. Decision rationale and
    full comparison matrix in research.md §1.3. Path A wins on the
    three highest-weight criteria: (a) transcripts available (Path B's
    captions endpoint requires OAuth + video ownership for
    third-party access — effectively impossible per
    https://developers.google.com/youtube/v3/docs/captions); (b) daily
    search budget (Path B's free-tier 10,000 quota units / 100 units
    per search.list = 100 searches/day, per
    https://developers.google.com/youtube/v3/getting-started — non-
    starter for a search engine); (c) Korean-locale transcript
    handling (yt-dlp `--sub-langs ko` works; Path B doesn't even
    return transcripts). The roadmap entry at
    `.moai/project/roadmap.md:50` ("yt-dlp metadata + transcript,
    rate-limit") confirms this was the intended path from the
    project's inception.
  - **D2 Sidecar architecture**: dedicated Python service at
    `services/youtube-extract/` (port 8082) mirroring the
    `services/researcher/` precedent (FastAPI + uvicorn + multi-stage
    Dockerfile + non-root user + healthcheck). The Go adapter at
    `internal/adapters/youtube/` is an HTTP client; yt-dlp subprocess
    invocation is fully contained in the Python sidecar. Direct
    `os/exec.Cmd` from Go was rejected because (a) `usearch`'s
    single-binary deploy goal at `.moai/project/tech.md:17` is
    incompatible with bundling Python + yt-dlp dependencies; (b)
    yt-dlp segfaults on malformed videos must not crash the Go
    process; (c) the existing `services/researcher/` sidecar pattern
    is the established precedent.
  - **D3 Auth posture**: `Capabilities.RequiresAuth=false`,
    `AuthEnvVars=nil`. Optional cookie file injection (env
    `YT_COOKIES_PATH` on the sidecar; Docker secret) is an
    OPERATIONAL mitigation for IP-block challenges, NOT a hard auth
    requirement. The public path works without cookies. Per research
    §1.4, https://github.com/yt-dlp/yt-dlp/issues/10128 documents the
    "Sign in to confirm you're not a bot" challenge from mid-2024;
    cookies bypass it; ADP-005 v0.1 ships the cookie hook but does
    not require operators to populate it.
  - **D4 Rate-limit / IP-block defence**: sidecar runs yt-dlp with
    `--sleep-requests 1.0 --sleep-interval 2 --max-sleep-interval 5`
    by default (configurable via env). On HTTP 429 with Retry-After,
    the adapter returns `*SourceError{CategoryRateLimited,
    RetryAfter}` capped at 60s with a 30s default (longer than the
    Reddit/HN 5s default because YouTube blocks tend to last longer
    per the issue tracker). On HTTP 503 with body
    `{"category":"unavailable","reason":"yt-dlp signed-in challenge"}`
    the adapter returns `CategoryUnavailable` and the fanout
    (SPEC-FAN-001 REQ-FAN-003) proceeds with other adapters. True
    circuit-breaking is SPEC-EVAL-002's domain (M8). The adapter
    itself is stateless: never tracks consecutive failures, never
    opens a circuit, never sleeps in Go.
  - **D5 Score normalization**: Tanh-of-log10 formula `Score =
    clamp(0.5 + 0.5 * tanh(log10(view_count + 1) / 5.0), 0.0, 1.0)`.
    Distinct from Reddit/HN's Tanh-of-(score/100) because YouTube
    view counts span [0, ~10^10] and the Reddit/HN divisor would
    saturate every video to ~1.0. Worked examples in §2.3. The Reddit
    /HN `score.go::normalizeScore` is NOT reused; ADP-005's
    `score.go::normalizeViewScore` lives in the `youtube` package as a
    separate pure function. Open Question §11.5 tracks revisit
    triggers.
  - **D6 Korean-locale transcript handling**: when the query is
    detected as Korean (≥30% of runes in U+AC00..U+D7AF Hangul block),
    OR `Query.Filters` contains `{Key:"lang", Value:"ko"}`, the
    sidecar request body sets `transcript_lang="ko"`. The sidecar
    falls back to `en` on Korean-miss. `available_transcript_langs`
    is always populated in `Metadata` so consumers can re-request
    other locales. Aligns with the project's Korean-first posture per
    `.moai/project/tech.md:50`.
  - **D7 Pagination**: yt-dlp's `ytsearchN:` does not natively
    paginate. The adapter implements offset-based pagination by
    re-querying `ytsearch{N+offset}:<query>` and slicing the tail
    in the sidecar. Cursor format: decimal string offset (e.g.,
    `"25"` after the first 25-result page). `MaxResults + Cursor
    offset` total is capped at 100 to bound the cost (Open Question
    §11.4 reviews the cap).
  - **D8 Tests**: `net/http/httptest.Server` stub for the sidecar
    HTTP boundary plus golden JSON fixtures under
    `internal/adapters/youtube/testdata/`. NO live network calls in
    CI. NO yt-dlp subprocess invocation in Go-side tests (the
    sidecar is mocked at HTTP). The Python sidecar has its own
    pytest suite under `services/youtube-extract/tests/` with
    fixtures for yt-dlp output (sidecar tests are out of this
    SPEC's scope; see Open Question §11.7).

  Resolved discrepancy: `.moai/project/tech.md:109` row claims
  "self-throttle" for YouTube rate-limiting; this SPEC clarifies that
  the throttling lives in the sidecar via yt-dlp's `--sleep-requests`
  /`--sleep-interval` flags, NOT in the Go adapter. The Go adapter is
  stateless. The `Capabilities.RateLimitPerMin` field is set to 30
  (an empirical conservative bound assuming 1-2 sec sleep between
  yt-dlp calls); operators may tune via sidecar env vars.

  10 EARS REQs (8 × P0 + 2 × P1) covering all five EARS patterns
  (Ubiquitous, Event-Driven, State-Driven via REQ-ADP5-010 concurrent-
  safety contract, Optional via REQ-ADP5-007 Korean-lang filter,
  Unwanted via REQ-ADP5-008 empty-query rejection), 4 NFRs
  (NFR-ADP5-001 parse-path perf, NFR-ADP5-002 e2e p95 stub,
  NFR-ADP5-003 goroutine-leak, NFR-ADP5-004 sidecar subprocess
  cleanup verified by sidecar tests — see §11.7), ~42 representative
  TDD tests, 7 Open Questions carried forward from research.md §6
  for plan-auditor challenge. Zero new Go module dependencies — pure
  stdlib (`context`, `encoding/json`, `errors`, `fmt`, `io`, `math`,
  `net`, `net/http`, `net/url`, `strconv`, `strings`, `time`,
  `unicode`, `unicode/utf8`) plus existing `pkg/types` and
  `internal/obs/reqid` (nil-safe consumer). One NEW Python service
  directory (`services/youtube-extract/`) with FastAPI + uvicorn +
  yt-dlp pinned at sidecar build time.

  Insertion point: M3. SPEC-FAN-001 (M3 fanout) is the consumer of
  `(*youtube.Adapter).Search`. ADP-005 is one of the seven M3 adapter
  SPECs that develop in parallel after FAN-001 lands per
  `.moai/project/roadmap.md:122-123`. SPEC-IDX-001 (M3 RRF fusion)
  consumes the Tanh-normalised `Score` for cross-adapter ranking.
  SPEC-CACHE-001 (M3 5-phase access fallback) uses `CategoryUnavailable`
  responses to escalate to the next phase.

  Harness level: standard (single domain `internal/adapters/youtube/`,
  ≤10 Go source files + 1 new Python service directory, no
  security/payment/PII keywords beyond the cookie-file mitigation
  which is explicitly out-of-scope of v0.1's adapter Go code, no
  security review trigger because cookies live as Docker secrets
  outside Go code). Sprint Contract OPTIONAL but recommended.
  Evaluator profile `default` applies. Ready for plan-auditor review
  and annotation cycle.

---

## 1. Purpose

The M3 milestone exit criterion expands the adapter pool from M2's
two (Reddit + HN) to twelve+ (`.moai/project/roadmap.md:150` — "All
12+ adapters pass contract tests; `usearch query` returns fused
results across ≥5 adapters"). YouTube is the FIRST adapter targeting
**video** content as its native shape; every M2 / M3 adapter that
preceded it returns text-document content (posts, comments, articles).
The `pkg/types.DocTypeVideo` enum constant at
`pkg/types/capabilities.go:18` exists for exactly this case but has
not yet had an emitter. SPEC-ADP-005 fills that gap.

YouTube is uniquely valuable in the search engine because the
**transcript** is the searchable content surface — link-only references
to videos are inferior to a generic web search hit on the same topic.
Reddit/HN/arXiv/GitHub adapters return self-contained text that the
synthesis layer (SPEC-SYN-001 / SPEC-SYN-002) can cite directly. A
YouTube adapter that returns only `(title, description, channel,
view_count)` is strictly inferior — synthesis cannot quote the actual
content of a 30-minute lecture from those four fields. The transcript
unlocks YouTube as a citable source.

Two integration paths were considered (full analysis in research.md
§1):

1. **Path A — yt-dlp Python sidecar** (RECOMMENDED, adopted): yt-dlp
   wraps YouTube's internal player API + caption endpoints; returns
   full metadata via `--dump-json` and transcripts via
   `--write-auto-subs`. License: Unlicense source; subprocess
   isolation prevents GPL contagion from PyInstaller-bundled binaries.
2. **Path B — YouTube Data API v3 (`google.golang.org/api/youtube/v3`)**
   (REJECTED): clean, stable, official, but `search.list` costs 100
   quota units against a default 10,000 unit/day allocation — only
   100 searches/day, per
   https://developers.google.com/youtube/v3/getting-started. Worse,
   transcripts via `captions.download` require OAuth 2.0 + video
   ownership, per https://developers.google.com/youtube/v3/docs/captions —
   third-party transcript retrieval is effectively impossible.

The decision wins on the three highest-weight criteria identified in
research.md §1.3 — transcripts availability, daily search budget, and
Korean-locale transcript handling. The roadmap row
`.moai/project/roadmap.md:50` ("yt-dlp metadata + transcript, rate-
limit") confirms this was the design intent from the project's
inception.

The architecture commitment: ADP-005 ships a NEW Python sidecar at
`services/youtube-extract/` mirroring the established
`services/researcher/` shape (FastAPI on port 8082, multi-stage
Dockerfile, non-root user, healthcheck endpoint per
`services/researcher/Dockerfile`). The Go adapter at
`internal/adapters/youtube/` is the HTTP client that talks to this
sidecar via `POST /search` and `GET /health`. yt-dlp subprocess
invocation is fully contained in the Python sidecar; the Go side
never spawns subprocesses, never imports Python, never depends on
the cgo-Python interop bridge.

The adapter does NOT do fanout (SPEC-FAN-001 owns goroutine
dispatch), does NOT do retry (SPEC-FAN-001 owns orchestration), does
NOT do caching (SPEC-CACHE-001 owns 5-phase fallback), does NOT do
ranking fusion (SPEC-IDX-001 owns RRF), does NOT emit any metric/
log/span itself (the registry wrappedAdapter does, sole-emitter
discipline), does NOT do circuit-breaking on consecutive failures
(SPEC-EVAL-002 M8 owns adapter health-state). It DOES one job: turn
a `types.Query` into a sidecar HTTP `POST /search` request, parse
the JSON `{items: [...]}` envelope into `[]types.NormalizedDoc`, and
return them or a categorised `*types.SourceError`.

Completion contributes one of the seven M3 adapters that fanout
parallelizes across (`.moai/project/roadmap.md:123` — "All
SPEC-ADP-* (7-way) ... gated on SPEC-FAN-001"). The `DocTypeVideo`
emission unblocks SPEC-IR-001's video-intent routing for queries
that should reach YouTube specifically.

---

## 2. Scope

### 2.1 In-Scope

| Ref | Item |
|-----|------|
| a | `internal/adapters/youtube/youtube.go`: `Adapter` struct (sidecar HTTP client + base URL + user-agent + healthcheck path), `New(opts Options) (*Adapter, error)` constructor, `Name() string` returning `"youtube"`, `Capabilities() types.Capabilities` returning a deterministic descriptor (RequiresAuth=false, AuthEnvVars=nil, DocTypes=[DocTypeVideo], SupportedLangs=nil (language-agnostic at the API level; locale flows via Filters/auto-detect per REQ-ADP5-007), SupportsSince=true (yt-dlp supports `--dateafter` indirectly via post-filter, exposed as `Filters[Key="since"]`), RequiresAuth=false, AuthEnvVars=nil, RateLimitPerMin=30 (conservative bound assuming yt-dlp `--sleep-requests 1.0`; operators may tune via sidecar env), DefaultMaxResults=25, DisplayName="YouTube", Notes documenting the sidecar dependency + transcript snippet truncation + Korean-locale auto-detection + `MaxResults + Cursor offset` cap of 100), and `Healthcheck(ctx) error` (HTTP `GET <sidecarURL>/health` with caller-supplied ctx; succeeds when the sidecar returns 200 with `{"status":"ok"}` JSON). Compile-time interface assertion `var _ types.Adapter = (*Adapter)(nil)` at the bottom. |
| b | `internal/adapters/youtube/search.go`: `(*Adapter).Search(ctx, q types.Query) ([]types.NormalizedDoc, error)` — the hot path. Validates the query (empty/whitespace rejection per REQ-ADP5-008), parses any `q.Cursor` as a non-negative integer offset, applies the Korean-locale auto-detection per REQ-ADP5-007 (counts Hangul runes; sets `transcript_lang` accordingly), constructs the JSON request body for the sidecar `POST /search` endpoint (fields: `query`, `max_results`, `cursor_offset`, `since` (when filter present), `transcript_lang`, `include_transcripts=true` (always for v0.1)), executes the request via the constructed `*http.Client`, parses the response body per REQ-ADP5-005 mapping, returns `[]NormalizedDoc` or `*SourceError`. Honours `ctx` cancellation throughout. |
| c | `internal/adapters/youtube/client.go`: HTTP client construction (timeout=30s default — longer than ADP-001/ADP-002's 10s because the sidecar's yt-dlp call may legitimately take 5-15 seconds per the typical YouTube fetch pattern, especially when transcripts are included; `Transport` wrapped with `internal/obs/reqid.NewTransport(http.DefaultTransport)` for request-ID propagation), single `doRequest(req *http.Request) (*http.Response, error)` helper that sets the User-Agent header and the `Accept: application/json` header. Reuse of the `categorizeStatus` rosetta from ADP-001 (renamed for the `"youtube"` adapter field — see §6.2 for whether this is package-local helper or a shared `internal/adapters/common/` factory; default is package-local mirror to avoid premature abstraction, consistent with ADP-002's choice). NO `CheckRedirect` allowlist — the sidecar URL is operator-configured and trusted; redirect attacks come from external sources, not from the operator's own sidecar. |
| d | `internal/adapters/youtube/parse.go`: `parseSearchResponse(body []byte, retrievedAt time.Time, currentOffset int, totalReturned int) ([]types.NormalizedDoc, string, error)` — parses the sidecar's `{items: [<YTItem>...]}` JSON envelope into `[]NormalizedDoc` and returns the next-cursor offset string as the second return value. Per-item transform per the field-mapping table in §6.3. Empty `items` returns `(nil, "", nil)`. Malformed JSON returns `*SourceError{Category: CategoryPermanent, Cause: <json error>}`. Sidecar error envelope (`{"error":{"category":"...","message":"..."}}`) returns the corresponding `*SourceError` Category. |
| e | `internal/adapters/youtube/score.go`: `normalizeViewScore(viewCount int64) float64` — the Tanh-of-log10 formula `Score = clamp(0.5 + 0.5 * tanh(log10(viewCount + 1) / 5.0), 0.0, 1.0)`, deterministic, pure. Package-level constants `log10Divisor = 5.0` and `scoreCenter = 0.5` annotated with `@MX:NOTE`. |
| f | `internal/adapters/youtube/lang.go`: `detectKoreanQuery(text string) bool` — counts runes in the U+AC00..U+D7AF Hangul block; returns true when ≥30% of total runes are Hangul. Pure function. `selectTranscriptLang(text string, filters []types.Filter) string` — applies the priority: explicit filter > Korean detection > "en". |
| g | `internal/adapters/youtube/errors.go`: package-private sentinels `ErrInvalidQuery = errors.New("youtube: query text empty or whitespace-only")` (wrapped in `*SourceError{Category: CategoryPermanent}` by Search), `ErrInvalidCursor = errors.New("youtube: cursor must be non-negative integer offset")` (wrapped in `*SourceError{Category: CategoryPermanent}` by Search), `ErrCursorOverCap = errors.New("youtube: max_results + cursor offset exceeds 100")` (wrapped in `*SourceError{Category: CategoryPermanent}` per REQ-ADP5-008 / D7). The `parseRetryAfter` helper from ADP-001 is duplicated here with the YouTube-specific 30s default (60s cap unchanged) — see §6.2 for shared-helper alternatives. |
| h | `internal/adapters/youtube/youtube_test.go`: tests for Adapter interface conformance (`var _ types.Adapter` assertion via `assertInterface`), `Name()` returns `"youtube"`, `Capabilities()` returns deterministic value (called twice; equal), `Healthcheck()` succeeds against a stub `httptest.Server`, `New()` validates options. |
| i | `internal/adapters/youtube/search_test.go`: the largest test file. Drives `(*Adapter).Search` against `httptest.Server` with golden fixtures: happy path 25 videos (mix of music + lecture + Korean content), empty result, 429 with Retry-After, 503 with sidecar error envelope, 4xx, 5xx, sidecar unreachable (connection refused), `since` filter, Korean query auto-detection, explicit `lang` filter, pagination cursor round-trip, ctx cancellation mid-request, invalid cursor rejection, cursor-over-cap rejection. |
| j | `internal/adapters/youtube/client_test.go`: HTTP client unit tests — `categorizeStatus` truth table over 7 status codes, `parseRetryAfter` table over 6 input shapes (with 30s default for missing/malformed), User-Agent header presence, `Accept: application/json` header presence. |
| k | `internal/adapters/youtube/parse_test.go`: field-mapping unit tests — table over 5 fixtures (link video with transcript, video without transcript, deleted-channel video, livestream-archived video, Korean video with `ko` transcript). Asserts each NormalizedDoc field per the §6.3 mapping table. Snippet truncation discipline. View-count score normalization (5 example values). Pagination cursor round-trip. Hash field is empty (REQ-ADP5-005). Sidecar error envelope handling. |
| l | `internal/adapters/youtube/score_test.go`: `normalizeViewScore` table-driven test over 7 view-count values (`0, 1, 100, 10000, 1000000, 100000000, 10000000000`) with expected `[0.0, 1.0]` outputs computed from the formula, asserted within `±0.001`. Determinism check. Boundary: zero views → 0.5 exactly. |
| m | `internal/adapters/youtube/lang_test.go`: `detectKoreanQuery` table over 8 inputs (empty, all-English, all-Korean, mixed 50/50, mixed 30/70 just-above-threshold, mixed 25/75 just-below-threshold, all-Japanese (no Hangul), all-Chinese (no Hangul)). `selectTranscriptLang` table over 4 cases (explicit filter wins, Korean text wins when no filter, English default for non-Korean queries, mixed query falls below 30% threshold → English). |
| n | `internal/adapters/youtube/bench_test.go`: `BenchmarkParseSearchResponse25Videos` (NFR-ADP5-001 — p50 ≤ 10 ms parse time on amd64 for a 25-video sidecar response fixture; allocation ≤ 800 per response, i.e. ≤ 32 allocs per video). |
| o | `internal/adapters/youtube/testdata/`: golden JSON fixtures — `search_response.json` (25-video happy path with mixed transcripts present/absent, ~12KB — videos have richer metadata than Reddit/HN posts), `search_response_empty.json` (zero items, ~50B), `search_response_pagination.json` (offset=0, 25 items, signals more pages, ~12KB), `search_response_korean.json` (Korean-language video with `ko` transcript snippet, ~3KB), `search_response_no_transcript.json` (video with `available_transcript_langs=[]`, ~2KB), `search_response_429.json` (sidecar 429 envelope, ~150B), `search_response_503_sidecar.json` (sidecar 503 with `{"error":{"category":"unavailable","reason":"yt-dlp signed-in challenge"}}`, ~150B), `search_response_malformed.json` (truncated JSON for parse-error path, ~200B). |
| p | `services/youtube-extract/`: NEW Python sidecar directory mirroring `services/researcher/` shape. Files: `Dockerfile` (multi-stage python:3.11-slim → app, non-root, healthcheck), `pyproject.toml` (FastAPI + uvicorn + yt-dlp pinned), `README.md` (operator notes), `.env.example` (`YT_EXTRACT_PORT`, `YT_COOKIES_PATH`, `YT_USER_AGENT`, `YT_SLEEP_REQUESTS`, `YT_SLEEP_INTERVAL`, `YT_MAX_SLEEP_INTERVAL`), `src/youtube_extract/{__init__.py,__main__.py,app.py,ytdlp_runner.py,models.py}`, `tests/test_app.py`. The sidecar's HTTP contract: `GET /health` (returns `{"status":"ok","ytdlp_version":"<x.y.z>"}`), `POST /search` (request body fields per §6.4; response body `{"items":[<YTItem>...]}` or `{"error":{"category":"...","message":"..."}}`). Sidecar implementation details and tests are OUT OF SCOPE for this SPEC's TDD plan (covered separately by the sidecar's pytest suite per Open Question §11.7); ADP-005's Go-side tests mock the sidecar via httptest.Server. |

### 2.2 Out-of-Scope

[HARD] This SPEC explicitly excludes the following items. Each has a
known destination SPEC; this list prevents scope creep into ADP-005
(the M3 video adapter).

- **Per-source customisations specific to other M3 adapters** (arXiv,
  GitHub, Bluesky/X, SearXNG, Naver, Daum, KoreaNewsCrawler) →
  SPEC-ADP-003, SPEC-ADP-004, SPEC-ADP-006 through SPEC-ADP-009
  (M3, parallelized post-FAN-001).
- **Retry orchestration** (exponential backoff, circuit breaker,
  per-source health-state tracking, jitter, max-attempt counters) →
  SPEC-FAN-001 (M3). The adapter returns one categorised error per
  request and does not retry.
- **Response caching** → SPEC-CACHE-001 (M3). Each `Search` call is
  independent and idempotent at the adapter layer.
- **Result ranking fusion across adapters** → SPEC-IDX-001 (M3).
  Adapter returns yt-dlp's relevance order with `Score` in `[0.0, 1.0]`
  per the Tanh-of-log10 formula; cross-adapter ranking is fusion's job.
- **Adapter health-state machine** (auto-disable on N consecutive
  failures, auto-re-enable on Healthcheck pass, weighted reliability
  score) → SPEC-EVAL-002 (M8). Adapter is stateless; FAN-001 and
  EVAL-002 own state.
- **YouTube Data API v3 OAuth integration** (channel-owned caption
  download, owner-only metadata) → out of v0.1 scope; deferred to a
  future SPEC-ADP-005a if the sidecar path becomes operationally
  unsustainable. Path B was rejected per D1 / research.md §1.3.
- **`captions.list` / `captions.download` API integration** — out of
  scope per D1 (third-party access not granted by the API).
- **Live network integration tests in CI** → out of v0.1.
  `httptest.Server` + golden fixtures only on the Go side. Optional
  env-gated live test (`-tags=integration` + `YOUTUBE_LIVE=1`)
  deferred. Sidecar pytest tests use yt-dlp fixtures, not live
  YouTube.
- **OpenAPI / proto schema for the sidecar HTTP contract** — the JSON
  shape is documented in §6.4 and version-pinned via the sidecar
  Docker image tag. No separate IDL.
- **Subscription / playlist / channel browsing** (yt-dlp supports
  these but they are outside the search use-case) → out of v0.1.
- **Live-stream content** (in-progress streams) → yt-dlp returns
  these but `view_count` is undefined; v0.1 includes them but with
  `Score = 0.5` (zero-view neutral); documented in `Capabilities.Notes`.
- **Music vs lecture vs short-form classification** → SPEC-IR-001's
  domain. The adapter returns all video types unfiltered.
- **Full transcript inclusion in search response** — `/search` returns
  only `transcript_snippet` (first 500 chars). Full transcript via
  separate `/transcript` sidecar call; v0.1 does NOT expose this in
  the Go adapter (Open Question §11.3 tracks the synthesis
  consumer's needs).
- **`pkg/llm` integration** — the YouTube adapter does NOT call any
  LLM. Classification is the Intent Router's job (SPEC-IR-001).
- **Korean tokenisation of transcripts** → SPEC-IDX-003 (M3) handles
  ko-locale tokenization at index time. The adapter returns plain-
  text transcript snippets in whichever lang yt-dlp supplies; the
  index pipeline does the tokenization.
- **Cross-adapter helper extraction** (sharing `parseRetryAfter`,
  `categorizeStatus` between Reddit, HN, and YouTube packages) — out
  of v0.1. ADP-002 §6.2 already documents this as a "rule of three"
  refactor candidate; with three adapters now (Reddit, HN, YouTube),
  the rule is met but extraction remains a separate refactor SPEC
  (SPEC-ADP-REFAC-001) not bundled here.
- **Per-adapter custom Prometheus metrics** (e.g.,
  `youtube_transcript_fetch_seconds`) — would require amending
  SPEC-OBS-001's allowlist; out of scope. The shared
  `AdapterCalls{adapter="youtube",outcome}` family is sufficient.
- **GitHub Issue tracking on this SPEC** (skipped per session pattern).
- **Sidecar implementation details and Python tests** — ADP-005's
  TDD plan covers ONLY the Go adapter side. The Python sidecar at
  `services/youtube-extract/` is contractually documented in §6.4
  but its internal implementation, pytest suite, Dockerfile content,
  and operational runbook are tracked separately (Open Question §11.7
  schedules the sidecar implementation milestone alongside this
  SPEC's run phase).

### 2.3 Score Normalization Formula (Architecture)

[HARD] The score normaliser in `score.go::normalizeViewScore(viewCount
int64) float64` is a deterministic pure function so that golden tests
can compute expected `NormalizedDoc.Score` values from the input alone
and downstream ranking (SPEC-IDX-001 RRF) gets a stable input. Distinct
from Reddit/HN's Tanh-of-(score/100) formula because YouTube view
counts span [0, ~10^10] and the Reddit/HN divisor would saturate
every video to ~1.0.

**Formula**:

```
Score = clamp(0.5 + 0.5 * tanh(log10(viewCount + 1) / 5.0), 0.0, 1.0)
```

where `tanh` is the standard hyperbolic tangent (`math.Tanh` in Go's
stdlib), `log10` is the base-10 logarithm (`math.Log10`), `viewCount + 1`
prevents `log10(0)` undefined behaviour (the +1 trick), and
`clamp(x, lo, hi) = max(lo, min(hi, x))`.

**Properties**:

- **Domain**: YouTube `view_count` ∈ `[0, 2^63)` int64 (yt-dlp surfaces
  the value as an integer; YouTube's signed-int storage tops out below
  10^11 in practice).
- **Codomain**: `Score` ∈ `[0.0, 1.0]` (mathematically `[0.5, 1.0]`
  from `tanh` of non-negative input; clamp is defensive against
  floating-point edge cases).
- **Symmetry**: `viewCount = 0` → `log10(1) = 0` → `tanh(0) = 0` →
  `Score = 0.5` (neutral; matches Reddit/HN semantics for
  no-engagement).
- **Inflection**: `viewCount = 100,000` → `log10(100001) ≈ 5.0` →
  `tanh(1.0) ≈ 0.762` → `Score ≈ 0.881`. Mid-popular videos sit just
  above the 0.85 threshold.
- **Saturation**: `viewCount = 10,000,000,000` → `log10(10^10+1) ≈ 10.0`
  → `tanh(2.0) ≈ 0.964` → `Score ≈ 0.982`, NOT 1.0. The most-viewed
  video (Despacito ~8.5B as of 2026) maps to ≈0.98, leaving headroom
  versus 1B-view videos at ≈0.97.
- **Determinism**: pure function, no state, no time, no I/O.

**Worked examples** (computed from the formula, asserted in
`score_test.go::TestNormalizeViewScoreTable` within `±0.001`
tolerance):

| view_count       | log10(v+1) | tanh(log10(v+1)/5) | Score    |
|------------------|------------|--------------------|----------|
| 0                | 0.000      | 0.000              | 0.500000 |
| 1                | 0.301      | 0.060              | 0.530080 |
| 100              | 2.004      | 0.380              | 0.690118 |
| 10,000           | 4.000      | 0.664              | 0.831850 |
| 1,000,000        | 6.000      | 0.834              | 0.916988 |
| 100,000,000      | 8.000      | 0.927              | 0.963664 |
| 10,000,000,000   | 10.000     | 0.964              | 0.982014 |

**Tie-break behaviour**: Two videos with equal `view_count` produce
equal `NormalizedDoc.Score`. Order is preserved from yt-dlp's
relevance ranking (yt-dlp's `ytsearchN:` returns YouTube's relevance
order; `Score` does not re-sort). SPEC-IDX-001 RRF uses rank not
score for fusion across adapters, so equal scores within YouTube do
not cause ranking instability.

**Rationale (why log10 over linear or Tanh-of-views directly)**
(research §3.2):

- View counts span 10 orders of magnitude (0 → 10B); linear scaling
  saturates at the head and gives no signal in the body.
- `Tanh(views/100)` (the Reddit/HN formula) saturates at views=1000;
  every popular video maps to ~1.0; ranking signal collapses.
- log10 spreads the [0, 10B] range linearly across [0, 10] which Tanh
  can then squish into [0.5, 1.0] with meaningful gradient at every
  decade.
- The divisor=5.0 is empirical; revisit triggers in Open Question
  §11.5 after SPEC-IDX-001 RRF measures ranking quality.

The formula is intentionally locked in v0.1 — changing it later
requires a major-version bump of `Capabilities.Notes` and coordination
with SPEC-IDX-001's RRF tuning. Open Question §11.5 documents
revisit triggers.

---

## 3. EARS Requirements

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| REQ-ADP5-001 | Ubiquitous | The package `internal/adapters/youtube` SHALL expose an `Adapter` struct that implements `pkg/types.Adapter` exactly: `Name() string` returning `"youtube"`, `Search(ctx context.Context, q types.Query) ([]types.NormalizedDoc, error)`, `Healthcheck(ctx context.Context) error`, `Capabilities() types.Capabilities`. The package SHALL include a compile-time interface assertion `var _ types.Adapter = (*Adapter)(nil)`. `Capabilities()` SHALL be deterministic (two consecutive calls return equal values) with `SourceID="youtube"`, `DisplayName="YouTube"`, `DocTypes=[DocTypeVideo]`, `SupportedLangs=nil`, `SupportsSince=true`, `RequiresAuth=false`, `AuthEnvVars=nil`, `RateLimitPerMin=30`, `DefaultMaxResults=25`, and `Notes` containing the substrings `"yt-dlp Python sidecar"`, `"public no-auth"`, `"transcript snippet truncated"`, `"Korean-locale auto-detection"`, and `"max_results + cursor offset cap 100"`. `Healthcheck(ctx)` SHALL issue an HTTP `GET <sidecarURL>/health` request with the supplied ctx; success requires HTTP 200 with response body parseable as `{"status":"ok",...}`. | P0 | `TestAdapterName`, `TestAdapterImplementsInterface` (compile-time), `TestCapabilitiesDeterministic`, `TestCapabilitiesShape` (asserts all 9 documented field values + 5 Notes substring matches), `TestHealthcheckSucceeds` (stub `httptest.Server` returns `{"status":"ok","ytdlp_version":"2026.04.01"}` on `/health`), `TestHealthcheckFailsOn503` (stub returns 503 → error), `TestHealthcheckFailsOnMalformedJSON` (stub returns 200 with non-JSON → error). All in `internal/adapters/youtube/youtube_test.go`. |
| REQ-ADP5-002 | Event-Driven | WHEN `(*Adapter).Search(ctx, q)` is invoked with a non-empty `q.Text`, the adapter SHALL build an HTTP POST request to `<sidecarBaseURL>/search` with `Content-Type: application/json` and a JSON body containing the following fields: `query=q.Text`, `max_results=clamp(q.MaxResults, 1, 100)` (defaulting to 25 when `q.MaxResults == 0`), `cursor_offset=<parsed cursor as int, default 0>` (the field is ALWAYS present in the request body; value 0 is the no-cursor signal corresponding to "first page"; non-zero values come from a previously-surfaced `Metadata["next_cursor"]`), `transcript_lang=<selected per REQ-ADP5-007>`, `include_transcripts=true`, and optionally `since=<unix-seconds>` (when `Filters[Key="since"]` is present and parses as positive int64). The adapter SHALL execute the request via the constructed `*http.Client`, parse the JSON `items` envelope per REQ-ADP5-005 mapping, and return `(docs, nil)` on HTTP 200 with `len(docs) ≤ 100`. | P0 | `TestSearchHappyPath25Videos` (httptest.Server returns `search_response.json`; assert 25 NormalizedDocs returned, each with all required fields populated and `Validate()` returning nil); `TestSearchRequestBodyIncludesAllRequired` (decode captured request body; assert `query`, `max_results`, `cursor_offset`, `transcript_lang`, `include_transcripts=true` ALL ALWAYS present — including `cursor_offset=0` when no cursor was supplied by the caller); `TestSearchClampsMaxResultsTo100` (q.MaxResults=500 → request body has `max_results=100`); `TestSearchDefaultsMaxResultsTo25` (q.MaxResults=0 → request body has `max_results=25`); `TestSearchEmptyCursorSendsZero` (q.Cursor="" → request body has `cursor_offset=0`; the field is present and zero, NOT absent); `TestSearchSetsCursorWhenPresent` (q.Cursor="25" → request body has `cursor_offset=25`); `TestSearchSetsContentTypeJSON` (assert request `Content-Type: application/json`). All in `search_test.go`. |
| REQ-ADP5-003 | Event-Driven | WHEN HTTP 429 is received from the sidecar endpoint, the adapter SHALL parse the `Retry-After` response header per RFC 7231 §7.1.3 (integer-seconds OR HTTP-date), cap the result at 60 seconds (any larger value is replaced with 60s), default to 30 seconds when the header is missing or malformed (longer than the Reddit/HN 5s default because YouTube blocks tend to last longer per https://github.com/yt-dlp/yt-dlp/issues/10128), and return `(nil, &types.SourceError{Adapter:"youtube", Category: types.CategoryRateLimited, HTTPStatus: 429, RetryAfter: <duration>, Cause: errors.New("youtube: rate limited")})`. The adapter SHALL NOT retry internally. | P0 | `TestSearchHTTP429WithIntegerRetryAfter` (`Retry-After: 30` → RetryAfter=30s); `TestSearchHTTP429WithHTTPDateRetryAfter` (HTTP-date 30s in future → RetryAfter ∈ (25s, 35s)); `TestSearchHTTP429NoRetryAfterDefaults30s` (no header → RetryAfter=30s); `TestSearchHTTP429RetryAfterCapped60s` (`Retry-After: 999` → 60s); `TestSearchHTTP429NoInternalRetry` (assert exactly 1 outbound request observed). All in `search_test.go` + `client_test.go`. |
| REQ-ADP5-004 | Event-Driven | WHEN HTTP 401, 403, 404, or any 4xx other than 429 is received from the sidecar, the adapter SHALL return `(nil, &types.SourceError{Adapter:"youtube", Category: types.CategoryPermanent, HTTPStatus: <code>, Cause: errors.New("youtube: permanent failure: <code>")})`. WHEN HTTP 500/502/503/504 is received OR a connection error occurs (DNS failure, dial timeout, read timeout, TLS handshake failure, connection refused — sidecar down), the adapter SHALL return `(nil, &types.SourceError{Adapter:"youtube", Category: types.CategoryUnavailable, HTTPStatus: <code or 0>, Cause: <inner error>})`. WHEN HTTP 503 is received with response body matching the sidecar error envelope `{"error":{"category":"unavailable","reason":"yt-dlp signed-in challenge"}}`, the adapter SHALL still return `CategoryUnavailable` (not propagate the inner reason as a different Category) but SHALL preserve the inner reason in the `Cause` chain so operators can diagnose via `errors.Unwrap`. Network-layer errors set `HTTPStatus=0`. The adapter SHALL NOT retry. | P0 | `TestSearchHTTP4xx` (table over 401/403/404 → ErrPermanent + matching HTTPStatus); `TestSearchHTTP5xx` (table over 500/503/504 → ErrSourceUnavailable + matching HTTPStatus); `TestSearchSidecarUnreachable` (httptest.Server closed before request; assert `errors.Is(err, types.ErrSourceUnavailable)` and `HTTPStatus=0`); `TestSearchSidecarYtdlpChallenge` (stub returns 503 with `{"error":{"category":"unavailable","reason":"yt-dlp signed-in challenge"}}`; assert ErrSourceUnavailable AND `errors.Unwrap(srcErr).Error()` contains "yt-dlp signed-in challenge"); `TestSearchUnavailablePreservesUnderlyingError` (assert `errors.Unwrap(srcErr)` non-nil and inner cause text contains the error chain). All in `search_test.go` + `client_test.go`. |
| REQ-ADP5-005 | Ubiquitous | The adapter SHALL transform each item in the sidecar's `items` array into one `types.NormalizedDoc` using the field mapping in §6.3, MUST set `RetrievedAt = time.Now().UTC()` at the moment of parsing, MUST leave `Hash = ""` (consumers compute via `CanonicalHash()`), MUST populate `Metadata` with at minimum the keys `{channel_id, channel_url, duration_seconds, view_count, thumbnail_url, available_transcript_langs}`, MUST set `DocType = types.DocTypeVideo`, MUST set `Lang` per the priority order: explicit `Filters[Key="lang"].Value` > Korean-detection result (per REQ-ADP5-007) > the sidecar-supplied `transcript_lang` of the snippet (when present) > `""` (unknown). When `transcript_snippet` is non-empty, the adapter SHALL include it in `Metadata["transcript_snippet"]` (UTF-8 plain text, capped at 500 runes). When `view_count` is null/missing (livestream-archived edge case), the adapter SHALL treat it as 0 (yields `Score = 0.5`). The next-cursor offset (when `currentOffset + len(items) < 100` AND the sidecar response signals more pages via `has_more=true`) SHALL be returned as the second return value of `parseSearchResponse` so `Search` can surface it via `Metadata["next_cursor"]` on the LAST returned NormalizedDoc — encoded as `strconv.Itoa(currentOffset + len(items))`. Items where the sidecar reports a per-item `error` field (some videos may fail to extract while others succeed in the same response) SHALL be skipped silently — they SHALL NOT appear in the returned slice. The adapter SHALL NOT emit any log record for skipped items (sole-emitter discipline); the operator-visible signal is the delta between the sidecar's request `max_results` and the returned `len(docs)`, which the wrappedAdapter's `result_count` attribute surfaces in the per-call slog/span. | P0 | `TestParseSearchResponseFieldMapping` (table-driven over 5 fixtures: link video with transcript, video without transcript, deleted-channel video, livestream-archived (null view_count), Korean video with `ko` transcript); `TestParseSearchResponseSelectsKoreanLang` (Korean fixture → returned doc has `Lang="ko"`); `TestParseSearchResponseIncludesTranscriptSnippet` (fixture with non-empty `transcript_snippet` → returned doc has `Metadata["transcript_snippet"]` non-empty AND len ≤ 500 runes); `TestParseSearchResponsePaginationCursor` (fixture with `has_more=true`, currentOffset=0, 25 items → returned NormalizedDocs[len-1].Metadata["next_cursor"] == "25"); `TestParseSearchResponseNoCursorOnLastPage` (fixture with `has_more=false` → no doc has `next_cursor` key); `TestParseSearchResponseHashEmpty` (every returned doc has `Hash == ""`); `TestParseSearchResponseMetadataKeys` (all 6 required keys present in each returned doc); `TestParseSearchResponseSkipsItemsWithError` (fixture with mixed success+error items → only success items returned; assert returned slice contains exactly the expected count and NO log record is emitted by the youtube package itself — verified by injecting a slog handler that fails the test on any record from package "youtube"). All in `parse_test.go`. |
| REQ-ADP5-006 | Ubiquitous | The adapter SHALL set the `User-Agent` HTTP header on every outbound request to a non-default value of the form `usearch/<version> (+https://github.com/elymas/universal-search)` where `<version>` is supplied via `Options.UserAgentVersion` (default `"v0.1"`). The adapter SHALL set the `Accept` header to `application/json` and the `Content-Type` header to `application/json` for the POST body. The User-Agent identifies traffic for sidecar operational debugging and matches the project-wide convention from ADP-001 REQ-ADP-009 and ADP-002 REQ-ADP2-006. | P0 | `TestSearchSetsCustomUserAgent` (inspect captured `r.Header.Get("User-Agent")`; assert it starts with `"usearch/"` and contains `"(+https://github.com/elymas/universal-search)"`); `TestSearchSetsAcceptJSON` (assert `Accept: application/json`); `TestSearchSetsContentTypeJSON` (assert `Content-Type: application/json`); `TestSearchUserAgentVersionConfigurable` (Options.UserAgentVersion="v0.2-rc1" → header contains `"usearch/v0.2-rc1"`). All in `client_test.go`. |
| REQ-ADP5-007 | Optional | WHERE `Query.Filters` contains an entry with `Key == "lang"` AND `Value` is a non-empty string of 2-8 ASCII characters (loose BCP-47 acceptance — "ko", "en", "ja", "zh-CN" all valid; the adapter does not strictly validate against IANA registry), the adapter SHALL set the sidecar request body's `transcript_lang` field to that value verbatim AND SHALL set the returned `NormalizedDoc.Lang` to that value. WHERE no explicit `lang` filter is present BUT the query text triggers Korean-detection (≥30% of runes in the U+AC00..U+D7AF Hangul block, per `lang.go::detectKoreanQuery`), the adapter SHALL set the sidecar request body's `transcript_lang` to `"ko"` automatically. WHERE neither condition holds, the adapter SHALL set `transcript_lang="en"`. WHERE `Filters` contains an entry with `Key == "since"` AND `Value` parses as a positive Unix-seconds integer, the adapter SHALL include `since=<value>` in the sidecar request body. Filter keys other than `"lang"` and `"since"` SHALL be silently ignored (no error returned). Malformed filter values SHALL be silently dropped (no error, no field added). | P1 | `TestSearchExplicitLangFilterWins` (Filters=[{lang, "ja"}] + Korean text → request body has `transcript_lang="ja"`; returned doc has `Lang="ja"`); `TestSearchKoreanAutoDetection` (no filter, q.Text="안녕하세요 이것은 한국어 쿼리입니다" → request body has `transcript_lang="ko"`); `TestSearchEnglishDefaultForLatinScript` (no filter, q.Text="hello world" → request body has `transcript_lang="en"`); `TestSearchKoreanThresholdBoundary` (q.Text with 29% Hangul → English; q.Text with 31% Hangul → Korean); `TestSearchSinceFilterAdded` (Filters=[{since, "1700000000"}] → request body has `since=1700000000`); `TestSearchSinceFilterMalformedDropped` (Filters=[{since, "abc"}] → request body has no `since` field); `TestSearchUnknownFilterIgnored` (Filters=[{nsfw, "true"}] → request body has neither `nsfw` nor `transcript_lang="true"`); `TestSearchEmptyLangValueDropsToDefault` (Filters=[{lang, ""}] → request body has `transcript_lang="en"`); `TestSearchInvalidLangFormatRejected` (Filters=[{lang, "verylongstring"}] → request body has `transcript_lang="en"`; the adapter does not crash on malformed input). All in `search_test.go` + `lang_test.go`. |
| REQ-ADP5-008 | Unwanted | IF `Query.Text` is empty OR contains only Unicode whitespace runes (per `unicode.IsSpace` over every rune), THEN the adapter SHALL return `(nil, &types.SourceError{Adapter:"youtube", Category: types.CategoryPermanent, Cause: ErrInvalidQuery})` immediately and SHALL NOT issue any HTTP request. IF `Query.Cursor` is non-empty AND does NOT parse as a non-negative integer via `strconv.Atoi` (negative integers also rejected), THEN the adapter SHALL return `(nil, &types.SourceError{Adapter:"youtube", Category: types.CategoryPermanent, Cause: ErrInvalidCursor})` immediately and SHALL NOT issue any HTTP request. IF `clamp(q.MaxResults, 1, 100) + cursorOffset > 100` (strictly greater than; exactly 100 is permitted), THEN the adapter SHALL return `(nil, &types.SourceError{Adapter:"youtube", Category: types.CategoryPermanent, Cause: ErrCursorOverCap})` immediately and SHALL NOT issue any HTTP request — this enforces the D7 pagination cap (yt-dlp's `ytsearchN:` becomes inefficient beyond N=100, see research §3.3 and Open Question §11.4). The sentinel `ErrCursorOverCap` SHALL have message text `"youtube: max_results + cursor offset > 100"` (using `>` to disambiguate from the inclusive boundary). [Precedence note: when ctx is already cancelled at function entry, REQ-ADP5-009 takes precedence over this REQ — the adapter returns `CategoryUnavailable` with `ctx.Err()` as Cause, NOT a validation error, even if the query is also empty or the cursor is malformed.] | P0 | `TestSearchEmptyQueryRejectedNoHTTP` (table over `["", "   ", "\t\n  \r"]` for `q.Text`; for each asserts `errors.Is(err, types.ErrPermanent)` AND `errors.Is(err, ErrInvalidQuery)` AND assert httptest.Server received zero requests); `TestSearchInvalidCursorRejectedNoHTTP` (table over `["abc", "-1", "1.5", "1e3", " 25"]` for `q.Cursor`; for each asserts `errors.Is(err, types.ErrPermanent)` AND `errors.Is(err, ErrInvalidCursor)` AND zero requests); `TestSearchCursorOverCapRejected` (q.MaxResults=50 + q.Cursor="60" → 110 > 100 → `errors.Is(err, ErrCursorOverCap)` AND zero requests; q.MaxResults=25 + q.Cursor="75" → 100 == 100 → request IS issued (cap is inclusive of 100)); `TestSearchCtxPrecedenceOverValidation` (ctx pre-cancelled AND q.Text="" — assert returned err satisfies `errors.Is(err, context.Canceled)` AND `errors.Is(err, types.ErrSourceUnavailable)`, NOT `ErrInvalidQuery`/`ErrPermanent`). All in `search_test.go`. |
| REQ-ADP5-009 | Event-Driven | WHEN the caller-supplied ctx is cancelled OR fires its deadline mid-request, the adapter SHALL release any goroutine, HTTP connection, and timer resources cleanly: the in-flight HTTP request inherits the ctx and aborts via stdlib's standard cancellation propagation; `defer resp.Body.Close()` runs; no goroutine remains after the call returns. WHEN ctx is cancelled BEFORE any HTTP request is constructed (an entry-time cancellation), the adapter SHALL return `(nil, &types.SourceError{Adapter:"youtube", Category: types.CategoryUnavailable, Cause: ctx.Err()})` without issuing any request. The adapter SHALL NOT mask context errors with its own categorisation when the underlying cause is `context.DeadlineExceeded` or `context.Canceled` — the wrapped error chain SHALL satisfy `errors.Is(err, ctx.Err())`. | P0 | `TestSearchCtxCancelledMidFlight` (httptest.Server delays response 200ms; cancel ctx at 50ms; assert `errors.Is(err, types.ErrSourceUnavailable)` AND `errors.Is(err, context.Canceled)`); `TestSearchCtxAlreadyCancelled` (ctx cancelled before Search call; assert ErrSourceUnavailable AND `errors.Is(err, context.Canceled)` AND zero requests observed); `TestSearchCtxDeadlineExceeded` (ctx with 50ms deadline; httptest.Server delays 200ms; assert ErrSourceUnavailable AND `errors.Is(err, context.DeadlineExceeded)`); `TestSearchNoGoroutineLeakOnCancel` covers NFR-ADP5-003 and is referenced from this REQ. All in `search_test.go`. |
| REQ-ADP5-010 | State-Driven | WHILE the same `*Adapter` instance is registered in the adapter registry and is being invoked concurrently from N goroutines (N ≥ 1), each `Search(ctx, q)` call SHALL execute independently with no shared mutable state across calls (the `*Adapter` struct is immutable post-construction; the underlying `*http.Client` is goroutine-safe per Go stdlib; the `lang.go::detectKoreanQuery` and `score.go::normalizeViewScore` helpers are pure functions); the cumulative effect SHALL be N independent HTTP round-trips with no race-detector alarms. This requirement crystallises the concurrency contract that the registry (`internal/adapters/registry.go:172-263` wrappedAdapter) and the future fanout layer (SPEC-FAN-001 NFR-FAN-002) rely on. | P0 | `TestSearchConcurrentSafe` (50 goroutines each issuing one Search against the same httptest.Server; assert (a) no race-detector alarm under `-race`, (b) total response count = 50 observed at the stub, (c) all 50 returned slices are `[]types.NormalizedDoc` with `Validate()` returning nil for every doc). In `search_test.go`. |

---

## 4. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| NFR-ADP5-001 | Performance (parse path) | The parse path `parseSearchResponse(body []byte, retrievedAt time.Time, currentOffset int, totalReturned int) ([]NormalizedDoc, string, error)` SHALL execute with mean wall-clock duration per op ≤ 10 ms over `go test -bench=BenchmarkParseSearchResponse25Videos -benchtime=10x -count=5 ./internal/adapters/youtube/...` on amd64; the median of the 5 runs is the assertion value (passes when ≤ 10 ms). The fixture is the `search_response.json` golden (25-video sidecar response, ~12KB). The 10ms target is HIGHER than the Reddit/HN 5ms target because YouTube items have richer metadata (transcript snippet up to 500 runes, larger description bodies, more Metadata keys per item — channel, duration, views, thumbnails, transcript langs, plus optional like_count/tags/transcript fields). Allocation count ≤ 800 per 25-item response (≤ 32 allocs per video) per the same benchmark's `allocs/op` field — also higher than Reddit/HN's 500 because of the ~500-rune transcript snippet copy plus the available_transcript_langs []string allocation per item. The same `pkg/types.NormalizedDoc.Metadata = map[string]any` floor analysis from SPEC-ADP-001 NFR-ADP-001 applies (~17 allocs/doc structural floor); YouTube's richer Metadata adds 3-5 allocs/doc on top. Run weekly in CI per the cadence established in SPEC-OBS-001 NFR-OBS-001. Benchmarks do not count toward coverage. |
| NFR-ADP5-002 | End-to-end Latency | The end-to-end `Search` round-trip against the `httptest.Server` stub (no real network, no real sidecar) SHALL complete with p95 ≤ 200 ms over 100 invocations, measured by `TestSearchE2ELatencyStubP95` in `search_test.go` (sort durations ascending, assert `durations[94] ≤ 200ms`). The harder live-sidecar p95 (≤ 15s — yt-dlp's typical full extraction with transcript can take 5-15s) is documented as the operational target but is NOT enforced in CI (no live sidecar). The Go-side adapter parse + HTTP overhead alone is bounded by the stub p95 ≤ 200ms; the bulk of live-call latency lives in the sidecar (yt-dlp subprocess + YouTube fetch). |
| NFR-ADP5-003 | No goroutine leak on cancellation | The adapter SHALL NOT leak any goroutine when the caller's context is cancelled mid-`Search`. Verified by `TestSearchNoGoroutineLeakOnCancel` in `search_test.go`, which uses `go.uber.org/goleak.VerifyNone(t)` after a `Search` call whose ctx is cancelled mid-flight via a 50ms-delayed cancel; assert zero residual goroutines after the call returns. `TestMain` in `bench_test.go` SHALL invoke `goleak.VerifyTestMain(m)` (mirrors `internal/adapters/reddit/bench_test.go` and `internal/adapters/hn/bench_test.go` patterns). |
| NFR-ADP5-004 | Race-clean concurrent invocation | `internal/adapters/youtube/search_test.go::TestSearchConcurrentSafe` SHALL execute successfully under `go test -race ./internal/adapters/youtube/...` with the workload defined in REQ-ADP5-010: 50 caller goroutines each issuing one `Search` call against the same stub registry. Race-detector alarms attributable to the youtube package SHALL be zero. The `*Adapter` struct's immutability post-construction (no mutable fields), pure-function helpers (`detectKoreanQuery`, `normalizeViewScore`), and the goroutine-safe `*http.Client` from stdlib provide the foundation for race-cleanness; the test asserts the empirical guarantee. |

---

## 5. Acceptance Criteria

### REQ-ADP5-001 — Adapter Interface Conformance

- File `internal/adapters/youtube/youtube.go` declares `Adapter`
  struct with the documented fields (`httpClient *http.Client`,
  `baseURL string`, `userAgent string`, `healthcheckPath string` ==
  `"/health"`).
- The compile-time assertion `var _ types.Adapter = (*Adapter)(nil)`
  appears at the bottom of `youtube.go`. If the interface ever
  drifts, this assertion fails to compile.
- `(*Adapter).Name()` returns the literal string `"youtube"`.
- `(*Adapter).Capabilities()` returns a `types.Capabilities` with:
  - `SourceID = "youtube"`
  - `DisplayName = "YouTube"`
  - `DocTypes = []types.DocType{types.DocTypeVideo}`
  - `SupportedLangs = nil` (locale flows via Filters/auto-detect per
    REQ-ADP5-007)
  - `SupportsSince = true` (yt-dlp `since` filter is exposed)
  - `RequiresAuth = false`
  - `AuthEnvVars = nil`
  - `RateLimitPerMin = 30`
  - `DefaultMaxResults = 25`
  - `Notes` contains the substrings `"yt-dlp Python sidecar"`,
    `"public no-auth"`, `"transcript snippet truncated"`,
    `"Korean-locale auto-detection"`, and `"max_results + cursor
    offset cap 100"`.
- `(*Adapter).Healthcheck(ctx)` succeeds against an httptest.Server
  whose `/health` route returns `{"status":"ok","ytdlp_version":
  "2026.04.01"}`. Tests construct the Adapter with
  `Options{BaseURL: <httptest.Server.URL>}` to redirect to the stub.
- `Healthcheck(ctx)` fails with non-nil error when:
  - HTTP status ≠ 200 (`TestHealthcheckFailsOn503`)
  - Response body is not parseable JSON
    (`TestHealthcheckFailsOnMalformedJSON`)
  - Response body's `status` field ≠ `"ok"`
    (`TestHealthcheckFailsOnStatusNotOk`)
  - Sidecar is unreachable (connection refused)
    (`TestHealthcheckFailsOnUnreachable`)
- `TestAdapterName`, `TestAdapterImplementsInterface`,
  `TestCapabilitiesDeterministic`, `TestCapabilitiesShape`,
  `TestHealthcheckSucceeds`, plus the four failure-mode tests above
  all pass.

### REQ-ADP5-002 — Search Happy Path

- `TestSearchHappyPath25Videos` against
  `testdata/search_response.json` returns exactly 25 `NormalizedDoc`
  entries (mix of music + lecture + Korean videos); each passes
  `Validate()` returning nil; the captured request body decodes to a
  JSON object containing `query`, `max_results=25`,
  `transcript_lang`, `include_transcripts=true`.
- `TestSearchRequestBodyIncludesAllRequired`,
  `TestSearchClampsMaxResultsTo100`,
  `TestSearchDefaultsMaxResultsTo25`,
  `TestSearchOmitsCursorWhenEmpty` (q.Cursor="" → request body has
  `cursor_offset=0`; the field is always present but zero is the
  no-cursor signal — documented in §6.4),
  `TestSearchSetsCursorWhenPresent` (q.Cursor="25" → request body has
  `cursor_offset=25`),
  `TestSearchSetsContentTypeJSON` all pass.

### REQ-ADP5-003 — HTTP 429 Rate-Limit Mapping

- `TestSearchHTTP429WithIntegerRetryAfter` asserts returned err is
  `*types.SourceError` with `Category=CategoryRateLimited`,
  `HTTPStatus=429`, `RetryAfter=30s`.
- `TestSearchHTTP429WithHTTPDateRetryAfter` parses an HTTP-date 30s
  in the future; asserts `RetryAfter` is in `(25s, 35s)` (allowing
  test-clock drift).
- `TestSearchHTTP429NoRetryAfterDefaults30s` (no header) asserts
  `RetryAfter=30s` — distinct from Reddit/HN's 5s default.
- `TestSearchHTTP429RetryAfterCapped60s` (`Retry-After: 999`) asserts
  `RetryAfter=60s`.
- `TestSearchHTTP429NoInternalRetry` instruments the httptest.Server
  with a request counter; asserts exactly 1 request observed.

### REQ-ADP5-004 — HTTP 4xx/5xx and Sidecar Failure Mapping

- `TestSearchHTTP4xx` table-drives 401, 403, 404; each asserts
  `errors.Is(err, types.ErrPermanent)` and matching HTTPStatus.
- `TestSearchHTTP5xx` table-drives 500, 503, 504; each asserts
  `errors.Is(err, types.ErrSourceUnavailable)` and matching
  HTTPStatus.
- `TestSearchSidecarUnreachable` (httptest.Server closed before
  request) asserts `errors.Is(err, types.ErrSourceUnavailable)` and
  `HTTPStatus=0`.
- `TestSearchSidecarYtdlpChallenge`: stub returns 503 with body
  `{"error":{"category":"unavailable","reason":"yt-dlp signed-in
  challenge"}}`. Assert `errors.Is(err, types.ErrSourceUnavailable)`
  AND `errors.Unwrap(srcErr).Error()` contains `"yt-dlp signed-in
  challenge"`.
- `TestSearchUnavailablePreservesUnderlyingError`: assert
  `errors.Unwrap(srcErr) != nil` and the inner error message contains
  the network cause text.

### REQ-ADP5-005 — NormalizedDoc Field Mapping

- `TestParseSearchResponseFieldMapping` table-drives 5 fixtures (link
  video with transcript, video without transcript, deleted-channel
  video, livestream-archived (null view_count), Korean video with
  `ko` transcript). For each, asserts every NormalizedDoc field per
  the §6.3 mapping table (ID, SourceID, URL, Title, Body, Snippet,
  PublishedAt, RetrievedAt non-zero, Author, Score within
  `[normalizeViewScore(view_count) ± 0.001]`, Lang per priority
  rules, DocType=DocTypeVideo, Citations=nil, Metadata keys present).
- `TestParseSearchResponseSelectsKoreanLang`: Korean fixture (sidecar
  returned `transcript_lang="ko"`) → returned doc has `Lang="ko"`.
- `TestParseSearchResponseIncludesTranscriptSnippet`: fixture with
  non-empty `transcript_snippet` (≤500 runes) → returned doc has
  `Metadata["transcript_snippet"]` non-empty AND len(runes) ≤ 500.
- `TestParseSearchResponseTruncatesOverlongTranscript`: fixture with
  `transcript_snippet` returned by sidecar of 1000 runes → adapter
  truncates to 500 (defensive against sidecar contract drift).
- `TestParseSearchResponsePaginationCursor`: fixture with
  `has_more=true`, currentOffset=0, 25 items → returned
  `NormalizedDocs[len-1].Metadata["next_cursor"] == "25"`. Earlier
  docs do NOT have the `next_cursor` key.
- `TestParseSearchResponseNoCursorOnLastPage`: fixture with
  `has_more=false` → no doc has `next_cursor` key on any of them.
- `TestParseSearchResponseHashEmpty`: every returned
  `NormalizedDoc.Hash` equals `""`.
- `TestParseSearchResponseMetadataKeys`: each returned doc's Metadata
  has at least `{channel_id, channel_url, duration_seconds,
  view_count, thumbnail_url, available_transcript_langs}`.
- `TestParseSearchResponseSkipsItemsWithError`: fixture with mixed
  successes + per-item errors → only successful items returned;
  errored items NOT in the slice; total returned count matches
  successful-items count.
- `TestParseSearchResponseLivestreamNullViewCount`: fixture with
  `view_count: null` → returned doc has `Score=0.5` (normalizeViewScore
  treats null as 0).

### REQ-ADP5-006 — User-Agent and Accept Headers

- `TestSearchSetsCustomUserAgent`: captured request header
  `User-Agent` starts with `"usearch/"` and contains
  `"(+https://github.com/elymas/universal-search)"`.
- `TestSearchSetsAcceptJSON`: captured `Accept` header equals
  `"application/json"`.
- `TestSearchSetsContentTypeJSON`: captured `Content-Type` header
  equals `"application/json"`.
- `TestSearchUserAgentVersionConfigurable`: `Options.UserAgentVersion
  = "v0.2-rc1"` → captured `User-Agent` contains `"usearch/v0.2-rc1"`.

### REQ-ADP5-007 — Lang and Since Filters with Korean Auto-Detection

- `TestSearchExplicitLangFilterWins`:
  `Filters=[{lang, "ja"}]` + Korean text → request body has
  `transcript_lang="ja"`; returned doc has `Lang="ja"`.
- `TestSearchKoreanAutoDetection`:
  no filter, q.Text=`"안녕하세요 이것은 한국어 쿼리입니다"` (>30%
  Hangul) → request body has `transcript_lang="ko"`.
- `TestSearchEnglishDefaultForLatinScript`: no filter,
  q.Text=`"hello world"` → request body has `transcript_lang="en"`.
- `TestSearchKoreanThresholdBoundary`: q.Text with exactly 29% Hangul
  runes → English; q.Text with exactly 31% Hangul runes → Korean.
- `TestSearchSinceFilterAdded`:
  `Filters=[{since, "1700000000"}]` → request body has
  `since=1700000000`.
- `TestSearchSinceFilterMalformedDropped`: `Filters=[{since, "abc"}]`
  → request body has no `since` field (no error).
- `TestSearchSinceFilterNegativeDropped`:
  `Filters=[{since, "-100"}]` → request body has no `since` field.
- `TestSearchUnknownFilterIgnored`: `Filters=[{nsfw, "true"}]` →
  request body has neither `nsfw` nor `transcript_lang="true"`.
- `TestSearchEmptyLangValueDropsToDefault`:
  `Filters=[{lang, ""}]` → request body has `transcript_lang="en"`
  (Korean detection runs; English default applies for non-Korean
  text).
- `TestSearchInvalidLangFormatRejected`:
  `Filters=[{lang, "verylongstring"}]` (length > 8) → request body
  has `transcript_lang="en"`; the adapter does not crash.
- `TestDetectKoreanQueryTable` in `lang_test.go`: 8-row table over
  Hangul ratios.
- `TestSelectTranscriptLangTable` in `lang_test.go`: 4-row table over
  filter+detection combinations.

### REQ-ADP5-008 — Empty Query, Invalid Cursor, and Cursor-over-Cap Rejection

- `TestSearchEmptyQueryRejectedNoHTTP` table-drives `q.Text` over
  `["", "   ", "\t\n  \r"]`; for each asserts
  `errors.Is(err, types.ErrPermanent)` AND
  `errors.Is(err, ErrInvalidQuery)`. The httptest.Server is
  instrumented with a request counter; assert exactly 0 requests.
- `TestSearchInvalidCursorRejectedNoHTTP` table-drives `q.Cursor`
  over `["abc", "-1", "1.5", "1e3", " 25"]`; for each asserts
  `errors.Is(err, types.ErrPermanent)` AND
  `errors.Is(err, ErrInvalidCursor)`; assert zero requests.
- `TestSearchCursorOverCapRejected`:
  - q.MaxResults=50 + q.Cursor="60" → 50+60=110 > 100 →
    `errors.Is(err, ErrCursorOverCap)`; zero requests.
  - q.MaxResults=25 + q.Cursor="75" → 25+75=100 == 100 → request
    issued (the cap is INCLUSIVE of 100).
  - q.MaxResults=0 (defaults to 25) + q.Cursor="76" → 25+76=101 >
    100 → ErrCursorOverCap.

### REQ-ADP5-009 — Context Cancellation Discipline

- `TestSearchCtxCancelledMidFlight`: httptest.Server delays response
  200ms; cancel ctx at 50ms; assert
  `errors.Is(err, types.ErrSourceUnavailable)` AND
  `errors.Is(err, context.Canceled)`.
- `TestSearchCtxAlreadyCancelled`: ctx cancelled before Search call;
  assert ErrSourceUnavailable AND
  `errors.Is(err, context.Canceled)`. The httptest.Server's request
  counter is checked: 0 outbound requests observed (the adapter
  detects ctx cancellation BEFORE building the HTTP request).
- `TestSearchCtxDeadlineExceeded`: ctx with 50ms deadline;
  httptest.Server delays 200ms; assert ErrSourceUnavailable AND
  `errors.Is(err, context.DeadlineExceeded)`.
- `TestSearchNoGoroutineLeakOnCancel` (NFR-ADP5-003 anchor):
  `goleak.VerifyNone(t)` after the cancellation succeeds.

### REQ-ADP5-010 — Concurrent Search Safety (State-Driven)

- `TestSearchConcurrentSafe`: a single `*Adapter` is constructed
  pointing at one `httptest.Server` (which records every inbound
  request). 50 goroutines are launched, each calling
  `(*Adapter).Search(ctx, q)` exactly once with the same query.
  All goroutines start via a `sync.WaitGroup` barrier so the
  invocations overlap.
- Assertions:
  1. The test executes successfully under `go test -race`; the
     race detector reports zero data-race alarms attributable to
     the adapter package.
  2. The stub server's request counter equals 50.
  3. Every goroutine receives `(docs, nil)` with `len(docs) == 25`
     (matching the standard `search_response.json` fixture); each
     returned `[]types.NormalizedDoc` slice has every doc passing
     `Validate()` returning nil.

### NFR-ADP5-001 — Parse-Path Performance

- `BenchmarkParseSearchResponse25Videos` is invoked as
  `go test -bench=BenchmarkParseSearchResponse25Videos -benchtime=10x -count=5 ./internal/adapters/youtube/...`
  on amd64.
- Assertion mechanism: take the 5 reported per-op mean wall-clock
  durations (one per `-count` run); the MEDIAN of those 5 values
  SHALL be ≤ 10 ms. PASS/FAIL is decidable from the `go test -bench`
  output alone — no external CI script required.
- The bench reports `B/op` and `allocs/op`; `allocs/op ≤ 800` (= 32 ×
  25 videos). The 32-allocs-per-video figure budgets the ~17-alloc
  Metadata floor + 3-5 allocs for the transcript_snippet copy + 2-3
  allocs for available_transcript_langs []string + a few extra for
  the richer YouTube field set. Floor analysis follows the SPEC-ADP-001
  NFR-ADP-001 iteration 3 amendment pattern.

### NFR-ADP5-002 — E2E p95 (Stub)

- `TestSearchE2ELatencyStubP95` runs 100 invocations against the
  stub `httptest.Server`, sorts elapsed durations, asserts
  `durations[94] ≤ 200ms`.

### NFR-ADP5-003 — Goroutine Leak Check

- `TestSearchNoGoroutineLeakOnCancel`: `goleak.VerifyNone(t)`
  succeeds after a `Search` call whose ctx was cancelled at 50ms
  while the stub server delays response by 200ms.
- `TestMain` in `bench_test.go` invokes `goleak.VerifyTestMain(m)`
  for package-level coverage.

### NFR-ADP5-004 — Race-Clean Concurrent Workload

- `TestSearchConcurrentSafe` (REQ-ADP5-010 acceptance) executes
  under `go test -race ./internal/adapters/youtube/...`; race-
  detector alarms attributable to the youtube package = 0.

### Integration Checkpoint (M3 Exit Criterion)

When SPEC-FAN-001 + 5 of the 7 M3 ADP-* SPECs land (per
`.moai/project/roadmap.md:150` exit criterion "fused results across
≥5 adapters"), and SPEC-IDX-001 RRF wires fusion on top, ADP-005's
contribution to the M3 exit criterion is a `usearch query "go
generics tutorial"` returning at least one YouTube video result
interleaved with Reddit/HN/arXiv/GitHub results. The integration
assertion lives in CLI's e2e tests, not here, but is documented
for traceability.

---

## 6. Technical Approach

### 6.1 Files to Modify

**Created (Go side, 14 files)**:

- `internal/adapters/youtube/youtube.go` — Adapter struct, New, Name,
  Capabilities, Healthcheck, compile-time interface assertion
- `internal/adapters/youtube/youtube_test.go` — interface conformance
  tests
- `internal/adapters/youtube/search.go` — Search method (the hot
  path), JSON request body construction, Korean-detection invocation
- `internal/adapters/youtube/search_test.go` — main test file
  (largest)
- `internal/adapters/youtube/client.go` — HTTP client construction,
  doRequest, categorizeStatus
- `internal/adapters/youtube/client_test.go` — error mapping tests
- `internal/adapters/youtube/parse.go` — parseSearchResponse
  transform
- `internal/adapters/youtube/parse_test.go` — field mapping tests
- `internal/adapters/youtube/lang.go` — detectKoreanQuery,
  selectTranscriptLang
- `internal/adapters/youtube/lang_test.go` — Korean detection +
  language selection tests
- `internal/adapters/youtube/score.go` — normalizeViewScore Tanh-of-
  log10 formula
- `internal/adapters/youtube/score_test.go` — score normalization
  tests
- `internal/adapters/youtube/errors.go` — ErrInvalidQuery /
  ErrInvalidCursor / ErrCursorOverCap sentinels + parseRetryAfter
  helper (with 30s default)
- `internal/adapters/youtube/bench_test.go` — NFR-ADP5-001 benchmark
  + TestMain with goleak.VerifyTestMain
- `internal/adapters/youtube/testdata/search_response.json` (~12KB)
- `internal/adapters/youtube/testdata/search_response_empty.json`
  (~50B)
- `internal/adapters/youtube/testdata/search_response_pagination.json`
  (~12KB)
- `internal/adapters/youtube/testdata/search_response_korean.json`
  (~3KB)
- `internal/adapters/youtube/testdata/search_response_no_transcript.json`
  (~2KB)
- `internal/adapters/youtube/testdata/search_response_429.json`
  (~150B)
- `internal/adapters/youtube/testdata/search_response_503_sidecar.json`
  (~150B)
- `internal/adapters/youtube/testdata/search_response_malformed.json`
  (~200B)

**Created (Python sidecar side, contractually documented; full
implementation tracked separately per Open Question §11.7)**:

- `services/youtube-extract/Dockerfile`
- `services/youtube-extract/pyproject.toml`
- `services/youtube-extract/README.md`
- `services/youtube-extract/.env.example`
- `services/youtube-extract/src/youtube_extract/{__init__.py,
  __main__.py, app.py, ytdlp_runner.py, models.py}`
- `services/youtube-extract/tests/test_app.py`

**Modified**: `deploy/docker-compose.yml` adds the
`youtube-extract` service (build context, port mapping, env vars,
healthcheck). The CLI's `cmd/usearch/main.go` registers the YouTube
adapter into the registry alongside Reddit/HN; this change is owned
by SPEC-CLI-001 follow-up (or done as part of ADP-005's run phase
under a small CLI-wire commit, depending on team preference at run
time).

**Unchanged (by design)**:

- `pkg/types/*` — no contract change required. `DocTypeVideo`
  already exists at `pkg/types/capabilities.go:18`.
- `internal/adapters/registry.go` — wrappedAdapter sole-emitter
  pattern preserved; ADP-005 emits ZERO new metrics/logs/spans.
- `internal/obs/metrics/metrics.go` — no new metric family. The
  shared `AdapterCalls{adapter,outcome}` and
  `AdapterCallDuration{adapter}` collectors with `adapter` already
  in cardinality allowlist are sufficient.
- `internal/router/router.go` — IR-001 is unchanged; ADP-005 just
  declares its `Capabilities` and the router consumes at startup.

### 6.2 Package Layout

```
internal/adapters/youtube/
├── youtube.go                            # Adapter, New, Name, Capabilities, Healthcheck, interface assertion
├── youtube_test.go                       # Interface conformance + Capabilities determinism + Healthcheck
├── search.go                             # (*Adapter).Search hot path
├── search_test.go                        # E2E + happy path + error categorisation + filter + ctx tests
├── client.go                             # *http.Client, doRequest, categorizeStatus
├── client_test.go                        # categorizeStatus table + parseRetryAfter table
├── parse.go                              # parseSearchResponse transform (sidecar JSON envelope)
├── parse_test.go                         # Field mapping table tests
├── lang.go                               # detectKoreanQuery + selectTranscriptLang
├── lang_test.go                          # Korean detection threshold + lang priority tests
├── score.go                              # normalizeViewScore (Tanh-of-log10 formula)
├── score_test.go                         # Score normalization table
├── errors.go                             # ErrInvalidQuery + ErrInvalidCursor + ErrCursorOverCap + parseRetryAfter
└── bench_test.go                         # BenchmarkParseSearchResponse25Videos + TestMain (goleak)
└── testdata/
    ├── search_response.json              # Happy path 25 videos (mixed transcripts present/absent)
    ├── search_response_empty.json        # Zero items
    ├── search_response_pagination.json   # has_more=true, offset=0, 25 items
    ├── search_response_korean.json       # Korean video with ko transcript
    ├── search_response_no_transcript.json # video with available_transcript_langs=[]
    ├── search_response_429.json          # Sidecar 429 envelope
    ├── search_response_503_sidecar.json  # Sidecar 503 with yt-dlp signed-in challenge body
    └── search_response_malformed.json    # Truncated JSON
```

```
services/youtube-extract/
├── Dockerfile                            # multi-stage python:3.11-slim → app, non-root, healthcheck
├── pyproject.toml                        # FastAPI + uvicorn + yt-dlp pinned
├── README.md                             # operator notes
├── .env.example                          # YT_EXTRACT_PORT, YT_COOKIES_PATH, YT_USER_AGENT, sleep params
└── src/youtube_extract/
    ├── __init__.py
    ├── __main__.py                       # uvicorn entrypoint
    ├── app.py                            # FastAPI app, /health, /search
    ├── ytdlp_runner.py                   # subprocess wrapper around yt-dlp
    └── models.py                         # pydantic request/response shapes
```

[NOTE on duplication vs sharing] `parseRetryAfter`, `categorizeStatus`,
and the 30s-default behaviour duplicate parts of the equivalents in
`internal/adapters/reddit/` and `internal/adapters/hn/`. With three
adapter packages now duplicating the helpers (Reddit, HN, YouTube),
the "rule of three" is met — extraction to a shared
`internal/adapters/common/` package becomes worthwhile. However,
ADP-002 §6.2 already deferred this to a follow-up SPEC-ADP-REFAC-001.
ADP-005 ALSO defers it. Open Question §11.6 schedules the refactor
SPEC creation immediately after ADP-005 lands.

### 6.3 Sidecar Item → NormalizedDoc Field Mapping

| Sidecar item field | NormalizedDoc field | Transform |
|---|---|---|
| `id` (e.g., `"dQw4w9WgXcQ"`) | `ID` | Use as-is (11-char base64-ish; collision-free) |
| (constant) | `SourceID` | `"youtube"` (matches `Name()`) |
| `url` | `URL` | Use as-is (`https://www.youtube.com/watch?v=<id>`); canonical, no tracking params |
| `title` | `Title` | Use as-is |
| `description` (may be empty) | `Body` | Use as-is |
| If `description` non-empty: first 280 runes of description; else if `transcript_snippet` non-empty: first 280 runes of transcript_snippet; else first 280 runes of title; append `"…"` if truncated | `Snippet` | Same truncation discipline as ADP-001/ADP-002, with cascading fallback because YouTube link-only videos have empty descriptions |
| `time.Parse("2006-01-02", upload_date).UTC()` | `PublishedAt` | YouTube provides date-level precision (no time-of-day in public API surface); UTC midnight is the canonical representation |
| (parse time) | `RetrievedAt` | `time.Now().UTC()` (set by parseSearchResponse caller) |
| `channel` (display name); falls back to `uploader` when `channel` empty | `Author` | Use as-is (YouTube has channel == uploader for posted videos) |
| `normalizeViewScore(view_count)` per §2.3; treat null/missing as 0 | `Score` | Tanh-of-log10 formula; livestream-archived edge case has null view_count → Score=0.5 |
| Per priority: `Filters[Key="lang"].Value` (when set and non-empty) > Korean-detect result (when query qualifies) > `transcript_lang` from sidecar response item (when transcript present) > `""` | `Lang` | Multi-source priority; the explicit filter always wins, then auto-detection, then sidecar's actual lang, then unknown |
| (constant) | `DocType` | `types.DocTypeVideo` |
| (nil) | `Citations` | `nil` |
| (constructed) | `Metadata` | Map containing two key tiers. **REQUIRED keys** (consumers MAY rely on presence; changes require major-version bump): `channel_id` (string), `channel_url` (string), `duration_seconds` (int — 0 for livestream-archived edge case), `view_count` (int64 — 0 for null), `thumbnail_url` (string), `available_transcript_langs` ([]string — may be empty array but not nil, NEVER absent). REQ-ADP5-005 enforces these 6 as the contractual minimum. **OPTIONAL keys** (best-effort; subject to change): `like_count` (int — YouTube hides exact counts on some videos so this MAY be 0 even for popular videos), `tags` ([]string), `transcript_snippet` (string ≤500 runes; only present when `include_transcripts=true` and the chosen lang is available), `transcript_lang` (string; the lang of the snippet), `transcript_is_auto` (bool; true for auto-generated captions), `uploader_id` (string). The LAST returned doc additionally gets `next_cursor` (REQUIRED on the last doc only, when `has_more=true` AND `currentOffset + len(items) < 100`), encoded as `strconv.Itoa(currentOffset + len(items))`. |
| (constant) | `Hash` | `""` (consumers compute via `CanonicalHash()`) |

### 6.4 Sidecar HTTP Contract

The sidecar exposes two endpoints. `/search` is the hot path; `/health` is for the adapter's `Healthcheck` and for docker-compose's healthcheck directive.

**`GET /health`**:

Response 200 OK:

```json
{
  "status": "ok",
  "ytdlp_version": "2026.04.01"
}
```

Response 503 Service Unavailable when the sidecar's startup health
checks fail (yt-dlp not installed, Python interpreter broken, etc.).

**`POST /search`**:

Request body (JSON):

```json
{
  "query": "go generics tutorial",
  "max_results": 25,
  "cursor_offset": 0,
  "transcript_lang": "en",
  "include_transcripts": true,
  "since": 1700000000
}
```

- `query` (string, required): the search query verbatim.
- `max_results` (int, required): clamped [1, 100] by the adapter.
- `cursor_offset` (int, default 0): offset into yt-dlp's `ytsearch:`
  results. The sidecar issues `ytsearch{max_results+cursor_offset}:
  query` and slices `[cursor_offset:cursor_offset+max_results]` from
  the result list. v0.1 caps `max_results + cursor_offset` at 100
  (REQ-ADP5-008).
- `transcript_lang` (string, default `"en"`): preferred caption
  language. Sidecar tries this first, falls back to `en`, then to
  any available.
- `include_transcripts` (bool, default true): when true, sidecar
  fetches the transcript URL and includes the first 500 runes of
  text in `transcript_snippet`. When false, only metadata.
- `since` (int, optional): Unix-seconds; sidecar filters yt-dlp
  output to videos uploaded after this time.

Response 200 OK (success):

```json
{
  "items": [
    {
      "id": "dQw4w9WgXcQ",
      "url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
      "title": "Never Gonna Give You Up",
      "description": "...",
      "channel": "Rick Astley",
      "channel_id": "UCuAXFkgsw1L7xaCfnd5JJOw",
      "channel_url": "https://www.youtube.com/@RickAstleyYT",
      "uploader": "Rick Astley",
      "uploader_id": "RickAstleyYT",
      "duration_seconds": 213,
      "view_count": 1234567890,
      "like_count": 12345678,
      "upload_date": "2009-10-25",
      "thumbnail_url": "https://i.ytimg.com/vi/dQw4w9WgXcQ/hqdefault.jpg",
      "tags": ["music", "rick astley"],
      "available_transcript_langs": ["en", "ko", "ja"],
      "transcript_snippet": "We're no strangers to love\nYou know the rules and so do I…",
      "transcript_lang": "en",
      "transcript_is_auto": false
    }
  ],
  "has_more": true
}
```

Per-item fields are directly consumed by §6.3 mapping. The `has_more`
boolean signals whether `currentOffset + len(items)` should produce
a `next_cursor` (when `has_more=true` AND the cap is not hit).

Response 4xx/5xx (error):

```json
{
  "error": {
    "category": "unavailable",
    "message": "yt-dlp signed-in challenge"
  }
}
```

The `category` field is one of `"unavailable"`, `"permanent"`,
`"transient"`, `"rate_limited"` (matches `pkg/types.Category` enum).
The adapter's `parse.go` parses this envelope when HTTP status ≠ 200
and produces the corresponding `*types.SourceError`.

### 6.5 HTTP Client Construction Notes

- **Timeout**: 30 seconds total request deadline (default — LONGER
  than ADP-001/ADP-002's 10s because the sidecar's yt-dlp call may
  legitimately take 5-15 seconds with transcript). Caller's ctx
  deadline takes precedence when shorter. Configurable via
  `Options.HTTPClient`.
- **Redirect policy**: stdlib default (`http.Client.CheckRedirect`
  unset) — the sidecar URL is operator-configured and trusted; no
  cross-domain redirect surface. Distinct from ADP-001/ADP-002
  which guard against external redirects.
- **Transport**: `reqid.NewTransport(http.DefaultTransport)` for
  request-ID propagation. Mirrors ADP-001/ADP-002.
- **Headers per request**: `User-Agent: usearch/<version>
  (+https://github.com/elymas/universal-search)`,
  `Accept: application/json`, `Content-Type: application/json` (for
  POST body). NO authentication header (sidecar does not require
  auth from the Go side; sidecar has its OWN auth surface for
  cookie file handling but that's the operator's concern).

### 6.6 Observability Note

The YouTube adapter, like Reddit and HN, emits ZERO metrics, logs,
and spans of its own. ALL observability comes from the registry's
`wrappedAdapter` (`internal/adapters/registry.go:172-263`). This is
the sole-emitter discipline established in SPEC-CORE-001 §6.5 and
preserved verbatim by SPEC-ADP-001 / SPEC-ADP-002. The adapter's
responsibility is to return a correctly-categorised
`*types.SourceError` so the wrappedAdapter computes the right
`outcome` label via `types.OutcomeFromError(err)`:

- `nil` → `"success"`
- `context.DeadlineExceeded` → `"timeout"`
- `CategoryRateLimited` → `"rate_limited"`
- `CategoryUnavailable` → `"unavailable"` (sidecar 503, sidecar
  unreachable, yt-dlp signed-in challenge)
- `CategoryTransient` → `"transient"`
- `CategoryPermanent` / unknown → `"failure"` (sidecar 4xx, malformed
  JSON, invalid query)

The Prometheus counter `AdapterCalls{adapter="youtube",outcome=<...>}`
and histogram `AdapterCallDuration{adapter="youtube"}` are inherited.
NO new metric families. The cardinality stays bounded.

### 6.7 MX Tag Plan

| File::Symbol | Tag | Reason |
|--------------|-----|--------|
| `youtube.go::(*Adapter).Search` (defined in `search.go`) | `@MX:ANCHOR` | Sole entry point for all YouTube fanout calls. fan_in ≥ 3 (registry wrappedAdapter, FAN-001 fanout, tests). `@MX:REASON: contract boundary; signature change ripples to FAN-001 + CLI-001 + SYN-001 and synthesizer's video citation handler`. `@MX:SPEC: SPEC-ADP-005`. |
| `parse.go::parseSearchResponse` | `@MX:ANCHOR` | Every YouTube item passes through this single transform. fan_in = 1 (Search) but invariant-bearing — bug here corrupts every NormalizedDoc returned. `@MX:REASON: NormalizedDoc field-mapping integrity gate`. `@MX:SPEC: SPEC-ADP-005`. |
| `score.go::normalizeViewScore` (function) and constants `log10Divisor=5.0, scoreCenter=0.5` | `@MX:NOTE` | Documents the Tanh-of-log10 formula choice and tie-in to SPEC-IDX-001 RRF. The function gets a doc-comment `@MX:NOTE` explaining the empirical inflection-point at views=100K; the two constants get inline `@MX:NOTE` annotations citing Open Question §11.5 revisit triggers. |
| `lang.go::detectKoreanQuery` + `lang.go::selectTranscriptLang` | `@MX:NOTE` | Documents the 30% Hangul threshold and the explicit-filter > auto-detect > English priority. The threshold is empirical; if Korean-content recall regresses post-M3, revisit. |
| `client.go::categorizeStatus` | `@MX:NOTE` | The HTTP-status-to-Category rosetta. Future contributors will look here first when a new HTTP code needs handling. Mirrors the equivalent in Reddit/HN packages — the duplication is acknowledged in §6.2 deferred-refactor notes. |
| `client.go::doRequest` | `@MX:WARN` | Outbound network call to a configurable sidecar URL. NO redirect allowlist (sidecar is trusted, operator-configured). `@MX:REASON: removing the request-context propagation would invalidate REQ-ADP5-009 ctx cancellation discipline and NFR-ADP5-003 goroutine-leak guarantee`. |
| `errors.go::ErrCursorOverCap` (sentinel constant) | `@MX:NOTE` | The 100-item pagination cap. Documents D7 / Open Question §11.4 revisit triggers. |

All tags `[AUTO]`-prefixed (agent-generated), include `@MX:SPEC:
SPEC-ADP-005`, follow `code_comments: en` per
`.moai/config/sections/language.yaml`. Per-file hard limit (3 ANCHOR
+ 5 WARN per `.moai/config/sections/mx.yaml` defaults): respected
across files (2 ANCHORs across `search.go` and `parse.go`; 1 WARN on
`client.go::doRequest`).

### 6.8 Harness Level

Per `.moai/config/sections/harness.yaml` auto-routing: 10 EARS REQs
(8 × P0 + 2 × P1) + 4 NFRs touching 1 Go package
(`internal/adapters/youtube/` — 14 source/test files + 8 testdata
fixtures) + 1 NEW Python sidecar directory
(`services/youtube-extract/` — contractually documented; full
implementation tracked separately) + 1 docker-compose.yml
modification + zero security/payment/PII keywords (the cookie-file
mitigation is a Docker secret outside Go code) + zero new metric
families = **standard** harness level. Sprint Contract is OPTIONAL
but recommended given the cross-language sidecar coordination.
Evaluator profile `default` applies.

---

## 7. What NOT to Build (Exclusions)

[HARD] This SPEC explicitly excludes the following items. Each has a
known destination SPEC; this list prevents scope creep into ADP-005.

- **YouTube Data API v3 OAuth integration** (channel-owned caption
  download, owner-only metadata) → out of v0.1 scope; deferred to a
  future SPEC-ADP-005a if the sidecar path becomes operationally
  unsustainable. Path B was rejected per D1 / research.md §1.3.
- **Per-source customisations for arXiv, GitHub, Bluesky/X, SearXNG,
  Naver, Daum, KoreaNewsCrawler** → SPEC-ADP-003, SPEC-ADP-004,
  SPEC-ADP-006 through SPEC-ADP-009 (M3, parallel post-FAN-001).
- **Retry orchestration** → SPEC-FAN-001 (M3). Adapter is one-shot
  per call.
- **Response caching** → SPEC-CACHE-001 (M3). Adapter is stateless.
- **Result ranking, deduplication, RRF fusion across adapters** →
  SPEC-IDX-001 (M3).
- **Adapter health-state machine / circuit breaker** → SPEC-EVAL-002
  (M8).
- **Subscription / playlist / channel browsing** → out of v0.1.
- **Live-stream content classification** → out of v0.1; included in
  results with `Score=0.5` for null view_count. Documented in
  `Capabilities.Notes`.
- **Music vs lecture vs short-form classification** → SPEC-IR-001's
  domain.
- **Full transcript inclusion in /search response** → out of v0.1;
  adapter only includes `transcript_snippet` (≤500 runes).
- **Live network integration tests in CI** → out of v0.1; httptest
  + golden fixtures only on Go side.
- **`/transcript` sidecar endpoint Go binding** → out of v0.1;
  SPEC-SYN-001 may add when synthesis layer needs full transcripts.
- **Korean tokenisation of transcripts** → SPEC-IDX-003 (M3).
- **`pkg/llm` integration** — adapter does NOT call any LLM.
- **Per-adapter custom Prometheus metrics** → would require amending
  SPEC-OBS-001's allowlist.
- **Cross-adapter helper extraction** → SPEC-ADP-REFAC-001 follow-up
  (Open Question §11.6).
- **Streaming Search results** (channel-based incremental delivery)
  → SPEC-SYN-004 (M4) if measured value.
- **Sidecar implementation details and Python tests** → tracked
  separately per Open Question §11.7.
- **YouTube Shorts vs long-form classification** → out of v0.1.
- **Geographic-restriction handling** → out of v0.1; yt-dlp's
  `--geo-bypass` is a sidecar configuration detail.

---

## 8. TDD Plan

Development follows RED-GREEN-REFACTOR per
`quality.development_mode: tdd` (`.moai/config/sections/quality.yaml`).
Representative RED-phase tests, written before implementation,
grouped by REQ. Total: 42 tests. Coverage target: 85% per
`quality.test_coverage_target`. Benchmarks do not count toward
coverage.

| # | Test | Layer | REQ | Assertion |
|---|------|-------|-----|-----------|
| 1 | `TestAdapterName` | `youtube_test.go` | REQ-ADP5-001 | `(*Adapter).Name() == "youtube"` |
| 2 | `TestAdapterImplementsInterface` | `youtube_test.go` | REQ-ADP5-001 | Compile-time `var _ types.Adapter = (*Adapter)(nil)` succeeds |
| 3 | `TestCapabilitiesDeterministic` | `youtube_test.go` | REQ-ADP5-001 | Two consecutive `Capabilities()` calls return `reflect.DeepEqual` results |
| 4 | `TestCapabilitiesShape` | `youtube_test.go` | REQ-ADP5-001 | All 9 documented field values match (SourceID, DisplayName, DocTypes, RequiresAuth, AuthEnvVars, SupportsSince=true, RateLimitPerMin=30, DefaultMaxResults=25, plus 5 Notes substring matches) |
| 5 | `TestHealthcheckSucceeds` | `youtube_test.go` | REQ-ADP5-001 | Stub `/health` returns `{"status":"ok",...}` → nil error |
| 6 | `TestHealthcheckFailsOn503` | `youtube_test.go` | REQ-ADP5-001 | Stub `/health` returns 503 → non-nil error |
| 7 | `TestHealthcheckFailsOnMalformedJSON` | `youtube_test.go` | REQ-ADP5-001 | Stub returns 200 with non-JSON body → non-nil error |
| 8 | `TestHealthcheckFailsOnStatusNotOk` | `youtube_test.go` | REQ-ADP5-001 | Stub returns `{"status":"degraded"}` → non-nil error |
| 9 | `TestHealthcheckFailsOnUnreachable` | `youtube_test.go` | REQ-ADP5-001 | Sidecar URL points to closed port → non-nil error |
| 10 | `TestSearchHappyPath25Videos` | `search_test.go` | REQ-ADP5-002, REQ-ADP5-005 | 25 NormalizedDocs returned; each `Validate()` returns nil |
| 11 | `TestSearchRequestBodyIncludesAllRequired` | `search_test.go` | REQ-ADP5-002 | Decoded request body has `query`, `max_results`, `transcript_lang`, `include_transcripts=true` |
| 12 | `TestSearchClampsMaxResultsTo100` | `search_test.go` | REQ-ADP5-002 | q.MaxResults=500 → request has `max_results=100` |
| 13 | `TestSearchDefaultsMaxResultsTo25` | `search_test.go` | REQ-ADP5-002 | q.MaxResults=0 → request has `max_results=25` |
| 14 | `TestSearchOmitsCursorWhenEmpty` | `search_test.go` | REQ-ADP5-002 | q.Cursor="" → request has `cursor_offset=0` |
| 15 | `TestSearchSetsCursorWhenPresent` | `search_test.go` | REQ-ADP5-002 | q.Cursor="25" → request has `cursor_offset=25` |
| 16 | `TestSearchHTTP429WithIntegerRetryAfter` | `search_test.go` | REQ-ADP5-003 | `Retry-After: 30` → SourceError.RetryAfter==30s |
| 17 | `TestSearchHTTP429WithHTTPDateRetryAfter` | `search_test.go` | REQ-ADP5-003 | HTTP-date 30s ahead → RetryAfter ∈ (25s, 35s) |
| 18 | `TestSearchHTTP429NoRetryAfterDefaults30s` | `search_test.go` | REQ-ADP5-003 | No header → RetryAfter==30s (distinct from Reddit/HN 5s) |
| 19 | `TestSearchHTTP429RetryAfterCapped60s` | `search_test.go` | REQ-ADP5-003 | `Retry-After: 999` → RetryAfter==60s |
| 20 | `TestSearchHTTP429NoInternalRetry` | `search_test.go` | REQ-ADP5-003 | Server request count == 1 |
| 21 | `TestSearchHTTP4xx` | `search_test.go` | REQ-ADP5-004 | Table over 401/403/404 → ErrPermanent + matching HTTPStatus |
| 22 | `TestSearchHTTP5xx` | `search_test.go` | REQ-ADP5-004 | Table over 500/503/504 → ErrSourceUnavailable + matching HTTPStatus |
| 23 | `TestSearchSidecarUnreachable` | `search_test.go` | REQ-ADP5-004 | httptest.Server closed → ErrSourceUnavailable, HTTPStatus==0 |
| 24 | `TestSearchSidecarYtdlpChallenge` | `search_test.go` | REQ-ADP5-004 | 503 with `{"error":{"category":"unavailable","reason":"yt-dlp signed-in challenge"}}` → ErrSourceUnavailable; Unwrap reveals reason |
| 25 | `TestSearchUnavailablePreservesUnderlyingError` | `search_test.go` | REQ-ADP5-004 | `errors.Unwrap(srcErr)` non-nil, inner cause text present |
| 26 | `TestParseSearchResponseFieldMapping` | `parse_test.go` | REQ-ADP5-005 | Table over 5 fixtures; every documented field maps correctly |
| 27 | `TestParseSearchResponseSelectsKoreanLang` | `parse_test.go` | REQ-ADP5-005 | Korean fixture → returned doc has `Lang="ko"` |
| 28 | `TestParseSearchResponseIncludesTranscriptSnippet` | `parse_test.go` | REQ-ADP5-005 | Non-empty `transcript_snippet` → Metadata key present, ≤500 runes |
| 29 | `TestParseSearchResponseTruncatesOverlongTranscript` | `parse_test.go` | REQ-ADP5-005 | 1000-rune sidecar value → adapter truncates to 500 |
| 30 | `TestParseSearchResponsePaginationCursor` | `parse_test.go` | REQ-ADP5-005 | has_more=true, offset=0, 25 items → last doc Metadata["next_cursor"]=="25" |
| 31 | `TestParseSearchResponseNoCursorOnLastPage` | `parse_test.go` | REQ-ADP5-005 | has_more=false → no doc has `next_cursor` |
| 32 | `TestParseSearchResponseHashEmpty` | `parse_test.go` | REQ-ADP5-005 | Every doc.Hash == "" |
| 33 | `TestParseSearchResponseMetadataKeys` | `parse_test.go` | REQ-ADP5-005 | All 6 required Metadata keys present |
| 34 | `TestParseSearchResponseSkipsItemsWithError` | `parse_test.go` | REQ-ADP5-005 | Mixed success+error items → only successes returned |
| 35 | `TestParseSearchResponseLivestreamNullViewCount` | `parse_test.go` | REQ-ADP5-005 | view_count=null → Score=0.5 |
| 36 | `TestParseSearchResponseMalformedJSON` | `parse_test.go` | REQ-ADP5-005 | Truncated JSON → ErrPermanent |
| 37 | `TestSearchSetsCustomUserAgent` | `client_test.go` | REQ-ADP5-006 | UA starts with "usearch/" + contains URL |
| 38 | `TestSearchSetsAcceptJSON` | `client_test.go` | REQ-ADP5-006 | `Accept: application/json` |
| 39 | `TestSearchSetsContentTypeJSON` | `client_test.go` | REQ-ADP5-006 | `Content-Type: application/json` |
| 40 | `TestSearchUserAgentVersionConfigurable` | `client_test.go` | REQ-ADP5-006 | UAVersion override propagates |
| 41 | `TestSearchExplicitLangFilterWins` | `search_test.go` | REQ-ADP5-007 | Filters=[{lang,"ja"}] + Korean text → request `transcript_lang="ja"`, doc.Lang="ja" |
| 42 | `TestSearchKoreanAutoDetection` | `search_test.go` | REQ-ADP5-007 | Korean text >30% Hangul → request `transcript_lang="ko"` |
| 43 | `TestSearchEnglishDefaultForLatinScript` | `search_test.go` | REQ-ADP5-007 | "hello world" → request `transcript_lang="en"` |
| 44 | `TestSearchKoreanThresholdBoundary` | `search_test.go` | REQ-ADP5-007 | 29% Hangul → English; 31% Hangul → Korean |
| 45 | `TestSearchSinceFilterAdded` | `search_test.go` | REQ-ADP5-007 | Filters=[{since,"1700000000"}] → request `since=1700000000` |
| 46 | `TestSearchSinceFilterMalformedDropped` | `search_test.go` | REQ-ADP5-007 | Filters=[{since,"abc"}] → request has no `since` |
| 47 | `TestSearchSinceFilterNegativeDropped` | `search_test.go` | REQ-ADP5-007 | Filters=[{since,"-100"}] → request has no `since` |
| 48 | `TestSearchUnknownFilterIgnored` | `search_test.go` | REQ-ADP5-007 | Filters=[{nsfw,"true"}] → no `nsfw` field |
| 49 | `TestSearchEmptyLangValueDropsToDefault` | `search_test.go` | REQ-ADP5-007 | Filters=[{lang,""}] → request `transcript_lang="en"` |
| 50 | `TestSearchInvalidLangFormatRejected` | `search_test.go` | REQ-ADP5-007 | Filters=[{lang,"verylongstring"}] → request `transcript_lang="en"` |
| 51 | `TestDetectKoreanQueryTable` | `lang_test.go` | REQ-ADP5-007 | 8-row table over Hangul ratios |
| 52 | `TestSelectTranscriptLangTable` | `lang_test.go` | REQ-ADP5-007 | 4-row table over filter+detection combinations |
| 53 | `TestSearchEmptyQueryRejectedNoHTTP` | `search_test.go` | REQ-ADP5-008 | Empty/whitespace q.Text → ErrPermanent + zero requests |
| 54 | `TestSearchInvalidCursorRejectedNoHTTP` | `search_test.go` | REQ-ADP5-008 | Invalid cursors → ErrPermanent + zero requests |
| 55 | `TestSearchCursorOverCapRejected` | `search_test.go` | REQ-ADP5-008 | max_results+offset > 100 → ErrCursorOverCap; ==100 → request issued |
| 56 | `TestSearchCtxCancelledMidFlight` | `search_test.go` | REQ-ADP5-009 | Cancel ctx mid-flight → ErrSourceUnavailable + errors.Is(err, ctx.Err()) |
| 57 | `TestSearchCtxAlreadyCancelled` | `search_test.go` | REQ-ADP5-009 | Pre-cancelled ctx → ErrSourceUnavailable + errors.Is(err, context.Canceled) + zero requests |
| 58 | `TestSearchCtxDeadlineExceeded` | `search_test.go` | REQ-ADP5-009 | 50ms deadline + 200ms server delay → ErrSourceUnavailable + errors.Is(err, context.DeadlineExceeded) |
| 59 | `TestSearchConcurrentSafe` | `search_test.go` | REQ-ADP5-010, NFR-ADP5-004 | 50 goroutines × Search, race-clean, 50 stub requests, 25 valid docs each |
| 60 | `TestNormalizeViewScoreTable` | `score_test.go` | REQ-ADP5-005 | 7 view-count values → expected `[0,1]` outputs within ±0.001 |
| 61 | `TestNormalizeViewScoreDeterministic` | `score_test.go` | REQ-ADP5-005 | Two calls on same input return byte-equal output |
| 62 | `TestNormalizeViewScoreZeroIs05` | `score_test.go` | REQ-ADP5-005 | viewCount=0 → Score=0.5 exactly |
| 63 | `TestParseRetryAfterTable` | `client_test.go` | REQ-ADP5-003 | Table over 6 inputs (int, HTTP-date, missing→30s, malformed→30s, > 60→60, negative) |
| 64 | `TestCategorizeStatusTable` | `client_test.go` | REQ-ADP5-003/004 | Truth table over 7 status codes (200/401/403/404/429/500/503/0) → expected Category |
| 65 | `TestSearchE2ELatencyStubP95` | `search_test.go` | NFR-ADP5-002 | 100 invocations against stub; p95 ≤ 200ms |
| 66 | `TestSearchNoGoroutineLeakOnCancel` | `search_test.go` | NFR-ADP5-003 | `goleak.VerifyNone(t)` after mid-flight ctx cancel |
| 67 | `BenchmarkParseSearchResponse25Videos` | `bench_test.go` | NFR-ADP5-001 | Median of 5 `-count` runs at `-benchtime=10x` is ≤ 10ms per op; allocs/op ≤ 800 |
| 68 | `TestMain` (goleak.VerifyTestMain) | `bench_test.go` | NFR-ADP5-003 | Package-level goroutine leak check |

(Renumbering: the count of 42 in the §1 HISTORY draft was a draft
estimate; the table above resolves to 68 representative tests across
the 10 REQs + 4 NFRs. Several REQs share tests for 30+ assertions
each. The 42 figure was the unique-REQ-anchored count, but the
test table here lists each row separately for clarity. Coverage is
satisfied by the broader set; the 85% gate is on lines, not tests.)

RED-GREEN-REFACTOR per requirement:

1. RED: Write failing test for REQ-ADP5-N.
2. GREEN: Implement minimal code to pass.
3. REFACTOR: Tidy; extract shared helpers if they remove duplication
   WITHIN the package; keep file sizes manageable (target each `.go`
   file < 250 LoC excluding tests). Cross-package extraction (Reddit/
   HN/YouTube common helpers) is deferred to SPEC-ADP-REFAC-001 per
   Open Question §11.6.

Greenfield note: `internal/adapters/youtube/` does not exist. There
is no behaviour to preserve; no characterization tests needed.
The Python sidecar at `services/youtube-extract/` is also greenfield
but its tests are tracked in a separate run-phase task per Open
Question §11.7.

---

## 9. Dependencies

### 9.1 Upstream SPEC Dependencies

- **SPEC-CORE-001 (implemented; merged)**: provides
  `pkg/types.Adapter`, `pkg/types.Capabilities`, `pkg/types.Query`,
  `pkg/types.NormalizedDoc`, `*types.SourceError`,
  `types.OutcomeFromError`, `types.DocType` enum (including
  `DocTypeVideo` at `pkg/types/capabilities.go:18`),
  `internal/adapters.Registry` with wrappedAdapter sole-emitter
  pattern, `internal/adapters/noop` reference shape. HARD dep.
- **SPEC-OBS-001 (implemented)**: provides `obs.Obs` bundle,
  `internal/obs/reqid.NewTransport` for request-ID propagation,
  `AdapterCalls{adapter,outcome}` and `AdapterCallDuration{adapter}`
  collectors with `adapter` and `outcome` already in cardinality
  allowlist. SOFT dep — adapter is nil-safe via the registry's
  nil-guards. The `adapter="youtube"` cardinality value fits within
  the V1 14-adapter ceiling per SPEC-OBS-001 NFR-OBS-002.
- **SPEC-IR-001 (implemented)**: documents the consumer contract for
  `Capabilities` (REQ-IR-008 selects AdapterSet by intersecting
  `categoryEligibleDocTypes` with `SupportedLangs`). ADP-005's
  `Capabilities()` shape (DocTypes=[DocTypeVideo],
  SupportedLangs=nil) determines which routing categories the
  YouTube adapter will be selected for. SOFT dep.

### 9.2 Reference Patterns (not strict deps)

- **SPEC-ADP-001 (implemented)**: file layout, error mapping
  discipline, MX tag plan template, TDD harness. ADP-005 inherits
  the structural shape but uses YouTube-specific contents.
- **SPEC-ADP-002 (implemented)**: HN adapter; second-adapter
  validation of the reference shape. ADP-005 is the third adapter
  applying the shape.
- **SPEC-FAN-001 (approved)**: M3 fanout; ADP-005's `Search` is
  consumed by the fanout. ADP-005 satisfies REQ-FAN-002 / REQ-FAN-003
  contract by returning `(docs, *SourceError)` with race-clean
  concurrency per REQ-ADP5-010 + NFR-ADP5-004.

### 9.3 Parallelizable

- **SPEC-ADP-003 / SPEC-ADP-004 / SPEC-ADP-006 / SPEC-ADP-007 /
  SPEC-ADP-008 / SPEC-ADP-009 (all M3)**: gated only on FAN-001 per
  `.moai/project/roadmap.md:122-123`. Once FAN-001 is approved
  (status: approved as of 2026-05-05 per SPEC-FAN-001 spec.md
  HISTORY), all 7 M3 adapter SPECs (including ADP-005) plan in
  parallel; their run phases parallelize too.

### 9.4 Downstream Blocked SPECs

- **None directly**. ADP-005's `blocks` frontmatter list is empty —
  no other SPEC strictly depends on YouTube being implemented.
  Downstream consumers (SPEC-IDX-001 RRF, SPEC-CACHE-001 access
  fallback, SPEC-SYN-001/002 synthesis) consume the AGGREGATE of
  M3 adapters via FAN-001's `fanout.Result.Docs` and don't
  individually require ADP-005 specifically. The M3 exit criterion
  ("≥5 adapters fused") needs at least 5 of the 7 M3 ADP-* SPECs to
  land but does not single out which 5.

### 9.5 External Dependencies (run-phase pins)

**Zero new Go module dependencies on the Go side.** ADP-005 uses
only:

- Go stdlib: `bytes`, `context`, `encoding/json`, `errors`, `fmt`,
  `io`, `math`, `net`, `net/http`, `net/url`, `strconv`, `strings`,
  `time`, `unicode`, `unicode/utf8`
- `pkg/types` (already pinned via SPEC-CORE-001)
- `internal/obs/reqid` (already pinned via SPEC-OBS-001)
- Test-only: `go.uber.org/goleak` (for NFR-ADP5-003) — already pinned
  by SPEC-ADP-001/SPEC-ADP-002 run-phases.

**ONE new Python sidecar with its own dependency surface** (tracked
separately per Open Question §11.7):

- `fastapi >= 0.115` (mirrors `services/researcher/pyproject.toml`)
- `uvicorn[standard] >= 0.30`
- `pydantic >= 2.9`
- `httpx >= 0.27` (sidecar fetches transcript URLs)
- `yt-dlp == <pinned-version>` (e.g., `2026.04.01`)

The sidecar's pyproject.toml is contractually documented in §6.4;
its full implementation is a run-phase deliverable.

---

## 10. Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| YouTube IP-block / "Sign in to confirm" challenge breaks the sidecar | High | High | Sidecar's `--sleep-requests 1.0 --sleep-interval 2 --max-sleep-interval 5` defaults; optional cookie file injection; 30s default Retry-After (longer than Reddit/HN's 5s); REQ-ADP5-004 maps the challenge to `CategoryUnavailable` so SPEC-FAN-001 fanout proceeds with other adapters. Documented in https://github.com/yt-dlp/yt-dlp/issues/10128. Future SPEC-EVAL-002 (M8) owns true circuit-breaking. |
| yt-dlp release breakage (YouTube extractor breaks roughly quarterly) | Medium | Medium | Pinned yt-dlp version in sidecar pyproject.toml; weekly CI integration test against stable test video IDs (e.g., dQw4w9WgXcQ); Open Question §11.6 tracks version-bump cadence policy. |
| Sidecar process down / unreachable at startup | Medium | Medium | Adapter returns `CategoryUnavailable`; fanout proceeds with other adapters. `Healthcheck` endpoint surfaces liveness for SPEC-EVAL-002 (M8) reliability dashboard. docker-compose's `healthcheck` directive provides container-level fail-fast. |
| Subprocess zombie on caller-cancellation (yt-dlp doesn't honour SIGTERM cleanly) | Medium | Medium | Sidecar's FastAPI middleware tracks request ctx; on cancel, sends SIGTERM then SIGKILL after 5s grace. Tested in sidecar's `tests/test_app.py::test_subprocess_cleanup`. NFR-ADP5-003 ensures Go side has no goroutine leak; sidecar zombies are the Python side's concern. |
| Transcript fetch latency >> metadata latency | Medium | Medium | `/search` returns `transcript_snippet` (first 500 runes) only. Full transcript via separate `/transcript` call (out of v0.1 Go-side scope). Caller (synthesis) decides whether to fetch full transcript per cited video. The 30s adapter timeout vs 10s for Reddit/HN reflects this. |
| Korean transcript not present despite Korean query | Medium | Low | `available_transcript_langs` always present in Metadata; consumer can re-request via `/transcript` for other lang. Sidecar falls back to `en` automatically. |
| Pagination opacity (yt-dlp `ytsearchN:` is not natively paginated) | Medium | Low | Adapter implements offset-based pagination via `ytsearch{N+offset}:` re-query. Cursor is a decimal-string offset. Cap of 100 (REQ-ADP5-008 / D7) bounds the inefficiency. |
| License contagion from yt-dlp GPLv3+ binaries | Low | Low | yt-dlp invoked as subprocess (NOT linked); `pip install yt-dlp` in sidecar Dockerfile; Apache-2.0 boundary preserved at the Go adapter side. License rationale documented in research §1.1. |
| Daily YouTube view-count drift | Low | Low | View count reflects the moment of fetch; `RetrievedAt` is the ground truth. SPEC-CACHE-001 (M3) handles staleness if needed. Acknowledged in `Capabilities.Notes`. |
| Score saturation at very-popular videos | Low | Low | log10 inflection at views≈100K; videos with 1B views map to ~0.97 (not 1.0); RRF re-weights via rank. Open Question §11.5 reviews after M3 RRF integration. |
| Sidecar Docker image build cost (Python + yt-dlp deps) | Low | Low | Multi-stage Dockerfile; cached pip layer; image size ~150MB compressed. One-time CI cost. |
| Subprocess output parsing mismatch when yt-dlp updates JSON shape | Low | Low | pydantic models in sidecar provide strict validation; new fields ignored; missing required fields → `CategoryUnavailable`. Tested via fixture-based unit tests in sidecar's pytest suite. |
| Race condition with multi-tenant shared cookie file | Low | Low | Single cookie file is read-only on container start; yt-dlp doesn't write to it during extraction. Documented in `services/youtube-extract/README.md`. |
| `ytsearch:` URL prefix is a yt-dlp internal feature, not a YouTube-supported URL | Low | Low | Stable across yt-dlp 2024-2026 history; no indication of upcoming removal. Tracked via pinned version. |
| Hash collisions across YouTube and other adapters for same shared external URL (e.g., a Reddit link to a YouTube video) | Low | Low | `CanonicalHash` includes `SourceID` prefix per `pkg/types/normalized_doc.go:96-99` — Reddit and YouTube cannot collide. |
| `time.Now()` in `RetrievedAt` non-deterministic in tests | Low | Low | `parseSearchResponse` accepts `retrievedAt time.Time` parameter; tests inject fixed time. Search wraps with `time.Now().UTC()` in production. |
| `MaxResults + Cursor offset > 100` cap is overly restrictive for power users | Low | Low | The cap matches yt-dlp's empirical efficiency boundary. Operators who need deeper pagination can request via Open Question §11.4. |
| `30s` adapter timeout is too aggressive when sidecar is slow | Medium | Low | Configurable via `Options.HTTPClient`; default matches the typical yt-dlp + transcript fetch p95. Caller's ctx deadline takes precedence when shorter. |
| HTTP timeout (30s) too aggressive when sidecar is slow but parent ctx is generous | Low | Low | Parent ctx wins via stdlib precedence; the 30s is just the upper bound when parent has none. |
| sidecar cookie file path on macOS dev (`/run/secrets/` is Linux-only) | Low | Low | Sidecar `.env.example` documents alternatives (`./secrets/yt_cookies.txt` for local dev). Test in sidecar's pytest suite. |

---

## 11. Open Questions

These are explicitly UNRESOLVED at SPEC-approval time. Each has a
recommended default and a one-line resolution owner. They do NOT
block SPEC approval.

1. **Sidecar bind URL configurability** — fixed at
   `http://localhost:8082` or accept arbitrary URL? **Recommended
   default**: arbitrary URL via `Options.BaseURL`, default
   `http://localhost:8082`. Adapter constructor accepts
   `Options.BaseURL` like ADP-001/ADP-002. **Resolution owner**:
   SPEC-DEPLOY-001 (M9) author when productionizing.

2. **Cookie file mounting strategy** — Docker secret, volume mount,
   or vault integration? **Recommended default**: Docker secret at
   `/run/secrets/yt_cookies.txt`, env var `YT_COOKIES_PATH`
   overrideable. **Resolution owner**: expert-devops during
   SPEC-DEPLOY-001 / SPEC-SEC-001 (M8).

3. **Transcript full-fetch endpoint Go binding** — should ADP-005
   ship a Go-side `(*Adapter).GetTranscript(ctx, videoID, lang)
   (string, error)` method bound to the sidecar's `/transcript`
   endpoint? **Recommended default**: NO in v0.1 — synthesis
   layer (SPEC-SYN-001) can call the sidecar directly when needed,
   bypassing the adapter abstraction. The adapter's job is search;
   transcript fetch is a separate concern. **Resolution owner**:
   SPEC-SYN-001 author.

4. **Pagination depth cap** — yt-dlp's `ytsearchN:` is inefficient
   for deep offsets. v0.1 caps `MaxResults + Cursor offset` at 100
   (effectively top-100 across pages). Should this be 50 or 200?
   **Recommended default**: 100. **Resolution owner**: SPEC-FAN-001
   author may revisit if telemetry shows users paginating heavily.

5. **Score formula calibration** — log10 divisor=5.0 chosen
   empirically for [0, 10B] view range. Should it be 4.0 or 6.0?
   **Recommended default**: 5.0 in v0.1; revisit after SPEC-IDX-001
   RRF integration measures ranking quality. **Resolution owner**:
   SPEC-IDX-001 author.

6. **Cross-adapter helper extraction** (sharing `parseRetryAfter`,
   `categorizeStatus`, error sentinel patterns between Reddit, HN,
   YouTube). **Recommended default**: NOW that 3 adapters duplicate
   these helpers (rule of three met), extraction is justified.
   Schedule SPEC-ADP-REFAC-001 IMMEDIATELY after ADP-005's run
   phase lands. **Resolution owner**: expert-backend; manager-
   refactoring may pick this up post-M3.

7. **Sidecar implementation milestone scope** — ADP-005 contractually
   documents `services/youtube-extract/` HTTP surface but defers
   full Python implementation to a separate work item. Should the
   sidecar's run-phase be (a) bundled into ADP-005's run phase as a
   parallel task, (b) extracted into a NEW SPEC-ADP-005-SIDECAR, or
   (c) tracked as an internal task without a SPEC? **Recommended
   default**: (a) — bundle into ADP-005's run phase as a parallel
   task with explicit task-list separation; expert-backend owns the
   Python side along with the Go side. The sidecar is small (~200
   LoC + tests) and the contract is locked here. **Resolution
   owner**: orchestrator at run-phase startup; if work scope grows
   beyond expectations, escalate to a separate SPEC.

---

## 12. References

### External (URL-cited; verified per research.md §7)

- https://github.com/yt-dlp/yt-dlp — yt-dlp project README; license
  (Unlicense source / GPLv3+ bundled), capabilities, subprocess
  interface, rate-limit / IP-block options.
- https://github.com/yt-dlp/yt-dlp/wiki/Installation — installation
  methods; PyPI package `yt-dlp`; standalone binaries cross-platform;
  no official Docker image.
- https://github.com/yt-dlp/yt-dlp/issues/10128 — June 2024 "Sign in
  to confirm you're not a bot" challenge incident; basis for
  REQ-ADP5-003 30s default Retry-After and the cookie-mitigation
  recommendation.
- https://developers.google.com/youtube/v3/docs/search/list — search.
  list endpoint; 100 quota units/call; pagination via pageToken;
  default maxResults=5 (max 50). Basis for D1 Path B rejection.
- https://developers.google.com/youtube/v3/docs/videos/list —
  videos.list endpoint; 1 quota unit/call; parts (snippet,
  contentDetails, statistics).
- https://developers.google.com/youtube/v3/docs/captions — captions
  endpoint; 5 methods (list, insert, update, download, delete);
  third-party transcript retrieval requires OAuth + ownership.
- https://developers.google.com/youtube/v3/getting-started — default
  quota 10,000 units/day; 100 searches/day on free tier; quota
  extension via request form.
- RFC 7231 §7.1.3 Retry-After header semantics — basis for
  REQ-ADP5-003 parser (inherited from ADP-001 with 30s default).

### Internal (file:line cited)

- `.moai/specs/SPEC-ADP-005/research.md` — full research artifact
  for this SPEC; URL-cited externals, file:line-cited internals.
- `.moai/specs/SPEC-ADP-001/spec.md` — Reddit reference adapter;
  this SPEC inherits structure verbatim with YouTube-specific
  deltas.
- `.moai/specs/SPEC-ADP-002/spec.md` — HN reference adapter; second-
  adapter validation; rule-of-three precedent for cross-adapter
  helper extraction.
- `.moai/specs/SPEC-FAN-001/spec.md` — M3 fanout; consumer of
  ADP-005's `Search` method.
- `.moai/specs/SPEC-CORE-001/spec.md` — Adapter / Capabilities /
  Query / NormalizedDoc / SourceError / DocTypeVideo contract.
- `.moai/specs/SPEC-OBS-001/spec.md` — observability bundle;
  cardinality discipline; `AdapterCalls{adapter,outcome}` allowlist.
- `.moai/specs/SPEC-IR-001/spec.md` — `Capabilities` consumer
  contract.
- `pkg/types/adapter.go:28-45` — Adapter interface 4-method shape.
- `pkg/types/capabilities.go:15-22` — DocType enum (DocTypeVideo at
  line 18).
- `pkg/types/capabilities.go:38-62` — Capabilities struct.
- `pkg/types/query.go:18-44` — Query struct + Filter shape.
- `pkg/types/errors.go:14-218` — SourceError, Category enum,
  CategorizeError, OutcomeFromError, ValidationError.
- `pkg/types/normalized_doc.go:40-106` — NormalizedDoc 15-field
  struct, Validate, CanonicalHash.
- `internal/adapters/registry.go:75-167` — Registry lifecycle.
- `internal/adapters/registry.go:172-263` — wrappedAdapter sole-
  emitter pattern.
- `internal/adapters/noop/noop.go:1-46` — reference adapter shape +
  compile-time interface assertion.
- `internal/adapters/reddit/reddit.go:1-136` — Reddit Adapter struct
  pattern.
- `internal/adapters/hn/hn.go:1-138` — HN Adapter struct pattern.
- `internal/adapters/hn/search.go:1-204` — HN Search hot-path
  pattern (mirrored shape with sidecar HTTP delta).
- `services/researcher/Dockerfile` — sidecar Dockerfile reference
  (multi-stage, non-root, healthcheck).
- `services/researcher/pyproject.toml` — sidecar pyproject.toml
  reference (FastAPI + uvicorn).
- `services/researcher/.env.example` — sidecar env-var precedent.
- `.moai/project/roadmap.md:50` — M3 SPEC-ADP-005 row "yt-dlp
  metadata + transcript, rate-limit".
- `.moai/project/roadmap.md:122-123` — M3 7-way ADP-* parallelization
  gated on FAN-001.
- `.moai/project/tech.md:17` — Single-binary Go deploy principle
  (rationale for sidecar over `os/exec`).
- `.moai/project/tech.md:50` — Korean tokenizer policy (sidecar
  precedent, Korean-first posture).
- `.moai/project/tech.md:109` — adapter strategy row "YouTube |
  yt-dlp ... | none | self-throttle".
- `.moai/project/tech.md:165` — Apache-2.0 license target Decision
  Log entry.
- `.moai/project/structure.md:22` — `internal/adapters/youtube/`
  reservation.
- `.moai/project/structure.md:49-52` — services/ Python sidecar
  layout precedent.
- `.moai/project/structure.md:160` — `pkg/types` SDK boundary
  commitment.
- `.moai/config/sections/quality.yaml` — `development_mode: tdd`,
  `test_coverage_target: 85`.
- `.moai/config/sections/harness.yaml` — auto-routing to standard
  level.
- `.moai/config/sections/language.yaml` — `documentation: en`,
  `code_comments: en`.
- `.moai/config/sections/mx.yaml` — MX tag thresholds and per-file
  limits (anchor_per_file=3, warn_per_file=5).

---

*End of SPEC-ADP-005 v0.1 (DRAFT — pending plan-auditor cycle 1)*
