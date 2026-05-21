# SPEC-DEEP-002 Deep Research

Generated: 2026-05-21T00:00:00Z
Author: Explore subagent (Phase 0.5)
Consumed by: manager-spec (Phase 1B), plan-auditor (Phase 2.3)

---

## 1. Architectural Decisions (Pinned — Do Not Re-debate)

1. Orchestration host: NEW Go module `internal/deepagent/` (no new Python sidecar)
2. Pipeline: Sequential Researcher → Reviewer → Writer → Verifier, Writer retried max 2 times when Verifier rejects
3. Mode dispatch: `/deep?mode=agents` (this SPEC) coexists with `/deep?mode=storm` (DEEP-001, already implemented)
4. Retrieval: Reuse existing `internal/fanout` (FAN-001) — Researcher consumes its output, no new retrieval code
5. Per-role model tiering: Researcher=Haiku, Reviewer=Haiku, Writer=Sonnet, Verifier=Sonnet, routed via LiteLLM model aliases (env-var pattern from STORM_MODEL_OUTLINE/STORM_MODEL_ARTICLE)
6. Verifier gate: Reuse `internal/synthesis` SYN-002 faithfulness check (uncited_sentences == 0 = PASS)
7. Streaming: Step-level SSE events (agent_started, agent_completed, retry_started, final_token) layered onto existing `internal/streamsynth` (SYN-004)
8. Cost guards: ONLY Prometheus metrics + hardcoded max_retries=2; per-user quota and daily budget are DEEP-004's responsibility, NOT this SPEC

---

## 2. Existing Code Reuse Map

### 2.1 DEEP-001 STORM Handler Pattern

**Files:**
- `cmd/usearch-api/handlers/synthesis.go` (handler registration stub; full mux registration in SPEC-IR-001)
- `cmd/usearch-api/main.go:38-42` (handler registration point)
- `internal/deepreport/client.go` (HTTP client for sidecar)
- `internal/deepreport/types.go` (request/response payloads)
- `.moai/specs/SPEC-DEEP-001/spec.md` (conventions, env-var patterns)

**Public API Surface:**
- `deepreport.Client.GenerateReport(ctx context.Context, req *deepreport.Request) (*deepreport.Report, error)`
  - Request struct: `RequestID`, `Query`, `Lang`, `Docs: []NormalizedDocPayload`, optional budget/timeout fields
  - Report struct: `RequestID`, `Title`, `Sections: []Section`, `Citations: []Citation`, model/provider/cost metadata, `SchemaVersion: int`
  - Retry semantics: 2 retries with exponential backoff (base 500ms, max 3s jitter), bounded by context deadline
  - Error mapping: `ErrTimeout`, `ErrBudgetExceeded`, `ErrDeadlineExceeded`, `ErrInvalidRequest`, `ErrSidecarUnreachable`

**How DEEP-002 Will Call It:**
- Researcher agent calls `deepreport.Client.GenerateReport()` with initial fanout results
- Verifier inspects report citations against faithfulness criteria (SPEC-SYN-002 gate)
- SSE streamer (DEEP-002) reuses `internal/streamsynth.StreamLongFormReport()` which already walks `Report.Sections` and emits section-aware events

**Quirks/Gotchas:**
- `Report.Sections: []Section` and `Section.Sentences: []Sentence` are already pre-segmented from STORM sidecar; DEEP-002 does not need to re-segment
- Citation markers are 1-indexed numeric references (not doc_id strings); `Report.Citations: []Citation` provides the marker→doc_id mapping
- `Report.SchemaVersion` tracks evolution; DEEP-002 should validate schema compatibility on receipt
- Latency tracking: `Report.LatencyMS` is captured from sidecar response; DEEP-002 streaming events add wall-clock latency on Go-side

---

### 2.2 SYN-002 Faithfulness Verification

**Files:**
- `internal/synthesis/` (does not own faithfulness directly; Python sidecar at `services/researcher/src/researcher/faithfulness.py` implements it)
- `.moai/specs/SPEC-SYN-002/spec.md` (enforcement contract)
- `.moai/specs/SPEC-SYN-002/research.md:§2.2` (enforcement chokepoint at `_process_markers`)

