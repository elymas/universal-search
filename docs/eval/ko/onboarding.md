# Rater onboarding — fluency criterion + anonymization

SPEC-EVAL-003 · v0.2.0 · NFR-EVAL-004, NFR-EVAL-005

## Fluency criterion (NFR-EVAL-005)

Raters MUST be **native or near-native Korean speakers** with sufficient
familiarity with Korean web culture to judge Naver vs Daum vs international
source relevance. A rater who cannot tell whether a Naver-blog result or an
international result better serves a Korean query is not qualified.

## Anonymization (D8, NFR-EVAL-004)

- Raters are identified ONLY by anonymous IDs: `R1`, `R2`, `R3`, …
- No real names, emails, phone numbers, or account identifiers appear in any
  sheet, snapshot, or doc.
- Recruitment evidence (anonymous ID + fluency self-attestation date only) is
  recorded in `rater-pool.md`. Nothing else about a rater is stored.

## A round needs ≥ 3 raters

Light's mean-κ requires at least 3 independent raters (REQ-EVAL-004, EC-001). A
single-rater or two-rater round is rejected by the aggregator.

## No collaboration during scoring

Each rater scores independently on a private sheet. Inter-rater agreement (κ) is
only meaningful if the raters did not influence each other. Discussion happens
only AFTER scoring, and only in the post-V1 calibration ceremony (deferred).

## What raters receive

Exactly four documents — nothing more:

1. `docs/eval/ko/protocol.md`
2. `docs/eval/ko/rubric.md`
3. `tests/eval/korean/golden-set.jsonl`
4. `tests/eval/korean/scoring-sheet-template.csv`

If a rater needs to consult the SPEC author to proceed, the protocol failed its
reproducibility gate (NFR-EVAL-001) and must be amended.
