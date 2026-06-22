# Codebase Concerns

**Analysis Date:** 2026-06-04

This document catalogs technical debt, stub implementations, security considerations, fragile areas, and test-coverage gaps in `universal-search` (Go orchestration plane + Python sidecars + Next.js web UI) at v1.0.0.

**Overarching reality (from release context):** v1.0.0 ships a working CLI search path (`usearch query`) but the REST/SSE API server (`usearch-api`) and the deep-research pipeline (`usearch deep`) are deliberately unwired stubs. The Next.js web UI is fully built but has no live backend to talk to. Several "wiring" SPECs (SPEC-IR-001, SPEC-AUTH-001, SPEC-IDX-002, SPEC-DEEP-004 Phase E) are referenced as the homes for the missing implementations.

---

## Tech Debt

**usearch-api server is a non-serving stub:**

- Issue: `main()` builds the route mux (synthesis SSE endpoint + admin routes) but never calls `ListenAndServe`. It prints `usearch-api: not implemented (see SPEC-IR-001)` and `os.Exit(0)` immediately. The `mux` variable is explicitly discarded with `_ = mux`.
- Files: `cmd/usearch-api/main.go` (lines 40-52)
- Impact: The entire REST/SSE backend is non-functional. The container builds and starts but exits instantly. The Next.js web UI (which calls this API) has no server to reach.
- Fix approach: Implement SPEC-IR-001 — add an `http.Server` with the registered mux, graceful shutdown (SIGINT/SIGTERM), and real adapter registry population (line 45 `adapters.NewRegistry(nil)` passes nil).

**`usearch deep` pipeline is a print-only stub:**

- Issue: The deep-research command prints the four pipeline stages (Researcher → Reviewer → Writer → Verifier) as static text, then returns `ExitSystemError` with message "Deep research pipeline not yet wired (requires LLM client)." No actual `deepagent.RunPipeline` invocation occurs.
- Files: `cmd/usearch/deep_cmd.go` (lines 32-44)
- Impact: The flagship "deep research" feature is cosmetic. The `--budget` flag is accepted but unused.
- Fix approach: Wire `deepagent.RunPipeline` once an LLM client is available (gated behind LLM client construction).

**REPL pipeline execution not wired inline:**

- Issue: Per `USAGE.md` line 232, in-REPL pipeline execution is not connected ("pipeline execution not yet wired in REPL"). Slash commands and history persistence work; query execution does not.
- Files: `cmd/usearch/repl.go` (304 lines)
- Impact: Interactive REPL mode cannot run searches.
- Fix approach: Wire the REPL command dispatch to the same orchestrator path used by `usearch query`.

**Cost ledger Postgres writes are no-ops:**

- Issue: `WriteLedgerEntry` returns `nil` without persisting anything. `ReconcileScheduler` is constructed with `interface{}` placeholders for redis/postgres and only exposes its interval. `errNotImplemented` sentinel exists for stubbed SQL paths.
- Files: `internal/deepagent/costguard/ledger.go` (lines 36-68)
- Impact: LLM cost accounting (DEEP-004) does not actually record spend. The `@MX:ANCHOR` marks this as fan_in >= 3 (all LLM cost flows through it) — meaning every caller believes costs are being tracked when they are silently dropped. Budget enforcement is therefore unreliable.
- Fix approach: Implement with a real Postgres client in "Phase E wiring" (per the inline `@MX:TODO`).

**Dense embeddings are zero-vectors (semantic search is non-functional):**

- Issue: `zeroEmbedder` returns all-zero vectors of dimension 1024 for every input text. This is the v0.1 default `Embedder` implementation.
- Files: `internal/index/embedder.go` (lines 21-35)
- Impact: Any dense/semantic retrieval through the hybrid index produces meaningless similarity (all vectors identical → no ranking signal). Only lexical/keyword retrieval is meaningful until BGE-M3 is wired.
- Fix approach: SPEC-IDX-002 replaces `zeroEmbedder` with the production BGE-M3 implementation (Python `services/embedder/` sidecar already has tests). No interface change needed.

**CLI auth (`usearch login`) is a no-op skeleton:**

