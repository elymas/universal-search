---
id: SPEC-SKILL-001
version: 1.0.0
status: implemented
created: 2026-05-22
updated: 2026-05-26
author: limbowl
priority: P1
issue_number: 0
title: Claude Skill marketplace package — usearch plugin wrapping the MCP server
milestone: M7 — Surfaces
owner: builder-skill
methodology: ddd
coverage_target: 85
depends_on: [SPEC-MCP-001, SPEC-CLI-002]
blocks: []
related: [SPEC-AUTH-001, SPEC-DEPLOY-001, SPEC-DOC-001, SPEC-REL-001]
---

# SPEC-SKILL-001: Claude Skill marketplace package — `usearch` plugin

## HISTORY

- 2026-05-22 (initial draft v0.1.0, limbowl via manager-spec):
  First EARS-formatted SPEC for the M7 Skill plugin surface, the last
  surface in the M7 chain per `.moai/project/roadmap.md` §3
  parallelization plan (after SPEC-MCP-001, SPEC-CLI-002, and the
  UI SPECs). SPEC-SKILL-001 packages the SPEC-MCP-001 server into a
  Claude Code **plugin** (Anthropic's distribution unit; see
  research §1) named `usearch`, containing a `SKILL.md` discovery
  surface, a `.mcp.json` wiring, a `plugin.json` manifest, and a
  README onboarding guide. The plugin is the surface that lets a
  Claude Code / Claude Desktop user install Universal Search with a
  single `/plugin install usearch` (or marketplace browse) after
  having installed the `usearch-mcp` binary locally.

  Pinned decisions:
  (D1) Distribution unit: **Plugin** (per Anthropic terminology), not a
       standalone Skill in `.claude/skills/`. Plugins are the only
       artifact that supports marketplace distribution, semantic
       versioning, and cross-project install. See research §1.
  (D2) MCP server bundling: **Strategy B** (reference user-installed
       binary via PATH). Strategy A (bundle binary) is excluded for V1
       due to plugin-size and cross-platform code-signing burden.
       Strategy C (hosted HTTP endpoint) is documented in README but
       not the default. See research §4.
  (D3) `SKILL.md` frontmatter: **standard Anthropic conventions only**
       (`description` required; `disable-model-invocation`,
       `user-invocable`, `allowed-tools`, `argument-hint` optional).
       MoAI-specific extensions (`metadata.*`, `progressive_disclosure`,
       `triggers.*`) are NOT used — the plugin ships outside the MoAI
       internal workflow and must be portable. See research §3.
  (D4) Plugin namespace: `usearch` (skill becomes `/usearch:search`,
       etc.). Matches the binary name; brevity over verbosity. See
       research §10 Q8.
  (D5) Versioning: explicit semver in `plugin.json:version`, decoupled
       from `usearch-mcp` binary version. README publishes a
       compatibility matrix. See research §7.
  (D6) Tool surface: passthrough only. The Skill plugin exposes the
       four SPEC-MCP-001 tools (`search`, `deep_research`,
       `list_sources`, `get_citation`) via MCP `tools/list` discovery.
       The plugin itself adds no new tools. See research §5.
  (D7) Marketplace target: **community** (`claude-plugins-community`)
       via the standard submission form. Self-hosted marketplace
       (`marketplace.json` in the plugin repo) is the documented
       fallback if review is gating V1 ship. See research §8.
  (D8) Scope boundary with non-MCP hosts: SPEC-SKILL-001 V1 is
       **Claude Code plugin only**. Gemini CLI / Codex CLI users
       configure the MCP server in their host's config directly (per
       SPEC-MCP-001 Phase 7 artifacts). The Skill does NOT wrap the
       CLI. See research §10 Q6.

  M7 release gate per `.moai/project/roadmap.md` §5 M9 exit criterion:
  "Claude Skill installs from marketplace; MCP server connects from
  Claude Code + Codex + Gemini CLI". SPEC-SKILL-001 ships the
  marketplace half of that criterion; SPEC-MCP-001 ships the multi-host
  MCP server connectivity half.

  Companion artifacts:
  - `.moai/specs/SPEC-SKILL-001/research.md` — Phase 0.5 research
    (12 sections: terminology, manifest schema, SKILL.md frontmatter,
    MCP bundling strategies, tool passthrough, onboarding UX, versioning,
    marketplace submission, security review, 10 open questions, 10
    risks, references)
  - `.moai/specs/SPEC-SKILL-001/plan.md` — phased implementation plan

  12 EARS REQs (3 × P0 + 7 × P1 + 2 × P2) + 6 NFRs + 1 plugin
  artifact + 1 SKILL.md + 1 .mcp.json + 1 README. Methodology: DDD
  (the plugin authors against an existing MCP server contract;
  characterization tests on plugin install / load semantics).
  Coverage target 85%. Harness: standard. Owner: builder-skill.

