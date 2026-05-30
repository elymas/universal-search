---
id: SPEC-DEPLOY-001
version: 0.2.0
status: draft
created: 2026-05-26
updated: 2026-05-31
author: limbowl (via manager-spec)
related_spec: SPEC-DEPLOY-001 (spec.md, plan.md)
format: Given/When/Then
---

# SPEC-DEPLOY-001 Acceptance Scenarios

> AMENDMENT 2026-05-31 (v0.2.0) — reconciled to spec.md v0.2.0 against
> live code. Corrections: Dockerfile path `deploy/Dockerfile.usearch-*`
> (not root `Dockerfile.api`); Go base `golang:1.25-alpine` (not 1.23);
> migration via existing `usearch migrate` / EnsureSchema runner (not
> golang-migrate) as a `pre-install,pre-upgrade` hook (not post-install);
> Python sidecars REUSE `services/*/Dockerfile` (no new sidecar
> Dockerfiles). V1 amd64-only (arm64 deferred). Signing/SBOM/SLSA + ESO
> tier-3 + NFR-008 SEC integration deferred to fast-follow / V1.1.

## 0. Document Purpose

This document specifies acceptance criteria for SPEC-DEPLOY-001 in Given/When/Then format, complementing the EARS requirement table in spec.md §2 and the test-scenario list in spec.md §5 (S1..S12). The criteria below are the externally-observable behaviors that the run phase MUST verify before declaring DEPLOY-001 release-ready.

Scope: 14 acceptance criteria (AC-001..AC-014) covering REQ-DEPLOY-001 through REQ-DEPLOY-024 + NFR-DEPLOY-001 through NFR-DEPLOY-008, plus 3 edge-case sections, plus a Definition of Done checklist.

Coverage policy: every REQ and every NFR in spec.md §2 / §3 has ≥1 matching AC below. See the Coverage Matrix at the end of this file.

---

## 1. Acceptance Criteria (Given/When/Then)

### AC-001 — Single Helm chart at canonical path passes `helm lint --strict`

Covers: REQ-DEPLOY-001, REQ-DEPLOY-020

**Given** the repository checkout with `charts/universal-search/Chart.yaml` (apiVersion: v2, name: universal-search, semver version), `charts/universal-search/values.yaml`, `charts/universal-search/values.schema.json`, and a populated `charts/universal-search/templates/` directory.

**When** the contributor runs:
```
helm lint --strict charts/universal-search
```

**Then**:
- Exit code is `0`.
- stdout contains zero `[ERROR]` lines (warnings are permitted).
- The chart directory exists at exactly `charts/universal-search/` (single canonical location, not duplicated).
- `find charts/universal-search/templates -name '*.yaml' | wc -l` returns `≥ 30`.

Verification: scenario S1 in spec.md §5; gate A1 + A2 in spec.md §6.

---

### AC-002 — Multi-stage Go Dockerfile produces minimal API image

Covers: REQ-DEPLOY-002

**Given** the file `deploy/Dockerfile.usearch-api` with multi-stage build (builder stage = `golang:1.25-alpine`, matching `go.mod` toolchain `go 1.25.8`; runtime stage = `gcr.io/distroless/static-debian12:nonroot`).

**When** CI builds the image via `docker buildx build --platform linux/amd64 --target runtime -t test-api:local -f deploy/Dockerfile.usearch-api .`.

**Then**:
- The image builds successfully on `linux/amd64` (arm64 multi-arch deferred to V1.1 per NFR-DEPLOY-007).
- The runtime image runs as `USER nonroot` (UID 65532).
- The runtime image exposes port `8080`.
- The image size is `< 100 MB` (distroless static; no shell, no package manager).
- (DEFERRED) `cosign verify` on the published image — image signing is a fast-follow owned by REL-001 (see AC-012).

---

### AC-003 — Migration Dockerfile + reuse of existing Python sidecar Dockerfiles

Covers: REQ-DEPLOY-003

