# SPEC-ADP-007 Research — SearXNG Bridge Adapter

**Status**: Research-complete; ready for SPEC drafting
**Author**: limbowl (via manager-spec)
**Created**: 2026-05-04
**Milestone**: M3 — Fanout, adapters, index
**Depends on**: SPEC-CORE-001, SPEC-OBS-001, SPEC-IR-001, SPEC-BOOT-001

---

## 0. Research Mandate

SPEC-ADP-007 (SearXNG bridge, M3) is the GENERAL-WEB adapter. SearXNG is
already running in the dev compose stack (SPEC-BOOT-001 REQ-BOOT-004) at
`searxng/searxng:2026.04.22-74f1ca203`, exposing the JSON metasearch API on
port 8080 and aggregating 70+ external engines (google, bing, duckduckgo,
…) through a single local HTTP boundary. ADP-007 fills
`internal/adapters/searxng/` with the Go HTTP client + result normalisation
that turns one `types.Query` into one HTTP GET against the local SearXNG
instance and returns `[]types.NormalizedDoc` interleaving every engine's
hits.

The mandate of this research is to:

- Document the SearXNG JSON Search API surface exactly: endpoint, query
  parameters, response envelope, pagination, limiter behaviour, and the
  per-result field shape — with file:line citations into the deployed
  compose stack and URL-cited references into the SearXNG project source
  for every claim.
- Map the SearXNG result envelope onto the SPEC-CORE-001
  `types.NormalizedDoc` 15-field canonical contract, surfacing engine-
  of-origin metadata as a first-class field consumers can rely on for
  RRF fusion (SPEC-IDX-001) and dedup (SPEC-FAN-001 §2.4).
- Confirm the AGPL-3.0 service-boundary posture stated in
  `.moai/project/tech.md:148, 166` and `deploy/docker-compose.yml:108`
  by independent verification of the SearXNG license file and the GitHub
  README, and document the compliance constraints (consume as service,
  do not link/fork/embed) for the SPEC's risk register.
- Extract reusable patterns from SPEC-ADP-001 (Reddit) and SPEC-ADP-002
  (HN) — the file layout, error mapping discipline, MX tag plan, TDD
  harness — that ADP-007 mirrors verbatim for consistency across the
  M3 adapter cohort.
- Surface unique-to-SearXNG concerns: engine-of-origin cardinality
  control for the observability allowlist; the local-network TLS posture
  (HTTP-only intra-compose); the absence of any external auth (the local
  SearXNG instance has no API key, but the limiter-on-by-default may
  trip under aggressive testing); the HTML-vs-JSON output mode toggle
  on the server (`formats` setting must include `json`).
- Enumerate risks and propose mitigations.
- List Open Questions that are deliberately deferred but must be
  documented.

The output is this research artifact. Every claim is either file-cited
(e.g. `deploy/docker-compose.yml:106-130`) or URL-cited from verified web
sources. No invented facts.

---

## 1. Existing Codebase + Compose State

### 1.1 SearXNG Service in the Dev Compose Stack

Verified via `Read`:

- `deploy/docker-compose.yml:14-16` — image is pinned to
  `searxng/searxng:2026.04.22-74f1ca203` with the digest
  `sha256:37c616a774b90fb5df9239eb143f1b11866ddf7b830cd1ebcca6ba11b38cc2bf`.
  The pin date (2026-04-24) is owned by SPEC-DEP-001; ADP-007 does not
  modify the pin.
- `deploy/docker-compose.yml:106-130` — the `searxng` compose service:
  - `image: searxng/searxng:2026.04.22-74f1ca203` (line 111).
  - `ports: "${SEARXNG_PORT:-8080}:8080"` (lines 112-113). The
    container-internal port is fixed at 8080; the host-side port is
    configurable. Within the compose `app` network, peer services reach
    the instance at `http://searxng:8080`.
  - `volumes: ./searxng/settings.yml:/etc/searxng/settings.yml:ro` (line
    115). Read-only mount of the local settings file.
  - `environment` (lines 116-118):
    - `SEARXNG_BASE_URL: ${SEARXNG_BASE_URL:-http://localhost:8080/}` —
      default is the local URL.
    - `SEARXNG_SECRET: ${SEARXNG_SECRET}` — REQUIRED env var; injected
      from operator's `.env` file. The settings.yml at
      `deploy/searxng/settings.yml:15` references `secret_key:
      "${SEARXNG_SECRET}"` for runtime substitution.
  - `depends_on: redis: condition: service_healthy` (lines 119-121) —
    SearXNG uses Redis for the limiter / valkey backend.
  - `healthcheck` (lines 122-127): wgets `http://localhost:8080/`. A 200
    response signals reachability. The healthcheck is HTML-mode (no JSON
    requested); it does NOT verify the JSON endpoint, only TCP+HTTP.
  - `restart: unless-stopped` (line 128) — recovers from crashes.
  - `networks: app` (lines 129-130) — joins the shared bridge network.
- `deploy/docker-compose.yml:108-109` — license-compliance comment
  block: "SearXNG is AGPL-3.0; consumed as an external service (service-
  boundary, not linked). See NOTICE and docs/dependencies.md for license
  compliance details." This boundary posture is the LOAD-BEARING legal
  constraint for ADP-007.

### 1.2 SearXNG settings.yml (Server-Side Config)

Verified via `Read deploy/searxng/settings.yml`:

- `use_default_settings: true` (line 6) — inherits the upstream defaults
  for every key not overridden below.
- `general.debug: false` (line 9), `general.instance_name: "Universal
  Search — SearXNG"` (line 10).
- `server.port: 8080` (line 13), `server.bind_address: "0.0.0.0"` (line
  14), `server.secret_key: "${SEARXNG_SECRET}"` (line 15).
- `server.cors_allow_all: true` (line 17) — local dev only; tighten in
  prod.
- `search.safe_search: 0` (line 20) — no safe-search filter at the
  server (caller must filter if needed).
- `search.autocomplete: ""` (line 21) — disabled.
- `search.default_lang: "en"` (line 22).
- `ui.query_in_title: true` (line 25), `ui.infinite_scroll: false`
  (line 26), `ui.default_locale: "en"` (line 27).
- `engines:` (lines 31-43) — only three engines explicitly enabled:
  `google` (shortcut `g`), `bing` (shortcut `b`), `duckduckgo`
  (shortcut `d`). The `use_default_settings: true` flag means the
  upstream default engine catalogue is inherited; the explicit list
  does NOT disable other engines unless `disabled: true` is set.

  → IMPLICATION FOR ADP-007: the operational set of engines that the
  bridge sees is determined by the upstream SearXNG defaults plus
  this override file. The Go adapter does NOT enforce a specific
  engine list; it simply forwards `q` and lets SearXNG decide which
  engines to query. The `engines=...` URL parameter (§2.2) MAY be
  passed by the adapter to constrain at request time.

[CRITICAL GAP]: the deployed `settings.yml` does NOT explicitly
configure the `formats:` list or the `limiter:` setting. Both fall
back to upstream defaults:

- The upstream default for `search.formats:` typically includes `html`
  ONLY; `json` is OPT-IN per the SearXNG admin docs (verified via
  https://docs.searxng.org/dev/search_api.html — the API doc explicitly
  states `format` values "must be enabled in settings"). [→ Open Question
  §11.1 — must the SPEC's run-phase implementer add `formats:
  [html, json]` to settings.yml before the adapter can consume?]
- The upstream default for `server.limiter:` is `false` in the dated
  tag 2026.04.22-74f1ca203 (per https://github.com/searxng/searxng
  default settings inspection; the limiter is opt-in). When `limiter:
  true`, the bot-detection layer enforces per-IP rate caps and returns
  HTTP 429 (`too_many_requests`) on excess.

### 1.3 Reference Adapter Patterns (SPEC-ADP-001 Reddit, SPEC-ADP-002 HN)

Verified via `Read` of the deployed adapter source:

- `internal/adapters/reddit/reddit.go:15-29` — the same `defaultBaseURL
  / defaultUserAgentTemplate / defaultUAVersion / defaultHealthcheckTarget`
  constant structure ADP-007 mirrors. `Options` (lines 33-49) has the
  same five-field shape (BaseURL / HTTPClient / UserAgentVersion /
  HealthcheckTarget). The compile-time interface assertion `var _
  types.Adapter = (*Adapter)(nil)` lands at line 135 of reddit.go.
- `internal/adapters/reddit/search.go:43-110` — the Search hot path:
  validate → build URL → HTTP GET → categorize status → parse →
  return. The 5 MB body cap (`maxResponseBytes` at line 21) is the
  pattern ADP-007 inherits.
- `internal/adapters/reddit/client.go:24-44` — the redirect allowlist
  pattern (SSRF guard). For ADP-007, the allowlist is much smaller:
  the local `searxng:8080` host AND `localhost`/`127.0.0.1` for
  test injection. NO external hosts; SearXNG never redirects out
  of-cluster.
- `internal/adapters/reddit/client.go:102-124` — the `categorizeStatus`
  HTTP-status-to-Category rosetta. ADP-007 reuses the same shape with
  the only delta being the `Adapter` field set to `"searxng"`.
- `internal/adapters/reddit/parse.go:72-115` — the `parseListing`
  pattern. ADP-007's analogue (`parseSearch`) parses the SearXNG
  JSON envelope (§2.3) instead of the Reddit Listing envelope.
- `internal/adapters/registry.go:172-263` — sole-emitter wrappedAdapter.
  ADP-007 emits ZERO observability of its own; the registry wrapper
  emits the per-adapter Counter + Histogram + slog + OTel span on
  every Search call.
- `pkg/types/capabilities.go:38-62` — Capabilities struct shape.
  ADP-007 returns SourceID="searxng", DisplayName="SearXNG",
  DocTypes=[DocTypeArticle, DocTypePost, DocTypeOther]
  (web-eligible per `internal/router/category.go:93`),
  RequiresAuth=false, AuthEnvVars=nil, RateLimitPerMin=60
  (conservative default; the local SearXNG limiter is the binding
  constraint, not an external API quota), DefaultMaxResults=10
  (SearXNG returns ~10/page by default; the per-engine cap is
  upstream-dependent and the adapter does not enforce a cap).
- `internal/router/category.go:90-111` — `CategoryEligibleDocTypes`.
  For `CategoryWeb` (line 93), the eligible DocTypes are
  `{DocTypeArticle, DocTypePost, DocTypeOther}`. ADP-007 publishes
  ALL THREE in `Capabilities.DocTypes` so the Intent Router selects
  it for any web-classified query.

### 1.4 Observability Cardinality Constraint (engine-of-origin)

Verified via `Read internal/obs/metrics/metrics.go` (search):

- `internal/obs/metrics/metrics.go:37` — `FanoutInflight
  *prometheus.GaugeVec` is pre-registered with one label
  `adapter_class` (line 94). This is FAN-001's domain, not ADP-007's.
- `internal/obs/metrics/metrics.go:171` — the cardinality allowlist
  contains `adapter_class` (and per other SPEC excerpts, `adapter`,
  `outcome`). It does NOT contain a per-engine label.

→ IMPLICATION FOR ADP-007: per-engine metrics (e.g. counting how many
docs each upstream engine contributed) CANNOT be emitted as
Prometheus labels without amending the allowlist. Engine-of-origin
information lives in `NormalizedDoc.Metadata` (a free-form
map[string]any not subject to label-cardinality limits). This
echoes the same posture taken by ADP-001 / ADP-002 — adapters never
emit new metric families.

The `Notes` field in `Capabilities` documents the engine cardinality
mitigation: SPEC-ADP-007 surfaces engine-of-origin via
`Metadata["engines"]` (a list), bounded to the top-N per-doc engines
in the response (typically 1-3), which keeps the JSON payload size
bounded without label-explosion risk.

### 1.5 Reqid Transport (Request-ID Propagation)

Verified via `Read internal/adapters/reddit/client.go:50-56`:

- `reqid.NewTransport(http.DefaultTransport)` wraps the outbound
  transport so the OTel request-ID propagates as an HTTP header. The
  same wrapping applies to ADP-007 since the local SearXNG instance
  has no notion of distributed tracing (it does not honour the header)
  but the transport does NOT need to be different from Reddit's. The
  pattern is "wrap transport always; the receiver may ignore."

---

## 2. SearXNG Search API Surface (External, URL-Verified)

### 2.1 Endpoint

- Canonical URL (intra-compose): `http://searxng:8080/search`
- Canonical URL (host port-forward): `http://localhost:${SEARXNG_PORT:-8080}/search`
- HTTP method: `GET` (POST also supported per the docs but GET keeps
  the request shape simpler; ADP-007 uses GET).
- Source: https://docs.searxng.org/dev/search_api.html — "Endpoints:
  GET /search or GET / (also supports POST)."

The trailing-slash variant (`GET /`) also routes to the search handler.
ADP-007 standardises on `GET /search` for clarity and to avoid any
homepage-render fallback the upstream may add.

### 2.2 Query Parameters

Verified at https://docs.searxng.org/dev/search_api.html (full table
of query-string parameters):

