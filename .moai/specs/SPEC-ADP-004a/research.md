# SPEC-ADP-004a Research — GitHub Commit Search Amendment

**SPEC**: SPEC-ADP-004a — GitHub Commit Search (Amendment to SPEC-ADP-004)
**Date**: 2026-06-17
**Author**: limbowl via manager-spec

---

## 1. Goal

Add a fourth GitHub search intent — `commit` — to the implemented
SPEC-ADP-004 adapter, routing `Query.Filters[kind=commit]` to GitHub's
`/search/commits` endpoint via the already-pinned `go-github/v73`
`SearchService.Commits`. Port the surface from github/github-mcp-server
v1.1's `search_commits` tool.

---

## 2. Upstream Evidence (Port Source)

### 2.1 github/github-mcp-server `search_commits`

Verified via WebFetch of
`https://github.com/github/github-mcp-server/blob/main/pkg/github/search.go`:

- A `search_commits` tool IS defined.
- It calls `client.Search.Commits(ctx, query, opts)` — the SAME go-github
  method this amendment uses.
- It targets GitHub's commit-search endpoint (`/search/commits`).
- Returns commit data (sha, message, author, committer, repository,
  html_url, score) via a `MinimalCommitSearchItem` projection.
- Accepts qualifiers `repo:owner/repo`, `author:`, `committer-date:`,
  `hash:`; supports sort by `author-date`/`committer-date`.

Implication for this amendment: the port is faithful — same method, same
endpoint. We deliberately do NOT port the commit-specific qualifiers
(`author:`, `committer-date:`, `hash:`) or sort options in v0.1 (scope
discipline; see Open Question §11.3). We reuse the parent's existing
8-key `appendQualifiers` unchanged.

### 2.2 GitHub REST `/search/commits`

`https://docs.github.com/en/rest/search/search#search-commits` —
the endpoint returns `total_count`, `incomplete_results`, and an `items`
array of commit objects. Each item carries `sha`, a nested `commit`
object (`message`, `author`, `committer`, with `name`/`email`/`date`),
top-level `author`/`committer` GitHub `User` objects (may be null when
the commit author has no GitHub account), `repository`, `html_url`, and
`score`. Commit search shares GitHub's 30/min secondary-rate-limit
search bucket (distinct from the 9/min code-search sub-ceiling).

---

## 3. Verified Current Code State (as-implemented)

Read directly from the working tree at draft time (not assumed):

### 3.1 `internal/adapters/github/search.go`

- Lines 24-29: `validIntents = {code, issues, repos}` (map of
  `struct{}`). No `commit`.
- Lines 67-80: intent resolved from `Filters[Key="kind"]`, default
  `repos`; unknown value → `*SourceError{Permanent, ErrInvalidIntent}`.
- Lines 103-110: dispatch `switch intent` over `code`/`issues`/default
  `repos`. No `commit` case.
- Lines 113-180: `searchCode`/`searchIssues`/`searchRepos` helpers —
  identical shape: call `a.ghClient.Search.<X>`, on error try
  `categorizeError` then raw err, capture `resp.NextPage`, call
  `parse<X>Results`, wrap parse errors in `*SourceError{Permanent}`.
  `searchCommits` will mirror this exactly.
- Lines 203-281: `appendQualifiers` / `buildQualifier` — the 8-key
  qualifier set (`since`→`created:>=`, `language`, `repo`, `org`, `user`,
  `topic`, `state`, `is_pr`) with per-intent applicability guards.
  Reused unchanged.

### 3.2 `internal/adapters/github/parse.go`

- `parseCodeResults`, `parseIssueResults`, `parseRepoResults` exist.
  `parseRepoResults` (lines 182-263) is the closest structural template
  for `parseCommitResults` (iterate slice, nil-guard, build Metadata,
  surface `next_cursor` on last doc).
- Line 67: code-intent uses `Score: 0.5` (neutral) — precedent for the
  commit intent's neutral score.
- Lines 15-16: `snippetMaxRunes = 280`.
- Lines 267-302: `safeStr(*string)`, `safeInt(*int)`,
  `safeInt64(*int64)`, `safeTime(*gogithub.Timestamp)`,
  `truncateRunes(string, int)`. All reused. NOTE: no `safeFloat` helper
  exists — irrelevant since the commit intent uses neutral `0.5`, not
  `*CommitResult.Score`.

### 3.3 `internal/adapters/github/errors.go`

- Line 23: `ErrInvalidIntent = errors.New("github: kind filter must be
  one of: code, issues, repos")`. Updated to append `, commit`. Sentinel
  identity unchanged (same `var`).

### 3.4 `internal/adapters/github/github.go`

- Lines 137-161: `Capabilities()` returns
  `DocTypes=[DocTypeRepo, DocTypeIssue]`, `RateLimitPerMin=30`, and a
  `Notes` string containing `"code search 9/min"` and
  `"issues/repos 30/min"`. The Notes rate-ceiling sentence is extended
  with the commit cadence; no struct field value changes.