---

## 1. Overview

SPEC-SKILL-001 packages the SPEC-MCP-001 MCP server into a
distributable Claude Code plugin so that end users — research-heavy
engineers, product leads, Korean analysts, team leads (per
`.moai/project/product.md` §3 personas) — can install Universal Search
from a marketplace and immediately ask Claude research questions that
get sourced, cited answers.

The plugin is the **install funnel** for Universal Search on the
Claude Code / Claude Desktop surface. Without it, every user must
manually edit `~/.claude.json` to add the `usearch-mcp` server entry —
a friction point that defeats the V1 promise of a polished surface.

### 1.1 What ships

A single Claude Code plugin named `usearch`, structured as:

```
usearch/
├── .claude-plugin/
│   └── plugin.json              [manifest: name, version, description, ...]
├── .mcp.json                    [MCP server config: launches usearch-mcp stdio]
├── skills/
│   └── usearch/
│       └── SKILL.md             [description + body teaching Claude when to invoke]
├── README.md                    [install guide, compat matrix, troubleshooting]
├── LICENSE                      [Apache-2.0]
└── docs/
    └── screenshots/             [marketplace listing assets]
```

Distribution path:

1. Plugin source lives in a public GitHub repo (location _TBD_ —
   research §10 Q3).
2. Plugin submitted to `claude-plugins-community` marketplace.
3. Users discover via `/plugin marketplace browse` in Claude Code.
4. Users install via `/plugin install @claude-community/usearch`.
5. Plugin's `.mcp.json` launches user-installed `usearch-mcp` binary.
6. Skill's `description` triggers Claude to call the MCP tools when
   the user asks a research-style question.

### 1.2 Motivation

Without the Skill plugin:

- Users must edit `~/.claude.json` by hand, adding ~20 lines of MCP
  config. This is friction enough that ~80% of potential users will
  not adopt the tool (per typical OSS onboarding funnel rates).
- There is no discoverability — Universal Search has no presence in
  the Claude Code plugin manager, so existing Claude Code users will
  not find it.
- There is no versioning story for the integration glue — config
  changes can't be shipped as updates.
- The M9 exit criterion "Claude Skill installs from marketplace" is
  blocked.

With the Skill plugin:

- Single-command install (`/plugin install ...`).
- Plugin appears in marketplace browse → discoverability for the
  general Claude Code audience.
- Skill description triggers automatic tool invocation — users do not
  even need to know "usearch" exists; they just ask research questions
  and Claude routes to the MCP server automatically.
- Plugin version-bumps ship config improvements without requiring
  binary upgrades.

### 1.3 Forward-compatibility commitments

This SPEC commits to consuming — never re-implementing — capabilities
shipped by sibling SPECs:

- **SPEC-MCP-001**: the plugin's `.mcp.json` launches the
  `usearch-mcp` binary; tool inventory is whatever MCP-001's
  `tools/list` returns. The plugin does not re-declare tools.
- **SPEC-CLI-002**: `usearch config init` is the canonical onboarding
  wizard cited by the plugin README. The plugin does not re-implement
  config UX.
- **SPEC-AUTH-001** (M6, draft): when HTTP transport with JWT auth
  ships, the plugin README adds a `.mcp.json` swap snippet. The plugin
  itself does not handle auth flows.
- **SPEC-DEPLOY-001** (M9): the plugin README points to the canonical
  binary install method (brew tap / Go install / container — _TBD_,
  research §10 Q5).
- **SPEC-DOC-001** (M9): plugin README cross-links to the user docs
  site once DOC-001 publishes.

### 1.4 Pinned architectural decisions

The 8 decisions in HISTORY's "Pinned decisions" block are restated
here as constraints binding §2 requirements. They are not re-litigated.

---

## 2. EARS Requirements

