# SPEC-UI-001 — Implementation Plan

Phased implementation plan for SPEC-UI-001 (Web UI v1 — query input,
streaming synthesis, citation rendering). This is a planning document;
no implementation. Phases are sequenced by dependency, not by time
estimate (per project policy: no time estimates).

---

## 1. Overview

| Property | Value |
|----------|-------|
| Methodology | TDD (RED-GREEN-REFACTOR) per `quality.development_mode` |
| Coverage target | 85% |
| Owner | expert-frontend |
| Primary write paths | `web/src/app/**`, `web/src/components/**`, `web/src/lib/**`, `web/e2e/**`, `web/package.json` (deps only), `web/tailwind.config.ts` (font stack only) |
| Go / Python files modified | NONE — SPEC-UI-001 is frontend-only |
| Upstream prerequisites at Run-time | SPEC-SYN-004 (implemented), SPEC-IR-001 (runtime — provides `cmd/usearch-api` synthesis endpoint URL; mocked until IR-001 ships) |

---

## 2. Pre-conditions (verified before Run starts)

- [ ] `web/` scaffold from SPEC-BOOT-001 is committed and CI green
  (BOOT-001 status: implemented).
- [ ] `cmd/usearch-api` synthesis endpoint reachable at
  `NEXT_PUBLIC_USEARCH_API_BASE` (V1 default
  `http://localhost:8080`). If SPEC-IR-001 has not landed at Run
  start, the implementer stands up a fixture server matching
  SPEC-SYN-004's SSE wire format and the SPEC-SYN-001 JSON fallback
  for local development + Playwright e2e tests.
- [ ] Annotation cycle resolved the 9 Open Questions in `research.md`
  §13 (or at minimum the V1 defaults documented there).
- [ ] LSP baseline captured per `.moai/config/sections/quality.yaml`
  (TypeScript LSP for `web/`).
- [ ] `pnpm install` succeeds inside `web/`.

---

## 3. Phased Milestones

Phases ordered by dependency. Each phase concludes with: tests
passing, type-check clean, lint clean, manual visual check on dev
server, and a PR-suitable commit.

### Phase 1 — Skeleton + chrome (Priority High)

Goal: app shell rendering on all four routes; no business logic
yet. Establishes the layout chrome, theme, dictionary, and shared
components.

Scope:
- Rewrite `web/src/app/layout.tsx` with header / footer / theme
  provider; preserve `<html lang>` and `<body>` baseline.
- Add `web/src/app/page.tsx` placeholder (query input shell, no
  submission wiring).
- Add `web/src/app/source/[doc_id]/page.tsx` placeholder
  (renders "doc_id: ..." plus Open source CTA shell).
- Add `web/src/app/history/page.tsx` placeholder (empty state only).
- Add `web/src/app/q/[request_id]/page.tsx` as a thin alias that
  redirects to `/` with request context as a URL param (V1 alias —
  per REQ-UI-001).
- Install shadcn/ui components: `Button`, `Input`, `Textarea`,
  `Skeleton`, `Dialog`, `Sheet`, `Collapsible`, `HoverCard`,
  `Popover`, `Toast`. Verify each renders in isolation via a Storybook-
  style scratch route or `pnpm dev` manual check.
- Add `web/src/lib/strings.ts` dictionary skeleton (English + Korean
  shells for ~10 chrome strings).
- Extend `web/tailwind.config.ts` `theme.extend.fontFamily.sans` with
  Korean fallback `["Inter", "Noto Sans KR", "Apple SD Gothic Neo", "sans-serif"]`.
- Add `web/src/app/_config/env.ts` exporting `USEARCH_API_BASE`.
- Add `web/.env.example` with `NEXT_PUBLIC_USEARCH_API_BASE` and
  `NEXT_PUBLIC_USEARCH_SSE_ENABLED` documented.
- Add Vitest + React Testing Library + Playwright dev deps to
  `web/package.json`; add `vitest.config.ts` and `playwright.config.ts`.

Exit gate:
- All four routes render valid HTML, return 200 in dev.
- Lighthouse on `/` shows no a11y critical errors on the empty shell.
- `pnpm typecheck`, `pnpm lint`, `pnpm build` all green.
- Smoke test: `pnpm dev` + manual visit to each route.

Test slices (RED-phase):
- `test_route_home_renders` (REQ-UI-001 partial).
- `test_route_source_detail_renders` (REQ-UI-001 partial).
- `test_route_history_renders` (REQ-UI-001 partial).
- `test_route_q_alias_resolves` (REQ-UI-001 partial).
- `test_korean_font_in_tailwind_config` (NFR-UI-001 partial).

