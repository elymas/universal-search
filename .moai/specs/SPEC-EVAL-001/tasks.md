# SPEC-EVAL-001 Tasks — atomic decomposition

Status: Phase 1 (analysis/planning) output — NOT yet approved for implementation.
Methodology: TDD (RED → GREEN → REFACTOR). Coverage target 85%.
Harness: standard (confirmed from harness.yaml — P1 feature SPEC, multi-domain, no security/payment keywords → standard, plan-auditor enabled).

> [!IMPORTANT] Pre-implementation gate: plan-auditor PASS required (standard harness `plan_audit.enabled: true`, `require_must_pass: true`). No plan-auditor report exists yet at `.moai/reports/plan-audit/SPEC-EVAL-001-*`. See progress.md blockers.

> [!WARNING] SPEC factual corrections (verified 2026-05-29). The SPEC's contract references must be corrected before/during implementation:
> 1. SYN-002 faithfulness path does NOT live at `services/researcher/src/researcher/faithfulness.py` (does not exist). Actual: `faithfulness_endpoint.py` + `internal/synthesis/faithfulness.go` (owner = SPEC-DEEP-002 REQ-DEEP2-006).
> 2. The marker/sentence regex the SPEC quotes (`[.!?。！？]\s+...` with CJK) does NOT match real code. Real: `_MARKER_RE = r"\[(\d+)\]"`, sentence split `(?<=[.!?])\s+` (no CJK). Korean-claim segmentation must be re-decided in run phase (affects 15 KO queries).
> 3. Synthesis Go result type is `synthesis.Result` (not `SynthesizeResponse`); citations are `[]Citation{Marker int, DocID, URL, Title}`. Bridge marshalling must target this shape.
> 4. Orphaned `.pyc` cache exists for `eval_judge`/`test_eval_judge` (no source). Clean `__pycache__` before Phase 2 to avoid import confusion.

---

## Task list (TDD-cycle atomic units)

