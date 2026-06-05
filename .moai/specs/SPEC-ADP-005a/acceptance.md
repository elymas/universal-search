# SPEC-ADP-005a Acceptance Criteria

**SPEC**: SPEC-ADP-005a — YouTube Extraction Sidecar (Build + Deploy)
**Format**: Given/When/Then per REQ + edge cases + Definition of Done
**Methodology**: tdd (all sidecar tests mock yt-dlp + httpx; no live network)

---

## 1. Acceptance Scenarios by REQ

### REQ-ADP5a-001 — Sidecar Directory + Package

**AC-001: Directory + files exist**
- Given a clean checkout, When listing `services/youtube-extract/`, Then
  `Dockerfile`, `pyproject.toml`, `README.md`, `.env.example`,
  `src/youtube_extract/{__init__,__main__,app,ytdlp_runner,models}.py`,
  and `tests/` are present.

**AC-002: Package installs and boots**
- Given the sidecar dir, When `pip install .` runs, Then it succeeds.
- When `python -m youtube_extract` runs, Then uvicorn binds the
  configured port (`YT_EXTRACT_PORT` default 8084) with `workers=1`.

### REQ-ADP5a-002 — /health Contract

**AC-003: Healthy → 200 ok**
- Given yt-dlp is importable, When `GET /health`, Then HTTP 200 with
  `{"status":"ok","ytdlp_version":"<x.y.z>"}` and `status` equals
  exactly `"ok"`.

**AC-004: Not ready → 503**
- Given the service is still loading (yt-dlp not yet probed), When
  `GET /health`, Then HTTP 503 with a non-`ok` status.

**AC-005: Go Healthcheck passes live**
- Given the sidecar running on 8084, When the Go adapter's
  `Healthcheck(ctx)` runs against `http://localhost:8084`, Then it
  returns nil (validates `youtube.go:127-144` parsing).

### REQ-ADP5a-003 — /search Envelope

**AC-006: Happy path envelope**
- Given a valid request `{"query":"go generics","max_results":25,
  "cursor_offset":0,"transcript_lang":"en","include_transcripts":true}`,
  When `POST /search` (mocked yt-dlp returns 25 videos), Then HTTP 200
  with `{"items":[...25...],"has_more":<bool>}` and every item field name
  matches the Go `parse.go` struct tags (spec.md §6.4).

**AC-007: Cursor slice**
- Given `cursor_offset=25, max_results=25`, When `POST /search`, Then the
  runner invokes `ytsearch50:<query>` and returns items `[25:50]`.

**AC-008: has_more flag**
- Given more results exist beyond the returned slice, When `POST /search`,
  Then `has_more` is `true`; given the slice exhausts results, `has_more`
  is `false`.

**AC-009: Go Search parses live response**
- Given the sidecar running, When the Go adapter `Search` runs against it,
  Then it returns `[]NormalizedDoc` with every doc passing `Validate()`.

### REQ-ADP5a-004 — Field Emission Contract

**AC-010: livestream → null view_count**
- Given a livestream-archived video (no view count), When mapped to a
  `YTItem`, Then `view_count` is JSON `null` (Go maps null→0→`Score=0.5`).

**AC-011: available_transcript_langs always an array**
- Given a video with no transcripts, When mapped, Then
  `available_transcript_langs` is `[]` (empty array, never null/absent).

**AC-012: upload_date YYYY-MM-DD**
- Given yt-dlp's native `upload_date="20091025"`, When mapped, Then the
  emitted `upload_date` is `"2009-10-25"` (Go layout `"2006-01-02"`).

**AC-013: REQUIRED Metadata-source fields present**
- Given any successful item, Then `channel_id`, `channel_url`,
  `duration_seconds`, `thumbnail_url` are present.

**AC-014: partial-failure item carries error**
- Given a search where one video fails extraction while others succeed,
  When `POST /search`, Then the failed item appears with a non-null
  `"error"` string and the others are normal (Go skips the errored one
  silently, `parse.go:116-118`).

### REQ-ADP5a-005 — Transcript Fetch (State-Driven)

**AC-015: snippet capped at 500 runes**
- Given a long caption track, When `include_transcripts=true`, Then
  `transcript_snippet` is ≤ 500 runes.

