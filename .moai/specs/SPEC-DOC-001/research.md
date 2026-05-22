# SPEC-DOC-001 Research — User guide docs site 사전 분석

Status: draft companion to spec.md
Author: limbowl via manager-spec
Date: 2026-05-22
Methodology: Pre-EARS research per `.claude/rules/moai/workflow/spec-workflow.md`
Plan Phase Sub-phase 1 (Research)

본 research는 SPEC-DOC-001 (M9 V1 release user guide) 작성 전 deep-
dive 분석이다. 본 SPEC은 **새로운 문서 시스템을 발명하지 않으며**, 11개
완료/예정 SPEC의 README + runbook + configuration 자산을 Nextra
사이트로 consolidate하는 DDD-style consolidation SPEC이다. 따라서
research는 (a) 현재 docs surface의 정확한 inventory, (b) Nextra v4
ecosystem 검증, (c) 대체 SSG 비교 + 거절 근거, (d) i18n + search +
deployment 도구 평가, (e) comparable OSS doc site audit, (f) WCAG
2.1 AA baseline 확인, (g) link-check + freshness CI 도구를 광범위
하게 다룬다.

목차:

1. Existing docs surface inventory — 현재 자산 정밀 측정
2. Nextra v4 — 아키텍처, i18n, search, RSC, 거절된 대안과의 비교
3. 대안 SSG (Docusaurus / VitePress / MkDocs / Astro Starlight) — 거절 근거 매트릭스
4. i18n 접근법 — file-system vs route-based vs split-repo
5. Search — Pagefind vs FlexSearch vs Algolia DocSearch
6. Accessibility — WCAG 2.1 AA + axe-core CI options
7. Link-check tooling — lychee vs markdown-link-check vs htmltest
8. Deployment — gh-pages + Docker container hybrid
9. Korean localization — 한국어 docs 작성 컨벤션 + 검토 워크플로우
10. Comparable OSS doc sites — SearXNG, Meilisearch, Qdrant, Helm
11. Open questions (canonical)
12. Risks + mitigations
13. References

---

## 1. Existing docs surface inventory

V1 사용자 문서 ship 전 현재 어떤 자산이 어디에 있는지 정밀 측정.
DDD ANALYZE 단계의 baseline.

### 1.1 Repo-root user-facing assets

`/Users/masterp/Projects/superwork/universal-search/`:

- `README.md` (87 lines, 2026-04-28 modified): repo-level quickstart.
  Sections: title + intro paragraph + Quickstart (6-step bash), Prerequisites
  table (Docker 24+ / Go 1.25+ / Python 3.11+ / Node 22+ / make / uv 0.4+ /
  pnpm 9+), Common Commands (make targets), Project Documentation links,
  Architecture (tree-style code block: cmd/, internal/, pkg/, services/,
  web/, deploy/), License (Apache-2.0 + SearXNG AGPL service-boundary note).
  **Authoritative for "first 30 seconds"**.
- `CHANGELOG.md` (39 KB, KaC v1.1.0 format, 2026-05-22): per-SPEC commit
  log from M1 (2026-04-24) through M6 AUTH rollout. Format: `## [Unreleased]
  → ### Added/Changed/Fixed → per-SPEC entries with commit SHA + scope`.
  Comprehensive SPEC reference index — docs site footer + `legal/changelog
  .mdx` source.
- `LICENSE` (11 KB): Apache-2.0 full text.
- `NOTICE` (1.6 KB): attribution notice including SearXNG AGPL boundary.
- `CLAUDE.md` (28 KB): MoAI execution directive — **internal agent-facing
  only**; not docs-site content.
- `docs/dependencies.md` + `docs/_deps-header.md` + `docs/_deps-compose-
  table.md`: auto-generated dependency manifest from `scripts/gen-deps-
  manifest.sh` per SPEC-DEP-001. `dependencies.md` is the user-facing
  artifact; the two underscore-prefixed files are generation templates.
  **Authoritative for dependency reference**. docs site MUST cross-link
  to canonical location (not re-render).
- `docs/licenses/.gitkeep`: placeholder for per-package license texts —
  populated by SPEC-DEP-001 license-scan workflow.

### 1.2 `.moai/` project-level assets

- `.moai/project/product.md` (172 lines, 2026-04-24 baseline + amendments):
  identity, problem statement, primary personas (4: Research-heavy engineer,
  Product/strategy lead, Korean analyst/journalist, Team lead), V1 scope,
  non-goals, success metrics (8 SLOs), differentiation matrix vs 6
  competitors (Perplexity, GPT Researcher, SearXNG, last30days-skill,
  Danswer/Onyx, Perplexica), upstream licenses + Apache-2.0 target,
  SearXNG AGPL caveat. **Authoritative for "what is Universal Search"**
  — docs `introduction/*.mdx` consumes this verbatim with link-back.
- `.moai/project/tech.md` (~200+ lines): architectural principles (7 axes:
  composition-over-reinvention, Go-for-plane Python-for-depth, hybrid-
  retrieval-always, shared-index-first-class, every-claim-cites-source,
  LLM-replaceable, provider-neutral-via-MCP), language/runtime matrix
  (Go 1.25+, Python 3.12+, TypeScript 5.4+, PostgreSQL 16+), V1-locked
  tech stack per layer. **Authoritative for `reference/architecture.mdx`**.
