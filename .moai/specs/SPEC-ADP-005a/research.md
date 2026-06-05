# SPEC-ADP-005a Research — YouTube Extraction Sidecar (Build + Deploy)

**Status**: Research-complete; ready for SPEC drafting
**Author**: limbowl (via manager-spec)
**Created**: 2026-06-04
**Milestone**: M3 — Fanout, adapters, index (deployment completion of SPEC-ADP-005)
**Depends on**: SPEC-ADP-005 (parent — Go adapter, already implemented)
**Reference shape**: `services/embedder/` (SPEC-IDX-002), `services/tokenizer-ko/` (SPEC-IDX-003)

---

## 0. Research Mandate

SPEC-ADP-005a is the **deployment-completing amendment** to SPEC-ADP-005.
The parent SPEC delivered the Go-side YouTube adapter
(`internal/adapters/youtube/`, implemented 2026-05-07, 91.2% coverage) as
a pure HTTP client to a yt-dlp Python sidecar. ADP-005 §2.1(p) and §6.4
**contractually documented** the sidecar but explicitly deferred its
implementation (ADP-005 Open Question §11.7 — "(a) bundle into ADP-005's
run phase as a parallel task"). That deferral never resolved: the sidecar
directory `services/youtube-extract/` does not exist on disk, so YouTube
search cannot run end-to-end. This research establishes:

1. The exact byte-level wire contract the sidecar MUST satisfy, extracted
   from the implemented Go adapter source (not re-derived from the SPEC
   prose, which could drift).
2. The port collision discovered post-ADP-005: the adapter's
   `defaultBaseURL` hardcodes `:8082`, which collides with the embedder
   sidecar (also `:8082`). The fix and its blast radius.
3. The reference sidecar shape (`services/embedder/`,
   `services/tokenizer-ko/`) that ADP-005a mirrors for Dockerfile,
   pyproject.toml, src-layout, /health endpoint, and docker-compose
   wiring.
4. yt-dlp invocation, pinning, and rate-limit defence specifics carried
   forward from ADP-005 D2/D4.

Every internal claim is `file:line`-cited against the working tree at
`/Users/masterp/Projects/superwork/universal-search/`. External claims
reuse ADP-005 research.md's already-verified citations (yt-dlp behaviour
verified 2026-05-04; not re-verified here — see ADP-005 research §1).

---

## 1. The Problem — Sidecar Does Not Exist + Port Collision

### 1.1 Sidecar absence

`internal/adapters/youtube/youtube.go:20` sets:

```go
defaultBaseURL = "http://localhost:8082"
```

The adapter is a complete HTTP client. `Search` (`search.go:54-180`) POSTs
to `<baseURL>/search`; `Healthcheck` (`youtube.go:114-146`) GETs
`<baseURL>/health`. But there is no service listening at that URL:
`ls services/` returns `embedder koreanews researcher storm tokenizer-ko`
— no `youtube-extract`. ADP-005 §2.1(p) reserved the directory and §6.4
documented the contract, but the implementation was deferred via Open
Question §11.7 and never built.

### 1.2 Port collision (8082)

