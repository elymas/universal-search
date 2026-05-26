# SPEC-ADP-009 Implementation Plan (Post-Hoc)

**SPEC**: SPEC-ADP-009 — KoreaNewsCrawler + Daum + Korean RSS Composite Adapter
**Status**: implemented (2026-05-04)
**Methodology**: TDD (RED → GREEN → REFACTOR)
**Coverage Target**: 85%
**Owner**: expert-backend
**Priority**: P1

---

## 1. Overview

ADP-009 is the Korean-locale FALLBACK breadth adapter — a composite
over three sub-sources gated by individual env flags:

| Sub-source | Default | v0.1 State |
|------------|---------|------------|
| RSS (gofeed) | ENABLED | Full implementation |
| KoreaNewsCrawler (KNC) | DISABLED | Go HTTP client + Python sidecar SCAFFOLD (stub returns 503) |
| Daum | DISABLED | Stub returns `ErrDaumDisabled` regardless of flag |

Key architectural deltas:

1. **Single composite over three separate adapters** — cardinality
   discipline (one `adapter="koreanews"` Prometheus label vs three);
   routing simplicity. `Metadata["subsource"]` carries per-doc
   attribution. `structure.md:18-22` reservations of `daum/` and
   `rss_korean/` directories are consolidated into the composite.

2. **Per-feed parallel RSS fetch with errgroup-bounded fan-out** —
   `errgroup.SetLimit(min(8, len(feeds)))` (mirroring SPEC-FAN-001
   §2.5 + §2.6 verbatim). Pre-allocated per-index
   `[][]NormalizedDoc` + `[]error` slices; no shared map writes.
   One feed's failure does NOT cancel siblings.

3. **Daum hard-disabled** per
   `https://search.daum.net/robots.txt` returning `User-agent: *
   Disallow: /` (verified 2026-05-04 via WebFetch). Stub returns
   `ErrDaumDisabled` regardless of env flag. Future SPEC-ADP-009-DAUM
   may unlock with explicit Kakao authorisation.

4. **Hangul ratio Korean-locale heuristic** — `detectKorean(text)`
   counts runes where `unicode.Is(unicode.Hangul, r)` divided by
   total non-whitespace runes; ≥ 0.30 → `Lang="ko"`. Handles
   operator-configured English tech-blog feeds alongside Korean
   newspaper feeds.

5. **Intra-adapter URL-canonicalization dedup** — mirrors
   SPEC-FAN-001 §2.4 8 rules verbatim; `CanonicalHash` fallback for
   unparseable URLs. First-occurrence-wins. Runs BEFORE FAN-001's
   cross-adapter dedup.

6. **`Score=0.5` constant** — RSS items have no upvote signal; KNC
   items have no relevance signal; Daum is disabled. SPEC-IDX-001
   RRF re-ranks by rank.

---

## 2. Architecture

### 2.1 Package Layout

```
internal/adapters/koreanews/
├── koreanews.go         — Adapter, New, Name, Capabilities, Healthcheck
├── koreanews_test.go    — interface conformance + Capabilities + Healthcheck
├── options.go           — Options + env-var loader
├── options_test.go      — env-var parsing (JSON array + comma list, 32-feed cap)
├── search.go            — (*Adapter).Search composite hot path + sub-source dispatch + dedup
├── search_test.go       — dispatch combinations + ctx cancel + empty query
├── rss.go               — searchRSS (gofeed-based, errgroup-bounded, per-feed isolation)
├── rss_test.go          — gofeed parse (RSS 2.0, Atom 1.0, JSON Feed 1.1), per-feed timeout, error isolation
├── daum.go              — searchDaum stub (always ErrDaumDisabled when enabled)
├── daum_test.go         — stub returns ErrDaumDisabled regardless of flag
├── knc.go               — searchKNC (HTTP client to Python sidecar at port 8002)
├── knc_test.go          — 503 sidecar default; 200 with JSON; 4xx; 5xx; ctx cancel
├── locale.go            — detectKorean (Hangul ratio heuristic)
├── locale_test.go       — table over 6 Hangul/Latin ratio inputs
├── strip.go             — stripHTML (verbatim from ADP-002)
├── strip_test.go        — table over 8 inputs
├── dedup.go             — dedupDocs (URL canonicalization + CanonicalHash fallback)
├── dedup_test.go        — dedup table over 5 fixtures + determinism
├── errors.go            — 4 sentinels (ErrInvalidQuery, ErrDaumDisabled, ErrKNCSidecarDown, ErrEmptyRSSFeedList)
├── concurrent_test.go   — NFR-ADP9-002 race-clean workload (50 goroutines)
├── bench_test.go        — BenchmarkParseRSSFeed10Items + TestMain goleak
└── testdata/            — 5 RSS/Atom/JSON Feed fixtures + 1 KNC sidecar JSON

services/koreanews/                          (Python sidecar scaffold; v0.1 stub only)
├── Dockerfile + pyproject.toml + README.md
├── src/main.py (FastAPI stub returning 503)
└── tests/test_stub.py
```

