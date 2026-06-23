---
id: SPEC-SEC-005
version: 0.1.1
status: implemented
created: 2026-06-23
updated: 2026-06-24
author: limbowl
priority: P1
issue_number: 0
title: Container image CVE remediation — base-image digest pinning, koreanews non-root, storm uv pin, vuln-exceptions entries (trivy gate)
milestone: CI debt remediation
owner: expert-security
methodology: ddd
coverage_target: 0
depends_on: [SPEC-SEC-001]
blocks: []
related: [SPEC-DEP-001]
---

# SPEC-SEC-005: Container image CVE remediation (trivy)

## HISTORY

- 2026-06-23 (audit-fix v0.1.1, limbowl via manager-spec): plan-auditor
  remediation. (1) Fixed the false-green AC-1 / REQ-SEC5-010 grep — the old
  `^FROM python:3.11-slim$` anchor matched only koreanews's bare line and
  returned empty once koreanews was pinned, silently passing the AS-suffixed
  builder/base/runtime stages; replaced with a scoped, suffix-aware regex
  plus an `@sha256:` digest assertion. (2) Corrected line numbers: embedder
  has a SINGLE base FROM (line 5; line 31 is a RUN), and storm's base FROMs
  are lines 5 and 20 with line 10 being the `COPY --from=ghcr.io/astral-sh/uv`
  external image (now a separate item, not a base FROM). (3) Relabelled EARS
  patterns (WHERE → Optional; REQ-SEC5-070 → Event-Driven "When remediating").
  (4) Namespaced REQ ids REQ-SEC-0x0 → REQ-SEC5-0x0 to avoid collision with
  SPEC-SEC-001 / SPEC-SEC-003 across the requirements table, EARS notes, and
  acceptance refs.

- 2026-06-23 (initial draft v0.1.0, limbowl via manager-spec):
  Remediation SPEC for EXISTING CI debt. The `trivy-config` and
  `trivy-image` jobs in `.github/workflows/security.yml` (introduced by
  SPEC-SEC-001 REQ-SEC-001) currently block merges because the five Python
  sidecar service Dockerfiles (`services/{tokenizer-ko,researcher,koreanews,
  embedder,storm}/Dockerfile`) pin their base image `python:3.11-slim` by
  TAG only. trivy resolves the tag at scan time to the current Debian
  bookworm + CPython OS-package layer, which accumulates FIXED CRITICAL/HIGH
  OS-package CVEs over time. With `ignore-unfixed: true`, the only image
  findings that block merge are FIXED ones, so a base-image digest re-pin
  clears the bulk of them. Two independent IaC (`trivy-config`) failures
  also exist: `koreanews` runs as root (no `USER` directive), and `storm`
  references the unpinned external image `ghcr.io/astral-sh/uv:latest`.

  This SPEC INVENTS NO new security infrastructure. The remediation
  scaffolding already exists and is correctly wired: `ops/security/
  vuln-exceptions.yaml` (schema + 90-day contract, currently `exceptions:
  []`) and `scripts/check-vuln-exceptions.sh` (executable deadline
  enforcer with PyYAML-optional parser + `VULN_EXCEPTIONS_NOW` override),
  both consumed by the `vuln-exceptions (deadline check)` job in
  `security.yml`. No change is required to the script or the workflow jobs —
  they are confirmed already-correct. The remaining work is: (1) digest-pin
  the base image in all five Dockerfiles, (2) add `USER` to koreanews, (3)
  digest-pin storm's `uv` COPY source, and (4) record concrete
  `vuln-exceptions.yaml` entries for the residual UNFIXED CRITICAL/HIGH
  app-layer CVEs (torch/transformers in embedder, knowledge-storm/dspy-ai
  in storm).

  Methodology: DDD (remediate existing Dockerfiles + config without altering
  service runtime behavior). Coverage target N/A (no new Go/Python product
  code; verification is via the trivy CI gate itself). Owner: expert-security.

  Live-state note: the trivy steps currently pin `aquasecurity/trivy-action@
  v0.33.1` (verified in `security.yml`, not the `@0.24.0` referenced in
  SPEC-SEC-001 §REQ-SEC-001). This SPEC does not modify the action version.

---

## 1. Overview

