# SPEC-SKILL-001 Research — Claude Skill Marketplace Package

Status: draft companion to spec.md
Author: limbowl via manager-spec
Date: 2026-05-22

This research artifact is the Phase 0.5 deep-dive that informed
SPEC-SKILL-001. It captures the Claude Code plugin/skill distribution
model, packaging tradeoffs, MCP server bundling strategies, marketplace
submission process, install UX options, versioning posture, and the open
questions the SPEC explicitly defers.

---

## 1. Terminology — "Skill" vs "Plugin" in Claude Code

Anthropic's terminology splits the surface into two distinct concepts
that the roadmap entry ("SPEC-SKILL-001 | Claude Skill package | SKILL.md,
install via marketplace, wraps MCP server") collapses into one:

- **Skill** = a model-invocable capability defined by a `SKILL.md` file.
  Lives at `<root>/skills/<skill-name>/SKILL.md`. Triggered by Claude
  based on description keywords. Project-scoped when placed in `.claude/
  skills/` directly; namespaced as `/<plugin>:<skill>` when shipped inside
  a plugin.

- **Plugin** = the **distribution unit**. A directory containing a
  `.claude-plugin/plugin.json` manifest plus any combination of `skills/`,
  `agents/`, `hooks/`, `.mcp.json`, `.lsp.json`, `monitors/`,
  `settings.json`, `bin/`. Plugins are the only thing that can be
  installed via marketplace, versioned, and shared cross-project.

For SPEC-SKILL-001, the "Claude Skill package" deliverable per the
roadmap is therefore a **Plugin** (`usearch-skill` or similar) that
contains:

1. A `SKILL.md` describing when Claude should invoke universal-search.
2. A `.mcp.json` wiring the plugin to the SPEC-MCP-001 server.
3. Optionally: bundled binaries, screenshots, README, license.

The skill itself becomes invocable as `/usearch:search` (namespace
prefix from `plugin.json:name`). The MCP server is what does the actual
work; the Skill is the discovery + onboarding surface.

This SPEC uses **"Skill plugin"** when both concepts apply together, and
distinguishes "Skill" (the SKILL.md surface) from "Plugin" (the
distribution unit) where precision matters.

References:
- https://code.claude.com/docs/en/plugins (plugin creation)
- https://code.claude.com/docs/en/plugins-reference (manifest schema)
- https://code.claude.com/docs/en/discover-plugins (install / marketplace)

---

## 2. Plugin manifest schema (`.claude-plugin/plugin.json`)

The manifest is **optional** (Claude Code auto-discovers components from
default directories if absent) but **required for marketplace
distribution** because the marketplace catalog indexes on its fields.

Required fields:

| Field | Type | Notes |
|-------|------|-------|
| `name` | string | Kebab-case, no spaces, unique. Becomes the skill namespace prefix (e.g., `usearch` → `/usearch:search`). |

Optional but strongly recommended for marketplace:

| Field | Type | Notes |
|-------|------|-------|
| `displayName` | string | Human-readable name shown in plugin manager. |
| `version` | string | Semver. If omitted in git-distributed plugins, the commit SHA is used and **every commit counts as a new version**. SPEC fixes this to explicit semver. |
| `description` | string | Shown in plugin manager + marketplace listing. |
| `author` | object | `{ name, email, url }`. |
| `homepage` | string | URL to docs site. |
| `repository` | string | GitHub URL. |
| `license` | string | SPDX identifier. Apache-2.0 per project-wide license target. |
| `keywords` | string[] | Marketplace discovery. |

Custom component paths (when components live outside default locations):

| Field | Notes |
|-------|-------|
| `skills` | Path to skills directory. Default `./skills/`. |
| `mcpServers` | Path to `.mcp.json` OR inline object. |
| `lspServers` | Path to `.lsp.json`. |
| `hooks` | Path to `hooks.json`. |
| `agents` | Path or path[]. |

For SPEC-SKILL-001 the SPEC fixes default paths to keep the layout
auto-discoverable, with the manifest carrying metadata + version.

Reference: https://code.claude.com/docs/en/plugins-reference (Plugin
manifest schema section).

---

## 3. Skill body (`SKILL.md`) frontmatter conventions

### 3.1 Minimal community / Anthropic convention

