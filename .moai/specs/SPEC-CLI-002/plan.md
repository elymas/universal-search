# Plan — SPEC-CLI-002: `usearch` CLI v1

Date: 2026-05-22
Author: limbowl (via manager-spec)
Status: draft
Methodology: TDD (per `quality.development_mode`)
Coverage target: 85%
Harness level: standard

This plan operationalizes SPEC-CLI-002 into a phased TDD implementation
sequence. The plan is structured as **five sequential phases** with
explicit gates between phases. Each phase concludes with green tests +
no-leak goroutine verification + binary-size check.

This plan does NOT contain implementation code (per moai-constitution
"Enforce Simplicity" and Coding Standards §"Content Restrictions"). It
enumerates files, test ordering, and acceptance gates only. The actual
RED-GREEN-REFACTOR cycles happen in the run phase under
`/moai run SPEC-CLI-002`.

---

## 1. Phasing Overview

The implementation is structured to **minimize backward-compat risk**:
cobra migration lands first (Phase 1) so all subsequent additions plug
into a stable framework. Streaming and `deep` land last because they
have the largest test surface and external dependencies.

| Phase | Priority | Scope | Gate to next phase |
|-------|----------|-------|--------------------|
| 1. Cobra migration + Config | High | `usearch query` migrated to cobra; `config` subcommand tree + koanf loader + XDG path | All v0 tests pass; binary ≤ 30 MB; `config show / path / init / get / set` work |
| 2. History | High | JSONL backend + async writer + retention + FIFO; `history list / show / search / clear` subcommands | History writes don't block CLI exit; goleak passes; retention logic verified |
| 3. Interactive REPL | Medium | Zero-args readline REPL + slash commands + history integration; bubbletea REPL behind `tui` build tag | Default REPL works without TUI deps; TUI build passes under `-tags tui` |
| 4. Streaming | Medium | SSE consumer + `--stream`/`--no-stream` flag + TTY auto-detect; wire into `query` | `--stream` against SPEC-SYN-004 SSE produces sentence-by-sentence rendering; piped invocation unchanged |
| 5. Deep + Markdown + Login + Polish | Medium | `usearch deep`; DEEP-004 quota error matrix; `--format markdown`; `sources` subcommand; `login` placeholder; completion generators | All NFRs hold; v0 tests still pass; v1 tests at ≥ 85% coverage |

Total estimated EARS REQ coverage per phase:

- Phase 1: REQ-CLI2-001, REQ-CLI2-002 (partial), REQ-CLI2-007,
  REQ-CLI2-012
- Phase 2: REQ-CLI2-010, REQ-CLI2-011
- Phase 3: REQ-CLI2-008
- Phase 4: REQ-CLI2-005
- Phase 5: REQ-CLI2-002 (complete), REQ-CLI2-003, REQ-CLI2-004,
  REQ-CLI2-006, REQ-CLI2-009, REQ-CLI2-013, REQ-CLI2-014; all NFRs
  validated.

---

## 2. Phase 1 — Cobra Migration + Config Foundation

**Goal**: Replace stdlib `flag` with cobra; preserve v0 invocation;
land `usearch config` subcommand tree on top of koanf.

### 2.1 Files

**Modify**:

- `cmd/usearch/main.go` — replace stdlib `flag` dispatcher with cobra
  root command; register `query` subcommand wrapping
  `query.go::Execute` as `RunE`. Preserve `--version` / `-v` (cobra
  has built-in `--version`, hook into existing Version constant).
- `cmd/usearch/query.go` — minor surface adjustment so cobra `RunE`
  can call `Execute(ctx, args, stdout, stderr)` without conflict.
- `cmd/usearch/main_test.go` — assert cobra-built binary still passes
  v0 invocations (additive tests only; existing tests must pass
  unchanged).

**Create**:

- `cmd/usearch/root.go` — cobra root command definition; registers
  all subcommands and global flags (`--config`, `--log-level`).
- `cmd/usearch/config_cmd.go` — cobra command tree for `usearch
  config {path,show,init,get,set}`.
- `cmd/usearch/config_cmd_test.go` — table-driven tests for each
  config subcommand.
- `internal/usearch/config/config.go` — koanf loader; XDG path
  resolution; schema struct (matching `research.md` §7.3 TOML).
- `internal/usearch/config/config_test.go` — precedence, defaults,
  malformed file rejection.
- `internal/usearch/config/xdg.go` — XDG_CONFIG_HOME / XDG_DATA_HOME
  resolution (via `adrg/xdg`).
- `internal/usearch/config/xdg_test.go` — env-overrideable resolution
  test.

