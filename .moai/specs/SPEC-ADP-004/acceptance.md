# SPEC-ADP-004 Acceptance Criteria (Post-Hoc)

**SPEC**: SPEC-ADP-004 — GitHub Adapter
**Status**: implemented (2026-05-07)
**Format**: Given/When/Then per REQ + edge cases + Definition of Done

---

## 1. Acceptance Scenarios by REQ

### REQ-ADP4-001 — Adapter Interface Conformance + Auth Declaration

**AC-001: Compile-time interface assertion**
- Given `var _ types.Adapter = (*Adapter)(nil)` in `github.go`,
- When `go build` runs,
- Then build succeeds.

**AC-002: Name returns "github"**
- `(*Adapter).Name() == "github"`.

**AC-003: Capabilities deterministic + shape-correct**
- Two consecutive `Capabilities()` calls return `reflect.DeepEqual`
  results with `SourceID="github"`, `DisplayName="GitHub"`,
  `DocTypes=[DocTypeRepo, DocTypeIssue]`, `SupportedLangs=nil`,
  `SupportsSince=true`, `RequiresAuth=true`,
  `AuthEnvVars=["USEARCH_GITHUB_TOKEN"]`, `RateLimitPerMin=30`,
  `DefaultMaxResults=25`, and `Notes` contains all 6 documented
  substrings.

**AC-004: RequiresAuth + AuthEnvVars declaration**
- `len(Capabilities().AuthEnvVars) == 1` AND
  `Capabilities().AuthEnvVars[0] == "USEARCH_GITHUB_TOKEN"` AND
  `Capabilities().RequiresAuth == true`.

**AC-005: Healthcheck succeeds**
- TCP dial to httptest.Server bound to 127.0.0.1:0 via
  `Options.HealthcheckTarget` → nil error.

**AC-006: New rejects empty token unless SkipAuthCheck**
- `New(Options{Token: "", SkipAuthCheck: false})` returns
  `*SourceError{Permanent, Cause: ErrMissingToken}`.

**AC-007: SkipAuthCheck allows empty token (test usage)**
- `New(Options{Token: "", SkipAuthCheck: true})` returns nil error.

### REQ-ADP4-002 — Multi-Intent Happy Path

**AC-008: Code intent**
- Stub returns `testdata/search_code_25.json`;
  `Filters=[{kind, "code"}]` → 25 NormalizedDocs; URL path observed
  at stub is `/search/code`.

**AC-009: Issues intent**
- Stub returns `testdata/search_issues_25.json`; `kind=issues` →
  25 NormalizedDocs; URL path `/search/issues`.

**AC-010: Repos intent**
- Stub returns `testdata/search_repos_25.json`; `kind=repos` →
  25 NormalizedDocs; URL path `/search/repositories`.

**AC-011: Default intent is repos**
- `Filters=nil` → URL path is `/search/repositories`.

**AC-012: per_page clamp and default**
- `MaxResults=500` → URL has `per_page=100`.
- `MaxResults=0` → URL has `per_page=25`.

**AC-013: Page cursor round-trip**
- `Cursor="3"` → URL has `page=3`.
- `Cursor=""` → URL has `page=1` (1-indexed).

### REQ-ADP4-003 — Rate-Limit Mapping

**AC-014: Primary rate limit**
- Stub returns 403 + `X-RateLimit-Remaining: 0` + `X-RateLimit-Reset:
  <future>` → `Category=CategoryRateLimited`, `HTTPStatus=403`,
  `RetryAfter > 0`, `RetryAfter ≤ 90s`.

**AC-015: Abuse rate limit**
- Stub returns abuse-detection body recognised by go-github as
  `*AbuseRateLimitError` → `Category=CategoryRateLimited`,
  `RetryAfter` parsed from the body's `Retry-After: <N>` directive.

**AC-016: RetryAfter capped at 90s**
- `Retry-After: 999` → `RetryAfter=90s`.

**AC-017: Negative/zero RetryAfter defaults to 5s**
- Synthesised case where `Rate.Reset` is in the past → `RetryAfter=5s`.

**AC-018: No internal retry on rate limit**
- Stub counts requests; exactly 1 outbound request observed.

### REQ-ADP4-004 — 4xx Permanent / 5xx Unavailable / Network

**AC-019: 4xx → Permanent**
- Table over 401/403 (non-abuse)/404/422 →
  `errors.Is(err, types.ErrPermanent)` AND matching HTTPStatus.

**AC-020: 5xx → Unavailable**
- Table over 500/503 →
  `errors.Is(err, types.ErrSourceUnavailable)` AND matching HTTPStatus.

**AC-021: Connection refused → Unavailable**
- httptest.Server closed before request →
  `errors.Is(err, types.ErrSourceUnavailable)` AND `HTTPStatus=0`.

**AC-022: Underlying error preserved**
- `errors.Unwrap(srcErr) != nil` AND inner error message contains
  "connection refused".

### REQ-ADP4-005 — NormalizedDoc Field Mapping (Per Intent)

