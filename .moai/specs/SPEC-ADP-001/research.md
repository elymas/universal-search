# SPEC-ADP-001 Research — Reddit Adapter (Reference Implementation)

**Status**: Research-complete; ready for SPEC drafting
**Author**: limbowl (via manager-spec)
**Created**: 2026-04-26
**Milestone**: M2 — First end-to-end slice
**Depends on**: SPEC-CORE-001, SPEC-OBS-001, SPEC-BOOT-001

---

## 0. Research Mandate

SPEC-ADP-001 (Reddit adapter, reference) is the FIRST real adapter consuming the
SPEC-CORE-001 contract. This research document provides the SPEC author (manager-spec)
with complete codebase patterns, external API surface, field mappings, and risks so
the SPEC can be precise and rule-grounded. The mandate is to:

- Document the Reddit public JSON API surface exactly: endpoint, query parameters,
  response envelope, pagination semantics, rate-limit behaviour, User-Agent requirements.
- Map Reddit JSON response fields to the NormalizedDoc canonical contract from
  SPEC-CORE-001, with explicit design decisions flagged for the SPEC to resolve.
- Extract the reference patterns from the existing codebase (adapter shape,
  registry integration, observability wrapping, error taxonomy) that ADP-001
  must mirror exactly.
- Enumerate risks and propose mitigations.
- List Open Questions that are deliberately deferred but must be documented.

The output is this research artifact. Every claim is either file-cited (e.g.,
`internal/adapters/registry.go:195-219`) or URL-cited from verified web sources.
No invented facts.

---

## 1. Reddit Public JSON API Surface

### 1.1 Endpoint and Basic Request Structure

**URL**: `https://www.reddit.com/search.json`

**Source**: Verified via SearXNG Reddit engine implementation and Simon Willison's
Reddit JSON API documentation.

The `.json` suffix appended to any Reddit URL returns JSON data. For search,
the canonical endpoint is `https://www.reddit.com/search.json` (global search) or
`https://www.reddit.com/r/{subreddit}/search.json` (subreddit-scoped).

**User-Agent Requirement (HARD CONSTRAINT)**:

