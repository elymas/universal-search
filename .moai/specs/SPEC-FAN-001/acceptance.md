# SPEC-FAN-001 Acceptance Scenarios

Generated: 2026-05-26 (reverse-engineered from implemented code)
Format: Given / When / Then with verifying test references.
Status: SPEC implemented (2026-05-07 commit `04308b8`); scenarios
reflect realised behaviour.

This document enumerates the testable acceptance criteria for
SPEC-FAN-001 (multi-source fanout orchestrator). Verifying tests live
under `internal/fanout/`. Total: 51 tests + 1 benchmark; coverage
98.1%.

---

## AC-001 — `New` validates registry non-empty

**Coverage**: REQ-FAN-001 (Ubiquitous)

### Given

- `Options{Registry: nil, Obs: &obs.Obs{}}` (nil registry).

### When

`fanout.New(opts)` is invoked.

### Then

- Returns `(nil, ErrAdapterRegistryEmpty)`.
- `errors.Is(err, ErrAdapterRegistryEmpty)` is `true`.

### Boundary: Empty registry (zero adapters)

`Options{Registry: emptyRegistry}` (registry constructed but no
adapters registered) → same return: `(nil, ErrAdapterRegistryEmpty)`.

### Boundary: Defaults applied

`Options{Registry: validRegistry}` with `MaxParallel=0`,
`PerAdapterTimeout=0`, `DefaultDeadline=0` → `New` returns `(*Fanout,
nil)` with defaults applied: `MaxParallel=8`, `PerAdapterTimeout=8s`,
`DefaultDeadline=30s`.

**Verifying tests**: `TestNewRequiresNonEmptyRegistry`,
`TestNewAppliesDefaults` in `internal/fanout/options_test.go`.

---

## AC-002 — Dispatch happy path: all adapters return docs

**Coverage**: REQ-FAN-002 (Event-Driven)

### Given

- A registry of 3 stub adapters, each returning 3 docs on `Search`.
- A `RoutingDecision{AdapterSet: ["a1","a2","a3"], Category: "web"}`.
- A parent ctx with a generous deadline.

### When

`fanout.Dispatch(ctx, decision, types.Query{Text: "x"})` is invoked.

### Then

- Returns `(*Result, nil)`.
- `result.Docs` is non-empty (after dedup; if no overlap, 9 docs).
- `result.AdapterErrors` is nil (no errors).
- `result.Stats.AdapterCount == 3`.
- `result.Stats.SuccessCount == 3`.
- `result.Stats.ErrorCount == 0`.
- `result.Stats.DedupDropped >= 0` (could be > 0 if adapters return
  overlapping docs).
- `result.Stats.ElapsedSeconds > 0`.

**Verifying test**: `TestDispatchHappyPath3Adapters` in
`internal/fanout/fanout_test.go`.

---

## AC-003 — Dispatch returns ErrEmptyAdapterSet for empty AdapterSet

**Coverage**: REQ-FAN-008 (Unwanted)

### Given

- A valid `*Fanout` instance.
- A `RoutingDecision{AdapterSet: []}` (empty slice).

### When

`fanout.Dispatch(ctx, decision, q)` is invoked.

### Then

- Returns `(*Result, ErrEmptyAdapterSet)`.
- `result.Docs == []NormalizedDoc{}` (empty slice).
- `result.Stats.AdapterCount == 0`.
- `errors.Is(err, ErrEmptyAdapterSet)` is `true`.
- One slog record emitted with `event_type:
  "fanout.dispatch.empty_adapter_set"`.

**Verifying test**: `TestDispatchEmptyAdapterSetReturnsSentinel` in
`internal/fanout/fanout_test.go`.

---

## AC-004 — Partial failure: surviving adapters contribute, failed recorded

**Coverage**: REQ-FAN-003 (Event-Driven), REQ-FAN-009 (concurrent
safety boundary)

### Given

- 3 stub adapters: `a1` returns 5 docs successfully; `a2` returns
  `*types.SourceError{Category: CategoryUnavailable}`; `a3` returns
  3 docs successfully.

### When

