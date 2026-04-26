# SPEC Review Report: SPEC-ADP-001

Iteration: 1/3
Verdict: **FAIL**
Overall Score: 0.78

Reasoning context ignored per M1 Context Isolation. The four SPEC files are
treated as the only inputs; pkg/types/* and internal/adapters/registry.go
were consulted only to verify factual citations.

---

## Must-Pass Results

- **[PASS] MP-1 REQ number consistency**: REQ-ADP-001 through REQ-ADP-010 are
  sequential, no gaps, no duplicates, consistent zero-padding (3-digit).
  Evidence: spec.md:L327–L336 (table rows), spec-compact.md:L11–L20
  (one-line summary mirror), acceptance.md headings S-001 through S-010,
  plan.md task→REQ links L44, L55, L66, L85, L97, L111, L124, L140,
  L163, L179, L197, L214, L237.

- **[FAIL] MP-2 EARS format compliance**: Two issues.
  1. The spec self-attests in HISTORY (spec.md:L77–L78) that the SPEC covers
     "all five EARS patterns (Ubiquitous, Event-Driven, State-Driven,
     Optional, Unwanted)". Inspection of the §3 table (spec.md:L327–L336)
     shows only **four** patterns are used:
     - Ubiquitous: REQ-001, REQ-006, REQ-009 (3)
     - Event-Driven: REQ-002, REQ-003, REQ-004, REQ-005 (4)
     - Optional: REQ-007, REQ-010 (2)
     - Unwanted: REQ-008 (1)
     - **State-Driven (WHILE …, the system shall …): zero**
     The HISTORY claim is factually false; the spec is missing one of the
     five EARS patterns.
  2. Each individual REQ does match an EARS pattern syntactically
     (Ubiquitous "shall …", Event-Driven "WHEN …", Optional "WHERE …",
     Unwanted "IF … THEN …"), so no REQ is malformed. The failure is
     coverage-of-pattern-set, not per-REQ malformation.
  Evidence: spec.md:L77–L78 (false claim), spec.md:L325–L336 (the table
  that disproves it).

- **[PASS] MP-3 YAML frontmatter validity**: All required fields present
  with correct types. id (string, "SPEC-ADP-001"), version ("0.1.0"),
  status ("draft"), created (date "2026-04-26") — note this project uses
  `created` not `created_at`, consistent with SPEC-CORE-001 and SPEC-IR-001
  conventions, so accepting per project calibration. priority ("P0"),
  labels — absent but `methodology`, `coverage_target`, `owner`, `milestone`,
  `depends_on`, `blocks` are present and consistent with the project
  convention. Evidence: spec.md:L1–L17.

- **[PASS] MP-4 Section 22 language neutrality**: N/A. SPEC is scoped to a
  single Go-language adapter; no template-bound multi-language tooling.
  No language-server names hardcoded. Auto-passes.

**Net must-pass: 1 FAIL → overall verdict FAIL** regardless of other
dimension scores (M5 firewall).

---

## Category Scores (rubric-anchored)

| Dimension      | Score | Rubric Band | Evidence |
|----------------|-------|-------------|----------|
| Clarity        | 0.85  | 0.75 (lower edge of 0.75 band) | Most REQs unambiguous (spec.md:L327–L336); a few sketch/acceptance gaps (Healthcheck test seam, NFR-001 p50 measurement mechanism). |
| Completeness   | 0.80  | 0.75 band | All required sections present (HISTORY L21, Purpose L93, Scope L166, EARS L323, NFRs L340, Acceptance L350, Technical L514, Exclusions L801, TDD L851, Dependencies L921, Risks L990, Open Q L1009, References L1067). HISTORY metadata wrong (see defects). |
| Testability    | 0.80  | 0.75 band | Most ACs are binary-testable. NFR-ADP-001 p50 assertion mechanism unspecified; Healthcheck test seam unspecified. |
| Traceability   | 0.95  | 1.0 band | Every REQ has ≥1 test (see coverage matrix below); every test names a REQ. No orphans. |

---

## Defects Found

### D1 [BLOCKER] — Missing State-Driven EARS pattern; HISTORY misrepresents pattern coverage

- **Location**: spec.md:L77–L78 (false claim) and spec.md:L325–L336 (table proving it false).
- **Description**: HISTORY says "covering all five EARS patterns (Ubiquitous,
  Event-Driven, State-Driven, Optional, Unwanted)". The §3 table contains
  zero State-Driven (`WHILE <condition>, the <system> SHALL <response>`)
  requirements. Pattern coverage is 4/5 not 5/5.
- **Evidence**: spec.md:L77 quotes the claim verbatim. The §3 table
  patterns are: Ubiquitous (REQ-001, 006, 009), Event-Driven (REQ-002,
  003, 004, 005), Optional (REQ-007, 010), Unwanted (REQ-008). No row
  uses "WHILE".
- **Recommended fix**: Either (a) drop the false claim from HISTORY and
  acknowledge "covers four of five EARS patterns; State-Driven not
  applicable to a stateless adapter — adapter holds no per-call state
  whose presence would warrant a `WHILE` clause"; OR (b) add a genuine
  State-Driven REQ such as: "REQ-ADP-011 (State-Driven): WHILE the
  caller's `ctx` has not been cancelled, the adapter SHALL stream the
  HTTP response body to `parseListing` and return the parsed result; if
  ctx is cancelled mid-read, the partial body SHALL be discarded and
  `ctx.Err()` returned wrapped in `*SourceError{CategoryUnavailable}`."
  (a) is cheaper and honest; (b) adds genuine value if ctx-mid-flight
  semantics deserve a contractual statement separate from REQ-005.

### D2 [BLOCKER] — HISTORY misstates REQ count and priority distribution

- **Location**: spec.md:L77 ("8 EARS REQs (5 × P0 + 2 × P0/P1 + 1 × P1)").
- **Description**: The §3 table contains 10 REQs, not 8. Counting
  priorities: REQ-001 P0, 002 P0, 003 P0, 004 P0, 005 P0, 006 P0,
  007 P1, 008 P0, 009 P0, 010 P1 = **8 × P0 + 2 × P1**, not "5 × P0 +
  2 × P0/P1 + 1 × P1". §6.8 (spec.md:L792–L796) compounds the
  inconsistency by claiming "10 EARS REQs (7 × P0 + 3 × P1)" — also
  wrong (should be 8 P0 + 2 P1). Three different numbers for the same
  fact across one document.
- **Evidence**: spec.md:L77 vs spec.md:L327–L336 vs spec.md:L792.
- **Recommended fix**: Replace HISTORY line with "10 EARS REQs
  (8 × P0 + 2 × P1) covering four EARS patterns…" (paired with D1 fix).
  Update §6.8 to "10 EARS REQs (8 × P0 + 2 × P1)".

### D3 [BLOCKER] — HISTORY misstates test count

- **Location**: spec.md:L79 ("~30 representative TDD tests").
- **Description**: spec.md §8 (TDD Plan, L862–L907) enumerates 45 tests
  (rows numbered 1 through 45). Plan.md §8 quality gate (L413) similarly
  references "All 45 tests in §8 of spec.md". The "~30" figure in HISTORY
  is contradicted by the spec's own enumerated table.
- **Evidence**: spec.md:L862–L907 (numbered 1–45), plan.md:L413.
- **Recommended fix**: Update HISTORY to "~45 representative TDD tests"
  (or whatever the recount yields after D1 fix is applied).

### D4 [MAJOR] — Healthcheck acceptance contradicts implementation sketch; test seam unspecified

- **Location**: spec.md:L376–L378 (acceptance) vs spec.md:L666–L675
  (sketch) vs plan.md:L57–L66 (Task A3).
- **Description**: REQ-ADP-001 acceptance says "(*Adapter).Healthcheck(ctx)
  succeeds against an httptest.Server binding `127.0.0.1:0` (the test
  substitutes the dial target via test-only configuration)". The §6.4
  sketch hardcodes `d.DialContext(ctx, "tcp", "www.reddit.com:443")`
  with no override mechanism. The `Options` struct (spec.md:L622–L626)
  has no `DialTarget` / `HealthcheckHost` field. plan.md A3 says "package-
  level test seam (if cleaner)" — non-committal. The test cannot be
  written as the acceptance criterion describes; it requires an Options
  field or package-level variable that the spec does not define.
- **Evidence**: spec.md:L378 (acceptance asserts a test-only override),
  spec.md:L622–L626 (Options has no such field), spec.md:L666–L675 (sketch
  hardcodes the URL), plan.md:L62–L65 ("test seam" deferred to "if cleaner").
- **Recommended fix**: Add an `Options.HealthcheckTarget string` (default
  `"www.reddit.com:443"`) field to the §6.4 sketch and document it in
  REQ-ADP-001. Alternatively, replace the acceptance scenario with a
  documented behavior test (e.g., S-001-F already covers ctx cancellation;
  drop the loopback substitution requirement and rely on
  `-tags=integration` for live healthcheck). Either path closes the gap;
  the current text leaves the implementer to invent the seam.

### D5 [MAJOR] — NFR-ADP-001 p50 assertion mechanism unspecified

- **Location**: spec.md:L344 (NFR-ADP-001), spec.md:L494–L498 (acceptance),
  plan.md:L260–L262 (Task E3), plan.md:L418 (quality gate).
- **Description**: NFR says "p50 ≤ 5 ms on a 25-doc Listing fixture
  (the `search_response.json` golden, ~5KB) on amd64". `go test -bench`
  reports the per-op average (or median over -count runs), not a
  distribution. The acceptance criterion says "asserted via the CI
  bench-comparison script established by SPEC-OBS-001 NFR-OBS-001"
  but does not name the script, command, or pass/fail mechanism. A
  human operator cannot run the SPEC and decide PASS/FAIL without
  external knowledge of an unspecified script.
- **Evidence**: spec.md:L344, spec.md:L494–L496, plan.md:L260.
- **Recommended fix**: Either (a) cite the exact CI script path
  (`scripts/bench-compare.sh` or similar) and the command-line invocation
  that asserts p50, OR (b) replace "p50 ≤ 5 ms" with "mean ≤ 5 ms over
  `go test -bench=BenchmarkParseListing25Docs -benchtime=10x -count=1`
  on amd64". Option (b) is testable today; option (a) requires a
  documented script.

### D6 [MAJOR] — REQ-ADP-006 Metadata key list inconsistent within spec

- **Location**: spec.md:L332 (REQ-ADP-006 — required keys), spec.md:L599
  (mapping table — extended keys), spec.md:L446–L448 (acceptance — 6 keys),
  spec.md:L1058–L1062 (Open Q §11.7 — public vs implementation keys).
- **Description**: REQ-ADP-006 says Metadata MUST contain
  `{subreddit, over_18, num_comments, upvote_ratio, external_url, kind}`
  (6 keys). §6.3 mapping table says Metadata contains
  `{subreddit, subreddit_name_prefixed, num_comments, upvote_ratio, ups,
  over_18, spoiler, locked, stickied, link_flair_text, post_hint,
  external_url, kind}` (13 keys). Open Q §11.7 distinguishes 7
  "public" keys from 7 "implementation" keys. The ambiguity is open by
  the author's own admission, but REQ-ADP-006 nonetheless states a
  6-key minimum that the implementer must guarantee. Tests S-006-H
  and test #25 assert the 6-key minimum, so the contractual surface
  is at least clear there. The risk is downstream consumers reading
  the §6.3 table will assume the 13-key set is guaranteed.
- **Evidence**: spec.md:L332 (6 keys), spec.md:L599 (13 keys),
  spec.md:L1058–L1062 (Open Q acknowledges the gap).
- **Recommended fix**: In §6.3 mark the 6 contractual keys as REQUIRED
  and the remaining 7 as OPTIONAL ("subject to change without major
  bump"). Move Open Q §11.7's classification into §6.3 inline so the
  document does not contradict itself in two places.

### D7 [MINOR] — Spec.md HISTORY says noop adapter is "47 LoC"; file is 46 lines

- **Location**: spec.md:L108 ("`internal/adapters/noop/noop.go`, 47 LoC").
- **Description**: The noop file ends at line 46. Off-by-one. Trivial but
  the audit must note factual claims that don't survive verification.
- **Evidence**: `internal/adapters/noop/noop.go` total line count = 46.
- **Recommended fix**: Change to "46 lines" or drop the count.

### D8 [MINOR] — REQ-ADP-009 hardcodes a specific GitHub URL not present in any verified source

- **Location**: spec.md:L335 ("`usearch/<version>
  (+https://github.com/elymas/universal-search)`").
- **Description**: The User-Agent literal embeds
  `https://github.com/elymas/universal-search`. The Go module path is
  `github.com/elymas/universal-search` (verified in `go.mod`), so the
  URL matches the module identity. However, no `git remote` or HTTP
  presence at that URL was verified in this audit (and the SPEC
  reasonably should not require URL liveness). The minor concern is that
  if the repository moves, the User-Agent becomes stale and Reddit's
  contact information is misdirected. Reddit specifically asks scrapers
  to identify a working contact path.
- **Evidence**: spec.md:L335 vs go.mod (module path matches; URL liveness
  unverified).
- **Recommended fix**: Make the contact URL a `defaultUserAgentContact
  string` constant (default the current value, override via
  `Options.UserAgentContact`). Add an Open Question to confirm the
  GitHub repo is publicly resolvable before cutting v0.1.

### D9 [MINOR] — REQ-ADP-008 Unicode whitespace test has inconsistent input list

- **Location**: spec.md:L334 (REQ text), spec.md:L461–L465 (acceptance),
  acceptance.md:L334 (Scenario S-008-B).
- **Description**: REQ text says "contains only Unicode whitespace runes
  (per `unicode.IsSpace` over every rune)". Test inputs are
  `["", "   ", "\t\n  \r", " "]` where the last `" "` is annotated
  "(non-breaking space)". Go's `unicode.IsSpace` returns true for U+0020
  and the breaking whitespace set but **false** for U+00A0 NBSP. The
  test as written would fail for NBSP because the implementation per
  spec uses `unicode.IsSpace`, which would NOT classify NBSP as
  whitespace, so the request would be sent (not rejected). Either the
  REQ should say "per `unicode.IsSpace` OR `runeIsSpaceLike` covering
  NBSP" OR the test fixture should not include NBSP.
- **Evidence**: spec.md:L334 (REQ uses `unicode.IsSpace`), spec.md:L463
  ("Text=`" "` (non-breaking space)"), Go stdlib `unicode.IsSpace` source.
- **Recommended fix**: Drop the NBSP test case (unicode.IsSpace returns
  false for NBSP), OR widen the predicate to
  `unicode.In(r, unicode.White_Space)` (which DOES include NBSP) and
  update REQ text accordingly. Pick one and align all three locations.

### D10 [NIT] — Spec.md §6.7 lists the same constants twice as MX targets

- **Location**: spec.md:L780, L783.
- **Description**: Row "score.go::normalizeScore" tags constants as
  `@MX:NOTE`, then row "score.go constants tanhDivisor=100.0,
  scoreCenter=0.5" duplicates the same target. spec-compact.md:L101, L104
  mirrors the duplication.
- **Evidence**: spec.md:L780 vs L783; spec-compact.md:L101 vs L104.
- **Recommended fix**: Collapse the two rows into one or distinguish
  "function" vs "constants" cleanly.

---

## Coverage Matrix: REQ-ADP-* × Acceptance Scenarios × Test Names

| REQ | EARS Pattern | Acceptance Scenarios | Tests (from §8) |
|-----|--------------|----------------------|------------------|
| REQ-ADP-001 | Ubiquitous | S-001-A..F | 1, 2, 3, 4, 5 |
| REQ-ADP-002 | Event-Driven | S-002-A..F | 6, 7, 8, 9, 10 |
| REQ-ADP-003 | Event-Driven | S-003-A..F | 11, 12, 13, 14, 15, 41 (parseRetryAfter table) |
| REQ-ADP-004 | Event-Driven | S-004-A..D | 16 (3 sub-cases for 401/403/404) |
| REQ-ADP-005 | Event-Driven | S-005-A..E | 17, 18, 19 |
| REQ-ADP-006 | Ubiquitous | S-006-A..K | 6 (shared), 20, 21, 22, 23, 24, 25, 26, 27, 39, 40 |
| REQ-ADP-007 | Optional | S-007-A..E | 28, 29, 30, 31 |
| REQ-ADP-008 | Unwanted | S-008-A..C | 32 |
| REQ-ADP-009 | Ubiquitous | S-009-A..D | 33, 34, 35 |
| REQ-ADP-010 | Optional | S-010-A..D | 36, 37, 38 |
| (cross-REQ) | n/a | n/a | 42 (categorizeStatus table covers 003/004/005) |
| NFR-ADP-001 | n/a | N-001 | 45 (BenchmarkParseListing25Docs) |
| NFR-ADP-002 | n/a | N-002 | 43 (TestSearchE2ELatencyStubP95) |
| NFR-ADP-003 | n/a | N-003 | 44 (TestSearchNoGoroutineLeakOnCancel) |

**Orphan REQs**: none.
**Orphan tests**: none. Test #6 covers REQ-002 + REQ-006 (combined happy path)
which is acceptable per the test name table.
**Orphan scenarios**: none. All S-XXX-* scenarios map to REQs by ID prefix.

Coverage matrix is **complete**.

---

## EARS Pattern Coverage Check

| Pattern | Required | Present in SPEC | REQs |
|---------|----------|-----------------|------|
| Ubiquitous (`The X SHALL …`) | ≥1 | YES | REQ-001, 006, 009 |
| Event-Driven (`WHEN …, X SHALL …`) | ≥1 | YES | REQ-002, 003, 004, 005 |
| State-Driven (`WHILE …, X SHALL …`) | ≥1 | **NO** | (none) |
| Optional (`WHERE …, X SHALL …`) | ≥1 | YES | REQ-007, 010 |
| Unwanted (`IF … THEN X SHALL …`) | ≥1 | YES | REQ-008 |

**Result: 4/5 patterns. State-Driven absent.** This violates the audit
PASS criterion ("All 5 EARS patterns represented") and is a BLOCKER
defect (D1). The defect is compounded because spec.md:L77 falsely
claims all 5 are present.

---

## Citation Spot-Check (5 selected file:line citations)

| # | Spec Citation | Verified Against File | Result |
|---|---------------|------------------------|--------|
| 1 | spec.md:L1098 → `pkg/types/adapter.go:28-45` (Adapter interface) | File: `type Adapter interface {` at L28; closing `}` at L45 | **PASS** — exact match |
| 2 | spec.md:L1099 → `pkg/types/capabilities.go:38-62` (Capabilities struct + DocType enum) | File: `type Capabilities struct {` at L38; closing `}` at L62 | **PASS** — struct boundaries match exactly. (Note: DocType enum is actually at L8–L23; the cited range covers Capabilities only, not the enum mentioned in the parenthetical. Minor wording slip, the line range itself is accurate for the struct.) |
| 3 | spec.md:L1102 → `pkg/types/errors.go:14-218` (full taxonomy) | File ends at L218; range covers entire taxonomy section | **PASS** — accurate |
| 4 | spec.md:L1107 → `internal/adapters/registry.go:172-263` (wrappedAdapter sole-emitter) | File: `type wrappedAdapter struct {` at L172; final `}` at L263 | **PASS** — exact match |
| 5 | spec.md:L1110 → `internal/adapters/noop/noop.go:1-46` (reference shape) | File ends at L46 (no L47) | **PASS for citation range**; **FAIL for prose claim** spec.md:L108 says "47 LoC" — file is 46 lines. Off-by-one (D7). |

**Spot-check result**: 5/5 cited line ranges verified accurate. One companion
prose claim ("47 LoC") is off by one and recorded as D7 (MINOR).

---

## Internal Consistency Check (spec.md ↔ acceptance.md ↔ plan.md ↔ spec-compact.md)

| Item | spec.md | acceptance.md | plan.md | spec-compact.md | Consistent? |
|------|---------|---------------|---------|-----------------|-------------|
| REQ count | 10 (§3) but L77 says "8" | S-001..S-010 (10) | Tasks reference REQ-001..010 | L11–L20 lists 10 | INCONSISTENT internally within spec.md (D2) |
| Priority breakdown | 8 P0 + 2 P1 in table; L77 says 5+2+1; L792 says 7+3 | n/a | n/a | n/a | INCONSISTENT within spec.md (D2) |
| Test count | §8 has 45 numbered rows; L79 says ~30 | n/a | L413 says "All 45 tests" | n/a | INCONSISTENT within spec.md (D3) |
| EARS patterns claimed | L77 says "all five"; table has 4 | n/a | n/a | n/a | INCONSISTENT (D1, BLOCKER) |
| Files to create | §6.1 lists 12 source + 6 testdata | n/a | Tasks A1..F3 enumerate matching files | L49–L72 confirms 12 source + 6 testdata | CONSISTENT |
| Score formula | §2.3 Tanh divisor 100 | S-006-A asserts ≈0.881 for score=100 | C1 confirms divisor 100 | L83 confirms | CONSISTENT |
| Capabilities.Notes substrings | §5 lists 4 substrings | S-001-D lists same 4 | F3 confirms | L33 confirms | CONSISTENT |
| Metadata required keys | REQ-006 lists 6 | S-006-H confirms 6 | L162 (parse_test) confirms | L16 confirms | CONSISTENT (6 keys); but §6.3 lists 13 (D6) |
| Healthcheck mechanism | §6.4 hardcodes `www.reddit.com:443` | S-001-E asserts loopback test | A3 mentions "test seam if cleaner" | n/a | INCONSISTENT (D4) |
| Redirect allowlist hosts | §6.5 lists 4 hosts | S-010-A..D test allowlist | B1 confirms 4 hosts | L20 confirms 4 hosts | CONSISTENT |
| External Reddit URL | `https://www.reddit.com/search.json` | confirmed | confirmed | L12 confirms | CONSISTENT |
| User-Agent format | `usearch/<version> (+https://github.com/elymas/universal-search)` | S-009-A/B confirm | confirmed | L19 confirms (truncated) | CONSISTENT |
| Default UA version | "v0.1" | S-009-A confirms | n/a | n/a | CONSISTENT |
| Retry-After cap/default | cap 60s, default 5s | S-003-D/C confirm | B3 confirms | L13 confirms | CONSISTENT |

Cross-document consistency holds **between** spec.md, acceptance.md,
plan.md, and spec-compact.md for the contractual surface (REQs, tests,
files, formulas). The failures are concentrated **within** spec.md
(HISTORY metadata vs body, §6.3 table vs REQ-006, §6.4 sketch vs §5
acceptance).

---

## Chain-of-Verification Pass

Re-read targets after first pass:
- §3 EARS table re-read end-to-end (L327–L336); confirmed 4 patterns
  not 5. Confirmed priority count 8 P0 + 2 P1.
- §8 TDD plan re-read end-to-end (L862–L907); confirmed 45 tests, every
  REQ has ≥1 test.
- §7 Exclusions re-read (L801–L848); 16 distinct exclusion entries each
  citing destination SPEC. Specific. PASS for completeness.
- §11 Open Questions re-read (L1009–L1063); 7 questions each with
  recommended default + resolution owner. PASS.
- Cross-checked HISTORY metadata claims (L77–L79) against bodies: 3
  factual errors (D1, D2, D3) confirmed.
- Re-checked Healthcheck path (§5 acceptance vs §6.4 sketch vs plan A3):
  contradiction confirmed (D4).
- Re-checked Metadata keys: REQ-006 says 6 keys minimum, §6.3 table
  shows 13. Both can be true (6 is the floor) but the SPEC does not
  explicitly say so; documented as D6 (MAJOR, ambiguity at the
  contractual surface).
- Re-checked NFR-ADP-001 measurement mechanism: still unspecified (D5).
- Re-checked Unicode whitespace test: NBSP (U+00A0) is NOT in
  `unicode.IsSpace` per Go stdlib spec; the test fixture is
  inconsistent with the implementation rule (D9).
- Citation spot-check expanded from 5 to a sweep: also verified
  `pkg/types/normalized_doc.go:40-77` (Validate) — accurate;
  `pkg/types/query.go:18-44` — accurate; `internal/llm/client.go:51-54`
  (cited in plan.md) — accurate.
- Drive-by exclusions audited: spec.md:L545–L555 explicitly lists files
  NOT to modify (registry.go, pkg/types/*, metrics.go, cmd/main.go).
  plan.md:L442–L444 reaffirms. Strong scope discipline. No defect.

**New defects from second pass**: none beyond the 10 listed above. The
defect surface is concentrated in HISTORY metadata accuracy, the EARS
pattern claim, the Healthcheck test seam gap, the NFR-001 measurement
mechanism, the §6.3 vs REQ-006 Metadata key tension, and the NBSP
test case. The body of the SPEC (REQs, ACs, tests, exclusions,
dependencies, mapping table, MX plan) is otherwise solid.

---

## Recommendation

**Verdict: FAIL** at iteration 1. One BLOCKER (D1: missing State-Driven
EARS pattern + false HISTORY claim) is sufficient to fail per the audit
criterion "All 5 EARS patterns represented". Two additional BLOCKERs
(D2, D3) compound the HISTORY accuracy problem.

Manager-spec must in iteration 2:

1. **Fix D1 (BLOCKER, EARS pattern coverage)**: Either drop the
   "all five EARS patterns" claim from HISTORY (and accept 4/5 is
   sufficient for a stateless adapter, with explicit rationale), OR
   add a genuine State-Driven REQ. Recommended path: add REQ-ADP-011
   (State-Driven) covering ctx-mid-flight body-streaming behaviour, OR
   covering "WHILE the registry holds this adapter, Search invocations
   SHALL be safe to call concurrently from N goroutines without lock
   coordination by the caller" (concurrency-safety contract — not
   currently stated and meaningfully State-Driven).
2. **Fix D2 (BLOCKER, REQ count)**: Update spec.md:L77 to read
   "10 EARS REQs (8 × P0 + 2 × P1)". Update spec.md:L792 to match.
3. **Fix D3 (BLOCKER, test count)**: Update spec.md:L79 to read
   "~45 representative TDD tests" (or whatever the recount yields after
   D1 is applied).
4. **Fix D4 (MAJOR, Healthcheck test seam)**: Add
   `Options.HealthcheckTarget string` to the §6.4 Options sketch; default
   `"www.reddit.com:443"`. Reference it in REQ-ADP-001 acceptance.
5. **Fix D5 (MAJOR, NFR-001 measurement)**: Replace "p50 ≤ 5 ms" with
   a concrete benchmark invocation that reports the assertion target
   (e.g., "mean ≤ 5 ms over `go test -bench=BenchmarkParseListing25Docs
   -benchtime=10x -count=5` on amd64") OR cite the exact CI script path.
6. **Fix D6 (MAJOR, Metadata key contract)**: In §6.3 mark the 6
   contractual keys REQUIRED and the remaining 7 OPTIONAL inline; merge
   Open Q §11.7's classification into §6.3.
7. **Fix D7..D10 (MINOR/NIT)**: low priority for iteration 2; can be
   batched into iteration 3 or sync-phase cleanup.

Iteration 2 will re-audit the BLOCKER + MAJOR items above plus regression
check on the resolved defects.

---

*End of SPEC-ADP-001 review iteration 1*
