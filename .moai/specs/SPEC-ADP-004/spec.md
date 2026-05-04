---
id: SPEC-ADP-004
title: GitHub Adapter
version: 0.1.0
milestone: M3 — Fanout, adapters, index
status: draft
priority: P0
owner: expert-backend
methodology: tdd
coverage_target: 85
created: 2026-05-04
updated: 2026-05-04
author: limbowl
issue_number: null
depends_on: [SPEC-CORE-001, SPEC-OBS-001, SPEC-IR-001]
blocks: []
---

# SPEC-ADP-004: GitHub Adapter

## HISTORY

- 2026-05-04 (iteration 2 — plan-auditor cycle 1, limbowl via manager-spec):
  Self-audit pass surfaced 3 HIGH and 5 MEDIUM concerns; all addressed
  inline. HIGH fixes: (H1) §6.3 + REQ-ADP4-006 acceptance softened the
  "go-github does not echo Authorization headers" claim from an asserted
  fact to a TESTED PROPERTY — the adapter SHALL NOT deliberately log the
  token, and `TestSearchTokenNotInErrorMessage` constructs the test stub
  to ECHO the `Authorization` header back in the response body so the
  assertion has a concrete leakable surface to discriminate against; if
  go-github ever begins echoing headers in error formatting the test
  catches it. (H2) §2.1 a + new Note paragraph in §2.1 clarify the
  interaction between the registry's `AuthEnvVars` validation
  (`internal/adapters/registry.go:123-129` checks env presence) and the
  adapter's constructor token check (`New` validates Options.Token);
  the CLI orchestration layer reads the env and passes via Options.Token
  — the registry's env-presence check and the constructor's
  Options.Token check are TWO INDEPENDENT GATES, not a redundant pair,
  so missing env produces ErrMissingAuth at registration time and an
  empty Options.Token (without SkipAuthCheck) produces ErrMissingToken
  at constructor time. (H9) §6.5 type sketch corrected: replaced
  ambiguous `ghClient.SetBaseURL(opts.BaseURL)` with the documented
  go-github v85 idiom of constructing a fresh `*github.Client` via
  `gogithub.NewClient(httpClient)` and assigning to `client.BaseURL`
  (parsed `*url.URL`) directly when `Options.BaseURL` is non-empty —
  this matches how go-github exposes test-harness URL override.
  MEDIUM fixes: (M2) HISTORY count corrected from "~40 tests" to "~64
  tests" matching the §8 TDD Plan table; (M3) HISTORY now flags the
  Path A → Path B deviation explicitly as a "Resolved discrepancy"
  block (mirrors the SPEC-ADP-001 HISTORY discrepancy block pattern at
  spec.md:73-77); (M4) §2.1 (m) cleaned up — removed placeholder
  parenthetical about HN's stripHTML; (M5) §2.1 a clarified the
  ErrMissingToken constructor return path to be a `*types.SourceError`
  (consistent with our error taxonomy) rather than a bare error. (M7)
  REQ-ADP4-006 acceptance text for `TestSearchTokenNotInErrorMessage`
  now specifies the token-leak test setup: stub MUST echo the
  Authorization header value in its 401 response body so the assertion
  has a real attack-surface to test against. Total: still 10 REQs (no
  count change). 4 NFRs unchanged. 64 tests in §8 TDD Plan. Status
  remains `draft` until cycle-2 audit confirms zero HIGH residual.

- 2026-05-04 (initial draft v0.1, limbowl via manager-spec):
  Fourth-adapter SPEC drafted following the SPEC-ADP-001 reference shape
  verbatim with two GitHub-specific deltas (authentication + multi-intent
  routing). Scope and contracts derived from
  `.moai/specs/SPEC-ADP-004/research.md` (every external claim URL-cited;
  every internal claim file:line-cited). Built on SPEC-CORE-001
  (`pkg/types.Adapter` interface, `pkg/types.Capabilities` descriptor with
  `RequiresAuth`/`AuthEnvVars` fields, `pkg/types.Query`,
  `pkg/types.NormalizedDoc` 15-field struct, `*types.SourceError` taxonomy,
  registry wrappedAdapter sole-emitter pattern at
  `internal/adapters/registry.go:172-263` plus the `AuthEnvVars`-validation
  path at `internal/adapters/registry.go:123-129`), SPEC-OBS-001 (pre-
  registered AdapterCalls / AdapterCallDuration with `adapter` and
  `outcome` in cardinality allowlist; ZERO new metric families needed),
  SPEC-ADP-001 (file layout + error mapping + MX tag plan + TDD harness +
  Tanh score normaliser, all reused as-is or as pattern), and SPEC-ADP-002
  (HN adapter validation that the reference shape is portable). Soft dep
  on SPEC-IR-001 for `Capabilities` consumer contract.

  Key structural inheritance from ADP-001 / ADP-002:

  - File layout (`github.go` / `search.go` / `client.go` / `parse.go` /
    `score.go` / `errors.go` / `bench_test.go` + testdata/) → identical
    layout in `internal/adapters/github/`.
  - HTTP client construction (10s timeout, `reqid.NewTransport` wrapping)
    — managed by go-github via injected `*http.Client` (mirrors the
    `internal/llm/client.go:51-54` pattern); redirect allowlist is
    enforced inside `*http.Client.CheckRedirect` against
    `{api.github.com, github.com, raw.githubusercontent.com,
    codeload.github.com}`.
  - `categorizeStatus` rosetta — adopted with adapter name swap to
    `"github"`, plus 422 (Validation Failed) mapped to
    `CategoryPermanent`.
  - `parseRetryAfter` helper — adopted but with cap raised from 60s
    (Reddit) to 90s (GitHub guidance for secondary-rate-limit recovery
    can document several minutes; 90s is a conservative ceiling that
    keeps user-facing tail latency reasonable). Open Question §11.7
    documents revisit.
  - `normalizeScore` Tanh formula (divisor=100, center=0.5) — adopted
    verbatim. Score synthesis varies by intent (code search has GitHub-
    provided score; issues/PRs synthesise from comment-count; repos
    synthesise from stargazer-count). Documented in §2.3 + Open
    Question §11.6.
  - Sole-emitter discipline (zero metrics/logs/spans emitted by the
    adapter; registry wrappedAdapter emits all observability).
  - `var _ types.Adapter = (*Adapter)(nil)` compile-time interface
    assertion.

  GitHub-specific deltas from ADP-001 / ADP-002 documented inline in §6
  and §7:

  - **First authenticated adapter** in the project. `Capabilities`
    declares `RequiresAuth=true` and
    `AuthEnvVars=["USEARCH_GITHUB_TOKEN"]`. The registry's
    `RegisterWithOptions` (`internal/adapters/registry.go:123-129`)
    validates env-var presence at startup unless `SkipAuthCheck=true`
    (tests use this). PAT must have `public_repo` scope; private-repo
    search is OUT OF SCOPE.
  - **Multi-intent routing**. The adapter handles three GitHub search
    modes in one Search method — the routing key is
    `Query.Filters[Key="kind"]` with `Value` ∈ `{"code", "issues",
    "repos"}`, defaulting to `"repos"` when omitted (broadest scope,
    cheapest rate-limit cost). Each intent dispatches to a different
    go-github call: `client.Search.Code`, `client.Search.Issues`, or
    `client.Search.Repositories`.
  - **Library dependency**: `github.com/google/go-github/v85` (and its
    transitive `github.com/google/go-querystring`) added to `go.mod`.
    The major-version path-encoding (`/v85/`) prevents accidental
    upgrades. Pinning policy mirrors the powernap pattern in
    `.claude/rules/moai/core/lsp-client.md`.
  - **DocTypes** = `[DocTypeRepo, DocTypeIssue]`. Code search hits map
    to `DocTypeRepo` in v0.1 (Open Question §11.1 documents the
    deferred `DocTypeCode` enum addition that would require amending
    `pkg/types/capabilities.go` SDK boundary).
  - **Three NormalizedDoc field-mapping tables** (one per intent) —
    code, issue/PR, repo — each documented in §6.3 with REQUIRED vs
    OPTIONAL Metadata key tiers.
  - **Path A (wrap `github/github-mcp-server`) explicitly REJECTED for
    v0.1** in research §2.1 + §2.2; rationale: synchronous
    `pkg/types.Adapter` contract mismatches MCP's async streaming model;
    no Go client library provided by github-mcp-server; operational
    footprint addition (one more docker container) outweighs the
    abstraction benefit for a single source. Open Question §11.4 keeps
    Path A revisit door open.

  Resolved discrepancy: `.moai/project/roadmap.md:49` says "wrap
  official `github/github-mcp-server`" (Path A). This SPEC chose Path B
  (direct `google/go-github` REST client) and documents the deviation
  in §2.2 + Open Question §11.4 + research.md §2.1-§2.2. Rationale:
  Path A's MCP subprocess / HTTP-protocol model does not align with the
  synchronous `pkg/types.Adapter` contract; no Go client library is
  provided by github-mcp-server (verified via WebFetch); operational
  footprint of running two binaries outweighs abstraction benefit for
  one source. Path A revisit deferred to a future SPEC-ADP-004a if
  GitHub deprecates `/search/code` in favour of MCP-only access.

  10 EARS REQs (8 × P0 + 2 × P1) covering all five EARS patterns
  (Ubiquitous, Event-Driven, State-Driven via REQ-ADP4-010 concurrency
  contract, Optional via REQ-ADP4-007 numeric/qualifier filters, Unwanted
  via REQ-ADP4-008 empty-query / invalid-cursor / invalid-intent
  rejection), 4 NFRs (parse-path performance, E2E p95 stub, no goroutine
  leak on cancellation, resource cleanup), 8 Open Questions carried
  forward from research.md §7. ONE new Go module dependency
  (`github.com/google/go-github/v85`) plus Go stdlib (`net/http`,
  `encoding/json`, `time`, `context`, `errors`, `strings`, `strconv`,
  `unicode`, `unicode/utf8`, `math`) plus existing `pkg/types`,
  `internal/obs/reqid` (nil-safe consumer). Inserted into M3 as the FIRST
  authenticated adapter; the wedge for the M3 7-way ADP-* parallel
  implementation. Harness level: standard (single domain, ≤10 source
  files, no payment/PII keywords; auth env-var addition is non-secret-
  managing — the registry handles validation). Sprint Contract optional.
  Ready for plan-auditor review and annotation cycle.

---

## 1. Purpose

The M3 milestone exit criterion (`.moai/project/roadmap.md:150`) requires
that `usearch query` returns fused results across ≥5 adapters. SPEC-ADP-001
implemented Reddit; SPEC-ADP-002 implemented Hacker News; SPEC-ADP-004
implements **GitHub** as the third real adapter and the FIRST authenticated
adapter, surfacing code search + issue/PR search + repository metadata in a
single uniform contract.

GitHub is the natural choice for the third M3 adapter because:

1. **Rich multi-intent surface**. Code search maps to the "code" intent;
   issues/PRs map to the "social" intent (issues are also social signals —
   developer pain points, community discussion); repository metadata
   straddles both. Implementing all three in one adapter validates that the
   `pkg/types.NormalizedDoc` 15-field shape is expressive enough for
   non-trivial heterogeneous sources.
2. **First authenticated adapter**. The `Capabilities.RequiresAuth=true` +
   `Capabilities.AuthEnvVars=["USEARCH_GITHUB_TOKEN"]` declaration is the
   first time the registry's auth-validation path
   (`internal/adapters/registry.go:123-129`) executes against a real
   adapter. ADP-004 validates that the contract works.
3. **Different response shape than Reddit / HN**. Three response shapes
   (`*github.CodeResult`, `*github.Issue`, `*github.Repository`) over a
   single transport exercise the JSON-mapping discipline established in
   ADP-001 §6.3 and ADP-002 §6.3 against genuinely different envelopes.
4. **Mature client library available**. `github.com/google/go-github` is a
   13k-star, production-grade Go client with typed search-result structs,
   built-in `*RateLimitError` / `*AbuseRateLimitError` types that map
   directly to our `*SourceError` taxonomy, and page-based pagination via
   parsed `Link` headers. Adopting it eliminates ~400 LoC of hand-rolled
   HTTP / JSON-decode / error-mapping code that ADP-001 and ADP-002 carry.
5. **All four error categories reachable** in normal operation. 401/403
   for invalid/scoped tokens (CategoryPermanent), 422 for malformed query
   syntax (CategoryPermanent), 5xx during incidents (CategoryUnavailable),
   primary 5000/hr + secondary 30/min (and 9/min for code search) limits
   produce 403/429 (CategoryRateLimited). Timeouts via
   context.DeadlineExceeded (CategoryTransient).
6. **Establishes the `RequiresAuth=true` reference shape** for SPEC-ADP-006
   (Bluesky + X), SPEC-ADP-008 (Naver), and any future authenticated
   adapter.

Like ADP-001/002, the GitHub adapter does NOT do fanout (SPEC-FAN-001 owns
goroutine dispatch), does NOT do retry (SPEC-FAN-001 D6 owns orchestration;
v0.1 zero-retry), does NOT do caching (SPEC-CACHE-001 M3 owns 5-phase
fallback), does NOT do ranking fusion (SPEC-IDX-001 M3 owns RRF), does NOT
emit any metric/log/span itself (the registry wrappedAdapter does, sole-
emitter discipline). It DOES one job: turn a `types.Query` into one of three
GitHub search calls (code/issues/repos via go-github), parse the typed
response, and return `[]types.NormalizedDoc` or `*types.SourceError`.

Completion contributes to M3 exit criterion progress. ADP-004 is the third
adapter consuming the SPEC-CORE-001 contract; combined with Reddit and HN, a
ranked-fanout query across three sources becomes feasible. The remaining six
M3 adapters (ADP-003, 005, 006, 007, 008, 009) inherit ADP-004's authenticated-
adapter shape where they need authentication.

---

## 2. Scope

### 2.1 In-Scope

