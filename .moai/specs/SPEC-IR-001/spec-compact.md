# SPEC-IR-001 (Compact) — Intent Router v0

**Status**: draft | **Priority**: P0 | **Methodology**: TDD | **Coverage**: 85%
**Depends on**: SPEC-CORE-001, SPEC-LLM-001, SPEC-OBS-001
**Blocks**: SPEC-FAN-001, SPEC-CLI-001, SPEC-SYN-001, SPEC-ADP-001, SPEC-ADP-002

---

## REQ List

### Functional

- **REQ-IR-001** (Ubiquitous): Router classifies non-empty queries into one of {web, social, academic, korean, mixed, unknown} returning RoutingDecision{Category, Confidence∈[0,1], AdapterSet, Lang, Source, Metadata}.
- **REQ-IR-002** (Event-Driven): WHEN Classify invoked, rule-based scoring runs first; LLM-fallback only if confidence < τ_high (0.85).
- **REQ-IR-003** (State-Driven): WHILE LLM provider chain unavailable (`llm.ErrAllProvidersFailed`), skip LLM, return rule-based + Metadata["llm_unavailable"]=true + degraded_confidence flag, do NOT propagate error.
- **REQ-IR-004** (Optional): WHERE caller provides Lang override, skip Hangul detection, set Metadata["lang_override"]=true.
- **REQ-IR-005** (Unwanted): IF Text empty/whitespace, return ErrInvalidQuery without LLM call, increment outcome=error_invalid.
- **REQ-IR-006** (Ubiquitous): Per Classify: 1 counter + 1 histogram + 1 OTel span + 1 slog record; nil-safe across obs/Metrics/collectors/Logger.
- **REQ-IR-007** (Event-Driven): WHEN LLM exceeds 2s deadline, cancel, degrade to rule-based, set Metadata["llm_timeout"]=true, outcome=error_timeout.
- **REQ-IR-008** (Ubiquitous): AdapterSet = {Category-eligible DocTypes adapters} ∩ {SupportedLangs⊇detected_lang OR empty}; sorted; for Category=Unknown, eligible DocTypes = web ∪ social union (Unknown is recoverable, not terminal); web fallback flag set when intersection is empty for any Category.

### Non-Functional

- **NFR-IR-001**: Rule-based path p50 ≤ 1ms on 100-char query (BenchmarkClassifyRulePath100Chars; ≤10 allocs/op).
- **NFR-IR-002**: LLM-fallback path p95 ≤ 3s end-to-end.

---

## Acceptance Criteria (per scenario)

1. **S-1** Korean-heavy query → CategoryKorean, Confidence≥0.90, no LLM call, AdapterSet excludes English-only adapters
2. **S-2** Pure English academic → CategoryAcademic, no LLM call, AdapterSet={arxiv,github}
3. **S-3** 15% hangul mixed query → escalates to LLM exactly once, Source=LLMFallback, Category=Mixed
4. **S-4** Empty/whitespace query → ErrInvalidQuery, outcome=error_invalid, no LLM call
5. **S-5** LLM unavailable (ErrAllProvidersFailed) → rule-based + Metadata["llm_unavailable"]=true, outcome=error_breaker_open
6. **S-6** LLM blocks 3s → 2s timeout, rule-based + Metadata["llm_timeout"]=true, total elapsed ≤ 2.5s
7. **S-7** Korean lang adapter selection → excludes English-only, includes language-agnostic
8. **S-8** Empty intersection → web fallback flag set
9. **S-9** Benchmark rule-based p50 ≤ 1ms
10. **S-10** LLM-fallback p95 ≤ 3s over 200 queries
11. **S-11** Concurrent Classify (200 goroutines × 50 invocations) → race-clean
12. **S-12** Lang override on Korean text + Lang="ja" → decision.Lang="ja"
13. **S-13** Lang override on English text + Lang="ko" → decision.Lang="ko" + AdapterSet ko-filtered
14. **S-14** Web fallback when intersection empty
15. **S-15** Parent ctx deadline 500ms supersedes 2s internal timeout
16. **S-16** 10K-char oversized query → handled gracefully, no panic
17. **S-17** Malformed LLM JSON (4 sub-cases) → outcome=error_parse + degraded_confidence
18. **S-18** Empty registry at New → ErrAdapterRegistryEmpty

---

## Files to Modify

### Created (16)

```
internal/router/category.go
internal/router/category_test.go
internal/router/query_input.go
internal/router/query_input_test.go
internal/router/routing_decision.go
internal/router/routing_decision_test.go
internal/router/korean.go
internal/router/korean_test.go
internal/router/rules.go
internal/router/rules_test.go
internal/router/llm.go
internal/router/llm_test.go
internal/router/errors.go
internal/router/metrics.go
internal/router/metrics_test.go
internal/router/router_test.go
internal/router/bench_test.go
internal/router/testdata/queries_golden.json (30+ fixtures)
internal/obs/metrics/router.go
```

