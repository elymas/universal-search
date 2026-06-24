---
id: SPEC-CLI-002
title: usearch CLI v1 — Full Subcommand Tree, Interactive Mode, Streaming
version: 1.0.0
milestone: M7 — Surfaces
status: in-progress
priority: P1
owner: expert-backend
methodology: tdd
coverage_target: 85
created: 2026-05-22
updated: 2026-06-24
author: limbowl
issue_number: null
depends_on: [SPEC-CLI-001, SPEC-SYN-004, SPEC-DEEP-001, SPEC-DEEP-004, SPEC-IR-001, SPEC-OBS-001]
blocks: [SPEC-SKILL-001]
---

# SPEC-CLI-002: `usearch` CLI v1

## HISTORY

- 2026-05-22 (initial draft v0.1, limbowl via manager-spec):
  Companion artifact: `research.md` (this same directory). First
  EARS-formatted SPEC extending SPEC-CLI-001 (implemented v0.1,
  2026-05-04) into the full M7 CLI surface. Adds 4 new top-level
  subcommands (`deep`, `sources`, `history`, `config`, `login`),
  an interactive REPL mode (zero-args entry), streaming output
  rendering against SPEC-SYN-004 SSE, a `markdown` output format,
  XDG-compliant config file support, persistent session/history
  backend, and SPEC-DEEP-004 quota error surfacing. Migrates CLI
  library from stdlib `flag` to `github.com/spf13/cobra` (deferred
  decision in CLI-001 §2.5). Treats v0 `usearch query` invocation
  as a **backward-compatible contract** — no breaking changes to
  flags, exit codes, or output shape. New behaviour is exposed via
  new flags and new subcommands only.

  Open design points are marked `_TBD_` for resolution during the
  annotation cycle (see §6). Highlights:
  - Binary size cap (NFR-CLI-003 inherited from v0; v1 delta from
    cobra + optional bubbletea + optional SQLite needs measurement).
  - TUI gated behind `tui` build tag (default OFF) so default builds
    stay within budget.
  - History backend defaults to JSONL (zero dep cost); SQLite opt-in
    via build tag + config.
  - `--stream` default behaviour: auto-detect TTY.
  - Brand context (`.moai/project/brand/`) currently `_TBD_`;
    user-facing copy will be finalized post-brand-interview.

  14 EARS REQs (4 × P0 + 8 × P1 + 2 × P2) + 5 NFRs. Five EARS patterns
  used (Ubiquitous + Event-Driven + State-Driven + Unwanted + Optional).
  Status `draft` pending plan-auditor + annotation cycle.

---

## Implementation Status (2026-06-24 audit correction)

The broader CLI subcommand tree (cobra migration, `sources`, `history`, `config`, `login`,
streaming, `--format markdown`, shell completions) is implemented and unit-tested
(72/72 tests pass).

Deferred — REQ-CLI2-003 (`usearch deep`): `cmd/usearch/deep_cmd.go:29-44` RunE prints
placeholder stage labels and returns an error with the message "Deep research pipeline
not yet wired (requires LLM client)." — the `@MX:TODO` at line 32 tracks the wiring.

Remediation path: wiring requires an `llm.Client` (LITELLM) + a real `FanoutFn`
(DEEP-003 Phase E) + mounting `NewDeepHandler` on the usearch-api mux + a storm sidecar
client; tracked for a future implementation pass.

---

## 1. Purpose

SPEC-CLI-001 shipped `usearch query` as the v0 entry point to the
universal-search system. It is intentionally minimal: one subcommand,
no config file, no TUI, no streaming, no history. v0 was the wedge
that proved the M2 end-to-end slice.

SPEC-CLI-002 grows that wedge into the **M7 CLI v1 surface** — the
primary command-line UX that ships with V1.0.0. The roadmap entry at
`.moai/project/roadmap.md:94` declares:

> SPEC-CLI-002 | CLI full | `usearch deep`, `usearch team list/members`,
> `--format md/json`, bubbletea TUI optional | expert-backend

V1.0.0 ships four user-facing surfaces (CLI, MCP, Claude Skill, Web UI).
CLI is the most-used surface in self-hosted team environments because
it is scriptable, fast, and integrates with existing developer tools
(jq pipelines, shell history, tmux panes). v1's job is to make `usearch`
feel **as polished as `gh`** while preserving v0's UNIX-y
"stdout-is-payload, stderr-is-noise" discipline.

Key v1 additions (extending v0):

- **`usearch deep "..."`** — invoke the M5 `/deep` multi-agent
  pipeline; surface SPEC-DEEP-004 quota errors gracefully.
- **`usearch sources [list|status|show]`** — discover what adapters
  are registered and healthy.
- **`usearch history [list|show|search|clear]`** — persistent query
  history across sessions.
- **`usearch config [show|set|get|path|init]`** — XDG-compliant config
  file management.
- **`usearch login [status|logout]`** — auth skeleton (real OIDC
  deferred to SPEC-AUTH-001 M6).
- **Interactive REPL** — zero-args invocation (`usearch`) enters a
  query loop with optional bubbletea TUI.
- **Streaming output** — consume SPEC-SYN-004 SSE; render sentence-by-
  sentence with incremental citation display.
- **`--format markdown`** — emit copy-paste-ready Markdown alongside
  existing `text` and `json` formats.

This SPEC is INTENTIONALLY ADDITIVE. It does NOT re-specify the v0
`query` subcommand — v0 invocation continues unchanged. New behaviour
is exposed via new flags (`--stream`, `--no-stream`, `--render`,
`--format markdown`) and new subcommands only.

---

## 2. Scope

### 2.1 In-Scope

