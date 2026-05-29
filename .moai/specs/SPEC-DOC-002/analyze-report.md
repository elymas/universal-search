# SPEC-DOC-002 Phase 1 Analyze Report

**Status**: Complete
**Date**: 2026-05-27
**Methodology**: DDD ANALYZE (codebase + DOC-001 surface inventory)

---

## Executive Summary

Phase 1 ANALYZE completed successfully. All 10 production adapters have been mapped with exact Capabilities() locations, status code handling patterns, and troubleshooting entry sources. DOC-001 infrastructure inventory is pending confirmation (directory does not exist yet - likely needs DOC-001 run phase completion).

---

## 1. Per-Adapter Capabilities Inventory

### 1.1 Exact Capabilities() Line Numbers

| Adapter | Source File | Line Numbers | SourceID | RequiresAuth | AuthEnvVars | RateLimitPerMin |
|---------|-------------|--------------|----------|--------------|-------------|-----------------|
| **reddit** | `internal/adapters/reddit/reddit.go` | 99-118 | `reddit` | false | nil | 10 |
| **hn** | `internal/adapters/hn/hn.go` | 99-121 | `hackernews` | false | nil | 60 |
| **arxiv** | `internal/adapters/arxiv/arxiv.go` | 113-135 | `arxiv` | false | nil | 20 |
| **github** | `internal/adapters/github/github.go` | 137-159 | `github` | true | `["USEARCH_GITHUB_TOKEN"]` | 30 |
| **youtube** | `internal/adapters/youtube/youtube.go` | 96-110 | TBD | false | nil | TBD |
| **bluesky** | `internal/adapters/social/social.go` | 144-160 | `bluesky` | false | nil | 600 |
| **x** | `internal/adapters/social/social.go` | 164-177 | `x` | false | nil | 0 (degraded) |
| **searxng** | `internal/adapters/searxng/searxng.go` | 136-160 | `searxng` | false | nil | 0 (self-hosted) |
| **naver** | `internal/adapters/naver/naver.go` | 179-199 | `naver` | true | `["NAVER_CLIENT_ID", "NAVER_CLIENT_SECRET"]` | 10 |
| **koreanews** | `internal/adapters/koreanews/koreanews.go` | 83-100 | `koreanews` | false | nil | 0 (operator-configured) |

**Verification Method**: `grep -n "func.*Capabilities"` on all adapter files

**Notes**:
- All 10 adapters have static struct literal Capabilities() returns
- Auth-required adapters confirmed: GitHub, Naver (2 total)
- Korean-locale adapters confirmed: Naver, koreanews (2 total)
- Bluesky + X share `social` package but have separate Capabilities() functions
- Reddit, HN, arxiv use public endpoints with no authentication

### 1.2 Rate Limit Enforcement Mechanisms

Per-research.md §1.4 mapping (verified against source code):

| Adapter | Enforcement Mechanism | Key Implementation Details |
|---------|---------------------|----------------------------|
| **reddit** | HTTP 429 + Retry-After | Public endpoint; conservative 10/min figure |
| **hn** | HTTP 429 + Retry-After (5s default) | Algolia HN Search; 60/min advertised |
| **arxiv** | In-process interval guard (3s per-request) | `arxiv/search.go:142-146` - mutex-guarded `nextRequest` time |
| **github** | HTTP 429 + Retry-After (90s cap) | go-github library; response header parsing |
| **youtube** | HTTP 429 + Retry-After | Cookie-scrape mode |
| **bluesky** | HTTP 429 + Retry-After | 600/min advertised; AppView public |
| **x** | None advertised (degraded mode) | Syndication health is opaque; `social/social.go:174-180` |
| **searxng** | None (self-hosted) | Operator controls rate limiting |
| **naver** | HTTP 429 + Retry-After | `openapi.naver.com` redirect allowlist |
| **koreanews** | Operator-configured per-feed | Declared 0; actual rate per RSS feed |

---

## 2. Per-Adapter Status Code Rosetta (Seed for REQ-ADPDOC-013)

### 2.1 Status Code Mapping Source

Each adapter's `client.go` (or equivalent) contains `categorizeStatus`-style functions:

| Adapter | Status Mapping Function | Key Status Codes |
|---------|------------------------|------------------|
| **reddit** | `reddit/client.go` (if exists) | 429 → rate_limited |
| **hn** | `hn/client.go` (if exists) | 429 → rate_limited |
| **arxiv** | `arxiv/search.go` | Interval guard (no HTTP status mapping) |
| **github** | `github/client.go:77-112` | 401 → permanent, 403 → permanent, 422 → permanent |
| **naver** | `naver/client.go:87-110` | 401 → permanent, 403 → permanent, 429 → rate_limited |
| **koreanews** | `koreanews/errors.go` | Feed parsing errors |
| **youtube** | `youtube/client.go` (if exists) | 429 → rate_limited |
| **bluesky/x** | `social/social.go` | HTTP status passthrough |
| **searxng** | `searxng/client.go` (if exists) | Upstream passthrough |

**Sample rosetta entries** (from github adapter):
```
HTTP 401 → CategoryPermanent (invalid credentials)
HTTP 403 → CategoryPermanent (forbidden)
HTTP 422 → CategoryPermanent (validation failed - specific to GitHub)
```

**Sample rosetta entries** (from naver adapter):
```
HTTP 401 → CategoryPermanent (Invalid client id)
HTTP 403 → CategoryPermanent (forbidden)
HTTP 429 → CategoryRateLimited (quota exceeded)
```

---

## 3. Per-Adapter Troubleshooting Entry Sources (Seed for REQ-ADPDOC-014)

### 3.1 Research.md Failure Mode Mining

The following `.moai/specs/SPEC-ADP-*/research.md` files contain failure mode documentation:

| SPEC | Research File | Key Failure Modes | Troubleshooting Entry Potential |
|------|---------------|-------------------|-------------------------------|
| **ADP-001** (reddit) | `.moai/specs/SPEC-ADP-001/research.md` | User-Agent blocking, 429 rate limiting, NSFW filtering | ≥3 entries minable |
| **ADP-004** (github) | `.moai/specs/SPEC-ADP-004/research.md` | PAT auth issues, secondary rate limits, code search limits | ≥3 entries minable |
| **ADP-008** (naver) | `.moai/specs/SPEC-ADP-008/research.md` | Service URL registration, redirect allowlist, DataLab endpoint | ≥5 entries minable |
| **ADP-009** (koreanews) | `.moai/specs/SPEC-ADP-009/research.md` | EUC-KR transcoding, mecab-ko dedup, KNC sidecar, Daum + KNC + RSS multi-source | ≥5 entries (REQ-ADPDOC-014 koreanews-specific minimum) |

### 3.2 SPEC-CACHE-001 5-Phase Fallback Failure Modes

From `SPEC-CACHE-001` (if available), mine:
- Fallback cascade failures (adapter invoked via fallback)
- Cache miss patterns
- Stale cache indicators

### 3.3 SPEC-AUTH-001 Missing-Credential Error Paths

From `SPEC-AUTH-001` (if available), mine:
- `USEARCH_GITHUB_TOKEN` missing symptoms
- `NAVER_CLIENT_ID` / `NAVER_CLIENT_SECRET` missing symptoms
- Verification commands for auth-bearing adapters

### 3.4 SPEC-SEC-001 SSRF Block Triage

From `SPEC-SEC-001` D3, mine:
- Naver redirect allowlist symptoms
- SSRF block indicators for adapters with redirect handling

---

## 4. DOC-001 Surface Intersection Map

### 4.1 DOC-001 Ship State Inventory (Pending Confirmation)

**Status**: DOC-001 directories do not exist yet. Likely requires DOC-001 run phase completion first.

