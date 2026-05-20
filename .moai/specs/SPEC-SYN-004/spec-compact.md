# SPEC-SYN-004 Compact Reference

Compact (one-page) summary of `.moai/specs/SPEC-SYN-004/spec.md`.
For full context, EARS detail, and acceptance scenarios see the
companion files in this directory.

---

## What

A new HTTP streaming surface on `cmd/usearch-api` that emits
synthesis output incrementally over **Server-Sent Events**
(`text/event-stream`), one SSE event per validated sentence. Adds two
new Go packages — `internal/sse/` (SSE writer + heartbeat) and
`internal/streamsynth/` (orchestration between
`synthesis.Client.Synthesize()` and the SSE wire).

Public surfaces:

```
internal/sse.NewWriter(w http.ResponseWriter) *Writer
internal/sse.RunHeartbeat(ctx, writer, interval) error
internal/streamsynth.StreamSynthesize(ctx, w, req) (Stats, error)
```

Insertion: `cmd/usearch-api` HTTP synthesis handler (file path owned
by SPEC-IR-001's server scaffolding). Accept-header content
negotiation dispatches to either the SSE path or the existing
buffered JSON path.

---

## Why

The existing `synthesis.Client.Synthesize()` returns the full result
synchronously after the upstream LLM call completes (typically 3-8 s
per SPEC-SYN-001 NFR-SYN-001). HTTP clients today must wait for the
full RTT before any output arrives. SPEC-SYN-004 reduces *perceived*
TTFB to the duration of the first complete cited sentence (~300-800
ms typical) by streaming sentences incrementally as their citations
are validated, while preserving the SPEC-SYN-002 citation
faithfulness invariant verbatim — un-cited content NEVER reaches
the wire.

---

## How

| Stage | Mechanism |
|---|---|
| 1. Accept-header negotiation | Substring match `text/event-stream` (case-insensitive) → SSE path; otherwise → buffered JSON path (backward compatible with SYN-001 contract). |
| 2. Synthesis call | `synthesis.Client.Synthesize()` invoked synchronously (UNCHANGED single-shot in v0; sidecar token-streaming is a documented exclusion). |
| 3. Sentence segmentation | SYN-002 canonical regex `[.!?。！？]\s+\|[.!?。！？]$` (Korean + English punctuation) — SAME regex used by faithfulness gate. |
| 4. Per-sentence citation resolution | Scan `[N]` markers; look up in `Result.Citations`; build per-event `citations` array `[{marker, doc_id, url, title}]`. |
| 5. SSE event emission | One `event: sentence` per validated sentence; final `event: done` carries totals; `event: error` on failure. Each event terminated by `\n\n` (W3C SSE conformant). |
| 6. Heartbeat keepalive | Dedicated goroutine emits `: ping\n\n` SSE comment every `SYN004_SSE_HEARTBEAT_MS` (default 15000 ms) to defeat proxy buffering (nginx, ALB, Cloudflare). |
| 7. Disconnect handling | `r.Context().Done()` watcher cancels parent synth ctx within `SYN004_DISCONNECT_CANCEL_MS` (default 1000 ms) on TCP close; counter `client_disconnect` + WARN log. |

---

## Modes

| Dispatch | Trigger | Behavior | Counter emitted |
|---|---|---|---|
| **SSE path** | `Accept: text/event-stream` (substring match) | Sentence-by-sentence emission with heartbeat keepalive; preserves SYN-002 invariant. | `streamed_complete` (success) / `client_disconnect` / `write_timeout` / `error_upstream` |
| **JSON fallback** | Any other Accept value (or absent) | Existing SYN-001 buffered JSON `Result` response, byte-equivalent to non-streaming call. Zero SSE overhead. | `accept_fallback_to_json` |

---

## EARS REQ index (5 patterns covered; 8 REQs after iter 1 audit fixes)

| ID | Pattern | Summary |
|---|---|---|
| REQ-SYN4-001a | Ubiquitous | Every SSE response includes `Content-Type: text/event-stream` + `Cache-Control: no-cache` + `Connection: keep-alive` headers. |
| REQ-SYN4-001b | Ubiquitous | Every SSE response conforms to W3C SSE wire format (`\n\n` terminator, multi-line `data:`, `: <text>\n\n` comments). |
| REQ-SYN4-001c | Ubiquitous | **[HARD]** Un-cited content NEVER reaches the wire (SPEC-SYN-002 citation faithfulness invariant preserved verbatim). |
| REQ-SYN4-002 | Event-Driven | WHEN synthesis returns successfully → segment by SYN-002 regex, resolve citations, emit one `event: sentence` per validated sentence + final `event: done`. |
| REQ-SYN4-003 | State-Driven | WHILE stream open AND heartbeat enabled → emit `: ping\n\n` every `SYN004_SSE_HEARTBEAT_MS`. |
| REQ-SYN4-004 | Unwanted | IF client disconnects mid-stream → cancel upstream synth ctx within `SYN004_DISCONNECT_CANCEL_MS`; emit `client_disconnect` counter + WARN log; release all three goroutines (main writer, heartbeat, watcher exits within 100 ms). |
| REQ-SYN4-005 | Optional | WHERE Accept header lacks `text/event-stream` → fall back to existing buffered JSON response (backward compatible). |
| REQ-SYN4-006 | Unwanted | IF a single SSE write blocks longer than `SYN004_SSE_WRITE_TIMEOUT_MS` (default 5000 ms) → cancel upstream ctx; emit `write_timeout` counter; best-effort `event: error`; WARN log; release all three goroutines. |

NFRs:

- **NFR-SYN4-001** Latency: TTFB to first sentence ≤ synth_latency
  + 50 ms (buffered mode v0); total stream overhead ≤ 100 ms p95;
  heartbeat CPU < 1% at default interval.
- **NFR-SYN4-002** Property: SSE wire format conformance + lossless
  text reconstruction across `event: sentence` joining + exactly one
  terminator (`event: done` OR `event: error`).
- **NFR-SYN4-003** Invariant: **outcome counter exactly-once per
  request** — `usearch_syn004_outcomes_total` increments by exactly 1
  across all five `outcome` label values per request lifecycle.
  Binds REQ-002/004/005/006 into a single mutually-exclusive
  guarantee via sync.Once-style guard.

---

## What NOT to build (top exclusions)

- **NOT modifying synthesis logic** — `internal/synthesis/` Go side
  AND `services/researcher/` Python side are consumed read-only.
  SYN-001 / SYN-002 contracts preserved verbatim.
- **NOT WebSocket transport** — SSE only; no bidirectional protocol;
  no gRPC streaming.
- **NOT bidirectional streaming** — server → client only.
- **NOT changing citation format** — SYN-002 `[N]` marker convention
  + `Citation` shape preserved.
- **NOT token-level streaming from sidecar** — research §6 R5 gap;
  buffered-then-streamed v0 ships first; sidecar streaming upgrade
  gated on follow-up SPEC.
- **NOT cross-call event resume** (no `Last-Event-ID`) — fresh stream
  per request; resume is SPEC-IDX-005 (M6) territory.
- **NOT character/token-level event granularity** — sentence is the
  atomic unit; sub-sentence events would require a different
  citation-validation strategy (research §4.3).
- **NOT custom Accept types beyond `text/event-stream`** — no
  `application/x-ndjson`, no gRPC-web.
- **NOT modifying `cmd/usearch` CLI** — CLI streaming UX is
  SPEC-CLI-002 (M7).

---

## File impact (delta markers)

- `[NEW]` `internal/sse/` (5 files: types/writer/heartbeat + 2 tests, ~510 LOC total)
- `[NEW]` `internal/streamsynth/` (10 files: types/options/streamsynth/segmenter/citations/observability + 4 tests, ~1310 LOC)
- `[NEW]` `internal/obs/metrics/streamsynth.go` (~60 LOC)
- `[NEW]` `cmd/usearch-api/handlers/synthesis_stream_test.go` (path per SPEC-IR-001 layout, ~220 LOC)
- `[MODIFY]` `cmd/usearch-api/handlers/synthesis.go` (Accept-header dispatch, ~60 LOC inserted)
- `[MODIFY]` `cmd/usearch-api/main.go` (router wire-up, ~15 LOC)
- `[MODIFY]` `internal/obs/metrics/metrics.go` (collector registration + cardinality allowlist amendment)
- `[MODIFY]` `internal/obs/obs.go` (re-export collector handles)
- `[MODIFY]` `.env.example` (5 new env vars + nginx config recommendation)
- `[EXISTING]` `internal/synthesis/` UNCHANGED (consumed read-only)
- `[EXISTING]` `internal/synthesis/types.go` UNCHANGED (`Result`, `Citation` shapes)
- `[EXISTING]` `services/researcher/` UNCHANGED (sidecar streaming is a documented exclusion gated on follow-up SPEC; current single-shot httpx pattern at `gateway.py:36-72` is the v0 floor)
- `[EXISTING]` `pkg/types/normalized_doc.go` UNCHANGED

---

## Quality gate

- 85% coverage on `internal/sse/`, `internal/streamsynth/`
- `go vet` clean, `golangci-lint` clean, `go test -race` PASS
- Goroutine-leak detector (`go.uber.org/goleak` or equivalent) PASS
- Property test (NFR-SYN4-002) ≥ 1000 generated cases PASS
- Benchmarks meet NFR-SYN4-001 thresholds (TTFB, total overhead,
  heartbeat CPU)
- Pre-submission self-review documented in `progress.md`
- @MX tags: 1 ANCHOR on `StreamSynthesize` (REASON: public boundary,
  fan_in expected ≥ 2 from HTTP handler + tests, ≥ 3 with future
  MCP/CLI variants), ≥ 1 NOTE (heartbeat interval default
  rationale), 1 WARN on SSE writer goroutine concurrency (REASON:
  heartbeat + main writer mutex invariant + leak risk on disconnect)

---

## Cross-SPEC contracts preserved

- **SPEC-IR-001** (PREREQUISITE): `cmd/usearch-api` HTTP server
  scaffolding. SYN-004 cannot ship until SPEC-IR-001 establishes the
  request handler entry point. `cmd/usearch-api/main.go:1-40` is a
  stub today.
- **SPEC-SYN-001** REQ-SYN-001 through REQ-SYN-007 + NFR-SYN-001
  through NFR-SYN-004: contract preserved verbatim. SYN-004 consumes
  `synthesis.Client.Synthesize()` read-only.
- **SPEC-SYN-002** REQ-SYN2-001 through REQ-SYN2-005: citation
  faithfulness contract preserved. SYN-004 re-uses the canonical
  sentence-segmentation regex literally; NEVER emits un-cited content
  to the wire.
- **SPEC-SYN-003** (parallel M4 work, if landed): orthogonal — SYN-004
  emits whatever sentences come back from synthesis regardless of
  whether SYN-003's clustering pre-filtered the input docs. No
  ordering dependency.

---

*End of SPEC-SYN-004 spec-compact.md.*
