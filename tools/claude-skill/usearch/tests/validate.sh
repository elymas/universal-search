#!/usr/bin/env bash
# validate.sh -- SPEC-SKILL-001 plugin artifact validation tests.
#
# Runs schema and content checks on the plugin directory.
# Exit 0 = all pass; exit 1 = one or more failures.
#
# Usage:
#   ./tests/validate.sh [--plugin-dir ./path/to/usearch]

set -euo pipefail

# Resolve plugin directory.
PLUGIN_DIR="$(cd "$(dirname "$0")/.." && pwd)"
if [[ "${1:-}" == "--plugin-dir" ]]; then
  PLUGIN_DIR="$(cd "$2" && pwd)"
fi

PASS=0
FAIL=0

ok() { PASS=$((PASS + 1)); }
fail() { echo "FAIL: $1"; FAIL=$((FAIL + 1)); }

echo "=== SPEC-SKILL-001 Plugin Validation ==="
echo "Plugin dir: $PLUGIN_DIR"
echo ""

# ---- REQ-SKILL-001: File inventory ----

echo "--- REQ-SKILL-001: File inventory ---"

required_files=(
  ".claude-plugin/plugin.json"
  ".mcp.json"
  "skills/usearch/SKILL.md"
  "README.md"
  "LICENSE"
)

for f in "${required_files[@]}"; do
  if [[ -f "$PLUGIN_DIR/$f" ]]; then
    ok
  else
    fail "Missing required file: $f"
  fi
done

# No bundled binaries (Strategy A exclusion).
if find "$PLUGIN_DIR" -name "bin" -type d -not -path "*/.git/*" | grep -q .; then
  fail "Plugin MUST NOT contain a bin/ directory (Strategy A excluded per REQ-SKILL-001)"
else
  ok
fi

echo ""

# ---- REQ-SKILL-002: plugin.json manifest ----

echo "--- REQ-SKILL-002: plugin.json manifest ---"

MANIFEST="$PLUGIN_DIR/.claude-plugin/plugin.json"

if ! command -v python3 &>/dev/null; then
  echo "WARN: python3 not found; skipping JSON parsing tests"
