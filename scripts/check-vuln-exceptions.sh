#!/usr/bin/env bash
# scripts/check-vuln-exceptions.sh — Vulnerability exception deadline enforcement
# SPEC-SEC-001 REQ-SEC-003
#
# Validates ops/security/vuln-exceptions.yaml schema and fails (exit 1) if any
# exception's review_deadline has passed without renewal. Also enforces the
# 90-day cap (review_deadline <= discovered_at + 90 days).
#
# Uses python3 (stdlib only) for YAML-lite parsing to avoid extra CI deps;
# falls back gracefully if the file is absent (exit 0 with a notice).
#
# Override "today" for testing via VULN_EXCEPTIONS_NOW=YYYY-MM-DD.

set -euo pipefail

FILE="${1:-ops/security/vuln-exceptions.yaml}"

if [ ! -f "$FILE" ]; then
  echo "NOTICE: $FILE not found; no vulnerability exceptions to check."
  exit 0
fi

python3 - "$FILE" <<'PY'
import datetime
import os
import sys

path = sys.argv[1]

now_override = os.environ.get("VULN_EXCEPTIONS_NOW")
today = (
    datetime.date.fromisoformat(now_override)
    if now_override
    else datetime.date.today()
)

# Prefer PyYAML if available; otherwise use a minimal parser sufficient for the
# flat list-of-mappings schema in vuln-exceptions.yaml.
def load(path):
    try:
        import yaml  # type: ignore

        with open(path) as f:
            return yaml.safe_load(f) or {}
    except ImportError:
        return load_minimal(path)


def load_minimal(path):
    """Minimal parser for `exceptions:` as a list of single-level mappings."""
    data = {"exceptions": []}
    current = None
    in_exceptions = False
    with open(path) as f:
        for raw in f:
            line = raw.rstrip("\n")
            stripped = line.strip()
            if not stripped or stripped.startswith("#"):
                continue
            if stripped.startswith("exceptions:"):
                in_exceptions = True
                rest = stripped.split(":", 1)[1].strip()
                if rest and rest != "[]":
                    pass
                continue
            if not in_exceptions:
                continue
            if stripped.startswith("- "):
                current = {}
                data["exceptions"].append(current)
                kv = stripped[2:]
                if ":" in kv:
                    k, v = kv.split(":", 1)
                    current[k.strip()] = v.strip()
            elif current is not None and ":" in stripped:
                k, v = stripped.split(":", 1)
                current[k.strip()] = v.strip()
    return data


data = load(path)
exceptions = data.get("exceptions") or []

required = ["cve_id", "dependency", "severity", "rationale",
           "discovered_at", "review_deadline", "owner"]
valid_sev = {"critical", "high", "medium", "low"}

errors = []
for i, exc in enumerate(exceptions):
    if not isinstance(exc, dict):
        errors.append(f"exception #{i}: not a mapping")
        continue
    label = exc.get("cve_id", f"#{i}")
    for key in required:
        if key not in exc or exc[key] in (None, ""):
            errors.append(f"{label}: missing required field '{key}'")
    sev = str(exc.get("severity", "")).strip().lower()
    if sev and sev not in valid_sev:
        errors.append(f"{label}: invalid severity '{sev}'")

    disc = exc.get("discovered_at")
    deadline = exc.get("review_deadline")
    try:
        if deadline:
            d = datetime.date.fromisoformat(str(deadline).strip())
            if d < today:
                errors.append(
                    f"{label}: review_deadline {d} has PASSED "
                    f"(today={today}) — renew or remediate"
                )
            if disc:
                dd = datetime.date.fromisoformat(str(disc).strip())
                if d > dd + datetime.timedelta(days=90):
                    errors.append(
                        f"{label}: review_deadline {d} exceeds "
                        f"discovered_at+90d ({dd + datetime.timedelta(days=90)})"
                    )
    except ValueError as e:
        errors.append(f"{label}: invalid date format ({e})")

if errors:
    print("FAIL: vuln-exceptions.yaml validation errors:")
    for e in errors:
        print(f"  - {e}")
    sys.exit(1)

print(f"PASS: {len(exceptions)} vulnerability exception(s) valid; "
      f"no expired deadlines (today={today}).")
PY
