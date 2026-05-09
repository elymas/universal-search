# SPEC-SYN-004 Research — Streaming Response (SSE)

Phase 0.5 Deep Research artifact for SPEC-SYN-004.
Status: research-complete; informs spec.md / plan.md / acceptance.md.
Date: 2026-05-09.

---

## 1. Pipeline Position

```
adapter fanout → SPEC-FAN-001 dedup → SPEC-SYN-003 cluster → synthesis sidecar
   (Go)              (Go)                 (Go, Wave 4)        (Python sidecar, SYN-001)
                                                                      │
                                                                      ▼
                                                            SynthesizeResponse (single-shot today)
                                                                      │
                                                                      ▼
                                                       internal/synthesis.Client.Synthesize()
                                                                      │
                                                                      ▼
                                                      cmd/usearch (CLI today; cmd/usearch-api stub)
                                                                      │
                                                                      ▼
                                                       SPEC-SYN-004 — SSE emit (this SPEC)
                                                                      │
                                                                      ▼
                                                              HTTP client (browser/CLI)
```

SPEC-SYN-004 inserts a **streaming surface** between the synthesis
result and the HTTP client of `cmd/usearch-api`. It does NOT touch
synthesis logic itself (SYN-001) and does NOT modify the citation
marker contract (SYN-002).

---

## 2. Codebase Trace — Current State

### 2.1 `cmd/usearch-api/` is a STUB

`cmd/usearch-api/main.go:1-40` (current state):

- Single file (`main.go`, 41 LOC).
- Initializes `obs.Init` and prints
  `"usearch-api: not implemented (see SPEC-IR-001)"` to stderr before
  exiting with code 0.
- **No HTTP server is wired up today.** Full server scaffolding
  (router, listener, request handlers) is owned by SPEC-IR-001 and is
  pre-requisite to SPEC-SYN-004's HTTP-level work.

**Implication for SPEC-SYN-004**: SYN-004's REQs MUST be expressed
against the *future* `cmd/usearch-api` server surface that SPEC-IR-001
will deliver. SYN-004 cannot ship before SPEC-IR-001's HTTP server
exists. The SPEC frontmatter therefore declares `depends_on:
[SPEC-IR-001, SPEC-SYN-001, SPEC-SYN-002]`.

### 2.2 Synthesis sidecar is single-shot today

`services/researcher/src/researcher/synthesis.py:151-220`
(`synthesize()` async function):

- Calls `gateway.complete(messages, model, lang)` exactly once.
- `gateway.complete()` at
  `services/researcher/src/researcher/gateway.py:36-72` uses
  `httpx.AsyncClient(...).post(url, json=payload)` with the OpenAI
  Chat Completions REST shape — **no `stream: true` parameter**.
- Returns the full `text` once the upstream LiteLLM/OpenAI call
  completes. Citations are extracted post-hoc via
  `_process_markers(text_raw, docs)` at
  `synthesis.py:66-118` — i.e. the *full* LLM output is in hand
  before any `[N]` marker is validated.

**Implication for SPEC-SYN-004**: There is a real **upstream streaming
gap**. To stream tokens from the Python sidecar to the Go server, the
sidecar must be upgraded to (a) call LiteLLM with `stream: true`,
(b) parse the SSE chunks, and (c) re-emit them over the FastAPI
endpoint. This is non-trivial work and **falls outside SPEC-SYN-004's
scope** by the rules below — SYN-004 concerns the Go HTTP server's
SSE surface. The sidecar streaming upgrade is a **prerequisite gap**
documented in §6 below; SPEC-SYN-004 declares a fallback strategy
("buffered fallback over SSE") so that SYN-004 can ship even if the
sidecar streaming work is deferred to a follow-up SPEC.

### 2.3 Go-side synthesis client is single-shot

`internal/synthesis/client.go:61-100` (`Client.Synthesize()`):

- Builds a JSON payload via `c.buildPayload(reqID, query, lang, docs)`.
- POSTs once via `c.doOnce()` wrapped in `withRetry()` (REQ-SYN-005;
  2 retries with exponential backoff).
- Returns `(Result, error)` synchronously after the full response is
  decoded.
- Has `@MX:ANCHOR` at `client.go:39` (fan_in ≥ 3).

