---
id: SPEC-EVAL-002
version: 0.2.0
status: draft
created: 2026-05-26
updated: 2026-05-30
author: limbowl (via manager-spec)
related_spec: SPEC-EVAL-002 (spec.md, plan.md)
format: Given/When/Then
---

# SPEC-EVAL-002 Acceptance Scenarios

## 0. Document Purpose

This document specifies acceptance criteria for SPEC-EVAL-002 in Given/When/Then format, expanding the scenario index in spec.md §5 (§5.1..§5.12) into externally-observable behaviors that the run phase MUST verify before declaring EVAL-002 ship-ready.

Scope: 12 acceptance criteria (AC-001..AC-012) covering REQ-EVAL2-001 through REQ-EVAL2-010 + NFR-EVAL2-001 through NFR-EVAL2-005, plus 3 edge-case sections, plus a Definition of Done checklist.

> **v0.2.0 amendment (2026-05-30):** (A1) AC-009 rewritten — health
> surface reuses SPEC-UI-002's `/api/admin/adapters` handler + adds
> `/api/admin/adapters/health` sibling (LoopbackOnly, no new port).
> (A2) AC-008 (circuit alert) marked **DEFERRED post-V1 — NOT a V1
> gate item**; AC-004 reduced to 4 V1 core panels (circuit-state matrix
> deferred). (A5) AC-003 Loki panel marked optional/non-gate. The DoD
> checklist and Coverage Matrix below reflect these deferrals.

Coverage policy: every REQ and every NFR in spec.md §2 / §3 has ≥1 matching AC below. See Coverage Matrix at end of file.

---

## 1. Acceptance Criteria (Given/When/Then)

### AC-001 — Recording-rule correctness against seeded fixtures

Covers: REQ-EVAL2-001, REQ-EVAL2-002

**Given** a Prometheus instance with `deploy/prometheus/recording-rules.yml` loaded and `usearch_adapter_calls_total` seeded with deterministic per-adapter outcome counts (e.g., reddit: 95 success + 5 failure in last hour).

**When** Prometheus evaluates the recording rules at the 1-minute tick.

**Then**:
- The file defines exactly 5 recording rules with the prescribed names: `usearch:adapter_success_rate_1h`, `usearch:adapter_success_rate_24h`, `usearch:adapter_success_rate_7d`, `usearch:adapter_failure_rate_by_outcome_24h`, `usearch:adapter_fanout_partial_ratio_24h`.
- `promtool check rules deploy/prometheus/recording-rules.yml` exits `0`.
- Querying `usearch:adapter_success_rate_1h{adapter="reddit"}` returns `0.95 ± 0.001` (hand-calculated).
- The success-rate denominator includes all 6 `outcome` values from `pkg/types/errors.go:OutcomeFromError` (success/failure/timeout/rate_limited/unavailable/transient).
- After 8 days of synthetic data, the 7d query for `reddit` excludes day-0 data (window rolls forward).
- The 7d series is queryable for every adapter in `Registry.List()` (at minimum the 12 SPEC-ADP-001..009 adapters).

Maps to scenario §5.1 in spec.md.

---

### AC-002 — fanout partial counter increments exactly once per failed adapter

Covers: REQ-EVAL2-003, REQ-EVAL2-004

**Given** `fanout.Dispatch` invoked with 5 mock adapters where 2 return errors.

**When** `eg.Wait()` returns and the Result is returned to the caller.

**Then**:
- `usearch_fanout_partial_total{adapter="failing_a"}` and `usearch_fanout_partial_total{adapter="failing_b"}` each increase by exactly 1.
- The other 3 adapters' counters are unchanged.
- The increment happens AFTER `eg.Wait()` and BEFORE the Result is returned (verified by counter sampling at both points).
- `usearch_adapter_health_status` and `usearch_adapter_circuit_state` are registered without panic.

Maps to scenarios §5.2, §5.10 in spec.md.

---

### AC-003 — failure_class slog attribute emitted (no Prometheus label)

Covers: REQ-EVAL2-005

**Given** an adapter that returns a TLS handshake error.

**When** `wrappedAdapter.emit` writes the slog record.