| Ref | Item |
|-----|------|
| a | Migrate CLI library from stdlib `flag` to `github.com/spf13/cobra` — wrap existing `cmd/usearch/query.go::Execute` as a `cobra.Command.RunE`; preserve invocation contract verbatim. |
| b | Add `usearch deep <prompt>` subcommand — POSTs to the deep endpoint exposed by SPEC-IR-001; consumes SPEC-DEEP-004 cost-guard response codes (200/400/429/503); renders streaming output by default. |
| c | Add `usearch sources` subcommand tree — `list` (always implemented), `status` and `show <name>` (optional, gated on SPEC-EVAL-002 health endpoint). |
| d | Add `usearch history` subcommand tree — `list`, `show <id>`, `search <prompt>`, `clear`; persists query+deep invocations to a configurable backend (JSONL default, SQLite opt-in). |
| e | Add `usearch config` subcommand tree — `path`, `show`, `init`, `get <key>`, `set <key> <value>`; reads XDG-compliant config file (TOML); layered precedence via koanf (flag > env > file > default). |
| f | Add `usearch login` skeleton — `status`, `logout`. v1 placeholder; real OIDC integration deferred to SPEC-AUTH-001 (M6). Surface is reserved so M6 addition is additive (no breaking CLI changes). |
| g | Add interactive REPL mode — zero-args (`usearch`) enters a query loop. Default implementation: plain readline-style prompt with history (in-process up/down recall). Behind `tui` build tag: bubbletea-rendered TUI with viewport, syntax-highlighted input, streaming sentence-by-sentence display. |
| h | Add `--stream / --no-stream` flag to `query` and `deep` — when streaming, the CLI requests `Accept: text/event-stream` and consumes SPEC-SYN-004 events incrementally. Auto-detect default: `--stream` if stdout is a TTY, `--no-stream` if piped. |
| i | Add `--format markdown` (alias `md`) output format — renders summary + footnoted citation list as copy-paste-ready Markdown for both `query` and `deep`. Implementation via `text/template`; optional in-terminal styled render via `--render` flag (gated on `tui` build tag, uses glamour). |
| j | Add config file support at XDG-compliant path (`$XDG_CONFIG_HOME/usearch/config.toml`, default `~/.config/usearch/config.toml`). Layered precedence via koanf (already in tech stack). Schema includes `[server]`, `[auth]`, `[output]`, `[history]`, `[deep]`, `[sources]`, `[tui]` sections. |
| k | Add session/history persistence — default backend JSONL appended to `$XDG_DATA_HOME/usearch/history.jsonl`, FIFO eviction at `max_entries` (default 1000), retention cap `retention_days` (default 90). Build tag `history-sqlite` enables modernc.org/sqlite backend with FTS5 search. |
| l | Surface SPEC-DEEP-004 quota errors humanely — parse HTTP 429 (`cap_exceeded`), HTTP 400 (`deep_not_warranted`, `query_rejected_by_screen`), HTTP 503 (`costguard_unavailable`), HTTP 200 + `X-Deep-Degraded` header; render human-readable stderr messages with appropriate exit codes; preserve structured error in JSON format. |
| m | Extend exit code table from v0 — preserve 0/1/2/3 semantics; add code 4 reserved for "auth required" (placeholder for M6 SPEC-AUTH-001); document the v1 mapping in `--help`. |
| n | Add auto-generated `--help` per subcommand via cobra; add suggestions on typo ("Did you mean ...?"); add shell completion generators (`usearch completion bash|zsh|fish|powershell`). |
| o | Preserve all v0 invariants: stdout is payload-only, stderr is noise; `goleak.VerifyTestMain` for goroutine hygiene; OTel root span per invocation; request ID propagation. |
| p | Build tags: `tui` (gates bubbletea + lipgloss + bubbles + glamour), `history-sqlite` (gates modernc.org/sqlite). Default build excludes both for binary size discipline. |

### 2.2 Out-of-Scope (Exclusions — What NOT to Build)

[HARD] This SPEC explicitly excludes the following items. Each has a
known destination SPEC or follow-up; this list prevents scope creep.

- **NOT re-specifying `usearch query` v0 behaviour** — v0 is the
  canonical reference for the `query` subcommand. v1 migrates the
  underlying library (`flag` → cobra) and adds NEW flags (`--stream`,
  `--no-stream`, `--render`) only. Existing flags/output/exit-codes
  unchanged.
- **NOT real OIDC integration in `login`** — `login` v1 is a skeleton.
  Real OIDC device-code flow, token storage, refresh, JWT
  claims-based identity all owned by SPEC-AUTH-001 (M6).
- **NOT real team plane in `team` subcommand** — `team list`,
  `team members`, `team add` are NOT in v1. They depend on M6 SPEC-AUTH-001
  + SPEC-AUTH-002 + SPEC-IDX-004. v1 does NOT add a `team` subcommand;
  it will be added in a follow-up SPEC after M6 ships.
- **NOT MCP server** — owned by SPEC-MCP-001 (parallel in M7).
- **NOT web UI** — owned by SPEC-UI-001 (parallel in M7).
- **NOT Claude Skill** — owned by SPEC-SKILL-001 (M7, gated on MCP).
- **NOT cross-session shared history** — v1 history is local-only.
  Team-shared answer reuse is SPEC-IDX-005 (M6) territory; that
  uses Qdrant/Meili, not the CLI's local JSONL/SQLite store.
- **NOT cache layer for query results** — same posture as v0;
  caching is the index layer's job, not the CLI's.
- **NOT WebSocket / gRPC streaming** — v1 streaming is SSE-only,
  consuming what SPEC-SYN-004 emits.
- **NOT YAML / CSV output formats** — only `human`/`text`, `json`,
  and `markdown`/`md` in v1. JSON covers programmatic needs;
  YAML/CSV deferred.
- **NOT real-time cost websocket push** — same posture as
  SPEC-DEEP-004 §4 exclusions; pull-only via response body.
