---
id: SPEC-EVAL-003
version: 0.2.0
status: draft
created: 2026-05-26
updated: 2026-05-30
author: limbowl (via manager-spec)
related_spec: SPEC-EVAL-003 (spec.md, plan.md)
format: Given/When/Then
---

# SPEC-EVAL-003 Acceptance Scenarios

## 0. Document Purpose

This document specifies acceptance criteria for SPEC-EVAL-003 in Given/When/Then format, expanding the scenario index in spec.md §5 (§5.1..§5.10) into externally-observable behaviors that the run phase MUST verify before declaring EVAL-003 ship-ready.

Scope: 10 acceptance criteria (AC-001..AC-010) covering REQ-EVAL-001 through REQ-EVAL-010 + NFR-EVAL-001 through NFR-EVAL-005, plus 3 edge-case sections, plus a Definition of Done checklist.

Coverage policy: every REQ and every NFR in spec.md §2 / §3 has ≥1 matching AC below. See Coverage Matrix at end of file.

---

## 1. Acceptance Criteria (Given/When/Then)

### AC-001 — Golden-set schema validation with exact category distribution + zero PII

Covers: REQ-EVAL-001, REQ-EVAL-002, NFR-EVAL-004

**Given** `tests/eval/korean/golden-set.jsonl` and `docs/eval/ko/golden-set-provenance.md`.

**When** the CI schema validator + PII grep gate runs.

**Then**:
- The file parses as exactly 50 JSON objects (one per line).
- Each object has all required fields: `query_id` (format `KR-NNN`), `query_text`, `category`, `expected_lang ∈ {ko, mixed}`, `expected_router_class`, `expected_naver_relevant` (boolean), `expected_naver_vertical` (optional, ∈ {blog, news, web, shop, datalab} per the live `naver_vertical` filter values), `expected_sources` (array of **registered SourceID strings only**), and optional `notes` ≤ 200 chars.
- Every `expected_sources` entry is a real registered adapter SourceID (e.g. `naver`, `koreanews`, `arxiv`, `github`). The legacy phantom strings `naver-news`/`naver-blog`/`naver-shopping`/`naver-academic`/`daum-news`/`korea-news-crawler` produce a schema-validation FAILURE (zero phantom IDs allowed).
- When present, `expected_naver_vertical ∈ {blog, news, web, shop, datalab}`; `academic` is NOT a valid vertical (Naver has no academic vertical in live code).
- Category distribution is EXACTLY: 12 news + 10 blog + 8 shopping + 8 academic-tech + 6 code-mixed + 6 cultural.
- Provenance for every query is recorded in `docs/eval/ko/golden-set-provenance.md` as either a Naver DataLab URL + capture date, or `synthetic, authored YYYY-MM-DD`.
- PII grep gate (email regex, Korean phone `010-XXXX-XXXX`, surname+given-name patterns) returns ZERO matches across the JSONL.
- Source-type audit: zero entries from user-submitted queries or log-extracted queries.

Maps to scenarios §5.1, §5.10 in spec.md.

---

### AC-002 — Manual scoring round with 3+ raters + Light's mean-κ ≥ 0.6

Covers: REQ-EVAL-003, REQ-EVAL-004

**Given** `tests/eval/korean/scoring-sheet-template.csv` with the prescribed header row and 3 independent rater sheets covering the same golden-set version.

**When** the round aggregator computes Cohen's κ pairwise and Light's mean-κ.

**Then**:
- CSV header is exactly: `query_id, rater_id, ranking_score, source_relevance, code_switching_handling, tokenization_quality, top3_naver_hit, mrr_top10, notes`.
- Field constraints enforced: `ranking_score`, `source_relevance`, `code_switching_handling`, `tokenization_quality` are integer 1–5; `top3_naver_hit` is boolean; `mrr_top10` is float [0.0, 1.0].
- 3 raters → 3 pairwise κ values computed.
- Light's mean-κ is the arithmetic mean of the 3 pairwise κ values.
- Mean-κ ≥ 0.6 → round marked `valid`; mean-κ < 0.6 → round marked `invalid` (triggers calibration per AC-007).
- Unit test boundary: 3 identical sheets → κ = 1.0; random sheets → κ ≈ 0.0; realistic divergent sheets → κ ∈ [0.3, 0.5] marked `invalid`.

Maps to scenario §5.2 in spec.md.

---

### AC-003 — M3 exit gate: top-3 Naver recall ≥ 0.80 on filtered subset

Covers: REQ-EVAL-005

**Given** a valid baseline round at the M3 implementation milestone, restricted to golden-set queries where `expected_naver_relevant == true`.

