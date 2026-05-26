# SPEC-ADP-006 Implementation Plan (Post-Hoc)

**SPEC**: SPEC-ADP-006 — Bluesky + X Adapter
**Status**: implemented (2026-05-07)
**Methodology**: TDD (RED → GREEN → REFACTOR)
**Coverage Target**: 85%
**Owner**: expert-backend
**Priority**: P0

---

## 1. Overview

ADP-006 is the M3 SOCIAL-PLATFORM adapter. v0.1 ships Bluesky-INTEGRATED
+ X RESERVED-but-DISABLED in a single package
`internal/adapters/social/` with TWO constructors and TWO Adapter
instance registrations. Key architectural deltas:

1. **Single package, two Adapter instances** — one `*Adapter` struct
   type, distinguished by an unexported `subSource string` field
   (`"bluesky"` or `"x"`). `(*Adapter).Name()` returns `subSource`
   so the registry emits two distinct `adapter` label values
   uniformly. `(*Adapter).Search` dispatches on `subSource` to
   `searchBluesky` (live HTTP) or `searchXDisabled` (env-gated
   stub). Mirrors a "single package, multi-source dispatch" pattern
   future similar adapters may adopt (e.g., fediverse).

2. **X reserved-but-disabled with two-state error semantics**:
   - `USEARCH_X_ENABLED` not set or != `"true"` → `ErrXDisabled`
     (operator did not opt in).
   - `USEARCH_X_ENABLED="true"` but no provider wired →
     `ErrXProviderNotConfigured` (operator opted in but v0 has no
     provider). Distinguishes operator intent from missing
     implementation.
   - ZERO HTTP requests in v0 regardless of env state.

3. **No `github.com/bluesky-social/indigo` dependency** — indigo's
   own README warns "interfaces have not stabilized"; stdlib
   `net/http` + `encoding/json` suffices for one HTTP call. Mirrors
   ADP-001 §5.2's go-reddit rejection.

4. **Goroutine-safe env injection via `Options.EnvLookup`** —
   tests inject a closure rather than mutating `os.Setenv` which is
   not goroutine-safe under `-race` (per Go testing docs). Each
   `*Adapter` carries its own closure (no global mutation).

5. **Bluesky URL construction** — deterministic from `author.handle`
   + `rkey` (last segment of AT-URI):
   `https://bsky.app/profile/<handle>/post/<rkey>`. Full AT-URI
   preserved in `Metadata["post_uri"]`. DID fallback when handle is
   empty.

---

## 2. Architecture

### 2.1 Package Layout

```
internal/adapters/social/
├── social.go              — Adapter, NewBluesky, NewX, Name, Capabilities, Healthcheck
├── social_test.go         — interface conformance + Capabilities + Healthcheck (both sub-sources)
├── search_bluesky.go      — (*Adapter).searchBluesky live path
├── search_x.go            — (*Adapter).searchXDisabled env-gated stub
├── search_test.go         — Bluesky live + X disabled paths, all error categorisation
├── client.go              — *http.Client, doRequest, categorizeStatus(adapterName)
├── client_test.go         — categorizeStatus table for both adapter names, redirect allowlist
├── parse.go               — parseSearchPosts transform (Bluesky AppView envelope)
├── parse_test.go          — field mapping + XRPC error envelope + pagination cursor
├── url.go                 — parseATURI + constructBlueskyURL (handle / DID fallback)
├── url_test.go            — AT-URI parse table + URL construction edge cases
├── score.go               — normalizeScore(likeCount, repostCount) Tanh formula
├── score_test.go          — score table over 7 (like, repost) tuples
├── errors.go              — 4 sentinels (ErrInvalidQuery, ErrInvalidCursor, ErrXDisabled, ErrXProviderNotConfigured) + parseRetryAfter
├── errors_test.go         — sentinel comparison + parseRetryAfter table
├── bench_test.go          — BenchmarkParseSearchPosts25Docs + TestMain goleak
└── testdata/              — 7 JSON fixtures (Bluesky-only; X has no HTTP path)
```

### 2.2 Key Data Structures

**`Adapter` struct** (`social.go`): `httpClient`, `baseURL`,
`userAgent`, `healthcheckTarget`, `subSource string`
(`"bluesky"` or `"x"`), `envLookup func(string) string` (goroutine-
safe; default `os.Getenv`).

**`BlueskyOptions` + `XOptions` structs**: distinct types so the
constructor signatures cannot be confused. Both share the common
HTTP / UA / Healthcheck override fields; `XOptions` adds
`EnvLookup func(string) string` for testability.

**Bluesky response envelope** (`parse.go`): structs match
`app.bsky.feed.searchPosts` Lexicon — `{cursor string, posts
[]postView}`. XRPC error envelope: `{error, message}`.

**Sentinels** (`errors.go`):
- `ErrInvalidQuery` — empty/whitespace.
- `ErrInvalidCursor` — for Bluesky; opaque cursor must be non-empty
  if provided. (Empty cursor is the start-of-search signal.)
- `ErrXDisabled` — `USEARCH_X_ENABLED` env not set to `"true"`.
- `ErrXProviderNotConfigured` — env opted in but no provider wired.

### 2.3 Hot-Path Flow

**Bluesky (REQ-ADP6-002)**:
1. Validate `q.Text` (REQ-ADP6-002 acceptance includes empty/
   whitespace rejection).
2. Build URL via `url.Values` with `q`, `limit` (clamped 1–100,
   default 25), `cursor` (when non-empty), `lang` (when non-empty),
   `since` (when filter present and parseable as ISO datetime),
   `sort=top` (hardcoded v0).
