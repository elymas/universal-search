# SPEC-EVAL-001 Plan — phased implementation

Status: draft companion to spec.md
Author: limbowl via manager-spec
Date: 2026-05-22
Methodology: TDD (per `.moai/config/sections/quality.yaml`
`development_mode: tdd` default — preserved as TDD because the
benchmark introduces net-new logic on both Go and Python sides;
test-first discipline is the right fit for a brand-new evaluation
package with no characterization-test obligation)
Coverage target: 85% (per quality.yaml `test_coverage_target: 85`)
Harness: standard (per `.moai/config/sections/harness.yaml` auto-
routing — 11 REQs (4 × P0 + 5 × P1 + 2 × P2) + 5 NFRs + 2 new
packages (Go `internal/eval/*`, Python `eval_judge.py`) + 2 CI
workflows; Sprint Contract RECOMMENDED — judge prompt stability
and per-claim scoring semantics are cross-iteration concerns that
benefit from explicit contract negotiation between expert-testing
and evaluator-active per
`.claude/rules/moai/design/constitution.md` §11)

This plan sequences SPEC-EVAL-001 implementation into priority-
ordered phases. Per `.claude/rules/moai/core/agent-common-protocol.md`,
time estimates are PROHIBITED — phases use priority + ordering, never
duration.

---

## 1. Implementation principle

The benchmark is a **read-only consumer** of the synthesis path. The
plan favours:

1. **Read-only of production code** — the runner exercises
   `internal/synthesis`, `internal/fanout`, `services/researcher`
   without modifying them. Any change to production code paths
   that the benchmark relies on is captured as a separate SPEC.
2. **Frozen corpus, deterministic seed** — the corpus
   (`internal/eval/golden/corpus/`) and judge params
   (`temperature=0, top_p=1, seed=42`) are the determinism anchors.
   Tests assert determinism within the ±0.02 variance budget
   (NFR-EVAL1-001).
3. **TDD red-green-refactor** — write the failing test before the
   bridge / runner / gate code. Bridge and runner are pure logic
   (no LLM calls in unit tests); the deepeval call is mocked in
   unit tests and exercised against a real Haiku judge in the
   integration-test sub-phase.
4. **CI workflow last** — the GitHub Actions workflow is the
   final phase because it needs the Go runner + Python sidecar
   + corpus + queries all working in concert. Earlier phases
   validate locally (`go run ./cmd/eval`).
5. **Single judge per V1** — Haiku 4.5 only. Multi-judge
   ensembles, A/B comparisons, and per-locale judge overrides are
   out of scope (spec.md §4).
6. **Cost discipline** — every phase that invokes the real judge
   API logs its cost; phase exit criteria include cost-within-
   budget assertions.

---

## 2. Phase ordering

Priority labels per MoAI rule (no time estimates).

### Phase 0 — Plan-auditor PASS (Priority High)

- Plan-auditor reviews spec.md + research.md + plan.md +
  (Phase 0 also authors) acceptance.md.
- Address MAJOR / MINOR / NIT findings via amendment commits.
- Resolve research §10 Q3 (DeepEval version pin `~= 1.0` vs
  `>= 1.0.0`) and Q7 (PR comment overwrite vs append) during
  annotation cycle before Phase 1 starts.
- Status transition: `draft → approved` once PASS.
- Block: no implementation work begins until Phase 0 completes.

### Phase 1 — Golden set + corpus authoring (Priority High)

Goal: hand-curated 50-query golden set + ≥ 200 doc corpus exist
on disk and load cleanly.

Tasks:

1. Author `internal/eval/golden/queries.jsonl` with all 50
   records per REQ-EVAL1-001 (35 EN + 15 KO, distributed across
   `factual`/`comparison`/`synthesis`/`korean`/`edge` per
   research §3.1).
2. Author `internal/eval/golden/corpus/doc-*.json` fixtures
   (≥ 200 files, each a valid `pkg/types.NormalizedDoc`).
   Strip PII per research §3.3; respect license compliance.
3. Write `internal/eval/golden/manifest.json` with
   `corpus_revision: "1.0.0"`, `total_docs`, `created_at`,
   `license_summary`.
4. Initialise empty `internal/eval/golden/overrides.json` with
   schema `{overrides: []}`.
