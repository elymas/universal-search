---
id: SPEC-UI-001
version: 1.0.0
status: implemented
created: 2026-05-22
updated: 2026-05-26
author: limbowl
priority: P1
issue_number: 0
title: Web UI v1 — query input, streaming results, citation rendering
milestone: M7 — Surfaces
owner: expert-frontend
methodology: tdd
coverage_target: 85
depends_on: [SPEC-BOOT-001, SPEC-SYN-001, SPEC-SYN-002, SPEC-SYN-004, SPEC-IR-001]
blocks: [SPEC-UI-002]
---

# SPEC-UI-001: Web UI v1 — query, streaming synthesis, citation rendering

## HISTORY

- 2026-05-22 (initial draft v0.1, limbowl via manager-spec):
  First EARS-formatted SPEC for the M7 Web UI v1. Builds the first
  real pages under `web/` on top of the BOOT-001 scaffold (Next.js 16
  App Router, React 19, TypeScript 5+ strict, Tailwind 3.4,
  shadcn/ui configured but uninstalled, pnpm, Node 22 LTS).

  Surfaces: (1) home/query page with input + advanced filters, (2)
  results page rendering the streaming synthesized paragraph + inline
  citation badges + source cards, (3) source-detail panel (citation
  data only in V1 per OQ-1), (4) local-session history page (no
  login). Login + team-scoped views deferred to SPEC-AUTH-001 (M6,
  draft at SPEC-time). Admin UI deferred to SPEC-UI-002.

  Streaming consumes SPEC-SYN-004's SSE wire contract verbatim —
  `event: sentence` / `event: done` / `event: error` over
  `text/event-stream`, with the SPEC-SYN-002 citation invariant
  (every emitted sentence carries ≥1 valid `[N]` marker) preserved
  end-to-end through the rendering pipeline. Native browser
  `EventSource` API is the V1 client; switch to `fetch` +
  `ReadableStream` is a V1.1 path when SPEC-AUTH-001 requires
  custom JWT headers.

  Korean-first display is a HARD requirement per
  `.moai/project/product.md` §3 (Korean analyst persona) and roadmap
  M3 exit criterion. V1 detects query language at submit time via
  Hangul unicode-block ratio (≥30% Hangul ⇒ Korean) and renders
  source titles/snippets in source language verbatim; chrome stays
  in browser default. Korean-capable font fallback in the Tailwind
  `font-sans` stack.

  Brand visual identity (color palette, primary font, logo) is
  _TBD_ — `.moai/project/brand/visual-identity.md` is in template
  `_TBD_` state at SPEC-time. V1 ships with shadcn/ui slate defaults
  and CSS variables that swap to brand tokens via `/moai design`
  path B pipeline output (`tokens.json`). The brand-design pipeline
  is referenced, NOT duplicated.

  10 EARS REQs (8 × P0 + 2 × P1) covering all five EARS patterns. 4
  NFRs (a11y baseline WCAG 2.1 AA, performance aligned with
  SPEC-SYN-004, responsive breakpoint set, bundle budget advisory).
  Companion research artifact at
  `.moai/specs/SPEC-UI-001/research.md` — frontend stack confirmation,
  upstream contract trace, citation pattern survey, Korean rendering
  research, 9 Open Questions for annotation cycle. Detailed Given/
  When/Then scenarios in `.moai/specs/SPEC-UI-001/acceptance.md`.

  No GitHub issue tracking on this SPEC (`issue_number: 0`). Ready
  for plan-auditor review and annotation cycle.

---

## 1. Purpose

`.moai/project/roadmap.md` line 96 declares M7 SPEC-UI-001:

> Web UI v1 | Next.js 16 app, query + streaming citation UI, team
> dashboard, shadcn/ui | expert-frontend

`.moai/project/product.md` §4 commits Web UI as one of four V1
surfaces (alongside CLI, MCP, Skill). Today, the BOOT-001 scaffold
delivers an empty Next.js 16 app with a placeholder "Universal Search
— coming soon" home page and zero shadcn/ui components installed.
SPEC-SYN-004 is implemented and exposes the SSE streaming synthesis
contract over `cmd/usearch-api`. SPEC-IR-001 owns the HTTP server
scaffolding (prerequisite for runtime wiring). The Korean-first
display and citation faithfulness invariants are HARD constitutional
requirements.

SPEC-UI-001 delivers the **first usable Web UI** that lets a single
anonymous user submit a query and watch the synthesized, fully-cited
answer stream in sentence-by-sentence — with citations the user can
hover, click, and navigate from. It is the **smallest UI shippable
slice** that satisfies the M7 surfaces exit criterion for the
Web-UI lane.

Completion delivers four routes (`/`, `/q/[request_id]` or merged
into `/`, `/source/[doc_id]`, `/history`), an SSE streaming client
that consumes SPEC-SYN-004's wire format with zero protocol
modifications, a citation rendering surface that preserves the
SPEC-SYN-002 invariant end-to-end, Korean-locale rendering on
Korean queries, a WCAG 2.1 AA a11y baseline, and a responsive mobile-
first layout. It does NOT implement login, team-shared history, admin
controls, or brand visual identity — each of these has a known
destination SPEC (AUTH-001, IDX-005, UI-002, `/moai design` path B).

