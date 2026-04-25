# Research — SPEC-IR-001 Intent Router v0

**Status**: research artifact (Plan-phase deliverable)
**Author**: limbowl via manager-spec
**Created**: 2026-04-26
**SPEC**: SPEC-IR-001
**Milestone**: M2 — First end-to-end slice
**Depends on (research scope)**: SPEC-CORE-001, SPEC-LLM-001, SPEC-OBS-001, SPEC-BOOT-001, SPEC-DEP-001

---

## 0. Problem Framing

The Intent Router is the FIRST real consumer of the SPEC-CORE-001 contract.
SPEC-BOOT-001 reserved `internal/router/router.go` as a 4-line stub
(`internal/router/router.go:1-4`); SPEC-CORE-001 published `Adapter`,
`Capabilities`, `Query`, and the registry pattern (`internal/adapters/registry.go:75-167`);
SPEC-LLM-001 delivered the LLM client with a pre-existing `Classify`
ModelClass (`internal/llm/provider.go:34-38`) routed to Haiku 4.5 with
gpt-4o-mini and Ollama fallthrough. SPEC-IR-001 lives at the intersection.

The Router's job is one well-defined function:

```
RouterQuery → (Category, Confidence, AdapterSet, Lang, Source, Metadata)
```

It does NOT invoke adapters. It does NOT do fanout. It DECIDES which
adapters should be invoked. SPEC-FAN-001 (M3) consumes the
`RoutingDecision.AdapterSet` and dispatches to the registry.

The router is a **pipeline**: deterministic Hangul-ratio detection →
keyword rule scoring → confidence gate → optional Haiku LLM adjudication →
adapter set selection via `Capabilities.SupportedLangs ∩ Category-eligible`.

---

## 1. Existing Patterns in the Universal Search Repo

### 1.1 Provider router shape (PROVIDER router, not intent router)

`internal/llm/router.go:148-198` is the closest precedent — a tiny,
sync.RWMutex-protected struct that selects providers from a static
priority map. The IR Router will mirror its shape:

| Aspect | LLM Router | Intent Router |
|---|---|---|
| Concurrency | `sync.RWMutex` reads dominate | Same — `Classify` is a hot read path; rules are immutable post-construction |
| Static config | `priorities map[ModelClass][]ProviderRef` | `Rules` struct with keyword tables and thresholds |
| Per-call state | `breakers` map | None (v0 — no caching, no per-query state) |
| Method shape | `Route(ctx, ModelClass) ([]ProviderRef, error)` | `Classify(ctx, RouterQuery) (RoutingDecision, error)` |

(`internal/llm/router.go:151-157` — Router struct fields)
(`internal/llm/router.go:176-198` — Route method)

The intent router is **simpler** than the provider router: there's no
circuit breaker (the IR doesn't talk to flaky external services
itself; the LLM call goes through `internal/llm` which already has
its own circuit breaker per SPEC-LLM-001 NFR-LLM-002).

### 1.2 Per-call observability emit pattern

`internal/llm/client.go:230-252` is the canonical observability emit:

```go
func (c *defaultClient) emitObservability(ctx context.Context, provider, model, outcome string, ...) {
    rid := reqid.FromContext(ctx)
    c.obs.Logger.InfoContext(ctx, "llm call",
        slog.String("request_id", rid),
        slog.String("provider", provider),
        slog.String("model", model),
        ...
    )
    reg := c.obs.Metrics
    if reg != nil && reg.LLMCalls != nil {
        reg.LLMCalls.WithLabelValues(provider, model, outcome).Inc()
    }
    ...
}
```

Two invariants from this pattern that SPEC-IR-001 MUST mirror:

1. **Nil-safety on every collector access** — the obs bundle, the
   Metrics registry, AND each collector pointer can independently be
   nil. The pattern at `internal/llm/client.go:244-251` checks all three.
   SPEC-IR-001's `Router.emit` will use the same triple-guard.
2. **Use `reqid.FromContext(ctx)`** for the `request_id` slog attr
   (`internal/obs/reqid/reqid.go:1-30`).

Same shape applies to the adapter wrappedAdapter (`internal/adapters/registry.go:223-252`).

### 1.3 Adapter registry capability access

`internal/adapters/registry.go:147-152` exposes `Get(name) (Adapter, bool)`,
`internal/adapters/registry.go:157-166` exposes `List() []string` (sorted
lexicographically per REQ-CORE-005). The intent router calls
`registry.List()` ONCE at startup to enumerate adapter names, then
calls `registry.Get(name)` PER NAME to read `Capabilities()`. We
can EITHER cache the resulting `map[string]Capabilities` at
`Router.New()` time OR read it fresh per-Classify.

