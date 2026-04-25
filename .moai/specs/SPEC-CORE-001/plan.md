# SPEC-CORE-001 Plan — Implementation Strategy

Created: 2026-04-26
Author: limbowl (via manager-spec)
Status: draft (with spec.md)
Methodology: TDD (RED-GREEN-REFACTOR) per `.moai/config/sections/quality.yaml`
Coverage Target: 85%

## 0. Plan Scope

This document is the implementation roadmap for SPEC-CORE-001 — the foundational
adapter contract. It decomposes the spec.md acceptance criteria into discrete
RED-GREEN-REFACTOR tasks ordered by dependency, identifies reference patterns
from the existing M1 codebase, and assigns @MX tag targets.

This plan is consumed by the run phase (`/moai run SPEC-CORE-001`) and should
be read alongside spec.md (requirements) and acceptance.md (Given/When/Then
scenarios).

## 1. Approach Summary

**One-paragraph statement of approach**: Implement five public types in
`pkg/types/` (NormalizedDoc, Adapter, Query, Capabilities, plus the error
taxonomy) with strong validation and JSON round-trip fidelity. Add a
concurrency-safe registry in `internal/adapters/` whose `wrappedAdapter`
emits Prometheus + slog + OTel telemetry per Search call by reusing the
collectors SPEC-OBS-001 already registered (AdapterCalls,
AdapterCallDuration) — we ship zero new metric families. Provide a 50-LoC
noop reference adapter as a compile-time interface check and stable test
fixture for downstream SPECs. All work follows TDD: RED tests before any
implementation, with the dependency order Errors → Query → Capabilities →
NormalizedDoc → Adapter interface → Registry → Noop adapter.

## 2. Reference Implementations

These existing files in the repo are the patterns we mirror, cited with
file:line for the run-phase implementer to read first.

| Concern | Reference (file:line) | What we reuse |
|---------|-----------------------|---------------|
| Sentinel errors | `internal/llm/llm.go:108-121` | `var Err... = errors.New("...")` pattern |
| Typed-error classification | `internal/llm/retry.go:14-50` | `*httpStatusError` + `isNonRetryable` pattern; we generalise to `*SourceError` + `CategorizeError` |
| Registry with RWMutex | `internal/llm/router.go:148-198` | `Router` struct shape, `NewRouter` constructor, `Route()` returning ordered slice; we mirror as `Registry` + `NewRegistry` + `List()` |
| Concurrent breaker map | `internal/llm/router.go:151-156` | `map[string]*breaker` under `sync.RWMutex` — we replace with `map[string]types.Adapter` |
| Per-call observability emit | `internal/llm/client.go:230-252` | `emitObservability` shape: 1 slog + 1 counter + 1 histogram + 1 OTel span; we emit on `wrappedAdapter.Search` |
| Nil-safe metric guards | `internal/llm/client.go:244-251` | `if reg != nil && reg.X != nil { reg.X.Inc() }` pattern — applied to AdapterCalls/AdapterCallDuration |
| Obs bundle DI | `internal/obs/obs.go:51-66` | Constructor takes `*obs.Obs`; mirrors LLM client constructor at `internal/llm/client.go:43-65` |
| @MX:ANCHOR placement | `internal/obs/obs.go:49-50,71-72` | High-fan_in functions get `@MX:ANCHOR` + `@MX:REASON` |
| @MX:WARN placement | `internal/obs/metrics/metrics.go:187-188` | Goroutine launch and other "danger zone" patterns get `@MX:WARN` |
| AdapterCalls / AdapterCallDuration collectors | `internal/obs/metrics/metrics.go:86-101` | Already registered; we consume |
| Cardinality allowlist | `internal/obs/metrics/metrics.go:147-154` | `adapter` and `outcome` already in allowlist; we add 5 outcome values |

**Implementer-first reading list**: Before writing any code, the run-phase
implementer should read these 5 files in order:
1. `internal/llm/llm.go` (full) — interface + sentinel patterns
2. `internal/llm/router.go` (full) — registry shape
3. `internal/llm/client.go:43-65,230-252` — DI + emit pattern
4. `internal/obs/obs.go` (full) — Obs bundle contract
5. `internal/obs/metrics/metrics.go:86-156` — existing adapter collectors + allowlist

## 3. Task Decomposition (Priority-Ordered)

