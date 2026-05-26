# SPEC-ADP-003 Implementation Plan (Post-Hoc)

**SPEC**: SPEC-ADP-003 — arXiv + Paper Search Adapter
**Status**: implemented (2026-05-07)
**Methodology**: TDD (RED → GREEN → REFACTOR)
**Coverage Target**: 85%
**Owner**: expert-backend
**Priority**: P0

---

## 1. Overview

ADP-003 is the FIRST M3 adapter and the FIRST academic-domain adapter,
consuming arXiv's public Atom API at
`https://export.arxiv.org/api/query`. The adapter introduces TWO
delta patterns versus ADP-001/ADP-002:

1. **XML response handling** via Go stdlib `encoding/xml` with
   namespace-aware decoding (Atom 1.0 default namespace +
   opensearch + arxiv extension namespaces).
2. **Per-instance rate-limit serialisation gate** — the ONLY
   adapter with shared mutable state (`rateMu sync.Mutex`,
   `nextRequest time.Time`) honouring arXiv's "play nice" 3-second
   courtesy interval, with the actual wait happening OUTSIDE the
   lock and respecting ctx cancellation.

Score is the constant `0.5` (arXiv has no per-paper relevance
score); RRF in SPEC-IDX-001 weights rank not score, so the constant
is harmless.

The adapter is one-shot per call: no fanout, no retry, no caching,
no ranking fusion, zero observability emission.

---

## 2. Architecture

### 2.1 Package Layout

```
internal/adapters/arxiv/
├── arxiv.go         — Adapter, Options, New, Name, Capabilities, Healthcheck, rate-limit state
├── arxiv_test.go    — interface conformance + Capabilities determinism + New defaults
├── search.go        — (*Adapter).Search hot path + waitForRateSlot + buildSearchQuery
├── search_test.go   — E2E + happy path + error categorisation + ctx tests
├── client.go        — *http.Client, doRequest, categorizeStatus, redirectAllowlist
├── client_test.go   — categorizeStatus table + parseRetryAfter + redirect allowlist + UA/Accept headers
├── parse.go         — parseFeed transform (Atom XML, namespace-aware) + collapseWS helper
├── parse_test.go    — field-mapping + namespace + whitespace-collapse + pagination cursor
├── errors.go        — ErrInvalidQuery + ErrInvalidStart sentinels + parseRetryAfter (5s/60s)
├── rate_test.go     — REQ-ADP3-012 rate-limit serialisation tests
├── bench_test.go    — BenchmarkParseFeed25Entries + TestMain goleak
└── testdata/        — 11 Atom XML fixtures
```

### 2.2 Key Data Structures

**`Adapter` struct** (`arxiv.go`): `httpClient`, `baseURL`,
`userAgent`, `healthcheckTarget`, plus rate-limit state — `rateMu
sync.Mutex`, `nextRequest time.Time`, `minInterval time.Duration`.
The mutex is held only for the compute+update sequence (~10ns); the
actual wait happens outside the lock.

**`Options` struct**: `BaseURL`, `HTTPClient`, `UserAgentVersion`,
`HealthcheckTarget`, `MinRequestInterval` (default 3s; tests inject
0 or 1ms).

**`atomFeed` / `atomEntry` / `atomAuthor` / `categoryAttr` /
`linkAttr` struct types** (`parse.go`): XML-tagged structs using
fully-qualified namespace URIs as `xml.Name` match keys
(`http://www.w3.org/2005/Atom feed`, `http://arxiv.org/schemas/atom
doi`, etc.).

**Sentinels** (`errors.go`): `ErrInvalidQuery`, `ErrInvalidStart`.
Both wrapped in `*types.SourceError{Category: CategoryPermanent}`.

**Constants** (`arxiv.go`): `defaultBaseURL`,
`defaultUAVersion="v0.1"`, `defaultHealthcheckTarget`,
`defaultMinInterval=3*time.Second`, `constantScore=0.5` (with
`@MX:NOTE` documenting §2.3 rationale).

### 2.3 Rate-Limit Algorithm (REQ-ADP3-012)

`waitForRateSlot(ctx)` in `search.go`:

1. Lock `rateMu`. Compute `wait := time.Until(a.nextRequest)`.
2. Update `a.nextRequest = time.Now().Add(a.minInterval)`. Unlock.
3. If `wait <= 0`, return `nil` immediately.
4. Otherwise: `select { case <-time.After(wait): return nil; case
   <-ctx.Done(): return ctx.Err() }`.

Properties: per-instance state (two `*Adapter` instances do not
share state); ctx cancellation breaks the wait immediately; the
`nextRequest` update happens BEFORE the wait so subsequent
goroutines see the slot reserved.

### 2.4 Hot-Path Flow (REQ-ADP3-002)

