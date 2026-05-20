# SPEC-DEEP-001 Research — STORM Integration

Companion artifact for `.moai/specs/SPEC-DEEP-001/spec.md`.
Version: 0.1.0 (draft)
Created: 2026-05-10
Author: limbowl (via manager-spec)

---

## 1. Goal

`.moai/project/roadmap.md` line 74 declares M5 deliverable
SPEC-DEEP-001:

> STORM integration | wrap STORM as Python service, long-form report
> generation | expert-backend

M5 milestone exit criterion (`roadmap.md` line 154):

> `usearch deep "..."` returns STORM-style report with ≥10 cited
> sources in ≤5 min.

SPEC-DEEP-001 is the **first** of four M5 SPECs (DEEP-001/002/003/004).
DEEP-001 establishes the **long-form report generation surface**:
wrapping the upstream `stanford-oval/storm` (PyPI: `knowledge-storm`)
library as a Python sidecar service (`services/storm/`) reachable
from the Go-side via an HTTP contract. DEEP-002 (multi-agent
Researcher→Reviewer→Writer→Verifier pipeline), DEEP-003 (tree
exploration), and DEEP-004 (quota + cost guard) layer on top of this
service and are explicitly out of scope here.

This research artifact documents (a) the upstream STORM library's
public surface and operational characteristics, (b) the
`services/researcher/` sidecar pattern that DEEP-001 mirrors, (c) the
M4 contracts (SPEC-SYN-002 citation faithfulness, SPEC-SYN-004 SSE
streaming) that the long-form output must respect, (d) Korean-language
considerations relevant to STORM, and (e) risks for the SPEC author.

---

## 2. Codebase Trace — Where Long-Form Output Will Land

### 2.1 Existing sidecar pattern: `services/researcher/`

The Python sidecar pattern is established by SPEC-SYN-001 and hardened
by SPEC-SYN-002. Layout under `services/researcher/`:

```
services/researcher/
├── Dockerfile               # multi-stage, slim base
├── pyproject.toml           # uv / hatchling, ruff, pytest
├── README.md                # operator quickstart
├── src/researcher/
│   ├── __init__.py
│   ├── __main__.py          # uvicorn entry point
│   ├── app.py               # FastAPI router + endpoint registration
│   ├── gateway.py           # LiteLLM-proxy LLM client (httpx)
│   ├── models.py            # Pydantic request/response shapes
│   ├── obs.py               # slog-equivalent JSON logging + counters
│   ├── synthesis.py         # core orchestration (LLM call → markers → faithfulness)
│   └── faithfulness.py      # SPEC-SYN-002 citation gate
└── tests/
    ├── test_app.py          # HTTP-level integration
    ├── test_synthesis.py    # synthesis unit tests
    ├── test_faithfulness.py # SPEC-SYN-002 unit tests
    ├── test_gateway.py      # LiteLLM client tests
    └── test_obs.py          # log/metric assertions
```

Public HTTP surface (consumed by Go-side `internal/synthesis`):

```
POST /synthesize
  Request:  { request_id, query, lang, docs: [NormalizedDocPayload] }
  Response: { request_id, text, citations: [{marker, doc_id, url, title}],
              model, provider, cost_usd, prompt_tokens, completion_tokens,
              latency_ms, degraded, notice }
GET /health → { status: "ok" }
GET /readyz  → { ready: true | false, deps: {...} }
```

Observability conventions (per SPEC-SYN-001 / SPEC-OBS-001):
- JSON log records via `obs.py:log_synthesis(payload)`
- Prometheus collectors registered Go-side (`internal/obs/metrics/`)
  — Python emits via process scrape; Go reads via instrumented client
- Cardinality allowlist enforced (no unbounded labels)

