---
id: SPEC-DEP-003
version: 0.1.0
status: draft
created: 2026-06-23
updated: 2026-06-23
author: limbowl
priority: P3
issue_number: 0
title: LangChain 1.x API migration (researcher) — CI-debt remediation, NOT-REPRODUCIBLE / false-positive against first-party code
milestone: CI-debt cleanup
owner: manager-spec
methodology: ddd
coverage_target: 0
depends_on: []
blocks: []
related: [SPEC-DEP-001]
---

# SPEC-DEP-003: LangChain 1.x API migration (researcher)

## HISTORY

- 2026-06-23 (initial draft v0.1.0, limbowl via manager-spec):
  Remediation SPEC for the CI-debt item cited as "researcher pytest fails:
  `ModuleNotFoundError: No module named langchain.schema`", attributed to
  LangChain 1.x removing/moving the `langchain.schema` module. Grounded in
  a source-level investigation re-run against the current `main`.

  **Headline conclusion: NOT REPRODUCIBLE / FALSE-POSITIVE against
  first-party code.** No first-party Python source imports `langchain`
  (let alone `langchain.schema`) anywhere — not in `researcher`, not in
  `storm`, not in any service `.py` outside `.venv`. No first-party import
  migration is required. The most likely cause is a stale CI log captured
  before `storm` pinned `knowledge-storm==1.1.1` / before mock-based test
  isolation, or a log copied from an upstream project that imported
  `langchain.schema` directly.

  Verified at draft time against the live tree:
  - `grep -rn 'langchain' services/researcher/src services/researcher/tests`
    → zero hits. `researcher` pyproject deps = fastapi, uvicorn, pydantic,
    httpx, openai, deepeval~=1.0, json-repair (no langchain).
  - `grep -rn 'langchain.schema' services/` (excl `.venv`) → zero hits.
  - `services/storm/uv.lock`: `langchain-core` resolves to `version =
    "1.4.0"` (1.x); `langchain-text-splitters` to `version = "1.1.2"`
    (1.x). These are TRANSITIVE deps of `knowledge-storm==1.1.1` — already
    on langchain 1.x. No `langchain` meta-package is pinned anywhere.

  This SPEC therefore proposes NO first-party code change. It (a) records
  the false-positive cluster, (b) specifies the verification + closure
  actions, and (c) captures hardening invariants (lazy/TYPE_CHECKING
  import discipline, mock-based test isolation, `langchain_core.*` over
  `langchain.schema` if langchain is ever adopted) so the debt does not
  re-open. The ONLY scenario that produces a first-party fix surface is
  conditional (REQ-DEP3-050): a genuine runtime error from INSIDE the
  third-party `knowledge-storm` wheel, fixable only in
  `services/storm/pyproject.toml` + `services/storm/uv.lock`, never in
  `researcher`.

  Methodology: DDD (audit existing surface, characterize, only-then act).
  Coverage target N/A (no production code authored by default). Priority
  P3 — informational debt-closure, not a release blocker.

---

## 1. Overview

SPEC-DEP-003 closes the CI-debt item tracked as a LangChain 1.x API
migration for the `researcher` service. The cited failure signature is:

> `researcher pytest fails: ModuleNotFoundError: No module named
> langchain.schema` (attributed to LangChain 1.x removing/moving
> `langchain.schema`).

A source-level re-investigation against the current `main` shows this
signature does **not reproduce** from first-party code. This SPEC's
essence is verification and closure, not feature work — DDD methodology
applies (ANALYZE existing surface → confirm no behavior to PRESERVE →
no IMPROVE needed unless the conditional REQ-DEP3-050 trigger fires).

### 1.1 Investigation findings

Each finding below is reproduced verbatim from the verified
investigation and labelled with its status. **Findings marked
`false-positive` or `already-fixed` require NO action.** The single
conditional fix surface is isolated in REQ-DEP3-050.

