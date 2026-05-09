# SPEC-SYN-004 Acceptance Criteria

Companion to `.moai/specs/SPEC-SYN-004/spec.md`.
Format: Given / When / Then scenarios + edge-case enumeration + quality
gate criteria.

---

## 1. Scope

This document specifies the testable acceptance criteria for
SPEC-SYN-004 v0.1. Each scenario maps to one or more EARS REQs / NFRs
in `spec.md` §3. Together they constitute the Definition of Done.

---

## 2. Definition of Done (gate summary)

A scenario is "DONE" when ALL of the following hold:

1. The scenario's listed test(s) PASS in the project's standard
   `go test ./...` invocation.
2. The scenario's REQ/NFR ID is referenced in the test name comment
   block (`// REQ-SYN4-NNN: <one-line>`).
3. Coverage for the affected files in `internal/sse/` and
   `internal/streamsynth/` is ≥ 85%.
4. `go vet ./...` is clean.
5. `golangci-lint run` is clean.
6. `go test -race ./internal/sse/... ./internal/streamsynth/...`
   PASS.
7. SPEC-SYN-001 / SPEC-SYN-002 acceptance suites remain GREEN after
   SPEC-SYN-004 implementation (no regression on the buffered JSON
   path).
8. Pre-submission self-review per project workflow-modes rule:
   reviewed full diff, no simpler approach found.

---

## 3. Given/When/Then Scenarios (minimum 3, expanded)

### 3.1 Scenario A — SSE happy path (5-sentence English synthesis)

Maps to: REQ-SYN4-001a (Ubiquitous content-type headers),
REQ-SYN4-001b (Ubiquitous W3C wire format), REQ-SYN4-001c
(Ubiquitous un-cited content invariant), REQ-SYN4-002 (Event-Driven
per-sentence emission), NFR-SYN4-001 (latency).

> **Given** an HTTP request to the `cmd/usearch-api` synthesis
> endpoint with header `Accept: text/event-stream` and a query that
> produces 5 input docs after fanout, AND a mocked
> `synthesis.Client.Synthesize()` returning a `Result` with
> `Text = "Event X happened in Seoul [1]. The incident was confirmed
> by police [2]. Three people were injured [3]. Recovery operations
> are underway [4]. Officials warned of further risks [5]."` and
> `Citations = [{1, "doc-1", "https://reuters.com/...", "Reuters: ..."},
> {2, "doc-2", "https://ap.example/...", "AP: ..."},
> {3, "doc-3", "https://yna.kr/...", "Yonhap: ..."},
> {4, "doc-4", "https://news.example/...", "Local: ..."},
> {5, "doc-5", "https://gov.example/...", "Gov: ..."}]`,
>
> **When** the handler dispatches to
> `streamsynth.StreamSynthesize(ctx, sseWriter, req)`,
>
> **Then** the response headers include `Content-Type:
> text/event-stream`, `Cache-Control: no-cache`,
> `Connection: keep-alive`,
>
> **And** the response body emits exactly 5 `event: sentence` events
> in the order of `Result.Text` segmentation, each with
> `data: {"request_id":"...","sentence_index":N,
> "text":"<sentence>","citations":[{"marker":N,"doc_id":"doc-N",
> "url":"...","title":"..."}],"schema_version":1}`,
>
> **And** the stream terminates with exactly one `event: done`
> carrying `data: {"request_id":"...","total_sentences":5,
> "latency_ms":<positive>,"model":"...","provider":"...",
> "cost_usd":<float>,"schema_version":1}`,
>
> **And** every event payload terminates with `\n\n` (W3C SSE
> conformant),
>
> **And** `usearch_syn004_outcomes_total{outcome="streamed_complete"}
> += 1` (exactly once),
>
> **And** `usearch_syn004_sentences_emitted` histogram observes the
> value `5`.

### 3.2 Scenario B — Korean synthesis (mixed punctuation segmentation)

