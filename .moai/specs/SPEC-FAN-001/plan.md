# SPEC-FAN-001 Implementation Plan (Post-Hoc)

Generated: 2026-05-26 (reverse-engineered from implemented code)
Methodology: TDD (RED-GREEN-REFACTOR) — completed 2026-05-07
Coverage target: 85% (achieved: 98.1%)
Harness: standard
Status: implemented (verified against `internal/fanout/`)

---

## 1. Overview

This plan.md is a post-hoc summary of the SPEC-FAN-001 implementation
that already shipped (status: `implemented`, spec.md HISTORY 2026-05-07,
commit `04308b8`). The original RED-GREEN-REFACTOR cycle has completed;
this document reconstructs the milestone breakdown so SPEC-FAN-001 has
the canonical 3-file SPEC layout.

SPEC-FAN-001 delivers the **multi-source dispatch orchestrator** at
`internal/fanout/`. The package was a 4-line stub before this SPEC; it
now totals 9 source files + 11 test files = 2,232 LOC. The orchestrator
consumes `router.RoutingDecision` and returns `fanout.Result{Docs,
AdapterErrors, Stats}`.

The implementation:

- Builds a bounded goroutine pool via
  `golang.org/x/sync/errgroup.SetLimit(N)` with the suppress-error
  idiom (workers return `nil` even on adapter error so first-error
  cancellation does not kill siblings).
- Derives per-adapter ctx via
  `context.WithTimeout(parent, min(Options.PerAdapterTimeout,
  remaining-to-parent-deadline))`.
- Collects partial results when parent ctx fires; records incomplete
  adapters in `Result.AdapterErrors[name]`.
- Deduplicates via 8-rule URL canonicalization (PRIMARY key) with
  `NormalizedDoc.CanonicalHash()` fallback (SECONDARY tie-breaker for
  unparseable URLs).
- Sorts by `Score` descending, adapter-name ascending, `RetrievedAt`
  descending as tie-breakers.
- Emits one OTel parent span `fanout.dispatch`, one slog summary, and
  reuses the pre-registered `FanoutInflight{adapter_class}` Gauge (no
  new metric families).
- Adds two distinct sentinels: `ErrAdapterRegistryEmpty` (returned by
  `New`) and `ErrEmptyAdapterSet` (returned by `Dispatch`).

The implementation went through 2 plan-auditor cycles (HISTORY
2026-05-05) that surfaced and resolved 3 HIGH concerns (H1 worker map
writes, H15 Query.Deadline contract, H18 errgroup deadlock) and 5
MEDIUM concerns before final approval.

---

## 2. Phase Breakdown (Post-Hoc Reconstruction)

### Phase A — Type Foundation + URL Canonicalization

Files (implemented):

- `internal/fanout/result.go` (39 LOC) — `Result{Docs []NormalizedDoc,
  AdapterErrors map[string]error, Stats Stats}` + `Stats{AdapterCount,
  SuccessCount, ErrorCount, DedupDropped, ElapsedSeconds}`. JSON-
  marshalable.
- `internal/fanout/options.go` (62 LOC) — `Options{Registry, Obs,
  MaxParallel, PerAdapterTimeout, DefaultDeadline}` with defaults
  (`MaxParallel=8`, `PerAdapterTimeout=8s`, `DefaultDeadline=30s`).
- `internal/fanout/errors.go` (16 LOC) — Sentinels
  `ErrAdapterRegistryEmpty` (for `New`), `ErrEmptyAdapterSet` (for
  `Dispatch`), `ErrAdapterNotFound` (wrapped in worker errors).
- `internal/fanout/canonical.go` (101 LOC) — `canonicalURL(raw string)
  (string, error)` implementing 8 normalization rules (lowercase
  scheme + host, strip fragment, strip 11 tracking params, trim
  trailing slash unless root, sort query keys, preserve path case +
  percent-encoding). Pure function.
- `internal/fanout/canonical_test.go` (134 LOC) — table-driven over
  15+ URL inputs covering all 8 rules + 11 tracking params.
- `internal/fanout/options_test.go` (56 LOC) — `New` validation (nil
  registry, empty registry, zero MaxParallel, negative timeout).

REQ coverage: REQ-FAN-001 (constructor + sentinel), REQ-FAN-006
(dedup tie-breaker URL canonicalization).

### Phase B — Dedup + Sort Helpers

Files (implemented):