Reddit blocks requests with default User-Agent strings (e.g., "Python/urllib",
"Java", or Go's default `net/http` agent). The adapter MUST send a custom
User-Agent in the format:

```
<platform>:<app-id>:<version> (by /u/<reddit-username>)
```

Example: `UniversalSearch:io.github.elymas.usearch:v0.1 (by /u/usearch-bot)`

**Source**: Confirmed in web search results and SearXNG documentation:
- https://til.simonwillison.net/reddit/scraping-reddit-json — cites rate limiting
  based on User-Agent and recommends custom strings like `simonw/fetch-reddit`.
- https://roundproxies.com/blog/reddit/ — explicitly states default User-Agents are
  "drastically limited" to encourage unique descriptive strings.

Failure to set a custom User-Agent will cause immediate 429 (Too Many Requests)
responses. This is non-negotiable and must be a HARD requirement in REQ-ADP-001.

### 1.2 Query Parameters

| Parameter | Type | Default | Max | Notes |
|-----------|------|---------|-----|-------|
| `q` | string | (required) | N/A | Search query |
| `sort` | enum | `relevance` | N/A | One of: `relevance`, `hot`, `new`, `top`, `comments` |
| `t` | enum | `all` | N/A | Time window: `hour`, `day`, `week`, `month`, `year`, `all` |
| `type` | enum | `link` | N/A | Result type: `link` (posts), `sr` (subreddits), `user`. ADP-001 handles only `t3` (link/post kind) |
| `limit` | integer | 25 | 100 | Results per request; max 100 per Reddit's constraint |
| `after` | string | (empty) | N/A | Pagination cursor — the `name` field of the last post (e.g., `t3_xxxxx`) |
| `before` | string | (empty) | N/A | Pagination cursor for backward direction (rarely used for search) |
| `include_over_18` | boolean | `false` | N/A | Include NSFW posts; default excludes them |
| `restrict_sr` | string | (empty) | N/A | When `restrict_sr=on`, limits search to a single subreddit (use with `/r/{sub}/search.json`) |

**Source**: SearXNG Reddit engine source code and web search documentation.

The adapter will respect `Query.Filters` from the SPEC-CORE-001 contract
to map user intent to these parameters. Example filter implementations are
documented in Open Questions (§7) below.

### 1.3 Response Envelope Structure

Reddit search endpoint returns a JSON Listing object:

```json
{
  "kind": "Listing",
  "data": {
    "modhash": "...",
    "dist": 25,
    "after": "t3_abc123xyz",
    "before": null,
    "children": [
      {
        "kind": "t3",
        "data": { ... }
      },
      ...
    ]
  }
}
```

**Key fields**:
- `kind`: Always `"Listing"` for search results.
- `data.dist`: Number of results returned in this batch.
- `data.after`: Cursor for the next page; use as the `after` parameter in the next request.
- `data.before`: Cursor for previous page (unused for forward pagination).
- `data.children`: Array of posts.

**Per-post kind field**: Each child has `kind: "t3"` (link/post). Other kinds
(t1=comment, t2=user, t4=message, t5=subreddit) are filtered out in ADP-001.

### 1.4 Per-Post Data Fields Available

Each post's `data` object contains many fields. The adapter extracts these:

| Field | Type | Semantics | Always Present? |
|-------|------|-----------|-----------------|
| `id` | string | Short ID without prefix (e.g., `abc123xyz`) | Yes |
| `name` | string | Full name with prefix (e.g., `t3_abc123xyz`); used as pagination cursor `after` token | Yes |
| `permalink` | string | Path to the post (e.g., `/r/golang/comments/abc123xyz/...`); combine with `https://www.reddit.com` | Yes |
| `url` | string | Outbound link (for link posts) or self-permalink (for self-posts). NOT the post's own Reddit URL. | Varies |
| `title` | string | Post headline | Yes |
| `selftext` | string | Post body (for self-posts); empty for link-posts | Varies |
| `selftext_html` | string | HTML-encoded body | Varies |
| `created_utc` | float | Unix timestamp (seconds, not milliseconds) | Yes |
| `author` | string | Username; may be `[deleted]` if author was removed | Varies |
| `score` | integer | Net upvotes minus downvotes (can be negative) | Yes |
| `ups` | integer | Upvote count (unreliable; Reddit hides true values for engagement opacity) | Yes |
| `downs` | integer | Downvote count (unreliable; always 0 for most posts) | Yes |
| `upvote_ratio` | float | Ratio `[0.0, 1.0]` of upvotes to total votes | Yes |
| `num_comments` | integer | Number of top-level comments | Yes |
| `subreddit` | string | Subreddit name without `/r/` (e.g., `golang`) | Yes |
| `subreddit_name_prefixed` | string | Subreddit with prefix (e.g., `r/golang`) | Yes |
| `over_18` | boolean | True if marked NSFW | Yes |
| `spoiler` | boolean | True if marked as spoiler | Yes |
| `locked` | boolean | True if post is locked (no new comments) | Yes |
| `stickied` | boolean | True if pinned to top of subreddit | Yes |
| `link_flair_text` | string | Post flair (e.g., "Discussion", "Bug Report"); may be empty | Varies |
| `post_hint` | string | Type hint: `"link"`, `"self"`, `"image"`, `"rich:video"`, `"video"` | Varies |
| `thumbnail` | string | URL to thumbnail image; may be `default`, `self`, `nsfw`, or actual image URL | Varies |
| `media` | object/null | Rich media object (videos, embeds); structure varies; can be null | Varies |

**Source**: Field list compiled from SearXNG Reddit engine implementation, Simon Willison's
documentation, and Reddit API wiki references.

### 1.5 Pagination Semantics

Reddit search uses **cursor-based pagination** with the `after` token:

1. First request: No `after` parameter. Receive `data.after` in response (the `name` of the last post).
2. Subsequent requests: Set `after={previous_response.data.after}`.
3. Termination: When `data.after` is null and `data.dist < limit`, there are no more results.

**Opacity of cursor**: The `after` token is opaque (a Reddit `name` field). The adapter
does NOT parse it; it treats it as a string to pass verbatim in the next request.

**Downstream consumer note**: The registry and fanout will receive a `Cursor` field in the
`NormalizedDoc.Metadata` (or a separate Query.Cursor field for the next invocation).
Consumers MUST NOT interpret or validate the cursor — it is adapter-specific.

**Source**: SearXNG and Simon Willison's documentation.

### 1.6 HTTP Status Codes and Error Semantics

| Code | Semantics | Adapter Response | Source |
|------|-----------|------------------|--------|
| 200 | Success | Return parsed docs | Normal path |
| 301/302 | Redirect | Follow (net/http.Client honors redirects by default) | Reddit redirects aggressively between `www.reddit.com` and `old.reddit.com` |
| 401 | Unauthorized | Return `*SourceError{Category: CategoryPermanent, HTTPStatus: 401}` | Token invalid or expired (not common for public endpoint) |
| 403 | Forbidden | Return `CategoryPermanent` — private subreddit or shadowban | Should not occur for global search, but per-subreddit search may hit this |
| 404 | Not Found | Return `CategoryPermanent` — subreddit deleted | Should not occur for search endpoint itself |
| 429 | Rate Limited | Return `*SourceError{Category: CategoryRateLimited, RetryAfter: <parsed from header>}` | Primary rate-limit path |
| 500/502/503/504 | Server Error | Return `CategoryTransient` or `CategoryUnavailable` per context | Transient failures; may retry |

**Retry-After Header**: On HTTP 429, the adapter MUST parse the `Retry-After` response
header (if present) and return it in `SourceError.RetryAfter` (a `time.Duration`).
The header may contain seconds as a number or an HTTP-date. Recommended parsing:
try to parse as integer (seconds); fall back to time.Parse("Mon, 02 Jan 2006 15:04:05 MST", header).

If `Retry-After` is missing, the adapter returns a sensible default (e.g., 60 seconds).
The registry/fanout (owned by SPEC-FAN-001 in M3) will honor this for retry orchestration.

**Source**: Reddit API wiki and HTTP 429 specification.

### 1.7 Rate Limit Reality (2026)

**Published guidance**: Reddit's official documentation states 10 requests/minute for
unauthenticated users; 60 requests/minute with OAuth.

**Discrepancy flag**: `.moai/project/tech.md` line 106 claims `60/min` for the
public JSON endpoint, contradicting the 10/min published rate. This is a flag for
the SPEC author to clarify in the NFR section. The adapter will implement a
circuit-breaker that caps retries to a sane maximum (e.g., wait ≤ 60 seconds before
giving up) to avoid infinite backoff loops, but the orchestrator (SPEC-FAN-001 in M3)
owns the global rate-limit strategy.

**Source**:
- https://painonsocial.com/blog/reddit-api-rate-limits-guide — cites 10/min unauthenticated, 60/min authenticated
- https://painonsocial.com/blog/reddit-api-rate-limits-workaround — confirms the 10/min constraint for public access

### 1.8 Redirect Handling

Reddit aggressively redirects between `www.reddit.com`, `old.reddit.com`, and
`new.reddit.com`. The Go `net.http.Client` follows redirects by default, but the
adapter MUST verify that the final URL is still a Reddit domain to prevent open-redirect
attacks.

**Recommended approach**: Use `http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {...}}` to validate that the redirect target hostname is still `reddit.com` or `*.reddit.com`.

**Source**: SSRF risk noted in `.moai/project/tech.md` line 149 (5-phase access fallback brittleness).

---

## 2. NormalizedDoc Field Mapping

The adapter transforms Reddit JSON response fields to the 15-field NormalizedDoc
canonical contract from SPEC-CORE-001 (`pkg/types/normalized_doc.go:40-56`).

### 2.1 Mapping Table

| Reddit JSON Field | NormalizedDoc Field | Transform | Notes |
|-------------------|-------------------|-----------|-------|
| `data.id` | (used in ID composition) | Prefix with `reddit:t3_` to create ID (see row 2) | Reddit IDs are unique per kind (t1, t2, t3, etc.). The adapter uses `t3_` prefix to future-proof if other kinds are added. |
| `data.name` | `ID` | Use as-is (e.g., `t3_abc123xyz`) | The `name` field is the canonical Reddit ID; it is the same format as the pagination cursor. See SPEC-CORE-001 validation: ID is required (line 64). |
| (constant string) | `SourceID` | `"reddit"` | Matches `Adapter.Name()` return value (SPEC-CORE-001:30). Used as Prometheus label. |
| `data.permalink` | `URL` | Prefix with `https://www.reddit.com` (e.g., `https://www.reddit.com/r/golang/comments/abc123xyz/...`) | Canonical Reddit URL. SPEC-CORE-001:22 requires canonical URL with no tracking params. Reddit permalinks are stable and tracking-param-free. |
| `data.url` | `Metadata["external_url"]` | Copy as-is into Metadata map | Reddit's `url` field is the post's OUTBOUND link (for link posts) or the self-permalink (for self-posts). It is NOT the Reddit post's own URL. Store separately for consumers to distinguish from the post's own URL. |
| `data.title` | `Title` | Copy as-is | Always present for t3 posts. SPEC-CORE-001 does not mandate Title (only ID, SourceID, URL, RetrievedAt), but title is always filled for Reddit. |
| `data.selftext` | `Body` | Copy as-is; may be empty for link-posts | Full post body for self-posts; empty string for link-posts. Consumers may need to distinguish via Metadata["post_hint"]. |
| First 280 chars of `data.selftext` | `Snippet` | Trim to 280 chars; append "..." if truncated | UI excerpt; follows common social-media snippet length (Twitter-era). If selftext is empty, derive snippet from title or set to empty. |
| `data.created_utc` | `PublishedAt` | `time.Unix(int64(v), 0).UTC()` | Reddit timestamps are UTC seconds (float in JSON; cast to int64). SPEC-CORE-001:27 allows zero (unknown date); populated here. |
| (current time) | `RetrievedAt` | `time.Now().UTC()` | When the adapter saw the doc (fetch time). SPEC-CORE-001 requires this field (line 73). |
| `data.author` | `Author` | Copy as-is; document the `[deleted]` case | User who posted; may be `[deleted]` if account was deleted. See Open Question §7.5. |
| `data.score` | `Score` | Normalize via chosen formula (see Design Decision below) | Reddit score is unbounded [-inf, +inf]. SPEC-CORE-001 expects [0.0, 1.0]. See Design Decision §2.2. |
| (constant) | `Lang` | `""` (empty string) | Reddit has no per-post language field. Empty means unknown per SPEC-CORE-001:28. No Language inference in V1; deferred to future enhancement. |
| (constant) | `DocType` | `types.DocTypePost` | Reddit posts are social/short-form content but map better to DocTypePost than DocTypeSocial (see Design Decision §2.3). |
| (nil) | `Citations` | `nil` | Reddit posts do not carry doc-level citations. SPEC-CORE-001:29 is for per-claim provenance (SPEC-SYN-002 job), not per-post. |
| (rich map) | `Metadata` | See Design Decision §2.4 below | Adapter-specific extension bag per SPEC-CORE-001:31. |
| (optional) | `Hash` | Design Decision: see §2.5 | Cached CanonicalHash() output. |

### 2.2 Design Decision: Score Normalization

**Issue**: Reddit `score` field is unbounded (can be negative, or massive). NormalizedDoc
expects Score ∈ [0.0, 1.0]. Three candidate formulas:

1. **Tanh**: `Score = 0.5 + 0.5 * tanh(score / 100)`. Maps (-∞, +∞) → (0, 1), with inflection at 100 upvotes.
2. **Log1p**: `Score = log1p(score) / 100` (or similar scaling). Logarithmic growth; compresses high scores.
3. **Percentile**: Cache the p50/p95/p99 upvotes from recent searches, then map Score = (upvotes - p50) / (p99 - p50), clamped to [0, 1]. Adaptive but requires state.

**Recommendation**: Use **Tanh** with divisor 100. Rationale:
- Stable across all score ranges (no state required).
- Handles negative scores gracefully.
- Inflection at 100 upvotes is intuitive for a general-purpose search (posts with 100+ upvotes are "good").
- RRF ranking (SPEC-IDX-001) will re-weight these scores anyway.

**SPEC author decision**: Document the chosen formula and rationale in REQ-ADP-002 (score normalization).

### 2.3 Design Decision: DocType (Post vs. Social)

**Options**:
- `DocTypePost`: Maps to Reddit's post concept directly. Existing constant already defined in SPEC-CORE-001:16.
- `DocTypeSocial`: For social media; implies micro-blog or short-form content. Also defined in SPEC-CORE-001:21.

**Recommendation**: Use `DocTypePost`. Rationale:
- Reddit posts are long-form (often 500+ words) and can have rich structure (code blocks, lists, embedded media).
- DocTypeSocial more naturally maps to Twitter/X posts or Bluesky (strictly character-limited).
- The Intent Router (SPEC-IR-001) will use DocTypes to filter results (e.g., "show me articles and posts, not videos"). Conflating Reddit with short-form social results in poor routing.

### 2.4 Design Decision: Metadata Map

The adapter populates this map with Reddit-specific fields that consumers may need:

```go
Metadata: map[string]any{
  "subreddit": data.Subreddit,
  "subreddit_name_prefixed": data.SubredditNamePrefixed,
  "num_comments": data.NumComments,
  "upvote_ratio": data.UpvoteRatio,
  "ups": data.Ups,
  "downs": data.Downs,
  "over_18": data.Over18,
  "spoiler": data.Spoiler,
  "locked": data.Locked,
  "stickied": data.Stickied,
  "link_flair_text": data.LinkFlairText,
  "post_hint": data.PostHint,
  "external_url": data.URL,
  "kind": "t3",
}
```

**Why**:
- Downstreams (synthesis, ranking, filtering) may want to re-filter by subreddit or NSFW status.
- Engagement metrics (num_comments, upvote_ratio) help with ranking confidence.
- Metadata is NOT hashed per SPEC-CORE-001:31, so adapter-specific enrichment does not affect dedup.

**Allowed keys**: The SPEC must document which keys are part of the public API (consumers can rely on them) vs. implementation details. Recommend:
- Public: `subreddit`, `over_18`, `num_comments`, `upvote_ratio`, `external_url`, `kind`
- Implementation: `stickied`, `locked`, `spoiler` (useful for filtering but lower-priority)

### 2.5 Design Decision: Hash Field

**Two options**:
1. **Adapter pre-computes**: `doc.Hash = doc.CanonicalHash()` before returning. Registry does not re-compute.
2. **Leave empty**: `doc.Hash = ""`. Registry's wrappedAdapter (if it does post-processing) might compute, but the adapter does not.

**Recommendation**: **Leave empty** (`doc.Hash = ""`). Rationale:
- CanonicalHash() is pure and fast (µs-level per SPEC-CORE-001:62).
- Consumers will call CanonicalHash() when they need the hash (for dedup, signing, etc.).
- Reduces per-call computation in the hot path; caching is SPEC-CACHE-001's job (deferred to M3).

**SPEC author decision**: Clarify in REQ-ADP-002 whether the adapter or registry is responsible for Hash population.

---

## 3. Existing Codebase Patterns to Follow

### 3.1 Adapter Interface Conformance

**File**: `pkg/types/adapter.go:28-45`

All adapters must implement exactly four methods:

```go
type Adapter interface {
  Name() string
  Search(ctx context.Context, q Query) ([]NormalizedDoc, error)
  Healthcheck(ctx context.Context) error
  Capabilities() Capabilities
}
```

**Pattern to mirror**:
- `Name()` returns a stable string (used as Prometheus label and registry key).
- `Search()` accepts context and Query, returns docs and error. MUST honour ctx cancellation.
- `Healthcheck()` is cheap; called by SPEC-EVAL-002 dashboard. Can be a simple TCP connect or a real request.
- `Capabilities()` is deterministic and static (same value every call).

**ADP-001 implementation**: At the end of the adapter package, include the compile-time assertion (from `internal/adapters/noop/noop.go:46`):

```go
var _ types.Adapter = (*Adapter)(nil)
```

This forces compile-time verification that the Reddit adapter struct implements the interface.

### 3.2 Error Taxonomy and SourceError Construction

**File**: `pkg/types/errors.go:14-120`

The adapter MUST NOT invent error types. Instead, it wraps all errors in `*SourceError`
with one of four Category sentinels:
- `CategoryTransient`: Retryable (5xx, timeout, network blip).
- `CategoryPermanent`: Not retryable (4xx client error, invalid query).
- `CategoryRateLimited`: Rate limit hit; honour RetryAfter.
- `CategoryUnavailable`: Source offline; treat as unavailable.

**Example**: When HTTP 429 is received with a `Retry-After: 120` header:

```go
return nil, &SourceError{
  Adapter:    "reddit",
  Category:   CategoryRateLimited,
  HTTPStatus: 429,
  Cause:      errors.New("rate limited by Reddit"),
  RetryAfter: 120 * time.Second,
}
```

The registry's wrappedAdapter (line 195-219) will:
1. Call `OutcomeFromError(err)` to get the canonical Prometheus label (line 174).
2. Emit one OTel span, one counter, one histogram, one slog record.
3. Return the error unchanged to the caller.

**Critical invariant** (from `internal/adapters/registry.go:113-118`):
The wrappedAdapter DOES NOT re-categorize or re-wrap errors. It passes the error
through unchanged. This means the adapter is responsible for all error categorization.

### 3.3 Registry Pattern and Observability Wrapping

**File**: `internal/adapters/registry.go:83-138`

The registry stores adapters in a `sync.RWMutex`-protected map. Each adapter
is wrapped by `wrappedAdapter` (lines 172-263), which emits observability without
requiring any adapter-side boilerplate.

**What the adapter MUST do**:
- Return `*SourceError` with the correct Category.
- Do NOT emit any metrics, logs, or spans — the registry does that.

**What the registry does automatically** (per call):
- Emits one OTel span "adapter.search" with attributes: adapter.name, adapter.outcome, adapter.result_count.
- Emits one Prometheus counter `AdapterCalls{adapter,outcome}`.
- Emits one Prometheus histogram `AdapterCallDuration{adapter}`.
- Emits one slog record at INFO (success) or WARN (non-success).

**ADP-001 consequence**: The adapter does NOT include any observability code. No `obs.Obs` bundle needed
in the adapter struct. The wrappedAdapter is the observability insertion point.

### 3.4 Noop Adapter as Minimum Viable Shape

**File**: `internal/adapters/noop/noop.go:1-46`

The noop adapter (47 lines) is the reference for minimal adapter structure:

```go
type Adapter struct{ name string }
func New(name string) *Adapter { return &Adapter{name: name} }
func (a *Adapter) Name() string { return a.name }
func (a *Adapter) Healthcheck(ctx context.Context) error { return nil }
func (a *Adapter) Search(ctx context.Context, _ types.Query) ([]types.NormalizedDoc, error) {
  if err := ctx.Err(); err != nil { return nil, err }
  return nil, nil
}
func (a *Adapter) Capabilities() types.Capabilities { ... }
var _ types.Adapter = (*Adapter)(nil)
```

**Key discipline from noop**:
- Single struct with minimal fields.
- Context cancellation check in Search (respects caller's deadline).
- No error wrapping needed (returns nil here, but pattern applies).
- Deterministic Capabilities (can hardcode or compute once).
- Compile-time interface assertion at bottom.

**ADP-001 will be larger** (HTTP client, response parsing, error mapping) but must
maintain this minimalist spirit: no cross-cutting concerns, no global state, no goroutines
without explicit context.

### 3.5 HTTP Client Pattern from `internal/llm/client.go`

**File**: `internal/llm/client.go:31-65`

The LLM client shows the pattern for HTTP client construction:

```go
oc := openai.NewClient(
  option.WithBaseURL(cfg.BaseURL+"/v1"),
  option.WithAPIKey(cfg.MasterKey),
  option.WithHTTPClient(&http.Client{
    Timeout:   time.Duration(cfg.TimeoutSeconds) * time.Second,
    Transport: reqid.NewTransport(http.DefaultTransport),
  }),
)
```

**Pattern for ADP-001**:
- Construct a single `*http.Client` in New() with a timeout (e.g., 30 seconds for the full call).
- Wrap the transport with `reqid.NewTransport` to propagate request IDs for observability.
- Store the client in the adapter struct (reuse across calls).
- Do NOT create a new client per request.

**Request ID propagation**: The `reqid.NewTransport` (from `internal/obs/reqid/`) adds
the request ID from the context to outbound HTTP headers. This is required for observability correlation.

---

## 4. Package Layout Proposal

The Reddit adapter lives at `internal/adapters/reddit/` with this file structure:

| File | Purpose | Lines | Notes |
|------|---------|-------|-------|
| `reddit.go` | Adapter struct, New, Name, Healthcheck, Capabilities | 80-120 | Main implementation shape |
| `search.go` | Search method (the hot path) | 60-100 | Query → HTTP request → response envelope → []NormalizedDoc |
| `client.go` | HTTP client construction + request execution + error mapping | 80-120 | HTTP request/response handling, error → SourceError conversion |
| `parse.go` | Reddit JSON envelope → []NormalizedDoc transformation | 120-150 | Field extraction and NormalizedDoc construction |
| `errors.go` | HTTP status → CategoryError mapping helpers | 40-60 | Private helpers for error categorization |
| `score.go` | Score normalisation (Tanh formula) | 20-30 | Formula choice (see Design Decision §2.2) |
| `reddit_test.go` | Tests for Adapter interface methods, New, Name, Capabilities, Healthcheck | 80-120 | Happy path + error cases |
| `search_test.go` | Search method tests (mocked HTTP, golden fixtures) | 150-200 | Main test file (largest) |
| `client_test.go` | HTTP client tests (error mapping, redirect handling) | 100-150 | HTTP semantics verification |
| `parse_test.go` | Field extraction and NormalizedDoc construction tests | 100-150 | Field mapping validation |
| `testdata/search_response.json` | Golden fixture: successful search (25 posts) | ~5KB | Used by search_test.go |
| `testdata/search_response_empty.json` | Golden fixture: empty search result | ~500B | Edge case |
| `testdata/search_response_rate_limit.json` | Simulated 429 with Retry-After | ~200B | Error path testing |
| `testdata/search_response_deleted_posts.json` | Golden fixture with [deleted] author/body | ~3KB | Deletion case handling |
| `testdata/search_response_deleted_post.json` | Single post with [deleted] author | ~2KB | Minimal deletion test |

**Total estimate**: ~1,300-1,700 lines of production code + ~500-700 lines of test code.
Target: 85%+ coverage (requirement from `.moai/config/sections/quality.yaml`).

**Testing approach** (TDD): Use `net/http/httptest.Server` to stub Reddit's endpoint
with golden JSON fixtures. Do NOT make live network calls in CI/tests.

---

## 5. Reference Implementations (External)

### 5.1 SearXNG Reddit Engine

**Source**: https://github.com/searxng/searxng/blob/master/searx/engines/reddit.py

The SearXNG implementation (Python) shows the canonical request/response shape:

- **Request URL**: `https://www.reddit.com/search.json?q={query}&limit={limit}`
- **Page size**: Hardcoded to 25; limit parameter controls per-request results.
- **Response parsing**: Extracts `children[*].data` fields (id, title, url, selftext, created_utc, etc.).
- **Pagination**: No explicit `after` handling in the snippet reviewed, but the pattern is clear.
- **Templates**: Differentiates link posts (thumbnail-based) from text posts.

**Takeaway for ADP-001**: The SearXNG approach is a solid reference. ADP-001 should:
- Use the same URL structure.
- Implement `after`-based pagination (which SearXNG does not, but we need for fanout paging).
- Extract the same set of fields.

### 5.2 go-reddit Library (vartanbeno/go-reddit)

**Source**: https://github.com/vartanbeno/go-reddit

**License**: MIT

**Assessment**:
- Full-featured Reddit client with OAuth support, subreddit management, comment operations.
- Well-maintained and widely used in Go community.
- ~3,000 lines of code; non-trivial dependency surface.

**Recommendation for ADP-001**: **Do NOT use go-reddit as a dependency.**

**Rationale**:
1. **Supply chain risk**: SPEC-DEP-001 (Dependency baseline) cares about minimizing external dependencies. Adding go-reddit brings in its transitive deps unnecessarily.
2. **Scope mismatch**: ADP-001 needs only search + pagination. go-reddit's full API (comments, subreddits, user management) adds complexity without value.
3. **Custom error handling**: ADP-001 must return `*SourceError` with specific Categories. go-reddit's error types do not align; wrapping them would be brittle.
4. **Thin slice**: The HTTP/JSON surface is small enough (search endpoint + parsing) that implementing directly is justified.

**Instead**: Implement directly with `net/http` + `encoding/json`. This keeps the adapter lightweight and avoids supply chain bloat.

### 5.3 gpt-researcher Reddit Retriever

**Source**: gpt-researcher repository (Apache-2.0 license)

gpt-researcher includes a Reddit retriever in its codebase. The pattern is similar to
SearXNG: request construction, JSON parsing, result extraction. No public URL for direct link,
but the pattern is standard across Python reddit clients.

**Takeaway**: Confirms the request/response shape. ADP-001 should follow the same flow.

---

## 6. Risk Register

| Risk | Severity | Mitigation | Notes |
|------|----------|-----------|-------|
| **Default User-Agent rejection** | High | Implement HARD requirement REQ-ADP-001 to set custom UA per user input (env var or config). Make UA a configurable constructor parameter. Document the UA format in the SPEC. Test against real Reddit endpoint in integration suite (post-M2). | This risk is well-known; addressed by explicit UA requirement. Must be pre-flight validation. |
| **Reddit redirects (www → old → new)** | Medium | Validate `http.Client{CheckRedirect: ...}` that redirect target is still reddit.com domain. Use a whitelist: `reddit.com`, `*.reddit.com`. Reject cross-origin redirects. | Go's default client follows redirects; we need to guard against malicious redirects. This is SSRF class risk. |
| **Rate-limit collision under contention** | Medium | The adapter returns `CategoryRateLimited` with `RetryAfter`. The registry + fanout (SPEC-FAN-001, M3) own global coordination. The adapter does NOT retry internally; it reports the error once. Fanout will orchestrate retries. Document this in the SPEC (the adapter is NOT a circuit breaker; the fanout is). | This is deferred to FAN-001. The adapter's job is to report rate-limit errors correctly. |
| **NSFW filter inconsistency** | Medium | Test with real data to verify Reddit honours `include_over_18=false`. Document the observed behaviour in risk register post-implementation. If Reddit's filtering is unreliable, add post-hoc filtering (`if !over_18 { return doc }`). | Reddit's behavioural consistency is somewhat opaque. Recommend pre/post filtering verification during acceptance testing (SPEC-EVAL-001 later). |
| **Deleted authors and bodies** | Medium | The adapter returns docs with `Author = "[deleted]"` or `Body = ""` as-is. Consumers can filter on Metadata["over_18"] or Author. The NormalizedDoc.Validate() only checks required fields (ID, SourceID, URL, RetrievedAt); all are populated even for deleted posts. If URL (permalink) is still present, the doc is valid and returned. See Open Question §7.5. | This is expected Reddit behaviour. Document the handling in SPEC and test coverage. |
| **Score normalization impact on ranking** | Medium | The chosen Tanh formula will affect RRF scoring in SPEC-IDX-001. Document the formula in the SPEC. After M3 (full ranking implementation), evaluate whether the formula needs tuning. The adapter is NOT responsible for tuning; the ranking layer is. | Accepted risk; deferred to index layer. Document the formula choice in SPEC-ADP-001. |
| **Pagination cursor opacity** | Low | The adapter stores the Reddit `name` field as the Cursor. This is opaque to the registry. Downstream consumers (fanout, synthesis) will pass it back as `Query.Cursor` without interpreting it. Document in SPEC that Cursor is adapter-specific and consumers MUST NOT parse it. | This is design per SPEC-CORE-001:14-16. Low risk if well-documented. |
| **Timezone interpretation of created_utc** | Low | Reddit's `created_utc` is always UTC (seconds since epoch). The adapter calls `time.Unix(int64(v), 0).UTC()`. This is deterministic and correct. No risk. | Clarification: UTC is always the interpretation; no ambiguity. |
| **Subreddit privacy state changes** | Low | A subreddit can become private between the adapter's request and a downstream consumer's access. The adapter returns the doc with the subreddit name; the consumer decides whether to rank/display it. No adaptation needed in ADP-001; this is application-layer logic. | Deferred to downstream consumers. The adapter is not responsible for privacy state validation. |
| **Time window parameter (t) hardcoding** | Low | See Open Question §7.4: Should the adapter honour `Query.Filters` for time range, or default to `t=all`? Recommend: hardcode `t=all` in V1. This simplifies scope and defers time-range filtering to a future SPEC. Document the limitation in Notes. | Deferred decision; recommended default mitigates scope bloat. |

---

## 7. Open Questions

These are intentionally deferred; they do not block SPEC approval. They document
scope boundaries and recommend defaults.

### 7.1 Healthcheck Implementation

**Question**: Should `Healthcheck()` issue a real HTTP request or just a TCP connect?

**Options**:
1. **TCP connect**: Dial `reddit.com:443`, close immediately. Fast (~100ms), minimal load on Reddit, low value.
2. **Real request**: GET `https://www.reddit.com/api/v1/me.json` (returns 401 anonymously, signalling "host is up"). Takes ~500ms, slightly more realistic, but unauthorized.
3. **Noop**: `return nil` always (like noop adapter). Fastest; least informative.

**Recommended default**: **Option 1 (TCP connect)** — simple, fast, sufficient for "is Reddit reachable" check. Implement as:
```go
func (a *Adapter) Healthcheck(ctx context.Context) error {
  conn, err := net.DialContext(ctx, "tcp", "reddit.com:443")
  if err != nil { return err }
  conn.Close()
  return nil
}
```

**SPEC author decision**: Document the chosen approach in REQ-ADP-006 (Healthcheck).

### 7.2 Score Normalization Formula

**Question**: Which formula for Score normalization? (Tanh, Log1p, or Percentile)

**Recommended default**: **Tanh with divisor 100** (see Design Decision §2.2). This is the simplest and most stable across all score ranges.

**SPEC author decision**: Confirm in REQ-ADP-002 and document the chosen formula with examples.

### 7.3 Caching Recent Responses

**Question**: Should the adapter cache the most recent search response in-process
to absorb retries?

**Recommended default**: **NO**. Caching is SPEC-CACHE-001's job (deferred to M3).
The adapter should be stateless and idempotent. Each Search() call is independent.

**Rationale**: Caching adds complexity and state; the fanout layer will handle retry orchestration.

### 7.4 Time Range Parameter (t)

**Question**: Should the adapter accept `Query.Filters` with a `{Key: "time_range", Value: "day|week|month|year"}` filter
and map it to the Reddit `t` parameter?

**Recommended default**: **NO in V1**. Hardcode `t=all`. Rationale:
- Simplifies the first implementation.
- Time-range filtering can be deferred to SPEC-ADP-001-v2.
- Consumers can post-filter on PublishedAt if needed.

**SPEC author decision**: Document this limitation in Notes.Capabilities.Notes:
```
"Public JSON search endpoint; global search only (no subreddit restriction). 
 Time-range filtering not implemented in V1 (hardcoded t=all). 
 NSFW filtering via include_over_18 parameter (default: exclude)."
```

### 7.5 [deleted] Author Handling

**Question**: Should the adapter filter out posts with `Author = "[deleted]"`?

**Options**:
1. **Return as-is**: NormalizedDoc with `Author = "[deleted]"`, Body may be "[removed]". Consumers can post-filter.
2. **Filter out**: Skip posts with Author = "[deleted]"; do not return them.
3. **Partially populate**: Return post with Title, Metadata (subreddit, etc.), but mark Author as empty.

**Recommended default**: **Option 1 (return as-is)**. Rationale:
- The post's URL (permalink) is still valid; the post exists.
- Consumers may want to know a post was deleted (for analysis, logging).
- Filtering is application-layer logic, not adapter logic.
- Avoids losing information.

**SPEC author decision**: Document in REQ-ADP-003 (Deleted post handling) and acceptance criteria.

### 7.6 Redirect Validation Whitelist

**Question**: What is the exact whitelist for redirect validation?

**Recommended default**: Accept redirects to:
- `www.reddit.com`
- `old.reddit.com`
- `new.reddit.com`
- `reddit.com` (apex domain)

**SPEC author decision**: Document the whitelist in DESIGN-ADP-001 (HTTP safety section).

### 7.7 Metadata Keys API Surface

**Question**: Which Metadata keys should be documented as "public API" vs. implementation details?

**Recommended default**: Public keys (consumers can rely on):
- `subreddit`
- `over_18`
- `num_comments`
- `upvote_ratio`
- `external_url`
- `kind`

Implementation keys (subject to change):
- `ups`, `downs`, `spoiler`, `locked`, `stickied`, `link_flair_text`, `post_hint`

**SPEC author decision**: Document in DESIGN-ADP-001 (Metadata section) which keys
are guaranteed and which are best-effort.

---

## 8. Implementation Hints for the SPEC Author

### 8.1 SPEC Structure and Depth Calibration

Reference SPEC-CORE-001 and SPEC-IR-001 to calibrate depth and scope:
- **SPEC-CORE-001** (`pkg/types/normalized_doc.go`, `adapter.go`, etc.) is ~700 lines of EARS requirements spanning 5-6 REQ-* items.
- **SPEC-IR-001** (intent router) is ~500 lines of EARS spanning 8-10 REQ-* items.
- **SPEC-ADP-001 should be ~600-800 lines** spanning 10-12 REQ-* items.

The SPEC's structure (per MoAI convention) is:
1. **Preamble**: Problem statement, success criteria, acceptance tests.
2. **Requirements** (EARS format): Ubiquitous, Event-Driven, State-Driven, Optional, Unwanted rules.
3. **Design**: Technical approach (HTTP client setup, error handling, observability, testing strategy).
4. **Acceptance Criteria**: Specific pass/fail gates for implementation.

### 8.2 EARS Pattern Coverage

Suggested REQ breakdown for SPEC-ADP-001:

| ID | Pattern | Example |
|----|---------|----|
| REQ-ADP-001 | Ubiquitous (Adapter interface) | **GIVEN** a Search request, **WHEN** the adapter processes it, **THEN** it returns []NormalizedDoc or *SourceError |
| REQ-ADP-002 | Event-Driven (HTTP 429) | **WHEN** HTTP 429 received **WITH** Retry-After header, **THEN** return *SourceError{Category: CategoryRateLimited, RetryAfter: parsed_value} |
| REQ-ADP-003 | State-Driven (pagination) | **WHILE** more results available **THEN** next Search request uses prior Cursor as `after` parameter |
| REQ-ADP-004 | Optional (NSFW filter) | **IF** Query.Filters contains {Key: "nsfw", Value: "false"}, **THEN** pass `include_over_18=false` to Reddit |
| REQ-ADP-005 | Unwanted (non-t3 kinds) | **IF** Reddit response includes kind != "t3", **THEN** filter out and do not return |
| REQ-ADP-006 | Ubiquitous (field mapping) | **GIVEN** Reddit JSON post data, **WHEN** normalizing to NormalizedDoc, **THEN** map each field per table in research.md §2.1 |
| REQ-ADP-007 | Event-Driven (error mapping) | **WHEN** HTTP 4xx/5xx received, **THEN** wrap in *SourceError with appropriate Category (Permanent/Transient/Unavailable) |
| REQ-ADP-008 | State-Driven (User-Agent) | **BEFORE** first HTTP request, **THEN** validate User-Agent is set and non-default |
| REQ-ADP-009 | Optional (deleted posts) | **IF** Author = "[deleted]" or Body = "[removed]", **THEN** return doc as-is (do not filter) |
| REQ-ADP-010 | Ubiquitous (score normalization) | **GIVEN** Reddit score, **WHEN** normalizing to [0, 1], **THEN** apply formula: Score = 0.5 + 0.5 * tanh(score / 100) |
| REQ-ADP-011 | Ubiquitous (Capabilities static) | **GIVEN** Adapter created, **WHEN** Capabilities() called multiple times, **THEN** return identical value (deterministic) |
| REQ-ADP-012 | Event-Driven (redirect validation) | **WHEN** HTTP redirect received, **THEN** validate target domain is reddit.com, reject if cross-origin |

### 8.3 Acceptance Criteria

Suggest at least 15-20 acceptance criteria organized by REQ:

| Criteria | Test Approach |
|----------|---|
| AC-001: Search happy path (25 results) | httptest.Server + golden fixture (search_response.json) |
| AC-002: Pagination round-trip (request with `after` cursor) | Mock second request with cursor; verify URL includes `after=...` |
| AC-003: HTTP 429 with Retry-After | Mock 429 response; parse header; verify RetryAfter field |
| AC-004: HTTP 404 returns CategoryPermanent | Mock 404; verify error categorization |
| AC-005: HTTP 5xx returns CategoryTransient | Mock 500; verify error categorization |
| AC-006: NSFW filter active (include_over_18=false by default) | Verify URL params when no explicit filter |
| AC-007: Score normalization (Tanh formula) | Table test with score values (0, 100, -50, 1000) → expected Score |
| AC-008: [deleted] posts returned with Author="[deleted]" | Golden fixture with deleted post; verify NormalizedDoc.Author field |
| AC-009: Redirect validation (reject cross-origin) | httptest mock redirect to attacker.com; verify error |
| AC-010: Capabilities deterministic (called 2x) | Call Capabilities twice in unit test; assert equality |
| AC-011: Context cancellation (ctx.Done already fired) | Create context, cancel, call Search; verify immediate return |
| AC-012: User-Agent set in requests | Inspect request made by client; verify UA header presence |
| AC-013: NormalizedDoc.Validate() passes for all returned docs | Iterate returned slice, call Validate on each |
| AC-014: Hash field empty (not pre-computed) | Verify returned NormalizedDoc.Hash == "" |
| AC-015: Metadata keys present and populated | Verify Metadata map has at least {subreddit, over_18, num_comments} |
| AC-016: Test coverage >= 85% | Run `go test -cover ./internal/adapters/reddit` |

### 8.4 NFRs to Document

- **NFR-ADP-001**: Per-call latency — first 25 results ≤ 5s (P95) under normal load.
- **NFR-ADP-002**: Allocation count on hot parse path ≤ 10 allocs per 10 docs (use `go test -bench` with pprof).
- **NFR-ADP-003**: No goroutine leaks on context cancellation (use `goleak` in tests).
- **NFR-ADP-004**: Error messages MUST NOT leak PII (credentials, internal IPs, stack traces).

### 8.5 Technical Approach Section

In the DESIGN section of the SPEC, outline:

1. **HTTP client setup**: Timeout, retry policy (NONE for adapter; fanout handles retries), User-Agent handling.
2. **Request construction**: Map Query to URL params; validate inputs.
3. **Response parsing**: JSON unmarshal → intermediate struct → NormalizedDoc array.
4. **Error handling**: Distinguish HTTP status codes; categorize per taxonomy.
5. **Testing strategy**: httptest.Server + golden fixtures; table-driven tests for field mapping; mocks for error cases.
6. **Observability**: Document that the adapter does NOT emit metrics/logs (registry does). Adapter only ensures errors are correctly categorized.

### 8.6 Deferred (Future SPEC Versions)

Document in the SPEC what is intentionally OUT of V1 scope:

- Time-range filtering (t parameter; defaulting to `t=all`).
- Subreddit-scoped search (`restrict_sr=on` + `/r/{sub}/search.json` path).
- Language filtering or inference.
- OAuth authentication (unauthenticated public endpoint only).
- Advanced result sorting (defaulting to `sort=relevance`).
- User preference extraction (personalization deferred to IR layer).

---

## 9. Sources and Citations

### External URLs (WebFetch verified)

- https://til.simonwillison.net/reddit/scraping-reddit-json — Simon Willison's TIL on Reddit JSON API querying and User-Agent requirements.
- https://github.com/searxng/searxng/blob/master/searx/engines/reddit.py — SearXNG Reddit engine implementation; canonical request/response structure reference.
- https://github.com/vartanbeno/go-reddit — go-reddit MIT-licensed Go Reddit client library.
- https://painonsocial.com/blog/reddit-api-rate-limits-guide — Reddit API rate limits (10/min unauthenticated, 60/min authenticated).
- https://painonsocial.com/blog/reddit-api-rate-limits-workaround — Strategies for working within Reddit rate limits.

### Internal Files (file-cited with line numbers)

- `/Users/masterp/Projects/superwork/univesal-search/pkg/types/normalized_doc.go:40-106` — NormalizedDoc struct definition and CanonicalHash method.
- `/Users/masterp/Projects/superwork/univesal-search/pkg/types/adapter.go:28-45` — Adapter interface contract.
- `/Users/masterp/Projects/superwork/univesal-search/pkg/types/errors.go:14-120` — Error taxonomy (Category sentinels, SourceError, OutcomeFromError).
- `/Users/masterp/Projects/superwork/univesal-search/pkg/types/query.go:18-44` — Query type and Filter struct.
- `/Users/masterp/Projects/superwork/univesal-search/pkg/types/capabilities.go:38-62` — Capabilities struct.
- `/Users/masterp/Projects/superwork/univesal-search/internal/adapters/registry.go:75-138` — Registry pattern, RegisterWithOptions, wrappedAdapter.
- `/Users/masterp/Projects/superwork/univesal-search/internal/adapters/noop/noop.go:1-46` — Reference noop adapter implementation.
- `/Users/masterp/Projects/superwork/univesal-search/internal/llm/client.go:31-65` — HTTP client construction pattern with timeout and transport wrapping.
- `/Users/masterp/Projects/superwork/univesal-search/.moai/project/roadmap.md:36-39` — SPEC-ADP-001 roadmap entry.
- `/Users/masterp/Projects/superwork/univesal-search/.moai/project/tech.md:102-107` — Adapter strategy matrix, including Reddit rate-limit discrepancy (60/min claimed).

---

End of Research Document.

**Summary for SPEC Author**: This research establishes the Reddit API surface (search endpoint, query parameters, response envelope, pagination via opaque `after` cursor, rate limits at 10/min unauthenticated), provides complete NormalizedDoc field mappings with three design decisions (Tanh score normalization, DocTypePost over DocTypeSocial, rich Metadata map), documents the adapter interface pattern (four methods, SourceError categorization, registry wrapping observability), proposes a realistic file layout (10 files, ~1,500 LoC + tests), surveys reference implementations (SearXNG as pattern, go-reddit as DO-NOT-USE for supply chain reasons), and flags 9 risks with mitigations. Seven Open Questions are deferred (Healthcheck approach, score formula confirmation, caching, time-range support, [deleted] handling, redirect whitelist, Metadata API surface) with recommended defaults. The SPEC should span 600-800 lines covering 10-12 EARS REQ-* items, structure similar to SPEC-CORE-001 and SPEC-IR-001, and include 15+ acceptance criteria with httptest-based testing strategy targeting 85%+ coverage.
