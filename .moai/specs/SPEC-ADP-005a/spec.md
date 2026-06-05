---
id: SPEC-ADP-005a
title: YouTube Extraction Sidecar (Build + Deploy)
version: 0.1.0
milestone: M3 — Fanout, adapters, index
status: implemented
priority: P1
owner: expert-devops
methodology: tdd
coverage_target: 85
created: 2026-06-04
updated: 2026-06-04
author: limbowl
issue_number: null
depends_on: [SPEC-ADP-005]
blocks: []
---

# SPEC-ADP-005a: YouTube Extraction Sidecar (Build + Deploy)

## HISTORY

- 2026-06-04 (initial draft v0.1, limbowl via manager-spec):
  Deployment-completing AMENDMENT to SPEC-ADP-005. The parent SPEC
  shipped the Go-side YouTube adapter (`internal/adapters/youtube/`,
  implemented 2026-05-07, 91.2% coverage) as a pure HTTP client to a
  yt-dlp sidecar but DEFERRED the sidecar implementation (ADP-005
  §2.1(p), §6.4 contract + Open Question §11.7 deferral). The sidecar
  directory `services/youtube-extract/` does not exist on disk, so
  YouTube search cannot run end-to-end. This SPEC builds and deploys
  the sidecar and resolves the port collision discovered post-ADP-005.

  Findings (all `file:line`-verified, see research.md):
  - The adapter's `defaultBaseURL` is `http://localhost:8082`
    (`youtube.go:20`), colliding with the embedder sidecar (also 8082;
    `embedder/__main__.py:14`, `docker-compose.yml:204`). Commit
    `742564d` (2026-06-04) gated YouTube registration behind
    `YOUTUBE_BASE_URL` (`query.go:488`) as a stop-gap, so YouTube
    silently no-ops when the env is unset.
  - The exact `/search` request + response JSON schema and the
    `/health` schema were extracted from the implemented adapter source
    (`search.go:37-44`, `parse.go:22-63`, `youtube.go:114-146`) — not
    re-derived from prose — so the Python contract is byte-accurate.
    The full schema is in §6.4 and §6.5.

  User-locked decisions (Decision Points D1-D4 below) baked in:

  - **D1 Port reassignment**: the YouTube sidecar binds port **8084**
    (8082=embedder, 8083=tokenizer-ko, 8081=researcher; 8084 is the
    next free port in the docker-compose stack — `storm` uses 8084 only
    in the unused Helm chart, not in `deploy/docker-compose.yml`). This
    requires changing the Go adapter's `defaultBaseURL` constant from
    `:8082` → `:8084` (`youtube.go:20`) AND adding `.env.example` /
    docker-compose env so embedder and youtube run simultaneously. The
    `YOUTUBE_BASE_URL` gating from commit `742564d` is RETAINED as the
    production override + integration-test redirect hook.
  - **D2 Sidecar tech** (per ADP-005 D2): Python **FastAPI + uvicorn +
    yt-dlp** (exact version pin), multi-stage Dockerfile, non-root user,
    runs yt-dlp as a **subprocess** (GPL process-isolation — no linking)
    with rate-limit flags per ADP-005 D4
    (`--sleep-requests 1.0 --sleep-interval 2 --max-sleep-interval 5`,
    env-tunable).
  - **D3 Auth** (per ADP-005 D3): `RequiresAuth=false`; optional cookie
    injection via `YT_COOKIES_PATH` env (operational IP-block
    mitigation, not required for the public path).
  - **D4 Endpoints**: `GET /health` (→ `{"status":"ok","ytdlp_version":
    ...}`) and `POST /search`. The contract MUST match the Go adapter
    exactly (extracted from `search.go`/`parse.go`/`youtube.go`).

  Methodology: **tdd** (the Python sidecar is greenfield, test-first per
  RED-GREEN-REFACTOR). The Go-side change is a single-constant + doc/Notes
  sync (minimal; owner expert-backend). Owner: **expert-devops** (sidecar
  build/deploy) with **expert-backend** for the Go constant change.
  Harness level: **standard** (single new service directory + 1 Go
  constant edit + docker-compose + .env). Sprint Contract OPTIONAL.

---

## 1. Purpose

SPEC-ADP-005 (implemented) delivered a complete Go YouTube adapter that
is an HTTP client to a yt-dlp Python sidecar. The adapter's `Search`
POSTs to `<baseURL>/search` and `Healthcheck` GETs `<baseURL>/health`,
but **no such service exists**: `services/youtube-extract/` was reserved
and contractually documented (ADP-005 §2.1(p), §6.4) yet deferred to a
never-resolved Open Question (§11.7). Consequently YouTube search cannot
execute end-to-end — the only wired search path is the CLI (per Local
run state), and YouTube is registration-gated to a no-op
(`query.go:488`).

This SPEC closes that gap. It implements the deployment ADP-005
specified: a dedicated FastAPI sidecar at `services/youtube-extract/`
that satisfies the adapter's wire contract byte-for-byte, wires it into
docker-compose on a non-colliding port (8084), and fixes the adapter's
default base URL to match.