5. Write Go test `internal/eval/golden/golden_test.go`:
   - `TestGoldenSetCount`: 50 records.
   - `TestGoldenSetSchema`: every record has all required fields.
   - `TestGoldenSetLocalePartition`: 35 EN + 15 KO.
   - `TestCorpusDeserializes`: every JSON file parses to
     `NormalizedDoc`.
   - `TestCorpusSize`: ≥ 200 files.
   - `TestExpectedSourcesResolveToCorpus`: every record's
     `expected_sources` is a subset of corpus `doc_id`s.
   - `TestOverridesSchemaValid`: empty overrides file parses.

Exit criterion:
- All `golden_test.go` tests green.
- Corpus committed to git; `manifest.json:corpus_revision: 1.0.0`.
- `go test ./internal/eval/golden/...` exit 0.

### Phase 2 — Python judge service (Priority High)

Goal: `eval_judge.py` exposes `/judge/faithfulness` endpoint
returning DeepEval-scored per-claim faithfulness verdicts.

Tasks:

1. RED: write `services/researcher/tests/test_eval_judge.py`
   covering all REQ-EVAL1-004 behaviours:
   - `test_judge_endpoint_returns_per_claim_scores`: POST request
     → response has `claim_scores[]` matching input `claims[]`
     length.
   - `test_judge_score_formula`: with 5 claims, 3 supported,
     `faithfulness_score == 0.6`.
   - `test_judge_uses_deterministic_params`: assert deepeval is
     instantiated with `temperature=0, top_p=1, seed=42`.
   - `test_judge_returns_rationale_per_claim`: each
     `claim_scores[i]` has non-empty `judge_rationale`.
   - `test_judge_handles_empty_claims`: 0 claims → score 1.0
     (vacuous truth, documented edge case).
   - `test_judge_handles_unknown_doc_id`: claim cites
     `doc_id` not in `corpus` → claim marked unsupported with
     rationale "cited doc not in retrieval context".
2. GREEN: implement `services/researcher/src/researcher/eval_judge.py`:
   - FastAPI router (extends existing `app.py` from SPEC-SYN-001)
   - Wraps DeepEval `FaithfulnessMetric` via LiteLLM judge
   - Reads `EVAL_JUDGE_MODEL` env var (default
     `claude-haiku-4-5`)
   - Returns the documented JSON schema
3. MODIFY `services/researcher/src/researcher/app.py` to mount
   the new router.
4. MODIFY `services/researcher/pyproject.toml` to add
   `deepeval ~= 1.0` (resolution of research §10 Q3).
5. REFACTOR: extract claim-segmentation logic from
   SPEC-SYN-002's `faithfulness.py` if it can be shared (DRY
   only if the abstraction is clean; otherwise inline).
6. Run `pytest services/researcher/tests/test_eval_judge.py` —
   green.
7. **Calibration sub-phase**: run the judge against 15 Korean
   queries (subset from Phase 1 golden set) with both Haiku 4.5
   and Sonnet 4.5 judges; compare scores. If aggregate gap > 0.10,
   surface as a blocker (open follow-up SPEC for per-locale
   judge override per research §10 Q4).

Exit criterion:
- All `test_eval_judge.py` tests green.
- Calibration sub-phase: Korean judge bias measured at ≤ 0.10
  (or blocker opened with sibling SPEC reference).
- Cost log shows ≤ $0.05 for the calibration run.

### Phase 3 — Go deepeval bridge (Priority High)

Goal: `internal/eval/scorer/deepeval_bridge.go` calls the Python
judge service over HTTP, marshalling synthesis outputs to the
judge's expected schema.

Tasks:

1. RED: write `internal/eval/scorer/deepeval_bridge_test.go`:
   - `TestBridgeMarshalsClaims`: synthesis text
     `"A [1]. B [2]. C [1]."` + 2 docs → 3 claims with correct
     `cited_doc_ids`.
   - `TestBridgeExtractsCitations`: marker-to-doc_id mapping
     correct (consumes `SynthesizeResponse.Citations`).
   - `TestBridgeRespectsTimeout`: judge endpoint stub sleeps 35s;
     bridge times out at 30s; returns error with
     `error_class: "judge_timeout"`.
   - `TestBridgeReturnsPerClaimRationale`: bridge round-trip
     preserves rationale fields.
   - `TestBridgeHandlesJudge5xx`: judge returns 503; bridge
     returns error with `error_class: "judge_unavailable"`.
   - `TestBridgeHandlesMalformedResponse`: judge returns invalid
     JSON; bridge returns error with `error_class:
     "judge_parse_error"`.
