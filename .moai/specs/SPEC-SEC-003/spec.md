---
id: SPEC-SEC-003
version: 0.1.0
status: draft
created: 2026-06-23
updated: 2026-06-23
author: limbowl
priority: P1
issue_number: 0
title: Go crypto and TLS security hardening — gosec G404/G402/G115 + semgrep math-random/missing-ssl-minversion suppression-mechanism alignment
milestone: CI debt remediation
owner: expert-security
methodology: ddd
coverage_target: 85
depends_on: [SPEC-SEC-001]
blocks: []
related: [SPEC-SEC-002]
---

# SPEC-SEC-003: Go crypto and TLS security hardening (gosec + semgrep)

## HISTORY

- 2026-06-23 (initial draft v0.1.0, limbowl via manager-spec):
  CI-debt remediation SPEC. The `security.yml` workflow (introduced by
  SPEC-SEC-001 REQ-SEC-010) runs **standalone gosec** (`gosec -severity
  high -confidence medium -exclude-dir testdata -exclude-dir vendor
  -exclude-dir node_modules -exclude-dir web -conf .gosec.json ./...`,
  gosec @v2.27.1) AND **semgrep** (`semgrep ci --config p/golang --config
  p/owasp-top-ten --config p/jwt`, image `semgrep/semgrep:1.85.0`). These
  are two separate gates with two separate inline-suppression syntaxes.

  This SPEC does **NOT** invent a new security system. It closes a
  suppression-mechanism mismatch discovered during investigation of the
  Go crypto/TLS findings:

  **Root cause (two distinct):**
  1. **Suppression-mechanism mismatch.** The codebase suppresses gosec
     findings with `//nolint:gosec` comments, but CI runs STANDALONE gosec,
     which only honors `#nosec G<NNN> -- reason` directives, not
     golangci-lint's `//nolint`. Separately, semgrep ignores BOTH annotation
     styles — only inline `// nosemgrep: <rule-id>` comments (or
     `.semgrepignore` path globs) suppress semgrep. So the math/rand jitter
     and InsecureSkipVerify sites that LOOK annotated are still live findings
     for the actual CI gates.
  2. **Missing justification for intentional non-crypto randomness.** Using
     `math/rand` for retry/backoff jitter is correct by design (crypto/rand
     is unnecessary and slower for timing randomness), but it was never given
     a gate-recognized `#nosec G404` justification nor migrated to a form the
     gates accept (`math/rand/v2`). The InsecureSkipVerify sites are
     genuinely security-relevant: the `// test-only` comment is inaccurate —
     the path is reachable at runtime whenever `allow_private_issuer: true`
     is set in OIDC config (config-gated, NOT build-tag-gated).

  **Confirmed findings** (verified against the live codebase, file:line):
  - **G404 / semgrep math-random (math/rand, no directive)** —
    `internal/synthesis/client.go:271`, `internal/embedder/client.go:185`,
    `internal/deepreport/client.go:210`. Retry/backoff jitter via
    `rand.Float64()`; `import "math/rand"`; NO `#nosec`, NO `// nosemgrep`.
  - **G402 InsecureSkipVerify=true** — `internal/auth/discovery.go:47` and
    `:52` (nil-Transport branch). `//nolint:gosec // test-only` does NOT
    suppress standalone gosec or semgrep; the `// test-only` comment is
    factually wrong (gated by `allowPrivateIssuer` config, runtime-reachable).
  - **G115 int→uint64 conversion** — `internal/index/index.go:101`
    (`uint64(idx.embedder.Dimensions())`) and `internal/index/dispatch.go:77`
    (`uint64(maxRes)`, `maxRes` clamped to >0 at lines 63-64). G115 is MEDIUM
    severity (does NOT fail the `-severity high` gosec gate) but semgrep/audit
    rulesets may surface it.

  **needs-decision findings** (flagged for run-phase / reviewer judgment —
  fix vs suppress-with-justification):
  - `internal/llm/retry.go:56` — has `//nolint:gosec` but NOT `#nosec`;
    `//nolint` suppresses golangci-lint's gosec, NOT standalone gosec, and
    semgrep ignores both. Same remediation class as the confirmed G404 sites.
  - `internal/index/tokenizer/client.go:53` — already imports `math/rand/v2`
    (not `math/rand`). gosec G404 does not flag `math/rand/v2`; the semgrep
    `go.lang...math-random` pattern keys on the `math/rand` import string and
    may not match v2. Add `// nosemgrep` ONLY if semgrep still flags it
    against the pinned ruleset version.
  - `internal/obs/metrics/metrics.go:388` — graceful-shutdown goroutine uses
    `context.WithTimeout(context.Background(), 5*time.Second)` (the call at
    line 388, inside the `go func()` block spanning lines 386-389). This is the
    path triggered BY parent `ctx.Done()`; deriving from the cancelled ctx
    would defeat the 5-second drain window. Likely correct as-is — apply a
    suppression/explanatory comment ONLY if a gate flags it.

  **false-positive findings** (explicitly NO code change):
  - `internal/access/phase4_tls.go:46` — semgrep `missing-ssl-minversion`.
    The adjacent line 47 already sets `MinVersion: tls.VersionTLS12`
    (and `MaxVersion: tls.VersionTLS13`). The config is compliant; this is a
    semgrep false positive. If it persists, suppress with `// nosemgrep`
    referencing the present MinVersion — do NOT add redundant code.

  **Blockers (reviewer/run-phase decisions, not mechanical):**
  1. gosec and semgrep use mutually incompatible inline-suppression
     syntaxes. A single `#nosec` will NOT satisfy semgrep; a single
     `// nosemgrep` will NOT satisfy gosec. Every co-flagged line needs BOTH
     directives, validated by running both gates locally.
  2. The `// test-only` comment on `discovery.go:47/52` is factually wrong.
     A security reviewer must decide whether `allow_private_issuer` is
     acceptable in production or should be hard-disabled in release builds.
     This is a security policy decision, not a mechanical fix.

  **External pins (rule behavior is version-tied):**
  - gosec **v2.27.1** (pinned in `.github/workflows/security.yml`) —
    G404/G402/G115 rule behavior tied to this version.
  - semgrep **1.85.0** (`semgrep/semgrep:1.85.0`) with rulesets `p/golang`,
    `p/owasp-top-ten`, `p/jwt` — `math-random` and `missing-ssl-minversion`
    rule definitions are ruleset-version-dependent; whether `math-random`
    matches `math/rand/v2` must be verified against this exact version.

  Scope: remediation of EXISTING CI debt only — not a new feature.
  Methodology: DDD (audit existing surface → preserve behavior →
  improve with gate-recognized directives or minimal code change).
  Owner: expert-security.