### Phase 2 — Query input + JSON path (Priority High)

Goal: query submission works end-to-end against the buffered JSON
path (REQ-UI-007). Streams come in Phase 3. This phase de-risks the
HTTP plumbing without depending on SSE.

Scope:
- Build `web/src/components/query-input/` — Textarea + advanced-
  filters disclosure + submit + cancel + character counter +
  validation logic.
- Build `web/src/lib/citation-resolver.ts` (pure function; no React).
- Build `web/src/lib/language-detect.ts` (pure function).
- Build `web/src/components/results-static/` — renders the buffered
  JSON response as one block with citations substituted (proves the
  citation pipeline works before SSE complicates things).
- Wire `/` route: on submit, POST/GET query to API with
  `Accept: application/json`, render results-static.
- Build `web/src/components/empty-state/`, `error-state/`,
  `loading-skeleton/`.
- Build `web/src/components/citation-badge/` — clickable badge with
  shadcn `HoverCard`, `aria-label`, "Open source" link to `url` in
  new tab.

Exit gate:
- Submit a query to the JSON endpoint → see paragraph with citation
  badges in DOM.
- Click a citation → HoverCard opens with title + Open source link.
- Click Open source → new tab opens to `url`.
- Empty / error states render.
- Korean query renders Korean text correctly in Hangul font.

Test slices (RED-phase):
- `test_query_input_rejects_empty` (REQ-UI-002 partial).
- `test_query_input_rejects_over_1024_chars` (REQ-UI-002).
- `test_query_input_accepts_korean_text` (REQ-UI-002, REQ-UI-008).
- `test_advanced_filters_toggle_visible` (REQ-UI-002).
- `test_citation_resolver_substitutes_markers` (REQ-UI-004 partial).
- `test_language_detect_korean_ratio` (REQ-UI-008 partial).
- `test_citation_badge_aria_label` (REQ-UI-004 partial).
- `test_citation_badge_opens_url_in_new_tab` (REQ-UI-004 partial).
- `test_empty_state_renders_with_zero_results` (REQ-UI-003 partial).
- `test_error_state_renders_with_message` (REQ-UI-006 partial).

### Phase 3 — Streaming (SSE) + happy path (Priority High)

Goal: replace the JSON path with the SSE streaming path for the
default config. Sentence-by-sentence reveal, citation invariant
preserved, done summary on completion.

Scope:
- Build `web/src/lib/sse-client.ts` — thin native `EventSource`
  wrapper. Handles `event: sentence` / `event: done` / `event: error`
  callbacks. Surfaces connection-lost after one auto-reconnect
  attempt.
- Build `web/src/components/results-stream/` — per-sentence keyed
  `<span>` rendering, append-only state via `useReducer`, citation
  badges substituted inline, `aria-live="polite"` live region,
  done-summary banner, skeleton reserved space before first sentence.
- Wire `/` route: on submit, if `NEXT_PUBLIC_USEARCH_SSE_ENABLED ===
  "true"` (or undefined defaulting to true) AND `typeof EventSource
  !== "undefined"`, use SSE path; else fall back to results-static.
- Implement REQ-UI-007 fallback decision logic.
- Implement REQ-UI-008 Korean-first display: wrap answer region in
  `<div lang={detectedLang}>`.
- Implement REQ-UI-010 defensive unresolved-marker handling in
  citation-resolver: literal text + console.warn.

Exit gate:
- Submit query → skeleton → sentences arrive one by one → done
  summary → history entry appended.
- Klima E2E: Playwright test (i) "submit query → watch streamed
  sentences → click citation" passes against fixture server.
- Cancel button stops stream within 200 ms.
- Connection lost → "Retry" affordance appears.
- Korean query → `lang="ko"` set on answer region.
- Property test NFR-UI-004 passes on 50 generated event sequences.

Test slices (RED-phase):
- `test_sse_connection_opens_with_accept_header` (REQ-UI-003).
- `test_skeleton_visible_pre_first_sentence` (REQ-UI-003, NFR-UI-002).
- `test_sentences_render_in_order_by_sentence_index` (REQ-UI-003).
- `test_done_summary_visible_post_stream` (REQ-UI-003).
- `test_aria_live_region_announces_new_sentence` (REQ-UI-003, NFR-UI-001).
- `test_history_entry_appended_post_done` (REQ-UI-003, REQ-UI-009).
- `test_korean_query_sets_lang_attr` (REQ-UI-008).
- `test_unresolved_marker_renders_literal_text` (REQ-UI-010).
- `test_unresolved_marker_warns_console` (REQ-UI-010).
- `test_property_wire_conformance` (NFR-UI-004).
- Playwright e2e: `submit → stream → click citation → new tab`.