**Then**:
- The slog record contains `failure_class="tls"` attribute (emitted by `wrappedAdapter.emit`, `internal/adapters/registry.go:433`).
- The slog record contains `outcome="failure"` (existing label).
- No new Prometheus label is created (`failure_class` lives ONLY in slog/Loki).
- **(OPTIONAL, A5 — NOT a V1 gate item)** WHERE a Loki datasource is configured, the Grafana dashboard JSON MAY contain a Loki/log-link panel with query `{service="usearch"} | json | adapter = <selected> and outcome != "success"`. When Loki is absent, the panel renders empty and the 4 core panels are unaffected; its absence does NOT fail the gate.
- Other taxonomy values are emitted for: 5xx, 4xx, dns, parse, transcript, unknown — verified per error fixture.

Maps to scenario §5.3 in spec.md.

---

### AC-004 — Grafana dashboard renders 5 panels with auto-provisioning

Covers: REQ-EVAL2-006, REQ-EVAL2-007, NFR-EVAL2-002

**Given** `docker compose up grafana prometheus` with `deploy/grafana/dashboards/adapter-reliability.json` + `deploy/grafana/provisioning/dashboards/adapter-reliability.yaml` + `deploy/grafana/provisioning/datasources/prometheus.yaml`.

**When** the contributor polls `http://localhost:3000/api/dashboards/uid/adapter-reliability`.

**Then**:
- HTTP 200 received within 30 seconds of compose up.
- The dashboard contains the **V1: 4 core panels**: (1) per-adapter 24h success-rate heatmap, (2) per-adapter 7d success-rate trendline, (3) failure-cause stacked bar by outcome, (4) fanout partial-result ratio time-series.
- **(DEFERRED post-V1, A2 — NOT a V1 gate item)** Panel (5) circuit-state matrix is excluded from the V1 dashboard because no upstream emits `usearch_adapter_circuit_state`; it MAY be added (rendering "no data") when a future SPEC emits the gauge.
- Each core panel references the recording rules from REQ-EVAL2-001 (NOT raw `rate()` queries).
- A Grafana template variable `adapter` is populated from `label_values(usearch_adapter_calls_total, adapter)`.
- All **4 core panels** render in < 2 seconds against a Prometheus instance with 30 days of retention and 12 adapters × ~1 req/sec sustained traffic.
- Visual smoke screenshot shows the **4 core panels** with non-empty data when fed the seeded fixture.

Maps to scenario §5.4 in spec.md.

---

### AC-005 — 7d threshold alert fires after sustained 30 min below 0.85

Covers: REQ-EVAL2-008, REQ-EVAL2-009, NFR-EVAL2-003

**Given** Prometheus + Alertmanager + `deploy/prometheus/alerts.yml` + `deploy/alertmanager/alertmanager.yml`.

**When** synthetic time series are injected where `usearch:adapter_success_rate_7d{adapter="reddit"}` < 0.85 sustained for 30 minutes.

**Then**:
- The alert transitions `inactive → pending → firing`.
- Alertmanager receives the alert with labels `severity=warning|critical`, `spec=SPEC-EVAL-002`, `adapter=reddit`.
- The alert annotation includes a `runbook_url` linking to `docs/operations/adapter-reliability-runbook.md#<alert-name>`.
- Default Alertmanager route uses the `null` receiver (silent by default); 3 commented-out receiver examples (generic webhook, OpsGenie, PagerDuty) are present in the YAML.
- `amtool check-config deploy/alertmanager/alertmanager.yml` exits `0`.
- End-to-end alert latency (threshold cross → Alertmanager firing) ≤ 60 seconds under default config.

Maps to scenarios §5.5 in spec.md.

---

### AC-006 — Acute 1h alert fires within 5 min of sustained acute degradation

Covers: REQ-EVAL2-008, NFR-EVAL2-003

**Given** a simulated sudden adapter outage where `usearch:adapter_success_rate_1h{adapter="reddit"}` < 0.50 sustained for 5 minutes.

**When** Prometheus evaluates alerts.

**Then**:
- The acute alert fires within 5 minutes of the threshold cross.
- The alert has `severity=critical` label.
- The alert reaches Alertmanager and routes per the configured (or null) receiver.

Maps to scenario §5.6 in spec.md.

---

### AC-007 — Partial-result ratio alert fires when ratio > 0.30 sustained 15 min

Covers: REQ-EVAL2-008

