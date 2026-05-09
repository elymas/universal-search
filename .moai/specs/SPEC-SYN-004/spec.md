---
id: SPEC-SYN-004
version: 0.1.0
status: approved
created: 2026-05-09
updated: 2026-05-09
author: limbowl
priority: P0
issue_number: 0
title: Streaming response (SSE)
milestone: M4 — Basic Synthesis Hardening
owner: expert-backend
methodology: tdd
coverage_target: 85
depends_on: [SPEC-IR-001, SPEC-SYN-001, SPEC-SYN-002]
blocks: []
---

# SPEC-SYN-004: Streaming response (SSE over usearch-api)

## HISTORY

- 2026-05-09 — status draft → approved (plan-auditor PASS iter 2, ND1 stale ID references resolved):
  Mechanically replaced 6 stale `REQ-SYN4-001` references (no a/b/c
  suffix) flagged by ND1 MINOR in
  `.moai/reports/plan-audit/SPEC-SYN-004-review-2.md`. Replacements
  per audit guidance: acceptance.md §3.1 → 001a+001b+001c (full
  Ubiquitous trio); §3.2 (Korean) → 001c (citation invariant under
  Korean text); §3.7 (empty result) → 001a (header defensive
  boundary); plan.md L120 RED-phase test list expanded to enumerate
  001a/001b/001c; plan.md §3 Milestone P0-B (wire format) →
  001a + 001b; plan.md §3 Milestone P0-C + exit gate (invariant) →
  001c; plan.md §8 R4 (SYN-002 invariant violation) → 001c.
  Bonus: research.md L276 (same-flavor stale reference, not in audit
  list) updated to 001c for cross-doc coherence. spec.md and
  spec-compact.md REQ definitions already used the split IDs and
  required no change. Verification: `grep -n "REQ-SYN4-001\b"` (word
  boundary) returns 0 hits across spec/plan/acceptance/research/
  compact.

- 2026-05-09 — iter 1 audit fixes (REQ-SYN4-006 added; REQ-001 split into 001a/001b/001c; D3-D5 MINOR resolved):
  Addresses plan-auditor review at `.moai/reports/plan-audit/SPEC-SYN-004-review-1.md`.
  D1 MAJOR: Added REQ-SYN4-006 (Unwanted) governing slow-client write
  timeout via `SYN004_SSE_WRITE_TIMEOUT_MS`; promoted acceptance §4.5.3
  edge case to first-class Scenario H.
  D2/D3 MINOR: Split REQ-SYN4-001 into REQ-SYN4-001a (Ubiquitous —
  content-type), REQ-SYN4-001b (Ubiquitous — W3C wire format), and
  REQ-SYN4-001c (Ubiquitous — un-cited content invariant); the
  former WHEN-Accept-header trigger clause is now implicit (REQ-002
  already covers it via "AND the request advertised text/event-stream").
  D4 MINOR: REQ-SYN4-004 now enumerates all three goroutines (main
  writer, heartbeat, disconnect watcher) and asserts watcher exits
  within 100 ms of stream close.
  D5 MINOR: Added NFR-SYN4-003 promoting "outcome counter
  increments at most once per request" from prose to a HARD
  invariant; cross-linked in acceptance §4.5.2.
  Acceptance.md §4.2 gained an abbreviation edge case explicitly
  inheriting segmentation behavior from SPEC-SYN-002.
  REQ count: 5 → 8 (7 × P0 + 1 × P1). NFR count: 2 → 3. All five
  EARS patterns retained (Ubiquitous + Event-Driven + State-Driven +
  Unwanted + Optional). Status remains `draft` pending re-audit.

- 2026-05-09 (initial draft v0.1, limbowl via manager-spec):
  First EARS-formatted SPEC for incremental synthesis-result emission
  via Server-Sent Events on the `cmd/usearch-api` HTTP server. Adds
  a NEW SSE writer + heartbeat goroutine to `cmd/usearch-api/`
  (currently a stub at `cmd/usearch-api/main.go:1-40`; full server
  scaffolding owned by SPEC-IR-001). Streaming surface emits one SSE
  event per **complete sentence** of the synthesis output, where the
  sentence boundary is defined by SPEC-SYN-002 REQ-SYN2-001's
  canonical regex `[.!?。！？]\s+|[.!?。！？]$`. Each sentence event
  payload includes the sentence text plus its resolved citation
  array (`marker → doc_id → url`). Preserves SPEC-SYN-002's
  citation faithfulness invariant verbatim — un-cited content is
  NEVER emitted to the stream. Heartbeat `: ping\n\n` SSE comments
  defeat proxy buffering. Client disconnect cancels the upstream
  synthesis call within `SYN004_DISCONNECT_CANCEL_MS` (default
  1000 ms). Accept-header content negotiation enables backward
  compatibility — clients without `text/event-stream` receive the
  existing buffered JSON response unchanged. v0 ships
  **buffered-then-streamed** mode (server reads full
  `synthesis.Client.Synthesize()` result, then segments + emits)
  because the Python sidecar does not yet support upstream LLM
  token streaming (research §6 documents the gap). Token-level
  streaming is a documented exclusion gated on a follow-up SPEC
  (sidecar streaming upgrade). 5 EARS REQs (4 × P0 + 1 × P1)
  covering all five EARS patterns. 2 NFRs. Companion research
  artifact at `.moai/specs/SPEC-SYN-004/research.md` — pipeline
  trace, SSE wire-format survey, citation streaming strategy,
  proxy-buffering risk analysis, sidecar gap documentation. No
  GitHub issue tracking on this SPEC (`issue_number: 0`). Ready for
  plan-auditor review.

  Note: Original v0.1 declared "5 EARS REQs (4 × P0 + 1 × P1)" and "2
  NFRs"; iter 1 audit fixes (entry above) supersede those counts —
  current totals are 8 EARS REQs (7 × P0 + 1 × P1) and 3 NFRs.

