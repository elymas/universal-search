---
id: SPEC-DEEP-001
version: 0.3.1
status: in-progress
created: 2026-05-10
updated: 2026-06-24
author: limbowl
priority: P0
issue_number: 0
title: STORM long-form report sidecar
milestone: M5 — /deep multi-agent
owner: expert-backend
methodology: tdd
coverage_target: 85
depends_on: [SPEC-SYN-001, SPEC-SYN-002, SPEC-SYN-004, SPEC-IR-001, SPEC-FAN-001, SPEC-LLM-001, SPEC-CORE-001]
blocks: [SPEC-DEEP-002, SPEC-DEEP-003, SPEC-DEEP-004]
---

# SPEC-DEEP-001: STORM long-form report sidecar

## HISTORY

- 2026-05-10 (iter-3 cleanup v0.3.1, limbowl via manager-spec):
  plan-auditor APPROVE-WITH-CHANGES verdict 0.82. Applied 5 small
  fixes flagged by the iter-3 audit:
  (N1 MAJOR) spec.md §2.1(p) `depends_on` reconciled to YAML map form
  `depends_on: { litellm: { condition: service_healthy } }` matching
  plan.md M9 and the researcher precedent at
  `deploy/docker-compose.yml:178-180`; the previous list form
  `depends_on: [litellm]` is invalid docker-compose v2/v3 syntax (the
  list form does not accept the `condition` key) and was a
  documentation defect carried over from earlier drafts.
  (N2 MINOR) plan.md frontmatter version bumped 0.1.0 → 0.3.1 with a
  HISTORY mirror entry; the previous "0.1.0 (draft)" was a stale
  artifact from the initial scaffolding pass.
  (N3 MINOR) acceptance.md frontmatter version bumped 0.1.0 → 0.3.1
  for consistency with spec.md.
  (N4 MINOR) spec-compact.md gained a `Version: 0.3.1` line under the
  title plus Status field updated `draft → approved` to mirror
  spec.md's new status.
  (N5 MINOR) Truncated REQ-LLM-005 quotes at spec.md HISTORY v0.3
  (lines 39-42), spec.md §7 References (lines 625-628), and
  research.md §2.4 (lines 181-184) annotated with
  `…[clause omitted: secret-redaction obligations]…` to mark the
  omission. Verified against SPEC-LLM-001:120 — REQ-LLM-005 has a
  second clause: "the key value SHALL NEVER appear in any slog
  record, OTel span attribute, Prometheus label, or wrapped error
  message." DEEP-001's quotes only cite the auth-mechanism clause;
  the secret-redaction clause is intentionally out-of-scope for
  DEEP-001's discussion (which centers on env-var naming
  conventions, not redaction obligations).
  Also: status transitioned `draft → approved`. No content changes
  beyond the five enumerated fixes and the status/version bumps.
  Version 0.3.0 → 0.3.1.

- 2026-05-10 (evidence-first audit-driven amendment v0.3, limbowl via
  manager-spec): Applied iter-3 plan-auditor patches (D2 re-regression,
  D12 SYN-004 precedent reframing, D14 stale allowlist amendment line,
  D15 section numbering, D16 port mapping, D17 LITELLM env-var
  convention, D18 spec-compact mirroring, D19 HISTORY line range, D20
  research.md superseded callout). Major reconciliations grounded in
  literal file evidence:
  (D17 BLOCKER) Adopted Option A — mirror researcher fully. Verbatim
  evidence: `services/researcher/src/researcher/gateway.py:27` reads
  `api_key = os.environ.get("LITELLM_API_KEY", "")` (NOT
  `LITELLM_MASTER_KEY` directly); `deploy/docker-compose.yml:173` aliases
  `LITELLM_API_KEY: ${LITELLM_MASTER_KEY}` host→container. Storm sidecar
  Python container internally reads `LITELLM_API_KEY` (matching
  researcher gateway.py:27 verbatim); host `.env` declares
  `LITELLM_MASTER_KEY` which docker-compose interpolates into the
  container's `LITELLM_API_KEY`; the docker-compose alias is mandatory.
  SPEC-LLM-001 REQ-LLM-005 verbatim text: "The Client SHALL authenticate
  to the LiteLLM proxy via the `LITELLM_MASTER_KEY` environment variable
  sent as an `Authorization: Bearer <key>` header on every request…[clause
  omitted: secret-redaction obligations]…" —
  scope of REQ-LLM-005 is the Go-side `internal/llm` Client (cmd/usearch-api),
  NOT Python sidecars; DEEP-001's Python sidecar follows the researcher
  Python convention (LITELLM_API_KEY in container) which is consistent
  with REQ-LLM-005's intent (master key flows from host to upstream
  proxy auth) without requiring the same env-var name inside the
  container. (D16 BLOCKER) Port mapping reconciled to
  `${STORM_PORT:-8001}:8001` mirroring researcher's
  `${RESEARCHER_PORT:-8081}:8081` (verbatim docker-compose.yml:170).
  Dropped the "8001:8000" claim. Storm container internal port = 8001;
  healthcheck targets `localhost:8001/health`.
  (D2 re-regression) Retracted any claim that "matches researcher
  gateway.py:26" covers auth — gateway.py:26 only sets `LITELLM_BASE_URL`
  (`base_url = os.environ.get("LITELLM_BASE_URL", "http://litellm:4000")`);
  the auth env-var anchor is gateway.py:27.
  (D12 partial regression) Reframed NFR-DEEP1-003 disconnect wording.
  SPEC-SYN-004 NFR-SYN4-003 verbatim: "the `usearch_syn004_outcomes_total`
  counter SHALL be incremented exactly once across the entire set of
  `outcome` label values (`streamed_complete`, `client_disconnect`,
  `write_timeout`, `error_upstream`, `accept_fallback_to_json`)... the
  first terminal outcome wins via a sync.Once-style guard". SYN-004 IS
  exactly-once for client_disconnect on its OWN counter family.
  DEEP-001 deviates: SYN-004 owns the disconnect emission via
  `usearch_syn004_outcomes_total{outcome=client_disconnect}`; the DEEP
  counter `usearch_deep_outcomes_total` is zero-or-one (at-most-once)
  for disconnect, exactly-once otherwise.
  (D14) Removed stale `cardinality_test.go (allowlist amend)` line
  from spec-compact.md MODIFY section — `outcome` label NAME is
  pre-existing in `internal/obs/metrics/metrics_test.go:257` (verbatim
  `"outcome": true,` in allowlist).
  (D15) Renumbered §3 to tripartite: §3.1 Functional Requirements,
  §3.2 Status-to-Outcome Map, §3.3 Non-Functional Requirements.
  (D18) spec-compact.md observability section now mirrors the
  status-to-outcome map.
  (D19) Removed specific test line numbers from v0.2 HISTORY entry
  citation (test names rot under future edits).
  (D20) research.md OPENAI_API_KEY supersedure note converted to
  explicit `> [SUPERSEDED v0.2]` callout block.
  Version bumped 0.2.0 → 0.3.0.