### Phase 4 — Citation UX polish (Priority Medium)

Goal: complete the citation interaction polish — popover on touch,
keyboard nav, source detail page, "All sources" panel.

Scope:
- Verify citation HoverCard opens on touch tap (mobile, Playwright
  mobile viewport test).
- Build `web/src/components/source-card/` (citation-shape fields
  only per REQ-UI-005).
- Build `web/src/components/all-sources-panel/` — deduplicated
  citation list, first-occurrence order, used in the results view
  right-rail (`md:` breakpoint and up).
- Wire `/source/[doc_id]` route to resolve `doc_id` against the
  session's last-stream citation cache (React Context).
- Empty state for unknown `doc_id` per REQ-UI-005.
- Implement keyboard nav: Tab order through citations, ESC closes
  popover, focus-visible styling.
- Implement responsive: bottom Sheet for citation popover on mobile
  (`< 640 px`) replacing HoverCard.
- Implement stream error component (REQ-UI-006): error message,
  Retry button.

Exit gate:
- All keyboard interactions reachable (manual + axe-core check).
- All-sources panel renders correctly at `≥ 768 px`.
- Source detail page renders for known + unknown `doc_id`.
- Mobile viewport (375 px Playwright test) shows bottom Sheet for
  citation popover.
- Retry button creates a fresh stream.

Test slices (RED-phase):
- `test_hovercard_opens_on_tap_touch` (REQ-UI-004, NFR-UI-003).
- `test_source_page_renders_with_known_doc_id` (REQ-UI-005).
- `test_source_page_shows_empty_state_for_unknown_doc_id` (REQ-UI-005).
- `test_all_sources_panel_dedup_by_doc_id` (NFR-UI-004 partial).
- `test_keyboard_nav_reaches_every_citation` (NFR-UI-001).
- `test_esc_closes_popover` (NFR-UI-001).
- `test_mobile_viewport_uses_sheet_not_hovercard` (NFR-UI-003).
- `test_error_event_shows_error_component` (REQ-UI-006).
- `test_retry_button_creates_fresh_stream` (REQ-UI-006).
- `test_previously_rendered_sentences_preserved_on_error` (REQ-UI-006).

### Phase 5 — History (Priority Medium)

Goal: local-session history page with re-run + clear, capped at 50
entries.

Scope:
- Build `web/src/lib/history-store.ts` — localStorage wrapper, cap
  50, FIFO eviction.
- Wire history-store: `append` on `event: done` (Phase 3 already
  scaffolds this call site).
- Build `/history` page: list rendering, per-entry re-run link,
  clear-history button with `Dialog` confirm.
- Empty state when store is empty.
- Localized timestamp formatting via `Intl.DateTimeFormat`.

Exit gate:
- Submit query → see entry in `/history`.
- Re-run from history → navigate to `/` with query pre-filled.
- Clear-history with confirm → empty state.
- 51st append → 50 entries, oldest evicted.

Test slices (RED-phase):
- `test_history_renders_with_zero_entries_empty_state` (REQ-UI-009).
- `test_history_capped_at_50_fifo_eviction` (REQ-UI-009).
- `test_history_rerun_navigates_to_home_with_prefilled_query` (REQ-UI-009).
- `test_history_clear_after_confirm_empties_store` (REQ-UI-009).
- Playwright e2e: `submit Korean query → see Korean entry in history`.

### Phase 6 — A11y + responsive verification + final polish (Priority Medium)

Goal: WCAG 2.1 AA verification per NFR-UI-001; manual responsive
test across breakpoints; perf advisory check per NFR-UI-002; bundle
size advisory check.

Scope:
- Run Lighthouse on each of `/`, `/source/[doc_id]`, `/history`
  (with empty + populated states); record scores in
  `acceptance.md` §4.1.
- Run axe-core dev-tools on each route; record violations.
- Manual visual + keyboard test at 375, 640, 768, 1024, 1280 px
  viewport widths.
- Verify touch targets ≥ 44×44 px on mobile.
- Run Next.js bundle analyzer; document JS shipped per route.
- Verify CLS < 0.1 during streaming using `PerformanceObserver`.
- Verify FCP / LCP targets (NFR-UI-002).
- Manual visual test on a Korean query: font rendering, no
  mid-character truncation.

