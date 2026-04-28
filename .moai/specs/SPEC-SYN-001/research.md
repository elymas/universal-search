# SPEC-SYN-001 Research — Basic synthesis v0

Status: draft companion to spec.md
Created: 2026-04-28
Updated: 2026-04-28
Author: limbowl via manager-spec

This artifact captures the codebase analysis, external library survey, and
contract-design rationale that inform the SPEC-SYN-001 EARS requirements. It
follows the same shape as `.moai/specs/SPEC-LLM-001/research.md` and
`.moai/specs/SPEC-IR-001/research.md`.

---

## 1. Internal Codebase State

### 1.1 `services/researcher/` — Python sidecar scaffold

The Python service directory is currently a minimal scaffold reserved by
SPEC-BOOT-001 and the M2 roadmap entry for SPEC-SYN-001:

```
services/researcher/
├── Dockerfile
├── README.md
├── pyproject.toml
├── src/
│   └── researcher/
│       └── __init__.py        # 8 lines: __version__ + main() stub
└── tests/
    └── test_version.py
```

Key facts captured by `Read`:

- `pyproject.toml` declares `name = "researcher"`, `version = "0.1.0"`,
  `requires-python = ">=3.11"`, `license = "Apache-2.0"`, and an empty
  runtime `dependencies = []`. Dev group has `ruff>=0.8.0`,
  `pytest>=8.3.0`, `pip-audit>=2.7.0`.
- `[tool.hatch.build.targets.wheel] packages = ["src/researcher"]` —
  src-layout is fixed.
- `[tool.pytest.ini_options]` follows MoAI Python rule conventions
  (`tests/`, `test_*.py`, `Test*`, `test_*`).
- `__init__.py` declares `__version__ = "0.1.0"` and a `main()`
  entrypoint stub explicitly tagged `"full implementation lands in
  SPEC-SYN-001"`.
- `Dockerfile` is `python:3.11-slim` based, single-stage (no multi-stage
  yet), runs as `appuser`, `CMD ["python", "-m", "researcher"]`.
- README documents `uv sync --package researcher` and references a
  `.env.example` that does not yet exist for this service.

This means SPEC-SYN-001 is the first SPEC to put runtime code in this
service. There is no prior FastAPI app, no Pydantic models, no HTTP routes.
The Dockerfile's `CMD` target (`python -m researcher`) does not yet
correspond to a runnable server — `main()` is a no-op.

### 1.2 `internal/synthesis/` — Go-side stub

`internal/synthesis/synthesis.go` is a 4-line package stub:

```go
// Package synthesis is the stub for the synthesis layer (gpt-researcher + STORM).
// Full implementation lands in SPEC-SYN-001.
package synthesis
```

No types, no functions. SPEC-SYN-001 is the first SPEC to fill this
package. Mirroring the pattern set by SPEC-IR-001 (which filled the empty
`internal/router/` stub), this SPEC owns the entire initial public API
surface for synthesis.

### 1.3 `pkg/types.NormalizedDoc` — input contract

The synthesis input is `[]NormalizedDoc`, defined and frozen by
SPEC-CORE-001 in `pkg/types/normalized_doc.go`:

| Field        | Type             | Notes                                              |
|--------------|------------------|----------------------------------------------------|
| ID           | string           | Adapter-assigned, unique within `(SourceID, URL)`. |
| SourceID     | string           | Matches `Adapter.Name()`; bounded enum.            |
| URL          | string           | Canonical (tracking-param stripped).               |
| Title        | string           | Display.                                           |
| Body         | string           | Ranking input; full text.                          |
| Snippet      | string           | Short UI excerpt.                                  |
| PublishedAt  | time.Time        | Zero when source has no date.                      |
| RetrievedAt  | time.Time        | Required.                                          |
| Author       | string           | Optional.                                          |
| Score        | float64          | `[0.0, 1.0]`; 0 means unscored.                    |
| Lang         | string           | BCP-47.                                            |
| DocType      | DocType          | Bounded enum (`article`/`post`/...).               |
| Citations    | []string         | Doc IDs referenced by this doc (transitive).       |
| Metadata     | map[string]any   | Adapter-specific bag; NOT in CanonicalHash.        |
| Hash         | string           | First 16 hex chars of canonical SHA-256.           |