### 2.1 Plugin Artifact Module

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-SKILL-001** | Ubiquitous | The plugin artifact SHALL be a directory containing a `.claude-plugin/plugin.json` manifest, a `skills/usearch/SKILL.md` skill file, a `.mcp.json` MCP server config, a `README.md` install guide, and a `LICENSE` file. The plugin SHALL pass `claude plugin validate` with zero errors before any marketplace submission. The plugin directory SHALL NOT contain bundled `usearch-mcp` binaries (Strategy A is explicitly excluded per HISTORY D2). | P0 | `claude plugin validate ./usearch/` exits 0; file inventory test asserts the required set is present and no `bin/usearch-mcp-*` paths exist. |
| **REQ-SKILL-002** | Ubiquitous | The `.claude-plugin/plugin.json` manifest SHALL declare: `name: "usearch"` (kebab-case, matches the binary name per HISTORY D4), `version` (explicit semver string matching the parent project's release line — e.g., `"1.0.0"` at V1 release), `displayName: "Universal Search"`, `description` (one-line marketplace listing text, ≤ 250 chars per Anthropic menu display recommendation), `author` object, `license: "Apache-2.0"`, `repository` URL, `homepage` URL, and `keywords` array including at minimum `["research", "search", "citations", "korean", "deep-research", "mcp", "rag"]`. | P0 | Manifest schema test asserts all required fields present and field values match the documented format. |
| **REQ-SKILL-003** | Ubiquitous | The plugin SHALL ship under the **Apache-2.0** license (matching the parent project license target per `.moai/project/product.md` §8). The `LICENSE` file SHALL contain the standard Apache-2.0 text with copyright line matching the parent project's NOTICE file. The `plugin.json:license` field SHALL be the SPDX identifier `"Apache-2.0"`. | P0 | LICENSE file SHA matches the parent repo's LICENSE; SPDX field validates against the SPDX license list. |

### 2.2 Skill Discovery Module

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-SKILL-004** | Ubiquitous | The `skills/usearch/SKILL.md` file SHALL declare a YAML frontmatter using standard Anthropic conventions only (per HISTORY D3): `description` field is REQUIRED and SHALL be ≤ 500 characters. The description SHALL name concrete trigger conditions covering at least: (a) research-style question, (b) citations needed, (c) multi-source synthesis (web/social/academic/Korean), (d) long-form deep-research reports, (e) team-shared query memory. MoAI-internal extension fields (`metadata.*`, `progressive_disclosure.*`, `triggers.*`) SHALL NOT appear in this SKILL.md (those are reserved for `.claude/skills/` internal use, not marketplace-distributed plugins). | P0 | Frontmatter schema test asserts only allowed fields present; description length ≤ 500 chars; description contains the documented trigger condition keywords (a)-(e). |
| **REQ-SKILL-005** | Event-Driven | WHEN a user asks Claude a question whose intent matches the SKILL.md description triggers (research / citation / deep / Korean keywords or paraphrases), Claude SHALL invoke the appropriate MCP tool exposed by the connected `usearch-mcp` server (`search` for basic synthesis, `deep_research` for long-form reports, `list_sources` for adapter discovery, `get_citation` for citation drill-down). The SKILL.md body SHALL include a "Tool Selection Guide" section that documents the rule of thumb for choosing each tool, so Claude's invocation matches user intent. | P1 | Manual interaction test from a fresh Claude Code session: 5 sample queries (3 research-style, 1 deep-research, 1 citation drill-down) trigger the correct tool ≥ 4/5 times; failure of this gate triggers description-tuning iteration. |
| **REQ-SKILL-006** | Optional | WHERE the user's query contains Korean characters OR explicitly references Korean topics (e.g., "Naver news", "한국어로", "Korean sources"), the SKILL.md body SHALL document that the underlying MCP server auto-routes to Korean-locale adapters (Naver, Daum, KoreaNewsCrawler, Korean RSS) via the SPEC-IR-001 intent router — no special tool flag or argument is required from Claude. | P1 | SKILL.md body inspection test asserts the Korean-routing note is present in the "Tool Selection Guide" section. |

### 2.3 MCP Server Wiring Module

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-SKILL-007** | Ubiquitous | The plugin's `.mcp.json` SHALL declare a single MCP server entry under key `usearch` with `command: "usearch-mcp"` (resolved from the user's PATH per HISTORY D2 Strategy B), `args: ["--transport", "stdio"]` (matches SPEC-MCP-001 REQ-MCP-004 default), and NO `env` block embedding secrets. The configuration SHALL launch the user-installed `usearch-mcp` binary as a subprocess; the plugin SHALL NOT bundle the binary itself. | P0 | `.mcp.json` schema validation; integration test launches Claude Code with the plugin loaded, asserts MCP server status is "connected" when `usearch-mcp` is on PATH, asserts clear error message in MCP server status pane when binary is missing. |
| **REQ-SKILL-008** | Optional | WHERE the operator deploys a team-shared HTTP transport server (SPEC-MCP-001 REQ-MCP-005), the plugin's README SHALL document a `.mcp.json` swap snippet that replaces the stdio entry with `type: "http"`, `url: "<team-endpoint>"`, `headers: { "Authorization": "Bearer ${USEARCH_TOKEN}" }` using env-var substitution. The README SHALL warn against committing `.mcp.json` with literal tokens. _TBD_ until plan: confirm `.mcp.json` HTTP MCP server entry schema against Anthropic plugin docs in run phase. | P1 | README inspection test asserts the HTTP swap snippet exists and includes the env-var substitution pattern and the security warning. |
| **REQ-SKILL-009** | Event-Driven | WHEN the `usearch-mcp` server returns a tool-call error code from the SPEC-MCP-001 REQ-MCP-016 namespace (e.g., `-32002 usearch.unauthorized`, `-32000 usearch.cap_exceeded`, `-32007 usearch.citation_not_found`), the SKILL.md body SHALL document for Claude how to surface the error to the user with an actionable next step (e.g., for `unauthorized`: "Run `usearch config set auth.user_id <id>` and retry"; for `cap_exceeded`: "Daily limit reached, resets at <time>"). The Skill SHALL NOT attempt to parse or rewrite the error structure — it relies on Claude reading the error data fields and the SKILL.md guidance to respond. | P1 | SKILL.md body inspection test asserts an "Error Handling" section exists naming at least the three error codes above with example responses. |

