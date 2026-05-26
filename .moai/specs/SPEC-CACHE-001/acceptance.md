# SPEC-CACHE-001 Acceptance — Given/When/Then Scenarios

Created: 2026-05-04
Updated: 2026-05-08 (post-implementation reconciliation)
Author: limbowl (via manager-spec)
Status: implemented

## 0. Document Purpose

Given/When/Then acceptance scenarios for SPEC-CACHE-001 — the 5-phase
content-fetch cascade. Each scenario maps to one or more EARS REQs in
spec.md §3. Edge cases (EC-NNN) are enumerated separately. The intent
is that this document is executable as a Go test plan for the
`internal/access` package.

## 1. Coverage Matrix

| AC | Scenario | REQs covered |
|----|----------|--------------|
| AC-001 | Fetcher construction with defaults | REQ-CACHE-001 |
| AC-002 | Phase 1 hit short-circuits cascade | REQ-CACHE-002 |
| AC-003 | Phase 2 robots.txt allow → Phase 3 escalation | REQ-CACHE-003 |
| AC-004 | Phase 2 robots.txt disallow → fail-fast | REQ-CACHE-003 |
| AC-005 | Phase 3 happy path with body cap | REQ-CACHE-004 |
| AC-006 | Phase 3 WAF status escalates to Phase 4 | REQ-CACHE-004 |
| AC-007 | Phase 4 JS-challenge escalates to Phase 5 | REQ-CACHE-005 |
| AC-008 | Phase 5 Playwright renders JS page | REQ-CACHE-006 |
| AC-009 | Parent ctx cancellation halts cascade | REQ-CACHE-007 |
| AC-010 | Combined skip flags bypass Phase 2 | REQ-CACHE-008 |
| AC-011 | Cache write-through async non-blocking | REQ-CACHE-009 |
| AC-012 | Observability emitted per call | REQ-CACHE-010 |
| AC-013 | Phase panic captured + cascade continues | REQ-CACHE-011 |
| AC-014 | Concurrent invocation race-clean | REQ-CACHE-012 |
| AC-015 | SSRF guards reject crafted URLs | REQ-CACHE-013 |
| AC-016 | shouldEscalate deterministic | REQ-CACHE-014 |
| AC-017 | Shutdown drains write-throughs | REQ-CACHE-015 |
| AC-018 | Invalid URL rejected without side effects | REQ-CACHE-016 |
| NFR-001 | Cheap-path budget (Phase 1+2) ≤ 200ms p95 | NFR-CACHE-001 |
| NFR-002 | Mid-path budget (Phase 3) ≤ 10s p95 | NFR-CACHE-002 |
| NFR-003 | Heavy-path budget (Phase 5) ≤ 30s p95 | NFR-CACHE-003 |
| NFR-004 | Race-clean workload (50×100 fetches) | NFR-CACHE-004 |
| NFR-005 | Zero goroutine leaks (with exclusions) | NFR-CACHE-005 |
| NFR-006 | Per-browser memory ≤ 200 MB | NFR-CACHE-006 |

## 2. Definition of Done

- [x] All 16 EARS REQs (13 P0 + 3 P1) have at least one green Go test.
- [x] All 6 NFRs validated with explicit measurement evidence.
- [x] All edge cases in §5 covered or explicitly documented as deferred.
- [x] `go test -race ./internal/access/...` clean.
- [x] `goleak.VerifyTestMain` clean with documented playwright-go
      exclusions (`internal/poll.runtime_pollWait`, `os/exec.(*Cmd).Wait`).
- [x] Coverage ≥ 85% in `internal/access/` (benchmarks excluded).
- [x] Three new metric families registered (`AccessPhaseAttempts`,
      `AccessPhaseDuration`, `AccessFetchTotal`); cardinality allowlist
      extended with `phase` + `blocked`.
- [x] Two new Go module deps pinned (`playwright-go v0.5700.1`,
      `temoto/robotstxt`).
