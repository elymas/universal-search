# SPEC-ADP-004a Acceptance Criteria

**SPEC**: SPEC-ADP-004a — GitHub Commit Search (Amendment to SPEC-ADP-004)
**Status**: draft
**Format**: Given/When/Then per REQ + edge cases + Definition of Done

---

## 1. Acceptance Scenarios by REQ

### REQ-ADP4a-001 — Commit Intent Routing (Optional)

**AC-001: commit is a valid intent**
- Given the `validIntents` map in `search.go`,
- When the package is built,
- Then `validIntents` contains the key `"commit"`.

**AC-002: commit intent hits /search/commits**
- Given a stub `httptest.Server` serving
  `testdata/search_commits_response.json` and recording the request path,
- When `Search(ctx, Query{Text:"fix bug", Filters:[{kind,"commit"}]})`
  is called,
- Then the observed URL path is `/search/commits`, the returned slice
  length equals the fixture item count, and every doc passes
  `Validate()` (returns nil).

**AC-003: per_page and page honour parent rules**
- Given `kind=commit`,
- When `MaxResults=500` → URL has `per_page=100`; `MaxResults=0` → URL
  has `per_page=25`; `Cursor="3"` → URL has `page=3`; `Cursor=""` → URL
  has `page=1`.
- Then the existing clamp/default logic (shared by all intents) applies
  unchanged to the commit intent.

### REQ-ADP4a-002 — CommitResult → NormalizedDoc Field Mapping (Ubiquitous)

**AC-004: Full-commit field mapping**
- Given a `*CommitResult` with `SHA`, `Commit.Message`,
  `Commit.Author.{Name,Email,Date}`, `Commit.Committer.{Name,Date}`,
  `HTMLURL`, `Repository.FullName`, and top-level `Author` (`*User`) all
  populated,
- When `parseCommitResults` runs,
- Then the produced `NormalizedDoc` has:
  - `ID == "github:commit:" + repoFullName + "@" + sha`
  - `SourceID == "github"`
  - `URL == HTMLURL`
  - `Title ==` first line of the message, ≤80 runes
  - `Body == Commit.Message` (full, unmodified)
  - `Snippet == truncateRunes(Commit.Message, 280)`
  - `PublishedAt == Commit.Author.Date` (UTC)
  - `Author == Commit.Author.Name`
  - `Score == 0.5`
  - `DocType == types.DocTypeRepo`
  - `Lang == ""`, `Hash == ""`
  - `Metadata` REQUIRED keys present:
    `{sha, repo_full_name, message_subject, kind:"commit"}`
  - `Metadata` OPTIONAL keys present when source non-nil:
    `{author_name, author_email, committer_name, authored_date}`

**AC-005: Author fallback to GitHub login**
- Given a commit with empty `Commit.Author.Name` but a non-nil
  top-level `Author.Login`,
- When parsed,
- Then `doc.Author == Author.Login`.

**AC-006: Nil Commit pointer is nil-safe and Validate-valid**
- Given a `*CommitResult` with `Commit == nil` but a non-nil `SHA` and a
  non-nil `Repository.FullName`,
- When parsed,
- Then `doc.Title == ""`, `doc.Body == ""`, `doc.Snippet == ""`,
  `doc.PublishedAt` is the zero time, `doc.URL ==
  "https://github.com/" + repoFullName + "/commit/" + sha` (synthesized
  because `HTMLURL` may be absent), and `doc.Validate()` passes (the
  synthesized URL satisfies the non-empty-URL requirement); no panic.

**AC-007: Nil GitHub User pointers are nil-safe**
- Given a commit with `Author == nil` and `Committer == nil` (`*User`),
  but populated `Commit.Author`/`Commit.Committer` (`*CommitAuthor`) and a
  non-nil `SHA`+`Repository.FullName`,
- When parsed,
- Then `doc.Author == Commit.Author.Name`; `doc.URL` is non-empty
  (`HTMLURL` or the synthesized permalink); no panic; `Validate()` passes.

**AC-007b: Commit lacking both SHA and repo is skipped**
- Given a `*CommitResult` with `SHA == nil` AND `Repository == nil` (no
  deterministic URL can be synthesized),
- When parsed,
- Then that commit is SKIPPED (not emitted) — it does not appear in the
  returned slice; no NormalizedDoc that would fail `Validate()` is ever
  emitted.

### REQ-ADP4a-003 — Pagination Cursor (Event-Driven)

**AC-008: Cursor on last doc when NextPage > 0**
- Given a result parsed with `nextPage = 2`,
- When `parseCommitResults` runs,
- Then the LAST returned doc has `Metadata["next_cursor"] == "2"` and
  all earlier docs do NOT have the `next_cursor` key.

**AC-009: No cursor on last page**
- Given a result parsed with `nextPage = 0`,
- When parsed,
- Then no returned doc has a `next_cursor` key.

### REQ-ADP4a-004 — Rate-Limit Categorization Reuse (Event-Driven)

**AC-010: Commit rate-limit maps to CategoryRateLimited**
- Given a stub returning HTTP 403 with `X-RateLimit-Remaining: 0` and a
  future `X-RateLimit-Reset` for a `kind=commit` request,
- When `Search` is called,
- Then `errors.Is(err, types.ErrRateLimited)`,
  `srcErr.Category == CategoryRateLimited`, `srcErr.RetryAfter > 0`,
  and `srcErr.RetryAfter <= 90s`.
- And no commit-specific error-handling code exists — the existing
  `categorizeError` rosetta handles the commit path identically to the
  other intents.

