---
id: SPEC-CACHE-001
title: 5-Phase Access Fallback (insane-search pattern port)
version: 0.1.0
milestone: M3 — Fanout, adapters, index
status: draft
priority: P0
owner: expert-backend
methodology: tdd
coverage_target: 85
created: 2026-05-04
updated: 2026-05-04
author: limbowl
issue_number: null
depends_on: [SPEC-IDX-001, SPEC-FAN-001, SPEC-OBS-001]
blocks: [SPEC-CACHE-002 (potential), SPEC-EVAL-002]
---

# SPEC-CACHE-001: 5-Phase Access Fallback (insane-search pattern port)

## HISTORY

- 2026-05-04 (initial draft v0.1, limbowl via manager-spec):
  First EARS-formatted SPEC for the M3 5-phase access fallback. Drafted
  after deep research into the existing-code state
  (`.moai/specs/SPEC-CACHE-001/research.md`, every claim file:line-cited
  or URL-cited). Ports the insane-search 5-phase pattern (originally 4
  phases 0-3 in `https://github.com/fivetaku/insane-search` MIT) to Go,
  reconciled with the project-locked "5-phase" labeling at three
  documents (`.moai/project/roadmap.md:58`,
  `.moai/project/product.md:9`, `.moai/project/research.md:9, :74`) by
  splitting the original Phase 1 into Phase 2 (HEAD probe + robots.txt)
  and Phase 3 (standard GET) per research §2.2.

  User-locked decisions baked in:

  - **D1 Library selection**: `playwright-community/playwright-go
    v0.5700.1` (MIT, verified 2026-05-04 via WebFetch — research §7.1)
    for Phase 5. `chromedp/chromedp v0.15.1` and `go-rod/rod v0.116.2`
    rejected per research §4.2 + §4.3 (single-browser Chromium-only
    constraint; CDP-only loses Playwright's WebKit option for future
    SPEC-CACHE-002 iOS-shaped fingerprint extension). Run-phase pin
    confirmed at module/version.
  - **D2 robots.txt library**: `github.com/temoto/robotstxt` (MIT,
    production-active per research §7.4 — 285 stars, queryable
    `*RobotsData` with `FindGroup` + `Test` API). `jimsmart/grobotstxt`
    rejected (Apache-2.0, last release March 2022, ~3 years stale per
    research §7.6).
  - **D3 5-phase split**: Phase 1 = local index lookup (was Phase 0 in
    insane-search); Phase 2 = HEAD probe + robots.txt (split from
    insane-search Phase 1.a); Phase 3 = standard HTTP GET (was Phase
    1.b); Phase 4 = TLS-aware HTTP GET (was Phase 2); Phase 5 =
    Playwright headless browser (was Phase 3). Mapping documented in
    research §2.2 and §3.1 below.
  - **D4 Per-phase budget**: `Phase1=100ms, Phase2=200ms, Phase3=10s,
    Phase4=15s, Phase5=30s` (research §3.1). Operator-tunable via
    `.moai/config/sections/access.yaml`. Caller's parent ctx
    deadline takes precedence per the SPEC-FAN-001 §2.7 H15 idiom.
  - **D5 Soft-fail discipline**: Each phase records its attempt in
    `FetchResult.PhaseAttempts []PhaseAttempt`; failure cascades to
    next phase. Hard error (`ErrAllPhasesFailed`) only when all five
    phases fail OR all phases are blocked by phase-skip rules (§3.3
    research). Mirrors SPEC-IDX-001 §2.6 + SPEC-FAN-001 §2.5
    soft-fail pattern.
  - **D6 SSRF guards (HARD)**: Scheme allowlist (`http`/`https` only),
    private/loopback IP deny-by-default (RFC1918 + 169.254.0.0/16 +
    IPv6 ULA + link-local), DNS-rebind mitigation (resolve-then-pin),
    redirect host re-validation, redirect-chain cap (5 hops). Research
    §5.1. REQ-CACHE-013 makes testable.
  - **D7 robots.txt compliance**: Phase 2 fetches `/robots.txt` via
    stdlib `*http.Client.Get` with 5 s timeout, parses via
    `temoto/robotstxt`, caches per-host for `Options.RobotsTTL`
    (default 24h). Disallow → fail-fast cascade with
    `*FetchError{Category: CategoryBlocked}`. RFC 9309 semantics on
    4xx (allow) / 5xx (disallow). Research §5.2.
  - **D8 Cache write-through**: When Phase 3-5 succeeds AND
    `Options.CacheWriteThrough = true` AND IndexLookup port is wired,
    spawn an async goroutine to call `Index.Upsert` with the fetched
    content as a `NormalizedDoc{SourceID:"access-cache", DocType:
    types.DocTypeWebpage, Body: <fetched body>, ...}`. Default OFF
    until IDX-001 + IDX-002 are in production. Research §6. Decoupled
    from response latency.
  - **D9 Playwright lifecycle**: One `*playwright.Playwright` runtime
    per process (constructed in `New`, held for process lifetime).
    Per-fetch: one `*Browser` from a small pool (default
    `MaxBrowsers = 2`, ~150 MB resident each). `defer browser.Close()`
    + `defer pw.Stop()` mandated. SIGINT/SIGTERM signal handler calls
    `pw.Stop()` to prevent orphan Chromium processes (research §7.3).
    `goleak` exclusions for stdio child-process goroutines documented
    in NFR-CACHE-005 (research §7.2).
  - **D10 Content type**: Content-Type-aware response handling. HTML
    fetched in Phase 3-5 is returned verbatim (no extraction in v0.1;
    SPEC-SYN-003 M4 owns text extraction). PDF / binary content is
    returned with `FetchedContent.ContentType` populated and the body
    bytes intact; callers downstream parse if needed. v0.1 does NOT
    convert PDF → text (deferred to a future SPEC-EXTRACT-001 if
    measured value).
  - **D11 Observability**: ONE new metric family group:
    `usearch_access_phase_attempts_total{phase, outcome}` (Counter),
    `usearch_access_phase_duration_seconds{phase}` (Histogram),
    `usearch_access_fetch_total{outcome}` (Counter, no `phase` label —
    it counts whole-cascade outcomes). Bounded label values:
    `phase ∈ {1,2,3,4,5}`, `outcome ∈ {success, failure, timeout,
    blocked}`. Cardinality allowlist extended with `phase` and
    `blocked` (the latter is a new outcome value; research §1.4).
    ONE OTel parent span `access.fetch` per call with attributes
    `access.url_host`, `access.final_phase`, `access.outcome`,
    `access.elapsed_seconds`. Per-phase child spans `access.phase{N}`
    via ctx propagation. ONE slog summary record at end of `Fetch`.
    Mirrors SPEC-IDX-001 REQ-IDX-011 + SPEC-LLM-001 sole-emitter
    pattern.

  Resolved discrepancies:
  - Original insane-search uses 4 phases (0-3) per WebFetch §7.5
    research. Project documents lock "5-phase". Reconciled by
    splitting the original Phase 1 into the M3 Phase 2 (HEAD probe +
    robots.txt) and Phase 3 (standard GET) per research §2.2.
  - `.moai/project/structure.md:30-34` reserves `internal/access/
    {phase0_index,phase1_probe,phase2_tls,phase3_browser}` (4 files
    matching the 4-phase original). CACHE-001 will populate the
    directory with FIVE phase files renumbered 1-5 per the 5-phase
    model. Recommended structure.md sync follow-up: rename file
    reservations to phase1..phase5; OQ §11.4 tracks.

  16 EARS REQs (13 × P0 + 3 × P1) covering all five EARS patterns
  (Ubiquitous via REQ-CACHE-001/010/014, Event-Driven via
  REQ-CACHE-002/003/004/005/006/009/011/015, State-Driven via
  REQ-CACHE-007/012, Optional via REQ-CACHE-008/013, Unwanted via
  REQ-CACHE-016). 6 NFRs (NFR-CACHE-001 cheap-path budget,
  NFR-CACHE-002 mid-path budget, NFR-CACHE-003 heavy-path budget,
  NFR-CACHE-004 race-clean concurrent invocation, NFR-CACHE-005
  zero goroutine leaks (with playwright-go exclusions),
  NFR-CACHE-006 per-Playwright-instance memory ceiling). 8 Open
  Questions carried forward from research.md §8 for plan-auditor
  challenge.

  Two new Go module dependencies introduced by CACHE-001 (run-phase
  pins per SPEC-DEP-001 REQ-DEP-007):
  - `github.com/playwright-community/playwright-go v0.5700.1`
  - `github.com/temoto/robotstxt` (latest stable; pin at run phase)

  Insertion point: M3 access-fallback SPEC. Sequential cascade
  domain — distinct from FAN-001's parallel dispatch domain and
  IDX-001's parallel multi-store dispatch. CACHE-001 is the THIRD
  dispatch shape in M3 (alongside FAN-001 and IDX-001), each owning
  a distinct concurrency primitive: FAN-001 = errgroup parallel
  fan-out, IDX-001 = errgroup 3-way fanout, CACHE-001 = sequential
  cascade with per-phase ctx.

  Harness level: standard (single domain, ~18 source files in
  `internal/access/`, two new Go module deps with verified upstream
  stability — playwright-go is the riskier of the two due to
  Node.js runtime dependency, but MIT-licensed and production-active;
  one new optional config file `.moai/config/sections/access.yaml`
  for default tunables — see §6.7). Sprint Contract optional. Ready
  for plan-auditor review and annotation cycle (deferred to M3 SPEC
  batch end per orchestrator instruction).

---

## 1. Purpose

SPEC-CORE-001 published the typed adapter contract
(`pkg/types.NormalizedDoc` 15-field struct at
`pkg/types/normalized_doc.go:40-56` with `Body` as the ranking-input
text field). SPEC-FAN-001 (status: approved per
`.moai/specs/SPEC-FAN-001/spec.md:6`) defined `fanout.Result.Docs` as
the canonical post-dispatch slice. SPEC-IDX-001 (status: draft per
`.moai/specs/SPEC-IDX-001/spec.md:6`) declared `*Index` with
`Search(ctx, IndexQuery)` and `Upsert(ctx, []NormalizedDoc)`.
SPEC-OBS-001 (status: implemented) provides `obs.Logger`, `obs.Tracer`,
the named-collector cardinality allowlist at
`internal/obs/metrics/metrics.go:169-176`, and the request-ID
propagation infrastructure.

The `internal/access/` package does not yet exist on disk; the
target tree is reserved at `.moai/project/structure.md:30-34`. The
existing adapters at `internal/adapters/reddit/parse.go:160` and
`internal/adapters/hn/parse.go:124, :143` produce `NormalizedDoc.Body`
from the source's API response. For some classes of source — RSS-style
news feeds, blog posts behind partial-snippet APIs, academic PDFs
behind links — the API response provides snippet-only or URL-only
bodies; the actual document content must be fetched separately via
the source URL.

SPEC-CACHE-001 fills `internal/access/` with the **5-phase content-
fetch cascade** that:

1. Provides one `*Fetcher` orchestrator with `Fetch(ctx context.Context,
   url string, opts FetchOptions) (*FetchResult, error)` as the sole
   public entry point.
2. Cascades through five phases — local index lookup → HEAD probe +
   robots.txt → standard HTTP GET → TLS-aware HTTP GET → Playwright
   headless browser — escalating only on signals from the previous
   phase (per the insane-search pattern at
   `https://github.com/fivetaku/insane-search` MIT).
3. Enforces per-phase context budgets (research §3.1) so that no
   single phase can monopolise the caller's deadline.
4. Returns a normalised `FetchedContent{ url, body, content_type,
   status_code, fetched_at }` regardless of which phase succeeded.
5. Records per-phase attempt history in
   `FetchResult.PhaseAttempts []PhaseAttempt` for diagnostics + audit.
6. Implements HARD-rule SSRF guards (scheme allowlist, private-network
   deny, DNS-rebind mitigation, redirect re-validation) per research
   §5.1 — flagged explicitly as security-sensitive surface.
7. Honours robots.txt per RFC 9309 (research §5.2) using
   `temoto/robotstxt` (MIT) with per-host caching keyed on host with
   24h TTL.
8. Optionally writes successful Phase 3-5 fetches back to IDX-001 via
   the IndexLookup port (write-through cache; default OFF in v0.1
   pending IDX-001 + IDX-002 production deploy).
9. Manages Playwright runtime lifecycle (one `*Playwright` per
   process; `MaxBrowsers` browser pool; SIGINT/SIGTERM cleanup;
   goleak-clean with documented exclusions for child-process stdio
   goroutines).
10. Emits per-call observability through ONE new metric family group
    (`AccessPhaseAttempts` Counter, `AccessPhaseDuration` Histogram,
    `AccessFetchTotal` Counter), one OTel parent span `access.fetch`
    + per-phase child spans, and one slog summary record per call.

The fetcher does NOT classify intent (SPEC-IR-001 owns), does NOT
fan out to multiple adapters (SPEC-FAN-001 owns), does NOT synthesise
answers (SPEC-SYN-001 owns), does NOT generate embeddings (SPEC-IDX-002
owns), does NOT extract text from PDF / binary content (deferred), does
NOT cluster duplicates (SPEC-SYN-003 M4 owns), and does NOT
auto-disable a flapping host (SPEC-EVAL-002 M8 owns).

Completion unblocks SPEC-CACHE-002 (potential follow-up for TLS
fingerprinting + per-host rate limiting) and SPEC-EVAL-002 (M8
reliability dashboard reads CACHE-001 observability for per-phase
success rates). Closes the M3 access-fallback exit-criterion gate
(`.moai/project/roadmap.md:150` indirectly — M3 exit requires "fused
results across ≥5 adapters"; for sources whose adapters return
partial bodies, CACHE-001 backfills before fusion runs in
SPEC-IDX-001 RRF).

This is the **content-fetch infrastructure SPEC** for M3. Its
`Fetch(ctx, url, opts) (*FetchResult, error)` shape is consumed by:
- Adapters that signal partial-body content (e.g., a future
  `internal/adapters/rss/`, `internal/adapters/blog/`).
- The 5-phase access fallback when an adapter reports
  `NormalizedDoc.Metadata["body_complete"] = false` or equivalent.
- A future caller-side orchestrator (SPEC-RETRIEVE-001) that
  conditionally invokes CACHE-001 between FAN-001 and IDX-001.

---

## 2. Scope

### 2.1 In-Scope