- [x] TRUST 5 gates: Tested (≥85%), Readable (godoc + naming),
      Unified (gofmt/goimports), Secured (SSRF guards + robots.txt
      RFC 9309), Trackable (Conventional Commit with SPEC-CACHE-001).
- [x] Pre-submission self-review per workflow-modes.md.

## 3. Functional Scenarios

### AC-001 — Fetcher construction with defaults

Maps to REQ-CACHE-001.

**Given** zero-valued `Options{}` (no Playwright, no IndexLookup).

**When** the caller invokes `New(opts)`.

**Then** the returned `*Fetcher` has:
- `MaxBrowsers = 2`
- `PerPhaseTimeout = {1:100ms, 2:200ms, 3:10s, 4:15s, 5:30s}`
- `RobotsTTL = 24h`
- `MaxBodyBytes = 10 * 1024 * 1024`
- `RedirectMaxHops = 5`
- `PlaywrightEnabled = false` (opt-in)
- `CacheWriteThrough = false` (opt-in)

**And** `Close()` is a no-op (no Playwright runtime; no browser pool).

**And** when `Options.PlaywrightEnabled = true` AND
`AutoInstallPlaywright = false` AND `playwright.Install` would fail,
`New` returns `(nil, ErrPlaywrightUnavailable)`.

**Verification**: `TestNewNormalisesDefaults`,
`TestNewPlaywrightUnavailable`, `TestFetchAlwaysReturnsResult`,
`TestCloseClosesPlaywright` in `access_lifecycle_test.go`.

### AC-002 — Phase 1 hit short-circuits

Maps to REQ-CACHE-002.

**Given** an `IndexLookup` stub returning `(&NormalizedDoc{...}, true, nil)`
for the target URL.

**When** the caller invokes
`(*Fetcher).Fetch(ctx, "https://example.com/x", FetchOptions{})`.

**Then** the cascade halts at Phase 1; `result.FinalPhase == 1`,
`result.Outcome == "success"`, `len(result.PhaseAttempts) == 1`.

**And** Phases 2-5 are NOT attempted (verified via stub adapter
counters that remain at 0).

**Verification**: `TestFetchPhase1HitReturnsImmediately`,
`TestFetchPhase1MissEscalatesToPhase2`,
`TestFetchPhase1NilPortSkips` in `cascade_test.go`.

### AC-003 — Robots.txt allow → Phase 3

Maps to REQ-CACHE-003.

**Given** an `httptest.Server` returning `User-agent: *\nAllow: /` for
`/robots.txt`.

**When** Phase 2 runs.

**Then** `data.TestAgent(path, UserAgent) == true`; the cascade
escalates to Phase 3; Phase 2 attempt is recorded with
`Outcome: "success"`.

### AC-004 — Robots.txt disallow → fail-fast

Maps to REQ-CACHE-003.

**Given** an `httptest.Server` returning `User-agent: *\nDisallow: /`
for `/robots.txt`.

**When** Phase 2 runs.

**Then** the cascade halts; the returned error is
`*FetchError{Category: "blocked", Reason: "robots.txt disallow"}`.

**And** the robots.txt is cached per-host (24 h TTL); a second `Fetch`
to the same host within TTL does not re-fetch `/robots.txt`.

**And** the RFC 9309 corner cases hold:
- 4xx response → "allow all" (Phase 3 attempted)
- 5xx response → "disallow all" (cascade halts with `CategoryBlocked`)

**Verification**: 7 tests in `robots_test.go` + `phase2_test.go`.

### AC-005 — Phase 3 happy path with body cap

Maps to REQ-CACHE-004.

**Given** an `httptest.Server` returning a 20MB JSON body.

**When** Phase 3 runs with `Options.MaxBodyBytes = 10*1024*1024`.

**Then** the returned `FetchedContent.Body` is exactly 10 MiB long
(verified by `len(result.Content.Body) == 10*1024*1024`).

