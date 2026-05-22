# Acceptance — SPEC-CLI-002: `usearch` CLI v1

Date: 2026-05-22
Author: limbowl (via manager-spec)
Status: draft

This document enumerates the Given-When-Then acceptance scenarios for
SPEC-CLI-002 v0.1. Each scenario is mapped to one or more EARS
requirements in `spec.md` §3. Edge cases are surfaced explicitly.

The intent is that this document is **directly executable as a test
plan** in the run phase. Each scenario corresponds to one or more
Go tests named in `spec.md` §3 "Acceptance Summary" column.

---

## 1. Scenario Index

| § | Scenario | REQs covered |
|---|----------|--------------|
| 3.1 | Cobra-migrated `query` preserves v0 invocation | REQ-CLI2-001, NFR-CLI2-001 |
| 3.2 | Subcommand registry includes all v1 surfaces | REQ-CLI2-002 |
| 3.3 | `usearch deep` happy path with streaming | REQ-CLI2-003, REQ-CLI2-005 |
| 3.4 | `usearch deep` non-streaming JSON | REQ-CLI2-003, REQ-CLI2-005 |
| 3.5 | DEEP-004 quota error matrix (6 cases) | REQ-CLI2-004 |
| 3.6 | Output format `markdown` shape | REQ-CLI2-006 |
| 3.7 | Config file precedence + XDG resolution | REQ-CLI2-007, REQ-CLI2-012 |
| 3.8 | Interactive REPL — zero args entry + slash commands | REQ-CLI2-008 |
| 3.9 | `usearch sources list / show` | REQ-CLI2-009 |
| 3.10 | History write + list + show + search + clear | REQ-CLI2-010, REQ-CLI2-011 |
| 3.11 | `usearch config` subcommand tree | REQ-CLI2-012 |
| 3.12 | `usearch login` placeholder | REQ-CLI2-013 |
| 3.13 | Unknown subcommand handling | REQ-CLI2-014 |
| 3.14 | Streaming flag auto-detect and override | REQ-CLI2-005 |
| 4 | Non-functional acceptance | NFRs |
| 5 | Edge cases | various |

---

## 2. Definition of Done (per SPEC)

- [ ] All 14 EARS REQs have at least one acceptance scenario below
      with a green Go test.
- [ ] All 5 NFRs validated in §4 with explicit measurement evidence.
- [ ] All edge cases in §5 have either a green test or an explicit
      documented rationale for deferral.
- [ ] v0 backward compat suite passes byte-equivalent against the v1
      binary (NFR-CLI2-001).
- [ ] Binary size: default build ≤ 30 MB, full tags ≤ 45 MB
      (NFR-CLI2-002).
- [ ] Coverage ≥ 85%.
- [ ] `goleak.VerifyTestMain` passes (NFR-CLI2-003).
- [ ] No credentials in stderr/stdout/history (NFR-CLI2-005 audit).
- [ ] TRUST 5 gates green (Tested, Readable, Unified, Secured,
      Trackable).
- [ ] Pre-submission self-review per workflow-modes.md.

---

## 3. Functional Scenarios

### 3.1 Cobra-migrated `query` preserves v0 invocation

**Mapping**: REQ-CLI2-001, NFR-CLI2-001

#### Scenario 3.1.1: v0 happy path

- **Given** a v1 binary built with `go build ./cmd/usearch` and a
  registered adapter (Reddit stub returning 3 docs)
- **When** the user runs `usearch query "hello world"`
- **Then** the binary exits with code 0
- **And** stdout contains the synthesized summary followed by
  `Citations:\n[1] ... — http...`
- **And** stderr contains structured progress markers (`[router]`,
  `[fanout]`, `[synthesis]`)
- **And** the byte sequence of stdout is identical to what v0 produced
  for the same input

#### Scenario 3.1.2: v0 `--format json` preserved

