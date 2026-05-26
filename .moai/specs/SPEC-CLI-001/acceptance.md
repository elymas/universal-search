# SPEC-CLI-001 Acceptance — Given/When/Then Scenarios

Created: 2026-04-28
Updated: 2026-05-04 (post-implementation reconciliation)
Author: limbowl (via manager-spec)
Status: implemented

## 0. Document Purpose

Given/When/Then acceptance scenarios for SPEC-CLI-001 — the `usearch
query` subcommand v0. Each scenario maps to one or more EARS REQs in
spec.md §3. Each scenario is directly executable as a Go test case in
the run phase.

## 1. Coverage Matrix

| AC | Scenario | REQs covered |
|----|----------|--------------|
| AC-001 | Subcommand dispatch — query vs version vs unknown | REQ-CLI-001 |
| AC-002 | Flag parsing — positional + flags via FlagSet | REQ-CLI-002 |
| AC-003 | `--source` filter intersects router decision | REQ-CLI-003 |
| AC-004 | `--format json` schema versioned object | REQ-CLI-004 |
| AC-005 | `--format text` summary + numbered citations | REQ-CLI-004 |
| AC-006 | `--timeout` cancels fanout + leaves no goroutine | REQ-CLI-005 |
| AC-007 | stdout exclusive to payload; stderr to progress | REQ-CLI-006 |
| AC-008 | Empty prompt rejected without any side effect | REQ-CLI-007 |
| AC-009 | Exit code matrix (0 / 1 / 2 / 3) | REQ-CLI-008 |
| AC-010 | Synthesis nopclient soft-fail → exit 3 | REQ-CLI-009 |
| AC-011 | Root OTel span with attributes + parent of router | REQ-CLI-010 |
| AC-012 | Request ID surfaced in JSON + slog | REQ-CLI-011 |
| NFR-001 | End-to-end buildability (integration test) | NFR-CLI-001 |
| NFR-002 | Goroutine hygiene (goleak clean) | NFR-CLI-002 |
| NFR-003 | Binary size ≤ 30 MB | NFR-CLI-003 |
| NFR-004 | Human-readable errors (no stack trace in text) | NFR-CLI-004 |

## 2. Definition of Done

- [x] All 11 EARS REQs (8 P0 + 3 P1) have at least one green test.
- [x] All 4 NFRs validated.
- [x] `cmd/usearch/usearch query "hello"` exits ∈ {0, 3} end-to-end
      with at least one registered adapter.
- [x] `goleak.VerifyTestMain` clean across all test cases.
- [x] Binary size ≤ 30 MB (release build with `-ldflags "-s -w"
      -trimpath`).
- [x] Coverage ≥ 80% in `cmd/usearch/`.
- [x] TRUST 5 gates green.
- [x] Forward dependencies confirmed: SPEC-ADP-002 + SPEC-SYN-001
      implemented or nopclient soft-fail path verified.

## 3. Functional Scenarios

### AC-001 — Subcommand dispatch

Maps to REQ-CLI-001.

#### AC-001.1: unknown subcommand → exit 2

- **Given** the `usearch` binary.
- **When** the user runs `usearch foobar`.
- **Then** exit code 2; stderr contains `"unknown subcommand"` and
  `"available: query, --version"`.

#### AC-001.2: `--version` preserved

- **When** the user runs `usearch --version` or `usearch -v`.
- **Then** the existing BOOT-001 version handler runs; output matches
  `^usearch v\d+\.\d+\.\d+`; exit 0.

#### AC-001.3: `query` dispatched

- **When** the user runs `usearch query "test"` with mocked
  router/adapter/synthesis.
- **Then** `runQuery` is invoked; not `runVersion`.

### AC-002 — Flag parsing

Maps to REQ-CLI-002.

#### AC-002.1: positional + flags parsed

- **When** the user runs `query --format json --timeout 10s --source
  reddit "hello"`.
- **Then** parsed `queryFlags{Source: ["reddit"], Format: "json",
  Timeout: 10*time.Second}`; prompt == "hello".

#### AC-002.2: zero positional → exit 1

- **When** the user runs `query --format json`.
- **Then** exit 1; stderr contains `"prompt argument required"`.

#### AC-002.3: two positionals → exit 1

- **When** the user runs `query "a" "b"`.
- **Then** exit 1; stderr contains `"exactly one positional argument
  expected"`.

### AC-003 — `--source` filter

Maps to REQ-CLI-003.

#### AC-003.1: filter narrows router set

- **Given** registry has reddit + hackernews; router returns
  `AdapterSet=["hackernews", "reddit"]`.
- **When** the user runs `query --source reddit "x"`.
- **Then** only reddit was called.

#### AC-003.2: empty `--source` means all

- **When** the user runs `query --source '' "x"`.
- **Then** both reddit and hackernews were called.

#### AC-003.3: filter intersects router exclusion

- **Given** router returns `AdapterSet=["reddit"]` (HN excluded by lang).
- **When** the user runs `query --source hackernews "x"`.
- **Then** effective set is empty; CLI prints `"no adapters matched"`
  warning; exit 2.