**Public API Surface:**
- Faithfulness check is encapsulated in `services/researcher/src/researcher/synthesis.py:enforce_faithfulness(text, docs, mode, retry_state)`
  - Input: LLM output text, citation markers, docs list
  - Output: `EnforcementOutcome` enum (`accepted`, `retry_required`, `stripped`, `rejected`)
  - HARD requirement: `uncited_sentences == 0` means PASS (no un-cited sentences remain)
  - Python-side, not Go-side API

**How DEEP-002 Will Call It:**
- Verifier agent (Go-side) calls a NEW Go wrapper function `internal/synthesis.CheckFaithfulness(text, citations, docs)` (TBD)
- This wrapper sends the Report to `services/researcher/src/researcher/synthesis.py` endpoint (NEW endpoint) which runs the faithfulness check
- Verifier receives outcome: if uncited_sentences > 0, Verifier returns REJECT, Writer retries (max 2 times)

**Quirks/Gotchas:**
- Faithfulness check is NOT idempotent across retries — each retry re-runs the check on new LLM output
- SPEC-SYN-002 gate operates on text-level claims (sentences), not tokens; Verifier must work at sentence granularity
- Citation markers are stripped by `_process_markers`, so Verifier inspects the FINAL `Report.Sections[].Sentences[].Text` which should have no dangling markers

---

### 2.3 SYN-004 SSE Streamer

**Files:**
- `internal/streamsynth/longform.go` (section-aware event emitter for DEEP-001)
- `internal/sse/writer.go` (thread-safe SSE frame writer)
- `.moai/specs/SPEC-SYN-004/spec.md` (wire format, event taxonomy)

**Public API Surface:**

Event struct (implicit; sent as JSON via SSE writer):
```go
type SSEEvent struct {
    event string   // "section_start", "sentence", "section_done", "done", etc.
    data  string   // JSON-encoded payload
}
```

Frame format per W3C spec (SYN-004 REQ-SYN4-001b):
```
event: <event_type>
data: <json_line_1>
data: <json_line_2>
...
<blank line>
```

Existing event types (SYN-004):
- `section_start: SectionStartPayload` (SectionIndex, Heading, Level, SchemaVersion)
- `sentence: LongFormSentencePayload` (SectionIndex, SentenceIndex, Text, Citations)
- `section_done: SectionDonePayload` (SectionIndex, SentencesEmitted)
- `done: LongFormDonePayload` (TotalSections, TotalSentences, LatencyMS, Model, Provider, CostUSD, SchemaVersion)
- `: ping\n\n` (heartbeat comment, no JSON payload)

Streamer function:
- `streamsynth.StreamLongFormReport(ctx context.Context, w *sse.Writer, report deepreport.Report) (StreamStats, error)`
  - Walks `report.Sections`, respects context cancellation, emits events in order
  - Returns `StreamStats{SentencesEmitted: int}` and error if context cancelled
  - Preserves SYN-002 citation invariant: sentences without citations are skipped

**How DEEP-002 Will Call It:**
- DEEP-002 handler calls `streamsynth.StreamLongFormReport()` for DEEP-001 mode (`/deep?mode=storm`)
- For DEEP-002 mode (`/deep?mode=agents`), handler needs to emit NEW agent-level events BEFORE the report section events
- New events (TBD design): `agent_started`, `agent_completed`, `retry_started`, `error_occurred`

**Quirks/Gotchas:**
- Heartbeat goroutine (`sse.RunHeartbeat()`, not yet found in code) runs independently; DEEP-002 must start it before emitting section events
- SSE writer is thread-safe via `sync.Mutex`; concurrent writes from main thread + heartbeat thread are safe
- Write errors (e.g., client disconnect) are returned; caller must handle and propagate to Prometheus outcome counter
- Flusher interface check in `sse.NewWriter()` ensures HTTP flushing works; nil-flusher systems get no-op Flush()

---

### 2.4 FAN-001 Fanout (Researcher Consumes This)

**Files:**
- `internal/fanout/dispatch.go` (main entry point)
- `internal/fanout/` (full package: `fanout.go`, `dedup.go`, `canonical.go`, etc.)
- `.moai/specs/SPEC-FAN-001/spec.md` (contract)

**Public API Surface:**

```go
type Registry interface {
    Get(name string) (Adapter, error)
    List() []string
    Register(adapter Adapter) error
}

type Dispatch(ctx context.Context, query Query, registry Registry, router *Router) (*Result, error)

type Result struct {
    Docs          []NormalizedDoc           // deduplicated, scored union
    Stats         Stats                     // {AdapterCount, SuccessCount, ErrorCount}
    AdapterErrors map[string]error          // non-nil only when ErrorCount > 0
}
```

