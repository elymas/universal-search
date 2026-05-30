#!/usr/bin/env bash
# SPEC-DEPLOY-001 REQ-DEPLOY-024 — compose ↔ chart env-var parity check.
#
# Compares the set of environment variables that the application consumes on the
# DEV side (deploy/docker-compose.yml ${VAR} interpolation + .env.example keys)
# against the set the CHART surfaces (charts/universal-search/values.yaml
# secrets.values + config keys). The build fails if either side carries an
# application env-var the other lacks, modulo a documented allowlist of genuinely
# compose-only or chart-only knobs (ports, infra wiring, sidecar-internal vars).
#
# Exit 0 = parity holds. Exit 1 = unexplained delta.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE="${REPO_ROOT}/deploy/docker-compose.yml"
ENV_EXAMPLE="${REPO_ROOT}/.env.example"
VALUES="${REPO_ROOT}/charts/universal-search/values.yaml"

# ── Allowlist ────────────────────────────────────────────────────────────────
# Vars that legitimately exist on only ONE side. Reasons in comments.
#  - *_PORT / *_HOST / *_URL / *_BASE_URL: compose host-networking knobs; the
#    chart derives intra-cluster service DNS instead (configmap.yaml).
#  - DATABASE_URL / REDIS_URL: compose DSNs; chart assembles from host+port+secret.
#  - sidecar-internal tuning vars consumed only inside the Python sidecars
#    (passed via per-sidecar `env:` maps in values, not the shared ConfigMap).
COMPOSE_ONLY_ALLOW='(_PORT|_HOST|_URL|_BASE_URL|_TIMEOUT|_RETRIES|_REQUEST_TIMEOUT|_SAMPLE_RATIO|_SCRAPE_PORT|DEEP_|TOKENIZER_KO_|RESEARCHER_|EMBEDDER_|MEILI_ENV|OLLAMA_BASE_URL|LOKI_ENDPOINT|OTLP_ENDPOINT|LOG_LEVEL|USEARCH_ADMIN_PORT|PROMETHEUS_|LITELLM_BUDGET_USD|POSTGRES_USER|POSTGRES_DB|SEARXNG_BASE_URL|OIDC_ISSUER_URL|OIDC_CLIENT_ID|OIDC_REDIRECT_URL)'
# Vars that legitimately exist only on the CHART side (k8s-specific).
CHART_ONLY_ALLOW='(^$)'

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# ── compose side: ${VAR} interpolation + .env.example keys ──────────────────
{
  grep -oE '\$\{[A-Z0-9_]+' "$COMPOSE" | sed 's/^\${//'
  grep -oE '^[A-Z0-9_]+=' "$ENV_EXAMPLE" | sed 's/=$//'
} | sort -u | grep -vxE 'VAR' > "$tmp/compose.txt"  # VAR = doc-comment placeholder

# ── chart side: secrets.values keys (4-space indent) + config keys (2-space) ─
# Both blocks hold UPPER_SNAKE env-var keys; sidecar `env:` maps are at 4-space
# indent under sidecar blocks and are intentionally excluded (sidecar-internal).
{
  # secrets.values: keys are 4-space-indented UPPER_SNAKE between `  values:` and
  # the next 2-space key (`  existingSecret:`).
  sed -n '/^  values:/,/^  existingSecret:/p' "$VALUES" \
    | grep -oE '^    [A-Z0-9_]+:' | sed -E 's/^ +//; s/:$//'
  # config: keys are 2-space-indented UPPER_SNAKE between `config:` and the next
  # top-level key.
  sed -n '/^config:/,/^[a-z]/p' "$VALUES" \
    | grep -oE '^  [A-Z0-9_]+:' | sed -E 's/^ +//; s/:$//'
} | sort -u > "$tmp/chart.txt"

# Application SECRET vars that MUST appear on both sides (the parity core).
# These are the keys the chart templates as secretKeyRef and that the app reads.
CORE_SECRETS="MEILI_MASTER_KEY POSTGRES_PASSWORD SEARXNG_SECRET LITELLM_MASTER_KEY OPENAI_API_KEY ANTHROPIC_API_KEY OIDC_CLIENT_SECRET JWT_SIGNING_KEY SESSION_SECRET"

fail=0

echo "== Core secret parity (must be on BOTH compose/.env.example and chart) =="
for k in $CORE_SECRETS; do
  on_compose=$(grep -qx "$k" "$tmp/compose.txt" && echo yes || echo no)
  on_chart=$(grep -qx "$k" "$tmp/chart.txt" && echo yes || echo no)
  if [ "$on_compose" != "yes" ] || [ "$on_chart" != "yes" ]; then
    echo "  MISMATCH $k: compose=$on_compose chart=$on_chart"
    fail=1
  else
    echo "  ok       $k"
  fi
done

echo "== compose-only vars not on chart (allowlist-filtered) =="
comm -23 "$tmp/compose.txt" "$tmp/chart.txt" | while read -r v; do
  if echo "$v" | grep -qE "$COMPOSE_ONLY_ALLOW"; then
    :  # allowed (infra/sidecar/derived knob)
  else
    echo "  UNEXPLAINED compose-only var: $v"
    echo "unexplained" >> "$tmp/fail"
  fi
done

echo "== chart-only vars not on compose (allowlist-filtered) =="
comm -13 "$tmp/compose.txt" "$tmp/chart.txt" | while read -r v; do
  if echo "$v" | grep -qE "$CHART_ONLY_ALLOW"; then
    :
  else
    echo "  UNEXPLAINED chart-only var: $v"
    echo "unexplained" >> "$tmp/fail"
  fi
done

if [ -f "$tmp/fail" ] || [ "$fail" -ne 0 ]; then
  echo "PARITY FAIL"
  exit 1
fi
echo "PARITY OK"
