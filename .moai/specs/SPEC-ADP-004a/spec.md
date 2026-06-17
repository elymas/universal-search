---
id: SPEC-ADP-004a
title: GitHub Commit Search (Amendment to SPEC-ADP-004)
version: 0.1.0
milestone: M3 — Fanout, adapters, index
status: draft
priority: P1
owner: expert-backend
methodology: tdd
coverage_target: 85
created: 2026-06-17
updated: 2026-06-17
author: limbowl
issue_number: null
depends_on: [SPEC-ADP-004]
---

# SPEC-ADP-004a: GitHub Commit Search (Amendment to SPEC-ADP-004)

## HISTORY

- 2026-06-17 (initial draft v0.1, limbowl via manager-spec):
  Single-intent amendment to the implemented SPEC-ADP-004 GitHub adapter.
  Adds a FOURTH search intent — `kind=commit` — routing to GitHub's
  `/search/commits` endpoint via the existing `gogithub` dependency's
  `SearchService.Commits`. Ported from `github/github-mcp-server` v1.1's
  `search_commits` tool (which calls `client.Search.Commits(ctx, query,
  opts)` against `/search/commits` — verified via WebFetch of
  github-mcp-server `pkg/github/search.go`). Scope is deliberately
  minimal: ONE new intent, ONE new parse function
  (`parseCommitResults`), the `validIntents` set extended by one entry,
  the `ErrInvalidIntent` message text updated, and the
  `Capabilities.Notes` rate-limit ceiling line extended to advertise the
  commit-search cadence. No redesign of the existing routing, error
  taxonomy, qualifier append, score normaliser, HTTP client, or redirect
  allowlist — all are reused verbatim from SPEC-ADP-004.

  Verified current code state at draft time (read, not assumed):
  - `internal/adapters/github/search.go:24-29`: `validIntents` =
    `{code, issues, repos}`; default `repos`. No `commit` entry.
  - `internal/adapters/github/search.go:103-110`: intent dispatch
    `switch` over code/issues/repos. No commit case.
  - `internal/adapters/github/parse.go`: `parseCodeResults`,
    `parseIssueResults`, `parseRepoResults` exist. No
    `parseCommitResults`.
  - `internal/adapters/github/errors.go:23`: `ErrInvalidIntent`
    message = `"github: kind filter must be one of: code, issues,
    repos"`.
  - `internal/adapters/github/github.go:148-160`: `Capabilities.Notes`
    documents `"code search 9/min"` / `"issues/repos 30/min"`; no
    commit-search ceiling line.
  - `gogithub` (`github.com/google/go-github/v73`) is already a direct
    dep (`go.mod:94`). `SearchService.Commits(ctx, query, opts)
    (*CommitsSearchResult, *Response, error)` exists
    (`search.go:152`). `CommitsSearchResult.Commits []*CommitResult`
    (`search.go:126-130`). `CommitResult` exposes `SHA *string`,
    `Commit *Commit`, `Author *User`, `Committer *User`, `HTMLURL
    *string`, `Repository *Repository`, `Score *float64`
    (`search.go:133-145`). `Commit.Message *string`, `Commit.Author
    *CommitAuthor`, `Commit.Committer *CommitAuthor`
    (`git_commits.go:45-60`); `CommitAuthor.Name/Email/Login *string`,
    `CommitAuthor.Date *Timestamp` (`git_commits.go:69-76`).

  NOTE on `go.mod` deviation: parent SPEC-ADP-004 §6.1 references
  `github.com/google/go-github/v85`, but the implemented adapter ships
  on `v73.0.0` (documented deviation at `github.go:5-8`). THIS
  amendment targets the as-implemented `v73` API surface (verified
  above). No new module dependency is added — `SearchService.Commits`
  is already present in the pinned `v73`.

  5 EARS REQs, all P1 (this is a non-blocking
  enhancement to an implemented adapter). 0 new NFRs (parent NFRs
  unchanged; the commit parse path inherits the parent's allocation /
  goroutine-leak / FD discipline transitively). Harness level:
  standard (single package, ≤3 source files touched, no
  security/payment/PII keywords, no new dependency, no config/compose
  deltas). Sprint Contract OPTIONAL. Evaluator profile `default`.

---

## 1. Purpose

SPEC-ADP-004 (implemented 2026-05-07) ships the GitHub adapter with
three search intents routed by `Query.Filters[Key="kind"]`:
`code` → `/search/code`, `issues` → `/search/issues`, `repos` →
`/search/repositories` (default). GitHub's REST search API exposes a
fourth search surface — `/search/commits` — that the v0.1 adapter does
not reach.

This amendment adds the `commit` intent. Commit search lets a
`usearch query` surface git commits matching a free-text query (commit
message content) plus the SPEC-ADP-004 §6.4 qualifier set
(`author:`, `committer-date:`/`since`, `repo:`, `org:`, `user:` map
naturally onto commit search). Each commit becomes one
`types.NormalizedDoc` carrying the SHA, message, authoring metadata,
repository, permalink, and authored date.

The feature is a direct port of github/github-mcp-server v1.1's
`search_commits` tool, which calls the same go-github
`SearchService.Commits` method this amendment uses. The parent SPEC's
Open Question §11.x referenced "users/commits" only as examples of
intents the v0.1 router would REJECT (`ErrInvalidIntent`); this
amendment promotes `commit` from rejected-example to supported-intent.