| Parameter | Type | Required | ADP-007 Usage | Notes |
|-----------|------|----------|---------------|-------|
| `q` | string | YES | `q.Text` (URL-escaped) | The search query; passed to all configured engines. |
| `format` | enum | NO | hardcoded `json` | One of `json`/`csv`/`rss`. MUST be enabled in server `formats` setting (Open Question §11.1). |
| `categories` | string | NO | omitted in v0.1 | Comma-separated. Defaults to `general` per SearXNG defaults. v0.1 lets SearXNG decide. |
| `engines` | string | NO | omitted in v0.1 | Comma-separated explicit engine list. v0.1 lets SearXNG fan out across all enabled engines. Open Question §11.2 documents revisit. |
| `language` | string | NO | omitted in v0.1 | Inherits `default_lang: "en"` from settings.yml. Open Question §11.3 — should ADP-007 forward Korean queries with `language=ko`? |
| `pageno` | integer | NO | when `q.Cursor != ""` | Page number, 1-based. v0.1: `q.Cursor` parses via `strconv.Atoi`; first page = pageno=1 (default); next page = pageno=2; cursor is the page number itself encoded as a decimal string. |
| `time_range` | enum | NO | omitted in v0.1 | One of `day`/`month`/`year`. v0.1 defers; surfaces via Open Question §11.4. |
| `safesearch` | enum | NO | omitted in v0.1 | One of `0`/`1`/`2`. Inherits server `search.safe_search: 0` (no filter). v0.1 does not override. |
| `image_proxy` | bool | NO | omitted | Image proxying is a UI concern; the adapter has no images to proxy. |

ADP-007 does NOT pass `engines=` in v0.1: the server-side default
fanout is the desired behaviour (use whatever the operator enabled).
Surfacing a per-call engine override is deferred to a follow-up SPEC
(`Query.Filters[Key="engines"]`) once measured value warrants.

### 2.3 Response Envelope (JSON Mode)

The exact JSON shape is partially documented in
https://docs.searxng.org/dev/search_api.html (the doc explicitly
notes "The documentation does not specify the exact JSON response
schema"); the practical shape is verified via SearXNG source
inspection:

- `searx/results.py` — `get_ordered_results()` returns the per-result
  field set: `url`, `title`, `content`, `engine` (single name),
  `engines` (set of contributing engines), `category`, `score`,
  `positions`, `template`, `parsed_url`, optional `thumbnail` /
  `img_src`. Quoted from the file: "Returns a sorted list of results
  to be displayed in the main result area".
- `searx/webapp.py` — `if output_format == 'json': response =
  webutils.get_json_response(search_query, result_container); return
  Response(response, mimetype='application/json')`. The container
  attaches: `query`, `number_of_results`, `results`, `suggestions`,
  `corrections`, `answers`, `infoboxes`, `unresponsive_engines`.