**Deep-001 mirrors this layout verbatim.** The skeleton at
`services/storm/` (committed at commit `[storm scaffold]`,
2026-04-24, currently empty `src/storm/__init__.py` + scaffolding
pyproject + .env.example) is the landing site. Note: the
.env.example is reconciled in the v0.3 evidence-first amendment to
`LITELLM_BASE_URL` (matches `services/researcher/src/researcher/gateway.py:26`
literal default) and `LITELLM_API_KEY` for the in-container API-key
env (matches `services/researcher/src/researcher/gateway.py:27`
literal `api_key = os.environ.get("LITELLM_API_KEY", "")`); the
host-side `LITELLM_MASTER_KEY` is aliased into the container's
`LITELLM_API_KEY` via `deploy/docker-compose.yml:173` per the
researcher precedent. The auth-key anchor is gateway.py:**27** (NOT
gateway.py:26 which only sets the base URL).

### 2.2 Existing Go client pattern: `internal/synthesis/`

```
internal/synthesis/
├── citation/
├── client.go              # @MX:ANCHOR Client.Synthesize
├── client_test.go
├── config.go              # env-var driven base URL, timeouts
├── synthesis.go           # higher-level workflow helpers
└── types.go               # Request/Response/Error sentinels
```

`Client.Synthesize(ctx, req) (Result, error)` shape:
- `Request{ RequestID, Query, Lang, Docs []Doc }`
- `Result{ RequestID, Text, Citations []Citation, Model, Provider,
   CostUSD, PromptTokens, CompletionTokens, LatencyMs, Degraded, Notice }`
- Sentinels: `ErrInvalidRequest` (4xx), `ErrSidecarUnreachable`
  (network), `ErrTimeout` (ctx deadline)

DEEP-001 introduces a **parallel** Go client at
`internal/deepreport/` (proposed name) that consumes a NEW Python
sidecar at `services/storm/`. Contract is **distinct from**
`internal/synthesis/` — long-form is a multi-section report, not a
single paragraph — but the operational pattern (Request/Response,
sentinels, retry+ctx, slog+prom+otel) is **identical**.

### 2.3 Streaming integration: `internal/streamsynth/` + `internal/sse/`

SPEC-SYN-004 (implemented) provides:
- `internal/sse/` — pure-Go SSE writer (`Writer`, `WriteEvent`,
  `WriteComment`, `Flush`, heartbeat goroutine)
- `internal/streamsynth/` — orchestration between
  `synthesis.Client.Synthesize()` and `sse.Writer`; segments by
  SPEC-SYN-002 sentence regex; emits `event: sentence`, `event: done`,
  `event: error` per-sentence with citations

DEEP-001 long-form output is **multi-section** (introduction,
multiple body sections, conclusion, references); the streaming
contract for long-form is therefore **section-granular** for
section starts and **sentence-granular** within each section, all
flowing through the same SSE wire format defined by SPEC-SYN-004.
DEEP-001 establishes the section emission contract; DEEP-002 will
likely add intermediate stage events (`event: research_done`,
`event: writer_started`). v0 streaming for DEEP-001 is
**buffered-then-streamed** — the sidecar generates the full report,
then the Go-side `streamsynth` (extended) walks sections + sentences
and emits SSE.

### 2.4 LLM access: `internal/llm` (SPEC-LLM-001)

[HARD] All LLM access on both Go and Python sides routes through the
LiteLLM proxy at `LITELLM_BASE_URL` (default `http://localhost:4000`).
No direct Anthropic/OpenAI/Ollama SDKs.

For DEEP-001 this is a **constitutional constraint**: STORM's upstream
library accepts an `lm` argument (a generic LM wrapper) that ours must
configure to route through LiteLLM. Direct vendor SDK use is
explicitly prohibited.

Concrete wiring path:
- Python sidecar uses `knowledge_storm.lm.LitellmModel` (preferred
  per upstream README v1.1.0+ `litellm integration`; falls back to
  `dspy.LM` if needed) pointed at `LITELLM_BASE_URL`
  (default `http://litellm:4000`, matches
  `services/researcher/src/researcher/gateway.py:26` verbatim
  literal `base_url = os.environ.get("LITELLM_BASE_URL", "http://litellm:4000")`)
  with provider prefix (`openai/claude-haiku-4-5`,
  `openai/claude-sonnet-4-6`, etc.) — the `openai/` prefix is
  LiteLLM's universal protocol, not Anthropic bypassed.