**Given** the new `deploy/Dockerfile.usearch-migrate` and the EXISTING Python sidecar Dockerfiles at `services/{researcher,embedder,tokenizer-ko,storm,koreanews}/Dockerfile` (all five verified present — the chart references their images and does NOT author new sidecar Dockerfiles).

**When** CI builds the migrate image via `docker buildx build --platform linux/amd64 -f deploy/Dockerfile.usearch-migrate .`, and the sidecar images are built from their existing `services/<name>/Dockerfile`.

**Then**:
- `deploy/Dockerfile.usearch-migrate` runtime is distroless and its entrypoint is `usearch migrate` (the `internal/index/pg` EnsureSchema runner — NOT golang-migrate), with `deploy/postgres/migrations/` COPY-ed in.
- The chart does NOT introduce `Dockerfile.embedder` / `.tokenizer-ko` / `.storm` / `.koreanews`; sidecar images are referenced by tag from values.
- `embedder` image is `linux/amd64` only (torch + CUDA constraint; documented in NFR-DEPLOY-007).
- V1 builds amd64 only for the Go images; arm64 multi-arch is deferred to V1.1.

---

### AC-004 — Per-service Deployment manifests render with correct topology

Covers: REQ-DEPLOY-004

**Given** chart + `ci/values-prod.yaml`.

**When** the contributor runs:
```
helm template universal-search charts/universal-search -f charts/universal-search/ci/values-prod.yaml
```

**Then**:
- stdout contains one `apiVersion: apps/v1, kind: Deployment` per ENABLED workload. Default-enabled: the 2 newly-containerized host binaries (api, mcp) + compose-derived services (researcher, embedder, tokenizer-ko, litellm, searxng, meilisearch). `storm` + `koreanews` are `enabled: false` by default (services/ dirs only, not compose services) and render NO Deployment unless opted in.
- Each Deployment has `spec.replicas` matching its values key (default: api=2, sidecars=1).
- Each Deployment has `resources.requests` and `resources.limits` set.
- Each Deployment has `securityContext.runAsNonRoot: true`.

---

### AC-005 — `values.schema.json` rejects malformed configuration BEFORE k8s resource creation

Covers: REQ-DEPLOY-005, NFR-DEPLOY-001

**Given** an invalid values file `ci/values-invalid.yaml` containing:
```yaml
usearch:
  api:
    replicas: "two"   # type: string, schema requires integer ≥ 1
```

**When** the contributor runs:
```
helm install usearch charts/universal-search -f ci/values-invalid.yaml --dry-run
```

**Then**:
- Exit code is non-zero.
- stderr contains a schema validation error identifying the path `usearch.api.replicas` and the type mismatch (`expected integer, got string`).
- Zero kubernetes resources are created (validation fails BEFORE API server contact).

Verification: scenario S6 in spec.md §5.

---

### AC-006 — Migration Job runs `usearch migrate` (EnsureSchema) idempotently as a pre-install/pre-upgrade hook

Covers: REQ-DEPLOY-006

**Given** a fresh kind cluster (k8s 1.30) + chart + `ci/values-test.yaml`.

**When** the contributor runs `helm install usearch charts/universal-search -f ci/values-test.yaml --wait --timeout 5m`, then immediately runs `helm upgrade usearch charts/universal-search -f ci/values-test.yaml` (same chart version, no schema change).

**Then**:
- Initial `helm install` exits `0`; migration Job (`Job/usearch-migrate-<rev>`) runs `usearch migrate` (the `internal/index/pg` EnsureSchema runner — NOT golang-migrate; execs all `*.sql` in `deploy/postgres/migrations/` lexicographically) and completes `Succeeded`.
- `helm upgrade` exits `0`; migration Job re-runs and completes `Succeeded` (idempotent — EnsureSchema re-exec on an existing schema is a no-op plus a drift check; `CREATE TABLE/INDEX IF NOT EXISTS` semantics preserved).
- No schema modification between runs (verified via `psql -c '\d'` snapshot comparison).
- The migration Job carries `helm.sh/hook: pre-install,pre-upgrade` annotation with `hook-weight: "-5"`, so the schema is ensured BEFORE any application Deployment starts.