### 2.2 Key Data Structures

**`Adapter` struct** (`koreanews.go`): HTTP clients per sub-source,
base URLs, user-agent, sub-source enable flags, RSS feed list,
healthcheck target. Immutable post-construction.

**`Options` struct** (`options.go`): `RSSEnabled bool` (default true),
`RSSFeeds []string` (cap 32), `RSSPerFeedTimeout time.Duration`
(default 30s), `DaumEnabled bool` (default false), `KNCEnabled
bool` (default false), `KNCBaseURL string` (default
`http://localhost:8002`), `MaxParallelFeeds int` (default 8),
`HTTPClient *http.Client`, `UserAgentVersion string`,
`HealthcheckTarget string`.

**Env-var loader** (`options.go`):
- `USEARCH_ADP009_RSS_ENABLED` → bool.
- `USEARCH_ADP009_RSS_FEEDS` → JSON array OR comma-list; ≤32
  entries (truncate with slog WARN if more).
- `USEARCH_ADP009_DAUM_ENABLED` → bool.
- `USEARCH_ADP009_KNC_ENABLED` → bool.
- `USEARCH_ADP009_KNC_BASE_URL` → string.

**Sentinels** (`errors.go`):
- `ErrInvalidQuery` — empty/whitespace.
- `ErrDaumDisabled` — Daum permanently disabled in v0.1 per robots.txt.
- `ErrKNCSidecarDown` — sidecar 503 / unreachable.
- `ErrEmptyRSSFeedList` — RSS enabled but no feeds configured.

### 2.3 Composite Hot-Path Flow

1. Validate `q.Text` (REQ-ADP9-005).
2. Build per-sub-source enable list from Options.
3. RSS path special case: if `RSSEnabled` and `len(RSSFeeds)==0` →
   `ErrEmptyRSSFeedList`.
4. Dispatch enabled sub-sources concurrently via internal errgroup
   (one worker per enabled sub-source).
5. Each sub-source returns `[]NormalizedDoc + error`; per-sub-source
   errors do NOT cancel siblings.
6. After all workers complete: merge per-sub-source `[]NormalizedDoc`,
   deduplicate via `dedupDocs()` (URL canonicalization + hash
   fallback), sort by `PublishedAt` descending then `SourceID`
   ascending.
7. Return merged result + composite `*SourceError` (if all
   sub-sources failed) or `(docs, nil)`.

### 2.4 RSS Path (REQ-ADP9-002 / -003)

`searchRSS(ctx, q)`:
1. Fan the configured RSS feed URLs across
   `errgroup.SetLimit(opts.MaxParallelFeeds)`.
2. Each worker derives per-feed ctx via
   `min(opts.RSSPerFeedTimeout, time-until-parent-deadline)`.
3. Invokes `gofeed.NewParser().ParseURLWithContext(perFeedCtx, feedURL)`.
4. Transforms `gofeed.Item` per §6.3 mapping table (Title, Link,
   Published/Updated, Description, Author).
5. Per-feed errors isolated; partial failures do not fail the
   sub-source.
6. After `eg.Wait()`, merge and return docs + per-feed-index error
   slice.

### 2.5 KNC Path (REQ-ADP9-007)

`searchKNC(ctx, q)`:
- When `opts.KNCEnabled=false` → `(nil, nil)` (no-op).
- When `opts.KNCEnabled=true` → POST to
  `${opts.KNCBaseURL}/search` with JSON body `{query, max_results}`.
- 503 (sidecar stub default) → `*SourceError{Unavailable, Cause:
  ErrKNCSidecarDown}`.
- 200 with JSON → decode + map each article per §6.3 KNC mapping.
- 4xx → `CategoryPermanent`; 5xx → `CategoryUnavailable`.

### 2.6 Daum Path (REQ-ADP9-006)

`searchDaum(ctx, q)`:
- When `opts.DaumEnabled=false` → `(nil, nil)` (no-op).
- When `opts.DaumEnabled=true` → `*SourceError{Permanent, Cause:
  ErrDaumDisabled, Notes: "subsource: daum"}`.

The flag is plumbed for future SPEC-ADP-009-DAUM consumption; the
stub deliberately ignores the flag at the implementation level.

### 2.7 Korean Locale Detection (REQ-ADP9-008)