- Issue: `login status` always prints "Not authenticated."; `login logout` always prints "Already logged out." No credential file is read or written.
- Files: `cmd/usearch/login_cmd.go` (lines 40, 56)
- Impact: No real authentication for the CLI. Deferred to SPEC-AUTH-001.
- Fix approach: Implement credential-file existence check (status) and clearing (logout) when SPEC-AUTH-001 lands.

**Admin audit endpoint returns empty results:**

- Issue: The `/api/admin/audit/queries` route is wired with a `nil` AuditQuerier; the handler returns empty results until the audit store gains a `QueryEntries` method.
- Files: `cmd/usearch-api/main.go` (lines 81-83), `internal/api/admin/handler_audit.go`
- Impact: Admin audit query UI surface returns nothing even once the server is serving.
- Fix approach: Add `QueryEntries` to the audit store and pass a real querier.

**X (Twitter) adapter is stub-only / disabled:**

- Issue: The X sub-source makes no HTTP requests under any env state; `searchX` returns `ErrXDisabled` sentinel. Social adapter advertises "social sub-source stub; no live path wired" pending SPEC-ADP-006-XENABLE.
- Files: `internal/adapters/social/search_x.go` (lines 14-26), `internal/adapters/social/social.go` (lines 105, 176)
- Impact: X/Twitter results are never returned. Only Bluesky is live in the social adapter.
- Fix approach: Wire a live X provider under SPEC-ADP-006-XENABLE.

---

## Stale SPEC References (Planning Hazard)

**2026-05-22-era draft SPECs cite nonexistent paths/IDs/types:**

- Issue: A batch of M8+M9 draft SPECs were authored before (or in parallel with) the code they describe and drifted from reality. Documented failures include:
  - SEC-001: proposed building a Merkle hash chain that already exists (`internal/audit/chain.go` + `0003_audit_events.sql`).
  - EVAL-001: referenced `services/researcher/.../faithfulness.py` (real file is `faithfulness_endpoint.py`), a CJK split regex that does not exist, and a `SynthesizeResponse` type that does not exist.
  - EVAL-002: cited emit line `:223` (real `:433`) and duplicated an existing `/api/admin/adapters` endpoint.
  - EVAL-003: keyed a gate metric on 4 Naver adapter IDs that do not exist (real model is a single `SourceID="naver"` + vertical filter); treated ADP-009 as `daum-news`/`korea-news-crawler` (real: single `koreanews`).
- Files: `.moai/specs/SPEC-SEC-001/`, `.moai/specs/SPEC-EVAL-001/`, `.moai/specs/SPEC-EVAL-002/`, `.moai/specs/SPEC-EVAL-003/`
- Impact: Implementing these SPECs verbatim creates phantom migrations, duplicate endpoints, and references to nonexistent symbols.
- Fix approach: NEVER skip the plan-auditor gate for a 2026-05-22-era draft SPEC. Verify every cited file/path/type/ID against the live codebase via grep/Read before planning; amend stale refs first.

---

## Security Considerations

**Header-based identity spoofing when AUTH-001 is disabled:**

- Risk: `extractIdentity` falls back to `X-User-Id` / `X-Team-Id` / `X-Roles` request headers when the AUTH-001 JWT context is absent. A client can set arbitrary identity and team scope, gaining cross-team data access and elevated roles.
- Files: `internal/auth/rbac/middleware.go` (lines 66-110), specifically the header fallback at lines 87-101
- Current mitigation: An `@MX:WARN` documents the risk; the design intent is that production with AUTH-001 enabled ignores headers. The header path exists for dev/CI.
- Recommendations: Gate the header fallback behind an explicit `auth-001-ga=true` (or equivalent) production flag so it cannot be reached in production builds. Until AUTH-001 is GA, the API server (once serving) must not be exposed beyond loopback/trusted networks.

**Cross-team data exposure if tenancy filter is omitted:**

- Risk: An `@MX:WARN` on the index tenancy filter states "Filter expression omission causes cross-team data exposure." A missing/empty filter expression silently widens query scope to all teams.
- Files: `internal/index/tenancy/filter.go` (line 13)
- Current mitigation: Tagged as a load-bearing invariant.
- Recommendations: Add a defensive guard that fails closed (rejects the query) when no tenancy filter is present, rather than returning unfiltered results.

