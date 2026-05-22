---
id: SPEC-EVAL-001
version: 0.1.0
status: draft
created: 2026-05-22
updated: 2026-05-22
author: limbowl
priority: P1
issue_number: 0
title: Citation faithfulness benchmark — 50-query golden set, DeepEval scorer, CI gate at ≥0.85
milestone: M8 — Eval + polish
owner: expert-testing
methodology: tdd
coverage_target: 85
depends_on: [SPEC-SYN-002, SPEC-OBS-001, SPEC-CLI-002]
blocks: [SPEC-REL-001]
related: [SPEC-EVAL-002, SPEC-EVAL-003]
---

# SPEC-EVAL-001: Citation faithfulness benchmark — 50-query golden set + DeepEval CI gate

## HISTORY

- 2026-05-22 (initial draft v0.1.0, limbowl via manager-spec):
  First EARS-formatted SPEC for the M8 evaluation gate that closes
  the citation-faithfulness loop opened by SPEC-SYN-002. SPEC-SYN-002
  delivered **structural** faithfulness (every sentence carries a
  valid `[N]` marker) inline at the Python sidecar
  (`services/researcher/src/researcher/faithfulness.py` —
  implemented 2026-05-09). SPEC-EVAL-001 closes the **semantic**
  half — does the cited document actually *support* the claim — via
  an out-of-band benchmark suite that runs against a frozen
  50-query golden set, scores each response with DeepEval's
  `FaithfulnessMetric`, and gates merges in CI at an aggregate
  ≥0.85 mean score per the M8 exit criterion in
  `.moai/project/roadmap.md` §5 ("DeepEval CI gate at ≥0.85").

  Pinned decisions (D1..D9):
  (D1) **Scorer framework: DeepEval** (confident-ai/deepeval) chosen
       over RAGAS, TruLens, and Promptfoo. DeepEval's
       `FaithfulnessMetric` ships a reference implementation of the
       "claim-level entailment against retrieved context" definition
       that exactly matches SPEC-SYN-002's contract — every sentence
       must be supported by the cited `doc_id`'s body text. RAGAS
       was the runner-up; rejection rationale: heavier numpy/pandas
       dependency stack (~120 MB) blocks fast CI cold starts, and
       its public API is less stable across 0.x → 1.x. See
       research.md §1.
  (D2) **Golden set size: 50 queries** per roadmap line 103
       (`50-query golden set`). Composition fixed at **35 English +
       15 Korean** for V1 — Korean coverage is deliberately
       under-weighted because SPEC-EVAL-003 (M8) owns the
       Korean-first benchmark with its own scoring protocol.
       Splitting now avoids double-counting Korean coverage between
       the two evals. See research.md §3.1.
  (D3) **Faithfulness metric definition: claim-level entailment**
       per DeepEval's default. A "claim" is a sentence segmented by
       SPEC-SYN-002's regex (`[.!?。！？]\s+|[.!?。！？]$`). The metric
       returns `score ∈ [0, 1]` per query, computed as
       `(supported_claims / total_claims)`. A claim is **supported**
       iff the LLM judge confirms the cited `doc_id`'s body text
       entails the claim. The aggregate benchmark score is the
       arithmetic mean across all 50 queries. See research.md §2.
  (D4) **LLM judge: Claude Haiku 4.5 via LiteLLM** (the same router
       SPEC-LLM-001 already wires for synthesis cost-optimization).
       Haiku is the cost-quality sweet spot for binary entailment
       judgments — research.md §5.1 cost analysis shows ≤ $0.45 per
       full 50-query CI run at Haiku 4.5 pricing (2026-05 rates).
       GPT-4o-mini is the documented fallback if Haiku is
       unavailable; Sonnet judge is reserved for monthly
       calibration runs (not per-PR CI). See research.md §5.
  (D5) **CI gate threshold: aggregate mean ≥ 0.85** per roadmap M8
       exit criterion. Per-query floor is `≥ 0.50` (no query may
       score below 0.50 even if the mean clears 0.85) — this
       prevents a few perfect queries from masking systematically
       broken behaviour on a small subset. Both thresholds are
       FROZEN at the SPEC level; they may be tightened (raised) by
       a follow-up SPEC but never lowered without a constitution
       amendment. See research.md §4.
  (D6) **CI integration: dedicated GitHub Actions workflow**
       (`.github/workflows/eval.yml`) gated on `pull_request` events
       touching `internal/synthesis/**`, `services/researcher/**`,
       `internal/llm/**`, `internal/fanout/**`, or the golden set
       itself. The workflow runs against a mocked adapter pool with
       a frozen document corpus (no live network calls during CI),
       so determinism is preserved and the benchmark is reproducible
       across runs. Inline-in-`go.yml` was rejected because the
       eval workflow needs the Python sidecar stack
       (deepeval + LiteLLM), which inflates Go CI cold-start time.
       See research.md §6.1.
  (D7) **Determinism strategy: ±0.02 tolerance across re-runs** on
       the same judge model. Haiku 4.5 entailment judgments are
       non-deterministic by default (temperature default = 1.0); the
       SPEC pins `temperature=0`, `top_p=1`, and a fixed
       `seed=42` per query. Empirical re-runs MUST stay within
       ±0.02 of the recorded baseline; outliers trigger a
       "calibration drift" alert but do not block CI. See
       research.md §2.3.
  (D8) **False-positive escape hatch**: every query in the golden
       set carries a `manual_override` field. Setting
       `manual_override: pass` (with mandatory `override_reason`
       text) marks a query as known-flaky and excludes it from the
       aggregate score. Maximum 5 overrides allowed simultaneously;
       overrides expire after 30 days unless re-confirmed. Override
       history is committed to git for audit. This prevents
       judge-model false negatives from blocking unrelated PRs.
       See research.md §2.4.
  (D9) **Out-of-band nightly regression run**: in addition to the
       PR-gating CI, a nightly cron runs the full benchmark and
       writes the result to `.moai/eval/history/EVAL-001-{date}.json`.
       This populates the regression baseline for the
       `evaluator-active` Mechanism 2 (regression baseline) per
       `.claude/rules/moai/design/constitution.md` §12. Score
       drops > 0.05 day-over-day trigger a Slack alert (operator
       contact list owned by SPEC-OBS-001). See research.md §6.3.

  M8 release gate: this SPEC is the **last gate** between M7
  surface completion and M9 V1.0.0 tag — `.moai/project/roadmap.md`
  §5 explicitly names "DeepEval CI gate at ≥0.85" as the M8 exit
  criterion. SPEC-EVAL-001 thus blocks SPEC-REL-001 (V1 tag); a
  failing benchmark blocks the release.

  Companion artifacts:
  - `.moai/specs/SPEC-EVAL-001/research.md` — Phase 0.5 research
    (framework comparison, scoring methodology, golden set
    construction, CI patterns, cost analysis, references)
  - `.moai/specs/SPEC-EVAL-001/plan.md` — phased implementation
    plan (TDD methodology, 6 phases, no time estimates)

  11 EARS REQs (4 × P0 + 5 × P1 + 2 × P2) + 5 NFRs covering golden
  set schema, DeepEval scorer wiring, judge model abstraction, CI
  gate, nightly regression, escape hatch, and report artifact.
  Methodology: TDD per `.moai/config/sections/quality.yaml`
  `development_mode: tdd`. Coverage target 85%. Harness: standard
  (Sprint Contract recommended — judge prompt stability is a
  cross-iteration concern). Owner: expert-testing.

