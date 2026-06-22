# Inter-rater agreement — κ interpretation + threshold policy

SPEC-EVAL-003 · v0.2.0 · REQ-EVAL-004

The benchmark uses **Cohen's κ** (pairwise) aggregated by **Light's mean-κ**
(arithmetic mean of all pairwise κ across the 3 raters). Krippendorff α is
**deferred to post-V1** — Cohen/Light κ is sufficient for the V1 inter-rater
gate.

## Landis-Koch (1977) bands

| κ range       | Interpretation                           |
| ------------- | ---------------------------------------- |
| < 0.00        | Poor (worse than chance)                 |
| 0.00–0.20     | Slight                                   |
| 0.21–0.40     | Fair                                     |
| 0.41–0.60     | Moderate                                 |
| **0.61–0.80** | **Substantial ← V1 gate floor (≥ 0.60)** |
| 0.81–1.00     | Almost perfect                           |

## Threshold policy

- **Gate: Light's mean-κ ≥ 0.60** → round is **valid**, may produce a snapshot.
- mean-κ < 0.60 → round is **invalid** → discard and re-round (protocol §6).
- The 0.60 floor adopts Landis-Koch "substantial agreement". V1 fixes it at
  0.60; raising it to 0.70 is an open question for a future amendment (spec §8).

## How it is computed (deterministic)

- `CohenKappa(a, b)` over two raters' `ranking_score` vectors (position-aligned,
  equal length). κ = (po − pe)/(1 − pe). Identical constant vectors → κ = 1.0.
- `MeanKappa(raters)` requires ≥ 3 raters, returns the per-pair κ list + the
  mean + the validity verdict. Same input → same output (1e-6 tolerance in
  tests).

## What κ does and does not tell you

- κ measures **agreement between raters**, not correctness of the ranking. A
  high-κ round can still report low recall — that is a real ranking regression,
  not a rater problem.
- A low-κ round means the raters disagreed too much for the metrics to be
  trustworthy; re-round before drawing any conclusion.