**Implication for SPEC-SYN-004**: When SYN-004 implements streaming,
it can either (a) keep the existing single-shot `Client.Synthesize()`
and **buffer-then-stream** (server reads the full result, then chunks
it into SSE events with synthetic pacing — reduces TTFB only modestly
but preserves backward compatibility), or (b) add a new
`Client.SynthesizeStream(ctx, ...) (<-chan StreamEvent, error)` API
that requires sidecar streaming support. Option (b) is cleaner but
requires the sidecar gap to be closed first. SPEC-SYN-004 ships
option (a) as the v0 floor and declares a delta marker for option (b)
gated on the sidecar upgrade.

### 2.4 SYN-002 citation invariant

`.moai/specs/SPEC-SYN-002/spec.md:159-163` REQ-SYN2-001 through
REQ-SYN2-005:

- **REQ-SYN2-001 (Ubiquitous)**: every claim (sentence) in
  `SynthesizeResponse.text` carries at least one `[N]` marker
  resolving to a `doc_id` in input `docs[]`.
- **REQ-SYN2-002 (Event-Driven)**: when un-cited sentences are
  detected, the sidecar retries once with stricter prompt; on
  retry-failure, the configured mode (`strip` / `reject` / `off`)
  applies.
- The faithfulness gate runs **after** the full LLM output is in hand
  (sentence-level segmentation needs the complete text).

**Implication for SPEC-SYN-004**: The faithfulness gate is
**incompatible with naïve token-by-token streaming** — the SYN-002
gate cannot validate sentences until they are complete. Two options:

1. **Sentence-level streaming**: emit one SSE event per *complete
   sentence* after the SYN-002 gate has validated it. This preserves
   SYN-002's invariant but caps perceived streaming granularity at
   sentence boundaries (~30-100 chars per chunk).
2. **Withhold-until-cited streaming**: emit tokens as they arrive,
   but **buffer un-cited prefix** until the LLM emits the `[N]`
   marker that resolves the prefix. Once the marker arrives and
   `_process_markers()` validates it against `len(docs)`, the
   buffered prefix + marker are emitted as a single SSE event. If
   no marker arrives within a sentence terminator, SYN-002's
   `strip`/`reject` mode applies — the sentence is silently dropped
   from the stream (strip) or the stream terminates with an
   `error` event (reject).

SPEC-SYN-004 selects **sentence-level streaming** as the v0 default
because it is simpler, deterministic, and provably preserves
SYN-002's invariant. The "withhold-until-cited" variant is documented
as a future enhancement (out of scope for v0).

---

## 3. SSE — Library Survey & Reference Patterns

### 3.1 SSE wire format

The W3C "Server-Sent Events" spec defines `text/event-stream` with
fields:

- `event: <type>` — optional event type (defaults to `message`)
- `data: <payload>` — required; can be repeated for multi-line
  payloads (the receiver concatenates with `\n`)
- `id: <string>` — optional cursor for resume
- `retry: <ms>` — optional client-side reconnect hint

A complete SSE event is terminated by a **blank line** (`\n\n`).

A `comment` line begins with `:` and is ignored by clients — the
canonical heartbeat mechanism for defeating proxy buffering is to
emit a `: ping\n\n` comment every N seconds.

### 3.2 Reference: gpt-researcher SSE forks

The `gpt-researcher` reference upstream supports SSE via FastAPI's
`StreamingResponse` and emits `event: message` chunks during the
research-loop. Several community forks implement
`StreamingResponse(generator(), media_type="text/event-stream")`
patterns. **Verify in Run phase via WebFetch
(github.com/assafelovic/gpt-researcher).**

### 3.3 Reference: Anthropic / OpenAI streaming SDKs

Anthropic Messages API and OpenAI Chat Completions both support
`stream: true` over SSE with chunk shapes:

- Anthropic: `event: content_block_delta` with
  `data: {"delta": {"text": "..."}}` per token batch.
- OpenAI: `data: {"choices": [{"delta": {"content": "..."}}]}` per
  token batch, terminated by `data: [DONE]`.

LiteLLM proxy (gateway.py) passes `stream: true` through to the
underlying provider transparently. **Verify in Run phase via
WebFetch (litellm.ai docs / OpenAI streaming docs).**

### 3.4 Go SSE libraries

Surveyed candidates (verify versions/quality in Run phase):

