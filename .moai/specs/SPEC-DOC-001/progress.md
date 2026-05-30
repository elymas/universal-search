# SPEC-DOC-001 Progress

## 2026-05-30 — Phase 1 (Analysis & Planning), manager-strategy

- Read spec.md (18 EARS + 7 NFR), plan.md (7 phases), harness.yaml.
- Infra reality check on docs/ and web/ (recurring stale-assumption risk).
- Wrote tasks.md: 10 atomic tasks (T1-T10), DDD content + TDD scripts.

Harness: **standard** (confirmed). plan-auditor: **REQUIRED** (standard
harness enables plan_audit, max_iterations 3, require_must_pass true).

Key findings (stale-assumption flags):
- B1: docs/ has stale Nextra **v3.1.1 / Next 14** build artifacts
  (docs/.next/, docs/out/, docs/node_modules/ — gitignored; only
  docs/{dependencies.md,_deps-*.md,licenses/} are git-tracked). No
  package.json / next.config / theme.config in working tree. SPEC
  assumes greenfield v4 — must clean + pin v4/Next16 in T2.
- B2: ops/security/* (runbook, owasp-asvs-checklist, threat-model)
  do NOT exist on main — they live on SPEC-SEC-001 branches
  (feat/spec-sec-001-security, feature/SPEC-SEC-001), PR #42 unmerged.
  SPEC REQ-DOC-005 cross-links them; on a main-based docs branch these
  are forward-references, not live cross-links.
- B3: REQ-DOC-007 (spec ln438) says CLI "currently query" only. Real
  surface = query, config, history, deep, sources, login, repl (7
  top-level + nested subcommands, all implemented under cmd/usearch/).
  gen-cli-reference.sh must target the full set.
- Verified present: .moai/docs/MCP_OAUTH_SETUP.md, scripts/gen-deps-
  manifest.sh (referenced pattern), MCP tools (ListSources, GetCitation,
  Search, DeepResearch in internal/mcpserver/server.go).

Dependency verification: 8/10 deps' surfaces present on main (CLI-001/002,
UI-001 web app, MCP-001 tools, AUTH-001/002/003 + OBS-001 via code/docs).
SEC-001 (B2) and full OBS metric catalog are forward-ref. No hard blocker
— docs may describe forward-referenced features, flagged in tasks.md.

Recommendation: needs-plan-auditor-first (standard harness mandates it;
amend SPEC for B1/B2/B3 during audit cycle).
