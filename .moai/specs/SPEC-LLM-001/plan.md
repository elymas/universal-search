# SPEC-LLM-001 Plan — Post-Hoc Implementation Summary

Created: 2026-04-25
Updated: 2026-04-26 (post-implementation reconciliation)
Author: limbowl (via manager-spec)
Status: implemented
Methodology: TDD (RED-GREEN-REFACTOR)
Coverage Target: 85%

## 0. Plan Scope

Reverse-engineered description of how SPEC-LLM-001 was implemented as
the final M1 SPEC. The LLM client was delivered alongside its LiteLLM
proxy config and observability hooks, landing on 2026-04-26 (commit
5005eb0). Read alongside spec.md (requirements) and acceptance.md
(Given/When/Then scenarios).

## 1. Approach Summary

A provider-agnostic `internal/llm.Client` interface (`Complete`,
`Stream`, `Embed`) wraps `github.com/openai/openai-go` and targets the
LiteLLM proxy at `cfg.BaseURL+"/v1"`. The implementation layers
priority-list routing with per-provider circuit breakers, retry with
exponential backoff (250/500/1000 ms with ±10% jitter), provider
fallthrough on retry exhaustion, post-flight budget cap
(`ErrBudgetExceeded`), and uniform per-call observability (1 slog INFO
+ 1 counter + 1 histogram + 1 OTel span + 1 cost counter increment)
via SPEC-OBS-001's public API. Three new collectors (`LLMCalls`,
`LLMCost`, `LLMLatency`) were registered in
`internal/obs/metrics/llm.go` and re-exported via `obs.Obs`. The
LiteLLM proxy config was committed at `deploy/litellm/config.yaml`
declaring claude-opus-4-7 / claude-sonnet-4-6 / claude-haiku-4-5
(Anthropic), gpt-4o / gpt-4o-mini (OpenAI), ollama/llama3.1 (Ollama),
text-embedding-3-large (embeddings) with priority-based-routing +
per-model-group fallback chains.

## 2. Reference Implementations (consumed)

| Concern | Reference (file:line) | Pattern reused |
|---------|-----------------------|----------------|
| Sentinel error pattern | (already used by future SPEC-CORE-001) | `var Err... = errors.New(...)` |
| Obs bundle DI | `internal/obs/obs.go::Init` (SPEC-OBS-001) | Constructor takes `*obs.Obs`; nil-safe access patterns |
| Named collector emission | `internal/obs/metrics/metrics.go` (SPEC-OBS-001) | `WithLabelValues(...).Inc()` / `.Observe(...)` |
| Request-ID propagation | `internal/obs/reqid/reqid.go::NewTransport` (SPEC-OBS-001) | Wrapped `http.RoundTripper` adds `X-Request-ID` on outbound |
| OTel span shape | `internal/obs/trace/trace.go::Tracer` (SPEC-OBS-001) | `tracer.Start(ctx, "llm.call")` + `span.SetAttributes(...)` + `span.End()` |
| Cardinality allowlist | `internal/obs/metrics/metrics.go` (SPEC-OBS-001) | Extended allowlist with `provider`, `model` (both bounded) |

## 3. Package Layout (as implemented)

```
internal/llm/
├── llm.go                   # Package doc + Client interface + types + errors
├── llm_test.go              # API surface + import-boundary tests
├── client.go                # defaultClient + Complete + Stream + Embed
├── client_test.go           # REQ-LLM-002/003/005/006
├── provider.go              # ModelClass enum + defaultPriorities + ProviderRef
├── provider_test.go
├── router.go                # Router + per-provider circuit breaker
├── router_test.go           # REQ-LLM-004 + NFR-LLM-002
├── retry.go                 # Backoff + retryable classification
├── retry_test.go            # backoff timing tests
├── cost.go                  # x-litellm-response-cost parser + budget cap
├── cost_test.go             # REQ-LLM-006 + NFR-LLM-003
├── stream.go                # Channel-based Delta iterator + backpressure
├── stream_test.go           # REQ-LLM-008
├── config/
│   ├── config.go            # Config struct + env loader (koanf)
│   └── config_test.go       # env binding + validation + defaults
└── bench/
    └── (NFR-LLM-001 benchmark — to be added in a scheduled-bench follow-up)

internal/obs/metrics/
└── llm.go (logical) — LLMCalls, LLMCost, LLMLatency collectors
    registered via `registerLLM(r)` from `NewRegistry`.

deploy/litellm/
└── config.yaml              # model_list + router_settings + general_settings
```

