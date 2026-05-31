# SPEC-REL-001 Tasks — atomic decomposition

Status: draft (Phase 1 analysis output, manager-strategy)
Methodology: DDD (ANALYZE-PRESERVE-IMPROVE), harness: thorough
Owner: manager-git (release ceremony) / manager-ddd (infra build)
Generated: 2026-05-31

Scope note: REL-001 ships RELEASE INFRASTRUCTURE + procedure. The actual
`v1.0.0` git tag + CHANGELOG `[1.0.0]` body + MIGRATION per-section content
are OPERATIONAL steps deferred to release ceremony AFTER all 7 dependency
PRs (#42-#48) merge to main. This task list covers infra only.

Pre-implementation blockers to resolve in annotation cycle (see progress.md):
- B1: org/registry naming reconciliation (REL hardcodes `elymas`; deps use `<org>`)
- B2: REQ-REL-017 verifies `universal-search` image that DEPLOY-001 never builds
- B3: TestVersionFlag regex in SPEC is wrong (stale) vs actual code

---

## T1 — ANALYZE: version surface + cross-SPEC contract reconciliation
Requirement: REQ-REL-001, REQ-REL-002, REQ-REL-017 / DDD ANALYZE
Depends: none
- Capture actual version const locations (verified): `cmd/usearch/main.go:14`
  `const Version`, `cmd/usearch-api/main.go:20` `const version`,
  `cmd/usearch-mcp/main.go:19` `const version`.
- Capture actual test regex (verified): `cmd/usearch/main_test.go:12`
  `^usearch v\d+\.\d+\.\d+` (prefix-anchored, NO `$`, NO prerelease group).
  SPEC's cited regex `^usearch v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.-]+)?$`
  is STALE — reconcile before writing REQ-REL-002 tests.
- grep consumers of ServiceVersion/Commit/BuildDate to resolve OQ1 (export level).
- Reconcile B1/B2 with DEPLOY-001 + SEC-001 image/org naming (annotation cycle).
Acceptance: line-accurate inventory recorded; B1/B2/B3 dispositions agreed.

## T2 — Sprint Contract negotiation (thorough harness MANDATORY)
Requirement: constitution §11 / plan Phase 0
Depends: T1
- Negotiate acceptance checklist + priority dimension (Completeness first) with
  evaluator-active; write `.moai/sprints/SPEC-REL-001/sprint-01.md`.
- Sprint 1 = S1,S2,S3,S4,S5,S6,S7,S12 (infra-verifiable); Sprint 2 deferred
  (S8-S11 require real tag/staging + merged dep workflows).
Acceptance: sprint-01.md signed off by evaluator-active.

## T3 — IMPROVE: `internal/version/` package (TDD sub-cycle)
Requirement: REQ-REL-001 / DDD IMPROVE
Depends: T1
- RED: `internal/version/version_test.go` (default, String(), Short(), ldflags
  injection integration test). Use the ACTUAL regex from T1, not SPEC's.
- GREEN: `internal/version/version.go` — vars Version/Commit/BuildDate
  (+ GoVersion via runtime.Version() at call site), String(), Short().
- @MX:ANCHOR on exported vars (fan_in 3 binaries).
Acceptance: `go build ./internal/version/...` exit 0; coverage ≥ 90%.

## T4 — PRESERVE+IMPROVE: 3-binary refactor to consume version package
Requirement: REQ-REL-001, REQ-REL-002 (HARD characterization) / DDD
Depends: T3
- `cmd/usearch/main.go`: remove `const Version`, import version pkg, update
  `--version` print + obs ServiceVersion. Preserve `-v` alias semantics.
- `cmd/usearch-api/main.go` + `cmd/usearch-mcp/main.go`: remove local
  `const version`, import (aliased to avoid shadow), update obs.Init.
- Resolve OQ2 (whether api/mcp get `--version` flag) per T1 grep.
Acceptance: `go test -run TestVersion ./cmd/usearch/...` PASS;
`go test -race ./...` PASS; LSP zero errors.

## T5 — IMPROVE: `.goreleaser.yml` (v2) + snapshot validation
Requirement: REQ-REL-006 / DDD IMPROVE
Depends: T3
- 3 binaries × {linux,darwin} × {amd64,arm64} = 12 archives, CGO_ENABLED=0,
  ldflags inject version pkg vars, SHA256SUMS, syft SBOM, Windows excluded
  w/ rationale comment, `release.disable: true`.
- Validate: `goreleaser check`; `goreleaser release --snapshot --clean`.
Acceptance: dist/ has 12 archives + checksums + SBOM; `goreleaser check` exit 0.

## T6 — IMPROVE: MIGRATION.md + RELEASE.md skeletons
Requirement: REQ-REL-004, REQ-REL-005, REQ-REL-016 / DDD IMPROVE
Depends: T1
- MIGRATION.md: 12-section skeleton. NOTE: spec.md §D4 and acceptance.md
  AC-005 list DIFFERENT 12-section orderings — reconcile in T1/annotation
  before authoring (B-doc conflict).
- RELEASE.md: 5 sections (A pre-tag matrix, B GPG signed-tag procedure,
  C emergency rollback, D post-release checklist, E KST locale/timing).
- Per-section content deferred to release ceremony; skeleton + placeholders only.
Acceptance: structural lint passes (12 / 5 headers); verify cmds copy-pasteable.

## T7 — IMPROVE: CHANGELOG format contract + README badge
Requirement: REQ-REL-003, NFR-REL-005 / DDD IMPROVE
Depends: T1
- `scripts/gen-changelog-format-check.sh`: SPEC-ID count gate (NFR-REL-005
  basis = 42 implemented SPECs today; gate is dynamic count).
- README.md: shields.io release badge + install snippet (v1.0.0 URL 404 until
  ceremony — expected).
- Document CHANGELOG `[1.0.0]` promotion procedure (no body edit — HARD).
Acceptance: format-check script passes synthetic `[1.0.0]` fixture.

## T8 — IMPROVE: `.github/workflows/release.yml` — G1..G12 gates + pipeline
Requirement: REQ-REL-007, REQ-REL-009, REQ-REL-010/011/012, REQ-REL-013/014/015/018
Depends: T4, T5, T7; cross-dep PR merges for G5-G9 evidence wiring
- pre-tag-verify job: G1..G12 as needs graph. G5/G6/G7/G8/G9 reference dep
  workflows that DO NOT EXIST on main yet (security.yml, eval-*.yml, chart-ci.yml,
  docs.yml, adapter-reference-drift.yml) — wire as `gh run list` lookups that
  will resolve only post-merge. Author with placeholders + TODO until merge.
- goreleaser-build → slsa-provenance (generator_generic_slsa3.yml@v2.0.0)
  → cosign-sign (cosign-installer@v3.7.0 sign-blob) → publish-release
  (CHANGELOG awk extraction) → post-release-tasks (roadmap PR, gated on !dry_run).
- workflow_dispatch dry_run input (NFR-REL-006).
- G7 image-name set MUST match DEPLOY-001 actual output (see B2) — do not
  hardcode `universal-search` image if DEPLOY never builds it.
Acceptance: `actionlint` exit 0; dry_run end-to-end builds+skips publish;
synthetic G-fail aborts (S7).

## T9 — VERIFY: Sprint 1 scenarios + plan-auditor sign-off
Requirement: acceptance A1-A13, S1-S7+S12 / DDD VERIFY
Depends: T3-T8
- Run S1,S2,S3,S4,S5,S6,S7,S12.
- TRUST5: Tested (≥85% internal/version + shell), Secured (gitleaks+Trivy on
  YAML), Trackable (PR cites SPEC-REL-001).
- plan-auditor independent review → `.moai/reports/plan-auditor/SPEC-REL-001.md`.
- Confirm HARD deferrals intact (no tag, no CHANGELOG body, no MIGRATION content).
Acceptance: 8 Sprint-1 scenarios PASS; plan-auditor sign-off; baton to manager-git.

---

## Task → REQ coverage (P0 emphasis)
- REQ-REL-001/002: T1,T3,T4 | REQ-REL-003: T7 | REQ-REL-004: T6 | REQ-REL-005: T6
- REQ-REL-006: T5 | REQ-REL-007/009/010/011/012: T8 | REQ-REL-008: T6(B procedure)/ceremony
- REQ-REL-013/014/015/018: T8 | REQ-REL-016: T6 | REQ-REL-017: T1(B2)/T8
- NFRs: T5(NFR-002), T6(NFR-004/007), T7(NFR-005), T8(NFR-001/006), T9(verify)

## HARD deferred to release ceremony (post 7-PR merge) — NOT in tasks
- Actual `git tag -a -s v1.0.0` creation + push
- CHANGELOG `[1.0.0]` section body promotion
- MIGRATION.md per-section breaking-change content
- Live cross-SPEC G6/G7/G8/G9 PASS (needs merged dep workflows)