- Authentication: storm Python container internally reads
  `LITELLM_API_KEY` (verbatim mirror of
  `services/researcher/src/researcher/gateway.py:27` literal
  `api_key = os.environ.get("LITELLM_API_KEY", "")` — gateway.py:27,
  NOT gateway.py:26, is the auth-key anchor; gateway.py:26 only sets
  the base URL). The host-side `LITELLM_MASTER_KEY` env var is
  aliased into the container's `LITELLM_API_KEY` by
  `deploy/docker-compose.yml:173` verbatim
  `LITELLM_API_KEY: ${LITELLM_MASTER_KEY}` (mirror this for the storm
  service entry). The master key value reaches the upstream LiteLLM
  proxy as `Authorization: Bearer <key>`. SPEC-LLM-001 REQ-LLM-005
  verbatim text is "The Client SHALL authenticate to the LiteLLM
  proxy via the `LITELLM_MASTER_KEY` environment variable sent as an
  `Authorization: Bearer <key>` header on every request…[clause
  omitted: secret-redaction obligations]…" — the
  literal env-var name `LITELLM_MASTER_KEY` in REQ-LLM-005 governs
  the **Go-side** `internal/llm` Client (cmd/usearch-api), not Python
  sidecars; DEEP-001's Python container follows the researcher Python
  convention (`LITELLM_API_KEY` inside the container) which is
  consistent with REQ-LLM-005's intent (the master-key value reaches
  the upstream proxy as Bearer auth) without requiring identical
  env-var names inside the container.

> **[SUPERSEDED v0.2]** The previous draft suggesting
> `OPENAI_API_KEY = LITELLM_MASTER_KEY` aliasing inside the storm
> container is superseded. v0.3 (evidence-first amendment) further
> refined this: the storm Python container reads `LITELLM_API_KEY`
> (matching researcher gateway.py:27 verbatim), NOT
> `LITELLM_MASTER_KEY` directly. The host's `${LITELLM_MASTER_KEY}`
> is aliased into `LITELLM_API_KEY` via docker-compose.

### 2.5 `/deep` CLI and intent-router contract

`SPEC-IR-001` (implemented) classifies queries into
`{web, social, academic, korean, mixed, unknown}`. The `/deep`
verb is **CLI-side**: invoking `usearch deep "..."` triggers the
deep-research path which:

1. Performs intent-router classification (SPEC-IR-001) to pick adapter
   set + lang.
2. Invokes fanout (SPEC-FAN-001) to gather a doc corpus.
3. Calls the new STORM sidecar with the corpus + query to produce a
   long-form report.
4. Streams the report via SSE (SPEC-SYN-004 wire format extended for
   sections).

DEEP-001 owns step 3 only. Steps 1, 2, and 4 are pre-existing
contracts that DEEP-001 must respect verbatim.

---

## 3. Upstream STORM Library Survey

### 3.1 Project identification

- Repository: `https://github.com/stanford-oval/storm`
- Paper: "Assisting in Writing Wikipedia-like Articles From Scratch
  with Large Language Models" (Shao et al., NAACL 2024)
- PyPI package: `knowledge-storm`
- Latest stable (verify via WebFetch in run phase): see PyPI badge in
  upstream README; project follows `0.x` versioning.
- License: MIT (verify via WebFetch in run phase per
  anti-hallucination policy)
- Active maintenance: last commit and last release dates SHALL be
  verified in run phase via WebFetch on the PyPI page; if upstream is
  unmaintained for >12 months we pin a known-good commit SHA.

[HARD] All claims about specific version numbers, public class names,
and method signatures in this section are **descriptions of the
expected upstream surface based on the published paper and known
patterns**; the run-phase implementation MUST verify each via
`pip show knowledge-storm` and reading the installed package's
`__init__.py`. If the installed surface differs, the SPEC's
"in-scope" lifts may need adjustment via amendment.

### 3.2 Core concepts (per published paper)

STORM is a **knowledge-grounded long-form generation** pipeline. Two
phases:

**Phase 1: Pre-Writing** (research)
- Generate a diverse set of perspectives by simulating
  Wikipedia-editor-style debate
- For each perspective, conduct a multi-turn conversation between a
  curious "questioner" and a "knowledgeable" agent (the latter cites
  retrieved sources)
