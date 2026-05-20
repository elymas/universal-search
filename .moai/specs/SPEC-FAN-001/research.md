# SPEC-FAN-001 Research — Multi-source Fanout

**Status**: Research-complete; ready for SPEC drafting
**Author**: limbowl (via manager-spec)
**Created**: 2026-05-04
**Milestone**: M3 — Fanout, adapters, index
**Depends on**: SPEC-CORE-001, SPEC-IR-001, SPEC-ADP-001, SPEC-ADP-002, SPEC-OBS-001

---

## 0. Research Mandate

SPEC-FAN-001 (Multi-source Fanout) is the M3 gateway SPEC. It replaces the
placeholder `runFanout` in `cmd/usearch/query.go:324-368` with a production
package at `internal/fanout/` that:

1. Dispatches `RoutingDecision.AdapterSet` to N adapters in parallel under a
   bounded goroutine pool.
2. Enforces per-adapter timeouts derived from `Capabilities.RecommendedTimeout`
   with a `Query.Deadline` override path.
3. Assembles partial results when a subset of adapters fail or exceed their
   per-adapter deadline before the overall ctx deadline fires.
4. Deduplicates results across adapters (URL canonicalization + content hash)
   before returning `[]types.NormalizedDoc` to the caller.
5. Emits per-adapter observability via the existing `FanoutInflight` gauge plus
   the inherited `wrappedAdapter` counter/histogram from SPEC-CORE-001.
