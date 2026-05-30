---
id: SPEC-DOC-002
version: 0.2.0
status: approved
created: 2026-05-22
updated: 2026-05-31
author: limbowl
priority: P1
issue_number: 0
title: Adapter reference — per-adapter pages on the Nextra docs site covering authentication, rate limits, query syntax, error taxonomy, Korean-tokenizer setup, troubleshooting, and version compatibility with CI-gated drift detection against Go source
milestone: M9 — V1 release
owner: manager-docs
methodology: ddd
coverage_target: 85
depends_on: [SPEC-ADP-001, SPEC-ADP-002, SPEC-ADP-003, SPEC-ADP-004, SPEC-ADP-005, SPEC-ADP-006, SPEC-ADP-007, SPEC-ADP-008, SPEC-ADP-009, SPEC-IDX-001, SPEC-IDX-002, SPEC-IDX-003, SPEC-IDX-004, SPEC-IDX-005, SPEC-FAN-001, SPEC-CACHE-001, SPEC-EVAL-002, SPEC-DOC-001]
blocks: [SPEC-REL-001]
related: [SPEC-DEPLOY-001, SPEC-EVAL-001, SPEC-EVAL-003, SPEC-SEC-001]
---

# SPEC-DOC-002: Adapter reference — per-adapter pages with drift-gated Capabilities, status badges, and Korean-locale operator notes

## HISTORY

