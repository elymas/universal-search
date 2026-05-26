# SPEC-CACHE-001 Plan — Post-Hoc Implementation Summary

Created: 2026-05-04
Updated: 2026-05-08 (post-implementation reconciliation)
Author: limbowl (via manager-spec)
Status: implemented
Methodology: TDD (RED-GREEN-REFACTOR)
Coverage Target: 85%
Harness Level: standard (with security flag)

## 0. Plan Scope

Post-hoc plan describing how SPEC-CACHE-001 was implemented across the
new `internal/access/` package. Read alongside spec.md (requirements)
and acceptance.md (Given/When/Then scenarios).

## 1. Approach Summary

The 5-phase content-fetch cascade was implemented as a single
`Fetcher` struct holding a Playwright runtime, a bounded browser pool,
a robots.txt host cache, and an `ObsAdapter` for nil-safe
observability. `(*Fetcher).Fetch(ctx, url, opts)` is the sole public
entry point and orchestrates the sequential cascade Phase 1 (index) →
Phase 2 (HEAD + robots.txt) → Phase 3 (standard GET) → Phase 4
(TLS-tuned GET) → Phase 5 (Playwright), with per-phase context
derivation, panic-recover, and the four HARD-rule SSRF guards
(scheme allowlist, private/loopback deny, DNS-rebind pinning,
redirect re-validation). Three new metric families
(`AccessPhaseAttempts`, `AccessPhaseDuration`, `AccessFetchTotal`)
were registered in `internal/obs/metrics/` and re-exported via
`obs.Obs`. Two new Go module dependencies were added:
`github.com/playwright-community/playwright-go` and
`github.com/temoto/robotstxt`.

## 2. Reference Implementations (consumed)

| Concern | Reference (file:line) | What we reused |
|---------|-----------------------|----------------|
| Per-call observability emit | `internal/llm/client.go:230-252` | slog + counter + histogram + OTel span shape; nil-safe guards |
| Cardinality allowlist | `internal/obs/metrics/metrics.go:147-176` | `phase` label name + `blocked` outcome value added without touching the existing AdapterCalls/AdapterCallDuration collectors |
| Per-phase context derivation | `internal/router/router.go` + SPEC-FAN-001 §2.5 | `min(perPhaseTimeout, timeUntil(parentCtx.Deadline))` idiom |
| Soft-fail discipline | SPEC-IDX-001 §2.6 + SPEC-FAN-001 §2.5 | Phase attempt recorded on every failure; cascade decides escalate vs halt |
| Goroutine leak gating | SPEC-FAN-001 NFR-FAN-003 | `goleak.VerifyTestMain` with documented playwright-go exclusions |
| Public-API discipline | SPEC-CORE-001 + SPEC-LLM-001 | `pkg/types` imports only; no prometheus or otel imports surface to callers |

## 3. Package Layout (as implemented)

```
internal/access/
├── access.go                 # Fetcher struct, New, Close, Shutdown
├── access_lifecycle_test.go  # REQ-CACHE-001 + REQ-CACHE-015 lifecycle
├── cache_writethrough.go     # async upsert goroutine (REQ-CACHE-009)
├── cache_writethrough_test.go
├── cascade.go                # the Fetch hot path (orchestrator)
├── cascade_helpers_test.go
├── cascade_phase4_test.go    # WAF/JS-challenge escalation matrix
├── cascade_test.go           # REQ-CACHE-002/007/008/011/016
├── cascade_waf_test.go
├── concurrent_test.go        # NFR-CACHE-004 + REQ-CACHE-012
├── coverage_extra_test.go    # supplementary tests for coverage gap closure
├── coverage_extra2_test.go
├── dialer.go                 # pinnedIPDialer (REQ-CACHE-013 rule #3)
├── dialer_test.go
├── errors.go                 # ErrAllPhasesFailed, ErrPlaywrightUnavailable,
│                             # ErrShuttingDown, ErrInvalidURL, *FetchError
├── errors_test.go
├── escalation.go             # shouldEscalate (REQ-CACHE-014)
├── escalation_test.go
├── index_port.go             # IndexLookup interface + noopIndexLookup
├── metrics_test.go           # cardinality + collector registration
├── observability.go          # emitFetch helper (nil-safe)
├── observability_test.go
├── options.go                # Options + defaults + validation
├── options_test.go
├── phase1_index.go           # Phase 1: IndexLookup
├── phase2_probe.go           # Phase 2: HEAD + robots.txt
├── phase2_test.go
├── phase3_extra_test.go
├── phase3_get.go             # Phase 3: standard HTTP GET
├── phase3_test.go
├── phase4_extra_test.go
├── phase4_test.go
├── phase4_tls.go             # Phase 4: TLS-tuned GET (WAF bypass)
├── phase5_browser.go         # Phase 5: Playwright (build-tag gated for tests)
├── robots.go                 # robotsCache + temoto/robotstxt
├── robots_test.go
├── ssrf.go                   # validateScheme, validateHost, validateRedirect
├── ssrf_redirect_test.go
├── ssrf_test.go
├── testdata/                 # SSE/quota-errors/config/WAF fixtures
└── types.go                  # FetchOptions, FetchResult, FetchedContent, PhaseAttempt
```