Verification: scenarios S4, S7 in spec.md §5.

---

### AC-007 — ConfigMap per service is generated from values

Covers: REQ-DEPLOY-007

**Given** chart + `ci/values-prod.yaml` with service-specific config blocks (e.g., `usearch.api.config.logLevel: "info"`).

**When** `helm template` is run.

**Then**:
- stdout contains one `kind: ConfigMap` per service with declared config.
- ConfigMap `data` keys match the values structure.
- Deployments mount the matching ConfigMap as env-from or volume per chart convention.

---

### AC-008 — All secret material is sourced via `secretKeyRef` (no plaintext in manifests)

Covers: REQ-DEPLOY-008, REQ-DEPLOY-016

**Given** chart + `ci/values-prod.yaml`.

**When** `helm template universal-search charts/universal-search -f ci/values-prod.yaml | grep -E "(password|secret|token|apiKey):"` is executed.

**Then**:
- Zero plaintext secret values appear in rendered output (all references go through `secretKeyRef.name` + `secretKeyRef.key`).
- All `secretKeyRef.name` references resolve to either an in-chart Secret (`backend: values`) or a pre-existing K8s Secret (`backend: existingSecret`).
- Switching `secrets.backend` from `values` → `existingSecret` removes the chart-managed Secret and reconfigures Deployments without breaking the rendered manifest's `secretKeyRef` resolution (see scenario S9 in spec.md §5).

Verification: scenarios S2, S9 in spec.md §5.

---

### AC-009 — Service + ServiceAccount + RBAC per service

Covers: REQ-DEPLOY-009

**Given** chart + `ci/values-prod.yaml`.

**When** `helm template` is run.

**Then**:
- One `Service` (ClusterIP) per Deployment with matching selector labels.
- One `ServiceAccount` per service (named `<release>-<service>`).
- Each ServiceAccount has minimal RBAC (no cluster-admin; namespace-scoped Role + RoleBinding).

---

### AC-010 — NetworkPolicy is enforced when CNI supports it

Covers: REQ-DEPLOY-010

**Given** chart installed on a kind cluster with Cilium CNI + `networkPolicy.enabled: true`.

**When** a test Pod in a different namespace executes:
```
curl --max-time 5 http://usearch-api.usearch.svc.cluster.local:8080/healthz
```

**Then**:
- The connection is denied (curl exits with timeout or "connection refused").
- Traffic from an in-namespace Pod to `usearch-api:8080` SUCCEEDS (whitelist still works).
- A warning is logged in NOTES.txt if the cluster's CNI does not enforce NetworkPolicy (graceful degradation).

Verification: scenario S10 in spec.md §5.

---

### AC-011 — Per-Python-sidecar PVC for model cache and per-service PDB

Covers: REQ-DEPLOY-011, REQ-DEPLOY-012, REQ-DEPLOY-013

**Given** chart with `embedder.persistence.enabled: true` + `pdb.enabled: true`.

**When** `helm template` is run.

**Then**:
- A `PersistentVolumeClaim` is rendered for `embedder` with `storage` matching `embedder.persistence.size` (default 5Gi).
- The PVC `storageClassName` defers to `global.storageClass` or cluster default if unset.
- A `PodDisruptionBudget` is rendered per service with `maxUnavailable: 1` (default) or value from values.
- The embedder Deployment mounts the PVC at `/models` (or per-sidecar conventional mount point).

---

### AC-012 — amd64 image build (V1) + reproducible manifests; signing deferred

Covers: REQ-DEPLOY-018, NFR-DEPLOY-007, NFR-DEPLOY-002

**Given** the Go images built via `.github/workflows/build-images.yml` (V1: amd64 BUILD + verify; registry PUSH blocked on unresolved `<org>`).