---

## 2. Scope

### 2.1 In-Scope

| Ref | Item |
|-----|------|
| a | [NEW] `web/src/app/page.tsx` rewritten as home + query input + (post-submit) inline results view. Combines the query input and the streaming result display in a single route to keep V1 navigation flat. |
| b | [NEW] `web/src/app/source/[doc_id]/page.tsx` — source detail page rendering the citation data (`marker, doc_id, url, title`) plus a prominent "Open source" link to `url`. V1 renders citation-data only per OQ-1. |
| c | [NEW] `web/src/app/history/page.tsx` — local-session history list. Reads from `localStorage` key `usearch:history` (capped at 50 entries per OQ-7); displays `{query, timestamp, request_id}`; clicking an entry re-submits the query. No server-side persistence. |
| d | [NEW] `web/src/app/layout.tsx` updated with global chrome (header with brand mark placeholder, footer with version, theme provider for CSS-variable token swap). Brand mark + footer copy use placeholder strings until brand interview ships. |
| e | [NEW] `web/src/components/query-input/` — query input component with:  (i) text area (multiline, max 1024 chars per REQ-UI-002), (ii) advanced-filters disclosure (initially collapsed) exposing source-category toggles (`web`, `social`, `academic`, `korean`) and a `/deep` mode toggle (OQ-9 default: in-scope, UI-only — backend wiring is per existing SYN-004 endpoint), (iii) submit button, (iv) cancel button visible while a stream is in-flight. |
| f | [NEW] `web/src/components/results-stream/` — results display consuming SSE events:  per-sentence reveal with append-only React state, inline `[N]` citation badges, "All sources" panel listing union of citations (dedup by `doc_id`, first-occurrence order), live region (`aria-live="polite"`) announcing new sentences for screen readers, post-stream `done` summary (model, provider, latency, cost). |
| g | [NEW] `web/src/components/citation-badge/` — clickable inline `[N]` button rendering as a shadcn/ui `HoverCard` (desktop hover, mobile tap). Popover shows `title`, `url`, "Open source" link. Hidden text via `aria-label` ("Citation 3 of 7: <title>") for SR users. |
| h | [NEW] `web/src/components/source-card/` — source card used in (i) the "All sources" panel on the results view, (ii) the source-detail page. V1 renders only citation-shape fields (`marker, doc_id, url, title`). The card is open-source-link-forward (primary CTA opens `url` in a new tab). |
| i | [NEW] `web/src/components/empty-state/`, `web/src/components/error-state/`, `web/src/components/loading-skeleton/` — three small components covering the empty-results, stream-error, and pre-first-sentence loading affordances. The skeleton reserves vertical space to avoid CLS while the first sentence is in transit. |
| j | [NEW] `web/src/lib/sse-client.ts` — thin wrapper over native `EventSource` exposing `subscribe(url, { onSentence, onDone, onError, onConnectionLost, signal }) → cancel()`. Handles the three SPEC-SYN-004 event types, normalizes payload JSON parsing, surfaces connection-lost after one failed auto-reconnect (per research §7.2). |
| k | [NEW] `web/src/lib/citation-resolver.ts` — pure function that, given a sentence text + citations array (from one `event: sentence` payload), returns a render plan: `[{type:"text", value:"..."}, {type:"citation", index:3, citation:{...}}, ...]`. Used by the sentence renderer to substitute `[N]` markers with citation badges. |
| l | [NEW] `web/src/lib/language-detect.ts` — pure function detecting `ko` vs `en` from the first 50 non-whitespace chars of the query via Hangul unicode-block ratio (≥30% Hangul ⇒ `ko`). |
| m | [NEW] `web/src/lib/history-store.ts` — thin wrapper over `localStorage` for the `usearch:history` key. Cap 50 entries; FIFO eviction. JSON-shape `{query, request_id, timestamp_iso, language}`. |
| n | [NEW] `web/src/lib/strings.ts` — chrome-string dictionary keyed by locale (`en`, `ko`). V1 dictionary pattern; no i18n library. Covers ~50 UI strings (button labels, placeholders, empty-state messages, error messages). Locale choice for chrome follows browser `navigator.language` and falls back to `en`. |
| o | [NEW] `web/src/app/globals.css` extended with CSS-variable token definitions inherited from shadcn/ui slate baseColor; placeholders for brand tokens to be supplied later by `/moai design` path B pipeline output (`tokens.json`). Font stack includes Korean fallback: `Inter, "Noto Sans KR", "Apple SD Gothic Neo", sans-serif`. |
| p | [NEW] shadcn/ui components installed: `Button`, `Input`, `Textarea`, `HoverCard`, `Popover`, `Sheet`, `Skeleton`, `Toast`, `Collapsible`. Installation via `pnpm dlx shadcn@latest add ...`. |
| q | [NEW] Three additional `web/package.json` dependencies (chosen by V1 default unless annotation cycle changes): native `fetch` + native `EventSource` (no library); React Context + `useReducer` for client state (no library); dictionary pattern for i18n (no library); Vitest + React Testing Library for unit tests; Playwright for end-to-end tests. Net new prod dependencies are limited to whatever shadcn/ui installs pull in (Radix UI primitives, `clsx`, `tailwind-merge`). |
| r | [NEW] `web/src/app/_config/env.ts` — exports `USEARCH_API_BASE` from `process.env.NEXT_PUBLIC_USEARCH_API_BASE` with fallback `http://localhost:8080`. Documented in `web/.env.example` (file new). |
| s | [NEW] Unit tests under `web/src/**/*.test.ts(x)` for: `sse-client`, `citation-resolver`, `language-detect`, `history-store`, plus component tests for query-input, citation-badge, results-stream. |
| t | [NEW] Playwright e2e tests under `web/e2e/` for the three top user journeys: (i) submit query → watch streamed sentences → click citation → land on source URL in new tab; (ii) submit query → cancel mid-stream → verify stream closes; (iii) submit Korean query → verify Korean text rendered correctly + history entry recorded with `language:"ko"`. |
| u | [EXISTING — UNCHANGED] `web/package.json` framework-level pins (Next.js 16, React 19, TypeScript 5+, Tailwind 3.4) inherited from SPEC-BOOT-001. SPEC-UI-001 does NOT bump these. |
| v | [EXISTING — UNCHANGED] `cmd/usearch-api/`, `internal/synthesis/`, `internal/streamsynth/`, `internal/sse/`, `services/researcher/` — SPEC-UI-001 is a pure consumer of these. No Go-side or Python-side changes. |
| w | [EXISTING — UNCHANGED] SPEC-SYN-004 SSE wire contract — no protocol modifications. The UI consumes the schema-versioned (`schema_version: 1`) payloads verbatim. |
| x | [EXISTING — UNCHANGED] SPEC-SYN-002 citation invariant — preserved end-to-end through the citation badge rendering pipeline. |

