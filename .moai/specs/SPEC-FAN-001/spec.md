---
id: SPEC-FAN-001
title: Multi-source Fanout
version: 0.1.0
milestone: M3 — Fanout, adapters, index
status: approved
priority: P0
owner: expert-backend
methodology: tdd
coverage_target: 85
created: 2026-05-04
updated: 2026-05-05
author: limbowl
issue_number: null
depends_on: [SPEC-CORE-001, SPEC-IR-001, SPEC-ADP-001, SPEC-ADP-002, SPEC-OBS-001]
blocks: [SPEC-ADP-003, SPEC-ADP-004, SPEC-ADP-005, SPEC-ADP-006, SPEC-ADP-007, SPEC-ADP-008, SPEC-ADP-009, SPEC-IDX-001, SPEC-CACHE-001]
status_history: |
  draft (2026-05-04) → approved (2026-05-05) after plan-auditor cycle-2 confirmed zero HIGH residuals.
---

# SPEC-FAN-001: Multi-source Fanout

## HISTORY

- 2026-05-05 (iteration 3 — plan-auditor cycle 2, status APPROVED):
  Cycle-2 audit confirmed all 3 cycle-1 HIGH concerns CLOSED via the
  iteration-2 fixes (H1 worker map writes, H15 Query.Deadline
  contract, H18 errgroup deadlock). Cycle-2 surfaced 2 fresh MEDIUM
  concerns: (N1) the eg.Go race window between `ctx.Err()` check and
  `eg.Go` invocation; addressed by adding a "Race window note"
  paragraph in §2.5 documenting the worst-case wait bound (≤
  perAdapterTimeout, never infinite). (N7) the original Stats
  invariant text mentioned a "debug-build-only panic" without a
  documented build-tag mechanism; revised to compute Stats as a
  structural property in `assembleResult` with no runtime assertion
  required. Five LOW concerns (N3 same-(Score, SourceID, RetrievedAt)
  test scenario naturalness, N4 bench MaxParallel implicit
  assumption, N5 race-detector tolerance numeric, N8 "BEFORE every"
  vs "BEFORE next" phrasing, N10 HISTORY count reconciliation) are
  documented but not blocking. Status flipped from `draft` to
  `approved`. Zero HIGH residuals.

- 2026-05-05 (iteration 2 — plan-auditor cycle 1, limbowl via manager-spec):
  Audit identified 3 HIGH and 5 MEDIUM concerns; all addressed inline
  in this revision. HIGH fixes: (H1) §2.6 added explicit
  per-worker-no-shared-state contract — workers write to per-index
  pre-allocated `[]error` and `[][]NormalizedDoc` slices, NEVER
  directly to a `map[string]error`; the supervisor goroutine builds
  `Result.AdapterErrors` from the per-index slices AFTER `eg.Wait()`
  returns. REQ-FAN-002 acceptance text reworded to remove the "into
  Result.AdapterErrors[name]" language that suggested direct map
  writes. (H15) §2.7 added explicit `Query.Deadline` contract clause
  — Fanout DOES NOT consume `Query.Deadline`; the caller MUST apply it
  to the parent ctx via `context.WithDeadline` BEFORE invoking
  `Dispatch`, matching the documented contract at
  `pkg/types/query.go:32-34`. New REQ-FAN-013 surfaces this as a
  testable pre-condition. (H18) §2.5 added explicit guard clause for
  `errgroup.SetLimit` deadlock under cancelled-ctx-with-queued-workers:
  `Dispatch` SHALL check `ctx.Err()` BEFORE every `eg.Go` call and
  short-circuit by recording `context.Canceled` for un-launched
  adapters. New REQ-FAN-012 makes the guard testable. MEDIUM fixes:
  (H2) REQ-FAN-013 acceptance covers the already-cancelled parent ctx
  path. (H7) REQ-FAN-006 reclassified Event-Driven (was Optional);
  Optional pattern is removed (5 EARS patterns no longer all required;
  the Unwanted REQ-FAN-008 + State-Driven REQ-FAN-004/009 + Event-Driven
  REQ-FAN-002/003/005/006/011 + Ubiquitous REQ-FAN-001/007/010 = 4
  patterns covered, which is acceptable per `.claude/rules/moai/...`
  EARS guidance). (H11) §2.3 dedup algorithm clarifies mixed
  valid/invalid URL case — a doc with parseable URL and a doc with
  unparseable URL produce DIFFERENT dedup keys and are NEVER merged;
  acceptance test added. (H14) Stats invariant added to §2.6: "For
  every successful Dispatch call, `Stats.AdapterCount = len(decision.AdapterSet)`
  AND `Stats.SuccessCount + Stats.ErrorCount = Stats.AdapterCount`."
  (H16/H17) Renamed `ErrEmptyAdapterSet` to TWO distinct sentinels:
  `ErrAdapterRegistryEmpty` (for `New` failures, mirrors IR-001
  naming) and `ErrEmptyAdapterSet` (for `Dispatch` failures);
  acceptance criteria updated. `Result.AdapterErrors` is now spec'd
  as nil when `Stats.ErrorCount == 0`, non-nil map otherwise.
  Total: 13 REQs (was 11; +REQ-FAN-012 ctx-cancellation guard,
  +REQ-FAN-013 Query.Deadline caller-honour). 4 NFRs unchanged. 32
  tests (was 30; +TestDispatchAlreadyCancelledCtx,
  +TestDispatchCancelledMidQueue). Status remains `draft` until
  cycle-2 audit confirms zero HIGH residual.