| # | Locus | Issue (as cited) | Status | Evidence |
|---|-------|------------------|--------|----------|
| F1 | `services/researcher/src/researcher/**` (all `.py`) | "researcher pytest fails: `ModuleNotFoundError: No module named langchain.schema`" — researcher has NO langchain import anywhere | **false-positive** | `grep -rn 'langchain' services/researcher/src services/researcher/tests` → zero hits; pyproject deps = fastapi, uvicorn, pydantic, httpx, openai, deepeval~=1.0, json-repair (no langchain) |
| F2 | repo-wide `grep 'langchain.schema' --include=*.py` (excl `.venv`) | `langchain.schema` import path does not exist in any first-party source | **false-positive** | `grep -rn 'langchain.schema' services/` → no output; `grep` for `from langchain import` / `import langchain` (excl `.venv`) → zero hits |
| F3 | `services/researcher/src/researcher/eval_judge.py:134-136` | deepeval import (suspected transitive langchain puller) is lazy, never triggered at module/test import time | **false-positive** | `from deepeval.metrics import FaithfulnessMetric  # type: ignore` is nested inside `deepeval_judge()` function body, not module top-level; module docstring: "imports cleanly even when deepeval is absent" |
| F4 | `services/storm/uv.lock:1535-1543` | langchain-* packages present, but as TRANSITIVE deps of `knowledge-storm==1.1.1`, already pinned to langchain 1.x | **already-fixed** | `knowledge-storm` 1.1.1 deps: langchain-huggingface, langchain-qdrant, langchain-text-splitters; `langchain-core` resolves to `version = "1.4.0"` (1.x); `langchain-text-splitters` to `1.1.2`. No `langchain` meta-package pinned anywhere |
| F5 | `services/storm/src/storm/pipeline.py:27,122` + `tests/test_pipeline.py:33-82` | storm (real langchain consumer via knowledge_storm) never imports knowledge_storm in CI — lazy import + tests fully mock it | **false-positive** | `pipeline.py:27` `from knowledge_storm import (...)` is under `if TYPE_CHECKING:`; `pipeline.py:122` import is inside `build_lm_configs()`; `test_pipeline.py:42` `ks_mock = types.ModuleType('knowledge_storm')` injects mocks into `sys.modules` |
| F6 | `.github/workflows/python.yml:17,41` | CI matrix `[researcher, storm, embedder]` runs `uv run pytest` — neither service exercises a real `langchain.schema` import | **false-positive** | matrix service: `[researcher, storm, embedder]`; step: `uv run --directory services/${{ matrix.service }} pytest`. researcher has no langchain; storm mocks knowledge_storm |

### 1.2 Root cause

The cited CI-debt signature does NOT reproduce against the current
codebase. There is no first-party import of `langchain` (let alone
`langchain.schema`) anywhere — not in `researcher`, not in `storm`, not
in any service `.py` outside `.venv`.

1. The `researcher` service has zero langchain dependency; its only
   LLM-eval dep is `deepeval`, which is lazy-imported inside
   `eval_judge.deepeval_judge()` and never loaded at module/test import
   time.
2. The only langchain packages in the repo are TRANSITIVE deps of
   `knowledge-storm==1.1.1` in `storm/uv.lock`, and they are ALREADY on
   langchain 1.x (`langchain-core` 1.4.0, `langchain-text-splitters`
   1.1.2). `storm` imports `knowledge_storm` only lazily / under
   `TYPE_CHECKING`, and `storm`'s pytest fully mocks `knowledge_storm` +
   `knowledge_storm.lm`, so the real langchain chain is never imported in
   CI.

Any `langchain.schema` `ModuleNotFoundError` could only originate INSIDE
the third-party `knowledge-storm` wheel's own runtime code — out of repo
scope — and the lock already resolves a langchain 1.x set that
`knowledge-storm` 1.1.1 declares as compatible. Conclusion: no
first-party import migration is required; the finding is a
false-positive / misattribution against first-party code, most likely a
stale CI log or a log copied from an upstream project.

### 1.3 Flagged findings requiring an explicit decision

[HARD] The following are surfaced for an explicit human decision rather
than silent closure:

