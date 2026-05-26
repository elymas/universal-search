# SPEC-ADP-006 Acceptance Criteria (Post-Hoc)

**SPEC**: SPEC-ADP-006 — Bluesky + X Adapter
**Status**: implemented (2026-05-07)
**Format**: Given/When/Then per REQ + edge cases + Definition of Done

---

## 1. Acceptance Scenarios by REQ

### REQ-ADP6-001 — Adapter Interface Conformance (Both Sub-Sources)

**AC-001: Compile-time interface assertion**
- `var _ types.Adapter = (*Adapter)(nil)` in `social.go`; build
  succeeds.

**AC-002: Names differ by constructor**
- `NewBluesky(opts).Name() == "bluesky"`.
- `NewX(opts).Name() == "x"`.

**AC-003: Bluesky Capabilities deterministic + shape-correct**
- `SourceID="bluesky"`, `DisplayName="Bluesky"`,
  `DocTypes=[DocTypePost]`, `SupportedLangs=nil`,
  `RequiresAuth=false`, etc.

**AC-004: X Capabilities deterministic + shape-correct**
- `SourceID="x"`, `DisplayName="X"`, `DocTypes=[DocTypePost]`.
- `Notes` includes Healthcheck disclaimer: "Healthcheck verifies
  TCP reachability of x.com:443 only; Search remains disabled
  regardless of Healthcheck outcome".

**AC-005: Healthcheck succeeds for both**
- Stub `httptest.Server` reachable on configured target → nil error
  for both sub-sources.

### REQ-ADP6-002 — Bluesky Search Happy Path

**AC-006: Happy path 25 posts**
- Stub returns `testdata/bluesky_search_response.json` → 25
  NormalizedDocs; each `Validate()` returns nil.

**AC-007: URL parameters required**
- URL contains `q`, `limit`, `sort=top`.

**AC-008: limit clamp / default**
- `MaxResults=500` → `limit=100`.
- `MaxResults=0` → `limit=25`.

**AC-009: cursor pass-through (opaque)**
- `Cursor="bsky_cursor_abc"` → URL has `cursor=bsky_cursor_abc`.

**AC-010: Empty query rejected with zero HTTP**
- `Text=""` → `*SourceError{Permanent, Cause: ErrInvalidQuery}`;
  ZERO requests.

### REQ-ADP6-003 — Bluesky HTTP 429 Mapping

**AC-011: Integer Retry-After → matching seconds**
**AC-012: HTTP-date Retry-After**
**AC-013: No header defaults to 5s**
**AC-014: Capped at 60s**
**AC-015: No internal retry** (1 request observed)

### REQ-ADP6-004 — Bluesky HTTP 4xx / 5xx / Network

**AC-016: 4xx → Permanent** (table over 400/401/403/404)
**AC-017: 5xx → Unavailable** (table over 500/503)
**AC-018: Connection refused → Unavailable, HTTPStatus=0**
**AC-019: XRPC error envelope preserved**
- Bluesky returns `{error: "InvalidRequest", message: "..."}` →
  `*SourceError{Permanent, Cause: <error containing "InvalidRequest"
  + message>}`.

### REQ-ADP6-005 — NormalizedDoc Field Mapping

**AC-020: Field mapping table (5 fixtures)**
- Typical post with non-empty langs, post with empty langs, post
  with empty handle (DID fallback URL), high-engagement post, empty
  repostCount/likeCount.

**AC-021: Title = "" (iteration-2 H2 fix)**
- Bluesky has no headline field; Title is empty; Body = `record.text`;
  Snippet = truncated `record.text`.

**AC-022: URL construction**
- `https://bsky.app/profile/<handle>/post/<rkey>` from
  `author.handle` + `parseATURI(post.uri).rkey`.

**AC-023: DID fallback when handle empty**
- `https://bsky.app/profile/<did>/post/<rkey>` when
  `author.handle == ""`.

**AC-024: Full AT-URI preserved in Metadata["post_uri"]**

**AC-025: Score = normalizeScore(likeCount + repostCount)**

**AC-026: Pagination cursor on last doc**
- Response with non-empty `cursor` → last doc
  `Metadata["next_cursor"] = <cursor>`.

**AC-027: Hash always empty**

### REQ-ADP6-006 — User-Agent + Accept Headers (Bluesky)

**AC-028: Custom UA + Accept**
**AC-029: UA version configurable**

### REQ-ADP6-007 — Lang and Since Filters (Optional)

**AC-030: lang filter passed through**
- `Filters=[{lang, "ko"}]` → URL has `lang=ko`.

**AC-031: since filter parsed as ISO datetime**
- `Filters=[{since, "2026-04-01T00:00:00Z"}]` → URL has `since=...`.

**AC-032: Malformed since dropped silently**
- `Filters=[{since, "abc"}]` → no `since` param; no error.

**AC-033: Unknown filter ignored**

### REQ-ADP6-008 — X Sub-Source Two-State Error Semantics (Event-Driven)

**AC-034: USEARCH_X_ENABLED unset → ErrXDisabled**
- `Options.EnvLookup` returns `""` for `USEARCH_X_ENABLED` →
  `*SourceError{Permanent, Cause: ErrXDisabled}`; ZERO HTTP requests.

**AC-035: USEARCH_X_ENABLED="true" → ErrXProviderNotConfigured**
- `Options.EnvLookup` returns `"true"` →
  `*SourceError{Permanent, Cause: ErrXProviderNotConfigured}`;
  ZERO HTTP requests.

**AC-036: Case-sensitive "true" only**
- `Options.EnvLookup` returns `"yes"`, `"1"`, or `"TRUE"` →
  ErrXDisabled (only exact `"true"` opts in); ZERO HTTP requests.