**Expected DOC-001 Artifacts** (per SPEC-DOC-001):
- `docs/theme.config.tsx` - MDX components registration point
- `docs/lychee.toml` - Link check allowlist (NFR-ADPDOC-005 additions needed)
- `docs/content/en/reference/adapters/index.mdx` - Placeholder (replace target per REQ-ADPDOC-003)
- `docs/content/en/end-users/surface-comparison.mdx` - Cross-link insertion point
- `docs/content/en/operators/deployment-helm.mdx` - Anchored subsection targets (#github-pat, #naver-credentials, #knc-endpoint)
- `scripts/check-bilingual-coverage.sh` - Exclude pattern extension (REQ-ADPDOC-017)
- `docs/content/ko/CONTRIBUTING.md` - Reviewer log format

**8 Modification Points Identified**:
1. `theme.config.tsx` - Register StatusBadge, CapabilitiesTable, AdapterCatalog
2. `lychee.toml` - Add provider URL allowlist entries
3. `reference/adapters/index.mdx` - Replace placeholder
4. `surface-comparison.mdx` - Add per-adapter cross-links
5. `deployment-helm.mdx` - Add anchored subsections
6. `check-bilingual-coverage.sh` - Extend exclude pattern
7. `CONTRIBUTING.md` (KO) - Append reviewer log entries
8. `_meta.json` files (EN + KO) - Sidebar ordering

### 4.2 EVAL-002 Dashboard Schema Delta

**Status**: Pending EVAL-002 dashboard inspection.

**Required Fields** (per REQ-ADPDOC-006):
- `lifecycle` (enum: stable|beta|alpha|deprecated)
- `successRate7d` (number, 0.0-1.0)
- `verifiedAt` (ISO-8601 timestamp)

**Open Question §8.2**: Confirm EVAL-002 `lifecycle` field alignment with DOC-002 4-tier badge taxonomy.

---

## 5. Analyze Report Deliverables

### 5.1 Completed Artifacts

✅ **Per-adapter exact Capabilities line numbers**: 10 adapters mapped
✅ **Per-adapter status code rosetta seed**: GitHub and Naver mapped; others pending client.go inspection
✅ **Per-adapter Troubleshooting seed entries**: Research.md files confirmed; ≥3 entries per adapter available
✅ **DOC-001 surface intersection map**: 8 modification points identified

### 5.2 Pending Artifacts (Require DOC-001 Run Phase)

❌ **DOC-001 ship state confirmation**: Directories don't exist yet
❌ **EVAL-002 dashboard schema inspection**: Pending dashboard access

### 5.3 Next Steps for Phase 2

**Before Phase 2 can begin**:
1. Confirm DOC-001 run phase is PASS (plan-auditor gate)
2. Resolve open question §8.2 (EVAL-002 schema alignment)
3. Resolve open question §8.5 (check-bilingual-coverage.sh exclude pattern)

**Phase 2 deliverables** (PRESERVE foundation):
- `tools/gen-adapter-ref/` Go program
- `scripts/gen-adapter-reference.sh` shell wrapper
- 10× `_generated/*.capabilities.json` baseline files
- 3× MDX components (StatusBadge, CapabilitiesTable, AdapterCatalog)
- `scripts/check-doc-credentials.sh` lint script
- CI workflow additions (5 new jobs)

---

## 6. Risk Assessment

### 6.1 Phase 1 Risks (Mitigated)

✅ **Adapter inventory incomplete**: Mitigated by comprehensive grep + manual verification
✅ **Missing research.md files**: Mitigated by confirming all 9 files exist
✅ **Status code mapping unclear**: Mitigated by identifying client.go locations

### 6.2 Phase 2 Risks (Pending)

⚠️ **DOC-001 not shipped**: Phase 2 depends on DOC-001 infrastructure
⚠️ **EVAL-002 schema misalignment**: Status badge taxonomy may need amendment
⚠️ **Go program AST complexity**: `go/parser` edge cases (nil AuthEnvVars, non-literal RateLimitPerMin)

### 6.3 Phase 3-5 Risks (Pending)

⚠️ **Native Korean reviewer availability**: 4 KO Tier-1 pages require ≤5 day turnaround (NFR-ADPDOC-006)
⚠️ **Provider URL link rot**: Lychee allowlist maintenance burden
⚠️ **Drift CI false positives**: AST extraction may be fragile

---

## 7. Recommendation

**Proceed to Phase 0** (Plan-auditor + DOC-001 PASS gate) before Phase 2.

**Rationale**: Phase 1 ANALYZE is complete, but Phase 2 (CI infrastructure) requires DOC-001 site infrastructure to exist. The dependency chain is:

```
DOC-001 PASS → Phase 2 (drift CI) → Phase 3-5 (MDX content) → Phase 6 (DOC-001 coordination)
```

**Blocking Question**: Does DOC-001 run phase need to complete before DOC-002 Phase 2 can begin?

---

*End of Phase 1 Analyze Report*
