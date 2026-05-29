---
id: SPEC-EVAL-002
title: Adapter reliability dashboard — atomic task decomposition
phase: run / Phase 1 (analysis + planning)
author: manager-strategy
created: 2026-05-30
methodology: ddd
harness: standard
status: pending-plan-auditor
---

# SPEC-EVAL-002 — Atomic Task Decomposition

Derived from plan.md Phase 0–10 and acceptance.md AC-001..AC-012.
Ordering preserves plan.md dependencies. Each task is one DDD cycle.
No task starts before Phase 0 (plan-auditor PASS).

NOTE: plan.md enumerates 11 phases (0–10). To stay within the 10-task
cap, declarative artifact phases (4+5 recording/alert rules, 6 Grafana)
are grouped where they share a verification gate, and sync (Phase 10)
is folded into T10. Each task lists the spec phase(s) it covers.

| ID | Task | Plan Phase | Covers (REQ/AC) | Depends on | Done-when |
|----|------|-----------|-----------------|------------|-----------|
| T0 | Plan-auditor PASS: audit spec+plan+research+acceptance; resolve open Q1 (circuit-state source) + Q2 (Grafana min version) in annotation cycle; status draft→approved | 0 | all | — | plan-auditor returns PASS (max 3 iter); no open MAJOR |
| T1 | Register 3 new metric families in `internal/obs/metrics/fanout_partial.go` (NEW) + wire into `metrics.go` NewRegistry struct/return; add `"state"` to test allowlist. Characterization test that existing collectors unchanged. | 1 | REQ-EVAL2-003, NFR-EVAL2-001, AC-002(reg), AC-010 | T0 | `go test ./internal/obs/... -race` green; 3 families on `/metrics`; cardinality test passes with only `state` added |
| T2 | Instrument `internal/fanout/dispatch.go`: after `assembleResult`/`eg.Wait()`, increment `usearch_fanout_partial_total{adapter}` once per key in `Result.AdapterErrors`. Add emission test. Re-run `bench_test.go`. | 2 | REQ-EVAL2-004, AC-002, AC-011, NFR-EVAL2-005 | T1 | partial counter +1 per failed adapter, others unchanged; existing fanout tests green; p99 delta < 1% |
| T3 | Add `classifyFailure(err) string` + `failure_class` slog attr in `wrappedAdapter.emit` (registry.go:433). Table-driven test (5xx/4xx/dns/tls/parse/transcript/unknown). NO new Prometheus label. | 3 | REQ-EVAL2-005, AC-003, EC-003 | T1 | failure_class on WARN slog record; 7-class table test green; cardinality test still passes (no new label) |
| T4 | `deploy/prometheus/recording-rules.yml` (5 rules, research §3.3) + `recording-rules-test.yml` fixture; edit `prometheus.yml` (rule_files, alerting block, evaluation_interval→1m, retention 30d). CI gate `.github/workflows/promtool-validate.yml`. | 4 | REQ-EVAL2-001/002, AC-001, NFR-EVAL2-004 | T0 (parallel w/ T1-3) | `promtool check rules` + `promtool test rules` exit 0; 8-day fixture excludes day-0 |
| T5 | `deploy/prometheus/alerts.yml` (4 rules) + `deploy/alertmanager/alertmanager.yml` (null default + 3 commented receivers + inhibit). Extend CI gate with `amtool check-config`. Alert unit tests via `promtool test rules`. | 5 | REQ-EVAL2-008/009, AC-005/006/007/008 | T4 | promtool+amtool exit 0; 4 alert scenarios reach firing in unit test |
| T6 | `deploy/grafana/dashboards/adapter-reliability.json` (5 panels, recording-rule queries only, `adapter` template var) + 2 provisioning YAML. Edit `docker-compose.yml` (grafana+alertmanager services). | 6 | REQ-EVAL2-006/007, AC-004, NFR-EVAL2-002 | T4, T5 | dashboard lints; compose-up serves dashboard < 30s; 5 panels render < 2s; zero raw `rate()` in JSON |
| T7 | `/admin/health/adapters` endpoint. REUSE existing `internal/api/admin/` infra (LoopbackOnly, handler pattern) — see findings. Derive status from in-process AdapterCalls snapshot; 503 if any unhealthy. | 7 | REQ-EVAL2-010, AC-009 | T1 | endpoint returns documented schema; status-code mapping correct; loopback-only verified |
| T8 | `docs/operations/adapter-reliability-runbook.md` — 4 alert sections (anchors match runbook_url), threshold tuning, Prometheus sizing, `amtool silence` examples. | 8 | REQ-EVAL2-008 (annotations), DoD | T5 | 4 sections w/ matching anchors; tuning + sizing present |
| T9 | End-to-end integration validation: full compose stack, synthetic 50%-fail adapter, verify metric→recording-rule→alert-firing→dashboard→health-endpoint chain. Capture screenshots. | 9 | AC-004/005/009 (e2e), DoD | T6, T7, T8 | 4-stage chain verified; screenshots captured |
| T10 | Sync: update `roadmap.md` M8 row, `CHANGELOG.md` M8 entry, README Observability section; PR via manager-git. status approved→implemented. | 10 | — | T9 | PR opened with acceptance matrix; roadmap/changelog updated |

## Parallelism notes
- T4 (recording rules) is pure declarative and can run in parallel with the Go-code track (T1→T2→T3) once T0 passes.
- T5 depends on T4 (alerts query recording-rule series). T6 depends on T4+T5.
- Go track is strictly sequential: T1 (registry) → T2 (fanout uses FanoutPartial field) → T3 (independent of T2 but shares registry.go test file). T7 depends only on T1.

## DDD posture per task
- T1, T2, T3, T7 follow ANALYZE→PRESERVE(characterization)→IMPROVE. Existing emission behavior must not change.
- T4, T5, T6, T8 are declarative; verification is promtool/amtool/grafana-lint, not characterization tests.