- **F1, F2, F3, F5, F6 (`false-positive`)** — No code defect. Decision
  required: **close-as-not-reproducible** in the remediation tracker
  (recommended) vs keep open pending a fresh CI run. This SPEC recommends
  closure after the REQ-DEP3-010 verification run confirms green.
- **F4 (`already-fixed`)** — `knowledge-storm` transitive langchain set
  is already on 1.x. No action; informational.
- **Suppress-with-justification vs fix** — There is nothing to suppress
  (no failing first-party import) and nothing to fix (no first-party
  importer). The only conditional fix path is REQ-DEP3-050, gated on a
  genuine reproduction.

---

## 2. Scope

### 2.1 In scope

- Verifying the cited failure does not reproduce on current `main`
  (re-run the exact failing CI job).
- Recording the false-positive cluster (F1–F6) and a closure decision in
  the remediation tracker.
- Capturing hardening invariants so the debt cannot silently re-open:
  lazy / `TYPE_CHECKING` import discipline in `storm`, mock-based test
  isolation, and a `langchain_core.*`-over-`langchain.schema` rule if
  langchain is ever adopted first-party.
- A single CONDITIONAL fix path (REQ-DEP3-050) scoped ONLY to
  `services/storm/pyproject.toml` + `services/storm/uv.lock`, triggered
  exclusively if a genuine runtime `ModuleNotFoundError` for a moved
  langchain module surfaces from inside the `knowledge-storm` wheel.

### 2.2 Out of scope

- Any edit to `services/researcher/**` (no langchain importer exists
  there).
- Adding a `langchain` direct dependency to any first-party service.
- Migrating first-party imports from `langchain.schema` to
  `langchain_core.*` (there are none to migrate).
- Patching `knowledge-storm` wheel internals (third-party; fixable only
  via an upstream release or a version bump in `services/storm`).
- Any production behavior change to `researcher`, `storm`, or `embedder`
  request handling, synthesis, or eval logic.

---

## 3. EARS Requirements

REQ ids use the `DEP3` domain, numbered 10/20/.../70.

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-DEP3-010** | Event-Driven | WHEN closing the SPEC-DEP-003 cluster, the remediation process SHALL re-run the exact `python.yml` CI job for the `researcher` and `storm` matrix legs on current `main` and SHALL record the run result (green / specific error) in the tracker before marking the cluster closed. | P3 | `gh run list --workflow python.yml` shows a recent run; the `researcher` and `storm` legs are green for the `langchain.schema` signature; the result is recorded in the remediation tracker. |
| **REQ-DEP3-020** | Optional | WHERE first-party Python services (`researcher`, `storm`, `embedder`) are tested in CI, the system SHALL keep first-party source free of any direct `langchain.schema` import, using `langchain_core.*` symbols instead if langchain is ever adopted first-party. | P3 | `grep -rn 'langchain.schema' services/researcher services/storm services/embedder` (excluding `.venv`) returns zero matches; the equivalent CI grep step (or manual check) reports zero first-party hits. |
| **REQ-DEP3-030** | Ubiquitous | The `storm` service SHALL import `knowledge_storm` only via lazy (function-local) or `TYPE_CHECKING`-guarded imports, so that module import and unit tests succeed without the optional heavy dependency installed. | P3 | `services/storm/src/storm/pipeline.py` imports `knowledge_storm` only under `if TYPE_CHECKING:` (line 27) and inside `build_lm_configs()` (line 122); `python -c 'import storm.pipeline'` succeeds without `knowledge_storm` installed. |
| **REQ-DEP3-040** | State-Driven | WHILE `storm` unit tests exercise pipeline code, the system SHALL inject mock `knowledge_storm` and `knowledge_storm.lm` modules into `sys.modules` so that no real langchain transitive import is required to pass pytest. | P3 | `services/storm/tests/test_pipeline.py` constructs `ks_mock = types.ModuleType('knowledge_storm')` and injects `knowledge_storm` + `knowledge_storm.lm` into `sys.modules`; `uv run --directory services/storm pytest` passes with no real `knowledge_storm` wheel imported. |
| **REQ-DEP3-050** | Conditional (IF-THEN) | IF the `knowledge-storm` transitive dependency raises `ModuleNotFoundError` for a moved langchain module at runtime, THEN the `storm` service SHALL surface the error from the lazy import boundary (`build_lm_configs`) rather than at service-module import time, AND the remediation SHALL be confined to `services/storm/pyproject.toml` (the `knowledge-storm==1.1.1` specifier) plus `services/storm/uv.lock` (pin/bump to a release whose internal imports target `langchain_core.*`), with NO `langchain` direct dependency added and NO edit to `researcher`. | P3 | `uv run --directory services/storm python -c 'import knowledge_storm'` reproduces (or fails to reproduce) the error; any fix touches only `services/storm/pyproject.toml` + `services/storm/uv.lock`; the error, if any, surfaces from `build_lm_configs()` not at module import; `git diff --stat` shows zero changes under `services/researcher/`. |
| **REQ-DEP3-060** | Ubiquitous | The `researcher` service SHALL keep `deepeval` and LiteLLM imports lazy inside `deepeval_judge()`, so the `eval_judge` module imports cleanly when those optional dependencies (and their transitives) are absent. | P3 | `services/researcher/src/researcher/eval_judge.py` imports `deepeval.metrics` only inside `deepeval_judge()` (lines 134-136); `python -c 'import researcher.eval_judge'` succeeds without `deepeval` installed. |
| **REQ-DEP3-070** | Event-Driven | WHEN a CI-debt finding cites a `ModuleNotFoundError`, the remediation process SHALL verify the cited import path exists in first-party source before scheduling a migration SPEC, to prevent acting on stale or misattributed logs. | P3 | The remediation runbook / tracker records a "cited import path verified in first-party source" step; for SPEC-DEP-003 this check fails (path absent), and the cluster is consequently classified NOT-REPRODUCIBLE. |