- Lines 5-8: documented deviation — adapter ships on `go-github/v73`,
  not the `v85` referenced in parent §6.1. THIS amendment targets v73.

### 3.5 `go.mod`

- Line 94: `github.com/google/go-github/v73 v73.0.0`. Already a direct
  dep. No change needed — `SearchService.Commits` is in v73.

---

## 4. go-github v73 API Surface (verified in module cache)

Module cache: `/Users/masterp/go/pkg/mod/github.com/google/go-github/v73@v73.0.0/github/`

### 4.1 `search.go`

```go
// line 152
func (s *SearchService) Commits(ctx context.Context, query string, opts *SearchOptions) (*CommitsSearchResult, *Response, error)

// line 126
type CommitsSearchResult struct {
    Total             *int            `json:"total_count,omitempty"`
    IncompleteResults *bool           `json:"incomplete_results,omitempty"`
    Commits           []*CommitResult `json:"items,omitempty"`
}

// line 133
type CommitResult struct {
    SHA         *string     `json:"sha,omitempty"`
    Commit      *Commit     `json:"commit,omitempty"`
    Author      *User       `json:"author,omitempty"`
    Committer   *User       `json:"committer,omitempty"`
    Parents     []*Commit   `json:"parents,omitempty"`
    HTMLURL     *string     `json:"html_url,omitempty"`
    URL         *string     `json:"url,omitempty"`
    CommentsURL *string     `json:"comments_url,omitempty"`
    Repository  *Repository `json:"repository,omitempty"`
    Score       *float64    `json:"score,omitempty"`
}
```

### 4.2 `git_commits.go`

```go
// line 45
type Commit struct {
    SHA          *string                `json:"sha,omitempty"`
    Author       *CommitAuthor          `json:"author,omitempty"`
    Committer    *CommitAuthor          `json:"committer,omitempty"`
    Message      *string                `json:"message,omitempty"`
    // ... Tree, Parents, HTMLURL, URL, Verification, NodeID, CommentCount
}

// line 69
type CommitAuthor struct {
    Date  *Timestamp `json:"date,omitempty"`
    Name  *string    `json:"name,omitempty"`
    Email *string    `json:"email,omitempty"`
    Login *string    `json:"username,omitempty"` // webhook-only
}
```

Nullable-pointer surface to nil-guard: `CommitResult.Commit`,
`CommitResult.Author`, `CommitResult.Committer`, `CommitResult.SHA`,
`CommitResult.HTMLURL`, `CommitResult.Repository`, and nested
`Commit.Message`, `Commit.Author`, `Commit.Committer`,
`CommitAuthor.{Name,Email,Date}`.

---

## 5. Mapping Decisions

| Field | Source | Decision |
|-------|--------|----------|
| `ID` | `"github:commit:" + repoFullName + "@" + SHA` | Composite; mirrors code-intent composite ID style |
| `Title` | first line of `Commit.Message`, ≤80 runes | Subject line (git convention) |
| `Body` | full `Commit.Message` | unmodified |
| `Snippet` | `truncateRunes(Commit.Message, 280)` | reuse `snippetMaxRunes` |
| `PublishedAt` | `Commit.Author.Date` | authored date is the canonical commit time |
| `Author` | `Commit.Author.Name` (fallback `Author.Login`) | commit metadata is always present; GitHub `User` may be nil |
| `Score` | constant `0.5` | neutral; no engagement signal; matches code-intent precedent |
| `DocType` | `DocTypeRepo` | no `DocTypeCommit` enum; commits live in repos |

---

## 6. Scope Boundary Confirmation

This amendment adds exactly ONE intent. It does NOT:
- touch the other three intents
- add a dependency (v73 already has `Search.Commits`)
- add `Capabilities` struct fields (only `Notes` text)
- add commit-specific qualifiers or sort options
- add a `DocTypeCommit` enum
- normalize `*CommitResult.Score` (neutral 0.5)

These deferrals are recorded as Open Questions §11.1-§11.3 in spec.md.

---

## 7. Risks Surfaced

- Nil-pointer on nested `*Commit`/`*CommitAuthor`/`*User` → mitigated by
  explicit nil-guards + `safeStr`/`safeTime`; tested via nil-safe case.
- `created:` (from `since`) is not the natural commit-date qualifier
  (`committer-date:`) → out of scope; behaviour preserved; Open
  Question §11.3.
- Updating `ErrInvalidIntent` text → NO parent test asserts the exact
  message string. The only parent invalid-intent test
  (`TestSearchInvalidIntentRejectedNoHTTP`, search_test.go:754-779)
  checks ONLY `Category==CategoryPermanent` and zero outbound requests,
  so the message edit breaks nothing. Verified against the working tree.

---

*End of SPEC-ADP-004a research.md*