---

## 1. Overview

SPEC-EVAL-001 is the **acceptance gate** for the citation
faithfulness contract introduced by SPEC-SYN-002 and the broader
M4 synthesis hardening. It answers a single question on every
PR that touches the synthesis path:

> Does the system's synthesized output still faithfully cite its
> sources, at the quality bar promised by M4 (≥ 0.85 aggregate
> faithfulness on the golden set)?

The benchmark is **out-of-band**: it does not modify production
code paths, it does not add latency to user queries, it does not
interact with live adapter rate limits. It exercises the
already-shipped citation enforcement path (SPEC-SYN-002) end-to-end
against a frozen corpus and scores each response with an LLM judge.

### 1.1 Why this SPEC exists separately from SPEC-SYN-002

SPEC-SYN-002 implemented the **structural** half of citation
faithfulness — every sentence carries a `[N]` marker; markers
resolve to a real input doc; un-cited sentences are stripped or
the response is rejected. That gate runs inline at synthesis time
and is fast enough (< 50ms p99 per request per NFR-SYN2-001) to
sit in the request hot path.

What SPEC-SYN-002 cannot do inline:

- **Semantic verification**: Does the cited document's body text
  actually *support* the synthesized claim? A response like
  "The author argues that quantum supremacy was achieved in 2019
  [1]" is structurally faithful even if `[1]` is a paper on
  classical computing — the structural gate cannot tell. Semantic
  verification requires reading both the claim and the source's
  full body, which is an LLM-grade reasoning task too expensive
  to run inline per request.
- **Aggregate regression detection**: A single faithfulness
  failure rate jumping from 5% to 20% over a release window is a
  P0 quality regression, but no single request can detect it.
  Detection requires a fixed test corpus run repeatedly.

SPEC-EVAL-001 is the out-of-band complement that catches both.

### 1.2 What ships

A complete benchmark harness consisting of:

```
internal/eval/
├── golden/
│   ├── queries.jsonl                 [50 queries: 35 EN + 15 KO]
│   ├── corpus/                       [frozen NormalizedDoc fixture corpus]
│   │   ├── doc-001.json
│   │   ├── doc-002.json
│   │   └── ...
│   └── overrides.json                [manual_override registry, ≤5 active]
├── scorer/
│   ├── deepeval_bridge.go            [Go → Python deepeval HTTP bridge]
│   └── deepeval_bridge_test.go
├── runner/
│   ├── runner.go                     [orchestrates: query → synth → judge → score]
│   ├── runner_test.go
│   └── report.go                     [aggregate report writer]
├── ci/
│   ├── gate.go                       [CI exit code logic: ≥0.85 → 0, else 1]
│   └── gate_test.go
└── eval.go                           [stub replaced by real package]

services/researcher/src/researcher/
└── eval_judge.py                     [DeepEval FaithfulnessMetric wrapper]

.github/workflows/
└── eval.yml                          [CI workflow]

.moai/eval/
├── history/
│   └── EVAL-001-2026-MM-DD.json     [nightly regression history]
└── reports/
    └── latest.md                     [most recent run report]
```

The Go runner orchestrates the query path (intent router → fanout
→ synthesis), captures the synthesized response, ships each
(claim, cited_doc_body) pair to the Python `eval_judge.py` service,
collects scores, aggregates per query and across the suite, and
exits 0 / 1 per the CI gate threshold.

### 1.3 What does NOT ship (delegated)

- **Adapter reliability scoring**: SPEC-EVAL-002 (sibling, M8) owns
  per-adapter success rate dashboards.
- **Korean-locale benchmark**: SPEC-EVAL-003 (sibling, M8) owns the
  50-query Korean-first eval with manual scoring protocol.
  SPEC-EVAL-001's 15 Korean queries are a *baseline* check, not the
  full Korean evaluation.
- **Live network calls in CI**: all CI runs use the frozen corpus
  fixture; live adapter exercise is the responsibility of
  SPEC-EVAL-002 dashboards (off-line of CI).
