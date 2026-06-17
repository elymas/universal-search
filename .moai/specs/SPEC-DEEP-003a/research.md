# SPEC-DEEP-003a Research — Robust LLM JSON Parsing for Decomposition

Phase 0.5 research artifact. Captures upstream port evidence and the
verified current-code state for the SPEC-DEEP-003 amendment.

---

## 1. Current Code State (verified)

### 1.1 The function being amended

`services/researcher/src/researcher/deep_tree.py`, function
`parse_sub_queries(raw: str, breadth: int) -> list[str]` (lines ~84-123).

Two-tier parse strategy as it exists today:

- **Tier 1 (standard)**: `parsed = json.loads(raw)`.
- **Tier 2 (substring)**: on `json.JSONDecodeError`, compute
  `start = raw.find("[")` and `end = raw.rfind("]") + 1`; if a valid
  bracket pair exists, `parsed = json.loads(raw[start:end])`. On a second
  `JSONDecodeError`, it logs
  `logger.warning("Failed to parse LLM response as JSON: %s", raw[:200])`
  and returns `[]`. If no `[`...`]` pair is found it logs
  `logger.warning("No JSON array found in LLM response: %s", raw[:200])`
  and returns `[]`.
- **Post-parse normalization** (applies after a successful parse from
  either tier):
  - `if not isinstance(parsed, list): logger.warning(...); return []`
  - `queries = [str(item) for item in parsed if item]` (drops falsy,
    coerces to str)
  - if `len(queries) > breadth`: log warning, `queries = queries[:breadth]`
  - `return queries`

### 1.2 The gap

Both tiers feed input into the strict `json.loads`. The substring tier
only strips surrounding text — it does NOT fix malformations *inside* the
array. Inputs that defeat BOTH tiers and silently return `[]`:

- Trailing comma: `["a", "b", "c",]`
- Single-quoted keys/strings: `['a', 'b', 'c']`
- Unquoted keys (in object-shaped output): `[{name: a}]`
- Truncated / cut-off output (reasoning model hit token limit):
  `["a", "b", "c"` (no closing bracket, or mid-string cut)
- JSON wrapped in prose / markdown fences where the inner array itself is
  also malformed.

Effect: `parse_sub_queries` returns `[]` → zero child sub-queries → the
node's frontier collapses → degenerate single-node tree. The only signal
is one WARNING log line; there is no tier attribution, so operators
cannot tell standard-failure from repairable-failure from total-failure.

### 1.3 Public contract (unchanged by this amendment)

- `DecomposeRequest` / `DecomposeResponse` Pydantic v2 models
  (`model_config = ConfigDict(extra="forbid")`, `breadth: int =
  Field(ge=1, le=8)`).
- `POST /decompose_query` endpoint returns `DecomposeResponse(sub_queries=...)`.
- `parse_sub_queries(raw, breadth)` signature.

The amendment changes ONLY the internal body of `parse_sub_queries`
(adding a third tier + tier-attribution logging) plus one dependency
line. No model, signature, route, or HTTP-status change.

### 1.4 Other LLM-output parse sites (scope boundary check)

`grep -rn "json.loads" services/researcher/src/researcher/` returns
exactly two hits, both in `deep_tree.py`:

- `deep_tree.py:93` — tier 1.
- `deep_tree.py:100` — tier 2.

`synthesis.py` and `eval_judge.py` were inspected and contain NO
`json.loads` call sites and no other LLM-output JSON parsing that mirrors
this pattern. Conclusion: `deep_tree.py` is the sole site in scope.
`synthesis.py`/`eval_judge.py` are explicitly excluded (spec.md §4,
Open Question OQ-2 RESOLVED).

### 1.5 Existing test file

`services/researcher/tests/test_deep_tree.py` exists (≈176 lines). It
uses `fastapi.testclient.TestClient`, mocks `Gateway.complete` via
`unittest.mock.AsyncMock`, and drives the endpoint. Test classes are
`Test*`-named with `test_*` methods (per `pyproject.toml` pytest config).
The new repair-tier tests target `parse_sub_queries` directly (a pure
function) for tight, table-style coverage, complementing the existing
endpoint-level tests.

### 1.6 Dependency manifest

`services/researcher/pyproject.toml`:
- `[project].dependencies`: fastapi, uvicorn, pydantic, httpx, openai,
  deepeval.
- `[dependency-groups].dev`: ruff, pytest, pytest-asyncio, pytest-cov,
  hypothesis, pip-audit.
- Build system: `hatchling` (no `[tool.uv]` section). Verified there is
  NO `uv.lock` for this service (`find services/researcher -name uv.lock`
  returns nothing) — `uv.lock` is N/A for `services/researcher`. (The
  repo's only recent uv.lock work was for `youtube-extract`, a different
  service.) The service's actual dependency-management/install flow is
  confirmed at run phase.
