# Audit Report — SPEC-IR-001 (iteration 1)

## Verdict
FAIL

## Summary
The SPEC is structurally strong (EARS coverage complete, all 5 patterns, frontmatter complete, locked decisions honored, MX plan present, new-metric precedent verified real). However, four blocking defects prevent PASS: an internal contradiction between REQ-IR-008 and Open Question 6 default for the Unknown Category fallback, a self-contradictory expected-output block in acceptance scenario S-7, a systematic citation mismatch (`internal/llm/provider.go:38-42` cited four times for the Classify ModelClass priority, which is actually at lines 34-38), and a hand-waved rule-based confidence scoring algorithm that violates TDD's testable-scoring requirement.

Reasoning context ignored per M1 Context Isolation. Audit performed against SPEC files and codebase only.

## Blocking Defects

1. **Internal contradiction: REQ-IR-008 vs Open Question 6 (Unknown Category fallback)**
   - Location: `spec.md:203` (REQ-IR-008) vs `spec-compact.md:158` and `research.md:667-669` (Open Question 6).
   - REQ-IR-008 explicitly requires the web fallback ONLY "WHEN the intersection is empty AND `decision.Category != CategoryUnknown`". OQ-6 default declares "Unknown Category → web+social fallback". These two rules disagree on the Unknown case. acceptance.md provides no scenario for `Category == CategoryUnknown`, so the gap is not closed by tests.
   - Proposed fix: pick one policy. Recommend amending REQ-IR-008 to also produce a `web+social` fallback for Unknown (drop the `Category != CategoryUnknown` guard). Add an acceptance scenario S-19 that exercises Unknown Category with a non-empty registry and asserts the chosen fallback set.

2. **Self-contradictory expected output in acceptance scenario S-7**
   - Location: `acceptance.md:185-188`.
   - The Then block first asserts `decision.AdapterSet == ["daum", "naver", "rss_korean"]` (3 entries), then commentary re-derives the answer as `["arxiv", "daum", "naver", "rss_korean"]` (4 entries) and writes "Therefore final assert: ...". A single scenario cannot have two different expected outputs. The implementer cannot determine which assertion to write a test for.
   - Proposed fix: delete the first 3-entry assertion; keep only the 4-entry final assert. Move the commentary into the Given block (or a separate explanatory note) so the Then block lists exactly one expected `AdapterSet` value.