### 2.2 RED-GREEN-REFACTOR sequence

**RED 1**: `TestQueryV0InvocationBackwardCompatible` — table-driven
asserting all v0 flag combinations produce identical exit codes and
stdout/stderr byte shapes. (Fails because cobra not yet wired.)

**GREEN 1**: Implement cobra root + `query` subcommand wrapper. Verify
RED 1 passes; verify all CLI-001 v0 tests pass unchanged.

**REFACTOR 1**: Extract shared cobra patterns (PersistentPreRun for
obs init, error → exit code mapping helper).

**RED 2**: `TestConfigPath`, `TestConfigInitCreatesFile`,
`TestConfigShow` (against a temp HOME).

**GREEN 2**: Implement koanf loader + XDG resolution + `config` subcommand
tree.

**REFACTOR 2**: Consolidate config-key validation; ensure
`set`-refuse-token-key works (REQ-CLI2-012).

**RED 3**: `TestConfigPrecedenceFlagOverEnvOverFileOverDefault`.

**GREEN 3**: Wire flag/env/file layering via koanf providers.

**REFACTOR 3**: Schema struct exposes typed getters for downstream
consumers (history, deep).

### 2.3 Exit gate

- All v0 acceptance tests pass byte-for-byte unchanged.
- `usearch --help` lists `query` and `config`.
- `usearch config init` creates `~/.config/usearch/config.toml` in a
  temp HOME and `usearch config show` round-trips.
- Binary size ≤ 30 MB (release build).
- `goleak.VerifyTestMain` passes for `query` and `config`.

---

## 3. Phase 2 — History Backend + Subcommands

**Goal**: Persist successful query/deep invocations; expose
`history list / show / search / clear`; default JSONL backend; SQLite
opt-in behind `history-sqlite` build tag.

### 3.1 Files

**Create**:

- `internal/usearch/history/history.go` — `Entry` struct, `Backend`
  interface (`Write`, `List`, `Get`, `Search`, `Clear`).
- `internal/usearch/history/jsonl.go` — append-only JSONL writer +
  reader; FIFO eviction; retention purge.
- `internal/usearch/history/jsonl_test.go` — concurrency, FIFO,
  retention.
- `internal/usearch/history/sqlite.go` — SQLite + FTS5 backend (build
  tag `history-sqlite`).
- `internal/usearch/history/sqlite_test.go` — gated by build tag.
- `internal/usearch/history/async.go` — background goroutine writer
  with 100 ms drain on shutdown.
- `internal/usearch/history/async_test.go` — verify non-blocking +
  goleak.
- `cmd/usearch/history.go` — cobra command tree.
- `cmd/usearch/history_test.go` — each subcommand.

**Modify**:

- `cmd/usearch/query.go` — emit history entry on successful exit
  (fire-and-forget).
- `cmd/usearch/deep.go` (will exist after phase 5) — same.

### 3.2 RED-GREEN-REFACTOR sequence

**RED 1**: `TestHistoryWriteOnSuccessfulQuery` — invoke query; assert
JSONL line appended with full entry.

**GREEN 1**: Implement JSONL backend + writer integration.

**REFACTOR 1**: Extract Entry construction into a shared helper.

**RED 2**: `TestHistoryWriteAsyncDoesNotBlockExit` — block writer
(channel-full simulation); assert CLI exits within 100 ms with WARN
log.

**GREEN 2**: Implement async drain + grace period.

**RED 3**: History subcommand tests.

**GREEN 3**: `usearch history list / show / search / clear`.

**RED 4**: SQLite backend tests (under `-tags history-sqlite`).

**GREEN 4**: Implement SQLite backend with FTS5 indexing.

### 3.3 Exit gate

- History entries written for every successful `query` invocation.
- `history list/show/search/clear` work against JSONL default.
- `go test -tags history-sqlite ./internal/usearch/history/...` passes.
- Async writer goleak-clean.
- Binary size default ≤ 30 MB; `-tags history-sqlite` ≤ 36 MB.

---

## 4. Phase 3 — Interactive REPL

**Goal**: Zero-args invocation enters a REPL with slash-commands;
default uses readline; bubbletea TUI behind `tui` build tag.

### 4.1 Files

**Create**:

- `cmd/usearch/repl.go` — default readline-based REPL; slash-command
  dispatcher (`/exit`, `/help`, `/deep`, `/sources`, `/history`,
  `/config`).
- `cmd/usearch/repl_test.go` — stdin/stdout piped via bytes buffers.
- `cmd/usearch/repl_tui.go` — bubbletea REPL; gated by `//go:build
  tui`.
