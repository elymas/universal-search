# SPEC-CORE-001 Research — Adapter Interface and NormalizedDoc

Created: 2026-04-26
Author: limbowl (via manager-spec)
Status: research-complete; SPEC drafted in spec.md

## 0. Research Mandate

This research artifact grounds SPEC-CORE-001 — the foundational contract that
all 12+ M3 adapter SPECs depend on. The mandate is to:

- Identify which existing patterns in the universal-search Go codebase
  (M1 foundation: SPEC-BOOT-001, SPEC-DEP-001, SPEC-OBS-001, SPEC-LLM-001) we
  must mirror so that CORE-001 feels native rather than imposed.
- Survey reference designs from the upstream OSS lineages cited in
  `.moai/project/product.md §1` (gpt-researcher, SearXNG, Perplexica,
  STORM, last30days-skill) for adapter / retriever / engine contracts.
- Resolve four type-system trade-offs that determine the long-term ergonomics
  of the package: where NormalizedDoc lives, interface vs struct for
  Capabilities, error taxonomy shape, and how observability is wrapped without
  forcing every adapter to hand-write metric calls.
- Produce a risk register and a list of explicitly-rejected alternatives so
  the run-phase implementer doesn't relitigate decisions made here.

The output of this research is the EARS spec at
`.moai/specs/SPEC-CORE-001/spec.md` — every claim made here is either
file-cited or marked as a deliberately-deferred Open Question.

## 1. The Wedge — Why CORE-001 Exists As a Standalone SPEC

### 1.1 What the roadmap originally proposed

`.moai/project/roadmap.md` lines 36-39 list **SPEC-ADP-001 (Reddit adapter,
reference)** as the first adapter SPEC, scoped as:

> SPEC-ADP-001 | Reddit adapter (reference) | public JSON API, pagination,
> normalization to `NormalizedDoc`, contract tests | expert-backend

The implication: `NormalizedDoc` is defined inside SPEC-ADP-001, mixed in
with Reddit-specific concerns. `.moai/project/structure.md` line 150 already
sketches a `NormalizedDoc` shape:

> ```
> NormalizedDoc  { id, query_id, source_type, url, title, content, authors,
>                  published_at, engagement_metrics, lang }
> ```

But the structure docs do not say *where* this type lives. The natural
reading of "structure.md §3 Bounded Contexts" (lines 127-143) places adapter
concerns under `internal/adapters/`. If `NormalizedDoc` is also there, then
every M3 adapter SPEC ends up importing `internal/adapters/reddit/` (or
similar) just to get the result type — a circular-ish dependency disaster.

### 1.2 The seven-way parallelization gate

`.moai/project/roadmap.md` line 121:

> M3 | All SPEC-ADP-* (7-way), SPEC-IDX-* (3-way) — gated on SPEC-FAN-001

For seven adapter SPECs to develop in parallel via Agent Teams or independent
human contributors, each adapter SPEC must depend ONLY on a stable contract
that does not itself change while the adapters are in flight. If
NormalizedDoc lives in SPEC-ADP-001, then:

- SPEC-ADP-001 carries an outsized scope (its own logic + the contract).
- SPEC-ADP-002..009 cannot start their TDD red phase until ADP-001's
  contract stabilises — serialising what should be parallel work.
- A breaking change to NormalizedDoc means rewriting SPEC-ADP-001's tests
  alongside seven other SPECs.

By extracting CORE-001 first, we get a clean two-layer dependency graph:

```
SPEC-CORE-001  (pkg/types/{normalized_doc,adapter,query,capabilities,errors}.go
                + internal/adapters/registry.go + internal/adapters/noop/)
       |
       +---> SPEC-ADP-001  (Reddit, reference implementation)
       +---> SPEC-ADP-002  (Hacker News)
       +---> SPEC-ADP-003  (arXiv + paper-search)
       +---> SPEC-ADP-004  (GitHub)
       +---> SPEC-ADP-005  (YouTube)
       +---> SPEC-ADP-006  (Bluesky + X)
       +---> SPEC-ADP-007  (SearXNG bridge)
       +---> SPEC-ADP-008  (Naver suite)
       +---> SPEC-ADP-009  (KoreaNewsCrawler + 다음 + Korean RSS)
       +---> SPEC-IR-001   (Intent Router consumes Capabilities for routing)
       +---> SPEC-FAN-001  (Fanout iterates Adapter contract)
       +---> SPEC-IDX-001  (Hybrid index ingests []NormalizedDoc)
```

CORE-001 is therefore inserted into M2 (between IR-001 and ADP-001), or
treated as a "M1.5" foundation extension. Either placement is consistent
with the M2 milestone owner constraints — see §6 below for placement
rationale.

### 1.3 Risk reduction from extraction

