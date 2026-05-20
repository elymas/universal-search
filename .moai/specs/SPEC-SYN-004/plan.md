# SPEC-SYN-004 Implementation Plan

Companion to `.moai/specs/SPEC-SYN-004/spec.md`.
Status: draft (pre-annotation).
Methodology: TDD (RED → GREEN → REFACTOR), per project default
(`.moai/config/sections/quality.yaml` `development_mode: tdd`).

---

## 1. Approach (one-page summary)

SPEC-SYN-004 introduces three NEW Go packages and modifies the
`cmd/usearch-api` HTTP synthesis handler to perform Accept-header
content negotiation and dispatch to either an SSE streaming path or
the existing buffered JSON path:

```
cmd/usearch-api/handlers/synthesis.go (file path owned by SPEC-IR-001)
        │
        ├── (Accept: text/event-stream) ──► internal/streamsynth.StreamSynthesize(...)
        │                                          │
        │                                          ├── synthesis.Client.Synthesize(ctx, ...)  [UNCHANGED]
        │                                          │       returns Result (full text + citations)
        │                                          │
        │                                          ├── segment Result.Text by SYN-002 regex
        │                                          ├── resolve [N] markers via Result.Citations
        │                                          ├── emit one event: sentence per validated sentence
        │                                          └── emit event: done
        │                                                   ▲
        │                                                   │ writes via
        │                                                   │
        │              internal/sse.Writer (W3C SSE wire format)
        │                              ▲
        │                              │ heartbeat goroutine
        │                              │ emits `: ping\n\n` every interval
        │              internal/sse.RunHeartbeat(ctx, writer, interval)
        │
        └── (otherwise) ───────────────► synthesis.Client.Synthesize(...) → marshal Result as JSON  [UNCHANGED]
```

The SSE path is **buffered-then-streamed** in v0: the upstream
synthesis call is invoked synchronously, then the result text is
segmented + emitted with sentence granularity. Token-level streaming
from the Python sidecar is a **prerequisite gap** (research §6 R5)
gated on a follow-up SPEC. SYN-004 v0 ships a streaming surface that
is contractually correct (preserves SYN-002 invariant) regardless of
upstream chunk granularity.

---

## 2. File Impact

### 2.1 NEW files

| File | Purpose | Approximate LOC |
|---|---|---|
| `internal/sse/types.go` | `Writer`, `Event` value types, error sentinels | 60 |
| `internal/sse/writer.go` | SSE writer wrapping `http.ResponseWriter` + `http.ResponseController`; `WriteEvent`, `WriteComment`, `Flush`, `Close`; mutex-guarded for concurrent heartbeat goroutine | 140 |
| `internal/sse/heartbeat.go` | `RunHeartbeat(ctx, writer, interval) error`; ticker-based; selects on ctx.Done() | 60 |
| `internal/sse/writer_test.go` | Wire-format conformance, blank-line termination, `WriteComment` correctness, mutex/race assertions | 150 |
| `internal/sse/heartbeat_test.go` | Cadence assertion via mock clock, ctx cancellation cleanup, disabled mode | 100 |
| `internal/streamsynth/types.go` | `StreamRequest`, `StreamStats`, error types, `EventType` enum (`sentence`, `done`, `error`) | 70 |
| `internal/streamsynth/options.go` | `OptionsFromEnv()`, defaults, validation for the five `SYN004_*` env vars | 90 |
| `internal/streamsynth/streamsynth.go` | `StreamSynthesize(ctx, w, req)`: invokes synthesis client, segments by SYN-002 regex, resolves citations, emits events | 200 |
| `internal/streamsynth/segmenter.go` | Sentence segmentation using SYN-002 canonical regex; pure function, fully testable | 80 |
| `internal/streamsynth/citations.go` | Per-sentence citation resolver: scan `[N]` markers, look up in `Result.Citations`, build event payload's `citations` array | 70 |
| `internal/streamsynth/observability.go` | `emit()` helper for `obs.StreamSynthOutcomes` and `obs.StreamSynthSentencesEmitted`; structured logger | 60 |
| `internal/streamsynth/streamsynth_test.go` | All 5 EARS REQ acceptance tests; sentence emission, citation embedding, done event, counter assertions | 350 |
| `internal/streamsynth/segmenter_test.go` | Korean + English regex correctness; edge cases (single sentence, empty, terminator at end) | 100 |
| `internal/streamsynth/citations_test.go` | Marker resolution; out-of-range markers; multi-marker sentences | 80 |
| `internal/streamsynth/property_test.go` | NFR-SYN4-002 property test (W3C wire format + citation invariant + lossless reconstruction) | 130 |
| `internal/streamsynth/bench_test.go` | NFR-SYN4-001 benchmarks (TTFB, total overhead, heartbeat CPU) | 80 |
| `internal/obs/metrics/streamsynth.go` | Two new Prometheus collectors + `registerStreamSynth(pr)` helper | 60 |
| `cmd/usearch-api/handlers/synthesis_stream_test.go` (path per SPEC-IR-001 layout) | Integration tests: SSE happy path, client-disconnect cancellation, Accept-header fallback | 220 |

