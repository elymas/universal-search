# SPEC-DEPLOY-001 Tasks — atomic decomposition

Status: Phase 1 analysis output (manager-strategy). Harness: thorough (P0 + migration domain → plan-auditor + Sprint Contract MANDATORY).
Methodology: DDD (per-SPEC override of repo-default TDD; justified — chart extracted from existing dev-compose).

> IMPORTANT — recommendation is `needs-plan-auditor-first`. Several SPEC core assumptions are stale (see progress.md / blockers). Do NOT begin IMPROVE until B1 (migration tool) and the spec/acceptance contradictions are resolved by plan-auditor + user.

## Pre-implementation blockers (resolve before IMPROVE)

- **B1 [BLOCKER]** — Migration tool mismatch. Spec D4/REQ-DEPLOY-003/006 assume `golang-migrate v4.18` over `deploy/postgres/migrations/`, but the actual files are applied by a custom Go runner (`internal/index/pg/client.go:87 EnsureSchema`) that lexicographically execs ALL `*.sql`. Files are NOT golang-migrate-shaped: duplicate version prefixes (two `0002_*`, two `0003_*`), mixed bare-`.sql` vs `.up.sql` suffixes, no `schema_migrations` table. golang-migrate will error on duplicate versions and ignore bare `.sql`. RESOLUTION OPTIONS for plan-auditor/user: (a) migration Job runs `usearch-migrate` = the existing Go `EnsureSchema` runner (re-use, not golang-migrate); (b) renumber/rename all SQL into golang-migrate format (cross-SPEC SQL ownership change — heavy); (c) keep custom runner, drop golang-migrate from spec. Recommend (a).
- **B2 [BLOCKER-coordination]** — SEC-001 not on main. `internal/security/` does not exist on `main` (entirely on PR #42). Chart's 3-tier secret strategy forward-references SEC-001's resolver, but the chart only needs to template K8s Secrets — decoupled per research §14.1. Confirm chart ships without SEC-001; NFR-DEPLOY-008 integration test deferred to post-PR#42 merge.
- **B3 [spec contradiction]** — Dockerfile path/name + base image + hook timing disagree between spec.md and acceptance.md (see progress.md §contradictions). plan-auditor must pick one before B-tasks author Dockerfiles/Job.

## Task list (atomic; ≤10; each one DDD cycle)

> STATUS 2026-05-31 (IMPROVE complete, expert-devops): B1→(a) EnsureSchema runner; B3→spec wins (deploy/Dockerfile.usearch-*, golang:1.25-alpine, pre-install,pre-upgrade); B2→chart decoupled. T1-T10 ALL DONE. D2 *.down.sql fix landed at source (internal/index/pg) + characterization test. helm lint --strict exit 0, schema rejects invalid, parity OK, go test ./... 53 OK / 0 FAIL. kubeconform + kind = CI-only (binaries absent locally). Image build = build-verify wired (no docker daemon locally). Deferrals per scope: cosign/SBOM/SLSA/arm64/push, tier-3 ESO, NFR-008.

| ID | Status | Task | REQ mapping | Depends | Acceptance |
|----|--------|------|-------------|---------|------------|
| T1 | DONE | ANALYZE: reconcile real topology (10 compose services, no storm/koreanews/api/mcp in compose) into values.yaml service map; resolve B1 migration mechanism; resolve B3 naming contradictions | REQ-004, D2, OQ1 | B1,B3 | research↔spec service map 1:1; migration tool decision recorded in CHANGELOG |
| T2 | DONE | Chart skeleton: Chart.yaml (v2, subchart deps pinned), values.yaml (~300 keys reflecting ACTUAL 10 services + api/mcp), values.schema.json (strict), _helpers.tpl, NOTES/README/CHANGELOG/.helmignore | REQ-001,004,005,014,016 | T1 | `helm lint --strict` exit 0; schema validates own values.yaml |
| T3 | DONE | Dockerfiles: usearch-api, usearch-mcp, usearch-migrate (migrate = existing Go EnsureSchema runner per B1, NOT golang-migrate unless B1 chooses (b)) | REQ-002,003; NFR-007 | T1 | `docker buildx` amd64+arm64 succeeds; distroless non-root; <100MB |
| T4 | DONE | api + mcp template set (Deployment/Service/SA/HPA/PDB/NetworkPolicy/ConfigMap/Secret/ServiceMonitor/Ingress/ExternalSecret) | REQ-009,010,013,016,019,021,023 | T2 | helm-unittest: enabled toggles + 3 secret backends + probe params render; kubeconform PASS |
| T5 | DONE | Python sidecar templates: researcher, embedder(+PVC), tokenizer-ko, storm(disabled), koreanews(disabled); GPU state-driven; amd64 nodeAffinity for embedder | REQ-011,012,013; NFR-007 | T4 | enabled:false omits all; GPU overlay renders nvidia.com/gpu; embedder PVC + startupProbe ≥120s |
| T6 | DONE | In-chart custom: litellm, searxng (AGPL notice), meilisearch (StatefulSet); image tags = exact compose tags | REQ-015 | T4 | tags match compose; AGPL warning in NOTES+README |
| T7 | DONE | Jobs: migrate (pre-install/pre-upgrade hook per B3 resolution), smoke-test (helm test /healthz+/metrics) | REQ-006,022; NFR-004 | T3,T4 | hook-weight ordering; idempotent re-run; helm test PASS on kind |
| T8 | DONE | Multi-env values overlays: values-test (minimal: api+pg+redis), values-prod, values-gpu | REQ-005, D8 | T4-T7 | each `helm template` succeeds; prod enables HPA/PDB/NetPol+secret tier2 |
| T9 | DONE | CI workflows: chart-ci.yml (lint+template+kubeconform 1.28-1.31+kind smoke+parity), build-images.yml (7 img multi-arch+cosign+SBOM+SLSA), chart-release.yml (OCI+cosign) | REQ-017,018,020,024; NFR-002 | T2-T8 | all gates green within hosted-runner limits; parity script catches drift |
| T10 | DONE | DOC cross-link + `.env.example` OIDC/JWT/SESSION_SECRET sync (currently ABSENT — OQ3); parity script | REQ-007,008,024; OQ3 | T2,T9 | README/NOTES cross-link DOC-001; parity passes after env sync |

## Deferral candidates (V1-essential vs deferrable — for user decision)

V1-ESSENTIAL (chart deploys the real 10 services + api/mcp to k8s): T1-T8, chart-ci (T9 partial: lint+template+kubeconform+kind).
DEFERRABLE to post-V1 (flag to user):
- arm64 multi-arch — amd64-only may suffice for V1 ship (cuts buildx matrix + per-sidecar arm64 risk R12).
- cosign keyless + SBOM(syft) + SLSA L2 — supply-chain hardening; can ship as fast-follow if V1 deadline tight.
- kind smoke-test depth — minimal (api+pg+redis) is already the plan; full-stack deferred to self-hosted runner (R7).
- ServiceMonitor — if operator scrapes manually, OBS wiring can be ConfigMap-only V1.
- Tier-3 ExternalSecrets (REQ-023, P2) — depends on SEC-001 (PR#42); defer to post-merge.
- meilisearch StatefulSet polish, NetworkPolicy egress allowlist completeness.
