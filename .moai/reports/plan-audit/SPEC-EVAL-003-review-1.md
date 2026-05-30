# SPEC Review Report: SPEC-EVAL-003 (v0.2.0)
Iteration: 1/3
Verdict: PASS-WITH-FINDINGS
Overall Score: 0.88

> Reasoning context ignored per M1 Context Isolation. The amendment's self-report (B1 fix claim, scope reductions) was NOT trusted; every claim was verified against the spec.md / plan.md / acceptance.md text and against LIVE code (`naver.go`, `koreanews.go`, `category.go`).

## Must-Pass Results

- **[PASS] MP-1 REQ number consistency**: `REQ-EVAL-001`..`REQ-EVAL-010` sequential, 3-digit zero-padded, no gaps/dupes (spec.md:237–261). `NFR-EVAL-001`..`NFR-EVAL-005` sequential (spec.md:269–273). No duplicates.
- **[PASS] MP-2 EARS format compliance**: All 10 REQs match an EARS pattern — Ubiquitous (001/002/003/005/006/008), Event-Driven (004 "WHEN…SHALL" L245, 009 "WHEN…SHALL" L260), Optional (010 "WHERE…SHALL" L261), Unwanted/If-Then (007 "IF…THEN…SHALL" L253). Minor: REQ-EVAL-007 is *labeled* "State-Driven" but uses If/Then syntax (the Unwanted pattern). The form is valid EARS; only the label is imprecise → minor finding D2, not a must-pass failure.
- **[PASS] MP-3 YAML frontmatter validity**: id (SPEC-EVAL-003), version ("0.2.0"), status ("draft"), created (2026-05-22, ISO), priority ("P1") all present with correct types (spec.md:2–17). `labels` per the generic rubric is absent; the project schema substitutes `milestone`/`related`/`priority` consistently across sibling SPECs → noted as D5 (project-consistent, not a real defect).
- **[N/A] MP-4 Section 22 language neutrality**: N/A — single-domain Korean-locale eval SPEC, not multi-language tooling. Auto-pass.

## B1 Verification — THE CRUX (verified against LIVE code)

**Result: CONFIRMED-RESOLVED.**

| Claim | Spec evidence | Live-code evidence | Status |
|-------|---------------|--------------------|--------|
| recall gate keyed on single `naver` SourceID | REQ-EVAL-005 spec.md:251; AC-003 acceptance.md:68–83 | `naver.go:181` `SourceID:"naver"` | CONFIRMED |
| verticals via `naver_vertical` filter ∈ {blog,news,web,shop,datalab}, NOT separate adapters | spec.md:251, 398; REQ-EVAL-001 enum spec.md:237 | `naver.go:181,183–197` (`DocTypePost→blog`, `DocTypeArticle→news`, `DocTypeOther→{web,shop,datalab}`); Notes L194–197 confirm `Filters[naver_vertical]` | CONFIRMED |
| `naver-academic` deleted; no academic vertical | spec.md:34,42; REQ-EVAL-001 NOTE spec.md:237 | `naver.go:183–187` DocTypes carry NO academic | CONFIRMED |
| `koreanews` single SourceID | spec.md:39,210; §6.1 L403 | `koreanews.go:85` `SourceID:"koreanews"` | CONFIRMED |
| `daum-news`/`korea-news-crawler` separate IDs do not exist | spec.md:36,48,406 | `koreanews.go:85` single composite ID | CONFIRMED |
| §1.3 / D4 corrected | §1.3 L206–213; D4 L102–111 | — | CONFIRMED |

1. **REQ-EVAL-005 implementable?** YES. Gate = `recall@3 = Naver-hit / expected_naver_relevant`, Naver-hit = ≥1 top-3 result with `SourceID=="naver"` (+ optional vertical-via-DocType). Threshold 0.80. Concretely computable from the real result model.
2. **REQ-EVAL-001 expected_sources constrained?** YES — "valid values are exactly the registered SourceIDs"; phantom strings explicitly PROHIBITED (spec.md:237); `expected_naver_vertical` enum excludes `academic` (spec.md:237, AC-001 L38).
3. **REQ-EVAL-008 adapter_versions real IDs?** YES — keyed on registered SourceIDs, phantoms "NOT" allowed (spec.md:259; AC-006 L136).
4. **§1.3 / D4 corrected?** YES (spec.md:48, 102–111, 206–213).
5. **Recall still meaningful (non-degenerate)?** YES, with a disclosed soft edge — see Finding D1.

## Residual Phantom IDs (positive uses)

**NONE.** Every occurrence of `naver-news`/`naver-blog`/`naver-shopping`/`naver-academic`/`daum-news`/`korea-news-crawler` is in NEGATIVE/PROHIBITIVE context:
- spec.md:30,34,36,42,48,109–110,211–212,400,405 — all "존재하지 않는다 / 정정 / PROHIBITED / 분리 ID는 코드에 존재하지 않는다".
- plan.md:142 — "분리 `naver-news` ID는 사용하지 않는다" (negative).
- acceptance.md:37,83,136 — "produce a schema-validation FAILURE / rejected / does NOT appear".

No phantom ID is used as a positive key anywhere in the scoring/snapshot/golden-set schema.

## Reductions Coherence: CLEAN

