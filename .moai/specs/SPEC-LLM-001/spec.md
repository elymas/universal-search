---
id: SPEC-LLM-001
title: LiteLLM Proxy Integration
milestone: M1 — Foundation
status: approved
priority: P0
owner: expert-backend
methodology: tdd
coverage_target: 85
created: 2026-04-25
updated: 2026-04-25
approved_by: limbowl
approved_at: 2026-04-25
depends_on: [SPEC-BOOT-001, SPEC-OBS-001]
blocks: [SPEC-SYN-001, SPEC-DEEP-001, SPEC-DEEP-002, SPEC-DEEP-003, SPEC-DEEP-004]
---

# SPEC-LLM-001: LiteLLM Proxy Integration

## 1. Purpose

SPEC-BOOT-001 installed the LiteLLM proxy as a compose service
(`ghcr.io/berriai/litellm:v1.83.7-stable.patch.1` at `deploy/docker-compose.yml`
line 134) and reserved `internal/llm/llm.go` as an empty stub. SPEC-OBS-001
established the observability baseline (`obs.Logger`, `obs.Tracer`, named
Prometheus collectors) that every domain package must consume without
importing prometheus/otel libraries directly. SPEC-LLM-001 now delivers the
**Go-side LLM client** that brings those two foundations together:

- A provider-agnostic `internal/llm.Client` interface (`Complete`, `Stream`,
  `Embed`) backed by `github.com/openai/openai-go` targeting the LiteLLM
  proxy endpoint.
- A committed LiteLLM configuration (`deploy/litellm/config.yaml`) that
  declares the V1 model_list (Claude Opus/Sonnet/Haiku, GPT-4o/4o-mini,
  Ollama Llama-3.1 70B/8B, text-embedding-3-large), the
  `priority-based-routing` strategy, and the Claude → OpenAI → Ollama
  fallback chains per task class.
- Uniform per-call observability: one slog event, one counter increment,
  one histogram observation (with trace exemplar), one OTel span, and — when
  present — one cost entry on `usearch_llm_cost_usd_total`, all wired
  through the SPEC-OBS-001 public API.
- A retry-then-fallback policy (3 retries with exponential backoff per
  provider, fall through to next provider on exhaustion) and per-provider
  circuit breaker (50% failure over 1-min window, 30s half-open probe) that
  cover both transient proxy-side errors and the case where LiteLLM itself
  is unreachable.
- Per-request budget cap (`ErrBudgetExceeded` at post-flight) as a cost
  safety net orthogonal to LiteLLM's own virtual-key budget features.

Completion unblocks every M5 `/deep` SPEC (SPEC-DEEP-001 STORM integration,
SPEC-DEEP-002 multi-agent pipeline, SPEC-DEEP-003 tree exploration, and
SPEC-DEEP-004 quota + cost guard) and M2's SPEC-SYN-001 (basic synthesis
wrapper). Every agent and synthesis path in M2–M5 talks to LLMs through this
package and through no other path. The LiteLLM proxy is the single point of
routing; `internal/llm` is the single point of Go-side LLM contact.

This is the final SPEC of Milestone M1. Together with SPEC-BOOT-001 (scaffold
+ compose), SPEC-DEP-001 (dependency policy + audit CI), and SPEC-OBS-001
(observability baseline), SPEC-LLM-001 satisfies the M1 exit criterion in
`.moai/project/roadmap.md` §5 that every foundation primitive — compose up,
`usearch --version`, CI green, telemetry emitting, LLM routing — exists and
is wired for M2 consumption.

## 2. Scope

### 2.1 In-Scope