1. Validate `q.Text` (REQ-ADP3-008) via `unicode.IsSpace` scan.
2. Parse `q.Cursor` as non-negative int via `strconv.Atoi`.
3. `waitForRateSlot(ctx)` (REQ-ADP3-012).
4. Build URL via `url.Values` with `search_query` (with optional
   `cat:<value> AND ` prepend per REQ-ADP3-007), `start`,
   `max_results` (clamped 1–100, default 25), `sortBy=relevance`,
   `sortOrder=descending`.
5. `doRequest()` sets `User-Agent` and `Accept: application/atom+xml`.
6. Route by HTTP status via `categorizeStatus()`.
7. `parseFeed()` decodes Atom XML, applies `collapseWS` to title and
   summary, strips `http://arxiv.org/abs/` prefix from `<id>` for
   bare arXiv ID, surfaces `next_cursor` on the last doc when
   `currentStart + len(entries) < totalResults`.

### 2.5 Integration Points

- Registered with `internal/adapters/registry.go` for the
  `CategoryAcademic` intent (`internal/router/category.go:97`).
- Consumes `pkg/types` and `internal/obs/reqid.NewTransport`.
- Score is constant `0.5` per §2.3 rationale; SPEC-IDX-001 RRF
  weights rank not score.

---

## 3. Test Coverage Notes

- Coverage meets 85% target.
- `go test -race` clean across `rateMu` contention (50-goroutine
  `TestSearchConcurrentSafe` with `MinRequestInterval=0`).
- `rate_test.go::TestSearchRateLimitInterval` verifies 3
  sequential calls with `MinRequestInterval=10ms` elapse in
  `[20ms, 50ms]`.
- `rate_test.go::TestSearchRateLimitCtxCancel` verifies ctx cancel
  during a 10s wait returns within 5ms.
- `rate_test.go::TestSearchRateLimitPerInstance` verifies two
  `*Adapter` instances do not serialise across each other.
- `BenchmarkParseFeed25Entries` ≤ 5ms median, allocs/op ≤ 700
  (XML floor is higher than JSON's ≤ 500 due to `encoding/xml`
  constant overhead).
- `TestMain` calls `goleak.VerifyTestMain(m)` for package-wide
  leak detection.

---

## 4. Technical Decisions (Locked During Implementation)

| Decision | Rationale |
|----------|-----------|
| Per-instance rate-limit gate honouring arXiv's 3s guideline | arXiv's published "play nice" rule; the mutex is held only for ~10ns (compute+update); ctx cancellation respected via `select` outside the lock. |
| `Score=0.5` constant | arXiv's Atom feed has no per-entry relevance score. RRF in SPEC-IDX-001 uses rank not score. |
| Whitespace collapse via `strings.Join(strings.Fields(s), " ")` | arXiv pretty-prints XML with arbitrary newlines/indentation in `<title>` and `<summary>`. Trim is insufficient; regex is slower. |
| `parseRetryAfter` and `categorizeStatus` duplicated from Reddit/HN | Rule-of-three barely reached; extraction deferred to SPEC-ADP-REFAC-001 to avoid coupling parallel SPECs to a shifting shape. |
| LaTeX pass-through | arXiv abstracts contain `$E=mc^2$`; synthesis decides whether to render or strip. |
| `id_list` mode, `search_by_date` mode, date-range filter | Deferred to P2; v0.1 hardcodes `sortBy=relevance`. |

---

## 5. Risks Mitigated

- **`encoding/xml` namespace handling** — uses
  `xml.Name{Space, Local}` with fully-qualified URI keys; tested via
  `TestParseFeedDOIInArxivNamespace`.
- **Rate-limit mutex contention** — held only for 10ns; wait
  outside lock; 50-goroutine race test verifies.
- **Goroutine leak via `time.After`** — `waitForRateSlot` uses
  `time.NewTimer` + `defer t.Stop()` + `select` with ctx.Done.
- **Score=0.5 collision in RRF** — RRF re-ranks by rank within
  source; equal scores within arxiv don't destabilise fusion.

---

## 6. Out-of-Scope Reminders (from spec.md §7)

- Wrapping `openags/paper-search-mcp` → deferred to SPEC-ADP-003-MCP.
- Citation count enrichment via Semantic Scholar → SPEC-ENRICH-001.
- OAI-PMH bulk-export protocol → out of v0.1.
- Author affiliation extraction (`<arxiv:affiliation>`) → future
  patch SPEC.
- PDF download integration → SPEC-CACHE-001 owns 5-phase fallback.
- Date-range filter via `submittedDate:[…]` → `SupportsSince=true`
  declared but translation deferred to P2.
- Cross-adapter helper extraction → SPEC-ADP-REFAC-001 post-M3.

---

*End of SPEC-ADP-003 plan.md (post-hoc, v1.0)*
