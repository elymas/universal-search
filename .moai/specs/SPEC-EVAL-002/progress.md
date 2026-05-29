# SPEC-EVAL-002 Progress Log

## 2026-05-30 — Phase 1 (Analysis + Planning) by manager-strategy

- Read spec.md (10 REQ + 5 NFR), plan.md (11 phases), acceptance.md (12 AC + 3 EC), research.md.
- Harness confirmed: **standard** (P1 feature, no security/payment keywords, multi-file/multi-domain). plan-auditor required (standard enables plan_audit, max 3 iter, require_must_pass; evaluator final-pass).
- Methodology: DDD (spec/plan override of repo tdd default — justified: instrumentation on existing emission contracts, minimal new domain logic).

### Stack/asset reality check vs LIVE code (grep-verified)
- `usearch_adapter_calls_total` + `AdapterCalls` CounterVec: EXISTS, metrics.go:147/40. SPEC consumes, does not reinvent. CONFIRMED.
- `usearch_adapter_call_duration_seconds` + `AdapterCallDuration`: EXISTS, metrics.go:155/41. CONFIRMED.
- `OutcomeFromError` 6-tuple: EXISTS, pkg/types/errors.go:174 (SPEC cited :174 — exact). Tuple matches D1 exactly. CONFIRMED.
- `Result.AdapterErrors` (nil when ErrorCount==0, built post eg.Wait by supervisor): EXISTS, dispatch.go:75-76/161. CONFIRMED.
- `wrappedAdapter.emit` slog point: EXISTS but at registry.go:**433** (SPEC/research cited :223 — STALE line, capability intact). emit already writes adapter/outcome/elapsed/error attrs; failure_class is a clean addition.
- 3 new metric families (fanout_partial, adapter_health_status, adapter_circuit_state): DO NOT exist in obs/metrics. Genuine addition, NOT reinvention. (Unrelated `CircuitState` exists in deepagent/costguard — different domain, not adapter-level.)
- Cardinality allowlist (metrics_test.go:251): `adapter`,`outcome`,`reason` present; `state` absent (correctly needs add); `failure_class` absent (correctly stays out). CONFIRMED.
- `deploy/` has prometheus.yml only — NO grafana/, alertmanager/, recording-rules. All net-new as SPEC states. prometheus.yml evaluation_interval is 15s (SPEC rules assume 1m → must edit).

### Major finding affecting plan
- Existing `internal/api/admin/` (SPEC-UI-002 Phase A) already serves `GET /api/admin/adapters` via `SnapshotForAdmin()` → `[]AdapterAdminView` (id/status/last_sync/success_count/fail_count/...), with `LoopbackOnly` middleware. SuccessCount/FailCount are stubbed (zero). REQ-EVAL2-010's `/admin/health/adapters` overlaps this; EVAL-002 SPEC was unaware. T7 should REUSE this infra (handler + loopback pattern), not build a parallel admin server. Path/port differs (SPEC says :9090 admin port; existing is /api/admin on main mux).

### Outputs written
- tasks.md (11 tasks T0-T10), progress.md (this file).

### Status
- draft → awaiting plan-auditor (T0). Not ready-to-implement until plan-auditor PASS.

---

## 2026-05-30 — Run phase (DDD) by manager-ddd

Branch: feature/SPEC-EVAL-002. Methodology DDD (ANALYZE → PRESERVE
characterization → IMPROVE). Tasks T1-T9 implemented; T10 (sync/PR) left to
orchestrator.

### PRESERVE baseline (before instrumentation)
- `go test ./internal/obs/metrics/... ./internal/fanout/... ./internal/adapters/... ./internal/api/admin/...` → all `ok` (green). Captured as the regression baseline.

### Implemented
- **T1** `internal/obs/metrics/adapter_reliability.go` (NEW) — registers
  `usearch_fanout_partial_total` (CounterVec, adapter), `usearch_adapter_health_status`
  (GaugeVec, adapter), `usearch_adapter_circuit_state` (GaugeVec, adapter+state;
  register-only, pre-init closed/open/half_open, no V1 emit). Wired into
  metrics.go Registry struct + NewRegistry + labelNames (`state` only). Added
  `state` to BOTH test allowlists (metrics_test.go map + router_test.go switch).
  Tests: register, existing-collectors-unchanged (characterization),
  state-enum-bounded, cardinality-budget-132. obs/metrics coverage 95.8%.
