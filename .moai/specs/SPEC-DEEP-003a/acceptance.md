# SPEC-DEEP-003a Acceptance Scenarios

Concrete test scenarios for the `json-repair` third parse tier. Tests
target `parse_sub_queries(raw, breadth)` directly (a pure function) for
tight table-style coverage, in
`services/researcher/tests/test_deep_tree.py`. Each scenario maps to one
or more REQ-DEEP3a-* requirements.

Coverage target: 85% of `parse_sub_queries` and the new repair branch.

---

## Test harness conventions

- Import under test: `from researcher.deep_tree import parse_sub_queries`.
- Pure-function tests call `parse_sub_queries(raw, breadth)` with crafted
  malformed strings — no `Gateway` mock or `TestClient` needed for the
  parse-tier cases.
- Logging assertions use pytest's `caplog` fixture (`caplog.set_level`,
  inspect `caplog.records` for `levelname` and message substring).
- Test classes follow the existing `Test*` / `test_*` convention
  (`pyproject.toml` pytest config); `asyncio_mode = "auto"`.

---

## A1 — Trailing comma inside array recovers (REQ-DEEP3a-101, 102)

**Given** an LLM response with a trailing comma inside the array that
defeats both `json.loads` and the substring tier:
`raw = '["alpha", "beta", "gamma",]'`, `breadth = 4`.

**When** `parse_sub_queries(raw, 4)` is called.

**Then** it returns `["alpha", "beta", "gamma"]` (3 items, recovered via
the repair tier). The result is a non-empty list of strings.

Test: `test_repair_recovers_trailing_comma`.

---

## A2 — Single-quoted keys/strings recover (REQ-DEEP3a-101, 102)

**Given** `raw = "['alpha', 'beta', 'gamma']"` (single quotes — invalid
strict JSON), `breadth = 4`.

**When** `parse_sub_queries(raw, 4)` is called.

**Then** it returns `["alpha", "beta", "gamma"]` via the repair tier.

Test: `test_repair_recovers_single_quotes`.

---

## A3 — Unquoted keys recover (REQ-DEEP3a-101)

**Given** an array of objects with unquoted keys, as a reasoning model
might emit: `raw = '[{query: alpha}, {query: beta}]'`, `breadth = 4`.

**When** `parse_sub_queries(raw, 4)` is called.

**Then** it returns a non-empty list (length ≥ 1) of `str` items via the
repair tier (json-repair recovers the structure; each element is coerced
via `str()`). Assert `len(result) >= 1` and every element is a `str` and
no exception is raised.

Test: `test_repair_recovers_unquoted_keys`.

---

## A4 — Truncated / cut-off array recovers (REQ-DEEP3a-101)

**Given** a response cut off by a token limit (no closing bracket):
`raw = '["alpha", "beta", "gamm'`, `breadth = 4`.

**When** `parse_sub_queries(raw, 4)` is called.

