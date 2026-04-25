# Plan — SPEC-IR-001 Intent Router v0

**Status**: implementation plan (Plan-phase deliverable)
**Methodology**: TDD (RED-GREEN-REFACTOR per `quality.development_mode: tdd`)
**SPEC**: SPEC-IR-001
**Coverage target**: 85%

---

## 1. Implementation Strategy

SPEC-IR-001's run phase follows TDD with strict RED-GREEN-REFACTOR
cycles, sequenced by file dependencies. Greenfield on
`internal/router/` (the existing 4-line stub at
`internal/router/router.go:1-4` has no behaviour to preserve;
brownfield characterization tests are not needed).

**Invariants the plan respects**:

- Touch only `internal/router/`, `internal/obs/metrics/router.go`,
  one minor edit to `internal/obs/metrics/metrics.go`, and one
  minor edit to `cmd/usearch/main.go`.
- No SPEC-OBS-001 cardinality allowlist amendment.
- No HTTP/gRPC endpoint, no chi v5 routes.
- No caching of classifications.
- Build on the existing `llm.Classify` ModelClass via `Request.Override`.

---

## 2. Dependency Order (TDD Sequence)

Files are implemented bottom-up so that each stage's RED tests have
all needed types in place.

### Stage 1: Foundation (no I/O, no orchestration)

| Order | File | RED Test Highlights | GREEN Description |
|-------|------|--------------------|-------------------|
| 1.1 | `errors.go` | `TestSentinelErrorsExist` | Declare `ErrInvalidQuery`, `ErrLLMTimeout`, `ErrAdapterRegistryEmpty` |
| 1.2 | `category.go` | `TestCategoryEnumComplete`, `TestCategoryEligibleDocTypes`, `TestCategoryEligibleDocTypesUnknownIsWebSocialUnion` | `Category` type, 6 constants, `categoryEligibleDocTypes(c) []DocType` mapping per spec.md REQ-IR-008. For `CategoryUnknown`, return the union of web-eligible (`{article, post, other}`) and social-eligible (`{post, social, video}`) DocTypes — Unknown is recoverable, not terminal. `ClassificationSource` enum (3 values) |
| 1.3 | `query_input.go` | `TestRouterQueryEmptyTextFails`, `TestRouterQueryAcceptsPopulated`, `TestRouterQueryWhitespaceOnly` | `RouterQuery` struct embedding/wrapping `pkg/types.Query`; `Validate() error` returns `ErrInvalidQuery` on empty/whitespace |
| 1.4 | `routing_decision.go` | `TestRoutingDecisionMarshal`, `TestRoutingDecisionMetadataKeys`, `TestRoutingDecisionEmptyAdapterSetSerializesAsNull` | `RoutingDecision` struct; `MarshalJSON` ensuring stable key order; documented Metadata key allowlist as a comment |

### Stage 2: Korean Detection

| Order | File | RED Test Highlights | GREEN Description |
|-------|------|--------------------|-------------------|
| 2.1 | `korean.go` | `TestHangulRatioPureKorean` (≈ 1.0), `TestHangulRatioPureEnglish` (= 0.0), `TestHangulRatioMixed` (verifiable fraction), `TestKoreanSignalsDetectsParticles`, `TestKoreanSignalsHandlesEmoji` | Implement `HangulRatio(s) float64` walking runes once and counting Hangul-block hits divided by total non-whitespace rune count. Implement `KoreanSignals(s) (ratio float64, hasParticle bool)`. Particle list is a package-private slice of 11 Korean postpositions |
| 2.2 | `korean_test.go` | (above) | + `TestHangulRatioOnlyHangulCompatibilityJamo` covers U+3130-318F edge case |

### Stage 3: Rule-Based Scoring

| Order | File | RED Test Highlights | GREEN Description |
|-------|------|--------------------|-------------------|
| 3.1 | `rules.go` | `TestRulesScoreAcademicHigh`, `TestRulesScoreSocialHigh`, `TestRulesScoreKoreanHigh`, `TestRulesScoreUnknownLowConfidence`, `TestRulesScoreReturnsTriggers`, `TestRulesScoreFormulaTraces` | `Rules` struct holds keyword tables + thresholds (τ_high=0.85, ratio_high=0.30, ratio_low=0.10, all package-level constants). `Score(q) (Category, confidence, triggers)` implements the **deterministic confidence-scoring formula specified in spec.md §2.3**: per-category raw scores from `(hangul_ratio, particle_density, kwd_density_C, has_english_token)` signals; `argmax` aggregation with the fixed tie-break order `academic > korean > social > mixed > web > unknown`. Compile regexes ONCE in `init()` for performance. Keyword tables: ~30 academic terms, ~25 social-platform names, ~15 Korean signal terms, ~20 web markers. `TestRulesScoreFormulaTraces` reproduces the three worked examples in spec.md §2.3 byte-for-byte and asserts each intermediate signal value within `±0.005`. |
| 3.2 | `rules_test.go` | (above) | + `TestRulesScoreOrderingDeterministic` (same input → same output, even at confidence ties; tie-break order matches spec.md §2.3) |

