# SPEC-ADP-004a Implementation Plan

**SPEC**: SPEC-ADP-004a — GitHub Commit Search (Amendment to SPEC-ADP-004)
**Status**: draft
**Methodology**: TDD (RED → GREEN → REFACTOR)
**Coverage Target**: 85%
**Owner**: expert-backend
**Priority**: P1
**Depends on**: SPEC-ADP-004 (implemented)

---

## 1. Overview

Additive, single-intent amendment to the implemented GitHub adapter.
Adds `kind=commit` routing to `/search/commits` via the already-pinned
`go-github/v73` `SearchService.Commits`. No redesign — the existing
routing skeleton, error rosetta, qualifier append, HTTP client, redirect
allowlist, and nil-guard helpers are reused verbatim. The work is four
small source edits + one new parse function + tests + one fixture.

---

## 2. File-Level Change Plan (in order)

Execute in this order so each step compiles and its RED test can be
written against a known surface.

### Step 1 — `internal/adapters/github/errors.go` (1 line)

- Update `ErrInvalidIntent` message text from
  `"github: kind filter must be one of: code, issues, repos"` to
  `"github: kind filter must be one of: code, issues, repos, commit"`.
- Sentinel `var` identity unchanged; only the string literal changes.
- RED first: `TestSearchInvalidIntentUpdatedMessage` (REQ-ADP4a-005)
  expects the new substring `"commit"` in the error chain.

### Step 2 — `internal/adapters/github/search.go` (~10 lines)

- Add `"commit": {}` to the `validIntents` map (lines 24-29).
- Add a `case "commit": return a.searchCommits(ctx, queryStr, opts,
  retrievedAt)` arm to the dispatch `switch` (lines 103-110).
- Add the `(*Adapter).searchCommits` helper after `searchRepos`,
  mirroring its shape exactly (call `a.ghClient.Search.Commits`, on
  error try `categorizeError` then raw err, capture `resp.NextPage`,
  call `parseCommitResults`, wrap parse errors in
  `*types.SourceError{Category: CategoryPermanent}`).
- `appendQualifiers` is reused UNCHANGED — no new branch.

### Step 3 — `internal/adapters/github/parse.go` (~50 lines)

- Add `parseCommitResults(result *gogithub.CommitsSearchResult,
  nextPage int, retrievedAt time.Time) ([]types.NormalizedDoc, error)`.
- Structure parallel to `parseRepoResults`:
  1. `if result == nil { return nil, nil }`
  2. `docs := make([]types.NormalizedDoc, 0, len(result.Commits))`
  3. iterate `result.Commits`; `if cr == nil { continue }`
  4. nil-guard `cr.Commit`, `cr.Commit.Author`, `cr.Commit.Committer`,
     `cr.Author`, `cr.Committer`, `cr.Repository`, `cr.SHA`,
     `cr.HTMLURL` before access
  4b. resolve `sha = safeStr(cr.SHA)` and `repoFullName =
     safeStr(cr.Repository.FullName)` (nil-safe). If BOTH are empty,
     `continue` (skip) — no deterministic URL can be synthesized, so
     emitting the doc would fail `Validate()` (non-empty URL required)
  4c. resolve `url = safeStr(cr.HTMLURL)`; if empty, synthesize
     `"https://github.com/" + repoFullName + "/commit/" + sha`
  5. build the §6.2 field mapping (URL from step 4c; Title = first
     message line ≤80 runes; Body = full message; Snippet =
     `truncateRunes(message, snippetMaxRunes)`; PublishedAt =
     `safeTime(Commit.Author.Date)`; Author = `safeStr(Commit.Author.Name)`
     fallback `Author.Login`; Score = `0.5`; DocType =
     `types.DocTypeRepo`)
  6. build Metadata REQUIRED keys `{sha, repo_full_name,
     message_subject, kind:"commit"}` + OPTIONAL keys when source
     non-nil (`author_name`, `author_email`, `committer_name`,
     `authored_date`, `committed_date`)
  7. surface `next_cursor` on the last doc when `nextPage > 0` via
     `strconv.Itoa(nextPage)` (existing convention)
- Reuse `safeStr`, `safeTime`, `truncateRunes`, `snippetMaxRunes`. No
  new helper unless it removes duplication.

### Step 4 — `internal/adapters/github/github.go` (1 line in Notes)

- Extend the `Capabilities.Notes` rate-ceiling sentence (currently
  `"Rate ceilings: code search 9/min vs issues/repos 30/min ..."`) to
  also mention commit search 30/min (e.g. append `"; commit search
  30/min"`).
- Do NOT change `DocTypes`, `RateLimitPerMin`, or any other field.
- The parent `TestCapabilitiesShape` (github_test.go:73) asserts only 2
  substrings (`"go-github"`, `"USEARCH_GITHUB_TOKEN"`) — those MUST remain
  present (append, do not rewrite).

