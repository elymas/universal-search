---
id: SPEC-EVAL-001
version: 0.1.0
status: draft
created: 2026-05-26
author: limbowl (via manager-spec)
related_spec: SPEC-EVAL-001 (spec.md, plan.md)
format: Given/When/Then
---

# SPEC-EVAL-001 Acceptance Scenarios

## 0. Document Purpose

This document specifies acceptance criteria for SPEC-EVAL-001 in Given/When/Then format, expanding the scenario index in spec.md §5 (§5.1..§5.16) into externally-observable behaviors that the run phase MUST verify before declaring EVAL-001 ship-ready.

Scope: 16 acceptance criteria (AC-001..AC-016) covering REQ-EVAL1-001 through REQ-EVAL1-011 + NFR-EVAL1-001 through NFR-EVAL1-005, plus 3 edge-case sections, plus a Definition of Done checklist.

Coverage policy: every REQ and every NFR in spec.md §2 / §3 has ≥1 matching AC below. See Coverage Matrix at end of file.

---

## 1. Acceptance Criteria (Given/When/Then)

### AC-001 — Golden set + corpus load with correct partition

Covers: REQ-EVAL1-001, REQ-EVAL1-002

**Given** `internal/eval/golden/queries.jsonl` with 50 query records and `internal/eval/golden/corpus/*.json` with ≥ 200 NormalizedDoc fixtures.

**When** the runner starts and loads the golden set.

**Then**:
- Exactly 50 query records parse successfully (one per line).
- Locale partition is exactly 35 `locale: "en"` + 15 `locale: "ko"`.
- Each record has the required fields: `id` matching format `EVAL-001-Q{NNN}`, `query`, `locale ∈ {"en","ko"}`, `category ∈ {"factual","comparison","synthesis","korean","edge"}`.
- Corpus contains ≥ 200 distinct docs, each deserializing into `pkg/types.NormalizedDoc`.
- Every `expected_sources` entry in any query record resolves to a valid `doc_id` in the corpus.
- `.moai/eval/golden/manifest.json` contains a `corpus_revision` field.

Maps to scenario §5.1 in spec.md.

---

### AC-002 — Single-query happy path: faithful synthesis scores 1.0

Covers: REQ-EVAL1-004, REQ-EVAL1-005

**Given** query Q001 (`category: "factual"`, `locale: "en"`) and a synthesis response with 3 claims each citing a valid `[N]` marker matching the retrieved corpus.

**When** the runner executes Q001 through the synthesis path + DeepEval bridge.

**Then**:
- The Python `eval_judge.py` `/judge/faithfulness` endpoint receives `{query_id, claims, corpus}`.
- DeepEval `FaithfulnessMetric` is invoked with `temperature=0, top_p=1, seed=42` (deterministic).
- The endpoint returns 3 per-claim scores all `supported: true`.
- `faithfulness_score = 3/3 = 1.0`.
- The runner records `{query_id: "EVAL-001-Q001", score: 1.0}` in the report.

Maps to scenario §5.2 in spec.md.

---

### AC-003 — Partial faithfulness captures judge rationale for unsupported claim

Covers: REQ-EVAL1-005, REQ-EVAL1-007

**Given** query Q010 (`category: "synthesis"`, `locale: "en"`) with a 4-claim synthesis response where 1 claim cites a doc that does not actually support it.

**When** the runner executes Q010.

**Then**:
- The bridge splits the response into 4 claims using the SPEC-SYN-002 sentence regex.
- The judge returns `claim_scores: [supported=true, supported=true, supported=true, supported=false]`.
- The unsupported claim includes a non-empty `judge_rationale` text.
- `faithfulness_score = 3/4 = 0.75`.
- The per-query report record contains the full rationale text for the unsupported claim.
- If Q010 ranks in the 10 lowest-scoring queries of the run, it appears under `## Lowest-Scoring Queries` in the report with its rationale.

Maps to scenario §5.3 in spec.md.

---

### AC-004 — Korean query routes through ko-locale path and scores in Korean entailment

Covers: REQ-EVAL1-001 (locale partition), REQ-EVAL1-004

**Given** query Q036 (`locale: "ko"`, `category: "korean"`) with Korean source text.

**When** the runner executes Q036 with the mocked Korean adapter fixtures.

**Then**:
- The synthesis path returns Korean response text.
- The judge endpoint receives Korean claims + Korean corpus excerpts.
- DeepEval scores in Korean entailment context (judge model has multilingual capability).
- The score is recorded in the report with `locale: "ko"` tag for per-locale slicing.