`fanout.Dispatch(ctx, decision, q)` is invoked.

### Then

- Returns `(*Result, nil)`.
- `result.Docs` contains the union of `a1` + `a3` docs (after dedup
  + sort).
- `result.AdapterErrors["a2"]` is non-nil and wraps the
  `*types.SourceError`.
- `result.AdapterErrors["a1"]` and `["a3"]` are absent.
- `result.Stats.SuccessCount == 2`.
- `result.Stats.ErrorCount == 1`.
- The `errgroup` first-error-cancel did NOT kill `a1` or `a3` (verified
  by `a3` returning docs even though `a2` errored).

**Verifying tests**: `TestDispatchOneAdapterFailsOthersSucceed`,
`TestDispatchAllAdaptersFailReturnsResultWithErrors` in
`internal/fanout/dispatch_test.go`.

---

## AC-005 — Per-adapter timeout honors the smaller bound

**Coverage**: REQ-FAN-004 (State-Driven)

### Given

- `Options.PerAdapterTimeout = 8 * time.Second`.
- A stub adapter that sleeps 5 seconds before returning.
- A parent ctx with `WithDeadline(now + 100*time.Millisecond)`.

### When

`fanout.Dispatch(ctx, decision, q)` is invoked.

### Then

- The adapter's ctx is cancelled at ~100ms (the smaller of
  `PerAdapterTimeout` and remaining-to-parent-deadline).
- `result.AdapterErrors[adapterName]` wraps `context.DeadlineExceeded`.
- Total elapsed wall-clock ≤ 150ms (timeout + cleanup).

### Boundary: No parent deadline

Parent ctx without deadline → per-adapter ctx uses
`Options.PerAdapterTimeout` directly (8s).

### Boundary: Per-adapter timeout > parent remaining

Parent has 50ms remaining; `PerAdapterTimeout=8s` → adapter ctx
cancelled at ~50ms.

**Verifying tests**: `TestDispatchPerAdapterTimeout`,
`TestDispatchParentDeadlineClampsPerAdapter`,
`TestDispatchNoParentDeadlineUsesFloor` in
`internal/fanout/dispatch_test.go`.

---

## AC-006 — Dedup: same-URL different-content first wins

**Coverage**: REQ-FAN-005 (Event-Driven), REQ-FAN-006 (Optional
tie-breaker)

### Given

- 3 docs: `D1 {URL: "https://x.com/a", Title: "Original"}`,
  `D2 {URL: "https://x.com/a", Title: "Edited"}` (same URL, different
  content), `D3 {URL: "https://x.com/b", Title: "Other"}`.

### When

`dedupDocs([]NormalizedDoc{D1, D2, D3})` is invoked.

### Then

- Returns `([D1, D3], 1)` — `D2` is dropped, drop count = 1.
- First-occurrence-wins discipline preserved (`D1` retained).

### Boundary: Different-URL same-content (mirror sites)

`D1 {URL: "https://example.com/a", Body: "X"}`,
`D2 {URL: "https://mirror.com/a", Body: "X"}` → URL parses differently;
both URLs emit; hash fallback does NOT engage (because both URLs
parse). The deeper near-dup case is SPEC-SYN-003's domain.

### Boundary: Mixed valid/invalid URLs (H11 fix)

`D1` with parseable URL, `D2` with unparseable URL (e.g., scheme-less
or empty) → live in DIFFERENT namespaces (`url:` vs `hash:`). Even if
underlying bytes coincide, they are NEVER merged.

**Verifying tests**: `TestDedupSameURLDifferentContentFirstWins`,
`TestDedupSameURLSameContentDropsDuplicate`,
`TestDedupDifferentURLSameContentEmitsBoth`,
`TestDedupMixedValidInvalidURL` in `internal/fanout/dedup_test.go`.

---

## AC-007 — Sort order: Score desc, adapter asc, RetrievedAt desc

**Coverage**: REQ-FAN-006 (Optional sort), REQ-FAN-007 (deterministic)

### Given

- 5 docs with varied (Score, SourceID, RetrievedAt) tuples designed to
  exercise all three tie-breakers.

### When