- **NOT ML-tuned Haiku score override** — same posture as
  SPEC-DEEP-004 §4 exclusions.
- **NOT modifying SPEC-SYN-004 SSE wire format or server-side
  behaviour** — v1 is a pure SSE CLIENT. SPEC-SYN-004 owns the
  server contract; CLI-002 consumes it read-only.
- **NOT modifying SPEC-DEEP-004 cost guard / cap logic** — v1
  parses DEEP-004 response codes and surfaces them; it does NOT
  alter cap rules, ledger, or rate-limit behaviour.
- **NOT a force-override of Haiku pre-screen rejection** — if
  DEEP-004 supports an `X-Force-Deep: 1` header, the CLI MAY
  surface a `--force` flag, but its semantics are owned by DEEP-004.
  v1 does NOT change DEEP-004 behaviour. _TBD_ in annotation cycle.
- **NOT GitHub Issue tracking on this SPEC** (`issue_number: null`).

### 2.3 Forward-Looking Dependencies

[HARD] SPEC-CLI-002's `depends_on` list includes SPEC-IR-001 (HTTP
server scaffolding). At time of drafting, IR-001 is implemented per
SPEC-DEEP-004 §6.1 P-N6 amendment. The deep endpoint
(`POST /deep` or equivalent) is exposed by IR-001's server layer; the
exact URL pattern is owned by IR-001 and is consumed via the
`[server] endpoint` config key.

`SPEC-DEEP-002` and `SPEC-DEEP-003` are in `draft` status. v1 does
NOT require their full implementation to ship — it requires the
DEEP-004 cost-guard error contract (parsed regardless of whether
DEEP-002/003 are fully wired). If `/deep` returns HTTP 503 because
the deep pipeline is not yet ready, the CLI surfaces that cleanly.

---

## 3. EARS Requirements