**How DEEP-002 Researcher Will Call It:**
- Researcher agent calls `fanout.Dispatch(ctx, query, registry, router)` after receiving user `/deep` request
- Returns `Result.Docs: []NormalizedDoc` (deduplicated union from all adapters)
- Researcher passes `Result.Docs` to STORM sidecar as `deepreport.Request.Docs: []NormalizedDocPayload`

**Quirks/Gotchas:**
- FAN-001 deduplicates by content hash (SourceID + URL + Title + Body), not doc ID; adapters must return canonical URLs (no tracking params) for dedup to work
- `Result.AdapterErrors` is a map only when `ErrorCount > 0`; nil map otherwise (SPEC-FAN-001 H17 fix)
- Per-adapter timeout is `min(perAdapterTimeout, remainingParentDeadline)` (SPEC-FAN-001 §2.5 deriveAdapterCtx)
- Cancellation guard: FAN-001 checks `ctx.Err()` BEFORE every `eg.Go` call (SPEC-FAN-001 H18 fix, REQ-FAN-012)

---

### 2.5 LLM-001 LiteLLM Client (All 4 Agents Route Through This)

**Files:**
- `internal/llm/client.go` (main Client interface and default impl)
- `internal/llm/router.go` (provider router, circuit breaker)
- `internal/llm/config/config.go` (Config struct, env binding)
- `.moai/specs/SPEC-LLM-001/spec.md` (contract)

**Public API Surface:**

```go
type Client interface {
    Complete(ctx context.Context, req Request) (Response, error)
    Stream(ctx context.Context, req Request) (<-chan StreamChunk, error)
    Embed(ctx context.Context, text string) ([]float32, error)
}

type Request struct {
    Class   ModelClass  // "outline", "article", "rag", "ranking", etc.
    Model   string      // "claude-3-5-sonnet", "gpt-4o-mini", "ollama-llama-3.1-70b", etc.
    Messages []any      // OpenAI-compatible message format
}

type Response struct {
    Text            string
    PromptTokens    int
    CompletionTokens int
    CostUSD         float64
    Model           string
    Provider        string
}
```

**Env-Var Conventions for Model Aliases:**

DEEP-001 example (from DEEP-001 research.md):
```
STORM_MODEL_OUTLINE=claude-3-5-sonnet-20241022
STORM_MODEL_ARTICLE=claude-3-5-sonnet-20241022
```

DEEP-002 should follow the same pattern:
```
DEEP_AGENT_RESEARCHER_MODEL=claude-3-5-haiku-20241022
DEEP_AGENT_REVIEWER_MODEL=claude-3-5-haiku-20241022
DEEP_AGENT_WRITER_MODEL=claude-3-5-sonnet-20241022
DEEP_AGENT_VERIFIER_MODEL=claude-3-5-sonnet-20241022
```

Alternative: Model aliases per model-class stored in `deploy/litellm/config.yaml` (SPEC-LLM-001 scope), then Go-side looks up via class:
```go
modelAlias := router.ResolveModelClass(ctx, req.Class)
// modelAlias becomes the final model string for the completion call
```

**How DEEP-002 Agents Will Call It:**
- Each agent (Researcher, Reviewer, Writer, Verifier) calls `llm.Client.Complete()` with a Request specifying the desired Model
- Callers do NOT select provider directly; router handles priority-list and fallthrough
- Retry logic: 3 retries per provider with exponential backoff (250ms → 500ms → 1000ms), then fallthrough to next provider

**Quirks/Gotchas:**
- Auth: `LITELLM_MASTER_KEY` env-var is sent as `Authorization: Bearer <key>` header to proxy (REQ-LLM-005)
- Budget cap: Post-flight check via `x-litellm-response-cost` header; if exceeded, response IS returned (as partial content) but ErrBudgetExceeded is also returned (NFR-LLM-003)
- Circuit breaker: Per-provider failure tracking (50% failure rate over 60s window → half-open, 30s probe); router.Record(provider, success) updates state
- All callers use the SAME `defaultClient` instance (singleton); no per-agent clients

---

### 2.6 NormalizedDoc (Researcher Output, Writer Input)

