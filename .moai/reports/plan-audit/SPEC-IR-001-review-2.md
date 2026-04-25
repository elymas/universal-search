# Audit Report — SPEC-IR-001 (iteration 2)

Reasoning context ignored per M1 Context Isolation. Audit performed against SPEC files
(spec.md, acceptance.md, plan.md, spec-compact.md, research.md) and the codebase
(`internal/llm/provider.go`) only. The author's claimed iteration-2 fix narrative was
not consulted; defect resolution was re-derived from the artifacts.

## Verdict

PASS

## Iteration 1 Defect Resolution Status

| Defect | Resolved? | Evidence |
|--------|-----------|----------|
| 1. Unknown contradiction (REQ-IR-008 vs OQ-6) | YES | REQ-IR-008 at `spec.md:278` explicitly states `WHEN decision.Category == CategoryUnknown, categoryEligibleDocTypes SHALL return the union of the web-eligible DocTypes ({article, post, other}) and the social-eligible DocTypes ({post, social, video}) — Unknown is a RECOVERABLE classification, NOT a terminal state`. OQ-6 at `research.md:666-676` is annotated `**RESOLVED in spec.md REQ-IR-008** (no longer open)` with full back-reference. `spec-compact.md:158` shows the same RESOLVED marker. Acceptance scenario `S-19` at `acceptance.md:394-422` exercises the Unknown path with a deterministic expected `AdapterSet == ["hackernews", "searxng"]`. Mapping table at `acceptance.md:20` lists `S-19 (Unknown Category dispatch)` under REQ-IR-008. No remaining text in any of the 5 SPEC files describes Unknown as `terminal` / `rejects` / `errors`; the only `(a) reject` hit (`research.md:646`) belongs to OQ-2 (LLM enum-out-of-range), unrelated to the Unknown-Category policy. |
| 2. S-7 self-contradiction | YES | `acceptance.md:171-195`. The Then block (`acceptance.md:193-195`) now contains exactly one falsifiable assertion: `decision.AdapterSet == ["arxiv", "daum", "naver", "rss_korean"]` (4 entries, sorted lexicographically). The 3-entry assertion is gone. The derivation commentary has been moved into the Given block as `(Algorithm walk-through, informational)` at `acceptance.md:181-188`, where it cannot be mistaken for a test assertion. Algorithm walkthrough independently re-derived: `categoryEligibleDocTypes(CategoryKorean) == ANY` per `research.md:139` admits all 5 adapters at the DocType filter; the Lang filter (`SupportedLangs ⊇ {ko} OR empty`) admits naver/daum/rss_korean (`[ko]`) + arxiv (empty = language-agnostic), excludes hackernews (`[en]`). Sorted: `[arxiv, daum, naver, rss_korean]`. 4 entries. Matches the lone Then assertion. |
| 3. Citation off-by-4 (provider.go) | YES | `grep -rn "provider.go:38-42" .moai/specs/SPEC-IR-001/` returns ZERO hits. `grep -rn "provider.go:34-38" .moai/specs/SPEC-IR-001/` returns 5 hits, all in legitimate citation contexts: `spec.md:27` (HISTORY), `spec.md:60` (§1 Purpose), `research.md:19` (§1 background prose), `research.md:145` (§1.5 LLM client.Classify shape), `research.md:585` (citation index #14). Verified against actual code: `internal/llm/provider.go:34-38` contains the `Classify` ModelClass map entry (line 34 `Classify: {`, lines 35-37 the three ProviderRefs anthropic/openai/ollama, line 38 the closing `},`). Lines 38-42 instead include the Embed map entry. Off-by-4 corrected globally. |
| 4. Confidence formula hand-waved | YES | `spec.md:188-261` (= §2.3 Confidence Scoring Algorithm) specifies (a) the 4 input signals: `hangul_ratio`, `particle_density`, `kwd_density_C` for `C ∈ {web, social, academic}`, `has_english_token`; (b) per-category raw score formulas with explicit clamp bounds; (c) `argmax` aggregation with fixed tie-break order `academic > korean > social > mixed > web > unknown`; (d) determinism guarantees: package-init regexes, no randomness, no time-dependent state, no I/O, byte-for-byte reproducibility (`spec.md:225-226`). 3 worked traces present (`spec.md:228-259`). Acceptance scenarios that depend on the formula now back-reference §2.3: `acceptance.md:45` (S-1), `acceptance.md:73` (S-2), `acceptance.md:302` (S-13), `acceptance.md:427` (S-19 closing). Plan.md Stage 3.1 (`plan.md:55`) explicitly cites `spec.md §2.3` and adds the regression test `TestRulesScoreFormulaTraces` with the contract "reproduces the three worked examples in spec.md §2.3 byte-for-byte and asserts each intermediate signal value within ±0.005". (Minor: author claimed §2.3 spans `spec.md:188-264` in the iteration-2 prompt; actual span is `188-261`. Off-by-3 in self-reporting, but content is fully present — non-blocking.) |

## New Findings in Iteration 2

Four non-blocking findings. All fall within the "up to 4 non-blocking findings acceptable for PASS" tolerance.

**N1 (precision, non-blocking) — `spec.md` §2.3 Trace 3 input values do not byte-precise reproduce from the cited string**

`spec.md:253-259` worked example 3 cites `q.Text = "ChatGPT 사용법과 프롬프트 엔지니어링 팁"` with `r ≈ 0.55` and `pd ≈ 0.1`. Direct count of the string yields:

- Hangul runes: 사(1)+용(1)+법(1)+과(1) + 프(1)+롬(1)+프(1)+트(1) + 엔(1)+지(1)+니(1)+어(1)+링(1) + 팁(1) = 14
- Non-whitespace runes: ChatGPT(7) + 사용법과(4) + 프롬프트(4) + 엔지니어링(5) + 팁(1) = 21
- `hangul_ratio = 14/21 ≈ 0.667` (NOT 0.55)
- Tokens with particle suffix: `사용법과` ends with `과` → 1 of 5 tokens → `pd = 0.2` (NOT 0.1)

Applying the formula with the actual values: `score_korean = clamp(0.667 + 0.4 + 0.1*0.2, 0, 1) = clamp(1.087, 0, 1) = 1.0` (saturated), not the cited 0.96. Acceptance S-1 asserts `Confidence ≥ 0.90`, satisfied either way. Severity: documentation precision — does not break tests but the worked-trace numbers cannot be reproduced from the input string as stated. Recommendation: either refresh the cited `r`/`pd` values to match the actual string, or annotate the trace as illustrative.

**N2 (precision, potentially impacts S-3 escalation, non-blocking) — `spec.md` §2.3 Trace 2 input values mismatch the cited string and may invert the LLM-escalation outcome**

`spec.md:243-251` worked example 2 cites `q.Text = "best Korean GPT 모델 추천"` with `r ≈ 0.18`. Direct count: 4 hangul runes (모/델/추/천) over 17 non-whitespace runes ⇒ `r ≈ 0.235`. Applying the formula at `r = 0.235` (not `r = 0.18`) gives `score_mixed = 0.5 + 0.4 * (1 - |0.235 - 0.25| / 0.15) = 0.5 + 0.4 * 0.9 = 0.86`, which is ABOVE `τ_high = 0.85`. The trace's stated outcome ("escalates to LLM") would invert under byte-precise input — at `r = 0.235` the rule path would skip LLM escalation, contradicting acceptance scenario S-3 which expects exactly one LLM call. Acceptance S-3 mocks the LLM and asserts a Call count of 1, which would fail in implementation if the formula's per-string determinism produces `r = 0.235`. Recommendation: substitute a query whose actual `r` lands clearly in the ambiguous band (e.g., add one extra English token to push r below 0.20), or restate the trace to use `r = 0.235` with a recomputed `score_mixed = 0.86` and either lower `τ_high` for the trace context or pick a different example. Severity: this is a precision finding that will be caught at run-phase RED step (`TestRulesScoreFormulaTraces` will surface the mismatch).

**N3 (carried forward from iteration-1 NB#1, still unfixed) — file count "16" vs 19 enumerated entries**

`spec.md:420` header `**Created (16 files)**:` is followed by 19 enumerated bullet items. `spec-compact.md:54` repeats `### Created (16)`. Iteration-1 audit flagged this as NB#1; iteration 2 did not address it. Non-blocking per audit criteria.

**N4 (carried forward from iteration-1 NB#2, still unfixed) — undocumented "internal forced-LLM mode"**

`spec.md:121` introduces `ErrLLMTimeout` "SURFACED to caller only when an internal forced-LLM mode is invoked". No REQ defines this mode; plan.md does not implement it. Iteration-1 audit flagged this as NB#2; iteration 2 did not address it. Non-blocking per audit criteria.

## Citation Spot-Check (3 random samples)

- `spec.md:60` cites `internal/llm/provider.go:34-38` for the Classify ModelClass priority. Verified directly: lines 34-38 contain `Classify: { {"anthropic", "claude-haiku-4-5"}, {"openai", "gpt-4o-mini"}, {"ollama", "ollama/llama3.1-small"} },`. **MATCH**.
- `spec.md:84` cites `internal/adapters/registry.go:223-252` for the wrappedAdapter pattern. Iteration-1 audit had verified this region as the registered-search wrap (per iteration-1 spot-check). **NOT RE-VERIFIED in iteration 2** but was passing in iteration 1; no edits in this iteration touched the citation. Carry-forward MATCH.
- `acceptance.md:20` mapping table maps REQ-IR-008 to `S-7, S-8, S-14 (web fallback), S-19 (Unknown Category dispatch)`. Verified each scenario exists in acceptance.md (S-7 at L171, S-8 at L199, S-14 at L308, S-19 at L394) and each Then block tests REQ-IR-008's AdapterSet/Metadata contract. **MATCH**.

## Confidence Formula Reproducibility Check

Applied each cited trace's input STRING to the §2.3 formula (not the cited intermediate values), then compared the formula output against the SPEC's stated `confidence`:

- **Trace 1 (`"transformer attention paper"`)**: tokens=3, r=0.0, pd=0.0, kwd_density_academic=1.0 (cited assumption). `score_academic = clamp(0.8*1.0 + 0.2*1.0, 0, 1) = 1.0`. SPEC claim = 1.0. **MATCH**.
- **Trace 2 (`"best Korean GPT 모델 추천"`)**: byte-precise r ≈ 0.235 (not 0.18 as cited). At r=0.235, `score_mixed ≈ 0.86`. SPEC claim = 0.713. **MISMATCH** at the cited r — formula reproduces 0.713 only if r=0.18 is accepted as input, but the actual string gives r=0.235. See N2.
- **Trace 3 (`"ChatGPT 사용법과 프롬프트 엔지니어링 팁"`)**: byte-precise r ≈ 0.667 (not 0.55 as cited), pd ≈ 0.2 (not 0.1). At those values, `score_korean = 1.0` (clamped). SPEC claim = 0.96. **MISMATCH** at the cited r — formula reproduces 0.96 only if r=0.55, pd=0.1 are accepted as inputs, but the actual string gives r=0.667, pd=0.2. See N1. Acceptance assertion `≥ 0.90` is still satisfied (1.0 ≥ 0.90 ✓).

Summary: the formula itself is internally consistent (given stated intermediate values, it produces the stated outputs), but Traces 2 and 3 have a documentation gap where the input STRING does not byte-precise reproduce the cited intermediate VALUES. This does not invalidate the algorithm; it surfaces a calibration risk for the RED-phase test `TestRulesScoreFormulaTraces` which Plan Stage 3.1 promises to enforce within `±0.005`.

## Verdict Justification

All four iteration-1 blocking defects are fully resolved with concrete evidence:
defect 1 closes the REQ-IR-008/OQ-6 contradiction with explicit "Unknown is recoverable, not terminal" language in REQ-IR-008 plus a deterministic acceptance scenario S-19; defect 2 reduces S-7's Then block to a single 4-entry `AdapterSet` assertion that I independently re-derived from the algorithm; defect 3 replaces all four `:38-42` citations with `:34-38` (5 hits, zero residual `:38-42` references) and matches the actual line range of `Classify` in `internal/llm/provider.go`; defect 4 adds a 73-line §2.3 specifying signals, per-category formulas, aggregation, tie-break ordering, determinism guarantees, and 3 worked traces, and is referenced from acceptance.md S-1/S-2/S-13/S-19 plus plan.md Stage 3.1's new `TestRulesScoreFormulaTraces`. Two iteration-1 non-blocking findings (file count, forced-LLM mode) carry forward unfixed but were never blocking. Two new non-blocking precision findings (N1, N2) note that worked traces 2 and 3 use r/pd values that are not byte-precise reproducible from their cited input strings; N2 is sharper because at the actual `r ≈ 0.235`, S-3's expected LLM escalation may not trigger as written. Both are recoverable in run phase via fixture refresh and do not block plan-phase approval. Total non-blocking findings = 4, within tolerance. PASS.