| Ref | Item |
|-----|------|
| a | `internal/llm/` package: `client.go` (Client interface), `provider.go` (ModelClass + provider registry), `router.go` (priority-list router + circuit breaker), `retry.go` (exponential backoff + fallthrough), `cost.go` (x-litellm-response-cost extraction + budget cap), `stream.go` (channel-based delta iterator) |
| b | `internal/llm/config/config.go`: `Config` struct (BaseURL, MasterKey, PerRequestCapUSD, Timeout, ModelClass aliases), env binding via koanf, validation |
| c | `deploy/litellm/config.yaml`: committed LiteLLM proxy config (model_list for ≥3 Anthropic + ≥2 OpenAI + ≥2 Ollama + 1 embedding model; priority-based-routing; fallbacks per model-group; `store_prompts_in_spend_logs: false`) |
| d | Compose delta: volume mount `./litellm/config.yaml:/app/config.yaml:ro` on the LiteLLM service in `deploy/docker-compose.yml`; add `--config /app/config.yaml` command; healthcheck unchanged |
| e | Named metric collectors registered in `internal/obs/metrics/llm.go` and re-exported via `obs.LLMCalls`, `obs.LLMCost`, `obs.LLMLatency` (preserves SPEC-OBS-001 import-boundary test) |
| f | Go-side retry (3×, 250/500/1000 ms exponential) and fallthrough on exhaustion; per-provider circuit breaker (50% over 60s window, 30s half-open) |
| g | Per-call observability: slog event + counter + histogram + OTel span + (optional) exemplar; secrets never logged |
| h | Post-flight budget cap: `ErrBudgetExceeded` returned when `cost_usd > cfg.PerRequestCapUSD`; response still delivered; cost still counted |
| i | `cmd/usearch/main.go` conditional init: when `LITELLM_MASTER_KEY` is set, construct `llm.Client`; gated off by `--no-llm` flag for tests/dev |
| j | `.env.example` additions: `LITELLM_MASTER_KEY`, `LITELLM_BUDGET_USD`, `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `OLLAMA_BASE_URL` |
| k | `go.mod` additions: `github.com/openai/openai-go` (exact 1.x minor pinned at run phase) |

### 2.2 Out-of-Scope

- **Per-tenant virtual keys** — LiteLLM supports virtual keys; V1 uses a
  single `LITELLM_MASTER_KEY` shared across all services. Per-service and
  per-team keys land in SPEC-AUTH-002 (M6).
- **Pre-flight cost estimation** — requires a Go-native tokenizer
  (tiktoken-equivalent); maintenance-heavy and model-family-specific. V1
  does post-flight rejection only. Revisit in a post-V1 SPEC.
- **LiteLLM admin UI and `/spend/logs` aggregation** — we emit Prometheus
  counters; LiteLLM's own spend database is not consumed by the Go client.
  Cross-process audit belongs to SPEC-AUTH-003.
- **LiteLLM guardrails, prompt filters, content moderation** — deferred to
  SPEC-SEC-001 (M8).
- **Prompt caching orchestration** — Anthropic prompt caching works
  transparently via LiteLLM passthrough; no Go-side orchestration in V1.
- **Task queue (Asynq)** for async `/deep` requests — reserved in SPEC-DEP-001
  future-deps table; consumed by SPEC-DEEP-004 (M5), not SPEC-LLM-001.
- **Python-side LiteLLM SDK usage** in `services/researcher` / `services/storm`
  / `services/embedder` — each Python service talks to the same LiteLLM proxy
  endpoint via its own Python litellm SDK; Python wiring is owned by those
  services' M2/M5 SPECs.
- **Tool calls, structured outputs, image/audio modalities** — OpenAI schema
  supports them; openai-go surfaces them; the Go client's initial interface
  exposes only `Complete` (text), `Stream` (text+deltas), and `Embed`.
  Extensions land in later SPECs as features consume them.
- **Hot reload of `deploy/litellm/config.yaml`** — requires LiteLLM container
  restart. A future SPEC may add SIGHUP support.
- **Ollama model auto-discovery** — V1 requires explicit `OLLAMA_BASE_URL`;
  no model probing. Deferred.

## 3. EARS Requirements

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| REQ-LLM-001 | Ubiquitous | The file `deploy/litellm/config.yaml` SHALL declare a `model_list` containing at least claude-opus-4-7, claude-sonnet-4-6, claude-haiku-4-5 (Anthropic), gpt-4o, gpt-4o-mini (OpenAI), ollama/llama3.1 (Ollama), and text-embedding-3-large (embeddings); a `router_settings` block with `routing_strategy: priority-based-routing` and per-model-group `fallbacks` chains (claude-opus → gpt-4o; claude-sonnet → gpt-4o-mini, ollama/llama3.1; claude-haiku → gpt-4o-mini, ollama/llama3.1-small); and `general_settings` with `master_key: os.environ/LITELLM_MASTER_KEY` and `store_prompts_in_spend_logs: false`. | P0 | `TestConfigYamlValid` parses the file as LiteLLM schema; `TestConfigYamlContainsClaudeProvider`, `TestConfigYamlContainsOpenAIProvider`, `TestConfigYamlContainsOllamaProvider`, `TestConfigYamlContainsEmbeddingModel` verify presence; `TestConfigYamlFallbackChains` asserts each required fallback; `TestConfigYamlStorePromptsDisabled` verifies privacy flag. |
| REQ-LLM-002 | Ubiquitous | The `internal/llm` package SHALL expose a stable `Client` interface with three methods — `Complete(ctx, Request) (Response, error)`, `Stream(ctx, Request) (<-chan Delta, error)`, `Embed(ctx, EmbedRequest) (EmbedResponse, error)` — where Request/Response/Delta/EmbedRequest/EmbedResponse are provider-agnostic value types that do NOT re-export `openai-go` types to callers; Response includes `PromptTokens`, `CompletionTokens`, `LatencyMs`, `CostUSD`, and `Provider/Model` fields. | P0 | `TestPublicAPISurface` verifies symbol presence; `TestNoOpenaiGoImportOutsideLLM` asserts `go list -deps` shows `openai-go` only under `internal/llm/`; `TestResponseTypeFields` verifies Response struct field set. |
| REQ-LLM-003 | Event-Driven | WHEN a `Client.Complete`, `Client.Stream`, or `Client.Embed` call resolves (success, failure, or timeout), the package SHALL (a) emit a single slog record at level INFO with attributes `{request_id, provider, model, prompt_tokens, completion_tokens, latency_ms, cost_usd}` via `obs.Logger(ctx)`; (b) increment `obs.LLMCalls.WithLabelValues(provider, model, outcome)` where outcome ∈ `{success, failure, timeout}`; (c) observe `obs.LLMLatency.WithLabelValues(provider, model)` with duration in seconds; (d) create and end an OTel span named `llm.call` with attributes mirroring the slog record. | P0 | `TestClientCompleteEmitsSlogEventWithRequestID`, `TestClientCompleteIncrementsCallsCounter`, `TestClientCompleteRecordsLatencyHistogram`, `TestClientCompleteCreatesOTelSpan` each verify one aspect. |
| REQ-LLM-004 | Event-Driven | WHEN a provider returns HTTP 429, 408, 500, 502, 503, 504, or a network error, the Client SHALL retry up to 3 times with exponential backoff of 250 ms, 500 ms, 1000 ms (with ±10% jitter); IF all 3 retries fail THEN the Client SHALL move to the next provider in the configured priority list (Anthropic → OpenAI → Ollama); IF a provider returns HTTP 400, 401, 403, or 404 THEN the Client SHALL return the error immediately with NO retry and NO fallthrough. | P0 | `TestClientCompleteRetriesOn5xx`, `TestClientCompleteBackoffTimings`, `TestClientCompleteFailsThroughToNextProvider`, `TestClientCompleteAuthErrorNoRetry`, `TestClientCompleteBadRequestNoRetry`. |
| REQ-LLM-005 | Ubiquitous | The Client SHALL authenticate to the LiteLLM proxy via the `LITELLM_MASTER_KEY` environment variable sent as an `Authorization: Bearer <key>` header on every request; the key value SHALL NEVER appear in any slog record, OTel span attribute, Prometheus label, or wrapped error message. | P0 | `TestAuthBearerHeaderSent` asserts header on outbound request; `TestNoMasterKeyInLogs` captures 100 synthetic calls' log output and asserts `grep LITELLM_MASTER_KEY` returns zero matches (and that no literal key bytes appear); `TestNoMasterKeyInSpanAttributes` verifies OTel span attribute set. |
| REQ-LLM-006 | Event-Driven | WHEN a LiteLLM response includes the `x-litellm-response-cost` header, the Client SHALL parse it as a decimal float, expose it as `Response.CostUSD`, and increment `obs.LLMCost.WithLabelValues(provider, model)` by that value; WHEN the header is absent, the Client SHALL record `CostUSD = 0.0`, emit a DEBUG-level slog record `"cost header missing"`, and NOT increment the cost counter. | P0 | `TestCostTrackerSumsByProvider`, `TestCostTrackerExemplarWithRequestID`, `TestCostHeaderMissingDoesNotIncrement`, `TestCostHeaderMalformedLogsAndDoesNotPanic`. |
| REQ-LLM-007 | Ubiquitous | Prometheus metric labels emitted by `internal/llm` SHALL be drawn ONLY from bounded enums: `provider ∈ {anthropic, openai, ollama}`, `model ∈ deploy/litellm/config.yaml model_list aliases (≤15 values at V1)`, `outcome ∈ {success, failure, timeout}`; NO label SHALL carry request ID, user ID, prompt text, tenant ID, or any other unbounded value. Per-request trace linking SHALL use Prometheus exemplars (via histogram observation exemplar API), not labels. | P0 | `TestNoSensitiveDataInLabels` is a static-scan test extending SPEC-OBS-001's `TestNoUnboundedLabels` allowlist to include `provider` and `model`; `TestExemplarContainsRequestID` verifies exemplar emission on histogram observations when ctx carries a request ID. |
| REQ-LLM-008 | Optional / Event-Driven | WHERE the caller invokes `Client.Stream(ctx, req)`, the Client SHALL return a buffered channel of `Delta` values (capacity 16); each Delta MAY contain `{Content, ToolCallDelta, FinishReason}`; the channel SHALL be closed when the upstream stream ends OR when `ctx` is cancelled; IF the consumer does not read for longer than 30 seconds the Client SHALL cancel the upstream stream and close the channel. | P1 | `TestClientStreamIteratesDeltas`, `TestClientStreamClosesOnContextCancel`, `TestClientStreamBackpressureTimeout`, `TestClientStreamErrorSurfacedOnChannelClose`. |

## 4. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| NFR-LLM-001 | Performance | Go-side Client overhead — the wall-clock cost of `Client.Complete` minus the LiteLLM proxy round-trip — SHALL add less than 10 ms at p99 across a synthetic benchmark of 1000 concurrent calls against a stub LiteLLM (httptest.Server) returning fixed 2 KB responses. The benchmark `BenchmarkClientCompleteOverhead` lives at `internal/llm/bench/bench_test.go` and runs in CI on a scheduled weekly job (not per-PR, per SPEC-OBS-001 NFR-OBS-001 cadence). |
| NFR-LLM-002 | Reliability | A per-provider circuit breaker in `internal/llm/router.go` SHALL open when the failure ratio (failures / (successes + failures)) reaches or exceeds 50% within a 60-second rolling window of at least 10 observations; SHALL hold open for 30 seconds; SHALL transition to half-open on the next request after the 30-second window, admitting one probe; SHALL close on probe success or re-open (for another 30 seconds) on probe failure. While a provider's circuit is open the router SHALL skip that provider and move to the next in priority. |
| NFR-LLM-003 | Cost Safety | The Client SHALL enforce a per-request budget cap read from `LITELLM_BUDGET_USD` (default `0.50`). WHEN a response's extracted `cost_usd` exceeds the cap, the Client SHALL return `errors.Is(err, llm.ErrBudgetExceeded) == true` alongside the response (response is NOT discarded); the cost counter SHALL still be incremented; a WARN-level slog record SHALL be emitted with `{request_id, provider, model, cost_usd, cap_usd}`. Pre-flight estimation is OUT OF SCOPE (see §11.5). |

## 5. Acceptance Criteria

### REQ-LLM-001 — deploy/litellm/config.yaml

- File `deploy/litellm/config.yaml` exists and parses as valid YAML.
- `model_list` contains at least the 7 named entries with their LiteLLM
  provider prefixes (`anthropic/`, `openai/`, `ollama/`).
- Every entry sources its API key or base URL from `os.environ/<VAR>`;
  no literal key strings in the file.
- `router_settings.routing_strategy == "priority-based-routing"`.
- `router_settings.num_retries >= 3` and `router_settings.timeout >= 30`.
- `router_settings.fallbacks` contains exactly the three chains listed in
  REQ-LLM-001 (claude-opus-4-7, claude-sonnet-4-6, claude-haiku-4-5).
- `general_settings.master_key == "os.environ/LITELLM_MASTER_KEY"`.
- `general_settings.store_prompts_in_spend_logs == false`.
- Tests `TestConfigYamlValid`, `TestConfigYamlContainsClaudeProvider`,
  `TestConfigYamlContainsOpenAIProvider`,
  `TestConfigYamlContainsOllamaProvider`,
  `TestConfigYamlContainsEmbeddingModel`,
  `TestConfigYamlFallbackChains`,
  `TestConfigYamlStorePromptsDisabled` all pass.

### REQ-LLM-002 — Client Interface

- Exported types under `internal/llm/`:
  - `Client` interface with `Complete`, `Stream`, `Embed` methods.
  - `Request`, `Response`, `Delta`, `EmbedRequest`, `EmbedResponse` value types.
  - `ModelClass` enum (`DeepResearch`, `Summary`, `Classify`, `Embed`).
- `Response` has all fields: `Text string`, `PromptTokens int`,
  `CompletionTokens int`, `LatencyMs int64`, `CostUSD float64`,
  `Provider string`, `Model string`, `FinishReason string`.
- No exported symbol of `internal/llm` has a field of type
  `github.com/openai/openai-go.*`.
- `go list -deps ./...` output: `github.com/openai/openai-go` appears only
  under import paths rooted at `github.com/elymas/universal-search/internal/llm/`.
- `TestPublicAPISurface`, `TestNoOpenaiGoImportOutsideLLM`,
  `TestResponseTypeFields` all pass.

### REQ-LLM-003 — Per-Call Observability

- Single slog record per `Complete` / `Stream` / `Embed` call, captured by a
  buffered test handler; parses as JSON; contains the 7 named attributes.
- `obs.LLMCalls` counter incremented exactly once per call with the correct
  label tuple.
- `obs.LLMLatency` histogram receives exactly one observation per call.
- An OTel span `llm.call` is recorded (via
  `sdktrace/tracetest.NewInMemoryExporter`) with attributes mirroring the slog
  record.
- `TestClientCompleteEmitsSlogEventWithRequestID`,
  `TestClientCompleteIncrementsCallsCounter`,
  `TestClientCompleteRecordsLatencyHistogram`,
  `TestClientCompleteCreatesOTelSpan` all pass.

### REQ-LLM-004 — Retry and Fallthrough

- `TestClientCompleteRetriesOn5xx`: stub returns 503 three times then 200 →
  4 outbound requests observed, final response is the 200.
- `TestClientCompleteBackoffTimings`: measured inter-retry delays within
  ±25% of 250/500/1000 ms (wider tolerance to absorb scheduler jitter).
- `TestClientCompleteFailsThroughToNextProvider`: provider A returns 503×3,
  router moves to provider B which returns 200 → outbound request count:
  A=3, B=1; final response is B's.
- `TestClientCompleteAuthErrorNoRetry`: provider returns 401 → 1 outbound
  request, error returned immediately, no fallthrough.
- `TestClientCompleteBadRequestNoRetry`: provider returns 400 → same pattern
  as 401.
- `TestClientCompleteNotFoundNoRetry`: provider returns 404 → same pattern.
- `TestClientCompleteCtxCancelStopsRetry`: context cancelled during backoff
  wait → function returns `context.Canceled` promptly, no further retries.

### REQ-LLM-005 — Auth and Secret Handling

- `TestAuthBearerHeaderSent`: httptest server records received
  `Authorization` header; asserts value starts with `Bearer ` and equals
  the configured master key.
- `TestNoMasterKeyInLogs`: fixture runs 100 `Complete` calls against a stub
  that returns both success and error responses; captured slog output scanned
  with regex — no substring match for the master key value.
- `TestNoMasterKeyInSpanAttributes`: captured OTel spans walked, no attribute
  value equals the master key.
- `TestErrorWrappingRedactsKey`: inject a provider error that includes the
  master key in its raw body; verify that the returned `error.Error()` does
  NOT contain the key.

### REQ-LLM-006 — Cost Extraction

- `TestCostTrackerSumsByProvider`: 5 synthetic calls across 2 providers with
  known costs → `obs.LLMCost.WithLabelValues(provider, model)` counter values
  match expected sums per label tuple.
- `TestCostTrackerExemplarWithRequestID`: when `obs.WithRequestID(ctx, "X")`
  is set and an OTel span is active, the histogram exemplar for that
  observation includes both `trace_id` and `request_id=X`.
- `TestCostHeaderMissingDoesNotIncrement`: stub returns no cost header →
  counter unchanged, DEBUG-level slog record `"cost header missing"` emitted,
  `Response.CostUSD == 0.0`.
- `TestCostHeaderMalformedLogsAndDoesNotPanic`: stub returns
  `x-litellm-response-cost: notanumber` → counter unchanged,
  WARN-level slog record emitted, `Response.CostUSD == 0.0`, no panic.

### REQ-LLM-007 — Cardinality and Secret Labels

- `TestNoSensitiveDataInLabels`: walks all `CounterVec` / `HistogramVec`
  registrations in `internal/obs/metrics/llm.go`; asserts label names are a
  subset of `{provider, model, outcome}`; extends SPEC-OBS-001's
  allowlist map to include `provider` and `model`.
- `TestExemplarContainsRequestID`: when histogram observation occurs within
  a ctx carrying a non-empty request ID AND an active OTel span,
  `WithExemplar` is called with a `Labels{trace_id, request_id}` pair (via
  `prometheus.Exemplar`). Verified via `io_prometheus_client.MetricFamily`
  introspection of the handler output.

### REQ-LLM-008 — Streaming API (P1)

- `TestClientStreamIteratesDeltas`: stub streams 10 SSE chunks → consumer
  receives 10 Delta values followed by channel close; no error on
  `<-streamErrCh`.
- `TestClientStreamClosesOnContextCancel`: consumer cancels ctx midway →
  channel closes within 200 ms; upstream `http.Request.Context().Err() ==
  context.Canceled`.
- `TestClientStreamBackpressureTimeout`: consumer does not read for 31 seconds
  → channel closes with `llm.ErrStreamBackpressureTimeout`; upstream cancelled.
- `TestClientStreamErrorSurfacedOnChannelClose`: stub emits 5 deltas then
  returns upstream error → consumer receives 5 deltas, then channel closes,
  and a separate error getter (`stream.Err()` or final Delta with non-nil
  error field, depending on API choice in §6.2) surfaces the error.

### NFR-LLM-001 — Performance Budget

- `BenchmarkClientCompleteOverhead` at `internal/llm/bench/bench_test.go`
  drives 1000 goroutines through `Client.Complete` against a
  `httptest.NewServer` stub returning a fixed 2 KB LiteLLM-compatible response
  with `x-litellm-response-cost: 0.001` header.
- Baseline measurement: `Response.LatencyMs` field (LLM round-trip as
  measured from inside openai-go).
- Overhead measurement: wall-clock duration of `Client.Complete` minus
  `Response.LatencyMs`.
- Assertion: p99 overhead < 10 ms.
- Ships in the same scheduled-weekly CI bench workflow as SPEC-OBS-001's
  NFR-OBS-001 benchmarks.

### NFR-LLM-002 — Circuit Breaker

- `TestCircuitBreakerOpensAt50PercentFailure`: feed the breaker 10
  observations with 5 successes + 5 failures within 60 s → state transitions
  to Open.
- `TestCircuitBreakerHalfOpensAfter30s`: breaker in Open state for 30 s →
  next request transitions to Half-Open and is allowed through.
- `TestCircuitBreakerHalfOpenProbeFailureReopens`: probe fails → state
  returns to Open for another 30 s.
- `TestCircuitBreakerHalfOpenProbeSuccessCloses`: probe succeeds → state
  transitions to Closed; counters reset.
- `TestRouterSkipsOpenProvider`: provider A circuit is Open, provider B is
  Closed → router delivers request to B without attempting A.

### NFR-LLM-003 — Budget Cap

- `TestBudgetCapRejectsRequestExceedingCap`: cap = 0.10, stub returns
  `x-litellm-response-cost: 0.15` → `Client.Complete` returns
  `errors.Is(err, llm.ErrBudgetExceeded) == true`; `Response` is non-nil
  and contains full text; cost counter incremented by 0.15.
- `TestBudgetCapAllowsRequestWithinCap`: cap = 0.10, cost = 0.05 → no error.
- `TestBudgetCapConfigurableFromEnv`: `LITELLM_BUDGET_USD=1.00` at Init
  overrides the default 0.50.
- `TestBudgetCapWarnLogEmitted`: breach case emits exactly one WARN slog
  record.

## 6. Technical Approach

### 6.1 Package Layout (to be created by run phase)

```
internal/llm/
├── llm.go                    # replace stub with package doc + public re-exports
├── client.go                 # Client interface + New(cfg, obs) constructor
├── provider.go               # ModelClass enum, Provider struct, static registry
├── router.go                 # priority-list router + circuit breaker
├── retry.go                  # backoff + classification of retryable errors
├── cost.go                   # cost header parser + budget cap + counter emit
├── stream.go                 # channel-based Delta iterator + backpressure guard
├── client_test.go            # REQ-LLM-002, REQ-LLM-003, REQ-LLM-005, REQ-LLM-006
├── router_test.go            # REQ-LLM-004, NFR-LLM-002
├── retry_test.go             # backoff timing tests
├── cost_test.go              # REQ-LLM-006, NFR-LLM-003
├── stream_test.go            # REQ-LLM-008
├── config/
│   ├── config.go             # Config struct + env loader (koanf)
│   └── config_test.go        # env binding, validation, default values
└── bench/
    └── bench_test.go         # NFR-LLM-001