Maps to scenario §5.4 in spec.md.

---

### AC-005 — Aggregate pass with mean ≥ 0.85 and floor ≥ 0.50

Covers: REQ-EVAL1-008, REQ-EVAL1-009

**Given** a full 50-query run where mean = 0.89 and every non-null query score ≥ 0.62.

**When** the CI gate executes against the run report.

**Then**:
- Exit code is `0`.
- stdout contains `EVAL-001 result=PASS mean=0.890 floor=0.62 overrides=0 nulls=0`.
- The PR comment is posted (or updated, if a prior EVAL-001 comment exists from the same PR) with the markdown report.
- Workflow `.github/workflows/eval.yml` triggered on a PR touching `internal/synthesis/**`.

Maps to scenarios §5.5, §5.9 in spec.md (eval.yml trigger).

---

### AC-006 — Aggregate fail (mean below threshold)

Covers: REQ-EVAL1-008, REQ-EVAL1-007

**Given** a 50-query run where mean = 0.82 (< 0.85).

**When** the CI gate executes.

**Then**:
- Exit code is `1`.
- stdout contains `EVAL-001 result=FAIL mean=0.820 floor=<X.XX> overrides=<N> nulls=0`.
- The PR comment shows the top 10 lowest-scoring queries with judge rationales.
- The CI workflow status is `failure`.

Maps to scenario §5.6 in spec.md.

---

### AC-007 — Aggregate fail (per-query floor violation)

Covers: REQ-EVAL1-008

**Given** a 50-query run where mean = 0.87 but Q017 scored 0.40 (< 0.50 floor).

**When** the CI gate executes.

**Then**:
- Exit code is `1`.
- The summary explicitly cites "floor violation: Q017 scored 0.40 < 0.50".
- The PR comment highlights Q017 in the lowest-scoring section with its judge rationale.

Maps to scenario §5.7 in spec.md.

---

### AC-008 — Judge unavailability marks scores null, exit code 2

Covers: REQ-EVAL1-006, NFR-EVAL1-002

**Given** a 50-query run where the `eval_judge.py` HTTP service returns 503 on 3 specific queries.

**When** the runner executes.

**Then**:
- ERROR-level log records are emitted with `{query_id, judge_model, error_class}` for each affected query.
- Affected query scores are `null` (NOT zero).
- The aggregate mean is computed over the remaining 47 non-null scores.
- The runner exits with code `2` (judge availability error).
- The report summary shows `nulls=3` and the 3 affected query IDs.
- Any single judge call exceeding 30s wall-clock is treated as unavailability per NFR-EVAL1-002.

Maps to scenario §5.8 in spec.md.

---

### AC-009 — Override applied: excluded from aggregate, logged in report

Covers: REQ-EVAL1-003

**Given** `internal/eval/golden/overrides.json` containing:
```json
[{"query_id": "EVAL-001-Q023", "manual_override": "pass", "override_reason": "judge mis-scores known compound sentence", "expires_at": "2026-06-20", "created_at": "2026-05-22T12:00:00Z", "created_by": "elymas"}]
```
and 5 total active overrides (within cap).

**When** the runner executes the 50-query benchmark.

**Then**:
- Q023 is treated as a forced pass (excluded from aggregate calculation OR scored as 1.0 per implementation decision; either way reported as an override).
- The override usage is logged in the per-query report with the reason.
- Expired overrides (e.g., entries with `expires_at < today`) are auto-removed before scoring.
- The report summary shows `overrides=N` where N is the count of active applied overrides.

Maps to scenario §5.9 in spec.md.

---

### AC-010 — Override cap (5) enforced; exceed fails CI

Covers: REQ-EVAL1-003

**Given** `overrides.json` containing 6 active (non-expired) override entries.

**When** the runner starts.

**Then**:
- The pre-check fails BEFORE any query runs.
- Exit code is `1`.
- The summary contains "override cap exceeded: 6 active overrides, max 5 allowed".
- CI status is `failure`.

Maps to scenario §5.10 in spec.md.

---

### AC-011 — Nightly cron run writes history without gating

Covers: REQ-EVAL1-010

**Given** GitHub Actions cron schedule `0 3 * * *` (03:00 UTC).

**When** the scheduled workflow fires.