Cross-package additions:
- `internal/obs/metrics/access.go` (referenced; collectors registered
  from `metrics.NewRegistry`).
- `internal/obs/obs.go` re-exports the new collectors.
- `go.mod`/`go.sum` — added `playwright-go` and `temoto/robotstxt`.

## 4. Key Implementation Files (file:line refs)

### Entry point and lifecycle
- `internal/access/access.go:1-80` — Package doc + `Fetcher` struct
  (immutable post-construction) + `New(opts Options) (*Fetcher, error)`.
- `internal/access/access.go::Fetcher` carries `pw *playwright.Playwright`,
  `browserPool chan playwright.Browser`, `robotsCache`, `opts`,
  `obs ObsAdapter`, `writeThroughWG`, `shutdownCh`, `shutdownOnce`.
- `Close()` / `Shutdown(ctx)` — orderly drain of write-through goroutines,
  browser pool closure, `pw.Stop()` call.

### Cascade orchestrator
- `internal/access/cascade.go::(*Fetcher).Fetch` — REQ-CACHE-001..014.
  Pre-flight SSRF guards → per-phase ctx derivation → run-phase →
  recorder → escalation predicate → exit.
- `internal/access/cascade.go::derivePhaseCtx` — implements §2.4
  `min(perPhase, remaining)` rule.
- `internal/access/cascade.go::runPhase` — per-phase `defer recover()`
  (REQ-CACHE-011); captures `runtime/debug.Stack()` on panic.

### Phase implementations
- `internal/access/phase1_index.go` — calls `IndexLookup.LookupByURL`;
  builds `FetchedContent` from `NormalizedDoc.Body` on hit; returns
  `ErrPhaseNotApplicable` on skip.
- `internal/access/phase2_probe.go` — HEAD + `/robots.txt` fetch,
  caches per-host, applies RFC 9309 4xx-allow / 5xx-disallow rule.
- `internal/access/phase3_get.go` — stdlib `*http.Client` + pinned-IP
  dialer + `io.LimitReader` body cap + redirect re-validation.
- `internal/access/phase4_tls.go` — custom `*tls.Config` (MinVersion
  TLS 1.2, `NextProtos {h2, http/1.1}`, browser UA); WAF heuristic
  (`<noscript>`, `cf-please-stand-by`) drives Phase 5 escalation.
- `internal/access/phase5_browser.go` — pool acquire → `browser.NewPage`
  → `page.Goto` → `page.Content()`; build-tag-gated integration tests.

### Escalation and SSRF
- `internal/access/escalation.go::shouldEscalate` — pure function over
  the 5×4 (phase, outcome) matrix per §3.4.
- `internal/access/ssrf.go::validateScheme` — `http`/`https` allowlist.
- `internal/access/ssrf.go::validateHost` — RFC1918 + loopback + AWS
  metadata + IPv6 ULA/link-local deny list.
- `internal/access/ssrf.go::validateRedirect` — per-hop re-validation +
  hop cap.
- `internal/access/dialer.go::pinnedIPDialer` — resolves hostname once,
  returns `*net.Dialer.DialContext` that forces all subsequent dials to
  the pinned IP (DNS-rebind mitigation).