Total NEW: ~2,100 LOC (~960 production + ~1,140 test).

### 2.2 MODIFY files

| File | Change | Approximate LOC delta |
|---|---|---|
| `cmd/usearch-api/handlers/synthesis.go` (path per SPEC-IR-001) | Add Accept-header content negotiation; dispatch to `streamsynth.StreamSynthesize` for SSE clients OR existing buffered JSON path otherwise; emit accept-fallback counter on JSON path | +60 / -0 |
| `cmd/usearch-api/main.go` | Register the synthesis handler with the router (exact API per SPEC-IR-001); wire env-var loading via `streamsynth.OptionsFromEnv()` | +15 |
| `internal/obs/metrics/metrics.go` | Register the two new collectors via `registerStreamSynth(pr)`; amend cardinality allowlist for `outcome` label values (`streamed_complete`, `client_disconnect`, `write_timeout`, `error_upstream`, `accept_fallback_to_json`) | +18 |
| `internal/obs/obs.go` | Re-export `obs.StreamSynthOutcomes`, `obs.StreamSynthSentencesEmitted` | +6 |
| `.env.example` | Add the five `SYN004_*` env vars + nginx `proxy_buffering off` recommendation comment | +20 |

### 2.3 EXISTING — UNCHANGED (verified by integration test suite)

| File | Why touched but verified unchanged |
|---|---|
| `internal/synthesis/client.go` | `Client.Synthesize()` consumed read-only; signature, retry semantics, observability all unchanged. SYN-001 NFR-SYN-001 / NFR-SYN-004 preserved. |
| `internal/synthesis/types.go` | `Result`, `Citation` Go shapes unchanged. SSE event payloads embed these existing shapes. |
| `services/researcher/src/researcher/synthesis.py` | Python sidecar `synthesize()` unchanged. Token-level streaming upgrade is a documented exclusion. |
| `services/researcher/src/researcher/gateway.py` | LiteLLM gateway unchanged. Sidecar streaming gap (research §6 R5) is documented; not addressed by this SPEC. |
| `services/researcher/src/researcher/app.py` | FastAPI `/synthesize` endpoint unchanged. SYN-001 REQ-SYN-001 contract preserved. |
| `pkg/types/normalized_doc.go` | UNCHANGED. SPEC-SYN-004 does not touch domain types. |
| `internal/synthcluster/` (if SPEC-SYN-003 has shipped) | UNCHANGED. SYN-004 sits downstream of synthesis, not clustering. |
| `internal/fanout/` | UNCHANGED. |

---

## 3. Milestone Sequencing (priority-based, no time estimates)

### Milestone P0-A — Skeleton + RED (priority HIGH)

Order: 1 of 5.

- Create `internal/sse/`, `internal/streamsynth/` package directories.
- Add `internal/sse/types.go` with `Writer` struct, `Event` type,
  error sentinels.
- Add `internal/streamsynth/types.go` with `StreamRequest`,
  `StreamStats`, `EventType` enum.
- Add `internal/streamsynth/options.go` with `OptionsFromEnv()` and
  validation.
- Add stub `internal/sse/writer.go` and `internal/sse/heartbeat.go`
  with stubs returning `errors.New("not implemented")`.
- Add stub `internal/streamsynth/streamsynth.go` with stub
  `StreamSynthesize` returning `errors.New("not implemented")`.
- Write all tests for REQ-SYN4-001a/001b/001c, REQ-SYN4-002, REQ-SYN4-003, REQ-SYN4-004, and REQ-SYN4-005 (RED phase).
  Tests fail on the stubs.
- Write property test for NFR-SYN4-002 (RED).
- Write benchmark scaffolding for NFR-SYN4-001 (RED).

Exit gate: `go test ./internal/sse/... ./internal/streamsynth/...`
shows expected RED failures across all REQ test groups.

### Milestone P0-B — SSE writer + heartbeat (GREEN for REQ-SYN4-001a + REQ-SYN4-001b wire format + REQ-SYN4-003)