internal/obs/metrics/
└── llm.go                    # NEW: LLMCalls, LLMCost, LLMLatency collectors + allowlist extension
```

### 6.2 Public API Sketch (`internal/llm/client.go`)

```go
// Package llm is the Universal Search LLM gateway client. It targets the
// LiteLLM proxy (deploy/docker-compose.yml litellm service) via an
// OpenAI-compatible HTTP surface, adds provider priority routing, retry with
// exponential backoff and fallthrough, circuit breaking, cost tracking, and
// a per-request budget cap. All LLM I/O in the Go orchestration plane flows
// through this package.
//
// Domain packages MUST NOT import github.com/openai/openai-go directly.
package llm

import (
    "context"
    "errors"

    "github.com/elymas/universal-search/internal/llm/config"
    "github.com/elymas/universal-search/internal/obs"
)

type ModelClass string

const (
    DeepResearch ModelClass = "deep_research"
    Summary      ModelClass = "summary"
    Classify     ModelClass = "classify"
    Embed        ModelClass = "embed"
)

type Request struct {
    Class       ModelClass
    System      string
    Messages    []Message
    MaxTokens   int
    Temperature float64
    // Override routes this request to a specific model alias, bypassing Class.
    Override string
}

type Message struct {
    Role    string // "user" | "assistant" | "system"
    Content string
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
    // Err is populated on the final delta if the stream ended in error.
    Err error
}

