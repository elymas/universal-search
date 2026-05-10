# SPEC-DEEP-001 Compact Summary

Version: 0.3.1

One-page distillation of `.moai/specs/SPEC-DEEP-001/spec.md` for
loading-into-context efficiency. Keep under 200 lines.

---

## Identity

- **ID**: SPEC-DEEP-001
- **Title**: STORM long-form report sidecar
- **Status**: approved
- **Priority**: P0
- **Milestone**: M5 — /deep multi-agent
- **Methodology**: TDD (RED-GREEN-REFACTOR), coverage ≥85%
- **Harness**: thorough (Sprint Contracts required)
- **Owner**: expert-backend
- **Depends on**: SPEC-SYN-001, SPEC-SYN-002, SPEC-SYN-004,
  SPEC-IR-001, SPEC-FAN-001, SPEC-LLM-001, SPEC-CORE-001
- **Blocks**: SPEC-DEEP-002, SPEC-DEEP-003, SPEC-DEEP-004
- **Issue**: 0 (no GH tracking)

---

## Purpose (1 paragraph)

M4 hardened single-paragraph synthesis (`/synthesize`); DEEP-001
introduces the long-form generation surface. A NEW Python sidecar
at `services/storm/` wraps `stanford-oval/storm` (PyPI
`knowledge-storm`) and produces a structured multi-section report
grounded in the same input doc corpus. The sidecar mirrors
`services/researcher/` in shape (FastAPI app, LiteLLM-rooted
`dspy.LM` configs, Pydantic models, JSON-log obs). Citation
faithfulness preserved verbatim from SPEC-SYN-002 (per-sentence
`[N]` marker → `doc_id`); SSE wire format preserved + extended
from SPEC-SYN-004 with `event: section_start` / `event: section_done`.
LiteLLM proxy (SPEC-LLM-001) is the only LLM access path. v0
ships **buffered-then-streamed** mode (full report → segment →
emit). Per-call caps `STORM_MAX_LATENCY_MS=300000` and
`STORM_MAX_COST_USD=2.50`; per-user/per-day quota deferred to
SPEC-DEEP-004.

---

## EARS Requirements (6)

| ID | Pattern | One-line |
|----|---------|----------|
| REQ-DEEP1-001 | Ubiquitous | `POST /generate_report` returns structured `{title, sections, citations, ...}` JSON; `/health`, `/readyz` aux endpoints; schema_version=1 |
| REQ-DEEP1-002 | Ubiquitous | Every `[N]` marker SHALL resolve to `Citation.doc_id` via URL canonicalization (lowercase host, strip query, normalize protocol, strip trailing slash); unresolved markers stripped + counter +1 |
| REQ-DEEP1-003 | Event-Driven | WHEN faithfulness gate runs (mode != "off"), THEN per-section sentence gate (SYN-002 regex) strips/rejects/bypasses un-cited sentences; empty sections removed |
| REQ-DEEP1-004 | Unwanted | IF latency exceeds `STORM_MAX_LATENCY_MS`, THEN HTTP 504 + ErrDeadlineExceeded; IF cost exceeds `STORM_MAX_COST_USD`, THEN HTTP 402 + ErrBudgetExceeded |
| REQ-DEEP1-005 | State-Driven | WHILE Accept advertises `text/event-stream`, SSE emits `section_start` → `sentence` ×N → `section_done` per section, then terminal `done`; inherits SYN-004 heartbeat + disconnect + write-timeout |
| REQ-DEEP1-006 | Optional | WHERE Accept does not advertise SSE, fall back to buffered JSON `Report` body (HTTP 200) |

## NFRs (3)

- **NFR-DEEP1-001 Latency**: end-to-end p50 ≤ 180 s, p95 ≤ 300 s
  (default knobs, 20–50 doc corpus); TTFB to first
  `event: section_start` ≥ end-to-end latency − 5 s
- **NFR-DEEP1-002 Property**: invariant suite — every marker
  resolves to doc_id; markers in [1, len(citations)]; lossless
  reconstruction; non-empty sections/heading/title; citations
  sorted 1-indexed (hypothesis property tests)
