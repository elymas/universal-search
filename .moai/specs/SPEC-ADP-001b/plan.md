# Implementation Plan — SPEC-ADP-001b: Reddit RSS Adapter

## Technical Approach

Create a new package `internal/adapters/reddit_rss/` (Go package name
`redditrss`) that implements `types.Adapter` with `Name()="reddit-rss"`. It is a
near-mirror of the single-feed slice of `internal/adapters/koreanews/rss.go`,
specialised to one feed URL (the global `search.rss` endpoint). It reuses
`github.com/mmcdole/gofeed` for parsing and `*types.SourceError` for error
classification. It shares no code with the OAuth `internal/adapters/reddit/`
package.

### Key files (new)

- `internal/adapters/reddit_rss/reddit_rss.go` — `Adapter` struct, `New`,
  `Name`, `Capabilities`, `Healthcheck`, compile-time `var _ types.Adapter`.
- `internal/adapters/reddit_rss/options.go` — `Options` (`BaseURL`, `UserAgent`,
  `Timeout`, optional `HTTPClient`, `NowFunc`, `UserAgentVersion`) +
  `applyDefaults`.
- `internal/adapters/reddit_rss/search.go` — `Search`, URL building
  (`q`/`sort`/`t`), gofeed parse, `feedItemsToDocs` mapping, status→Category.
- `internal/adapters/reddit_rss/client.go` — default `*http.Client` with timeout
  and redirect allowlist constrained to `www.reddit.com`; default User-Agent.

### Key files (modified — single file)

- `internal/pipeline/pipeline.go` — add one `reg.Register(a)` block for
  `redditrss.New(redditrss.Options{BaseURL: os.Getenv("REDDIT_RSS_BASE_URL")})`
  in the non-credentialed section, next to the `hn`/`arxiv`/`searxng`/`koreanews`
  entries. No change to the credentialed (Resolver) section.

### Reuse map (cite in code comments)

| Concern | Source pattern to mirror |
|---|---|
| Adapter contract | `pkg/types/adapter.go` (4 methods) |
| gofeed parse + per-call timeout + NormalizedDoc mapping | `internal/adapters/koreanews/rss.go` (`fetchFeed`, `feedItemsToDocs`) |
| HTML stripping / snippet truncation | `internal/adapters/koreanews/strip.go` (replicate or extract minimal helpers; do NOT import koreanews internals) |
| Custom User-Agent + redirect allowlist + status→Category | `internal/adapters/reddit/client.go` |
| Capabilities/Healthcheck shape | `internal/adapters/koreanews/koreanews.go` |
| Error Category names | `pkg/types/errors.go` (5-value enum; no Timeout/Network) |
| Registration block | `internal/pipeline/pipeline.go` non-credentialed section |
| Source visibility | `cmd/usearch/sources_cmd.go` (no change needed) |

## Milestones (priority-ordered, no time estimates)

1. **M1 — Options + construction (P0).** `Options`, `applyDefaults`, `New`,
   `Name`, default client with `www.reddit.com` redirect allowlist and default
   User-Agent. Compile-time `types.Adapter` assertion. (REQ-ADP1B-001..004, 017)
2. **M2 — Search mapping (P0).** URL builder (`q`+`sort=relevance` only; v0.1
   emits no `t=` time-window param), gofeed parse, `feedItemsToDocs` →
   `NormalizedDoc` with `SourceID="reddit-rss"`, `DocTypePost`, neutral constant
   Score 0.5. Empty-link item skip. (REQ-ADP1B-005, 007, 008, 009)
3. **M3 — Error + cancellation mapping (P0).** ctx-cancel guard, status→Category
   (429→RateLimited, 403→Unavailable, other 4xx→Permanent, 5xx/network→Unavailable,
   parse→Transient), empty-query rejection. (REQ-ADP1B-010..016, incl. 012a)
4. **M4 — Capabilities + Healthcheck (P1).** Deterministic Capabilities
   (`RequiresAuth=false`), lightweight Healthcheck probe. (REQ-ADP1B-018, 019)
5. **M5 — Registry wiring (P1).** Always-on `reg.Register` block in pipeline with
   `REDDIT_RSS_BASE_URL` override; verify `sources status` shows it.
   (REQ-ADP1B-020, 021)
6. **M6 — Tests + coverage (P0).** Table-driven unit tests with loopback
   `httptest` server + canned `search.rss` fixture; reach >=85% coverage.

## Technical Approach Notes

- Single feed → no `errgroup`. Use `gofeed.NewParser()` with `fp.UserAgent` set,
  then `fp.ParseURLWithContext(url, ctx)`. To map non-2xx status to specific
  Categories (gofeed alone surfaces a generic parse/HTTP error), perform an
  explicit `http.NewRequestWithContext` + `client.Do` first, classify the status
  via a `categorizeStatus` helper (mirror `reddit/client.go`), and only feed the
  2xx body into `gofeed.Parser.Parse(resp.Body)`. This gives precise
  429/4xx/5xx mapping that `ParseURLWithContext` cannot.
- Per-request timeout = `min(opts.Timeout, time-until-ctx-deadline)`, mirroring
  `koreanews/rss.go fetchFeed`.
- `NowFunc` option enables deterministic `RetrievedAt` in tests.

## Risks and Mitigations

- See spec.md Risks R1–R3. Additionally: gofeed's `ParseURLWithContext` hides the
  HTTP status; the explicit-request-then-Parse approach (above) is required to
  satisfy REQ-ADP1B-011..015 — flag this to the implementer so they do not take
  the simpler `ParseURLWithContext` shortcut and lose status fidelity.

## Out of Scope

See spec.md "Exclusions (What NOT to Build)".
