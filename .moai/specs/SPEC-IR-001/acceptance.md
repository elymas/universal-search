# Acceptance Criteria — SPEC-IR-001 Intent Router v0

**SPEC**: SPEC-IR-001
**Format**: Given/When/Then scenarios with explicit observability assertions
**Coverage**: All 8 EARS REQs + 2 NFRs

---

## 1. Mapping: REQ → Scenario(s)

| REQ | Scenarios |
|-----|-----------|
| REQ-IR-001 | S-1, S-2, S-3 |
| REQ-IR-002 | S-1, S-2 |
| REQ-IR-003 | S-5 |
| REQ-IR-004 | S-12, S-13 |
| REQ-IR-005 | S-4 |
| REQ-IR-006 | S-1 (and observability assertions on every other scenario) |
| REQ-IR-007 | S-6, S-15 (parent ctx) |
| REQ-IR-008 | S-7, S-8, S-14 (web fallback), S-19 (Unknown Category dispatch) |
| NFR-IR-001 | S-9 (rule-based perf) |
| NFR-IR-002 | S-10 (LLM-fallback p95) |

Plus edge-case scenarios S-11 (concurrent), S-16 (oversized query),
S-17 (LLM JSON parse error), S-18 (registry empty at New), S-19
(Unknown Category dispatch — recoverable, not terminal).

---

## 2. Given/When/Then Scenarios

### S-1: Korean-heavy query — deterministic rule-based classification

**Given**:
- A `Router` with adapter registry containing `naver`, `daum`, `rss_korean`, `hackernews`, `searxng`
- A `RouterQuery{Text: "ChatGPT 사용법과 프롬프트 엔지니어링 팁", Lang: "", MaxResults: 10}`
- The query has Hangul ratio ≈ 0.55 (above 0.30 threshold)

**When**:
- `Classify(ctx, q)` is invoked

**Then**:
- Returned err is nil
- `decision.Category == CategoryKorean`
- `decision.Confidence ≥ 0.90` (per spec.md §2.3 worked example 3: `score_korean = clamp(0.55 + 0.4 + 0.1*pd, 0, 1) ≈ 0.96` for this fixture)
- `decision.Source == SourceRuleBased`
- `decision.Lang == "ko"`
- `decision.AdapterSet ⊇ {daum, naver, rss_korean, searxng}` (sorted, alphabetical)
- `decision.AdapterSet` does NOT contain `hackernews`
- `decision.Metadata["hangul_ratio"] ≈ 0.55 ± 0.05`
- `decision.Metadata["rule_triggers"]` is a non-empty slice including `"hangul_ratio_high"`
- `RouterClassifications.WithLabelValues("classified_korean")` incremented exactly once
- `RouterClassificationDuration.WithLabelValues("classified_korean")` observed exactly once
- One OTel span `router.classify` recorded with attributes `router.category=korean`, `router.source=rule_based`, `router.lang=ko`, `router.adapter_count=4`, `router.confidence ≥ 0.90`
- One slog INFO record emitted with the documented attribute set including `request_id` (if ctx carries one)
- ZERO outbound LLM calls (assert via mock LLM client invocation count)

---

### S-2: Pure-English academic query — rule-based, no LLM

**Given**:
- A `Router` with registry `{arxiv, github, hackernews, searxng}` where arxiv has `DocTypes:[paper], SupportedLangs:[]`, github has `DocTypes:[repo,issue], SupportedLangs:[]`, hackernews has `DocTypes:[post], SupportedLangs:[en]`, searxng has `DocTypes:[article,other], SupportedLangs:[]`
- `RouterQuery{Text: "transformer attention is all you need 2017 paper", Lang: "", MaxResults: 10}`
- 0% hangul, multiple academic keyword hits ("transformer", "paper", "2017")

**When**:
- `Classify(ctx, q)` is invoked

**Then**:
- err is nil
- `decision.Category == CategoryAcademic`
- `decision.Confidence ≥ 0.85` (per spec.md §2.3 worked example 1: `score_academic = clamp(0.8 * 1.0 + 0.2 * 1.0, 0, 1) = 1.0` for "transformer attention paper"; this fixture has equal or higher academic keyword density)
- `decision.Source == SourceRuleBased`
- `decision.Lang == "en"` (default detected)
- `decision.AdapterSet == ["arxiv", "github"]` (sorted; hackernews excluded — `social` only)
- ZERO LLM calls
- `RouterClassifications{outcome="classified_academic"}` +1

---