Per https://code.claude.com/docs/en/plugins quickstart and
https://code.claude.com/docs/en/skills, the minimum SKILL.md
frontmatter is:

```yaml
---
description: One-line description used by Claude to decide when to invoke.
---
```

`description` is the **only required** field. Claude reads the
description from every loaded skill and decides which to invoke based on
keyword match against the user query. Best practice: include 5-10
relevant keywords naturally in the description.

Optional standard fields per Anthropic docs:

- `disable-model-invocation: true` — only user can invoke (not Claude).
  Useful for skills with side effects.
- `user-invocable: false` — Claude can invoke but skill is hidden from
  `/` menu.
- `allowed-tools` — CSV string restricting tool permissions while skill
  is active.
- `argument-hint` — string shown to user when invoking with arguments.

### 3.2 MoAI-specific extensions

The MoAI project additionally supports (per
`.claude/rules/moai/development/skill-authoring.md`):

- `metadata.version` (quoted string)
- `metadata.category` (foundation / workflow / domain / language /
  platform / library / tool)
- `metadata.status` (active / experimental / deprecated)
- `metadata.tags` (CSV string)
- `progressive_disclosure` (level1_tokens / level2_tokens for budget
  optimization)
- `triggers.keywords` / `triggers.agents` / `triggers.phases` /
  `triggers.languages`

These are MoAI-internal conventions and **not** required by Claude Code
to function. Since SPEC-SKILL-001 ships outside the MoAI workflow (to
external users of universal-search), the SPEC uses **standard Anthropic
frontmatter only** for the SKILL.md, keeping the file portable.

The MoAI extensions are reserved for project-internal skills that ship
in `.claude/skills/`. The universal-search skill plugin ships in its own
repository (see §10 Q3) and follows Anthropic's standard skill
conventions.

### 3.3 Description keyword anchoring

The Skill's `description` field is the **only** signal Claude uses to
decide whether to invoke the skill. Per Anthropic docs, descriptions
that name concrete trigger conditions ("Use when X happens") outperform
generic ones ("Helps with Y").

For SPEC-SKILL-001, the description must surface:

- Domain: research, deep research, citation-backed answer
- Source breadth: web, social, academic, Korean-locale
- Differentiators that disambiguate from built-in WebSearch:
  - Multi-source fanout (vs single Google call)
  - Cited synthesis (vs raw links)
  - Korean-locale support (vs English-only)
  - Team-shared memory (vs single-tenant)
  - `/deep` long-form report option

Concrete draft (refined in spec.md REQ section):

> Universal Search — team-scale research meta-agent. Use when the user
> asks a research-style question that needs (a) citations, (b) breadth
> across web/social/academic/Korean sources, (c) long-form deep-research
> reports, or (d) reuses prior team queries. Calls the Universal Search
> MCP server for sourced synthesis.

---

## 4. MCP server bundling — three strategies

The plugin's `.mcp.json` is a standard MCP server configuration. The
question is **how the plugin connects to a usearch-mcp server**: as a
local subprocess it launches itself, as a user-supplied binary, or as a
remote HTTP endpoint.

### 4.1 Strategy A — bundle the binary (`${CLAUDE_PLUGIN_ROOT}` launch)

Layout:
```
usearch-skill/
├── .claude-plugin/plugin.json
├── .mcp.json
├── skills/usearch/SKILL.md
└── bin/
    ├── usearch-mcp-linux-amd64
    ├── usearch-mcp-linux-arm64
    ├── usearch-mcp-darwin-amd64
    ├── usearch-mcp-darwin-arm64
    └── usearch-mcp-windows-amd64.exe
```

`.mcp.json`:
```json
{
  "mcpServers": {
    "usearch": {
      "command": "${CLAUDE_PLUGIN_ROOT}/bin/usearch-mcp-${platform}",
      "args": []
    }
  }
}
```

**Pros**:
- Zero install steps beyond `/plugin install` — works out of the box.
- Plugin version pin = MCP server version pin (single artifact).

**Cons**:
- Plugin size balloons (NFR-MCP-006 binary cap ≤ 40 MB × 5 platforms =
  200 MB plugin). Marketplace catalogs typically cap plugin size.
