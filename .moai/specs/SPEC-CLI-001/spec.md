---
id: SPEC-CLI-001
title: usearch query Subcommand v0
version: 0.1.0
milestone: M2 — First end-to-end slice
status: implemented
priority: P0
owner: expert-backend
methodology: tdd
coverage_target: 80
created: 2026-04-28
updated: 2026-05-04
implemented_at: 2026-05-04
author: limbowl
issue_number: null
depends_on: [SPEC-IR-001, SPEC-ADP-001, SPEC-ADP-002, SPEC-SYN-001]
blocks: [SPEC-CLI-002, SPEC-MCP-001]
---

# SPEC-CLI-001: `usearch query` Subcommand v0

## HISTORY

- 2026-04-28 (initial draft v0.1, limbowl via manager-spec):
  First user-facing CLI subcommand. Wires SPEC-IR-001 Intent Router + SPEC-ADP-001 Reddit adapter + SPEC-ADP-002 HN adapter (forward-looking, both still draft) + SPEC-SYN-001 synthesis client (forward-looking) into an end-to-end query response. Replaces the placeholder `"usearch: no command given"` stderr message at `cmd/usearch/main.go:73-74` with a real subcommand dispatcher. Decision-locked for v0: stdlib `flag` package only (no cobra/urfave/kong), per research.md §2.5; basic CLI-internal fanout (SPEC-FAN-001 owns full fanout in M3); soft-fail to exit code 3 when synthesis is unavailable; stdout for answer payload, stderr for progress/logs; `--format text` (default) and `--format json` only. 11 EARS REQs (8 × P0 + 3 × P1) + 4 NFRs covering subcommand parsing, source/format/timeout flags, exit codes, stdout/stderr separation, observability wiring, and binary size discipline. Research artifact at `.moai/specs/SPEC-CLI-001/research.md` captures the CLI library survey, integration surfaces, and 6 open questions. Ready for plan-auditor review and annotation cycle.

- 2026-05-04 (implemented v0.1, manager-tdd via TDD RED-GREEN-REFACTOR):
  All 11 EARS REQs + 4 NFRs implemented. Forward dependencies SPEC-ADP-002 (HN adapter) and SPEC-SYN-001 (synthesis client) confirmed implemented before this run phase. Key reconciliation: synthesis client wired via `synthClientIface` interface with `nopSynthClient` as the degradation path (REQ-CLI-009) — real synthesis.Client deferred to a future SPEC when production credentials are configured. Key files delivered: `cmd/usearch/query.go` (Execute, runFanout, parseQueryFlags), `cmd/usearch/exitcode.go` (exit code constants), `cmd/usearch/progress.go` (progressEmitter interface), `cmd/usearch/output_text.go`, `cmd/usearch/output_json.go`, `cmd/usearch/query_response.go`, `cmd/usearch/coverage_supplement_test.go`. Coverage: 80.1% (target: 80%). MX tags applied: `@MX:ANCHOR` on Execute(), `@MX:WARN` on runFanout(), `@MX:NOTE` on progressEmitter interface.

---

## 1. Purpose

SPEC-BOOT-001 delivered the `usearch` binary as a `--version`-only stub (`cmd/usearch/main.go:1-83`). SPEC-OBS-001 wired observability. SPEC-LLM-001 added the LLM client. SPEC-IR-001 (implemented) provides `router.Router.Classify` returning a `RoutingDecision` with adapter set + lang + category. SPEC-ADP-001 (implemented) provides the first concrete adapter. SPEC-ADP-002 and SPEC-SYN-001 are draft and will be implemented in parallel with this SPEC's run phase.

SPEC-CLI-001 fills the subcommand layer in `cmd/usearch/`: it adds a `query` subcommand that takes a positional prompt argument, classifies it via the router, dispatches to the adapter set, calls the synthesis client, and prints a synthesized paragraph plus numbered citations to stdout. It is the FIRST USER-FACING SURFACE of the entire system — every M2-and-later feature flows through it.

The subcommand is the gating artifact for the M2 exit criterion at `.moai/project/roadmap.md:149`:

> M2 | `usearch query "hello world"` returns Reddit + HN results with one synthesized paragraph + citations.

Once SPEC-CLI-001 lands alongside SPEC-ADP-002 and SPEC-SYN-001, M2 is complete and M3 (full fanout, twelve adapters, hybrid index) can begin.

The subcommand is INTENTIONALLY MINIMAL:

- One subcommand only: `query`. (`usearch deep`, `usearch team list`, TUI deferred to SPEC-CLI-002 in M7.)
- Synchronous request/response. (Streaming token output deferred to SPEC-SYN-004 in M4.)
- Basic CLI-internal fanout. (Full goroutine pool / per-adapter timeout / dedup / retry deferred to SPEC-FAN-001 in M3.)
- stdlib `flag` only. (Cobra migration deferred to SPEC-CLI-002.)

This minimum is the wedge that proves the end-to-end slice works. SPEC-CLI-002 grows the subcommand tree once the slice is verified.

---

## 2. Scope

### 2.1 In-Scope