### 2.2 Exclusions (What NOT to Build)

[HARD] This SPEC explicitly excludes the following items. Each has a
known destination SPEC or follow-up; this list prevents scope creep.
At least one exclusion entry is required per project policy; this
section lists ten.

- **NOT login / authentication / authorization UI** — `/login`,
  OAuth callback, JWT handling, "Sign in with..." buttons.
  → SPEC-AUTH-001 (M6, draft at SPEC-time). V1 assumes anonymous,
  single-user, single-team baseline.
- **NOT team-scoped or team-shared history views** — "Shared
  queries", "Team activity", team member list. → SPEC-IDX-005
  (M6 team-shared answer reuse) + SPEC-AUTH-002 (RBAC). V1 history
  is local-session only (browser `localStorage`).
- **NOT admin surface** — adapter status dashboard, API key
  management, audit-log viewer. → SPEC-UI-002 (M7 second SPEC).
- **NOT rich source cards** — snippet text, published_at, score,
  source-engine badge with favicon, social-engagement metrics.
  → Either a follow-up SPEC that extends the synthesis endpoint
  payload, OR V1.1 of UI-001. V1 renders citation-shape fields only
  (`marker, doc_id, url, title`) per research §3.3 / OQ-1.
- **NOT brand visual identity** — brand color palette, primary
  typeface, logo design, hero layout, photography style. → `/moai
  design` path B pipeline (`moai-domain-brand-design`,
  `evaluator-active` GAN loop). V1 ships with shadcn/ui slate
  defaults and CSS variables prepared to consume the pipeline's
  `tokens.json` output without component changes.
- **NOT brand voice / microcopy refinement** — final tone, English-
  Korean voice parity, hero headline copy. → `/moai design` path B
  pipeline (`moai-domain-copywriting`). V1 chrome uses placeholder
  English + Korean strings in `web/src/lib/strings.ts`.
- **NOT cross-call event-stream resume** — `Last-Event-ID` header
  support, mid-stream resume on transient disconnect. SPEC-SYN-004
  §2.2 explicitly excludes resume server-side; SPEC-UI-001 mirrors
  that exclusion client-side. After one failed auto-reconnect, the
  UI shows "Retry?" instead of looping.
- **NOT WebSocket transport, gRPC-Web, or any non-SSE streaming
  protocol** — SPEC-SYN-004 is SSE-only; SPEC-UI-001 mirrors.
- **NOT token-level / sub-sentence streaming UI** — sentence is the
  atomic render unit. Sub-sentence streaming is gated on SPEC-SYN-004
  follow-up (sidecar streaming upgrade).
- **NOT automated visual regression tests, automated a11y CI gates
  (`@axe-core/playwright`), or perf budget CI gates** in V1. These
  are V1.1 candidates. V1 uses manual Lighthouse + axe-core
  dev-tools extension review before merge.
- **NOT a real "team dashboard" page** despite the roadmap row
  mentioning it — the roadmap row is aspirational for M7's UI lane
  as a whole. The team dashboard depends on team data (SPEC-AUTH-*,
  SPEC-IDX-*) that are M6 SPECs in draft. SPEC-UI-001 v1 ships
  without it; a follow-up SPEC adds it after M6 lands.