- **Given** the same setup as 3.1.1
- **When** the user runs `usearch query --format json "hello world"`
- **Then** exit code 0
- **And** stdout is a single valid JSON object
- **And** `schema_version == "1"` (preserved from v0)
- **And** keys exactly `{schema_version, query, category, lang,
  adapters, summary, citations, stats}`

#### Scenario 3.1.3: v0 `--version` preserved

- **Given** the v1 binary
- **When** the user runs `usearch --version` or `usearch -v`
- **Then** exit code 0, stdout `usearch v<semver>\n`

#### Scenario 3.1.4: v0 `--timeout` preserved

- **Given** an adapter that blocks for 60s
- **When** the user runs `usearch query --timeout 100ms "x"`
- **Then** exit code 2, stderr contains `timeout: fanout stage`,
  elapsed wall-clock ≤ 300 ms

### 3.2 Subcommand registry includes all v1 surfaces

**Mapping**: REQ-CLI2-002

#### Scenario 3.2.1: `--help` enumerates subcommands

- **Given** the v1 binary
- **When** the user runs `usearch --help`
- **Then** stdout contains lines for each of: `query`, `deep`,
  `sources`, `history`, `config`, `login`, `completion`
- **And** the version line is also present
- **And** exit code 0

#### Scenario 3.2.2: Per-subcommand `--help`

- **Given** the v1 binary
- **When** the user runs `usearch <subcommand> --help` for each
  registered subcommand
- **Then** stdout contains the subcommand name, a one-line synopsis,
  the supported flags with descriptions, the exit code documentation,
  and at least one example invocation
- **And** exit code 0

### 3.3 `usearch deep` happy path with streaming

**Mapping**: REQ-CLI2-003, REQ-CLI2-005

#### Scenario 3.3.1: deep streamed render

- **Given** a `usearch-api` stub responding to `POST /deep` with
  `Accept: text/event-stream` by emitting 4 `event: sentence` events
  followed by `event: done`
- **And** `[server] endpoint = "http://localhost:8080"` in config
- **When** the user runs `usearch deep "complex research question"`
  in a TTY
- **Then** stdout receives 4 sentence segments progressively (verified
  by capturing intermediate stdout state at 250 ms intervals)
- **And** the final stdout contains all 4 sentences joined into a
  paragraph followed by a citation list
- **And** stderr contains stats `(latency_ms, model, cost_usd)` after
  `event: done`
- **And** exit code 0

#### Scenario 3.3.2: deep headers from config

- **Given** config with `[auth] user_id = "alice"`, `tenant_id = "team-1"`
- **When** the user runs `usearch deep "x"`
- **Then** the stub receives the request with headers
  `X-User-Id: alice`, `X-Tenant-Id: team-1`
- **And** no other auth-related headers are present

#### Scenario 3.3.3: deep `--allow-degrade` flag

- **Given** config with `[deep] allow_degrade = false`
- **When** the user runs `usearch deep --allow-degrade "x"`
- **Then** the stub receives `X-Allow-Degrade: 1` header

### 3.4 `usearch deep` non-streaming JSON

**Mapping**: REQ-CLI2-003, REQ-CLI2-005

#### Scenario 3.4.1: piped invocation defaults to JSON

- **Given** a `usearch-api` stub
- **When** the user runs `usearch deep "x" | jq .summary` (i.e.
  stdout is piped, not a TTY)
- **Then** the stub receives request with `Accept: application/json`
  (not `text/event-stream`)
- **And** stdout is a single valid JSON object parseable by `jq`
- **And** exit code 0

#### Scenario 3.4.2: explicit `--no-stream` over TTY

- **Given** the same stub
- **When** the user runs `usearch deep --no-stream "x"` in a TTY
- **Then** Accept header is `application/json`
- **And** stdout is buffered JSON
- **And** exit code 0

### 3.5 DEEP-004 quota error matrix

**Mapping**: REQ-CLI2-004