`Validate()` is `< 1 µs/op` and only checks `ID`, `SourceID`, `URL`,
`RetrievedAt`. `CanonicalHash()` excludes `Metadata` so adapter-specific
extension keys never affect dedup. Both methods are pure.

The synthesis layer consumes `[]NormalizedDoc` (already deduped + ranked
upstream by SPEC-FAN-001 in M3, and ranked-as-arrived by the M2
single-adapter path). It produces a synthesized paragraph plus citations
keyed by `NormalizedDoc.ID`.

### 1.4 `internal/llm/` — Go LLM gateway (existing, SPEC-LLM-001)

The Go-side LLM client is fully implemented and merged. Salient details:

- Public surface: `llm.Client{Complete, Stream, Embed, Close}` plus
  value types `Request`, `Response`, `Delta`, `EmbedRequest`,
  `EmbedResponse`, and `ModelClass` constants
  (`DeepResearch`, `Summary`, `Classify`, `Embed`).
- Talks to LiteLLM proxy at `http://localhost:4000` by default,
  configurable via `LITELLM_BASE_URL` / `LITELLM_MASTER_KEY`
  (`internal/llm/config/config.go`).
- Per-call observability: 1 slog record + 1
  `obs.LLMCalls{provider,model,outcome}` counter increment + 1
  `obs.LLMLatency{provider,model}` histogram observation + 1
  `usearch_llm_cost_usd_total{provider,model}` increment + 1
  OTel `llm.call` span (`client.go:230-252`).
- Retry-then-fallthrough with circuit breaker per provider; budget cap
  via `ErrBudgetExceeded`; auth via `Authorization: Bearer
  $LITELLM_MASTER_KEY`.
- 18 `@MX` tags applied; coverage 89.9% / 94.7%.

Importantly, the Go LLM client is the single Go-side path for LLM calls;
**Python services do NOT route through `internal/llm` and instead call
the same LiteLLM proxy via their own Python SDK** (per SPEC-LLM-001 §2.2
Out-of-Scope). SPEC-SYN-001's Python sidecar is the first concrete
example of this dual-language pattern.

### 1.5 `internal/obs/` — Observability baseline (existing, SPEC-OBS-001)

Contract surface relevant to synthesis:

- `obs.Logger(ctx)` for slog records with auto-injected `request_id`,
  `trace_id`, `span_id`.
- `obs.Tracer("synthesis")` for OTel spans.
- Named Prometheus collectors live in `internal/obs/metrics/`. New metric
  families (e.g., `usearch_synthesis_*`) must be declared there to
  preserve the SPEC-OBS-001 import-boundary test (`TestNoUnboundedLabels`,
  `TestNoSensitiveDataInLabels`).
- Bounded label discipline: only `provider`, `model`, `outcome`, and a
  small set of pre-allowlisted names. New label names require an
  explicit allowlist amendment (see SPEC-LLM-001 REQ-LLM-007).

Existing metric naming convention (verified by Grep):

```
usearch_http_requests_total
usearch_http_request_duration_seconds
usearch_fanout_goroutines_inflight
usearch_adapter_calls_total
usearch_adapter_call_duration_seconds
usearch_llm_calls_total
usearch_llm_cost_usd_total
usearch_llm_latency_seconds
usearch_router_classifications_total
usearch_router_classification_duration_seconds
usearch_build_info
```

SPEC-SYN-001 will add the family `usearch_synthesis_*` (see §3.5).

### 1.6 `internal/router/` — Intent Router (existing, SPEC-IR-001)

Not a direct dependency of synthesis, but its `RoutingDecision` shape
is the upstream caller pattern: a Go struct with `Source`, `Confidence`,
and `Metadata map[string]any`. SPEC-SYN-001 mirrors this stylistically
for `SynthesisResult` (a Go-side response value).

### 1.7 `.moai/project/roadmap.md` §M2 — Exit criterion

The M2 milestone exit criterion is verbatim:

> `usearch query "hello world"` returns Reddit + HN results with one
> synthesized paragraph + citations.

SPEC-SYN-001 is the SPEC that delivers the **synthesized paragraph +
citations** half of that criterion. ADP-001 (Reddit) and a future
ADP-002 (HN) deliver the **results** half. CLI-001 (the `usearch
query` CLI command) wires them together.

This means SPEC-SYN-001 is on the M2 critical path — without it, M2 does
not exit.