| Ref | Item |
|-----|------|
| a | `cmd/usearch/main.go` refactor: insert subcommand dispatcher BEFORE `flag.Parse()`. Preserves existing `--version` / `-v` / `--no-llm` semantics |
| b | `cmd/usearch/query.go`: NEW file owning the `query` subcommand. Exports `Execute(ctx context.Context, args []string, stdout, stderr io.Writer) int` for in-process testing |
| c | `cmd/usearch/query.go::queryFlags` struct: holds `Source []string`, `Format string`, `Timeout time.Duration` parsed from `*flag.FlagSet` |
| d | `cmd/usearch/query.go::runQuery`: orchestration pipeline — parse → classify → fanout → synthesise → format |
| e | `cmd/usearch/query.go::runFanout(ctx, decision, registry) ([]types.NormalizedDoc, []error)`: basic CLI-internal fanout; `errgroup.WithContext`, one goroutine per `decision.AdapterSet` member, results merged in adapter-name order. Replaced by `fanout.Dispatch` when SPEC-FAN-001 lands |
| f | `cmd/usearch/output_text.go`: NEW file. `formatText(w io.Writer, resp *queryResponse)` writes the human-readable answer + citation list to stdout |
| g | `cmd/usearch/output_json.go`: NEW file. `formatJSON(w io.Writer, resp *queryResponse)` writes the schema-versioned JSON object |
| h | `cmd/usearch/progress.go`: NEW file. `progressEmitter` interface + two implementations (`humanProgress` writes to stderr in text format; `jsonProgress` no-ops because `obs.Logger` already emits structured slog JSON to stderr) |
| i | `cmd/usearch/exitcode.go`: NEW file. Exit-code constants + `classifyError(err) int` mapper |
| j | `cmd/usearch/query_test.go`: NEW. Table-driven `TestExecute` covering success / partial / timeout / no-adapters / empty-query / invalid-format / invalid-source. Black-box `os/exec` test for `--help` is added to `main_test.go` |
| k | `cmd/usearch/main_test.go`: extend with `TestQuerySubcommandHelp` and `TestUnknownSubcommandExits2` (additive; existing tests unchanged) |
| l | `internal/synthesis/`: NEW package skeleton (`client.go` interface + `nopclient.go` no-op fallback). The Go-side HTTP client implementation is owned by SPEC-SYN-001; this SPEC adds ONLY the interface declaration so CLI compiles without SYN-001 |
| m | `cmd/usearch/main.go` adapter-registry wiring: construct `*adapters.Registry`, register Reddit (always when ADP-001 is on the binary), register HN (when ADP-002 is on the binary). Registration is conditional on adapter package presence — guarded by build tags or by checking for env-keyed credentials per Capabilities.AuthEnvVars |

### 2.2 Out-of-Scope (Exclusions — What NOT to Build)

[HARD] This SPEC explicitly excludes the following items. Each has a known destination SPEC; this list prevents scope creep into CLI-001.

- **`usearch deep` subcommand** (multi-agent, long-form report). → SPEC-CLI-002 (M7) and SPEC-DEEP-* (M5).
- **`usearch team list` / `usearch team members` / `usearch team add`** subcommands. → SPEC-CLI-002 (M7) gated on M6 team plane.
- **TUI / bubbletea interactive mode**. → SPEC-CLI-002 (M7).
- **Stdin-piped query input** (`echo "..." | usearch query`). → SPEC-CLI-002 (M7).
- **Multiple positional prompts** (`usearch query "a" "b"` for batch). → Out of scope; future SPEC if measured value warrants.
- **Tag-based `--source` filtering** (e.g., `--source social` to mean "all social adapters"). v0 accepts adapter names only. → SPEC-CLI-002.
- **Output formats beyond `text` and `json`** (yaml, csv, markdown). → SPEC-CLI-002.
- **True LLM-streaming output** (token-by-token incremental synthesis). → SPEC-SYN-004 (M4).
- **Full SPEC-FAN-001 fanout** (per-adapter timeout, retry, dedup, partial-result ranking, RRF fusion). → SPEC-FAN-001 (M3); CLI-001 ships a deliberately-minimal CLI-internal fanout.
- **Cobra / urfave/cli / kong CLI library adoption**. → SPEC-CLI-002 if subcommand tree growth justifies.
- **Shell completion scripts** (bash / zsh / fish). → SPEC-CLI-002.
- **Configuration file support** (`~/.config/usearch/config.toml`). → Out of scope; env vars only for v0.
- **Interactive credential prompts**. → Adapter credentials come from env vars per `Capabilities.AuthEnvVars`.
- **Per-source rate-limit display** in CLI output. → Future SPEC-EVAL-002 (M8).
- **Result caching** (per-query cache key, TTL). → Out of scope; future SPEC if measured value warrants.
- **GitHub Issue tracking on this SPEC** (skipped per session pattern — `--auto` mode).

### 2.3 Forward-Looking Dependencies

[HARD] SPEC-CLI-001's `depends_on` list includes two SPECs that are currently in `draft` status:

- **SPEC-ADP-002 (Hacker News adapter)** — required for the M2 exit criterion's "Reddit + HN" claim. CLI-001 compiles and ships with Reddit-only if ADP-002 lands later; the adapter registry is dynamic.
- **SPEC-SYN-001 (Basic synthesis v0)** — required for the "synthesized paragraph + citations" claim. CLI-001 introduces the `internal/synthesis` interface and a `nopclient` (no-op) fallback so the binary compiles before SYN-001 is implemented; with the no-op, `query` exits with code 3 (partial) and prints the raw `NormalizedDoc.Snippet` list.

The plan-auditor MUST acknowledge this forward dependency and confirm that the run phase only begins once both SPECs reach at least `implemented` (the registry stub) status, OR explicitly approve CLI-001 to ship in a degraded mode.

---

## 3. EARS Requirements

