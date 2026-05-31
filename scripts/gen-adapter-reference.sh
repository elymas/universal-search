#!/usr/bin/env bash
# scripts/gen-adapter-reference.sh
# Shell wrapper for tools/gen-adapter-ref — invoke the Go AST extraction tool.
# Usage:
#   ./scripts/gen-adapter-reference.sh           # emit 10 JSON files
#   ./scripts/gen-adapter-reference.sh --check   # diff-check vs committed JSON; exit 1 on drift
#
# SPEC-DOC-002 REQ-ADPDOC-007 · NFR-ADPDOC-001

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
TOOL_DIR="${PROJECT_ROOT}/tools/gen-adapter-ref"
TOOL_BIN="${PROJECT_ROOT}/_bin/gen-adapter-ref"

# Build the tool if binary is stale or missing.
if [[ ! -f "${TOOL_BIN}" ]] || \
   [[ "${TOOL_DIR}/main.go" -nt "${TOOL_BIN}" ]] || \
   [[ "${TOOL_DIR}/extract.go" -nt "${TOOL_BIN}" ]]; then
  mkdir -p "$(dirname "${TOOL_BIN}")"
  echo "Building gen-adapter-ref..." >&2
  go build -o "${TOOL_BIN}" "${TOOL_DIR}"
fi

# Pass all arguments through to the tool.
exec "${TOOL_BIN}" -root "${PROJECT_ROOT}" "$@"