This SPEC reuses ADP-005's Decision Points D1-D8 as upstream constraints
(it does NOT restate them in full; see `.moai/specs/SPEC-ADP-005/spec.md`
§1 HISTORY): D2 (sidecar architecture), D3 (auth posture), D4 (rate-limit
defence), D6 (Korean transcript handling), D7 (pagination cap),
D8 (mock-only tests). The wire contract here is the implemented form of
ADP-005 §6.3/§6.4, extracted from source as ground truth.

This SPEC does NOT change the Go adapter's behaviour (only its default
URL constant + doc strings). It does NOT add fanout, caching, retry,
ranking, or circuit-breaking — those remain owned by their respective
SPECs per ADP-005 §2.2.

---

## 2. Scope

### 2.1 In-Scope

| Ref | Item |
|-----|------|
| a | NEW directory `services/youtube-extract/` mirroring `services/tokenizer-ko/` (multi-stage Dockerfile) and `services/embedder/` (FastAPI lifespan + /health 200/503, README.md, .env.example — only embedder has these two files; tokenizer-ko has neither). |
| b | `services/youtube-extract/pyproject.toml`: `fastapi>=0.115`, `uvicorn[standard]>=0.30`, `pydantic>=2.9`, `httpx>=0.27`, `yt-dlp==<pinned>`; dev group `ruff`/`pytest`/`pytest-asyncio`/`pytest-cov`; `license="Apache-2.0"`, `requires-python=">=3.11"`, hatchling, `packages=["src/youtube_extract"]`. |
| c | `services/youtube-extract/Dockerfile`: multi-stage `python:3.11-slim` (builder `--prefix=/install` → runtime copy), non-root `appuser`, `EXPOSE 8084`, `HEALTHCHECK` polling `/health`, `CMD ["python","-m","youtube_extract"]`. |
| d | `services/youtube-extract/src/youtube_extract/`: `__init__.py`, `__main__.py` (uvicorn on `YT_EXTRACT_PORT` default 8084, `workers=1`), `app.py` (FastAPI + lifespan + `GET /health` + `POST /search`), `ytdlp_runner.py` (yt-dlp subprocess wrapper + caption fetch), `models.py` (pydantic request/response matching §6.4/§6.5). |
| e | `services/youtube-extract/tests/`: `conftest.py`, `test_app.py` (FastAPI TestClient over /health + /search), `test_ytdlp_runner.py` (mocked subprocess + caption fetch); golden fixtures under `tests/fixtures/`. NO live network. |
| f | `services/youtube-extract/README.md` (operator notes) + `services/youtube-extract/.env.example` (`YT_EXTRACT_PORT`, `YT_COOKIES_PATH`, `YT_USER_AGENT`, `YT_SLEEP_REQUESTS`, `YT_SLEEP_INTERVAL`, `YT_MAX_SLEEP_INTERVAL`). |
| g | `deploy/docker-compose.yml`: NEW `youtube-extract` service block (build context `../services/youtube-extract`, port `${YOUTUBE_PORT:-8084}:8084`, env interpolation, healthcheck, `restart: unless-stopped`, `networks: [app]`), mirroring the tokenizer-ko block (`docker-compose.yml:312-335`). |
| h | Root `.env.example`: add `YOUTUBE_BASE_URL=http://localhost:8084`, `YOUTUBE_PORT=8084`, `YT_COOKIES_PATH=` (+ optional `YT_SLEEP_*`), mirroring the embedder block (`.env.example:91,95`). |
| i | Go adapter change (expert-backend): `internal/adapters/youtube/youtube.go` `defaultBaseURL` `:8082` → `:8084` (`youtube.go:20`); sync the package doc comment (`youtube.go:2`) and `capabilitiesNotes` "port 8082" → "port 8084" (`youtube.go:34`). No test edit needed — `youtube_test.go::TestCapabilitiesShape` pins 5 Notes substrings, none containing "8082". `YOUTUBE_BASE_URL` gating at `query.go:488` is UNCHANGED. |

### 2.2 Out-of-Scope

[HARD] This SPEC explicitly excludes the following. Each has a known
destination or is owned by another SPEC.

- **Any Go adapter behaviour change** beyond the default-URL constant +
  doc/Notes sync. The adapter logic (Search/parse/errors/score/lang) is
  ADP-005's implemented domain and is NOT touched.
- **YouTube Data API v3 / OAuth path** → rejected per ADP-005 D1.
- **`/transcript` full-fetch endpoint** → ADP-005 §11.3 deferred to
  SPEC-SYN-001.
- **ffmpeg in the image / audio-video download** → metadata + caption-URL
  fetch needs no ffmpeg (research §3.2); omitted in v0.1.
- **Live YouTube network calls in CI** → ADP-005 D8; sidecar tests mock
  the yt-dlp subprocess + caption fetch with golden fixtures.
- **Retry / caching / fanout / RRF / circuit-breaking** → SPEC-FAN-001,
  SPEC-CACHE-001, SPEC-IDX-001, SPEC-EVAL-002 (per ADP-005 §2.2).
- **Per-adapter custom Prometheus metrics** → the registry
  wrappedAdapter already emits `AdapterCalls{adapter="youtube"}`;
  `deploy/prometheus/alerts-test.yml`'s
  `usearch:adapter_fanout_partial_ratio_24h{adapter="youtube"}` works
  once the adapter is registered. No sidecar metric surface in v0.1.
