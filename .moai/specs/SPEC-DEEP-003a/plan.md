# SPEC-DEEP-003a Implementation Plan

File-level TDD plan for adding the `json-repair` third parse tier to
`parse_sub_queries`. Methodology: TDD (RED-GREEN-REFACTOR). Priority
labels only (no time estimates).

---

## Files to Create / Modify

| Marker | Path | Purpose |
|--------|------|---------|
| [MODIFY] | `services/researcher/pyproject.toml` | Add `json-repair` to `[project].dependencies` with a bounded pin `>=0.61,<1.0` (REQ-DEEP3a-301; see OQ-1/OQ-4) |
| [MODIFY] | `services/researcher/src/researcher/deep_tree.py` | Add tier-3 repair branch + tier-attribution logging in `parse_sub_queries` (REQ-DEEP3a-101..104, 201, 202); add `import json_repair` |
| [MODIFY] | `services/researcher/tests/test_deep_tree.py` | Add repair-tier pytest cases (A1-A11) |
| [N/A] | `services/researcher/uv.lock` | No `uv.lock` exists for this service (build-system is `hatchling`, no `[tool.uv]` section); nothing to regenerate. Confirm the service's actual dependency-management/install flow at run phase. |
| [EXISTING — UNCHANGED] | `services/researcher/src/researcher/synthesis.py` | No `json.loads` site; out of scope |
| [EXISTING — UNCHANGED] | `services/researcher/src/researcher/eval_judge.py` | No `json.loads` site; out of scope |
| [EXISTING — UNCHANGED] | `internal/deepagent/*.go` | Consumes parsed `sub_queries`; unaffected |

---

## Implementation Order (TDD)

Priority High → Low. Each step is RED (failing test) → GREEN (minimal
code) → REFACTOR.

### Step 1 (Priority High) — Dependency declaration (REQ-DEEP3a-301)

- Add the dependency directly to `[project].dependencies` in
  `pyproject.toml` with a bounded pin (default `json-repair>=0.61,<1.0`
  matching the validated release; see OQ-1). There is no `uv.lock` for
  this service (build-system is `hatchling`, no `[tool.uv]`), so the lock
  step is N/A; confirm the service's actual install flow at run phase.
- Add `import json_repair` to `deep_tree.py` (top-level import block,
  alongside `import json`).
- Verify the import resolves (acceptance §A11).

Rationale: dependency must exist before the repair branch can import it.

### Step 2 (Priority High) — Repair tier (REQ-DEEP3a-101, 102, 104)

RED: write tests A1-A6 + A9 asserting:
- trailing comma, single quotes, unquoted keys, truncated array,
  prose/fence-wrapped malformed array each recover a non-empty
  sub-query list via the repair tier;
- the repaired list is truncated to `breadth` (A6);
- clean JSON still parses at tier 1 without invoking repair (A9).

GREEN: inside `parse_sub_queries`, restructure the tier-2 failure
branches so that instead of `return []`, control falls through to a
tier-3 attempt:
```
# tier 3 (repaired): json_repair.loads(raw)
parsed = json_repair.loads(raw)
```
Route the tier-3 `parsed` value through the SAME existing post-parse
normalization (list-check, `str(item)` coercion, breadth truncation).
The cleanest GREEN structure: refactor the parse cascade so a single
post-parse normalization block runs once on whichever tier produced
`parsed`, with a `tier` variable tracking provenance.

REFACTOR: extract a small helper or local variable `parse_tier: str`;
keep the function under a reasonable size; ensure tiers 1/2 fast path is
unchanged.

### Step 3 (Priority High) — No-regression / no-raise (REQ-DEEP3a-103)

RED: write tests A7 (totally unparseable garbage → `[]`) and A8 (repair
tier never raises — feed input that could make json_repair return a
non-list or empty; assert `[]` and no exception).

GREEN: wrap the tier-3 call and its normalization defensively so any
exception OR non-list OR empty-after-normalization yields `[]`. Confirm
that every input that returned `[]` under the parent SPEC still returns
`[]`.

### Step 4 (Priority Medium) — Tier-attribution logging (REQ-DEEP3a-201, 202)

RED: write test A10 asserting that each invocation logs exactly one of
`standard` / `substring` / `repaired` / `failed`, and that `repaired`
and `failed` are emitted at WARNING level (use `caplog` to assert level
and message content).

GREEN: set `parse_tier` at each successful/failed branch and emit a
single `logger.info(...)` (for standard/substring success) or
`logger.warning(...)` (for repaired success and for failed) at the end
of the function with the tier identifier.

REFACTOR: ensure exactly one log statement fires per call (no
double-logging from the parent SPEC's existing warnings — consolidate or
preserve them deliberately so total log count per call is exactly one
tier-attribution line; the parent's pre-existing per-failure warnings may
be folded into the single `failed`/`repaired` line).

### Step 5 (Priority Low) — Self-review + coverage

- Run `ruff check` + `ruff format` (line-length 120).
- Run `pytest --cov=researcher.deep_tree` — confirm ≥85% coverage of the
  amended function and the new repair branch.
- Confirm existing endpoint tests in `test_deep_tree.py` still pass
  (no contract regression).

---

## Technical Approach Notes

- **Tier ordering**: `json.loads(raw)` → `json.loads(raw[start:end])` →
  `json_repair.loads(raw)` → `[]`. The repair tier receives the original
  `raw` (OQ-3 default) because json-repair strips surrounding prose/fences
  itself; acceptance §A5 verifies fenced input recovers.
- **Single normalization block**: the list-check + `str()` coercion +
  breadth truncation must run identically for all three tiers. The GREEN
  implementation should converge the three tiers onto one normalization
  path rather than duplicating it three times.
- **Logging consolidation**: the function emits exactly one
  tier-attribution log line per call (REQ-DEEP3a-201). The parent SPEC's
  two pre-existing `logger.warning` calls in the tier-2 failure branches
  are subsumed into the `failed` path's single WARNING line.
- **No-raise guarantee**: the entire tier-3 block (call + normalization)
  is guarded so the function's worst case is `return []`, matching the
  parent behavior exactly.

---

## MX Tag Plan

| File::Symbol | Tag | Reason |
|--------------|-----|--------|
| `deep_tree.py::parse_sub_queries` | `@MX:NOTE` | Document the three-tier parse cascade (standard/substring/repaired) and the no-raise contract. Update the existing function docstring; reference SPEC-DEEP-003a. `code_comments: en`. |

(No `@MX:ANCHOR`/`@MX:WARN` warranted: fan_in is 1 — only `decompose_query`
calls it — and there is no goroutine/complexity danger zone. The existing
`@MX` tags on the endpoint/decompose path in `deep_tree.py` are unchanged.)

---

## Out of Scope (mirror of spec.md §4)

- Regex fallback tier (json-repair only).
- synthesis.py / eval_judge.py (no json.loads sites).
- Prompt redesign, `response_format` API enforcement.
- New Prometheus metric for repair-tier usage.
- Go-side changes; LLM-call retry on parse failure.
