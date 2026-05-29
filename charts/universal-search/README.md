# Universal Search Helm Chart

Helm chart for deploying the Universal Search Engine to Kubernetes.

## Prerequisites

- Kubernetes 1.28+ (tested up to 1.31)
- Helm 3.8+ (OCI registry support)
- cert-manager (optional, for TLS ingress)
- ingress-nginx (optional, for HTTP ingress)
- External Secrets Operator (optional, for tier 3 secrets)
- kube-prometheus-stack (optional, for ServiceMonitor)
- NetworkPolicy-capable CNI: Calico, Cilium, etc. (optional, enforced when enabled)

## Quick Start

```bash
# Add chart dependencies
helm dependency update charts/universal-search

# Install with defaults (dev)
helm install usearch charts/universal-search

# Install with production overlay
helm install usearch charts/universal-search -f charts/universal-search/ci/values-prod.yaml

# Install with GPU support for embedder
helm install usearch charts/universal-search \
  -f charts/universal-search/values.yaml \
  -f charts/universal-search/ci/values-gpu.yaml
```

## Configuration

### Secret Backend (3-tier strategy)

| Tier | Backend | Use Case | Production? |
|------|---------|----------|-------------|
| 1 | `values` | Dev/CI only — secrets in values.yaml | NO |
| 2 | `existingSecret` | Small team — operator pre-creates K8s Secrets | YES (recommended) |
| 3 | `externalSecrets` | Enterprise — ESO syncs from Vault/cloud SM | YES |

```bash
# Tier 2: Pre-create secrets
kubectl create secret generic usearch-secrets-api \
  --from-literal=POSTGRES_PASSWORD=your-password \
  --from-literal=MEILI_MASTER_KEY=your-key

helm install usearch charts/universal-search \
  --set secrets.backend=existingSecret
```

### External Infrastructure

```bash
# Use external PostgreSQL (RDS, Cloud SQL, etc.)
helm install usearch charts/universal-search \
  --set postgresql.enabled=false \
  --set postgresql.external.host=your-rds-endpoint \
  --set postgresql.external.port=5432 \
  --set postgresql.external.database=usearch
```

### Image Verification

```bash
# Verify cosign signature
cosign verify ghcr.io/elymas/universal-search/usearch-api:0.1.0 \
  --certificate-identity-regexp 'https://github.com/elymas/universal-search/' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com'
```

## SearXNG License Notice

This chart optionally deploys SearXNG, which is licensed under AGPL-3.0.
SearXNG is consumed as an external service (service-boundary, not linked).
See NOTICE and docs/dependencies.md for license compliance details.

## Docker Hub Rate Limit

Bitnami subchart images (PostgreSQL, Redis) are pulled from Docker Hub.
Anonymous pulls are limited to 100 per 6 hours. For production, configure
`global.imagePullSecrets` or use an internal registry mirror.

```bash
helm install usearch charts/universal-search \
  --set global.imagePullSecrets[0].name=my-registry-secret
```

## Documentation

Full operator documentation: https://elymas.github.io/universal-search/operators/deployment-helm

## Values

See values.yaml for the complete configuration reference (~300 keys).

| Key | Default | Description |
|-----|---------|-------------|
| `secrets.backend` | `values` | Secret backend tier: values, existingSecret, externalSecrets |
| `usearch.api.enabled` | `true` | Enable the API server |
| `usearch.api.replicas` | `1` | Number of API replicas |
| `usearch.api.hpa.enabled` | `true` | Enable horizontal pod autoscaler |
| `usearch.api.ingress.enabled` | `false` | Enable HTTP ingress |
| `postgresql.enabled` | `true` | Deploy Bitnami PostgreSQL subchart |
| `redis.enabled` | `true` | Deploy Bitnami Redis subchart |
| `qdrant.enabled` | `true` | Deploy Qdrant subchart |
| `embedder.gpu.enabled` | `false` | Enable GPU support for embedder |
| `observability.serviceMonitor.enabled` | `true` | Deploy ServiceMonitor CRDs |