- Cross-platform binary distribution requires CI build matrix.
- Updates require shipping new binaries on every MCP-001 release.
- Code signing / notarization issues on macOS / Windows.
- Variable substitution for `${platform}` is **not standard MCP** — the
  `.mcp.json` syntax does not natively expand platform-specific paths.
  Would need a shell wrapper script in `bin/` that detects platform.

### 4.2 Strategy B — reference a separately installed binary (`PATH` launch)

Layout:
```
usearch-skill/
├── .claude-plugin/plugin.json
├── .mcp.json
└── skills/usearch/SKILL.md
```

`.mcp.json`:
```json
{
  "mcpServers": {
    "usearch": {
      "command": "usearch-mcp",
      "args": ["--transport", "stdio"]
    }
  }
}
```

User installs `usearch-mcp` via `brew install universal-search` or
`go install` or container or Helm chart (per SPEC-DEPLOY-001), then
installs the Skill plugin.

**Pros**:
- Plugin is tiny (a SKILL.md + manifest + .mcp.json — under 10 KB).
- Server upgrades happen independently of plugin upgrades.
- Each user / org can deploy the server flavour that fits their
  infrastructure (local binary, container, k8s).
- Standard MCP convention — matches every official MCP plugin pattern
  (GitHub, Linear, Sentry plugins all launch user-installed binaries).

**Cons**:
- Two-step install: install server, install plugin.
- Onboarding error surface: missing binary, version mismatch, PATH
  issues.
- User needs to authenticate the server separately (config file,
  env vars, or `usearch config set`).

### 4.3 Strategy C — connect to a hosted HTTP endpoint

Layout: same as Strategy B (no bundled binary).

`.mcp.json`:
```json
{
  "mcpServers": {
    "usearch": {
      "type": "http",
      "url": "https://search.team.example.com/mcp",
      "headers": {
        "Authorization": "Bearer ${USEARCH_TOKEN}"
      }
    }
  }
}
```

(Note: the exact `.mcp.json` shape for HTTP MCP servers — `type`, `url`,
`headers` — is per the MCP spec but the SDK and `.mcp.json` schema for
plugins is `_TBD_` — Anthropic plugin docs do not yet document HTTP
MCP server entries explicitly. Verify in run phase against the
modelcontextprotocol/specification 2025-06-18 transport section.)

**Pros**:
- Zero local install — pure config wiring.
- Centralised auth, ops, quota enforcement on the server side.
- Aligns with team-deployed usearch-mcp HTTP transport
  (SPEC-MCP-001 REQ-MCP-005).

**Cons**:
- Requires a deployed HTTP MCP server (SPEC-MCP-001's opt-in HTTP
  transport, locked behind SPEC-AUTH-001 for JWT auth).
- Universal Search V1 is **self-hosted only** per product.md §4 — there
  is no canonical hosted endpoint. Each team brings their own.
- Endpoint URL must be configured per-team — requires user
  configuration at install time.

### 4.4 Recommendation

SPEC-SKILL-001 V1 recommends **Strategy B (default) with Strategy C
support** documented:

- Default `.mcp.json` launches `usearch-mcp` from PATH via stdio (the
  SPEC-MCP-001 V1 default transport).
- README documents how to swap `.mcp.json` for HTTP mode pointing at a
  team-deployed HTTP endpoint when SPEC-AUTH-001 ships.
- Strategy A (bundled binary) is **explicitly excluded** from V1 for the
  plugin-size + cross-platform binary distribution reasons above. May
  revisit if marketplace UX feedback warrants.

The exclusion + recommendation is restated as a HARD constraint in
spec.md §4.

---

## 5. Tools the Skill exposes (passthrough from MCP server)

Per SPEC-MCP-001 REQ-MCP-007, the MCP server exposes four tools:
`search`, `deep_research`, `list_sources`, `get_citation`.

When the Skill plugin's MCP server is connected, Claude Code
automatically discovers these tools and lists them in its toolkit. The
plugin does **not** need to re-declare them in `plugin.json` — MCP tool
discovery is dynamic per the MCP `tools/list` handshake.

What the Skill's `SKILL.md` **does** need to do:

1. Tell Claude **when** to invoke these tools (description field).
2. Optionally include a body that gives Claude examples of when each
   tool fits (basic search vs deep research vs citation lookup).

Per Anthropic docs the skill body is loaded into Claude's context when
the description matches. The body is a free-form Markdown document with
no required structure — it serves as system-prompt material for the
session.