6. Closes M3's exit-criterion gate
   (`.moai/project/roadmap.md:150` — "`usearch query` returns fused results
   across ≥5 adapters").

This research catalogs (a) the existing-code state that SPEC-FAN-001 must
respect, (b) the external library landscape (errgroup vs semaphore vs
sourcegraph/conc) with verifiable URLs, (c) the design alternatives and
rejection rationale, (d) integration surfaces (CLI today, MCP tomorrow),
(e) race / leak / cancellation analysis, and (f) open questions that the
plan-auditor will challenge.

Every claim is either file-cited (e.g.,
`internal/adapters/registry.go:195-219`) or URL-cited from verified web
sources. No invented facts.

---

## 1. Existing-Code State

### 1.1 The `internal/fanout/` Stub

The fanout package is currently a 4-line stub:

```go
// Package fanout is the stub for the multi-source fan-out orchestrator.
// Full implementation lands in SPEC-IR-001 / SPEC-FAN-001.
package fanout
```

Source: `internal/fanout/fanout.go:1-3`. Reservation made by SPEC-BOOT-001
and reaffirmed in `.moai/project/structure.md:17`
(`internal/fanout/                   # Multi-source dispatch (SPEC-FAN)`).

The package directory contains exactly one file (`fanout.go`); no tests, no
sub-packages. SPEC-FAN-001 fills the package end-to-end.

### 1.2 The Placeholder `runFanout` in `cmd/usearch/query.go`

The CLI ships a private `runFanout` helper that SPEC-FAN-001 will retire:

```go
func runFanout(ctx context.Context, names []string, reg *adapters.Registry, prompt string) (
    docs []types.NormalizedDoc, errs map[string]error,
)
```

Source: `cmd/usearch/query.go:324-368`. Two `@MX:ANCHOR` + `@MX:WARN`
annotations document its replacement target:

- `@MX:ANCHOR: [AUTO] CLI-internal fanout; replacement target when SPEC-FAN-001 lands.`
  (`cmd/usearch/query.go:316`)
- `@MX:WARN: [AUTO] runFanout spawns one goroutine per adapter using errgroup. ...
   goroutine cancellation discipline is load-bearing for NFR-CLI-002.`
  (`cmd/usearch/query.go:320-323`)

Behaviour today (lines 324-368):

- Spawns one goroutine per `name` in `effectiveSet` via
  `golang.org/x/sync/errgroup.WithContext(ctx)` (line 334).
- Captures `(docs, err)` per adapter into a slice indexed by position,
  returning `nil` to errgroup so the group never short-circuits on a single
  adapter error (line 346: `return nil // never return error to eg; collect individually`).
- Calls `eg.Wait()` (line 351). Then walks the result slice to merge
  successes and surface the per-adapter error map.
- Honors ctx cancellation via the errgroup-derived `egCtx` (line 363-365).
- **No concurrency limit** — `effectiveSet` of length 12 spawns 12 goroutines
  without a cap; M3's fan-out target is exactly 12+ adapters
  (`.moai/project/roadmap.md:150`).
- **No per-adapter timeout** — every adapter inherits the caller's ctx
  unchanged. A slow adapter pins the wall-clock at the caller's deadline.
- **No deduplication** — results from different adapters are simply appended
  (line 358: `docs = append(docs, r.docs...)`).
- **No partial-result observability** — the `FanoutInflight` gauge is
  registered (`internal/obs/metrics/metrics.go:89-95`) but `runFanout` never
  increments it.

The placeholder is intentionally minimal because SPEC-CLI-001 needed end-to-end
flow ahead of M3.

### 1.3 The `Adapter.Search` Contract (Inherited)

The adapter contract is fixed by SPEC-CORE-001:

```go
type Adapter interface {
    Name() string
    Search(ctx context.Context, q Query) ([]NormalizedDoc, error)
    Healthcheck(ctx context.Context) error
    Capabilities() Capabilities
}
```

Source: `pkg/types/adapter.go:28-45`.

The interface contract requires implementations to:

- "MUST honour ctx cancellation in Search and Healthcheck"
  (`pkg/types/adapter.go:16`)
- "MUST wrap raw errors in *SourceError with the appropriate Category so the
  wrappedAdapter can classify outcomes uniformly"
  (`pkg/types/adapter.go:17-18`)
- "MUST keep Name() stable across the process lifetime — it is the
  Prometheus label value and the registry key"
  (`pkg/types/adapter.go:20-21`)

This means SPEC-FAN-001 can rely on:

- ctx-cancellation correctness when fanout cancels via per-adapter timeout
- Categorised errors via `errors.Is(err, types.ErrTransient)` /
  `errors.Is(err, types.ErrRateLimited)` (`pkg/types/errors.go:103-119`)
- A stable string identity per adapter for fanout's internal state map

The Reddit adapter (SPEC-ADP-001) and HN adapter (SPEC-ADP-002) both
implement this contract today; ADP-001 spec.md REQ-ADP-011 explicitly
guarantees concurrent-safety under N goroutines invoking `Search` against
the same adapter instance (`internal/adapters/reddit/reddit.go` test
`TestSearchConcurrentSafe`).

### 1.4 Registry `wrappedAdapter` (Sole-Emitter Discipline)

Every adapter retrieved via `registry.Get(name)` is wrapped:

```go
r.adapters[name] = &wrappedAdapter{inner: a, obs: r.obs}
```

Source: `internal/adapters/registry.go:136`.

The `wrappedAdapter.Search` (`internal/adapters/registry.go:195-219`) emits
per-call observability so adapter authors and FAN-001 do NOT add their own
counter/histogram for `usearch_adapter_calls_total` or
`usearch_adapter_call_duration_seconds`:

- 1 OTel span `adapter.search` with attributes `adapter.name` /
  `adapter.outcome` / `adapter.result_count`
- 1 Prometheus counter increment on
  `AdapterCalls{adapter,outcome}` (`internal/adapters/registry.go:228-230`)
- 1 Prometheus histogram observation on `AdapterCallDuration{adapter}`
  (`internal/adapters/registry.go:231-233`)
- 1 slog record at INFO (success) or WARN (non-success)
  (`internal/adapters/registry.go:235-251`)

[HARD] SPEC-FAN-001 MUST NOT re-emit these metric families. The fanout
layer's observability scope is restricted to:

- `usearch_fanout_goroutines_inflight{adapter_class}` (Gauge, already
  registered at `internal/obs/metrics/metrics.go:89-95`)
- ONE NEW metric family or ZERO new families (decision in §6 below)
- OTel spans named `fanout.dispatch` (parent span, contains all adapter
  spans as children)
- slog records summarising fanout-level events (start, partial result,
  complete)

This sole-emitter discipline mirrors the boundary CORE-001 enforced for
adapters and IR-001 inherited for the router.

### 1.5 The `FanoutInflight` Gauge (Pre-Registered)

The Prometheus collector is already registered:

```go
fanoutInflight := prometheus.NewGaugeVec(
    prometheus.GaugeOpts{
        Name: "usearch_fanout_goroutines_inflight",
        Help: "Number of fanout goroutines currently active.",
    },
    []string{"adapter_class"},
)
```

Source: `internal/obs/metrics/metrics.go:89-95`.

The label name `adapter_class` is in the cardinality allowlist
(`internal/obs/metrics/metrics.go:171`). SPEC-FAN-001 needs to define what
values this label takes. Two candidates:

- (a) Use `adapter_class` matching one of the SPEC-IR-001 categories
  (`web`/`social`/`academic`/`korean`/`mixed`/`unknown`) — bounded set,
  6 values, low cardinality.
- (b) Use `adapter_class` matching the literal adapter name (12+ values)
  — duplicates `AdapterCalls{adapter}` cardinality, which is also already
  in the allowlist.

The safer reading of the existing label name (`adapter_class`, not
`adapter`) is option (a): the gauge measures aggregate concurrency per
**class** of adapter, not per individual adapter. SPEC-FAN-001 will adopt
this reading and document the mapping.

The metric is pre-initialised with `WithLabelValues("web").Add(0)` in
`NewRegistry()` (`internal/obs/metrics/metrics.go:139`), so the family
appears in `/metrics` output before any real fanout traffic. The other 5
class values (`social`, `academic`, `korean`, `mixed`, `unknown`) will get
their first observation on real traffic; the `TestMetricsIncludesAllFamilies`
test (NFR-OBS-002 cardinality guard at
`internal/obs/metrics/metrics_test.go:166`) is unaffected.

### 1.6 The `RoutingDecision.AdapterSet` Input Contract

SPEC-IR-001 publishes the input shape:

```go
type RoutingDecision struct {
    Category    Category
    Confidence  float64
    AdapterSet  []string  // lexicographically sorted
    Lang        string
    Source      ClassificationSource
    Metadata    map[string]any
}
```

Source: `internal/router/routing_decision.go:23-37`.

`AdapterSet` is "the lexicographically-sorted set of adapter names eligible
to serve this query" (`internal/router/routing_decision.go:29-30`). The
fanout consumes this slice verbatim. The Router's `selectAdapterSet`
(`internal/router/router.go:258-294`) guarantees:

- Names exist in the registry at construction time
- Capabilities-Lang compatibility was checked
- Empty-set fallback flagged via `Metadata["adapter_set_fallback"] = true`

This means SPEC-FAN-001 can iterate the slice without re-validating
membership. If `registry.Get(name)` returns `(_, false)` for any name in
AdapterSet, that is a registry-corruption signal worth a structured error
return (the registry deletes are not currently supported — see
`internal/adapters/registry.go:75-167` — but FAN-001's failure mode must
still cope).

### 1.7 The `Query.Deadline` and `Capabilities` Inputs

`pkg/types/query.go:32-34` documents `Query.Deadline` as:

> "Deadline is a soft deadline; the orchestrator SHOULD honour this via
> context.WithDeadline before invoking Search."

`pkg/types/capabilities.go:38-62` declares 10 fields. Notably MISSING from
the current Capabilities struct:

- **No `RecommendedTimeout` field** today.
- **No `MaxParallel` hint** today.

SPEC-FAN-001 has two options:

- (A) Add a `RecommendedTimeout time.Duration` field to `Capabilities`
  (a `pkg/types` change requires a major-version SDK bump per
  `.moai/project/structure.md:160`).
- (B) Hardcode a per-adapter timeout policy in fanout, with the
  `Query.Deadline` and a global default (e.g., 8 seconds) as the only
  inputs. Adapters that need a faster timeout can wrap their own
  `context.WithDeadline` inside `Search`.

Adopting (B) keeps SPEC-CORE-001 untouched and matches the philosophy that
"Internal packages (`internal/*`) have no stability guarantee — free to
refactor" (`.moai/project/structure.md:163`). Open Question §6.1 carries
the (A) alternative for SPEC author consideration.

There is no `MaxParallel` field; `Query.MaxResults` exists but bounds
results, not concurrency. SPEC-FAN-001's concurrency cap is therefore a
fanout-internal config knob (default 8, configurable via Options).

### 1.8 Adapter Concurrency Safety (Inherited)

`internal/adapters/reddit/` ships `TestSearchConcurrentSafe` which validates
50 goroutines × one Search against a single `*Adapter` produce 50
independent HTTP round-trips with no race-detector alarms (per ADP-001
REQ-ADP-011 spec.md:373-374). HN follows the same shape.

This means SPEC-FAN-001 MAY hold a single `*Adapter` instance across all
goroutines for a single fanout call AND across multiple concurrent fanout
calls — the per-adapter `Search` is goroutine-safe under the contract.

SPEC-FAN-001's own race-safety test must therefore validate:

- (i) Multiple goroutines fanning out within one call (already covered by
  ADP-001's adapter-level test pattern).
- (ii) Multiple concurrent fanout calls from N caller goroutines, each
  invoking `Fanout(ctx, decision)` on the same `*Fanout` instance. This
  is fanout-specific; ADP-001's test does not cover it.

### 1.9 Existing Concurrency Dependencies

Pinned in `go.mod`:

- `golang.org/x/sync v0.20.0` (indirect): provides `errgroup` and
  `semaphore`. Source: `go.mod:33`.
- `go.uber.org/goleak v1.3.0` (indirect): provides goroutine-leak
  verification. Source: `go.mod:30`.

Already in use:

- `cmd/usearch/query.go:25` imports `golang.org/x/sync/errgroup` for the
  placeholder `runFanout`.
- ADP-001 / ADP-002 use `goleak.VerifyNone` in their bench/test setup.

NOT pinned:

- `golang.org/x/sync/semaphore` — sub-package of the same module; no
  separate go.mod line needed.
- `github.com/sourcegraph/conc` — would require a new go.mod entry.

Adopting `errgroup.WithContext` + `errgroup.SetLimit` (added in Go
errgroup v0.8.0 — see §2 below) requires zero new module additions; the
pinned `golang.org/x/sync v0.20.0` is well above that floor.

---

## 2. External Library Survey

### 2.1 `golang.org/x/sync/errgroup`

URL: https://pkg.go.dev/golang.org/x/sync/errgroup
Verified: 2026-05-04 via WebFetch.

Quoted API guarantees:

- `WithContext(ctx)` — "The derived Context is canceled the first time a
  function passed to Go returns a non-nil error or the first time Wait
  returns, whichever occurs first."
- `Go(f func() error)` — "Go calls the given function in a new goroutine.
  ... It blocks until the new goroutine can be added without the number of
  goroutines in the group exceeding the configured limit."
- `Wait() error` — "Wait blocks until all function calls from the Go
  method have returned, then returns the first non-nil error (if any)
  from them."
- `TryGo(f func() error) bool` — non-blocking; reports whether the
  goroutine started.
- `SetLimit(n int)` — "SetLimit limits the number of active goroutines in
  this group to at most n. A negative value indicates no limit."

Implications for SPEC-FAN-001:

- **First-error short-circuit is HARMFUL for fanout**: if one adapter
  returns an error, the derived ctx is cancelled, killing all other
  adapters. Fanout MUST use the errgroup-pattern-but-suppress-error idiom
  visible in the placeholder `cmd/usearch/query.go:346`
  (`return nil // never return error to eg; collect individually`).
  This pattern is widely accepted but loses the semantic value of
  errgroup.Wait()'s error return.
- **`SetLimit(N)` is the natural concurrency cap** — combines with
  `Go(f)` for a clean pool implementation, no separate semaphore needed.
- The placeholder already uses errgroup; FAN-001 keeps the same library.

### 2.2 `golang.org/x/sync/semaphore`

URL: https://pkg.go.dev/golang.org/x/sync/semaphore
Verified: 2026-05-04 via WebFetch.

Provides `Weighted` with `NewWeighted(n int64)`, `Acquire(ctx, n)` /
`TryAcquire(n)` / `Release(n)`. Documented use case: "implementing a
'worker pool' pattern without explicitly managing worker lifecycle"
(WebFetch quote, 2026-05-04).

Implications for SPEC-FAN-001:

- More flexible than `errgroup.SetLimit` (supports weighted permits) but
  SPEC-FAN-001 doesn't need weights — every adapter is one permit.
- Does NOT couple to ctx-cancellation; would need to be paired with
  `sync.WaitGroup` or manual coordination.
- Adds a second primitive for a problem `errgroup.SetLimit` already
  handles cleanly.

Verdict: Rejected for v0.1. errgroup with SetLimit is sufficient.

### 2.3 `github.com/sourcegraph/conc`

URL: https://pkg.go.dev/github.com/sourcegraph/conc
Verified: 2026-05-04 via WebFetch.

Provides:

- `pool.Pool` / `pool.NewWithResults[T]()` / `pool.ContextPool` /
  `pool.ResultErrorPool`
- Panic recovery via `WaitAndRecover()`
- Generic typed result collection

Documented philosophy (WebFetch quote): "all concurrency should be scoped.
That is, goroutines should have an owner and that owner should always
ensure that its owned goroutines exit properly."

Implications for SPEC-FAN-001:

- `pool.NewWithResults[T]()` would simplify the result-collection pattern
  (replaces the per-index slice + manual merge).
- Panic recovery is a net positive: an adapter that panics today
  crashes the goroutine without surfacing the cause to the caller. With
  `conc`, the panic is captured and re-raised on `Wait`.
- **STATUS**: Pre-1.0 (per the package's own description). The package
  is "used by 154+ projects" but breaking changes possible before 1.0.
- Adding it requires a new `go.mod` line; SPEC-DEP-001's spirit prefers
  fewer dependencies.

Verdict: Tempting but rejected for v0.1. The benefits (typed results,
panic recovery) are nice-to-have; the cost (pre-1.0 dependency, additional
SBOM entry) is concrete. SPEC-FAN-001 implements the result collection
manually with a typed slice. Open Question §6.5 documents revisit
triggers (e.g., if fanout panic-handling becomes a recurring pain).

### 2.4 Comparison Summary

| Library | License | API fit | Cancellation | Concurrency cap | Panic recovery | New dep |
|---------|---------|---------|--------------|-----------------|----------------|---------|
| errgroup | BSD-3 | High | First-error cancels ctx | `SetLimit(n)` | None | No |
| semaphore | BSD-3 | Medium | Manual via ctx | `NewWeighted(n)` | None | No |
| sourcegraph/conc | MIT | Highest | `ContextPool` cancels on first error | `WithMaxGoroutines(n)` | Built-in | Yes (pre-1.0) |

[HARD] SPEC-FAN-001 selects **errgroup with `SetLimit` + the suppress-error
idiom** for v0.1. Rejection rationale recorded above; revisit gates listed
in Open Questions.

---

## 3. Design Alternatives + Rejection Rationale

### 3.1 Concurrency Model

| Option | Description | Decision |
|--------|-------------|----------|
| A | errgroup with `SetLimit(N)`, suppress per-task error to avoid first-error cancel | **SELECTED** for v0.1 |
| B | `sync.WaitGroup` + manual `chan error` accumulation | Rejected — re-implements errgroup primitive without adding value |
| C | `sourcegraph/conc.pool.NewWithResults` | Rejected — pre-1.0 dep risk; revisit per OQ §6.5 |
| D | Per-call goroutine spawn without cap | Rejected — fanout is the only place 12+ goroutines spawn; cap protects under high-QPS scenarios |
| E | Channel-based pipeline with worker pool reading from job channel | Rejected — overkill for fan-out-fan-in; latency floor of channel hand-offs adds 100s of ns/op without benefit |

### 3.2 Per-Adapter Timeout

| Option | Description | Decision |
|--------|-------------|----------|
| A | `context.WithTimeout(parentCtx, perAdapterDeadline)` derived inside fanout | **SELECTED** for v0.1 |
| B | Add `Capabilities.RecommendedTimeout` field, use per-adapter override | Rejected for v0.1 — pkg/types stability bump cost; OQ §6.1 keeps option open |
| C | Hardcode a global 8s timeout, no per-adapter knob | Rejected — different adapters have different SLOs (Reddit = 10s today per ADP-001 §6.5; HN Algolia is faster) |
| D | Carry timeout in `Query.Filters` keyed `_per_adapter_timeout_<name>` | Rejected — abuses Filters which is "adapter-specific" per pkg/types/query.go:38-39 |

[HARD] Decision A: Default 8 seconds per adapter, `Options.PerAdapterTimeout`
overrides at construction. `Query.Deadline` (the caller's overall budget)
takes precedence — per-adapter ctx inherits the smaller of (default 8s,
remaining time to Query.Deadline).

### 3.3 Partial-Result Assembly

| Option | Description | Decision |
|--------|-------------|----------|
| A | Return whatever completed before the overall deadline; per-adapter errors collected separately | **SELECTED** for v0.1 |
| B | Wait for all adapters regardless of overall deadline | Rejected — caller's ctx must be honoured |
| C | Return "first N results" early, cancel remaining | Rejected — caller doesn't know N; the fanout output is consumed by IDX-001 RRF (M3) which needs the full set |
| D | Streaming/incremental delivery via channel | Deferred to SPEC-SYN-004 (M4) per `.moai/project/roadmap.md:66` |

### 3.4 Deduplication Strategy

| Option | Description | Decision |
|--------|-------------|----------|
| A | URL canonicalization (strip tracking params, lowercase host, sort query keys) + content hash via `NormalizedDoc.CanonicalHash()`; dedup key = canonical URL FIRST, then hash | **SELECTED** for v0.1 |
| B | Hash-only dedup using `NormalizedDoc.CanonicalHash()` | Rejected — same URL with different titles (Reddit edited title, HN repost) would not dedup; URL is the more meaningful key |
| C | URL-only dedup | Rejected — different URLs with identical content (mirrors, crossposts) would surface as duplicates |
| D | Defer dedup to SPEC-SYN-003 (M4 dedup + clustering) | Rejected — fanout is the natural choke point; SYN-003 (`.moai/project/roadmap.md:65`) does deeper clustering, not basic exact dedup |

URL canonicalization rules (defined in §3.4.1 below) intentionally MILD —
SPEC-IDX-001 RRF (M3) and SYN-003 (M4) layer richer near-dup detection on
top.

#### 3.4.1 URL Canonicalization Rules

For dedup-key purposes ONLY (the `NormalizedDoc.URL` returned to callers is
unchanged — adapters already normalize per their own SPEC, e.g., ADP-001
REQ-ADP-006). The canonicalization function is internal to fanout.

1. Lowercase the host (`www.reddit.com` → `www.reddit.com`; `WWW.Reddit.COM`
   → `www.reddit.com`). Per RFC 3986 §6.2.2.1, host is case-insensitive.
2. Strip the URL fragment (`...#section` → `...`). Fragments don't reach
   the server.
3. Strip well-known tracking query parameters: `utm_source`, `utm_medium`,
   `utm_campaign`, `utm_term`, `utm_content`, `gclid`, `fbclid`,
   `mc_eid`, `mc_cid`, `_ga`, `ref`, `ref_src`. (List sourced from
   common analytics platforms; adapters that need richer stripping can
   layer in their own SPEC.)
4. Trim trailing slash from path (`/foo/` → `/foo`) — except for paths
   equal to `/`.
5. Sort remaining query parameters alphabetically by key.
6. Lowercase scheme (`HTTPS://...` → `https://...`).
7. Do NOT touch path case — paths are case-sensitive in HTTP.
8. Do NOT touch percent-encoding — RFC 3986 normalisation here would
   require a full encoder/decoder pass; keep it simple in v0.1.

The output of canonicalization is the dedup key, NOT the displayed URL.

### 3.5 Result Ordering Rules

| Option | Description | Decision |
|--------|-------------|----------|
| A | Order by adapter priority (= AdapterSet input order, which is lexicographic by name) | **SELECTED** as TIE-BREAKER only |
| B | Order by `NormalizedDoc.Score` descending | **SELECTED** as PRIMARY key (stable Go sort) |
| C | Order by intent confidence × score | Rejected — IR-001 confidence is a per-query value, not per-doc; mixing scales muddles semantics |
| D | Defer to IDX-001 RRF | Rejected for the fanout output — fanout's output is RRF's input, but the caller (CLI `usearch query` per CLI-001 today) consumes pre-RRF docs and needs a deterministic order |

[HARD] Final ordering rule: PRIMARY `Score` descending; TIE-BREAKER adapter
name (= input order, lexicographic); SECONDARY TIE-BREAKER `RetrievedAt`
descending (newer first when scores AND adapters tie).

### 3.6 Error Categorisation in Fanout

The fanout layer interprets categorised errors from `*types.SourceError`
(`pkg/types/errors.go:14-120`) but does NOT retry. Retry orchestration is
deferred (Open Question §6.4 — possible scope for a future
SPEC-FAN-001a).

For v0.1, fanout treats every error category equivalently:

- The doc slice from a failing adapter is empty.
- The error is recorded in `Result.AdapterErrors[name]`.
- The fanout returns success at the call level; caller inspects per-adapter
  state.

Rationale: Adding retry policy now bloats the SPEC; the placeholder
`runFanout` does not retry today and there's no measured pain. ADP-001
spec.md §1 (HISTORY 2026-04-26 D3) explicitly assigns retry policy to
SPEC-FAN-001 — but the SPEC author can defer the retry sub-feature to
v0.2 if v0.1 surface gets too wide.

### 3.7 Panic Handling

If an adapter panics inside `Search`, the goroutine crashes by default. The
errgroup library does NOT recover panics. Three options:

| Option | Description | Decision |
|--------|-------------|----------|
| A | Per-goroutine `defer recover()` that converts panic to `*types.SourceError{Category: CategoryUnknown, Cause: errors.New("adapter %q panicked: %v", name, r)}` | **SELECTED** for v0.1 |
| B | Let panics crash the process | Rejected — one bad adapter takes down `usearch query` |
| C | Adopt `sourcegraph/conc` for built-in recovery | Rejected per §2.4 |

The recovered panic is logged at WARN with stack trace via `runtime/debug.Stack()`.

---

## 4. Integration Surfaces

### 4.1 CLI (Today)

The CLI consumer is `cmd/usearch/query.go`. The migration path:

1. SPEC-FAN-001 v0.1 adds `internal/fanout/Fanout` struct + `Dispatch`
   method.
2. The placeholder `runFanout` in `cmd/usearch/query.go:324-368` is
   deleted; the call site at `cmd/usearch/query.go:208`
   (`docs, adapterErrs := runFanout(...)`) replaces with
   `result, err := fanout.Dispatch(ctx, decision)`.
3. The CLI's `runFanout`-specific `@MX:ANCHOR`/`@MX:WARN` annotations
   (`cmd/usearch/query.go:316,320`) are removed.
4. CLI tests (`cmd/usearch/query_test.go`,
   `cmd/usearch/integration_test.go`) update to use the new shape.

The fanout package's shape is a struct — `cmd/usearch/main.go` constructs
it once at process start and reuses it across CLI invocations. The
`buildProductionRegistry` helper at `cmd/usearch/query.go:484-499`
expands to also wire the fanout instance.

### 4.2 MCP (Tomorrow)

SPEC-MCP-001 (M7, `.moai/project/roadmap.md:91`) will expose `usearch query`
as an MCP tool. The MCP server constructs one `*fanout.Fanout` on startup
and reuses it across tool calls. The fanout's concurrent-safe contract
(REQ-FAN, see §1.8 above and the SPEC's REQ-FAN-009 below) makes this
straightforward.

### 4.3 HTTP API (Future)

SPEC-API-001 (deferred per IR-001 §2.2) will expose `usearch query` over
HTTP. Same construction pattern as MCP — single fanout instance, reused
across N concurrent HTTP requests. The N-goroutines × N-adapters scenario
is the worst-case race-detector workload that SPEC-FAN-001's NFR-FAN-002
prescribes.

---

## 5. Race / Leak / Cancellation Analysis

### 5.1 Goroutine Lifetime

For one fanout call against an `AdapterSet` of size K, the call spawns:

- 1 supervisor goroutine (the calling goroutine itself running `Dispatch`)
- K worker goroutines (one per adapter, gated by `errgroup.SetLimit(N)`
  where N defaults to 8)

Cleanup discipline:

- Every worker goroutine has a `defer` to release the `FanoutInflight`
  gauge increment.
- Every worker goroutine has a `defer recover()` for panic-to-error
  conversion.
- The supervisor calls `errgroup.Wait()` BEFORE returning. This is the
  goroutine-leak-prevention contract — `goleak.VerifyNone` after
  `Dispatch` returns SHALL succeed.

### 5.2 Context Cancellation

Three context layers:

1. Caller's parent ctx — the overall budget. Set by CLI to
   `min(--timeout flag, query Deadline)`.
2. Errgroup-derived ctx — used internally by errgroup for first-error
   cancellation. Fanout SUPPRESSES per-adapter errors (returns nil from
   the errgroup task) so this ctx only cancels via the parent ctx
   propagation.
3. Per-adapter ctx — `context.WithTimeout(parentCtx, perAdapterDeadline)`.
   This is the ctx passed to `adapter.Search`. Two cancellation paths:
   (i) the adapter's `perAdapterDeadline` fires (most common), or
   (ii) the parent ctx is cancelled first (caller hit overall deadline).

When the parent ctx is cancelled mid-flight:

- All in-flight per-adapter ctxs are also cancelled (Go context
  inheritance).
- Each adapter's `Search` returns whatever it has (per ADP-001 REQ-ADP-005,
  the adapter wraps `context.DeadlineExceeded` as
  `*SourceError{Category: CategoryUnavailable}` when it crosses the
  network boundary; otherwise returns the bare ctx error).
- Workers complete their result-collection step quickly and exit.
- Errgroup.Wait returns. Supervisor merges partial results, returns
  what's available.

### 5.3 Race Detector Workload

SPEC-FAN-001's NFR-FAN-002 mandates `go test -race ./internal/fanout/...`
clean under the following workload:

- 50 caller goroutines, each invoking `Dispatch(ctx, decision)` on the
  SAME `*Fanout` instance.
- Each call iterates 5 adapter names from a stub registry.
- Each goroutine runs 100 calls.

Total: 50 × 100 × 5 = 25,000 adapter Search invocations, distributed
across ~250,000 short-lived goroutines.

The reference test pattern is `internal/router/router_test.go::TestClassifyConcurrent`
(50 goroutines × 20 calls per IR-001 spec.md HISTORY) and ADP-001's
`TestSearchConcurrentSafe` (50 goroutines × 1 call per spec.md REQ-ADP-011).

### 5.4 Leak Verification

`go.uber.org/goleak v1.3.0` is already pinned as indirect (`go.mod:30`).
Three ways FAN-001 will use it:

- `TestMain(m *testing.M)` in `internal/fanout/bench_test.go` calls
  `goleak.VerifyTestMain(m)`. Pattern matches `internal/adapters/reddit/bench_test.go`.
- `TestDispatchNoGoroutineLeakOnCancel` invokes `Dispatch` with a ctx
  cancelled mid-flight, then asserts `goleak.VerifyNone(t)` returns nil.
- `TestDispatchNoGoroutineLeakOnAdapterPanic` injects a stub adapter that
  panics; asserts panic is captured in `Result.AdapterErrors` AND no
  goroutine leaks.

### 5.5 Channel Discipline

If the chosen result-collection mechanism uses channels (one alternative
to the `[]result` slice indexed by position), the channel MUST be:

- Buffered to size K (= AdapterSet length), so workers never block on
  send.
- Closed by the supervisor AFTER `errgroup.Wait()` returns.
- Drained by the supervisor in a `range` loop.

For v0.1, the SPEC adopts the placeholder's pattern (per-index `[]result`
slice; no channel) because it's simpler and the existing code already uses
it. Channel-based pipeline is OQ §6.6.

---

## 6. Open Questions (numbered)

These are explicitly UNRESOLVED at SPEC-approval time. They do NOT block
SPEC approval. Each has a recommended default and a one-line resolution
owner. The plan-auditor is invited to challenge any of these.

### 6.1 `Capabilities.RecommendedTimeout` field addition

Should `pkg/types/capabilities.go::Capabilities` get a new
`RecommendedTimeout time.Duration` field so each adapter declares its
preferred per-call deadline?

**Recommended default**: NO in v0.1. Hardcode 8s default in fanout
Options. Adopt as a follow-up SPEC-FAN-001a if measured pain (e.g.,
SearXNG bridge needs 20s while Reddit needs 5s).

**Resolution owner**: SPEC-FAN-001a author after M3 traffic.

### 6.2 Default concurrency limit

Should `Options.MaxParallel` default to 8, 12, or unlimited?

**Recommended default**: 8 in v0.1. Rationale: M3 has 12+ adapters;
running all 12 in parallel saturates outbound socket pool on default
`http.DefaultTransport.MaxIdleConnsPerHost = 2`. Capping at 8 leaves
headroom for unrelated outbound calls (LLM, synthesis). Configurable
via `Options.MaxParallel`.

**Resolution owner**: SPEC-FAN-001 author after first M3 load test.

### 6.3 Dedup key — URL alone vs URL+hash

When two `NormalizedDoc`s share canonicalized URL but differ in Title or
Body (e.g., Reddit edited the title after HN crossposted), should fanout
treat them as duplicates?

**Recommended default**: YES — URL alone is the dedup key. Different
content under the same URL is unusual; the user sees one canonical entry
and the cite layer (SPEC-SYN-002) preserves provenance per claim. Open
Question kept in SPEC for plan-auditor challenge.

**Resolution owner**: SPEC-SYN-003 (M4 dedup + clustering) author may
override if telemetry shows false-merges hurting citation accuracy.

### 6.4 Retry policy ownership

ADP-001 spec.md §1 HISTORY 2026-04-26 D3 assigns retry orchestration to
SPEC-FAN-001. This SPEC defers retry to v0.2. Should v0.1 ship with NO
retry, or with a minimal "retry once on `ErrTransient`" policy?

**Recommended default**: NO retry in v0.1. Rationale: M3 has zero retry
machinery today; adding retry without a well-tested backoff/jitter scheme
risks thundering herd against rate-limited sources. v0.2 introduces
exponential backoff per adapter Capabilities.RateLimitPerMin (already
declared at `pkg/types/capabilities.go:57`).

**Resolution owner**: future SPEC-FAN-001-RETRY (M3 follow-up) author.

### 6.5 Adopt `sourcegraph/conc` for panic recovery + typed results

Pre-1.0 dependency risk vs. cleaner code. Currently rejected.

**Recommended default**: NO in v0.1. Revisit after sourcegraph/conc
reaches 1.0 (target was March 2023 per package docs; if still pre-1.0 by
v1.0 of usearch, rejection stands).

**Resolution owner**: SPEC-DEP-001 owner periodic dependency review.

### 6.6 Channel-based result collection

The SPEC adopts the index-slice pattern (`results[i] = ...`). A buffered
channel-based pattern is more idiomatic Go. Performance-wise: channel
ops add ~50ns/op; for K=12 adapters this is 600ns total — negligible
compared to network I/O.

**Recommended default**: index-slice in v0.1 (matches placeholder; less
ceremony). Channel pattern is a refactor for v0.2 if integrating with
streaming consumer (SPEC-SYN-004).

**Resolution owner**: SPEC-SYN-004 author may request channel for
streaming-friendliness.

### 6.7 Per-adapter circuit-breaker integration

`internal/llm` ships a circuit-breaker per provider (cited in IR-001
spec.md REQ-IR-003 — `errors.Is(llmErr, llm.ErrAllProvidersFailed)`).
Should fanout also short-circuit a flapping adapter?

**Recommended default**: NO in v0.1. Adapter health-state tracking is
SPEC-EVAL-002's domain (M8 per `.moai/project/roadmap.md:102`). Fanout
calls every adapter in `AdapterSet`; the registry is the only source of
truth for "is this adapter live."

**Resolution owner**: SPEC-EVAL-002 author may add a registry-level
disable flag that fanout consults.

### 6.8 `FanoutInflight{adapter_class}` label semantics

The pre-registered Gauge labels by `adapter_class`. SPEC-FAN-001 must
choose between (a) reusing IR-001 categories (`web`/`social`/`academic`/
`korean`/`mixed`/`unknown`, 6 values) or (b) using literal adapter names
(12+ values, duplicating `AdapterCalls{adapter}` cardinality).

**Recommended default**: (a) — Use IR-001 Category as the
`adapter_class` value, derived from `RoutingDecision.Category` passed to
`Dispatch`. Bounded cardinality (6 values), aligns with the Router
contract.

**Resolution owner**: SPEC-FAN-001 author bakes this into the SPEC.

---

## 7. Sources and Citations

### External URLs (WebFetch verified 2026-05-04)

- https://pkg.go.dev/golang.org/x/sync/errgroup — Go errgroup package
  documentation; `WithContext` / `Go` / `Wait` / `TryGo` / `SetLimit`
  semantics quoted in §2.1.
- https://pkg.go.dev/golang.org/x/sync/semaphore — Go semaphore package
  documentation; `Weighted` / `Acquire` / `Release` quoted in §2.2.
- https://pkg.go.dev/github.com/sourcegraph/conc — sourcegraph/conc package
  documentation; `pool.NewWithResults` / `WaitAndRecover` / structured-
  concurrency philosophy quoted in §2.3.

### Internal Files (file:line cited)

- `pkg/types/adapter.go:28-45` — Adapter interface (4 methods).
- `pkg/types/query.go:18-44` — Query struct + Filter.
- `pkg/types/capabilities.go:38-62` — Capabilities (10 fields, no
  RecommendedTimeout).
- `pkg/types/normalized_doc.go:40-106` — NormalizedDoc (15 fields),
  Validate, CanonicalHash.
- `pkg/types/errors.go:14-218` — SourceError taxonomy, Category enum,
  CategorizeError, OutcomeFromError.
- `internal/adapters/registry.go:75-167` — Registry lifecycle (Register,
  Get, List).
- `internal/adapters/registry.go:172-263` — wrappedAdapter
  sole-emitter pattern.
- `internal/router/routing_decision.go:23-37` — RoutingDecision shape.
- `internal/router/router.go:258-294` — selectAdapterSet algorithm.
- `internal/router/category.go:90-111` — CategoryEligibleDocTypes
  (input that produces the AdapterSet fanout consumes).
- `internal/obs/metrics/metrics.go:89-95` — FanoutInflight gauge
  registration.
- `internal/obs/metrics/metrics.go:139` — Gauge pre-init for "web"
  adapter_class.
- `internal/obs/metrics/metrics.go:171` — `adapter_class` in cardinality
  allowlist.
- `internal/fanout/fanout.go:1-3` — current 4-line stub.
- `cmd/usearch/query.go:316-368` — placeholder `runFanout` with @MX
  annotations marking SPEC-FAN-001 replacement target.
- `cmd/usearch/query.go:208` — call site
  (`docs, adapterErrs := runFanout(...)`).
- `cmd/usearch/integration_test.go:100-118` — E2E stub-server pattern
  template for FAN-001 integration tests.
- `internal/adapters/reddit/bench_test.go` — `goleak.VerifyTestMain`
  reference pattern.
- `.moai/specs/SPEC-CORE-001/spec.md` — Adapter / NormalizedDoc /
  SourceError contract.
- `.moai/specs/SPEC-IR-001/spec.md` — `RoutingDecision` shape, Category
  enum, REQ-IR-008 AdapterSet selection.
- `.moai/specs/SPEC-IR-001/spec.md:705` — IR-001 spec § REQ-IR-008
  documents `RoutingDecision.AdapterSet` lexicographic-sort guarantee.
- `.moai/specs/SPEC-ADP-001/spec.md:373-374` — REQ-ADP-011 adapter
  concurrent-safety contract that fanout depends on.
- `.moai/specs/SPEC-OBS-001/spec.md` — observability bundle, cardinality
  discipline, `adapter_class` label.
- `.moai/project/roadmap.md:47` — M3 row "SPEC-FAN-001 | Multi-source
  fanout | goroutine pool, per-adapter timeout, partial-result
  assembly, deduplication".
- `.moai/project/roadmap.md:117-128` — M3 parallelization plan.
- `.moai/project/roadmap.md:150` — M3 exit criterion ("`usearch query`
  returns fused results across ≥5 adapters").
- `.moai/project/structure.md:17` — `internal/fanout/` reservation.
- `.moai/project/structure.md:160` — `pkg/types` SDK boundary clause
  (justifies B-over-A choice in §1.7).
- `.moai/config/sections/quality.yaml` — `development_mode: tdd`,
  `test_coverage_target: 85`.
- `.moai/config/sections/language.yaml` — `documentation: en`,
  `code_comments: en`.
- `go.mod:30` — `go.uber.org/goleak v1.3.0` indirect.
- `go.mod:33` — `golang.org/x/sync v0.20.0` indirect.

---

End of Research Document.

**Summary for SPEC Author**: SPEC-FAN-001 implements a bounded-pool
goroutine fanout dispatcher at `internal/fanout/` consuming
`RoutingDecision.AdapterSet` from SPEC-IR-001. The library choice
is `golang.org/x/sync/errgroup` with `SetLimit(N)` plus the
suppress-error idiom (errgroup's first-error cancel is harmful for
fan-out-fan-in; semaphore is unnecessary; sourcegraph/conc is
pre-1.0). The per-adapter timeout is a hardcoded fanout default
(8s, configurable) with `Query.Deadline` honoured first; the
`Capabilities.RecommendedTimeout` field addition is deferred per OQ §6.1
to keep `pkg/types` stable. Partial-result assembly returns whatever
completed before the parent ctx fires. Deduplication uses URL
canonicalization (8 rules in §3.4.1) as the primary dedup key with
`NormalizedDoc.CanonicalHash()` as a tie-breaker. Result ordering is
Score-descending, then adapter-name ascending, then RetrievedAt
descending. Observability uses the pre-registered
`FanoutInflight{adapter_class}` Gauge with IR-001 Category as the
class value (OQ §6.8 resolves to bounded 6-value cardinality); zero
new metric families. Race-safety is verified by `TestDispatchConcurrent`
(50 goroutines × 100 calls × 5 adapters = 25K adapter invocations,
race-clean). Goroutine-leak verification via `goleak.VerifyTestMain` +
mid-flight cancel test. Eight Open Questions are deferred for plan-auditor
challenge: (1) Capabilities.RecommendedTimeout field, (2) default
concurrency, (3) dedup key choice, (4) retry policy ownership,
(5) sourcegraph/conc adoption, (6) channel-based collection, (7) per-
adapter circuit breaker, (8) FanoutInflight label semantics. Zero new
Go module dependencies — pure stdlib + already-pinned errgroup +
goleak. The SPEC body should target ~700-900 lines covering 11-13 EARS
REQs (10 P0 + 1-2 P1 mix) + 4 NFRs.