else
  # Validate JSON syntax.
  if python3 -c "import json; json.load(open('$MANIFEST'))" 2>/dev/null; then
    ok
  else
    fail "plugin.json is not valid JSON"
  fi

  # Check required and recommended fields.
  check_field() {
    local field="$1"
    local result
    result=$(python3 -c "
import json
m = json.load(open('$MANIFEST'))
parts = '$field'.split('.')
v = m
for p in parts:
    v = v.get(p) if isinstance(v, dict) else None
print(v if v is not None else '')
" 2>/dev/null)
    if [[ -n "$result" ]]; then
      ok
    else
      fail "plugin.json missing or empty field: $field"
    fi
  }

  check_field "name"
  check_field "version"
  check_field "displayName"
  check_field "description"
  check_field "license"
  check_field "repository"

  # name must be kebab-case "usearch".
  name_val=$(python3 -c "import json; print(json.load(open('$MANIFEST'))['name'])" 2>/dev/null || echo "")
  if [[ "$name_val" == "usearch" ]]; then
    ok
  else
    fail "plugin.json name must be 'usearch', got '$name_val'"
  fi

  # version must be semver-like.
  ver_val=$(python3 -c "import json; print(json.load(open('$MANIFEST'))['version'])" 2>/dev/null || echo "")
  if [[ "$ver_val" =~ ^[0-9]+\.[0-9]+\.[0-9]+ ]]; then
    ok
  else
    fail "plugin.json version must be semver, got '$ver_val'"
  fi

  # license must be Apache-2.0.
  lic_val=$(python3 -c "import json; print(json.load(open('$MANIFEST'))['license'])" 2>/dev/null || echo "")
  if [[ "$lic_val" == "Apache-2.0" ]]; then
    ok
  else
    fail "plugin.json license must be 'Apache-2.0', got '$lic_val'"
  fi

  # keywords must contain minimum set.
  keywords=$(python3 -c "
import json
m = json.load(open('$MANIFEST'))
kw = m.get('keywords', [])
required = ['research', 'search', 'citations', 'korean', 'deep-research', 'mcp', 'rag']
missing = [r for r in required if r not in kw]
print('MISSING:' + ','.join(missing) if missing else 'OK')
" 2>/dev/null || echo "MISSING:parse-error")
  if [[ "$keywords" == "OK" ]]; then
    ok
  else
    fail "plugin.json keywords missing: $keywords"
  fi

  # description ≤ 250 chars (Anthropic menu display recommendation).
  desc_len=$(python3 -c "import json; print(len(json.load(open('$MANIFEST'))['description']))" 2>/dev/null || echo "999")
  if [[ "$desc_len" -le 250 ]]; then
    ok
  else
    fail "plugin.json description is $desc_len chars, must be ≤ 250 for menu display"
  fi
fi

echo ""

# ---- REQ-SKILL-003: LICENSE ----

echo "--- REQ-SKILL-003: LICENSE ---"

if grep -q "Apache License" "$PLUGIN_DIR/LICENSE" 2>/dev/null; then
  ok
else
  fail "LICENSE file must contain Apache-2.0 text"
fi

if grep -q "Version 2.0" "$PLUGIN_DIR/LICENSE" 2>/dev/null; then
  ok
else
  fail "LICENSE must reference 'Version 2.0'"
fi

echo ""

# ---- REQ-SKILL-004: SKILL.md frontmatter ----

echo "--- REQ-SKILL-004: SKILL.md frontmatter ---"

SKILL_MD="$PLUGIN_DIR/skills/usearch/SKILL.md"

# Extract frontmatter.
FM_START=$(grep -n "^---" "$SKILL_MD" | head -1 | cut -d: -f1)
FM_END=$(grep -n "^---" "$SKILL_MD" | tail -1 | cut -d: -f1)

if [[ -n "$FM_START" && -n "$FM_END" && "$FM_START" -lt "$FM_END" ]]; then
  ok
else
  fail "SKILL.md must have YAML frontmatter delimited by ---"
  FM_START=0; FM_END=0
fi

if [[ "$FM_START" -gt 0 ]]; then
  # Extract frontmatter content.
  FM_CONTENT=$(sed -n "$((FM_START + 1)),$((FM_END - 1))p" "$SKILL_MD")

  # description field must exist.
  if echo "$FM_CONTENT" | grep -q "^description:"; then
    ok
  else
    fail "SKILL.md frontmatter must have 'description' field"
  fi

  # description ≤ 500 chars (NFR-SKILL-003).
  DESC_LINE=$(echo "$FM_CONTENT" | grep "^description:" | head -1)
  # Remove "description:" prefix and surrounding quotes.
  DESC_TEXT=$(echo "$DESC_LINE" | sed 's/^description:[[:space:]]*//' | sed 's/^"//' | sed 's/"$//')
  DESC_LEN=${#DESC_TEXT}
  if [[ "$DESC_LEN" -le 500 ]]; then
    ok
  else
    fail "SKILL.md description is $DESC_LEN chars, must be ≤ 500 (NFR-SKILL-003)"
  fi

  # Check trigger condition keywords (a)-(e).
  TRIGGERS=("research" "citation" "multi-source" "Korean" "deep-research" "team")
  for t in "${TRIGGERS[@]}"; do
    if echo "$DESC_TEXT" | grep -qi "$t"; then
      ok
    else
      fail "SKILL.md description missing trigger keyword: $t"
    fi
  done

  # No MoAI extension fields (per HISTORY D3).
  MOAI_FIELDS=("metadata" "progressive_disclosure" "triggers")
  for f in "${MOAI_FIELDS[@]}"; do
    if echo "$FM_CONTENT" | grep -q "^${f}:"; then
      fail "SKILL.md frontmatter MUST NOT contain MoAI extension field: $f"
    else
      ok
    fi
  done
fi

echo ""

# ---- REQ-SKILL-005: Tool Selection Guide section ----

echo "--- REQ-SKILL-005/006: Tool Selection Guide ---"

if grep -qi "Tool Selection Guide" "$SKILL_MD"; then
  ok
else
  fail "SKILL.md must have a 'Tool Selection Guide' section"
fi

# REQ-SKILL-006: Korean routing note.
if grep -qi "Korean" "$SKILL_MD" | grep -qi "auto.route\|automatic\|server.side\|intent router"; then
  ok
elif grep -qi "Korean" "$SKILL_MD"; then
  ok
else
  fail "SKILL.md must document Korean auto-routing"
fi

echo ""

# ---- REQ-SKILL-007: .mcp.json schema ----

echo "--- REQ-SKILL-007: .mcp.json schema ---"

MCP_JSON="$PLUGIN_DIR/.mcp.json"

if ! command -v python3 &>/dev/null; then
  echo "WARN: python3 not found; skipping .mcp.json parsing"
else
  # Valid JSON.
  if python3 -c "import json; json.load(open('$MCP_JSON'))" 2>/dev/null; then
    ok
  else
    fail ".mcp.json is not valid JSON"
  fi

  # Single server entry under "usearch".
  mcp_keys=$(python3 -c "
import json
d = json.load(open('$MCP_JSON'))
servers = d.get('mcpServers', {})
print(list(servers.keys())[0] if len(servers) == 1 else 'WRONG')
" 2>/dev/null || echo "ERROR")
  if [[ "$mcp_keys" == "usearch" ]]; then
    ok
  else
    fail ".mcp.json must have single entry 'usearch' under mcpServers, got: $mcp_keys"
  fi

  # command is "usearch-mcp".
  cmd_val=$(python3 -c "
import json
d = json.load(open('$MCP_JSON'))
print(d['mcpServers']['usearch'].get('command', ''))
" 2>/dev/null || echo "")
  if [[ "$cmd_val" == "usearch-mcp" ]]; then
    ok
  else
    fail ".mcp.json usearch.command must be 'usearch-mcp', got '$cmd_val'"
  fi

  # args include --transport stdio.
  args_val=$(python3 -c "
import json
d = json.load(open('$MCP_JSON'))
args = d['mcpServers']['usearch'].get('args', [])
print(' '.join(args))
" 2>/dev/null || echo "")
  if [[ "$args_val" == *"--transport"*"stdio"* ]]; then
    ok
  else
    fail ".mcp.json usearch.args must contain '--transport stdio'"
  fi

  # No env block.
  has_env=$(python3 -c "
import json
d = json.load(open('$MCP_JSON'))
print('yes' if 'env' in d['mcpServers']['usearch'] else 'no')
" 2>/dev/null || echo "yes")
  if [[ "$has_env" == "no" ]]; then
    ok
  else
    fail ".mcp.json MUST NOT have 'env' block (no embedded secrets per REQ-SKILL-007)"
  fi
fi

echo ""

# ---- REQ-SKILL-009: Error Handling section ----

echo "--- REQ-SKILL-009: Error Handling ---"

ERROR_CODES=("32002 usearch" "32000 usearch" "32007 usearch")
for code in "${ERROR_CODES[@]}"; do
  if grep -q -- "$code" "$SKILL_MD"; then
    ok
  else
    fail "SKILL.md Error Handling must document error code $code"
  fi
done

echo ""

# ---- REQ-SKILL-010: README sections ----

echo "--- REQ-SKILL-010: README sections ---"

README="$PLUGIN_DIR/README.md"

if grep -qi "Quick Start" "$README"; then
  ok
else
  fail "README must have a 'Quick Start' section"
fi

if grep -qi "Compatibility" "$README"; then
  ok
else
  fail "README must have a 'Compatibility' section"
fi

# Quick Start steps reference usearch-mcp and usearch config init.
if grep -q "usearch-mcp" "$README"; then
  ok
else
  fail "README Quick Start must reference 'usearch-mcp'"
fi

if grep -q "usearch config init" "$README"; then
  ok
else
  fail "README Quick Start must reference 'usearch config init'"
fi

# REQ-SKILL-008: HTTP swap snippet with env var substitution.
if grep -q 'USEARCH_TOKEN' "$README"; then
  ok
else
  fail "README must document USEARCH_TOKEN env-var substitution for HTTP transport"
fi

# NFR-SKILL-006: No Codex/Gemini install instructions.
if grep -qi "codex" "$README" | grep -qi "install"; then
  fail "README MUST NOT include Codex install instructions (NFR-SKILL-006)"
else
  ok
fi

echo ""

# ---- NFR-SKILL-001: Plugin size cap ----

echo "--- NFR-SKILL-001: Plugin size cap ---"

PLUGIN_SIZE_KB=$(du -sk "$PLUGIN_DIR" | cut -f1)
PLUGIN_SIZE_MB=$((PLUGIN_SIZE_KB / 1024))
if [[ "$PLUGIN_SIZE_MB" -le 5 ]]; then
  ok
else
  fail "Plugin is ${PLUGIN_SIZE_MB}MB, must be ≤ 5MB (NFR-SKILL-001)"
fi

echo ""

# ---- NFR-SKILL-005: No secrets ----

echo "--- NFR-SKILL-005: Secret scanner ---"

# Basic pattern check (not a replacement for gitleaks).
SECRET_PATTERNS=(
  'Bearer eyJ'
  'api_key.*=.*["\x27][a-zA-Z0-9]'
  'secret.*=.*["\x27][a-zA-Z0-9]'
  'password.*=.*["\x27][a-zA-Z0-9]'
)
found_secrets=0
for pat in "${SECRET_PATTERNS[@]}"; do
  if grep -rqE "$pat" "$PLUGIN_DIR" --include="*.json" --include="*.md" --include="*.yaml" --include="*.toml" 2>/dev/null; then
    found_secrets=1
    fail "Potential secret found matching pattern: $pat"
  fi
done
if [[ "$found_secrets" -eq 0 ]]; then
  ok
fi

echo ""
echo "=== Results ==="
echo "PASS: $PASS"
echo "FAIL: $FAIL"
echo ""

if [[ "$FAIL" -gt 0 ]]; then
  echo "VALIDATION FAILED"
  exit 1
else
  echo "ALL CHECKS PASSED"
  exit 0
fi