### S-3: Mixed Korean+English at 15% hangul → escalates to LLM

**Given**:
- A `Router` with normal registry
- `RouterQuery{Text: "best Korean GPT 모델 추천", Lang: "", MaxResults: 10}` — hangul ratio ≈ 0.18 (in ambiguous band 0.10-0.30)
- A mock `llm.Client` whose `Complete` returns
  `{"category":"mixed","confidence":0.78,"rationale":"Korean-English code-mixed query asking for Korean LLM recommendations"}`
  in 800ms

**When**:
- `Classify(ctx, q)` is invoked

**Then**:
- err is nil
- `decision.Category == CategoryMixed`
- `decision.Confidence == 0.78` (LLM's value wins)
- `decision.Source == SourceLLMFallback`
- `decision.Metadata["hangul_ratio"] ≈ 0.18`
- `decision.Metadata["llm_rationale"]` is the truncated rationale
- `decision.Metadata["rule_confidence"]` is set (debug-aid, the rule confidence shadowed by LLM)
- Mock `llm.Client.Complete` was called exactly ONCE
- Total elapsed ≈ 800ms ± 100ms (no timeout, no retry)
- `RouterClassifications{outcome="classified_mixed"}` +1
- One slog INFO record with `llm_used=true`, `request_id` populated

---

### S-4: Empty query — ErrInvalidQuery, no LLM, observability emitted

**Given**:
- A `Router` constructed normally
- Five test inputs: `RouterQuery{Text: ""}`, `Text: "   "`, `Text: "\t\n"`, `Text: " "` (non-breaking space), `Text: "  \r  "`

**When**:
- `Classify(ctx, q)` is invoked for each

**Then** (for each input):
- Returned err is `ErrInvalidQuery` (`errors.Is(err, ErrInvalidQuery) == true`)
- Returned `RoutingDecision` is the zero value
- ZERO LLM calls
- `RouterClassifications{outcome="error_invalid"}` +1
- `RouterClassificationDuration{outcome="error_invalid"}` +1 observation (small elapsed)
- One slog WARN record with attribute `error="ErrInvalidQuery"`
- One OTel span recorded with `RecordError(ErrInvalidQuery)` and status `Error`

---

### S-5: LLM unavailable (circuit-breaker open) — graceful degradation

**Given**:
- A `Router` with a mock `llm.Client` whose `Complete` returns `llm.ErrAllProvidersFailed` immediately
- An ambiguous query `"GPT review tutorial Korean"` (hangul ratio = 0.0, but with Korean signal in text)

**When**:
- `Classify(ctx, q)` is invoked

**Then**:
- Returned err is nil (LLM error NOT propagated)
- `decision.Category` matches the rule-based classification (likely `web` or `mixed` per rules)
- `decision.Source == SourceRuleBased`
- `decision.Metadata["llm_unavailable"] == true`
- `decision.Metadata["degraded_confidence"] == true`
- Mock `llm.Client.Complete` was called exactly ONCE (not retried — IR catches the
  consolidated `ErrAllProvidersFailed` once and degrades)
- `RouterClassifications{outcome="error_breaker_open"}` +1

---

### S-6: LLM timeout (>2s) — graceful degradation

**Given**:
- A `Router` with a mock `llm.Client` whose `Complete` blocks on `<-time.After(3*time.Second)` then returns success
- An ambiguous query that would otherwise require LLM
- `ctx = context.Background()` (no parent deadline)

**When**:
- `Classify(ctx, q)` is invoked

**Then**:
- Returned err is nil
- `decision.Source == SourceRuleBased`
- `decision.Metadata["llm_timeout"] == true`
- `decision.Metadata["degraded_confidence"] == true`
- Total elapsed ≤ 2.5s (2s deadline + ~500ms degradation overhead)
- The mock LLM call's underlying ctx returns `context.DeadlineExceeded`
- `RouterClassifications{outcome="error_timeout"}` +1

---

### S-7: AdapterSet selection — Korean Lang excludes English-only adapters

**Given**:
- A `Router` with registry containing 5 adapters with capabilities:
  - `naver{DocTypes:[article,post], SupportedLangs:[ko]}`
  - `daum{DocTypes:[article], SupportedLangs:[ko]}`
  - `rss_korean{DocTypes:[article], SupportedLangs:[ko]}`
  - `hackernews{DocTypes:[post,social], SupportedLangs:[en]}`
  - `arxiv{DocTypes:[paper], SupportedLangs:[]}`
- A query that triggers `Category == CategoryKorean`, `Lang == "ko"`
- (Algorithm walk-through, informational): per spec.md REQ-IR-008,
  `categoryEligibleDocTypes(CategoryKorean) == ANY` (Korean is
  language-driven, not type-driven), so the DocType filter passes
  all 5 adapters. The Lang filter `SupportedLangs ⊇ {ko} OR
  empty` admits `naver` (`[ko]`), `daum` (`[ko]`), `rss_korean`
  (`[ko]`), and `arxiv` (empty = language-agnostic), and excludes
  `hackernews` (`[en]`). After lexicographic sort the intersection
  is `["arxiv", "daum", "naver", "rss_korean"]` — 4 entries.

**When**:
- `Classify(ctx, q)` returns `decision`

**Then**:
- `decision.AdapterSet == ["arxiv", "daum", "naver", "rss_korean"]` (4 entries, sorted lexicographically)
- `decision.Metadata["adapter_set_fallback"]` is NOT set (intersection is non-empty)

---

### S-8: AdapterSet selection — empty intersection triggers web fallback

**Given**:
- A `Router` with registry containing only:
  - `arxiv{DocTypes:[paper], SupportedLangs:[]}`
  - `github{DocTypes:[repo,issue], SupportedLangs:[]}`
- A query classified as `Category == CategoryKorean` (no Korean adapters in registry)

**When**:
- `Classify(ctx, q)` returns `decision`

**Then**:
- `decision.AdapterSet` is `[]` (no adapters can serve `korean` Category)
- BUT per REQ-IR-008 fallback rule: `decision.Metadata["adapter_set_fallback"] == true`
- If a `searxng{DocTypes:[article,other], SupportedLangs:[]}` were also registered, the fallback would yield `["searxng"]`. With only arxiv+github, the fallback set is also empty.
- Decision fields are still all populated (Category, Confidence, etc.)

---

### S-9: NFR-IR-001 — rule-based path performance

**Given**:
- `BenchmarkClassifyRulePath100Chars` benchmark in `bench_test.go`
- 100-character ASCII academic query (e.g., "transformer attention paper 2017 yoshua bengio gradient descent neural networks")
- Mock LLM client (never invoked because rule-based path is high-confidence)

**When**:
- Benchmark runs 10000 iterations on amd64

**Then**:
- p50 ≤ 1 ms per `Classify` call
- ≤ 10 allocs/op
- Benchmark exits with `PASS`

---

### S-10: NFR-IR-002 — LLM-fallback p95

**Given**:
- A stub `llm.Client` whose `Complete` returns success after a delay sampled from
  `Exp(λ=0.7)` capped at 2.5 seconds (mean ≈ 1.4s, tail to 2.5s)
- 200 distinct ambiguous queries that require LLM
- Test `TestClassifyLLMFallbackP95UnderLimit`

**When**:
- The 200 Classify invocations complete; durations are sorted

**Then**:
- `durations[190]` (p95 of 200) ≤ 3.0 s
- `durations[199]` (p100 / max) ≤ 3.5 s
- ZERO timeout errors (the stub is bounded at 2.5s)

---

### S-11: Concurrent Classify — race-clean

**Given**:
- A single `Router` instance constructed with normal registry + LLM client
- 200 goroutines, each invoking `Classify(ctx, q)` 50 times with diverse queries
- Test runs with `go test -race`

**When**:
- All 10000 invocations complete

**Then**:
- ZERO race detector reports
- ALL invocations return successful RoutingDecisions
- Total `RouterClassifications` counter sum = 10000
- No goroutine leaks (verified via `goleak.VerifyNone(t)` if available)

---

### S-12: Lang override (REQ-IR-004) — explicit Lang wins

**Given**:
- A `Router` with normal registry
- `RouterQuery{Text: "ChatGPT 사용법", Lang: "ja", MaxResults: 10}` (heavily Korean text but caller declares Japanese)

**When**:
- `Classify(ctx, q)` is invoked

**Then**:
- err is nil
- `decision.Lang == "ja"` (override honoured)
- `decision.Metadata["lang_override"] == true`
- `decision.Metadata["hangul_ratio"]` is still computed and ≈ 0.7 (diagnostic)
- Hangul detection branch was SKIPPED for category determination
- The Category may still be `korean` (rules can fire on text content) BUT the Lang is `ja`

---

### S-13: Lang override does NOT skip rule-based scoring

**Given**:
- A `Router` with normal registry
- `RouterQuery{Text: "transformer paper review", Lang: "ko", MaxResults: 10}` (caller forces Korean lang on English text)

**When**:
- `Classify(ctx, q)` is invoked

**Then**:
- `decision.Lang == "ko"` (override honoured)
- `decision.Category == CategoryAcademic` (rules fire on English keyword)
- `decision.Confidence ≥ 0.85` (per spec.md §2.3 formula on `"transformer paper review"`: `score_academic ≥ 0.85` because all three tokens hit the academic keyword table and `r = 0.0`)
- `decision.AdapterSet` filtered by `Lang=="ko"` — academic adapters with Korean support OR language-agnostic
- `decision.Metadata["lang_override"] == true`

---

### S-14: Web fallback when intersection is empty AND Category != Unknown

**Given**:
- A `Router` with registry containing `searxng{DocTypes:[article,other], SupportedLangs:[]}` only
- A query classified as `Category == CategoryAcademic`, `Lang == "en"`

**When**:
- `Classify(ctx, q)` is invoked

**Then**:
- `decision.AdapterSet ⊇ {"searxng"}` (web fallback)
- `decision.Metadata["adapter_set_fallback"] == true`

---

### S-15: Parent context deadline supersedes 2s internal timeout

**Given**:
- A `Router` with mock `llm.Client` blocking 3s
- `ctx, cancel := context.WithTimeout(parent, 500*time.Millisecond); defer cancel()`
- An ambiguous query

**When**:
- `Classify(ctx, q)` is invoked

**Then**:
- Total elapsed ≤ 700ms (parent deadline wins, not the 2s internal)
- `decision.Source == SourceRuleBased`
- `decision.Metadata["llm_timeout"] == true`
- The internal 2-second `WithTimeout` is derived from the parent ctx; when parent expires first, the LLM call's ctx is cancelled

---

### S-16: Oversized query (10K chars) — handled gracefully

**Given**:
- A `Router` with normal registry
- `RouterQuery{Text: <10KB random ASCII>, Lang: "", MaxResults: 10}`

**When**:
- `Classify(ctx, q)` is invoked

**Then**:
- err is nil (oversized but valid query is acceptable)
- `decision` is a valid RoutingDecision
- Rule-based path completes within p99 latency budget (likely < 50ms — still well under 1ms-per-100-chars NFR; this scenario tests no panic or unbounded scan)
- Hangul ratio is computed correctly
- No regex backtracking pathological case

---

### S-17: LLM returns malformed JSON — graceful parse error

**Given**:
- A `Router` with mock `llm.Client.Complete` returning text:
  - Sub-case A: `"category: web"` (not JSON, no braces)
  - Sub-case B: `'{"category": "web", "confidence": notanumber}'` (invalid JSON syntax)
  - Sub-case C: `'{"category": "vehicle", "confidence": 0.8}'` (category not in enum)
  - Sub-case D: `'```json\n{"category":"web","confidence":0.7}\n```'` (code-fence-wrapped, valid)
- An ambiguous query

**When**:
- `Classify(ctx, q)` is invoked for each sub-case

**Then**:
- For sub-cases A, B, C: err is nil, `decision.Source == SourceRuleBased`, `decision.Metadata["degraded_confidence"] == true`, `RouterClassifications{outcome="error_parse"}` +1, slog WARN record emitted with `parse_error="..."`
- For sub-case D: parser strips code-fence successfully, `decision.Source == SourceLLMFallback`, `decision.Category == CategoryWeb`, `decision.Confidence == 0.7`, no error counter

---

### S-18: Registry empty at construction — ErrAdapterRegistryEmpty

**Given**:
- An empty `adapters.Registry` (zero adapters registered)
- A normal `llm.Client` and `obs.Obs`

**When**:
- `router.New(Options{Registry: emptyReg, LLMClient: c, Obs: o})` is called

**Then**:
- Returned `*Router` is nil
- Returned err is `ErrAdapterRegistryEmpty` (`errors.Is(err, ErrAdapterRegistryEmpty) == true`)
- No partial Router state leaked

---

### S-19: Unknown Category dispatch — recoverable, not terminal

**Given**:
- A `Router` with registry containing 4 adapters with capabilities:
  - `searxng{DocTypes:[article,other], SupportedLangs:[]}`
  - `hackernews{DocTypes:[post,social], SupportedLangs:[en]}`
  - `arxiv{DocTypes:[paper], SupportedLangs:[]}`
  - `naver{DocTypes:[article,post], SupportedLangs:[ko]}`
- A short, ambiguous query like `"asdf qwerty"` whose rule scores are uniformly low and (after the LLM-fallback path also returns `unknown` OR is bypassed by a stub) yields `decision.Category == CategoryUnknown`, `decision.Lang == "en"`
- (Algorithm walk-through, informational): per spec.md REQ-IR-008,
  `categoryEligibleDocTypes(CategoryUnknown)` returns the union of
  web-eligible (`{article, post, other}`) and social-eligible
  (`{post, social, video}`) DocTypes. The DocType filter admits
  `searxng` (`article, other` ⊆ union), `hackernews`
  (`post, social` ⊆ union), and `naver` (`article, post` ⊆ union);
  it excludes `arxiv` (`paper` ∉ union). The Lang filter
  (`SupportedLangs ⊇ {en} OR empty`) admits `searxng` (empty),
  `hackernews` (`[en]`), and excludes `naver` (`[ko]` ⊉ `{en}`).

**When**:
- `Classify(ctx, q)` is invoked

**Then**:
- err is nil (Unknown is a valid, recoverable classification — never an error)
- `decision.Category == CategoryUnknown`
- `decision.AdapterSet == ["hackernews", "searxng"]` (sorted lexicographically; 2 entries)
- `decision.Metadata["adapter_set_fallback"]` is NOT set (intersection is non-empty)
- `RouterClassifications{outcome="classified_unknown"}` +1
- `RouterClassificationDuration{outcome="classified_unknown"}` +1 observation

This scenario closes the audit gap: REQ-IR-008's Unknown handling
is now exercised by an explicit acceptance scenario, and the
expected AdapterSet is fully derivable from the algorithm in
spec.md REQ-IR-008 + spec.md §2.3.

---

## 3. Quality Gate Criteria

For SPEC-IR-001 to pass acceptance:

- [ ] All 19 scenarios above pass (test assertions met)
- [ ] All 8 EARS REQs covered by at least one scenario
- [ ] All 2 NFRs covered (S-9 + S-10)
- [ ] Coverage ≥ 85% across `internal/router/` and the new `internal/obs/metrics/router.go`
- [ ] `go test ./internal/router/... -race` returns clean
- [ ] `go vet ./...` returns clean
- [ ] `golangci-lint run ./...` returns clean
- [ ] `go test ./...` for the repo returns clean (no regression in CORE-001/LLM-001/OBS-001)
- [ ] Benchmark `BenchmarkClassifyRulePath100Chars` reports p50 < 1ms
- [ ] No new label name added to `internal/obs/metrics/metrics.go::labelNames` allowlist
- [ ] No new direct dep added to `go.mod`
- [ ] `cmd/usearch --version` runs successfully
- [ ] `cmd/usearch --help` runs successfully
- [ ] All MX tags from spec.md §5.6 are present in source
- [ ] All sentinel errors are exported and `errors.Is` testable

---

## 4. Definition of Done

The SPEC is DONE when:

- [ ] HISTORY entry in spec.md updated with implementation details (commit hash, coverage, test counts)
- [ ] Status in spec.md frontmatter changed from `draft` to `implemented`
- [ ] `internal/router/router.go` no longer is the 4-line stub
- [ ] `internal/router/` is a fully-tested package with 50+ tests
- [ ] PR opened, reviewed by codeowner, merged
- [ ] M2 parallelization unblocked: SPEC-ADP-001, SPEC-ADP-002, SPEC-CLI-001, SPEC-SYN-001 can begin run-phase against the implemented IR-001 contract

---

## 5. Edge Cases Beyond the Scenarios

These are documented but not separate scenarios; they are covered
within the scenarios above.

| Edge Case | Covered by |
|-----------|------------|
| nil ctx | S-4 (combine with empty Text) — Classify must validate ctx is non-nil |
| Whitespace-only query (Unicode whitespace including U+00A0) | S-4 |
| 0-character query | S-4 |
| Query with only hangul filler chars (no real content) | Implicit in S-1 — passes if rule scoring still produces a Category |
| Query with only emoji | Tested via golden fixtures |
| LLM returns confidence outside [0,1] | S-17 sub-case (clamped to [0,1]) |
| Concurrent calls during Router.New | Not a scenario — N/A; New is a one-shot constructor |
| `obs.Obs` is nil | S-1 with nil obs param (REQ-IR-006 nil-safety) |
| Multiple LLM model overrides via env var changing during process lifetime | Not supported in v0; env is read once at New |

---

*End of acceptance.md for SPEC-IR-001 v0.1*
