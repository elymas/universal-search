---
id: SPEC-DEEP-003a
title: Robust LLM JSON Parsing for Deep-Research Decomposition (Amendment to SPEC-DEEP-003)
version: 0.1.0
status: draft
created: 2026-06-17
updated: 2026-06-17
author: limbowl
priority: P1
owner: expert-backend
methodology: tdd
coverage_target: 85
issue_number: null
milestone: M5 — /deep multi-agent
depends_on: [SPEC-DEEP-003]
---

# SPEC-DEEP-003a: Robust LLM JSON Parsing for Deep-Research Decomposition (Amendment to SPEC-DEEP-003)

## HISTORY

- 2026-06-17 (initial draft v0.1.0, limbowl via manager-spec): First
  EARS-formatted SPEC for a focused amendment to SPEC-DEEP-003. The
  parent SPEC's `parse_sub_queries(raw, breadth)` function in
  `services/researcher/src/researcher/deep_tree.py` (lines ~84-123)
  parses LLM decomposition responses with a two-tier strategy:
  (tier 1) `json.loads(raw)`; on `JSONDecodeError`, (tier 2) extract the
  array substring `raw[start:end]` between the first `[` and last `]`
  and `json.loads` that. Both tiers fail when the JSON is structurally
  malformed *inside* the array — trailing commas, single-quoted keys,
  unquoted keys, truncated/cut-off output from reasoning models. On that
  failure the function logs a warning and silently returns `[]`, which
  produces an empty decomposition and a degenerate (single-node) tree
  with no observable signal beyond the log line.

  This amendment adds a THIRD parse tier using the `json-repair` PyPI
  library, attempted after the substring tier and before giving up.
  Ported from assafelovic/gpt-researcher v3.5 (PR #1773), which added
  `json-repair` + a regex fallback to harden LLM JSON parsing for
  reasoning-model output. The repair tier MUST NOT raise; on total
  failure across all tiers the function returns `[]` exactly as today
  (no regression). The amendment also adds observability: the function
  logs WHICH parse tier produced the result (standard / substring /
  repaired / failed) so operators can detect reasoning-model JSON drift.

  Scope is deliberately minimal: one new dependency line in
  `services/researcher/pyproject.toml`, the third tier inside
  `parse_sub_queries`, tier-attribution logging, and pytest cases.
  Verified during research that `deep_tree.py` is the ONLY site that
  parses LLM output with `json.loads` — `synthesis.py` and
  `eval_judge.py` contain no `json.loads` call sites (see Open Questions
  OQ-2). Companion artifacts: research.md, plan.md, acceptance.md.

  7 EARS REQs (5 × P1 + 2 × P2), ≥1 exclusion entry, ≥7 acceptance
  criteria. Methodology: TDD (per quality.yaml), coverage target 85%.
  Owner: expert-backend. Status `draft` pending plan-auditor review +
  annotation cycle; run phase promotes to `approved`.

---

## 1. Overview

SPEC-DEEP-003 introduced per-node sub-query generation for the `/deep`
tree explorer via the Python sidecar endpoint `POST /decompose_query`
(`services/researcher/src/researcher/deep_tree.py`). The LLM is
prompted to "Return ONLY a JSON array of strings". Reasoning models in
practice do not always comply: they emit trailing commas, single-quoted
keys, unquoted keys, output truncated mid-array by a token limit, or
JSON wrapped in prose / markdown code fences.

The current `parse_sub_queries(raw, breadth)` has two parse tiers:

1. **Standard**: `json.loads(raw)`.
2. **Substring**: on `JSONDecodeError`, slice `raw[first '[' : last ']' + 1]`
   and `json.loads` that slice.

Both tiers feed the slice (or whole string) into the *strict* standard
library loader. When the malformation is *inside* the array — not merely
surrounding prose — the substring tier extracts a still-invalid slice and
also fails. The function then logs a warning and returns `[]`. An empty
return collapses the node's child frontier to zero, yielding a degenerate
tree, with no signal beyond a single WARN log line.

### 1.1 What this amendment changes

This amendment inserts a **third parse tier** between the substring tier
and the `return []` fallback:

3. **Repaired**: feed `raw` (or the substring slice) into
   `json_repair.loads(...)` from the `json-repair` PyPI library, which
   tolerates the malformations above and returns a best-effort parse.

It also adds **tier-attribution logging**: the function logs which tier
(`standard` / `substring` / `repaired` / `failed`) produced the result,
so reasoning-model JSON drift is observable in production.

The repair tier MUST NOT raise. On total failure (all three tiers
exhausted, or the repaired output is not a usable list of sub-queries),
the function returns `[]` exactly as today — no behavioral regression for
the already-handled paths.

### 1.2 Architecture decision (pinned)

- **Library**: `json-repair` (PyPI). Import name `json_repair`; primary
  function `json_repair.loads(s)`. It is a drop-in replacement for
  `json.loads` that first tries the standard loader and falls back to a
  repair parser, returning a best-effort value (empty value when the
  input is unrecoverable) rather than raising in its default mode.
  Ported from assafelovic/gpt-researcher v3.5 (PR #1773).
- **Tier ordering is preserved**: standard → substring → repaired →
  `[]`. The repair tier is additive; tiers 1 and 2 are unchanged so
  already-passing inputs take the identical fast path and produce
  byte-identical results.
- **Truncation-to-breadth is unchanged**: the existing post-parse
  normalization (filter falsy items, `str(item)`, truncate to `breadth`)
  applies to the repaired result identically.

---

## 2. EARS Requirements

REQ IDs are inline. Group numbering: 10 (parse tiers), 20 (observability),
30 (dependency).

### 2.1 Parse Tiers (REQ-DEEP3a-10x)

**REQ-DEEP3a-101** (Event-Driven) [P1]:
WHEN an LLM decomposition response cannot be parsed by the standard JSON
loader (tier 1 `json.loads(raw)` raises `JSONDecodeError`) NOR by the
array-substring extraction (tier 2 `json.loads(raw[start:end])` raises
`JSONDecodeError`, or no `[`...`]` array bracket pair is found), the
system SHALL attempt repair via the `json-repair` library
(`json_repair.loads`) before returning an empty result.
(Acceptance §A1, §A2, §A3, §A4, §A5)

**REQ-DEEP3a-102** (State-Driven) [P1]:
WHILE the repair tier (REQ-DEEP3a-101) is producing a value, the system
SHALL apply the SAME post-parse normalization used by tiers 1 and 2:
verify the parsed value is a list, coerce each truthy element via
`str(item)`, drop falsy elements, and truncate the result to at most
`breadth` items. The repaired result SHALL NOT bypass the list-type
check nor the breadth truncation.
(Acceptance §A1, §A2, §A6)

**REQ-DEEP3a-103** (Unwanted Behavior) [P1]:
IF the repair tier raises any exception, OR the repaired value is not a
list, OR the repaired value yields zero usable sub-queries, THEN the
system SHALL return `[]` (the existing tier-2-failure behavior) and
SHALL NOT propagate the exception to the caller. The repair tier SHALL
be a no-regression addition: any input that already returned `[]` under
the parent SPEC SHALL still return `[]`.
(Acceptance §A7, §A8)

**REQ-DEEP3a-104** (Ubiquitous) [P1]:
The system SHALL preserve the existing tier-1 (standard) and tier-2
(substring) behavior unchanged: any `raw` input that tier 1 or tier 2
parsed successfully under the parent SPEC SHALL produce a byte-identical
sub-query list and SHALL NOT invoke the repair tier (the repair tier is
reached only after both prior tiers fail).
(Acceptance §A9)

### 2.2 Observability (REQ-DEEP3a-20x)

**REQ-DEEP3a-201** (Ubiquitous) [P2]:
The system SHALL log which parse tier produced the result, using the
literal tier identifiers `standard`, `substring`, `repaired`, or
`failed`. A successful parse at any tier SHALL log the identifier of the
tier that succeeded; total failure SHALL log `failed`. The log statement
SHALL be emitted exactly once per `parse_sub_queries` invocation.
(Acceptance §A10)

**REQ-DEEP3a-202** (Event-Driven) [P2]:
WHEN the repaired tier succeeds (tiers 1 and 2 having failed), the system
SHALL emit the log at a severity that surfaces reasoning-model JSON drift
to operators (WARNING level), because recurring reliance on repair
signals an upstream prompt or model-compliance problem worth attention.
A `failed` outcome SHALL also be logged at WARNING level (preserving the
parent SPEC's existing warning on total failure).
(Acceptance §A10)

### 2.3 Dependency (REQ-DEEP3a-30x)

**REQ-DEEP3a-301** (Ubiquitous) [P1]:
The system SHALL declare `json-repair` as a runtime dependency in
`services/researcher/pyproject.toml` under `[project].dependencies` with
a bounded version constraint pinned to the validated release line
(`>=0.61,<1.0`; see OQ-1), and SHALL import it as `json_repair` within
`deep_tree.py`. No other dependency manifest (Go `go.mod`, other service
`pyproject.toml`) SHALL be modified.
(Acceptance §A11)

---

## 3. Acceptance Criteria (Summary)

Detailed Given/When/Then scenarios are in
`.moai/specs/SPEC-DEEP-003a/acceptance.md`. Summary:

| Ref | Scenario | Maps to REQ |
|-----|----------|-------------|
| A1 | Trailing comma inside array recovers via repair tier | 101, 102 |
| A2 | Single-quoted keys/strings recover via repair tier | 101, 102 |
| A3 | Unquoted keys recover via repair tier | 101 |
| A4 | Truncated / cut-off array recovers via repair tier | 101 |
| A5 | JSON wrapped in prose / markdown fences recovers | 101 |
| A6 | Repaired result is truncated to breadth | 102 |
| A7 | Totally unparseable garbage still returns `[]` | 103 |
| A8 | Repair tier never raises (exception-safe) | 103 |
| A9 | Clean JSON still parses at tier 1, repair not invoked | 104 |
| A10 | Tier-attribution logging (standard/substring/repaired/failed) | 201, 202 |
| A11 | `json-repair` declared in pyproject.toml, imported as json_repair | 301 |

Coverage target: 85% of `parse_sub_queries` and the new repair branch
(per `quality.test_coverage_target`).

---

## 4. Exclusions (What NOT to Build)

[HARD] This amendment is deliberately narrow. The following are explicitly
out of scope:

- **Regex fallback as a 4th tier** — gpt-researcher PR #1773 pairs
  `json-repair` with a regex extraction fallback. This amendment adopts
  ONLY the `json-repair` library tier. A regex tier is not added; if
  `json-repair` cannot recover the value the function returns `[]`.
- **Changes to other LLM-output parse sites** — `synthesis.py` and
  `eval_judge.py` were verified to contain NO `json.loads` call sites
  (research.md §2). This amendment touches only `deep_tree.py`. Hardening
  any future parse site is a separate SPEC. (See Open Questions OQ-2.)
- **Decompose prompt redesign** — the system/user prompt in
  `build_decompose_prompt` is unchanged. Improving the prompt to reduce
  malformed output is a separate concern; this amendment hardens the
  parser, not the prompt.
- **`response_format` / structured-output API enforcement** — switching
  the LiteLLM/gateway call to enforce JSON schema at the model API level
  is out of scope. This amendment is a parser-side defensive measure.
- **New Prometheus metric for repair-tier usage** — observability is
  limited to a structured log line (REQ-DEEP3a-201/202). A
  `usearch_deep_tree_decompose_parse_tier{tier}` counter would require
  amending SPEC-OBS-001's cardinality allowlist and is out of scope.
- **Go-side changes** — `internal/deepagent/tree.go` and its callers
  consume the endpoint's parsed `sub_queries` list and are unaffected.
  No Go file is modified.
- **Retry of the LLM call on parse failure** — the function still does a
  single parse pass per response. Re-prompting the model on repair
  failure is out of scope (owned by tree-orchestration retry policy, not
  the parser).

---

## 5. Dependencies & Relationship to Parent

- **SPEC-DEEP-003** (implemented): owns `parse_sub_queries`,
  `build_decompose_prompt`, the `POST /decompose_query` endpoint, and the
  `DecomposeRequest`/`DecomposeResponse` Pydantic models. This SPEC is a
  surgical amendment to the body of `parse_sub_queries` only; it changes
  no public contract (signature, request/response models, HTTP status
  codes are all unchanged).
- No new downstream SPEC is blocked by this amendment.

---

## 6. Open Questions

- **OQ-1 (json-repair version pin)** — RECOMMENDED DEFAULT: pin a
  bounded range `json-repair>=0.61,<1.0` in `pyproject.toml`, matching
  the release line the acceptance criteria were validated against
  (verified release 0.61.0 on PyPI 2026-06-16). A bounded upper limit is
  used deliberately: acceptance A3/A4/A5 assert *specific* repair-output
  shapes, and json-repair's recovery shape can drift across versions
  (see OQ-4). The library's `loads` API and default non-raising behavior
  have been stable across the 0.x series. Whether to use a
  compatible-release pin (`~=0.61`) instead of the explicit range is a
  minor run-phase choice. Resolution owner: run-phase implementer.
- **OQ-2 (other parse sites)** — RESOLVED during research:
  `synthesis.py` and `eval_judge.py` contain NO `json.loads` LLM-output
  call sites (`grep` of `services/researcher/src/researcher/` finds
  `json.loads` only at `deep_tree.py:93` and `deep_tree.py:100`).
  Therefore those files are explicitly out of scope for this amendment
  (see §4 Exclusions). No follow-up parse-hardening work is required at
  this time.
- **OQ-3 (substring-then-repair vs raw-then-repair input)** —
  RECOMMENDED DEFAULT: pass the original `raw` string to
  `json_repair.loads` in the repair tier (json-repair handles
  surrounding prose / fences itself, making the substring step
  redundant for the repair tier). Whether to instead feed the tier-2
  substring slice is a minor implementation choice confirmed at run
  phase; both satisfy the EARS requirements. Resolution owner:
  run-phase implementer. Acceptance §A5 (prose/fence case) constrains
  this choice — the chosen input MUST recover the array from
  fenced/prose-wrapped input.
- **OQ-4 (recovery-shape drift across json-repair versions)** —
  RISK/OPEN: acceptance A3 (unquoted keys), A4 (truncation) and
  especially A5 (exact `["alpha","beta"]` from fenced+malformed input)
  assert *specific* repair-output shapes validated against json-repair
  0.61.0. json-repair's best-effort recovery shape can shift within a
  version range without any change to our code, which could break these
  ACs on a future release. MITIGATION (default): the bounded pin
  `>=0.61,<1.0` (OQ-1) caps drift to the validated major-version line.
  RESIDUAL: even within the range, minor releases could alter recovery
  output; if A3/A4/A5 fail after a routine dependency bump, prefer
  relaxing A5 to a bounded assertion (matching A4's `len>=N` +
  membership style) over loosening the pin. Resolution owner: run-phase
  implementer / dependency-bump reviewer.

---

## 7. References

### 7.1 Internal

- `.moai/specs/SPEC-DEEP-003/spec.md` — parent SPEC (tree exploration).
- `services/researcher/src/researcher/deep_tree.py:84-123` —
  `parse_sub_queries` (the function this SPEC amends).
- `services/researcher/tests/test_deep_tree.py` — existing endpoint
  tests (new repair-tier tests added here).
- `services/researcher/pyproject.toml` — dependency manifest.

### 7.2 External (verify URLs via WebFetch in Run phase)

- `https://github.com/assafelovic/gpt-researcher` — port source; v3.5
  PR #1773 added json-repair + regex fallback for reasoning-model JSON.
- `https://pypi.org/project/json-repair/` — `json-repair` PyPI package
  (import `json_repair`; `json_repair.loads`; non-raising default).

### 7.3 Companion Artifacts

- `.moai/specs/SPEC-DEEP-003a/research.md` — upstream evidence + current
  code state.
- `.moai/specs/SPEC-DEEP-003a/plan.md` — file-level implementation plan.
- `.moai/specs/SPEC-DEEP-003a/acceptance.md` — pytest scenarios.

---

*End of SPEC-DEEP-003a v0.1.0 (draft).*