Exit gate:
- Lighthouse a11y ≥ 95 on all four routes.
- 0 axe-core critical violations.
- All breakpoints render correctly (no horizontal scroll on mobile,
  no excessive line length on desktop).
- Bundle size documented; under advisory budget of 200 KB gzipped
  for `/` route.
- All NFR-UI-001/002/003/004 acceptance checks pass.

Test slices (RED-phase):
- N/A — Phase 6 is verification, not feature work.
- However, if any verification fails, file an issue and either fix
  in a sub-phase or document as V1.1 deferral.

---

## 4. File Impact

| Path | Phase | Purpose |
|------|-------|---------|
| `web/src/app/layout.tsx` | 1 | Global chrome (header, footer, theme provider) |
| `web/src/app/page.tsx` | 1 → 3 | Home + query input + results (rewritten across phases) |
| `web/src/app/source/[doc_id]/page.tsx` | 1 → 4 | Source detail page |
| `web/src/app/history/page.tsx` | 1 → 5 | History page |
| `web/src/app/q/[request_id]/page.tsx` | 1 | Alias to home with request context |
| `web/src/app/globals.css` | 1 → 6 | CSS variables; brand-token placeholders |
| `web/src/app/_config/env.ts` | 1 | Env var exports |
| `web/src/components/query-input/` | 2 | Query input + advanced filters + submit / cancel |
| `web/src/components/results-static/` | 2 | Buffered JSON render (REQ-UI-007 fallback) |
| `web/src/components/results-stream/` | 3 | Per-sentence streaming render |
| `web/src/components/citation-badge/` | 2 → 4 | Inline `[N]` badge with HoverCard / Sheet |
| `web/src/components/source-card/` | 4 | Source card (citation-shape only V1) |
| `web/src/components/all-sources-panel/` | 4 | Deduplicated source list right-rail |
| `web/src/components/empty-state/` | 2 | Empty results / unknown source / empty history |
| `web/src/components/error-state/` | 2 → 4 | Stream error / transport error / cancelled |
| `web/src/components/loading-skeleton/` | 1 → 3 | Reserved-space skeleton |
| `web/src/components/header/`, `footer/` | 1 | Chrome |
| `web/src/lib/sse-client.ts` | 3 | Native EventSource wrapper |
| `web/src/lib/citation-resolver.ts` | 2 | Pure: render plan from sentence + citations |
| `web/src/lib/language-detect.ts` | 2 | Pure: ko vs en detection |
| `web/src/lib/history-store.ts` | 5 | localStorage wrapper |
| `web/src/lib/strings.ts` | 1 → ongoing | Dictionary (en + ko chrome strings) |
| `web/src/lib/utils.ts` | 1 | shadcn/ui `cn()` helper (created on first shadcn install) |
| `web/tailwind.config.ts` | 1 | Add Korean font fallback to `font-sans` |
| `web/components.json` | 1 | (Already exists from BOOT-001; no change) |
| `web/package.json` | 1, 2, 3 | Add Vitest, RTL, Playwright; shadcn install pulls Radix UI + clsx + tailwind-merge |
| `web/pnpm-lock.yaml` | 1, 2, 3 | Updated by `pnpm install` |
| `web/.env.example` | 1 | New file documenting `NEXT_PUBLIC_*` env vars |
| `web/vitest.config.ts` | 1 | Vitest config |
| `web/playwright.config.ts` | 1 | Playwright config (chromium + webkit; mobile viewport) |
| `web/e2e/01-submit-stream-citation.spec.ts` | 3 | E2E: submit → stream → click citation |
| `web/e2e/02-cancel-mid-stream.spec.ts` | 3 | E2E: submit → cancel mid-stream |
| `web/e2e/03-korean-query.spec.ts` | 3 | E2E: Korean query + history entry |
| `web/src/**/*.test.ts(x)` | 2, 3, 4, 5 | Component + unit tests |

Note: `web/src/components/` and `web/src/lib/` currently contain
only `.gitkeep` placeholders from BOOT-001 — SPEC-UI-001 is the
first SPEC to add real files under these paths.

---

## 5. TDD Plan

Per `methodology: tdd`, each phase follows RED-GREEN-REFACTOR per
test slice. Representative test slices are enumerated under each
Phase above; the implementer derives the full set from every REQ
and NFR acceptance bullet in `spec.md` §4 and `acceptance.md`.

