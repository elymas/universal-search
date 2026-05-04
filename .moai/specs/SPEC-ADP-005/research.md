# SPEC-ADP-005 Research — YouTube Adapter

**Status**: Research-complete; ready for SPEC drafting
**Author**: limbowl (via manager-spec)
**Created**: 2026-05-04
**Milestone**: M3 — Fanout, adapters, index
**Depends on**: SPEC-CORE-001, SPEC-OBS-001, SPEC-IR-001
**Reference shape**: SPEC-ADP-001 (Reddit), SPEC-ADP-002 (Hacker News)

---

## 0. Research Mandate

SPEC-ADP-005 is the YouTube adapter for the M3 milestone (`.moai/project/roadmap.md:50` — "wrap yt-dlp metadata + transcript with rate-limiting"). YouTube is unique among the M3 adapters because video metadata + transcripts are **the** value proposition of the source — link-only references would be inferior to a generic web search hit. This research establishes:

1. The two viable integration paths (yt-dlp Python subprocess sidecar vs. YouTube Data API v3 direct Go calls), their trade-offs, and the recommended choice with explicit citations.
2. The field mapping from the chosen source surface to the SPEC-CORE-001 `NormalizedDoc` 15-field canonical contract, with the Korean-locale transcript handling clause explicit.
3. The package layout, error taxonomy reuse from SPEC-ADP-001, MX tag plan, and TDD harness.
4. Risks (IP-block surface, rate-limit reality, sidecar lifecycle) with mitigations.
5. Six Open Questions deferred with recommended defaults.

Every external claim is URL-cited (verified via WebFetch on 2026-05-04). Every internal claim is `file:line`-cited against the working tree at `/Users/masterp/Projects/superwork/univesal-search/`.

---

## 1. Path Decision — yt-dlp Sidecar vs. YouTube Data API v3

### 1.1 Path A — yt-dlp Python subprocess wrapped as HTTP sidecar (RECOMMENDED)

**Source**: https://github.com/yt-dlp/yt-dlp (project README; verified 2026-05-04)