| Risk if NormalizedDoc lives in ADP-001 | Mitigation by extracting CORE-001 |
|----------------------------------------|------------------------------------|
| ADP-001 scope >2x the next-largest adapter SPEC | CORE-001 absorbs the type-design risk; ADP-001 becomes a thin wrapper |
| Cross-SPEC type drift (every adapter inventing its own result shape) | One canonical type referenced everywhere; CI guards via interface assertion in noop adapter |
| Observability inconsistency (each adapter hand-rolls its own metric label set) | Registry wraps Search() emitting `usearch_adapter_calls_total{adapter,outcome}` once; SPEC-OBS-001 cardinality allowlist already includes `adapter` and `outcome` (`internal/obs/metrics/metrics.go:150`) |
| Error taxonomy fragmentation (one adapter returns `ErrTimeout`, another returns `*RateLimitError`, fanout can't classify uniformly) | One `ErrTransient`/`ErrPermanent`/`ErrRateLimited`/`ErrSourceUnavailable` taxonomy + `CategorizeError(err) Category` classifier |

## 2. Existing Patterns in `universal-search/` to Mirror

### 2.1 The Provider/Router pattern in `internal/llm/`

The most directly analogous pattern in the existing repo is the LLM provider
router from SPEC-LLM-001. Adapter ↔ Provider, Registry ↔ Router are the
1:1 conceptual matches.

**Provider definition** — `internal/llm/provider.go:8-14`:

```
// ProviderRef identifies a specific model alias on a named provider.
type ProviderRef struct {
    // Provider is one of "anthropic", "openai", "ollama".
    Provider string
    // Model is the LiteLLM model_list alias (e.g. "claude-sonnet-4-6").
    Model string
}
```

A small immutable value type. An Adapter's analogue is its `Capabilities`
struct (see §4 below) — descriptive metadata about what the adapter can do.

**Registry / Router** — `internal/llm/router.go:148-171`:

```
type Router struct {
    priorities map[ModelClass][]ProviderRef
    breakers   map[string]*breaker // keyed by provider name
    mu         sync.RWMutex
}

func NewRouter(priorities map[ModelClass][]ProviderRef) *Router { ... }
```

The Router uses `sync.RWMutex` for concurrent access to the breaker map
(`router.go:154`), and `Route()` returns an ordered slice of available
providers (`router.go:176-198`). The Adapter registry will mirror this:
`sync.RWMutex` for concurrent Register / Get / List, with deterministic
iteration order via sorted name slice.

**Per-call observability emission** — `internal/llm/client.go:230-252`:

```
func (c *defaultClient) emitObservability(ctx context.Context, provider, model, outcome string, prompt, completion int, latencyMs int64, cost float64) {
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
    if reg != nil && reg.LLMLatency != nil {
        latencySec := float64(latencyMs) / 1000.0
        reg.LLMLatency.WithLabelValues(provider, model).Observe(latencySec)
    }
    ...
}
```

Notice three things:

1. **One shared emit function** — every LLM call goes through `emitObservability`,
   not 12 copies in 12 provider-specific files. We will mirror this with a
   `wrappedAdapter` in the registry that owns the emit (counter + histogram +
   span + slog).
2. **Counter labels are tightly bounded** — `(provider, model, outcome)`,
   matching the SPEC-OBS-001 allowlist (`metrics.go:147-154`). Adapter labels
   will be `(adapter, outcome)` — adapter name comes from the bounded V1
   registry (≤20 adapters per `metrics.go:147-154` comment), outcome from
   `{success, failure, timeout, rate_limited, unavailable}` (we extend by
   two beyond LLM's 3-value set; see §5 below).
3. **Nil-safe observability** — `if reg != nil && reg.LLMCalls != nil` guards
   are present so that tests can pass a partial Obs bundle. The wrappedAdapter
   in CORE-001 will replicate these guards.

**Sentinel error pattern** — `internal/llm/llm.go:108-121`:

```
var (
    ErrBudgetExceeded = errors.New("llm: per-request budget exceeded")
    ErrStreamBackpressureTimeout = errors.New("llm: stream consumer stalled")
    ErrAllProvidersFailed = errors.New("llm: all providers in priority list exhausted")
    ErrModelNotConfigured = errors.New("llm: model class has no configured provider")
)
```

Sentinel errors via `errors.New` with `errors.Is` matching at call sites.
Adapter taxonomy will follow this exact pattern, plus a small typed-error
struct (`*adapters.SourceError`) that wraps a Category to support both
`errors.Is` and `errors.As` ergonomics.

**Retryable vs non-retryable classification** — `internal/llm/retry.go:14-50`:

```
var nonRetryableStatusCodes = map[int]bool{
    http.StatusBadRequest:   true, // 400
    http.StatusUnauthorized: true, // 401
    http.StatusForbidden:    true, // 403
    http.StatusNotFound:     true, // 404
}

// httpStatusError carries an HTTP status code for retryability checks.
type httpStatusError struct {
    code int
    msg  string
}

func isNonRetryable(err error) bool {
    if err == nil {
        return false
    }
    var hse *httpStatusError
    if errors.As(err, &hse) {
        return nonRetryableStatusCodes[hse.code]
    }
    return false
}
```

This is the canonical brownfield pattern: typed error + classification function.
CORE-001 will export `Category` enum + `CategorizeError(err) Category`. The
fanout SPEC (FAN-001) will then call `CategorizeError` instead of
`errors.Is(err, X)` once for each sentinel.

### 2.2 The Obs bundle DI pattern

`internal/obs/obs.go:51-66`:

```
// Obs bundles the initialised observability components for use by callers.
//
// @MX:ANCHOR: [AUTO] Central obs bundle; callers: cmd mains, HTTP handlers, tests
// @MX:REASON: fan_in >= 3; single struct passed to all instrumentation call sites
type Obs struct {
    Logger *slog.Logger
    Metrics *metrics.Registry
    AdminAddr string
    tracerProvider func(name string) oteltrace.Tracer
}

func (o *Obs) Tracer(name string) oteltrace.Tracer {
    return o.tracerProvider(name)
}
```

Every existing M1 component takes `*obs.Obs` as a constructor argument
(`internal/llm/client.go:43-46`). CORE-001's registry constructor will take
the same:

```
func NewRegistry(o *obs.Obs) *Registry
```

This is non-negotiable: the wrappedAdapter needs a logger, a metrics registry,
and a tracer, and the only sanctioned source of all three is `*obs.Obs`.
SPEC-OBS-001 REQ-OBS-006 forbids importing `prometheus/client_golang` or
`go.opentelemetry.io/otel` outside `internal/obs/` — see
`.moai/specs/SPEC-OBS-001/spec.md` lines 92-93. The registry sits in
`internal/adapters/` and therefore goes through `obs.Obs`.

### 2.3 Adapter-specific metric collectors already exist

`internal/obs/metrics/metrics.go:39-41` and `:86-101`:

```
// Adapter reliability metrics.
AdapterCalls        *prometheus.CounterVec
AdapterCallDuration *prometheus.HistogramVec
...
adapterCalls := prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "usearch_adapter_calls_total",
        Help: "Total adapter calls, partitioned by adapter and outcome.",
    },
    []string{"adapter", "outcome"},
)
adapterDuration := prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
        Name:    "usearch_adapter_call_duration_seconds",
        Help:    "Adapter call latency distribution.",
        Buckets: adapterCallBuckets,
    },
    []string{"adapter"},
)
```

The metric families and labels we need are **already registered**. The
allowlist in `metrics.go:147-154` already includes `"adapter"` and
`"outcome"`. **CORE-001 ships zero new metric collectors** — it consumes
the existing AdapterCalls / AdapterCallDuration vectors. This is a major
simplification: no `internal/obs/metrics/adapter.go` file, no SPEC-OBS-001
allowlist amendment.

The `outcome` label values for adapter calls were not enumerated in
SPEC-OBS-001 — only `{success, failure, timeout}` was listed for HTTP
(`metrics.go:128-129` initialisation). CORE-001 will declare the canonical
adapter outcome enum: `{success, failure, timeout, rate_limited, unavailable}`.
This is a refinement, not a contradiction, of SPEC-OBS-001 NFR-OBS-002 —
the `outcome` label is bounded; the only change is the size of the bound
(3 → 5 values).

### 2.4 The MoAI ANCHOR/REASON pattern for high-fan-in functions

`internal/obs/obs.go:49-50`, `:71-72`:

```
// @MX:ANCHOR: [AUTO] Central obs bundle; callers: cmd mains, HTTP handlers, tests
// @MX:REASON: fan_in >= 3; single struct passed to all instrumentation call sites

// @MX:ANCHOR: [AUTO] Obs lifecycle entry point; callers: cmd/usearch, cmd/usearch-api, cmd/usearch-mcp, tests
// @MX:REASON: fan_in >= 3; wires slog+prometheus+otel in a single call
```

`internal/llm/router.go:148-150`:

```
// Router selects providers for a ModelClass and tracks per-provider circuit state.
// @MX:ANCHOR: [AUTO] Provider selection + circuit breaker; callers: client.go, router_test.go, tests
// @MX:REASON: fan_in >= 3; all retry/fallthrough logic flows through Route
```

The MX tag protocol mandates @MX:ANCHOR for fan_in ≥ 3
(`.claude/rules/moai/workflow/mx-tag-protocol.md` — "When to Add Tags"
section). The Adapter.Search method has fan_in ≥ 13 (12 adapters call it
internally, plus the registry wrapper, plus FAN-001), so the run phase
must add @MX:ANCHOR there. Similarly registry.Register/Get/List all see
fan_in ≥ 3 across run-time + tests.

### 2.5 Public package boundary discipline

`pkg/types/types.go:1-3` is currently:

```
// Package types exposes shared public types for Universal Search consumers.
package types
```

`pkg/client/client.go:1-3`:

```
// Package client is the public Go client for Universal Search.
// Full implementation lands in SPEC-CLI-001.
package client
```

The convention from SPEC-BOOT-001 was clear: empty stubs reserved for the
SPECs that will fill them. `pkg/types/` is reserved for the SDK boundary;
`structure.md:159-164` reads:

> **`pkg/types`** is the public SDK boundary. Breaking changes require major
> version bump.

CORE-001 fills `pkg/types/` with `normalized_doc.go`, `adapter.go`,
`query.go`, `capabilities.go`, `errors.go`. Any public consumer of
universal-search Go SDK (e.g., a third-party MCP integration writing its
own adapter) imports `github.com/elymas/universal-search/pkg/types` and
gets the contract. The adapter registry stays in `internal/adapters/` —
it's a runtime concern, not part of the SDK.

## 3. Reference Designs from Upstream OSS

### 3.1 gpt-researcher — Retriever protocol

`.moai/project/product.md:10` describes gpt-researcher's role as "research
orchestration". gpt-researcher's adapter pattern is informally a Python
class with two methods: `search(query)` returning a list of dicts, and an
optional `get_credentials()` for auth-bearing retrievers. Reference:
[Context7 /assafelovic/gpt-researcher, retrievers/retriever.py module pattern].

Salient lessons:

- **Loose typing is a liability**: gpt-researcher's `search()` returns
  `List[Dict[str, str]]` with informal field expectations (`href`, `body`,
  `title`). Multiple retriever bugs in the project's history stem from one
  retriever returning `url` and another returning `href`. CORE-001 will
  fix this with a strongly-typed `NormalizedDoc` struct.
- **Healthcheck is rare in the wild**: gpt-researcher does not have a
  `Healthcheck` method. Failures surface only at search time. Universal
  Search's M8 SPEC-EVAL-002 (adapter reliability dashboard) needs a
  proactive way to mark an adapter unhealthy before a query is dispatched
  to it. CORE-001 includes `Adapter.Healthcheck(ctx) error` for this
  reason, even though no V1 adapter is required to do anything beyond
  `return nil`.
- **Capabilities are implicit**: gpt-researcher infers retriever capability
  from the class name (`tavily` ⇒ web, `arxiv` ⇒ academic). This bites
  at routing time — the Intent Router has to hard-code class-to-capability
  mappings. CORE-001 makes this explicit via `Capabilities` struct returned
  from each adapter, consumed by SPEC-IR-001.

### 3.2 SearXNG — Engine module pattern

SearXNG's engine plugin pattern (Python) has a very explicit contract:
each engine module exports a `request(query, params)` function and a
`response(resp)` function that parses HTML / JSON into a list of dicts
with stable field names: `url`, `title`, `content`, `publishedDate`,
`author`, `engine`, `category`, `score`. Reference: SearXNG searx/engines/
module convention [Context7 /searxng/searxng].

Key extraction for CORE-001:

| SearXNG field | NormalizedDoc field | Notes |
|---------------|---------------------|-------|
| url | URL | identical |
| title | Title | identical |
| content | Body | renamed; "content" is overloaded with HTTP semantics |
| publishedDate | PublishedAt | `time.Time` rather than ambiguous string |
| author | Author | identical |
| engine | SourceID | "engine" implies web-search-specific; SourceID is generic |
| category | DocType | richer: article/post/paper/video/repo/issue/social |
| score | Score | identical |

The seven-decade convention of `engine` ⇒ "search engine" is too narrow for
us — Naver-news, arXiv, GitHub, and Reddit are not search engines. We use
`SourceID` (the adapter's `Name()`).

SearXNG also hard-codes a `engines.yaml` config that declares per-engine
metadata (timeout, language, category). This is where our `Capabilities`
struct's purpose comes from: each adapter declares its own metadata
programmatically rather than via config, because Go's type system gives us
a stronger guarantee than YAML.

### 3.3 Perplexica — Search agent flow

`.moai/project/product.md:84` mentions Perplexica as a closest competitor
("OSS Perplexity clone"). Perplexica wraps SearXNG via a TypeScript HTTP
client and post-processes results into a `Source` interface
[Context7 /itzcrazykns/perplexica]:

```ts
interface Source {
  pageContent: string;     // == NormalizedDoc.Body
  metadata: {
    title: string;         // == NormalizedDoc.Title
    url: string;           // == NormalizedDoc.URL
  };
}
```

Notice that Perplexica conflates "metadata" with "title and URL", which is
a downgrade — the actual rich metadata (publication date, author, score)
gets lost in the wrapping. CORE-001 separates `Metadata` (extension map)
from the canonical fields.

### 3.4 STORM — Citation provenance

`.moai/project/product.md:12` introduces STORM for "long-form knowledge
curation with per-claim provenance". STORM's data model has a
`Reference` type linking each generated claim back to a source URL +
content snippet. Reference: stanford-oval/storm `knowledge_graph.py`.

For CORE-001, this means `NormalizedDoc.Citations` is a forward-looking
field: in V1 most adapters won't populate it (a tweet has no internal
citations), but academic adapters (arXiv, Semantic Scholar, paper-search-mcp)
*can* populate it with the paper's reference list. SPEC-SYN-002 (M4)
"enforce `doc_id` trace on every synthesized claim, reject un-cited LLM
output" (`.moai/project/roadmap.md:62`) consumes this field.

### 3.5 last30days-skill — Engagement scoring

`.moai/project/product.md:11` cites mvanhorn/last30days-skill as the
"breadth / real-time signal" inspiration. Its data model assigns a
**single normalized engagement score** to every result, regardless of
source — Reddit upvotes, X likes, HN points, YouTube view counts all
collapse to a `score: float` in [0.0, 1.0].

CORE-001's `Score` field follows this pattern: a single normalized float
per doc, computed by the adapter, consumed by ranking. Per-source raw
metrics (upvote count, like count, view count) go into `Metadata` as
extension data — they're available for debugging or specialized re-ranking
but they're NOT in the canonical surface.

### 3.6 Stanford STORM, last30days-skill, gpt-researcher convergence

The cross-cutting observation: every reference implementation we cite has
**some normalization layer** that converts source-specific results into a
unified shape. The shape varies in detail but converges on:
`{id, source, url, title, content, published_at, score}` plus an
extension bag. CORE-001 is, in this sense, well-trodden. The novel work
is the strong typing, the error taxonomy, and the registry-level
observability wrapping — not the field set.

## 4. Type-System Trade-offs

### 4.1 Where does NormalizedDoc live?

**Decision: `pkg/types/normalized_doc.go`**.

Three-way option analysis:

| Option | Pros | Cons | Verdict |
|--------|------|------|---------|
| `pkg/types/` (chosen) | Public SDK boundary; external Go consumers can build adapters; `structure.md §5` (lines 159-164) already designates this path | Public API stability commitment; breaking change costs a major version | Accepted — V1 SDK boundary already exists |
| `internal/adapters/` | Closer to adapter implementations; no SDK commitment | Every M3 adapter SPEC ends up exposing a leaky abstraction; future MCP plugins (Skill marketplace) cannot import internal/ paths | Rejected — re-creates the original problem the SPEC solves |
| `internal/types/` | Internal but shared | Same MCP / external-Go-SDK problem as above; also fragments "types" between internal and pkg | Rejected — adds an unnecessary boundary |

`pkg/types` already exists as an empty stub from SPEC-BOOT-001 and is
mentioned in `pkg/client/client.go:3` as the SDK companion. The decision
matches the existing project intent.

### 4.2 Adapter — interface vs struct?

**Decision: interface.**

```go
type Adapter interface {
    Name() string
    Search(ctx context.Context, q Query) ([]NormalizedDoc, error)
    Healthcheck(ctx context.Context) error
    Capabilities() Capabilities
}
```

- Interface lets each adapter be a struct with private fields (HTTP client,
  rate limiter, API key) without exposing them.
- Interface enables the registry's `wrappedAdapter` to compose: `Wrap(a Adapter)
  Adapter` returns an Adapter that delegates to `a` while emitting metrics.
  A struct cannot be wrapped this way without exposing fields.
- Interface satisfies the "duck typing for tests" requirement: SPEC-FAN-001
  tests need to mock the adapter, and a stub mock just needs to implement
  the four methods.

The closest analogue in the existing codebase is `internal/llm/llm.go:100-105`:

```
type Client interface {
    Complete(ctx context.Context, req Request) (Response, error)
    Stream(ctx context.Context, req Request) (<-chan Delta, error)
    Embed(ctx context.Context, req EmbedRequest) (EmbedResponse, error)
    Close() error
}
```

Same shape — methods only, no fields. CORE-001's Adapter is a 4-method
peer.

### 4.3 Capabilities — struct vs interface?

**Decision: struct.**

```go
type Capabilities struct {
    SourceID         string
    DisplayName      string
    DocTypes         []DocType
    SupportedLangs   []string
    SupportsSince    bool
    RequiresAuth     bool
    AuthEnvVars      []string
    RateLimitPerMin  int
    DefaultMaxResults int
    Notes            string
}
```

- Capabilities is descriptive metadata, not behavior. Interfaces describe
  behavior; structs describe data.
- The Intent Router (SPEC-IR-001) needs to *read* Capabilities at startup
  to build a routing table. Structs are trivially marshalable; interfaces
  are not.
- Future capability extensions (e.g., `SupportsCursor bool`) are field
  additions, which are non-breaking. An interface addition would be
  breaking for all 12 adapters.

### 4.4 Error taxonomy — sentinels vs typed errors vs both?

**Decision: both — sentinel for `errors.Is`, struct for `errors.As`.**

```go
// Sentinel category errors (errors.Is targets).
var (
    ErrTransient         = errors.New("adapter: transient failure")
    ErrPermanent         = errors.New("adapter: permanent failure")
    ErrRateLimited       = errors.New("adapter: rate limited")
    ErrSourceUnavailable = errors.New("adapter: source unavailable")
)

// SourceError carries adapter-specific context.
type SourceError struct {
    Adapter   string
    Category  Category   // Transient | Permanent | RateLimited | Unavailable
    HTTPStatus int       // 0 if not HTTP-specific
    Cause     error      // original error
    RetryAfter time.Duration // 0 if not specified by source
}

func (e *SourceError) Error() string { ... }
func (e *SourceError) Unwrap() error { return e.Cause }
func (e *SourceError) Is(target error) bool { /* match against sentinels by Category */ }
```

This dual pattern is well-established in stdlib. Examples:
- `os.ErrNotExist` (sentinel) + `*os.PathError` (struct) — both work with
  the same underlying error.
- `net.ErrClosed` (sentinel) + `*net.OpError` (struct).
- `context.DeadlineExceeded` (sentinel) + `*context.DeadlineExceededError`
  (struct, post-Go 1.21).

The benefit:
- Fanout (SPEC-FAN-001) calls `errors.Is(err, ErrTransient)` for
  retry/fanout decisions — terse and obvious.
- Adapter implementations can use `*SourceError` to carry rich context
  (HTTP status code, RetryAfter) without callers having to know the concrete
  type.
- `CategorizeError(err) Category` is a one-liner: `var se *SourceError;
  if errors.As(err, &se) { return se.Category }`.

### 4.5 Observability wrapping — per-adapter vs registry-wrapper?

**Decision: registry-wrapper.**

The risk to avoid: 12 adapters each writing 30 lines of boilerplate to
emit `usearch_adapter_calls_total` + `usearch_adapter_call_duration_seconds`
+ OTel span. That's 360 lines of duplication, with the inevitable result
that one adapter forgets the histogram, another inverts the success/failure
label, and a third uses a non-allowlisted label value.

Pattern:

```go
type wrappedAdapter struct {
    inner Adapter
    obs   *obs.Obs
}

func (w *wrappedAdapter) Search(ctx context.Context, q Query) ([]NormalizedDoc, error) {
    name := w.inner.Name()
    tracer := w.obs.Tracer("adapter")
    ctx, span := tracer.Start(ctx, "adapter.search",
        oteltrace.WithAttributes(attribute.String("adapter", name)))
    defer span.End()

    start := time.Now()
    docs, err := w.inner.Search(ctx, q)
    elapsed := time.Since(start).Seconds()

    outcome := classifyOutcome(err) // success | failure | timeout | rate_limited | unavailable

    if reg := w.obs.Metrics; reg != nil {
        if reg.AdapterCalls != nil {
            reg.AdapterCalls.WithLabelValues(name, outcome).Inc()
        }
        if reg.AdapterCallDuration != nil {
            reg.AdapterCallDuration.WithLabelValues(name).Observe(elapsed)
        }
    }
    if w.obs.Logger != nil {
        w.obs.Logger.InfoContext(ctx, "adapter call",
            slog.String("adapter", name),
            slog.String("outcome", outcome),
            slog.Float64("elapsed_seconds", elapsed),
            slog.Int("result_count", len(docs)))
    }

    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, outcome)
    }
    return docs, err
}
```

`classifyOutcome` uses `errors.Is` against the sentinel taxonomy (§4.4) +
`errors.Is(err, context.DeadlineExceeded)` for timeout detection.

Pros:
- 30 lines of boilerplate, written once.
- Adapters can't forget to instrument — they don't write the instrumentation.
- Adapters can't pick wrong label values — the wrapper computes them.
- SPEC-OBS-001 cardinality guard
  (`internal/obs/metrics/metrics_test.go::TestNoUnboundedLabels`)
  remains green, because the only `internal/adapters/...` import of
  `prometheus/client_golang` is via `obs.Metrics` (transitive, allowed)
  and the wrapper only emits labels already present in the allowlist
  at `internal/obs/metrics/metrics.go:147-154`.

Cons:
- Adapters can no longer *add* extra instrumentation (e.g., a per-source
  cache-hit ratio gauge). This is fine for V1 — adapter-specific metrics
  belong to a per-adapter SPEC, not the registry.

This is the same pattern as the openai-go middleware in
`internal/llm/cost.go` (cited in `internal/llm/client.go:138-147`) — wrap
the inner thing, instrument at the wrapper.

## 5. Outcome Label Set

`internal/obs/metrics/metrics.go:150` already includes `outcome` in the
allowlist. The HTTP middleware (`metrics.go:240-255`) uses `{2xx, 3xx, 4xx,
5xx}` as `status_class`. The LLM client uses `{success, failure, timeout}`
(`internal/llm/client.go:199`).

For adapters we need finer granularity, because the fanout (FAN-001) routes
*decisions* (retry vs skip vs back off) on outcome. Proposal:

| Outcome | Trigger | Fanout reaction |
|---------|---------|-----------------|
| `success` | err == nil | Use docs |
| `failure` | err != nil, `errors.Is(err, ErrPermanent)` | Skip; log; do not retry |
| `timeout` | err != nil, `errors.Is(err, context.DeadlineExceeded)` | Retry once; if still times out, count as `failure` |
| `rate_limited` | err != nil, `errors.Is(err, ErrRateLimited)` | Honor RetryAfter; back off; retry once |
| `unavailable` | err != nil, `errors.Is(err, ErrSourceUnavailable)` | Skip; do not retry; mark adapter unhealthy in registry |
| `transient` | err != nil, `errors.Is(err, ErrTransient)` | Retry up to N (per FAN-001 policy) |

Five distinct values. Bounded. The registry validates that any custom
outcome string passed to `WithLabelValues` is in this set (panic at
test-time, not runtime — same pattern as
`internal/obs/metrics/metrics.go::TestNoUnboundedLabels`).

This is a refinement of SPEC-OBS-001 NFR-OBS-002 ("outcome ∈ {success,
failure, timeout}"); SPEC-OBS-001 only enumerated outcomes for HTTP, and
explicitly noted in `metrics.go:148` that "adapter, outcome" is in the
allowlist without enumerating values. SPEC-CORE-001 enumerates them, with
`transient` as a sixth value the wrapper may emit when classification cannot
be more precise.

## 6. Milestone Placement

The roadmap originally placed `NormalizedDoc` inside SPEC-ADP-001 (M2).
SPEC-CORE-001 inserts itself BEFORE SPEC-ADP-001, conceptually as "M2 —
Foundation contracts (inserted)" — the contract layer required before the
first adapter can be implemented.

Three options for sequencing:

1. **CORE-001 in M2, before ADP-001**: Keeps M1 foundation milestones intact
   (BOOT, DEP, OBS, LLM all complete). CORE-001 becomes the first M2 SPEC,
   alongside SPEC-IR-001 (Intent Router). ADP-001 picks up the contract.
   **This is the recommended placement.**

2. **CORE-001 as M1.5**: Pull CORE-001 back into M1. Awkward — M1 is owned
   by infrastructure (BOOT, DEP) and observability (OBS, LLM) themes;
   CORE-001 is a contract.

3. **Embed CORE-001 inside SPEC-ADP-001's scope**: The original roadmap.
   Rejected per §1.

Option 1 wins. The SPEC frontmatter sets:

```
milestone: M2 — Foundation contracts (inserted)
depends_on: [SPEC-BOOT-001]
blocks: [SPEC-IR-001, SPEC-ADP-001, SPEC-ADP-002, SPEC-ADP-003, SPEC-ADP-004,
         SPEC-ADP-005, SPEC-ADP-006, SPEC-ADP-007, SPEC-ADP-008, SPEC-ADP-009,
         SPEC-FAN-001, SPEC-IDX-001]