Two test layers:

1. **Vitest + React Testing Library** — fast unit + component tests.
   Run on every save in dev and as a CI gate.
2. **Playwright** — slow end-to-end tests for the three top user
   journeys. Run pre-merge and on PR.

Property test (NFR-UI-004) is run via Vitest with `fast-check`
generators.

---

## 6. Dependencies

Upstream prerequisites for Run-time (not for SPEC-time):

| SPEC | Status at SPEC-time | Required for |
|------|---------------------|--------------|
| SPEC-BOOT-001 | implemented | `web/` scaffold (Phase 0) |
| SPEC-SYN-001 | implemented | JSON fallback contract (Phase 2) |
| SPEC-SYN-002 | implemented | Citation invariant (Phases 2, 3) |
| SPEC-SYN-004 | implemented | SSE wire contract (Phase 3) |
| SPEC-IR-001 | TBD at SPEC-time | `cmd/usearch-api` HTTP synthesis endpoint (Phase 2 onward — fixture server until IR-001 lands) |

SPEC-UI-001 **blocks**:

| SPEC | Consumption point |
|------|--------------------|
| SPEC-UI-002 | Admin UI reuses chrome (layout, header, footer), shared shadcn/ui component set, theme tokens, lib/strings.ts dictionary |

Deferred dependencies (UI does NOT block, but the UI inherits
constraints when these land):

- SPEC-AUTH-001 — `/login`, JWT, custom headers (V1.1 switch from
  `EventSource` to `fetch` + `ReadableStream`).
- SPEC-AUTH-002 — team scoping for history.
- SPEC-IDX-005 — team-shared answer reuse for shared history view.
- `/moai design` path B pipeline output — `tokens.json` for brand
  visual identity (one-file CSS variable swap in `globals.css`).

---

## 7. Open Questions to Resolve in Annotation Cycle

The 9 OQ items from `research.md` §13 are restated here for the
annotation cycle. V1 defaults are documented; any change to a
default modifies one or more REQ scopes.

| OQ | Topic | V1 default |
|----|-------|-----------|
| OQ-1 | Source detail data (rich vs citation-only) | Citation-only |
| OQ-2 | Synthesis language tracking query language | UI does not enforce |
| OQ-3 | `/moai design` brand pipeline timing | After UI v1; CSS-variable swap path |
| OQ-4 | Data fetching client library | Plain `fetch` + native `EventSource` |
| OQ-5 | State management library | React Context + `useReducer` |
| OQ-6 | i18n library | Dictionary pattern (no library) |
| OQ-7 | History persistence | localStorage cap 50 FIFO |
| OQ-8 | Bundle size / perf budget enforcement | Advisory in V1 |
| OQ-9 | `/deep` mode toggle in V1 | In-scope as UI flag |

---

## 8. Risks (top-level summary, full register in research.md §14)

1. **R1 — Streaming render perf**: per-sentence keyed components +
   `React.memo` + append-only state.
2. **R2 — Citation popover stack on mobile**: shadcn `Sheet`
   variant for mobile, Radix portal for desktop.
3. **R3 — EventSource auto-reconnect re-charging LLM**: one
   reconnect then surface "Retry?".
4. **R4 — Korean text mis-render**: Korean font in Tailwind config;
   manual visual test on a Korean query at every phase exit.
5. **R5 — Brand tokens land mid-cycle**: CSS-variable indirection;
   one-file token swap.
6. **R6 — IR-001 not landed at Run start**: fixture server matching
   SYN-004 wire format; Playwright e2e tests run against fixture.
7. **R7 — URL serialization > 2 KB**: REQ-UI-002 caps query at 1024
   chars.

---

## 9. Definition of Done (per SPEC-UI-001)

- All 10 REQs in `spec.md` §3 pass acceptance criteria in
  `acceptance.md`.
- All 4 NFRs verified per `acceptance.md` §4.
- TDD coverage ≥ 85% on `web/src/**` lines (Vitest coverage report).
- `pnpm typecheck`, `pnpm lint`, `pnpm build` all green.
- Lighthouse a11y ≥ 95 on all four routes.
- Manual Playwright e2e green on chromium + webkit.
- Manual visual test on Korean query passes.
- `web/` README updated with run instructions for the new env vars.
- `.moai/specs/SPEC-UI-001/spec.md` status flipped from `draft` →
  `implemented` after PR merge.
- TRUST 5 quality gates passed.

---

*End of SPEC-UI-001 plan.md (draft v0.1).*