- `cmd/usearch/repl_tui_test.go` — under `tui` tag; smoke test for
  Model-View-Update cycle.

**Modify**:

- `cmd/usearch/root.go` — when `RunE` is invoked with zero args + TTY
  stdin, dispatch to `repl.Run()`.

### 4.2 RED-GREEN-REFACTOR sequence

**RED 1**: `TestREPLEntryOnZeroArgs` — invoke `usearch` (no args)
with TTY-mocked stdin; assert REPL prompt emitted.

**GREEN 1**: Implement zero-args + TTY detection + REPL entry.

**RED 2**: `TestREPLSkipsWhenStdinPiped` — invoke `usearch` with piped
stdin; assert help printed, exit 0, no REPL.

**GREEN 2**: TTY guard.

**RED 3**: `TestREPLBasicQueryFlow`, `TestREPLSlashExit`,
`TestREPLSlashHelp`.

**GREEN 3**: Implement input loop + slash dispatch.

**RED 4**: `TestREPLHistoryPersisted` — REPL query writes a history
entry.

**GREEN 4**: Wire phase 2 history into REPL.

**RED 5** (under `-tags tui`): `TestTUIReplStreamingRender` — bubbletea
viewport receives streamed sentences.

**GREEN 5**: Implement bubbletea Model with input field + viewport;
SSE messages drive viewport updates.

### 4.3 Exit gate

- `usearch` (no args, TTY) enters REPL.
- `usearch` (no args, piped stdin) prints help.
- `/exit` exits within 50 ms (NFR-CLI2-003).
- Default build does NOT include bubbletea (binary ≤ 30 MB).
- `-tags tui` build passes and binary ≤ 45 MB.

---

## 5. Phase 4 — Streaming (SPEC-SYN-004 SSE Consumer)

**Goal**: `--stream`/`--no-stream` flag on `query` and `deep` (deep
landed in phase 5); SSE consumer renders sentence-by-sentence; TTY
auto-detect.

### 5.1 Files

**Create**:

- `internal/usearch/sse/client.go` — stdlib `bufio.Scanner`-based SSE
  parser; emits typed event channel.
- `internal/usearch/sse/client_test.go` — table of SSE wire fragments
  → events; handles partial reads, heartbeats, EOF, error events.
- `cmd/usearch/stream.go` — orchestrator: builds HTTP request with
  proper Accept header; consumes SSE events; routes to renderer per
  `--format`.
- `cmd/usearch/stream_test.go` — `httptest.Server` emits pre-recorded
  SSE sequences; assert renderer output.

**Modify**:

- `cmd/usearch/query.go` — accept `--stream`/`--no-stream` flag;
  when streaming, delegate response handling to `stream.go`.
- `cmd/usearch/output_text.go` — add incremental-render mode for
  streaming; preserve buffered v0 behaviour for non-streaming.
- `cmd/usearch/output_json.go` — collect events into final v0
  schema_version=1 object on `event: done`; non-streaming unchanged.

### 5.2 RED-GREEN-REFACTOR sequence

**RED 1**: `TestSSEParserHandlesW3CWireFragments` — table of SSE
wire shapes (single event, multi-line data, heartbeat comment,
event: done, event: error, partial chunk split across reads).

**GREEN 1**: Implement parser.

**RED 2**: `TestStreamFlagRequestsSSE`, `TestNoStreamFlagRequestsJSON`,
`TestStreamAutoDetectTTY`.

**GREEN 2**: Wire flag + TTY auto-detect.

**RED 3**: `TestStreamRendersSentenceByQ`, `TestStreamHandlesEventDone`,
`TestStreamHandlesEventError`, `TestStreamCancelOnEOF`.

**GREEN 3**: Render integration into text/markdown formats; JSON
buffering.

**RED 4**: `TestStreamGoroutineLeakOnCancel` (goleak).

**GREEN 4**: Ensure parser goroutine released within
`SYN004_DISCONNECT_CANCEL_MS + 100 ms`.

### 5.3 Exit gate

- Streaming consumer handles all SYN-004 wire events correctly.
- Piped invocation (`usearch query "x" | jq`) defaults to non-stream
  → v0 byte-equivalent JSON.
- TTY invocation defaults to stream when supported.
- No goroutine leaks under any cancel/EOF scenario.

---

## 6. Phase 5 — Deep + Markdown + Sources + Login + Polish

**Goal**: Land `usearch deep` with full DEEP-004 quota error matrix;
`--format markdown`; `sources` subcommand; `login` placeholder; cobra
completion generators. Validate all NFRs.