---

## 4. Acceptance Criteria

Headline acceptance (per CI-debt SPEC convention): **the named CI job
passes on `main`** — specifically, the `python.yml` `researcher` and
`storm` matrix legs are green and free of the `langchain.schema`
`ModuleNotFoundError` signature.

| Scenario | Description | Coverage |
|----------|-------------|----------|
| AC-1 | Re-run `python.yml` on current `main`; `researcher` and `storm` legs are green; result recorded in tracker; cluster marked closed. | REQ-DEP3-010, REQ-DEP3-070 |
| AC-2 | Grep first-party services for `langchain.schema` (excluding `.venv`) → zero matches; confirms no migration target exists. | REQ-DEP3-020 |
| AC-3 | `storm` imports `knowledge_storm` only under `TYPE_CHECKING` + inside `build_lm_configs()`; `import storm.pipeline` succeeds with `knowledge_storm` absent. | REQ-DEP3-030 |
| AC-4 | `storm` pytest injects mock `knowledge_storm` + `knowledge_storm.lm` into `sys.modules`; suite passes with no real langchain transitive import. | REQ-DEP3-040 |
| AC-5 (conditional) | IF a genuine `ModuleNotFoundError` reproduces inside the `knowledge-storm` wheel, THEN `git diff --stat` shows zero changes under `services/researcher/` (single binary check: pass iff the researcher diff is empty). ELSE this scenario is N/A and documented as such. | REQ-DEP3-050 |
| AC-6 | `researcher` `eval_judge` imports `deepeval`/LiteLLM only inside `deepeval_judge()`; `import researcher.eval_judge` succeeds with `deepeval` absent. | REQ-DEP3-060 |
| AC-7 | The remediation tracker records the "cited import path verified in first-party source" check; the absence of `langchain.schema` in first-party source classifies the cluster NOT-REPRODUCIBLE. | REQ-DEP3-070 |

---

## 5. Exclusions (What NOT to Build)

[HARD] The following are explicitly excluded from this SPEC. Each entry
records a known destination, rationale, or follow-up.

- **First-party LangChain import migration**. → No-op. No first-party
  source imports `langchain` or `langchain.schema` (F1, F2). There is
  nothing to migrate.
