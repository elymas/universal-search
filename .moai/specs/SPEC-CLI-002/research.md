# Research — SPEC-CLI-002: `usearch` CLI v1

Date: 2026-05-22
Author: limbowl (via manager-spec)
Phase: plan / research
Status: draft

This artifact captures the codebase analysis, library survey, and design
tradeoffs underpinning SPEC-CLI-002. SPEC-CLI-002 extends the v0 CLI
shipped by SPEC-CLI-001 (`usearch query`) into the full M7 surface:
additional subcommands (`deep`, `sources`, `history`, `config`, `login`),
an optional interactive REPL mode, streaming output rendering against
SPEC-SYN-004 SSE, configuration file support, session/history persistence,
and an expanded output-format matrix. Every claim is file-cited
(path + line range) or URL-cited.

This SPEC is INTENTIONALLY a **superset** of SPEC-CLI-001 — it does NOT
re-specify the `query` subcommand. Where v1 needs to evolve the v0
behaviour (e.g. cobra migration), the changes are framed as **additive
migrations** with backward-compatible CLI invocation contracts.

Forward-looking notes are marked `_TBD_` where the v1 design space is
intentionally left open for the annotation cycle to resolve.

---

## 1. Parent SPEC Carry-Forward (SPEC-CLI-001)

### 1.1 What SPEC-CLI-001 shipped (2026-05-04, implemented)

- `usearch query "..."` subcommand with `--source`, `--format`, `--timeout`
  flags. Source: `cmd/usearch/query.go`.
- Output formats: `text` (default), `json` (schema-versioned).
- Exit codes: 0 (success), 1 (user error), 2 (system / timeout / no
  adapters), 3 (partial result / synthesis unavailable).
- Stdout reserved for answer payload; stderr reserved for progress,
  slog records, errors.
- stdlib `flag` only — no cobra, no TUI, no config file. Decision
  explicitly deferred to SPEC-CLI-002 (this SPEC).
- `internal/synthesis.Client` interface + `nopclient` fallback for
  degraded mode when SPEC-SYN-001 is unavailable.
- OTel root span `usearch.cli.query` with `request_id` propagation.
- `goleak.VerifyTestMain` for goroutine hygiene.
- Binary size cap: ≤ 30 MB (NFR-CLI-003).

### 1.2 What SPEC-CLI-001 explicitly deferred to CLI-002

From SPEC-CLI-001 §2.2 exclusions, the following items are CLI-002's
inherited scope:

- `usearch deep` subcommand (multi-agent, long-form report) — gated on
  M5 SPEC-DEEP-001/002/003/004.
- `usearch team list` / `team members` / `team add` — gated on M6
  SPEC-AUTH-001/002 + SPEC-IDX-004.
- TUI / bubbletea interactive mode.
- Stdin-piped query input (`echo "..." | usearch query`).
- Tag-based `--source` filtering (e.g. `--source social`).
- Output formats beyond `text` and `json` (markdown explicitly requested
  by user; yaml/csv possible follow-up).
- True LLM-streaming output (SPEC-SYN-004 SSE) — landed 2026-05-09,
  CLI-002 consumes it.
- Cobra / urfave/cli / kong CLI library adoption.
- Shell completion scripts.
- Configuration file support (`~/.config/usearch/config.toml`).
- Interactive credential prompts.
- Result caching (per-query cache key, TTL).

### 1.3 Constraints inherited unchanged

- `usearch query` invocation contract MUST remain backward-compatible
  (existing CLI users, integration tests, and downstream automation
  continue to work).
- Binary size cap: ≤ 30 MB (NFR-CLI-003) — re-evaluated for cobra+TUI
  delta in §3.5.
- `goleak.VerifyTestMain` for goroutine hygiene.
- stdout/stderr separation rule.
- Exit code semantics (0/1/2/3) extended consistently for new
  subcommands; no breaking changes.
- All instruction documents in English; `code_comments: en`.

---

## 2. M7 Surface Requirements (Roadmap)

`.moai/project/roadmap.md` line 94 declares M7 SPEC-CLI-002:

> SPEC-CLI-002 | CLI full | `usearch deep`, `usearch team list/members`,
> `--format md/json`, bubbletea TUI optional | expert-backend

Cross-referenced with the M7 parallelization plan (`roadmap.md:129`):
SPEC-MCP-001 + SPEC-CLI-002 + SPEC-UI-001 are 3-way parallel.

M9 exit criterion (`roadmap.md:158`):
> Helm chart deployable; docs site live.

