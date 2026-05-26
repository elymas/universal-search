# SPEC-LLM-001 Acceptance — Given/When/Then Scenarios

Created: 2026-04-25
Updated: 2026-04-26 (post-implementation reconciliation)
Author: limbowl (via manager-spec)
Status: implemented

## 0. Document Purpose

Given/When/Then acceptance scenarios for SPEC-LLM-001 — the Go-side
LLM client backed by openai-go and targeting the LiteLLM proxy. Each
scenario maps to one or more EARS REQs in spec.md §3.

## 1. Coverage Matrix

| AC | Scenario | REQs covered |
|----|----------|--------------|
| AC-001 | LiteLLM `config.yaml` declares full model_list + fallbacks | REQ-LLM-001 |
| AC-002 | Client interface exposes Complete/Stream/Embed (no openai-go leak) | REQ-LLM-002 |
| AC-003 | Per-call observability (slog + counter + histogram + OTel span) | REQ-LLM-003 |
| AC-004 | Retry on retryable errors; fallthrough on exhaustion | REQ-LLM-004 |
| AC-005 | Auth via LITELLM_MASTER_KEY; key never logged | REQ-LLM-005 |
| AC-006 | Cost extraction from x-litellm-response-cost header | REQ-LLM-006 |
| AC-007 | Cardinality allowlist enforced (provider, model bounded) | REQ-LLM-007 |
| AC-008 | Streaming API: channel iteration + cancel + backpressure | REQ-LLM-008 |
| NFR-001 | Performance budget (Go overhead < 10 ms p99) | NFR-LLM-001 |
| NFR-002 | Per-provider circuit breaker state machine | NFR-LLM-002 |
| NFR-003 | Post-flight budget cap (ErrBudgetExceeded) | NFR-LLM-003 |

## 2. Definition of Done

- [x] All 8 EARS REQs (7 P0 + 1 P1) have at least one green test.
- [x] All 3 NFRs validated.
- [x] `deploy/litellm/config.yaml` parses as valid YAML and matches the
      LiteLLM schema.
- [x] `go list -deps` confirms `openai-go` consumed only under
      `internal/llm/`.
- [x] `go test -race ./internal/llm/...` clean.
- [x] Coverage ≥ 85% (achieved: llm 89.9% / config 94.7%).
- [x] No master-key bytes appear in any slog record, OTel attribute,
      Prometheus label, or wrapped error message (100-call fixture
      regex scan).
- [x] TRUST 5 gates green.
- [x] 18 @MX tags applied across 7 source files.
- [x] `docs/dependencies.md` updated with openai-go pin (SPEC-DEP-001
      manifest regen on next push).

## 3. Functional Scenarios

### AC-001 — `deploy/litellm/config.yaml`

Maps to REQ-LLM-001.

#### AC-001.1: YAML validity

- **Given** the file at `deploy/litellm/config.yaml`.
- **When** parsed as YAML.
- **Then** parsing succeeds; the top-level keys include `model_list`,
  `router_settings`, `general_settings`.

#### AC-001.2: model_list coverage

- **Then** `model_list` contains:
  - `anthropic/claude-opus-4-7`
  - `anthropic/claude-sonnet-4-6`
  - `anthropic/claude-haiku-4-5`
  - `openai/gpt-4o`
  - `openai/gpt-4o-mini`
  - `ollama/llama3.1` (or equivalent ollama alias)
  - `openai/text-embedding-3-large` (embeddings)
- **And** every entry sources its API key or base URL from
  `os.environ/<VAR>` (no literal keys).

#### AC-001.3: router_settings

- **Then** `router_settings.routing_strategy == "priority-based-routing"`.
- **And** `router_settings.num_retries >= 3` and
  `router_settings.timeout >= 30`.
- **And** `router_settings.fallbacks` contains exactly:
  - `claude-opus-4-7 → [gpt-4o]`
  - `claude-sonnet-4-6 → [gpt-4o-mini, ollama/llama3.1]`
  - `claude-haiku-4-5 → [gpt-4o-mini, ollama/llama3.1:8b]`

#### AC-001.4: general_settings

- **Then** `general_settings.master_key == "os.environ/LITELLM_MASTER_KEY"`.
- **And** `general_settings.store_prompts_in_spend_logs == false`.

