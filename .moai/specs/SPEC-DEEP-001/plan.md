# SPEC-DEEP-001 Implementation Plan

Companion artifact for `.moai/specs/SPEC-DEEP-001/spec.md`.
Version: 0.3.1
Created: 2026-05-10
Author: limbowl (via manager-spec)

## HISTORY (mirror)

- 2026-05-10 (v0.3.1, iter-3 cleanup): plan-auditor APPROVE-WITH-CHANGES
  (0.82). Applied 5 small fixes per spec.md HISTORY v0.3.1 entry: N1
  (docker-compose `depends_on` map form), N2 (this version bump + HISTORY
  mirror), N3 (acceptance.md version bump), N4 (spec-compact.md version
  field), N5 (REQ-LLM-005 quote ellipsis for omitted secret-redaction
  clause). Status transitioned `draft → approved`. No content changes
  beyond the five enumerated fixes.

---

## 1. Overview

SPEC-DEEP-001 introduces the long-form report generation surface.
It adds a NEW Python sidecar (`services/storm/`) wrapping the
upstream `stanford-oval/storm` library, a NEW Go-side HTTP client
(`internal/deepreport/`), an extended SSE streamer
(`internal/streamsynth/longform.go`), an HTTP handler at
`cmd/usearch-api`, and two new Prometheus collectors at
`internal/obs/metrics/deepreport.go`.

Methodology: **TDD (RED-GREEN-REFACTOR)** per
`.moai/config/sections/quality.yaml` `development_mode: tdd`.
Coverage target: 85% (matches project default).

Harness level: **thorough**. The change crosses three trust
boundaries (LLM-output → faithfulness gate → SSE wire), introduces
a new external dependency (`knowledge-storm`), and is the
foundational SPEC for M5 — Sprint Contract Protocol per
`.claude/rules/moai/design/constitution.md` §11 is required.

This SPEC is a **greenfield service** (Python sidecar) plus a
**brownfield extension** (Go-side streaming + observability).
File-impact split across the two reflects this.

---

## 2. Milestones (priority-based, no time estimates)

### Milestone 1 [Priority High] — Sidecar skeleton + LiteLLM gateway

Goal: a runnable `services/storm/` sidecar that can accept a
`/generate_report` request, configure STORM's LM configs to point
at LiteLLM, and return a stub response. No real STORM invocation
yet.

Deliverables:
- [MODIFY] `services/storm/pyproject.toml` — add real dep set:
  `knowledge-storm == 1.1.1` (verified via WebFetch 2026-05-10),
  `dspy-ai` (transitive range from knowledge-storm), `fastapi`,
  `uvicorn[standard]`, `httpx`, `pydantic>=2`. Python requirement
  `>=3.10` per knowledge-storm v1.1.1 metadata.
- [NEW] `services/storm/src/storm/__main__.py` — uvicorn entry
  point.
- [NEW] `services/storm/src/storm/app.py` — FastAPI app factory,
  routes for `/health`, `/readyz`, `/generate_report`.
- [NEW] `services/storm/src/storm/models.py` — Pydantic request +
  response shapes including `Section`, `Sentence`, `Citation`.
- [NEW] `services/storm/src/storm/gateway.py` — LiteLLM-rooted
  `dspy.LM` (or `knowledge_storm.lm.LitellmModel`, see §5.4) factory.
  Reads `LITELLM_BASE_URL` (matches researcher gateway.py:26 verbatim
  literal `base_url = os.environ.get("LITELLM_BASE_URL", "http://litellm:4000")`)
  and `LITELLM_API_KEY` for the in-container API key (matches
  researcher gateway.py:27 verbatim literal
  `api_key = os.environ.get("LITELLM_API_KEY", "")` — the Python
  sidecar reads `LITELLM_API_KEY`, NOT `LITELLM_MASTER_KEY` directly;
  the host's `${LITELLM_MASTER_KEY}` is aliased into the container's
  `LITELLM_API_KEY` by `deploy/docker-compose.yml:173`); the master
  key value is then sent upstream as Bearer auth to the LiteLLM proxy
  consistent with SPEC-LLM-001 REQ-LLM-005's intent (REQ-LLM-005's
  literal env-var name `LITELLM_MASTER_KEY` governs the Go-side
  `internal/llm` Client, not Python sidecars). Also reads
  `STORM_MODEL_OUTLINE`, `STORM_MODEL_ARTICLE`.
- [NEW] `services/storm/src/storm/obs.py` — JSON log emission +
  outcome counter dispatcher (Python side; counter actually held
  Go-side via Prometheus scrape, sidecar exposes `/metrics`).
- [NEW] `services/storm/Dockerfile` — multi-stage Python 3.11
  slim, uv-based install, uvicorn boot.
- [MODIFY] `services/storm/.env.example` — add all `STORM_*`
  knobs documented in spec.md §2.1(g).
- [NEW] `services/storm/tests/test_app.py` — `/health` and
  `/readyz` GET tests (HTTP-level; mocked LiteLLM ping).
- [NEW] `services/storm/tests/test_gateway.py` — LiteLLM
  config-factory tests; assert no direct vendor SDK imports.
- Stub `/generate_report` returns
  `{request_id, title: "stub", sections: [], citations: [],
  schema_version: 1}` — pure mechanical happy-path.

Exit criterion: `uv run --directory services/storm pytest` green;
`docker compose up storm` starts and `curl localhost:8001/health`
returns `{"status":"ok"}` (port 8001 is the container internal port,
mirroring the researcher same-port convention `${RESEARCHER_PORT:-8081}:8081`
at `deploy/docker-compose.yml:170`; see M9 for the
`${STORM_PORT:-8001}:8001` mapping).

### Milestone 2 [Priority High] — Injected retrieval module + STORM invocation

Goal: real STORM pipeline invocation backed by the request-payload
`docs[]` array (no external retrieval).

Deliverables:
- [NEW] `services/storm/src/storm/inject_rm.py` — custom
  retrieval module mirroring the
  `from knowledge_storm.rm import YouRM` interface (verified
  2026-05-10 via WebFetch on upstream README). Public surface:
  `InjectedRM(docs: list[NormalizedDocPayload], top_k: int)`.
  `forward(query: str, k: int) -> list[dict[str, str]]` returns
  top-k `{url, title, snippets, body}` items selected by simple
  lexical scoring against the input docs.
- [NEW] `services/storm/src/storm/pipeline.py` — orchestration
  using the verified API surface (WebFetch 2026-05-10):
  `from knowledge_storm import STORMWikiRunner, STORMWikiLMConfigs,
  STORMWikiRunnerArguments` and
  `from knowledge_storm.lm import LitellmModel`. Builds
  `STORMWikiLMConfigs` via gateway-supplied `LitellmModel` instances
  (`set_conv_simulator_lm`, `set_article_gen_lm`), builds
  `STORMWikiRunnerArguments` from env-var knobs (`search_top_k`,
  `max_perspectives`, etc.), builds `InjectedRM(req.docs,
  top_k=args.search_top_k)`, invokes `STORMWikiRunner(args,
  lm_configs, rm).run(topic=req.query, do_research=True,
  do_generate_article=True)`, captures the output article +
  references via `runner.summary()`.
- [MODIFY] `services/storm/src/storm/app.py:/generate_report` —
  wire pipeline.py.
- [NEW] `services/storm/tests/test_inject_rm.py` — unit tests for
  the retrieval module (lexical match, top-k slicing).