**Given** simulated fanout dispatches where > 30% of recent dispatches had at least one failing adapter, sustained for 15 minutes.

**When** Prometheus evaluates the partial-ratio alert.

**Then**:
- The alert fires.
- The alert has labels `spec=SPEC-EVAL-002`, `severity=warning`.
- The alert annotation includes the runbook URL.

Maps to scenario §5.7 in spec.md.

---

### AC-008 — Circuit-open alert (DEFERRED post-V1, A2 — NOT a V1 gate item)

Covers: REQ-EVAL2-008 (deferred portion)

> **v0.2.0 amendment A2:** Because no upstream emits `usearch_adapter_
> circuit_state` in V1, the circuit-open alert (and the circuit-state
> dashboard panel) are deferred to post-V1 and **removed from the V1
> acceptance gate**. The metric family stays registered for forward
> compatibility. This AC is retained for documentation but MUST NOT
> block V1 ship-readiness; it is re-enabled when a future resilience
> SPEC emits real circuit transitions.

**Given** (post-V1, once an upstream emits it) the metric `usearch_adapter_circuit_state{adapter="reddit",state="open"} == 1` sustained for 10 minutes.

**When** Prometheus evaluates alerts.

**Then** (post-V1 only):
- The circuit-open alert fires.
- The alert has labels `spec=SPEC-EVAL-002`, `adapter=reddit`, `severity=warning|critical`.

**V1 behavior (asserted by EC-001):** the gauge is never emitted, the alert rule is NOT shipped in V1 `alerts.yml`, and the panel is NOT in the V1 dashboard — no spurious firing, no "no data" panel in the V1 gate.

Maps to scenario §5.8 in spec.md (deferred).

---

### AC-009 — Adapter health surface reusing SPEC-UI-002 admin handler

Covers: REQ-EVAL2-010

> **v0.2.0 amendment A1:** The health surface REUSES SPEC-UI-002's
> existing `/api/admin/adapters` handler (`internal/api/admin/handler_
> adapters.go`, mounted at `cmd/usearch-api/main.go:71` behind
> `LoopbackOnly`) and adds a sibling `/api/admin/adapters/health` on
> the SAME admin mux. The separate `:9090` server from v0.1.0 is
> removed.

**Given** the usearch admin mux (LoopbackOnly, same listener as `/api/admin/adapters`) and a mixed-status metrics fixture (some healthy, some degraded, one unhealthy).

**When** the operator issues:
```
curl http://127.0.0.1:<admin-port>/api/admin/adapters/health
```
(and also `curl http://127.0.0.1:<admin-port>/api/admin/adapters`)

**Then**:
- `GET /api/admin/adapters/health` returns HTTP `503` (because at least one adapter is unhealthy).
- The JSON body contains an `adapters` array.
- Each element includes: `name`, `status ∈ {healthy, degraded, unhealthy}`, `success_rate_24h ∈ [0.0, 1.0]`, `success_rate_7d ∈ [0.0, 1.0]`, `last_call_at` (ISO-8601). The `circuit_state` field, IF present, always reports `closed` in V1 (deferred per A2) and is NOT asserted as a gate item.
- Status mapping: ≥ 0.95 → healthy; 0.85–0.95 → degraded; < 0.85 → unhealthy.
- `GET /api/admin/adapters` now returns populated `success_count` / `fail_count` / `success_rate` (no longer 0 stubs) for adapters with telemetry.
- The endpoint is loopback-only (existing SPEC-UI-002 `LoopbackOnly` middleware): a request with a non-loopback `RemoteAddr` is rejected (403), and the endpoint is NOT on the public API listener.
- When all adapters are healthy, `GET /api/admin/adapters/health` returns HTTP `200`.
- Existing SPEC-UI-002 admin tests (secret non-leakage, status, resync, toggle) continue to pass unchanged.

Maps to scenario §5.9 in spec.md.

---

### AC-010 — Cardinality budget: total series ≤ 132 with 12 adapters

Covers: NFR-EVAL2-001, REQ-EVAL2-003

**Given** all three new metric families registered with 12 adapters.

**When** Prometheus scrapes the metrics endpoint.

