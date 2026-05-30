# universal-search Helm chart

Team-scale Kubernetes deployment of the universal-search stack (SPEC-DEPLOY-001).

Templates the 10-service dev-compose topology (Qdrant, Meilisearch, PostgreSQL,
Redis, SearXNG, LiteLLM, Prometheus-scraping, plus the researcher / embedder /
tokenizer-ko Python sidecars) and newly containerizes the two host Go binaries
(`usearch-api`, `usearch-mcp`).

## Requirements

- Kubernetes >= 1.27 (CI validates 1.28..1.31)
- Helm v3 (>= 3.16)
- For Ingress: `ingress-nginx` + `cert-manager` pre-installed (cluster-wide singletons; this chart does NOT bundle them)
- For ServiceMonitor: Prometheus Operator (optional — falls back to pod annotations)
- For NetworkPolicy enforcement: a CNI policy controller (Calico/Cilium); otherwise policies are a no-op

## Install

```bash
helm dependency build charts/universal-search

# Dev (full stack, tier-1 secrets):
helm install usearch charts/universal-search \
  -f charts/universal-search/values-dev.yaml

# Production (layer your own cluster overlay last):
helm install usearch charts/universal-search \
  -f charts/universal-search/values.yaml \
  -f charts/universal-search/values-prod.yaml \
  -f my-cluster-overlay.yaml
```

## Image registry

V1 ships **built-but-unsigned linux/amd64** images. The `ghcr.io/<org>` registry
placeholder is unresolved, so the chart references images by short name; set
`global.imageRegistry` (or per-image `image.registry`) to your registry after
building/pushing. arm64 multi-arch and cosign signing/SBOM/SLSA are deferred to
fast-follow (signing owned by SPEC-REL-001).

The `embedder` image is **amd64-only** (PyTorch + CUDA); its Deployment carries
a `nodeAffinity` keeping it off non-amd64 nodes.

## Secret strategy (2-tier; tier-3 deferred to V1.1)

| backend | use | how |
|---|---|---|
| `values` | dev / CI ONLY | chart authors a Secret from `secrets.values.*` (plaintext) |
| `existingSecret` | **production default** | reference an operator-created Secret by name; keys must match `secrets.values` |
| `externalSecrets` | RESERVED (V1.1) | install-blocked in V1 — depends on SPEC-SEC-001 (PR#42) |

Create the production Secret with the 9 keys: `MEILI_MASTER_KEY`,
`POSTGRES_PASSWORD`, `SEARXNG_SECRET`, `LITELLM_MASTER_KEY`, `OPENAI_API_KEY`,
`ANTHROPIC_API_KEY`, `OIDC_CLIENT_SECRET`, `JWT_SIGNING_KEY`, `SESSION_SECRET`.

Rotation (tier 2) is operator-driven: `kubectl apply` the updated Secret, then
`kubectl rollout restart deployment` for affected components.

## Migrations

Schema migrations run as a **pre-install / pre-upgrade** Helm hook Job
(`-migrate`, weight -5) BEFORE app pods start. It runs the existing
`usearch migrate` (EnsureSchema) runner — NOT golang-migrate — over
`deploy/postgres/migrations/*.sql` in lexicographic order, idempotently.
Down-migrations (`*.down.sql`) are never applied on forward apply.

`helm rollback` reverses chart MANIFESTS only — it does NOT roll back the
database schema. Use forward-fix migrations; treat `*.down.sql` as a manual,
data-loss-risky operation reviewed by hand.

## License notice — SearXNG (AGPL-3.0)

SearXNG is licensed **AGPL-3.0** and is deployed as a separate container
(service boundary, not linked into the application). If you expose or modify it,
review your AGPL obligations. Disable with `searxng.enabled=false`.

## Docker Hub rate limits (NFR-DEPLOY-006)

Bitnami subchart images pull from Docker Hub (anonymous limit 100 pulls / 6h).
Configure `global.imagePullSecrets` with authenticated credentials or mirror the
images to an internal registry for production.

## Subcharts (D3)

`postgresql` (Bitnami), `redis` (Bitnami), `qdrant` (official) are bundled,
default-enabled, and pinned to exact patch versions. Opt out per service with
`<name>.enabled=false` + `<name>.external.*` to point at a managed service.
Bumps are recorded in CHANGELOG.md (quarterly audit, NFR-DEPLOY-005).

## See also

Operator walkthrough: `operators/deployment-helm` (SPEC-DOC-001).