#### Scenario 3.5.1: 429 cap_exceeded (calls dimension)

- **Given** `/deep` returns HTTP 429 with header
  `Retry-After: 3600` and body
  `{"error":"cap_exceeded","dimension":"calls",
  "remaining":{"calls":0,"usd":2.50},"reset_at":"2026-05-23T00:00:00Z"}`
- **When** the user runs `usearch deep "x"` (default `--format
  human`)
- **Then** stderr contains a single line matching
  `usearch deep: daily limit reached (calls); resets at
  2026-05-23T00:00:00Z`
- **And** exit code 2
- **And** stdout is empty in human mode

#### Scenario 3.5.2: 429 cap_exceeded with `--format json`

- **Given** the same 429 response
- **When** the user runs `usearch deep --format json "x"`
- **Then** stdout contains the original error JSON body verbatim
  (so `jq` consumers can parse it)
- **And** stderr contains the human-readable line
- **And** exit code 2

#### Scenario 3.5.3: 400 deep_not_warranted

- **Given** `/deep` returns HTTP 400 body
  `{"error":"deep_not_warranted","suggested_mode":"basic",
  "screen_score":5,"rationale":"single-fact query, /basic suffices"}`
- **When** the user runs `usearch deep "what is the capital of France?"`
- **Then** stderr contains
  `usearch deep: pre-screen suggests /basic mode (score 5/10):
  single-fact query, /basic suffices. Try 'usearch query' instead.`
- **And** exit code 1
- **And** stdout is empty in human mode

#### Scenario 3.5.4: 400 query_rejected_by_screen

- **Given** `/deep` returns HTTP 400 body
  `{"error":"query_rejected_by_screen","screen_score":2,
  "rationale":"query is incoherent"}`
- **When** the user runs `usearch deep "..."`
- **Then** stderr contains the rationale
- **And** exit code 1

#### Scenario 3.5.5: 503 costguard_unavailable

- **Given** `/deep` returns HTTP 503 body
  `{"error":"costguard_unavailable","detail":"redis unreachable"}`
- **When** the user runs `usearch deep "x"`
- **Then** stderr contains
  `usearch deep: cost guard unavailable; retry later or use
  'usearch query'.`
- **And** exit code 2

#### Scenario 3.5.6: 200 with X-Deep-Degraded fallback

- **Given** `/deep` returns HTTP 200 with header
  `X-Deep-Degraded: cap-exceeded` and a normal `/basic`-shaped
  response body
- **When** the user runs `usearch deep --allow-degrade "x"`
- **Then** stderr contains
  `usearch deep: warning — cap exceeded, fell back to /basic mode`
- **And** stdout contains the normal response payload
- **And** exit code 0

### 3.6 Output format `markdown` shape

**Mapping**: REQ-CLI2-006

#### Scenario 3.6.1: markdown happy path

- **Given** a successful query with 3 citations
- **When** the user runs `usearch query --format markdown "x"`
- **Then** stdout begins with `# Query: x\n\n`
- **And** contains the summary paragraph with inline `[^1]`, `[^2]`,
  `[^3]` markers
- **And** contains a `## Sources\n\n` section followed by 3 lines
  matching `[^N]: [<title>](<url>) — <source>`
- **And** ends with `---\nGenerated by usearch v<semver> | <model> |
  $<cost> | <latency>ms\n`
- **And** exit code 0

#### Scenario 3.6.2: `md` alias

- **Given** the same setup
- **When** the user runs `usearch query --format md "x"`
- **Then** stdout is byte-identical to the `--format markdown` output

#### Scenario 3.6.3: invalid format rejected

- **When** the user runs `usearch query --format yaml "x"`
- **Then** exit code 1, stderr contains
  `unsupported format 'yaml'; valid: human, text, json, markdown, md`

### 3.7 Config file precedence + XDG resolution

**Mapping**: REQ-CLI2-007, REQ-CLI2-012