**Files:**
- `pkg/types/normalized_doc.go` (canonical search-result shape)
- `pkg/types/adapter.go` (Adapter interface that produces NormalizedDoc)
- `.moai/specs/SPEC-CORE-001/spec.md` (contract)

**Fields Relevant to Citation and Streaming:**
```go
type NormalizedDoc struct {
    ID          string         // unique within (SourceID, URL); adapter-assigned
    SourceID    string         // matches Adapter.Name(); used as Prometheus label
    URL         string         // canonical (no tracking params); dedup input
    Title       string         // short text content
    Body        string         // ranking input; not in CanonicalHash
    Snippet     string         // short UI excerpt
    PublishedAt time.Time      // zero when source provides no date
    RetrievedAt time.Time      // when adapter saw this doc (REQUIRED)
    Author      string         // optional
    Score       float64        // normalized [0.0, 1.0]; 0 = unscored (NOT zero engagement)
    Lang        string         // BCP-47; empty = unknown
    Citations   []string       // doc IDs referenced BY this doc (SPEC-SYN-002 uses for per-claim provenance)
    Metadata    map[string]any // adapter-specific extension (NOT in CanonicalHash)
    Hash        string         // cached CanonicalHash() output
}

// Validate returns error for missing: ID, SourceID, URL, RetrievedAt
func (d *NormalizedDoc) Validate() error
```

**How DEEP-002 Will Use It:**
- Fanout.Dispatch() returns `[]NormalizedDoc`
- Researcher marshals to `deepreport.NormalizedDocPayload` (JSON-compatible copy with string timestamps)
- Verifier inspects `Report.Citations: []Citation` (Marker→DocID mapping) against original docs for faithfulness
- Writer receives validated Report with section/sentence structure (NormalizedDoc is consumed, not produced by Writer)

**Quirks/Gotchas:**
- `Citations: []string` field is doc IDs referenced BY the doc (not citations IN the doc); SPEC-SYN-002 uses this for per-claim provenance in synthesis output
- Hash is content-only (SourceID + URL + Title + Body); excludes Metadata so adapter-specific fields don't cause false dedup misses
- Score == 0.0 means unscored, NOT zero engagement; adapters must distinguish (SPEC-CORE-001 note)

---

### 2.7 Obs/Metrics Conventions

**Files:**
- `internal/obs/metrics/deepreport.go` (DEEP-001 metric collectors)
- `internal/obs/metrics/metrics.go` (registry and registration helpers)
- `.moai/specs/SPEC-OBS-001/spec.md` (NFR-OBS-002 cardinality safety)

**Existing Metric Collectors:**
- `usearch_deep_outcomes_total{outcome}` (CounterVec, SPEC-DEEP-001)
  - Label values: `success`, `deadline_exceeded`, `budget_exceeded`, `error_invalid`, `error_upstream`, `error_unresolved_citations_threshold`
  - Cardinality: 6 values (bounded, pre-declared)
- `usearch_deep_latency_seconds` (Histogram, SPEC-DEEP-001)
  - Buckets: [5, 15, 30, 60, 120, 180, 240, 300] seconds (aligned with NFR-DEEP1-001 p50≤180s, p95≤300s)

**How DEEP-002 Extends Metrics:**

New collectors for multi-agent pipeline:
- `usearch_deep_agent_duration_seconds{agent, outcome}` (Histogram)
  - agent label values: `researcher`, `reviewer`, `writer`, `verifier` (4 values, bounded)
  - outcome: `success`, `error`, `timeout`, `retry` (4 values, bounded)
  - Cardinality: 4 × 4 = 16 (safe, pre-declared)

- `usearch_deep_agent_retries_total{agent}` (CounterVec)
  - agent: `writer` (only writer is retried per spec)
  - Cardinality: 1 (safe)

**Cardinality Safety (NFR-OBS-002):**
- All label values MUST be bounded (no user input, no unbounded dimensions)
- Pre-declare all label values in registration code (e.g., `outcomes.WithLabelValues(outcome).Add(0)`)
- Existing allowlist in `internal/obs/metrics/metrics_test.go` allows new label NAME additions only if bounded; DEEP-002 must NOT use unbounded labels

---

## 3. New Components to Build

