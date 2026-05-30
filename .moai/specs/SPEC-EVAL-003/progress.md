# SPEC-EVAL-003 Progress

## 2026-05-30 — Phase 1 (manager-strategy analysis)

Phase: Analysis & Planning (no code written; tasks.md authored).
Harness: standard (confirmed). Methodology: TDD. Coverage target: 85%.

### Dependency verification (all `status: implemented`)
- SPEC-IR-001 — router `korean`/`mixed` categories live in
  `internal/router/category.go:19-21` (CategoryKorean="korean",
  CategoryMixed="mixed"). Confirmed real.
- SPEC-ADP-008 — Naver adapter at `internal/adapters/naver/naver.go`,
  `Name()`/`SourceID` == **"naver"** (single ID), multi-vertical via
  `Filters[naver_vertical]` (blog/news/web/shop/datalab).
- SPEC-ADP-009 — `internal/adapters/koreanews/` adapter,
  `Name()`/`SourceID` == **"koreanews"** (single ID).
- SPEC-IDX-003 — mecab-ko sidecar at `services/tokenizer-ko/`
  (Dockerfile + pyproject `mecab-ko>=1.0,<2.0`); Go client
  `internal/index/tokenizer/`. Confirmed real.
- SPEC-SYN-002 — implemented.

### Asset reality — STALE ADAPTER-ID MODEL (blocker)
The SPEC's central gating metric (REQ-EVAL-005) and snapshot schema
(REQ-EVAL-008) assume four distinct Naver adapter IDs:
`naver-news`, `naver-blog`, `naver-shopping`, `naver-academic`.
Grep of `internal/` returns ZERO matches for any of these.
Reality: one `SourceID: "naver"` (naver.go:181, parse.go:189/256/328/394);
verticals are `DocType` (Post/Article/Other) + request filter, not IDs.
`naver-academic` has NO basis at all — the adapter has no academic
vertical. `daum-news`/`korea-news-crawler` (spec.md §1.3) are also
fictional; the only Korean-news adapter ID is `koreanews`.

This matches the recurring repo pattern (see manager-strategy memory
`feedback_spec-stale-paths`): M8-band SPECs cite IDs/paths written
ahead of code. The capability exists; the IDs in the SPEC do not.

### EVAL-001 overlap
Complementary, NOT duplicate. EVAL-001 spec.md:51-54 + 231-234
explicitly cedes the Korean-first benchmark to EVAL-003 ("Korean
coverage deliberately under-weighted because SPEC-EVAL-003 owns it").
EVAL-001 = semantic citation faithfulness (DeepEval, automated, gates
≥0.85). EVAL-003 = Korean ranking/retrieval relevance (manual). No
shared golden-set or scoring infra (the SPEC intentionally keeps
schemas separate — Exclusions §4 last bullet).

### Manual-protocol decision
Automated (TDD surface): loader, scoring (recall/MRR/per-category),
kappa, snapshot writer, CI artifact + PII gate. Manual (human):
the rater scoring itself (3 raters fill sheets). CI gate is
NON-BLOCKING (artifact-only, HISTORY D7) — no live human in CI. The
"gate" (top-3 recall ≥0.80) runs once per quarter offline against
recorded sheets; the recorded baseline snapshot is the durable
artifact, not a per-PR check.

### Phase 0
No plan-audit report present. Harness standard → plan_audit.enabled:
true. plan-auditor REQUIRED before run. Status still `draft`.

### Recommendation: needs-plan-auditor-first (+ adapter-ID amendment)
