#!/usr/bin/env sh
# check-env-example.sh — REQ-BOOT-006
# Scans deploy/docker-compose.yml for ${VAR} references and verifies each
# has a matching entry in .env.example.
# Exit 0 on success, 1 on missing entries.

set -eu

COMPOSE_FILE="deploy/docker-compose.yml"
ENV_EXAMPLE=".env.example"
EXIT_CODE=0

# Extract ${VAR} references from compose file, strip ${VAR:-default} defaults.
vars=$(grep -oE '\$\{[A-Z_]+[^}]*\}' "$COMPOSE_FILE" | \
    sed 's/\${//;s/}.*//' | \
    sed 's/[:-].*//' | \
    sort -u)

for var in $vars; do
    if ! grep -qE "^${var}=" "$ENV_EXAMPLE"; then
        echo "MISSING in .env.example: $var (referenced in $COMPOSE_FILE)" >&2
        EXIT_CODE=1
    fi
done

if [ "$EXIT_CODE" -eq 0 ]; then
    echo "All compose \${VAR} references are documented in $ENV_EXAMPLE."
fi

exit "$EXIT_CODE"
