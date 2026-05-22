# SPEC-SKILL-001 Plan — phased implementation

Status: draft companion to spec.md
Author: limbowl via manager-spec
Date: 2026-05-22
Methodology: DDD (per `.moai/config/sections/quality.yaml`
`development_mode: tdd` default — overridden to DDD for this SPEC
because the plugin authors against an existing MCP server contract
(SPEC-MCP-001); characterization tests dominate over new-logic tests)
Coverage target: 85%
Harness: standard (per `.moai/config/sections/harness.yaml` auto-
routing — 12 REQs (3 × P0 + 7 × P1 + 2 × P2) + 6 NFRs + 1 plugin
artifact directory + 1 SKILL.md + 1 .mcp.json + 1 README; Sprint
Contract optional)

This plan sequences the SPEC-SKILL-001 implementation into priority-
ordered phases. Per `.claude/rules/moai/core/agent-common-protocol.md`,
time estimates are PROHIBITED — phases use priority + ordering, never
duration.

---

## 1. Implementation principle

The Skill plugin is a **declarative artifact** wrapping the
SPEC-MCP-001 server. The plan favours:

1. **Zero plugin-side logic** — the plugin is configuration + Markdown.
   All behaviour lives in `usearch-mcp` (SPEC-MCP-001). The plugin
   adds no tools, no agents, no hooks, no bundled binaries.
2. **Description-first iteration** — the SKILL.md `description` field
   is the single most important UX surface (it decides whether Claude
   invokes the plugin at all). Iterate the description with manual
   trigger tests before locking V1.
3. **Marketplace submission early** — Anthropic community marketplace
   review pipeline runs nightly with up to 24h sync lag. Submit during
   M8 so review can complete in parallel with M9 V1.0.0 tag (per
   research §10 Q4 resolution).
4. **README is the install contract** — Quick Start steps must be
   sufficient for a user who has never seen Universal Search to be
   running a cited query within 5 minutes (NFR-SKILL-002).
5. **No bundled binaries** — Strategy B (PATH launch) only; never
   ship `usearch-mcp` binaries inside the plugin (REQ-SKILL-001,
   §4 exclusion).
6. **Characterization tests over invention** — DDD methodology: write
   tests that capture how Claude Code loads, validates, and invokes
   the plugin given a fixed MCP-001 server stub. Refactor the plugin
   artifacts to keep tests green.

---

## 2. Phase ordering

Priority labels per MoAI rule (no time estimates).

### Phase 0 — Plan-auditor PASS (Priority High)

- Plan-auditor reviews spec.md + research.md + plan.md + acceptance.md
  (the latter authored alongside this plan).
- Address MAJOR / MINOR / NIT findings via amendment commits.
- Resolve research §10 Q3 (plugin repository hosting) and Q8 (plugin
  namespace `usearch` vs `universal-search`) during annotation cycle
  before Phase 1 starts.
- Status transition: `draft → approved` once PASS.
- Block: no implementation work begins until Phase 0 completes.

### Phase 1 — Plugin scaffolding (Priority High)

Goal: plugin directory structure exists, manifest validates,
`claude plugin validate` exits 0 against an empty-but-valid plugin.

Tasks:
1. Resolve Phase 0 _TBD_: choose plugin repo location (monorepo
   subdir `tools/claude-skill/usearch/` OR dedicated repo
   `universal-search-claude-skill`).
2. Create directory tree per spec.md §7.1 (`.claude-plugin/`, `skills/
   usearch/`, `docs/screenshots/` placeholder, `.github/workflows/`).
3. Write `.claude-plugin/plugin.json` with placeholder description
   and version `"0.0.1-dev"` for development iteration.
4. Write minimal `skills/usearch/SKILL.md` with placeholder
   description (`Universal Search — placeholder, see SPEC-SKILL-001
   Phase 2`).
5. Write minimal `.mcp.json` per REQ-SKILL-007 default (stdio +
   `usearch-mcp` from PATH).
6. Write minimal `README.md` Quick Start skeleton.
7. Add `LICENSE` (Apache-2.0 text) per REQ-SKILL-003.
8. Run `claude plugin validate ./usearch/` — assert exit 0.
9. Add CI workflow (`.github/workflows/validate.yml`) that runs the
   same `claude plugin validate` on every push.

