#!/usr/bin/env bash
# Verifies that every ${VAR} referenced in deploy/docker-compose.yml
# has a matching entry in the root .env.example file.
# Exit 0 if all present, exit 1 with missing var list.
set -euo pipefail

COMPOSE_FILE="$(dirname "$0")/../deploy/docker-compose.yml"
ENV_EXAMPLE="$(dirname "$0")/../.env.example"

if [[ ! -f "$COMPOSE_FILE" ]]; then
  echo "ERROR: $COMPOSE_FILE not found" >&2
  exit 1
fi

if [[ ! -f "$ENV_EXAMPLE" ]]; then
  echo "ERROR: $ENV_EXAMPLE not found" >&2
  exit 1
fi

# Extract variable names from ${VAR} and ${VAR:-default} patterns in compose file.
# Capture only the name part (before :- or }) — excludes numeric defaults.
VARS=$(grep -oE '\$\{[A-Za-z_][A-Za-z0-9_]*[^}]*\}' "$COMPOSE_FILE" \
  | sed -E 's/^\$\{([A-Za-z_][A-Za-z0-9_]*).*/\1/' \
  | grep -E '^[A-Z_][A-Z0-9_]*$' \
  | sort -u)

MISSING=()
for VAR in $VARS; do
  if ! grep -qE "^${VAR}=" "$ENV_EXAMPLE"; then
    MISSING+=("$VAR")
  fi
done

if [[ ${#MISSING[@]} -gt 0 ]]; then
  echo "ERROR: The following variables are in docker-compose.yml but missing from .env.example:" >&2
  for V in "${MISSING[@]}"; do
    echo "  - $V" >&2
  done
  exit 1
fi

echo "OK: all compose variables are documented in .env.example"