Maps to: REQ-SYN4-002 (Korean punctuation in canonical regex),
REQ-SYN4-001c (citation invariant under Korean text).

> **Given** a `Result` with
> `Text = "오늘 서울에서 사건이 발생했다 [1]. 경찰은 조사 중이다 [2]. 부상자 3명이 보고되었다 [3]."`
> and 3 corresponding citations,
>
> **When** `streamsynth.StreamSynthesize` runs with
> `Accept: text/event-stream`,
>
> **Then** the segmenter (using SYN-002 canonical regex
> `[.!?。！？]\s+|[.!?。！？]$`) produces exactly 3 sentences,
>
> **And** 3 `event: sentence` events emit in order with their
> respective citations attached,
>
> **And** the stream terminates with `event: done` reporting
> `total_sentences == 3`,
>
> **And** the lossless reconstruction property holds: joining the 3
> emitted `text` fields with a single space matches `Result.Text`
> modulo whitespace normalization.

### 3.3 Scenario C — Heartbeat keepalive during slow synthesis

Maps to: REQ-SYN4-003 (State-Driven heartbeat).

> **Given** a synthesis call mocked to delay 350 ms before returning,
> AND `SYN004_SSE_HEARTBEAT_ENABLED=true` AND
> `SYN004_SSE_HEARTBEAT_MS=100` (test-only short interval),
>
> **When** `streamsynth.StreamSynthesize` runs with
> `Accept: text/event-stream`,
>
> **Then** the response body contains at least 3 `: ping\n\n` SSE
> comment lines emitted at ~100 ms intervals before the first
> `event: sentence`,
>
> **And** the comments do NOT interleave inside any `event:` /
> `data:` line (mutex enforcement; race detector PASS),
>
> **And** after the synthesis completes, the heartbeat goroutine
> terminates within 100 ms of stream end (no leaked goroutine).

### 3.4 Scenario D — Client disconnect mid-stream cancels upstream

Maps to: REQ-SYN4-004 (Unwanted disconnect handling).

> **Given** a synthesis call mocked to take 5 seconds to complete,
> AND `SYN004_DISCONNECT_CANCEL_MS=500` (test-only short deadline),
> AND a client that closes its TCP connection 200 ms after
> initiating the request,
>
> **When** `streamsynth.StreamSynthesize` is mid-stream (heartbeats
> emitting; synthesis call in flight),
>
> **Then** the request context's `Done()` channel signals within
> ~200 ms of client close,
>
> **And** the parent ctx passed to `synthesis.Client.Synthesize()` is
> cancelled within `SYN004_DISCONNECT_CANCEL_MS + 100 ms` jitter
> (assertion: cancellation observed at most 600 ms after client
> close),
>
> **And** `usearch_syn004_outcomes_total{outcome="client_disconnect"}
> += 1`,
>
> **And** exactly ONE WARN-level log record is emitted with
> `{request_id, reason:"client_disconnect",
> sentences_emitted_before_disconnect:<N>}`,
>
> **And** the goroutine-leak detector reports zero leaked goroutines
> 500 ms after disconnect cleanup — verifying that all three
> goroutines (main writer, heartbeat, AND the disconnect-watcher
> goroutine itself) have terminated within 100 ms of stream close,
>
> **And** the upstream LLM call is not allowed to complete its
> remaining ~4.8 s of work (cost not billed; resources released).

### 3.5 Scenario E — Accept-header fallback to JSON

Maps to: REQ-SYN4-005 (Optional Accept-header content negotiation).

> **Given** an HTTP request to the synthesis endpoint with NO
> `Accept` header (or `Accept: application/json`),
>
> **When** the handler processes the request,
>
> **Then** the dispatch branch selects the existing buffered JSON
> path,
>
> **And** the SSE writer is NOT instantiated (mock constructor
> records 0 invocations),
>
> **And** the heartbeat goroutine is NOT started (mock records 0
> invocations),
>
> **And** the response is `Content-Type: application/json` with body
> matching the SPEC-SYN-001 `Result` JSON contract byte-equivalent
> to a non-streaming call,
>
> **And** `usearch_syn004_outcomes_total{outcome=
> "accept_fallback_to_json"} += 1`,
>
> **And** SPEC-SYN-001 / SPEC-SYN-002 existing acceptance tests for
> the buffered JSON path remain GREEN — no regression.