---

## 1. Overview

SPEC-SEC-003 closes a CI-debt gap in the Go crypto/TLS security gates that
SPEC-SEC-001 introduced. The `security.yml` `gosec` and `semgrep` jobs run
with annotation-suppression syntaxes (`#nosec`, `// nosemgrep`) that the
existing `//nolint:gosec` comments in the codebase do NOT satisfy. As a
result, several math/rand jitter sites, two InsecureSkipVerify sites, and
two int→uint64 conversion sites are live (un-suppressed) findings for the
actual CI gates even though they appear annotated.

This SPEC does NOT add new security infrastructure. It (a) replaces or
augments the ineffective suppressions with gate-recognized directives,
(b) migrates non-crypto jitter to a gate-friendly form where cleaner,
(c) corrects a factually wrong `// test-only` comment and surfaces the
`allow_private_issuer` runtime risk, and (d) records the false-positive and
needs-decision findings so they are not silently re-suppressed.

### 1.1 What ships

| Layer | Artifact | Purpose |
|-------|----------|---------|
| Code | `internal/synthesis/client.go` | G404/math-random directive or `math/rand/v2` migration (line 271) |
| Code | `internal/embedder/client.go` | G404/math-random directive or `math/rand/v2` migration (line 185) |
| Code | `internal/deepreport/client.go` | G404/math-random directive or `math/rand/v2` migration (line 210) |
| Code | `internal/llm/retry.go` | replace ineffective `//nolint:gosec` with gate-recognized directives (line 56) |
| Code | `internal/index/tokenizer/client.go` | add `// nosemgrep` ONLY if semgrep flags the existing `math/rand/v2` (line 53) |
| Code | `internal/auth/discovery.go` | replace `//nolint:gosec`/`// test-only` with `#nosec G402` + `// nosemgrep` + accurate config-gated comment (lines 47, 52) |
| Code | `internal/index/index.go` | guard or `#nosec G115` for `uint64(Dimensions())` (line 101) |
| Code | `internal/index/dispatch.go` | guard or `#nosec G115` for `uint64(maxRes)` (line 77) |
| Code | `internal/obs/metrics/metrics.go` | explanatory/suppression comment for shutdown `context.Background()` IF a gate flags it (line 388; the `context.WithTimeout(context.Background(), …)` call inside the `go func()` block at lines 386-389) |
| CI | `.github/workflows/security.yml` (existing) | gosec + semgrep gate must pass clean on `main` (unchanged config) |