### Functional Requirements

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| REQ-CLI2-001 | Ubiquitous | The `usearch` binary SHALL dispatch all subcommands via `github.com/spf13/cobra`. The v0 `usearch query "..."` invocation contract (positional prompt, `--source`/`--format`/`--timeout` flags, stdout payload, stderr progress, exit codes 0/1/2/3, schema-versioned JSON shape) SHALL be preserved verbatim — no breaking changes. Migration from stdlib `flag` SHALL be implemented as `cobra.Command.RunE` wrapping the existing `cmd/usearch/query.go::Execute(args, stdout, stderr)` function. | P0 | All existing CLI-001 acceptance tests pass unchanged; new `TestQueryV0InvocationBackwardCompatible` table covers each v0 flag combination; `usearch --help` lists `query` as a subcommand alongside the new commands. |
| REQ-CLI2-002 | Ubiquitous | The `usearch` binary SHALL register the following top-level subcommands: `query` (from v0), `deep`, `sources`, `history`, `config`, `login`, `completion`, and a hidden `version` (existing `--version` flag preserved). Each subcommand SHALL provide its own `--help` text describing positional arguments, flags, exit codes, and at least one usage example. Subcommand routing SHALL fall through to interactive REPL mode (REQ-CLI2-008) when invoked with zero arguments AND stdin is a TTY. | P0 | `usearch --help` lists all subcommands; `usearch <subcommand> --help` shows per-subcommand help; `TestSubcommandRegistry` enumerates registered commands. |
| REQ-CLI2-003 | Event-Driven | WHEN the user invokes `usearch deep <prompt>`, the CLI SHALL POST the prompt to the `/deep` endpoint configured via `[server] endpoint`, attaching `X-User-Id` and `X-Tenant-Id` headers from `[auth]` config when set, and `X-Allow-Degrade: 1` header when `[deep] allow_degrade = true` OR `--allow-degrade` flag is present. The CLI SHALL parse SPEC-DEEP-004 cost-guard response codes per §3.5 of this SPEC's acceptance.md. | P0 | `TestDeepSubcommandPostsToConfiguredEndpoint`, `TestDeepHeadersFromConfig`, `TestDeepHeadersFromFlags`, `TestDeepAllowDegradeFlag`. |
| REQ-CLI2-004 | Event-Driven | WHEN the `/deep` endpoint returns HTTP 429 with body shape `{"error":"cap_exceeded","dimension":"calls"\|"usd","remaining":{...},"reset_at":"..."}` AND `Retry-After` header, the CLI SHALL render to stderr a single human-readable line of the form `usearch deep: daily limit reached (<dimension>); resets at <reset_at> (~<N> remaining)`, SHALL exit with code 2, AND in `--format json` mode SHALL emit the structured error object to stdout verbatim. WHEN the endpoint returns HTTP 400 with `{"error":"deep_not_warranted","suggested_mode":"basic","screen_score":N,"rationale":"..."}`, the CLI SHALL render `usearch deep: pre-screen suggests /basic mode (score N/10): <rationale>. Try 'usearch query' instead.` to stderr and exit code 1. WHEN the endpoint returns HTTP 400 with `{"error":"query_rejected_by_screen",...}` the CLI SHALL render the rationale to stderr and exit 1. WHEN HTTP 503 `{"error":"costguard_unavailable",...}` the CLI SHALL render `usearch deep: cost guard unavailable; retry later or use 'usearch query'.` and exit 2. WHEN HTTP 200 includes `X-Deep-Degraded: cap-exceeded` header, the CLI SHALL render a warning to stderr `usearch deep: warning — cap exceeded, fell back to /basic mode` and proceed with normal output (exit 0). | P0 | Six tests in `TestDeepQuotaErrorMatrix` covering 429/400-not-warranted/400-rejected/503/200-degraded/200-clean cases; each asserts stderr text, stdout shape, and exit code. |
| REQ-CLI2-005 | Optional | WHERE the user supplies `--stream`, the CLI SHALL set `Accept: text/event-stream` on the HTTP request and SHALL consume the SPEC-SYN-004 SSE wire format incrementally; for each `event: sentence`, the CLI SHALL render the sentence's `text` field (with `[N]` citation markers preserved) to stdout in `--format human`/`text`/`markdown` modes OR append to a buffered JSON array for emission on `event: done` in `--format json` mode; on `event: done` the CLI SHALL emit final stats (latency, model, cost) to stderr in human mode OR include in the `stats` object in JSON mode; on `event: error` the CLI SHALL render the error to stderr and exit 2; on client EOF / network reset the CLI SHALL cancel the parent context and exit 2. WHERE the user supplies `--no-stream`, the CLI SHALL set `Accept: application/json` and consume the buffered JSON response unchanged from v0. WHEN neither flag is supplied, the CLI SHALL auto-detect: `--stream` if stdout is a TTY, `--no-stream` otherwise (so piped invocation like `usearch query "x" | jq` remains byte-compatible with v0 JSON output). | P1 | `TestStreamFlagRequestsSSE`, `TestNoStreamFlagRequestsJSON`, `TestStreamAutoDetectTTY`, `TestStreamRendersSentenceByQ`, `TestStreamHandlesEventError`, `TestStreamHandlesEventDone`, `TestStreamCancelOnEOF`. |
| REQ-CLI2-006 | Ubiquitous | The CLI SHALL support output formats `human` (alias `text`, default in TTY), `json` (default in non-TTY, schema-versioned object identical to v0), and `markdown` (alias `md`). For `markdown` format, the CLI SHALL emit a single Markdown document with `# Query: <prompt>` heading, the summary paragraph with inline `[^N]` footnote markers, a `## Sources` section listing each citation as `[^N]: [<title>](<url>) — <source>`, and a trailing horizontal rule + metadata footer `Generated by usearch v<version> | <model> | $<cost> | <latency>ms`. Invalid `--format` values SHALL be rejected with exit code 1 and stderr message `unsupported format '<value>'; valid: human, text, json, markdown, md`. | P1 | `TestFormatMarkdownShape`, `TestFormatHumanIsDefaultInTTY`, `TestFormatJSONIsDefaultPiped`, `TestFormatMarkdownAlias`, `TestFormatInvalidRejected`. |
| REQ-CLI2-007 | Ubiquitous | The CLI SHALL load configuration via `github.com/knadh/koanf` from an XDG-compliant TOML file at `$XDG_CONFIG_HOME/usearch/config.toml` (default `~/.config/usearch/config.toml` on Linux/macOS; `%APPDATA%\usearch\config.toml` on Windows). Precedence order SHALL be: (1) command-line flags (highest), (2) environment variables with `USEARCH_` prefix, (3) config file, (4) built-in defaults (lowest). The config schema SHALL include sections `[server]` (endpoint, timeout_seconds), `[auth]` (user_id, tenant_id, token_file), `[output]` (default_format, color, pager), `[history]` (enabled, backend, path, max_entries, retention_days), `[deep]` (allow_degrade, force_override), `[sources]` (defaults), `[tui]` (enabled, theme). Missing config file SHALL be tolerated (built-in defaults applied); malformed config file SHALL exit 1 with stderr message naming the parse error. | P1 | `TestConfigLoadsFromXDGPath`, `TestConfigPrecedenceFlagOverEnvOverFileOverDefault`, `TestConfigMissingFileUsesDefaults`, `TestConfigMalformedExitsOne`, `TestConfigSchemaAllSections`. |
| REQ-CLI2-008 | State-Driven | WHILE `usearch` is invoked with zero positional arguments AND zero subcommands AND stdin is a TTY, the CLI SHALL enter interactive REPL mode. The default REPL (always-available, no build tag) SHALL provide a readline-style prompt `usearch> ` with in-session up/down history recall, support for `/exit` and `/help` slash-commands, and execute each non-slash input line as if it were the argument to `usearch query`. WHEN the `tui` build tag is active, the REPL SHALL use a bubbletea-rendered interface with a viewport for streaming output, a syntax-highlighted input field, and incremental sentence-by-sentence rendering of SSE responses; the TUI SHALL preserve the same `/exit`, `/help`, and slash-command surface, AND SHALL additionally support `/deep`, `/sources`, `/history`, `/config` slash-commands that invoke the equivalent subcommands inline. The REPL SHALL persist queries to the history backend (REQ-CLI2-010) regardless of TUI mode. | P1 | `TestREPLEntryOnZeroArgs`, `TestREPLSkipsWhenStdinPiped`, `TestREPLBasicQueryFlow`, `TestREPLSlashExit`, `TestREPLSlashHelp`, `TestREPLHistoryPersisted`; `TestTUIReplStreamingRender` (under `tui` build tag); `TestTUISlashDeep` (under `tui` build tag). |
| REQ-CLI2-009 | Ubiquitous | The CLI SHALL register the `sources` subcommand tree with `list` (always implemented), `status` and `show <name>` (gated on SPEC-EVAL-002 adapter health endpoint per §6 _TBD_). `usearch sources list` SHALL emit one line per registered adapter with the shape `<name>\t<category>\t<lang>\t<auth_required:y/n>` to stdout in human/text mode, OR an array of objects with the same fields in JSON mode, OR a Markdown table in markdown mode. `usearch sources show <name>` (if implemented) SHALL print full adapter capabilities including rate limits, required env vars, and supported intent categories. Unknown adapter SHALL exit 1 with stderr `usearch sources: unknown adapter '<name>'`. | P1 | `TestSourcesList`, `TestSourcesListJSONFormat`, `TestSourcesListMarkdownFormat`, `TestSourcesShowKnown`, `TestSourcesShowUnknownExitsOne`. |
| REQ-CLI2-010 | Ubiquitous | The CLI SHALL persist every successful `query` and `deep` invocation to a history backend configured via `[history] backend` (default `jsonl`; alternative `sqlite` gated on `history-sqlite` build tag). Each history entry SHALL include `{id, timestamp, command, prompt, category, adapters, summary, citations, exit_code, latency_ms, cost_usd, request_id, schema_version: 1}`. The history write SHALL be asynchronous (background goroutine) and SHALL NOT block the CLI's exit; write errors SHALL be logged to stderr at WARN level and SHALL NOT change the exit code. FIFO eviction SHALL apply at `max_entries` (default 1000); time-based retention SHALL apply at `retention_days` (default 90). The history file SHALL live at `$XDG_DATA_HOME/usearch/history.{jsonl,db}` (default `~/.local/share/usearch/`). | P1 | `TestHistoryWriteOnSuccessfulQuery`, `TestHistoryWriteAsyncDoesNotBlockExit`, `TestHistoryWriteErrorLoggedNotFatal`, `TestHistoryFIFOEvictionAtMaxEntries`, `TestHistoryRetentionPurgesOldEntries`, `TestHistoryBackendJSONLDefault`, `TestHistoryBackendSQLite` (under `history-sqlite` build tag). |
| REQ-CLI2-011 | Ubiquitous | The CLI SHALL register the `history` subcommand tree with `list`, `show <id>`, `search <prompt>`, and `clear` (with `--confirm` flag required for non-interactive invocation). `history list` SHALL emit recent entries reverse-chronological with `<id>\t<timestamp>\t<command>\t<prompt-truncated>` shape; `--limit <N>` (default 20), `--since <duration>` (e.g. `7d`, `24h`) flags supported. `history show <id>` SHALL print the full entry in the user's `--format` choice. `history search <prompt>` SHALL substring-match on the `prompt` field by default (JSONL backend), OR FTS5 full-text rank when SQLite backend is active. `history clear` SHALL prompt for confirmation in TTY mode OR require `--confirm` non-interactively; `history clear --since <duration>` SHALL purge only entries older than the cutoff. | P1 | `TestHistoryList`, `TestHistoryListLimitFlag`, `TestHistoryListSinceFlag`, `TestHistoryShowKnownId`, `TestHistoryShowUnknownIdExitsOne`, `TestHistorySearchSubstring`, `TestHistorySearchFTS5` (under `history-sqlite`), `TestHistoryClearRequiresConfirm`, `TestHistoryClearSinceFilter`. |
| REQ-CLI2-012 | Ubiquitous | The CLI SHALL register the `config` subcommand tree with `path`, `show`, `init`, `get <key>`, `set <key> <value>`. `config path` SHALL print the resolved config file path to stdout. `config show` SHALL print the effective merged configuration (after precedence resolution) in TOML format. `config init` SHALL create the config file at `config path` with default values; in TTY mode, SHALL prompt the user via plain prompts (or huh-based form under `tui` build tag) for the most-common keys (`server.endpoint`, `auth.user_id`, `auth.tenant_id`, `output.default_format`, `history.enabled`). `config get <key>` SHALL print the value of a single key in the user's `--format` choice (json prints as `{"<key>":<value>}`). `config set <key> <value>` SHALL write the key to the config file, creating the file if missing; SHALL refuse to write keys in the reserved `[auth] token` namespace (those go to a separate credentials file with mode 0600). | P1 | `TestConfigPath`, `TestConfigShow`, `TestConfigInitCreatesFile`, `TestConfigInitWizardTTY` (interactive), `TestConfigGetKnownKey`, `TestConfigGetUnknownKeyExitsOne`, `TestConfigSetWritesToFile`, `TestConfigSetRefusesTokenKey`. |
| REQ-CLI2-013 | Optional | WHERE SPEC-AUTH-001 (M6) has not yet shipped (detected via absence of `[auth] oidc_endpoint` config OR explicit `[auth] enabled = false`), the `usearch login` subcommand SHALL print to stderr `usearch login: OIDC auth not yet enabled. Set [auth] user_id and [auth] tenant_id in your config with 'usearch config set'.` and exit code 1. `usearch login status` SHALL print the current `[auth] user_id` and `[auth] tenant_id` values (or `<not set>`) to stdout. `usearch login logout` SHALL clear the `[auth] user_id`, `[auth] tenant_id`, and `[auth] token_file` keys from the config file, AND SHALL delete the `[auth] token_file` if it exists (mode-0600 enforcement). The v1 placeholder surface SHALL be additive: when SPEC-AUTH-001 ships, `usearch login` (without subcommand) SHALL invoke the OIDC device-code flow without breaking existing `login status` / `logout` invocations. | P2 | `TestLoginPlaceholderMessage`, `TestLoginStatusShowsConfigValues`, `TestLoginLogoutClearsConfig`, `TestLoginLogoutDeletesTokenFile`, `TestLoginForwardCompatHooksReserved`. |
| REQ-CLI2-014 | Unwanted | IF the binary is invoked with a subcommand name that is not registered (e.g. `usearch typo`), THEN cobra SHALL print to stderr `Error: unknown command "typo" for "usearch"\n` followed by `Did you mean "<suggestion>"?` when the Levenshtein distance to a registered subcommand is ≤ 2, AND the CLI SHALL exit with code 1. The CLI SHALL NEVER fall through to interactive REPL mode when an unknown subcommand is supplied — REPL entry is exclusively the zero-args + TTY path. | P2 | `TestUnknownSubcommandExitsOne`, `TestUnknownSubcommandSuggestsClose`, `TestUnknownSubcommandDoesNotEnterREPL`. |

### Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| NFR-CLI2-001 | Backward Compatibility (v0 invariants preserved) | Every test in `cmd/usearch/query_test.go` and `cmd/usearch/main_test.go` from SPEC-CLI-001 SHALL pass unchanged against the v1 binary. Specifically: stdout-only-payload, stderr-only-noise, exit codes 0/1/2/3 with v0 semantics, `--source`/`--format`/`--timeout` flag parsing, `--version` / `-v` invocation, JSON schema_version `"1"` for `query` responses. CI SHALL fail if any v0 test regresses. |
| NFR-CLI2-002 | Binary Size Cap | The release-mode binary (`go build -ldflags "-s -w" -trimpath ./cmd/usearch`) SHALL be ≤ 30 MB on linux/amd64 in the default build configuration (no build tags). With `-tags tui,history-sqlite`, the binary SHALL be ≤ 45 MB. CI SHALL fail on regression at either threshold. _TBD_ during plan: confirm 30 MB headroom holds after cobra + koanf delta; if not, propose raising default cap to 35 MB with documented justification. |
| NFR-CLI2-003 | Goroutine Hygiene | All subcommand handlers SHALL pass `goleak.VerifyTestMain` — no goroutines SHALL leak after `Execute(...)` returns. The history write goroutine SHALL be explicitly awaited (with a 100 ms grace period) before process exit; if the wait exceeds 100 ms, the write is abandoned with a stderr WARN log (REQ-CLI2-010). The REPL SHALL release all bubbletea/readline goroutines on `/exit` within 50 ms. The streaming consumer SHALL release the SSE parser goroutine within `SYN004_DISCONNECT_CANCEL_MS + 100 ms` of EOF / cancel. |
| NFR-CLI2-004 | Human-Readable Errors (extends v0 NFR-CLI-004) | All stderr error messages SHALL be single-line plain English ≤ 200 characters, naming the stage and underlying cause. Stack traces and panic dumps SHALL be reserved for `LOG_LEVEL=DEBUG` mode (JSON slog DEBUG records). Quota-error messages (REQ-CLI2-004) SHALL include actionable next steps (e.g. "Try 'usearch query' instead", "Resets at <timestamp>"). Unknown-subcommand suggestions SHALL be human-friendly. The `--format json` mode SHALL emit structured errors to stdout (NOT stderr) so jq-style consumers can parse them. |
| NFR-CLI2-005 | No Credentials in Logs / History | No subcommand SHALL emit credentials (`[auth] token`, OIDC token, API keys read from env) to stdout, stderr, slog records, OTel spans, OR the history backend. The history entry schema (REQ-CLI2-010) SHALL NOT include any field derived from the `[auth]` config section beyond `user_id` and `tenant_id` (which are opaque identifiers, not secrets). The credentials file `~/.config/usearch/credentials` SHALL be created with mode 0600 enforced at write time; if the file exists with looser permissions, the CLI SHALL refuse to read it and exit 1 with stderr message `usearch: refusing to read credentials with mode <octal>; expected 0600`. |