- **Helm chart `youtube-extract` service** (`charts/`) → the storm
  Helm entry holds 8084 in the chart; production K8s wiring is a
  separate deploy SPEC (SPEC-DEPLOY-001, M9). v0.1 targets the
  docker-compose stack only.
- **Cookie provisioning automation / vault** → ADP-005 §11.2;
  `YT_COOKIES_PATH` env + optional bind mount only.

## 2.3 Exclusions (What NOT to Build)

[HARD] Beyond §2.2, this SPEC will NOT:

- Re-implement or refactor the Go adapter's request/response handling.
  The sidecar conforms to the EXISTING Go contract; the Go side does not
  bend to the sidecar.
- Introduce a new wire schema. The `/search` and `/health` JSON shapes
  are FROZEN by the implemented Go structs (`search.go:37-44`,
  `parse.go:22-63`, `youtube.go:136-138`). The sidecar MUST emit exactly
  these field names and types.
- Add authentication between the Go adapter and the sidecar (D3:
  `RequiresAuth=false`; the sidecar is operator-trusted on the `app`
  network).
- Build a sidecar-side circuit breaker, cache, or health-state machine.
- Emit transcripts longer than ~500 runes from `/search` (the adapter
  re-caps at 500; the sidecar SHOULD pre-cap to bound payload).

---