The `deploy/docker-compose.yml` litellm service entry was extended
with:
```yaml
volumes:
  - ./litellm/config.yaml:/app/config.yaml:ro
command: ["--config", "/app/config.yaml"]
```

`.env.example` (root) was appended with 5 new vars:
`LITELLM_MASTER_KEY`, `LITELLM_BUDGET_USD`, `ANTHROPIC_API_KEY`,
`OPENAI_API_KEY`, `OLLAMA_BASE_URL`.

`go.mod` gained one new direct dependency:
`github.com/openai/openai-go v1.x` (exact patch captured in
`docs/dependencies.md`).

## 4. Key Implementation Files (file:line refs)

### Public surface
- `internal/llm/llm.go:16-130` — Package doc; `ModelClass` enum;
  `Message`, `Request`, `Response`, `Delta`, `EmbedRequest`,
  `EmbedResponse` value types (no openai-go types leaked); `Client`
  interface; sentinel errors (`ErrBudgetExceeded`,
  `ErrStreamBackpressureTimeout`, `ErrAllProvidersFailed`,
  `ErrModelNotConfigured`); `New(cfg, obs) (Client, error)`
  constructor.

### Default client implementation
- `internal/llm/client.go:35-65` — `defaultClient` struct + constructor.
  Wraps `openai.NewClient(WithBaseURL, WithAPIKey, WithHTTPClient)` and
  installs `reqid.NewTransport(http.DefaultTransport)` for request-ID
  propagation.
- `internal/llm/client.go::Complete` (REQ-LLM-002/003/004/005/006,
  NFR-LLM-003) — loops over `router.Route(ctx, class)`, calls
  `completeWithProvider`, classifies error via `isNonRetryable`,
  records breaker outcome, returns on first success.
- `internal/llm/client.go:230-252` (approximate range, observed in
  `emit` helper) — per-call observability: 1 slog INFO + 1 counter +
  1 histogram + 1 OTel span. Nil-safe guards
  (`if reg != nil && reg.X != nil { ... }`) protect against test-mode
  zero-value `Obs` bundles. This block is the reference pattern that
  SPEC-CORE-001 mirrors for adapter observability.

### Router and circuit breaker
- `internal/llm/router.go:1-60` — `BreakerState` enum (Closed / Open /
  HalfOpen); `observation` struct; `breaker` with `sync.Mutex`,
  rolling 60 s window, configurable threshold (default 0.50) and
  minimum samples (default 10).
- `internal/llm/router.go::breaker.Allow` — state machine: Closed →
  always allow; Open → time-since-open ≥ 30 s admits one probe
  (transitions to HalfOpen); HalfOpen → reject (probe already in
  flight).
- `internal/llm/router.go::breaker.Record(success bool)` — appends
  observation; trims to window; recomputes state per threshold.
- `internal/llm/router.go::Router` — holds `priorities
  map[ModelClass][]ProviderRef` + `breakers map[string]*breaker`.
- `internal/llm/router.go::Router.Route(ctx, class)` returns providers
  in priority order, skipping those whose breaker is Open.

### Retry policy
- `internal/llm/retry.go::isNonRetryable(err) bool` — true for HTTP
  400/401/403/404; false for 408/429/500/502/503/504 and network
  errors.
