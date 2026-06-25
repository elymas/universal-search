# Acceptance Criteria — SPEC-ADP-001b: Reddit RSS Adapter

All scenarios use a loopback `httptest.Server` as the search host (via
`Options.BaseURL`) serving a canned `search.rss` fixture. No live network calls
and no credentials are used in any test.

## Given-When-Then Scenarios

### Scenario 1 — Successful search maps RSS items to NormalizedDoc (REQ-ADP1B-005,007,018)

- **Given** an `httptest` server that returns HTTP 200 with a canned Reddit
  `search.rss` body containing 3 items (each with title, link, description,
  pubDate), and an adapter built with `Options.BaseURL` pointing at it.
- **When** `Search(ctx, types.Query{Text: "golang generics"})` is called.
- **Then** the request path is `/search.rss` with query `q=golang+generics`
  (url-encoded) and `sort=relevance`; AND exactly 3 `NormalizedDoc` are returned;
  AND each doc has `SourceID == "reddit-rss"`, `DocType == types.DocTypePost`, a
  non-empty `URL`, a non-empty `Hash`, and `PublishedAt` parsed from the item.

### Scenario 2 — Empty query is rejected without a network call (REQ-ADP1B-016)

- **Given** an adapter whose `httptest` server would fail the test if hit.
- **When** `Search(ctx, types.Query{Text: "   "})` is called.
- **Then** a `*types.SourceError` with `Category == types.CategoryPermanent` is
  returned; AND the server received zero requests.

### Scenario 3 — HTTP 429 maps to rate-limited with RetryAfter (REQ-ADP1B-011)

- **Given** an `httptest` server returning HTTP 429 with `Retry-After: 30`.
- **When** `Search(ctx, q)` is called with a non-empty query.
- **Then** the error is a `*types.SourceError` with
  `Category == types.CategoryRateLimited` and `RetryAfter == 30s`; AND
  `errors.Is(err, types.ErrRateLimited)` is true.

### Scenario 3a — HTTP 403 (anon-block) maps to Unavailable / retryable (REQ-ADP1B-012)

- **Given** an `httptest` server returning HTTP 403 (the response Reddit serves
  to anonymous/unidentified RSS traffic).
- **When** `Search(ctx, q)` is called with a non-empty query.
- **Then** the error is a `*types.SourceError` with
  `Category == types.CategoryUnavailable` and `HTTPStatus == 403`; AND
  `errors.Is(err, types.ErrSourceUnavailable)` is true (so fanout treats it as
  retryable, not a permanent zero-result failure).

### Scenario 4 — Context cancellation aborts the fetch (REQ-ADP1B-010)

- **Given** an adapter and an already-cancelled `ctx` (or a server that delays
  past a short ctx deadline).
- **When** `Search(ctx, q)` is called.
- **Then** the call returns promptly with a `*types.SourceError`
  (`CategoryTransient` on deadline, `CategoryUnavailable` on cancel); AND no
  goroutine leak is observed.

### Scenario 5 — Always-on registration and source visibility (REQ-ADP1B-020)

- **Given** an environment with NO `REDDIT_CLIENT_ID` / `REDDIT_CLIENT_SECRET`
  set (and `REDDIT_RSS_BASE_URL` pointing at an `httptest` server returning 200).
- **When** the production registry is built and `usearch sources status` (or the
  equivalent `classifyAdapters` path) probes adapters.
- **Then** `reddit-rss` appears in the registry list; AND its status is
  `connected` (Healthcheck returns nil against the 200 server); AND the adapter
  registered even though no Reddit OAuth credentials were present.

## Edge Cases

- EC1 — Item with empty link is skipped, not emitted as an invalid doc
  (REQ-ADP1B-009).
- EC2 — Item without a parseable `pubDate` yields a zero `PublishedAt` but a
  still-valid doc.
- EC3 — Malformed (non-XML) body on HTTP 200 yields
  `*types.SourceError{CategoryTransient}` (REQ-ADP1B-015).
- EC4 — HTTP 503 yields `*types.SourceError{CategoryUnavailable, HTTPStatus:503}`
  (REQ-ADP1B-013).
- EC5 — HTTP 404 (a 4xx that is neither 429 nor 403) yields
  `*types.SourceError{CategoryPermanent, HTTPStatus:404}` (REQ-ADP1B-012a).
- EC6 — Every emitted doc has `Score == 0.5` (the sole score assertion; the
  adapter assigns the neutral constant and parses no engagement signal in v0.1)
  (REQ-ADP1B-008).
- EC7 — Production redirect to a host outside `{www.reddit.com}` is rejected
  (REQ-ADP1B-017); the test `BaseURL` (loopback) is exempt.

## Quality Gate Criteria

- `go vet ./internal/adapters/reddit_rss/...` clean.
- `golangci-lint run ./internal/adapters/reddit_rss/...` clean.
- `go test -race ./internal/adapters/reddit_rss/...` passes.
- Statement coverage for `internal/adapters/reddit_rss/` >= 85%.
- No credentials, no live network calls in the test suite.
- The OAuth `reddit` package and its tests are unchanged (no diff in
  `internal/adapters/reddit/`).

## Definition of Done

- [ ] `reddit-rss` adapter package implements `types.Adapter` (compile-time
      assertion present).
- [ ] All REQ-ADP1B requirements satisfied with corresponding tests
      (001..005, 007..021, plus 012a; note 006 is intentionally absent — moved to
      Exclusions).
- [ ] Table-driven tests with loopback `httptest` server + canned fixture.
- [ ] Coverage >= 85%; `go test -race`, `go vet`, `golangci-lint` all green.
- [ ] Registered always-on in `pipeline.go` non-credentialed block.
- [ ] `usearch sources status` shows `reddit-rss connected` with no creds.
- [ ] Existing OAuth `reddit` adapter untouched.