- `internal/fanout/dedup.go` (54 LOC) — `dedupDocs(docs []NormalizedDoc)
  ([]NormalizedDoc, int)` returning deduped slice + drop count. Uses
  two disjoint namespaces in the seen-map: `url:<canonical-url>` for
  parseable URLs, `hash:<canonical-hash>` for unparseable. First-
  occurrence wins for collisions. Pure function.
- `internal/fanout/dedup_test.go` (175 LOC) — covers same-URL same-
  content, same-URL different-content, different-URL same-content
  (hash fallback), mixed valid/invalid URLs (H11 fix).
- `internal/fanout/sort.go` (31 LOC) — `sortDocs(docs)` mutates in place
  via `sort.SliceStable` with ordering `(Score desc, adapter-name asc,
  RetrievedAt desc)`.
- `internal/fanout/sort_test.go` (129 LOC) — table-driven ordering
  including equal-Score adapter-name tie-break + RetrievedAt secondary
  tie-break.

REQ coverage: REQ-FAN-005 (dedup), REQ-FAN-006 (sort ordering),
REQ-FAN-007 (deterministic).

### Phase C — Dispatch Orchestration

Files (implemented):

- `internal/fanout/dispatch.go` (164 LOC) — `dispatch(ctx, obs,
  registry, maxParallel, perAdapterTimeout, decision, q) (*Result,
  error)`. Internal helper called from `Fanout.Dispatch`. Implements:
  - Per-adapter `errgroup.SetLimit(maxParallel)`.
  - `deriveAdapterCtx(parent, perAdapterTimeout)` — clamps to
    `min(perAdapterTimeout, parent.Remaining())`.
  - `ctx.Err()` check BEFORE every `eg.Go` (H18 fix: prevents deadlock
    when ctx is already cancelled with queued workers).
  - Per-adapter goroutine with `defer recover()` converting panics to
    `*types.SourceError{Category: CategoryUnknown}` (D7 locked
    decision).
  - Per-adapter `FanoutInflight{adapter_class}.Inc()` / `.Dec()`
    around the registry call.
  - Workers `return nil` even on per-adapter error (soft-fail
    discipline).
  - Per-index pre-allocated `[]error` and `[][]NormalizedDoc` slices
    written by workers (H1 fix: no concurrent map writes).
  - Supervisor goroutine builds `Result.AdapterErrors` from per-index
    slices AFTER `eg.Wait()`.
- `internal/fanout/dispatch_test.go` (495 LOC) — partial-failure,
  per-adapter timeout, panic recovery, ctx cancellation mid-flight,
  empty AdapterSet path, already-cancelled-ctx path, cancelled-mid-
  queue path.
- `internal/fanout/dispatch_internal_test.go` (123 LOC) — internal
  helper tests (ctx derivation, registry lookup, panic recovery).
- `internal/fanout/concurrent_test.go` (59 LOC) — 50 caller goroutines
  × 100 calls × 5 stub adapters = 25,000 invocations under `go test
  -race`.

REQ coverage: REQ-FAN-002 (dispatch entry), REQ-FAN-003 (partial-
result), REQ-FAN-004 (per-adapter timeout), REQ-FAN-009 (concurrent
safety), REQ-FAN-011 (panic recovery), REQ-FAN-012 (ctx-cancellation
guard), REQ-FAN-013 (Query.Deadline caller-honour).

### Phase D — Public Surface + Observability + CLI Migration

Files (implemented):

- `internal/fanout/fanout.go` (97 LOC) — `Fanout` struct + `New(opts)
  (*Fanout, error)` + `Dispatch(ctx, decision, q) (*Result, error)`.
  Dispatch invokes the internal `dispatch` helper, then calls
  `dedupDocs` and `sortDocs` for post-processing, and finally emits
  observability.
- `internal/fanout/fanout_test.go` (75 LOC) — orchestration tests
  (happy path, partial failure, all-failure, empty AdapterSet
  returns sentinel).
- `internal/fanout/observability.go` (106 LOC) — `emitDispatch(ctx, obs,
  span, decision, result)` writing `fanout.dispatch` span attributes
  (`fanout.category`, `fanout.adapter_count`, `fanout.result_count`,
  `fanout.errors_count`, `fanout.dedup_dropped`) + slog summary record.
  Nil-safe across `*obs.Obs`, `Obs.Metrics`, `Obs.Logger`.
- `internal/fanout/observability_test.go` (156 LOC) — span attrs +
  slog record + nil-safe.
- `internal/fanout/testhelpers_test.go` (105 LOC) — shared stub
  adapter constructors for orchestration tests.