For SPEC-SKILL-001, the SKILL.md body documents:

- When to call `search` vs `deep_research` (rule of thumb: deep when the
  user asks for "report" / "comprehensive" / "thorough" / "for the team"
  / explicit `/deep` mention).
- When to call `list_sources` (when the user asks "what sources do you
  have" / "which adapters are available").
- When to call `get_citation` (when the user wants details on a specific
  source from a prior answer).
- Korean-locale routing hint (when the query contains Korean characters
  or asks about Korean topics, Universal Search auto-routes to Naver +
  Daum + Korean RSS — no special tool flag needed; the IR-001 router
  handles it server-side).

---

## 6. First-run onboarding — auth + endpoint configuration

The auth UX divides cleanly by MCP transport:

### 6.1 stdio transport (V1 default)

Per SPEC-MCP-001 §6.1 (research) the stdio transport has **no
authentication** — trust boundary is the OS process. The user has
already installed `usearch-mcp` locally (Strategy B), and the binary
reads its config from `~/.config/usearch/config.toml` (per SPEC-CLI-002
REQ-CLI2-007 layout — the same config file shared between CLI and MCP
server).

Onboarding steps for the user installing the Skill plugin:

1. `brew install universal-search` (or equivalent install).
2. `usearch config init` — interactive wizard sets endpoint, auth headers,
   default sources (per SPEC-CLI-002 REQ-CLI2-012).
3. `claude` → `/plugin install usearch-skill` (or similar marketplace
   command).
4. Open a conversation → Claude auto-invokes the `search` tool when
   appropriate.

The Skill plugin's README documents steps 1-2 as **prerequisites** for
step 3. The plugin itself does not prompt for auth — it just launches
the local binary which reads its own config.

### 6.2 HTTP transport (V1 opt-in / V1.1+ default for team deployments)

User edits `.mcp.json` after install to switch from stdio to HTTP:
```json
{
  "mcpServers": {
    "usearch": {
      "type": "http",
      "url": "https://search.myteam.example.com/mcp",
      "headers": { "Authorization": "Bearer eyJ..." }
    }
  }
}
```

Token acquisition is out of scope for the Skill plugin — it is the
user's responsibility to obtain a JWT from the SPEC-AUTH-001 OIDC IdP
configured by their org's universal-search deployment.

V1 README documents this swap as a "team deployment" subsection.

### 6.3 Skill arguments as a config UX (rejected)

An alternative UX is to make the Skill take arguments like
`/usearch:configure endpoint=...`. This is rejected because:

- The plugin is meant to be **invoked by Claude**, not by the user
  directly typing `/usearch:` commands.
- Configuration is one-time setup; cluttering the skill surface with a
  configure command is poor UX.
- `usearch config init` (CLI-002) already provides the wizard.

`_TBD_` per spec.md §6 — re-evaluate after first round of community
plugin user feedback.

---

## 7. Versioning + update flow

Per https://code.claude.com/docs/en/plugins-reference#version-management:

- If `plugin.json:version` is **set** to a semver string, users only
  receive updates when the version is bumped. The marketplace catalog
  pins commit SHA per version.
- If `version` is **absent** and the plugin is distributed via git, the
  commit SHA is used and every commit produces a new "version" that
  users may pull.

For SPEC-SKILL-001 we **always set semver** in `plugin.json:version`,
following the parent project's semver line (universal-search V1.0.0 →
plugin V1.0.0, etc.). This gives users explicit control over upgrades
and matches the SPEC-REL-001 (M9) release cadence.

Coupling between MCP server version and Skill plugin version:

- The plugin's `.mcp.json` launches `usearch-mcp` from PATH; the binary
  version is whatever the user has installed.
- Plugin version bumps happen when the **Skill plugin metadata** changes
  (SKILL.md description, README, plugin.json keywords, .mcp.json config).
- Binary version bumps happen when **MCP server behaviour** changes
  (handled by `brew upgrade universal-search` etc., decoupled from
  plugin updates).
- The plugin README documents a **compatibility matrix** mapping plugin
  version → minimum / maximum supported `usearch-mcp --version` output.
  The Skill body can mention "requires usearch-mcp ≥ 1.0.0" so Claude
  can hint at it during onboarding errors.