**Then**:
- The full 50-query benchmark runs.
- A file is written at `.moai/eval/history/EVAL-001-YYYY-MM-DD.json` containing `{date, commit_sha, branch, mean_score, per_query_scores, judge_model, override_count, null_count, runtime_seconds}`.
- `.moai/eval/reports/latest.md` is updated with the human-readable report.
- The nightly run does NOT block any merge or PR (it only writes the baseline).

Maps to scenario §5.11 in spec.md.

---

### AC-012 — Determinism: 3 consecutive runs within ±0.02 variance

Covers: NFR-EVAL1-001

**Given** the same revision + same `EVAL_JUDGE_MODEL` + same corpus revision.

**When** the runner is invoked 3 times consecutively.

**Then**:
- Aggregate mean scores from the 3 runs differ by ≤ 0.02 from each other.
- Variance > 0.02 emits a "calibration drift" warning (non-blocking).
- Variance > 0.05 BLOCKS CI with exit code 2 until calibration is re-stabilised.
- The deterministic params `temperature=0, top_p=1, seed=42` are propagated to LiteLLM in every call.

Maps to scenario §5.12 in spec.md.

---

### AC-013 — Cost report: ≤ $0.50 per full run

Covers: NFR-EVAL1-003

**Given** a complete 50-query CI run with Claude Haiku 4.5 as judge.

**When** the runner finishes.

**Then**:
- The run report contains a total cost line: `Total LLM judge cost: $X.XXXX USD`.
- The total cost is ≤ $0.50.
- A per-query cost breakdown is available in the report (e.g., as a collapsed details block).
- A cost > $1.00 emits an alert AND requires human review (CI continues but flags the run).

Maps to scenario §5.13 in spec.md.

---

### AC-014 — Runtime budget: ≤ 15 min on standard runner

Covers: NFR-EVAL1-004

**Given** a complete 50-query run on a 4 vCPU + 16 GB RAM GitHub Actions runner.

**When** the runner is invoked.

**Then**:
- Wall-clock time ≤ 15 minutes.
- The runner parallelizes at most 5 concurrent queries.
- Runtime between 15 and 25 minutes emits a WARNING.
- Runtime > 25 minutes fails the job (GitHub Actions timeout, exit 124).

Maps to scenario §5.14 in spec.md.

---

### AC-015 — Provider swap via EVAL_JUDGE_MODEL env var (no code change)

Covers: NFR-EVAL1-005

**Given** the deployed benchmark suite with `EVAL_JUDGE_MODEL` env var.

**When** the operator sets `EVAL_JUDGE_MODEL=gpt-4o-mini` and re-runs.

**Then**:
- The benchmark runs successfully against the OpenAI-backed judge.
- ZERO code changes are required in `services/researcher/src/researcher/eval_judge.py` or the Go bridge `internal/eval/scorer/deepeval_bridge.go`.
- Routing goes through the SPEC-LLM-001 LiteLLM router.
- The report records the active judge model identifier.

Maps to scenario §5.15 in spec.md.

---

### AC-016 — Local CLI eval invokes same runner with subset filter

Covers: REQ-EVAL1-011

**Given** the SPEC-CLI-002 Phase 8 `usearch eval` subcommand wired to the Go runner.

**When** the operator runs:
```
usearch eval --queries=EVAL-001-Q001..Q005
```

**Then**:
- Only the 5 specified queries execute.
- The same runner code path as CI is invoked (no divergence).
- The report is printed to stdout in the same markdown format.
- The CLI exits with the same exit code mapping (0=pass, 1=score fail, 2=judge unavailable, 3=parse error).
- `--queries=category=korean` filters to the 15 Korean queries.

Maps to scenario §5.16 in spec.md.

---

## 2. Edge Cases

### EC-001 — Workflow skipped on docs-only PR

**Given** a PR modifying only files matching `**.md` or `**/docs/**` (e.g., README change).

**When** `.github/workflows/eval.yml` evaluates path filters.

**Then**:
- The workflow does NOT trigger.
- No benchmark cost is incurred for documentation-only changes.

### EC-002 — Malformed run report fails gate with exit 3

**Given** a corrupted or truncated run report JSON.

**When** the CI gate consumes it.

**Then**:
- Exit code is `3` (malformed input).
- stderr contains the parse error message and the line / byte offset.
- Distinguishable from exit 1 (score fail) and exit 2 (judge unavailable).

### EC-003 — Override expires mid-run (race condition)

**Given** an override with `expires_at: 2026-05-26T03:30:00Z` while the nightly cron runs at 03:00:00 and the query is processed at 03:31:00.

