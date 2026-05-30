# SPEC-EVAL-003 Tasks — atomic decomposition

Status: draft (Phase 1 analysis output, manager-strategy 2026-05-30)
Methodology: TDD | Harness: standard | Coverage target: 85%

These tasks decompose the plan.md phases into atomic units, each
completable in one TDD cycle. Ordering follows dependency.

> [HARD BLOCKER] Tasks T03/T04/T08 reference the adapter-ID model from
> the SPEC (`naver-news`/`naver-blog`/`naver-shopping`/`naver-academic`).
> Live code exposes a SINGLE adapter `SourceID: "naver"` with verticals
> distinguished by `DocType` + request `Filters[naver_vertical]`. The
> SPEC's gating metric is unimplementable as written. This must be
> resolved (SPEC amendment) before T03 onward. See progress.md
> "Asset reality" + risk R1.

| ID | Description | REQ mapping | Depends on | Acceptance |
|----|-------------|-------------|------------|------------|
| T01 | Author `docs/eval/ko/` protocol + rubric + onboarding + kappa-interpretation + calibration-log template + rater-pool template (6 docs). Reproducibility dry-run. | REQ-EVAL-003, REQ-EVAL-009, NFR-EVAL-001/002/005 | — | 6 docs published; external-reader dry-run summary matches intended workflow (NFR-EVAL-001). |
| T02 | Curate 50-query golden set + provenance doc. PII grep gate wired (CI pre-check). | REQ-EVAL-001, REQ-EVAL-002, NFR-EVAL-004 | T01 (rubric anchors inform curation) | 50 objects, 12/10/8/8/6/6 distribution exact, provenance 100%, PII grep 0 matches. |
| T03 | RED+GREEN `loader.go` + `loader_test.go`: JSONL → []GoldenQuery, schema/distribution/required-field validation. | REQ-EVAL-001 | T02 (needs the schema confirmed), **R1 resolved** | TestLoadGoldenSet_50Objects, _CategoryDistribution, _AllRequiredFields, _InvalidJSON pass. |
| T04 | RED+GREEN `scoring.go` + `scoring_test.go`: top-3 Naver recall, MRR@10, per-category recall. **Recall over the corrected Naver-source model (see R1).** | REQ-EVAL-005, REQ-EVAL-006 | T03, **R1 resolved** | recall fixtures pass; per-category map for 6 buckets. |
| T05 | RED+GREEN `kappa.go` + `kappa_test.go`: Cohen κ, Light mean-κ, Krippendorff α aux, ≥3-rater enforcement. | REQ-EVAL-004 (+EC-001) | T03 | identical→1.0, random→~0.0, divergent→0.3–0.5; <3 raters errors. |
| T06 | Author `scoring-sheet-template.csv` with prescribed 9-column header. | REQ-EVAL-003 | T01 | CSV header lint passes; field constraints documented in protocol. |
| T07 | RED+GREEN `snapshot.go` + `snapshot_test.go`: append-only writer, ≥0.6 gate, 4-retention+archive, SHA256, full field set. | REQ-EVAL-008, REQ-EVAL-009, NFR-EVAL-003 (+EC-002/003) | T04, T05 | valid→writes, invalid→no write, overwrite rejected, retention keeps 4. |
| T08 | CI workflow `.github/workflows/eval-ko.yml`: PII gate + `go test` + 3 artifacts (golden-set, sheet template, baseline-diff-report) on release tag, non-blocking. | REQ-EVAL-010 (+ NFR-EVAL-004 gate) | T02, T07 | smoke test: 3 artifacts uploaded, build success, no fail path. |
| T09 | Baseline run on M3 stack: recruit 3 raters, run round, compute κ≥0.6, verify top-3 Naver recall ≥0.80, write `v1.0.0.json`. | REQ-EVAL-005, REQ-EVAL-008, blocks SPEC-REL-001 | T01–T07, **R1 resolved**, all deps implemented | valid round, recall ≥0.80, snapshot created. OPERATIONAL — not pure code. |
| T10 | Sync: README eval section, CHANGELOG entry, PR, status draft→implemented. | — | T09 | PR with spec/plan/acceptance links + baseline link. |

## Notes on filename drift (NIT, not blocking)
- spec.md §7 says CI file is `.github/workflows/korean-eval.yml`;
  acceptance.md AC-008 says `.github/workflows/eval-ko.yml`. Pick one
  in run phase (T08). Recommend `eval-ko.yml` (acceptance is the
  contract surface).

## Task count: 10 (within the 10-task ceiling).
