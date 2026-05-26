---
id: SPEC-DEPLOY-001
version: 0.1.0
status: draft
created: 2026-05-26
author: limbowl (via manager-spec)
related_spec: SPEC-DEPLOY-001 (spec.md, plan.md)
format: Given/When/Then
---

# SPEC-DEPLOY-001 Acceptance Scenarios

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

**Given** the file `Dockerfile.api` at repository root with multi-stage build (builder stage = `golang:1.23-alpine`, runtime stage = `gcr.io/distroless/static-debian12:nonroot`).

**When** CI builds the image via `docker buildx build --target runtime -t test-api:local -f Dockerfile.api .`.

**Then**:
- The image builds successfully on both `linux/amd64` and `linux/arm64`.
- The runtime image runs as `USER nonroot` (UID 65532).
- The runtime image exposes port `8080`.
- The image size is `< 100 MB` (distroless static; no shell, no package manager).
- `cosign verify` on the published image PASSES (see AC-012).

---

### AC-003 — Multi-stage Python sidecar Dockerfiles produce hardened runtime images

Covers: REQ-DEPLOY-003

**Given** the Dockerfiles for Python sidecars (`Dockerfile.embedder`, `Dockerfile.tokenizer-ko`, `Dockerfile.storm`, `Dockerfile.koreanews`).

**When** CI builds each image via `docker buildx build -f Dockerfile.<sidecar> .`.

**Then**:
- Each runtime stage uses a slim base (e.g., `python:3.12-slim` or distroless) — not full Python image.
- Each image exposes its declared port (embedder 8000, tokenizer-ko 9000, etc.).
- Each image runs as non-root user.
- `embedder` image is `linux/amd64` only (torch + CUDA constraint; documented in NFR-DEPLOY-007).
- All other sidecar images are multi-arch (amd64 + arm64).

---

### AC-004 — Per-service Deployment manifests render with correct topology

Covers: REQ-DEPLOY-004

**Given** chart + `ci/values-prod.yaml`.

**When** the contributor runs:
```
helm template universal-search charts/universal-search -f charts/universal-search/ci/values-prod.yaml
```

**Then**:
- stdout contains one `apiVersion: apps/v1, kind: Deployment` per declared service (api, embedder, tokenizer-ko, storm, koreanews, mcp).
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

### AC-006 — Migration Job runs `golang-migrate up` idempotently on install and upgrade

Covers: REQ-DEPLOY-006

**Given** a fresh kind cluster (k8s 1.30) + chart + `ci/values-test.yaml`.

**When** the contributor runs `helm install usearch charts/universal-search -f ci/values-test.yaml --wait --timeout 5m`, then immediately runs `helm upgrade usearch charts/universal-search -f ci/values-test.yaml` (same chart version, no schema change).

**Then**:
- Initial `helm install` exits `0`; migration Job (`Job/usearch-migrate-<rev>`) completes `Succeeded`.
- `helm upgrade` exits `0`; migration Job re-runs and completes `Succeeded` (idempotent — `CREATE TABLE IF NOT EXISTS` semantics preserved).
- No schema modification between runs (verified via `psql -c '\d'` snapshot comparison).
- The migration Job carries `helm.sh/hook: post-install,post-upgrade` annotation.

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

### AC-012 — Multi-arch image manifest + cosign signature verification

Covers: REQ-DEPLOY-018, NFR-DEPLOY-007, NFR-DEPLOY-002

**Given** the image `ghcr.io/<org>/usearch-api:1.0.0` published via `.github/workflows/build-images.yml`.

**When** the contributor runs:
```
docker buildx imagetools inspect ghcr.io/<org>/usearch-api:1.0.0
cosign verify ghcr.io/<org>/usearch-api:1.0.0 \
  --certificate-identity-regexp 'https://github.com/<org>/universal-search/' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com'
```

**Then**:
- The manifest list contains both `linux/amd64` and `linux/arm64` entries (except `embedder` — amd64-only by NFR-DEPLOY-007).
- `cosign verify` exits `0` for every image listed in the chart.
- The same Chart version installed twice on identical cluster produces byte-identical rendered manifests (`diff` of two `helm template` runs is empty) — NFR-DEPLOY-002.

Verification: scenarios S11, S12 in spec.md §5; gates A5, A6 in spec.md §6.

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
- The chart is pushed to `oci://ghcr.io/<org>/charts/universal-search` (REQ-DEPLOY-017); `helm pull oci://...` succeeds.
- A SPDX SBOM is attached to each image via `cosign attest --predicate-type=spdx.dev/Document` (gate A7); `cosign download attestation` retrieves the SBOM.
- The chart manifests declare a `ServiceMonitor` resource (when `serviceMonitor.enabled: true`) that targets every Service's `/metrics` endpoint (REQ-DEPLOY-014).
- ConfigMap or values surface OTLP exporter env vars (`OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_RESOURCE_ATTRIBUTES`) consumed by all services (REQ-DEPLOY-015).
- `scripts/compose-chart-parity.sh` reports zero unexplained delta between `deploy/docker-compose.yml` and rendered chart manifests (REQ-DEPLOY-024, gate A8).
- When `ingress.enabled: true`, an `Ingress` resource is rendered with TLS annotations for cert-manager (REQ-DEPLOY-021).
- When `secrets.backend: "externalSecrets"`, an `ExternalSecret` CR is rendered for ESO integration (REQ-DEPLOY-023).
- `Chart.lock` pins every subchart to a major.minor version (NFR-DEPLOY-005).
- The chart README documents `global.imagePullSecrets` configuration for Docker Hub rate-limit mitigation (NFR-DEPLOY-006).
- A cross-SPEC integration test confirms that the chart-rendered Secret references work end-to-end with SPEC-SEC-001's `internal/security/secrets` once both ship (NFR-DEPLOY-008).
- When `serviceMonitor.enabled: false`, no ServiceMonitor is rendered (REQ-DEPLOY-019 state-driven gate).

---

## 2. Edge Cases

### EC-001 — Bitnami subchart minor-version drift breaks chart render

**Given** `Chart.lock` pins `postgresql ~16.4` and Bitnami publishes `16.5` with a breaking change to the values schema.

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

- [ ] All 14 AC scenarios pass on a CI kind cluster.
- [ ] All 12 test scenarios from spec.md §5 (S1..S12) are implemented as automated tests.
- [ ] All 13 acceptance gates A1..A13 in spec.md §6 PASS.
- [ ] `helm-unittest` coverage for chart helpers + scripts ≥ 85% (gate A11).
- [ ] `gitleaks` + `Trivy` + `cosign verify` report zero finding (gate A12).
- [ ] Every PR cites `SPEC-DEPLOY-001` in commit message (gate A13).
- [ ] DOC-001 cross-link integrity verified (gate A10).
- [ ] Open Questions OQ1..OQ7 in spec.md §8 are resolved or explicitly deferred with mitigation in CHANGELOG.
- [ ] NFR-DEPLOY-008 cross-SPEC integration test PASSES after SPEC-SEC-001 ships.
- [ ] `values.schema.json` documented in DOC-002 (`operators/configuration/chart-values.mdx`).
- [ ] CI workflow `chart-ci.yml` runs on every PR touching `charts/` and BLOCKS merge on failure.
- [ ] CI workflow `chart-release.yml` runs on tag `v*.*.*` and publishes signed chart + signed images to GHCR.

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