### Stage 4: LLM Fallback

| Order | File | RED Test Highlights | GREEN Description |
|-------|------|--------------------|-------------------|
| 4.1 | `llm.go` | `TestLLMFallbackBuildsCachedSystemPrompt`, `TestLLMFallbackParsesValidJSON`, `TestLLMFallbackRejectsInvalidEnum`, `TestLLMFallbackHandlesParseError`, `TestLLMFallbackHonorsTimeout`, `TestLLMFallbackHonorsCircuitBreaker` | Build prompt (system + user), call `llmClient.Complete(ctx, llm.Request{Class: llm.Classify, Override: model, MaxTokens: 100, Temperature: 0, System: classifySystemPrompt, Messages: ...})`. Strip code-fence (`json` or plain), `json.Unmarshal` into local struct, validate `category` is in 6-enum, clamp `confidence` to [0,1], truncate `rationale` to 200 chars. On `errors.Is(err, llm.ErrAllProvidersFailed)` return `(_, _, ErrLLMUnavailable)` (internal sentinel; surface to caller as Metadata flag, not error). On `context.DeadlineExceeded` return `ErrLLMTimeout` |
| 4.2 | `llm_test.go` | (above) + `TestLLMFallbackSystemPromptByteIdentity` (idempotent string for prompt caching) | + `TestLLMFallbackUsesOverrideModel` |

### Stage 5: Metrics

| Order | File | RED Test Highlights | GREEN Description |
|-------|------|--------------------|-------------------|
| 5.1 | `internal/obs/metrics/router.go` | (Tested via Stage 7 router_test) | Declare `RouterClassifications *prometheus.CounterVec{outcome}` and `RouterClassificationDuration *prometheus.HistogramVec{outcome}`. `registerRouter(pr) routerCollectors`. Mirror `registerLLM` pattern (`internal/obs/metrics/metrics.go:134`) |
| 5.2 | `internal/obs/metrics/metrics.go` | (Tested via Stage 7) | Add 2 fields to `Registry` struct; call `registerRouter(pr)` in `NewRegistry()`; pre-init each Vec with placeholder labels per existing pattern (line 126-131); NO `labelNames` change |
| 5.3 | `internal/router/metrics.go` | `TestOutcomeFromDecisionTable` | 10 outcome constants + `outcomeFromDecision(d, err) string` helper. NO prometheus imports |
| 5.4 | `internal/router/metrics_test.go` | (above) | Cover all 10 outcome paths |

### Stage 6: Router Orchestration

| Order | File | RED Test Highlights | GREEN Description |
|-------|------|--------------------|-------------------|
| 6.1 | `router.go` | `TestNewRouterReturnsErrEmptyRegistry`, `TestNewRouterCachesCapabilities`, `TestClassifyDispatchesRulesFirst`, `TestClassifyEscalatesAtLowConfidence`, `TestClassifyHonorsLangOverride`, `TestClassifySelectsAdapterSet`, `TestClassifyDegradesGracefully`, `TestClassifyEmitsObservability` | `Router` struct, `Options`, `New(opts)`, `Classify(ctx, q)` orchestrator. The orchestrator: (1) start span + start time, (2) validate query (REQ-IR-005), (3) run rules → score, triggers, (4) hangul ratio → set lang unless overridden, (5) gate on confidence → either keep rule-based result OR run LLM, (6) on LLM result, merge into decision; on LLM err, degrade per REQ-IR-003/REQ-IR-007, (7) selectAdapterSet via cached capabilities + sort, (8) emit observability + return |
| 6.2 | `router_test.go` | (above + golden-fixture-driven `TestClassifyGoldenFixtures`) | Drive 30+ golden queries through Classify + assert expected category and confidence range |

### Stage 7: Cmd Wiring

