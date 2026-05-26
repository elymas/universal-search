# SPEC-ADP-004 Implementation Plan (Post-Hoc)

**SPEC**: SPEC-ADP-004 — GitHub Adapter
**Status**: implemented (2026-05-07)
**Methodology**: TDD (RED → GREEN → REFACTOR)
**Coverage Target**: 85%
**Owner**: expert-backend
**Priority**: P0

---

## 1. Overview

ADP-004 is the FIRST authenticated adapter and the FIRST multi-intent
adapter in MoAI Universal Search. It consumes GitHub's REST search
API via `github.com/google/go-github/v85` and dispatches across three
search modes (code / issues / repos) within a single Search method,
routed by `Query.Filters[Key="kind"]`.

Key deltas versus ADP-001/002/003:

1. **PAT authentication** — `Capabilities.RequiresAuth=true` and
   `AuthEnvVars=["USEARCH_GITHUB_TOKEN"]`. The registry's
   `RegisterWithOptions` validates env presence at startup
   (`internal/adapters/registry.go:123-129`); the constructor
   independently validates `Options.Token` presence (two independent
   gates, not redundant).
2. **Multi-intent routing** — `kind` filter ∈ `{code, issues, repos}`,
   default `repos`. Each intent dispatches to a different go-github
   call (`client.Search.Code` / `client.Search.Issues` /
   `client.Search.Repositories`) and is parsed by its own
   intent-specific function (`parseCodeResults`, `parseIssueResults`,
   `parseRepoResults`).
3. **Path B chosen over Path A** — direct go-github REST client
   rather than wrapping `github/github-mcp-server`. Roadmap entry
   `.moai/project/roadmap.md:49` suggested Path A; this SPEC
   documented the deviation rationale in research.md §2.
4. **Typed go-github error rosetta** — `categorizeError()` uses
   `errors.As` against `*github.RateLimitError`,
   `*github.AbuseRateLimitError`, `*github.ErrorResponse` to produce
   `*SourceError` with the correct Category. Retry-After cap raised
   from 60s (Reddit/HN) to 90s for GitHub's secondary-rate-limit
   recovery semantics.
5. **Per-intent score synthesis** — code uses `int(*CodeResult.Score)`,
   issues use `Comments * 10` (engagement proxy), repos use
   `StargazersCount`. All pass through the same Tanh formula.

The adapter is one-shot per call: no fanout, no retry, no caching,
no ranking fusion, sole-emitter discipline preserved.

---

## 2. Architecture

### 2.1 Package Layout

```
internal/adapters/github/
├── github.go        — Adapter, Options, New (validates token), Name, Capabilities, Healthcheck
├── github_test.go   — interface conformance + Capabilities + auth declaration + New validation
├── search.go        — (*Adapter).Search hot path + intent dispatch + appendQualifiers
├── search_test.go   — E2E + per-intent happy paths + error categorisation + filter qualifier tests
├── client.go        — *http.Client construction (10s timeout, reqid Transport, redirect allowlist), categorizeError
├── client_test.go   — categorizeError table + redirect allowlist + UA/Authorization headers
├── parse.go         — parseCodeResults / parseIssueResults / parseRepoResults + safe* nil-guard helpers
├── parse_test.go    — per-intent field mapping (3 sub-tables × 5 fixtures each)
├── score.go         — normalizeScore Tanh formula (verbatim from ADP-001)
├── score_test.go    — score normalization table
├── errors.go        — 4 sentinels (ErrInvalidQuery, ErrInvalidCursor, ErrInvalidIntent, ErrMissingToken) + parseRetryAfter (90s cap)
├── bench_test.go    — BenchmarkParseGitHubResponse25Results + TestMain goleak
└── testdata/        — 10 JSON fixtures (code, issues, repos, deleted-user, abuse, validation, malformed)
```

### 2.2 Key Data Structures

**`Adapter` struct** (`github.go`): `ghClient *gogithub.Client`,
`httpClient *http.Client`, `baseURL string`, `userAgent string`,
`healthcheckTarget string`. All unexported; immutable
post-construction.