CLI-002 is NOT in the M9 release-gate critical path, but it is the
primary command-line UX for V1.0.0. User-visible polish (interactive
mode, history, markdown rendering) matters for adoption.

---

## 3. CLI Library Survey (v1)

The v0 decision (stdlib `flag`) was bounded by single-subcommand scope.
v1 introduces a multi-level subcommand tree:

```
usearch
├── query <prompt>          [v0 — preserve invocation]
├── deep <prompt>           [v1 — new]
├── sources                 [v1 — new; lists registered adapters]
│   ├── list
│   ├── status              [optional — per-adapter health]
│   └── show <name>         [optional — adapter detail]
├── history                 [v1 — new]
│   ├── list                [recent queries]
│   ├── show <id>           [replay a stored result]
│   ├── search <prompt>     [search prior queries]
│   └── clear               [delete history]
├── config                  [v1 — new]
│   ├── show                [print current effective config]
│   ├── set <key> <value>   [write a key to config file]
│   ├── get <key>           [read a key]
│   ├── path                [print config file path]
│   └── init                [create default config file]
├── login                   [v1 — new; auth detail deferred to M6]
│   ├── status              [print current auth state]
│   └── logout              [clear stored credentials]
├── (no subcommand)         [v1 — interactive REPL mode]
├── --version / -v          [v0 — preserve]
└── --help / -h             [v1 — auto-generated by cobra]
```

This is **3 levels of nesting** and **5+ top-level subcommands** with
sub-subcommands under `sources`, `history`, `config`, `login`. This is
the regime where cobra wins decisively (SPEC-CLI-001 research §2.5).

### 3.1 Option A — `github.com/spf13/cobra` (RECOMMENDED for v1)

- **Pros**:
  - Industry standard for multi-subcommand Go CLIs (`gh`, `kubectl`,
    `helm`, `docker`, `hugo`, `gpg`, `go` itself uses a similar
    pattern).
  - Rich auto-generated `--help` per subcommand including positional
    args, flag descriptions, usage examples, and subcommand listings.
  - Auto-completion for bash, zsh, fish, powershell via
    `cobra.Command.GenBashCompletion()` / `GenZshCompletion()`.
  - Suggestions on typo (`Did you mean 'query'?`).
  - Nested command tree scales to N levels — required for `sources`,
    `history`, `config`, `login` sub-subcommands.
  - Integrates with `spf13/viper` for layered config (env > flag > file
    > default) which exactly matches v1's config requirement.
- **Cons**:
  - Adds ~500 KB to binary size and ~12 transitive dependencies (per
    SPEC-CLI-001 research §2.2; verified 2026-04-28).
  - Migration cost from stdlib `flag` is mechanical for the existing
    `query` subcommand — `Execute(args, stdout, stderr) int` becomes
    `*cobra.Command.RunE`. Test surface (`exec.Command`) unchanged.
- **Verdict**: SELECTED for v1. The break-even point (≥ 4 subcommands
  with help text) is decisively crossed. NFR-CLI-003 (30 MB) headroom
  is preserved: current binary is well under cap; cobra delta keeps it
  within budget. _TBD_ in plan phase: measure actual delta on first
  cobra-migrated build and update NFR if needed.

### 3.2 Option B — stdlib `flag` (REJECTED for v1)

Continues to scale poorly past 2 subcommand levels. Custom subcommand
dispatch + per-subcommand `*flag.FlagSet` becomes 500+ LOC of
hand-rolled help text and routing logic. Suggestions and completion
are not feasible without re-implementing major chunks of cobra. Not
viable.

### 3.3 Option C — `github.com/urfave/cli/v2` (REJECTED for v1)

Lighter than cobra (~200 KB binary delta, single dependency) but:

- Smaller ecosystem; less idiomatic in modern Go projects.
- Struct-tag-based flags conflict with the existing `flag.FlagSet`
  pattern that v0 uses.
- Migration story from `flag` is less clean than cobra's.
- Less production proof in highly-nested subcommand trees.

### 3.4 Option D — `github.com/alecthomas/kong` (REJECTED for v1)

Declarative struct-tag-based; tiny binary delta (~80 KB); type-safe.
But:

- The project has no other `kong` usage.
- Smaller community; fewer references for new contributors.
- Less battle-tested for multi-level subcommand trees with
  auto-completion.

### 3.5 Binary size impact (cobra + bubbletea + viper)

Estimated delta:

- cobra v1.8+: ~500 KB
- viper (for config layering): ~600 KB transitive (includes mapstructure,
  hashicorp/hcl, magiconair/properties, spf13/afero, etc.)