Order: 2 of 5.

- Implement `internal/sse/writer.go`: `WriteEvent(eventType, data)`
  emits `event: <type>\ndata: <data>\n\n` via `http.ResponseWriter`,
  flushes via `http.Flusher`, write-deadline via
  `http.ResponseController.SetWriteDeadline`. Mutex guards
  concurrent writes from heartbeat + main goroutines.
- Implement `internal/sse/heartbeat.go`: `time.NewTicker(interval)`;
  loop on ticker C; emits `WriteComment("ping")` (translates to
  `: ping\n\n`); selects on `ctx.Done()`.
- Implement `WriteComment` in writer for `: <text>\n\n`.

Exit gate: `internal/sse/...` GREEN. REQ-SYN4-001a + REQ-SYN4-001b
wire-format tests GREEN. REQ-SYN4-003 heartbeat cadence +
disabled-mode + ctx-cancel tests GREEN. `go test -race
./internal/sse/...` PASS.

### Milestone P0-C — streamsynth orchestration (GREEN for REQ-SYN4-002 + REQ-SYN4-001c invariant)

Order: 3 of 5.

- Implement `internal/streamsynth/segmenter.go`: pure-function
  sentence segmentation using SYN-002 canonical regex
  `[.!?。！？]\s+|[.!?。！？]$`. Returns `[]string` sentences in
  order.
- Implement `internal/streamsynth/citations.go`: per-sentence
  `[N]` marker scanner; look up in `Result.Citations`; build the
  per-event citation array.
- Implement `internal/streamsynth/streamsynth.go`
  `StreamSynthesize`:
  1. Invoke `synthesis.Client.Synthesize(ctx, query, lang, docs)`.
  2. Segment `Result.Text` via segmenter.
  3. For each sentence: resolve citations, build event payload, emit
     via `sse.Writer.WriteEvent("sentence", json)`.
  4. After loop: emit `event: done` with totals.
  5. On error: emit `event: error` and return.
- Implement `internal/streamsynth/observability.go`:
  `emit(outcome)` increments `obs.StreamSynthOutcomes`;
  `observeSentenceCount(n)` observes the histogram.

Exit gate: REQ-SYN4-001c invariant tests GREEN (no un-cited content
emitted). REQ-SYN4-002 segmentation + per-sentence emission + done
event tests GREEN. Property test (NFR-SYN4-002) GREEN.

### Milestone P0-D — Disconnect handling + write timeout + observability collectors (GREEN for REQ-SYN4-004, REQ-SYN4-005, REQ-SYN4-006, NFR-SYN4-003)

Order: 4 of 5.

- Implement client-disconnect watcher in
  `streamsynth.StreamSynthesize`: spawn watcher goroutine that
  selects on `r.Context().Done()`; on disconnect, cancel the parent
  ctx (passed to `synthesis.Client.Synthesize`) within
  `SYN004_DISCONNECT_CANCEL_MS`. Watcher goroutine MUST exit within
  100 ms after stream close (REQ-SYN4-004 D3 fix).
- Implement slow-client write timeout (REQ-SYN4-006) in
  `internal/sse/writer.go`: wrap each `Write` via
  `http.ResponseController.SetWriteDeadline(SYN004_SSE_WRITE_TIMEOUT_MS)`;
  on `os.ErrDeadlineExceeded`, cancel parent ctx, increment
  `outcome="write_timeout"`, attempt one best-effort `event: error`
  payload with `error_code:"write_timeout"`, emit WARN log, and
  release all three goroutines (main writer, heartbeat, watcher).
- Implement at-most-once outcome counter guard (NFR-SYN4-003) via a
  `sync.Once`-style sentinel inside `streamsynth.StreamSynthesize` so
  that race windows between `streamed_complete` / `client_disconnect`
  / `write_timeout` resolve to a single counter increment per
  request lifecycle.
- Wire all five outcome counters (`streamed_complete`,
  `client_disconnect`, `write_timeout`, `error_upstream`,
  `accept_fallback_to_json`).
- Add the two new Prometheus collectors and the
  `registerStreamSynth(pr)` helper in
  `internal/obs/metrics/streamsynth.go`. Update the metric label
  allowlist for the new `outcome` values.
- Re-export collector handles in `internal/obs/obs.go`.
- Implement Accept-header content negotiation in the
  `cmd/usearch-api` synthesis handler. Dispatch to either
  `streamsynth.StreamSynthesize` or the existing JSON path. On JSON
  path, increment `outcome="accept_fallback_to_json"`.