---

## 2. External Library Survey

### 2.1 gpt-researcher (`/assafelovic/gpt-researcher`, Apache-2.0)

The reference implementation we wrap. Architecture per WebFetch
(2026-04-28) and the existing notes in
`.moai/specs/SPEC-IR-001/research.md` §2.1 + `SPEC-ADP-001/research.md`
§5.3:

- **Pattern**: planner-executor-publisher. The planner LLM call
  decomposes a query into sub-queries; executor agents fetch documents;
  the publisher LLM call summarizes with citations.
- **Module map**: `gpt_researcher/` (main package), `backend/` (server),
  `multi_agents/` (LangGraph), `mcp-server/` (MCP integration).
- **Single-pass entry point**:

  ```python
  from gpt_researcher import GPTResearcher
  r = GPTResearcher(query=...)
  await r.conduct_research()
  report = await r.write_report()
  ```

- **Local-document mode**: setting `report_source="local"` and providing
  `DOC_PATH` allows summarization of pre-fetched documents instead of
  web retrieval. This is the mode SPEC-SYN-001 targets — adapters in the
  Go orchestration plane have already fetched the documents, so
  gpt-researcher must NOT do its own retrieval.
- **LLM configuration**: `OPENAI_BASE_URL` env var routes to any
  OpenAI-compatible endpoint. Setting it to the LiteLLM proxy
  (`http://localhost:4000`) routes all LLM traffic through our existing
  cost tracking and provider routing.

### 2.2 STORM (`/stanford-oval/storm`)

Out of scope for SPEC-SYN-001 (deferred to SPEC-DEEP-* in M5). STORM's
multi-perspective conversational retrieval is materially different from
single-pass summarization and adds operational complexity (multi-step
LLM workflows, perspective generation, refinement loops). Mentioned here
only to document the explicit deferral.

### 2.3 LangChain `RetrievalQA` / `create_stuff_documents_chain`

A common citation-assembly pattern in Python ecosystems:

1. Build a prompt that instructs the LLM to cite sources by `[N]`
   numeric markers.
2. Embed each source in the prompt with a `Source [N]:` prefix.
3. Post-process the LLM output to verify markers exist and map them to
   source URLs.

SPEC-SYN-001 adopts this pattern directly. It is well-understood and
does not require structured output (tool use) — a constrained JSON
response with a list of citation indices keyed to input doc IDs is
sufficient.

### 2.4 FastAPI 0.115+ patterns

Per `.claude/rules/moai/languages/python.md`:

- Async endpoints with `Depends` for DI.
- `lifespan` async context manager for startup/shutdown — required for
  initializing the gpt-researcher singleton or LLM SDK client at app
  start instead of per-request.
- Pydantic v2.9 for request/response with `ConfigDict(extra="forbid",
  str_strip_whitespace=True)` and `model_validator(mode="after")` for
  cross-field validation.

### 2.5 OpenAI Python SDK (used as gpt-researcher's transport)

gpt-researcher's runtime LLM transport is OpenAI Python SDK keyed on
`OPENAI_BASE_URL` and `OPENAI_API_KEY`. Setting
`OPENAI_BASE_URL=http://litellm:4000` (or the host equivalent) and
`OPENAI_API_KEY=$LITELLM_MASTER_KEY` routes all traffic through our
LiteLLM proxy without modifying gpt-researcher source.

---

## 3. Contract Surface — Go ↔ Python sidecar

### 3.1 Why HTTP, not in-process Go LLM call

A reasonable alternative would be: skip the Python sidecar, do
single-pass synthesis directly in Go via `internal/llm.Client.Complete`
with `Class: llm.Summary`. Considered and rejected for these reasons:

1. **gpt-researcher reuse**. The Python library encodes
   citation-assembly heuristics (source numbering, prompt scaffolds,
   citation verification) that have been refined across thousands of
   community uses. Reimplementing them in Go for V1 would duplicate
   effort with no measured win.
2. **M5 reuse**. STORM (SPEC-DEEP-001) and the deep-research multi-agent
   pipeline (SPEC-DEEP-002) are Python-native libraries with no Go
   equivalent. The Python sidecar pattern established in SPEC-SYN-001
   becomes the host for those workloads in M5. Building the sidecar
   pattern now avoids re-architecting in M5.