---

## 1. Purpose

`.moai/project/roadmap.md` line 66 declares M4 SPEC-SYN-004:

> Streaming response | SSE over usearch-api, incremental citation
> emission | expert-backend

Today the `cmd/usearch` CLI consumes
`internal/synthesis.Client.Synthesize()` synchronously and prints the
final paragraph after the full LLM response is decoded. The future
`cmd/usearch-api` HTTP server (scaffolding owned by SPEC-IR-001)
will expose this synthesis behavior over HTTP. Without streaming, an
HTTP client must wait for the entire upstream LLM call (typically
3-8 s end-to-end per SPEC-SYN-001 NFR-SYN-001) before any output
arrives — perceived latency is the full upstream RTT.

SPEC-SYN-004 introduces an **incremental emission surface** so the
client receives the synthesized paragraph one sentence at a time as
each sentence's citations are validated. This reduces *perceived*
TTFB to the duration of the first complete cited sentence (~300-800
ms typical), preserves the SPEC-SYN-002 citation faithfulness
invariant unchanged (un-cited content NEVER reaches the stream), and
produces a deterministic SSE event stream that browsers, CLIs, and
intermediate proxies handle natively.

This SPEC is **the HTTP streaming surface only**. Whether the
upstream LLM call streams tokens to the Python sidecar is a
**separate prerequisite** (research §6, R5) — SYN-004 v0 ships
**buffered-then-streamed** mode that works regardless of sidecar
streaming support.

Completion delivers an SSE-emitting `cmd/usearch-api` synthesis
endpoint with sentence-level event granularity, configurable
heartbeat keepalive, client-disconnect upstream cancellation, and
Accept-header content-negotiation fallback to the existing buffered
JSON response.

---

## 2. Scope

### 2.1 In-Scope

| Ref | Item |
|-----|------|
| a | [MODIFY] `cmd/usearch-api/` — add HTTP request handler for the synthesis endpoint that performs Accept-header content negotiation: `text/event-stream` ⇒ SSE streaming path; otherwise ⇒ existing buffered JSON path. The exact handler file location depends on SPEC-IR-001's server layout (likely `cmd/usearch-api/handlers/synthesis.go` or equivalent); SPEC-SYN-004 declares *behavior*, not the file structure that SPEC-IR-001 establishes. |
| b | [NEW] `internal/sse/` package — pure-Go SSE writer abstraction. Public surface: `NewWriter(w http.ResponseWriter) *Writer`, `(w *Writer) WriteEvent(eventType string, data []byte) error`, `(w *Writer) WriteComment(text string) error`, `(w *Writer) Flush() error`, `(w *Writer) Close() error`. Wraps `net/http`'s `http.Flusher` and `http.ResponseController` for write-deadline support. |
| c | [NEW] `internal/sse/heartbeat.go` — heartbeat goroutine helper. Signature: `RunHeartbeat(ctx context.Context, w *Writer, interval time.Duration) error`. Emits `: ping\n\n` SSE comments every `interval` until ctx is done. Returns ctx error or write error. |
| d | [NEW] `internal/streamsynth/` package — orchestration layer between `synthesis.Client.Synthesize()` and the SSE writer. Public surface: `StreamSynthesize(ctx context.Context, w *sse.Writer, req StreamRequest) (StreamStats, error)`. v0 implementation: invokes `synthesis.Client.Synthesize()`, segments the resulting text into sentences via the SYN-002 canonical regex, validates each sentence's citations against `Result.Citations`, emits one `event: sentence` per validated sentence, emits `event: done` on completion or `event: error` on failure. |
| e | [NEW] `cmd/usearch-api` configuration: env vars `SYN004_SSE_HEARTBEAT_MS` (default 15000, range [1000, 60000]), `SYN004_SSE_HEARTBEAT_ENABLED` (default true), `SYN004_SSE_WRITE_TIMEOUT_MS` (default 5000, range [500, 30000]), `SYN004_DISCONNECT_CANCEL_MS` (default 1000, range [100, 10000]), `SYN004_BUFFERED_PACE_MS` (default 0, range [0, 1000]; synthetic per-sentence pacing in buffered mode for testing perceived latency). |
| f | [NEW] SSE event schema (versioned). Each event payload is a JSON object: `event: sentence` carries `{request_id, sentence_index, text, citations: [{marker, doc_id, url, title}], schema_version: 1}`; `event: done` carries `{request_id, total_sentences, latency_ms, model, provider, cost_usd, schema_version: 1}`; `event: error` carries `{request_id, error_code, error_message, partial_sentences_emitted, schema_version: 1}`. The schema is versioned to allow future evolution without breaking client parsers. |
| g | [NEW] Two Prometheus collectors in `internal/obs/metrics/streamsynth.go`: `StreamSynthOutcomes *prometheus.CounterVec{outcome}` (label values: `streamed_complete`, `client_disconnect`, `write_timeout`, `error_upstream`, `accept_fallback_to_json`) and `StreamSynthSentencesEmitted prometheus.Histogram` (per-call distribution of sentence count actually emitted before stream end; buckets `[0, 1, 2, 3, 5, 8, 13]`). Cardinality allowlist amendment per SPEC-OBS-001 discipline (5 pre-declared `outcome` values). |
| h | [MODIFY] `internal/obs/metrics/metrics.go` — register the two new collectors via `registerStreamSynth(pr)`. |
| i | [MODIFY] `internal/obs/obs.go` — re-export `obs.StreamSynthOutcomes`, `obs.StreamSynthSentencesEmitted`. |
| j | [MODIFY] `.env.example` — add the five `SYN004_*` env vars with explanatory comments and recommended nginx config snippet (`proxy_buffering off;`). |
| k | [EXISTING — UNCHANGED] `internal/synthesis/client.go` — `Client.Synthesize()` signature, retry semantics, observability emission all unchanged. SPEC-SYN-004 is a *consumer* of the existing single-shot API. |
| l | [EXISTING — UNCHANGED] `internal/synthesis/types.go` — `Result`, `Citation` Go shapes unchanged. |
| m | [EXISTING — UNCHANGED] `services/researcher/` Python sidecar — synthesis endpoint, gateway, citation marker processing all unchanged. SPEC-SYN-004 does NOT modify the Python side. The token-level streaming upgrade for the sidecar is gated on a follow-up SPEC (research §6 R5). |
| n | [EXISTING — UNCHANGED] `pkg/types/normalized_doc.go`, `internal/synthcluster/` (if SPEC-SYN-003 lands first), `internal/fanout/` — none of these are touched by SPEC-SYN-004. |
| o | [NEW] `internal/sse/writer_test.go`, `internal/sse/heartbeat_test.go`, `internal/streamsynth/streamsynth_test.go` — unit tests including SSE wire-format compliance, heartbeat cadence, sentence segmentation correctness, citation-invariant preservation. |
| p | [NEW] `cmd/usearch-api/handlers/synthesis_stream_test.go` (exact path per SPEC-IR-001 layout) — integration tests for the three modes: SSE happy path, client-disconnect cancellation, Accept-header fallback. |