- `internal/llm/retry.go` — backoff sequence 250/500/1000 ms with
  ±10% jitter; ctx-respecting (returns `ctx.Err()` if cancelled
  during backoff wait).

### Cost extraction and budget cap
- `internal/llm/cost.go` — openai-go middleware that captures
  `resp.Header.Get("x-litellm-response-cost")`, parses to float64 via
  `strconv.ParseFloat`, stashes in context via package-private key;
  `Client.Complete` reads via `getCost(ctx)`, sets `Response.CostUSD`,
  increments `LLMCost` counter, applies budget cap.
- NFR-LLM-003: `if costUSD > cfg.PerRequestCapUSD` → return Response
  AND `ErrBudgetExceeded` (Response is NOT discarded; cost counter is
  still incremented; WARN slog record emitted).

### Streaming
- `internal/llm/stream.go` — channel-based `<-chan Delta` (buffered
  capacity 16); 30 s backpressure timeout closes channel + cancels
  upstream; error surfaced as final Delta with `Err != nil`.

### LiteLLM proxy config
- `deploy/litellm/config.yaml` — committed; declares the seven model
  aliases + 3 fallback chains (claude-opus-4-7 → gpt-4o;
  claude-sonnet-4-6 → gpt-4o-mini, ollama/llama3.1;
  claude-haiku-4-5 → gpt-4o-mini, ollama/llama3.1:8b);
  `routing_strategy: priority-based-routing`;
  `general_settings.master_key: os.environ/LITELLM_MASTER_KEY`;
  `general_settings.store_prompts_in_spend_logs: false`.

## 5. Integration Points

| Upstream SPEC | Consumed via |
|---------------|--------------|
| SPEC-BOOT-001 | `deploy/docker-compose.yml` litellm service (extended with config-mount + command); `internal/llm/llm.go` stub (replaced with full package); `cmd/usearch*/main.go` (conditional LLM init) |
| SPEC-OBS-001 | `obs.Logger(ctx)` for slog INFO; `obs.Tracer("usearch.llm")` for `llm.call` span; `obs.Obs.Metrics.LLMCalls/LLMCost/LLMLatency` collectors; `reqid.NewTransport` for X-Request-ID propagation; cardinality allowlist extended with `provider` + `model` |
| SPEC-DEP-001 | `docs/dependencies.md` regenerated with openai-go pin; audit CI runs on the expanded `go.mod` |

| Downstream SPEC | Provides |
|-----------------|----------|
| SPEC-CORE-001 | Reference pattern for per-call observability emit (`internal/llm/client.go:230-252`); reference pattern for sentinel errors + sub-typed errors |
| SPEC-SYN-001 | `llm.Client.Complete` consumed for citation assembly + summary generation |
| SPEC-DEEP-001/002/003/004 | All `/deep` agents call `Client.Complete` with different `ModelClass` routing |
| SPEC-IR-001 | Router's LLM-fallback path calls `Client.Complete` with `Classify` class |
| SPEC-SYN-004 | Streaming synthesis builds on `Client.Stream` |

## 6. Data Structures and Interfaces