### AC-002 — Client interface

Maps to REQ-LLM-002.

- **Given** the `internal/llm` package.
- **Then** exported symbols include: `Client` interface,
  `ModelClass` enum (4 constants), `Request`, `Response`, `Delta`,
  `EmbedRequest`, `EmbedResponse` value types, and sentinel errors
  (`ErrBudgetExceeded`, `ErrStreamBackpressureTimeout`,
  `ErrAllProvidersFailed`, `ErrModelNotConfigured`).
- **And** `Response` has fields: `Text`, `Provider`, `Model`,
  `PromptTokens`, `CompletionTokens`, `LatencyMs`, `CostUSD`,
  `FinishReason`.
- **And** NO exported symbol has a field of type
  `github.com/openai/openai-go.*`.
- **And** `go list -deps github.com/elymas/universal-search/...`
  output: `github.com/openai/openai-go` appears only under import
  paths rooted at `github.com/elymas/universal-search/internal/llm/`.

### AC-003 — Per-call observability

Maps to REQ-LLM-003.

- **Given** an `httptest.Server` stub returning a 200 LiteLLM response.
- **When** `Client.Complete(ctx, req)` resolves.
- **Then**:
  - ONE slog INFO record is captured with attributes `{request_id,
    provider, model, prompt_tokens, completion_tokens, latency_ms,
    cost_usd}` (via buffered test handler).
  - `obs.LLMCalls.WithLabelValues(provider, model, "success")`
    counter is incremented by exactly 1.
  - `obs.LLMLatency.WithLabelValues(provider, model)` histogram
    records exactly 1 observation; sample sum > 0.
  - One OTel span named `llm.call` is recorded with attributes
    mirroring the slog record.
- **And** the same pattern holds for `outcome ∈ {failure, timeout}`.

### AC-004 — Retry and fallthrough

Maps to REQ-LLM-004.

#### AC-004.1: retry on 5xx

- **Given** a stub that returns 503 three times then 200.
- **When** `Complete` is called.
- **Then** 4 outbound requests observed; final response is the 200's
  body.

#### AC-004.2: backoff timings

- **Given** stub always returns 503.
- **When** `Complete` is called.
- **Then** measured inter-retry delays are within
  `[187, 312]` / `[375, 625]` / `[750, 1250]` ms (250 / 500 / 1000 ms
  with ±25% tolerance for scheduler jitter).

#### AC-004.3: provider fallthrough

- **Given** provider A stub returns 503×3; provider B stub returns 200.
- **When** `Complete` is called with the corresponding ModelClass.
- **Then** A request count == 3; B request count == 1; final
  response is B's.

#### AC-004.4: 401/403/404 no retry

- **Given** stub returns 401 (or 400 / 403 / 404).
- **Then** 1 outbound request; error returned immediately; NO
  fallthrough to next provider.

#### AC-004.5: ctx cancel during backoff

- **Given** stub returns 503; ctx cancelled during the first backoff
  wait.
- **Then** `Complete` returns `context.Canceled` promptly; outbound
  request count == 1.

### AC-005 — Auth and secret handling

Maps to REQ-LLM-005.

#### AC-005.1: Bearer header

- **Given** `LITELLM_MASTER_KEY=test-key-123`.
- **When** `Complete` is called.
- **Then** the captured request `Authorization` header == `Bearer
  test-key-123`.

#### AC-005.2: no master key in logs

- **Given** 100 mixed-outcome calls.
- **When** captured slog output is scanned.
- **Then** `strings.Contains(out, masterKey) == false`.

#### AC-005.3: no master key in OTel span attributes

- **Then** every captured span attribute value is checked; none
  equals the master key.

#### AC-005.4: error wrapping redacts key

- **Given** a provider error whose raw body includes the master key.
- **When** the wrapped error is converted to string via `err.Error()`.
- **Then** the returned string does NOT contain the master key.

### AC-006 — Cost extraction

Maps to REQ-LLM-006.

#### AC-006.1: cost sums per provider

- **Given** 5 calls across 2 providers with costs
  `[0.01, 0.02, 0.01]` (provider A) and `[0.03, 0.02]` (provider B).
- **Then** `LLMCost.WithLabelValues("A", model)` counter == 0.04;
  `LLMCost.WithLabelValues("B", model)` counter == 0.05.