**Then**:
- Total series across the 3 new families ≤ 132 (12 × 6 outcomes for calls_total + 12 partial + 12 health + 12 × 3 circuit_states).
- The `internal/obs/metrics/metrics_test.go` `Registry.labelNames` allowlist test passes with ONLY `state` added (no other new label names).
- Build fails if any unexpected label appears (e.g., `team_id` or `failure_class`).
- Under the production label set, total series stays ≤ 500 with up to 38 adapters.

Maps to scenario §5.10 in spec.md.

---

### AC-011 — Bench: zero-impact instrumentation (< 1% p99 overhead)

Covers: NFR-EVAL2-005

**Given** the existing `internal/fanout/bench_test.go` baseline.

**When** the contributor runs the bench before and after instrumentation lands.

**Then**:
- p99 latency delta is < 1%.
- p50 and p99 results are recorded in CI artefact for regression tracking.
- The bench is re-run in CI after this SPEC ships; regression > 1% blocks merge.

Maps to scenario §5.11 in spec.md.

---

### AC-012 — 30-day Prometheus retention preserves 7d window across restart

Covers: NFR-EVAL2-004

**Given** `deploy/prometheus/prometheus.yml` configured with `--storage.tsdb.retention.time=30d`.

**When** the contributor:
- Seeds 10 days of metric data.
- Restarts the Prometheus container.
- Queries `usearch:adapter_success_rate_7d` for any adapter.

**Then**:
- The 7d series has continuous data spanning the full window (no gap from the restart).
- The week-over-week comparison query (`offset 7d`) returns valid baseline values.
- Data older than 30 days drops off cleanly.

Maps to scenario §5.12 in spec.md.

---

## 2. Edge Cases

### EC-001 — Circuit-state metric absent (V1 default; alert/panel deferred per A2)

**Given** `usearch_adapter_circuit_state` is never emitted by any upstream package at run time (V1 default — no upstream emits it yet).

**When** V1 EVAL-002 ships.

**Then**:
- The metric family `usearch_adapter_circuit_state` is still registered (forward-compat, REQ-EVAL2-003c) and stays at default `closed`.
- The circuit-open alert is NOT present in the V1 `deploy/prometheus/alerts.yml` (deferred, A2) — so there is no "inactive-forever" or spurious-firing rule in V1.
- The circuit-state matrix panel is NOT present in the V1 dashboard (deferred, A2) — so there is no "no data" panel in the V1 gate.
- No log error or warning is emitted.
- (Post-V1) When a future SPEC adds both the emit source and re-enables the alert/panel, the panel renders live data and the alert fires per AC-008.

### EC-002 — Adapter added/removed mid-window

**Given** an adapter `newsource` is added to `Registry.List()` mid-day.

**When** the recording rule evaluates the next 1-minute tick.

**Then**:
- A new series `usearch:adapter_success_rate_24h{adapter="newsource"}` appears once observations exist.
- Removing an adapter (or deregistering it) causes its series to age out cleanly per Prometheus retention.
- The Grafana adapter template variable picks up the new adapter on next dashboard load (via `label_values` query).

### EC-003 — Failure-class taxonomy expansion

**Given** a new error class (e.g., `quota_exceeded`) emerges that does not fit the existing 7 categories.

**When** `wrappedAdapter.emit` encounters such an error.

**Then**:
- `failure_class="unknown"` is emitted (graceful catch-all).
- A follow-up PR may extend the taxonomy ENUM without breaking the cardinality budget (taxonomy lives in slog, not in Prometheus label).
- Existing alerts and panels continue to function unchanged.

---

## 3. Definition of Done Checklist

**V1 gate items:**