**`InsecureSkipVerify` in OIDC discovery:**

- Risk: `internal/auth/discovery.go` sets `tls.Config{InsecureSkipVerify: true}` in two places, disabling TLS certificate verification for the OIDC issuer.
- Files: `internal/auth/discovery.go` (lines 47, 52)
- Current mitigation: Both are annotated `//nolint:gosec // test-only` and guarded by `allowPrivateIssuer` (dev/CI). An `@MX:WARN` notes "only used in test builds."
- Recommendations: Confirm via a build-tag or runtime assertion that this path is unreachable in release binaries; a stray dev-config flag in production would expose OIDC discovery to MITM.

**Admin replay can re-execute arbitrary queries:**

- Risk: `internal/audit/replay.go` lets an admin re-execute arbitrary queries from `audit_events`. If the AUTH-002 permission check is bypassed or runs in the wrong order, an admin could replay all historical queries.
- Files: `internal/audit/replay.go` (lines 43-44)
- Current mitigation: `@MX:WARN`/`@MX:REASON` flag the RBAC-check-order dependency.
- Recommendations: Add a regression test that asserts the RBAC check runs before replay; verify order is preserved on any refactor.

**Irreversible audit partition drop:**

- Risk: `internal/audit/cleanup.go` runs `DROP PARTITION` which is irreversible.
- Files: `internal/audit/cleanup.go` (line 36)
- Current mitigation: `@MX:WARN` only.
- Recommendations: Add a retention-window guard and a dry-run mode; require explicit confirmation/config for destructive cleanup.

**SSRF / DNS-rebind dialers — do not regress:**

- Risk: SSRF protection depends on pinned DNS resolution to prevent rebind attacks (resolve to public IP first, then 127.0.0.1 on TCP connect). Removing the pinned resolution re-opens SSRF.
- Files: `internal/access/dialer.go` (line 27), `internal/security/ssrf/dialer.go` (line 16)
- Current mitigation: `@MX:WARN` "do NOT remove the pinned resolution." Adapter clients (`naver`, `searxng`, `social`, `reddit`, `hn`, `github`) document "do not bypass or replace the SSRF-guarded httpClient."
- Recommendations: Keep the SSRF dialer as the only outbound path; add a test that fails if an adapter constructs a raw `http.Client`.

---

## Performance Bottlenecks

**Retry loops cause cost amplification:**

- Problem: Multiple components run bounded retry loops issuing up to 3 HTTP calls each per invocation. In the deep-agent orchestrator, retry creates a documented "cost amplification surface."
- Files: `internal/deepagent/orchestrator.go` (line 96), `internal/synthesis/client.go` (lines 91, 246), `internal/deepreport/client.go` (line 85), `internal/llm/retry.go` (line 64)
- Cause: Each retry re-issues the full upstream call (and for LLM, re-incurs token cost). The synthesis client timeout applies across all retries, so a slow first attempt starves later ones.
- Improvement path: Add per-attempt budgets, exponential backoff with jitter (jitter already present in `internal/llm/retry.go:56`), and circuit breaking on repeated upstream failure.

**Goroutine fan-out hot paths:**

- Problem: Several paths spawn bounded goroutine pools (errgroup) per request. The rate limiter executes on every rate-limited request on the hot path.
- Files: `internal/fanout/dispatch.go` (line 82), `internal/index/dispatch.go` (lines 39, 149 — 3 goroutines per batch), `internal/adapters/koreanews/rss.go` (line 25), `internal/security/ratelimit/limiter.go` (line 55)
- Cause: Fan-out concurrency is bounded but the zero-leak guarantee (NFR-IDX-004) depends on exact per-goroutine `defer cancel()` ordering — fragile under refactor.
- Improvement path: Keep concurrency bounds configurable; preserve the `defer cancel()` sequence (covered by `@MX:WARN`); benchmark the rate-limiter hot path under load.

---

## Fragile Areas

**Per-phase panic recovery in the access cascade:**

