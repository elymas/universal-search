# Research — SPEC-LLM-001 LiteLLM Proxy Integration

Author: limbowl (via manager-spec, plan phase)
Date: 2026-04-25
Status: Research complete, feeds into `.moai/specs/SPEC-LLM-001/spec.md`
Scope: LiteLLM proxy v1.83.7+ feature surface, Go client selection, provider
routing, cost tracking, observability integration, configuration model, auth,
error handling — for Go 1.25.8 on Universal Search monorepo
(`github.com/elymas/universal-search`).

---

## 1. LiteLLM Proxy Feature Surface (v1.83.7+)

LiteLLM Proxy is an OpenAI-compatible gateway that normalizes 100+ LLM providers
(Anthropic, OpenAI, Azure, Google Vertex, Bedrock, Ollama, vLLM, etc.) behind a
single `/v1/chat/completions` + `/v1/embeddings` + `/v1/completions` endpoint.
The proxy is the canonical choice in `tech.md` §3 ("LLM router: **LiteLLM
proxy** — single endpoint, per-key provider routing, cost tracking").

### 1.1 Pinned Version

- Image: `ghcr.io/berriai/litellm:v1.83.7-stable.patch.1`
- Already committed in `deploy/docker-compose.yml` (line 134) by SPEC-BOOT-001.
- License: MIT (on allowlist per SPEC-DEP-001 REQ-DEP-004 Table 5.1).
- Release channel: `stable.patch.N` is the long-term-stable track (patched
  quarterly for security + provider schema drift).

### 1.2 Capabilities Consumed by SPEC-LLM-001

| Feature | Use in Universal Search | Source |
|---------|-------------------------|--------|
| Provider routing (model_list + router_settings) | Claude Opus / Sonnet / Haiku, GPT-4o-mini, Ollama local | docs.litellm.ai/docs/proxy/configs |
| Fallback chains (`fallbacks`) | Primary Anthropic → secondary OpenAI → tertiary Ollama on 429/5xx | docs.litellm.ai/docs/proxy/reliable_completions |
| Cost tracking (`x-litellm-response-cost` header + `/spend/logs`) | Prom counter `usearch_llm_cost_usd_total` | docs.litellm.ai/docs/proxy/cost_tracking |
| Virtual keys (per-service) | `researcher`, `storm`, future agents isolated by key | docs.litellm.ai/docs/proxy/virtual_keys |
| Embeddings (`/v1/embeddings`) | Route BGE-M3 through proxy or direct to services/embedder | (Open Question §11.2) |
| Streaming (`stream: true`) | SSE deltas for /deep and synthesis surfaces | OpenAI-compat |
| Prompt caching (Anthropic) | 75% cost savings on repeated planner prompts (tech.md §3) | Anthropic native; LiteLLM passthrough |
| Budget limits (`max_budget`, `budget_duration`) | Per-virtual-key cap (complementary to in-process NFR-LLM-003) | docs.litellm.ai/docs/proxy/virtual_keys |
| Observability hooks (`LITELLM_CALLBACK_URL`, Prometheus endpoint) | Deferred — we observe at the Go client layer, not proxy layer | — |

### 1.3 Capabilities Not Used in V1

- LiteLLM Admin UI (out of scope for V1; future deploy SPEC).
- LiteLLM model_group load-balancing with Redis — we use static priority list.
- LiteLLM guardrails + prompt filters — deferred to SPEC-SEC-001.
- LiteLLM team/organization RBAC — overlaps with SPEC-AUTH-002 (M6); explicit
  non-goal for V1 LLM client.

### 1.4 Health Check

LiteLLM exposes `GET /health` (already wired to docker-compose healthcheck at
line 149 of `deploy/docker-compose.yml`: `wget -qO- http://localhost:4000/health`).
The Go client MUST NOT assume LiteLLM is up at init time — it connects lazily
on first request and recovers from transient unavailability via retry (REQ-LLM-004).

---

## 2. Go Client Selection

LiteLLM exposes an OpenAI-compatible API. Two credible Go clients target this
surface:

| Library | Maintainer | GitHub | Status 2026 | License | Notes |
|---------|-----------|--------|-------------|---------|-------|
| **`github.com/openai/openai-go`** | OpenAI (official) | openai/openai-go | v1.x stable; auto-generated from OpenAPI spec | Apache-2.0 | Newer (2024+); matches current OpenAI SDK conventions; well-documented (Context7 benchmark 76.52, High reputation). |
| `github.com/sashabaranov/go-openai` | Community | sashabaranov/go-openai | Mature; wide adoption pre-2024 | Apache-2.0 | De-facto community SDK; larger existing ecosystem; slightly behind on new features. |

### 2.1 Recommendation: `github.com/openai/openai-go`

Rationale:

- **Official provenance** — aligns with `.moai/project/tech.md` principle 6
  ("LLM is a replaceable dependency — LiteLLM proxy in front. Switching
  Claude ↔ Opus ↔ local model must be a config change, not a code change"):
  the client library should be the one with the longest support horizon.
- **Base URL override** — both clients support setting a custom base URL
  (`option.WithBaseURL("http://localhost:4000")` in openai-go) to point at
  LiteLLM proxy. Confirmed at pkg.go.dev/github.com/openai/openai-go.
- **Context7 score** — openai-go at 76.52 / 297 snippets / High reputation
  vs. go-openai at 74.95 / 87 snippets — roughly equivalent quality; official
  provenance is the tie-breaker.
- **Streaming** — openai-go exposes `chat.Completions.NewStreaming(ctx, params)`
  returning an `*ssestream.Stream[ChatCompletionChunk]` with `.Next()` /
  `.Current()` / `.Err()` / `.Close()` — maps cleanly onto our channel-based
  delta iterator in REQ-LLM-008.
- **Structured request/response** — the Go types match OpenAI's OpenAPI
  schema verbatim, which is what LiteLLM targets for compatibility.

### 2.2 Pin Version

Pin via run-phase `go get github.com/openai/openai-go@vN.N.N` and lock into
`go.sum`. Exact patch captured at run-phase merge time. Minimum: latest 1.x
minor available at run-phase execution.

### 2.3 What We Do NOT Get From openai-go

- **Per-request cost extraction** — LiteLLM returns the cost in the
  `x-litellm-response-cost` HTTP header (and `response.usage.total_cost` field
  when LiteLLM's `general_settings.store_prompts_in_spend_logs: True` is set).
  The openai-go SDK does not surface arbitrary response headers by default.
  Solution: use `option.WithMiddleware` + `httputil` response capture, OR
  extract from `response.Usage` when present. Both paths are plumbed in
  `internal/llm/cost.go` (§6.4).
- **Fallback chain on the Go side** — LiteLLM handles fallback internally;
  our Go retry-then-fallback in REQ-LLM-004 is a second layer for the case
  when LiteLLM itself becomes unreachable.
- **Custom headers on every request** — we need `X-Request-ID` propagation
  (SPEC-OBS-001 REQ-OBS-002). Handled via `option.WithHeader(...)` per-request
  or an `http.RoundTripper` wrapper registered via `option.WithHTTPClient`.

### 2.4 Alternative: Raw `net/http`

Rejected. Rewriting an OpenAI-compatible Go client (~2000 LoC) is orthogonal
to Universal Search's value prop. The marginal savings (no external dep, full
control of response headers) do not outweigh the maintenance burden across
the full OpenAI schema surface (tool calls, structured outputs, image parts,
audio, etc. that may land in M5+ SPECs).

---

## 3. Provider Routing Strategy

### 3.1 Model Classes (Universal Search Internal)

Universal Search tasks fall into four cost/latency tiers. The Go client exposes
these as named `ModelClass` values; `internal/llm/router.go` resolves each
class to a proxy-side `model_list` alias.

| Class | Primary | Secondary | Tertiary | Used By |
|-------|---------|-----------|----------|---------|
| `deep_research` | `claude-opus-4-7` | `gpt-4o` | — | SPEC-DEEP-001/002/003 (M5) |
| `summary` | `claude-sonnet-4-6` | `gpt-4o-mini` | `ollama/llama3.1` | SPEC-SYN-001 (M2) |
| `classify` | `claude-haiku-4-5` | `gpt-4o-mini` | `ollama/llama3.1:8b` | SPEC-IR-001 (M2) |
| `embed` | `bge-m3` (via embedder sidecar OR LiteLLM route) | `text-embedding-3-large` | — | SPEC-IDX-002 (M3) |

### 3.2 Provider Priority List

Per the planning prompt: **Claude → OpenAI-compat → Ollama**.

This is encoded in `deploy/litellm/config.yaml` as `fallbacks` per model-group
(LiteLLM native) AND mirrored as `internal/llm/router.go` priority list
(Go-side fallback when LiteLLM itself is unreachable, per REQ-LLM-004 clause
"fall through to the next provider in the configured priority list").

### 3.3 Circuit Breaker (NFR-LLM-002)

Per-provider circuit breaker opens at 50% failure rate over 1-minute rolling
window, half-opens after 30s. Implemented inline in `internal/llm/router.go`
(no external dep needed for single-window ring counter) OR via
`github.com/sony/gobreaker` (Open Question §11.4). V1 default: in-package
implementation (~80 LoC) to avoid adding a dep.

---

## 4. Cost Tracking Architecture

### 4.1 LiteLLM Cost Sources

LiteLLM emits cost data via three channels:

1. **Response header `x-litellm-response-cost`**: dollar-denominated float,
   populated on every successful `/v1/chat/completions` and `/v1/embeddings`
   call when model pricing is known. Source:
   <https://docs.litellm.ai/docs/proxy/cost_tracking#track-spend-per-request>.
2. **Response body `usage.total_cost`**: same value, embedded in the usage
   object when LiteLLM `general_settings.log_raw_request_response: True` and
   `store_prompts_in_spend_logs` are set. Less reliable than the header
   (depends on config); we treat the header as authoritative.
3. **Persistent `/spend/logs` endpoint**: LiteLLM maintains a spend database
   (SQLite by default, Postgres via `DATABASE_URL`) and exposes aggregate
   queries via `GET /spend/logs`. We DO NOT consume this endpoint in V1 —
   it duplicates the Prometheus time-series we emit; its purpose in future
   SPECs is cross-process audit (SPEC-AUTH-003).

### 4.2 Prometheus Emission (via SPEC-OBS-001)

Two new metrics registered in `internal/llm/cost.go`, imported into
`internal/obs/metrics` via the seam pattern (SPEC-OBS-001 allows domain
packages to register metrics on the shared registry via `metrics.Register(...)`
helper — the seam is documented in SPEC-OBS-001 §6.2):

- `usearch_llm_calls_total` (counter, labels: `{provider, model, outcome}`
  where `outcome ∈ {success, failure, timeout}`) — REQ-LLM-003.
- `usearch_llm_cost_usd_total` (counter, labels: `{provider, model}`) —
  REQ-LLM-006.
- `usearch_llm_latency_seconds` (histogram, labels: `{provider, model}`) —
  NFR-LLM-001 supporting metric.

Label cardinality is bounded by the `model_list` in `deploy/litellm/config.yaml`
(≤15 entries at V1), and `provider` is the enum `{anthropic, openai, ollama}`.
This satisfies NFR-OBS-002 (cardinality safety) from SPEC-OBS-001.

### 4.3 Exemplar Linking

Per SPEC-OBS-001 §11 Open Question 5 (exemplar sampling), the
`usearch_llm_latency_seconds` histogram emits a Prometheus exemplar with
`trace_id=<current span>` and `request_id=<from ctx>` when an OTel recording
span is active in the request context. This enables one-click pivot from a
latency-spike dashboard to the exact trace.

### 4.4 Per-Request Budget Cap (NFR-LLM-003)

Pre-flight cost estimation from LiteLLM is not reliable (prompt tokens are
counted by the proxy after request normalization). We enforce the cap
post-flight:

- Before returning the response, the Go client checks
  `cost > cfg.Budget.PerRequestCapUSD` (default $0.50 from
  `LITELLM_BUDGET_USD`).
- On breach, the client returns `ErrBudgetExceeded` alongside the response
  (response is still returned so callers can decide to honor or drop; cost
  is still tracked for accounting).
- This is intentionally post-flight, not pre-flight, per Open Question §11.5.
  The alternative (pre-flight estimation) requires a tiktoken-equivalent
  tokenizer in Go, which is maintenance-heavy and model-family-specific.

---

## 5. Observability Integration (SPEC-OBS-001 Consumption)

SPEC-OBS-001 defines the public API:

- `obs.Logger(ctx) *slog.Logger` — enriched with request_id / trace_id / span_id.
- `obs.Tracer(name) trace.Tracer` — OTel tracer factory.
- `obs.WithRequestID(ctx, id)` / `obs.RequestID(ctx)` — context plumbing.
- Named Prometheus collectors as exported vars.

SPEC-LLM-001 consumes this API without importing `prometheus/client_golang` or
`go.opentelemetry.io/otel` directly (per SPEC-OBS-001 REQ-OBS-006 "no direct
prometheus import outside obs"). Path for LLM-specific metrics registration:

1. `internal/obs/metrics/registry.go` exposes `(r *Registry) Register(c prometheus.Collector) error`
   as a public method on the registry. (This is already supported by
   `promclient.Registry.Register()` in the stdlib idiom.)
2. `internal/llm/cost.go` defines its `CounterVec` / `HistogramVec` instances
   as package-private vars, populated at `llm.Init(obs *obs.Obs)` via
   `obs.Metrics.Register(...)`.
3. The boundary check in SPEC-OBS-001 REQ-OBS-006's `TestNoDirectPrometheusImportOutsideObs`
   needs to be updated in SPEC-LLM-001's run phase to allow `internal/llm/` as
   a permitted non-obs consumer — OR we restructure so that the metric
   collectors are defined in `internal/obs/metrics/llm.go` and consumed by
   `internal/llm`. **Decision: restructure.** All named metric collectors live
   under `internal/obs/metrics/` (the existing pattern from SPEC-OBS-001);
   `internal/llm/cost.go` holds only the business logic that calls
   `obs.LLMCalls.WithLabelValues(...).Inc()`. This preserves the import
   boundary test verbatim and needs no changes to SPEC-OBS-001.

### 5.1 Per-Call Observability Record

Every `Client.Complete` / `Client.Stream` / `Client.Embed` call emits:

- **slog event** (INFO level):
  ```
  {
    "time": "...",
    "level": "INFO",
    "msg": "llm call",
    "request_id": "01JXXX...",   # from ctx
    "trace_id": "...",            # from OTel span (if active)
    "span_id": "...",
    "provider": "anthropic",
    "model": "claude-opus-4-7",
    "prompt_tokens": 1234,
    "completion_tokens": 567,
    "latency_ms": 2345,
    "cost_usd": 0.0234
  }
  ```
- **Counter increment**: `usearch_llm_calls_total{provider,model,outcome}`.
- **Cost counter**: `usearch_llm_cost_usd_total{provider,model}` added by
  `cost_usd`.
- **Histogram observation**: `usearch_llm_latency_seconds{provider,model}`.
- **OTel span**: name `llm.call`, attributes `{llm.provider, llm.model,
  llm.prompt_tokens, llm.completion_tokens, llm.cost_usd}` (following OTel
  GenAI semantic convention draft; see Ref §9).
- **Exemplar** on the histogram: `{trace_id, request_id}` when active span.

### 5.2 Secret Redaction (REQ-LLM-007)

`LITELLM_MASTER_KEY` and provider API keys MUST NEVER appear in logs. Guards:

- The slog enrich handler in `internal/obs/log/enrich.go` (SPEC-OBS-001) does
  not log request bodies. We add an assertion test in
  `internal/llm/client_test.go` that captures the full slog output for 100
  synthetic calls and asserts `grep "sk-"` / `grep "LITELLM_MASTER_KEY"` returns
  no matches.
- Error paths that wrap HTTP errors use `errors.Is` / `errors.As` — never
  `fmt.Errorf("... %v", req)` where `req` might include headers.

### 5.3 Cardinality Guard (REQ-LLM-007 + NFR-OBS-002)

Per SPEC-OBS-001's `TestNoUnboundedLabels` allowlist, we add three label
names to the allowlist in SPEC-OBS-001's run phase artifact (a compatible
update, not a breaking change):

- `provider` (enum: anthropic, openai, ollama)
- `model` (enum: 15 or fewer values at V1)

`outcome` already exists in the allowlist (`{success, failure, timeout}`).

No per-request-ID, per-user-ID, per-prompt labels. Exemplars carry trace
linkage.

---

## 6. LiteLLM Configuration Model

### 6.1 File Location

Committed at `deploy/litellm/config.yaml`, mounted into the LiteLLM container
as `/app/config.yaml` via a new volume entry in `deploy/docker-compose.yml`.
Command line: `--config /app/config.yaml`.

### 6.2 Config Schema (excerpt)

```yaml
model_list:
  # Anthropic primary
  - model_name: claude-opus-4-7
    litellm_params:
      model: anthropic/claude-opus-4-7-20260101
      api_key: os.environ/ANTHROPIC_API_KEY
  - model_name: claude-sonnet-4-6
    litellm_params:
      model: anthropic/claude-sonnet-4-6-20260101
      api_key: os.environ/ANTHROPIC_API_KEY
  - model_name: claude-haiku-4-5
    litellm_params:
      model: anthropic/claude-haiku-4-5-20260101
      api_key: os.environ/ANTHROPIC_API_KEY

  # OpenAI fallback
  - model_name: gpt-4o
    litellm_params:
      model: openai/gpt-4o
      api_key: os.environ/OPENAI_API_KEY
  - model_name: gpt-4o-mini
    litellm_params:
      model: openai/gpt-4o-mini
      api_key: os.environ/OPENAI_API_KEY

  # Ollama local fallback
  - model_name: ollama/llama3.1
    litellm_params:
      model: ollama/llama3.1:70b
      api_base: os.environ/OLLAMA_BASE_URL
  - model_name: ollama/llama3.1-small
    litellm_params:
      model: ollama/llama3.1:8b
      api_base: os.environ/OLLAMA_BASE_URL

  # Embeddings
  - model_name: text-embedding-3-large
    litellm_params:
      model: openai/text-embedding-3-large
      api_key: os.environ/OPENAI_API_KEY

router_settings:
  routing_strategy: priority-based-routing
  num_retries: 3
  timeout: 30
  fallbacks:
    - claude-opus-4-7: [gpt-4o]
    - claude-sonnet-4-6: [gpt-4o-mini, ollama/llama3.1]
    - claude-haiku-4-5: [gpt-4o-mini, ollama/llama3.1-small]

general_settings:
  master_key: os.environ/LITELLM_MASTER_KEY
  database_url: os.environ/DATABASE_URL
  store_prompts_in_spend_logs: false   # privacy: do not persist prompt text
```

### 6.3 Required Env Vars

Added to root `.env.example` in SPEC-LLM-001:

```
# LLM (SPEC-LLM-001)
LITELLM_MASTER_KEY=
LITELLM_BUDGET_USD=0.50
ANTHROPIC_API_KEY=
OPENAI_API_KEY=
OLLAMA_BASE_URL=http://localhost:11434
```

(`DATABASE_URL` is already required by the Postgres service in
SPEC-BOOT-001 via `deploy/docker-compose.yml` line 144.)

---

## 7. Authentication

### 7.1 Client → LiteLLM

Bearer token: `Authorization: Bearer ${LITELLM_MASTER_KEY}`. Source:
`docs.litellm.ai/docs/proxy/virtual_keys`. Implementation: the openai-go
client uses `option.WithAPIKey(cfg.MasterKey)` which sets this header
automatically.

### 7.2 LiteLLM → Upstream Providers

LiteLLM reads `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `OLLAMA_BASE_URL` from
its own environment (already wired in `deploy/docker-compose.yml` lines
141–143). Universal Search's Go client does NOT see these keys.

### 7.3 Virtual Keys (Deferred)

LiteLLM supports per-consumer virtual keys created via the admin API. For V1
all services share `LITELLM_MASTER_KEY`. Per-service keys
(researcher, storm, future agents) land in SPEC-AUTH-002 (M6) as part of team
RBAC.

### 7.4 Key Rotation

Rotation is a deploy-time operation: update env var, restart LiteLLM
container. Go client picks up the new key on next `llm.Init` or on hot
reload (not supported in V1 — process restart required).

---

## 8. Error Handling and Retry Policy

### 8.1 Transient Errors — Retry (REQ-LLM-004)

| Status Code | Retry? | Backoff |
|-------------|--------|---------|
| 429 Too Many Requests | Yes, up to 3 times | 250ms, 500ms, 1s (exponential with jitter) |
| 500 / 502 / 503 / 504 | Yes, up to 3 times | Same |
| 408 Request Timeout | Yes, up to 3 times | Same |
| Network connection error (DNS, refused, reset) | Yes, up to 3 times | Same |

After 3 retry failures on the same provider, fall through to the next
provider in the Universal Search-side priority list (`Anthropic → OpenAI →
Ollama`). Total attempt budget: 3 providers × 3 retries = 9 requests worst
case, with each successive provider getting the remaining context deadline
from `ctx`.

### 8.2 Fatal Errors — No Retry

| Status Code | Behavior |
|-------------|----------|
| 400 Bad Request | Return error immediately (malformed prompt, token limit exceeded) |
| 401 / 403 Unauthorized | Return error immediately; do NOT retry with different provider (auth is per-proxy, not per-provider) |
| 404 Not Found | Return error immediately (model name typo) |

### 8.3 Circuit Breaker (NFR-LLM-002)

Per-provider circuit breaker in `internal/llm/router.go`:

- Rolling 1-minute window counter of successes vs. failures.
- When failure ratio ≥ 50% AND window has ≥ 10 requests, open circuit for 30s.
- After 30s, transition to half-open: allow 1 probe request; if success,
  close circuit; if failure, re-open for another 30s.
- When circuit is open, the router skips that provider and moves to the
  next in priority.

Implementation option A: `github.com/sony/gobreaker/v2` (mature, single dep,
~500 LoC maintained). Option B: in-package (~80 LoC, no dep). **Default: B**
(keep deps minimal). Revisit in Open Question §11.4.

### 8.4 Streaming Backpressure (REQ-LLM-008)

The `Stream(ctx, req) (<-chan Delta, error)` method returns a buffered channel
(capacity 16 by default). If the consumer does not read for >30s the client
cancels the upstream stream via `ctx` and closes the channel. This prevents
memory growth on stalled consumers and matches the `context` idiom for
Go long-running operations.

### 8.5 Context Cancellation

All methods accept `ctx context.Context` and honor cancellation immediately
(in-flight HTTP request is cancelled via `http.Request.WithContext(ctx)`).
This is the standard Go pattern and is already provided by openai-go.

---

## 9. Reference Implementations

### 9.1 charmbracelet/crush (LSP client for agent IDEs)

charmbracelet/crush integrates LiteLLM as the LLM proxy for its agent panel.
Inspection of `go.mod` and `internal/llm/providers/openai/` confirms:

- Uses `github.com/sashabaranov/go-openai` (community client, not
  openai-go). Chose to pre-date openai-go's v1 release.
- Sets base URL to LiteLLM proxy endpoint via `openai.ClientConfig.BaseURL`.
- Does NOT implement Go-side fallback chains — delegates to LiteLLM's
  `router_settings.fallbacks`. This is the "simpler is better" baseline.

Universal Search goes one layer further (REQ-LLM-004 Go-side fallback) because
we have the additional failure mode of LiteLLM itself being unreachable
(container restart, network partition in k8s deploy). Crush runs LiteLLM
in-process as a sidecar; we run it as a separate compose/k8s service with its
own failure domain.

### 9.2 gpt-researcher LLM Router Patterns

gpt-researcher (consumed as a Python sidecar in `services/researcher`, per
SPEC-BOOT-001) has its own Python LLM router with a config file. For Universal
Search Go orchestration, we do NOT reuse gpt-researcher's router — each layer
owns its LLM routing:

- Python services (researcher, storm): use litellm Python SDK directly,
  pointed at the same LiteLLM proxy endpoint.
- Go orchestration plane (`internal/llm`): uses openai-go pointed at LiteLLM
  proxy.

This keeps the proxy as the single point of truth for routing + cost. The Go
and Python sides share the same model aliases from `deploy/litellm/config.yaml`.

### 9.3 OpenAI Go Streaming Reference

From pkg.go.dev/github.com/openai/openai-go (verified via Context7 snippets):

Pattern for chat completions streaming: create a streaming request with
`client.Chat.Completions.NewStreaming(ctx, params)`, iterate with
`for stream.Next() { chunk := stream.Current(); acc.AddChunk(chunk); ... }`,
check `stream.Err()`, and call `stream.Close()` in defer. This maps onto our
channel-based API without re-exporting openai-go types to callers (REQ-LLM-002's
provider-agnostic semantics).

### 9.4 LiteLLM Python Reference — Cost Header

From docs.litellm.ai/docs/proxy/cost_tracking#track-spend-per-request, the
`x-litellm-response-cost` header is set on every successful response when
model pricing is configured. Value format: decimal string (e.g. `0.0234`).
Missing header = LiteLLM did not compute cost (unknown model pricing or
disabled). The Go client treats absence as `cost_usd = 0.0` with a DEBUG-level
slog warning.

---

## 10. Research Conclusions (feed into spec.md)

1. **LiteLLM version**: `v1.83.7-stable.patch.1` — already pinned in compose
   by SPEC-BOOT-001. No go.mod change; LiteLLM is a sidecar service, not a
   linked library.

2. **Go client**: `github.com/openai/openai-go` (official OpenAI Go SDK).
   Pinned to latest 1.x at run phase. Base URL set to LiteLLM proxy endpoint.

3. **Package layout**:
   `internal/llm/{client.go,provider.go,router.go,cost.go,retry.go}` +
   `internal/llm/config/config.go`. Named metrics collectors live in
   `internal/obs/metrics/llm.go` to preserve SPEC-OBS-001 import-boundary test.

4. **Routing**: Priority list `Anthropic → OpenAI → Ollama`, encoded twice —
   once in `deploy/litellm/config.yaml` `router_settings.fallbacks`, once in
   `internal/llm/router.go` for LiteLLM-unreachable fallback. Model classes
   `{deep_research, summary, classify, embed}` resolve to proxy aliases.

5. **Cost tracking**: Read `x-litellm-response-cost` response header per call;
   emit to `usearch_llm_cost_usd_total{provider,model}` Prometheus counter;
   exemplar link via `{trace_id, request_id}` when span is active.

6. **Observability**: Consume SPEC-OBS-001 `obs.Logger`, `obs.Tracer`, and
   named metrics (registered under `internal/obs/metrics/llm.go`). Emit one
   slog event + counter + histogram + span per LLM call. No sensitive data
   in labels or log messages.

7. **Auth**: `LITELLM_MASTER_KEY` via Bearer header. Virtual keys deferred to
   SPEC-AUTH-002 (M6). Keys never logged — assertion test enforced.

8. **Error handling**: 3-retry exponential backoff (250ms/500ms/1s) on
   429/5xx/network; fall through to next provider; auth/400/404 fail-fast.
   Per-provider circuit breaker at 50% failure rate over 1-min window, 30s
   half-open probe.

9. **Budget cap**: Post-flight check against `LITELLM_BUDGET_USD` (default
   $0.50 per request); breach returns `ErrBudgetExceeded` alongside the
   response. Pre-flight estimation deferred.

10. **New env vars** (.env.example delta): `LITELLM_MASTER_KEY`,
    `LITELLM_BUDGET_USD`, `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`,
    `OLLAMA_BASE_URL`.

11. **Compose delta**: Add `./litellm/config.yaml:/app/config.yaml:ro` volume
    mount + `--config /app/config.yaml` command to the LiteLLM service in
    `deploy/docker-compose.yml`. Existing healthcheck remains unchanged.

12. **Open questions for spec.md §11**: openai-go vs. sashabaranov/go-openai
    tie-break; embedder routing (proxy vs. direct); virtual key rollout
    (V1 vs. M6); streaming API ergonomics (chan vs. iterator); cost cap
    enforcement (pre vs. post); Ollama discovery (env vs. autodetect).

---

## References

External:

1. LiteLLM proxy documentation: <https://docs.litellm.ai/docs/>
2. LiteLLM proxy config: <https://docs.litellm.ai/docs/proxy/configs>
3. LiteLLM cost tracking: <https://docs.litellm.ai/docs/proxy/cost_tracking>
4. LiteLLM router settings: <https://docs.litellm.ai/docs/proxy/reliable_completions>
5. LiteLLM virtual keys: <https://docs.litellm.ai/docs/proxy/virtual_keys>
6. OpenAI Go SDK: <https://pkg.go.dev/github.com/openai/openai-go>
7. sashabaranov/go-openai: <https://pkg.go.dev/github.com/sashabaranov/go-openai>
8. gobreaker v2: <https://pkg.go.dev/github.com/sony/gobreaker/v2>
9. OpenTelemetry GenAI semconv: <https://opentelemetry.io/docs/specs/semconv/gen-ai/>
10. Prometheus exemplars: <https://prometheus.io/docs/prometheus/latest/feature_flags/#exemplars-storage>
11. Anthropic prompt caching: <https://docs.anthropic.com/en/docs/build-with-claude/prompt-caching>

Internal:

- `.moai/project/tech.md` §3 Synthesis layer — LiteLLM proxy authoritative choice
- `.moai/project/structure.md` §3 Bounded Contexts — `internal/llm/` owner
- `.moai/project/roadmap.md` M1 — SPEC-LLM-001 owner: expert-backend
- `.moai/specs/SPEC-BOOT-001/spec.md` §7 File Impact — `internal/llm/llm.go` stub; LiteLLM compose entry
- `.moai/specs/SPEC-DEP-001/spec.md` §6.1 Future-Dependencies — SPEC-LLM-001 referenced via Asynq (task queue, deferred, not in this SPEC)
- `.moai/specs/SPEC-OBS-001/spec.md` — Logger/Tracer/named metrics public API consumed here

Ready to hand off to `.moai/specs/SPEC-LLM-001/spec.md`.