### 2.2 Out-of-Scope (Exclusions — What NOT to Build)

[HARD] This SPEC explicitly excludes the following items. Each has a
known destination SPEC or follow-up; this list prevents scope creep.

- **NOT modifying synthesis logic itself** — `internal/synthesis/`
  Go side AND `services/researcher/` Python side are consumed
  read-only. SYN-001's REQ-SYN-001 through REQ-SYN-007 contract is
  preserved verbatim. SYN-002's REQ-SYN2-001 through REQ-SYN2-005
  contract is preserved verbatim.
- **NOT WebSocket transport** — SPEC-SYN-004 is SSE-only
  (`text/event-stream`). Bidirectional streaming, custom binary
  protocols, and gRPC streaming are explicitly out of scope. SSE is
  one-way (server → client) and SYN-004 needs only one-way.
- **NOT bidirectional streaming or client-to-server message flow** —
  the synthesis pipeline is request-response with progressive
  emission; no client commands or mid-stream interactions are
  supported.
- **NOT changing citation format** — SPEC-SYN-002's `Citation`
  Pydantic shape (`marker, doc_id, url, title`) and the `[N]` marker
  convention are preserved. SSE event payloads embed citations in
  the existing shape.
- **NOT token-level streaming from the Python sidecar** — the
  sidecar today calls LiteLLM single-shot via `httpx.AsyncClient.post`
  (research §2.2). Upgrading the sidecar to support `stream: true`
  upstream and re-emit tokens to the Go server is a **prerequisite
  gap** (research §6, R5) gated on a follow-up SPEC (likely
  SPEC-SYN-006 or SPEC-SYN-004-v2). SPEC-SYN-004 v0 ships
  **buffered-then-streamed** mode that is byte-equivalent in stream
  shape regardless of upstream chunk granularity.
- **NOT cross-call event-stream resume** — there is no `Last-Event-ID`
  header support, no cursor-based resume, no replay buffer. Each
  client connection produces a fresh stream from a fresh
  synthesis call. Resume is a SPEC-IDX-005 (M6 team-shared answer
  reuse) candidate, NOT in M4.
- **NOT character-level or token-level event granularity** — sentence
  is the atomic event unit. Sub-sentence streaming is documented as
  a future enhancement (research §4.3 "withhold-until-cited"
  variant) but is out of scope for v0. Rationale: SPEC-SYN-002
  citation invariant requires per-sentence validation; sub-sentence
  events would require a fundamentally different validation
  strategy.
- **NOT custom Accept-header types beyond `text/event-stream`** —
  e.g. `application/x-ndjson` or `application/grpc-web+proto` are
  not supported. Either `text/event-stream` (SSE) or fall back to
  JSON.
- **NOT modifying `cmd/usearch` CLI** — the existing CLI continues
  to consume `synthesis.Client.Synthesize()` synchronously. CLI
  streaming UX is a SPEC-CLI-002 (M7) candidate.
- **NOT new authentication / authorization mechanisms** — SYN-004
  inherits whatever auth scaffolding SPEC-IR-001 establishes for
  `cmd/usearch-api`. No SSE-specific auth.
- **NOT SSE multi-subscriber broadcast** — each request gets its
  own dedicated stream; no pub-sub. The `r3labs/sse/v2` library
  pattern is explicitly NOT adopted (research §3.4).
- **NOT GitHub Issue tracking on this SPEC** (`issue_number: 0`).

---

## 3. EARS Requirements