**AC-016: lang fallback ko→en→any**
- Given `transcript_lang="ko"` requested and no `ko` track, When fetched,
  Then the sidecar serves `en` (or any available) and `transcript_lang`
  reflects the actual lang served; `available_transcript_langs` is
  populated.

**AC-017: include_transcripts=false skips fetch**
- Given `include_transcripts=false`, When `POST /search`, Then no httpx
  caption fetch occurs and `transcript_snippet` is empty.

### REQ-ADP5a-006 — Failure Mapping (Unwanted)

**AC-018: signed-in challenge → 503 envelope**
- Given yt-dlp hits the "Sign in to confirm you're not a bot" challenge,
  When `POST /search`, Then HTTP 503 with
  `{"error":{"category":"unavailable","reason":"yt-dlp signed-in
  challenge"}}` (Go → `CategoryUnavailable`, preserves reason).

**AC-019: rate-limit → 429 + Retry-After**
- Given yt-dlp reports rate-limiting, When `POST /search`, Then HTTP 429
  with a `Retry-After` header in seconds (Go → `CategoryRateLimited`,
  30s default if header absent).

**AC-020: malformed request → 4xx permanent**
- Given a body missing `query` or with wrong types, When `POST /search`,
  Then HTTP 4xx (e.g. 400) with `{"error":{"category":"permanent",
  "message":...}}` (Go → `CategoryPermanent`).

### REQ-ADP5a-007 — Cookies + Sleep Flags (Optional)

**AC-021: cookies flag added when env set**
- Given `YT_COOKIES_PATH=/run/secrets/yt_cookies.txt`, When the runner
  builds yt-dlp argv, Then `--cookies /run/secrets/yt_cookies.txt` is
  present.

**AC-022: public path when cookies unset**
- Given `YT_COOKIES_PATH` unset, When the runner builds argv, Then no
  `--cookies` flag; the public path executes without error.

**AC-023: sleep flags default + override**
- Given no overrides, Then argv has `--sleep-requests 1.0
  --sleep-interval 2 --max-sleep-interval 5`.
- Given `YT_SLEEP_REQUESTS=2.0`, Then argv has `--sleep-requests 2.0`.

### REQ-ADP5a-008 — Port 8084 + Deploy

**AC-024: compose brings both sidecars healthy**
- Given `docker compose up embedder youtube-extract`, When healthchecks
  run, Then both reach healthy within `start_period` — embedder on 8082,
  youtube-extract on 8084 — with no port conflict.

**AC-025: Go default URL is 8084**
- Given `youtube.go`, Then `defaultBaseURL == "http://localhost:8084"`
  and `Capabilities().Notes` references port 8084 (not 8082).

**AC-026: Go tests green after constant change (no test edit)**
- When `go test -race ./internal/adapters/youtube/...` runs, Then it
  exits 0 with NO change to `youtube_test.go` — `TestCapabilitiesShape`
  pins 5 Notes substrings, none containing "8082", so the constant +
  Notes-prose change leaves all assertions intact.

---

## 2. NFR Acceptance

**AC-N01 (NFR-ADP5a-001): yt-dlp exact pin + upgrade policy**
- `pyproject.toml` has `yt-dlp==<x.y.z>` (exact, not `>=`); `README.md`
  documents the bump procedure (branch → pytest + smoke against
  `dQw4w9WgXcQ` → update pin).

**AC-N02 (NFR-ADP5a-002): GPL process-isolation**
- The runner invokes yt-dlp via subprocess exec, NOT in-process import
  for distribution; asserted by a test inspecting the call mechanism.

**AC-N03 (NFR-ADP5a-003): subprocess reaped on cancel**
- Given a `/search` cancelled mid-extraction, Then the yt-dlp subprocess
  receives SIGTERM (then SIGKILL after grace) and no zombie remains.

**AC-N04 (NFR-ADP5a-004): healthcheck timing**
- The compose healthcheck uses `interval:15s timeout:5s retries:5
  start_period:30s`.

**AC-N05 (NFR-ADP5a-005): coverage ≥85%, no live network**
- `pytest --cov` reports ≥85% line coverage with yt-dlp + httpx mocked;
  CI makes zero live YouTube calls.

---

## 3. Edge Cases

**EC-001: empty results** — yt-dlp returns 0 videos → `{"items":[],
"has_more":false}` (Go `parseSearchResponse` returns `(nil,"",nil)`).

