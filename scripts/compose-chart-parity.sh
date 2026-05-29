#!/usr/bin/env bash
# compose-chart-parity.sh — verify env-var parity between docker-compose and Helm chart
# REQ-DEPLOY-024: fails if either side adds a variable not present in the other.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
CHART_DIR="${ROOT_DIR}/charts/universal-search"
COMPOSE_FILE="${ROOT_DIR}/deploy/docker-compose.yml"
ENV_EXAMPLE="${ROOT_DIR}/.env.example"
VALUES_FILE="${CHART_DIR}/values.yaml"

# Documented allowlist for compose-only or chart-only variables
COMPOSE_ONLY_ALLOWLIST=(
    "PROMETHEUS_PORT"
    "PROMETHEUS_SCRAPE_PORT"
)
CHART_ONLY_ALLOWLIST=(
    # OIDC vars not yet in .env.example (OQ3)
    "OIDC_ISSUER_URL"
    "OIDC_CLIENT_ID"
    "OIDC_CLIENT_SECRET"
    "OIDC_REDIRECT_URL"
    "OIDC_SCOPES"
    "JWT_SIGNING_KEY"
    "SESSION_SECRET"
    # Helm-specific
    "DEEP_TREE_PERSISTENCE_DIR"
)

echo "=== Compose-Chart Parity Check ==="

# Extract ${VAR} references from docker-compose.yml
COMPOSE_VARS=$(grep -oP '\$\{(\w+)' "${COMPOSE_FILE}" | sed 's/\${//' | sort -u)

# Extract KEY= from .env.example
ENV_VARS=$(grep -oP '^\w+' "${ENV_EXAMPLE}" | sort -u)

# Extract env-var names from values.yaml (keys under .env sections)
CHART_VARS=$(grep -E '^\s+[A-Z_]+:' "${VALUES_FILE}" | sed 's/^\s*//' | sed 's/:.*//' | sort -u)

# Union of compose + env vars
ALL_COMPOSE=$(echo -e "${COMPOSE_VARS}\n${ENV_VARS}" | sort -u | grep -v '^$')
ALL_CHART=$(echo "${CHART_VARS}" | sort -u)

# Check compose vars missing from chart
MISSING_IN_CHART=0
echo ""
echo "--- Vars in compose/env but NOT in chart values ---"
for var in ${ALL_COMPOSE}; do
    if ! echo "${ALL_CHART}" | grep -qx "${var}"; then
        if ! echo "${COMPOSE_ONLY_ALLOWLIST[@]}" | grep -qw "${var}"; then
            echo "  MISSING: ${var}"
            MISSING_IN_CHART=$((MISSING_IN_CHART + 1))
        else
            echo "  ALLOWED: ${var} (compose-only allowlist)"
        fi
    fi
done

# Check chart vars missing from compose
MISSING_IN_COMPOSE=0
echo ""
echo "--- Vars in chart values but NOT in compose/env ---"
for var in ${ALL_CHART}; do
    if ! echo "${ALL_COMPOSE}" | grep -qx "${var}"; then
        if ! echo "${CHART_ONLY_ALLOWLIST[@]}" | grep -qw "${var}"; then
            echo "  MISSING: ${var}"
            MISSING_IN_COMPOSE=$((MISSING_IN_COMPOSE + 1))
        else
            echo "  ALLOWED: ${var} (chart-only allowlist)"
        fi
    fi
done

echo ""
echo "=== Result ==="
echo "  Missing in chart:  ${MISSING_IN_CHART}"
echo "  Missing in compose: ${MISSING_IN_COMPOSE}"

if [ "${MISSING_IN_CHART}" -gt 0 ] || [ "${MISSING_IN_COMPOSE}" -gt 0 ]; then
    echo "FAIL: Parity check failed. Add missing vars or update allowlist."
    exit 1
fi

echo "PASS: All env-vars are in parity."