**AC-023: Code field mapping table (5 fixtures)**
- Go file, Python file, TypeScript file, file with `Language=nil`,
  deleted-fork file → every NormalizedDoc field per §6.3.1.

**AC-024: Issue/PR field mapping (5 fixtures)**
- Open issue, closed issue, open PR, closed PR, deleted-author →
  `DocType=DocTypeIssue` always; `Metadata["is_pull_request"]`
  distinguishes.

**AC-025: Repo field mapping (5 fixtures)**
- Public popular, archived, fork, no-description, no-owner → every
  field per §6.3.3.

**AC-026: Nil User → Author=""**
- `search_issues_nil_user.json` → returned doc has `Author == ""`
  AND `Validate()` passes.

**AC-027: Nil Language → Metadata["language"]=""**
- `search_repos_no_language.json` → returned doc has
  `Metadata["language"] == ""` AND `Validate()` passes.

**AC-028: Pagination cursor**
- `*Response.NextPage = 2` → last doc `Metadata["next_cursor"] ==
  "2"`.

**AC-029: No cursor on last page**
- `*Response.NextPage = 0` → no doc has `next_cursor` key.

**AC-030: Hash always empty**
- Every returned `doc.Hash == ""`.

**AC-031: Per-intent Metadata keys**
- code: `{repo_full_name, path, sha, language, kind:"code"}`.
- issues: `{repo_full_name, number, state, is_pull_request,
  comments, kind ∈ {"issue","pr"}}`.
- repos: `{full_name, language, stars, forks, open_issues, kind:"repo"}`.

**AC-032: Malformed JSON → Permanent**
- Truncated JSON → `*SourceError{Permanent}`.

### REQ-ADP4-006 — User-Agent + Authorization Headers

**AC-033: Custom UA**
- Captured `User-Agent` starts with `"usearch/"` and contains
  `"(+https://github.com/elymas/universal-search)"`.

**AC-034: Authorization Bearer header**
- Captured `Authorization` header starts with `"Bearer "`; the
  bearer token equals the test fixture token.

**AC-035: UA version configurable**
- `Options.UserAgentVersion = "v0.2-rc1"` → UA contains
  `"usearch/v0.2-rc1"`.

**AC-036: Token NOT in error message**
- Stub forces 401 that echoes the Authorization header in the body;
  returned `*SourceError.Cause.Error()` does NOT contain the token
  substring.

**AC-037: Token NOT in slog output**
- Captured slog records during Search do NOT contain the token
  substring in any attribute key or value.

### REQ-ADP4-007 — Filter Qualifier Append (Optional)

**AC-038: since qualifier**
- `Filters=[{since, "2026-01-01T00:00:00Z"}]` → URL `q` parameter
  ends in `created:>=2026-01-01T00:00:00Z`.

**AC-039: language qualifier**
- `Filters=[{language, "go"}]` → URL `q` ends in `language:go`.

**AC-040: repo qualifier**
- `Filters=[{repo, "facebook/react"}]` → URL `q` ends in
  `repo:facebook/react`.

**AC-041: Multiple filters space-separated**
- Two filters → URL `q` contains both qualifiers separated by a
  single space.

**AC-042: Unknown filter ignored**
- `Filters=[{nsfw, "true"}]` → URL `q` unchanged; no error.

**AC-043: Malformed since dropped**
- `Filters=[{since, "not-a-date"}]` → URL `q` unchanged; no error.

**AC-044: Empty filter value dropped**
- `Filters=[{language, ""}]` → URL `q` unchanged.

**AC-045: Filter applicability per intent**
- `Filters=[{is_pr, "true"}]` with `kind=code` → ignored;
  with `kind=issues` → applied (`is:pr`).

### REQ-ADP4-008 — Empty Query / Invalid Cursor / Invalid Intent

**AC-046: Empty/whitespace Text rejected with zero HTTP**
- `Text` in `["", "   ", "\t\n  \r"]` →
  `*SourceError{Permanent, Cause: ErrInvalidQuery}`; ZERO requests.

**AC-047: Invalid cursor rejected with zero HTTP**
- `Cursor` in `["abc", "0", "-1", "1.5", "1e3"]` →
  `*SourceError{Permanent, Cause: ErrInvalidCursor}`; ZERO requests.
- Note: "0" rejected because v0.1 requires `Page ≥ 1` (1-indexed).

**AC-048: Invalid intent rejected with zero HTTP**
- `Filters=[{kind, "users"}]` →
  `*SourceError{Permanent, Cause: ErrInvalidIntent}`; ZERO requests.

### REQ-ADP4-009 — Redirect Allowlist (Optional)

**AC-049: Allowlist redirect followed**
- Server A 302 → server B (Host `api.github.com`) → docs from B.

**AC-050: Cross-domain redirect rejected**
- Server A 302 to `attacker.com` → `errors.Is(err,
  types.ErrPermanent)` AND error contains `"cross-domain redirect"`.

**AC-051: Chain over 3 hops rejected**
- 4-hop chain → error contains `"too many redirects"`.