SPEC-SEC-005 remediates EXISTING CI debt: the container-image and
Dockerfile-config security gates added by SPEC-SEC-001 are red on `main`
for the five Python sidecar services. The fix is mechanical and
infrastructure-level — re-pin base images by immutable digest, close the
koreanews run-as-root IaC finding, pin storm's external `uv` image, and
record any genuinely-UNFIXED app-layer CVEs as time-boxed exceptions. No
service runtime behavior changes.

### 1.1 What ships

| Layer | Artifact | Change |
|-------|----------|--------|
| Build | `services/tokenizer-ko/Dockerfile` (lines 8, 31) | repin both `FROM python:3.11-slim` stages to a digest |
| Build | `services/researcher/Dockerfile` (line 4) | repin `FROM python:3.11-slim AS base` to a digest |
| Build | `services/koreanews/Dockerfile` (line 1) | repin `FROM python:3.11-slim` to a digest + add non-root `USER` |
| Build | `services/embedder/Dockerfile` (line 5) | repin the single `FROM python:3.11-slim AS base` to a digest |
| Build | `services/storm/Dockerfile` (lines 5, 20) | repin both `FROM python:3.11-slim` stages (builder + runtime) to a digest |
| Build | `services/storm/Dockerfile` (line 10) | digest-pin the `COPY --from=ghcr.io/astral-sh/uv` external image (separate from the base FROMs) |
| Config | `ops/security/vuln-exceptions.yaml` (exists, empty) | add concrete entries for residual UNFIXED CRITICAL/HIGH app-layer CVEs |

### 1.2 What is already done (do NOT rebuild)

- `ops/security/vuln-exceptions.yaml` EXISTS with the documented schema
  (`cve_id` / `dependency` / `severity` / `rationale` / `discovered_at` /
  `review_deadline` / `owner` / `fixed_at`) and a 90-day deadline contract.
  Currently `exceptions: []`. Only entries need adding — the file and its
  schema are structurally complete (status: already-fixed).
- `scripts/check-vuln-exceptions.sh` EXISTS, is executable, validates the
  schema, enforces the 90-day cap (`review_deadline <= discovered_at + 90
  days`), fails CI on a passed `review_deadline`, supports
  `VULN_EXCEPTIONS_NOW` override and a PyYAML-optional minimal parser
  (status: already-fixed).
- The `trivy-config`, `trivy-image` (matrix: researcher, storm, embedder,
  tokenizer-ko, koreanews; `severity: CRITICAL,HIGH`; `exit-code: 1`;
  image scan `ignore-unfixed: true`), and `vuln-exceptions (deadline
  check)` jobs in `.github/workflows/security.yml` are correct and require
  NO change.

### 1.3 Root cause (verified)

The blocking trivy failures originate primarily from the shared base image:
all five Dockerfiles use `python:3.11-slim` pinned by TAG only (no digest).
trivy resolves the tag at scan time to the current Debian bookworm + CPython
layer, whose OS packages accumulate FIXED CRITICAL/HIGH CVEs over time; with
`ignore-unfixed: true` the only image findings that block merge are FIXED
ones, so a base-image digest bump to a freshly-patched layer clears the bulk
of them. A second, independent source is app-layer HIGH CVEs in heavy ML
stacks — embedder (torch/transformers via `FlagEmbedding>=1.3.0`) and storm
(`knowledge-storm==1.1.1` + `dspy-ai`) — many of which are UNFIXED upstream
and therefore informational under `ignore-unfixed`, requiring
`vuln-exceptions.yaml` entries rather than bumps. The `trivy-config` (IaC)
gate is failed separately by koreanews running as root (no `USER`) and storm
referencing the unpinned `ghcr.io/astral-sh/uv:latest` external image.

### 1.4 Findings requiring a decision (flagged)

Two findings carry a fix-vs-suppress decision that the run phase must resolve
WITH evidence (see Blockers §6 — both depend on a live `trivy image` run):

- **Residual UNFIXED CRITICAL/HIGH in embedder (torch/transformers) and
  storm (knowledge-storm 1.1.1 / dspy-ai)** — DECISION (suppress-with-
  justification vs bump): suppress via a `vuln-exceptions.yaml` entry ONLY
  when the CVE is confirmed UNFIXED upstream (no patched wheel / no
  compatible release exists). If a fixed version exists, bump the dependency
  instead. Each suppression MUST carry a `rationale` and a `review_deadline
  <= discovered_at + 90d`.
