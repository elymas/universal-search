# Audit Report — SPEC-CORE-001 (iteration 1)

Reasoning context ignored per M1 Context Isolation. Audit conducted only against the SPEC files themselves and the codebase at HEAD.

## Verdict
PASS

## Summary
SPEC-CORE-001 is a high-quality foundational contract SPEC: 8 REQs cover all five EARS patterns (no gaps/duplicates), YAML frontmatter is complete, acceptance criteria are largely falsifiable, traceability between spec.md / plan.md / acceptance.md is tight, and the seven file:line citations spot-checked against `internal/llm/` and `internal/obs/` resolve correctly. Three non-blocking findings remain — most importantly an internal inconsistency where `acceptance.md` introduces a `types.ErrValidation` sentinel that `spec.md` REQ-CORE-007/REQ-CORE-008 never declare, plus two minor citation/sketch issues. None of these is blocking; all can be repaired in run-phase RED tests or with a single-line spec amendment.

## Blocking Defects (must fix before PASS)
None.

## Non-Blocking Findings (recommended improvements)

1. **Inconsistency between spec.md and acceptance.md regarding `ErrValidation` sentinel.**
   - `acceptance.md:134-135` (Scenario 3) asserts `errors.Is(err, types.ErrValidation)` returns true, calling it "a sentinel that the ValidationError matches via Is method".
   - `spec.md` never declares this sentinel. REQ-CORE-008 (`spec.md:146`) enumerates exactly four sentinels (`ErrTransient`, `ErrPermanent`, `ErrRateLimited`, `ErrSourceUnavailable`); REQ-CORE-007 (`spec.md:145`) declares only the typed `*ValidationError` struct.
   - Recommended fix: either (a) add `ErrValidation` as a fifth declared sentinel in REQ-CORE-008/§5/§6 with explicit godoc, or (b) drop the `errors.Is(err, ErrValidation)` line from acceptance.md Scenario 3 and rely solely on `errors.As(err, &ve)`.

2. **`TestNoDirectPrometheusImportOutsideObs` is referenced as if it exists in the codebase but only its principle is implemented.**
   - `spec.md:95` (Scope row `l`) and `research.md:678-679` cite "SPEC-OBS-001's `TestNoDirectPrometheusImportOutsideObs`". The test name appears in code comments (`internal/obs/metrics/llm.go:3`) and in SPEC-OBS-001 itself, but the actual function is not implemented in any `*_test.go` file under `internal/obs/`. Only `TestNoUnboundedLabels` and `TestCardinalityGuardRejectsUnboundedLabels` exist there. This is a SPEC-OBS-001 implementation gap, not a CORE-001 contract problem — but the citation in spec.md row l ("extends SPEC-OBS-001's `TestNoDirectPrometheusImportOutsideObs` allowlist principle") implies the test exists. CORE-001 itself promises a new `TestPkgTypesNoInternalImports`, which is sound; the citation is the only issue.
   - Recommended fix: in `spec.md:95`, replace "extends SPEC-OBS-001's `TestNoDirectPrometheusImportOutsideObs` allowlist principle" with "follows the SPEC-OBS-001 REQ-OBS-006 import-boundary principle" (avoiding a bare reference to a non-existent test symbol).

3. **OTel API in spec.md §6.5 sketch is outdated and inconsistent with the codebase.**
   - `spec.md:508` shows `tracer = oteltrace.NewNoopTracerProvider().Tracer("adapter")` for the nil-Obs branch.
   - The codebase actually uses `go.opentelemetry.io/otel/trace/noop` (`internal/obs/trace/trace.go:20,43` — `noop.NewTracerProvider()`). `oteltrace.NewNoopTracerProvider()` is not part of the v1.43.0 `go.opentelemetry.io/otel/trace` API; it was removed/relocated to the `trace/noop` subpackage.
   - This is a sketch (illustrative code), not a normative requirement, so it does not break the contract. But a run-phase implementer copying it verbatim will hit a compile error and need to discover the correct package.
   - Recommended fix: in `spec.md:508`, change `oteltrace.NewNoopTracerProvider().Tracer("adapter")` to `tracenoop.NewTracerProvider().Tracer("adapter")` (with appropriate import alias) OR simply note "use the OTel noop tracer per `internal/obs/trace/trace.go`".