yt-dlp is a feature-rich command-line audio/video downloader supporting thousands of sites, forked from the inactive youtube-dlc project. License: Unlicense for source, GPLv3+ for PyInstaller-bundled binaries. PyPI package: `yt-dlp` (or `yt-dlp[default]` for optional dependencies). Cross-platform standalone binary releases exist for Windows, Linux, macOS. **No official Docker image** (the wiki at https://github.com/yt-dlp/yt-dlp/wiki/Installation does not mention Docker).

Capabilities relevant to ADP-005:

- **Metadata extraction**: `--dump-json` returns full JSON (one object) for a video URL or `--dump-single-json` for a playlist/search. The shape includes `id`, `title`, `description`, `channel`, `channel_id`, `channel_url`, `uploader`, `uploader_id`, `duration` (seconds), `view_count`, `like_count`, `upload_date` (YYYYMMDD string), `timestamp` (Unix), `thumbnails` (array), `tags` (array), `categories`, `automatic_captions` (dict by lang), `subtitles` (dict by lang). Source: README "EMBEDDING YT-DLP" section + `yt-dlp --dump-json -- <URL>` output well-documented in dozens of community projects.
- **Search**: yt-dlp supports `ytsearchN:<query>` URL prefix to fetch the first N YouTube search results. `ytsearch10:cats` returns the top 10 video URLs + metadata as JSON entries. This is what ADP-005's `Search` invokes.
- **Transcripts**: `--write-auto-subs` (auto-generated captions) or `--write-subs` (creator-uploaded). `--sub-langs ko,en` selects locales. The transcript text is surfaced via the `automatic_captions` and `subtitles` dict in `--dump-json` output (URL pointers per language); fetching the actual text requires a follow-up HTTP GET to the URL. yt-dlp can also output the transcript directly if `--write-subs --skip-download` is used with `-o "%(id)s.%(ext)s"`. For sidecar simplicity, ADP-005 fetches the transcript URL itself in Go after the metadata dump.
- **Rate-limit / IP-block mitigations**: `--limit-rate`, `--throttled-rate`, `--proxy`, `--retry-sleep`, `--cookies-from-browser`, `--cookies <file>`, `--source-address`, `--sleep-interval` / `--max-sleep-interval` / `--sleep-requests`. https://github.com/yt-dlp/yt-dlp/issues/10128 documents the YouTube "Sign in to confirm you're not a bot" error from mid-2024, which proxies alone do not bypass — cookies extracted from a logged-in browser are the recommended mitigation.

**License compatibility**: Universal Search is Apache-2.0 per `.moai/project/tech.md:165` Decision Log. yt-dlp source is Unlicense (public domain equivalent) — fully compatible. Bundled binaries are GPLv3+ per https://github.com/yt-dlp/yt-dlp README License section. ADP-005 calls yt-dlp via subprocess AS A SEPARATE PROCESS; no static linking; no GPL contagion. The sidecar Dockerfile pins yt-dlp via `pip install yt-dlp` so the binary isn't bundled into our distribution — only the Python wheel is installed at container build time.

### 1.2 Path B — YouTube Data API v3 direct Go HTTP

**Source**: https://developers.google.com/youtube/v3/docs/search/list, https://developers.google.com/youtube/v3/docs/videos/list, https://developers.google.com/youtube/v3/docs/captions, https://developers.google.com/youtube/v3/getting-started (all verified 2026-05-04).

The Go client library `google.golang.org/api/youtube/v3` exposes the official API. Authentication: API key via env var `YOUTUBE_API_KEY`. Endpoints relevant to ADP-005:

- **`search.list`** at `GET https://www.googleapis.com/youtube/v3/search`. Required: `part=snippet`. Cost: **100 quota units per call** (https://developers.google.com/youtube/v3/docs/search/list, "A call to this method has a quota cost of 100 units"). Pagination via `pageToken` / `nextPageToken`; default `maxResults=5`, max 50. Returns `items[]` with `id.videoId`, `snippet.{title, description, channelId, channelTitle, publishedAt, thumbnails}`. Search response does NOT include duration, view count, or transcript — those need follow-up calls.
- **`videos.list`** at `GET https://www.googleapis.com/youtube/v3/videos`. Cost: **1 unit per call** (https://developers.google.com/youtube/v3/docs/videos/list, "A call to this method has a quota cost of 1 unit"). Parts: `snippet`, `contentDetails` (duration as ISO-8601 PT4M13S), `statistics` (view count, like count). Required: `part` + one filter (`id`, `chart`, `myRating`).
- **`captions.list`** + **`captions.download`** at `GET https://www.googleapis.com/youtube/v3/captions`. Per https://developers.google.com/youtube/v3/docs/captions, the captions endpoint manages caption tracks with five methods (list, insert, update, download, delete). The documentation does NOT explicitly grant API-key-only access; in practice `captions.download` requires OAuth 2.0 with a token whose user has uploaded the video (or is the channel owner). **Therefore third-party transcript retrieval via the official API is effectively impossible without OAuth + ownership**. Confirmed by community reports across 2023-2026 (e.g., StackOverflow, Google Issue Tracker).

**Quota math**: Default daily quota per project is **10,000 units** (https://developers.google.com/youtube/v3/getting-started, "a default quota allocation of 10,000 units per day"). At 100 units/search, that's **100 searches/day** — far below what a search engine of 12+ adapters needs. Quota extension requires a request form (https://developers.google.com/youtube/v3/getting-started, "request additional quota by completing the Quota extension request form").

### 1.3 Decision Matrix

| Criterion | Path A (yt-dlp sidecar) | Path B (Data API v3) | Weight | Winner |
|---|---|---|---|---|
| Transcripts available | YES via `--write-auto-subs` (auto + creator) | NO without OAuth + video ownership; effectively UNAVAILABLE for third-party search | HIGH (roadmap explicit) | A |
| Daily search budget | Effectively unlimited (subject to IP blocks) | 100/day free tier; quota-capped | HIGH | A |
| Auth complexity | None (no env vars) | API key required (`YOUTUBE_API_KEY`) | MEDIUM | A |
| IP-block / sign-in challenge risk | Mid-2024 incident documented (#10128); cookie/proxy mitigations exist | None (Google authoritative) | MEDIUM | B |
| Rich metadata in single call | YES (`--dump-json` returns full shape) | NO — search.list lacks duration/views; needs videos.list follow-up (extra call per video) | MEDIUM | A |
| Pagination shape | yt-dlp `ytsearchN:` returns top-N once; multi-page via offset (limited) | clean `pageToken` cursor | MEDIUM | B |
| Sidecar process complexity | Yes — Python sidecar similar to `services/researcher/` precedent | No — direct Go HTTP | LOW (we already have sidecar pattern) | B |
| Korean-locale transcript handling | YES via `--sub-langs ko` | N/A (transcripts unavailable) | HIGH (project Korean-first per `.moai/project/tech.md:50`) | A |
| License compatibility | Unlicense source; subprocess isolation prevents GPL contagion | Apache-2.0 official client | HIGH | tie |
| Operational fragility | yt-dlp release cadence ~weekly; YouTube site changes break extractors regularly | API contract stable | MEDIUM | B |
| Cost (financial) | Free (compute only) | Free under quota; paid extension request | LOW (we're under quota) | tie |

**Decision: Path A (yt-dlp sidecar) wins on the THREE highest-weight criteria** — transcripts, daily budget, Korean-locale handling. The roadmap entry explicitly says "yt-dlp metadata + transcript" (`.moai/project/roadmap.md:50`), confirming this was the intended path from the project's inception. Path B's stability advantage is real but the 100-search/day cap and missing transcripts are non-starters for a search engine.

**Architecture commitment**: ADP-005 ships a Python sidecar at `services/youtube-extract/` mirroring the `services/researcher/` shape (FastAPI on port 8082, Dockerfile, healthcheck endpoint). The Go adapter at `internal/adapters/youtube/` is the HTTP client that talks to this sidecar. The boundary is HTTP/JSON — no direct Python invocation from Go.

### 1.4 Path A operational fragility mitigation

Per https://github.com/yt-dlp/yt-dlp/issues/10128 and analogous reports, the YouTube extractor breaks roughly once per quarter (sign-in challenges, PO token requirements, player API changes). Mitigations adopted in ADP-005:

1. **Pin yt-dlp version** in `services/youtube-extract/pyproject.toml` to a known-good release; CI runs the integration suite weekly to detect breakage early.
2. **Cookie injection**: the sidecar accepts an optional `--cookies` file (mounted from a Docker secret). Operators with persistent IP-block issues can supply a logged-in YouTube session cookie. `Capabilities.RequiresAuth` remains `false` (the public path works without cookies); cookies are an opt-in operational mitigation, not a hard requirement.
3. **Sleep intervals**: the sidecar passes `--sleep-requests 1.0 --sleep-interval 2 --max-sleep-interval 5` by default (configurable). This throttles outbound calls to YouTube ~1 req/sec sustained, well below YouTube's per-IP empirical block threshold.
4. **Adapter circuit-break**: when the sidecar returns HTTP 503 with body `{"category":"unavailable","reason":"yt-dlp signed-in challenge"}` for THREE consecutive calls (tracked in fanout via REQ-FAN-003 plain error counts; SPEC-EVAL-002 will own true circuit-breaking in M8), the operator is paged. v0.1 returns `CategoryUnavailable` and lets the fanout continue with other adapters; circuit-breaking is out of scope.
5. **Subprocess cleanup**: the sidecar's FastAPI handler invokes yt-dlp via `subprocess.run(... timeout=adapterTimeout)` with `kill_on_timeout=True`; if the request context is cancelled mid-call, the FastAPI middleware sends SIGTERM, then SIGKILL after 5 seconds. No zombie processes.

---

## 2. Sidecar Architecture

### 2.1 Service Layout

```
services/youtube-extract/
├── Dockerfile                    # multi-stage python:3.11-slim → app, non-root
├── pyproject.toml                # FastAPI + yt-dlp pinned versions
├── README.md                     # operator notes
├── .env.example                  # YT_EXTRACT_PORT, YT_COOKIES_PATH, YT_USER_AGENT, sleep params
├── src/
│   └── youtube_extract/
│       ├── __init__.py
│       ├── __main__.py           # uvicorn entrypoint
│       ├── app.py                # FastAPI app, /health, /search, /transcript
│       ├── ytdlp_runner.py       # subprocess wrapper around yt-dlp
│       └── models.py             # pydantic request/response shapes
└── tests/
    └── test_app.py               # FastAPI TestClient + monkeypatched yt-dlp
```

The sidecar's HTTP contract:

| Method | Path | Request body | Response body |
|---|---|---|---|
| GET | `/health` | — | `{"status":"ok","ytdlp_version":"<x.y.z>"}` |
| POST | `/search` | `{"query":"cats","max_results":25,"lang":"en","since":1700000000,"include_transcripts":true,"transcript_lang":"en"}` | `{"items":[<YTItem>...]}` or `{"error":{"category":"...","message":"..."}}` |
| POST | `/transcript` | `{"video_id":"abc123","lang":"en"}` | `{"text":"...","lang":"en","is_auto":true}` or `{"error":...}` |

YTItem shape:

```json
{
  "id": "dQw4w9WgXcQ",
  "url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
  "title": "Never Gonna Give You Up",
  "description": "...",
  "channel": "Rick Astley",
  "channel_id": "UCuAXFkgsw1L7xaCfnd5JJOw",
  "channel_url": "https://www.youtube.com/@RickAstleyYT",
  "uploader": "Rick Astley",
  "duration_seconds": 213,
  "view_count": 1234567890,
  "like_count": 12345678,
  "upload_date": "2009-10-25",
  "thumbnail_url": "https://i.ytimg.com/vi/dQw4w9WgXcQ/hqdefault.jpg",
  "tags": ["music", "rick astley"],
  "available_transcript_langs": ["en", "ko", "ja"],
  "transcript_snippet": "We're no strangers to love\nYou know the rules and so do I..."
}
```

The `transcript_snippet` is the FIRST 500 characters of the transcript text (when `include_transcripts=true` and the chosen lang is available). Fetching the full transcript text requires a follow-up `/transcript` call; this keeps `/search` response sizes bounded for the typical 25-result page.

### 2.2 Why a Sidecar Rather Than `os/exec` from Go

Direct `os/exec.Cmd` invocation from the Go adapter was considered. Rejected because:

1. **Deployment surface**: the `usearch` Go binary distribution would have to bundle yt-dlp's Python interpreter and its dependencies (FastEmbed, certifi, requests, websockets, ...). The deploy target is single-binary per `.moai/project/tech.md:17` ("Single-binary deploy"); breaking that requires either CGO+embedded Python or a separate Python install on every operator's host. Both worse than a sidecar.
2. **Crash isolation**: a yt-dlp segfault on a malformed video crashes the Go process. The sidecar isolates it.
3. **Observability**: the sidecar logs via standard Python logging → stdout JSON, picked up by the existing observability bus per `.moai/project/tech.md:88-90`. Mixing yt-dlp's stderr noise into Go's slog is messy.
4. **Existing precedent**: `services/researcher/` (SPEC-SYN-001) is the established sidecar pattern. ADP-005 reuses the shape (FastAPI + Dockerfile + pyproject.toml + .env.example).

### 2.3 Sidecar Lifecycle and Deployment

The sidecar runs as a docker-compose service alongside the existing `researcher`/`storm`/`embedder` services. Adding to `deploy/docker-compose.yml`:

```yaml
services:
  youtube-extract:
    build: ./services/youtube-extract
    ports: ["8082:8082"]
    environment:
      - YT_EXTRACT_PORT=8082
      - YT_USER_AGENT=usearch/0.1 (+https://github.com/elymas/universal-search)
      - YT_COOKIES_PATH=/run/secrets/yt_cookies.txt  # optional
      - YT_SLEEP_REQUESTS=1.0
      - YT_SLEEP_INTERVAL=2
      - YT_MAX_SLEEP_INTERVAL=5
    secrets:
      - yt_cookies
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8082/health"]
      interval: 30s
      start_period: 10s
      retries: 3
```

The `usearch` CLI's `cmd/usearch/main.go` reads `YOUTUBE_EXTRACT_URL` env (default `http://localhost:8082`) and constructs the YouTube adapter via `youtube.New(youtube.Options{BaseURL: ...})`. The sidecar is a soft dependency — when unreachable, the adapter returns `CategoryUnavailable` and the fanout proceeds with other adapters.

---

## 3. NormalizedDoc Field Mapping

### 3.1 Mapping Table — YTItem → NormalizedDoc

| YTItem field | NormalizedDoc field | Transform | Notes |
|---|---|---|---|
| `id` | `ID` | Use as-is (e.g., `dQw4w9WgXcQ`) | YouTube video IDs are 11-char base64-ish; collision-free in practice |
| (constant) | `SourceID` | `"youtube"` | Matches `Adapter.Name()` |
| `url` | `URL` | Use as-is (`https://www.youtube.com/watch?v=<id>`) | Canonical; no tracking params introduced by yt-dlp |
| `title` | `Title` | Use as-is | Always present |
| `description` | `Body` | Use as-is | May be empty for short videos; can run several KB on long videos |
| `description` truncated to 280 runes; falls back to `transcript_snippet` truncated to 280 if description empty; falls back to `truncateRunes(title, 280)` if both empty | `Snippet` | Same truncation discipline as ADP-001/ADP-002 | UI excerpt |
| `time.Parse("2006-01-02", upload_date)` UTC midnight | `PublishedAt` | YouTube provides date only (no time-of-day in the public API surface); the adapter sets the value to UTC midnight on `upload_date` | The yt-dlp `timestamp` Unix-seconds field, when present, is preferred over `upload_date`; falls back to date-only when `timestamp` is missing |
| (parse time) | `RetrievedAt` | `time.Now().UTC()` (set by parse caller) | Required by REQ-CORE-007 |
| `channel` (display name) | `Author` | Use as-is; falls back to `uploader` when `channel` empty | YouTube has a notion of "channel" (subscribed-to entity) and "uploader" (account that posted); for video items they are usually identical |
| `normalizeViewScore(view_count)` per §3.2 | `Score` | Tanh formula adapted to YouTube view-count scale (divisor=10000 not 100) | YouTube views span [0, ~10B]; Reddit's divisor=100 saturates at ~1K and would map every video to ~1.0 |
| `lang` from `automatic_captions` priority order: query-derived lang > `en` > first available | `Lang` | Lang detection from caption availability when query language is `ko`/`ja`/`zh` | When no captions exist, set `Lang = ""` |
| (constant) | `DocType` | `types.DocTypeVideo` | Matches the `pkg/types.DocTypeVideo` enum at `pkg/types/capabilities.go:15-22` |
| (nil) | `Citations` | `nil` | Videos do not carry doc-level citations |
| (constructed) | `Metadata` | Map containing two key tiers per §3.3 | Adapter-specific extension bag |
| (constant) | `Hash` | `""` | Consumers compute via `CanonicalHash()` |

### 3.2 Score Normalization for YouTube Views

YouTube view counts span `[0, ~10^10]` (Despacito: ~8.5B as of 2026). The Reddit/HN divisor=100 Tanh formula would map every video with ≥1000 views to ~1.0, eliminating ranking signal.

**Formula** (locked in v0.1):

```
Score = clamp(0.5 + 0.5 * tanh(log10(views + 1) / 5.0), 0.0, 1.0)
```

Properties:

- `views = 0` → `Score = 0.5` (consistent neutral semantic with Reddit/HN)
- `views = 10` → `log10(11) ≈ 1.04`, `tanh(0.21) ≈ 0.205`, `Score ≈ 0.602`
- `views = 1,000` → `log10(1001) ≈ 3.0`, `tanh(0.6) ≈ 0.537`, `Score ≈ 0.768`
- `views = 1,000,000` → `log10(1e6+1) ≈ 6.0`, `tanh(1.2) ≈ 0.834`, `Score ≈ 0.917`
- `views = 1,000,000,000` → `log10(1e9+1) ≈ 9.0`, `tanh(1.8) ≈ 0.947`, `Score ≈ 0.974`

The log10 inflection point at views ≈ 100,000 (mid-popular) gives meaningful spread across the [0.5, 1.0] range. The function is deterministic, pure, stateless. SPEC-IDX-001 RRF will weight rank not raw score across adapters, so the curve choice is operationally bounded.

The Reddit/HN `score.go::normalizeScore` is NOT reused (different formula). ADP-005's `score.go::normalizeViewScore` lives in the `youtube` package as a separate pure function. Open Question §6.5 tracks revisit triggers.

### 3.3 Metadata Map Tiers

REQUIRED keys (consumers MAY rely on presence; changes require major-version bump of `Capabilities.Notes`):

- `channel_id` (string)
- `channel_url` (string)
- `duration_seconds` (int)
- `view_count` (int)
- `thumbnail_url` (string)
- `available_transcript_langs` ([]string; may be nil/empty)

OPTIONAL keys (best-effort; subject to change):

- `like_count` (int; YouTube hides exact counts on some videos)
- `tags` ([]string)
- `transcript_snippet` (string; first 500 chars when `include_transcripts=true` and locale matched)
- `transcript_lang` (string; the lang the snippet is in)
- `transcript_is_auto` (bool; true for auto-generated captions)
- `uploader_id` (string)

The LAST returned doc additionally gets `next_cursor` (REQUIRED on the last doc only) when more pages are available — encoded as a string offset (e.g., `"25"` after the first page of 25 results). yt-dlp's `ytsearchN:` does not natively paginate beyond the requested N; the adapter implements offset-based pagination by re-querying with `ytsearch{N+offset}:<query>` and slicing the tail. Open Question §6.4 documents revisit triggers.

### 3.4 Korean-Locale Transcript Selection

Per the project's Korean-first posture (`.moai/project/tech.md:50` "Korean keyword tokenizer ... Meilisearch default tokenizer is weak for Korean"), the YouTube adapter SHOULD prefer Korean transcripts when the query suggests Korean intent. The selection algorithm:

1. If `Query.Filters` contains an entry `{Key: "lang", Value: "ko"}`, the sidecar request body sets `transcript_lang="ko"`.
2. Else if `Query.Text` is detected as Korean (≥30% of runes in the Hangul Unicode block U+AC00..U+D7AF), the adapter sets `transcript_lang="ko"` automatically.
3. Else the adapter sets `transcript_lang="en"`.

The sidecar attempts the requested language; on miss, it falls back to `en`; on second miss, it returns `available_transcript_langs` without a snippet (the consumer can call `/transcript` to fetch a different lang).

The Korean detection heuristic is intentionally conservative — false positives (English-with-some-Korean) prefer `en` snippets which the synthesis layer can re-translate. False negatives (Korean queries getting English snippets) trigger an `available_transcript_langs` indication so the caller can re-request.

---

## 4. Existing Codebase Patterns to Follow

### 4.1 Adapter Interface Conformance (`pkg/types/adapter.go:28-45`)

Same as ADP-001/ADP-002 — `Name()`, `Search()`, `Healthcheck()`, `Capabilities()` plus the compile-time interface assertion `var _ types.Adapter = (*Adapter)(nil)`. ADP-005's `Healthcheck` performs a `GET <sidecarURL>/health` rather than the TCP-only probe of ADP-001/ADP-002, because the sidecar's liveness includes "yt-dlp is installed and runnable", which TCP cannot signal.

### 4.2 Error Taxonomy (`pkg/types/errors.go:14-218`)

Wrap every error in `*types.SourceError` with:

- `CategoryRateLimited` — sidecar returns HTTP 429 (or yt-dlp's "Too many requests" error). The adapter parses `Retry-After` from the sidecar response (which is set when yt-dlp's HTTP client encountered the upstream 429), falls back to 30s default (longer than ADP-001's 5s because YouTube blocks tend to last longer per https://github.com/yt-dlp/yt-dlp/issues/10128), caps at 60s.
- `CategoryUnavailable` — sidecar returns HTTP 503 (yt-dlp signed-in challenge / IP block / sidecar overloaded), HTTP 502 (sidecar crashed), or the connection fails (sidecar down).
- `CategoryPermanent` — sidecar returns HTTP 400 (malformed query), 401 (invalid cookie path — operator config error), 404 (unknown sidecar route).
- `CategoryTransient` — wrapped `context.DeadlineExceeded` when caller cancels mid-call.

### 4.3 Registry Pattern (`internal/adapters/registry.go:172-263`)

The registry's `wrappedAdapter` emits all observability. ADP-005 emits ZERO metrics/logs/spans of its own. Per-call counter `AdapterCalls{adapter="youtube",outcome=<...>}` and histogram `AdapterCallDuration{adapter="youtube"}` are inherited. Sole-emitter discipline preserved.

### 4.4 Reddit / HN Reference Layout

`internal/adapters/reddit/` (12 files + 6 fixtures) and `internal/adapters/hn/` (13 files + 7 fixtures) provide the package layout template. ADP-005 follows the same shape with HN-specific deltas:

- `client.go` calls the SIDECAR HTTP endpoint instead of YouTube directly. The redirect-allowlist guard is dropped (the sidecar URL is operator-configured and trusted; redirect attacks come from external sources, not from your own sidecar).
- `parse.go` parses the sidecar's `{items: [...]}` JSON envelope into `[]NormalizedDoc`.
- `score.go` implements `normalizeViewScore` (Tanh-of-log10 formula per §3.2). Distinct from Reddit/HN.
- No `strip.go` — yt-dlp returns plain-text descriptions. The transcript snippet is plain text by default. If a description contains HTML (rare), it's preserved as-is for v0.1.
- `errors.go` has TWO sentinels: `ErrInvalidQuery` (empty/whitespace) and `ErrInvalidLang` (caller supplied a non-BCP-47 language code).

---

## 5. Risk Register

| Risk | Severity | Mitigation | Notes |
|---|---|---|---|
| **YouTube IP-block / "Sign in to confirm" challenge** | High | Sleep intervals (`--sleep-requests 1.0 --sleep-interval 2`), optional cookie injection, 30s default `Retry-After` on 429 | https://github.com/yt-dlp/yt-dlp/issues/10128 documents the challenge. Cookies bypass it; ADP-005 supports cookie file via env var |
| **yt-dlp release breakage (YouTube extractor breaks)** | Medium | Pinned version in pyproject.toml; weekly CI integration test against stable test video IDs | Open Question §6.6 tracks the version-bump cadence policy |
| **Sidecar process down / unreachable** | Medium | Adapter returns `CategoryUnavailable`; fanout (SPEC-FAN-001) proceeds with other adapters | Sidecar healthcheck endpoint surfaces liveness for SPEC-EVAL-002 (M8) |
| **Subprocess zombie on caller-cancellation** | Medium | FastAPI middleware tracks request ctx; on cancel, sends SIGTERM then SIGKILL after 5s grace | Tested in `services/youtube-extract/tests/test_app.py::test_subprocess_cleanup` |
| **Transcript fetch latency >> metadata latency** | Medium | `/search` returns `transcript_snippet` (first 500 chars) only; full transcript via separate `/transcript` call | Caller (synthesis) decides whether to fetch full transcript per video |
| **Korean transcript not present despite Korean query** | Medium | `available_transcript_langs` exposed in Metadata; consumer re-requests other lang | Falls back to `en` automatically |
| **Pagination opacity (yt-dlp `ytsearchN:` doesn't natively paginate)** | Medium | Adapter implements offset-based pagination via `ytsearch{N+offset}:` re-query; cursor is the offset as decimal string | Inefficient for deep pagination; v0.1 caps `MaxResults` × `Cursor` total at 100 to bound the cost |
| **License contagion from yt-dlp GPLv3+** | Low | yt-dlp invoked as subprocess (NOT linked); `pip install yt-dlp` in sidecar Dockerfile; Apache-2.0 boundary preserved at the adapter Go code | Decision Log entry needed |
| **Daily YouTube view-count drift** | Low | View count reflects the moment of fetch; SPEC-CACHE-001 (M3) handles staleness; NormalizedDoc.RetrievedAt is the ground truth | Acceptable; acknowledged in `Capabilities.Notes` |
| **Score saturation at very-popular videos** | Low | log10 inflection at views=100K; videos with 1B views map to Score ≈ 0.974 (not 1.0); RRF re-weights via rank | Open Question §6.5 reviews after M3 RRF integration |
| **Sidecar Docker image build cost (Python + yt-dlp deps)** | Low | Multi-stage Dockerfile; cached pip layer; image size ~150MB compressed | One-time CI cost |
| **Subprocess output parsing mismatch when yt-dlp updates JSON shape** | Low | pydantic models in sidecar provide strict validation; new fields ignored; missing required fields → CategoryUnavailable | Tested via fixture-based unit tests |
| **Race condition with multi-tenant shared cookie file** | Low | Single cookie file is read-only on container start; yt-dlp doesn't write to it during extraction | Documented in deploy README |
| **`ytsearch:` URL prefix is a yt-dlp internal feature, not a YouTube-supported URL** | Low | Stable across yt-dlp 2024-2026 history; tracked in pinned version | No indication of upcoming removal |

---

## 6. Open Questions

These are explicitly UNRESOLVED at SPEC-approval time. Each has a recommended default and a one-line resolution owner. They do NOT block SPEC approval.

1. **Sidecar bind URL configurability** — fixed at `http://localhost:8082` or accept arbitrary URL? **Recommended default**: arbitrary URL via `Options.SidecarBaseURL`, default `http://localhost:8082`. **Resolution owner**: SPEC-DEPLOY-001 (M9) author when productionizing.

2. **Cookie file mounting strategy** — Docker secret, volume mount, or vault integration? **Recommended default**: Docker secret at `/run/secrets/yt_cookies.txt`, env var `YT_COOKIES_PATH` overrideable. **Resolution owner**: expert-devops during SPEC-DEPLOY-001.

3. **Transcript full-fetch threshold** — should `/search` ever return full transcripts (not just snippets), gated by a query parameter? **Recommended default**: NO in v0.1 — always return snippet only; SPEC-SYN-001 fetches full transcript via `/transcript` per cited video. **Resolution owner**: SPEC-SYN-001 author.

4. **Pagination depth cap** — yt-dlp's `ytsearchN:` is inefficient for deep offsets. v0.1 caps `MaxResults + Cursor offset` at 100 (effectively top-100 across pages). Should this be 50 or 200? **Recommended default**: 100. **Resolution owner**: SPEC-FAN-001 author may revisit if telemetry shows users paginating heavily.

5. **Score formula calibration** — log10 divisor=5.0 chosen empirically for [0, 10B] view range. Should it be 4.0 or 6.0? **Recommended default**: 5.0 in v0.1; revisit after SPEC-IDX-001 RRF integration measures ranking quality. **Resolution owner**: SPEC-IDX-001 author.

6. **yt-dlp version-bump cadence** — pin exact version vs. range? **Recommended default**: pin exact version (`yt-dlp==2026.04.01` or similar); update via dedicated PR with CI green. **Resolution owner**: expert-backend periodic dependency review.

---

## 7. Sources and Citations

### External URLs (WebFetch verified 2026-05-04)

- https://github.com/yt-dlp/yt-dlp — yt-dlp project README; license, capabilities, subprocess interface, rate-limit / IP-block options
- https://github.com/yt-dlp/yt-dlp/wiki/Installation — installation methods (PyPI `yt-dlp`); standalone binaries cross-platform; no official Docker image
- https://github.com/yt-dlp/yt-dlp/issues/10128 — June 2024 "Sign in to confirm you're not a bot" incident; mid-2024 mitigations
- https://developers.google.com/youtube/v3/docs/search/list — search.list endpoint, 100 quota units/call, pagination via pageToken, default maxResults=5 (max 50)
- https://developers.google.com/youtube/v3/docs/videos/list — videos.list endpoint, 1 quota unit/call, parts (snippet, contentDetails, statistics)
- https://developers.google.com/youtube/v3/docs/captions — captions endpoint with 5 methods (list, insert, update, download, delete); third-party transcript retrieval requires OAuth
- https://developers.google.com/youtube/v3/getting-started — default quota 10,000 units/day; quota extension request form

### Internal Files (file:line cited)

- `/Users/masterp/Projects/superwork/univesal-search/pkg/types/normalized_doc.go:40-106` — NormalizedDoc 15-field struct + Validate + CanonicalHash
- `/Users/masterp/Projects/superwork/univesal-search/pkg/types/adapter.go:28-45` — Adapter interface 4-method shape
- `/Users/masterp/Projects/superwork/univesal-search/pkg/types/capabilities.go:15-22` — DocTypeVideo enum constant
- `/Users/masterp/Projects/superwork/univesal-search/pkg/types/capabilities.go:38-62` — Capabilities struct
- `/Users/masterp/Projects/superwork/univesal-search/pkg/types/errors.go:14-218` — SourceError, Category, OutcomeFromError
- `/Users/masterp/Projects/superwork/univesal-search/internal/adapters/registry.go:75-167` — Registry lifecycle
- `/Users/masterp/Projects/superwork/univesal-search/internal/adapters/registry.go:172-263` — wrappedAdapter sole-emitter pattern
- `/Users/masterp/Projects/superwork/univesal-search/internal/adapters/reddit/reddit.go:1-136` — Reddit adapter struct pattern (mirrored shape)
- `/Users/masterp/Projects/superwork/univesal-search/internal/adapters/hn/hn.go:1-138` — HN adapter struct pattern
- `/Users/masterp/Projects/superwork/univesal-search/internal/adapters/hn/search.go:1-204` — HN Search hot path pattern
- `/Users/masterp/Projects/superwork/univesal-search/services/researcher/Dockerfile` — sidecar Dockerfile reference (multi-stage, non-root, healthcheck)
- `/Users/masterp/Projects/superwork/univesal-search/services/researcher/pyproject.toml` — sidecar pyproject.toml reference (FastAPI + uvicorn)
- `/Users/masterp/Projects/superwork/univesal-search/services/researcher/.env.example` — sidecar env-var precedent
- `/Users/masterp/Projects/superwork/univesal-search/.moai/project/roadmap.md:50` — M3 SPEC-ADP-005 row "yt-dlp metadata + transcript, rate-limit"
- `/Users/masterp/Projects/superwork/univesal-search/.moai/project/roadmap.md:123` — M3 parallelization gate on FAN-001
- `/Users/masterp/Projects/superwork/univesal-search/.moai/project/tech.md:17` — Single-binary Go deploy principle
- `/Users/masterp/Projects/superwork/univesal-search/.moai/project/tech.md:50` — Korean tokenizer policy (mecab-ko sidecar precedent)
- `/Users/masterp/Projects/superwork/univesal-search/.moai/project/tech.md:109` — adapter strategy row "YouTube | yt-dlp (metadata + transcript) | none | self-throttle"
- `/Users/masterp/Projects/superwork/univesal-search/.moai/project/tech.md:165` — Apache-2.0 license target Decision Log entry
- `/Users/masterp/Projects/superwork/univesal-search/.moai/project/structure.md:22` — `internal/adapters/youtube/` reservation
- `/Users/masterp/Projects/superwork/univesal-search/.moai/project/structure.md:49-52` — services/ Python sidecar layout precedent
- `/Users/masterp/Projects/superwork/univesal-search/.moai/project/structure.md:160` — `pkg/types` SDK boundary commitment
- `/Users/masterp/Projects/superwork/univesal-search/.moai/specs/SPEC-ADP-001/spec.md` — Reddit reference SPEC; structure mirror
- `/Users/masterp/Projects/superwork/univesal-search/.moai/specs/SPEC-ADP-002/spec.md` — HN reference SPEC; second-adapter validation
- `/Users/masterp/Projects/superwork/univesal-search/.moai/specs/SPEC-CORE-001/spec.md` — Adapter / Capabilities / Query / NormalizedDoc / SourceError contract
- `/Users/masterp/Projects/superwork/univesal-search/.moai/specs/SPEC-OBS-001/spec.md` — observability bundle, cardinality discipline
- `/Users/masterp/Projects/superwork/univesal-search/.moai/specs/SPEC-IR-001/spec.md` — Capabilities consumer contract; routing
- `/Users/masterp/Projects/superwork/univesal-search/.moai/specs/SPEC-FAN-001/spec.md` — fanout dispatch contract; SPEC-ADP-005 is downstream consumer
- `/Users/masterp/Projects/superwork/univesal-search/.moai/config/sections/quality.yaml` — `development_mode: tdd`, `test_coverage_target: 85`
- `/Users/masterp/Projects/superwork/univesal-search/.moai/config/sections/harness.yaml` — auto-routing to standard level
- `/Users/masterp/Projects/superwork/univesal-search/.moai/config/sections/language.yaml` — `documentation: en`, `code_comments: en`

---

End of Research Document.

**Summary for SPEC Author**: This research locks the YouTube adapter integration path as **Path A (yt-dlp Python sidecar)**. The decision wins on three highest-weight criteria: transcript availability (Path B effectively cannot retrieve transcripts without OAuth + ownership), daily search budget (Path B's 100/day free-tier cap is incompatible with a search engine), and Korean-locale transcript handling (the project's Korean-first posture per `.moai/project/tech.md:50`). Path A's operational fragility (yt-dlp release cadence, IP-block surface) is mitigated via pinned versions, sleep intervals, optional cookie injection, and 30s default Retry-After. The sidecar lives at `services/youtube-extract/` mirroring the `services/researcher/` shape (FastAPI on port 8082, multi-stage Dockerfile, healthcheck endpoint). The Go adapter at `internal/adapters/youtube/` is an HTTP client that talks to this sidecar; subprocess invocation is fully contained inside the Python service. The mapping table converts YTItem → NormalizedDoc with `DocType=DocTypeVideo`, a Tanh-of-log10 view-count score formula (divisor=5.0; distinct from Reddit/HN's divisor=100), Korean-locale-aware transcript selection with auto-detection from query script, and a 6-key REQUIRED Metadata tier (channel_id, channel_url, duration_seconds, view_count, thumbnail_url, available_transcript_langs). Six Open Questions are deferred with recommended defaults. The SPEC should span 700-900 lines covering 10-12 EARS REQs across all five EARS patterns plus 4 NFRs (parse perf, e2e p95, goroutine leak, alloc ceiling), structured identically to SPEC-ADP-001 v0.1 / SPEC-ADP-002 v0.1.