### Public types (`internal/llm/llm.go`)
```go
type ModelClass string

const (
    DeepResearch ModelClass = "deep_research"  // claude-opus-4-7 → gpt-4o
    Summary      ModelClass = "summary"        // claude-sonnet-4-6 → gpt-4o-mini → ollama/llama3.1
    Classify     ModelClass = "classify"       // claude-haiku-4-5 → gpt-4o-mini → ollama/llama3.1:8b
    Embed        ModelClass = "embed"          // text-embedding-3-large
)

type Message struct {
    Role    string  // "user" | "assistant" | "system"
    Content string
}

type Request struct {
    Class       ModelClass
    System      string
    Messages    []Message
    MaxTokens   int
    Temperature float64
    Override    string  // bypass class routing; route to specific model alias
}

type Response struct {
    Text             string
    Provider         string
    Model            string
    PromptTokens     int
    CompletionTokens int
    LatencyMs        int64
    CostUSD          float64
    FinishReason     string
}

type Delta struct {
    Content      string
    FinishReason string
    Err          error  // populated on final delta if stream ended in error
}

type EmbedRequest struct {
    Class ModelClass  // must be Embed
    Input []string
}

type EmbedResponse struct {
    Vectors      [][]float32
    Provider     string
    Model        string
    PromptTokens int
    LatencyMs    int64
    CostUSD      float64
}

type Client interface {
    Complete(ctx context.Context, req Request) (Response, error)
    Stream(ctx context.Context, req Request) (<-chan Delta, error)
    Embed(ctx context.Context, req EmbedRequest) (EmbedResponse, error)
    Close() error
}

var (
    ErrBudgetExceeded            = errors.New("llm: per-request budget exceeded")
    ErrStreamBackpressureTimeout = errors.New("llm: stream consumer stalled")
    ErrAllProvidersFailed        = errors.New("llm: all providers in priority list exhausted")
    ErrModelNotConfigured        = errors.New("llm: model class has no configured provider")
)

func New(cfg config.Config, o *obs.Obs) (Client, error)
```

### Config (`internal/llm/config/config.go`)
```go
type Config struct {
    BaseURL          string         // LITELLM_BASE_URL (default http://localhost:4000)
    MasterKey        string         // LITELLM_MASTER_KEY (required for production)
    PerRequestCapUSD float64        // LITELLM_BUDGET_USD (default 0.50)
    TimeoutSeconds   int            // LLM_TIMEOUT_SECONDS (default 60)
    // ... ModelClass aliases per env binding
}

func Load() (Config, error)  // koanf-based env loader with validation
```

### Router (`internal/llm/router.go`)
```go
type ProviderRef struct {
    Provider string  // "anthropic" | "openai" | "ollama"
    Model    string  // LiteLLM alias
}

type Router struct {
    priorities map[ModelClass][]ProviderRef
    breakers   map[string]*breaker
    mu         sync.RWMutex
}

func NewRouter(priorities map[ModelClass][]ProviderRef) *Router
func (r *Router) Route(ctx context.Context, class ModelClass) ([]ProviderRef, error)
func (r *Router) Record(provider string, success bool)
```

## 7. Test Coverage Notes

Test inventory (46 representative tests per spec.md §8):
- `client_test.go` — REQ-LLM-002 (API surface), REQ-LLM-003
  (observability per call), REQ-LLM-005 (auth + no-leak),
  REQ-LLM-006 (cost extraction).
- `router_test.go` — REQ-LLM-004 (retry + fallthrough), NFR-LLM-002
  (circuit breaker: opens at 50% / 10-sample, half-opens after 30 s,
  closes on probe success / reopens on probe failure).
- `retry_test.go` — Backoff timings within ±25% tolerance.
- `cost_test.go` — Cost tracker per-provider sums; exemplar with
  request_id; missing/malformed header handling; NFR-LLM-003 budget
  cap including the HARD rule that Response is returned alongside
  `ErrBudgetExceeded`.
- `stream_test.go` — REQ-LLM-008 (Delta iteration, ctx cancel,
  backpressure timeout, error surfaced on close).
- `llm_test.go` — `TestPublicAPISurface`, `TestNoOpenaiGoImportOutsideLLM`
  (import-boundary; `go list -deps` walked), `TestResponseTypeFields`
  (reflection over Response struct).
- `config/config_test.go` — env binding, validation, defaults.

Coverage at completion: llm 89.9% / config 94.7% (both ≥85% target).

## 8. MX Tag Plan (applied — 18 tags across 7 source files)