3. **Roadmap alignment**. `.moai/project/roadmap.md` M2 row explicitly
   names `services/researcher` as the synthesis owner and Go-side
   `internal/synthesis/` as the orchestration consumer, not the
   producer.

### 3.2 Why FastAPI, not gRPC / connect-go

- **Simplicity**. FastAPI + Pydantic is the documented MoAI Python
  stack (see `.claude/rules/moai/languages/python.md` §FastAPI 0.115+).
- **Single internal consumer**. Only `internal/synthesis/client.go` calls
  this endpoint. The Go side already has a generic JSON HTTP client
  pattern from SPEC-LLM-001; reusing it here costs no new dependencies.
- **No streaming yet**. SPEC-SYN-004 (M4) introduces streaming.
  Plain JSON POST is sufficient for V1.
- **Tracing**. FastAPI receives `traceparent` headers via the
  `X-Request-ID` / W3C TraceContext middleware that SPEC-OBS-001
  established. Cross-service trace propagation works out of the box.

### 3.3 Endpoint shape (POST `/synthesize`)

Request schema:

```json
{
  "request_id": "01J...",            // ULID, propagated from Go side
  "query": "hello world",            // raw query text
  "lang": "en",                       // BCP-47 hint (from RoutingDecision.Lang)
  "docs": [
    {
      "id": "reddit:t3_abc123",
      "source_id": "reddit",
      "url": "https://reddit.com/...",
      "title": "...",
      "body": "...",
      "snippet": "...",
      "published_at": "2026-04-25T12:00:00Z",
      "score": 0.42,
      "lang": "en"
    }
  ]
}
```

Response schema:

```json
{
  "request_id": "01J...",
  "text": "Rust async runtimes have evolved [1] alongside ... [2].",
  "citations": [
    { "marker": 1, "doc_id": "reddit:t3_abc123",
      "url": "https://reddit.com/...", "title": "..." },
    { "marker": 2, "doc_id": "hn:42312345",
      "url": "https://news.ycombinator.com/item?id=42312345", "title": "..." }
  ],
  "model": "claude-haiku-4-5",
  "provider": "anthropic",
  "cost_usd": 0.0023,
  "prompt_tokens": 1245,
  "completion_tokens": 187,
  "latency_ms": 2104,
  "degraded": false,
  "notice": ""
}
```

Status codes:

- `200` — success (including degraded mode).
- `400` — `query` empty or `docs` missing required NormalizedDoc fields
  (Validate-equivalent check Python-side).
- `503` — LiteLLM proxy unreachable for longer than the request budget;
  body still includes `degraded=true` and a synthesized text that is
  literally the bullet-list of doc titles + URLs (graceful degradation
  per NFR-SYN-003).
- `504` — request exceeded server-side hard timeout (10 s default).

### 3.4 Why citation markers are integers, not doc IDs inline

Two choices were considered:

A. Inline: `"... [reddit:t3_abc123]..."`
B. Numeric: `"... [1] ..."` plus a separate citations list.

Choice B (numeric) wins for these reasons:

- **LLM compliance**. LLMs reliably emit short integer markers; doc IDs
  with colons and underscores are frequently mangled or hallucinated.
- **Display ergonomics**. CLI rendering can render `[1]` as a footnote
  marker without parsing complex IDs.
- **Hallucination defense**. SPEC-SYN-002 (citation faithfulness) has a
  cleaner job verifying `marker → doc_id` table consistency than parsing
  inline IDs from prose.

### 3.5 Observability surface (per call)

The Python service emits:

- `obs_log.info("synthesis call", request_id=..., query_len=...,
  docs_count=..., model=..., cost_usd=..., latency_ms=...,
  degraded=..., outcome=...)` as JSON to stdout (slog-equivalent for
  Python, picked up by Promtail / future Loki sink).
- The Go-side caller emits its own per-call observability via
  `internal/synthesis/client.go`:
  - `obs.SynthesisCalls.WithLabelValues(outcome).Inc()` —
    counter, label `outcome ∈ {success, degraded, error_invalid,
    error_timeout, error_unreachable}`.
  - `obs.SynthesisLatency.WithLabelValues(outcome).Observe(seconds)` —
    histogram.
  - `obs.SynthesisCost.Add(cost_usd)` — counter, no labels (cost is
    already attributed by provider/model on the LiteLLM-side LLM
    counter; this is a synthesis-domain rollup).
  - 1 OTel span `synthesis.call`.
  - 1 slog record at INFO (success/degraded) or WARN (error_*) with
    attributes `{request_id, query_len, docs_count, model, cost_usd,
    latency_ms, degraded, outcome}`.