**`Options` struct**: `BaseURL` (test override pointing at
httptest.Server), `HTTPClient`, `Token` (PAT; required unless
`SkipAuthCheck=true`), `UserAgentVersion`, `HealthcheckTarget`,
`SkipAuthCheck` (test-only).

**Constructor validation**: when `!SkipAuthCheck && HTTPClient ==
nil && Token == ""`, returns `*types.SourceError{Category:
CategoryPermanent, Cause: ErrMissingToken}`. The go-github client
gets `WithAuthToken(opts.Token)` and `client.UserAgent = ua`.

**Sentinels** (`errors.go`): `ErrInvalidQuery`, `ErrInvalidCursor`,
`ErrInvalidIntent`, `ErrMissingToken`. All wrapped in
`*types.SourceError{Category: CategoryPermanent}`.

**`categorizeError(err)` rosetta** (`client.go`):
- `errors.As(err, &*RateLimitError)` → `CategoryRateLimited` with
  `RetryAfter = time.Until(Rate.Reset)` capped at 90s, default 5s.
- `errors.As(err, &*AbuseRateLimitError)` → `CategoryRateLimited`
  with `RetryAfter = *abuseErr.RetryAfter` capped at 90s.
- `errors.As(err, &*ErrorResponse)` → 4xx → `CategoryPermanent`;
  5xx → `CategoryUnavailable`.
- Default → `CategoryUnavailable` with `HTTPStatus=0`.

### 2.3 Hot-Path Flow (REQ-ADP4-002)

1. Validate `q.Text` (REQ-ADP4-008).
2. Parse `q.Cursor` as positive int (≥1) via `strconv.Atoi`.
3. Resolve `kind` filter (default `repos`); reject unknown values.
4. Apply `appendQualifiers(q.Text, q.Filters)` per §6.4 — 8
   recognised keys (`since`, `language`, `repo`, `org`, `user`,
   `topic`, `state`, `is_pr`) translated to GitHub search syntax
   qualifiers and appended to the query string.
5. Build `ListOptions{PerPage: clamp(q.MaxResults, 1, 100), Page:
   parsedCursor}`; default `Page=1` when cursor empty.
6. Dispatch to `client.Search.Code` / `Issues` / `Repositories`.
7. On error: `categorizeError(err)` → `*SourceError`.
8. On success: call the intent-specific `parseXxxResults()` →
   `[]NormalizedDoc`; surface `next_cursor =
   strconv.Itoa(*Response.NextPage)` on last doc when `NextPage > 0`.

### 2.4 Per-Intent Field Mapping (REQ-ADP4-005)

See spec.md §6.3.1 (code), §6.3.2 (issue/PR), §6.3.3 (repo).
Highlights:
- **code**: `ID = fullname@sha:path` composite; `DocType=DocTypeRepo`
  (Open Question §11.1 deferred adding `DocTypeCode` enum); `Body=""`
  (no content fetch in v0.1).
- **issue/PR**: `ID = "github:issue:" + strconv.FormatInt(*ID, 10)`;
  `DocType=DocTypeIssue` regardless of issue-vs-PR;
  `Metadata["is_pull_request"]` distinguishes; markdown body
  preserved unchanged (no HTML stripping).
- **repo**: `ID = "github:repo:" + strconv.FormatInt(*ID, 10)`;
  `DocType=DocTypeRepo`; `Author = Owner.Login`.

All three use `safeStr`, `safeInt`, `safeBool`, `safeTime` helpers
(`parse.go`) to nil-guard `*string`, `*int`, `*bool`, `*Timestamp`
fields returned by go-github for nullable values.

### 2.5 Integration Points

- **Consumed by**: `internal/adapters/registry.go::wrappedAdapter`
  (sole-emitter pattern); registry's `RegisterWithOptions` validates
  `AuthEnvVars` presence at startup
  (`internal/adapters/registry.go:123-129`) — ADP-004 is the FIRST
  real consumer of this path.
- **Consumes**: `pkg/types`, `github.com/google/go-github/v85`,
  `internal/obs/reqid.NewTransport`.
- **Downstream**: SPEC-FAN-001 fanout; SPEC-IDX-001 RRF; SPEC-AUTH-002
  (M6) may swap PAT for GitHub App authentication.