Exit criterion:
- Plugin directory exists and validates.
- CI green on initial commit.
- Plugin loads in dev mode via `claude --plugin-dir ./usearch` without
  errors (`usearch-mcp` binary not yet required to be installed —
  MCP handshake error is acceptable at this phase, plugin LOAD must
  succeed).

### Phase 2 — SKILL.md description authoring + trigger testing (Priority High)

Goal: SKILL.md description triggers correctly on representative
research queries.

Tasks:
1. Author SKILL.md `description` field per REQ-SKILL-004
   requirements (≤ 500 chars; covers research / citation / multi-
   source / Korean / deep-research / team-memory triggers).
2. Author SKILL.md body sections:
   - "Tool Selection Guide" per REQ-SKILL-005 + REQ-SKILL-006 (when
     to call `search` vs `deep_research` vs `list_sources` vs
     `get_citation`; Korean auto-routing note).
   - "Error Handling" per REQ-SKILL-009 (three documented error codes
     with actionable user-facing responses).
3. Set up `usearch-mcp` binary locally with stub adapters returning
   deterministic fixture responses.
4. Manual interaction test from a fresh Claude Code session with 5
   sample queries:
   - "Find recent papers on diffusion models with citations" → expect
     `search`.
   - "Give me a comprehensive report on quantum computing for the
     team" → expect `deep_research`.
   - "한국어로 최근 AI 뉴스 검색해줘" → expect `search` (server-side
     routes to Korean adapters).
   - "What sources do you have for academic papers?" → expect
     `list_sources`.
   - "Tell me more about source [3] from your last answer" → expect
     `get_citation`.
5. Score: ≥ 4/5 must trigger correctly. If < 4/5, refine description
   keywords and retry.
6. Record final description text + iteration history in plugin's
   `CHANGELOG.md`.

Exit criterion:
- ≥ 4/5 manual trigger test pass rate.
- Description ≤ 500 chars (NFR-SKILL-003).
- Tool Selection Guide + Error Handling sections present.

### Phase 3 — `.mcp.json` wiring + handshake validation (Priority High)

Goal: plugin's `.mcp.json` correctly launches `usearch-mcp` and the
MCP handshake completes; missing-binary failure mode produces clear
error.

Tasks:
1. Finalise `.mcp.json` per REQ-SKILL-007 (stdio default, no embedded
   env, no bundled binary path).
2. Integration test: Claude Code with plugin loaded, `usearch-mcp` on
   PATH → MCP status pane shows "connected", `tools/list` returns the
   four MCP-001 tools.
3. Integration test: Claude Code with plugin loaded, `usearch-mcp`
   NOT on PATH → MCP status pane shows "command not found" with
   actionable error text.
4. Document the HTTP transport swap snippet in README per
   REQ-SKILL-008 (with `_TBD_` note: verify HTTP MCP entry schema
   against current Anthropic plugin docs in run phase).
5. Add NFR-SKILL-005 secret scanner CI step (gitleaks or equivalent);
   gate at zero findings.

Exit criterion:
- Handshake integration tests green for both binary-present and
  binary-missing paths.
- README HTTP swap snippet documented and includes env-var
  substitution + security warning.
- Secret scanner gates at zero.

### Phase 4 — README authoring + compatibility matrix (Priority Medium)

Goal: README enables first-time users to install and query within
5 minutes (NFR-SKILL-002 (a) path).

Tasks:
1. Write "Quick Start" section per REQ-SKILL-010:
   - Step 1: install `usearch-mcp` binary (cite SPEC-DEPLOY-001 method
     — _TBD_ until DEPLOY-001 lands; placeholder pointer for V1 draft).
   - Step 2: `usearch config init` (cite SPEC-CLI-002 REQ-CLI2-012).
   - Step 3: `/plugin install` (post-marketplace-approval; pre-
     approval document `claude --plugin-dir` for testing).
2. Write "Compatibility Matrix" section per REQ-SKILL-010:
   - Plugin 0.x → `usearch-mcp` ≥ 0.x.
   - Plugin 1.0 → `usearch-mcp` ≥ 1.0.
   - Document `usearch-mcp --version` as the probe.