| Ref | Item |
|-----|------|
| a | `internal/access/access.go`: `Fetcher` struct (immutable post-construction; holds `*playwright.Playwright` runtime, browser pool, `IndexLookup` port, robots.txt host cache, `*obs.Obs`, `Options`), `New(opts Options) (*Fetcher, error)` constructor (validates options, optionally pre-installs Playwright via `playwright.Install()`, opens robots.txt cache), `Close() error` (orderly shutdown of Playwright + browser pool + cache flush). |
| b | `internal/access/access.go`: public method `Fetch(ctx context.Context, url string, opts FetchOptions) (*FetchResult, error)` as the sole public entry point. |
| c | `internal/access/types.go`: `FetchOptions{UserAgent, MaxBodyBytes, AllowPrivateNetworks, SkipRobotsTxt, SkipHEADProbe, PlaywrightEnabled, CacheWriteThrough, MaxRedirects}` value type for fetch parameters; `FetchResult{Content *FetchedContent, PhaseAttempts []PhaseAttempt, FinalPhase int, Outcome string, ElapsedSeconds float64}`; `FetchedContent{URL, Body, ContentType, StatusCode, FetchedAt, Headers}`; `PhaseAttempt{Phase, StartedAt, ElapsedSeconds, Outcome, Error}`. JSON-marshalable for diagnostic dumps. |
| d | `internal/access/options.go`: `Options{Playwright PlaywrightConfig, IndexLookup IndexLookup (port), Obs *obs.Obs, MaxBrowsers int, PerPhaseTimeout map[int]time.Duration, RobotsTTL time.Duration, MaxBodyBytes int64, RedirectMaxHops int, AllowPrivateNetworks bool, CacheWriteThrough bool, PlaywrightEnabled bool, AutoInstallPlaywright bool}` with documented zero-value defaults (per §6.6 + research §3.1) and validation in `New`. |
| e | `internal/access/index_port.go`: `IndexLookup` interface — `LookupByURL(ctx, url string) (*types.NormalizedDoc, bool, error)` and `Upsert(ctx, []types.NormalizedDoc) error`. SPEC-IDX-001 `*Index` satisfies this port; CACHE-001 holds the port (not the concrete type) so IDX-001 remains a soft dependency. v0.1 ships a `noopIndexLookup` for tests. |
| f | `internal/access/phase1_index.go`: `phase1Index(ctx, url string) (*FetchedContent, error)` — calls `idx.LookupByURL(ctx, url)`; on hit, builds `FetchedContent` from the `NormalizedDoc.Body`; on miss, returns `(nil, ErrPhaseNotApplicable)`. Skipped when `IndexLookup` port is nil. |
| g | `internal/access/phase2_probe.go`: `phase2Probe(ctx, url string) (probeResult, error)` — issues HEAD request to target + parallel GET of `/robots.txt` (per research §5.2). Returns combined ProbeResult with `IsAllowed bool`, `ContentType string`, `StatusCode int`, `CacheControl string`. Caches `*RobotsData` per host for `Options.RobotsTTL`. Honours `Options.SkipHEADProbe` for testing. Fail-fast on robots.txt disallow with `*FetchError{Category: CategoryBlocked, Reason: "robots.txt disallow"}`. |
| h | `internal/access/phase3_get.go`: `phase3Get(ctx, url string, opts FetchOptions) (*FetchedContent, error)` — standard HTTP GET via stdlib `*http.Client` with default `Transport`. Caps response body at `Options.MaxBodyBytes` (default 10 MB). Follows up to `Options.RedirectMaxHops` redirects with per-hop SSRF re-validation. Returns the body + Content-Type. |
| i | `internal/access/phase4_tls.go`: `phase4TLS(ctx, url string, opts FetchOptions) (*FetchedContent, error)` — HTTP GET with custom `*tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS13, NextProtos: []string{"h2", "http/1.1"}, ServerName: <host>}` and a custom User-Agent that mimics a real browser (`Mozilla/5.0 ...`). Same body-cap and redirect rules as Phase 3. Used when Phase 3 returned a TLS handshake error or a 403/429/503 with WAF-suspicious headers. |
| j | `internal/access/phase5_browser.go`: `phase5Browser(ctx, url string, opts FetchOptions) (*FetchedContent, error)` — Playwright headless browser navigation. Acquires a `*Browser` from the pool (waits if all are busy up to per-phase budget), opens a new `*Page`, calls `page.Goto(url)` with default timeout, calls `page.Content()` to get the rendered HTML, closes the page. Browser is returned to the pool on success or discarded on browser-level error. |
| k | `internal/access/cascade.go`: the orchestrator. `(*Fetcher).Fetch(ctx, url, opts)` runs the 5-phase cascade, recording each phase attempt + escalating per the rules in §3.4 below. Returns the first successful `FetchedContent` OR `*FetchResult{...}` with all phases failed. |
| l | `internal/access/escalation.go`: `shouldEscalate(prev *PhaseAttempt) bool` — pure function encoding the escalation predicates from research §3.3. Phase 1 miss → Phase 2; Phase 2 robots.txt allow → Phase 3; Phase 3 TLS error or WAF status → Phase 4; Phase 4 JS challenge → Phase 5. |
| m | `internal/access/ssrf.go`: `validateURL(u *url.URL, opts FetchOptions) error` — implements the four HARD-rule SSRF guards (research §5.1). Pure function (no I/O). `validateRedirect(prev, next *url.URL, opts FetchOptions) error` re-runs the checks per redirect. |
| n | `internal/access/dialer.go`: `pinnedIPDialer(opts FetchOptions) *net.Dialer` — custom dialer that resolves the URL hostname ONCE and pins the IP (research §5.1 rule #3 DNS-rebind mitigation). Used by Phases 3-5 HTTP clients. |
| o | `internal/access/robots.go`: `robotsCache` — host-keyed `sync.Map` of `*RobotsData` with TTL per `Options.RobotsTTL`. Methods: `Get(host) (*RobotsData, bool)`, `Put(host, data, fetchedAt)`. Robots.txt fetcher uses `temoto/robotstxt.FromBytes`. |
| p | `internal/access/observability.go`: `emitFetch(ctx, span, result, elapsed)` helper writing `access.fetch` span attributes + slog summary record. Mirrors `internal/router/router.go:341-383::emit`. Nil-safe across `Obs`, `Obs.Metrics`, `Obs.Logger`. |
| q | `internal/access/errors.go`: package-level sentinels — `ErrAllPhasesFailed = errors.New("access: all 5 phases failed")` (returned by `Fetch` only when phases 1-5 all fail OR are all skipped); `ErrPhaseNotApplicable = errors.New("access: phase not applicable")` (sentinel used internally for skipped phases; never returned to caller); `ErrPlaywrightUnavailable = errors.New("access: playwright not installed")` (returned at `New` when `Options.PlaywrightEnabled = true` AND `playwright.Install()` fails AND `AutoInstallPlaywright = false`). |
| r | `internal/access/cache_writethrough.go`: `cacheWriteThrough(idx IndexLookup, content *FetchedContent)` — async goroutine spawned post-Phase-3-5 success. Constructs `[]types.NormalizedDoc{...}` from FetchedContent and calls `idx.Upsert(ctx, docs)`. Failures logged at WARN; not surfaced to caller. Decoupled from response latency (research §6.4). Spawned with derived ctx that has its own short timeout; the caller's ctx is NOT consumed. |
| s | `internal/obs/metrics/access.go`: `AccessPhaseAttempts *prometheus.CounterVec` (labels `[phase, outcome]`), `AccessPhaseDuration *prometheus.HistogramVec` (label `phase`), `AccessFetchTotal *prometheus.CounterVec` (label `outcome`). Registered via `registerAccess(r *prometheus.Registry)` called from `NewRegistry` in `internal/obs/metrics/metrics.go`. Cardinality allowlist extended with `phase` (5 bounded values 1-5) and `blocked` outcome value. |
| t | `.moai/config/sections/access.yaml`: NEW optional config file with default per-phase timeouts, MaxBrowsers, MaxBodyBytes, RobotsTTL, etc. |
| u | Test files matching the implementation tree: `access_test.go`, `cascade_test.go`, `escalation_test.go`, `ssrf_test.go`, `robots_test.go`, `phase1_test.go` through `phase5_test.go`, `observability_test.go`, `concurrent_test.go`, `bench_test.go` (with `TestMain` invoking `goleak.VerifyTestMain` with documented playwright-go exclusions). |

### 2.2 Out-of-Scope

[HARD] This SPEC explicitly excludes the following items. Each has a known
destination SPEC; this list prevents scope creep into CACHE-001 (the
M3 access-fallback foundation).

- **PDF / binary text extraction** (PDFMiner equivalent; OCR for image
  PDFs; HTML → plaintext via `go-readability` or Mercury Reader). →
  Future SPEC-EXTRACT-001 (post-V1). v0.1 returns the body bytes
  + Content-Type intact; downstream callers extract if needed.
- **Per-host rate limiting** (token-bucket per
  `<scheme>://<host>` keyed limiter). → Future SPEC-CACHE-001a
  (research OQ §8.3). v0.1 fetches each URL independently.
- **TLS fingerprint impersonation** (uTLS / cycleTLS — mimicking
  Chrome's exact TLS handshake byte sequence). → Future SPEC-CACHE-001b
  (research OQ §8.8). v0.1 only adjusts `MinVersion` + `NextProtos`
  + custom UA via stdlib `crypto/tls`.
- **Per-host circuit breaker** (auto-disable a host after N
  consecutive Phase 5 failures). → SPEC-EVAL-002 (M8 per
  `.moai/project/roadmap.md:102`).
- **Streaming body delivery** (return a `*FetchedContent` whose Body
  is an `io.Reader` rather than `[]byte`). → Out of v0.1; the
  `MaxBodyBytes` cap (default 10 MB) bounds memory usage; future
  SPEC-CACHE-001-STREAM if measured value.
- **Cookie / session reuse across fetches** (logged-in scrape).
  → Out of v0.1; explicitly outside the access-fallback domain.
  Adapters that need authenticated access wire their own clients.
- **JavaScript execution beyond `page.Goto` + `page.Content`**
  (clicking buttons, typing into forms, waiting for specific
  selectors). → Out of v0.1. Phase 5 is "render the page and grab
  the content"; interactive scraping is a different use case.
- **Headless browser fingerprint randomisation** (rotating User-Agent,
  viewport, timezone). → Future SPEC-CACHE-001c if measured value.
  v0.1 uses default Playwright Chromium fingerprint.
- **Compose service for browser isolation**
  (`chromedp/headless-shell` sidecar in `deploy/docker-compose.yml`).
  → Future SPEC-DEPLOY-001 if operational pain emerges. v0.1 runs
  Playwright as a child process of the Go binary.
- **Multi-tenancy enforcement on cache write-through** (per-team
  `team_id` on cached docs). → SPEC-IDX-004 (M6) when IDX-001's
  multi-tenancy enforcement lands. v0.1 writes with
  `team_id = NULL` matching IDX-001 v0.1's reservation.
- **Cross-process robots.txt cache sharing** (Redis-backed, shared
  across multiple `usearch` processes). → Out of v0.1; in-process
  `sync.Map` is sufficient at single-process scale.
- **Per-phase observability for "browser pool wait queue depth"** —
  the `MaxBrowsers = 2` default keeps queues short; metric is not
  useful at this scale.
- **HTTP/3 / QUIC support**. → Out of v0.1; stdlib `*http.Client`
  uses HTTP/1.1 + HTTP/2 via ALPN.
- **Cardinality allowlist amendment beyond `phase` + `blocked`**
  (the two new label/value additions are bounded enums).
- **HTTP / gRPC server exposure of `Fetcher`**. → SPEC-MCP-001 (M7)
  and future SPEC-API-001. Fetcher is a Go library only in v0.1.
- **GitHub Issue tracking on this SPEC** (skipped per session
  pattern).

### 2.3 Phase Pipeline Architecture

[HARD] The cascade is **strictly sequential** (one phase at a time), not
parallel like FAN-001's adapter dispatch. Each phase has its own
context derived from the parent ctx via `context.WithTimeout`. When a
phase succeeds (returns non-nil `*FetchedContent` and nil error), the
cascade halts and returns. When a phase fails or is blocked,
`shouldEscalate(prev)` decides whether to attempt the next phase.

```
Caller's parentCtx
        │
        ▼
  ┌─────────────────────────────────────────────────────┐
  │ Fetcher.Fetch(ctx, url, opts)                       │
  │                                                     │
  │   For phaseNum := 1..5 {                            │
  │     phaseCtx = WithTimeout(parentCtx, phaseN budget)│
  │     result := runPhase(phaseNum, phaseCtx, url)     │
  │     PhaseAttempts.append(...)                       │
  │     if result.success { return result }             │
  │     if !shouldEscalate(result) { break }            │
  │   }                                                 │
  │   return ErrAllPhasesFailed (if no success)         │
  └─────────────────────────────────────────────────────┘
```

Properties:
- Worst-case wall-clock = sum of all 5 per-phase budgets ≈ 55 s.
- Caller's parentCtx propagates cancellation to every per-phase ctx
  via Go's context inheritance (mirrors SPEC-FAN-001 §2.5).
- No phase runs in a goroutine spawned from `Fetch` (except Phase 5's
  Playwright child-process stdio goroutines, which Playwright owns).
  The cache-write-through goroutine in §2.1 (r) is the ONLY
  fire-and-forget goroutine; it runs AFTER `Fetch` returns
  successfully, decoupled from response latency.

### 2.4 Per-Phase Context Derivation

[HARD] Each phase's ctx derives from the parent ctx as:

```
phaseDeadline = min(
    perPhaseTimeout[phase],          // Options.PerPhaseTimeout per §6.6
    timeUntil(parentCtx.Deadline())  // remaining caller budget; ∞ if no parent deadline
)
phaseCtx, cancel = context.WithTimeout(parentCtx, phaseDeadline)
```

Mirroring SPEC-FAN-001 §2.5 + SPEC-IDX-001 §2.4:
- Parent ctx propagation preserved.
- Per-phase timeout never exceeds caller's budget.
- `defer cancel()` mandatory immediately after `context.WithTimeout`.

When the per-phase ctx expires before the phase's network operation
returns, the phase function returns
`*FetchError{Category: CategoryTimeout, Cause: phaseCtx.Err()}`. The
cascade records the timeout in `PhaseAttempts` and consults
`shouldEscalate` to decide on the next phase.

### 2.5 Worker State Discipline

[HARD] CACHE-001 is single-goroutine per `Fetch` call (sequential cascade).
There are no per-phase goroutines spawned from `Fetch`. The
`PhaseAttempts` slice is built incrementally by the single calling
goroutine — no shared state, no map writes by multiple goroutines.

Caller-side concurrency: multiple goroutines MAY call
`(*Fetcher).Fetch` concurrently. NFR-CACHE-004 + REQ-CACHE-014
mandate race-clean behaviour. The shared state across concurrent
calls:
- `*playwright.Playwright` runtime — treated unsafe-by-default; per-fetch
  `*Browser` from pool is the isolation boundary (one browser per
  fetch in flight at a time).
- `*Fetcher.robotsCache` — `sync.Map`; goroutine-safe by stdlib
  contract.
- `*Fetcher.browserPool` — internal channel-based pool; goroutine-safe
  via send/recv discipline.
- `*http.Client` instances — goroutine-safe per Go stdlib.

REQ-CACHE-014 acceptance covers concurrent invocation race-cleanness.

### 2.6 SSRF Guard Implementation

[HARD] Three guard functions in `ssrf.go` enforce the security boundary:

1. `validateScheme(u *url.URL) error`:
   ```
   if u.Scheme != "http" && u.Scheme != "https" {
       return &FetchError{Category: CategoryBlocked,
           Reason: fmt.Sprintf("scheme %q not allowed", u.Scheme)}
   }
   ```

2. `validateHost(u *url.URL, opts FetchOptions) error`:
   ```
   ips, err := net.DefaultResolver.LookupIPAddr(ctx, u.Host)
   if err != nil { return ... }
   for _, ip := range ips {
       if isPrivateOrLoopback(ip.IP) && !opts.AllowPrivateNetworks {
           return &FetchError{Category: CategoryBlocked,
               Reason: fmt.Sprintf("private/loopback IP %s", ip.IP)}
       }
   }
   ```
   `isPrivateOrLoopback` checks RFC1918, loopback, link-local
   (including 169.254.169.254), IPv6 ULA, IPv6 link-local.

3. `validateRedirect(prev, next *url.URL, opts FetchOptions, hopCount
   int) error`:
   - hopCount > opts.MaxRedirects → reject.
   - Re-runs validateScheme + validateHost on `next.URL`.

The `pinnedIPDialer` in `dialer.go` resolves the hostname ONCE and
returns a `*net.Dialer.DialContext` that forces all subsequent
connections to the pinned IP — preventing DNS rebinding per research
§5.1 rule #3.

REQ-CACHE-013 makes the four guards testable.

### 2.7 Caller Responsibilities for Deadlines

[HARD] `Fetch` derives per-phase ctxs from the parent ctx exclusively. The
caller MUST apply any wall-clock deadline via
`context.WithTimeout(parentCtx, callerBudget)` BEFORE invoking
`Fetch` (mirrors SPEC-FAN-001 §2.7 H15 idiom; matches `pkg/types/
query.go:32-34` `Query.Deadline` semantics for the broader
search pipeline).

`Fetch` does NOT apply its own wall-clock cap on the entire 5-phase
cascade. The per-phase budgets (Phase1=100ms ... Phase5=30s) sum to
~55 s worst case; callers with tighter SLAs MUST set a parent
deadline.

REQ-CACHE-007 (State-Driven) makes the parent-deadline contract
testable.

---

## 3. EARS Requirements

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| REQ-CACHE-001 | Ubiquitous | The package `internal/access` SHALL expose a `Fetcher` struct constructed via `New(opts Options) (*Fetcher, error)` and a public method `Fetch(ctx context.Context, url string, opts FetchOptions) (*FetchResult, error)` that returns a non-nil `*FetchResult` for every non-`ErrAllPhasesFailed` invocation. `New` SHALL return `ErrPlaywrightUnavailable` when `opts.PlaywrightEnabled == true` AND `playwright.Install()` fails AND `opts.AutoInstallPlaywright == false`. `New` SHALL normalise zero-valued Options fields to documented defaults (`MaxBrowsers=2`, per-phase timeouts per §6.6, `RobotsTTL=24h`, `MaxBodyBytes=10*1024*1024`, `RedirectMaxHops=5`). `Close()` SHALL stop the Playwright runtime and close every browser in the pool, returning the first non-nil error. | P0 | `TestNewNormalisesDefaults` (zero Options → documented defaults observed via reflection on returned `*Fetcher`); `TestNewPlaywrightUnavailable` (force playwright.Install failure → `errors.Is(err, ErrPlaywrightUnavailable)`); `TestFetchAlwaysReturnsResult` (50 invocations across success/partial-fail/all-fail paths; for success/partial assert `result != nil` AND `err` is nil; for all-fail assert `result != nil` (with PhaseAttempts populated) AND `errors.Is(err, ErrAllPhasesFailed)`); `TestCloseClosesPlaywright` (verify pw.Stop is called; verify all pooled browsers are closed). All in `access_test.go`. |
| REQ-CACHE-002 | Event-Driven | WHEN `Fetch(ctx, url, opts)` is invoked AND the `IndexLookup` port is non-nil, the cascade SHALL execute Phase 1 (`phase1Index`) which calls `idx.LookupByURL(ctx, url)`. If the lookup returns `(*NormalizedDoc, true, nil)`, the cascade SHALL halt and return `(*FetchResult{Content: <built from doc.Body>, FinalPhase: 1, Outcome: "success", PhaseAttempts: [...1 entry]}, nil)` immediately, SHALL NOT execute Phases 2-5, AND SHALL emit observability matching this outcome. If the lookup returns `(nil, false, nil)` (miss) OR an error, the cascade SHALL escalate to Phase 2. WHEN the `IndexLookup` port is nil, Phase 1 SHALL be skipped (recorded as `PhaseAttempt{Phase: 1, Outcome: "skipped"}`) and the cascade SHALL start at Phase 2. | P0 | `TestFetchPhase1HitReturnsImmediately` (stub IndexLookup returns hit; assert `result.FinalPhase == 1`, `result.Outcome == "success"`, `len(result.PhaseAttempts) == 1`, no Phase 2-5 attempted); `TestFetchPhase1MissEscalatesToPhase2` (stub returns `(nil, false, nil)`; assert `result.FinalPhase >= 2`, Phase 1 attempt recorded with `Outcome: "miss"`); `TestFetchPhase1NilPortSkips` (Options.IndexLookup is nil; assert Phase 1 is recorded as `Outcome: "skipped"`, cascade starts at Phase 2). All in `cascade_test.go`. |
| REQ-CACHE-003 | Event-Driven | WHEN Phase 2 (`phase2Probe`) runs, the cascade SHALL: (a) issue an HTTP HEAD request to the target URL with the configured User-Agent and a 5 s timeout; (b) issue a parallel HTTP GET to `<scheme>://<host>/robots.txt` (cached per-host with `Options.RobotsTTL`, default 24h); (c) parse the robots.txt body via `temoto/robotstxt.FromBytes`; (d) test the target URL path via `data.TestAgent(path, opts.UserAgent)`; (e) IF the test returns FALSE (disallowed), the cascade SHALL halt with `*FetchError{Category: CategoryBlocked, Reason: "robots.txt disallow"}`, recording the Phase 2 attempt with `Outcome: "blocked"`; (f) IF the test returns TRUE (allowed), the cascade SHALL escalate to Phase 3, recording Phase 2 attempt with `Outcome: "success"` and capturing HEAD response metadata (`ContentType`, `StatusCode`, `Cache-Control` header) for Phase 3+ context. WHEN robots.txt fetch returns HTTP 4xx (not 429), Phase 2 SHALL treat as "allow all" per RFC 9309 §2.3.1. WHEN robots.txt fetch returns HTTP 5xx OR network error, Phase 2 SHALL treat as "disallow all" (conservative, matching Google reference parser). WHEN `Options.SkipRobotsTxt == true`, Phase 2 SHALL skip the robots.txt fetch + test step (HEAD probe still runs unless `Options.SkipHEADProbe == true`). | P0 | `TestPhase2RobotsAllowEscalates` (stub robots.txt allows; assert Phase 3 attempted); `TestPhase2RobotsDisallowFailsFast` (stub robots.txt disallows path; assert err is `*FetchError{CategoryBlocked}` with reason "robots.txt disallow", NO Phase 3-5 attempted); `TestPhase2Robots4xxAllowAll` (robots.txt returns 404; assert allow-all behaviour, Phase 3 attempted); `TestPhase2Robots5xxDisallowAll` (robots.txt returns 503; assert disallow-all, err is CategoryBlocked); `TestPhase2RobotsCachedPerHost` (two consecutive Fetch calls to same host; assert robots.txt fetched once, second call uses cache); `TestPhase2RobotsTTLExpiry` (cache TTL = 1ms; second call after sleep 10ms re-fetches); `TestPhase2SkipRobotsTxt` (Options.SkipRobotsTxt=true; assert no robots.txt fetch). All in `robots_test.go` + `phase2_test.go`. |
| REQ-CACHE-004 | Event-Driven | WHEN Phase 3 (`phase3Get`) runs, the cascade SHALL: (a) construct `*http.Request` with the configured User-Agent and `Accept: */*`; (b) execute via stdlib `*http.Client` whose `Transport` uses the `pinnedIPDialer` (DNS-rebind mitigation per §2.6); (c) follow up to `Options.RedirectMaxHops` redirects (default 5), re-validating each Location URL via `validateRedirect` (SSRF guard); (d) read the response body up to `Options.MaxBodyBytes` (default 10 MB) using `io.LimitReader`; (e) on HTTP 200, return `*FetchedContent{URL: finalURL, Body: <body bytes>, ContentType: <header>, StatusCode: 200, FetchedAt: time.Now().UTC()}`; (f) on HTTP 4xx (excluding 429), return `*FetchError{Category: CategoryPermanent, HTTPStatus: <code>}`; (g) on HTTP 429, return `*FetchError{Category: CategoryRateLimited, HTTPStatus: 429}` (cascade does NOT escalate to Phase 4 for rate-limit; rate-limit is caller's responsibility); (h) on HTTP 5xx OR network error, return `*FetchError{Category: CategoryUnavailable, HTTPStatus: <code or 0>}` triggering escalation to Phase 4. The cascade SHALL escalate from Phase 3 to Phase 4 ONLY when the error category is CategoryUnavailable AND the underlying cause is a TLS handshake error OR a 403/429/503 with WAF-suspicious header pattern (Cloudflare `cf-ray`, Akamai `x-akamai-`, Fastly `x-served-by`); other Phase 3 failures halt the cascade. | P0 | `TestPhase3HappyPath200` (httptest.Server returns 200 with body; assert `result.Outcome == "success"`, `FinalPhase == 3`, body content matches); `TestPhase3HonoursMaxBodyBytes` (server returns 20MB; Options.MaxBodyBytes=10MB; assert returned body is exactly 10MB); `TestPhase3FollowsRedirect` (3-hop redirect within same host; assert 200-path fetched); `TestPhase3RejectsRedirectChainOver5` (6-hop chain; assert error after 5 hops); `TestPhase3HTTP404PermanentNoEscalate` (assert `errors.Is(err, ErrPermanent)`, NO Phase 4 attempted); `TestPhase3HTTP500EscalatesToPhase4` (assert Phase 4 attempted); `TestPhase3CloudflareWAFEscalates` (response 403 with header `cf-ray: abc123`; assert Phase 4 attempted); `TestPhase3HTTP429NoEscalate` (assert err CategoryRateLimited, no Phase 4). All in `phase3_test.go`. |
| REQ-CACHE-005 | Event-Driven | WHEN Phase 4 (`phase4TLS`) runs (escalation triggered per REQ-CACHE-004 (h)), the cascade SHALL: (a) construct an `*http.Client` with a custom `*tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS13, NextProtos: []string{"h2", "http/1.1"}, ServerName: <host>}`; (b) use a browser-shaped User-Agent (e.g., `Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36`); (c) execute the GET with the same `pinnedIPDialer`, body cap, and redirect rules as Phase 3; (d) return `*FetchedContent` on 200; (e) on TLS handshake error or status in `{403, 429, 503}` with JS-required challenge body pattern (presence of `<noscript>` block OR known WAF bodies like `cf-please-stand-by`), return `*FetchError{Category: CategoryUnavailable, Reason: "js-challenge"}` triggering escalation to Phase 5 (only if `opts.PlaywrightEnabled == true`); (f) other failures halt the cascade with the appropriate error category. | P0 | `TestPhase4HappyPath200` (httptest.Server with self-signed cert + ServerName=localhost; assert success); `TestPhase4CustomUserAgent` (capture request header; assert UA contains "Mozilla/"); `TestPhase4MinVersionTLS12` (server supports only TLS 1.0; assert handshake error); `TestPhase4JSChallengeEscalatesToPhase5` (response body contains `<noscript>cf-please-stand-by</noscript>`; assert Phase 5 attempted when PlaywrightEnabled=true); `TestPhase4JSChallengeNoPlaywrightHaltsCascade` (same but PlaywrightEnabled=false; assert cascade halts with err); `TestPhase4HTTP200Success` (no Phase 5 needed). All in `phase4_test.go`. |
| REQ-CACHE-006 | Event-Driven | WHEN Phase 5 (`phase5Browser`) runs (escalation triggered per REQ-CACHE-005 (e) AND `opts.PlaywrightEnabled == true`), the cascade SHALL: (a) acquire a `*Browser` from the pool with timeout `min(perPhase5Timeout, parentCtx.Deadline-time.Now())`; (b) open a new `*Page` via `browser.NewPage()`; (c) call `page.Goto(url, ...)` with default 30 s timeout; (d) call `page.Content()` to retrieve the rendered HTML; (e) call `page.Close()` (deferred); (f) return the browser to the pool on success or close + discard on browser-level error; (g) return `*FetchedContent{URL: finalURL, Body: <rendered HTML bytes>, ContentType: "text/html", StatusCode: 200, FetchedAt: time.Now().UTC()}`. WHEN the browser pool is empty AND `Options.MaxBrowsers` browsers are already in flight, the call SHALL block on the pool channel up to the per-phase budget; on budget exhaustion, return `*FetchError{Category: CategoryTimeout, Reason: "browser pool exhausted"}`. WHEN `playwright.Install()` failed at construction AND `Options.PlaywrightEnabled` was downgraded to false, Phase 5 SHALL NOT run; the cascade SHALL halt at Phase 4 with the underlying error. | P0 | `TestPhase5RendersJSPage` (headless integration test against a local HTTP server returning JS-only content; assert `result.Content.Body` contains rendered output not the raw HTML). Build tag `integration` gates this test on the presence of installed Playwright browsers; CI workflow installs via `playwright.Install()` in TestMain when build tag set. `TestPhase5BrowserPoolBlocking` (MaxBrowsers=1; spawn 2 concurrent fetches; assert second blocks then succeeds); `TestPhase5BrowserPoolTimeout` (MaxBrowsers=0 — invalid; assert `New` rejects); `TestPhase5DisabledHaltsAtPhase4` (Options.PlaywrightEnabled=false; assert FinalPhase == 4 max); `TestPhase5BrowserClosedOnPanic` (stub adapter goroutine panics; defer browser.Close called via recover). All in `phase5_test.go`. |
| REQ-CACHE-007 | State-Driven | WHILE the parent ctx is cancelled or its deadline expires DURING any phase's network operation, the cascade SHALL: (a) record the in-flight phase's attempt with `Outcome: "timeout"` (or "cancelled" if the err is `context.Canceled`); (b) NOT escalate to subsequent phases; (c) return `(*FetchResult{Content: nil, PhaseAttempts: [...partial], FinalPhase: <last attempted>, Outcome: "timeout"}, nil)` — NOT `ctx.Err()`; (d) emit observability with the truncated cascade. The cascade SHALL NOT consume `Options.PerPhaseTimeout` beyond what fits inside the parent ctx's remaining time per §2.4. | P0 | `TestFetchParentCtxCancelMidPhase3` (parent ctx with 100ms timeout; Phase 3 server sleeps 5s; assert `result.Outcome == "timeout"`, no Phase 4-5 attempted, total elapsed ≈ 100ms); `TestFetchParentCtxAlreadyCancelled` (parent ctx pre-cancelled; assert no goroutines spawned, result.Outcome == "cancelled", no Phase 1-5 attempted (or all skipped)); `TestFetchParentCtxRemainingShorterThanPhaseBudget` (parent has 50ms left; per-phase budget 200ms; assert Phase ctx times out at ~50ms not 200ms). All in `cascade_test.go`. |
| REQ-CACHE-008 | Optional | WHERE `Options.SkipHEADProbe == true` AND `Options.SkipRobotsTxt == true`, the cascade SHALL skip Phase 2 entirely (recorded as `PhaseAttempt{Phase: 2, Outcome: "skipped"}`). This combined-skip is intended for `httptest.Server`-backed integration tests where the test server does not serve robots.txt and HEAD-probe latency adds noise. Production callers MUST NOT set both flags; a future SPEC-SEC-001 (M8) audit will enforce a build-time check that this combination is gated by build tag `integration`. | P1 | `TestFetchSkipPhase2WhenBothSkipsTrue` (Options.SkipHEADProbe=true + SkipRobotsTxt=true; assert Phase 2 attempt has `Outcome: "skipped"`, cascade goes 1 → 3 directly); `TestFetchSkipsHEADOnly` (only SkipHEADProbe=true; assert robots.txt still fetched and tested). In `cascade_test.go`. |
| REQ-CACHE-009 | Event-Driven | WHEN any Phase 3, 4, or 5 succeeds AND `Options.CacheWriteThrough == true` AND `Options.IndexLookup` is non-nil, the cascade SHALL spawn an asynchronous goroutine (with derived ctx that has its own 30 s timeout, separate from the caller's ctx) that constructs `[]types.NormalizedDoc{...}` from the FetchedContent (with `SourceID: "access-cache"`, `DocType: types.DocTypeWebpage`, `Body: <fetched body>`, `ID: docID(SourceID, URL)`) and calls `Options.IndexLookup.Upsert(ctx, docs)`. The goroutine SHALL log any error at WARN via `obs.Logger`. The cascade SHALL NOT block on the upsert; `Fetch` returns to the caller as soon as the FetchedContent is in hand. The goroutine SHALL be tracked by the goleak harness; on `Close()`, in-flight write-through goroutines SHALL drain within a 5 s timeout. | P0 | `TestFetchCacheWriteThroughAsync` (Phase 3 succeeds; assert IndexLookup.Upsert is called within 5s after Fetch returns); `TestFetchCacheWriteThroughDisabledByDefault` (CacheWriteThrough=false; assert Upsert NEVER called); `TestFetchCacheWriteThroughDoesNotBlockResponse` (Upsert sleeps 5s; assert Fetch returns within ~Phase3 budget, well before Upsert completes); `TestFetchCloseDrainsCacheWriteThroughGoroutines` (start 5 fetches with active write-throughs; call Close; assert goleak-clean within 5s). In `cascade_test.go`. |
| REQ-CACHE-010 | Ubiquitous | The `internal/access` package SHALL emit per-`Fetch` invocation: (a) ONE OTel parent span `access.fetch` (kind = internal) with attributes `access.url_host`, `access.final_phase`, `access.outcome` (one of `success`, `failure`, `timeout`, `blocked`), `access.elapsed_seconds`; per-phase child spans `access.phase1`, `access.phase2`, ..., `access.phase5` are children via ctx propagation. (b) ONE counter increment on `obs.Metrics.AccessPhaseAttempts.WithLabelValues(strconv.Itoa(phase), outcome)` PER PHASE ATTEMPTED (so a 3-phase cascade emits 3 increments). (c) ONE histogram observation on `obs.Metrics.AccessPhaseDuration.WithLabelValues(strconv.Itoa(phase))` per phase attempt. (d) ONE counter increment on `obs.Metrics.AccessFetchTotal.WithLabelValues(outcome)` per `Fetch` call (whole-cascade outcome; cardinality 4 values). (e) ONE slog record at level INFO (success) or WARN (`failure` / `timeout` / `blocked`) via `obs.Logger` with attributes `{request_id, url_host, final_phase, outcome, elapsed_seconds, phase_attempt_count}`. The package SHALL be nil-safe across `obs.Obs`, `obs.Metrics`, individual collectors, and `obs.Logger` per the pattern at `internal/router/router.go:387-401` and `internal/llm/client.go:230-252`. The package SHALL NOT register or emit ANY new Prometheus metric family beyond `AccessPhaseAttempts`, `AccessPhaseDuration`, `AccessFetchTotal`. | P0 | `TestEmitParentSpanWithAttributes` (in-memory OTel exporter; assert `access.fetch` span exists with all 4 attributes); `TestEmitChildPhaseSpansAreChildren` (Phase 3 succeeds; assert `access.phase2` and `access.phase3` spans have `access.fetch` as parent via SpanContext); `TestEmitAccessPhaseAttemptsCounterPerPhase` (3-phase cascade → 3 counter increments with correct (phase, outcome) tuples); `TestEmitAccessFetchTotalSingleIncrement` (1 Fetch → 1 increment on AccessFetchTotal); `TestEmitSlogIncludesRequestID` (ctx with `reqid.WithContext`; assert slog record contains request_id); `TestEmitSafeOnNilObs` (Fetcher with `Obs: nil`; Fetch does not panic); `TestNoNewMetricFamilies` (snapshot Gather() before+after; assert exactly 3 new families). All in `observability_test.go`. |
| REQ-CACHE-011 | Event-Driven | WHEN any phase function panics (e.g., a Playwright session segfault inside Phase 5 manifesting as a Go panic), the cascade's per-phase `defer recover()` SHALL convert the panic into `*FetchError{Category: CategoryUnavailable, Reason: fmt.Sprintf("phase %d panicked: %v", phaseNum, recovered)}`, SHALL log a slog WARN record with the captured `runtime/debug.Stack()` output, AND SHALL allow the cascade to continue to the next phase per `shouldEscalate`. The process SHALL NOT crash. | P0 | `TestCascadePhasePanicCaptured` (stub Phase 3 panics; assert cascade escalates to Phase 4); `TestCascadePhasePanicLogsStackTrace` (capture slog JSON; assert stack_trace attribute contains "goroutine "); `TestCascadePhasePanicNoLeak` (`goleak.VerifyNone` after panicking call; mirrors SPEC-FAN-001 NFR-FAN-003). All in `cascade_test.go`. |
| REQ-CACHE-012 | State-Driven | WHILE the same `*Fetcher` instance is invoked concurrently from N caller goroutines (N ≥ 1) — each calling `Fetch(ctx, url, opts)` against possibly the same OR different URLs — each call SHALL execute independently with no shared mutable state across calls (the `*Fetcher` struct is immutable post-construction; the underlying `*playwright.Playwright`, browser pool, robots cache are all goroutine-safe per their internal contracts at §2.5), the cumulative effect SHALL be N independent fetch operations with no race-detector alarms, AND the AccessPhaseAttempts counter values SHALL be monotonically non-decreasing across the workload. | P0 | `TestFetchConcurrent` in `concurrent_test.go`: 50 caller goroutines × 100 URLs (5,000 total Fetch invocations) against a stub server with deterministic responses, MaxBrowsers=4 (Phase 5 disabled to avoid Playwright in race-test for speed). Assertions: (1) zero race-detector alarms attributable to the access package; (2) every successful `*FetchResult.PhaseAttempts` slice is non-nil; (3) `goleak.VerifyNone(t)` after the test confirms zero residual goroutines (modulo documented playwright-go exclusions); (4) `AccessPhaseAttempts` counter values are monotonically non-decreasing (snapshot at start + end). |
| REQ-CACHE-013 | Optional | WHERE the SSRF guards detect a violation, the cascade SHALL fail-fast at the earliest possible point (before any network operation): (1) `validateScheme` checks `u.Scheme ∈ {"http", "https"}`; (2) `validateHost` checks resolved IPs against the private/loopback deny list (RFC1918, 127.0.0.0/8, 169.254.0.0/16 including AWS metadata, IPv6 ::1, fc00::/7, fe80::/10) UNLESS `Options.AllowPrivateNetworks == true`; (3) `pinnedIPDialer` resolves the hostname ONCE before any HTTP attempt and pins the IP for the duration; (4) `validateRedirect` re-runs scheme + host checks per redirect, capping at `Options.MaxRedirects` (default 5). On any violation, return `*FetchError{Category: CategoryBlocked, Reason: <specific>}` immediately, recording a `PhaseAttempt{Phase: 0, Outcome: "blocked"}` (Phase 0 is reserved for pre-flight checks). The cascade SHALL NOT attempt any phase when any guard fails. | P1 | `TestSSRFRejectsFileScheme` (URL `file:///etc/passwd` → CategoryBlocked, reason contains "scheme"); `TestSSRFRejectsJavascriptScheme` (URL `javascript:alert(1)`); `TestSSRFRejectsLoopback` (URL `http://127.0.0.1:9090/`; AllowPrivateNetworks=false → blocked); `TestSSRFAllowsLoopbackWhenFlagSet` (same but flag true → proceeds); `TestSSRFRejectsAWSMetadata` (URL `http://169.254.169.254/latest/meta-data/` → blocked); `TestSSRFRejectsRFC1918` (URL `http://10.0.0.1/` → blocked); `TestSSRFRejectsIPv6Loopback` (URL `http://[::1]/` → blocked); `TestSSRFPinnedIPDialerPreventsRebind` (DNS resolver returns public IP first call, 127.0.0.1 second call; assert all attempts use first-pinned IP); `TestSSRFRedirectToPrivateRejected` (Phase 3 redirect to `http://10.0.0.1/` → blocked at hop 1). All in `ssrf_test.go`. |
| REQ-CACHE-014 | Ubiquitous | The package SHALL provide pure functions for the cascade's deterministic logic: `shouldEscalate(prev *PhaseAttempt) bool` SHALL return true ONLY for the documented escalation predicates per §3.4 (Phase 1 miss → escalate; Phase 2 robots-allow → escalate; Phase 3 TLS error or WAF status → escalate; Phase 4 JS challenge AND PlaywrightEnabled → escalate; Phase 5 always halts). The function SHALL be deterministic (no I/O, no time, no randomness): same input → same output. The `validateScheme`, `validateHost`, `validateRedirect` SSRF guards SHALL be pure (where possible — `validateHost` does DNS lookup, but the deny-list check is pure given the resolved IPs). | P0 | `TestShouldEscalateTable` (table over all 5×4 phase/outcome combinations; expected escalate true/false per §3.4 spec); `TestShouldEscalateDeterministic` (same input → byte-equal output across calls); `TestValidateSchemeTable` (5 schemes: http, https, file, javascript, ftp); `TestValidateHostDenyListTable` (10 IPs: 127.0.0.1, 10.0.0.1, 172.16.0.1, 192.168.1.1, 169.254.169.254, ::1, fc00::1, fe80::1, 8.8.8.8 (allow), 1.1.1.1 (allow)); `TestValidateRedirectHopCap` (chain of 6 hops; assert 6th rejected). All in `escalation_test.go` + `ssrf_test.go`. |
| REQ-CACHE-015 | Event-Driven | WHEN the host process receives SIGINT or SIGTERM AND a `*Fetcher` instance is alive, the package SHOULD provide a `(*Fetcher).Shutdown(ctx context.Context) error` method that: (a) stops accepting new `Fetch` calls (returning `ErrShuttingDown` from any subsequent invocation); (b) drains in-flight cache-write-through goroutines up to ctx deadline; (c) closes the browser pool; (d) calls `pw.Stop()` on the Playwright runtime. The package SHALL NOT auto-register signal handlers — the host process owns SIGINT/SIGTERM via its own `signal.Notify` and calls `Shutdown(ctx)` accordingly. This contract enables clean Playwright child-process termination, preventing orphaned Chromium PIDs (research §7.3). | P1 | `TestShutdownStopsAcceptingNewFetches` (call Shutdown then Fetch → ErrShuttingDown); `TestShutdownDrainsCacheWriteThrough` (start 3 fetches with active write-throughs; call Shutdown(ctx with 10s timeout); assert all 3 write-throughs complete or timeout cleanly); `TestShutdownClosesBrowserPool` (Shutdown then assert all browsers closed); `TestShutdownStopsPlaywright` (Shutdown then assert pw.Stop called). In `access_test.go`. |
| REQ-CACHE-016 | Unwanted | IF `Fetch` is invoked with `url == ""` OR `url` containing only whitespace OR `url` failing `url.Parse`, THEN the cascade SHALL return `(nil, *FetchError{Category: CategoryPermanent, Reason: "invalid URL"})` immediately, SHALL NOT spawn any goroutine, SHALL NOT increment any AccessPhaseAttempts counter, AND SHALL emit ONE slog WARN record at the call boundary. | P0 | `TestFetchEmptyURLRejected` (url=""; assert `errors.Is(err, ErrInvalidURL)` with CategoryPermanent); `TestFetchWhitespaceURLRejected` (url=" "); `TestFetchUnparseableURLRejected` (url="://not a url"); for each: assert NO observability counters incremented (snapshot Gather before+after; delta == 0 for AccessPhaseAttempts). All in `cascade_test.go`. |

---

## 4. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| NFR-CACHE-001 | Performance (cheap-path budget) | The cumulative latency of Phase 1 + Phase 2 SHALL be ≤ 200ms p95 measured by `BenchmarkFetchCheapPath` against a stub `httptest.Server` returning a HEAD response in < 5ms and a robots.txt allow-all body. The benchmark runs as `go test -bench=BenchmarkFetchCheapPath -benchtime=100x -count=5 ./internal/access/...` on amd64. Median of 5 runs is the assertion value (passes when ≤ 200ms). This represents the typical "already-cached, network-reachable" path. |
| NFR-CACHE-002 | Performance (mid-path budget) | The Phase 3 standard HTTP GET wall-clock SHALL be ≤ 10s p95 measured by `BenchmarkFetchPhase3` against a stub `httptest.Server` returning a 1KB JSON body after a 1s simulated network delay. The benchmark runs as `go test -bench=BenchmarkFetchPhase3 -benchtime=10x -count=5`. Median of 5 runs SHALL be ≤ 10s. This budget bounds the most common fetch path for public-document URLs. |
| NFR-CACHE-003 | Performance (heavy-path budget) | The Phase 5 Playwright wall-clock SHALL be ≤ 30s p95 measured by `BenchmarkFetchPhase5` against a stub HTTP server returning a 100KB JS-heavy HTML page. The benchmark runs as `go test -tags=integration -bench=BenchmarkFetchPhase5 -benchtime=10x -count=5` (requires installed Playwright browsers; CI installs via `playwright.Install()`). Median of 5 runs SHALL be ≤ 30s. |
| NFR-CACHE-004 | Race-clean concurrent invocation | `internal/access/concurrent_test.go::TestFetchConcurrent` SHALL execute successfully under `go test -race ./internal/access/...` with the workload defined in REQ-CACHE-012: 50 caller goroutines × 100 Fetch calls each, against a stub server (Phase 5 disabled for speed; Phases 1-4 exercised). Race-detector alarms attributable to the access package SHALL be zero. Cumulative call count: 5,000 fetch invocations. |
| NFR-CACHE-005 | Zero goroutine leaks | The package SHALL pass `goleak.VerifyNone(t)` after every test that invokes `Fetch`, with documented exclusions for playwright-go runtime goroutines: `goleak.IgnoreTopFunction("internal/poll.runtime_pollWait")` (Go runtime poll for stdio pipes); `goleak.IgnoreTopFunction("os/exec.(*Cmd).Wait")` (child process reaper). The exact ignore list is confirmed during run-phase against playwright-go v0.5700.1 + Go 1.25.8. `internal/access/bench_test.go::TestMain` SHALL invoke `goleak.VerifyTestMain(m, ...exclusions...)` (mirrors SPEC-FAN-001 + SPEC-IDX-001 patterns). The package itself SHALL launch only the bounded cache-write-through goroutines (one per successful Phase 3-5 fetch with `CacheWriteThrough=true`); these are tracked + drained on `Shutdown()` per REQ-CACHE-015. |
| NFR-CACHE-006 | Memory ceiling per Playwright instance | Each Chromium browser instance SHALL consume ≤ 200 MB resident memory at steady state (post-launch, pre-navigation). Measured by `BenchmarkPhase5MemoryFootprint` which launches one browser, navigates to a 1KB page, calls `runtime.MemStats` before and after, and asserts `delta_HeapAlloc < 200*1024*1024`. NOTE: the per-browser RSS as observed by the OS may exceed 200MB (Chromium baseline is ~150 MB shared library + per-instance heap); the NFR is on Go-process-observable allocations, not OS-level RSS. With `MaxBrowsers = 2`, total Go-process overhead from browsers is bounded at ~400 MB. |

---

## 5. Acceptance Criteria

### REQ-CACHE-001 — Fetcher Construction and Public Surface

- File `internal/access/access.go` declares `Fetcher` struct with the
  documented fields (`pw *playwright.Playwright`, `browserPool chan
  *playwright.Browser`, `idx IndexLookup`, `obs *obs.Obs`, `opts
  Options`, `robotsCache *robotsCache`).
- The compile-time signature `Fetch(ctx context.Context, url string,
  opts FetchOptions) (*FetchResult, error)` is in place.
- `New(Options{PlaywrightEnabled: true, AutoInstallPlaywright: false,
  ...})` with simulated `playwright.Install()` failure returns
  `(nil, ErrPlaywrightUnavailable)`.
- `New(Options{})` accepts zero-valued non-store fields and substitutes
  defaults: `MaxBrowsers=2`, `PerPhaseTimeout` per §6.6,
  `RobotsTTL=24h`, `MaxBodyBytes=10*1024*1024`, `RedirectMaxHops=5`,
  `PlaywrightEnabled=false` (default OFF; opt-in for Phase 5),
  `CacheWriteThrough=false`.
- `Close()` closes Playwright + browser pool; returns the first
  non-nil error after attempting all closes.
- `TestNewNormalisesDefaults`, `TestNewPlaywrightUnavailable`,
  `TestFetchAlwaysReturnsResult`, `TestCloseClosesPlaywright` all
  pass.

### REQ-CACHE-002 — Phase 1 Index Lookup

- `TestFetchPhase1HitReturnsImmediately`: stub `IndexLookup`
  returning hit; assert FinalPhase=1, Outcome="success",
  PhaseAttempts has 1 entry.
- `TestFetchPhase1MissEscalatesToPhase2`: stub returns
  `(nil, false, nil)`; assert FinalPhase >= 2, Phase 1 attempt
  recorded with Outcome="miss".
- `TestFetchPhase1NilPortSkips`: Options.IndexLookup=nil; Phase 1
  Outcome="skipped"; cascade starts at Phase 2.

### REQ-CACHE-003 — Phase 2 HEAD Probe + Robots.txt

- All 7 tests in §3 REQ-CACHE-003 acceptance summary pass.
- robots.txt cache assertion: log line "robots.txt cache hit for
  example.com" appears on second consecutive Fetch to same host;
  not on third Fetch after TTL expiry.

### REQ-CACHE-004 — Phase 3 Standard GET

- All 8 tests in §3 REQ-CACHE-004 acceptance summary pass.
- Body cap test specifically asserts `len(result.Content.Body) ==
  10*1024*1024` exactly when server returns 20MB.
- Redirect cap test asserts error message contains "too many
  redirects".

### REQ-CACHE-005 — Phase 4 TLS-Aware GET

- All 6 tests in §3 REQ-CACHE-005 acceptance summary pass.
- Custom UA assertion: captured request header `User-Agent` starts
  with "Mozilla/5.0".

### REQ-CACHE-006 — Phase 5 Playwright Browser

- Build-tag-gated integration tests pass when Playwright is installed:
  `TestPhase5RendersJSPage`, `TestPhase5BrowserPoolBlocking`.
- Build-tag-free unit tests pass: `TestPhase5BrowserPoolTimeout`
  (rejects MaxBrowsers=0 at New), `TestPhase5DisabledHaltsAtPhase4`,
  `TestPhase5BrowserClosedOnPanic`.

### REQ-CACHE-007 — Parent Context Cancellation

- All 3 tests in §3 REQ-CACHE-007 acceptance summary pass.
- Specifically: total elapsed in `TestFetchParentCtxCancelMidPhase3`
  is in `[100ms, 150ms]` (parent timeout was 100ms; allow 50ms
  cleanup).

### REQ-CACHE-008 — Combined Skip Flags

- `TestFetchSkipPhase2WhenBothSkipsTrue`: Phase 2 Outcome="skipped";
  cascade goes 1 → 3.
- `TestFetchSkipsHEADOnly`: only HEAD skipped; robots.txt still
  fetched.

### REQ-CACHE-009 — Cache Write-Through

- `TestFetchCacheWriteThroughAsync`: stub IndexLookup.Upsert is
  invoked within 5s after Fetch returns.
- `TestFetchCacheWriteThroughDisabledByDefault`: Upsert never
  invoked.
- `TestFetchCacheWriteThroughDoesNotBlockResponse`: Fetch returns
  in `[Phase3 budget]`, well before Upsert's simulated 5s delay
  completes.
- `TestFetchCloseDrainsCacheWriteThroughGoroutines`: 5 active
  write-throughs; `Close()` returns within 5s; goleak clean.

### REQ-CACHE-010 — Per-Call Observability

- All 7 tests in §3 REQ-CACHE-010 acceptance summary pass.
- `TestNoNewMetricFamilies` snapshot delta == 3 (AccessPhaseAttempts,
  AccessPhaseDuration, AccessFetchTotal).

### REQ-CACHE-011 — Phase Panic Captured

- All 3 tests in §3 REQ-CACHE-011 acceptance summary pass.
- Phase Outcome recorded as "failure" (not "success" / "blocked").

### REQ-CACHE-012 — Concurrent Invocation Safety

- `TestFetchConcurrent`: 50 × 100 invocations under `-race`; zero
  alarms attributable to access package.
- `goleak.VerifyNone` clean at test end (with documented playwright
  exclusions).

### REQ-CACHE-013 — SSRF Guards

- All 9 tests in §3 REQ-CACHE-013 acceptance summary pass.
- Each guard rejection observable via `*FetchError.Reason` containing
  the specific reason ("scheme", "private/loopback IP", "redirect
  hop limit").

### REQ-CACHE-014 — Pure Function Determinism

- `TestShouldEscalateTable`: all 20 (5 phases × 4 outcomes)
  combinations match documented predicates.
- `TestShouldEscalateDeterministic`: byte-equal output across two
  calls.
- All `validateScheme` / `validateHost` / `validateRedirect` tests
  pass.

### REQ-CACHE-015 — Shutdown Method

- All 4 tests in §3 REQ-CACHE-015 acceptance summary pass.
- Post-`Shutdown` `Fetch` returns `ErrShuttingDown`.

### REQ-CACHE-016 — Invalid URL Rejection

- All 3 tests in §3 REQ-CACHE-016 acceptance summary pass.
- Counter snapshot delta exactly 0 across all AccessPhaseAttempts
  label combinations.

### NFR-CACHE-001 — Cheap-Path Budget

- `BenchmarkFetchCheapPath` invoked as
  `go test -bench=BenchmarkFetchCheapPath -benchtime=100x -count=5
  ./internal/access/...` on amd64.
- Median of 5 runs: cumulative Phase 1 + Phase 2 latency ≤ 200ms.

### NFR-CACHE-002 — Mid-Path Budget

- `BenchmarkFetchPhase3` invoked as
  `go test -bench=BenchmarkFetchPhase3 -benchtime=10x -count=5`.
- Median of 5 runs: Phase 3 wall-clock ≤ 10s.

### NFR-CACHE-003 — Heavy-Path Budget

- Build-tag-gated `BenchmarkFetchPhase5` (requires Playwright).
- Median of 5 runs: Phase 5 wall-clock ≤ 30s.

### NFR-CACHE-004 — Race-Clean Workload

- `TestFetchConcurrent` (REQ-CACHE-012) executes under `go test
  -race`; zero race-detector alarms.

### NFR-CACHE-005 — Zero Goroutine Leaks

- `TestMain` in `bench_test.go` invokes `goleak.VerifyTestMain(m,
  goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"),
  goleak.IgnoreTopFunction("os/exec.(*Cmd).Wait"))`.
- Every Fetch-invoking test passes `goleak.VerifyNone(t)` with the
  same exclusion list.

### NFR-CACHE-006 — Per-Browser Memory Ceiling

- `BenchmarkPhase5MemoryFootprint` measures `delta_HeapAlloc` after
  one browser launch; assert `< 200*1024*1024`.

---

## 6. Technical Approach

### 6.1 Files to Modify

**Created (~22 files)**:

- `internal/access/access.go` — Fetcher struct, New, Close, Shutdown,
  public Fetch surface
- `internal/access/access_test.go` — REQ-CACHE-001 + REQ-CACHE-015
  acceptance
- `internal/access/types.go` — FetchOptions, FetchResult,
  FetchedContent, PhaseAttempt
- `internal/access/options.go` — Options struct + defaults +
  validation
- `internal/access/options_test.go`
- `internal/access/index_port.go` — IndexLookup interface +
  noopIndexLookup stub
- `internal/access/cascade.go` — orchestrator (the `Fetch` body)
- `internal/access/cascade_test.go` — REQ-CACHE-002, REQ-CACHE-007,
  REQ-CACHE-008, REQ-CACHE-009, REQ-CACHE-011, REQ-CACHE-016
- `internal/access/escalation.go` — shouldEscalate pure function
- `internal/access/escalation_test.go` — REQ-CACHE-014 part 1
- `internal/access/ssrf.go` — validateScheme, validateHost,
  validateRedirect
- `internal/access/ssrf_test.go` — REQ-CACHE-013
- `internal/access/dialer.go` — pinnedIPDialer
- `internal/access/robots.go` — robotsCache + temoto/robotstxt
  integration
- `internal/access/robots_test.go` — REQ-CACHE-003 part 1
- `internal/access/phase1_index.go` — Phase 1 IndexLookup call
- `internal/access/phase2_probe.go` — Phase 2 HEAD + robots.txt
- `internal/access/phase2_test.go` — REQ-CACHE-003 part 2
- `internal/access/phase3_get.go` — Phase 3 standard GET
- `internal/access/phase3_test.go` — REQ-CACHE-004
- `internal/access/phase4_tls.go` — Phase 4 TLS-tuned GET
- `internal/access/phase4_test.go` — REQ-CACHE-005
- `internal/access/phase5_browser.go` — Phase 5 Playwright
- `internal/access/phase5_test.go` — REQ-CACHE-006 (build-tag gated
  integration)
- `internal/access/cache_writethrough.go` — async upsert goroutine
- `internal/access/observability.go` — emitFetch helper
- `internal/access/observability_test.go` — REQ-CACHE-010
- `internal/access/concurrent_test.go` — NFR-CACHE-004 + REQ-CACHE-012
- `internal/access/bench_test.go` — BenchmarkFetchCheapPath +
  BenchmarkFetchPhase3 + BenchmarkFetchPhase5 + TestMain
  (goleak.VerifyTestMain)
- `internal/access/errors.go` — sentinel errors
- `internal/obs/metrics/access.go` — AccessPhaseAttempts +
  AccessPhaseDuration + AccessFetchTotal collectors
- `.moai/config/sections/access.yaml` — NEW optional config (defaults)

**Modified (3 files)**:

- `internal/obs/metrics/metrics.go` — call `registerAccess(r)` from
  `NewRegistry`; extend cardinality allowlist with `phase` (label
  name) and `blocked` (outcome value).
- `internal/obs/obs.go` — re-export `AccessPhaseAttempts`,
  `AccessPhaseDuration`, `AccessFetchTotal` from `obs.Obs.Metrics`.
- `go.mod` / `go.sum` — add two new direct dependencies
  (playwright-go + temoto/robotstxt).

**Unchanged (by design)**:

- `pkg/types/*` — no contract change required.
- `internal/router/*`, `internal/fanout/*`, `internal/index/*`,
  `internal/adapters/*` — CACHE-001 stands alone; consumers
  call `Fetch` post-adapter or post-fanout per the caller-orchestrated
  pattern (research §1.5).
- `deploy/docker-compose.yml` — no compose service added in v0.1
  (Playwright runs as child process).
- `.moai/config/sections/quality.yaml` — `development_mode: tdd` and
  `test_coverage_target: 85` already in place.

### 6.2 Package Layout

```
internal/access/
├── access.go                                 # Fetcher struct, New, Close, Shutdown
├── access_test.go                            # REQ-CACHE-001 + REQ-CACHE-015
├── types.go                                  # FetchOptions, FetchResult, FetchedContent, PhaseAttempt
├── options.go                                # Options + defaults + validation
├── options_test.go
├── index_port.go                             # IndexLookup interface + noopIndexLookup
├── cascade.go                                # The Fetch hot path (orchestrator)
├── cascade_test.go                           # REQ-CACHE-002/007/008/009/011/016
├── escalation.go                             # shouldEscalate pure function
├── escalation_test.go
├── ssrf.go                                   # validateScheme, validateHost, validateRedirect
├── ssrf_test.go
├── dialer.go                                 # pinnedIPDialer (DNS-rebind mitigation)
├── robots.go                                 # robotsCache + temoto/robotstxt integration
├── robots_test.go
├── phase1_index.go                           # Phase 1: IndexLookup
├── phase2_probe.go                           # Phase 2: HEAD + robots.txt
├── phase2_test.go
├── phase3_get.go                             # Phase 3: standard HTTP GET
├── phase3_test.go
├── phase4_tls.go                             # Phase 4: TLS-tuned GET
├── phase4_test.go
├── phase5_browser.go                         # Phase 5: Playwright
├── phase5_test.go                            # build-tag integration
├── cache_writethrough.go                     # async upsert
├── observability.go                          # emitFetch helper
├── observability_test.go
├── concurrent_test.go                        # NFR-CACHE-004 + REQ-CACHE-012
├── bench_test.go                             # 3 benchmarks + TestMain (goleak)
├── errors.go                                 # ErrAllPhasesFailed, ErrPlaywrightUnavailable, ErrShuttingDown, ErrInvalidURL
└── testdata/
    ├── robots_allow_all.txt
    ├── robots_disallow_path.txt
    ├── waf_cf_challenge.html
    └── js_only_page.html

internal/obs/metrics/
└── access.go                                 # AccessPhaseAttempts/Duration/Total
```

### 6.3 Type Sketches (illustrative; final shapes in run phase)

```go
// internal/access/options.go
package access

const (
    defaultMaxBrowsers     = 2
    defaultRobotsTTL       = 24 * time.Hour
    defaultMaxBodyBytes    = 10 * 1024 * 1024 // 10 MB
    defaultRedirectMaxHops = 5
)

var defaultPerPhaseTimeout = map[int]time.Duration{
    1: 100 * time.Millisecond,  // Phase 1: index lookup
    2: 200 * time.Millisecond,  // Phase 2: HEAD + robots.txt
    3: 10 * time.Second,        // Phase 3: standard GET
    4: 15 * time.Second,        // Phase 4: TLS-tuned GET
    5: 30 * time.Second,        // Phase 5: Playwright
}

type Options struct {
    Playwright            PlaywrightConfig
    IndexLookup           IndexLookup
    Obs                   *obs.Obs
    MaxBrowsers           int
    PerPhaseTimeout       map[int]time.Duration
    RobotsTTL             time.Duration
    MaxBodyBytes          int64
    RedirectMaxHops       int
    AllowPrivateNetworks  bool
    CacheWriteThrough     bool
    PlaywrightEnabled     bool
    AutoInstallPlaywright bool
}

// internal/access/types.go
type FetchOptions struct {
    UserAgent            string
    SkipRobotsTxt        bool
    SkipHEADProbe        bool
    AllowPrivateNetworks bool // overrides Options.AllowPrivateNetworks per-call
}

type FetchResult struct {
    Content        *FetchedContent
    PhaseAttempts  []PhaseAttempt
    FinalPhase     int
    Outcome        string  // success | failure | timeout | blocked
    ElapsedSeconds float64
}

type FetchedContent struct {
    URL         string
    Body        []byte
    ContentType string
    StatusCode  int
    FetchedAt   time.Time
    Headers     map[string]string  // Cache-Control, etc.
}

type PhaseAttempt struct {
    Phase          int
    StartedAt      time.Time
    ElapsedSeconds float64
    Outcome        string
    Error          string  // serialised *FetchError on failure
}

// internal/access/cascade.go (the Fetch hot path)
func (f *Fetcher) Fetch(ctx context.Context, url string, opts FetchOptions) (*FetchResult, error) {
    // REQ-CACHE-016: invalid URL → permanent error.
    u, err := url.Parse(strings.TrimSpace(url))
    if err != nil || u.Scheme == "" || u.Host == "" {
        f.logInvalidURL(ctx, url)
        return nil, &FetchError{Category: CategoryPermanent, Reason: "invalid URL"}
    }

    // REQ-CACHE-013: SSRF guards.
    if err := validateScheme(u); err != nil {
        return nil, err
    }
    if err := validateHost(ctx, u, f.opts, opts); err != nil {
        return nil, err
    }

    tracer := f.tracer()
    spanCtx, span := tracer.Start(ctx, "access.fetch",
        oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
    defer span.End()

    start := time.Now()
    result := &FetchResult{}

    for phaseNum := 1; phaseNum <= 5; phaseNum++ {
        // REQ-CACHE-007: parent ctx propagation.
        if err := ctx.Err(); err != nil {
            result.Outcome = "cancelled"
            f.emit(spanCtx, span, result, time.Since(start))
            return result, nil
        }

        phaseCtx, cancel := f.derivePhaseCtx(spanCtx, phaseNum)
        attempt := f.runPhase(phaseCtx, phaseNum, u, opts)
        cancel()

        result.PhaseAttempts = append(result.PhaseAttempts, attempt)
        result.FinalPhase = phaseNum

        if attempt.Outcome == "success" {
            result.Outcome = "success"
            result.Content = attempt.content
            f.maybeWriteThrough(spanCtx, attempt.content, opts)
            break
        }

        if !shouldEscalate(&attempt) {
            result.Outcome = attempt.Outcome  // blocked / failure / timeout
            break
        }
    }

    if result.Outcome == "" {
        result.Outcome = "failure"  // all 5 phases ran without success
    }

    result.ElapsedSeconds = time.Since(start).Seconds()
    f.emit(spanCtx, span, result, time.Since(start))

    if result.Outcome == "failure" {
        return result, ErrAllPhasesFailed
    }
    return result, nil
}
```

### 6.4 Per-Phase Context Derivation

```go
// internal/access/cascade.go
func (f *Fetcher) derivePhaseCtx(parent context.Context, phase int) (context.Context, context.CancelFunc) {
    deadline := f.opts.PerPhaseTimeout[phase]
    if pDeadline, ok := parent.Deadline(); ok {
        if remaining := time.Until(pDeadline); remaining < deadline {
            deadline = remaining
        }
    }
    if deadline <= 0 {
        ctx, cancel := context.WithCancel(parent)
        cancel()
        return ctx, cancel
    }
    return context.WithTimeout(parent, deadline)
}
```

### 6.5 Escalation Predicates (REQ-CACHE-014)

```go
// internal/access/escalation.go
func shouldEscalate(prev *PhaseAttempt) bool {
    switch prev.Phase {
    case 1:
        // Phase 1 miss → escalate.
        return prev.Outcome == "miss" || prev.Outcome == "skipped"
    case 2:
        // Phase 2 robots-allow → escalate. Disallow halts.
        return prev.Outcome == "success" || prev.Outcome == "skipped"
    case 3:
        // Phase 3 escalates only on TLS error or WAF status.
        return prev.escalateTLS()  // helper inspects prev.Error structure
    case 4:
        // Phase 4 escalates on JS challenge.
        return prev.escalateJSChallenge()
    case 5:
        // Phase 5 always halts.
        return false
    }
    return false
}
```

### 6.6 Default Per-Phase Timeouts

```go
// internal/access/options.go (excerpt)
var defaultPerPhaseTimeout = map[int]time.Duration{
    1: 100 * time.Millisecond,
    2: 200 * time.Millisecond,
    3: 10 * time.Second,
    4: 15 * time.Second,
    5: 30 * time.Second,
}
```

The cumulative budget is ~55 s worst-case. Callers MUST set their
own `context.WithTimeout` per §2.7.

### 6.7 Configuration

The fetcher introduces ONE new optional config section:

```yaml
# .moai/config/sections/access.yaml (NEW; optional)
access:
  max_browsers: 2                    # NFR-CACHE-006 / OQ §11.2
  per_phase_timeout_ms:
    1: 100                           # Phase 1 index lookup
    2: 200                           # Phase 2 probe + robots.txt
    3: 10000                         # Phase 3 standard GET
    4: 15000                         # Phase 4 TLS-tuned GET
    5: 30000                         # Phase 5 Playwright
  robots_ttl_seconds: 86400          # 24h default
  max_body_bytes: 10485760           # 10 MB
  redirect_max_hops: 5
  allow_private_networks: false      # HARD: production MUST be false
  cache_write_through: false         # OFF until IDX-001/IDX-002 production-ready
  playwright:
    enabled: false                   # Opt-in; cold-launch ~2-5s
    auto_install: true               # programmatic install at startup
```

The CLI's `cmd/usearch/main.go` consumes this file and constructs
`Options` accordingly.

### 6.8 MX Tag Plan

| File::Symbol | Tag | Reason |
|--------------|-----|--------|
| `access.go::(*Fetcher).Fetch` | `@MX:ANCHOR` | Sole entry point. fan_in ≥ 4 (CLI today, MCP tomorrow, future RetrieveOrchestrator, tests). `@MX:REASON: contract boundary; signature change ripples to consumers`. `@MX:SPEC: SPEC-CACHE-001`. |
| `cascade.go::(*Fetcher).runPhase` | `@MX:WARN` | Sequential cascade with per-phase ctx + panic-recover for each phase. Removing the per-phase `defer recover()` invalidates NFR-CACHE-005 zero-leak guarantee. `@MX:REASON: panic propagation across phase boundaries`. |
| `ssrf.go::validateScheme/validateHost/validateRedirect` | `@MX:ANCHOR` | Security boundary functions. Bug here re-opens SSRF. fan_in = 1 (Fetch) but invariant-bearing. `@MX:REASON: SSRF guard; bypassing this is a CVE-class issue`. `@MX:SPEC: SPEC-CACHE-001`. |
| `dialer.go::pinnedIPDialer` | `@MX:WARN` | DNS-rebind mitigation; removing the pin breaks SSRF guard #3. `@MX:REASON: DNS-rebind attack vector`. |
| `phase5_browser.go::(*Fetcher).phase5Browser` | `@MX:WARN` | Playwright Browser pool acquire/release. `@MX:REASON: removing defer browser.Close()/pool-return invalidates NFR-CACHE-005 + NFR-CACHE-006`. |
| `cache_writethrough.go::cacheWriteThrough` | `@MX:WARN` | Spawned async goroutine; tracked via `*Fetcher.writeThroughWG`. `@MX:REASON: removing goroutine tracking breaks `Shutdown` drain contract`. |
| `escalation.go::shouldEscalate` | `@MX:NOTE` | Documents the 5-phase escalation predicates per §3.4. Future contributors look here when adding new phase conditions. |
| `cascade.go::derivePhaseCtx` | `@MX:NOTE` | Magic constants (per-phase defaults). The note documents §6.6 derivation rules. |

All tags `[AUTO]`-prefixed, include `@MX:SPEC: SPEC-CACHE-001`,
follow `code_comments: en` per `.moai/config/sections/language.yaml`.
Per-file hard limit (3 ANCHOR + 5 WARN per
`.moai/config/sections/mx.yaml`): respected.

### 6.9 Harness Level

Per `.moai/config/sections/harness.yaml` auto-routing: 16 EARS REQs
(13 × P0 + 3 × P1) + 6 NFRs touching 1 package (`internal/access/`,
~22 source/test files) + 1 cross-package edit
(`internal/obs/metrics/{metrics.go,access.go}`,
`internal/obs/obs.go`) + 1 new optional config file +
**security-sensitive surface** (SSRF guards in REQ-CACHE-013, robots.txt
compliance in REQ-CACHE-003) = the security flag pushes harness level
to **standard with security review**. Sprint Contract is OPTIONAL but
recommended. Evaluator profile `default` applies. Two new Go module
dependencies — `playwright-community/playwright-go v0.5700.1` (MIT,
production-active per research §7.1) and `temoto/robotstxt` (MIT,
production-active per research §7.4).

### 6.10 Security-Sensitive Surface (SSRF flag)

[HARD] CACHE-001 is a **security-sensitive SPEC**. The cascade fetches
arbitrary URLs supplied by upstream callers (which themselves consume
URLs from external API responses). The threat model and mitigations
are documented in §2.6 + research §5.1; the four SSRF guards are
mandatory and tested in REQ-CACHE-013. A future SPEC-SEC-001 (M8 per
`.moai/project/roadmap.md:104`) will perform the formal OWASP review;
CACHE-001 v0.1 implements the controls but does not claim a
formal-review pass.

Operators MUST:
- Set `Options.AllowPrivateNetworks = false` in production
  (test-only opt-in for `httptest.Server` 127.0.0.1 stubs).
- Set `Options.SkipRobotsTxt = false` and
  `Options.SkipHEADProbe = false` in production.
- Pin both new Go dependencies to exact versions per
  SPEC-DEP-001 REQ-DEP-007.
- Subscribe to playwright-go security advisories
  (`https://github.com/playwright-community/playwright-go/security/advisories`).
- Subscribe to temoto/robotstxt security advisories.

---

## 7. What NOT to Build (Exclusions)

[HARD] This SPEC explicitly excludes the following items. Each has a known
destination SPEC; this list prevents scope creep into CACHE-001.

- **Per-host rate limiting** → future SPEC-CACHE-001a (research §8.3).
- **TLS fingerprint impersonation** (uTLS) → future SPEC-CACHE-001b
  (research §8.8).
- **PDF / HTML → plaintext extraction** → future SPEC-EXTRACT-001
  (post-V1).
- **Cookie / session reuse** → out of scope.
- **Headless browser fingerprint randomisation** → future
  SPEC-CACHE-001c if measured value.
- **Compose service for browser isolation** (chromedp/headless-shell
  sidecar) → future SPEC-DEPLOY-001.
- **Multi-tenancy enforcement on cache write-through** → SPEC-IDX-004
  (M6).
- **Cross-process robots.txt cache (Redis-backed)** → out of v0.1.
- **Per-host circuit breaker** → SPEC-EVAL-002 (M8).
- **Streaming body delivery** (`io.Reader` instead of `[]byte`) →
  out of v0.1.
- **JavaScript interaction beyond Goto + Content** → out of v0.1.
- **HTTP/3 support** → out of v0.1.
- **Cardinality allowlist amendment beyond `phase` + `blocked`** —
  both bounded enums.
- **HTTP / gRPC API exposure** → SPEC-MCP-001 (M7).
- **GitHub Issue tracking on this SPEC** (skipped per session
  pattern).

---

## 8. TDD Plan

Development follows RED-GREEN-REFACTOR per
`quality.development_mode: tdd` (`.moai/config/sections/quality.yaml`).
Representative RED-phase tests, written before implementation, grouped
by REQ. Total: ~52 tests + 3 benchmarks. Coverage target: 85% per
`quality.test_coverage_target`. Benchmarks do not count toward coverage.
Build-tag-gated tests are excluded from default `go test ./...`; run
with `-tags=integration` to exercise Phase 5.

| # | Test | Layer | REQ | Assertion |
|---|------|-------|-----|-----------|
| 1 | `TestNewNormalisesDefaults` | `access_test.go` | REQ-CACHE-001 | Zero Options → defaults applied |
| 2 | `TestNewPlaywrightUnavailable` | `access_test.go` | REQ-CACHE-001 | Force install fail → ErrPlaywrightUnavailable |
| 3 | `TestFetchAlwaysReturnsResult` | `access_test.go` | REQ-CACHE-001 | Result is non-nil for success/partial; nil for hard-error |
| 4 | `TestCloseClosesPlaywright` | `access_test.go` | REQ-CACHE-001 | Close invokes pw.Stop + browser.Close |
| 5 | `TestFetchPhase1HitReturnsImmediately` | `cascade_test.go` | REQ-CACHE-002 | FinalPhase=1, no Phase 2-5 |
| 6 | `TestFetchPhase1MissEscalatesToPhase2` | `cascade_test.go` | REQ-CACHE-002 | Phase 2 attempted |
| 7 | `TestFetchPhase1NilPortSkips` | `cascade_test.go` | REQ-CACHE-002 | Phase 1 Outcome="skipped" |
| 8 | `TestPhase2RobotsAllowEscalates` | `phase2_test.go` | REQ-CACHE-003 | Phase 3 attempted on allow |
| 9 | `TestPhase2RobotsDisallowFailsFast` | `phase2_test.go` | REQ-CACHE-003 | CategoryBlocked, no Phase 3-5 |
| 10 | `TestPhase2Robots4xxAllowAll` | `phase2_test.go` | REQ-CACHE-003 | 404 robots.txt → allow |
| 11 | `TestPhase2Robots5xxDisallowAll` | `phase2_test.go` | REQ-CACHE-003 | 503 robots.txt → disallow |
| 12 | `TestPhase2RobotsCachedPerHost` | `robots_test.go` | REQ-CACHE-003 | Second call uses cache |
| 13 | `TestPhase2RobotsTTLExpiry` | `robots_test.go` | REQ-CACHE-003 | Re-fetches after TTL |
| 14 | `TestPhase2SkipRobotsTxt` | `cascade_test.go` | REQ-CACHE-003 | No robots.txt fetch |
| 15 | `TestPhase3HappyPath200` | `phase3_test.go` | REQ-CACHE-004 | FinalPhase=3 |
| 16 | `TestPhase3HonoursMaxBodyBytes` | `phase3_test.go` | REQ-CACHE-004 | Body capped at MaxBodyBytes |
| 17 | `TestPhase3FollowsRedirect` | `phase3_test.go` | REQ-CACHE-004 | 3-hop redirect succeeds |
| 18 | `TestPhase3RejectsRedirectChainOver5` | `phase3_test.go` | REQ-CACHE-004 | 6-hop rejected |
| 19 | `TestPhase3HTTP404PermanentNoEscalate` | `phase3_test.go` | REQ-CACHE-004 | CategoryPermanent, no Phase 4 |
| 20 | `TestPhase3HTTP500EscalatesToPhase4` | `phase3_test.go` | REQ-CACHE-004 | Phase 4 attempted |
| 21 | `TestPhase3CloudflareWAFEscalates` | `phase3_test.go` | REQ-CACHE-004 | 403 + cf-ray header → Phase 4 |
| 22 | `TestPhase3HTTP429NoEscalate` | `phase3_test.go` | REQ-CACHE-004 | CategoryRateLimited, no Phase 4 |
| 23 | `TestPhase4HappyPath200` | `phase4_test.go` | REQ-CACHE-005 | TLS handshake succeeds |
| 24 | `TestPhase4CustomUserAgent` | `phase4_test.go` | REQ-CACHE-005 | UA contains "Mozilla/" |
| 25 | `TestPhase4MinVersionTLS12` | `phase4_test.go` | REQ-CACHE-005 | TLS 1.0 server fails |
| 26 | `TestPhase4JSChallengeEscalatesToPhase5` | `phase4_test.go` | REQ-CACHE-005 | Phase 5 attempted |
| 27 | `TestPhase4JSChallengeNoPlaywrightHaltsCascade` | `phase4_test.go` | REQ-CACHE-005 | Cascade halts |
| 28 | `TestPhase4HTTP200Success` | `phase4_test.go` | REQ-CACHE-005 | No Phase 5 |
| 29 | `TestPhase5RendersJSPage` | `phase5_test.go` (integration) | REQ-CACHE-006 | Rendered output present |
| 30 | `TestPhase5BrowserPoolBlocking` | `phase5_test.go` (integration) | REQ-CACHE-006 | Second blocks then succeeds |
| 31 | `TestPhase5BrowserPoolTimeout` | `phase5_test.go` | REQ-CACHE-006 | New rejects MaxBrowsers=0 |
| 32 | `TestPhase5DisabledHaltsAtPhase4` | `phase5_test.go` | REQ-CACHE-006 | FinalPhase ≤ 4 |
| 33 | `TestPhase5BrowserClosedOnPanic` | `phase5_test.go` | REQ-CACHE-006 | defer browser.Close called |
| 34 | `TestFetchParentCtxCancelMidPhase3` | `cascade_test.go` | REQ-CACHE-007 | Outcome="timeout" |
| 35 | `TestFetchParentCtxAlreadyCancelled` | `cascade_test.go` | REQ-CACHE-007 | Outcome="cancelled" |
| 36 | `TestFetchParentCtxRemainingShorterThanPhaseBudget` | `cascade_test.go` | REQ-CACHE-007 | Phase ctx times out at parent budget |
| 37 | `TestFetchSkipPhase2WhenBothSkipsTrue` | `cascade_test.go` | REQ-CACHE-008 | Phase 2 Outcome="skipped" |
| 38 | `TestFetchSkipsHEADOnly` | `cascade_test.go` | REQ-CACHE-008 | Robots.txt still fetched |
| 39 | `TestFetchCacheWriteThroughAsync` | `cascade_test.go` | REQ-CACHE-009 | Upsert called within 5s |
| 40 | `TestFetchCacheWriteThroughDisabledByDefault` | `cascade_test.go` | REQ-CACHE-009 | Upsert NOT called |
| 41 | `TestFetchCacheWriteThroughDoesNotBlockResponse` | `cascade_test.go` | REQ-CACHE-009 | Fetch returns within Phase budget |
| 42 | `TestFetchCloseDrainsCacheWriteThroughGoroutines` | `cascade_test.go` | REQ-CACHE-009 | goleak clean within 5s |
| 43 | `TestEmitParentSpanWithAttributes` | `observability_test.go` | REQ-CACHE-010 | access.fetch span with 4 attrs |
| 44 | `TestEmitChildPhaseSpansAreChildren` | `observability_test.go` | REQ-CACHE-010 | Phase spans have access.fetch parent |
| 45 | `TestEmitAccessPhaseAttemptsCounterPerPhase` | `observability_test.go` | REQ-CACHE-010 | 3 attempts → 3 increments |
| 46 | `TestEmitAccessFetchTotalSingleIncrement` | `observability_test.go` | REQ-CACHE-010 | 1 increment per Fetch |
| 47 | `TestEmitSlogIncludesRequestID` | `observability_test.go` | REQ-CACHE-010 | request_id in slog |
| 48 | `TestEmitSafeOnNilObs` | `observability_test.go` | REQ-CACHE-010 | Nil Obs → no panic |
| 49 | `TestNoNewMetricFamilies` | `observability_test.go` | REQ-CACHE-010 | Exactly 3 new families |
| 50 | `TestCascadePhasePanicCaptured` | `cascade_test.go` | REQ-CACHE-011 | Cascade escalates after panic |
| 51 | `TestCascadePhasePanicLogsStackTrace` | `cascade_test.go` | REQ-CACHE-011 | Stack trace in slog |
| 52 | `TestCascadePhasePanicNoLeak` | `cascade_test.go` | REQ-CACHE-011 | goleak.VerifyNone clean |
| 53 | `TestFetchConcurrent` | `concurrent_test.go` | REQ-CACHE-012, NFR-CACHE-004 | 50×100 race-clean |
| 54 | `TestSSRFRejectsFileScheme` | `ssrf_test.go` | REQ-CACHE-013 | file:// blocked |
| 55 | `TestSSRFRejectsJavascriptScheme` | `ssrf_test.go` | REQ-CACHE-013 | javascript: blocked |
| 56 | `TestSSRFRejectsLoopback` | `ssrf_test.go` | REQ-CACHE-013 | 127.0.0.1 blocked |
| 57 | `TestSSRFAllowsLoopbackWhenFlagSet` | `ssrf_test.go` | REQ-CACHE-013 | Allowed with flag |
| 58 | `TestSSRFRejectsAWSMetadata` | `ssrf_test.go` | REQ-CACHE-013 | 169.254.169.254 blocked |
| 59 | `TestSSRFRejectsRFC1918` | `ssrf_test.go` | REQ-CACHE-013 | 10.0.0.1 blocked |
| 60 | `TestSSRFRejectsIPv6Loopback` | `ssrf_test.go` | REQ-CACHE-013 | ::1 blocked |
| 61 | `TestSSRFPinnedIPDialerPreventsRebind` | `ssrf_test.go` | REQ-CACHE-013 | First-pinned IP used |
| 62 | `TestSSRFRedirectToPrivateRejected` | `ssrf_test.go` | REQ-CACHE-013 | Redirect to 10.0.0.1 blocked |
| 63 | `TestShouldEscalateTable` | `escalation_test.go` | REQ-CACHE-014 | 20 phase/outcome combos |
| 64 | `TestShouldEscalateDeterministic` | `escalation_test.go` | REQ-CACHE-014 | Byte-equal output |
| 65 | `TestValidateSchemeTable` | `ssrf_test.go` | REQ-CACHE-014 | 5 schemes |
| 66 | `TestValidateHostDenyListTable` | `ssrf_test.go` | REQ-CACHE-014 | 10 IPs |
| 67 | `TestValidateRedirectHopCap` | `ssrf_test.go` | REQ-CACHE-014 | 6 hops rejected |
| 68 | `TestShutdownStopsAcceptingNewFetches` | `access_test.go` | REQ-CACHE-015 | ErrShuttingDown |
| 69 | `TestShutdownDrainsCacheWriteThrough` | `access_test.go` | REQ-CACHE-015 | Drains within ctx |
| 70 | `TestShutdownClosesBrowserPool` | `access_test.go` | REQ-CACHE-015 | All browsers closed |
| 71 | `TestShutdownStopsPlaywright` | `access_test.go` | REQ-CACHE-015 | pw.Stop called |
| 72 | `TestFetchEmptyURLRejected` | `cascade_test.go` | REQ-CACHE-016 | ErrInvalidURL |
| 73 | `TestFetchWhitespaceURLRejected` | `cascade_test.go` | REQ-CACHE-016 | ErrInvalidURL |
| 74 | `TestFetchUnparseableURLRejected` | `cascade_test.go` | REQ-CACHE-016 | ErrInvalidURL |
| 75 | `BenchmarkFetchCheapPath` | `bench_test.go` | NFR-CACHE-001 | Phase 1+2 ≤ 200ms p95 |
| 76 | `BenchmarkFetchPhase3` | `bench_test.go` | NFR-CACHE-002 | Phase 3 ≤ 10s p95 |
| 77 | `BenchmarkFetchPhase5` | `bench_test.go` (integration) | NFR-CACHE-003 | Phase 5 ≤ 30s p95 |
| 78 | `TestMain` (goleak.VerifyTestMain w/ exclusions) | `bench_test.go` | NFR-CACHE-005 | Package-level leak check |

RED-GREEN-REFACTOR per requirement:
1. RED: Write failing test for REQ-CACHE-N.
2. GREEN: Implement minimal code to pass.
3. REFACTOR: Tidy; extract shared helpers if they remove duplication;
   keep file sizes manageable (target each `.go` file < 250 LoC
   excluding tests).

Greenfield note: `internal/access/` does not exist today. There is no
behaviour to preserve; no characterization tests needed. RED tests for
REQ-CACHE-001's public API surface are written against the planned
package surface.

---

## 9. Dependencies

### 9.1 Upstream SPEC Dependencies

- **SPEC-IDX-001 (status: draft per `.moai/specs/SPEC-IDX-001/spec.md:6`)**:
  CACHE-001 declares an `IndexLookup` port that `*index.Index` is
  expected to satisfy (LookupByURL + Upsert). SOFT dep — when
  IndexLookup is nil, Phase 1 is skipped gracefully and cascade
  starts at Phase 2. CACHE-001 v0.1 ships a `noopIndexLookup` for
  tests; production wiring lands when IDX-001 is in production.
- **SPEC-FAN-001 (approved per `.moai/specs/SPEC-FAN-001/spec.md:6`)**:
  CACHE-001 borrows the per-phase ctx derivation idiom (§2.5 H18 +
  H15) and the soft-fail discipline (§2.6 H1 + H17). SOFT dep —
  CACHE-001 does not link FAN-001's package; it copies the patterns.
- **SPEC-OBS-001 (implemented)**: provides `obs.Logger`, `obs.Tracer`,
  the named-collector cardinality allowlist
  (`internal/obs/metrics/metrics.go:169-176`), `reqid.WithContext`.
  HARD dep — CACHE-001 extends the registry with `internal/obs/metrics/access.go`
  and adds `phase` to the allowlist.

### 9.2 Parallelizable

- **SPEC-CACHE-002 (potential)**: a possible follow-up SPEC for
  per-host rate limiting and TLS fingerprint impersonation. Plan
  phase can begin as soon as CACHE-001 spec.md is approved.
- **SPEC-EVAL-002 (M8)**: reliability dashboard reads CACHE-001
  observability for per-phase success rates.

### 9.3 Downstream Blocked SPECs

- **SPEC-CACHE-002 (potential)**: extends CACHE-001 with rate
  limiting + TLS fingerprinting.
- **SPEC-EVAL-002 (M8)**: consumes per-phase metrics for the
  reliability dashboard.

### 9.4 External Dependencies (run-phase pins)

TWO new Go module dependencies introduced by CACHE-001:

- `github.com/playwright-community/playwright-go v0.5700.1` —
  Playwright Go bindings (MIT, verified 2026-05-04 via WebFetch
  research §7.1)
- `github.com/temoto/robotstxt` (latest stable; pin at run phase) —
  robots.txt parser (MIT, verified 2026-05-04 research §7.4)

ZERO new module dependencies for the rest of CACHE-001:
- Go stdlib: `context`, `crypto/sha256`, `crypto/tls`, `encoding/hex`,
  `errors`, `fmt`, `io`, `net`, `net/http`, `net/url`, `os`,
  `os/exec`, `runtime/debug`, `strings`, `strconv`, `sync`,
  `time`, `unicode`
- `pkg/types` (already pinned via SPEC-CORE-001)
- `internal/obs` and `internal/obs/reqid` and `internal/obs/metrics`
  (already pinned via SPEC-OBS-001)
- `go.opentelemetry.io/otel/{attribute,codes,trace}` (already
  pinned via SPEC-OBS-001)
- Test-only: `go.uber.org/goleak v1.3.0` (already pinned indirect
  via `go.mod:30`)

---

## 10. Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| SSRF via crafted redirect chain | Medium | High | REQ-CACHE-013 + four HARD-rule guards (§2.6); pinnedIPDialer prevents DNS rebind; redirect re-validation per hop; `TestSSRFRedirectToPrivateRejected` asserts. |
| robots.txt non-compliance complaint from site owners | Medium | High | REQ-CACHE-003 enforces RFC 9309 semantics; `Options.SkipRobotsTxt` gated to test-only build tag in production; per-host cache reduces fetch overhead. |
| Playwright child process orphaning on crash | Medium | High | `os/exec.SysProcAttr.Setpgid = true`; SIGINT/SIGTERM handler calls Shutdown; documented in §6.10; REQ-CACHE-015 testable. |
| Memory blowout from 10MB body cap × 50 concurrent fetches | Medium | Medium | Default MaxBodyBytes=10MB; total memory bound = 500MB at 50 concurrent fetches; tunable via Options.MaxBodyBytes; future SPEC-CACHE-001-STREAM if streaming needed. |
| Playwright runtime install failure in CI | Medium | Medium | `Options.AutoInstallPlaywright = true` default; CI workflow runs `playwright.Install()` in TestMain for integration tests; PlaywrightEnabled=false default keeps unit tests independent. |
| TLS handshake errors misclassified as escalation triggers | Medium | Medium | `escalateTLS()` predicate inspects specific error types (`*tls.RecordHeaderError`, `tls.AlertError`); table tests in `escalation_test.go` cover. |
| WAF detection heuristic (cf-ray, x-akamai-) gives false positives | Medium | Low | `escalateJSChallenge()` requires both status code AND header pattern; conservative; if false positive, Phase 4 just runs unnecessarily (no functional harm). |
| robots.txt cache memory unbounded across million-host campaign | Low | Medium | sync.Map with TTL eviction; future SPEC-CACHE-001a may add LRU cap if production telemetry shows >100k hosts cached simultaneously. |
| Cache write-through Upsert overload under high traffic | Medium | Medium | Async fire-and-forget; failures logged WARN not surfaced; `Shutdown()` drains within 5s ctx; future SPEC-IDX-005 (M6) may add rate limiting on Upsert path. |
| Playwright version drift breaks API contract | Low | High | Run-phase pin to exact version v0.5700.1; CI integration test exercises `Run/Launch/Goto/Content/Close/Stop` end-to-end on every PR; security advisory subscribe required. |
| temoto/robotstxt parser bug on edge-case directives | Low | Medium | 285-star production-active library (research §7.4); fall-through to "allow all" on parse error mitigates; future migration to Google's reference parser fork is an option. |
| Phase 5 browser pool starvation under high concurrency | Medium | Medium | MaxBrowsers=2 default; OS-bound by Chromium memory (~150MB per browser); operators can tune via config but CPU/memory becomes the binding constraint at >8 browsers. |
| Cardinality allowlist amendment (`phase` + `blocked`) blocks NFR-OBS-002 | Low | Low | `phase ∈ 5 values`, `blocked` is one new outcome value; both are bounded enums. NFR-OBS-002 test already extended for IR/LLM/IDX additions; this delta is the same pattern. |
| Per-phase ctx leak (cancel never called) | Medium | High | `defer cancel()` immediately after `context.WithTimeout` in cascade; `go vet` enforces; NFR-CACHE-005 + goleak verification close the loop. |
| Goroutine leak on Playwright path with documented exclusions hides real leaks | Low | Medium | Exclusion list in NFR-CACHE-005 is narrowly scoped to two specific top-functions; non-Playwright leaks still detected; integration test reviews on each Playwright version bump. |

---

## 11. Open Questions

These are explicitly UNRESOLVED at SPEC-approval time. Each has a
recommended default and a one-line resolution owner. They do NOT block
SPEC approval.

1. **Caller wiring**: Does FAN-001 internally invoke CACHE-001 after
   each adapter Search, OR does the caller (CLI / future
   RetrieveOrchestrator) sequence `fanout.Dispatch` →
   `access.Fetch` per doc that needs body backfill? **Recommended
   default**: CALLER orchestrates. Keeps FAN-001 single-domain.
   Resolution owner: SPEC-CLI-002 / future SPEC-RETRIEVE-001 author.

2. **Playwright pool sizing**: Default `MaxBrowsers = 2` (~300 MB
   resident). Operators tune via `.moai/config/sections/access.yaml`.
   Resolution owner: future SPEC-EVAL-002 (M8) author may propose
   tuned defaults from production telemetry.

3. **Per-host rate limiting**: CACHE-001 v0.1 does NOT per-host
   rate-limit. **Recommended default**: add token-bucket per host in
   v0.2 (SPEC-CACHE-001a). Resolution owner: future SPEC-CACHE-001a
   author after M3 traffic analysis.

4. **5-phase numbering vs structure.md reservation**:
   `.moai/project/structure.md:30-34` reserves
   `phase0_index/phase1_probe/phase2_tls/phase3_browser` (4 files,
   matching insane-search original). CACHE-001 ships 5 phases (1-5)
   per the locked roadmap/product/research wording. **Recommended
   default**: CACHE-001 is authoritative; recommend a structure.md
   sync follow-up renaming to phase1..phase5 in next `/moai sync`.
   Resolution owner: docs-sync agent in next `/moai sync` pass.

5. **chromedp vs Playwright**: Playwright selected (research §4.1);
   chromedp rejected (research §4.2). Revisit if Node.js runtime
   operational cost becomes a deployment problem (container size
   review, supply-chain review). Resolution owner: future
   SPEC-DEPLOY-001 author.

6. **Cache write-through default**: OFF in v0.1 (`CacheWriteThrough
   = false`). Flip to ON when IDX-001 + IDX-002 are in production
   with Embedder wired. Resolution owner: SPEC-IDX-002 author at M3
   sync.

7. **Robots.txt TTL**: 24 h default. Trade-off between staleness and
   per-host fetch overhead. Resolution owner: future SPEC-CACHE-001a
   author after telemetry.

8. **TLS impersonation library**: v0.1 uses stdlib `crypto/tls.Config`
   with operator-tunable knobs. Original insane-search uses
   `curl_cffi` for fingerprint-level spoofing. v0.1 does NOT spoof
   fingerprints. If fingerprint-level bypass is needed, evaluate
   `github.com/refraction-networking/utls` or
   `github.com/Danny-Dasilva/CycleTLS`. Resolution owner: future
   SPEC-CACHE-001b author after a documented case where MinVersion
   tuning is insufficient.

---

## 12. References

### External (URL-cited; verified per research.md §9)

- `https://pkg.go.dev/github.com/playwright-community/playwright-go` —
  Playwright Go bindings v0.5700.1 (MIT). Quoted in research §4.1 +
  §7.1.
- `https://github.com/chromedp/chromedp` — chromedp v0.15.1 (rejected
  per research §4.2).
- `https://github.com/go-rod/rod` — rod v0.116.2 (rejected per
  research §4.3).
- `https://github.com/temoto/robotstxt` — robots.txt parser, MIT,
  production-active. Quoted in research §5.2 + §7.4.
- `https://github.com/jimsmart/grobotstxt` — Apache-2.0, last release
  March 2022 (rejected per research §7.6).
- `https://github.com/fivetaku/insane-search` — original 4-phase
  Python pattern reference, MIT. Quoted in research §2.1 + §7.5.
- `https://pkg.go.dev/net/http` — stdlib HTTP client (Client.Timeout,
  CheckRedirect, Transport fields). Quoted in research §7.
- `https://pkg.go.dev/crypto/tls` — TLS Config fields (ServerName,
  NextProtos, MinVersion, MaxVersion, CipherSuites,
  InsecureSkipVerify). Quoted in research §7.
- `https://playwright.dev/docs/intro` — Playwright overview.
- `https://www.robotstxt.org/robotstxt.html` — robots.txt protocol
  (HTTP 403 at fetch time; deferred to RFC 9309 reference; library
  implementation is the de-facto spec).
- RFC 9309 — Robots Exclusion Protocol (IETF Sept 2022).
- `https://qdrant.tech/documentation/concepts/collections/` — Qdrant
  multitenancy strategy (cited via SPEC-IDX-001).

### Internal (file:line cited)

- `.moai/specs/SPEC-CACHE-001/research.md` — full research artifact
  (this SPEC's research sibling).
- `.moai/specs/SPEC-CORE-001/spec.md` — Adapter / NormalizedDoc /
  SourceError contract.
- `.moai/specs/SPEC-FAN-001/spec.md:6` — FAN-001 status: approved.
- `.moai/specs/SPEC-FAN-001/spec.md:296` — REQ-FAN-001 Fanout/Dispatch
  contract.
- `.moai/specs/SPEC-FAN-001/spec.md:466-531` — §2.5 per-adapter
  timeout derivation (CACHE-001 §2.4 mirrors).
- `.moai/specs/SPEC-FAN-001/spec.md:535-572` — §2.6 worker state
  discipline (CACHE-001 §2.5 follows).
- `.moai/specs/SPEC-FAN-001/spec.md:581-608` — §2.7 caller deadline
  responsibility (CACHE-001 §2.7 mirrors).
- `.moai/specs/SPEC-FAN-001/spec.md:765-783` — TestDispatchConcurrent
  workload (CACHE-001 NFR-CACHE-004 mirrors).
- `.moai/specs/SPEC-FAN-001/spec.md:843-849` —
  `goleak.VerifyTestMain` pattern (CACHE-001 NFR-CACHE-005 reuses
  with playwright-go ignore list).
- `.moai/specs/SPEC-IDX-001/spec.md:6` — IDX-001 status: draft.
- `.moai/specs/SPEC-IDX-001/spec.md:546` — REQ-IDX-001 Index
  construction contract (CACHE-001 IndexLookup port).
- `.moai/specs/SPEC-IDX-001/spec.md:551` — REQ-IDX-006 Search
  contract (CACHE-001 Phase 1 consumes).
- `.moai/specs/SPEC-IDX-001/spec.md:556` — REQ-IDX-011 observability
  pattern (CACHE-001 mirrors).
- `.moai/specs/SPEC-OBS-001/spec.md:88-93` — REQ-OBS-001..006
  baseline collectors.
- `.moai/specs/SPEC-OBS-001/spec.md:101` — NFR-OBS-002 cardinality
  safety (CACHE-001 extends with `phase` + `blocked`).
- `.moai/specs/SPEC-ADP-001/spec.md:373-374` — REQ-ADP-011
  concurrent-safety contract precondition for NFR-CACHE-004.
- `pkg/types/normalized_doc.go:40-56` — NormalizedDoc 15-field
  struct; CACHE-001 returns FetchedContent that callers merge.
- `pkg/types/query.go:32-34` — Query.Deadline contract reference for
  caller-applies-deadline idiom.
- `internal/adapters/reddit/parse.go:160` — Reddit Body =
  `data.selftext` (snippet-only for link posts).
- `internal/adapters/hn/parse.go:124,143` — HN Body = HTML-stripped
  `story_text` (snippet-only for URL-only submissions).
- `internal/obs/metrics/metrics.go:33-65` — Registry shape.
- `internal/obs/metrics/metrics.go:169-176` — cardinality allowlist.
- `internal/llm/client.go:230-252` — observability emission pattern;
  CACHE-001 mirrors.
- `internal/router/router.go:341-401` — emit + nil-safe pattern;
  CACHE-001 emitFetch mirrors.
- `internal/index/index.go:1-3` — current 3-line stub (IDX-001
  context).
- `deploy/docker-compose.yml:31-216` — compose stack; CACHE-001 does
  NOT add a service in v0.1.
- `go.mod:3` — Go 1.25.8 baseline.
- `go.mod` (full) — existing direct deps; CACHE-001 adds two
  (`playwright-go`, `temoto/robotstxt`).
- `.moai/project/structure.md:30-34` — `internal/access/{phase0_index,
  phase1_probe,phase2_tls,phase3_browser}` reservation (4-file
  reservation; CACHE-001 ships 5 files per OQ §11.4 reconciliation).
- `.moai/project/structure.md:160` — `pkg/types` SDK boundary clause.
- `.moai/project/research.md:9` — fivetaku/insane-search reference.
- `.moai/project/research.md:74-78` — insane-search 5-phase
  reimplementation in Go.
- `.moai/project/research.md:91` —
  `https://github.com/fivetaku/insane-search`.
- `.moai/project/roadmap.md:58` — M3 row "SPEC-CACHE-001 | 5-phase
  access fallback".
- `.moai/project/roadmap.md:117-128` — M3 parallelization plan.
- `.moai/project/product.md:9` — "Access layer: inspired by
  fivetaku/insane-search".
- `.moai/project/product.md:93` — "insane-search — MIT (pattern
  reference only; not bundled)".
- `.moai/project/tech.md:49` — "Browser fallback: Playwright MCP per
  insane-search pattern".
- `.moai/project/tech.md:153` — "5-phase access fallback brittleness"
  risk row.
- `.moai/config/sections/quality.yaml` — `development_mode: tdd`,
  `test_coverage_target: 85`.
- `.moai/config/sections/harness.yaml` — auto-routing to standard
  level (with security flag).
- `.moai/config/sections/language.yaml` — `documentation: en`,
  `code_comments: en`.

---

*End of SPEC-CACHE-001 v0.1 (status: draft; pending plan-auditor cycle
deferred to M3 SPEC batch end)*
