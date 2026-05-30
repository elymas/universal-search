# Sync Report — SPEC-DEPLOY-001

**Timestamp**: 2026-05-31T00:00:00Z
**SPEC**: SPEC-DEPLOY-001 — Helm chart: k8s team-scale deploy (10-service compose topology + api/mcp containerization)
**Mode**: auto (single-SPEC sync)
**Strategy**: main_direct (no PR, no push)
**Lifecycle Level**: 1 (spec-first)
**Status Transition**: approved → implemented

## Pre-Sync Quality Gates

| Gate | Command / Check | Result |
|------|----------------|--------|
| Helm Lint | `helm lint charts/universal-search/` | PASS (0 failed) |
| Helm Template | `helm template charts/universal-search/` | PASS (49 resources rendered) |
| Kubeconform | `kubeconform` schema validation | PASS |
| kind Smoke Test | chart install + migration Job completion | PASS |
| Go Build | `go build ./...` | PASS (53 packages, 0 errors) |
| Go Test | `go test ./...` | PASS (0 regressions) |
| D2 Parity | EnsureSchema *.down.sql exclusion verified | PASS (latent data-loss bug fixed) |

## Evaluator-Active Verdict

| Dimension | Score | Threshold | Status |
|-----------|-------|-----------|--------|
| Functionality | 90 | ≥75 | PASS |
| Security | 92 | ≥75 | PASS |
| Craft | 88 | ≥75 | PASS |
| Consistency | 91 | ≥75 | PASS |
| **Fix Cycles** | **0** | — | PASS |

## Commit List (branch: feature/SPEC-DEPLOY-001)

| Commit | Description |
|--------|-------------|
| `5309478` | plan gate: SPEC-DEPLOY-001 plan-auditor PASS (v0.2.0 amendment) |
| `554f800` | impl: Helm chart charts/universal-search/ — 10-service topology + api/mcp Deployments + EnsureSchema Job + 2-tier secrets + ServiceMonitor + CI gates |

## Divergence Analysis

- Files in plan vs reality: 1:1 match (all `charts/universal-search/` artifacts planned in SPEC §2)
- Unplanned additions: none
- Scope drift: 0%
- All 24 EARS REQs + 8 NFRs implementation-mapped to chart artifacts

## SPEC Updates Applied

| File | Changes | Status |
|------|---------|--------|
| `.moai/specs/SPEC-DEPLOY-001/spec.md` | version 0.2.0→1.0.0, status approved→implemented, HISTORY entry appended | DONE |

## Documents Updated

| File | Lines | Type | Content |
|------|-------|------|---------|
| `CHANGELOG.md` | +15 | Unreleased/Added | SPEC-DEPLOY-001 entry under M9 V1 release block |
| `.moai/reports/sync-report-DEPLOY-001.md` | new | Sync report | Quality gates + evaluator verdict + carry-forward |

## Notable Findings

### D2 (Notable) — EnsureSchema *.down.sql exclusion

The EnsureSchema runner (`internal/index/pg/client.go:90`) previously executed ALL `*.sql`
files in `deploy/postgres/migrations/` in lexicographic order. This included
`0002_deep_runs.down.sql`, which issues a `DROP TABLE` statement. On a fresh install this
would silently destroy the `deep_runs` table immediately after creating it.

The Helm chart's migration Job (pre-install,pre-upgrade hook) runs via a `usearch migrate`
entrypoint that filters to `*.sql` files **excluding** `*.down.sql`. This fix is implemented
in the chart's Job spec and confirmed passing in the kind smoke test.

**Impact**: latent data-loss bug on forward install eliminated. No existing migration files
were renamed or reformatted (golang-migrate layout incompatibility noted; reformatting
deferred to post-V1 cleanup per SPEC amendment B1).

## Carry-Forward (deferred items accurately documented)

| Item | Status | Resolution Path |
|------|--------|----------------|
| cosign image signing | Deferred | REL-001 release workflow owns signing; blocked on `<org>/ghcr` registry resolution |
| SBOM (syft) + SLSA L2 provenance | Deferred | Fast-follow; REL-001 owns release signing pipeline |
| arm64 multi-arch | Deferred | V1 ships `linux/amd64` only; embedder was already amd64-only |
| `<org>/ghcr` registry placeholder | Unresolved | Image/chart PUSH deferred — build-verify only; resolve with REL-001/BOOT-001 |
| Tier-3 ExternalSecrets (ESO) | Deferred to V1.1 | Depends on SEC-001 PR#42 (unmerged); 2-tier secrets (tier-1 values + tier-2 existingSecret) ship in V1 |
| NFR-DEPLOY-008 SEC integration test | Deferred to V1.1 | Blocked on SEC-001 PR#42 merge |
| `.env.example` OIDC/JWT/SESSION_SECRET keys | Coordination item (OQ3) | compose↔chart parity script (REQ-DEPLOY-024) will FAIL until keys added — not a chart blocker |
| D1/D3/D4 plan-auditor minors | Carried | AC traceability, timing assertion, gate threshold — non-blocking, noted for V1.1 |

## Downstream Impact

SPEC-DEPLOY-001 is a release-blocking dependency of SPEC-REL-001. With status now
`implemented`, SPEC-REL-001 V1.0.0 tagging is unblocked (pending other V1 exit criteria).

## README Change

Skipped. The README Architecture section lists `deploy/ → docker-compose dev stack` which
remains accurate. No natural insertion point exists for a Helm pointer without restructuring
the Architecture block. A "Deployment (Helm)" section can be added when the chart reaches
a publishable state (post `<org>/ghcr` resolution in REL-001/BOOT-001).

---

**Sync Status**: READY FOR COMMIT
**Git Strategy**: main_direct (no push, no PR)
**Lifecycle Level**: 1 (spec-first)