#### Scenario 3.7.1: `config path` resolves XDG_CONFIG_HOME

- **Given** `XDG_CONFIG_HOME=/tmp/cfg` env var
- **When** the user runs `usearch config path`
- **Then** stdout is `/tmp/cfg/usearch/config.toml\n`
- **And** exit code 0

#### Scenario 3.7.2: `config init` creates file with defaults

- **Given** `/tmp/cfg/usearch/config.toml` does not exist
- **When** the user runs `usearch config init` (non-interactive,
  `--non-interactive` flag)
- **Then** the file is created with default TOML schema (all sections
  present)
- **And** mode 0644 (or 0600 for any `[auth] token_file` references)
- **And** exit code 0

#### Scenario 3.7.3: precedence — flag > env > file > default

- **Given** `~/.config/usearch/config.toml` sets
  `[server] endpoint = "http://from-file:8080"`
- **And** env `USEARCH_SERVER_ENDPOINT=http://from-env:8081`
- **When** the user runs `usearch config show`
- **Then** the effective `server.endpoint` value is
  `http://from-env:8081` (env wins over file)
- **When** the user runs `usearch query --server http://from-flag:8082 "x"`
- **Then** the resolved endpoint is `http://from-flag:8082` (flag
  wins over env)

#### Scenario 3.7.4: missing config file tolerated

- **Given** no config file exists
- **When** the user runs `usearch config show`
- **Then** all default values are emitted as TOML
- **And** exit code 0

#### Scenario 3.7.5: malformed config rejected

- **Given** `~/.config/usearch/config.toml` contains invalid TOML
  (e.g. unclosed quote)
- **When** the user runs any command
- **Then** exit code 1, stderr contains
  `usearch: config parse error at ~/.config/usearch/config.toml:<line>:<col>`

### 3.8 Interactive REPL — zero args entry + slash commands

**Mapping**: REQ-CLI2-008

#### Scenario 3.8.1: REPL entry on zero args + TTY

- **Given** stdin is a TTY (mocked via `pty`)
- **When** the user runs `usearch` (no arguments)
- **Then** stdout shows banner `Welcome to usearch v<semver>\nType
  /help or /exit\n`
- **And** prompt `usearch> ` is emitted

#### Scenario 3.8.2: REPL skipped when stdin piped

- **Given** stdin is piped (not a TTY)
- **When** the user runs `usearch`
- **Then** stdout contains `--help`-equivalent text
- **And** exit code 0
- **And** no `usearch> ` prompt

#### Scenario 3.8.3: REPL basic query loop

- **Given** REPL active with mocked input
- **When** the user types `hello world\n`
- **Then** the REPL invokes `query` with the line as prompt
- **And** the response is rendered in the user's `--format` choice

#### Scenario 3.8.4: `/exit` slash command

- **Given** REPL active
- **When** the user types `/exit\n`
- **Then** the REPL exits cleanly within 50 ms
- **And** exit code 0

#### Scenario 3.8.5: `/help` slash command

- **Given** REPL active
- **When** the user types `/help\n`
- **Then** stdout lists available slash commands

#### Scenario 3.8.6: REPL query persisted to history

- **Given** REPL active with history enabled
- **When** the user submits a query then `/exit`
- **Then** the history backend contains an entry for the query

#### Scenario 3.8.7 (under `-tags tui`): TUI streaming render

- **Given** the `tui`-tagged binary, TTY stdin, REPL active
- **When** the user submits a query that streams 4 sentences
- **Then** the bubbletea viewport progressively renders each sentence
  (verified via TUI snapshot test)

### 3.9 `usearch sources list / show`

**Mapping**: REQ-CLI2-009

#### Scenario 3.9.1: `sources list` enumerates registry

- **Given** Reddit and HN adapters registered
- **When** the user runs `usearch sources list`
- **Then** stdout contains exactly 2 lines, each matching
  `<name>\t<category>\t<lang>\t<auth_required:y|n>`