`sortDocs(docs)` is invoked (mutates slice in place).

### Then

- Output ordered by:
  1. PRIMARY: `Score` descending.
  2. TIE-BREAKER 1: `SourceID` ascending (adapter name lex-sorted per
     IR-001).
  3. TIE-BREAKER 2: `RetrievedAt` descending (newer first).
- Same input → byte-equal output (deterministic via `sort.SliceStable`).

**Verifying tests**: `TestSortByScoreDescending`,
`TestSortStableForEqualScores`,
`TestSortAdapterNameAscendingTieBreak`,
`TestSortRetrievedAtDescendingSecondaryTieBreak`,
`TestSortDeterministicAcrossRuns` in
`internal/fanout/sort_test.go`.

---

## AC-008 — URL canonicalization strips tracking + sorts query

**Coverage**: REQ-FAN-006 (canonicalization rules)

### Given

- 6 input URLs covering all 8 canonicalization rules.

### When

`canonicalURL(rawURL)` is invoked.

### Then

| Input | Output |
|-------|--------|
| `https://www.reddit.com/r/golang/comments/abc/title?utm_source=newsletter&utm_medium=email#section1` | `https://www.reddit.com/r/golang/comments/abc/title` |
| `HTTPS://Example.COM/Path/?b=2&a=1` | `https://example.com/Path?a=1&b=2` |
| `https://news.ycombinator.com/item?id=12345` | `https://news.ycombinator.com/item?id=12345` |
| `https://example.com/?gclid=xyz&fbclid=abc&id=42` | `https://example.com/?id=42` |
| `https://example.com/foo/bar/` | `https://example.com/foo/bar` |
| `https://example.com/` | `https://example.com/` (root preserved) |

### Boundary: 11 tracking params stripped

All of `utm_source`, `utm_medium`, `utm_campaign`, `utm_term`,
`utm_content`, `gclid`, `fbclid`, `mc_eid`, `mc_cid`, `_ga`, `ref`,
`ref_src` are removed.

### Boundary: Path case preserved

`https://example.com/Foo/Bar` → `https://example.com/Foo/Bar` (HTTP
paths are case-sensitive).

### Boundary: Unparseable URL returns error

`canonicalURL("not a url")` returns a non-nil error; caller falls back
to `NormalizedDoc.CanonicalHash()` for dedup.

**Verifying tests**: `TestCanonicalURLTable` (15+ entries) in
`internal/fanout/canonical_test.go`.

---

## AC-009 — Panic in adapter is recovered, converted to SourceError

**Coverage**: REQ-FAN-011 (Event-Driven)

### Given

- A stub adapter whose `Search` panics with a string message.

### When

`fanout.Dispatch(ctx, decision, q)` is invoked.

### Then

- `defer recover()` in the per-adapter goroutine catches the panic.
- `result.AdapterErrors[adapterName]` is non-nil and wraps a
  `*types.SourceError{Category: CategoryUnknown, Cause: fmt.Errorf("adapter %q
  panicked: %v", name, r)}`.
- Stack trace logged at WARN via `runtime/debug.Stack()`.
- Other (non-panicking) adapters complete normally.
- `usearch query` does NOT crash.

**Verifying tests**: `TestDispatchAdapterPanicRecovered`,
`TestDispatchPanicDoesNotKillSiblings` in
`internal/fanout/dispatch_test.go`.

---

## AC-010 — ctx cancellation BEFORE eg.Go records remaining adapters

**Coverage**: REQ-FAN-012 (H18 fix — guard against errgroup deadlock)

### Given

- A 5-adapter decision; parent ctx is cancelled BEFORE `Dispatch` is
  invoked.

### When

`fanout.Dispatch(cancelledCtx, decision, q)` is invoked.

### Then

- `Dispatch` checks `ctx.Err() != nil` BEFORE each `eg.Go`.
- Un-launched adapters are recorded with
  `result.AdapterErrors[name] = context.Canceled`.
- `result.Stats.ErrorCount == 5` (all 5 recorded as errors).
- `result.Stats.SuccessCount == 0`.
- No goroutine leak (verified by `goleak.VerifyNone`).
- Returns `(*Result, nil)` (NOT an error at the call level).