- Aggregate the conversation into an outline (section + subsection
  hierarchy)

**Phase 2: Writing**
- For each section, retrieve relevant sources (using the conversation
  context as query expansion)
- Generate section text grounded in retrieved sources, with inline
  citations
- Concatenate all sections into a final report
- Optionally apply a polish/editor pass

### 3.3 Public Python surface — VERIFIED (audit-driven amendment v0.2)

VERIFIED 2026-05-10 via WebFetch on
`https://pypi.org/project/knowledge-storm/` and
`https://github.com/stanford-oval/storm/blob/main/README.md`.
Pinned: `knowledge-storm == 1.1.1`. Minimum Python: ≥3.10.

Verified entry point from upstream README:

```python
from knowledge_storm import (
    STORMWikiRunner,
    STORMWikiRunnerArguments,
    STORMWikiLMConfigs,
)
# v1.1.0+ introduced LiteLLM integration — preferred over generic OpenAIModel
from knowledge_storm.lm import LitellmModel
from knowledge_storm.rm import YouRM, BingSearch  # retrieval modules (DEEP-001 injects custom RM)

lm_configs = STORMWikiLMConfigs()
lm_configs.set_conv_simulator_lm(LitellmModel(model="claude-haiku-4-5", max_tokens=500, api_base=os.environ["LITELLM_BASE_URL"], api_key=os.environ["LITELLM_API_KEY"]))
lm_configs.set_question_asker_lm(LitellmModel(...))
lm_configs.set_outline_gen_lm(LitellmModel(...))
lm_configs.set_article_gen_lm(LitellmModel(model="claude-sonnet-4-6", max_tokens=3000, api_base=os.environ["LITELLM_BASE_URL"], api_key=os.environ["LITELLM_API_KEY"]))
lm_configs.set_article_polish_lm(LitellmModel(...))

args = STORMWikiRunnerArguments(
    output_dir="...",
    max_conv_turn=3,
    max_perspective=3,
    search_top_k=3,
    max_thread_num=3,
)

runner = STORMWikiRunner(args, lm_configs, rm)
runner.run(topic="...", do_research=True, do_generate_outline=True,
           do_generate_article=True, do_polish_article=True)
```

Critical observation for DEEP-001:
- STORM expects a **retrieval module (`rm`)** to fetch sources. The
  default upstream RMs hit external search APIs. **We MUST inject a
  custom RM that returns the doc corpus assembled by SPEC-FAN-001
  upstream of the sidecar call** — STORM's own retrieval is not
  invoked; the sidecar receives a pre-assembled `docs[]` array
  identical in shape to `services/researcher/`'s payload.
- STORM produces sources cited by **URL string**, not by a stable
  `doc_id`. The sidecar MUST translate STORM's URL-cited output into
  our `doc_id`-cited output (REQ-DEEP1-002 below).

### 3.4 Output format (expected; verify in run phase)

STORM writes section-structured text, typically Markdown-like, with
inline citations such as `[1]`, `[2]`, ... pointing to a references
list at the end. Section structure (`# Title`, `## Section`,
`### Subsection`) is preserved.

For DEEP-001 we adopt a **structured JSON response** rather than raw
markdown — the sidecar parses STORM's output into a sectioned schema
that the Go-side can stream section-by-section without re-parsing
markdown on the wire.

Proposed response shape (for SPEC author review):

```jsonc
{
  "request_id": "...",
  "title": "...",
  "sections": [
    {
      "section_index": 0,
      "heading": "Introduction",
      "level": 1,
      "text": "...",                    // full section text with [N] markers
      "sentences": [                    // pre-segmented for streaming
        { "sentence_index": 0, "text": "...", "markers": [1, 2] },
        ...
      ]
    },
    ...
  ],
  "citations": [
    { "marker": 1, "doc_id": "...", "url": "...", "title": "..." },
    ...
  ],
  "model": "...",
  "provider": "...",
  "cost_usd": 1.23,
  "prompt_tokens": 12345,
  "completion_tokens": 6789,
  "latency_ms": 145000,
  "degraded": false,
  "notice": "",
  "schema_version": 1
}
```