- [ ] All V1 AC scenarios pass on CI (AC-001..AC-007, AC-009..AC-012; AC-008 deferred per A2).
- [ ] V1 scenario index entries (§5.1..§5.7, §5.9..§5.12) in spec.md are implemented as automated tests. (§5.8 circuit alert deferred.)
- [ ] `deploy/prometheus/recording-rules.yml` + `deploy/prometheus/alerts.yml` (V1: 3 alert rules) pass `promtool check rules`.
- [ ] `deploy/prometheus/prometheus.yml` `evaluation_interval` set to `1m` (was 15s) per A4 / NFR-EVAL2-003.
- [ ] `deploy/alertmanager/alertmanager.yml` passes `amtool check-config`.
- [ ] `deploy/grafana/dashboards/adapter-reliability.json` (V1: 4 core panels) imports cleanly into Grafana 11.x.
- [ ] `deploy/grafana/provisioning/*.yaml` enables auto-provisioning via docker compose.
- [ ] `internal/obs/metrics` registers the 3 new metric families without panic (circuit_state registered but unused in V1).
- [ ] `internal/fanout` increments `usearch_fanout_partial_total` exactly once per failed adapter.
- [ ] `wrappedAdapter.emit` (registry.go:433) writes the `failure_class` slog attribute for the documented taxonomy.
- [ ] `GET /api/admin/adapters` returns populated `success_count`/`fail_count`/`success_rate` (SPEC-UI-002 handler reuse, A1).
- [ ] `GET /api/admin/adapters/health` on the same LoopbackOnly admin mux returns the documented schema + status mapping; no new :9090 server.
- [ ] Existing SPEC-UI-002 admin tests pass unchanged.
- [ ] `Registry.labelNames` allowlist test passes with `state` added; no other new labels.
- [ ] Bench delta < 1% p99 verified on `internal/fanout/bench_test.go`.
- [ ] Prometheus 30-day retention verified across container restart.
- [ ] Runbook `docs/operations/adapter-reliability-runbook.md` exists with sections matching each V1 alert name in `runbook_url` annotations.
- [ ] Open Questions in spec.md §8 are resolved or explicitly deferred with mitigation.

**Deferred (post-V1, NOT V1 gate items):**

- [ ] (post-V1) AC-008 circuit-open alert + circuit-state panel re-enabled when an upstream emits `usearch_adapter_circuit_state` (A2).
- [ ] (optional, non-gate) Loki log-link panel present when a Loki datasource is configured (A5).

---

## 4. Coverage Matrix (REQ → AC)

| REQ / NFR | AC-001 | AC-002 | AC-003 | AC-004 | AC-005 | AC-006 | AC-007 | AC-008 | AC-009 | AC-010 | AC-011 | AC-012 | EC |
|-----------|--------|--------|--------|--------|--------|--------|--------|--------|--------|--------|--------|--------|----|
| REQ-EVAL2-001 | ✓ |   |   |   |   |   |   |   |   |   |   |   |   |
| REQ-EVAL2-002 | ✓ |   |   |   |   |   |   |   |   |   |   |   |   |
| REQ-EVAL2-003 |   | ✓ |   |   |   |   |   |   |   | ✓ |   |   | EC-002 |
| REQ-EVAL2-004 |   | ✓ |   |   |   |   |   |   |   |   |   |   |   |
| REQ-EVAL2-005 |   |   | ✓ |   |   |   |   |   |   |   |   |   | EC-003 |
| REQ-EVAL2-006 |   |   |   | ✓ |   |   |   |   |   |   |   |   |   |
| REQ-EVAL2-007 |   |   |   | ✓ |   |   |   |   |   |   |   |   |   |
| REQ-EVAL2-008 (V1: 3 alerts) |   |   |   |   | ✓ | ✓ | ✓ | (✓ deferred, A2) |   |   |   |   | EC-001 |
| REQ-EVAL2-009 |   |   |   |   | ✓ |   |   |   |   |   |   |   |   |
| REQ-EVAL2-010 |   |   |   |   |   |   |   |   | ✓ |   |   |   |   |
| NFR-EVAL2-001 |   |   |   |   |   |   |   |   |   | ✓ |   |   |   |
| NFR-EVAL2-002 |   |   |   | ✓ |   |   |   |   |   |   |   |   |   |
| NFR-EVAL2-003 |   |   |   |   | ✓ | ✓ |   |   |   |   |   |   |   |
| NFR-EVAL2-004 |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |
| NFR-EVAL2-005 |   |   |   |   |   |   |   |   |   |   | ✓ |   |   |

Every REQ and NFR has ≥ 1 AC; edge cases EC-001..EC-003 supplement upstream-absent graceful degradation, adapter set drift, and taxonomy expansion. Per v0.2.0 amendment A2, the REQ-EVAL2-008 circuit-open portion (AC-008) is deferred to post-V1 and is NOT a V1 gate item; the other 3 alert conditions (AC-005, AC-006, AC-007) constitute the V1 alerting gate.

---

*End of SPEC-EVAL-002 acceptance.md (v0.2.0).*