### 3.6 Scenario F — Proxy-buffering defeat assertion (heartbeat verification)

Maps to: REQ-SYN4-003 (heartbeat enables proxy flush), R3 mitigation.

> **Given** a simulated nginx-like proxy in the test harness that
> buffers HTTP responses by default but flushes on idle ≥ 50 ms,
> AND a synthesis call delaying 200 ms before completing, AND
> `SYN004_SSE_HEARTBEAT_MS=30`,
>
> **When** the request flows through the simulated proxy,
>
> **Then** the client observes the first byte (a `: ping\n\n`
> heartbeat) within ~30 ms (the heartbeat interval), NOT after the
> 200 ms synthesis completion,
>
> **And** at least 5 heartbeat comments arrive at the client during
> the 200 ms synthesis delay,
>
> **And** the proxy never holds the response in its buffer for >
> heartbeat-interval + jitter (proves heartbeat defeats buffering).

### 3.7 Scenario G — Empty synthesis result (defensive)

Maps to: REQ-SYN4-001a (defensive boundary — headers still emitted),
REQ-SYN4-002 (zero-sentence edge case).

> **Given** a `Result` with `Text = ""` (degenerate; production code
> path should not produce this because SYN-001 REQ-SYN-004 returns
> 400 on empty inputs, but defensive),
>
> **When** `streamsynth.StreamSynthesize` runs with
> `Accept: text/event-stream`,
>
> **Then** zero `event: sentence` events emit,
>
> **And** the stream terminates with `event: done` carrying
> `total_sentences: 0`,
>
> **And** `usearch_syn004_sentences_emitted` histogram observes the
> value `0`.

### 3.8 Scenario H — Slow client write timeout (backpressure mitigation)

Maps to: REQ-SYN4-006 (Unwanted slow-client write timeout),
NFR-SYN4-003 (outcome counter exactly-once), R1 mitigation.

> **Given** a synthesis call mocked to return a 5-sentence `Result`
> after 50 ms (so synthesis itself is fast), AND a mock client that
> performs the TCP handshake and accepts the response headers + the
> first `event: sentence` but then **stalls** — does NOT drain its
> TCP receive buffer (simulating a hung browser, network congestion,
> or a malicious slowloris-style client), AND
> `SYN004_SSE_WRITE_TIMEOUT_MS=200` (test-only short deadline),
>
> **When** `streamsynth.StreamSynthesize` attempts to emit the
> second `event: sentence` and the underlying TCP `Write` blocks
> beyond 200 ms because the receive buffer is full,
>
> **Then** the per-write deadline (set via
> `http.ResponseController.SetWriteDeadline`) fires within
> `SYN004_SSE_WRITE_TIMEOUT_MS + 50 ms` jitter (assertion: write
> error observed at most 250 ms after the stall begins),
>
> **And** the `Write` returns `os.ErrDeadlineExceeded` (or a
> wrapping error matching `errors.Is(err,
> os.ErrDeadlineExceeded)`),
>
> **And** the parent ctx passed to `synthesis.Client.Synthesize()` is
> cancelled (verified via `ctx.Err() == context.Canceled`),
>
> **And** `usearch_syn004_outcomes_total{outcome="write_timeout"}
> += 1` (exactly once per request — NFR-SYN4-003 invariant; no
> double-increment with `streamed_complete` or `client_disconnect`),
>
> **And** the server attempts (best-effort) to emit one final
> `event: error` payload with
> `data: {"request_id":"...","error_code":"write_timeout",
> "error_message":"<brief>","partial_sentences_emitted":1,
> "schema_version":1}`; if this second write also fails the failure
> is silently dropped (no panic, no double-counter, no second
> log record),
>
> **And** exactly ONE WARN-level log record is emitted with
> attributes `{request_id, reason:"write_timeout",
> sentences_emitted_before_timeout:1}`,
>
> **And** all three goroutines terminate cleanly within 100 ms of
> the cancellation: main writer (returns on the timeout error),
> heartbeat (exits on ctx.Done()), AND the disconnect-watcher
> goroutine itself,
>
> **And** the goroutine-leak detector reports zero leaked
> goroutines 200 ms after teardown.