Sentence-level pre-segmentation (using the SPEC-SYN-002 canonical
regex `[.!?。！？]\s+|[.!?。！？]$`) is performed sidecar-side so the
Go-side streamer can emit per-sentence SSE without re-splitting.

### 3.5 Operational characteristics (from paper + community reports)

- **Latency**: a full STORM run (research + outline + article +
  polish) on a single topic at default settings (3 perspectives × 3
  conv turns × 3 search results) takes 60-180 seconds typically;
  paper reports 5+ minute runs for complex topics. Upper bound for
  DEEP-001 budget: **5 minutes (300 s)** per `roadmap.md` M5 exit.
- **Token cost**: paper reports ~150k–400k tokens per article (sum of
  input + output across all calls). At Haiku 4.5 pricing (~$1/Mtok
  input, $5/Mtok output) this is $0.30-$2 per report. SPEC-DEEP-004
  will impose per-user/per-day caps; DEEP-001 must accept a
  per-call cap parameter.
- **Concurrency**: STORM's `max_thread_num` defaults to 3; thread-pool
  parallelism for the perspective-loop. We pass through this knob.
- **Determinism**: outputs are non-deterministic by design (LLM
  sampling). Tests MUST mock the underlying LM, not assert on
  generated text content.

### 3.6 Python dependency surface

Expected major deps (verify in run phase via `uv pip list`):
- `dspy-ai` (LM wrapper layer used by knowledge-storm)
- `langchain-text-splitters` (or similar; verify)
- `tqdm`, `httpx`, `pydantic`, `numpy`, `scikit-learn`
- `wikipedia-api`, `trafilatura` (HTML extraction; bundled with
  default RM but unused in our injected-RM mode)

For DEEP-001, the sidecar `pyproject.toml` MUST pin
`knowledge-storm` to a specific minor version and declare the
direct deps explicitly (no transitive surprises). Run phase will
populate a verified pin.

### 3.7 Korean-language considerations

- knowledge-storm's outline generation prompt is English-biased per
  upstream README. Korean queries may produce English outlines
  unless system prompts are localized.
- Sentence segmentation by our SYN-002 canonical regex
  (`[.!?。！？]\s+|[.!?。！？]$`) handles Korean punctuation; mecab-ko
  (SPEC-IDX-003) is NOT required at the segmentation layer — it is a
  retrieval/tokenization concern, not a generation concern.
- The `lang` hint in the Request payload SHALL be threaded to STORM's
  LM configs as a system-prompt directive ("respond in Korean"),
  identical to SPEC-SYN-001's NFR-SYN-007 `lang` plumbing.
- Empirical risk: STORM's perspective simulator may produce
  English-only conversations even for Korean queries; mitigation is
  to localize the conv-simulator system prompt with a Korean
  directive when `lang == "ko"`. This is a run-phase prompt-tuning
  task documented under Risk 6.

### 3.8 Citation faithfulness vs. STORM's native citation

STORM produces inline `[N]` citations natively, but:
1. The cited references are **URLs** (or upstream-RM-specific
   identifiers), not our stable `NormalizedDoc.ID`.
2. STORM's citation density varies — some sentences are uncited, some
   sections may have few citations.
3. STORM's article-polish stage may rewrite text in ways that drop
   citation markers.

Therefore DEEP-001 MUST apply a **citation faithfulness gate
identical in contract to SPEC-SYN-002**:
- Each sentence in the long-form output SHALL carry at least one
  valid `[N]` marker that resolves to a `doc_id` in the input docs
  array.
- Marker → `doc_id` resolution happens via URL match (STORM's `[N]`
  → URL → `doc.URL` → `doc.ID`).
- The `RESEARCHER_FAITHFULNESS_MODE` env-var (`strip` | `reject` |
  `off`) governs handling of un-cited sentences. Default is `strip`
  for best UX (matches SYN-002).