- **No false-positive findings are asserted in scope.** This SPEC does not
  pre-classify any CVE as a false positive; the FIXED-vs-UNFIXED triage is
  determined empirically by the run-phase `trivy image` execution.

---

## 2. EARS Requirements

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-SEC5-010** | Ubiquitous | The container build SHALL pin every service base image (every `FROM python:3.11-slim` line across `services/{tokenizer-ko,researcher,koreanews,embedder,storm}/Dockerfile`, including each multi-stage builder and runtime stage) by immutable digest (`python:3.11-slim@sha256:...`) rather than by mutable tag. | P1 | `grep -rnE "^FROM python:3\.11-slim($\| AS )" services/{tokenizer-ko,researcher,koreanews,embedder,storm}/Dockerfile` returns zero matches (catches both the bare `koreanews` form and the `AS builder/base/runtime` suffixed stages), AND every base `FROM` carries an `@sha256:` digest. |
| **REQ-SEC5-020** | Optional | WHERE a service Dockerfile defines a runtime stage that lacks a non-root `USER` directive, the build SHALL add an explicit non-root user (`RUN useradd ... appuser` + `USER appuser` before `CMD`), closing the koreanews run-as-root IaC finding by mirroring the pattern already present in researcher/embedder/storm/tokenizer-ko. | P1 | `grep -n "USER " services/koreanews/Dockerfile` returns a non-root `USER` line; `trivy-config` no longer flags DS002 (run-as-root) for koreanews. |
| **REQ-SEC5-030** | Optional | WHERE a Dockerfile copies content from an external image (e.g. `COPY --from=ghcr.io/astral-sh/uv`), the build SHALL pin that external image by digest and SHALL NOT reference the `:latest` tag. | P1 | `grep -n "astral-sh/uv" services/storm/Dockerfile` shows an `@sha256:` digest and no `:latest`. |
| **REQ-SEC5-040** | Event-Driven | WHEN the `trivy-image` scan reports a FIXED CRITICAL or HIGH (CVSS >= 7.0) CVE for any of {tokenizer-ko, researcher, koreanews, embedder, storm}, the build SHALL block the merge until the affected base image or dependency is upgraded. | P1 | A deliberately stale base-image pin produces a FIXED CRITICAL/HIGH and a red `trivy-image` job; after the digest bump the job is green. |
| **REQ-SEC5-050** | Conditional (IF-THEN) | IF a CRITICAL or HIGH CVE has no available upstream fix (UNFIXED), THEN the build SHALL record it in `ops/security/vuln-exceptions.yaml` with `cve_id`, affected `dependency`, `severity`, `rationale`, `discovered_at`, an accountable `owner`, and a `review_deadline` no later than `discovered_at + 90 days`. | P1 | `vuln-exceptions.yaml` contains one well-formed entry per residual UNFIXED CRITICAL/HIGH; `scripts/check-vuln-exceptions.sh ops/security/vuln-exceptions.yaml` exits 0. |
| **REQ-SEC5-060** | Event-Driven | WHEN any exception's `review_deadline` has passed without renewal, the `vuln-exceptions (deadline check)` CI job SHALL fail the build. | P1 | Running `scripts/check-vuln-exceptions.sh` with `VULN_EXCEPTIONS_NOW` advanced past an entry's `review_deadline` exits non-zero; with a current date it exits 0. (Enforcement already implemented — verified, not re-built.) |
| **REQ-SEC5-070** | Event-Driven | WHEN remediating embedder image CVEs, the build SHALL distinguish OS-package CVEs (resolved by a base-image digest bump) from torch/transformers app-layer CVEs (recorded as `vuln-exceptions.yaml` entries only when confirmed UNFIXED upstream). The build SHALL NOT suppress an OS-package CVE that a base-image bump would fix. | P1 | The embedder remediation diff shows OS-package CVEs cleared by the base-image bump and only confirmed-UNFIXED torch/transformers CVEs present as exceptions; no FIXED OS-package CVE appears in `vuln-exceptions.yaml`. |

