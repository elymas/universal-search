# SPEC Review Report: SPEC-DEEP-003a
Iteration: 1/3
Verdict: PASS-WITH-FINDINGS
Overall Score: 0.86

> Reasoning context ignored per M1 Context Isolation. The author's summary
> (REQ counts, "json-repair verified", "deep_tree.py is the only json.loads
> site") was treated as a claim to disprove, not as fact. All code-state
> claims were independently verified against the repository via Grep/Read.

## Must-Pass Results

- **[PASS] MP-1 REQ number consistency**: Grouped/themed scheme
  (10x parse, 20x observability, 30x dependency), consistent with the parent
  SPEC-DEEP-003's REQ-DEEP3-NNN convention. IDs: REQ-DEEP3a-101, 102, 103, 104
  (spec.md:L127, L136, L145, L154), 201, 202 (spec.md:L164, L172), 301
  (spec.md:L183). Within each group: 101-104 sequential, 201-202 sequential,
  301 — no gaps, no duplicates, consistent 3-digit zero-padding. PASS.

- **[PASS] MP-2 EARS format compliance**: Every REQ is labeled with its EARS
  pattern and matches it, with inline REQ-IDs and AC back-references:
  - REQ-101 Event-Driven: "WHEN ... cannot be parsed ... the system SHALL
    attempt repair" (spec.md:L127-134) ✓
  - REQ-102 State-Driven: "WHILE the repair tier is producing a value, the
    system SHALL apply the SAME post-parse normalization" (spec.md:L136-143) ✓
  - REQ-103 Unwanted: "IF the repair tier raises ... THEN the system SHALL
    return `[]`" (spec.md:L145-152) ✓
  - REQ-104 Ubiquitous: "The system SHALL preserve the existing tier-1 ...
    behavior" (spec.md:L154-160) ✓
  - REQ-201 Ubiquitous: "The system SHALL log which parse tier produced the
    result" (spec.md:L164-170) ✓
  - REQ-202 Event-Driven: "WHEN the repaired tier succeeds ... the system
    SHALL emit the log at ... WARNING level" (spec.md:L172-179) ✓
  - REQ-301 Ubiquitous: "The system SHALL declare `json-repair` as a runtime
    dependency" (spec.md:L183-189) ✓
  No double-negatives; no informal language; no Given/When/Then mislabeled as
  EARS (the GWT scenarios live correctly in acceptance.md). PASS.

- **[PASS] MP-3 YAML frontmatter validity** (against project schema):
  spec.md:L1-16 carries id, title, version, status, created, updated, author,
  priority, owner, methodology, coverage_target, issue_number, milestone,
  depends_on. This matches the established repo SPEC schema **exactly** —
  verified against parent SPEC-DEEP-003 (same field set). `created` (ISO date,
  L6) is this project's convention in place of the generic `created_at`.
  See MEDIUM finding D3: the generic auditor schema's `labels`/`created_at`
  fields are absent, but their absence is consistent across every SPEC in the
  repo. Failing a project-consistent, internally-valid frontmatter would be a
  false positive, so MP-3 is PASS with an advisory.

- **[N/A] MP-4 Section 22 language neutrality**: N/A — single-language scope.
  This SPEC amends one Python function in the `services/researcher` sidecar
  (spec.md:L46-48, L297). No multi-language tooling claims; no language is
  enumerated as primary. Auto-pass.

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.90 | 0.75-1.0 | Each REQ has a single unambiguous reading; tier ordering pinned (spec.md:L102-117); no pronoun ambiguity. Minor: "6 EARS REQs" self-description (L54) contradicts the 7 actual REQs (D1). |
| Completeness | 0.85 | 0.75-1.0 | All sections present: HISTORY (L20), Overview/WHY+WHAT (L61), architecture/HOW (L102), EARS REQs (L120), Acceptance summary (L193), Exclusions (L217, 7 entries), Open Questions (L263), References (L292). Companion research.md/plan.md/acceptance.md present. Knocked for the uv.lock current-state inaccuracy (D2) and missing version-range OQ (D4). |
| Testability | 0.88 | 0.75-1.0 | A1/A2/A6/A9/A11 exact-output binary (acceptance.md:L34, L48, L113, L161, L206). A3/A4 use bounded assertions (len>=1 / len>=2 + membership) to absorb json-repair variance — testable. No weasel words in normative ACs. A5 exact-match is brittle vs unbounded version range (see D4). |
| Traceability | 1.00 | 1.0 | Every REQ has >=1 AC; every AC maps to an existing REQ. Forward map (spec.md:L198-210) and reverse matrix (acceptance.md:L233-241) agree: 101→A1-A5, 102→A1/A2/A6, 103→A7/A8, 104→A9, 201/202→A10, 301→A11. No orphan ACs, no uncovered REQs. |

## Code-State Verification (factual accuracy — independently confirmed)

The SPEC makes many specific claims about existing code. I verified each; the
SPEC is **accurate** on all of the following (no invented symbols):

- `services/researcher/src/researcher/deep_tree.py` exists; `parse_sub_queries`
  at **L84** (spec claims ~84-123; actual 84-123) ✓
- `json.loads` tier 1 at **deep_tree.py:93**, tier 2 at **deep_tree.py:100** —
  exactly as research.md:L70-71 claims ✓
- Two-tier strategy, `logger.warning` on failure, `return []` fallback,
  post-parse normalization (`[str(item) for item in parsed if item]`, breadth
  truncation) — all match actual L92-123 ✓
- `synthesis.py` and `eval_judge.py` exist and contain **NO** `json.loads`
  (grep of `services/researcher/src/researcher/` returns only the 2 deep_tree
  hits) — OQ-2 RESOLVED claim is correct ✓
- `pyproject.toml` deps (fastapi, uvicorn, pydantic, httpx, openai, deepeval)
  and dev group (ruff, pytest, pytest-asyncio, pytest-cov, hypothesis,
  pip-audit), ruff line-length 120, `asyncio_mode=auto`, `python_classes=Test*`
  — all match actual pyproject.toml ✓
- Go consumer claim: `internal/deepagent/researcher_http.go:37`
  `SubQueries []string json:"sub_queries"`; `tree.go` `Decompose(...)` — Go
  side consumes the parsed list and is unaffected ✓
- `json-repair` resolves on PyPI (HTTP 200) ✓
- coverage_target 85 == quality.yaml `test_coverage_target` (85); methodology
  tdd == quality.yaml `development_mode` (tdd) ✓
- Parent SPEC-DEEP-003 status is `implemented`, so `depends_on:[SPEC-DEEP-003]`
  is satisfied ✓
- plan.md fan_in claim for `parse_sub_queries` ("fan_in is 1") is correct —
  sole caller is `decompose_query` at deep_tree.py:152 ✓

## Defects Found

**D1. spec.md:L54 — REQ self-count is wrong. — Severity: major**
HISTORY states "6 EARS REQs (4 × P1 + 2 × P2)". Actual count is **7 REQs**:
P1 = REQ-101, 102, 103, 104, **301** (five, not four); P2 = REQ-201, 202 (two).
So both the total (6→7) and the P1 count (4→5) are incorrect — REQ-DEEP3a-301
(the dependency REQ, also P1) is omitted from the tally. An auditor or
implementer relying on the count could miss REQ-301. The author's own report
repeated this ("6 EARS REQs"). Fix to "7 EARS REQs (5 × P1 + 2 × P2)".

**D2. research.md:L96 / plan.md:L16 / acceptance.md:L226 — `uv.lock`
current-state claim is inaccurate. — Severity: medium**
research.md:L96 asserts "uv.lock present in repo per recent commits"; plan.md
marks `[REGENERATE] services/researcher/uv.lock`; acceptance DoD says "uv.lock
regenerated". Verified: **`find services/researcher -name uv.lock` returns
nothing** — there is no uv.lock for this service, and the build-system is
`hatchling` with no `[tool.uv]` section. (The repo's only recent uv.lock work
was for `youtube-extract`, a different service — likely the source of the
confusion.) Consequence: `uv add json-repair` would *create* a lock, not
*regenerate* one, and it is unverified that `uv` is even this service's package
manager. Reclassify the step as "[CREATE-OR-N/A] uv.lock" and confirm the
service's actual dependency-management flow at run phase.

**D3. spec.md:L1-16 — generic-schema fields `labels`/`created_at` absent. —
Severity: medium (advisory)**
Against the generic SPEC frontmatter schema, `labels` is missing and the field
is `created` rather than `created_at`. This is consistent with every SPEC in
the repo (verified vs parent SPEC-DEEP-003), so it is NOT treated as a must-pass
failure. Recorded only so the team can decide whether to align the project
schema with the generic auditor schema, or accept the divergence permanently.

**D4. spec.md:L265-271 / acceptance.md:L54-102 — missing Open Question on
version-range vs exact-recovery brittleness. — Severity: minor**
OQ-1 pins only a lower bound (`json-repair>=0.30`, validated against 0.61.0)
with no upper bound, while A3 (unquoted keys), A4 (truncation) and especially
A5 (exact `["alpha","beta"]` from fenced+malformed input) assert *specific*
repair output. json-repair's recovery *shape* can shift within an unbounded
version range, which could break these ACs without any code change. The risk
table (research.md:L160-165) covers "library starts raising" (via the no-raise
wrapper) but not "recovery output changes shape." Add an OQ/risk and consider a
compatible-release pin (`~=`) or upper bound, or relax A5 to a bounded
assertion like A4.

## Chain-of-Verification Pass

Second-look findings (re-read spec.md §2 every REQ, §4 Exclusions, acceptance
matrix, and cross-checked for contradictions):
- Re-read all 7 REQs individually — all EARS-valid, all carry inline IDs and AC
  refs. Confirmed.
- REQ sequencing checked end-to-end (not spot-checked): 101-104 / 201-202 / 301,
  no gaps/dupes/padding drift.
- Traceability verified for **every** REQ against both the forward table
  (spec.md:L198-210) and the reverse matrix (acceptance.md:L233-241) — they
  agree; no orphans, no uncovered REQs.
- Exclusions (spec.md:L217-247) checked for specificity, not just presence: 7
  concrete entries, each naming the excluded item + rationale (regex tier,
  synthesis/eval_judge sites, prompt redesign, response_format, Prometheus
  metric, Go changes, LLM retry). Genuinely specific.
- Contradiction sweep: REQ-201/202 logging levels (spec.md:L164-179) vs
  plan.md:L83-85 (info for standard/substring, warning for repaired/failed) vs
  acceptance A10 (acceptance.md:L185-192) — all mutually consistent. No
  contradiction.
- New finding surfaced on second pass: **D1** (the REQ self-count) and the
  exact magnitude of **D2** (uv.lock absence) were confirmed here — the first
  pass flagged them; the second pass quantified P1=5 and the empty `find`.

## Recommendation

PASS-WITH-FINDINGS. The SPEC is unusually well-grounded: every code-state claim
was independently verified true (file paths, line numbers, the json.loads
sites, the no-json.loads sibling files, the Go consumer, pyproject contents,
quality.yaml alignment, parent dependency status). EARS is clean, traceability
is perfect (1.00), scope is tight with specific exclusions, and there is no
over-engineering (single library tier, regex deliberately excluded).

No must-pass criterion fails. The findings are non-blocking but should be fixed
before/at run phase:

1. **D1 (major)** — spec.md:L54: change "6 EARS REQs (4 × P1 + 2 × P2)" to
   "7 EARS REQs (5 × P1 + 2 × P2)". REQ-DEEP3a-301 is P1 and was dropped from
   the count.
2. **D2 (medium)** — Correct the uv.lock claims (research.md:L96, plan.md:L16,
   acceptance.md:L226): no uv.lock exists for `services/researcher`. Mark the
   lock step CREATE-or-N/A and confirm the service's real package-management
   flow (build-system is hatchling; no `[tool.uv]`).
3. **D4 (minor)** — Add an OQ/risk for json-repair version-range vs the exact
   recovery assertions in A3/A4/A5; consider `~=` pin or relax A5.
4. **D3 (advisory)** — Decide whether to keep the repo's `created`/no-`labels`
   frontmatter convention or align with the generic schema; no action required
   for this SPEC if the convention stands.

---
*plan-auditor iteration 1/3 — SPEC-DEEP-003a*
