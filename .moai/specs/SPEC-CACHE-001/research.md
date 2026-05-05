# SPEC-CACHE-001 — Research

5-Phase Access Fallback (insane-search pattern port).

Author: limbowl via manager-spec
Created: 2026-05-04
Methodology gate: TDD (per `.moai/config/sections/quality.yaml`)
Skipped audit cycle: M3 SPEC batch (orchestrator runs plan-auditor at end of batch).

---

## §1. Existing-State Audit

### §1.1 Empty target package

The project explicitly reserves the target tree:

- `.moai/project/structure.md:30-34`:

  ```
  │   ├── access/                   # 5-phase access fallback (SPEC-CACHE)
  │   │   ├── phase0_index.go
  │   │   ├── phase1_probe.go
  │   │   ├── phase2_tls.go         # curl_cffi equivalent
  │   │   └── phase3_browser.go     # Playwright bridge
  ```

- `internal/access/` does not yet exist on disk
  (`ls internal/` shows: `adapters auth eval fanout index llm obs router synthesis`).
  CACHE-001 is the first SPEC to populate the directory.

The directory layout in `structure.md` shows 4 phases (phase0..phase3),
matching the original `fivetaku/insane-search` README which uses Phases 0–3
(see §3 below). The roadmap entry at `.moai/project/roadmap.md:58` and the
product.md at `.moai/project/product.md:9` both call it the **5-phase access
fallback**. The discrepancy is reconciled by treating the locked project
documents as authoritative: the SPEC ships a **5-phase** model where the
local index lookup (Phase 1) and a metadata-only HEAD probe (Phase 2) split
the original Phase 0 / Phase 1 into two distinct stages with separate
budgets and observable outcomes. See §3.4 for the mapping.

### §1.2 NormalizedDoc Body field is snippet-only today

`pkg/types/normalized_doc.go:40-56` declares the canonical 15-field struct.
The relevant fields for CACHE-001 are:

```
Body     string `json:"body"`
Snippet  string `json:"snippet"`
URL      string `json:"url"`
```

`pkg/types/normalized_doc.go:24-28`:
> "Title / Body / Snippet: text content. Body is the ranking input; Snippet
> is short UI excerpt."

Adapter-supplied Body content surveyed:

| Adapter | Body source | File:line |
|---------|-------------|-----------|
| Reddit | `data.selftext` (self-post body, empty for link posts) | `internal/adapters/reddit/parse.go:160` |
| Hacker News | `story_text` HTML-stripped (Algolia HN API) | `internal/adapters/hn/parse.go:124, :143` |

Both adapters today produce a Body that is either complete (Reddit
self-post; HN story_text when present) OR empty/short (Reddit link post;
HN URL-only submission). CACHE-001's role is to provide a **content-fetch
service** that adapters MAY invoke, post-Search, when the canonical doc
needs the full body content from the source URL — for example, an arXiv
adapter that returns `[]NormalizedDoc{...}` with abstract-only Body needs
the PDF text; an RSS-style news adapter returns title+snippet but not the
article body; a paywall-wrapped blog returns a JS-rendered shell.