### Boundary: ctx cancelled mid-queue

If `ctx` is cancelled after the first 2 adapters launch but before the
remaining 3 are queued → first 2 may complete or be cancelled
(depending on timing); remaining 3 record `context.Canceled`.

**Verifying tests**: `TestDispatchAlreadyCancelledCtx`,
`TestDispatchCancelledMidQueue` in
`internal/fanout/dispatch_test.go`.

---

## AC-011 — Query.Deadline is a caller-honour contract

**Coverage**: REQ-FAN-013 (H15 fix — Query.Deadline contract)

### Given

- A `Query{Deadline: 1 * time.Second}` and a parent ctx with NO
  deadline applied.

### When

`fanout.Dispatch(parentCtx, decision, q)` is invoked.

### Then

- The fanout does NOT internally consume `q.Deadline`.
- Per-adapter timeout uses `Options.PerAdapterTimeout` (no clamping
  via `Query.Deadline`).

### Boundary: Correct usage (caller applies deadline)

```
ctx, cancel := context.WithDeadline(ctx, time.Now().Add(q.Deadline))
defer cancel()
result, err := f.Dispatch(ctx, decision, q)
```

→ per-adapter timeout correctly clamped to remaining-to-deadline.

**Verifying tests**: `TestDispatchDoesNotConsumeQueryDeadline`,
`TestDispatchCallerAppliedDeadlineHonored` in
`internal/fanout/dispatch_test.go`.

---

## AC-012 — Per-call observability emits OTel + slog + Gauge

**Coverage**: REQ-FAN-010 (Ubiquitous)

### Given

- A `*Fanout` constructed with a non-nil `*obs.Obs` carrying an
  in-memory OTel exporter and Prometheus registry snapshot.

### When

`f.Dispatch(ctx, decision, q)` is invoked.

### Then

- Exactly ONE OTel parent span named `fanout.dispatch` with attributes:
  - `fanout.category` (from `decision.Category`).
  - `fanout.adapter_count`.
  - `fanout.result_count`.
  - `fanout.errors_count`.
  - `fanout.dedup_dropped`.
- Per-adapter child spans (`adapter.search`) are children via ctx
  propagation through the registry's `wrappedAdapter` (sole-emitter
  discipline; no new span created in fanout itself).
- `FanoutInflight{adapter_class}.Inc()` and `.Dec()` invoked exactly
  once around each adapter dispatch.
- ONE slog record at INFO with attributes `{request_id, adapter_count,
  result_count, errors_count, dedup_dropped, elapsed_seconds}`.

### Boundary: ZERO new Prometheus metric families

A snapshot of `prometheus.Registry.Gather()` before + after
construction confirms NO new metric families beyond the pre-registered
`FanoutInflight`.

### Boundary: Nil obs is safe

Construct Fanout with `Obs: nil`; `Dispatch` does not panic; returns
valid `*Result`.

**Verifying tests**: `TestDispatchEmitsParentSpan`,
`TestDispatchEmitsFanoutInflight`, `TestDispatchEmitsSlogSummary`,
`TestDispatchEmitsSafeOnNilObs`, `TestDispatchAddsNoNewMetricFamilies`
in `internal/fanout/observability_test.go`.

---

## AC-013 — Stats invariants hold structurally

**Coverage**: REQ-FAN-002, REQ-FAN-009 (H14 invariant)

### Given

- Any successful `Dispatch` call.

### When

The returned `*Result.Stats` is inspected.

### Then

- `Stats.AdapterCount == len(decision.AdapterSet)`.
- `Stats.SuccessCount + Stats.ErrorCount == Stats.AdapterCount`.
- `Result.AdapterErrors == nil` when `Stats.ErrorCount == 0`.
- `Result.AdapterErrors` is non-nil map with exactly
  `Stats.ErrorCount` entries when `Stats.ErrorCount > 0`.

**Verifying tests**: `TestStatsAdapterCountMatchesAdapterSet`,
`TestStatsSuccessPlusErrorEqualsAdapterCount`,
`TestAdapterErrorsNilWhenZeroErrors` in
`internal/fanout/dispatch_test.go`.