```
internal/deepagent/
├── orchestrator.go          # Main pipeline orchestration (Researcher → Reviewer → Writer → Verifier)
├── researcher.go            # Query → Fanout → STORM sidecar (calls internal/fanout, internal/deepreport)
├── reviewer.go              # Validates initial report structure (calls internal/llm for optional review step)
├── writer.go                # Refines/improves report (calls internal/deepreport for iterative generation)
├── verifier.go              # Faithfulness gate (calls internal/synthesis for SYN-002 check)
├── config.go                # DEEP_AGENT_* env-var loading
├── types.go                 # Internal types: AgentState, PipelineRequest, PipelineResult
├── metrics.go               # Agent-level metric emission (latency, outcome counters)
└── orchestrator_test.go     # Happy-path, error, timeout, retry scenarios

cmd/usearch-api/handlers/
├── deep_agent_handler.go    # NEW: /deep?mode=agents endpoint (calls deepagent.Orchestrator)
└── (existing deep_storm_handler.go for /deep?mode=storm — unchanged)

internal/streamsynth/
└── agent_events.go          # NEW: Agent-level event payloads (agent_started, agent_completed, retry_started, etc.)
                             # Extends SYN-004 event taxonomy
```

**Responsibility Summary:**
- `deepagent.orchestrator`: Pipelines agents in sequence, retries Writer up to 2 times on Verifier rejection
- `deepagent.researcher`: Calls fanout.Dispatch() for retrieval, marshals to deepreport.Request, awaits Report
- `deepagent.reviewer`: Optional validation step (lightweight agent-driven quality check)
- `deepagent.writer`: Calls deepreport.Client.GenerateReport() with optional refinement prompt
- `deepagent.verifier`: Calls internal/synthesis faithfulness check, returns PASS/REJECT for writer retry decision
- `cmd/usearch-api/handlers/deep_agent_handler`: HTTP handler, SSE emission, context lifecycle
- `internal/streamsynth/agent_events.go`: JSON payloads for new SSE event types

---

## 4. Env-Var Conventions to Add

**Model Aliases (Required):**
```bash
DEEP_AGENT_RESEARCHER_MODEL=claude-3-5-haiku-20241022
DEEP_AGENT_REVIEWER_MODEL=claude-3-5-haiku-20241022
DEEP_AGENT_WRITER_MODEL=claude-3-5-sonnet-20241022
DEEP_AGENT_VERIFIER_MODEL=claude-3-5-sonnet-20241022
```

**Optional Overrides:**
```bash
DEEP_AGENT_MAX_RETRIES=2                           # Default; hardcoded per spec decision #2
DEEP_AGENT_WRITER_RETRY_DELAY_MS=500               # Backoff between retries
DEEP_AGENT_VERIFIER_TIMEOUT_MS=30000               # Faithfulness check timeout
```

**Flags (Optional):**
```bash
DEEP_AGENT_SKIP_REVIEWER=false                     # Skip reviewer step (default: false; reviewer is required per spec)
DEEP_AGENT_SKIP_VERIFIER=false                     # DANGER: Skip verifier gate (default: false; must NOT be skipped in prod)
```

**Follow DEEP-001 Precedent:**
- DEEP-001 uses `STORM_MODEL_OUTLINE`, `STORM_MODEL_ARTICLE` env-vars
- DEEP-002 follows same naming: `DEEP_AGENT_<ROLE>_<SETTING>` where ROLE ∈ {RESEARCHER, REVIEWER, WRITER, VERIFIER}
- Prefix: `DEEP_AGENT_` (not `DEEP_PIPELINE_` or generic `AGENT_`) for consistency with DEEP-001 `STORM_` naming

---

## 5. Metrics to Add

**New Collectors (respecting NFR-OBS-002):**

1. **`usearch_deep_agent_duration_seconds`** (Histogram)
   - Label `agent` (values: `researcher`, `reviewer`, `writer`, `verifier` — 4, bounded)
   - Label `outcome` (values: `success`, `error`, `timeout`, `retry` — 4, bounded)
   - Buckets: [0.5, 1, 2, 5, 10, 30, 60, 120] (agent latency, shorter than end-to-end report)
   - Cardinality: 4 × 4 = 16 (safe, pre-declared)

2. **`usearch_deep_agent_retries_total`** (CounterVec)
   - Label `agent` (values: `writer` — 1, bounded; only writer is retried)
   - Cardinality: 1 (safe, pre-declared)