**And** redirect chain ≤ 5 hops is followed; the 6th hop returns
`*FetchError{Category: "permanent", Reason: "too many redirects"}`.

### AC-006 — Phase 3 WAF status → Phase 4

Maps to REQ-CACHE-004.

**Given** an `httptest.Server` returning HTTP 403 with header
`cf-ray: abc123`.

**When** Phase 3 runs.

**Then** the cascade escalates to Phase 4 (verified by Phase 4 stub
hit count).

**Counterexample**: HTTP 429 (rate limited) returns
`*FetchError{Category: "rate_limited"}` and does NOT escalate; the
caller handles rate-limit recovery.

### AC-007 — Phase 4 JS challenge → Phase 5

Maps to REQ-CACHE-005.

**Given** an `httptest.Server` returning HTTP 403 with body containing
`<noscript>cf-please-stand-by</noscript>` AND
`Options.PlaywrightEnabled == true`.

**When** Phase 4 runs.

**Then** the cascade escalates to Phase 5; the Phase 4 attempt is
recorded with `Outcome: "failure"` and reason "js-challenge".

**Counterexample**: same condition but `PlaywrightEnabled == false`
→ cascade halts at Phase 4 with the underlying error.

### AC-008 — Phase 5 Playwright renders JS

Maps to REQ-CACHE-006.

**Given** the `tag integration` build with Playwright browsers
pre-installed.

**Given** an `httptest.Server` returning HTML that requires JavaScript
to render its primary content.

**When** the caller invokes `Fetch` with `PlaywrightEnabled = true`.

**Then** the cascade reaches Phase 5; `result.Content.Body` contains
the rendered output (not the raw HTML stub).

**And** the browser is returned to the pool (verified via pool counter
delta == 0 after `Fetch` returns).

**And** browser pool blocking is enforced: with `MaxBrowsers = 1` and
two concurrent fetches, the second blocks then succeeds within the
per-phase budget.

### AC-009 — Parent ctx cancellation

Maps to REQ-CACHE-007.

**Given** an `httptest.Server` that sleeps for 5 s before responding;
`Options.PerPhaseTimeout[3] = 10s`; the caller's `ctx` has a 100 ms
deadline.

**When** the caller invokes `Fetch(ctx, ...)`.

**Then** the cascade halts within ~100 ms (verified within
`[100ms, 150ms]` band); `result.Outcome == "timeout"`; no Phase 4-5
attempt is recorded; `Fetch` returns `(*FetchResult, nil)` (NOT
`ctx.Err()` directly — the cascade exposes the truncated cascade in the
result).

**And** the per-phase ctx is bounded by `min(perPhaseTimeout,
timeUntil(parentCtx.Deadline))` — when parent has 50 ms remaining and
per-phase budget is 200 ms, the phase ctx times out at ~50 ms.

### AC-010 — Combined skip flags

Maps to REQ-CACHE-008.

**Given** `FetchOptions{SkipHEADProbe: true, SkipRobotsTxt: true}`.

**When** the caller invokes `Fetch`.

**Then** Phase 2 is recorded as `Outcome: "skipped"`; the cascade goes
1 → 3 directly.

**Documented constraint**: production deploys MUST NOT set both flags;
a future SPEC-SEC-001 audit gates this combination behind build tag
`integration`.

### AC-011 — Cache write-through async

Maps to REQ-CACHE-009.

**Given** `Options.CacheWriteThrough = true` AND a non-nil
`IndexLookup` whose `Upsert` simulates a 5 s delay.

**When** Phase 3 succeeds.

**Then** `Fetch` returns within the Phase 3 budget (well before the
5 s `Upsert` completes); the caller does NOT block on the upsert.

**And** within 30 s, `IndexLookup.Upsert` is called with a
`[]types.NormalizedDoc{...}` constructed from the FetchedContent
(`SourceID: "access-cache"`, `DocType: DocTypeWebpage`,
`Body: <fetched bytes>`).