| Deferred item | Gating part kept? | Deferred part non-gating? | Evidence |
|---------------|-------------------|---------------------------|----------|
| Krippendorff α | κ (Cohen/Light ≥ 0.6) kept as V1 gate (REQ-004) | α not in any AC; Exclusions §4 L346 | CLEAN |
| Calibration ceremony | minimal "invalid→re-round" gated (AC-007 V1) | ceremony explicitly "NOT a V1 acceptance item" (acceptance.md:160–164) | CLEAN |
| per-category 0.10 warning | `per_category` map populated & tested (AC-004) | warning "gates NOTHING" (acceptance.md:100) | CLEAN |
| EC-002 SHA / EC-003 tokenizer | `golden_set_sha256` + `tokenizer_version` fields kept (AC-006 L136) | drift WARNING deferred, "absence does NOT fail any V1 criterion" (acceptance.md:251,265) | CLEAN |

**No V1 acceptance criterion gates on a deferred REQ.** The snapshot schema retains the two fields the amendment promised to keep.

## Manual-Protocol + Design-Risk Disclosure

**Manual protocol: COHERENT.** D1 manual-only + D7 CI non-blocking (artifact-only) + quarterly once-per-cycle baseline + no per-PR scoring. AC-008 confirms CI uploads 3 artifacts with no failure path (acceptance.md:182). EC-001 blocks single-rater κ-bypass (acceptance.md:228–237). Exclusions §4 reinforce (L334–339).

**Design-risk disclosure: PRESENT but the silent-window framing is IMPLICIT (Finding D4).** The mechanism (quarterly manual, CI non-blocking, REL-001 gated on a one-time offline baseline) is disclosed in HISTORY D1/D7, §1.1–1.2, §4 Exclusions, and AC-008. However, the SPEC does NOT explicitly name the residual risk that *a Korean-ranking regression introduced between quarterly rounds goes undetected by automated CI until the next manual round*. §1.2 frames artifacts as "회귀 시 진단 자료 즉시 확보" (diagnostic AFTER detection) — which does not detect the regression. For a SPEC that gates the V1 release (REL-001), making this latency window an explicit accepted-limitation statement is advisable. Not hidden; improvable.

## EVAL-001 Non-Duplication: CONFIRMED

Same JSONL *format convention* but separate files, distributions, gates; schema unification explicitly deferred to a future refactor SPEC and "intentionally separated in V1" (spec.md:219–220, 341–344, §6.2 L420–422). No shared golden set.

## Defects Found

- **D1. acceptance.md:77 / spec.md:251 — `DocTypeOther→{web,shop,datalab}` is a 1-to-3 ambiguous mapping** — the shopping/web/datalab verticals are mutually indistinguishable from `DocType` alone, so any `expected_naver_vertical: shop` assertion (shopping bucket = 8 queries, plan.md:151) silently degrades to the SourceID-only "unverified" fallback. The fallback is disclosed, but the SPEC never states that shop/web/datalab vertical assertions are effectively unverifiable. Recall stays non-degenerate at SourceID + per-category level, so this is a craft finding, not a blocker. — Severity: minor
- **D2. spec.md:253 — REQ-EVAL-007 labeled "State-Driven" but written as "IF…THEN…SHALL"** (the Unwanted/If-Then pattern). Form is valid EARS; the pattern label is imprecise. — Severity: minor
- **D3. spec.md:466 vs acceptance.md:175 — CI workflow filename mismatch**: spec §7.1 says `.github/workflows/korean-eval.yml`; acceptance AC-008 says `.github/workflows/eval-ko.yml`. Run phase needs one canonical name. — Severity: minor
- **D4. design-risk: silent-regression-window not explicitly framed as an accepted residual limitation** (see disclosure section). For a REL-001-gating SPEC, add one explicit sentence. — Severity: minor
- **D5. frontmatter has no generic `labels` field** — project schema substitutes `milestone`/`related`/`priority`; consistent across sibling SPECs. Informational only. — Severity: minor

No critical or major defects. No must-pass failures.

## Chain-of-Verification Pass

Second-look findings, verified by re-reading: (a) every phantom-ID hit across all 3 docs via grep — confirmed 100% negative-context; (b) full Coverage Matrix (acceptance.md:289–305) — every REQ-001..010 and NFR-001..005 has ≥1 AC, no orphan AC, no AC references a non-existent REQ → traceability CLEAN; (c) each deferred item cross-checked against its AC to confirm no V1 gate rests on it → CLEAN; (d) live-code line numbers independently confirmed (`naver.go:175,181`; `koreanews.go:79,85`; `category.go:18–21` `CategoryKorean`/`CategoryMixed` exist for `expected_router_class`). New defect surfaced in second pass: D3 (CI filename mismatch) — added above. First pass otherwise thorough.

## EARS Findings / Traceability Gaps

- EARS: 10/10 REQs in valid patterns; one label imprecision (D2). No informal/weasel ACs detected; observational targets (e.g. mean code-switching ≥ 4/5) are explicitly marked "NOT gating" (acceptance.md:120).
- Traceability: ZERO gaps. No orphan AC, no uncovered REQ/NFR, no AC pointing at a deferred-only requirement.

## Must-Fix Before Implementation

(empty — no blocking defects)

## Recommendation

The HARD blocker B1 is genuinely resolved against live code: the gate metric, golden-set schema, and snapshot schema are now built on the real single-`naver` + `naver_vertical`-filter and single-`koreanews` model, with zero positive phantom-ID uses. Scope reductions are internally coherent — every deferred item is non-gating and the promised retained fields (`golden_set_sha256`, `tokenizer_version`, `per_category`) are kept and tested. Traceability and EARS are clean.

**status_transition_recommendation: approve.** The five findings (D1–D5) are all minor and SHOULD-fix, not MUST-fix; they can be absorbed in the run phase (canonicalize the CI filename; relabel REQ-007; add one residual-risk sentence; note the shop/web/datalab indistinguishability in REQ-EVAL-005). None block Phase 0 gate or REL-001 unblocking.
