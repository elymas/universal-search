# SPEC-DOC-002 Research — Adapter reference documentation

Status: companion to spec.md
Date: 2026-05-22
Author: limbowl via manager-spec
Phase: Plan / Research (deep codebase analysis + reference-doc pattern survey)

---

## 0. Purpose

Ground SPEC-DOC-002 ("Adapter reference — per-adapter keys, rate limits,
Korean tokenizer setup, troubleshooting") in **what the code already
shows** so the spec.md REQs are testable against the binary, not against
assumptions. Three pillars of investigation:

1. **Adapter inventory** — what exists in `internal/adapters/` today, what
   each adapter declares via `types.Capabilities`, what its error
   taxonomy looks like, what env vars it consumes, and what
   Korean-locale-specific behaviour (if any) it exposes.
2. **Reference-doc patterns** — how other open-source projects with
   multi-adapter / multi-connector architectures (Meilisearch
   integrations, Logstash inputs, Airbyte connectors, OpenSearch
   plugins, SearXNG engines) lay out per-adapter reference pages.
3. **Drift detection mechanisms** — how to keep MDX reference pages
   in sync with Go source so a CI gate fails when the two diverge,
   matching the SPEC-DOC-001 `gen-cli-reference.sh` pattern.

This file is **read-only investigation**. It does not propose code
changes. spec.md (companion) translates findings into EARS REQs;
plan.md (companion) phases the work.

---

## 1. Adapter inventory (codebase-grounded)

Source of truth: `internal/adapters/` tree as of `git status` on
2026-05-22 (HEAD: 761381d "docs(spec): add SPEC-SEC-001 draft —
security hardening (M8)").

Total: **10 production adapters** distributed across 9 SPEC IDs
(SPEC-ADP-001..009). `SPEC-ADP-006` ships **two** adapter
implementations (Bluesky + X) behind a single `social` package because
they share normalization + URL extraction code (`internal/adapters/
social/url.go`, `internal/adapters/social/parse.go`). Plus 1 internal
`noop` adapter used only in tests — out of scope for this SPEC's
documentation surface.

`registry.go:108-138` wraps each adapter via `wrappedAdapter` to emit
`AdapterCalls{adapter,outcome}` Counter + `AdapterCallDuration{
adapter}` Histogram + OTel span + slog record. This wrapper layer is
**transparent to the docs** — adapter reference pages describe the
underlying adapter's contract, not the observability wrapping (already
documented by SPEC-OBS-001 + SPEC-EVAL-002).

### 1.1 Per-adapter summary table

| SPEC ID | Package | Capabilities.SourceID | RequiresAuth | AuthEnvVars | RateLimitPerMin | DefaultMaxResults | Korean-locale? | LOC (impl) |
|---------|---------|----------------------|--------------|-------------|-----------------|-------------------|----------------|------------|
| ADP-001 | `internal/adapters/reddit/` | `reddit` | false | `nil` | 10 (conservative unauth, research.md §1.7) | 25 | no | impl ~707 (5 files) |
| ADP-002 | `internal/adapters/hn/` | `hn` | false | `nil` | 60 | 25 | no | impl ~700 (6 files incl. strip.go) |
| ADP-003 | `internal/adapters/arxiv/` | `arxiv` | false | `nil` | 20 | 25 | no | impl ~620 (5 files + rate_test) |
| ADP-004 | `internal/adapters/github/` | `github` | **true** | `["USEARCH_GITHUB_TOKEN"]` | 30 (code-search 9/min separately enforced via per-route bucket per `github.go:155`) | 25 | no | impl ~720 (6 files) |
| ADP-005 | `internal/adapters/youtube/` | `youtube` | false (cookie-based scraper) | `nil` | 30 | 25 | no (but lang.go handles ko-KR locale negotiation) | impl ~810 (6 files) |
| ADP-006 (Bluesky) | `internal/adapters/social/` (sub=`bluesky`) | `bluesky` | false (public AppView) | `nil` | 600 (public AppView, blueskyCapabilities() `social.go:151`) | 25 | no | shared impl |
| ADP-006 (X) | `internal/adapters/social/` (sub=`x`) | `x` | false | `nil` | 0 (advertised — actual depends on syndication endpoint health, xCapabilities() `social.go:173`) | 0 (degraded, syndication-only) | no | shared impl ~700 (8 files) |
| ADP-007 | `internal/adapters/searxng/` | `searxng` | false | `nil` | 0 (post-audit H3, self-hosted no external limit, `searxng.go:152`) | 10 | no | impl ~625 (5 files) |
| ADP-008 | `internal/adapters/naver/` | `naver` | **true** | `["NAVER_CLIENT_ID", "NAVER_CLIENT_SECRET"]` | 10 | 25 | **yes** (Naver web/news/blog/shopping + DataLab; openapi.naver.com) | impl ~960 (7 files incl. datalab.go) |
| ADP-009 | `internal/adapters/koreanews/` | `koreanews` | false | `nil` | 0 (governed by operator-configured feed count × per-feed rate, `koreanews.go:92`) | 20 | **yes** (Daum news + KNC scraper + Korean RSS aggregator + mecab-ko-aware dedup via `dedup.go`) | impl ~1100 (10 files incl. locale.go, options.go) |
| `noop` | `internal/adapters/noop/` | `noop` | false | `nil` | 0 | 10 | no | impl ~46 (test-only, **excluded from docs**) |

Verification: `grep -rn "Capabilities()" internal/adapters/*/[!_]*.go`
returns one Capabilities() function per adapter package; the
`social` package has three (top-level dispatch + bluesky + x).

### 1.2 Per-adapter file layout (consistent contract)

Each adapter follows the canonical 5-file pattern established by
SPEC-ADP-001 (Reddit) as the reference implementation:

| File | Purpose | Approximate size |
|------|---------|------------------|
| `{name}.go` | Adapter struct + `New()` constructor + `Name()` + `Capabilities()` + `Healthcheck()` | 100-220 LOC |
| `client.go` | HTTP client construction + auth header injection + status-to-Category mapping | 60-200 LOC |
| `search.go` | `Search()` implementation: param construction → request → parse → normalize | 130-250 LOC |
| `parse.go` | Provider-specific response decoder → `[]types.NormalizedDoc` | 180-270 LOC |
| `errors.go` | `*types.SourceError` constructors + status-code rosetta | 60-90 LOC |

Variants:

- **arxiv** adds `rate_test.go` and exposes `waitForRateSlot(ctx)`
  with per-instance mutex (`arxiv.go:62` "Rate-limit state
  (REQ-ADP3-012). Per-instance; mutex-guarded.").
- **hn** adds `strip.go` for HTML tag stripping in comment bodies.
- **github** adds `score.go` for ranking signal extraction (stars,
  forks, recency).
- **youtube** adds `lang.go` (line 1-68) for `Accept-Language` /
  `hl=` parameter negotiation supporting `ko-KR` locale (not full
  Korean tokenization — handled by SPEC-IDX-003).
- **naver** adds `datalab.go` for DataLab trend API (separate
  endpoint, separate rate budget).
- **social** is the most divergent: separate `search_bluesky.go`
  (135 LOC, full AppView client) + `search_x.go` (31 LOC, degraded
  Nitter/syndication fallback) + shared `parse.go` / `url.go` /
  `score.go`. **Documentation MUST split bluesky and x into two
  reference pages** despite the shared Go package, because the
  operator-facing setup, rate limits, and reliability profile
  differ materially.
- **koreanews** is the largest (10 files): `daum.go`, `knc.go`
  (KoreaNewsCrawler bridge), `rss.go` (RSS aggregator),
  `locale.go` (Korean MIME/encoding handling for legacy
  EUC-KR feeds), `options.go` (operator-configured feed list +
  per-feed rate, mapped to `koreanews.RateLimitPerMin=0`),
  `dedup.go` (mecab-ko-aware near-duplicate detection
  cross-referencing `services/tokenizer-ko/`).

### 1.3 Auth-bearing adapters (2 of 10)

Only **GitHub** and **Naver** declare `RequiresAuth: true`. All
others either need no credentials (Reddit unauth, HN, arxiv,
Bluesky public AppView, SearXNG self-hosted, YouTube cookie-based
scrape) or use degraded/optional auth at the HTTP client layer
without declaring it via `Capabilities.AuthEnvVars` (X/Twitter
degraded mode, koreanews RSS).

**Verification reference**:
- `internal/adapters/github/github.go:146-148` →
  `RequiresAuth: true, AuthEnvVars: ["USEARCH_GITHUB_TOKEN"]`.
- `internal/adapters/naver/naver.go:190-191` →
  `RequiresAuth: true, AuthEnvVars: ["NAVER_CLIENT_ID",
  "NAVER_CLIENT_SECRET"]`.
- `internal/adapters/naver/client.go:67-69` confirms credentials
  are sent via `X-Naver-Client-Id` + `X-Naver-Client-Secret`
  headers, NOT query params (PII-safe for slog).

Implication for docs: per-adapter "Authentication" section is
**materially different per adapter**. A flat template forced on
all 10 produces low-quality pages for the 8 no-auth adapters.
Solution: template SHALL have an "Authentication" section that
becomes "Not required — public endpoint" for the no-auth set,
"Personal access token via env var" for GitHub, and "Application
credentials (client_id + client_secret) via env vars" for Naver.

### 1.4 Rate-limit semantics (heterogeneous)

The `RateLimitPerMin` field in `types.Capabilities` is **declarative
metadata** — FAN-001 fanout dispatcher reads it for advisory
backoff, but it does NOT enforce a runtime token bucket in the
adapter package itself. Each adapter handles its own rate budgeting:

- **arxiv**: explicit per-instance minimum-interval guard
  (`waitForRateSlot` at `search.go:142-146`) — strictest
  enforcement.
- **github**: relies on go-github's `RateLimitError` /
  `AbuseRateLimitError` types being raised by the GitHub API
  response headers (`client.go:77-112`); adapter parses
  `X-RateLimit-Remaining` + `X-RateLimit-Reset` and translates
  to `CategoryRateLimited` with `RetryAfter`.
- **naver**: HTTP-level — 429 response → `categorizeStatus`
  (`client.go:87-110`) → `CategoryRateLimited` with `RetryAfter`
  parsed from `Retry-After` header.
- **reddit**, **hn**, **youtube**, **bluesky**: same pattern as
  naver (HTTP 429 → CategoryRateLimited).
- **searxng**: no rate limit advertised (self-hosted; operator
  controls).
- **koreanews**: declared 0 because rate budgeting is per-feed and
  operator-configured via `options.go` `FeedConfig.RateLimitPerMin`
  field. Documentation MUST explain this delegation explicitly —
  users seeing 0 in the Capabilities printout will incorrectly
  assume "unlimited".
- **x (Twitter)**: declared 0 because syndication endpoint health
  is opaque; advertised limit cannot be promised. Documentation
  MUST explain the degraded-mode contract.

Implication for docs: per-adapter "Rate limits" section is **not
a single number from Capabilities**. The template needs:
(a) the advertised Capabilities value (cited from Go source line
number for traceability), (b) the enforcement mechanism (in-process
interval guard vs HTTP 429 response handling vs operator-
configured), (c) the upstream provider's published quota with a
verified URL, (d) the failure mode when exceeded (CategoryRateLimited
+ RetryAfter handling by SPEC-FAN-001 fanout + SPEC-CACHE-001
fallback). The "Korean tokenizer setup" requirement (roadmap.md:113)
applies only to ADP-008 (Naver) and ADP-009 (koreanews), with cross-
reference to SPEC-IDX-003 for the index-side mecab-ko configuration.

### 1.5 Error envelope (uniform — `*types.SourceError`)

Every adapter returns `*types.SourceError` (defined in
`pkg/types/errors.go`) wrapping a `Category` enum:

- `CategoryPermanent` — 4xx (other than 429) auth/permission
  failures
- `CategoryRateLimited` — 429 with `RetryAfter`
- `CategoryUnavailable` — 5xx or network-layer
- `CategoryTransient` — parse failure, partial response
- `CategoryUnknown` — non-mapped status

Cross-adapter consistency: confirmed by
`grep "CategoryRateLimited" internal/adapters/*/client.go`
returning 7 hits in arxiv, github, hn, naver, reddit, searxng,
youtube client.go files. social (bluesky+x) handles errors in
search_bluesky.go and errors.go directly. koreanews has a
multi-source aggregation envelope in `errors.go` because partial
feed failure must not fail the whole adapter.

Implication for docs: a **shared "Error categories" reference page**
sits at `reference/adapters/errors.mdx` documenting the 5 Category
values once; per-adapter pages link to it and only enumerate
**adapter-specific status code mappings** (e.g., GitHub's 422
"Validation failed" → CategoryPermanent vs Naver's 401 "Invalid
client id" → CategoryPermanent with operator-actionable hint
"check NAVER_CLIENT_ID env var").

### 1.6 Korean-locale-specific surface (ADP-008 + ADP-009)

Two adapters carry Korean-specific configuration that the docs MUST
surface explicitly (per roadmap.md:113 scope clause "Korean
tokenizer setup"):

**Naver (ADP-008)**:
- Cross-domain redirect allowlist: only `openapi.naver.com`
  (`naver/client.go:22-24`). Korean operators self-hosting behind
  a corporate proxy must NOT redirect through CDN; documentation
  notes this constraint.
- DataLab (`datalab.go`) is a separate endpoint
  (`openapi.naver.com/v1/datalab/search`) with its own rate budget
  and request body format (JSON POST vs GET for search APIs).
  Often missed by operators; deserves a dedicated subsection.
- Korean query strings: no special tokenization in the adapter
  itself — passes UTF-8 query verbatim. Index-side tokenization is
  SPEC-IDX-003's responsibility (mecab-ko Meili plugin). Per-adapter
  doc cross-links to the operator-facing Korean tokenizer setup at
  `operators/korean-locale-setup.mdx` (SPEC-DOC-001 D3 KO-authoritative
  page).

**koreanews (ADP-009)**:
- `locale.go` handles EUC-KR → UTF-8 transcoding for legacy Korean
  RSS feeds (Hankyoreh archived feeds, Chosun pre-2018 archives,
  etc.) that still ship `Content-Type: text/xml; charset=euc-kr`.
  Documentation MUST list known-EUC-KR feeds with a migration
  note when the upstream feed flips to UTF-8.
- `dedup.go` uses mecab-ko **morpheme-level** near-duplicate
  detection (not just title hash) because Korean news syndication
  often republishes identical articles across Daum / Naver / direct
  RSS with minor headline edits. The dedup contract is documented
  inline at `dedup.go` but invisible to end users — surface it.
- KNC bridge (`knc.go`) shells out to a Python sidecar
  (`services/storm/koreanewscrawler/`); documentation MUST list
  the sidecar dependency, expected version, and the env var
  `USEARCH_KNC_ENDPOINT` that points the Go side at the sidecar.

### 1.7 Lifecycle status per adapter (alpha / beta / stable)

Source of truth — derived from SPEC-ADP-* `status:` frontmatter +
SPEC-EVAL-002 reliability dashboard `lifecycle` field:

- **stable**: SPECs marked `status: implemented` AND 7-day rolling
  success rate (per EVAL-002 dashboard) ≥ 0.95.
- **beta**: `status: implemented` AND success rate 0.80–0.94.
- **alpha**: any of (a) `status: draft|in_progress`, (b) success
  rate < 0.80, (c) explicitly degraded-mode (X/Twitter).

Current SPEC frontmatter sweep (verified by grep on
`.moai/specs/SPEC-ADP-*/spec.md`):

- ADP-001, ADP-002, ADP-003, ADP-004, ADP-005, ADP-007, ADP-008,
  ADP-009 → `status: implemented`. EVAL-002 dashboard provides
  the success-rate input. At V1.0.0 ship time, expected
  classification: **stable** (assuming no regression in M8 eval).
- ADP-006 (social) → ships both `bluesky` (expected **beta** —
  AppView public API stable but recent rate-limit changes per
  Bluesky team upstream) and `x` (expected **alpha** — degraded
  syndication mode, ongoing reliability volatility).

Implication for docs: each per-adapter reference page surfaces a
**status badge** at the top, sourced from a single JSON file
(`docs/data/adapter-status.json`) populated by an EVAL-002
dashboard export job. SPEC-DOC-002 owns the badge taxonomy + the
JSON schema; SPEC-EVAL-002 owns the data feed. Badge values:
`alpha`, `beta`, `stable`, `deprecated` (last reserved for future
removal).

---

## 2. Reference-doc pattern survey (external precedent)

Goal: ground the per-adapter MDX template in established OSS
conventions so contributors do not have to invent structure.

### 2.1 Meilisearch language guides

Pattern: per-language reference at `docs.meilisearch.com/learn/
indexing/discover_the_settings.html#korean-language-support`
includes: (1) installation note, (2) configuration example,
(3) tokenizer setup, (4) known limitations, (5) troubleshooting.
**Direct precedent for Korean-tokenizer-setup cross-link** in
ADP-008/009 pages (since SPEC-IDX-003 ships Meili mecab-ko
integration, the docs IA mirrors Meilisearch's own structure).

### 2.2 Logstash inputs / Airbyte source connectors

Pattern: per-input reference at `www.elastic.co/guide/en/logstash/
current/input-plugins.html` (e.g., `http_poller`, `rss`, `twitter`)
includes a uniform 8-section template:
1. **Compatibility note** (Logstash version, dependencies)
2. **Synopsis** (1-line summary)
3. **Description** (full prose)
4. **Common options** (table of cross-plugin shared fields)
5. **Plugin options** (table of plugin-specific fields with
   type, default, required, description)
6. **HTTP poller / RSS-specific notes** (rate limits, polling
   intervals, gotchas)
7. **Logstash semantic version compatibility matrix**
8. **Troubleshooting** (common errors, log patterns)

This template is the **closest established precedent** for what
SPEC-DOC-002 should ship. Adoption proposal:

- **Section 1 "Status & Compatibility"** — version badge
  (alpha/beta/stable from EVAL-002), Adapter SPEC ID,
  implementation source path, last-verified date.
- **Section 2 "Overview"** — what data this adapter retrieves,
  upstream provider, typical use case.
- **Section 3 "Setup"** — authentication (env vars), required
  external service config (e.g., Naver Developer Console app
  registration), Korean-locale prerequisites (cross-link to
  IDX-003) where applicable.
- **Section 4 "Capabilities"** — table sourced from
  `Capabilities()` Go source via auto-extract script (drift-
  checked by CI), showing SourceID, RequiresAuth, AuthEnvVars,
  RateLimitPerMin, DefaultMaxResults, optional flags.
- **Section 5 "Query syntax"** — what user-supplied query
  strings translate into for this adapter (e.g., Reddit:
  passed to `/search.json?q=`; arxiv: translated to arxiv
  API syntax; Naver: passes UTF-8 verbatim to openapi.naver.com).
- **Section 6 "Rate limits"** — advertised value + enforcement
  mechanism + upstream provider's published quota link +
  exhaustion behaviour (`CategoryRateLimited` + retry semantics).
- **Section 7 "Error reference"** — link to shared
  `reference/adapters/errors.mdx` + adapter-specific status
  code rosetta (e.g., GitHub 422 vs Naver 401).
- **Section 8 "Troubleshooting"** — symptom → likely cause →
  diagnostic command → resolution, mirroring SPEC-DOC-001
  `troubleshooting/index.mdx` 5-field format.
- **Section 9 "Version compatibility"** — which usearch versions
  ship this adapter, last upstream provider API version verified.
- **Section 10 "Related"** — cross-links to operator pages
  (deployment-helm, korean-locale-setup), end-user pages
  (surface-comparison, cli-tour), and SPECs.

### 2.3 OpenSearch / Elasticsearch plugin docs

Pattern: per-plugin page at `opensearch.org/docs/latest/install-
and-configure/plugins/` follows a similar 8-section template but
adds **"Permissions" section** describing the user-facing
permissions the plugin operates under. usearch adapters do not
require runtime per-tenant permissions beyond SPEC-AUTH-002 RBAC,
so this section becomes "Tenant scoping" — explaining how
per-tenant query routing through this adapter works (SPEC-AUTH-002
+ SPEC-IDX-004 cross-reference for multi-tenant operators).

### 2.4 SearXNG engine docs (most direct analogue)

SearXNG itself documents 200+ engines at `docs.searxng.org/admin/
engines/` with a per-engine page template that is the **closest
formal precedent** because SearXNG engines are conceptually
identical to usearch adapters:

- Engine summary table (categories, language support, paging,
  time range, safe search)
- Settings YAML snippet
- API key acquisition note (when applicable)
- Known issues

Adoption proposal: borrow the **summary table at the top of each
page** convention — a 1-screen "at-a-glance" block before the
prose, mirroring Capabilities + status + key env vars + key
provider URLs.

### 2.5 Anti-patterns observed

- **Plumbum** / **Click** plugin docs that auto-dump CLI flags as
  reference without prose context — unhelpful when the adapter
  has provider-specific configuration not captured in CLI flags
  (e.g., Naver Developer Console URL whitelisting).
- **Helm chart per-component README files** that drift from
  `values.yaml` because no drift CI — directly motivating the
  drift-detection REQ in spec.md.
- **Vendored-only reference docs** (Postman, Stoplight) that
  require a SaaS account — violates SPEC-DOC-001 D4
  anti-decision (Algolia DocSearch / SaaS lock-in).

---

## 3. Drift detection: keeping MDX in sync with Go source

Per the SPEC-DOC-001 `gen-cli-reference.sh` precedent (REQ-DOC-007:
auto-extract `usearch --help` → MDX with CI gate that fails on
drift), SPEC-DOC-002 needs an analogous mechanism for the
**Capabilities table** in each adapter reference page.

### 3.1 What can be auto-extracted

From each adapter's `Capabilities()` function, the following
fields can be machine-extracted using `go/parser` AST walking
(no runtime execution required — the function literal returns a
struct literal):

- `SourceID` (string literal)
- `RequiresAuth` (bool literal)
- `AuthEnvVars` (string slice literal, or nil)
- `RateLimitPerMin` (int literal)
- `DefaultMaxResults` (int literal)
- Adapter `Name()` return value (string literal in same struct
  literal or returned directly)

These are the values that drift most often — every time an
adapter SPEC amendment changes a rate limit or adds a new auth
env var (e.g., future ADP-006 X/Twitter v2 OAuth migration), the
docs page must follow within the same PR or CI fails.

### 3.2 What CANNOT be auto-extracted

- Prose description fields
- Korean-locale operational notes (require human authoring;
  bilingual review)
- Troubleshooting entries (synthesized from operator experience)
- Provider's upstream documentation URLs (require human
  verification via lychee — SPEC-DOC-001 REQ-DOC-013)
- Version compatibility matrix entries (require human
  attestation: "verified against arxiv API v2 on YYYY-MM-DD")

### 3.3 Proposed drift script: `scripts/gen-adapter-reference.sh`

Mirroring `scripts/gen-cli-reference.sh` (SPEC-DOC-001 REQ-DOC-007):

- Input: walk `internal/adapters/*/` (skip `noop/`).
- Tool: small Go program (`tools/gen-adapter-ref/main.go`) using
  `go/parser` to load `{package}.go`, extract Capabilities struct
  literal, emit a structured **fragment** file (`docs/content/en/
  reference/adapters/_generated/{adapter}.capabilities.json`).
- Each MDX page imports the JSON fragment via a custom MDX
  component (`<CapabilitiesTable src="_generated/reddit.
  capabilities.json" />`).
- CI gate (`docs.yml` extends `gen-reference-drift` job from
  SPEC-DOC-001 REQ-DOC-007 OR adds a new `gen-adapter-ref-drift`
  job): re-run the script, assert committed JSON fragments
  match output. Drift = CI fail.
- The drift gate ensures: any PR modifying an adapter's
  `Capabilities()` MUST also commit the updated JSON fragment.
  Prose around the table can drift independently — only the
  table itself is gated.

### 3.4 Alternative considered: full MDX generation

Rejected. Full MDX generation (à la Sphinx autodoc) produces
low-quality pages for adapter docs because the high-value content
is operator narrative, not Go struct dump. The hybrid (JSON
fragment + hand-written MDX) is the same compromise SearXNG took
for its engine docs and Logstash took for plugin docs.

### 3.5 Adapter status badge data source

Mirroring above — `docs/content/en/reference/adapters/_generated/
adapter-status.json` populated by an EVAL-002 dashboard export
job (cron'd, daily). MDX page imports JSON via `<StatusBadge
adapter="reddit" />` component. Stale data (mtime > 7 days)
triggers CI warning (not fail). SPEC-EVAL-002 owns the dashboard;
SPEC-DOC-002 owns the badge presentation + JSON schema. Schema:

```json
{
  "reddit": {"lifecycle": "stable", "successRate7d": 0.97, "verifiedAt": "2026-05-22T00:00:00Z"},
  "x": {"lifecycle": "alpha", "successRate7d": 0.42, "verifiedAt": "2026-05-22T00:00:00Z"}
}
```

---

## 4. Content sourcing strategy (DDD lens)

This is a **DDD SPEC**, not TDD — adapter implementations exist
and behave correctly per SPEC-ADP-* acceptance tests. SPEC-DOC-002
ANALYZE-PRESERVE-IMPROVE breaks down:

- **ANALYZE**: §1 inventory above. Each adapter's `Capabilities`,
  `client.go` auth pattern, `errors.go` status mapping, and
  per-adapter quirks (rate limit enforcement mechanism, Korean
  locale handling) are documented as-they-exist in the Go source.
  No code changes.

- **PRESERVE**: zero changes to adapter behaviour. Documentation
  describes existing behaviour; if a doc draft surfaces a behaviour
  the operator finds surprising, the docs change to **describe
  reality**, not the code to match the docs. (Exception: clear bug
  in adapter code surfaced during documentation review is escalated
  as a separate fix SPEC; not absorbed silently into DOC-002 scope.)

- **IMPROVE**: the IMPROVE delta is documentation completeness +
  drift CI + status badges + Korean-locale operational notes
  (currently invisible to operators) + troubleshooting decision
  trees synthesized from M3 implementation reviews + SEC-001
  runbook patterns.

Content sources per page (10 reference pages: 9 from ADP-001..009
plus separate Bluesky vs X pages from ADP-006):

| Source type | Files | Treatment |
|-------------|-------|-----------|
| Capabilities table | `internal/adapters/{name}/{name}.go` `Capabilities()` | **AUTO-GEN** via §3.3 script; drift-gated |
| Auth env vars | Same | Auto-extracted; prose around it hand-written |
| Status code rosetta | `internal/adapters/{name}/client.go` `categorizeStatus`-style functions | Hand-written by manager-docs reading the Go switch statement (small enough; auto-gen overkill) |
| Provider's API quota docs | External (Reddit API docs, GitHub REST docs, Naver Developer docs, etc.) | Hand-written + lychee link-check |
| Korean-locale operational notes (ADP-008, ADP-009) | `internal/adapters/naver/`, `internal/adapters/koreanews/locale.go`, `dedup.go` | Hand-written, **KO-authoritative** (per SPEC-DOC-001 D3); EN counterpart for global operators |
| Troubleshooting | Synthesized from `.moai/specs/SPEC-ADP-*/research.md` failure-mode sections + SPEC-CACHE-001 fallback failure modes + operator field reports | Hand-written; cross-links to SPEC-DOC-001 troubleshooting top-10 |
| Version compatibility | Manual attestation by manager-docs at each minor release | Hand-written |
| Cross-links (related) | SPEC IDs, SPEC-DOC-001 operator pages, SPEC-DEPLOY-001 Helm values | Hand-written |

---

## 5. Integration with SPEC-DOC-001

SPEC-DOC-001 (just merged at 761381d... wait, DOC-001 was the
preceding doc-site SPEC drafted on 2026-05-22 — see HISTORY of
spec.md) reserves the IA slot at `reference/adapters/index.mdx`
via REQ-DOC-008 (Optional pattern: "WHERE SPEC-DOC-002 has
shipped, ...includes a `reference/adapters/` subtree owned by
SPEC-DOC-002 with cross-links from `end-users/surface-comparison.
mdx` and `operators/deployment-helm.mdx`"). DOC-001 ships a
placeholder index that links to DOC-002 status.

SPEC-DOC-002's contribution to that slot:

- `reference/adapters/index.mdx` — adapter catalog page (replaces
  DOC-001 placeholder): table of all 10 adapters with status
  badge + 1-line summary + link to detail page, filterable by
  category (search-engine / social / academic / news /
  Korean-locale).
- `reference/adapters/{adapter}.mdx` × 10 — per-adapter detail
  pages following the 10-section template (§2.2 above).
- `reference/adapters/errors.mdx` — shared `*types.SourceError`
  category reference (linked from every per-adapter page).
- `reference/adapters/_generated/*.capabilities.json` × 10 —
  auto-extracted JSON fragments.
- `reference/adapters/_generated/adapter-status.json` — EVAL-002
  badge data feed.

Bilingual coverage (per SPEC-DOC-001 D3 + REQ-DOC-016 90% gate):

- **ADP-008 Naver**, **ADP-009 koreanews**, **errors.mdx**, and
  **index.mdx** MUST have KO counterparts (Tier-1 priority — these
  are Korean operators' primary entry points).
- The other 8 adapter pages have **EN-authoritative** content;
  KO translation is Tier-2 (deferred to V1.1 minor release per
  DOC-001 D3).
- This deviates from DOC-001's "Tier-1 KO coverage of all
  operator-core pages" but is justified by: (a) most non-Korean
  adapter operators read English upstream provider docs anyway,
  (b) Korean operators primarily care about Naver + koreanews,
  (c) reference pages have **less narrative content** than
  operator runbooks so machine-assisted translation post-V1 is
  more feasible.
- Coverage gate adjustment: `scripts/check-bilingual-coverage.sh`
  (SPEC-DOC-001 REQ-DOC-016) needs an **exclude pattern
  extension** for `reference/adapters/*.mdx` EXCEPT
  `reference/adapters/{naver,koreanews,errors,index}.mdx`.
  This must be agreed with SPEC-DOC-001 owner during plan-auditor
  (open question §11.5).

---

## 6. Failure modes considered

What could go wrong, and how spec.md addresses each:

| Failure mode | spec.md REQ that addresses |
|--------------|----------------------------|
| Operator copies API key into a docs example, then commits it | REQ-ADPDOC-014 forbids real-credential examples; CI lint via SPEC-SEC-001 D2 gitleaks extension covers PR-time detection. |
| Adapter Capabilities changes without docs update | REQ-ADPDOC-007 drift CI gate. |
| Naver API quota silently changes (Naver Developer Console policy update) | REQ-ADPDOC-012 quarterly attestation date in version compatibility table; stale > 180 days = CI warn. |
| EUC-KR feed flips to UTF-8 (koreanews) and `locale.go` codepath becomes dead | research note in DOC-002 page; tracked as "known evolving" — not a CI gate, just a docs maintenance hint. |
| X/Twitter syndication endpoint goes down → ADP-006 alpha → SPEC-ADP-006 amendment downgrades it to "deprecated" | EVAL-002 status feed picks this up; badge auto-flips to deprecated. REQ-ADPDOC-005 status taxonomy contract. |
| New adapter added (post-V1 SPEC-ADP-010+) | REQ-ADPDOC-013 acceptance checklist for new adapters: must ship reference page in same PR as the adapter SPEC's run-phase completion. |
| Operator runs `usearch query` without setting `NAVER_CLIENT_ID` | Troubleshooting decision tree in `naver.mdx` covers the registry's `ErrMissingAuth` error path (`registry.go:124-128`). |

---

## 7. Reviewer pool considerations

Same constraint as SPEC-DOC-001 REQ-DOC-010 — KO content for
`naver.mdx`, `koreanews.mdx`, `errors.mdx` (KO), `index.mdx` (KO)
requires native-Korean-speaking reviewer. SPEC-DOC-001 already
opens this as Open Question §8.2; SPEC-DOC-002 inherits the same
reviewer pool. No new reviewer commitment; just a smaller delta
on the Tier-1 batch (4 KO pages added vs SPEC-DOC-001's ~22).

---

## 8. Risk matrix

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Vendor API drift (Reddit OAuth, GitHub REST, Naver Developer Console UI changes) | High | Medium (pages mention stale steps) | Quarterly attestation per REQ-ADPDOC-012; "last verified" date prominent on each page |
| Korean-tokenizer docs reviewed by non-Korean speakers (misinterpretation of mecab-ko semantics) | Medium | High (operator misconfigures Meili) | Mandatory native-reviewer signoff per SPEC-DOC-001 REQ-DOC-010 inheritance |
| Secret leakage in examples (operator copy-paste from page → commit real key) | Low (controlled examples) | Critical (credential compromise) | REQ-ADPDOC-014 placeholder-only policy; gitleaks pre-commit hook (SPEC-SEC-001 D2) catches commit-time leaks |
| Drift CI false positives blocking unrelated PRs | Medium | Low (annoyance) | Drift gate scoped to `Capabilities()` AST extraction only; prose drift not gated |
| Reference doc completeness measured loosely → ship V1 with stub-quality pages | Medium | Medium (operator trust loss) | REQ-ADPDOC-008 completeness checklist: every page MUST have all 10 sections non-empty; CI `check-adapter-page-completeness.sh` validates |
| Adapter status badge stale (EVAL-002 dashboard export job fails silently) | Low | Low (badge wrong, page still functional) | Stale (mtime > 7d) JSON triggers CI warn + GitHub Issue auto-creation tagged `docs/stale-adapter-status` (mirrors SPEC-DOC-001 REQ-DOC-014 screenshot-freshness pattern) |
| Bilingual coverage gate misconfigured → ships without KO Naver/koreanews | Low | High (Korean operator persona alienation) | Coverage gate exclude pattern explicitly enumerates Tier-1 KO adapter pages; `check-bilingual-coverage.sh` test fixture asserts the 4 required KO pages |
| Reviewer pool capacity insufficient | Medium | Medium (delay V1.0.0) | Tier-1 KO scope limited to 4 pages (vs DOC-001's ~22); plan-auditor confirms reviewer commitment before run phase starts |

---

## 9. Alternatives considered for the documentation structure

**Alternative A — single mega-page `reference/adapters.mdx`**
listing all adapters in one document.
Rejected. (a) Page would exceed 5000 lines once all 10 adapters
have 10 sections each. (b) Linking from `cli-tour.mdx` /
`deployment-helm.mdx` becomes anchor-link based instead of
filename-based — fragile. (c) Search relevance suffers (Pagefind
index hit returns the mega-page; user must scroll to find their
adapter). (d) Bilingual coverage at sub-page granularity is
impossible.

**Alternative B — per-adapter SPEC-ADP-* embedded as the doc**
(symlink or include).
Rejected. SPEC-ADP-* documents are developer-facing
(EARS requirements, acceptance criteria, file lists). Operators
do not want EARS syntax; they want "how to get a key + set the
env var + what error means what". Different audience.

**Alternative C — auto-generated docs from godoc**.
Rejected. godoc comments live in `internal/` packages — not
intended for external consumption. They describe code semantics,
not operator semantics. Same anti-pattern as §2.5 "Plumbum/Click
auto-dump CLI flags".

**Chosen — hand-authored per-adapter MDX with auto-extracted
JSON fragments for Capabilities tables + status badges, drift-
checked by CI**. Reasons documented above.

---

## 10. Open questions (to be resolved in plan-auditor cycle)

These do NOT block plan-auditor PASS — they are scope edges
flagged with rationale.

1. **Korean-tokenizer setup scope inside DOC-002** — does the
   per-adapter Naver/koreanews page **duplicate** the
   `operators/korean-locale-setup.mdx` content (SPEC-DOC-001 D3
   KO-authoritative page), or **cross-link only**? Proposal:
   cross-link only with a 3-line summary inline; full procedure
   lives in DOC-001's operator page. Avoids duplication, keeps
   single source of truth.

2. **Status badge taxonomy authority** — SPEC-DOC-002 defines
   the badge values (alpha/beta/stable/deprecated) AND the
   success-rate thresholds (≥0.95 stable, 0.80–0.94 beta, <0.80
   alpha). SPEC-EVAL-002 owns the dashboard that computes
   success rate. Need confirmation that EVAL-002 dashboard's
   `lifecycle` field matches DOC-002 taxonomy; if not, schema
   alignment PR before V1.0.0.

3. **Bluesky vs X page split** — research §1 confirms separate
   pages despite shared Go package. Plan-auditor confirms with
   user.

4. **EVAL-002 dashboard export cadence** — daily cron sufficient
   for V1.0.0 (acceptable lag for status badge), but if EVAL-002
   does not yet implement the export job, DOC-002 needs to ship
   a static initial `adapter-status.json` and the export job is
   tracked separately. Coordination with EVAL-002 owner.

5. **Bilingual coverage script exclusion pattern** — per §5,
   `check-bilingual-coverage.sh` needs to exclude most of
   `reference/adapters/` from the 90% gate while explicitly
   requiring the 4 Tier-1 KO adapter pages. Needs SPEC-DOC-001
   owner sign-off (since DOC-001 owns the script).

6. **`tools/gen-adapter-ref/` Go program location** — under
   `tools/` (repo-root sibling to `cmd/`) vs under
   `internal/tools/`. Convention check: repo currently has no
   `tools/` dir; `scripts/` is the precedent for build-time
   helpers. Proposal: put the Go program at `tools/gen-adapter-
   ref/` (sibling to `cmd/`) with the shell wrapper at
   `scripts/gen-adapter-reference.sh`. Plan-auditor confirms.

7. **Provider doc URL canonicalisation** — link to the **English**
   provider doc OR the **Korean** Naver Developer doc on the
   `naver.mdx` page? Proposal: EN page links EN provider doc;
   KO page links KO provider doc when available (Naver
   Developer docs have KO version). lychee allowlist covers both.

8. **Page completeness CI gate threshold** — REQ-ADPDOC-008
   asserts "all 10 sections non-empty". Defining "non-empty":
   ≥ 50 characters of MDX content per section excluding code
   blocks? ≥ 1 sentence? ≥ 1 paragraph? Proposal: ≥ 50
   characters of plain text after MDX → plaintext conversion
   per section. Plan-auditor finalizes.

---

## 11. Verification trail

References cited in this research file are verifiable against
the codebase at HEAD 761381d (2026-05-22):

- `internal/adapters/registry.go:108-138` — Register / wrapper layer
- `internal/adapters/arxiv/arxiv.go:112-124` — arxiv Capabilities
- `internal/adapters/github/github.go:137-160` — github Capabilities
- `internal/adapters/hn/hn.go:97-115` — hn Capabilities
- `internal/adapters/koreanews/koreanews.go:81-100` — koreanews Capabilities
- `internal/adapters/naver/naver.go:177-198` — naver Capabilities
- `internal/adapters/naver/client.go:22-110` — naver SSRF allowlist + status mapping
- `internal/adapters/reddit/reddit.go:97-115` — reddit Capabilities
- `internal/adapters/searxng/searxng.go:130-160` — searxng Capabilities
- `internal/adapters/social/social.go:130-180` — bluesky + x Capabilities
- `internal/adapters/youtube/youtube.go:94-110` — youtube Capabilities
- `internal/adapters/koreanews/locale.go` — EUC-KR transcoding
- `internal/adapters/koreanews/dedup.go` — mecab-ko-aware dedup
- `internal/adapters/koreanews/knc.go` — KNC sidecar bridge
- `internal/adapters/youtube/lang.go` — ko-KR locale negotiation
- `.moai/specs/SPEC-ADP-001/spec.md`..`SPEC-ADP-009/spec.md` —
  upstream SPEC frontmatter for `status:` field
- `.moai/specs/SPEC-EVAL-002/spec.md` — adapter reliability dashboard
- `.moai/specs/SPEC-IDX-003/spec.md` — Korean tokenization
  (cross-link target for Naver + koreanews pages)
- `.moai/specs/SPEC-DOC-001/spec.md` REQ-DOC-008, REQ-DOC-010,
  REQ-DOC-016, REQ-DOC-007 — IA slot reservation, Tier-1 KO
  policy, coverage gate, drift-check precedent
- `.moai/project/roadmap.md:113` — DOC-002 scope clause

---

## 12. External references (for spec.md §9)

- Logstash input plugin docs (template precedent):
  https://www.elastic.co/guide/en/logstash/current/input-plugins.html
- SearXNG engine docs (closest analogue):
  https://docs.searxng.org/admin/engines/
- Meilisearch language docs (Korean tokenizer cross-link pattern):
  https://docs.meilisearch.com/learn/indexing/discover_the_settings.html
- OpenSearch plugin docs:
  https://opensearch.org/docs/latest/install-and-configure/plugins/
- Airbyte source connector docs:
  https://docs.airbyte.com/integrations/sources/
- Naver Developers (Korean):
  https://developers.naver.com/docs/serviceapi/search/
- GitHub REST API rate limits:
  https://docs.github.com/en/rest/overview/resources-in-the-rest-api#rate-limiting
- Reddit API (rate limit policy):
  https://github.com/reddit-archive/reddit/wiki/API
- arxiv API (rate guidance):
  https://info.arxiv.org/help/api/user-manual.html
- Hacker News Algolia API:
  https://hn.algolia.com/api
- YouTube Data API quota:
  https://developers.google.com/youtube/v3/getting-started
- Bluesky AppView (atproto):
  https://docs.bsky.app/

---

*End of SPEC-DOC-002 research.md v0.1.0 (draft).*