#### AC-006.2: exemplar with request_id

- **Given** ctx with `reqid.WithContext(ctx, "X")` AND an active OTel
  span.
- **When** the histogram observation occurs.
- **Then** the exemplar contains both `trace_id` (non-empty) and
  `request_id="X"`.

#### AC-006.3: cost header missing

- **Given** stub omits `x-litellm-response-cost`.
- **Then** `Response.CostUSD == 0.0`; LLMCost counter unchanged;
  DEBUG slog record `"cost header missing"` emitted.

#### AC-006.4: cost header malformed

- **Given** stub returns `x-litellm-response-cost: notanumber`.
- **Then** `Response.CostUSD == 0.0`; LLMCost counter unchanged;
  WARN slog record emitted; no panic.

### AC-007 — Cardinality and labels

Maps to REQ-LLM-007.

- **Given** the SPEC-OBS-001 allowlist (extended with `provider` +
  `model`).
- **When** `TestNoUnboundedLabels` walks all `*CounterVec` /
  `*HistogramVec` registrations in `internal/obs/metrics/llm.go`.
- **Then** the label names are a subset of `{provider, model,
  outcome}`.
- **And** `provider ∈ {anthropic, openai, ollama}` (3 bounded values).
- **And** `model` is bounded by the LiteLLM `model_list` aliases
  (≤15 values at V1).
- **And** `outcome ∈ {success, failure, timeout}` (3 bounded values).
- **And** per-request trace linking uses Prometheus exemplars (via
  `WithExemplar`), NOT labels.

### AC-008 — Streaming API

Maps to REQ-LLM-008.

#### AC-008.1: delta iteration

- **Given** SSE stub emits 10 chunks.
- **Then** consumer reads 10 `Delta` values followed by channel close;
  no error on a final-Delta `Err` field.

#### AC-008.2: ctx cancel closes channel

- **Given** consumer cancels ctx midway.
- **Then** channel closes within 200 ms; upstream
  `http.Request.Context().Err() == context.Canceled`.

#### AC-008.3: backpressure timeout

- **Given** consumer does not read for 31 seconds.
- **Then** channel closes with final Delta carrying
  `ErrStreamBackpressureTimeout`; upstream request cancelled.

#### AC-008.4: error surfaced on close

- **Given** stub emits 5 deltas then returns upstream error.
- **Then** consumer reads 5 deltas, then channel closes, and the
  6th read returns the zero `Delta` value with `Err != nil`
  (or a separate `stream.Err()` getter — API choice resolved at run
  phase).

## 4. Non-Functional Acceptance

### NFR-LLM-001 — Performance budget

- `BenchmarkClientCompleteOverhead` at `internal/llm/bench/bench_test.go`
  drives 1000 goroutines through `Client.Complete` against an
  `httptest.NewServer` stub returning a fixed 2 KB body with
  `x-litellm-response-cost: 0.001`.
- Overhead = wall-clock(`Complete`) − `Response.LatencyMs`.
- Assertion: p99 overhead < 10 ms.
- Runs in scheduled-weekly CI bench job (per SPEC-OBS-001 NFR-OBS-001
  cadence).

### NFR-LLM-002 — Circuit breaker

#### NFR-002.1: opens at 50% / 10 samples

- Feed 10 observations (5 success + 5 failure within 60 s) → state
  transitions to Open.

#### NFR-002.2: half-opens after 30 s

- Open for 30 s + 1 ms → next request transitions to Half-Open and
  is admitted.

#### NFR-002.3: probe failure reopens

- Probe returns 500 → state returns to Open for another 30 s.

#### NFR-002.4: probe success closes

- Probe returns 200 → state Closed; ring buffer reset.

#### NFR-002.5: router skips Open provider

- Provider A circuit Open, provider B Closed → outbound request to B;
  A request count == 0.

### NFR-LLM-003 — Budget cap

#### NFR-003.1: cap exceeded returns Response + Error

- `cfg.PerRequestCapUSD = 0.10`; stub returns cost = 0.15.
- **Then** `errors.Is(err, ErrBudgetExceeded) == true`; Response is
  non-nil and contains full text; LLMCost counter incremented by
  0.15.

#### NFR-003.2: within cap → no error

