# Audit Findings — adapter / module gaps

Source: code audit (2026-06-04, gsd-audit-fix pivot — no `.planning/phases/*-UAT.md` present).
Verification of working adapters: `usearch query "<q>" --format json | jq '.adapters'`.

## Resolved (auto-fixed)

| # | Finding | Commit |
|---|---|---|
| F-01 | GitHub adapter env var mismatch: wiring read `GITHUB_TOKEN` while adapter metadata/errors declared `USEARCH_GITHUB_TOKEN`. Aligned to `USEARCH_GITHUB_TOKEN` canonical (+`GITHUB_TOKEN` fallback). | c95514d |
| F-02 | YouTube adapter registered unconditionally; unset `YOUTUBE_BASE_URL` fell back to default `localhost:8082`, colliding with embedder sidecar → 404 noise. Gated registration behind `YOUTUBE_BASE_URL`. | 742564d |

## Implemented — working tree (uncommitted, pending verify + commit)

Implementation landed in a separate session (2026-06-05). Code present but NOT yet committed or independently verified here.

| # | Finding | SPEC | Evidence (working tree) |
|---|---|---|---|
| F-03 | Reddit App-Only OAuth (replaces blocked anon path) | SPEC-ADP-001a | new `internal/adapters/reddit/oauth.go`(+test); `client.go` adds `oauth.reddit.com` + 401 refresh carve-out; `search.go`/`errors.go`/`reddit.go` modified |
| F-04 | YouTube extraction sidecar (build + deploy) | SPEC-ADP-005a | new `services/youtube-extract/`; `youtube.go` modified |
| F-05 | usearch-api HTTP server | SPEC-API-001 | new `cmd/usearch-api/api_handlers.go`(+test); `main.go` modified |
| F-06 | `usearch sources` live health + registry-backed listing | SPEC-CLI-003 | `sources_cmd.go` rewritten (concurrent Healthcheck fan-out, 4-state classify); new `internal/pipeline/` (BuildProductionRegistry extracted) |
| F-07 | Adapter creds via configured secret backend | SPEC-SEC-002 | `naver.go` modified + new `naver_resolver_test.go` |

## SPEC authored + plan-audited (impl pending)

| # | Finding | Sev | SPEC | SPEC title |
|---|---|---|---|---|
| F-08 | X (Twitter) adapter stub-only, disabled in v0 (`ErrXDisabled`); `NewX` not registered. | medium | SPEC-ADP-006-XENABLE | X (Twitter) Adapter — Live Provider Enablement |
| F-09 | Facebook + Threads have no adapter (0 code). | medium | SPEC-ADP-010 | Facebook + Threads (Meta) Adapter — Feasibility & Integration Contract |

plan-audit reviews present: `.moai/reports/plan-audit/{SPEC-ADP-006-XENABLE,SPEC-ADP-010,SPEC-CLI-003,SPEC-SEC-002}-review-1.md`.

## Working social coverage
Only **bluesky** is live (`social.NewBluesky`). X/facebook/threads do not work.