### 1.2 Motivation

The headline driver is a clean security CI gate on `main`. The `security.yml`
`gosec` and `semgrep` jobs are the gates SPEC-SEC-001 REQ-SEC-010 promised
would block on HIGH gosec findings and on new semgrep `p/golang` findings.
Today those gates either pass only because the relevant rule severities
(G115 MEDIUM, G404 confidence) fall below the configured threshold, or are
suppressed by directives the standalone tools ignore — leaving the suppression
intent unenforced. Aligning the suppressions with the gate syntaxes makes the
gate honest: a future genuinely-dangerous finding will not be masked by an
ineffective `//nolint`, and an intentional non-crypto jitter will not require
a reviewer to re-litigate it.

### 1.3 Forward-compatibility commitments

- **SPEC-SEC-001 (implemented)**: REQ-SEC-010 defines the gosec + semgrep
  gate. This SPEC does NOT change `.gosec.json`, `.semgrepignore`, or the
  `security.yml` job invocations; it only changes call-site annotations and
  (where cleaner) the randomness source. Behavior of the retry/backoff and
  OIDC discovery code paths is PRESERVED (DDD characterization).
- **SPEC-SEC-002 (related)**: sibling security-hardening SPEC; no shared
  files asserted. Coordinate only if both touch `internal/auth/`.

---

## 2. EARS Requirements

REQ ids use the `SEC3` domain prefix (`REQ-SEC3-0x0`), distinct from
SPEC-SEC-001 (`REQ-SEC-010`/`REQ-SEC-017`) and SPEC-SEC-005 to avoid
collision. Numbered 10/20/30… Each requirement is
testable by running the two gates locally (`gosec -severity high
-confidence medium -conf .gosec.json ./...` and `semgrep ci --config
p/golang --config p/owasp-top-ten --config p/jwt`) and/or by grep/test
assertions on the named files.