- 2026-05-10 (audit-driven amendment v0.2, limbowl via manager-spec):
  Applied plan-auditor REJECT verdict 0.45 fixes (10 defects: D1, D2,
  D3, D4, D5, D6, D7, D8, D10, D12). Major reconciliations:
  (D1) Dropped the "preserves SYN-002 Python contract" framing — no
  `services/researcher/src/researcher/faithfulness.py` exists; the
  SYN-002 Python-side gate was descoped at build time and the only
  surviving SYN-002 reference is the Go-side
  `internal/streamsynth/streamsynth.go:28` regex literal. DEEP-001
  declares the canonical sentence regex as a fresh DEEP-001-owned
  literal in `services/storm/src/storm/faithfulness.py`; this is
  the single source of truth on the Python side going forward.
  (D2) Reconciled all LiteLLM env-var references to `LITELLM_BASE_URL`
  (matching `services/researcher/src/researcher/gateway.py:26`) and
  `LITELLM_MASTER_KEY` Bearer auth (per SPEC-LLM-001 REQ-LLM-005)
  in spec.md, plan.md, research.md, spec-compact.md, and .env.example.
  (D3) Pinned cardinality budget concretely against
  `internal/obs/metrics/metrics_test.go:284 TestNoUnboundedLabels`
  (label-NAME allowlist; the `outcome` label NAME is pre-existing,
  no new label name added — only 6 new VALUES pre-initialised).
  (D4) Added `max_latency_ms?` to REQ-DEEP1-001 request schema and
  explicit clamp behaviour to REQ-DEEP1-004 so Edge Cases 12/13
  trace cleanly. (D5) Distinguished Python-side per-section
  `usearch_storm_faithfulness_outcomes_total` (independent budget)
  from Go-side per-request `usearch_deep_outcomes_total`
  (exactly-once) in NFR-DEEP1-003; added explicit HTTP-status →
  outcome mapping table adjacent to REQ-DEEP1-001. (D6) Added
  `.moai/sprints/SPEC-DEEP-001-sprint-{1..N}.md` (max 5) to
  plan.md §6.1 [NEW] Files per design constitution §11.
  (D7) Promoted OQ2 to "Verified Prerequisite" using WebFetch on
  `pypi.org/project/knowledge-storm/` (v1.1.1, Python ≥3.10) and
  `github.com/stanford-oval/storm/blob/main/README.md`: classes
  `STORMWikiRunner`, `STORMWikiLMConfigs`, `STORMWikiRunnerArguments`
  confirmed; canonical import path `from knowledge_storm import ...`
  with `from knowledge_storm.lm import LitellmModel`. Pinned
  `knowledge-storm == 1.1.1`. (D8) Replaced the @MX:TODO planning
  contradiction in plan.md §3.7 with the auto-generation policy from
  mx-tag-protocol.md. (D10) Split REQ-DEEP1-004 cancellation clause
  into REQ-DEEP1-004a (Python sidecar httpx pool / asyncio task) and
  REQ-DEEP1-004b (Go client goroutine / response-body close).
  (D12) Reconciled NFR-DEEP1-003 with Edge Case 10 by adopting the
  SYN-004 NFR-SYN4-003 precedent: "exactly-once for non-disconnect
  terminal states; at-most-once when client disconnects mid-stream".
  Version bumped 0.1.0 → 0.2.0.

- 2026-05-10 (initial draft v0.1, limbowl via manager-spec):
  First EARS-formatted SPEC for the M5 STORM integration. Wraps the
  upstream `stanford-oval/storm` (PyPI: `knowledge-storm`) library as
  a Python sidecar at `services/storm/` mirroring the
  `services/researcher/` pattern from SPEC-SYN-001/002. Adds a NEW
  Go-side client at `internal/deepreport/` consuming a NEW
  `POST /generate_report` HTTP endpoint. Long-form output flows
  through the existing SPEC-SYN-002 citation faithfulness contract
  (REQ-DEEP1-003) and through the existing SPEC-SYN-004 SSE wire
  format extended with `event: section_start` and
  `event: section_done` (REQ-DEEP1-005). LiteLLM proxy
  (SPEC-LLM-001) is the only LLM access path — STORM's `lm` configs
  point at `LITELLM_BASE_URL`. Per-call latency cap
  `STORM_MAX_LATENCY_MS` (default 300 000) and per-call cost cap
  `STORM_MAX_COST_USD` (default 2.50) provide the v0 budget envelope
  ahead of SPEC-DEEP-004's per-user/per-day enforcement. /deep CLI
  routing happens via the existing intent-router (SPEC-IR-001) and
  fanout (SPEC-FAN-001); DEEP-001 owns only the long-form report
  generation phase. Companion research artifact at
  `.moai/specs/SPEC-DEEP-001/research.md` — upstream STORM analysis,
  sidecar mirror of researcher/, M4 contract integration map, Korean
  considerations, 8-risk register. 6 EARS REQs (5 × P0 + 1 × P1)
  covering all five EARS patterns. 3 NFRs. Explicitly defers
  per-user/per-day quota to SPEC-DEEP-004 (M5 same milestone),
  multi-agent pipeline to SPEC-DEEP-002, tree exploration to
  SPEC-DEEP-003, semantic faithfulness scoring to SPEC-EVAL-001
  (M8). No GitHub issue tracking on this SPEC (`issue_number: 0`).
  Ready for plan-auditor review and annotation cycle.

---

## Implementation Status (2026-06-24 audit correction)

The STORM sidecar service and `NewDeepHandler` (storm + agents modes, SSE/buffered
streaming) are fully implemented and unit-tested (72/72 Go tests pass).

Deferred — HTTP entrypoint unmounted: `cmd/usearch-api/handlers/deep.go:40`
`NewDeepHandler` is built but never registered on the mux in
`cmd/usearch-api/main.go` (which registers only `/`, `/query/stream`, `/api/admin/*`).
The `/deep` surface is therefore unreachable end-to-end.

Remediation path: wiring requires an `llm.Client` (LITELLM) + a real `FanoutFn`
(DEEP-003 Phase E) + mounting `NewDeepHandler` on the usearch-api mux + a storm sidecar
client; tracked for a future implementation pass.

---

## 1. Purpose

`.moai/project/roadmap.md` line 74 declares M5 SPEC-DEEP-001:

> STORM integration | wrap STORM as Python service, long-form report
> generation | expert-backend

M5 milestone exit criterion (`roadmap.md` line 154):

> `usearch deep "..."` returns STORM-style report with ≥10 cited
> sources in ≤5 min.

M4 (Basic Synthesis Hardening) is implemented as of 2026-05-09 — the
`/synthesize` Python sidecar produces single-paragraph synthesized
output with per-sentence citation faithfulness (SPEC-SYN-002) and
sentence-granular SSE streaming (SPEC-SYN-004). DEEP-001 introduces
the **long-form** generation surface: a multi-section, multi-paragraph
report grounded in the same input doc corpus, produced by wrapping the
upstream `stanford-oval/storm` (PyPI `knowledge-storm`) library as a
Python sidecar at `services/storm/`.

The new sidecar mirrors `services/researcher/` verbatim in shape
(FastAPI app, LiteLLM gateway, Pydantic models, JSON-log
observability), differing only in domain logic: instead of one LLM
call producing one paragraph, the sidecar drives STORM's
research-then-write pipeline producing a structured sectioned
response with per-claim `[N]` citations resolved against
`NormalizedDoc.ID`.

DEEP-001 is the **first** of four M5 SPECs. It establishes the
generation surface and core contracts. DEEP-002 (multi-agent
pipeline), DEEP-003 (tree exploration), and DEEP-004 (quota +
cost guard) layer on top and are explicitly out of scope here.

The long-form output MUST flow through:

- **SPEC-SYN-002 citation faithfulness contract** verbatim — every
  emitted sentence carries at least one valid `[N]` marker resolving
  to a `doc_id`; un-cited sentences are stripped (default), rejected,
  or bypassed per `RESEARCHER_FAITHFULNESS_MODE` (renamed at the
  long-form layer to `STORM_FAITHFULNESS_MODE` for service-scoped
  control).
- **SPEC-SYN-004 SSE wire format** verbatim, extended with two new
  event types (`section_start`, `section_done`) — clients unaware of
  the new types ignore them per W3C SSE spec.
- **SPEC-LLM-001 LiteLLM proxy** verbatim — STORM's `lm` configs
  route through `LITELLM_BASE_URL`; no direct vendor SDKs.

Completion delivers an SSE-streamable long-form report endpoint
producing ≥10 cited sources per topic in ≤5 min p95, callable from
the `usearch deep` CLI verb via the existing /deep routing path
established by SPEC-IR-001 + SPEC-FAN-001.

---

## 2. Scope

### 2.1 In-Scope