- `.moai/project/structure.md`: repo layout reference.
- `.moai/project/roadmap.md`: 9-milestone map (M1 Foundation → M9 V1
  release) + per-milestone SPEC backlog tables + parallelization plan +
  priority labels + exit criteria. **Cross-link source for milestone
  context** — not directly user-surfaced (internal-process artifact).
- `.moai/project/brand/{brand-voice,visual-identity,target-audience}.md`:
  template `_TBD_` state per SPEC-UI-001 HISTORY. V1.0.0 docs site uses
  Nextra default theme; updates when brand interview ships.
- `.moai/docs/MCP_OAUTH_SETUP.md` (M6 deliverable): operator-facing OAuth/
  OIDC setup walk-through. Mixed audience (operator + agent reference);
  V1.0.0 docs site copies content to `operators/auth-setup.mdx` with
  operator-only framing.
- `.moai/specs/SPEC-*/`: 41 SPEC directories (BOOT, OBS, LLM, DEP, IR,
  ADP×9, IDX×5, SYN×4, CACHE, CORE, FAN, DEEP×4, AUTH×3, MCP, SKILL,
  CLI×2, UI, EVAL×3, SEC, IDX-005?). **Internal SPEC docs — NOT surfaced
  on docs site** (only SPEC IDs as anchors). User-facing docs reference
  SPEC IDs ("see SPEC-AUTH-001 for OIDC details") but do not expose the
  full EARS bodies.

### 1.3 Operator-facing assets (created by other SPECs)

- `ops/security/runbook.md`, `ops/security/owasp-asvs-checklist.md`,
  `ops/security/threat-model.md` (SPEC-SEC-001 deliverables, M8 drafted):
  incident response procedures, ASVS L1 evidence, STRIDE model.
  **Migration target — cross-linked from docs site**. Canonical files
  preserved in `ops/security/`; docs `operators/security/*.mdx`
  provides MDX wrapper + locale context (KO 번역 포함).
- `ops/security/vuln-exceptions.yaml`, `ops/security/gitleaks-fp-log.md`
  (SPEC-SEC-001): operator-tracking artifacts. NOT surfaced on docs
  site (operational state, not documentation).

### 1.4 What's missing (gap inventory)

V1.0.0 ship 전 hand-write 필요 narrative content:

- `getting-started/first-query.mdx` — 모든 prerequisite 완료 후 첫
  쿼리 실행 + 기대 출력 narrative. 현재 어디에도 없음.
- `end-users/cli-tour.mdx` — `usearch query` + (CLI-002 ship 후)
  full subcommand tour. CLI-001 spec.md는 EARS requirements만 보유;
  사용자 narrative 없음.
- `end-users/web-ui-tour.mdx` — UI-001 4 routes 스크린샷 + 사용
  narrative. UI-001 implementation 완료 후 작성 필요.
- `end-users/skill-claude.mdx` — Claude Skill 설치 + 사용.
  SKILL-001 implementation 완료 후 작성.
- `end-users/mcp-integration.mdx` — Claude Code / Codex / Gemini CLI
  host config. MCP-001 implementation 완료 후 작성.
- `end-users/surface-comparison.mdx` — 4 surface 비교 + 선택 가이드.
  완전 신규.
- `operators/deployment-helm.mdx` — Helm install. DEPLOY-001 ship
  후 작성 (placeholder까지 V1.0.0 ship).
- `operators/team-rbac.mdx`, `audit-log.mdx`, `observability.mdx` —
  운영 가이드. 구현된 SPEC (AUTH-002/003, OBS-001) 기반 narrative
  작성 필요.
- `troubleshooting/*.mdx` — top-10 issues. 완전 신규 (SPEC들의
  acceptance.md 시나리오에서 failure mode 추출).
- All `legal/*.mdx` — `licenses.mdx` (dependencies.md 기반),
  `changelog.mdx` (CHANGELOG.md 링크), `security.mdx` (SEC-001
  SECURITY.md), `accessibility.mdx` (V1.0.0 audit), `performance.mdx`
  (V1.0.0 Lighthouse).
- All `content/ko/**` Tier-1 mirrors — manager-docs agent 초벌 +
  native reviewer 검수.

### 1.5 Implication for SPEC-DOC-001

본 SPEC의 작업량은 (a) Nextra 앱 부트스트랩 (작음), (b) **PRESERVE
migration** (12+ 기존 자산을 MDX wrapper로 import — 중간 분량), (c)
**hand-write 신규 콘텐츠** (≈ 25+ MDX 페이지 EN + ≈ 22+ MDX KO 번역
— 대부분 작업), (d) CI gate 스크립트 (작음). 즉 본 SPEC은 80%
콘텐츠 작성 작업이며, Nextra 인프라 작업은 20% 미만.

---

## 2. Nextra v4 — 아키텍처, i18n, search, RSC

roadmap.md:112가 "Nextra"를 명시하므로 framework choice는 사실상
locked. 본 절은 v4가 V1 요구를 충족하는지 검증 + 알려진 제약 측정.

### 2.1 v4 release context