- `internal/fanout/bench_test.go` (55 LOC) — `BenchmarkDispatch5Adapters`
  (NFR-FAN-001 + NFR-FAN-004) + `TestMain` calling
  `goleak.VerifyTestMain(m)`.
- `cmd/usearch/query.go` — Migration: removed inline `runFanout`
  (lines 324-368) + `@MX` placeholders (lines 316, 320-323). Call
  site at line 208 now invokes `fanout.New` + `fanout.Dispatch`.
- `cmd/usearch/integration_test.go` — E2E tests updated to use the new
  shape.

REQ coverage: REQ-FAN-007 (deterministic output), REQ-FAN-010
(observability), and all remaining NFRs.

---

## 3. Test Catalog Summary

| Phase | Source LOC | Test LOC | REQs Covered | NFRs Covered |
|-------|------------|----------|--------------|--------------|
| A | 218 (result + options + errors + canonical) | 190 | 001, 006 | — |
| B | 85 (dedup + sort) | 304 | 005, 006, 007 | — |
| C | 164 (dispatch internal) | 677 (dispatch + internal + concurrent) | 002, 003, 004, 009, 011, 012, 013 | 002 |
| D | 97 (Fanout + observability) | 391 (obs + helpers + bench + Fanout) | 007, 008, 010 | 001, 003, 004 |
| **Totals** | **564 src** | **1,562 test** + **9 source files = 2,232 LOC** | **13 / 13** | **4 / 4** |

Test count: 51 passing tests + 1 benchmark. Coverage 98.1% (target 85%).

---

## 4. Risk Mitigation Table (Plan-Auditor Resolved)

| Risk (concern ID from spec.md HISTORY) | Realised? | Resolution |
|----------------------------------------|-----------|------------|
| H1: Concurrent map writes to `Result.AdapterErrors` | Yes (cycle-1 finding) | Workers write to per-index pre-allocated `[]error` slices; supervisor builds the map AFTER `eg.Wait()`. REQ-FAN-002 acceptance text reworded. |
| H15: Caller-vs-callee responsibility for `Query.Deadline` | Yes (cycle-1 finding) | §2.7 added explicit contract: caller MUST apply `Query.Deadline` to parent ctx via `context.WithDeadline` BEFORE invoking `Dispatch`. New REQ-FAN-013 makes it testable. |
| H18: `errgroup.SetLimit` deadlock under cancelled-ctx-with-queued-workers | Yes (cycle-1 finding) | Explicit guard added: `Dispatch` checks `ctx.Err()` BEFORE every `eg.Go` and short-circuits by recording `context.Canceled` for un-launched adapters. New REQ-FAN-012 makes it testable. |
| H11: Mixed valid/invalid URL dedup collision | Yes (cycle-1 finding) | Dedup map uses two disjoint namespaces (`url:` and `hash:`). `TestDedupMixedValidInvalidURL` added. |
| H14: `Stats.AdapterCount` invariant | Yes (cycle-1 finding) | Computed structurally in `assembleResult`: `AdapterCount = len(decision.AdapterSet)`; `SuccessCount + ErrorCount = AdapterCount`. |
| H16/H17: Sentinel naming `ErrEmptyAdapterSet` ambiguity | Yes (cycle-1 finding) | Split into two distinct sentinels: `ErrAdapterRegistryEmpty` (for `New`) + `ErrEmptyAdapterSet` (for `Dispatch`). |
| N1: `eg.Go` race window between `ctx.Err()` check and invocation | Yes (cycle-2 finding) | §2.5 added "Race window note" paragraph documenting worst-case wait bound (≤ perAdapterTimeout, never infinite). |
| N7: `Stats` invariant "debug-build-only panic" without build tag | Yes (cycle-2 finding) | Revised to compute `Stats` structurally in `assembleResult` with no runtime assertion. |

---

## 5. MX Tag Plan (Applied in Source)

### 5.1 @MX:ANCHOR

- `internal/fanout/fanout.go::Fanout` (line 24) — `@MX:ANCHOR` (sole
  entry point for all multi-source fanout dispatches; fan_in ≥ 3 from
  `cmd/usearch/query.go`, tests, future SPEC-MCP-001). `@MX:REASON`:
  contract boundary; signature change ripples to CLI-001 + MCP-001 +
  IDX-001.
- `internal/fanout/dedup.go::dedupDocs` (line 26 per spec) — `@MX:ANCHOR`
  (sole dedup transform; every fanned-out doc passes through this).

### 5.2 @MX:WARN