---

## 4. Edge Cases

### 4.1 Citation invariant edge cases (preserves SYN-002)

1. **Mixed cited / un-cited sentences in `Result.Text`** (production
   code path: SYN-002 strip mode pre-filters this; defensive
   test-only state): synthetically inject an un-cited sentence into
   `Result.Text`; assert it is silently dropped from the stream
   (strip-mode behavior matching SYN-002 default). The number of
   emitted `event: sentence` events MUST match the count of cited
   sentences only.
2. **Multi-marker sentence** (e.g. `"X happened [1][2]."`): one
   `event: sentence` event with `citations` array containing both
   marker entries.
3. **Out-of-range marker in raw text** (e.g. `"X happened [99]."`
   when only 3 docs exist): production code path — SYN-002
   `_process_markers()` strips the `[99]` before SYN-004 sees the
   text. Test asserts that `Result.Citations` contains only valid
   markers and that no `event: sentence` references an
   out-of-range `marker`.

### 4.2 Sentence segmentation edge cases

1. **Single sentence (no terminator at end)**: `Text = "Hello [1]"`
   (no period); regex matches `[.!?。！？]$` only at terminal
   position. With no terminal punctuation, segmenter MUST treat the
   entire string as one sentence (defensive; production output from
   SYN-001 always has terminal punctuation). Test asserts: 1
   `event: sentence` emitted.
2. **Single sentence with terminator**: `Text = "Hello [1]."`; 1
   `event: sentence` emitted.
3. **Repeated whitespace between sentences**: regex's `\s+` matches
   any whitespace run; segmentation is robust to multi-space /
   newline separators.
4. **Korean punctuation only**: `Text = "오늘이다 [1]。내일이다 [2]。"`;
   2 events emitted using fullwidth period.
5. **Mixed Korean + English punctuation in same paragraph**: handled
   by the regex alternation; segmentation produces correct sentence
   count.
6. **Abbreviation-heavy text (false-positive sentence boundary)**:
   `Text = "Dr. Smith confirmed [1]. Mr. Park noted [2]."` — the
   SYN-002 canonical regex `[.!?。！？]\s+|[.!?。！？]$` will
   over-segment on `Dr.` and `Mr.` abbreviations, producing
   false-positive sentence boundaries (4 segments instead of the
   intended 2). **SPEC-SYN-004 inherits this segmentation behavior
   verbatim from SPEC-SYN-002** (research.md §4.2; SYN-002
   REQ-SYN2-001 owns the canonical regex). Abbreviation handling
   (e.g. tokenizer-aware segmentation, abbreviation dictionary, or
   ML-based sentence boundary detection) is **explicitly
   out-of-scope for SYN-004** and is deferred to either a future
   SYN-002 upgrade or a dedicated SPEC (e.g. SPEC-EVAL-003 with a
   documented known false-positive rate). The acceptance test for
   this case is a **regression check, not a correctness check**:
   `test_segmenter_abbreviation_inherits_syn002_behavior` asserts
   that SPEC-SYN-004's segmenter output for abbreviation-rich input
   matches SPEC-SYN-002's reference output byte-for-byte. A
   change in SYN-004 segmentation behavior relative to SYN-002 is
   the failure mode (drift detection), not the false-positive
   itself.

### 4.3 SSE wire-format edge cases