**When** the contributor runs:
```
docker buildx build --platform linux/amd64 -f deploy/Dockerfile.usearch-api .
```

**Then**:
- The amd64 image builds successfully for api/mcp/migrate. `linux/arm64` is NOT expected in V1 (deferred to V1.1; embedder is amd64-only regardless).
- (DEFERRED to fast-follow) `cosign verify` for signed images — image signing is owned by SPEC-REL-001's release workflow and blocked on `<org>` registry resolution; NOT a V1 gate.
- The same Chart version rendered twice on an identical cluster produces byte-identical manifests (`diff` of two `helm template` runs is empty) — NFR-DEPLOY-002.

Verification: scenario S11 in spec.md §5 (S12 deferred); gate A5 in spec.md §6 (A6 deferred).

---

### AC-013 — `helm test` smoke pod verifies `/healthz` and `/metrics`

Covers: REQ-DEPLOY-022

**Given** a successful `helm install` on a kind cluster.

**When** the contributor runs:
```
helm test usearch
```

**Then**:
- The smoke-test Pod runs to completion with exit code `0`.
- Smoke test contains at minimum: `GET /healthz` → 200, `GET /metrics` → 200 (Prometheus format), `GET /readyz` → 200.
- Total cold install + helm test PASS time is `< 5 minutes` on a 3-node k8s 1.30 cluster (NFR-DEPLOY-003).
- `helm rollback usearch 1` after an upgrade restores the previous revision's manifests AND triggers idempotent migration Job (NFR-DEPLOY-004).

Verification: scenarios S4, S5, S8 in spec.md §5.

---

### AC-014 — OCI chart publication, SBOM attestation, compose ↔ chart parity, ServiceMonitor wiring

Covers: REQ-DEPLOY-014, REQ-DEPLOY-015, REQ-DEPLOY-017, REQ-DEPLOY-019, REQ-DEPLOY-021, REQ-DEPLOY-023, REQ-DEPLOY-024, NFR-DEPLOY-005, NFR-DEPLOY-006, NFR-DEPLOY-008

**Given** a tagged release commit on the `main` branch.

**When** the release pipeline runs `.github/workflows/chart-release.yml`.