- **T2** `internal/fanout/dispatch.go` + `observability.go` — `emitPartialResult`
  increments fanout_partial_total once per key in Result.AdapterErrors after
  eg.Wait(), before return. Nil-safe. Tests: emission (1 fail of 3) + full-success
  no-increment. Existing fanout tests unchanged. Bench ran clean (counter Inc is
  O(1), no-op on full success). fanout coverage 98.2%.
- **T3** `internal/adapters/registry.go` — `classifyFailure(err)` 7-class taxonomy
  (5xx/4xx/dns/tls/parse/transcript/unknown) + `failure_class` slog ATTR in
  wrappedAdapter.emit (NOT a Prometheus label). White-box table test + black-box
  slog-attr test + success-has-no-failure-class test. classifyFailure 100%.
- **T7** REUSE SPEC-UI-002 admin mux: `AdapterAdminView.SuccessRate` field added
  (additive); SnapshotForAdmin fills success/fail/rate from in-process
  `usearch_adapter_calls_total` via new `internal/adapters/telemetry.go`
  (Gather snapshot, D1 6-tuple denominator). New `HealthSnapshot()` +
  `internal/api/admin/handler_adapters_health.go` (`/api/admin/adapters/health`,
  503-if-any-unhealthy) wired on the SAME mux behind LoopbackOnly in main.go.
  NO new server/port. circuit_state always "closed" (A2). admin tests green
  (UI-002 secret-non-leak + 9-adapter list unchanged).
- **T4** `deploy/prometheus/recording-rules.yml` (5 rules, research §3.3 PromQL)
  + `recording-rules-test.yml` (hand-calc fixtures reddit=0.95, naver=0.60,
  partial=0.05). `prometheus.yml`: evaluation_interval 15s→1m, rule_files,
  alerting block. retention 30d set in docker-compose command flag.
- **T5** `deploy/prometheus/alerts.yml` (V1: 3 rules — 7dLow/1hCritical/PartialHigh;
  circuit-open DEFERRED A2) + `alerts-test.yml` (3 firing scenarios) +
  `deploy/alertmanager/alertmanager.yml` (null default + 3 commented receivers
  + inhibit). CI gate `.github/workflows/promtool-validate.yml`.
- **T6** `deploy/grafana/dashboards/adapter-reliability.json` (V1: 4 panels —
  heatmap/7d-trendline/failure-stacked-bar/partial-ratio; circuit panel DEFERRED;
  recording-rule queries ONLY, zero raw rate(); `adapter` template var) +
  2 provisioning YAML. docker-compose: grafana (11.3.0, localhost:3000) +
  alertmanager (0.27.0, `alerts` profile) services + grafana_data volume.
- **T8** `docs/operations/adapter-reliability-runbook.md` — 3 V1 alert sections
  (anchors match runbook_url exactly) + threshold tuning + sizing + amtool silence.

### Verification (after instrumentation)
- `go build ./...` OK. `go vet` (touched pkgs) clean. golangci-lint 0 issues.
- `go test -race -cover` all touched pkgs green. New EVAL-002 functions:
  registerAdapterReliability/classifyFailure/classifyHealth/HealthSnapshot/
  health ServeHTTP/successRate/callStats = 100%; callStatsByAdapter 91.7%;
  emitPartialResult 100%. All ≥ 85% new-code target.
- Cardinality: 3 allowlist locations updated with `state` ONLY; budget test
  asserts 132 series for 12 adapters. All 3 cardinality guard tests green.
- promtool/amtool absent locally → CI-only gate (promtool-validate.yml). All
  declarative YAML python-yaml validated; docker compose config OK.

### Acceptance status (V1 gate)
- AC-001(rules), AC-002(partial+register), AC-003(failure_class), AC-009(health+
  populated counts), AC-010(cardinality) verified by Go/fixture tests.
- AC-004(dashboard), AC-005/006/007(alerts firing), AC-012(retention) covered by
  declarative artifacts + promtool fixtures, executable only in CI/compose.
- AC-008(circuit alert) DEFERRED post-V1 (A2) — not a V1 gate item; EC-001
  satisfied (no circuit alert/panel shipped, family registered).
- AC-011(bench <1%): instrumentation is O(1) per-error Inc, no-op on success;
  bench ran clean. Exact delta to be confirmed in CI bench compare.