- **NFR-DEEP1-003 Invariant**: `usearch_deep_outcomes_total`
  counter SHALL increment exactly once per non-disconnect request
  lifecycle across the 6 outcome label values; **at-most-once
  (zero-or-one) for disconnect** — DEEP-001 deviates from
  SPEC-SYN-004 NFR-SYN4-003 in the disconnect path: SYN-004 owns
  the disconnect emission via
  `usearch_syn004_outcomes_total{outcome=client_disconnect}` (exactly-once
  on that counter family per SYN-004's own NFR-SYN4-003); the DEEP
  counter is zero-or-one for disconnect (HARD; sync.Once-style guard
  Go-side + single emission Python-side); race tests for all pairs

---

## Architecture Delta

```
cmd/usearch-api POST /deep
  ├→ Accept: text/event-stream  → streamsynth.StreamLongFormReport
  └→ Accept: application/json   → buffered JSON Report

internal/deepreport.Client.GenerateReport(ctx, req)
  └→ HTTP POST /generate_report
        services/storm/app.py:generate_report_endpoint
          └→ pipeline.run(req)
                ├→ gateway.build_lm_configs   [LiteLLM-rooted dspy.LM]
                ├→ inject_rm.InjectedRM(docs) [request-payload docs only]
                ├→ STORMWikiRunner(...).run() [upstream]
                ├→ citation_translator.translate (URL → doc_id)
                ├→ ★ faithfulness.enforce_long_form_faithfulness
                ├→ [drop empty sections]
                └→ obs.emit_outcome (exactly-once guard)
```

---

## Files Touched

**[NEW Python — services/storm/src/storm/]**
- `__main__.py`, `app.py`, `models.py`, `gateway.py`, `obs.py`
- `pipeline.py` (orchestration)
- `inject_rm.py` (custom RM consuming request docs[])
- `citation_translator.py` (URL → doc_id)
- `faithfulness.py` (long-form gate, SYN-002 regex literal)
- `services/storm/Dockerfile`
- `services/storm/tests/test_*.py` (8 test files)

**[NEW Go — internal/deepreport/]**
- `types.go`, `client.go`, `config.go`, `client_test.go`

**[NEW Go — internal/streamsynth/]**
- `longform.go`, `longform_test.go` (sibling of streamsynth.go)

**[NEW Go — internal/obs/metrics/]**
- `deepreport.go`, `deepreport_test.go`

**[MODIFY]**
- `services/storm/pyproject.toml` (deps: knowledge-storm, dspy-ai, fastapi, ...)
- `services/storm/.env.example` (STORM_* knobs + `LITELLM_BASE_URL` + `LITELLM_API_KEY` for in-container env per researcher gateway.py:27; `LITELLM_MASTER_KEY` documented as host-side var aliased by docker-compose)
- `services/storm/README.md` (operator quickstart)
- `internal/obs/metrics/metrics.go` (registerDeepReport)
- `internal/obs/obs.go` (re-export collectors)
- (no allowlist amendment required — `outcome` label NAME is pre-existing in `internal/obs/metrics/metrics_test.go:257` literal `"outcome": true,`; DEEP-001 only adds new VALUES)
- `cmd/usearch-api/handlers/...` (POST /deep + content-negotiation)
- `deploy/docker-compose.yml` (storm service entry: port `${STORM_PORT:-8001}:8001` mirroring researcher line 170; alias `LITELLM_API_KEY: ${LITELLM_MASTER_KEY}` mirroring line 173)

**[UNCHANGED]**
- `services/researcher/` (distinct service; SYN-001/002 preserved)
- `internal/synthesis/` (distinct client; long-form is parallel)
- `internal/sse/`, `internal/streamsynth/streamsynth.go`
  (single-paragraph stream)
- `pkg/types/normalized_doc.go`, `internal/router/`,
  `internal/fanout/`, `internal/llm/`

---

## Configuration

- `STORM_MAX_LATENCY_MS` = 300000 (5 min ctx ceiling)
- `STORM_MAX_COST_USD` = 2.50 (per-call budget cap)
- `STORM_MAX_PERSPECTIVES` = 2 (conservative; STORM default is 3)
- `STORM_MAX_CONV_TURNS` = 2 (conservative)
- `STORM_SEARCH_TOP_K` = 3
- `STORM_MAX_THREAD_NUM` = 3
- `STORM_DO_POLISH` = false (default off; enable empirically)
- `STORM_FAITHFULNESS_MODE` = strip (default) | reject | off
- `STORM_MODEL_OUTLINE` = claude-haiku-4-5
- `STORM_MODEL_ARTICLE` = claude-sonnet-4-6
- `STORM_SIDECAR_URL` (Go-side) = http://localhost:8001
- Inherits `LITELLM_BASE_URL` (default `http://litellm:4000`,
  matches `services/researcher/src/researcher/gateway.py:26` literal)
  and `LITELLM_API_KEY` for in-container env (matches
  `services/researcher/src/researcher/gateway.py:27` literal); host's
  `LITELLM_MASTER_KEY` aliased into the container's `LITELLM_API_KEY`
  by `deploy/docker-compose.yml:173` per researcher precedent.
  SPEC-LLM-001 REQ-LLM-005 governs the Go-side `internal/llm` Client
  (cmd/usearch-api), not the Python sidecar.

---

## Observability

**New Go-side Prometheus collectors** (no cardinality allowlist
amendment — the `outcome` label NAME is pre-existing per
`internal/obs/metrics/metrics_test.go:251-272` line 257; only new
VALUES added, pre-initialised per the SYN-004
`streamsynth.go:48-56` Add(0) pattern):
- `usearch_deep_outcomes_total{outcome=...}` (6 values: success,
  deadline_exceeded, budget_exceeded, error_invalid,
  error_upstream, error_unresolved_citations_threshold — last
  reserved per plan.md §10 D4)
- `usearch_deep_latency_seconds` (histogram, 8 buckets [5, 15, 30,
  60, 120, 180, 240, 300] s)

**Python-side counters** (scraped from sidecar `/metrics`):
- `usearch_storm_faithfulness_outcomes_total{outcome=...}` (4
  values: accepted, stripped, rejected, off — DEEP-001 emits
  off-mode counter unlike SYN-002 D2; rationale plan.md §10 D1)
- `usearch_storm_unresolved_citations_total` (no labels)

**JSON log extensions**:
- `outcome`, `sections_count`, `sentences_count`,
  `citations_count` on success
- `reason` ("deadline_exceeded" / "budget_exceeded" / etc.) on
  error paths

**OTel spans**: `deep.generate_report` (Go), `storm.run` (Python)

**Status → outcome map**: 200→`success`, 422→`error_invalid`,
504→`deadline_exceeded`, 402→`budget_exceeded`,
502/503→`error_upstream`, disconnect→at-most-once on DEEP counter
(SYN-004 owns disconnect emission via
`usearch_syn004_outcomes_total{outcome=client_disconnect}`). See
spec.md §3.2 (renumbered).

---

## SSE Wire Vocabulary (extends SPEC-SYN-004)

| Event | DEEP-001 Status | Payload |
|-------|-----------------|---------|
| `section_start` | NEW | `{request_id, section_index, heading, level, schema_version}` |
| `sentence` | EXTENDED (+section_index) | `{request_id, section_index, sentence_index, text, citations[], schema_version}` |
| `section_done` | NEW | `{request_id, section_index, sentences_emitted, schema_version}` |
| `done` | EXTENDED (+totals) | `{request_id, total_sections, total_sentences, latency_ms, model, provider, cost_usd, schema_version}` |
| `error` | UNCHANGED (SYN-004) | `{request_id, error_code, error_message, partial_sentences_emitted, schema_version}` |
| `: ping` heartbeat | UNCHANGED (SYN-004) | `: ping\n\n` |

Backward compat: pre-DEEP-001 SYN-004 clients calling
`/synthesize` (single-paragraph) see no change. New `/deep`
clients ignore unknown event types per W3C SSE.

---

## MX Tag Plan

- `services/storm/src/storm/pipeline.py:run()` →
  `@MX:ANCHOR` + `@MX:WARN` (LLM-trust + budget-guard)
- `services/storm/src/storm/citation_translator.py:translate()`
  → `@MX:ANCHOR` (URL→doc_id chokepoint)
- `services/storm/src/storm/faithfulness.py:enforce_long_form_faithfulness()`
  → `@MX:ANCHOR` + `@MX:WARN` (SYN-002 contract preservation)
- `services/storm/src/storm/inject_rm.py:InjectedRM.forward()` →
  `@MX:NOTE`
- `services/storm/src/storm/gateway.py:build_lm_configs()` →
  `@MX:NOTE`
- `internal/deepreport/client.go:Client.GenerateReport()` →
  `@MX:ANCHOR`
- `internal/streamsynth/longform.go:StreamLongFormReport()` →
  `@MX:ANCHOR`
- `internal/obs/metrics/deepreport.go` collectors → `@MX:NOTE`

---

## Top 3 Risks (one-line each)

1. **Citation marker → doc_id translation gap** — STORM emits URL-cited;
   we resolve via canonicalization. Without robust canonicalization,
   strip-mode wipes the entire long-form output.
2. **Latency exceeds 5-min budget** — conservative default knobs +
   `STORM_MAX_LATENCY_MS=300000` ctx ceiling; NFR-DEEP1-001 targets
   p50 ≤ 180 s, p95 ≤ 300 s.
3. **Token cost runaway** — `STORM_MAX_COST_USD=2.50` per-call cap;
   SPEC-DEEP-004 will tie to per-user/per-day cap.

---

## Exclusions (key)

NOT multi-agent pipeline (→ SPEC-DEEP-002). NOT tree exploration
(→ SPEC-DEEP-003). NOT per-user/per-day quota (→ SPEC-DEEP-004).
NOT semantic faithfulness scoring (→ SPEC-EVAL-001). NOT
intra-sentence token streaming (buffered-then-streamed v0). NOT
non-LiteLLM LLM access. NOT modifying `/synthesize` behavior. NOT
`/deep` CLI surface (→ SPEC-CLI-002). NOT MCP tool surface (→
SPEC-MCP-001). NOT WebSocket/gRPC/NDJSON. NOT Last-Event-ID
resume.

---

## Acceptance Quick-Check

10 acceptance scenarios + 15 edge cases in `acceptance.md`.
Definition of Done has 14 binary gates including SPEC-SYN-001/002/004
regression suites green and M5 exit criterion (≥10 cited sources)
verified on at least one golden topic. Edge Case 3 (all sentences
un-cited even in mode=strip) resolves to HTTP 422 — parallels SYN-002
plan.md §10 D1 decision.

---

*Companion: spec.md, plan.md, acceptance.md, research.md*