- **And** exit code 0

#### Scenario 3.9.2: `sources list --format json`

- **Given** the same setup
- **When** the user runs `usearch sources list --format json`
- **Then** stdout is a JSON array of 2 objects
- **And** keys exactly `{name, category, lang, auth_required}`

#### Scenario 3.9.3: `sources show <unknown>` exits 1

- **When** the user runs `usearch sources show nosuchadapter`
- **Then** exit code 1, stderr `usearch sources: unknown adapter
  'nosuchadapter'`

### 3.10 History write + list + show + search + clear

**Mapping**: REQ-CLI2-010, REQ-CLI2-011

#### Scenario 3.10.1: history write after successful query

- **Given** history enabled (JSONL backend), empty history file
- **When** the user runs `usearch query "test"` (success)
- **Then** within 200 ms, the history file contains exactly 1 JSON
  line with all required Entry fields
- **And** `schema_version == 1`

#### Scenario 3.10.2: history write async non-blocking

- **Given** history backend simulated as slow (write delays 500 ms)
- **When** the user runs `usearch query "test"`
- **Then** CLI exits within 200 ms (history write awaited up to 100
  ms grace)
- **And** stderr contains a WARN log `history write timed out;
  entry abandoned` if write was abandoned

#### Scenario 3.10.3: `history list` reverse-chronological

- **Given** 5 history entries (timestamps t0, t1, t2, t3, t4)
- **When** the user runs `usearch history list`
- **Then** stdout lists entries in order t4, t3, t2, t1, t0
- **And** exit code 0

#### Scenario 3.10.4: `history list --limit 3`

- **Given** the same 5 entries
- **When** the user runs `usearch history list --limit 3`
- **Then** stdout lists only the 3 most recent

#### Scenario 3.10.5: `history list --since 24h`

- **Given** entries with various timestamps
- **When** the user runs `usearch history list --since 24h`
- **Then** stdout includes only entries with timestamp within the
  last 24 hours

#### Scenario 3.10.6: `history show <id>`

- **Given** a known entry id `01JF...`
- **When** the user runs `usearch history show 01JF...`
- **Then** stdout is the full entry in the user's `--format` choice
- **And** exit code 0

#### Scenario 3.10.7: `history show <unknown>` exits 1

- **When** the user runs `usearch history show notanid`
- **Then** exit code 1, stderr `usearch history: entry 'notanid' not found`

#### Scenario 3.10.8: `history search` substring (JSONL)

- **Given** entries with prompts including "machine learning"
- **When** the user runs `usearch history search "machine"`
- **Then** stdout lists only entries whose prompt contains
  "machine" (case-insensitive)

#### Scenario 3.10.9: `history clear --confirm`

- **Given** non-empty history
- **When** the user runs `usearch history clear --confirm`
- **Then** history file is truncated
- **And** exit code 0

#### Scenario 3.10.10: `history clear` without `--confirm` in non-TTY

- **Given** stdin piped
- **When** the user runs `usearch history clear`
- **Then** exit code 1, stderr `usearch history clear: --confirm
  flag required in non-interactive mode`
- **And** history file is unchanged

#### Scenario 3.10.11: FIFO eviction at max_entries

- **Given** `[history] max_entries = 3` and 3 existing entries
- **When** the user runs a 4th successful query
- **Then** the history file contains exactly 3 entries (oldest
  evicted)

#### Scenario 3.10.12 (under `-tags history-sqlite`): SQLite + FTS5

- **Given** SQLite backend, 10 entries with various content
- **When** the user runs `usearch history search "neural network"`
- **Then** results are ranked by FTS5 relevance, not chronologically

### 3.11 `usearch config` subcommand tree

**Mapping**: REQ-CLI2-012

#### Scenario 3.11.1: `config show` after `init`

- See §3.7.4 above.

#### Scenario 3.11.2: `config get` known key