3. **`usearch_deep_agent_verifier_gate_results_total`** (CounterVec)
   - Label `result` (values: `pass`, `fail_uncited`, `fail_timeout` — 3, bounded)
   - Cardinality: 3 (safe, pre-declared)

**Existing Collectors (Reused, No New Ones):**
- `usearch_deep_outcomes_total{outcome}` (DEEP-001; extends with new outcomes per DEEP-002 errors)
- `usearch_deep_latency_seconds` (DEEP-001; histogram observation at end of pipeline)

**Cardinality Validation:**
- All label values pre-declared in registration code (`.WithLabelValues(agent).Add(0)`)
- No user input, adapter names, or model names in label values
- Allowlist entry in `internal/obs/metrics/metrics_test.go:` for new label NAME only (if new dimension added; value sets are bounded)

---

## 6. SSE Event Types to Add

**Existing Event Types (SYN-004, Emitted by streamsynth.StreamLongFormReport):**
- `section_start: SectionStartPayload`
- `sentence: LongFormSentencePayload`
- `section_done: SectionDonePayload`
- `done: LongFormDonePayload`
- `: ping` (heartbeat comment)

**New Event Types (DEEP-002 Agent Pipeline):**

All JSON payloads, emitted BEFORE the first `section_start` event.

```json
{
  "event": "agent_started",
  "data": {
    "request_id": "string",
    "agent": "researcher|reviewer|writer|verifier",
    "timestamp_ms": "number",
    "schema_version": 1
  }
}
```

```json
{
  "event": "agent_completed",
  "data": {
    "request_id": "string",
    "agent": "researcher|reviewer|writer|verifier",
    "outcome": "success|error|timeout",
    "duration_ms": "number",
    "error_message": "string or null",
    "timestamp_ms": "number",
    "schema_version": 1
  }
}
```

```json
{
  "event": "retry_started",
  "data": {
    "request_id": "string",
    "agent": "writer",
    "retry_count": "number (1 or 2)",
    "reason": "verifier_rejection|timeout|error",
    "timestamp_ms": "number",
    "schema_version": 1
  }
}
```

```json
{
  "event": "verifier_result",
  "data": {
    "request_id": "string",
    "result": "pass|fail_uncited|fail_timeout",
    "details": "string or null",
    "timestamp_ms": "number",
    "schema_version": 1
  }
}
```

**Event Ordering (Temporal Sequence):**
```
agent_started{agent: researcher}
  → agent_completed{agent: researcher, outcome: success}
agent_started{agent: reviewer}
  → agent_completed{agent: reviewer, outcome: success}
agent_started{agent: writer}
  → agent_completed{agent: writer, outcome: success}
agent_started{agent: verifier}
  → verifier_result{result: fail_uncited}
  → retry_started{agent: writer, retry_count: 1}
agent_started{agent: writer}  [2nd attempt]
  → agent_completed{agent: writer, outcome: success}
agent_started{agent: verifier}
  → verifier_result{result: pass}
  → agent_completed{agent: verifier, outcome: success}
section_start{section_index: 0}
  → sentence{section_index: 0, sentence_index: 0, text: "...", citations: [...]}
  → sentence{section_index: 0, sentence_index: 1, ...}
  → ...
  → section_done{section_index: 0}
section_start{section_index: 1}
  → ...
done{total_sections: 3, total_sentences: 42, latency_ms: 145000, cost_usd: 0.18}
```

**Wire Format (W3C SSE):**
Each event is emitted as:
```
event: <event_type>
data: <json_payload>

```

---

## 7. Risks / Open Questions

1. **Verifier SYN-002 Integration (Missing Go-Side Wrapper)**
   - SPEC-SYN-002 faithfulness check is implemented Python-side (`services/researcher/src/researcher/faithfulness.py`)
   - DEEP-002 Verifier (Go-side agent) needs a Go function to call this check
   - **Risk**: No existing Go API; requires NEW endpoint on researcher sidecar or cross-language abstraction
   - **Mitigation**: Define new POST `/faithfulness_check` endpoint on researcher sidecar that Verifier calls, OR wrap the check logic in internal/synthesis
   - **Question**: Should Verifier call a dedicated endpoint or re-use /synthesize with a flag?