- [NEW] `services/storm/tests/test_pipeline.py` — integration
  tests with `dspy.LM` mocked; assert `STORMWikiRunner` is invoked
  with our LM configs and our RM; assert the runner's output is
  captured into the response shape.

Exit criterion: a single `/generate_report` call produces a
report shape with at least one section (mocked LM produces
deterministic stub output); LiteLLM is the only LLM access path
(verified by import-graph test).

### Milestone 3 [Priority High] — Citation translator (URL → doc_id)

Goal: STORM's URL-cited inline `[N]` markers translated to our
`doc_id`-cited markers, with the unresolved-citation counter.

Deliverables:
- [NEW] `services/storm/src/storm/citation_translator.py` — URL
  canonicalization + marker translation. Public surface:
  `canonicalize_url(url: str) -> str` (strip query, lowercase
  host, normalize protocol, strip trailing slash) and
  `translate(text: str, storm_refs: list[StormRef], docs:
  list[NormalizedDocPayload]) -> tuple[str, list[Citation], int]`.
- [MODIFY] `services/storm/src/storm/pipeline.py` — invoke
  translator after STORM run; populate `response.citations`.
- [NEW] `services/storm/tests/test_citation_translator.py` —
  parameterized tests over the 8 protocol/query/trailing-slash
  combinations; unresolved-marker stripping; counter assertions.
- [NEW] Counter `usearch_storm_unresolved_citations_total`
  emitted via Python `obs.py` (Go-side reads via /metrics scrape).

Exit criterion: REQ-DEEP1-002 acceptance tests green;
counter visible in Python sidecar `/metrics` output.

### Milestone 4 [Priority High] — Faithfulness gate (long-form variant)

Goal: per-section citation faithfulness gate matching SPEC-SYN-002
contract; un-cited sentences stripped (default), rejected, or
bypassed per `STORM_FAITHFULNESS_MODE`.

Deliverables:
- [NEW] `services/storm/src/storm/faithfulness.py` — DEEP-001-owned
  long-form citation gate. Declares the canonical sentence regex
  literally (`[.!?。！？]\s+|[.!?。！？]$`) as a fresh Python-side
  literal — single source of truth on the Python side (there is no
  `services/researcher/src/researcher/faithfulness.py` to import
  from; the SYN-002 Python-side gate was descoped at build time and
  the only surviving SYN-002 reference is the Go-side
  `internal/streamsynth/streamsynth.go:28` regex). Public surface:
  `enforce_long_form_faithfulness(sections: list[Section], docs:
  list[NormalizedDocPayload], mode: str) -> tuple[list[Section],
  EnforcementOutcome, int, int]`.
- [MODIFY] `services/storm/src/storm/pipeline.py` — invoke gate
  after citation translation; remove empty sections; set
  `notice` field accordingly.
- [MODIFY] `services/storm/src/storm/app.py` — register
  `UncitedLongFormError` exception handler returning HTTP 422.
- [NEW] `services/storm/tests/test_faithfulness.py` — long-form
  variant tests (parallel to researcher's
  `test_faithfulness.py`); modes strip / reject / off; empty
  section removal.
- New counter `usearch_storm_faithfulness_outcomes_total{outcome}`
  with 4 label values per spec.md REQ-DEEP1-003.

Exit criterion: REQ-DEEP1-003 acceptance tests green;
faithfulness modes verified end-to-end via HTTP tests.

### Milestone 5 [Priority High] — Latency + budget caps

Goal: hard ctx deadline at `STORM_MAX_LATENCY_MS` and cumulative
cost cap at `STORM_MAX_COST_USD`; clean cancellation paths;
structured error responses.

Deliverables:
- [MODIFY] `services/storm/src/storm/pipeline.py` — wrap STORM
  invocation in `asyncio.wait_for(... , timeout=...)`. On
  `asyncio.TimeoutError`, raise `DeadlineExceededError` carrying
  partial-section count.
- [MODIFY] `services/storm/src/storm/gateway.py` — accumulate
  `cost_usd` per LiteLLM call; on exceeding `STORM_MAX_COST_USD`,
  raise `BudgetExceededError`.
- [MODIFY] `services/storm/src/storm/app.py` — exception handlers
  returning 504 (deadline) and 402 (budget) with structured body
  schemas matching spec.md REQ-DEEP1-004.
- [MODIFY] `services/storm/src/storm/obs.py` — single-emission
  `emit_outcome(outcome)` guarded by request-scoped flag for
  NFR-DEEP1-003 invariant.
- [NEW] `services/storm/tests/test_caps.py` — deadline + budget
  scenarios; race-window tests.

Exit criterion: REQ-DEEP1-004 acceptance tests green;
NFR-DEEP1-003 race tests green.

### Milestone 6 [Priority High] — Go-side client + observability

Goal: `internal/deepreport/` package consumes the sidecar via
HTTP; observability collectors registered.

Deliverables:
- [NEW] `internal/deepreport/types.go` — `Request`, `Report`,
  `Section`, `Sentence`, `Citation` Go shapes (mirrors Pydantic).
  Error sentinels: `ErrInvalidRequest`, `ErrSidecarUnreachable`,
  `ErrTimeout`, `ErrBudgetExceeded`, `ErrDeadlineExceeded`.
- [NEW] `internal/deepreport/client.go` — `Client.GenerateReport(ctx,
  req) (Report, error)` mirrors `internal/synthesis.Client.Synthesize`
  in shape: retry+ctx+observability. Maps HTTP 422→ErrInvalidRequest,
  504→ErrDeadlineExceeded, 402→ErrBudgetExceeded.
- [NEW] `internal/deepreport/config.go` — env-var driven base
  URL `STORM_SIDECAR_URL` (default `http://localhost:8001`),
  request timeout default 360 s.
- [NEW] `internal/deepreport/client_test.go` — unit tests with
  `httptest.NewServer` stub responses for each terminal outcome.
- [NEW] `internal/obs/metrics/deepreport.go` — declares
  `DeepReportOutcomes *prometheus.CounterVec{outcome}` (6 label
  values per spec.md §2.1(l)) and `DeepReportLatency
  prometheus.Histogram` (8 buckets).
- [MODIFY] `internal/obs/metrics/metrics.go` — register via
  `registerDeepReport(pr)` helper.
- [MODIFY] `internal/obs/obs.go` — re-export
  `obs.DeepReportOutcomes`, `obs.DeepReportLatency`.
- No cardinality allowlist amendment required: the `outcome` label
  NAME is already in `internal/obs/metrics/metrics_test.go:251-272`
  (line 257); DEEP-001 only adds 6 new VALUES on the existing
  allowlisted name, pre-initialised per the SYN-004 pattern at
  `streamsynth.go:48-56`. See §3.5 for the cross-collector
  cardinality table.

Exit criterion: Go test suite green; collectors visible in
`/metrics` scrape; stub HTTP server tests pass for all 5 terminal
outcomes.

### Milestone 7 [Priority Medium] — SSE long-form streaming

Goal: section-aware SSE emission via extended `internal/streamsynth/`.

Deliverables:
- [NEW] `internal/streamsynth/longform.go` — `StreamLongFormReport(ctx,
  w *sse.Writer, report deepreport.Report) (StreamStats, error)`.
  Walks sections; emits `section_start` → per-sentence
  `event: sentence` → `section_done` → terminal `event: done`.
