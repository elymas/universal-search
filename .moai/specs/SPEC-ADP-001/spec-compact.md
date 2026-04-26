# SPEC-ADP-001 Compact Reference

**Reddit Adapter (Reference Implementation)** | M2 | P0 | TDD | 85% coverage | standard harness

Token-efficient summary for run-phase context. Source of truth: `spec.md`.

---

## EARS Requirements (one-line each)

- **REQ-ADP-001 (Ubiquitous)** Adapter implements `pkg/types.Adapter` exactly (Name, Search, Healthcheck, Capabilities) with compile-time assertion `var _ types.Adapter = (*Adapter)(nil)`. Capabilities deterministic; SourceID="reddit".
- **REQ-ADP-002 (Event-Driven)** WHEN Search is invoked with non-empty `q.Text`, GET `https://www.reddit.com/search.json?q=...&sort=relevance&t=all&type=link&limit=clamp(MaxResults,1,100)&include_over_18=<filter>&after=<Cursor>` and return `[]NormalizedDoc` per §6.3 mapping.
- **REQ-ADP-003 (Event-Driven)** WHEN HTTP 429, parse `Retry-After` (int seconds OR HTTP-date, capped 60s, default 5s), return `*SourceError{CategoryRateLimited, HTTPStatus:429, RetryAfter:<dur>, Cause:<inner>}`. NO internal retry.
- **REQ-ADP-004 (Event-Driven)** WHEN HTTP 401/403/404, return `*SourceError{CategoryPermanent, HTTPStatus:<code>, Cause:<inner>}`. NO retry.
- **REQ-ADP-005 (Event-Driven)** WHEN HTTP 5xx OR connection error, return `*SourceError{CategoryUnavailable, HTTPStatus:<code or 0>, Cause:<inner>}`. NO retry.
- **REQ-ADP-006 (Ubiquitous)** Transform each `kind=="t3"` child to `NormalizedDoc` per §6.3 mapping; skip non-t3; set `RetrievedAt=time.Now().UTC()`, `Hash=""`, `DocType=DocTypePost`, `Lang=""`; populate Metadata with `{subreddit, over_18, num_comments, upvote_ratio, external_url, kind}`; surface `data.after` as `Metadata["next_cursor"]` on the LAST returned doc.
- **REQ-ADP-007 (Optional)** WHERE `Filters[{nsfw, true}]` present → `include_over_18=true`; absent OR `Value!="true"` → `include_over_18=false` (default exclude NSFW).
- **REQ-ADP-008 (Unwanted)** IF `q.Text` is empty or all-whitespace, return `*SourceError{CategoryPermanent, Cause: ErrInvalidQuery}` with ZERO HTTP requests.
- **REQ-ADP-009 (Ubiquitous)** Set `User-Agent: usearch/<version> (+https://github.com/elymas/universal-search)` and `Accept: application/json` on every request. Default Go UA is blocked by Reddit.
- **REQ-ADP-010 (Optional)** WHERE redirect: follow up to 3 hops within `{www.reddit.com, old.reddit.com, new.reddit.com, reddit.com}`; cross-domain → `*SourceError{Permanent}` (SSRF guard).
- **REQ-ADP-011 (State-Driven)** WHILE the same `*Adapter` instance is registered and invoked concurrently from N goroutines (N≥1), each `Search(ctx, q)` call SHALL execute independently with no shared mutable state; cumulative effect = N independent HTTP round-trips, race-detector clean.

## NFRs

- **NFR-ADP-001** `parseListing` median-of-5 mean per-op ≤ 5ms, allocs ≤ 10/doc on 25-doc fixture (amd64). Bench: `go test -bench=BenchmarkParseListing25Docs -benchtime=10x -count=5 ./internal/adapters/reddit/...` (weekly CI).
- **NFR-ADP-002** Stub E2E p95 ≤ 200ms over 100 invocations. Test: `TestSearchE2ELatencyStubP95`.
- **NFR-ADP-003** Zero goroutine leak on ctx cancel. Test: `TestSearchNoGoroutineLeakOnCancel` via `goleak.VerifyNone`.

---

## Acceptance Scenarios (Given/When/Then highlights)