## 3. EARS Requirements

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| REQ-ADP5a-001 | Ubiquitous | The repository SHALL contain a new service directory `services/youtube-extract/` with the src-layout package `src/youtube_extract/` (`__init__.py`, `__main__.py`, `app.py`, `ytdlp_runner.py`, `models.py`), a multi-stage `Dockerfile`, a `pyproject.toml` (FastAPI + uvicorn + pydantic + httpx + pinned yt-dlp), a `README.md`, a `.env.example`, and a `tests/` directory — mirroring the `services/embedder/` shape for `README.md`/`.env.example` (only embedder has those two files) and the `services/tokenizer-ko/` shape for the multi-stage Dockerfile. The package SHALL be importable as `youtube_extract` and runnable via `python -m youtube_extract`. | P1 | Directory + files exist; `python -m youtube_extract` boots uvicorn on the configured port; `pyproject.toml` validates and `pip install .` succeeds. |
| REQ-ADP5a-002 | Event-Driven | WHEN an HTTP `GET /health` request is received, the sidecar SHALL respond HTTP 200 with a JSON body `{"status":"ok","ytdlp_version":"<version>"}` once yt-dlp is importable, and HTTP 503 with `{"status":"loading",...}` (or equivalent non-`ok` status) until ready. The `status` field MUST equal exactly `"ok"` on success (the Go adapter at `youtube.go:142` rejects any other value). | P1 | `GET /health` → 200 + `status=="ok"` + `ytdlp_version` present; pre-ready → 503; Go `Healthcheck` succeeds against the live sidecar. |
| REQ-ADP5a-003 | Event-Driven | WHEN an HTTP `POST /search` request is received with a JSON body containing `query` (string), `max_results` (int), `cursor_offset` (int), `transcript_lang` (string), `include_transcripts` (bool), and optional `since` (int64), the sidecar SHALL invoke yt-dlp `ytsearch{max_results+cursor_offset}:<query>` via subprocess, slice `[cursor_offset : cursor_offset+max_results]`, and respond HTTP 200 with `{"items":[<YTItem>...],"has_more":<bool>}` where each `<YTItem>` contains exactly the fields the Go `parse.go:38-63` struct unmarshals (see §6.4). `has_more` SHALL be `true` when more results exist beyond the returned slice. | P1 | `POST /search` returns the §6.4 envelope; field names/types match `parse.go` exactly; `has_more` set correctly; Go `Search` parses the live response into valid `NormalizedDoc`s. |
| REQ-ADP5a-004 | Ubiquitous | For each video item, the sidecar SHALL emit `view_count` as a JSON integer OR JSON `null` (for livestream-archived videos with no view count), `available_transcript_langs` as a JSON array that is ALWAYS present (possibly empty `[]`, never null/absent), `upload_date` as a `YYYY-MM-DD` string (reformatted from yt-dlp's native `YYYYMMDD`), and the REQUIRED Metadata-source fields `channel_id`, `channel_url`, `duration_seconds`, `thumbnail_url`. When a single video fails to extract while others succeed, the sidecar SHALL include that item with a non-null `"error"` string field so the Go adapter skips it silently (`parse.go:116-118`). | P1 | Golden-fixture tests assert `null` view_count for livestream; `available_transcript_langs` always an array; `upload_date` is `YYYY-MM-DD`; partial-failure item carries `error` and is skipped by Go. |
| REQ-ADP5a-005 | State-Driven | WHILE `include_transcripts` is `true` in the request, the sidecar SHALL attempt to fetch the caption track for `transcript_lang` (trying that lang first, falling back to `en`, then to any available), populate `transcript_snippet` with the first ≤500 runes of caption text, set `transcript_lang` to the actual lang served and `transcript_is_auto` accordingly, and ALWAYS populate `available_transcript_langs`. WHILE `include_transcripts` is `false`, the sidecar SHALL omit transcript fetch and emit an empty `transcript_snippet`. | P1 | Korean-query fixture yields `ko` snippet + `transcript_is_auto`; missing-`ko` falls back to `en`; `available_transcript_langs` populated regardless; `include_transcripts=false` skips fetch. |
| REQ-ADP5a-006 | Unwanted | IF yt-dlp encounters the YouTube "Sign in to confirm you're not a bot" challenge, THEN the sidecar SHALL respond HTTP **503** with body `{"error":{"category":"unavailable","reason":"yt-dlp signed-in challenge"}}`. IF yt-dlp reports rate-limiting, THEN the sidecar SHALL respond HTTP **429** with a `Retry-After` header (seconds). IF the request body is malformed (missing `query`, wrong types), THEN the sidecar SHALL respond HTTP **4xx** (e.g. 400) with `{"error":{"category":"permanent","message":...}}`. The Go adapter maps 503→`CategoryUnavailable`, 429→`CategoryRateLimited`(30s default), 4xx→`CategoryPermanent` (`search.go:200-229`). | P1 | Tests simulate each yt-dlp failure mode (mocked subprocess) and assert the exact HTTP status + envelope; Go-side categorisation verified against the live status codes. |
| REQ-ADP5a-007 | Optional | WHERE `YT_COOKIES_PATH` is set to a readable file path, the sidecar SHALL pass `--cookies <path>` to yt-dlp (optional IP-block mitigation per ADP-005 D3). WHERE the env is unset or the file is absent, the sidecar SHALL run the public no-auth path without error. WHERE `YT_SLEEP_REQUESTS`/`YT_SLEEP_INTERVAL`/`YT_MAX_SLEEP_INTERVAL` are set, the sidecar SHALL pass the corresponding `--sleep-requests`/`--sleep-interval`/`--max-sleep-interval` flags (defaults `1.0`/`2`/`5` per ADP-005 D4). | P1 | With cookies env set, `--cookies` appears in the constructed yt-dlp argv; unset → no `--cookies`, public path works; sleep flags honoured with documented defaults. |
| REQ-ADP5a-008 | Event-Driven | WHEN the YouTube sidecar service is started via `docker compose up`, it SHALL bind port **8084** (not 8082), pass its docker-compose healthcheck within `start_period`, and join the `app` network so the Go services reach it at `http://youtube-extract:8084`. The Go adapter's compiled `defaultBaseURL` SHALL be `http://localhost:8084` and `Capabilities().Notes` SHALL reference port 8084. The embedder (8082) and youtube-extract (8084) SHALL run simultaneously without port conflict. | P1 | `docker compose up` brings both embedder and youtube-extract healthy; `youtube.go:20` is `:8084`; Notes references 8084; `youtube_test.go` passes UNCHANGED (its 5 pinned Notes substrings exclude "8082"). |

---

## 4. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| NFR-ADP5a-001 | yt-dlp version pin + upgrade policy | `pyproject.toml` SHALL pin yt-dlp to an EXACT version (`yt-dlp==<x.y.z>`, never `>=`). `README.md` SHALL document the upgrade procedure (bump in a branch, run sidecar pytest + a manual smoke against a stable video id `dQw4w9WgXcQ`, then update the pin). Rationale: yt-dlp's YouTube extractor breaks roughly quarterly (ADP-005 risk row). |
| NFR-ADP5a-002 | GPL process-isolation | yt-dlp SHALL be invoked as a SUBPROCESS (`subprocess`/`asyncio.create_subprocess_exec`), never imported and linked into a distributable binary, preserving the Apache-2.0 boundary at the Go side (ADP-005 §1.1 license rationale). The sidecar Docker image installs yt-dlp via `pip` (its own process space). |
| NFR-ADP5a-003 | Subprocess lifecycle + resource bounds | The sidecar SHALL terminate the yt-dlp subprocess on request cancellation/timeout (SIGTERM, then SIGKILL after a grace period) so no zombie processes accumulate (ADP-005 NFR-ADP5-004 + risk row "Subprocess zombie"). Verified by a sidecar test that cancels mid-extraction and asserts the subprocess is reaped. uvicorn runs `workers=1` (embedder/tokenizer precedent). |
| NFR-ADP5a-004 | Healthcheck timing | The docker-compose healthcheck SHALL use `interval: 15s`, `timeout: 5s`, `retries: 5`, `start_period: 30s` (mirroring tokenizer-ko at `docker-compose.yml:329-332`), allowing yt-dlp import + first-readiness within the start period without flapping. |
| NFR-ADP5a-005 | Test coverage (no live network) | The sidecar pytest suite SHALL achieve ≥85% line coverage (`quality.test_coverage_target`) with the yt-dlp subprocess and httpx caption fetch fully mocked; ZERO live YouTube calls in CI (ADP-005 D8). |

---

## 5. Acceptance Criteria

Detailed Given/When/Then scenarios, edge cases, and the Definition of
Done are in `acceptance.md`. Summary by REQ:

- **REQ-ADP5a-001**: `services/youtube-extract/` exists with all files;
  `python -m youtube_extract` boots; `pip install .` succeeds.
- **REQ-ADP5a-002**: `/health` returns 200 `{"status":"ok",...}` when
  ready, 503 otherwise; Go `Healthcheck` passes against the live sidecar.
- **REQ-ADP5a-003**: `/search` returns the §6.4 envelope byte-compatible
  with `parse.go`; Go `Search` parses live responses into valid
  `NormalizedDoc`s.
- **REQ-ADP5a-004**: livestream → `null` view_count; transcript langs
  always an array; `upload_date` `YYYY-MM-DD`; partial-failure item
  carries `error`.
- **REQ-ADP5a-005**: transcript fetch honours lang fallback + 500-rune
  cap; `include_transcripts=false` skips fetch.
- **REQ-ADP5a-006**: 503 challenge / 429 rate-limit / 4xx malformed map
  to the exact statuses + envelopes the Go adapter categorises.
- **REQ-ADP5a-007**: cookies + sleep flags honoured; public path works
  without cookies.
- **REQ-ADP5a-008**: compose brings embedder (8082) + youtube-extract
  (8084) healthy together; Go default is `:8084`; Notes references 8084.

---

## 6. Technical Approach

### 6.1 Files to Modify / Create

**Created (Python sidecar, ~10 files + fixtures)**:

- `services/youtube-extract/Dockerfile`
- `services/youtube-extract/pyproject.toml`
- `services/youtube-extract/README.md`
- `services/youtube-extract/.env.example`
- `services/youtube-extract/src/youtube_extract/__init__.py`
- `services/youtube-extract/src/youtube_extract/__main__.py`
- `services/youtube-extract/src/youtube_extract/app.py`
- `services/youtube-extract/src/youtube_extract/ytdlp_runner.py`
- `services/youtube-extract/src/youtube_extract/models.py`
- `services/youtube-extract/tests/conftest.py`
- `services/youtube-extract/tests/test_app.py`
- `services/youtube-extract/tests/test_ytdlp_runner.py`
- `services/youtube-extract/tests/fixtures/*.json` — REUSE the 6
  existing Go testdata fixtures (`search_response.json` happy 25,
  `search_response_empty.json`, `search_response_pagination.json`,
  `search_response_korean.json`, `search_response_no_transcript.json`,
  and `search_response_malformed.json`) AND AUTHOR 3 fresh golden
  fixtures the Go side does NOT have: a 429 rate-limit response, a 503
  signed-in-challenge envelope, and a partial-success `/search` response
  carrying a per-item `"error"` field. These 3 cannot be copied — no Go
  testdata file exists for them (the Go 429/503 tests synthesise the
  HTTP status at the httptest.Server rather than via a fixture file).
  NOTE two DISTINCT error paths: `search_response_malformed.json` is a
  TRUNCATED-JSON envelope exercising the top-level parse-failure path
  (`parse.go:86`, → CategoryPermanent for the whole response); the
  per-item `"error"` field exercises the silent per-item SKIP path
  (`parse.go:116`, errored item dropped, others returned). They are not
  interchangeable and need separate fixtures.

**Modified**:

- `deploy/docker-compose.yml` — add `youtube-extract` service block.
- `.env.example` — add `YOUTUBE_BASE_URL`, `YOUTUBE_PORT`,
  `YT_COOKIES_PATH` (+ optional `YT_SLEEP_*`).
- `internal/adapters/youtube/youtube.go` — `defaultBaseURL` `:8082` →
  `:8084` (`:20`); package doc (`:2`) + `capabilitiesNotes` (`:34`).
  (`youtube_test.go` needs NO edit — its 5 pinned Notes substrings do
  not include the "8082" literal.)

**Unchanged (by design)**:

- `internal/adapters/youtube/{search,parse,errors,client,score,lang}.go`
  — NO logic change; the sidecar conforms to the existing contract.
- `cmd/usearch/query.go:488` — `YOUTUBE_BASE_URL` gate retained.
- `pkg/types/*`, `internal/adapters/registry.go` — unchanged.

### 6.2 Sidecar Module Layout

```
services/youtube-extract/
├── Dockerfile                  # multi-stage python:3.11-slim, non-root, EXPOSE 8084
├── pyproject.toml              # fastapi + uvicorn + pydantic + httpx + yt-dlp==<pin>
├── README.md                   # operator notes + yt-dlp upgrade policy
├── .env.example                # YT_EXTRACT_PORT, YT_COOKIES_PATH, YT_SLEEP_*
├── src/youtube_extract/
│   ├── __init__.py             # __version__
│   ├── __main__.py             # uvicorn.run(app, port=YT_EXTRACT_PORT default 8084, workers=1)
│   ├── app.py                  # FastAPI, lifespan (probe yt-dlp), GET /health, POST /search
│   ├── ytdlp_runner.py         # subprocess wrapper: ytsearch + dump-json + caption fetch
│   └── models.py               # pydantic SearchRequest / SearchResponse / YTItem / ErrorEnvelope
└── tests/
    ├── conftest.py
    ├── test_app.py             # TestClient over /health + /search (mocked runner)
    ├── test_ytdlp_runner.py    # mocked subprocess + httpx caption fetch
    └── fixtures/*.json
```

### 6.3 Sidecar → Adapter alignment

The sidecar's pydantic models in `models.py` MUST serialise to the exact
JSON the Go structs unmarshal. The Go structs are the FROZEN contract
(see §6.4/§6.5). The pydantic field names use the JSON tag names from the
Go structs verbatim (`max_results`, `cursor_offset`, `transcript_lang`,
`include_transcripts`, `available_transcript_langs`, `transcript_snippet`,
`transcript_is_auto`, `duration_seconds`, `view_count`, `channel_id`,
`channel_url`, `thumbnail_url`, `upload_date`, `has_more`).

### 6.4 Wire Contract — `POST /search` (extracted from Go source)

**Request body** (Go `search.go:37-44`; all fields the adapter sends):

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

Adapter-guaranteed invariants (the sidecar MAY rely on these — enforced
before the HTTP call in `search.go`):
- `query` is never empty/whitespace.
- `max_results` ∈ [1, 100] (default 25).
- `cursor_offset` ≥ 0, always present (`0` = first page).
- `max_results + cursor_offset` ≤ 100.
- `transcript_lang` always present, non-empty (floor `"en"`).
- `include_transcripts` is `true` in v0.1.
- `since` present only when a positive Unix-seconds `since` filter was
  supplied (`omitempty`).
- Headers: `Content-Type: application/json`, `Accept: application/json`,
  `User-Agent: usearch/<ver> (+https://github.com/elymas/universal-search)`.

**Success response** HTTP 200 (Go `parse.go:22-63` — authoritative
field set the adapter unmarshals):

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
      "transcript_snippet": "We're no strangers to love…",
      "transcript_lang": "en",
      "transcript_is_auto": false,
      "error": null
    }
  ],
  "has_more": true
}
```

Field-by-field contract (from `parse.go` struct tags + transform):

| JSON field | Go type | Notes (from parse.go) |
|---|---|---|
| `id` | string | `NormalizedDoc.ID` |
| `url` | string | `NormalizedDoc.URL` |
| `title` | string | `Title` |
| `description` | string | `Body`; `""` allowed → Snippet cascades to transcript/title |
| `channel` | string | `Author`; if empty, adapter uses `uploader` (`parse.go:160-163`) |
| `channel_id` | string | REQUIRED Metadata key (`parse.go:186`) |
| `channel_url` | string | REQUIRED Metadata key (`parse.go:187`) |
| `uploader` | string | Author fallback |
| `uploader_id` | string | OPTIONAL Metadata key (`parse.go:201`) |
| `duration_seconds` | int64 | REQUIRED key; `0` for livestream-archived |
| `view_count` | int64 OR **null** | `*int64` (`parse.go:51`); null/missing → 0 → `Score=0.5` |
| `like_count` | int64 OR null/absent | OPTIONAL key (`parse.go:195-197`) |
| `upload_date` | string | `"2006-01-02"` layout (`parse.go:153`); reformat from `YYYYMMDD` |
| `thumbnail_url` | string | REQUIRED Metadata key |
| `tags` | []string | OPTIONAL key (emit when non-empty) |
| `available_transcript_langs` | []string | REQUIRED key; ALWAYS present, may be `[]`, never null |
| `transcript_snippet` | string | adapter re-caps at 500 runes; sidecar SHOULD pre-cap |
| `transcript_lang` | string | actual lang served |
| `transcript_is_auto` | bool | auto-caption flag |
| `error` | string OR null | non-null → adapter SKIPS item silently (`parse.go:116-118`) |
| `has_more` (envelope) | bool | true → adapter emits `next_cursor` when offset+len < 100 |

### 6.5 Wire Contract — `GET /health` + error envelope

**`GET /health`** (Go `youtube.go:127-144` requires 200 + `status=="ok"`):

```json
{ "status": "ok", "ytdlp_version": "2026.04.01" }
```

503 while not ready: `{"status":"loading","reason":"yt-dlp not importable"}`.

**Error envelope** (non-200; Go `parse.go:29-35`, `search.go:200-229`):

```json
{ "error": { "category": "unavailable", "message": "yt-dlp signed-in challenge" } }
```

- `category` ∈ {`unavailable`, `permanent`, `transient`, `rate_limited`}.
- The 503 challenge body may use `"reason"` instead of `"message"`
  (adapter accepts both, `parse.go:33-34`).
- HTTP **status** is primary for categorisation: 429→rate-limited
  (+`Retry-After`), 5xx→unavailable, 4xx→permanent.

### 6.6 docker-compose service block (D1, port 8084)

Mirrors tokenizer-ko (`docker-compose.yml:312-335`):

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

Operators set `YOUTUBE_BASE_URL=http://youtube-extract:8084` (compose) so
`query.go:488` registers the adapter against the live sidecar.