### Functional Requirements

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| REQ-CLI-001 | Ubiquitous | The `usearch` binary SHALL dispatch on the first positional argument: `query` invokes the query pipeline; `--version` / `-v` invoke the existing version handler; any other first argument SHALL print a usage message to stderr and exit with code 2. | P0 | `TestUnknownSubcommandExits2`, `TestVersionFlagStillWorks`, `TestQuerySubcommandDispatched`. |
| REQ-CLI-002 | Ubiquitous | The `query` subcommand SHALL accept exactly one positional argument (the user prompt) and the flags `--source`, `--format`, `--timeout`; the subcommand SHALL parse via a per-subcommand `*flag.FlagSet` named `"query"` so flags do not leak between subcommands. | P0 | `TestQueryParsesPositionalAndFlags`, `TestQueryRejectsZeroPositional`, `TestQueryRejectsTwoPositionals`. |
| REQ-CLI-003 | Optional | WHERE the caller supplies `--source <names>`, the CLI SHALL parse the value as a comma-separated list of adapter names; an empty value (`--source ''` or absence of the flag) SHALL be equivalent to "all enabled adapters in the registry"; the effective adapter set SHALL be the INTERSECTION of `--source` and `RoutingDecision.AdapterSet`. | P1 | `TestSourceFlagFiltersAdapters`, `TestSourceFlagEmptyMeansAll`, `TestSourceFlagIntersectsRouterSet`. |
| REQ-CLI-004 | Event-Driven | WHEN the caller supplies `--format json`, the CLI SHALL emit a single JSON object to stdout containing fields `{schema_version, query, category, lang, adapters, summary, citations[], stats}`; WHEN `--format text` (default) is used, the CLI SHALL emit the synthesized paragraph followed by a `Citations:` block with numbered `[N] <Title> — <URL>` entries. The CLI SHALL reject any other `--format` value with exit code 1. | P0 | `TestFormatJSONShape`, `TestFormatTextShape`, `TestFormatInvalidExitsOne`. |
| REQ-CLI-005 | State-Driven | WHILE the parent context is active, the CLI SHALL enforce a total-pipeline deadline derived from `--timeout` (default 30s, max 300s). IF the deadline is exceeded, THEN every spawned goroutine SHALL be cancelled, no goroutine SHALL leak, and the CLI SHALL exit with code 2 and emit a stderr message naming the stage that timed out. | P0 | `TestTimeoutCancelsFanout`, `TestTimeoutLeavesNoGoroutineLeak` (uses `goleak`), `TestTimeoutExceedsMaxRejected`. |
| REQ-CLI-006 | Ubiquitous | The CLI SHALL emit the answer payload (text or JSON) to **stdout** EXCLUSIVELY; structured progress, slog records, error messages, and OTel admin output SHALL be emitted to **stderr** EXCLUSIVELY. The CLI SHALL NOT print any non-payload byte to stdout regardless of format. | P0 | `TestStdoutContainsOnlyPayload`, `TestStderrContainsProgress`, `TestStderrContainsErrorOnFailure`. |
| REQ-CLI-007 | Unwanted | IF the positional prompt is empty after `strings.TrimFunc(unicode.IsSpace)`, THEN the CLI SHALL print `"usearch query: prompt argument required"` to stderr, SHALL NOT call the router, SHALL NOT call any adapter, and SHALL exit with code 1. | P0 | `TestEmptyPromptExitsOne`, `TestWhitespacePromptExitsOne`, `TestNoLLMOrAdapterCallOnEmptyPrompt`. |
| REQ-CLI-008 | Event-Driven | WHEN at least one adapter returns at least one `NormalizedDoc` AND the synthesis client returns a non-empty summary, the CLI SHALL exit with code 0; WHEN at least one adapter returns docs but synthesis fails OR some adapters error but at least one succeeds, the CLI SHALL exit with code 3 (partial result) and still emit whatever payload is available. | P0 | `TestExitZeroOnFullSuccess`, `TestExitThreeOnSynthesisFailure`, `TestExitThreeOnPartialAdapterFailure`. |
| REQ-CLI-009 | State-Driven | WHILE the synthesis client is the no-op fallback (`internal/synthesis/nopclient.Client`) — i.e., SPEC-SYN-001 has not yet been implemented — the CLI SHALL still complete the pipeline, format a degraded answer (raw `NormalizedDoc.Snippet` list with manual numbering and a `[synthesis: unavailable]` warning on stderr), and exit with code 3. | P1 | `TestNopSynthesisProducesDegradedOutput`, `TestNopSynthesisExitsThree`, `TestNopSynthesisStderrCarriesWarning`. |
| REQ-CLI-010 | Ubiquitous | The CLI SHALL open one root OTel span named `usearch.cli.query` covering the entire pipeline; the span SHALL carry attributes `cli.prompt_length` (int), `cli.format` (string), `cli.source_filter_count` (int), `cli.adapter_set` (string, comma-joined), `cli.exit_code` (int); the span SHALL be a parent of the router/adapter/synthesis spans emitted by their packages. The span SHALL be ended exactly once before `os.Exit`. | P0 | `TestRootSpanCapturesAttributes`, `TestRootSpanIsParentOfRouter`, `TestRootSpanEndsBeforeExit`. |
| REQ-CLI-011 | Ubiquitous | The CLI SHALL generate a fresh request ID via `reqid.New()` at the start of every `query` invocation, attach it via `reqid.WithContext(ctx, id)`, and surface it in the JSON output's `stats.request_id` field; all stderr slog records SHALL carry the same `request_id` attribute. | P1 | `TestRequestIDInJSONStats`, `TestRequestIDInStderrSlogRecords`. |

### Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| NFR-CLI-001 | End-to-End Buildability | `usearch query "test"` SHALL succeed end-to-end on a freshly built scaffold (clean checkout + `docker-compose up -d` for LiteLLM proxy + `go build -o bin/usearch ./cmd/usearch` + `./bin/usearch query "test"`) with at least one adapter registered. The integration test SHALL run in CI under build tag `integration`, use stub adapter HTTP servers (no live network), and assert exit code ∈ {0, 3}. |
| NFR-CLI-002 | Goroutine Hygiene | Fanout cancellation SHALL honour the parent context's deadline: when the context is cancelled, every adapter goroutine SHALL return within 200ms, and `goleak.VerifyNone(t)` (called from a `TestMain` in `cmd/usearch/query_test.go`) SHALL detect zero leaked goroutines after every `Execute(...)` test case. |
| NFR-CLI-003 | Binary Size | The release-mode binary (`go build -ldflags "-s -w" -trimpath ./cmd/usearch`) SHALL be ≤ 30 MB on linux/amd64. CI SHALL fail on regression. This preserves the M1 footprint goal and forecloses premature cobra adoption. |
| NFR-CLI-004 | Human-Readable Errors | In default `--format text` mode, no error message printed to stderr SHALL contain a Go runtime stack trace, raw JSON error blob, or panic dump. Each user-visible error SHALL be a single line of plain English ≤ 200 characters that names the stage and the underlying cause. Stack traces are reserved for `LOG_LEVEL=DEBUG` mode (slog DEBUG records to stderr in JSON form). |

---

## 4. Acceptance Criteria

### REQ-CLI-001 — Subcommand Dispatch

- `cmd/usearch/main.go` declares a `dispatch(args []string) int` function that switches on `args[0]` (the first non-binary arg) and routes to `runVersion`, `runQuery`, or prints usage and exits 2.
- `TestUnknownSubcommandExits2`: invokes `go run . foobar`, asserts exit code 2 and stderr contains `"unknown subcommand"` and `"available: query, --version"`.
- `TestVersionFlagStillWorks`: existing `TestVersionFlag` and `TestVersionShortFlag` continue to pass unchanged.
- `TestQuerySubcommandDispatched`: invokes `go run . query "test"` with mocked router/adapter/synthesis; asserts `runQuery` was called.

### REQ-CLI-002 — Flag Parsing

- `cmd/usearch/query.go` constructs `fs := flag.NewFlagSet("query", flag.ContinueOnError)` so usage errors do not call `os.Exit` directly; the wrapper converts errors to exit code 1.
- `TestQueryParsesPositionalAndFlags`: invokes `query --format json --timeout 10s --source reddit "hello"`, asserts parsed `queryFlags{Source: ["reddit"], Format: "json", Timeout: 10*time.Second}` and `prompt == "hello"`.
- `TestQueryRejectsZeroPositional`: `query --format json` exits 1; stderr contains `"prompt argument required"`.
- `TestQueryRejectsTwoPositionals`: `query "a" "b"` exits 1; stderr contains `"exactly one positional argument expected"`.

### REQ-CLI-003 — Source Filter

- `TestSourceFlagFiltersAdapters`: registry contains reddit + hackernews; router returns `AdapterSet=["hackernews", "reddit"]`; invoke `query --source reddit "x"`; assert only reddit was called.
- `TestSourceFlagEmptyMeansAll`: registry contains reddit + hackernews; invoke `query --source '' "x"`; assert both reddit and hackernews were called.
- `TestSourceFlagIntersectsRouterSet`: router returns `AdapterSet=["reddit"]` (e.g., HN excluded by lang); invoke `query --source hackernews "x"`; assert empty effective set; CLI prints `"no adapters matched"` warning and exits 2.
- `TestSourceFlagUnknownAdapterRejected`: invoke `query --source nosuchadapter "x"`; CLI exits 1 with `"unknown adapter 'nosuchadapter'"`.

### REQ-CLI-004 — Format Flag

- `TestFormatJSONShape`: invoke `query --format json "test"`; capture stdout; assert it parses as JSON; assert top-level keys exactly `{schema_version, query, category, lang, adapters, summary, citations, stats}`; assert `schema_version == "1"`.
- `TestFormatTextShape`: invoke `query --format text "test"`; assert stdout starts with the summary paragraph, contains a blank line, then `"Citations:"`, then numbered `[1] <Title> — <URL>` lines.
- `TestFormatInvalidExitsOne`: `query --format yaml "x"` exits 1; stderr contains `"unsupported format 'yaml'; valid: text, json"`.
- `TestFormatDefaultIsText`: `query "x"` (no `--format`) produces text output identical to `--format text`.

### REQ-CLI-005 — Timeout

- `TestTimeoutCancelsFanout`: stub adapter blocks for 60s; `query --timeout 100ms "x"`; assert total elapsed ≤ 300ms; exit code 2; stderr contains `"timeout: fanout stage"`.
- `TestTimeoutLeavesNoGoroutineLeak`: same scenario; `goleak.VerifyNone(t)` reports zero leaks.
- `TestTimeoutExceedsMaxRejected`: `query --timeout 10m "x"` (exceeds 5min ceiling) exits 1; stderr contains `"--timeout exceeds maximum 5m0s"`.
- `TestTimeoutDefaultIs30s`: invoke without `--timeout`; capture parsed value; assert `30 * time.Second`.

### REQ-CLI-006 — stdout/stderr Separation

- `TestStdoutContainsOnlyPayload`: invoke `query --format json "x"` with a working pipeline; assert stdout is parseable as a SINGLE JSON object with no preamble; assert stderr contains progress lines and slog records but is not JSON-by-itself.
- `TestStderrContainsProgress`: text mode; assert stderr contains `"[router] classified"`, `"[fanout]"`, `"[synthesis]"` markers.
- `TestStderrContainsErrorOnFailure`: invoke a path that fails; assert stdout is empty (or contains a partial-result payload only); assert stderr names the failure stage.

