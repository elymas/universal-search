# SPEC-DEPLOY-001 Progress

## Status: IMPLEMENTING

Date: 2026-05-27

## Completed

### Phase 5.1-5.8: Chart Templates (DONE)

- **Chart.yaml**: apiVersion v2, kubeVersion >=1.28-0 <1.32-0, 3 subchart deps
  - postgresql 16.4.5 (Bitnami HTTPS)
  - redis 20.6.2 (Bitnami HTTPS)
  - qdrant 1.16.3 (official HTTPS)
- **values.yaml**: ~300 keys covering all services, 3-tier secret strategy, HPA/PDB/NetworkPolicy
- **_helpers.tpl**: 11 helpers (name, fullname, chart, labels, componentFullname, image with nil-safe AppVersion, secretEnvVar/Entries, serviceAccountName, databaseUrl, redisUrl)
- **API templates (11)**: deployment, service, configmap, secret, serviceaccount, hpa, pdb, networkpolicy, servicemonitor, ingress, externalsecret
- **MCP templates (9)**: deployment, service, configmap, secret, serviceaccount, servicemonitor, hpa, pdb, networkpolicy
- **Researcher templates (3)**: deployment, service, configmap
- **Embedder templates (4)**: deployment (startupProbe, GPU, amd64 nodeAffinity, PVC), service, configmap, pvc
- **Tokenizer-ko templates (3)**: deployment, service, configmap
- **Storm templates (3)**: deployment, service, configmap (default disabled)
- **Koreanews templates (3)**: deployment, service, configmap (default disabled)
- **LiteLLM templates (3)**: deployment (inline config.yaml volume), service, configmap
- **SearXNG templates (3)**: deployment (settings.yml volume, AGPL warning), service, configmap
- **Meilisearch templates (2)**: statefulset (volumeClaimTemplates), service
- **Jobs (2)**: migrate (pre-install/pre-upgrade hook), smoke-test (helm test)
- **NOTES.txt**: Post-install guidance

Total: **48 template files** (exceeds A1 gate of 30 minimum)

### Phase 5.9: Dockerfiles (DONE)

- **Dockerfile.usearch-api**: Multi-stage golang:1.24-alpine -> distroless/static-debian12:nonroot, CGO_ENABLED=0, USER 65532, EXPOSE 8080 9090, HEALTHCHECK
- **Dockerfile.usearch-mcp**: Same pattern, EXPOSE 8081
- **Dockerfile.usearch-migrate**: Alpine downloader -> distroless with golang-migrate v4.18.2 + /migrations

### Phase 5.10: CI Workflows (DONE)

- **chart-ci.yml**: PR-triggered helm lint, helm template, kubeconform 1.28-1.31, kind smoke-test
- **build-images.yml**: Main merge + tag, multi-arch builds, cosign keyless signing, SBOM via syft
- **chart-release.yml**: Tag-triggered helm package, cosign sign-blob, helm push to OCI

### Supporting Files (DONE)

- **ci/values-test.yaml**: Minimal (api + postgres + redis only)
- **ci/values-prod.yaml**: Production reference (HPA/PDB/NetworkPolicy, existingSecret tier 2, ingress, SSD)
- **ci/values-gpu.yaml**: Embedder GPU overlay with nvidia.com/gpu
- **values.schema.json**: JSON Schema Draft-07 with additionalProperties: false
- **README.md**: Install/upgrade/uninstall guide, secret tier docs, cosign verify, AGPL notice
- **scripts/compose-chart-parity.sh**: 3-way diff tool

## Validation Results

- `helm dependency build`: PASS (3 subcharts downloaded: postgresql, redis, qdrant)
- `helm lint`: PASS (0 errors, 0 failures)
- `helm template`: PASS (60 rendered manifests)
- Bitnami subchart registry fix: `global.security.allowInsecureImages: true` + explicit `image.registry: docker.io` per subchart

## Bugs Fixed

1. Image helper nil pointer on `.Chart.AppVersion` -> added nil-check + passed Chart context to all invocations
2. tokenizer-ko bash heredoc corruption (`$ko` expanded to empty) -> rewrote via Write tool
3. Qdrant chart version 0.1.2 -> corrected to 1.16.3
4. .helmignore `*.tgz` blocked subchart tgz scanning -> removed pattern, added `.venv/` exclusion
5. Unicode em-dash in Chart.yaml -> ASCII hyphen

## Remaining

- kubeconform validation against rendered output
- kind smoke test execution (requires running cluster)
- helm-unittest for template unit tests (REQ-DEPLOY-005)
- Acceptance gate verification against acceptance.md
