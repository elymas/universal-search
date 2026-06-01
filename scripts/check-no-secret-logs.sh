#!/usr/bin/env bash
# scripts/check-no-secret-logs.sh — REQ-SEC-018 secret-in-logs guard.
# SPEC-SEC-001 REQ-SEC-018
#
# Asserts (exit 1 on violation) that no secret-bearing variable is formatted
# into a log, error, or response path in NON-TEST Go source.
#
# Detection model (conservative — favors precision over recall to keep the gate
# actionable; TestNoSecretInLogs in Go covers the runtime side):
#
#   1. The line calls a formatting/logging/response sink (fmt/log/slog/Error/
#      http.Error).
#   2. The line references a SECRET-bearing identifier as a *value* — a field
#      access (`.MasterKey`, `.ClientSecret`) or bare identifier
#      (`masterKey`, `apiToken`) — NOT inside a string literal (env-var NAMES
#      like "NAVER_CLIENT_SECRET" are safe to log).
#
# Excluded to avoid known false positives:
#   - redaction helpers (redactKey / redact*) — they consume the secret to
#     REMOVE it from output.
#   - token-COUNT identifiers (TokenBudget, TokensUsed, TokenEstimate,
#     token budget) — these are integers, not credentials.
#   - "Keyword(s)" — search terms, not keys.
#
# Scope: internal/ and cmd/. Test files excluded (fixtures embed sample creds).

set -euo pipefail

python3 - <<'PY'
import os
import re
import sys

# Sinks where a secret value must never appear.
SINK = re.compile(
    r"(fmt\.(Sprint|Sprintf|Sprintln|Print|Printf|Println|Errorf)\(|"
    r"log\.(Print|Printf|Println|Fatal|Fatalf)\(|"
    r"slog\.(Info|Warn|Error|Debug)\(|"
    r"\.Errorf?\(|http\.Error\()"
)

# A secret-bearing VALUE: a field access or bare identifier whose name ends in
# Secret/Password, or Key/Token NOT preceded by a count-ish word and NOT
# "Keyword". We approximate by scanning identifiers outside string literals.
SECRET_VALUE = re.compile(
    r"(?<![\"'])\b"
    r"[A-Za-z0-9_]*"
    r"(Secret|Password|secret|password|"
    r"MasterKey|masterKey|APIKey|apiKey|ApiKey|"
    r"ClientSecret|AccessToken|accessToken|BearerToken|bearerToken|"
    r"AuthToken|authToken|JWTSecret|jwtSecret)"
    r"\b"
)

# Lines that are safe despite matching: redaction helpers consume the secret.
SAFE_LINE = re.compile(r"\bredact[A-Za-z]*\(")

violations = []
for root in ("internal", "cmd"):
    for dirpath, _dirs, files in os.walk(root):
        for fn in files:
            if not fn.endswith(".go") or fn.endswith("_test.go"):
                continue
            path = os.path.join(dirpath, fn)
            try:
                with open(path, encoding="utf-8") as f:
                    lines = f.readlines()
            except OSError:
                continue
            for n, line in enumerate(lines, 1):
                if not SINK.search(line):
                    continue
                if SAFE_LINE.search(line):
                    continue
                # Strip double-quoted string literals so env-var NAMES inside
                # strings (e.g. "NAVER_CLIENT_SECRET not set") are not matched.
                stripped = re.sub(r'"(?:[^"\\]|\\.)*"', '""', line)
                if SECRET_VALUE.search(stripped):
                    violations.append((path, n, line.strip()))

if violations:
    print("FAIL: secret-bearing value formatted into a log/error/response path "
          "(REQ-SEC-018):")
    for path, n, text in violations:
        print(f"::error file={path},line={n}::{text}")
        print(f"  {path}:{n}: {text}")
    print("Fix: log the key NAME only, or redact the value (see redactKey).")
    sys.exit(1)

print("PASS: no secret-bearing values formatted into log/error/response "
      "paths (REQ-SEC-018).")
PY
