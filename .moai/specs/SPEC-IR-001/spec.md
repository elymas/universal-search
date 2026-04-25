---
id: SPEC-IR-001
title: Intent Router v0
version: 0.1.0
milestone: M2 — First end-to-end slice
status: draft
priority: P0
owner: expert-backend
methodology: tdd
coverage_target: 85
created: 2026-04-26
updated: 2026-04-26
author: limbowl
issue_number: null
depends_on: [SPEC-CORE-001, SPEC-LLM-001, SPEC-OBS-001]
blocks: [SPEC-FAN-001, SPEC-CLI-001, SPEC-SYN-001, SPEC-ADP-001, SPEC-ADP-002]
---

# SPEC-IR-001: Intent Router v0

## HISTORY

- 2026-04-26 (initial draft v0.1, limbowl via manager-spec):
  First EARS-formatted SPEC. M2 entry-point SPEC; library-only exposure
  (no HTTP endpoint). LLM fallback uses existing
  `llm.Classify` ModelClass (Haiku 4.5 default per
  `internal/llm/provider.go:34-38`) via `Request.Override` when the
  `INTENT_ROUTER_LLM_MODEL` env is set; circuit-breaker awareness
  inherited transparently from SPEC-LLM-001 NFR-LLM-002. Korean
  detection is deterministic (regex over Hangul Unicode blocks +
  function-word presence) with two thresholds (0.30 high-confidence
  Korean, 0.10 low-confidence non-Korean) and an LLM-adjudicated
  ambiguous band. No caching (locked decision; future SPEC). No
  hot-reload of rules. Adapter set is selected by intersecting
  category-eligible DocTypes with adapters whose
  `Capabilities.SupportedLangs` matches detected `Lang`. Builds on
  SPEC-CORE-001 (`pkg/types.Adapter`/`Capabilities`/`Query` +
  `internal/adapters.Registry`), SPEC-LLM-001 (`llm.Client.Complete`
  with `Class: Classify`, `Override: ...`, free retry/fallthrough),
  SPEC-OBS-001 (`obs.Logger`/`obs.Tracer`/Prometheus collectors —
  registers two NEW metric families `usearch_router_classifications_total`
  and `usearch_router_classification_duration_seconds` reusing the
  existing `outcome` label name without amending the cardinality
  allowlist). 8 EARS REQs (6 × P0 + 1 × P1 + 1 × P0) covering all
  five EARS patterns, 2 NFRs, 30+ golden-file fixtures planned.
  Research artifact at `.moai/specs/SPEC-IR-001/research.md` captures
  47 internal file:line citations + 8 external Context7/WebFetch
  sources. Ready for plan-auditor review and annotation cycle.

---

## 1. Purpose

SPEC-BOOT-001 reserved `internal/router/router.go` as a 4-line stub
(`internal/router/router.go:1-4`). SPEC-CORE-001 published the typed
contract (`pkg/types.Adapter`, `pkg/types.Capabilities`,
`pkg/types.Query`) and the `internal/adapters.Registry`
(`internal/adapters/registry.go:75-167`). SPEC-LLM-001 delivered the
LLM client with a pre-existing `llm.Classify` ModelClass routed to
Haiku 4.5 → gpt-4o-mini → ollama (`internal/llm/provider.go:34-38`).
SPEC-OBS-001 delivered the observability bundle and cardinality
discipline.

SPEC-IR-001 fills `internal/router/` with the **Intent Router** —
the first real consumer of the SPEC-CORE-001 contract. The router
is a pure library function:

- A `Router` struct (constructed once, concurrent-safe via the
  immutable `Rules` plus the underlying registry/llm/obs bundles
  which are themselves concurrent-safe).
- A `Classify(ctx, RouterQuery) (RoutingDecision, error)` method
  that classifies the query into ONE of six categories
  (`web`, `social`, `academic`, `korean`, `mixed`, `unknown`), with
  a confidence score, an adapter set, the detected language, and
  the source of the classification (rule-based, LLM-fallback, or
  default).
- A pipeline: deterministic Hangul-ratio detection + Korean
  function-word signal → keyword-rule scoring → confidence gate
  → optional LLM adjudication via `llm.Client.Complete` with
  `Class: llm.Classify` → adapter set selection via
  `Capabilities.SupportedLangs` and `Capabilities.DocTypes`.
- Per-call observability: 1 Prometheus counter increment +
  1 OTel span + 1 slog record per `Classify`, mirroring the
  wrappedAdapter pattern at `internal/adapters/registry.go:223-252`
  and the LLM client emit at `internal/llm/client.go:230-252`.

The Router does NOT invoke adapters. It does NOT do fanout. It
DECIDES which adapters should be invoked. SPEC-FAN-001 (M3)
consumes `RoutingDecision.AdapterSet` and dispatches.

Completion unblocks five downstream SPECs (FAN-001, CLI-001,
SYN-001, ADP-001 reference, ADP-002), all of which need the
Router as a precondition for end-to-end query flow. The M2 exit
criterion in `.moai/project/roadmap.md:147` (`usearch query "hello
world"` returns Reddit + HN results with one synthesized paragraph
+ citations) becomes achievable only after IR-001, ADP-001,
ADP-002, SYN-001, and CLI-001 all land.

