# SPEC-EVAL-001 — Citation Faithfulness Benchmark (Operator Guide)

This directory holds the artifacts produced by the citation faithfulness
benchmark (SPEC-EVAL-001). The benchmark is the **M8 release gate** between
M7 surface completion and the M9 V1.0.0 tag: a PR that drops the aggregate
faithfulness mean below **0.85** is blocked.

## What runs

`go run ./cmd/eval` (driven in CI by `.github/workflows/eval.yml`):

1. Loads the frozen golden set (`internal/eval/golden/queries.jsonl`,
   50 queries: 35 EN + 15 KO) and the `NormalizedDoc` corpus
   (`internal/eval/golden/corpus/*.json`).
2. Drives the synthesis path for each query against the frozen corpus.
3. Ships each (claim, cited-doc-body) pair to the DeepEval judge endpoint
   (`POST /judge/faithfulness`) on the researcher sidecar.
4. Aggregates a mean score and exits with the gate code.

## Reading the report

`reports/latest.md` is the most recent run. Key sections:

- **Header** — mean score, null count, overrides applied, judge model, cost.
- **Per-Category Breakdown** — mean by category (factual / comparison /
  synthesis / korean / edge).
- **Lowest-Scoring Queries** — the 10 lowest-scoring queries with the judge's
  rationale for each unsupported claim. This is your primary debugging surface.
- **Null (Unscoreable) Queries** — queries the judge could not score (sidecar
  down / timeout). Null != zero: a null forces gate exit code 2.

## Exit codes (REQ-EVAL1-008)

| Code | Meaning |
|------|---------|
| 0 | PASS — mean >= 0.85, every query >= 0.50, no nulls, overrides <= 5 |
| 1 | FAIL — mean below 0.85, a query below the 0.50 floor, or override cap exceeded |
| 2 | Judge availability error — one or more queries scored null |
| 3 | Malformed input — golden set / corpus / overrides failed to load |

The grep-friendly summary line is:

```
EVAL-001 result=PASS|FAIL mean=<X.XXX> floor=<X.XX> overrides=<N> nulls=<N>
```

## Determinism (NFR-EVAL1-001, FROZEN)

The judge is invoked via LiteLLM with `temperature=0`, `top_p=1`, `seed=42`.
Re-runs on the same code + corpus revision must stay within ±0.02 of the
recorded mean; ±0.05 drift blocks CI. These parameters may not be changed
without a constitution amendment.

## Manual overrides (REQ-EVAL1-003)

If the LLM judge systematically mis-scores a genuinely faithful query, add an
entry to `internal/eval/golden/overrides.json`:

```json
[
  {
    "query_id": "EVAL-001-Q017",
    "manual_override": "skip",
    "override_reason": "judge mis-scores valid Korean entailment; tracked in SPEC-EVAL-003",
    "created_at": "2026-05-29T00:00:00Z",
    "created_by": "your-github-handle",
    "expires_at": "2026-06-29"
  }
]
```

Rules:

- `manual_override` is `"pass"` or `"skip"` — both exclude the query from the
  aggregate.
- A maximum of **5** active overrides is enforced (a 6th fails CI with exit 1).
- `expires_at` is advisory only in V1 — prune stale entries **by hand**.
  (Automated 30-day expiry is deferred to a follow-up patch SPEC.)
- The override list is committed to git for audit.

## Judge model swap (NFR-EVAL1-005)

Set `EVAL_JUDGE_MODEL` to any LiteLLM model string (e.g. `gpt-4o-mini`).
No code change is required — the judge is routed exclusively through LiteLLM.

## Deferred to V1.1 (REQ-EVAL1-010)

The nightly regression cron, the `eval-nightly.yml` workflow, and the
`history/` JSON writer are **not** part of V1. The PR-gate above is the V1
release gate.