### REQ-CLI-007 — Empty Prompt Rejection

- `TestEmptyPromptExitsOne`: `query ""` exits 1.
- `TestWhitespacePromptExitsOne`: `query "   "` and `query "\t\n"` both exit 1.
- `TestNoLLMOrAdapterCallOnEmptyPrompt`: assert mock router's `Classify` was called zero times when prompt was empty.

### REQ-CLI-008 — Exit Codes

- `TestExitZeroOnFullSuccess`: full mock pipeline (≥1 doc, non-empty summary); assert exit 0.
- `TestExitThreeOnSynthesisFailure`: adapter returns docs; synthesis client returns error; assert exit 3; stdout contains the docs in degraded format.
- `TestExitThreeOnPartialAdapterFailure`: 2 adapters; one returns docs, one errors; synthesis succeeds with the partial set; assert exit 3.
- `TestExitTwoOnAllAdaptersFail`: all adapters error; assert exit 2; stderr names the failure.

### REQ-CLI-009 — Synthesis Soft-Fail

- `TestNopSynthesisProducesDegradedOutput`: register `internal/synthesis/nopclient.Client`; invoke `query`; assert stdout contains the raw `Snippet` list with manual numbering.
- `TestNopSynthesisExitsThree`: same scenario; exit code 3.
- `TestNopSynthesisStderrCarriesWarning`: same scenario; stderr contains `"[synthesis: unavailable]"`.

### REQ-CLI-010 — Root Span

- `TestRootSpanCapturesAttributes`: in-memory OTel exporter captures the span; assert attributes set: `cli.prompt_length`, `cli.format`, `cli.source_filter_count`, `cli.adapter_set`, `cli.exit_code`.
- `TestRootSpanIsParentOfRouter`: assert the captured `router.classify` span's parent ID matches the root span's ID.
- `TestRootSpanEndsBeforeExit`: assert `span.End()` is called before `os.Exit`; verified via mock exit function.

### REQ-CLI-011 — Request ID

- `TestRequestIDInJSONStats`: `query --format json "x"`; parse stdout; assert `stats.request_id` is a valid 26-char Crockford Base32 ULID.
- `TestRequestIDInStderrSlogRecords`: capture stderr; parse each JSON slog line; assert every line has the SAME `request_id` value matching the JSON output.

### NFR-CLI-001 — End-to-End Build

- Integration test under `// +build integration` tag (`cmd/usearch/integration_test.go`):
  - Spins up stub HTTP servers for Reddit + HN.
  - Spins up stub LiteLLM proxy stub for the router's LLM-fallback path.
  - Builds the binary fresh (`go build -o /tmp/usearch ./cmd/usearch`).
  - Invokes `/tmp/usearch query "hello world"` with adapter base URLs pointed at the stubs.
  - Asserts exit code ∈ {0, 3} and stdout is non-empty.
- CI runs this test under `go test -tags=integration ./cmd/usearch/...`.

### NFR-CLI-002 — Goroutine Hygiene

- `cmd/usearch/query_test.go` declares `func TestMain(m *testing.M)` invoking `goleak.VerifyTestMain(m)`.
- `BenchmarkExecute` runs 100 iterations and reports `runtime.NumGoroutine()` before / after; delta SHALL be 0.

### NFR-CLI-003 — Binary Size

- CI step: `go build -ldflags "-s -w" -trimpath -o /tmp/usearch ./cmd/usearch && du -m /tmp/usearch | awk '{print $1}'`.
- Asserts size ≤ 30; fails with `"binary size regressed: <N>MB > 30MB cap"` on violation.

### NFR-CLI-004 — Human-Readable Errors

- `TestErrorMessagesNoStackTrace`: table-driven; for each error path, capture stderr; assert NO line contains `"goroutine "` or `"runtime.gopark"` or starts with `"\t"`.
- `TestErrorMessagesUnder200Chars`: assert each error line ≤ 200 chars.
- `TestDebugModeRevealsStackTraces`: with `LOG_LEVEL=DEBUG`, assert stderr DOES contain a JSON slog DEBUG record on errors.

---

## 5. Technical Approach

### 5.1 Files to Modify (Summary)

**Created (10 files)**:

- `cmd/usearch/query.go` — subcommand orchestrator, `Execute`, `runQuery`, `runFanout`, `queryFlags`
- `cmd/usearch/query_test.go` — `TestExecute` table-driven + `TestMain` with goleak
- `cmd/usearch/output_text.go` — `formatText`
- `cmd/usearch/output_text_test.go`
- `cmd/usearch/output_json.go` — `formatJSON`, schema-versioned object
- `cmd/usearch/output_json_test.go`
- `cmd/usearch/progress.go` — `progressEmitter` interface + `humanProgress` + `jsonProgress`
- `cmd/usearch/progress_test.go`
- `cmd/usearch/exitcode.go` — exit-code constants + `classifyError`
- `cmd/usearch/exitcode_test.go`
- `cmd/usearch/integration_test.go` — under `// +build integration` tag (NFR-CLI-001)
- `internal/synthesis/client.go` — `Client` interface + `Request` / `Response` / `Citation` types
- `internal/synthesis/nopclient/nopclient.go` — `Client` implementation that returns `ErrSynthesisUnavailable`
- `internal/synthesis/nopclient/nopclient_test.go`

**Modified (2 files)**:

- `cmd/usearch/main.go` — replace lines 23-75 with subcommand dispatcher; preserve `--version` / `-v` / `--no-llm` semantics; construct adapter registry post-`obs.Init`
- `cmd/usearch/main_test.go` — additive: `TestQuerySubcommandHelp`, `TestUnknownSubcommandExits2`