- bubbletea + lipgloss + bubbles: ~800 KB combined for TUI
- huh (form library, optional): ~200 KB

Worst-case total v1 delta: ~2.1 MB on top of v0 (~5 MB build).

NFR-CLI-003 v1 target: _TBD_ — propose raising cap to **40 MB** to
accommodate v1 surface, OR keep at 30 MB and selectively shed deps
(skip viper, use koanf already in project; skip huh; make bubbletea
opt-in via build tag).

**Recommendation for plan phase**: keep cap at 30 MB; use koanf (already
pinned per `.moai/project/tech.md` line 32) for config instead of viper;
bubbletea behind `tui` build tag (off by default) so the lean binary
size stays under cap and TUI is opt-in for developers who want it.

---

## 4. Interactive TUI / REPL Options

The user request includes "Interactive REPL mode (no args)" — invoking
`usearch` with zero arguments enters an interactive session.

### 4.1 Option A — `github.com/charmbracelet/bubbletea` (RECOMMENDED)

- **Pros**:
  - The de-facto standard for Go TUIs in 2025+ (charmbracelet ecosystem).
  - Elm-style model-view-update architecture; clean separation of state
    and rendering.
  - Already battle-tested at scale (Glow, Soft Serve, Gum, Glow CLI).
  - First-class composition with `lipgloss` (styling) and `bubbles`
    (input widgets, viewport, spinner, progress bar).
  - Excellent support for streaming output (incremental render on
    message tick) — fits SPEC-SYN-004 SSE sentence-level streaming.
  - Same vendor as `powernap` (already pinned per
    `.claude/rules/moai/core/lsp-client.md`) — single vendor risk
    surface.
- **Cons**:
  - ~800 KB binary delta (combined with lipgloss + bubbles).
  - Mitigated by build tag (see §3.5).
- **Verdict**: SELECTED for v1 interactive mode.

### 4.2 Option B — `github.com/charmbracelet/huh` (form library)

- **Pros**:
  - High-level form/prompt library on top of bubbletea.
  - Drop-in for setup wizards (`usearch config init`, `usearch login`).
  - ~200 KB delta on top of bubbletea.
- **Verdict**: USEFUL for `config init` and `login` wizards. _TBD_:
  evaluate during plan phase whether the value justifies the extra
  dependency, OR roll our own minimal prompts on top of bubbletea.

### 4.3 Option C — `github.com/c-bata/go-prompt` (REJECTED)

- Older library, less maintained; readline-style input only; no full
  TUI framework.
- Rejected.

### 4.4 Option D — Plain readline (`golang.org/x/term`)

- Minimal: no dependency cost.
- Limited UX: no syntax highlighting, no inline rendering, no
  streaming-friendly partial-redraw model.
- Acceptable fallback if `tui` build tag is off.
- **Verdict**: SHIP a minimal readline-based REPL in the default build;
  bubbletea-rich REPL behind `tui` build tag.

### 4.5 REPL session model — open questions

_TBD_ during plan phase:

- Does the REPL preserve query history across sessions? (Probably yes
  per the `history` subcommand requirement.)
- Does the REPL share the same history backend as the `history`
  subcommand?
- Are slash-commands inside the REPL (e.g. `/deep`, `/sources`, `/exit`)
  the in-session subcommand surface, OR are subcommands the only entry
  and the REPL is just a query-loop wrapper?
- Multi-line prompt input support (paste a long query)?

---

## 5. Streaming Output Rendering (SPEC-SYN-004 SSE)

SPEC-SYN-004 (implemented 2026-05-09) provides Server-Sent Events on
`cmd/usearch-api`'s synthesis endpoint. Per `acceptance.md` and
`spec.md`:

- Wire format: W3C SSE; `event: sentence`, `event: done`, `event: error`.
- Sentence-level event granularity (one `event: sentence` per validated
  sentence with citations attached).
- Heartbeat `: ping\n\n` every 15 s (configurable).
- Client-disconnect cancels upstream LLM call within
  `SYN004_DISCONNECT_CANCEL_MS` (default 1 s).
- Accept-header fallback: `text/event-stream` → SSE, otherwise → JSON.
- Schema versioned (`schema_version: 1`).

### 5.1 Consumption strategy from CLI

The CLI's streaming consumer MUST:

1. Issue HTTP GET to `/synthesize` (or whatever IR-001 exposes) with
   `Accept: text/event-stream`.