1. **Newlines inside `data:` payload** (JSON with embedded `\n`): the
   SSE spec requires multi-line `data:` to repeat the `data:` prefix.
   The writer MUST split JSON on `\n` and emit one `data:` line per
   line; concatenation rule MUST be the SSE-canonical
   `lines.join("\n")` on the receiver. Test asserts a JSON payload
   with `\n` inside survives round-trip via a real EventSource
   parser harness.
2. **Empty event type**: writer rejects via error sentinel
   `ErrEmptyEventType`; never emits a malformed event.
3. **Empty data**: writer emits `event: <type>\ndata: \n\n` (allowed
   by spec; client receives empty `data` field). Defensive — not
   used by SYN-004 production path.

### 4.4 Heartbeat edge cases

1. **Heartbeat fires during sentence emission**: mutex serializes;
   heartbeat's `: ping\n\n` does NOT split a sentence event. Race
   detector PASS asserts no torn writes.
2. **`SYN004_SSE_HEARTBEAT_MS=0`**: invalid; `OptionsFromEnv()`
   rejects with error. Acceptance test asserts validation error.
3. **`SYN004_SSE_HEARTBEAT_MS > 60000`**: invalid (above range
   ceiling); rejected.
4. **Heartbeat interval longer than synth completion**: synth
   completes in 200 ms with heartbeat at 5000 ms → zero heartbeat
   comments emitted; not an error.

### 4.5 Disconnect edge cases

1. **Disconnect before first sentence emitted**:
   `sentences_emitted_before_disconnect == 0`; no
   `event: sentence` events; counter
   `outcome="client_disconnect"` += 1.
2. **Disconnect after `event: done` already written**: race window
   — disconnect detected after stream-end; cleanup is a no-op;
   counter `outcome="streamed_complete"` already fired; no
   `client_disconnect` increment. Test asserts at-most-once outcome
   counter per call — this is a **NFR-SYN4-003 invariant** binding
   REQ-SYN4-002 / REQ-SYN4-004 / REQ-SYN4-005 / REQ-SYN4-006 into a
   single mutually-exclusive guarantee enforced via a sync.Once-style
   guard. See `test_outcome_counter_race_streamed_complete_vs_disconnect`.
3. **Slow client (write blocks)**: promoted to first-class Scenario H
   (§3.8). Per-write deadline `SYN004_SSE_WRITE_TIMEOUT_MS` fires;
   counter `outcome="write_timeout"` += 1 (NFR-SYN4-003 enforces
   at-most-once); ctx cancellation propagates same as disconnect
   path; encoded as REQ-SYN4-006 (Unwanted, P0).

### 4.6 Accept-header edge cases

1. **`Accept: */*`**: ambiguous; SYN-004 fallback rule treats
   missing-explicit-`text/event-stream` as JSON path. Test asserts
   `*/*` triggers fallback.
2. **`Accept: text/event-stream, application/json`** (multi-value):
   substring match for `text/event-stream` succeeds; SSE path.
3. **`Accept: TEXT/EVENT-STREAM`** (uppercase): case-insensitive
   substring match; SSE path.
4. **`Accept` header present but empty string**: fallback to JSON.

---

## 5. NFR-SYN4-001 Latency Test Method

| Test | Configuration | Iterations | Target |
|---|---|---|---|
| `BenchmarkSSE_TTFB_FirstSentence` | mocked synth latency 500 ms (constant); buffered-streamed mode | 100 | TTFB to first `event: sentence` byte ≤ 550 ms p95 (synth_latency + 50 ms overhead) |
| `BenchmarkSSE_TotalOverhead_vs_JSON` | identical 5-sentence response; SSE path vs JSON path | 100 | total wall-clock SSE - JSON ≤ 100 ms p95 |
| `BenchmarkSSE_HeartbeatCPU_60sIdle` | 60-second idle stream, heartbeat at 15000 ms | 1 long-running | heartbeat goroutine CPU < 1% (process-level sampling via `runtime.ReadMemStats` + `time.Now()` deltas) |

