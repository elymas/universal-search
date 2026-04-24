#!/usr/bin/env bash
# scripts/check-license-allowlist.sh — License compliance enforcement
# SPEC-DEP-001 REQ-DEP-004
#
# Reads docs/licenses/{go,python,web}.txt and checks every line against the
# approved license allowlist. Exits 1 if any disallowed license is found.
# Missing license files produce a warning and are skipped gracefully (exit 0).
#
# Environment variable:
#   LICENSE_DIR — override the directory containing {go,python,web}.txt
#                 (default: docs/licenses relative to script directory)
#
# Pre-approved exceptions (service-boundary):
#   searxng/searxng — AGPL-3.0; consumed as an external Docker service, not linked.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Allow test fixtures to override the license directory via env var
LICENSE_DIR="${LICENSE_DIR:-${ROOT}/docs/licenses}"

# Approved SPDX license identifiers (regex alternation)
ALLOWLIST="MIT|Apache-2.0|BSD-2-Clause|BSD-3-Clause|ISC|PostgreSQL|MPL-2.0"

# Service-boundary pre-approved exceptions (these lines are always skipped)
# searxng/searxng — AGPL-3.0; external service, not linked (see NOTICE)
EXCEPTIONS="searxng/searxng"

found_bad=0

for f in "${LICENSE_DIR}/go.txt" "${LICENSE_DIR}/python.txt" "${LICENSE_DIR}/web.txt"; do
  if [ ! -f "${f}" ]; then
    echo "WARNING: ${f} not found — skipping (run license-scan CI job to generate)" >&2
    continue
  fi

  while IFS= read -r line || [ -n "${line}" ]; do
    # Skip empty lines and comment lines
    [ -z "${line}" ] && continue
    [[ "${line}" =~ ^#.*$ ]] && continue

    # Skip pre-approved service-boundary exceptions
    if echo "${line}" | grep -qE "${EXCEPTIONS}"; then
      continue
    fi

    # Check if the line contains an approved license
    if ! echo "${line}" | grep -qE "${ALLOWLIST}"; then
      echo "DISALLOWED LICENSE: ${line}" >&2
      found_bad=1
    fi
  done < "${f}"
done

if [ "${found_bad}" -eq 1 ]; then
  echo "" >&2
  echo "License check FAILED. See SPEC-DEP-001 §5.1 for the allowlist." >&2
  echo "To add an exception, document it in this script with justification." >&2
  exit 1
fi

echo "License check PASSED."