### 2.6 New External Dependency

`github.com/google/go-github/v85` added as direct dep in `go.mod`;
pulls `github.com/google/go-querystring` transitively. Pinning policy
mirrors `.claude/rules/moai/core/lsp-client.md`: bumping requires
running the integration test suite first.

---

## 3. Test Coverage Notes

- Coverage meets 85% target.
- 64 representative tests per spec.md §8 TDD Plan.
- Per-intent happy-path tests against 3 separate fixtures
  (`search_code_25.json`, `search_issues_25.json`,
  `search_repos_25.json`).
- Nil-safety tests: `TestParseDeletedUserNilSafe`,
  `TestParseNoLanguageNilSafe` against
  `search_issues_nil_user.json` and
  `search_repos_nil_owner.json`.
- Token-leak prevention: `TestSearchTokenNotInErrorMessage` (stub
  forced 401 echoes Authorization header in body; assert token
  substring absent from `*SourceError.Error()`) and
  `TestSearchTokenNotInSlogOutput`.
- `BenchmarkParseGitHubResponse25Results` median ≤ 5ms,
  allocs/op ≤ 625 (higher than ADP-001/002's 500 due to
  `*Repository`'s 12+ nullable pointer fields).
- `TestSearchNoLeakedFileDescriptors` — counts process FDs before
  and after 100 Search calls; delta ≤ 5.

---

## 4. Technical Decisions (Locked During Implementation)

| Decision | Rationale |
|----------|-----------|
| Path B (direct go-github) over Path A (MCP wrap) | MCP's async streaming model mismatches the synchronous Adapter contract; no Go client provided by github-mcp-server; operational overhead of one extra container outweighs abstraction benefit for one source. |
| 90s Retry-After cap (vs 60s in Reddit/HN) | GitHub's secondary-rate-limit recovery documents several minutes; 90s is a conservative ceiling keeping user-facing tail latency reasonable. |
| Code hits → `DocTypeRepo` (not `DocTypeCode`) | Adding `DocTypeCode` enum is a breaking-ish SDK boundary change (`pkg/types/capabilities.go`). Deferred to SPEC-IDX-001 author if RRF tuning needs the distinction. |
| Single `RateLimitPerMin=30` despite code-search 9/min ceiling | `Capabilities` only has one int. The conservative 30/min is advertised; code-search caveat documented in `Notes`. Per-intent ceiling field deferred (Open Question §11.7). |
| Two independent auth-validation gates (registry env + constructor token) | The CLI reads the env and passes via `Options.Token`; registry catches env-missing case (`ErrMissingAuth`); constructor catches token-not-passed case (`ErrMissingToken`). Neither alone is sufficient. |
| Markdown body preserved (no HTML strip) | GitHub returns markdown for issues/PRs; synthesis consumes markdown directly. HN's `stripHTML` NOT inherited. |

---

## 5. Risks Mitigated

- **Token missing at runtime** → registry validates `AuthEnvVars` at
  startup; constructor double-checks `Options.Token`.
- **Token logged inadvertently** → go-github does not echo headers
  in error messages; explicit tests assert token substring absent
  from `*SourceError.Error()` and slog records.
- **Abuse vs forbidden 403 ambiguity** → typed-error
  discrimination via `errors.As(err, &abuseErr)`.
- **Code-search 9/min vs declared 30/min** → documented in
  `Capabilities.Notes`; per-intent ceiling deferred.
- **Nil-pointer dereference on go-github nullable fields** → 4
  `safeXxx` helpers wrap every nullable access.

---

## 6. Out-of-Scope Reminders (from spec.md §7)

- Code-content fetch follow-up → deferred (Open Question §11.2).
- GitHub App authentication → SPEC-AUTH-002 (M6).
- OAuth user-flow → SPEC-AUTH-001 (M6).
- Private-repository search → SPEC-AUTH-002.
- GraphQL v4 variant → never in v0.1 or v1.
- MCP wrapping (Path A) → REJECTED; revisit door open via Open
  Question §11.4.

---

*End of SPEC-ADP-004 plan.md (post-hoc, v1.0)*