### 6.7 Go-side change (D1, expert-backend)

`youtube.go:20`: `defaultBaseURL = "http://localhost:8084"` (was `:8082`).
`youtube.go:2`: package doc "port 8082 by default" → "port 8084 by default".
`youtube.go:34`: `capabilitiesNotes` "port 8082 (default)" → "port 8084
(default)". `youtube_test.go::TestCapabilitiesShape` pins 5 Notes
substrings (`"yt-dlp Python sidecar"`, `"public no-auth"`, `"transcript
snippet truncated"`, `"Korean-locale auto-detection"`, `"max_results +
cursor offset cap 100"`) — NONE contains the "8082" literal, so the
constant change requires NO test edit. The `capabilitiesNotes` rewording
only touches the un-asserted "port 8082" prose. No `search.go`/`parse.go`/
`client.go` changes — the contract is unchanged, only the default URL.

### 6.8 Harness Level

Standard: 1 new service directory + 1 Go constant edit + docker-compose
+ .env, no security/PII keywords (cookies are an optional operator
secret outside the Go contract), no new metric families. tdd methodology
for the greenfield Python sidecar. Evaluator profile `default`.

---

## 7. What NOT to Build (Exclusions)

See §2.2 and §2.3. Summary: no Go adapter behaviour change (only the
default-URL constant + doc/Notes), no new wire schema, no auth between
adapter and sidecar, no sidecar circuit-breaker/cache/health-state, no
ffmpeg/download, no live-network CI tests, no Helm/K8s wiring (M9), no
`/transcript` endpoint binding (SPEC-SYN-001).