| Order | File | RED Test Highlights | GREEN Description |
|-------|------|--------------------|-------------------|
| 7.1 | `cmd/usearch/main.go` | (existing tests guard) | Conditional: when `LITELLM_MASTER_KEY` non-empty AND adapter registry has ≥1 adapter (currently only the noop), construct `router.New(...)`; store on cmd context for future SPECs. No CLI surface change |

### Stage 8: Benchmarks & Golden Fixtures

| Order | File | RED Test Highlights | GREEN Description |
|-------|------|--------------------|-------------------|
| 8.1 | `bench_test.go` | `BenchmarkClassifyRulePath100Chars` | Benchmark rule-based path; guard p50 < 1ms, ≤ 10 allocs/op |
| 8.2 | `testdata/queries_golden.json` | Loaded by `TestClassifyGoldenFixtures` | 30+ entries covering 6 categories with varied phrasing, language mix, and confidence expectations |

---

## 3. Risk Analysis

(Full risk table in `research.md` §6 and `spec.md` §7. This section
focuses on implementation-time risks the run-phase agent should
proactively address.)

### High-Severity Implementation Risks

| Risk | Run-Phase Action |
|------|-------------------|
| LLM stub behaviour drifts during testing → flaky `TestClassifyLLMTimeoutDegrades` | Use deterministic stub `llm.Client` with `time.Sleep` + `select { ctx.Done() }`; never use real network |
| Prompt cache effectiveness untested | Run-phase implementer logs `cache_creation_input_tokens` once at startup using a known-good probe call to verify ≥ 1024 tokens in system prompt; abort if below |
| Hangul ratio computation iterating runes-then-strings → 2x cost | Single-pass implementation: `for _, r := range s { ... }`; do not do `[]byte(s)` then `string(b)` |
| Goroutine leak in LLM-with-deadline implementation | Use `ctx, cancel := context.WithTimeout(...)`; `defer cancel()`; on timeout the LLM call's underlying HTTP request is cancelled via the propagated ctx |
| Test pollution of shared metrics registry | Per-test `metrics.NewRegistry()` instance (already the SPEC-OBS-001 pattern at `internal/obs/metrics/metrics.go:58-156`) — IR-001 inherits this isolation |
| `outcomeFromDecision` returns wrong label for edge cases (LLM ran AND failed AND rule succeeded) | TDD covers this: `TestOutcomeFromDecisionTable` enumerates 10 cases; the helper is total |
| Capability cache becomes stale if registry grows post-New | Documented: V1 has no hot-add; the cache is a snapshot. Future SPEC if needed |

### Medium-Severity Implementation Risks

| Risk | Run-Phase Action |
|------|-------------------|
| Keyword tables collide (a query matches both academic AND social) | `Score` returns the HIGHEST-confidence category per the formula in spec.md §2.3; ties broken by the fixed order `academic > korean > social > mixed > web > unknown` (codified in `rules.go` as a package-level constant slice) |
| Confidence math produces NaN | Runtime guard: `if math.IsNaN(c) { c = 0.5 }`; covered by `TestRulesScoreNoNaN` |
| `Metadata` map shared across requests | Always allocate a fresh map per Classify call |
| Adapter capability fields evolve in CORE-001 → IR-001 caches stale shape | `pkg/types.Capabilities` field additions are non-breaking per CORE-001 SPEC §6.7; IR-001 reads only `SupportedLangs` and `DocTypes` (stable fields) |

### Low-Severity Implementation Risks

| Risk | Run-Phase Action |
|------|-------------------|
| `regexp` package init cost | Compile-once at init; no per-call MustCompile |
| JSON marshal field-order drift | Use `encoding/json` Default ordering or explicit MarshalJSON if golden tests need byte-equality |
| Slog attr name typos | Use a constants block `attrRequestID = "request_id"` etc. |
| Test coverage drops below 85% target | TDD discipline: every REQ has at least 2 tests; running `go test -cover` per stage catches drift early |

---

## 4. Test Plan Summary

| Category | Count | Coverage Target |
|----------|-------|-----------------|
| Unit tests (per source file) | ~50 | 85%+ |
| Golden-fixture integration tests | 30+ | (counted in unit) |
| Benchmark tests | 2 (NFR-IR-001) | N/A |
| Race-clean concurrent tests | 1 (`TestClassifyConcurrent`) | N/A |

Total target: ~50 tests + 30+ fixtures + 2 benchmarks. Coverage
target 85% per `quality.test_coverage_target`.

### Coverage Distribution Target