A future enhancement could have the plugin run `usearch-mcp --version`
at handshake time and refuse to start with a clear error message, but
that requires plugin-side process execution which is `_TBD_`.

---

## 8. Marketplace listing + submission process

Per https://code.claude.com/docs/en/plugins (community marketplace
section):

### 8.1 Two Anthropic-hosted marketplaces

- **`claude-plugins-official`** — curated by Anthropic. Available
  automatically in every Claude Code installation. No application
  process; Anthropic decides what to include. Listing here would enable
  a CLI install prompt (`claude plugin hints`).
- **`claude-plugins-community`** — public third-party marketplace.
  Users add it via `/plugin marketplace add anthropics/claude-plugins-
  community`. Submissions go through a review pipeline.

### 8.2 Community marketplace submission steps

1. Develop plugin in a public GitHub repo (project's own repo or a
   dedicated one — see §10 Q3).
2. Run `claude plugin validate` locally to confirm manifest, file
   layout, and security checks pass.
3. Submit via one of:
   - https://claude.ai/settings/plugins/submit
   - https://platform.claude.com/plugins/submit
4. Anthropic review pipeline runs validation + automated safety
   screening.
5. On approval, plugin is pinned to a specific commit SHA in the
   `anthropics/claude-plugins-community` catalog
   (`.claude-plugin/marketplace.json`).
6. CI bumps the SHA pin on subsequent pushes.
7. Public catalog syncs **nightly** from the review pipeline (i.e.,
   approval-to-install lag is up to 24h).

### 8.3 Third-party marketplaces

Plugins can also be installed via custom marketplaces (`/plugin
marketplace add <git-url>`). This is the fallback path if Anthropic
review takes too long for the V1 launch window — Universal Search could
host its own `marketplace.json` in the same repo as the plugin and
instruct users to add it manually.

The SPEC's M9 exit criterion ("Claude Skill installs from marketplace")
does **not** specifically require the Anthropic community marketplace —
any marketplace counts as long as users can `/plugin install`. The SPEC
fixes the target as community marketplace with **fallback to
self-hosted marketplace** if review is gating V1 ship.

### 8.4 Marketplace listing required fields

The `marketplace.json` schema (per docs §plugin marketplaces) requires:
- `plugins[].name` — matches plugin.json
- `plugins[].source` — `{ type: "git", repository: "...", commit: "..." }`
- `plugins[].description` — listing text
- `plugins[].keywords` — search tags

Optional:
- `plugins[].displayName`
- `plugins[].license`
- `plugins[].author`
- `plugins[].screenshots` — array of image URLs (rendered in the
  install UX)

Universal Search will need at minimum:
- 3-5 screenshots showing example invocations (`search` result with
  citations, `deep_research` STORM report, Korean query routing).
- License: Apache-2.0.
- Repository URL.
- Description: keyword-rich (research, citation, Korean, team, MCP, ...).

---

## 9. Security review surface for marketplace submission

Anthropic's review pipeline ("automated safety screening" per docs)
screens plugins for:

1. **No arbitrary code execution at install time** — plugins are
   declarative; binary launches happen at MCP server start, not install.
   Plugin satisfied: we have no install-time scripts.
2. **No exfiltration of user credentials** — `.mcp.json` may not embed
   tokens; users provide their own. Plugin satisfied: tokens live in
   user's local config or env vars, never in the plugin repo.
3. **No bundled binaries from unverified sources** — if Strategy A
   (§4.1) is chosen, binaries must be reproducibly built from source
   and signed. Plugin satisfies by **not bundling binaries** in V1
   (Strategy B).
4. **Manifest claims match behaviour** — description / keywords cannot
   misrepresent the plugin's function. Plugin satisfies: SKILL.md
   description is honest about what the MCP server does.

Forward-looking risks to monitor:

- If V1.1 adds `search_team_memory` tool that reads from a team-shared
  index, the SKILL.md description must disclose this so Claude doesn't
  inadvertently leak cross-team queries through the search tool.
- If a hosted endpoint is ever offered (Strategy C with Anthropic-
  managed default URL), the privacy disclosures must be documented in
  the plugin README.

---

## 10. Open questions (deferred decisions)