- Long-form-specific concession: at the **section** level, the
  section MAY contain a non-trivial fraction of un-cited
  introductory/transitional prose ("The rest of this section will
  cover..."). v0 applies the gate to **claim sentences only**;
  defining "claim sentence" sidesteps semantic territory by deferring
  to SPEC-EVAL-001. v0 conservative behavior: gate every sentence;
  empirical strip rate informs whether a heuristic is needed.

This is the **single most load-bearing consistency contract** between
DEEP-001 and the M4 hardening work.

---

## 4. Streaming Integration with SPEC-SYN-004

DEEP-001 streaming flows through the existing SSE infrastructure but
extends the event vocabulary:

| Event Type | Source | When Emitted |
|------------|--------|--------------|
| `event: section_start` | NEW | At the start of each section |
| `event: sentence` | EXISTING (SYN-004) | Per-sentence within a section |
| `event: section_done` | NEW | At the end of each section |
| `event: done` | EXISTING (SYN-004) | At the end of the report |
| `event: error` | EXISTING (SYN-004) | On any failure path |

`section_start` payload: `{request_id, section_index, heading, level,
schema_version: 1}`
`sentence` payload: same as SYN-004 plus `section_index` field
`section_done` payload: `{request_id, section_index,
sentences_emitted, schema_version: 1}`
`done` payload: same as SYN-004 plus `total_sections, total_sentences`

This is an **additive** extension. Pre-DEEP-001 SYN-004 clients (CLI
v0) ignore unknown event types per SSE spec — backward compatible.

The buffered-then-streamed mode of SYN-004 is preserved for v0:
sidecar produces the full report; Go-side `internal/streamsynth`
walks the structured response and emits per-section + per-sentence
events.

---

## 5. Risks

### Risk 1: Upstream STORM library is heavyweight and slow to install

**Likelihood**: Medium-High. `knowledge-storm` pulls `dspy-ai`,
scientific Python stack (`numpy`, `scikit-learn`), `wikipedia-api`,
`trafilatura`. Cold install on CI may take 60-120 s.

**Mitigation**:
1. Pin specific versions in `pyproject.toml`.
2. Use uv with lockfile for deterministic installs.
3. Consider Docker layer caching for the sidecar image.
4. Document install time in README; CI uses `uv pip install --system`
   with cache.

### Risk 2: Citation marker ↔ doc_id translation gap

**Likelihood**: High. STORM emits URLs, not `doc_id`s. If the
URL-to-`doc_id` map fails to resolve a citation (e.g., STORM
hallucinates a URL not in the input corpus, or normalizes the URL
differently), the citation is functionally un-cited.

**Mitigation**:
1. Apply URL canonicalization (strip query strings, trailing slashes,
   protocol normalization) on both sides before lookup.
2. On unresolved-URL: fall through to faithfulness mode handler
   (strip the sentence under default `mode=strip`).
3. Emit a counter `usearch_deep_unresolved_citations_total` for
   operational visibility.
4. Document this as the most likely cause of high strip rates.

### Risk 3: Latency exceeds 5-min budget for some topics

**Likelihood**: Medium. STORM's perspective + conversation loop is
LLM-call-heavy. Complex topics or high `max_perspective` × `max_conv_turn`
products push latency into 5-10 min territory.

**Mitigation**:
1. Default to conservative knobs:
   `max_perspective=2, max_conv_turn=2, search_top_k=3,
   max_thread_num=3` (below upstream defaults).
2. Hard ctx deadline at `STORM_MAX_LATENCY_MS` (default 300 000 =
   5 min) on the Go-side. On deadline exceeded: cancel, emit
   `event: error` with `error_code: "deadline_exceeded"`,
   counter `outcome=deadline_exceeded`.
3. Per-call cap parameter (`max_perspective_override`,
   `max_conv_turn_override`) so callers can dial down for
   latency-sensitive paths. SPEC-DEEP-004 will tie this to per-user
   quota.

### Risk 4: Token cost runaway

**Likelihood**: Medium. A single STORM run can consume 100k-400k
tokens; without caps, a single user could rack up $1-10 per report.

**Mitigation**:
1. Per-call token cap `STORM_MAX_TOKENS_PER_CALL` (default 500 000)
   passed to LiteLLM's per-request budget. On cap exceeded: LiteLLM
   surfaces an error; sidecar returns degraded response.
2. Per-call cost cap `STORM_MAX_COST_USD` (default 2.50 = ~5x
   typical Haiku 4.5 cost). Wraps SPEC-LLM-001's `LITELLM_BUDGET_USD`.
3. Detailed per-stage cost tracking (research / outline / article /
   polish) emitted via existing `usearch_synthesis_cost_usd_total`
   metric family — extended with a new label dimension only via the
   cardinality allowlist (run-phase decision).
4. SPEC-DEEP-004 owns the per-user/per-day enforcement; DEEP-001
   only owns the per-call ceiling.

### Risk 5: Citation faithfulness rate too low to be useful

**Likelihood**: Medium-High. STORM's article-polish stage rewrites
text and can drop markers. Even without polish, the conv-simulator's
output has variable citation density.

**Mitigation**:
1. Disable polish by default (`do_polish_article=False`) — empirics
   in run phase will determine whether to re-enable.
2. Apply faithfulness gate per-section (not just per-document) so
   strip mode produces well-formed sections (no orphaned section
   headings with empty body).
3. Operational counter `usearch_deep_faithfulness_strip_rate` aimed
   at <30%; if higher, prompt-tuning becomes a follow-up.
4. Acceptance criterion: STORM-style report with **≥10 cited sources**
   per `roadmap.md` line 154. The 10-source bar is below typical
   STORM output (~20-30 sources); achievable even with conservative
   strip rates.

### Risk 6: Korean-language output quality

**Likelihood**: Medium. STORM is English-biased; Korean queries may
produce English-only outlines or mixed-language conversations even
when the final article is Korean.

**Mitigation**:
1. System-prompt localization in conv-simulator and outline-gen LM
   configs when `lang == "ko"`.
2. Acceptance check for Korean-locale long-form (deferred to
   SPEC-EVAL-003 in M8); DEEP-001 v0 ships English-first with Korean
   as best-effort.
3. Monitor counter
   `usearch_deep_outcomes_total{outcome=...,lang="ko"}` for parity
   with `lang="en"` strip rates.

### Risk 7: Streaming back-pressure on long reports

**Likelihood**: Low-Medium. A 5-min upstream call followed by a
buffered-streamed emission of (say) 50 sentences over a slow client
connection could trip SYN-004's `SYN004_SSE_WRITE_TIMEOUT_MS` (5 s
default).

