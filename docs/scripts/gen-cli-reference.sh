#!/usr/bin/env bash
# gen-cli-reference.sh — extract usearch --help output into MDX reference pages
# SPEC-DOC-001 REQ-DOC-007 / T5
# Usage: ./docs/scripts/gen-cli-reference.sh [--binary path/to/usearch] [--dry-run]
# Generates: docs/content/en/reference/cli/{query,config,history,deep,sources,login,repl}.mdx
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DOCS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${DOCS_DIR}/.." && pwd)"

BINARY="${REPO_ROOT}/cmd/usearch/usearch"
DRY_RUN=false
LAST_GENERATED="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --binary)
            BINARY="${2}"
            shift 2
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        *)
            shift
            ;;
    esac
done

CLI_REF_DIR="${DOCS_DIR}/content/en/reference/cli"

if [[ ! -x "${BINARY}" ]]; then
    echo "ERROR: Binary not found or not executable: ${BINARY}" >&2
    echo "       Build with 'make build' first." >&2
    exit 1
fi

# All 7 CLI subcommands (SPEC-DOC-001 B3)
SUBCOMMANDS=(query config history deep sources login repl)

generate_mdx() {
    local subcmd="$1"
    local help_output
    help_output=$("${BINARY}" "${subcmd}" --help 2>&1 || true)

    local output_file="${CLI_REF_DIR}/${subcmd}.mdx"

    local content
    content="---
title: usearch ${subcmd}
description: Auto-generated reference for usearch ${subcmd}
generated: true
source: usearch ${subcmd} --help
lastGenerated: ${LAST_GENERATED}
---

# usearch ${subcmd}

> This page is auto-generated from \`usearch ${subcmd} --help\`.
> Run \`make gen-docs\` after CLI changes to keep it in sync.

\`\`\`
${help_output}
\`\`\`

---

*Generated: ${LAST_GENERATED}*
*Source: \`usearch ${subcmd} --help\`*"

    if [[ "${DRY_RUN}" == "true" ]]; then
        echo "=== DRY RUN: ${output_file} ==="
        echo "${content}"
        echo ""
    else
        echo "${content}" > "${output_file}"
        echo "Generated: ${output_file}"
    fi
}

echo "=== CLI Reference Generator ==="
echo "Binary: ${BINARY}"
echo "Output: ${CLI_REF_DIR}/"
echo ""

for cmd in "${SUBCOMMANDS[@]}"; do
    generate_mdx "${cmd}"
done

echo ""
echo "Done. Generated ${#SUBCOMMANDS[@]} CLI reference pages."
if [[ "${DRY_RUN}" == "true" ]]; then
    echo "(DRY RUN — no files written)"
fi