### 2.4 Onboarding & Versioning Module

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-SKILL-010** | Ubiquitous | The plugin's `README.md` SHALL document a "Quick Start" section listing the prerequisite steps in exact order: (1) install `usearch-mcp` binary (canonical install method per SPEC-DEPLOY-001 — _TBD_), (2) run `usearch config init` to set endpoint and auth (per SPEC-CLI-002 REQ-CLI2-012), (3) install the plugin via `/plugin install` from the configured marketplace. The README SHALL include a "Compatibility" section publishing the minimum required `usearch-mcp --version` for each plugin version. | P1 | README inspection test asserts both sections exist; Quick Start steps are numbered and reference the documented commands; Compatibility section has at least one entry. |
| **REQ-SKILL-011** | Ubiquitous | The plugin SHALL use explicit semantic versioning in `plugin.json:version`. The version SHALL follow the parent project's release line: the plugin's `MAJOR.MINOR` SHALL match the universal-search release that introduced it (e.g., V1.0.0 → plugin `1.0.0`; V1.1.0 → plugin `1.1.0`). Patch versions are independent (plugin `1.0.3` may exist with parent V1.0.0). The plugin SHALL NOT rely on the git-commit-SHA fallback (per Anthropic plugin docs version management); `version` is always explicit. | P1 | Version string parses as semver; CI gate asserts `plugin.json:version` is set on every release tag. |
| **REQ-SKILL-012** | Optional | WHERE the user installs the plugin via the `claude-plugins-community` marketplace, the marketplace listing SHALL include: 3-5 screenshots demonstrating example invocations (basic search with citations, `/deep` STORM-style report, Korean-locale query, citation drill-down via `get_citation`), `keywords` array (REQ-SKILL-002), `displayName` "Universal Search", and a license badge. _TBD_ until plan: screenshot capture environment + style guide (research §10 Q9). | P2 | Marketplace submission preflight test asserts screenshots directory contains ≥3 PNG files; listing JSON contains all required fields. |

---

## 3. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| **NFR-SKILL-001** | Plugin artifact size cap | The plugin directory (excluding `.git/` and any `node_modules/` artifacts but including `docs/screenshots/`) SHALL be ≤ 5 MB. This budget supports the manifest + SKILL.md + .mcp.json + README + LICENSE + screenshots, and explicitly forbids bundled binaries (REQ-SKILL-007). Marketplace catalogs typically penalise large plugins; 5 MB is the comfortable floor for the assets in scope. CI gates this on every plugin release. |
| **NFR-SKILL-002** | First-run latency | The wall-clock time from `/plugin install usearch` completion to the first successful tool call SHALL be: (a) ≤ 5 seconds when `usearch-mcp` is already on PATH and configured (covers MCP handshake + tool discovery + invocation), OR (b) failure with a clear actionable error message (REQ-SKILL-009 mapping) when `usearch-mcp` is missing or misconfigured. Measured by a manual install-test on each supported OS (macOS, Linux) before V1 ship. |
| **NFR-SKILL-003** | Description token cost | The SKILL.md `description` field SHALL be ≤ 500 characters (≤ ~125 tokens). Claude loads every installed plugin's skill description on every turn; oversized descriptions inflate per-turn token cost for users with many plugins. Compliance gated by a lint test on the SKILL.md frontmatter. |
| **NFR-SKILL-004** | Plugin install reversibility | The plugin SHALL be cleanly removable via `/plugin uninstall usearch` with no residual files outside the plugin directory and no global state changes (no PATH modifications, no shell rc edits, no system service registration). Verified by a snapshot-diff test: capture filesystem state pre-install, install, uninstall, diff — outside the plugin directory and Claude Code's plugin state file, no diff is permitted. |
| **NFR-SKILL-005** | No secrets in plugin repo | The plugin source tree SHALL NOT contain any literal API key, OAuth token, bearer token, password, or other secret. `.mcp.json` SHALL use env-var substitution (`${USEARCH_TOKEN}`) for any auth headers. CI runs a secret scanner (e.g., `gitleaks`) gated to zero findings per release. |
| **NFR-SKILL-006** | Cross-host portability claim accuracy | The plugin's README SHALL be accurate about supported hosts: claim **Claude Code** and **Claude Desktop** support (since both consume Claude Code plugins). Other MCP hosts (Codex CLI, Gemini CLI) are NOT served by this plugin — those hosts install `usearch-mcp` directly via their own MCP config (per SPEC-MCP-001 Phase 7 docs). README MUST NOT claim otherwise. Linter asserts no "Codex" or "Gemini" install instructions appear in the plugin README install section (they may appear in a "Other Hosts" link-out section pointing to the MCP-001 docs). |

