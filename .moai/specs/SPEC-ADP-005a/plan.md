# SPEC-ADP-005a Implementation Plan

**SPEC**: SPEC-ADP-005a — YouTube Extraction Sidecar (Build + Deploy)
**Methodology**: tdd (greenfield Python sidecar) + minimal Go edit
**Owner**: expert-devops (sidecar) + expert-backend (Go constant)
**Status**: draft

---

## 1. Technical Approach

Build the missing yt-dlp sidecar that the implemented ADP-005 Go adapter
expects, conforming to the EXISTING wire contract (extracted from Go
source — spec.md §6.4/§6.5, FROZEN by `search.go`/`parse.go`/`youtube.go`),
wire it into docker-compose on port 8084, and fix the adapter's default
base URL constant to match. The sidecar mirrors the established
`services/tokenizer-ko/` (multi-stage Dockerfile, compose block) and
`services/embedder/` (FastAPI lifespan, /health 200/503) shapes.

The sidecar is the conforming party: it bends to the Go contract, never
the reverse. The only Go change is a single non-functional constant +
doc/Notes sync.

---

## 2. Milestones (priority-ordered, no time estimates)

### Milestone M1 — Sidecar skeleton + contract models (Priority High)

- Scaffold `services/youtube-extract/` (pyproject.toml, src-layout,
  `__init__.py`, `__main__.py`).
- `models.py`: pydantic `SearchRequest`, `YTItem`, `SearchResponse`,
  `ErrorEnvelope`, `HealthResponse` with JSON field names matching the
  Go struct tags verbatim (spec.md §6.3/§6.4).
- `app.py`: FastAPI app + lifespan (probe yt-dlp importability) +
  `GET /health` (200/503).
- RED: `test_app.py::test_health_*`.
- Covers: REQ-ADP5a-001, REQ-ADP5a-002.

### Milestone M2 — yt-dlp runner + /search happy path (Priority High)

- `ytdlp_runner.py`: subprocess wrapper —
  `ytsearch{max+offset}:<query>` + `--dump-single-json`, slice
  `[offset:offset+max]`, map yt-dlp JSON → `YTItem` (reformat
  `upload_date`, `null` view_count for livestream, always-array
  transcript langs).
- `app.py`: `POST /search` happy path returning
  `{"items":[...],"has_more":bool}`.
- RED→GREEN: search happy path, cursor slice, has_more, field mapping
  (null view_count, transcript-langs array, upload_date reformat,
  partial-failure `error` item).
- Covers: REQ-ADP5a-003, REQ-ADP5a-004.

### Milestone M3 — Transcript fetch + failure mapping (Priority High)

- `ytdlp_runner.py`: caption fetch via httpx (lang fallback ko→en→any,
  500-rune cap, `transcript_is_auto`); `include_transcripts=false`
  short-circuit.
- `app.py`: failure mapping — 503 signed-in challenge envelope, 429 +
  `Retry-After`, 4xx malformed.
- Covers: REQ-ADP5a-005, REQ-ADP5a-006.

### Milestone M4 — Cookies/sleep flags + subprocess lifecycle (Priority Medium)

- `ytdlp_runner.py`: `--cookies` when `YT_COOKIES_PATH` set;
  `--sleep-requests/--sleep-interval/--max-sleep-interval` from env with
  defaults `1.0`/`2`/`5`; SIGTERM→SIGKILL grace on cancel.
- Covers: REQ-ADP5a-007, NFR-ADP5a-002, NFR-ADP5a-003.

### Milestone M5 — Containerisation + compose + env (Priority High)

- `Dockerfile` (multi-stage, non-root, EXPOSE 8084, HEALTHCHECK).
- `deploy/docker-compose.yml`: add `youtube-extract` block (spec.md §6.6).
- Root `.env.example`: `YOUTUBE_BASE_URL`, `YOUTUBE_PORT`,
  `YT_COOKIES_PATH` (+ optional `YT_SLEEP_*`).
- `services/youtube-extract/.env.example` + `README.md` (yt-dlp upgrade
  policy, NFR-ADP5a-001).