**Unchanged (by design)**:

- `internal/router/*` — consumed via existing API; no changes
- `internal/adapters/registry.go` — consumed via `Register` / `Get` / `List`; no changes
- `internal/adapters/reddit/*` — consumed via existing `New` + `Register`; no changes
- `internal/llm/*` — consumed via existing `Client`; no changes
- `internal/obs/*` — consumed via existing `Init` + `Logger` + `Tracer`; no changes
- `pkg/types/*` — consumed via existing `Query` + `NormalizedDoc`; no changes

### 5.2 Package layout

```
cmd/usearch/
├── main.go               # subcommand dispatcher (modified)
├── main_test.go          # existing + 2 new tests (modified)
├── query.go              # query subcommand orchestrator (NEW)
├── query_test.go         # table-driven Execute tests + TestMain goleak (NEW)
├── output_text.go        # text formatter (NEW)
├── output_text_test.go   # (NEW)
├── output_json.go        # JSON formatter, schema_version=1 (NEW)
├── output_json_test.go   # (NEW)
├── progress.go           # progress emitter interface + impls (NEW)
├── progress_test.go      # (NEW)
├── exitcode.go           # constants + classifyError (NEW)
├── exitcode_test.go      # (NEW)
└── integration_test.go   # NFR-CLI-001, build tag integration (NEW)

internal/synthesis/
├── client.go             # Client interface + types (NEW)
└── nopclient/
    ├── nopclient.go      # no-op fallback (NEW)
    └── nopclient_test.go # (NEW)
```

### 5.3 CLI Library Choice — stdlib `flag` (justification)

Selected: **stdlib `flag` package**.

Decision rationale (full survey in `research.md` §2):

1. M2 introduces ONE subcommand (`query`). The argument count (3 flags + 1 positional) is well below cobra's break-even point. `cobra` adds ~500KB to binary size and ~12 transitive dependencies.
2. NFR-CLI-003 caps binary size at 30MB. Adding cobra here is premature complexity.
3. The existing pattern at `cmd/usearch/main.go:24-27` uses stdlib `flag` already; subcommand dispatch is a small extension via `*flag.FlagSet`-per-subcommand.
4. Tests already use process-boundary `os/exec` invocation (`main_test.go:18-30`); the test pattern carries forward verbatim.
5. Migration to cobra in SPEC-CLI-002 is mechanical: extract `Execute(args, stdout, stderr) int` becomes `*cobra.Command`'s `RunE`. The boundary is preserved.

Tradeoff vs cobra:

| Aspect | stdlib `flag` (chosen) | cobra (deferred) |
|--------|------------------------|------------------|
| Binary size delta | 0 KB | +500 KB |
| Transitive deps | 0 | ~12 |
| Help generator | hand-rolled, minimal | rich, auto-generated |
| Subcommand tree depth | flat, OK for ≤ 2 commands | scales to N levels |
| Shell completion | none | bash/zsh/fish auto |
| Test surface | identical | identical |
| Breakeven point | ≤ 4 subcommands | > 4 subcommands |

When cobra wins: SPEC-CLI-002 (M7) introduces `usearch deep`, `usearch team list`, `usearch team members`, `usearch team add`, `usearch admin status`, etc. Five+ subcommands across two levels of nesting is the cobra sweet spot.

### 5.4 Pipeline Sketch (illustrative; final shape in run phase)

```go
// cmd/usearch/query.go
func Execute(ctx context.Context, args []string, stdout, stderr io.Writer) int {
    flags, prompt, err := parseQueryFlags(args)
    if err != nil {
        fmt.Fprintln(stderr, err)
        return ExitUserError // 1
    }
    if strings.TrimSpace(prompt) == "" {
        fmt.Fprintln(stderr, "usearch query: prompt argument required")
        return ExitUserError
    }

    ctx, cancel := context.WithTimeout(ctx, flags.Timeout)
    defer cancel()
    ctx = reqid.WithContext(ctx, reqid.New())

    o, _, err := obs.Init(ctx, obsConfig())
    if err != nil { return ExitSystemError }

    tracer := o.Tracer("usearch.cli")
    spanCtx, span := tracer.Start(ctx, "usearch.cli.query",
        oteltrace.WithAttributes(
            attribute.Int("cli.prompt_length", len(prompt)),
            attribute.String("cli.format", flags.Format),
        ),
    )
    defer span.End()

    registry := buildRegistry(o)
    llmClient := buildLLMClient(o)
    rtr, err := router.New(router.Options{
        Registry:  registry,
        LLMClient: llmClient,
        Obs:       o,
    })
    if err != nil { return ExitSystemError }

    decision, err := rtr.Classify(spanCtx, router.RouterQuery{
        Query: types.Query{Text: prompt, Deadline: time.Now().Add(flags.Timeout)},
    })
    if err != nil { return ExitUserError } // ErrInvalidQuery

    effectiveSet := intersectSources(decision.AdapterSet, flags.Source)
    if len(effectiveSet) == 0 { return ExitSystemError }

    docs, errs := runFanout(spanCtx, effectiveSet, registry, prompt)

    synth := buildSynthesisClient(o)  // returns nopclient.Client when SYN-001 not ready
    resp, synthErr := synth.Synthesize(spanCtx, synthesis.Request{
        Query: prompt, Decision: decision, Docs: docs,
    })

    exitCode := determineExitCode(docs, errs, resp, synthErr)
    span.SetAttributes(attribute.Int("cli.exit_code", exitCode))

    if flags.Format == "json" {
        formatJSON(stdout, buildResponse(prompt, decision, effectiveSet, resp, errs))
    } else {
        formatText(stdout, buildResponse(prompt, decision, effectiveSet, resp, errs))
    }
    return exitCode
}
```