- **Inline per-request faithfulness scoring**: that is SPEC-SYN-002's
  job (structural) and remains so. EVAL-001 is out-of-band.

### 1.4 Forward-compatibility commitments

- **SPEC-SYN-002**: EVAL-001 consumes SPEC-SYN-002's `Citation`
  schema (`pkg/types/normalized_doc.go` — already declared) and
  the `SynthesizeResponse` shape unchanged. The benchmark
  exercises the already-shipped faithfulness path including the
  retry-once policy.
- **SPEC-OBS-001**: EVAL-001 emits two new Prometheus metrics
  (`usearch_eval_runs_total`, `usearch_eval_score_gauge`) following
  the OBS-001 cardinality allowlist discipline (no new
  high-cardinality labels).
- **SPEC-LLM-001**: the judge model is invoked via the
  SPEC-LLM-001 LiteLLM router; the SPEC does not introduce a
  parallel LLM client.
- **SPEC-CLI-002**: a `usearch eval` subcommand (deferred to
  Phase 6 of the plan) lets developers run the benchmark locally
  without GitHub Actions. The SPEC defines the contract; CLI-002
  Phase 8 wires the subcommand surface.
- **SPEC-REL-001** (M9): EVAL-001 blocks REL-001. The V1.0.0 tag
  CANNOT ship until the benchmark passes on `main` branch.

---

## 2. EARS Requirements

### 2.1 Golden Set Module

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-EVAL1-001** | Ubiquitous | The system SHALL maintain a versioned golden set at `internal/eval/golden/queries.jsonl` containing exactly 50 query records. Each record SHALL be a single-line JSON object with required fields `id` (string, format `EVAL-001-Q{NNN}`), `query` (string, user-facing query text), `locale` (string ∈ `{"en", "ko"}`), `expected_sources` (string[], optional — `doc_id`s the response SHOULD cite), `category` (string ∈ `{"factual", "comparison", "synthesis", "korean", "edge"}`), `notes` (string, optional). The 50 queries SHALL be partitioned as 35 `locale: "en"` + 15 `locale: "ko"` (per HISTORY D2). The file SHALL be append-only between releases — query records can be added (with a follow-up SPEC) but never silently rewritten. Schema changes require a SPEC amendment. | P0 | `TestGoldenSetSchema` — parse every line, assert all required fields present and locale partition matches 35/15; `TestGoldenSetCount` asserts exactly 50 records. |
| **REQ-EVAL1-002** | Ubiquitous | The system SHALL maintain a frozen document corpus at `internal/eval/golden/corpus/*.json` containing the `NormalizedDoc` fixtures the benchmark's mocked adapter pool returns. Each fixture file SHALL deserialize cleanly into `pkg/types.NormalizedDoc` per its existing schema. The corpus SHALL contain at minimum 200 distinct docs to ensure realistic fanout sizes; per-query expected_sources MUST be subsets of the corpus's `doc_id` set. The corpus SHALL be reproducibility-pinned: any addition or modification commits the new fixture to git and bumps a `corpus_revision` field in `.moai/eval/golden/manifest.json`. | P0 | `TestCorpusDeserializes` (every fixture parses); `TestCorpusSize ≥ 200`; `TestExpectedSourcesResolveToCorpus` (all `expected_sources` are valid `doc_id`s in the corpus). |
| **REQ-EVAL1-003** | Optional | WHERE a query exhibits a known judge-model false-positive pattern (LLM judge mis-scores a faithful response as un-faithful), the operator MAY add an entry to `internal/eval/golden/overrides.json` with required fields `query_id` (must match REQ-EVAL1-001 `id`), `manual_override` (∈ `{"pass", "skip"}`), `override_reason` (string, non-empty), `expires_at` (ISO-8601 date, ≤ 30 days from `created_at`), `created_at` (ISO-8601 timestamp), `created_by` (GitHub handle). The total number of active (non-expired) overrides SHALL NOT exceed 5 at any time (per HISTORY D8); CI SHALL fail if this cap is exceeded. Expired overrides SHALL be auto-removed by the runner before scoring. | P1 | `TestOverridesSchemaValid`; `TestOverridesCapEnforced` (>5 active triggers CI failure); `TestExpiredOverridesAutoRemoved`. |