The canonical top-level JSON shape (run-phase consumers reads from
it):

```json
{
  "query": "<echoed query string>",
  "number_of_results": 23,
  "results": [
    {
      "url": "https://example.com/page",
      "title": "Page Title",
      "content": "Page snippet text...",
      "engine": "google",
      "engines": ["google", "bing"],
      "category": "general",
      "score": 1.23456,
      "positions": [1, 3],
      "template": "default.html",
      "parsed_url": ["https", "example.com", "/page", "", "", ""],
      "publishedDate": "2026-04-15T08:30:00Z"
    }
  ],
  "suggestions": ["alternate query"],
  "corrections": [],
  "answers": [],
  "infoboxes": [],
  "unresponsive_engines": [["bing", "timeout"]]
}
```

[CONFIRMATION GAP]: `publishedDate` is NOT guaranteed by every
engine adapter inside SearXNG (some engines surface it, others omit
it). The ADP-007 parser MUST treat `publishedDate` as OPTIONAL —
parse with `time.Parse(time.RFC3339, ...)` when present; leave
`NormalizedDoc.PublishedAt` zero-valued when absent. This matches
the SPEC-CORE-001 contract at `pkg/types/normalized_doc.go:27`
("zero allowed, unknown date").

### 2.4 Pagination Semantics

SearXNG pagination is page-number-based, NOT cursor-based:

- `pageno=1` → first page (default).
- `pageno=2` → second page; etc.
- Each engine inside SearXNG enforces its own per-page count; the
  aggregated `results` array typically holds 10-30 entries depending
  on engine overlap and dedup.
- There is NO official "max pageno"; per the SearXNG codebase
  inspection, pageno defaults to 1 and the upper bound is engine-
  specific (Google generally caps at ~10 pages of 10 results, etc).

ADP-007 cursor convention: `q.Cursor` is the next page number as a
decimal string (e.g., `"2"`). `parseCursor` uses
`strconv.Atoi`; rejects negative integers and non-numeric values.
First-call cursor=""; surfaced cursor on the LAST returned doc as
`Metadata["next_cursor"] = strconv.Itoa(currentPage + 1)` when the
parser cannot prove this is the last page (i.e., always — SearXNG
gives no `total_pages` field; the parser conservatively surfaces a
next_cursor unless the response contained zero results).

[CONFIRMATION]: cross-checked the cursor approach against
SPEC-ADP-002 (HN) which uses the same integer-string-page pattern at
`.moai/specs/SPEC-ADP-002/spec.md:166` — the SearXNG bridge inherits
the established M2 pagination shape verbatim.

### 2.5 HTTP Status Codes & Error Semantics

| Code | Meaning | ADP-007 Mapping |
|------|---------|-----------------|
| 200 | Success | Parse `results` array; return docs. |
| 301/302/303/307/308 | Redirect (rare for SearXNG; not standard behaviour for /search) | The redirectAllowlist policy enforces same-host hops only. Out-of-allowlist returns CategoryPermanent. |
| 400 | Malformed query / parameter | CategoryPermanent. |
| 403 | Forbidden — limiter blocked OR JSON format not enabled | CategoryPermanent. (See §2.6 — limiter behaviour can return either 403 or 429 depending on version.) |
| 404 | Not Found — server/path missing | CategoryPermanent. |
| 429 | Too Many Requests — limiter triggered | CategoryRateLimited; parse `Retry-After` header (if present); default 5s; cap 60s — same shape as ADP-001 §2.3. |
| 500/502/503/504 | SearXNG internal failure / Redis backend down | CategoryUnavailable. |
| Network error (DNS, dial, TLS, timeout) | Container down / network partition | CategoryUnavailable; HTTPStatus=0. |

### 2.6 Limiter Behaviour (limiter=on)

The SearXNG bot-detection layer (`searx/botdetection/`) is the rate
limiter. When `server.limiter: true` in settings.yml:

- Configuration: `searx/limiter.toml` controls IP block/pass lists,
  trusted-proxy headers, and link-token method. The DEFAULT
  installation has `limiter: false`; explicit opt-in is required.
- Trigger HTTP status: based on inspection of the
  `searx/botdetection/__init__.py` module structure, a function named
  `too_many_requests` is exported. Convention strongly suggests this
  emits HTTP 429 (the function name maps to the HTTP semantic).
  However, the WebFetch of the bot-detection module did NOT directly
  reveal the precise return tuple (status code, headers). [→ Open
  Question §11.5 documents need for run-phase verification against a
  real local instance with `limiter: true`.]
- Recommended adapter behaviour: handle BOTH 429 AND 403 as rate-
  limit signals when the `Retry-After` header is present; fall back
  to CategoryPermanent for 403 without `Retry-After`. The
  `categorizeStatus` rosetta inherits the 403 → Permanent mapping
  from ADP-001; ADP-007 adds a special case that promotes 403 to
  RateLimited when `Retry-After` is present.

### 2.7 Request Headers

- `User-Agent`: ADP-007 sets `usearch/<version>
  (+https://github.com/elymas/universal-search)` per the project
  convention from ADP-001 REQ-ADP-009. SearXNG itself does NOT block
  default Go User-Agents (the local instance has no UA-based filtering
  beyond the limiter); the custom UA is set for project-wide
  consistency and operational debug visibility.