type EmbedRequest struct {
    Class ModelClass // must be Embed
    Input []string
}

type EmbedResponse struct {
    Vectors          [][]float32
    Provider         string
    Model            string
    PromptTokens     int
    LatencyMs        int64
    CostUSD          float64
}

type Client interface {
    Complete(ctx context.Context, req Request) (Response, error)
    Stream(ctx context.Context, req Request) (<-chan Delta, error)
    Embed(ctx context.Context, req EmbedRequest) (EmbedResponse, error)
    Close() error
}

// Errors surfaced to callers.
var (
    ErrBudgetExceeded            = errors.New("llm: per-request budget exceeded")
    ErrStreamBackpressureTimeout = errors.New("llm: stream consumer stalled")
    ErrAllProvidersFailed        = errors.New("llm: all providers in priority list exhausted")
    ErrModelNotConfigured        = errors.New("llm: model class has no configured provider")
)

// New constructs a Client wired to the LiteLLM proxy defined by cfg and
// emits telemetry through the provided obs bundle.
func New(cfg config.Config, o *obs.Obs) (Client, error)
```

### 6.3 Router Sketch (`internal/llm/router.go`)

```go
// Router selects a provider for a ModelClass and handles failover.
type Router struct {
    priorities map[ModelClass][]ProviderRef  // e.g. Summary -> [anthropic, openai, ollama]
    breakers   map[string]*breaker           // keyed by provider name
}

type ProviderRef struct {
    Provider string // "anthropic" | "openai" | "ollama"
    Model    string // LiteLLM alias resolved from config.yaml
}

// Route yields providers in priority order, skipping those whose circuit is Open.
// Returns ErrAllProvidersFailed if no provider is available.
func (r *Router) Route(ctx context.Context, class ModelClass) ([]ProviderRef, error)