---

## Edge Cases

### EC-001 — Race-clean concurrent Dispatch invocations

**Coverage**: REQ-FAN-009 (State-Driven), NFR-FAN-002

#### Given

- One `*Fanout` instance.
- 50 caller goroutines × 100 `Dispatch` calls each × 5 stub adapters
  = 25,000 invocations.

#### When

`go test -race ./internal/fanout/...` is executed.

#### Then

- Zero race-detector alarms attributable to the fanout package
  (race-detector tolerance ≤ 10 false positives accepted but expected 0).
- `goleak.VerifyTestMain` clean at test end.
- All 25,000 calls return valid `*Result`.

**Verifying test**: `TestDispatchConcurrent` in
`internal/fanout/concurrent_test.go`.

### EC-002 — All adapters fail, result is still non-nil

**Coverage**: REQ-FAN-003 (soft-fail boundary)

#### Given

- All 3 stub adapters return errors.

#### When

`fanout.Dispatch(ctx, decision, q)` is invoked.

#### Then

- Returns `(*Result, nil)` — Dispatch NEVER returns an error for
  all-fail (only `ErrEmptyAdapterSet` for empty AdapterSet).
- `result.Docs == []NormalizedDoc{}` (empty).
- `result.AdapterErrors` has 3 entries.
- `result.Stats.SuccessCount == 0`, `Stats.ErrorCount == 3`.

**Verifying test**: `TestDispatchAllAdaptersFailReturnsResultWithErrors`.

---

## NFR Coverage

| NFR | Verifying Test | Threshold |
|-----|----------------|-----------|
| NFR-FAN-001 (fanout overhead p50/p95) | `BenchmarkDispatch5Adapters` | p50 + p95 minimal (≤ adapter ceiling + a few ms) |
| NFR-FAN-002 (race-clean) | `TestDispatchConcurrent` + `go test -race` | 0 alarms |
| NFR-FAN-003 (zero goroutine leaks) | `goleak.VerifyTestMain` in `bench_test.go` | 0 leaks |
| NFR-FAN-004 (alloc/op ceiling) | `BenchmarkDispatch5Adapters` reports `allocs/op` | Measured at ship; tunable |

---

## Definition of Done (Verified at 2026-05-07)

- [x] 9 source files + 11 test files = 2,232 LOC under `internal/fanout/`.
- [x] 13 EARS REQs (REQ-FAN-001 through REQ-FAN-013) covered by 51
      passing tests.
- [x] 4 NFRs verified (race-clean + goleak + bench).
- [x] Coverage 98.1% (target 85%).
- [x] `go vet ./internal/fanout/...` → 0 issues.
- [x] `golangci-lint run ./internal/fanout/...` → 0 issues.
- [x] `go test -race ./internal/fanout/...` PASS.
- [x] Full project build PASS.
- [x] No regression in 14 dependent packages.
- [x] CLI `cmd/usearch/query.go` migrated from inline `runFanout` to
      `fanout.New` + `fanout.Dispatch`; integration tests still pass.
- [x] ZERO new Prometheus metric families (reuses pre-registered
      `FanoutInflight{adapter_class}`).
- [x] ZERO new cardinality allowlist entries.
- [x] MX tags applied: 2 ANCHOR (`Fanout`, `dedupDocs`), 2 WARN
      (goroutine spawn `Dispatch` line 58 + per-adapter goroutine
      `dispatch.go` line 82), 4 NOTE (locked decisions D1/D2/D3
      inline), 6 `@MX:SPEC: SPEC-FAN-001` anchors.
- [x] All 3 HIGH concerns from plan-auditor cycle 1 resolved
      (H1 worker map writes, H15 Query.Deadline contract, H18 errgroup
      deadlock).
- [x] Conventional commit `feat(fanout): SPEC-FAN-001 multi-source
      dispatch orchestrator` references the SPEC ID (commit
      `04308b8`).

---

*End of SPEC-FAN-001 acceptance.md (post-hoc).*