### REQ-ADP6-009 — Bluesky Redirect Allowlist (Optional)

**AC-037: Allowlist hosts**
- Allowed: `public.api.bsky.app`, `api.bsky.app`, `bsky.app`.
- 3-hop cap enforced.

**AC-038: Cross-domain rejected**
- `attacker.com` redirect → `*SourceError{Permanent, Cause: <error
  containing "cross-domain redirect">}`.

**AC-039: Chain over 3 hops rejected**

### REQ-ADP6-010 — Concurrent Search Safety (State-Driven)

**AC-040: 50 goroutines race-clean for both sub-sources**
- Mixed 25 Bluesky + 25 X concurrent calls; `-race` clean; expected
  outcomes per sub-source.

---

## 2. NFR Acceptance

### NFR-ADP6-001 — Parse-Path Performance

**AC-N01: Benchmark within target**
- `BenchmarkParseSearchPosts25Docs` median ≤ 5 ms;
  `allocs/op ≤ 500` (target may rise in future iteration per
  iteration-2 M1 fix).

### NFR-ADP6-002 — E2E p95 (Stub)

**AC-N02: p95 ≤ 200ms** over 100 invocations against Bluesky stub.

### NFR-ADP6-003 — No Goroutine Leak on Cancellation

**AC-N03: goleak.VerifyNone + TestMain VerifyTestMain**

### NFR-ADP6-004 — Race-Clean Across Both Sub-Sources

**AC-N04: 50 goroutines mixed across Bluesky + X**
- `go test -race ./internal/adapters/social/...` reports zero
  data-race alarms.

---

## 3. Edge Cases

**EC-001: Empty posts array**
- `parseSearchPosts` returns `(nil, "", nil)`.

**EC-002: XRPC error envelope distinct from HTTP error**
- HTTP 200 + JSON `{error, message}` body → `*SourceError{Permanent,
  Cause: <error containing both fields>}`.

**EC-003: AT-URI parse edge cases**
- `url_test.go` table over 6 inputs (typical, missing scheme,
  missing collection, missing rkey, empty, malformed).

**EC-004: Score(0,0) = 0.5 exactly**
- Zero engagement → neutral middle.

**EC-005: Negative counts defensively coerced to 0**
- Defensive: `(like=-1, repost=0)` → Score=0.5 (not panic).

**EC-006: X sub-source emits via wrappedAdapter as failure outcome**
- Disabled errors map to `outcome="failure"` via
  `types.OutcomeFromError`.

---

## 4. Coverage Matrix

| REQ-ID | EARS Pattern | Acceptance Criteria | Implementation |
|--------|-------------|---------------------|----------------|
| REQ-ADP6-001 | Ubiquitous | AC-001..005 | `social.go`, `social_test.go` |
| REQ-ADP6-002 | Event-Driven | AC-006..010 | `search_bluesky.go`, `search_test.go` |
| REQ-ADP6-003 | Event-Driven | AC-011..015 | `client.go::categorizeStatus`, `errors.go::parseRetryAfter` |
| REQ-ADP6-004 | Event-Driven | AC-016..019 | `client.go::categorizeStatus`, `parse.go` XRPC error path |
| REQ-ADP6-005 | Ubiquitous | AC-020..027 | `parse.go::parseSearchPosts`, `url.go::constructBlueskyURL`, `score.go::normalizeScore` |
| REQ-ADP6-006 | Ubiquitous | AC-028..029 | `client.go::doRequest` |
| REQ-ADP6-007 | Optional | AC-030..033 | `search_bluesky.go` (filter handling) |
| REQ-ADP6-008 | Event-Driven | AC-034..036 | `search_x.go::searchXDisabled`, `errors.go::ErrXDisabled` + `ErrXProviderNotConfigured` |
| REQ-ADP6-009 | Optional | AC-037..039 | `client.go::redirectAllowlist` |
| REQ-ADP6-010 | State-Driven | AC-040 | `search_test.go::TestSearchConcurrentSafe` |
| NFR-ADP6-001 | Performance | AC-N01 | `bench_test.go::BenchmarkParseSearchPosts25Docs` |
| NFR-ADP6-002 | Latency | AC-N02 | `search_test.go::TestSearchE2ELatencyStubP95` |
| NFR-ADP6-003 | Resource | AC-N03 | `search_test.go::TestSearchNoGoroutineLeakOnCancel` |
| NFR-ADP6-004 | Race-clean | AC-N04 | `search_test.go::TestSearchConcurrentSafe` (mixed sub-sources) |

---

## 5. Definition of Done

- [x] All 10 EARS REQs have passing tests for both sub-sources
      where applicable.
- [x] All 4 NFRs have passing measurements.
- [x] `go test ./internal/adapters/social/...` exits 0.
- [x] `go test -race ./internal/adapters/social/...` exits 0
      (concurrent X tests use `Options.EnvLookup`, NOT `t.Setenv`).
- [x] `go test -cover` reports ≥ 85%.
- [x] `go vet` and `golangci-lint run` clean.
- [x] `BenchmarkParseSearchPosts25Docs` median ≤ 5ms;
      allocs/op ≤ 500.
- [x] X disabled-path table-driven over env states.
- [x] Capabilities.Notes for X includes Healthcheck disclaimer.
- [x] MX tags applied per spec.md plan.
- [x] `var _ types.Adapter = (*Adapter)(nil)` present.
- [x] No drive-by changes outside `internal/adapters/social/`.
- [x] SPEC status updated to `implemented` (2026-05-07).

---

*End of SPEC-ADP-006 acceptance.md (post-hoc, v1.0)*