- **Given** config with `[server] endpoint = "http://x:8080"`
- **When** the user runs `usearch config get server.endpoint`
- **Then** stdout `http://x:8080\n`, exit 0

#### Scenario 3.11.3: `config get` unknown key

- **When** the user runs `usearch config get nosuch.key`
- **Then** exit 1, stderr `usearch config: unknown key 'nosuch.key'`

#### Scenario 3.11.4: `config set` writes file

- **Given** a config file
- **When** the user runs `usearch config set server.endpoint
  http://new:9090`
- **Then** the file is updated; running `config get server.endpoint`
  returns `http://new:9090`

#### Scenario 3.11.5: `config set` refuses token key

- **When** the user runs `usearch config set auth.token sk_xxx`
- **Then** exit 1, stderr
  `usearch config: refusing to write 'auth.token' to config.toml;
  use the credentials file at ~/.config/usearch/credentials instead`

### 3.12 `usearch login` placeholder

**Mapping**: REQ-CLI2-013

#### Scenario 3.12.1: `login` bare invocation pre-M6

- **Given** SPEC-AUTH-001 not yet shipped (no `[auth] oidc_endpoint`)
- **When** the user runs `usearch login`
- **Then** stderr contains
  `usearch login: OIDC auth not yet enabled. Set [auth] user_id and
  [auth] tenant_id in your config with 'usearch config set'.`
- **And** exit code 1

#### Scenario 3.12.2: `login status`

- **Given** config with `[auth] user_id = "alice"`, `tenant_id = "team-1"`
- **When** the user runs `usearch login status`
- **Then** stdout contains both values; exit 0
- **Given** config with `[auth]` empty
- **When** the user runs `usearch login status`
- **Then** stdout shows `<not set>` for both; exit 0

#### Scenario 3.12.3: `login logout` clears values

- **Given** config with `[auth] user_id = "alice"`, `token_file =
  "~/.config/usearch/credentials"` (which exists with mode 0600)
- **When** the user runs `usearch login logout`
- **Then** config `[auth] user_id` and `tenant_id` are cleared
- **And** the credentials file is deleted
- **And** exit code 0

### 3.13 Unknown subcommand handling

**Mapping**: REQ-CLI2-014

#### Scenario 3.13.1: typo with close suggestion

- **When** the user runs `usearch queery "x"` (typo)
- **Then** stderr contains `Error: unknown command "queery"` and
  `Did you mean "query"?`
- **And** exit code 1

#### Scenario 3.13.2: completely unknown

- **When** the user runs `usearch wibble`
- **Then** stderr contains `Error: unknown command "wibble"` (no
  suggestion because no near match)
- **And** exit code 1
- **And** REPL is NOT entered (even on TTY)

### 3.14 Streaming flag auto-detect and override

**Mapping**: REQ-CLI2-005

#### Scenario 3.14.1: TTY auto-stream

- **Given** TTY stdout, SSE-capable endpoint
- **When** the user runs `usearch query "x"` (no `--stream` flag)
- **Then** request Accept header is `text/event-stream`
- **And** rendering is incremental

#### Scenario 3.14.2: Piped auto-buffer

- **Given** piped stdout
- **When** the user runs `usearch query "x"` (no `--stream` flag)
- **Then** request Accept header is `application/json`
- **And** stdout is byte-equivalent to v0 buffered JSON

#### Scenario 3.14.3: explicit override

- **Given** TTY stdout
- **When** the user runs `usearch query --no-stream "x"`
- **Then** request Accept header is `application/json`

---

## 4. Non-Functional Acceptance

### 4.1 NFR-CLI2-001 — v0 backward compatibility

- All test files from `cmd/usearch/query_test.go` and
  `cmd/usearch/main_test.go` (CLI-001 v0 suite) MUST pass against
  the v1 binary.
