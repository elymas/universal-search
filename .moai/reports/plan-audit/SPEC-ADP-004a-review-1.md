# SPEC Review Report: SPEC-ADP-004a
Iteration: 1/3
Verdict: PASS-WITH-FINDINGS
Overall Score: 0.82

> Reasoning context from the SPEC author was provided in the prompt. It was read
> but NOT used as evidence. Per M1 Context Isolation, all conclusions below derive
> only from the four SPEC files and independent verification against the working
> tree and the go-github v73 module cache.

## Must-Pass Results

- **[PASS] MP-1 REQ number consistency** — REQ-ADP4a-001 … REQ-ADP4a-005, sequential,
  no gaps, no duplicates, consistent 3-digit padding. Verified in spec.md:L188-192
  (table), §5 (L214-293), §8 TDD table (L490-498), and acceptance.md coverage
  matrix (L167-172). All five appear in every cross-reference.

- **[PASS] MP-2 EARS format compliance** — All five ACs match an EARS pattern with
  inline REQ-IDs and `SHALL`:
  - REQ-ADP4a-001 Optional: `"WHERE Query.Filters contains an entry with Key=="kind" AND Value=="commit", the adapter SHALL route…"` (spec.md:L188)
  - REQ-ADP4a-002 Ubiquitous: `"The adapter SHALL transform each *gogithub.CommitResult…"` (spec.md:L189)
  - REQ-ADP4a-003 Event-Driven: `"WHEN a.ghClient.Search.Commits returns a paginated response with *Response.NextPage > 0, the adapter SHALL surface…"` (spec.md:L190)
  - REQ-ADP4a-004 Event-Driven: `"WHEN a commit-search request returns a rate-limit signal…, the adapter SHALL return…"` (spec.md:L191)
  - REQ-ADP4a-005 Unwanted: `"IF Query.Filters contains an entry with Key=="kind" AND Value is not one of {…}, THEN the adapter SHALL return…"` (spec.md:L192)
  No double-negatives. No informal `should`/`may` in normative text. PASS.
  (Note: the requirements are very long compound sentences — see D7 — but each is
  a single well-formed EARS statement, so MP-2 holds.)

- **[PASS] MP-3 YAML frontmatter validity** — `id`, `title`, `version`, `status`,
  `priority`, `created`, `updated`, `owner`, `methodology`, `coverage_target`,
  `depends_on` all present with correct types (spec.md:L1-16). The generic rubric
  field names `created_at` and `labels` are not used; this matches the project's
  established SPEC schema (parent SPEC-ADP-004 uses the same fields), so the
  semantic content (creation date as ISO string, priority) is present. Marked PASS
  against the project schema; `labels` absence noted as MINOR (D5), not a firewall
  failure.

- **[N/A] MP-4 Section 22 language neutrality** — N/A: single-language SPEC. This is
  a Go-only adapter amendment scoped entirely to `internal/adapters/github/`. The
  16-language neutrality criterion does not apply.

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.75 | 0.75 (minor ambiguity in 1-2 reqs) | REQ-ADP4a-002 contains an invented field reference `Commit.repo_full_name` (spec.md:L189) that contradicts the §6.2 table (L343) and the real go-github `Commit` struct (no repo field). Otherwise unambiguous. |
| Completeness | 0.90 | 1.0-ish (all sections present, well-populated) | All sections present: HISTORY (L20-77), Purpose/WHY (L81-111), Scope/WHAT (L115-180), EARS REQS (L184-192), Acceptance (L212-293), Exclusions §7 (L454-476 with 9 specific entries), Open Questions §11 (L563-587). |
| Testability | 0.65 | 0.50-0.75 | Most ACs are binary-testable with concrete fixtures, but AC-006/AC-007 + the REQ-002 nil-guard contract assert a "valid zero-filled NormalizedDoc" that can be FALSE: `Validate()` requires non-empty `URL` (verified normalized_doc.go:L70-72), yet `HTMLURL` is listed as nullable (spec.md:L358). See D4. |
| Traceability | 1.0 | 1.0 | Every REQ has ≥1 AC; every AC traces to a valid REQ; coverage matrix (acceptance.md:L167-172) maps all 5 REQs + the Capabilities extension to implementation files and named tests. No orphans, no uncovered REQs. |

## Defects Found