**Then**:
- (V1) `helm package` produces a valid chart tarball and verifies cleanly. OCI PUSH to `oci://ghcr.io/<org>/charts/universal-search` and `helm pull` are DEFERRED to fast-follow — blocked on unresolved `<org>` (REQ-DEPLOY-017).
- (DEFERRED to fast-follow) A SPDX SBOM attached to each image via `cosign attest` (gate A7) — ships with the signing fast-follow.
- The chart manifests declare a `ServiceMonitor` resource when `serviceMonitor.enabled: true` and the Prometheus Operator CRD is present; when the CRD is absent, pod-annotation scrape hints (`prometheus.io/scrape`, `prometheus.io/port`, `prometheus.io/path`) are emitted instead (REQ-DEPLOY-014, REQ-DEPLOY-019 fallback).
- ConfigMap or values surface OTLP exporter env vars (`OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_RESOURCE_ATTRIBUTES`) consumed by all services (REQ-DEPLOY-015).
- `scripts/compose-chart-parity.sh` reports zero unexplained delta between `deploy/docker-compose.yml` + `.env.example` and the chart values surface (REQ-DEPLOY-024, gate A8) — NOTE: this FAILs until `.env.example` gains `OIDC_*`/`JWT_*`/`SESSION_SECRET` (coordination item OQ3), at which point it passes.
- When `ingress.enabled: true`, an `Ingress` resource is rendered with TLS annotations for cert-manager (REQ-DEPLOY-021).
- (DEFERRED to V1.1) `secrets.backend: "externalSecrets"` ExternalSecret emission — depends on SEC-001 PR#42; in V1 the schema RESERVES the enum but selecting it blocks install with a "V1.1 feature" message (REQ-DEPLOY-023).
- `Chart.lock` pins every subchart to an exact patch version, not a range (NFR-DEPLOY-005).
- The chart README documents `global.imagePullSecrets` configuration for Docker Hub rate-limit mitigation (NFR-DEPLOY-006).
- (DEFERRED to post SEC-001 PR#42 merge) the cross-SPEC integration test confirming chart-rendered Secret refs work end-to-end with SEC-001's `internal/security/secrets` (NFR-DEPLOY-008) — NOT a V1 gate; chart is decoupled.
- When `serviceMonitor.enabled: false`, no ServiceMonitor is rendered (REQ-DEPLOY-019 state-driven gate).

---

## 2. Edge Cases

### EC-001 — Bitnami subchart minor-version drift breaks chart render

**Given** `Chart.lock` pins `postgresql` to an exact patch (e.g. `16.4.5`, per NFR-DEPLOY-005 — no `~`/`^` ranges) and a quarterly audit considers Bitnami `16.5.x` which carries a breaking change to the values schema.

**When** the quarterly audit (NFR-DEPLOY-005) runs `helm dependency update`.

**Then**:
- CI detects the schema break via integration test (helm template + kubeconform fails on the new lock).
- The PR is blocked; the audit produces a CHANGELOG entry documenting the change and either the upgrade or the deferral.
- Operators are not surprised at deploy time (drift is caught in CI, not in production).

### EC-002 — Operator on arm64-only cluster attempts to install chart

**Given** an arm64-only k8s cluster (e.g., Graviton-only EKS node group).

**When** `helm install usearch ...` is executed.

**Then**:
- Non-embedder Deployments schedule successfully (all multi-arch).
- `embedder` Deployment Pods enter `Pending` with `FailedScheduling` event citing nodeAffinity mismatch (`kubernetes.io/arch: amd64`).
- NOTES.txt explicitly warns about this constraint and points to the `embedder` external-mode (`embedder.enabled: false` + remote endpoint).

### EC-003 — Helm rollback after a values-only change

**Given** chart installed at revision 1, then upgraded to revision 2 (values change only, no migration schema change).

**When** `helm rollback usearch 1` is executed.

**Then**:
- Revision 1's manifests are re-applied.
- Pods restart with revision 1's config (verified via env-var diff).
- Migration Job re-runs and succeeds (idempotent).
- No data loss occurs (no down-migration is invoked — this is explicitly out-of-scope per spec.md §4.2).

---

## 3. Definition of Done Checklist

- [ ] All 14 AC scenarios pass on a CI kind cluster (signing/SBOM/ESO/NFR-008 items marked DEFERRED do not block V1).
- [ ] V1 test scenarios from spec.md §5 are implemented as automated tests (S12 cosign deferred to fast-follow).
- [ ] V1 acceptance gates in spec.md §6 PASS (A6/A7 deferred; A9 = package-verify only).
- [ ] `helm-unittest` coverage for chart helpers + scripts ≥ 85% (gate A11).
- [ ] `gitleaks` + `Trivy` report zero finding (gate A12; `cosign verify` deferred to signing fast-follow).
- [ ] Every PR cites `SPEC-DEPLOY-001` in commit message (gate A13).
- [ ] DOC-001 cross-link integrity verified (gate A10).
- [ ] Open Questions OQ1..OQ7 in spec.md §8 are resolved or explicitly deferred with mitigation in CHANGELOG.
- [ ] (DEFERRED) NFR-DEPLOY-008 cross-SPEC integration test PASSES after SPEC-SEC-001 PR#42 merges — not a V1 gate.
- [ ] `values.schema.json` documented in DOC-002 (`operators/configuration/chart-values.mdx`).
- [ ] `.env.example` gains `OIDC_*`/`JWT_*`/`SESSION_SECRET` so the parity script passes (coordination item OQ3).
- [ ] CI workflow `chart-ci.yml` runs on every PR touching `charts/` and BLOCKS merge on failure.
- [ ] CI workflow `chart-release.yml` runs on tag `v*.*.*` and packages + verifies the chart (signed chart + signed-image PUSH deferred to fast-follow, blocked on `<org>`).

---

## 4. Coverage Matrix (REQ → AC)

| REQ / NFR | AC-001 | AC-002 | AC-003 | AC-004 | AC-005 | AC-006 | AC-007 | AC-008 | AC-009 | AC-010 | AC-011 | AC-012 | AC-013 | AC-014 | EC |
|-----------|--------|--------|--------|--------|--------|--------|--------|--------|--------|--------|--------|--------|--------|--------|----|
| REQ-DEPLOY-001 | ✓ |   |   |   |   |   |   |   |   |   |   |   |   |   |   |
| REQ-DEPLOY-002 |   | ✓ |   |   |   |   |   |   |   |   |   |   |   |   |   |
| REQ-DEPLOY-003 |   |   | ✓ |   |   |   |   |   |   |   |   |   |   |   |   |
| REQ-DEPLOY-004 |   |   |   | ✓ |   |   |   |   |   |   |   |   |   |   |   |
| REQ-DEPLOY-005 |   |   |   |   | ✓ |   |   |   |   |   |   |   |   |   |   |
| REQ-DEPLOY-006 |   |   |   |   |   | ✓ |   |   |   |   |   |   |   |   |   |
| REQ-DEPLOY-007 |   |   |   |   |   |   | ✓ |   |   |   |   |   |   |   |   |
| REQ-DEPLOY-008 |   |   |   |   |   |   |   | ✓ |   |   |   |   |   |   |   |
| REQ-DEPLOY-009 |   |   |   |   |   |   |   |   | ✓ |   |   |   |   |   |   |
| REQ-DEPLOY-010 |   |   |   |   |   |   |   |   |   | ✓ |   |   |   |   |   |
| REQ-DEPLOY-011 |   |   |   |   |   |   |   |   |   |   | ✓ |   |   |   |   |
| REQ-DEPLOY-012 |   |   |   |   |   |   |   |   |   |   | ✓ |   |   |   |   |
| REQ-DEPLOY-013 |   |   |   |   |   |   |   |   |   |   | ✓ |   |   |   |   |
| REQ-DEPLOY-014 |   |   |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |
| REQ-DEPLOY-015 |   |   |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |
| REQ-DEPLOY-016 |   |   |   |   |   |   |   | ✓ |   |   |   |   |   |   |   |
| REQ-DEPLOY-017 |   |   |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |
| REQ-DEPLOY-018 |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |   |   |
| REQ-DEPLOY-019 |   |   |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |
| REQ-DEPLOY-020 | ✓ |   |   |   |   |   |   |   |   |   |   |   |   |   |   |
| REQ-DEPLOY-021 |   |   |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |
| REQ-DEPLOY-022 |   |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |   |
| REQ-DEPLOY-023 |   |   |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |
| REQ-DEPLOY-024 |   |   |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |
| NFR-DEPLOY-001 |   |   |   |   | ✓ |   |   |   |   |   |   |   |   |   |   |
| NFR-DEPLOY-002 |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |   |   |
| NFR-DEPLOY-003 |   |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |   |
| NFR-DEPLOY-004 |   |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |   |
| NFR-DEPLOY-005 |   |   |   |   |   |   |   |   |   |   |   |   |   | ✓ | EC-001 |
| NFR-DEPLOY-006 |   |   |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |
| NFR-DEPLOY-007 |   |   | ✓ |   |   |   |   |   |   |   |   | ✓ |   |   | EC-002 |
| NFR-DEPLOY-008 |   |   |   |   |   |   |   |   |   |   |   |   |   | ✓ |   |

Every REQ and NFR has ≥ 1 AC; edge cases supplement EC-001..EC-003.

---

*End of SPEC-DEPLOY-001 acceptance.md.*
