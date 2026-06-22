# Korean-locale baseline snapshot schema (SPEC-EVAL-003 REQ-EVAL-008)

A baseline snapshot is the append-only, release-tagged record of one **valid**
3-rater scoring round (mean-κ ≥ 0.6). Snapshots are written by
`internal/eval/korean.WriteSnapshot` and are never modified after creation.

> The real `v1.0.0.json` is NOT produced by this repository's automated tests.
> It requires an offline 3-rater human round on the M3 stack (task T09,
> operational). `v1.0.0.example.json` in this directory is a **placeholder
> example only** — every metric is zeroed and must not be read as the V1
> ranking baseline.

## File location

`tests/eval/korean/baseline-snapshots/{release-tag}.json`

The four most-recent snapshots stay here; older ones are moved (never deleted)
to `baseline-snapshots/archive/` by the writer's retention policy
(NFR-EVAL-003).

## Fields

| Field                            | Type                 | Required | Meaning                                                                                                 |
| -------------------------------- | -------------------- | -------- | ------------------------------------------------------------------------------------------------------- |
| `release_tag`                    | string               | yes      | Release this round baselines (e.g. `v1.0.0`). Doubles as the filename stem.                             |
| `round_date`                     | string (YYYY-MM-DD)  | yes      | Date the round was scored.                                                                              |
| `rater_ids`                      | string[]             | yes      | Anonymous rater IDs only (R1/R2/R3…). No PII (D8).                                                      |
| `mean_kappa`                     | float                | yes      | Light's mean-κ for the round. Must be ≥ 0.6 for a snapshot to exist.                                    |
| `top3_naver_recall`              | float [0,1]          | yes      | Aggregate top-3 Naver recall — the only V1 gate metric (≥ 0.80 at M3 exit).                             |
| `top3_naver_recall_per_category` | map<category,float>  | yes      | Per-category recall for the 6 buckets. Observational, non-gating (REQ-EVAL-006).                        |
| `mrr_top10`                      | float [0,1]          | yes      | Supplementary MRR@10 over Naver-relevant queries. Non-gating.                                           |
| `mean_ranking_score`             | float [1,5]          | yes      | Mean rater `ranking_score`.                                                                             |
| `router_class_accuracy_mixed`    | float [0,1]          | yes      | % of code-mixed queries SPEC-IR-001 classified as `mixed` (REQ-EVAL-007).                               |
| `tokenizer_version`              | string               | yes      | mecab-ko version pin (D6).                                                                              |
| `adapter_versions`               | map<sourceID,string> | yes      | **Registered** adapter SourceID → version pin. Phantom IDs (`naver-news`, `daum-news`, …) are rejected. |
| `golden_set_sha256`              | string (64 hex)      | yes      | SHA256 of the exact `golden-set.jsonl` scored. Binds the snapshot to its corpus.                        |

## Invariants

- **Append-only**: a writer never overwrites an existing `{release-tag}.json`
  (`ErrSnapshotExists`). A new round → a new file.
- **Valid-only**: an invalid round (mean-κ < 0.6) produces no snapshot
  (`ErrRoundInvalid`); the V1 path is re-round (REQ-EVAL-009).
- **Real SourceIDs only**: every `adapter_versions` key must be a registered
  adapter SourceID (`ErrPhantomAdapterID`).
- **Retention**: 4 most-recent live; older archived, never deleted.

## Deferred (post-V1, NOT in this schema's gating)

- golden-set SHA-drift WARNING in the diff report (EC-002).
- tokenizer version-drift annotation (EC-003).
- Krippendorff α (Cohen/Light κ only for V1).
