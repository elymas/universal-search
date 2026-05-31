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

---

## RUN PHASE — TDD Session 1 (2026-05-30, manager-tdd)

Branch: feature/SPEC-EVAL-003 | Methodology: TDD | Harness: standard

### Tasks completed (8 of 10 — T09/T10 operational/sync, out of code scope)

| Task | Status | Notes |
|------|--------|-------|
| T01 docs (protocol/rubric/onboarding/kappa-interp/provenance/rater-pool) | DONE | 6 rater-facing docs under docs/eval/ko/. Minimal re-round path stated; calibration-log ledger deferred post-V1. |
| T02 golden set + provenance + PII | DONE | 50 queries, 12/10/8/8/6/6 exact, synthetic (no PII), real SourceIDs only. |
| T03 loader.go (RED→GREEN→REFACTOR) | DONE | JSONL→[]GoldenQuery, schema validation, phantom-ID rejection tested. |
| T04 scoring.go | DONE | top-3 Naver recall (SourceID=="naver" + DocType→vertical, DocTypeOther→unverified fallback), MRR@10, per-category. |
| T05 kappa.go | DONE | Cohen κ pairwise + Light mean-κ, ≥3-rater gate, ≥0.6 validity. NO Krippendorff (deferred). |
| T06 scoring-sheet CSV + snapshot SCHEMA + placeholder | DONE | 9-col header lint test; v1.0.0.example.json placeholder (zeroed) + SCHEMA.md. |
| T07 snapshot.go | DONE | append-only (ErrSnapshotExists), invalid-round refusal, SHA256, 4-retention+archive, phantom adapter rejection. |
| T08 CI eval-ko.yml | DONE | NON-BLOCKING (continue-on-error everywhere): schema validate + PII grep + go test + 3 artifacts. |
| T09 baseline v1.0.0.json | DEFERRED (operational) | Needs real 3-rater offline round. Placeholder + SCHEMA shipped only. |
| T10 sync (README/CHANGELOG/PR) | PENDING | Orchestrator handles commit + sync. |

### Test evidence
- `go build ./...` OK, `go vet ./internal/eval/korean/...` OK
- `go test -race -cover ./internal/eval/korean/...` → ok, 44 tests pass, coverage 88.8% (target 85%)
- golden-set.jsonl parses + schema-validates (50 queries, exact distribution), PII grep 0 matches, 0 phantom IDs.

### Scope adherence (HARD)
- expected_sources: REAL SourceIDs only; loader REJECTS phantom IDs (6 phantoms tested rejected).
- Cohen/Light κ only — Krippendorff ABSENT.
- baseline v1.0.0: schema + placeholder example ONLY (real recording deferred to rater round).
- CI eval-ko.yml NEVER fails build (every step continue-on-error / exit 0).
- EVAL-001 citation harness NOT reimplemented — independent package internal/eval/korean.

### MX tags added
- loader.go: @MX:ANCHOR (golden-set schema/phantom gate +REASON), 2× @MX:NOTE (distribution, allowlist)
- scoring.go: @MX:ANCHOR (top-3 recall release gate +REASON)
- kappa.go: @MX:ANCHOR (inter-rater κ gate +REASON)
- snapshot.go: @MX:NOTE (append-only writer)

### Acceptance criteria met this session
AC-001 (schema+PII), AC-002 (κ), AC-003 (recall gate), AC-004 (per-category), AC-006 (snapshot+retention), AC-007 (invalid→re-round), AC-008 (CI artifacts), AC-010 (PII grep), EC-001 (single-rater reject) — automated/code portions.
AC-005 (code-switching) docs + struct field present; AC-009 (repro dry-run) docs present — manual verification deferred to rater round.