Exit gate: REQ-SYN4-004 disconnect-cancellation + counter + WARN log
+ goroutine-leak tests (3 goroutines) GREEN. REQ-SYN4-005 fallback +
zero-overhead + counter tests GREEN. REQ-SYN4-006 write-timeout +
best-effort error event + 3-goroutine cleanup tests GREEN.
NFR-SYN4-003 race-window tests (3 pair tests) GREEN under
`go test -race`. Cardinality allowlist amendment validated by
existing observability test (mirror SPEC-SYN-002 / SPEC-SYN-003
pattern).

### Milestone P0-E — Quality gates + sync (REFACTOR + sync)

Order: 5 of 5.

- Pre-submission self-review per
  `.claude/rules/moai/workflow/workflow-modes.md` Pre-submission
  Self-Review section: review full diff for unnecessary
  abstractions / premature generalization.
- @MX tag application:
  - `streamsynth.StreamSynthesize` (public entry point, fan_in ≥ 2:
    HTTP handler + tests; will reach 3+ if MCP/CLI variants land
    later) → `@MX:ANCHOR` with `@MX:REASON` and
    `@MX:SPEC: SPEC-SYN-004`.
  - SSE writer goroutine (heartbeat + main writer concurrency) →
    `@MX:WARN` with `@MX:REASON` documenting the mutex invariant
    and the goroutine-leak risk on disconnect.
  - Heartbeat interval constant / default → `@MX:NOTE` documenting
    the 15-second default rationale (proxy keepalive headroom).
  - Public `cmd/usearch-api` streaming entrypoint → `@MX:ANCHOR`
    once SPEC-IR-001 establishes the file path.
- TRUST 5 gate: 85%+ coverage for `internal/sse/`,
  `internal/streamsynth/` and the modified `cmd/usearch-api/`
  regions; `go vet` zero; `golangci-lint` zero; race-detector PASS.
- Update `CHANGELOG.md` (sync-phase responsibility).

Exit gate: `manager-quality` PASS. Sync phase ready (deferred to
`/moai sync SPEC-SYN-004`).

---

## 4. Test Plan Summary

(Detailed Given/When/Then in `acceptance.md`.)

| REQ / NFR | Test File | Test Count (planned) |
|---|---|---|
| REQ-SYN4-001a | `writer_test.go`, `synthesis_stream_test.go` | 2 (content-type + JSON-fallback negative) |
| REQ-SYN4-001b | `writer_test.go` | 3 (blank-line + multi-line data + comment grammar) |
| REQ-SYN4-001c | `streamsynth_test.go` | 2 + property test (b)(c) |
| REQ-SYN4-002 | `streamsynth_test.go`, `segmenter_test.go` | 6 |
| REQ-SYN4-003 | `heartbeat_test.go` | 4 |
| REQ-SYN4-004 | `streamsynth_test.go` | 5 (added watcher-goroutine assertion) |
| REQ-SYN4-005 | `synthesis_stream_test.go` | 5 |
| REQ-SYN4-006 | `streamsynth_test.go`, `writer_test.go` | 6 (deadline fires + cancel upstream + counter once + best-effort error + 3-goroutine cleanup + WARN log) |
| NFR-SYN4-001 | `bench_test.go` | 3 (TTFB + total overhead + heartbeat CPU) |
| NFR-SYN4-002 | `property_test.go` | 3 (wire format + lossless reconstruction + terminator uniqueness) |
| NFR-SYN4-003 | `streamsynth_test.go` | 4 (aggregate at-most-once + 3 race-window pair tests) |
| Integration | `synthesis_stream_test.go` | 4 (SSE happy + disconnect + JSON fallback + slow-client write timeout) |
| **Total new tests** | | **~47** |

Coverage target: 85% for `internal/sse/`, `internal/streamsynth/`.
Coverage of modified `cmd/usearch-api/` regions: ≥ 85% on the diff.

---

## 5. Dependencies

### 5.1 Runtime dependencies

- `internal/synthesis` (existing, SPEC-SYN-001) — read-only consumer
  of `Client.Synthesize()`. SYN-004 does NOT modify this package.
- `internal/obs` (existing, SPEC-OBS-001) — collectors + logger.
- `pkg/types` (existing, SPEC-CORE-001) — `NormalizedDoc`,
  `Query` types (read-only).
- Standard library: `net/http` (Flusher, ResponseController),
  `context`, `time`, `encoding/json`, `regexp`. No new external Go
  modules.

### 5.2 SPEC dependencies