### 2.2 Scorer & Judge Module

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-EVAL1-004** | Ubiquitous | The system SHALL expose a Python service `services/researcher/src/researcher/eval_judge.py` providing a single HTTP POST endpoint `/judge/faithfulness` that accepts a JSON body `{query_id, claims: [{text, cited_doc_ids: string[]}], corpus: {doc_id: doc_body_text}}` and returns `{query_id, claim_scores: [{text, supported: bool, judge_rationale: string}], faithfulness_score: float ∈ [0, 1], total_claims: int, supported_claims: int}`. The endpoint SHALL wrap DeepEval's `FaithfulnessMetric` (per HISTORY D1) configured with judge model from `EVAL_JUDGE_MODEL` env var (default `claude-haiku-4-5` per HISTORY D4). The endpoint SHALL pass `temperature=0, top_p=1, seed=42` through LiteLLM to enforce determinism per HISTORY D7. | P0 | `test_judge_endpoint_returns_per_claim_scores`; `test_judge_uses_deterministic_params`; `test_judge_score_formula` (supported_claims/total_claims). |
| **REQ-EVAL1-005** | Ubiquitous | The system SHALL expose a Go bridge `internal/eval/scorer/deepeval_bridge.go` that calls the Python `eval_judge` endpoint via HTTP, marshalling the synthesis response into the judge's expected schema. The bridge SHALL: (a) split the synthesis output into claims using the SPEC-SYN-002 sentence regex (`[.!?。！？]\s+|[.!?。！？]$`); (b) extract `cited_doc_ids` per claim from the trailing `[N]` markers and the response's `Citations` array; (c) build the `corpus` map by reading the docs the fanout returned (NOT the full golden corpus — only what was actually retrieved for this query); (d) POST to `/judge/faithfulness`; (e) return `{score, per_claim, judge_rationales}` to the runner. The bridge SHALL respect a 30s per-query timeout (NFR-EVAL1-002). | P0 | `TestBridgeMarshalsClaims`; `TestBridgeExtractsCitations`; `TestBridgeTimeoutEnforced`; `TestBridgeReturnsPerClaimRationale`. |
| **REQ-EVAL1-006** | State-Driven | WHILE the judge model is unavailable (HTTP 5xx, connection refused, or timeout > 30s), the runner SHALL: (a) log an ERROR-level record with `{query_id, judge_model, error_class}`; (b) mark the query's score as `null` (NOT zero — null preserves the distinction between "judge could not score" and "judge confidently scored zero"); (c) continue with the next query (no fail-fast); (d) on benchmark completion, if any query has a `null` score, the runner SHALL exit with code 2 (judge availability error) — distinct from code 1 (score below threshold) and code 0 (pass). The aggregate mean SHALL be computed over non-null scores only; the count of null scores SHALL be reported in the run summary. | P1 | `TestJudgeUnavailableMarksNullNotZero`; `TestRunnerExitCode2OnJudgeError`; `TestAggregateMeanExcludesNullScores`. |
| **REQ-EVAL1-007** | Event-Driven | WHEN the judge model returns a per-claim `supported: false` verdict on a structurally faithful claim (i.e. the claim has a `[N]` marker that SPEC-SYN-002 accepted), the runner SHALL record the full judge rationale text in the per-query report record. The report SHALL surface the top 10 lowest-scoring queries with their judge rationales in a `## Lowest-Scoring Queries` section of the markdown report. This is the operator's primary debugging surface — un-explained low scores must never appear in the report. | P1 | `TestReportContainsJudgeRationalesForLowScores`; `TestReportTopTenLowestQueriesSection`. |

### 2.3 CI Gate Module

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-EVAL1-008** | Ubiquitous | The system SHALL provide a CI gate at `internal/eval/ci/gate.go` that consumes the runner's report and exits with code `0` iff (a) aggregate mean score ≥ 0.85 AND (b) every individual non-null query score ≥ 0.50 AND (c) no judge-availability errors occurred AND (d) active overrides count ≤ 5. Exit code mapping: `0` = pass, `1` = score below threshold (a, b, or d failed), `2` = judge availability error (c failed), `3` = malformed input (report parse error). The gate SHALL print a one-line summary to stdout matching `EVAL-001 result=PASS|FAIL mean=<X.XXX> floor=<X.XX> overrides=<N> nulls=<N>` for grep-friendly CI log parsing. | P0 | `TestGatePassesAt085MeanAnd050Floor`; `TestGateFailsBelowMean`; `TestGateFailsBelowFloor`; `TestGateExitCodeMapping`; `TestGateStdoutSummaryFormat`. |
| **REQ-EVAL1-009** | Event-Driven | WHEN a PR touches any file matching `internal/synthesis/**`, `services/researcher/**`, `internal/llm/**`, `internal/fanout/**`, `internal/eval/**`, or `.moai/specs/SPEC-EVAL-001/**`, the `.github/workflows/eval.yml` GitHub Actions workflow SHALL run the full 50-query benchmark and gate the PR on the result via REQ-EVAL1-008. The workflow SHALL use the frozen corpus (no live adapter calls) and SHALL post the report markdown as a PR comment (overwriting any prior EVAL-001 comment from the same PR). The workflow SHALL be skipped for documentation-only PRs (paths matching `**.md`, `**/docs/**`) per the path-filters config. | P1 | `TestWorkflowTriggersOnSynthesisChange`; `TestWorkflowSkipsOnDocsOnly`; manual `ManualPRCommentRendersReport`. |
| **REQ-EVAL1-010** | Ubiquitous | The system SHALL run the full benchmark as a nightly cron job at 03:00 UTC and write the result to `.moai/eval/history/EVAL-001-{YYYY-MM-DD}.json`. The historical file SHALL include `{date, commit_sha, branch, mean_score, per_query_scores, judge_model, override_count, null_count, runtime_seconds}`. The runner SHALL also update `.moai/eval/reports/latest.md` with the most recent human-readable report. The nightly run SHALL NOT gate any merge — its purpose is regression baseline establishment per HISTORY D9. | P1 | `TestNightlyHistoryFileSchema`; `TestNightlyDoesNotGateMerge`; `TestNightlyUpdatesLatestReport`. |
| **REQ-EVAL1-011** | Optional | WHERE the operator wants to run the benchmark locally, a `usearch eval [--queries=<id-pattern>]` CLI subcommand (delegated to SPEC-CLI-002 Phase 8) SHALL invoke the same Go runner used by CI and print the report to stdout. The `--queries` flag MAY filter to a subset (e.g., `EVAL-001-Q001..Q010` or `category=korean`). The CLI SHALL exit with the same code mapping as the CI gate (REQ-EVAL1-008). | P2 | `TestCLIInvokesSameRunner`; `TestCLIQueryFilterWorks`; manual `ManualLocalEvalRun`. |

---

