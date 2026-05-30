#!/usr/bin/env bash
# check-screenshot-freshness.sh — verify UI screenshots are <= MAX_AGE_DAYS old
# SPEC-DOC-001 REQ-DOC-013 / T7
# Usage: ./docs/scripts/check-screenshot-freshness.sh [--max-age-days N]
# Returns exit 0 if all screenshots are fresh, exit 1 if any are stale.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DOCS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PUBLIC_DIR="${DOCS_DIR}/public"
MAX_AGE_DAYS=90

# Parse --max-age-days flag
while [[ $# -gt 0 ]]; do
    case "$1" in
        --max-age-days)
            MAX_AGE_DAYS="${2:-90}"
            shift 2
            ;;
        *)
            shift
            ;;
    esac
done

NOW=$(date +%s)
MAX_AGE_SECONDS=$(( MAX_AGE_DAYS * 86400 ))

stale=()
total=0

# Find screenshots tagged for UI (screenshot:ui:*)
# Convention: images stored under public/images/screenshots/
SCREENSHOT_DIR="${PUBLIC_DIR}/images/screenshots"

if [[ ! -d "${SCREENSHOT_DIR}" ]]; then
    echo "INFO: Screenshot directory ${SCREENSHOT_DIR} does not exist — skipping freshness check."
    echo "      Create screenshots under docs/public/images/screenshots/ to enable this gate."
    exit 0
fi

while IFS= read -r -d '' img; do
    ((total++)) || true
    # Get file modification time
    if [[ "$(uname)" == "Darwin" ]]; then
        mtime=$(stat -f %m "${img}")
    else
        mtime=$(stat -c %Y "${img}")
    fi
    age=$(( NOW - mtime ))
    age_days=$(( age / 86400 ))

    if [[ ${age} -gt ${MAX_AGE_SECONDS} ]]; then
        stale+=("${img} (${age_days} days old)")
    fi
done < <(find "${SCREENSHOT_DIR}" \( -name "*.png" -o -name "*.jpg" -o -name "*.webp" \) -print0 2>/dev/null)

echo "=== Screenshot Freshness Report ==="
echo "Screenshots checked : ${total}"
echo "Max age             : ${MAX_AGE_DAYS} days"

if [[ ${total} -eq 0 ]]; then
    echo "INFO: No screenshots found — gate passes vacuously."
    echo "PASS"
    exit 0
fi

if [[ ${#stale[@]} -gt 0 ]]; then
    echo ""
    echo "STALE screenshots (older than ${MAX_AGE_DAYS} days):"
    for s in "${stale[@]}"; do
        echo "  - ${s}"
    done
    echo ""
    echo "FAIL: ${#stale[@]} stale screenshot(s) found." >&2
    exit 1
else
    echo ""
    echo "PASS: All ${total} screenshot(s) are within ${MAX_AGE_DAYS} days."
    exit 0
fi