**And** `Close()` / `Shutdown(ctx)` drain in-flight write-through
goroutines within 5 s; `goleak.VerifyNone(t)` is clean.

### AC-012 — Observability per call

Maps to REQ-CACHE-010.

**Given** an in-memory OTel exporter and a Prometheus snapshot before
the call.

**When** `Fetch` runs a 3-phase cascade (Phase 1 miss → Phase 2 success
→ Phase 3 success).

**Then**:
- ONE OTel parent span `access.fetch` is recorded with attributes
  `access.url_host`, `access.final_phase=3`, `access.outcome="success"`,
  `access.elapsed_seconds`.
- THREE child spans `access.phase1`, `access.phase2`, `access.phase3`
  are recorded with parent SpanContext == `access.fetch`.
- THREE counter increments on `AccessPhaseAttempts.WithLabelValues(phase, outcome)`
  (one per attempted phase).
- THREE histogram observations on `AccessPhaseDuration.WithLabelValues(phase)`.
- ONE counter increment on `AccessFetchTotal.WithLabelValues("success")`.
- ONE slog INFO record with attributes
  `{request_id, url_host, final_phase, outcome, elapsed_seconds,
  phase_attempt_count}`.

**And** the package is nil-safe: with `Obs: nil`, `Fetch` completes
without panicking.

**And** the package introduces NO metric families beyond the three
named above (verified by Gather snapshot delta).

### AC-013 — Phase panic captured

Maps to REQ-CACHE-011.

**Given** a stub Phase 3 implementation that panics.

**When** `Fetch` runs.

**Then** the per-phase `defer recover()` converts the panic into
`*FetchError{Category: "unavailable", Reason: "phase 3 panicked: ..."}`;
the cascade escalates to Phase 4 (per `shouldEscalate`); the process
does NOT crash.

**And** a slog WARN record is emitted with the captured
`runtime/debug.Stack()` output.

**And** `goleak.VerifyNone(t)` is clean after the panicking call.

### AC-014 — Concurrent race-clean workload

Maps to REQ-CACHE-012, NFR-CACHE-004.

**Given** one `*Fetcher` instance with `MaxBrowsers = 4`,
`PlaywrightEnabled = false` (Phases 1-4 exercised).

**When** 50 caller goroutines × 100 fetches each (5,000 total) run
against a stub server under `go test -race`.

**Then** zero race-detector alarms attributable to the access package.

**And** every `*FetchResult.PhaseAttempts` slice is non-nil and
properly ordered.

**And** `goleak.VerifyNone(t)` is clean after the workload completes
(playwright-go exclusions documented in NFR-CACHE-005).

**And** `AccessPhaseAttempts` counter values are monotonically
non-decreasing across the workload.

### AC-015 — SSRF guards

Maps to REQ-CACHE-013.

Table of inputs → expected outcomes (9 cases):

| Input URL | Expected |
|-----------|----------|
| `file:///etc/passwd` | `*FetchError{Category:"blocked", Reason:"scheme \"file\" not allowed"}` |
| `javascript:alert(1)` | `Reason:"scheme \"javascript\" not allowed"` |
| `http://127.0.0.1:9090/` | `Reason:"private/loopback IP 127.0.0.1"` |
| `http://10.0.0.1/` | `Reason:"private/loopback IP 10.0.0.1"` (RFC1918) |
| `http://172.16.0.1/` | RFC1918 blocked |
| `http://192.168.1.1/` | RFC1918 blocked |
| `http://169.254.169.254/latest/meta-data/` | AWS metadata blocked |
| `http://[::1]/` | IPv6 loopback blocked |
| Redirect to `http://10.0.0.1/` after public start | Blocked at hop 1 by `validateRedirect` |

**Counterexample**: `Options.AllowPrivateNetworks = true` permits
loopback (test fixture only).

**And** `pinnedIPDialer` prevents DNS rebinding: when the DNS resolver
returns a public IP on first call and `127.0.0.1` on second call, all
subsequent dials use the first-pinned IP.