### Functional Requirements

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| REQ-SYN4-001a | Ubiquitous | Every SSE response from the `cmd/usearch-api` synthesis endpoint SHALL include the response headers `Content-Type: text/event-stream`, `Cache-Control: no-cache`, AND `Connection: keep-alive`. The trigger condition (request `Accept` header advertising `text/event-stream` via case-insensitive substring match) is governed by REQ-SYN4-002 (which selects the SSE dispatch path) and REQ-SYN4-005 (which defines the JSON fallback path); REQ-SYN4-001a applies unconditionally to every response that takes the SSE path. | P0 | `test_sse_content_type_set` (assert all three headers present on every SSE response), `test_sse_headers_absent_on_json_fallback` (negative — JSON path emits `Content-Type: application/json`, not SSE headers). |
| REQ-SYN4-001b | Ubiquitous | Every SSE response SHALL conform to the W3C Server-Sent Events wire format: each event terminated by a blank line `\n\n`; `event:` and `data:` fields formatted per the W3C spec; multi-line `data:` payloads SHALL repeat the `data:` prefix per line; SSE comments SHALL use the `: <text>\n\n` form. The wire format invariant applies to every emitted event regardless of event type (`sentence`, `done`, `error`) and to every heartbeat comment. | P0 | `test_sse_wire_format_blank_line_terminator` (every emitted event ends with `\n\n`), `test_sse_data_multiline_repeats_prefix` (JSON payload with embedded `\n` survives round-trip via reference EventSource parser), `test_sse_comment_wire_format` (heartbeat `: ping\n\n` matches W3C SSE comment grammar). |
| REQ-SYN4-001c | Ubiquitous | **[HARD constraint preserving SPEC-SYN-002 invariant]**: NO `event: sentence` data SHALL be emitted to the stream until that sentence's `[N]` citation markers have been validated against the input docs via the existing `synthesis.Result.Citations` array — un-cited content NEVER reaches the wire. This invariant SHALL preserve SPEC-SYN-002 REQ-SYN2-001 (every sentence carries at least one valid `[N]` marker resolving to a `doc_id`) AND SPEC-SYN-001 NFR-SYN-002 (every `[N]` marker maps to a real input doc) verbatim. The invariant is an unconditional property of every SSE response and is enforced regardless of upstream mode (buffered v0 or future token-streaming). | P0 | `test_no_uncited_sentence_emitted` (mock LLM emits a sentence missing `[N]` → assert no `event: sentence` for that sentence reaches the stream — strip mode), `test_syn002_invariant_preserved_under_streaming` (every emitted sentence has at least one valid citation in its event payload), property test NFR-SYN4-002 (b)(c). |
| REQ-SYN4-002 | Event-Driven | WHEN `internal/synthesis.Client.Synthesize()` returns a successful `Result` AND the request advertised `text/event-stream`, THEN `internal/streamsynth.StreamSynthesize` SHALL segment `Result.Text` into sentences using the canonical regex from SPEC-SYN-002 REQ-SYN2-001 (`[.!?。！？]\s+\|[.!?。！？]$`), SHALL resolve each sentence's `[N]` markers against `Result.Citations`, SHALL emit one SSE `event: sentence` per validated sentence with payload `{request_id, sentence_index, text, citations: [{marker, doc_id, url, title}], schema_version: 1}`, AND SHALL emit a final `event: done` payload `{request_id, total_sentences, latency_ms, model, provider, cost_usd, schema_version: 1}` once all sentences are emitted. The function SHALL increment `usearch_syn004_outcomes_total{outcome="streamed_complete"}` once per successful stream completion AND SHALL observe `usearch_syn004_sentences_emitted` histogram with the actual sentence count. | P0 | `test_sentence_segmentation_matches_syn002_regex`, `test_one_sse_event_per_sentence`, `test_event_payload_includes_citations`, `test_done_event_emitted_on_success`, `test_streamed_complete_counter_increments_once_per_call`, `test_sentences_emitted_histogram_observation`. |
| REQ-SYN4-003 | State-Driven | WHILE an SSE stream is open AND `SYN004_SSE_HEARTBEAT_ENABLED == true`, the server SHALL emit one `: ping\n\n` SSE comment line every `SYN004_SSE_HEARTBEAT_MS` milliseconds (default 15000) on a dedicated heartbeat goroutine until the stream terminates. The heartbeat SHALL be implemented via `internal/sse.RunHeartbeat(ctx, writer, interval)` with `ctx` derived from the request context. Heartbeats SHALL be idempotent and lossless under proxy buffering — comments are no-ops to client EventSource implementations but force flush of intermediate proxies (nginx, ALB, Cloudflare). The heartbeat SHALL NOT emit when `SYN004_SSE_HEARTBEAT_ENABLED == false` (test mode and special deployments). | P0 | `test_heartbeat_emits_at_configured_interval` (mock clock; assert `: ping\n\n` written every `interval ± jitter`), `test_heartbeat_disabled_emits_nothing`, `test_heartbeat_terminates_on_ctx_done`, `test_heartbeat_does_not_interleave_with_sentence_events` (synchronization assertion — heartbeat goroutine and main writer share writer mutex, no torn writes). |
| REQ-SYN4-004 | Unwanted | IF the client TCP connection closes mid-stream (browser tab close, network loss) — detected via `r.Context().Done()` by a dedicated disconnect-watcher goroutine — THEN the server SHALL cancel the parent synthesis call's context within `SYN004_DISCONNECT_CANCEL_MS` milliseconds (default 1000), SHALL propagate the cancellation to `synthesis.Client.Synthesize()` ctx, SHALL increment `usearch_syn004_outcomes_total{outcome="client_disconnect"}` exactly once (NFR-SYN4-003 invariant), SHALL emit one WARN-level structured log record with `{request_id, reason:"client_disconnect", sentences_emitted_before_disconnect:<N>}`, AND SHALL release all three goroutines cleanly: (i) main writer goroutine returns within the cancellation deadline, (ii) heartbeat goroutine exits within 100 ms of ctx cancel, AND (iii) the disconnect-watcher goroutine itself SHALL exit within 100 ms after stream close to prevent leak. The server SHALL NOT continue writing to the closed connection (subsequent writes return an error which is logged at DEBUG and discarded). The server SHALL NEVER leak the upstream LLM call beyond the cancellation deadline. | P0 | `test_client_disconnect_cancels_upstream_within_deadline` (close client connection mid-stream; assert upstream synthesis ctx cancellation observed within `SYN004_DISCONNECT_CANCEL_MS + 100ms` jitter), `test_client_disconnect_increments_counter`, `test_client_disconnect_emits_warn_log`, `test_client_disconnect_releases_heartbeat_goroutine`, `test_client_disconnect_releases_watcher_goroutine` (watcher exits within 100 ms; goroutine leak detector PASS for all three goroutines). |
| REQ-SYN4-005 | Optional | WHERE the request `Accept` header does NOT advertise `text/event-stream` (the substring is absent or the header is `application/json` / missing entirely), the server SHALL fall back to the existing buffered JSON response shape — invoke `synthesis.Client.Synthesize()`, marshal `Result` to JSON, return HTTP 200 with `Content-Type: application/json` body matching the SPEC-SYN-001 contract verbatim. The fallback path SHALL increment `usearch_syn004_outcomes_total{outcome="accept_fallback_to_json"}` once per such request AND SHALL NOT instantiate the SSE writer or heartbeat goroutine (zero overhead vs. a hypothetical pre-SYN-004 server). This preserves backward compatibility with existing CLI clients, integration tests, and any client unaware of SSE. | P1 | `test_accept_missing_falls_back_to_json` (Accept header omitted → JSON response, byte-equivalent to SPEC-SYN-001 contract), `test_accept_application_json_falls_back_to_json`, `test_accept_fallback_no_sse_overhead` (mock heartbeat / writer constructors record zero invocations), `test_accept_fallback_counter_increments`, `test_accept_text_html_falls_back_to_json` (any non-`text/event-stream` value triggers fallback). |
| REQ-SYN4-006 | Unwanted | IF a single SSE write to the client connection blocks longer than `SYN004_SSE_WRITE_TIMEOUT_MS` milliseconds (default 5000, range [500, 30000]) — detected via `http.ResponseController.SetWriteDeadline` returning an `os.ErrDeadlineExceeded` from the underlying `Write` call — THEN the server SHALL cancel the parent synthesis call's context (propagating cancellation to `synthesis.Client.Synthesize()`), SHALL increment `usearch_syn004_outcomes_total{outcome="write_timeout"}` exactly once per request (NFR-SYN4-003 invariant), SHALL attempt one best-effort `event: error` payload with `{error_code:"write_timeout", error_message, partial_sentences_emitted, schema_version:1}` before connection teardown (silently dropped if the second write also fails), SHALL emit one WARN-level structured log record with `{request_id, reason:"write_timeout", sentences_emitted_before_timeout:<N>}`, AND SHALL release all three goroutines cleanly: main writer (returns on the timeout error), heartbeat (exits within 100 ms of ctx cancel), AND disconnect watcher (exits within 100 ms after stream close). The server SHALL NEVER leak the upstream LLM call beyond the cancellation deadline. This requirement governs the slow-client backpressure mitigation declared as risk R1 in §6 and research §5. | P0 | `test_slow_client_write_timeout_fires_within_deadline` (mock client accepts headers but stalls on body; assert per-write deadline fires within `SYN004_SSE_WRITE_TIMEOUT_MS + 50 ms` jitter), `test_slow_client_write_timeout_cancels_upstream`, `test_write_timeout_counter_increments_once`, `test_write_timeout_emits_error_event_best_effort`, `test_write_timeout_releases_all_three_goroutines` (goroutine-leak detector PASS), `test_write_timeout_emits_warn_log`. |

### Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| NFR-SYN4-001 | Performance (TTFB to first sentence) | In **buffered-then-streamed** mode (v0), the time from request acceptance to first `event: sentence` byte on the wire SHALL be ≤ `synthesis.Client.Synthesize()` end-to-end latency + 50 ms (sentence segmentation + first SSE write overhead). For SPEC-SYN-001 NFR-SYN-001's p50 ≤ 8 s upstream synthesis, this means TTFB to first sentence p50 ≤ 8.05 s in buffered mode. The total stream duration SHALL preserve SPEC-SYN-001 NFR-SYN-001 (p50 ≤ 8 s end-to-end) — the streaming surface SHALL NOT add more than 100 ms of total wall-clock overhead on the happy path. Heartbeat goroutine overhead SHALL be < 1% CPU when emitting at the default 15 s interval. Detailed test method (iteration counts, percentile assertions) is specified in `acceptance.md` §4.4. |
| NFR-SYN4-002 | Property: SSE wire-format conformance + citation invariant preservation | For all valid `Result` inputs (`Result.Text` non-empty, `Result.Citations` array consistent), the streamed event sequence SHALL satisfy: (a) every event is `event: <type>\ndata: <json>\n\n` (W3C SSE conformant); (b) every `event: sentence` payload's `text` field passes the SYN-002 sentence-cited regex (every sentence has at least one `[N]` marker); (c) every `[N]` marker in any `event: sentence` `text` resolves to a `doc_id` in that event's `citations` array; (d) the union of `text` fields across all `event: sentence` events, joined by ` ` (single space), reconstructs `Result.Text` modulo whitespace normalization (lossless reconstruction); (e) exactly one `event: done` OR `event: error` terminates the stream; (f) no `event: sentence` follows an `event: done` or `event: error`. Property test via `testing/quick` over a generator producing realistic `Result` shapes (mixed Korean + English, varying citation densities, varying sentence counts). |
| NFR-SYN4-003 | Invariant: outcome counter exactly-once per request | **[HARD invariant]** For every request lifecycle (defined as: handler entry until both the main writer and disconnect-watcher goroutines have returned), the `usearch_syn004_outcomes_total` counter SHALL be incremented exactly once across the entire set of `outcome` label values (`streamed_complete`, `client_disconnect`, `write_timeout`, `error_upstream`, `accept_fallback_to_json`). Terminal state transitions where multiple outcomes might race (e.g. client disconnect arriving milliseconds after `event: done` was already written; or write timeout firing during a disconnect cancel) SHALL NOT produce double-increments — the first terminal outcome wins via a sync.Once-style guard. This invariant binds REQ-SYN4-002 (`streamed_complete` once-per-call), REQ-SYN4-004 (`client_disconnect` once-per-call), REQ-SYN4-005 (`accept_fallback_to_json` once-per-call), AND REQ-SYN4-006 (`write_timeout` once-per-call) into a single mutually-exclusive guarantee. Tested via `test_outcome_counter_at_most_once_per_request` (race-window tests for each pair: streamed_complete vs client_disconnect, streamed_complete vs write_timeout, client_disconnect vs write_timeout). |