- `cfg.PerRequestCapUSD = 0.10`; stub returns cost = 0.05.
- **Then** `err == nil`.

#### NFR-003.3: env override

- `LITELLM_BUDGET_USD=1.00` at Init → `cfg.PerRequestCapUSD == 1.00`
  (overrides default 0.50).

#### NFR-003.4: WARN log on breach

- Breach case emits exactly one WARN slog record with
  `{request_id, provider, model, cost_usd, cap_usd}`.

## 5. Edge Cases

### EC-001 — Rate limit (HTTP 429)

- Stub returns 429 → retried up to 3 times (per retry policy); if all
  retry attempts fail, falls through to next provider per REQ-LLM-004.
- 429 is treated as retryable (transient rate-limit) — distinct from
  401/403/404 which are non-retryable.

### EC-002 — Timeout cancellation mid-retry

- Ctx deadline expires during the second backoff wait → returns
  `context.DeadlineExceeded`; LLMCalls counter incremented with
  `outcome: "timeout"`; OTel span has `Status.Code: Error`.

### EC-003 — Cost header is 0 for Ollama

- Some Ollama configurations omit the cost header → treated as `0.0`;
  no LLMCost increment; DEBUG log once per (provider, model).

### EC-004 — Concurrent breaker access

- Multiple goroutines call `Record(success)` and `Allow()`
  simultaneously on the same breaker.
- The breaker's `sync.Mutex` serialises access; state machine
  transitions are atomic; no race-detector alarms under `-race`.

### EC-005 — Empty model_list at startup

- LiteLLM proxy fails to start; `compose-check.yml` catches the
  failure; Client.New returns `ErrModelNotConfigured` when invoked
  with no priority entries.

### EC-006 — Master-key bytes embedded in response

- The provider's error body literally echoes the master key.
- `TestErrorWrappingRedactsKey` confirms the wrapped error string
  does NOT contain the key.

### EC-007 — Embedder routing

- V1 routes embeddings through LiteLLM (single config file, unified
  cost tracking).
- Future: SPEC-IDX-002 may add a direct route to `services/embedder`
  for high-volume indexing path.

### EC-008 — Cost metric cardinality

- `LLMCost.WithLabelValues(provider, model)` — both labels are
  bounded enums; cardinality ≤ 3 × 15 = 45 series; well within the
  cardinality budget for the lifetime of V1.

## 6. Quality Gate Criteria

| Criterion | Threshold | Source |
|-----------|-----------|--------|
| Coverage (`internal/llm/`) | ≥ 85% (achieved 89.9%) | quality.yaml |
| Coverage (`internal/llm/config/`) | ≥ 85% (achieved 94.7%) | quality.yaml |
| `go vet ./internal/llm/...` | clean | go.md |
| `golangci-lint run` | zero issues | go.md |
| `go test -race ./internal/llm/...` | clean | NFR-LLM-002 |
| `BenchmarkClientCompleteOverhead` p99 | < 10 ms | NFR-LLM-001 |
| `TestNoMasterKeyInLogs` | zero matches | REQ-LLM-005 |
| `TestNoOpenaiGoImportOutsideLLM` | zero violations | REQ-LLM-002 |
| `TestNoUnboundedLabels` | passes (extended allowlist) | REQ-LLM-007 |
| 18 @MX tags across 7 source files | applied | plan.md §8 |
| TRUST 5 gates | all green | constitution |

## 7. Out-of-Scope Confirmations

Restated from spec.md §2.2:

- Per-tenant virtual keys → SPEC-AUTH-002 (M6)
- Pre-flight cost estimation → post-V1 (needs Go tiktoken)
- LiteLLM admin UI / `/spend/logs` aggregation → out of scope
  (we emit our own counters)
- Guardrails, prompt filters, content moderation → SPEC-SEC-001 (M8)
- Prompt caching orchestration → works transparently via LiteLLM
- Asynq task queue → SPEC-DEEP-004 (M5)
- Python-side LiteLLM SDK wiring → owned by each Python service's SPEC
- Tool calls / structured outputs / image-audio modalities → later SPECs
- Hot reload of `config.yaml` → requires LiteLLM restart; future SPEC
- Ollama model auto-discovery → requires explicit `OLLAMA_BASE_URL`

---

*End of acceptance.md (post-hoc).*