**When** the scoring aggregator computes top-3 recall for the `naver` SourceID (single registered adapter; verticals are derived from result `DocType`, NOT separate adapter IDs).

**Then**:
- A query is *Naver-hit at k=3* when at least one top-3 result has `SourceID == "naver"`; when the query sets `expected_naver_vertical`, the result's vertical (via `DocTypePost→blog`, `DocTypeArticle→news`, `DocTypeOther→{web,shop,datalab}`) must match, else the match is recorded `unverified` and falls back to SourceID-only.
- `recall@3 = (Naver-hit query count) / (expected_naver_relevant query count)`.
- Recall@3 ≥ 0.80 → M3 exit gate PASSES.
- Recall@3 < 0.80 → diagnostic trigger: regression analysis required across SPEC-ADP-008, SPEC-IDX-003, SPEC-IR-001.
- MRR@10 is computed and recorded as a supplementary metric but does NOT gate.
- Unit test fixture: known golden-set keyed on `SourceID == "naver"` with expected recall = 0.85 → snapshot reports 0.85 ± 0.01.
- No phantom adapter ID (`naver-news`/`naver-blog`/`naver-shopping`/`naver-academic`) appears anywhere in the scoring path.

Maps to scenario §5.3 in spec.md.

---

### AC-004 — Per-category recall surfaced + regression flag for drops > 0.10

Covers: REQ-EVAL-006

**Given** the current baseline snapshot + the immediately preceding snapshot.

**When** the snapshot-diff report generator runs.

**Then**:
- The snapshot JSON includes a `per_category` map with all 6 buckets: news, blog, shopping, academic-tech, code-mixed, cultural (recall keyed on the `naver` SourceID per AC-003).
- For each category, recall@3 is computed and persisted.
- (v0.2.0 — observational, NOT a V1 gate) If any bucket's recall drops by > 0.10 between snapshots, the diff report MAY include an informational flagged-warning naming the bucket + delta. This warning gates NOTHING; absence of the flagged-warning logic does NOT fail any V1 acceptance criterion.
- Aggregate (all-categories) top-3 Naver recall remains the only gating metric (per AC-003).

Maps to scenario §5.4 in spec.md.

---

### AC-005 — Code-mixed handling: router classification + mecab-ko segmentation

Covers: REQ-EVAL-007

**Given** the 6 code-mixed (한영 혼용) queries in the golden-set.

**When** the round aggregator analyzes their scoring sheets.

**Then**:
- Every code-mixed query's `code_switching_handling` field is NON-NULL across all rater sheets (validation enforced).
- The snapshot emits `router_class_accuracy_mixed`: percentage of code-mixed queries where SPEC-IR-001 correctly classified as `mixed`.
- The snapshot emits the mean `code_switching_handling` score across all raters on the 6 code-mixed queries.
- The protocol document `docs/eval/ko/protocol.md` describes rubric anchors for: mecab-ko tokenization on Korean segments, English passthrough on English segments, no over-segmentation of common code identifiers.
- Mean `code_switching_handling` ≥ 4/5 is the recommended target (observational, NOT gating).

Maps to scenario §5.5 in spec.md.

---

### AC-006 — Baseline snapshot creation + 4-snapshot retention with archive

Covers: REQ-EVAL-008, NFR-EVAL-003

**Given** a completed valid scoring round.

**When** the snapshot writer runs.

**Then**:
- A file is created at `tests/eval/korean/baseline-snapshots/{release-tag}.json`.
- The snapshot contains all required fields: `release_tag`, `round_date`, `rater_ids` (anonymous IDs only — no PII), `mean_kappa`, `top3_naver_recall` (aggregate + per-category), `mrr_top10`, `mean_ranking_score`, `router_class_accuracy_mixed`, `tokenizer_version` (mecab-ko version), `adapter_versions` (map of **real adapter SourceID → version pin**; every key is a registered SourceID such as `naver`/`koreanews` — phantom IDs like `naver-news`/`daum-news` are rejected), `golden_set_sha256`.
- The snapshot is append-only (never modified after creation; verified by file permission + commit-history check).
- The 4 most recent snapshots remain in `baseline-snapshots/`; older snapshots are auto-moved to `baseline-snapshots/archive/` by CI.
- Older snapshots are NEVER deleted (`archive/` is permanent).
- The `golden_set_sha256` value matches `sha256sum tests/eval/korean/golden-set.jsonl`.

Maps to scenario §5.6 in spec.md.

---

### AC-007 — Invalid round → re-round flow (minimal V1 path)

Covers: REQ-EVAL-009

**Given** a simulated invalid round (mean-κ < 0.6).