- **depends_on**:
  - **SPEC-IR-001** (PREREQUISITE): the `cmd/usearch-api` HTTP server
    scaffolding. SPEC-SYN-004 cannot ship until SPEC-IR-001
    establishes the request handler entry point. This is documented
    in spec.md frontmatter and is a hard ordering constraint.
  - **SPEC-SYN-001**: provides `synthesis.Client.Synthesize()` and
    the `Result` shape. Contract preserved verbatim.
  - **SPEC-SYN-002**: provides the canonical sentence-segmentation
    regex AND the citation-faithfulness invariant that SYN-004's
    streaming surface must preserve. SYN-004 explicitly re-uses the
    regex from REQ-SYN2-001.
- **blocks**: none.
- **does NOT depend on**:
  - **SPEC-SYN-003** (clustering, parallel M4 work) — orthogonal;
    SYN-004's streaming surface emits whatever sentences come back
    from synthesis, regardless of whether SYN-003's clustering
    pre-filtered the input docs. The roadmap M4 3-way
    parallelization plan permits SYN-002 + SYN-003 + SYN-004 to
    develop concurrently per `.moai/project/roadmap.md:124`.

### 5.3 Module dependencies

To be added to `go.mod`: **none**. SYN-004 is implementable entirely
on the standard library. Hand-rolled SSE writer is ~140 LOC; the
`r3labs/sse/v2` library was surveyed (research §3.4) and rejected as
overkill for per-request streaming.

No Python dependencies — the Python sidecar is unchanged. No new
sidecars.

---

## 6. Risk Mitigations (linked to research §5)

| Risk ID | Mitigation in plan |
|---|---|
| R1 (slow client backpressure) | Per-write deadline via `http.ResponseController.SetWriteDeadline(SYN004_SSE_WRITE_TIMEOUT_MS)`. On write timeout: cancel parent ctx, emit best-effort `event: error`, increment `outcome="write_timeout"`, release all three goroutines. Encoded as REQ-SYN4-006 (Unwanted, P0); counter at-most-once invariant covered by NFR-SYN4-003. Tested in `test_slow_client_write_timeout_*` suite (acceptance.md Scenario H + edge case 4.5.3). |
| R2 (client disconnect mid-stream) | `r.Context().Done()` watcher goroutine; cancellation propagates to `synthesis.Client.Synthesize()` ctx within `SYN004_DISCONNECT_CANCEL_MS`. Encoded as REQ-SYN4-004 (HARD). Tested in `test_client_disconnect_*`. |
| R3 (proxy buffering) | Heartbeat goroutine emits `: ping\n\n` every 15 s (default). Document `proxy_buffering off;` in `.env.example`. Encoded as REQ-SYN4-003. |
| R4 (SYN-002 invariant violation) | Sentence-level streaming only; sentences emit AFTER citation validation. Property test NFR-SYN4-002 enforces wire-format + invariant on every generated `Result` shape. Encoded as REQ-SYN4-001c HARD constraint. |
| R5 (sidecar streaming gap) | v0 ships **buffered-then-streamed** mode; works regardless of upstream chunk granularity. Documented exclusion in spec.md §2.2; gated on follow-up SPEC. Future token-streaming upgrade replaces the buffer step without changing the wire contract. |
| R6 (concurrency: heartbeat + main writer torn writes) | `sse.Writer` uses internal mutex; both goroutines acquire the mutex before any `Write` to `http.ResponseWriter`. `go test -race ./internal/sse/...` PASS in exit gate. |

---

## 7. Open questions (deferred to Run phase or out of scope)

- Whether to support `Last-Event-ID` resume header — out of scope
  for v0; would require server-side replay buffer or cursor-based
  re-synthesis. Tracked as future SPEC candidate (SPEC-IDX-005 M6
  team-shared answer reuse touches this space).
- Whether to expose `event: progress` mid-stream (e.g. fanout/cluster
  progress before synthesis starts) — out of scope; M4 SYN-004 v0
  emits only `event: sentence` + `event: done` / `event: error`.
  Progressive pipeline-stage events are a SPEC-IR-002 (server
  observability) concern.
- Whether token-level streaming should land as SPEC-SYN-004-v2
  (in-place evolution) or SPEC-SYN-006 (new SPEC). Decision
  deferred to roadmap update post-v0 ship.
- Whether to support additional Accept content types
  (`application/x-ndjson` for line-delimited JSON streaming) — out
  of scope for v0; document as future enhancement if demand
  materializes.

---

*End of SPEC-SYN-004 plan.md.*
