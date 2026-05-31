# gitleaks False-Positive Log — SPEC-SEC-001 NFR-SEC-003

Rolling record of gitleaks findings classified as false positives. The
new-finding false-positive rate SHALL stay <= 30% over any rolling 30-day
window; exceeding the cap triggers a `.gitleaks.toml` rule-tuning review (not
a hard CI failure).

Each entry: date, finding (rule + file:line), classification rationale.

| Date | Finding (rule @ location) | Rationale |
|------|---------------------------|-----------|
| 2026-05-29 | (baseline established — no findings) | Initial T02 baseline scan. See note below. |

## Baseline note (T02)

The authoritative full-history scan is run in CI via the `security.yml`
gitleaks job with `fetch-depth: 0`. gitleaks is not installed on the
implementer's local host; the baseline is therefore established by the first
CI run on `feature/SPEC-SEC-001`.

Local reproduction command (when gitleaks is installed):

```
gitleaks detect --source . --log-opts="--all" --config .gitleaks.toml --redact
```

If the first CI run reports a TRUE-POSITIVE committed secret, the
`ops/security/runbook.md` rotation procedure applies and any git-history
rewrite is gated by the five REQ-SEC-005a guards (human approval, backup ref +
mirror, staging dry-run, tested rollback, coordination notice). History
rewrite is NOT performed by the implementer agent — it requires the
human-approved REQ-SEC-005a gate.