| ID | Description | REQ mapping | Depends | Planned files/dirs | TDD note |
|----|-------------|-------------|---------|--------------------|----------|
| **T-EVAL1-00** | Plan-auditor gate + SPEC contract corrections. Run plan-auditor on spec/plan/acceptance/research; resolve the 4 factual corrections above + research §10 Q3 (deepeval pin) & Q7 (PR comment policy); transition status draft→approved. | (gate) | — | spec.md, plan.md amendments; `.moai/reports/plan-audit/SPEC-EVAL-001-review-1.md` | n/a (pre-impl gate) |
| **T-EVAL1-01** | Golden set + frozen corpus authoring. 50 queries (35 EN + 15 KO) + ≥200 NormalizedDoc fixtures + manifest + empty overrides; Go schema/partition/resolve tests. | REQ-EVAL1-001, -002, -003 (schema) | T-00 | `internal/eval/golden/{queries.jsonl,corpus/*.json,manifest.json,overrides.json}`, `internal/eval/golden/golden_test.go` | RED: golden_test.go (7 tests) → GREEN: author data → REFACTOR: loader helpers |
| **T-EVAL1-02** | Python DeepEval judge service. `/judge/faithfulness` wrapping DeepEval FaithfulnessMetric via LiteLLM, deterministic params (temp=0,top_p=1,seed=42), `EVAL_JUDGE_MODEL` env. Add `deepeval` to pyproject. Clean stale `__pycache__` first. | REQ-EVAL1-004; NFR-EVAL1-005 | T-00 | `services/researcher/src/researcher/eval_judge.py`, `services/researcher/tests/test_eval_judge.py`, MODIFY `app.py` (mount router), `pyproject.toml` (+deepeval ~=1.0) | RED: test_eval_judge.py (6 tests) → GREEN: endpoint → REFACTOR: share claim-seg only if clean |
| **T-EVAL1-03** | Judge calibration sub-phase (Korean bias check). Run 15 KO queries through Haiku vs Sonnet judge; gap >0.10 → BLOCKER + follow-up SPEC. Re-decide KO claim segmentation (correction #2). | REQ-EVAL1-004 (KO); research §10 Q4 | T-01, T-02 | calibration log (manual), possible blocker report | n/a (validation; may gate T-04) |
| **T-EVAL1-04** | Go DeepEval bridge. `Score(ctx, result synthesis.Result, docs) (Result, error)`: claim-split, extract cited doc_ids from `[]Citation`, POST to judge, 30s timeout, per-claim rationale, error_class taxonomy. `@MX:ANCHOR` on Score. | REQ-EVAL1-005; NFR-EVAL1-002 | T-02, T-03 | `internal/eval/scorer/deepeval_bridge.go`, `deepeval_bridge_test.go` | RED: 6 bridge tests (-race) → GREEN → REFACTOR: error wrapping |
| **T-EVAL1-05** | Runner orchestrator. Loads golden set, drives synthesis path (mocked corpus adapter via fanout registry interface), bounded concurrency ≤5, override apply+cap+expiry, null-score handling, exit-code mapping, cost tracking. `@MX:ANCHOR` on Run, `@MX:WARN` on parallel loop. | REQ-EVAL1-006; NFR-EVAL1-004; REQ-EVAL1-003 (runtime) | T-04 | `internal/eval/runner/runner.go`, `runner_test.go` | RED: 10 runner tests → GREEN → REFACTOR |
| **T-EVAL1-06** | Report writer. Markdown (latest.md + PR comment) + JSON history; sections incl. Lowest-Scoring Queries w/ rationales, per-category + cost breakdown, regression delta. | REQ-EVAL1-007, -010 | T-05 | `internal/eval/runner/report.go`, `report_test.go` | RED: 6 report tests → GREEN → REFACTOR: <100 LOC/fn |
| **T-EVAL1-07** | CI gate decision logic. Pure `Decide(report) Result` + thin CLI; exit codes 0/1/2/3; grep-friendly stdout summary line. | REQ-EVAL1-008 | T-06 | `internal/eval/ci/gate.go`, `gate_test.go`, `cmd/eval/main.go`, MODIFY `internal/eval/eval.go` (re-exports) | RED: 7 gate tests → GREEN → REFACTOR: split I/O from logic |
| **T-EVAL1-08** | CI workflows. PR-gating `eval.yml` (path filters, boots researcher sidecar, runs runner+gate, posts PR comment, concurrency group) + nightly `eval-nightly.yml` (cron 03:00 UTC, writes history, no gate). actionlint-clean. | REQ-EVAL1-009, -010 | T-07 | `.github/workflows/{eval.yml,eval-nightly.yml}`, `.moai/eval/{history,reports}/.gitkeep`, `.moai/eval/README.md` | n/a (workflow lint + draft-PR manual verify) |
| **T-EVAL1-09** | Observability metrics + sync. Register `usearch_eval_runs_total{outcome}` + `usearch_eval_score_gauge` via existing allowlist; emit from runner; extend cardinality test. Then docs/CHANGELOG/roadmap status + PR. CLI `usearch eval` surface = contract doc only (delegate to CLI-002, already implemented — coordinate, do not wire here). | NFR (metrics); REQ-EVAL1-011 (contract); roadmap | T-05, T-08 | MODIFY `internal/obs/metrics/metrics.go`, `router_test.go`, `runner.go`; `internal/eval/cmd-eval-contract.md`; README/CHANGELOG/roadmap | RED: extend allowlist test → GREEN: register collectors |

---

## Notes on decomposition choices

- **9 implementation tasks + 1 gate = 10 total** (at the max-10 limit). Plan's Phase 8 (CLI surface) is folded into T-09 as a contract-doc-only deliverable because SPEC-CLI-002 is already `implemented` — the `usearch eval` subcommand is CLI-002's to wire, not EVAL-001's. EVAL-001 ships `cmd/eval/main.go` as the standalone entry point (T-07).
- **T-03 (calibration) is a validation gate**, not a code-writing task — placed between Python judge (T-02) and Go bridge (T-04) per plan §2 because a Korean-bias blocker must surface before bridge work commits.
- **Dependency chain is mostly linear** (T-01..T-09 sequential), which is correct for a single-domain eval harness — this argues for **solo sub-agent mode, not team mode** (low parallelism opportunity; heavy inter-task dependencies).
- All implementation tasks own non-overlapping file trees except T-09 which touches shared `internal/obs/metrics/` (sequence it last, after runner is stable).