This is the **wedge SPEC** for M2 parallelization
(`.moai/project/roadmap.md:117-122` "M2 | SPEC-ADP-001 + SPEC-ADP-002
+ SPEC-CLI-001 (3-way, after SPEC-IR-001)"). All three M2
parallel SPECs depend on IR-001's `RoutingDecision` shape; once
IR-001's spec.md is approved, those three can begin their plan
phases without waiting for IR-001's run phase.

---

## 2. Scope

### 2.1 In-Scope

| Ref | Item |
|-----|------|
| a | `internal/router/category.go`: `Category` enum (6 values: web/social/academic/korean/mixed/unknown), `ClassificationSource` enum (rule_based/llm_fallback/default), `categoryEligibleDocTypes(Category) []types.DocType` mapping |
| b | `internal/router/query_input.go`: `RouterQuery` struct wrapping `pkg/types.Query` + hints (`Lang` override, `Categories` override for testing), validation method |
| c | `internal/router/routing_decision.go`: `RoutingDecision{Category, Confidence, AdapterSet []string, Lang, Source, Metadata map[string]any}` with `MarshalJSON` for downstream serialization; documented Metadata key allowlist |
| d | `internal/router/korean.go`: `HangulRatio(s string) float64` (rune-count-weighted ratio over the 4 Hangul Unicode blocks U+AC00-D7A3 + U+1100-11FF + U+3130-318F + U+A960-A97F); `KoreanSignals(s string) (ratio float64, hasParticle bool)` returning composite signal; package-private `koreanParticles []string` (11 entries) |
| e | `internal/router/rules.go`: `Rules` struct with keyword tables (academic terms, social platform names, Korean signal terms, generic web markers); `Score(q RouterQuery) (Category, float64, []string)` returning best-match category, confidence, and the names of triggers fired (for Metadata.rule_triggers); compiled regexes via package-init for performance; `New()` constructor with default rules; configurable thresholds via package constants (τ_high, ratio_high, ratio_low) documented as @MX:NOTE |
| f | `internal/router/llm.go`: LLM-fallback path: builds Anthropic-tool-use-style JSON-only prompt with cached system prompt (≥1024 tokens for prompt-cache eligibility), calls `llmClient.Complete(ctx, llm.Request{Class: llm.Classify, Override: <env>, ...})`, parses response (strip code-fence + `json.Unmarshal` + enum validate + clamp confidence + truncate rationale); on `errors.Is(err, llm.ErrAllProvidersFailed)` returns "circuit open" signal; on `context.DeadlineExceeded` returns "timeout" signal |
| g | `internal/router/router.go`: replace 4-line stub with `Router struct{rules, llmClient, registry, obs, caps, confidenceThreshold, llmModelOverride, llmDeadline}` + `New(opts Options) (*Router, error)` constructor + `Classify(ctx, RouterQuery) (RoutingDecision, error)` method; `caps` is a `map[string]types.Capabilities` snapshot taken at New() (per research.md §1.3) |
| h | `internal/router/errors.go`: sentinels `ErrInvalidQuery` (empty/whitespace-only Text), `ErrLLMTimeout` (LLM exceeded 2s — SURFACED to caller only when an internal forced-LLM mode is invoked; in the normal Classify path timeout is converted to a graceful degraded-confidence rule-based result), `ErrAdapterRegistryEmpty` (registry has zero adapters at New time) |
| i | `internal/router/metrics.go`: outcome label CONSTANTS (10 values: `classified_web`, `classified_social`, `classified_academic`, `classified_korean`, `classified_mixed`, `classified_unknown`, `error_invalid`, `error_timeout`, `error_breaker_open`, `error_parse`) + `outcomeFromDecision(d RoutingDecision, err error) string` helper. NO direct `prometheus/client_golang` import (preserves SPEC-OBS-001 boundary) |
| j | `internal/obs/metrics/router.go`: NEW file owned by SPEC-IR-001 but living in obs/metrics/ to preserve the import-boundary test from SPEC-OBS-001 REQ-OBS-006. Declares `RouterClassifications *prometheus.CounterVec{outcome}` and `RouterClassificationDuration *prometheus.HistogramVec{outcome}` collectors. `registerRouter(pr *prometheus.Registry) routerCollectors` returns the bundle, mirroring `registerLLM` at `internal/obs/metrics/metrics.go:134` |
| k | `internal/obs/metrics/metrics.go`: minor edit — extend `Registry` struct with two new fields `RouterClassifications` + `RouterClassificationDuration`; call `registerRouter(pr)` from `NewRegistry()`; NO change to `labelNames` allowlist (no new label name introduced — `outcome` already allowlisted at line 152) |
| l | `internal/router/testdata/queries_golden.json`: 30+ classification fixtures across all 6 categories with expected confidence ranges, tagged by category, used by router_test.go for golden-file regression testing |
| m | One unit test file per source file: `category_test.go`, `query_input_test.go`, `routing_decision_test.go`, `korean_test.go`, `rules_test.go`, `llm_test.go`, `router_test.go`, `metrics_test.go` |
| n | `cmd/usearch/main.go` minor wiring: when `LITELLM_MASTER_KEY` is set AND adapter registry is non-empty, construct `router.New(...)` and store on the cmd context for future M2 SPECs (CLI-001, SYN-001) to consume. No CLI-visible behavior change in IR-001 itself |

### 2.2 Out-of-Scope (Exclusions — What NOT to Build)

[HARD] This SPEC explicitly excludes the following items. Each has a known
destination SPEC; this list prevents scope creep into IR-001.

- **HTTP / gRPC endpoint exposure**. The Router is a Go library
  function. No `cmd/usearch-api/` route, no chi v5 handler, no
  connect-go service. → SPEC-CLI-001 (M2) for CLI surface, future
  `SPEC-API-001` for HTTP, SPEC-MCP-001 (M7) for MCP.
- **Adapter invocation**. The Router DECIDES; FAN-001 dispatches. The
  registry's `Get(name).Search(ctx, q)` is NOT called from the Router.
  → SPEC-FAN-001 (M3).
- **Fanout / parallel dispatch / dedup / partial-result assembly**.
  → SPEC-FAN-001 (M3).
- **Synthesis consumption** (the LLM call here is for classification
  ONLY, not for answer generation). → SPEC-SYN-001 (M2).
- **Caching of classification results** (by query hash, normalized
  text, or any other key). v0 is a pure functional classifier.
  → Future SPEC (post-V1 if measured value).
- **Hot-reload of rules** (`rules.go` keyword tables are static at
  package init). → Out of scope; future SPEC if drift becomes a
  measured concern.
- **Configurable rule loading from YAML / koanf**. Rules ship
  hard-coded; the only runtime tunable is the
  `INTENT_ROUTER_LLM_MODEL` env var (and only impacts LLM model
  choice, not rules). → Future SPEC.
- **A `services/researcher/` Python sidecar invocation**. The LLM
  call goes through `internal/llm.Client.Complete` to the LiteLLM
  proxy directly. The Python researcher is for SYNTHESIS, not
  CLASSIFICATION. → SPEC-SYN-001 (M2).
- **Tool-use / structured-output API additions to
  `pkg/llm.Request`**. v0 uses string-prompt JSON output with
  parser-side validation. → Future SPEC-LLM-002 if measured value
  for synthesis or deep-research planner.
- **Prompt cache observability** (cache_creation_input_tokens vs
  cache_read_input_tokens reporting). → Future SPEC if we add
  `Response.CacheHit bool` to `pkg/llm.Response`.
- **Multi-adapter result fusion** (RRF, BGE-reranker). → SPEC-IDX-001
  (M3) RRF fusion.
- **Korean tokenization** (Meilisearch + mecab-ko). → SPEC-IDX-003 (M3).
- **Cardinality allowlist amendment** — SPEC-IR-001 explicitly DOES
  NOT amend SPEC-OBS-001's cardinality allowlist. The new
  `RouterClassifications` and `RouterClassificationDuration`
  collectors use only the existing `outcome` label name.
- **Admin / debug surface for routing-table dump**. The Router is
  a black box from CMD's perspective in v0. → Future SPEC if needed
  for operations.
- **Per-tenant rule customization** (e.g., a "Korean-team" tenant
  with stricter Korean threshold). → Future SPEC-AUTH-002 (M6) +
  rule-customization SPEC.
- **Localization beyond Korean and English** (Japanese, Chinese,
  Spanish). → Out of scope for V1 (`.moai/project/product.md:36-43`).
- **Streaming Classify** (incremental category-as-we-classify
  delivery). N/A — classification is sub-second in all paths.
- **Pre-flight tokenization or query expansion**. Query is opaque;
  the Router classifies the raw text.
- **GitHub Issue tracking on this SPEC** (skipped per session
  pattern — `--auto` mode).

### 2.3 Confidence Scoring Algorithm (Architecture)

[HARD] The rule-based scorer in `rules.go::Score(q RouterQuery) (Category, float64, []string)` is a deterministic, pure-function aggregator over four input signals. This subsection specifies the formula precisely so that golden tests can compute expected confidence values from the input query alone.

**Signals** (all derived deterministically from `q.Text`):

1. `hangul_ratio(q)` — count of Hangul runes / count of non-whitespace runes. Range: `[0.0, 1.0]`. Defined in §2.1(d) and computed by `korean.HangulRatio`.
2. `particle_density(q)` — count of whitespace-tokens whose suffix matches the 11-entry Korean particle list (을/를/이/가/은/는/에서/에/와/과/의) divided by total token count. Range: `[0.0, 1.0]`.
3. `kwd_density_C(q)` for each `C ∈ {web, social, academic}` — count of lowercased tokens that appear in the static `keyword_table[C]` divided by total token count. Range: `[0.0, 1.0]`.
4. `has_english_token(q)` — boolean: at least one token consists entirely of ASCII letters of length ≥ 3.

**Per-category raw scores** (let `r = hangul_ratio(q)`, `pd = particle_density(q)`, `ratio_high = 0.30`, `ratio_low = 0.10`):

- `score_korean(q)` =
  - if `r >= ratio_high`: `clamp(r + 0.4 + 0.1 * pd, 0, 1)`
  - elif `r >= ratio_low`: `0.3 + 0.5 * (r - ratio_low) / (ratio_high - ratio_low)`
  - else: `0.1 * pd` (rare: romanized Korean without Hangul)
- `score_academic(q)` = `clamp(0.8 * kwd_density_academic + 0.2 * (1 - r), 0, 1)`
- `score_social(q)`   = `clamp(0.7 * kwd_density_social + 0.2 * (1 - r) + 0.1 * indicator(kwd_density_social > 0), 0, 1)`
- `score_web(q)`      = `clamp(0.5 * kwd_density_web + 0.3 * (1 - r) + 0.2 * indicator(kwd_density_web > 0), 0, 1)`
- `score_mixed(q)` =
  - if `ratio_low <= r <= 0.40` AND `has_english_token(q)`: `0.5 + 0.4 * (1 - |r - 0.25| / 0.15)` (peaks at `r = 0.25`)
  - else: `0`
- `score_unknown(q)` = `1 - max(score_web, score_social, score_academic, score_korean, score_mixed)`

**Aggregation**:

1. Compute all six raw scores.
2. `winner = argmax_C raw_scores[C]`. Tie-break order (most-specific to least-specific): `academic > korean > social > mixed > web > unknown`.
3. `confidence = raw_scores[winner]`, already in `[0.0, 1.0]` from clamping.
4. `triggers` (returned for `Metadata.rule_triggers`) is the ordered list of signal names whose contribution to `raw_scores[winner]` was non-zero (e.g., `["hangul_ratio_high", "particle_density"]` when winner is `korean`).

**Determinism guarantees**:

- All keyword tables are static slices compiled at package `init()`.
- All thresholds (`τ_high = 0.85`, `ratio_high = 0.30`, `ratio_low = 0.10`) are package-level constants with `@MX:NOTE` per spec §5.6.
- Tie-break order is fixed in code as a constant slice.
- No randomness, no time-dependent inputs, no I/O.
- `Score` is pure: same input string → same `(Category, confidence, triggers)` triple, byte-for-byte.

**Worked example 1** — `q.Text = "transformer attention paper"`:

- Tokens: `["transformer", "attention", "paper"]`, `|T| = 3`
- `r = 0.0`, `pd = 0.0`
- `kwd_density_academic = 3/3 = 1.0` (all three are in academic keyword table per §5.4)
- `kwd_density_social = 0`, `kwd_density_web = 0`
- `score_korean = 0` (`r < ratio_low`)
- `score_academic = clamp(0.8*1.0 + 0.2*1.0, 0, 1) = 1.0`
- `score_social = clamp(0 + 0.2 + 0, 0, 1) = 0.2`
- `score_web = clamp(0 + 0.3 + 0, 0, 1) = 0.3`
- `score_mixed = 0` (`r` not in `[0.10, 0.40]`)
- `score_unknown = 1 - max(0.3, 0.2, 1.0, 0, 0) = 0.0`
- `winner = academic`, `confidence = 1.0`
- `1.0 ≥ τ_high` → no LLM escalation. Acceptance S-2 asserts `Confidence ≥ 0.85` (formula gives 1.0; passes).

**Worked example 2** — `q.Text = "best Korean GPT 모델 추천"` (the S-3 mixed query):

- Tokens: `["best", "korean", "gpt", "모델", "추천"]`, `|T| = 5`
- `r ≈ 0.18` (in ambiguous band `[0.10, 0.30]`); `pd = 0`; `has_english_token = true`
- `score_korean = 0.3 + 0.5 * (0.18 - 0.10) / (0.30 - 0.10) = 0.3 + 0.5 * 0.4 = 0.50`
- `score_mixed = 0.5 + 0.4 * (1 - |0.18 - 0.25| / 0.15) = 0.5 + 0.4 * 0.533 = 0.713`
- (other scores lower by inspection)
- `winner = mixed`, `confidence = 0.713`
- `0.713 < τ_high (0.85)` → **escalates to LLM**. Acceptance S-3 confirms exactly one LLM call.

**Worked example 3** — `q.Text = "ChatGPT 사용법과 프롬프트 엔지니어링 팁"` (the S-1 Korean-heavy query):

- `r ≈ 0.55` (well above `ratio_high`); `pd ≈ 0.1` (`과` particle in `사용법과`)
- `score_korean = clamp(0.55 + 0.4 + 0.1*0.1, 0, 1) = clamp(0.96, 0, 1) = 0.96`
- All other scores ≤ 0.20 by inspection
- `winner = korean`, `confidence = 0.96`
- `0.96 ≥ τ_high` → no LLM escalation; `0.96 ≥ 0.90` → S-1 assertion passes.

These three traces are reproduced in `internal/router/testdata/queries_golden.json` as fixtures with their expected `(Category, confidence_band)` tuples; `rules_test.go::TestRulesScoreFormulaTraces` walks the formula step-by-step on these fixtures and asserts each intermediate signal value within `±0.005` of the documented derivation.

---

## 3. EARS Requirements

### Functional Requirements

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| REQ-IR-001 | Ubiquitous | The Intent Router SHALL classify any non-empty `RouterQuery` into exactly one of the six categories `{web, social, academic, korean, mixed, unknown}`, returning a `RoutingDecision{Category, Confidence ∈ [0.0, 1.0], AdapterSet []string, Lang string, Source ClassificationSource, Metadata map[string]any}` value with no panic and no goroutine leak. | P0 | `TestClassifyReturnsRoutingDecision`, `TestClassifyAllSixCategoriesReachable` (driven by golden fixtures), `TestClassifyConfidenceInRange` (clamped to [0,1]). |
| REQ-IR-002 | Event-Driven | WHEN `Classify(ctx, q)` is invoked, the Router SHALL apply rule-based scoring first (deterministic, no I/O) and SHALL escalate to the LLM-fallback path only when the rule-based confidence < `confidenceThreshold` (default 0.85); the rule-based path SHALL NOT make any LLM call. | P0 | `TestClassifyHighConfidenceSkipsLLM` (assert no `obs.LLMCalls` increment when rule confidence ≥ τ_high); `TestClassifyLowConfidenceInvokesLLM`; `TestRuleBasedPathHasNoIO` (mock `llm.Client.Complete` to track invocation count = 0 when expected). |
| REQ-IR-003 | State-Driven | WHILE the LLM provider chain in `internal/llm` is unavailable (all providers' circuit breakers per SPEC-LLM-001 NFR-LLM-002 are simultaneously open OR `errors.Is(llmErr, llm.ErrAllProvidersFailed)`), the Router SHALL skip the LLM-fallback step, return the best rule-based result with `Source = SourceRuleBased`, set `Metadata["llm_unavailable"] = true` and `Metadata["degraded_confidence"] = true`, and SHALL NOT propagate the LLM error to the caller. | P0 | `TestClassifyDegradesGracefullyWhenLLMUnavailable` (inject a `llm.Client` whose `Complete` returns `llm.ErrAllProvidersFailed`); assert `Metadata["llm_unavailable"] == true`, `Source == SourceRuleBased`, the returned error is nil; counter `RouterClassifications{outcome="error_breaker_open"}` increments by 1. |
| REQ-IR-004 | Optional | WHERE the caller supplied a non-empty `RouterQuery.Lang` value, the Router SHALL use that value as the detected language and SKIP Hangul detection; the `Metadata["lang_override"]` field SHALL be set to `true`; the Hangul ratio MAY still be computed and stored in `Metadata["hangul_ratio"]` for diagnostic purposes. | P1 | `TestClassifyHonorsLangOverride` (set `Lang: "ja"` on a heavily Hangul query; assert `decision.Lang == "ja"` and `Metadata["lang_override"] == true`); `TestLangOverrideStillRecordsHangulRatio`. |
| REQ-IR-005 | Unwanted | IF `RouterQuery.Text` is empty OR contains only Unicode whitespace runes, THEN the Router SHALL return `(RoutingDecision{}, ErrInvalidQuery)` immediately, SHALL NOT invoke the LLM, SHALL increment `RouterClassifications{outcome="error_invalid"}` exactly once, and SHALL emit one slog WARN record with `request_id`, `error="ErrInvalidQuery"`. | P0 | `TestClassifyEmptyQueryReturnsErr` covers Text=`""`, Text=`"   "`, Text=`"\t\n  \r"`; assert no LLM call, no other counter increment except `error_invalid`. |
| REQ-IR-006 | Ubiquitous | The Router SHALL emit per-`Classify` invocation: (a) one increment on `obs.Metrics.RouterClassifications.WithLabelValues(outcome)` where outcome ∈ `{classified_web, classified_social, classified_academic, classified_korean, classified_mixed, classified_unknown, error_invalid, error_timeout, error_breaker_open, error_parse}`; (b) one observation on `obs.Metrics.RouterClassificationDuration.WithLabelValues(outcome)` with elapsed seconds; (c) one OTel span named `router.classify` started/ended within `Classify`, with attributes `router.category`, `router.source`, `router.confidence`, `router.lang`, `router.adapter_count`, recording the underlying error via `span.RecordError` on non-success outcomes; (d) one slog record at level INFO (success) or WARN (error_*) via `obs.Logger` with attributes `{request_id, category, source, confidence, lang, adapter_count, hangul_ratio, llm_used}`. The Router SHALL be nil-safe across `obs.Obs`, `obs.Metrics`, individual collectors, and `obs.Logger` per the pattern at `internal/llm/client.go:244-251`. | P0 | `TestEmitObservabilityForEachOutcome` (×10 outcomes); `TestEmitObservabilitySafeOnNilObs`; `TestEmitOTelSpanCapturesAttributes`; `TestEmitSlogRecordIncludesRequestID`. |
| REQ-IR-007 | Event-Driven | WHEN the LLM-fallback call exceeds the deadline of 2 seconds (enforced via `context.WithTimeout(ctx, 2*time.Second)` derived from the caller's ctx), the Router SHALL cancel the LLM call, return the best rule-based result with `Source = SourceRuleBased`, set `Metadata["llm_timeout"] = true` and `Metadata["degraded_confidence"] = true`, and increment `RouterClassifications{outcome="error_timeout"}` exactly once. The caller's ctx, if it has its own earlier deadline, SHALL be honoured ahead of the 2-second internal timeout. | P0 | `TestClassifyLLMTimeoutDegrades` (inject a `llm.Client` whose `Complete` blocks for 3s; assert returned `Metadata["llm_timeout"] == true`, `Source == SourceRuleBased`, `outcome="error_timeout"` counter +1, total elapsed ≤ 2.5s). |
| REQ-IR-008 | Ubiquitous | The Router SHALL select `RoutingDecision.AdapterSet` by INTERSECTING (a) the set of adapters whose `Capabilities.DocTypes` contains at least one DocType from `categoryEligibleDocTypes(decision.Category)` AND (b) the set of adapters whose `Capabilities.SupportedLangs` contains `decision.Lang` OR whose `Capabilities.SupportedLangs` is empty (= language-agnostic). The intersection SHALL be returned as a sorted (lexicographic by adapter name) `[]string`. WHEN `decision.Category == CategoryUnknown`, `categoryEligibleDocTypes` SHALL return the union of the web-eligible DocTypes (`{article, post, other}`) and the social-eligible DocTypes (`{post, social, video}`) — Unknown is a RECOVERABLE classification, NOT a terminal state, and is dispatched to a default ensemble of web-supporting and social-supporting adapters after Lang compatibility filtering. WHEN the intersection is empty for ANY Category (including Unknown), the Router SHALL fall back to the language-agnostic web set (adapters with empty SupportedLangs and DocType containing `article` or `other`) and set `Metadata["adapter_set_fallback"] = true`. | P0 | `TestSelectAdapterSetKoreanLang` (registry contains naver+daum+rss_korean+hackernews; `decision.Category=korean, Lang=ko` → adapter_set excludes hackernews); `TestSelectAdapterSetWebFallback` (registry only has language-agnostic adapters; korean Category → fallback web set + flag); `TestAdapterSetSorted` (lexicographic); `TestSelectAdapterSetUnknownCategory` (Unknown Category → web+social union, NOT empty AdapterSet). |

### Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| NFR-IR-001 | Performance (rule-based path) | The rule-based classification path (no LLM call invoked) SHALL execute with p50 ≤ 1 ms on a 100-character ASCII or 100-character Korean query, measured via `BenchmarkClassifyRulePath100Chars` running 10000 iterations on amd64. The benchmark lives at `internal/router/bench_test.go` and runs in CI on the same scheduled-weekly cadence as SPEC-OBS-001 NFR-OBS-001. Allocation count SHALL be bounded (target: ≤ 10 allocs/op). |
| NFR-IR-002 | Performance (LLM-fallback path) | The LLM-fallback classification path SHALL complete with p95 ≤ 3 seconds end-to-end (Classify entry to RoutingDecision return), measured via integration test `TestClassifyLLMFallbackP95UnderLimit` against a stub LiteLLM proxy that returns within 1.5s on average with a tail to 2.5s. Stricter contract: p99 ≤ 3s; the 2-second internal `WithTimeout` plus ~500ms degradation overhead bounds the worst case. |

---

## 4. Acceptance Criteria

### REQ-IR-001 — Six Categories Reachable

- File `internal/router/category.go` declares `Category` type as
  `string` with exactly six exported constants
  `CategoryWeb`/`CategorySocial`/`CategoryAcademic`/`CategoryKorean`/
  `CategoryMixed`/`CategoryUnknown`.
- `TestClassifyReturnsRoutingDecision` invokes `Classify(ctx, q)`
  on a fully-populated RouterQuery and asserts the returned
  `RoutingDecision` has all five required fields populated and
  `Confidence` in `[0.0, 1.0]`.
- `TestClassifyAllSixCategoriesReachable` is driven by the
  golden-file fixture set; for each of the 6 categories, at least
  ONE fixture exists that classifies into that category.
- `TestClassifyConfidenceInRange` asserts that for 100 random
  fuzzy queries, returned `Confidence ∈ [0.0, 1.0]` always
  (no NaN, no out-of-range).

### REQ-IR-002 — Confidence Gate

- `TestClassifyHighConfidenceSkipsLLM`: feed a clearly-Korean query
  (`"ChatGPT 사용법"`, 70% hangul); assert `decision.Source ==
  SourceRuleBased`; assert mock `llm.Client.Complete` was called
  ZERO times.
- `TestClassifyLowConfidenceInvokesLLM`: feed an ambiguous
  code-mixed query (`"Korean GPT review"`, 0% hangul but Korean
  semantic intent); assert mock `llm.Client.Complete` was called
  exactly ONCE.
- `TestRuleBasedPathHasNoIO`: assert when rule confidence ≥ 0.85,
  ZERO outbound LLM requests OR adapter calls happen.

### REQ-IR-003 — Circuit-Breaker Open Degradation

- `TestClassifyDegradesGracefullyWhenLLMUnavailable`: inject
  fake `llm.Client` returning `llm.ErrAllProvidersFailed`; ambiguous
  query; assert returned err is nil, `Metadata["llm_unavailable"]
  == true`, `Metadata["degraded_confidence"] == true`,
  `Source == SourceRuleBased`,
  `RouterClassifications{outcome="error_breaker_open"}` +1.

### REQ-IR-004 — Lang Override

- `TestClassifyHonorsLangOverride`: `RouterQuery{Text: "ChatGPT
  사용법", Lang: "ja"}` (heavily Korean text but caller
  hint = Japanese); assert `decision.Lang == "ja"`,
  `Metadata["lang_override"] == true`.
- `TestLangOverrideStillRecordsHangulRatio`: same fixture; assert
  `Metadata["hangul_ratio"] > 0.6` despite the override.

### REQ-IR-005 — Empty Query Rejection

- `TestClassifyEmptyQueryReturnsErr` table-drives `RouterQuery.Text
  ∈ {"", "   ", "\t\n  \r", "   "}`; for each case
  asserts `(RoutingDecision{}, ErrInvalidQuery)`,
  `RouterClassifications{outcome="error_invalid"}` +1, no LLM call.

### REQ-IR-006 — Per-Call Observability

- `TestEmitObservabilityForEachOutcome`: 10 sub-tests, one per
  outcome label; each fires the corresponding code path; asserts
  exactly ONE counter increment, ONE histogram observation, ONE
  span, ONE slog record per Classify.
- `TestEmitObservabilitySafeOnNilObs`: construct Router with
  `obs: nil`; Classify does not panic; returns valid
  RoutingDecision.
- `TestEmitOTelSpanCapturesAttributes`: in-memory exporter
  captures span; attribute set is exactly the documented 5 attrs.
- `TestEmitSlogRecordIncludesRequestID`: ctx with `reqid.WithContext(ctx, "TEST-REQ")`;
  captured slog JSON contains `"request_id":"TEST-REQ"`.

### REQ-IR-007 — LLM Timeout Degradation

- `TestClassifyLLMTimeoutDegrades`: inject `llm.Client.Complete`
  that blocks for 3s; ambiguous query; assert total elapsed ≤
  2.5s, `Metadata["llm_timeout"] == true`, `Source ==
  SourceRuleBased`, `RouterClassifications{outcome="error_timeout"}` +1.
- `TestClassifyHonorsParentDeadline`: ctx with
  `WithTimeout(ctx, 500*time.Millisecond)`; LLM stub blocks for
  3s; assert total elapsed ≤ 700ms (parent ctx wins over the 2s
  internal timeout).

### REQ-IR-008 — AdapterSet Selection by Capabilities

- `TestSelectAdapterSetKoreanLang`: registry contains 4 adapters
  with capabilities — `naver{Lang:[ko]}`, `daum{Lang:[ko]}`,
  `rss_korean{Lang:[ko]}`, `hackernews{Lang:[en]}`,
  `searxng{Lang:[]}`; classify Korean query; assert
  `AdapterSet = [daum, naver, rss_korean, searxng]` (alphabetical
  sorted, hackernews excluded).
- `TestSelectAdapterSetAcademicEnglish`: registry contains
  `arxiv{Lang:[],DocTypes:[paper]}`, `github{Lang:[],DocTypes:[repo,issue]}`,
  `naver{Lang:[ko]}`; classify English academic query; assert
  `AdapterSet = [arxiv, github]` (naver excluded by lang).
- `TestSelectAdapterSetWebFallback`: registry contains only
  `arxiv{Lang:[],DocTypes:[paper]}`; classify Korean query;
  assert `Metadata["adapter_set_fallback"] == true` and
  `AdapterSet ⊇ {searxng}` if present, else empty.
- `TestAdapterSetSorted`: register `{naver, daum, hackernews}`;
  ensure returned slice is `[daum, hackernews, naver]`.
- `TestSelectAdapterSetUnknownCategory`: registry contains
  `searxng{Lang:[],DocTypes:[article,other]}`,
  `hackernews{Lang:[en],DocTypes:[post,social]}`,
  `arxiv{Lang:[],DocTypes:[paper]}`; classify a query that scores
  as `CategoryUnknown` with `Lang="en"`; assert
  `AdapterSet == ["hackernews", "searxng"]` (Unknown's
  web+social DocType union matches searxng's `article` and
  hackernews's `post`+`social`; arxiv's `paper` is excluded
  because `paper` is in neither web nor social DocType set);
  `Metadata["adapter_set_fallback"]` is NOT set (intersection
  non-empty).

### NFR-IR-001 — Rule-Based Performance

- `BenchmarkClassifyRulePath100Chars` reports < 1 ms per op p50
  over 10000 iterations on amd64.
- `BenchmarkClassifyRulePathAllocs` reports ≤ 10 allocs/op.

### NFR-IR-002 — LLM-Fallback p95

- `TestClassifyLLMFallbackP95UnderLimit`: stub LLM with random
  delay sampled from `Exp(λ=0.7)` capped at 2.5s; 200 ambiguous
  queries; sort elapsed durations; assert `durations[190] ≤ 3.0s`
  (p95 = index 190 of 200).

---

## 5. Technical Approach

### 5.1 Files to Modify (Summary)

**Created (16 files)**:
- `internal/router/category.go` (REQ-IR-001, REQ-IR-008)
- `internal/router/category_test.go`
- `internal/router/query_input.go` (REQ-IR-001, REQ-IR-005)
- `internal/router/query_input_test.go`
- `internal/router/routing_decision.go` (REQ-IR-001, REQ-IR-006)
- `internal/router/routing_decision_test.go`
- `internal/router/korean.go` (REQ-IR-002, REQ-IR-004)
- `internal/router/korean_test.go`
- `internal/router/rules.go` (REQ-IR-001, REQ-IR-002)
- `internal/router/rules_test.go`
- `internal/router/llm.go` (REQ-IR-002, REQ-IR-007)
- `internal/router/llm_test.go`
- `internal/router/errors.go` (REQ-IR-005, REQ-IR-007)
- `internal/router/metrics.go` (REQ-IR-006)
- `internal/router/metrics_test.go`
- `internal/router/router_test.go` (orchestration + all REQs)
- `internal/router/bench_test.go` (NFR-IR-001)
- `internal/router/testdata/queries_golden.json` (30+ fixtures)
- `internal/obs/metrics/router.go` (NEW; declares
  `RouterClassifications` + `RouterClassificationDuration` collectors)

**Modified (3 files)**:
- `internal/router/router.go` — replace 4-line stub with full Router struct, New, Classify
- `internal/obs/metrics/metrics.go` — add 2 new fields to `Registry` struct; call `registerRouter(pr)` in `NewRegistry()`; NO change to `labelNames` allowlist
- `cmd/usearch/main.go` — minor wiring delta when adapter registry + LLM client both present

**Unchanged (by design)**:
- `internal/llm/*` — no API change; v0 uses string-prompt JSON
- `internal/adapters/registry.go` — no change; Router consumes existing API
- `pkg/types/*` — no change; Router uses Adapter/Capabilities/Query as-is
- `deploy/litellm/config.yaml` — no change (claude-haiku-4-5 already declared)
- Cmd `usearch-api` and `usearch-mcp` mains — IR-001 is library only; HTTP/MCP exposure is out of scope

### 5.2 Package layout

```
internal/router/
├── router.go                # Router, Options, New, Classify
├── router_test.go           # Orchestration tests
├── category.go              # Category enum, ClassificationSource, mapping
├── category_test.go
├── query_input.go           # RouterQuery, validation
├── query_input_test.go
├── routing_decision.go      # RoutingDecision, JSON marshal, Metadata key allowlist
├── routing_decision_test.go
├── korean.go                # HangulRatio, KoreanSignals, particle list
├── korean_test.go
├── rules.go                 # Rules, Score, keyword tables, package-init regexes
├── rules_test.go
├── llm.go                   # LLM-fallback prompt + parse + error handling
├── llm_test.go
├── errors.go                # Sentinels
├── metrics.go               # Outcome constants + helpers (NO prometheus import)
├── metrics_test.go
├── bench_test.go            # NFR-IR-001
└── testdata/
    └── queries_golden.json  # 30+ golden classifications

internal/obs/metrics/
└── router.go                # NEW: registerRouter + RouterClassifications collectors
```

### 5.3 Type sketches (illustrative; final shapes in run phase)

```go
// internal/router/router.go
type Options struct {
    Rules         *Rules
    LLMClient     *llm.Client
    Registry      *adapters.Registry
    Obs           *obs.Obs
    LLMModelOverride string         // INTENT_ROUTER_LLM_MODEL env
    LLMDeadline   time.Duration     // default 2s
    ConfidenceThreshold float64     // default 0.85
}

type Router struct {
    rules               *Rules
    llmClient           *llm.Client
    registry            *adapters.Registry
    obs                 *obs.Obs
    caps                map[string]types.Capabilities
    confidenceThreshold float64
    llmModelOverride    string
    llmDeadline         time.Duration
}

func New(opts Options) (*Router, error) { ... }
func (r *Router) Classify(ctx context.Context, q RouterQuery) (RoutingDecision, error)
```

### 5.4 LLM-fallback prompt sketch (v0, string-prompt JSON)

```go
const classifySystemPrompt = `You are a query intent classifier for a research meta-search engine. Output ONLY a JSON object with no preamble or trailing text.

Schema:
{
  "category": "<one of: web | social | academic | korean | mixed | unknown>",
  "confidence": <float 0.0-1.0>,
  "rationale": "<one sentence, max 200 chars>"
}

Categories:
- web: generic web search; news, blogs, general info
- social: Reddit, Hacker News, X/Twitter, Bluesky, YouTube, Polymarket
- academic: arXiv, GitHub repos/issues, papers, technical research
- korean: queries primarily targeting Korean-locale sources (Naver, Daum, Korean RSS, Korean news)
- mixed: multi-category intent (e.g., "Korean ML papers" → academic AND korean)
- unknown: cannot determine; rule-based fallback recommended

Examples:
[12-15 examples spanning all 6 categories with varying confidence]
`

// Total tokens after examples: 1100-1400 (above the 1024-byte prompt-cache minimum
// for Haiku 4.5; verified at run phase via Anthropic usage response).
```

### 5.5 Outcome enumeration

The `outcome` label values for `RouterClassifications` are
exactly 10:

| Outcome | Triggered by |
|---|---|
| `classified_web` | `decision.Category == web` AND no error |
| `classified_social` | `decision.Category == social` AND no error |
| `classified_academic` | `decision.Category == academic` AND no error |
| `classified_korean` | `decision.Category == korean` AND no error |
| `classified_mixed` | `decision.Category == mixed` AND no error |
| `classified_unknown` | `decision.Category == unknown` AND no error |
| `error_invalid` | `ErrInvalidQuery` returned |
| `error_timeout` | LLM-fallback exceeded 2s deadline |
| `error_breaker_open` | LLM unavailable / circuit open |
| `error_parse` | LLM response failed to parse / enum-validate |

This enumeration is the static, bounded label-value set referenced
by the test in `metrics_test.go`. No new label NAME is introduced
(only `outcome`, already in SPEC-OBS-001's allowlist at
`internal/obs/metrics/metrics.go:147-154`).

### 5.6 MX Tag plan

| File | Tag | Reason |
|------|-----|--------|
| `router.go::Router.Classify` | @MX:ANCHOR | fan_in ≥ 5 expected (FAN-001 + CLI-001 + SYN-001 + future debug + tests). @MX:REASON: sole sanctioned classification entry point |
| `router.go::Router.classifyByRules` | @MX:ANCHOR | fan_in ≥ 3 within Classify + tests. @MX:REASON: the deterministic path that all queries enter |
| `router.go::Router.classifyByLLM` | @MX:ANCHOR | fan_in ≥ 3 within Classify + tests. @MX:REASON: the network path; foot-gun if changed |
| `korean.go::HangulRatio` | @MX:NOTE | Magic constants (4 Unicode block ranges). @MX:NOTE explains threshold derivation |
| `rules.go::τ_high, ratio_high, ratio_low` | @MX:NOTE | Magic constants 0.85 / 0.30 / 0.10. @MX:NOTE documents empirical choice and future tuning surface |
| `llm.go::doClassifyByLLM` | @MX:WARN | Timeout-and-fall-through path silently downgrades confidence. @MX:REASON: caller may not realise LLM was attempted+failed unless they inspect Metadata |
| `rules.go::academicKeywords, socialKeywords` | @MX:NOTE | Curated word lists; sources documented (`.moai/project/product.md` persona language + manual curation) |
| `metrics.go::OutcomeClassified*` constants | @MX:NOTE | Static label-value enumeration; documented bounded cardinality (10 values) |

Per `.claude/rules/moai/workflow/mx-tag-protocol.md`: `[AUTO]`
prefix on agent-generated tags; `@MX:REASON` mandatory for ANCHOR
+ WARN; `@MX:SPEC: SPEC-IR-001` on all tags. Per
`.moai/config/sections/language.yaml` (`code_comments: en`), all
@MX descriptions in English.

### 5.7 Harness Level

Per `.moai/config/sections/harness.yaml` auto-routing: 8 REQs
(7 × P0 + 1 × P1) + 2 NFRs touching 1 package (8 sub-files in
internal/router/) + 1 cross-package edit (internal/obs/metrics/) +
1 minor cmd edit + 1 testdata file = **standard** harness level.
Sprint Contract is optional but recommended (`design.yaml` §11);
evaluator profile `default` applies. The IR-001's interaction
with SPEC-LLM-001 is contract-level only (uses existing API), so
the harness is no higher than CORE-001 / LLM-001.

---

## 6. Dependencies

### 6.1 Upstream SPEC Dependencies

- **SPEC-CORE-001 (implemented)**: provides `pkg/types.Adapter`,
  `pkg/types.Capabilities`, `pkg/types.Query`, `pkg/types.DocType`
  enum, `internal/adapters.Registry` with `List` / `Get` / `Register`.
  HARD dep — IR-001 cannot exist without it.
- **SPEC-LLM-001 (implemented)**: provides
  `internal/llm.Client.Complete`, `internal/llm.Request`,
  `internal/llm.Response`, `internal/llm.Classify` ModelClass with
  Haiku 4.5 → gpt-4o-mini → ollama priority, circuit-breaker
  semantics. HARD dep.
- **SPEC-OBS-001 (implemented)**: provides `obs.Obs` bundle,
  `obs.Logger` / `obs.Tracer` / `obs.Metrics`, `reqid.FromContext`,
  Prometheus collector registration pattern. HARD dep.

### 6.2 Parallelizable (with this SPEC's plan-phase)

- **SPEC-ADP-001 / SPEC-ADP-002 / SPEC-CLI-001 / SPEC-SYN-001**
  (all M2): each can begin its plan phase as soon as IR-001's
  spec.md is approved, in parallel with IR-001's run phase.
  RoutingDecision shape is the contract; once approved, downstream
  consumers can depend on it.

### 6.3 Downstream Blocked SPECs

- **SPEC-FAN-001 (M3)**: consumes `RoutingDecision.AdapterSet` and
  iterates the registry to dispatch in parallel.
- **SPEC-CLI-001 (M2)**: instantiates Router via `New()` for the
  CLI's `usearch query` command.
- **SPEC-SYN-001 (M2)**: gpt-researcher wrapper consumes
  `RoutingDecision` to inform planner sub-query generation.
- **SPEC-ADP-001 (Reddit, M2)**: declares `Capabilities.SupportedLangs:
  []` and `DocTypes:[post,social]`; classified into `social`
  Category by IR-001.
- **SPEC-ADP-002 (HN, M2)**: similar; `social` Category.

### 6.4 External Dependencies (run-phase pins)

No new Go module dependencies. `internal/router/` uses only:
- Go stdlib (`context`, `errors`, `regexp`, `strings`, `time`, `unicode`, `unicode/utf8`, `encoding/json`)
- `pkg/types` (already pinned)
- `internal/adapters` (already pinned)
- `internal/llm` (already pinned)
- `internal/obs` (already pinned)
- `go.opentelemetry.io/otel/{attribute,codes,trace}` (already pinned)

---

## 7. Risks (Summary — full risk table in research.md §6)

| Risk | Severity | Mitigation |
|------|----------|------------|
| Hangul-ratio false negative on Roman-letter Korean queries | Medium | LLM-fallback adjudication for ambiguous band; `mixed` exists for code-mixed |
| Prompt cache miss on cold proxy boot | Low | Document; pad system prompt above 1024-token threshold |
| Keyword-list staleness | Medium | Hot-editable in `rules.go`; counter ratios surface drift |
| LLM hallucination outside enum | Low | Parser strict-validates enum; degrade to rule-based + `error_parse` |
| Adapter registry empty at New | Low | `ErrAdapterRegistryEmpty` at construction |
| Confidence threshold 0.85 too aggressive | Medium | Tunable constant; revisit after M3 traffic |
| τ thresholds untested on real distribution | Medium | 30+ golden fixtures; revisit after CLI-001 ships and traffic flows |

---

## 8. Open Questions (8 entries in research.md §9)

These are explicitly unresolved at SPEC-approval time and documented
in the research artifact rather than pre-decided. They do not block
SPEC approval. See `.moai/specs/SPEC-IR-001/research.md` §9 for the
full annotated list.

---

## 9. References

External (cited in research.md):

- Context7 `/assafelovic/gpt-researcher` — planner/intent decomposition
- Context7 `/tmc/langchaingo` — RouterChain destination/default pattern
- Context7 `/itzcrazykns/perplexica` — focus-mode classification
- Context7 `/openai/openai-go` — Go SDK API
- https://platform.claude.com/docs/en/docs/build-with-claude/prompt-caching
- https://platform.claude.com/docs/en/docs/build-with-claude/tool-use
- https://en.wikipedia.org/wiki/Hangul_Syllables
- https://en.wikipedia.org/wiki/Korean_postpositions

Internal (project files; full citation count in research.md §8):

- `.moai/specs/SPEC-CORE-001/spec.md` — Adapter / Capabilities / Query contract
- `.moai/specs/SPEC-LLM-001/spec.md` — Client / Classify ModelClass / circuit-breaker
- `.moai/specs/SPEC-OBS-001/spec.md` — obs bundle, cardinality discipline
- `.moai/project/product.md` — V1 source categories
- `.moai/project/tech.md` — per-source adapter strategy table (§4)
- `.moai/project/structure.md` — `internal/router/` reservation (§1)
- `.moai/project/roadmap.md` — M2 SPEC-IR-001 row, parallelization plan
- `internal/router/router.go` — current 4-line stub
- `internal/llm/router.go`, `client.go`, `provider.go`, `retry.go` — pattern references
- `internal/adapters/registry.go` — Registry pattern reference
- `internal/obs/metrics/metrics.go` — registration pattern + cardinality allowlist
- `pkg/types/{adapter.go, capabilities.go, query.go}` — typed contract
- `deploy/litellm/config.yaml` — claude-haiku-4-5 alias declaration

---

*End of SPEC-IR-001 v0.1*