2. Parse SSE wire format (event/data lines, blank-line terminator).
   - Recommended Go libraries:
     - **stdlib** `bufio.Scanner` with custom SplitFunc — ZERO
       dependency cost, ~80 LOC. Sufficient for one-way SSE.
     - `github.com/r3labs/sse/v2` client — ~200 KB delta. Overkill for
       our needs (we don't need multi-subscriber broadcast).
   - **Recommendation**: roll our own minimal SSE parser. SPEC-CLI-001
     research §2 demonstrates the team's "minimal dependency" preference
     and SPEC-SYN-004 spec.md §3.4 ("NOT SSE multi-subscriber broadcast")
     confirms the simple consumer pattern.
3. Render each `event: sentence` payload incrementally:
   - **text** format: print the sentence text + citation markers
     `[N]` to stdout, append to a running paragraph buffer; emit
     citation list on `event: done`.
   - **json** format: collect events into an array; emit a single JSON
     object on `event: done` (NOT one JSON-per-line; the v0 JSON
     contract is a single object).
   - **markdown** format (NEW in v1, see §6): render to a streaming
     markdown buffer with footnote-style citations.
   - **TUI/REPL** mode: render to a bubbletea viewport with
     incremental redraw on each `event: sentence` message tick.
4. On `event: done`: emit final stats (latency, model, cost) to stderr
   in text mode, OR include in the JSON `stats` object in JSON mode.
5. On `event: error`: render error to stderr; exit 2.
6. On client disconnect (Ctrl-C, EOF, network loss): close the HTTP
   request; SPEC-SYN-004 server-side handles cancellation.

### 5.2 Backward compatibility

The v0 `usearch query` invocation MUST continue to work against
non-streaming `synthesis.Client.Synthesize()` callers. v1 strategy:

- New flag `--stream / --no-stream` (default behaviour _TBD_):
  - `--stream`: request SSE from server; render incrementally.
  - `--no-stream`: request buffered JSON; behave identically to v0.
- Default behaviour proposal: `--stream` enabled if stdout is a TTY,
  disabled if piped (e.g. `usearch query "x" | jq`). This is the
  "do the right thing" pattern (cf. `gh` CLI's auto-detection).
- `usearch deep` defaults to `--stream` always (deep queries are long
  enough that incremental output is a strong UX win).

### 5.3 Quota error surfacing (SPEC-DEEP-004)

SPEC-DEEP-004 governs `/deep` quota and cost guard. When the user
invokes `usearch deep "..."` and the server returns:

- **HTTP 429** + `Retry-After` header + body
  `{"error":"cap_exceeded","dimension":"calls"|"usd","remaining":...,
  "reset_at":"..."}`: CLI MUST:
  - Render a human-readable message to stderr: "Daily /deep limit
    reached (<dimension>). Resets at <reset_at>." with the parsed
    `Retry-After`.
  - Exit code 2 (system error — quota exhausted).
  - In `--format json` mode, still emit the structured error object to
    stdout for programmatic consumption.
  - In TUI mode, surface a dismissible alert with the same content.
- **HTTP 400** + `{"error":"deep_not_warranted","suggested_mode":"basic",
  "screen_score":N,"rationale":"..."}`: CLI MUST:
  - Render: "The Haiku pre-screen suggests this query is fine for
    /basic mode (score N/10). Reason: <rationale>. Run with
    --force to override, or use `usearch query` instead."
  - Exit code 1 (user error — query routing mismatch).
  - Optional v1 flag `--force` (alias `-f`) re-issues with header
    `X-Force-Deep: 1` _TBD_ — design decision in plan phase whether
    SPEC-DEEP-004 supports a force header or whether we just leave
    the user to choose `usearch query`.
- **HTTP 400** + `{"error":"query_rejected_by_screen", ...}`: render
  "Query rejected by pre-screen: <rationale>. Refine and retry."; exit
  1.
- **HTTP 503** + `{"error":"costguard_unavailable",...}`: render
  "Cost guard unavailable; deep mode unsafe. Retry later or use
  `usearch query`."; exit 2.
- **HTTP 200** + `X-Deep-Degraded: cap-exceeded`: render warning to
  stderr "Note: cap exceeded, fell back to /basic mode" and proceed
  with normal output. Exit 0 (success but degraded).
- Optional flag `--allow-degrade` injects `X-Allow-Degrade: 1` header.

---

## 6. Output Formats: human / json / markdown

User request: "Output formats: human/json/markdown". Mapping:

- **human** = the v0 `text` format. Renamed for clarity (and to align
  with the `gh` CLI convention). Alias `text` retained for back-compat.
- **json** = v0 `json` format, schema-versioned. Unchanged.
- **markdown** = NEW in v1.

### 6.1 Markdown rendering specification

Each `query` / `deep` response in `--format markdown` produces:

```markdown
# Query: {prompt}

> {summary paragraph with inline [^N] footnote markers}

## Sources

[^1]: [{title}]({url}) — {source name}
[^2]: [{title}]({url}) — {source name}
...

---
Generated by usearch v{version} | {model} | ${cost} | {latency_ms}ms
```

For `deep` responses (STORM-style report), output is the full long-form
report with proper heading hierarchy as emitted by SPEC-DEEP-001.

Rendering library options:

- **stdlib** `text/template` — sufficient, zero dependency cost. PREFERRED.
- `github.com/charmbracelet/glamour` — renders markdown to styled
  terminal output. Useful for IN-TERMINAL preview (`--format markdown
  --render`) but not for the raw markdown emission. Optional, behind
  `tui` build tag.

### 6.2 Format aliases and default

- `--format human` (default in TTY) | `--format text` (alias) — for
  humans reading the terminal output.
- `--format json` — for programmatic consumers and CI integration.
- `--format markdown` (alias `md`) — for copy-paste into docs, GitHub
  issues, Notion, etc.
- _TBD_: should `--format yaml` or `--format csv` be in v1, or
  deferred? Recommendation: defer — `json` covers programmatic needs;
  YAML/CSV add code without measurable user demand yet.

---

## 7. Configuration File

User request: "Config file location".

### 7.1 Standard location conventions

Per `https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html`
(XDG Base Directory Specification), Go convention is:

- Linux/macOS: `$XDG_CONFIG_HOME/usearch/config.toml` (default
  `~/.config/usearch/config.toml`)
- Windows: `%APPDATA%\usearch\config.toml`
- Fallback: `$HOME/.usearch/config.toml`

Discovery library: `github.com/adrg/xdg` (small, well-maintained,
~5 KB delta).

### 7.2 Format choice

- **TOML** (PREFERRED): human-readable, comments allowed, widely used
  in Go ecosystem (cargo, helm v3, hugo). Already in tech stack via
  koanf (`tech.md` line 32: "Config | koanf | layered TOML / env /
  flag").
- YAML: also acceptable; team plane already uses YAML
  (`.moai/config/sections/*.yaml`) so familiarity is high. Slightly
  larger parser footprint.
- JSON: programmer-friendly but no comments; awkward for hand-edited
  configs.

**Recommendation**: TOML, consistent with koanf default and tech stack.

### 7.3 Configuration schema (initial proposal — _TBD_ refinement in plan)

```toml
[server]
endpoint = "http://localhost:8080"   # usearch-api URL
timeout_seconds = 30

[auth]
# populated by `usearch login`; tokens NEVER committed.
token_file = "~/.config/usearch/credentials"
user_id = ""                          # X-User-Id header
tenant_id = ""                        # X-Tenant-Id header

[output]
default_format = "human"              # human | json | markdown
color = "auto"                        # auto | always | never
pager = "auto"                        # auto | always | never

[history]
enabled = true
backend = "sqlite"                    # sqlite | jsonl
path = ""                             # default: $XDG_DATA_HOME/usearch/history.db
max_entries = 1000
retention_days = 90

[deep]
allow_degrade = false                 # X-Allow-Degrade header default
force_override = false                # X-Force-Deep header default (if supported)

[sources]
# Per-source overrides for query routing
defaults = []                         # e.g. ["reddit", "hackernews", "naver"]

[tui]
enabled = true                        # falls back to plain if tui build tag off
theme = "default"                     # default | dark | light | high-contrast
```

### 7.4 Precedence

Per koanf's layered model:

1. Command-line flags (highest)
2. Environment variables (`USEARCH_*` prefix)
3. Config file (`~/.config/usearch/config.toml`)
4. Built-in defaults (lowest)

### 7.5 `usearch config` subcommands

- `usearch config path` — print the resolved config file path.
- `usearch config show` — print the effective merged config (after all
  precedence layers).
- `usearch config init` — create a default config file (interactive
  wizard if `tui` enabled, else writes defaults).
- `usearch config get <key>` — read one key.
- `usearch config set <key> <value>` — write one key.

---

## 8. Session / History Backend

User request: "Session history backend (SQLite/JSONL — TBD)".

### 8.1 Requirements

- Persist `usearch query` and `usearch deep` invocations across
  sessions.
- Capture: timestamp, prompt, intent category, adapters used, summary,
  citations, exit code, latency, cost, request_id.
- Support fast lookup by request_id (`history show <id>`).
- Support search by prompt substring (`history search "..."`).
- Support time-range queries (`history list --since 7d`).
- Bound the storage (max entries OR max disk space) with FIFO eviction.

### 8.2 Option A — SQLite (PREFERRED)

- **Pros**:
  - Mature, ubiquitous, single-file database.
  - Powerful query language; trivial to index by `(timestamp, prompt)`.
  - Excellent Go bindings: `modernc.org/sqlite` (pure-Go, no CGO —
    aligns with single-binary deploy) or `mattn/go-sqlite3` (CGO,
    faster but requires C toolchain).
  - Supports FTS5 for `history search` full-text queries.
  - Schema migrations are tractable.
- **Cons**:
  - Binary size: `modernc.org/sqlite` adds ~6 MB. NFR-CLI-003 cap
    impact: significant. Mitigation: behind `history-sqlite` build tag
    OR accept the cap raise to 40 MB (see §3.5).
- **Verdict**: PREFERRED for v1; binary size impact requires NFR
  reconciliation.

### 8.3 Option B — JSONL (Append-Only Log)

- **Pros**:
  - Zero dependency cost; stdlib only.
  - Tail-friendly; human-readable.
  - Trivial to grep / jq externally.
- **Cons**:
  - Linear scan for search; slow at high entry counts (>10k).
  - No indexes; manual FIFO eviction requires rewriting the file.
  - No transactional guarantees.
- **Verdict**: ACCEPTABLE as the default backend (low binary cost).
  Acceptable up to ~1k entries with `max_entries` cap.

### 8.4 Option C — BoltDB / bbolt

- **Pros**:
  - Pure-Go, ~200 KB binary delta. Key-value semantics.
  - Used by etcd, Consul, k3s.
- **Cons**:
  - No full-text search; we'd need to build our own index for
    `history search`.
  - Single-writer model; OK for CLI use case.
- **Verdict**: VIABLE MIDDLE GROUND. Smaller than SQLite, faster than
  JSONL.

### 8.5 Recommendation

Default to **JSONL** (zero dep cost, satisfies the basic requirement);
make **SQLite** opt-in via config (`[history] backend = "sqlite"`) with
build tag `history-sqlite` for users who want FTS and high-volume
storage.

**Schema (JSONL)** — one JSON object per line:

```json
{
  "id": "01JF...",
  "timestamp": "2026-05-22T10:30:45Z",
  "command": "query",
  "prompt": "...",
  "category": "social",
  "adapters": ["reddit", "hackernews"],
  "summary": "...",
  "citations": [{"index":1,"title":"...","url":"...","source":"reddit","doc_id":"..."}],
  "exit_code": 0,
  "latency_ms": 4523,
  "cost_usd": 0.0023,
  "request_id": "01JF...",
  "schema_version": 1
}
```

### 8.6 Open questions — _TBD_ in plan

- Should history be team-shared (synced to SPEC-IDX-005) or local-only?
  Recommendation: local-only for v1; team sharing is SPEC-IDX-005's
  job and uses a different storage backend (Qdrant/Meili).
- Privacy: should `history search` results include the full summary
  text, or only metadata + prompt?
- Retention: hard delete after `retention_days`, OR soft delete with
  audit trail?

---

## 9. `usearch login` (auth detail deferred to M6)

User request: "`login` (auth detail deferred)".

### 9.1 v1 surface (skeleton only)

The `login` subcommand exists in v1 as a UX placeholder. Real OIDC/SSO
integration is SPEC-AUTH-001 (M6). v1 scope:

- `usearch login` — prints "Auth not yet enabled. Use X-User-Id
  header via config." with a pointer to `usearch config set auth.user_id`.
- `usearch login status` — prints whether `auth.user_id` and
  `auth.tenant_id` are set in the config.
- `usearch login logout` — clears stored credentials (truncates the
  `[auth]` section in config, removes any token file).

### 9.2 Forward-compatibility with SPEC-AUTH-001

When M6 ships, SPEC-AUTH-001 will replace the v1 placeholder with:

- OIDC device-code flow (browser-based login).
- Token storage in `~/.config/usearch/credentials` (mode 0600, never
  committed).
- Automatic token refresh.
- `--user-id` / `--tenant-id` flags + `X-User-Id` / `X-Tenant-Id`
  headers populated automatically.

The v1 placeholder reserves the subcommand surface so that M6's
addition is purely additive — no breaking CLI changes.

---

## 10. Exit Codes (v1 extension of v0)

SPEC-CLI-001 established 4 codes (0/1/2/3). v1 preserves these and adds
context for new subcommands:

| Code | Meaning | When |
|------|---------|------|
| 0 | Success | All operations completed; payload emitted to stdout. |
| 1 | User error | Invalid args, unknown subcommand, unknown adapter, invalid format, empty prompt, deep-not-warranted, query-rejected-by-screen, write to read-only config, etc. |
| 2 | System error / timeout | Timeout exceeded, all adapters failed, no adapters matched, cap exceeded (429), cost guard unavailable (503), config file unreadable, history backend unreachable. |
| 3 | Partial result / degraded | Synthesis unavailable (nopclient), some adapters errored but partial response emitted, deep-degraded fallback (200 + X-Deep-Degraded header). |
| 4 | Auth required (_TBD_ — v1 placeholder) | M6 reservations for "credentials missing" / "token expired". |

Open question: do we need code 4 in v1 or wait for M6? Recommendation:
reserve in spec; v1 placeholder login doesn't emit it (uses code 1
"please run config set auth.user_id").

---

## 11. Risks & Mitigations

| # | Risk | Severity | Mitigation |
|---|------|----------|------------|
| R1 | Binary size exceeds NFR-CLI-003 (30 MB) with cobra+bubbletea+sqlite | High | Build tag `tui` (off by default) gates bubbletea; build tag `history-sqlite` (off by default) gates SQLite; use koanf not viper; re-evaluate cap at end of plan phase. |
| R2 | cobra migration breaks v0 `usearch query` invocation | High | Treat v0 invocation as a contract; integration test asserts exact CLI surface; cobra `RunE` wraps the existing `cmd/usearch/query.go::Execute`. |
| R3 | SSE consumer drops events under network blip | Medium | SPEC-SYN-004 already handles client-side disconnect via Ctrl-C → ctx cancel; CLI maps EOF/connection-reset to exit 2; no resume mechanism in v1 (matches SPEC-SYN-004 exclusion). |
| R4 | Interactive REPL conflicts with subcommand routing | Medium | REPL is the ZERO-args entry path only; if any args present, cobra dispatches normally. |
| R5 | History DB grows unbounded | Medium | `max_entries` cap (FIFO eviction); `retention_days` cap; `history clear` subcommand. |
| R6 | Markdown output format ambiguous for `deep` long-form reports | Medium | DEEP-001 already produces markdown-friendly output; format is pass-through with CLI-added metadata footer. |
| R7 | Config file world-readable; credentials leak | High | Force `0600` on `~/.config/usearch/credentials`; never write `auth.token` to `config.toml`; document in `config init`. |
| R8 | `--stream` default behaviour surprises users (auto-detect TTY) | Low | Document the auto-detection; provide explicit `--stream / --no-stream` override. |
| R9 | SPEC-DEEP-004 quota error formats change after M5 ships | Medium | DEEP-004 is implemented per ref reading; parse against the documented schema; tolerate unknown fields. |
| R10 | TUI input handling on Windows terminals (cmd.exe / PowerShell) | Medium | bubbletea has known Windows quirks; document supported terminals (Windows Terminal, iTerm2, tmux); recommend `tui` build tag off for Windows by default. |
| R11 | JSONL history file corruption (interrupted write) | Medium | Atomic append with `O_APPEND`; line-by-line decode skips corrupt entries with warning. |
| R12 | Backward-compat: existing scripts pipe `usearch query "..."` to jq | High | Auto-detect TTY for `--stream`; piped invocation defaults to `--no-stream --format json` for `jq`-style consumers? _TBD_ — must not break existing automation. |
| R13 | History writes block fast queries | Low | History write is async (background goroutine after response emitted); fail-soft (log to stderr, don't block CLI). |
| R14 | `usearch login` placeholder confuses users into thinking auth works | Medium | Help text + first-run message both clearly state "auth not yet enabled". |

---

## 12. Open Questions (carry to plan phase annotation)

Numbered for tracking through the annotation cycle:

1. **CLI library**: confirm cobra (vs alternatives) — recommendation
   per §3.1.
2. **Config layering**: confirm koanf reuse (already in tech stack) vs
   viper.
3. **TUI default**: bubbletea behind `tui` build tag (off by default)
   OR enabled by default — depends on NFR-CLI-003 cap re-evaluation.
4. **History backend default**: JSONL (PREFERRED) vs SQLite (richer).
5. **History storage location**: `$XDG_DATA_HOME/usearch/history.{db,jsonl}`
   confirmation.
6. **`--stream` default**: auto-detect TTY (recommended) OR always
   stream OR always buffered for `query`?
7. **`usearch deep --force`**: support force override of Haiku
   pre-screen rejection? Depends on SPEC-DEEP-004 support.
8. **Markdown format scope**: include for both `query` and `deep` v1,
   or just `deep` (where the long-form report is markdown-native)?
9. **Output format aliases**: `human` vs `text` — pick one canonical or
   support both as aliases?
10. **REPL interaction model**: slash-commands (`/deep`, `/sources`)
    inside REPL, OR REPL just loops `query` calls?
11. **Multi-line prompt input**: support in REPL via Ctrl-J or
    explicit "open editor" hotkey?
12. **`config init` interactive**: huh form library OR plain prompts?
13. **NFR-CLI-003 binary cap**: keep at 30 MB OR raise to 40 MB?
14. **`history search` ranking**: chronological OR relevance-ranked
    (requires FTS5 → SQLite)?
15. **`login` placeholder message**: how prescriptive? Mention M6
    SPEC-AUTH-001 timeline?
16. **Config schema**: confirm TOML structure proposed in §7.3.
17. **`sources status`**: include in v1 OR defer to SPEC-EVAL-002
    (M8 adapter reliability dashboard) which also reports per-adapter
    health?
18. **Exit code 4 (auth required)**: reserve in v1 or wait for M6?

---

## 13. References

### Internal

- `.moai/specs/SPEC-CLI-001/spec.md` — parent SPEC; carry-forward
  contract.
- `.moai/specs/SPEC-CLI-001/research.md` — v0 CLI library survey;
  cobra deferral rationale.
- `.moai/specs/SPEC-SYN-004/spec.md` — SSE wire format, sentence event
  schema, client-disconnect handling, Accept-header fallback.
- `.moai/specs/SPEC-DEEP-004/spec.md` — quota error response schemas
  (HTTP 429, 400, 503, 200+X-Deep-Degraded headers).
- `.moai/specs/SPEC-IR-001/` — HTTP server scaffolding; `cmd/usearch-api`
  endpoint exposure.
- `.moai/specs/SPEC-AUTH-001/` (M6, draft) — OIDC integration that
  will replace v1's `login` placeholder.
- `.moai/specs/SPEC-IDX-005/` (M6, draft) — team-shared answer reuse;
  potential future home for shared history.
- `.moai/project/roadmap.md:94, 129, 152, 158` — M7 SPEC-CLI-002
  scope, parallelization, M7/M9 exit criteria.
- `.moai/project/tech.md:32, 80` — koanf config, cobra+bubbletea CLI
  stack confirmation.
- `.moai/project/product.md:42-44` — V1 surfaces include CLI as
  primary.
- `.moai/project/brand/` — currently all `_TBD_` placeholders; no
  brand voice/visual identity content to load for CLI copy yet.
  Re-check before user-facing copy is finalized in v1.
- `cmd/usearch/main.go`, `cmd/usearch/query.go`, `cmd/usearch/output_*.go`,
  `cmd/usearch/progress.go`, `cmd/usearch/exitcode.go` — v0 CLI
  surface that v1 migrates to cobra without breaking.
- `internal/synthesis/client.go` — synthesis interface reused for
  streaming consumer.

### External (verify in plan phase per anti-hallucination policy)

- `github.com/spf13/cobra` — `https://pkg.go.dev/github.com/spf13/cobra`
- `github.com/charmbracelet/bubbletea` — `https://pkg.go.dev/github.com/charmbracelet/bubbletea`
- `github.com/charmbracelet/lipgloss` — `https://pkg.go.dev/github.com/charmbracelet/lipgloss`
- `github.com/charmbracelet/bubbles` — `https://pkg.go.dev/github.com/charmbracelet/bubbles`
- `github.com/charmbracelet/huh` — `https://pkg.go.dev/github.com/charmbracelet/huh`
- `github.com/charmbracelet/glamour` — `https://pkg.go.dev/github.com/charmbracelet/glamour`
- `github.com/knadh/koanf` — `https://pkg.go.dev/github.com/knadh/koanf` (already in tech stack)
- `github.com/adrg/xdg` — `https://pkg.go.dev/github.com/adrg/xdg`
- `modernc.org/sqlite` — `https://pkg.go.dev/modernc.org/sqlite` (pure-Go, no CGO)
- W3C Server-Sent Events — `https://html.spec.whatwg.org/multipage/server-sent-events.html`
- XDG Base Directory Spec — `https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html`
- `gh` CLI (subcommand + auto-TTY pattern reference) — `https://github.com/cli/cli`

---

*End of SPEC-CLI-002 research v0.1 (draft).*