Reporting: `go test -bench=. -benchmem ./internal/streamsynth/...`
output captured in `progress.md` post-Run-phase.

---

## 6. Quality Gate Criteria

A SPEC-SYN-004 implementation passes the quality gate when:

| Gate | Tool | Threshold |
|---|---|---|
| Unit tests | `go test ./internal/sse/... ./internal/streamsynth/...` | 100% pass |
| Integration tests | `go test ./cmd/usearch-api/...` | 100% pass; SPEC-SYN-001 / SPEC-SYN-002 acceptance tests remain GREEN |
| Coverage | `go test -coverprofile=cover.out` | ≥ 85% line coverage on `internal/sse/`, `internal/streamsynth/` |
| Race | `go test -race` | PASS (heartbeat + main writer concurrency) |
| Vet | `go vet ./...` | 0 findings |
| Lint | `golangci-lint run` | 0 findings |
| Property | `testing/quick` over generated `Result` shapes | ≥ 1000 generated cases PASS |
| Benchmarks | `go test -bench=.` | NFR-SYN4-001 thresholds met |
| Goroutine leak | `go.uber.org/goleak` (or equivalent) | 0 leaked goroutines after each test cleanup |
| Self-review | Pre-submission review per workflow-modes rule | One pass; documented in `progress.md` |
| @MX tags | manual placement | 1 ANCHOR (`StreamSynthesize` + future API entrypoint) + ≥ 1 NOTE (heartbeat interval default) + 1 WARN (SSE writer goroutine concurrency, with REASON) |

---

## 7. Definition of Done — checklist

- [ ] All 8 EARS REQs (SYN4-001a, 001b, 001c, 002, 003, 004, 005,
      006) have GREEN tests with `// REQ-SYN4-NNN:` comment headers.
- [ ] All 3 NFRs (SYN4-001 perf, SYN4-002 property, SYN4-003 counter
      exactly-once invariant) have GREEN benchmarks / property tests
      / race-window tests.
- [ ] `internal/sse/` package coverage ≥ 85%.
- [ ] `internal/streamsynth/` package coverage ≥ 85%.
- [ ] `cmd/usearch-api/` integration covered for all four dispatch
      modes (SSE happy, disconnect, JSON fallback, slow-client write
      timeout).
- [ ] SPEC-SYN-001 / SPEC-SYN-002 acceptance suites remain GREEN —
      no regression on the buffered JSON path.
- [ ] `go vet` clean. `golangci-lint` clean. `go test -race` PASS.
      Goroutine-leak detector PASS.
- [ ] Pre-submission self-review documented in `progress.md`.
- [ ] @MX tags applied per `plan.md` §3 Milestone P0-E.
- [ ] `.env.example` updated with the five `SYN004_*` env vars +
      nginx `proxy_buffering off;` recommendation.
- [ ] `internal/obs/metrics/` cardinality allowlist amended to
      declare the new `outcome` label values.
- [ ] No modification to `internal/synthesis/` Go side or
      `services/researcher/` Python side (consumed read-only).
- [ ] No modification to `pkg/types/normalized_doc.go`.
- [ ] SSE event payload `schema_version: 1` round-trips through
      JSON and is consumable by future SPEC variants.
- [ ] Verified browser EventSource reference client (or equivalent
      Go SSE client harness) consumes the stream without parse
      errors across all 8 G/W/T scenarios (A through H).
- [ ] NFR-SYN4-003 race-window tests pass under `go test -race`
      (3 pair tests: streamed_complete-vs-disconnect,
      disconnect-vs-write_timeout,
      streamed_complete-vs-write_timeout).
- [ ] Segmentation abbreviation regression test
      (`test_segmenter_abbreviation_inherits_syn002_behavior`)
      asserts byte-for-byte parity with SPEC-SYN-002 reference
      output (drift detection only; correctness deferred).

---

*End of SPEC-SYN-004 acceptance.md.*