- [NEW] `internal/streamsynth/longform_test.go` — unit tests for
  event order, payload shapes, citation invariant preservation
  (NFR-DEEP1-002 (a)(b)).
- [MODIFY] `cmd/usearch-api/handlers/...` (path per SPEC-IR-001
  layout) — add `POST /deep` (or equivalent) request handler with
  Accept-header content negotiation; SSE path → `streamsynth.StreamLongFormReport`;
  JSON path → marshalled `Report`.
- [NEW] `cmd/usearch-api/handlers/deep_stream_test.go` —
  integration tests for the three modes (SSE happy path, client
  disconnect, accept fallback) inheriting the SYN-004 test
  helpers.

Exit criterion: REQ-DEEP1-005 + REQ-DEEP1-006 acceptance tests
green; SYN-004 inherited tests (heartbeat, disconnect, write
timeout) re-verified for long-form path.

### Milestone 8 [Priority Medium] — Property tests + observability validation

Goal: NFR-DEEP1-002 property tests green; cardinality + log
shape validations.

Deliverables:
- [MODIFY] `services/storm/tests/test_pipeline.py` — add
  `hypothesis>=6` property tests covering NFR-DEEP1-002 (a)–(g).
- [MODIFY] `internal/streamsynth/longform_test.go` — add Go-side
  property tests via `testing/quick` for stream wire format
  conformance (extends NFR-SYN4-002 to long-form scale).
- [NEW] `internal/obs/metrics/deepreport_test.go` — verifies the
  6 outcome values are pre-initialised in /metrics output (SYN-004
  `streamsynth.go:48-56` pattern); no allowlist amendment to
  `metrics_test.go:251-272` because `outcome` label NAME is
  pre-existing.
- Log-record shape validation: add assertions in
  `services/storm/tests/test_obs.py` for the JSON record schema
  on each terminal outcome.

Exit criterion: all property tests green at default
`max_examples`; cardinality test green.

### Milestone 9 [Priority Medium] — Documentation + deployment

Goal: operator-runnable; sample requests in README; docker-compose
wiring.

Deliverables:
- [MODIFY] `services/storm/README.md` — operator quickstart, env
  vars, sample curl, expected SSE event vocabulary, latency
  expectations, cost expectations, troubleshooting.
- [MODIFY] `deploy/docker-compose.yml` (or equivalent) — add
  `storm` service entry mirroring the researcher service block
  (`deploy/docker-compose.yml:165-189`). Port mapping
  `${STORM_PORT:-8001}:8001` (host:container — same-port convention
  per researcher's `${RESEARCHER_PORT:-8081}:8081` at line 170;
  drops the previous v0.2 "8001:8000" claim). Env block includes
  `LITELLM_BASE_URL: http://litellm:4000` and the alias
  `LITELLM_API_KEY: ${LITELLM_MASTER_KEY}` (mirroring researcher
  service block lines 172-173 verbatim — host's
  `${LITELLM_MASTER_KEY}` is interpolated into the container's
  `LITELLM_API_KEY`). `depends_on: { litellm: { condition: service_healthy } }`
  matching researcher line 178-180. Healthcheck targets
  `localhost:8001/health` (matching researcher's pattern at line
  181-186 with the storm port substituted). Env file
  `services/storm/.env`.
- [MODIFY] `services/storm/.env.example` — finalize all env vars.
- Sample curl snippets for SSE path and JSON path.

Exit criterion: docker-compose brings storm up alongside
researcher; curl samples produce expected responses; README is
sufficient for an operator to run the service.

### Milestone 10 [Priority Low] — Pre-submission self-review

Goal: aggregate diff review per `workflow-modes.md` Pre-submission
Self-Review gate.

Deliverables:
- Review the full diff: are all abstractions earning their
  complexity? Is there shared code with `services/researcher/` that
  should be factored to a shared package? (Answer expected: NO —
  intentional duplication is preferred over premature abstraction
  at this M5 milestone; record decision under §10 D2.)
- Verify TRUST 5 gates pass.
- Verify @MX tags applied per §3.6.
- Verify Sprint Contract acceptance criteria all met (harness
  thorough → Sprint Contracts required).

Exit criterion: pre-submission review notes captured;
implementation marked complete via `<moai>COMPLETE</moai>`.

---

## 3. Technical Approach

### 3.1 Module boundary diagram

```
┌────────────────────────────────────────────────────────────────────┐
│                        cmd/usearch-api/                             │
│  POST /deep handler                                                 │
│   ├→ Accept: text/event-stream  → streamsynth.StreamLongFormReport  │
│   └→ Accept: application/json  → buffered JSON Report               │
│                                                                     │
│  internal/deepreport.Client.GenerateReport(ctx, req)                │
│   └→ HTTP POST /generate_report  (cross-process boundary)           │
│      target: http://storm:8001 (container) or                       │
│              http://localhost:${STORM_PORT:-8001} (host-mapped)     │
└────────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌────────────────────────────────────────────────────────────────────┐
│                       services/storm/  (port 8001)                  │
│                                                                     │
│  app.py:generate_report_endpoint                                    │
│   ├→ models.GenerateReportRequest validation                        │
│   └→ pipeline.run(req)                                              │
│        ├→ gateway.build_lm_configs()       [LiteLLM-rooted dspy.LM] │
│        ├→ inject_rm.InjectedRM(req.docs)                            │
│        ├→ STORMWikiRunner(args, lms, rm).run(...)  [upstream]       │
│        ├→ citation_translator.translate(text, refs, docs)           │
│        │   └→ canonicalize_url + URL→doc_id lookup                  │
│        ├→ ★ faithfulness.enforce_long_form_faithfulness(...)        │
│        │   └→ per-section sentence gate (SYN-002 regex)             │
│        ├→ [drop empty sections]                                     │
│        ├→ obs.emit_outcome("success" | "..." )  [exactly-once]      │
│        └→ models.GenerateReportResponse                             │
│                                                                     │
│  Deadline: asyncio.wait_for(... , STORM_MAX_LATENCY_MS / 1000)      │
│  Budget:    cost accumulator in gateway.py raises if cap exceeded   │
└────────────────────────────────────────────────────────────────────┘
```

### 3.2 Sidecar Pydantic models (`services/storm/src/storm/models.py`)

```python
class GenerateReportRequest(BaseModel):
    request_id: str
    query: str
    lang: str = ""
    docs: list[NormalizedDocPayload]
    # Per-call overrides; clamped to env-var ceilings per REQ-DEEP1-004.
    # max_tokens has no env-var ceiling (accepted as-is).
    max_tokens: int | None = None
    max_cost_usd: float | None = None
    max_perspectives: int | None = None
    max_conv_turns: int | None = None
    max_latency_ms: int | None = None  # clamped to STORM_MAX_LATENCY_MS

class Citation(BaseModel):
    marker: int
    doc_id: str
    url: str
    title: str

class Sentence(BaseModel):
    sentence_index: int
    text: str
    markers: list[int]

class Section(BaseModel):
    section_index: int
    heading: str
    level: int  # 1 for top-level, 2 for sub-sections
    text: str
    sentences: list[Sentence]

class GenerateReportResponse(BaseModel):
    request_id: str
    title: str
    sections: list[Section]
    citations: list[Citation]
    model: str
    provider: str
    cost_usd: float
    prompt_tokens: int
    completion_tokens: int
    latency_ms: float
    degraded: bool = False
    notice: str = ""
    schema_version: int = 1
```