| Ref | Item |
|-----|------|
| a | [NEW] `services/storm/src/storm/` Python sidecar package mirroring `services/researcher/src/researcher/` layout: `__init__.py`, `__main__.py` (uvicorn entry), `app.py` (FastAPI router + endpoint registration + exception handlers), `gateway.py` (LiteLLM-proxy LM config factory; injectable `dspy.LM`), `models.py` (Pydantic Request/Response/Section/Sentence/Citation shapes), `obs.py` (JSON log + counter emission), `pipeline.py` (STORM orchestration: builds injected retrieval module, configures LMs, invokes `STORMWikiRunner`, parses output to structured response), `faithfulness.py` (sentence segmentation + per-sentence citation gate; reuses SPEC-SYN-002 canonical regex). |
| b | [NEW] `services/storm/src/storm/inject_rm.py` — custom `dspy.Retrieve` (or upstream-equivalent) module that returns the pre-assembled `docs[]` array from the request payload, bypassing STORM's default RMs. The injected RM is a thin adapter: input = sub-query string, output = top-k `(url, title, snippet, body)` tuples; selection is by simple lexical match against the input docs (since fanout already pre-filtered relevance). |
| c | [NEW] `services/storm/src/storm/citation_translator.py` — translates STORM's URL-cited inline `[N]` markers to our `doc_id`-cited markers. Algorithm: (1) collect STORM's references list at end of report; (2) canonicalize each URL (strip query, trailing slash, lowercase host); (3) match against `doc.URL` (also canonicalized); (4) on match, replace `[N]` → `[M]` where `M` is the canonical 1-indexed marker into a fresh `Citation` array; (5) on non-match, log + emit unresolved-citation counter and treat the sentence as un-cited (handed to the faithfulness gate). |
| d | [NEW] `services/storm/tests/` — `test_app.py` (HTTP-level integration), `test_pipeline.py` (mocked-LM unit tests over `pipeline.py`), `test_inject_rm.py`, `test_citation_translator.py`, `test_faithfulness.py` (long-form variant; can import + reuse the researcher faithfulness module via shared dep), `test_obs.py`. |
| e | [NEW] `services/storm/Dockerfile` — multi-stage Python 3.11 slim base mirroring `services/researcher/Dockerfile`; installs uv, syncs lockfile, runs `uvicorn storm.__main__:app`. |
| f | [MODIFY] `services/storm/pyproject.toml` — populate dependencies: `knowledge-storm` (pinned exact version, run-phase verified), `dspy-ai` (pinned), `fastapi`, `uvicorn[standard]`, `httpx`, `pydantic>=2`, plus dev deps already declared. |
| g | [MODIFY] `services/storm/.env.example` — reconcile to `LITELLM_BASE_URL=http://litellm:4000` (matches `services/researcher/src/researcher/gateway.py:26` literal: `base_url = os.environ.get("LITELLM_BASE_URL", "http://litellm:4000")`) and `LITELLM_API_KEY` for the in-container API-key env var (matches `services/researcher/src/researcher/gateway.py:27` literal: `api_key = os.environ.get("LITELLM_API_KEY", "")` — the Python sidecar reads `LITELLM_API_KEY` inside the container, NOT `LITELLM_MASTER_KEY` directly; `LITELLM_MASTER_KEY` is the **host-side** env var that `deploy/docker-compose.yml` aliases into the container's `LITELLM_API_KEY` per the researcher precedent at line 173: `LITELLM_API_KEY: ${LITELLM_MASTER_KEY}`); document `LITELLM_MASTER_KEY` in the .env.example as the host-side var that docker-compose substitutes into the container's `LITELLM_API_KEY`. SPEC-LLM-001 REQ-LLM-005 governs the Go-side `internal/llm` Client (cmd/usearch-api) which reads `LITELLM_MASTER_KEY` directly; the Python sidecar's container-internal `LITELLM_API_KEY` is consistent with REQ-LLM-005's intent (the master key value reaches the upstream LiteLLM proxy as Bearer auth) without requiring identical env-var names inside the container; add `STORM_MAX_LATENCY_MS=300000`, `STORM_MAX_COST_USD=2.50`, `STORM_MAX_PERSPECTIVES=2`, `STORM_MAX_CONV_TURNS=2`, `STORM_SEARCH_TOP_K=3`, `STORM_MAX_THREAD_NUM=3`, `STORM_DO_POLISH=false`, `STORM_FAITHFULNESS_MODE=strip` (values: strip, reject, off), `STORM_MODEL_OUTLINE` (default `claude-haiku-4-5`), `STORM_MODEL_ARTICLE` (default `claude-sonnet-4-6`); preserve existing `STORM_LOG_LEVEL`. |
| h | [NEW] `services/storm/README.md` enhancement — operator quickstart: docker run, env vars, sample curl request to `/generate_report`, expected SSE event vocabulary documentation. |
| i | [NEW] `internal/deepreport/` Go package — `client.go` (HTTP client mirroring `internal/synthesis/client.go`: `Client.GenerateReport(ctx, req) (Report, error)`, retry+ctx+observability), `types.go` (Go-side `Request`, `Report`, `Section`, `Sentence`, `Citation`, error sentinels `ErrInvalidRequest`, `ErrSidecarUnreachable`, `ErrTimeout`, `ErrBudgetExceeded`, `ErrDeadlineExceeded`), `config.go` (env-var driven base URL `STORM_SIDECAR_URL`, default `http://localhost:8001`, request timeout default 360 s = 5 min + 60 s slack), `client_test.go`. |
| j | [MODIFY] `internal/streamsynth/` — extend with section-aware emission. Specifically: a new `StreamLongFormReport(ctx, w *sse.Writer, report deepreport.Report) (StreamStats, error)` function that walks `report.Sections`, emits `event: section_start` per section, then per-sentence `event: sentence` events (reusing existing logic), then `event: section_done` per section, then a single `event: done` at the end. The existing single-paragraph `StreamSynthesize` function is unchanged. |
| k | [NEW] SSE event schema extensions (versioned via existing `schema_version: 1` already on SYN-004 events): `event: section_start` payload `{request_id, section_index, heading, level, schema_version: 1}`; `event: section_done` payload `{request_id, section_index, sentences_emitted, schema_version: 1}`; existing `event: sentence` payload extended with `section_index` field (additive; pre-DEEP-001 SYN-004 clients ignore unknown fields per JSON convention); existing `event: done` payload extended with `total_sections` and `total_sentences` fields (additive). |
| l | [NEW] Two Prometheus collectors in `internal/obs/metrics/deepreport.go`: `DeepReportOutcomes *prometheus.CounterVec{outcome}` (label values: `success`, `deadline_exceeded`, `budget_exceeded`, `error_invalid`, `error_upstream`, `error_unresolved_citations_threshold` — 6 pre-declared values, pre-initialised per the SYN-004 `streamsynth.go:48-56` Add(0) pattern; the `outcome` label NAME is pre-existing in `internal/obs/metrics/metrics_test.go:251-272` line 257 — no cardinality allowlist amendment required) and `DeepReportLatency prometheus.Histogram` (per-call wall-clock; buckets `[5, 15, 30, 60, 120, 180, 240, 300]` seconds). |
| m | [MODIFY] `internal/obs/metrics/metrics.go` — register the two new collectors via `registerDeepReport(pr)` helper (single edit point per SPEC-OBS-001 import-boundary discipline). |
| n | [MODIFY] `internal/obs/obs.go` — re-export `obs.DeepReportOutcomes`, `obs.DeepReportLatency`. |
| o | [MODIFY] `cmd/usearch-api/` — add HTTP request handler for the long-form endpoint at `POST /deep` (or equivalent path per SPEC-IR-001's server scaffolding; the SPEC declares **behavior**, not the file structure). The handler performs Accept-header content negotiation: `text/event-stream` ⇒ invoke `streamsynth.StreamLongFormReport`; otherwise ⇒ return the buffered JSON `Report` as HTTP 200 `application/json`. |
| p | [MODIFY] `deploy/docker-compose.yml` (or equivalent in `deploy/`) — add `storm` service entry mirroring the `researcher` service block. Port mapping `${STORM_PORT:-8001}:8001` (host:container — same-port convention mirroring researcher's `${RESEARCHER_PORT:-8081}:8081` at `deploy/docker-compose.yml:170`); container internal port = 8001. Env vars include `LITELLM_BASE_URL: http://litellm:4000` and the alias `LITELLM_API_KEY: ${LITELLM_MASTER_KEY}` (mirroring researcher service block lines 172-173 verbatim — host's `${LITELLM_MASTER_KEY}` is interpolated into the container's `LITELLM_API_KEY` env var). `depends_on: { litellm: { condition: service_healthy } }` (mirroring researcher service block lines 178-180 verbatim — docker-compose v2/v3 schema requires the map form to attach the `condition: service_healthy` key; the list form `depends_on: [litellm]` is a legacy v1 shorthand that does not accept the condition key); healthcheck uses `localhost:8001/health`. |
| q | [EXISTING — UNCHANGED] `services/researcher/` Python sidecar and `internal/synthesis/` Go client are NOT modified by DEEP-001. The two services are independent; the researcher continues to handle single-paragraph `/synthesize`, storm handles long-form `/generate_report`. |
| r | [EXISTING — UNCHANGED] `pkg/types/normalized_doc.go`, `internal/fanout/`, `internal/router/`, `internal/sse/writer.go`, `internal/sse/heartbeat.go`, `internal/synthesis/types.go`, `internal/synthesis/client.go` — all consumed read-only. |

### 2.2 Out-of-Scope (Exclusions — What NOT to Build)

[HARD] This SPEC explicitly excludes the following items. Each has a
known destination SPEC; this list prevents scope creep.

- **Multi-agent Researcher → Reviewer → Writer → Verifier pipeline**
  — that is SPEC-DEEP-002's body of work. DEEP-001 is the **plain
  STORM wrapper**, not the multi-agent re-orchestration. DEEP-002
  may decide to factor STORM as one stage among many; v0 ships
  STORM as the only stage.
- **Tree exploration with configurable breadth/depth** — SPEC-DEEP-003.
  DEEP-001 takes whatever default knobs the operator configures;
  no exposed depth/breadth flexibility beyond the env-var config.
- **Per-user / per-day quota and cost cap** — SPEC-DEEP-004.
  DEEP-001 owns only the **per-call** cap (`STORM_MAX_COST_USD`
  default 2.50 USD per single report). Multi-tenant quota
  enforcement, prompt-cache reuse, and Haiku pre-screen are
  DEEP-004's territory.
- **Semantic faithfulness scoring** (does the cited doc actually
  *support* the claim?). → SPEC-EVAL-001 (M8, RAGAS / DeepEval CI
  gate at ≥0.85). DEEP-001 enforces only **structural**
  faithfulness — same scope as SPEC-SYN-002.
- **Multi-claim-per-sentence detection** — same exclusion as
  SPEC-SYN-002. → SPEC-EVAL-001.
- **Hallucinated content under valid `[N]`** — same exclusion as
  SPEC-SYN-002. → SPEC-EVAL-001.
- **Token-level streaming from STORM internals** — STORM's
  internal LM calls are buffered (single-shot per call). Streaming
  is at the **section** and **sentence** level, not at the
  intra-sentence token level. Token-level streaming is gated on a
  follow-up SPEC parallel to the SYN-006 / SYN-004-v2 sidecar
  upgrade noted in research §6 R5.
- **Modifying retrieval, fanout, or adapter behavior** — STORM's
  injected RM consumes whatever `[]NormalizedDoc` the request
  payload carries. Retrieval is upstream-of-DEEP-001
  (SPEC-FAN-001).
- **Non-LiteLLM LLM access** — direct provider SDKs are prohibited
  per SPEC-LLM-001. STORM's `dspy.LM` configs MUST route through
  `LITELLM_BASE_URL`. Bypass attempts are blocked at the
  configuration layer.
- **WebSocket transport, gRPC streaming, NDJSON streaming** — same
  exclusion as SPEC-SYN-004. Long-form streaming is SSE-only.
- **Resume / replay of in-progress reports** — no `Last-Event-ID`
  support. Each report run is a fresh client connection. Resume is
  a SPEC-IDX-005 (M6) candidate.
- **/deep CLI verb implementation in `cmd/usearch`** — DEEP-001
  ships only the sidecar + Go client + API handler. The CLI
  surface that wires `usearch deep "..."` end-to-end is owned by
  SPEC-CLI-002 (M7). DEEP-001's acceptance can be exercised via
  direct curl against `cmd/usearch-api`.
- **Markdown-on-the-wire** — the response is structured JSON, not
  raw markdown. Sections are structured `{heading, level, text,
  sentences[]}`. Clients that want markdown can reconstruct from
  `text` fields.
- **Changing single-paragraph `/synthesize` behavior** — SPEC-SYN-001/002
  contract is preserved verbatim. DEEP-001 is additive at the
  service plane.
- **GitHub Issue tracking on this SPEC** (`issue_number: 0`).

---

## 3. EARS Requirements

### 3.1 Functional Requirements

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| REQ-DEEP1-001 | Ubiquitous | The Python sidecar at `services/storm/` SHALL expose `POST /generate_report` accepting a JSON request body `{request_id, query, lang, docs: [NormalizedDocPayload], max_tokens?, max_cost_usd?, max_perspectives?, max_conv_turns?, max_latency_ms?}` (where the five `max_*` fields are optional per-call overrides bounded by the env-var ceilings; clamping per REQ-DEEP1-004) and SHALL return either a structured JSON report `{request_id, title, sections: [{section_index, heading, level, text, sentences: [{sentence_index, text, markers: [int]}]}], citations: [{marker, doc_id, url, title}], model, provider, cost_usd, prompt_tokens, completion_tokens, latency_ms, degraded, notice, schema_version: 1}` (HTTP 200) OR a structured error response `{error, detail, ...}` (HTTP 422 / 408 / 503 / 504 per error class). The sidecar SHALL also expose `GET /health` returning `{status: "ok"}` and `GET /readyz` returning `{ready: true \| false, deps: {litellm: bool, storm_lib: bool}}`. The endpoint contract is versioned via `schema_version: 1` on the response payload. | P0 | `test_post_generate_report_returns_200_with_structured_response`, `test_response_schema_version_present`, `test_health_returns_ok`, `test_readyz_reflects_dep_state`. |
| REQ-DEEP1-002 | Ubiquitous | Every `[N]` marker emitted in any `section.text` or `section.sentences[].text` field SHALL resolve to a `Citation.doc_id` value that exists in the request's input `docs[]` array (where `doc.id` matches `Citation.doc_id`). The translation from STORM's URL-cited output to our `doc_id`-cited output SHALL canonicalize URLs by (a) lowercasing the host, (b) stripping the query string, (c) stripping a single trailing slash from the path, AND (d) treating `http://` and `https://` as equivalent for matching purposes. Markers whose underlying URL fails to resolve to any input doc SHALL be removed from the text (the sentence is then handed to the faithfulness gate per REQ-DEEP1-003). The counter `usearch_storm_unresolved_citations_total` SHALL increment by the count of un-resolved markers per request. The `Citations[]` array in the response SHALL be 1-indexed, sorted by `marker`, and contain ONLY successfully resolved markers. This invariant MIRRORS SPEC-SYN-001 NFR-SYN-002 (every marker → real doc) at the long-form scale. | P0 | `test_marker_to_doc_id_resolution_via_url_canonicalization`, `test_url_canonicalization_handles_protocol_query_trailing_slash`, `test_unresolved_marker_stripped_from_text`, `test_unresolved_citations_counter_increments`, `test_citations_array_sorted_and_one_indexed`. |
| REQ-DEEP1-003 | Event-Driven | WHEN the sidecar produces a STORM report AND `STORM_FAITHFULNESS_MODE != "off"`, THEN the sidecar SHALL invoke a faithfulness gate (declaring the canonical sentence regex `[.!?。！？]\s+\|[.!?。！？]$` — English + Korean — as a fresh DEEP-001-owned literal in `services/storm/src/storm/faithfulness.py`; this is the single source of truth on the Python side. The Go-side `internal/streamsynth/streamsynth.go:28` regex is a separate symbol with the same literal string but is NOT imported by the Python sidecar) over each section's text such that every emitted sentence in the response carries at least one valid `[N]` marker resolving (per REQ-DEEP1-002) to a `doc_id`. The gate SHALL apply per-section: un-cited sentences within a section are removed under `mode=strip` (the section retains its heading + remaining cited sentences); under `mode=reject`, the entire request SHALL respond with HTTP 422 body `{error: "un_cited_long_form", detail: "<N> uncited sentence(s) across <S> section(s)", uncited_count: N, sections_affected: S}`; under `mode=off`, the gate SHALL be bypassed entirely (no gating, no counter increment). The faithfulness gate SHALL NOT trigger any retry to the LLM — long-form retry is too expensive (full STORM re-run is 60–300 s); strip is the only practical fallback. Sections that become empty (all sentences stripped) SHALL be removed from the response; the `Sections[]` array in the response contains only non-empty sections. The counter `usearch_storm_faithfulness_outcomes_total{outcome}` (label values: `accepted`, `stripped`, `rejected`, `off` — note: `off` is included here unlike SYN-002's mode=off counter bypass, because long-form is high-value-per-call enough that off-mode emission is operationally useful). | P0 | `test_faithfulness_strip_removes_uncited_sentences`, `test_faithfulness_reject_returns_422`, `test_faithfulness_off_bypasses_gate`, `test_empty_section_removed_from_response`, `test_faithfulness_outcomes_counter_increments`. |
| REQ-DEEP1-004 | Unwanted | The sidecar SHALL clamp the per-call `max_latency_ms` request override to the env-var ceiling `STORM_MAX_LATENCY_MS` (default 300 000 ms = 5 min) — i.e., `effective_deadline_ms = min(request.max_latency_ms, STORM_MAX_LATENCY_MS)`. Per-call overrides MAY tighten the deadline below the ceiling but MUST NOT exceed it; clamping above the ceiling SHALL emit a WARN-level structured log record `{request_id, reason: "per_call_override_clamped", requested_max_latency_ms, effective_max_latency_ms}` (Edge Case 13). The same clamp pattern applies to `max_cost_usd` (clamped to `STORM_MAX_COST_USD`), `max_perspectives` (clamped to `STORM_MAX_PERSPECTIVES`), `max_conv_turns` (clamped to `STORM_MAX_CONV_TURNS`), and `max_tokens` (no env-var ceiling — accepted as-is). IF a single `/generate_report` invocation exceeds the effective deadline (whether env-var ceiling or per-call clamp) measured from request acceptance, THEN the sidecar SHALL cancel the in-progress STORM run, SHALL return HTTP 504 with body `{error: "deadline_exceeded", detail: "STORM pipeline exceeded <N> ms deadline", elapsed_ms: <N>, partial_sections_completed: <S>}`, SHALL increment `usearch_deep_outcomes_total{outcome="deadline_exceeded"}` exactly once, SHALL emit one WARN-level structured log record with `{request_id, reason: "deadline_exceeded", elapsed_ms, partial_sections_completed}`, AND SHALL NOT return any partial report content in the response body (no leakage of incomplete output). The Go-side `internal/deepreport.Client` SHALL receive the 504 and return `errors.Is(err, deepreport.ErrDeadlineExceeded) == true`. Similarly, IF the cumulative `cost_usd` summed across all internal LiteLLM calls exceeds `STORM_MAX_COST_USD` USD (default 2.50) before the report is complete, THEN the sidecar SHALL cancel the run, SHALL return HTTP 402 with body `{error: "budget_exceeded", detail: "cumulative cost <X> USD exceeded cap <Y> USD", cost_usd: <X>, cap_usd: <Y>}`, SHALL increment `usearch_deep_outcomes_total{outcome="budget_exceeded"}` exactly once, AND the Go-side client SHALL return `errors.Is(err, deepreport.ErrBudgetExceeded) == true`. The two cancellation paths SHALL release all internal resources cleanly per the two sub-clauses below: **REQ-DEEP1-004a (Python sidecar runtime)** — on cancellation (deadline OR budget), the sidecar's asyncio task SHALL be cancelled cleanly via `asyncio.wait_for` propagation; the httpx `AsyncClient` connection pool SHALL return all in-flight connections to the pool (verified by absence of dangling sockets in `lsof` after cancellation); no Python thread leak (verified via `threading.enumerate()` count returning to baseline within 100 ms of cancellation). **REQ-DEEP1-004b (Go client runtime)** — on `ctx.Done()` propagation from the Go-side `internal/deepreport.Client.GenerateReport` callsite, no goroutine leak (verified via `goleak.VerifyNone(t)` at the end of each cancellation test); the HTTP response body SHALL be drained and closed via `defer resp.Body.Close()` even on error paths; no descriptor leak. The two sub-clauses are tested independently (Python via `services/storm/tests/test_caps.py`; Go via `internal/deepreport/client_test.go` with `go.uber.org/goleak`). | P0 | `test_deadline_exceeded_returns_504`, `test_deadline_exceeded_increments_counter`, `test_deadline_exceeded_no_partial_text_in_response`, `test_budget_exceeded_returns_402`, `test_budget_exceeded_increments_counter`, `test_go_client_maps_504_to_err_deadline_exceeded`, `test_go_client_maps_402_to_err_budget_exceeded`. |
| REQ-DEEP1-005 | State-Driven | WHILE the request `Accept` header advertises `text/event-stream` (case-insensitive substring match), the `cmd/usearch-api` HTTP handler SHALL invoke `internal/streamsynth.StreamLongFormReport` to emit one SSE `event: section_start` per section header (payload `{request_id, section_index, heading, level, schema_version: 1}`), then one `event: sentence` per validated sentence within that section (payload `{request_id, section_index, sentence_index, text, citations: [{marker, doc_id, url, title}], schema_version: 1}` — note the `section_index` field added for long-form), then one `event: section_done` per completed section (payload `{request_id, section_index, sentences_emitted, schema_version: 1}`), then a single terminal `event: done` (payload `{request_id, total_sections, total_sentences, latency_ms, model, provider, cost_usd, schema_version: 1}`). Heartbeats (`: ping\n\n` per `SYN004_SSE_HEARTBEAT_MS`), client-disconnect cancellation (`SYN004_DISCONNECT_CANCEL_MS`), and write-timeout (`SYN004_SSE_WRITE_TIMEOUT_MS`) SHALL inherit verbatim from SPEC-SYN-004 — DEEP-001 reuses `internal/sse.Writer` and `internal/sse.RunHeartbeat` without modification. The SPEC-SYN-002 invariant SHALL be preserved at the streaming layer: NO `event: sentence` whose `text` lacks a valid `[N]` marker SHALL reach the wire (the faithfulness gate runs sidecar-side per REQ-DEEP1-003 before the response is parsed by `streamsynth`). | P0 | `test_sse_emits_section_start_per_section`, `test_sse_emits_sentence_per_sentence_with_section_index`, `test_sse_emits_section_done_per_section`, `test_sse_emits_done_with_totals`, `test_sse_inherits_syn004_heartbeat`, `test_sse_inherits_syn004_disconnect_cancel`, `test_sse_inherits_syn004_write_timeout`, `test_sse_preserves_syn002_invariant_at_long_form_scale` (every emitted sentence carries ≥1 valid marker). |
| REQ-DEEP1-006 | Optional | WHERE the request `Accept` header does NOT advertise `text/event-stream`, the `cmd/usearch-api` handler SHALL fall back to the existing buffered JSON response shape — invoke `internal/deepreport.Client.GenerateReport`, marshal `Report` to JSON, return HTTP 200 with `Content-Type: application/json` body matching the structured response defined by REQ-DEEP1-001 verbatim. The fallback path SHALL increment `usearch_deep_outcomes_total{outcome="success"}` once on success, SHALL NOT instantiate any SSE writer or heartbeat goroutine, AND SHALL preserve full backward compatibility with non-SSE clients (test harnesses, scripted callers, future programmatic consumers). | P1 | `test_accept_missing_falls_back_to_json`, `test_accept_application_json_returns_buffered_report`, `test_accept_fallback_no_sse_overhead`, `test_accept_fallback_response_byte_identical_to_direct_sidecar_call_modulo_request_id`. |

### 3.2 Status-to-Outcome Map (Go-side counter `usearch_deep_outcomes_total`)

This table (renumbered as §3.2 in v0.3) is the canonical mapping
from HTTP response status (and the strip-threshold side-condition)
to the Go-side outcome label value. Per NFR-DEEP1-003, the Go-side
counter increments exactly once per non-disconnect request lifecycle
following this table; for the disconnect carve-out see NFR-DEEP1-003
deviation note (DEEP-001 deviates from SPEC-SYN-004 NFR-SYN4-003 in
the disconnect path):

| HTTP Status | Trigger | Outcome Label Value |
|-------------|---------|---------------------|
| 200 (default) | Successful generation | `success` |
| 200 (with strip count exceeding `STORM_UNRESOLVED_THRESHOLD`) | Strip rate exceeds reserved threshold (DEEP-001 v0 reserves the value but does NOT wire the threshold; reserved for a future SPEC per §10 D4) | `error_unresolved_citations_threshold` |
| 422 | `un_cited_long_form` (faithfulness reject mode) OR `invalid_request` (Pydantic validation failure) | `error_invalid` |
| 402 | Cumulative cost exceeded `STORM_MAX_COST_USD` (REQ-DEEP1-004) | `budget_exceeded` |
| 504 | Deadline exceeded `STORM_MAX_LATENCY_MS` or per-call clamp (REQ-DEEP1-004) | `deadline_exceeded` |
| 502, 503 | Upstream LiteLLM / STORM library failure | `error_upstream` |
| Other 5xx | Unexpected internal error | `error_upstream` |
| Client disconnect mid-stream | TCP close before terminal outcome | (no `usearch_deep_outcomes_total` increment; `usearch_syn004_outcomes_total{outcome="client_disconnect"}` increments instead per inherited SYN-004 contract) |

The 408 entry mentioned in REQ-DEEP1-001 is reserved for client-side
read-timeout handling and does NOT increment the Go-side counter
(the Python sidecar does not emit 408; client-side timeouts surface
as Go-side `ErrTimeout` mapped to `error_upstream` for counter
purposes).

### 3.3 Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| NFR-DEEP1-001 | Performance (long-form latency budget) | The end-to-end latency from `cmd/usearch-api` request acceptance to `event: done` (or final JSON response on the buffered fallback path) SHALL be **p50 ≤ 180 s, p95 ≤ 300 s** when invoked with default conservative knobs (`STORM_MAX_PERSPECTIVES=2, STORM_MAX_CONV_TURNS=2, STORM_SEARCH_TOP_K=3, STORM_MAX_THREAD_NUM=3, STORM_DO_POLISH=false`) and a doc corpus of 20–50 input docs. The TTFB to first `event: section_start` SHALL be ≤ end-to-end latency minus 5 s (i.e., the buffered-then-streamed model holds for v0 — there is no artificial delay between full-report production and stream emission). The `STORM_MAX_LATENCY_MS=300000` ctx ceiling SHALL be enforced at the sidecar; deadline-exceeded responses (REQ-DEEP1-004) are NOT a violation of this NFR — they are the documented degraded path. Detailed test method (iteration counts with mocked LM, percentile assertions) is specified in `acceptance.md` §4.4. |
| NFR-DEEP1-002 | Property: long-form citation invariant + report well-formedness | For all valid `(query, docs[])` request inputs and all non-error responses, the streamed/buffered report SHALL satisfy: (a) every `Section.text` and every `Section.sentences[].text` containing a `[N]` marker has `N` resolving to a `Citation.doc_id` in the response's `Citations[]` array; (b) every `Section.sentences[].markers[]` integer M satisfies `1 ≤ M ≤ len(Citations)`; (c) the union of all `Section.text` fields across all sections, joined by section heading + newline, structurally reconstructs the report; (d) every `Section.heading` is non-empty; (e) every `Section.sentences[]` is non-empty (empty sections are removed per REQ-DEEP1-003); (f) `report.title` is non-empty; (g) `Citations[]` is sorted by `marker`, 1-indexed, and contains exactly the markers that appear at least once in the report text. Property test via `hypothesis>=6` over a generator producing realistic STORM-shaped responses (mixed Korean + English, varying section counts, varying citation densities, varying sentence counts per section). |
| NFR-DEEP1-003 | Invariant: outcome counter exactly-once per request (with disconnect carve-out — DEEP-001 deviation from SPEC-SYN-004 NFR-SYN4-003) | **[HARD invariant]** This NFR binds two independent counter families with distinct scopes — they MUST NOT be conflated: **(A) Go-side per-request counter** `usearch_deep_outcomes_total{outcome ∈ {success, deadline_exceeded, budget_exceeded, error_invalid, error_upstream, error_unresolved_citations_threshold}}` — exactly-once for non-disconnect terminal states; **at-most-once (zero-or-one) for disconnect**. **DEEP-001 deviates from SPEC-SYN-004 NFR-SYN4-003 in the disconnect path**: SPEC-SYN-004 NFR-SYN4-003 specifies that `usearch_syn004_outcomes_total` is incremented **exactly once** across its entire `outcome` label set (`streamed_complete`, `client_disconnect`, `write_timeout`, `error_upstream`, `accept_fallback_to_json`) per request lifecycle, with `client_disconnect` as a fully-counted terminal outcome on that counter family. DEEP-001's contract differs because the SSE layer (which owns disconnect detection) is a SYN-004 inheritance: when the client disconnects mid-stream, **SYN-004 owns the disconnect emission** via `usearch_syn004_outcomes_total{outcome="client_disconnect"}` (exactly-once on that counter per SYN-004's NFR-SYN4-003 invariant); **the DEEP counter `usearch_deep_outcomes_total` is zero-or-one (at-most-once) for disconnect, exactly-once otherwise** — DEEP MAY remain at zero for the request if the disconnect arrives before any DEEP terminal outcome is committed, OR DEEP MAY increment once if a DEEP terminal outcome won the race ahead of the disconnect detection. The first terminal outcome on the DEEP counter wins via a `sync.Once`-style guard on the Go-side. The two counter families are not double-counted: a single disconnect produces at most one SYN-004 increment (`client_disconnect`) plus zero-or-one DEEP increment, never two DEEP increments. **(B) Python sidecar per-section counter** `usearch_storm_faithfulness_outcomes_total{outcome ∈ {accepted, stripped, rejected, off}}` — independent budget; one emission **per section** (not per request). For a 3-section report, this counter increments up to 3 times in a single `/generate_report` lifecycle. The Python sidecar `obs.py:emit_outcome(outcome)` function SHALL be the single emission site for **(A)** the Go-side counter and SHALL be guarded by an idempotent flag on the request-scoped context. The faithfulness counter **(B)** is per-section and is NOT guarded by the request-scoped flag. The `usearch_storm_unresolved_citations_total` counter is independent again (per-marker, no exactly-once guarantee). HTTP status → Go-side outcome mapping table is enumerated at §3.2 Status-to-Outcome Map. Tested via `test_outcome_counter_at_most_once_per_request` with race-window tests for each pair (success vs deadline; success vs budget; deadline vs budget; deadline vs error_upstream); disconnect race tests assert SYN-004 counter == 1 AND DEEP counter ∈ {0, 1} (not == 2). |

---

## 4. Acceptance Criteria

Detailed Given/When/Then scenarios with edge cases live in
`.moai/specs/SPEC-DEEP-001/acceptance.md`. This section enumerates
the acceptance gate per requirement.

### REQ-DEEP1-001 — Ubiquitous: structured `/generate_report` endpoint

- File `services/storm/src/storm/app.py` exists and registers the
  `/generate_report`, `/health`, `/readyz` routes.
- File `services/storm/src/storm/models.py` exports
  `GenerateReportRequest`, `GenerateReportResponse`, `Section`,
  `Sentence`, `Citation` Pydantic models with `schema_version: int = 1`
  on the response.
- `test_post_generate_report_returns_200_with_structured_response`:
  POST a valid request with mocked LM; assert HTTP 200; assert
  response shape matches `GenerateReportResponse` schema; assert
  `schema_version == 1`.
- `test_response_schema_version_present`: assert every successful
  response carries `schema_version: 1`.
- `test_health_returns_ok`: `GET /health` → `{status: "ok"}`,
  HTTP 200.
- `test_readyz_reflects_dep_state`: `GET /readyz` returns
  `{ready: true, deps: {litellm: true, storm_lib: true}}` when both
  dependencies are reachable; `{ready: false, deps: {litellm: false,
  storm_lib: true}}` when LiteLLM is down (mock `httpx.get` to raise).

### REQ-DEEP1-002 — Ubiquitous: marker → doc_id resolution

- File `services/storm/src/storm/citation_translator.py` exists with
  `canonicalize_url(url: str) -> str` and
  `translate_citations(text: str, storm_refs: list[dict], docs:
  list[NormalizedDocPayload]) -> tuple[str, list[Citation], int]`
  (returns translated text, citations array, unresolved count).
- `test_marker_to_doc_id_resolution_via_url_canonicalization`:
  given STORM output `"GPT-4 [1]."` with refs `[{n: 1, url:
  "HTTPS://Example.com/Page?q=1/"}]` and docs containing
  `{id: "doc_a", url: "https://example.com/page"}`; assert
  translated text contains `[1]` (renumbered if needed) AND
  `Citation{marker: 1, doc_id: "doc_a"}` in output.
- `test_url_canonicalization_handles_protocol_query_trailing_slash`:
  parameterized over `(http vs https, ?query absent vs present,
  trailing slash absent vs present)` — assert canonical form is
  identical across all 8 combinations.
- `test_unresolved_marker_stripped_from_text`: STORM marker
  references a URL not in `docs[]`; assert `[N]` removed from
  output text; counter `usearch_storm_unresolved_citations_total`
  +1.
- `test_unresolved_citations_counter_increments`: aggregate over
  multiple unresolved markers in a single response.
- `test_citations_array_sorted_and_one_indexed`: response
  `citations[]` is sorted by `marker` ascending and starts at 1.

### REQ-DEEP1-003 — Event-Driven: faithfulness gate

- File `services/storm/src/storm/faithfulness.py` exists with
  `enforce_long_form_faithfulness(sections: list[Section], docs:
  list[NormalizedDocPayload], mode: str) -> tuple[list[Section],
  EnforcementOutcome, int, int]` (returns sections, outcome,
  uncited_count, sections_affected).
- `test_faithfulness_strip_removes_uncited_sentences`:
  `mode=strip`, section with sentences `["A. [1]", "B."]`; assert
  output section has `sentences == [{text: "A. [1]", ...}]` only.
- `test_faithfulness_reject_returns_422`: `mode=reject`, any
  un-cited sentence; assert HTTP 422 with body
  `{error: "un_cited_long_form", uncited_count: ..., sections_affected: ...}`.
- `test_faithfulness_off_bypasses_gate`: `mode=off`; assert
  un-cited sentences pass through verbatim; counter
  `usearch_storm_faithfulness_outcomes_total{outcome="off"}` +1.
- `test_empty_section_removed_from_response`: section becomes empty
  after strip; assert it does NOT appear in `response.sections[]`.
- `test_faithfulness_outcomes_counter_increments`: parameterized
  over the 4 outcome values; assert counter +1 per request.
- `test_sentence_segmentation_canonical_regex`:
  internal regex string in `services/storm/src/storm/faithfulness.py`
  equals `[.!?。！？]\s+|[.!?。！？]$` exactly. DEEP-001 owns this
  Python-side regex literal (single source of truth on the Python
  side — there is no `services/researcher/src/researcher/faithfulness.py`
  to reuse, since the SYN-002 Python-side gate was descoped at build
  time). The Go-side `internal/streamsynth/streamsynth.go:28` regex
  is a separate symbol with the same literal string. Documented in
  code comment with `@MX:ANCHOR: regex contract; @MX:REASON: any
  future change must be coordinated across the Go-side streamsynth
  file to avoid divergence`.

### REQ-DEEP1-004 — Unwanted: latency + budget caps

- `test_deadline_exceeded_returns_504`: mock STORM pipeline to take
  301 s; assert HTTP 504 with body matching schema; assert no
  `text` field in body (no leakage).
- `test_deadline_exceeded_increments_counter`: counter
  `usearch_deep_outcomes_total{outcome="deadline_exceeded"}` +1.
- `test_deadline_exceeded_no_partial_text_in_response`: response
  body schema does not contain `sections` or `text` fields.
- `test_deadline_exceeded_emits_warn_log`: exactly one WARN log
  record with attributes `{request_id, reason:
  "deadline_exceeded", elapsed_ms, partial_sections_completed}`.
- `test_budget_exceeded_returns_402`: mock cumulative cost to
  reach 2.51 USD mid-pipeline; assert HTTP 402 with body matching
  schema.
- `test_budget_exceeded_increments_counter`: counter
  `usearch_deep_outcomes_total{outcome="budget_exceeded"}` +1.
- `test_go_client_maps_504_to_err_deadline_exceeded`: integration
  test — `internal/deepreport.Client.GenerateReport` against a
  stub HTTP server returning 504; assert returned error satisfies
  `errors.Is(err, deepreport.ErrDeadlineExceeded)`.
- `test_go_client_maps_402_to_err_budget_exceeded`: same pattern
  for budget exceeded.
- `test_no_resource_leak_on_cancel`: goroutine-leak detector PASS
  on both deadline + budget cancellation paths.

### REQ-DEEP1-005 — State-Driven: SSE long-form streaming

- File `internal/streamsynth/longform.go` exists and exports
  `StreamLongFormReport(ctx, w *sse.Writer, report deepreport.Report)
  (StreamStats, error)`.
- `test_sse_emits_section_start_per_section`: 3-section report;
  assert exactly 3 `event: section_start` events on the wire in
  section order.
- `test_sse_emits_sentence_per_sentence_with_section_index`:
  every `event: sentence` payload includes `section_index` field
  matching its parent section.
- `test_sse_emits_section_done_per_section`: 3-section report;
  exactly 3 `event: section_done` events.
- `test_sse_emits_done_with_totals`: terminal `event: done`
  payload includes `total_sections == 3`, `total_sentences == N`.
- `test_sse_inherits_syn004_heartbeat`: heartbeat goroutine emits
  `: ping\n\n` at `SYN004_SSE_HEARTBEAT_MS` interval; identical
  behavior to SYN-004 single-paragraph streaming.
- `test_sse_inherits_syn004_disconnect_cancel`: client disconnect
  mid-section; upstream sidecar HTTP call gets ctx cancel within
  `SYN004_DISCONNECT_CANCEL_MS + 100 ms`.
- `test_sse_inherits_syn004_write_timeout`: slow client triggers
  `SYN004_SSE_WRITE_TIMEOUT_MS` write-deadline; behavior identical
  to SYN-004 REQ-SYN4-006.
- `test_sse_preserves_syn002_invariant_at_long_form_scale`: every
  emitted `event: sentence` `.text` field contains at least one
  `[N]` marker; property test with synthesized inputs.

### REQ-DEEP1-006 — Optional: Accept-header fallback to JSON

- `test_accept_missing_falls_back_to_json`: omit Accept header →
  HTTP 200 `application/json` body matching
  `GenerateReportResponse` schema byte-equivalent (modulo
  request_id) to a direct sidecar `/generate_report` call.
- `test_accept_application_json_returns_buffered_report`: explicit
  `Accept: application/json` → JSON path.
- `test_accept_fallback_no_sse_overhead`: SSE writer / heartbeat
  goroutine constructors record zero invocations.
- `test_accept_fallback_response_byte_identical_to_direct_sidecar_call_modulo_request_id`:
  byte comparison test.

### NFR-DEEP1-001 — Latency

- `test_long_form_latency_p50_under_180s` (mocked-LM 50 iterations):
  assert end-to-end p50 ≤ 180 s.
- `test_long_form_latency_p95_under_300s` (mocked-LM 50 iterations):
  assert p95 ≤ 300 s.
- `test_ttfb_section_start_within_5s_of_done` (mocked-LM):
  buffered-then-streamed mode — TTFB to first `event:
  section_start` ≥ end-to-end latency − 5 s (i.e., emission lag
  is ≤ 5 s).

### NFR-DEEP1-002 — Property: invariant + well-formedness

- `test_property_long_form_marker_resolution` (hypothesis): every
  `[N]` marker resolves to a `doc_id` in `Citations[]`.
- `test_property_section_sentences_markers_in_range` (hypothesis):
  every `Section.sentences[].markers[]` integer ∈ `[1,
  len(Citations)]`.
- `test_property_section_text_reconstruction`: union of
  `Section.text` fields preserves report content.
- `test_property_no_empty_sections`: every `Section.sentences[]`
  is non-empty.
- `test_property_title_non_empty`: `report.title != ""`.
- `test_property_citations_sorted_one_indexed`: `Citations[]`
  sorted by `marker` ascending; first `marker == 1`.

### NFR-DEEP1-003 — Invariant: outcome counter exactly-once

- `test_outcome_counter_at_most_once_per_request`: aggregate
  assertion across REQ-001/004 — union of all 6 outcome label
  values increments by exactly 1 per request lifecycle.
- `test_outcome_counter_race_success_vs_deadline`: success path
  finishes ≤1 ms before deadline timer fires; assert counter == 1.
- `test_outcome_counter_race_deadline_vs_budget`: budget exceeded
  fires concurrently with deadline; assert counter == 1.
- `test_outcome_counter_race_budget_vs_error_upstream`: LLM
  provider returns 5xx during budget cancel; assert counter == 1.
- All race tests run with `go test -race` for the Go-side
  contribution; Python side uses `asyncio.run` with explicit task
  cancellation.

---

## 5. Technical Approach (high-level, no implementation code)

Detailed plan, file impact, and test plan live in
`.moai/specs/SPEC-DEEP-001/plan.md`. High-level approach:

- **Sidecar mirrors `services/researcher/`**: same FastAPI app
  factory, same gateway pattern (LiteLLM-rooted `dspy.LM`), same
  obs.py JSON-log + counter discipline. The Python file structure is
  parallel to researcher; tests are parallel to researcher. No new
  shared library; small amount of intentional duplication is
  preferred over premature abstraction (TRUST 5 Readable + Enforce
  Simplicity).
- **STORM is invoked via its public Python API** (`STORMWikiRunner`
  + `STORMWikiLMConfigs` + `STORMWikiRunnerArguments` per upstream
  README; verify in run phase). LM configs all point at LiteLLM
  proxy; retrieval module is our injected shim consuming the
  request-payload `docs[]`.
- **Citation translation is sidecar-side** (no Go-side knowledge of
  STORM internals). The sidecar produces `[N]` markers already
  resolved against `doc_id`s; the Go-side `Result` shape is
  symmetric with `internal/synthesis.Result` + section structure.
- **Faithfulness gate runs sidecar-side** before the response is
  serialized. Long-form has no LLM retry — strip is the only
  practical fallback (full STORM re-run is 60–300 s).
- **Streaming reuses `internal/sse.Writer` + `internal/sse.RunHeartbeat`
  verbatim**. New code in `internal/streamsynth/longform.go` walks
  the structured response (sections → sentences) and emits SSE.
- **Latency + budget caps enforced sidecar-side** via httpx call
  budgeting and an asyncio task running the STORM pipeline with a
  ctx-cancel deadline. The Go-side client receives 504/402 as
  documented; the per-call cap parameter for DEEP-004 is already
  threaded through.

---

## 6. Risks (top-level summary)

Detailed risk register lives in `.moai/specs/SPEC-DEEP-001/research.md`
§5. Top three for SPEC-author attention:

1. **Citation marker → doc_id translation gap (R2)** — STORM emits
   URL-cited content; we want `doc_id`-cited. URL canonicalization
   + unresolved-URL fallback to faithfulness mode handler.
   REQ-DEEP1-002 encodes the contract. Without this, faithfulness
   gate strips the entire long-form output.
2. **Latency exceeds 5-min budget (R3)** — conservative default knobs
   + `STORM_MAX_LATENCY_MS=300000` ctx ceiling + per-call override
   knobs. SPEC-DEEP-004 layers per-user quota; DEEP-001 owns the
   per-call ceiling. REQ-DEEP1-004 + NFR-DEEP1-001 encode.
3. **Token cost runaway (R4)** — per-call cap `STORM_MAX_COST_USD=2.50`
   plus per-stage cost tracking via existing
   `usearch_synthesis_cost_usd_total` family. SPEC-DEEP-004 will
   tie to per-user/per-day cap. REQ-DEEP1-004 encodes.

---

## 7. References

Internal:

- `services/researcher/src/researcher/app.py:49-81` — FastAPI
  endpoint pattern being mirrored
- `services/researcher/src/researcher/synthesis.py:153-220` — sidecar
  orchestration pattern being mirrored
- `services/researcher/src/researcher/faithfulness.py` — SPEC-SYN-002
  gate; same contract applied to long-form (REQ-DEEP1-003)
- `services/researcher/src/researcher/gateway.py:36-72` — LiteLLM
  client pattern; STORM's LM-config layer wires the same way
- `services/researcher/src/researcher/obs.py` — JSON log + counter
  pattern being mirrored
- `services/researcher/src/researcher/models.py:55-63` — `Citation`
  Pydantic shape (REUSED for long-form)
- `services/storm/` — empty skeleton (Dockerfile, pyproject.toml,
  README.md, src/storm/__init__.py); v0 build site
- `services/storm/.env.example` — `LITELLM_BASE_URL=http://litellm:4000`
  (matches `services/researcher/src/researcher/gateway.py:26` default
  literal `base_url = os.environ.get("LITELLM_BASE_URL", "http://litellm:4000")`;
  override to `http://localhost:4000` when running outside the
  docker-compose network) and `LITELLM_API_KEY` for in-container
  API-key env (matches `services/researcher/src/researcher/gateway.py:27`
  literal `api_key = os.environ.get("LITELLM_API_KEY", "")` — the host
  `${LITELLM_MASTER_KEY}` is aliased into the container's
  `LITELLM_API_KEY` per `deploy/docker-compose.yml:173`
  `LITELLM_API_KEY: ${LITELLM_MASTER_KEY}`); SPEC-LLM-001 REQ-LLM-005
  (verbatim: "The Client SHALL authenticate to the LiteLLM proxy via
  the `LITELLM_MASTER_KEY` environment variable sent as an
  `Authorization: Bearer <key>` header on every request…[clause
  omitted: secret-redaction obligations]…") governs the
  Go-side `internal/llm` Client (cmd/usearch-api), not the Python
  sidecar — DEEP-001's Python container follows the researcher Python
  convention; v0 extends with `STORM_*` knobs
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
- `.moai/specs/SPEC-SYN-001/spec.md` — citation marker contract
  (NFR-SYN-002 marker → doc_id mapping invariant; REUSED at
  long-form scale by REQ-DEEP1-002)
- `.moai/specs/SPEC-SYN-002/spec.md:159-163` — faithfulness contract
  (REQ-SYN2-001..005); REUSED verbatim by REQ-DEEP1-003
- `.moai/specs/SPEC-SYN-002/spec.md` — sentence segmentation regex
  `[.!?。！？]\s+|[.!?。！？]$` (REUSED verbatim by REQ-DEEP1-003)
- `.moai/specs/SPEC-SYN-004/spec.md:226-234` — SSE wire format
  (REQ-SYN4-001a..c, REQ-SYN4-002..006, NFR-SYN4-001..003); REUSED +
  EXTENDED by REQ-DEEP1-005
- `.moai/specs/SPEC-LLM-001/spec.md:116-122` — LiteLLM proxy contract
  (REQ-LLM-001..007); HARD constraint for STORM LM configs
- `.moai/specs/SPEC-IR-001/spec.md` — `/deep` verb classification
  precondition; `RoutingDecision.Lang` flows into long-form `lang`
- `.moai/specs/SPEC-FAN-001/spec.md` — fanout produces the doc
  corpus consumed by the injected RM
- `.moai/project/roadmap.md:74` — SPEC-DEEP-001 row
- `.moai/project/roadmap.md:154` — M5 exit criterion (≥10 cited
  sources, ≤5 min)
- `.moai/specs/SPEC-DEEP-001/research.md` — companion research
  artifact (run-phase prerequisite reading)

External (verify URLs via WebFetch in Run phase per
anti-hallucination policy):

- `https://github.com/stanford-oval/storm` — upstream STORM
  repository
- `https://pypi.org/project/knowledge-storm/` — PyPI page
- `https://arxiv.org/abs/2402.14207` — Shao et al. 2024 STORM paper
- `https://github.com/stanfordnlp/dspy` — dspy-ai LM-wrapper
  library (transitive dep)
- W3C "Server-Sent Events" living standard,
  whatwg.org/multipage/server-sent-events.html (cross-ref via
  SPEC-SYN-004 §7)
- LiteLLM proxy docs, litellm.ai (cross-ref via SPEC-LLM-001 §7)

---

*End of SPEC-DEEP-001 v0.1 (draft).*