### Modified (3)

```
internal/router/router.go               (replace 4-line stub)
internal/obs/metrics/metrics.go         (+2 fields, +1 init call)
cmd/usearch/main.go                     (conditional Router wiring)
```

### Unchanged (by design)

- `pkg/types/*` (no contract change)
- `internal/llm/*` (uses existing `llm.Classify` ModelClass via `Request.Override`)
- `internal/adapters/registry.go` (consumes existing API)
- `deploy/litellm/config.yaml` (claude-haiku-4-5 already declared at line 23-26)

---

## Exclusions (HARD — not built in v0)

- HTTP/gRPC endpoint exposure (cmd/usearch-api routes deferred to SPEC-CLI-001/future SPEC-API-001)
- Adapter invocation / fanout (deferred to SPEC-FAN-001 M3)
- Caching of classification results (locked decision; future SPEC)
- Hot-reload of rules
- Configurable rule loading from YAML/koanf
- Tool-use / structured-output API additions to llm.Request (string-prompt JSON in v0)
- Prompt cache observability (cache_hit reporting)
- SPEC-OBS-001 cardinality allowlist amendment (NEW metric families use existing `outcome` label name only)
- Multi-language tokenization beyond Korean
- Per-tenant rule customization
- Streaming Classify

---

## Outcome Label Enumeration (10 values; no new label NAME)

```
classified_web | classified_social | classified_academic |
classified_korean | classified_mixed | classified_unknown |
error_invalid | error_timeout | error_breaker_open | error_parse
```

All emitted on `RouterClassifications.WithLabelValues(outcome)` and
`RouterClassificationDuration.WithLabelValues(outcome)`. The
`outcome` label NAME is already in SPEC-OBS-001's allowlist
(`internal/obs/metrics/metrics.go:147-154`).

---

## MX Tag Plan (8 tags)

- @MX:ANCHOR on `Router.Classify`, `classifyByRules`, `classifyByLLM` (fan_in ≥ 3-5)
- @MX:NOTE on Hangul Unicode-block constants, τ thresholds, keyword-table provenance, outcome enum
- @MX:WARN on `llm.go::doClassifyByLLM` timeout-and-fall-through path

All [AUTO]-prefixed; @MX:REASON mandatory for ANCHOR+WARN; @MX:SPEC: SPEC-IR-001 on each.

---

## Constants & Defaults

| Constant | Value | Rationale |
|---|---|---|
| `τ_high` (confidence threshold) | 0.85 | Empirical; tunable |
| `ratio_high` (Hangul → korean) | 0.30 | 30% hangul → almost certainly Korean intent |
| `ratio_low` (Hangul → non-korean) | 0.10 | < 10% hangul → almost certainly not Korean |
| LLM deadline | 2s | Internal `WithTimeout`; parent ctx wins if shorter |
| `INTENT_ROUTER_LLM_MODEL` env | empty (= use Class default) | Override; `claude-haiku-4-5` is the default via `llm.Classify` priority |
| `MaxRetries` (LLM) | inherited from SPEC-LLM-001 | 3 attempts via `withRetry` |
| Korean particle count | 11 | Common postpositions: 을/를/이/가/은/는/에서/에/와/과/의 |
| Hangul Unicode blocks | U+AC00-D7A3, U+1100-11FF, U+3130-318F, U+A960-A97F | Standard Unicode |

---

## Open Questions (deferred to run phase; see research.md §9)

1. Cache map[name]Capabilities at New OR fresh per Classify? **Default: cache at New**.
2. LLM returns non-enum category → reject + rule-based fallback? **Default: reject**.
3. Per-request prompt cache hit rate target? **Default: none in v0**.
4. LLM confidence vs rule confidence merge policy? **Default: LLM wins; rule shadowed in Metadata**.
5. Lang override + Hangul disagreement → log only? **Default: log DEBUG, override wins**.
6. Unknown Category → empty AdapterSet OR web+social fallback? **RESOLVED in REQ-IR-008**: Unknown dispatches to the web+social DocType union (`{article, post, other}` ∪ `{post, social, video}`) intersected with Lang-compatible adapters. Unknown is RECOVERABLE, not terminal.
7. LLM and rule disagree → LLM wins? **Default: LLM wins**.
8. YAML-driven rule customization? **Default: NO (locked)**.

---

*Compact spec generated 2026-04-26 from spec.md v0.1*