New metric families introduced (subject to allowlist amendment in run
phase):

```
usearch_synthesis_calls_total{outcome}
usearch_synthesis_latency_seconds{outcome}
usearch_synthesis_cost_usd
```

Label names: `outcome`. No new label names are introduced beyond those
already allowlisted by SPEC-OBS-001 + SPEC-IR-001.

### 3.6 Failure modes and degradation

| Failure                                | Detection                              | Response                                                   |
|----------------------------------------|----------------------------------------|------------------------------------------------------------|
| LiteLLM proxy unreachable              | HTTP error at OpenAI SDK layer         | `degraded=true`, `text = bulletList(docs)`, 503 status     |
| LLM timeout (per-request)              | `httpx.ReadTimeout` / asyncio cancel   | `degraded=true`, `text = bulletList(docs)`, 504 status     |
| Zero docs in input                     | Pydantic validator                     | 400 with `notice="no documents to synthesize"`             |
| LLM emits markers not in input         | Citation validator                     | Drop those markers, log WARN, continue (faithfulness in 002)|
| Python sidecar unreachable from Go     | Go HTTP client error                   | Go-side returns `synthesis.ErrSidecarUnreachable`; CLI prints raw doc list |

The **partial-results case** — some adapters returned, some failed —
is not synthesis's concern. Upstream (SPEC-FAN-001 in M3, single-adapter
in M2) decides what `[]NormalizedDoc` to forward. Synthesis sees
whatever it sees.

### 3.7 Environment variables consumed

Python sidecar reads:

| Env var              | Default                  | Purpose                                            |
|----------------------|--------------------------|----------------------------------------------------|
| `LITELLM_BASE_URL`   | `http://litellm:4000`    | OpenAI-compatible base URL (set on OpenAI SDK).    |
| `LITELLM_API_KEY`    | (unset)                  | Bearer token; same value as `LITELLM_MASTER_KEY` Go-side. |
| `RESEARCHER_PORT`    | `8081`                   | FastAPI bind port.                                 |
| `RESEARCHER_LOG_LEVEL` | `INFO`                 | Python logging level.                              |
| `RESEARCHER_MODEL_DEFAULT` | `claude-haiku-4-5` | Default LiteLLM model alias for synthesis.         |
| `RESEARCHER_TIMEOUT_SECONDS` | `8`              | Per-LLM-call timeout inside the service.           |

Go-side `internal/synthesis/client.go` reads:

| Env var              | Default                  | Purpose                                            |
|----------------------|--------------------------|----------------------------------------------------|
| `RESEARCHER_BASE_URL` | `http://localhost:8081` | HTTP endpoint for the Python sidecar.              |
| `RESEARCHER_REQUEST_TIMEOUT_SECONDS` | `10`     | Outer wall-clock budget on Go side.                |

Note the deliberate naming distinction: `LITELLM_*` are LLM-proxy
variables already established by SPEC-LLM-001; `RESEARCHER_*` are
introduced by this SPEC. We do not reuse `LITELLM_BASE_URL` for the
Python sidecar address — they point at different services.

### 3.8 Distinction between API keys

- `LITELLM_MASTER_KEY` (Go-side, established by SPEC-LLM-001) — Bearer
  token to LiteLLM proxy from Go.
- `LITELLM_API_KEY` (Python-side, this SPEC) — same value, named per
  OpenAI SDK convention so gpt-researcher works without monkey-patching.

In practice, both env vars read from the same secret in the deployed
environment. Documented in `.env.example` (Python sidecar) as a
reference forward to `LITELLM_MASTER_KEY`.

---

## 4. Open Questions for Plan Phase

1. **gpt-researcher dependency size.** The full `gpt-researcher` Python
   package pulls a large transitive dependency tree (LangChain,
   tiktoken, Selenium for browsing, etc.). For single-pass local-doc
   synthesis we may need only a subset. Question: install full
   `gpt-researcher` and live with the image size, or extract just the
   prompt scaffolds and reimplement the citation loop in ~150 LoC of
   Python? Default: install full package; revisit in M4 if Docker image
   exceeds 2 GB.
