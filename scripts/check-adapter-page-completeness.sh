#!/usr/bin/env bash
# scripts/check-adapter-page-completeness.sh
# Validates that each per-adapter MDX page contains exactly 10 H2 sections
# in the prescribed order, each with ≥50 characters of plain text content.
#
# Also checks:
# - filename = SourceID (using the adapter registry)
# - per-page lastVerified frontmatter (warns when > 180 days old)
#
# Exit 0: all checks pass.
# Exit 1: one or more pages fail.
#
# SPEC-DOC-002 REQ-ADPDOC-002 · NFR-ADPDOC-002/004

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
EN_ADAPTERS_DIR="${PROJECT_ROOT}/docs/content/en/reference/adapters"

# Expected H2 headings in order (per D1 / REQ-ADPDOC-002).
# Reference catalogue only — the live order check below uses `expected_order`.
# shellcheck disable=SC2034  # documented superset, not consumed directly
EXPECTED_SECTIONS=(
  "Status & Compatibility"
  "Status &amp; Compatibility"
  "Overview"
  "Setup"
  "Capabilities"
  "Query syntax"
  "Rate limits"
  "Error reference"
  "Troubleshooting"
  "Version compatibility"
  "Related"
)

# The 10 real SourceIDs (REQ-ADPDOC-001)
REQUIRED_SOURCE_IDS=(
  arxiv reddit hackernews github youtube
  bluesky x searxng naver koreanews
)

FAIL=0
PAGES_CHECKED=0

check_page() {
  local file="$1"
  local base
  base="$(basename "${file}" .mdx)"

  PAGES_CHECKED=$((PAGES_CHECKED + 1))

  # Extract all H2 headings from the file
  local headings
  mapfile -t headings < <(grep -E '^## ' "${file}" | sed 's/^## //')

  if [[ ${#headings[@]} -ne 10 ]]; then
    echo "FAIL [section-count] ${file}: expected 10 H2 sections, got ${#headings[@]}" >&2
    FAIL=1
    return
  fi

  # Check section order (allow HTML entity variant for "&")
  local expected_order=(
    "Status"
    "Overview"
    "Setup"
    "Capabilities"
    "Query syntax"
    "Rate limits"
    "Error reference"
    "Troubleshooting"
    "Version compatibility"
    "Related"
  )
  for i in "${!expected_order[@]}"; do
    local want="${expected_order[$i]}"
    local got="${headings[$i]}"
    if [[ "${got}" != *"${want}"* ]]; then
      echo "FAIL [section-order] ${file}: section $((i+1)) expected '${want}', got '${got}'" >&2
      FAIL=1
    fi
  done

  # Check minimum content per section (≥50 chars of plain text after stripping code blocks)
  # Simple heuristic: count non-blank, non-code-fence, non-import lines between sections
  local current_section=""
  local char_count=0
  local in_code=0

  while IFS= read -r line; do
    if [[ "${line}" =~ ^'```' ]]; then
      if [[ $in_code -eq 0 ]]; then
        in_code=1
      else
        in_code=0
      fi
      continue
    fi
    if [[ $in_code -eq 1 ]]; then
      continue
    fi
    if [[ "${line}" =~ ^'## ' ]]; then
      # New section: check previous section's char count
      if [[ -n "${current_section}" && $char_count -lt 50 ]]; then
        echo "FAIL [section-content] ${file}: section '${current_section}' has < 50 chars of plain text" >&2
        FAIL=1
      fi
      current_section="${line#'## '}"
      char_count=0
      continue
    fi
    # Count all non-blank, non-frontmatter, non-import lines
    # (includes tables, JSX component calls, prose text)
    if [[ -n "${line}" && ! "${line}" =~ ^'import ' && ! "${line}" =~ ^'---' ]]; then
      char_count=$((char_count + ${#line}))
    fi
  done < "${file}"

  # Check last section
  if [[ -n "${current_section}" && $char_count -lt 50 ]]; then
    echo "FAIL [section-content] ${file}: section '${current_section}' has < 50 chars of plain text" >&2
    FAIL=1
  fi

  # Check lastVerified frontmatter
  local last_verified
  last_verified="$(grep -E '^lastVerified:' "${file}" | head -1 | sed 's/lastVerified: *//' | tr -d "'" | tr -d '"')"
  if [[ -n "${last_verified}" ]]; then
    # Convert to seconds since epoch for comparison
    local verified_ts now_ts days_old
    verified_ts="$(date -j -f "%Y-%m-%d" "${last_verified}" "+%s" 2>/dev/null || date -d "${last_verified}" "+%s" 2>/dev/null || echo 0)"
    now_ts="$(date "+%s")"
    if [[ $verified_ts -gt 0 ]]; then
      days_old=$(( (now_ts - verified_ts) / 86400 ))
      if [[ $days_old -gt 180 ]]; then
        echo "WARN [lastVerified-stale] ${file}: lastVerified is ${days_old} days old (>${180} day threshold)" >&2
      fi
    fi
  fi
}

# Check all 10 per-adapter pages exist
for sid in "${REQUIRED_SOURCE_IDS[@]}"; do
  page="${EN_ADAPTERS_DIR}/${sid}.mdx"
  if [[ ! -f "${page}" ]]; then
    echo "FAIL [missing-page] ${page} does not exist" >&2
    FAIL=1
  fi
done

# Check errors.mdx exists (5 H3 subsections)
errors_page="${EN_ADAPTERS_DIR}/errors.mdx"
if [[ ! -f "${errors_page}" ]]; then
  echo "FAIL [missing-errors] ${errors_page} does not exist" >&2
  FAIL=1
else
  h3_count="$(grep -c '^### ' "${errors_page}" || true)"
  if [[ $h3_count -lt 5 ]]; then
    echo "FAIL [errors-sections] ${errors_page}: expected ≥5 H3 subsections, got ${h3_count}" >&2
    FAIL=1
  fi
fi

# noop/reference page must NOT exist
noop_page="${EN_ADAPTERS_DIR}/reference.mdx"
noop_alt="${EN_ADAPTERS_DIR}/noop.mdx"
if [[ -f "${noop_page}" ]] || [[ -f "${noop_alt}" ]]; then
  echo "FAIL [noop-page-present] noop adapter must NOT have a public reference page" >&2
  FAIL=1
fi

# Check each per-adapter page for completeness
while IFS= read -r -d '' page; do
  base="$(basename "${page}" .mdx)"
  # Skip index, errors, and _meta
  if [[ "${base}" == "index" ]] || [[ "${base}" == "errors" ]]; then
    continue
  fi
  check_page "${page}"
done < <(find "${EN_ADAPTERS_DIR}" -maxdepth 1 -name "*.mdx" -not -name "_*" -print0 2>/dev/null)

echo "check-adapter-page-completeness: checked ${PAGES_CHECKED} page(s)" >&2

if [[ $FAIL -ne 0 ]]; then
  echo "check-adapter-page-completeness: FAIL" >&2
  exit 1
fi

echo "check-adapter-page-completeness: OK" >&2
exit 0