### 3.3 Faithfulness gate (long-form variant)

```python
class EnforcementOutcome(StrEnum):
    ACCEPTED = "accepted"
    STRIPPED = "stripped"
    REJECTED = "rejected"
    OFF      = "off"

# DEEP-001-owned canonical sentence regex (single source of truth on the Python
# side; no services/researcher/src/researcher/faithfulness.py exists to import
# from — the SYN-002 Python-side gate was descoped at build time. The Go-side
# internal/streamsynth/streamsynth.go:28 regex is a separate symbol with the
# same literal string but is NOT imported here).
# @MX:ANCHOR: regex contract; @MX:REASON: any future change must be coordinated
# across the Go-side streamsynth file to avoid divergence.
_SENTENCE_END_RE = re.compile(r"[.!?。！？]\s+|[.!?。！？]$")

def enforce_long_form_faithfulness(
    sections: list[Section],
    docs: list[NormalizedDocPayload],
    mode: Literal["strip", "reject", "off"],
) -> tuple[list[Section], EnforcementOutcome, int, int]:
    """Returns (gated_sections, outcome, uncited_count, sections_affected)."""
    if mode == "off":
        return sections, EnforcementOutcome.OFF, 0, 0

    gated = []
    uncited_total = 0
    sections_affected = 0

    for section in sections:
        kept = []
        section_uncited = 0
        for sentence in section.sentences:
            if not sentence.markers:  # no [N] resolved markers
                section_uncited += 1
                continue
            kept.append(sentence)

        if section_uncited > 0:
            sections_affected += 1
            uncited_total += section_uncited

        if mode == "reject" and section_uncited > 0:
            # raised by caller after loop completes; collect total first
            pass

        if kept:  # non-empty section
            gated.append(section.copy(update={
                "sentences": kept,
                "text": " ".join(s.text for s in kept),
            }))

    if mode == "reject" and uncited_total > 0:
        raise UncitedLongFormError(uncited_count=uncited_total,
                                   sections_affected=sections_affected)

    outcome = (
        EnforcementOutcome.STRIPPED if uncited_total > 0
        else EnforcementOutcome.ACCEPTED
    )
    return gated, outcome, uncited_total, sections_affected
```

### 3.4 Citation translator (`citation_translator.py`)

```python
def canonicalize_url(url: str) -> str:
    """Strip query, lowercase host, normalize protocol, strip trailing slash."""
    parsed = urllib.parse.urlparse(url)
    scheme = "https"  # http+https treated as equivalent for matching
    netloc = parsed.netloc.lower()
    path = parsed.path.rstrip("/") or "/"
    return f"{scheme}://{netloc}{path}"

def translate(
    text: str,
    storm_refs: list[StormRef],   # [{n, url, title}]
    docs: list[NormalizedDocPayload],
) -> tuple[str, list[Citation], int]:
    """
    Returns (translated_text, citations[], unresolved_count).
    - storm_refs[i].n is the original [N] marker in `text`
    - returned citations[] is 1-indexed by `marker`, sorted
    """
    # build URL → doc_id lookup
    canon_to_doc = {canonicalize_url(d.url): d for d in docs}

    # iterate storm refs; keep only resolved
    resolved: list[Citation] = []
    old_to_new: dict[int, int] = {}
    unresolved = 0
    for ref in storm_refs:
        canon = canonicalize_url(ref.url)
        doc = canon_to_doc.get(canon)
        if doc is None:
            unresolved += 1
            continue
        new_marker = len(resolved) + 1
        resolved.append(Citation(
            marker=new_marker, doc_id=doc.id, url=doc.url, title=doc.title,
        ))
        old_to_new[ref.n] = new_marker

    # rewrite text: [old_n] → [new_marker] when resolved; remove if unresolved
    def replace(match: re.Match[str]) -> str:
        old = int(match.group(1))
        new = old_to_new.get(old)
        return f"[{new}]" if new is not None else ""

    translated = re.sub(r"\[(\d+)\]", replace, text)
    return translated, resolved, unresolved
```

### 3.5 New Prometheus collectors (Go-side)

```go
// internal/obs/metrics/deepreport.go

DeepReportOutcomes *prometheus.CounterVec  // labels: [outcome]
DeepReportLatency  prometheus.Histogram

// outcome label values (6):
//   success
//   deadline_exceeded
//   budget_exceeded
//   error_invalid          // 4xx from sidecar
//   error_upstream         // 5xx other than 504
//   error_unresolved_citations_threshold  // future use; reserved
```

Cardinality budget — verified against
`internal/obs/metrics/metrics_test.go:248
TestCardinalityGuardRejectsUnboundedLabels` (alias
`TestNoUnboundedLabels` at line 284). Verified facts:

- The cardinality guard is a label-NAME allowlist (not a value-count
  cap). Current allowlist (lines 251-272): `method, route,
  status_class, adapter_class, adapter, outcome, version, commit,
  go_version, provider, model, mode, store, op, shard` (15 names).
- The `outcome` label NAME is already in the allowlist (line 257);
  no new label NAME is added by DEEP-001.
- DEEP-001 adds 6 new VALUES on the `outcome` label, pre-initialised
  per the SYN-004 streamsynth.go:48-56 pattern (one Add(0) per value
  so all families appear in /metrics output before first real call).

Per-collector outcome label-value counts (verified from source):

| Collector | File | outcome value count | Notes |
|-----------|------|---------------------|-------|
| `usearch_llm_calls_total` | `llm.go:35` | 3 | `success, failure, timeout` per REQ-LLM-007 |
| `usearch_router_classifications_total` | `router.go:39` | 10 declared by `internal/router/metrics.go` constants (per `router.go:11` comment) | label-NAME unchanged |
| `usearch_synthesis_calls_total` | `synthesis.go:46` | 1 pre-init (`success`); values populated dynamically | SPEC-SYN-001 |
| `usearch_synthesis_faithfulness_outcomes_total` | `internal/synthesis/citation/citation.go:34-65` | 6 (`accepted, stripped, rejected, retry_succeeded, retry_failed, off`) | Go-side faithfulness, SPEC-SYN-002 |
| `usearch_syn004_outcomes_total` | `streamsynth.go:48-56` | 5 (`streamed_complete, client_disconnect, write_timeout, error_upstream, accept_fallback_to_json`) | SPEC-SYN-004 |
| `usearch_synthcluster_outcomes_total{outcome,mode}` | `synthcluster.go:44` | (multi-label; not a target for DEEP-001) | SPEC-SYN-003 |
| `usearch_deep_outcomes_total` (NEW) | `deepreport.go` | 6 (`success, deadline_exceeded, budget_exceeded, error_invalid, error_upstream, error_unresolved_citations_threshold`) | DEEP-001 — `error_unresolved_citations_threshold` reserved per §10 D4 |

Total `outcome` series across all Go-side collectors after DEEP-001
lands: 3 + 10 + ≥1 + 6 + 5 + 6 = 31 series (well under any
practical Prometheus per-metric ceiling; the project enforces
NAME-allowlist discipline rather than a numeric cap). The
`outcome` label NAME remains a single allowlisted name; no
amendment to `metrics_test.go:251-272` is required for DEEP-001.

### 3.6 SSE event schema extensions