#### AC-003.4: unknown adapter → exit 1

- **When** the user runs `query --source nosuchadapter "x"`.
- **Then** exit 1; stderr contains `"unknown adapter
  'nosuchadapter'"`.

### AC-004 — `--format` flag

Maps to REQ-CLI-004.

#### AC-004.1: JSON shape

- **When** the user runs `query --format json "test"`.
- **Then** stdout is a single valid JSON object with top-level keys
  exactly `{schema_version, query, category, lang, adapters, summary,
  citations, stats}`; `schema_version == "1"`.

#### AC-004.2: text shape

- **When** the user runs `query --format text "test"`.
- **Then** stdout starts with the summary paragraph; followed by blank
  line; then `"Citations:"`; then numbered `[N] <Title> — <URL>` lines.

#### AC-004.3: invalid format → exit 1

- **When** the user runs `query --format yaml "x"`.
- **Then** exit 1; stderr contains `"unsupported format 'yaml'; valid:
  text, json"`.

#### AC-004.4: default is text

- **When** the user runs `query "x"` (no `--format`).
- **Then** stdout is text-mode output identical to `--format text`.

### AC-005 — Timeout

Maps to REQ-CLI-005.

#### AC-005.1: timeout cancels fanout

- **Given** a stub adapter that blocks for 60 s.
- **When** the user runs `query --timeout 100ms "x"`.
- **Then** total elapsed ≤ 300 ms; exit 2; stderr contains `"timeout:
  fanout stage"`.

#### AC-005.2: timeout leaves no goroutine

- **Same scenario as AC-005.1**.
- **Then** `goleak.VerifyNone(t)` reports zero leaks.

#### AC-005.3: timeout > 5m rejected

- **When** the user runs `query --timeout 10m "x"`.
- **Then** exit 1; stderr contains `"--timeout exceeds maximum
  5m0s"`.

#### AC-005.4: default timeout is 30 s

- **When** the user runs `query "x"` (no `--timeout`).
- **Then** parsed value == `30 * time.Second`.

### AC-006 — stdout/stderr separation

Maps to REQ-CLI-006.

#### AC-006.1: stdout payload only

- **When** the user runs `query --format json "x"` with a working
  pipeline.
- **Then** stdout is parseable as a SINGLE JSON object with no
  preamble.
- **And** stderr contains progress lines and slog records but is not
  itself JSON-by-itself.

#### AC-006.2: stderr contains progress markers (text mode)

- **When** the user runs `query "x"` (text mode).
- **Then** stderr contains `"[router] classified"`, `"[fanout]"`,
  `"[synthesis]"` markers.

#### AC-006.3: stderr names failure stage

- **When** the user runs a path that fails.
- **Then** stdout is empty (or contains partial-result payload only);
  stderr names the failing stage.

### AC-007 — Empty prompt rejection

Maps to REQ-CLI-007.

#### AC-007.1: empty string → exit 1

- **When** the user runs `query ""`.
- **Then** exit 1; stderr contains `"usearch query: prompt argument
  required"`.

#### AC-007.2: whitespace → exit 1

- **When** the user runs `query "   "` or `query "\t\n"`.
- **Then** exit 1 with the same message.

#### AC-007.3: no router/adapter call on empty prompt

- **And** the mock router's `Classify` was called zero times; the
  adapter registry was not consulted.

### AC-008 — Exit codes

Maps to REQ-CLI-008.

| Scenario | Adapter result | Synthesis result | Expected exit |
|----------|----------------|------------------|---------------|
| AC-008.1 full success | ≥1 doc | non-empty summary | 0 |
| AC-008.2 synthesis failure | ≥1 doc | error | 3 (partial; degraded output) |
| AC-008.3 partial adapter failure | some docs + some errors | success | 3 |
| AC-008.4 all adapters fail | all error | n/a | 2 |

### AC-009 — Synthesis nopclient soft-fail

Maps to REQ-CLI-009.

- **Given** `internal/synthesis/nopclient.Client` is registered (i.e.,
  SPEC-SYN-001 not yet implemented OR `LITELLM_MASTER_KEY` unset).
- **When** the user runs `query "x"` with at least one adapter
  returning docs.
- **Then** exit 3; stdout contains the raw `NormalizedDoc.Snippet`
  list with manual numbering; stderr contains `"[synthesis:
  unavailable]"`.

### AC-010 — Root OTel span

Maps to REQ-CLI-010.

- **Given** an in-memory OTel exporter.
- **When** any `query` invocation runs.
- **Then** a single root span named `usearch.cli.query` is captured
  with attributes:
  - `cli.prompt_length` (int)
  - `cli.format` (string: "text" | "json")
  - `cli.source_filter_count` (int)
  - `cli.adapter_set` (string, comma-joined)
  - `cli.exit_code` (int)
- **And** the captured `router.classify` span has parent ID matching
  the root span.