- **S-001-A** Compile-time interface assertion succeeds; remove a method → build fails.
- **S-001-D** Capabilities: SourceID=reddit, DocTypes=[DocTypePost], RequiresAuth=false, RateLimitPerMin=10, DefaultMaxResults=25; Notes contains "public no-auth", "NSFW excluded by default", "t=all", "rate limit discrepancy".
- **S-002-A** 25-doc fixture → 25 NormalizedDocs, all `Validate()` pass, ID starts `t3_`, URL starts `https://www.reddit.com/`, Hash="", Metadata has 6 keys.
- **S-002-B/C/D/E/F** URL has q,sort,t,type,limit,include_over_18; limit clamps to 100; defaults to 25; honours Cursor; omits after when Cursor empty.
- **S-003-A..F** 429 with int Retry-After=30 → RetryAfter=30s; HTTP-date → ±5s of computed; missing → 5s; >60 → 60s; negative/malformed → 5s; exactly 1 request.
- **S-004-A..D** 401/403/404 → ErrPermanent; HTTPStatus matches; 1 request.
- **S-005-A..E** 500/503/connection-refused → ErrSourceUnavailable; HTTPStatus 500/503/0; underlying error preserved via Unwrap; ctx pre-cancelled wrapped.
- **S-006-A..K** Field mapping; snippet ≤283 runes; non-t3 filtered; cursor on last doc only; Hash=""; Metadata 6 keys; [deleted] returned as-is; malformed JSON → Permanent; empty children → (nil,"",nil).
- **S-007-A..E** NSFW filter: true→include_over_18=true; false/absent/unknown→false; mixed-NSFW fixture: 25 docs returned (no client-side post-filter).
- **S-008-A..C** Empty Text or all-`unicode.IsSpace` whitespace → ErrPermanent + 0 requests; NBSP excluded from fixture (not in `unicode.IsSpace`); "x" accepted.
- **S-009-A..D** UA = "usearch/v0.1 (+https://github.com/elymas/universal-search)"; configurable via Options.UserAgentVersion; Accept=application/json; never contains "Go-http-client".
- **S-010-A..D** Allowlist redirect followed; cross-domain rejected with "cross-domain redirect rejected: <host>"; >3 hops rejected; 2-hop within allowlist succeeds.
- **S-011-A** 50 goroutines call Search on shared `*Adapter` against one stub; race-detector clean (`-race`); stub observes 50 requests; every goroutine receives 25 valid `NormalizedDoc`s.
- **N-001..N-003** Bench median-of-5 ≤5ms (`-benchtime=10x -count=5`), allocs ≤250; stub p95 ≤200ms; goleak clean.
- **E-001..E-008** Empty children → success(nil,nil); malformed → Permanent; NSFW mixed; pagination round-trip; SSRF prevented; ctx cancel ≤150ms; 4096-char query passes; score saturates at extremes.

---

## Files to Create (12 source + 6 testdata)

```
internal/adapters/reddit/
├── reddit.go                                # Adapter, New, Name, Capabilities, Healthcheck, var _ types.Adapter = (*Adapter)(nil)
├── reddit_test.go
├── search.go                                # (*Adapter).Search hot path, buildSearchURL, nsfwFilterValue
├── search_test.go                           # E2E + happy + errors + NSFW + URL + ctx + p95 + goleak
├── client.go                                # newDefaultClient, doRequest, redirectAllowlist, allowedRedirectHosts, categorizeStatus
├── client_test.go                           # categorizeStatus, parseRetryAfter, redirect tests, headers
├── parse.go                                 # parseListing, transformPost, redditListing/redditChild/redditPostData structs
├── parse_test.go                            # field mapping table, cursor, filter non-t3, deleted, malformed, empty
├── score.go                                 # normalizeScore (Tanh formula), tanhDivisor=100.0, scoreCenter=0.5
├── score_test.go                            # 7-value table, determinism
├── errors.go                                # ErrInvalidQuery sentinel, parseRetryAfter helper
├── bench_test.go                            # BenchmarkParseListing25Docs
└── testdata/
    ├── search_response.json                 # 25-post happy path (~5KB)
    ├── search_response_empty.json           # children:[] (~500B)
    ├── search_response_pagination.json      # data.after set (~5KB)
    ├── search_response_with_nsfw.json       # mixed NSFW + safe (~5KB)
    ├── search_response_deleted_post.json    # [deleted] author (~2KB)
    └── search_response_malformed.json       # truncated JSON (~200B)
```