### AC-016 — shouldEscalate determinism

Maps to REQ-CACHE-014.

**Given** the 5×4 (phase, outcome) matrix.

**When** `shouldEscalate(&PhaseAttempt{Phase, Outcome})` is called.

**Then** the result matches the documented predicate table per
spec.md §3.4 and is byte-equal across repeated calls (no I/O,
no time, no randomness).

### AC-017 — Shutdown drains write-throughs

Maps to REQ-CACHE-015.

**Given** an active `*Fetcher` with 3 in-flight cache-write-through
goroutines.

**When** the caller invokes `Shutdown(ctx)` with a 10 s timeout.

**Then** all 3 write-throughs complete or time out cleanly within the
ctx deadline; subsequent `Fetch` calls return `ErrShuttingDown`; the
browser pool is closed; `pw.Stop()` is called.

### AC-018 — Invalid URL rejected

Maps to REQ-CACHE-016.

**Given** `url ∈ {"", " ", "://not a url"}`.

**When** `Fetch` is called.

**Then** the function returns immediately with
`*FetchError{Category: "permanent", Reason: "invalid URL"}` (wraps
`ErrInvalidURL`); zero goroutines are spawned; zero counter
increments on `AccessPhaseAttempts` (verified by Gather snapshot
delta == 0); ONE slog WARN record is emitted at the call boundary.

## 4. Non-Functional Acceptance

### NFR-CACHE-001 — Cheap-path budget

- `BenchmarkFetchCheapPath` against a stub `httptest.Server` returning
  HEAD < 5 ms + robots.txt allow-all.
- `go test -bench=BenchmarkFetchCheapPath -benchtime=100x -count=5`.
- Median of 5 runs: cumulative Phase 1 + Phase 2 ≤ 200 ms p95 on amd64.

### NFR-CACHE-002 — Mid-path budget

- `BenchmarkFetchPhase3` against a stub with 1 s simulated network
  delay.
- `go test -bench=BenchmarkFetchPhase3 -benchtime=10x -count=5`.
- Median ≤ 10 s.

### NFR-CACHE-003 — Heavy-path budget

- Build-tag-gated `BenchmarkFetchPhase5` (requires Playwright install).
- Median ≤ 30 s.

### NFR-CACHE-004 — Race-clean workload

- `TestFetchConcurrent` per AC-014.

### NFR-CACHE-005 — Zero goroutine leaks

- `goleak.VerifyTestMain(m,
   goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"),
   goleak.IgnoreTopFunction("os/exec.(*Cmd).Wait"))` in
  `bench_test.go::TestMain`.
- Every `Fetch`-invoking test passes `goleak.VerifyNone(t)` with the
  same exclusion list.

### NFR-CACHE-006 — Per-browser memory ceiling

- `BenchmarkPhase5MemoryFootprint` measures
  `runtime.MemStats.HeapAlloc` before and after one browser launch.
- Assertion: `delta_HeapAlloc < 200 * 1024 * 1024`.
- Note: OS-level RSS may exceed 200 MB (Chromium baseline ~150 MB
  shared library); the NFR is on Go-process-observable allocations.

## 5. Edge Cases

### EC-001 — Eviction: robots.txt cache TTL expiry

- **Given** cache TTL = 1 ms.
- **When** a second `Fetch` runs 10 ms after the first.
- **Then** the second `Fetch` re-fetches `/robots.txt` (eviction).

### EC-002 — Metric cardinality protection

- The package introduces exactly two bounded-enum extensions:
  `phase ∈ {1,2,3,4,5}` (label name) and `blocked` (one new outcome
  value). NFR-OBS-002 test extended; no unbounded values.

### EC-003 — Rate-limit at Phase 3 (HTTP 429)

- Cascade does NOT escalate to Phase 4 (rate-limit is the caller's
  problem); returns `*FetchError{Category: "rate_limited"}`.

### EC-004 — TLS handshake error at Phase 3