3. Write "Troubleshooting" section covering:
   - Binary not on PATH.
   - Config file missing or invalid.
   - Quota exhaustion (`/deep` cap from SPEC-DEEP-004).
   - Korean tokenizer not installed (if relevant for self-hosted
     deployments).
4. Write "Other Hosts (Gemini CLI / Codex CLI)" section per
   NFR-SKILL-006 — links out to SPEC-MCP-001 Phase 7 multi-host
   instructions; does NOT include install steps for those hosts.
5. README inspection lint test asserts all required sections present.

Exit criterion:
- README structurally complete (all REQ-SKILL-010 sections present).
- NFR-SKILL-006 cross-host accuracy test green.
- Manual NFR-SKILL-002 (a) walkthrough: ≤ 5 minutes from fresh
  install to first cited query.

### Phase 5 — Marketplace screenshots + listing prep (Priority Medium)

Goal: marketplace listing materials ready for submission.

Tasks:
1. Capture 3-5 screenshots per REQ-SKILL-012:
   - Basic search with cited paragraph (e.g., "what are the latest
     SearXNG forks").
   - `/deep` STORM-style long-form report.
   - Korean query routed to Naver adapters.
   - `get_citation` drill-down on a specific source.
   - (Optional) plugin install UX in Claude Code's `/plugin` browser.
2. Capture environment: terminal output from CLI (SPEC-CLI-002
   `--format markdown` mode renders cleanly) OR Web UI screenshots
   (SPEC-UI-001) — resolution depends on ship timing of UI-001 per
   research §10 Q9.
3. Write marketplace listing JSON snippet (for self-hosted
   marketplace fallback per research §8.3): `name`, `displayName`,
   `description`, `keywords`, `license`, `repository`, `screenshots`,
   `author`.
4. Verify all keywords (REQ-SKILL-002) align with the user query
   patterns the description should match.

Exit criterion:
- ≥ 3 screenshots in `docs/screenshots/`.
- Listing JSON snippet validates against the marketplace.json schema.

### Phase 6 — `claude plugin validate` preflight + uninstall test (Priority Medium)

Goal: plugin passes all submission-preflight gates; uninstall is
clean.

Tasks:
1. Run final `claude plugin validate` against the locked plugin
   artifact — exit 0 required.
2. Snapshot-diff uninstall test per NFR-SKILL-004:
   - Capture filesystem state pre-install.
   - `/plugin install --plugin-dir ./usearch`.
   - `/plugin uninstall usearch`.
   - Diff filesystem; assert no residue outside plugin directory
     and Claude Code plugin state file.
3. Plugin-size cap test per NFR-SKILL-001: `du -s ./usearch/`
   excluding `.git/`, assert ≤ 5 MB.
4. Bump `plugin.json:version` to V1 candidate per REQ-SKILL-011
   (e.g., `"1.0.0"` if shipping with V1.0.0 release).
5. Tag plugin repo with the matching version tag.

Exit criterion:
- All preflight gates green.
- Version locked.

### Phase 7 — Marketplace submission (Priority Medium)

Goal: plugin submitted to `claude-plugins-community` per HISTORY D7.

Tasks:
1. Resolve research §10 Q4 (submission timing vs V1.0.0 tag) at
   project leadership level. Default: submit during M8 so review
   completes by M9 tag.
2. Submit via https://claude.ai/settings/plugins/submit OR
   https://platform.claude.com/plugins/submit.
3. Address any review-pipeline feedback (typically: manifest
   correctness, security screening, description honesty).
4. Wait for approval + nightly sync (up to 24h lag) to
   `anthropics/claude-plugins-community` catalog.
5. Verify plugin installable via `/plugin marketplace add anthropics/
   claude-plugins-community` → `/plugin install @claude-community/
   usearch` from a clean Claude Code installation.
6. Parallel: prepare self-hosted marketplace fallback per research
   §8.3 (`.claude-plugin/marketplace.json` in plugin repo) in case
   review is gating V1 ship.

Exit criterion:
- Plugin listed in community catalog (or self-hosted fallback ready).
- Installable end-to-end from fresh Claude Code session.

### Phase 8 — Cross-client smoke test (Priority Low)

Goal: confirm the M9 exit criterion is met from a real user
perspective.

Tasks:
1. From a fresh Claude Code installation (no prior plugins), run the
   §5.1 acceptance scenario end-to-end.
2. From Claude Desktop, repeat (Claude Desktop consumes the same
   plugin format).
3. Cross-check the M9 exit criterion: "Claude Skill installs from
   marketplace; MCP server connects from Claude Code + Codex + Gemini
   CLI". The plugin half is covered by 1+2 above; the Codex/Gemini
   half is delegated to SPEC-MCP-001 Phase 7 docs.
4. File any discovered gaps as patch-version follow-ups.

Exit criterion:
- Both Claude Code and Claude Desktop install paths verified.
- M9 exit criterion preflight complete from the plugin side.

### Phase 9 — Sync phase (Priority Low)

Goal: documentation + PR.

Tasks:
1. `manager-docs` updates user-facing docs:
   - Parent repo `README.md`: add "Install via Claude Code" section
     linking to the marketplace listing.
   - SPEC-DOC-001 user docs site (when published): add Skill plugin
     install page cross-linked from the MCP-001 install page.
2. CHANGELOG entry in plugin repo + parent repo.
3. `manager-git` opens PR per V1 release process.
4. Status transition: `approved → implemented` after merge +
   marketplace listing live.

---

## 3. Test inventory (DDD characterization checkpoints)

Per-phase characterization tests (DDD: capture how Claude Code +
marketplace tooling currently behave around the plugin artifact, then
preserve that behaviour through any plugin edits):

- Phase 1: `TestPluginValidateExitsZero` (`claude plugin validate`
  passes), `TestPluginDirectoryStructureMatchesSpec`,
  `TestPluginLoadsViaPluginDirFlag`.

- Phase 2: `TestSkillMdFrontmatterIsAnthropicStandardOnly`,
  `TestDescriptionLengthUnderCap`, `TestDescriptionContainsRequiredTriggerKeywords`,
  `TestToolSelectionGuideSectionPresent`, `TestErrorHandlingSectionPresent`,
  manual `ManualTriggerScoreMinFourOfFive` (recorded in CHANGELOG).

- Phase 3: `TestMcpJsonHandshakeWithBinaryPresent`,
  `TestMcpJsonHandshakeWithBinaryMissingProducesClearError`,
  `TestSecretScannerFindings == 0`,
  `TestReadmeHttpSwapSnippetIncludesEnvVarSubstitution`.

- Phase 4: `TestReadmeQuickStartSectionStructure`,
  `TestReadmeCompatibilityMatrixHasAtLeastOneEntry`,
  `TestReadmeOtherHostsSectionLinksOutOnly`,
  manual `ManualFiveMinuteOnboardingWalkthrough`.

- Phase 5: `TestScreenshotsDirectoryHasMinimumThree`,
  `TestMarketplaceListingJsonSchemaValidates`.

- Phase 6: `TestPluginSizeUnderFiveMB`, `TestUninstallLeavesNoResidue`,
  `TestPluginVersionIsExplicitSemver`.

- Phase 7: manual `ManualMarketplaceInstallSmokeTest`.

- Phase 8: manual `ManualClaudeCodeInstallE2E`,
  `ManualClaudeDesktopInstallE2E`.

Note: there are no Go unit tests because the plugin contains no Go
code. Tests are CI workflow scripts (Bash + `claude plugin validate`
+ frontmatter linter + secret scanner + size measurement) and manual
verification logs co-located with the plugin source.

---

## 4. MX tag plan

The plugin contains no source code — no @MX tags apply within the
plugin directory itself.

The plan does update one @MX:ANCHOR comment in the parent repo:

| File | Tag | Reason |
|------|-----|--------|
| `cmd/usearch-mcp/main.go::main` | `@MX:ANCHOR` (extend `@MX:REASON`) | Add "and SPEC-SKILL-001 Claude Skill plugin" to the existing caller list. |

All other parent-repo @MX:ANCHOR / @MX:REASON updates that cite "MCP
server" already implicitly cover Skill-plugin-driven invocations (the
plugin launches the same binary; from the binary's perspective there
is no distinction between a CLI-launched MCP client and a plugin-
launched MCP client).