---

## 8. TDD Plan

RED-GREEN-REFACTOR per `quality.development_mode: tdd`. The sidecar is
greenfield; the Go change is a single constant. Representative tests:

| # | Test | File | REQ | Assertion |
|---|------|------|-----|-----------|
| 1 | `test_health_returns_ok_when_ready` | test_app.py | REQ-ADP5a-002 | 200 + `status=="ok"` + `ytdlp_version` |
| 2 | `test_health_returns_503_while_loading` | test_app.py | REQ-ADP5a-002 | 503 + non-`ok` status |
| 3 | `test_search_happy_path_returns_items_envelope` | test_app.py | REQ-ADP5a-003 | `{"items":[...],"has_more":bool}`; field names match parse.go |
| 4 | `test_search_request_validates_required_fields` | test_app.py | REQ-ADP5a-003/006 | missing `query`/wrong type → 4xx + permanent envelope |
| 5 | `test_search_slices_by_cursor_offset` | test_ytdlp_runner.py | REQ-ADP5a-003 | `ytsearch{N+offset}:` then `[offset:offset+N]` slice |
| 6 | `test_has_more_true_when_more_results` | test_ytdlp_runner.py | REQ-ADP5a-003 | `has_more` set per remaining results |
| 7 | `test_view_count_null_for_livestream` | test_ytdlp_runner.py | REQ-ADP5a-004 | livestream → `view_count: null` |
| 8 | `test_available_transcript_langs_always_array` | test_ytdlp_runner.py | REQ-ADP5a-004 | no transcripts → `[]`, never null/absent |
| 9 | `test_upload_date_reformatted_yyyy_mm_dd` | test_ytdlp_runner.py | REQ-ADP5a-004 | `YYYYMMDD` → `YYYY-MM-DD` |
| 10 | `test_partial_failure_item_carries_error` | test_ytdlp_runner.py | REQ-ADP5a-004 | one bad video → item with non-null `error` |
| 11 | `test_transcript_snippet_capped_500_runes` | test_ytdlp_runner.py | REQ-ADP5a-005 | snippet ≤ 500 runes |
| 12 | `test_transcript_lang_fallback_ko_to_en` | test_ytdlp_runner.py | REQ-ADP5a-005 | requested `ko` missing → `en` served; lang reflects actual |
| 13 | `test_include_transcripts_false_skips_fetch` | test_ytdlp_runner.py | REQ-ADP5a-005 | `include_transcripts=false` → empty snippet, no httpx call |
| 14 | `test_signed_in_challenge_returns_503_envelope` | test_app.py | REQ-ADP5a-006 | 503 + `{"error":{"category":"unavailable","reason":"yt-dlp signed-in challenge"}}` |
| 15 | `test_rate_limit_returns_429_retry_after` | test_app.py | REQ-ADP5a-006 | 429 + `Retry-After` header |
| 16 | `test_cookies_path_adds_cookies_flag` | test_ytdlp_runner.py | REQ-ADP5a-007 | `YT_COOKIES_PATH` set → `--cookies <path>` in argv |
| 17 | `test_no_cookies_runs_public_path` | test_ytdlp_runner.py | REQ-ADP5a-007 | unset → no `--cookies`; no error |
| 18 | `test_sleep_flags_default_and_override` | test_ytdlp_runner.py | REQ-ADP5a-007 | defaults `1.0`/`2`/`5`; env override honoured |
| 19 | `test_subprocess_killed_on_cancel` | test_ytdlp_runner.py | NFR-ADP5a-003 | cancel mid-extract → subprocess reaped (no zombie) |
| 20 | `test_ytdlp_invoked_as_subprocess` | test_ytdlp_runner.py | NFR-ADP5a-002 | runner uses subprocess exec, not in-process import |
| 21 | (Go) existing `TestCapabilitiesShape` still green | youtube_test.go | REQ-ADP5a-008 | After `:8082`→`:8084` constant + Notes prose change, the 5 pinned Notes substrings still match; no test edit |

