# Golden-set provenance

SPEC-EVAL-003 · v0.2.0 · REQ-EVAL-002, NFR-EVAL-004

Every query in `tests/eval/korean/golden-set.jsonl` is recorded here with its
source. The set contains **no user-submitted queries, no log-extracted
queries, and no PII**. Sources are limited to synthetic queries authored for
this benchmark and (where applicable) public Naver DataLab trending-keyword
shapes.

## Summary

- KR-001 .. KR-050: **synthetic, authored 2026-05-30** by a native Korean
  speaker for this benchmark, modeled on Naver DataLab public trending-keyword
  _shapes_ (category, length, vocabulary register) without copying any
  individual user query. No personal names, phone numbers, emails, or account
  identifiers appear in any query.

## Per-category notes

| Category      | IDs            | Provenance                                                                             |
| ------------- | -------------- | -------------------------------------------------------------------------------------- |
| news          | KR-001..KR-012 | synthetic, authored 2026-05-30 — generic daily-news shapes (시황/금리/날씨/스포츠 등). |
| blog          | KR-013..KR-022 | synthetic, authored 2026-05-30 — Naver-blog-style how-to/후기 shapes.                  |
| shopping      | KR-023..KR-030 | synthetic, authored 2026-05-30 — product-comparison shapes (no brand-specific PII).    |
| academic-tech | KR-031..KR-038 | synthetic, authored 2026-05-30 — arXiv/GitHub-targeting technical shapes.              |
| code-mixed    | KR-039..KR-044 | synthetic, authored 2026-05-30 — 한영 혼용 (5:5 and 7:3 ratios).                       |
| cultural      | KR-045..KR-050 | synthetic, authored 2026-05-30 — Korean cultural/holiday shapes.                       |

## PII guarantee

The CI PII grep gate (`.github/workflows/eval-ko.yml`) and the Go test
`TestGoldenSetFile_NoPII` enforce zero matches for:

- email regex
- Korean mobile phone `01[016789]-XXXX-XXXX`
- (CI also screens common surname+given-name patterns from the name deny list)

A PR adding any PII to the golden set fails the gate.
