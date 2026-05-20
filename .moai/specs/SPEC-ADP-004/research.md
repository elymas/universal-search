# SPEC-ADP-004 Research — GitHub Adapter

Research artifact for SPEC-ADP-004 (GitHub adapter, M3). Produced during the
plan phase to inform EARS requirements before drafting spec.md. Every external
claim is URL-cited; every internal claim is file:line-cited per
`.moai/config/sections/research.yaml`.

---

## 0. Research Mandate

SPEC-ADP-004 is the **fourth real adapter** in the M3 batch and the FIRST
adapter to ship `RequiresAuth=true` plus `AuthEnvVars=[…]` in the
SPEC-CORE-001 contract — every prior adapter (Reddit, HN) shipped against a
fully public no-auth endpoint. This research document provides the SPEC author
with:

1. The two integration paths under evaluation:
   - **Path A**: wrap the official `github/github-mcp-server`
     (https://github.com/github/github-mcp-server) — distributed as binary +
     Docker image + remote HTTP service, communicates via MCP `stdio` or HTTP
     transport.
   - **Path B**: call GitHub REST v3 (or GraphQL v4) directly from Go, using
     `github.com/google/go-github` as the client library
     (https://github.com/google/go-github).
2. A definitive recommendation with file-cited rationale.
3. Mapping tables for `code` search hits, `issues` search hits, and
   `repository` search hits to `pkg/types.NormalizedDoc`.
4. Authentication model selection (PAT vs GitHub App vs OAuth) with
   environment-variable plumbing.
5. Rate-limit semantics in detail (primary 5000/hr authenticated, secondary
   30/min per search, per-IP concurrent-100 ceiling) and the categorisation
   discipline that maps each failure mode to one of the four
   `*types.SourceError` Categories.
6. The set of known risks and Open Questions that the SPEC must either resolve
   or carry forward.

Output: this research artifact. No invented facts. Every claim file-cited
(e.g., `internal/adapters/reddit/reddit.go:135`) or URL-cited from a
WebFetch-verified source. The SPEC author (also limbowl, also via
manager-spec) will draft `.moai/specs/SPEC-ADP-004/spec.md` from this
artifact, mirroring the SPEC-ADP-001/002 reference shape verbatim where
behaviour is identical, deviating only where GitHub-specific semantics demand.

---

## 1. Existing-Pattern Citations (SPEC-ADP-001/002 Reference Shape)

SPEC-ADP-004 inherits the file layout, error mapping discipline, MX tag plan,
and TDD harness from SPEC-ADP-001 (Reddit) verbatim. SPEC-ADP-002 (Hacker
News) was the second-adapter validation that the reference shape is portable;
ADP-004 is the third application of the same pattern with two new wrinkles:

- First **authenticated** adapter (PAT via env var `USEARCH_GITHUB_TOKEN`).
- First adapter that produces **multiple intent classes** within one source
  (code search → "code" intent; issue/PR search → "social" intent;
  repository search → both).

Implementation files in `internal/adapters/reddit/` and `internal/adapters/hn/`
define the canonical package shape (cited by line):

| File | Role | Lines | Reused As-Is by ADP-004? |
|------|------|-------|--------------------------|
| `internal/adapters/reddit/reddit.go:1-136` | Adapter struct, New, Name, Capabilities, Healthcheck, compile-time interface assertion | 136 | Pattern; substitute `"reddit"` → `"github"`, base URL, defaults; ADD `RequiresAuth=true` + `AuthEnvVars=["USEARCH_GITHUB_TOKEN"]` |
| `internal/adapters/reddit/search.go:1-167` | Search hot path | 167 | Pattern; rewrite for go-github client invocation, branch on intent (code vs issues vs repos) |
| `internal/adapters/reddit/client.go:1-125` | HTTP client + redirect allowlist + categorizeStatus | 125 | Mostly pattern; `categorizeStatus` adopted with adapter name swap; redirect allowlist tuned to `{api.github.com, github.com, raw.githubusercontent.com}`; HTTP client managed inside go-github (replaces our custom `*http.Client`) |
| `internal/adapters/reddit/parse.go:1-203` | JSON → NormalizedDoc transform | 203 | Pattern; rewrite for `github.CodeResult`, `github.Issue`, `github.Repository` types from `go-github` package |
| `internal/adapters/reddit/score.go:1-41` | Tanh score formula | 41 | Verbatim formula; HN inherited it (SPEC-ADP-002 §2.3); ADP-004 inherits same constants for repo-stars / issue-comments scoring |
| `internal/adapters/reddit/errors.go:1-64` | parseRetryAfter helper, ErrInvalidQuery sentinel | 64 | Pattern; sentinel variants `ErrInvalidQuery` + `ErrInvalidIntent` (new — see §2.4); `parseRetryAfter` adopted but with a 90s cap (GitHub recommends honouring up to several minutes per its rate-limit guidance) |
| `internal/adapters/reddit/bench_test.go` | NFR benchmark | — | Pattern; `BenchmarkParseGitHubResponse25Results` |

The compile-time interface assertion pattern at
`internal/adapters/reddit/reddit.go:135` (`var _ types.Adapter =
(*Adapter)(nil)`) is mandatory.

The registry's wrappedAdapter at `internal/adapters/registry.go:172-263`
emits ALL observability — counter, histogram, span, slog — so the GitHub
adapter, like Reddit and HN, MUST emit nothing of its own. Sole-emitter
discipline preserved verbatim.

The error taxonomy is provided by `pkg/types/errors.go:14-218`
(SPEC-CORE-001 REQ-CORE-008): four sentinels (`ErrTransient`,
`ErrPermanent`, `ErrRateLimited`, `ErrSourceUnavailable`), `Category` enum,
typed `*SourceError`, and `OutcomeFromError` for the Prometheus label. The
GitHub adapter consumes this contract identically to Reddit/HN. The two new
GitHub-specific HTTP statuses to map are 422 (Unprocessable Entity →
`CategoryPermanent`, query syntax error) and the abuse-detection 403 (which
go-github surfaces as `*github.AbuseRateLimitError` with `RetryAfter` →
`CategoryRateLimited`).

The `pkg/types.NormalizedDoc` 15-field struct
(`pkg/types/normalized_doc.go:40-56`) is the return shape. ADP-004 maps three
distinct GitHub search response shapes onto this single canonical struct (see
§3 below).

---

## 2. Two Integration Paths Evaluated

### 2.1 Path A — Wrap `github/github-mcp-server`

**Distribution shape** (verified via WebFetch of
https://github.com/github/github-mcp-server):
- Docker image: `ghcr.io/github/github-mcp-server`
- Self-built binary: `go build` in `cmd/github-mcp-server`
- Remote HTTP server hosted at `https://api.githubcopilot.com/mcp/`
- Communicates via **MCP protocol over stdio** (subprocess) OR HTTP (remote)

**Tools provided** (all relevant to ADP-004 scope):
- `search_code` — code search
- `search_issues` — issues + PRs
- `search_pull_requests` — PRs specifically
- `search_repositories` — repo metadata + filters
- `search_users` — out of scope here

**Authentication**: PAT, OAuth (remote only), GitHub Apps (implied).

**Cost of Path A**:

1. **Operational complexity**: a Go process running `usearch` would have to
   either (a) spawn `github-mcp-server` as a subprocess and pipe MCP frames
   over stdio, OR (b) connect to a remote HTTP MCP service. Both are heavier
   than a single in-process function call.
2. **No Go client library is provided** by the MCP server (verified by
   WebFetch: "No Go client library is provided — it's consumed through MCP
   protocol communication"). MoAI would have to implement an MCP
   client-side stdio transport (or HTTP transport) plus session management
   plus tool-call serialisation. For one source. The implementation cost is
   roughly `internal/llm/client.go`-shaped — multiple hundred LoC of
   plumbing per the `internal/llm/client.go:31-65` reference shape.
3. **Adapter contract mismatch**: `pkg/types.Adapter` is a 4-method
   synchronous interface returning `[]NormalizedDoc`. MCP is async streaming
   tool calls with JSON-RPC framing. We would have to bridge those models in
   the adapter, which means the MCP transport leaks complexity into a
   contract designed for a synchronous HTTP shape.
4. **Operational footprint**: docker-compose.yml or k8s pods grow by one
   service. The dev stack (`.moai/project/structure.md:60-64`) currently has
   Qdrant, Meili, PG, SearXNG, LiteLLM, Redis. Adding github-mcp-server is
   one more thing to start, monitor, version, and trouble-shoot — for a
   single source.
5. **Versioning**: github-mcp-server ships its own release cadence; we'd
   pin a Docker image tag (or git SHA for binary build), and the MCP tool
   schema may change between versions in ways that affect our parser. By
   contrast, `go-github` follows GitHub's published API version
   (2022-11-28 at the time of v85.0.0) and bumps via Go module versioning
   that we already track.

**Benefit of Path A**:
- Free GitHub-specific abstractions (MCP tools encapsulate query syntax).
- Future MCP-aware orchestrators could compose with us at the protocol layer.
- Aligns with the long-term roadmap entry SPEC-MCP-001 (M7 per
  `.moai/project/roadmap.md:91`).

**Verdict**: Path A is **rejected for v0.1**. The operational complexity and
the protocol-mismatch make it a poor fit for the M3 thin-slice exit criterion
(`.moai/project/roadmap.md:150`: "`usearch query` returns fused results
across ≥5 adapters"). The project's MCP exposure (SPEC-MCP-001) is OUTBOUND
(usearch hosts an MCP server for clients), not INBOUND (usearch consuming an
external MCP server). Mixing the two creates an awkward layering. Defer Path
A as a future enhancement (SPEC-ADP-004a) if measured value warrants — e.g.,
if GitHub deprecates the REST `/search/code` endpoint in favour of an MCP-only
path.

### 2.2 Path B — Direct GitHub REST API via `google/go-github`

**Distribution shape** (verified via WebFetch of
https://github.com/google/go-github):
- Stable release **v85.0.0** (April 20, 2026).
- Import path: `github.com/google/go-github/v85/github`.
- Pure Go library; consumed via `go get` / `go.mod`.
- Tracks GitHub REST API v3 with the 2022-11-28 date-based version.

**Coverage**:
- `search_code`, `search_issues`, `search_repositories` all surfaced via
  `client.Search.Code(ctx, query, opts)`,
  `client.Search.Issues(ctx, query, opts)`, and
  `client.Search.Repositories(ctx, query, opts)`.
- `*Response` struct includes rate-limit headers (`Rate.Limit`,
  `Rate.Remaining`, `Rate.Reset`) and pagination state (`NextPage`,
  `LastPage`).
- Built-in `RateLimitError` (primary) and `AbuseRateLimitError` (secondary)
  typed errors with `RetryAfter` field — direct map to
  `types.CategoryRateLimited` with `*SourceError.RetryAfter` populated.

**Authentication**:
- `client := github.NewClient(nil).WithAuthToken(pat)` for PAT — the
  one-line v85 idiom.
- GitHub App auth via `bradleyfalzon/ghinstallation` integration — out of
  v0.1 scope (Open Question §7.3).
- OAuth via standard HTTP client injection — same out-of-scope deferral.

**Pagination**:
- `ListOptions{Page: int, PerPage: int}` for page-based (search uses this).
- Returned `*Response.NextPage` (zero when no more pages) — surfaced via
  `Metadata["next_cursor"]` on the LAST returned doc, encoded as
  `strconv.Itoa(NextPage)` (mirrors HN's `strconv.Itoa(currentPage + 1)`
  pattern from SPEC-ADP-002 §6.3).
- Page-based pagination is governed by the **Link header** parsed
  internally by go-github; we do not parse the Link header ourselves.

**Cost of Path B**:
- **One new module dependency**: `github.com/google/go-github` (and its
  transitive `github.com/google/go-querystring` runtime dep). go.mod entry
  required.
- **Adapter must wire the HTTP client + token**: the `github.NewClient(nil)`
  default uses `http.DefaultTransport`; we want our `reqid.NewTransport`
  wrapper for request-ID propagation (mirrors
  `internal/llm/client.go:51-54` and the Reddit adapter at
  `internal/adapters/reddit/client.go:50-56`). go-github accepts a custom
  `*http.Client` via `github.NewClient(httpClient)`. Easy.

**Benefit of Path B**:
- **Direct fit** with the `pkg/types.Adapter` synchronous interface — no
  protocol bridging.
- **Mature library**: 13k+ stars, used in production by Kubernetes,
  Argo CD, Atlantis, and dozens of CNCF projects. Verified API
  surface coverage 100% of v0.1 needs.
- **First-class typed errors**: `*github.RateLimitError` and
  `*github.AbuseRateLimitError` map cleanly to our `*SourceError`. The
  registry wrappedAdapter at `internal/adapters/registry.go:204-218` already
  consumes errors via `OutcomeFromError`; integration is one-step.
- **Zero operational footprint addition**: no new docker container, no
  IPC, no version skew between two binaries.
- **Pinning discipline**: go-github follows semver; major-version path
  (`v85`) is import-path-encoded so accidental upgrades are impossible.

**Verdict**: Path B is the **chosen approach for v0.1**. The decision is
revisable (see Open Question §7.4) if Path A's operational properties
become a measured win in M7 (when SPEC-MCP-001 lands).

### 2.3 REST vs GraphQL Within Path B

GitHub publishes both a REST v3 surface (https://docs.github.com/en/rest)
and a GraphQL v4 surface (https://docs.github.com/en/graphql).

**REST trade-offs**:
- Each search endpoint is a separate path: `/search/code`, `/search/issues`,
  `/search/repositories`. ADP-004 needs all three.
- Pagination via `Link` header (parsed by go-github); standard.
- Rate limits are per-resource: the search resource has its own
  `X-RateLimit-Resource: search` bucket (30 req/min authenticated; per
  https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api).
- Response schemas are documented per endpoint with stable field names.

**GraphQL trade-offs**:
- One endpoint (`/graphql`); query shape determines what's fetched.
- Pagination via `pageInfo { endCursor, hasNextPage }`; opaque cursor.
- Rate-limit accounting is **point-cost based**: each query accumulates
  points up to a 5000-point hourly budget (different from REST's request
  count). `query { rateLimit { cost remaining resetAt } }` introspects.
- Response shape is dictated by the query — we'd write three GraphQL queries
  (one per intent), each with explicit field selection. More maintenance
  surface in the SPEC.
- `go-github` supports REST exclusively. GraphQL would require
  `shurcooL/githubv4` or hand-rolled queries.

**Verdict**: REST + go-github is the chosen approach. GraphQL's main
advantage (over-fetch reduction) doesn't matter at our scale: the search
endpoints return modest-size payloads (tens of KB) and we map a fixed set
of fields per response. GraphQL's main disadvantage (point-cost
accounting) introduces operational complexity not justified by v0.1's needs.
Open Question §7.5 keeps the door open for a future SPEC-ADP-004b GraphQL
variant if the measured search-endpoint rate ceiling (30/min) becomes
binding.

### 2.4 Intent Routing — Code vs Social

The GitHub adapter is the FIRST adapter to produce results across two
intent classes (per the roadmap M3 entry):

- **code** intent: code search hits (`/search/code` results: file path, repo,
  language, line numbers when available).
- **social** intent: issues + PRs (`/search/issues`, also covers PRs because
  PRs are issues with a `pull_request` field).

Repository search (`/search/repositories`) is metadata-shaped and arguably
serves both intents (a repo is "code" but the search request often originates
from a social-intent query like "react state management library").

**Routing mechanism**: `pkg/types.Query.Filters` carries one filter
`Key="kind"` with `Value` ∈ `{"code", "issues", "repos"}`. When omitted,
default is `"repos"` (broadest, lowest auth requirement, doesn't hit the
`/search/code` 9-req/min ceiling).

The Capabilities descriptor declares
`DocTypes=[DocTypeRepo, DocTypeIssue]` — the IR-001 router (per
SPEC-IR-001 REQ-IR-008) intersects `categoryEligibleDocTypes` with
`Capabilities.SupportedLangs` and the adapter's `DocTypes` slice to decide
whether GitHub is in the AdapterSet for a given category. GitHub is eligible
for `code` and `social` and `mixed` categories; ineligible for `korean`
(SupportedLangs is nil → IR-001 treats nil as language-agnostic, so this
gate is permissive).

**Important**: A new `DocType` value may be needed. The current enum is
`{Article, Post, Paper, Video, Repo, Issue, Social, Other}` per
`pkg/types/capabilities.go:14-23`. `DocTypeRepo` and `DocTypeIssue` already
exist. Code search results don't fit cleanly — they're file fragments, not
documents. Open Question §7.1 documents this; v0.1 maps code hits to
`DocTypeRepo` (code lives in repos, and the URL points to the file in a
repo) and stores the file path + line numbers in `Metadata`. A future
`DocTypeCode` enum value would require amending `pkg/types/capabilities.go`
which is the SDK boundary per `.moai/project/structure.md:160`. Out of v0.1.

---

## 3. NormalizedDoc Field Mapping (Three Intents)

The adapter transforms three distinct GitHub response shapes onto the single
15-field NormalizedDoc canonical contract. Field semantics from
`pkg/types/normalized_doc.go:13-55`.

### 3.1 Code Search Hit → NormalizedDoc

Source shape (from `*github.CodeResult` in go-github v85, verified against
https://docs.github.com/en/rest/search/search#search-code response):

```
type CodeResult struct {
    Name       *string         // file name
    Path       *string         // path within repo
    SHA        *string         // commit SHA
    HTMLURL    *string         // canonical web URL to the file
    Repository *Repository     // minimal Repository object
    Score      *float64        // Algolia-style relevance score (0..N+)
    FileSize   *int            // bytes
    Language   *string         // detected language (e.g., "Go", "Python")
    LineNumbers []string       // present only with text-match Accept header
}
```

| GitHub field | NormalizedDoc field | Transform |
|--------------|---------------------|-----------|
| `Repository.FullName + "@" + SHA + ":" + Path` | `ID` | Composite ID; preserves uniqueness across forks of the same path |
| (constant) | `SourceID` | `"github"` (matches `Name()`) |
| `HTMLURL` | `URL` | Use as-is (already canonical permalink) |
| `Path` (basename or full path) | `Title` | File path stripped to last 80 runes (display preference) |
| `""` | `Body` | Code search does not return file content in v0.1; the adapter does NOT make a follow-up `/repos/.../contents/...` call (would inflate latency + rate-limit cost). Future SPEC may add opt-in body fetch. |
| First 280 runes of `Path + " — " + Repository.FullName + " (" + Language + ")"` | `Snippet` | Synthesised summary string |
| (zero) | `PublishedAt` | Code search does not provide a per-file timestamp; left zero per `pkg/types/normalized_doc.go:25` ("zero when the source provides no date") |
| (parse time) | `RetrievedAt` | `time.Now().UTC()` |
| (none) | `Author` | `""` — code files don't have a single author at this level |
| `normalizeScore(int(Score))` rounded | `Score` | Tanh(Score/100). GitHub's relevance score is ~`[0, 100+]`; Tanh normaliser saturates around 1.0 for top hits |
| (constant) | `Lang` | `""` — Lang is BCP-47 (human language); programming language is stored in Metadata |
| (constant) | `DocType` | `types.DocTypeRepo` (see §2.4 — code lives in repos; future `DocTypeCode` deferred) |
| (nil) | `Citations` | `nil` |
| (constructed) | `Metadata` | REQUIRED keys: `repo_full_name` (string), `path` (string), `sha` (string), `language` (string, may be empty), `kind` ("code"). OPTIONAL: `file_size` (int), `score` (float64 raw). |
| (constant) | `Hash` | `""` (consumers compute via `CanonicalHash()`) |

### 3.2 Issue / PR Search Hit → NormalizedDoc

Source shape (from `*github.Issue` in go-github v85; PRs share this struct
because `*github.Issue.PullRequestLinks != nil` indicates a PR):

```
type Issue struct {
    ID        *int64
    Number    *int
    Title     *string
    Body      *string
    State     *string
    HTMLURL   *string
    User      *User
    CreatedAt *Timestamp
    UpdatedAt *Timestamp
    Comments  *int
    Labels    []*Label
    PullRequestLinks *PullRequestLinks
    Repository *Repository    // populated by search results
    Reactions  *Reactions     // rollup
}
```

| GitHub field | NormalizedDoc field | Transform |
|--------------|---------------------|-----------|
| `"github:issue:" + strconv.FormatInt(*ID, 10)` | `ID` | Stable global issue ID |
| (constant) | `SourceID` | `"github"` |
| `HTMLURL` | `URL` | Use as-is |
| `Title` | `Title` | Use as-is |
| `Body` (may be empty for issues with no body) | `Body` | Use as-is; markdown preserved |
| First 280 runes of `Body`; falls back to `Title` truncated similarly when Body empty | `Snippet` | Same truncation discipline as Reddit/HN |
| `CreatedAt.Time.UTC()` | `PublishedAt` | go-github wraps GitHub's RFC 3339 |
| (parse time) | `RetrievedAt` | `time.Now().UTC()` |
| `User.Login` (may be empty for ghost user) | `Author` | Use as-is |
| `normalizeScore(int(Comments) * 10)` (engagement proxy) | `Score` | Issues have no native score; we synthesise from `Comments * 10` so a 10-comment issue ≈ 0.76 (Tanh(1.0)). Open Question §7.6 documents revisit. |
| (constant) | `Lang` | `""` |
| (conditional) | `DocType` | `types.DocTypeIssue` regardless of issue-vs-PR (the PR distinction lives in Metadata) |
| (nil) | `Citations` | `nil` |
| (constructed) | `Metadata` | REQUIRED keys: `repo_full_name`, `number` (int), `state` (string: "open"/"closed"), `is_pull_request` (bool, true when `PullRequestLinks != nil`), `comments` (int), `kind` ("issue" or "pr"). OPTIONAL: `labels` ([]string), `updated_at` (RFC 3339 string), `reactions_total_count` (int). |
| (constant) | `Hash` | `""` |

### 3.3 Repository Search Hit → NormalizedDoc

Source shape (from `*github.Repository`):

```
type Repository struct {
    ID        *int64
    Name      *string
    FullName  *string
    HTMLURL   *string
    Description *string
    Language    *string
    StargazersCount *int
    WatchersCount   *int
    ForksCount      *int
    OpenIssuesCount *int
    Size            *int        // KB
    DefaultBranch   *string
    Topics          []string
    CreatedAt       *Timestamp
    UpdatedAt       *Timestamp
    PushedAt        *Timestamp
    Owner           *User
}
```

| GitHub field | NormalizedDoc field | Transform |
|--------------|---------------------|-----------|
| `"github:repo:" + strconv.FormatInt(*ID, 10)` | `ID` | Stable global repo ID |
| (constant) | `SourceID` | `"github"` |
| `HTMLURL` | `URL` | Use as-is |
| `FullName` | `Title` | e.g., `"facebook/react"` |
| `Description` (may be empty) | `Body` | Use as-is |
| First 280 runes of `Description`; falls back to `FullName` when empty | `Snippet` | Same truncation |
| `CreatedAt.Time.UTC()` | `PublishedAt` | Repo creation timestamp |
| (parse time) | `RetrievedAt` | `time.Now().UTC()` |
| `Owner.Login` | `Author` | Owner login (user or org) |
| `normalizeScore(int(StargazersCount))` | `Score` | Tanh of stars; a 100-star repo ≈ 0.88, a 10000-star repo ≈ 1.0 |
| (constant) | `Lang` | `""` |
| (constant) | `DocType` | `types.DocTypeRepo` |
| (nil) | `Citations` | `nil` |
| (constructed) | `Metadata` | REQUIRED keys: `full_name`, `language` (may be empty), `stars` (int), `forks` (int), `open_issues` (int), `kind` ("repo"). OPTIONAL: `topics` ([]string), `default_branch` (string), `pushed_at` (RFC 3339 string), `size_kb` (int). |
| (constant) | `Hash` | `""` |

### 3.4 Score Synthesis (Cross-Intent)

GitHub's three search modes have three different scoring shapes:

- **Code search**: GitHub provides `score *float64`, range `[0, ~100+]`,
  Algolia-style relevance.
- **Issue/PR search**: NO score; we synthesise `Comments * 10`.
- **Repository search**: NO score; we synthesise from `StargazersCount`.

All three pass through the same `normalizeScore(int)` Tanh formula
(divisor=100, center=0.5) inherited verbatim from SPEC-ADP-001 §2.3 and
SPEC-ADP-002 §2.3. No GitHub-specific calibration in v0.1; the constant
choice is documented in Open Question §7.6 for post-M3 revisit. SPEC-IDX-001
RRF (M3) re-ranks by rank not raw score across adapters, so the precise score
curve matters less than determinism + bounded `[0, 1]` codomain.

---

## 4. Authentication and Environment Variables

### 4.1 Choice — PAT via Env Var

**Decision**: PAT via `USEARCH_GITHUB_TOKEN` environment variable.

Rationale:

1. **Simplest plumbing**: env var + one `os.Getenv` call. Mirrors the
   authentication model the registry already validates at
   `internal/adapters/registry.go:123-129` (the `RegisterWithOptions`
   path checks `caps.AuthEnvVars` against the process environment unless
   `SkipAuthCheck=true`).
2. **5000 req/hr ceiling**: PAT-authenticated requests use the
   per-user 5000/hr primary rate limit
   (https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api).
   60/hr unauthenticated is too low for fanout (Reddit's already at 10/min;
   adding GitHub at 60/hr would dominate the slowest-adapter tail).
3. **No OAuth UX surface**: v0.1 is CLI-only and personal-use; OAuth
   browser flow is overkill.
4. **No GitHub App private-key management**: GitHub Apps require a private
   key (PEM) per installation; significantly more secret-management
   complexity than a single PAT. SPEC-AUTH-002 (M6) is the future home
   for per-team GitHub Apps.

**Env-var name choice**: `USEARCH_GITHUB_TOKEN` (uppercase, project-prefixed
to avoid clashing with `GITHUB_TOKEN` which GitHub Actions sets in CI). This
mirrors the project-prefix convention from SPEC-LLM-001 (which uses
`LITELLM_MASTER_KEY` / `OPENAI_BASE_URL`-style names — see
`internal/llm/client.go:31-65`).

### 4.2 Capabilities Declaration

```go
RequiresAuth: true,
AuthEnvVars: []string{"USEARCH_GITHUB_TOKEN"},
```

The registry's `Register` validates presence at startup
(`internal/adapters/registry.go:122-129`) unless `SkipAuthCheck` is set
(tests use this). Missing-env-var produces
`*RegistryError{Cause: ErrMissingAuth}` from the registry — the adapter is
not constructed. CLI startup behaviour is the registry's concern, not the
adapter's.

### 4.3 PAT Scopes Required

Minimum scopes for v0.1 search needs:

- `public_repo` — sufficient for code/issue/repo search across PUBLIC
  repos.
- `repo` — required ONLY if private-repo search is needed; out of v0.1
  scope (would expose PII and require team-scope tracking — SPEC-AUTH-002
  M6).

Documented in `Capabilities.Notes`: "PAT must have `public_repo` scope.
Private-repo search is out of v0.1 scope."

### 4.4 Failure Modes

| Failure | HTTP | go-github Error Type | NormalizedDoc Result |
|---------|------|----------------------|----------------------|
| Token missing | (caught at startup) | `*RegistryError{ErrMissingAuth}` | Adapter not constructed |
| Token invalid | 401 | `*github.ErrorResponse` | `*SourceError{CategoryPermanent, HTTPStatus:401}` |
| Token lacks required scope | 403 | `*github.ErrorResponse` | `*SourceError{CategoryPermanent, HTTPStatus:403}` |
| Token revoked | 401 | `*github.ErrorResponse` | `*SourceError{CategoryPermanent, HTTPStatus:401}` |
| Token expired (fine-grained PAT) | 401 | `*github.ErrorResponse` | `*SourceError{CategoryPermanent, HTTPStatus:401}` |

---

## 5. Rate-Limit Semantics (Detail)

Verified via WebFetch of
https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api:

### 5.1 Primary Rate Limits

| Auth Mode | Limit |
|-----------|-------|
| Unauthenticated | 60 req/hr per IP |
| PAT (user) | 5000 req/hr |
| GitHub App (Cloud) | 5000–15000 req/hr |
| Git LFS | separate bucket |
| GitHub Actions GITHUB_TOKEN | 1000/hr/repo or 15000/hr Enterprise |

### 5.2 Search-Specific Secondary Limit

The search resource has its own 30/min ceiling (per
https://docs.github.com/en/rest/search/search):

> "Up to 30 requests per minute for authenticated requests" (search endpoints).

For code search specifically:

> "Limits you to 9 requests per minute" (per
> https://docs.github.com/en/rest/search/search#search-code).

This is BELOW the standard 30/min — code search is the most expensive
endpoint and has a tighter ceiling. ADP-004's `Capabilities.RateLimitPerMin`
declares the conservative figure: **30** (for issue/repo search) — code
search routes through the same limit but the IR-001 routing is per-adapter,
not per-intent. v0.1 accepts this conservative pessimism. Open Question §7.7
documents whether to expose the per-intent finer ceiling at the SPEC layer.

### 5.3 Concurrency Limit

> "No more than 100 concurrent requests are allowed across REST and GraphQL
> APIs"
> (https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api).

The fanout layer (SPEC-FAN-001 §6.5) caps at `MaxParallel=8` by default.
Adding GitHub adds at most 1 concurrent request (the adapter doesn't
internally fan out). 100-concurrent-req ceiling is not a binding constraint.

### 5.4 HTTP Response Headers (parsed by go-github automatically)

| Header | Semantics | go-github Field |
|--------|-----------|-----------------|
| `X-RateLimit-Limit` | Hourly quota | `Response.Rate.Limit` |
| `X-RateLimit-Remaining` | Calls left in window | `Response.Rate.Remaining` |
| `X-RateLimit-Reset` | UTC epoch reset | `Response.Rate.Reset` (time.Time) |
| `X-RateLimit-Used` | Used in window | `Response.Rate.Used` |
| `X-RateLimit-Resource` | Bucket affected (e.g., `search`) | `Response.Rate.Resource` |
| `Retry-After` | Backoff seconds for 429 | `*github.AbuseRateLimitError.RetryAfter` |

The adapter does NOT pre-flight check `X-RateLimit-Remaining` — it issues
the call and lets the server enforce. Fanout (SPEC-FAN-001) consumes
`*SourceError.RetryAfter` after a `CategoryRateLimited` to inform any future
retry policy (today none — see SPEC-FAN-001 OQ §6.4).

### 5.5 Error Mapping

| go-github Error | HTTP | Category | RetryAfter | Notes |
|-----------------|------|----------|------------|-------|
| `*github.RateLimitError` | 403 + `X-RateLimit-Remaining: 0` | CategoryRateLimited | `Rate.Reset.Sub(time.Now())`, capped 90s | Primary limit |
| `*github.AbuseRateLimitError` | 403 / 429 | CategoryRateLimited | `RetryAfter` (struct field), capped 90s | Secondary / abuse |
| `*github.ErrorResponse` (4xx) | 401/403/404/422 | CategoryPermanent | 0 | 422 = malformed query syntax |
| `*github.ErrorResponse` (5xx) | 5xx | CategoryUnavailable | 0 | Server failure |
| `context.DeadlineExceeded` | n/a | (passes through) | 0 | wrappedAdapter classifies via `OutcomeFromError` |
| Network error | n/a | CategoryUnavailable | 0 | DNS, dial, TLS — `HTTPStatus=0` |

**Note on 422 + abuse-403**: GitHub returns 422 for "Validation Failed"
(unknown qualifier in `q`, malformed search syntax). v0.1 maps 422 →
`CategoryPermanent` because retrying without changing `q` won't help. GitHub
also returns 403 for both forbidden access AND abuse detection — go-github
distinguishes them via the typed-error path (`*github.AbuseRateLimitError`
specifically). The adapter inspects with `errors.As` to disambiguate.

### 5.6 Retry-After Cap Choice

SPEC-ADP-001 caps Retry-After at 60 seconds. SPEC-ADP-004 caps at **90
seconds** because GitHub's secondary-rate-limit recovery can document up to
several minutes per https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api,
and the conservative 60s in ADP-001 was Reddit-specific. 90s gives the fanout
layer (SPEC-FAN-001 D6) enough headroom to honour GitHub's guidance without
inflating the user-facing tail latency unreasonably. Open Question §7.8
documents revisit triggers.

---

## 6. Pagination Semantics

GitHub search uses page-based pagination:

- Request: `?per_page=N&page=K` where K is 1-indexed.
- Response: `Link` header with `rel="next"`, `rel="last"`, etc.
  go-github parses this internally and exposes `*Response.NextPage`,
  `LastPage`, `FirstPage`, `PrevPage`. When `NextPage == 0`, there are no
  more pages.

ADP-004's cursor surface (matching the project convention from ADP-001
REQ-ADP-006 and ADP-002 REQ-ADP2-005):

- `Query.Cursor` is parsed via `strconv.Atoi`. Empty → page 1 (NOT page 0;
  GitHub uses 1-indexed pages, unlike Algolia's 0-indexed). Non-numeric or
  negative → reject with `*SourceError{CategoryPermanent, Cause:
  ErrInvalidCursor}` immediately, no HTTP request.
- After successful response, when `*Response.NextPage > 0`, the LAST returned
  doc gets `Metadata["next_cursor"] = strconv.Itoa(NextPage)`.
- When the `LastPage > 0 && NextPage == 0`, no `next_cursor` key is set.

Maximum results constraint: `per_page` clamped to 100 (GitHub ceiling per
https://docs.github.com/en/rest/search/search). Total search results
capped by GitHub at 1000 (i.e., max 10 pages of 100). The adapter does NOT
enforce the 1000-result ceiling; if a caller paginates past it, GitHub
returns an empty `items` array — handled normally by the parser.

---

## 7. Open Questions (Carried Forward to spec.md §11)

These are intentionally deferred; they do not block SPEC approval. Each has a
recommended default and a one-line resolution owner.

### 7.1 `DocTypeCode` enum addition

Question: should `pkg/types/capabilities.go:14-23` add a new `DocTypeCode`
value to distinguish code-search file fragments from repository hits?

**Recommended default**: NO in v0.1. Map code hits to `DocTypeRepo` and
store `kind="code"` in `Metadata`. Adding an enum value is a
breaking-ish change to the SDK boundary (per
`.moai/project/structure.md:160`), and synthesis quality won't suffer
because the IR-001 router uses `Capabilities.DocTypes` for adapter
SELECTION not SCORING.

**Resolution owner**: SPEC-IDX-001 author (M3) may request the new enum
when RRF tuning shows code hits and repo hits should weight differently.

### 7.2 Code-content fetch follow-up

Question: should the adapter make a second `/repos/.../contents/{path}`
call per code hit to populate `Body` with the file content?

**Recommended default**: NO in v0.1. Doubles the rate-limit cost and inflates
latency. Synthesis (SPEC-SYN-001) operates on Title + Snippet for v0.1; if
M3 measurements show synthesis quality on code-intent queries is too low,
add an opt-in `Query.Filters[Key="fetch_code_body"]` filter.

**Resolution owner**: SPEC-SYN-002 / SPEC-SYN-003 author.

### 7.3 GitHub App authentication

Question: when does ADP-004 add GitHub App auth (per-team installation key,
15000/hr Cloud limit)?

**Recommended default**: NEVER in v0.1. Defer to SPEC-AUTH-002 (M6 per
`.moai/project/roadmap.md:82`). Adding GitHub App now would couple the
adapter to per-team plumbing that doesn't exist yet.

**Resolution owner**: SPEC-AUTH-002 author.

### 7.4 Path A revisit (MCP wrapping)

Question: if SPEC-MCP-001 (M7 per `.moai/project/roadmap.md:91`) lands and
exposes an OUTBOUND MCP server, should ADP-004 be re-implemented as an
INBOUND consumer of `github/github-mcp-server`?

**Recommended default**: NO unless GitHub deprecates `/search/code` in
favour of an MCP-only endpoint. The operational complexity of running two
binaries (usearch + github-mcp-server) outweighs the abstraction benefit for
a single source.

**Resolution owner**: SPEC-MCP-001 author + SPEC-ADP-004a (TBD post-M7).

### 7.5 GraphQL variant

Question: when does the search-endpoint 30/min ceiling become binding
enough to justify a GraphQL variant (with point-cost accounting and
`pageInfo` cursors)?

**Recommended default**: NEVER in v0.1 or v1. GraphQL adds maintenance burden
without measured benefit at our scale.

**Resolution owner**: SPEC-IDX-001 / SPEC-IDX-002 author may revisit if
multi-source RRF measurements show GitHub search throughput is the bottleneck.

### 7.6 Score synthesis from comments / stars

Question: the v0.1 score for issues uses `comments * 10` (so 10 comments
→ Tanh(1.0) ≈ 0.76); for repos uses `stars` directly. Are these the right
inflection points?

**Recommended default**: keep both in v0.1. SPEC-IDX-001 RRF re-ranks by
rank not score across adapters, so per-adapter score curves matter less than
determinism + bounded codomain. Revisit after M3 RRF tuning measurements.

**Resolution owner**: SPEC-IDX-001 author.

### 7.7 Per-intent rate-limit declaration

Question: `Capabilities.RateLimitPerMin` is a single int. GitHub's code
search is 9/min while issue/repo search is 30/min. Should ADP-004 declare a
conservative single value (=9) or the most-common value (=30)?

**Recommended default**: 30 (issue/repo cadence). Document the code-search
9/min in `Capabilities.Notes`. The fanout layer (SPEC-FAN-001) doesn't
currently honour per-intent rate ceilings — adding that would require
amending `pkg/types.Capabilities`, out of v0.1 scope.

**Resolution owner**: SPEC-FAN-001 author may upgrade to a per-intent
ceiling field in a future SPEC.

### 7.8 Retry-After cap (90s vs 60s)

Question: ADP-001 caps at 60s; ADP-004 wants 90s for GitHub's
secondary-rate-limit recovery semantics. Should the cap become a
per-adapter Options field?

**Recommended default**: hardcode 90s in v0.1. Make it Options-tunable in a
future iteration if measured pain warrants. Keeps the SPEC narrow.

**Resolution owner**: run-phase implementer; revisit if SPEC-EVAL-002
reliability dashboard shows GitHub adapter is timing out under sustained load.

---

## 8. Risk Register (GitHub-Specific)

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Token missing at startup | Low | High | Registry's `RegisterWithOptions` already validates `AuthEnvVars` per `internal/adapters/registry.go:123-129`; missing env → `*RegistryError{ErrMissingAuth}` returned at startup. CLI surfaces this clearly (consumer of the registry). |
| Token logged inadvertently | Low | High | Token never appears in `*SourceError.Cause` — go-github does not echo headers; the registry's slog emitter at `internal/adapters/registry.go:240-251` logs only `name/outcome/elapsed/result_count/error.Error()`. The error string from `*github.ErrorResponse` does not include the Authorization header. |
| 422 query syntax errors confused with auth failures | Medium | Low | Distinct mapping: 422 → `CategoryPermanent` with `HTTPStatus:422`; 401/403 → `CategoryPermanent` with `HTTPStatus:401/403`. Tests assert the discrimination. |
| Abuse detection (403) vs forbidden (403) ambiguity | Medium | Medium | go-github already disambiguates via typed errors; adapter uses `errors.As(err, &abuseErr)` to detect abuse, otherwise treats 403 as `CategoryPermanent`. |
| Rate-limit coordination with fanout | Medium | Medium | Adapter returns one categorised error per request; SPEC-FAN-001 D6 owns retry policy (currently zero-retry). When retry lands, fanout reads `*SourceError.RetryAfter`. |
| Code-search 9/min ceiling under burst | Medium | Medium | `Capabilities.RateLimitPerMin=30` is the cross-intent figure; code-search may hit 9/min ceiling first. Open Question §7.7 + Capabilities.Notes documents. Fanout's 8-parallel cap means 8 concurrent code-search calls hit the ceiling within 1.5s. |
| go-github major version pinned to v85; minor bumps may change types | Low | Low | Major-version path is import-encoded (`/v85/`); accidental major bumps impossible. Minor-version bumps follow GitHub's API versioning (2022-11-28); the typed-search-result struct has been stable for 3+ years. SPEC-DEP-001 owns version bumps. |
| New adapter dependency on `github.com/google/go-querystring` (transitive) | Low | Low | Pure Go, BSD-3 license; widely used (transitively pulled into thousands of Go projects). No supply-chain anomaly. |
| Network IP geolocation triggers GitHub abuse detection | Low | Medium | Fixed by setting a custom User-Agent (mirrors REQ-ADP-009 from ADP-001). go-github sets `User-Agent: go-github` by default; we override to `usearch/<version>`. |
| Empty `items` arrays on past-1000-results pagination | Low | Low | GitHub caps total at 1000 results; pages 11+ return empty `items`. Adapter's parser treats empty as zero-doc response, not an error. |
| Issue/PR distinction (PR is an issue with `pull_request` field) | Low | Low | REQUIRED Metadata key `is_pull_request` (bool) and `kind` ("issue" or "pr") makes it consumer-visible. SPEC-SYN-002 may filter on this. |
| Composite ID format (`fullname@sha:path`) for code hits | Low | Low | NormalizedDoc.ID is opaque to consumers; only the registry uses it (and registry doesn't parse). |
| `Score` field absent on issue/PR/repo (synthesised from comments/stars) | Medium | Low | Documented in §3.4 + Open Question §7.6. SPEC-IDX-001 RRF uses rank not raw score across adapters. |
| Time zone of `created_at` / `updated_at` | Low | Low | go-github parses RFC 3339 to `time.Time`; `.UTC()` enforces UTC for consistency. |
| Nil-pointer dereference on `*github.Issue.User`, `*github.Repository.Owner` etc. | Medium | Low | go-github returns pointers for nullable fields; the parser uses `safeStr` / `safeInt` helpers that nil-guard each field. Tests cover deleted-user / ghost-user cases. |

---

## 9. Sources and Citations

### External URLs (WebFetch verified)

- https://github.com/github/github-mcp-server — Official GitHub MCP server;
  binary + Docker + remote HTTP. No Go client library.
- https://github.com/google/go-github — Mature Go client library v85.0.0
  (April 2026); structured client with typed search results, rate-limit
  helpers, page-based pagination.
- https://docs.github.com/en/rest — REST API v3 documentation portal.
- https://docs.github.com/en/rest/search/search — Search endpoints, query
  parameters, response shapes, pagination, rate-limit specifics.
- https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api
  — Primary 5000/hr, secondary 30/min for search, 100-concurrent ceiling,
  X-RateLimit-* headers, Retry-After semantics.
- https://docs.github.com/en/graphql — GraphQL v4 endpoint (alternative
  rejected for v0.1).

### Internal Files (file:line cited)

- `pkg/types/normalized_doc.go:13-55` — NormalizedDoc 15-field struct,
  field semantics, Validate, CanonicalHash.
- `pkg/types/adapter.go:28-45` — Adapter 4-method interface contract.
- `pkg/types/capabilities.go:14-23` — DocType enum (DocTypeRepo,
  DocTypeIssue, DocTypeOther available).
- `pkg/types/capabilities.go:38-62` — Capabilities struct with
  `RequiresAuth`, `AuthEnvVars`, `RateLimitPerMin`, `DocTypes`,
  `SupportedLangs`, `Notes`.
- `pkg/types/query.go:18-44` — Query struct with `Filters []Filter`,
  `Cursor`, `MaxResults`, `Deadline`.
- `pkg/types/errors.go:14-218` — SourceError, Category, four sentinels,
  CategorizeError, OutcomeFromError, ValidationError.
- `internal/adapters/registry.go:75-167` — Registry lifecycle; `Register`
  validates `AuthEnvVars` against `os.LookupEnv` at line 123-129.
- `internal/adapters/registry.go:172-263` — wrappedAdapter sole-emitter
  pattern; the GitHub adapter inherits this for all metrics/logs/spans.
- `internal/adapters/noop/noop.go:1-46` — minimal reference adapter shape.
- `internal/adapters/reddit/reddit.go:1-136` — Adapter struct + Options
  pattern (mirrored by ADP-004).
- `internal/adapters/reddit/search.go:1-167` — Search hot path pattern.
- `internal/adapters/reddit/parse.go:1-203` — JSON → NormalizedDoc
  transform pattern.
- `internal/adapters/reddit/client.go:1-125` — HTTP client + redirect
  allowlist + categorizeStatus pattern.
- `internal/adapters/reddit/score.go:1-41` — Tanh score formula
  (duplicated verbatim by ADP-002 + ADP-004).
- `internal/adapters/reddit/errors.go:1-64` — `parseRetryAfter` helper +
  ErrInvalidQuery sentinel pattern.
- `internal/adapters/hn/hn.go:1-139` — Second-adapter validation that the
  reference shape is portable.
- `internal/adapters/hn/parse.go` — Algolia hits → NormalizedDoc transform
  example for a different envelope.
- `internal/llm/client.go:31-65` — HTTP client construction with timeout +
  reqid Transport wrapping (pattern adopted for ADP-004).
- `.moai/specs/SPEC-ADP-001/spec.md` — Reference SPEC structure (1216
  lines); authoritative shape for ADP-004's spec.md.
- `.moai/specs/SPEC-ADP-002/spec.md` — Second-adapter SPEC; ADP-004's
  closest analog because both depart from the no-auth baseline (ADP-002
  doesn't, ADP-004 does — but the file layout and TDD discipline are
  identical).
- `.moai/specs/SPEC-CORE-001/spec.md` — Adapter / Capabilities / Query /
  NormalizedDoc / SourceError contract origin.
- `.moai/specs/SPEC-OBS-001/spec.md` — observability bundle; AdapterCalls /
  AdapterCallDuration cardinality allowlist.
- `.moai/specs/SPEC-IR-001/spec.md` — RoutingDecision shape; REQ-IR-008
  selectAdapterSet algorithm consumes `Capabilities.DocTypes` and
  `SupportedLangs`.
- `.moai/specs/SPEC-FAN-001/spec.md` — M3 fanout layer; consumes
  `*SourceError.RetryAfter` (currently zero-retry per D6).
- `.moai/project/roadmap.md:49` — M3 row: "SPEC-ADP-004 | GitHub adapter |
  wrap official `github/github-mcp-server`". Note: roadmap suggests Path
  A; this research recommends Path B and documents the deviation in §2.
- `.moai/project/roadmap.md:113` — tech.md line about GitHub: "GitHub
  REST + Search API, PAT per team, 5000/hr with auth, wrap via
  `github/github-mcp-server`".
- `.moai/project/structure.md:18-29` — `internal/adapters/github/`
  reservation.
- `.moai/project/structure.md:160` — `pkg/types` SDK boundary clause —
  reason `DocTypeCode` enum addition is deferred (Open Question §7.1).
- `.moai/project/tech.md:113` — Per-source adapter strategy table row for
  GitHub.
- `.moai/config/sections/quality.yaml` — `development_mode: tdd`,
  `test_coverage_target: 85`.
- `.moai/config/sections/harness.yaml` — auto-routing to standard level.
- `.moai/config/sections/language.yaml` — `documentation: en`,
  `code_comments: en`.
- `go.mod:5-9` — direct deps; `github.com/google/go-github/v85` MUST be
  added at SPEC-ADP-004 run phase.
- `go.mod:30` — `go.uber.org/goleak v1.3.0` indirect (used by NFR-ADP4-003).

---

End of Research Document.

**Summary for SPEC Author**: This research establishes the GitHub REST API
surface across three search modes (code, issues/PRs, repositories) with
distinct response shapes mapped to a single 15-field NormalizedDoc.
Recommended approach is **Path B (direct `google/go-github` REST client)**
over Path A (wrapping `github/github-mcp-server`) because the synchronous
adapter contract aligns naturally with REST and avoids subprocess /
HTTP-protocol overhead. PAT auth via `USEARCH_GITHUB_TOKEN` env is
documented; rate-limit semantics (5000/hr primary, 30/min search secondary,
9/min code-search secondary, 100-concurrent ceiling) are mapped to the four
`*SourceError` Categories. Pagination via 1-indexed page integer surfaced as
`Metadata["next_cursor"]`. 8 Open Questions deferred (DocTypeCode enum,
code-content fetch, GitHub App auth, Path A revisit, GraphQL variant, score
synthesis, per-intent rate-limit, Retry-After cap). The spec.md should span
~800-1000 lines covering 10-12 EARS REQ-* items plus 4 NFRs, mirroring the
SPEC-ADP-001/002 structure verbatim with auth + multi-intent routing as the
two GitHub-specific deltas.