yt-dlp subprocess and httpx caption fetch are MOCKED in all tests (golden
fixtures under `tests/fixtures/`). No live YouTube (ADP-005 D8,
NFR-ADP5a-005). Coverage target 85%.

---

## 9. Dependencies

### 9.1 Upstream

- **SPEC-ADP-005 (implemented)**: parent. Provides the Go adapter +
  wire contract this SPEC satisfies. HARD dep.

### 9.2 Reference Patterns

- `services/embedder/` (SPEC-IDX-002): FastAPI lifespan + /health 200/503.
- `services/tokenizer-ko/` (SPEC-IDX-003): multi-stage Dockerfile +
  docker-compose block.

### 9.3 External (run-phase pins)

- `fastapi>=0.115`, `uvicorn[standard]>=0.30`, `pydantic>=2.9`,
  `httpx>=0.27`, `yt-dlp==<pinned>` (exact). All in the sidecar's own
  Python dependency space (subprocess isolation; NFR-ADP5a-002).

### 9.4 Downstream

- None strictly blocked. Once deployed, YouTube becomes one of the M3
  fanout adapters (ADP-005 §1) — but the M3 "≥5 adapters fused" exit
  criterion does not single out YouTube.

---

## 10. Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Sidecar JSON drifts from Go struct field names | Medium | High | §6.4 freezes field names from `parse.go`; golden fixtures mirror Go `testdata/`; an integration smoke (compose up + Go `Healthcheck` + one `Search`) validates the contract end-to-end. |
| yt-dlp extractor breaks (quarterly) | Medium | Medium | Exact version pin (NFR-ADP5a-001) + README upgrade procedure + manual smoke against stable video id. |
| yt-dlp subprocess zombie on cancel | Medium | Medium | SIGTERM→SIGKILL grace (NFR-ADP5a-003); reaped-process test. |
| Port 8084 later claimed by storm in compose | Low | Low | storm has no compose block today; if added, pick the next free port; `YOUTUBE_PORT` env makes the mapping configurable. |
| Go default-URL change breaks a Notes-pinned test | Very Low | Low | `TestCapabilitiesShape` pins 5 Notes substrings, none containing "8082"; the constant + prose change leaves all 5 intact. No test edit needed. |
| Korean transcript absent despite `ko` request | Medium | Low | Fallback to `en`; `available_transcript_langs` always populated (REQ-ADP5a-005). |
| Image build cost (yt-dlp + httpx) | Low | Low | Multi-stage Dockerfile; no ffmpeg; ~slim image. |
| Cookie file path differs macOS dev vs Linux | Low | Low | `.env.example` documents `YT_COOKIES_PATH` alternatives; unset path is valid (public path). |