// Record observes a call outcome against the provider's breaker.
func (r *Router) Record(provider string, success bool)
```

Circuit breaker state machine: Closed → Open (on ≥50% failure over ≥10
observations in 60 s rolling window) → Half-Open (after 30 s) → {Closed on
probe success, Open on probe failure}. Implemented as ~80 LoC in `router.go`
with a ring buffer of timestamped observations; no external dep.

### 6.4 Cost Extraction (`internal/llm/cost.go`)

Openai-go exposes raw HTTP responses via `option.WithMiddleware(...)` — a
`RequestOption` that wraps an `http.RoundTripper`. The cost middleware
captures `resp.Header.Get("x-litellm-response-cost")`, parses to float64 with
`strconv.ParseFloat`, and stashes it in the response context via a
package-private key. `Client.Complete` then reads the value after the openai-go
call returns, applies the budget cap, and emits the metric.

Pseudo:

```go
func costMiddleware(next option.MiddlewareNext) option.Middleware {
    return func(req *http.Request, opts ...option.RequestOption) (*http.Response, error) {
        resp, err := next(req, opts...)
        if err != nil {
            return resp, err
        }
        if s := resp.Header.Get("x-litellm-response-cost"); s != "" {
            if v, perr := strconv.ParseFloat(s, 64); perr == nil {
                setCost(req.Context(), v)
            } else {
                obs.Logger(req.Context()).Warn("malformed cost header", "raw", s)
            }
        }
        return resp, err
    }
}
```

### 6.5 Cmd Integration (`cmd/usearch/main.go`)

Minimal delta:

```go
noLLM := flag.Bool("no-llm", false, "disable LLM client (dev/test mode)")
flag.Parse()

// ... obs.Init from SPEC-OBS-001 ...

var client llm.Client
if !*noLLM && os.Getenv("LITELLM_MASTER_KEY") != "" {
    cfg := config.Load() // reads env
    client, err = llm.New(cfg, observability)
    if err != nil {
        observability.Logger.Error("llm init failed", "err", err)
        os.Exit(1)
    }
    defer client.Close()
}
_ = client // wired to future subcommands
```

Same delta in `cmd/usearch-api/main.go` and `cmd/usearch-mcp/main.go`.

### 6.6 Compose Delta (`deploy/docker-compose.yml`)

Add two lines to the existing `litellm` service:

```yaml
  litellm:
    # ... existing fields from SPEC-BOOT-001 (lines 132–156) ...
    volumes:
      - ./litellm/config.yaml:/app/config.yaml:ro    # NEW
    command: ["--config", "/app/config.yaml"]        # NEW
    # healthcheck unchanged: GET /health
```

No image change. Existing healthcheck + `depends_on` + env vars remain.

### 6.7 .env.example Delta

Append to root `.env.example`:

```
# LLM (SPEC-LLM-001)
LITELLM_MASTER_KEY=
LITELLM_BUDGET_USD=0.50
ANTHROPIC_API_KEY=
OPENAI_API_KEY=
OLLAMA_BASE_URL=http://localhost:11434
```

### 6.8 go.mod Impact

New direct dependency (pinned at run phase, exact version captured in
`docs/dependencies.md` per SPEC-DEP-001 REQ-DEP-007):

```
github.com/openai/openai-go v1.x.y     // OpenAI-compatible Go SDK
```

No other direct deps. `github.com/sony/gobreaker` is NOT added in V1 (circuit
breaker is in-package). `github.com/prometheus/client_golang` already present
via SPEC-OBS-001; we consume it only through `internal/obs/metrics`.

### 6.9 Metric Registration Path (`internal/obs/metrics/llm.go`)

New file owned by SPEC-LLM-001 but living under `internal/obs/metrics/` to
preserve SPEC-OBS-001's import-boundary tests:

```go
package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
    LLMCalls   *prometheus.CounterVec   // labels: provider, model, outcome
    LLMCost    *prometheus.CounterVec   // labels: provider, model
    LLMLatency *prometheus.HistogramVec // labels: provider, model
)

