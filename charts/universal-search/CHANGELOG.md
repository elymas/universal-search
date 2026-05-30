# Changelog — universal-search Helm chart

All notable changes to this chart. Subchart version bumps are recorded here per
NFR-DEPLOY-005 (quarterly audit).

## 0.1.0 — 2026-05-31 (SPEC-DEPLOY-001)

Initial chart. Templates the 10-service dev-compose topology + 2 newly
containerized Go binaries (usearch-api, usearch-mcp).

### Subchart pins (exact patch versions)

| subchart | version | source |
|---|---|---|
| postgresql | 16.4.5 | Bitnami (oci://registry-1.docker.io/bitnamicharts) |
| redis | 20.6.1 | Bitnami |
| qdrant | 1.13.1 | official (https://qdrant.github.io/qdrant-helm) |

### Notes

- V1 scope: amd64-only images, 2-tier secrets (values + existingSecret), build-verify CI.
- Deferred to fast-follow / V1.1: cosign signing + SBOM + SLSA, arm64 multi-arch,
  tier-3 ExternalSecrets (SEC-001 PR#42), image/chart registry PUSH (ghcr `<org>` unresolved).
- Migrations run via the existing EnsureSchema runner (NOT golang-migrate); `*.down.sql`
  excluded on forward apply (D2 fix in internal/index/pg).