**Decision**: cache at construction. Capabilities are deterministic
per `pkg/types/capabilities.go:25-37` ("MUST be deterministic — calling
Capabilities twice on the same adapter returns equal values"); reading
fresh per Classify wastes RWMutex acquisitions on the hot path.
Cache invalidation = process restart (V1 has no hot-reload of
adapters).

### 1.4 Capabilities-driven adapter selection

`pkg/types/capabilities.go:38-62` declares the relevant fields:

```go
type Capabilities struct {
    SourceID          string
    DisplayName       string
    DocTypes          []DocType
    SupportedLangs    []string  // BCP-47; empty = language-agnostic
    SupportsSince     bool
    RequiresAuth      bool
    AuthEnvVars       []string
    RateLimitPerMin   int
    DefaultMaxResults int
    Notes             string
}
```

The two fields IR-001 reads:

- **`SupportedLangs`**: language gate. Empty = language-agnostic
  (e.g., `searxng` web fanout). Otherwise BCP-47 list (e.g.,
  `naver` → `["ko"]`).
- **`DocTypes`**: drives Category eligibility. The mapping from
  Category → eligible DocTypes is static (defined in `category.go`):
  - `web` → `{article, post, other}`
  - `social` → `{post, social, video}`
  - `academic` → `{paper, repo, issue}`
  - `korean` → ANY (Korean Category is language-driven, not type-driven)
  - `mixed` → ANY
  - `unknown` → fall back to `web` + `social` set

### 1.5 LLM client.Classify shape

`internal/llm/provider.go:34-38` already declares:

```go
Classify: {
    {Provider: "anthropic", Model: "claude-haiku-4-5"},
    {Provider: "openai", Model: "gpt-4o-mini"},
    {Provider: "ollama", Model: "ollama/llama3.1-small"},
},
```

This is a HUGE simplification: SPEC-IR-001 does NOT instantiate a
new LLM client; it consumes the existing one. The Router struct
holds `*llm.Client`; `Classify` calls `client.Complete(ctx,
llm.Request{Class: llm.Classify, ...})` and gets fallthrough +
circuit-breaker for free per SPEC-LLM-001 REQ-LLM-004 / NFR-LLM-002.

Key implication: when SPEC-IR-001's `INTENT_ROUTER_LLM_MODEL` env
is unset, the request uses `Class: Classify` (Haiku). When it is
set (e.g., `claude-sonnet-4-6` for evaluation), it routes via
`Request.Override` (`internal/llm/client.go:107-110`):

```go
model := ref.Model
if req.Override != "" {
    model = req.Override
}
```

This passes through the LiteLLM proxy — ANY alias in
`deploy/litellm/config.yaml` is valid (`deploy/litellm/config.yaml:11-44`
— claude-haiku-4-5 already declared on line 23-26).

### 1.6 LLM circuit breaker visibility

The IR cannot directly inspect the LLM circuit breaker state
(`internal/llm/router.go:64-84` — `breaker.Allow()` is unexported and
the breaker is per-Router internal state). Instead, the IR detects
"circuit open" indirectly: `client.Complete` returns
`ErrAllProvidersFailed` (`internal/llm/client.go:99` — last fallthrough
return). When ALL providers' breakers are open OR all classify-tier
providers fail, that error surfaces. SPEC-IR-001 catches it via
`errors.Is(err, llm.ErrAllProvidersFailed)` and degrades to
rule-based result with `Source = RuleBased` and a flag bit on
`RoutingDecision.Metadata` indicating LLM was unavailable.

Same applies to `ErrModelNotConfigured` (which would occur if the
`Classify` model class is somehow missing from `defaultPriorities` —
defensive guard, not expected in V1).

### 1.7 LLM timeout & cancellation

`internal/llm/client.go:51-54` configures a HTTP client timeout
from `cfg.TimeoutSeconds` (default per
`internal/llm/config/config.go`). SPEC-IR-001 imposes a TIGHTER
deadline via `context.WithTimeout(ctx, 2*time.Second)` per the
locked decision (no LLM call exceeds 2s). On `context.DeadlineExceeded`
(or any error during the LLM phase), the router returns the
rule-based result and sets `Metadata["llm_timeout"] = true`.

### 1.8 The 30% / 10% Hangul threshold rationale

The Hangul block ranges are well-defined Unicode standards
(verified via Wikipedia `Hangul_Syllables`):

| Block | Range | Use |
|---|---|---|
| Hangul Syllables | U+AC00..U+D7A3 | Modern composed Korean characters (the bulk) |
| Hangul Jamo | U+1100..U+11FF | Component letters (initial/medial/final) |
| Hangul Compatibility Jamo | U+3130..U+318F | Legacy single-letter forms |
| Hangul Jamo Extended-A | U+A960..U+A97F | Old Korean (rare in queries but counted) |

**Threshold reasoning**:
- ratio ≥ 0.30 (30%) → almost certainly a Korean-locale query
  (mixed code-switched queries usually have hangul ratio < 0.30
  because Latin-letter words dominate token count).
- ratio < 0.10 (10%) → almost certainly NOT Korean-primary (a few
  loanwords, brand names, or punctuation don't change category).
- 0.10 ≤ ratio < 0.30 → AMBIGUOUS: e.g., `"Korean GPT 사용법"`
  (Korean usage of GPT) is 14% hangul but is genuinely Korean-locale
  intent. LLM adjudication is justified for this band.

The `function-words signal` (presence of any Korean particle from
the list `[을, 를, 이, 가, 은, 는, 에서, 에, 와, 과, 의]`) is
boolean. When ratio < 0.10 AND no particles → `non_korean` is
deterministic. When ratio < 0.10 AND particles present → escalate
to LLM (rare but possible: `"GPT를 추천해줘"` is 25% hangul, well
above the lower bound; but `"Korean GPT를"` could be 12% hangul
with particle).

### 1.9 Logger pattern with WarnContext / InfoContext

`internal/llm/client.go:215-222` shows the `WarnContext(ctx, msg, ...)` form.
SPEC-IR-001 uses INFO for normal classification, WARN for LLM-fallback
errors, DEBUG for ambiguous-band escalations.

---

## 2. Reference Designs (External)

### 2.1 GPT-Researcher (`/assafelovic/gpt-researcher`)

GPT-Researcher uses LLM-driven planning rather than rule-based
classification — its `planner.create_plan` calls a LLM to decompose
a research question into sub-queries. The relevant pattern for IR-001
is the **structured-output prompt** they use to force JSON: a
system prompt enumerates the JSON schema, then the user prompt is
the question. We adopt the same approach for the LLM-fallback
adjudication step.

### 2.2 LangChain-Go (`/tmc/langchaingo`)

LangChain-Go has a `RouterChain` abstraction that maps inputs to
named destination chains based on an LLM-emitted route key.
The key insight from LangChain: **the LLM ALWAYS returns a
`destination: <category>` field with a fallback `default` route**.
SPEC-IR-001 adopts the same convention — the LLM's JSON output
includes `category` AND `confidence` AND `rationale`, with `unknown`
as the explicit fallback when none of the six categories fit.

### 2.3 Perplexica (`/itzcrazykns/perplexica`)

Perplexica uses a small set of **focus modes** (`webSearch`, `academic`,
`writing`, `wolframAlpha`, `youtube`, `reddit`) and routes via an
LLM-generated JSON object. The mapping is more user-driven (focus
mode is a UI toggle); IR-001 makes it automatic. The shape is
similar: small enum, single LLM call, structured JSON.

### 2.4 OpenAI Function Calling / Anthropic Tool Use

Anthropic's tool-use API lets us force structured output:

```json
{
  "tools": [
    {
      "name": "classify_query",
      "description": "Classify a search query into one of six categories",
      "input_schema": {
        "type": "object",
        "properties": {
          "category": {
            "type": "string",
            "enum": ["web", "social", "academic", "korean", "mixed", "unknown"]
          },
          "confidence": { "type": "number", "minimum": 0, "maximum": 1 },
          "rationale": { "type": "string", "maxLength": 200 }
        },
        "required": ["category", "confidence"]
      }
    }
  ],
  "tool_choice": { "type": "tool", "name": "classify_query" }
}
```

`tool_choice = {type: "tool", name: ...}` FORCES the model to
emit a `tool_use` block — no free-text fallback (verified via
Anthropic docs: "Strict tool use" guarantees schema conformance
when `strict: true` is added to the tool definition).

**However**, SPEC-LLM-001 v1's `llm.Request` shape does NOT yet
expose tool definitions or `tool_choice` (REQ-LLM-002 explicitly
defers tool-use to a future SPEC: "Tool calls, structured outputs
... extensions land in later SPECs as features consume them"
— `.moai/specs/SPEC-LLM-001/spec.md:95-105`).

This is a **friction point**. Two options:

**Option A** — extend `llm.Request` with a `JSONSchema` field that
maps to OpenAI/Anthropic tool-use under the hood. This is a
SPEC-LLM-001 amendment (small surface delta) and unblocks every
future structured-output use case (synthesis, dedup classification,
deep-research planner). Recommended.

**Option B** — string-prompt JSON parsing (less reliable, validate
with `json.Unmarshal` and fall through to rule-based if parse
fails). Pragmatic for v0; can amend in future.

**SPEC-IR-001 v0 decision** (documented in §11 Open Questions):
**Option B** for v0 (no SPEC-LLM-001 amendment in M2). The LLM
prompt strictly instructs the model to emit ONLY a JSON object
matching `{"category": "...", "confidence": 0.X, "rationale": "..."}`.
Parse failure → rule-based result, log WARN, count once on
`outcome=error_parse`. Migration to Option A is a future SPEC-IR-002
or SPEC-LLM-002 amendment.

### 2.5 Anthropic Prompt Caching

Per `https://platform.claude.com/docs/en/docs/build-with-claude/prompt-caching`
(verified 2026-04-26):

| Metric | Value |
|---|---|
| Default TTL | 5 minutes |
| Extended TTL | 1 hour (2x cost) |
| Cache write cost | 1.25× base input price |
| Cache read cost | 0.10× base input price (90% savings) |
| Minimum cacheable length | 1024-4096 tokens (model-dependent) |

For Haiku 4.5, the system-prompt template (≈800 tokens with 8-10
few-shot examples) MAY fall below the 1024-token minimum threshold
on some models. **Implementation requirement**: pad the system
prompt with sufficient few-shot examples to comfortably exceed
1024 tokens. ETA in `llm.go`: 12-15 examples covering each of the
6 categories twice, 1.2-1.6 KB.

**Caching mechanic at the LiteLLM proxy boundary**: LiteLLM passes
Anthropic's `cache_control` field through transparently when
`anthropic/...` model is targeted. The Go client's
`llm.Request` does not currently expose `cache_control` — but
`x-litellm-response-cost` header inclusion already proves
LiteLLM passes Anthropic-specific response fields back. Caching
will be transparent if the system prompt is identical bytes
across calls (deterministic ordering of map keys, etc.).

**Risk**: if `Request.System` is constructed via `fmt.Sprintf`
with a date or counter, every call writes a fresh cache entry.
Implementation MUST use a constant-string system prompt at
package level.

### 2.6 STORM (Stanford OVAL)

STORM uses topic classification but at a different level
(article-section topic generation, not query intent). Less directly
applicable; cited for completeness.

### 2.7 last30days-skill (mvanhorn)

last30days uses category labels (`reddit`, `hackernews`, `youtube`,
`twitter`, etc.) tied to specific scoring formulas. The IR's
`social` category collapses several of these. last30days does NOT
do query-side classification — its category dispatch is implicit
(every category is queried in parallel for every input). The
opposite design choice; cited as the alternative we explicitly
rejected for v0 (rationale: latency & cost — querying 12 adapters
on every query when 4 of them obviously don't apply is wasteful).

---

## 3. Korean Detection Precedents

### 3.1 naver-search-mcp (isnow890)

The naver-search-mcp adapter (referenced from
`.moai/project/tech.md:116`) accepts both Korean and Latin-letter
queries; Naver itself handles routing. The MCP doesn't classify —
it dispatches all queries to the appropriate Naver endpoint.
Implication for IR-001: the `naver` adapter declares
`SupportedLangs: ["ko"]` (filtering happens BEFORE the adapter
is dispatched), but the adapter would technically accept any
input. Filtering at the IR boundary keeps non-Korean queries from
hitting the Naver API quota wastefully (25,000/day per
`.moai/project/tech.md:116`).

### 3.2 Meilisearch Korean tokenization

Meilisearch's default tokenizer is documented as weak for Korean
(`.moai/project/tech.md:50`); the project's V1 plan addresses
this with mecab-ko sidecar (SPEC-IDX-003, M3, blocked by
SPEC-IR-001). IR-001 does NOT touch Meilisearch — it only
classifies; tokenization is a downstream concern.

### 3.3 Korean function-word signal

The 11 selected particles (을, 를, 이, 가, 은, 는, 에서, 에, 와,
과, 의) have very high frequency in Korean text and almost zero
collision with Latin/Chinese/Japanese strings. They are NOT
sufficient to classify alone (a Korean noun list might have 0
particles), but they are a powerful **disambiguator** in the
ambiguous band:

- Hangul ratio = 12%, function word present → likely Korean
- Hangul ratio = 12%, no function word → likely code-mixed but
  primary intent is non-Korean (escalate to LLM)

The list comes from `https://en.wikipedia.org/wiki/Korean_postpositions`
(common subject/object/topic markers). The list is a STATIC
constant in `korean.go` — not user-extensible in v0.

---

## 4. Prompt Engineering for the LLM Fallback

### 4.1 System prompt template (sketch — concrete content lives in spec.md)

The system prompt has three sections:

1. **Role** (~50 tokens): "You are a query intent classifier for a
   research meta-search engine. Output ONLY a JSON object."
2. **Categories enumeration with rubric** (~200 tokens): one line per
   category describing its scope with examples.
3. **Few-shot examples** (~600-800 tokens): 12-15 examples covering
   each category 2-3 times, including ambiguous cases. Examples
   include input + expected JSON output.

Total: ~1100 tokens — **just** above the cache minimum. Pad with
2 extra examples if the first cache write reports `cache_creation_input_tokens
< 1024`.

### 4.2 User prompt template

Single message: `Classify this query: "<query text>"\nReturn ONLY
the JSON object.`

Variable bytes per call ≈ query length + 35 fixed bytes. NOT cached
(intentional — only the system prompt benefits from caching).

### 4.3 Generation parameters

- `max_tokens`: 100 (more than enough for `{"category":"academic",
  "confidence":0.85,"rationale":"..."}`).
- `temperature`: 0 (deterministic; same query → same classification).
- `top_p`: not set (default).

### 4.4 Response shape & parsing

Expected:

```json
{
  "category": "academic",
  "confidence": 0.85,
  "rationale": "Query mentions arxiv-style topic with author names"
}
```

Parser (in `llm.go`):

1. Trim whitespace from response.
2. If response starts with ` ```json` or ` ``` `, strip code-fence wrapping.
3. `json.Unmarshal` into a struct with three fields.
4. Validate `category` is one of the 6 enum values — else rule-based.
5. Clamp `confidence` to `[0.0, 1.0]`.
6. Truncate `rationale` to 200 chars.

Five failure modes, all degrade to rule-based (none panic):
- Empty response.
- Response that is not valid JSON after fence-stripping.
- `category` field missing.
- `category` field is not in the enum.
- `confidence` field is missing or negative.

Each failure mode increments `outcome=error_parse` (or its specific
sub-variant if we choose to differentiate — v0 collapses to one
label).

---

## 5. Rejected Alternatives

| Alternative | Reason rejected |
|---|---|
| Local fine-tuned BERT classifier (e.g., distilBERT-multilingual) | (a) Adds Python dep into the Go orchestration plane (we already deferred Python ML to `services/embedder` — IR is in the hot Go path). (b) Maintenance overhead (model file storage, retraining cycle, CI inference test). (c) The LLM fallback already covers <15% of queries (only the ambiguous band) — sub-millisecond classifier vs network call to Haiku is the wrong optimisation. (d) Redundant with the LLM. |
| Pure rule-based, no LLM | Korean-mixed queries ("Korean GPT 추천") hit the ambiguous band. Rules can't disambiguate "Korean ML papers" (academic + korean = mixed) from "the news from Korea" (korean only). |
| Pure LLM, no rules | (a) Latency: 2s p95 LLM round-trip vs 1ms rule-based on every query is a 2000× regression. (b) Cost: at $0.25/1M input tokens for Haiku 4.5, every 100k queries costs ~$5 — cheap, but unnecessary when rules suffice for >85% of cases. (c) Anthropic outage = router unavailable. |
| CL+T (Chinese-Local Tokenizer) | Not relevant — universal-search V1 does not target Chinese-locale (`.moai/project/product.md:36-43` — V1 is web/social/academic/Korean). Future SPEC if Japanese or Chinese are added. |
| LLM as primary, rules as fallback (inverted order) | More expensive. Current order: cheap path first, expensive fallback. |
| Full chain-of-thought reasoning prompt | Wastes tokens. Classification is short-output; force structured JSON. |
| Caching the rule-based result by query hash | Out of scope for v0 (locked decision). Future SPEC. |
| HTTP/gRPC endpoint at `cmd/usearch-api/` | Out of scope for v0 (locked decision). The Router is a library function consumed in-process by SPEC-FAN-001 / SPEC-CLI-001 / SPEC-SYN-001. |

---

## 6. Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Prompt cache miss on every cold proxy boot | High | Low (extra cost ~$0.001 per cold start, infrequent) | Document; pad system prompt above the 1024-token threshold; same prompt-bytes invariant in tests |
| Hangul-ratio false negative on roman-letter Korean queries (`"Korean GPT"`) | Medium | Medium (mis-routes Korean intent to web) | LLM-fallback adjudication for ambiguous band catches these; `mixed` category exists for true code-mixed queries |
| Hangul-ratio false positive on Korean character names embedded in non-Korean queries (`"라인 IPO 분석"` 80% hangul but actually finance/news) | Low | Low | Both `korean` AND `web` adapters dispatched would still cover; the cost is one extra Naver API call |
| Keyword-list staleness (social platforms rebrand: X-was-Twitter, "Bluesky" recently added) | Medium | Low | Keyword tables are hot-editable in `rules.go`; drift is observable via per-category counter ratio over time |
| LLM hallucination on classification (returning a non-enum category) | Low | Low | Strict enum validation in parser; fallback to rule-based on enum mismatch |
| Forced-JSON format drift across LLM versions (Haiku 4.5 vs 5.0) | Low | Medium (if every call fails to parse) | Pin `claude-haiku-4-5` in env default; canary the prompt against new model before bumping the LiteLLM config |
| Anthropic outage cascades to ALL of our Anthropic-priority requests (sync, deep, IR) | Low | Medium | Each request goes through the LLM router with circuit breaker; IR specifically degrades to rule-based (still functional) on `ErrAllProvidersFailed` |
| Adapter registry empty at Router construction | Low | High (Router becomes useless) | `New()` returns `ErrAdapterRegistryEmpty` if `registry.List()` is empty; CMD callers wire registry first |
| Korean function-word list collision with non-Korean text | Very Low | Low | The 11 particles use exclusively hangul codepoints; no collision possible with Latin/Latin-extended text |
| Rule-based path exceeds 1ms p50 due to inefficient regex compilation | Low | Low | Compile regexes once at package init (Go convention); benchmark gate at NFR-IR-001 |
| `metadata` enrichment of RoutingDecision becomes a free-for-all | Medium | Low | Document allowed keys in spec.md; future lint test in M8 if drift observed |
| LLM Override env var allows arbitrary model alias including non-classify-tier models | Medium | Low | If user sets `INTENT_ROUTER_LLM_MODEL=gpt-4o`, that's their cost. Document `claude-haiku-4-5` as default and recommend `*-mini` / `*-haiku` tier |
| Confidence threshold τ_high too aggressive (0.85), causing too many LLM escalations | Medium | Medium (LLM volume + cost) | Tunable via constant in `rules.go`; empirical adjustment after M3 traffic |
| The 30%/10% Hangul ratio thresholds untested on real query distribution | Medium | Low | 30+ golden fixtures cover 6 categories with 5 representative queries each; revisit after SPEC-CLI-001 or SPEC-MCP-001 ships and real traffic flows |

---

## 7. Performance Budget

### Rule-based path (target: NFR-IR-001 ≤ 1ms p50 for 100-char query)

| Step | Cost (rough) |
|---|---|
| `HangulRatio(text)` — single pass over rune slice | ~200 ns for 100 chars |
| `KoreanSignals(text)` — substring search on 11 particles | ~500 ns |
| Keyword rule scan (15-20 categories × ~10 keywords each, hash-set probe) | ~5-10 μs |
| Confidence score computation | ~100 ns |
| `selectAdapterSet(category, lang)` — registry list iteration ≤ 14 adapters with capability-cache hits | ~1-2 μs |
| Marshal RoutingDecision (in caller) | not in IR scope |
| **Total** | **~10-15 μs p50** ≪ 1 ms target. Comfortably under budget. |

### LLM-fallback path (target: NFR-IR-002 ≤ 3s p95)

| Step | Cost |
|---|---|
| Construct prompt | ~100 μs |
| Network round-trip to LiteLLM proxy | ~50 ms (intra-host) |
| LiteLLM → Anthropic API | ~0.8-2.5s p95 (Haiku 4.5; Anthropic SLA-ish) |
| LiteLLM → response with cost header | included above |
| openai-go parse + return | ~100 μs |
| JSON parse + enum validate | ~10 μs |
| Adapter set selection | ~1 μs |
| **Total** | **~1-2.5s p95** under budget. Cap at 2s deadline (caller's `WithTimeout`) leaves 1s headroom. |

### Total budget at p99 (combined paths)

Most queries take rule-based path (≤ 1ms). LLM-fallback path adds ~2s
on the ambiguous band (~15% of queries per estimate). Effective p99
across all queries depends on rule:LLM ratio — for 85:15 split,
overall p99 is dominated by the 15% LLM tail.

---

## 8. File:Line Citation Index

This research artifact references the following file:line locations
(target: 30+ unique citations).

### 8.1 Internal (Universal Search repo)

1. `internal/router/router.go:1-4` — current 4-line stub
2. `internal/llm/router.go:148-198` — provider router (pattern reference)
3. `internal/llm/router.go:151-157` — Router struct fields
4. `internal/llm/router.go:64-84` — breaker.Allow state machine
5. `internal/llm/router.go:176-198` — Route method
6. `internal/llm/client.go:230-252` — emitObservability shape
7. `internal/llm/client.go:69-100` — Complete with retry/fallthrough
8. `internal/llm/client.go:99` — ErrAllProvidersFailed return path
9. `internal/llm/client.go:107-110` — Override model logic
10. `internal/llm/client.go:215-222` — WarnContext logger usage
11. `internal/llm/client.go:51-54` — HTTP client timeout config
12. `internal/llm/client.go:174-178` — error path observability emit
13. `internal/llm/client.go:244-251` — nil-guard pattern
14. `internal/llm/provider.go:34-38` — Classify ModelClass priority
15. `internal/llm/provider.go:9-15` — ProviderRef type
16. `internal/llm/retry.go:14-50` — retryability classification
17. `internal/llm/retry.go:66-88` — withRetry loop
18. `internal/adapters/registry.go:75-167` — Registry implementation
19. `internal/adapters/registry.go:147-152` — Get method
20. `internal/adapters/registry.go:157-166` — List method (sorted)
21. `internal/adapters/registry.go:223-252` — wrappedAdapter.emit
22. `internal/adapters/registry.go:195-219` — Search wrapping
23. `pkg/types/capabilities.go:25-37` — Capabilities deterministic note
24. `pkg/types/capabilities.go:38-62` — Capabilities struct fields
25. `pkg/types/capabilities.go:14-23` — DocType enum
26. `pkg/types/query.go:18-35` — Query struct
27. `pkg/types/query.go:40-43` — Filter struct
28. `internal/obs/metrics/metrics.go:147-154` — cardinality allowlist
29. `internal/obs/metrics/metrics.go:86-101` — adapter metric registration
30. `internal/obs/metrics/metrics.go:112-122` — collector registration
31. `internal/obs/obs.go:51-66` — Obs bundle struct + Tracer method
32. `internal/obs/obs.go:74-142` — Init lifecycle
33. `internal/obs/reqid/reqid.go:1-30` — reqid package
34. `deploy/litellm/config.yaml:23-26` — claude-haiku-4-5 alias
35. `deploy/litellm/config.yaml:11-44` — full model_list
36. `.moai/project/product.md:36-43` — V1 source categories
37. `.moai/project/tech.md:44-50` — retrieval layer choices
38. `.moai/project/tech.md:103-119` — per-source adapter strategy table
39. `.moai/project/structure.md:14-43` — `internal/router/` reservation
40. `.moai/project/roadmap.md:34-42` — M2 SPEC-IR-001 row
41. `.moai/project/roadmap.md:147` — M2 exit criterion
42. `.moai/specs/SPEC-CORE-001/spec.md:1-15` — CORE-001 frontmatter
43. `.moai/specs/SPEC-CORE-001/spec.md:139-146` — REQ-CORE-001 NormalizedDoc
44. `.moai/specs/SPEC-LLM-001/spec.md:1-16` — LLM-001 frontmatter
45. `.moai/specs/SPEC-LLM-001/spec.md:95-105` — LLM-001 out-of-scope (tool-use deferred)
46. `.moai/specs/SPEC-OBS-001/spec.md:147-154` — cardinality allowlist
47. `internal/obs/metrics/metrics.go:147-154` — current allowlist (no `category`)

### 8.2 External (verified via Context7 / WebFetch)

1. Context7 `/assafelovic/gpt-researcher` (855 snippets, source reputation High) — planner/intent decomposition pattern
2. Context7 `/tmc/langchaingo` (988 snippets) — RouterChain destination/default pattern
3. Context7 `/itzcrazykns/perplexica` (127 snippets) — focus-mode classification
4. Context7 `/openai/openai-go` (297 snippets) — Go SDK API for LiteLLM
5. https://platform.claude.com/docs/en/docs/build-with-claude/prompt-caching — caching mechanics, 5-min TTL, 1024-token minimum, 90% read savings
6. https://platform.claude.com/docs/en/docs/build-with-claude/tool-use — tool-use forced JSON, `tool_choice: {type: tool}` API
7. https://en.wikipedia.org/wiki/Hangul_Syllables — Unicode block ranges (U+AC00-D7A3, U+1100-11FF, U+3130-318F, U+A960-A97F)
8. https://en.wikipedia.org/wiki/Korean_postpositions — common Korean particles (을/를/이/가/은/는/에서/에/와/과/의)

---

## 9. Open Questions (annotated, NOT blockers)

These are documented for the run-phase implementer; SPEC approval
does not require resolution.

1. **Should we cache the `map[string]Capabilities` snapshot taken at
   `Router.New()` for the lifetime of the process, or refresh it on
   every Classify?** Default: cache once at New (deterministic by
   contract per `pkg/types/capabilities.go:25`). Hot-reload of
   adapters is a non-goal in V1.

2. **When the LLM returns a category outside the 6-value enum, do
   we (a) reject the response and use rule-based, (b) coerce to
   `unknown`, or (c) coerce to closest match?** Default: (a) reject
   + rule-based fallback + WARN log + `outcome=error_parse` count.
   No fuzzy matching.

3. **What is the per-request prompt cache hit rate target?** No
   target in v0 — observability for cache_read_input_tokens vs
   cache_creation_input_tokens lands in a future SPEC if we add
   `Response.CacheHit bool` to `pkg/llm.Response`. v0 ignores cache
   metrics; the cost-saving is an emergent benefit.

4. **Should the `confidence` from the LLM REPLACE or BE COMBINED
   WITH the rule-based confidence when both ran?** Default:
   when LLM ran, its confidence is canonical; rule-based confidence
   is shadowed in `Metadata["rule_confidence"]` for debugging.

5. **What happens when `RouterQuery.Lang` is set explicitly AND
   Hangul detection disagrees?** Per REQ-IR-004 (Optional pattern),
   the explicit override wins. Disagreement is logged at DEBUG level
   for offline analysis.

6. **Should `unknown` Category dispatch to a default adapter set
   ({searxng, hackernews}) or return an empty AdapterSet?** **RESOLVED
   in spec.md REQ-IR-008** (no longer open): Unknown dispatches to the
   default ensemble of `{web-supporting} ∪ {social-supporting}`
   adapters after Lang compatibility filtering. The eligible DocType
   set for Unknown is the union of the web-eligible DocTypes
   (`{article, post, other}`) and the social-eligible DocTypes
   (`{post, social, video}`). Unknown is a RECOVERABLE classification,
   NOT a terminal state — being "unsure" must not break the user
   query. SPEC-FAN-001 still owns the final dispatch policy at the
   fanout layer; SPEC-IR-001 only owns the adapter-set decision.

7. **When the LLM responds in <2s but the rule-based and LLM
   disagree, do we go with LLM or rule?** Default: LLM (it had more
   information). Rule confidence is bagged in Metadata.

8. **Should the keyword tables in `rules.go` be loadable from a
   YAML file at startup for hot-tuning?** Default: NO in v0 (locked
   decision — no hot-reload). Future SPEC if drift becomes a
   measured concern.

---

## 10. Concrete Implementation Sketches (Plan-phase)

### 10.1 Package layout

```
internal/router/
├── router.go               # Replace 4-line stub with Router struct + Classify
├── router_test.go          # TestClassifyKoreanRule, ConfidenceGate, etc.
├── category.go             # Category enum, AdapterSet builder, ClassificationSource
├── category_test.go
├── query_input.go          # RouterQuery struct
├── query_input_test.go
├── routing_decision.go     # RoutingDecision struct + JSON marshal
├── routing_decision_test.go
├── korean.go               # HangulRatio, KoreanSignals, particle list
├── korean_test.go
├── rules.go                # Rules struct, keyword tables, confidence scoring
├── rules_test.go
├── llm.go                  # LLM-fallback: prompt, parse, error handling
├── llm_test.go
├── errors.go               # ErrInvalidQuery, ErrLLMTimeout, ErrAdapterRegistryEmpty
├── metrics.go              # outcome label helpers (no new collectors)
├── metrics_test.go
└── testdata/
    └── queries_golden.json # 30+ classification fixtures
```

### 10.2 Router struct sketch

```go
type Router struct {
    rules       *Rules
    llmClient   *llm.Client          // injected; nil = no LLM fallback
    registry    *adapters.Registry
    obs         *obs.Obs

    // Cached at New() — adapter capabilities by name. See §1.3.
    caps        map[string]types.Capabilities

    // Configuration
    confidenceThreshold float64       // τ_high; default 0.85
    llmModelOverride    string        // INTENT_ROUTER_LLM_MODEL env; default ""
    llmDeadline         time.Duration // default 2s
}

func New(opts Options) (*Router, error) { ... }
func (r *Router) Classify(ctx context.Context, q RouterQuery) (RoutingDecision, error)
```

### 10.3 Categories enum (sketch)

```go
type Category string
const (
    CategoryWeb      Category = "web"
    CategorySocial   Category = "social"
    CategoryAcademic Category = "academic"
    CategoryKorean   Category = "korean"
    CategoryMixed    Category = "mixed"
    CategoryUnknown  Category = "unknown"
)

type ClassificationSource string
const (
    SourceRuleBased   ClassificationSource = "rule_based"
    SourceLLMFallback ClassificationSource = "llm_fallback"
    SourceDefault     ClassificationSource = "default"
)
```

### 10.4 RoutingDecision sketch

```go
type RoutingDecision struct {
    Category    Category               `json:"category"`
    Confidence  float64                `json:"confidence"`
    AdapterSet  []string               `json:"adapter_set"`
    Lang        string                 `json:"lang"`
    Source      ClassificationSource   `json:"source"`
    Metadata    map[string]any         `json:"metadata,omitempty"`
}
```

`Metadata` keys (documented allowlist in spec.md):

| Key | Type | When set |
|---|---|---|
| `hangul_ratio` | `float64` | Always |
| `rule_triggers` | `[]string` | When SourceRuleBased; names of rule-functions that fired |
| `llm_rationale` | `string` | When SourceLLMFallback (truncated to 200 chars) |
| `llm_unavailable` | `bool` | When LLM was needed but unreachable; SourceRuleBased + flag |
| `llm_timeout` | `bool` | When LLM exceeded 2s deadline |
| `degraded_confidence` | `bool` | When confidence is from rule-based but LLM was attempted and failed |
| `lang_override` | `bool` | When RouterQuery.Lang was non-empty (REQ-IR-004) |
| `rule_confidence` | `float64` | When SourceLLMFallback (shadows LLM's confidence; debug aid) |

### 10.5 Outcome label values (cardinality-bounded)

Per the locked decision (no SPEC-OBS-001 amendment), the `outcome`
label uses the existing `outcome` allowlist. The router reuses
`obs.AdapterCalls` (NO — that's adapter-specific) — actually,
SPEC-IR-001 reuses NEITHER the LLM metric (that's per-LLM-call)
NOR the adapter metric (that's per-adapter). It must increment
EITHER:

(a) **A new metric `usearch_router_classifications_total{outcome}`**:
    creates a new metric family. SPEC-OBS-001 NFR-OBS-002 cardinality
    allowlist contains `{method, route, status_class, adapter_class,
    adapter, outcome, version, commit, go_version, provider, model}`
    — `outcome` is already allowed. A new metric using only `outcome`
    is allowed by NFR-OBS-002 because no NEW label name is introduced.
    The METRIC NAME is new; that's fine — only label *names* are
    allowlisted.

(b) Reuse `obs.AdapterCalls` with adapter name `"router"`. Hacky;
    pollutes adapter dashboards.

**Decision**: option (a). Add a new collector to `internal/obs/metrics/`:

```go
// In a new file internal/obs/metrics/router.go (or extending metrics.go):
RouterClassifications *prometheus.CounterVec   // labels: outcome
RouterClassificationDuration *prometheus.HistogramVec  // labels: outcome
```

This adds NO new label names (only `outcome` which is already
allowlisted), only new metric families. The IR's `metrics.go`
declares the constants for the outcome enumeration:

```go
const (
    OutcomeClassifiedWeb       = "classified_web"
    OutcomeClassifiedSocial    = "classified_social"
    OutcomeClassifiedAcademic  = "classified_academic"
    OutcomeClassifiedKorean    = "classified_korean"
    OutcomeClassifiedMixed     = "classified_mixed"
    OutcomeClassifiedUnknown   = "classified_unknown"
    OutcomeErrorInvalid        = "error_invalid"
    OutcomeErrorTimeout        = "error_timeout"
    OutcomeErrorBreakerOpen    = "error_breaker_open"
    OutcomeErrorParse          = "error_parse"
)
```

10 distinct values — well bounded. The collector registration
DOES live in `internal/obs/metrics/` (extends SPEC-OBS-001's
register pattern), not in `internal/router/metrics.go` — to
preserve the import-boundary invariant (`internal/router/` does
NOT import `prometheus/client_golang` directly). The `metrics.go`
file in `internal/router/` only declares OUTCOME-VALUE constants
and an emit helper that takes the obs bundle.

This is precisely the pattern SPEC-LLM-001 used at
`internal/obs/metrics/llm.go` (registered via `registerLLM(pr)`
called from `metrics.NewRegistry`). SPEC-IR-001 follows the same
shape: `registerRouter(pr)` returns the new collectors, threaded
into `Registry` via two new fields (`RouterClassifications`,
`RouterClassificationDuration`).

### 10.6 SPEC-OBS-001 minor extension required

To register the new router metric family, SPEC-IR-001's run phase
MUST add (to `internal/obs/metrics/`):

1. New file `router.go` declaring `registerRouter(pr) {calls, duration}`.
2. Two new fields on `*metrics.Registry`: `RouterClassifications`,
   `RouterClassificationDuration`.
3. Initialisation in `NewRegistry()` (line 134 area, mirror
   `registerLLM` invocation).

This is a SPEC-IR-001 owned extension to a SPEC-OBS-001 file —
permitted because:
- It does NOT modify the cardinality allowlist.
- It does NOT change existing exported types or methods.
- It's ADDITIVE in the same shape as SPEC-LLM-001's allowed
  extension (`registerLLM` at `internal/obs/metrics/metrics.go:134`).

This is mentioned in spec.md File Impact §7.

---

## 11. End of Research Artifact

This document captures the design decisions, references, and
performance budget that informed SPEC-IR-001's EARS requirements.
The companion `spec.md` file translates the conclusions into typed
acceptance criteria; `plan.md` sequences the implementation;
`acceptance.md` enumerates the Given/When/Then test scenarios.

Citation count: 47 internal file:line references + 8 external sources.

---

*Generated 2026-04-26 by manager-spec for SPEC-IR-001 v0.1*
