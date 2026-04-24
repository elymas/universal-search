#!/usr/bin/env bash
# Verifies that `make help` lists the required 10 targets.
# Exit 0 if exactly 10 matches found, exit 1 otherwise.
set -euo pipefail

MAKEFILE_DIR="$(dirname "$0")/.."

COUNT=$(make -C "$MAKEFILE_DIR" help 2>/dev/null | grep -cE "^  (dev|test|lint|build|clean|compose-up|compose-down|fmt|tidy|install-py) " || true)

if [[ "$COUNT" -ne 10 ]]; then
  echo "ERROR: expected 10 help targets, found $COUNT" >&2
  echo "Run 'make help' to see current targets" >&2
  exit 1
fi

echo "OK: Makefile exposes all 10 required targets"