- **And** `span.End()` is called before `os.Exit` (verified via mock
  exit function).

### AC-011 — Request ID

Maps to REQ-CLI-011.

- **When** the user runs `query --format json "x"`.
- **Then** parsed stdout JSON has `stats.request_id` as a valid 26-char
  Crockford Base32 ULID.
- **And** every stderr JSON slog line carries the SAME `request_id`
  attribute value.

## 4. Non-Functional Acceptance

### NFR-CLI-001 — End-to-end buildability

- Integration test under `// +build integration` tag at
  `cmd/usearch/integration_test.go`:
  1. Spins up stub HTTP servers for Reddit + HN + LiteLLM proxy.
  2. Builds binary: `go build -o /tmp/usearch ./cmd/usearch`.
  3. Invokes `/tmp/usearch query "hello world"` with stub URLs.
  4. Asserts exit ∈ {0, 3}; stdout non-empty.
- CI invocation: `go test -tags=integration ./cmd/usearch/...`.

### NFR-CLI-002 — Goroutine hygiene

- `cmd/usearch/query_test.go::TestMain` invokes
  `goleak.VerifyTestMain(m)`.
- 100 iterations of `BenchmarkExecute` show `runtime.NumGoroutine()`
  delta == 0 before / after.

### NFR-CLI-003 — Binary size

- CI step: `go build -ldflags "-s -w" -trimpath -o /tmp/usearch
  ./cmd/usearch && du -m /tmp/usearch | awk '{print $1}'`.
- Assert size ≤ 30; fail with `"binary size regressed: <N>MB > 30MB
  cap"` on violation.

### NFR-CLI-004 — Human-readable errors

- `TestErrorMessagesNoStackTrace`: for each error path, captured
  stderr line has no `"goroutine "`, no `"runtime.gopark"`, no
  leading `"\t"`.
- `TestErrorMessagesUnder200Chars`: each error line ≤ 200 chars.
- `TestDebugModeRevealsStackTraces`: with `LOG_LEVEL=DEBUG`, stderr
  DOES contain JSON slog DEBUG records on errors.

## 5. Edge Cases

### EC-001 — Both adapters fail with different errors

- Adapter A returns rate-limit; adapter B returns network error.
- `query` exits 2; stderr names the highest-priority failure
  (rate-limit) first.

### EC-002 — Router returns empty AdapterSet after LLM-fallback

- The CLI prints `"no adapters matched"` warning; exits 2.

### EC-003 — `--timeout` smaller than per-adapter min latency

- The fanout returns zero docs; exit 2.

### EC-004 — Synthesis returns empty summary string

- Treated as failure; exit 3; degraded output renders adapter snippets.

### EC-005 — Stdin piped (deferred to CLI-002)

- v0 accepts only positional prompt argument; piped stdin is ignored.

### EC-006 — Multiple positional arguments

- Exit 1 with `"exactly one positional argument expected"`.

### EC-007 — JSON output when `--no-llm` flag is set

- Router degrades gracefully when LLM client is nil; CLI still emits a
  valid JSON object with `category` and `lang` set to defaults
  (per SPEC-IR-001 fallback behaviour).

### EC-008 — Request ID format-agnostic

- Public `stats.request_id` field accepts ULID or any 26-char
  string; internal implementation uses ULID; downstream consumers
  must not parse the format.

## 6. Quality Gate Criteria

| Criterion | Threshold | Source |
|-----------|-----------|--------|
| Coverage (`cmd/usearch/`) | ≥ 80% | spec.md frontmatter |
| `go vet ./cmd/usearch/...` | clean | go.md |
| `golangci-lint run` | zero issues | go.md |
| `go test -race ./cmd/usearch/...` | clean | NFR-CLI-002 |
| `goleak.VerifyTestMain` | clean | NFR-CLI-002 |
| Binary size (release) | ≤ 30 MB | NFR-CLI-003 |
| End-to-end integration test | exit ∈ {0, 3} | NFR-CLI-001 |
| TRUST 5 gates | all green | constitution |

## 7. Out-of-Scope Confirmations

Restated from spec.md §2.2 (Exclusions):

- `usearch deep` subcommand → SPEC-CLI-002 + SPEC-DEEP-*
- `usearch team list` / `team members` / `team add` → SPEC-CLI-002
  + SPEC-AUTH-002
- TUI / bubbletea mode → SPEC-CLI-002
- Stdin-piped input → SPEC-CLI-002
- Multiple positional prompts → out of scope
- Tag-based `--source` filtering → SPEC-CLI-002
- Output formats beyond text/json → SPEC-CLI-002
- True LLM-streaming output → SPEC-SYN-004 (M4)
- Full SPEC-FAN-001 fanout features → SPEC-FAN-001 (M3)
- Cobra adoption → SPEC-CLI-002
- Shell completion scripts → SPEC-CLI-002
- Configuration file support → SPEC-CLI-002
- Per-source rate-limit display → SPEC-EVAL-002 (M8)

---

*End of acceptance.md (post-hoc).*