EARS label notes: REQ-SEC5-010 is Ubiquitous (SHALL always hold);
REQ-SEC5-020/030 are Optional (WHERE a feature/condition exists);
REQ-SEC5-040/060/070 are Event-Driven (WHEN a scan/deadline/remediation event
occurs); REQ-SEC5-050 is Conditional (IF UNFIXED THEN record). No requirement
uses a double negative; each is independently testable via grep, the trivy
gate, or the deadline script.

---

## 3. Scope

### 3.1 In scope

- Digest-pinning `python:3.11-slim` across all five service Dockerfiles
  (every `FROM` line, including multi-stage builder + runtime stages).
- Adding a non-root `USER` directive to `services/koreanews/Dockerfile`.
- Digest-pinning `ghcr.io/astral-sh/uv` in `services/storm/Dockerfile`.
- Adding concrete `vuln-exceptions.yaml` entries for residual UNFIXED
  CRITICAL/HIGH app-layer CVEs (embedder torch/transformers; storm
  knowledge-storm/dspy-ai).
- Re-scanning per service to confirm the trivy gate passes on `main`.

### 3.2 Out of scope (What NOT to Build)

[HARD] The following are explicitly excluded.

- **Changes to `scripts/check-vuln-exceptions.sh`**. → Already-fixed: the
  deadline enforcer is complete, executable, and correctly wired. No change.
- **Changes to the `trivy-config` / `trivy-image` / `vuln-exceptions` jobs
  in `.github/workflows/security.yml`**. → Already-correct (matrix, severity,
  `exit-code`, `ignore-unfixed`, deadline job all confirmed). No change.
- **Creating or re-schema-ing `ops/security/vuln-exceptions.yaml`**. → The
  file and its schema already exist; only entries are added.
- **Upgrading torch / transformers / knowledge-storm / dspy-ai to chase
  UNFIXED CVEs**. → Gated on upstream releases (see §6 External deps);
  UNFIXED CVEs are recorded as exceptions, not force-bumped.
- **Migrating off `python:3.11-slim` to a different base distro (alpine,
  distroless, chainguard)**. → Out of scope; remediation is a digest re-pin
  of the same official image, not a base-image redesign.
- **Go service images** (`usearch-api`, `mcp`, `migrate`). → The trivy
  matrix targets only the five Python sidecars; Go images are not in this
  gate's failing set.
- **Trivy action version changes**. → The live `@v0.33.1` pin is unchanged.
- **GitHub Issue tracking on this SPEC** (`issue_number: 0`). → CI-debt
  remediation pattern.
- **False-positive suppressions**. → No finding is pre-classified as a false
  positive; FIXED-vs-UNFIXED triage is empirical (run-phase trivy run).

---

## 4. Acceptance Criteria

Headline acceptance: **the `trivy-config`, `trivy-image` (all five matrix
services), and `vuln-exceptions (deadline check)` jobs in `security.yml`
pass on `main`.**