### Step 5 — Testdata

- Create `internal/adapters/github/testdata/search_commits_response.json`
  — a `/search/commits` response with 3-5 items (one full commit, one
  with null GitHub `author`/`committer`, one minimal) each with `sha`,
  `commit.{message,author,committer}`, `html_url`, `repository`,
  `score`.
- Optionally `search_commits_response_pagination.json` if a Link-header
  pagination test needs a distinct body (otherwise the parse-level
  pagination tests construct the struct directly with `nextPage` injected).

### Step 6 — Tests

- `parse_test.go`: `TestParseCommitResultsFieldMapping` (table ≥3),
  `TestParseCommitNilSafe`, `TestParseCommitPaginationCursor`,
  `TestParseCommitNoCursorOnLastPage`.
- `search_test.go`: `TestSearchCommitIntentHappyPath`,
  `TestSearchCommitRateLimited`,
  `TestSearchCommitIntentAcceptedNotRejected`,
  `TestSearchInvalidIntentUpdatedMessage`.
- `github_test.go`: `TestCapabilitiesNotesCommitCadence`.
- No existing parent test asserts the `ErrInvalidIntent` message string
  (the only parent invalid-intent test,
  `TestSearchInvalidIntentRejectedNoHTTP` at search_test.go:754-779,
  checks only `Category==CategoryPermanent` and zero requests), so the
  message edit requires NO parent-test update. The new
  `TestSearchInvalidIntentUpdatedMessage` is the only test that inspects
  the message text.

---

## 3. Reuse vs New

| Concern | Reuse (unchanged) | New |
|---------|-------------------|-----|
| Routing skeleton | `Search`, intent resolution | `"commit"` in `validIntents` + `case` |
| Per-intent helper | `searchCode/Issues/Repos` shape | `searchCommits` |
| Parse | `parseRepoResults` template, `safe*` helpers, `truncateRunes`, `snippetMaxRunes` | `parseCommitResults` |
| Error taxonomy | `categorizeError`, `*SourceError` | — |
| Qualifier append | `appendQualifiers`, `buildQualifier` | — |
| Score | (neutral `0.5` constant) | — (no `normalizeScore` call) |
| HTTP client / redirect | `newDefaultHTTPClient`, `redirectAllowlist` | — |
| Dependency | `go-github/v73` `Search.Commits` | — (no go.mod change) |
| Capabilities | struct shape | `Notes` text only |
| Sentinel | `ErrInvalidIntent` var | message string only |

---

## 4. Technical Decisions (Locked at Plan Time)

| Decision | Rationale |
|----------|-----------|
| Target as-implemented `go-github/v73` (not parent §6.1's v85) | The shipped adapter is v73; `SearchService.Commits` is present. No version bump in scope. |
| Commits → `DocTypeRepo` (not new `DocTypeCommit`) | Enum addition is an SDK-boundary change; deferred (Open Question §11.1). Mirrors code-intent precedent. |
| Neutral `Score=0.5` | Commit search has no engagement signal; matches code-intent precedent (`parse.go:67`); RRF re-ranks by rank anyway. |
| Reuse `appendQualifiers` unchanged | No commit-specific qualifiers in v0.1 (scope discipline). `created:` vs `committer-date:` mismatch noted as Open Question §11.3. |
| `Notes` text extension, no `RateLimitPerMin` change | Single int can't express per-intent ceilings (parent §11.7). Commit shares the 30/min search bucket. |
| Update `ErrInvalidIntent` text in place | Keeps one sentinel; the message must enumerate the now-valid `commit` intent. |

---

## 5. Out-of-Scope Reminders (from spec.md §7)

- `DocTypeCommit` enum → deferred (Open Question §11.1).
- Per-intent rate-limit field → parent Open Question §11.7.
- Commit-content / diff fetch → deferred.
- Commit-specific qualifiers (`hash:`, `author:`, `committer-date:`) /
  sort → out of scope; Open Question §11.3.
- `*CommitResult.Score` normalization → neutral 0.5; Open Question §11.2.
- Any change to code/issues/repos intents → out of scope.
- New dependency / `go.mod` change → none.

---

## 6. Verification Gate (run phase)

- `go test ./internal/adapters/github/...` exits 0 (including all
  existing parent tests — no regression).
- `go test -race ./internal/adapters/github/...` exits 0.
- `go test -cover ./internal/adapters/github/...` ≥ 85% (new paths
  exercised by §2 Step 6 tests).
- `go vet` + `golangci-lint run` clean.
- `git diff` touches only the 4 source files + 2/3 test files + 1/2
  testdata fixtures. NO `go.mod` change. NO cross-package edits.

---

*End of SPEC-ADP-004a plan.md (v0.1, draft)*