2. **Reviewer Role (Spec Decision #2 does not detail reviewer logic)**
   - Spec says: "Sequential Researcher → Reviewer → Writer → Verifier"
   - Reviewer responsibility is not defined: lightweight structural check? LLM-driven quality assessment? Optional step?
   - **Risk**: Implementation may diverge from intent
   - **Mitigation**: Clarify reviewer contract in SPEC EARS requirements (e.g., "REQ-DEEP2-002: Reviewer validates report structure (title, section count) without LLM re-generation")

3. **Context Deadline Propagation Across Agents**
   - Each agent (Researcher, Writer) makes sidecar calls; if parent context is cancelled mid-pipeline, what happens?
   - **Risk**: Partial results, mid-stream cancellation, unclear SSE teardown
   - **Mitigation**: Orchestrator checks `ctx.Err()` before each agent step; partial result is returned with `context.Canceled` error
   - **Question**: Should orchestrator emit a `pipeline_cancelled` event or just let handler close the SSE stream?

4. **Writer Retry Semantics (Spec Decision #2 says "max 2 times" but ambiguous)**
   - Does "2 times" mean 2 total attempts (1 initial + 1 retry) or 2 retries (1 initial + 2 retries)?
   - **Risk**: Off-by-one bugs, inconsistent with DEEP-001 retry patterns
   - **Mitigation**: Clarify in SPEC: "Writer may be retried UP TO 2 times on Verifier rejection (max 3 total attempts: initial + 2 retries)"
   - **Current assumption**: max_retries=2 per hardcoded constant (implies 3 attempts total)

5. **Model Aliasing via LiteLLM vs. Env-Vars**
   - Spec says "LiteLLM model aliases (env-var pattern from STORM_MODEL_OUTLINE/STORM_MODEL_ARTICLE)"
   - Unclear whether to use `deploy/litellm/config.yaml` model-class mapping or direct env-vars
   - **Risk**: Inconsistency with DEEP-001 precedent if approach differs
   - **Mitigation**: Follow DEEP-001 exactly: define DEEP_AGENT_<ROLE>_MODEL env-vars, caller looks up via `os.Getenv()` before calling llm.Client
   - **Question**: Should model alias resolution be centralized in deepagent.config or scattered across agents?

6. **Streaming with Retry (SSE stream restart on Writer retry)**
   - When Writer is retried, SSE stream is already active, sending section events
   - Does retry emit a `section_restart` event or just emit new sections with same index?
   - **Risk**: Client confusion if stream re-emits same section with different content
   - **Mitigation**: DEEP-002 spec should clarify: retry restarts report generation entirely (new Report from sidecar), SSE emits entirely new sections (overwriting previous)
   - **Question**: Or should retry return buffered report and restart streaming, sending ALL sections anew?

7. **Cost Tracking (Spec Decision #8: "hardcoded max_retries=2; cost guards only via metrics")**
   - DEEP-002 does NOT implement per-user quota or daily budget
   - DEEP-004 (M5, future) owns quota enforcement
   - **Risk**: DEEP-002 users can consume unlimited budget via retries
   - **Mitigation**: Document in SPEC that cost guards are a DEEP-004 responsibility; DEEP-002 only emits cost metrics
   - **Question**: Should DEEP-002 still implement a global per-request budget cap (e.g., max 1.00 USD per /deep request)?

8. **Metrics Cardinality for Agent Names**
   - Spec says agent label values: `researcher`, `reviewer`, `writer`, `verifier` (4 bounded values)
   - **Risk**: Future agents added without updating pre-declared set
   - **Mitigation**: Hardcode agent names in deepagent package as constants, enforce via type system (e.g., enum-like type) 

9. **Error Handling in Orchestrator**
   - If Researcher fails (fanout error, sidecar timeout), what does orchestrator return to handler?
   - Should pipeline abort immediately or attempt fallback?
   - **Risk**: Unclear error propagation
   - **Mitigation**: Spec defines: on Researcher error, orchestrator returns error to handler; handler emits error event and closes SSE stream
   - **Question**: Should any agent errors trigger a retry or only Verifier rejection triggers Writer retry?

10. **Streaming Heartbeat Timing**
    - SYN-004 heartbeat is `: ping\n\n` comment every N seconds
    - DEEP-002 adds agent events BEFORE report sections
    - **Risk**: Long delay between agent_completed and first section_start may look like stall
    - **Mitigation**: Ensure heartbeat continues during agent phases (not just during section emission)
    - **Question**: Handler must start heartbeat BEFORE orchestrator begins, not after first agent completes

---

## 8. Reference Implementations

For each new file in §3, closest analog in codebase (pattern-match against):

| New File | Closest Analog | Reference Lines |
|----------|---|---|
| `deepagent/orchestrator.go` | `internal/deepreport/client.go` | [GenerateReport retry loop](file:///Users/masterp/Projects/superwork/universal-search/internal/deepreport/client.go:87-104) — retry orchestration, error mapping |
| `deepagent/researcher.go` | `cmd/usearch/query.go` | Query fanout orchestration pattern (implied by FAN-001 spec) |
| `deepagent/writer.go` | `internal/deepreport/client.go` | [NewClientWithMetrics + doOnce](file:///Users/masterp/Projects/superwork/universal-search/internal/deepreport/client.go:49-75) — HTTP client call, metric emission |
| `deepagent/verifier.go` | `internal/synthesis/client.go` | [Synthesize + emitObs](file:///Users/masterp/Projects/superwork/universal-search/internal/synthesis/client.go:57-80) — call sidecar, observability wrapper |
| `deepagent/metrics.go` | `internal/obs/metrics/deepreport.go` | [registerDeepReport](file:///Users/masterp/Projects/superwork/universal-search/internal/obs/metrics/deepreport.go:43-73) — metric collectors, label pre-declaration |
| `cmd/usearch-api/handlers/deep_agent_handler.go` | `cmd/usearch-api/handlers/synthesis.go` | HTTP handler registration, SSE response setup (stub exists) |
| `internal/streamsynth/agent_events.go` | `internal/streamsynth/longform.go` | [SectionStartPayload + LongFormSentencePayload](file:///Users/masterp/Projects/superwork/universal-search/internal/streamsynth/longform.go:27-68) — event payload structs, JSON tagging |

---

## 9. Sources

### SPEC Files (Reference Implementations)
- `.moai/specs/SPEC-DEEP-001/spec.md` — Long-form report surface, STORM integration contract
- `.moai/specs/SPEC-DEEP-001/research.md` — Sidecar pattern, Python layout precedent
- `.moai/specs/SPEC-LLM-001/spec.md` — LiteLLM client contract, env-var conventions
- `.moai/specs/SPEC-SYN-002/spec.md` — Citation faithfulness enforcement contract
- `.moai/specs/SPEC-SYN-002/research.md` — Faithfulness enforcement chokepoint
- `.moai/specs/SPEC-SYN-004/spec.md` — SSE event types, wire format, heartbeat
- `.moai/specs/SPEC-FAN-001/spec.md` — Fanout dispatch contract, dedup, error taxonomy
- `.moai/specs/SPEC-CORE-001/spec.md` — NormalizedDoc contract, Adapter interface
- `.moai/specs/SPEC-OBS-001/spec.md` — Metrics cardinality safety (NFR-OBS-002)

### Implementation Files (Public APIs)
- `internal/deepreport/client.go` — DeepReport HTTP client, retry loop
- `internal/deepreport/types.go` — Request/Response/Report/Section/Sentence/Citation struct definitions
- `internal/llm/client.go` — LLM Client interface, Complete/Stream/Embed methods
- `internal/llm/router.go` — Provider priority routing, circuit breaker
- `internal/synthesis/client.go` — Synthesis HTTP client pattern, observability wrapper
- `internal/fanout/dispatch.go` — Fanout entry point, per-adapter context derivation, result assembly
- `internal/streamsynth/longform.go` — Section-aware SSE streaming, citation map, cancellation
- `internal/sse/writer.go` — Thread-safe SSE frame writer, W3C wire format
- `internal/obs/metrics/deepreport.go` — Deep report metric collectors, outcome pre-declaration
- `pkg/types/normalized_doc.go` — NormalizedDoc struct, field semantics, Validate method
- `cmd/usearch-api/main.go` — Handler registration point, obs initialization

### Configuration/Environment
- `deploy/docker-compose.yml` — LiteLLM service, researcher service (precedent for STORM service)
- `deploy/litellm/config.yaml` — Model list, routing strategy, fallback chains
- `.env.example` — Env-var documentation (LITELLM_API_KEY, LITELLM_BASE_URL pattern)

---

**End of Research Document**