`services/embedder/src/embedder/__main__.py:14` binds the embedder to
`EMBEDDER_PORT` default `8082`; `deploy/docker-compose.yml:204` maps
`${EMBEDDER_PORT:-8082}:8082`. The YouTube adapter's compiled-in default
(`youtube.go:20`) is ALSO `:8082`. If a YouTube sidecar bound to 8082 it
would collide with the embedder. Commit `742564d` (2026-06-04,
"fix(cli): resolve F-02 — gate YouTube adapter registration behind
YOUTUBE_BASE_URL") worked around this by gating registration: at
`cmd/usearch/query.go:488` the adapter is only registered when
`YOUTUBE_BASE_URL` is set, "mirroring the GitHub token gate". With the
env unset, YouTube silently no-ops — search never reaches it.

### 1.3 Free port survey

In the docker-compose stack the sidecar ports in use are:

| Port | Service | Citation |
|------|---------|----------|
| 8080 | searxng | `docker-compose.yml:119` |
| 8081 | researcher | `docker-compose.yml:177,181` |
| 8082 | embedder | `docker-compose.yml:204`, `embedder/__main__.py:14` |
| 8083 | tokenizer-ko | `docker-compose.yml:320,322`, `tokenizer-ko/Dockerfile:56` |
| 8084 | **FREE** | not present in `docker-compose.yml` |

8084 appears only in `charts/universal-search/values.yaml:481` (Helm
chart for `storm`), which is NOT part of the docker-compose stack the
operator runs locally (`storm` has no service block in
`deploy/docker-compose.yml`). Therefore **8084 is the next free port in
the compose stack** and is the user-locked choice (D1).

---

## 2. Wire Contract Extracted From Go Adapter Source (Ground Truth)

The sidecar contract is NOT re-derived from SPEC prose. It is extracted
verbatim from the implemented adapter so the Python side is byte-accurate.

### 2.1 Request body — `POST /search`

Source: `internal/adapters/youtube/search.go:37-44` (`searchRequestBody`
struct) and `:120-128` (construction).

```go
type searchRequestBody struct {
    Query              string `json:"query"`
    MaxResults         int    `json:"max_results"`
    CursorOffset       int    `json:"cursor_offset"`
    TranscriptLang     string `json:"transcript_lang"`
    IncludeTranscripts bool   `json:"include_transcripts"`
    Since              *int64 `json:"since,omitempty"`
}
```

Adapter guarantees the sidecar can rely on (from `search.go`):
- `query`: never empty/whitespace (`:65` rejects before HTTP issued).
- `max_results`: always in `[1, 100]` (`:88-94` clamp; default 25).
- `cursor_offset`: always present, `>= 0` (`:74-85`); `0` = first page.
- `max_results + cursor_offset`: always `<= 100` (`:97` cap enforced
  before HTTP).
- `transcript_lang`: always present, non-empty (`selectTranscriptLang`
  at `:106` returns `"en"` as floor).
- `include_transcripts`: hardcoded `true` for v0.1 (`:126`).
- `since`: present only when a positive int64 `since` filter parsed
  (`:109-118`; `omitempty` drops it otherwise).
- `Content-Type: application/json` (`:148`).
- `User-Agent: usearch/<ver> (+https://github.com/elymas/universal-search)`
  (`client.go:40`, `youtube.go:27`).
- `Accept: application/json` (`client.go:41`).

### 2.2 Success response — HTTP 200

Source: `internal/adapters/youtube/parse.go:22-63` (`ytSearchResponse` +
`ytItem` structs — the authoritative JSON shape the Go side unmarshals).

```jsonc
{
  "items": [
    {
      "id": "dQw4w9WgXcQ",                 // string, used as NormalizedDoc.ID
      "url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
      "title": "Never Gonna Give You Up",
      "description": "...",                 // → Body; "" allowed
      "channel": "Rick Astley",            // → Author (falls back to uploader)
      "channel_id": "UC...",               // REQUIRED Metadata key
      "channel_url": "https://...",        // REQUIRED Metadata key
      "uploader": "Rick Astley",
      "uploader_id": "RickAstleyYT",       // OPTIONAL Metadata key
      "duration_seconds": 213,             // int64; 0 for livestream-archived; REQUIRED key
      "view_count": 1234567890,            // int64 OR null (livestream); null→0→Score 0.5; REQUIRED key
      "like_count": 12345678,              // int64 OR null/absent; OPTIONAL key
      "upload_date": "2009-10-25",         // "2006-01-02" layout; parsed UTC midnight
      "thumbnail_url": "https://...",      // REQUIRED Metadata key
      "tags": ["music"],                   // []string; OPTIONAL key
      "available_transcript_langs": ["en","ko"], // []string, NEVER null/absent; REQUIRED key
      "transcript_snippet": "We're no...", // string; sidecar SHOULD cap ~500 runes (adapter re-caps)
      "transcript_lang": "en",             // string
      "transcript_is_auto": false,         // bool
      "error": null                        // OPTIONAL; non-null → adapter skips item silently
    }
  ],
  "has_more": true                         // bool; true → adapter emits next_cursor
}
```

Critical observations from `parse.go`:
- `view_count` is a **JSON pointer** (`*int64` at `parse.go:51`) — the
  sidecar MUST emit `null` (not omit, not 0) for livestream-archived
  videos to trigger the `Score=0.5` path; `0` also works.
- `available_transcript_langs` MUST be present and an array (may be
  empty `[]`) — the adapter normalises `nil` to `[]string{}` at
  `parse.go:181-184` defensively, but the contract is "always present".
- A per-item `"error"` field (`*string` at `parse.go:62`), when
  non-null, causes the adapter to **skip that item silently** with no
  log (`parse.go:116-118`, sole-emitter discipline). This lets the
  sidecar return partial success: some videos extracted, others failed.
- `upload_date` parsed with Go layout `"2006-01-02"` (`parse.go:153`)
  — the sidecar MUST emit `YYYY-MM-DD` (NOT yt-dlp's native
  `YYYYMMDD`; the sidecar reformats). A malformed/empty date yields a
  zero `PublishedAt` (no error).
- `has_more=true` at `parse.go:127` drives `next_cursor` emission when
  `currentOffset + len(items) < 100`.

### 2.3 Error response — HTTP non-200

Source: `internal/adapters/youtube/parse.go:29-35` (`ytErrEnvelope`) and
`search.go:200-229` (`handleNonOK`).

```json
{ "error": { "category": "unavailable", "message": "yt-dlp signed-in challenge" } }
```

- `category` ∈ {`unavailable`, `permanent`, `transient`, `rate_limited`}
  (`parse.go:244-256` maps each to `types.Category`; unknown →
  `unavailable`).
- The 503 yt-dlp-challenge case uses `"reason"` instead of `"message"`
  in some bodies — the adapter accepts both (`parse.go:33-34,97-100`).
- HTTP status drives categorisation FIRST (`search.go:201-229`):
  - **429** → `parseRetryAfter(Retry-After)` (`errors.go:44`); 30s
    default, 60s cap → `CategoryRateLimited`.
  - **5xx** → `CategoryUnavailable`; if body has the error envelope, the
    inner `message`/`reason` is preserved in the Cause chain
    (`search.go:211-225`).
  - **4xx (non-429)** → `CategoryPermanent` (`search.go:228`).

So the sidecar's HTTP **status code** is what the adapter primarily keys
on; the JSON envelope only enriches the error message. The sidecar MUST:
- Return **429** + `Retry-After` header when yt-dlp is rate-limited.
- Return **503** + the `{"error":{"category":"unavailable","reason":
  "yt-dlp signed-in challenge"}}` body when it hits the
  "Sign in to confirm you're not a bot" challenge.
- Return **4xx** (e.g., 400) for malformed requests (permanent).

### 2.4 Health response — `GET /health`

Source: `internal/adapters/youtube/youtube.go:114-146`.

```json
{ "status": "ok", "ytdlp_version": "2026.04.01" }
```

- Adapter requires HTTP **200** AND a JSON body whose `status` field
  equals exactly `"ok"` (`youtube.go:127-144`). Any other status string,
  non-200 code, or unparseable body fails the healthcheck.
- The embedder precedent (`embedder/app.py:100-116`) returns 503 with
  `{"status":"loading"}` while not ready and 200 `{"status":"ok",...}`
  when ready — the YouTube sidecar mirrors this: 200/`ok` once yt-dlp is
  importable, 503 otherwise.
- `ytdlp_version` is informational (the Go side ignores it beyond
  parsing `status`); ADP-005 §6.4 documents it for operator visibility.

---

## 3. Reference Sidecar Shape (to mirror)

### 3.1 Directory layout (src-layout)

From `services/embedder/` and `services/tokenizer-ko/`. NOTE: only
`services/embedder/` ships a `README.md` and `.env.example`;
`services/tokenizer-ko/` has neither (verified by directory listing).
ADP-005a mirrors **embedder** for those two files and **tokenizer-ko**
for the multi-stage Dockerfile.

```
services/<name>/
├── Dockerfile          # multi-stage python:3.11-slim, non-root, HEALTHCHECK
├── pyproject.toml      # fastapi + uvicorn[standard] + pydantic; hatchling; ruff; pytest
├── README.md           # operator notes (embedder precedent only)
├── .env.example        # env vars (embedder precedent only)
├── src/<pkg>/
│   ├── __init__.py
│   ├── __main__.py     # uvicorn.run(app, host="0.0.0.0", port=<PORT>, workers=1)
│   ├── app.py          # FastAPI app, lifespan, /health + business endpoint
│   ├── models.py       # pydantic request/response
│   └── <runner>.py     # the domain logic (embed.py / tokenize.py analogue)
└── tests/
    ├── conftest.py
    ├── test_app.py     # FastAPI TestClient against /health + endpoint
    └── test_<runner>.py
```

### 3.2 Dockerfile pattern

Two valid precedents:
- **embedder** (`embedder/Dockerfile`): single-stage `python:3.11-slim`,
  `useradd --uid 1001 appuser`, `EXPOSE 8082`, `HEALTHCHECK` via
  `urllib.request.urlopen`, `USER appuser`, `CMD ["python","-m","embedder"]`.
- **tokenizer-ko** (`tokenizer-ko/Dockerfile`): true multi-stage
  (builder installs into `--prefix=/install`, runtime copies
  `/install → /usr/local`), `useradd -r appuser`, `EXPOSE 8083`,
  `HEALTHCHECK`, `USER appuser`. Lighter runtime image.

ADP-005a uses the **multi-stage tokenizer-ko pattern** (cleaner runtime
image; yt-dlp has no heavy C-extension build deps, only `ffmpeg` runtime
optional — but transcript fetch is pure-Python via httpx, so ffmpeg is
NOT required for metadata + caption extraction; ADP-005 §6.4 only needs
`--dump-json` + caption URL fetch).

### 3.3 pyproject.toml dependency floor

From `embedder/pyproject.toml` + ADP-005 §9.5:
- `fastapi>=0.115`
- `uvicorn[standard]>=0.30`
- `pydantic>=2.9`
- `httpx>=0.27` (fetch caption URLs)
- `yt-dlp==<pinned>` (e.g. `2026.04.01`) — **exact pin**, not `>=`
- dev: `ruff`, `pytest`, `pytest-asyncio`, `pytest-cov`
- `license = "Apache-2.0"`, `requires-python = ">=3.11"`,
  `build-backend = "hatchling.build"`, `packages = ["src/youtube_extract"]`

### 3.4 docker-compose service block template

From `docker-compose.yml:312-335` (tokenizer-ko, the closest analogue —
no model volume, simple healthcheck):

```yaml
  youtube-extract:
    build:
      context: ../services/youtube-extract
      dockerfile: Dockerfile
    ports:
      - "${YOUTUBE_PORT:-8084}:8084"
    environment:
      YT_EXTRACT_PORT: "8084"
      YT_COOKIES_PATH: ${YT_COOKIES_PATH:-}
      YT_SLEEP_REQUESTS: ${YT_SLEEP_REQUESTS:-1.0}
      YT_SLEEP_INTERVAL: ${YT_SLEEP_INTERVAL:-2}
      YT_MAX_SLEEP_INTERVAL: ${YT_MAX_SLEEP_INTERVAL:-5}
    healthcheck:
      test: ["CMD-SHELL", 'python -c "import urllib.request; urllib.request.urlopen(''http://localhost:8084/health'')" || exit 1']
      interval: 15s
      timeout: 5s
      retries: 5
      start_period: 30s
    restart: unless-stopped
    networks:
      - app
```

Note the embedder uses `EMBEDDER_PORT` env both in compose and `__main__`;
the sidecar's internal `__main__.py` reads its OWN port env. The Go-side
override env is `YOUTUBE_BASE_URL` (the URL, not the port) per
`cmd/usearch/query.go:488` and ADP-005 Open Question §11.1.

### 3.5 .env.example additions

Current root `.env.example` has NO YouTube vars (grep confirms only
`EMBEDDER_*`, `TOKENIZER_KO_*`, etc.). The embedder block at
`.env.example:91,95` is the precedent:
```
EMBEDDER_BASE_URL=http://localhost:8082
EMBEDDER_PORT=8082
```
ADP-005a adds the YouTube analogue: `YOUTUBE_BASE_URL`, `YOUTUBE_PORT`,
`YT_COOKIES_PATH` (+ optional sleep knobs).

---

## 4. Port Reassignment Blast Radius (D1)

Changing `youtube.go:20` `defaultBaseURL` from `:8082` to `:8084`:

| Touched | File:line | Change |
|---------|-----------|--------|
| Go const | `youtube.go:20` | `:8082` → `:8084` |
| Go const doc | `youtube.go:2` | package doc "port 8082 by default" → 8084 |
| Capabilities.Notes | `youtube.go:34` | `"port 8082 (default)"` → `"port 8084 (default)"` |
| Test assertions | `youtube_test.go` (TestCapabilitiesShape) | Notes substring may assert "8082"; verify + update if so |

The `YOUTUBE_BASE_URL` gating (`query.go:488`) is RETAINED — it remains
the production override and the integration-test redirect hook. With the
sidecar deployed, operators set `YOUTUBE_BASE_URL=http://youtube-extract:8084`
(compose) or `http://localhost:8084` (local). The default `:8084` only
matters when the env is unset AND registration is forced; in practice
`YOUTUBE_BASE_URL` is always set when the sidecar runs. The const fix
ensures the default is at least non-colliding if someone removes the gate.

**Risk**: low. The Go change is a single-constant edit + doc/notes sync +
one possible test-string update. No logic change. Greenfield on the
Python side.

---

## 5. yt-dlp Invocation (carried from ADP-005 D2/D4)

The sidecar runs yt-dlp as a **subprocess** (GPL process-isolation — no
linking; ADP-005 risk row "License contagion", research §1.1):

- Search: `ytsearch{max_results+cursor_offset}:<query>` with
  `--dump-single-json`, then slice `[cursor_offset : cursor_offset+max_results]`.
- Rate-limit flags (ADP-005 D4): `--sleep-requests 1.0 --sleep-interval 2
  --max-sleep-interval 5` (env-tunable via `YT_SLEEP_*`).
- Optional cookies: `--cookies <YT_COOKIES_PATH>` when the env is set
  (ADP-005 D3 — optional IP-block mitigation, not required).
- Transcripts: when `include_transcripts=true`, read `automatic_captions`
  /`subtitles` from the dump-json, fetch the `transcript_lang` caption URL
  via httpx, extract first ~500 runes → `transcript_snippet`.
- Korean handling (ADP-005 D6): the adapter already sends
  `transcript_lang="ko"` for Korean queries; the sidecar tries `ko` first,
  falls back to `en`, populates `available_transcript_langs` always.

### 5.1 Pin + upgrade policy

yt-dlp's YouTube extractor breaks ~quarterly (ADP-005 risk row). Pin
exact version in `pyproject.toml`; document the bump procedure in
`README.md` (mirrors `.claude/rules/moai/core/lsp-client.md` upgrade-policy
style): bump in a branch, run sidecar pytest + a manual smoke against a
stable video id (`dQw4w9WgXcQ`), then update the pin line.

### 5.2 Subprocess cleanup

ADP-005 NFR-ADP5-004 (sidecar subprocess cleanup) and risk row
"Subprocess zombie on caller-cancellation": the sidecar's FastAPI handler
must SIGTERM then SIGKILL-after-grace the yt-dlp subprocess on request
cancellation. Tested in the sidecar pytest suite (mock subprocess).

---

## 6. Testing Strategy (no live network)

ADP-005 D8 + §2.2 mandate NO live YouTube in CI. The sidecar tests:
- Mock the yt-dlp subprocess (patch the runner's `subprocess.run`/
  `asyncio.create_subprocess_exec`) with golden JSON fixtures of
  `--dump-single-json` output.
- Mock httpx caption fetch (respx or monkeypatch) with golden caption text.
- FastAPI `TestClient` drives `/health` (200 ok) and `/search` (happy
  path, empty results, per-item error skip, 429 mapping, 503 challenge,
  Korean transcript, pagination has_more).
- Golden fixtures live under `services/youtube-extract/tests/fixtures/`,
  reusing the 6 existing Go fixtures under
  `internal/adapters/youtube/testdata/` (happy, empty, pagination,
  korean, no-transcript, malformed) plus 3 fresh ones the Go side lacks
  (429, 503-challenge, per-item-error). The Go 429/503 tests synthesise
  status codes at the httptest.Server, so no fixture file exists for
  those paths to copy.

---

## 7. Open Questions

1. **ffmpeg in image?** Metadata + caption-URL fetch needs no ffmpeg.
   Recommended default: **omit ffmpeg** (smaller image). Revisit only if
   a future SPEC needs audio/video download. Owner: expert-devops.
2. **yt-dlp pin value** — exact tag (e.g. `2026.04.01` vs latest stable
   at build time). Recommended default: pin the latest stable at
   implementation time, record in README. Owner: expert-devops at run.
3. **Cookie mount strategy** — Docker secret vs volume. Inherited from
   ADP-005 Open Question §11.2; recommended `YT_COOKIES_PATH` env +
   optional bind mount. Owner: expert-devops.
4. **Worker count** — uvicorn `workers=1` (embedder/tokenizer precedent)
   vs >1 for concurrent yt-dlp calls. Recommended default: `workers=1`
   in v0.1 (yt-dlp subprocess is the parallelism unit; FastAPI async
   handles concurrency). Revisit under load. Owner: expert-devops.

---

## 8. References (internal, file:line cited)

- `internal/adapters/youtube/youtube.go:20,27,34,114-146` — defaultBaseURL,
  UA template, Notes, Healthcheck contract.
- `internal/adapters/youtube/search.go:37-44,54-180,200-229` —
  request body struct, Search hot path, non-OK handling.
- `internal/adapters/youtube/parse.go:22-63,116-118,127,153,181-184,244-256`
  — response envelope structs, item-skip, cursor, date layout, category map.
- `internal/adapters/youtube/errors.go:44` — parseRetryAfter (30s/60s).
- `internal/adapters/youtube/client.go:20-25,40-41` — 30s timeout, headers.
- `cmd/usearch/query.go:449,488-494` — YOUTUBE_BASE_URL registration gate.
- `services/embedder/Dockerfile`, `embedder/pyproject.toml`,
  `embedder/.env.example`, `embedder/src/embedder/{app,__main__}.py` —
  reference sidecar (single-stage, lifespan, /health 200/503).
- `services/tokenizer-ko/Dockerfile` — multi-stage reference pattern.
- `deploy/docker-compose.yml:198-226` (embedder block),
  `:312-335` (tokenizer-ko block) — compose service templates.
- `deploy/docker-compose.yml:177,204,320` — port occupancy (8081/8082/8083).
- `.env.example:91,95,112,118` — sidecar env-var precedent (no YouTube vars).
- `charts/universal-search/values.yaml:481` — 8084 used only by storm in
  Helm (NOT compose), confirming 8084 free in the compose stack.
- `deploy/prometheus/alerts-test.yml` — references
  `usearch:adapter_fanout_partial_ratio_24h{adapter="youtube"}` (the
  metric is emitted by the registry wrappedAdapter once the adapter is
  registered; no sidecar change needed).
- `.moai/specs/SPEC-ADP-005/spec.md` §2.1(p), §6.4, §11.7 — parent
  sidecar contract + deferral.
- `.moai/specs/SPEC-ADP-005/research.md` §1 — yt-dlp behaviour
  (externally verified 2026-05-04; reused here).

---

*End of SPEC-ADP-005a research.md*