**EC-002: all items fail extraction** — every item carries `error` → Go
returns an empty slice with no error (sole-emitter: no logs).

**EC-003: view_count present but 0** — emit `0` (not null) → `Score=0.5`
either way.

**EC-004: transcript track exists but empty body** — `transcript_snippet`
empty; `available_transcript_langs` still lists the lang.

**EC-005: cursor at cap** — `max_results=25, cursor_offset=75` (sum 100,
the inclusive cap) → request reaches the sidecar; `ytsearch100:` then
`[75:100]`.

**EC-006: 8084 already bound on host** — `YOUTUBE_PORT` env remaps the
host-side port; container always listens on 8084 internally.

---

## 4. Coverage Matrix

| REQ-ID | EARS Pattern | Acceptance Criteria | Implementation |
|--------|-------------|---------------------|----------------|
| REQ-ADP5a-001 | Ubiquitous | AC-001..002 | `services/youtube-extract/` scaffold |
| REQ-ADP5a-002 | Event-Driven | AC-003..005 | `app.py::health`, lifespan |
| REQ-ADP5a-003 | Event-Driven | AC-006..009 | `app.py::search`, `ytdlp_runner.py` |
| REQ-ADP5a-004 | Ubiquitous | AC-010..014 | `ytdlp_runner.py` mapping, `models.py` |
| REQ-ADP5a-005 | State-Driven | AC-015..017 | `ytdlp_runner.py` caption fetch |
| REQ-ADP5a-006 | Unwanted | AC-018..020 | `app.py` failure mapping |
| REQ-ADP5a-007 | Optional | AC-021..023 | `ytdlp_runner.py` argv builder |
| REQ-ADP5a-008 | Event-Driven | AC-024..026 | `Dockerfile`, `docker-compose.yml`, `.env.example`, `youtube.go:20,34` |
| NFR-ADP5a-001 | Quality | AC-N01 | `pyproject.toml`, `README.md` |
| NFR-ADP5a-002 | Security | AC-N02 | `ytdlp_runner.py` subprocess |
| NFR-ADP5a-003 | Resource | AC-N03 | `ytdlp_runner.py` lifecycle |
| NFR-ADP5a-004 | Reliability | AC-N04 | `docker-compose.yml` healthcheck |
| NFR-ADP5a-005 | Tested | AC-N05 | `tests/**` mocked |

---

## 5. Definition of Done

- [ ] `services/youtube-extract/` created with all files (REQ-ADP5a-001).
- [ ] `python -m youtube_extract` boots; `pip install .` succeeds.
- [ ] `/health` returns 200 `{"status":"ok",...}` when ready, 503 else;
      Go `Healthcheck` passes against the live sidecar.
- [ ] `/search` returns the §6.4 envelope byte-compatible with `parse.go`
      (validated by reusing the Go `testdata/` fixtures).
- [ ] Field emission: `null` view_count for livestream;
      `available_transcript_langs` always an array; `upload_date`
      `YYYY-MM-DD`; partial-failure item carries `error`.
- [ ] Transcript: ≤500-rune snippet; ko→en fallback;
      `include_transcripts=false` skips fetch.
- [ ] Failure mapping: 503 challenge / 429 Retry-After / 4xx malformed.
- [ ] Cookies + sleep flags honoured; public path works without cookies.
- [ ] yt-dlp pinned exactly; README documents upgrade policy.
- [ ] yt-dlp runs as subprocess; reaped on cancel.
- [ ] `docker-compose.yml` youtube-extract block on port 8084 with
      healthcheck + `restart: unless-stopped` + `app` network.
- [ ] `.env.example` adds `YOUTUBE_BASE_URL`, `YOUTUBE_PORT`,
      `YT_COOKIES_PATH`.
- [ ] `youtube.go:20` `defaultBaseURL` = `http://localhost:8084`; Notes
      references 8084.
- [ ] `docker compose up` brings embedder (8082) + youtube-extract (8084)
      healthy simultaneously.
- [ ] `pytest --cov` ≥ 85% (mocked, no live network).
- [ ] `go test -race ./internal/adapters/youtube/...` exits 0;
      `go vet` + `golangci-lint` clean.
- [ ] `ruff check` clean on the sidecar.
- [ ] SPEC status updated to `implemented` after run.

---

*End of SPEC-ADP-005a acceptance.md*