3. **Citation mismatch: `internal/llm/provider.go:38-42` claimed for Classify ModelClass priority**
   - Location: `spec.md:26` (HISTORY), `spec.md:60` (§1 Purpose), `research.md:147-152` (§1.5), `research.md:585` (citation index #14). Four occurrences.
   - Verified: actual `Classify` ModelClass priority list is at `internal/llm/provider.go:34-38`. Lines 38-42 contain the closing `},` of Classify followed by the `Embed` map entry — they do NOT contain the Classify priority. Off-by-4 systematic mis-citation propagated across multiple documents.
   - Proposed fix: replace all four occurrences of `:38-42` with `:34-38`. Single global edit across spec.md (twice) and research.md (twice).

4. **Rule-based confidence scoring algorithm hand-waved (TDD discipline failure)**
   - Location: `spec.md:118` (§2.1 e), `plan.md:55` (Stage 3.1), `acceptance.md:44, 73, 296` (multiple "Confidence ≥ 0.85/0.90" assertions).
   - The SPEC and plan declare `Score(q RouterQuery) (Category, float64, []string)` and assert numeric confidence floors (≥ 0.85, ≥ 0.90), but neither the SPEC nor the plan defines the formula that produces the `float64`. There is no description of trigger weights, score normalization, or confidence aggregation across categories. With no algorithm specified, TDD tests will be written to whatever number the implementation happens to produce, defeating the test-first discipline. acceptance S-1 asserts `Confidence ≥ 0.90` — a number untraceable to any specified formula.
   - Proposed fix: add a §3.x "Rule scoring formula" subsection to spec.md that specifies (a) per-trigger weights, (b) the aggregation function (e.g., `min(1.0, sum(weights) / max_observed_weight)`), and (c) tie-breaking ordering across categories. Plan.md Stage 3.1 must reference this formula. Update acceptance scenarios to derive the asserted confidence floors from the formula, not as opaque magic numbers.

## Non-Blocking Findings

1. **File-count inconsistency**: `spec.md:334` header says "Created (16 files)" but the enumerated list contains 19 entries (18 router files + 1 obs/metrics/router.go). spec-compact.md and plan.md DoD repeat "16". Reconcile to the actual count.
2. **`ErrLLMTimeout` sentinel for undocumented "internal forced-LLM mode"**: `spec.md:121` introduces an "internal forced-LLM mode" as the only path that surfaces `ErrLLMTimeout` to callers. No REQ defines this mode and plan.md does not implement it. Either (a) document the mode as a real feature with its own REQ, or (b) drop the sentinel and rely on the Metadata-flag degradation path.
3. **Acceptance density below threshold for two REQs**: REQ-IR-003 maps to S-5 only; REQ-IR-005 maps to S-4 only. Audit criterion expects ≥2 G/W/T per REQ. Each scenario has multiple Then-assertions, so test density is acceptable, but adding one more positive/negative scenario per REQ would improve robustness.
4. **REQ-IR-008 is composite (Ubiquitous + embedded WHEN clause)**: pure EARS would split into a Ubiquitous core and a separate Event-Driven fallback REQ. Stylistic.
5. **Minor citation drift**: `deploy/litellm/config.yaml:23-26` claims claude-haiku-4-5 alias but actual lines are 22-25; `roadmap.md:147` claims M2 exit criterion but actual table row is at line 149. Off-by-1 each. Not blocking but tighten if convenient.

## Positive Observations

1. **All five EARS patterns are covered** with correctly-shaped sentences (Ubiquitous ×3, Event-Driven ×2, State-Driven ×1, Optional ×1, Unwanted ×1). REQ tags match clauses faithfully.
2. **YAML frontmatter is complete** with all required fields including correctly populated `depends_on` (CORE-001, LLM-001, OBS-001) and `blocks` (5 downstream SPECs).
3. **Locked architectural decisions are honored throughout**: library-only exposure (no HTTP/gRPC), Haiku 4.5 default via existing `llm.Classify` ModelClass + `INTENT_ROUTER_LLM_MODEL` Override (model alias verified at `deploy/litellm/config.yaml:22-25`), Hangul Unicode regex with explicit thresholds (0.30 / 0.10 / τ_high 0.85), no caching in v0.
4. **Out-of-scope section is exhaustive (18+ items)** with explicit destination SPECs for each deferred feature, including the cardinality-allowlist non-amendment commitment.
5. **New-metric-family precedent is real**: `internal/obs/metrics/llm.go` exists and follows exactly the `registerLLM(pr) → bundle → store on Registry` shape that SPEC-IR-001's `internal/obs/metrics/router.go` will mirror. The `outcome` label name is already in the allowlist at `internal/obs/metrics/metrics.go:147-154` (line 150). The new metric families add no new label names.
6. **Adapter selection algorithm is concrete, not hand-waved**: Category → DocType eligibility mapping enumerated in `research.md:138-141`; lang compatibility rule explicit; acceptance scenarios S-1, S-2, S-7, S-8, S-13, S-14 enumerate concrete adapter sets with named DocTypes/Langs.
7. **Per-call observability contract is comprehensive**: REQ-IR-006 enumerates the 4-emission contract (counter, histogram, span, slog), nil-safety triple-guard pattern is referenced by file:line, and each emission has a corresponding acceptance assertion.

## Coverage by Audit Item

| # | Audit Item | Result | Notes |
|---|-----------|--------|-------|
| 1 | EARS compliance | PASS | 8 REQs, all 5 patterns covered (`spec.md:196-203`); REQ-IR-008 is composite Ubiquitous+WHEN — stylistic only |
| 2 | Acceptance sufficiency (≥2 per REQ) | WARN | REQ-IR-003 → S-5 only; REQ-IR-005 → S-4 only (`acceptance.md:14-22`) |
| 3 | YAML frontmatter completeness | PASS | All required fields present; depends_on and blocks correct (`spec.md:1-17`) |
| 4 | Files-to-modify accuracy | WARN | Paths clean; "16 files" header conflicts with 19 enumerated (`spec.md:334`) |
| 5 | Out-of-scope completeness | PASS | 18+ exclusions including allowlist amendment, mecab-ko, RRF, caching (`spec.md:131-186`) |
| 6 | Locked-decision compliance | PASS | All 4 decisions honored verbatim (library-only, Haiku, Hangul regex, no cache) |
| 7 | MX tag plan presence | PASS | 8 tags with REASON for ANCHOR/WARN (`spec.md:480-488`) |
| 8 | Defensive scaffolding traps | WARN | `ErrLLMTimeout` for undocumented forced-LLM mode (`spec.md:121`); else clean |
| 9 | Citation accuracy spot-check | FAIL | 1 of 7 sampled citations is mismatched (`provider.go:38-42` should be `:34-38`); systematic across 4 occurrences |
| 10 | Internal consistency (REQ IDs) | WARN | REQ IDs consistent across all 4 docs; file-count claim "16" inconsistent with 19 listed |
| 11 | Confidence threshold soundness | FAIL | τ_high=0.85 cited but rule-scoring formula NOT specified; TDD discipline broken |
| 12 | Hangul detection edge cases | PASS | Korean-heavy, ambiguous-band, 0%-hangul-Korean-topic all covered (S-1, S-3, risk acknowledged in research §6) |
| 13 | LLM-fallback timeout semantics | PASS | REQ-IR-007, S-6, S-15 mutually consistent; err nil, Source=RuleBased, Metadata flags set |
| 14 | Adapter selection algorithm | FAIL | Algorithm itself is concrete; but S-7 expected output is self-contradictory (`acceptance.md:185-188`) |
| 15 | Coverage realism (85% on 8-10 files) | PASS | Distribution table in plan.md §4 is achievable with mocks + golden fixtures |

## Citation Spot-Check

Sampled 7 of the citations in research.md (audit prompt referenced 69; actual research.md §8 contains 47 internal + 8 external = 55 total):

- `internal/llm/provider.go:38-42` → **MISMATCH** (Classify ModelClass priority is at lines 34-38; lines 38-42 contain `},` + Embed entries). Cited 4 times across spec.md HISTORY, spec.md §1, research.md §1.5, research.md citation index #14.
- `internal/llm/router.go:148-198` → verified (Router struct + Route method).
- `internal/llm/client.go:230-252` → verified (`emitObservability` function exactly).
- `internal/adapters/registry.go:147-152` → verified (`Get` method exactly).
- `pkg/types/capabilities.go:38-62` → verified (Capabilities struct exactly).
- `internal/obs/metrics/metrics.go:147-154` → verified (labelNames allowlist exactly).
- `deploy/litellm/config.yaml:23-26` → minor drift (actual lines 22-25; off-by-1).

## New-Metric-Family Precedent Check

- Existing precedent: `internal/obs/metrics/llm.go` **exists** (62 lines). Pattern is exactly `registerLLM(r *prometheus.Registry) llmCollectors` returning a bundle, called from `NewRegistry()` at `internal/obs/metrics/metrics.go:134`. Two new fields (`LLMCalls`, `LLMCost`, `LLMLatency` — actually three) are stored on `Registry` and labels (`provider`, `model`, `outcome`) are appended to the `labelNames` allowlist at `internal/obs/metrics/metrics.go:147-154`.
- Allowlist amendment required: **NO**. The `outcome` label is already present at line 150 of `internal/obs/metrics/metrics.go`. SPEC-IR-001 adds two new metric *families* (`usearch_router_classifications_total`, `usearch_router_classification_duration_seconds`) but no new label *name*. Per NFR-OBS-002, the cardinality discipline is on label names, not metric family names.
- Verdict on the addition: **ACCEPTABLE**. The pattern faithfully mirrors the SPEC-LLM-001 precedent. The cross-package edit to `internal/obs/metrics/metrics.go` (add 2 fields + call `registerRouter(pr)`) is the same shape as the existing LLM extension. SPEC-IR-001's package-boundary preservation (no direct `prometheus/client_golang` import in `internal/router/`) is correct and respects the SPEC-OBS-001 import-boundary test.

## Chain-of-Verification Pass

Second-pass review:
- Re-read every REQ end-to-end: confirmed all 8 REQs match their declared EARS pattern. Confirmed REQ-IR-008 contains an embedded WHEN clause but the primary form is Ubiquitous.
- Re-checked every REQ→AC mapping in acceptance.md §1: confirmed REQ-IR-003 has only S-5 and REQ-IR-005 has only S-4 — already noted.
- Re-counted Created files in spec.md §5.1: 19 entries vs "16 files" header — already noted.
- Re-checked Exclusions list line-by-line: 18+ entries, all required exclusions present.
- Re-checked all four locked decisions across spec.md, plan.md, spec-compact.md: all consistent and honored.
- Re-checked acceptance scenario internal consistency: S-7 contradiction confirmed; no other scenarios contradict themselves.
- Re-checked citation index against actual code: provider.go mis-citation propagates across 4 documents — counted as ONE defect with 4 manifestations.
- Re-checked OQ-6 vs REQ-IR-008 contradiction: confirmed the Unknown-Category policy is contradictory across documents.

No additional defects discovered.

## Recommendation

Return to manager-spec with the following mandatory edits before iteration 2:

1. Resolve the Unknown Category policy. Either amend REQ-IR-008 to drop the `Category != CategoryUnknown` guard (matching OQ-6 default) and add an acceptance scenario S-19 for Unknown, OR amend OQ-6 to state "Unknown Category → empty AdapterSet". Pick one and update spec.md, spec-compact.md, research.md §9, and acceptance.md.
2. Rewrite acceptance.md S-7 Then block to assert exactly one expected `AdapterSet` value. Move the derivation commentary out of the Then block.
3. Replace `provider.go:38-42` with `provider.go:34-38` in spec.md HISTORY (line 26), spec.md §1 Purpose (line 60), research.md §1.5 (line 147), research.md citation index #14 (line 585).
4. Add a "Rule scoring formula" subsection to spec.md (between §3 and §4) that specifies trigger weights, aggregation function, and tie-breaking. Update plan.md Stage 3.1 to reference the formula. Re-derive acceptance confidence floors (≥0.85, ≥0.90, etc.) from the formula or replace them with rubric-anchored bands.
5. Reconcile the "16 files" header in spec.md §5.1 (and downstream in spec-compact.md and plan.md DoD) with the 19 enumerated entries.
6. Either define the "internal forced-LLM mode" as a documented REQ or remove the `ErrLLMTimeout` sentinel commentary at spec.md §2.1(h).

Once these are addressed, re-run plan-auditor for iteration 2.
