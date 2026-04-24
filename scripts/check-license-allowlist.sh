#!/usr/bin/env bash
# scripts/check-license-allowlist.sh — License compliance enforcement
# SPEC-DEP-001 REQ-DEP-004
#
# Scans docs/licenses/{go,python,web}.txt for known DISALLOWED license
# strings (blocklist approach). Exits 1 on first match. Missing files
# produce a warning and are skipped gracefully (exit 0).
#
# Blocklist approach rationale:
#   Allowlist-based line-by-line grep produces false positives on
#   tree-formatted output from license-checker (metadata lines like
#   "├─ repository:" don't contain license strings). Blocklist only
#   flags lines that explicitly name a forbidden license, passing
#   both tree-style and tabular outputs.
#
# Environment variable:
#   LICENSE_DIR — override the directory (default: docs/licenses)
#
# Pre-approved exceptions (service-boundary):
#   searxng/searxng — AGPL-3.0 consumed as external Docker service,
#     not linked. See NOTICE.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

LICENSE_DIR="${LICENSE_DIR:-${ROOT}/docs/licenses}"

# Disallowed SPDX identifiers — strong copyleft / unknown / proprietary.
# SearXNG AGPL-3.0 is the only exception (service-boundary, below).
DISALLOWED="GPL-[0-9]|AGPL-[0-9]|LGPL-[0-9]|SSPL|UNKNOWN|UNLICENSED|Proprietary|Commercial"

# Service-boundary pre-approved exceptions (matching lines are skipped)
EXCEPTIONS="searxng/searxng"

found_bad=0

for f in "${LICENSE_DIR}/go.txt" "${LICENSE_DIR}/python.txt" "${LICENSE_DIR}/web.txt"; do
  if [ ! -f "${f}" ]; then
    echo "WARNING: ${f} not found — skipping (run license-scan CI job to generate)" >&2
    continue
  fi

  while IFS= read -r line || [ -n "${line}" ]; do
    [ -z "${line}" ] && continue
    [[ "${line}" =~ ^#.*$ ]] && continue

    # Skip pre-approved service-boundary exceptions
    if echo "${line}" | grep -qE "${EXCEPTIONS}"; then
      continue
    fi

    # Flag only if the line explicitly names a disallowed license
    if echo "${line}" | grep -qE "${DISALLOWED}"; then
      echo "DISALLOWED LICENSE: ${line}" >&2
      found_bad=1
    fi
  done < "${f}"
done

if [ "${found_bad}" -eq 1 ]; then
  echo "" >&2
  echo "License check FAILED. See SPEC-DEP-001 §5.1 for the allowlist." >&2
  echo "To add a specific exception, document it in this script with justification." >&2
  exit 1
fi

echo "License check PASSED."