- Cascade escalates to Phase 4 via `escalateTLS` predicate (inspects
  `*tls.RecordHeaderError`, `tls.AlertError`).

### EC-005 — Streaming heartbeat-only SSE in Phase 5

- Out of scope (Phase 5 only renders HTML via `page.Content()`).

### EC-006 — Robots.txt parser bug on edge directives

- Mitigated by fall-through to "allow all" on parse error; documented
  in spec.md §10 Risks.

### EC-007 — Browser pool exhaustion under high concurrency

- With `MaxBrowsers = 2` and 4 concurrent fetches, the 3rd and 4th
  block on the pool channel up to the per-phase budget; on budget
  exhaustion, return `*FetchError{Category: "timeout", Reason:
  "browser pool exhausted"}`.

### EC-008 — Cache write-through ctx independence

- The write-through goroutine uses a derived ctx with its own 30 s
  timeout; the caller's ctx cancellation does NOT cancel the
  write-through (it is fire-and-forget).

### EC-009 — Redirect to private network rejected at hop N

- `validateRedirect` re-runs `validateScheme` + `validateHost` per
  hop; rejection at any hop returns `CategoryBlocked` immediately.

### EC-010 — Empty robots.txt body

- temoto/robotstxt parses empty body as "allow all" (per RFC 9309
  default); Phase 3 attempted.

### EC-011 — Cascade halts at Phase 5 always

- `shouldEscalate(&PhaseAttempt{Phase:5, ...})` always returns false;
  there is no Phase 6.

### EC-012 — Caller deadline absent

- Without parent ctx deadline, the cascade may take up to ~55 s
  (sum of per-phase budgets). Callers MUST set their own
  `context.WithTimeout` per spec.md §2.7 H15.

## 6. Quality Gate Criteria

| Criterion | Threshold | Source |
|-----------|-----------|--------|
| Coverage (`internal/access/`) | ≥ 85% | quality.yaml |
| `go vet ./internal/access/...` | clean | go.md |
| `golangci-lint run` | zero issues | go.md |
| `go test -race ./internal/access/...` | clean | NFR-CACHE-004 |
| `goleak.VerifyTestMain` | clean with exclusions | NFR-CACHE-005 |
| BenchmarkFetchCheapPath median | ≤ 200 ms | NFR-CACHE-001 |
| BenchmarkFetchPhase3 median | ≤ 10 s | NFR-CACHE-002 |
| BenchmarkFetchPhase5 median (integration tag) | ≤ 30 s | NFR-CACHE-003 |
| Per-browser HeapAlloc delta | < 200 MB | NFR-CACHE-006 |
| Cardinality allowlist | extended with `phase` + `blocked` only | NFR-OBS-002 |
| New metric families | exactly 3 | REQ-CACHE-010 |
| New direct Go deps | exactly 2 (playwright-go, temoto/robotstxt) | §9.4 |
| TRUST 5 gates | all green | constitution |

## 7. Out-of-Scope Confirmations

Restated from spec.md §7 (Exclusions):

- PDF / HTML → plaintext extraction → SPEC-EXTRACT-001
- Per-host rate limiting → SPEC-CACHE-001a (potential)
- TLS fingerprint impersonation → SPEC-CACHE-001b (potential)
- Cookie / session reuse → out of v0.1
- Headless browser fingerprint randomisation → potential follow-up
- Compose service for browser isolation → SPEC-DEPLOY-001
- Multi-tenancy on cache write-through → SPEC-IDX-004 (M6)
- Cross-process robots.txt cache (Redis) → out of v0.1
- Per-host circuit breaker → SPEC-EVAL-002 (M8)
- Streaming body delivery (`io.Reader`) → out of v0.1
- JS interaction beyond `page.Goto + page.Content` → out of v0.1
- HTTP/3 / QUIC → out of v0.1
- HTTP/gRPC API exposure of `Fetcher` → SPEC-MCP-001 (M7)

---

*End of acceptance.md (post-hoc).*