**Mitigation**:
1. Inherit SYN-004's REQ-SYN4-006 (write-timeout) handling verbatim
   — DEEP-001 streaming reuses `internal/sse.Writer` and inherits
   the per-write deadline.
2. Document the worst case (client-stall mid-section) in operator
   docs.

### Risk 8: knowledge-storm version drift / abandonment

**Likelihood**: Medium. The upstream is a research project (Stanford
OVAL); maintenance velocity may slow.

**Mitigation**:
1. Pin to a specific version SHA in `pyproject.toml`; no `>=` ranges.
2. Vendored fork plan: if upstream becomes unavailable, fork to a
   vendored path under `vendor/python/` (run-phase decision; not in
   v0 scope).
3. Run-phase verification of upstream URL via WebFetch (per
   anti-hallucination policy).

---

## 6. Summary of Top 3 Risks for SPEC Author Attention

1. **Citation marker ↔ doc_id translation gap** — STORM emits URLs,
   we want stable `doc_id`s; URL canonicalization + unresolved-URL
   fallback to faithfulness-mode handler. Without this,
   faithfulness gate strips the entire long-form output.
2. **Latency exceeds 5-min budget** — conservative default knobs +
   `STORM_MAX_LATENCY_MS=300000` ctx ceiling + per-call override
   knobs. SPEC-DEEP-004 layers per-user quota; DEEP-001 owns the
   per-call ceiling.
3. **Token cost runaway** — per-call cap `STORM_MAX_COST_USD=2.50`
   plus per-stage cost tracking. SPEC-DEEP-004 will tie to per-user
   per-day cap.

---

## 7. References

### Internal (file:line)

- `services/researcher/src/researcher/synthesis.py:153-220` — sidecar
  pattern (synthesize entry point) being mirrored
- `services/researcher/src/researcher/faithfulness.py` — SPEC-SYN-002
  gate; same contract applied to long-form