- **Any edit to `services/researcher/**`**. → Excluded. The researcher
  service has zero langchain dependency (F1); editing it would be scope
  creep with no defect to fix.
- **Adding a `langchain` direct dependency**. → Excluded. Would introduce
  a heavy dependency the codebase deliberately avoids; the only langchain
  packages present are `knowledge-storm` transitives already on 1.x (F4).
- **Patching `knowledge-storm` wheel internals**. → Out of repo scope.
  Any genuine `langchain.schema` error lives inside the third-party wheel
  and is fixable only by an upstream `knowledge-storm` release or a
  version bump in `services/storm` (REQ-DEP3-050), never by editing wheel
  code in this repo.
- **De-mocking `storm` tests to import real `knowledge_storm`**. →
  Excluded. The mock-based isolation (F5, REQ-DEP3-040) is the correct
  pattern that keeps CI fast and free of the heavy transitive chain;
  removing it would re-introduce the risk this SPEC closes.
- **Production behavior changes** to `researcher` / `storm` / `embedder`
  request handling, synthesis, or eval logic. → Out of scope; this is
  debt closure, not feature work.

---

## 6. Dependencies & Blockers

### 6.1 Related SPECs (related)

- **SPEC-DEP-001 (implemented)** — Dependency baseline + the
  `deps-audit.yml` / dependency-scanning conventions. SPEC-DEP-003 is a
  sibling debt-closure item in the same `DEP` cluster and inherits its
  pin/exception conventions (e.g. `uv lock --locked` strict mode for
  Python sidecars). Soft relation only — no hard code dependency.

### 6.2 Blockers

- **Cannot reproduce the cited failure from source.** No
  `langchain.schema` importer exists in `researcher` or `storm`
  first-party code (F1, F2, F5). Closing the cluster as resolved (vs
  defect) needs a fresh CI run (REQ-DEP3-010) to confirm the cited log is
  stale rather than a current code defect.

### 6.3 External dependencies (third-party, out of repo)

- **`knowledge-storm==1.1.1` (PyPI third-party wheel)** is the sole
  upstream owner of any internal `langchain.schema` usage. Any genuine
  `langchain.schema` error lives inside this wheel and is fixable only by
  an upstream `knowledge-storm` release or a version bump in
  `services/storm` — not patchable in this repo.
- **`langchain-core` 1.4.0 / `langchain-text-splitters` 1.1.2 /
  `langchain-huggingface` 1.2.2 / `langchain-qdrant` 1.1.0** are
  transitive and version-managed by `knowledge-storm`'s dependency
  constraints (already langchain 1.x — F4).

---

## 7. Files in Scope

### 7.1 Read / verified (no change by default)

| Path | Role |
|------|------|
| `services/researcher/src/researcher/eval_judge.py` | confirms lazy deepeval import (F3, REQ-DEP3-060) |
| `services/researcher/pyproject.toml` | confirms no langchain dependency (F1) |
| `services/storm/src/storm/pipeline.py` | confirms lazy / `TYPE_CHECKING` `knowledge_storm` import (F5, REQ-DEP3-030) |
| `services/storm/tests/test_pipeline.py` | confirms `sys.modules` mock injection (F5, REQ-DEP3-040) |
| `services/storm/uv.lock` | confirms langchain-core 1.4.0 / text-splitters 1.1.2 (F4) |
| `.github/workflows/python.yml` | the CI matrix to re-run (F6, REQ-DEP3-010) |

### 7.2 Conditionally modified — ONLY if REQ-DEP3-050 triggers

| Path | Change (conditional) |
|------|----------------------|
| `services/storm/pyproject.toml` | bump/pin the `knowledge-storm` specifier to a release whose internal imports target `langchain_core.*` |
| `services/storm/uv.lock` | re-lock for the bumped `knowledge-storm` |

NO other files are in scope. In particular, NOT
`services/researcher/**`, and NO new `langchain` direct dependency.

---

*End of SPEC-DEP-003 v0.1.0 (draft).*