2. GREEN: implement `internal/eval/scorer/deepeval_bridge.go`:
   - Public `Score(ctx, synthesisResp, docs) (Result, error)`
   - Claim segmentation via SPEC-SYN-002 regex (shared module
     or inline if simpler)
   - HTTP POST to `${EVAL_JUDGE_URL}/judge/faithfulness`
   - 30s deadline enforced via `context.WithTimeout`
3. REFACTOR: error handling — wrap errors with `fmt.Errorf("%w",
   err)` per Go MUST rules.
4. Add `@MX:ANCHOR` on `Score` function (it will have fan_in ≥ 3
   from runner + nightly cron + CLI eval) with
   `@MX:SPEC: SPEC-EVAL-001` and
   `@MX:REASON: external service contract, judge endpoint changes
   ripple to runner + CI`.
5. Run `go test ./internal/eval/scorer/... -race` — green.

Exit criterion:
- All bridge tests green with `-race`.
- Bridge cleanly wraps DeepEval, no direct LLM API call in Go
  code (confirms NFR-EVAL1-005 — provider abstraction via
  LiteLLM-routing Python service).
- `@MX:ANCHOR` on `Score` documented.

### Phase 4 — Runner + report writer (Priority High)

Goal: `internal/eval/runner/runner.go` orchestrates the full
50-query benchmark with parallelism + retries + aggregation.

Tasks:

1. RED: write `internal/eval/runner/runner_test.go`:
   - `TestRunnerLoadsGoldenSet`: runner reads queries.jsonl,
     iterates 50 records.
   - `TestRunnerCallsSynthesisPerQuery`: with mocked
     synthesis client, asserts 50 calls.
   - `TestRunnerParallelismCap`: with 50 queries, max in-flight
     count ≤ 5 (NFR-EVAL1-004).
   - `TestRunnerCollectsScores`: returns per-query scores.
   - `TestRunnerExcludesOverrides`: queries with active overrides
     not included in mean.
   - `TestRunnerEnforcesOverrideCap`: 6 active overrides → early
     return with `error_class: "override_cap_exceeded"`.
   - `TestRunnerAutoExpiresOverrides`: expired overrides not
     applied; logged as auto-removed.
   - `TestRunnerHandlesJudgeUnavailable`: stub judge fails on
     3 queries; runner marks scores null, continues.
   - `TestRunnerNullScoreExitCode`: any null score → run summary
     `exit_code: 2`.
   - `TestRunnerCostTracking`: per-query cost summed; reported
     in summary.
2. GREEN: implement `runner.go`:
   - Load golden set via `internal/eval/golden` package
   - Spawn synthesis path via existing `internal/fanout` +
     `internal/synthesis` clients (with mocked corpus adapter)
   - Bounded concurrency via `golang.org/x/sync/semaphore`
   - Per-query score collection
   - Override application + cap enforcement
3. RED: write `internal/eval/runner/report_test.go`:
   - `TestReportContainsAllRequiredSections`: Summary, Score by
     category, Lowest-Scoring Queries, Regression Delta, Active
     Overrides.
   - `TestReportTopTenLowestQueriesSection`: with 50 queries
     scored, report lists exactly 10 (or fewer if total < 10).
   - `TestReportContainsJudgeRationalesForLowScores`: every
     low-scoring query has its judge rationale included.
   - `TestReportPerCategoryBreakdown`: aggregation by `category`
     field (NFR-EVAL1-003 transparency).
   - `TestReportCostBreakdown`: per-category + aggregate cost.
   - `TestReportNightlyHistoryWriter`: writes
     `.moai/eval/history/EVAL-001-{date}.json` with documented
     schema (REQ-EVAL1-010).
4. GREEN: implement `report.go`:
   - Markdown rendering (for `.moai/eval/reports/latest.md` +
     PR comment)
   - JSON serialisation (for history)
5. REFACTOR: extract report sections into named functions for
   readability; ensure < 100 LOC per function (TRUST 5
   Readable).
6. Add `@MX:ANCHOR` on `Run` function with `@MX:SPEC:
   SPEC-EVAL-001` and `@MX:REASON: top-level orchestrator, all
   benchmark behaviour funnels through here`.
7. Run `go test ./internal/eval/runner/... -race -cover` —
   green, coverage ≥ 85%.

