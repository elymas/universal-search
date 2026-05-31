# SPEC-REL-001 Progress

## 2026-05-31 — Phase 1 Analysis (manager-strategy)

Phase: Plan / analysis only (no code). tasks.md written (9 tasks).
Harness: thorough (confirmed) → plan-auditor + Sprint Contract MANDATORY.
Recommendation: needs-plan-auditor-first (after 3 blockers reconciled).

### Release-asset reality check (grep-verified vs SPEC claims)
- 3 version consts confirmed but SPEC line refs are STALE:
  - usearch: actual `main.go:14` (SPEC says :21) — `const Version`
  - api: actual `main.go:20` (SPEC says :18) — `const version`
  - mcp: actual `main.go:19` (SPEC says :13) — `const version`
- TestVersionFlag regex MISMATCH (B3): actual `main_test.go:12`
  `^usearch v\d+\.\d+\.\d+` (prefix-anchored, no `$`, no prerelease group).
  SPEC/acceptance repeatedly cite `^usearch v[0-9]+...(-[a-zA-Z0-9.-]+)?$`.
  Real regex is LOOSER → easier to satisfy, but tests must target real regex.
- CHANGELOG.md: 182 lines, KaC v1.1.0, single `[Unreleased]`, 0 released
  versions. SPEC description accurate.
- git tags = 0 (clean slate, confirmed).
- Missing on main (confirmed absent): VERSION, MIGRATION.md, RELEASE.md,
  .goreleaser.yml, release.yml, internal/version/. Matches SPEC gap inventory.
- VERSION file (`1.0.0`) confirmed on PR#41 branch chore/track-version-marker,
  NOT on main. REL-001 correctly does NOT depend on it (uses ldflags single-source,
  rejects VERSION-file approach in D1 anti-decision). No conflict.
- go.mod: go 1.25.8, module github.com/elymas/universal-search (ldflags path OK).

### Dependency verification
- All 7 deps (DOC-001/002, DEPLOY-001, SEC-001, EVAL-001/002/003) = status:draft
  on main; implementations on open PRs #46/#48/#47/#42/#43/#44/#45 (unmerged).
- REL-001 infra (version pkg, goreleaser, release.yml, docs) does NOT import dep
  code → infra CAN ship decoupled. Actual tag + live G6-G9 PASS deferred post-merge.
- 42 SPECs status:implemented (NFR-REL-005 count basis), 47 total dirs.

### Pre-tag verification matrix (G1..G12)
Master gate = release.yml `pre-tag-verify` job, runs on tag push, all-PASS-or-abort.
G1-G3 local code health/lint/LSP; G4 deps-audit (exists); G5 security.yml,
G6 eval-*, G7 chart-ci, G8 docs.yml, G9 adapter-drift — these 5 reference dep
workflows that DON'T EXIST on main yet (only on PRs). So matrix is authored now
with `gh run list` lookups that resolve only after merge. G10 24h CI green,
G11 git verify-tag (GPG), G12 version==tag consistency.

### org_resolution (KEY DECISION — B1)
REL-001 RESOLVES `<org>` → hardcodes `ghcr.io/elymas/...` everywhere (0 placeholders).
But all 7 dep SPECs still use `ghcr.io/<org>/<repo>` placeholder (SEC-001,
DEPLOY-001, DOC-001/002 unresolved). So `<org>` IS resolved by REL-001 to `elymas`,
but the deps were authored before that decision → they must be amended (or their
workflows templated) to emit `elymas` or REL's G5-G9 cosign/registry verification
will target images the deps pushed under a different/placeholder path. NOT blocking
infra build; blocking live release. Decision is made (elymas); propagation pending.

### signing_reconciliation
ALIGNED, not duplicated. SEC-001 REQ-SEC-016 owns SLSA-L2 + cosign keyless +
SBOM POLICY and the security.yml + pinned installer versions (cosign-installer
@v3.7.0, slsa-github-generator generator_generic_slsa3.yml@v2.0.0). DEPLOY-001
REQ-DEPLOY-018 owns IMAGE signing (7-8 images) + chart signing. REL-001 REUSES
the same pinned versions and applies the SAME generator to the GO BINARIES only
(the gap neither SEC nor DEPLOY covers). No re-require/conflict: binary-sign is
REL's exclusive slice. cosign verify identity regexp consistent across all
(release.yml@.* + token.actions OIDC issuer).

### B2 — image-name conflict (cross-SPEC, real)
REQ-REL-017 G7 verifies `ghcr.io/elymas/universal-search:1.0.0` as PRIMARY image
+ usearch-api/usearch-mcp/usearch-migrate. DEPLOY-001 REQ-DEPLOY-018 actually
builds: usearch-api, usearch-mcp, usearch-migrate, researcher, embedder,
tokenizer-ko, storm, koreanews — there is NO `universal-search` image. G7 as
written will fail (verifies a nonexistent image). Must reconcile REQ-REL-017's
image set to DEPLOY's actual output, or rename one side. Annotation-cycle item.

### tag_deferral — YES
The live `v1.0.0` tag is operational, post-merge. REL-001 ships machinery +
RELEASE.md procedure. spec/plan explicitly defer CHANGELOG body, MIGRATION
content, tag creation (HARD). Confirmed self-consistent.

### phase0_status
plan-auditor REQUIRED (thorough: plan_audit.enabled=true,
cross_validate_with_evaluator_active=true, sprint_contract=true, evaluator strict
per-sprint). Sprint Contract MANDATORY.

### Doc-internal conflict (B-doc)
MIGRATION.md 12-section ordering differs between spec.md §D4 and acceptance.md
AC-005 (different section titles/order). Reconcile before T6.

### Blockers (3) — annotation-cycle, none block infra START
- B1 org propagation: deps must emit `elymas` (REL decided) for live G5-G9.
- B2 REQ-REL-017 verifies `universal-search` image DEPLOY never builds.
- B3 SPEC's TestVersionFlag regex is stale vs real code.

### Risks (top 3)
1. Live release blocked until 7 dep PRs merge + their workflows land on main
   AND org/image naming (B1/B2) reconciled — REL infra ready ≠ release-able.
2. B2 image-name mismatch → G7 hard-fails on a phantom `universal-search` image.
3. 3-const → single-source migration touches 3 binaries with import-shadow risk
   (api/mcp local `version` var vs imported pkg); HARD char-test on real regex.

### Deferral candidates (proportionality)
V1-essential: internal/version single-source, .goreleaser.yml, release.yml
skeleton, RELEASE.md, MIGRATION skeleton, CHANGELOG format contract, pre-tag matrix.
Could fast-follow (align w/ SEC/DEPLOY deferred-signing posture): binary SLSA-L2
+ cosign sign-blob + SBOM attach could ship as v1.0.1 if release momentum needs
it — but SEC-001 already commits L2 so keep in V1 for parity. Genuinely
post-V1: gitsign, Homebrew/apt, Windows, release-please automation, LTS.