## 3. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| **NFR-EVAL1-001** | Determinism — re-run score variance | Re-running the benchmark with the same `EVAL_JUDGE_MODEL`, the same code revision, and the same corpus revision SHALL produce an aggregate mean score within ±0.02 of the prior run's score, computed across at least 3 consecutive re-runs (Mechanism 4 calibration per `.claude/rules/moai/design/constitution.md` §12). Variance above ±0.02 SHALL trigger a "calibration drift" warning in the report but SHALL NOT block CI. Variance above ±0.05 SHALL block CI (exit 2) until the calibration is re-stabilised. The deterministic params (`temperature=0, top_p=1, seed=42`) are FROZEN at the SPEC level. |
| **NFR-EVAL1-002** | Per-query judge timeout | The judge call for any single query SHALL complete within 30 seconds wall-clock (synthesis time + bridge HTTP + DeepEval scoring). Exceeding this triggers the REQ-EVAL1-006 unavailability path. The 30s budget allows: synthesis path ≤ 8s (SPEC-SYN-001 NFR-SYN-001 p50) + bridge marshalling ≤ 500ms + deepeval call ≤ 20s + buffer. |
| **NFR-EVAL1-003** | Cost bound per CI run | A complete 50-query benchmark run SHALL cost ≤ $0.50 USD in LLM judge API spend at current Claude Haiku 4.5 pricing (2026-05 rates per research.md §5.1: ~$0.008 per query × 50 + retry buffer). Cost SHALL be reported in the run report. Monthly CI cost cap is informational: assuming ~100 PRs/month + 30 nightly runs = $0.50 × 130 ≈ $65/month — well within the project budget. Cost overruns above $1.00 per run trigger an alert and require human review. |
| **NFR-EVAL1-004** | Runtime budget per CI run | A complete 50-query benchmark run SHALL complete within 15 minutes wall-clock on a standard GitHub Actions runner (4 vCPU, 16 GB RAM). The runner SHALL parallelize across at most 5 concurrent queries (bounded by judge model rate limits and SPEC-SYN-001 sidecar concurrency). Exceeding 15 minutes triggers a runtime warning; exceeding 25 minutes fails the CI job (exit 124 by GitHub Actions timeout). |
| **NFR-EVAL1-005** | LLM provider abstraction (LiteLLM) | The judge model SHALL be invoked exclusively via the SPEC-LLM-001 LiteLLM router. The judge model identifier SHALL be a LiteLLM model string (e.g., `claude-haiku-4-5`, `gpt-4o-mini`). Switching judge providers SHALL require only an `EVAL_JUDGE_MODEL` env var change — no code changes in `eval_judge.py` or the Go bridge. This preserves the SPEC-LLM-001 contract that no client outside the LLM package may instantiate a direct provider SDK. |

---

## 4. Exclusions (What NOT to Build)

[HARD] This SPEC explicitly excludes the following items. Each has a
known destination SPEC, rationale, or follow-up; this list prevents
scope creep into EVAL-001.

- **Korean-locale full benchmark suite** — only 15 Korean queries
  baseline-checked here. Full Korean evaluation with manual scoring
  protocol → SPEC-EVAL-003 (M8). Splitting Korean coverage prevents
  EVAL-001 from triple-counting the Korean dimension when
  SPEC-EVAL-003 ships.

- **Adapter-level reliability scoring** (per-adapter success rate,
  uptime, latency dashboards). → SPEC-EVAL-002 (M8). EVAL-001
  exercises the synthesis path on a frozen corpus and does NOT
  measure live adapter behaviour.

- **Live network adapter calls during CI** — the benchmark uses a
  mocked adapter pool returning the frozen `internal/eval/golden/
  corpus/` fixtures. Live adapter exercise belongs to SPEC-EVAL-002
  off-line dashboards and to integration tests outside CI.

- **Inline per-request faithfulness scoring** — that is SPEC-SYN-002's
  job (structural enforcement in the request hot path) and remains
  inline. EVAL-001 is strictly out-of-band benchmark, never in the
  user-query path.

- **Semantic faithfulness scoring beyond claim-level entailment** —
  e.g., factual accuracy of the cited doc itself (a doc may
  faithfully support a claim while itself being wrong), or
  multi-claim-per-sentence detection. DeepEval `FaithfulnessMetric`
  scores claim-level entailment; deeper semantic checks are reserved
  for future SPECs if measured value emerges.

- **Custom DeepEval metric authoring** — V1 uses DeepEval's stock
  `FaithfulnessMetric` unmodified. Custom metrics
  (`AnswerRelevancyMetric`, `ContextualPrecisionMetric`, etc.) are
  deferred. If the operator wants additional dimensions, a sibling
  SPEC (e.g., SPEC-EVAL-004 for answer relevancy) is the path —
  EVAL-001 does not bloat its scope to host all DeepEval metrics.

- **A/B testing of multiple judge models in CI** — V1 pins a single
  judge model (Haiku 4.5 default). Calibration runs against Sonnet
  are monthly and out-of-band. Per-PR multi-judge ensemble is too
  expensive (NFR-EVAL1-003 cap) and out of scope.

- **Golden set generation pipeline** — V1 golden set is hand-curated
  at SPEC author time (50 queries authored as part of Phase 2 of
  the plan, committed once). Auto-generation of golden queries from
  production logs is a future enhancement requiring its own privacy
  + selection-bias analysis.

- **Cross-team or production query replay** — the corpus is frozen
  fixtures; no real user queries appear in the golden set. Privacy
  + reproducibility constraints make production replay out of
  scope for V1.

- **Faithfulness scoring for `/deep` (STORM-style) reports** — V1
  benchmark targets the basic-synthesis path (SPEC-SYN-001 +
  SPEC-SYN-002 + SPEC-SYN-003 + SPEC-SYN-004). Long-form `/deep`
  output (SPEC-DEEP-001..004) has different acceptance characteristics
  and gets its own benchmark SPEC when M8 polish for `/deep`
  becomes a priority.