**When** the minimal V1 calibration path executes.

**Then** (V1 scope):
- The invalid round is discarded and a fresh round is re-run from scratch on a new sheet (invalid round → re-round).
- No baseline snapshot is produced from the original invalid round.
- A new snapshot is produced only after a re-round reaches mean-κ ≥ 0.6.
- The protocol document `docs/eval/ko/protocol.md` states this minimal re-round rule plainly.

**DEFERRED to post-V1 (NOT a V1 acceptance item):**
- Joint re-scoring of the 5 lowest-agreement queries.
- Rubric-anchor divergence discussion + FROZEN rubric amendment review by SPEC owner.
- The structured `docs/eval/ko/calibration-log.md` ledger (date / participants / queries discussed / rubric amendments).

Maps to scenario §5.7 in spec.md.

---

### AC-008 — CI artifacts on release tag (non-blocking diff report)

Covers: REQ-EVAL-010

**Given** a release tag commit (e.g., `v1.0.0`).

**When** the CI workflow `.github/workflows/eval-ko.yml` runs.

**Then**:
- Three artifacts are uploaded:
  - (a) Current `tests/eval/korean/golden-set.jsonl` (immutable evidence).
  - (b) Fresh `tests/eval/korean/scoring-sheet-template.csv` (rater-ready).
  - (c) `baseline-diff-report.md` comparing the most recent valid baseline snapshot against the previous one (top-3 recall delta, per-category delta, mean ranking score delta, flagged regressions).
- The workflow does NOT fail on any metric delta (non-blocking per HISTORY D7).
- The diff report is human-readable markdown suitable for release notes.

Maps to scenario §5.8 in spec.md.

---

### AC-009 — Reproducibility dry-run: protocol-only scoring

Covers: NFR-EVAL-001, NFR-EVAL-002, NFR-EVAL-005

**Given** a new native-Korean rater who has never spoken to the SPEC author or any prior rater.

**When** the rater is provided ONLY the 4 documents: `docs/eval/ko/protocol.md`, `docs/eval/ko/rubric.md`, `tests/eval/korean/golden-set.jsonl`, `tests/eval/korean/scoring-sheet-template.csv`.

**Then**:
- The rater can complete a scoring round (or at minimum 1 query end-to-end) WITHOUT asking the SPEC author or any prior rater for clarification.
- The rater's procedural summary (described from scratch) matches the intended workflow per SPEC owner review.
- The protocol document states the throughput estimate (50 queries per rater per round) as a planning aid, NOT a gate.
- The rater recruitment record exists in `docs/eval/ko/rater-pool.md` with anonymous IDs + fluency self-attestation date (no PII).
- The fluency criterion (native or near-native Korean + familiarity with Korean web culture) is stated plainly in the onboarding document.

Maps to scenario §5.9 in spec.md.

---

### AC-010 — PII regression detection in golden-set PRs

Covers: NFR-EVAL-004

**Given** a contributor opens a PR adding an entry to `golden-set.jsonl` containing intentional PII (e.g., `query_text: "박지원 010-1234-5678"`).

**When** the CI PII grep gate runs.

**Then**:
- The gate FAILS the PR.
- The error message identifies the line + matched PII pattern.
- Removing the PII (or using a synthetic, non-PII query) returns the PR to a passing state.
- The gate covers: email regex, Korean phone `010-XXXX-XXXX`, surname+given-name pattern from the name allow/deny list.

Maps to scenario §5.10 in spec.md.

---

## 2. Edge Cases

### EC-001 — Single-rater round attempted (κ gate bypass)

**Given** a contributor submits a scoring round with only 1 rater sheet.

**When** the round aggregator runs.

**Then**:
- The aggregator REFUSES to compute mean-κ (Cohen's κ requires ≥ 2 raters; Light's mean-κ requires ≥ 3 pairwise comparisons → ≥ 3 raters).
- An error message instructs the contributor to recruit ≥ 3 raters and re-submit.
- No snapshot is produced.

### EC-002 — Golden-set SHA256 mismatch between snapshot and current file (DEFERRED post-V1)

(v0.2.0 — nice-to-have, NOT a V1 acceptance item. The `golden_set_sha256` field is still written per AC-006; only the diff-report drift WARNING is deferred.)

**Given** a snapshot file with `golden_set_sha256: "abc..."` but the current `golden-set.jsonl` has SHA256 `xyz...`.

**When** the snapshot-diff report tool runs.

**Then** (post-V1 enhancement):
- The tool emits a WARNING noting the SHA mismatch.
- The diff report header explicitly states the golden-set drifted between snapshots.
- This is informational (golden-set evolution is allowed); the next round produces a new snapshot bound to the new SHA.
- V1 does NOT require this drift WARNING; its absence does NOT fail any V1 criterion.