Like the parent adapter, the commit intent does NOT do fanout
(SPEC-FAN-001), retry (SPEC-FAN-001 D6), caching (SPEC-CACHE-001),
ranking fusion (SPEC-IDX-001), and emits ZERO metrics/logs/spans of its
own (registry wrappedAdapter sole-emitter discipline preserved). It does
ONE additional job: turn a `types.Query` with `kind=commit` into a
`/search/commits` call and normalize the typed `*CommitResult` slice
into `[]types.NormalizedDoc`.

---

## 2. Scope

### 2.1 In-Scope

| Ref | Item |
|-----|------|
| a | `internal/adapters/github/search.go`: extend `validIntents` (currently `{code, issues, repos}`) with one entry `"commit"`. Add a `case "commit"` arm to the intent-dispatch `switch` (currently lines 103-110) that calls a new `(*Adapter).searchCommits(ctx, queryStr, opts, retrievedAt)` helper mirroring the existing `searchCode`/`searchIssues`/`searchRepos` helper shape (invoke `a.ghClient.Search.Commits`, capture `resp.NextPage`, delegate to `parseCommitResults`, wrap parse errors in `*types.SourceError{Category: CategoryPermanent}`, wrap call errors via `categorizeError`). |
| b | `internal/adapters/github/parse.go`: new `parseCommitResults(result *gogithub.CommitsSearchResult, nextPage int, retrievedAt time.Time) ([]types.NormalizedDoc, error)` mirroring `parseRepoResults`/`parseIssueResults` shape — iterates `result.Commits` (`[]*CommitResult`), nil-guards each element and nested pointers via the existing `safeStr`/`safeTime` helpers, maps fields per the §6.3 table, sets `URL` from `HTMLURL` ELSE the synthesized `https://github.com/{repoFullName}/commit/{sha}` permalink (skipping any commit lacking BOTH `sha` and repo full name so `URL` is never empty), sets `DocType=DocTypeRepo` (Open Question §11.1 — same deferral as code hits; commits live in repos and no `DocTypeCommit` enum exists), and surfaces `next_cursor` on the last doc when `nextPage > 0` using the existing `strconv.Itoa(nextPage)` convention. |
| c | `internal/adapters/github/errors.go`: update the `ErrInvalidIntent` message string (currently `"github: kind filter must be one of: code, issues, repos"`) to `"github: kind filter must be one of: code, issues, repos, commit"`. The sentinel identity is unchanged (same `var`, same `errors.Is` behaviour); only the human-readable text changes. |
| d | `internal/adapters/github/github.go`: extend the `Capabilities.Notes` rate-limit ceiling sentence to advertise the commit-search cadence. GitHub's secondary-rate-limit for commit search is the same 30/min search bucket as issues/repos. The Notes line currently reads `"Rate ceilings: code search 9/min vs issues/repos 30/min ..."`; append `"; commit search 30/min"` (or equivalent) so consumers see the commit cadence. No `Capabilities` STRUCT field changes — `DocTypes` stays `[DocTypeRepo, DocTypeIssue]` (commits map to `DocTypeRepo`), `RateLimitPerMin` stays `30`. |
| e | `internal/adapters/github/parse_test.go`: new `TestParseCommitResultsFieldMapping` table-driven test (≥3 fixtures: normal commit with full author/committer, commit with nil `Commit` pointer, commit with nil `Author`/`Committer` `*User`) asserting every NormalizedDoc field per §6.3; plus `TestParseCommitPaginationCursor` (NextPage>0 → last doc `next_cursor`) and `TestParseCommitNoCursorOnLastPage` (NextPage=0 → no `next_cursor`). |
| f | `internal/adapters/github/search_test.go`: new `TestSearchCommitIntentHappyPath` (httptest stub serving the commit fixture; `Filters=[{kind,"commit"}]` → N NormalizedDocs; observed URL path `/search/commits`), extend the existing invalid-intent test to confirm `commit` is now ACCEPTED (no longer `ErrInvalidIntent`) AND that a still-unknown value such as `users` is still rejected with the UPDATED error text, and `TestSearchCommitRateLimited` (stub 403 + rate-limit headers under `kind=commit` → `CategoryRateLimited`, reusing the parent's `categorizeError`). |
| g | `internal/adapters/github/testdata/`: one new golden fixture `search_commits_response.json` — a representative `/search/commits` response body (3-5 commit items with `sha`, `commit.message`, `commit.author`, `commit.committer`, `author`, `committer`, `repository`, `html_url`, `score`), plus optionally `search_commits_response_pagination.json` (Link header / NextPage). Existing rate-limit / malformed fixtures are reused. |

### 2.2 Out-of-Scope

This amendment is a single-intent addition. The following are explicitly
excluded; each is either already owned by another SPEC or deliberately
deferred.

- **The other three intents** (code/issues/repos) — already implemented
  in SPEC-ADP-004. This amendment does not touch their routing, parsing,
  or fixtures.
- **A `DocTypeCommit` enum value** in `pkg/types/capabilities.go` →
  Open Question §11.1 (same deferral rationale as parent §11.1 for
  `DocTypeCode`). Commits map to `DocTypeRepo` in this amendment.
- **Per-intent rate-limit ceiling field** on `Capabilities` → parent
  Open Question §11.7. The single `RateLimitPerMin=30` is unchanged;
  the commit cadence is documented in `Notes` text only.
- **Commit-content / diff fetch follow-up** (a second
  `/repos/{owner}/{repo}/commits/{sha}` call to populate `Body` with the
  full patch) → deferred; `Body` is the commit message only.
- **New commit-specific qualifier keys** (e.g., `hash:`,
  `merge:false`, `committer-date:`) beyond the parent §6.4 8-key set →
  out of scope. The commit intent reuses the EXISTING `appendQualifiers`
  unchanged; `author`/`committer-date` style filtering via new keys is a
  future enhancement.
- **Sort customisation** (`sort=author-date|committer-date`) → out of
  scope; default relevance ("best match") only, consistent with the
  parent adapter's no-sort decision.
- **Retry orchestration, caching, RRF fusion, fanout** → unchanged
  ownership (SPEC-FAN-001 / SPEC-CACHE-001 / SPEC-IDX-001).
- **New external dependency** → none. `SearchService.Commits` is already
  in the pinned `go-github/v73`. `go.mod` is NOT modified.
- **Changes to `Capabilities` struct shape** (`DocTypes`,
  `RateLimitPerMin`, `SupportedLangs`, etc.) → only the `Notes` text is
  extended; no field value changes.
- **Live network integration tests** → httptest stub + golden fixtures
  only, consistent with parent §2.2.

### 2.3 Score Synthesis (Commit Intent)

[HARD] The commit intent passes through the SAME `normalizeScore(int)
float64` Tanh formula (`score.go`) used by the other intents, but commit
search has no engagement/popularity signal analogous to issue-comments
or repo-stars. `*CommitResult.Score` is a `*float64` GitHub relevance
score, but the existing `normalizeScore` takes an `int` and the existing
code-search intent already uses a neutral constant `0.5`
(`parse.go:67`) precisely because v73's `CodeResult.Score` is not used.

For consistency with the code intent, the commit intent SHALL set
`Score = 0.5` (neutral). This is the cheapest decision that keeps the
field bounded and deterministic; SPEC-IDX-001 RRF re-ranks by rank not
raw score across adapters, so a neutral commit score does not
destabilise cross-adapter fusion. Revisit is deferred (Open Question
§11.2).

---

## 3. EARS Requirements

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| REQ-ADP4a-001 | Optional | WHERE `Query.Filters` contains an entry with `Key=="kind"` AND `Value=="commit"`, the adapter SHALL route the search to GitHub's `/search/commits` endpoint via `a.ghClient.Search.Commits(ctx, queryStr, opts)` (reusing the same `appendQualifiers`-built `queryStr` and `SearchOptions{ListOptions{Page, PerPage}}` construction as the other intents), parse the typed `*CommitsSearchResult` per REQ-ADP4a-002, and return `(docs, nil)` on success. The value `"commit"` SHALL be a member of `validIntents` so it is NOT rejected by the existing invalid-intent guard. | P1 | `TestSearchCommitIntentHappyPath` (stub serves `search_commits_response.json`; `Filters=[{kind,"commit"}]`; assert N NormalizedDocs, each `Validate()` nil; observed URL path is `/search/commits`; `per_page`/`page` honour the parent clamp/default rules). In `search_test.go`. |
| REQ-ADP4a-002 | Ubiquitous | The adapter SHALL transform each `*gogithub.CommitResult` in `result.Commits` into one `types.NormalizedDoc` via `parseCommitResults` using the §6.3 field-mapping table: `ID = "github:commit:" + repoFullName + "@" + safeStr(SHA)` (where `repoFullName = safeStr(Repository.FullName)`, nil-safe through `Repository`); `SourceID="github"`; `URL = safeStr(HTMLURL)` WHEN non-empty, ELSE the deterministic synthesized permalink `"https://github.com/" + repoFullName + "/commit/" + safeStr(SHA)` (a GitHub commit's web URL is deterministic from repo + sha, so the adapter SHALL synthesize it whenever the API omits `HTMLURL`); `Title` = first line (≤80 runes) of `safeStr(Commit.Message)`; `Body = safeStr(Commit.Message)`; `Snippet = truncateRunes(Commit.Message, 280)`; `PublishedAt = safeTime(Commit.Author.Date)`; `RetrievedAt = retrievedAt`; `Author = safeStr(Commit.Author.Name)` (falling back to `safeStr(Author.Login)` when the GitHub `*User` is present and the `CommitAuthor.Name` is empty); `Score = 0.5` (neutral, §2.3); `Lang=""`; `DocType = types.DocTypeRepo`; `Hash=""`. The adapter SHALL populate `Metadata` with REQUIRED keys `{sha, repo_full_name, message_subject, kind:"commit"}` and OPTIONAL keys `{author_name, author_email, committer_name, committed_date, authored_date}` when the corresponding source pointers are non-nil. All nested-pointer access (`Commit`, `Commit.Author`, `Commit.Committer`, `Author`, `Committer`, `Repository`) SHALL be nil-guarded so a result with any nil sub-object yields a non-panicking NormalizedDoc. The adapter SHALL NEVER emit a NormalizedDoc that would fail `NormalizedDoc.Validate()`: because `Validate()` requires a non-empty `URL`, the URL synthesis above guarantees a populated `URL` whenever both `SHA` and the repo full name are present; as a guard, the adapter SHALL skip (not emit) any commit lacking BOTH a `SHA` and a repo full name, since no deterministic URL can be synthesized for it. | P1 | `TestParseCommitResultsFieldMapping` (table ≥3 fixtures: full commit / nil `Commit` / nil `Author`+`Committer` `*User`); `TestParseCommitNilSafe` (commit with `Commit:nil` but `SHA`+`Repository.FullName` present → `Title==""`, `Body==""`, synthesized `URL`, `Validate()` passes). In `parse_test.go`. |
| REQ-ADP4a-003 | Event-Driven | WHEN `a.ghClient.Search.Commits` returns a paginated response with `*Response.NextPage > 0`, the adapter SHALL surface the next-page cursor via `Metadata["next_cursor"] = strconv.Itoa(NextPage)` on the LAST returned NormalizedDoc only (identical convention to the parent intents). WHEN `*Response.NextPage == 0` (last page), NO returned doc SHALL carry a `next_cursor` key. | P1 | `TestParseCommitPaginationCursor` (NextPage=2 → last doc `Metadata["next_cursor"]=="2"`; earlier docs lack the key); `TestParseCommitNoCursorOnLastPage` (NextPage=0 → no doc has `next_cursor`). In `parse_test.go`. |
| REQ-ADP4a-004 | Event-Driven | WHEN a commit-search request returns a rate-limit signal (`*gogithub.RateLimitError` or `*gogithub.AbuseRateLimitError`), the adapter SHALL return `(nil, &types.SourceError{Adapter:"github", Category: types.CategoryRateLimited, ...})` by reusing the existing `categorizeError` rosetta UNCHANGED — commit search produces the identical typed errors as the other intents. The adapter SHALL NOT add commit-specific error handling. | P1 | `TestSearchCommitRateLimited` (stub returns 403 + `X-RateLimit-Remaining: 0` + future `X-RateLimit-Reset` under `kind=commit`; assert `errors.Is(err, types.ErrRateLimited)` AND `srcErr.RetryAfter > 0` AND `srcErr.RetryAfter <= 90s`). In `search_test.go`. |
| REQ-ADP4a-005 | Unwanted | IF `Query.Filters` contains an entry with `Key=="kind"` AND `Value` is not one of `{"code", "issues", "repos", "commit"}`, THEN the adapter SHALL return `(nil, &types.SourceError{Adapter:"github", Category: types.CategoryPermanent, Cause: ErrInvalidIntent})` immediately and SHALL NOT issue any HTTP request, where `ErrInvalidIntent.Error()` SHALL read `"github: kind filter must be one of: code, issues, repos, commit"` (updated text including `commit`). The value `"commit"` SHALL NOT trigger this path (it is now valid). | P1 | `TestSearchCommitIntentAcceptedNotRejected` (`kind=commit` → no `ErrInvalidIntent`; at least one HTTP request issued); `TestSearchInvalidIntentUpdatedMessage` (`kind=users` → `errors.Is(err, ErrInvalidIntent)` AND zero HTTP requests AND `err`-chain message contains the substring `"commit"`). In `search_test.go`. |

---

## 4. Non-Functional Requirements

No NEW NFRs. The parent SPEC-ADP-004 NFRs (NFR-ADP4-001 parse-path
performance, NFR-ADP4-002 E2E p95, NFR-ADP4-003 no goroutine leak,
NFR-ADP4-004 resource cleanup) apply transitively to the commit intent
because it shares the same HTTP client, parse-helper discipline, and
`searchXxx` helper shape. The commit parse path uses the same
`safeStr`/`safeTime` nil-guard helpers and the same map-allocation
pattern as `parseRepoResults`, so its allocation and latency profile
falls within the existing NFR-ADP4-001 envelope. No new benchmark is
required for this amendment; the run-phase implementer MAY add a
commit-fixture case to the existing `BenchmarkParseGitHubResponse25Results`
table if convenient, but it is not a gate.

---

## 5. Acceptance Criteria

### REQ-ADP4a-001 — Commit Intent Routing

- `validIntents` (in `search.go`) contains the key `"commit"`.
- The intent-dispatch `switch` has a `case "commit"` arm calling
  `a.searchCommits(...)`.
- `TestSearchCommitIntentHappyPath`: stub serves
  `testdata/search_commits_response.json`; `Filters=[{kind,"commit"}]`;
  assert the returned slice length equals the fixture item count, every
  doc passes `Validate()`, and the URL path observed at the stub is
  `/search/commits`. Assert `per_page` and `page` query parameters honour
  the parent clamp (`MaxResults` → `per_page`) and default (`Cursor` →
  `page`) rules unchanged.

### REQ-ADP4a-002 — CommitResult → NormalizedDoc Field Mapping

- `parseCommitResults(result, nextPage, retrievedAt)` exists in
  `parse.go` with the documented signature.
- `TestParseCommitResultsFieldMapping` table-drives ≥3 fixtures:
  1. Full commit: `Commit.Message`, `Commit.Author.{Name,Email,Date}`,
     `Commit.Committer.{Name,Date}`, `SHA`, `HTMLURL`,
     `Repository.FullName`, `Author` (`*User`) all populated → assert
     every NormalizedDoc field per §6.3.
  2. Nil `Commit` pointer (but `SHA`+`Repository.FullName` present) →
     `Title==""`, `Body==""`, `Snippet==""`, `PublishedAt` zero,
     `URL` synthesized from repo+sha; doc still `Validate()`-valid.
  3. Nil `Author`/`Committer` `*User` (GitHub login absent, commit
     metadata present) → `Author == safeStr(Commit.Author.Name)`;
     no panic.
- For the full-commit fixture, assert:
  - `ID == "github:commit:" + repoFullName + "@" + sha`
  - `URL == HTMLURL`
  - `Title` is the first line of the message, ≤80 runes
  - `Body == Commit.Message` (full, unmodified)
  - `Snippet == truncateRunes(Commit.Message, 280)`
  - `PublishedAt == Commit.Author.Date` (UTC)
  - `Author == Commit.Author.Name`
  - `Score == 0.5`
  - `DocType == types.DocTypeRepo`
  - `Lang == ""`, `Hash == ""`
  - `Metadata` REQUIRED keys present:
    `{sha, repo_full_name, message_subject, kind:"commit"}`
- `TestParseCommitNilSafe`: `Commit:nil` element with `SHA`+repo present →
  non-panicking doc with synthesized `URL`, passes `Validate()`. A
  `*CommitResult` lacking BOTH `SHA` and repo full name is SKIPPED (not
  emitted), so no `Validate()`-failing doc is ever produced.

### REQ-ADP4a-003 — Pagination Cursor

- `TestParseCommitPaginationCursor`: a result built with `nextPage=2`
  → the LAST doc has `Metadata["next_cursor"] == "2"`; all earlier docs
  do NOT have the `next_cursor` key.
- `TestParseCommitNoCursorOnLastPage`: `nextPage=0` → no doc carries the
  `next_cursor` key.

### REQ-ADP4a-004 — Rate-Limit Categorization (Reuse)

- `TestSearchCommitRateLimited`: stub returns HTTP 403 with
  `X-RateLimit-Remaining: 0` and a future `X-RateLimit-Reset` while the
  request is `kind=commit`; assert `errors.Is(err, types.ErrRateLimited)`,
  `srcErr.Category == CategoryRateLimited`, `srcErr.RetryAfter > 0`, and
  `srcErr.RetryAfter <= 90s`.
- No new code in `client.go::categorizeError` — the same rosetta handles
  the commit path. The test asserts behaviour parity, not new code.

### REQ-ADP4a-005 — Intent Validation (Updated Error Text)

- `ErrInvalidIntent.Error()` returns
  `"github: kind filter must be one of: code, issues, repos, commit"`.
- `TestSearchCommitIntentAcceptedNotRejected`: `Filters=[{kind,"commit"}]`
  → the returned error is NOT `ErrInvalidIntent`; the stub observes ≥1
  request (the commit search is actually issued).
- `TestSearchInvalidIntentUpdatedMessage`: `Filters=[{kind,"users"}]`
  → `errors.Is(err, ErrInvalidIntent)`, ZERO HTTP requests observed, and
  the error message contains the substring `"commit"` (proving the
  updated text is wired).

### Capabilities Notes Extension

- `Capabilities().Notes` contains a substring advertising the commit
  cadence (e.g. `"commit search 30/min"`). The parent's
  `TestCapabilitiesShape` (github_test.go:73) asserts ONLY 2 substrings —
  `"go-github"` and `"USEARCH_GITHUB_TOKEN"` — both of which remain
  present, so that test still passes unchanged (the Notes edit only
  appends text).
- `Capabilities().DocTypes` is UNCHANGED (`[DocTypeRepo, DocTypeIssue]`).
- `Capabilities().RateLimitPerMin` is UNCHANGED (`30`).

---

## 6. Technical Approach

### 6.1 Files to Modify

**Modified (4 files)**:

- `internal/adapters/github/search.go` — add `"commit"` to
  `validIntents`; add `case "commit"` dispatch arm; add
  `(*Adapter).searchCommits` helper mirroring `searchCode`/`searchIssues`/
  `searchRepos`.
- `internal/adapters/github/parse.go` — add `parseCommitResults`.
- `internal/adapters/github/errors.go` — update `ErrInvalidIntent`
  message text (append `, commit`).
- `internal/adapters/github/github.go` — extend `Capabilities.Notes`
  rate-ceiling sentence with the commit cadence.

**Modified (test files)**:

- `internal/adapters/github/parse_test.go` — add commit field-mapping +
  pagination tests.
- `internal/adapters/github/search_test.go` — add commit happy-path +
  rate-limit + intent-acceptance tests; update the existing
  invalid-intent test for the new message text.

**Created (testdata)**:

- `internal/adapters/github/testdata/search_commits_response.json`
  (+ optional `search_commits_response_pagination.json`).

**Unchanged (by design)**:

- `internal/adapters/github/client.go` — `categorizeError`,
  redirect allowlist, HTTP client all reused as-is.
- `internal/adapters/github/score.go` — `normalizeScore` unused by the
  commit intent (neutral 0.5 per §2.3); no change.
- `go.mod` — NO new dependency. `SearchService.Commits` already in v73.
- `pkg/types/*` — no contract change. `DocTypeRepo` reused for commits.
- `internal/adapters/registry.go` — no change.

### 6.2 Commit Result → NormalizedDoc Field Mapping (§6.3)

`*gogithub.CommitResult` (v73 `search.go:133-145`) →
`types.NormalizedDoc`:

| go-github Field | NormalizedDoc Field | Transform |
|-----------------|---------------------|-----------|
| `"github:commit:" + repoFullName + "@" + safeStr(SHA)` | `ID` | Composite stable ID (repoFullName from `Repository.FullName`, nil-safe). NOTE: go-github v73 `Commit` has NO `repo_full_name` field; the repo full name comes ONLY from `CommitResult.Repository.FullName`. |
| (constant) | `SourceID` | `"github"` |
| `safeStr(HTMLURL)` if non-empty, else `"https://github.com/" + repoFullName + "/commit/" + safeStr(SHA)` | `URL` | Commit permalink. Synthesized deterministically when the API omits `HTMLURL`, so `URL` is never empty when `SHA`+repo are present (Validate requires non-empty URL). |
| first line of `safeStr(Commit.Message)`, ≤80 runes | `Title` | Subject line |
| `safeStr(Commit.Message)` | `Body` | Full message, unmodified (markdown/plain) |
| `truncateRunes(safeStr(Commit.Message), 280)` | `Snippet` | Same truncation discipline as other intents |
| `safeTime(Commit.Author.Date)` (nil-safe through `Commit`) | `PublishedAt` | Authored date, UTC |
| `retrievedAt` | `RetrievedAt` | Injected (parse time = `time.Now().UTC()` in `Search`) |
| `safeStr(Commit.Author.Name)`; fallback `safeStr(Author.Login)` | `Author` | Commit author display name, nil-safe |
| (constant) | `Score` | `0.5` (neutral; §2.3) |
| (constant) | `Lang` | `""` |
| (constant) | `DocType` | `types.DocTypeRepo` (Open Question §11.1) |
| (constant) | `Hash` | `""` |
| (constructed) | `Metadata` | REQUIRED: `sha`, `repo_full_name`, `message_subject` (the Title line), `kind="commit"`. OPTIONAL (when source non-nil): `author_name`, `author_email` (`Commit.Author.Email`), `committer_name` (`Commit.Committer.Name`), `authored_date` (`Commit.Author.Date` RFC3339), `committed_date` (`Commit.Committer.Date` RFC3339). |

Nil-guard contract: `Commit`, `Commit.Author`, `Commit.Committer`,
`Author`, `Committer`, `Repository`, `SHA`, `HTMLURL` are ALL `*`
pointers in v73 and any may be nil. `parseCommitResults` MUST guard each
access (reusing the existing `safeStr`/`safeTime` helpers and explicit
nil-checks for the struct pointers) so a malformed/partial commit yields
a non-panicking doc.

Validate-safety: `NormalizedDoc.Validate()` (normalized_doc.go:63-77)
requires a non-empty `URL`. Because a GitHub commit's web URL is
deterministic, `parseCommitResults` synthesizes
`"https://github.com/" + repoFullName + "/commit/" + sha` whenever the
API `HTMLURL` is nil/empty, guaranteeing a populated `URL`. As a guard,
`parseCommitResults` SHALL `continue` (skip, not emit) any commit
lacking BOTH a `SHA` and a repo full name — no deterministic URL can be
synthesized for it, mirroring the existing `if cr == nil { continue }`
skip. The adapter therefore NEVER emits a NormalizedDoc that fails
`Validate()`.

### 6.3 searchCommits Helper Shape (illustrative; final form in run phase)

```go
// internal/adapters/github/search.go (new helper, mirrors searchRepos)
func (a *Adapter) searchCommits(ctx context.Context, query string, opts *gogithub.SearchOptions, retrievedAt time.Time) ([]types.NormalizedDoc, error) {
    result, resp, err := a.ghClient.Search.Commits(ctx, query, opts)
    if err != nil {
        if se := categorizeError(err); se != nil {
            return nil, se
        }
        return nil, err
    }
    nextPage := 0
    if resp != nil {
        nextPage = resp.NextPage
    }
    docs, pErr := parseCommitResults(result, nextPage, retrievedAt)
    if pErr != nil {
        return nil, &types.SourceError{
            Adapter:  "github",
            Category: types.CategoryPermanent,
            Cause:    pErr,
        }
    }
    return docs, nil
}
```

The `validIntents` map gains one entry and the dispatch `switch` gains
one `case`:

```go
var validIntents = map[string]struct{}{
    "code":   {},
    "issues": {},
    "repos":  {},
    "commit": {},
}

// ... in Search():
switch intent {
case "code":
    return a.searchCode(ctx, queryStr, opts, retrievedAt)
case "issues":
    return a.searchIssues(ctx, queryStr, opts, retrievedAt)
case "commit":
    return a.searchCommits(ctx, queryStr, opts, retrievedAt)
default: // "repos"
    return a.searchRepos(ctx, queryStr, opts, retrievedAt)
}
```

### 6.4 Filter Qualifier Append (Unchanged)

The commit intent reuses `appendQualifiers` (parent §6.4) UNCHANGED. The
8-key set (`since`, `language`, `repo`, `org`, `user`, `topic`, `state`,
`is_pr`) applies as-is. Keys not applicable to commit search
(`language`, `topic`, `state`, `is_pr` per the existing
intent-applicability guards in `buildQualifier`) are silently dropped by
the existing logic — no commit-specific qualifier branches are added in
this amendment (deferred to §2.2 out-of-scope).

NOTE: `since` → `created:>=<RFC3339>` is what the existing
`buildQualifier` emits for the `since` key; GitHub commit search uses
`committer-date:`/`author-date:` qualifiers rather than `created:`.
Whether `created:` is honoured by `/search/commits` is documented as
Open Question §11.3 — but this amendment does NOT add a new
commit-specific date qualifier (scope discipline). The existing behaviour
is preserved verbatim.

### 6.5 MX Tag Plan

| File::Symbol | Tag | Reason |
|--------------|-----|--------|
| `parse.go::parseCommitResults` | `@MX:ANCHOR` | Commit field-mapping integrity gate — every commit NormalizedDoc passes through; a mapping error corrupts all commit search results. `@MX:REASON: NormalizedDoc field-mapping integrity gate for commit intent`. `@MX:SPEC: SPEC-ADP-004a`. Mirrors the existing `parseIssueResults`/`parseRepoResults` ANCHOR pattern. |
| `search.go::validIntents` (updated) | (existing `@MX:NOTE` on `Search`, no new tag) | The `Search` ANCHOR already covers the routing surface; adding one intent key does not warrant a new tag. |
| `parse.go::parseCommitResults` nil-guard block | `@MX:NOTE` | Documents the 6-pointer nil-guard contract (`Commit`, nested authors, `Author`/`Committer` `*User`, `Repository`). `@MX:SPEC: SPEC-ADP-004a`. |

All tags `[AUTO]`-prefixed, `code_comments: en`.

### 6.6 Harness Level

Standard. Single package (`internal/adapters/github/`), ≤3 source files
+ 2 test files + 1 testdata fixture, zero cross-package edits, zero new
dependency, zero security/payment/PII keywords, zero config/compose
deltas. Sprint Contract OPTIONAL. Evaluator profile `default`.

---

## 7. What NOT to Build (Exclusions)

[HARD] This amendment explicitly excludes the following. Each is either
owned elsewhere or deliberately deferred.

- **A `DocTypeCommit` enum** in `pkg/types/capabilities.go` — commits
  map to `DocTypeRepo`; Open Question §11.1.
- **Per-intent rate-limit ceiling field** on `Capabilities` — parent
  Open Question §11.7; only `Notes` text is extended.
- **Commit-content / diff fetch** (`/repos/{owner}/{repo}/commits/{sha}`)
  — `Body` is the commit message only; deferred.
- **New commit-specific qualifier keys** (`hash:`, `author:`,
  `committer-date:`, `merge:`) — the existing 8-key `appendQualifiers`
  is reused unchanged; Open Question §11.3.
- **Sort customisation** (`sort=author-date|committer-date`) — default
  relevance only.
- **Score calibration from `*CommitResult.Score`** — neutral `0.5` per
  §2.3; Open Question §11.2.
- **Any change to the code/issues/repos intents** — out of scope; this
  is an additive amendment.
- **New external dependency or `go.mod` change** — `SearchService.Commits`
  is already in the pinned v73.
- **Retry / caching / RRF / fanout** — unchanged ownership.

---

## 8. TDD Plan

Development follows RED-GREEN-REFACTOR per
`quality.development_mode: tdd`. Representative RED-phase tests grouped
by REQ. Coverage target 85% for the new code paths (the amendment adds
`parseCommitResults` + `searchCommits` + one `validIntents` entry + one
error-string change; all are exercised by the tests below).

| # | Test | Layer | REQ | Assertion |
|---|------|-------|-----|-----------|
| 1 | `TestSearchCommitIntentHappyPath` | `search_test.go` | REQ-ADP4a-001 | `kind=commit` → N docs from `/search/commits`; per_page/page honoured |
| 2 | `TestParseCommitResultsFieldMapping` | `parse_test.go` | REQ-ADP4a-002 | Table ≥3 fixtures; every field per §6.3 |
| 3 | `TestParseCommitNilSafe` | `parse_test.go` | REQ-ADP4a-002 | `Commit:nil` (SHA+repo present) → non-panicking doc, synthesized URL, Validate passes; commit with no SHA+repo skipped |
| 4 | `TestParseCommitPaginationCursor` | `parse_test.go` | REQ-ADP4a-003 | NextPage=2 → last doc `next_cursor="2"` |
| 5 | `TestParseCommitNoCursorOnLastPage` | `parse_test.go` | REQ-ADP4a-003 | NextPage=0 → no `next_cursor` key |
| 6 | `TestSearchCommitRateLimited` | `search_test.go` | REQ-ADP4a-004 | 403 + rate headers → CategoryRateLimited; RetryAfter in (0, 90s] |
| 7 | `TestSearchCommitIntentAcceptedNotRejected` | `search_test.go` | REQ-ADP4a-005 | `kind=commit` → not ErrInvalidIntent; ≥1 request |
| 8 | `TestSearchInvalidIntentUpdatedMessage` | `search_test.go` | REQ-ADP4a-005 | `kind=users` → ErrInvalidIntent; zero requests; message contains "commit" |
| 9 | `TestCapabilitiesNotesCommitCadence` | `github_test.go` | §5 Capabilities | `Notes` contains commit-cadence substring; the 2 substrings asserted by parent `TestCapabilitiesShape` (`"go-github"`, `"USEARCH_GITHUB_TOKEN"`) still present |

RED-GREEN-REFACTOR per requirement:

1. RED: write the failing test.
2. GREEN: minimal code (one `validIntents` entry, one `switch` case, one
   helper, one parse function, one error-string edit, one Notes edit).
3. REFACTOR: keep `parseCommitResults` structurally parallel to
   `parseRepoResults`; reuse `safeStr`/`safeTime`/`truncateRunes`/
   `snippetMaxRunes`; no new helpers unless they remove duplication.

Greenfield note: `parseCommitResults` and `searchCommits` are new
functions; no existing behaviour to preserve. The 4 modified files have
existing tests that MUST continue to pass (the parent's
`TestCapabilitiesShape`, `TestSearchInvalidIntentRejectedNoHTTP`-style
tests) — the run phase verifies no regression.

---

## 9. Dependencies

### 9.1 Upstream SPEC Dependencies

- **SPEC-ADP-004 (implemented)**: HARD dep. This amendment edits the
  files SPEC-ADP-004 created (`search.go`, `parse.go`, `errors.go`,
  `github.go`) and reuses its `categorizeError`, `appendQualifiers`,
  `safeStr`/`safeTime`/`truncateRunes` helpers, HTTP client, and
  redirect allowlist verbatim. SPEC-ADP-004 MUST be implemented (it is)
  before this amendment runs.
- **SPEC-CORE-001 (implemented)**: provides `pkg/types.NormalizedDoc`,
  `types.DocTypeRepo`, `*types.SourceError`, `types.Query`/`Filter`.
  Transitive via the parent. SOFT dep (no new contract use).

### 9.2 Parallelizable

- None required. This is a self-contained edit within one package.

### 9.3 Downstream

- **SPEC-FAN-001 / SPEC-IDX-001 / SPEC-CACHE-001**: consume the adapter's
  `Search` output unchanged; commit docs flow through the same
  `[]NormalizedDoc` contract. No downstream change required.

### 9.4 External Dependencies

- **NONE new.** `github.com/google/go-github/v73`
  (`SearchService.Commits`, `CommitsSearchResult`, `CommitResult`,
  `Commit`, `CommitAuthor`) is already the pinned direct dep
  (`go.mod:94`). Verified present at draft time.

---

## 10. Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Nil-pointer dereference on `*CommitResult.Commit` / nested `*CommitAuthor` / `*User` | Medium | Low | Explicit nil-guards + reused `safeStr`/`safeTime`; `TestParseCommitNilSafe` covers the all-nil path. |
| `created:` qualifier (from existing `since` mapping) not honoured by `/search/commits` | Low | Low | Out of scope; behaviour preserved verbatim from parent. Open Question §11.3 documents the commit-date qualifier mismatch for a future SPEC. |
| Updated `ErrInvalidIntent` text changes the sentinel's message string | Low | Low | The amendment OWNS the message change. No parent test asserts the exact `ErrInvalidIntent` message string — the only parent invalid-intent test (`TestSearchInvalidIntentRejectedNoHTTP`, search_test.go:754-779) checks ONLY `Category==CategoryPermanent` and zero outbound requests, so no existing test breaks. The NEW `TestSearchInvalidIntentUpdatedMessage` (REQ-ADP4a-005) is the only test that inspects the message text. |
| Neutral `Score=0.5` miscalibrates commit ranking | Low | Low | Open Question §11.2; SPEC-IDX-001 RRF re-ranks by rank not raw score. Consistent with code-intent precedent (`parse.go:67`). |
| Commit-search rate-ceiling differs from 30/min assumption | Low | Low | Notes text documents 30/min (the search bucket); actual ceiling is GitHub's; `RateLimitPerMin` unchanged. Per-intent ceiling field deferred (parent §11.7). |
| `go-github/v85` reference in parent §6.1 vs as-implemented v73 | Low | Low | This amendment explicitly targets the as-implemented v73 surface (verified). No version change attempted. |

---

## 11. Open Questions

These do NOT block SPEC approval. Each has a recommended default.

1. **`DocTypeCommit` enum addition** (in `pkg/types/capabilities.go`).
   **Recommended default**: NO. Map commits to `DocTypeRepo` and store
   `kind="commit"` in Metadata (same rationale as parent §11.1 for
   `DocTypeCode`). Adding an enum value is an SDK-boundary change.
   **Resolution owner**: SPEC-IDX-001 author if RRF tuning needs commits
   weighted distinctly from repos/code.

2. **Commit score synthesis.** Commit search has no engagement signal;
   this amendment uses neutral `Score=0.5`. `*CommitResult.Score`
   (`*float64`) is a GitHub relevance score that COULD be normalized.
   **Recommended default**: keep `0.5` in v0.1 (consistent with code
   intent). **Resolution owner**: SPEC-IDX-001 author after M3 RRF
   measurements.

3. **Commit-date qualifier.** The existing `since` filter maps to
   `created:>=` (`buildQualifier`), but GitHub commit search uses
   `author-date:`/`committer-date:`. This amendment does NOT add a
   commit-specific date qualifier (scope discipline). **Recommended
   default**: defer. A future SPEC may add `committer-date:` support to
   `buildQualifier` gated on `intent=="commit"`. **Resolution owner**:
   future SPEC-ADP-004b or the SPEC-IR author.

---

## 12. References

### External (URL-cited; verified)

- https://github.com/github/github-mcp-server — v1.1 `search_commits`
  tool; calls `client.Search.Commits(ctx, query, opts)` against
  `/search/commits` (verified via WebFetch of `pkg/github/search.go`).
- https://docs.github.com/en/rest/search/search#search-commits —
  `/search/commits` endpoint; commit search response shape.
- https://github.com/google/go-github — v73 `SearchService.Commits`,
  `CommitsSearchResult`, `CommitResult`, `Commit`, `CommitAuthor`.

### Internal (file:line cited)

- `.moai/specs/SPEC-ADP-004/spec.md` — parent SPEC; this amendment
  inherits its routing, error taxonomy, qualifier append, score
  normaliser, HTTP client, and field-mapping discipline.
- `internal/adapters/github/search.go:24-29` — `validIntents` (extended
  here).
- `internal/adapters/github/search.go:103-110` — intent dispatch
  `switch` (one `case` added here).
- `internal/adapters/github/search.go:159-180` — `searchRepos` helper
  shape (mirrored by `searchCommits`).
- `internal/adapters/github/parse.go:182-263` — `parseRepoResults`
  (structural template for `parseCommitResults`).
- `internal/adapters/github/parse.go:267-302` — `safeStr`/`safeTime`/
  `truncateRunes` helpers + `snippetMaxRunes` (reused).
- `internal/adapters/github/parse.go:67` — code-intent neutral
  `Score: 0.5` precedent.
- `internal/adapters/github/errors.go:23` — `ErrInvalidIntent` message
  (updated here).
- `internal/adapters/github/github.go:148-160` — `Capabilities` /
  `Notes` (rate-ceiling line extended here).
- `internal/adapters/github/client.go` (via parent) — `categorizeError`
  reused unchanged.
- `pkg/types/capabilities.go:15-21` — DocType enum (`DocTypeRepo` reused
  for commits).
- `go.mod:94` — `github.com/google/go-github/v73 v73.0.0` (already
  present; `SearchService.Commits` available; no change).
- go-github v73 `github/search.go:126-175` — `CommitsSearchResult`,
  `CommitResult`, `SearchService.Commits` (verified in module cache).
- go-github v73 `github/git_commits.go:45-76` — `Commit`,
  `CommitAuthor` struct fields (verified in module cache).

---

*End of SPEC-ADP-004a v0.1 (DRAFT)*