- **Per-language judge model differentiation** — V1 uses the same
  Haiku judge for English + Korean queries. If Korean entailment
  scoring proves systematically biased (research.md §10 Q4 open
  question), a follow-up SPEC may introduce a per-locale judge
  override.

- **Schema-breaking changes to `Citation`, `NormalizedDoc`, or
  `SynthesizeResponse`** — V1 reuses the shapes SPEC-SYN-002 and
  SPEC-CORE-001 already established. The benchmark consumes these
  contracts read-only.

- **Score persistence in a database** — V1 stores history as flat
  JSON in `.moai/eval/history/`. Migration to a queryable backing
  store (Postgres, ClickHouse) is deferred — flat JSON suffices
  for the modest write volume (≤ 1/day cron + per-PR ephemeral).

- **Slack / PagerDuty alert wiring** — the SPEC defines the alert
  trigger conditions (NFR-EVAL1-001 ±0.05 variance,
  NFR-EVAL1-003 cost overrun, day-over-day drop > 0.05 per
  HISTORY D9). Actual webhook delivery is operator-owned and
  configured outside this SPEC (SPEC-OBS-001 alerting layer).

- **GitHub Issue tracking on this SPEC** (`issue_number: 0`). →
  Per project pattern for M8 polish-band SPECs.

- **Custom golden set authoring UI** — queries are authored by
  hand-editing `queries.jsonl`. A guided UI for golden set
  curation is a far-future productivity feature, not V1.

- **Integration with `evaluator-active` agent scoring** — the
  `evaluator-active` agent (per `.claude/rules/moai/design/
  constitution.md` §11-12) scores design artifacts; EVAL-001
  scores synthesis output. The two systems are intentionally
  separate; EVAL-001 does NOT register itself as a contributor to
  evaluator-active rubrics.

---

## 5. Acceptance Criteria

Per-REQ acceptance summaries are documented in §2 alongside each
requirement. The full Given-When-Then scenarios are owned by an
upcoming `.moai/specs/SPEC-EVAL-001/acceptance.md` (authored
alongside the plan-auditor cycle). The scenario index:

| Scenario | Description | Coverage |
|----------|-------------|----------|
| §5.1 | Golden set load: runner reads `queries.jsonl`, parses 50 records, partitions 35 EN + 15 KO; corpus loads ≥ 200 docs; `expected_sources` resolve. | REQ-EVAL1-001, REQ-EVAL1-002 |
| §5.2 | Single-query happy path: query Q001 (factual EN) runs through synthesis with frozen corpus, judge scores 3/3 claims supported, query score = 1.0, recorded in report. | REQ-EVAL1-004, REQ-EVAL1-005 |
| §5.3 | Single-query partial faithfulness: query Q010 (synthesis EN) returns 4 claims; judge marks 3 supported, 1 unsupported; query score = 0.75; judge rationale captured for the unsupported claim. | REQ-EVAL1-005, REQ-EVAL1-007 |
| §5.4 | Korean query: query Q036 (`locale: "ko"`) routes through Korean adapters (mocked); synthesis returns Korean text; judge scores in Korean entailment context; score recorded. | REQ-EVAL1-001 (locale partition), REQ-EVAL1-004 |
| §5.5 | Aggregate pass: all 50 queries score; mean ≥ 0.85, no individual < 0.50; CI gate exit 0; PR comment posted with summary. | REQ-EVAL1-008, REQ-EVAL1-009 |
| §5.6 | Aggregate fail (mean): mean = 0.82 < 0.85; gate exits 1; PR comment shows top 10 lowest queries with rationales. | REQ-EVAL1-008, REQ-EVAL1-007 |
| §5.7 | Aggregate fail (floor): mean = 0.87 but Q017 scores 0.40 < 0.50; gate exits 1 with reason "floor violation". | REQ-EVAL1-008 |
| §5.8 | Judge unavailable: deepeval HTTP returns 503 on 3 queries; runner marks scores null; gate exits 2; report shows 3 null scores. | REQ-EVAL1-006, NFR-EVAL1-002 |
| §5.9 | Override applied: Q023 has active override (`pass`, expires 2026-06-20); runner excludes Q023 from aggregate; report logs override usage. | REQ-EVAL1-003 |
| §5.10 | Override cap exceeded: overrides.json has 6 active entries; runner pre-check fails; exit 1 with reason "override cap exceeded". | REQ-EVAL1-003 |
| §5.11 | Nightly cron run: scheduled workflow fires at 03:00 UTC; writes `.moai/eval/history/EVAL-001-2026-MM-DD.json`; does not gate any merge; updates `latest.md`. | REQ-EVAL1-010 |
| §5.12 | Determinism re-run: 3 consecutive runs on same revision; variance ≤ 0.02; passes NFR-EVAL1-001. | NFR-EVAL1-001 |
| §5.13 | Cost report: run summary includes total LLM judge cost ≤ $0.50; per-query cost breakdown available in report. | NFR-EVAL1-003 |
| §5.14 | Runtime budget: 50-query run completes in ≤ 15 min on standard GitHub runner. | NFR-EVAL1-004 |
| §5.15 | Provider swap: setting `EVAL_JUDGE_MODEL=gpt-4o-mini` succeeds without code change; benchmark runs against OpenAI judge. | NFR-EVAL1-005 |
| §5.16 | Local CLI eval: `usearch eval --queries=EVAL-001-Q001..Q005` runs 5 queries and prints report to stdout; exit code matches CI gate logic. | REQ-EVAL1-011 |

---

## 6. Dependencies & Blocks

### 6.1 Upstream SPEC dependencies (depends_on)

- **SPEC-SYN-002** (implemented 2026-05-09) — provides the
  structural citation-faithfulness path that EVAL-001 exercises.
  EVAL-001 consumes `SynthesizeResponse.text + Citations` per
  SPEC-SYN-002's contract. The benchmark would have no
  faithfulness invariant to score against without SPEC-SYN-002
  having shipped first.