- **NOT marketing / landing-page content** beyond the home query
  input. No about page, no pricing, no signup.
- **NOT GitHub Issue tracking on this SPEC** (`issue_number: 0`).

---

## 3. EARS Requirements

### Functional Requirements

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| REQ-UI-001 | Ubiquitous | The Web UI SHALL expose four user-reachable routes under the Next.js 16 App Router: `/` (home + query input + post-submit inline results), `/source/[doc_id]` (source detail), `/history` (local-session history list), and a `/q/[request_id]` route alias resolving to `/` with a pre-loaded request context. All routes SHALL render valid HTML5 with `<html lang>` reflecting the browser default or the per-results-region language when the language attribute is overridden for the streamed answer region. The chrome (header, footer, navigation affordances) SHALL be consistent across all four routes via a shared `web/src/app/layout.tsx`. | P0 | `test_route_home_renders`, `test_route_source_detail_renders_with_citation_data`, `test_route_history_renders_with_zero_or_many_entries`, `test_route_q_alias_resolves_to_home_view`. |
| REQ-UI-002 | Ubiquitous | The query input SHALL accept a free-text query of length 1 to 1024 Unicode characters (post-`normalize("NFC")`), SHALL reject submission of empty / whitespace-only / over-length input with an inline error message localized to the chrome language, SHALL expose an advanced-filters disclosure that toggles source-category checkboxes (`web`, `social`, `academic`, `korean`) and a `/deep` mode toggle (UI-only flag passed to the backend), SHALL render a submit button enabled only when the input is valid, AND SHALL render a cancel button visible only while a stream is in-flight that calls `EventSource.close()` (or equivalent `fetch` `AbortController.abort()` in the V1.1 path). | P0 | `test_query_input_rejects_empty`, `test_query_input_rejects_over_1024_chars`, `test_query_input_accepts_korean_text`, `test_advanced_filters_toggle_visible`, `test_deep_mode_toggle_passes_flag_to_backend`, `test_cancel_button_hidden_pre_submit`, `test_cancel_button_visible_during_stream`. |
| REQ-UI-003 | Event-Driven | WHEN the user submits a valid query AND the `NEXT_PUBLIC_USEARCH_API_BASE` env var is configured, THEN the UI SHALL open an SSE connection to the synthesis endpoint with `Accept: text/event-stream`, SHALL render a skeleton loader reserving vertical space for the answer region (preventing layout shift per NFR-UI-002), SHALL accumulate `event: sentence` payloads into ordered per-sentence components keyed by `sentence_index`, SHALL render each sentence with inline `[N]` citation badges substituted by the `citation-resolver` library, SHALL announce each new sentence via an `aria-live="polite"` live region for screen readers, SHALL render a final `done` summary banner on `event: done` containing `{model, provider, latency_ms, cost_usd}`, AND SHALL append the query to the local-session history store via `history-store.append({query, request_id, timestamp_iso, language})`. | P0 | `test_sse_connection_opens_with_accept_header`, `test_skeleton_visible_pre_first_sentence`, `test_sentences_render_in_order_by_sentence_index`, `test_citation_badges_substituted_inline`, `test_aria_live_region_announces_new_sentence`, `test_done_summary_visible_post_stream`, `test_history_entry_appended_post_done`. |
| REQ-UI-004 | Ubiquitous | The citation badge SHALL preserve the SPEC-SYN-002 citation faithfulness invariant end-to-end: every visible `[N]` marker in the rendered answer region SHALL be replaced by a clickable badge that resolves to a `Citation` object (`{marker, doc_id, url, title}`) found in the corresponding `event: sentence` payload's `citations` array. NO sentence text SHALL render in the answer region if its source `event: sentence` payload does not pass the SPEC-SYN-004 NFR-SYN4-002 (b)(c) invariant (every sentence has ≥1 valid `[N]` marker, every marker resolves to a `doc_id`). The badge SHALL expose `aria-label="Citation <N> of <total>: <title>"` for SR users, SHALL render a `HoverCard` (shadcn/ui Radix-based) on pointer hover or touch tap revealing `{title, url, "Open source" link}`, AND SHALL open `url` in a new tab when clicked. | P0 | `test_every_marker_substituted_by_badge`, `test_badge_aria_label_present`, `test_hovercard_opens_on_hover_pointer`, `test_hovercard_opens_on_tap_touch`, `test_open_source_opens_url_in_new_tab`, `test_sentence_with_missing_citation_not_rendered` (defensive — should never occur per SYN-004 invariant; test asserts UI defense in depth). |
| REQ-UI-005 | Ubiquitous | The source detail page at `/source/[doc_id]` SHALL render the citation-shape fields (`marker, doc_id, url, title`) for the supplied `doc_id` resolved against the current session's last-seen citation set (held in client state during the active session; refresh-survival is _TBD_ per OQ-1 — V1 default: page resolves doc_id against URL params + last-stream citation cache; if doc_id is unknown, show "Source not in current session" empty state with a link back to `/`). The page SHALL present a prominent "Open source" CTA navigating to `url` in a new tab as the primary action. The page is intentionally minimal in V1; richer source-card content (snippet, published_at, source-engine badge) is excluded per §2.2. | P0 | `test_source_page_renders_with_known_doc_id`, `test_source_page_shows_empty_state_for_unknown_doc_id`, `test_open_source_cta_navigates_new_tab`. |
| REQ-UI-006 | Event-Driven | WHEN a stream is interrupted by `event: error` from the server, by a transport-level disconnect, or by the user clicking cancel, THEN the UI SHALL stop the EventSource within 200 ms, SHALL render a stream-error component showing the `error_message` from the error payload (or "Connection lost" for transport errors, or "Cancelled" for user-initiated cancel — localized via `strings.ts`), SHALL preserve any sentences already rendered (no destructive re-paint), SHALL hide the live skeleton, SHALL show a "Retry" button that re-submits the same query (re-creating a fresh stream — no `Last-Event-ID` resume per §2.2 exclusion), AND SHALL increment client-side metric `usearch_ui_stream_outcome{outcome=...}` via a `console.log` / future telemetry hook with one of `{client_disconnect, server_error, user_cancel, connection_lost}` outcomes. | P0 | `test_error_event_shows_error_component`, `test_transport_disconnect_shows_connection_lost`, `test_user_cancel_shows_cancelled`, `test_retry_button_creates_fresh_stream`, `test_previously_rendered_sentences_preserved`, `test_skeleton_hidden_post_error`. |
| REQ-UI-007 | Optional | WHERE the environment configuration sets `NEXT_PUBLIC_USEARCH_SSE_ENABLED=false`, OR the user-agent reports `EventSource` is `undefined`, the UI SHALL fall back to the buffered JSON path: SHALL send the same query with `Accept: application/json` (or omit `Accept` entirely) to the synthesis endpoint, SHALL display the full-paragraph response as one block once received (no progressive reveal), SHALL still substitute `[N]` markers with citation badges using the same `citation-resolver` library, AND SHALL log a single console.info "SSE disabled; using buffered JSON fallback" once per session. This preserves the SPEC-SYN-004 REQ-SYN4-005 backward-compatible JSON contract. | P1 | `test_sse_disabled_env_uses_json_path`, `test_no_eventsource_global_uses_json_path`, `test_json_fallback_renders_full_paragraph`, `test_json_fallback_substitutes_citations`. |
| REQ-UI-008 | State-Driven | WHILE the submitted query is classified as Korean (per `language-detect.ts`: Hangul block `U+AC00–U+D7A3` ≥30% of the first 50 non-whitespace characters), THE answer region SHALL render with `lang="ko"` on the wrapping element, SHALL use the Korean-fallback font stack (`Inter, "Noto Sans KR", "Apple SD Gothic Neo", sans-serif`), SHALL NOT truncate Korean text mid-character (avoid mid-Hangul-syllable ellipsis), AND SHALL persist `{language:"ko"}` on the appended history entry. Source titles and snippets SHALL be rendered in the source language verbatim regardless of query language (no client-side translation). The chrome (header / nav / footer) language is independent and follows `navigator.language` with `en` fallback. | P0 | `test_korean_query_sets_lang_attr`, `test_korean_query_uses_korean_font_stack`, `test_korean_query_recorded_in_history_with_language_ko`, `test_english_query_does_not_set_lang_ko`, `test_korean_text_not_truncated_mid_character`. |
| REQ-UI-009 | Ubiquitous | The `/history` route SHALL render the local-session history list from `localStorage` key `usearch:history`, capped at 50 entries (FIFO eviction on append per `history-store.ts`), SHALL display per-entry `{query, timestamp_iso (formatted via `Intl.DateTimeFormat`), language badge}`, SHALL render a "Re-run" affordance per entry that re-submits the query (navigating to `/` with the query pre-filled), SHALL render a "Clear history" button that empties the store after a confirmation dialog (shadcn/ui `Dialog`), AND SHALL render an empty-state component when the store is empty. History persistence is local-session only; server-side persistence is deferred to SPEC-AUTH-001. | P0 | `test_history_renders_with_zero_entries_empty_state`, `test_history_renders_50_entries_capped`, `test_history_rerun_navigates_to_home_with_prefilled_query`, `test_history_clear_after_confirm_empties_store`, `test_history_capped_at_50_fifo_eviction`. |
| REQ-UI-010 | Unwanted | IF the citation-resolver encounters an `[N]` marker for which no matching citation exists in the current sentence's `citations` array (a defensive check — this state SHOULD be unreachable per SPEC-SYN-002 invariant), THEN the UI SHALL render the literal `[N]` text verbatim (no badge, no popover), SHALL emit a `console.warn` with `{request_id, sentence_index, marker, citations_seen}`, AND SHALL increment a client-side `usearch_ui_invariant_violation{kind:"unresolved_marker"}` metric. The UI SHALL NEVER crash, render nothing, or silently drop the sentence — defense in depth against upstream contract drift. | P1 | `test_unresolved_marker_renders_literal_text`, `test_unresolved_marker_warns_console`, `test_unresolved_marker_does_not_crash_sentence_render`. |

### Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| NFR-UI-001 | A11y baseline (WCAG 2.1 AA) | The Web UI SHALL meet WCAG 2.1 AA for the four V1 routes: (a) all interactive elements (query input, advanced-filters disclosure, submit, cancel, citation badges, source cards, re-run, clear-history) SHALL be reachable via keyboard Tab order with a visible focus indicator (≥3:1 contrast ratio); (b) body text contrast ≥4.5:1, large text ≥3:1, focus ring ≥3:1; (c) streaming sentences SHALL be announced to screen readers via an `aria-live="polite"` region on the answer container; (d) all images and icons SHALL have descriptive `alt` text or `aria-hidden="true"` when decorative; (e) all form inputs SHALL have associated `<label>` or `aria-label`; (f) all popovers (citation HoverCards) SHALL be dismissible via ESC key and outside-click; (g) `<html lang>` SHALL reflect the active locale, with the answer region overriding to `lang="ko"` for Korean queries per REQ-UI-008. Verification in V1 is manual Lighthouse run + axe-core dev-tools review documented in `acceptance.md` §4.1. Automated CI gating is V1.1. |
| NFR-UI-002 | Performance (TTFB-to-first-sentence, total stream overhead, CLS) | Aligned with SPEC-SYN-004 NFR-SYN4-001: time from user submit to first `event: sentence` rendered in DOM SHALL be ≤ (server TTFB to first event) + 100 ms client overhead p95; total stream completion (last sentence + done summary rendered) SHALL be ≤ (server total stream wall-clock) + 200 ms client overhead p95; First Contentful Paint of `/` SHALL be ≤ 1.5 s on a fast 3G simulated network; Largest Contentful Paint on the post-stream results view SHALL be ≤ 2.5 s; Cumulative Layout Shift during the streaming reveal SHALL be < 0.1 (skeleton loader reserves vertical space). Total JavaScript shipped to the client for the home + query path SHALL be ≤ 200 KB gzipped (advisory budget in V1; hard CI gate in V1.1 per OQ-8). Verification per `acceptance.md` §4.4. |
| NFR-UI-003 | Responsive baseline (mobile-first) | The Web UI SHALL render correctly across the Tailwind default breakpoint set (`sm: 640 px`, `md: 768 px`, `lg: 1024 px`, `xl: 1280 px`): (a) `< 640 px`: single-column stack, query input full-width, source cards stacked vertically, citation HoverCard becomes a bottom Sheet, advanced filters become a bottom Sheet; (b) `≥ 640 px`: two-column option for source-list + answer; (c) `≥ 768 px`: source list as a right-rail at ~33% width, answer at ~67%; (d) `≥ 1024 px`: persistent left-rail for the history view; (e) `≥ 1280 px`: content max-width capped at 1280 px for readability. All interactive elements on touch targets SHALL meet WCAG 2.5.5 minimum 44×44 px. Verification per `acceptance.md` §4.5. |
| NFR-UI-004 | SSE wire conformance + invariant preservation (property) | For all valid SPEC-SYN-004 `event: sentence` payloads (where every sentence text contains ≥1 valid `[N]` marker resolving to a `doc_id` in the payload's `citations` array, per SYN-004 NFR-SYN4-002), the UI rendering pipeline SHALL satisfy: (a) every visible `[N]` marker in the rendered DOM is wrapped in a citation badge component (no literal `[N]` text strings in the answer region, except in the defensive REQ-UI-010 unreachable case); (b) the union of `[N]` badges across all rendered sentences, deduplicated by `doc_id`, equals the "All sources" panel list; (c) the order of sentence rendering matches monotonic `sentence_index` from the SSE stream; (d) exactly one of `event: done` summary OR error component is rendered after stream termination; (e) the live region announcements SR users hear contain the sentence text with `[N]` markers replaced by spoken citation labels (e.g. "citation 3"). Property test via Vitest + fast-check generators producing realistic event sequences (mixed Korean + English, varying citation densities, varying sentence counts, occasional error mid-stream). |

---

## 4. Acceptance Criteria

Detailed Given/When/Then scenarios with edge cases live in
`.moai/specs/SPEC-UI-001/acceptance.md`. This section enumerates the
high-level acceptance gate per requirement; the full set of test
slices is derived by the Run-phase implementer per `methodology: tdd`.

### REQ-UI-001 — Four routes render
- `test_route_home_renders` — `GET /` returns HTML containing query input.
- `test_route_source_detail_renders_with_citation_data` — `GET /source/<known-doc-id>` returns HTML with citation fields and "Open source" CTA.
- `test_route_history_renders_with_zero_or_many_entries` — both empty and full history states render correctly.
- `test_route_q_alias_resolves_to_home_view` — `/q/<request_id>` renders the same component tree as `/` with request context pre-loaded.

### REQ-UI-002 — Query input validation
- Empty input → submit disabled.
- 1024-char input → submit enabled.
- 1025-char input → inline error "Query too long".
- Whitespace-only input → submit disabled.
- Korean text (Hangul) accepted.
- Advanced filters disclosure toggles visibility.
- `/deep` toggle passes flag.
- Cancel button visible only during in-flight stream.

### REQ-UI-003 — SSE happy path
- Submit valid query → EventSource opens with `Accept: text/event-stream`.
- Skeleton visible until first `event: sentence`.
- N `event: sentence` events → N sentence components rendered, in `sentence_index` order, with citation badges.
- `event: done` → summary banner with model, provider, latency, cost.
- History entry appended with `{query, request_id, timestamp_iso, language}`.

### REQ-UI-004 — Citation invariant preservation
- Every `[N]` in sentence text → badge in DOM.
- Badge `aria-label` matches "Citation <N> of <total>: <title>".
- HoverCard opens on desktop hover.
- HoverCard opens on touch tap.
- "Open source" link opens `url` in new tab (`target="_blank" rel="noopener noreferrer"`).
- Defensive: sentence missing citation marker → not rendered (should never occur).

### REQ-UI-005 — Source detail page
- Known `doc_id` from current session → citation data + Open source CTA.
- Unknown `doc_id` → "Source not in current session" empty state.

### REQ-UI-006 — Stream interruption
- `event: error` → error component with error message.
- Transport disconnect → "Connection lost" component.
- User cancel → "Cancelled" component.
- Retry button creates fresh stream.
- Previously rendered sentences preserved.
- Skeleton hidden.

### REQ-UI-007 — JSON fallback (Optional)
- `NEXT_PUBLIC_USEARCH_SSE_ENABLED=false` → uses JSON path.
- Undefined `EventSource` → uses JSON path.
- Full paragraph rendered as one block, citations substituted.

### REQ-UI-008 — Korean-first display
- Korean query (≥30% Hangul) → answer region has `lang="ko"`.
- Korean font stack applied.
- History entry recorded with `language:"ko"`.
- English query → `lang="ko"` NOT set on answer region.
- Korean text not truncated mid-character.

### REQ-UI-009 — History page
- Zero entries → empty state.
- 50 entries → all rendered; 51st append evicts oldest (FIFO).
- Re-run navigates to `/` with query pre-filled.
- Clear button after confirm → empty store.

### REQ-UI-010 — Defensive citation rendering
- Unresolved marker → literal `[N]` text rendered.
- `console.warn` emitted.
- No crash.

### NFR-UI-001 — A11y baseline
- Lighthouse a11y score ≥ 95 on all four routes.
- axe-core dev-tools shows 0 violations on critical issues.
- Keyboard-only navigation reaches every interactive element.
- Screen reader announces streamed sentences.

### NFR-UI-002 — Performance
- TTFB-to-first-sentence client overhead p95 ≤ 100 ms (measured via PerformanceObserver hooks).
- FCP `/` ≤ 1.5 s on fast-3G.
- LCP results view ≤ 2.5 s.
- CLS during stream < 0.1.
- JS shipped (gzipped) ≤ 200 KB (advisory).

### NFR-UI-003 — Responsive
- Manual visual test at each breakpoint: 375, 640, 768, 1024, 1280 px wide.
- Touch targets ≥ 44×44 px on `< 768 px` viewport.

### NFR-UI-004 — Wire conformance property test
- 50 generated event sequences → 100% pass on invariants (a)–(e).

---

## 5. Technical Approach (high-level — no implementation code)

Detailed plan, file impact, and test plan live in
`.moai/specs/SPEC-UI-001/plan.md`. High-level approach:

- **Stack baseline confirmed from BOOT-001**: Next.js 16 App Router,
  React 19, TypeScript 5+ (strict, `paths: { "@/*": ["./src/*"] }`),
  Tailwind 3.4, shadcn/ui (slate baseColor, configured via
  `web/components.json`), pnpm, Node 22 LTS. ESLint 9 flat config +
  Prettier 3 per BOOT-001.
- **Stack additions (_TBD_ — final pick during annotation cycle)**:
  data fetching = plain `fetch` + native `EventSource` (V1 default);
  client state = React Context + `useReducer` (V1 default; small
  state surface); i18n = dictionary pattern (V1 default; no library);
  testing = Vitest + React Testing Library + Playwright;
  shadcn/ui component installs = `Button`, `Input`, `Textarea`,
  `HoverCard`, `Popover`, `Sheet`, `Skeleton`, `Toast`, `Collapsible`,
  `Dialog`.
- **Page layout**: home route (`/`) hosts query input + results
  stream in one component tree (no navigation step between submit
  and results). `/q/[request_id]` is an alias that hydrates the same
  component tree with request context. `/source/[doc_id]` and
  `/history` are separate routes with the shared chrome layout.
- **SSE client**: thin wrapper over native `EventSource` —
  `web/src/lib/sse-client.ts`. Surface:
  `subscribe(url, callbacks, signal) → cancel()`. Handles JSON parse
  for the three event types, surfaces connection-lost after one
  failed auto-reconnect (per research §7.2). Switch to `fetch` +
  `ReadableStream` is a V1.1 path when SPEC-AUTH-001 lands.
- **Citation rendering pipeline**: per-sentence pure function in
  `web/src/lib/citation-resolver.ts` produces a render plan
  `[{type:"text",value:"..."}, {type:"citation",index:3,citation:{...}}, ...]`.
  The sentence renderer maps this to React nodes (text spans +
  CitationBadge components). Citation badge is a shadcn/ui
  `HoverCard` wrapping a `Button` with `aria-label`. The "All
  sources" panel is the deduplicated union across all rendered
  sentences, keyed by `doc_id`, first-occurrence-order.
- **Korean-first display**: `web/src/lib/language-detect.ts` returns
  `"ko"` if Hangul block ≥30% of first 50 non-whitespace chars; the
  answer region wrapper sets `lang={detectedLang}`. Tailwind
  `font-sans` extended to include Korean fallback via
  `tailwind.config.ts` `theme.extend.fontFamily.sans`.
- **History store**: `web/src/lib/history-store.ts` — localStorage,
  cap 50, FIFO. Hydrated on client mount (no SSR for history page —
  use `"use client"` directive).
- **Error / empty / loading states**: three small components
  (`error-state`, `empty-state`, `loading-skeleton`). The skeleton
  reserves the same vertical space as ~6 lines of text to avoid CLS.
- **Brand token integration**: `web/src/app/globals.css` CSS variables
  follow shadcn/ui slate default. When `/moai design` path B
  pipeline ships `tokens.json`, a future SPEC swaps the variable
  definitions in `globals.css` without touching components.
- **Testing**: Vitest + React Testing Library for unit + component;
  Playwright for the three e2e user journeys (submit → cancel
  → Korean query). No mock service worker for V1; Playwright tests
  run against a real `cmd/usearch-api` (or a fixture server in CI
  per Playwright config).

---

## 6. Risks (top-level summary)

Detailed risk register lives in
`.moai/specs/SPEC-UI-001/research.md` §14. Top three for SPEC-author
attention:

1. **Streaming render perf (R1)** — naïve full-paragraph re-render
   on every `event: sentence` is O(n²) DOM work. Mitigated by
   per-sentence keyed `<span>`s, append-only state, `React.memo` on
   sentence component.
2. **Source detail without rich data (OQ-1 risk)** — V1 source page
   has only citation-shape fields (no snippet, no published_at).
   This may feel thin to product/strategy persona. Mitigation: clear
   "Open source" CTA + documented exclusion + V1.1 path.
3. **Brand tokens land mid-cycle (R3)** — `/moai design` path B may
   produce `tokens.json` while UI v1 is in Run phase. Mitigated by
   CSS-variable indirection so token swap is a one-file change.

---

## 7. References

Internal:

- `.moai/project/product.md` §3, §4 — Korean persona; V1 Web UI surface
- `.moai/project/roadmap.md` line 96 — SPEC-UI-001 row; line 129 M7 plan
- `.moai/project/tech.md` §3 Surfaces — Next.js 16 + shadcn/ui + Tailwind
- `.moai/project/brand/{brand-voice,target-audience,visual-identity}.md` — _TBD_; deferred to `/moai design` path B
- `.claude/rules/moai/design/constitution.md` — design system constitutional rules
- `.moai/specs/SPEC-BOOT-001/spec.md` REQ-BOOT-003, §10 — frontend scaffold (Next.js 16 + shadcn/ui ready)
- `.moai/specs/SPEC-SYN-004/spec.md` REQ-SYN4-001a/b/c, 002, 003, 004, 005, 006; NFR-SYN4-001/002/003 — SSE wire contract
- `.moai/specs/SPEC-SYN-002/spec.md` REQ-SYN2-001 — citation faithfulness invariant
- `.moai/specs/SPEC-SYN-001/spec.md` `Result` JSON shape (preserved by SYN-004 fallback per REQ-SYN4-005)
- `.moai/specs/SPEC-IR-001/` — `cmd/usearch-api` HTTP server scaffolding (runtime prerequisite)
- `.moai/specs/SPEC-AUTH-001/spec.md` — login deferral (M6, draft)
- `web/package.json`, `web/components.json`, `web/tsconfig.json`,
  `web/tailwind.config.ts`, `web/eslint.config.mjs` — confirmed
  stack from BOOT-001
- `.moai/specs/SPEC-UI-001/research.md` — companion research
- `.moai/specs/SPEC-UI-001/plan.md` — implementation plan
- `.moai/specs/SPEC-UI-001/acceptance.md` — detailed acceptance scenarios

External (verify via WebFetch in Run phase per anti-hallucination policy):

- W3C "Server-Sent Events" living standard
- MDN `EventSource` API reference
- Next.js 16 App Router docs (vercel/next.js)
- React 19 docs (facebook/react)
- shadcn/ui component docs (`HoverCard`, `Popover`, `Sheet`, `Dialog`, `Skeleton`)
- Radix UI primitives (a11y semantics)
- Tailwind CSS responsive + a11y docs
- WCAG 2.1 AA quick reference (w3.org)
- Perplexity, ChatGPT Search, Anthropic Citations (citation interaction product references)

---

*End of SPEC-UI-001 v0.1 (draft).*