| Scenario | Description | Coverage |
|----------|-------------|----------|
| AC-1 | No tag-only `FROM python:3.11-slim` line remains, including AS-suffixed stages: `grep -rnE "^FROM python:3\.11-slim($\| AS )" services/{tokenizer-ko,researcher,koreanews,embedder,storm}/Dockerfile` is empty; every base `FROM` carries an `@sha256:` digest. (The earlier `^FROM python:3.11-slim$` form was false-green — it matched only koreanews's bare line and returned empty once koreanews was pinned, missing the `AS builder/base/runtime` stages.) | REQ-SEC5-010 |
| AC-2 | `services/koreanews/Dockerfile` has a non-root `USER` directive before `CMD`; `trivy-config` no longer reports run-as-root (DS002) for koreanews. | REQ-SEC5-020 |
| AC-3 | `services/storm/Dockerfile` `COPY --from=ghcr.io/astral-sh/uv` references an `@sha256:` digest, not `:latest`; `trivy-config` IaC gate is green. | REQ-SEC5-030 |
| AC-4 | `trivy-image` matrix is green for all five services after the digest bumps — no FIXED CRITICAL/HIGH remains. A stale-pin negative control reproduces a red job; the digest bump turns it green. | REQ-SEC5-040 |
| AC-5 | Each residual UNFIXED CRITICAL/HIGH (embedder torch/transformers; storm knowledge-storm/dspy-ai) has a well-formed `vuln-exceptions.yaml` entry; `scripts/check-vuln-exceptions.sh ops/security/vuln-exceptions.yaml` exits 0 with a current date. | REQ-SEC5-050, REQ-SEC5-070 |
| AC-6 | Deadline enforcement: running the script with `VULN_EXCEPTIONS_NOW` advanced past an entry's `review_deadline` exits non-zero; with the current date it exits 0. | REQ-SEC5-060 |
| AC-7 | OS-vs-app separation for embedder: the remediation diff shows OS-package CVEs cleared by the base-image bump (not suppressed), and only confirmed-UNFIXED torch/transformers CVEs as exceptions. | REQ-SEC5-070 |

Edge cases:

- **EC-1**: A chosen replacement digest may itself accrue new CVEs over time.
  Acceptance is evaluated against the digest selected at fix time; a future
  drift is handled by a subsequent re-pin, not by this SPEC.
- **EC-2**: If a residual CRITICAL/HIGH turns out to be FIXED (a patched
  wheel exists), it MUST be bumped, not suppressed (REQ-SEC5-070 / §1.4
  decision).

---

## 5. Dependencies & Blocks

### 5.1 Upstream (depends_on)

- **SPEC-SEC-001 (implemented)** — owns `.github/workflows/security.yml`
  including the `trivy-config` / `trivy-image` jobs (REQ-SEC-001) and the
  `vuln-exceptions` deadline job that this SPEC's fixes are verified against.
  This SPEC remediates the inputs to that gate; it does not modify the gate.

### 5.2 Related (related)

- **SPEC-DEP-001 (implemented)** — dependency severity-policy precedent
  (CRITICAL/HIGH block, UNFIXED informational). REQ-SEC5-050's exception
  contract builds on the same severity model.

### 5.3 Blocks

- None.

---

## 6. Blockers & External Dependencies

### 6.1 Blockers (run-phase, require a live environment)

- **Cannot enumerate the actual trivy CRITICAL/HIGH CVE list from static
  source alone.** Determining which CVEs are FIXED (blocking — needs a bump)
  vs UNFIXED (informational — needs an exception) requires running `trivy
  image` against the built image of each of the five services, which depends
  on Docker + network image pulls not available during read-only SPEC
  authoring.
- **The specific replacement `python:3.11-slim` digest must be chosen against
  the current registry state at fix time** (a digest valid today may itself
  accrue CVEs). Requires a live `docker pull` + re-scan loop.

### 6.2 External dependencies (upstream-gated)

- **torch / transformers** (via `FlagEmbedding>=1.3.0` in embedder): some
  HIGH CVEs are UNFIXED upstream — a bump is possible only after PyTorch
  publishes a patched CPU-index wheel; until then a `vuln-exceptions.yaml`
  entry is required.
- **knowledge-storm==1.1.1 + dspy-ai** (storm): pinned old; transitive CVE
  fixes are gated on upstream releases compatible with the `knowledge-storm
  1.1.1` pin.
- **python:3.11-slim base image**: FIXED OS-package CVEs clear only once the
  Python Docker official-image team publishes a refreshed Debian-patched
  layer upstream.

---

## 7. References

Internal (verified):

- `.github/workflows/security.yml` (trivy-config / trivy-image matrix /
  vuln-exceptions deadline job — the live gate this SPEC targets)
- `ops/security/vuln-exceptions.yaml` (exists; schema + 90-day contract;
  `exceptions: []`)
- `scripts/check-vuln-exceptions.sh` (exists; executable deadline enforcer)
- `services/tokenizer-ko/Dockerfile` (FROM lines 8, 31)
- `services/researcher/Dockerfile` (FROM line 4; openai/deepeval deps)
- `services/koreanews/Dockerfile` (FROM line 1; no USER — runs as root)
- `services/embedder/Dockerfile` (single base FROM line 5; line 31 is a RUN pip install, not a base FROM; torch + FlagEmbedding)
- `services/storm/Dockerfile` (base FROM lines 5, 20; line 10 is the unpinned `COPY --from=ghcr.io/astral-sh/uv:latest`, not a base FROM; knowledge-storm 1.1.1 + dspy-ai)
- `.moai/specs/SPEC-SEC-001/spec.md` REQ-SEC-001/002/003 (the originating
  trivy + vuln-exceptions requirements)

---

*End of SPEC-SEC-005 v0.1.1 (draft).*