### REQ-ADP4-010 — Concurrent Search Safety

**AC-052: 50 goroutines race-clean**
- 50 goroutines × 1 Search against shared `*Adapter` and one stub →
  no race alarms under `-race`; 50 requests observed; every
  goroutine receives 25 valid NormalizedDocs.

---

## 2. NFR Acceptance

### NFR-ADP4-001 — Parse-Path Performance

**AC-N01: Benchmark within target**
- `BenchmarkParseGitHubResponse25Results` median ≤ 5 ms;
  `allocs/op ≤ 625` (higher floor than ADP-001/002 due to
  `*Repository`'s 12+ nullable pointer fields).

### NFR-ADP4-002 — E2E p95 (Stub)

**AC-N02: p95 ≤ 200ms**
- 100 invocations against stub → `durations[94] ≤ 200ms`.

### NFR-ADP4-003 — No Goroutine Leak on Cancellation

**AC-N03: goleak verifies clean shutdown**
- `goleak.VerifyNone(t)` after mid-flight ctx cancel.
- `TestMain` invokes `goleak.VerifyTestMain(m)`.

### NFR-ADP4-004 — Resource Cleanup

**AC-N04: No leaked file descriptors**
- Process FD count delta over 100 Search calls ≤ 5.

---

## 3. Edge Cases

**EC-001: Composite code ID stability**
- Code hit ID = `fullname@sha:path`; same hit across pages produces
  identical ID.

**EC-002: PR vs issue distinction**
- `*Issue.PullRequestLinks != nil` → `Metadata["is_pull_request"]
  == true` AND `Metadata["kind"] == "pr"`.

**EC-003: 100-concurrent ceiling not binding**
- FAN-001 caps `MaxParallel=8`; ADP-004 contributes ≤1 per
  fanout dispatch.

**EC-004: go-github WithAuthToken idiom**
- `Authorization: Bearer <token>` set by go-github's client; not by
  the adapter directly.

---

## 4. Coverage Matrix

| REQ-ID | EARS Pattern | Acceptance Criteria | Implementation |
|--------|-------------|---------------------|----------------|
| REQ-ADP4-001 | Ubiquitous | AC-001..007 | `github.go`, `github_test.go` |
| REQ-ADP4-002 | Event-Driven | AC-008..013 | `search.go`, `search_test.go` |
| REQ-ADP4-003 | Event-Driven | AC-014..018 | `client.go::categorizeError`, `errors.go::parseRetryAfter` (90s cap) |
| REQ-ADP4-004 | Event-Driven | AC-019..022 | `client.go::categorizeError` |
| REQ-ADP4-005 | Ubiquitous | AC-023..032 | `parse.go::parseCodeResults` / `parseIssueResults` / `parseRepoResults` + `safe*` helpers |
| REQ-ADP4-006 | Ubiquitous | AC-033..037 | `client.go::doRequest` + go-github `WithAuthToken` |
| REQ-ADP4-007 | Optional | AC-038..045 | `search.go::appendQualifiers` |
| REQ-ADP4-008 | Unwanted | AC-046..048 | `search.go` (input validation) |
| REQ-ADP4-009 | Optional | AC-049..051 | `client.go::redirectAllowlist` |
| REQ-ADP4-010 | State-Driven | AC-052 | `search_test.go::TestSearchConcurrentSafe` |
| NFR-ADP4-001 | Performance | AC-N01 | `bench_test.go::BenchmarkParseGitHubResponse25Results` |
| NFR-ADP4-002 | Latency | AC-N02 | `search_test.go::TestSearchE2ELatencyStubP95` |
| NFR-ADP4-003 | Resource | AC-N03 | `search_test.go::TestSearchNoGoroutineLeakOnCancel` |
| NFR-ADP4-004 | Resource | AC-N04 | `search_test.go::TestSearchNoLeakedFileDescriptors` |

---

## 5. Definition of Done

- [x] All 10 EARS REQs have passing tests.
- [x] All 4 NFRs have passing measurements.
- [x] `go test ./internal/adapters/github/...` exits 0.
- [x] `go test -race ./internal/adapters/github/...` exits 0.
- [x] `go test -cover ./internal/adapters/github/...` reports ≥ 85%.
- [x] `go vet` and `golangci-lint run` clean.
- [x] `BenchmarkParseGitHubResponse25Results` median ≤ 5ms;
      allocs/op ≤ 625.
- [x] Token leak prevention tests pass.
- [x] No leaked file descriptors (≤5 delta over 100 calls).
- [x] MX tags applied per spec.md §6.8 plan.
- [x] Capabilities.Notes contains all 6 documented substrings.
- [x] `var _ types.Adapter = (*Adapter)(nil)` present.
- [x] `go.mod` updated with `github.com/google/go-github/v85` and
      transitive `github.com/google/go-querystring`.
- [x] No drive-by changes outside `internal/adapters/github/` and
      `go.mod`.
- [x] SPEC status updated to `implemented` (2026-05-07).

---

*End of SPEC-ADP-004 acceptance.md (post-hoc, v1.0)*