---

## 4. Exclusions (What NOT to Build)

[HARD] This SPEC explicitly excludes the following items. Each has a
known destination SPEC, rationale, or follow-up; this list prevents
scope creep into SKILL-001.

- **Bundling `usearch-mcp` binaries inside the plugin** (Strategy A from
  research §4.1). → Excluded for V1 due to plugin-size, cross-platform
  build matrix, and code-signing burden. May revisit if marketplace UX
  feedback warrants. Distribution of the binary is delegated to
  SPEC-DEPLOY-001.

- **Skill plugins for hosts other than Claude Code / Claude Desktop**
  (Gemini CLI, Codex CLI, OpenAI Assistants, etc.). → Those hosts
  consume MCP servers via their own host config; they do not use Claude
  Code plugins. Users of those hosts install `usearch-mcp` directly per
  SPEC-MCP-001 Phase 7 host-specific instructions. SPEC-SKILL-001 V1 is
  Claude Code plugin only (HISTORY D8).

- **Wrapping the `usearch` CLI** (SPEC-CLI-002) in the Skill body or
  bundling CLI invocation scripts. → The MCP server IS the universal
  contract per `.moai/project/tech.md` §1 principle 7. Wrapping the CLI
  inside a Skill would duplicate the contract and add a parallel
  surface that diverges from the MCP path. CLI remains the
  shell-scripting surface; Skill remains the LLM-host surface.

- **Skill arguments for configuration** (e.g., `/usearch:configure
  endpoint=...`). → Configuration is one-time setup handled by
  `usearch config init` (SPEC-CLI-002 REQ-CLI2-012). The Skill is
  model-invoked, not user-typed; cluttering it with a `:configure`
  surface conflicts with its discovery-oriented design (research §6.3).

- **OAuth flow inside the plugin** for HTTP MCP transport bootstrap. →
  Plugin README documents how to obtain a JWT from the user's
  SPEC-AUTH-001 OIDC provider; plugin itself accepts an already-issued
  token via `${USEARCH_TOKEN}` env-var substitution in `.mcp.json`.

- **Telemetry / usage analytics phoning home from the plugin**. →
  Privacy-first stance matching the parent project's "auditable
  self-hosted" positioning per product.md §1. The MCP server already
  ships observability metrics for the operator (SPEC-OBS-001);
  Anthropic's marketplace catalog has its own install-count metrics.
  Plugin adds nothing on top.

- **Plugin-side handshake version check between plugin and
  `usearch-mcp` binary**. → Reserved as a future enhancement (research
  §7). V1 surfaces version mismatch as MCP tool errors or via the
  README compatibility matrix. Adding plugin-side process execution to
  check `--version` requires careful scoping against Anthropic plugin
  security model — out of scope for V1.

- **Custom marketplace hosting infrastructure**. → V1 targets the
  Anthropic `claude-plugins-community` marketplace via the standard
  submission form. A self-hosted `marketplace.json` is documented as a
  fallback (research §8.3) but not deployed as primary infrastructure.

- **Submission to `claude-plugins-official` (curated marketplace)**. →
  Official marketplace is Anthropic's discretion; there is no
  application process. SPEC fixes target as community marketplace
  (HISTORY D7). If Anthropic later invites the plugin to official, no
  plugin changes required — same artifact, different catalog pin.

- **MoAI-internal `metadata.*` / `progressive_disclosure.*` /
  `triggers.*` SKILL.md frontmatter extensions**. → Excluded per
  HISTORY D3. Those are MoAI workflow internals. The marketplace
  plugin must be portable to non-MoAI Claude Code environments.