### 5.5 Synthesis Client Interface (NEW package)

```go
// internal/synthesis/client.go
package synthesis

import (
    "context"
    "github.com/elymas/universal-search/internal/router"
    "github.com/elymas/universal-search/pkg/types"
)

type Client interface {
    Synthesize(ctx context.Context, req Request) (*Response, error)
}

type Request struct {
    Query    string
    Decision router.RoutingDecision
    Docs     []types.NormalizedDoc
}

type Response struct {
    Summary   string
    Citations []Citation
    CostUSD   float64
}

type Citation struct {
    Index    int    `json:"index"`
    Title    string `json:"title"`
    URL      string `json:"url"`
    Source   string `json:"source"` // adapter name
    DocID    string `json:"doc_id"`
}

var ErrSynthesisUnavailable = errors.New("synthesis: client unavailable")
```

The `nopclient.Client` returns `ErrSynthesisUnavailable` from `Synthesize`. SPEC-SYN-001 will provide a real HTTP client implementation in this same `internal/synthesis/` package.

### 5.6 MX Tag Plan

| File | Tag | Reason |
|------|-----|--------|
| `query.go::Execute` | @MX:ANCHOR | fan_in ≥ 3 (main.go, query_test.go, future cobra wrapper). @MX:REASON: sole CLI entry boundary; signature stability matters for SPEC-CLI-002 migration |
| `query.go::runFanout` | @MX:ANCHOR | fan_in ≥ 2 initially (Execute + tests); @MX:REASON: replacement target when SPEC-FAN-001 lands; clean function boundary |
| `query.go::runFanout` | @MX:WARN | basic CLI-internal fanout; lacks retry, dedup, partial-result ranking. @MX:REASON: temporary scaffolding until SPEC-FAN-001 (M3); update tag when replaced |
| `exitcode.go::classifyError` | @MX:NOTE | exit-code mapping is the load-bearing UX contract; behaviour change ripples to CI scripts |
| `output_json.go::schemaVersion` | @MX:NOTE | constant `"1"`; bump on breaking changes; documented in CHANGELOG |
| `progress.go::progressEmitter` | @MX:NOTE | interface boundary between text and JSON progress modes |

Per `.claude/rules/moai/workflow/mx-tag-protocol.md`: `[AUTO]` prefix on agent-generated tags; `@MX:REASON` mandatory for ANCHOR + WARN; `@MX:SPEC: SPEC-CLI-001` on all tags. Per `.moai/config/sections/language.yaml` (`code_comments: en`), all @MX descriptions in English.

### 5.7 Harness Level

Per `.moai/config/sections/harness.yaml` auto-routing: 11 REQs (8 × P0 + 3 × P1) + 4 NFRs touching 1 cmd directory (`cmd/usearch/`) + 1 new package (`internal/synthesis/`) = **standard** harness level. Sprint Contract is optional; `evaluator-active` profile `default` applies.

### 5.8 Test Plan (`cmd/usearch/query_test.go`)

Primary test: `TestExecute` — table-driven with the following case matrix:

| Case | Prompt | Flags | Mock router | Mock adapters | Mock synth | Expect stdout | Expect stderr | Expect exit |
|------|--------|-------|-------------|---------------|------------|---------------|---------------|-------------|
| success_text | "hello" | text/30s | social, [reddit] | reddit→3 docs | summary+3 cites | text payload | progress | 0 |
| success_json | "hello" | json/30s | social, [reddit] | reddit→3 docs | summary+3 cites | JSON object | slog records | 0 |
| empty_prompt | "" | text/30s | n/a | n/a | n/a | empty | "prompt required" | 1 |
| whitespace_prompt | "   " | text/30s | n/a | n/a | n/a | empty | "prompt required" | 1 |
| invalid_format | "hi" | yaml/30s | n/a | n/a | n/a | empty | "unsupported format" | 1 |
| timeout_exceeded | "hi" | text/100ms | social, [slow] | slow→blocks 60s | n/a | empty | "timeout: fanout" | 2 |
| timeout_max | "hi" | text/10m | n/a | n/a | n/a | empty | "exceeds maximum" | 1 |
| no_adapters | "hi" | text/30s | empty AdapterSet (post-fallback) | n/a | n/a | empty | "no adapters matched" | 2 |
| source_filter | "hi" | source=reddit/text | social, [reddit, hn] | reddit→3 docs | summary | text payload | progress | 0 |
| source_unknown | "hi" | source=foo/text | n/a | n/a | n/a | empty | "unknown adapter" | 1 |
| partial_failure | "hi" | text/30s | social, [a, b] | a→2 docs, b→error | summary | text payload | warning on b | 3 |
| synthesis_failure | "hi" | text/30s | social, [a] | a→2 docs | error | docs (degraded) | "synthesis failed" | 3 |
| nop_synthesis | "hi" | text/30s | social, [a] | a→2 docs | nopclient | docs (degraded) | "[synthesis: unavailable]" | 3 |
| all_adapters_fail | "hi" | text/30s | social, [a, b] | a→error, b→error | n/a | empty | "all adapters failed" | 2 |

`TestMain` invokes `goleak.VerifyTestMain(m)` per NFR-CLI-002.

---

## 6. Dependencies

### 6.1 Upstream SPEC Dependencies