- `internal/fanout/fanout.go::Dispatch` (line 58) — `@MX:WARN`
  (Dispatch spawns up to `MaxParallel` goroutines via errgroup).
  `@MX:REASON`: removing per-goroutine `defer recover()` /
  `FanoutInflight.Dec()` invalidates NFR-FAN-003 zero-leak guarantee.
- `internal/fanout/dispatch.go` (line 82 per spec) — `@MX:WARN`
  (per-adapter goroutine spawn). `@MX:REASON`: suppress-error idiom
  (workers return nil) prevents first-error cancellation; removing it
  invalidates partial-result discipline.

### 5.3 @MX:NOTE

- D1 (errgroup choice), D2 (per-adapter timeout policy), D3 (partial-
  result assembly) documented inline as `@MX:NOTE` per the SPEC §1
  HISTORY notes 4 NOTE tags.

---

## 6. File Touch Order (as realised)

1. Phase A: `result.go` → `options.go` → `errors.go` → `canonical.go`
   → `canonical_test.go` → `options_test.go`.
2. Phase B: `dedup.go` → `dedup_test.go` → `sort.go` → `sort_test.go`.
3. Phase C: `dispatch.go` → `dispatch_test.go` → `dispatch_internal_test.go`
   → `concurrent_test.go`.
4. Phase D: `fanout.go` → `fanout_test.go` → `observability.go` →
   `observability_test.go` → `testhelpers_test.go` → `bench_test.go`
   → `cmd/usearch/query.go` migration.

---

## 7. Coverage and Quality Gates (Achieved)

- Coverage on `internal/fanout/`: 98.1% (target 85%).
- 51 tests passing + 1 benchmark.
- `go vet ./internal/fanout/...` → 0 issues.
- `golangci-lint run ./internal/fanout/...` → 0 issues.
- `go test -race ./internal/fanout/...` → PASS (NFR-FAN-002).
- `goleak.VerifyTestMain` clean (NFR-FAN-003).
- Full project build PASS, no regression in 14 dependent packages.
- LSP gate: zero errors / zero type errors / zero lint errors.

---

## 8. Pre-submission Self-Review

Verified at implementation time:

- `Fanout.Dispatch` is the sole public entry point (`internal/fanout`
  has no other exported functions besides `New` and type constructors).
- `dispatch.go::dispatch` checks `ctx.Err()` BEFORE every `eg.Go` (H18
  fix verified).
- Per-adapter goroutines `return nil` even on error (suppress-error
  idiom verified by `TestDispatchOneAdapterFailsOthersSucceed`).
- Workers write to per-index pre-allocated slices, NOT to the shared
  map (H1 fix verified by race-detector run).
- `dedup.go::dedupDocs` uses two disjoint namespaces (`url:` and
  `hash:`); verified by `TestDedupMixedValidInvalidURL`.
- `canonical.go::canonicalURL` strips exactly 11 tracking params; the
  list is package-level immutable.
- `Stats.AdapterCount == len(decision.AdapterSet)` and
  `Stats.SuccessCount + Stats.ErrorCount == Stats.AdapterCount`
  invariants hold structurally (computed in `assembleResult`).
- `Result.AdapterErrors` is `nil` when `Stats.ErrorCount == 0`,
  non-nil map otherwise.
- ZERO new Prometheus metric families (reuses pre-registered
  `FanoutInflight{adapter_class}` from SPEC-OBS-001).
- ZERO new cardinality allowlist entries.

---

## 9. Downstream Wiring

- **SPEC-IDX-001** consumes `fanout.Result.Docs` (`[]NormalizedDoc`) as
  the input to its `Upsert` ingestion path. The CLI orchestrates
  `fanout.Dispatch` → `index.Upsert` (OQ §11.5 default: CLI
  orchestrates).
- **SPEC-CACHE-001** (M3) wraps `fanout.Dispatch` in the 5-phase
  access fallback try-then-degrade harness.
- **SPEC-ADP-003 through SPEC-ADP-009** (the 7 M3 adapter SPECs) all
  ship adapters that the fanout consumes via the registry's
  `wrappedAdapter` indirection (sole-emitter discipline preserved).
- **SPEC-SYN-001** (M2, implemented before FAN-001) consumes the
  CLI-assembled `[]NormalizedDoc` — synthesis is downstream of fanout
  in the request pipeline.
- **SPEC-MCP-001** (M7, future) and SPEC-API-001 (post-V1) will
  re-expose `fanout.Dispatch` over MCP / HTTP. The current `*Fanout`
  surface is library-only.

---

*End of SPEC-FAN-001 plan.md (post-hoc).*