**D1. spec.md:L290-291 & acceptance.md:L130-136 — FALSE claim about the parent test's substring assertions — Severity: major**
The SPEC states the parent `TestCapabilitiesShape` asserts 6 `Notes` substrings
(`"GitHub REST"`, `"PAT"`, `"public_repo scope"`, `"code search 9/min"`,
`"issues/repos 30/min"`, `"Retry-After cap 90s"`) and "must still pass unchanged".
Verified against `internal/adapters/github/github_test.go:L73` — the actual test
checks ONLY two substrings: `"go-github"` and `"USEARCH_GITHUB_TOKEN"`. The
6-substring list is copied from the parent SPEC's *requirement text* (SPEC-ADP-004
REQ-ADP4-001), not from the implemented test. The DoD outcome ("don't break the
test") still holds by accident, but the cited test behavior is wrong and will
mislead the run-phase implementer when writing/reading the acceptance assertions.

**D2. spec.md:L556 (Risk table) + L192 (REQ-005) + research.md:L211-212 — phantom risk: no parent test asserts the exact `ErrInvalidIntent` string — Severity: major**
The SPEC repeatedly warns that updating the `ErrInvalidIntent` message "may break a
brittle parent test asserting exact string" and that "the run phase updates any
parent test asserting the old exact text." Verified: the only parent invalid-intent
test, `TestSearchInvalidIntentRejectedNoHTTP` (search_test.go:L754-779), asserts
ONLY `se.Category == CategoryPermanent` and zero outbound requests — it never
inspects the message string. No such brittle test exists. The mitigation/risk is
fabricated against a non-existent test state; the run phase will waste effort
hunting for a test to update.

**D4. spec.md:L189, L122, L362-363 & acceptance.md:L66-77 — "valid zero-filled NormalizedDoc" guarantee is contradicted by Validate() — Severity: major**
REQ-ADP4a-002 (L189) and §6.2 (L362-363) promise that "a result with any nil
sub-object yields a valid (zero-filled) NormalizedDoc rather than panicking", and
AC-006/AC-007 assert `doc.Validate()` passes for nil-`Commit` / nil-`User` cases.
But `NormalizedDoc.Validate()` (normalized_doc.go:L63-77) requires non-empty
**URL** (from `safeStr(HTMLURL)`), and `HTMLURL` is itself listed as nullable in
the nil-guard set (spec.md:L358). Therefore a `CommitResult` with a nil `HTMLURL`
produces `URL==""` → `Validate()` FAILS, directly contradicting the "valid
zero-filled doc" guarantee. AC-006/AC-007 happen to pass only because they keep
`HTMLURL` populated, but that precondition is unstated. The run phase needs an
explicit resolution: either (a) the contract must say "non-panicking" not
"Validate-valid", or (b) AC-006/AC-007 must pin `HTMLURL` non-nil as a precondition,
or (c) define behavior when `URL==""` (skip the doc, like the `if cr == nil` skip).

**D3. spec.md:L189 — invented struct field `Commit.repo_full_name` in REQ-002 ID mapping — Severity: medium**
REQ-ADP4a-002 writes `ID = "github:commit:" + safeStr(Commit.repo_full_name or Repository.FullName) + "@" + safeStr(SHA)`. Verified against go-github v73
`git_commits.go` `Commit` struct — it has NO `repo_full_name` field; repo full name
is only available via `CommitResult.Repository.FullName`. The §6.2 mapping table
(L343) gets this right ("repoFullName from `Repository.FullName`, nil-safe"). The
REQ text is internally inconsistent with its own table and cites a non-existent
field.

**D5. spec.md:L1-16 — frontmatter omits `labels`; uses `created`/`updated` not `created_at` — Severity: medium**
Matches the project SPEC schema, so not a must-pass failure, but the generic SPEC
frontmatter contract expects a `labels` key. Worth confirming the project template
intentionally drops it.

**D6. spec.md:L71 — nonsensical REQ-priority phrasing — Severity: minor**
HISTORY says "5 EARS REQs (3 × P1 + 2 × P1; all P1 …)". "3 × P1 + 2 × P1" is
self-contradictory filler (all are P1 anyway). Cosmetic; rewrite as "5 EARS REQs,
all P1".

**D7. spec.md:L188-192 — EARS requirements are overloaded compound sentences — Severity: minor**
Each REQ packs routing + parsing delegation + validIntents membership + error
wrapping + acceptance summary into one multi-clause sentence (REQ-002 is ~20 lines).
Still EARS-valid, but readability/atomicity suffers; consider splitting derived
obligations into the §5 acceptance bullets (where most already exist).

## Chain-of-Verification Pass

Second-look findings (re-read every section, not just spot-checks):

- **Every REQ read end-to-end** (not skimmed): REQ-001..005 each re-verified for
  pattern + SHALL + inline ID. Found D3 (invented field) and D7 (overloading) only
  on the close re-read of REQ-002.
- **REQ sequencing checked end-to-end** across the table, §5, §8, and the coverage
  matrix — consistent, no gaps.
- **Traceability verified for EVERY REQ** (not sampled): all 5 + Capabilities map to
  ACs and named tests; reverse-checked AC-001..015 + EC-001..004 all trace to a REQ
  or the Capabilities extension. Clean.
- **Exclusions checked for specificity** (not just presence): §7 (L454-476) has 9
  concrete, named exclusions (DocTypeCommit enum, per-intent rate field, diff fetch,
  commit-specific qualifiers, sort, score calibration, other intents, go.mod,
  retry/cache/RRF/fanout). Specific, not vague.
- **Contradiction scan ACROSS requirements**: found the cross-section contradiction
  D4 (REQ-002 "valid doc" vs Validate URL requirement) and D3 (REQ-002 vs §6.2
  table) on the second pass — these were NOT caught in the first read.
- **Factual cross-check of cited code state** (independently verified, credit where
  due): `validIntents` at search.go:L25-29 ✓; dispatch switch at search.go:L103-110 ✓;
  `ErrInvalidIntent` text at errors.go:L23 quoted verbatim correctly ✓; `Score: 0.5`
  code-intent precedent at parse.go:L67 ✓; `Capabilities`/`Notes` at github.go:L137-160 ✓;
  `go.mod:94 = go-github/v73 v73.0.0` ✓; go-github v73 `SearchService.Commits`,
  `CommitsSearchResult`, `CommitResult{SHA,Commit,Author,Committer,HTMLURL,Repository,Score}`,
  `Commit{Message,Author,Committer}`, `CommitAuthor{Date,Name,Email,Login}` — ALL
  verified present in the module cache exactly as described ✓; `categorizeError`
  rate-limit path caps `RetryAfter` at `maxRetryAfter` (90s) ✓, so the `RetryAfter <= 90s`
  assertion (AC-010) is sound. The current-code citations are overwhelmingly
  accurate — the defects are confined to (a) parent *test* behavior (D1, D2) and
  (b) the Validate contract (D4) and one invented field (D3).

## Recommendation

PASS-WITH-FINDINGS. The SPEC is structurally strong: scope discipline is exemplary
(the named gap — adding the `commit` intent — is held tightly; the `since`→`created:`
vs `committer-date:` mismatch is correctly deferred to Open Question §11.3 rather
than scope-crept; no go.mod change; reuse vs new is explicit). EARS, traceability,
and most line citations are clean. None of the findings block approval, but the run
phase MUST resolve them before/while writing tests, because three of them describe
non-existent or contradictory test/contract state:

1. **D4 (do first)** — Decide the nil-`HTMLURL` behavior. `Validate()` requires
   non-empty `URL` (normalized_doc.go:L70-72). Either (a) reword the REQ-002 /
   §6.2 guarantee from "valid zero-filled NormalizedDoc" to "non-panicking
   NormalizedDoc", and add explicit handling for `URL==""` (skip or fixture
   precondition), or (b) make AC-006/AC-007 state `HTMLURL` non-nil as a
   precondition. Update the §6.2 nil-guard note (L362-363) accordingly.

2. **D1** — Correct spec.md:L290-291 and acceptance.md:L130-136: the parent
   `TestCapabilitiesShape` (github_test.go:L73) asserts only `"go-github"` and
   `"USEARCH_GITHUB_TOKEN"`. If the 6 substrings should be guaranteed, the run phase
   must ADD those assertions in `TestCapabilitiesNotesCommitCadence`, not rely on the
   parent test.

3. **D2** — Remove the phantom risk (spec.md:L556, research.md:L211-212) and the
   "update any parent test asserting the old exact text" clause in REQ-005's
   acceptance: no parent test asserts the `ErrInvalidIntent` string
   (search_test.go:L754-779 checks only Category + request count).

4. **D3** — Fix REQ-002 (spec.md:L189): remove the invented `Commit.repo_full_name`;
   the repo full name comes only from `CommitResult.Repository.FullName` (already
   correct in the §6.2 table).

5. **D5/D6/D7** — Optional polish: confirm `labels` omission is intentional; reword
   the "3 × P1 + 2 × P1" line; consider splitting REQ-002's compound obligations
   into §5 bullets.

*End of review — iteration 1/3.*