---

## 5. Risk-driven sequencing notes

Risks from research.md §11 with their mitigation phase:

- User installs plugin without `usearch-mcp` binary on PATH → Phase 3
  (handshake-failure test asserts clear error path) + Phase 4 (README
  Quick Start step 1 makes binary install prerequisite).
- Plugin version drifts from MCP server version → Phase 4
  (compatibility matrix) + Phase 6 (version lock) + future enhancement
  reserved for plugin-side handshake check.
- Skill description triggers too aggressively → Phase 2 (manual
  trigger test gate ≥ 4/5; iteration cycle in plugin CHANGELOG).
- Skill description triggers too rarely → Phase 2 (same mitigation).
- Marketplace review delays V1 ship → Phase 7 (submit during M8;
  self-hosted marketplace fallback ready).
- Anthropic plugin schema change between V1 plugin and V1.1+ Claude
  Code → Phase 6 (validate against current schema at submission
  time); patch-release reserved for schema migrations.
- Strategy A binary bundling temptation → mitigated by REQ-SKILL-001
  + §4 exclusion + NFR-SKILL-001 size cap. Plan does not introduce
  bundled binaries at any phase.
- Token cost of always-loaded description → Phase 2 (NFR-SKILL-003 ≤
  500 chars cap enforced by frontmatter linter).