| File::Symbol | Tag | Reason |
|--------------|-----|--------|
| `llm.go::Client` | @MX:ANCHOR | Primary LLM interface; callers: cmd/usearch, future SPEC-SYN-001, SPEC-DEEP-*, tests; fan_in ≥ 3 |
| `llm.go::New` | @MX:ANCHOR | Sole constructor; cmd entrypoints + tests; fan_in ≥ 3 |
| `client.go::defaultClient` | @MX:ANCHOR | All production LLM calls flow through this struct; fan_in ≥ 3 |
| `client.go::Complete` | @MX:ANCHOR | Hot path; called by router fallthrough, synthesis, classify, deep agents |
| `router.go::breaker` | @MX:WARN | Concurrent state machine; protected by mu; multiple goroutines may call Record/Allow |
| `router.go::Router.Route` | @MX:ANCHOR | Provider iteration is the load-bearing dispatch contract |
| `retry.go::isNonRetryable` | @MX:NOTE | Mapping table from HTTP status → retry behaviour; bug here changes resilience semantics |
| `cost.go::costMiddleware` | @MX:WARN | openai-go middleware boundary; capturing the cost header is privileged context-store access |
| `cost.go::applyBudgetCap` | @MX:NOTE | NFR-LLM-003: Response is returned alongside ErrBudgetExceeded |
| `stream.go::Stream` | @MX:WARN | Channel + goroutine; backpressure timeout cancels upstream |

All tags: `[AUTO]` prefix, `@MX:SPEC: SPEC-LLM-001`,
`@MX:REASON:` mandatory for ANCHOR/WARN; `code_comments: en`.

## 9. Risks Realised

| Original Risk | Outcome |
|---------------|---------|
| LiteLLM API drift | Pinned to `v1.83.7-stable.patch.1`; DEBUG log on missing cost header gives early warning |
| Cost header missing for Ollama | Treated as `0.0`; DEBUG log once per (provider, model); no block |
| openai-go v1 breaking changes | Pinned exact minor; Renovate weekly bumps gate on CI |
| Streaming backpressure deadlock | 30 s timeout closes channel + cancels upstream; unit test guards |
| Master key leak via slog | Confirmed clean across 100-call test; no full `http.Request` ever passed to slog |
| Auth failures cascading through fallthrough | 401/403 fail-fast (no retry, no fallthrough); operator sees immediately |
| In-package circuit breaker bugs | 5 dedicated tests (open at 50%, half-open after 30s, probe success/failure transitions, router skips Open) all green |
| `priority-based-routing` ignored by future LiteLLM | Go-side router is the source of truth for fallthrough; LiteLLM is the second layer |
| Budget cap $0.50 too low for /deep | Configurable via env; SPEC-DEEP-004 will raise for /deep tier |
| openai-go raw headers exposure | Confirmed via `option.WithMiddleware`; RED test `TestCostTrackerSumsByProvider` is the canary |
| Ollama base URL unreachable in CI | Fallback chain terminates at Ollama; tests use stubs not real Ollama |

## 10. Self-Review Outcome

Resolved Open Questions (from spec.md §11):

- **Q1 openai-go vs sashabaranov/go-openai** → openai-go chosen
  (official provenance; matching schema; middleware API for cost
  extraction).
- **Q2 Embedder routing** → Route via LiteLLM proxy (unified cost
  tracking); direct route to `services/embedder` deferred until
  SPEC-IDX-002 measures volume.
- **Q3 Per-service virtual keys** → Deferred to SPEC-AUTH-002 (M6);
  V1 uses single master key.
- **Q4 Circuit breaker library** → In-package (~80 LoC); no gobreaker
  dep added.
- **Q5 Budget cap timing** → Post-flight only (V1); pre-flight
  estimation requires Go tiktoken-equivalent + per-model pricing
  table (maintenance-heavy).
- **Q6 Streaming API shape** → `<-chan Delta` with final Delta
  carrying `Err` on error.
- **Q7 Ollama base URL** → Explicit env (`OLLAMA_BASE_URL`); silently
  dropped from priority list with WARN log if unset.

---

*End of plan.md (post-hoc).*