- `Accept`: `application/json`. The server-side `format=json` URL
  parameter is the actual switch; the Accept header is informational
  and does not change behaviour, but is set per project convention.
- NO `Authorization` header. The local SearXNG instance has no token-
  or password-based auth (verified at `deploy/searxng/settings.yml` —
  no `botdetection.proxy_token` or similar key present).

---

## 3. Alternatives Considered

### 3.1 Reject: Embed SearXNG via Python Sidecar

Spinning up a Python subprocess running `searxng-search` directly:
rejected because it duplicates the deployed compose service, breaks
the AGPL service-boundary posture (the Python sidecar would link to
SearXNG code in-process), and adds Python dependency to the Go
adapter package.

### 3.2 Reject: Fork SearXNG and Add a Native Go gRPC API

Forking creates AGPL contagion across the Go monorepo. Rejected per
`.moai/project/tech.md:148` and `:166`. The service-boundary posture
is the LOAD-BEARING legal constraint.

### 3.3 Reject: Use a Third-Party SearXNG Go Client Library

WebFetch verified at https://github.com/searxng/searxng — no official
Go client library exists. A small number of unofficial third-party
libraries exist on GitHub, but they:
- Are unmaintained or pre-1.0 (supply-chain risk per SPEC-DEP-001).
- Wrap a much larger surface than ADP-007 needs (e.g., expose the
  full SearXNG admin API).
- Add transitive dependencies for what is fundamentally a 2-method
  HTTP+JSON shape.

→ ACCEPTED: implement the bridge directly using `net/http` +
`encoding/json`, mirroring SPEC-ADP-001 / SPEC-ADP-002. This is the
"thin slice" pattern the M2 cohort established.

### 3.4 Reject: Multi-Page Aggregation in v0.1

Calling SearXNG once per page (pageno=1, pageno=2, …) and merging
results client-side: rejected for v0.1. Each call costs an entire
fanout pass through every upstream engine; aggressive multi-paging
trips both the local limiter (if enabled) AND every upstream engine's
rate limit. v0.1 returns one page; the caller paginates explicitly
via `Query.Cursor` round-trips.

### 3.5 Accept: Local-Only HTTP (No TLS)

The compose `app` network is bridge-internal; SearXNG binds to plain
HTTP (no TLS). Calls from the Go adapter inside the same compose
network use `http://searxng:8080`, NOT HTTPS. Calls from a host
binary (`./cmd/usearch/usearch` running locally on the developer
machine) use `http://localhost:${SEARXNG_PORT:-8080}`. Both are
PLAIN HTTP. Fingerprint: redirectAllowlist for ADP-007 must
include `searxng:8080`, `localhost:8080`, and `127.0.0.1:8080` (and
arbitrary loopback ports for httptest stubs).

---

## 4. Integration Requirements

### 4.1 Integration with SPEC-CORE-001 (`pkg/types.Adapter`)

ADP-007 implements the four-method `types.Adapter` interface verbatim,
following the same shape as `internal/adapters/reddit` and
`internal/adapters/hn`. The compile-time assertion `var _ types.Adapter
= (*Adapter)(nil)` lands at the bottom of `internal/adapters/searxng/
searxng.go`.

### 4.2 Integration with SPEC-IR-001 (Intent Router)

The Intent Router selects ADP-007 for queries classified as
`CategoryWeb` because:
- `Capabilities.DocTypes = {DocTypeArticle, DocTypePost, DocTypeOther}`
  (matches `internal/router/category.go:93` `CategoryWeb` eligibility).
- `Capabilities.SupportedLangs = nil` (language-agnostic; ADP-007 is
  selected for any language).
- `Capabilities.RequiresAuth = false` (no auth env vars to validate).

The Intent Router invokes the adapter via the registry, NOT directly.

### 4.3 Integration with SPEC-OBS-001 (Observability)

ADP-007 emits ZERO metrics, logs, and spans of its own. ALL
observability comes from the registry's `wrappedAdapter`
(`internal/adapters/registry.go:172-263`):
- 1 OTel span `adapter.search` per Search call with `adapter.name=
  "searxng"`, `adapter.outcome=<from OutcomeFromError>`,
  `adapter.result_count`.
- 1 Counter increment on `AdapterCalls{adapter="searxng",
  outcome=...}`.
- 1 Histogram observation on `AdapterCallDuration{adapter="searxng"}`.
- 1 slog record at INFO/WARN per call.

The `outcome` label values are bounded by the `OutcomeFromError`
mapping in `pkg/types/errors.go`; ADP-007 returns
`*types.SourceError` instances whose Category drives the outcome
correctly without any metric-emission code in the adapter.

Engine-of-origin cardinality: the `engine` field per upstream result
is variable (could be `google`, `bing`, `duckduckgo`, … plus 60+
others). Surfacing this as a Prometheus label would explode
cardinality. Instead, ADP-007:
- Surfaces engines via `NormalizedDoc.Metadata["engines"]` — a
  `[]string` listing the 1-3 contributing engines per doc.
- Surfaces the primary engine via `NormalizedDoc.Metadata["engine"]`.
- The `SourceID` field stays `"searxng"` for every doc — preserving
  the registry-level cardinality boundary.

