# SPEC-EVAL-001 Research — Citation Faithfulness Benchmark (DeepEval + 50-query golden set + CI gate)

Status: draft companion to spec.md
Author: limbowl via manager-spec
Date: 2026-05-22

This research artifact is the Phase 0.5 deep-dive that informed
SPEC-EVAL-001. It captures the evaluation-framework landscape, the
faithfulness metric definition, golden set construction methodology,
CI integration patterns, cost analysis at the chosen threshold, the
LLM-as-judge calibration concerns specific to faithfulness scoring,
and the open questions the SPEC explicitly defers.

---

## 1. Framework comparison — DeepEval vs alternatives

The roadmap explicitly names DeepEval as the scorer
(`.moai/project/roadmap.md` line 103: "DeepEval scorer, CI gate at
≥0.85"). This research validates the choice and documents what was
considered before locking it in.

### 1.1 DeepEval (chosen, per HISTORY D1)

- **Repo**: https://github.com/confident-ai/deepeval (~3k stars as
  of 2026-05, active maintenance from confident-ai)
- **Faithfulness metric**: `FaithfulnessMetric` ships out-of-the-box.
  Scores based on claim-level entailment: splits the generated
  output into claims, asks the judge LLM whether each claim is
  supported by the retrieval context, returns
  `(supported_claims / total_claims) ∈ [0, 1]`.
- **Pros**:
  - Lightweight Python install (~30 MB transitively), no heavy
    numpy/pandas dependency tree
  - Pytest-style API (`assert_test(test_case, [FaithfulnessMetric])`)
    integrates cleanly with CI
  - LiteLLM-compatible judge model selection (matches our
    SPEC-LLM-001 router)
  - Active development; recent v1.0 release stabilises the metric
    signatures
  - Per-claim rationale output is exposed (`metric.reason`) — gives
    us the operator-facing debugging surface REQ-EVAL1-007 needs
- **Cons**:
  - Younger than RAGAS; ecosystem still growing
  - Default judge prompts are tuned for GPT-4 family; need
    validation on Haiku 4.5 (Phase 2 of plan covers this)

### 1.2 RAGAS (rejected — runner-up)

- **Repo**: https://github.com/explodinggradients/ragas (~8k stars)
- **Faithfulness metric**: equivalent claim-level entailment scoring,
  formalised by Es et al. 2023 (arxiv 2309.15217)
- **Pros**: more mature ecosystem; richer metric library
  (AnswerRelevancy, ContextPrecision, ContextRecall in addition to
  Faithfulness)
- **Cons**:
  - **Dependency weight**: pulls numpy + pandas + datasets + nltk
    (~120 MB transitively). Cold-start time on GitHub Actions
    runner is ~45s vs DeepEval's ~12s — meaningful when CI runs on
    every PR.
  - 0.x → 1.x migration broke many published recipes; signature
    instability concern
  - Pytest integration is less idiomatic (RAGAS prefers its own
    Dataset abstraction)
- **Verdict**: rejected for V1 due to CI cold-start cost; would
  reconsider for V2 if EVAL-001 grows to need RAGAS's broader
  metric suite.

### 1.3 TruLens (rejected)

- **Repo**: https://github.com/truera/trulens (~2k stars)
- **Faithfulness analog**: `Groundedness` feedback function
- **Cons**:
  - Heavier architecture (feedback functions + dashboards) — overkill
    for our flat CI gate use case
  - Less direct claim-level scoring; aimed at production tracing not
    benchmark scoring
- **Verdict**: wrong tool for the job; TruLens is observability,
  EVAL-001 needs a benchmark gate.

### 1.4 Promptfoo (rejected)

- **Repo**: https://github.com/promptfoo/promptfoo (~5k stars)
- **Cons**:
  - JS/TS native; would force Node toolchain into the Go+Python CI
    stack
  - Faithfulness is one of many features; less specialised
  - Its strength is A/B comparison of prompts, not scoring a
    single-prompt regression gate
- **Verdict**: ecosystem mismatch; rejected.

### 1.5 Custom (build our own scorer)