Exit criterion:
- All runner + report tests green.
- Coverage report ≥ 85% on `internal/eval/runner/`.
- Manual: `go run ./cmd/eval --queries=EVAL-001-Q001..Q005`
  produces valid report against local researcher service.

### Phase 5 — CI gate (Priority High)

Goal: `internal/eval/ci/gate.go` consumes the runner's report and
exits with the documented code mapping.

Tasks:

1. RED: write `internal/eval/ci/gate_test.go`:
   - `TestGatePassesAt085MeanAnd050Floor`: mean=0.87, min=0.55 →
     exit 0.
   - `TestGateFailsBelowMean`: mean=0.82 → exit 1, reason
     "mean below threshold".
   - `TestGateFailsBelowFloor`: mean=0.90, min=0.40 → exit 1,
     reason "floor violation".
   - `TestGateFailsOnNullScores`: any null → exit 2, reason
     "judge availability error".
   - `TestGateFailsOnOverrideCap`: > 5 active overrides → exit 1.
   - `TestGateExitCodeMapping`: assert {pass: 0, score: 1,
     judge: 2, parse: 3}.
   - `TestGateStdoutSummaryFormat`: stdout matches grep-friendly
     `EVAL-001 result=PASS|FAIL mean=<X.XXX> ...` pattern.
2. GREEN: implement `gate.go`:
   - Read JSON report from `--report=path` flag or stdin
   - Apply threshold logic per REQ-EVAL1-008
   - Print one-line summary
   - Exit with documented code
3. REFACTOR: pure function `Decide(report) Result` separates
   logic from I/O; CLI wrapper is trivial.
4. Run `go test ./internal/eval/ci/... -race -cover` — green.

Exit criterion:
- All gate tests green.
- Manual: pipe a sample report to `go run ./cmd/eval gate` →
  correct exit code.

### Phase 6 — CI workflow + nightly cron (Priority Medium)

Goal: GitHub Actions workflows run the benchmark on PRs touching
synthesis paths and nightly at 03:00 UTC.

Tasks:

1. Author `.github/workflows/eval.yml` per REQ-EVAL1-009:
   - Trigger: `pull_request` with path filters per research §6.2
   - Job: checkout, set up Go + Python, install deepeval, boot
     researcher service in background, run
     `go run ./cmd/eval --report=eval-report.json`, then
     `go run ./cmd/eval gate --report=eval-report.json`
   - Post report markdown as PR comment (using existing GitHub
     CLI in workflow)
   - Concurrency group keyed on `${{ github.workflow }}-${{
     github.event.pull_request.number }}` to prevent collisions
2. Author `.github/workflows/eval-nightly.yml` per REQ-EVAL1-010:
   - Trigger: `schedule: cron: '0 3 * * *'`
   - Job: same as eval.yml but writes to
     `.moai/eval/history/EVAL-001-${date}.json` and commits the
     history file back via PR-bot
   - No PR comment; nightly job does NOT gate any merge
3. Add path filter test: simulate dry-run with
   `act` or workflow lint to verify paths trigger / skip
   correctly.
4. Add concurrency / secret access scoping per research §9 risk 3.
5. Manual: open a draft PR touching `internal/synthesis/synthesis.go`;
   verify eval workflow triggers, runs, and posts comment.

Exit criterion:
- Both workflows pass `actionlint`.
- Draft PR demonstrates end-to-end CI workflow.
- PR comment renders correctly.

### Phase 7 — Observability + metrics (Priority Medium)

Goal: two new Prometheus collectors wired through SPEC-OBS-001
allowlist.

Tasks:

1. MODIFY `internal/obs/metrics/metrics.go`:
   - Register `usearch_eval_runs_total{outcome}` (Counter)
   - Register `usearch_eval_score_gauge` (Gauge, no labels)
2. MODIFY `internal/obs/metrics/router_test.go`:
   - Extend cardinality allowlist test to assert
     `usearch_eval_*` metrics are allowlisted
   - Assert `outcome` label values are bounded:
     `{pass, fail, null, override_cap}` (4 values, well below
     cardinality limits)
3. MODIFY `internal/eval/runner/runner.go`:
   - Emit `usearch_eval_runs_total.WithLabelValues(outcome).Inc()`
     at end of each run
   - Set `usearch_eval_score_gauge.Set(meanScore)` after each
     run
4. Verify allowlist test passes: `go test ./internal/obs/...`.

Exit criterion:
- Both metrics registered.
- Cardinality allowlist test still passing (no new label keys
  introduced).