- **Web UI screenshot generation pipeline**. → Screenshots come from
  manual capture against running deployments (V1 Web UI per
  SPEC-UI-001, or terminal output from CLI). Auto-generation pipeline
  is out of scope.

- **GitHub Issue tracking on this SPEC** (`issue_number: 0`). → Per
  project pattern for M7 surfaces SPECs.

- **Multi-language SKILL.md** (Korean + English description, etc.). →
  V1 SKILL.md is English-only. Claude can respond in any language at
  runtime regardless of the SKILL.md language; the description's job
  is to match keywords, and Anthropic plugin docs do not document
  multi-language description support. Korean adapter routing happens
  server-side via SPEC-IR-001 regardless of plugin description
  language.

- **`bin/` directory in the plugin** (per Anthropic plugin docs,
  `bin/` contents are added to Bash tool PATH when plugin is enabled).
  → Plugin has no executables to ship beyond what `usearch-mcp`
  already provides on the user's PATH. Adding `bin/` would create the
  Strategy A coupling problem (excluded above).

- **`hooks/` directory in the plugin** (per Anthropic plugin docs,
  `hooks/hooks.json` triggers on Claude Code events). → Plugin has no
  hook semantics — the MCP server handles all behaviour. Adding hooks
  would extend the surface beyond "wraps MCP server" per the roadmap
  scope.

- **`agents/` directory in the plugin** (custom sub-agents). → Plugin
  has no custom agents to ship. The MCP server tools (`search`,
  `deep_research`, etc.) are agents enough — they encapsulate the
  multi-agent pipeline server-side (SPEC-DEEP-002 Researcher-Reviewer-
  Writer-Verifier).

- **`settings.json` default activation** of a plugin-provided agent. →
  No plugin-provided agents (above); no default activation.

---

## 5. Acceptance Criteria