### REQ-ADP4a-005 — Intent Validation with Updated Error Text (Unwanted)

**AC-011: Error text enumerates commit**
- `ErrInvalidIntent.Error() ==
  "github: kind filter must be one of: code, issues, repos, commit"`.

**AC-012: commit accepted, not rejected**
- Given `Filters=[{kind,"commit"}]`,
- When `Search` is called against a stub,
- Then the returned error is NOT `ErrInvalidIntent`, and the stub
  observes ≥1 HTTP request (the commit search is actually issued).

**AC-013: unknown intent still rejected with new message**
- Given `Filters=[{kind,"users"}]`,
- When `Search` is called,
- Then `errors.Is(err, ErrInvalidIntent)`, ZERO HTTP requests are
  observed, and the error chain message contains the substring
  `"commit"` (proving the updated text is wired).

### Capabilities Notes Extension

**AC-014: Notes advertises commit cadence**
- `Capabilities().Notes` contains a commit-cadence substring (e.g.
  `"commit search 30/min"`).

**AC-015: Existing Capabilities contract preserved**
- The 2 substrings actually asserted by the parent `TestCapabilitiesShape`
  (github_test.go:73) — `"go-github"` and `"USEARCH_GITHUB_TOKEN"` — are
  both still present (the Notes edit only appends commit-cadence text);
  `DocTypes == [DocTypeRepo, DocTypeIssue]` unchanged;
  `RateLimitPerMin == 30` unchanged. The parent `TestCapabilitiesShape`
  passes unmodified.

---

## 2. Edge Cases

**EC-001: Multi-line message subject extraction**
- A commit message `"fix: handle nil\n\nDetailed body..."` → `Title ==
  "fix: handle nil"` (first line only); `Body` retains the full
  multi-line message.

**EC-002: Empty commits slice**
- `CommitsSearchResult.Commits == nil` or `len == 0` → `parseCommitResults`
  returns `(nil, nil)` (or empty slice) with no error.

**EC-003: Composite ID stability across pages**
- The same commit (same `SHA` + repo) returned on different pages
  produces an identical `ID`.

**EC-004: created: qualifier preserved verbatim**
- `Filters=[{since,"2026-01-01T00:00:00Z"}]` with `kind=commit` →
  `appendQualifiers` still emits `created:>=2026-01-01T00:00:00Z`
  (unchanged behaviour; the commit-date qualifier mismatch is Open
  Question §11.3, NOT fixed here).

---

## 3. Coverage Matrix

| REQ-ID | EARS Pattern | Acceptance Criteria | Implementation | Test |
|--------|-------------|---------------------|----------------|------|
| REQ-ADP4a-001 | Optional | AC-001..003 | `search.go` (`validIntents`, `case`, `searchCommits`) | `TestSearchCommitIntentHappyPath` |
| REQ-ADP4a-002 | Ubiquitous | AC-004..007, AC-007b | `parse.go::parseCommitResults` | `TestParseCommitResultsFieldMapping`, `TestParseCommitNilSafe` |
| REQ-ADP4a-003 | Event-Driven | AC-008..009 | `parse.go::parseCommitResults` (cursor block) | `TestParseCommitPaginationCursor`, `TestParseCommitNoCursorOnLastPage` |
| REQ-ADP4a-004 | Event-Driven | AC-010 | `client.go::categorizeError` (reused) | `TestSearchCommitRateLimited` |
| REQ-ADP4a-005 | Unwanted | AC-011..013 | `errors.go::ErrInvalidIntent` (text), `search.go` (`validIntents`) | `TestSearchCommitIntentAcceptedNotRejected`, `TestSearchInvalidIntentUpdatedMessage` |
| — | — | AC-014..015 | `github.go::Capabilities.Notes` | `TestCapabilitiesNotesCommitCadence` + parent `TestCapabilitiesShape` |

---

## 4. Definition of Done

- [ ] `validIntents` contains `"commit"`; dispatch `switch` has a
      `case "commit"` arm.
- [ ] `(*Adapter).searchCommits` helper exists, mirroring
      `searchCode`/`searchIssues`/`searchRepos`.
- [ ] `parseCommitResults` exists with the §6.2 field mapping and full
      nil-guard contract.
- [ ] `ErrInvalidIntent.Error()` enumerates `commit`.
- [ ] `Capabilities().Notes` advertises the commit cadence; the 2
      substrings asserted by parent `TestCapabilitiesShape` (`"go-github"`,
      `"USEARCH_GITHUB_TOKEN"`) still present; `DocTypes`/`RateLimitPerMin`
      unchanged.
- [ ] `testdata/search_commits_response.json` fixture added.
- [ ] All new tests (REQ-ADP4a-001..005 + Capabilities) pass.
- [ ] All existing parent tests still pass (no regression), including
      `TestCapabilitiesShape` and the updated invalid-intent test.
- [ ] `go test ./internal/adapters/github/...` exits 0.
- [ ] `go test -race ./internal/adapters/github/...` exits 0.
- [ ] `go test -cover ./internal/adapters/github/...` ≥ 85%.
- [ ] `go vet` and `golangci-lint run` clean.
- [ ] `@MX:ANCHOR` on `parseCommitResults`; `@MX:NOTE` on its nil-guard
      block; both `@MX:SPEC: SPEC-ADP-004a`.
- [ ] No `go.mod` change. No cross-package edits. Diff confined to
      `internal/adapters/github/`.
- [ ] SPEC status promoted to `implemented` (run phase).

---

*End of SPEC-ADP-004a acceptance.md (v0.1, draft)*