- Manual: hit `/metrics` endpoint after a benchmark run; verify
  both metrics present with expected values.

### Phase 8 — CLI surface (Priority Low)

Goal: `usearch eval` subcommand wraps the runner for local
developer use.

Tasks:

1. Coordinate with SPEC-CLI-002 owner: the `eval` subcommand is
   delegated to CLI-002 Phase 8. SPEC-EVAL-001 plan emits the
   contract (flags, output, exit codes) for CLI-002 to wire.
2. Write contract doc at `internal/eval/cmd-eval-contract.md`:
   - Flags: `--queries=<id-pattern>`, `--report=<path>`,
     `--judge-model=<litellm-id>`, `--quiet`
   - Stdout format matches CI gate summary
   - Exit codes match CI gate mapping (REQ-EVAL1-008)
3. If CLI-002 Phase 8 is already in progress, file a coordination
   issue with cross-references; otherwise, the contract doc
   stands as the spec for the future wiring.
4. Manual: `go run ./cmd/eval --queries=EVAL-001-Q001..Q005`
   demonstrates the same behaviour the CLI surface will expose.

Exit criterion:
- Contract doc committed.
- `go run ./cmd/eval` is the documented local-dev entry point
  pre-CLI-002 wiring.

### Phase 9 — Sync phase (Priority Low)

Goal: documentation + PR.

Tasks:

1. `manager-docs` updates user-facing docs:
   - Parent repo `README.md`: add "Evaluation" section linking
     to `.moai/eval/README.md`
   - `.moai/eval/README.md`: operator guide for reading reports,
     applying overrides, interpreting calibration drift alerts
2. CHANGELOG entry in parent repo under M8.
3. Update `.moai/project/roadmap.md` SPEC-EVAL-001 row:
   `status: implemented`.
4. `manager-git` opens PR per V1 release process.
5. Status transition: `approved → implemented` after merge +
   first green CI run on main.

---

## 3. Test inventory (TDD checkpoints)

Per-phase tests are listed inline above. Aggregated coverage:

- Phase 1: 7 Go tests (`golden_test.go`).
- Phase 2: 6 Python tests (`test_eval_judge.py`) + calibration
  sub-phase (manual log).
- Phase 3: 6 Go tests (`deepeval_bridge_test.go`).
- Phase 4: 10 Go tests (`runner_test.go` + `report_test.go`).
- Phase 5: 7 Go tests (`gate_test.go`).
- Phase 6: workflow lint + manual draft-PR test.
- Phase 7: 1 modified test (cardinality allowlist).
- Phase 8: manual contract validation.

Total automated: 37 unit tests + integration tests in Phase 2
(judge service end-to-end) + Phase 4 (runner against local
synthesis stack).

Coverage target: 85% per quality.yaml. Achievable with
table-driven Go tests + property tests via `hypothesis` on the
Python side for judge endpoint edge cases.

---

## 4. MX tag plan

| File | Tag | Function | Reason |
|------|-----|----------|--------|
| `internal/eval/scorer/deepeval_bridge.go` | `@MX:ANCHOR` | `Score` | External service contract; judge endpoint changes ripple to runner + CI. fan_in ≥ 3 (runner + cron + CLI). |
| `internal/eval/runner/runner.go` | `@MX:ANCHOR` | `Run` | Top-level orchestrator; benchmark behaviour funnels through here. fan_in ≥ 3 (CLI + CI workflow + nightly cron). |
| `internal/eval/ci/gate.go` | `@MX:ANCHOR` | `Decide` | Pure decision function; threshold semantics MUST be preserved across refactors. fan_in ≥ 2 (CI workflow + nightly cron, growing). |
| `services/researcher/src/researcher/eval_judge.py` | `# @MX:NOTE` | judge endpoint | Deterministic params (`temperature=0, top_p=1, seed=42`) MUST be preserved per NFR-EVAL1-001; changes invalidate the regression baseline. |
| `internal/eval/runner/runner.go` | `@MX:WARN` | parallel exec loop | Bounded goroutine pool; if parallelism cap changes, NFR-EVAL1-004 runtime budget needs re-validation. `@MX:REASON: judge rate-limit + sidecar concurrency assumptions are encoded in cap = 5`. |

All tags follow `code_comments: en` per
`.moai/config/sections/language.yaml`.

---

## 5. Risk-driven sequencing notes