- `services/researcher/src/researcher/app.py:49-81` — FastAPI
  endpoint + UncitedOutputError handler being mirrored
- `services/researcher/src/researcher/gateway.py:36-72` — LiteLLM
  client pattern; STORM's LM-config layer wires the same way
- `services/researcher/src/researcher/obs.py` — JSON log + counter
  pattern being mirrored
- `services/researcher/src/researcher/models.py:55-63` — `Citation`
  Pydantic shape (REUSED for long-form)
- `internal/synthesis/types.go:9-58` — Go-side Request/Result/Citation
  shapes (the long-form variant is parallel, not extension)
- `internal/synthesis/client.go` — Go-side `Client.Synthesize`; the
  long-form `internal/deepreport.Client.GenerateReport` mirrors the
  retry+ctx+observability pattern verbatim
- `internal/sse/writer.go` — SSE writer reused by long-form streamer
- `internal/streamsynth/streamsynth.go` — sentence segmentation and
  per-event emission; extended with section-aware emission for
  long-form
- `pkg/types/normalized_doc.go:40-58` — `NormalizedDoc` schema
  (input to STORM via injected RM)
- `services/storm/` — empty skeleton (Dockerfile, pyproject.toml,
  README.md, src/storm/__init__.py); v0 build site
- `services/storm/.env.example` — `LITELLM_API_KEY` (in-container
  API-key env, matches `services/researcher/src/researcher/gateway.py:27`
  literal `api_key = os.environ.get("LITELLM_API_KEY", "")`),
  `LITELLM_MASTER_KEY` documented as host-side var that
  docker-compose aliases into the container's `LITELLM_API_KEY` per
  `deploy/docker-compose.yml:173` `LITELLM_API_KEY: ${LITELLM_MASTER_KEY}`,
  and `LITELLM_BASE_URL=http://litellm:4000` (matches
  `services/researcher/src/researcher/gateway.py:26` default literal —
  the `http://localhost:4000/v1` form previously documented in this
  file was inconsistent with the researcher and has been reconciled
  in the v0.2/v0.3 amendments)
- `.moai/specs/SPEC-SYN-001/spec.md` — citation marker contract
  (NFR-SYN-002 marker → doc_id mapping invariant)
- `.moai/specs/SPEC-SYN-002/spec.md:159-163` — faithfulness contract
  (REQ-SYN2-001..005); REUSED verbatim by DEEP-001
- `.moai/specs/SPEC-SYN-004/spec.md:226-234` — SSE wire format
  (REQ-SYN4-001a..c, REQ-SYN4-002..006, NFR-SYN4-001..003); REUSED +
  EXTENDED by DEEP-001
- `.moai/specs/SPEC-LLM-001/spec.md:116-122` — LiteLLM proxy contract
  (REQ-LLM-001..007); HARD constraint for STORM LM configs
- `.moai/specs/SPEC-IR-001/spec.md` — `/deep` verb classification
  precondition; `RoutingDecision.Lang` flows into long-form `lang`
- `.moai/specs/SPEC-FAN-001/spec.md` — fanout produces the doc
  corpus consumed by the injected RM
- `.moai/project/roadmap.md:74` — SPEC-DEEP-001 row
- `.moai/project/roadmap.md:154` — M5 exit criterion (≥10 cited
  sources, ≤5 min)

### External (verify URLs via WebFetch in run phase per
anti-hallucination policy)

- `https://github.com/stanford-oval/storm` — upstream STORM
  repository; verify license, latest stable version, install
  instructions
- `https://pypi.org/project/knowledge-storm/` — PyPI page; verify
  current version + dep graph
- Shao et al. 2024 (NAACL) "Assisting in the Writing of
  Wikipedia-like Articles From Scratch with Large Language Models"
  — `https://arxiv.org/abs/2402.14207` — the STORM paper; verify
  pipeline architecture description
- `https://github.com/stanfordnlp/dspy` — dspy-ai LM-wrapper library
  (transitive dep) — verify `dspy.LM` interface matches our
  LiteLLM-backed config

---

*End of SPEC-DEEP-001 research v0.1*