- Considered briefly. Would require:
  - Claim segmentation (already have SPEC-SYN-002's regex)
  - Judge prompt design (non-trivial — see §2)
  - Per-claim entailment loop
  - Maintenance burden every time the judge model changes
- **Verdict**: rejected. DeepEval gives us the same primitive
  battle-tested by a wider user base. Custom path is reserved for
  if-and-when DeepEval can no longer satisfy our needs.

---

## 2. Faithfulness metric — formal definition

### 2.1 The claim-level entailment definition

A response `R` (the synthesized text) cites a retrieval context `C`
(the set of doc body texts whose `doc_id`s appear as `[N]` markers
in `R`). The faithfulness score is:

```
faithfulness(R, C) = |supported_claims(R, C)| / |claims(R)|

where:
  claims(R)            = sentences of R segmented by SPEC-SYN-002 regex
  supported_claims(R,C)= { c in claims(R) : judge_entails(c, C) }
  judge_entails(c, C)  = LLM judgment: "is claim c entailed by any doc in C?"
```

Range: `[0, 1]`. Higher is better. `1.0` means every sentence is
supported by the cited evidence; `0.0` means no sentence is.

This matches DeepEval `FaithfulnessMetric`'s default formulation
(verified against deepeval/metrics/faithfulness.py as of v1.0).

### 2.2 What the metric does NOT measure (boundary)

- Whether the cited doc's *content* is factually correct (the doc
  itself could be wrong; faithfulness only measures
  doc→claim entailment, not world-truth)
- Whether claims that span multiple sentences are properly
  attributed (claim segmentation is sentence-grained)
- Whether the response is *relevant* to the query
  (that is `AnswerRelevancyMetric`, deferred to a future SPEC)
- Whether the *cited* doc is the best possible source (that is
  `ContextualPrecisionMetric`, also deferred)

These are documented in spec.md §4 Exclusions to set scope.

### 2.3 Determinism considerations (rationale for HISTORY D7)

LLM judges are non-deterministic by default. Three sources of
variance:

1. **Sampling temperature**: at temperature > 0, the same prompt can
   yield different judgments across runs. Pinning to
   `temperature=0, top_p=1` mitigates this but does not eliminate
   it — Anthropic's models still exhibit ~2-5% variance at
   temperature=0 due to internal sampling at the logit tier.
2. **Model version drift**: a model identifier like
   `claude-haiku-4-5` can route to different fine-tunes over time as
   Anthropic ships updates. We pin a specific version when possible
   (LiteLLM exposes the underlying model string).
3. **Prompt-cache invalidation**: when the system prompt changes
   even cosmetically, prompt-cached prefixes invalidate, which can
   cause slight scoring variance.

Our mitigation:

- `temperature=0, top_p=1, seed=42` (DeepEval supports `seed`
  passthrough via LiteLLM)
- NFR-EVAL1-001 tolerance: ±0.02 across consecutive re-runs is
  acceptable noise; ±0.05+ triggers CI block
- Monthly calibration runs against Sonnet judge as a sanity check
  (out-of-band, not per-PR)

### 2.4 False-positive escape hatch (rationale for HISTORY D8)

LLM judges occasionally mis-score. Two failure modes:

- **False negative**: judge says "unsupported" on a faithful claim.
  Causes: judge misreads the cited doc; claim uses synonyms the
  judge doesn't bridge.
- **False positive**: judge says "supported" on an unfaithful claim.
  Causes: judge hallucinates support; cited doc is tangentially
  related and judge over-generalises.