These items are explicitly UNRESOLVED at SPEC-draft time. The SPEC
marks each with `_TBD_` and a rationale; they do not block plan-auditor
PASS.

1. **Hosted MCP endpoint URL**: Universal Search V1 is self-hosted only
   per product.md §4. There is no canonical hosted endpoint to ship in
   the default `.mcp.json`. _TBD_ — confirmed self-hosted; plugin
   defaults to stdio (Strategy B), HTTP endpoint configuration is
   user-provided per team deployment.

2. **Auth UX on first install**: Strategy B (stdio + local binary)
   relies on the user already running `usearch config init` before
   plugin install. Is there a way for the plugin's first invocation to
   detect missing config and emit a helpful error via Claude? Possibly
   via the MCP server returning `-32002 usearch.unauthorized` (per
   SPEC-MCP-001 REQ-MCP-016) with a `data.setup_url` field that Claude
   can surface to the user. _TBD_ — design after MCP-001 onboarding
   error messages are finalised.

3. **Plugin repository hosting**: ship the plugin in the
   universal-search monorepo (`tools/claude-skill/` subdirectory) vs a
   dedicated `universal-search-claude-skill` repository? Monorepo gives
   atomic plugin-binary version coupling; dedicated repo gives cleaner
   marketplace listing (repository URL points only at plugin code).
   _TBD_ — defer to plan annotation cycle.

4. **Marketplace submission timing vs M9 V1.0.0**: do we submit the
   plugin to the Anthropic community marketplace **before** V1.0.0
   tag (so review can complete in parallel) or **after** tag (so the
   reviewed version matches the released version)? Community catalog
   sync is nightly so post-tag is the safer ordering. _TBD_ — decide
   when M9 ship date is firm.

5. **Binary distribution method outside plugin** (for Strategy B): brew
   tap? Go install? Curl install script? Container image? Helm chart?
   This is SPEC-DEPLOY-001 (M9) territory but the Skill plugin's README
   must point to one canonical install method. _TBD_ — coordinate with
   DEPLOY-001 ownership.

6. **Should the plugin also wrap the CLI for non-MCP hosts?** Per user
   instruction to consider this — research conclusion is **no**:
   - The plugin is a Claude Code construct; non-MCP hosts (Gemini CLI,
     Codex CLI) discover MCP servers via their own config, not via
     Claude Code plugins.
   - Wrapping the CLI in the Skill body would duplicate the MCP
     contract.
   - Each MCP-capable host can install `usearch-mcp` directly; the
     SPEC-MCP-001 server is already the universal contract.
   - The CLI (SPEC-CLI-002) is for shell scripting / piping, not for
     embedding in LLM hosts.
   - Conclusion: SPEC-SKILL-001 V1 is **Claude Code plugin only**.
     Gemini CLI / Codex CLI users install `usearch-mcp` and configure
     their host directly (instructions in SPEC-MCP-001 Phase 7
     three-client verification artifacts).

7. **Submit to `claude-plugins-official` (curated) vs community?**
   Official is Anthropic's discretion (no application process).
   Community is the explicit submission path. SPEC fixes target as
   community. If Anthropic invites the plugin to official later, no
   plugin changes required — same artifact.

8. **Plugin namespace prefix**: `usearch` (matches binary name) vs
   `universal-search` (matches product name). `usearch` is shorter (the
   skill becomes `/usearch:search` vs `/universal-search:search`). SPEC
   recommends `usearch` for brevity in line with the binary name.
   `_TBD_` to confirm in annotation.

9. **Plugin includes README screenshots**: who produces them, and
   captured against which environment? Likely sourced from SPEC-UI-001
   (Web UI) once it ships, or from terminal-based `usearch query`
   output. _TBD_ — coordinate with M9 docs effort.

10. **Skill description token cost vs match quality**: Claude reads
    every loaded skill's description on every turn. The description
    needs to be keyword-dense enough to trigger appropriately, but not
    so long that it costs tokens on every interaction. Per docs,
    keep description under 250 characters for menu display (v2.1.86+).
    SPEC fixes a target of ≤ 500 characters for description and
    delegates body content to the SKILL.md body which loads only on
    invocation. _TBD_ in run phase: iterate on description text after
    real-world trigger testing.

---

## 11. Risks