| Library | Notes |
|---|---|
| `net/http` (stdlib) + `http.Flusher` | Sufficient for v0; no extra dependency. Pattern: `w.Header().Set("Content-Type","text/event-stream")` → `fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ...)` → `w.(http.Flusher).Flush()`. Used by hashicorp/consul for SSE endpoints. |
| `github.com/r3labs/sse/v2` | Wraps stdlib with broker + client abstractions. Useful for multi-subscriber broadcasts. **Likely overkill** for SYN-004's per-request streaming. |
| `github.com/donovanhide/eventsource` | Client-side library; not needed for server emission. |

**Decision (deferred to Run phase)**: prefer `net/http` stdlib pattern
unless multi-subscriber broadcast is required (it is not for
SYN-004). Hand-rolled SSE writer is ~50 LOC and avoids dependency
churn.

### 3.5 Heartbeat / proxy-buffering defeat

Common deployment topologies (nginx, AWS ALB, Cloudflare) buffer HTTP
responses by default. SSE requires explicit configuration:

- **nginx**: `proxy_buffering off;` + `proxy_cache off;` in the
  location block. Without this, nginx batches responses and breaks
  perceived streaming.
- **Heartbeat**: emit `: ping\n\n` (SSE comment) every N seconds
  (default 15 s) to keep TCP and proxy idle timers fresh and to
  signal liveness when no LLM tokens arrive (e.g. during the LiteLLM
  upstream call's TTFB).
- **Browser EventSource API** treats SSE comments as no-ops, so
  heartbeats are invisible to the application layer.

SPEC-SYN-004 makes the heartbeat **interval configurable** via
`SYN004_SSE_HEARTBEAT_MS` (default 15000) and the **enabled flag**
via `SYN004_SSE_HEARTBEAT_ENABLED` (default true).

---

## 4. Citation Marker Streaming Strategy

### 4.1 The "partial citation" problem

If we naïvely stream LLM tokens character by character, the client
can see `"Event X happened in Seoul ["` before the `1]` marker
arrives. This violates the spirit of SYN-002 (the `[1]` is not yet
resolvable to a `doc_id` because the marker is incomplete).

### 4.2 Selected strategy: sentence-level streaming

Emit one SSE `event: sentence` per complete sentence, where a sentence
is defined by SYN-002's canonical regex
`[.!?。！？]\s+|[.!?。！？]$`. Each event payload is JSON:

```json
{
  "request_id": "...",
  "sentence_index": 0,
  "text": "Event X happened in Seoul [1].",
  "citations": [{"marker": 1, "doc_id": "doc-uuid-1", "url": "..."}]
}
```

Pros:
- SYN-002 invariant trivially preserved — sentences only emit *after*
  `_process_markers()` validates their citations.
- Deterministic event boundaries; easy to test.
- Granular enough for typical 4-8-sentence synthesis output (per
  `synthesis.py:48` "Output one paragraph (4-8 sentences)").

Cons:
- TTFB to first sentence is bounded by LLM's first sentence-terminator
  emission latency (~300-800 ms typical for first claim).
- Less granular than token-streaming; perceived speed is sentence
  cadence, not token cadence.

### 4.3 Cited-marker-first invariant

[HARD CONSTRAINT] **A sentence MUST NOT be emitted to the SSE stream
before its `[N]` markers are validated.** This means:

- If the sidecar is single-shot today (current state), the server
  buffers the full response, runs `_process_markers()`, then emits
  sentence-by-sentence with synthetic pacing. TTFB ≈ full LLM
  latency, but the streaming surface is correct.
- If/when the sidecar supports token streaming (future), the server
  accumulates tokens until a sentence terminator is detected, then
  validates citations within that sentence and either emits or
  withholds per SYN-002 mode.

This invariant is encoded as REQ-SYN4-001c (Ubiquitous) in spec.md.

---

## 5. Risks (Top 5 — for spec.md §5 risk register)

### R1. Backpressure on slow clients [P0]

**Risk**: A slow client (mobile, throttled network) cannot drain the
SSE response fast enough; the server's HTTP write buffer fills; the
goroutine writing SSE events blocks indefinitely; the upstream LLM
call holds open longer than necessary.

**Mitigation**:
- Per-request **write deadline** via `http.ResponseController.SetWriteDeadline`
  (Go 1.20+). Default `SYN004_SSE_WRITE_TIMEOUT_MS=5000` per
  individual write call.
- On write timeout: cancel the parent `context.Context`, terminate
  the upstream `synthesis.Client.Synthesize()` call, emit a final
  `event: error` with `{reason: "client_slow"}`, close the stream.
- Counter `usearch_syn004_disconnects_total{reason="write_timeout"}`
  increments.

### R2. Client disconnect mid-stream [P0]

**Risk**: Client closes the connection (browser tab close, network
loss) while the upstream LLM is still generating. Without explicit
handling, the LLM call continues to completion and bills cost; the
SSE goroutine writes to a closed connection (recoverable error but
wasted work).

**Mitigation**:
- Watch `r.Context().Done()` (FastAPI / net/http both expose request
  context cancellation on TCP close in modern versions).
- On disconnect detection: cancel the parent ctx within
  `SYN004_DISCONNECT_CANCEL_MS` (default 1000 ms) — propagates to
  `synthesis.Client.Synthesize()` ctx, which propagates to LiteLLM
  HTTP call, which (per OpenAI/Anthropic streaming docs) terminates
  upstream generation.
- Counter `usearch_syn004_disconnects_total{reason="client_close"}`
  increments.

### R3. Proxy buffering defeats streaming [P1]

**Risk**: nginx / ALB / Cloudflare buffers the SSE response; client
sees no events until the entire response is generated; perceived
latency is identical to non-streaming.

**Mitigation**:
- Heartbeat `: ping\n\n` every `SYN004_SSE_HEARTBEAT_MS` (default
  15000 ms) keeps the TCP connection from going idle and forces
  most proxies to flush.
- Document required nginx config (`proxy_buffering off;`) in
  `.env.example` and project README.
- Acceptance test asserts heartbeat is emitted at the configured
  interval when the upstream LLM is slow.

### R4. SYN-002 invariant accidentally violated by streaming [P0]

**Risk**: A streaming implementation emits LLM tokens before
`_process_markers()` validates them; out-of-range `[N]` markers reach
the client; SYN-002's structural invariant breaks.

**Mitigation** (HARD constraint, encoded as REQ-SYN4-002 / REQ-SYN4-003):
- Sentence-level streaming only — sentences emit after citation
  validation.
- The SSE writer never accepts un-validated text from the upstream
  buffer.
- Acceptance test §4.1 asserts that an LLM mock emitting an
  out-of-range marker (`[99]` when only 3 docs exist) produces a
  stream where the offending sentence is **stripped** (per SYN-002
  `strip` mode default) or the stream terminates with `event: error`
  (per SYN-002 `reject` mode).

### R5. Sidecar streaming gap (prerequisite) [P1]

**Risk**: The Python sidecar does not support `stream: true` today
(`gateway.py:36-72`). SPEC-SYN-004 cannot deliver token-level
streaming until the sidecar is upgraded.

**Mitigation**:
- v0 ships **buffered-then-streamed** mode: server reads the full
  `SynthesizeResponse`, segments by sentence, emits SSE events with
  optional pacing (`SYN004_BUFFERED_PACE_MS`, default 0 — emit
  immediately).
- Acceptance test asserts buffered mode produces a valid SSE stream
  that is **byte-equivalent to non-buffered mode** for a
  synthetically-paced single-shot response.
- The token-streaming upgrade is a documented exclusion (§Exclusions
  in spec.md) and gated on a follow-up SPEC (tentatively
  SPEC-SYN-006 or SPEC-SYN-004-v2).

---

## 6. Prerequisite Gap — Sidecar Streaming Support

**Status**: NOT IN SCOPE for SPEC-SYN-004 v0.

**Description**: Token-level streaming from `services/researcher/`
requires:

1. `gateway.py:complete()` upgraded to support
   `stream: bool = False` parameter; when true, returns
   `AsyncIterator[ChatCompletionChunk]` instead of a tuple.
2. `synthesis.py:synthesize()` upgraded to consume the iterator and
   either (a) buffer-then-validate-then-emit, or (b) sentence-stream
   with sliding citation validation.
3. FastAPI `app.py:synthesize_endpoint()` upgraded to return
   `StreamingResponse(generator(), media_type="text/event-stream")`
   when the request includes `stream: true` field.

**Recommendation**: Track as follow-up SPEC after SPEC-SYN-004 v0
ships with buffered fallback. Likely candidate ID:
**SPEC-SYN-006** (sidecar token streaming) or
**SPEC-SYN-004-v2** as an in-place evolution.

This SPEC's REQs and acceptance criteria are written so that **both**
buffered (v0) and token-streamed (future) implementations satisfy
them — the gating contract is "SSE event boundaries respect
SYN-002's per-sentence citation invariant", regardless of upstream
chunk granularity.

---

## 7. References

### Internal

- `cmd/usearch-api/main.go:1-40` — current stub; future SSE-emitting
  server entry point (gated on SPEC-IR-001).
- `internal/synthesis/client.go:39-100` — `Client.Synthesize()`,
  current single-shot Go-side synthesis client; @MX:ANCHOR at line 39.
- `internal/synthesis/types.go` — `Result` / `Citation` Go shapes
  (UNCHANGED by SPEC-SYN-004).
- `services/researcher/src/researcher/synthesis.py:151-220` —
  `synthesize()` async function, current single-shot.
- `services/researcher/src/researcher/gateway.py:36-72` —
  `Gateway.complete()`, single-shot httpx POST (no streaming
  support today).
- `services/researcher/src/researcher/synthesis.py:66-118` —
  `_process_markers()`, citation validation post-hoc.
- `.moai/specs/SPEC-SYN-001/spec.md:189-204` — REQ-SYN-001 through
  REQ-SYN-007; NFR-SYN-001 through NFR-SYN-004.
- `.moai/specs/SPEC-SYN-002/spec.md:159-163` — REQ-SYN2-001 through
  REQ-SYN2-005 (citation faithfulness contract preserved by
  SPEC-SYN-004).
- `.moai/specs/SPEC-IR-001/` — HTTP server scaffolding (prerequisite,
  not yet implemented per `cmd/usearch-api/main.go:31-32` stub
  reference).
- `.moai/project/roadmap.md:66` — SPEC-SYN-004 row.
- `.moai/project/roadmap.md:124` — M4 3-way parallelization.

### External (verify via WebFetch in Run phase per anti-hallucination policy)

- W3C "Server-Sent Events" spec (HTML Living Standard,
  whatwg.org/multipage/server-sent-events.html).
- OpenAI Chat Completions streaming docs
  (platform.openai.com/docs/api-reference/streaming).
- Anthropic Messages streaming docs
  (docs.anthropic.com/en/api/messages-streaming).
- LiteLLM proxy streaming pass-through (litellm.ai docs).
- nginx `proxy_buffering off` documentation
  (nginx.org/en/docs/http/ngx_http_proxy_module.html).
- gpt-researcher SSE pattern reference
  (github.com/assafelovic/gpt-researcher).
- Go `net/http` Flusher / ResponseController pattern (Go 1.20+ docs).

---

## 8. Decisions Locked In Research Phase

| # | Decision | Rationale |
|---|---|---|
| 1 | SSE over WebSocket | Browser support universal via EventSource; auto-reconnect built in; simpler than bidirectional protocol; SYN-004 is one-way (server → client). |
| 2 | Sentence-level event granularity | Provably preserves SYN-002 invariant; deterministic event boundaries; matches "1 paragraph (4-8 sentences)" output shape. |
| 3 | Buffered fallback (v0) | Sidecar streaming gap (R5) is prerequisite; buffered mode unblocks SPEC-SYN-004 ship without waiting for sidecar upgrade. |
| 4 | `net/http` stdlib over r3labs/sse | No multi-subscriber broadcast needed; ~50 LOC hand-rolled writer avoids dependency churn. |
| 5 | Heartbeat default 15000 ms | Aligns with nginx default `keepalive_timeout 75s` and Cloudflare 100s idle timer; 15 s leaves comfortable headroom. |
| 6 | Disconnect cancellation default 1000 ms | Conservative — gives in-flight HTTP write 1 s to flush before forcing upstream LLM cancel; tunable via env. |
| 7 | Accept-header content-negotiation | `text/event-stream` triggers SSE; missing/other Accept falls back to single JSON response (backward compatibility with existing CLI clients). |

---

*End of SPEC-SYN-004 research.md.*
