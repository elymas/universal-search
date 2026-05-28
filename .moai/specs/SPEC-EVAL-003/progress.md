# SPEC-EVAL-003 Progress

## Session 1 — TDD Phase 3 + Phase 4 (2026-05-27)

### Completed

- **Phase 3 (Scoring sheet template + kappa calculator)** — TDD RED-GREEN-REFACTOR
  - `internal/eval/korean/loader.go` — Golden set JSONL loader with schema validation
  - `internal/eval/korean/scoring.go` — Top-3 Naver recall, MRR@10, per-category recall
  - `internal/eval/korean/kappa.go` — Cohen's kappa, Light's mean-kappa, Krippendorff alpha ordinal
  - `internal/eval/korean/loader_test.go` — 8 tests
  - `internal/eval/korean/scoring_test.go` — 11 tests
  - `internal/eval/korean/kappa_test.go` — 14 tests

- **Phase 4 (Snapshot writer + retention policy)** — TDD RED-GREEN-REFACTOR
  - `internal/eval/korean/snapshot.go` — Baseline snapshot JSON serialization, append-only policy, retention (4 max)
  - `internal/eval/korean/snapshot_test.go` — 7 tests

- **Phase 2 artifacts (golden set curation)**
  - `tests/eval/korean/golden-set.jsonl` — 50 queries x 6 categories (synthetic queries)
  - `tests/eval/korean/scoring-sheet-template.csv` — Rater input template per REQ-EVAL-003
  - `tests/eval/korean/baseline-snapshots/.gitkeep` — Empty directory placeholder
  - `tests/eval/korean/baseline-snapshots/archive/.gitkeep` — Archive directory placeholder

### Metrics

- Total tests: 40 (all passing)
- Race detector: clean (0 warnings)
- Coverage: 85.7% (target: 85%)
- `go vet`: clean
- Build: clean

### Test Inventory

| Component | Tests | Status |
|-----------|-------|--------|
| loader | 8 | PASS |
| scoring | 11 | PASS |
| kappa | 14 | PASS |
| snapshot | 7 | PASS |

### REQ Coverage

| REQ | Status | Notes |
|-----|--------|-------|
| REQ-EVAL-001 | Partial | Loader validates golden set schema; golden-set.jsonl created with correct distribution |
| REQ-EVAL-002 | Deferred | Provenance doc to be created in protocol phase |
| REQ-EVAL-003 | Partial | scoring-sheet-template.csv created; protocol docs pending |
| REQ-EVAL-004 | Implemented | CohenKappa + LightMeanKappa + KrippendorffAlphaOrdinal |
| REQ-EVAL-005 | Implemented | Top3NaverRecall + MRRAt10 |
| REQ-EVAL-006 | Implemented | PerCategoryRecall |
| REQ-EVAL-007 | Partial | Router class accuracy computed; full code-mixed validation pending golden set fixtures |
| REQ-EVAL-008 | Implemented | WriteSnapshot with all fields, append-only, retention |
| REQ-EVAL-009 | Deferred | Calibration protocol docs pending |
| REQ-EVAL-010 | Deferred | CI workflow pending |

### Deferred to Session 2

- Phase 1: Protocol docs (protocol.md, rubric.md, onboarding.md, kappa-interpretation.md, calibration-log.md, rater-pool.md, golden-set-provenance.md)
- Phase 5: CI workflow (.github/workflows/korean-eval.yml)
- Phase 6: Baseline run (requires 3 human raters — operational)
- Phase 7: Sync (README, CHANGELOG, PR)

### Files Created

```
internal/eval/korean/loader.go
internal/eval/korean/loader_test.go
internal/eval/korean/scoring.go
internal/eval/korean/scoring_test.go
internal/eval/korean/kappa.go
internal/eval/korean/kappa_test.go
internal/eval/korean/snapshot.go
internal/eval/korean/snapshot_test.go
tests/eval/korean/golden-set.jsonl
tests/eval/korean/scoring-sheet-template.csv
tests/eval/korean/baseline-snapshots/.gitkeep
tests/eval/korean/baseline-snapshots/archive/.gitkeep
docs/eval/ko/ (empty, awaiting protocol docs)
```
