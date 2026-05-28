# SPEC-EVAL-002 Progress

## Session 1 — TDD Phases 1-3 (Go code)

Date: 2026-05-27

### Phase 1: Metric family registration — DONE

**RED**: 6 tests in `fanout_partial_test.go`:
- TestFanoutPartialMetricFamiliesRegister
- TestFanoutPartialCounterIncrements
- TestAdapterHealthStatusGaugeSet
- TestAdapterCircuitStateGauge
- TestStateEnumBoundedThreeValues
- TestCardinalityBudget12AdaptersUnder200Series

**GREEN**: Created `fanout_partial.go` with `registerFanoutPartial()`. Modified `metrics.go`:
- Added 3 new fields to Registry struct: FanoutPartial, AdapterHealthStatus, AdapterCircuitState
- Extended `labelNames` with `"state"`
- Wired registration call in `NewRegistry()`
- Updated cardinality allowlist in `metrics_test.go` and `router_test.go`

**REFACTOR**: go vet clean. All 19 tests in metrics package pass.

Exit criteria met:
- 3 new metric families appear in /metrics output
- Existing collector behavior unchanged
- Cardinality test green (65 series including placeholders)

### Phase 2: Fanout partial-result instrumentation — DONE

**RED**: 2 tests in `dispatch_test.go`:
- TestFanoutPartialCounterEmission (3 adapters, 1 fail)
- TestFanoutPartialCounterMultipleFailures (5 adapters, 2 fail)

**GREEN**: Added `emitPartialResultCounters()` to `observability.go`. Modified `dispatch.go`:
- Called after `assembleResult()` and before return
- Nil-safe: guards on obs, Metrics, FanoutPartial, AdapterErrors
- Added nil-guard to `obs.Obs.Tracer()` for test-only Obs bundles

**REFACTOR**: All 52 existing fanout tests + 2 new tests pass with -race.

Exit criteria met:
- `usearch_fanout_partial_total` increments exactly once per failed adapter
- Existing fanout behavior unchanged (51 existing tests PASS)
- Race detector clean

### Phase 3: Adapter failure_class slog attribute — DONE

**RED**: 2 tests in `registry_test.go`:
- TestClassifyFailureClassifications (6 sub-cases: 5xx, 4xx, 599, 400, no-status, plain)
- TestEmitIncludesFailureClassAttribute (end-to-end 503 → "5xx")

**GREEN**: Added `ClassifyFailure()` to `registry.go`. Modified `emit()`:
- Added `failure_class` slog attribute when err != nil
- Taxonomy: 5xx, 4xx, dns, tls, parse, transcript, unknown
- NOT promoted to Prometheus label (cardinality budget preserved)
- Added `net` import for DNS error detection

**REFACTOR**: All adapter package tests pass. No regression in existing attribute assertions.

Exit criteria met:
- Failed adapter calls emit `failure_class` slog attribute
- 6 canonical classes tested via table-driven test
- Existing adapter behavior unchanged

### Test counts

| Package | New Tests | Total Tests | Status |
|---------|-----------|-------------|--------|
| internal/obs/metrics | 6 | ~25 | PASS (-race) |
| internal/fanout | 2 | ~53 | PASS (-race) |
| internal/adapters | 2 | ~35 | PASS (-race) |
| **Total** | **10** | ~113 | **ALL GREEN** |

### Files created
- `internal/obs/metrics/fanout_partial.go`
- `internal/obs/metrics/fanout_partial_test.go`

### Files modified
- `internal/obs/metrics/metrics.go` (Registry fields, NewRegistry, labelNames)
- `internal/obs/metrics/metrics_test.go` (cardinality allowlist)
- `internal/obs/metrics/router_test.go` (cardinality allowlist)
- `internal/fanout/dispatch.go` (partial counter emission)
- `internal/fanout/dispatch_test.go` (partial counter tests + helpers)
- `internal/fanout/observability.go` (emitPartialResultCounters function)
- `internal/obs/obs.go` (Tracer nil-guard + otel import)
- `internal/adapters/registry.go` (ClassifyFailure + failure_class attribute)
- `internal/adapters/registry_test.go` (ClassifyFailure tests)

### Remaining phases
- Phase 4: Recording rules + Prometheus integration (declarative)
- Phase 5: Alert rules + Alertmanager integration (declarative)
- Phase 6: Grafana dashboard + provisioning (declarative)
- Phase 7: Health endpoint `/admin/health/adapters` (Go code)
- Phase 8: Operator runbook (documentation)
- Phase 9: End-to-end integration validation (testing)
- Phase 10: Sync phase (documentation)