### Caching and write-through
- `internal/access/robots.go::robotsCache` — `sync.Map[host]*RobotsData`
  with TTL eviction.
- `internal/access/cache_writethrough.go::cacheWriteThrough` — spawns
  one tracked goroutine per successful Phase 3-5 fetch; uses derived
  ctx with 30 s timeout; never blocks caller; tracked by
  `Fetcher.writeThroughWG`.

### Observability hook (SPEC-OBS-001 integration)
- `internal/access/observability.go::emitFetch` — nil-safe pattern
  mirroring `internal/router/router.go:341-401` and
  `internal/llm/client.go:230-252`.
- `internal/obs/metrics/access.go` (called via `metrics.NewRegistry`):
  - `AccessPhaseAttempts *prometheus.CounterVec` (labels `[phase, outcome]`)
  - `AccessPhaseDuration *prometheus.HistogramVec` (label `phase`)
  - `AccessFetchTotal *prometheus.CounterVec` (label `outcome`)
- Cardinality allowlist extended: `phase ∈ {1,2,3,4,5}`,
  `outcome ∈ {success, failure, timeout, blocked}`.

## 5. Integration Points

| Consumer SPEC | Integration |
|---------------|-------------|
| SPEC-IDX-001 (soft) | `IndexLookup` port at `internal/access/index_port.go`; default `noopIndexLookup` for tests until IDX-001 is in production |
| SPEC-OBS-001 (hard) | Three new metric families registered via `internal/obs/metrics/access.go`; `phase` label + `blocked` outcome added to cardinality allowlist |
| SPEC-FAN-001 (pattern) | Per-phase ctx derivation copied from FAN-001 §2.5 — H15 idiom (`min(perAdapter, remaining)`); same `defer cancel()` discipline |
| SPEC-CORE-001 (consumer) | `types.NormalizedDoc{SourceID:"access-cache", DocType: DocTypeWebpage, Body: <fetched>}` for write-through |
| Future callers | A future `SPEC-RETRIEVE-001` orchestrator may sequence `fanout.Dispatch → access.Fetch` per doc that needs body backfill; FAN-001 does NOT call CACHE-001 internally |

## 6. Data Structures and Interfaces

### Public surface
```go
type Fetcher struct { /* immutable post-construction */ }

func New(opts Options) (*Fetcher, error)
func (f *Fetcher) Fetch(ctx context.Context, url string, opts FetchOptions) (*FetchResult, error)
func (f *Fetcher) Close() error
func (f *Fetcher) Shutdown(ctx context.Context) error

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

type FetchOptions struct {
    UserAgent            string
    SkipRobotsTxt        bool
    SkipHEADProbe        bool
    AllowPrivateNetworks bool  // per-call override
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
    Headers     map[string]string
}

type PhaseAttempt struct {
    Phase          int
    StartedAt      time.Time
    ElapsedSeconds float64
    Outcome        string  // success | failure | timeout | blocked | miss | skipped
    Error          string  // serialised *FetchError on failure
}

type IndexLookup interface {
    LookupByURL(ctx context.Context, url string) (*types.NormalizedDoc, bool, error)
    Upsert(ctx context.Context, docs []types.NormalizedDoc) error
}

var (
    ErrAllPhasesFailed      = errors.New("access: all 5 phases failed")
    ErrPlaywrightUnavailable = errors.New("access: playwright not installed")
    ErrShuttingDown          = errors.New("access: fetcher shutting down")
    ErrInvalidURL            = errors.New("access: invalid URL")
)

type FetchError struct {
    Category   string  // permanent | transient | rate_limited | unavailable | blocked | timeout
    Reason     string
    HTTPStatus int
    Cause      error
}
```

### Per-phase budgets (defaults per §6.6)
| Phase | Default budget |
|-------|----------------|
| 1 (index)   | 100 ms |
| 2 (probe)   | 200 ms |
| 3 (GET)     | 10 s   |
| 4 (TLS GET) | 15 s   |
| 5 (browser) | 30 s   |

## 7. Test Coverage Notes