### 6.1 Files

**Create**:

- `cmd/usearch/deep.go` — cobra command for `usearch deep`; reuses
  phase 4 stream consumer; adds DEEP-004 error parsing.
- `cmd/usearch/deep_test.go` — 6-case quota error matrix.
- `cmd/usearch/sources.go` — cobra command tree (`list`, `status`,
  `show`).
- `cmd/usearch/sources_test.go`.
- `cmd/usearch/login.go` — cobra command tree (`status`, `logout`,
  bare invocation).
- `cmd/usearch/login_test.go`.
- `cmd/usearch/output_markdown.go` — markdown renderer (text/template).
- `cmd/usearch/output_markdown_test.go`.
- `cmd/usearch/completion.go` — cobra completion subcommand
  (`bash`, `zsh`, `fish`, `powershell`).

**Modify**:

- `cmd/usearch/root.go` — register `deep`, `sources`, `login`,
  `completion`.
- `cmd/usearch/exitcode.go` — add exit code 4 (reserved for auth).

### 6.2 RED-GREEN-REFACTOR sequence

**RED 1**: `TestDeepSubcommandPostsToConfiguredEndpoint`,
`TestDeepHeadersFromConfig`, `TestDeepAllowDegradeFlag`.

**GREEN 1**: Implement `usearch deep`; reuse stream consumer.

**RED 2**: 6-case `TestDeepQuotaErrorMatrix`:
- 429 cap_exceeded → exit 2, human stderr, JSON stdout in JSON mode
- 400 deep_not_warranted → exit 1, suggest /basic
- 400 query_rejected_by_screen → exit 1
- 503 costguard_unavailable → exit 2
- 200 + X-Deep-Degraded → exit 0, warning stderr
- 200 clean → exit 0

**GREEN 2**: Parse each error shape and render appropriately.

**RED 3**: `TestFormatMarkdownShape`, `TestFormatMarkdownAlias`,
`TestFormatInvalidRejected`.

**GREEN 3**: Markdown renderer.

**RED 4**: `TestSourcesList`, `TestSourcesShowKnown`,
`TestSourcesShowUnknownExitsOne`.

**GREEN 4**: Sources subcommand via adapter registry walk.

**RED 5**: `TestLoginPlaceholderMessage`, `TestLoginStatusShowsConfigValues`,
`TestLoginLogoutClearsConfig`, `TestLoginLogoutDeletesTokenFile`.

**GREEN 5**: Login placeholder.

**RED 6**: `TestUnknownSubcommandSuggestsClose`,
`TestUnknownSubcommandDoesNotEnterREPL`.

**GREEN 6**: Verify cobra suggestion + REPL guard.

**RED 7**: NFR audits — `TestNoCredentialsInStderr`,
`TestNoCredentialsInHistory`, `TestErrorMessagesUnder200Chars`,
`TestBinarySizeWithinCap`.

**GREEN 7**: Address audit failures.

### 6.3 Exit gate

- All 14 REQs pass acceptance.
- All 5 NFRs verified.
- Coverage ≥ 85%.
- v0 backward compat suite passes unchanged.
- Binary size: default ≤ 30 MB; `-tags tui,history-sqlite` ≤ 45 MB.

---

## 7. Cross-Phase Concerns

### 7.1 MX Tag Plan

Per `.claude/rules/moai/workflow/mx-tag-protocol.md`. Tags added in
the run phase per file:

| File | Function | Tag | Reason |
|------|----------|-----|--------|
| `cmd/usearch/root.go` | `Execute` | @MX:ANCHOR | fan_in ≥ 3 (main, test, future subcommand additions); v1's stable boundary |
| `cmd/usearch/query.go` | `Execute` | @MX:ANCHOR | preserved from v0; v1 wrapped via cobra RunE |
| `cmd/usearch/deep.go` | `runDeep` | @MX:ANCHOR | new boundary for deep pipeline consumers |
| `cmd/usearch/stream.go` | `consumeSSE` | @MX:ANCHOR | streaming consumer used by query + deep |
| `cmd/usearch/stream.go` | `consumeSSE` | @MX:WARN | cancellation propagation across reader / renderer / heartbeat handler; goleak-sensitive |
| `internal/usearch/history/async.go` | `Drain` | @MX:WARN | graceful drain on shutdown; data loss risk if not awaited |
| `internal/usearch/config/config.go` | `Load` | @MX:NOTE | precedence order is the load-bearing contract — flag > env > file > default |
| `cmd/usearch/output_json.go` | `schemaVersion` | @MX:NOTE | constant `"1"` preserved from v0; bump on breaking changes |
| `cmd/usearch/exitcode.go` | exit constants | @MX:NOTE | exit-code mapping is the UX contract; CI scripts depend on it |

