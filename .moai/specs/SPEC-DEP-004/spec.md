---
id: SPEC-DEP-004
version: 0.1.0
status: draft
created: 2026-06-23
updated: 2026-06-23
author: limbowl
priority: P3
issue_number: 0
title: Go govulncheck remediation — bump go-jose/go-jose/v3 to v3.0.5 (GO-2026-4945)
milestone: CI debt cleanup
owner: expert-security
methodology: ddd
coverage_target: 0
depends_on: [SPEC-DEP-001]
blocks: []
related: [SPEC-SEC-001]
---

# SPEC-DEP-004: Go govulncheck remediation — bump go-jose/go-jose/v3 to v3.0.5 (GO-2026-4945)

## HISTORY

- 2026-06-23 (initial draft v0.1.0, limbowl via manager-spec):
  CI-debt remediation SPEC. govulncheck flags `github.com/go-jose/go-jose/v3`
  pinned at `v3.0.4` (`go.mod:24`) as **GO-2026-4945** (panic in JWE
  decryption; fixed in `v3.0.5`). This is a purely transitive/indirect
  dependency dragged in by `playwright-community/playwright-go` (used in
  `internal/access` for browser-driven content access). No first-party Go
  source imports `go-jose/v3` (zero `*.go` import hits), and the vulnerable
  JWE-decryption symbol is never reachable — govulncheck explicitly reports
  it as a **module-level finding only**: "1 vulnerability in modules you
  require, but your code doesn't appear to call" / "Your code is affected by
  0 vulnerabilities." The fix is a patch-level bump on the same v3 major
  line; no v4 migration is required. Owner: expert-security. Methodology:
  DDD (the change is a manifest-only patch; "implementation" is the manifest
  edit + re-verification of existing behavior).

  **CI-status clarification (do NOT claim "job goes red→green")**: the
  govulncheck CI job in `deps-audit.yml` is **call-graph filtered** — it
  runs `govulncheck -json ./...` and `jq` filters findings on
  `.finding.trace[0].module`, surfacing only vulnerabilities that are
  actually *called* by first-party code. Because GO-2026-4945 is reachable-
  symbol-clean (module-level only), **that CI job already PASSES today** and
  was never red for this advisory. The goal of this SPEC is therefore NOT to
  flip a red job green; it is to **eliminate the module-level advisory line**
  for `go-jose/v3` from the unfiltered `govulncheck ./...` output (a
  verifiable text-diff: the GO-2026-4945 stanza disappears).

  **False-positive flag (corrected SPEC hypothesis)**: an earlier hypothesis
  held that the auth/OIDC package (`internal/auth/validator.go:10`,
  `internal/auth/discovery.go:12`) imports the vulnerable v3. This is
  **FALSE**. `internal/auth` uses `github.com/coreos/go-oidc/v3/oidc`, whose
  transitive `go-jose` dependency is the **separate v4 line**
  (`go-jose/v4`, already on the patched `v4.1.4`). The v3 advisory does NOT
  touch the auth subsystem. This finding is recorded so the remediation is
  scoped only to the v3 indirect bump and does NOT modify `internal/auth`.

  **Run-phase verification caveat (UNVERIFIED fix version)**: `v3.0.5` is the
  *assumed* fix version but has **not been confirmed to exist or to clear
  GO-2026-4945** at SPEC-authoring time — the latest known pinned version is
  `v3.0.4`. Before committing, the run phase MUST verify, in order:
  (a) the version exists upstream —
  `go list -m -versions github.com/go-jose/go-jose/v3` lists `v3.0.5` (or the
  next published v3.0.x); and (b) govulncheck confirms the chosen version
  clears GO-2026-4945 (the advisory's `Fixed in:` field names it). If
  `v3.0.5` does not exist, the lowest published v3.0.x that govulncheck
  reports as fixed is used instead. Do NOT hardcode `v3.0.5` in the commit
  without this check.

---

## 1. Goal

Remediate govulncheck advisory **GO-2026-4945** by bumping the indirect
dependency `github.com/go-jose/go-jose/v3` from the vulnerable `v3.0.4`
(`go.mod:24`) to the patched `v3.0.5`-or-later, **eliminating the
module-level GO-2026-4945 advisory line** for `go-jose/v3` from the
unfiltered `govulncheck ./...` output (a verifiable text-diff). The
vulnerability is module-level only (the JWE decryption symbol is never
called by first-party code), so the call-graph-filtered `deps-audit.yml` CI
job **already passes** — this SPEC does not flip a red job green, it removes
the residual module-level advisory stanza. `v3` is pulled in transitively by
`playwright-community/playwright-go`, and the fix is a patch-level bump on
the same major line — so this is a manifest-only change (`go.mod` +
`go.sum`) with no source edits, no `internal/auth` change, and no v4
migration. (The exact fix version `v3.0.5` is UNVERIFIED at authoring time;
see the HISTORY run-phase verification caveat.)

---

## 2. Scope

### 2.1 In-scope

- Bump `github.com/go-jose/go-jose/v3` from `v3.0.4` to `v3.0.5` (or later
  v3.0.x) in `go.mod` (the indirect `require` line at `go.mod:24`), and
  refresh the corresponding `go.sum` hashes.
- Keep `go-jose/v3` as an **indirect** require (no new first-party import is
  added — it remains a transitive dep of `playwright-community/playwright-go`).
- Re-run the unfiltered `govulncheck ./...` to confirm the GO-2026-4945
  module-level advisory stanza for `go-jose/v3` no longer appears (text-diff
  vs the pre-bump run). NOTE: the call-graph-filtered `deps-audit.yml` job
  already passes for this advisory; the verifiable change is in the
  unfiltered output.
- Re-verify that `playwright-go` (`internal/access`) and the OIDC/auth
  subsystem (`internal/auth`) still build and pass their test suites after
  the bump.

### 2.2 Out-of-scope

- Any change to `internal/auth` or the OIDC/`go-oidc/v3` → `go-jose/v4`
  dependency chain. The v4 line is already on the patched `v4.1.4` and is
  untouched by this SPEC. (See HISTORY false-positive flag.)
- A v3 → v4 migration. The advisory is fixed within the v3 major line;
  migrating majors is unnecessary and explicitly excluded.
- Adding a first-party direct import of `go-jose/v3`. It stays indirect.
- Upgrading `playwright-community/playwright-go` itself, or any other
  unrelated dependency.
- Source-code edits of any kind (there are no `go-jose/v3` import sites to
  change).

---

## 3. EARS Requirements

| ID | Pattern | Requirement | Priority | Testable Acceptance |
|----|---------|-------------|----------|---------------------|
| **REQ-DEP4-010** | Ubiquitous | The build dependency manifest (`go.mod`) SHALL pin `github.com/go-jose/go-jose/v3` at version `v3.0.5` or later to remediate GO-2026-4945. | P3 | `grep 'go-jose/go-jose/v3' go.mod` shows `v3.0.5` or higher; `go mod verify` succeeds. |
| **REQ-DEP4-020** | Optional (Where) | WHERE `go-jose/v3` is present only as a transitive dependency of `playwright-community/playwright-go`, the system SHALL retain it as an `// indirect` require and SHALL bump only its version (no first-party direct import added). | P3 | `go.mod` line for `go-jose/v3` retains the `// indirect` marker; `grep -rn 'go-jose/go-jose/v3' --include='*.go' .` returns zero import hits. |
| **REQ-DEP4-030** | Event-Driven (When) | WHEN the unfiltered `govulncheck ./...` runs, the system SHALL no longer emit the module-level GO-2026-4945 advisory stanza for `github.com/go-jose/go-jose/v3`. | P3 | The GO-2026-4945 stanza naming `go-jose/v3` is absent from `govulncheck ./...` output (a text-diff vs the pre-bump run). NOTE: the call-graph-filtered `deps-audit.yml` job already passes — this REQ targets the unfiltered output, not a job flip. |
| **REQ-DEP4-040** | Ubiquitous | The OIDC/auth subsystem SHALL continue to depend exclusively on `github.com/go-jose/go-jose/v4` (via `coreos/go-oidc/v3`) and SHALL remain unmodified by this remediation. | P3 | `go mod why github.com/go-jose/go-jose/v4` shows the `internal/auth → go-oidc/v3 → go-jose/v4` chain; `git diff` touches no file under `internal/auth/`. |
| **REQ-DEP4-050** | Ubiquitous | The `go-jose/v3` require line SHALL stay pinned at the fixed version (`v3.0.5`+) for as long as `playwright-go` transitively requires it. | P3 | After `go mod tidy`, the `go-jose/v3` require line is still present at `v3.0.5`+ (retained because `playwright-go`'s `go.mod` constrains it); `go mod verify` succeeds. |
| **REQ-DEP4-060** | Event-Driven (When) | WHEN the `go-jose/v3` version is bumped, the system SHALL pass `go build ./...` and the `internal/access` and `internal/auth` test suites before merge. | P3 | `go build ./...` exits 0; `go test ./internal/access/... ./internal/auth/...` passes. |

EARS notes:
- All requirements are positively phrased (no double negatives) and each maps
  to a concrete shell-verifiable check.
- The remediation decision is **fix**, not suppress-with-justification: the
  fix is a published, low-risk patch bump (`v3.0.5`), so there is no need to
  add an `ops/security/vuln-exceptions.yaml` exception entry. No
  needs-decision findings remain open.

---

## 4. Acceptance Criteria

Headline acceptance (CI-debt SPEC): **the GO-2026-4945 module-level advisory
stanza for `go-jose/v3` is eliminated from the unfiltered `govulncheck ./...`
output** (verifiable text-diff). The call-graph-filtered `deps-audit.yml` job
already passes for this advisory and is not the success signal here.

| # | Given | When | Then | Maps to |
|---|-------|------|------|---------|
| AC-1 | `go.mod:24` currently pins `go-jose/v3 v3.0.4`; `v3.0.5` existence is unverified | `go list -m -versions github.com/go-jose/go-jose/v3` confirms `v3.0.5`+ exists, then `go get github.com/go-jose/go-jose/v3@v3.0.5 && go mod tidy && go mod verify` is run | `v3.0.5`+ is confirmed published, `go.mod` pins it, `go.sum` hashes refreshed, `go mod verify` reports "all modules verified" | REQ-DEP4-010 |
| AC-2 | The bumped `go.mod` | `grep` for the `go-jose/v3` line and a repo-wide `*.go` import grep are run | the line retains `// indirect` and the import grep returns zero hits | REQ-DEP4-020 |
| AC-3 | The patched dependency tree | the unfiltered `govulncheck ./...` is run and its output diffed against the pre-bump run | the GO-2026-4945 module-level stanza for `go-jose/v3` is absent from the post-bump output (the call-graph-filtered `deps-audit.yml` job already passed and is unchanged) | REQ-DEP4-030 |
| AC-4 | The remediation diff | `git diff` and `go mod why go-jose/v4` are inspected | no file under `internal/auth/` is modified; the v4 chain via `go-oidc/v3` is intact | REQ-DEP4-040 |
| AC-5 | The bumped manifest | `go mod tidy` is re-run | the `go-jose/v3 v3.0.5`+ require line is retained (constrained by `playwright-go`) | REQ-DEP4-050 |
| AC-6 | The bumped dependency | `go build ./...` and `go test ./internal/access/... ./internal/auth/...` run | build exits 0 and both test suites pass | REQ-DEP4-060 |

### Edge cases

- **EC-1 (false-positive guard)**: A reviewer must confirm the diff does NOT
  touch `internal/auth`. The original SPEC hypothesis that auth imports v3 is
  false; auth uses `go-oidc/v3 → go-jose/v4`. A diff that modifies
  `internal/auth` indicates mis-scoped work and MUST be rejected.
- **EC-2 (tidy drop)**: If a future `go mod tidy` attempts to remove the v3
  require line, it will be retained because `playwright-go`'s own `go.mod`
  transitively constrains it (REQ-DEP4-050). Verify presence post-tidy.
- **EC-3 (playwright compatibility)**: `v3.0.5` is a patch bump on the same
  major; `playwright-go` is expected to remain compatible. `go build ./...`
  after `go mod tidy` is the compatibility gate.

---

## 5. Findings Classification

| Locus | Issue | Status | Action |
|-------|-------|--------|--------|
| `go.mod:24` | `go-jose/v3` pinned at vulnerable `v3.0.4` (GO-2026-4945) | confirmed | **Fix** — bump to `v3.0.5`+ (REQ-DEP4-010) |
| govulncheck Module Results | GO-2026-4945 reported at MODULE level only; vulnerable symbol not called | confirmed | Module-only finding; bump clears the advisory stanza in the unfiltered output (REQ-DEP4-030). The call-graph-filtered `deps-audit.yml` job already passes. No code path remediation needed. |
| `go mod why -m github.com/go-jose/go-jose/v3` (module graph) | `v3` pulled transitively by `playwright-community/playwright-go` (`internal/access`); zero `*.go` import hits | confirmed | Keep indirect; bump only (REQ-DEP4-020) |
| `internal/auth/validator.go:10`, `discovery.go:12` | hypothesis that auth/OIDC imports vulnerable v3 | **false-positive** | **Flagged & excluded.** Auth uses `go-oidc/v3 → go-jose/v4` (patched `v4.1.4`). Do NOT modify `internal/auth` (REQ-DEP4-040, EC-1). |

No `needs-decision` findings: the remediation is a straightforward **fix**
(patch bump), not a suppress-with-justification.

---

## 6. Dependencies & Blockers

### 6.1 Upstream SPEC dependencies (depends_on)

- **SPEC-DEP-001 (implemented)** — dependency baseline + the govulncheck CI
  job (`deps-audit.yml`) that surfaces GO-2026-4945. This SPEC remediates a
  finding produced by that job and is verified by re-running it.

### 6.2 Related (related)

- **SPEC-SEC-001 (implemented)** — security hardening / dependency-audit
  consolidation. This SPEC is a downstream remediation of a govulncheck
  finding within the audit policy SEC-001/DEP-001 established. No code
  dependency; classification reference only.

### 6.3 Blockers

- None expected. `go-jose/go-jose/v3 v3.0.5` is the *assumed* documented fix
  per govulncheck (`Fixed in:` field), but its existence is UNVERIFIED at
  authoring time (latest known pinned is `v3.0.4`). The run phase confirms
  publication via `go list -m -versions` (see HISTORY caveat). If `v3.0.5` is
  not yet published, this becomes an upstream-wait blocker until the fixed
  v3.0.x lands.

### 6.4 External dependencies (run-phase)

| Dependency | Action | Note |
|------------|--------|------|
| `github.com/go-jose/go-jose/v3` | bump `v3.0.4` → `v3.0.5`+ | assumed GO-2026-4945 fix; existence UNVERIFIED — confirm via `go list -m -versions` before commit (see HISTORY caveat) |
| `playwright-community/playwright-go` | unchanged | must remain compatible with `go-jose/v3 v3.0.5` (patch bump on same major; verify via `go build` after tidy) |

---

## 7. Files to Modify

| Path | Change |
|------|--------|
| `go.mod` | bump indirect require `github.com/go-jose/go-jose/v3` from `v3.0.4` to `v3.0.5` (line 24); retain `// indirect` marker |
| `go.sum` | refresh `go-jose/v3` module + hash entries for `v3.0.5` (via `go mod tidy`) |

Run-phase command sequence (manifest-only, no source edits):

```
go get github.com/go-jose/go-jose/v3@v3.0.5
go mod tidy
go mod verify
govulncheck ./...
go build ./...
go test ./internal/access/... ./internal/auth/...
```

### 7.1 Explicitly unchanged

- `internal/auth/**` — uses `go-oidc/v3 → go-jose/v4` (patched). NOT modified
  (REQ-DEP4-040, EC-1).
- `internal/access/**` — consumer of `playwright-go`; no source change, only
  the transitive `go-jose/v3` version it sees is bumped. Re-tested via
  REQ-DEP4-060.
- All other Go source — zero `go-jose/v3` import sites exist; nothing to edit.

---

*End of SPEC-DEP-004 v0.1.0 (draft).*