- **SPEC-OBS-001** (implemented) — provides the Prometheus metrics
  endpoint the runner emits to (`usearch_eval_runs_total`,
  `usearch_eval_score_gauge`). EVAL-001 adheres to the SPEC-OBS-001
  cardinality allowlist discipline — no new label keys, only label
  values (`outcome={"pass","fail","null"}` reuses existing label
  name).

- **SPEC-CLI-002** (draft) — the `usearch eval` subcommand path
  (REQ-EVAL1-011, P2). Plan Phase 6 emits the CLI surface contract
  for CLI-002 Phase 8 to wire. Pre-CLI-002, the runner is invoked
  via `go run` or directly via the CI workflow.

### 6.2 Related but soft (related)

- **SPEC-EVAL-002 (sibling, M8)** — adapter reliability dashboard.
  Coordination: EVAL-001 uses frozen corpus; EVAL-002 uses live
  adapter exercise. The two together provide the M8 quality
  picture but are independently shippable.

- **SPEC-EVAL-003 (sibling, M8)** — Korean-locale benchmark with
  manual scoring. Coordination: EVAL-001 carries 15 Korean baseline
  queries (per HISTORY D2); EVAL-003 will carry 50 Korean queries
  with a distinct scoring protocol (manual + LLM hybrid). The two
  evals jointly cover the Korean dimension without double-counting.

### 6.3 Downstream blocked SPECs (blocks)

- **SPEC-REL-001 (M9)** — V1.0.0 release tag. Per
  `.moai/project/roadmap.md` §5 M8 exit criterion + M9 gate,
  the release tag CANNOT ship without a passing EVAL-001 benchmark
  on `main`. The CI gate is the enforcement mechanism.

### 6.4 External dependencies (run-phase pins)

- **deepeval** (Python package, `confident-ai/deepeval`) — pinned
  to ≥ 1.0.0 (latest stable as of 2026-05). Pin verified against
  the existing `services/researcher/pyproject.toml` constraint set.
- **LiteLLM** (already pinned via SPEC-LLM-001) — used for judge
  model routing.
- **Claude Haiku 4.5** — default judge model. Fallback: GPT-4o-mini
  (OpenAI). Both reachable via LiteLLM.
- **GitHub Actions runner** — `ubuntu-latest`, Python 3.12, Go 1.23
  per existing project workflow constraints.

No new runtime dependencies in the production query path
(EVAL-001 runs only in CI and the nightly cron — never per user
request).

---

## 7. Files to Create / Modify

### 7.1 Created (Go side)

| Marker | Path | Purpose |
|--------|------|---------|
| [NEW] | `internal/eval/golden/queries.jsonl` | 50-query golden set per REQ-EVAL1-001. |
| [NEW] | `internal/eval/golden/corpus/*.json` | ≥ 200 `NormalizedDoc` fixtures per REQ-EVAL1-002. |
| [NEW] | `internal/eval/golden/overrides.json` | Manual override registry per REQ-EVAL1-003. |
| [NEW] | `internal/eval/golden/manifest.json` | Corpus revision pin per REQ-EVAL1-002. |
| [NEW] | `internal/eval/scorer/deepeval_bridge.go` | Go→Python judge bridge per REQ-EVAL1-005. |
| [NEW] | `internal/eval/scorer/deepeval_bridge_test.go` | Bridge tests. |
| [NEW] | `internal/eval/runner/runner.go` | Orchestrator per REQ-EVAL1-004..007. |
| [NEW] | `internal/eval/runner/runner_test.go` | Runner tests. |
| [NEW] | `internal/eval/runner/report.go` | Report writer per REQ-EVAL1-007, REQ-EVAL1-010. |
| [NEW] | `internal/eval/runner/report_test.go` | Report tests. |
| [NEW] | `internal/eval/ci/gate.go` | CI gate logic per REQ-EVAL1-008. |
| [NEW] | `internal/eval/ci/gate_test.go` | Gate tests. |
| [NEW] | `cmd/eval/main.go` | Standalone entry point for CI workflow + local CLI. |
| [MODIFY] | `internal/eval/eval.go` | Replace stub with package doc + re-exports. |

### 7.2 Created (Python sidecar side)

| Marker | Path | Purpose |
|--------|------|---------|
| [NEW] | `services/researcher/src/researcher/eval_judge.py` | DeepEval `FaithfulnessMetric` wrapper per REQ-EVAL1-004. |
| [NEW] | `services/researcher/tests/test_eval_judge.py` | Endpoint unit tests + property tests. |
| [MODIFY] | `services/researcher/src/researcher/app.py` | Mount `/judge/faithfulness` route. |
| [MODIFY] | `services/researcher/pyproject.toml` | Add `deepeval >= 1.0.0` dependency. |

### 7.3 Created (CI + artifacts)

| Marker | Path | Purpose |
|--------|------|---------|
| [NEW] | `.github/workflows/eval.yml` | PR-gating workflow per REQ-EVAL1-009. |
| [NEW] | `.github/workflows/eval-nightly.yml` | Cron workflow per REQ-EVAL1-010. |
| [NEW] | `.moai/eval/history/.gitkeep` | History directory placeholder. |
| [NEW] | `.moai/eval/reports/.gitkeep` | Reports directory placeholder. |
| [NEW] | `.moai/eval/README.md` | Operator guide for reading reports + applying overrides. |

### 7.4 Modified