---

## 4. Acceptance Criteria

Detailed Given/When/Then scenarios with edge cases live in
`.moai/specs/SPEC-SYN-004/acceptance.md`. This section enumerates the
acceptance gate per requirement.

### REQ-SYN4-001a — Ubiquitous: SSE content-type headers

- File `internal/sse/writer.go` exists and exposes `Writer` with
  `WriteEvent`, `WriteComment`, `Flush`, `Close`.
- `test_sse_content_type_set`: response headers include
  `Content-Type: text/event-stream`, `Cache-Control: no-cache`,
  `Connection: keep-alive` on every SSE response.
- `test_sse_headers_absent_on_json_fallback`: negative — JSON
  fallback path emits `Content-Type: application/json`, NOT the SSE
  trio.

### REQ-SYN4-001b — Ubiquitous: W3C SSE wire format

- `test_sse_wire_format_blank_line_terminator`: each emitted event
  ends with `\n\n`; W3C SSE-conformant for all event types
  (`sentence`, `done`, `error`) and for heartbeat comments.
- `test_sse_data_multiline_repeats_prefix`: a JSON payload with
  embedded `\n` is emitted as multiple `data:` lines per the SSE
  spec; survives round-trip via reference EventSource parser.
- `test_sse_comment_wire_format`: heartbeat comment matches
  `: <text>\n\n` grammar exactly.

### REQ-SYN4-001c — Ubiquitous: un-cited content invariant (SYN-002)

- `test_no_uncited_sentence_emitted`: mock synthesis returns a
  `Result.Text` containing one sentence without any `[N]` marker
  (synthetic test-only state — production SYN-002 strip mode would
  pre-filter this); assert that sentence is NOT in any
  `event: sentence` data.
- `test_syn002_invariant_preserved_under_streaming`: every emitted
  `event: sentence`'s `text` field passes the SYN-002 canonical
  regex (`every sentence has ≥ 1 valid [N] marker`).
- Property test NFR-SYN4-002 (b)(c): every `[N]` marker in emitted
  text resolves to a `doc_id` in that event's `citations` array.

### REQ-SYN4-002 — Event-Driven: per-sentence emission

- `test_sentence_segmentation_matches_syn002_regex`: feed a 5-sentence
  paragraph; assert exactly 5 `event: sentence` events emitted in
  order.
- `test_one_sse_event_per_sentence`: invariant — sentence count from
  segmentation == event count.
- `test_event_payload_includes_citations`: each event's `citations`
  field is a non-empty array of `{marker, doc_id, url, title}`
  objects matching the input `Result.Citations`.
- `test_done_event_emitted_on_success`: stream terminates with
  exactly one `event: done` carrying the post-call totals.
- `test_streamed_complete_counter_increments_once_per_call`: counter
  delta == 1 per successful stream.
- `test_sentences_emitted_histogram_observation`: histogram observes
  the count value (e.g. 5).

### REQ-SYN4-003 — State-Driven: heartbeat keepalive

- `test_heartbeat_emits_at_configured_interval`: mock clock; with
  `SYN004_SSE_HEARTBEAT_MS=100` and a 350 ms upstream synth call,
  assert at least 3 `: ping\n\n` lines on the wire.
- `test_heartbeat_disabled_emits_nothing`: with
  `SYN004_SSE_HEARTBEAT_ENABLED=false`, assert zero `: ping`
  comments regardless of stream duration.
- `test_heartbeat_terminates_on_ctx_done`: ctx cancel → heartbeat
  goroutine returns within 100 ms.