| Event Type | New for DEEP-001? | Payload |
|------------|-------------------|---------|
| `section_start` | YES | `{request_id, section_index, heading, level, schema_version: 1}` |
| `sentence` | EXTENDED | `{request_id, section_index, sentence_index, text, citations: [...], schema_version: 1}` (added `section_index`) |
| `section_done` | YES | `{request_id, section_index, sentences_emitted, schema_version: 1}` |
| `done` | EXTENDED | `{request_id, total_sections, total_sentences, latency_ms, model, provider, cost_usd, schema_version: 1}` (added `total_sections`, `total_sentences`) |
| `error` | UNCHANGED | inherits from SYN-004 |
| `: ping` (heartbeat) | UNCHANGED | inherits from SYN-004 |

Backward compatibility analysis:
- Pre-DEEP-001 SYN-004 clients (single-paragraph synthesis) call
  the existing `/synthesize` SSE path; that path is unchanged. They
  do not call `/deep`.
- A future client calling `/deep` SSE that does not understand
  `section_start` / `section_done` events will skip them per W3C
  SSE spec (unknown event names are dispatched but typically
  filtered by the client's `addEventListener` set). The
  `event: sentence` extension adds the `section_index` JSON field;
  clients deserializing into a strict struct must accept additional
  fields (standard JSON convention). This is documented in the
  README sample.

### 3.7 MX Tag Plan

Per `.claude/rules/moai/workflow/mx-tag-protocol.md`:

| Target | Tag | Rationale |
|--------|-----|-----------|
| `services/storm/src/storm/pipeline.py:run()` | `@MX:ANCHOR` | Public sidecar orchestration entry point; fan_in ≥ 3 (app handler + tests + DEEP-002 future caller); LLM-trust + budget-guard boundary |
| `services/storm/src/storm/pipeline.py:run()` | `@MX:WARN` | LLM-trust + budget-guard boundary; @MX:REASON: cancels long-running asyncio task on deadline/budget; thread-pool concurrency from STORM upstream |
| `services/storm/src/storm/citation_translator.py:translate()` | `@MX:ANCHOR` | URL-to-doc_id resolution chokepoint; fan_in ≥ 2 (pipeline + tests); contract-critical |
| `services/storm/src/storm/faithfulness.py:enforce_long_form_faithfulness()` | `@MX:ANCHOR` | Long-form citation gate; fan_in ≥ 2 (pipeline + tests); DEEP-001 owns the Python-side regex literal (no SYN-002 Python source to reuse — Go-side `internal/streamsynth/streamsynth.go:28` is the sibling literal) |
| `services/storm/src/storm/faithfulness.py:enforce_long_form_faithfulness()` | `@MX:WARN` | LLM-trust boundary; @MX:REASON: accepts un-validated LLM output and decides reject/strip |
| `services/storm/src/storm/inject_rm.py:InjectedRM.forward()` | `@MX:NOTE` | Custom retrieval shim; bypasses STORM's default RMs |
| `services/storm/src/storm/gateway.py:build_lm_configs()` | `@MX:NOTE` | LiteLLM-rooted; SPEC-LLM-001 constraint surface |
| `internal/deepreport/client.go:Client.GenerateReport()` | `@MX:ANCHOR` | Public Go-side entry point; fan_in ≥ 3 (handler + tests + DEEP-002); cross-process boundary |
| `internal/streamsynth/longform.go:StreamLongFormReport()` | `@MX:ANCHOR` | Streaming entry point; fan_in ≥ 2 (handler + tests); preserves SYN-004 invariants |
| `internal/obs/metrics/deepreport.go` (new collectors) | `@MX:NOTE` | Counter cardinality discipline note (6 outcome values pre-initialised per SYN-004 `streamsynth.go:48-56` pattern; no allowlist amendment — the `outcome` label NAME is pre-existing) |

@MX:TODO tags will be auto-generated during the RED phase per
`.claude/rules/moai/workflow/mx-tag-protocol.md`; cataloged inline
at function level (not pre-cataloged in §3.7); auto-resolved in
GREEN/REFACTOR phases. This avoids the upstream contradiction
where the TODO list at plan time becomes stale by the time RED
phase begins.

---

## 4. Risks (top 3, summary — full register in research.md §5)

1. **Citation marker → doc_id translation gap (R2)** — STORM emits
   URL-cited content; we translate via canonicalization. Without
   robust canonicalization, faithfulness gate strips the entire
   long-form output. Mitigated by REQ-DEEP1-002 + 8-combination
   canonicalization tests.
2. **Latency exceeds 5-min budget (R3)** — conservative knobs +
   `STORM_MAX_LATENCY_MS=300000` ctx ceiling. NFR-DEEP1-001
   targets p50 ≤ 180 s, p95 ≤ 300 s.
3. **Token cost runaway (R4)** — `STORM_MAX_COST_USD=2.50` per-call
   cap. SPEC-DEEP-004 will tie to per-user/per-day cap.

---

## 5. Dependencies

### 5.1 Upstream SPEC dependencies (must be implemented)

- **SPEC-SYN-001** (implemented): `Citation` Pydantic model;
  marker→doc_id mapping invariant. REUSED at long-form scale.
- **SPEC-SYN-002** (implemented): faithfulness gate contract +
  canonical sentence regex. REUSED verbatim.
- **SPEC-SYN-004** (implemented): SSE wire format, heartbeat,
  disconnect cancel, write timeout, NFR-SYN4-003 exactly-once
  invariant pattern. REUSED + EXTENDED.
- **SPEC-IR-001** (implemented): `RoutingDecision.Lang` flows into
  the long-form `lang` hint.
- **SPEC-FAN-001** (implemented): produces the doc corpus consumed
  by the injected RM.
- **SPEC-LLM-001** (implemented): LiteLLM proxy; `Client`
  interface; cost accumulation header. STORM's LM configs route
  through LiteLLM exclusively.
- **SPEC-CORE-001** (implemented): `pkg/types.NormalizedDoc.ID`.

### 5.2 Coordinating SPECs (no hard dependency)

- **SPEC-OBS-001** (implemented): metric registration pattern.
  No cardinality allowlist amendment required (the `outcome` label
  NAME is pre-existing in `internal/obs/metrics/metrics_test.go:257`;
  see plan.md §3.5 verified per-collector breakdown).
- **SPEC-IDX-003** (Korean tokenization): not a hard dep — STORM
  generation does not invoke mecab-ko; sentence segmentation by
  SYN-002 regex handles Korean punctuation.

### 5.3 Downstream blocked SPECs

- **SPEC-DEEP-002** (M5): multi-agent pipeline consumes the STORM
  surface as one stage among many.
- **SPEC-DEEP-003** (M5): tree exploration may layer on top of
  STORM's perspective generation.
- **SPEC-DEEP-004** (M5): per-user/per-day quota + Haiku
  pre-screen + prompt-cache reuse — extends the per-call caps
  established by DEEP-001.
- **SPEC-EVAL-001** (M8): RAGAS-style semantic faithfulness
  scoring extends to long-form once DEEP-001 lands.
- **SPEC-CLI-002** (M7): `usearch deep "..."` end-to-end CLI
  surface; consumes `internal/deepreport.Client`.
- **SPEC-MCP-001** (M7): MCP `deep` tool wraps the Go-side client.

### 5.4 External dependencies (run-phase verification)

New Python runtime dependencies:
- `knowledge-storm == 1.1.1` (verified via WebFetch 2026-05-10
  against `https://pypi.org/project/knowledge-storm/`; minimum
  Python ≥3.10; upstream `https://github.com/stanford-oval/storm`)
- `dspy-ai` (transitive dep of `knowledge-storm`; pinned to the
  range declared by the upstream — verified at run phase via
  `pip show knowledge-storm` and `pip show dspy-ai`)
- `fastapi`, `uvicorn[standard]` (mirrors researcher)
- `httpx`, `pydantic>=2` (mirrors researcher)
- Dev deps: `pytest`, `ruff`, `pip-audit`, `hypothesis>=6`

No new Go module dependencies (reuses standard `net/http`,
existing `internal/sse`, existing `internal/obs/metrics`).

---

## 6. File Impact

### 6.1 [NEW] Files to create

| Path | Purpose |
|------|---------|
| `services/storm/src/storm/__main__.py` | uvicorn entry point |
| `services/storm/src/storm/app.py` | FastAPI router + endpoints + handlers |
| `services/storm/src/storm/models.py` | Pydantic request/response shapes |
| `services/storm/src/storm/gateway.py` | LiteLLM-rooted dspy.LM factory |
| `services/storm/src/storm/obs.py` | JSON log + counter dispatcher |
| `services/storm/src/storm/pipeline.py` | STORM orchestration (REQ-DEEP1-001..004) |
| `services/storm/src/storm/inject_rm.py` | Injected retrieval module (request docs[]) |
| `services/storm/src/storm/citation_translator.py` | URL→doc_id translation (REQ-DEEP1-002) |
| `services/storm/src/storm/faithfulness.py` | Long-form citation gate (REQ-DEEP1-003) |
| `services/storm/Dockerfile` | Multi-stage Python 3.11 slim |
| `services/storm/tests/test_app.py` | HTTP-level integration |
| `services/storm/tests/test_gateway.py` | LiteLLM config-factory tests |
| `services/storm/tests/test_pipeline.py` | Mocked-LM unit tests + property tests |
| `services/storm/tests/test_inject_rm.py` | Retrieval module tests |
| `services/storm/tests/test_citation_translator.py` | URL canonicalization tests |
| `services/storm/tests/test_faithfulness.py` | Long-form gate tests |
| `services/storm/tests/test_obs.py` | Log/counter shape assertions |
| `services/storm/tests/test_caps.py` | Deadline + budget cancellation tests |
| `internal/deepreport/types.go` | Go shapes + error sentinels |
| `internal/deepreport/client.go` | HTTP client (mirrors synthesis.Client) |
| `internal/deepreport/config.go` | Env-var driven config |
| `internal/deepreport/client_test.go` | Stub-server unit tests |
| `internal/streamsynth/longform.go` | Section-aware SSE emitter (REQ-DEEP1-005) |
| `internal/streamsynth/longform_test.go` | Stream tests + property tests |
| `internal/obs/metrics/deepreport.go` | Two new collectors |
| `internal/obs/metrics/deepreport_test.go` | Cardinality test |
| `.moai/sprints/SPEC-DEEP-001-sprint-1.md` | Sprint Contract per `.claude/rules/moai/design/constitution.md` §11 (harness=thorough → required); acceptance checklist + priority dimension + test scenarios + pass conditions for the first GAN Loop iteration |
| `.moai/sprints/SPEC-DEEP-001-sprint-2.md` | Sprint Contract for iteration 2 (refined per iteration 1 feedback; passed criteria carry forward) |
| `.moai/sprints/SPEC-DEEP-001-sprint-3.md` | Sprint Contract for iteration 3 (if iterations 1-2 do not pass) |
| `.moai/sprints/SPEC-DEEP-001-sprint-4.md` | Sprint Contract for iteration 4 (escalation gate per design constitution §11 escalation_after) |
| `.moai/sprints/SPEC-DEEP-001-sprint-5.md` | Sprint Contract for iteration 5 (max iteration per design constitution §11 max_iterations; deadlock report if not passed) |

### 6.2 [MODIFY] Files to modify

| Path | Change |
|------|--------|
| `services/storm/pyproject.toml` | Add deps: knowledge-storm, dspy-ai, fastapi, uvicorn, httpx, pydantic, hypothesis |
| `services/storm/.env.example` | Reconcile to `LITELLM_BASE_URL` (matches researcher/gateway.py:26 literal default `http://litellm:4000`) and `LITELLM_API_KEY` for in-container env (matches researcher/gateway.py:27 literal); document `LITELLM_MASTER_KEY` as the host-side var that docker-compose aliases into the container's `LITELLM_API_KEY` per researcher precedent (`deploy/docker-compose.yml:173`); add `STORM_*` env vars |
| `services/storm/README.md` | Operator quickstart, sample curl, SSE vocabulary docs |
| `internal/obs/metrics/metrics.go` | Register deepreport collectors via `registerDeepReport(pr)` |
| `internal/obs/obs.go` | Re-export `obs.DeepReportOutcomes`, `obs.DeepReportLatency` |
| (no modification needed) | The cardinality guard at `internal/obs/metrics/metrics_test.go:248 TestCardinalityGuardRejectsUnboundedLabels` (alias `TestNoUnboundedLabels` line 284) gates label NAMES — `outcome` is pre-existing on line 257; DEEP-001 only adds VALUES. See §3.5. |
| `cmd/usearch-api/handlers/...` | Add `POST /deep` handler with Accept-header content negotiation (path per SPEC-IR-001 server layout) |
| `cmd/usearch-api/handlers/deep_stream_test.go` | Integration tests for SSE + JSON paths |
| `deploy/docker-compose.yml` | Add `storm` service entry mirroring the researcher block (`deploy/docker-compose.yml:165-189`). Port `${STORM_PORT:-8001}:8001` (same-port convention per researcher line 170); env alias `LITELLM_API_KEY: ${LITELLM_MASTER_KEY}` (mirroring line 173 verbatim); `depends_on: { litellm: { condition: service_healthy } }`; healthcheck `localhost:8001/health` |

### 6.3 [EXISTING — UNCHANGED]

| Path | Reason |
|------|--------|
| `services/researcher/` | Distinct service; SYN-001/002 contract preserved |
| `internal/synthesis/` | Distinct client; long-form is parallel package |
| `internal/sse/writer.go`, `internal/sse/heartbeat.go` | Reused as-is |
| `internal/streamsynth/streamsynth.go` | Single-paragraph variant unchanged; long-form is sibling file |
| `pkg/types/normalized_doc.go` | Schema reused; no extension |
| `internal/router/`, `internal/fanout/` | Upstream of DEEP-001; consumed read-only |
| `internal/llm/` | LiteLLM client; STORM uses Python-side dspy.LM, not Go-side llm.Client |

---

## 7. Test Plan (development cycle)

Methodology: TDD RED-GREEN-REFACTOR per
`.moai/config/sections/quality.yaml`.

### RED phase order (failing tests first)

#### Python (services/storm/)

1. `test_health_returns_ok` — `/health` smoke.
2. `test_readyz_reflects_dep_state` — `/readyz` dep-state.
3. `test_post_generate_report_returns_200_with_structured_response`.
4. `test_response_schema_version_present`.
5. `test_canonicalize_url_strips_query_and_lowercases_host`.
6. `test_canonicalize_url_normalizes_protocol_and_trailing_slash`.
7. `test_translate_resolves_marker_to_doc_id`.
8. `test_translate_strips_unresolved_marker`.
9. `test_translate_unresolved_counter_increments`.
10. `test_translate_citations_array_sorted_and_one_indexed`.
11. `test_inject_rm_forward_returns_top_k_lexical_matches`.
12. `test_inject_rm_no_external_http_calls` (mock confirms zero).
13. `test_pipeline_run_invokes_storm_runner_with_injected_rm`.
14. `test_pipeline_uses_litellm_lm_configs_only` (import-graph
    test).
15. `test_pipeline_response_schema_well_formed` (Pydantic
    validation).
16. `test_faithfulness_strip_removes_uncited_sentences`.
17. `test_faithfulness_reject_returns_422`.
18. `test_faithfulness_off_bypasses_gate`.
19. `test_faithfulness_empty_section_removed`.
20. `test_faithfulness_outcomes_counter_increments`.
21. `test_faithfulness_uses_syn002_canonical_regex` (regex literal
    equality).
22. `test_deadline_exceeded_returns_504`.
23. `test_deadline_exceeded_no_partial_text`.
24. `test_deadline_exceeded_emits_warn_log`.
25. `test_budget_exceeded_returns_402`.
26. `test_budget_exceeded_increments_counter`.
27. `test_outcome_counter_at_most_once_per_request`.
28. `test_outcome_counter_race_success_vs_deadline`.
29. `test_outcome_counter_race_deadline_vs_budget`.
30. `test_property_long_form_marker_resolution` (hypothesis).
31. `test_property_section_sentences_markers_in_range` (hypothesis).
32. `test_property_no_empty_sections` (hypothesis).

#### Go (internal/deepreport, internal/streamsynth, internal/obs)

33. `TestClientGenerateReportSuccess` — stub server returns 200,
    parsed correctly.
34. `TestClientGenerateReportInvalidRequest` — 422 → ErrInvalidRequest.
35. `TestClientGenerateReportDeadlineExceeded` — 504 → ErrDeadlineExceeded.
36. `TestClientGenerateReportBudgetExceeded` — 402 → ErrBudgetExceeded.
37. `TestClientGenerateReportSidecarUnreachable` — conn refused →
    ErrSidecarUnreachable.
38. `TestClientGenerateReportContextCanceled` — ctx cancel → ErrTimeout.
39. `TestStreamLongFormReportEmitsSectionStartPerSection`.
40. `TestStreamLongFormReportEmitsSentencePerSentenceWithSectionIndex`.
41. `TestStreamLongFormReportEmitsSectionDonePerSection`.
42. `TestStreamLongFormReportEmitsDoneWithTotals`.
43. `TestStreamLongFormReportInheritsHeartbeat`.
44. `TestStreamLongFormReportInheritsDisconnectCancel`.
45. `TestStreamLongFormReportInheritsWriteTimeout`.
46. `TestStreamLongFormReportPreservesSyn002Invariant` (property).
47. `TestStreamLongFormReportNoUncitedSentenceEmitted`.
48. `TestDeepReportOutcomesCounterRegistered`.
49. `TestDeepReportLatencyHistogramRegistered`.
50. `TestDeepReportCardinalityWithinAllowlist`.
51. `TestUsearchAPIDeepHandlerSSEPath` (integration).
52. `TestUsearchAPIDeepHandlerJSONFallbackPath` (integration).
53. `TestUsearchAPIDeepHandlerAcceptHeaderContentNegotiation`.

### GREEN phase

Implement minimum code to pass each test. Order: M1 → M2 → M3 →
M4 → M5 → M6 → M7 → M8. M9 (docs) is post-GREEN. M10
(self-review) is the final gate.

### REFACTOR phase

- Extract repeated counter-emission patterns into `obs.py:emit_outcome`
  helper (single source of truth for NFR-DEEP1-003 invariant).
- DRY any duplicated test fixtures across Python tests.
- Consider extracting `services/storm/src/storm/_common.py` for
  any utility shared between modules (e.g., URL canonicalization)
  but resist premature shared package with `services/researcher/`
  until the structural-mirror count > 3 (TRUST 5 Enforce Simplicity).
  The faithfulness regex is NOT a candidate — it is a fresh
  DEEP-001-owned literal because no SYN-002 Python source exists
  (see §10 D2).
- Pre-submission self-review per `workflow-modes.md`: confirm
  abstractions earn their complexity; remove anything that does
  not.

### Coverage targets

- Python `services/storm/`: ≥85% (project default).
- Go `internal/deepreport/`: ≥85%.
- Go `internal/streamsynth/longform.go`: ≥85%.
- Go `internal/obs/metrics/deepreport.go`: ≥85%.

---

## 8. Quality gates (TRUST 5)

| Pillar | Check |
|--------|-------|
| **Tested** | 53 RED-phase tests + property tests; ≥85% coverage on every new module |
| **Readable** | Each Python module ≤ 300 LOC; each Go file ≤ 400 LOC; godoc/docstring on every exported function; no nested function > 60 LOC |
| **Unified** | `ruff check services/storm/` green; `ruff format --check services/storm/` green; `gofmt -d` empty for new Go files; `golangci-lint run ./internal/deepreport/... ./internal/streamsynth/... ./internal/obs/metrics/...` green |
| **Secured** | No new external attack surface beyond the LiteLLM proxy already established; LLM input validated via Pydantic before STORM invocation; no PII in logs (REQ-SYN-006 redaction discipline); 422 / 504 / 402 response bodies contain no `text` / `sections` content (no leakage) |
| **Trackable** | Conventional commit `feat(deep): SPEC-DEEP-001 STORM long-form report sidecar`; @MX tags applied per §3.7; SPEC reference in commit message |

LSP gates (per `.moai/config/sections/quality.yaml`):
- run phase: zero errors, zero type errors, zero lint errors
- sync phase: zero errors, max 10 warnings, clean LSP

---

## 9. Open Questions (to resolve in run phase)

1. **knowledge-storm version pin** — Resolved: pin
   `knowledge-storm == 1.1.1` (verified via WebFetch on
   `https://pypi.org/project/knowledge-storm/` 2026-05-10; latest
   stable released 2025-09-29; minimum Python ≥3.10). Run-phase
   `pip install knowledge-storm==1.1.1` + functional smoke required
   before unpinning.
2. **STORM's actual public surface — VERIFIED PREREQUISITE
   (was Open Question)** — Resolved via WebFetch on
   `https://pypi.org/project/knowledge-storm/` and
   `https://github.com/stanford-oval/storm/blob/main/README.md`
   (2026-05-10). Verified API surface for `knowledge-storm == 1.1.1`:
   - Top-level imports: `from knowledge_storm import STORMWikiRunner,
     STORMWikiLMConfigs, STORMWikiRunnerArguments`
   - LM wrapper: `from knowledge_storm.lm import LitellmModel`
     (preferred path for LiteLLM-rooted LM access; supersedes any
     `dspy.LM` direct usage suggested in research.md §3.3 — research
     was correct that the upstream README v1.1.0 introduced
     `litellm integration`)
   - Retrieval module base: `from knowledge_storm.rm import YouRM,
     BingSearch, ...` — DEEP-001 does NOT use any of these; DEEP-001
     injects a custom RM via the same interface (`forward(query, k)
     -> list[dict]`)
   - Canonical invocation:
     ```python
     lm_configs = STORMWikiLMConfigs()
     gpt_haiku = LitellmModel(model="claude-haiku-4-5", max_tokens=500, **kwargs)
     gpt_sonnet = LitellmModel(model="claude-sonnet-4-6", max_tokens=3000, **kwargs)
     lm_configs.set_conv_simulator_lm(gpt_haiku)
     lm_configs.set_article_gen_lm(gpt_sonnet)
     args = STORMWikiRunnerArguments(...)  # search_top_k, max_perspectives, etc.
     rm = InjectedRM(req.docs, top_k=args.search_top_k)  # DEEP-001 custom
     runner = STORMWikiRunner(args, lm_configs, rm)
     runner.run(topic=req.query, do_research=True, do_generate_article=True)
     runner.summary()
     ```
   - Co-Storm collaborative variant (`CoStormRunner`,
     `CollaborativeStormLMConfigs`) is OUT OF SCOPE for DEEP-001
     (defer to SPEC-DEEP-002).
   Milestones 1–2 reference these names verbatim; no amendment
   needed. The `gateway.py` factory uses
   `knowledge_storm.lm.LitellmModel`, NOT raw `dspy.LM`.
3. **Should sentence segmentation regex be imported from
   `services/researcher/`?** Resolved: NO — moot. There is no
   `services/researcher/src/researcher/faithfulness.py` to import
   from. The SYN-002 Python-side gate was descoped at build time;
   only the Go-side `internal/streamsynth/streamsynth.go:28` regex
   literal survives. DEEP-001 declares the canonical Python-side
   literal in `services/storm/src/storm/faithfulness.py` as a fresh
   DEEP-001-owned source of truth. Coordinated change with the
   Go-side sibling is enforced via @MX:ANCHOR cross-reference.
4. **Korean conv-simulator localization** — should we customize
   STORM's perspective-simulator system prompt for `lang == "ko"`
   to ensure Korean conversation? Default: NO for v0 (best-effort);
   localization is a follow-up if Korean strip rates exceed 30%.
5. **Cardinality allowlist amendment timing** — Resolved: NO
   amendment required. Verified against
   `internal/obs/metrics/metrics_test.go:251-272` (alias
   `TestNoUnboundedLabels` at line 284): the cardinality guard
   gates label NAMES, not VALUES. The `outcome` label NAME is
   already allowlisted (line 257). DEEP-001's 6 new values
   (`success, deadline_exceeded, budget_exceeded, error_invalid,
   error_upstream, error_unresolved_citations_threshold`) are
   pre-initialised per the SYN-004 pattern at `streamsynth.go:48-56`
   without modifying the allowlist test. See §3.5 cardinality table.

---

## 10. Decisions (resolved before run-phase entry)

These items are pre-committed before the annotation cycle begins.

### D1 — Faithfulness mode `off` emits counter (unlike SYN-002)

**Context**: SPEC-SYN-002 REQ-SYN2-003 mandates that
`mode=off` bypass the entire faithfulness gate AND not increment
the counter. DEEP-001 REQ-DEEP1-003 includes `off` as one of the
4 counter values.

**Decision**: DEEP-001 emits the counter with
`outcome=off` even in mode=off, unlike SYN-002.

**Rationale**:
- Long-form is high-value-per-call (~$0.30–$2.50, ~3 min). Knowing
  whether mode=off is in use operationally is more valuable than
  the metric purity argument that drove SYN-002 D2.
- The cardinality cost is 1 additional series; below the
  SPEC-OBS-001 budget.
- The `STORM_FAITHFULNESS_MODE` env var is a per-deployment
  config, not a per-request runtime decision; aggregate counters
  per deployment are useful.

**Propagation**:
- spec.md REQ-DEEP1-003: 4-value enum (accepted, stripped,
  rejected, off).
- plan.md §3.3 enforce_long_form_faithfulness returns
  `EnforcementOutcome.OFF` in mode=off path.
- acceptance.md scenarios for mode=off assert counter +1 with
  `outcome="off"`.

### D2 — Structural mirroring of services/researcher/ (NOT regex duplication)

**Context**: `gateway.py`, `obs.py`, `app.py`, `models.py` mirror
their `services/researcher/` counterparts in shape (FastAPI app
factory, LiteLLM gateway pattern, Pydantic shapes, JSON-log obs).
Note: `faithfulness.py` is NOT a duplication — there is no
`services/researcher/src/researcher/faithfulness.py` (the SYN-002
Python-side gate was descoped at build time; the only SYN-002
reference is the Go-side `internal/streamsynth/streamsynth.go:28`
regex literal). DEEP-001's `faithfulness.py` is wholly new and
DEEP-001-owned.

**Decision**: Accept structural mirroring for `gateway.py`,
`obs.py`, `app.py` shape. Do NOT extract to a shared
`services/_shared/` package.

**Rationale**:
- Premature abstraction (TRUST 5 Enforce Simplicity); the two
  services' contracts may diverge as DEEP-002/003 land.
- Each service is independently deployable; shared package would
  introduce a build-order coupling.
- If duplication count exceeds 3 modules, revisit in M5 close-out.
- The faithfulness regex literal is NOT a duplication concern —
  it is two independent source-of-truth declarations (Python in
  DEEP-001, Go in SYN-004's streamsynth.go). Coordinated change
  is enforced via @MX:ANCHOR + @MX:REASON pointing at the sibling
  file.

**Propagation**:
- plan.md §7 REFACTOR phase notes; §10 D2 captures the decision.
- M10 Pre-submission Self-Review explicitly checks this.

### D3 — Buffered-then-streamed mode for v0 streaming

**Context**: STORM's internal LM calls are buffered; the only
streaming we can offer at v0 is post-completion section/sentence
emission.

**Decision**: v0 ships buffered-then-streamed; intra-section
token streaming is deferred to a follow-up SPEC parallel to the
SYN-006 / SYN-004-v2 sidecar streaming upgrade.

**Rationale**:
- Matches SPEC-SYN-004 v0 buffered-then-streamed mode (precedent).
- Section-level granularity + heartbeat already provides
  perceived progress for ≤5-min-latency workloads.
- Token-level streaming requires deep integration with STORM's
  internal call graph (not a public surface).

**Propagation**:
- spec.md §2.2 lists "token-level streaming from STORM internals"
  as exclusion.
- NFR-DEEP1-001 latency budget tolerates buffered-then-streamed.

### D4 — `error_unresolved_citations_threshold` outcome reserved for future use

**Context**: the unresolved-citation rate is operational signal
but does not in itself fail a request — high unresolved rates
just feed the strip path.

**Decision**: pre-declare `error_unresolved_citations_threshold`
as an outcome label value in the spec.md REQ-DEEP1-001's collector
set, but do NOT wire it in v0. Reserved for a future SPEC that
introduces a hard threshold.

**Rationale**:
- Allowlist amendments are expensive (cross-team coordination);
  pre-reserving the value avoids a future amendment.
- Cardinality cost is identical (the value is enumerated even if
  never emitted; metric series only exists once first observation
  fires).

**Propagation**:
- spec.md §2.1(l): 6 outcome values listed (5 active + 1
  reserved).
- plan.md §3.5: 6 outcome values listed; reserved noted.
- acceptance.md NFR-DEEP1-003: assertion ranges over all 6
  values; the reserved value never fires in v0 tests, no test
  failure.

---

*End of SPEC-DEEP-001 plan v0.1*