| Ref | Item |
|-----|------|
| a | `internal/adapters/github/github.go`: `Adapter` struct (go-github client + base URL + user-agent + healthcheck target), `New(opts Options) (*Adapter, error)` constructor (validates token presence when `SkipAuthCheck=false`; returns `*types.SourceError{Adapter:"github", Category:CategoryPermanent, Cause:ErrMissingToken}` when `Options.Token` is empty AND `SkipAuthCheck=false` AND no `HTTPClient` override is provided), `Name() string` returning `"github"`, `Capabilities() types.Capabilities` returning a deterministic descriptor (`RequiresAuth=true`, `AuthEnvVars=["USEARCH_GITHUB_TOKEN"]`, `DocTypes=[DocTypeRepo, DocTypeIssue]`, `SupportedLangs=nil`, `SupportsSince=true` (GitHub supports `created:>=` qualifier in `q`), `RateLimitPerMin=30`, `DefaultMaxResults=25`, `DisplayName="GitHub"`, `Notes` documenting the PAT scope requirement + multi-intent routing + per-intent rate-limit caveat (code search 9/min vs issues/repos 30/min) + 90s Retry-After cap + Path A rejection rationale), and `Healthcheck(ctx) error` (TCP-connect probe to `api.github.com:443` with caller-supplied ctx; target injectable via Options). Compile-time interface assertion `var _ types.Adapter = (*Adapter)(nil)` at the bottom. **Note on auth-validation gate interaction**: `Capabilities.AuthEnvVars=["USEARCH_GITHUB_TOKEN"]` causes the registry's `RegisterWithOptions` (`internal/adapters/registry.go:123-129`) to verify env-var PRESENCE in the process environment via `os.LookupEnv` (not the constructor's responsibility); the constructor independently verifies that the CLI-orchestrated layer passed `Options.Token` (a non-empty string) — these are TWO INDEPENDENT GATES, not redundant. The CLI is responsible for reading the env var and passing the value to `Options.Token`; the registry catches the env-missing case (returns `ErrMissingAuth`) and the constructor catches the value-not-passed case (returns `ErrMissingToken`). Out-of-scope for this SPEC: how the CLI wires the env-var read into `Options.Token` — that's SPEC-CLI-001's concern. |
| b | `internal/adapters/github/search.go`: `(*Adapter).Search(ctx, q types.Query) ([]types.NormalizedDoc, error)` — the hot path. Validates the query (REQ-ADP4-008), parses any `q.Cursor` as a positive integer page (1-indexed; GitHub uses 1-indexed pages), parses `q.Filters[Key="kind"]` to determine intent (`code`/`issues`/`repos`; default `repos`), builds the appropriate go-github call options (`per_page`, `page`, plus optional qualifier suffixes injected into `q.Text` via §6.4 `appendQualifiers`), invokes the corresponding `client.Search.Code` / `client.Search.Issues` / `client.Search.Repositories`, delegates response parsing to `parse.go`, returns `[]NormalizedDoc` or `*SourceError`. Honours `ctx` cancellation throughout. |
| c | `internal/adapters/github/client.go`: HTTP client construction. The default `*http.Client` is constructed via `newDefaultHTTPClient()` with `Timeout=10s`, `Transport=reqid.NewTransport(http.DefaultTransport)` (mirrors `internal/llm/client.go:51-54`), and `CheckRedirect=redirectAllowlist` enforcing the 4-host allowlist `{api.github.com, github.com, raw.githubusercontent.com, codeload.github.com}` capped at 3 hops. The go-github client is constructed via `github.NewClient(httpClient).WithAuthToken(token)` (the v85 idiom). The package also defines `categorizeError(err error) *types.SourceError` — a typed-error rosetta that uses `errors.As` against `*github.RateLimitError`, `*github.AbuseRateLimitError`, and `*github.ErrorResponse` to produce a `*SourceError` with the correct Category + RetryAfter + HTTPStatus (see §6.3 mapping table). |
| d | `internal/adapters/github/parse.go`: three intent-specific parse functions: `parseCodeResults(resp *github.CodeSearchResult, retrievedAt time.Time) ([]NormalizedDoc, string, error)`, `parseIssueResults(resp *github.IssuesSearchResult, retrievedAt time.Time) ([]NormalizedDoc, string, error)`, `parseRepoResults(resp *github.RepositoriesSearchResult, retrievedAt time.Time) ([]NormalizedDoc, string, error)`. Each transforms the typed go-github result into `[]NormalizedDoc` per the field-mapping tables in §6.3.1, §6.3.2, §6.3.3. The next-page cursor (when `*github.Response.NextPage > 0`) is surfaced via `Metadata["next_cursor"]` on the LAST returned doc, encoded as `strconv.Itoa(NextPage)`. Empty results return `(nil, "", nil)`. Nil-pointer-safe access via `safeStr` / `safeInt` / `safeBool` / `safeTime` helpers (defined in `parse.go`). |
| e | `internal/adapters/github/score.go`: `normalizeScore(rawScore int) float64` — Tanh formula identical to ADP-001 §2.3 (`clamp(0.5 + 0.5 * tanh(rawScore / 100.0), 0.0, 1.0)`). Package-level constants `tanhDivisor = 100.0` and `scoreCenter = 0.5` annotated with `@MX:NOTE`. Score synthesis is intent-specific: code search uses `int(*CodeResult.Score)` (GitHub-provided ~`[0, 100+]` relevance); issue/PR search uses `int(*Issue.Comments) * 10` (engagement proxy); repository search uses `int(*Repository.StargazersCount)`. The synthesis logic lives in `parse.go`; `score.go` exposes only the pure normaliser. |
| f | `internal/adapters/github/errors.go`: package-private sentinels: `ErrInvalidQuery = errors.New("github: query text empty or whitespace-only")` (REQ-ADP4-008); `ErrInvalidCursor = errors.New("github: cursor must be positive integer page (1-indexed)")` (REQ-ADP4-008); `ErrInvalidIntent = errors.New("github: filter kind must be one of code, issues, repos")` (REQ-ADP4-008); `ErrMissingToken = errors.New("github: USEARCH_GITHUB_TOKEN not set")` (constructor failure path, returned by `New` when validation runs). The `parseRetryAfter` helper from ADP-001 is duplicated here with cap raised from 60s to 90s (per §6.5 + Open Question §11.7). |
| g | `internal/adapters/github/github_test.go`: tests for Adapter interface conformance (`var _ types.Adapter` assertion via compile-time check), `Name()` returns `"github"`, `Capabilities()` returns deterministic value (called twice; equal), `Healthcheck()` succeeds against a stub `httptest.Server`, `New()` validates options (token presence + `SkipAuthCheck` interaction). |
| h | `internal/adapters/github/search_test.go`: the largest test file. Drives `(*Adapter).Search` against `httptest.Server` with golden fixtures, with `Options.BaseURL` redirected to the stub. Covers per-intent happy paths (25 code hits, 25 issue hits, 25 repo hits), default-intent fallback (no `kind` filter → repos), empty result, 429 with Retry-After, abuse-403, 401, 403, 404, 422 (malformed query), 5xx, redirect to allowed and disallowed hosts, ctx cancellation mid-request, invalid cursor rejection, invalid intent rejection, empty/whitespace query rejection, pagination cursor round-trip. |
| i | `internal/adapters/github/client_test.go`: HTTP client unit tests — `categorizeError` truth table over the typed-error space (`*github.RateLimitError` → CategoryRateLimited; `*github.AbuseRateLimitError` → CategoryRateLimited with RetryAfter; `*github.ErrorResponse` 4xx → CategoryPermanent; 5xx → CategoryUnavailable; `context.DeadlineExceeded` → passes through; nil → nil), `parseRetryAfter` table over 6 input shapes including the new 90s cap, redirect allowlist enforcement (allowlist + cross-domain rejection + chain-over-3 rejection), User-Agent header presence, `Accept: application/json` header presence (set by go-github's default client on top of our wrapped Transport). |
| j | `internal/adapters/github/parse_test.go`: field-mapping unit tests, three table-driven sub-tables (one per intent): code search (5 fixtures spanning Go / Python / TypeScript / no-language / deleted-fork-with-nil-language); issue/PR search (5 fixtures: open issue / closed issue / open PR / closed PR / deleted-author); repo search (5 fixtures: public popular / private (filtered out by token scope, but go-github doesn't fail — repo just doesn't appear; fixture exercises this) / archived / fork / no-description). Each asserts every NormalizedDoc field per the §6.3 mapping tables. Snippet truncation to 280 runes. Score synthesis (4 example values per intent). Pagination cursor round-trip. Hash field is empty (REQ-ADP4-005). |
| k | `internal/adapters/github/score_test.go`: `normalizeScore` table-driven test over 7 score values (`-1000, -10, 0, 10, 100, 1000, 10000`) with expected `[0.0, 1.0]` outputs computed from the formula, asserted within `±0.001`. Determinism check. |
| l | `internal/adapters/github/bench_test.go`: `BenchmarkParseGitHubResponse25Results` (NFR-ADP4-001 — p50 ≤ 5 ms parse time on amd64 for a 25-result repo-search fixture; allocation ≤ 25 allocs per result parsed = ≤ 625 allocs total — slightly higher than ADP-001/002's 500-alloc ceiling because `*Repository` has more nullable pointer fields requiring nil-guard helpers). `TestMain` calls `goleak.VerifyTestMain(m)` (NFR-ADP4-003). |
| m | `internal/adapters/github/testdata/`: 9 golden JSON fixtures — `search_code_response.json` (25-hit code search, ~8KB), `search_issues_response.json` (25-hit issue/PR mix, ~12KB), `search_repos_response.json` (25-hit repo search, ~10KB), `search_response_empty.json` (zero hits, ~200B), `search_response_pagination.json` (page 1 with NextPage=2, ~10KB), `search_response_deleted_author.json` (issue with `User: nil` exercising safeStr nil-guard), `search_response_429_rate_limited.json` (rate-limit response from go-github stub), `search_response_403_abuse.json` (abuse rate limit), `search_response_422_validation.json` (malformed query syntax), `search_response_malformed.json` (truncated JSON for parse-error path). HN's `stripHTML` helper is NOT inherited — GitHub returns markdown bodies for issues/PRs and plain-text descriptions for repos; no HTML decoding required. |

### 2.2 Out-of-Scope

This SPEC explicitly excludes the following items. Each has a known
destination SPEC; this list prevents scope creep into ADP-004 (the
authenticated-adapter reference).

- **Per-source customisations specific to other sources** (arXiv OAI-PMH,
  YouTube yt-dlp metadata, Bluesky AT Protocol, X / Twitter, SearXNG
  bridge, Naver suite, Daum scraper, KoreaNewsCrawler, RSS, Polymarket) →
  SPEC-ADP-003, SPEC-ADP-005..009.
- **Retry orchestration** (exponential backoff, jitter, per-adapter
  retry budget keyed on `RateLimitPerMin`) → SPEC-FAN-001 D6 (currently
  zero-retry) + future SPEC-FAN-001-RETRY. Adapter returns one
  categorised error per request; fanout owns retry decisions.
- **Response caching** → SPEC-CACHE-001 (M3). Each Search call is
  independent and idempotent at the adapter layer.
- **Result ranking, deduplication, RRF fusion across adapters** →
  SPEC-IDX-001 (M3). Fanout / IDX-001 own cross-adapter ranking.
- **Code-content fetch** (a follow-up `/repos/{owner}/{repo}/contents/{path}`
  call to populate `Body` with the file content for code-search hits) →
  Open Question §11.2; deferred. v0.1 leaves `Body=""` for code hits and
  uses `Path + " — " + RepoFullName + " (" + Language + ")"` as Snippet.
- **GitHub App authentication** (per-team installation with private-key
  PEM, 15000/hr Cloud limit) → SPEC-AUTH-002 (M6 per
  `.moai/project/roadmap.md:82`). v0.1 PAT-only.
- **OAuth user-flow authentication** → SPEC-AUTH-001 (M6).
- **Private-repository search** (token with `repo` scope instead of
  `public_repo`) → SPEC-AUTH-002 (M6); private-data exposure requires
  team-scope tracking.
- **Sort customisation** (GitHub `sort=stars|forks|updated|comments|...`
  per endpoint) → out of v0.1; default relevance ("best match") only.
  Future P2 enhancement post-M3.
- **Code search `text_matches` highlighting** (the `Accept:
  application/vnd.github.text-match+json` header surfaces line-number
  highlights in `*CodeResult.LineNumbers`) → out of v0.1; not requested.
- **GraphQL v4 variant** (point-cost accounting, opaque `pageInfo`
  cursors) → Open Question §11.5; not in v0.1 or v1.
- **MCP wrapping (Path A)** (running `github/github-mcp-server` as a
  subprocess or remote service) → research §2.1 + §2.2 + Open Question
  §11.4; explicitly REJECTED for v0.1.
- **Live network integration tests in CI** → out of v0.1. `httptest.Server`
  + golden fixtures only. Optional env-gated live test (`-tags=integration`
  + `GITHUB_LIVE=1`) deferred.
- **OpenAPI / proto schema for the adapter response** — the
  `[]types.NormalizedDoc` return type IS the schema; no separate IDL.
- **Korean tokenisation or language inference** for GitHub repos / issues
  → SPEC-IDX-003 (M3). The adapter sets `NormalizedDoc.Lang = ""`.
- **`pkg/llm` integration** — the GitHub adapter does NOT call any LLM.
  Classification is the Intent Router's job (SPEC-IR-001).
- **`DocTypeCode` enum addition** (new value in `pkg/types/capabilities.go`
  to distinguish code-search file fragments from repo metadata) → Open
  Question §11.1; deferred. Code hits map to `DocTypeRepo` in v0.1.
- **Per-intent rate-limit ceiling declaration** (a single
  `Capabilities.RateLimitPerMin` cannot represent code=9/min vs
  issues/repos=30/min) → Open Question §11.7. v0.1 declares the
  conservative single value and documents the per-intent caveat in
  `Capabilities.Notes`.
- **Per-adapter custom Prometheus metrics** (e.g.,
  `github_pagination_pages_total`) → would require amending SPEC-OBS-001
  allowlist. Out of v0.1.
- **HTML body stripping** (HN's `stripHTML` helper) — GitHub returns
  markdown bodies, not HTML; the adapter passes Body through unchanged.
  Synthesis (SPEC-SYN-002) consumes markdown directly.

### 2.3 Score Synthesis (Per Intent)

[HARD] The adapter synthesises `NormalizedDoc.Score` differently per intent
because GitHub's three search modes have three different scoring shapes. All
three pass through the same `normalizeScore(int) float64` Tanh formula
inherited verbatim from SPEC-ADP-001 §2.3:

```
Score = clamp(0.5 + 0.5 * tanh(rawScore / 100.0), 0.0, 1.0)
```

Per-intent input:

| Intent | Raw input | Rationale |
|--------|-----------|-----------|
| `code` | `int(*CodeResult.Score)` | GitHub-provided Algolia-style relevance, ~`[0, 100+]`. A score=100 hit ≈ Tanh(1.0) ≈ 0.88 (top-of-results match) |
| `issues` | `int(*Issue.Comments) * 10` | Engagement proxy. 10-comment issue ≈ Tanh(1.0) ≈ 0.76 (active discussion) |
| `repos` | `int(*Repository.StargazersCount)` | Star count. 100-star repo ≈ Tanh(1.0) ≈ 0.88 (notable); 10000-star repo ≈ 1.0 (saturated) |

Properties (inherited from ADP-001 §2.3):

- Codomain `[0.0, 1.0]` enforced by `clamp`.
- Determinism: pure function, no state, no time, no I/O.
- Saturation: very large positive scores asymptote to 1.0.

Tie-break behaviour: equal raw inputs → equal `NormalizedDoc.Score`. Order
within an intent is preserved from go-github's response (GitHub's "best
match" ordering). SPEC-IDX-001 RRF fuses across adapters by rank not raw
score, so same-Score ties within GitHub do not destabilise cross-adapter
ranking.

The choice to keep divisor=100, center=0.5 across all three intents (rather
than per-intent calibration) is intentional for v0.1 — Open Question §11.6
documents revisit triggers post-M3 RRF measurements.

### 2.4 Intent Routing (Multi-Mode Adapter)

The GitHub adapter is the FIRST adapter to produce results across two intent
classes (per the roadmap M3 entry):

- **code** intent → `client.Search.Code` (GitHub's `/search/code`
  endpoint).
- **social** intent → `client.Search.Issues` (GitHub's `/search/issues`,
  which covers BOTH issues and PRs because PRs are issues with a
  `pull_request` field).

Repository search (`client.Search.Repositories`) serves both intents
(repos are "code" but the search request often originates from a social-
intent query like "react state management library").

[HARD] **Routing mechanism**: `pkg/types.Query.Filters` carries one filter
`Key="kind"` with `Value` ∈ `{"code", "issues", "repos"}`. When
omitted, default is `"repos"` (broadest scope, lowest auth requirement,
doesn't hit the `/search/code` 9-req/min ceiling). Any other Value (e.g.,
`"users"`, `"commits"`) returns
`*SourceError{Adapter:"github", Category:CategoryPermanent,
Cause:ErrInvalidIntent}` immediately, no HTTP request issued.

REQ-ADP4-008 makes this contract testable.

The `Capabilities` descriptor declares
`DocTypes=[types.DocTypeRepo, types.DocTypeIssue]` — IR-001 REQ-IR-008
intersects `categoryEligibleDocTypes` with `Capabilities.SupportedLangs`
and the adapter's `DocTypes` slice; GitHub is eligible for `code`,
`social`, and `mixed` categories; ineligible for `korean` (SupportedLangs
nil → IR-001 treats nil as language-agnostic, so this gate is permissive).

---

## 3. EARS Requirements

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| REQ-ADP4-001 | Ubiquitous | The package `internal/adapters/github` SHALL expose an `Adapter` struct that implements `pkg/types.Adapter` exactly: `Name() string` returning `"github"`, `Search(ctx context.Context, q types.Query) ([]types.NormalizedDoc, error)`, `Healthcheck(ctx context.Context) error`, `Capabilities() types.Capabilities`. The package SHALL include a compile-time interface assertion `var _ types.Adapter = (*Adapter)(nil)`. `Capabilities()` SHALL be deterministic (two consecutive calls return equal values) with `SourceID="github"`, `DisplayName="GitHub"`, `DocTypes=[DocTypeRepo, DocTypeIssue]`, `SupportedLangs=nil`, `SupportsSince=true`, `RequiresAuth=true`, `AuthEnvVars=["USEARCH_GITHUB_TOKEN"]`, `RateLimitPerMin=30`, `DefaultMaxResults=25`, and `Notes` containing the substrings `"GitHub REST"`, `"PAT"`, `"public_repo scope"`, `"code search 9/min"`, `"issues/repos 30/min"`, and `"Retry-After cap 90s"`. | P0 | `TestAdapterName`, `TestAdapterImplementsInterface` (compile-time), `TestCapabilitiesDeterministic`, `TestCapabilitiesShape` (asserts all 9 documented field values + Notes substring matches), `TestCapabilitiesDeclaresRequiresAuth` (asserts `RequiresAuth=true` AND `len(AuthEnvVars)==1` AND `AuthEnvVars[0]=="USEARCH_GITHUB_TOKEN"`), `TestHealthcheckSucceeds` (stub `httptest.Server` injected via Options). All in `internal/adapters/github/github_test.go`. |
| REQ-ADP4-002 | Event-Driven | WHEN `(*Adapter).Search(ctx, q)` is invoked with non-empty `q.Text` AND a valid `kind` filter (one of `code`/`issues`/`repos`, default `repos` when omitted), the adapter SHALL build the appropriate go-github call (`client.Search.Code` / `client.Search.Issues` / `client.Search.Repositories`) with `ListOptions{PerPage: clamp(q.MaxResults, 1, 100), Page: parseCursor(q.Cursor)}` (defaulting `Page=1` when `q.Cursor==""`, `PerPage=25` when `q.MaxResults==0`), and SHALL append qualifier suffixes from `q.Filters` (per §6.4) to `q.Text` before invocation. The adapter SHALL parse the typed response per REQ-ADP4-005 mapping and return `(docs, nil)` on success with `len(docs) ≤ 100`. | P0 | `TestSearchCodeIntentHappyPath25Hits`, `TestSearchIssuesIntentHappyPath25Hits`, `TestSearchReposIntentHappyPath25Hits` (each httptest.Server returns the corresponding fixture; assert 25 NormalizedDocs returned, each with all required fields populated and `Validate()` returning nil); `TestSearchDefaultIntentIsRepos` (Filters=nil → URL path is `/search/repositories`); `TestSearchClampsPerPageTo100` (q.MaxResults=500 → URL has `per_page=100`); `TestSearchDefaultsPerPageTo25` (q.MaxResults=0 → URL has `per_page=25`); `TestSearchSetsPageWhenCursorPresent` (q.Cursor="3" → URL has `page=3`); `TestSearchSetsPage1WhenCursorEmpty` (q.Cursor="" → URL has `page=1` (1-indexed)). All in `search_test.go`. |
| REQ-ADP4-003 | Event-Driven | WHEN go-github returns a `*github.RateLimitError` (primary 5000/hr exceeded) OR `*github.AbuseRateLimitError` (secondary / abuse), the adapter SHALL return `(nil, &types.SourceError{Adapter:"github", Category: types.CategoryRateLimited, HTTPStatus: <403 or 429 from inner response>, RetryAfter: <duration>, Cause: <inner error>})`. RetryAfter computation: for `*RateLimitError`, RetryAfter = `Rate.Reset.Sub(time.Now())` capped at 90 seconds; for `*AbuseRateLimitError`, RetryAfter = struct field `RetryAfter` capped at 90 seconds; if either is ≤0, default to 5 seconds. The adapter SHALL NOT retry internally. | P0 | `TestSearchPrimaryRateLimitMapsToCategory` (stub returns 403 + `X-RateLimit-Remaining: 0` + `X-RateLimit-Reset: <future>`; assert `errors.Is(err, types.ErrRateLimited)` AND `srcErr.RetryAfter > 0`); `TestSearchAbuseRateLimitMapsToCategory` (stub returns 403 + abuse-detection body that go-github recognises; assert `errors.Is(err, types.ErrRateLimited)` AND `srcErr.RetryAfter` reflects the `Retry-After: <N>` header capped at 90s); `TestSearchRateLimitRetryAfterCapped90s` (`Retry-After: 999` → RetryAfter=90s); `TestSearchRateLimitNoInternalRetry` (assert exactly 1 outbound request observed). All in `search_test.go` + `client_test.go`. |
| REQ-ADP4-004 | Event-Driven | WHEN go-github returns a `*github.ErrorResponse` with HTTP 401, 403 (non-abuse), 404, or 422, the adapter SHALL return `(nil, &types.SourceError{Adapter:"github", Category: types.CategoryPermanent, HTTPStatus: <code>, Cause: errors.New("github: permanent failure: <code>")})`. WHEN HTTP 5xx is received OR a connection error occurs (DNS failure, dial timeout, read timeout, TLS handshake failure), the adapter SHALL return `(nil, &types.SourceError{Adapter:"github", Category: types.CategoryUnavailable, HTTPStatus: <code or 0>, Cause: <inner error>})`. Network-layer errors set `HTTPStatus=0`. The adapter SHALL NOT retry. | P0 | `TestSearchHTTP401`, `TestSearchHTTP403NonAbuse`, `TestSearchHTTP404`, `TestSearchHTTP422ValidationFailed` (each asserts `errors.Is(err, types.ErrPermanent)` AND matching HTTPStatus); `TestSearchHTTP500`, `TestSearchHTTP503` (each asserts `errors.Is(err, types.ErrSourceUnavailable)` AND matching HTTPStatus); `TestSearchConnectionRefused` (httptest.Server closed before request; `errors.Is(err, types.ErrSourceUnavailable)`; HTTPStatus=0); `TestSearchUnavailablePreservesUnderlyingError` (assert `errors.Unwrap(srcErr).Error()` contains the inner cause). All in `search_test.go` + `client_test.go`. |
| REQ-ADP4-005 | Ubiquitous | The adapter SHALL transform each go-github typed search result (`*github.CodeResult`, `*github.Issue`, `*github.Repository`) into one `types.NormalizedDoc` using the per-intent field mappings in §6.3.1 / §6.3.2 / §6.3.3, MUST set `RetrievedAt = time.Now().UTC()` at the moment of parsing, MUST leave `Hash = ""` (consumers compute via `CanonicalHash()`), MUST populate `Metadata` with at minimum the per-intent REQUIRED keys (code: `repo_full_name`, `path`, `sha`, `language`, `kind="code"`; issues: `repo_full_name`, `number`, `state`, `is_pull_request`, `comments`, `kind` ∈ {"issue","pr"}; repos: `full_name`, `language`, `stars`, `forks`, `open_issues`, `kind="repo"`), MUST set the per-intent `DocType` (`DocTypeRepo` for code AND repos; `DocTypeIssue` for issues/PRs), MUST set `Lang = ""` (programming language is in Metadata, not Lang). The next-page cursor (when `*github.Response.NextPage > 0`) SHALL be returned as the second return value of the per-intent parse function so `Search` can surface it via `Metadata["next_cursor"]` on the LAST returned NormalizedDoc — encoded as `strconv.Itoa(NextPage)`. Nil-pointer fields in the typed responses (e.g., deleted-author `*Issue.User == nil`, no-language `*CodeResult.Language == nil`) SHALL be handled via `safeStr` / `safeInt` / `safeBool` / `safeTime` nil-guard helpers; resulting NormalizedDoc fields are zero-valued (empty string or zero) for nil source fields. | P0 | `TestParseCodeResultsFieldMapping` (table over 5 fixtures including no-language and deleted-fork); `TestParseIssueResultsFieldMapping` (table over 5 fixtures including open-issue, closed-PR, deleted-author); `TestParseRepoResultsFieldMapping` (table over 5 fixtures including archived, fork, no-description); `TestParseDeletedUserNilSafe` (issue with `User: nil` → returned doc has `Author == ""`); `TestParseNoLanguageNilSafe` (code result with `Language: nil` → Metadata["language"] == ""); `TestParsePaginationCursor` (response with `NextPage=2, LastPage=10` → last doc Metadata["next_cursor"] == "2"); `TestParseNoCursorOnLastPage` (response with `NextPage=0` → no doc has `next_cursor` key); `TestParseHashEmpty` (every returned doc has `Hash == ""`); `TestParseMetadataKeysPerIntent` (asserts the per-intent REQUIRED key set is present). All in `parse_test.go`. |
| REQ-ADP4-006 | Ubiquitous | The adapter SHALL set the `User-Agent` HTTP header on every outbound request to a non-default value of the form `usearch/<version> (+https://github.com/elymas/universal-search)` where `<version>` is supplied via `Options.UserAgentVersion` (default `"v0.1"`). The adapter SHALL set the `Authorization: Bearer <token>` header (provided by go-github's `WithAuthToken` idiom) on every authenticated request. While GitHub does not block default Go User-Agents as aggressively as Reddit, setting a custom UA preserves the project-wide convention from ADP-001 REQ-ADP-009 / ADP-002 REQ-ADP2-006 and identifies traffic for operational debugging at GitHub's side. The adapter SHALL NOT log the Authorization header value at any level. | P0 | `TestSearchSetsCustomUserAgent` (inspect captured `r.Header.Get("User-Agent")`; assert it starts with `"usearch/"` and contains `"(+https://github.com/elymas/universal-search)"`); `TestSearchSetsAuthorizationHeader` (assert `Authorization` header starts with `"Bearer "`; assert the captured token equals the test fixture token); `TestSearchUserAgentVersionConfigurable` (Options.UserAgentVersion="v0.2-rc1" → header contains `"usearch/v0.2-rc1"`); `TestSearchTokenNotInErrorMessage` (force a 401; assert returned `*SourceError.Cause.Error()` does NOT contain the token substring); `TestSearchTokenNotInSlogOutput` (capture slog JSON; assert no key/value in the captured records contains the token substring). All in `client_test.go`. |
| REQ-ADP4-007 | Optional | WHERE `Query.Filters` contains entries with one of the recognised qualifier keys (`since` → `created:>=<RFC3339>`; `language` → `language:<value>` (code/repos only); `repo` → `repo:<owner>/<name>` (issues only); `org` → `org:<name>`; `user` → `user:<name>`; `topic` → `topic:<value>` (repos only); `state` → `state:<open|closed>` (issues only); `is_pr` (any non-empty value) → `is:pr` (issues only)), the adapter SHALL append the corresponding qualifier suffix to `q.Text` separated by a single space, escaping the value via `url.QueryEscape` ONLY for values containing whitespace or shell-meta characters (else passing literally to preserve readability in GitHub's query syntax). Filter keys other than these 8 SHALL be silently ignored (no error returned). Malformed filter values (empty Value, or `since` value not parseable as RFC 3339) SHALL be silently dropped (no error, no qualifier added). | P1 | `TestSearchSinceFilterAddsCreatedQualifier` (Filters=[{since, "2026-01-01T00:00:00Z"}] → URL has `q=...+created%3A%3E%3D2026-01-01T00%3A00%3A00Z`); `TestSearchLanguageFilterAddsLanguageQualifier` (Filters=[{language, "go"}] → URL has `q=...+language%3Ago`); `TestSearchRepoFilterAddsRepoQualifier` (Filters=[{repo, "facebook/react"}] → URL has `q=...+repo%3Afacebook%2Freact`); `TestSearchMultipleFiltersJoinedBySpace` (two filters → both appended to q with space separator); `TestSearchUnknownFilterIgnored` (Filters=[{nsfw, "true"}] → URL `q` unchanged); `TestSearchMalformedSinceDropped` (Filters=[{since, "not-a-date"}] → URL `q` unchanged); `TestSearchEmptyFilterValueDropped` (Filters=[{language, ""}] → URL `q` unchanged); `TestSearchFiltersOnlyAppendForApplicableIntent` (Filters=[{is_pr, "true"}] with `kind=code` → ignored; with `kind=issues` → applied). All in `search_test.go`. |
| REQ-ADP4-008 | Unwanted | IF `Query.Text` is empty OR contains only Unicode whitespace runes (per `unicode.IsSpace` over every rune), THEN the adapter SHALL return `(nil, &types.SourceError{Adapter:"github", Category: types.CategoryPermanent, Cause: ErrInvalidQuery})` immediately and SHALL NOT issue any HTTP request. IF `Query.Cursor` is non-empty AND does NOT parse as a positive integer ≥1 via `strconv.Atoi` (zero, negative, non-numeric all rejected), THEN the adapter SHALL return `(nil, &types.SourceError{Adapter:"github", Category: types.CategoryPermanent, Cause: ErrInvalidCursor})` immediately and SHALL NOT issue any HTTP request. IF `Query.Filters` contains an entry with `Key=="kind"` AND `Value` is not one of `{"code", "issues", "repos"}`, THEN the adapter SHALL return `(nil, &types.SourceError{Adapter:"github", Category: types.CategoryPermanent, Cause: ErrInvalidIntent})` immediately and SHALL NOT issue any HTTP request. | P0 | `TestSearchEmptyQueryRejectedNoHTTP` (table over `["", "   ", "\t\n  \r"]` for `q.Text`; for each asserts `errors.Is(err, types.ErrPermanent)` AND `errors.Is(err, ErrInvalidQuery)` AND assert httptest.Server received zero requests); `TestSearchInvalidCursorRejectedNoHTTP` (table over `["abc", "0", "-1", "1.5", "1e3"]` for `q.Cursor`; for each asserts `errors.Is(err, types.ErrPermanent)` AND `errors.Is(err, ErrInvalidCursor)` AND zero requests); `TestSearchInvalidIntentRejectedNoHTTP` (Filters=[{kind, "users"}]; assert `errors.Is(err, types.ErrPermanent)` AND `errors.Is(err, ErrInvalidIntent)` AND zero requests). All in `search_test.go`. |
| REQ-ADP4-009 | Optional | WHERE the response is HTTP 301/302/303/307/308, the adapter's `*http.Client.CheckRedirect` SHALL follow up to 3 redirects WITHIN the allowlist `{api.github.com, github.com, raw.githubusercontent.com, codeload.github.com}`. Cross-domain redirects (any other host) SHALL be rejected by returning an error from `CheckRedirect`; the adapter wraps this as `*SourceError{Adapter:"github", Category: CategoryPermanent, Cause: errors.New("github: cross-domain redirect rejected: <target host>")}` to prevent SSRF. Redirect chains exceeding 3 hops SHALL be rejected with a "too many redirects" message. The allowlist is defensive and consistent with the pattern established in SPEC-ADP-001 REQ-ADP-010. | P1 | `TestSearchFollowsAllowlistRedirect` (httptest.Server returns 302 to a second httptest.Server with Host header rewritten to `api.github.com`; assert 200-path NormalizedDocs returned); `TestSearchRejectsCrossDomainRedirect` (httptest.Server returns 301 to `attacker.com`; assert `errors.Is(err, types.ErrPermanent)` AND error message contains `"cross-domain redirect"`); `TestSearchRejectsRedirectChainOver3` (httptest.Server bouncing within allowlist 4 times; assert error after 3 hops). All in `client_test.go`. |
| REQ-ADP4-010 | State-Driven | WHILE the same `*Adapter` instance is registered in the adapter registry and is being invoked concurrently from N goroutines (N ≥ 1), each `Search(ctx, q)` call SHALL execute independently with no shared mutable state across calls (the `*Adapter` struct fields are all unexported and immutable post-construction; the underlying `*github.Client` and `*http.Client` are goroutine-safe per the go-github documentation and Go stdlib respectively); the cumulative effect SHALL be N independent HTTP round-trips with no race-detector alarms. This requirement crystallises the concurrency contract that the registry (`internal/adapters/registry.go:172-263` wrappedAdapter) and the future fanout layer (SPEC-FAN-001) rely on. | P0 | `TestSearchConcurrentSafe` (50 goroutines each issuing one Search against the same httptest.Server; assert (a) no race-detector alarm under `-race`, (b) total response count = 50 observed at the stub, (c) all 50 returned slices are `[]types.NormalizedDoc` with `Validate()` returning nil for every doc). In `search_test.go`. |

---

## 4. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| NFR-ADP4-001 | Performance (parse path) | The parse path `parseRepoResults(resp *github.RepositoriesSearchResult, retrievedAt time.Time) ([]NormalizedDoc, string, error)` (chosen as the representative because Repository has the most nullable pointer fields and is the default intent) SHALL execute with mean wall-clock duration per op ≤ 5 ms over `go test -bench=BenchmarkParseGitHubResponse25Results -benchtime=10x -count=5 ./internal/adapters/github/...` on amd64; the median of the 5 runs is the assertion value (passes when ≤ 5 ms). The fixture is `search_repos_response.json` (25-result repo search, ~10KB). Allocation count ≤ 25 per result parsed (i.e. ≤ 625 allocs total for 25 results) per the same benchmark's `allocs/op` field. The slightly higher per-result allocation budget vs ADP-001/002 (≤20/doc) is justified by `*github.Repository`'s richer nil-safe field handling: 12+ nullable pointer fields each requiring a `safeStr`/`safeInt`/`safeBool`/`safeTime` helper call (vs ~6 pointer fields in `*redditPostData`). Measured via `BenchmarkParseGitHubResponse25Results` in `internal/adapters/github/bench_test.go`, run weekly in CI per the cadence established in SPEC-OBS-001 NFR-OBS-001. Benchmarks do not count toward coverage. |
| NFR-ADP4-002 | End-to-end Latency | The end-to-end `Search` round-trip against the `httptest.Server` stub (no real network) SHALL complete with p95 ≤ 200 ms over 100 invocations, measured by `TestSearchE2ELatencyStubP95` in `search_test.go` (sort durations ascending, assert `durations[94] ≤ 200ms`). The harder live-GitHub p95 (≤ 3s, slightly tighter than HN's ≤ 2s because GitHub's API has a global edge cache with sub-100ms first-byte) is documented as the operational target but NOT enforced in CI (no live network). |
| NFR-ADP4-003 | No goroutine leak on cancellation | The adapter SHALL NOT leak any goroutine when the caller's context is cancelled mid-`Search`. Verified by `TestSearchNoGoroutineLeakOnCancel` in `search_test.go`, which uses `go.uber.org/goleak.VerifyNone(t)` after a `Search` call whose ctx is cancelled mid-flight via a 50ms-delayed cancel; assert zero residual goroutines after the call returns. Additionally, `internal/adapters/github/bench_test.go::TestMain` SHALL invoke `goleak.VerifyTestMain(m)` to catch package-level leaks. |
| NFR-ADP4-004 | Resource cleanup | The adapter SHALL ensure every constructed HTTP response Body is closed via `defer resp.Body.Close()` (when the adapter holds the response directly) OR via go-github's internal `io.Copy(ioutil.Discard, resp.Body)` + `resp.Body.Close()` (which go-github guarantees per its source). The adapter SHALL ensure every per-call `context.CancelFunc` (if any are constructed by the adapter, e.g., within the redirect allowlist policy) is invoked via `defer cancel()`. Verified by `TestSearchNoLeakedFileDescriptors` in `search_test.go` — counts the test process's open file descriptors before+after 100 Search calls; asserts the delta is ≤ 5 (allowing minor httptest churn). |

---

## 5. Acceptance Criteria

### REQ-ADP4-001 — Adapter Interface Conformance

- File `internal/adapters/github/github.go` declares `Adapter` struct with
  the documented fields (`ghClient *github.Client`, `httpClient
  *http.Client`, `baseURL string`, `userAgent string`, `healthcheckTarget
  string`).
- The compile-time assertion `var _ types.Adapter = (*Adapter)(nil)`
  appears at the bottom of `github.go`. If the interface ever drifts,
  this assertion fails to compile.
- `(*Adapter).Name()` returns the literal string `"github"`.
- `(*Adapter).Capabilities()` returns a `types.Capabilities` with:
  - `SourceID = "github"`
  - `DisplayName = "GitHub"`
  - `DocTypes = []types.DocType{types.DocTypeRepo, types.DocTypeIssue}`
  - `SupportedLangs = nil` (language-agnostic)
  - `SupportsSince = true` (GitHub `created:>=` qualifier supported)
  - `RequiresAuth = true`
  - `AuthEnvVars = []string{"USEARCH_GITHUB_TOKEN"}` (length 1)
  - `RateLimitPerMin = 30`
  - `DefaultMaxResults = 25`
  - `Notes` contains the substrings `"GitHub REST"`, `"PAT"`,
    `"public_repo scope"`, `"code search 9/min"`,
    `"issues/repos 30/min"`, and `"Retry-After cap 90s"`.
- `(*Adapter).Healthcheck(ctx)` succeeds against an httptest.Server bound
  to `127.0.0.1:0`. Tests construct the Adapter with
  `Options{HealthcheckTarget: <httptest.Server.Listener.Addr().String()>}`;
  the production default is `"api.github.com:443"`.
- `TestAdapterName`, `TestAdapterImplementsInterface`,
  `TestCapabilitiesDeterministic`, `TestCapabilitiesShape`,
  `TestCapabilitiesDeclaresRequiresAuth`, `TestHealthcheckSucceeds` all
  pass.

### REQ-ADP4-002 — Multi-Intent Happy Path

- `TestSearchCodeIntentHappyPath25Hits`: stub returns
  `testdata/search_code_response.json`; Filters=[{kind,"code"}]; assertion
  is 25 NormalizedDoc entries with each `Validate()` returning nil; URL
  path observed at the stub is `/search/code`; URL has `q=<text>`,
  `per_page=25`, `page=1`.
- `TestSearchIssuesIntentHappyPath25Hits`: same shape, kind=issues, URL
  path is `/search/issues`.
- `TestSearchReposIntentHappyPath25Hits`: same shape, kind=repos, URL
  path is `/search/repositories`.
- `TestSearchDefaultIntentIsRepos`: Filters=nil; URL path is
  `/search/repositories`.
- `TestSearchClampsPerPageTo100`: q.MaxResults=500 → URL has
  `per_page=100`.
- `TestSearchDefaultsPerPageTo25`: q.MaxResults=0 → URL has
  `per_page=25`.
- `TestSearchSetsPageWhenCursorPresent`: q.Cursor="3" → URL has `page=3`.
- `TestSearchSetsPage1WhenCursorEmpty`: q.Cursor="" → URL has `page=1`.

### REQ-ADP4-003 — Rate-Limit Mapping

- `TestSearchPrimaryRateLimitMapsToCategory`: stub returns 403 +
  `X-RateLimit-Remaining: 0` + `X-RateLimit-Reset: <future>`; assert
  returned err is `*types.SourceError` with
  `Category=CategoryRateLimited`, `HTTPStatus=403`, `RetryAfter > 0`,
  `RetryAfter ≤ 90s`.
- `TestSearchAbuseRateLimitMapsToCategory`: stub returns abuse-detection
  body that go-github recognises as `*AbuseRateLimitError`; assert
  Category=CategoryRateLimited, RetryAfter parsed from the body's
  `Retry-After: <N>` directive.
- `TestSearchRateLimitRetryAfterCapped90s`: `Retry-After: 999` →
  RetryAfter=90s.
- `TestSearchRateLimitNoInternalRetry`: stub records request count;
  assert exactly 1 outbound request observed.
- `TestSearchRateLimitNegativeOrZeroDefaults5s`: synthesised case where
  Rate.Reset is in the past → RetryAfter=5s (the default).

### REQ-ADP4-004 — HTTP 4xx Permanent / 5xx Unavailable / Network

- `TestSearchHTTP401`, `TestSearchHTTP403NonAbuse`, `TestSearchHTTP404`,
  `TestSearchHTTP422ValidationFailed` each assert `errors.Is(err,
  types.ErrPermanent)` and matching HTTPStatus.
- `TestSearchHTTP500`, `TestSearchHTTP503` each assert
  `errors.Is(err, types.ErrSourceUnavailable)` and matching HTTPStatus.
- `TestSearchConnectionRefused` (httptest.Server closed before request)
  asserts `errors.Is(err, types.ErrSourceUnavailable)` and HTTPStatus=0.
- `TestSearchUnavailablePreservesUnderlyingError`: assert
  `errors.Unwrap(srcErr) != nil` and the inner error message contains
  "connection refused" or equivalent.
- `TestSearchHTTP4xxNoInternalRetry`: assert exactly 1 request observed.

### REQ-ADP4-005 — NormalizedDoc Field Mapping

- `TestParseCodeResultsFieldMapping` table-drives 5 fixtures (Go file,
  Python file, TypeScript file, file with `Language=nil`, deleted-fork
  file with various nil pointers). For each, asserts every NormalizedDoc
  field per the §6.3.1 mapping table.
- `TestParseIssueResultsFieldMapping` table-drives 5 fixtures (open
  issue, closed issue, open PR, closed PR, deleted-author issue with
  `User=nil`). For each, asserts every NormalizedDoc field per §6.3.2;
  asserts `DocType=DocTypeIssue` regardless of issue-vs-PR; asserts
  `Metadata["is_pull_request"]` correctly distinguishes.
- `TestParseRepoResultsFieldMapping` table-drives 5 fixtures (public
  popular repo, archived repo, fork, repo with `Description=nil`, repo
  with `Owner=nil`). For each, asserts every NormalizedDoc field per
  §6.3.3.
- `TestParseDeletedUserNilSafe`: issue with `User: nil` → returned doc
  has `Author == ""` and `Validate()` still passes.
- `TestParseNoLanguageNilSafe`: code result with `Language: nil` →
  Metadata["language"] == "" and `Validate()` still passes.
- `TestParsePaginationCursor`: response with `NextPage=2, LastPage=10` →
  last doc Metadata["next_cursor"] == "2"; earlier docs do NOT have the
  `next_cursor` key.
- `TestParseNoCursorOnLastPage`: response with `NextPage=0` (last page)
  → no doc has the `next_cursor` key.
- `TestParseHashEmpty`: every returned `NormalizedDoc.Hash` equals `""`.
- `TestParseMetadataKeysPerIntent`: per-intent assertions:
  - code: every doc has `{repo_full_name, path, sha, language, kind:"code"}`
  - issues: every doc has
    `{repo_full_name, number, state, is_pull_request, comments,
    kind ∈ {"issue","pr"}}`
  - repos: every doc has
    `{full_name, language, stars, forks, open_issues, kind:"repo"}`

### REQ-ADP4-006 — User-Agent + Authorization Headers

- `TestSearchSetsCustomUserAgent`: captured request header `User-Agent`
  starts with `"usearch/"` and contains
  `"(+https://github.com/elymas/universal-search)"`.
- `TestSearchSetsAuthorizationHeader`: captured `Authorization` header
  starts with `"Bearer "` (the go-github idiom). Header value's bearer
  token equals the test fixture token.
- `TestSearchUserAgentVersionConfigurable`:
  `Options.UserAgentVersion = "v0.2-rc1"` → captured `User-Agent`
  contains `"usearch/v0.2-rc1"`.
- `TestSearchTokenNotInErrorMessage`: stub forces a 401; capture
  returned `*SourceError`; assert `srcErr.Cause.Error()` does NOT
  contain the test token substring.
- `TestSearchTokenNotInSlogOutput`: capture all slog records emitted by
  the registry's wrappedAdapter during a Search call; assert no
  attribute key/value pair contains the test token substring.

### REQ-ADP4-007 — Filter Qualifier Append

- `TestSearchSinceFilterAddsCreatedQualifier`:
  Filters=[{since, "2026-01-01T00:00:00Z"}] → captured request URL has
  `q` parameter ending in `created:>=2026-01-01T00:00:00Z` (URL-decoded
  inspection).
- `TestSearchLanguageFilterAddsLanguageQualifier`:
  Filters=[{language, "go"}] → URL `q` ends in `language:go`.
- `TestSearchRepoFilterAddsRepoQualifier`:
  Filters=[{repo, "facebook/react"}] → URL `q` ends in
  `repo:facebook/react`.
- `TestSearchMultipleFiltersJoinedBySpace`: two filters → URL `q`
  contains both qualifiers separated by a single space.
- `TestSearchUnknownFilterIgnored`: Filters=[{nsfw, "true"}] → URL `q`
  unchanged (no qualifier appended); no error returned.
- `TestSearchMalformedSinceDropped`: Filters=[{since, "not-a-date"}]
  → URL `q` unchanged; no error returned.
- `TestSearchEmptyFilterValueDropped`: Filters=[{language, ""}]
  → URL `q` unchanged; no error returned.
- `TestSearchFiltersOnlyAppendForApplicableIntent`:
  Filters=[{is_pr, "true"}] with kind=code → ignored (URL `q`
  unchanged); kind=issues → applied (URL `q` contains `is:pr`).

### REQ-ADP4-008 — Empty Query / Invalid Cursor / Invalid Intent

- `TestSearchEmptyQueryRejectedNoHTTP` table-drives `q.Text` over
  `["", "   ", "\t\n  \r"]`; for each asserts
  `errors.Is(err, types.ErrPermanent)` AND
  `errors.Is(err, ErrInvalidQuery)`. The httptest.Server is instrumented
  with a request counter; assert exactly 0 requests.
- `TestSearchInvalidCursorRejectedNoHTTP` table-drives `q.Cursor` over
  `["abc", "0", "-1", "1.5", "1e3"]`; for each asserts
  `errors.Is(err, types.ErrPermanent)` AND
  `errors.Is(err, ErrInvalidCursor)`; assert zero requests.
- `TestSearchInvalidIntentRejectedNoHTTP`:
  Filters=[{kind, "users"}]; assert
  `errors.Is(err, types.ErrPermanent)` AND
  `errors.Is(err, ErrInvalidIntent)`; assert zero requests.

### REQ-ADP4-009 — Redirect Allowlist

- `TestSearchFollowsAllowlistRedirect`: server A returns 302 with
  Location header pointing to server B (Host header rewritten to
  `api.github.com`); test installs server B as a custom
  `http.RoundTripper` resolver. Assert search succeeds and returns the
  body from server B.
- `TestSearchRejectsCrossDomainRedirect`: server A returns 302 with
  Location `https://attacker.com/x`. Assert
  `errors.Is(err, types.ErrPermanent)` and error message contains
  `"cross-domain redirect"`.
- `TestSearchRejectsRedirectChainOver3`: 4 servers chained within the
  allowlist; assert error returned after 3 hops with message containing
  `"too many redirects"`.

### REQ-ADP4-010 — Concurrent Search Safety (State-Driven)

- `TestSearchConcurrentSafe`: a single `*Adapter` is constructed pointing
  at one `httptest.Server` (which records every inbound request). 50
  goroutines are launched, each calling `(*Adapter).Search(ctx, q)`
  exactly once with the same query. All goroutines start via a
  `sync.WaitGroup` barrier so the invocations overlap.
- Assertions:
  1. The test executes successfully under `go test -race`; the race
     detector reports zero data-race alarms attributable to the adapter
     package.
  2. The stub server's request counter equals 50.
  3. Every goroutine receives `(docs, nil)` with `len(docs) == 25`
     (matching the standard `search_repos_response.json` fixture); each
     returned `[]types.NormalizedDoc` slice has every doc passing
     `Validate()` returning nil.

### NFR-ADP4-001 — Parse-Path Performance

- `BenchmarkParseGitHubResponse25Results` is invoked as
  `go test -bench=BenchmarkParseGitHubResponse25Results -benchtime=10x -count=5 ./internal/adapters/github/...`
  on amd64.
- Assertion mechanism: take the 5 reported per-op mean wall-clock
  durations (one per `-count` run); the MEDIAN of those 5 values SHALL
  be ≤ 5 ms. PASS/FAIL is decidable from the `go test -bench` output
  alone — no external CI script required.
- The bench reports `B/op` and `allocs/op`; `allocs/op` ≤ 625 (= 25 × 25
  results). Floor analysis: `*github.Repository` exposes 12+ nullable
  pointer fields each requiring a `safeStr/safeInt/safeBool/safeTime`
  call; combined with the `pkg/types.NormalizedDoc.Metadata =
  map[string]any` floor (~17 allocs/doc inherited from ADP-001
  iteration-3 amendment), the per-result floor is empirically expected
  near 22-25 allocs.

### NFR-ADP4-002 — E2E p95 (Stub)

- `TestSearchE2ELatencyStubP95` runs 100 invocations against the stub
  `httptest.Server`, sorts elapsed durations, asserts
  `durations[94] ≤ 200ms`.

### NFR-ADP4-003 — Goroutine Leak Check

- `TestSearchNoGoroutineLeakOnCancel`: `goleak.VerifyNone(t)` succeeds
  after a `Search` call whose ctx was cancelled at 50ms while the stub
  server delays response by 200ms.
- `TestMain` in `bench_test.go` invokes `goleak.VerifyTestMain(m)`
  (mirrors `internal/adapters/reddit/bench_test.go` pattern).

### NFR-ADP4-004 — Resource Cleanup

- `TestSearchNoLeakedFileDescriptors`: counts the test process's open
  file descriptors before+after 100 Search calls; asserts the delta is
  ≤ 5 (allowing minor httptest churn from net/http connection-pool
  warmup).

### Integration Checkpoint (M3 Exit Criterion Progress)

When SPEC-FAN-001 (M3 fanout) is consumed by a `usearch query` invocation
that has access to all three M3-completed adapters (Reddit + HN + GitHub),
a query like `"react state management"` SHOULD return interleaved hits
across all three sources. ADP-004's acceptance includes a smoke check
(via `cmd/usearch/integration_test.go` once the CLI integration lands —
NOT this SPEC's responsibility) that
`registry.Get("github").Search(ctx, q)` against a stub returns parseable
`[]NormalizedDoc` per the per-intent shape. This integration assertion
lives in CLI-001 / FAN-001's acceptance criteria, not here, but is
documented for traceability.

---

## 6. Technical Approach

### 6.1 Files to Modify

**Created (12 files)**:

- `internal/adapters/github/github.go` — Adapter struct, New, Name,
  Capabilities, Healthcheck, compile-time interface assertion
- `internal/adapters/github/github_test.go` — interface conformance tests
- `internal/adapters/github/search.go` — Search method (the hot path),
  intent dispatch, qualifier append
- `internal/adapters/github/search_test.go` — main test file (largest)
- `internal/adapters/github/client.go` — HTTP client construction,
  go-github client construction, redirect allowlist, categorizeError
  rosetta
- `internal/adapters/github/client_test.go` — error mapping + redirect
  tests
- `internal/adapters/github/parse.go` — parseCodeResults /
  parseIssueResults / parseRepoResults transforms + safe* nil-guard
  helpers
- `internal/adapters/github/parse_test.go` — per-intent field mapping
  tests
- `internal/adapters/github/score.go` — normalizeScore Tanh formula
  (verbatim from ADP-001)
- `internal/adapters/github/score_test.go` — score normalization tests
- `internal/adapters/github/errors.go` — ErrInvalidQuery /
  ErrInvalidCursor / ErrInvalidIntent / ErrMissingToken sentinels +
  parseRetryAfter helper (90s cap)
- `internal/adapters/github/bench_test.go` — NFR-ADP4-001 benchmark + TestMain goleak
- `internal/adapters/github/testdata/` (9 JSON fixtures) — see §2.1.m

**Modified (1 file)**:

- `go.mod` — add `github.com/google/go-github/v85` direct dep + transitive
  `github.com/google/go-querystring`. The run-phase implementer runs
  `go mod tidy` once the import is added; pinning policy follows
  `.claude/rules/moai/core/lsp-client.md` (track upgrades through
  integration test suite).

**Unchanged (by design)**:

- `pkg/types/*` — no contract change required. ADP-004 consumes the
  existing `RequiresAuth` + `AuthEnvVars` fields verified by reading
  `pkg/types/capabilities.go:51-55` (already shipped in SPEC-CORE-001).
- `internal/adapters/registry.go` — wrappedAdapter sole-emitter pattern
  preserved; the auth-validation path
  (`internal/adapters/registry.go:123-129`) ALREADY supports
  `AuthEnvVars` lookup. ADP-004 is the first adapter to exercise this
  path against a real env var.
- `internal/obs/metrics/metrics.go` — no new metric family.
- `cmd/usearch/main.go` — registry construction + adapter registration
  is owned by SPEC-CLI-001; ADP-004 does not modify cmd code. The CLI
  may add a startup-time check that `USEARCH_GITHUB_TOKEN` is set in the
  environment when `--source github` is requested (or simply when
  configuring the registry); that wiring lives in CLI-001's scope.

### 6.2 Package Layout

```
internal/adapters/github/
├── github.go                              # Adapter, New, Name, Capabilities, Healthcheck, interface assertion
├── github_test.go                         # Interface conformance + Capabilities determinism + auth declaration
├── search.go                              # (*Adapter).Search hot path + intent dispatch + qualifier append
├── search_test.go                         # E2E + per-intent happy paths + error categorisation tests
├── client.go                              # HTTP client + go-github client + redirect allowlist + categorizeError
├── client_test.go                         # categorizeError table + redirect allowlist + UA/Auth headers
├── parse.go                               # parseCodeResults / parseIssueResults / parseRepoResults + safe* helpers
├── parse_test.go                          # Field mapping table tests (3 sub-tables)
├── score.go                               # normalizeScore (Tanh formula, identical to ADP-001)
├── score_test.go                          # Score normalization table
├── errors.go                              # 4 sentinels + parseRetryAfter helper (90s cap)
├── bench_test.go                          # BenchmarkParseGitHubResponse25Results + TestMain goleak
└── testdata/
    ├── search_code_response.json          # 25-hit code search
    ├── search_issues_response.json        # 25-hit issue/PR mix
    ├── search_repos_response.json         # 25-hit repo search (default intent)
    ├── search_response_empty.json         # Zero hits
    ├── search_response_pagination.json    # NextPage=2, LastPage=10
    ├── search_response_deleted_author.json # User=nil exercise
    ├── search_response_429_rate_limited.json # Primary rate limit
    ├── search_response_403_abuse.json     # Abuse rate limit
    ├── search_response_422_validation.json # Malformed query syntax
    └── search_response_malformed.json     # Truncated JSON for parse-error path
```

### 6.3 GitHub Typed Result → NormalizedDoc Field Mapping

#### 6.3.1 Code Search Hit (`*github.CodeResult`)

| go-github Field | NormalizedDoc Field | Transform |
|-----------------|---------------------|-----------|
| `Repository.GetFullName() + "@" + safeStr(SHA) + ":" + safeStr(Path)` | `ID` | Composite ID |
| (constant) | `SourceID` | `"github"` |
| `safeStr(HTMLURL)` | `URL` | Use as-is |
| `safeStr(Path)` truncated to 80 runes | `Title` | File path display |
| (constant) | `Body` | `""` (no follow-up content fetch in v0.1) |
| First 280 runes of `safeStr(Path) + " — " + Repository.GetFullName() + " (" + safeStr(Language) + ")"` | `Snippet` | Synthesised summary |
| (zero) | `PublishedAt` | Code search has no per-file timestamp |
| (parse time) | `RetrievedAt` | `time.Now().UTC()` |
| (none) | `Author` | `""` |
| `normalizeScore(int(safeFloat(Score)))` | `Score` | GitHub-provided ~`[0, 100+]` |
| (constant) | `Lang` | `""` (programming language is in Metadata) |
| (constant) | `DocType` | `types.DocTypeRepo` (Open Question §11.1) |
| (nil) | `Citations` | `nil` |
| (constructed) | `Metadata` | REQUIRED: `repo_full_name`, `path`, `sha`, `language`, `kind="code"`. OPTIONAL: `file_size` (int when SafeIntFileSize > 0), `score` (raw float64). |
| (constant) | `Hash` | `""` |

#### 6.3.2 Issue / PR Search Hit (`*github.Issue`)

| go-github Field | NormalizedDoc Field | Transform |
|-----------------|---------------------|-----------|
| `"github:issue:" + strconv.FormatInt(safeInt64(ID), 10)` | `ID` | Stable global issue ID |
| (constant) | `SourceID` | `"github"` |
| `safeStr(HTMLURL)` | `URL` | Use as-is |
| `safeStr(Title)` | `Title` | Use as-is |
| `safeStr(Body)` | `Body` | Markdown preserved (no HTML stripping) |
| First 280 runes of `safeStr(Body)`; falls back to `truncateRunes(safeStr(Title), 280)` when Body empty | `Snippet` | Same truncation discipline |
| `safeTime(CreatedAt).UTC()` | `PublishedAt` | RFC 3339 → UTC |
| (parse time) | `RetrievedAt` | `time.Now().UTC()` |
| `safeStr(User.Login)` (when User != nil; else `""`) | `Author` | Nil-safe |
| `normalizeScore(safeInt(Comments) * 10)` | `Score` | Engagement proxy |
| (constant) | `Lang` | `""` |
| (constant) | `DocType` | `types.DocTypeIssue` (regardless of issue-vs-PR) |
| (nil) | `Citations` | `nil` |
| (constructed) | `Metadata` | REQUIRED: `repo_full_name` (derived from URL or RepositoryURL), `number` (int), `state` (string: "open"/"closed"), `is_pull_request` (bool, true when `PullRequestLinks != nil`), `comments` (int), `kind` ("issue" when `PullRequestLinks==nil`, "pr" when not). OPTIONAL: `labels` ([]string from Labels[*].Name), `updated_at` (RFC 3339 string), `reactions_total_count` (int when Reactions != nil). |
| (constant) | `Hash` | `""` |

#### 6.3.3 Repository Search Hit (`*github.Repository`)

| go-github Field | NormalizedDoc Field | Transform |
|-----------------|---------------------|-----------|
| `"github:repo:" + strconv.FormatInt(safeInt64(ID), 10)` | `ID` | Stable global repo ID |
| (constant) | `SourceID` | `"github"` |
| `safeStr(HTMLURL)` | `URL` | Use as-is |
| `safeStr(FullName)` | `Title` | e.g., `"facebook/react"` |
| `safeStr(Description)` | `Body` | Use as-is |
| First 280 runes of `safeStr(Description)`; falls back to `safeStr(FullName)` when Description empty | `Snippet` | |
| `safeTime(CreatedAt).UTC()` | `PublishedAt` | Repo creation timestamp |
| (parse time) | `RetrievedAt` | `time.Now().UTC()` |
| `safeStr(Owner.Login)` (when Owner != nil; else `""`) | `Author` | Owner login |
| `normalizeScore(safeInt(StargazersCount))` | `Score` | Stars |
| (constant) | `Lang` | `""` |
| (constant) | `DocType` | `types.DocTypeRepo` |
| (nil) | `Citations` | `nil` |
| (constructed) | `Metadata` | REQUIRED: `full_name`, `language` (may be empty), `stars` (int), `forks` (int), `open_issues` (int), `kind="repo"`. OPTIONAL: `topics` ([]string), `default_branch`, `pushed_at` (RFC 3339 string), `size_kb` (int). |
| (constant) | `Hash` | `""` |

### 6.4 Filter Qualifier Append (REQ-ADP4-007)

GitHub's search syntax supports inline qualifiers (`language:go`,
`repo:owner/name`, `created:>=2026-01-01`, etc.) appended to the `q`
parameter. The adapter translates a small subset of `Query.Filters` keys
into qualifier suffixes, appended to `q.Text` separated by single spaces
before the request is built.

| Filter Key | Applicable Intents | Qualifier Form |
|------------|---------------------|----------------|
| `since` | code, issues, repos | `created:>=<RFC3339>` (when Value parses) |
| `language` | code, repos | `language:<Value>` (passed literally; GitHub's syntax is forgiving) |
| `repo` | issues | `repo:<owner>/<name>` (Value passed literally) |
| `org` | code, issues, repos | `org:<Value>` |
| `user` | code, issues, repos | `user:<Value>` |
| `topic` | repos | `topic:<Value>` |
| `state` | issues | `state:<open\|closed>` (Value validated; else dropped) |
| `is_pr` | issues | `is:pr` (any non-empty Value) |

Filters with non-applicable intent are silently ignored. Malformed values
(empty, unparseable for `since`, invalid for `state`) are silently
dropped. Unknown keys are silently ignored. The append uses a single
space separator; the resulting `q` is URL-escaped by go-github when the
request is built.

### 6.5 Type Sketches (illustrative; final shapes in run phase)

```go
// internal/adapters/github/github.go
package github

import (
    "context"
    "fmt"
    "net"
    "net/http"

    gogithub "github.com/google/go-github/v85/github"

    "github.com/elymas/universal-search/pkg/types"
)

const (
    defaultBaseURL           = "" // empty → go-github uses the canonical https://api.github.com
    defaultUserAgentTemplate = "usearch/%s (+https://github.com/elymas/universal-search)"
    defaultUAVersion         = "v0.1"
    defaultHealthcheckTarget = "api.github.com:443"
)

type Options struct {
    BaseURL           string        // override; primarily for tests pointing at httptest.Server
    HTTPClient        *http.Client  // override; default is newDefaultHTTPClient()
    Token             string        // PAT; required when SkipAuthCheck is false and HTTPClient is default
    UserAgentVersion  string        // default "v0.1"
    HealthcheckTarget string        // default "api.github.com:443"
    SkipAuthCheck     bool          // tests only; when true, Token may be empty
}

type Adapter struct {
    ghClient          *gogithub.Client
    httpClient        *http.Client
    baseURL           string
    userAgent         string
    healthcheckTarget string
}

func New(opts Options) (*Adapter, error) {
    if !opts.SkipAuthCheck && opts.HTTPClient == nil && opts.Token == "" {
        return nil, &types.SourceError{
            Adapter:  "github",
            Category: types.CategoryPermanent,
            Cause:    ErrMissingToken,
        }
    }
    httpClient := opts.HTTPClient
    if httpClient == nil {
        httpClient = newDefaultHTTPClient()
    }
    ghClient := gogithub.NewClient(httpClient)
    if opts.Token != "" {
        ghClient = ghClient.WithAuthToken(opts.Token)
    }
    if opts.BaseURL != "" {
        baseURL, err := ghClient.SetBaseURL(opts.BaseURL)
        if err != nil {
            return nil, err
        }
        _ = baseURL
    }
    version := firstNonEmpty(opts.UserAgentVersion, defaultUAVersion)
    ua := fmt.Sprintf(defaultUserAgentTemplate, version)
    ghClient.UserAgent = ua

    target := firstNonEmpty(opts.HealthcheckTarget, defaultHealthcheckTarget)
    return &Adapter{
        ghClient:          ghClient,
        httpClient:        httpClient,
        baseURL:           opts.BaseURL,
        userAgent:         ua,
        healthcheckTarget: target,
    }, nil
}

func (a *Adapter) Name() string { return "github" }

func (a *Adapter) Capabilities() types.Capabilities {
    return types.Capabilities{
        SourceID:          "github",
        DisplayName:       "GitHub",
        DocTypes:          []types.DocType{types.DocTypeRepo, types.DocTypeIssue},
        SupportedLangs:    nil,
        SupportsSince:     true,
        RequiresAuth:      true,
        AuthEnvVars:       []string{"USEARCH_GITHUB_TOKEN"},
        RateLimitPerMin:   30,
        DefaultMaxResults: 25,
        Notes: "GitHub REST search via google/go-github/v85. PAT auth via " +
            "USEARCH_GITHUB_TOKEN env var (public_repo scope sufficient; " +
            "private-repo search out of v0.1 scope). Multi-intent routing " +
            "via Query.Filters[Key=\"kind\"] in {code, issues, repos}; " +
            "default repos when omitted. Per-intent rate ceilings: code " +
            "search 9/min vs issues/repos 30/min — Capabilities advertises " +
            "the conservative 30/min single value. 5000 req/hr primary " +
            "(authenticated). Retry-After cap 90s (vs 60s in Reddit/HN; " +
            "GitHub's secondary-rate-limit recovery semantics warrant the " +
            "longer cap). Path A (wrapping github/github-mcp-server) " +
            "rejected for v0.1 — see SPEC-ADP-004 §2.2.",
    }
}

func (a *Adapter) Healthcheck(ctx context.Context) error {
    var d net.Dialer
    conn, err := d.DialContext(ctx, "tcp", a.healthcheckTarget)
    if err != nil {
        return err
    }
    return conn.Close()
}

var _ types.Adapter = (*Adapter)(nil)
```

```go
// internal/adapters/github/client.go (excerpt)
import "github.com/elymas/universal-search/internal/obs/reqid"

func newDefaultHTTPClient() *http.Client {
    return &http.Client{
        Timeout:       10 * time.Second,
        Transport:     reqid.NewTransport(http.DefaultTransport),
        CheckRedirect: redirectAllowlist,
    }
}

var allowedRedirectHosts = map[string]struct{}{
    "api.github.com":            {},
    "github.com":                {},
    "raw.githubusercontent.com": {},
    "codeload.github.com":       {},
}

func redirectAllowlist(req *http.Request, via []*http.Request) error {
    if len(via) >= 3 {
        return errors.New("github: too many redirects (max 3)")
    }
    host := req.URL.Hostname()
    if _, ok := allowedRedirectHosts[host]; !ok {
        return fmt.Errorf("github: cross-domain redirect rejected: %s", host)
    }
    return nil
}

// categorizeError uses errors.As against the typed go-github errors and
// returns a *types.SourceError with the appropriate Category. nil → nil.
func categorizeError(err error) *types.SourceError {
    if err == nil {
        return nil
    }
    var rateLimitErr *gogithub.RateLimitError
    if errors.As(err, &rateLimitErr) {
        retryAfter := time.Until(rateLimitErr.Rate.Reset.Time)
        if retryAfter <= 0 {
            retryAfter = defaultRetryAfter
        }
        if retryAfter > maxRetryAfter {
            retryAfter = maxRetryAfter
        }
        return &types.SourceError{
            Adapter:    "github",
            Category:   types.CategoryRateLimited,
            HTTPStatus: rateLimitErr.Response.StatusCode,
            RetryAfter: retryAfter,
            Cause:      rateLimitErr,
        }
    }
    var abuseErr *gogithub.AbuseRateLimitError
    if errors.As(err, &abuseErr) {
        retryAfter := defaultRetryAfter
        if abuseErr.RetryAfter != nil && *abuseErr.RetryAfter > 0 {
            retryAfter = *abuseErr.RetryAfter
        }
        if retryAfter > maxRetryAfter {
            retryAfter = maxRetryAfter
        }
        status := 0
        if abuseErr.Response != nil {
            status = abuseErr.Response.StatusCode
        }
        return &types.SourceError{
            Adapter:    "github",
            Category:   types.CategoryRateLimited,
            HTTPStatus: status,
            RetryAfter: retryAfter,
            Cause:      abuseErr,
        }
    }
    var errResp *gogithub.ErrorResponse
    if errors.As(err, &errResp) {
        status := errResp.Response.StatusCode
        if status >= 400 && status < 500 {
            return &types.SourceError{
                Adapter:    "github",
                Category:   types.CategoryPermanent,
                HTTPStatus: status,
                Cause:      errResp,
            }
        }
        if status >= 500 && status < 600 {
            return &types.SourceError{
                Adapter:    "github",
                Category:   types.CategoryUnavailable,
                HTTPStatus: status,
                Cause:      errResp,
            }
        }
    }
    // Network-layer or unknown — treat as unavailable.
    return &types.SourceError{
        Adapter:    "github",
        Category:   types.CategoryUnavailable,
        HTTPStatus: 0,
        Cause:      err,
    }
}
```

### 6.6 HTTP Client Construction Notes

Identical philosophy to ADP-001 §6.5 / ADP-002 §6.5:

- **Timeout**: 10 seconds total request deadline (default). Caller's ctx
  deadline takes precedence.
- **Redirect policy**: `CheckRedirect` enforces the allowlist
  `{api.github.com, github.com, raw.githubusercontent.com,
  codeload.github.com}` and caps at 3 hops.
- **Transport**: `reqid.NewTransport(http.DefaultTransport)` for
  request-ID propagation.
- **Headers per request**: `User-Agent: usearch/<version>
  (+https://github.com/elymas/universal-search)` (set on go-github client
  via `client.UserAgent`); `Accept: application/json` (set by go-github
  by default); `Authorization: Bearer <token>` (set by go-github via
  `WithAuthToken`).

### 6.7 Observability Note

The GitHub adapter emits ZERO metrics, logs, and spans of its own. ALL
observability comes from the registry's `wrappedAdapter`
(`internal/adapters/registry.go:172-263`). Sole-emitter discipline
preserved verbatim from ADP-001/002.

The adapter's responsibility is to return a correctly-categorised
`*types.SourceError` so the wrappedAdapter computes the right `outcome`
label via `types.OutcomeFromError(err)` (per
`pkg/types/errors.go:174-193`):

- `nil` → `"success"`
- `context.DeadlineExceeded` → `"timeout"`
- `CategoryRateLimited` → `"rate_limited"`
- `CategoryUnavailable` → `"unavailable"`
- `CategoryTransient` → `"transient"`
- `CategoryPermanent` / Unknown → `"failure"`

### 6.8 MX Tag Plan

| File::Symbol | Tag | Reason |
|--------------|-----|--------|
| `github.go::(*Adapter).Search` (defined in `search.go`) | `@MX:ANCHOR` | Sole entry point for all GitHub fanout calls. fan_in ≥ 3 (registry wrappedAdapter, FAN-001 fanout, tests). `@MX:REASON: contract boundary; signature change ripples to FAN-001 + IDX-001 + SYN-001`. |
| `parse.go::parseRepoResults` | `@MX:ANCHOR` | Default-intent transform; every default-routed GitHub search passes through this transform. fan_in = 1 (Search) but invariant-bearing — bug here corrupts every NormalizedDoc returned for the most common intent. `@MX:REASON: NormalizedDoc field-mapping integrity gate for default repo intent`. |
| `score.go::normalizeScore` (function) and constants `tanhDivisor=100.0, scoreCenter=0.5` | `@MX:NOTE` | Documents the Tanh formula choice and tie-in to SPEC-IDX-001 RRF. Same shape as ADP-001/002 score.go. |
| `client.go::categorizeError` | `@MX:NOTE` | The go-github typed-error rosetta. Future contributors will look here first when a new go-github error type needs handling. |
| `client.go::newDefaultHTTPClient` | `@MX:WARN` | Outbound network call (via go-github). Redirect allowlist enforces SSRF safety boundary. `@MX:REASON: removing the CheckRedirect guard re-opens SSRF via GitHub CDN redirects`. |
| `client.go::allowedRedirectHosts` map | `@MX:NOTE` | The 4-entry redirect allowlist. Adding a host requires a security review. |
| `search.go::appendQualifiers` | `@MX:NOTE` | The 8-key filter-qualifier mapping. Future contributors adding a new qualifier key should update this function and the table in §6.4. |

All tags are `[AUTO]`-prefixed (agent-generated), include
`@MX:SPEC: SPEC-ADP-004`, and follow `code_comments: en` per
`.moai/config/sections/language.yaml`.

### 6.9 Harness Level

Per `.moai/config/sections/harness.yaml` auto-routing: 10 EARS REQs (8 ×
P0 + 2 × P1) + 4 NFRs touching 1 package (`internal/adapters/github/`,
~12 source/test files + 9 testdata fixtures) + zero cross-package edits
(only go.mod for the new dep) + zero security/payment/PII keywords (auth
env-var addition is non-secret-managing — the registry handles
validation; the SPEC does not handle credentials, just declares them) +
zero compose/env/config deltas beyond go.mod = **standard** harness
level. Sprint Contract is OPTIONAL but recommended. Evaluator profile
`default` applies.

---

## 7. What NOT to Build (Exclusions)

[HARD] This SPEC explicitly excludes the following items. Each has a
known destination SPEC; this list prevents scope creep into ADP-004.

- **Per-source customisations for arXiv, YouTube, Bluesky, X, SearXNG,
  Naver, Daum, KoreaNewsCrawler, RSS, Polymarket** → SPEC-ADP-003,
  SPEC-ADP-005..009 (M3).
- **Retry orchestration** → SPEC-FAN-001 D6 + future SPEC-FAN-001-RETRY.
  Adapter is one-shot per call.
- **Response caching** → SPEC-CACHE-001 (M3). Adapter is stateless.
- **Result ranking, deduplication, RRF fusion** → SPEC-IDX-001 (M3).
- **Code-content fetch follow-up** (`/repos/{owner}/{repo}/contents/{path}`)
  → out of v0.1; Open Question §11.2.
- **GitHub App authentication** → SPEC-AUTH-002 (M6).
- **OAuth user-flow authentication** → SPEC-AUTH-001 (M6).
- **Private-repository search** (`repo` PAT scope) → SPEC-AUTH-002 (M6).
- **Sort customisation** (GitHub `sort=stars|forks|...` per endpoint) →
  out of v0.1; default relevance only.
- **Code search `text_matches` highlighting** → out of v0.1.
- **GraphQL v4 variant** → out of v0.1; Open Question §11.5.
- **MCP wrapping (Path A)** → REJECTED for v0.1; Open Question §11.4.
- **Live network integration tests in CI** → out of v0.1.
- **`DocTypeCode` enum addition** → SDK boundary change; Open Question
  §11.1; deferred.
- **Per-intent rate-limit ceiling field** → would require amending
  `pkg/types.Capabilities`; Open Question §11.7; deferred.
- **Per-adapter custom Prometheus metrics** → SPEC-OBS-001 allowlist
  amendment; out of v0.1.
- **HTML body stripping** — GitHub returns markdown, not HTML.
- **Korean-locale handling for GitHub** → SPEC-IDX-003 (M3); GitHub
  returns Lang="" (unknown).

---

## 8. TDD Plan

Development follows RED-GREEN-REFACTOR per
`quality.development_mode: tdd` (`.moai/config/sections/quality.yaml`).
Representative RED-phase tests, written before implementation, grouped
by REQ. Total: ~40 tests covering REQ-ADP4-001..010 + NFRs. Coverage
target: 85% per `quality.test_coverage_target`. Benchmarks do not count
toward coverage.

| # | Test | Layer | REQ | Assertion |
|---|------|-------|-----|-----------|
| 1 | `TestAdapterName` | `github_test.go` | REQ-ADP4-001 | `(*Adapter).Name() == "github"` |
| 2 | `TestAdapterImplementsInterface` | `github_test.go` | REQ-ADP4-001 | Compile-time assertion |
| 3 | `TestCapabilitiesDeterministic` | `github_test.go` | REQ-ADP4-001 | Two calls reflect.DeepEqual |
| 4 | `TestCapabilitiesShape` | `github_test.go` | REQ-ADP4-001 | All 9 fields + Notes substrings |
| 5 | `TestCapabilitiesDeclaresRequiresAuth` | `github_test.go` | REQ-ADP4-001 | RequiresAuth=true; AuthEnvVars=["USEARCH_GITHUB_TOKEN"] |
| 6 | `TestHealthcheckSucceeds` | `github_test.go` | REQ-ADP4-001 | TCP dial success against loopback |
| 7 | `TestNewMissingTokenRejected` | `github_test.go` | REQ-ADP4-001 | Empty Token + SkipAuthCheck=false → ErrMissingToken |
| 8 | `TestNewSkipAuthCheckAllowsEmptyToken` | `github_test.go` | REQ-ADP4-001 | Empty Token + SkipAuthCheck=true → no error (test usage) |
| 9 | `TestSearchCodeIntentHappyPath25Hits` | `search_test.go` | REQ-ADP4-002, REQ-ADP4-005 | 25 NormalizedDocs from /search/code |
| 10 | `TestSearchIssuesIntentHappyPath25Hits` | `search_test.go` | REQ-ADP4-002, REQ-ADP4-005 | 25 NormalizedDocs from /search/issues |
| 11 | `TestSearchReposIntentHappyPath25Hits` | `search_test.go` | REQ-ADP4-002, REQ-ADP4-005 | 25 NormalizedDocs from /search/repositories |
| 12 | `TestSearchDefaultIntentIsRepos` | `search_test.go` | REQ-ADP4-002 | Filters=nil → /search/repositories |
| 13 | `TestSearchClampsPerPageTo100` | `search_test.go` | REQ-ADP4-002 | q.MaxResults=500 → per_page=100 |
| 14 | `TestSearchDefaultsPerPageTo25` | `search_test.go` | REQ-ADP4-002 | q.MaxResults=0 → per_page=25 |
| 15 | `TestSearchSetsPageWhenCursorPresent` | `search_test.go` | REQ-ADP4-002 | q.Cursor="3" → page=3 |
| 16 | `TestSearchSetsPage1WhenCursorEmpty` | `search_test.go` | REQ-ADP4-002 | q.Cursor="" → page=1 |
| 17 | `TestSearchPrimaryRateLimitMapsToCategory` | `search_test.go` | REQ-ADP4-003 | 403 + X-RateLimit-Remaining=0 → CategoryRateLimited |
| 18 | `TestSearchAbuseRateLimitMapsToCategory` | `search_test.go` | REQ-ADP4-003 | abuse-detection → CategoryRateLimited with RetryAfter |
| 19 | `TestSearchRateLimitRetryAfterCapped90s` | `search_test.go` | REQ-ADP4-003 | Retry-After=999 → 90s |
| 20 | `TestSearchRateLimitNegativeOrZeroDefaults5s` | `search_test.go` | REQ-ADP4-003 | past Reset → 5s default |
| 21 | `TestSearchRateLimitNoInternalRetry` | `search_test.go` | REQ-ADP4-003 | 1 outbound request observed |
| 22 | `TestSearchHTTP4xx` | `search_test.go` | REQ-ADP4-004 | Table over 401/403/404/422 → ErrPermanent + matching HTTPStatus |
| 23 | `TestSearchHTTP5xx` | `search_test.go` | REQ-ADP4-004 | Table over 500/503 → ErrSourceUnavailable + matching HTTPStatus |
| 24 | `TestSearchConnectionRefused` | `search_test.go` | REQ-ADP4-004 | ErrSourceUnavailable; HTTPStatus=0 |
| 25 | `TestSearchUnavailablePreservesUnderlyingError` | `search_test.go` | REQ-ADP4-004 | Inner cause preserved |
| 26 | `TestParseCodeResultsFieldMapping` | `parse_test.go` | REQ-ADP4-005 | Table over 5 code fixtures |
| 27 | `TestParseIssueResultsFieldMapping` | `parse_test.go` | REQ-ADP4-005 | Table over 5 issue/PR fixtures |
| 28 | `TestParseRepoResultsFieldMapping` | `parse_test.go` | REQ-ADP4-005 | Table over 5 repo fixtures |
| 29 | `TestParseDeletedUserNilSafe` | `parse_test.go` | REQ-ADP4-005 | User=nil → Author="" |
| 30 | `TestParseNoLanguageNilSafe` | `parse_test.go` | REQ-ADP4-005 | Language=nil → Metadata["language"]="" |
| 31 | `TestParsePaginationCursor` | `parse_test.go` | REQ-ADP4-005 | NextPage=2 → last doc Metadata["next_cursor"]="2" |
| 32 | `TestParseNoCursorOnLastPage` | `parse_test.go` | REQ-ADP4-005 | NextPage=0 → no `next_cursor` key |
| 33 | `TestParseHashEmpty` | `parse_test.go` | REQ-ADP4-005 | Every doc Hash="" |
| 34 | `TestParseMetadataKeysPerIntent` | `parse_test.go` | REQ-ADP4-005 | Per-intent REQUIRED key set present |
| 35 | `TestParseMalformedJSON` | `parse_test.go` | REQ-ADP4-005 | Truncated JSON → SourceError{Permanent} |
| 36 | `TestSearchSetsCustomUserAgent` | `client_test.go` | REQ-ADP4-006 | UA starts with "usearch/" |
| 37 | `TestSearchSetsAuthorizationHeader` | `client_test.go` | REQ-ADP4-006 | Authorization: Bearer <token> present |
| 38 | `TestSearchUserAgentVersionConfigurable` | `client_test.go` | REQ-ADP4-006 | Options override propagates |
| 39 | `TestSearchTokenNotInErrorMessage` | `client_test.go` | REQ-ADP4-006 | Token substring NOT in *SourceError.Error() |
| 40 | `TestSearchTokenNotInSlogOutput` | `client_test.go` | REQ-ADP4-006 | Token substring NOT in slog records |
| 41 | `TestSearchSinceFilterAddsCreatedQualifier` | `search_test.go` | REQ-ADP4-007 | URL q ends in created:>=<RFC3339> |
| 42 | `TestSearchLanguageFilterAddsLanguageQualifier` | `search_test.go` | REQ-ADP4-007 | URL q ends in language:<value> |
| 43 | `TestSearchRepoFilterAddsRepoQualifier` | `search_test.go` | REQ-ADP4-007 | URL q ends in repo:owner/name |
| 44 | `TestSearchMultipleFiltersJoinedBySpace` | `search_test.go` | REQ-ADP4-007 | Two filters → single space separator |
| 45 | `TestSearchUnknownFilterIgnored` | `search_test.go` | REQ-ADP4-007 | Unknown key → no qualifier append |
| 46 | `TestSearchMalformedSinceDropped` | `search_test.go` | REQ-ADP4-007 | Bad RFC 3339 → no qualifier append |
| 47 | `TestSearchEmptyFilterValueDropped` | `search_test.go` | REQ-ADP4-007 | Empty Value → no qualifier append |
| 48 | `TestSearchFiltersOnlyAppendForApplicableIntent` | `search_test.go` | REQ-ADP4-007 | is_pr ignored on code intent; applied on issues intent |
| 49 | `TestSearchEmptyQueryRejectedNoHTTP` | `search_test.go` | REQ-ADP4-008 | Table over empty/whitespace q.Text → ErrPermanent + zero requests |
| 50 | `TestSearchInvalidCursorRejectedNoHTTP` | `search_test.go` | REQ-ADP4-008 | Table over invalid cursors → ErrPermanent + zero requests |
| 51 | `TestSearchInvalidIntentRejectedNoHTTP` | `search_test.go` | REQ-ADP4-008 | kind="users" → ErrInvalidIntent + zero requests |
| 52 | `TestSearchFollowsAllowlistRedirect` | `client_test.go` | REQ-ADP4-009 | 302 within allowlist followed |
| 53 | `TestSearchRejectsCrossDomainRedirect` | `client_test.go` | REQ-ADP4-009 | 302 to attacker.com → ErrPermanent + "cross-domain redirect" |
| 54 | `TestSearchRejectsRedirectChainOver3` | `client_test.go` | REQ-ADP4-009 | 4-hop chain rejected with "too many redirects" |
| 55 | `TestSearchConcurrentSafe` | `search_test.go` | REQ-ADP4-010 | 50 goroutines × 1 stub; race-clean; 50 requests; valid docs |
| 56 | `TestNormalizeScoreTable` | `score_test.go` | REQ-ADP4-005 | 7 score values within ±0.001 |
| 57 | `TestNormalizeScoreDeterministic` | `score_test.go` | REQ-ADP4-005 | Two calls byte-equal output |
| 58 | `TestParseRetryAfterTable` | `client_test.go` | REQ-ADP4-003 | Table over 6 inputs (incl. 90s cap) |
| 59 | `TestCategorizeErrorTable` | `client_test.go` | REQ-ADP4-003/004 | Table over typed-error space |
| 60 | `TestSearchE2ELatencyStubP95` | `search_test.go` | NFR-ADP4-002 | 100 invocations; p95 ≤ 200ms |
| 61 | `TestSearchNoGoroutineLeakOnCancel` | `search_test.go` | NFR-ADP4-003 | goleak.VerifyNone after mid-flight cancel |
| 62 | `TestSearchNoLeakedFileDescriptors` | `search_test.go` | NFR-ADP4-004 | FD delta ≤ 5 over 100 calls |
| 63 | `BenchmarkParseGitHubResponse25Results` | `bench_test.go` | NFR-ADP4-001 | Median of 5 ≤ 5ms; allocs/op ≤ 625 |
| 64 | `TestMain` (goleak.VerifyTestMain) | `bench_test.go` | NFR-ADP4-003 | Package-level leak check |

RED-GREEN-REFACTOR per requirement:

1. RED: Write failing test for REQ-ADP4-N.
2. GREEN: Implement minimal code to pass.
3. REFACTOR: Tidy; extract shared helpers if they remove duplication
   WITHIN the package; keep file sizes manageable (target each `.go`
   file < 250 LoC excluding tests).

Greenfield note: `internal/adapters/github/` does not exist. There is no
behaviour to preserve; no characterization tests needed.

---

## 9. Dependencies

### 9.1 Upstream SPEC Dependencies

- **SPEC-CORE-001 (implemented)**: provides `pkg/types.Adapter`,
  `pkg/types.Capabilities` (with `RequiresAuth` + `AuthEnvVars`
  fields verified at `pkg/types/capabilities.go:51-55`),
  `pkg/types.Query`, `pkg/types.NormalizedDoc`, `*types.SourceError`,
  `types.OutcomeFromError`, `types.DocType` enum (`DocTypeRepo` +
  `DocTypeIssue` available),
  `internal/adapters.Registry` with the auth-validation path
  (`internal/adapters/registry.go:123-129`) and wrappedAdapter sole-
  emitter pattern, `internal/adapters/noop` reference shape. HARD dep.
- **SPEC-OBS-001 (implemented)**: provides `obs.Obs` bundle,
  `internal/obs/reqid.NewTransport` for request-ID propagation,
  `AdapterCalls{adapter,outcome}` and `AdapterCallDuration{adapter}`
  collectors. SOFT dep — adapter is nil-safe via the registry's
  nil-guards. The `adapter="github"` value fits within the V1
  14-adapter ceiling.
- **SPEC-IR-001 (implemented)**: documents the consumer contract for
  `Capabilities` (REQ-IR-008 selects AdapterSet by intersecting
  `categoryEligibleDocTypes` with `SupportedLangs`). ADP-004's
  `Capabilities()` shape (DocTypes=[Repo, Issue], SupportedLangs=nil)
  determines which routing categories the GitHub adapter is selected
  for. SOFT dep.

### 9.2 Parallelizable

- **SPEC-FAN-001 (implemented; M3)**: fanout consumes
  `(*github.Adapter).Search` via `registry.Get("github").Search(ctx, q)`.
  ADP-004 does not block FAN-001; FAN-001 already merged.
- **SPEC-CLI-001 (implemented; M2)**: consumes the registry. ADP-004
  registration may require a CLI-side change to surface the
  `USEARCH_GITHUB_TOKEN` env-var requirement when the user passes
  `--source github` — out of ADP-004 scope.
- **SPEC-ADP-003 / SPEC-ADP-005..009 (M3)**: develop in parallel; each
  may copy ADP-004's authenticated-adapter shape if it requires auth.

### 9.3 Downstream Blocked SPECs

None — ADP-004 is a leaf. Future SPECs that may consume:

- **SPEC-IDX-001** (M3): consumes `NormalizedDoc.Score` (Tanh-normalised
  per intent) as one input to RRF fusion.
- **SPEC-CACHE-001** (M3): wraps fanout in 5-phase access fallback;
  ADP-004's adapter is one of the sources fanout dispatches to.
- **SPEC-AUTH-002** (M6): may add per-team GitHub Apps replacing the
  PAT auth path.

### 9.4 External Dependencies (run-phase pins)

**ONE new Go module dependency**:

- `github.com/google/go-github/v85` — direct dep; pinned to v85.0.0
  (released 2026-04-20). Pinning policy follows
  `.claude/rules/moai/core/lsp-client.md` — track upgrades through the
  integration test suite. The major-version path (`/v85/`) prevents
  accidental major bumps; minor-version bumps follow GitHub's API
  versioning (2022-11-28).
- `github.com/google/go-querystring` — transitive of go-github; BSD-3
  license; widely used.

**Existing dependencies consumed**:

- Go stdlib: `context`, `encoding/json`, `errors`, `fmt`, `math`,
  `net`, `net/http`, `net/url`, `strconv`, `strings`, `time`,
  `unicode`, `unicode/utf8`
- `pkg/types` (already pinned via SPEC-CORE-001)
- `internal/obs/reqid` (already pinned via SPEC-OBS-001)
- Test-only: `go.uber.org/goleak` (for NFR-ADP4-003) — already added by
  SPEC-ADP-001 run-phase.

---

## 10. Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Token missing at runtime under default CLI invocation | Medium | High | Registry's `RegisterWithOptions` validates `AuthEnvVars` at startup per `internal/adapters/registry.go:123-129`; missing env → `*RegistryError{ErrMissingAuth}` returned at startup. CLI surfaces this clearly (consumer of the registry). REQ-ADP4-001 + tests confirm `Capabilities` declares the env var correctly. |
| Token logged inadvertently | Low | High | go-github does not echo headers in error messages; adapter explicitly tests via `TestSearchTokenNotInErrorMessage` and `TestSearchTokenNotInSlogOutput` that the token substring never appears in error output or slog records. The registry's slog emitter at `internal/adapters/registry.go:240-251` logs only `name/outcome/elapsed/result_count/error.Error()` — no header dump. |
| 422 query syntax errors confused with auth failures | Medium | Low | Distinct mapping in `categorizeError`: 422 → `CategoryPermanent` with `HTTPStatus:422`; 401/403 → `CategoryPermanent` with `HTTPStatus:401/403`. Tests `TestSearchHTTP4xx` table-drives the discrimination. |
| Abuse detection (403) vs forbidden (403) ambiguity | Medium | Medium | go-github already disambiguates via typed errors; adapter uses `errors.As(err, &abuseErr)` to detect abuse, otherwise treats 403 as `CategoryPermanent`. Tests cover both paths. |
| Code-search 9/min ceiling vs declared 30/min `RateLimitPerMin` | Medium | Medium | `Capabilities.RateLimitPerMin=30` is the cross-intent figure; code-search may hit 9/min ceiling first. Open Question §11.7 + Capabilities.Notes documents. Fanout's 8-parallel cap means 8 concurrent code-search calls fill the ceiling within ~1.3 minutes. Acceptable for v0.1 thin slice; SPEC-FAN-001 retry policy will eventually consume the per-source `RetryAfter`. |
| go-github API changes between minor versions | Low | Low | Pinning policy mirrors powernap (`.claude/rules/moai/core/lsp-client.md`): bumping requires running the integration test suite first. `*github.RateLimitError`, `*github.AbuseRateLimitError`, `*github.ErrorResponse` types have been stable across go-github v40+ (3+ years). |
| New transitive dep `go-querystring` introduces supply-chain risk | Low | Low | go-querystring is a 13-year-old library, BSD-3 licensed, transitively pulled into thousands of Go projects. SPEC-DEP-001 owns supply-chain audit. |
| GitHub returns extra fields not in v0.1 schema | Low | Low | `encoding/json` (used internally by go-github) tolerates unknown fields; the adapter consumes only the documented field set. New fields can be opted-in via Metadata keys without breaking consumers. |
| Score synthesis for issues (`comments * 10`) miscalibrates ranking | Medium | Low | Open Question §11.6. SPEC-IDX-001 RRF re-ranks by rank not score across adapters, so per-adapter score curves matter less than determinism + bounded codomain. Revisit after M3 RRF measurements. |
| Pagination cursor opacity confuses downstream consumers | Low | Low | REQ-ADP4-005 surfaces the cursor via `Metadata["next_cursor"]` on the LAST doc. Cursor is conceptually opaque (string); consumers MUST pass it back as `Query.Cursor` without parsing. Documented in `Capabilities.Notes`. |
| `time.Now()` in `RetrievedAt` non-deterministic in tests | Low | Low | Per-intent parse functions accept `retrievedAt time.Time` parameter; tests inject a fixed time. Search wraps with `time.Now().UTC()` in production. |
| HTTP timeout (10s) too aggressive for GitHub during incidents | Low | Low | Configurable via `Options.HTTPClient`; default 10s aligns with NFR-ADP4-002 stub p95 200ms × 50× safety margin. |
| Nil-pointer dereference on `*Issue.User`, `*Repository.Owner`, etc. | Medium | Low | go-github returns pointers for nullable fields; the parser uses `safeStr`/`safeInt`/`safeBool`/`safeTime` helpers that nil-guard each field. Tests `TestParseDeletedUserNilSafe` + `TestParseNoLanguageNilSafe` cover. |
| Composite ID format (`fullname@sha:path`) for code hits collides with future format | Low | Low | NormalizedDoc.ID is opaque to consumers; only the registry uses it (and registry doesn't parse). |
| GitHub deprecates `/search/code` endpoint | Low | High | Out of v0.1 control. If deprecated, MoAI revisits Path A (Open Question §11.4) or moves to GraphQL (Open Question §11.5). Capabilities.Notes documents the v0.1 endpoint dependency. |
| 100-concurrent-request ceiling triggered by aggressive callers | Low | Low | SPEC-FAN-001 §6.5 caps at MaxParallel=8 by default. ADP-004 contributes at most 1 concurrent request per fanout dispatch. 100-concurrent ceiling is not a binding constraint at our scale. |

---

## 11. Open Questions

These are explicitly unresolved at SPEC-approval time. Each has a
recommended default and a one-line resolution owner. They do NOT block
SPEC approval.

1. **`DocTypeCode` enum addition** (in `pkg/types/capabilities.go`).
   **Recommended default**: NO in v0.1. Map code hits to `DocTypeRepo`
   and store `kind="code"` in Metadata. Adding an enum value is a
   breaking-ish change to the SDK boundary
   (`.moai/project/structure.md:160`).
   **Resolution owner**: SPEC-IDX-001 author may request when RRF tuning
   shows code hits and repo hits should weight differently.

2. **Code-content fetch follow-up** (a second
   `/repos/{owner}/{repo}/contents/{path}` call to populate `Body` for
   code-search hits). **Recommended default**: NO in v0.1. Doubles the
   rate-limit cost and inflates latency. Synthesis (SPEC-SYN-001)
   operates on Title + Snippet for v0.1.
   **Resolution owner**: SPEC-SYN-002 / SPEC-SYN-003 author.

3. **GitHub App authentication** (per-team installation key, 15000/hr
   Cloud limit). **Recommended default**: NEVER in v0.1. Defer to
   SPEC-AUTH-002 (M6).
   **Resolution owner**: SPEC-AUTH-002 author.

4. **Path A revisit** (wrap `github/github-mcp-server` once SPEC-MCP-001
   M7 lands). **Recommended default**: NO unless GitHub deprecates
   `/search/code` in favour of an MCP-only endpoint. The operational
   complexity of running two binaries (usearch + github-mcp-server)
   outweighs the abstraction benefit for a single source.
   **Resolution owner**: SPEC-MCP-001 author + future SPEC-ADP-004a.

5. **GraphQL v4 variant** (point-cost accounting, `pageInfo` cursors).
   **Recommended default**: NEVER in v0.1 or v1. GraphQL adds
   maintenance burden without measured benefit.
   **Resolution owner**: SPEC-IDX-001 / SPEC-IDX-002 author may revisit
   if multi-source RRF measurements show GitHub search throughput is
   bottleneck.

6. **Score synthesis from comments / stars**. Issues use
   `comments * 10` (10 comments → Tanh(1.0) ≈ 0.76); repos use raw
   stars. **Recommended default**: keep both in v0.1. SPEC-IDX-001 RRF
   re-ranks by rank not score across adapters.
   **Resolution owner**: SPEC-IDX-001 author.

7. **Per-intent rate-limit declaration**. `Capabilities.RateLimitPerMin`
   is a single int. GitHub's code search is 9/min while issue/repo
   search is 30/min. **Recommended default**: 30 (issue/repo cadence).
   Document the code-search 9/min in `Capabilities.Notes`. Adding a
   per-intent ceiling field requires amending `pkg/types.Capabilities`,
   out of v0.1 scope.
   **Resolution owner**: SPEC-FAN-001 author may upgrade to a
   per-intent ceiling field in a future SPEC.

8. **Retry-After cap (90s vs 60s)**. ADP-001/002 cap at 60s; ADP-004
   wants 90s for GitHub's secondary-rate-limit recovery semantics.
   **Recommended default**: hardcode 90s in v0.1. Make it
   Options-tunable in a future iteration if measured pain warrants.
   **Resolution owner**: run-phase implementer; revisit if SPEC-EVAL-002
   reliability dashboard shows GitHub adapter is timing out under
   sustained load.

---

## 12. References

### External (URL-cited; verified per research.md §9)

- https://github.com/google/go-github — v85.0.0 (April 2026); structured
  Go REST client; typed `*RateLimitError`, `*AbuseRateLimitError`,
  `*ErrorResponse`; page-based pagination; `WithAuthToken` PAT idiom.
- https://github.com/github/github-mcp-server — Path A; rejected for
  v0.1 (research §2.1).
- https://docs.github.com/en/rest — REST API v3 portal.
- https://docs.github.com/en/rest/search/search — Search endpoints
  (code, issues, repositories) with response shapes, query parameters,
  pagination.
- https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api
  — Primary 5000/hr authenticated; secondary 30/min for search; 9/min
  code-search; 100-concurrent ceiling; X-RateLimit-* headers.
- https://docs.github.com/en/graphql — GraphQL alternative (rejected for
  v0.1; Open Question §11.5).

### Internal (file:line cited)

- `.moai/specs/SPEC-ADP-004/research.md` — full research artifact (this
  SPEC's research sibling).
- `.moai/specs/SPEC-ADP-001/spec.md` — reference adapter SPEC; this
  SPEC inherits structure verbatim.
- `.moai/specs/SPEC-ADP-002/spec.md` — Hacker News adapter SPEC; second-
  adapter validation that the reference shape is portable.
- `.moai/specs/SPEC-CORE-001/spec.md` — Adapter / Capabilities (with
  RequiresAuth + AuthEnvVars fields) / Query / NormalizedDoc /
  SourceError contract.
- `.moai/specs/SPEC-OBS-001/spec.md` — observability bundle and
  cardinality discipline.
- `.moai/specs/SPEC-IR-001/spec.md` — `Capabilities` consumer contract
  (REQ-IR-008).
- `.moai/specs/SPEC-FAN-001/spec.md` — M3 fanout; consumer of this
  adapter via registry.
- `pkg/types/adapter.go:28-45` — Adapter interface 4-method shape.
- `pkg/types/capabilities.go:38-62` — Capabilities struct;
  `RequiresAuth` + `AuthEnvVars` at lines 51-55.
- `pkg/types/capabilities.go:14-23` — DocType enum (DocTypeRepo,
  DocTypeIssue available).
- `pkg/types/query.go:18-44` — Query struct + Filter shape.
- `pkg/types/errors.go:14-218` — SourceError, Category, sentinels,
  CategorizeError, OutcomeFromError, ValidationError.
- `pkg/types/normalized_doc.go:40-106` — NormalizedDoc 15-field struct,
  Validate, CanonicalHash.
- `internal/adapters/registry.go:75-167` — Registry lifecycle.
- `internal/adapters/registry.go:123-129` — AuthEnvVars validation path
  (first executed against a real adapter by ADP-004).
- `internal/adapters/registry.go:172-263` — wrappedAdapter sole-emitter
  pattern.
- `internal/adapters/noop/noop.go:1-46` — reference adapter shape.
- `internal/adapters/reddit/reddit.go:1-136` — Adapter struct pattern
  (mirrored by ADP-004 github.go).
- `internal/adapters/reddit/search.go:1-167` — Search hot path pattern.
- `internal/adapters/reddit/parse.go:1-203` — JSON → NormalizedDoc
  transform pattern.
- `internal/adapters/reddit/client.go:1-125` — HTTP client + redirect
  allowlist + categorizeStatus pattern.
- `internal/adapters/reddit/score.go:1-41` — Tanh score formula
  (duplicated verbatim by ADP-002 + ADP-004).
- `internal/adapters/reddit/errors.go:1-64` — parseRetryAfter helper +
  ErrInvalidQuery sentinel pattern (cap raised to 90s in ADP-004).
- `internal/adapters/hn/hn.go:1-139` — Second-adapter validation that
  the reference shape is portable.
- `internal/llm/client.go:31-65` — HTTP client construction pattern
  with timeout + reqid Transport wrapping (consumed by ADP-004
  newDefaultHTTPClient).
- `.moai/project/roadmap.md:49` — M3 row "SPEC-ADP-004 | GitHub adapter
  | wrap official `github/github-mcp-server`". Note: roadmap suggests
  Path A; this SPEC chose Path B and documents the deviation in §2.2 +
  Open Question §11.4 + research.md §2.
- `.moai/project/roadmap.md:113` — tech.md GitHub row.
- `.moai/project/structure.md:18-29` — `internal/adapters/github/`
  reservation.
- `.moai/project/structure.md:160` — `pkg/types` SDK boundary clause —
  reason `DocTypeCode` enum addition is deferred (Open Question §11.1).
- `.moai/project/tech.md:113` — Per-source adapter strategy GitHub row
  ("GitHub REST + Search API, PAT per team, 5000/hr with auth").
- `.moai/config/sections/quality.yaml` — `development_mode: tdd`,
  `test_coverage_target: 85`.
- `.moai/config/sections/harness.yaml` — auto-routing to standard
  level.
- `.moai/config/sections/language.yaml` — `documentation: en`,
  `code_comments: en`.
- `.claude/rules/moai/core/lsp-client.md` — pinning-policy reference for
  `github.com/google/go-github/v85` upgrades.
- `go.mod:5-9` — direct deps; `github.com/google/go-github/v85` MUST be
  added at SPEC-ADP-004 run phase.
- `go.mod:30` — `go.uber.org/goleak v1.3.0` indirect (used by
  NFR-ADP4-003).

---

*End of SPEC-ADP-004 v0.1 (DRAFT)*