All @MX descriptions in English (per language.yaml `code_comments: en`).

### 7.2 Test Infrastructure

- `cmd/usearch/testmain_test.go` — new `TestMain(m *testing.M)` invoking
  `goleak.VerifyTestMain(m)` (NFR-CLI2-003).
- `testdata/sse/` — pre-recorded SSE wire fragments for stream consumer
  tests.
- `testdata/quota-errors/` — pre-recorded JSON bodies for each DEEP-004
  error response shape.
- `testdata/config/` — sample TOML configs (valid + malformed) for
  config loader tests.

### 7.3 CI Integration

Per phase, CI extends with:

- Phase 1: cobra binary size measurement step.
- Phase 2: SQLite-tagged build job alongside default.
- Phase 3: TUI-tagged build job.
- Phase 4: SSE consumer race test (`go test -race`).
- Phase 5: full audit suite (NFR-CLI2-001 backward compat,
  NFR-CLI2-005 credential leak).

### 7.4 Documentation Updates

Phase 5 also produces (handled by `/moai sync SPEC-CLI-002`):

- `README.md` — add CLI v1 quickstart section.
- `docs/cli.md` (new) — per-subcommand reference.
- `docs/config.md` (new) — config schema reference.
- `CHANGELOG.md` — v1 entry.

(These are sync-phase outputs; not in this plan's run scope.)

---

## 8. Risks (cross-references research.md §11)

Risks already enumerated in spec.md §8 and research.md §11. Plan-level
mitigations:

| Risk | Plan-level mitigation |
|------|----------------------|
| R1 binary size regression | CI gate at each phase; build tags `tui` and `history-sqlite` opt-in |
| R2 cobra migration breaks v0 | Phase 1 backward-compat suite is the gate to Phase 2 |
| R3 SSE consumer drops events | stdlib bufio.Scanner; explicit EOF → exit 2 path |
| R4 REPL conflicts with subcommands | REQ-CLI2-014 + `TestUnknownSubcommandDoesNotEnterREPL` |
| R7 credentials leak | NFR-CLI2-005 audit test in phase 5; mode 0600 enforced in phase 1 (config.go) |
| R12 piped invocation breaks | NFR-CLI2-001 v0 suite includes piped scenarios |

---

## 9. Open Questions Resolution Plan

The 18 open questions from research.md §12 + 11 questions in spec.md
§6 are resolved as follows:

- **Phase 1 (cobra, koanf, config schema)**: Q1, Q2, Q16 resolved by
  shipping the recommendation; adjust if annotation cycle dissents.
- **Phase 2 (history default backend, storage location, retention,
  search ranking)**: Q4, Q5, Q14 — ship JSONL default; SQLite under
  build tag.
- **Phase 3 (TUI default, REPL interaction model, multi-line input,
  config wizard library)**: Q3, Q10, Q11, Q12 — TUI behind build tag;
  slash-command model; plain prompts for `config init` (huh deferred).
- **Phase 4 (`--stream` default)**: Q6 — auto-detect TTY.
- **Phase 5 (`deep --force`, markdown scope, format aliases, NFR cap,
  `sources status`, exit code 4, `login` message, brand)**: Q7, Q8,
  Q9, Q13, Q15, Q17, Q18 + spec.md §6 brand item — resolved per
  spec.md recommendations; flagged for annotation cycle if user
  disagrees.

The annotation cycle will iterate over these decisions before the
run phase starts. Any decision reversal triggers a plan revision
within the same annotation cycle (no fresh SPEC needed).

---

## 10. Run-Phase Entry Conditions

Before `/moai run SPEC-CLI-002` begins:

- [ ] plan-auditor PASS on this plan + spec.md + acceptance.md.
- [ ] Annotation cycle complete; user has explicitly confirmed
      "Proceed" (per spec-workflow.md Plan → Run transition).
- [ ] SPEC-IR-001 server exposes `/synthesize` and `/deep` endpoints
      (already implemented).
- [ ] SPEC-SYN-004 SSE handler responding with documented wire format
      (already implemented).
- [ ] SPEC-DEEP-004 cost guard responding with documented error
      shapes (already implemented).
- [ ] `/clear` executed to free Plan-phase tokens (~30K → 0).

Run phase budget: 180K tokens (per spec-workflow.md). Five phases
expected to consume ~120K cumulative with leaks bounded by goleak
gates.

---

*End of SPEC-CLI-002 plan v0.1 (draft).*