**Then** it returns a non-empty list containing at least the complete
leading elements (e.g. `["alpha", "beta"]` or `["alpha", "beta", "gamm"]`
depending on json-repair's recovery), all `str`, via the repair tier.
Assert `len(result) >= 2` and `"alpha"` and `"beta"` are present.

Test: `test_repair_recovers_truncated_array`.

---

## A5 — JSON wrapped in prose / markdown fences recovers (REQ-DEEP3a-101)

**Given** a fenced, prose-wrapped response whose inner array is also
malformed (trailing comma), so the substring tier extracts an invalid
slice:
```
raw = "Here are the sub-queries:\n```json\n[\"alpha\", \"beta\",]\n```\nHope this helps!"
```
`breadth = 4`.

**When** `parse_sub_queries(raw, 4)` is called.

**Then** it returns `["alpha", "beta"]` via the repair tier (constrains
OQ-3: the chosen repair-tier input must recover the array from
fence/prose-wrapped malformed input).

Test: `test_repair_recovers_prose_and_fences`.

---

## A6 — Repaired result truncated to breadth (REQ-DEEP3a-102)

**Given** a malformed array (single quotes) with MORE than `breadth`
elements: `raw = "['a', 'b', 'c', 'd', 'e', 'f']"`, `breadth = 4`.

**When** `parse_sub_queries(raw, 4)` is called.

**Then** it returns exactly 4 items (`["a", "b", "c", "d"]`) — the
repaired result passes through the same breadth truncation as tiers 1/2.

Test: `test_repair_result_truncated_to_breadth`.

---

## A7 — Totally unparseable garbage still returns [] (REQ-DEEP3a-103)

**Given** input that no tier can turn into a list of sub-queries:
`raw = "this is not json at all, no brackets, nothing useful"`,
`breadth = 4`.

**When** `parse_sub_queries(raw, 4)` is called.

**Then** it returns `[]` (no regression vs parent SPEC). No exception is
raised.

Test: `test_unparseable_returns_empty`.

Additional sub-case: `raw = ""` (empty string) → `[]`.
Test: `test_empty_string_returns_empty`.

---

## A8 — Repair tier is exception-safe / never raises (REQ-DEEP3a-103)

**Given** a parametrized set of pathological inputs (deeply nested junk,
binary-ish noise, a lone `[`, a non-list JSON scalar like `"42"` or
`'"just a string"'`), each with `breadth = 4`.

**When** `parse_sub_queries(raw, 4)` is called for each.

**Then** every call returns a `list` (possibly `[]`) and NONE raises.
Assert via `pytest.raises` NOT being triggered — i.e. wrap each call and
assert it completes and `isinstance(result, list)`.

Test: `test_repair_tier_never_raises` (parametrized).

---

## A9 — Clean JSON still parses at tier 1; repair not invoked (REQ-DEEP3a-104)

**Given** well-formed JSON: `raw = '["alpha", "beta", "gamma", "delta"]'`,
`breadth = 4`.

**When** `parse_sub_queries(raw, 4)` is called.

**Then** it returns `["alpha", "beta", "gamma", "delta"]` (byte-identical
to parent behavior) AND the tier-attribution log records `standard`
(NOT `repaired`) — proving the repair tier was not reached.

Test: `test_clean_json_uses_standard_tier`.

Sub-case (tier 2): prose-wrapped but internally VALID array
`raw = "Sure!\n[\"alpha\", \"beta\"]\nDone."` → returns
`["alpha", "beta"]` and logs `substring`.
Test: `test_valid_array_in_prose_uses_substring_tier`.

---

## A10 — Tier-attribution logging (REQ-DEEP3a-201, 202)

**Given** the four representative inputs that exercise each tier:
- standard: `'["a", "b"]'`
- substring: `'noise ["a", "b"] noise'`
- repaired: `'["a", "b",]'` (trailing comma)
- failed: `'not json'`

**When** `parse_sub_queries` is called for each with `caplog` capturing
at DEBUG level.

**Then**:
- exactly ONE tier-attribution log record is emitted per call;
- the record message contains the correct literal tier identifier
  (`standard` / `substring` / `repaired` / `failed`);
- the `repaired` and `failed` records are at WARNING level
  (`record.levelname == "WARNING"`);
- the `standard` and `substring` records are at a non-warning level
  (INFO or DEBUG).

Tests: `test_log_tier_standard`, `test_log_tier_substring`,
`test_log_tier_repaired_is_warning`, `test_log_tier_failed_is_warning`,
`test_exactly_one_tier_log_per_call`.

---

## A11 — Dependency declared and importable (REQ-DEEP3a-301)

**Given** the amended `services/researcher/pyproject.toml`.

**When** the test environment imports the module.

**Then**:
- `import json_repair` succeeds (the package is installed via the
  declared dependency);
- `json_repair.loads` is callable;
- `pyproject.toml` `[project].dependencies` contains an entry whose
  normalized name is `json-repair` (assert by reading and parsing the
  manifest, or by `importlib.metadata.version("json-repair")` returning a
  value).

Tests: `test_json_repair_importable`,
`test_pyproject_declares_json_repair`.

---

## Definition of Done

- [ ] All A1-A11 tests pass under `pytest services/researcher/tests/test_deep_tree.py`.
- [ ] Existing endpoint tests in `test_deep_tree.py` still pass (no contract regression).
- [ ] `ruff check` and `ruff format --check` clean (line-length 120).
- [ ] Coverage of `parse_sub_queries` (incl. repair branch) ≥ 85% via `pytest --cov=researcher.deep_tree`.
- [ ] `json-repair` present in `pyproject.toml` `[project].dependencies` (bounded pin per OQ-1). `uv.lock` is N/A for this service (build-system `hatchling`, no `[tool.uv]`); nothing to regenerate.
- [ ] No file outside the four listed in plan.md is modified.

---

## REQ → Scenario Coverage Matrix

| REQ | Scenarios |
|-----|-----------|
| REQ-DEEP3a-101 (repair tier attempted) | A1, A2, A3, A4, A5 |
| REQ-DEEP3a-102 (shared normalization + truncation) | A1, A2, A6 |
| REQ-DEEP3a-103 (no-raise, no-regression) | A7, A8 |
| REQ-DEEP3a-104 (tiers 1/2 unchanged, repair not invoked early) | A9 |
| REQ-DEEP3a-201 (tier-attribution log, one per call) | A10 |
| REQ-DEEP3a-202 (repaired/failed at WARNING) | A10 |
| REQ-DEEP3a-301 (dependency declared + importable) | A11 |