- `test_heartbeat_does_not_interleave_with_sentence_events`:
  concurrent emission test under race detector — assert no torn
  writes (no `event: sentenc<heartbeat>e` patterns).

### REQ-SYN4-004 — Unwanted: client disconnect cancellation

- `test_client_disconnect_cancels_upstream_within_deadline`: simulate
  client connection close mid-stream; assert
  `synthesis.Client.Synthesize()` ctx receives cancellation within
  `SYN004_DISCONNECT_CANCEL_MS + 100 ms`.
- `test_client_disconnect_increments_counter`: counter
  `outcome="client_disconnect"` += 1.
- `test_client_disconnect_emits_warn_log`: exactly one WARN log with
  attributes `{request_id, reason:"client_disconnect",
  sentences_emitted_before_disconnect}`.
- `test_client_disconnect_releases_heartbeat_goroutine`: heartbeat
  goroutine exits within 100 ms of ctx cancel.
- `test_client_disconnect_releases_watcher_goroutine`: the
  disconnect-watcher goroutine itself exits within 100 ms of stream
  close. Goroutine-leak detector reports zero leaked goroutines for
  all three goroutines (main writer, heartbeat, watcher) after
  disconnect + cleanup deadline.

### REQ-SYN4-005 — Optional: Accept-header fallback

- `test_accept_missing_falls_back_to_json`: omit Accept header →
  response is `Content-Type: application/json` with body matching
  SPEC-SYN-001 `Result` contract byte-equivalent to a non-streaming
  call.
- `test_accept_application_json_falls_back_to_json`:
  `Accept: application/json` → JSON response.
- `test_accept_fallback_no_sse_overhead`: SSE writer / heartbeat
  goroutine constructors record zero invocations.
- `test_accept_fallback_counter_increments`: counter
  `outcome="accept_fallback_to_json"` += 1.
- `test_accept_text_html_falls_back_to_json`: any non-event-stream
  Accept value triggers fallback.

### REQ-SYN4-006 — Unwanted: slow-client write timeout

- `test_slow_client_write_timeout_fires_within_deadline`: mock
  client accepts headers + first sentence then stalls (does not
  drain TCP receive buffer); assert per-write deadline fires within
  `SYN004_SSE_WRITE_TIMEOUT_MS + 50 ms` jitter.
- `test_slow_client_write_timeout_cancels_upstream`: parent ctx
  passed to `synthesis.Client.Synthesize()` receives cancellation
  after the write timeout fires.
- `test_write_timeout_counter_increments_once`: counter
  `outcome="write_timeout"` += 1 exactly once per request
  (NFR-SYN4-003 invariant).
- `test_write_timeout_emits_error_event_best_effort`: server
  attempts one final `event: error` payload with
  `error_code:"write_timeout"`; if the second write also fails it
  is silently dropped (no panic, no double-counter).
- `test_write_timeout_releases_all_three_goroutines`: goroutine-leak
  detector reports zero leaked goroutines (main writer, heartbeat,
  disconnect watcher) within 100 ms of cancellation.
- `test_write_timeout_emits_warn_log`: exactly one WARN log with
  attributes `{request_id, reason:"write_timeout",
  sentences_emitted_before_timeout}`.

### NFR-SYN4-001 — Latency

- `test_sse_ttfb_within_synth_latency_plus_50ms`: 50 iterations of
  buffered-streamed mode; assert TTFB to first
  `event: sentence` byte is within `synth_latency + 50 ms` p95.
- `test_sse_total_overhead_under_100ms`: end-to-end stream wall-clock
  vs. equivalent non-streaming JSON response → diff p95 ≤ 100 ms.
- `test_heartbeat_cpu_overhead_under_1pct`: 60-second idle stream
  with heartbeat enabled at 15 s interval; assert CPU usage of
  heartbeat goroutine < 1% (process-level sampling).

### NFR-SYN4-002 — Property: wire format + citation invariant

- `test_property_sse_wire_format_conformant` (fuzz/quick): generated
  `Result` inputs produce W3C-conformant SSE byte streams.
- `test_property_lossless_text_reconstruction`: union of
  `event: sentence.text` joined by single space matches
  `Result.Text` modulo whitespace normalization.
- `test_property_terminator_event_uniqueness`: exactly one
  `event: done` OR `event: error` per stream; never both, never
  neither, never `event: sentence` after a terminator.

### NFR-SYN4-003 — Invariant: outcome counter exactly-once per request

- `test_outcome_counter_at_most_once_per_request`: aggregate
  assertion across REQ-002/004/005/006 — the union of all five
  `outcome` label values increments by exactly 1 per request,
  regardless of which terminal state (success, disconnect, write
  timeout, upstream error, accept fallback) wins.
- `test_outcome_counter_race_streamed_complete_vs_disconnect`:
  client disconnects within 1 ms of `event: done` being written;
  assert counter = 1 (whichever fired first wins; no double-count).
- `test_outcome_counter_race_disconnect_vs_write_timeout`: write
  timeout fires concurrently with disconnect detection; assert
  counter = 1.
- `test_outcome_counter_race_streamed_complete_vs_write_timeout`:
  write timeout fires on the final `event: done` write; assert
  counter = 1.
- All race tests run with `go test -race`; sync.Once-style guard
  enforces the invariant deterministically.

---

## 5. Technical Approach (high-level, no implementation code)

Detailed plan, file impact, and test plan live in
`.moai/specs/SPEC-SYN-004/plan.md`. High-level approach:

- **Insertion point**: `cmd/usearch-api` HTTP synthesis handler
  (file location owned by SPEC-IR-001's server scaffolding). The
  handler reads the `Accept` header, dispatches to either the SSE
  path or the existing JSON path.
- **Layering**: `cmd/usearch-api/handlers/...` (HTTP boundary) →
  `internal/streamsynth` (stream orchestration) →
  `internal/synthesis.Client.Synthesize()` (existing single-shot,
  unchanged) AND `internal/sse.Writer` (SSE wire emission). No
  cross-layer leakage; HTTP semantics live at the boundary,
  domain semantics in `streamsynth`.
- **Sentence segmentation**: re-use the SYN-002 canonical regex
  literally — no new regex, no language-specific tweaks. Korean +
  English handled by the punctuation alternative
  `[.!?。！？]\s+|[.!?。！？]$`.
- **Citation resolution per sentence**: scan each segmented sentence
  for `[N]` markers; look up each `N` in the existing
  `Result.Citations` array (`citations[i].marker == N`); attach
  `{marker, doc_id, url, title}` to the event payload. Markers
  outside `[1, len(citations)]` should not appear here because
  `_process_markers()` already stripped them server-side
  (REQ-SYN-002).
- **Heartbeat**: dedicated goroutine wakes every interval, writes
  `: ping\n\n` via the shared `*sse.Writer` (mutex-guarded). Selects
  on `ctx.Done()` for clean shutdown.
- **Disconnect detection**: net/http exposes
  `r.Context().Done()` on TCP close in Go 1.20+. Watcher goroutine
  cancels the parent synthesis ctx and triggers cleanup.
- **Buffered-then-streamed mode (v0)**: `streamsynth.StreamSynthesize`
  invokes `synthesis.Client.Synthesize()` to completion, then drives
  the segmentation + emission loop. TTFB to first sentence ≈ full
  upstream latency. Future token-streaming upgrade replaces the
  buffer step with a streaming consumer of an upstream chunk
  iterator (gated on follow-up SPEC).

---

## 6. Risks (top-level summary)

Detailed risk register lives in `.moai/specs/SPEC-SYN-004/research.md`
§5. Top three for SPEC-author attention:

1. **Backpressure on slow clients (R1)** — slow client cannot drain
   SSE response; HTTP write blocks; upstream LLM holds resources.
   Mitigated by per-write deadline (`SYN004_SSE_WRITE_TIMEOUT_MS`)
   and ctx cancellation propagation. Encoded as REQ-SYN4-006
   (Unwanted, P0); counter `outcome="write_timeout"` covered by
   NFR-SYN4-003 exactly-once invariant.
2. **Client disconnect mid-stream (R2)** — without explicit handling,
   LLM call continues to completion and bills cost.
   Mitigated by `r.Context().Done()` watcher + `SYN004_DISCONNECT_CANCEL_MS`
   ctx cancellation deadline. Encoded as REQ-SYN4-004 (HARD).
3. **Proxy buffering defeats streaming (R3)** — nginx/ALB/Cloudflare
   buffer responses by default. Mitigated by heartbeat
   (REQ-SYN4-003) emitting `: ping\n\n` every 15 s and by documenting
   required nginx config (`proxy_buffering off;`) in `.env.example`.

---

## 7. References

Internal:

- `cmd/usearch-api/main.go:1-40` — current stub; future SSE-emitting
  server entry point (gated on SPEC-IR-001).
- `internal/synthesis/client.go:39-100` — `Client.Synthesize()`,
  consumed read-only by SPEC-SYN-004; @MX:ANCHOR at line 39.
- `internal/synthesis/types.go` — `Result`, `Citation` Go shapes
  (UNCHANGED).
- `services/researcher/src/researcher/synthesis.py:151-220` —
  Python sidecar `synthesize()` (UNCHANGED; sidecar streaming
  upgrade is a documented exclusion).
- `services/researcher/src/researcher/gateway.py:36-72` — single-shot
  httpx pattern (sidecar streaming gap, research §6 R5).
- `.moai/specs/SPEC-SYN-001/spec.md:189-204` — REQ-SYN-001 through
  REQ-SYN-007; NFR-SYN-001 through NFR-SYN-004 contract preserved.
- `.moai/specs/SPEC-SYN-002/spec.md:159-163` — REQ-SYN2-001 through
  REQ-SYN2-005 citation faithfulness contract preserved verbatim.
- `.moai/specs/SPEC-IR-001/` — HTTP server scaffolding prerequisite.
- `.moai/project/roadmap.md:66` — SPEC-SYN-004 row.
- `.moai/project/roadmap.md:124` — M4 3-way parallelization plan.
- `.moai/specs/SPEC-SYN-004/research.md` — companion research artifact.

External (verify via WebFetch in Run phase per anti-hallucination policy):

- W3C "Server-Sent Events" living standard,
  whatwg.org/multipage/server-sent-events.html.
- OpenAI Chat Completions streaming reference,
  platform.openai.com/docs/api-reference/streaming.
- Anthropic Messages streaming reference,
  docs.anthropic.com/en/api/messages-streaming.
- LiteLLM proxy streaming pass-through, litellm.ai docs.
- nginx `proxy_buffering off` docs,
  nginx.org/en/docs/http/ngx_http_proxy_module.html.
- Go `net/http.Flusher`, `http.ResponseController` (Go 1.20+ docs).
- gpt-researcher SSE pattern reference,
  github.com/assafelovic/gpt-researcher.

---

*End of SPEC-SYN-004 v0.1 (draft).*
