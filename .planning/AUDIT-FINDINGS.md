# Audit Findings — adapter / module gaps

Source: code audit (2026-06-04, gsd-audit-fix pivot — no `.planning/phases/*-UAT.md` present).
Verification of working adapters: `usearch query "<q>" --format json | jq '.adapters'`.

## Resolved (committed)

Build verified (`go build ./...` OK) as of 2026-06-05. Working tree clean (remaining untracked = build binaries only).

| # | Finding | SPEC | Commit |
|---|---|---|---|
| F-01 | GitHub adapter env var mismatch; aligned to `USEARCH_GITHUB_TOKEN` canonical (+`GITHUB_TOKEN` fallback). | — | c95514d |
| F-02 | YouTube registered unconditionally → default `localhost:8082` collided with embedder (404 noise). Gated behind `YOUTUBE_BASE_URL`. | — | 742564d |
| F-03 | Reddit App-Only OAuth (replaces blocked anon `search.json`); `oauth.reddit.com` allowlist + 401 refresh carve-out. | SPEC-ADP-001a | 9ffd66c |
| F-04 | YouTube extraction sidecar — full Python service `services/youtube-extract/` (yt-dlp runner, app, tests, Dockerfile) + `youtube.go` wiring. | SPEC-ADP-005a | 2d61d8f |
| F-05 | usearch-api HTTP server + pipeline (`cmd/usearch-api/api_handlers.go`). | SPEC-API-001 | 13a8bd0 |
| F-06 | `usearch sources` live health + registry-backed listing (concurrent Healthcheck fan-out, 4-state classify); `internal/pipeline/` extracted. | SPEC-CLI-003 | f178064 |
| F-07 | Adapter creds via configured secret backend (`naver_resolver_test.go`). | SPEC-SEC-002 | 2d61d8f |

VERIFIED 2026-06-05: `go test ./...` all packages PASS (0 FAIL); youtube-extract `pytest` 49 passed.
Minor (SPEC-ADP-005a) RESOLVED c7ef619: added `pythonpath = ["."]` to youtube-extract `[tool.pytest.ini_options]`; bare `uv run pytest` now resolves the `tests` package (49 passed, no PYTHONPATH needed).

## SPEC authored + plan-audited (impl pending)

| # | Finding | Sev | SPEC | SPEC title |
|---|---|---|---|---|
| F-08 | X (Twitter) adapter stub-only, disabled in v0 (`ErrXDisabled`); `NewX` not registered. | medium | SPEC-ADP-006-XENABLE | X (Twitter) Adapter — Live Provider Enablement |
| F-09 | Facebook + Threads have no adapter (0 code). | medium | SPEC-ADP-010 | Facebook + Threads (Meta) Adapter — Feasibility & Integration Contract |

plan-audit reviews present: `.moai/reports/plan-audit/{SPEC-ADP-006-XENABLE,SPEC-ADP-010,SPEC-CLI-003,SPEC-SEC-002}-review-1.md`.

## Working social coverage
Only **bluesky** is live (`social.NewBluesky`). X/facebook/threads do not work.