- CI step: `go test ./cmd/usearch/... -run 'TestVersionFlag|TestVersionShortFlag|TestExecute|TestQueryParsesPositionalAndFlags|TestQueryRejectsZeroPositional|TestQueryRejectsTwoPositionals|TestSourceFlagFiltersAdapters|TestFormatJSONShape|TestFormatTextShape|TestFormatInvalidExitsOne|TestTimeoutCancelsFanout|TestStdoutContainsOnlyPayload|TestEmptyPromptExitsOne|TestExitZeroOnFullSuccess|TestExitThreeOnSynthesisFailure'`
  asserts byte-equivalence.

### 4.2 NFR-CLI2-002 — Binary size

- CI step (default build): `go build -ldflags "-s -w" -trimpath -o
  /tmp/usearch ./cmd/usearch && du -m /tmp/usearch | awk '{print $1}'`
  → assert ≤ 30.
- CI step (full tags): `go build -tags tui,history-sqlite -ldflags
  "-s -w" -trimpath -o /tmp/usearch-full ./cmd/usearch && du -m
  /tmp/usearch-full` → assert ≤ 45.
- _TBD_: if default build exceeds 30 MB after cobra+koanf+xdg are
  pinned, propose adjusting NFR to 35 MB with run-phase justification.

### 4.3 NFR-CLI2-003 — Goroutine hygiene

- `cmd/usearch/testmain_test.go` declares
  `func TestMain(m *testing.M)` invoking `goleak.VerifyTestMain(m)`.
- All subcommand tests run under this TestMain.
- Specific assertions:
  - REPL `/exit` releases all goroutines within 50 ms.
  - Streaming consumer releases parser goroutine within
    `SYN004_DISCONNECT_CANCEL_MS + 100 ms` of EOF / cancel.
  - History async writer drained within 100 ms grace.

### 4.4 NFR-CLI2-004 — Human-readable errors

- `TestErrorMessagesNoStackTrace`: table-driven; for each error path,
  capture stderr; assert NO line contains `goroutine ` or
  `runtime.gopark` or starts with `\t`.
- `TestErrorMessagesUnder200Chars`: each error line ≤ 200 chars.
- `TestDebugModeRevealsStackTraces`: with `LOG_LEVEL=DEBUG`, stderr
  DOES contain JSON slog DEBUG records on errors.
- `TestJSONFormatErrorsToStdout`: in `--format json` mode, structured
  errors go to stdout (not stderr).

### 4.5 NFR-CLI2-005 — No credentials leak

- `TestNoCredentialsInStderr`: across all test scenarios, regex-grep
  stderr for `Bearer [A-Za-z0-9._-]+`, `eyJ[A-Za-z0-9._-]+`,
  `sk_[A-Za-z0-9_-]+`, `pk_[A-Za-z0-9_-]+`; assert zero matches.
- `TestNoCredentialsInStdout`: same for stdout in JSON mode.
- `TestNoCredentialsInHistory`: scan all written history entries;
  assert no token-shaped strings present.
- `TestCredentialsFileMode0600`: after `config set auth.token ...`
  (if such a path exists in v1 — likely it doesn't; only the future
  M6 path writes tokens), the file mode is exactly 0600.
- `TestCredentialsFileLooserModeRefused`: pre-create the credentials
  file with mode 0644; assert CLI exit 1 with the documented error.

---

## 5. Edge Cases

### 5.1 Empty prompt to `usearch deep`

- **When** the user runs `usearch deep ""`
- **Then** exit code 1, stderr `usearch deep: prompt argument
  required` (mirrors REQ-CLI-007 v0 behaviour for `query`)

### 5.2 Multiple positional arguments to `usearch deep`

- **When** the user runs `usearch deep "a" "b"`
- **Then** exit code 1, stderr `exactly one positional argument
  expected`

### 5.3 `--stream` without server SSE support

- **Given** an old `usearch-api` (pre-SYN-004) that ignores Accept
  header