Per-REQ acceptance summaries are documented in §2 alongside each
requirement. The full Given-When-Then scenarios are owned by
`.moai/specs/SPEC-SKILL-001/acceptance.md` (to be authored alongside
this SPEC's plan-auditor cycle). The scenario index:

| Scenario | Description | Coverage |
|----------|-------------|----------|
| §5.1 | Local install end-to-end: user has `usearch-mcp` on PATH and `~/.config/usearch/config.toml` configured; `/plugin install usearch` completes; Claude Code MCP server status pane shows `usearch` as "connected"; first research query triggers `search` tool and returns a cited answer. | REQ-SKILL-001, 004, 005, 007 |
| §5.2 | Missing binary failure mode: user installs plugin without `usearch-mcp` on PATH; MCP handshake fails; Claude Code surfaces "command not found" error in MCP server status pane; SKILL.md description does not falsely promise functionality. | REQ-SKILL-007, NFR-SKILL-002 |
| §5.3 | Korean-locale query: user asks "한국어로 최근 AI 뉴스 검색해줘"; Skill description triggers; Claude invokes `search` tool; MCP server's IR-001 routes to Naver+Daum+KR-RSS adapters; result returned with Korean sources. | REQ-SKILL-005, REQ-SKILL-006 |
| §5.4 | Deep research invocation: user asks "give me a comprehensive report on quantum computing for the team"; Skill body's Tool Selection Guide steers Claude to `deep_research` tool; STORM-style report returned. | REQ-SKILL-005 (deep), SPEC-MCP-001 REQ-MCP-009 |
| §5.5 | Quota error surfacing: user repeatedly invokes `deep_research`, hits SPEC-DEEP-004 cap; MCP returns `-32000 usearch.cap_exceeded`; Claude reads SKILL.md "Error Handling" guidance and tells the user "Daily deep-research limit reached, resets at <reset_at>". | REQ-SKILL-009, SPEC-MCP-001 REQ-MCP-016 |
| §5.6 | Citation drill-down: user asks "tell me more about source [3] from your last answer"; Claude invokes `get_citation` tool with the prior response's `doc_id`; full NormalizedDoc returned and rendered. | REQ-SKILL-005 (citation), SPEC-MCP-001 REQ-MCP-012 |
| §5.7 | Plugin uninstall is clean: snapshot filesystem; install plugin; uninstall plugin; diff — no residual files outside plugin directory and Claude Code's plugin state file. | NFR-SKILL-004 |
| §5.8 | Plugin manifest validation: `claude plugin validate ./usearch/` exits 0 against the shipped plugin directory; required fields present. | REQ-SKILL-001, REQ-SKILL-002 |
| §5.9 | Marketplace listing preflight: screenshots, keywords, description, license, repository URL all populated and pass the marketplace submission lint. | REQ-SKILL-012 |
| §5.10 | HTTP transport swap: README's `.mcp.json` swap snippet works against a deployed team MCP HTTP server (when SPEC-AUTH-001 is available); env-var token substitution functions; no token leak. | REQ-SKILL-008, NFR-SKILL-005 |

---

## 6. Dependencies & Blocks

### 6.1 Upstream SPEC dependencies (depends_on)

- **SPEC-MCP-001** (draft) — the MCP server this plugin wraps. The
  plugin's `.mcp.json` launches `usearch-mcp`; the Skill body
  references the four tools MCP-001 exposes (`search`,
  `deep_research`, `list_sources`, `get_citation`). The plugin's
  release is gated on MCP-001 reaching `implemented` status — until
  the binary exists, the plugin has nothing to launch. The plugin's
  error-handling guidance (REQ-SKILL-009) consumes MCP-001's
  REQ-MCP-016 error namespace.

- **SPEC-CLI-002** (draft) — `usearch config init` is the canonical
  onboarding wizard cited by the plugin README's Quick Start
  (REQ-SKILL-010). `usearch-mcp --version` (CLI parity) is the
  compatibility-matrix probe. CLI-002 must ship `config init` and
  `--version` in time for plugin release; both are P0/P1 REQs in
  CLI-002.

### 6.2 Related but soft (related)

- **SPEC-AUTH-001 (draft, M6)** — JWT middleware for HTTP MCP
  transport. Plugin README's HTTP swap snippet (REQ-SKILL-008) is
  fully usable only once AUTH-001 ships. Pre-AUTH-001, the snippet
  is documented with a note that JWT integration requires AUTH-001 GA.

- **SPEC-DEPLOY-001 (M9, not yet drafted)** — canonical binary install
  method (brew tap / Go install / container — _TBD_, research §10 Q5).
  Plugin README Quick Start step 1 cites this method. Coordination
  required to land the install method before plugin marketplace
  submission.

- **SPEC-DOC-001 (M9)** — user docs site. Plugin README cross-links to
  the deep-dive docs once DOC-001 publishes. Pre-DOC-001, README is
  self-contained.

- **SPEC-REL-001 (M9)** — V1.0.0 release tag. Plugin version is pinned
  to the parent release line per REQ-SKILL-011. Marketplace
  submission timing relative to V1.0.0 tag is _TBD_ (research §10 Q4).

### 6.3 Downstream blocked SPECs (blocks)

- None. SPEC-SKILL-001 is a leaf in the dependency graph — no
  downstream SPECs depend on it. It is the last surface of M7 per
  roadmap §3 parallelization plan.

### 6.4 External dependencies (run-phase pins)

- Claude Code (≥ v2.1.x) — plugin system + marketplace required.
- `claude plugin validate` CLI command — used as the pre-submission
  gate.
- GitHub repository for plugin source (location _TBD_, research §10
  Q3).
- `gitleaks` or equivalent secret scanner — CI gate per NFR-SKILL-005.

No new direct runtime dependencies beyond what the user already has
(Claude Code + `usearch-mcp` binary).

---

## 7. Files to Create / Modify

### 7.1 Created (estimated; final list owned by run phase)

Plugin artifacts (in the plugin's own repo OR a `tools/claude-skill/`
subdirectory of the main repo — placement _TBD_ per research §10 Q3):

| Marker | Path | Purpose |
|--------|------|---------|
| [NEW] | `.claude-plugin/plugin.json` | Manifest per REQ-SKILL-002. |
| [NEW] | `.mcp.json` | MCP server config per REQ-SKILL-007. |
| [NEW] | `skills/usearch/SKILL.md` | Skill description + body per REQ-SKILL-004, 005, 006, 009. |
| [NEW] | `README.md` | Install guide + compatibility matrix per REQ-SKILL-010. |
| [NEW] | `LICENSE` | Apache-2.0 text per REQ-SKILL-003. |
| [NEW] | `docs/screenshots/*.png` | 3-5 marketplace listing screenshots per REQ-SKILL-012. |
| [NEW] | `.github/workflows/validate.yml` | CI running `claude plugin validate`, secret scanner, plugin-size cap, frontmatter linter. |
| [NEW] | `CHANGELOG.md` | Plugin version history per REQ-SKILL-011. |

### 7.2 Modified (in the parent universal-search repo if monorepo path
chosen)

| Path | Change |
|------|--------|
| `.moai/specs/SPEC-MCP-001/plan.md` | Phase 7 (cross-client integration) may add a "Claude Skill smoke test" line item once SKILL-001 exists. (Not required for MCP-001 ship.) |
| `README.md` (parent repo) | Add "Install via Claude Code" section linking to the Skill plugin marketplace listing. |