- ruff line-length 120; pytest `asyncio_mode = "auto"`.

The amendment adds one line to `[project].dependencies` in
`pyproject.toml`: `json-repair` (runtime, not dev — it executes on the
hot path).

---

## 2. Port Source Evidence

### 2.1 assafelovic/gpt-researcher v3.5 (PR #1773)

The port source added the `json-repair` library plus a regex extraction
fallback as additional tiers for parsing malformed LLM JSON, motivated by
reasoning models that emit non-strict JSON (trailing commas, prose
wrapping, truncation). This amendment adopts the `json-repair` tier ONLY;
the regex fallback is deliberately excluded (spec.md §4) to keep the
change minimal — `json-repair` already handles the malformation classes
the regex tier targeted.

### 2.2 json-repair PyPI package (verified 2026-06-17)

- Import name: `json_repair`. Primary function: `json_repair.loads(s)`.
- Behavior: a drop-in replacement for `json.loads` — it first tries the
  standard-library loader and only falls back to the repair parser when
  strict parsing fails. In its default (non-strict) mode it returns a
  best-effort value and an empty value when the input is unrecoverable,
  rather than raising. A `strict=True` mode exists that raises
  `ValueError` on structural issues (NOT used here — the amendment relies
  on the non-raising default to satisfy REQ-DEEP3a-103 no-raise).
- Handles: missing quotes, commas, brackets; comments; stray prose;
  truncated values; single-quoted strings; unquoted keys.
- Latest version at research time: 0.61.0 (released 2026-06-16). The
  `loads` API has been stable across the 0.x series.

Source: `https://pypi.org/project/json-repair/`.

---

## 3. Design Decisions (pinned for spec.md)

1. **Third tier only** — standard → substring → repaired → `[]`. Tiers 1
   and 2 untouched; repair is purely additive and reached only on their
   failure. Preserves byte-identical output for already-passing inputs
   (REQ-DEEP3a-104).
2. **Non-raising default** — call `json_repair.loads(...)` in default
   mode; wrap defensively so any unexpected exception still yields `[]`
   (REQ-DEEP3a-103).
3. **Shared post-parse normalization** — the repaired result flows
   through the SAME list-check + `str()` coercion + breadth truncation as
   tiers 1/2 (REQ-DEEP3a-102). No separate code path for repaired output.
4. **Tier-attribution logging** — log `standard` / `substring` /
   `repaired` / `failed` exactly once per call; `repaired` and `failed`
   at WARNING level (REQ-DEEP3a-201/202).
5. **Runtime dependency** — `json-repair` goes in `[project].dependencies`
   (executes on the hot path), not dev deps (REQ-DEEP3a-301).
6. **Repair-tier input = raw** (OQ-3 default) — feed the original `raw`
   to `json_repair.loads` (it handles prose/fences itself), confirmed at
   run phase; acceptance §A5 constrains the choice.

---

## 4. Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| `json-repair` over-repairs and returns a plausible-but-wrong list | Low | Medium | Repaired output still passes through list-check + `str()` + breadth truncation; downstream tree expansion treats sub-queries as best-effort hints, not authoritative. WARNING log surfaces reliance for review. |
| New runtime dependency increases sidecar image size / supply-chain surface | Low | Low | Single pure-Python package, widely used; bounded pin `>=0.61,<1.0`; `pip-audit` already in dev deps gates known CVEs. |
| Repair tier masks a genuine prompt regression (model stops returning JSON) | Medium | Low | WARNING-level `repaired` log (REQ-DEEP3a-202) makes recurring repair reliance observable; not silently swallowed. |
| json-repair default behavior changes in a future release (starts raising) | Low | Medium | REQ-DEEP3a-103 wraps the call defensively (catch-all → `[]`); bounded version pin `>=0.61,<1.0`; acceptance §A8 asserts no-raise. |
| json-repair recovery *shape* drifts across versions, breaking exact-output ACs (A3/A4/A5) | Medium | Medium | Bounded pin `>=0.61,<1.0` (OQ-1/OQ-4) caps drift to the validated release line; on failure after a dependency bump, relax A5 to a bounded `len>=N`+membership assertion rather than loosening the pin. |

---

## 5. References

- `.moai/specs/SPEC-DEEP-003/spec.md` — parent SPEC.
- `services/researcher/src/researcher/deep_tree.py` — amended function.
- `services/researcher/tests/test_deep_tree.py` — existing tests.
- `services/researcher/pyproject.toml` — dependency manifest.
- `https://github.com/assafelovic/gpt-researcher` (PR #1773) — port source.
- `https://pypi.org/project/json-repair/` — library.