- **When** the user runs `usearch query --stream "x"`
- **Then** the CLI detects non-SSE response (Content-Type:
  application/json) and falls back to buffered JSON rendering with
  a stderr WARN `streaming requested but server returned JSON; falling
  back to buffered`
- **And** exit code matches the buffered path

### 5.4 SSE stream terminated early (`event: error`)

- **Given** SSE stream emits 2 `event: sentence` then `event: error`
- **When** consumed
- **Then** stdout shows the 2 partial sentences (buffered in JSON
  mode), exit 2, stderr names the error

### 5.5 History file unwritable (disk full)

- **Given** the history file path is on a full disk
- **When** the user runs a successful query
- **Then** the query still exits 0 (history is fire-and-forget)
- **And** stderr contains a WARN `history write failed: <reason>`

### 5.6 Config file with unknown keys

- **Given** config file contains an unknown key `[banana] flavor = "x"`
- **When** the user runs any command
- **Then** the unknown key is silently ignored (koanf default
  behaviour); other valid keys still loaded; exit 0

### 5.7 REPL multi-line input

- **Given** REPL active
- **When** the user pastes a query spanning multiple lines
- **Then** _TBD_: either accept until blank line, OR accept on Enter
  with explicit Ctrl-J for newline insertion. Resolution in
  annotation cycle.

### 5.8 Concurrent `usearch` invocations writing to history

- **Given** two `usearch query` invocations starting within 10 ms
- **When** both succeed
- **Then** both history entries appear in the file without corruption
  (verify by line count + JSON parse round-trip)
- **And** no entries are interleaved mid-line

### 5.9 `usearch deep` invoked while DEEP-002/003 not yet implemented

- **Given** `/deep` returns HTTP 501 Not Implemented or 503 with
  `{"error":"deep_pipeline_unavailable",...}`
- **When** the user runs `usearch deep "x"`
- **Then** the CLI handles the error gracefully (per generic 5xx
  path: stderr message, exit 2); does NOT panic or hang

### 5.10 `--help` for hidden / aliased commands

- **When** the user runs `usearch help` (no double-dash)
- **Then** cobra dispatches to the same `--help` output

### 5.11 Cobra completion script generation

- **When** the user runs `usearch completion bash`
- **Then** stdout contains valid bash completion script
- **And** exit code 0

### 5.12 Config init in non-empty directory

- **Given** `~/.config/usearch/config.toml` already exists
- **When** the user runs `usearch config init`
- **Then** the CLI refuses to overwrite by default; exit 1, stderr
  `config file already exists at <path>; use --force to overwrite`
- **Given** `--force` flag
- **Then** the existing file is renamed to `config.toml.bak.<timestamp>`
  and a fresh default is written

### 5.13 History entries from REPL share the same backend

- **Given** REPL active
- **When** the user submits 3 queries via REPL then exits
- **Then** running `usearch history list` from a fresh shell shows
  the 3 entries (i.e. REPL and subcommand writes hit the same backend)

### 5.14 Stream consumer encounters heartbeat-only stream

- **Given** SSE stream emits only `: ping\n\n` comments (no sentence
  events) for 30 seconds, then `event: done`
- **When** the user runs `usearch query --stream "x"`
- **Then** the CLI does not panic; on `event: done` emits empty
  payload with stats; exit 3 (degraded — no synthesis output)

---

## 6. Open Questions for Annotation Cycle

Promoted from spec.md §6 and research.md §12. Acceptance for each
will be added when the question is resolved.

- Q5.7: REPL multi-line input policy.
- Q3.9: should `sources status` be in v1, or wait for SPEC-EVAL-002?
- Q3.5.x: `--force` flag on `usearch deep` if SPEC-DEEP-004 grows
  override support.
- NFR-CLI2-002: binary size cap re-evaluation.
- Brand context: `.moai/project/brand/` populated before v1 ship?

---

*End of SPEC-CLI-002 acceptance v0.1 (draft).*