### EC-003 — Tokenizer version drift detected in snapshot diff (DEFERRED post-V1)

(v0.2.0 — nice-to-have, NOT a V1 acceptance item. The `tokenizer_version` field is still written per AC-006; only the diff annotation is deferred.)

**Given** the previous snapshot has `tokenizer_version: "mecab-ko 0.9.2"` and the current snapshot has `tokenizer_version: "mecab-ko 0.9.3"`.

**When** the diff report is generated.

**Then** (post-V1 enhancement):
- The report flags the tokenizer version change.
- Any per-category recall delta is annotated with "tokenizer change — investigate before attributing to adapter regression".
- The change does NOT block snapshot creation (informational only).
- V1 does NOT require this annotation; its absence does NOT fail any V1 criterion.

---

## 3. Definition of Done Checklist

- [ ] All 10 AC scenarios pass on CI.
- [ ] All 10 scenario index entries (§5.1..§5.10) in spec.md are implemented as automated tests or manual audit reports.
- [ ] `tests/eval/korean/golden-set.jsonl` contains exactly 50 queries with the prescribed category distribution.
- [ ] `tests/eval/korean/scoring-sheet-template.csv` exists with the prescribed header row.
- [ ] `docs/eval/ko/protocol.md` + `docs/eval/ko/rubric.md` + `docs/eval/ko/golden-set-provenance.md` + `docs/eval/ko/rater-pool.md` all exist. (v0.2.0: `docs/eval/ko/calibration-log.md` ledger is DEFERRED post-V1; protocol.md states the minimal "invalid round → re-round" rule instead.)
- [ ] `tests/eval/korean/baseline-snapshots/` directory exists with at least one initial valid snapshot.
- [ ] `kappa.go` unit tests cover the 3 boundary cases (κ=1.0, κ≈0.0, κ∈[0.3, 0.5]).
- [ ] `scoring.go` unit tests cover top-3 recall computation against a known fixture.
- [ ] M3 baseline round achieves top-3 Naver recall ≥ 0.80.
- [ ] CI workflow `.github/workflows/eval-ko.yml` uploads 3 artifacts on release tag without failing builds.
- [ ] PII grep gate verified by intentional PII insertion test.
- [ ] Reproducibility dry-run: new rater completes ≥ 1 query from documents alone.
- [ ] Open Questions in spec.md §8 are resolved or explicitly deferred with mitigation.

---

## 4. Coverage Matrix (REQ → AC)

| REQ / NFR | AC-001 | AC-002 | AC-003 | AC-004 | AC-005 | AC-006 | AC-007 | AC-008 | AC-009 | AC-010 | EC |
|-----------|--------|--------|--------|--------|--------|--------|--------|--------|--------|--------|----|
| REQ-EVAL-001 | ✓ |   |   |   |   |   |   |   |   |   |   |
| REQ-EVAL-002 | ✓ |   |   |   |   |   |   |   |   |   |   |
| REQ-EVAL-003 |   | ✓ |   |   |   |   |   |   |   |   |   |
| REQ-EVAL-004 |   | ✓ |   |   |   |   |   |   |   |   | EC-001 |
| REQ-EVAL-005 |   |   | ✓ |   |   |   |   |   |   |   |   |
| REQ-EVAL-006 |   |   |   | ✓ |   |   |   |   |   |   |   |
| REQ-EVAL-007 |   |   |   |   | ✓ |   |   |   |   |   |   |
| REQ-EVAL-008 |   |   |   |   |   | ✓ |   |   |   |   | EC-002 |
| REQ-EVAL-009 |   |   |   |   |   |   | ✓ |   |   |   |   |
| REQ-EVAL-010 |   |   |   |   |   |   |   | ✓ |   |   |   |
| NFR-EVAL-001 |   |   |   |   |   |   |   |   | ✓ |   |   |
| NFR-EVAL-002 |   |   |   |   |   |   |   |   | ✓ |   |   |
| NFR-EVAL-003 |   |   |   |   |   | ✓ |   |   |   |   |   |
| NFR-EVAL-004 | ✓ |   |   |   |   |   |   |   |   | ✓ |   |
| NFR-EVAL-005 |   |   |   |   |   |   |   |   | ✓ |   |   |

Every REQ and NFR has ≥ 1 AC; edge cases EC-001..EC-003 supplement single-rater bypass, golden-set drift, and tokenizer version change handling.

---

*End of SPEC-EVAL-003 acceptance.md.*