3. `doRequest` sets UA + `Accept: application/json`.
4. Route by HTTP status via `categorizeStatus("bluesky", ...)`.
5. `parseSearchPosts` decodes envelope, applies per-post transform
   per §6.5, surfaces opaque `cursor` via
   `Metadata["next_cursor"]` on last doc when non-empty.

**X (REQ-ADP6-008)**:
1. Read `os.Getenv("USEARCH_X_ENABLED")` (or `Options.EnvLookup`).
2. NOT `"true"` (case-sensitive) → `*SourceError{Permanent, Cause:
   ErrXDisabled}`.
3. == `"true"` → `*SourceError{Permanent, Cause:
   ErrXProviderNotConfigured}`.
4. Zero HTTP requests issued in either branch.

### 2.4 Bluesky Post → NormalizedDoc Mapping (REQ-ADP6-005)

Iteration 2 (plan-auditor cycle 1) tightened the Title/Body/Snippet
assignment:
- `Title = ""` (AT spec defines no headline field).
- `Body = record.text` (full post text).
- `Snippet = truncate(record.text, 280)`.
- `URL = constructBlueskyURL(author.handle, post.uri)` — falls back
  to DID when handle empty.
- `Metadata["post_uri"] = post.uri` (full AT-URI).
- `Score = normalizeScore(likeCount + repostCount)` per ADP-001 §2.3
  Tanh formula with divisor=100.

### 2.5 Integration Points

- **Two registry entries**: registry registers both `*Adapter`
  instances under names `"bluesky"` and `"x"`; the wrappedAdapter
  emits two distinct `adapter` label values uniformly.
- **Consumes**: `pkg/types`, `internal/obs/reqid.NewTransport`.
- **X disabled-path observability**: ErrXDisabled /
  ErrXProviderNotConfigured both map to `outcome="failure"` via
  `types.OutcomeFromError`; FAN-001 records in
  `Result.AdapterErrors["x"]` and contributes zero docs.

### 2.6 Redirect Allowlist

- Bluesky: `{public.api.bsky.app, api.bsky.app, bsky.app}` with 3-hop
  cap.
- X: `{}` (no redirects accepted since v0 makes no HTTP calls).

---

## 3. Test Coverage Notes

- Coverage meets 85% target.
- ~70 tests per spec.md iteration-2 HISTORY.
- X disabled-path table-driven over env states: unset, `"true"`,
  `"yes"`, `"1"`, `"TRUE"` (capitalised) — only exact `"true"` opts
  in; case-sensitive.
- `Options.EnvLookup` injection ensures concurrent tests (REQ-ADP6-009)
  do not race on `t.Setenv` (goroutine-unsafe under `-race`).
- `TestSearchConcurrentSafe` (50 goroutines, mixed Bluesky + X) —
  NFR-ADP6-004.
- `BenchmarkParseSearchPosts25Docs` median ≤ 5ms; allocs/op ≤ 500.

---

## 4. Technical Decisions (Locked During Implementation)

| Decision | Rationale |
|----------|-----------|
| Single package, two Adapter instances | Cardinality discipline (two `adapter` label values, not three or more); routing simplicity; future-portability if X gets de-merged later. |
| No `bluesky-social/indigo` dep | indigo carries "under active development" warning; transitive CBOR/libp2p/multiformats deps inappropriate for one HTTP call. |
| Goroutine-safe `Options.EnvLookup` over `t.Setenv` | `t.Setenv` is not goroutine-safe under `-race`; closure injection allows concurrent tests with per-instance env state. |
| `Title=""` for Bluesky | AT spec defines no headline field; the prior implementation duplicated `record.text` between Title and Snippet; cleaner shape preserves CORE-001:64 semantics (Title not required; empty permitted). |
| X reserved-not-removed | IR-001 routing emits `AdapterSet=["bluesky","x"]` for social queries; the X stub surfaces clean errors without router awareness of the env gate. |
| Case-sensitive `"true"` literal | Avoids ambiguity with `"yes"`, `"1"`, `"TRUE"`; matches Go's stricter env-truth idiom. |
| Score uses `likeCount + repostCount` only | Reply/quote counts surface in Metadata but not in score; matches ADP-001 inflection-at-100 posture without per-source recalibration. |

---

## 5. Risks Mitigated

- **`t.Setenv` race under `-race`** → `Options.EnvLookup` injection
  per spec.md HISTORY iteration-2 H1 fix.
- **AT spec drift** → `encoding/json` tolerates unknown fields;
  fixtures pinned to documented shape.
- **Operator confusion about Healthcheck on X** → Capabilities.Notes
  explicitly disclaims: "Healthcheck verifies TCP reachability of
  x.com:443 only; Search remains disabled regardless of Healthcheck
  outcome" (iteration-2 H3 fix).
- **Hash collisions across Bluesky / Reddit / HN** →
  `CanonicalHash` includes `SourceID` prefix.

---

## 6. Out-of-Scope Reminders (from spec.md §7)

- App Password / `com.atproto.server.createSession` authenticated
  variant → SPEC-ADP-006-AUTH (future).
- X provider integration (ScrapeCreators, Nitter, official tier) →
  SPEC-ADP-006-XENABLE (future, behind explicit ToS acknowledgement).
- AT Protocol feed generators / lists → out of v0.1.
- Bluesky media downloads → SPEC-CACHE-001 may consume embed URLs.
- Live network integration tests in CI.
- Per-adapter custom Prometheus metrics.

---

*End of SPEC-ADP-006 plan.md (post-hoc, v1.0)*