## Positive Observations

1. **Excellent EARS coverage with all five patterns represented across 8 REQs**, REQ numbers sequential with no gaps/duplicates, every REQ explicitly tagged with its pattern in the table at `spec.md:139-146`.
2. **Tight traceability across four documents**: REQ-CORE-001..008 referenced consistently in spec.md §3/§5/§8, plan.md §3 priority decomposition, acceptance.md §1 scenarios, and spec-compact.md §EARS Requirements. No orphan REQs, no orphan ACs.
3. **Reuse over reinvention is well-justified and verified**: research.md correctly identifies that `AdapterCalls`/`AdapterCallDuration` are already registered in `internal/obs/metrics/metrics.go:86-101` (verified) and that `adapter`/`outcome` are already in the cardinality allowlist (verified at metrics.go:150). The "zero new metric families" claim is accurate.
4. **Concurrency-critical type has a race test in acceptance.md Scenario 6** (100 readers + 1 writer × 1 second under `-race`), addressing audit item 13 directly.
5. **Out-of-scope is enumerated specifically per downstream SPEC** (ADP-*, FAN-001, IDX-001..003, SYN-001/002/004, CACHE-001, AUTH-001..003, EVAL-002), satisfying audit item 5.
6. **Defensive-scaffolding guardrails are explicit**: plan.md §7 self-review checklist questions the necessity of `RegisterOptions` (single field), the redundancy of `Hash` field vs `CanonicalHash()` method, and the fold-ability of three pkg/types files. This is exactly the Opus 4.7 over-engineering posture per `moai-constitution.md` Agent Core Behavior 4.
7. **Plug-in steps for downstream adapter SPECs are documented**: spec.md §1 lines 33-77 plus research.md §1.2 show ADP-* plug-in flow (implement `Adapter`, register, return `[]NormalizedDoc`, classify errors via `CategorizeError`). Future adapter SPECs will not face ambiguity about how to integrate.
8. **MX tag plan is concrete in plan.md §4** with @MX:ANCHOR targets identified for the high-fan_in functions (`Adapter` interface, `Registry`, `Register`, `Get`, `NormalizedDoc`, `CategorizeError`) and @MX:WARN on the duplicate-name detection branch — all consistent with the protocol's fan_in ≥ 3 and "danger zone" criteria.
9. **TDD plan is sufficient to hit 85% coverage**: 38 functional tests + 4 benchmarks across 7 test files; per-method coverage targets in plan.md §5.2 (Validate/CanonicalHash 100%, Registry methods 95%, wrappedAdapter.Search 100% across 6 branches).
10. **Sentinel + typed-error dual pattern is sound**: `*SourceError.Is(target)` matches against sentinels via Category enum, `Unwrap()` returns inner cause, `OutcomeFromError` maps to bounded label set. The taxonomy is orthogonal — each input maps to exactly one outcome (with `transient` reserved internally as a sixth catch-all explicitly documented in spec.md NFR-CORE-002 and §11 Open Question 5).

## Coverage by Audit Item