---

## 4. Acceptance Criteria

Detailed Given/When/Then scenarios live in
`.moai/specs/SPEC-CLI-002/acceptance.md`. This section enumerates the
acceptance gate per requirement; cross-references that document for
the full matrix.

### REQ-CLI2-001 — Cobra migration preserves v0 invocation

- All v0 acceptance tests pass against the cobra-built binary.
- `cmd/usearch/query.go::Execute` signature unchanged; cobra wraps via
  `RunE: func(cmd, args) { os.Exit(query.Execute(ctx, args, stdout,
  stderr)) }` pattern.
- `usearch --help` shows `query` alongside new subcommands.
- `usearch query --help` shows v0 flags.

### REQ-CLI2-002 — Subcommand tree registered

- `usearch --help` enumerates: `query`, `deep`, `sources`, `history`,
  `config`, `login`, `completion`.
- `usearch <subcommand> --help` shows per-subcommand text with at
  least one example.

### REQ-CLI2-003 / REQ-CLI2-004 — Deep + quota error surfacing

- See `acceptance.md` §3.5 for the full 6-case quota error matrix.

### REQ-CLI2-005 — Streaming flag

- `--stream` requests SSE; `--no-stream` requests JSON; default
  auto-detects TTY.
- TTY detection: `os.Stdout.Stat()` returns `(stat.Mode() &
  os.ModeCharDevice) != 0` (idiomatic Go TTY check).

### REQ-CLI2-006 — Markdown format

- `--format markdown` emits a single Markdown document per the §3
  shape.
- `--format md` is an alias; both produce byte-equivalent output.

### REQ-CLI2-007 — Config file

- See `acceptance.md` §3.7.

### REQ-CLI2-008 — Interactive REPL

- Zero-args + TTY enters REPL; zero-args + piped stdin prints help
  and exits 0 (does NOT enter REPL).
- `/exit` exits cleanly; `/help` lists slash-commands.

### REQ-CLI2-009 — Sources subcommand

- `usearch sources list` outputs one line per adapter from the
  registry.

### REQ-CLI2-010 / REQ-CLI2-011 — History

- History write is fire-and-forget after response emission.
- `history list`, `show`, `search`, `clear` cover the canonical
  operations.

### REQ-CLI2-012 — Config subcommands

- `config path`, `show`, `init`, `get`, `set` work against the
  loaded config.

### REQ-CLI2-013 — Login placeholder

- Pre-M6 invocation prints the placeholder message; `status` and
  `logout` work without OIDC.

### REQ-CLI2-014 — Unknown subcommand

- Typo handling via cobra suggestions; REPL never entered on unknown.

### NFR-CLI2-001 — v0 invariants

- CI runs both v0 and v1 test suites; v0 suite identical bytes.

### NFR-CLI2-002 — Binary size

- CI step measures `du -m` after release-mode build; fails on regression.

### NFR-CLI2-003 — Goroutine hygiene

- `goleak.VerifyTestMain` in each subcommand's `*_test.go`.

### NFR-CLI2-004 — Human-readable errors

- Error message linter test asserts ≤ 200 chars and no `goroutine ` /
  `runtime.gopark` strings.