---

## 11. Open Questions

1. **ffmpeg in image?** Recommended: omit (metadata + caption fetch
   needs none). Owner: expert-devops.
2. **yt-dlp pin value** — exact tag at implementation time. Recommended:
   latest stable at build, recorded in README. Owner: expert-devops.
3. **uvicorn workers** — `1` (precedent) vs `>1`. Recommended: `1` in
   v0.1. Owner: expert-devops.
4. **Helm chart youtube-extract** (storm holds 8084 in `charts/`) —
   deferred to SPEC-DEPLOY-001 (M9). Owner: SPEC-DEPLOY-001 author.

---

## 12. References

### Internal (file:line cited)

- `internal/adapters/youtube/youtube.go:2,20,27,34,114-146` — default URL,
  UA, Notes, Healthcheck contract.
- `internal/adapters/youtube/search.go:37-44,54-180,200-229` — request
  body, Search, non-OK handling.
- `internal/adapters/youtube/parse.go:22-63,116-118,127,153,181-184,244-256`
  — response structs, item-skip, cursor, date, category map.
- `internal/adapters/youtube/errors.go:44` — parseRetryAfter.
- `internal/adapters/youtube/client.go:20-25,40-41` — timeout, headers.
- `cmd/usearch/query.go:449,488-494` — YOUTUBE_BASE_URL gate.
- `services/embedder/{Dockerfile,pyproject.toml,.env.example}`,
  `services/embedder/src/embedder/{app,__main__}.py` — sidecar reference.
- `services/tokenizer-ko/Dockerfile` — multi-stage pattern.
- `deploy/docker-compose.yml:177,198-226,312-335` — port occupancy +
  service block templates.
- `.env.example:91,95,112,118` — sidecar env precedent (no YouTube vars).
- `charts/universal-search/values.yaml:481` — 8084 used by storm in Helm
  only (not compose); confirms 8084 free in the compose stack.
- `deploy/prometheus/alerts-test.yml` — `adapter="youtube"` metric (works
  once the adapter is registered; no sidecar change needed).
- `.moai/specs/SPEC-ADP-005/spec.md` §1 (D1-D8), §2.1(p), §6.3, §6.4,
  §11.7 — parent decisions + sidecar contract + deferral.
- `.moai/specs/SPEC-ADP-005/research.md` §1 — yt-dlp behaviour (externally
  verified 2026-05-04).

---

*End of SPEC-ADP-005a v0.1 (DRAFT — pending plan-auditor cycle 1)*