| Sub-package | Target |
|-------------|--------|
| `internal/router/router.go` | ≥ 85% (orchestration) |
| `internal/router/rules.go` | ≥ 90% (deterministic, fully testable) |
| `internal/router/korean.go` | ≥ 95% (pure, easy to cover) |
| `internal/router/llm.go` | ≥ 80% (network-dependent paths via stubs) |
| `internal/router/category.go` | ≥ 95% (pure data) |
| `internal/router/routing_decision.go` | ≥ 90% |
| `internal/router/metrics.go` | ≥ 95% |
| `internal/router/errors.go` | ≥ 100% (sentinels are easy) |
| `internal/router/query_input.go` | ≥ 95% |
| `internal/obs/metrics/router.go` | ≥ 90% (delta to existing test pattern) |

---

## 5. Reference Implementations Cited

(Full citations in `research.md` §8.)

| Pattern | Source | Use |
|---------|--------|-----|
| Router struct with sync.RWMutex + priority map | `internal/llm/router.go:148-198` | Shape of `internal/router.Router` (without breakers) |
| Per-call observability emit (slog + counter + histogram + span) | `internal/llm/client.go:230-252` | Shape of `internal/router.Router.emit` |
| Nil-safe collector access | `internal/llm/client.go:244-251` | Same triple-guard for `obs.Obs`, `obs.Metrics`, individual collectors |
| Adapter registry lookup | `internal/adapters/registry.go:147-152, 157-166` | Reading `Capabilities` at New time |
| Prometheus collector registration via `registerXxx(pr)` | `internal/obs/metrics/metrics.go:134` (`registerLLM`) | Shape of `internal/obs/metrics/router.go::registerRouter` |
| LLM client `Override` model | `internal/llm/client.go:107-110` | Mechanism for `INTENT_ROUTER_LLM_MODEL` env injection |
| Forced-JSON output via tool_choice | Anthropic tool-use docs | DEFERRED to v0.2 (string-prompt JSON in v0.1) |
| Anthropic prompt caching | platform.claude.com docs | System prompt structured to exceed 1024 tokens |
| RouterChain destination/default | LangChain-Go | Always-include `unknown` fallback in LLM enum |

---

## 6. Definition of Done

The Plan phase is complete when:

- [ ] All 16 created files exist with content matching spec.md §5.1
- [ ] All 50+ tests pass (`go test ./internal/router/... -race`)
- [ ] Coverage ≥ 85% (`go test ./internal/router/... -coverprofile=cover.out`)
- [ ] All 8 EARS REQs satisfied with 1+ test each
- [ ] NFR-IR-001 benchmark passes (rule-based p50 < 1ms)
- [ ] NFR-IR-002 integration test passes (LLM-fallback p95 < 3s)
- [ ] No new direct dependencies in `go.mod`
- [ ] No SPEC-OBS-001 cardinality allowlist amendment
- [ ] All MX tags present per the plan in spec.md §5.6
- [ ] `cmd/usearch/main.go` wiring delta validated (binary still builds and runs `--version`)
- [ ] No regression in existing SPEC-CORE-001 / SPEC-LLM-001 / SPEC-OBS-001 tests

---

## 7. Out of Plan Scope (HARD)

These are NOT in this plan and MUST NOT be added during the run phase:

- HTTP / gRPC route on `cmd/usearch-api/`
- Caching layer
- Hot-reload of rules
- New label name in any Prometheus metric
- Modification to `pkg/types.{Adapter, Capabilities, Query}`
- Modification to `internal/llm/Request` shape (no JSONSchema field)
- Python service invocation
- Multi-language tokenization beyond Korean

If any of these emerge as needed during the run phase, the
implementer MUST stop and surface a SPEC-amendment proposal via
the orchestrator (per the Re-planning Gate
in `.claude/rules/moai/workflow/spec-workflow.md`).

---

## 8. Milestone Sequencing

This plan executes in priority order:

- **Priority High**: Stages 1-6 (MUST happen for IR-001 to be functional)
- **Priority Medium**: Stage 7 (cmd wiring; can be deferred to a follow-up commit if registry has only noop adapter at IR-001 land time)
- **Priority Low**: Stage 8 (benchmarks; required for NFR-IR-001 acceptance but can land in a follow-up CI run)

Per `.claude/rules/moai/core/agent-common-protocol.md` Time
Estimation rule: NO time predictions in this plan. Stages execute
in dependency order; "complete Stage 1, then start Stage 2".

---

*End of plan.md for SPEC-IR-001 v0.1*