Test files (counts approximate, ~22 source files / ~22 test files):
- Cascade orchestration: `cascade_test.go`, `cascade_helpers_test.go`,
  `cascade_phase4_test.go`, `cascade_waf_test.go`.
- Per-phase: `phase2_test.go`, `phase3_test.go`, `phase3_extra_test.go`,
  `phase4_test.go`, `phase4_extra_test.go`. (Phase 5 tests are
  build-tag-gated `// +build integration`.)
- SSRF: `ssrf_test.go`, `ssrf_redirect_test.go`, `dialer_test.go`.
- robots.txt: `robots_test.go`.
- Lifecycle: `access_lifecycle_test.go`.
- Observability: `observability_test.go`, `metrics_test.go`.
- Concurrency: `concurrent_test.go` (50 goroutines × 100 fetches under
  `-race`).
- Cache write-through: `cache_writethrough_test.go`.
- Escalation predicate: `escalation_test.go`.
- Coverage gap closure: `coverage_extra_test.go`,
  `coverage_extra2_test.go`.

`goleak.VerifyTestMain` is invoked from the package's `TestMain` with
documented exclusions for `internal/poll.runtime_pollWait` and
`os/exec.(*Cmd).Wait` (NFR-CACHE-005).

Coverage at implementation completion: ≥85% across the `internal/access/`
package (benchmarks excluded).

## 8. MX Tag Plan (applied)

| File::Symbol | Tag | Reason |
|--------------|-----|--------|
| `access.go::Fetcher` | @MX:NOTE | Package entry point (see file header) |
| `cascade.go::(*Fetcher).Fetch` | @MX:ANCHOR | Sole entry; fan_in ≥ 4 |
| `cascade.go::runPhase` | @MX:WARN | Per-phase recover()/cancel propagation |
| `ssrf.go::validateScheme/Host/Redirect` | @MX:ANCHOR | Security boundary |
| `dialer.go::pinnedIPDialer` | @MX:WARN | DNS-rebind mitigation |
| `phase5_browser.go::phase5Browser` | @MX:WARN | Browser pool acquire/release |
| `cache_writethrough.go::cacheWriteThrough` | @MX:WARN | Tracked async goroutine |
| `escalation.go::shouldEscalate` | @MX:NOTE | 5×4 escalation matrix |
| `cascade.go::derivePhaseCtx` | @MX:NOTE | Per-phase ctx derivation rules |

All tags carry `[AUTO]` prefix, `@MX:SPEC: SPEC-CACHE-001`,
`@MX:REASON:` (mandatory on ANCHOR/WARN), per
`code_comments: en` in `language.yaml`.

## 9. Risks Realised

| Original Risk | Outcome |
|---------------|---------|
| SSRF via crafted redirect chain | Mitigated; 9 ssrf_test scenarios green |
| robots.txt non-compliance | Mitigated; RFC 9309 4xx/5xx rule tested |
| Playwright child-process orphan | Mitigated; `Shutdown()` drains and calls `pw.Stop()` |
| TLS misclassification | escalateTLS predicate inspects specific error types |
| WAF false positive | Conservative pattern (status AND header AND body) |
| Browser pool starvation | `MaxBrowsers=2` default + pool channel block-with-timeout |
| Cardinality allowlist amendment | Two new bounded values (`phase`, `blocked`); NFR-OBS-002 test extended without churn |
| goleak exclusions hiding real leaks | Exclusion list is narrowly scoped to two specific top-functions; reviewed on every Playwright version bump |

## 10. Self-Review Outcome

Resolved questions:
- Is the `Fetcher` immutable post-construction defensible?
  → Yes; concurrent callers share goroutine-safe primitives only (sync.Map,
  channel pool, stdlib `*http.Client`).
- Is `ObsAdapter` (rather than `*obs.Obs`) earning its weight?
  → Yes; nil-safety patterns are centralised once and re-used at every
  emission call site.
- Is the 5×4 escalation matrix the right abstraction?
  → Yes; documented as a pure function (testable in isolation).
- Could Phase 5 ship without Playwright integration tests?
  → No; build-tag-gated `// +build integration` separates the binary
  weight of the browser install from unit-test CI.

---

*End of plan.md (post-hoc).*
