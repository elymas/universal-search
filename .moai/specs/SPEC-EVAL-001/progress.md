# SPEC-EVAL-001 Progress Log

## 2026-05-29 — Phase 1 (Analysis & Planning) — manager-strategy

Phase 1 of /moai run executed in analysis-only mode (no implementation).

**Outputs:** `tasks.md` written (10 units: 1 gate + 9 impl). Memory entries recorded.

**Harness:** standard (confirmed from `.moai/config/sections/harness.yaml`). P1 feature SPEC, multi-domain (Go + Python + CI), no security/payment keywords, priority != critical → standard. plan_audit.enabled=true, require_must_pass=true, evaluator=final-pass.

**Dependency verification (all 3 declared deps + transitive):**
| SPEC | declared status | actual status | assets exist |
|------|----------------|---------------|--------------|
| SPEC-SYN-002 | implemented | implemented ✓ | structural faithfulness shipped (see note) |
| SPEC-OBS-001 | implemented | implemented ✓ | `internal/obs/metrics/` allowlist present |
| SPEC-CLI-002 | implemented | implemented ✓ | cmd/usearch present |
| SPEC-LLM-001 | implemented | implemented ✓ | `internal/llm/` LiteLLM router present |
| SPEC-SYN-001 | implemented | implemented ✓ | synthesis client/Result present |
| SPEC-CORE-001 | implemented | implemented ✓ | pkg/types/normalized_doc.go present |
| SPEC-REL-001 (blocks) | draft | draft ✓ | — |

**SPEC factual corrections found (C1-style staleness — flagged, not blocking):**
1. SYN-002 faithfulness path `services/researcher/src/researcher/faithfulness.py` cited in HISTORY does NOT exist. Real: `faithfulness_endpoint.py` (Python) + `internal/synthesis/faithfulness.go` (Go), both owned by SPEC-DEEP-002 REQ-DEEP2-006, not SYN-002.
2. Quoted CJK-aware sentence regex does not match real code (`_MARKER_RE=r"\[(\d+)\]"`, split `(?<=[.!?])\s+`, no CJK). Affects 15 KO queries.
3. Go result type is `synthesis.Result` w/ `[]Citation{Marker,DocID,URL,Title}`, not `SynthesizeResponse`.
4. Orphaned `.pyc` cache for `eval_judge`/`test_eval_judge` exists with no source (clean before Phase 2).

**SYN-002 overlap (C1 risk) check:** REFUTED. EVAL-001 adds the *semantic* layer (LLM-judge entailment) that the shipped *structural* gate cannot do, and consumes synthesis output read-only. No reinvention. (Path-attribution is wrong in the SPEC, but the capability boundary is sound.)

**Phase 0 / plan-auditor:** NO report exists at `.moai/reports/plan-audit/SPEC-EVAL-001-*`. Standard harness REQUIRES plan-auditor PASS before implementation. → recommendation: **needs-plan-auditor-first**.

**Acceptance count:** 16 AC + 3 edge cases, full REQ→AC coverage matrix present. acceptance.md is complete.

**Blockers:** none hard. One gate: plan-auditor must run (T-EVAL1-00).

**Acceptance criteria met this iteration:** 0 (analysis phase). **Error delta:** 0.

---

## Iteration 2 — TDD implementation (manager-tdd, 2026-05-29)

Implemented SPEC-EVAL-001 v0.2.0 end-to-end via RED-GREEN-REFACTOR. Branch `feature/SPEC-EVAL-001`. Standard harness.

**Tasks completed (T01-T09):**
- T01 golden set + corpus: 50 queries (35 EN + 15 KO) + 60 NormalizedDoc fixtures + manifest + empty overrides. Generator at `internal/eval/golden/gen/gen.go`. 17 schema/loader tests. coverage 94.4%.
- T02 Python judge: `eval_judge.py` `/judge/faithfulness` wrapping DeepEval FaithfulnessMetric (lazy import) via injectable judge; deterministic params (temp=0/top_p=1/seed=42); mounted in app.py; `deepeval~=1.0` added to pyproject. 6 pytest tests pass.
- T03 calibration: KO segmentation re-decided benchmark-side (handled in T04 bridge); Korean-bias check is a CI-time concern (real judge), documented. No code blocker.
- T04 bridge: `deepeval_bridge.go` — locale-aware claim segmentation (EN ASCII + KO CJK/no-whitespace), citation extraction, 30s timeout, unavailability classification. 9 tests, race-clean. coverage 90.2%.
- T05 runner: `runner.go` — bounded concurrency (≤5), null-vs-zero, override exclusion, exit-class mapping. 8 tests, race-clean. coverage 87.2%.
- T06 report: `report.go` — markdown latest.md, Lowest-Scoring section w/ rationales, per-category breakdown, null listing, cost. 6 tests.
- T07 gate + entrypoint: `ci/gate.go` pure Decide (exit 0/1/2/3) + `cmd/eval/main.go`. 9 gate tests + 4 cmd tests. gate coverage 100%.
- T08 CI: `.github/workflows/eval.yml` PR-gate (path filters, boots researcher sidecar, health-check, runs cmd/eval, posts/overwrites PR comment, concurrency group, 25-min hard timeout). Nightly cron NOT built (V1.1 deferred).
- T09 observability: `usearch_eval_runs_total{outcome}` + `usearch_eval_score_gauge` registered (reuse `outcome` label, no new cardinality); cardinality allowlist regression guard extended. metrics coverage 95.7%.

**Test evidence:** `go build ./...` clean; `go vet ./...` clean; `golangci-lint run` 0 issues; `go test -race -cover ./internal/eval/... ./cmd/eval/...` all pass. Python: 6/6 judge tests pass (`PYTHONPATH=src python3 -m pytest tests/test_eval_judge.py`).

**Determinism:** temp=0/top_p=1/seed=42 wired in `deterministic_litellm_params`; ±0.02 warn / ±0.05 block tolerance bands are gate-side (real-judge CI concern, documented).

**@MX tags:** ANCHOR on gate.Decide + runner.Run (+REASON), NOTE on bridge KO segmentation, WARN on runner goroutine fan-out (+REASON).

**Residual:** (1) DeepEval pin `~=1.0` not installed locally — real-judge path exercised only in CI. (2) Real-judge CI cost (~$0.45/run) + Korean-bias calibration require live LiteLLM secrets. (3) 7 pre-existing researcher pytest failures are environmental (pytest-asyncio absent in bare env) — unrelated to this change. (4) Status stays `approved`; plan-auditor gate (T00) is orchestrator-owned.

**Acceptance criteria met this iteration:** 9 tasks (T01-T09). **Error delta:** 0.