- Plugin namespace collision in marketplace → Phase 7 (verify
  catalog before submission).
- Auth header leak via committed `.mcp.json` with secret → Phase 3
  (secret scanner CI gate) + Phase 4 (README warning).

---

## 6. Sync-phase deliverables (Phase 9)

- Parent repo `README.md`: add "Install via Claude Code" section.
- Plugin repo `CHANGELOG.md`: V1 entry with description, included
  tools, MCP server compatibility note.
- Parent repo `CHANGELOG.md`: SPEC-SKILL-001 entry under M7.
- PR title: `feat(skill): implement SPEC-SKILL-001 — Claude Code
  Skill plugin for Universal Search (M7)`.
- PR body: links to spec.md, research.md, acceptance.md;
  marketplace listing URL once live; checklist of REQ acceptance.
- Status transition: `approved → implemented` on merge + marketplace
  listing live.
- Notify: M9 docs effort (SPEC-DOC-001 owner) that the plugin URL is
  available for cross-linking from the user docs site.

---

## 7. Open factoring decisions deferred to run phase

These items are explicitly NOT decided at plan time — they are
implementation-detail choices the run-phase agent will make:

1. **Plugin repo placement**: monorepo subdirectory (`tools/claude-
   skill/usearch/`) vs dedicated repo (`universal-search-claude-
   skill`). Plan recommends dedicated repo for cleaner marketplace
   listing (repository URL points only at plugin code, not the entire
   universal-search codebase). Annotation cycle confirms.

2. **`usearch` vs `universal-search` namespace prefix**: Plan
   recommends `usearch` for brevity (skill becomes `/usearch:search`
   not `/universal-search:search`). Annotation cycle confirms.

3. **HTTP MCP server entry shape in `.mcp.json`** (REQ-SKILL-008
   `_TBD_`): the `.mcp.json` `type: "http"` syntax for plugins is
   per the MCP spec but the plugin docs do not explicitly document
   it. Phase 3 verifies against current Anthropic docs and adjusts
   the README snippet accordingly.

4. **Screenshot capture environment**: terminal CLI vs Web UI.
   Depends on SPEC-UI-001 ship timing. Phase 5 picks whichever is
   available; if UI-001 ships before plugin submission, prefer UI
   screenshots for marketplace appeal.

5. **Compatibility matrix initial entries**: Phase 4 populates with
   the actual `usearch-mcp` version available at plugin Phase 1.
   Matrix grows by patch release as MCP-001 evolves.

6. **Plugin version at V1.0.0 ship**: `1.0.0` (matches parent) is
   the recommendation per REQ-SKILL-011. If marketplace review
   surfaces issues requiring resubmission, patch versions
   (`1.0.1`, `1.0.2`) absorb the iterations without re-ranking the
   parent project's release.

7. **README cross-link target for user docs**: depends on SPEC-DOC-
   001 publish status. Phase 4 ships with placeholder; Phase 9 sync
   updates the link.

8. **Description final text**: Phase 2 iterates against manual
   trigger test; final wording locked at Phase 2 exit, recorded in
   CHANGELOG.

These are scope-bounded — none change the SPEC contract; all are
mechanical implementation choices.

---

*End of SPEC-SKILL-001 plan v0.1.0 (draft).*