- 2026-05-04 (initial draft v0.1, limbowl via manager-spec):
  First EARS-formatted SPEC for the M3 fanout layer. Drafted after deep
  research into the existing-code state (`.moai/specs/SPEC-FAN-001/research.md`,
  every claim file:line-cited or URL-cited). Builds on SPEC-CORE-001 (the
  `pkg/types.Adapter` 4-method contract, `*types.SourceError` taxonomy with
  four Categories, `internal/adapters.Registry` with sole-emitter
  `wrappedAdapter` at `internal/adapters/registry.go:172-263`),
  SPEC-IR-001 (`RoutingDecision.AdapterSet` shape at
  `internal/router/routing_decision.go:23-37`, IR-001 REQ-IR-008
  lexicographic-sort guarantee), SPEC-OBS-001 (pre-registered
  `usearch_fanout_goroutines_inflight{adapter_class}` Gauge at
  `internal/obs/metrics/metrics.go:89-95`), SPEC-ADP-001 (REQ-ADP-011
  concurrent-safety contract at spec.md:373-374), and SPEC-ADP-002
  (HN adapter symmetric to ADP-001).

  User-locked decisions baked in:

  - **D1 Concurrency primitive**: `golang.org/x/sync/errgroup` with
    `SetLimit(N)` plus the suppress-error idiom (workers return `nil`
    even on adapter error so first-error cancellation does NOT kill
    other adapters). Already pinned at `go.mod:33`. Rejected
    alternatives: `golang.org/x/sync/semaphore` (no benefit beyond
    SetLimit), `github.com/sourcegraph/conc` (pre-1.0 dependency
    risk; revisit per OQ §11.5). Research §2 + §3.1.
  - **D2 Per-adapter timeout policy**: Hardcoded 8-second default in
    fanout `Options.PerAdapterTimeout`, with `Query.Deadline` (the
    caller's overall budget) taking precedence — per-adapter ctx
    inherits `min(perAdapterDeadline, remainingTimeToParent)`. The
    adding of a `Capabilities.RecommendedTimeout` field is DEFERRED
    to a future SPEC-FAN-001a (research OQ §6.1) so v0.1 keeps
    `pkg/types` untouched per the SDK boundary clause at
    `.moai/project/structure.md:160`. Research §3.2.
  - **D3 Partial-result assembly**: When the parent ctx fires before
    all adapters return, fanout collects whatever completed, records
    incomplete adapters in `Result.AdapterErrors[name]` with
    `context.DeadlineExceeded` (or the nested adapter error if the
    adapter wrapped the cancellation), and returns success at the
    call level. The caller inspects per-adapter state. No streaming
    in v0.1 (deferred to SPEC-SYN-004 M4). Research §3.3.
  - **D4 Deduplication strategy**: URL canonicalization (8 rules per
    research §3.4.1: lowercase host + scheme, strip fragment, strip
    11 well-known tracking params, trim trailing slash, sort query
    params, leave path case + percent-encoding alone) is the PRIMARY
    dedup key. `NormalizedDoc.CanonicalHash()` (`pkg/types/normalized_doc.go:91-106`)
    is the SECONDARY tie-breaker for differing-URL same-content cases.
    Same-URL different-content → first occurrence wins (preserves
    earliest `RetrievedAt` adapter's view). Research §3.4.
  - **D5 Result ordering**: PRIMARY `NormalizedDoc.Score` descending;
    TIE-BREAKER adapter name ascending (= input AdapterSet order, which
    IR-001 REQ-IR-008 already lexicographically sorts); SECONDARY
    TIE-BREAKER `RetrievedAt` descending (newer first). Stable Go sort.
    Research §3.5.
  - **D6 Error categorisation**: v0.1 does NOT retry. Every category of
    `*types.SourceError` is treated equivalently — the failing adapter's
    docs are empty, the error appears in `Result.AdapterErrors[name]`,
    fanout returns success. Retry orchestration is DEFERRED to a
    future SPEC-FAN-001-RETRY (research OQ §6.4) — v0.1 keeps the
    SPEC surface narrow. Research §3.6.
  - **D7 Panic handling**: Per-goroutine `defer recover()` converts
    panics into `*types.SourceError{Adapter: name, Category:
    CategoryUnknown, Cause: fmt.Errorf("adapter %q panicked: %v",
    name, r)}`. Stack trace is logged at WARN via
    `runtime/debug.Stack()`. One bad adapter cannot crash
    `usearch query`. Research §3.7.
  - **D8 Observability**: Reuses the pre-registered
    `FanoutInflight{adapter_class}` Gauge with `adapter_class` set to
    the IR-001 Category value (one of `web`/`social`/`academic`/
    `korean`/`mixed`/`unknown`, bounded 6-value cardinality —
    research OQ §6.8). ZERO new metric families: per-adapter
    counter/histogram inherited from the registry's wrappedAdapter
    (sole-emitter discipline preserved). One OTel parent span
    `fanout.dispatch` containing each adapter's `adapter.search` span as
    a child. One slog summary record at the end of `Dispatch` with
    `request_id`, `adapter_count`, `result_count`, `errors_count`,
    `dedup_dropped`, `elapsed_seconds`. Research §1.4 + §1.5.

  Resolved discrepancy: the placeholder `runFanout` at
  `cmd/usearch/query.go:324-368` already uses errgroup but ships with
  no concurrency cap, no per-adapter timeout, and no deduplication.
  SPEC-FAN-001 retires it and migrates the call site at
  `cmd/usearch/query.go:208` to the new `internal/fanout` package.
  The `@MX:ANCHOR` and `@MX:WARN` annotations on `runFanout`
  (`cmd/usearch/query.go:316`, `cmd/usearch/query.go:320-323`)
  document this replacement target explicitly.

  11 EARS REQs (10 × P0 + 1 × P1) covering all five EARS patterns
  (Ubiquitous, Event-Driven, State-Driven via REQ-FAN-009 concurrency
  contract, Optional via REQ-FAN-006 dedup tie-breaker, Unwanted via
  REQ-FAN-008 empty AdapterSet rejection), 4 NFRs (NFR-FAN-001
  fanout-overhead p50/p95, NFR-FAN-002 race-clean concurrent
  invocation, NFR-FAN-003 zero goroutine leaks, NFR-FAN-004 alloc/op
  ceiling on hot path), 8 Open Questions carried forward from
  research.md §6 for plan-auditor challenge. Zero new Go module
  dependencies — pure stdlib (`context`, `errors`, `fmt`, `net/url`,
  `runtime/debug`, `sort`, `strings`, `sync`, `time`) plus existing
  `golang.org/x/sync/errgroup`, `pkg/types`, `internal/adapters`,
  `internal/obs/metrics`, `internal/obs/reqid`, `internal/router`
  (for the Category type — fanout consumes `RoutingDecision`
  by value, no router.Router dependency at runtime), and
  `go.opentelemetry.io/otel/{attribute,codes,trace}` (already
  pinned via SPEC-OBS-001).

  Insertion point: M3 gateway SPEC. SPEC-IDX-001 (M3 RRF fusion) and
  SPEC-CACHE-001 (M3 5-phase access fallback) consume
  `fanout.Result.Docs`. SPEC-ADP-003 through SPEC-ADP-009 (the seven
  M3 adapter SPECs) gate on FAN-001 because the wedge for 7-way
  parallelization is "FAN-001 has been merged so we know the dispatch
  contract" (`.moai/project/roadmap.md:122-123`).

  Harness level: standard (single domain, ≤10 source files in
  `internal/fanout/`, no security/payment/PII keywords, no compose/
  env/config deltas beyond `.moai/config/sections/fanout.yaml` for
  default tunables — see §6.7). Sprint Contract optional. Ready for
  plan-auditor review and annotation cycle.

---

## 1. Purpose

SPEC-CORE-001 published the typed adapter contract (`pkg/types.Adapter`
4-method interface at `pkg/types/adapter.go:28-45`,
`pkg/types.NormalizedDoc` 15-field canonical struct at
`pkg/types/normalized_doc.go:40-56`, `*types.SourceError` taxonomy with
four Categories at `pkg/types/errors.go:14-218`, `pkg/types.Capabilities`
descriptor at `pkg/types/capabilities.go:38-62`) and the
`internal/adapters.Registry` with its sole-emitter `wrappedAdapter`
(`internal/adapters/registry.go:172-263`). SPEC-IR-001 published the
`RoutingDecision.AdapterSet` shape at
`internal/router/routing_decision.go:23-37` — IR-001's
`selectAdapterSet` algorithm (`internal/router/router.go:258-294`)
intersects category-eligible DocTypes with capability-language
compatibility and returns the lexicographically-sorted adapter name
slice. SPEC-ADP-001 (Reddit) and SPEC-ADP-002 (Hacker News) implemented
the contract end-to-end, both with explicit concurrent-safety guarantees
(REQ-ADP-011 spec.md:373-374 — 50 goroutines × one Search per `*Adapter`
race-clean). SPEC-OBS-001 pre-registered the
`usearch_fanout_goroutines_inflight{adapter_class}` Gauge at
`internal/obs/metrics/metrics.go:89-95` with `adapter_class` already in
the cardinality allowlist at `internal/obs/metrics/metrics.go:171`.

The `internal/fanout/` package is currently a 4-line stub
(`internal/fanout/fanout.go:1-3`); the working CLI ships a placeholder
private helper `runFanout` at `cmd/usearch/query.go:324-368` with two
`@MX` annotations marking the replacement target.

SPEC-FAN-001 fills `internal/fanout/` with the **multi-source dispatch
orchestrator** that consumes `RoutingDecision` and returns
`fanout.Result{Docs, AdapterErrors, Stats}`. The fanout layer:

1. Builds a bounded goroutine pool sized by `Options.MaxParallel` (default
   8, configurable per OQ §11.2) using `errgroup.SetLimit(N)`.
2. For each adapter name in `RoutingDecision.AdapterSet`, retrieves the
   `*wrappedAdapter` from the registry, derives a per-adapter
   `context.WithTimeout(parentCtx, perAdapterDeadline)` capped by both
   the fanout's `Options.PerAdapterTimeout` (default 8s) and the
   remaining time to `Query.Deadline`, and invokes `Search`.
3. Collects partial results when the parent ctx fires before all
   adapters return. Adapters that completed contribute their docs;
   adapters that timed out have `Result.AdapterErrors[name] =
   context.DeadlineExceeded` (or the wrapped `*SourceError` if the
   adapter normalised the cancellation per ADP-001 REQ-ADP-005).
4. Deduplicates the merged `[]NormalizedDoc` slice using URL
   canonicalization (PRIMARY key, 8 rules) plus
   `NormalizedDoc.CanonicalHash()` (SECONDARY tie-breaker for the
   different-URL-same-content edge case). Same-URL different-content
   collisions are resolved first-occurrence-wins (newest
   `RetrievedAt` actually loses; the dedup output is stable).
5. Sorts the deduped slice by `Score` descending, with adapter-name
   ascending and `RetrievedAt` descending as deterministic
   tie-breakers.
6. Emits per-call observability: `FanoutInflight{adapter_class}` Gauge
   inc/dec around each adapter dispatch (NOT around the overall
   fanout — the gauge measures concurrent goroutines, not in-flight
   fanout calls). One OTel parent span `fanout.dispatch` with
   attributes `fanout.category`, `fanout.adapter_count`,
   `fanout.result_count`, `fanout.errors_count`,
   `fanout.dedup_dropped`. Each adapter's span is a child via the
   registry's wrappedAdapter (no new span here). One slog summary
   record at end of `Dispatch`.

The fanout does NOT classify (SPEC-IR-001 owns), does NOT retry
(deferred per D6), does NOT cache responses (SPEC-CACHE-001 M3 owns
the 5-phase access fallback for blocked sources, but in-process LRU
on the *successful* path is post-V1), does NOT fuse rankings across
adapters (SPEC-IDX-001 RRF M3 owns), does NOT synthesize answers
(SPEC-SYN-001/002/003 owns), does NOT emit any per-adapter
counter/histogram (the registry wrappedAdapter does, sole-emitter
discipline preserved).

Completion unblocks every M3 adapter SPEC (SPEC-ADP-003 through
SPEC-ADP-009 — seven independent SPECs that develop in parallel per
`.moai/project/roadmap.md:123`), the index ingestion SPEC-IDX-001
(consumes `fanout.Result.Docs` for hybrid index population), and the
5-phase access fallback SPEC-CACHE-001 (the fallback wraps fanout
in a try-then-degrade harness). Closes M3's exit-criterion
(`.moai/project/roadmap.md:150` — "`usearch query` returns fused
results across ≥5 adapters").

This is the **gateway SPEC** for M3. The shape laid down here propagates
into every M3 adapter integration test and bounds the contract
SPEC-IDX-001 RRF assumes about its input.

---

## 2. Scope

### 2.1 In-Scope

| Ref | Item |
|-----|------|
| a | `internal/fanout/fanout.go`: `Fanout` struct (immutable post-construction; holds `*adapters.Registry`, `*obs.Obs`, `Options`), `New(opts Options) (*Fanout, error)` constructor (validates registry non-empty, normalises Options defaults), `Dispatch(ctx context.Context, decision router.RoutingDecision, q types.Query) (*Result, error)` method as the sole public entry point. |
| b | `internal/fanout/result.go`: `Result{Docs []types.NormalizedDoc, AdapterErrors map[string]error, Stats Stats}` and `Stats{AdapterCount int, SuccessCount int, ErrorCount int, DedupDropped int, ElapsedSeconds float64}` typed return shape with documented field semantics. JSON-marshalable for diagnostic dumps (the production `usearch query` text/json formatter consumes the `Docs` slice; `AdapterErrors` and `Stats` are diagnostic). |
| c | `internal/fanout/options.go`: `Options{Registry *adapters.Registry, Obs *obs.Obs, MaxParallel int, PerAdapterTimeout time.Duration, DefaultDeadline time.Duration}` with documented zero-value defaults (`MaxParallel=8`, `PerAdapterTimeout=8*time.Second`, `DefaultDeadline=30*time.Second` — matches CLI default at `cmd/usearch/query.go:37`) and validation in `New`. |
| d | `internal/fanout/dispatch.go`: the hot path. Contains the errgroup orchestration, per-adapter ctx derivation, partial-result collection, panic recovery, and the `FanoutInflight` Gauge inc/dec discipline. Splits orchestration from result post-processing for testability. |
| e | `internal/fanout/dedup.go`: `dedupDocs(docs []types.NormalizedDoc) ([]types.NormalizedDoc, int)` returns deduped slice + count of dropped docs. Uses `canonicalURL(rawURL string) (string, error)` (PRIMARY key, 8 rules per research §3.4.1) and falls back to `NormalizedDoc.CanonicalHash()` when the URL fails to parse. Same-URL different-content: first occurrence wins. |
| f | `internal/fanout/sort.go`: `sortDocs(docs []types.NormalizedDoc)` mutates the slice in-place using `sort.SliceStable` so equal-Score docs preserve adapter input order (lexicographic per IR-001 REQ-IR-008). |
| g | `internal/fanout/errors.go`: package-level sentinels (H16 fix splits the original single sentinel into two): `ErrAdapterRegistryEmpty = errors.New("fanout: registry has zero adapters")` (returned ONLY by `New` when registry is nil or empty — mirrors `internal/router.ErrAdapterRegistryEmpty` naming at `internal/router/router.go:95`); `ErrEmptyAdapterSet = errors.New("fanout: empty adapter set")` (returned ONLY by `Dispatch` when `decision.AdapterSet` is empty); `ErrAdapterNotFound = errors.New("fanout: adapter not found in registry")` (wrapped in worker errors when `registry.Get(name)` returns false). |
| h | `internal/fanout/observability.go`: `emitDispatch(ctx, span, result, elapsed)` — one helper that writes the `fanout.dispatch` span attributes and the slog summary record. Mirrors the `internal/router/router.go:341-383::emit` pattern. Nil-safe across `Obs`, `Obs.Metrics`, `Obs.Logger`. |
| i | `internal/fanout/canonical.go`: `canonicalURL(raw string) (string, error)` implementing the 8 normalisation rules from research §3.4.1 (lowercase scheme + host, strip fragment, strip 11 tracking params, trim trailing slash, sort query keys alphabetically, leave path case + percent-encoding intact). Pure function. Returns parse error for malformed URLs (caller falls back to `CanonicalHash`). |
| j | `internal/fanout/fanout_test.go`: orchestration tests (`Dispatch` against a stub registry of 3 adapters; happy path, partial failure, all-failure, empty AdapterSet). |
| k | `internal/fanout/dispatch_test.go`: per-adapter timeout tests; goroutine-leak verification via `goleak.VerifyNone`; panic-recovery test; ctx-cancellation mid-flight test. |
| l | `internal/fanout/dedup_test.go`: URL canonicalization table (15+ inputs spanning all 8 rules); `dedupDocs` table (same-URL same-content, same-URL different-content, different-URL same-content via hash, mixed). |
| m | `internal/fanout/sort_test.go`: ordering table (Score-descending primary, adapter-name ascending tiebreak, RetrievedAt descending secondary). |
| n | `internal/fanout/concurrent_test.go`: race-detector workload (50 caller goroutines × 100 calls × 5 stub adapters = 25,000 invocations; `go test -race` clean). NFR-FAN-002. |
| o | `internal/fanout/bench_test.go`: `BenchmarkDispatch5Adapters` (NFR-FAN-001 + NFR-FAN-004). `TestMain` calls `goleak.VerifyTestMain(m)` (NFR-FAN-003). |
| p | `internal/fanout/options_test.go`: `New` validation tests (nil registry, zero MaxParallel, negative timeout). |
| q | Migration of CLI: deletion of `runFanout` at `cmd/usearch/query.go:324-368`, replacement of call site at `cmd/usearch/query.go:208` to invoke `fanout.Dispatch`. Update of `cmd/usearch/integration_test.go` E2E tests to use the new shape. The placeholder's `@MX:ANCHOR` + `@MX:WARN` annotations (lines 316, 320-323) are removed; a new `@MX:ANCHOR` lands on `fanout.Fanout.Dispatch`. |

### 2.2 Out-of-Scope

[HARD] This SPEC explicitly excludes the following items. Each has a known
destination SPEC; this list prevents scope creep into FAN-001 (the M3
gateway).

- **Retry orchestration** (exponential backoff, jitter,
  per-adapter retry budget keyed on `Capabilities.RateLimitPerMin`).
  → Future SPEC-FAN-001-RETRY (research OQ §6.4). v0.1 ships zero-retry;
  every error category passes through to `Result.AdapterErrors`.
- **Per-adapter circuit breaker** (auto-disable adapter after N
  consecutive failures, auto-re-enable on Healthcheck pass).
  → SPEC-EVAL-002 (M8 per `.moai/project/roadmap.md:102`). The
  registry has no disable flag; fanout calls every name in
  `RoutingDecision.AdapterSet`.
- **Response caching** (in-process LRU keyed on canonical URL,
  Redis-backed, on-disk). → Out of fanout's domain. The 5-phase
  access fallback SPEC-CACHE-001 (M3) handles BLOCKED-source caching
  but on-success caching is post-V1.
- **Result ranking fusion across adapters** (Reciprocal Rank Fusion).
  → SPEC-IDX-001 (M3). Fanout's output is RRF's input. Fanout
  preserves `Score` from each adapter; RRF re-ranks by rank not
  score.
- **Streaming/incremental result delivery** (channel-based, SSE,
  WebSocket). → SPEC-SYN-004 (M4 per
  `.moai/project/roadmap.md:66`). v0.1 returns the complete
  `*Result` once `Dispatch` returns.
- **Adapter health-state machine**. → SPEC-EVAL-002 (M8).
- **`Capabilities.RecommendedTimeout` field addition** (per-adapter
  preferred deadline). → Future SPEC-FAN-001a (research OQ §6.1).
  v0.1 hardcodes the timeout in fanout `Options`.
- **Per-tenant adapter visibility / RBAC**. → SPEC-AUTH-002 (M6 per
  `.moai/project/roadmap.md:82`). Fanout has no notion of user/team.
- **Streaming progress events emitted to caller during Dispatch**.
  → Out of v0.1. The CLI's existing
  `cmd/usearch/query.go::progressEmitter` (line 391-396) emits at
  the call boundary, not from inside fanout.
- **Per-adapter custom Prometheus metrics**. → Would require
  amending SPEC-OBS-001's allowlist. Out of v0.1; the existing
  `AdapterCalls{adapter,outcome}` family covers per-adapter
  observability and `FanoutInflight{adapter_class}` covers fanout
  concurrency.
- **HTTP / gRPC server exposure of fanout**. → SPEC-MCP-001 (M7) and
  future SPEC-API-001. Fanout is a Go library only in v0.1.
- **Dynamic AdapterSet override** (caller passes a name list that
  bypasses IR-001 routing). → The CLI's `--source` flag at
  `cmd/usearch/query.go:189-198` already does the intersect-with-
  filter dance at the CLI layer. Fanout takes the post-filter
  AdapterSet from the caller verbatim.
- **Cardinality allowlist amendment**. ZERO new label names;
  `adapter_class` and `outcome` already allowlisted at
  `internal/obs/metrics/metrics.go:171`.
- **GitHub Issue tracking on this SPEC** (skipped per session
  pattern — orchestrator handles).

### 2.3 Dedup Algorithm Architecture

[HARD] The dedup function in `dedup.go::dedupDocs` is deterministic and
pure (input slice → output slice + drop count, no I/O, no time, no
randomness) so golden tests can compute expected output from input
alone.

**Algorithm**:

1. Iterate the input slice in order.
2. For each doc, compute the dedup key:
   - PRIMARY: `canonicalURL(doc.URL)` if it parses successfully.
   - FALLBACK: `doc.CanonicalHash()` if URL fails to parse (RFC 3986
     parse error, empty URL, scheme-less). The hash is content-only
     per `pkg/types/normalized_doc.go:79-106`.
3. If the key has not been seen, emit the doc and record the key.
4. If the key has been seen, drop the doc and increment the drop
   counter.
5. Return `(deduped slice, drop count)`.

**Same-URL different-content semantics**: first occurrence wins. The
later doc with the same canonical URL but a different title/body is
silently dropped. The CALLER (synthesis) sees one doc per canonical URL.
Provenance for the dropped variant is intentionally lost in v0.1; the
synthesis layer's per-claim citation (SPEC-SYN-002) retains its own
provenance trail per the citation contract.

**Different-URL same-content semantics** (mirror sites, Reddit
crossposts where the URL field is the post's external link not the
permalink, RSS feeds republishing news articles): caught by the
`CanonicalHash` fallback ONLY when the URL parses to something
different. If both URLs parse fine but resolve to the same content,
the hash fallback does not engage — both docs are emitted. SPEC-SYN-003
(M4 dedup + clustering, `.moai/project/roadmap.md:65`) handles this
deeper near-dup case via SimHash + embedding cosine.

**Mixed valid/invalid URL semantics (H11 fix)**: when one doc has a
parseable URL and another doc has an unparseable URL, they live in
DIFFERENT dedup key spaces. The dedup map uses TWO disjoint
namespaces:

- `url:<canonical-url-bytes>` for parseable URLs.
- `hash:<canonical-hash-hex>` for fallback (unparseable URL OR
  empty URL).

The namespace prefix is internal and never exposed to callers — it
exists only inside the dedup map. A doc in the `url:` namespace is
NEVER compared against a doc in the `hash:` namespace, even if the
underlying bytes happen to coincide. This guarantees that
`canonicalURL("https://x.com/a") != canonicalHash(...)` as dedup
keys regardless of the byte representations involved.

REQ-FAN-006 acceptance includes `TestDedupMixedValidInvalidURL` to
make this testable.

**Performance**: `O(n)` time, `O(n)` space for the seen-key map. n is
bounded by `sum_of_per_adapter_MaxResults` (typically ≤ 12 × 25 = 300
docs).

### 2.4 URL Canonicalization Rules

[HARD] `canonicalURL(raw string) (string, error)` implements exactly
these 8 transformations on the parsed `*url.URL` (Go stdlib) before
re-serialising. The output is the dedup key, NOT the displayed URL
(adapter-supplied `NormalizedDoc.URL` is preserved in the returned
slice).

| # | Rule | Source field | Action |
|---|------|--------------|--------|
| 1 | Lowercase scheme | `u.Scheme` | `strings.ToLower(u.Scheme)` |
| 2 | Lowercase host | `u.Host` | `strings.ToLower(u.Host)` |
| 3 | Strip fragment | `u.Fragment` | `u.Fragment = ""` |
| 4 | Strip 11 tracking params | `u.Query()` | Remove keys: `utm_source`, `utm_medium`, `utm_campaign`, `utm_term`, `utm_content`, `gclid`, `fbclid`, `mc_eid`, `mc_cid`, `_ga`, `ref`, `ref_src` |
| 5 | Trim trailing slash from path | `u.Path` | `strings.TrimRight(u.Path, "/")` UNLESS the path equals `"/"` (root path is preserved) |
| 6 | Sort remaining query params alphabetically | `u.RawQuery` | Re-encode `u.Query()` after sorting keys |
| 7 | Preserve path case | `u.Path` | NO CHANGE (HTTP paths are case-sensitive) |
| 8 | Preserve percent-encoding | the entire URL | NO CHANGE (full RFC 3986 normalisation requires a complete encoder pass; out of v0.1 scope) |

**Worked examples** (asserted in `canonical_test.go::TestCanonicalURLTable`
within byte-equal output):

| Input | Output |
|-------|--------|
| `https://www.reddit.com/r/golang/comments/abc/title?utm_source=newsletter&utm_medium=email#section1` | `https://www.reddit.com/r/golang/comments/abc/title` |
| `HTTPS://Example.COM/Path/?b=2&a=1` | `https://example.com/Path?a=1&b=2` |
| `https://news.ycombinator.com/item?id=12345` | `https://news.ycombinator.com/item?id=12345` |
| `https://example.com/?gclid=xyz&fbclid=abc&id=42` | `https://example.com/?id=42` |
| `https://example.com/foo/bar/` | `https://example.com/foo/bar` |
| `https://example.com/` | `https://example.com/` (root preserved) |

**Determinism guarantees**:

- The 11-entry tracking-param list is a package-level `[]string`
  constant; never mutated at runtime.
- The query-key sort uses `sort.Strings` (deterministic by Unicode
  code-point ordering).
- Same input string → byte-equal output, every time.

### 2.5 Per-Adapter Timeout Derivation

[HARD] The per-adapter ctx is derived as:

```
adapterDeadline = min(
    perAdapterTimeout,                 // Options.PerAdapterTimeout (default 8s)
    timeUntil(parentCtx.Deadline())    // remaining caller budget; ∞ if no parent deadline
)
adapterCtx, cancel = context.WithTimeout(parentCtx, adapterDeadline)
```

Properties:

- The PARENT ctx propagation is preserved: cancelling the parent
  cancels every per-adapter ctx via the Go context inheritance graph.
- The per-adapter timeout NEVER exceeds the caller's budget. A caller
  with a 2-second deadline against a fanout configured for 8s
  per-adapter sees adapters time out at 2s, not 8s.
- A caller with NO deadline (`parentCtx.Deadline()` returns
  `(zero, false)`) gets the fanout's `PerAdapterTimeout` floor.
- The `cancel` function MUST be called (via `defer cancel()`) to
  release timer resources — this is a `vet`-checked invariant in Go.

When the per-adapter ctx expires before the adapter returns:

- The adapter's `Search` returns either `context.DeadlineExceeded`
  directly (raw error) OR `*types.SourceError{Category:
  CategoryUnavailable, Cause: context.DeadlineExceeded}` (per
  ADP-001 REQ-ADP-005 wrapping behaviour).
- Fanout's worker captures whichever shape the adapter returned and
  stores it in the per-index `[]error` slice (per §2.6); the
  supervisor copies it into `Result.AdapterErrors[name]` after
  `eg.Wait()` returns.
- The wrappedAdapter's observability emission already classifies this
  via `OutcomeFromError(err)` which maps
  `context.DeadlineExceeded` → `outcome="timeout"`
  (`pkg/types/errors.go:174-193`). No new mapping needed.

[HARD] **Pre-launch ctx guard (H18 fix)**: BEFORE every
`eg.Go(...)` call inside the dispatch loop, the supervisor SHALL
check `ctx.Err()`. If the parent ctx is already cancelled, the
supervisor SHALL skip the `eg.Go` invocation and pre-populate the
per-index error slot with `&types.SourceError{Adapter: name,
Category: CategoryUnavailable, Cause: ctx.Err()}` (which wraps
`context.Canceled` or `context.DeadlineExceeded` as appropriate).
This guard prevents the deadlock case where (a) `errgroup.SetLimit(N)`
has reached its limit (N workers running), (b) the parent ctx is
cancelled, (c) the next `eg.Go` call blocks waiting for a slot but
the SetLimit primitive is NOT ctx-aware (per the Go errgroup docs at
https://pkg.go.dev/golang.org/x/sync/errgroup, `Go` "blocks until the
new goroutine can be added without the number of goroutines in the
group exceeding the configured limit" — no ctx-cancellation
short-circuit). REQ-FAN-012 makes this guard testable.

**Race window note**: The guard narrows but does not fully eliminate
the deadlock — a ctx that becomes cancelled BETWEEN `ctx.Err()` check
and `eg.Go(...)` invocation could still see a brief block. In
practice this window is sub-microsecond (one ctx.Err() comparison
plus one funcall) and the underlying `errgroup.SetLimit` token-channel
will release a slot the moment any in-flight worker exits — workers
exit promptly under cancelled ctx because adapters honour ctx
cancellation per the SPEC-CORE-001 contract (`pkg/types/adapter.go:16`).
The guard is therefore best-effort-bounded; the worst-case wait time
is `min(perAdapterTimeout, time-until-any-worker-exits)`, never
infinite.

### 2.6 Worker Goroutine State Discipline (H1 fix)

[HARD] Worker goroutines spawned by the dispatch loop SHALL NOT
write to any shared `map`. The dispatch hot path uses two
pre-allocated, fixed-size, per-index slices:

```go
perAdapterDocs := make([][]types.NormalizedDoc, len(decision.AdapterSet))
perAdapterErr  := make([]error, len(decision.AdapterSet))
```

The slice header is shared across goroutines (read-only after
allocation), but each worker writes ONLY to its own index `i`
(`perAdapterDocs[i] = docs` / `perAdapterErr[i] = err`). Concurrent
writes to disjoint indexes of the same slice are race-free per the
Go memory model (no overlapping memory writes; the slice header
itself is not mutated post-allocation).

After `eg.Wait()` returns, the supervisor goroutine (the one that
called `Dispatch`) reads the per-index slices, merges
`perAdapterDocs` into `Result.Docs`, and constructs
`Result.AdapterErrors map[string]error` from `perAdapterErr` —
keys are `decision.AdapterSet[i]` for indexes where
`perAdapterErr[i] != nil`. The supervisor performs ALL map writes;
no worker ever touches a map.

[HARD] **Stats invariant (H14 fix)**: For every successful
`Dispatch` call (i.e., the call returns `(*Result, nil)`), the
following SHALL hold by construction:

```
Stats.AdapterCount = len(decision.AdapterSet)
Stats.SuccessCount + Stats.ErrorCount = Stats.AdapterCount
```

A "success" entry is `perAdapterErr[i] == nil`. An "error" entry is
`perAdapterErr[i] != nil`. The two are mutually exclusive at every
index. The supervisor's `assembleResult` helper computes Stats
directly from `len(decision.AdapterSet)` and the per-index counts,
making the invariant a structural property (not a runtime assertion).
Tests assert the invariant explicitly per REQ-FAN-002 / REQ-FAN-003 /
REQ-FAN-011 acceptance text — no debug build tag or production
panic is required.

[HARD] **`Result.AdapterErrors` shape (H17 fix)**: When
`Stats.ErrorCount == 0`, `Result.AdapterErrors` SHALL be `nil`. When
`Stats.ErrorCount >= 1`, `Result.AdapterErrors` SHALL be a non-nil
`map[string]error` with exactly `Stats.ErrorCount` entries. Tests
assert both shapes.

### 2.7 Caller Responsibilities for Deadlines (H15 fix)

[HARD] `Dispatch` does NOT consume `Query.Deadline`. The caller MUST
apply `Query.Deadline` to the parent ctx via
`context.WithDeadline(parentCtx, q.Deadline)` (or
`context.WithTimeout(parentCtx, time.Until(q.Deadline))`) BEFORE
invoking `Dispatch`. This matches the contract documented at
`pkg/types/query.go:32-34`: "Deadline is a soft deadline; the
orchestrator SHOULD honour this via context.WithDeadline before
invoking Search."

`Dispatch` derives per-adapter ctx exclusively from `parentCtx`
(per §2.5). `Query.Deadline` is informational metadata that the
fanout MAY pass through to adapters (it is part of `Query` already);
fanout itself does NOT enforce it.

Rationale:

- Avoids double-application of the deadline (would shrink the
  effective per-adapter window unnecessarily).
- Keeps the contract simple: parent ctx is the single source of
  truth for "how much time do I have."
- Matches the existing CLI behaviour at `cmd/usearch/query.go:140-141`
  (`ctx, cancel := context.WithTimeout(ctx, flags.Timeout)`).

REQ-FAN-013 makes this contract testable.

---

## 3. EARS Requirements

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| REQ-FAN-001 | Ubiquitous | The package `internal/fanout` SHALL expose a `Fanout` struct constructed via `New(opts Options) (*Fanout, error)` and a public method `Dispatch(ctx context.Context, decision router.RoutingDecision, q types.Query) (*Result, error)` that returns a non-nil `*Result` for every non-error invocation. `New` SHALL return `ErrAdapterRegistryEmpty` (sentinel) when `opts.Registry == nil` or `opts.Registry.List()` returns zero entries — distinct from `ErrEmptyAdapterSet` which is returned only by `Dispatch` when `decision.AdapterSet` is empty (REQ-FAN-008). `New` SHALL normalise zero-valued Options fields to documented defaults (`MaxParallel=8`, `PerAdapterTimeout=8*time.Second`, `DefaultDeadline=30*time.Second`). | P0 | `TestNewRequiresRegistry` (nil registry → `errors.Is(err, ErrAdapterRegistryEmpty)`); `TestNewRequiresAtLeastOneAdapter` (empty registry → `errors.Is(err, ErrAdapterRegistryEmpty)`); `TestNewNormalisesDefaults` (zero Options → documented defaults observed via reflection on returned `*Fanout`); `TestDispatchAlwaysReturnsResult` (50 invocations across success/partial/failure paths; assert `result != nil` AND `err` is nil) — all in `fanout_test.go`. |
| REQ-FAN-002 | Event-Driven | WHEN `Dispatch(ctx, decision, q)` is invoked with `len(decision.AdapterSet) >= 1`, the fanout SHALL launch one goroutine per adapter name via `errgroup.SetLimit(opts.MaxParallel)` + `errgroup.Go`, retrieve the adapter via `opts.Registry.Get(name)`, derive a per-adapter ctx via `context.WithTimeout(ctx, perAdapterDeadline)` per §2.5, invoke `adapter.Search(adapterCtx, q)`, and store the `(docs, err)` tuple at the worker's pre-allocated per-index slot per §2.6. After `eg.Wait()` returns, the supervisor goroutine SHALL merge per-index docs into `Result.Docs` and construct `Result.AdapterErrors map[string]error` from per-index error slots (per §2.6 — workers NEVER write to a map directly). Goroutine count at any instant SHALL NOT exceed `opts.MaxParallel`. | P0 | `TestDispatchHappyPath3Adapters` (stub registry of 3 adapters returning 5+5+5 docs; assert `len(result.Docs) >= 12` post-dedup, `result.AdapterErrors == nil` (per §2.6 H17 fix: nil when ErrorCount==0), `result.Stats.SuccessCount == 3`, `result.Stats.SuccessCount + result.Stats.ErrorCount == result.Stats.AdapterCount`); `TestDispatchHonoursMaxParallel` (stub of 20 adapters, MaxParallel=4; instrument adapter to `defer atomic.AddInt32(&inflight, -1)`; assert `max(inflight) == 4` observed across run); `TestDispatchWorkerStateNoMapWrites` (run `go test -race`; stub adapters NEVER write to `Result.AdapterErrors` map directly; the supervisor performs all map writes). All in `dispatch_test.go`. |
| REQ-FAN-003 | Event-Driven | WHEN one or more adapters return a non-nil error from `Search`, the fanout SHALL store the error at the worker's per-index slot (per §2.6); the supervisor SHALL copy it into `Result.AdapterErrors[name]` (key being the failing adapter's `Name()`) AFTER `eg.Wait()` returns, SHALL NOT propagate the error out of `Dispatch` (the function returns `(*Result, nil)`), SHALL contribute zero docs from the failing adapter to `Result.Docs`, AND SHALL increment `Result.Stats.ErrorCount` accordingly. The fanout SHALL NOT cancel other in-flight adapters because of one adapter's error (the errgroup first-error-cancel default is suppressed via per-task `return nil`). When `Stats.ErrorCount >= 1`, `Result.AdapterErrors` SHALL be a non-nil map with exactly `Stats.ErrorCount` entries (per §2.6 H17 fix). | P0 | `TestDispatchOneAdapterFailsOthersSucceed` (3 adapters: ad1 returns 5 docs success, ad2 returns `*SourceError{CategoryPermanent}`, ad3 returns 5 docs success; assert `len(result.Docs) == 10`, `result.AdapterErrors != nil`, `len(result.AdapterErrors) == 1`, `result.AdapterErrors["ad2"] != nil`, `errors.Is(result.AdapterErrors["ad2"], types.ErrPermanent)`, `result.Stats.SuccessCount == 2`, `result.Stats.ErrorCount == 1`, invariant `SuccessCount + ErrorCount == AdapterCount` holds); `TestDispatchOneFailureDoesNotCancelOthers` (ad1 fails fast with `ErrPermanent`; ad2 sleeps 200ms then succeeds; assert ad2's docs ARE in result and the call elapsed ≥ 200ms). |
| REQ-FAN-004 | State-Driven | WHILE the parent ctx fires before all adapters return (the caller's overall deadline expires mid-fanout), the fanout SHALL collect partial results from any adapter that completed successfully prior to cancellation, SHALL record `context.DeadlineExceeded` (or the adapter-wrapped equivalent) in `Result.AdapterErrors[name]` for adapters that did NOT complete, SHALL return `(*Result, nil)` (NOT `ctx.Err()`), AND SHALL include the partially-completed `Result.Stats` reflecting the truncated dispatch. | P0 | `TestDispatchPartialResultsOnParentTimeout` (5 adapters: ad1/ad2 return 100ms; ad3/ad4/ad5 sleep 5s; parent ctx expires at 500ms; assert `len(result.Docs) == 10` (ad1+ad2 contributed); assert `result.AdapterErrors[ad3]` / `[ad4]` / `[ad5]` are non-nil and `errors.Is(*, context.DeadlineExceeded)` for each; `result.Stats.SuccessCount == 2`, `result.Stats.ErrorCount == 3`; total elapsed ∈ [500ms, 800ms]). |
| REQ-FAN-005 | Event-Driven | WHEN the per-adapter context derived per §2.5 expires before the adapter returns AND the parent ctx is still live, the fanout SHALL record the per-adapter timeout in `Result.AdapterErrors[name]` AND SHALL CONTINUE waiting for other in-flight adapters (the per-adapter ctx is independent of sibling per-adapter ctxs). | P0 | `TestDispatchPerAdapterTimeoutDoesNotKillOthers` (PerAdapterTimeout=200ms; 3 adapters: ad1 sleeps 1s (will time out), ad2 returns 5 docs at 100ms, ad3 returns 5 docs at 150ms; parent ctx is 5s; assert `len(result.Docs) == 10`, `result.AdapterErrors[ad1] != nil`, ad2/ad3 SUCCEEDED; total elapsed ≈ 200ms (ad1's per-adapter timeout)). |
| REQ-FAN-006 | Event-Driven | WHEN `Result.Docs` (post-merge) contains two or more docs sharing a canonical URL key (per §2.4 8-rule transformation), the fanout SHALL retain only the FIRST occurrence in input order, SHALL drop subsequent same-key occurrences, AND SHALL increment `Result.Stats.DedupDropped` by the count of dropped docs. WHEN a doc's URL fails to parse, the fanout SHALL fall back to `doc.CanonicalHash()` as the dedup key for THAT doc only. WHEN one doc has a parseable URL and another has an unparseable URL, they live in DIFFERENT dedup key spaces and SHALL NEVER be merged (a parseable canonical URL is never compared against a `CanonicalHash` value — even if the byte strings happen to coincide, the namespace tags differ). | P1 | `TestDedupSameURLFirstWins` (docs with `URL=https://example.com/a` from ad1 (Title="A") and ad2 (Title="A-edited"); assert output has Title="A", `Stats.DedupDropped == 1`); `TestDedupTrackingParamsStripped` (URLs differ ONLY in `utm_source`; assert deduped to one); `TestDedupHashFallbackOnUnparseableURL` (doc1 URL="not a url" Title="X" Body="Y"; doc2 URL="" Title="X" Body="Y"; assert deduped to one via hash); `TestDedupMixedValidInvalidURL` (doc1 URL=valid, doc2 URL="not a url" same content; assert KEPT separately — different key spaces); `TestDedupKeyDeterministic` (run dedup twice on same input; assert byte-equal outputs). All in `dedup_test.go`. |
| REQ-FAN-007 | Ubiquitous | The fanout SHALL sort `Result.Docs` in-place via `sort.SliceStable` using PRIMARY key `Score` descending, SECONDARY tie-breaker adapter name (= `SourceID`) ASCENDING, TERTIARY tie-breaker `RetrievedAt` descending. This sort happens AFTER dedup. The `Stats.ElapsedSeconds` SHALL be measured from the start of `Dispatch` to the end of sorting (just before observability emit). | P0 | `TestSortPrimaryScoreDescending` (input: 3 docs with Score 0.7/0.9/0.5; assert output order 0.9/0.7/0.5); `TestSortSecondaryAdapterAscending` (3 docs with equal Score 0.5; SourceID `c`/`a`/`b`; assert output order `a`/`b`/`c`); `TestSortTertiaryRetrievedAtDescending` (2 docs with equal Score AND equal SourceID; RetrievedAt 2026-04-01 vs 2026-05-01; assert 2026-05-01 first); `TestSortStableForEqualKeys` (10 docs all with equal Score+SourceID+RetrievedAt; assert output preserves input order). All in `sort_test.go`. |
| REQ-FAN-008 | Unwanted | IF `decision.AdapterSet` is empty (zero entries) when `Dispatch` is invoked, THEN the fanout SHALL return `(*Result{Docs: nil, AdapterErrors: nil, Stats: Stats{AdapterCount: 0}}, ErrEmptyAdapterSet)` immediately, SHALL NOT spawn any goroutines, AND SHALL emit one slog WARN record at the call boundary with `request_id`, `error="ErrEmptyAdapterSet"`. The sentinel `ErrEmptyAdapterSet` is distinct from `ErrAdapterRegistryEmpty` — the latter is only returned by `New` (REQ-FAN-001). | P0 | `TestDispatchEmptyAdapterSetRejected` (empty `decision.AdapterSet`; assert `errors.Is(err, ErrEmptyAdapterSet)`, NOT `ErrAdapterRegistryEmpty`, `result.Stats.AdapterCount == 0`, no goroutines spawned (use `runtime.NumGoroutine()` snapshot before/after — should be unchanged within race-detector tolerance)); in `fanout_test.go`. |
| REQ-FAN-009 | State-Driven | WHILE the same `*Fanout` instance is invoked concurrently from N goroutines (N ≥ 1) — N caller goroutines each calling `Dispatch(ctx, decision, q)` — each call SHALL execute independently with no shared mutable state across calls (the `*Fanout` struct is immutable post-construction; the underlying `*adapters.Registry` and `*obs.Obs` are themselves goroutine-safe per their own SPEC contracts), the cumulative effect SHALL be N independent fanout dispatches with no race-detector alarms, AND the `FanoutInflight{adapter_class}` Gauge SHALL never decrement below zero across the workload. | P0 | `TestDispatchConcurrent` in `concurrent_test.go`: 50 caller goroutines × 100 calls × 5-adapter stub registry = 25,000 adapter invocations under `go test -race`. Assertions: (1) zero race-detector alarms attributable to the fanout package; (2) every `*Result` returned has `result.Stats.AdapterCount == 5`; (3) `FanoutInflight{adapter_class=...}.Get()` ends at zero across all class values; (4) `goleak.VerifyNone(t)` after the test confirms zero residual goroutines. NFR-FAN-002 anchors the workload sizing. |
| REQ-FAN-010 | Ubiquitous | The fanout SHALL emit per-`Dispatch` invocation: (a) one OTel parent span `fanout.dispatch` (kind = internal) with attributes `fanout.category` (= `string(decision.Category)`), `fanout.adapter_count`, `fanout.result_count`, `fanout.errors_count`, `fanout.dedup_dropped`, `fanout.elapsed_seconds`; the registry's wrappedAdapter `adapter.search` spans appear as children of this span via ctx propagation. (b) For each adapter dispatched, ONE increment then ONE decrement on `obs.Metrics.FanoutInflight.WithLabelValues(string(decision.Category)).Inc/Dec`. (c) ONE slog record at level INFO (success or partial) or WARN (`ErrEmptyAdapterSet` / `ErrFanoutCancelled`) via `obs.Logger` with attributes `{request_id, category, adapter_count, result_count, errors_count, dedup_dropped, elapsed_seconds}`. The fanout SHALL be nil-safe across `obs.Obs`, `obs.Metrics`, individual collectors, and `obs.Logger` per the pattern at `internal/router/router.go:387-401` and `internal/adapters/registry.go:223-251`. The fanout SHALL NOT register or emit ANY new Prometheus metric family beyond `FanoutInflight` (already registered in SPEC-OBS-001). | P0 | `TestEmitParentSpanWithAttributes` (in-memory OTel exporter; assert `fanout.dispatch` span exists with all 6 attributes); `TestEmitAdapterSpansAreChildren` (assert `adapter.search` spans have `fanout.dispatch` as parent via SpanContext); `TestEmitFanoutInflightIncDec` (3 adapters, Category=`web`; assert `FanoutInflight{web}` peaked at 3 then returned to 0); `TestEmitSlogIncludesRequestID` (ctx with `reqid.WithContext`; assert captured slog JSON contains the request_id); `TestEmitSafeOnNilObs` (construct `*Fanout` with `Obs: nil`; Dispatch does not panic); `TestNoNewMetricFamilies` (snapshot `prometheus.Registry.Gather()` before+after; assert delta is zero new families). All in `dispatch_test.go` + a new `observability_test.go`. |
| REQ-FAN-011 | Event-Driven | WHEN an adapter's `Search` panics, the fanout's per-goroutine `defer recover()` SHALL convert the panic into `*types.SourceError{Adapter: name, Category: CategoryUnknown, Cause: fmt.Errorf("adapter %q panicked: %v", name, recovered)}`, SHALL record the entry in the per-index error slot (per §2.6), SHALL log one slog WARN record with the captured `runtime/debug.Stack()` output, AND SHALL allow the rest of the fanout to complete normally. The process SHALL NOT crash. The supervisor copies the per-index error slot into `Result.AdapterErrors[name]` after `eg.Wait()`. | P0 | `TestDispatchAdapterPanicCaptured` (3 adapters: ad1 returns 5 docs, ad2 panics with `panic("oops")`, ad3 returns 5 docs; assert `len(result.Docs) == 10`, `result.AdapterErrors["ad2"]` is `*SourceError{Category: CategoryUnknown}` and unwrapping reveals `"adapter \"ad2\" panicked: oops"`, `result.Stats.ErrorCount == 1`, `result.Stats.SuccessCount == 2`); `TestDispatchAdapterPanicLogsStackTrace` (capture slog JSON; assert `stack_trace` field contains `goroutine `); `TestDispatchAdapterPanicNoLeak` (`goleak.VerifyNone(t)` after the panicking call). All in `dispatch_test.go`. |
| REQ-FAN-012 | Event-Driven | WHEN the parent ctx is cancelled BEFORE every worker has been launched (the deadlock case where `errgroup.SetLimit(N)` queue is full and `eg.Go` would block; see §2.5 H18 fix), the supervisor SHALL detect `ctx.Err() != nil` BEFORE the next `eg.Go` call AND SHALL skip the launch, pre-populating the per-index error slot with `&types.SourceError{Adapter: name, Category: CategoryUnavailable, Cause: ctx.Err()}`. The supervisor SHALL continue scanning the remaining `decision.AdapterSet` entries, applying the same skip-and-pre-populate to each un-launched index. The fanout SHALL NOT deadlock; total elapsed for a fully-skipped dispatch SHALL be O(N) iterations of the for-loop, NOT bounded by any goroutine completion. | P0 | `TestDispatchCancelledMidQueue` (12 adapters with `MaxParallel=2`; instrument first 2 stub adapters to sleep 5s; cancel parent ctx at 50ms; assert `len(result.Docs) == 0`, `len(result.AdapterErrors) == 12` — 2 with `context.Canceled` from in-flight cancellation, 10 with `context.Canceled` from pre-populate skip; total elapsed ≤ 100ms; `goleak.VerifyNone(t)` clean); in `dispatch_test.go`. |
| REQ-FAN-013 | Event-Driven | WHEN `Dispatch` is invoked with a parent ctx that is already cancelled (`ctx.Err() != nil` at function entry), the fanout SHALL pre-populate every per-index error slot with `&types.SourceError{Adapter: name, Category: CategoryUnavailable, Cause: ctx.Err()}` WITHOUT launching any goroutine, SHALL return `(*Result{Stats: Stats{AdapterCount: K, ErrorCount: K}}, nil)` (K = `len(decision.AdapterSet)`), AND SHALL emit ONE slog WARN record summarising the early exit. The fanout SHALL NOT consume `Query.Deadline` itself (per §2.7 H15 fix); deadline application is the caller's responsibility (`cmd/usearch/query.go:140-141` shows the canonical pattern). | P0 | `TestDispatchAlreadyCancelledCtx` (parent ctx already cancelled; 3 adapters in decision; assert `result.Stats.ErrorCount == 3`, every `AdapterErrors[name]` satisfies `errors.Is(err, context.Canceled)`, total elapsed < 10ms, NO goroutines spawned (NumGoroutine before/after delta within race-detector tolerance)); `TestDispatchIgnoresQueryDeadline` (parent ctx = 5s; `Query.Deadline = time.Now().Add(100*time.Millisecond)` (much shorter); assert per-adapter ctxs derive from PARENT ctx (5s budget), NOT from `Query.Deadline` — adapters with 200ms latency SUCCEED). Both in `dispatch_test.go`. |

---

## 4. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| NFR-FAN-001 | Performance (fanout overhead) | The fanout overhead beyond the slowest adapter SHALL be: p50 ≤ 5 ms, p95 ≤ 20 ms, measured by `BenchmarkDispatch5Adapters` (`bench_test.go`) with 5 stub adapters each returning 5 NormalizedDocs after a 50ms sleep. The benchmark runs as `go test -bench=BenchmarkDispatch5Adapters -benchtime=10x -count=5 ./internal/fanout/...` on amd64. Overhead is computed per-iteration as `(elapsed - 50ms)` (the slowest adapter's response time). Median of 5 runs is the assertion value. The benchmark also reports `B/op` and `allocs/op`; `allocs/op ≤ 1000` (NFR-FAN-004). |
| NFR-FAN-002 | Race-clean concurrent invocation | `internal/fanout/concurrent_test.go::TestDispatchConcurrent` SHALL execute successfully under `go test -race ./internal/fanout/...` with the workload defined in REQ-FAN-009: 50 caller goroutines × 100 `Dispatch` calls each, each call iterating a stub registry of 5 adapters. Race-detector alarms attributable to the fanout package SHALL be zero. Cumulative call count: 5,000 fanout invocations × 5 adapters = 25,000 adapter Search invocations. |
| NFR-FAN-003 | Zero goroutine leaks | The fanout SHALL pass `goleak.VerifyNone(t)` after every test that invokes `Dispatch`, including the success path, the partial-failure path, the all-failure path, the parent-ctx-cancellation mid-flight path, the per-adapter-timeout path, and the panic-recovery path. `internal/fanout/bench_test.go::TestMain` SHALL invoke `goleak.VerifyTestMain(m)` (mirrors `internal/adapters/reddit/bench_test.go` pattern). The fanout itself SHALL launch only the bounded errgroup workers (capped at `Options.MaxParallel`); no detached background goroutines are permitted. |
| NFR-FAN-004 | Allocation ceiling on hot path | `BenchmarkDispatch5Adapters` SHALL report `allocs/op ≤ 1000` over 50 iterations × 5 adapters = 250 docs handled per op. This is a starting target derived from the dedup map (~5 allocs/doc), the per-adapter ctx + cancel pair (~6 allocs/adapter), errgroup state (~10 allocs/dispatch), and the slog/span emission (~50 allocs/dispatch). Adjustable downward in iteration 3 (HISTORY) once empirical baseline is established, mirroring the NFR-ADP-001 amendment pattern from SPEC-ADP-001 spec.md HISTORY 2026-04-26. |

---

## 5. Acceptance Criteria

### REQ-FAN-001 — `Fanout` Construction

- File `internal/fanout/fanout.go` declares `Fanout` struct with the
  documented fields (`registry *adapters.Registry`, `obs *obs.Obs`,
  `maxParallel int`, `perAdapterTimeout time.Duration`,
  `defaultDeadline time.Duration`).
- The compile-time signature `Dispatch(ctx context.Context, decision
  router.RoutingDecision, q types.Query) (*Result, error)` is in place.
- `New(opts Options)` returns `(nil, ErrAdapterRegistryEmpty)` for
  `opts.Registry == nil`.
- `New(opts Options)` returns `(nil, ErrAdapterRegistryEmpty)` for an
  empty registry (`opts.Registry.List() returns []`).
- `New(opts Options)` accepts zero-valued `MaxParallel` /
  `PerAdapterTimeout` / `DefaultDeadline` and substitutes documented
  defaults (8 / 8s / 30s).
- `TestNewRequiresRegistry`, `TestNewRequiresAtLeastOneAdapter`,
  `TestNewNormalisesDefaults`, `TestDispatchAlwaysReturnsResult` all
  pass.

### REQ-FAN-002 — Happy Path Dispatch

- `TestDispatchHappyPath3Adapters`: stub registry with 3 stub adapters
  each returning 5 unique-URL docs; `decision.AdapterSet =
  ["ad1","ad2","ad3"]`; Category=`web`. Assertion: `len(result.Docs) ==
  15` (no dedup needed — distinct URLs), `result.Stats.AdapterCount ==
  3`, `result.Stats.SuccessCount == 3`, `result.Stats.ErrorCount == 0`,
  `len(result.AdapterErrors) == 0`, `result.Stats.DedupDropped == 0`.
- `TestDispatchHonoursMaxParallel`: stub of 20 stub adapters configured
  with `MaxParallel=4`; each adapter sleeps 100ms then returns. Each
  adapter increments a shared `int32` counter on entry (atomic) and
  decrements on exit. `max(counter)` observed across run is 4 exactly.
  Total elapsed ≈ `(20/4) * 100ms = 500ms` ± 100ms.

### REQ-FAN-003 — Per-Adapter Failure Isolation

- `TestDispatchOneAdapterFailsOthersSucceed`: ad2 returns
  `*SourceError{Category: CategoryPermanent}`. ad1 + ad3 return 5 docs
  each. Assertion: `len(result.Docs) == 10`,
  `result.AdapterErrors["ad2"] != nil`,
  `errors.Is(result.AdapterErrors["ad2"], types.ErrPermanent)`,
  `result.Stats.SuccessCount == 2`, `result.Stats.ErrorCount == 1`.
- `TestDispatchOneFailureDoesNotCancelOthers`: ad1 fails fast (10ms
  with `ErrPermanent`); ad2 sleeps 200ms then returns 5 docs.
  Assertion: `len(result.Docs) == 5`, total elapsed ≥ 200ms (ad2's
  goroutine completed despite ad1's failure).

### REQ-FAN-004 — Partial Results on Parent Timeout

- `TestDispatchPartialResultsOnParentTimeout`: 5 adapters; ad1+ad2
  return docs at 100ms each, ad3+ad4+ad5 sleep 5s. Parent ctx is
  `context.WithTimeout(context.Background(), 500*time.Millisecond)`.
  Assertion: `result != nil`, `err == nil` (NOT `ctx.Err()`),
  `len(result.Docs) == 10` (ad1 + ad2 each 5 docs),
  `result.AdapterErrors[ad3]` / `[ad4]` / `[ad5]` non-nil, each
  satisfying `errors.Is(*, context.DeadlineExceeded)` (or
  `errors.Is(*, types.ErrSourceUnavailable)` if the adapter wraps,
  per ADP-001 REQ-ADP-005), `result.Stats.SuccessCount == 2`,
  `result.Stats.ErrorCount == 3`. Total elapsed in [500ms, 800ms].

### REQ-FAN-005 — Per-Adapter Timeout Independence

- `TestDispatchPerAdapterTimeoutDoesNotKillOthers`:
  `Options.PerAdapterTimeout=200ms`; parent ctx 5s deadline. ad1
  sleeps 1s (will exceed 200ms). ad2 returns 5 docs at 100ms. ad3
  returns 5 docs at 150ms. Assertion: `len(result.Docs) == 10`
  (ad2 + ad3), `result.AdapterErrors["ad1"] != nil` and
  `errors.Is(*, context.DeadlineExceeded)`,
  `result.Stats.SuccessCount == 2`, `result.Stats.ErrorCount == 1`,
  total elapsed ≈ 200ms (bounded by ad1's per-adapter timeout, NOT
  ad1's full 1s sleep).

### REQ-FAN-006 — Deduplication

- `TestDedupSameURLFirstWins`: 2 docs with
  `URL=https://example.com/a`; ad1 contributes Title="A", ad2
  contributes Title="A-edited". Output `result.Docs` has 1 entry with
  Title="A" (first occurrence wins); `result.Stats.DedupDropped == 1`.
- `TestDedupTrackingParamsStripped`: doc1 URL=
  `https://example.com/x?utm_source=A&id=1`; doc2 URL=
  `https://example.com/x?utm_source=B&id=1`. Canonical for both:
  `https://example.com/x?id=1`. Assertion: 1 entry in output,
  `Stats.DedupDropped == 1`.
- `TestDedupHashFallbackOnUnparseableURL`: doc1 has URL="`not a url`",
  Title="X", Body="Y"; doc2 has URL="", Title="X", Body="Y" (same
  content, both URLs unparseable). Both have identical
  `CanonicalHash()`. Assertion: 1 entry in output,
  `Stats.DedupDropped == 1`.
- `TestDedupKeyDeterministic`: run dedup twice on the same fixture;
  assert byte-equal output (slice contents and order).
- `TestCanonicalURLTable` in `canonical_test.go`: 6+ inputs from §2.4
  worked-examples table; each input → expected output (byte-equal).

### REQ-FAN-007 — Result Sorting

- `TestSortPrimaryScoreDescending`: input docs with Scores
  [0.7, 0.9, 0.5]; output order [0.9, 0.7, 0.5].
- `TestSortSecondaryAdapterAscending`: 3 docs all with Score=0.5;
  SourceIDs [`c`,`a`,`b`]; output order [`a`,`b`,`c`].
- `TestSortTertiaryRetrievedAtDescending`: 2 docs with equal Score AND
  equal SourceID; RetrievedAt 2026-04-01 vs 2026-05-01; output order
  [2026-05-01, 2026-04-01].
- `TestSortStableForEqualKeys`: 10 docs all with the SAME
  (Score, SourceID, RetrievedAt) triple, but different IDs in input
  order [d1..d10]; output order is identical [d1..d10] (sort.SliceStable
  preserves input order for equal keys).

### REQ-FAN-008 — Empty AdapterSet Rejected

- `TestDispatchEmptyAdapterSetRejected`: invoke
  `Dispatch(ctx, RoutingDecision{AdapterSet: nil}, q)`. Assertion:
  `errors.Is(err, ErrEmptyAdapterSet)`, `result.Stats.AdapterCount ==
  0`, `len(result.Docs) == 0`, `len(result.AdapterErrors) == 0`. Use
  `runtime.NumGoroutine()` before+after to confirm no goroutines
  leaked (delta within race-detector tolerance).
- Repeat with `decision.AdapterSet = []string{}` (empty slice, not
  nil); same assertions.

### REQ-FAN-009 — Concurrent-Safety State-Driven Contract

- `TestDispatchConcurrent` in `concurrent_test.go`:
  - Construct one `*Fanout` against a stub registry of 5 stub adapters
    each returning 5 unique docs.
  - Spawn 50 caller goroutines via `sync.WaitGroup` barrier.
  - Each goroutine performs 100 `Dispatch` calls in a loop.
  - Total: 50 × 100 = 5,000 fanout invocations × 5 adapters = 25,000
    adapter Search invocations.
- Assertions:
  1. The test executes successfully under `go test -race ./internal/fanout/...`;
     the race detector reports zero data-race alarms attributable to
     the fanout package.
  2. Every returned `*Result` has `Stats.AdapterCount == 5` and
     `len(Docs) == 25` (5 unique × 5 adapters; assumes each stub
     adapter produces unique URLs so dedup is a no-op).
  3. The `FanoutInflight{adapter_class="web"}` Gauge value via
     `reg.Prometheus.Gather` ends at zero (or +/- 1 race-detector
     tolerance during gather; final Gauge value MUST be exactly 0).
  4. `goleak.VerifyNone(t)` at the test's end confirms zero residual
     goroutines.

### REQ-FAN-010 — Per-Call Observability

- `TestEmitParentSpanWithAttributes`: in-memory OTel SpanRecorder; call
  Dispatch; gather spans; assert one span named `fanout.dispatch` with
  exactly the 6 attributes (`fanout.category`,
  `fanout.adapter_count`, `fanout.result_count`,
  `fanout.errors_count`, `fanout.dedup_dropped`,
  `fanout.elapsed_seconds`) populated and the Kind=Internal.
- `TestEmitAdapterSpansAreChildren`: assert the 3 adapter spans
  (`adapter.search` from wrappedAdapter) have `fanout.dispatch` as
  parent in their SpanContext.
- `TestEmitFanoutInflightIncDec`: 3 adapters, Category=`web`; instrument
  the stub adapters to capture the FanoutInflight value at Search
  start. Assertion: at least one observation of value 3 (peak), final
  value 0.
- `TestEmitSlogIncludesRequestID`: capture slog JSON via custom
  handler; ctx with `reqid.WithContext(ctx, "TEST-FAN-REQ")`; invoke
  Dispatch; assert exactly one slog record at INFO with attributes
  `request_id="TEST-FAN-REQ"`, `category="web"`, etc.
- `TestEmitSafeOnNilObs`: construct `*Fanout` with `Obs: nil`; Dispatch
  does not panic; returns valid `*Result`.
- `TestNoNewMetricFamilies`: snapshot the Prometheus registry's
  `Gather()` output before constructing the Fanout; snapshot after
  Dispatch; assert the family-count delta is exactly zero (no new
  metric families registered by FAN-001).

### REQ-FAN-011 — Adapter Panic Captured

- `TestDispatchAdapterPanicCaptured`: 3 adapters (ad1 returns 5 docs;
  ad2's Search calls `panic("oops")`; ad3 returns 5 docs). Assertion:
  `len(result.Docs) == 10`, `result.AdapterErrors["ad2"]` is a
  `*types.SourceError` with `Category == CategoryUnknown`, the Cause
  message matches `adapter "ad2" panicked: oops`,
  `result.Stats.ErrorCount == 1`, `result.Stats.SuccessCount == 2`.
  The test's parent `Dispatch` returns normally without panicking.
- `TestDispatchAdapterPanicLogsStackTrace`: capture slog JSON; assert
  one WARN record with attribute `stack_trace` whose value contains
  the substring `goroutine `.
- `TestDispatchAdapterPanicNoLeak`: `goleak.VerifyNone(t)` after the
  panicking call confirms no goroutine leak.

### NFR-FAN-001 — Performance Overhead

- `BenchmarkDispatch5Adapters` is invoked as
  `go test -bench=BenchmarkDispatch5Adapters -benchtime=10x -count=5 ./internal/fanout/...`
  on amd64.
- Each adapter sleeps 50ms then returns 5 NormalizedDocs.
- Per-iteration overhead = `wall_clock - 50ms`.
- Median of 5 runs: p50 overhead ≤ 5ms, p95 overhead ≤ 20ms.
- The bench reports `B/op` and `allocs/op`; `allocs/op ≤ 1000` (per
  NFR-FAN-004).

### NFR-FAN-002 — Race-Clean Concurrent Workload

- `TestDispatchConcurrent` (REQ-FAN-009 acceptance) executes under
  `go test -race`; race-detector alarms attributable to the fanout
  package = 0.

### NFR-FAN-003 — Zero Goroutine Leaks

- `TestMain` in `bench_test.go`:
  ```
  func TestMain(m *testing.M) {
      goleak.VerifyTestMain(m)
  }
  ```
  Mirrors `internal/adapters/reddit/bench_test.go`.
- Every `Dispatch`-invoking test SHALL pass `goleak.VerifyNone(t)` at
  its end.

### NFR-FAN-004 — Allocation Ceiling

- `BenchmarkDispatch5Adapters` reports `allocs/op ≤ 1000` over 5×5=25
  docs handled per op.

---

## 6. Technical Approach

### 6.1 Files to Modify

**Created (12 files)**:

- `internal/fanout/fanout.go` — `Fanout` struct, `New`, public method
  signature surface
- `internal/fanout/fanout_test.go` — orchestration tests
- `internal/fanout/options.go` — `Options` struct, defaults, validation
- `internal/fanout/options_test.go` — `New` validation tests
- `internal/fanout/result.go` — `Result` and `Stats` types
- `internal/fanout/dispatch.go` — `(*Fanout).Dispatch` hot path,
  errgroup orchestration, per-adapter ctx derivation, panic recovery
- `internal/fanout/dispatch_test.go` — happy path / failure / timeout /
  panic / observability tests
- `internal/fanout/dedup.go` — `dedupDocs` function
- `internal/fanout/dedup_test.go` — dedup table tests
- `internal/fanout/canonical.go` — `canonicalURL` function
- `internal/fanout/canonical_test.go` — URL canonicalization table
- `internal/fanout/sort.go` — `sortDocs` function
- `internal/fanout/sort_test.go` — sort ordering tests
- `internal/fanout/observability.go` — `emitDispatch` helper
- `internal/fanout/observability_test.go` — slog/span/Gauge tests
- `internal/fanout/concurrent_test.go` — NFR-FAN-002 race-clean
  workload
- `internal/fanout/bench_test.go` — `BenchmarkDispatch5Adapters` +
  `TestMain` with `goleak.VerifyTestMain`
- `internal/fanout/errors.go` — sentinel errors

**Modified (3 files)**:

- `internal/fanout/fanout.go` — replaces the 4-line stub at
  `internal/fanout/fanout.go:1-3`.
- `cmd/usearch/query.go` — DELETE the placeholder `runFanout` at
  lines 324-368 plus its 2 `@MX` annotations at lines 316-323;
  REPLACE the call site at line 208 (`docs, adapterErrs :=
  runFanout(...)`) with a `fanout.Dispatch(...)` call. Construct the
  `*Fanout` once in `Execute` (or at process init) and reuse.
- `cmd/usearch/integration_test.go` — update E2E tests at
  `TestQueryE2EWithStubs` (line 100+) to use the new fanout shape.

**Unchanged (by design)**:

- `pkg/types/*` — no contract change required.
- `internal/adapters/registry.go` — wrappedAdapter sole-emitter
  pattern preserved; FAN-001 emits ZERO new per-adapter
  metrics/logs/spans.
- `internal/obs/metrics/metrics.go` — `FanoutInflight` already
  registered. No new metric family.
- `internal/router/router.go` — IR-001 is unchanged; FAN-001 consumes
  `RoutingDecision` by value.
- `.moai/config/sections/quality.yaml` — `development_mode: tdd` and
  `test_coverage_target: 85` already in place.

### 6.2 Package Layout

```
internal/fanout/
├── fanout.go                                 # Fanout struct, New, public surface
├── fanout_test.go                            # Orchestration tests
├── options.go                                # Options + defaults + validation
├── options_test.go
├── result.go                                 # Result + Stats types
├── dispatch.go                               # Dispatch hot path
├── dispatch_test.go                          # Happy/fail/timeout/panic tests
├── dedup.go                                  # dedupDocs
├── dedup_test.go
├── canonical.go                              # canonicalURL
├── canonical_test.go
├── sort.go                                   # sortDocs
├── sort_test.go
├── observability.go                          # emitDispatch helper
├── observability_test.go
├── errors.go                                 # ErrEmptyAdapterSet, ErrAdapterNotFound, ErrFanoutCancelled
├── concurrent_test.go                        # NFR-FAN-002 race workload
└── bench_test.go                             # BenchmarkDispatch5Adapters + TestMain (goleak)
```

### 6.3 Type Sketches (illustrative; final shapes in run phase)

```go
// internal/fanout/options.go
package fanout

const (
    defaultMaxParallel       = 8
    defaultPerAdapterTimeout = 8 * time.Second
    defaultDeadline          = 30 * time.Second
)

type Options struct {
    Registry          *adapters.Registry
    Obs               *obs.Obs
    MaxParallel       int           // default 8
    PerAdapterTimeout time.Duration // default 8s
    DefaultDeadline   time.Duration // default 30s
}

// internal/fanout/fanout.go
type Fanout struct {
    registry          *adapters.Registry
    obs               *obs.Obs
    maxParallel       int
    perAdapterTimeout time.Duration
    defaultDeadline   time.Duration
}

func New(opts Options) (*Fanout, error) {
    if opts.Registry == nil {
        return nil, ErrEmptyAdapterSet
    }
    if len(opts.Registry.List()) == 0 {
        return nil, ErrEmptyAdapterSet
    }
    f := &Fanout{
        registry:          opts.Registry,
        obs:               opts.Obs,
        maxParallel:       firstNonZeroInt(opts.MaxParallel, defaultMaxParallel),
        perAdapterTimeout: firstNonZeroDuration(opts.PerAdapterTimeout, defaultPerAdapterTimeout),
        defaultDeadline:   firstNonZeroDuration(opts.DefaultDeadline, defaultDeadline),
    }
    return f, nil
}

// internal/fanout/result.go
type Result struct {
    Docs          []types.NormalizedDoc
    AdapterErrors map[string]error
    Stats         Stats
}

type Stats struct {
    AdapterCount   int
    SuccessCount   int
    ErrorCount     int
    DedupDropped   int
    ElapsedSeconds float64
}

// internal/fanout/dispatch.go (hot path)
func (f *Fanout) Dispatch(
    ctx context.Context,
    decision router.RoutingDecision,
    q types.Query,
) (*Result, error) {
    if len(decision.AdapterSet) == 0 {
        f.logEmpty(ctx)
        return &Result{Stats: Stats{AdapterCount: 0}}, ErrEmptyAdapterSet
    }

    tracer := f.tracer()
    spanCtx, span := tracer.Start(ctx, "fanout.dispatch",
        oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
    defer span.End()

    start := time.Now()
    classLabel := string(decision.Category)

    // §2.6: per-index pre-allocated slices; no map writes by workers.
    perAdapterDocs := make([][]types.NormalizedDoc, len(decision.AdapterSet))
    perAdapterErr := make([]error, len(decision.AdapterSet))

    eg, egCtx := errgroup.WithContext(spanCtx)
    eg.SetLimit(f.maxParallel)

    for i, name := range decision.AdapterSet {
        i, name := i, name

        // §2.5 H18 + REQ-FAN-012/013: pre-launch ctx guard prevents
        // SetLimit deadlock under cancelled-ctx-with-queued-workers
        // and handles the already-cancelled-ctx entry case.
        if err := ctx.Err(); err != nil {
            perAdapterErr[i] = &types.SourceError{
                Adapter:  name,
                Category: types.CategoryUnavailable,
                Cause:    err,
            }
            continue
        }

        eg.Go(func() error {
            f.incInflight(classLabel)
            defer f.decInflight(classLabel)
            defer func() {
                if r := recover(); r != nil {
                    perAdapterErr[i] = &types.SourceError{
                        Adapter:  name,
                        Category: types.CategoryUnknown,
                        Cause:    fmt.Errorf("adapter %q panicked: %v", name, r),
                    }
                    f.logPanic(spanCtx, name, r, debug.Stack())
                }
            }()

            ad, ok := f.registry.Get(name)
            if !ok {
                perAdapterErr[i] = fmt.Errorf("%w: %s", ErrAdapterNotFound, name)
                return nil
            }

            adapterCtx, cancel := f.deriveAdapterCtx(egCtx)
            defer cancel()

            docs, err := ad.Search(adapterCtx, q)
            if err != nil {
                perAdapterErr[i] = err
                return nil // suppress for partial-result assembly
            }
            perAdapterDocs[i] = docs
            return nil
        })
    }
    _ = eg.Wait() // errors are collected per-adapter via per-index slots

    // Supervisor (single goroutine) builds the AdapterErrors map.
    // No worker ever writes to a map directly (§2.6).
    res := assembleResult(decision.AdapterSet, perAdapterDocs, perAdapterErr)
    res.Docs, res.Stats.DedupDropped = dedupDocs(res.Docs)
    sortDocs(res.Docs)
    res.Stats.ElapsedSeconds = time.Since(start).Seconds()

    f.emit(spanCtx, span, decision, res)
    return res, nil
}
```

### 6.4 Per-Adapter Context Derivation

```go
// internal/fanout/dispatch.go
func (f *Fanout) deriveAdapterCtx(parent context.Context) (context.Context, context.CancelFunc) {
    deadline := f.perAdapterTimeout
    if pDeadline, ok := parent.Deadline(); ok {
        if remaining := time.Until(pDeadline); remaining < deadline {
            deadline = remaining
        }
    }
    if deadline <= 0 {
        // Parent ctx is already past its deadline; return an immediately-
        // cancelled ctx so the adapter sees ctx.Err() right away.
        ctx, cancel := context.WithCancel(parent)
        cancel()
        return ctx, cancel
    }
    return context.WithTimeout(parent, deadline)
}
```

### 6.5 Observability Note

The fanout emits ZERO new Prometheus metric families. ALL per-adapter
observability comes from the registry's `wrappedAdapter`
(`internal/adapters/registry.go:195-219`). The fanout's contribution is:

- ONE Gauge (`FanoutInflight`, already registered, `adapter_class` label)
  inc/dec PER WORKER GOROUTINE with `adapter_class = string(decision.Category)`.
- ONE OTel parent span (`fanout.dispatch`) — adapter spans are children
  via ctx propagation.
- ONE slog summary record at INFO (success / partial) or WARN
  (`ErrEmptyAdapterSet` / panic captured).

`outcome` label semantics for `AdapterCalls{adapter,outcome}` come from
the registry; FAN-001 does not need to map outcomes itself.

### 6.6 MX Tag Plan

| File::Symbol | Tag | Reason |
|--------------|-----|--------|
| `fanout.go::(*Fanout).Dispatch` | `@MX:ANCHOR` | Sole entry point for all fanout dispatches. fan_in ≥ 4 (CLI today, MCP tomorrow, SPEC-IDX-001 RRF input, tests). `@MX:REASON: contract boundary; signature change ripples to CLI-001 + MCP-001 + IDX-001`. `@MX:SPEC: SPEC-FAN-001`. |
| `dispatch.go::(*Fanout).Dispatch` (the hot path body) | `@MX:WARN` | Outbound fan-out spawns N goroutines without supervisor. `@MX:REASON: removing the per-goroutine defer recover()/deferred FanoutInflight.Dec() invalidates NFR-FAN-003 zero-leak guarantee`. |
| `dispatch.go::(*Fanout).deriveAdapterCtx` | `@MX:NOTE` | Magic constants (8s default, parent-deadline override). The note documents §2.5 derivation rules. |
| `dedup.go::dedupDocs` | `@MX:ANCHOR` | Every fanout-returned doc passes through this single transform. fan_in = 1 (Dispatch) but invariant-bearing — bug here corrupts every Result. `@MX:REASON: dedup invariant; first-occurrence-wins semantic must not change without SPEC amendment`. |
| `canonical.go::canonicalURL` | `@MX:NOTE` | The 8 canonicalization rules. Future contributors look here when adding/removing tracking-param entries. The 11-entry tracking list is annotated `@MX:NOTE` (single doc-comment, not 11 separate annotations). |
| `dispatch.go::eg.Go(...)` body (the per-goroutine closure) | `@MX:WARN` | Goroutine without explicit supervisor; relies on errgroup's bounded pool + defer recover for safety. `@MX:REASON: panic recovery + FanoutInflight Dec/Inc + ctx cancel sequence is load-bearing for NFR-FAN-003`. |

All tags `[AUTO]`-prefixed, include `@MX:SPEC: SPEC-FAN-001`, follow
`code_comments: en` per `.moai/config/sections/language.yaml`. Per-file
hard limit (3 ANCHOR + 5 WARN per `.moai/config/sections/mx.yaml`):
respected.

### 6.7 Configuration

The fanout introduces ONE new optional config section. Default values
work without it; this is documented for operators who want to tune.

```yaml
# .moai/config/sections/fanout.yaml (NEW; optional)
fanout:
  max_parallel: 8           # NFR-FAN-002 / OQ §11.2 default
  per_adapter_timeout_ms: 8000  # OQ §11.1 hardcoded default
  default_deadline_ms: 30000    # matches CLI defaultTimeout
```

The CLI's `cmd/usearch/main.go` (or the `executeConfig`) consumes this
file and constructs `Options` accordingly. v0.1 ships with all fields
defaulted; a SPEC-FAN-CFG-001 follow-up SPEC owns the full koanf-backed
config loader if needed.

### 6.8 Harness Level

Per `.moai/config/sections/harness.yaml` auto-routing: 11 EARS REQs
(10 × P0 + 1 × P1) + 4 NFRs touching 1 package (`internal/fanout/`,
~17 source/test files) + 1 cross-package edit (`cmd/usearch/query.go`,
deletion of placeholder + call-site update) + 1 minor cmd integration
test edit + 1 optional config file = **standard** harness level.
Sprint Contract is OPTIONAL but recommended. Evaluator profile
`default` applies.

---

## 7. What NOT to Build (Exclusions)

[HARD] This SPEC explicitly excludes the following items. Each has a known
destination SPEC; this list prevents scope creep into FAN-001.

- **Retry orchestration** (exponential backoff, jitter, per-adapter
  retry budget keyed on `Capabilities.RateLimitPerMin`) → future
  SPEC-FAN-001-RETRY (research OQ §6.4). v0.1 ships zero-retry.
- **Per-adapter circuit breaker** (auto-disable adapter after N
  consecutive failures) → SPEC-EVAL-002 (M8). The registry has no
  disable flag.
- **Response caching** (in-process LRU on success path) → out of v0.1;
  SPEC-CACHE-001 owns the BLOCKED-source 5-phase fallback only.
- **Result ranking fusion across adapters** (Reciprocal Rank Fusion)
  → SPEC-IDX-001 (M3). Fanout output is RRF input.
- **Streaming/incremental result delivery** (channel-based, SSE,
  WebSocket) → SPEC-SYN-004 (M4).
- **Adapter health-state machine** → SPEC-EVAL-002 (M8).
- **`Capabilities.RecommendedTimeout` field addition** (per-adapter
  preferred deadline) → future SPEC-FAN-001a (research OQ §6.1). v0.1
  hardcodes the timeout in fanout `Options`.
- **Per-tenant adapter visibility / RBAC** → SPEC-AUTH-002 (M6).
- **Streaming progress events emitted from inside Dispatch** → out of
  v0.1; CLI's existing progress emitter at the call boundary suffices.
- **Per-adapter custom Prometheus metrics** → would require amending
  SPEC-OBS-001 allowlist. Out of v0.1.
- **HTTP / gRPC server exposure of fanout** → SPEC-MCP-001 (M7) and
  future SPEC-API-001. Fanout is a Go library only in v0.1.
- **Dynamic AdapterSet override** → CLI `--source` flag handles
  intersect-with-filter at the CLI layer. Fanout consumes the post-
  filter set verbatim.
- **Cardinality allowlist amendment** — ZERO new label names.
- **Full RFC 3986 URL normalisation** (percent-encoding pass, IDN,
  default port elision) — out of v0.1; the 8 rules are enough for
  M3-scale dedup. SPEC-SYN-003 (M4 dedup + clustering) layers richer
  semantics on top.
- **Per-call observability for "errgroup queued but not yet started"
  state** — workers are launched immediately by errgroup; queueing
  observation is not a useful signal at the M3 scale (8-12 adapters).

---

## 8. TDD Plan

Development follows RED-GREEN-REFACTOR per
`quality.development_mode: tdd` (`.moai/config/sections/quality.yaml`).
Representative RED-phase tests, written before implementation, grouped
by REQ. Total: 30 tests. Coverage target: 85% per
`quality.test_coverage_target`. Benchmarks do not count toward
coverage.

| # | Test | Layer | REQ | Assertion |
|---|------|-------|-----|-----------|
| 1 | `TestNewRequiresRegistry` | `options_test.go` | REQ-FAN-001 | `New(Options{})` returns `(nil, ErrEmptyAdapterSet)` |
| 2 | `TestNewRequiresAtLeastOneAdapter` | `options_test.go` | REQ-FAN-001 | Empty registry → `ErrEmptyAdapterSet` |
| 3 | `TestNewNormalisesDefaults` | `options_test.go` | REQ-FAN-001 | Zero Options → defaults applied |
| 4 | `TestDispatchAlwaysReturnsResult` | `fanout_test.go` | REQ-FAN-001 | `*Result` is non-nil for success/partial/failure |
| 5 | `TestDispatchHappyPath3Adapters` | `dispatch_test.go` | REQ-FAN-002 | 3 adapters × 5 docs = 15 in result |
| 6 | `TestDispatchHonoursMaxParallel` | `dispatch_test.go` | REQ-FAN-002 | 20 adapters, MaxParallel=4; max(inflight)==4 |
| 7 | `TestDispatchOneAdapterFailsOthersSucceed` | `dispatch_test.go` | REQ-FAN-003 | 2 success + 1 fail; result has 10 docs + 1 error |
| 8 | `TestDispatchOneFailureDoesNotCancelOthers` | `dispatch_test.go` | REQ-FAN-003 | Slow ad2 still completes despite fast-fail ad1 |
| 9 | `TestDispatchPartialResultsOnParentTimeout` | `dispatch_test.go` | REQ-FAN-004 | Partial assembly under 500ms parent timeout |
| 10 | `TestDispatchPerAdapterTimeoutDoesNotKillOthers` | `dispatch_test.go` | REQ-FAN-005 | ad1 200ms timeout, ad2/ad3 succeed |
| 11 | `TestDedupSameURLFirstWins` | `dedup_test.go` | REQ-FAN-006 | Two same-URL docs → first kept |
| 12 | `TestDedupTrackingParamsStripped` | `dedup_test.go` | REQ-FAN-006 | utm_source-only difference dedups |
| 13 | `TestDedupHashFallbackOnUnparseableURL` | `dedup_test.go` | REQ-FAN-006 | Same content, unparseable URLs → dedup via hash |
| 14 | `TestDedupKeyDeterministic` | `dedup_test.go` | REQ-FAN-006 | Run twice → byte-equal output |
| 15 | `TestCanonicalURLTable` | `canonical_test.go` | REQ-FAN-006 | 6+ inputs from §2.4 worked-examples table |
| 16 | `TestSortPrimaryScoreDescending` | `sort_test.go` | REQ-FAN-007 | [0.7,0.9,0.5] → [0.9,0.7,0.5] |
| 17 | `TestSortSecondaryAdapterAscending` | `sort_test.go` | REQ-FAN-007 | Equal score → alphabetical SourceID |
| 18 | `TestSortTertiaryRetrievedAtDescending` | `sort_test.go` | REQ-FAN-007 | Equal score+source → newer first |
| 19 | `TestSortStableForEqualKeys` | `sort_test.go` | REQ-FAN-007 | sort.SliceStable preserves input order |
| 20 | `TestDispatchEmptyAdapterSetRejected` | `fanout_test.go` | REQ-FAN-008 | Empty AdapterSet → `ErrEmptyAdapterSet` + zero goroutines |
| 21 | `TestDispatchConcurrent` | `concurrent_test.go` | REQ-FAN-009, NFR-FAN-002 | 50 goroutines × 100 calls × 5 adapters race-clean |
| 22 | `TestEmitParentSpanWithAttributes` | `observability_test.go` | REQ-FAN-010 | `fanout.dispatch` span with 6 attributes |
| 23 | `TestEmitAdapterSpansAreChildren` | `observability_test.go` | REQ-FAN-010 | adapter.search spans have fanout.dispatch parent |
| 24 | `TestEmitFanoutInflightIncDec` | `observability_test.go` | REQ-FAN-010 | Gauge peaks at 3, ends at 0 |
| 25 | `TestEmitSlogIncludesRequestID` | `observability_test.go` | REQ-FAN-010 | Captured slog includes request_id |
| 26 | `TestEmitSafeOnNilObs` | `observability_test.go` | REQ-FAN-010 | Nil Obs → no panic |
| 27 | `TestNoNewMetricFamilies` | `observability_test.go` | REQ-FAN-010 | Gather() before+after delta == 0 |
| 28 | `TestDispatchAdapterPanicCaptured` | `dispatch_test.go` | REQ-FAN-011 | Panic → SourceError{Unknown}; siblings unaffected |
| 29 | `TestDispatchAdapterPanicLogsStackTrace` | `dispatch_test.go` | REQ-FAN-011 | slog WARN with stack_trace attribute |
| 30 | `TestDispatchAdapterPanicNoLeak` | `dispatch_test.go` | REQ-FAN-011 | goleak.VerifyNone clean |
| 31 | `TestDispatchCancelledMidQueue` | `dispatch_test.go` | REQ-FAN-012 | 12 adapters MaxParallel=2; cancel @50ms → 10 pre-populated, no deadlock |
| 32 | `TestDispatchAlreadyCancelledCtx` | `dispatch_test.go` | REQ-FAN-013 | Pre-cancelled ctx → all errors Cancelled; no goroutines |
| 33 | `TestDispatchIgnoresQueryDeadline` | `dispatch_test.go` | REQ-FAN-013 | Query.Deadline NOT consumed; parent ctx is sole truth |
| 34 | `TestDedupMixedValidInvalidURL` | `dedup_test.go` | REQ-FAN-006 | Disjoint key spaces (url: vs hash:) — never merged |
| 35 | `TestDispatchWorkerStateNoMapWrites` | `dispatch_test.go` | REQ-FAN-002 | go test -race confirms no worker writes to AdapterErrors map |
| 36 | `BenchmarkDispatch5Adapters` | `bench_test.go` | NFR-FAN-001, NFR-FAN-004 | 5 adapters @ 50ms; overhead p50≤5ms p95≤20ms; allocs/op≤1000 |
| 37 | `TestMain` (goleak.VerifyTestMain) | `bench_test.go` | NFR-FAN-003 | Package-level goroutine leak check |

RED-GREEN-REFACTOR per requirement:
1. RED: Write failing test for REQ-FAN-N.
2. GREEN: Implement minimal code to pass.
3. REFACTOR: Tidy; extract shared helpers if they remove duplication;
   keep file sizes manageable (target each `.go` file < 250 LoC
   excluding tests).

Greenfield note: `internal/fanout/` is a 4-line stub today; there is no
behaviour to preserve. The migration of `cmd/usearch/query.go` is a
one-shot deletion+replacement; characterization tests for the existing
`runFanout` are not needed because `cmd/usearch/integration_test.go`
already covers the end-to-end behaviour (the new test must continue to
pass against the new fanout implementation).

---

## 9. Dependencies

### 9.1 Upstream SPEC Dependencies

- **SPEC-CORE-001 (implemented)**: provides `pkg/types.Adapter`,
  `pkg/types.Capabilities`, `pkg/types.Query`,
  `pkg/types.NormalizedDoc`, `*types.SourceError`,
  `types.OutcomeFromError`, `types.DocType` enum,
  `internal/adapters.Registry` with wrappedAdapter sole-emitter
  pattern. HARD dep.
- **SPEC-IR-001 (implemented)**: provides
  `router.RoutingDecision` shape (Category, AdapterSet sorted, Lang).
  HARD dep — fanout consumes this struct verbatim.
- **SPEC-ADP-001 (implemented)**: REQ-ADP-011 concurrent-safety
  contract is the precondition for NFR-FAN-002.
- **SPEC-ADP-002 (implemented)**: HN adapter; same shape.
- **SPEC-OBS-001 (implemented)**: `FanoutInflight{adapter_class}` Gauge
  pre-registered with `adapter_class` in cardinality allowlist. HARD
  dep.

### 9.2 Parallelizable

- **SPEC-IDX-001 (M3)**: can begin its plan phase as soon as FAN-001's
  spec.md is approved. IDX-001 consumes `fanout.Result.Docs` for RRF
  fusion.
- **SPEC-CACHE-001 (M3)**: can begin its plan phase as soon as FAN-001's
  spec.md is approved. CACHE-001 wraps fanout in a 5-phase access
  fallback harness.
- **SPEC-ADP-003 / SPEC-ADP-004 / SPEC-ADP-005 / SPEC-ADP-006 /
  SPEC-ADP-007 / SPEC-ADP-008 / SPEC-ADP-009 (all M3)**: gated on
  FAN-001's spec.md per `.moai/project/roadmap.md:122-123` (`M3 | All
  SPEC-ADP-* (7-way), SPEC-IDX-* (3-way) — gated on SPEC-FAN-001`).
  Once FAN-001 is approved, the 7 adapter SPECs can develop in
  parallel.

### 9.3 Downstream Blocked SPECs

- **SPEC-IDX-001** (M3): consumes `fanout.Result.Docs` for RRF.
- **SPEC-CACHE-001** (M3): wraps fanout in 5-phase access fallback.
- **SPEC-ADP-003..009** (M3): each adapter needs the fanout dispatch
  contract to integrate.
- **SPEC-MCP-001** (M7): exposes `usearch query` over MCP; constructs
  one `*Fanout` instance, reuses across tool calls.
- **SPEC-API-001** (deferred): future HTTP API consumes the same
  `*Fanout` instance.

### 9.4 External Dependencies (run-phase pins)

**Zero new Go module dependencies.** FAN-001 uses only:

- Go stdlib: `context`, `errors`, `fmt`, `net/url`, `runtime/debug`,
  `sort`, `strings`, `sync`, `sync/atomic`, `time`
- `golang.org/x/sync/errgroup` (already pinned via `go.mod:33`
  `golang.org/x/sync v0.20.0`; the placeholder `cmd/usearch/query.go`
  already uses this)
- `pkg/types` (already pinned via SPEC-CORE-001)
- `internal/adapters` (already pinned via SPEC-CORE-001)
- `internal/router` (already pinned via SPEC-IR-001) — for the
  `router.RoutingDecision` and `router.Category` types
- `internal/obs` and `internal/obs/reqid` and `internal/obs/metrics`
  (already pinned via SPEC-OBS-001)
- `go.opentelemetry.io/otel/{attribute,codes,trace}` (already pinned
  via SPEC-OBS-001)
- Test-only: `go.uber.org/goleak` (already pinned indirect via
  `go.mod:30`; reddit/HN adapters already use it)

---

## 10. Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| First-error cancel from errgroup kills sibling adapters | High | High | REQ-FAN-003 explicitly documents the suppress-error idiom (workers `return nil` even on adapter error). Test `TestDispatchOneFailureDoesNotCancelOthers` asserts the behaviour. |
| `errgroup.SetLimit` deadlock under cancelled-ctx-with-queued-workers | Medium | High | §2.5 H18 fix + REQ-FAN-012 pre-launch `ctx.Err()` guard. `TestDispatchCancelledMidQueue` (12 adapters, MaxParallel=2, cancel at 50ms) asserts no deadlock and elapsed ≤ 100ms. |
| Caller forgets to apply `Query.Deadline` to parent ctx | Medium | Medium | §2.7 H15 fix documents the contract explicitly. `TestDispatchIgnoresQueryDeadline` makes the boundary testable. CLI's existing pattern at `cmd/usearch/query.go:140-141` is the canonical reference. |
| Worker writes to shared `AdapterErrors` map → race | Low | High | §2.6 H1 fix mandates per-index `[]error` slice; supervisor builds map post-Wait. `TestDispatchWorkerStateNoMapWrites` runs under `go test -race` to confirm. |
| Per-adapter ctx leak (cancel never called) | Medium | High | `defer cancel()` immediately after `context.WithTimeout` in every worker goroutine. `go vet` catches missing-defer-cancel. NFR-FAN-003 + goleak verification close the loop. |
| Goroutine leak on panic | Medium | High | `defer recover()` in worker goroutine + REQ-FAN-011 acceptance + `TestDispatchAdapterPanicNoLeak` goleak check. |
| FanoutInflight Gauge drift (under-decrement on early exit) | Medium | Medium | `defer f.decInflight(class)` ALWAYS pairs with `f.incInflight(class)` at the start of the worker. Test `TestEmitFanoutInflightIncDec` asserts final value 0. |
| Dedup discards a "better" doc (later occurrence with richer Body) | Medium | Low | First-occurrence-wins is documented in §2.3 + REQ-FAN-006. SPEC-SYN-003 (M4) handles richer near-dup clustering. Open Question §11.3 keeps revisit door open. |
| URL canonicalization too aggressive (false dedup) | Medium | Medium | The 8 rules in §2.4 are intentionally MILD. The 11-entry tracking-param list is conservative (only well-known analytics keys). New tracking keys can be added in iteration 2 without breaking dedup correctness. |
| URL canonicalization too lax (missed dedup) | Low | Low | Acceptable for v0.1; SPEC-SYN-003 layers SimHash + embedding cosine on top to catch the misses. |
| MaxParallel=8 too aggressive on shared `http.DefaultTransport` | Low | Medium | `Options.MaxParallel` is configurable. Default tunable in `.moai/config/sections/fanout.yaml`. SPEC-DEP-001's HTTP transport tuning is out of scope here; per-adapter SPECs own their own transports (ADP-001 ships its own `*http.Client` per spec.md §6.5). |
| `runtime/debug.Stack()` in panic path is heap-heavy | Low | Low | Stack trace is captured ONCE per panic (rare event). Allocation count NOT counted toward NFR-FAN-004 (NFR is about hot-path allocs, not error-path allocs). |
| Sort instability across runs | Low | Low | `sort.SliceStable` + deterministic 3-key compare → byte-equal output for identical input. `TestSortStableForEqualKeys` covers. |
| Test stub adapters introduce their own goroutine leaks | Low | Medium | Test stubs implement `Search` synchronously (no internal goroutines). Confirmed by goleak passing in tests. |
| `RoutingDecision.AdapterSet` contains a name not in registry | Low | Medium | `registry.Get(name)` returns `(_, false)`; worker records `ErrAdapterNotFound`-wrapped error in `Result.AdapterErrors`; sibling adapters unaffected. |
| Caller's parent ctx already past deadline at Dispatch entry | Low | Low | `deriveAdapterCtx` detects `remaining <= 0` and returns an immediately-cancelled ctx; adapter's `Search` returns ctx.Err() right away; result is partial-empty + per-adapter timeout errors. |

---

## 11. Open Questions

These are explicitly UNRESOLVED at SPEC-approval time. Each has a
recommended default and a one-line resolution owner. They do NOT block
SPEC approval.

1. **`Capabilities.RecommendedTimeout` field addition**. **Recommended
   default**: NO in v0.1. Hardcode 8s default in `Options.PerAdapterTimeout`.
   Adopt as follow-up SPEC-FAN-001a if measured pain (e.g., SearXNG
   bridge needs 20s while Reddit needs 5s). **Resolution owner**:
   SPEC-FAN-001a author after M3 traffic.

2. **Default concurrency limit**. 8, 12, or unlimited? **Recommended
   default**: 8. Rationale: M3 has 12+ adapters; running all 12 in
   parallel saturates outbound socket pool on default
   `http.DefaultTransport.MaxIdleConnsPerHost = 2`. Capping at 8 leaves
   headroom for unrelated outbound calls (LLM, synthesis).
   **Resolution owner**: SPEC-FAN-001 author after first M3 load test.

3. **Dedup key — URL alone vs URL + hash**. Same canonical URL, different
   Title/Body should be dedup-merged or kept distinct? **Recommended
   default**: YES — URL alone is the dedup key. Different content under
   the same URL is unusual; the user sees one canonical entry; SPEC-SYN-002
   preserves provenance per claim. **Resolution owner**: SPEC-SYN-003
   (M4) author may override if telemetry shows false-merges hurting
   citation accuracy.

4. **Retry policy ownership**. ADP-001 spec.md HISTORY 2026-04-26 D3
   assigns retry to FAN-001. v0.1 ships zero-retry. **Recommended
   default**: NO retry in v0.1. Adding retry without a well-tested
   backoff/jitter scheme risks thundering herd. v0.2 introduces
   exponential backoff per-adapter. **Resolution owner**: future
   SPEC-FAN-001-RETRY (M3 follow-up) author.

5. **Adopt `sourcegraph/conc` for panic recovery + typed results**.
   Pre-1.0 dependency risk vs. cleaner code. Currently rejected.
   **Recommended default**: NO in v0.1. Revisit after sourcegraph/conc
   reaches 1.0. **Resolution owner**: SPEC-DEP-001 owner periodic
   dependency review.

6. **Channel-based result collection**. Index-slice pattern adopted in
   v0.1. **Recommended default**: index-slice. Channel pattern is a
   refactor for v0.2 if integrating with streaming consumer (SPEC-SYN-004).
   **Resolution owner**: SPEC-SYN-004 author may request channel for
   streaming-friendliness.

7. **Per-adapter circuit-breaker integration**. Should fanout
   short-circuit a flapping adapter? **Recommended default**: NO in v0.1.
   Adapter health-state tracking is SPEC-EVAL-002's domain (M8).
   **Resolution owner**: SPEC-EVAL-002 author may add a registry-level
   disable flag that fanout consults.

8. **`FanoutInflight{adapter_class}` label semantics**. Use IR-001
   Category (6 values) or literal adapter name (12+ values)?
   **Recommended default**: IR-001 Category (`web`/`social`/`academic`/
   `korean`/`mixed`/`unknown`). Bounded cardinality, aligns with the
   Router contract. **Resolution owner**: SPEC-FAN-001 author bakes
   this into the SPEC. RESOLVED inline in §6.5 + REQ-FAN-010 — Open
   Question kept here for plan-auditor visibility.

---

## 12. References

### External (URL-cited; verified per research.md §7)

- https://pkg.go.dev/golang.org/x/sync/errgroup — Go errgroup
  documentation; `WithContext` + `SetLimit` + first-error-cancel
  semantics. Quoted in research §2.1.
- https://pkg.go.dev/golang.org/x/sync/semaphore — Go semaphore
  documentation; `Weighted` API. Quoted in research §2.2 (rejected
  alternative).
- https://pkg.go.dev/github.com/sourcegraph/conc — sourcegraph/conc
  documentation; `pool.NewWithResults` + structured concurrency
  philosophy. Quoted in research §2.3 (rejected alternative).

### Internal (file:line cited)

- `.moai/specs/SPEC-FAN-001/research.md` — full research artifact
  (this SPEC's research sibling).
- `.moai/specs/SPEC-CORE-001/spec.md` — Adapter / Capabilities / Query /
  NormalizedDoc / SourceError / Registry contract.
- `.moai/specs/SPEC-IR-001/spec.md` — `RoutingDecision` shape, Category
  enum, REQ-IR-008 lexicographic-sort guarantee.
- `.moai/specs/SPEC-IR-001/spec.md:373-374` — REQ-ADP-011 adapter
  concurrent-safety contract precondition for NFR-FAN-002.
- `.moai/specs/SPEC-ADP-001/spec.md` — Reddit reference adapter
  (concurrent-safe).
- `.moai/specs/SPEC-ADP-002/spec.md` — Hacker News adapter.
- `.moai/specs/SPEC-OBS-001/spec.md` — observability bundle, cardinality
  discipline, `FanoutInflight{adapter_class}` Gauge.
- `pkg/types/adapter.go:28-45` — Adapter interface.
- `pkg/types/capabilities.go:38-62` — Capabilities (no
  RecommendedTimeout).
- `pkg/types/query.go:18-44` — Query (Deadline + Filters + Cursor).
- `pkg/types/normalized_doc.go:40-106` — NormalizedDoc, Validate,
  CanonicalHash.
- `pkg/types/errors.go:14-218` — SourceError, Category, CategorizeError,
  OutcomeFromError.
- `internal/adapters/registry.go:75-167` — Registry lifecycle.
- `internal/adapters/registry.go:172-263` — wrappedAdapter sole-emitter
  pattern.
- `internal/router/routing_decision.go:23-37` — RoutingDecision input
  contract.
- `internal/router/router.go:258-294` — selectAdapterSet algorithm.
- `internal/router/category.go:90-111` — CategoryEligibleDocTypes.
- `internal/obs/metrics/metrics.go:89-95` — FanoutInflight registration.
- `internal/obs/metrics/metrics.go:139` — Gauge pre-init.
- `internal/obs/metrics/metrics.go:171` — `adapter_class` allowlist.
- `internal/fanout/fanout.go:1-3` — current 4-line stub.
- `cmd/usearch/query.go:316-368` — placeholder `runFanout` to be
  retired.
- `cmd/usearch/query.go:208` — call site
  (`docs, adapterErrs := runFanout(...)`).
- `cmd/usearch/integration_test.go:100-118` — E2E stub-server pattern
  template.
- `internal/adapters/reddit/bench_test.go` — `goleak.VerifyTestMain`
  reference pattern.
- `.moai/project/roadmap.md:47` — M3 row "SPEC-FAN-001 | Multi-source
  fanout | goroutine pool, per-adapter timeout, partial-result
  assembly, deduplication".
- `.moai/project/roadmap.md:117-128` — M3 parallelization plan.
- `.moai/project/roadmap.md:150` — M3 exit criterion.
- `.moai/project/structure.md:17` — `internal/fanout/` reservation.
- `.moai/project/structure.md:160` — `pkg/types` SDK boundary clause.
- `.moai/config/sections/quality.yaml` — `development_mode: tdd`,
  `test_coverage_target: 85`.
- `.moai/config/sections/harness.yaml` — auto-routing to standard
  level.
- `.moai/config/sections/language.yaml` — `documentation: en`,
  `code_comments: en`.
- `go.mod:30` — `go.uber.org/goleak v1.3.0` indirect.
- `go.mod:33` — `golang.org/x/sync v0.20.0` indirect (provides
  errgroup).

---

*End of SPEC-FAN-001 v0.1 (status: approved after plan-auditor cycle-2)*