The implementation proceeds in **dependency order**. Within each phase, RED
tests precede GREEN code; REFACTOR happens at phase boundaries when shared
helpers emerge.

Priority labels (no time estimates per `.claude/rules/moai/core/agent-common-protocol.md`
"Time Estimation" rule): each priority denotes execution order, not duration.

### Priority 1 (Highest) — Error Taxonomy

**Why first**: Every other type (NormalizedDoc.Validate, registry, wrappedAdapter)
references the error types. Implementing this first prevents circular file
edits.

Tasks:

- T1.1 [RED]: Write `errors_test.go` with TestSentinelErrorsExist,
  TestSourceErrorIsMatchesSentinels, TestSourceErrorUnwrapsCause,
  TestCategorizeErrorTable, TestOutcomeFromErrorTable.
- T1.2 [GREEN]: Create `pkg/types/errors.go` with the four sentinels,
  Category enum, *SourceError struct, Error/Unwrap/Is methods,
  CategorizeError, OutcomeFromError. ~80 LoC.
- T1.3 [REFACTOR]: Verify Is() correctly handles nested *SourceError chains
  (a *SourceError wrapping another *SourceError). Add table case.

Deliverable check:
- All Priority-1 tests in §8 of spec.md (#34-#38) pass.
- `go test -race ./pkg/types/...` clean for errors_test.go.

### Priority 2 — Query and Capabilities

**Why second**: NormalizedDoc references neither, but Adapter interface and
registry do. Decoupling these from NormalizedDoc reduces RED-test coupling.

Tasks:

- T2.1 [RED]: `query_test.go` — TestQueryStructFields (reflect-based shape check).
- T2.2 [GREEN]: `pkg/types/query.go` — Query + Filter structs. ~40 LoC.
- T2.3 [RED]: `capabilities_test.go` — TestCapabilitiesStructFields,
  TestDocTypeEnumComplete.
- T2.4 [GREEN]: `pkg/types/capabilities.go` — Capabilities struct, DocType enum
  with 8 constants. ~60 LoC.

Deliverable check:
- Tests #7, #8, #9 from spec.md §8 pass.

### Priority 3 — NormalizedDoc

**Why third**: Independent of Adapter interface; depends only on `time` stdlib
and pkg/types/capabilities.go (for DocType reference). Implementing before
the interface lets us verify JSON round-trip in isolation.

Tasks:

- T3.1 [RED]: `normalized_doc_test.go` — TestNormalizedDocFieldSet,
  TestNormalizedDocJSONRoundTrip, TestNormalizedDocValidateRequiredFields,
  TestNormalizedDocCanonicalHashStable, TestCanonicalHashIgnoresMetadata,
  TestValidateRejectsMissingID/SourceID/URL/ZeroRetrievedAt,
  TestValidationErrorWrapsFieldName.
- T3.2 [GREEN]: `pkg/types/normalized_doc.go` — NormalizedDoc struct (15
  fields), Validate() method, CanonicalHash() method, ValidationError typed
  error. ~120 LoC.
- T3.3 [REFACTOR]: Extract field-empty checks into a shared helper if the
  Validate() body has 4+ near-identical branches.

Deliverable check:
- Tests #1-#5, #29-#33 pass.
- `BenchmarkNormalizedDocValidate` passes the < 1 µs/op gate (NFR-CORE-001).

### Priority 4 — Adapter Interface and Noop Reference

**Why fourth**: Now that types are stable, define the contract and prove it
with the noop adapter. This is the gate before registry work.

Tasks:

- T4.1 [RED]: `adapter_test.go` — TestAdapterInterfaceShape (reflect-based
  4-method check).
- T4.2 [GREEN]: `pkg/types/adapter.go` — Adapter interface definition. ~30
  LoC including godoc.
- T4.3 [RED]: `noop/noop_test.go` — TestNoopAdapterImplementsInterface (uses
  `var _ types.Adapter = (*noop.Adapter)(nil)` for compile-time check; runtime
  test verifies 4 methods don't panic on cancelled ctx).
- T4.4 [GREEN]: `internal/adapters/noop/noop.go` — `Adapter` struct, `New(name)
  *Adapter`, four method implementations. Under 50 LoC including godoc per
  spec.md §6.6.

Deliverable check:
- Tests #6, #10 pass.
- `go vet ./internal/adapters/noop/...` clean.
- `wc -l internal/adapters/noop/noop.go` reports under 50.

### Priority 5 — Registry (Core)

**Why fifth**: The big-rock task. Requires all prior priorities to be stable.

Tasks:

- T5.1 [RED]: `registry_test.go` Phase A — TestRegisterSucceedsForNewAdapter,
  TestRegisterRejectsDuplicateName, TestRegisterStateUnchangedOnError,
  TestListReturnsSortedNames.
- T5.2 [GREEN]: `internal/adapters/registry.go` — Registry struct with
  sync.RWMutex, NewRegistry, Register (without auth check yet),
  RegisterWithOptions, Get, List. RegistryError typed error.
  ErrDuplicateAdapter sentinel. ~120 LoC for this slice.
- T5.3 [RED]: `registry_test.go` Phase B — TestRegisterValidatesAuthEnvVars,
  TestRegisterAllowsSkipAuthCheck, TestRegisterRequiresAuthEnvVarsTable
  (4-cell truth table).
- T5.4 [GREEN]: Add auth env-var validation to RegisterWithOptions; add
  ErrMissingAuth sentinel. ~30 LoC.

Deliverable check:
- Tests #11-#15, #28 pass.
- `go test -race ./internal/adapters/...` clean.

### Priority 6 — Registry (wrappedAdapter Observability)

**Why sixth**: The high-leverage observability layer; final big-rock.

Tasks:

- T6.1 [RED]: `registry_test.go` Phase C — TestWrappedAdapterEmitsCounter*
  × 5 (one per outcome value: success, failure, timeout, rate_limited,
  unavailable), TestWrappedAdapterEmitsHistogram,
  TestWrappedAdapterCreatesOTelSpan, TestWrappedAdapterEmitsSlogRecord,
  TestWrappedAdapterPreservesUnderlyingError, TestWrappedAdapterSafeOnNilObs.
- T6.2 [GREEN]: Add `wrappedAdapter` to `internal/adapters/registry.go`.
  Search method emits 1 counter + 1 histogram + 1 OTel span + 1 slog record;
  preserves underlying error via `%w`; nil-safe per pattern at
  `internal/llm/client.go:244-251`. ~80 LoC.
- T6.3 [RED]: `registry_test.go` Phase D — TestRegistryConcurrentReadWrite
  (100 readers + 1 writer × 1s under -race), TestAdapterOutcomeLabels (NFR
  cardinality enforcement).
- T6.4 [GREEN]: Confirm RWMutex usage in Get/List/Register is correct;
  verify allowed-outcome enum is enforced.
- T6.5 [REFACTOR]: Extract observability emission to a private
  `(w *wrappedAdapter) emit(ctx, name, outcome, elapsed, count, err)`
  helper. Mirror pattern at `internal/llm/client.go:230-252`.

Deliverable check:
- Tests #16-#27, #41 pass.
- `go test -race ./internal/adapters/...` clean.
- All 5 outcome label values in the SPEC-OBS-001 allowlist post-test.

### Priority 7 — Stub Replacement and Public-API Boundary

**Why last**: Cleanup phase; convert stubs to canonical doc + ANCHOR
placement; add the import-discipline test.

Tasks:

- T7.1 [GREEN]: Update `pkg/types/types.go` — replace 2-line stub with
  package doc + (optionally) re-exports from sibling files if needed; add
  @MX:ANCHOR if any function ends up high-fan_in.
- T7.2 [GREEN]: Update `internal/adapters/adapters.go` — replace 4-line stub
  with package doc; reference SPEC-CORE-001.
- T7.3 [RED]: `pkg/types/types_test.go` — TestPkgTypesNoInternalImports
  (walks `go list -deps -json` for `pkg/types/...`; asserts no listed import
  starts with `internal/`, `prometheus/`, or `go.opentelemetry.io/`).
- T7.4 [GREEN]: Confirm test passes by inspection (pkg/types/* should only
  import context, time, errors, encoding/json, hash/* stdlib).

Deliverable check:
- Test #42 passes.

### Priority 8 — Benchmarks and Pre-Submission Self-Review

Tasks:

- T8.1 [GREEN]: `pkg/types/bench_test.go` — BenchmarkNormalizedDocValidate,
  BenchmarkNormalizedDocCanonicalHash. Verify NFR-CORE-001 thresholds.
- T8.2 [SELF-REVIEW]: Run full diff against acceptance criteria. Ask:
  - Is there a simpler approach? (e.g., can NormalizedDoc.Hash and
    CanonicalHash() be unified — i.e., is the Hash field redundant if
    CanonicalHash is callable on demand?)
  - Are there any unnecessary abstractions? (RegisterOptions has only one
    field — is it pulling its weight?)
- T8.3 [VERIFY]: Run quality gate per `quality.yaml`:
  - `go vet ./...`
  - `golangci-lint run`
  - `go test -race -cover ./pkg/types/... ./internal/adapters/...`
  - Confirm coverage ≥ 85%.

## 4. @MX Tag Plan

Per `.claude/rules/moai/workflow/mx-tag-protocol.md`, agents add @MX tags
during run phase. Targets identified during planning:

| Tag | Target | Rationale |
|-----|--------|-----------|
| @MX:ANCHOR | `pkg/types/Adapter` (interface) | fan_in ≥ 12 (every M3 adapter implements it) |
| @MX:ANCHOR | `internal/adapters.Registry` struct | fan_in ≥ 3 (cmd mains, FAN-001, IR-001, tests) |
| @MX:ANCHOR | `(*Registry).Register` | fan_in ≥ 13 (12 adapters + tests) |
| @MX:ANCHOR | `(*Registry).Get` | fan_in ≥ 3 (FAN-001, IR-001, tests) |
| @MX:ANCHOR | `pkg/types/NormalizedDoc` struct | fan_in ≥ 13 (every adapter constructs it) |
| @MX:ANCHOR | `pkg/types.CategorizeError` | fan_in ≥ 3 (FAN-001 retry policy, wrappedAdapter, tests) |
| @MX:WARN | `(*Registry).RegisterWithOptions` | "danger zone": silent overwrite would invalidate FAN-001's routing table — duplicate-name detection is a load-bearing invariant |
| @MX:NOTE | Each NormalizedDoc field with a non-obvious invariant: URL ("must be canonical"), Hash ("content-only; excludes Metadata"), Score ("0.0 means unscored, not zero engagement"), Lang ("BCP-47, empty = unknown") |
| @MX:NOTE | `pkg/types.OutcomeFromError` | Exposes the canonical mapping from error category to Prometheus label value; downstream consumers must not invent their own mapping |
| @MX:TODO | Any test marked t.Skip during RED phase (should be zero by GREEN exit) |

@MX:REASON sub-lines accompany every ANCHOR/WARN. Per
`.moai/config/sections/language.yaml` `code_comments: en`, all tag text is
in English.

## 5. Test Plan

### 5.1 Test Coverage Matrix

42 representative tests (full enumeration in spec.md §8). Distribution:

| Layer | Test count |
|-------|-----------|
| `pkg/types/normalized_doc_test.go` | 9 |
| `pkg/types/adapter_test.go` | 1 |
| `pkg/types/query_test.go` | 1 |
| `pkg/types/capabilities_test.go` | 2 |
| `pkg/types/errors_test.go` | 5 |
| `pkg/types/types_test.go` | 1 (import discipline) |
| `pkg/types/bench_test.go` | 2 (benchmarks; not coverage-counted) |
| `internal/adapters/registry_test.go` | 17 |
| `internal/adapters/noop/noop_test.go` | 2 |
| **Total non-bench** | **38** |
| **Total with bench** | **42** |

### 5.2 Coverage Gates

- Per-file coverage target: 85% (TRUST 5 Tested + project quality.yaml).
- Per-method-of-NormalizedDoc: 100% (Validate, CanonicalHash are
  load-bearing).
- Per-method-of-Registry: 95% (Register, RegisterWithOptions, Get, List).
- wrappedAdapter.Search: 100% (5 outcome paths × 1 nil-obs path = 6 branches).

### 5.3 Test Tooling

Per `.claude/rules/moai/languages/go.md`:
- Race detection: `go test -race ./...`
- Coverage: `go test -cover ./...`
- Bench: `go test -bench=. -benchmem ./pkg/types/`
- Lint: `golangci-lint run`

Stub adapters for registry tests: implement `pkg/types.Adapter` inline
in `registry_test.go` (don't import noop, to avoid coupling tests to the
noop's evolution). Pattern matches `internal/llm/router_test.go` test
fakes.

### 5.4 Concurrency Verification

- TestRegistryConcurrentReadWrite uses `t.Parallel()` and `goroutine` fan-out
  with a stopping channel. Pattern matches existing test structure in
  `internal/obs/metrics/metrics_test.go` (per-Registry isolation note at
  `metrics.go:46-50`).
- All registry tests construct a fresh Registry per test (no shared global
  state).

### 5.5 Test Isolation

- Each test that exercises observability constructs its own `obs.Obs` via
  `obs.Init(ctx, obs.Config{...})` — same pattern as
  `internal/llm/client_test.go`.
- The metric collectors in the freshly-constructed `obs.Metrics` are
  per-Registry per `internal/obs/metrics/metrics.go:46-50` comment, so
  tests are race-safe under `t.Parallel()`.
- OTel span capture uses `sdktrace/tracetest.NewInMemoryExporter` per
  pattern documented in SPEC-OBS-001 acceptance criteria.

## 6. Risk Mitigations Specific to Plan

| Risk | Mitigation in plan |
|------|--------------------|
| Run phase implements registry before NormalizedDoc, hits cyclic-import wall | Priority order locks dependencies: Errors → Query → Capabilities → NormalizedDoc → Adapter → Registry → Noop |
| Test fixtures grow into "second adapter" by accident, leaking into noop's scope | Fakes used in `registry_test.go` are inline (not exported, not in noop package); noop stays under 50 LoC |
| Implementer adds new metric collectors (e.g., `usearch_adapter_health_total`) violating "zero new metrics" plan promise | Plan §3 Priority 6 explicitly states no new metric registrations; reviewer checks `internal/obs/metrics/` is unchanged |
| outcome label drift (e.g., implementer adds "partial_failure") | Plan Priority 6 T6.4 enumerates the exact 5 values; NFR-CORE-002 test enforces |
| Validate() becomes a magnet for adapter-specific rules | Plan §3.3 fixes Validate's check-set at the four required fields; cross-adapter rules belong in FAN-001 / IR-001 |

## 7. Self-Review Checklist (Pre-Completion Gate)

Per `workflow-modes.md` Pre-submission Self-Review section:

- [ ] Is there a simpler approach to the registry — e.g., a function instead of
      a struct? (No: concurrent state requires a struct.)
- [ ] Is the Hash field redundant given CanonicalHash() method? (Possibly. The
      field is set on construction so JSON round-trips preserve it; method is
      called by registry/dedup. Decision: keep both; field is the cached
      result.)
- [ ] Is RegisterOptions struct earning its weight with one field? (For
      v0.1, yes — adding more fields without growing the call signature is
      the value. Future fields will land here without API breaks.)
- [ ] Does NormalizedDoc need exactly 15 fields, or is some field
      redundant? (Snippet vs Body could be folded — but Snippet has UI
      semantics; keeping both is correct.)
- [ ] Could we merge `pkg/types/{adapter.go, query.go, capabilities.go}`
      into one file? (No: they're conceptually distinct; separating
      simplifies test discovery and review.)
- [ ] Does the noop adapter need both `Search` and `Healthcheck` returning
      `nil`? (Yes: the compile-time interface check requires all four
      methods.)

## 8. Plan Approval Gate

Before transition to `/moai run SPEC-CORE-001`:

- spec.md is complete and approved (annotation cycle ≤ 6 iterations).
- This plan.md is reviewed against spec.md REQ-CORE-001..008 + NFR-CORE-001..003.
- acceptance.md Given/When/Then scenarios are complete.
- All Open Questions in spec.md §11 are either resolved or explicitly
  marked as run-phase decisions.
- Plan-auditor (per CLAUDE.md §4) has reviewed for bias / completeness.

## 9. Out-of-Plan (Deferred to Run Phase)

- Exact xxhash algorithm choice — see spec.md §11 Open Question 1.
- Whether to add a `Capabilities.Region []string` field for geographic
  filtering — anticipated need from M3 ADP-008 Naver SPEC, but not
  required for CORE-001.
- Validation of `Filter.Key` against an enum — adapter-specific filter
  keys are opaque to CORE-001; FAN-001 may add validation later.
- Pluggable hash function (allow callers to override) — premature
  generalization; lock to xxhash64 in V1.

---

*End of plan.md v0.1*