**Modified**: none (adapter self-contains).
**Unchanged by design**: `pkg/types/*`, `internal/adapters/registry.go`, `internal/obs/metrics/*`, `cmd/usearch/main.go` (cmd wiring is SPEC-CLI-001's job).

---

## Reddit JSON → NormalizedDoc Mapping (one-liner)

`name→ID`, const→`SourceID="reddit"`, `permalink→URL=("https://www.reddit.com"+permalink)`, `title→Title`, `selftext→Body`, first 280 runes→`Snippet`, `created_utc→PublishedAt=time.Unix(int64(v),0).UTC()`, parse-time→`RetrievedAt`, `author→Author`, `score→Score=normalizeScore(int(score))`, const→`Lang=""`, const→`DocType=DocTypePost`, nil→`Citations`, map→`Metadata{REQUIRED: subreddit, over_18, num_comments, upvote_ratio, external_url, kind; OPTIONAL: subreddit_name_prefixed, ups, spoiler, locked, stickied, link_flair_text, post_hint; REQUIRED on last doc only when data.after!="": next_cursor}`, const→`Hash=""`.

## Score Formula

`Score = clamp(0.5 + 0.5 * math.Tanh(float64(score)/100.0), 0.0, 1.0)`. Worked: -1000→0.0, -10→0.450, 0→0.500, 10→0.550, 100→0.881, 1000→1.0.

## Error Categorisation Table

| HTTP | Category | Notes |
|------|----------|-------|
| 200 | (no error) | Parse body |
| 401/403/404 | Permanent | No retry |
| 429 | RateLimited | Parse Retry-After (cap 60s, default 5s) |
| 500/502/503/504 | Unavailable | No internal retry; FAN-001 owns |
| 0 (network err) | Unavailable | DNS/dial/TLS failure |

## MX Tag Plan

- `(*Adapter).Search` → `@MX:ANCHOR` (sole entry point; fan_in≥3)
- `parseListing` → `@MX:ANCHOR` (every doc passes through)
- `score.go::normalizeScore` (function) and `tanhDivisor=100.0, scoreCenter=0.5` (adjacent constants) → `@MX:NOTE` (Tanh formula choice + empirical inflection-point; SPEC-IDX-001 RRF tie-in)
- `categorizeStatus` → `@MX:NOTE` (HTTP→Category rosetta)
- `doRequest` → `@MX:WARN` (network call; redirect allowlist enforces SSRF safety; `@MX:REASON`)
- `allowedRedirectHosts` map → `@MX:NOTE` (4-entry security boundary)

All tags `[AUTO]`-prefixed, include `@MX:SPEC: SPEC-ADP-001`, English comments.

---

## Exclusions (HARD — do NOT build)

- Per-source customisations for HN, arXiv, GitHub, YouTube, Bluesky, X, SearXNG, Naver, Daum, KoreaNewsCrawler, RSS, Polymarket → SPEC-ADP-002..009.
- Retry orchestration → SPEC-FAN-001 (M3). Adapter is one-shot.
- Response caching → SPEC-CACHE-001 (M3). Adapter stateless.
- Result ranking, dedup, RRF fusion → SPEC-IDX-001 (M3).
- Tenant-scoped NSFW policy → SPEC-AUTH-002 (M6).
- Adapter health-state machine (auto-disable/re-enable) → SPEC-EVAL-002 (M8).
- OAuth-authenticated variant (`oauth.reddit.com`, 60/min) → future SPEC-ADP-001a.
- Subreddit-scoped search (`/r/{sub}/search.json` + `restrict_sr=on`) → out of v0.1.
- Time-range filtering (`t=hour|day|week|month|year`) → out of v0.1; hardcoded `t=all`.
- Sort customisation → out of v0.1; hardcoded `sort=relevance`.
- Comment retrieval (`t1_*` kinds) → out of scope.
- Live network tests in CI → out of v0.1; httptest + golden fixtures only.
- Per-adapter custom Prometheus metrics → would amend OBS-001 allowlist; out of v0.1.
- Korean-locale handling → SPEC-IDX-003 + SPEC-ADP-008/009 own; Reddit Lang="" (unknown).
- Streaming Search → SPEC-SYN-004 (M4).
- LLM calls in adapter → IR-001 owns classification.
- Drive-by edits to `pkg/types/*`, `internal/adapters/registry.go`, `internal/obs/*` are FORBIDDEN.

---

## Quality Gates (Definition of Done)

- [ ] `go test ./internal/adapters/reddit/...` exits 0
- [ ] `go test -race` clean
- [ ] Coverage ≥ 85%
- [ ] `go vet` clean
- [ ] `golangci-lint run` clean
- [ ] `BenchmarkParseListing25Docs` invoked `-benchtime=10x -count=5`; median ≤ 5ms; allocs/op ≤ 250
- [ ] `TestSearchE2ELatencyStubP95` passes
- [ ] `TestSearchNoGoroutineLeakOnCancel` passes
- [ ] All MX tags applied per §6.7
- [ ] `Capabilities.Notes` contains 4 documented substrings
- [ ] HISTORY appended with implementation commit hash

---

## Dependencies

**Upstream (HARD)**: SPEC-CORE-001 (implemented), SPEC-OBS-001 (implemented). **Soft**: SPEC-IR-001 (implemented; consumes Capabilities).
**Downstream (blocked)**: SPEC-ADP-002 (HN; copies shape), SPEC-FAN-001 (M3; consumes via registry), SPEC-CLI-001 (M2; wires into cmd), SPEC-SYN-001 (M2; consumes []NormalizedDoc).
**External**: zero new Go module deps; pure stdlib + `pkg/types` + `internal/obs/reqid`. Test-only: `go.uber.org/goleak` (NEW; add via `go get`).

## Open Questions Carried Forward (6)

1. Rate-limit doc discrepancy (10/min vs 60/min in tech.md) → adopt 10/min; sync tech.md follow-up.
2. Score formula revisit → keep Tanh divisor 100; revisit after SPEC-IDX-001 RRF.
3. Healthcheck depth → TCP-connect; SPEC-EVAL-002 may upgrade.
4. NSFW filter empirical verification → trust Reddit in v0.1; SPEC-AUTH-002 may post-filter.
5. `[deleted]` post handling → return as-is; SPEC-SYN-001 may filter at synthesis.
6. Cursor surfacing → Metadata["next_cursor"] on last doc; SPEC-FAN-001 may request wrapper.

(Original §11.7 "Metadata key API surface" was resolved in iteration 2 by the §6.3 REQUIRED/OPTIONAL classification; no longer open.)

---

*End of SPEC-ADP-001 spec-compact.md v0.1*