### 7.3 Existing — Unchanged

- `cmd/usearch-mcp/main.go` — read-only consumer; plugin launches the
  binary unchanged.
- `cmd/usearch/main.go` — read-only consumer; plugin onboarding cites
  `usearch config init`.
- All `internal/*` packages — untouched.

---

## 8. Open Questions

The SPEC's _TBD_ markers and the research artifact's §10 are the
canonical list. Restated here for plan-auditor convenience:

1. **Hosted MCP endpoint URL** (research §10 Q1) — V1 is self-hosted
   only per product.md; no canonical hosted endpoint exists. Plugin
   defaults to stdio; HTTP endpoint URL is per-team operator config.
   _Resolution_: stdio default; README documents HTTP swap; no shipped
   default URL. **Does not block plan-auditor.**

2. **Auth UX on first install** (research §10 Q2) — currently the
   plugin relies on user having already run `usearch config init`
   before install. Could enhance via MCP error → Claude-surfaced setup
   hint. _TBD_ — finalise after SPEC-MCP-001 error message catalog is
   firm.

3. **Plugin repository hosting** (research §10 Q3) — monorepo
   subdirectory vs dedicated repo. _TBD_ — annotation cycle decision.

4. **Marketplace submission timing vs M9 V1.0.0 tag** (research §10
   Q4) — submit during M8 (parallel review) vs post-tag (reviewed
   version matches release). _TBD_ — decide when M9 ship date is firm.

5. **Canonical `usearch-mcp` binary install method** (research §10
   Q5) — brew tap / Go install / container / Helm. Owned by
   SPEC-DEPLOY-001; plugin README cites once decided. _TBD_ — block
   M9 not SKILL-001 plan.

6. **Plugin namespace prefix** (research §10 Q8) — `usearch` vs
   `universal-search`. SPEC recommends `usearch`; confirm in
   annotation. _TBD_ — annotation cycle.

7. **Screenshot capture environment** (research §10 Q9) — terminal
   output vs Web UI screenshots. _TBD_ — coordinate with SPEC-UI-001
   ship timing.

8. **SKILL.md description text iteration** (research §10 Q10) —
   description text needs real-world trigger testing to confirm it
   fires on the right queries without false positives. _TBD_ — run
   phase iterates with manual interaction tests.

9. **HTTP MCP server entry schema in `.mcp.json`** (REQ-SKILL-008
   _TBD_ note) — confirm `.mcp.json` HTTP entry shape against current
   Anthropic plugin docs before shipping the README swap snippet.

These items do NOT block plan-auditor PASS; they are tagged as known
unresolved scope edges with rationale.

---

## 9. References

External (cited in research.md §12):

- Claude Code plugin creation guide: https://code.claude.com/docs/en/plugins
- Plugin reference / schema: https://code.claude.com/docs/en/plugins-reference
- Skill documentation: https://code.claude.com/docs/en/skills
- Plugin marketplace docs: https://code.claude.com/docs/en/plugin-marketplaces
- Discover and install plugins: https://code.claude.com/docs/en/discover-plugins
- Community marketplace catalog: https://github.com/anthropics/claude-plugins-community
- Submission form: https://claude.ai/settings/plugins/submit
- MCP specification (2025-06-18): https://modelcontextprotocol.io/specification/2025-06-18

Internal (project files):

- `.moai/project/product.md` §3 (personas), §4 (V1 surfaces include
  Claude Skill), §8 (Apache-2.0 license)
- `.moai/project/roadmap.md` §M7 SPEC-SKILL-001 row + §5 M9 exit
  criterion + §3 parallelization plan
- `.moai/specs/SPEC-MCP-001/spec.md` — MCP server contract this plugin
  wraps
- `.moai/specs/SPEC-MCP-001/research.md` — transport tradeoffs that
  inform plugin `.mcp.json` strategy choice
- `.moai/specs/SPEC-MCP-001/plan.md` — Phase 7 cross-client
  verification artifacts the plugin README leverages
- `.moai/specs/SPEC-CLI-002/spec.md` — `config init` onboarding
  wizard + `--version` flag the plugin cites
- `.moai/specs/SPEC-AUTH-001/spec.md` — JWT middleware (forward dep
  for HTTP transport swap)
- `.moai/project/brand/brand-voice.md` — currently `_TBD_`; Skill
  description tone is provisional until brand interview completes
- `.claude/rules/moai/development/skill-authoring.md` — MoAI-internal
  skill conventions (referenced for contrast — plugin uses Anthropic
  standard frontmatter only per HISTORY D3)

---

*End of SPEC-SKILL-001 v0.1.0 (draft).*