- Covers: REQ-ADP5a-008, NFR-ADP5a-001, NFR-ADP5a-004.

### Milestone M6 — Go default-URL fix (Priority High, expert-backend)

- `youtube.go:20` `defaultBaseURL` `:8082` → `:8084`.
- `youtube.go:2` package doc + `youtube.go:34` `capabilitiesNotes`
  port 8082 → 8084.
- No `youtube_test.go` edit — `TestCapabilitiesShape` pins 5 Notes
  substrings, none containing "8082"; the constant + prose change leaves
  them intact. Just confirm the suite stays green.
- `go test ./internal/adapters/youtube/...` + `go vet` + golangci-lint.
- Covers: REQ-ADP5a-008.

### Milestone M7 — End-to-end smoke (Priority Medium)

- `docker compose up embedder youtube-extract` → both healthy
  simultaneously (no 8082/8084 collision).
- Manual: set `YOUTUBE_BASE_URL=http://localhost:8084`, run a CLI query;
  confirm YouTube registers and `Healthcheck` passes.
- This smoke is NOT a CI gate (no live YouTube); it validates the
  contract wiring locally.

---

## 3. Dependency Graph (build order)

```
M1 (skeleton+models) ─→ M2 (runner+/search) ─→ M3 (transcript+failures)
                                                      │
                                              M4 (cookies/sleep/lifecycle)
                                                      │
M5 (Docker+compose+env) ───────────────────────────── ┘
M6 (Go const) — independent of M1-M5, can run in parallel
M7 (smoke) — requires M5 + M6
```

M6 (Go) is independent and small; can be done first or in parallel with
M1. M7 requires the container (M5) and the Go fix (M6).

---

## 4. File Ownership (parallel-safe)

| Agent | Files |
|-------|-------|
| expert-devops | `services/youtube-extract/**`, `deploy/docker-compose.yml`, root `.env.example` |
| expert-backend | `internal/adapters/youtube/youtube.go` (test files unchanged) |

No file overlap between the two owners → M6 parallelises with M1-M5.

---

## 5. Contract Fidelity Strategy

The sidecar MUST emit JSON byte-compatible with the Go structs. To
guarantee this:

1. REUSE the 6 existing Go testdata fixtures
   (`internal/adapters/youtube/testdata/*.json` — happy, empty,
   pagination, korean, no-transcript, malformed) by copying them into
   `services/youtube-extract/tests/fixtures/` as canonical expected
   `/search` responses; the sidecar's `models.py` serialisation MUST
   reproduce them. Then AUTHOR 3 fresh golden fixtures the Go side lacks
   — a 429 rate-limit response, a 503 signed-in-challenge envelope, and
   a partial-success response with a per-item `"error"` field — because
   no Go testdata file covers those paths. Keep the two error paths
   distinct: `search_response_malformed.json` (truncated JSON →
   top-level parse failure, `parse.go:86`) vs the new per-item-`error`
   fixture (silent item skip, `parse.go:116`).
2. The runner's yt-dlp→YTItem mapping is unit-tested against mocked
   `--dump-single-json` output → asserted equal to the fixture envelope.
3. M7 smoke runs the real Go `Healthcheck` + one `Search` against the
   live container to catch any residual drift.

---

## 6. Quality Gates

- Python: `ruff check`, `pytest --cov` ≥ 85% (NFR-ADP5a-005), all mocked
  (no live network).
- Go: `go vet`, `golangci-lint run`, `go test -race
  ./internal/adapters/youtube/...` (must stay green after the constant
  change).
- Docker: `docker compose config` validates; `docker compose up`
  healthchecks pass for embedder + youtube-extract together.

---

## 7. Risks (see spec.md §10)

Top risk: sidecar JSON drift from Go struct field names → mitigated by
fixture reuse + M7 smoke. yt-dlp breakage → exact pin + README upgrade
policy. Subprocess zombies → SIGTERM/SIGKILL grace + reaped-process test.

---

*End of SPEC-ADP-005a plan.md*