| Path | Change |
|------|--------|
| `internal/obs/metrics/metrics.go` | Register two new collectors (`usearch_eval_runs_total{outcome}`, `usearch_eval_score_gauge`). |
| `internal/obs/metrics/router_test.go` | Extend cardinality allowlist test to assert `eval` family is allowlisted. |
| `.moai/project/roadmap.md` | Update SPEC-EVAL-001 row to `status: implemented` (post-run-phase). |

### 7.5 Existing — Unchanged (read-only consumers)

- `internal/synthesis/*` — benchmark exercises the path read-only.
- `services/researcher/src/researcher/synthesis.py` — same.
- `services/researcher/src/researcher/faithfulness.py` — same.
- `pkg/types/normalized_doc.go` — schema consumed as-is.
- `internal/llm/router.go` — judge invoked via the existing router.

---

## 8. Open Questions

Restated from research.md §10 for plan-auditor convenience:

1. **Plugin repo placement of the golden set** (research.md §10 Q1) —
   monorepo `internal/eval/golden/` (recommended; co-located with
   the Go runner) vs separate `golden-sets` repo (allows
   independent versioning). Plan recommends monorepo for simplicity.

2. **Korean judge bias** (research.md §10 Q4) — open question
   whether Haiku 4.5 entailment scoring exhibits systematic bias
   against Korean responses. Validation deferred to Phase 2 of the
   plan; if bias > 0.10 between EN and KO at equivalent quality,
   open a follow-up SPEC for per-locale judge override.

3. **Override expiry policy** (HISTORY D8) — 30-day expiry chosen
   as a reasonable default. May need shortening to 14 days if
   override usage grows. _Resolution_: monitor usage in first
   3 months post-ship; tune in a follow-up patch SPEC.

4. **Per-query expected_sources usage** (REQ-EVAL1-001) — currently
   stored as documentation hint; not consumed by the scorer (since
   DeepEval scores against retrieved context, not expected). Could
   add an "expected coverage" sub-metric in V1.1 if measured value
   emerges. _Does not block plan-auditor._

5. **Cost report granularity** (NFR-EVAL1-003) — should the report
   break down cost per query, per category, or only show the
   aggregate? Plan recommends per-category breakdown + aggregate
   total. _TBD_ — annotation cycle.

6. **Nightly cron timing collision** (REQ-EVAL1-010) — 03:00 UTC
   chosen as a low-CI-contention slot. May shift if other nightly
   workflows arrive (e.g., SPEC-EVAL-002 dashboard refresh). _TBD_
   — coordinate with EVAL-002 owner.

7. **PR comment overwrite policy** (REQ-EVAL1-009) — overwrite
   prior EVAL-001 comments to avoid clutter. May want to preserve
   history for failing PRs. _TBD_ — annotation cycle.

8. **DeepEval version pin strategy** (§6.4) — `>= 1.0.0` or
   `~= 1.0` (compatible release operator). Plan recommends `~= 1.0`
   to allow patch upgrades without breaking changes. _TBD_ —
   annotation cycle.

9. **Reference handling for retrieved-but-not-cited docs**
   (REQ-EVAL1-005) — DeepEval's `FaithfulnessMetric` accepts a
   `retrieval_context` list. Plan recommends passing only the
   docs the synthesis actually cited (not the full fanout result)
   to focus the judge's scope. _TBD_ — verify against DeepEval
   docs in run phase.

These items do NOT block plan-auditor PASS; they are tagged as
known unresolved scope edges with rationale.

---

## 9. References

External (cited in research.md §11):

- DeepEval framework: https://github.com/confident-ai/deepeval
- DeepEval FaithfulnessMetric docs: https://docs.confident-ai.com/docs/metrics-faithfulness
- RAGAS faithfulness (comparison baseline): https://docs.ragas.io/en/stable/concepts/metrics/faithfulness.html
- TruLens evaluation (alternative considered): https://www.trulens.org/
- Promptfoo (alternative considered): https://www.promptfoo.dev/
- LiteLLM model routing: https://docs.litellm.ai/docs/
- GitHub Actions path filters: https://docs.github.com/en/actions/using-workflows/triggering-a-workflow#using-filters
- Claude Haiku 4.5 pricing: https://www.anthropic.com/pricing
- LLM-as-judge prompting pitfalls (Zheng et al. 2023): https://arxiv.org/abs/2306.05685

Internal (project files):

- `.moai/project/product.md` — V1 quality positioning ("auditable
  self-hosted research engine")
- `.moai/project/roadmap.md` §M8 SPEC-EVAL-001 row + §5 M8 exit
  criterion "DeepEval CI gate at ≥0.85" + §5 M9 release gate
- `.moai/project/tech.md` §1 — LiteLLM as universal LLM gateway
  (REQ-EVAL1-004 + NFR-EVAL1-005 rationale)
- `.moai/specs/SPEC-SYN-002/spec.md` — structural faithfulness
  contract this benchmark exercises
- `.moai/specs/SPEC-SYN-002/spec.md` §2.2 — explicit deferral of
  semantic faithfulness to SPEC-EVAL-001
- `.moai/specs/SPEC-OBS-001/spec.md` — Prometheus cardinality
  allowlist discipline EVAL-001 honours
- `.moai/specs/SPEC-LLM-001/spec.md` — LiteLLM router contract
  the judge invocation flows through
- `.moai/specs/SPEC-CLI-002/spec.md` — CLI surface contract for
  the `usearch eval` subcommand
- `.claude/rules/moai/design/constitution.md` §12 — evaluator
  leniency prevention mechanisms (NFR-EVAL1-001 anchors to
  Mechanism 4 calibration)
- `.moai/config/sections/quality.yaml` — `development_mode: tdd`
  (methodology selection rationale)
- `.moai/config/sections/harness.yaml` — harness routing for
  Sprint Contract decision

---

*End of SPEC-EVAL-001 v0.1.0 (draft).*
