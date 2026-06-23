#!/usr/bin/env bash
# scripts/check-doc-credentials.sh
# Credential-shape lint for adapter reference pages.
# Scans all docs/content/{en,ko}/reference/adapters/*.mdx for patterns that
# resemble real credentials — placeholders are allowed, real-shaped values fail.
#
# Exit 0: clean baseline (no matches).
# Exit 1: at least one potential credential-shaped value found.
#
# SPEC-DOC-002 REQ-ADPDOC-018 · D8

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
DOCS_DIR="${PROJECT_ROOT}/docs/content"

# ──────────────────────────────────────────────────────────────────────────────
# Pattern catalogue (intentionally aligned with .gitleaks.toml rule set per D8)
# Each pattern is tested against file content outside fenced code annotations.
# ──────────────────────────────────────────────────────────────────────────────

# GitHub PAT prefixes: ghp_, gho_, ghu_, ghr_, ghs_  followed by 36+ alnum chars
GITHUB_PAT_PATTERN='gh[pousr]_[A-Za-z0-9]{36,}'

# AWS access key: AKIA/ASIA followed by 16 uppercase alnum
AWS_KEY_PATTERN='(AKIA|ASIA)[A-Z0-9]{16}'

# Generic 40-char hex string (OAuth tokens, SHA1-derived secrets)
HEX40_PATTERN='[0-9a-fA-F]{40}'

# JWT: three base64url segments separated by dots (yyy.yyy.yyy pattern)
JWT_PATTERN='eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}'

# Naver client secret: 32-char alphanumeric (typical Naver API secret format).
# Uses a PCRE negative lookahead `(?!...)` so it requires a -P-capable grep
# (see GREP / GREP_PCRE_OK below). Wired into the scan per SPEC-SEC-004
# REQ-SEC4-030/040 after a one-time FP review confirmed zero unexcused matches
# against current docs/content/**/reference/adapters/*.mdx.
NAVER_SECRET_PATTERN='[A-Za-z0-9]{32}(?![A-Za-z0-9>_])'

# Placeholder patterns that are ALLOWED (grep -v to exclude from matches)
PLACEHOLDER_PATTERN='<[A-Z_]+>|\$\{[A-Z_]+\}|your-[a-z-]+-here|example-value-not-real|<YOUR_[A-Z_]+>|\*\*\*|PLACEHOLDER|REDACTED'

# ──────────────────────────────────────────────────────────────────────────────
# PCRE matcher selection (REQ-SEC4-040)
# The Naver pattern uses a PCRE negative lookahead, so check_pattern runs grep
# in PCRE mode (-P). `grep -P` is a GNU-grep extension: ubuntu CI (GNU grep) has
# it; macOS/BSD grep does not. Prefer `ggrep` (GNU coreutils) when present, else
# the system grep if it speaks -P. If no -P-capable grep exists, the Naver PCRE
# check is skipped with a warning (CI-only) while ERE patterns still run via -E.
# ──────────────────────────────────────────────────────────────────────────────
if command -v ggrep >/dev/null 2>&1 && echo "x" | ggrep -P "x" >/dev/null 2>&1; then
  GREP="ggrep"
  GREP_PCRE_OK=1
elif echo "x" | grep -P "x" >/dev/null 2>&1; then
  GREP="grep"
  GREP_PCRE_OK=1
else
  GREP="grep"
  GREP_PCRE_OK=0
fi

# ──────────────────────────────────────────────────────────────────────────────
# Scan
# ──────────────────────────────────────────────────────────────────────────────

FOUND=0
SCANNED=0

scan_file() {
  local file="$1"
  SCANNED=$((SCANNED + 1))
  local issues=0

  # check_pattern <label> <pattern> [flavor]
  # flavor=pcre forces a -P-capable matcher; if none is available the check is
  # skipped with a warning (REQ-SEC4-040 GNU/BSD guard). flavor=ere (default)
  # runs under -P when available (ERE patterns are valid PCRE) else falls back
  # to -E so basic patterns still run on macOS/BSD grep.
  check_pattern() {
    local label="$1"
    local pattern="$2"
    local flavor="${3:-ere}"
    local mflag nflag
    if [[ "${flavor}" == "pcre" ]]; then
      if [[ "${GREP_PCRE_OK}" -ne 1 ]]; then
        echo "check-doc-credentials: SKIP [${label}] — no -P-capable grep (CI-only check)" >&2
        return 0
      fi
      mflag="-P"; nflag="-nP"
    elif [[ "${GREP_PCRE_OK}" -eq 1 ]]; then
      mflag="-P"; nflag="-nP"
    else
      mflag="-E"; nflag="-nE"
    fi
    # Skip lines that are only placeholder patterns
    if "${GREP}" "${mflag}" "${pattern}" "${file}" 2>/dev/null | grep -vE "${PLACEHOLDER_PATTERN}" | grep -q .; then
      echo "FAIL [${label}] ${file}" >&2
      "${GREP}" "${nflag}" "${pattern}" "${file}" | grep -vE "${PLACEHOLDER_PATTERN}" | head -3 >&2
      issues=1
    fi
  }

  check_pattern "github-pat"    "${GITHUB_PAT_PATTERN}"
  check_pattern "aws-key"       "${AWS_KEY_PATTERN}"
  check_pattern "hex-40"        "${HEX40_PATTERN}"
  check_pattern "jwt"           "${JWT_PATTERN}"
  check_pattern "naver-secret"  "${NAVER_SECRET_PATTERN}" pcre

  if [[ $issues -gt 0 ]]; then
    FOUND=$((FOUND + 1))
  fi
}

# Find all adapter MDX pages
while IFS= read -r -d '' file; do
  scan_file "${file}"
done < <(find "${DOCS_DIR}" -path "*/reference/adapters/*.mdx" -print0 2>/dev/null)

echo "check-doc-credentials: scanned ${SCANNED} file(s)" >&2

if [[ $FOUND -gt 0 ]]; then
  echo "check-doc-credentials: FAIL — ${FOUND} file(s) with credential-shaped patterns" >&2
  exit 1
fi

echo "check-doc-credentials: OK — no credential-shaped patterns found" >&2
exit 0
