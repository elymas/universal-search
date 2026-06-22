#!/usr/bin/env bash
# check-bilingual-coverage.sh — verify KO coverage of Tier-1 EN pages
# SPEC-DOC-001 REQ-DOC-016 / T7
# Usage: ./docs/scripts/check-bilingual-coverage.sh [--threshold N]
# Returns exit 0 if KO coverage >= threshold (default 90%), exit 1 if below.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DOCS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
CONTENT_EN="${DOCS_DIR}/content/en"
CONTENT_KO="${DOCS_DIR}/content/ko"
THRESHOLD="${1:-90}"
if [[ "${1:-}" == "--threshold" ]]; then
    THRESHOLD="${2:-90}"
fi

# Tier-1 EN pages (scope of KO gate — excludes reference/cli + reference/api per D3)
TIER1_DIRS=(
    "introduction"
    "getting-started"
    "end-users"
    "operators"
    "troubleshooting"
)

total_en=0
total_ko=0
missing=()

for dir in "${TIER1_DIRS[@]}"; do
    en_dir="${CONTENT_EN}/${dir}"
    ko_dir="${CONTENT_KO}/${dir}"

    if [[ ! -d "${en_dir}" ]]; then
        continue
    fi

    # Count .mdx files in EN (exclude _meta.js)
    while IFS= read -r -d '' en_file; do
        rel="${en_file#"${en_dir}"/}"
        en_base="${rel%.mdx}"
        ko_file="${ko_dir}/${rel}"

        ((total_en++)) || true

        if [[ -f "${ko_file}" ]]; then
            ((total_ko++)) || true
        else
            missing+=("${dir}/${en_base}")
        fi
    done < <(find "${en_dir}" -name "*.mdx" -not -name "_meta*" -print0 2>/dev/null)
done

if [[ ${total_en} -eq 0 ]]; then
    echo "ERROR: No Tier-1 EN .mdx files found in ${CONTENT_EN}" >&2
    exit 1
fi

# Coverage percentage (integer arithmetic)
coverage=$(( total_ko * 100 / total_en ))

echo "=== Bilingual Coverage Report ==="
echo "EN Tier-1 pages : ${total_en}"
echo "KO mirror pages : ${total_ko}"
echo "Coverage        : ${coverage}%"
echo "Threshold       : ${THRESHOLD}%"

if [[ ${#missing[@]} -gt 0 ]]; then
    echo ""
    echo "Missing KO pages:"
    for m in "${missing[@]}"; do
        echo "  - ${m}"
    done
fi

if [[ ${coverage} -ge ${THRESHOLD} ]]; then
    echo ""
    echo "PASS: KO coverage ${coverage}% >= ${THRESHOLD}%"
    exit 0
else
    echo ""
    echo "FAIL: KO coverage ${coverage}% < ${THRESHOLD}%" >&2
    exit 1
fi