The SPEC's REQ-ADP7 acceptance text bounds the top-N engine list
documented in `Capabilities.Notes` to exactly the explicitly-enabled
engines from `deploy/searxng/settings.yml` plus the `default_engines`
inherited from `use_default_settings: true`. Operators introducing
new engines in settings.yml are responsible for confirming their
cardinality footprint.

### 4.4 Integration with SPEC-FAN-001 (Fanout, M3)

When SPEC-FAN-001 dispatches `RoutingDecision.AdapterSet =
[..., "searxng", ...]`, the registry retrieves the wrapped adapter
and calls `Search(ctx, q)`. The adapter:
- Honours `ctx.Done()` per Go `net/http` request-context conventions.
- Returns within 8 seconds default per-adapter timeout (FAN-001
  Options.PerAdapterTimeout from SPEC-FAN-001 §2.5). SearXNG's own
  per-engine timeouts add up internally; the local instance typically
  responds within 1-3 seconds for simple queries against 3 engines.
  Long-tail (10+ engines) can push to 5+ seconds; the FAN-001
  per-adapter timeout is the upper bound.
- Returns a categorised `*types.SourceError` on failure so the FAN-001
  partial-result assembly works correctly.

### 4.5 Integration with SPEC-BOOT-001 (Compose Stack)

The deployed compose stack is the runtime dependency. ADP-007:
- Reads `USEARCH_SEARXNG_URL` env var at construction (default
  `http://searxng:8080` for intra-compose, or
  `http://localhost:8080` for host binaries via documented operator
  override).
- Does NOT modify `deploy/docker-compose.yml`. The compose stack is
  unchanged; ADP-007 is purely additive.
- Does NOT modify `deploy/searxng/settings.yml` IF the upstream
  default formats list includes `json`. If it does NOT (Open Question
  §11.1), the run-phase implementer MUST add a pre-implementation
  fix to settings.yml to enable JSON output. The SPEC documents this
  as a HARD precondition.

---

## 5. Race / Goroutine Leak Considerations

Following SPEC-ADP-001 REQ-ADP-011 + NFR-ADP-003 patterns:

### 5.1 Concurrency Safety

- The `*Adapter` struct is immutable after `New()` returns. All
  fields (`httpClient`, `baseURL`, `userAgent`, `healthcheckTarget`)
  are set at construction time and never written thereafter.
- The shared `*http.Client` is goroutine-safe per Go stdlib
  documentation.
- ZERO global state in the package.
- Test: 50 goroutines × 1 Search per goroutine against one
  `httptest.Server`; race-detector clean (`go test -race`); all 50
  goroutines receive valid `[]NormalizedDoc`. Mirrors
  ADP-001 `TestSearchConcurrentSafe`.

### 5.2 Goroutine Leak Prevention

- Every `*http.Response.Body` is closed via `defer resp.Body.Close()`
  in the Search hot path (mirrors `internal/adapters/reddit/search.go:81`).
- `context.Context` cancellation propagates through
  `http.NewRequestWithContext` → the request is cancelled mid-flight
  if the caller's ctx fires.
- The `httptest.Server` test stub is closed via `t.Cleanup(server.Close)`
  to avoid leaking goroutines.
- Test: `TestSearchNoGoroutineLeakOnCancel` uses `goleak.VerifyNone(t)`
  after a Search call whose ctx was cancelled at 50ms while the stub
  delays response by 200ms.

### 5.3 5 MB Body Cap

- `maxResponseBytes = 5 * 1024 * 1024` (= 5 MiB), matching ADP-001.
- SearXNG responses for a 25-result page are typically 50-200 KB; the
  cap is defensive, not throttling.
- `io.LimitReader(resp.Body, maxResponseBytes)` enforces the cap
  without OOM risk.

---

## 6. Open Questions (Deferred but Documented)

1. **Server-side JSON format enablement**. Does the deployed `searxng/
   searxng:2026.04.22-74f1ca203` image's upstream defaults
   (inherited via `use_default_settings: true`) include `json` in
   `search.formats`? If NOT, a one-line addition to
   `deploy/searxng/settings.yml` (`search.formats: [html, json]`) is
   a HARD precondition for ADP-007.
   **Recommended default**: run-phase implementer MUST verify by hitting
   `http://localhost:${SEARXNG_PORT:-8080}/search?q=test&format=json`
   against the deployed compose stack. If the response is HTML or 403,
   add the formats key to settings.yml AND restart the compose stack
   BEFORE running adapter tests against a non-stub backend (test stubs
   via `httptest.Server` always work regardless of the real server's
   formats config).
   **Resolution owner**: ADP-007 run-phase implementer.