- Files: `internal/access/cascade.go` (line 155, 338 lines total)
- Why fragile: Removing the `defer recover()` per phase invalidates the cascade's isolation guarantee — a panic in one access phase would crash the whole request instead of degrading gracefully.
- Safe modification: Preserve the per-phase recover; never hoist it to a single outer recover.
- Test coverage: Verify panic-injection tests exist for each phase.

**Async write-through caches and write-back (goroutine leak surface):**

- Files: `internal/access/cache_writethrough.go` (line 25), `internal/idx5/writeback.go` (line 15), `internal/llm/stream.go` (line 26)
- Why fragile: Async goroutines tracked by WaitGroups; `idx5/writeback.go` mandates panic recovery. Dropping the WG tracking or recovery leaks goroutines or crashes on a single bad write.
- Safe modification: Keep WG tracking and `defer recover()`; ensure context cancellation reaches every spawned goroutine.

**Adapter registry duplicate-name invariant:**

- Files: `internal/adapters/registry.go` (line 142, 647 lines — largest file in the repo)
- Why fragile: Duplicate-name detection is a "load-bearing invariant." `SkipAuthCheck` (line 55) bypasses `AuthEnvVars` validation — a footgun if enabled outside tests.
- Safe modification: Treat the 647-line registry as a refactor candidate (largest source file); keep duplicate detection and auth-check enforcement intact.

**Browser pool / Playwright child-process management:**

- Files: `internal/access/phase5_browser.go` (line 20)
- Why fragile: Acquire/release of Playwright child processes; leaks or double-release corrupt the pool.
- Safe modification: Pair every acquire with a deferred release; verify process cleanup on context cancel.

**Faithfulness gate emergency-rollback bypass:**

- Files: `internal/synthesis/citation/citation.go` (lines 45, 96)
- Why fragile: `ModeOff` bypasses the faithfulness gate entirely. If left on, unverified/hallucinated citations pass through.
- Safe modification: Ensure `ModeOff` is never the default and is logged loudly when active.

---

## Deployment Concerns (Apple Silicon / Docker)

**Recently-fixed local stack issues (regression-prone):**

- Problem: The local docker stack required five fixes to run on Apple Silicon (arm64). Fixes: pass `--env-file .env` (vars were blank); pin searxng by digest (dated tag removed upstream); fix qdrant/litellm/meili healthchecks (images lack `wget`; busybox resolves localhost to IPv6 while services bind IPv4 only); COPY src before pip install for src-layout wheel builds; install CPU-only torch to avoid ~3GB of unused NVIDIA CUDA libs on arm64.
- Files: `Makefile`, `deploy/docker-compose.yml`, `services/embedder/Dockerfile`, `services/tokenizer-ko/Dockerfile`, fixed in commit `40dd75e`
- Impact: These are environment-specific workarounds. Upstream image/tag changes (e.g., another searxng digest removal) or a torch dependency bump can re-break the arm64 stack. Healthchecks assuming IPv4 binding are brittle.
- Fix approach: Add a CI job that builds and health-checks the full compose stack on arm64 (a `compose-check.yml` workflow exists at `.github/workflows/compose-check.yml` — verify it exercises arm64). Pin image digests deliberately and document the renewal process.

**searxng JSON format fix:**

- Problem: The searxng adapter required enabling JSON response format (commit `e0bedcc`) — a runtime config dependency on the searxng instance.
- Files: `internal/adapters/searxng/client.go`, `deploy/searxng/`
- Impact: A searxng instance without JSON format enabled silently breaks the only live web-search adapter.
- Fix approach: Validate searxng JSON support at adapter startup and fail loudly if misconfigured.

**Web UI CSP / interactivity fix:**

- Problem: Client interactivity was broken by CSP and required a fix (commit `828985f`).
- Files: `web/src/` (Next.js app), CSP config
- Impact: CSP regressions silently disable client-side behavior. The web UI is also blocked end-to-end because `usearch-api` does not serve (see Tech Debt).
- Fix approach: Add a smoke test that loads the UI and verifies hydration/interactivity.

---

## Dependencies at Risk

**powernap LSP client pinned to a moving monorepo subpackage:**