### 2.1 Gate-recognition baseline (D1)

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-SEC3-010** | Ubiquitous | The security CI gate SHALL fail on any gosec HIGH-severity finding and on any semgrep `p/golang` finding that is not suppressed by a directive the respective tool recognizes. The gosec suppression token is the literal `#nosec` (hash), distinct from the ineffective `//nolint:gosec` and `//nolint` forms; gosec only honors comments containing the `#nosec` substring, so a `//nosec` (double-slash, no hash) directive contains no `#nosec` token and would NOT suppress. A gosec finding SHALL be suppressed only by a `#nosec G<NNN> -- <reason>` directive (the form standalone gosec honors), NOT by `//nolint:gosec` (which only suppresses golangci-lint's bundled gosec, not the standalone `gosec` binary that CI runs). A semgrep finding SHALL be suppressed only by an inline `// nosemgrep: <rule-id>` directive or a `.semgrepignore` path glob, NOT by any gosec or golangci directive. | P1 | `gosec -severity high -confidence medium -conf .gosec.json ./...` exits 0 on `main`; `semgrep ci --config p/golang --config p/owasp-top-ten --config p/jwt` reports zero blocking findings on `main`; no remaining `//nolint:gosec` is relied on as the sole suppression for a gate-blocking finding. |

### 2.2 Non-cryptographic jitter (D2)

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-SEC3-020** | Optional (WHERE) | WHERE non-cryptographic randomness is used for retry or backoff jitter, the code SHALL satisfy both gates by EITHER (a) annotating the call site with BOTH a `#nosec G404 -- non-cryptographic retry jitter; timing randomness only` directive AND a `// nosemgrep: go.lang.security.audit.crypto.math-random-used` directive, OR (b) migrating the randomness source from `math/rand` to `math/rand/v2` (which gosec G404 does not flag) paired with a `// nosemgrep` directive where semgrep still matches. The retry/backoff timing behavior SHALL be preserved. This requirement covers `internal/synthesis/client.go:271`, `internal/embedder/client.go:185`, `internal/deepreport/client.go:210`, and `internal/llm/retry.go:56`. | P1 | After remediation, gosec reports no G404 finding for the four named lines AND semgrep reports no `math-random` finding for them; a unit/characterization test confirms jitter still falls within the documented bound (e.g. `[base-jitter, base+jitter]`). |
| **REQ-SEC3-021** | Optional (WHERE) | WHERE a retry-jitter site already imports `math/rand/v2` (currently `internal/index/tokenizer/client.go:53`), the code SHALL add a `// nosemgrep: go.lang.security.audit.crypto.math-random-used` directive ONLY when the pinned semgrep ruleset (`p/golang` @ semgrep 1.85.0) actually flags the `math/rand/v2` usage; when the ruleset does not match `math/rand/v2`, the site SHALL be left unchanged and recorded as not-applicable. The code SHALL NOT add a `#nosec G404` here, because gosec G404 does not flag `math/rand/v2`. | P1 | A local semgrep run against the pinned ruleset determines applicability; if flagged, the `// nosemgrep` directive is present and semgrep passes; if not flagged, the file is unchanged and the SPEC acceptance record marks the line not-applicable. |

### 2.3 Effective suppression syntax (D1)

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-SEC3-030** | Conditional (IF-THEN) | IF code suppresses a gosec finding, THEN the suppression SHALL use the `#nosec G<NNN> -- <reason>` form recognized by standalone gosec, because CI runs the standalone `gosec` binary (`@v2.27.1`) rather than golangci-lint. A `//nolint:gosec` comment SHALL NOT be the sole suppression for any finding that the configured gosec gate would otherwise block. | P1 | grep confirms no gate-blocking gosec finding relies solely on `//nolint:gosec`; each intentional suppression carries a `#nosec G<NNN> -- <reason>` with a human-readable reason. |

### 2.4 TLS InsecureSkipVerify gating (D3)

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-SEC3-040** | Optional (WHERE) | WHERE TLS `InsecureSkipVerify` is set (`internal/auth/discovery.go:47` and `:52`), the code SHALL (a) keep the setting gated behind the explicit `allowPrivateIssuer` (`config.AllowPrivateIssuer`) configuration flag, (b) suppress both gates with `#nosec G402 -- InsecureSkipVerify gated by allow_private_issuer config for dev/CI OIDC against self-signed issuers` AND a `// nosemgrep` directive for the corresponding semgrep TLS audit rule, and (c) replace the factually inaccurate `// test-only` comment with an accurate statement that the path is configuration-gated (NOT build-tag-gated) and runtime-reachable when `allow_private_issuer` is enabled. | P1 | gosec reports no G402 finding for the two lines; semgrep reports no TLS-audit finding for them; the adjacent comment no longer claims "test-only" and instead documents the config gate; a reviewer-visible note records the production-acceptability decision (see Blocker B2). |
| **REQ-SEC3-041** | Event-Driven (WHEN) | WHEN `allow_private_issuer` (`allowPrivateIssuer`) is enabled at OIDC-discovery startup, the system SHALL emit a warning log indicating that TLS certificate verification is disabled for OIDC discovery, so operators have runtime evidence that an insecure path is active. | P1 | A test or fixture confirms that constructing the OIDC discovery client with `allowPrivateIssuer = true` produces a WARN-level log mentioning disabled TLS verification; with the flag false, no such log is emitted and `InsecureSkipVerify` is never set. |

### 2.5 Integer-conversion safety (D4)

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-SEC3-050** | Optional (WHERE) | WHERE a signed integer is converted to `uint64` (`internal/index/index.go:101` `uint64(idx.embedder.Dimensions())`; `internal/index/dispatch.go:77` `uint64(maxRes)`), the code SHALL ensure the value is non-negative by a prior clamp or explicit guard, AND SHALL document the invariant with a `#nosec G115 -- <reason>` directive when a static non-negativity guarantee already exists (`Dimensions()` is statically > 0; `maxRes` is clamped to > 0 at `internal/index/dispatch.go:63-64`). The conversion result SHALL be unchanged for all valid inputs. | P1 | gosec reports no blocking G115 finding for the two lines (G115 is MEDIUM and does not fail the `-severity high` gate, but the directive documents the invariant for future audit-mode runs); a test confirms identical conversion output for representative non-negative inputs. |

### 2.6 TLS minimum version (D4)

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-SEC3-060** | Ubiquitous | The `tls.Config` used for outbound HTTPS fetches SHALL set `MinVersion` to at least `tls.VersionTLS12`. The existing `internal/access/phase4_tls.go:47` already sets `MinVersion: tls.VersionTLS12` (and `MaxVersion: tls.VersionTLS13`); this requirement SHALL be satisfied by preserving that line, NOT by adding redundant code. The semgrep `missing-ssl-minversion` finding on `phase4_tls.go:46` is a FALSE POSITIVE; IF it persists against the pinned ruleset, it SHALL be suppressed with a `// nosemgrep` directive that references the already-present `MinVersion` line. | P1 | grep confirms `phase4_tls.go` sets `MinVersion: tls.VersionTLS12`; no `MinVersion` line is added or removed; if semgrep still reports `missing-ssl-minversion`, a single `// nosemgrep` directive citing the present MinVersion is added and semgrep passes. |

---

## 3. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| **NFR-SEC-010** | Behavior preservation | All remediated files SHALL preserve their existing runtime behavior. Retry/backoff jitter SHALL remain within the same documented bounds; the OIDC discovery path SHALL behave identically when `allow_private_issuer` is unset; integer conversions SHALL yield identical results for all valid inputs. `go build ./...`, `go vet ./...`, and `go test ./...` SHALL pass with zero new failures. |
| **NFR-SEC-011** | Dual-gate validation | Every co-flagged line (flagged by BOTH gosec and semgrep) SHALL carry BOTH a gosec directive (`#nosec`) AND a semgrep directive (`// nosemgrep`), validated by actually running `gosec -severity high -confidence medium -conf .gosec.json ./...` AND `semgrep ci --config p/golang --config p/owasp-top-ten --config p/jwt` locally before claiming the gate is green. A single directive type SHALL NOT be assumed sufficient. |
| **NFR-SEC-012** | No gate-config drift | This SPEC SHALL NOT modify `.gosec.json`, `.semgrepignore`, or the `gosec`/`semgrep` job definitions in `.github/workflows/security.yml`. Remediation is confined to source-file annotations and (where cleaner) the randomness source. |

---

## 4. Exclusions (What NOT to Build)

[HARD] The following are explicitly out of scope. Each carries a rationale
or follow-up destination.

- **Changing `.gosec.json` / `.semgrepignore` / `security.yml` gate config.**
  → Out of scope (NFR-SEC-012). The gates are correct; only the call-site
  suppressions are wrong. Loosening the gate would mask future findings.

- **Migrating retry jitter to `crypto/rand`.** → Rejected by design.
  `math/rand` (or `math/rand/v2`) is the correct, faster source for
  non-security timing randomness. crypto/rand would add cost with no
  security benefit. The fix is a justification directive, not a crypto swap.

- **Code change at `internal/access/phase4_tls.go:46`.** → No fix. The
  adjacent line already sets `MinVersion: tls.VersionTLS12`; the semgrep
  `missing-ssl-minversion` finding is a FALSE POSITIVE (REQ-SEC3-060). At
  most a `// nosemgrep` directive is added; no `tls.Config` code changes.

- **Forcing a code change at `internal/obs/metrics/metrics.go:388`.**
  → needs-decision, default no-change. The shutdown goroutine's
  `context.Background()` is intentional: it is the graceful-shutdown path
  triggered BY parent `ctx.Done()`, and deriving from the cancelled ctx
  would defeat the 5-second drain window. Apply a `#nosec` or explanatory
  comment ONLY if a gate actually flags it; do NOT re-wire the context.

- **Hard-disabling `allow_private_issuer` in release builds.** → Reviewer
  policy decision (Blocker B2), deferred to run-phase / security review.
  This SPEC corrects the comment and adds a startup WARN (REQ-SEC3-041);
  whether to additionally compile it out is a separate security decision.

- **golangci-lint adoption / replacing standalone gosec with golangci.**
  → Out of scope. CI runs standalone gosec by deliberate SPEC-SEC-001
  choice. This SPEC aligns suppressions to that choice; it does not
  re-litigate the tool selection.

- **New security event types, metrics, or audit-log integration.**
  → Out of scope. This is annotation/comment remediation, not a feature.
  Security event taxonomy is owned by SPEC-SEC-001 REQ-SEC-017.

- **GitHub Issue tracking on this SPEC** (`issue_number: 0`). → CI-debt
  SPEC pattern; no tracking issue.

---

## 5. Acceptance Criteria

Headline acceptance for this CI-debt SPEC: **the `gosec` and `semgrep` jobs
in `.github/workflows/security.yml` pass clean on `main`** (no blocking
finding from either tool for the loci in this SPEC). Per-REQ acceptance
summaries are inline in §2. Scenario index:

| Scenario | Description | Coverage |
|----------|-------------|----------|
| §5.1 | Run `gosec -severity high -confidence medium -conf .gosec.json ./...` locally on `main` → exit 0; specifically no G402 on `internal/auth/discovery.go:47/52` and no G404 on the four jitter lines. | REQ-SEC3-010, REQ-SEC3-020, REQ-SEC3-030, REQ-SEC3-040, REQ-SEC3-050 |
| §5.2 | Run `semgrep ci --config p/golang --config p/owasp-top-ten --config p/jwt` locally on `main` → zero blocking findings; specifically no `math-random` on the jitter lines and no TLS-audit finding on `discovery.go:47/52`. | REQ-SEC3-010, REQ-SEC3-020, REQ-SEC3-040 |
| §5.3 | Jitter behavior characterization: `internal/synthesis/client.go`, `internal/embedder/client.go`, `internal/deepreport/client.go`, `internal/llm/retry.go` jitter values remain within their documented bounds after remediation (directive or `math/rand/v2`). | REQ-SEC3-020, NFR-SEC-010 |
| §5.4 | tokenizer/client.go v2 applicability: run pinned semgrep; if `math/rand/v2` at line 53 is flagged, `// nosemgrep` is present and semgrep passes; if not flagged, file unchanged and recorded not-applicable. | REQ-SEC3-021 |
| §5.5 | Effective gosec suppression: grep confirms no gate-blocking finding relies solely on `//nolint:gosec`; each intentional gosec suppression carries `#nosec G<NNN> -- <reason>`. | REQ-SEC3-030 |
| §5.6 | InsecureSkipVerify comment + gating: `discovery.go:47/52` carry `#nosec G402 -- ...` + `// nosemgrep`; the `// test-only` comment is replaced with an accurate config-gated statement; `InsecureSkipVerify` is set only when `allowPrivateIssuer` is true. | REQ-SEC3-040 |
| §5.7 | Startup WARN: constructing the OIDC discovery client with `allowPrivateIssuer = true` emits a WARN log mentioning disabled TLS verification; with false, no such log and no `InsecureSkipVerify`. | REQ-SEC3-041 |
| §5.8 | Integer-conversion guards: `internal/index/index.go:101` and `internal/index/dispatch.go:77` carry `#nosec G115 -- <reason>` documenting the static/clamped non-negativity invariant; conversion output identical for representative inputs. | REQ-SEC3-050 |
| §5.9 | TLS MinVersion preserved: `phase4_tls.go` still sets `MinVersion: tls.VersionTLS12`; no `tls.Config` code added/removed; if semgrep `missing-ssl-minversion` persists, a single `// nosemgrep` referencing the present MinVersion is added and semgrep passes. | REQ-SEC3-060 |
| §5.10 | Regression gate: `go build ./...`, `go vet ./...`, `go test ./...` pass with zero new failures across all remediated packages. | NFR-SEC-010 |
| §5.11 | Dual-gate proof: for every co-flagged line, both `#nosec` and `// nosemgrep` are present and BOTH tools were run locally to confirm green (no single-directive assumption). | NFR-SEC-011 |

### 5.1 Finding status map (run-phase checklist)

| Locus | Finding | Status | Required action |
|-------|---------|--------|-----------------|
| `internal/synthesis/client.go:271` | G404 / math-random (`math/rand`) | confirmed | `#nosec G404` + `// nosemgrep`, OR migrate to `math/rand/v2` + `// nosemgrep` |
| `internal/embedder/client.go:185` | G404 / math-random (`math/rand`) | confirmed | same as above |
| `internal/deepreport/client.go:210` | G404 / math-random (`math/rand`) | confirmed | same as above |
| `internal/llm/retry.go:56` | G404 / math-random; has `//nolint:gosec` only | **needs-decision** | replace ineffective `//nolint` with `#nosec G404` + `// nosemgrep`, OR `math/rand/v2` |
| `internal/index/tokenizer/client.go:53` | math-random; already `math/rand/v2` | **needs-decision** | add `// nosemgrep` ONLY if pinned semgrep flags v2; else not-applicable |
| `internal/auth/discovery.go:47` | G402 InsecureSkipVerify | confirmed | `#nosec G402` + `// nosemgrep` + fix `// test-only` comment + WARN log |
| `internal/auth/discovery.go:52` | G402 InsecureSkipVerify (nil-Transport) | confirmed | same as above |
| `internal/index/index.go:101` | G115 int→uint64 (`Dimensions()`) | confirmed | `#nosec G115 -- <reason>` documenting static >0 invariant |
| `internal/index/dispatch.go:77` | G115 int→uint64 (`maxRes`) | confirmed | `#nosec G115 -- <reason>` documenting clamp at lines 63-64 |
| `internal/access/phase4_tls.go:46` | missing-ssl-minversion | **false-positive** | NO code change; `// nosemgrep` only if it persists |
| `internal/obs/metrics/metrics.go:388` | shutdown `context.Background()` (call inside `go func()` block at lines 386-389) | **needs-decision** | NO code change by default; `#nosec`/comment only if a gate flags it |

---

## 6. Dependencies & Blocks

### 6.1 Upstream SPEC dependencies (depends_on)

- **SPEC-SEC-001 (implemented)** — owns the `security.yml` `gosec` +
  `semgrep` jobs (REQ-SEC-010) and the `.gosec.json` config. This SPEC
  remediates call sites so those gates pass clean; it does not change the
  gate configuration. Without SEC-001's gate definitions this SPEC has no
  target.

### 6.2 Related but soft (related)

- **SPEC-SEC-002** — sibling security-hardening SPEC. No shared files
  asserted; coordinate only if both modify `internal/auth/`.

### 6.3 Downstream blocked SPECs (blocks)

- None.

### 6.4 External dependencies (gate pins)

| Dependency | Pinned version | Source | Rule relevance |
|------------|---------------|--------|----------------|
| gosec | v2.27.1 | `.github/workflows/security.yml` (`go install github.com/securego/gosec/v2/cmd/gosec@v2.27.1`) | G404 / G402 / G115 rule behavior tied to this version |
| semgrep | 1.85.0 | `semgrep/semgrep:1.85.0` image | `math-random` / `missing-ssl-minversion` rule definitions ruleset-version-dependent; `math/rand/v2` match behavior must be verified against this version |
| semgrep rulesets | `p/golang`, `p/owasp-top-ten`, `p/jwt` | `semgrep ci --config ...` | the rulesets that produce the math-random / TLS findings |

---

## 7. Blockers (reviewer / run-phase decisions)

- **B1 — Dual-syntax suppression is non-optional.** gosec and semgrep use
  mutually incompatible inline-suppression syntaxes. A single `#nosec` will
  NOT satisfy semgrep; a single `// nosemgrep` will NOT satisfy gosec. Every
  co-flagged line needs BOTH, and both gates MUST be run locally to confirm
  green before claiming the CI gate passes (NFR-SEC-011).

- **B2 — `allow_private_issuer` production policy.** The `// test-only`
  comment on `discovery.go:47/52` is factually wrong: the path is
  config-gated and runtime-reachable whenever `allow_private_issuer: true`.
  A security reviewer must decide whether this is acceptable in production
  or should be hard-disabled in release builds (out of scope here — see
  §4 Exclusions). This SPEC corrects the comment and adds a startup WARN
  (REQ-SEC3-041) but does not make the compile-out decision.

---

## 8. Open Questions

1. **`math/rand/v2` vs directive (REQ-SEC3-020).** Migrate the three
   confirmed `math/rand` jitter sites to `math/rand/v2` (cleaner, matches
   `tokenizer/client.go`) or keep `math/rand` + dual directive? Run phase
   decides per-site; both satisfy the gates.

2. **tokenizer/client.go:53 applicability (REQ-SEC3-021).** Does
   `p/golang` @ semgrep 1.85.0 flag `math/rand/v2`? Must be confirmed by a
   local semgrep run; the answer determines whether any change is needed.

3. **metrics.go:388 (needs-decision).** Confirm whether either gate flags
   the shutdown `context.Background()`. If neither does, no change.

4. **phase4_tls.go:46 false-positive persistence.** Confirm whether the
   pinned semgrep ruleset still emits `missing-ssl-minversion` despite the
   present MinVersion; only then add `// nosemgrep`.

These do not block a plan-auditor PASS — they are known unresolved scope
edges with rationale, resolved during the run phase by running both gates.

---

## 9. References

Internal (project files — verified to exist):

- `.github/workflows/security.yml` (gosec @v2.27.1 + semgrep 1.85.0 jobs)
- `.gosec.json` (gosec config; unchanged by this SPEC)
- `.moai/specs/SPEC-SEC-001/spec.md` REQ-SEC-010 (gate definition)
- `internal/synthesis/client.go` (G404 jitter, line 271)
- `internal/embedder/client.go` (G404 jitter, line 185)
- `internal/deepreport/client.go` (G404 jitter, line 210)
- `internal/llm/retry.go` (G404 jitter + ineffective `//nolint`, line 56)
- `internal/index/tokenizer/client.go` (`math/rand/v2` jitter, line 53)
- `internal/auth/discovery.go` (G402 InsecureSkipVerify, lines 47, 52)
- `internal/index/index.go` (G115 int→uint64, line 101)
- `internal/index/dispatch.go` (G115 int→uint64, line 77; clamp at 63-64)
- `internal/access/phase4_tls.go` (TLS MinVersion present, lines 46-47)
- `internal/obs/metrics/metrics.go` (shutdown context, line 388; `go func()` block 386-389)

External:

- gosec #nosec directive: https://github.com/securego/gosec#annotating-code
- semgrep nosemgrep inline ignore: https://semgrep.dev/docs/ignoring-files-folders-code
- gosec G404 (insecure random): https://github.com/securego/gosec#available-rules
- gosec G402 (TLS InsecureSkipVerify): https://github.com/securego/gosec#available-rules
- gosec G115 (integer overflow conversion): https://github.com/securego/gosec#available-rules
- CWE-338 weak PRNG: https://cwe.mitre.org/data/definitions/338.html
- CWE-295 improper certificate validation: https://cwe.mitre.org/data/definitions/295.html

---

*End of SPEC-SEC-003 v0.1.0 (draft).*