2. **Per-call engines override**. Should a `Query.Filters[Key=
   "engines"]` filter route to the SearXNG `engines=...` URL parameter?
   **Recommended default**: NO in v0.1; rely on server-side default
   fanout. Add as P2 follow-up if measured value (e.g., a "Korean web
   only" route via `engines=naver,daum`) emerges.
   **Resolution owner**: SPEC-IR-002 author (a future Intent-Router
   v2 may surface engine routing).

3. **Korean-locale handling**. Should ADP-007 forward Korean queries
   with `language=ko` to SearXNG? The SPEC-IR-001 Korean classifier
   (`CategoryKorean`) currently routes to dedicated Korean-source
   adapters (Naver/Daum/KoreaNewsCrawler — SPEC-ADP-008/009). SearXNG
   v0.1 receives only `CategoryWeb` queries.
   **Recommended default**: NO `language` parameter in v0.1. Inherit
   server `default_lang: "en"`. Revisit when SPEC-ADP-008/009 are
   wired and the IR-001 v2 router decides whether SearXNG should also
   serve as a Korean fallback.
   **Resolution owner**: SPEC-IR-002 author.

4. **Time-range filter**. Should `Query.Filters[Key="time_range",
   Value="day|month|year"]` route to SearXNG `time_range`?
   **Recommended default**: NO in v0.1. Map `Capabilities.SupportsSince
   = false`. Add as a follow-up SPEC if measured value warrants.
   **Resolution owner**: SPEC-FAN-002 (M3 fanout filter routing) author.

5. **Limiter status code (429 vs 403)**. The `searx/botdetection/`
   module exports a `too_many_requests` function but the precise
   HTTP status returned was not directly observable via WebFetch.
   **Recommended default**: REQ-ADP7-006 categorises 429 as
   RateLimited (with Retry-After parsing). REQ-ADP7-007 categorises
   403 as Permanent BY DEFAULT but PROMOTES to RateLimited when
   `Retry-After` is present in the response. This dual handling
   covers both observed behaviours without requiring run-phase
   modification.
   **Resolution owner**: ADP-007 run-phase implementer; document
   actual observed status code in iteration-2 HISTORY entry.

6. **AGPL service-boundary documentation update**. Does ADP-007
   require an entry in `NOTICE` or `docs/dependencies.md`?
   **Recommended default**: NO — `deploy/docker-compose.yml:108-109`
   already documents the boundary; `NOTICE` (per
   `.moai/project/tech.md:148`) lists SearXNG as a service-boundary
   dependency. ADP-007 is a CONSUMER of an existing service-boundary
   relationship, not a new boundary itself. No NOTICE update needed.
   **Resolution owner**: SPEC-DEP-001 SECURITY-3 owner (run a final
   compliance check before M3 close).

7. **Engine-of-origin cardinality for top-N enumeration**. Should
   `Capabilities.Notes` enumerate the engines surfaced via
   `Metadata["engine"]` / `["engines"]`?
   **Recommended default**: YES — list the explicitly-enabled engines
   from `deploy/searxng/settings.yml` (`google`, `bing`, `duckduckgo`)
   plus a "+ default upstream engines" note. Bounded set; operators
   adding engines own the cardinality footprint going forward.
   **Resolution owner**: ADP-007 SPEC author (resolved inline in §2.6
   of the SPEC's `Capabilities()` shape).

---

## 7. AGPL-3.0 License Compliance Note

Verified at https://github.com/searxng/searxng — "This project is
licensed under the GNU Affero General Public License (AGPL-3.0)."
The relevant downstream constraints for Universal Search:

- **Service-boundary consumption**: The deployed compose stack at
  `deploy/docker-compose.yml:106-130` runs SearXNG as a separate
  container. Universal Search's Go adapter calls SearXNG over HTTP.
  This is a **service boundary, not a derived work**: AGPL §13 (network
  remote-interaction clause) requires sharing modifications to SearXNG
  itself with the network's users. Universal Search does NOT modify
  SearXNG; it consumes the upstream image as-is. The AGPL obligation
  flows through to operators IF they modify SearXNG; the unmodified
  image carries no Universal Search-side obligation beyond compliance
  with the original AGPL.
- **No linking, no embedding, no forking**: ADP-007 makes ZERO source-
  level dependencies on SearXNG code. The Go adapter parses the JSON
  HTTP response only. The `searxng/searxng` Docker image is the
  upstream's responsibility; the Universal Search project does not
  vendor, modify, or redistribute the image.
- **NOTICE update**: Already covered by SPEC-BOOT-001 / SPEC-DEP-001.
  ADP-007 requires no additional NOTICE entry.
- **SaaS posture**: `.moai/project/tech.md:148` already flags "SearXNG
  AGPL contagion if ever offered as SaaS — Medium / V1 is self-hosted
  only; SaaS path re-evaluates licensing." ADP-007 inherits this risk
  posture; the SPEC reaffirms in its risk register but does NOT add
  new constraints.

The legal posture is settled at the project level. ADP-007 is
strictly downstream of these decisions.

---

## 8. Sources and Citations

### External URLs (WebFetch verified)

- https://docs.searxng.org/ — SearXNG project home; verified.
- https://docs.searxng.org/dev/search_api.html — JSON Search API
  parameter table (q, format, categories, engines, language, pageno,
  time_range, safesearch, etc.) and "format must be enabled in
  settings" caveat.
- https://docs.searxng.org/admin/settings/settings.html — settings.yml
  reference; `secret_key` requirement; limiter / formats settings
  exist (precise behavioural docs absent on the rendered page; the
  source files are the actual reference).
- https://github.com/searxng/searxng — repo; verified license is
  AGPL-3.0; verified Python is the dominant language; no official Go
  client.
- https://github.com/searxng/searxng/blob/master/searx/results.py —
  per-result field shape (url/title/content/engine/engines/category/
  score/positions/template/parsed_url).
- https://github.com/searxng/searxng/blob/master/searx/webapp.py —
  JSON response construction path; response top-level keys (query,
  number_of_results, results, suggestions, corrections, answers,
  infoboxes, unresponsive_engines).
- https://github.com/searxng/searxng/blob/master/searx/limiter.toml —
  bot-detection IP filtering config; does NOT itself document HTTP
  status codes (those live in `searx/botdetection/__init__.py` —
  function `too_many_requests`).

### Internal Files (file:line cited)

- `/Users/masterp/Projects/superwork/univesal-search/deploy/docker-compose.yml:14-16,106-130` — SearXNG image pin and service definition.
- `/Users/masterp/Projects/superwork/univesal-search/deploy/searxng/settings.yml:1-43` — server-side SearXNG config (formats / limiter settings INHERIT defaults; only engines partially overridden).
- `/Users/masterp/Projects/superwork/univesal-search/internal/adapters/reddit/reddit.go:1-136` — reference adapter shape (Adapter struct, Options, New, Name, Capabilities, Healthcheck, compile-time interface assertion).
- `/Users/masterp/Projects/superwork/univesal-search/internal/adapters/reddit/search.go:1-167` — reference Search hot path (validate / build / execute / parse / categorise).
- `/Users/masterp/Projects/superwork/univesal-search/internal/adapters/reddit/parse.go:1-203` — reference parseListing transform pattern (HTTP→JSON→[]NormalizedDoc).
- `/Users/masterp/Projects/superwork/univesal-search/internal/adapters/reddit/client.go:1-125` — reference HTTP client construction + redirectAllowlist + categorizeStatus + parseRetryAfter.
- `/Users/masterp/Projects/superwork/univesal-search/internal/adapters/registry.go:75-167,172-263` — registry sole-emitter wrappedAdapter pattern.
- `/Users/masterp/Projects/superwork/univesal-search/pkg/types/types.go:1-22` — `pkg/types` SDK boundary description.
- `/Users/masterp/Projects/superwork/univesal-search/pkg/types/capabilities.go:38-62` — Capabilities struct + DocType enum (relied on by Capabilities()).
- `/Users/masterp/Projects/superwork/univesal-search/internal/router/category.go:13-122` — Category enum + CategoryEligibleDocTypes (`CategoryWeb` → DocTypeArticle/Post/Other).
- `/Users/masterp/Projects/superwork/univesal-search/internal/obs/metrics/metrics.go:37,89-95,171` — FanoutInflight Gauge pre-registration; cardinality allowlist (adapter_class, adapter, outcome).
- `/Users/masterp/Projects/superwork/univesal-search/.moai/specs/SPEC-ADP-001/spec.md` — Reddit reference SPEC; structural template for ADP-007.
- `/Users/masterp/Projects/superwork/univesal-search/.moai/specs/SPEC-ADP-002/spec.md` — Hacker News SPEC; integer-string-page cursor pattern reused by ADP-007.
- `/Users/masterp/Projects/superwork/univesal-search/.moai/specs/SPEC-FAN-001/spec.md` — Fanout SPEC; ADP-007 must satisfy the per-adapter timeout and concurrent-safety contract (REQ-ADP7-010).
- `/Users/masterp/Projects/superwork/univesal-search/.moai/specs/SPEC-BOOT-001/spec.md:60-79` — Compose stack baseline including SearXNG service.
- `/Users/masterp/Projects/superwork/univesal-search/.moai/project/roadmap.md:52,123,150` — M3 row "SPEC-ADP-007 SearXNG bridge", parallelization plan, M3 exit criterion.
- `/Users/masterp/Projects/superwork/univesal-search/.moai/project/tech.md:41,97,119,148,166` — SearXNG project-level posture (compose stack member; primary web source; AGPL contagion risk; service-boundary decision).
- `/Users/masterp/Projects/superwork/univesal-search/.moai/project/structure.md:28` — `internal/adapters/searxng/` package reservation.
- `/Users/masterp/Projects/superwork/univesal-search/go.mod:1-49` — Go module `github.com/elymas/universal-search`; ADP-007 introduces ZERO new module dependencies.

---

End of Research Document.

**Summary for SPEC Author**: This research establishes the SearXNG
deployment posture (compose service at `searxng:8080`, AGPL-3.0
service-boundary consumption per `deploy/docker-compose.yml:108-109`),
the JSON Search API surface (`GET /search?q=...&format=json` returning
a `{query, number_of_results, results, suggestions, corrections,
answers, infoboxes, unresponsive_engines}` envelope with per-result
fields `url, title, content, engine, engines, category, score,
positions, template, publishedDate?`), the page-number-based
pagination convention (`pageno=1..N`, decimal-string cursor reused
from SPEC-ADP-002), the limiter / 429+403 dual-status mapping, and the
engine-of-origin cardinality mitigation strategy (Metadata["engines"]
list, NEVER Prometheus labels). The reference adapter shapes from
SPEC-ADP-001 / SPEC-ADP-002 are reused VERBATIM at the file-layout,
HTTP-client, error-mapping, MX-tag-plan, and TDD-harness levels.
SearXNG-specific deltas: page-number cursor (already proven by HN);
local-HTTP-only redirect allowlist (`searxng:8080` + `localhost:8080`
+ `127.0.0.1:8080`); empty `Body` and `Snippet` derived from `content`
(SearXNG never returns rich `selftext`-style bodies; the upstream
`content` is the snippet); engine-of-origin metadata. 11 EARS REQs
target (8 P0 + 3 P1) covering all five EARS patterns; 4 NFRs aligned
with ADP-001 / ADP-002 / FAN-001; 7 Open Questions deferred with
recommended defaults. Zero new Go module dependencies.
