---
id: SPEC-DEP-002
version: 0.1.0
status: draft
created: 2026-06-23
updated: 2026-06-23
author: limbowl
priority: P1
issue_number: 0
title: Python dependency CVE remediation (pip-audit) + license-scan CI debt clearance
milestone: post-V1 CI debt
owner: expert-devops
methodology: ddd
coverage_target: 0
depends_on: [SPEC-DEP-001]
blocks: []
related: [SPEC-SEC-001]
---

# SPEC-DEP-002: Python dependency CVE remediation (pip-audit) + license-scan

## HISTORY

- 2026-06-23 (initial draft v0.1.0, limbowl via manager-spec):
  Existing CI debt remediation, NOT a new feature. The `deps-audit.yml`
  pip-audit job (storm/embedder/researcher Python sidecars) is red on
  `main`; the license-scan job is **asserted-red but not statically
  verifiable** (its `docs/licenses/*.txt` inputs are CI-generated, only
  `.gitkeep` is committed) and must be confirmed via the B1 reproduction
  (§6.3) before remediation is scoped — the sole known AGPL violator
  (`searxng`) is already in `EXCEPTIONS`, so a real violation would be a
  NEW dependency. This SPEC closes the newly-surfaced batch of transitive
  Python CVEs that the existing `--ignore-vuln` allowlist — last curated
  for an earlier CVE set (pip/lxml/diskcache/joblib/litellm/transformers) —
  does not cover, and conditionally resolves any confirmed license-scan
  violation. The `--ignore-vuln` mechanism and inline-rationale convention
  are owned by SPEC-DEP-001 (REQ-DEP-004, PR #29). Grounded entirely in a
  verified investigation of `services/storm/uv.lock`,
  `.github/workflows/deps-audit.yml`, and `scripts/check-license-allowlist.sh`.
  Two findings carry status `needs-decision` (torch CVE-2025-3000 with no
  upstream fix; the license-scan violating dependency, which cannot be
  identified statically). DDD methodology: ANALYZE the failing jobs →
  PRESERVE the existing ignore-list/allowlist conventions → IMPROVE by
  bumping where possible and documenting suppressions where upstream blocks.

---

## 1. Overview

SPEC-DEP-002 clears the remaining **safe-group CI debt** in the
`deps-audit.yml` workflow: the `pip-audit` job (Python sidecars), which is
red on `main`, and the `license-scan` job, which is **asserted-red but not
statically verifiable** (its inputs `docs/licenses/*.txt` are CI-generated;
only `docs/licenses/.gitkeep` is committed) — its red status must be
confirmed via the B1 reproduction (§6.3) before any license remediation is
scoped. The one known disallowed dependency, `searxng` (AGPL-3.0), is
already covered by the `EXCEPTIONS` service-boundary entry, so a confirmed
new violation would necessarily originate from a NEW dependency. This is
remediation of EXISTING dependency debt, not new functionality. No
application code or adapter behavior changes.

### 1.1 Root cause (verified)

Transitive Python dependencies in the storm / embedder / researcher
services resolve to versions carrying newly-disclosed CVEs. The
`pip-audit` `--ignore-vuln` allowlist was last curated for an earlier CVE
set and has not been extended to the newer batch (aiohttp ×11 CVEs,
starlette, langsmith, msgpack, ujson, torch). Several vulnerable pins are
forced by upstream constraints — `knowledge-storm==1.1.1` pins
`litellm==1.80.0` (which pins `aiohttp 3.13.5`), and `fastapi` pins
`starlette` — so a clean in-repo bump is blocked until upstream releases
compatible versions. Only `services/storm/uv.lock` is committed;
embedder/researcher have only `pyproject.toml` (no `uv.lock`), so their
exact failing transitive versions are resolved fresh at CI time and are
non-deterministic. Separately, `license-scan` runs a blocklist regex
against CI-generated `docs/licenses/*.txt` (only `docs/licenses/.gitkeep`
is committed, 0 bytes), so the specific GPL/AGPL/LGPL/SSPL-violating
dependency cannot be confirmed from the static repo.

### 1.2 Confirmed findings

| # | Locus | Issue | Status |
|---|-------|-------|--------|
| F1 | `services/storm/uv.lock` (aiohttp) | aiohttp pinned `3.13.5` → finding `3.14.1` (11 CVEs); transitive via litellm; not in `--ignore-vuln` | confirmed |
| F2 | `services/storm/uv.lock` (starlette) | starlette pinned `1.1.0` → finding `1.3.1`; via fastapi 0.136.1; not ignored | confirmed |
| F3 | `services/storm/uv.lock` (litellm) | litellm pinned `1.80.0` → finding `1.84.0`; pinned by knowledge-storm==1.1.1; some CVEs already ignored, 1.84.0-only CVEs may be newly surfaced | confirmed |
| F4 | `services/storm/uv.lock` (langsmith) | langsmith pinned `0.8.5` → finding `0.8.18`; via langchain tree; not ignored | confirmed |
| F5 | `services/storm/uv.lock` (msgpack) | msgpack pinned `1.1.2` → finding `1.2.1`; not ignored | confirmed |
| F6 | `services/storm/uv.lock` (ujson) | ujson pinned `5.12.1` → finding `5.13.0`; not ignored | confirmed |
| F7 | `services/storm/uv.lock` (torch) | torch pinned `2.12.0`; CVE-2025-3000 fix TBD; no upstream fix version | **needs-decision** |
| F8 | `.github/workflows/deps-audit.yml` `--ignore-vuln` | Existing list covers pip/lxml/diskcache/joblib/litellm/transformers (11 ignores); does NOT cover aiohttp/starlette/langsmith/msgpack/ujson/torch | confirmed |
| F9 | `scripts/check-license-allowlist.sh` `DISALLOWED` | Blocklist regex (GPL/AGPL/LGPL/SSPL/UNLICENSED/Proprietary/Commercial); violating dep cannot be statically identified — `docs/licenses/*.txt` generated at CI time, only `.gitkeep` committed | **needs-decision** |
| F10 | `services/embedder/pyproject.toml` + `services/researcher/pyproject.toml` | No `uv.lock` committed (only storm has one); transitive vulnerable versions resolved fresh at CI time; embedder pulls torch/transformers + starlette, researcher pulls litellm→aiohttp + langsmith + starlette | confirmed |

### 1.3 needs-decision flags (suppress-with-justification vs fix)

- **F7 (torch CVE-2025-3000)** — fix-vs-suppress is **suppress** for V1:
  no released upstream torch fix exists. Decision is to record an accepted,
  upstream-tracked exception, NOT a version bump (a bump target does not
  exist yet).
- **F9 (license-scan violation)** — fix-vs-suppress is **UNDECIDED until
  reproduced**: the offending dependency cannot be named from the static
  repo. The run phase MUST first reproduce the license-scan job locally to
  surface the package, then choose between replacing the dependency or
  recording a justified service-boundary `EXCEPTIONS` entry.

### 1.4 Pinned remediation strategy

Prefer real version bumps over `--ignore-vuln`. Only suppress where an
upstream pin blocks the fix, with an inline rationale comment naming the
blocking upstream package and the target fixed version.

| Package | Blocked by upstream? | Action |
|---------|---------------------|--------|
| aiohttp (F1) | YES — knowledge-storm==1.1.1 → litellm==1.80.0 → aiohttp 3.13.5 | suppress with rationale |
| litellm (F3) | YES — knowledge-storm==1.1.1 | suppress with rationale (where 1.84.0-only) |
| starlette (F2) | YES — fastapi 0.136.1 → starlette 1.1.0 | suppress with rationale |
| msgpack (F5) | NO | bump via `uv lock --upgrade-package msgpack` |
| ujson (F6) | NO | bump via `uv lock --upgrade-package ujson` |
| langsmith (F4) | NO (langchain tree, not hard-pinned) | bump via `uv lock --upgrade-package langsmith` |
| torch (F7) | YES — no upstream fix released | accepted exception, upstream-tracking note |

---

## 2. Scope

### 2.1 In scope

- Extend `.github/workflows/deps-audit.yml` `pip-audit` `--ignore-vuln`
  list (with per-CVE rationale comments matching the existing block style)
  for transitive CVEs blocked by upstream pins.
- Upgrade clean-bumpable transitive pins in `services/storm/uv.lock`
  (msgpack, ujson, langsmith) via `uv lock --upgrade-package`.
- Commit `uv.lock` for `services/embedder` and `services/researcher` so
  transitive versions are deterministic and auditable.
- Resolve the license-scan failure: reproduce locally, identify the
  violating dependency, then replace it or record a documented
  service-boundary `EXCEPTIONS` entry in `scripts/check-license-allowlist.sh`.
- Record the torch CVE-2025-3000 accepted exception with an
  upstream-tracking reference.

### 2.2 Out of scope

- Any application / adapter / synthesis code change.
- Go (`govulncheck`), Next.js (`pnpm audit`), `hadolint`, `trivy`, or
  `searxng-digest-check` jobs — those are not in the failing set this SPEC
  addresses (Go safe-group already cleared per PR #58/#59).
- Replacing `knowledge-storm`, `litellm`, or `fastapi` with alternative
  libraries (upstream-blocked; tracked as external dependencies).
- Converting the license-scan blocklist into an allowlist model.
- Vendoring or forking any upstream package.

---

## 3. EARS Requirements

REQ ids use the `DEP2` namespace (numbered 10/20/30...) to avoid collision
with the `REQ-DEP-*` ids owned by SPEC-DEP-001/003/004.

| ID | Pattern | Requirement | Priority |
|----|---------|-------------|----------|
| **REQ-DEP2-010** | Ubiquitous | The `pip-audit` job in `.github/workflows/deps-audit.yml` SHALL pass on `main` for services `storm`, `embedder`, and `researcher` with zero unresolved HIGH-or-above-severity Python CVE findings. | P1 |
| **REQ-DEP2-020** | Conditional (IF-THEN) | IF a transitive dependency CVE has an available patched version that is NOT blocked by an upstream pin, THEN the system SHALL upgrade the pinned version in the affected service `uv.lock` (via `uv lock --upgrade-package <pkg>`) rather than adding the CVE to `--ignore-vuln`. This SHALL apply at minimum to `msgpack`, `ujson`, and `langsmith` in `services/storm/uv.lock`. | P1 |
| **REQ-DEP2-030** | Conditional (IF-THEN) | IF a vulnerable transitive version is forced by an upstream pin (`knowledge-storm==1.1.1`, `litellm==1.80.0`, or `fastapi`→`starlette`), THEN the `deps-audit.yml` `pip-audit` job SHALL list the CVE in `--ignore-vuln` with an inline rationale comment naming the blocking upstream package AND the target fixed version. | P1 |
| **REQ-DEP2-040** | Ubiquitous | The `deps-audit.yml` `pip-audit` job SHALL address (via upgrade per REQ-DEP2-020 or suppression per REQ-DEP2-030) the currently-unaddressed CVEs for `aiohttp`, `starlette`, `litellm`, `langsmith`, `msgpack`, `ujson`, and `torch`. | P1 |
| **REQ-DEP2-050** | Conditional (IF-THEN) | IF `torch` CVE-2025-3000 has no released upstream fix, THEN the system SHALL record it as an accepted exception in the `--ignore-vuln` list with an inline upstream-tracking reference, retained until a fixed `torch` release is available. | P1 |
| **REQ-DEP2-060** | Ubiquitous | The services `embedder` and `researcher` SHALL each have a committed `uv.lock` so their transitive dependency versions are deterministic and auditable in CI. | P1 |
| **REQ-DEP2-070** | Conditional (IF-THEN) | IF the B1 license-scan reproduction (§6.3) surfaces a dependency reporting a `DISALLOWED`-matching license outside the documented `EXCEPTIONS` list, THEN the `license-scan` job in `.github/workflows/deps-audit.yml` SHALL be made to pass on `main` by resolving that dependency. (The only known disallowed dependency — `searxng` AGPL-3.0 — is already covered by the `EXCEPTIONS` service-boundary entry; a fresh violation would therefore originate from a NEW dependency.) | P1 |
| **REQ-DEP2-080** | Conditional (IF-THEN) | IF the B1 reproduction confirms a dependency whose license matches the `DISALLOWED` blocklist regex in `scripts/check-license-allowlist.sh` and which is NOT already in `EXCEPTIONS`, THEN the system SHALL either replace the dependency OR record a justified service-boundary exception in `scripts/check-license-allowlist.sh` `EXCEPTIONS`. | P1 |

All requirements are testable: each maps to a CI job outcome on `main` or
to a verifiable artifact (committed `uv.lock`, an `--ignore-vuln` entry
with a rationale comment, or an `EXCEPTIONS` entry). No double negatives.
REQ-DEP2-070/080 are conditional on the B1 reproduction (§6.3): they only
prescribe work IF a fresh disallowed-license dependency is surfaced.

---

## 4. Acceptance Criteria

Headline acceptance: **the named CI checks pass on `main`.**

| AC | Given-When-Then | REQ |
|----|-----------------|-----|
| **AC-1** | GIVEN the `deps-audit.yml` workflow on `main`, WHEN the `pip-audit` job runs for the storm / embedder / researcher matrix, THEN it exits 0 with zero unresolved HIGH+ Python CVEs. | REQ-DEP2-010, REQ-DEP2-040 |
| **AC-2** | GIVEN `services/storm/uv.lock`, WHEN inspected after remediation, THEN `msgpack`, `ujson`, and `langsmith` are pinned at non-vulnerable versions (off `1.1.2` / `5.12.1` / `0.8.5` respectively) via lockfile upgrade, NOT via `--ignore-vuln`. | REQ-DEP2-020 |
| **AC-3** | GIVEN the `--ignore-vuln` list, WHEN it contains an aiohttp/starlette/litellm/torch CVE entry, THEN each such entry has an inline comment naming the blocking upstream package and the target fixed version. | REQ-DEP2-030, REQ-DEP2-050 |
| **AC-4** | GIVEN the repository, WHEN `services/embedder` and `services/researcher` are inspected, THEN each contains a committed `uv.lock`. | REQ-DEP2-060 |
| **AC-5** | GIVEN the B1 license-scan reproduction (§6.3) on `main`, WHEN the `license-scan` job runs, THEN it exits 0 with zero disallowed-license dependencies outside the documented `EXCEPTIONS` list (which already covers `searxng` AGPL-3.0). | REQ-DEP2-070 |
| **AC-6** | GIVEN the locally-reproduced license-scan output (B1), WHEN a `DISALLOWED`-matching dependency NOT already in `EXCEPTIONS` is found, THEN it is either replaced OR has a justified `EXCEPTIONS` entry in `scripts/check-license-allowlist.sh` with rationale. | REQ-DEP2-080 |
| **AC-7** | GIVEN the torch CVE-2025-3000 entry, WHEN inspected, THEN it carries an upstream-tracking reference and is documented as an accepted exception pending a fixed torch release. | REQ-DEP2-050 |

---

## 5. Out-of-scope (What NOT to Build)

[HARD] The following are explicitly excluded:

- **Replacing knowledge-storm / litellm / fastapi** with alternatives →
  upstream-blocked; tracked as external dependencies (§6.2). A simple
  in-repo bump of aiohttp/litellm/starlette is impossible until upstream
  loosens its pins.
- **Application / adapter / synthesis logic changes** → this is dependency
  and CI-config debt only.
- **Go / Next.js / hadolint / trivy / searxng-digest jobs** → not in this
  SPEC's failing set (Go safe-group cleared in PR #58/#59).
- **Blocklist → allowlist migration for license-scan** → the existing
  blocklist model in `scripts/check-license-allowlist.sh` is preserved.
- **New security tooling, scanners, or workflows** → SPEC-SEC-001 owns the
  security tool surface; this SPEC only remediates existing `deps-audit.yml`
  job failures.

---

## 6. Dependencies & Blockers

### 6.1 Upstream SPEC dependencies

- **SPEC-DEP-001 (implemented)** — owns `deps-audit.yml` and the
  dependency severity policy. This SPEC extends the existing `--ignore-vuln`
  curation and license-scan job without changing DEP-001's structure.

### 6.2 Related (soft)

- **SPEC-DEP-001 (implemented)** — established the `--ignore-vuln`
  suppression mechanism and the inline-rationale convention this SPEC
  follows (script header REQ-DEP-004; pip-audit ignore-list curation merged
  in PR #29). This SPEC's suppressions reuse that DEP-001 convention.
- **SPEC-SEC-001 (implemented)** — context only: defines the broader
  security CI strategy. It does NOT own the `--ignore-vuln` list or the
  inline-rationale convention (those are DEP-001's, above).

### 6.3 Blockers (carry-forward, do NOT block plan-auditor)

- **B1 — license-scan violator unidentifiable statically.**
  `docs/licenses/*.txt` are generated at CI time; only
  `docs/licenses/.gitkeep` (0 bytes) is committed. The run phase MUST run
  the license-scan job steps locally (uv sync + pip-licenses per service)
  to surface the offending package before REQ-DEP2-070/080 can be scoped.
  Because the one known AGPL violator (`searxng`) is already covered by the
  `EXCEPTIONS` service-boundary entry, a freshly-surfaced violation would
  necessarily come from a NEW dependency. (F9, needs-decision)
- **B2 — aiohttp 3.13.5 / litellm 1.80.0 transitively pinned by
  knowledge-storm==1.1.1.** A clean in-repo bump is blocked until
  knowledge-storm releases a version loosening the litellm/aiohttp pin →
  suppression per REQ-DEP2-030. (F1/F3)
- **B3 — embedder / researcher have no committed `uv.lock`.** Their exact
  CI-failing transitive versions are non-deterministic and were inferred
  from the storm lockfile + dependency tree; committing `uv.lock`
  (REQ-DEP2-060) is a prerequisite to deterministic auditing. (F10)

### 6.4 External dependencies (blocking real bumps)

| External release | Unblocks |
|------------------|----------|
| `torch` fix for CVE-2025-3000 (fix TBD, no released patch) | REQ-DEP2-050 (currently suppress-only) |
| `knowledge-storm > 1.1.1` unpinning `litellm==1.80.0` | aiohttp 3.13.5 bump (F1) |
| `litellm 1.84.0` (or knowledge-storm-compatible release) carrying aiohttp 3.14.1 + litellm CVE fixes | F1/F3 real bump |
| `fastapi` release pinning `starlette >= 1.3.1` (current 0.136.1 → starlette 1.1.0) | starlette bump (F2) |

---

## 7. Files to Create / Modify

### 7.1 Modified

| Path | Change |
|------|--------|
| `.github/workflows/deps-audit.yml` | extend `pip-audit` `--ignore-vuln` for aiohttp/starlette/litellm(1.84.0-only)/torch with inline upstream-rationale comments (REQ-DEP2-030, REQ-DEP2-040, REQ-DEP2-050) |
| `services/storm/uv.lock` | `uv lock --upgrade-package` for msgpack, ujson, langsmith off their vulnerable pins (REQ-DEP2-020) |
| `scripts/check-license-allowlist.sh` | (conditional on B1) add justified service-boundary `EXCEPTIONS` entry for any NEW surfaced license violator, OR no change if the dependency is replaced or already covered (REQ-DEP2-080) |

### 7.2 Created

| Path | Purpose |
|------|---------|
| `services/embedder/uv.lock` | committed lockfile for deterministic transitive auditing (REQ-DEP2-060) |
| `services/researcher/uv.lock` | committed lockfile for deterministic transitive auditing (REQ-DEP2-060) |

### 7.3 Existing — Unchanged

- `services/embedder/pyproject.toml`, `services/researcher/pyproject.toml`
  — dependency declarations unchanged; only lockfiles added.
- All Go packages, adapters, synthesis flows — no behavior change.

---

*End of SPEC-DEP-002 v0.1.0 (draft).*