- Release: 2026-Q1 (https://nextra.site/blog 헤드라인 "Nextra 4.0
  has been released").
- 주요 변경 (v3 → v4):
  - React 19 + Next.js 16 App Router 기반 (V1 `web/` 스택과 매칭).
  - **Pagefind 기본 search** (v3의 FlexSearch 대체). zero-config,
    static, self-hosted.
  - RSC (React Server Components) 1차 시민.
  - file-system 기반 i18n (별도 next-intl 통합 불필요).
  - `content/` 디렉토리 컨벤션 (v3의 `pages/`에서 변경 — App Router
    정합).
- 라이선스: MIT (Nextra + nextra-theme-docs 모두). 라이선스 호환성
  이슈 없음.
- 메인테이너 활성도: shuding (https://github.com/shuding/nextra)
  적극 유지보수. issue tracker 응답성 양호.

### 2.2 File-system i18n

Nextra v4 i18n 컨벤션:

```
docs/
  content/
    en/
      index.mdx
      introduction/
        index.mdx
        personas.mdx
    ko/
      index.mdx
      introduction/
        index.mdx
        personas.mdx
```

- `next.config.mjs`에 `i18n: { locales: ['en', 'ko'], defaultLocale:
  'en' }` 설정.
- URL: `/en/introduction/personas` ↔ `/ko/introduction/personas`.
- 누락된 페이지: locale switcher가 fallback 처리 (KO 페이지 없으면
  EN으로 redirect; UX 정책 결정 필요 — research §4.3 참조).
- 메타데이터 (frontmatter `title`, `description`)는 locale별 독립.
- Nextra v4 docs의 i18n 가이드 (https://nextra.site/docs/guide/i18n)에
  공식 패턴 명시.

### 2.3 Pagefind search 통합

- v4는 빌드 시 `_pagefind/` 디렉토리를 자동 생성. `next build` →
  Pagefind CLI 호출.
- 멀티 locale 처리: locale별로 별도 index. URL이 `/en/*` 인 경우 EN
  index, `/ko/*`인 경우 KO index 로드.
- UTF-8 multi-byte 지원: Korean 텍스트 인덱싱 정상. 검증 사례: Nextra
  공식 사이트의 일본어/중국어 검색 (nextra.site 한국어 UI는 없으나
  논리적으로 동일 mechanism).
- index size: per-locale 5-20 MB 일반적 (콘텐츠 크기에 비례).
  Tier-1 약 25-30 페이지 기준 EN+KO 합산 20-30 MB 예상 (NFR-DOC-002
  bound 내).
- search UI: 기본 검색바 + dropdown 결과. 커스터마이즈 가능 (theme.
  config.tsx).

### 2.4 Static export 호환성

- `next.config.mjs`에 `output: 'export'` → `out/` 디렉토리에 정적
  파일.
- 동적 routes (`getStaticPaths`) 지원; SSR/SSG는 자동 SSG로 변환.
- API routes 미사용 (static export 시 비활성). docs site는 API 불필요.
- Pagefind은 정적 자산이므로 export 호환.
- gh-pages 배포: `out/`을 `peaceiris/actions-gh-pages` 또는 native
  `actions/upload-pages-artifact@v3` + `actions/deploy-pages@v4`로
  publish.
- Caddy/nginx 정적 serve: `out/`을 컨테이너에 COPY → 정적 파일 서빙.

### 2.5 알려진 제약

- React 19 + Next.js 16 요구 → 구버전 호환 불가 (`web/` 스택과 정합
  하므로 문제 없음).
- nextra-theme-docs 커스터마이즈는 React 컴포넌트 swap pattern. 깊은
  레이아웃 변경은 fork 필요 (V1은 default theme 유지 가정).
- MDX 컴파일 시간: 50+ 페이지 기준 30-60초 예상. CI 캐시 (`pnpm
  store` + `.next/`)로 최적화.

---

## 3. 대안 SSG — 거절 근거

V1 framework는 Nextra로 locked but, plan-auditor review가 거절
근거를 묻는 경우를 대비한 비교 매트릭스.

| Framework | Stack | i18n 1st-class? | 검색 | RSC 지원 | V1 거절 근거 |
|-----------|-------|------------------|------|----------|---------------|
| **Nextra v4** | Next.js 16 + React 19 | YES (file-system) | Pagefind 기본 | YES | **선택** — roadmap.md mandate + 스택 정합 |
| Docusaurus 3.x | React + Webpack | YES (plugin) | Algolia 기본 (third-party fallback) | NO | 무거운 deps 트리, 빌드 느림, Algolia SaaS lock-in 우려, RSC 미지원 |
| VitePress 1.x | Vue + Vite | YES (config) | 기본 (MiniSearch) | N/A | Vue 생태계 — repo 스택이 React/Next, 일관성 손실 |
| MkDocs (Material) | Python + Jinja | plugin (mkdocs-static-i18n) | 기본 (lunr) | N/A | Python 툴체인 — Go + Node 메인 스택과 분리, 관리 부담 |
| Astro Starlight | Astro + MDX | YES (config) | 기본 (Pagefind) | partial | Astro 학습 곡선, MoAI 컨벤션과 ecosystem drift |
| Hand-rolled (Next.js + MDX) | Next.js + custom | manual | manual | YES | 재발명 — i18n / search / theme 모두 수작업 |
| Sphinx | Python + reStructuredText | YES (gettext) | 기본 | N/A | rST 진입 장벽, MDX와 비교해 표현력 부족 |
| GitBook | SaaS | YES | 기본 | N/A | SaaS lock-in, self-hosted ethos 위반 |
| ReadTheDocs | Sphinx-based hosting | YES | 기본 | N/A | Python + Sphinx 종속 + RTD 호스팅 의존성 |

핵심: **roadmap.md:112가 "Nextra"를 명시했고, repo 스택이 Next.js
16 + React 19**이므로 Nextra v4가 zero-cost integration. 대안은 모두
재학습 + 별도 toolchain + lock-in trade-off.

---

## 4. i18n 접근법

### 4.1 File-system parallel trees (Nextra v4 default — 선택)

```
content/
  en/
    introduction/
      personas.mdx
  ko/
    introduction/
      personas.mdx
```

- 장점: 콘텐츠 위치 명확, locale별 독립 작성, 누락 검출 용이
  (REQ-DOC-016 bilingual-coverage 스크립트가 enumerate).
- 단점: 동일 콘텐츠의 두 사본 유지 → 동기화 부담. 자동 번역 도입 시
  diff 추적 어려움.
- V1 선택: parallel trees + 90% coverage gate. translation diff
  추적은 NFR-DOC-006 commitment (Tier-1 변경 시 KO 동기 release).

### 4.2 In-file front-matter localization (rejected)

단일 MDX 파일에 `i18n: { en: ..., ko: ... }` frontmatter. 일부
프레임워크 (Hugo)에서 사용.

- 단점: 파일이 커지고 가독성 저하, diff 노이즈, locale별 reviewer
  분리 불가.
- 거절.

### 4.3 Locale fallback policy

KO 페이지가 누락된 경우 처리:

- Option A: 404 (strict) — 사용자가 누락된 KO 페이지에 접근하면 404.
  엄격하지만 잘못된 UX (KO 사용자가 EN 콘텐츠 접근 불가).
- Option B: EN으로 redirect (lenient) — 누락 시 자동 fallback. 약간
  애매하지만 모든 콘텐츠 접근 보장.
- Option C: KO landing → "이 페이지는 한국어 번역이 준비 중입니다.
  English version is available at <link>" 같은 placeholder MDX.
  명시적이고 진척도 visibility 확보.
- V1 선택: **Option C** (placeholder MDX) — REQ-DOC-016 90% coverage
  gate가 누락된 KO 페이지를 명시; CI는 placeholder MDX 존재만
  허용 (no-content empty file은 fail). Tier-2 페이지가 placeholder
  로 운영되는 동안 사용자는 EN 링크로 안내됨.

---

## 5. Search

### 5.1 Pagefind (Nextra v4 default — 선택)

- Repo: https://github.com/CloudCannon/pagefind (~2.8k stars, MIT)
- 정적 인덱스 + 클라이언트 사이드 검색.
- multi-language UTF-8 지원 (Korean 검증 사례 다수).
- 인덱스 크기: 콘텐츠 양에 비례; Nextra v4 컴파일러가 자동 호출.
- 외부 의존성 zero (Algolia 등 SaaS 불필요).
- 자체 호스팅 ethos에 부합.

### 5.2 FlexSearch (Nextra v3 default — deprecated)

- 클라이언트 사이드 검색 라이브러리, JS 인덱스 메모리 로드.
- Pagefind 대비 인덱스 크기 더 크고 검색 속도 느림.
- v4에서 deprecated.

### 5.3 Algolia DocSearch (anti-decision)

- 무료 (오픈소스 docs site에 한해) but 외부 SaaS.
- 거절 근거: (a) self-hosted ethos 위반, (b) Algolia API key 노출
  (gitleaks SPEC-SEC-001 D2 reviewer 부담), (c) air-gapped operator
  미지원, (d) Algolia 측 outage 영향 받음.
- V1 단호한 anti-decision.

### 5.4 MiniSearch / Lunr (대안)

- 일반 JS 검색 라이브러리. Pagefind 대비 통합 부담 크고 i18n 처리
  수작업.
- 거절.

---

## 6. Accessibility — WCAG 2.1 AA

### 6.1 Nextra v4 기본 theme의 a11y 상태

- nextra-theme-docs는 WCAG 2.1 AA 준수 클레임 (공식 문서; 독립
  검증은 V1.0.0 audit 시점에 수행).
- 기본 색상 contrast: 라이트 모드 + 다크 모드 둘 다 AA 4.5:1 비율
  충족 (커뮤니티 감사 보고서 다수).
- 키보드 네비게이션: full support (Tab 순서, focus indicator,
  skip-to-content link).
- 스크린 리더: ARIA 라벨 + landmark roles 적용. 다크 모드 토글 +
  locale 스위처는 `aria-label` 포함.

### 6.2 CI 자동 a11y 테스트 (deferred to V1.1)

- axe-core CLI: https://github.com/dequelabs/axe-core
- Pa11y: https://pa11y.org/
- Lighthouse CI: a11y 점수 포함; 단 정적 export에 GitHub Actions
  통합 약간 복잡.
- V1 결정: 수동 audit + V1.0.0 freeze gate 결과 기록 (REQ-DOC-017).
  자동화는 V1.1.

### 6.3 KO 접근성 고려

- 한국어 폰트 fallback: Nextra 기본 sans-serif stack에 KO web font
  추가 필요 여부 검토. 기본 system font (Pretendard, Noto Sans KR
  등 OS-기본)로 충분한지 실측.
- 한국어 텍스트 contrast: 라틴 알파벳 대비 한글 stroke가 두꺼워
  contrast 인지가 다름; AA 기준은 동일 (4.5:1) but 실제 가독성 별도
  검토.

---

## 7. Link-check tooling

### 7.1 lychee (선택)

- Repo: https://github.com/lycheeverse/lychee (~9k stars, Apache-2.0)
- Rust로 작성, 빠름 (수천 링크 < 1분).
- 내부 + 외부 링크 모두 검사.
- GitHub Action: `lycheeverse/lychee-action@v2`.
- 설정 파일 `lychee.toml`: 검사 대상 패턴, 제외 패턴, 외부 도메인
  allowlist, retry 정책, timeout.
- 외부 도메인 캐시 (`.lycheecache`): 검사 결과 재사용 (NFR-DOC-005).

### 7.2 markdown-link-check (대안)

- Repo: https://github.com/tcort/markdown-link-check
- Node 기반, 느림 (lychee 대비 5-10x).
- 외부 도메인 retry 정책 약함.
- 거절.

### 7.3 htmltest

- Repo: https://github.com/wjdp/htmltest
- 빌드된 HTML 검사 (정적 export 후 실행).
- lychee와 보완 가능하지만 V1 단일 도구로 충분 (lychee가 MDX 직접
  검사 가능).
- 거절.

### 7.4 lychee 정책 결정

- 내부 링크: 100% 해결 가능해야 함. 실패 = CI fail (REQ-DOC-013).
- 외부 링크: warn-only (third-party flakiness 회피). retry 3회 +
  allowlist (github.com API, anthropic.com docs, naver developer
  docs, x.com).
- weekly cron으로 cache 무효화 + 전체 외부 링크 재검증.

---

## 8. Deployment — gh-pages + Docker container hybrid

### 8.1 gh-pages 배포

- `actions/upload-pages-artifact@v3` + `actions/deploy-pages@v4`
  (modern, native GitHub Actions API).
- 공개 canonical URL: `https://<org>.github.io/universal-search/`.
- GitHub Pages SLA: 99% monthly uptime (NFR-DOC-007).
- 빌드: `pnpm --dir docs build` → `docs/out/` 정적 자산.
- 한계: GitHub 종속 (org repo가 GitHub에 있어야 함; 현재 가정 충족).

### 8.2 Docker container 배포 (air-gapped operator)

- `Dockerfile.docs` multi-stage:
  - Stage 1: Node 22 base, `pnpm install`, `pnpm build`.
  - Stage 2: Caddy v2.8 (Apache-2.0, ~40 MB Alpine 기반), `COPY
    --from=builder /app/out /srv`.
- 최종 image: ~80-100 MB 압축 (NFR-DOC-004 bound 내).
- 배포: `docker run -p 8080:80 ghcr.io/<org>/usearch-docs:latest`.
- Helm chart (SPEC-DEPLOY-001): optional subchart로 docs container
  포함 → operator가 binary + docs 동시 deploy.

### 8.3 대안 host (anti-decisions)

- Vercel / Netlify: 빠르고 편하지만 SaaS lock-in. 본 제품의 self-
  hosted ethos 위반.
- ReadTheDocs: Python/Sphinx 종속. Nextra와 incompatible.
- AWS S3 + CloudFront: 가능하지만 AWS 종속 + 운영 부담.

---

## 9. Korean localization — 작성 컨벤션 + 검토 워크플로우

### 9.1 KO 콘텐츠 작성 원칙

- 어조: 기술적-정중 (operator 대상). 존댓말 vs 평어 선택:
  존댓말 (~합니다) — 매뉴얼 standard.
- 영어 기술 용어 처리:
  - 1차 발생 시: "OIDC 인증 (OpenID Connect)"처럼 한글 풀이 + 영문
    표기 병기.
  - 2차 이후: 영문 약어 (OIDC) 유지.
- 명사 vs 동사 변환: "Configure the auth provider" → "인증 제공자를
  설정합니다" (능동) 또는 "인증 제공자 설정" (명사구) — 문서 컨텍스트
  별로 일관.
- 코드 블록 + CLI 명령: 영문 원본 유지. 주석 (`# this does X`)은
  KO 번역.

### 9.2 한국어 문서 reviewer 풀

- V1 reviewer: 최소 1명 confirmed (Open Question §11.2).
- 검토 기준: (a) 기술 정확성, (b) 자연스러운 한국어 표현, (c) 영문
  원본 의미 보존.
- 검토 기록: `docs/content/ko/CONTRIBUTING.md`에 page batch별
  reviewer 이름 + 검토일 기록.

### 9.3 한국어-특이 콘텐츠

- `content/ko/operators/korean-locale-setup.mdx`: **KO-authoritative**.
  mecab-ko 설치 (Ubuntu apt, macOS brew), Naver Developer Center API
  key 발급 절차 (한국어 UI), Korean RSS feed 추천 목록. EN 카운터파트
  존재할 수 있으나 KO가 source of truth (D3 결정).
- 일자 형식: ISO 8601 (`2026-05-22`) 유지 (KO 콘텐츠도 동일). 한국어
  날짜 표기 (`2026년 5월 22일`)는 사용자 대화 surface에만 사용.
- 가격/단위: USD (LLM 비용) 유지 + KRW 환산 병기 가능 (선택).

### 9.4 자동 번역 (rejected for V1)

- DeepL / Claude / GPT-4 기반 자동 번역 도구 평가.
- 문제: 기술 용어 (RBAC, OIDC, mecab-ko, RRF fusion) 비일관 번역;
  KO operator 페르소나에 대한 trust 손실.
- V1 거절. 수동 번역 + native reviewer 검수만.

---

## 10. Comparable OSS doc sites — audit

V1 docs site 설계의 reference point.

| Project | Framework | i18n | Search | 배포 | 학습 포인트 |
|---------|-----------|------|--------|------|-------------|
| **SearXNG** (https://docs.searxng.org/) | Sphinx | YES (gettext) | 기본 | RTD | self-hosted ethos 명시. 거대 contributor pool, 한국어 docs 부재 |
| **Meilisearch** (https://www.meilisearch.com/docs/) | Custom Next.js | NO (EN-only) | Meilisearch self-dogfood | 자체 호스팅 | 자사 검색엔진 self-dogfood = "eating our own dogfood" 사례. 기술 reference + tutorial 분리 |
| **Qdrant** (https://qdrant.tech/documentation/) | Hugo | NO | Algolia | 자체 호스팅 | Hugo 가볍지만 i18n 약함. Algolia 채택 — V1 거절 사례와 대비 |
| **Helm** (https://helm.sh/docs/) | Docsy (Hugo theme) | YES (8 locales 포함 KO) | Google search box | Netlify | KO 번역 사례 — V1 KO 작업 참고. 번역 진척도 시각화 미흡 |
| **Kubernetes** (https://kubernetes.io/docs/) | Hugo + Docsy | YES (14 locales 포함 KO) | Google CSE | 자체 호스팅 | 대규모 i18n 관리 모범 사례. 번역 backlog 명시적 추적 |
| **Caddy** (https://caddyserver.com/docs/) | 자체 정적 사이트 | NO | 자체 | 자체 호스팅 | 단일 페이지 long-form docs 패턴 — V1 reference에 일부 적용 가능 |
| **Pagefind** (https://pagefind.app/) | Eleventy | NO | self-dogfood | gh-pages | 작은 docs site의 minimal 모범 사례 |
| **Astro Starlight** (https://starlight.astro.build/) | Astro Starlight | YES | Pagefind | Vercel | Pagefind 통합 사례 — V1과 동일 search 선택 |
| **shadcn/ui** (https://ui.shadcn.com/docs) | Next.js + 자체 | NO | Algolia | Vercel | shadcn/ui-style component 카탈로그. V1 reference 페이지 일부 적용 가능 |

핵심 학습:

- **Kubernetes / Helm 모델**: 다중 locale 운영. KO 번역 진척도 명시.
  V1 SPEC-DOC-001은 90% 커버리지 게이트로 진척도 자동 검증.
- **SearXNG 모델**: 거대 contributor pool + self-hosted ethos. V1
  초기에는 manager-docs agent 주도 + native reviewer 1명; V1.1
  이후 외부 기여자 contribution 활성화 고려.
- **Astro Starlight + Pagefind**: V1 결정과 동일한 search 선택 +
  Pagefind 통합 패턴 참조 가능.
- **Algolia 채택 사례 (Qdrant, shadcn/ui)**: 외부 SaaS 채택의 trade-
  off — V1은 self-hosted 우선이므로 거절.

---

## 11. Open questions (canonical)

본 절은 spec.md §8 Open Questions의 상세 버전. annotation cycle에서
하나씩 해결.

### 11.1 Brand identity timing

- `.moai/project/brand/visual-identity.md` 현재 `_TBD_`. logo + color
  palette + primary font 미확정.
- V1.0.0 docs site 시점에 brand이 ship되어 있을 가능성: 낮음
  (SPEC-UI-001 HISTORY가 V1에서 placeholder 사용 명시).
- 결정 필요: (a) docs site도 placeholder logo + Nextra default theme
  로 V1.0.0 ship → V1.0.x patch로 brand 적용; (b) brand ship 시점까지
  docs site 출시 보류 (X — SPEC-REL-001 blocking).
- 추천: (a) — V1.0.0 ship 시점에 placeholder, brand가 ship되면 V1.0.x
  patch로 즉시 적용.

### 11.2 KO reviewer pool composition

- REQ-DOC-010 요구: 최소 1명 native-Korean-speaking reviewer.
- 결정 필요: reviewer 식별 (자체 팀 vs 외부 contractor), 시간
  commitment 모델 (page batch별 vs monthly retainer), 검토 단가/
  보상 모델.
- run-phase 진입 전 user 결정 필요.

### 11.3 gh-pages canonical URL

- `<org>` placeholder. README §1에 "rename will happen at repository
  creation time" — SPEC-BOOT-001 Open Question §3 미해결.
- SPEC-REL-001과 동시 결정. V1.0.0 ship 전 확정 필요.

### 11.4 Container registry choice

- `ghcr.io/<org>/usearch-docs:<tag>` 가정. GitHub Container Registry
  사용.
- 대안: Docker Hub (대중적, 일부 organization rate-limit), Quay.io
  (RedHat, 일부 enterprise 사용), GitLab Container Registry (대안
  organization이 GitLab인 경우).
- 추천: ghcr.io (GitHub 통합 자연스러움 + usearch repo와 동일 host).
  organization 결정 시 재확인.

### 11.5 MCP tool catalog source

- SPEC-MCP-001이 OpenAPI/JSON schema를 ship할 경우 `reference/mcp/`
  를 그것에서 자동 생성 가능 (CLI reference 패턴과 유사).
- ship 여부는 MCP-001 run-phase 결정 사항.
- 추천: V1.0.0은 hand-curated MCP tool 목록. MCP-001이 OpenAPI ship
  시 V1.1에서 자동 생성으로 전환.

### 11.6 Auto-generated screenshot policy

- REQ-DOC-014는 수동 스크린샷 + freshness gate.
- 대안: Playwright 기반 auto-capture (SPEC-EVAL-002 dashboard infra
  재사용 가능).
- V1 결정: 수동. V1.1 검토.

### 11.7 Search index size on full bilingual site

- NFR-DOC-002: Pagefind index ≤ 20 MB per locale.
- 실제 측정 필요 (V1 콘텐츠 분량 기반).
- 초과 시: (a) section 단위 multi-index split, (b) low-priority
  page exclusion 옵션. plan-auditor + run-phase 실측 결정.

### 11.8 Lychee external-link allowlist baseline

- REQ-DOC-013은 known-rate-limited domains allowlist.
- baseline 후보: github.com API, anthropic.com docs, x.com, naver.com
  developer center.
- 초기 baseline 작성 시점에 user confirmation 필요.

### 11.9 V1.0.0 docs 컨테이너 이미지 base

- `Dockerfile.docs` Stage 2: Caddy vs nginx vs static (busybox httpd).
- 선택: Caddy v2.8 Alpine — Apache-2.0, 작고 안전 (auto-HTTPS는 사용
  안 함), 정적 자산 서빙에 충분.
- 대안: nginx Alpine (널리 사용, 익숙); Caddy 대비 별 차이 없음.
- 추천: Caddy (BSD 라이선스 친화, modern).

### 11.10 Auto-translated drafts as starting point?

- DeepL/Claude/GPT-4로 KO 초벌 → 인간 reviewer 검수 워크플로우 도입
  여부.
- 효율 vs 품질 trade-off. native reviewer가 자동 번역 검수보다 수동
  작성을 선호할 수도 있음.
- 추천: 인간 manager-docs agent가 초벌 (자동 번역 미사용), native
  reviewer 검수. 검토 결과 효율 부족이면 V1.1에서 재평가.

---

## 12. Risks + mitigations

### 12.1 Risk: KO 번역 품질 → operator trust 손실

- Likelihood: Medium. KO 사용자는 product.md §3 primary persona이며,
  품질 낮은 KO 콘텐츠는 Korean analyst의 trust 즉시 손상.
- Impact: High. V1 Korean-first 정체성 (roadmap M3 exit criterion)이
  docs에서 무너지면 마케팅 모순.
- Mitigation: REQ-DOC-010 native reviewer 의무 + reviewer log 추적.
  reviewer 의견과 다른 결정은 manager-docs + reviewer joint
  resolution.

### 12.2 Risk: 스크린샷 stale → 사용자 confusion

- Likelihood: High. UI / CLI는 V1 cycle 동안 빠르게 진화 가능.
- Impact: Medium. 실제 UI와 다른 스크린샷은 사용자가 잘못된 step을
  따라가게 함.
- Mitigation: REQ-DOC-014 90일 freshness gate + auto Issue creation.
  V1.1에서 Playwright auto-capture 검토.

### 12.3 Risk: Nextra v4 breaking change → 빌드 깨짐

- Likelihood: Low-Medium. v4는 신규 메이저; minor release에서 API
  변경 가능성 존재.
- Impact: High. 빌드 깨지면 CI 전체 차단.
- Mitigation: `docs/package.json`에서 nextra + nextra-theme-docs
  exact version pin (per SPEC-DEP-001 REQ-DEP-007 pin policy).
  Renovate weekly bump + 변경 사항 검토.

### 12.4 Risk: 외부 링크 third-party rot → CI 노이즈

- Likelihood: High. SearXNG / Naver / arXiv 등 외부 docs 링크가
  깨질 가능성 상존.
- Impact: Low (warn-only) but 무시 시 점진적 quality 손실.
- Mitigation: REQ-DOC-013 외부 warn-only + weekly cron + allowlist.
  분기별 link rot triage 운영.

### 12.5 Risk: gh-pages outage → docs unreachable

- Likelihood: Low (99% SLA).
- Impact: Medium (public docs 일시 미접근, but air-gapped operator는
  container image로 대응 가능).
- Mitigation: Docker container 이중 배포 (REQ-DOC-015). NFR-DOC-007
  명시.

### 12.6 Risk: bilingual coverage gate 90% → 누락 페이지 발견 시 PR
차단 → 발판 속도 저하

- Likelihood: Medium. Tier-1 페이지 추가 시 KO 동기 추가가 필수.
- Impact: Low-Medium. PR이 KO 작업까지 기다려야 merge 가능.
- Mitigation: NFR-DOC-006 — patch release는 EN-only 허용, minor
  release에서 close. 90% threshold가 부담이면 SPEC amendment로
  threshold 조정 가능 (단, D3 결정에 따라 임의 lowering 금지).

### 12.7 Risk: Pagefind index size 폭증

- Likelihood: Low at V1.0.0 (Tier-1 콘텐츠 한정). V1.1 이후 reference
  + adapters 전체 인덱싱 시 증가.
- Impact: Low (사용자 first-load 시간 증가).
- Mitigation: NFR-DOC-002 bound + section 단위 분할 옵션 (Open
  Question §11.7).

### 12.8 Risk: 보안 incident 시 docs에 sensitive info 노출

- Likelihood: Low. ops/security/runbook.md의 incident response 절차에
  secret rotation step 포함.
- Impact: Critical.
- Mitigation: SPEC-SEC-001 D2 gitleaks pre-commit + CI gate가 docs
  PR에도 적용. `.gitleaks.toml`에 docs path 포함.

---

## 13. References

External:

- Nextra 공식: https://nextra.site/
- Nextra v4 docs: https://nextra.site/docs
- Nextra i18n guide: https://nextra.site/docs/guide/i18n
- Nextra GitHub: https://github.com/shuding/nextra
- Pagefind: https://pagefind.app/
- Next.js 16 docs: https://nextjs.org/docs
- React 19 release: https://react.dev/blog/
- MDX: https://mdxjs.com/
- WCAG 2.1: https://www.w3.org/TR/WCAG21/
- axe-core: https://github.com/dequelabs/axe-core
- Pa11y: https://pa11y.org/
- lychee: https://github.com/lycheeverse/lychee
- lychee-action: https://github.com/lycheeverse/lychee-action
- Caddy server: https://caddyserver.com/docs/
- GitHub Pages deploy: https://github.com/actions/deploy-pages
- GitHub Container Registry: https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry
- Keep a Changelog v1.1.0: https://keepachangelog.com/en/1.1.0/
- Semantic Versioning 2.0.0: https://semver.org/spec/v2.0.0.html
- Docusaurus (rejected alt): https://docusaurus.io/
- VitePress (rejected alt): https://vitepress.dev/
- MkDocs Material (rejected alt): https://squidfunk.github.io/mkdocs-material/
- Astro Starlight (rejected alt): https://starlight.astro.build/
- Helm docs (KO 참고): https://helm.sh/docs/
- Kubernetes docs (KO 참고): https://kubernetes.io/docs/
- SearXNG docs (audit): https://docs.searxng.org/
- Meilisearch docs (audit): https://www.meilisearch.com/docs/
- Qdrant docs (audit): https://qdrant.tech/documentation/

Internal (project files):

- `README.md` (87 lines — quickstart source)
- `CHANGELOG.md` (39 KB — changelog source)
- `.moai/project/product.md` (172 lines — introduction source)
- `.moai/project/tech.md` (~200+ lines — architecture reference)
- `.moai/project/structure.md` (repo layout)
- `.moai/project/roadmap.md` §M9 SPEC-DOC-001 row + §5 M9 exit
  criterion "docs site live"
- `docs/dependencies.md`, `docs/_deps-header.md`, `docs/_deps-compose-
  table.md` (existing dependency manifest — reference cross-link
  target)
- `.moai/docs/MCP_OAUTH_SETUP.md` (M6 OAuth/OIDC guide — migration
  source)
- `ops/security/runbook.md`, `ops/security/owasp-asvs-checklist.md`,
  `ops/security/threat-model.md` (SEC-001 deliverables — operators/
  security/* cross-link target)
- `web/package.json` (Next.js 16 + React 19 + TS 5 pinning reference)
- `.claude/rules/moai/core/moai-constitution.md` (TRUST 5 framework)
- `.claude/rules/moai/design/constitution.md` §11 (Sprint Contract
  recommendation policy)
- `.claude/rules/moai/development/coding-standards.md` (English-only
  instruction documents rule)
- `.moai/specs/SPEC-CLI-001/spec.md`, `SPEC-CLI-002/spec.md`
- `.moai/specs/SPEC-UI-001/spec.md`
- `.moai/specs/SPEC-MCP-001/spec.md`, `SPEC-SKILL-001/spec.md`
- `.moai/specs/SPEC-AUTH-001/spec.md`, `SPEC-AUTH-002/spec.md`,
  `SPEC-AUTH-003/spec.md`
- `.moai/specs/SPEC-OBS-001/spec.md`
- `.moai/specs/SPEC-SEC-001/spec.md` (M8 sibling)
- `.moai/specs/SPEC-EVAL-001/spec.md`, `SPEC-EVAL-002/spec.md`,
  `SPEC-EVAL-003/spec.md` (M8 sibling, optional surface)
- `.moai/specs/SPEC-DEP-001/spec.md` (REQ-DEP-007 pin policy)

---

*End of SPEC-DOC-001 research v0.1.0 (draft).*