**When** the runner pre-check filters expired overrides.

**Then**:
- The runner uses a single snapshot of `overrides.json` at start (filters by start-of-run timestamp).
- Subsequent in-flight expiry does NOT alter the active override set for the current run.
- The next run picks up the new (expired-removed) state.

---

## 3. Definition of Done Checklist

- [ ] All 16 AC scenarios pass on CI.
- [ ] All 16 scenario index entries (§5.1..§5.16) in spec.md are implemented as automated tests.
- [ ] `internal/eval/golden/queries.jsonl` contains 50 records (35 EN + 15 KO).
- [ ] `internal/eval/golden/corpus/*.json` contains ≥ 200 docs.
- [ ] `internal/eval/golden/overrides.json` schema validates; cap of 5 enforced.
- [ ] `services/researcher/src/researcher/eval_judge.py` `/judge/faithfulness` endpoint live with deterministic params.
- [ ] `internal/eval/scorer/deepeval_bridge.go` returns per-claim rationale + 30s timeout enforced.
- [ ] `internal/eval/ci/gate.go` exit code mapping verified (0/1/2/3).
- [ ] `.github/workflows/eval.yml` runs on the configured path filters + skips on docs-only.
- [ ] Nightly cron writes `.moai/eval/history/EVAL-001-YYYY-MM-DD.json` + updates `latest.md`.
- [ ] Determinism re-runs: variance ≤ 0.02 across 3 consecutive runs.
- [ ] Cost cap ≤ $0.50 per run verified.
- [ ] Runtime ≤ 15 min on standard runner verified.
- [ ] Provider swap via `EVAL_JUDGE_MODEL` verified (Haiku 4.5 → gpt-4o-mini).
- [ ] `usearch eval` CLI subcommand delegated to SPEC-CLI-002 Phase 8 and shares the runner.
- [ ] Open Questions in spec.md §8 are resolved or explicitly deferred with mitigation.

---

## 4. Coverage Matrix (REQ → AC)

| REQ / NFR | AC-001 | AC-002 | AC-003 | AC-004 | AC-005 | AC-006 | AC-007 | AC-008 | AC-009 | AC-010 | AC-011 | AC-012 | AC-013 | AC-014 | AC-015 | AC-016 | EC |
|-----------|--------|--------|--------|--------|--------|--------|--------|--------|--------|--------|--------|--------|--------|--------|--------|--------|----|
| REQ-EVAL1-001 | ✓ |   |   | ✓ |   |   |   |   |   |   |   |   |   |   |   |   |   |
| REQ-EVAL1-002 | ✓ |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   |
| REQ-EVAL1-003 |   |   |   |   |   |   |   |   | ✓ | ✓ |   |   |   |   |   |   | EC-003 |
| REQ-EVAL1-004 |   | ✓ |   | ✓ |   |   |   |   |   |   |   |   |   |   |   |   |   |
| REQ-EVAL1-005 |   | ✓ | ✓ |   |   |   |   |   |   |   |   |   |   |   |   |   |   |
| REQ-EVAL1-006 |   |   |   |   |   |   |   | ✓ |   |   |   |   |   |   |   |   |   |
| REQ-EVAL1-007 |   |   | ✓ |   |   | ✓ |   |   |   |   |   |   |   |   |   |   |   |
| REQ-EVAL1-008 |   |   |   |   | ✓ | ✓ | ✓ |   |   |   |   |   |   |   |   |   | EC-002 |
| REQ-EVAL1-009 |   |   |   |   | ✓ |   |   |   |   |   |   |   |   |   |   |   | EC-001 |
| REQ-EVAL1-010 |   |   |   |   |   |   |   |   |   |   | ✓ |   |   |   |   |   |   |
| REQ-EVAL1-011 |   |   |   |   |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |
| NFR-EVAL1-001 |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |   |   |   |   |
| NFR-EVAL1-002 |   |   |   |   |   |   |   | ✓ |   |   |   |   |   |   |   |   |   |
| NFR-EVAL1-003 |   |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |   |   |   |
| NFR-EVAL1-004 |   |   |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |   |   |
| NFR-EVAL1-005 |   |   |   |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |   |

Every REQ and NFR has ≥ 1 AC; edge cases EC-001..EC-003 supplement workflow path filters, gate input handling, and override expiry race conditions.

---

*End of SPEC-EVAL-001 acceptance.md.*