```

Rationale for `depends_on: [SPEC-BOOT-001]` only (not OBS-001 / LLM-001):

- CORE-001 imports `internal/obs` (REQ-OBS-006 public API), which is owned
  by SPEC-OBS-001 — but the registry only USES the obs bundle, it doesn't
  *fail* if obs is partially initialised (nil-safe checks in §4.5). Hard
  dependency on BOOT-001 is sufficient because OBS-001's package surface is
  already stable.
- CORE-001 doesn't import `internal/llm`. No dependency.

## 7. Risk Register and Rejected Alternatives

### 7.1 Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| NormalizedDoc field set drift across M3 adapters (adapter A wants `Likes int`, adapter B wants `Karma int`) | High | High (forces public-API breaking change) | Lock canonical set in V1; per-adapter extras go in `Metadata map[string]any` |
| `Metadata map[string]any` becomes a free-for-all, defeating typed-contract benefit | Medium | Medium | Document allowed key conventions in adapter-specific SPEC docs; add lint test in M8 if drift is observed |
| Capabilities struct grows monotonically as new fields are added per adapter | Medium | Low | Field additions are non-breaking; document the policy in pkg/types/capabilities.go godoc |
| Registry's wrappedAdapter swallows or re-classifies adapter-returned errors incorrectly | Medium | High (fanout makes wrong retry decisions) | Wrapper preserves underlying err via `%w`; re-classification is `errors.Is`-based, no string parsing |
| `NormalizedDoc.Hash` collisions across adapters (Reddit's permalink and HN's storyURL hashing to the same value) | Low | High (dedup falsely merges distinct docs) | Hash includes `SourceID` prefix in canonicalisation; documented in pkg/types/normalized_doc.go |
| Registry locking contention if adapter list grows past 50 (post-V1) | Low | Medium | RWMutex; reads dominate; benchmark in CI if M9 adapter count exceeds 30 |
| Five-outcome label set underspecified (some err categories don't fit) | Low | Low | `transient` is the catch-all for "I don't know but you can retry"; add NFR-CORE-002 cardinality test extending OBS-001's allowlist test |
| Adapter authors bypass the registry and call SearchPipe directly, missing observability | Medium | Medium | Document that adapters MUST be registered to be production-eligible; FAN-001 only iterates registered adapters |

### 7.2 Rejected Alternatives

| Alternative | Why Rejected |
|-------------|--------------|
| Generic `Adapter[T any]` with type parameter for result type | Go generics don't compose well with `interface` polymorphism the registry needs; complicates the `[]Adapter` slice in registry; defers a real benefit until V2 federated-search use cases that aren't in roadmap |
| Use `*pb.NormalizedDoc` from a protobuf schema | Would force a proto compile step into M2; `structure.md:159-164` doesn't promise gRPC stability for `pkg/types`; Python services have their own Pydantic schemas via gRPC contracts under `proto/` (separate concern) |
| Channel-based `Search(ctx, q) <-chan NormalizedDoc` | Streaming results is appealing but most upstream adapters return a paginated batch; channel API forces every adapter into a goroutine model that has no analog in the underlying APIs (Reddit JSON, arXiv OAI-PMH); revisit if SPEC-SYN-004 (M4 streaming synthesis) needs streaming adapters specifically |
| Place registry in `pkg/registry/` | `pkg/` is for SDK consumers; the registry is a runtime singleton used by usearch-api; consumers building their own Adapter implementations don't need our registry, they need our types |
| Skip Healthcheck() in V1 ("just call Search with a sentinel query") | Sentinel queries cost API quota; explicit Healthcheck is cheap; SPEC-EVAL-002 (M8) consumes this; the noop `return nil` default is a 1-line cost |
| Combine Adapter + Capabilities into one interface (e.g., return Capabilities from Search() too) | Capabilities don't change per-query; they're adapter-static; calling Capabilities() once per startup is the right cardinality |
| Make Score nullable (`*float64`) to distinguish "no score" from "zero score" | Adds pointer-juggling for ~zero benefit; convention: 0.0 means "not scored"; documented in godoc; ranking layer treats 0.0 as "use as last-rank tiebreaker" |
| Errors are `string` (Python-style) for portability | Loses `errors.Is`/`errors.As`; Go convention; portability is preserved by the gRPC contract layer if/when needed |

## 8. Reference Implementations Cited

The following sources informed the design. URLs are canonical; Context7
library IDs are listed for in-context retrieval; full citations land in
spec.md §9 References.

| Reference | Library ID / URL | Used For |
|-----------|------------------|----------|
| gpt-researcher retriever module | Context7 `/assafelovic/gpt-researcher` | Adapter contract pattern, Healthcheck absence rationale |
| SearXNG engine plugin pattern | Context7 `/searxng/searxng` | Field set convergence (url/title/content/score), Capabilities-as-config rationale |
| Perplexica Source interface | Context7 `/itzcrazykns/perplexica` | Metadata/extension separation rationale |
| Stanford STORM knowledge_graph | https://github.com/stanford-oval/storm | Citations field forward-compat |
| last30days-skill scoring | https://github.com/mvanhorn/last30days-skill | Single normalized Score field convention |
| Go stdlib error patterns | https://pkg.go.dev/errors | Sentinel + struct dual pattern (os.ErrNotExist + *os.PathError) |
| prometheus/client_golang CounterVec | Context7 — already pinned at `internal/obs/metrics/metrics.go` v1.23.2 | Label allowlist conformance |
| Internal: SPEC-LLM-001 router | `internal/llm/router.go:148-198` | Registry pattern (RWMutex, Route returning ordered slice) |
| Internal: SPEC-LLM-001 client emit | `internal/llm/client.go:230-252` | Per-call observability emission shape |
| Internal: SPEC-OBS-001 obs bundle | `internal/obs/obs.go:51-66` | Constructor DI pattern |
| Internal: SPEC-OBS-001 metric registry | `internal/obs/metrics/metrics.go:86-101,151` | Reuse of AdapterCalls / AdapterCallDuration; allowlist conformance |
| Internal: SPEC-LLM-001 retry/classification | `internal/llm/retry.go:14-50` | Typed-error classification pattern |

## 9. Open Questions Carried Into spec.md

The following are deliberately unresolved at research time and recorded
in spec.md §11 Open Questions for run-phase / post-V1 decision.

1. **Should NormalizedDoc.PublishedAt be required or optional (zero value
   permitted)?** Some adapters can't extract a publication date (e.g.,
   Naver shopping listings). Defaulting to `time.Time{}` zero value lets
   the field be optional without using a pointer. Accepted as default;
   `Validate()` does not require non-zero PublishedAt.

2. **Hash algorithm: SHA-256 (collision-safe but 32 bytes) vs xxhash64
   (8 bytes, much faster, industry-standard for non-cryptographic dedup).**
   Default xxhash64 (existing project dep via prometheus client_golang's
   transitive); revisit if dedup precision becomes a concern.

3. **Should the registry expose a `Healthcheck-all` operation that probes
   every registered adapter in parallel?** Useful for SPEC-EVAL-002 dashboard.
   Default deferred — EVAL-002 can iterate registry.List() and call
   `Healthcheck` itself; registry stays minimal.

4. **Should `wrappedAdapter` enforce a default per-call timeout if the
   caller's context has none?** Risk of a runaway adapter blocking forever.
   Default no — adapters get the caller's context as-is; FAN-001 owns
   per-adapter timeout policy. Revisit if observed in M3 testing.

5. **Should `Capabilities.RequiresAuth` validation block registration when
   the named env var is unset, or only warn?** Block: no production query
   ever silently no-ops. Warn: developer ergonomics in early dev. Default
   block; provide `Register(adapter, RegisterOptions{SkipAuthCheck: true})`
   escape hatch for tests.

6. **Should `errors.Is(err, ErrSourceUnavailable)` automatically mark the
   adapter unhealthy in the registry (auto-disable until next Healthcheck
   passes)?** Powerful but risky — a transient outage during a single
   query could disable the adapter for the rest of the process lifetime.
   Default: registry tracks state but doesn't auto-disable; FAN-001 owns
   the unhealthy/healthy transition policy.

7. **Cursor pagination shape: opaque string vs typed struct?** Opaque
   string (`type Cursor string`) lets each adapter encode its own cursor
   format without leaking shape; typed struct forces a uniform pagination
   model that doesn't match all sources (HN's `numericFilters` cursor is
   different from Reddit's `after`). Default opaque string;
   adapter-specific format documented per-adapter.

## 10. Summary

CORE-001 extracts a foundational contract from what would otherwise be
SPEC-ADP-001's overflowing scope. The contract is small (5 type files +
registry + noop adapter, ~400 LoC implementation surface), well-grounded
in existing M1 patterns (`internal/llm/router.go`, `internal/obs/obs.go`),
reuses already-registered Prometheus collectors (`AdapterCalls`,
`AdapterCallDuration`) so it ships zero new metric families, and unblocks
true 12-way parallel development of M3 adapters.

The package-and-file plan is:

```
pkg/types/
├── normalized_doc.go    # NormalizedDoc + Validate()
├── adapter.go           # Adapter interface + Query
├── query.go             # Query type
├── capabilities.go      # Capabilities + DocType + Filter types
└── errors.go            # Sentinels + SourceError + Category + CategorizeError

internal/adapters/
├── registry.go          # Registry + Register/Get/List + wrappedAdapter
├── registry_test.go
└── noop/
    └── noop.go          # Reference Adapter implementation (compile-time check)
```

Token budget for run-phase implementation: ~6KLoC including tests
(estimated). Within the standard harness ceiling.

This research artifact is complete. Proceed to spec.md.

— end of research.md