// Called from NewRegistry in metrics.go alongside the six existing collectors.
func registerLLM(r *prometheus.Registry) {
    LLMCalls = prometheus.NewCounterVec(/* ... */)
    LLMCost = prometheus.NewCounterVec(/* ... */)
    LLMLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
        Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60},
    }, []string{"provider", "model"})
    r.MustRegister(LLMCalls, LLMCost, LLMLatency)
}
```

Then extend the `obs.Obs` struct (top-level re-exports in `internal/obs/obs.go`)
with `LLMCalls`, `LLMCost`, `LLMLatency` pointing to the new vars. The
cardinality allowlist in `internal/obs/metrics/metrics_test.go` adds `provider`
and `model` (a SPEC-OBS-001-compatible addition per research.md §5.3).

### 6.10 Harness Level

Per `.moai/config/sections/harness.yaml` auto-routing: 8 REQs (7 × P0 + 1 × P1)
+ 3 NFRs touching 1 package (6 sub-files) + compose delta + env delta + 1 new
file under `internal/obs/metrics/` = **standard** harness level. Sprint Contract
(design.yaml §11) is optional but recommended; evaluator profile `default`
applies.

## 7. File Impact

### 7.1 Created

| Path | Purpose |
|------|---------|
| `internal/llm/client.go` | Client interface + New constructor (REQ-LLM-002) |
| `internal/llm/provider.go` | ModelClass enum + Provider registry (REQ-LLM-002) |
| `internal/llm/router.go` | Priority-list router + circuit breaker (REQ-LLM-004, NFR-LLM-002) |
| `internal/llm/retry.go` | Exponential backoff + retryable-error classification (REQ-LLM-004) |
| `internal/llm/cost.go` | x-litellm-response-cost parser + budget cap (REQ-LLM-006, NFR-LLM-003) |
| `internal/llm/stream.go` | Channel-based Delta iterator + backpressure (REQ-LLM-008) |
| `internal/llm/client_test.go` | RED tests for REQ-LLM-002/003/005/006 |
| `internal/llm/router_test.go` | RED tests for REQ-LLM-004 + NFR-LLM-002 |
| `internal/llm/retry_test.go` | Backoff timing tests |
| `internal/llm/cost_test.go` | RED tests for REQ-LLM-006 + NFR-LLM-003 |
| `internal/llm/stream_test.go` | RED tests for REQ-LLM-008 |
| `internal/llm/config/config.go` | Config struct + env loader (koanf) |
| `internal/llm/config/config_test.go` | Env binding + validation |
| `internal/llm/bench/bench_test.go` | NFR-LLM-001 benchmark |
| `internal/obs/metrics/llm.go` | LLMCalls/LLMCost/LLMLatency collectors (lives in obs to preserve import-boundary) |
| `deploy/litellm/config.yaml` | LiteLLM proxy model_list + router_settings + general_settings (REQ-LLM-001) |

### 7.2 Modified

| Path | Change |
|------|--------|
| `internal/llm/llm.go` | Replace 2-line stub with package doc + re-exports of Client/Request/Response/Delta/ModelClass/errors |
| `internal/obs/obs.go` | Add `LLMCalls`, `LLMCost`, `LLMLatency` re-exports from `internal/obs/metrics` |
| `internal/obs/metrics/metrics.go` | Call `registerLLM(r)` from `NewRegistry`; extend cardinality allowlist with `provider`, `model` |
| `internal/obs/metrics/metrics_test.go` | Extend `TestNoUnboundedLabels` allowlist |
| `deploy/docker-compose.yml` | Add `volumes: ./litellm/config.yaml:/app/config.yaml:ro` and `command: ["--config", "/app/config.yaml"]` to the `litellm` service |
| `.env.example` | Append 5 LLM env vars (§6.7) |
| `cmd/usearch/main.go` | Conditional LLM init when `LITELLM_MASTER_KEY` set; `--no-llm` flag |
| `cmd/usearch-api/main.go` | Same |
| `cmd/usearch-mcp/main.go` | Same |
| `go.mod` / `go.sum` | Add `github.com/openai/openai-go` pinned to latest 1.x |
| `docs/dependencies.md` | Regenerated via `scripts/gen-deps-manifest.sh` (covers new openai-go dep) |

### 7.3 Unchanged (by design)

- `internal/router/`, `internal/fanout/`, `internal/adapters/*`,
  `internal/index/*`, `internal/synthesis/`, `internal/auth/`, `internal/eval/`
  — all remain stubs; they begin consuming `llm.Client` in their own
  M2–M5 SPECs.
- Python services (`services/researcher`, `services/storm`,
  `services/embedder`) — each talks to LiteLLM proxy via its own Python SDK
  in its own SPEC.

## 8. TDD Plan

Development follows RED-GREEN-REFACTOR per
`quality.development_mode: tdd` (quality.yaml). Representative RED-phase
tests, written before implementation, grouped by REQ. 25+ tests in total
covering every REQ and NFR acceptance criterion.

| # | Test | Layer | REQ | Assertion |
|---|------|-------|-----|-----------|
| 1 | `TestClientCompleteRoundTripsToLiteLLM` | `client_test.go` | REQ-LLM-002 | httptest stub receives POST `/v1/chat/completions`; response parsed into `Response`; `Response.Text` matches fixture |
| 2 | `TestClientCompleteEmitsSlogEventWithRequestID` | `client_test.go` | REQ-LLM-003 | Buffered slog handler captures one INFO record with 7 named attrs including `request_id` from ctx |
| 3 | `TestClientCompleteIncrementsCallsCounter` | `client_test.go` | REQ-LLM-003 | `obs.LLMCalls.WithLabelValues("anthropic","claude-sonnet-4-6","success")` counter delta == 1 |
| 4 | `TestClientCompleteRecordsLatencyHistogram` | `client_test.go` | REQ-LLM-003 | `obs.LLMLatency` histogram count == 1; sum > 0 |
| 5 | `TestClientCompleteCreatesOTelSpan` | `client_test.go` | REQ-LLM-003 | In-memory exporter captures span name `"llm.call"` with attrs `llm.provider`, `llm.model`, `llm.prompt_tokens`, `llm.completion_tokens`, `llm.cost_usd` |
| 6 | `TestClientCompleteRetriesOn5xx` | `router_test.go` | REQ-LLM-004 | Stub returns 503×3 then 200 → outbound request count == 4; final response is 200's body |
| 7 | `TestClientCompleteBackoffTimings` | `retry_test.go` | REQ-LLM-004 | Measured delays between retries within [187, 312] / [375, 625] / [750, 1250] ms (±25%) |
| 8 | `TestClientCompleteFailsThroughToNextProvider` | `router_test.go` | REQ-LLM-004 | Provider A stub returns 503×3; provider B stub returns 200 → A request count == 3, B == 1 |
| 9 | `TestClientCompleteAuthErrorNoRetry` | `retry_test.go` | REQ-LLM-004 | Stub returns 401 → 1 outbound request; returned error is the underlying 401 unwrap target |
| 10 | `TestClientCompleteBadRequestNoRetry` | `retry_test.go` | REQ-LLM-004 | Stub returns 400 → 1 outbound request |
| 11 | `TestClientCompleteNotFoundNoRetry` | `retry_test.go` | REQ-LLM-004 | Stub returns 404 → 1 outbound request |
| 12 | `TestClientCompleteCtxCancelStopsRetry` | `retry_test.go` | REQ-LLM-004 | Ctx cancelled during first backoff → returns `context.Canceled`; outbound request count == 1 |
| 13 | `TestAuthBearerHeaderSent` | `client_test.go` | REQ-LLM-005 | Stub captures `Authorization` header; asserts prefix `Bearer ` + correct key |
| 14 | `TestNoMasterKeyInLogs` | `client_test.go` | REQ-LLM-005 | 100 mixed-outcome calls; concatenated slog output scanned; `strings.Contains(out, masterKey) == false` |
| 15 | `TestNoMasterKeyInSpanAttributes` | `client_test.go` | REQ-LLM-005 | Captured spans; no attribute value equals master key |
| 16 | `TestErrorWrappingRedactsKey` | `client_test.go` | REQ-LLM-005 | Provider error body includes master key; returned error string does not |
| 17 | `TestCostTrackerSumsByProvider` | `cost_test.go` | REQ-LLM-006 | 5 calls across 2 providers with costs [0.01,0.02,0.01] and [0.03,0.02] → counter values 0.04 / 0.05 |
| 18 | `TestCostTrackerExemplarWithRequestID` | `cost_test.go` | REQ-LLM-006, REQ-LLM-007 | Histogram exemplar labels include `trace_id` (non-empty) and `request_id` (equals ctx-set value) |
| 19 | `TestCostHeaderMissingDoesNotIncrement` | `cost_test.go` | REQ-LLM-006 | Stub omits `x-litellm-response-cost` → counter unchanged, DEBUG log emitted, `Response.CostUSD == 0.0` |
| 20 | `TestCostHeaderMalformedLogsAndDoesNotPanic` | `cost_test.go` | REQ-LLM-006 | Stub returns garbage cost header → counter unchanged, WARN log emitted, no panic |
| 21 | `TestBudgetCapRejectsRequestExceedingCap` | `cost_test.go` | NFR-LLM-003 | Cap 0.10, cost 0.15 → `errors.Is(err, ErrBudgetExceeded)`; `Response` non-nil; counter incremented by 0.15 |
| 22 | `TestBudgetCapAllowsRequestWithinCap` | `cost_test.go` | NFR-LLM-003 | Cap 0.10, cost 0.05 → `err == nil` |
| 23 | `TestBudgetCapConfigurableFromEnv` | `config_test.go` | NFR-LLM-003 | `LITELLM_BUDGET_USD=1.00` → `cfg.PerRequestCapUSD == 1.00` |
| 24 | `TestBudgetCapWarnLogEmitted` | `cost_test.go` | NFR-LLM-003 | Breach → exactly one WARN slog record with `{request_id, provider, model, cost_usd, cap_usd}` |
| 25 | `TestCircuitBreakerOpensAt50PercentFailure` | `router_test.go` | NFR-LLM-002 | 10 observations (5 success + 5 failure) within 60s window → breaker state Open |
| 26 | `TestCircuitBreakerHalfOpensAfter30s` | `router_test.go` | NFR-LLM-002 | Open for 30 s + 1 ms → next request transitions to Half-Open and is admitted |
| 27 | `TestCircuitBreakerHalfOpenProbeFailureReopens` | `router_test.go` | NFR-LLM-002 | Probe returns 500 → state returns to Open |
| 28 | `TestCircuitBreakerHalfOpenProbeSuccessCloses` | `router_test.go` | NFR-LLM-002 | Probe returns 200 → state Closed; ring buffer reset |
| 29 | `TestRouterSkipsOpenProvider` | `router_test.go` | NFR-LLM-002 | Provider A circuit Open, provider B Closed → outbound request to B; A request count == 0 |
| 30 | `TestClientStreamIteratesDeltas` | `stream_test.go` | REQ-LLM-008 | SSE stub emits 10 chunks → consumer reads 10 Delta values; channel closed cleanly |
| 31 | `TestClientStreamClosesOnContextCancel` | `stream_test.go` | REQ-LLM-008 | Consumer cancels ctx → channel closes within 200 ms; upstream Request ctx == Canceled |
| 32 | `TestClientStreamBackpressureTimeout` | `stream_test.go` | REQ-LLM-008 | Consumer does not read for 31 s → channel closes with final Delta carrying `ErrStreamBackpressureTimeout` |
| 33 | `TestClientStreamErrorSurfacedOnChannelClose` | `stream_test.go` | REQ-LLM-008 | Stub emits 5 deltas + upstream error → consumer sees 5 deltas + 1 final Delta with non-nil `Err` |
| 34 | `TestConfigYamlValid` | `deploy/litellm/config_test.go` or a Go-side asset test | REQ-LLM-001 | File parses as YAML; matches minimal LiteLLM schema (model_list + router_settings + general_settings present) |
| 35 | `TestConfigYamlContainsClaudeProvider` | ditto | REQ-LLM-001 | model_list contains the three claude-* entries with `anthropic/` prefix |
| 36 | `TestConfigYamlContainsOpenAIProvider` | ditto | REQ-LLM-001 | model_list contains gpt-4o + gpt-4o-mini with `openai/` prefix |
| 37 | `TestConfigYamlContainsOllamaProvider` | ditto | REQ-LLM-001 | model_list contains ollama/llama3.1 entries with `ollama/` prefix |
| 38 | `TestConfigYamlContainsEmbeddingModel` | ditto | REQ-LLM-001 | model_list contains text-embedding-3-large |
| 39 | `TestConfigYamlFallbackChains` | ditto | REQ-LLM-001 | router_settings.fallbacks matches the three prescribed chains exactly |
| 40 | `TestConfigYamlStorePromptsDisabled` | ditto | REQ-LLM-001 | `general_settings.store_prompts_in_spend_logs == false` |
| 41 | `TestPublicAPISurface` | `llm_test.go` (package-level) | REQ-LLM-002 | `go/types.Lookup` confirms Client/Request/Response/Delta/ModelClass/errors all exported |
| 42 | `TestNoOpenaiGoImportOutsideLLM` | `llm_test.go` | REQ-LLM-002 | Walk `go list -deps -json`; assert openai-go consumers all under `internal/llm/` |
| 43 | `TestResponseTypeFields` | `llm_test.go` | REQ-LLM-002 | Reflection over Response struct confirms 8 named fields with correct types |
| 44 | `TestNoSensitiveDataInLabels` | `metrics_test.go` (extends SPEC-OBS-001's test) | REQ-LLM-007 | Scan of LLMCalls/LLMCost/LLMLatency registrations; label names ⊂ `{provider, model, outcome}` |
| 45 | `TestProviderHealthCheckRespectsTimeout` | `provider_test.go` | supporting | Construction with `LITELLM_BASE_URL` pointing at unreachable port + 1 s timeout → `New` returns error within 2 s |
| 46 | `BenchmarkClientCompleteOverhead` | `bench/bench_test.go` | NFR-LLM-001 | 1000 concurrent calls; p99 overhead < 10 ms (excluding `Response.LatencyMs`) |

Coverage target: 85% per `quality.test_coverage_target`. Benchmarks do not
count toward coverage. A `TestEmbedRoundTripsToProvider` is excluded from
the initial slice because embeddings are exercised by SPEC-IDX-002 (M3);
this SPEC includes the contract (REQ-LLM-002 `Embed` method) but defers
full round-trip testing to that SPEC. An acceptance-adequate smoke test
(`TestEmbedReturnsVectorsFromLiteLLM`, included in a later pass) covers the
basic shape.

RED-GREEN-REFACTOR per requirement:

1. RED: Write failing test for REQ-LLM-N.
2. GREEN: Implement the minimal code to pass.
3. REFACTOR: Tidy, extract shared helpers (backoff, exemplar emission)
   when they remove duplication across REQs.

Brownfield note: `internal/llm/llm.go` exists as a 2-line stub. Per
workflow-modes.md §Brownfield Enhancement, the existing stub has no
behavior to preserve, so characterization tests are not needed; RED tests
for REQ-LLM-002's public API surface are written against the planned
package surface.

## 9. Dependencies

### 9.1 Upstream SPEC Dependencies

- **SPEC-BOOT-001 (approved, merged to main)**: provides `internal/llm/llm.go`
  stub, `deploy/docker-compose.yml` with the LiteLLM service entry,
  `cmd/usearch*` binaries that will gain LLM-init blocks.
- **SPEC-OBS-001 (approved)**: provides `obs.Logger`, `obs.Tracer`,
  `obs.WithRequestID`, the Prometheus registry via `obs.Metrics`, and the
  import-boundary test infrastructure that we extend. MUST be merged before
  SPEC-LLM-001 run phase begins, because REQ-LLM-003 depends on `obs.LLMCalls`
  / `obs.LLMCost` / `obs.LLMLatency` collectors registered on the SPEC-OBS-001
  registry.

### 9.2 Parallelizable

- **SPEC-DEP-001** (in draft): SPEC-LLM-001 adds one new Go direct dep
  (`github.com/openai/openai-go`); the audit CI (SPEC-DEP-001 REQ-DEP-003)
  will run against the expanded `go.mod`. No blocking dependency in either
  direction. Per roadmap §3 "M1 | SPEC-BOOT-001 + SPEC-OBS-001 + SPEC-LLM-001
  (3-way)", these three SPECs are explicitly parallelizable; whichever lands
  first informs the others' lockfile state.

### 9.3 Downstream Blocked SPECs

- **SPEC-SYN-001 (M2 basic synthesis)**: `services/researcher` wrapper
  service talks to LiteLLM directly from Python, but the Go orchestration
  (`internal/synthesis/`) consumes `llm.Client` for citation assembly
  follow-up calls.
- **SPEC-DEEP-001 (M5 STORM integration)**: `services/storm` Python + Go
  orchestration both rely on `deploy/litellm/config.yaml` routing.
- **SPEC-DEEP-002 (M5 multi-agent pipeline)**: Researcher / Reviewer /
  Writer / Verifier all call `llm.Client.Complete` with different
  `ModelClass` routing.
- **SPEC-DEEP-003 (M5 tree exploration)**: depth/breadth budget enforcement
  consumes `usearch_llm_cost_usd_total` for per-query spend tracking.
- **SPEC-DEEP-004 (M5 /deep quota + cost guard)**: Haiku pre-screen + cost
  cap orchestration builds on `NFR-LLM-003` post-flight rejection pattern.

All M2+ domain SPECs that invoke an LLM (router LLM fallback in SPEC-IR-001,
synthesis in SPEC-SYN-001, streaming in SPEC-SYN-004) implicitly depend on
SPEC-LLM-001 for the Client contract, but only the five above are hard
blocks (`blocks:` front-matter).

### 9.4 External Dependencies (run-phase pins)

New Go module dependencies (see §6.8 for pinning policy):

```
github.com/openai/openai-go v1.x.y
```

No external service dependencies at SPEC-LLM-001 runtime by default
(integration tests use `httptest.NewServer` stubs). For full-stack dev
validation, developer runs `docker compose up litellm` and sets real API
keys in `.env`.

## 10. Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| LiteLLM upstream API drift between stable.patch.N releases (provider schema change, header rename) | Medium | High (silent cost tracking failure, cascading provider errors) | Pin exact patch version (`v1.83.7-stable.patch.1`); integration test `TestCostHeaderPresent` runs against a live LiteLLM container in a nightly job (deferred to SPEC-EVAL-002); DEBUG log on missing cost header gives early warning |
| `x-litellm-response-cost` missing or 0.0 for Ollama / unsupported providers | High | Low | Treat missing cost as `0.0`; DEBUG log once per (provider, model); never block request on cost |
| openai-go v1 breaking changes between minor versions | Low | Medium | Pin exact minor (`v1.N.x`); Renovate weekly bumps gate on CI passing; SDK follows semver |
| Streaming backpressure deadlocks block a goroutine indefinitely | Medium | Medium | 30 s backpressure timeout closes the channel + cancels upstream ctx (REQ-LLM-008); unit test `TestClientStreamBackpressureTimeout` guards it |
| Master key leak via slog enrichment handler (e.g., someone logs the `http.Request` struct) | Medium | High (credential exposure) | Never pass the full `http.Request` into slog; pass only extracted fields; `TestNoMasterKeyInLogs` + `TestErrorWrappingRedactsKey` enforce; code review checklist item |
| Provider auth failures cascading through retry-then-fallthrough chain, masking config errors | Medium | Medium | 401/403 fail-fast with no retry AND no fallthrough (REQ-LLM-004); auth error surfaces immediately to operator |
| In-package circuit breaker implementation bugs (off-by-one in window accounting) | Medium | Medium | 5 dedicated tests (Test 25–29); consider swapping to `github.com/sony/gobreaker/v2` if bugs persist (Open Question §11.4) |
| `priority-based-routing` strategy in LiteLLM ignored / deprecated in future versions | Low | Medium | Go-side router in `internal/llm/router.go` is the source of truth for fallthrough; LiteLLM's routing is a second layer — if the proxy stops honoring priority, Go side still works |
| Budget cap at $0.50 too low for `/deep` queries (Opus + large context) | Medium | Low (user impact: `ErrBudgetExceeded`) | Per-call override via ctx value or per-SPEC override; default configurable via `LITELLM_BUDGET_USD`; SPEC-DEEP-004 will raise default for /deep tier |
| openai-go does NOT expose raw response headers via its high-level API | Low | High (breaks REQ-LLM-006 cost extraction) | Mitigation confirmed via middleware pattern (`option.WithMiddleware`) in openai-go v1; research §2.3 validates; RED test 17 (`TestCostTrackerSumsByProvider`) fails loudly if the pattern stops working on a future SDK version |
| Ollama base URL not reachable in CI / some dev environments | High | Low | Fallback chain terminates at Ollama; if Ollama unreachable, previous providers are primary; tests use httptest stubs, not real Ollama |

## 11. Open Questions

The following are explicitly unresolved and documented here rather than
pre-decided. They do not block SPEC approval.

1. **Go client: openai-go (official) vs. sashabaranov/go-openai (community).**
   Research §2.1 recommends openai-go for official provenance, matching schema,
   and better-documented streaming. sashabaranov/go-openai has broader
   existing ecosystem (charmbracelet/crush uses it). **Default: openai-go.**
   Revisit if SDK stability issues emerge in first 4 weeks of operation.

2. **Embedder routing: LiteLLM proxy vs. direct to services/embedder.**
   Option A: route all embeddings through LiteLLM (single config file, unified
   cost tracking). Option B: bypass LiteLLM for BGE-M3 served by
   `services/embedder` Python sidecar (one less network hop for the
   high-volume indexing path). **Default: A (through LiteLLM).** Revisit
   in SPEC-IDX-002 (M3) when embedding volume measured; direct route can be
   added as a second branch in `llm.Client.Embed`.

3. **Virtual keys per service (researcher, storm, MCP server) in V1.** LiteLLM
   supports virtual keys created via admin API. Operational benefit: per-service
   spend visibility. Cost: admin API setup + key rotation runbook. **Default:
   deferred to SPEC-AUTH-002 (M6) where team RBAC lands.** V1 uses a single
   master key.

4. **Circuit breaker: in-package (~80 LoC) vs. `github.com/sony/gobreaker/v2`
   (external dep).** In-package is simpler to reason about and adds no dep.
   gobreaker is well-tested and used in production at Sony. **Default:
   in-package.** Revisit if the in-package implementation fails the 5
   dedicated tests or if future breakers (for adapters, embedder) would benefit
   from a shared library.

5. **Budget cap enforcement: post-flight (V1) vs. pre-flight estimation
   (future).** Pre-flight requires a Go tiktoken-equivalent + per-model pricing
   table. Maintenance cost is high; schemas drift across model families.
   **Default: post-flight only.** Revisit when pre-flight estimation libraries
   mature or when `/deep` tree exploration (SPEC-DEEP-003) needs tree-depth
   budget enforcement that cannot tolerate post-flight overrun.

6. **Streaming API: `<-chan Delta` (V1) vs. iterator-like `Next()`/`Current()`
   (openai-go style).** Channel API is idiomatic Go and composable with
   `select`. Iterator API mirrors openai-go and may feel natural to SDK-familiar
   callers. **Default: channel-based** with error surfaced as a final Delta
   carrying `Err` (§6.2). Revisit when SPEC-SYN-004 (streaming synthesis)
   measures consumer ergonomics.

7. **Ollama base URL: explicit env (`OLLAMA_BASE_URL`) vs. auto-discovery
   (probe `localhost:11434`, fall back to env).** Auto-discovery is convenient
   for dev but brittle in containerized deploys. **Default: explicit env,
   required when Ollama is in the priority list.** If env unset, Ollama is
   silently dropped from the priority list at `llm.New` time with a WARN log.

## 12. HISTORY

- 2026-04-25 (initial draft v0.1, limbowl via manager-spec):
  First EARS-formatted SPEC drafted after research phase. Scope derived from
  `.moai/project/roadmap.md` M1 row (owner: expert-backend, scope: "LiteLLM
  docker, Go client, provider routing table (Claude / OpenAI-compat / Ollama),
  cost tracking"). Built on SPEC-BOOT-001 (LiteLLM compose service + empty
  `internal/llm/llm.go` stub) and SPEC-OBS-001 (Logger/Tracer/named metric
  public API). Coordinates with SPEC-DEP-001 (openai-go addition will flow
  into the dependency manifest on next regeneration). Research artifact at
  `.moai/specs/SPEC-LLM-001/research.md` captures LiteLLM feature surface
  (v1.83.7+), Go client selection (openai-go over sashabaranov/go-openai),
  routing strategy, cost tracking paths, SPEC-OBS-001 consumption pattern,
  LiteLLM config model, auth, error handling, and reference implementations
  (charmbracelet/crush, gpt-researcher). 8 EARS REQs (7 × P0 + 1 × P1), 3
  NFRs, 46 representative RED tests, 7 Open Questions. Pinned: LiteLLM
  `v1.83.7-stable.patch.1` (already in compose via SPEC-BOOT-001),
  `github.com/openai/openai-go` latest 1.x (exact version captured at run
  phase in `docs/dependencies.md` per SPEC-DEP-001 REQ-DEP-007). Final SPEC
  of Milestone M1; completion satisfies the M1 exit criterion for LLM routing.
  Ready for plan-auditor review and annotation cycle.

---

*End of SPEC-LLM-001 v0.1*