`detectKorean(text)`:
1. Count runes where `unicode.Is(unicode.Hangul, r)`.
2. Divide by total non-whitespace runes.
3. ≥ 0.30 → return `"ko"`; else return `""`.

Applied per-doc after RSS parsing; `Lang` field set accordingly.

### 2.8 Integration Points

- **Consumed by**: `internal/adapters/registry.go::wrappedAdapter`.
- **Consumes**: `pkg/types`, `github.com/mmcdole/gofeed v1.3.0`,
  `internal/obs/reqid.NewTransport`,
  `golang.org/x/sync/errgroup`.
- **Downstream**: SPEC-FAN-001 cross-adapter dedup (the intra-adapter
  dedup runs first); SPEC-IDX-001 RRF.

### 2.9 New External Dependency

`github.com/mmcdole/gofeed v1.3.0` (MIT, well-maintained, ~5
transitive deps). Added via `go get` in run phase.

---

## 3. Test Coverage Notes

- Coverage meets 85% target.
- 40+ representative tests across the composite and 3 sub-sources.
- RSS path tested against 5 feed format fixtures (RSS 2.0, Atom 1.0,
  JSON Feed 1.1, malformed XML, empty feed).
- Per-feed isolation test: `RSSPerFeedTimeout=200ms`, one feed
  sleeps 1s → that feed times out, siblings succeed.
- Daum stub returns `ErrDaumDisabled` regardless of `DaumEnabled`
  flag (table test over both).
- KNC sidecar HTTP client tested against stub (503 default + 200 with
  JSON + 4xx + 5xx + ctx cancel).
- Hangul ratio detection: 6 inputs (pure Korean, pure English,
  50/50, 20/80, empty, whitespace-only).
- Dedup test over 5 fixtures including unparseable URL hash fallback
  + deterministic byte-equal output.
- `BenchmarkParseRSSFeed10Items` (NFR-ADP9-001).
- `concurrent_test.go::TestSearchConcurrentSafe` — NFR-ADP9-002.

---

## 4. Technical Decisions (Locked During Implementation)

| Decision | Rationale |
|----------|-----------|
| Single composite over three adapters | Cardinality (one Prometheus label); routing simplicity; future-portability. |
| Daum hard-disabled per robots.txt | `User-agent: * Disallow: /` verified via WebFetch; legal posture forbids v0.1 from shipping a scraper. |
| KNC default-off + sidecar scaffold | Python KNC v1.51 is 2022-stale; full sidecar implementation deferred to SPEC-ADP-009-KNC. Scaffold (Dockerfile + pyproject + stub) ships for future activation. |
| RSS default-on via gofeed | Most operationally valuable subsystem; covers small-press Korean publishers and operator-curated feeds not on Naver. |
| 32-feed cap | Bounds resource cost; exceeds truncate with slog WARN. Empty list with RSS enabled rejects with `ErrEmptyRSSFeedList`. |
| `errgroup.SetLimit(min(8, len(feeds)))` per-feed isolation | Mirrors SPEC-FAN-001 §2.5/§2.6 verbatim — pre-allocated per-index slices, no shared map writes. |
| Hangul ratio ≥ 0.30 heuristic | Empirical; cheap; handles mixed-locale feeds. Real lang-detect (SPEC-IDX-003) may upgrade. |
| `Score=0.5` constant | No engagement signal across all three sub-sources. |
| Intra-adapter dedup BEFORE FAN-001 cross-adapter dedup | Avoids cross-source false-positives if FAN-001 dedup is bypassed. |

---

## 5. Risks Mitigated

- **Per-feed failure cascading** → errgroup per-index slices; one
  feed's 4xx/5xx/malformed/timeout does not cancel siblings.
- **Daum scraping legal exposure** → hard-disable stub regardless of
  flag; future activation requires explicit legal review.
- **KNC library stale (2022)** → v0.1 ships scaffold only;
  operators implementing the handler against the documented
  contract own the risk.
- **EUC-KR encoding for legacy Korean feeds** → out of v0.1;
  gofeed honours XML `encoding=` declaration; invalid UTF-8 gets
  `Metadata["encoding_warning"]`.
- **32-feed cap exceeded silently** → slog WARN emitted on truncation.

---

## 6. Out-of-Scope Reminders (from spec.md §7)

- KoreaNewsCrawler full sidecar implementation → SPEC-ADP-009-KNC.
- Daum search activation → SPEC-ADP-009-DAUM.
- YAML file path for feed config → SPEC-ADP-CFG-001 (horizontal).
- EUC-KR / non-UTF-8 encoding conversion → future SPEC.
- Real language-detect library → SPEC-IDX-003.
- Pagination for RSS → not supported (RSS has no query-time
  pagination).

---

*End of SPEC-ADP-009 plan.md (post-hoc, v1.0)*