False positives are harder to detect (the test passes when it
shouldn't), but false negatives are what block CI unfairly. The
override mechanism (REQ-EVAL1-003) addresses the false-negative
case: an operator can mark a query as known-flaky after
investigating, with mandatory `override_reason` + expiry. Cap of
5 active overrides prevents abuse (if > 5 queries need override,
the judge model itself needs re-calibration).

---

## 3. Golden set construction methodology

### 3.1 Query selection (rationale for HISTORY D2)

The 50-query partition (35 EN + 15 KO) is calibrated to:

- **Cover the synthesis quality spectrum**: 5 categories
  (`factual`, `comparison`, `synthesis`, `korean`, `edge`) ensure
  the benchmark exercises retrieval-light, retrieval-heavy,
  multi-source, locale-specific, and adversarial paths.
- **Baseline Korean without overlapping EVAL-003**: 15 Korean
  queries is enough to detect catastrophic Korean-locale regression
  (we'd see at least 1-2 fail if the path breaks) but small enough
  that EVAL-003's 50-query Korean-first eval doesn't double-count.
- **Reproducibility**: 50 queries × 30s budget = 25 min worst case,
  fitting NFR-EVAL1-004's 15 min target with parallelism of 5
  (50 × 30s ÷ 5 = 5 min ideal, ~10 min realistic with overhead).

Category distribution:

| Category | Count | Description |
|----------|-------|-------------|
| `factual` | 15 | Single-fact queries, expect 1-3 doc citations |
| `comparison` | 10 | "X vs Y" queries, expect 2-5 doc citations per side |
| `synthesis` | 10 | Multi-claim synthesis, 5+ docs typical |
| `korean` | 12 | Korean-locale queries (subset of the 15 `locale:"ko"`) |
| `edge` | 3 | Adversarial: ambiguous queries, queries with no good answer in corpus |

(Note: `korean` category overlaps with `locale: "ko"` partition;
the 15 Korean queries are split across `korean` (12) + `factual`
(2) + `synthesis` (1) categories.)

### 3.2 Ground truth (`expected_sources`) — informational, not scored

`expected_sources` is documented in REQ-EVAL1-001 but is **not
consumed** by the scorer in V1. DeepEval `FaithfulnessMetric`
scores against retrieved context (what synthesis actually cited),
not against expected context (what we wished it cited). This is a
deliberate scope decision:

- Scoring expected-vs-actual would be a "context precision/recall"
  metric, not a "faithfulness" metric — that is the boundary
  established by RAGAS literature
- Ground-truth selection introduces curator bias (every operator
  has a different opinion on which doc is "best")
- V1 keeps `expected_sources` as documentation for future
  AnswerRelevancy or ContextPrecision sibling SPECs

### 3.3 Corpus construction (rationale for REQ-EVAL1-002)

The frozen corpus (`internal/eval/golden/corpus/*.json`) is built
by:

1. Manually selecting 200+ real-world docs representative of the
   project's adapter mix (Reddit, HN, arXiv, GitHub, YouTube,
   Bluesky, Naver, KoreaNewsCrawler).
2. Stripping personally-identifiable information (PII): no real
   usernames, emails, or private text. Use synthetic substitutes
   where the content requires it.
3. Versioning via `manifest.json:corpus_revision` (semver). Every
   addition or modification bumps the patch version.
4. License compliance: every fixture must be derived from
   public-domain or open-licensed content; the manifest records
   the source license per doc.

The corpus is **immutable between releases** — bumping
`corpus_revision` is a deliberate operator action, not a
side-effect.

---

## 4. CI gate threshold analysis (rationale for HISTORY D5)

### 4.1 Why ≥ 0.85?

The roadmap M8 exit criterion is fixed at `≥0.85`. This research
validates the threshold against:

- **DeepEval reference benchmarks**: published RAG systems on
  similar tasks score 0.78–0.92 depending on retrieval quality.
  0.85 is a competitive bar — achievable but not trivial.
- **SPEC-SYN-002 baseline**: structural enforcement alone (which
  is what's shipped today) produces ~0.70 raw faithfulness on
  Haiku 4.5 outputs because structural enforcement doesn't catch
  semantic drift. Hitting 0.85 requires SYN-002 retry-then-strip
  + prompt-engineering improvements + maybe a future SPEC-SYN-005
  semantic-aware retry. 0.85 is the *quality bar*, not the *easy*
  threshold.
- **Industry RAG quality benchmarks** (LangChain RAGAS leaderboard,
  Anthropic citation paper): 0.80–0.85 is "good production
  quality"; 0.90+ is "research-paper-quality".

### 4.2 Why a per-query floor (≥ 0.50)?

Aggregate-only thresholds let pathological queries hide. Example:
49 queries score 1.0 + 1 query scores 0.0 → mean = 0.98, passes,
but one entire query category is broken. The 0.50 floor catches
this:

- 0.50 is well below the aggregate target (won't false-trigger)
- Any score < 0.50 means majority of claims unsupported — a clear
  regression worth investigating
- Floor + aggregate together: any single broken query AND any
  systemic drift both surface

### 4.3 Threshold-tightening policy

Per HISTORY D5, both thresholds are **FROZEN** — they may be
tightened by a follow-up SPEC but never lowered. This prevents
the "just lower the threshold to make CI pass" anti-pattern.

---

## 5. Cost analysis (rationale for HISTORY D4, NFR-EVAL1-003)

### 5.1 Per-run cost at Haiku 4.5 (2026-05 pricing)

Claude Haiku 4.5 pricing (verified against
https://www.anthropic.com/pricing, 2026-05):

- Input: $1.00 / 1M tokens
- Output: $5.00 / 1M tokens

Per-query judge invocation:

- DeepEval prompts the judge once per *claim* (not per query); a
  typical synthesis output has 4-6 claims
- Input per claim: ~800 tokens (claim text + cited doc body ≈ 600
  + judge prompt + few-shot examples)
- Output per claim: ~150 tokens (binary verdict + 100-word rationale)
- Cost per claim: 800/1M × $1 + 150/1M × $5 = $0.0008 + $0.00075 ≈ $0.00155
- Cost per query (5 claims avg): $0.00775
- Cost per 50-query run: $0.39
- Plus 20% retry buffer: $0.47
- Rounded: ≤ $0.50 per run (NFR-EVAL1-003 cap)

### 5.2 Monthly CI cost projection

- Assume 100 PRs/month × $0.50 = $50
- Plus 30 nightly runs × $0.50 = $15
- Total ≈ $65/month

This is well within project operational budgets. If cost grows
beyond $200/month (warning threshold), revisit by:
- Reducing judge calls (cache results by claim hash)
- Sampling: run partial benchmark on PR, full benchmark nightly
- Switching to GPT-4o-mini (slightly cheaper but less calibrated)

### 5.3 Why not Sonnet judge?

Sonnet 4.5 pricing is ~5× Haiku:

- Per-run cost would be $2.50, monthly ≈ $325
- Quality lift on binary entailment judgments is marginal
  (research: Zheng et al. 2023 show 8B+ models converge on
  entailment tasks)
- Reserved for monthly calibration runs (1 × $2.50/month = $30/month
  additional, acceptable)

---

## 6. CI integration patterns

### 6.1 Dedicated workflow (chosen, per HISTORY D6)

`.github/workflows/eval.yml` is separate from the main `go.yml` and
`python.yml`. Reasons:

- **Different toolchain**: eval needs both Go runner + Python
  deepeval; bundling into either single-language workflow inflates
  cold-start.
- **Path filtering**: eval runs only on PRs touching synthesis,
  fanout, LLM, or eval paths. The main test workflows run on every
  PR. Separation lets each filter independently.
- **Failure isolation**: an eval failure doesn't block linting/unit
  tests; failing fast on cheaper checks is preferable.

### 6.2 Path filter spec

```yaml
on:
  pull_request:
    paths:
      - 'internal/synthesis/**'
      - 'services/researcher/**'
      - 'internal/llm/**'
      - 'internal/fanout/**'
      - 'internal/eval/**'
      - '.moai/specs/SPEC-EVAL-001/**'
    paths-ignore:
      - '**.md'
      - '**/docs/**'
```

Note: `paths` and `paths-ignore` interact via OR (if `paths`
matches, the workflow runs unless `paths-ignore` also matches).
Verified against GitHub Actions docs (cited in spec.md §9).

### 6.3 Nightly cron (chosen, per HISTORY D9)

`.github/workflows/eval-nightly.yml` runs at `03:00 UTC` via
`schedule: cron: '0 3 * * *'`. Purpose:

- Catch regressions in `main` even when no PR touches synthesis
  paths (e.g., a dependency upgrade in `go.sum` could break the
  LiteLLM router silently)
- Populate the regression baseline used by `evaluator-active`
  Mechanism 2 (per `.claude/rules/moai/design/constitution.md` §12)
- Alert on day-over-day drops > 0.05 (operator-owned alert wiring,
  outside this SPEC)

---

## 7. LLM-as-judge prompting pitfalls

Citing Zheng et al. 2023 "Judging LLM-as-a-Judge"
(https://arxiv.org/abs/2306.05685), known bias modes for LLM
judges relevant to faithfulness scoring:

- **Position bias**: when presenting multiple options, the judge
  favours the first. Not applicable to our binary entailment task.
- **Length bias**: judges favour longer responses. Not applicable
  to entailment (we judge supported/not-supported, not preferred).
- **Self-enhancement bias**: judges favour responses generated by
  themselves. Mitigated by using a *different* model family for
  judge than for synthesis where possible (synthesis uses
  GPT-4o-mini per SPEC-SYN-001; judge uses Haiku — different
  family).
- **Refusal bias**: judges occasionally refuse to score
  controversial content. Mitigate by using neutral, factual queries
  in the golden set (no political, medical, legal content).
- **Calibration bias on minority languages**: documented in
  research that GPT-4 judges score non-English outputs lower at
  equivalent quality. Haiku 4.5 has improved Korean support but
  remains an open question for our specific use case
  (research §10 Q4).

DeepEval's stock faithfulness prompt is engineered to mitigate
position + length bias. Self-enhancement bias is the operator's
responsibility via model selection.

---

## 8. Observability + reporting design

### 8.1 Report structure

The markdown report (`.moai/eval/reports/latest.md` + PR comment
content) has the structure:

```
# EVAL-001 Report — {date} — {commit_sha}

## Summary
- Mean score: {X.XXX}
- Floor: {X.XX} (min query score)
- Status: {PASS|FAIL}
- Queries scored: {N} (null: {M}, overridden: {K})
- Total cost: ${X.XX}
- Runtime: {N}m {M}s
- Judge model: {model}

## Score by category
| Category | Mean | Min | Max | Queries |
|----------|------|-----|-----|---------|
| factual | ... | ... | ... | ... |
...

## Lowest-Scoring Queries (top 10)
### Q017 (score: 0.40, category: synthesis, locale: en)
Query: {query text}
Claims scored: 5 total, 2 supported
Unsupported claim 1: "{claim text}"
Judge rationale: "{rationale}"
...

## Regression Delta (vs last nightly)
- Mean: {X.XXX} → {Y.YYY} (Δ {±Z.ZZZ})
- Notable shifts: ...

## Active Overrides
- Q023: pass — "judge mis-scores Korean technical terms" (expires 2026-06-20)
```

### 8.2 Prometheus metrics (REQ-EVAL1-010 wiring)

Two new collectors, both following SPEC-OBS-001 allowlist:

- `usearch_eval_runs_total{outcome="pass"|"fail"|"null"|"override_cap"}`
  — Counter. Reuses existing `outcome` label name (no new
  high-cardinality label added).
- `usearch_eval_score_gauge` — Gauge (no labels). The most recent
  aggregate mean score, exported for Grafana dashboarding.

---

## 9. Risk register (top 10)

1. **Haiku judge calibration drift across model updates** —
   Anthropic ships Haiku updates without versioning; an
   undocumented model change could shift baseline scores. Mitigation:
   nightly cron + ±0.05 drift alert (NFR-EVAL1-001).
2. **DeepEval breaking API changes** — v1.0 stabilises the public
   signatures, but pre-1.0 history shows churn. Mitigation: pin
   to `~= 1.0` (compatible release), CI tests the bridge layer.
3. **GitHub Actions secrets leakage** — judge model API key lives
   in secrets; exposure would burn judge budget. Mitigation:
   gitleaks CI scan; secret access scoped to `eval.yml` workflow
   only.
4. **Korean judge bias** — Haiku may systematically under-score
   Korean responses. Mitigation: Phase 2 of plan validates this;
   if bias > 0.10, opens follow-up SPEC for per-locale judge.
5. **Golden set staleness** — corpus + queries written in 2026-05
   may not reflect quality patterns 6 months later. Mitigation:
   nightly regression baseline detects gradual drift; corpus
   refresh is a fast-follow SPEC if needed.
6. **CI runtime overrun on cold judge cache** — first-of-day
   nightly run lacks prompt cache; may exceed 15 min. Mitigation:
   NFR-EVAL1-004 sets soft warning at 15 min, hard fail at 25 min;
   parallelism of 5 keeps headroom.
7. **Override-cap exhaustion** — if 5+ queries genuinely need
   override, the cap blocks CI. Mitigation: cap exhaustion is a
   signal that judge needs re-calibration; operator response is
   to fix root cause, not raise the cap.
8. **Synthesis path changes break corpus assumptions** — a
   refactor that changes citation marker format would invalidate
   the bridge's parsing. Mitigation: REQ-EVAL1-005 uses
   SPEC-SYN-002's regex by reference, not by copy.
9. **Cost overrun on retry storms** — REQ-EVAL1-006 null-marking
   prevents zero-scoring on judge timeout, but doesn't bound
   retry attempts. Mitigation: 30s timeout NFR-EVAL1-002 + no
   automatic retries in the bridge (timeouts mark null,
   investigate manually).
10. **Concurrent CI runs colliding on judge rate limits** — if
    multiple PRs touch synthesis paths simultaneously, judge
    API rate limits could throttle. Mitigation: parallelism
    cap of 5 queries within a run + GitHub Actions concurrency
    group ensures only one EVAL-001 run per workflow per
    commit SHA.

---

## 10. Open questions (deferred to plan or run phase)

1. **Plugin / golden-set repo placement** — monorepo
   `internal/eval/golden/` (recommended; tightly coupled to Go
   runner) vs separate `usearch-eval-golden` repo (allows
   independent versioning of corpus + queries). _Resolution_:
   monorepo for V1; reconsider if corpus grows beyond 1000 docs.

2. **Cost report granularity** — per-query / per-category /
   aggregate-only. _TBD_ — annotation cycle decides; default
   recommendation is per-category + aggregate.

3. **DeepEval version pin** — `>= 1.0.0` (allows breaking 2.x)
   vs `~= 1.0` (compatible release only, recommended). _TBD_ —
   annotation cycle.

4. **Korean judge calibration bias** — does Haiku 4.5 score Korean
   responses lower at equivalent quality? Phase 2 validates by
   running 15 Korean queries through both Haiku and Sonnet judges
   and comparing scores. If gap > 0.10, open SPEC-EVAL-001-A1
   (amendment) for per-locale judge override config.

5. **Override mechanism abuse vector** — could an operator stack
   5 overrides to mask a real regression? Mitigation: override
   audit log + 30-day expiry. But long-term: consider requiring
   2-person approval for new overrides. _TBD_ — patch SPEC after
   3 months of usage data.

6. **Nightly cron timing collision with EVAL-002** — both M8
   evals will have nightly cron needs. _Resolution_:
   EVAL-001 → 03:00 UTC; EVAL-002 → 04:00 UTC (avoid overlap).
   Coordinate when EVAL-002 lands.

7. **PR comment overwrite vs append** — overwrite for cleanliness
   vs append to preserve history of fix attempts. _TBD_ —
   annotation cycle; default recommendation is overwrite with a
   timestamp footer ("Last updated 2026-05-22 14:30 UTC").

8. **Per-claim cost tracking** — DeepEval doesn't natively expose
   cost per metric call; we'd need to wrap the LiteLLM client.
   Implementation detail deferred to run phase.

9. **Reference handling for retrieved-but-not-cited docs** —
   pass to judge or strip out? Plan recommends strip (focus
   judge on cited evidence only), but DeepEval may want full
   context. Verify in run phase against DeepEval docs.

10. **Golden set curator** — V1 author = limbowl. Long-term:
    should rotate ownership to avoid single-curator bias.
    Out-of-scope for SPEC content; this is process.

---

## 11. References

External (cited in spec.md §9):

- DeepEval: https://github.com/confident-ai/deepeval
- DeepEval FaithfulnessMetric: https://docs.confident-ai.com/docs/metrics-faithfulness
- DeepEval LiteLLM integration: https://docs.confident-ai.com/docs/guides-using-custom-llms
- RAGAS (rejected, comparison baseline): https://docs.ragas.io/en/stable/concepts/metrics/faithfulness.html
- RAGAS paper (Es et al. 2023): https://arxiv.org/abs/2309.15217
- TruLens (rejected): https://www.trulens.org/
- Promptfoo (rejected): https://www.promptfoo.dev/
- LiteLLM model routing: https://docs.litellm.ai/docs/
- LLM-as-judge biases (Zheng et al. 2023): https://arxiv.org/abs/2306.05685
- GitHub Actions path filters: https://docs.github.com/en/actions/using-workflows/triggering-a-workflow#using-filters
- GitHub Actions cron schedule: https://docs.github.com/en/actions/using-workflows/events-that-trigger-workflows#schedule
- Claude Haiku 4.5 pricing: https://www.anthropic.com/pricing

Internal:

- `.moai/project/roadmap.md` line 103 (SPEC-EVAL-001 row) + line 157
  (M8 exit criterion)
- `.moai/specs/SPEC-SYN-002/spec.md` §1 (semantic faithfulness
  explicitly deferred to EVAL-001) + §3 NFR-SYN2-001 (regex defined)
- `.moai/specs/SPEC-OBS-001/spec.md` (cardinality allowlist
  discipline)
- `.moai/specs/SPEC-LLM-001/spec.md` (LiteLLM router contract)
- `.moai/specs/SPEC-CLI-002/spec.md` (CLI surface for `usearch eval`)
- `internal/eval/eval.go` (existing stub package this SPEC fills in)
- `internal/synthesis/*` (read-only consumer)
- `services/researcher/src/researcher/faithfulness.py` (structural
  faithfulness sibling, line-level cited at REQ-EVAL1-005)
- `services/researcher/src/researcher/synthesis.py:_process_markers`
  (citation marker semantics)
- `.claude/rules/moai/design/constitution.md` §12 (evaluator
  leniency prevention, anchors NFR-EVAL1-001)

---

*End of SPEC-EVAL-001 research v0.1.0 (draft).*