2. **Default synthesis model.** Haiku is cheap and meets 8s p50 (per
   NFR-SYN-001) on 10-doc input; Sonnet is more faithful but 4x cost.
   V1 default: Haiku. Per-request override via a `model` field in the
   request payload (defaulting to env). SPEC-SYN-002 will measure and
   may bump to Sonnet for citation faithfulness.
3. **Embedding-based reranking before LLM call.** Putting all 10 docs
   into the prompt at full body costs ~5K-10K tokens. Truncation
   strategy: take `Snippet` not `Body` for V1, fall back to truncated
   `Body[:1000]` if Snippet empty. SPEC-SYN-003 (M4) introduces proper
   chunking and embedding-based selection.
4. **Citation marker collisions across multiple synthesis calls.** Each
   request is independent; markers `[1]..[N]` are local to that call.
   No cross-call mapping is needed in V1.
5. **Retries on Go side vs. Python side.** Go-side outer client has its
   own retry policy (3 attempts, exponential backoff). Python side does
   NOT retry the LLM call — it relies on LiteLLM's internal retry
   (already configured in `deploy/litellm/config.yaml` per SPEC-LLM-001
   REQ-LLM-001 `router_settings.num_retries >= 3`). Avoids retry
   amplification.
6. **Health endpoint.** A `GET /health` endpoint already exists in spirit
   (the README mentions it); SPEC-SYN-001 must declare it explicitly
   so docker-compose healthchecks work. Returns `{"status": "ok",
   "version": researcher.__version__}` plus `503` when the upstream
   LiteLLM proxy ping fails.
7. **Streaming.** Explicitly deferred to SPEC-SYN-004 (M4). The V1
   contract is request/response JSON; the streaming SSE variant in
   SPEC-SYN-004 will be additive (a new endpoint `POST /synthesize/stream`
   that does not break V1 callers).
8. **Token budget per request.** Hard cap at the Go-side budget cap
   (`LITELLM_BUDGET_USD`, default 0.50) inherited transitively because
   the LiteLLM proxy itself enforces it. Synthesis does not need its own
   cost cap in V1.

---

## 5. References

Internal:

1. `services/researcher/` — Python scaffold (`pyproject.toml`,
   `src/researcher/__init__.py`, `Dockerfile`, `README.md`).
2. `internal/synthesis/synthesis.go` — Go-side stub (4 lines).
3. `pkg/types/normalized_doc.go` — `NormalizedDoc` 15-field type.
4. `internal/llm/` — Go LLM client + LiteLLM config (SPEC-LLM-001).
5. `internal/llm/config/config.go` — env binding pattern.
6. `internal/obs/metrics/` — Prometheus collector layout
   (existing `usearch_*` families).
7. `.moai/specs/SPEC-CORE-001/spec-compact.md` §Synthesis consumption
   note → SPEC-SYN-001, SPEC-SYN-002.
8. `.moai/specs/SPEC-LLM-001/spec.md` §2.2 Out-of-Scope (Python services
   route through LiteLLM directly, not `internal/llm`).
9. `.moai/specs/SPEC-IR-001/spec.md` §2.2 (`services/researcher` for
   synthesis, not classification).
10. `.moai/specs/SPEC-OBS-001/spec.md` §2.1 metric naming + label
    cardinality discipline.
11. `.moai/project/roadmap.md` §M2 exit criterion (synthesized paragraph
    + citations).

External (verified URLs):

12. `https://github.com/assafelovic/gpt-researcher` — Apache-2.0
    library; planner/executor/publisher pattern; local-doc mode via
    `report_source="local"` + `DOC_PATH`; OpenAI-compatible LLM via
    `OPENAI_BASE_URL`. Verified via WebFetch 2026-04-28.
13. FastAPI 0.115+ patterns — `tiangolo/fastapi`, per
    `.claude/rules/moai/languages/python.md`. Lifespan context manager
    + Pydantic v2.9 `ConfigDict(extra="forbid")`.

Excluded / deferred:

14. STORM (`stanford-oval/storm`) — multi-perspective deep research;
    SPEC-DEEP-001 (M5).
15. LangGraph — multi-agent workflows; SPEC-DEEP-002 (M5).

---

*End of SPEC-SYN-001 research v0*