| Risk | Severity | Mitigation |
|------|----------|------------|
| User installs plugin without `usearch-mcp` binary on PATH | High | Plugin README states binary as prerequisite; MCP handshake failure produces clear "command not found" error in Claude Code's MCP server status pane. SKILL.md body documents the install steps as fallback. |
| Plugin version drifts from MCP server version, breaking tool schemas | High | Compatibility matrix in README; MCP server `--version` output documented; future enhancement is plugin-side handshake version check (§7). |
| Skill description triggers too aggressively, hijacking unrelated queries | Medium | Description keyword-tuned; user can disable plugin via `/plugin disable usearch-skill`; iterate description in patch releases based on usage feedback. |
| Skill description triggers too rarely, plugin appears broken | Medium | Same mitigation as above (iterate); SKILL.md body includes example invocation phrases so Claude can learn the pattern. |
| Marketplace review delays V1 ship | High | Submit early (during M8) so review can complete in parallel with V1.0.0 tag; self-hosted marketplace as fallback per §8.3. |
| Anthropic plugin schema change between V1 plugin and V1.1+ Claude Code release | Medium | Pin to documented plugin.json schema as of plugin publish date; reserve patch release for schema migrations. |
| Bundled binary (Strategy A) brings marketplace-size or signing issues if ever adopted | Low (V1 excludes) | Strategy A excluded from V1 per §4.4. Revisit only if user feedback demands it. |
| Token cost of always-loaded description on Claude Code interactions | Low | Description capped at 500 chars (§10 Q10); body only loads on match. |
| Plugin namespace collision with another `usearch` plugin in marketplace | Low | Namespace is unique per plugin in marketplace; verify by searching catalog before submission. |
| Auth headers leak via committed `.mcp.json` if user edits and pushes to public repo | Medium | README warns against committing `.mcp.json` with secrets; recommend env var substitution via `${USEARCH_TOKEN}`. |

---

## 12. References

External (verified):

- Plugin creation guide: https://code.claude.com/docs/en/plugins
- Plugin reference / schema: https://code.claude.com/docs/en/plugins-reference
- Skill documentation: https://code.claude.com/docs/en/skills
- Plugin marketplace docs: https://code.claude.com/docs/en/plugin-marketplaces
- Discover and install plugins: https://code.claude.com/docs/en/discover-plugins
- Community marketplace catalog repo:
  https://github.com/anthropics/claude-plugins-community
- Submission forms:
  - https://claude.ai/settings/plugins/submit
  - https://platform.claude.com/plugins/submit
- MCP specification (2025-06-18):
  https://modelcontextprotocol.io/specification/2025-06-18
- MoAI skill authoring rules:
  `.claude/rules/moai/development/skill-authoring.md`

Internal (project files):

- `.moai/project/product.md` — V1 surfaces include Claude Skill;
  self-hosted only; Apache-2.0 license target.
- `.moai/project/roadmap.md:95` — M7 SPEC-SKILL-001 row defining scope
  ("SKILL.md, install via marketplace, wraps MCP server").
- `.moai/project/roadmap.md:156` — M9 exit criterion: "Claude Skill
  installs from marketplace; MCP server connects from Claude Code +
  Codex + Gemini CLI".
- `.moai/specs/SPEC-MCP-001/spec.md` — the MCP server this Skill wraps.
- `.moai/specs/SPEC-MCP-001/research.md` — MCP transport tradeoffs;
  informs Skill `.mcp.json` strategy choice (§4).
- `.moai/specs/SPEC-MCP-001/plan.md` — Phase 7 three-client integration
  verification produces the config snippets the Skill plugin's README
  consumes.
- `.moai/specs/SPEC-CLI-002/spec.md` — `usearch config init` wizard
  cited as the onboarding entry point (§6.1); `--version` flag cited
  as the compatibility-check mechanism (§7).
- `.moai/specs/SPEC-AUTH-001/spec.md` — JWT middleware for HTTP MCP
  auth (§6.2).
- `.moai/specs/SPEC-DEPLOY-001/` (M9, not yet drafted) — binary
  distribution method coordination (§10 Q5).
- `.moai/project/brand/brand-voice.md` — currently `_TBD_` placeholders;
  Skill description tone is generic until brand interview completes
  (§10 Q10 follow-up).

---

*End of SPEC-SKILL-001 research v0.1.0 (draft).*