- Risk: `github.com/charmbracelet/x/powernap` is pinned to v0.1.4 (a subpackage of a large monorepo). Upstream bumps may drift the internal `sourcegraph/jsonrpc2` dependency or change abstractions.
- Files: `go.mod`, `.claude/rules/moai/core/lsp-client.md` (documents the pin + upgrade policy)
- Impact: This concerns the MoAI tooling layer, not the search product runtime; low product risk but documented.
- Migration plan: Follow the documented upgrade policy (run the three language integration tests before bumping).

**searxng image tag fragility:**

- Risk: The compose file had to pin searxng by digest because a dated tag was removed from the registry upstream.
- Files: `deploy/docker-compose.yml`
- Impact: Future digest removals or image deprecations break the only live web-search backend.
- Migration plan: Mirror critical images to a private registry, or vendor a known-good searxng.

---

## Missing Critical Features

**No live HTTP API server:**

- Problem: `usearch-api` does not serve (SPEC-IR-001 unimplemented). The synthesis SSE endpoint, admin endpoints, and adapter registry are wired into a mux that is never bound to a listener.
- Blocks: The entire web UI, any programmatic REST/SSE client, and the `pkg/client` Go SDK from being usable end-to-end.

**No deep-research execution:**

- Problem: `usearch deep` and the REPL pipeline are stubs; cost ledger persistence is a no-op.
- Blocks: The multi-agent deep-research product (Researcher → Reviewer → Writer → Verifier) and reliable budget enforcement.

**No real authentication:**

- Problem: CLI `login` is a skeleton; API identity falls back to spoofable headers (SPEC-AUTH-001 / AUTH-002 pending).
- Blocks: Safe multi-tenant / production deployment.

**Semantic retrieval inactive:**

- Problem: Embeddings are zero-vectors (SPEC-IDX-002 pending).
- Blocks: Dense/hybrid semantic ranking; only lexical retrieval is meaningful.

---

## Test Coverage Gaps

**Entrypoints without unit tests:**

- What's not tested: `cmd/usearch-api` (only handler-level tests under `handlers/`, no test for `main.go` route registration), `cmd/usearch-mcp` (no `_test.go`), `pkg/client` (the Go SDK has no test).
- Files: `cmd/usearch-api/main.go`, `cmd/usearch-mcp/main.go`, `pkg/client/`
- Risk: Route-registration / loopback-middleware wiring regressions and SDK contract breaks go unnoticed.
- Priority: Medium (usearch-api main is a stub today, but its route wiring is the foundation SPEC-IR-001 will build on).

**Stub code masks behavioral test coverage:**

- What's not tested (meaningfully): The cost ledger has tests against an SQL-validation stub (`parseSQL`/`migrationSQL`) but no test of real persistence because `WriteLedgerEntry` is a no-op. The zero-embedder "passes" tests trivially because it returns deterministic zeros.
- Files: `internal/deepagent/costguard/ledger.go`, `internal/index/embedder.go`
- Risk: Coverage numbers (release gate reached 85% per commit `7ced14b`) include stub paths, overstating real functional coverage of the unimplemented features.
- Priority: High — when SPEC-IDX-002 / DEEP-004 Phase E land, the stub tests must be replaced with behavioral tests, not extended.

**Compose / arm64 stack lacks automated verification:**

- What's not tested: The five Apple-Silicon deploy fixes are manual workarounds; there is a `compose-check.yml` workflow but its arm64 coverage and healthcheck assertions should be confirmed.
- Files: `.github/workflows/compose-check.yml`, `deploy/docker-compose.yml`
- Risk: Silent re-breakage on dependency/image bumps.
- Priority: Medium.

**Ignored errors (266 occurrences):**

- What's not tested: 266 `_ =` / `_, _ =` ignored-error sites across `internal/` and `cmd/` (non-test). Many are intentional (SSE writes, stderr prints) but the volume makes it easy to hide a real dropped error.
- Files: widespread; e.g., `cmd/usearch-api/handlers/synthesis.go` ignores SSE write/flush errors (lines 84, 112-122), `cmd/usearch/deep_cmd.go` ignores all fprintf errors.
- Risk: A genuinely actionable error (e.g., a failed cache write) could be silently swallowed.
- Priority: Low-to-Medium — audit the non-IO ignored errors specifically.

---

_Concerns audit: 2026-06-04_