### NFR-CLI2-005 — No credentials leak

- Audit test grep's all emitted stderr/stdout/history entries for
  patterns matching common token shapes (Bearer, eyJ..., sk_...);
  asserts zero matches under any test scenario.

---

## 5. Technical Approach (high-level — full plan in plan.md)

### 5.1 Phasing

Implementation proceeds in five phases (full details in plan.md):

1. **Config** — koanf integration, XDG path resolution, schema
   validation, `usearch config *` subcommands.
2. **History** — JSONL backend, async writer, retention/eviction,
   `usearch history *` subcommands. SQLite backend behind build tag.
3. **Interactive REPL** — readline default, bubbletea behind `tui`
   build tag, slash-command dispatch, history integration.
4. **Streaming** — SSE consumer, `--stream/--no-stream` flag, TTY
   auto-detect, rendering integration into `query` and `deep`.
5. **Deep + finishing** — `usearch deep` subcommand, DEEP-004 quota
   error matrix, `--format markdown` rendering, completion generators,
   `usearch login` placeholder.

Cobra migration (REQ-CLI2-001) happens FIRST in phase 1, so all
subsequent phases use cobra naturally.

### 5.2 Package layout (proposed — _TBD_ refinement in plan)

```
cmd/usearch/
├── main.go              [MODIFY] cobra root command registration
├── query.go             [MODIFY] wrap Execute as cobra RunE
├── deep.go              [NEW] cobra command for `usearch deep`
├── sources.go           [NEW] cobra command tree for `usearch sources *`
├── history.go           [NEW] cobra command tree for `usearch history *`
├── config_cmd.go        [NEW] cobra command tree for `usearch config *`
├── login.go             [NEW] cobra command tree for `usearch login *`
├── repl.go              [NEW] zero-args REPL entry
├── repl_tui.go          [NEW] bubbletea REPL, gated by `tui` build tag
├── completion.go        [NEW] cobra completion subcommand
├── stream.go            [NEW] SSE consumer; --stream/--no-stream flag
├── output_markdown.go   [NEW] markdown format renderer
├── output_text.go       [MODIFY] minor: `human` alias
├── output_json.go       [UNCHANGED] schema_version: 1 preserved
├── exitcode.go          [MODIFY] add code 4 (auth required, reserved)
├── progress.go          [UNCHANGED]
└── ... (existing tests + new tests per phase)

internal/usearch/config/
├── config.go            [NEW] koanf loader + schema struct
├── xdg.go               [NEW] XDG path resolution
└── config_test.go       [NEW]

internal/usearch/history/
├── history.go           [NEW] backend interface + Entry struct
├── jsonl.go             [NEW] JSONL backend
├── sqlite.go            [NEW] SQLite backend, gated by `history-sqlite`
├── async.go             [NEW] background writer
└── *_test.go            [NEW]

internal/usearch/sse/
├── client.go            [NEW] SSE consumer (stdlib bufio.Scanner-based)
└── client_test.go       [NEW]
```

### 5.3 Library Dependencies (new)

| Library | Purpose | Conditional | Size |
|---------|---------|-------------|------|
| `github.com/spf13/cobra` | CLI framework | always | ~500 KB |
| `github.com/knadh/koanf` | Config loader | always | already in tech stack |
| `github.com/adrg/xdg` | XDG path resolution | always | ~5 KB |
| `github.com/charmbracelet/bubbletea` | TUI framework | `tui` tag | ~500 KB |
| `github.com/charmbracelet/lipgloss` | Styling | `tui` tag | ~150 KB |
| `github.com/charmbracelet/bubbles` | TUI widgets | `tui` tag | ~150 KB |
| `github.com/charmbracelet/glamour` | Markdown render | `tui` tag | ~300 KB |
| `github.com/charmbracelet/huh` | Form library | `tui` tag (optional) | ~200 KB |
| `modernc.org/sqlite` | SQLite backend | `history-sqlite` tag | ~6 MB |

Default build: cobra + koanf + xdg = ~500 KB net delta over v0.
Full build: + bubbletea family = ~1.6 MB delta. + SQLite = ~7 MB delta.

### 5.4 Test Strategy

- **Backward compat**: re-run all v0 tests against v1 binary; assert
  byte-equivalent output for piped invocations.
- **Per-subcommand**: cobra `cmd.SetArgs(...)` + `cmd.Execute()` for
  in-process unit tests; `os/exec` for process-boundary integration
  tests.
- **SSE consumer**: `httptest.Server` emits pre-recorded SSE wire
  sequences; assert sentence rendering and event handling.
- **REPL**: stdin/stdout piped through `bytes.Buffer` for unit tests;
  manual smoke test for TUI mode.
- **History**: temp directory per test; verify JSONL append + FIFO +
  retention logic.
- **Config**: temp `HOME` env var for XDG path; verify precedence
  layers.
- **Quota error matrix**: `httptest.Server` returns each documented
  error shape; assert CLI rendering + exit code.

### 5.5 Coverage Target

85% (matches v0). Subcommand handlers are well-bounded; history and
config logic is data-flow dense; REPL and TUI carry small allowances
for terminal-state edge cases.

---

## 6. Open Questions

The 18 questions enumerated in `research.md` §12 are deferred to the
annotation cycle. Highlights summarized here for SPEC-author attention:

1. cobra vs alternatives (recommendation: cobra).
2. koanf vs viper (recommendation: koanf, already in tech stack).
3. TUI default: behind `tui` build tag (recommended) or always on.
4. History default backend: JSONL (recommended) or SQLite.
5. `--stream` default: auto-detect TTY (recommended).
6. `usearch deep --force`: depends on SPEC-DEEP-004 support.
7. `markdown` format: v1 for both `query` and `deep`.
8. NFR-CLI2-002 binary cap: 30 MB default, 45 MB with all tags.
9. `sources status / show`: gated on SPEC-EVAL-002 health endpoint.
10. Exit code 4 reservation for auth-required.
11. Brand context for user-facing copy (currently `_TBD_` in
    `.moai/project/brand/`; re-evaluate before v1 ship).

