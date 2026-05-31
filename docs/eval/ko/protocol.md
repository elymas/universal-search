# Korean-locale benchmark — scoring protocol (rater handbook)

SPEC-EVAL-003 · v0.2.0 · manual, once-per-cycle, offline

This document is self-contained: a qualified native-Korean rater can complete a
full 50-query round from this file + `rubric.md` + `golden-set.jsonl` +
`scoring-sheet-template.csv` **without consulting the SPEC author or any prior
rater** (NFR-EVAL-001).

---

## 0. What this benchmark is (and is not)

- It measures **retrieval/ranking quality** for Korean-first queries — whether
  Naver-suite results rank highly when a Korean user expects them.
- It is **manual and human-in-the-loop**. There is no LLM-as-judge in V1:
  Korean LLM judges carry English bias and tokenizer mismatch that distort
  ordinal relevance judgments (research §4). LLM-as-judge is deferred post-V1.
- It runs **once per release cycle, offline**. It is **not** a per-PR check.
- CI is **non-blocking**: it only attaches artifacts (the golden set, a fresh
  scoring sheet, a diff report). It never fails a build.

### Accepted limitation (disclosed, D4 finding)

Because scoring is manual and once-per-cycle, **regressions stay silent between
manual rounds**. A Korean-ranking regression introduced after a round will not
be detected until the next round is scored. This is an accepted V1 trade-off,
not a bug. Mitigation: the golden-set JSONL ships as a CI artifact every release
so a regression is diagnosable the moment a round runs.

---

## 1. Who can rate (fluency criterion)

See `onboarding.md`. In short: native or near-native Korean, familiar with
Korean web culture (Naver vs Daum vs international source relevance). Raters use
anonymous IDs (R1/R2/R3…). No PII anywhere.

---

## 2. The round at a glance

1. A round needs **at least 3 independent raters** (REQ-EVAL-004, EC-001). A
   single-rater round is rejected — Light's mean-κ requires ≥ 3 raters.
2. Each rater scores all 50 queries on a private copy of
   `scoring-sheet-template.csv`. No collaboration during scoring.
3. After all sheets are in, compute Light's mean-κ across the three raters'
   `ranking_score` columns (`internal/eval/korean.MeanKappa`).
4. **Validity gate**: mean-κ ≥ 0.6 → round is **valid**. mean-κ < 0.6 → round
   is **invalid** (see §6).
5. A valid round produces exactly one append-only baseline snapshot
   (`internal/eval/korean.WriteSnapshot`).

Throughput estimate (planning aid, NOT a gate, NFR-EVAL-002): ~50 queries per
rater per round; a 3-rater round is ~150 query-scorings of effort.

---

## 3. How to score one query

For each `query_id` you fill one row per rater. Steps:

1. Read `query_text` and `notes` (if any) from `golden-set.jsonl`.
2. Run the query against the live stack (or read the captured top-10 result
   list the round coordinator provides).
3. Fill the row:

| Column | Type | How to judge |
|--------|------|--------------|
| `ranking_score` | int 1–5 | Overall: do the top results match what a Korean user expects for this query? See `rubric.md` anchors. |
| `source_relevance` | int 1–5 | Are the SOURCES right (Naver for blog/shop/news, arXiv/GitHub for academic-tech)? |
| `code_switching_handling` | int 1–5 | **Required for code-mixed rows.** Korean segments tokenized with mecab-ko, English segments passed through, no over-segmentation of code identifiers. |
| `tokenization_quality` | int 1–5 | Were Korean terms segmented sensibly (mecab-ko)? |
| `top3_naver_hit` | bool | Is there a `naver`-source result in the top 3? (For `expected_naver_relevant:false` queries this is normally `false`.) |
| `mrr_top10` | float 0.0–1.0 | 1/(rank of first naver hit in top-10), or 0.0 if none. |
| `notes` | text | Optional rationale. |

5 = best on every 1–5 scale. Always anchor to `rubric.md`; do not invent your
own scale.

### Code-mixed handling (REQ-EVAL-007, D5)

For the 6 `code-mixed` queries, `code_switching_handling` is **mandatory** (no
blanks). Judge whether:

- Korean segments are tokenized with mecab-ko (not character-split).
- English segments pass through intact.
- Common code identifiers (e.g. `DataLoader`, `gRPC`, `OAuth2`) are NOT
  over-segmented into meaningless pieces.

---

## 4. Source expectations by category

| Category | Expected top sources | `naver` in top-3? |
|----------|----------------------|--------------------|
| news | `naver` (news vertical), `koreanews` | yes |
| blog | `naver` (blog vertical) | yes |
| shopping | `naver` (shop vertical) | yes |
| academic-tech | `arxiv`, `github` (NON-Naver) | no (`expected_naver_relevant:false`) |
| code-mixed | mixed — `github`/`arxiv`, sometimes `naver` | depends on the row |
| cultural | `naver` (blog/news), `koreanews` | yes |

Naver verticals come from the result `DocType` (`post`→blog, `article`→news,
`other`→web/shop/datalab); `other` is ambiguous and counts as a SourceID-only
"unverified" Naver hit. There is **no Naver `academic` vertical** — academic
queries target arXiv/GitHub.

---

## 5. Metrics computed from your sheets

- **top-3 Naver recall** (the only V1 gate): of the queries marked
  `expected_naver_relevant:true`, the fraction with a `naver` result in the
  top-3. M3-exit pass line is **0.80**.
- **MRR@10**: supplementary, recorded but never gating.
- **per-category recall**: observational, surfaced in the snapshot for
  localizing regressions; never gates.
- **router_class_accuracy_mixed**: % of code-mixed queries SPEC-IR-001 tagged
  `mixed`.

---

## 6. Invalid round → re-round (V1 minimal path, REQ-EVAL-009)

If the round's Light's mean-κ is **< 0.6**:

1. The round is **invalid**. **Discard it.**
2. **Re-run a fresh round from scratch** on a new sheet (new rater scoring).
3. **No baseline snapshot is produced** from an invalid round.
4. A snapshot is produced only once a re-round reaches mean-κ ≥ 0.6.

That is the entire V1 calibration path: **invalid round → re-round.**

**Deferred to post-V1** (NOT part of V1, do not perform for V1 rounds): joint
re-scoring of the 5 lowest-agreement queries, rubric-anchor divergence
discussion, FROZEN rubric amendment review by the SPEC owner, and a structured
`calibration-log.md` ledger. See `kappa-interpretation.md` for the κ bands.

---

## 7. After a valid round

The round coordinator writes the snapshot to
`tests/eval/korean/baseline-snapshots/{release-tag}.json`. Snapshots are
append-only — never edit an existing one; a new round is a new file. The four
most-recent snapshots stay live; older ones move to `archive/`.