- **SPEC-IR-001 (implemented)**: `router.Router.Classify` + `RoutingDecision`. HARD dep.
- **SPEC-ADP-001 (implemented)**: Reddit adapter. HARD dep — without at least one registered adapter, `router.New` returns `ErrAdapterRegistryEmpty`.
- **SPEC-OBS-001 (implemented)**: `obs.Init` + `obs.Logger` + `obs.Tracer`. HARD dep.
- **SPEC-LLM-001 (implemented)**: `llm.Client` (transitive via router). HARD dep.

### 6.2 Forward-Looking Dependencies (currently `draft`)

- **SPEC-ADP-002 (HN adapter)**: required for the M2 exit criterion's "Reddit + HN" wording. CLI-001 ships with Reddit-only if ADP-002 is not yet implemented; the registry is dynamic.
- **SPEC-SYN-001 (Basic synthesis v0)**: required for the synthesized paragraph claim. CLI-001 introduces the `internal/synthesis` interface and a `nopclient.Client` fallback; with the no-op, `query` exits 3 (partial) and prints raw snippets.

### 6.3 Downstream Blocked SPECs

- **SPEC-CLI-002 (M7)**: extends the subcommand tree (`deep`, `team list`, etc.); migrates from stdlib `flag` to cobra.
- **SPEC-MCP-001 (M7)**: the MCP server reuses the synthesis client interface introduced here.
- **SPEC-SYN-004 (M4)**: streaming response builds on the format/output layer here.

### 6.4 External Dependencies (run-phase pins)

No new Go module dependencies expected. `cmd/usearch/` uses only:

- Go stdlib (`context`, `errors`, `flag`, `fmt`, `io`, `os`, `strings`, `time`, `unicode`)
- `golang.org/x/sync/errgroup` (already pinned per ADP-001)
- `go.uber.org/goleak` (already pinned per ADP-001 NFR-ADP-003 testing)
- `github.com/elymas/universal-search/internal/{router,adapters,llm,obs,obs/reqid,synthesis}`
- `github.com/elymas/universal-search/pkg/types`
- `go.opentelemetry.io/otel/{attribute,trace}` (already pinned)

---

## 7. Risks (Summary — full risk table in research.md §9)

| Risk | Severity | Mitigation |
|------|----------|------------|
| SPEC-SYN-001 not ready by run phase | High | nopclient soft-fail (REQ-CLI-009); exit code 3; document degraded mode |
| SPEC-ADP-002 not ready | Medium | CLI runs Reddit-only; partial M2 exit; `--source reddit` works end-to-end |
| Goroutine leak in basic fanout | High | NFR-CLI-002 + `goleak.VerifyTestMain`; `errgroup.WithContext` |
| Binary size creep above 30MB | Medium | NFR-CLI-003 CI gate; stdlib `flag` only |
| Stdout pollution from libraries | Medium | All status via `obs.Logger` to stderr; reserve stdout strictly for payload |
| Empty AdapterSet from router | Medium | Router REQ-IR-008 fallback; CLI exits 2 if STILL empty |
| LLM timeout cascades into total CLI timeout | Medium | Router internal 2s deadline (REQ-IR-007); CLI parent ctx |
| Tests bind to live Reddit / HN / LiteLLM | High | All tests use `httptest.Server` per ADP-001 D4; CLI tests use mocks |

---

## 8. Open Questions (6 entries in research.md §10)

These are explicitly unresolved at SPEC-approval time and documented in the research artifact rather than pre-decided. They do not block SPEC approval. Highlights:

1. `--source` accepts adapter names only for v0; tag-based filtering deferred to CLI-002.
2. Stdin-piped query input deferred to CLI-002; positional argument only for v0.
3. Default `--timeout` 30s; revisit after SPEC-FAN-001 measured p95.
4. JSON `schema_version` set to `"1"` from day one.
5. Error-message locale honours `language.yaml` (`error_messages: en`).
6. `--no-llm` flag preserved; router degrades gracefully when LLM client is nil.

See `.moai/specs/SPEC-CLI-001/research.md` §10 for full annotated list.

---

## 9. References

External (cited in research.md):

- Go stdlib `flag` package — `https://pkg.go.dev/flag`
- `golang.org/x/sync/errgroup` — `https://pkg.go.dev/golang.org/x/sync/errgroup`
- `github.com/spf13/cobra` (deferred) — `https://pkg.go.dev/github.com/spf13/cobra`
- BSD `sysexits.h` (exit-code conventions)

Internal (project files; full citation list in research.md §11):

- `.moai/specs/SPEC-IR-001/spec.md` — Intent Router contract
- `.moai/specs/SPEC-ADP-001/spec.md` — Reddit adapter (reference)
- `.moai/specs/SPEC-ADP-002/spec.md` — HN adapter (draft, forward-looking)
- `.moai/specs/SPEC-SYN-001/spec.md` — synthesis v0 (draft, forward-looking)
- `.moai/specs/SPEC-OBS-001/spec.md` — observability bundle
- `.moai/specs/SPEC-CORE-001/spec.md` — Adapter / Capabilities / Query / NormalizedDoc contracts
- `.moai/project/roadmap.md` §M2 row + §5 M2 exit criterion
- `cmd/usearch/main.go` — current entrypoint (--version only)
- `cmd/usearch/main_test.go` — existing process-boundary test pattern
- `internal/router/router.go` — Router API surface
- `internal/router/routing_decision.go` — RoutingDecision type
- `internal/adapters/registry.go` — Registry API
- `internal/adapters/reddit/reddit.go` — adapter usage example
- `internal/obs/obs.go` — Obs bundle, Init lifecycle
- `internal/obs/reqid/reqid.go` — request ID generator
- `pkg/types/{adapter.go, query.go, normalized_doc.go}` — typed contract

---

*End of SPEC-CLI-001 v0.1 (draft).*