- 2026-05-31 (v0.2.0, limbowl via manager-spec — code-spec
  reconciliation + V1 scope reduction; status remains draft):
  grep-verified 4 stale code-spec contradictions against live
  source (HEAD) and corrected them; applied autonomous V1 scope
  reductions. No new REQs added; existing REQ semantics tightened
  to match reality.

  - **A1 — page slug `hackernews` (not `hn`)**: `internal/
    adapters/hn/hn.go:101` declares `SourceID: "hackernews"`. The
    drift gate keys on `Capabilities().SourceID` and REQ-ADPDOC-001
    mandates filename = SourceID, so the page MUST be
    `hackernews.mdx`. The `hn` package directory name is NOT the
    SourceID. Page slug = `hackernews`; "HN" / "Hacker News" is the
    display title only (frontmatter `DisplayName: "Hacker News"`
    from `hn.go:102`). All page lists, `_meta.json` ordering,
    catalog rows, and the drift/completeness scripts updated from
    `hn` → `hackernews`.

  - **A2 — social Capabilities is switch-dispatch over helper
    funcs, not per-file `{adapter}.go`**: `social.go:132`
    `Capabilities()` switches on `a.subSource` and returns
    `blueskyCapabilities()` (`social.go:144`) / `xCapabilities()`
    (`social.go:164`). There is NO `bluesky.go` / `x.go`. The AST
    extraction tool MUST special-case the `social` package: parse
    `social.go`, resolve the two package-level helper funcs
    `blueskyCapabilities()` + `xCapabilities()`, and emit two JSON
    fragments (`bluesky.capabilities.json`, `x.capabilities.json`).
    REQ-ADPDOC-007's "per-file `{adapter}.go` walk" assumption
    corrected to a SourceID-keyed registry that maps each SourceID
    to its (file, Capabilities-producing func) pair.

  - **A3 — `x` is a DISABLED v0 stub, not "alpha"/"degraded"**:
    `xCapabilities()` (`social.go:164-177`) returns
    `RateLimitPerMin: 0`, `DefaultMaxResults: 0`, Notes "DISABLED
    in v0. Set USEARCH_X_ENABLED=true ... no live path wired."
    There is no functional X search path at V1. The status badge
    taxonomy gains a `disabled` framing (compile/flag-gated stub,
    not a usable-but-unstable adapter). `x.mdx` MUST document the
    adapter as disabled / not-available-in-V1, not as "alpha". The
    old "degraded syndication" framing was inaccurate and is
    removed.

  - **A4 — EVAL-002 provides NO `adapter-status.json` and NO
    `lifecycle` field**: SPEC-EVAL-002 (PR #44, unmerged at this
    SPEC's authoring) ships Prometheus recording rules + a Grafana
    `adapter-reliability` dashboard + `/admin/health/adapters` +
    a `usearch_adapter_health_status` gauge. It does NOT export an
    `adapter-status.json` and has NO `lifecycle` field. The prior
    claim "SPEC-EVAL-002 owns the `lifecycle` field" was an
    over-claim and is removed. SPEC-DOC-002 DEFINES its own 4-tier
    lifecycle taxonomy and ships a STATIC, hand-curated
    `adapter-status.json` at V1 (populated by a manager-docs
    reviewer reading the EVAL-002 dashboard). The live EVAL-002
    reliability feed is a FORWARD-REFERENCE only — its dashboard +
    `/admin/health/adapters` endpoint are cross-linked from the
    status section, but DOC-002 does NOT depend on any nonexistent
    status export. Automated status-feed plumbing (live cron export
    + JSON-schema validation gate + staleness GitHub-Issue
    automation) is deferred.

  V1 scope reductions (autonomous):
  - Static `adapter-status.json` (hand-curated from the EVAL-002
    dashboard) is the V1 status-feed path. The live cron export,
    `adapter-status.schema.json` build-time validation, and
    `adapter-status-staleness` GitHub-Issue automation are deferred
    to a post-V1 EVAL-002 amendment. Status badges still render
    from the static JSON.
  - KO Tier-1 retained at exactly 4 pages (`index`, `naver`,
    `koreanews`, `errors`); per-adapter KO for the other 8 adapters
    stays deferred to V1.1 (unchanged from v0.1.0 D7, re-affirmed).
  - V1-essential surface kept intact: 10 per-adapter EN reference
    pages (real SourceIDs), the `gen-adapter-ref` drift CI gate
    (go/parser AST extraction of the 5 Capabilities literal fields
    vs committed `_generated/*.capabilities.json`), the shared
    error taxonomy page, and STATIC status badges.

- 2026-05-22 (initial draft v0.1.0, limbowl via manager-spec):
  M9 (V1 release)의 두 번째 SPEC. SPEC-DOC-001 (User guide —
  operator + end-user docs site on Nextra)이 docs **인프라**
  (Next.js 16 + Nextra v4 standalone app, bilingual EN+KO, 7-section
  IA, CI 게이트, gh-pages + 컨테이너 dual deploy)를 ship 한다면,
  본 SPEC은 그 인프라 위에 **10개 production 어댑터의 operator-
  facing 도메인 콘텐츠**를 채운다. roadmap.md:113 ("SPEC-DOC-002 |
  Adapter reference | per-adapter keys, rate limits, Korean
  tokenizer setup, troubleshooting | manager-docs")의 full EARS
  확장.

  본 SPEC이 DOC-001과 분리된 이유는 단일한 mega-PR이 아닌 **two
  reviewer pools, two change cadences** 모델 때문이다.

  - DOC-001 reviewers: Next.js / Nextra 인프라 ownership (web/
    UI와 비슷한 코드 베이스), CI workflow ownership. 변경 cadence:
    낮음 — site infra는 stable.
  - DOC-002 reviewers: per-adapter SPEC owners (SPEC-ADP-001..009
    contributor pool), Korean-locale 검수자 (Naver + koreanews
    pages만). 변경 cadence: 높음 — vendor API 변경마다 (Reddit
    OAuth flow 업데이트, GitHub REST v3 → v4 마이그레이션, Naver
    Developer Console UI 개편, X/Twitter syndication 정책 변경)
    문서 갱신 필요.

  코드 베이스 분석 (research.md §1, HEAD 761381d 기준):

  - **10개 production 어댑터** (`noop` 제외) `internal/adapters/`
    트리에 배치. 9개 SPEC ID (SPEC-ADP-001..009)로 매핑되지만
    SPEC-ADP-006이 `social` 패키지 안에서 Bluesky + X **두 어댑터**를
    ship — 따라서 docs page는 **10페이지**.
  - 모든 어댑터가 동일한 5-file canonical layout 준수
    (`{name}.go` / `client.go` / `search.go` / `parse.go` /
    `errors.go`) — research §1.2 확인.
  - `Capabilities()` 함수가 `types.Capabilities` struct literal을
    반환 — `SourceID`, `RequiresAuth`, `AuthEnvVars`,
    `RateLimitPerMin`, `DefaultMaxResults` 5개 필드는 모두 정적
    리터럴이므로 `go/parser` AST 워크로 추출 가능 (drift CI 게이트
    근거 — research §3.1).
  - 인증 필요 어댑터는 **2개**: GitHub (`USEARCH_GITHUB_TOKEN`,
    `github.go:148`) + Naver (`NAVER_CLIENT_ID`,
    `NAVER_CLIENT_SECRET`, `naver.go:191`). 나머지 8개는
    `RequiresAuth: false`.
  - Rate limit semantics는 **이종적**: arxiv (per-instance
    interval guard), github (response header 기반), naver/reddit/
    hn/youtube/bluesky (HTTP 429 + Retry-After), searxng (self-
    hosted, 0), koreanews (operator-configured per-feed, declared
    0), x (advertised 0 — degraded syndication). Per-adapter
    rate-limits 섹션은 **단일 숫자가 아니라** mechanism + upstream
    quota + exhaustion behaviour을 함께 설명해야 함.
  - Korean-locale 특수 surface는 **2개 어댑터**:
    - Naver (ADP-008): `naver/client.go:22-24` 단일 redirect
      allowlist (`openapi.naver.com`만); `datalab.go` 별도 endpoint
      + 별도 rate budget; Korean 쿼리는 UTF-8 verbatim 전달
      (index-side tokenization은 SPEC-IDX-003 책임).
    - koreanews (ADP-009): `locale.go` EUC-KR → UTF-8 transcoding
      (legacy Korean RSS feeds 대응); `dedup.go` mecab-ko
      morpheme-level near-duplicate 감지; `knc.go` Python
      `services/storm/koreanewscrawler/` sidecar 브리지
      (`USEARCH_KNC_ENDPOINT` 환경변수 필요).

  본 SPEC이 신규로 도입하는 것:

  - `docs/content/{en,ko}/reference/adapters/` 서브트리: SPEC-
    DOC-001 REQ-DOC-008이 reserve한 placeholder 슬롯을 채운다.
    EN: 10개 어댑터 페이지 + index + 공유 errors 페이지 = **12
    MDX 파일**. KO: Tier-1 4개 페이지 (`naver`, `koreanews`,
    `errors`, `index`) + Tier-2 deferred = **4 MDX 파일 V1.0.0
    ship**.
  - `tools/gen-adapter-ref/main.go` 신규 Go 프로그램: `go/parser`로
    각 어댑터의 `Capabilities()` 함수 본문 AST를 walk → 구조화된
    JSON fragment 생성 (research §3.3).
  - `scripts/gen-adapter-reference.sh`: shell wrapper. SPEC-DOC-001
    `scripts/gen-cli-reference.sh` 와 동일 패턴 (REQ-DOC-007
    precedent).
  - `docs/content/en/reference/adapters/_generated/*.capabilities.
    json` × 10: 위 스크립트 출력. MDX page가 `<CapabilitiesTable
    src="_generated/{adapter}.capabilities.json" />` 커스텀 컴포넌트로
    rendering.
  - `docs/content/en/reference/adapters/_generated/adapter-status.
    json`: V1에서는 **STATIC, hand-curated** 파일 (DOC-002 소유).
    manager-docs 리뷰어가 EVAL-002 dashboard를 읽고 각 어댑터의
    lifecycle classification (+ 적용 가능한 경우 7-day success
    rate)을 채운다. SPEC-EVAL-002는 `adapter-status.json` export도
    `lifecycle` 필드도 제공하지 않음 (A4) — live cron export는
    post-V1 EVAL-002 amendment로 deferred.
  - `docs/components/{StatusBadge,CapabilitiesTable,AdapterCatalog}
    .tsx`: 3개 Nextra v4 MDX 커스텀 컴포넌트. SPEC-DOC-001
    `theme.config.tsx`에 등록.
  - `.github/workflows/docs.yml` 확장: `gen-adapter-ref-drift` job
    (auto-extracted JSON fragments에 대한 drift check) +
    `adapter-page-completeness` job (각 페이지가 10개 표준 섹션을
    non-empty로 가지는지 검증) + `check-doc-credentials` job. DOC-001
    docs.yml의 `bilingual-coverage` job은 `reference/adapters/`
    서브트리 exclude pattern 확장 필요 (open question §8.5). NOTE:
    `adapter-status-staleness` job (static V1 feed에는 의미 없음)
    + status-feed schema-validation은 post-V1 EVAL-002 amendment로
    deferred (A4).

  Pinned decisions (6개 scope pillar D1..D6 + 보조 D7..D8):

  (D1) **Page template — Logstash 8-section + SearXNG at-a-glance
       table = 10 standardized sections per page**. research §2.2-
       2.4. 10개 섹션: (1) Status & Compatibility (badge + SPEC ID
       + source path + last-verified), (2) Overview (1-paragraph
       upstream provider + use case), (3) Setup (auth env vars
       + provider account registration + Korean-locale
       prerequisites where applicable), (4) Capabilities
       (auto-extracted table via `<CapabilitiesTable>`),
       (5) Query syntax (what user query strings translate to),
       (6) Rate limits (advertised value + enforcement mechanism
       + upstream quota link + exhaustion behaviour), (7) Error
       reference (cross-link to shared `errors.mdx` + adapter-
       specific status code rosetta), (8) Troubleshooting (5-
       field format: Symptom → Likely Cause → Diagnostic Command
       → Resolution → Related SPECs), (9) Version compatibility
       (usearch versions × upstream provider API versions matrix
       with quarterly attestation date), (10) Related (cross-
       links to operator docs, end-user docs, SPECs). Anti-
       decision: single mega-page (research §9 Alternative A),
       per-adapter godoc dump (research §9 Alternative C).

  (D2) **Content sourcing — hybrid hand-authored + auto-extracted**.
       Capabilities table + status badge auto-extracted from Go
       source / EVAL-002 dashboard with CI drift gate; all other
       content (prose, troubleshooting decision trees, Korean-
       locale operational notes, provider URLs, version
       compatibility attestations) hand-authored by manager-docs
       agent with native-Korean reviewer signoff for KO Tier-1
       pages. Anti-decision: full MDX auto-generation (research
       §9 Alternative C) — produces low-quality pages for the
       high-value narrative content.

  (D3) **Bluesky vs X — separate reference pages despite shared
       Go package** (`internal/adapters/social/`). research §1.2,
       §1.7. Rationale: operator-facing setup, rate limits
       (Bluesky 600/min advertised vs X 0 — DISABLED v0 stub),
       reliability profile (Bluesky beta vs X disabled/not-
       available-in-V1 at ship time), and Korean-locale relevance
       differ
       materially. Shared content (URL extraction, parse rules,
       scoring) cross-linked between the two pages via a shared
       "Shared implementation notes" sidebar component, NOT
       duplicated.

  (D4) **Drift detection — `Capabilities()` AST-extracted JSON
       fragments, gated by CI**. research §3.3. Mechanism:
       `tools/gen-adapter-ref/main.go`, driven by a SourceID-keyed
       registry (see REQ-ADPDOC-007 — handles the `hn/`→`hackernews`
       slug rename and the `social.go` switch-dispatch over
       `blueskyCapabilities()`/`xCapabilities()` helper funcs, NOT
       a naive per-file `{adapter}.go` glob), resolves each
       adapter's `Capabilities()` struct literal (NOT a binary
       execution path — purely static AST analysis, fast +
       deterministic), extracts struct literal fields, emits
       `_generated/{adapter}.capabilities.json`. The
       `<CapabilitiesTable src="_generated/{adapter}.capabilities
       .json" />` MDX component imports the JSON at build time.
       CI `gen-adapter-ref-drift` job re-runs the script,
       diffs against committed JSON — any difference fails the
       PR. Prose around the table can drift independently
       (manager-docs review owns prose). Anti-decision:
       godoc-based extraction (research §9 Alternative C),
       runtime introspection via `usearch debug adapters` CLI
       (would require binary build at every CI run — too slow).

  (D5) **Status badge taxonomy — 4 tiers DEFINED by DOC-002,
       rendered from a STATIC build-time JSON import at V1**.
       Taxonomy:
       - `stable`: SPEC-ADP-* `status: implemented` AND (per the
         EVAL-002 dashboard) 7-day rolling success rate ≥ 0.95.
       - `beta`: `status: implemented` AND success rate 0.80-0.94.
       - `disabled`: adapter is a compile/flag-gated stub with no
         live path at V1 — currently the `x` adapter
         (`xCapabilities()` `social.go:164`, "DISABLED in v0 ...
         no live path wired", `RateLimitPerMin: 0`). NOT a
         usable-but-unstable adapter; the badge MUST read
         "disabled / not available in V1".
       - `deprecated`: reserved for post-V1 adapter removal
         flow; not used at V1.0.0 ship.
       (The earlier `alpha`/"degraded syndication" tier is removed
       — it mis-framed `x`, which is a disabled stub, not a
       degraded live adapter.) SPEC-DOC-002 OWNS the entire
       lifecycle taxonomy definition + the `<StatusBadge>` MDX
       presentation AND the V1 status feed: a STATIC, hand-curated
       `adapter-status.json` populated by a manager-docs reviewer
       reading the EVAL-002 dashboard. SPEC-EVAL-002 does NOT own a
       `lifecycle` field and ships no `adapter-status.json` export
       (see A4 in HISTORY v0.2.0). The live EVAL-002 reliability
       feed (`/admin/health/adapters` + `usearch_adapter_health_
       status` gauge + Grafana `adapter-reliability` dashboard) is
       a FORWARD-REFERENCE, cross-linked but not depended on.
       Automated status plumbing (live cron export, JSON-schema
       build-time validation, staleness GitHub-Issue automation) is
       DEFERRED to a post-V1 EVAL-002 amendment.

  (D6) **Korean-tokenizer documentation scope — cross-link only,
       no duplication**. research §10.1 open question proposal.
       Naver + koreanews pages include a **3-line summary** of
       Korean-locale prerequisites + a prominent cross-link to
       SPEC-DOC-001's KO-authoritative
       `operators/korean-locale-setup.mdx` page (mecab-ko Meili
       plugin setup, SPEC-IDX-003 sidecar provisioning, EUC-KR
       legacy feed handling). Full procedure NOT duplicated.
       Rationale: single source of truth; DOC-001's page is
       reviewed by Korean-locale subject-matter experts and
       is bilingual-tier-1; DOC-002 adapter pages should
       reference, not republish. Anti-decision: full duplication
       (would create drift between two KO-authoritative
       documents).

  (D7) **Bilingual coverage — adjusted exclude pattern; only 4
       KO pages Tier-1**. research §5. Per SPEC-DOC-001 D3,
       Tier-1 KO coverage applied to operator-core content.
       For reference docs, KO coverage targets the Korean
       operator's primary entry points: `naver.mdx`,
       `koreanews.mdx`, `errors.mdx`, `index.mdx`. The other 8
       adapter pages are EN-authoritative at V1.0.0 with KO
       Tier-2 deferred to V1.1 minor releases (consistent with
       SPEC-DOC-001 D3 reference subtree exclude pattern).
       `scripts/check-bilingual-coverage.sh` (DOC-001 ownership)
       SHALL be extended to recognize `reference/adapters/`
       Tier-1 set explicitly. SPEC-DOC-001 owner sign-off
       required (open question §8.5).

  (D8) **Secret leakage prevention in examples — placeholder-only
       policy + lint script**. research §6 + §8. All example
       env-var values in adapter pages SHALL use placeholders
       (`<YOUR_NAVER_CLIENT_ID>`, `<PERSONAL_ACCESS_TOKEN>`).
       No example SHALL contain a value resembling a real
       credential pattern (40-char hex strings, common cloud
       provider prefixes, GitHub PAT prefixes, etc.).
       `scripts/check-doc-credentials.sh` (NEW, hard-fail CI
       gate) scans all adapter MDX for known credential-shaped
       patterns; finding = CI fail. Complementary to SPEC-SEC-
       001 D2 gitleaks pre-commit hook (which catches commit-time
       leaks); DOC-002 lint catches **shape-resembling**
       placeholders that gitleaks would miss but operators might
       copy-paste-edit. The exact pattern list lives in
       `scripts/check-doc-credentials.sh` config (open question
       §8 reserves pattern tuning to plan-auditor + SEC-001
       owner coordination — pattern duplication with `.gitleaks
       .toml` should be deliberate, not coincidental).

  Companion artifacts:
  - `.moai/specs/SPEC-DOC-002/research.md` — Phase 0.5 research
    (≥800 lines, 12 sections: adapter inventory, reference-doc
    pattern survey, drift detection design, content sourcing
    strategy, integration with DOC-001, failure modes, reviewer
    pool, risk matrix, alternatives, open questions, verification
    trail).
  - `.moai/specs/SPEC-DOC-002/plan.md` — DDD phased plan
    (ANALYZE existing adapter surface → PRESERVE behaviour
    description fidelity → IMPROVE with new MDX content + drift
    CI + status badges).

  17 EARS REQs (8 × P0 + 7 × P1 + 2 × P2) + 6 NFRs + 1 new Go
  program + 1 new shell script + 3 new MDX components + 12 EN
  MDX pages + 4 KO MDX pages + 2 new CI jobs (gen-adapter-ref-
  drift, adapter-page-completeness). Methodology: **DDD**
  (consolidation + documentation of existing adapter behavior —
  byte-fidelity preservation of Capabilities() AST extraction +
  prose IMPROVE on top). Coverage target 85% applies to the new
  Go program (`tools/gen-adapter-ref/`) + shell scripts + MDX
  components; MDX content measured by completeness percentage
  per REQ-ADPDOC-008 not test coverage. Harness: **standard**
  (P1 docs SPEC, no security domain — Sprint Contract
  RECOMMENDED but not required per `.claude/rules/moai/design/
  constitution.md` §11). Owner: manager-docs.

---

## 1. Overview

SPEC-DOC-002는 M9 (V1 release)의 두 번째 SPEC이자 SPEC-DOC-001
docs site의 `reference/adapters/` IA 슬롯 (DOC-001 REQ-DOC-008로
reserve됨)을 **operator-facing 어댑터 도메인 콘텐츠**로 채우는
SPEC다. 본 SPEC은 **새로운 어댑터를 발명하지 않으며**, 10개
production 어댑터의 (a) Capabilities + status를 코드/대시보드에서
추출, (b) 인증 / 쿼리 / 레이트 / 에러 / Korean-locale / 트러블슈팅
narrative를 hand-author, (c) 드리프트 CI 게이트 + 완비성 게이트로
보호 — 의 세 축으로 정리한다.

### 1.1 What ships

| Layer | Artifact | Purpose |
|-------|----------|---------|
| Content | `docs/content/en/reference/adapters/index.mdx` (NEW) | 어댑터 카탈로그 + filterable table + 상태 배지 |
| Content | `docs/content/en/reference/adapters/{adapter}.mdx` × 10 (NEW) | per-adapter 10-section reference page (reddit, hackernews, arxiv, github, youtube, bluesky, x, searxng, naver, koreanews) — page slug = `Capabilities().SourceID`, so the HN page is `hackernews.mdx` (display title "Hacker News") |
| Content | `docs/content/en/reference/adapters/errors.mdx` (NEW) | 공유 `*types.SourceError` Category 레퍼런스 |
| Content | `docs/content/ko/reference/adapters/{index,naver,koreanews,errors}.mdx` × 4 (NEW) | Tier-1 KO 번역 (D7) |
| Generated | `docs/content/en/reference/adapters/_generated/{adapter}.capabilities.json` × 10 (NEW) | drift-gated Capabilities() AST 추출 결과 |
| Generated | `docs/content/en/reference/adapters/_generated/adapter-status.json` (NEW) | STATIC hand-curated lifecycle feed (DOC-002-owned; populated from the EVAL-002 dashboard; NOT an EVAL-002 export) |
| Component | `docs/components/StatusBadge.tsx` (NEW) | `<StatusBadge adapter="...">` MDX component |
| Component | `docs/components/CapabilitiesTable.tsx` (NEW) | `<CapabilitiesTable src="...">` MDX component |
| Component | `docs/components/AdapterCatalog.tsx` (NEW) | filterable adapter catalog (used by index.mdx) |
| Tool | `tools/gen-adapter-ref/main.go` (NEW) | go/parser AST extraction of Capabilities() literals |
| Tool | `tools/gen-adapter-ref/main_test.go` (NEW) | 85%+ coverage on AST extraction |
| Script | `scripts/gen-adapter-reference.sh` (NEW) | shell wrapper invoking the Go tool |
| Script | `scripts/check-adapter-page-completeness.sh` (NEW) | 10-section completeness gate |
| Script | `scripts/check-doc-credentials.sh` (NEW) | placeholder-only policy lint (D8) |
| CI | `.github/workflows/docs.yml` (modified) | new jobs: `gen-adapter-ref-drift`, `adapter-page-completeness`, `check-doc-credentials`; modified `bilingual-coverage` exclude pattern (the `adapter-status-staleness` job is DEFERRED — A4/NFR-ADPDOC-003) |
| Config | `docs/content/en/reference/adapters/_meta.json` (NEW) | Nextra sidebar ordering for the 12 EN pages |
| Config | `docs/content/ko/reference/adapters/_meta.json` (NEW) | KO sidebar (4 pages) |

### 1.2 Motivation

V1.0.0 ship 직전 operator-facing 어댑터 가이드의 부재는
**onboarding-blocking**이다. SPEC-DOC-001이 docs site **인프라**를
ship 해도 다음 시나리오가 해결되지 않는다:

- 새 self-hosted operator가 `usearch query` 실행 → GitHub adapter가
  401 반환 → operator는 무엇이 잘못된지 모름 (Capabilities는 코드
  안에 묻혀 있고, `registry.go:124-128`의 `ErrMissingAuth` 에러는
  CLI 출력만 보여줌). reddit operator-facing 가이드 없이는
  `USEARCH_GITHUB_TOKEN` 발급 → scope 선택 → 환경변수 set → restart
  의 단계를 시행착오로 학습해야 함.
- Korean operator가 Naver Developer Console에서 Application
  등록 → `NAVER_CLIENT_ID` + `NAVER_CLIENT_SECRET` 발급 → 환경변수
  set → 그래도 검색 결과 0건. 원인: Naver Developer Console의
  "Service URL" 등록 누락. 이 step은 코드 어디에도 documented되어
  있지 않음 (`internal/adapters/naver/client.go:22-24`의 redirect
  allowlist 주석은 개발자용이고, operator는 읽지 않음).
- Korean analyst persona (SPEC product.md §3)가 koreanews adapter의
  `dedup.go` 동작이 mecab-ko-aware임을 알지 못해 "왜 Hankyoreh
  뉴스가 검색 결과에 1건만 나오는가" 혼란 — 사실은 daum + 직접
  RSS + KNC 3중 syndication을 dedup한 결과. documented되지 않으면
  operator는 bug로 인식.
- arxiv adapter의 `RateLimitPerMin: 20`은 advertised value이지만
  enforcement는 in-process interval guard (`arxiv/search.go:142-
  146`). operator가 fanout 동시 호출 시 적용되는 실효 rate를
  알려면 코드를 읽어야 함.
- X/Twitter adapter는 V1에서 **DISABLED v0 stub** (`social/
  social.go:164-177` `xCapabilities()`: "DISABLED in v0 ... no
  live path wired", `RateLimitPerMin: 0`, `DefaultMaxResults: 0`).
  `USEARCH_X_ENABLED=true` 없이는 live search path가 wired되지
  않음. operator가 X 결과가 항상 0건인 이유를 코드를 읽지 않고는
  알 수 없으므로, `x.mdx`는 이 adapter를 "disabled / not available
  in V1"로 정직하게 framing해야 함 (usable-but-unstable로
  오인하게 하는 "alpha"/"degraded" 표현 금지).

본 SPEC이 **PASS**해야 하는 이유: M9 exit criterion "docs site
live" (SPEC-DOC-001 PASS) + V1.0.0 binary 배포에서 외부 operator의
어댑터 onboarding 시간 단축이 commit된 결과 (`roadmap.md` M9
narrative). SPEC-DOC-002가 PASS하지 못하면 docs site는 `reference/
adapters/` 슬롯에 SPEC-DOC-001이 만든 placeholder만 표시 → SPEC-
REL-001 release notes에 "complete adapter reference" claim 불가 →
V1.0.0 태깅 차단.

### 1.3 Forward-compatibility commitments

본 SPEC은 다음 sibling/downstream SPEC과의 contract를 명시한다:

- **SPEC-DOC-001 (M9 sibling, drafted 2026-05-22)** — docs site
  infrastructure. 본 SPEC은 DOC-001이 reserve한 `reference/
  adapters/` IA 슬롯을 채우며, DOC-001의 Nextra v4 app, lychee
  link-check (REQ-DOC-013), screenshot freshness (REQ-DOC-014),
  bilingual coverage gate (REQ-DOC-016), gh-pages + container
  dual deploy (REQ-DOC-015)를 모두 consume. DOC-001 PASS가
  DOC-002 run phase 시작의 hard prerequisite.

- **SPEC-EVAL-002 (M8 sibling, on unmerged PR #44 at this
  SPEC's authoring)** — adapter reliability observability.
  EVAL-002 ships Prometheus recording rules + a Grafana
  `adapter-reliability` dashboard + `/admin/health/adapters` +
  a `usearch_adapter_health_status` gauge. It does **NOT** export
  an `adapter-status.json` and has **NO** `lifecycle` field (A4).
  DOC-002 therefore DEFINES its own 4-tier lifecycle taxonomy (D5)
  and ships a STATIC, hand-curated `adapter-status.json` at V1
  (manager-docs reads the EVAL-002 dashboard to populate it). The
  live EVAL-002 feed is a FORWARD-REFERENCE: DOC-002 cross-links
  the reliability dashboard + health endpoint but does not depend
  on any EVAL-002 status export. A live cron export + schema
  alignment is a post-V1 EVAL-002 amendment, not a V1 dependency.

- **SPEC-DEPLOY-001 (M9 sibling, not yet drafted)** — Helm
  chart. DEPLOY-001의 Helm `values.yaml`에서 각 어댑터의 인증
  환경변수 (`USEARCH_GITHUB_TOKEN`, `NAVER_CLIENT_ID`,
  `NAVER_CLIENT_SECRET`, `USEARCH_KNC_ENDPOINT`)가 표준 secret
  reference로 노출됨. 본 SPEC의 per-adapter Setup 섹션은
  DEPLOY-001의 Helm values 키 이름을 cross-link하여 operator가
  "docs 페이지의 env var → Helm values key → K8s Secret"
  경로를 trace 가능하게 함.

- **SPEC-REL-001 (M9, not yet drafted)** — V1.0.0 tag + release
  notes. release notes에 "10 production adapters fully
  documented" claim의 evidence가 본 SPEC의 12 EN MDX 페이지 +
  4 KO Tier-1 페이지의 CI completeness gate PASS.

- **SPEC-ADP-001..009 (M3 implemented, ADP-006 includes Bluesky
  + X)** — adapter implementations. 본 SPEC의 모든 페이지가
  source path를 cite. 어떤 SPEC-ADP-* amendment (예: ADP-006
  X v2 OAuth 마이그레이션, ADP-004 GitHub REST → GraphQL 전환)도
  본 SPEC의 대응 페이지 + Capabilities JSON fragment 동시
  업데이트를 요구 (drift CI 게이트가 enforce).

- **SPEC-IDX-003 (M3 implemented)** — Korean tokenization
  (mecab-ko Meili plugin). 본 SPEC의 Naver + koreanews 페이지가
  IDX-003 setup 가이드 (SPEC-DOC-001 `operators/korean-locale-
  setup.mdx`로 surface)를 cross-link. 본 SPEC은 IDX-003 콘텐츠를
  재게재하지 않음 (D6).

- **SPEC-FAN-001 (M3 implemented)** — fanout dispatcher. 본
  SPEC의 각 어댑터 페이지의 "Rate limits" 섹션은 FAN-001의
  `CategoryRateLimited` 처리 + retry semantics를 cross-link.

- **SPEC-CACHE-001 (M3 implemented)** — 5-phase access fallback.
  본 SPEC의 troubleshooting 섹션은 CACHE-001의 fallback failure
  modes를 cross-link (SPEC-DOC-001 troubleshooting top-10도
  같은 cross-link 사용).

- **SPEC-SEC-001 (M8 drafted)** — security hardening. 본 SPEC의
  D8 credential placeholder 정책 + `scripts/check-doc-credentials
  .sh` lint는 SEC-001 D2 gitleaks의 보완 (operator가 docs
  example의 placeholder를 실제 값으로 치환 후 commit 시도하는
  시나리오 대비).

### 1.4 Pinned architectural decisions

HISTORY의 D1..D8 8개 결정은 §2 requirements를 bind하는 constraint
이다. 재논의 대상이 아니며, annotation cycle에서만 modification
가능.

---

## 2. EARS Requirements

### 2.1 Per-adapter page template + IA (D1)

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-ADPDOC-001** | Ubiquitous | The docs site SHALL contain one MDX reference page per production adapter at `docs/content/en/reference/adapters/{sourceID}.mdx`, where `{sourceID}` is the adapter's `Capabilities().SourceID` value, for each of the 10 production adapters: `reddit`, `hackernews`, `arxiv`, `github`, `youtube`, `bluesky`, `x`, `searxng`, `naver`, `koreanews`. Note: the HN adapter's Go package is `internal/adapters/hn/` but its `SourceID` is `"hackernews"` (`hn.go:101`), so the page is `hackernews.mdx` (display title "Hacker News"). The `noop` adapter (SourceID `"reference"`, test-only fixture) SHALL NOT have a public reference page. Each page filename MUST match the adapter's `Capabilities().SourceID` value (verified by `scripts/check-adapter-page-completeness.sh` against the SourceID-keyed registry, NOT the package directory name). | P0 | 10 EN MDX files exist; filename = SourceID for each (e.g., `hackernews.mdx` matches `hn.go:101` `SourceID: "hackernews"`; `bluesky.mdx` matches `social.go:146` `SourceID: "bluesky"`); no `noop.mdx` / `reference.mdx` present. |
| **REQ-ADPDOC-002** | Ubiquitous | Each per-adapter reference page SHALL contain exactly 10 top-level sections in this order: (1) `## Status & Compatibility`, (2) `## Overview`, (3) `## Setup`, (4) `## Capabilities`, (5) `## Query syntax`, (6) `## Rate limits`, (7) `## Error reference`, (8) `## Troubleshooting`, (9) `## Version compatibility`, (10) `## Related`. Each section heading SHALL appear exactly once per page. Sections SHALL NOT be skipped — an inapplicable section (e.g., "Setup" for a no-auth adapter) SHALL render the explicit text "Not required — public endpoint" rather than being omitted. | P0 | All 10 EN pages parse to AST with exactly 10 H2 headings in the prescribed order; `check-adapter-page-completeness.sh` validates by matching the headings against an expected ordered list per page. |
| **REQ-ADPDOC-003** | Ubiquitous | The docs site SHALL contain `docs/content/en/reference/adapters/index.mdx` rendering an adapter catalog: a sortable + filterable table listing all 10 adapters with columns `Adapter`, `Status` (badge via `<StatusBadge>`), `Category` (one of `search-engine` / `social` / `academic` / `news` / `korean-locale`), `Auth required` (`yes` / `no`), `Korean-locale optimized` (`yes` / `no`), `Detail page` (link). The catalog SHALL be rendered via the `<AdapterCatalog>` MDX component reading category metadata from per-page frontmatter. The index page SHALL link to the shared `errors.mdx` from a "Common error categories" footnote. | P0 | `index.mdx` exists; rendered HTML contains a table with 10 rows and the prescribed columns; clicking the "Category: news" filter narrows to `koreanews` + `naver` (news + Korean-locale overlap). |
| **REQ-ADPDOC-004** | Ubiquitous | The docs site SHALL contain `docs/content/en/reference/adapters/errors.mdx` documenting the 5 `*types.SourceError` Category values from `pkg/types/errors.go`: `CategoryPermanent`, `CategoryRateLimited`, `CategoryUnavailable`, `CategoryTransient`, `CategoryUnknown`. Each Category SHALL describe: typical triggering HTTP status codes, fanout dispatcher behaviour (SPEC-FAN-001 cross-link for retry semantics), `RetryAfter` handling (where applicable), and one example error message from a real adapter. The page SHALL be linked from every per-adapter page's `## Error reference` section. | P0 | `errors.mdx` exists with 5 H3 subsections (one per Category); each subsection contains the 4 required fields; lychee link-check from each per-adapter page resolves to this page successfully. |

### 2.2 Status badge taxonomy + EVAL-002 data feed (D5)

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-ADPDOC-005** | Ubiquitous | The docs site SHALL render a status badge at the top of each per-adapter reference page using the `<StatusBadge adapter="{sourceID}">` MDX component. The component SHALL import the STATIC, hand-curated `_generated/adapter-status.json` (DOC-002-owned; see A4/D5) at build time and render one of four lifecycle values: `stable` (green badge), `beta` (yellow badge), `disabled` (grey badge — compile/flag-gated stub with no live V1 path, e.g. `x`), `deprecated` (red badge, reserved for post-V1). The taxonomy mapping rules SHALL be: `stable` = SPEC-ADP `status: implemented` AND 7-day rolling success rate (read from the EVAL-002 dashboard) ≥ 0.95; `beta` = `status: implemented` AND 0.80 ≤ rate < 0.95; `disabled` = adapter is a flag-gated stub with no live path (`x` per `xCapabilities()`). The component SHALL also render the 7-day success rate value (where applicable) and the `verifiedAt` ISO-8601 timestamp from the JSON. | P0 | `<StatusBadge>` component implemented; rendering `bluesky.mdx` shows badge with the JSON-driven lifecycle + rate + verifiedAt; `x.mdx` shows a `disabled` badge; unit test asserts taxonomy boundary cases (0.949 = beta, 0.950 = stable) + the `disabled` rendering path. |
| **REQ-ADPDOC-006** | Ubiquitous | The V1 `adapter-status.json` SHALL be a STATIC, hand-curated, committed file (DOC-002-owned; NOT produced by SPEC-EVAL-002, which ships no such export — see A4). It SHALL be a top-level object keyed by adapter `SourceID`, each value an object with fields `lifecycle` (enum: stable\|beta\|disabled\|deprecated), `successRate7d` (number 0.0-1.0, OPTIONAL for `disabled` adapters), `verifiedAt` (ISO-8601 timestamp). Unknown adapter keys SHALL be ignored by `<StatusBadge>`. A known adapter missing required fields SHALL cause `<StatusBadge>` to fall back to a neutral "Status unknown" badge. The lifecycle taxonomy values are normative in this SPEC (D5). NOTE: a live cron-published feed + a build-time JSON-Schema validation step (`adapter-status.schema.json`) + staleness automation are DEFERRED to a post-V1 EVAL-002 amendment (see Exclusions); they are NOT V1 ship requirements. | P1 | Static `adapter-status.json` is committed and parses; every key matches a real SourceID; injecting a malformed entry (missing `lifecycle`) renders the fallback badge; `x` carries `lifecycle: disabled`. |

### 2.3 Drift detection — auto-extracted Capabilities (D4)

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-ADPDOC-007** | Ubiquitous | The repository SHALL provide `tools/gen-adapter-ref/main.go`, a Go program that, driven by a SourceID-keyed adapter registry (NOT a naive per-file `{adapter}.go` glob), parses the relevant Go source via `go/parser`, resolves the `types.Capabilities` struct literal that backs each adapter's `Capabilities()`, AND emits one JSON file per SourceID to `docs/content/en/reference/adapters/_generated/{sourceID}.capabilities.json` with the schema: `{sourceID, requiresAuth, authEnvVars, rateLimitPerMin, defaultMaxResults, sourcePath, sourceLine, extractedAt}`. The registry MUST handle these real-source shapes: (a) standard adapters whose `Capabilities()` method returns a struct literal directly in `internal/adapters/{pkg}/{pkg}.go`; (b) the HN adapter, whose package dir is `hn/` but SourceID is `hackernews` → emit `hackernews.capabilities.json`; (c) the `social` package, whose `Capabilities()` (`social.go:132`) is a switch over `a.subSource` returning the package-level helper funcs `blueskyCapabilities()` (`social.go:144`) and `xCapabilities()` (`social.go:164`) — the tool MUST resolve both helper funcs and emit TWO JSON fragments (`bluesky` + `x`); there is no `bluesky.go` / `x.go`. The `noop` adapter (SourceID `reference`) is excluded. The shell wrapper `scripts/gen-adapter-reference.sh` SHALL invoke the Go program. The CI workflow `docs.yml` SHALL run the script as the `gen-adapter-ref-drift` job and fail if the committed JSON fragments differ from the freshly-generated output (drift = CI fail). | P0 | Go program builds; running it against the current adapter set produces 10 JSON files matching the per-SourceID Capabilities verbatim (incl. `hackernews` from `hn/`, and `bluesky`+`x` from `social.go` helper funcs); CI `gen-adapter-ref-drift` job fails when a committed JSON is artificially modified; modifying a real adapter's `RateLimitPerMin` (e.g., in `blueskyCapabilities()`) without updating the JSON also fails CI. |
| **REQ-ADPDOC-008** | Ubiquitous | Each per-adapter MDX page SHALL render its Capabilities table via `<CapabilitiesTable src="_generated/{adapter}.capabilities.json" />`. The component SHALL display the 5 extracted fields plus the source path + line number as a footer ("Extracted from `internal/adapters/{name}/{name}.go:NNN`"). The component SHALL NOT permit hand-overridden field values in MDX — to change a value, the underlying adapter Go source must be modified (which then triggers the drift gate). | P0 | All 10 EN pages use `<CapabilitiesTable>`; no per-page hardcoded Capabilities values; rendered HTML footer shows the correct source path + line number for each adapter (verifiable against `grep -n "Capabilities()" internal/adapters/*/[!_]*.go`). |

### 2.4 Per-adapter content sourcing (D2, D3)

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-ADPDOC-009** | Ubiquitous | The SPEC-ADP-006 `social` Go package SHALL be documented as TWO separate reference pages: `bluesky.mdx` and `x.mdx`. Each page SHALL have its own `Status & Compatibility`, `Setup`, `Capabilities`, `Rate limits`, and `Troubleshooting` sections; shared implementation notes (URL extraction, parse rules, scoring) SHALL be cross-linked between the two pages via a "Shared implementation notes" callout but NOT duplicated. Both pages SHALL appear in the `index.mdx` catalog as separate rows with category `social`. `x.mdx` SHALL frame the adapter as a DISABLED v0 stub (no live path at V1 without `USEARCH_X_ENABLED=true`; `RateLimitPerMin: 0`, `DefaultMaxResults: 0`) — its `Status & Compatibility` badge SHALL be `disabled`. | P0 | Both `bluesky.mdx` and `x.mdx` exist; `bluesky.mdx` shows `RateLimitPerMin: 600` (badge `beta`/`stable`) and `x.mdx` shows `RateLimitPerMin: 0` with a `disabled` badge + explicit "not available in V1" framing; `index.mdx` catalog renders 2 social-category rows; each page contains the shared-implementation callout linking the other. |
| **REQ-ADPDOC-010** | State-Driven | IF an adapter declares `RequiresAuth: true` in its `Capabilities()` (currently `github` AND `naver`), THEN its reference page's `## Setup` section SHALL include: (a) the upstream provider's account/application registration URL (link-checked by lychee), (b) the exact env var names from `AuthEnvVars` (cross-checked by `<CapabilitiesTable>`), (c) the recommended scopes/permissions to grant when issuing the token (e.g., GitHub PAT scopes; Naver app categories), (d) a verification command (`usearch query` with a known-safe query AND the adapter named in `--source` flag, asserting non-error response), (e) cross-link to SPEC-DEPLOY-001 Helm values key for the env var. If `RequiresAuth: false`, the `## Setup` section SHALL contain the text "Authentication: not required — public endpoint" with a 1-sentence explanation of the upstream access tier used. | P0 | `github.mdx` + `naver.mdx` Setup sections contain all 5 required fields; the other 8 pages contain the "not required" formulation; lychee link-check resolves all upstream provider URLs (with appropriate allowlist entries per SPEC-DOC-001 REQ-DOC-013). |
| **REQ-ADPDOC-011** | State-Driven | IF an adapter has Korean-locale-specific operational behaviour (currently `naver` AND `koreanews`), THEN its reference page SHALL include in `## Setup` a 3-line Korean-locale prerequisites summary AND a prominent cross-link to SPEC-DOC-001 `operators/korean-locale-setup.mdx` (KO-authoritative per DOC-001 D3). For `naver.mdx`: notes on Naver Developer Console "Service URL" registration + Korean query passes UTF-8 verbatim (no in-adapter tokenization) + DataLab endpoint distinction (`openapi.naver.com/v1/datalab/search` vs search endpoints). For `koreanews.mdx`: notes on EUC-KR legacy feed handling (`locale.go`), mecab-ko-aware dedup (`dedup.go`), AND KNC sidecar requirement (`USEARCH_KNC_ENDPOINT` env var + `services/storm/koreanewscrawler/` Python service). Full setup procedures SHALL NOT be duplicated from DOC-001. | P1 | `naver.mdx` Setup section contains the 3 Korean-specific notes + cross-link; `koreanews.mdx` Setup section contains the 3 Korean-specific notes + cross-link; both pages do NOT contain a full mecab-ko setup walkthrough (which lives in DOC-001 `operators/korean-locale-setup.mdx`). |

### 2.5 Rate limits, error mapping, troubleshooting (D1 sections 6-8)

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-ADPDOC-012** | Ubiquitous | Each per-adapter `## Rate limits` section SHALL document FOUR elements: (a) the advertised `RateLimitPerMin` value (auto-imported via `<CapabilitiesTable>`), (b) the enforcement mechanism — one of `in-process interval guard` (arxiv), `HTTP 429 response handling` (reddit/hn/github/naver/youtube/bluesky), `operator-configured per-feed` (koreanews), `none — self-hosted` (searxng), `none — disabled v0 stub` (x: no live path at V1), (c) a link to the upstream provider's published quota documentation (lychee-checked; for `x`, note no quota applies while disabled), (d) the exhaustion behaviour — fanout dispatcher returns `CategoryRateLimited` with `RetryAfter` from upstream response; SPEC-FAN-001 retry semantics cross-link. | P0 | All 10 pages' Rate limits section contains all 4 elements; element (b) matches research.md §1.4 inventory verbatim for each adapter; `x` reads `none — disabled v0 stub`; provider quota URLs resolve via lychee. |
| **REQ-ADPDOC-013** | Ubiquitous | Each per-adapter `## Error reference` section SHALL cross-link to `errors.mdx` (shared Category reference) AND provide an adapter-specific status code rosetta table with columns `HTTP status` / `Category` / `Cause` / `Operator action`. The rosetta SHALL list at minimum: the status codes handled by the adapter's `categorizeStatus`-style function (e.g., `naver/client.go:87-110` enumerates 401, 403, 429, 4xx, 5xx, 0) PLUS adapter-specific quirks (e.g., GitHub 422 "Validation failed" mapped to `CategoryPermanent`, Naver 401 "Invalid client id" mapped to `CategoryPermanent` with operator action "check NAVER_CLIENT_ID env var"). | P1 | Each page's Error reference section contains the rosetta table + the shared errors.mdx link; GitHub page lists 422 row; Naver page lists 401 + operator action mentioning the env var name. |
| **REQ-ADPDOC-014** | Ubiquitous | Each per-adapter `## Troubleshooting` section SHALL contain AT LEAST 3 entries in the 5-field format: `Symptom` / `Likely cause` / `Diagnostic command` / `Resolution` / `Related SPECs`. Entries SHALL be derived from: (a) the adapter's known failure modes documented in `.moai/specs/SPEC-ADP-*/research.md`, (b) SPEC-CACHE-001 5-phase fallback failure modes where the adapter is invoked via fallback, (c) SPEC-AUTH-001 missing-credential error path for auth-bearing adapters, (d) SPEC-SEC-001 SSRF block triage for adapters with redirect handling (currently `naver` per its redirect allowlist), (e) operator field reports surfaced during run-phase native-reviewer signoff. The `koreanews.mdx` Troubleshooting section SHALL have at minimum 5 entries due to multi-source aggregation complexity (Daum + KNC + RSS combinations). | P0 | All 10 pages contain ≥ 3 troubleshooting entries each (≥ 5 for koreanews); each entry has all 5 fields; "Related SPECs" field contains valid SPEC ID links resolved by lychee. |

### 2.6 Version compatibility + related (D1 sections 9-10)

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-ADPDOC-015** | Ubiquitous | Each per-adapter `## Version compatibility` section SHALL contain: (a) a table mapping `usearch version` × `upstream provider API version` × `last verified date` × `verifier` (manager-docs reviewer name from CONTRIBUTING.md log), (b) a "Last verified" date displayed prominently in the page frontmatter (`lastVerified: YYYY-MM-DD`). For V1.0.0 ship, each page SHALL have at least one verified-against row dated within 90 days of release. A CI `lastverified-staleness` check (part of `adapter-page-completeness`) SHALL warn (not fail) when a page's `lastVerified` frontmatter is older than 180 days. (This is distinct from — and replaces, at V1 — the deferred `adapter-status.json` mtime gate of NFR-ADPDOC-003.) | P1 | All 10 pages contain a non-empty Version compatibility table with at least one row; frontmatter `lastVerified` field populated and within 90 days of 2026-05-22 at V1.0.0 ship; backdating one page to 200 days produces a CI warning. |
| **REQ-ADPDOC-016** | Ubiquitous | Each per-adapter `## Related` section SHALL contain cross-links to: (a) the adapter's SPEC-ADP-XXX document, (b) SPEC-DOC-001 `end-users/surface-comparison.mdx` (decision matrix), (c) SPEC-DEPLOY-001 `operators/deployment-helm.mdx` with anchor to the adapter's env var subsection, (d) any SPECs cross-referenced from the page body (FAN-001, CACHE-001, IDX-003 for Korean adapters, EVAL-002 for status badge data lineage). All cross-links SHALL resolve via lychee internal-strict link-check (SPEC-DOC-001 REQ-DOC-013). | P1 | All 10 pages contain a Related section with ≥ 4 cross-links each; lychee resolves 100% of internal links (the 10 pages × ≥ 4 links = ≥ 40 internal references). |

### 2.7 Bilingual coverage (D7)

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-ADPDOC-017** | State-Driven | IF SPEC-DOC-002 ships at V1.0.0, THEN the docs site SHALL include KO Tier-1 translations of EXACTLY four reference adapter pages: `docs/content/ko/reference/adapters/index.mdx`, `naver.mdx`, `koreanews.mdx`, AND `errors.mdx`. KO translations SHALL be authored by manager-docs agent AND reviewed by a native-Korean-speaking reviewer (recorded in `docs/content/ko/CONTRIBUTING.md` reviewer log per SPEC-DOC-001 REQ-DOC-010). The remaining 8 adapter pages are EN-authoritative at V1.0.0 with KO Tier-2 deferred to V1.1. The `scripts/check-bilingual-coverage.sh` script (SPEC-DOC-001 owner) SHALL be amended to recognize the `reference/adapters/` Tier-1 set explicitly (excluding the 8 Tier-2 pages from the 90% gate while requiring all 4 Tier-1 pages). | P0 | 4 KO MDX files exist with corresponding native reviewer signoff in `content/ko/CONTRIBUTING.md`; bilingual coverage script passes at V1.0.0; deleting one KO Tier-1 page (e.g., `naver.mdx` KO) drops below threshold and fails CI; deleting any of the 8 Tier-2 EN-only pages does NOT fail (still EN-authoritative). |

### 2.8 Anti-patterns (D8 + Unwanted)

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-ADPDOC-018** | Unwanted | The docs site SHALL NOT contain example values that resemble real credentials in any adapter reference page. All env-var example values SHALL be placeholders matching the patterns `<UPPERCASE_NAME>`, `${UPPERCASE_NAME}`, `your-${name}-here`, or `example-value-not-real`. The script `scripts/check-doc-credentials.sh` (NEW, hard-fail CI gate) SHALL scan all `docs/content/{en,ko}/reference/adapters/*.mdx` for known credential-shaped patterns (the exact regex list lives in script config, intentionally aligned with the SPEC-SEC-001 D2 `.gitleaks.toml` rule set so both gates evolve together). Patterns covered SHALL include at minimum: AWS-style access key prefixes, GitHub personal access token prefixes, 40-character hexadecimal token strings outside fenced code blocks, JWT-shaped three-segment base64 strings, AND Naver client secret format. Any match SHALL fail the CI job. This requirement is complementary to SPEC-SEC-001 D2 gitleaks coverage (which catches commit-time leaks); this REQ catches realistic-looking placeholder leakage at PR time. | P0 | Script exists with regex set documented; injecting a realistic-shaped GitHub PAT pattern into `github.mdx` fails CI; injecting `<YOUR_GITHUB_TOKEN>` passes; existing placeholder patterns across all 12 pages return zero matches (clean baseline at V1.0.0 ship). |

---

## 3. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| **NFR-ADPDOC-001** | Drift gate runtime budget | The `gen-adapter-ref-drift` CI job SHALL complete within 60 seconds wall-clock on `ubuntu-24.04` hosted runner. The Go program `tools/gen-adapter-ref/main.go` SHALL process all 10 adapter packages via `go/parser` in ≤ 5 seconds; the remainder is checkout + dependency setup. Total docs CI overhead (SPEC-DOC-001 NFR-DOC-001 5-min budget + this) SHALL remain within SPEC-DOC-001 NFR-DOC-001 expanded ceiling of 6 minutes total. |
| **NFR-ADPDOC-002** | Page completeness runtime budget | The `adapter-page-completeness` CI job SHALL complete within 30 seconds. The job parses 12 EN MDX files + 4 KO Tier-1 MDX files using a JS-based MDX-to-AST walk asserting the 10-section heading list per page. |
| **NFR-ADPDOC-003** | Adapter status freshness (DEFERRED post-V1) | The V1 `adapter-status.json` is a STATIC hand-curated file (A4/D5); an mtime-based freshness gate is meaningless for a manually-edited file, so the `adapter-status-staleness` CI job + `docs/stale-adapter-status` GitHub-Issue automation are DEFERRED to the post-V1 EVAL-002 live-export amendment. V1 freshness is instead asserted by the per-page `lastVerified` frontmatter check (REQ-ADPDOC-015). When the live EVAL-002 export lands, this NFR's mtime gate is re-activated. |
| **NFR-ADPDOC-004** | Page completeness threshold | Each per-adapter MDX page SHALL contain ≥ 50 characters of plain text per section (after MDX → plaintext conversion stripping code blocks + frontmatter) across all 10 prescribed sections. Pages failing this threshold SHALL fail the `adapter-page-completeness` CI job. (Open question §8.8 — final threshold confirmed in plan-auditor.) |
| **NFR-ADPDOC-005** | Provider URL allowlist | The SPEC-DOC-001 `docs/lychee.toml` allowlist SHALL be extended with the following known-rate-limited or auth-walled provider documentation domains: `developers.naver.com`, `api.github.com`, `docs.github.com`, `reddit.com/dev`, `hn.algolia.com`, `info.arxiv.org`, `developers.google.com/youtube`, `docs.bsky.app`, `docs.searxng.org`. Broken external links to these domains SHALL warn (not fail) per SPEC-DOC-001 REQ-DOC-013 external-link policy. |
| **NFR-ADPDOC-006** | KO Tier-1 review SLO | KO Tier-1 page review turnaround (manager-docs draft → native-Korean reviewer signoff → run-phase completion) SHALL average ≤ 5 calendar days per page over the 4-page Tier-1 batch. Tracked in `docs/content/ko/CONTRIBUTING.md` reviewer log (per SPEC-DOC-001 REQ-DOC-010). |

---

## 4. Exclusions (What NOT to Build)

[HARD] 다음 항목은 본 SPEC 범위에서 명시적으로 제외된다. 각 항목은
known destination, rationale, 또는 follow-up이 기록되어 있다.

- **Adapter contributor / development guide** (how to write a new
  adapter from scratch, the `pkg/types.Adapter` 4-method contract,
  testing patterns, MX tag conventions). → 별도 SPEC-ADP-DEVGUIDE
  (post-V1). 본 SPEC은 **operator-facing** reference만; contributor
  audience는 SPEC-ADP-* + research.md + manager-strategy 호출로
  접근. 잘못된 audience 혼용은 SearXNG가 한 mistake (research §2.5
  anti-pattern).

- **Vendor API tutorials** (Naver Developer Console step-by-step
  with screenshots, GitHub PAT generation walkthrough with
  screenshots). → DOC-001 `operators/auth-setup.mdx`로 일반화된
  OAuth setup이 cover; 본 SPEC은 adapter-specific Setup 섹션에서
  3-line summary + provider URL link만. 풀 tutorial은 vendor가
  자체 docs로 보유 (lychee로 link 검증).

- **`noop` adapter reference page**. → 테스트 전용. `internal/
  adapters/noop/noop.go`는 production 빌드에 포함되지만 fanout
  dispatcher가 호출하지 않음 (testing 픽스처). docs site에 노출
  불필요.

- **per-adapter performance benchmarks (latency P50/P95/P99)**. →
  SPEC-EVAL-002 dashboard가 per-adapter latency를 surface; 본
  SPEC의 status badge는 success rate만 반영. latency 페이지는
  EVAL-002 dashboard URL로 cross-link.

- **Real-time adapter health status (live dashboard embed)**. →
  V1.0.0은 static MDX + JSON build-time import. live status는
  EVAL-002 dashboard URL을 따로 방문 (별도 surface). docs site에
  iframe embed는 V1.1 검토 (CSP + same-origin 정책 복잡도).

- **Automated adapter-status feed (live EVAL-002 cron export +
  JSON-Schema build-time validation + `adapter-status-staleness`
  GitHub-Issue automation)**. → A4. SPEC-EVAL-002 (PR #44) ships
  NO `adapter-status.json` export and NO `lifecycle` field. V1
  ships a STATIC, hand-curated `adapter-status.json` (DOC-002-
  owned). The live cron export, `adapter-status.schema.json`
  build-time validation step, and the mtime-based staleness gate
  (NFR-ADPDOC-003) are DEFERRED to a post-V1 EVAL-002 amendment.
  The EVAL-002 reliability dashboard + `/admin/health/adapters`
  endpoint are cross-linked (forward-reference), not consumed.

- **per-adapter cost/billing guidance** (예: GitHub PAT의 무료
  tier vs paid GitHub Enterprise, Naver API 무료 quota 초과 시
  과금). → vendor-side concern. 본 SPEC은 rate limit만 documented;
  cost는 vendor docs link로 redirect.

- **Adapter comparison matrix** (어떤 query 타입에 어떤 어댑터가
  최적인가). → SPEC-DOC-001 `end-users/surface-comparison.mdx`가
  surface (CLI vs UI vs Skill vs MCP). adapter-level 비교는
  product.md §7 differentiation matrix가 covers; per-adapter
  비교 매트릭스는 V1.1 검토 (UX complexity).

- **Auto-generated provider-doc embed** (Naver Developer docs HTML
  scrape + embed in `naver.mdx`). → 저작권 / scraping policy 우려.
  link-only.

- **Per-adapter telemetry deep-dive** (어떤 metric label이 어떤
  의미인지, label cardinality 분석). → SPEC-OBS-001
  `operators/observability.mdx` (SPEC-DOC-001) cover. per-adapter
  페이지는 metric 이름만 mention (e.g., `usearch_adapter_calls_
  total{adapter="naver",outcome="rate_limited"}`).

- **Korean-tokenizer setup full procedure duplication**. → D6 결정.
  Naver + koreanews 페이지는 3-line summary + cross-link only.

- **Machine-translated KO content for Tier-2 8개 어댑터** (Reddit
  HN arxiv GitHub YouTube Bluesky X SearXNG KO). → V1.1 minor
  release로 deferred. V1.0.0은 KO Tier-1 4개 (D7).

- **Bilingual coverage for `_generated/` JSON fragments**. → AST
  추출 결과는 language-agnostic; 한 source of truth만 존재
  (English schema). MDX 페이지의 prose만 bilingual.

- **GitHub Issue tracking on this SPEC** (`issue_number: 0`). →
  M9 polish SPEC 패턴 (SPEC-DOC-001 + SPEC-SEC-001과 동일).

- **Adapter retirement / deprecation flow documentation**. → V1.0.0
  ship 시점에 deprecated 어댑터 없음. retirement runbook은 첫
  실제 deprecation 발생 시 작성 (post-V1 별도 SPEC).

- **Custom badge designs / branding**. → Nextra v4 + Tailwind
  default styling 사용; brand-voice.md (SPEC-UI-001 `_TBD_`)
  ship 시점에 theme refresh로 통합.

- **Embedded interactive query playground per page** ("이 어댑터로
  검색해보기" 버튼). → SPEC-DOC-001 D5에서 정적 docs site 결정;
  interactive playground는 V2 검토.

- **Adapter SDK / wrapper library for third-party developers**. →
  adapter API는 `pkg/types.Adapter` 인터페이스로 노출됨 (Go
  packages만; non-Go 언어 SDK는 V2).

- **OpenAPI / AsyncAPI machine-readable spec** for the adapter
  contract. → post-V1. 본 SPEC은 markdown reference만.

---

## 5. Acceptance Criteria

per-REQ acceptance summary는 §2에 inline 문서화. 전체 Given-When-
Then scenarios는 `.moai/specs/SPEC-DOC-002/acceptance.md` (plan-
auditor cycle에서 작성). scenario index:

| Scenario | Description | Coverage |
|----------|-------------|----------|
| §5.1 | 10 EN adapter pages exist with filename matching `Capabilities().SourceID`; `noop.mdx` does NOT exist. | REQ-ADPDOC-001 |
| §5.2 | Each EN page contains exactly 10 H2 sections in the prescribed order; missing or out-of-order section fails `adapter-page-completeness` CI job. | REQ-ADPDOC-002 |
| §5.3 | `index.mdx` renders catalog table with 10 rows + filter UI; clicking "news" filter narrows to `koreanews` + `naver`. | REQ-ADPDOC-003 |
| §5.4 | `errors.mdx` documents 5 `CategoryX` values with all 4 required fields; lychee resolves the link from each per-adapter page. | REQ-ADPDOC-004 |
| §5.5 | `<StatusBadge adapter="bluesky">` renders correct lifecycle from JSON; boundary case (rate = 0.949) renders `beta`, (rate = 0.950) renders `stable`. | REQ-ADPDOC-005 |
| §5.6 | Static `adapter-status.json`: every key is a real SourceID; `x` carries `lifecycle: disabled`; malformed entry (missing `lifecycle`) renders fallback badge. (Live schema-validation gate DEFERRED — A4.) | REQ-ADPDOC-006 |
| §5.7 | Drift CI gate: artificially modify `_generated/reddit.capabilities.json` → CI fails; modify a real adapter `RateLimitPerMin` (incl. `blueskyCapabilities()` in `social.go`) without regen → CI fails; `hackernews`+`bluesky`+`x` JSONs emitted correctly from `hn/` and `social.go` helpers; clean state → CI passes. | REQ-ADPDOC-007 |
| §5.8 | `<CapabilitiesTable>` renders auto-extracted fields + source path footer; no per-page hardcoded values present (grep assertion). | REQ-ADPDOC-008 |
| §5.9 | `bluesky.mdx` + `x.mdx` exist as separate pages; `x.mdx` framed as DISABLED v0 stub with `disabled` badge; both reference the shared-implementation callout linking the other; `index.mdx` shows 2 social rows. | REQ-ADPDOC-009 |
| §5.10 | `github.mdx` Setup section contains all 5 auth-required fields (registration URL, env vars, scopes, verification command, DEPLOY-001 cross-link); `reddit.mdx` Setup section reads "not required — public endpoint". | REQ-ADPDOC-010 |
| §5.11 | `naver.mdx` + `koreanews.mdx` Setup sections contain the 3 Korean-locale summary points + cross-link to DOC-001 KO operator page; do NOT contain a full mecab-ko walkthrough. | REQ-ADPDOC-011 |
| §5.12 | Each page's Rate limits section has all 4 elements; provider URLs resolve via lychee; enforcement mechanism text matches research §1.4 verbatim per adapter. | REQ-ADPDOC-012 |
| §5.13 | Each page's Error reference contains adapter-specific status rosetta + shared `errors.mdx` cross-link; GitHub 422 row present; Naver 401 row with `NAVER_CLIENT_ID` operator action present. | REQ-ADPDOC-013 |
| §5.14 | Troubleshooting sections: ≥ 3 entries per page (≥ 5 for `koreanews.mdx`); each entry has all 5 fields; Related SPECs links resolve. | REQ-ADPDOC-014 |
| §5.15 | Version compatibility tables present on all 10 pages with ≥ 1 row each within 90 days of 2026-05-22; backdating one page's `lastVerified` to 200 days produces CI warn. | REQ-ADPDOC-015 |
| §5.16 | Related sections: ≥ 4 cross-links per page; lychee internal-strict resolves 100%. | REQ-ADPDOC-016 |
| §5.17 | KO Tier-1 set: 4 KO files exist with reviewer signoff log; bilingual coverage gate passes; deleting `naver.mdx` (KO) drops coverage below threshold and fails CI. | REQ-ADPDOC-017 |
| §5.18 | Credential placeholder lint: injecting a realistic-shaped GitHub PAT (matching the script's regex) into `github.mdx` fails `check-doc-credentials.sh`; injecting `<YOUR_GITHUB_TOKEN>` passes; clean baseline returns zero matches across all 12 pages. | REQ-ADPDOC-018 |
| §5.19 | Drift CI runtime ≤ 60 seconds; page-completeness CI runtime ≤ 30 seconds; combined with SPEC-DOC-001 docs.yml total ≤ 6 minutes. | NFR-ADPDOC-001, NFR-ADPDOC-002 |
| §5.20 | (DEFERRED — A4) mtime-based staleness gate is not a V1 scenario; the static `adapter-status.json` has no live freshness gate. V1 freshness asserted by per-page `lastVerified` (see §5.15). | NFR-ADPDOC-003 (deferred) |
| §5.21 | Page completeness threshold: deliberately blanking the Troubleshooting section of `reddit.mdx` (< 50 chars plain text) → CI fails; restoring → passes. | NFR-ADPDOC-004 |

---

## 6. Dependencies & Blocks

### 6.1 Upstream SPEC dependencies (depends_on)

- **SPEC-ADP-001 (Reddit, implemented, M3)** — adapter
  implementation. `internal/adapters/reddit/reddit.go:97-115`
  Capabilities source. Reference page `reddit.mdx`.

- **SPEC-ADP-002 (Hacker News, implemented, M3)** —
  `internal/adapters/hn/hn.go:99-119` Capabilities
  (`SourceID: "hackernews"` at `hn.go:101`). Reference page
  `hackernews.mdx` (slug = SourceID; pkg dir is `hn/`).

- **SPEC-ADP-003 (arxiv + Paper Search, implemented, M3)** —
  `internal/adapters/arxiv/arxiv.go:112-124` Capabilities +
  per-instance rate guard `arxiv/search.go:142-146`. Reference
  page `arxiv.mdx`.

- **SPEC-ADP-004 (GitHub, implemented, M3)** —
  `internal/adapters/github/github.go:137-160` Capabilities +
  go-github rate limit error parsing `github/client.go:77-112`.
  AUTH-required adapter (`USEARCH_GITHUB_TOKEN`). Reference page
  `github.mdx`.

- **SPEC-ADP-005 (YouTube, implemented, M3)** —
  `internal/adapters/youtube/youtube.go:94-110` Capabilities +
  ko-KR locale negotiation `youtube/lang.go`. Reference page
  `youtube.mdx`.

- **SPEC-ADP-006 (Bluesky + X, implemented, M3)** —
  `internal/adapters/social/social.go`: `Capabilities()`
  (`social.go:132`) switches on `a.subSource` and returns the
  package-level helper funcs `blueskyCapabilities()`
  (`social.go:144`, `SourceID: "bluesky"`, `RateLimitPerMin: 600`)
  and `xCapabilities()` (`social.go:164`, `SourceID: "x"`,
  `RateLimitPerMin: 0`, DISABLED v0 stub). NO separate
  `bluesky.go` / `x.go` files exist. Reference pages `bluesky.mdx`
  AND `x.mdx` (REQ-ADPDOC-009 splits these); the AST tool emits
  both JSON fragments by resolving the two helper funcs.

- **SPEC-ADP-007 (SearXNG bridge, implemented, M3)** —
  `internal/adapters/searxng/searxng.go:130-160` Capabilities;
  self-hosted, no auth, no rate limit advertised. Reference
  page `searxng.mdx`.

- **SPEC-ADP-008 (Naver Suite, implemented, M3)** —
  `internal/adapters/naver/naver.go:177-198` Capabilities;
  AUTH-required (`NAVER_CLIENT_ID` + `NAVER_CLIENT_SECRET`);
  `openapi.naver.com` SSRF allowlist `naver/client.go:22-24`;
  DataLab separate endpoint `naver/datalab.go`. Reference page
  `naver.mdx` (KO Tier-1).

- **SPEC-ADP-009 (KoreaNewsCrawler + Daum + Korean RSS,
  implemented, M3)** —
  `internal/adapters/koreanews/koreanews.go:81-100`
  Capabilities; operator-configured per-feed rate; EUC-KR
  transcoding `koreanews/locale.go`; mecab-ko-aware dedup
  `koreanews/dedup.go`; KNC sidecar bridge `koreanews/knc.go`.
  Reference page `koreanews.mdx` (KO Tier-1).

- **SPEC-IDX-001/002/003/004/005 (M3 implemented)** — indexing
  layer context for Korean tokenizer (IDX-003 mecab-ko Meili
  plugin specifically; cross-link target for Naver +
  koreanews pages). IDX-001/002/004/005 referenced as
  background context.

- **SPEC-FAN-001 (fanout dispatcher, implemented, M3)** —
  CategoryRateLimited + retry semantics cross-linked from
  every page's Rate limits + Error reference sections.

- **SPEC-CACHE-001 (5-phase access fallback, implemented,
  M3)** — fallback failure modes cross-linked from
  Troubleshooting sections (consistent with SPEC-DOC-001
  troubleshooting top-10 sourcing).

- **SPEC-EVAL-002 (adapter reliability observability, on
  unmerged PR #44, M8)** — forward-reference only. Provides
  Prometheus recording rules + Grafana `adapter-reliability`
  dashboard + `/admin/health/adapters` + `usearch_adapter_health_
  status` gauge. NO `adapter-status.json` export, NO `lifecycle`
  field (A4). DOC-002's static status feed reads the dashboard
  manually; it is NOT a hard run-phase dependency. A live export
  is a post-V1 EVAL-002 amendment.

- **SPEC-DOC-001 (User guide docs site, drafted, M9 sibling)** —
  docs site infrastructure (Nextra v4, lychee, screenshot
  freshness, bilingual coverage, gh-pages + container dual
  deploy). DOC-001 PASS is hard prerequisite for DOC-002 run
  phase. DOC-001 REQ-DOC-008 reserves the `reference/adapters/`
  IA slot that this SPEC fills. DOC-001 owner must agree to
  `check-bilingual-coverage.sh` exclude pattern extension
  (open question §8.5).

### 6.2 Related but soft (related)

- **SPEC-DEPLOY-001 (M9 sibling, not yet drafted)** — Helm
  chart. Each per-adapter Setup section's env-var notes
  cross-link to DEPLOY-001 Helm values keys. DEPLOY-001
  Helm values schema for `secrets.{github,naver,knc}` must
  be agreed before DOC-002 ship (mutual dependency at the
  M9 milestone level).

- **SPEC-EVAL-001 (citation faithfulness benchmark, drafted,
  M8)** — eval methodology. soft cross-reference only.

- **SPEC-EVAL-003 (Korean-locale benchmark, drafted, M8)** —
  Korean eval methodology. soft cross-reference from Naver
  + koreanews pages' Related section.

- **SPEC-SEC-001 (security hardening, drafted, M8)** —
  D2 gitleaks complements DOC-002 D8 placeholder lint;
  D3 SSRF mitigation is the source for Naver redirect
  allowlist documentation in `naver.mdx`.

### 6.3 Downstream blocked SPECs (blocks)

- **SPEC-REL-001 (V1.0.0 tag + release notes, M9, not yet
  drafted)** — release notes "complete adapter reference"
  claim depends on DOC-002 12 EN MDX pages + 4 KO Tier-1
  pages all passing completeness + drift CI gates. DOC-002
  PASS is hard prerequisite for REL-001 "docs site live"
  exit criterion satisfaction.

### 6.4 External dependencies (run-phase pins)

| Dependency | Pinned version | Source | License |
|------------|---------------|--------|---------|
| `go/parser` | Go 1.23+ stdlib | golang.org | BSD-3-Clause |
| `go/ast` | Go 1.23+ stdlib | golang.org | BSD-3-Clause |
| Nextra v4 MDX custom components API | v4.x (matching SPEC-DOC-001 D1 pin) | shuding/nextra | MIT |
| React (for `<StatusBadge>`, `<CapabilitiesTable>`, `<AdapterCatalog>`) | 19.x (matching SPEC-DOC-001 6.4) | Meta | MIT |
| JSON Schema validator (build-time `adapter-status.json` validation) | ajv 8.x or zod 3.x | open-source | MIT |
| MDX AST parser (for `adapter-page-completeness.sh`) | `@mdx-js/mdx` v3.x (matching Nextra v4 ecosystem) | MDX team | MIT |

신규 Go module direct deps: none (stdlib only — `go/parser`,
`go/ast`, `encoding/json`, `os/filepath` cover the AST extraction
+ JSON emit). SPEC-DEP-001 REQ-DEP-007 pin policy 자동 준수.

---

## 7. Files to Create / Modify

### 7.1 Created (estimated; final list owned by run phase)

**EN adapter reference pages (12 MDX files)**:

| Marker | Path | Purpose |
|--------|------|---------|
| [NEW] | `docs/content/en/reference/adapters/index.mdx` | catalog page replacing SPEC-DOC-001 placeholder per REQ-ADPDOC-003 |
| [NEW] | `docs/content/en/reference/adapters/reddit.mdx` | Reddit adapter (ADP-001) reference |
| [NEW] | `docs/content/en/reference/adapters/hackernews.mdx` | Hacker News adapter (ADP-002) reference — slug = SourceID `hackernews` (pkg dir is `hn/`) |
| [NEW] | `docs/content/en/reference/adapters/arxiv.mdx` | arxiv adapter (ADP-003) reference |
| [NEW] | `docs/content/en/reference/adapters/github.mdx` | GitHub adapter (ADP-004) reference, AUTH-required |
| [NEW] | `docs/content/en/reference/adapters/youtube.mdx` | YouTube adapter (ADP-005) reference |
| [NEW] | `docs/content/en/reference/adapters/bluesky.mdx` | Bluesky adapter (ADP-006) reference |
| [NEW] | `docs/content/en/reference/adapters/x.mdx` | X (Twitter) adapter (ADP-006) reference — DISABLED v0 stub, not available in V1 |
| [NEW] | `docs/content/en/reference/adapters/searxng.mdx` | SearXNG bridge adapter (ADP-007) reference |
| [NEW] | `docs/content/en/reference/adapters/naver.mdx` | Naver Suite adapter (ADP-008) reference, AUTH-required, Korean-locale |
| [NEW] | `docs/content/en/reference/adapters/koreanews.mdx` | KoreaNewsCrawler + Daum + Korean RSS adapter (ADP-009) reference, Korean-locale |
| [NEW] | `docs/content/en/reference/adapters/errors.mdx` | shared `*types.SourceError` Category reference per REQ-ADPDOC-004 |
| [NEW] | `docs/content/en/reference/adapters/_meta.json` | Nextra sidebar ordering |

**KO Tier-1 adapter reference pages (4 MDX files)**:

| Marker | Path | Purpose |
|--------|------|---------|
| [NEW] | `docs/content/ko/reference/adapters/index.mdx` | KO 카탈로그 페이지 |
| [NEW] | `docs/content/ko/reference/adapters/naver.mdx` | Naver 어댑터 KO 번역 (Tier-1) |
| [NEW] | `docs/content/ko/reference/adapters/koreanews.mdx` | koreanews 어댑터 KO 번역 (Tier-1) |
| [NEW] | `docs/content/ko/reference/adapters/errors.mdx` | 공유 Category 레퍼런스 KO 번역 |
| [NEW] | `docs/content/ko/reference/adapters/_meta.json` | KO sidebar ordering |

**Auto-generated artifacts** (committed; drift-gated):

| Marker | Path | Purpose |
|--------|------|---------|
| [NEW] | `docs/content/en/reference/adapters/_generated/reddit.capabilities.json` | drift-gated Capabilities extract |
| [NEW] | `docs/content/en/reference/adapters/_generated/hackernews.capabilities.json` | drift-gated Capabilities extract (slug = SourceID) |
| [NEW] | `docs/content/en/reference/adapters/_generated/arxiv.capabilities.json` | drift-gated Capabilities extract |
| [NEW] | `docs/content/en/reference/adapters/_generated/github.capabilities.json` | drift-gated Capabilities extract |
| [NEW] | `docs/content/en/reference/adapters/_generated/youtube.capabilities.json` | drift-gated Capabilities extract |
| [NEW] | `docs/content/en/reference/adapters/_generated/bluesky.capabilities.json` | drift-gated Capabilities extract |
| [NEW] | `docs/content/en/reference/adapters/_generated/x.capabilities.json` | drift-gated Capabilities extract |
| [NEW] | `docs/content/en/reference/adapters/_generated/searxng.capabilities.json` | drift-gated Capabilities extract |
| [NEW] | `docs/content/en/reference/adapters/_generated/naver.capabilities.json` | drift-gated Capabilities extract |
| [NEW] | `docs/content/en/reference/adapters/_generated/koreanews.capabilities.json` | drift-gated Capabilities extract |
| [NEW] | `docs/content/en/reference/adapters/_generated/adapter-status.json` | STATIC hand-curated lifecycle feed (DOC-002-owned; populated from EVAL-002 dashboard) |
| [DEFERRED] | `docs/content/en/reference/adapters/_generated/adapter-status.schema.json` | JSON Schema + build-time validation — DEFERRED to post-V1 EVAL-002 live-export amendment (A4); not a V1 artifact |

**MDX components**:

| Marker | Path | Purpose |
|--------|------|---------|
| [NEW] | `docs/components/StatusBadge.tsx` | `<StatusBadge adapter="...">` per REQ-ADPDOC-005 |
| [NEW] | `docs/components/StatusBadge.test.tsx` | unit tests covering taxonomy boundary cases |
| [NEW] | `docs/components/CapabilitiesTable.tsx` | `<CapabilitiesTable src="...">` per REQ-ADPDOC-008 |
| [NEW] | `docs/components/CapabilitiesTable.test.tsx` | unit tests |
| [NEW] | `docs/components/AdapterCatalog.tsx` | filterable catalog used by index.mdx per REQ-ADPDOC-003 |
| [NEW] | `docs/components/AdapterCatalog.test.tsx` | unit tests for filter logic |

**Go tool + shell scripts**:

| Marker | Path | Purpose |
|--------|------|---------|
| [NEW] | `tools/gen-adapter-ref/main.go` | go/parser AST extraction per REQ-ADPDOC-007 |
| [NEW] | `tools/gen-adapter-ref/extract.go` | Capabilities struct literal walker |
| [NEW] | `tools/gen-adapter-ref/extract_test.go` | 85%+ coverage on AST extraction |
| [NEW] | `tools/gen-adapter-ref/testdata/` | fixture adapter Go source files for golden tests |
| [NEW] | `scripts/gen-adapter-reference.sh` | shell wrapper invoking the Go tool |
| [NEW] | `scripts/check-adapter-page-completeness.sh` | 10-section completeness gate per REQ-ADPDOC-002 + NFR-ADPDOC-004 |
| [NEW] | `scripts/check-doc-credentials.sh` | placeholder-only policy lint per REQ-ADPDOC-018 |

### 7.2 Modified

| Path | Change |
|------|--------|
| `.github/workflows/docs.yml` | (a) add `gen-adapter-ref-drift` job, (b) add `adapter-page-completeness` job, (c) extend `bilingual-coverage` job to recognize `reference/adapters/` Tier-1 set per REQ-ADPDOC-017, (d) add `check-doc-credentials` job per REQ-ADPDOC-018. NOTE: the `adapter-status-staleness` job is DEFERRED (NFR-ADPDOC-003) along with the live EVAL-002 status export. |
| `docs/lychee.toml` | Add NFR-ADPDOC-005 provider URL allowlist entries |
| `docs/theme.config.tsx` (SPEC-DOC-001 ownership) | Register `<StatusBadge>`, `<CapabilitiesTable>`, `<AdapterCatalog>` MDX components |
| `docs/content/en/reference/adapters/index.mdx` (created by SPEC-DOC-001 as placeholder) | Replaced by REQ-ADPDOC-003 implementation |
| `docs/content/en/end-users/surface-comparison.mdx` (SPEC-DOC-001) | Add cross-links to each per-adapter reference page from the decision matrix table |
| `docs/content/en/operators/deployment-helm.mdx` (SPEC-DOC-001) | Add anchored subsections per adapter env var (`#github-pat`, `#naver-credentials`, `#knc-endpoint`) targeted by REQ-ADPDOC-010 cross-links |
| `docs/content/ko/CONTRIBUTING.md` (SPEC-DOC-001) | Add KO Tier-1 reviewer log entries for the 4 KO pages per REQ-ADPDOC-017 + NFR-ADPDOC-006 |
| `scripts/check-bilingual-coverage.sh` (SPEC-DOC-001 ownership) | Extend exclude pattern logic to recognize `reference/adapters/` Tier-1 set per REQ-ADPDOC-017 (requires DOC-001 owner sign-off, open question §8.5) |

### 7.3 Existing — Unchanged

- `internal/adapters/**/*.go` — adapter implementations.
  Source for AST extraction; not modified. (DDD PRESERVE
  invariant.)
- `pkg/types/errors.go` — Category enum source for
  `errors.mdx`; not modified.
- `.moai/specs/SPEC-ADP-*/spec.md` — adapter SPEC documents;
  cited from reference pages but not modified.
- `.moai/specs/SPEC-DOC-001/spec.md` — DOC-001 SPEC; not
  modified by this SPEC. (Coordination via `scripts/check-
  bilingual-coverage.sh` modification is handled in §7.2.)
- `internal/adapters/noop/` — test-only adapter; not
  documented (REQ-ADPDOC-001 excludes).

---

## 8. Open Questions

본 SPEC의 `_TBD_` markers + research.md §10는 canonical list. 요약:

1. **Korean-tokenizer documentation scope inside DOC-002** —
   per-adapter pages for Naver + koreanews should **cross-link
   only** to SPEC-DOC-001 `operators/korean-locale-setup.mdx`
   (D6 + REQ-ADPDOC-011 commit to this). Confirmed in HISTORY;
   plan-auditor verifies no inadvertent duplication.

2. **Status badge taxonomy** — RESOLVED (A4). SPEC-DOC-002
   DEFINES the 4-tier badge taxonomy (stable / beta / disabled /
   deprecated) and the success-rate thresholds (≥0.95 / 0.80–0.94).
   SPEC-EVAL-002 owns NO `lifecycle` field and ships no status
   export; DOC-002's taxonomy is self-contained. No EVAL-002
   schema alignment is required at V1.

3. **Bluesky vs X page split** — research §1.7 confirms separate
   pages despite shared `social` Go package; REQ-ADPDOC-009
   formalizes. Plan-auditor verifies operator-experience
   rationale with user.

4. **EVAL-002 status export** — RESOLVED (A4). EVAL-002 (PR #44)
   ships no `adapter-status.json` export. V1 ships a STATIC,
   hand-curated `adapter-status.json` (manager-docs reads the
   EVAL-002 dashboard). The live cron export + schema-validation
   + staleness automation are DEFERRED to a post-V1 EVAL-002
   amendment (tracked as follow-up; not a V1 blocker).

5. **`scripts/check-bilingual-coverage.sh` exclude pattern
   amendment** — per REQ-ADPDOC-017, the SPEC-DOC-001 owner
   must approve the exclude pattern extension. Coordination
   required before DOC-002 run-phase merge.

6. **`tools/gen-adapter-ref/` location convention** — RESOLVED.
   A `tools/` directory already exists (`tools/claude-skill`);
   `tools/gen-adapter-ref/` is a clean sibling. `scripts/gen-
   adapter-reference.sh` is the shell wrapper.

7. **Provider doc URL canonicalisation per locale** — EN
   page links EN provider doc; KO page links KO provider
   doc (Naver Developer docs have KO version; GitHub provides
   localized docs at `docs.github.com/ko`). lychee allowlist
   covers both. Plan-auditor confirms link strategy.

8. **Page completeness threshold definition** — NFR-ADPDOC-004
   proposes ≥ 50 characters of plain text per section. Final
   threshold (50 vs 100 vs 200 chars) confirmed in plan-auditor
   based on baseline page draft samples.

이 항목들은 plan-auditor PASS를 차단하지 않는다 — known
unresolved scope edges로 rationale과 함께 tagged.

---

## 9. References

External (research.md §12 cited):

- Logstash input plugin docs (template precedent): https://www.elastic.co/guide/en/logstash/current/input-plugins.html
- SearXNG engine docs (closest analogue): https://docs.searxng.org/admin/engines/
- Meilisearch language docs (Korean tokenizer cross-link pattern): https://docs.meilisearch.com/learn/indexing/discover_the_settings.html
- OpenSearch plugin docs: https://opensearch.org/docs/latest/install-and-configure/plugins/
- Airbyte source connector docs: https://docs.airbyte.com/integrations/sources/
- Naver Developers (Korean): https://developers.naver.com/docs/serviceapi/search/
- GitHub REST API rate limits: https://docs.github.com/en/rest/overview/resources-in-the-rest-api#rate-limiting
- Reddit API: https://github.com/reddit-archive/reddit/wiki/API
- arxiv API (rate guidance): https://info.arxiv.org/help/api/user-manual.html
- Hacker News Algolia API: https://hn.algolia.com/api
- YouTube Data API quota: https://developers.google.com/youtube/v3/getting-started
- Bluesky AppView (atproto): https://docs.bsky.app/
- MDX components (Nextra v4): https://nextra.site/docs/guide/custom-css
- JSON Schema: https://json-schema.org/
- ajv (JSON Schema validator): https://ajv.js.org/
- @mdx-js/mdx (MDX AST parser): https://mdxjs.com/

Internal (project files):

- `.moai/project/product.md` §3 (personas: Korean analyst →
  Tier-1 KO scope justification), §6 (success metrics)
- `.moai/project/roadmap.md:113` (SPEC-DOC-002 row); §M9
  narrative
- `.claude/rules/moai/core/moai-constitution.md` (TRUST 5 —
  docs MUST be Tested/Readable/Unified/Trackable)
- `.claude/rules/moai/design/constitution.md` §11 (Sprint
  Contract recommended for standard harness)
- `internal/adapters/registry.go:108-138` (wrappedAdapter
  observability layer)
- `internal/adapters/arxiv/arxiv.go:112-124` (Capabilities)
- `internal/adapters/arxiv/search.go:142-146` (per-instance
  rate guard)
- `internal/adapters/github/github.go:137-160` (Capabilities)
- `internal/adapters/github/client.go:77-112` (go-github rate
  limit error parsing)
- `internal/adapters/hn/hn.go:99-119` (Capabilities;
  `SourceID: "hackernews"` at `hn.go:101` — page slug
  `hackernews`, NOT `hn`)
- `internal/adapters/koreanews/koreanews.go:81-100`
  (Capabilities)
- `internal/adapters/koreanews/locale.go` (EUC-KR transcoding)
- `internal/adapters/koreanews/dedup.go` (mecab-ko-aware dedup)
- `internal/adapters/koreanews/knc.go` (KNC sidecar bridge)
- `internal/adapters/naver/naver.go:177-198` (Capabilities)
- `internal/adapters/naver/client.go:22-110` (SSRF allowlist +
  status mapping)
- `internal/adapters/naver/datalab.go` (separate DataLab
  endpoint)
- `internal/adapters/reddit/reddit.go:97-115` (Capabilities)
- `internal/adapters/searxng/searxng.go:130-160` (Capabilities)
- `internal/adapters/social/social.go:132-178` (`Capabilities()`
  switch-dispatch → `blueskyCapabilities()` `:144` +
  `xCapabilities()` `:164`; NO `bluesky.go`/`x.go`)
- `internal/adapters/social/search_bluesky.go` (Bluesky
  AppView search impl)
- `internal/adapters/social/search_x.go` (X search impl —
  DISABLED v0 stub, no live path wired)
- `internal/adapters/youtube/youtube.go:94-110` (Capabilities)
- `internal/adapters/youtube/lang.go` (ko-KR locale
  negotiation)
- `pkg/types/errors.go` (`*types.SourceError`, Category enum)
- `.moai/specs/SPEC-ADP-001/spec.md` ... `SPEC-ADP-009/spec.md`
  (per-adapter source SPEC documents)
- `.moai/specs/SPEC-DOC-001/spec.md` REQ-DOC-008
  (IA slot reservation), REQ-DOC-010 (KO Tier-1 policy),
  REQ-DOC-013 (lychee link-check policy), REQ-DOC-016
  (bilingual coverage gate)
- `.moai/specs/SPEC-EVAL-002/spec.md` (adapter reliability
  dashboard — status badge data source)
- `.moai/specs/SPEC-IDX-003/spec.md` (mecab-ko Meili plugin —
  Korean tokenizer cross-link target)
- `.moai/specs/SPEC-FAN-001/spec.md` (fanout — rate limit
  retry semantics cross-link)
- `.moai/specs/SPEC-CACHE-001/spec.md` (5-phase fallback —
  troubleshooting cross-link)
- `.moai/specs/SPEC-SEC-001/spec.md` D2 (gitleaks —
  complementary to REQ-ADPDOC-018 lint), D3 (SSRF —
  documented in `naver.mdx`)

---

*End of SPEC-DOC-002 v0.1.0 (draft).*
