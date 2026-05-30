#!/usr/bin/env bash
# check-doc-claims.sh — advisory check for doc claims against CLI --help output
# SPEC-DOC-001 REQ-DOC-018 / T7 (P2 advisory — non-blocking)
# Usage: ./docs/scripts/check-doc-claims.sh [--binary path/to/usearch]
# Returns exit 0 always (advisory mode); logs warnings if claims drift.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DOCS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${DOCS_DIR}/.." && pwd)"

# Binary path — default to build output
BINARY="${REPO_ROOT}/cmd/usearch/usearch"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --binary)
            BINARY="${2:-${BINARY}}"
            shift 2
            ;;
        *)
            shift
            ;;
    esac
done

echo "=== Doc Claims Advisory Check ==="
echo "Binary: ${BINARY}"

if [[ ! -x "${BINARY}" ]]; then
    echo "WARN: Binary not found or not executable at ${BINARY}"
    echo "      Skipping doc claims check (build the binary first with 'make build')"
    echo "ADVISORY: SKIPPED (binary not available)"
    exit 0
fi

WARNINGS=0
CHECKS=0

# Check that all 7 subcommands appear in --help
EXPECTED_CMDS=(query config history deep sources login repl)
HELP_OUTPUT=$("${BINARY}" --help 2>&1 || true)

for cmd in "${EXPECTED_CMDS[@]}"; do
    ((CHECKS++)) || true
    if ! echo "${HELP_OUTPUT}" | grep -q "^  ${cmd}\|^    ${cmd}\|  ${cmd} "; then
        echo "WARN: Expected subcommand '${cmd}' not found in --help output"
        ((WARNINGS++)) || true
    fi
done

# Check version output format
VERSION_OUTPUT=$("${BINARY}" --version 2>&1 || true)
((CHECKS++)) || true
if ! echo "${VERSION_OUTPUT}" | grep -qE "^usearch v[0-9]"; then
    echo "WARN: Version output '${VERSION_OUTPUT}' does not match expected 'usearch v<semver>' format"
    ((WARNINGS++)) || true
fi

echo ""
echo "Checks run : ${CHECKS}"
echo "Warnings   : ${WARNINGS}"

if [[ ${WARNINGS} -gt 0 ]]; then
    echo ""
    echo "ADVISORY: ${WARNINGS} claim(s) may have drifted. Review documentation."
    echo "          (This is a non-blocking advisory check — exit 0)"
else
    echo ""
    echo "ADVISORY: All ${CHECKS} doc claims appear consistent with binary."
fi

# Always exit 0 — advisory mode only
exit 0
