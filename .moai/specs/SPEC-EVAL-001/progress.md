# SPEC-EVAL-001 Progress

## Session 1 — 2026-05-27

### Phase 1: Golden Set + Corpus — COMPLETE

- Created `internal/eval/golden/queries.jsonl` (50 records: 35 EN + 15 KO)
- Created `internal/eval/golden/corpus/doc-*.json` (210 NormalizedDoc fixtures)
- Created `internal/eval/golden/manifest.json` (corpus_revision: 1.0.0)
- Created `internal/eval/golden/overrides.json` (empty initial)
- Created `internal/eval/golden/golden_test.go` (10 tests)
- All 10 golden tests PASS

### Phase 2: Python Judge Service — COMPLETE

- Created `services/researcher/src/researcher/eval_judge.py` (POST /judge/faithfulness)
- Created `services/researcher/tests/test_eval_judge.py` (11 tests)
- Modified `services/researcher/src/researcher/app.py` (mounted eval_judge router)
- Modified `services/researcher/pyproject.toml` (added deepeval ~= 1.0)
- All 11 Python judge tests PASS

### Phase 3: Go DeepEval Bridge — COMPLETE

- Created `internal/eval/scorer/deepeval_bridge.go` (HTTP bridge with 30s timeout)
- Created `internal/eval/scorer/deepeval_bridge_test.go` (7 tests)
- Coverage: 85.7%
- All 7 bridge tests PASS

### Phase 4: Runner + Report Writer — COMPLETE

- Created `internal/eval/runner/runner.go` (parallel benchmark orchestrator)
- Created `internal/eval/runner/report.go` (JSON + Markdown report writers)
- Created `internal/eval/runner/runner_test.go` (10 tests)
- Created `internal/eval/runner/report_test.go` (3 tests)
- Coverage: 92.7%
- All 13 runner/report tests PASS

### Phase 5: CI Gate — COMPLETE

- Created `internal/eval/ci/gate.go` (exit code mapping: 0/1/2/3)
- Created `internal/eval/ci/gate_test.go` (7 tests)
- Coverage: 94.4%
- All 7 CI gate tests PASS

### Phase 6: CI Workflows — COMPLETE

- Created `.github/workflows/eval.yml` (PR-gating workflow)
- Created `.github/workflows/eval-nightly.yml` (nightly cron workflow)

### Phase 7: Observability — COMPLETE

- Modified `internal/obs/metrics/metrics.go` (added eval benchmark metrics)
- Added: EvalBenchmarkRuns, EvalBenchmarkScore, EvalJudgeCalls, EvalJudgeDuration
- All existing metrics tests still PASS

### Summary

| Metric | Value |
|--------|-------|
| Go tests | 34 total, 34 PASS |
| Python tests | 11 total, 11 PASS |
| Total tests | 45 |
| Packages | 4 (golden, scorer, runner, ci) |
| Coverage (scorer) | 85.7% |
| Coverage (runner) | 92.7% |
| Coverage (ci) | 94.4% |
| New files | 14 |
| Modified files | 3 (app.py, pyproject.toml, metrics.go) |

### Deferred

- Full DeepEval integration (requires deepeval package install and LLM API key)
- Full 50-query benchmark execution in CI (requires judge service deployment)
- `usearch eval` CLI subcommand (SPEC-CLI-002 Phase 8)
- Nightly history writing automation