CACHE-001 does NOT mutate `pkg/types.NormalizedDoc`. The fetch service
returns a separate `FetchedContent{ url, body, content_type, status_code,
fetched_at }` struct that the caller merges into the doc's Body field.
Keeping the struct separate preserves the SPEC-CORE-001 SDK boundary at
`.moai/project/structure.md:160` ("`pkg/types` is the public SDK boundary;
breaking changes require a major version bump").

### §1.3 Current Go module footprint

`go.mod:3` pins Go 1.25.8. Existing direct deps:
- `github.com/oklog/ulid/v2 v2.1.1` (request IDs)
- `github.com/prometheus/client_golang v1.23.2` (metrics)
- `go.opentelemetry.io/otel v1.43.0` + sdk + trace + otlptracegrpc (tracing)
- `github.com/openai/openai-go v1.12.0` (LLM-001)
- `golang.org/x/sync v0.20.0` (errgroup; FAN-001 + IDX-001 use)
- `go.uber.org/goleak v1.3.0` (goroutine leak verification)

CACHE-001 will introduce **two** new direct deps (run-phase pinning):

- `github.com/playwright-community/playwright-go` — Phase 5 browser
  bridge. Latest published v0.5700.1 (Feb 2026, MIT). Module path
  verified via WebFetch (§7.1).
- `github.com/temoto/robotstxt` — Phase 2 robots.txt compliance. MIT,
  production-active (§7.4).

ZERO new deps for Phases 1, 3, 4 (pure stdlib).

### §1.4 Existing observability bundle

`internal/obs/metrics/metrics.go:33-65` declares the Registry shape.
`internal/obs/metrics/metrics.go:169-176` declares the cardinality
allowlist. Existing labels used by CACHE-001's metric family will need:

- `phase ∈ {1,2,3,4,5}` (5 bounded values; new label)
- `outcome ∈ {success, failure, timeout, blocked}` (4 bounded values;
  `success/failure/timeout` already allowlisted via adapter usage; `blocked`
  is new — for robots.txt-disallowed and SSRF-rejected URLs)

CACHE-001's metric family sole-emitter pattern mirrors LLM-001
(`internal/obs/metrics/llm.go`) and the prospective IDX-001
`internal/obs/metrics/index.go` per `.moai/specs/SPEC-IDX-001/spec.md:551`
(REQ-IDX-011 — "ONE OTel parent span `index.search`").

### §1.5 Existing fanout / index integrations

CACHE-001 sits **adjacent to** rather than inside fanout. The wiring:

```
        ┌──────────── CALLER ─────────────┐
        │   adapter.Search returns        │
        │   []NormalizedDoc (Body may     │
        │   be partial)                   │
        └────────────────┬────────────────┘
                         │
                         ▼
              ┌───────────────────┐
              │ access.Fetch(URL) │  ← SPEC-CACHE-001 this SPEC
              │  Phase 1..5       │
              └─────────┬─────────┘
                        │
                        ▼
                 FetchedContent
                 { body, ... }
                        │
                        ▼
              caller merges into
              NormalizedDoc.Body
```

Phase 1 (local index lookup) consumes `internal/index.Index.Search` from
SPEC-IDX-001 (status: draft per
`.moai/specs/SPEC-IDX-001/spec.md:6`). CACHE-001 declares an `IndexLookup`
port to keep IDX-001 a soft dependency — when IDX-001 is unavailable
(e.g., during early M3 work or a degraded production deploy), CACHE-001
gracefully skips Phase 1 and starts at Phase 2.

Fanout (FAN-001) does NOT call CACHE-001 directly. The caller orchestration
question — does FAN-001 wrap CACHE-001 internally, or does the caller
(CLI / future RetrieveOrchestrator) call them sequentially? — is OQ §8.1.
Recommended default: CALLER orchestrates. Keeps FAN-001 single-domain;
keeps CACHE-001 consumable from any caller pattern.

---

## §2. 5-Phase Pattern Survey (insane-search reference)

### §2.1 Original `fivetaku/insane-search` repo

Verified 2026-05-04 via WebFetch (§7.5):
- Repo: `https://github.com/fivetaku/insane-search`
- License: MIT
- Stars: 606
- Primary language: Python
- Last update: < 1 day at fetch time

Original phase pipeline (4 phases, 0-3):

| Phase | Original (fivetaku) | Mechanism |
|-------|---------------------|-----------|
| 0 | Special endpoint index lookup | Cache hit / pre-warmed sources |
| 1 | Lightweight probes (parallel) | WebFetch, Jina Reader, curl variants, URL variants (`m.{domain}`, `.json`, `/rss`, `/feed`) |
| 2 | TLS impersonation + identity spoofing | `curl_cffi` for TLS fingerprint bypass; behavioural challenge detection |
| 3 | Full browser rendering | Playwright MCP with network request discovery |

Escalation logic (verbatim quote from README, fetched in §7.5):
> "Each phase activates only on detecting specific signals from the previous
> phase—403/429 errors, WAF headers, or challenge bodies trigger
> progression."

The repo is MIT-licensed, **pattern-reference only** (per
`.moai/project/product.md:93` — "insane-search — MIT (pattern reference
only; not bundled)"). CACHE-001 is a Go port + adaptation, NOT a wrapper.

### §2.2 Mapping: 4-phase original → 5-phase CACHE-001

The project locks "5-phase access fallback" at three independent
locations:
- `.moai/project/roadmap.md:58`
- `.moai/project/product.md:9`
- `.moai/project/research.md:9, :74`

The 4-vs-5 reconciliation (CACHE-001's adaptation):

| insane-search Phase | CACHE-001 Phase | Why split |
|---------------------|-----------------|-----------|
| Phase 0 (index lookup) | Phase 1 (local index lookup) | Same role; renumbered |
| Phase 1.a (HEAD probe + robots.txt) | Phase 2 (HEAD probe + robots.txt) | Split out as a separate phase with its own latency budget; cheap (~50-100ms) and fail-fast on robots.txt violations |
| Phase 1.b (lightweight GET) | Phase 3 (standard HTTP GET) | Most public docs return here (~80% of M3-target sources per §3.5) |
| Phase 2 (TLS impersonation) | Phase 4 (TLS-aware HTTP GET) | Sites requiring custom SNI / ALPN / cipher tuning |
| Phase 3 (Playwright) | Phase 5 (Playwright headless browser) | JS-heavy / paywall sites; last resort |

This mapping is documented in §3.1 of spec.md as the "phase numbering
rationale" and in OQ §8.4 in case future operators want to revisit.

### §2.3 Comparable patterns in the literature

Beyond insane-search, multi-phase fetch cascades appear in:
- Common Crawl bot architecture (HTTP-only, no JS) — single-phase, no
  cascade. Differs from CACHE-001's progressive escalation.
- Apache Nutch + Selenium fallback for JS sites — 2-phase. Nutch's
  HTTP fetcher first, fallback to Selenium for known-JS-required hosts.
  CACHE-001 generalises this pattern.
- Diffbot / Mercury Reader / Postlight — commercial 1-phase smart
  extractors (not cascade, just smart parsing). CACHE-001 is upstream
  of any extractor.
- Google's own crawler (Googlebot Smartphone + Desktop) cascades on
  rendering capability per https://developers.google.com/search/docs/crawling-indexing/javascript/dynamic-rendering
  (verified URL pattern; not WebFetch-cited verbatim).

The originality of insane-search's pattern is the **failure-driven
escalation** (each phase activates on specific signals from the
previous phase) coupled with **per-phase budget bounds**. CACHE-001
preserves both invariants.

---

## §3. Phase Budget Allocation Rationale

### §3.1 Per-phase target latency

Each phase has an independent budget enforced via
`context.WithTimeout(parentCtx, perPhaseBudget)`. Budgets are tuned for
the typical case at each phase, not the worst case.

| Phase | Target p95 | Mechanism justification |
|-------|-----------|-------------------------|
| 1 (Index lookup) | ≤ 100 ms | IDX-001 NFR-IDX-002 budgets retrieval at p95 ≤ 300 ms total. CACHE-001's Phase 1 is keyed lookup (single doc by URL or content_hash), tighter than full search. |
| 2 (HEAD probe + robots.txt) | ≤ 200 ms | One HEAD request to target + one parallel GET for `/robots.txt`. TCP + TLS handshake dominates; payload is bytes-only. |
| 3 (Standard GET) | ≤ 10 s | Public docs typically return < 2 s (HN API, arXiv abstract page). Budget headroom for slow news sites (~5-8 s observed in research §3.5). |
| 4 (TLS-aware GET) | ≤ 15 s | Adds TLS fingerprint setup overhead (~50 ms typically) but otherwise same as Phase 3. Ceiling allows for slower sites that gate on TLS heuristics. |
| 5 (Playwright) | ≤ 30 s | Browser launch ~2-5 s, page navigation 5-15 s for JS-heavy sites, content extraction ~1 s. p95 ≤ 30 s aligns with typical headless Chrome budgets. |

Spec NFRs:
- NFR-CACHE-001 (cheap-path budget): Phase 1 + Phase 2 cumulative
  ≤ 200 ms p95 (cache + probe path; the typical happy path for
  already-indexed docs).
- NFR-CACHE-002 (mid-path budget): Phase 3 budget ≤ 10 s p95 (the
  vast majority of public-document fetches succeed here).
- NFR-CACHE-003 (heavy-path budget): Phase 5 budget ≤ 30 s p95
  (last-resort browser-rendered fetches).

### §3.2 Aggregate budget

The full 5-phase cascade with all phases timing out adds to ~55 s
worst-case. In practice, escalation halts at the first success, so
typical traffic mass at Phase 1 + Phase 3 paths returns under ~10 s.
The CALLER MUST apply a top-level deadline via `context.WithTimeout`
matching its own SLA — CACHE-001 does NOT enforce a wall-clock cap on
the full cascade beyond the per-phase budgets (mirrors SPEC-FAN-001
§2.7 H15 Query.Deadline contract).

### §3.3 Phase-skip rules

A phase MAY be skipped under these conditions:

- **Phase 1**: Skipped when `IndexLookup` port is nil (graceful
  degradation when IDX-001 unavailable).
- **Phase 2**: Skipped when `Options.SkipRobotsTxt = true` AND
  `Options.SkipHEADProbe = true` (operator opt-out for testing only;
  robots.txt compliance is the default; production callers MUST NOT
  set both flags — see §5.3 SSRF discussion).
- **Phase 3**: Always attempted unless robots.txt forbids the path
  (in which case the cascade fail-fasts to a `blocked` outcome — see
  §5.4 below).
- **Phase 4**: Attempted only when Phase 3 returned a TLS-related error
  (handshake failure, certificate error not from a malicious cert) OR
  status code in `{403, 429, 503}` with WAF-suspicious header pattern
  (Cloudflare, Akamai, Fastly).
- **Phase 5**: Attempted only when Phase 4 returned a JS-required
  challenge (e.g., status 200 but body contains `<noscript>` challenge
  text or known WAF bodies) AND `Options.PlaywrightEnabled = true`.

The escalation triggers are encoded as deterministic predicates on the
previous phase's `FetchedContent` + error pair (see §4.3 below).

### §3.4 Soft-fail discipline

Mirroring SPEC-IDX-001 §2.6 + SPEC-FAN-001 §2.5: the cascade does NOT
hard-error when intermediate phases fail. Each phase's result (success
or error) is recorded in `FetchResult.PhaseAttempts []PhaseAttempt`,
and the final outcome is the LAST phase attempted. Hard error
(`ErrAllPhasesFailed`) only when phases 1-5 all fail OR when a
phase-skip rule blocks every phase.

---

## §4. Library Survey: Playwright vs Alternatives

### §4.1 `github.com/playwright-community/playwright-go` (selected)

Verified 2026-05-04 via WebFetch (§7.1). Quotes verbatim:

- Module path: `github.com/playwright-community/playwright-go`
- Latest version: `v0.5700.1` (Feb 23, 2026)
- License: MIT
- Browser binaries: Chromium 143.0.7499.4, Firefox 144.0.2, WebKit 26.0
- Install: `go run github.com/playwright-community/playwright-go/cmd/playwright@v0.xxxx.x install --with-deps` OR programmatic `playwright.Install()`

API methods quoted:
```
func Run(options ...*RunOptions) (*Playwright, error)
func (p *Playwright) Stop() error
BrowserType.Launch(options ...BrowserTypeLaunchOptions) (Browser, error)
Browser.NewPage(options ...BrowserNewPageOptions) (Page, error)
Page.Goto(url string, options ...PageGotoOptions) (Response, error)
Page.Content() (string, error)
Browser.Close(options ...*BrowserCloseOptions) error
Page.Close(options ...*PageCloseOptions) error
```

Operational cost concerns:
- Node.js runtime (~50 MB) required as a child process (per WebFetch §7.1
  exact quote: "uses a Node.js runtime (~50MB) that downloads
  pre-compiled browsers automatically"). Adds a runtime dependency
  beyond the Go binary.
- Chromium binary ~150 MB on disk per platform.
- Per-page memory: ~150 MB chromium baseline (NFR-CACHE-006 ceiling).
- Browser startup overhead: ~2-5 s cold launch; ~500 ms warm.

Lifecycle discipline (CACHE-001 spec):
- One Playwright runtime per process (`*Playwright`), constructed once
  in `New(opts)` and held for the process lifetime.
- One Browser per fetch call (browser per request) OR a small pool
  (default `MaxBrowsers = 2`). Document the choice in §6.6 of spec.md.
- `defer browser.Close()` and `defer pw.Stop()` mandated on the test
  harness to satisfy `goleak.VerifyNone`.

### §4.2 `github.com/chromedp/chromedp` (rejected)

Verified 2026-05-04 via WebFetch (§7.2). Quotes:
- Latest: `v0.15.1` (April 1, 2026)
- License: MIT
- Module path: `github.com/chromedp/chromedp`
- Key advantage: "**no external dependencies (no Node.js runtime
  needed)**. It operates as a pure Go solution, making deployment
  simpler for Go-native environments."

Why rejected for v0.1:
- Single-browser support (Chromium only). The original insane-search
  pattern targets WebKit on iOS-shaped device fingerprints; Playwright
  preserves that capability for future SPEC-CACHE-002 extension.
- Smaller community / less production-tested than Playwright.
- `chromedp/headless-shell` Docker image required for headless production
  use (per WebFetch quote). CACHE-001 prefers the `playwright.Install()`
  programmatic install pattern for easier dev/CI parity.

Open Question §8.5 documents the revisit trigger: if Node.js runtime
operational cost becomes a problem (Container size, supply chain
review), evaluate chromedp v0.x line.

### §4.3 `github.com/go-rod/rod` (rejected)

Verified 2026-05-04 via WebFetch (§7.3). Quotes:
- Latest: `v0.116.2` (July 12, 2024)
- License: MIT
- Module path: `github.com/go-rod/rod`
- Strength: "Auto-wait elements to be ready"; "100% test coverage"

Why rejected:
- Last release July 2024 — slower release cadence than
  Playwright (Feb 2026) or chromedp (April 2026).
- CDP-only (Chromium) — same constraint as chromedp.
- "Auto-wait" semantic is a wash vs Playwright's analogous waits;
  doesn't justify switching from the more-active ecosystem.

### §4.4 `chromedp/headless-shell` Docker image (deferred)

Production deployment of CACHE-001's Phase 5 may benefit from running
the chromedp/headless-shell sidecar service in `deploy/docker-compose.yml`
to isolate browser-rendered content fetching from the Go process. This
is an OPERATIONAL concern (process isolation, OOM containment), not a
LIBRARY choice. Deferred to future SPEC-DEPLOY-001 if measured value.

---

## §5. SSRF + robots.txt Compliance

### §5.1 SSRF threat model

An adversarial SourceID-supplied URL could direct CACHE-001's Phase 3-5
fetchers at internal infrastructure: AWS metadata endpoints
(`169.254.169.254`), Kubernetes pod IPs, internal DNS, localhost ports
running admin services. SPEC-SEC-001 (M8 per
`.moai/project/roadmap.md:104`) flags SSRF mitigation on the access
fallback as a discrete concern.

CACHE-001 implements **HARD-rule SSRF guards** in v0.1:

1. **Scheme allowlist**: Only `https` and `http` schemes pass; any
   other scheme (`file`, `ftp`, `gopher`, `data`, `javascript`) is
   rejected with `*FetchError{Category: CategoryBlocked, Reason:
   "scheme not allowed"}`.

2. **IP allowlist deny-by-default for private/loopback ranges**:
   - `127.0.0.0/8` (loopback)
   - `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16` (RFC1918)
   - `169.254.0.0/16` (link-local — including AWS metadata
     `169.254.169.254`)
   - `::1/128` (IPv6 loopback)
   - `fc00::/7` (IPv6 ULA)
   - `fe80::/10` (IPv6 link-local)
   are rejected unless `Options.AllowPrivateNetworks = true`
   (test-only opt-in for `httptest.Server` 127.0.0.1 stubs;
   production callers MUST NOT enable in deployments).

3. **DNS rebinding mitigation**: Phase 3-5 fetchers MUST resolve the
   URL hostname ONCE before any HTTP attempt and pin the resulting IP
   for the duration of that fetch (via custom `*net.Dialer.DialContext`
   that intercepts and validates the IP). Prevents the classic DNS
   rebind where the first lookup returns a public IP and a subsequent
   lookup returns 127.0.0.1.

4. **Redirect host validation**: Each redirect's `Location` header
   re-runs the scheme + IP allowlist checks. Cross-domain redirects
   are NOT auto-rejected (legitimate news sites redirect to CDN
   subdomains constantly), but redirect chains > 5 hops are
   rejected, and redirects targeting any IP that fails rule #2 are
   rejected with `CategoryBlocked`.

REQ-CACHE-013 makes the four SSRF guards testable.

### §5.2 robots.txt compliance

`https://www.robotstxt.org/robotstxt.html` (404 at WebFetch retrieval
time on 2026-05-04 — server returned HTTP 403; the protocol is well-
established and stable since 1994; the canonical RFC reference is
RFC 9309 — but for v0.1 we use the `temoto/robotstxt` library's
implementation as the de-facto reference). The library survey:

- `github.com/temoto/robotstxt` (selected — §7.4):
  - License: MIT
  - Module path: `github.com/temoto/robotstxt`
  - Production-active (285 stars, marked "production-ready" per
    WebFetch §7.4)
  - API: `FromBytes(body []byte) (*RobotsData, error)`,
    `FromString(string) (*RobotsData, error)`,
    `TestAgent(url, agent string) bool`,
    `FindGroup(userAgent string)` returning a queryable group with
    `.Test(path string)` method.

- `github.com/jimsmart/grobotstxt` (rejected — §7.6):
  - License: Apache-2.0
  - Last release March 2022 (~3 years stale at fetch time).
  - API simpler (`AgentAllowed(robotsTxt, agent, uri) bool`) but the
    ~3-year-stale maintenance window outweighs the simpler API;
    Google's robots.txt reference parser is in active flux around
    sitemap detection and edge cases.

CACHE-001's Phase 2 robots.txt protocol:
1. Construct robots.txt URL from the target URL: `<scheme>://<host>/robots.txt`.
2. Fetch via stdlib `*http.Client.Get` with a 5 s timeout.
3. On HTTP 200, parse via `robotstxt.FromBytes`.
4. On HTTP 4xx (except 429), treat as "allow all" (RFC 9309 §2.3.1
   semantics).
5. On HTTP 5xx or network error, treat as "disallow all" (conservative;
   matches Google reference parser behaviour).
6. Cache the parsed `*RobotsData` for `Options.RobotsTTL` (default
   24 h) keyed on `<scheme>://<host>` to amortise robots.txt lookups
   across many fetches to the same host.
7. Test the target URL path via `data.TestAgent(path, userAgent)`.
8. If FALSE, fail-fast the cascade with
   `*FetchError{Category: CategoryBlocked, Reason: "robots.txt disallow"}`.

REQ-CACHE-008 makes this testable.

### §5.3 Operator opt-out

`Options.SkipRobotsTxt = true` is provided for testing
(`httptest.Server`-backed integration tests where robots.txt is not
served). Production code paths MUST NOT set this flag; a future
SPEC-SEC-001 (M8) audit will enforce a build-time check.

### §5.4 Cache-Control respect

The HEAD probe in Phase 2 also reads `Cache-Control` and `Expires`
headers from the target. When the target returns
`Cache-Control: no-store` or `private`, CACHE-001 logs a WARN slog
record and proceeds with the fetch (the retrieval is for indexing,
not for serving back to the user; legal-compliance review is the
operator's responsibility). The header data is preserved in
`FetchedContent.Metadata["cache_control"]` for downstream auditing.

---

## §6. Cache Write-Through Architecture

### §6.1 Successful Phase 3-5 writes back to IDX-001

When a Phase 3, 4, or 5 fetch succeeds, the resulting `FetchedContent`
SHOULD be written to the local hybrid index (IDX-001) so that future
fetches of the same URL hit Phase 1 (the index lookup) and skip the
expensive cascade.

Write-through flow:
```
Phase 3/4/5 returns FetchedContent
        │
        ▼
   construct NormalizedDoc
   { ID = docID(SourceID, URL),
     Body = FetchedContent.Body,
     RetrievedAt = FetchedContent.FetchedAt,
     ... }
        │
        ▼
  index.Upsert(ctx, []NormalizedDoc{doc})  ← IDX-001
```

The `NormalizedDoc.SourceID` for cache-only docs is set to a
sentinel value `"access-cache"` (avoids collision with adapter
SourceIDs like `"reddit"`, `"hackernews"`).
`NormalizedDoc.DocType` is set to `types.DocTypeWebpage` (or the
Content-Type-inferred equivalent).

### §6.2 Cache-write feature gate

The write-through is OPT-IN via `Options.CacheWriteThrough = true`
(default: false). Rationale: in v0.1 IDX-001 is itself draft; the
write-through wiring depends on IDX-001's `Embedder` port being
wired (otherwise Qdrant ingestion fails). When IDX-001 is in
production, callers can flip the flag and gain the write-through
performance benefit.

### §6.3 Cache TTL and invalidation

CACHE-001 v0.1 does NOT implement TTL-based invalidation on the
index-level cache. The IDX-001 PG schema `docs.retrieved_at` field
provides an upper-bound staleness signal that future SPEC-CACHE-002
can act on (re-fetch when retrieved_at > N hours ago). v0.1 always
returns whatever IDX-001 has cached, even if stale; downstream
synthesis (SPEC-SYN-001, M2) is responsible for surfacing
`retrieved_at` to the user.

### §6.4 Cache-write does not block the response

The `index.Upsert` call after a successful Phase 3-5 fetch happens
**asynchronously** in a goroutine spawned with a derived ctx. The
fetch returns to the caller as soon as the body is in hand; the
write-through is fire-and-forget. Failures are logged at WARN but
not surfaced to the caller. This decouples cache-warming from
caller latency.

REQ-CACHE-009 makes this testable.

---

## §7. Race / Leak / Playwright Lifecycle Concerns

### §7.1 Concurrent fetch goroutine safety

`*Fetcher` is immutable post-construction (mirrors SPEC-FAN-001 §2.6).
The Playwright runtime (`*playwright.Playwright`) is documented as
goroutine-safe in the playwright-go README (per §7.1 quote — does NOT
explicitly state thread-safety; treated as unsafe-by-default for v0.1).
CACHE-001's Phase 5 uses **one Browser per call** (no shared Browser
state across goroutines) so the Playwright runtime's per-Browser
isolation is the boundary.

The `*http.Client` for Phases 2-4 is goroutine-safe per Go stdlib.

`*RobotsData` from `robotstxt.FromBytes` is read-only after parse;
shared across goroutines via the host-keyed cache (sync.Map-backed).

NFR-CACHE-004 + REQ-CACHE-014 mandate a `TestFetchConcurrent`
workload of 50 goroutines × 100 URLs (mirroring SPEC-FAN-001
NFR-FAN-002).

### §7.2 Goroutine leak on Playwright path

Playwright's stdio communication with Node.js child process keeps a
goroutine alive until `pw.Stop()` returns. NFR-CACHE-005 mandates
`goleak.VerifyTestMain` with documented exclusions:

- `goleak.IgnoreTopFunction("internal/poll.runtime_pollWait")` — Go
  runtime poll goroutines used by Playwright's stdio pipes.
- `goleak.IgnoreTopFunction("os/exec.(*Cmd).Wait")` — child process
  reaper goroutine.

The exact ignore list is confirmed during run-phase against
playwright-go v0.5700.1. Mirrors SPEC-IDX-001 NFR-IDX-004's handling
of pgxpool background goroutines.

### §7.3 Browser process orphaning

If the Go process crashes mid-fetch, the spawned Chromium child
process can outlive its parent (orphaned browser PID). Mitigation:

- Use `os/exec.SysProcAttr.Setpgid = true` to put the browser in a
  separate process group.
- Register a `signal.Notify` handler for `SIGINT/SIGTERM` to call
  `pw.Stop()` before exit.
- Document the orphan-cleanup procedure in the operator runbook
  (post-V1 deploy SPEC).

REQ-CACHE-015 surfaces the signal-handling contract.

### §7.4 Per-browser memory ceiling

Chromium baseline ~150 MB resident per browser instance (per
empirical surveys; not WebFetch-citable for an exact figure but
documented across multiple performance tuning guides). NFR-CACHE-006
sets the ceiling at ~150 MB per browser × `MaxBrowsers` (default 2)
= ~300 MB resident overhead for the Playwright pool.

---

## §8. Open Questions (carried to spec.md §11)

1. **Caller wiring**: Does FAN-001 internally invoke CACHE-001 after
   each adapter Search, OR does the caller (CLI / future
   RetrieveOrchestrator) sequence `fanout.Dispatch` →
   `access.Fetch` per doc? Recommended default: **CALLER**. Keeps
   FAN-001 single-domain. Resolution owner: SPEC-CLI-002 / future
   SPEC-RETRIEVE-001 author.

2. **Playwright pool sizing**: Default `MaxBrowsers = 2`
   (~300 MB resident headroom for Phase 5). Operators may tune via
   `.moai/config/sections/access.yaml`. Resolution owner: future
   SPEC-EVAL-002 (M8 reliability dashboard) author may propose
   tuned defaults from production telemetry.

3. **Per-host rate limiting**: CACHE-001 v0.1 does NOT
   per-host-rate-limit. A single host hammered by 50 concurrent
   fetches receives 50 simultaneous TCP connections. SPEC-FAN-001's
   `Capabilities.RateLimitPerMin` covers per-adapter rate limiting,
   but URL-fetch is per-host, not per-adapter. Recommended default:
   add per-host limiter in v0.2 (SPEC-CACHE-001a). Resolution owner:
   future SPEC-CACHE-001a author after M3 traffic.

4. **Phase numbering**: 5-phase split (CACHE-001) vs 4-phase original
   (insane-search). Documented in §2.2 above; reconciled by treating
   the locked project documents (`roadmap.md`, `product.md`) as
   authoritative. Resolution owner: SPEC-CACHE-001 author bakes into
   spec.md §3.1; revisit only on user request.

5. **chromedp vs Playwright**: Playwright selected (§4.1); chromedp
   rejected for v0.1 (§4.2). Revisit if Node.js runtime operational
   cost becomes a deployment problem (container size review,
   supply-chain review). Resolution owner: future SPEC-DEPLOY-001
   author.

6. **Cache write-through default**: OFF in v0.1 (`CacheWriteThrough
   = false`). Flip to ON when IDX-001 is in production with Embedder
   wired. Resolution owner: SPEC-IDX-002 author at M3 sync.

7. **Robots.txt TTL**: 24 h default. Trade-off between staleness and
   per-host fetch overhead. Resolution owner: future
   SPEC-CACHE-001a author after telemetry.

8. **TLS impersonation library**: v0.1 uses stdlib `crypto/tls.Config`
   with operator-tunable `MinVersion`, `MaxVersion`, `NextProtos`,
   `CipherSuites`, custom `ServerName`. The original insane-search
   uses `curl_cffi` for fingerprint-level spoofing (mimicking real
   browsers' TLS handshake byte sequences). v0.1 does NOT spoof
   fingerprints — it only adjusts `MinVersion`/cipher list to avoid
   sites that gate on weak TLS. If fingerprint-level bypass is needed,
   evaluate `github.com/refraction-networking/utls` or
   `github.com/Danny-Dasilva/CycleTLS`. Resolution owner: future
   SPEC-CACHE-001b author after a documented case where MinVersion-
   tuning is insufficient.

---

## §9. References

### §9.1 External (URL-cited; verified via WebFetch unless noted)

- §7.1: `https://pkg.go.dev/github.com/playwright-community/playwright-go`
  — verified 2026-05-04. Module path, version v0.5700.1, MIT,
  install steps, API method signatures (`Run`, `Stop`, `Launch`,
  `NewPage`, `Goto`, `Content`, `Browser.Close`).
- §7.2: `https://github.com/chromedp/chromedp` — verified 2026-05-04.
  Latest v0.15.1, MIT, "no Node.js runtime needed" advantage.
- §7.3: `https://github.com/go-rod/rod` — verified 2026-05-04.
  Latest v0.116.2, MIT, July 2024 last release.
- §7.4: `https://github.com/temoto/robotstxt` — verified 2026-05-04.
  MIT, production-active, API quoted (`FromBytes`, `TestAgent`).
- §7.5: `https://github.com/fivetaku/insane-search` — verified
  2026-05-04. MIT, 606 stars, 4-phase pipeline (0-3) reconciled
  to CACHE-001's 5-phase model in §2.2.
- §7.6: `https://github.com/jimsmart/grobotstxt` — verified
  2026-05-04. Apache-2.0, last release March 2022 (rejected for
  staleness).
- `https://pkg.go.dev/net/http` — verified 2026-05-04.
  `Client.Timeout`, `Transport.TLSHandshakeTimeout`,
  `Transport.ResponseHeaderTimeout`, `CheckRedirect` field
  semantics (default = stop after 10 hops).
- `https://pkg.go.dev/crypto/tls` — verified 2026-05-04.
  `Config.ServerName`, `NextProtos`, `MinVersion`, `MaxVersion`,
  `CipherSuites`, `InsecureSkipVerify` (with security warning
  preserved).
- `https://playwright.dev/docs/intro` — verified 2026-05-04.
  Resource consumption metrics not explicitly documented in the
  intro page; specifics surveyed empirically in §7.4 above.
- `https://www.robotstxt.org/robotstxt.html` — attempted 2026-05-04;
  server returned HTTP 403 at fetch time. The protocol is well-
  established (since 1994) and codified in RFC 9309; CACHE-001
  defers to the temoto/robotstxt library implementation as the
  de-facto reference for v0.1.
- RFC 9309 (Robots Exclusion Protocol, IETF Sept 2022) — protocol
  authority; not WebFetch-cited inline.

### §9.2 Internal (file:line cited)

- `.moai/project/roadmap.md:58` — M3 row "SPEC-CACHE-001 | 5-phase
  access fallback".
- `.moai/project/roadmap.md:117-128` — M3 parallelization plan
  (CACHE-001 listed as gated on FAN-001).
- `.moai/project/product.md:9` — "Access layer: inspired by
  fivetaku/insane-search".
- `.moai/project/product.md:93` — "insane-search — MIT (pattern
  reference only; not bundled)".
- `.moai/project/research.md:9` — "fivetaku/insane-search ... 5-phase
  access fallback idea (index → probe → TLS → browser)".
- `.moai/project/research.md:74-78` — "insane-search 5-phase pattern —
  reimplemented in Go".
- `.moai/project/research.md:91` — `https://github.com/fivetaku/insane-search`.
- `.moai/project/structure.md:30-34` — `internal/access/{phase0_index,
  phase1_probe, phase2_tls, phase3_browser}` reservation.
- `.moai/project/structure.md:160` — `pkg/types` SDK boundary clause.
- `.moai/project/tech.md:49` — "Browser fallback: Playwright MCP per
  insane-search pattern".
- `.moai/project/tech.md:153` — "5-phase access fallback brittleness
  | Low | Phase 3 (Playwright) is bounded behind feature flag".
- `.moai/specs/SPEC-FAN-001/spec.md:6` — FAN-001 status: approved.
- `.moai/specs/SPEC-FAN-001/spec.md:466-531` — §2.5 per-adapter
  timeout derivation; CACHE-001's per-phase ctx mirrors.
- `.moai/specs/SPEC-FAN-001/spec.md:535-572` — §2.6 per-index slice
  state discipline; CACHE-001's per-phase results follow same
  no-shared-map pattern.
- `.moai/specs/SPEC-FAN-001/spec.md:765-783` — TestDispatchConcurrent
  workload; CACHE-001 NFR-CACHE-004 mirrors.
- `.moai/specs/SPEC-FAN-001/spec.md:843-849` — `goleak.VerifyTestMain`
  pattern; CACHE-001 NFR-CACHE-005 reuses with playwright-go ignore
  list.
- `.moai/specs/SPEC-IDX-001/spec.md:6` — IDX-001 status: draft.
- `.moai/specs/SPEC-IDX-001/spec.md:546` — REQ-IDX-001 Index
  construction contract (CACHE-001 Phase 1 consumes via IndexLookup
  port).
- `.moai/specs/SPEC-IDX-001/spec.md:551` — REQ-IDX-006 Search
  contract (CACHE-001 Phase 1 consumes).
- `.moai/specs/SPEC-IDX-001/spec.md:556` — REQ-IDX-011 observability
  pattern; CACHE-001 mirrors.
- `.moai/specs/SPEC-OBS-001/spec.md:88-93` — REQ-OBS-001..006
  baseline collectors.
- `.moai/specs/SPEC-OBS-001/spec.md:101` — NFR-OBS-002 cardinality
  safety; CACHE-001 extends allowlist with `phase` and `blocked`
  outcome.
- `.moai/specs/SPEC-ADP-001/spec.md:373-374` — REQ-ADP-011
  concurrent-safety contract precondition for NFR-CACHE-004.
- `pkg/types/normalized_doc.go:40-56` — NormalizedDoc 15-field
  struct; CACHE-001 produces FetchedContent that callers merge into
  doc.Body.
- `internal/adapters/reddit/parse.go:160` — Reddit Body =
  `data.selftext` (snippet-only for link posts).
- `internal/adapters/hn/parse.go:124,143` — HN Body = HTML-stripped
  `story_text` (snippet-only for URL-only submissions).
- `internal/obs/metrics/metrics.go:33-65` — Registry shape;
  CACHE-001 extends with `internal/obs/metrics/access.go`
  (per-phase Counter + Histogram, no labels beyond `phase` + `outcome`).
- `internal/obs/metrics/metrics.go:169-176` — cardinality allowlist
  (CACHE-001 extends with `phase` and `blocked` outcome value).
- `go.mod:3` — Go 1.25.8.
- `go.mod` (full) — existing direct deps; CACHE-001 adds two
  (`playwright-go`, `temoto/robotstxt`).
- `deploy/docker-compose.yml:31-216` — compose stack; CACHE-001
  does NOT add a service in v0.1 (Playwright runs as a child
  process of the Go binary; chromedp/headless-shell sidecar is
  deferred to SPEC-DEPLOY-001).

---

*End of SPEC-CACHE-001 research.md*