| # | Audit Item | Result | Notes |
|---|-----------|--------|-------|
| 1 | EARS compliance (≥5 REQs covering all 5 patterns) | PASS | 8 REQs in spec.md:139-146; patterns: 3× Ubiquitous (001/002/008), 2× Event-Driven (003/004), 1× State-Driven (005), 1× Optional (006), 1× Unwanted (007). All testable, atomic, file-referenced. |
| 2 | Acceptance criteria sufficiency (≥2 G/W/T per REQ, falsifiable) | PASS | acceptance.md §1 has 12 G/W/T scenarios; REQ-CORE-001/004/008 have multiple sub-scenarios (Scenario 5 has 5 sub-cases; Scenario 7 has 4-cell truth table). All scenarios have named verification tests. |
| 3 | YAML frontmatter completeness | PASS | spec.md:1-16 has id, title, milestone, status, priority, owner, methodology, coverage_target, created, updated, author, issue_number, depends_on, blocks (14 fields, all 8 standard fields present plus extensions). |
| 4 | Files-to-modify accuracy | PASS | All paths under `pkg/types/` and `internal/adapters/` are realistic; `pkg/types/types.go` and `internal/adapters/adapters.go` exist as stubs (verified). 12 created files + 2 modified files in spec.md §6.7 — proportionate to scope, not over-promising. |
| 5 | Out-of-scope completeness (ADP/FAN/IDX/SYN/AUTH) | PASS | spec.md §2.2 and §7 enumerate all 5 categories plus CACHE, EMBED, KOTOK, EVAL, gRPC, plugin loading, query validation, fusion. Each entry names the destination SPEC. |
| 6 | Methodology consistency (TDD R-G-R, no impl in spec.md) | PASS | plan.md §3 explicitly orders RED → GREEN → REFACTOR per priority; spec.md sketches in §6.2-§6.6 are illustrative type definitions and pseudocode comments, not full implementations (no business logic in spec). |
| 7 | MX tag plan presence | PASS | plan.md §4 enumerates 6 @MX:ANCHOR targets, 1 @MX:WARN, 4+ @MX:NOTE locations, and TODO lifecycle. spec.md §6.2/6.4 includes @MX:ANCHOR + @MX:REASON inline in sketches. |
| 8 | Defensive-scaffolding traps | PASS | plan.md §7 checklist explicitly questions `RegisterOptions` single-field abstraction, `Hash` vs `CanonicalHash()` redundancy, three-file split, dual `Search`/`Healthcheck` pattern. No premature generic types; `Adapter[T any]` was rejected with rationale in research.md §7.2. |
| 9 | Observability claims (zero new metric families) | PASS | Verified by reading `internal/obs/metrics/metrics.go:86-101` (AdapterCalls/AdapterCallDuration registered) and lines 147-154 (allowlist contains `"adapter", "outcome"` at line 150 — slight off-by-one from spec's claim of line 151, see Citation Spot-Check). spec.md §6.7 "Unchanged (by design): internal/obs/metrics/metrics.go" is correct. |
| 10 | Citation accuracy (5+ samples) | PASS (with 1 minor mismatch and 1 broken reference) | 6 of 7 samples verified exact; 1 minor off-by-one (allowlist line 150 not 151); `TestNoDirectPrometheusImportOutsideObs` not present in codebase (Finding 2). |
| 11 | Internal consistency (REQ IDs across 4 docs) | PASS (with 1 inconsistency) | REQ-CORE-001..008 referenced consistently in spec.md/plan.md/acceptance.md/spec-compact.md. One inconsistency: acceptance.md introduces `ErrValidation` not declared in spec.md (Finding 1). |
| 12 | Error taxonomy soundness (orthogonality) | PASS | Four sentinels + Category enum: `ErrTransient`→`CategoryTransient`, `ErrPermanent`→`CategoryPermanent`, `ErrRateLimited`→`CategoryRateLimited`, `ErrSourceUnavailable`→`CategoryUnavailable`. `CategoryUnknown` is the explicit catch-all. Each error maps to one Category and one outcome label. `context.DeadlineExceeded` is distinct: `CategoryTransient` for classification, `"timeout"` for outcome label — documented as explicit in REQ-CORE-008 and Scenarios 8/9. |
| 13 | Registry concurrency claim (race test) | PASS | acceptance.md Scenario 6 explicitly specifies `TestRegistryConcurrentReadWrite` with 100 readers + 1 writer × 1 second under `-race`. plan.md §5.4 confirms the test pattern. |
| 14 | Coverage realism (85% achievable) | PASS | plan.md §5.1 lists 38 non-bench tests across 7 test files. Per-file estimated LoC in spec.md §6.7 is small (~50 LoC noop, ~120 LoC NormalizedDoc, ~120-200 LoC registry); 38 tests covering 8 REQs across this surface is sufficient for 85% per-package coverage. |
| 15 | Downstream-impact analysis (plug-in steps) | PASS | spec.md §1 lines 60-77 explain how M3 ADP-* SPECs consume the contract. research.md §1.2 enumerates 12 downstream SPECs. spec.md §9.3 and Open Question 7 document cursor/pagination plug-in pattern. ADP-* SPECs will have unambiguous integration steps. |

## Citation Spot-Check

Sampled 7 of ~37 file:line references in research.md and spec.md (extra sampling beyond the required 5 because the SPEC's foundational nature warrants stricter verification):

- `internal/llm/router.go:148-198` Router struct + RWMutex + Route — verified (exact match: line 148 begins `// Router selects providers...`, line 198 returns `available`).
- `internal/llm/client.go:230-252` `emitObservability` shape (1 slog + 1 counter + 1 histogram + cost emit) — verified (exact match starting line 230 with `// emitObservability emits slog, counter, and histogram for one LLM call.`).
- `internal/llm/retry.go:14-50` `nonRetryableStatusCodes` + `httpStatusError` + `isNonRetryable` — verified (exact match; the typed error pattern is at lines 32-50; status-code map at lines 14-20).
- `internal/llm/llm.go:108-121` four sentinel errors — verified (exact match: `ErrBudgetExceeded`, `ErrStreamBackpressureTimeout`, `ErrAllProvidersFailed`, `ErrModelNotConfigured`).
- `internal/obs/metrics/metrics.go:86-101` `AdapterCalls` + `AdapterCallDuration` registration — verified (exact match including `[]string{"adapter", "outcome"}` and `[]string{"adapter"}`).
- `internal/obs/metrics/metrics.go:151` `outcome` in allowlist — minor mismatch: the labelNames slice spans lines 147-154; `"adapter", "outcome"` is on line 150, not 151. Off-by-one but the principle (`outcome` in allowlist) is correct. Non-blocking.
- `internal/obs/obs.go:51-66` Obs bundle DI struct — verified (the struct begins at line 51 with `type Obs struct {`; the `Tracer` method ends at line 66).
- (Implicit) `TestNoDirectPrometheusImportOutsideObs` referenced in spec.md:95 and research.md:679 — not implemented in codebase: only `TestNoUnboundedLabels` exists at `metrics_test.go:277` (Finding 2 above). Non-blocking because CORE-001 ships its own `TestPkgTypesNoInternalImports`.

Overall citation accuracy: 6/7 exact + 1 off-by-one + 1 dangling test reference. Acceptable; no fabricated citations detected; the codebase claims that drove the SPEC's "zero new metric families" decision are real.

## Chain-of-Verification Pass

Re-read sections that I had skimmed quickly on the first pass to look for missed defects:

- **REQ number sequencing end-to-end**: REQ-CORE-001 through REQ-CORE-008, each appearing in spec.md §3, §5, §8, plus plan.md §3, plus acceptance.md §1 mapping, plus spec-compact.md. No skips, no duplicates, no inconsistent numbering.
- **Every REQ has at least one AC**: REQ-CORE-001→§5 + Scenario 1 + Scenario 3; REQ-CORE-002→§5 + Scenario 4; REQ-CORE-003→§5 + Scenario 2; REQ-CORE-004→§5 + Scenario 5 + Scenario 11; REQ-CORE-005→§5 + Scenario 6; REQ-CORE-006→§5 + Scenario 7; REQ-CORE-007→§5 + Scenario 3; REQ-CORE-008→§5 + Scenario 8 + Scenario 9. Full traceability.
- **Every AC traces to a valid REQ**: All 12 G/W/T scenarios have explicit "Maps to REQ-CORE-XXX" headers (acceptance.md:22, 67, 117, 150, 180, 232, 256, 292, 326, 354, 378, 405). All maps are valid.
- **Exclusions specificity**: spec.md §7 lists 14 specific items, each naming the destination SPEC and milestone. Not a single vague entry like "future enhancements".
- **Contradiction check**: Looked for conflicts between (i) REQ-CORE-007 (Validate rejects missing fields) and REQ-CORE-004 (wrappedAdapter still emits metrics on invalid docs) — explicitly reconciled in REQ-CORE-007 final clause: "the wrappedAdapter SHALL still increment counters and log the call, but FAN-001 / downstream consumers MAY filter invalid docs". Not a contradiction; a layered policy. Looked for conflict between NFR-CORE-002's "5-value enum" and REQ-CORE-004's also-5-value list — consistent. Looked for conflict between scope §2.1.k five outcomes and research.md §5 six values (`transient` listed as 6th) — reconciled in NFR-CORE-002 by explicit "internal `transient` value used only when classification cannot be more precise".
- **Edge cases not contradicting normative requirements**: acceptance.md §2.1 (nil ctx → wrapper substitutes Background and pre-rejects) is not contradicted by REQ-CORE-004 because REQ-CORE-004's `Search` contract starts after a non-nil ctx is established. Worth a defensive note in spec.md §6.5 sketch but not blocking.

No additional blocking defects discovered in the second pass. The three findings noted above are the complete set.

## Recommendation

PASS with three non-blocking findings.

Brief rationale citing evidence for each must-pass criterion:
- **MP-1 REQ number consistency**: `spec.md:139-146` lists REQ-CORE-001 through REQ-CORE-008 sequentially with consistent zero-padding; no duplicates, no gaps. Verified against plan.md §3 priority decomposition and acceptance.md §1 mapping headers.
- **MP-2 EARS format compliance**: All 8 REQs in `spec.md:139-146` use one of the five EARS patterns explicitly tagged in the Pattern column. REQ-CORE-003 ("WHEN ... SHALL") and REQ-CORE-004 ("WHEN ... SHALL") are Event-Driven; REQ-CORE-005 ("WHILE ... SHALL") is State-Driven; REQ-CORE-006 ("WHERE ... SHALL") is Optional; REQ-CORE-007 ("IF ... THEN ... SHALL") is Unwanted; REQ-CORE-001/002/008 are Ubiquitous ("The package SHALL").
- **MP-3 YAML frontmatter validity**: `spec.md:1-16` declares id (string `SPEC-CORE-001`), version-equivalent (status: draft), status (string), created_at (`created: 2026-04-26` ISO date), priority (`P0`), labels (none required since `methodology`, `owner`, `coverage_target`, `depends_on`, `blocks` carry the labeling intent). All 8 standard fields present with correct types.
- **MP-4 Section 22 language neutrality**: N/A — SPEC-CORE-001 is single-language scoped (Go only). The `pkg/types/` import-discipline NFR-CORE-003 is Go-specific by design; no multi-language tooling claim is made. Auto-passes per audit rubric.

Recommended next steps for manager-spec (non-blocking):
1. Reconcile `ErrValidation` between spec.md REQ-CORE-007/008 and acceptance.md Scenario 3 (Finding 1). Either add the sentinel to REQ-CORE-008 with godoc, or remove the `errors.Is` line from acceptance.md.
2. Update spec.md:95 citation to remove the bare reference to non-existent `TestNoDirectPrometheusImportOutsideObs` (Finding 2). Replace with a principle reference.
3. Update spec.md:508 sketch to use `go.opentelemetry.io/otel/trace/noop` (Finding 3), aligning with `internal/obs/trace/trace.go:20,43`.

If these three are addressed, this SPEC is ready for run-phase delegation to manager-tdd. Even if they are not addressed, the SPEC contract is sound — none of the findings invalidates a REQ or makes a normative requirement unimplementable.