Full list with rationale in `research.md` §12.

---

## 7. Dependencies

### 7.1 Upstream SPEC dependencies (depends_on)

- **SPEC-CLI-001** (implemented) — parent SPEC; v1 extends additively
  without breaking the v0 invocation contract.
- **SPEC-SYN-004** (implemented) — SSE wire format and sentence event
  schema consumed by `--stream` mode.
- **SPEC-DEEP-001** (implemented) — STORM long-form report whose
  markdown emission is consumed by `usearch deep --format markdown`.
- **SPEC-DEEP-004** (implemented) — cost guard response codes
  (200/200+degraded/400/429/503) consumed and surfaced by
  `usearch deep`.
- **SPEC-IR-001** (implemented) — HTTP server scaffolding exposing
  `/synthesize` and `/deep` endpoints.
- **SPEC-OBS-001** (implemented) — slog + OTel + reqid infrastructure
  for v1 spans and request IDs.

### 7.2 Soft / forward-looking dependencies

- **SPEC-DEEP-002, SPEC-DEEP-003** (draft) — deep pipeline. v1 ships
  regardless; if the pipeline is not yet wired, `/deep` returns 503
  and the CLI surfaces it cleanly per REQ-CLI2-004.
- **SPEC-EVAL-002** (M8, future) — per-adapter health endpoint. `usearch
  sources status` and `show` MAY be implemented in v1 against a stub OR
  deferred until EVAL-002 ships. _TBD_ in plan annotation.
- **SPEC-AUTH-001** (M6, draft) — real OIDC integration that replaces
  v1's `login` placeholder. v1 reserves the subcommand surface so M6
  addition is additive.

### 7.3 Downstream blocked SPECs (blocks)

- **SPEC-SKILL-001** (M7) — Claude Skill packaging wraps the CLI as
  bundled scripts; relies on stable v1 invocation contract.

### 7.4 External dependencies (run-phase pins — proposal)

- `github.com/spf13/cobra@v1.8.x` (cobra latest stable)
- `github.com/knadh/koanf/v2@v2.x` (already in tech stack)
- `github.com/adrg/xdg@v0.5.x`
- `github.com/charmbracelet/bubbletea@v1.x` (under `tui` tag)
- `github.com/charmbracelet/lipgloss@v1.x` (under `tui` tag)
- `github.com/charmbracelet/bubbles@v0.x` (under `tui` tag)
- `github.com/charmbracelet/glamour@v0.x` (under `tui` tag)
- `modernc.org/sqlite@v1.x` (under `history-sqlite` tag)

Versions pinned in plan phase post-WebFetch verification.

---

## 8. Risks (full register in research.md §11)

| # | Risk | Severity | Mitigation |
|---|------|----------|------------|
| R1 | Binary size cap regression | High | Build tags `tui` and `history-sqlite` off by default; CI gate at 30 MB default / 45 MB full. |
| R2 | cobra migration breaks v0 invocation | High | NFR-CLI2-001 v0 backward compat test suite. |
| R3 | SSE consumer drops events on network blip | Medium | Stdlib `bufio.Scanner`; EOF → exit 2; no resume in v1 (matches SYN-004 exclusion). |
| R4 | REPL conflicts with subcommand routing | Medium | REPL is zero-args + TTY only; explicit guard in REQ-CLI2-014. |
| R7 | Credentials leak via logs or history | High | NFR-CLI2-005 audit test; mode 0600 enforced on credentials file. |
| R9 | DEEP-004 quota error format change | Medium | Parse against documented schema; tolerate unknown fields. |
| R12 | Auto-stream surprises piped consumers | High | TTY auto-detect; explicit `--no-stream` flag; v0 piped behaviour preserved by default. |

---

## 9. References

### Internal

- `.moai/specs/SPEC-CLI-001/spec.md` — parent SPEC; v0 contract.
- `.moai/specs/SPEC-CLI-001/research.md` — v0 CLI library survey;
  cobra deferral rationale.
- `.moai/specs/SPEC-SYN-004/spec.md` — SSE wire format, sentence
  event schema.
- `.moai/specs/SPEC-DEEP-004/spec.md` — quota error response schemas.
- `.moai/specs/SPEC-IR-001/` — server endpoint exposure.
- `.moai/specs/SPEC-AUTH-001/` (M6, draft) — future OIDC integration.
- `.moai/specs/SPEC-IDX-005/` (M6, draft) — team-shared answer reuse.
- `.moai/specs/SPEC-EVAL-002/` (M8, future) — adapter health endpoint.
- `.moai/specs/SPEC-SKILL-001/` (M7) — Claude Skill wrapping the CLI.
- `.moai/project/roadmap.md:94, 129, 152, 158` — M7 + M9 entries.
- `.moai/project/tech.md:32, 80` — koanf + cobra confirmation.
- `.moai/project/product.md:42-44` — V1 surfaces include CLI.
- `.moai/project/brand/` — `_TBD_` placeholders; re-check before v1
  user-facing copy ship.
- `cmd/usearch/main.go`, `cmd/usearch/query.go` — v0 entry to migrate.

### External (verify in plan phase per anti-hallucination policy)

- `github.com/spf13/cobra`
- `github.com/knadh/koanf`
- `github.com/adrg/xdg`
- `github.com/charmbracelet/bubbletea`, `lipgloss`, `bubbles`,
  `glamour`, `huh`
- `modernc.org/sqlite`
- W3C Server-Sent Events
- XDG Base Directory Specification
- `gh` CLI subcommand + auto-TTY pattern reference

---

*End of SPEC-CLI-002 v0.1 (draft).*