Risks from research.md §9 with their mitigation phase:

- **Haiku judge calibration drift** → Phase 2 calibration sub-phase
  + Phase 6 nightly cron baseline + NFR-EVAL1-001 alert wiring.
- **DeepEval API churn** → Phase 2 version pin `~= 1.0`; Phase 3
  bridge tests assert documented JSON schema (not internal
  DeepEval API).
- **GitHub Actions secrets leakage** → Phase 6 secret scoping +
  existing gitleaks CI (NFR-SKILL-005 pattern reused).
- **Korean judge bias** → Phase 2 calibration sub-phase blocks
  Phase 3 if bias > 0.10.
- **Golden set staleness** → Phase 1 + Phase 9 ship; refresh is a
  future patch SPEC, not gated in V1.
- **CI runtime overrun** → Phase 4 parallelism cap of 5 +
  NFR-EVAL1-004 25-min hard kill.
- **Override-cap exhaustion** → Phase 4 runner enforces cap with
  exit 1; documented as "operator must fix root cause, not raise
  cap".
- **Synthesis path changes breaking corpus assumptions** → Phase 3
  bridge consumes `SynthesizeResponse.Citations` by contract;
  changes to that contract are a separate SPEC.
- **Cost overrun on retry storms** → Phase 3 30s timeout enforced;
  no automatic retries; failed queries mark null and continue.
- **Concurrent CI runs colliding** → Phase 6 concurrency group
  ensures one EVAL-001 per commit.

---

## 6. Sync-phase deliverables (Phase 9)

- Parent repo `README.md`: add "Evaluation" section.
- `.moai/eval/README.md`: operator guide.
- Parent repo `CHANGELOG.md`: SPEC-EVAL-001 entry under M8.
- `.moai/project/roadmap.md`: SPEC-EVAL-001 row → `implemented`.
- PR title: `feat(eval): implement SPEC-EVAL-001 — citation
  faithfulness benchmark with DeepEval CI gate (M8)`.
- PR body: links to spec.md, research.md, acceptance.md;
  benchmark report from main-branch first run; checklist of REQ
  acceptance.
- Status transition: `approved → implemented` on merge + first
  green CI run on main.
- Notify: M9 owner (SPEC-REL-001) that the V1.0.0 tag gate is
  now active — releases blocked until benchmark passes on main.

---

## 7. Open factoring decisions deferred to run phase

These items are explicitly NOT decided at plan time — they are
implementation-detail choices the run-phase agent will make:

1. **DeepEval version pin** — `~= 1.0` recommended (allows
   patch upgrades). Annotation cycle confirms.

2. **PR comment overwrite vs append** — overwrite with timestamp
   footer recommended (cleanliness). Annotation cycle confirms.

3. **Reference handling for retrieved-but-not-cited docs** — pass
   only cited docs to judge (focused scope) vs full retrieval
   context. Phase 3 verifies against DeepEval docs; defaults to
   cited-only per spec.md §10 Q9.

4. **Cost report granularity** — per-category + aggregate
   recommended; annotation cycle confirms.

5. **Claim segmentation sharing** — Phase 3 decides whether to
   extract a shared module between SPEC-SYN-002 `faithfulness.py`
   and EVAL-001 bridge, or inline the regex in each. Decision
   anchored on DRY benefit vs coupling cost.

6. **Golden set authoring style** — V1 author = limbowl; manual
   curation. Phase 1 produces the 50 queries; format consistency
   is the author's discipline.

7. **Per-claim cost tracking** — DeepEval doesn't natively expose
   cost per metric call; Phase 4 wraps the LiteLLM client to
   capture per-call cost. Implementation pattern decided in
   Phase 4.

8. **Calibration cadence** — monthly Sonnet calibration is the
   plan; whether to wire it as a separate workflow
   (`.github/workflows/eval-calibration.yml`) or as a manual
   ad-hoc run is Phase 6 decision.

9. **Override approval workflow** — V1 lets any committer add
   overrides (with mandatory `override_reason`). 2-person
   approval is a future enhancement (research §10 Q5).

10. **Nightly history retention** — V1 keeps all history files
    forever (modest size, < 100 KB per file). Pruning policy is
    a Phase 9 docs note; not enforced in code for V1.

These are scope-bounded — none change the SPEC contract; all are
mechanical implementation choices.

---

*End of SPEC-EVAL-001 plan v0.1.0 (draft).*
