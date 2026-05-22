# SPEC-UI-001 — Research

Companion research artifact for SPEC-UI-001 (Web UI v1 — query + streaming
citation UI for Universal Search). This document captures the codebase
analysis, upstream contracts the UI consumes, and the external pattern
survey that informs the EARS requirements in `spec.md`. No
implementation; no code samples beyond the minimum needed to pin a
contract.

---

## 1. Scope of investigation

- The frontend scaffold delivered by SPEC-BOOT-001 (`web/`).
- The streaming synthesis contract delivered by SPEC-SYN-004 (SSE wire
  format, event schema, citation invariant from SPEC-SYN-002).
- The HTTP synthesis endpoint surface owned by SPEC-IR-001
  (`cmd/usearch-api/`) which the UI will consume.
- The brand context constitution (`.moai/project/brand/`) which is the
  upstream of any visual identity decisions.
- Korean-locale rendering requirements per `.moai/project/product.md`
  §3 and roadmap M3 exit criterion (Korean query → Korean sources
  ranked first).
- A11y baseline (WCAG 2.1 AA target) and responsive baseline
  (mobile-first, breakpoint set TBD).

---

## 2. Frontend stack — discovered from `web/` scaffold

The following is **confirmed** by reading the files committed in
SPEC-BOOT-001. Versions are the exact major lines pinned in
`web/package.json` at the time of this research.

| Concern | Choice (confirmed) | Source |
|---------|--------------------|--------|
| Framework | Next.js 16 (App Router, RSC enabled) | `web/package.json` `next: ^16.0.0`; `web/components.json` `"rsc": true` |
| Language | TypeScript 5.x (strict mode) | `web/tsconfig.json` `"strict": true`, `"moduleResolution": "bundler"`, `paths { "@/*": ["./src/*"] }` |
| React | React 19 | `web/package.json` `react: ^19.0.0` |
| Styling | Tailwind CSS 3.4 | `web/tailwind.config.ts`, `web/postcss.config.mjs` |
| Component library | shadcn/ui (slate baseColor, CSS variables, `@/components`, `@/lib/utils` aliases) — **configured, not installed** | `web/components.json` declares manifest; `web/src/components/` and `web/src/lib/` are empty (`.gitkeep` only) |
| Package manager | pnpm | `web/pnpm-lock.yaml`, `web/README.md` "Quickstart" |
| Lint / Format | ESLint 9 flat config + Prettier 3 + eslint-config-next | `web/eslint.config.mjs`, `web/.prettierrc.json`, `web/package.json` devDeps |
| Node runtime | 22 LTS (CI per BOOT-001 REQ-BOOT-007) | `web/.nvmrc` (file present); BOOT-001 `web.yml` workflow |
| Scripts | `dev`, `build`, `start`, `lint`, `typecheck`, `format` | `web/package.json` |

**Implication for SPEC-UI-001**: the scaffold is committed, components
directory is **empty**, and the only existing page is a placeholder
"coming soon" home (`web/src/app/page.tsx`, 10 LOC). SPEC-UI-001 is
the **first SPEC to add real pages and components** under `web/` —
confirmed by SPEC-BOOT-001 §2 Out-of-Scope row "Any UI pages beyond
the default Next.js scaffold — SPEC-UI-001" and §10 Dependencies row
"SPEC-UI-001 | Builds first real pages and components under `web/`".

### 2.1 What is NOT in the scaffold (and is therefore TBD or in-scope)

The BOOT-001 scaffold does NOT pre-select any of the following, so
SPEC-UI-001 must either pick or defer:

| Decision | Status | Recommendation source |
|----------|--------|------------------------|
| Data-fetching client (SWR, TanStack Query, plain `fetch`, `EventSource`) | _TBD_ — pick during SPEC review | Native `EventSource` for SSE is the W3C standard; TanStack Query v5 is the Next.js 16 / React 19 idiomatic choice for non-streaming queries |
| Client-state manager (Zustand, Jotai, React Context, none) | _TBD_ — pick during SPEC review | Universal Search V1 UI has small client state (query text, advanced filters open/closed, history list); React Context + `useReducer` is likely sufficient |
| Form library (react-hook-form, native form actions, zod) | _TBD_ | Zod 3.x is consistent with the SPEC-SYN-004 wire schema enforcement story; react-hook-form is the Next.js 16 default for client-side forms |
| Routing (App Router pages, route groups) | App Router is fixed by BOOT-001 | Page layout is in-scope for SPEC-UI-001 |
| Internationalization (next-intl, lingui, custom) | _TBD_ — Korean-first display is a HARD requirement, but the V1 surface is small enough that route-based locale (`/ko/...`) or per-component language detection is sufficient | Per `.moai/project/product.md` §3 Korean analyst persona |
| Theme system (dark mode, brand tokens) | shadcn/ui supports CSS variables; brand tokens are _TBD_ pending `/moai design` path B | `.moai/project/brand/visual-identity.md` is fully _TBD_ at research time |
| Testing library | Vitest + React Testing Library + Playwright (recommended) | Aligns with BOOT-001 test toolchain — none currently in `web/package.json` |
| Bundle analyzer / perf budgets | _TBD_ | Next.js 16 built-in bundle analyzer suffices for V1 |

### 2.2 Stack confirmation summary (for the TBD list)

The frontend STACK BASELINE is **confirmed** (Next.js 16, React 19,
TypeScript 5+ strict, Tailwind 3.4, shadcn/ui ready, pnpm, Node 22).
The STACK ADDITIONS (data fetching, state mgmt, form, i18n, theme,
test) are **TBD** in `spec.md` and must be answered during the
annotation cycle before Run phase.

---

## 3. Upstream contracts the UI consumes

### 3.1 SSE streaming synthesis (SPEC-SYN-004)

SPEC-SYN-004 is **implemented** (status: implemented per its frontmatter)
and defines the wire surface UI-001 consumes for the streaming
synthesis path. The relevant guarantees for the UI:

- **Endpoint**: HTTP synthesis endpoint on `cmd/usearch-api`. Path is
  owned by SPEC-IR-001; SPEC-SYN-004 §5 declares the handler location
  is `cmd/usearch-api/handlers/synthesis.go` or equivalent. For
  SPEC-UI-001, the API path is treated as a single env var (e.g.
  `NEXT_PUBLIC_USEARCH_API_BASE`) that resolves to the
  `cmd/usearch-api` server root.
- **Content negotiation**: the client SHALL send
  `Accept: text/event-stream` to request the streaming surface.
  Omitting the header (or sending `application/json` / `text/html`)
  triggers the JSON fallback path per REQ-SYN4-005.
- **Response headers on the SSE path**: `Content-Type: text/event-stream`,
  `Cache-Control: no-cache`, `Connection: keep-alive` per
  REQ-SYN4-001a.
- **Event types** (W3C SSE wire format per REQ-SYN4-001b):
  - `event: sentence` — payload
    `{request_id, sentence_index, text, citations: [{marker, doc_id, url, title}], schema_version: 1}`
  - `event: done` — payload
    `{request_id, total_sentences, latency_ms, model, provider, cost_usd, schema_version: 1}`
  - `event: error` — payload
    `{request_id, error_code, error_message, partial_sentences_emitted, schema_version: 1}`
- **Heartbeat**: server emits `: ping\n\n` SSE comments every
  `SYN004_SSE_HEARTBEAT_MS` (default 15000 ms) per REQ-SYN4-003. The
  W3C EventSource client treats SSE comments as no-ops automatically,
  so the UI does NOT need to render heartbeats — but the UI **does**
  need to keep its visible loading indicator alive between sentence
  events (the heartbeat is the server's promise that the connection
  is live).
- **Disconnect semantics**: closing the EventSource (e.g.
  `eventSource.close()` on tab close, Cancel button) propagates upstream
  via `r.Context().Done()` and cancels the LLM call within
  `SYN004_DISCONNECT_CANCEL_MS` (default 1000 ms) per REQ-SYN4-004.
  This means the UI's Cancel button is **cheap** (no client-side
  workaround needed) — `eventSource.close()` suffices.
- **Stream terminator**: every successful stream ends with exactly one
  `event: done`; failed streams end with exactly one `event: error`
  per NFR-SYN4-002 (e), (f). No `event: sentence` follows a terminator.
- **Citation invariant (SPEC-SYN-002, preserved by SPEC-SYN-004
  REQ-SYN4-001c)**: every emitted `event: sentence`'s `text` field
  contains at least one `[N]` marker, and every `[N]` resolves to a
  `doc_id` in that event's `citations` array. The UI can therefore
  trust that every sentence it renders has at least one citation
  badge to draw.

### 3.2 Buffered JSON fallback (SPEC-SYN-001 contract preserved by SPEC-SYN-004 REQ-SYN4-005)

When the UI does NOT send `Accept: text/event-stream` (or when SSE is
disabled by configuration), the same endpoint returns
`Content-Type: application/json` with the full `Result` shape:

- `text` — full synthesized paragraph with `[N]` markers
- `citations` — array of `{marker, doc_id, url, title}`
- `latency_ms, model, provider, cost_usd` — observability fields

This fallback is used by the existing CLI (`cmd/usearch`) per
SPEC-SYN-004 §2.1(a). SPEC-UI-001 v1 standardizes on the SSE path; the
JSON path is the documented fallback for the no-SSE configuration mode
(see EARS REQ-UI-007 in `spec.md`).

### 3.3 Result / source list (pre-synthesis fanout output)

The synthesis endpoint covered above returns the **synthesized
paragraph + citations**. The roadmap M7 SPEC-UI-001 row also
specifies "results" as a UI concern, distinct from synthesis.

At the time of this research, the underlying source-list HTTP surface
(the fanout result before synthesis) is not yet exposed by a SPEC. The
existing SPECs cover:

- SPEC-FAN-001 (M3, implemented) — Go-side `internal/fanout/` produces
  a `[]NormalizedDoc` from parallel adapter calls. Not HTTP-surfaced.
- SPEC-CORE-001 — declares `NormalizedDoc` shape in `pkg/types/`:
  `{id, source, url, title, snippet, published_at, score, citations: [], adapter_meta: {}}`.
- SPEC-IR-001 — owns `cmd/usearch-api/` HTTP server scaffolding;
  details of which JSON shape it exposes for the source list are owned
  by IR-001, not yet finalized.

**Implication for SPEC-UI-001**: the UI's "Source detail" and "Source
cards" must consume **the `Citation` shape** emitted inside
`event: sentence` payloads (which carries `marker, doc_id, url, title`
only — no snippet, no score, no published_at). For richer source
cards (snippet, published_at, source-engine badge, score), the UI
needs the underlying `NormalizedDoc`. This is a **gap** —
SPEC-UI-001 records it as Open Question OQ-1 and recommends one of:

- (a) The synthesis endpoint extends `event: done` payload to include
  the full `[]NormalizedDoc` used as synthesis input. This is the
  smallest change and gives the UI everything it needs in one stream.
- (b) The UI makes a second HTTP call to a `/sources?request_id=...`
  endpoint exposed by SPEC-IR-001 after `event: done`.
- (c) The UI renders **citation-only** source cards in V1 (marker,
  doc_id, url, title) and defers rich source cards to V1.1.

SPEC-UI-001 defaults to option (c) for V1 to keep scope bounded; (a) or
(b) is a follow-up SPEC (likely SPEC-UI-001-v2 or a new SPEC-API-001).

---

## 4. Pages and surface — derived from product/roadmap

Per `.moai/project/roadmap.md` line 96 — "SPEC-UI-001 | Web UI v1 |
Next.js 16 app, query + streaming citation UI, team dashboard,
shadcn/ui | expert-frontend". Per `.moai/project/product.md` §4 —
multi-surface (CLI, MCP, Skill, Web UI) is in V1 scope.

User-task-input mapping for the V1 surface:

| User task | Page route (proposed) | V1 inclusion |
|-----------|------------------------|--------------|
| Submit a query | `/` (home + query input combined) | YES |
| See streaming synthesis + cited sources | `/q/[request_id]` OR `/` after submit (same view) | YES |
| Inspect a single source | `/source/[doc_id]` modal or page | YES (citation-data only per OQ-1) |
| Browse my recent queries | `/history` | YES (local-session only — see §6) |
| Browse team-shared queries | `/team` or `/history?scope=team` | NO — deferred (depends on SPEC-AUTH-001 + SPEC-IDX-005) |
| Manage adapter status, API keys | `/admin/*` | NO — SPEC-UI-002 (M7 second SPEC) |
| Sign in | `/login` or auth callback | NO — deferred to SPEC-AUTH-001 (M6) |
| Use `/deep` long-form research | `/q/[request_id]?mode=deep` toggle | YES (UI affordance only; backend is SPEC-DEEP-* in M5) |

**Login deferred to AUTH-001 (per instruction).** SPEC-UI-001 assumes
an anonymous, single-user, single-team baseline. The history list is
stored in browser-local state (localStorage or IndexedDB) and is
re-synced to the user account once SPEC-AUTH-001 lands. No
authentication code paths in SPEC-UI-001.

---

## 5. Citation interaction pattern — research

The synthesized paragraph contains inline `[N]` markers. Industry
patterns for citation interaction, mapped to the SPEC-SYN-002 citation
shape (`marker, doc_id, url, title`):

| Pattern | Examples | Pros for Universal Search | Cons |
|---------|----------|---------------------------|------|
| Inline numbered link, hover popover | Perplexity, Anthropic Citations | Standard mental model; popover can show full citation w/o navigation | Mobile has no hover — needs tap-to-open variant |
| Footnote-style numbered link, scrolls to references list | Wikipedia, STORM reports | Familiar, accessible, prints well | Two-step interaction; loses context |
| Inline pill / chip with source favicon | ChatGPT Search | Visually rich; brand-of-source visible | Requires favicon fetch per source; layout heavier |
| Side-panel reveal on citation click | Bing Chat, Notion AI | Lots of room for full context | Hides the answer behind interaction; mobile-hostile |

**Recommendation for V1**: inline numbered link `[N]` rendered as a
button with the citation index visible. Behavior:

- **Hover** (pointer-capable devices): reveal a popover with `title`,
  `url`, and "Open source" link.
- **Click / tap**: open the source in a new tab using `url`. Optionally
  scroll the source detail panel into view on the same page.
- **Keyboard focus**: same popover content via `aria-describedby` /
  focus-visible styling.
- **Mobile / touch**: tap reveals the popover (no hover); second tap
  on the popover's "Open" link navigates.

A11y notes:

- Each `[N]` button must have an `aria-label` like "Citation 3 of 7;
  source: <title>" to be self-describing without sight.
- The popover must use `role="tooltip"` or be wrapped in a
  Radix-Popover-style accessible primitive (shadcn/ui provides
  `Popover` and `HoverCard` based on Radix UI).

---

## 6. Korean-first display — research

Per `.moai/project/product.md` §3, the "Korean analyst / journalist"
persona expects Korean-first display when the query is Korean. Per
roadmap M3 exit criterion: "Korean query returns Naver results ranked
first." The UI must **not undermine** this by:

- Showing Korean text in a font that lacks the full Hangul + CJK
  glyph set.
- Truncating Korean source titles mid-character (Hangul composes via
  combining jamo; CSS `text-overflow: ellipsis` with a Latin font
  may break).
- Rendering Korean labels in a hard-coded English string.

V1 approach (recommended in `spec.md`):

- **Locale detection**: detect query language at submit time using
  the Unicode block of the first 50 non-whitespace characters. If
  Hangul range `U+AC00–U+D7A3` represents ≥ 30% of characters,
  classify as Korean (`ko`); otherwise English (`en`). The
  classification is **per-query**, not per-session — a user may
  switch languages between queries.
- **Display direction**: the rendering surface (page chrome) stays in
  one locale per session (default browser language); the **query
  results** display source titles and snippets in the source
  language verbatim. The synthesized paragraph is rendered in the
  language the LLM produces (per SPEC-SYN-001 the synthesis language
  follows the query language — assumed; this is not yet a HARD SPEC
  requirement and is recorded as OQ-2).
- **Font stack**: include a Korean-capable fallback in the Tailwind
  `font-sans` stack — for example
  `Inter, "Noto Sans KR", "Apple SD Gothic Neo", sans-serif`. Exact
  primary font is _TBD_ pending `/moai design` path B brand interview.

i18n library selection (TBD):

- The UI chrome (button labels, page titles, empty states, error
  messages) is small in V1 (~50 strings). Three viable options:
  - Native `next-intl` with `[locale]` route segment.
  - Lightweight `dictionary` pattern (TypeScript object keyed by
    locale, no runtime library).
  - Defer i18n entirely (English-only chrome) and rely on per-query
    display logic above.
- V1 recommendation: **dictionary pattern** (no library, English +
  Korean chrome strings inlined). Full `next-intl` integration is a
  V1.1 candidate when more locales appear.

---

## 7. SSE client patterns — research

The W3C `EventSource` API is widely supported by all evergreen
browsers (Chromium, Firefox, Safari) and handles auto-reconnect,
parsing, and the `Last-Event-ID` header automatically. Two patterns
the UI must handle:

### 7.1 EventSource is the default, with caveats

Native `EventSource` has two known limitations relevant to SPEC-UI-001:

- **No custom request headers** — `EventSource` cannot send
  `Authorization` or other custom headers. This is fine for V1
  (anonymous baseline) but will require a switch to `fetch` +
  `ReadableStream.getReader()` (manual SSE parsing) when SPEC-AUTH-001
  lands and the UI must send a JWT.
- **No request body** — `EventSource` is GET-only. Universal Search
  query is a long string with optional advanced filters; if the
  composed URL exceeds ~2 KB, browsers and intermediate proxies start
  to choke. V1 mitigation: cap query length at 1024 chars per
  `spec.md` REQ-UI-002, and serialize advanced filters as URL params
  (already short for V1's filter set).

### 7.2 Recovery pattern: client-side reconnection

`EventSource` automatically reconnects on transport error but does NOT
resume the stream cursor (SPEC-SYN-004 §2.2 explicitly excludes
`Last-Event-ID` resume — each disconnect produces a fresh synthesis
call). For SPEC-UI-001 v1, the UI shows the user a
"Connection lost — Retry?" affordance after a single auto-reconnect
attempt fails, rather than retrying indefinitely (which would re-charge
the LLM cost per attempt).

### 7.3 Citation marker resolution during streaming

Per SPEC-SYN-004 each `event: sentence` payload carries its own
`citations` array. The UI accumulates sentences in order
(`sentence_index` is monotonic) and renders each with its own citation
badges as it arrives. The full citation list is a **union** of all
arriving citations, deduplicated by `doc_id`. The UI's "All sources"
panel shows this deduplicated list in marker order
(first-occurrence-wins).

---

## 8. A11y baseline — research

V1 target: **WCAG 2.1 AA**. Areas of focus, ranked by user-visible
impact:

| Area | V1 requirement | Why |
|------|---------------|-----|
| Keyboard navigation | All interactive elements reachable via Tab; visible focus indicator | Streaming UI is dense with citation chips — keyboard users must reach each one |
| Screen reader live region | Streaming sentences announced via `aria-live="polite"` | Without this, sighted users see the stream but SR users only hear the final paragraph (or nothing) |
| Color contrast | Body text ≥ 4.5:1, large text ≥ 3:1, focus ring ≥ 3:1 against adjacent colors | shadcn/ui slate default palette meets this; brand tokens TBD must preserve it |
| Citation popover | `role="tooltip"` or Radix `HoverCard`; ESC closes; focus trap not needed because popover is non-modal | Radix primitives (shadcn/ui base) provide this out-of-box |
| Form labels | Every input has a `<label>` or `aria-label`; advanced filters use `aria-expanded` for the disclosure | Standard practice |
| Error states | Error messages are programmatically associated via `aria-describedby` | Empty + error states must be perceivable by SR users |
| Reduced motion | `prefers-reduced-motion: reduce` disables streaming animation transitions | Sentence-by-sentence reveal can trigger motion sensitivity if animated |
| Language attribute | `<html lang="...">` matches the per-query detected language for the results region; chrome stays in browser default | Screen readers switch voice based on `lang` |

Testing approach in V1: manual Lighthouse run + axe-core dev-tools
extension. Automated a11y CI (e.g. `@axe-core/playwright`) is
recommended but not required for V1; recorded as a V1.1 candidate.

---

## 9. Responsive baseline — research

Mobile-first, breakpoint set (Tailwind defaults):

| Breakpoint | Tailwind | Surface adaptation |
|-----------|----------|--------------------|
| < 640 px (sm) | base | Single-column; query input full-width; source cards stacked; popovers become full-screen sheets |
| ≥ 640 px | `sm:` | Two-column option for source list + answer |
| ≥ 768 px (md) | `md:` | Source list as a right-rail (33% width); answer 67% |
| ≥ 1024 px (lg) | `lg:` | Wider answer column; persistent left-rail for history (when history view active) |
| ≥ 1280 px (xl) | `xl:` | Max content width capped at 1280 px to preserve line length for readability |

Interaction adaptations on touch:

- Citation popover: tap-to-open (not hover-to-open).
- "Open source" CTA inside popover is the primary touch target (44×44
  px min per WCAG 2.5.5).
- Advanced filters: bottom sheet on mobile, popover on desktop.

---

## 10. Performance — research, aligned with SPEC-SYN-004 NFR-SYN4-001

SPEC-SYN-004 NFR-SYN4-001 declares:

> In buffered-then-streamed mode (v0), TTFB to first `event: sentence`
> byte ≤ synthesis end-to-end latency + 50 ms; total stream wall-clock
> within 100 ms of the JSON path.

Translated to UI-visible metrics:

| Metric | Target | Source |
|--------|--------|--------|
| Time-to-first-sentence visible in DOM | ≤ (server TTFB to first event) + 100 ms client overhead | Client overhead = SSE parse + React render of first sentence |
| Total streaming completion (end-of-stream to "done" state visible) | ≤ (server total) + 200 ms client overhead | Render of last sentence + done badge |
| First contentful paint of home page | ≤ 1.5 s on a fast 3G simulated network | Next.js 16 SSR + RSC default |
| Largest contentful paint (results page after stream completes) | ≤ 2.5 s | LCP element = synthesized paragraph |
| Cumulative layout shift while streaming | < 0.1 | Streaming reveal must reserve vertical space to avoid CLS |
| Total JavaScript shipped to client (home + query page) | _TBD_ — recommended ≤ 200 KB gzipped | Next.js 16 RSC reduces client JS; shadcn/ui is tree-shakable |

The aggressive p50 ≤ 8 s synthesis latency from SPEC-SYN-001
NFR-SYN-001 dominates user perception; the UI's job is to **not add
material overhead** and to **mask the latency with progressive
disclosure** (heartbeat → first sentence → subsequent sentences →
done). Skeleton loaders during the pre-first-sentence window are
critical.

---

## 11. Brand context — research

`.moai/project/brand/brand-voice.md`, `target-audience.md`, and
`visual-identity.md` are all in the `_TBD_` template state — the brand
interview (via `/moai design`) has not yet been run for this project.
Per the design constitution at
`.claude/rules/moai/design/constitution.md` §3.1, brand context is a
constitutional parent — when populated, it constrains every UI
decision (color, typography, voice).

V1 strategy for SPEC-UI-001:

- **Do NOT block** on the brand interview. SPEC-UI-001 ships with
  shadcn/ui defaults (slate baseColor, Inter font, system-typical
  spacing) and CSS variables that can be swapped wholesale by a brand
  token sheet later.
- **Reference, do not duplicate**, the `/moai design` path B pipeline.
  The brand-design pipeline (`moai-domain-copywriting`,
  `moai-domain-brand-design`, `evaluator-active` GAN loop) is the
  authoritative source of brand tokens. SPEC-UI-001 declares the
  *integration point* (CSS variables in `web/src/app/globals.css`
  driven by `tokens.json` from the design pipeline) but does NOT
  author tokens itself.
- **Microcopy in V1**: short, direct, English-first. Korean strings
  for the chrome are added via the dictionary pattern (§6) when the
  brand interview supplies the Korean voice.

OQ-3 records this dependency.

---

## 12. Top external references (verify in Run phase via WebFetch per anti-hallucination policy)

| Reference | Purpose for SPEC-UI-001 |
|-----------|--------------------------|
| W3C "Server-Sent Events" living standard | SSE wire format the UI consumes |
| MDN `EventSource` API reference | Browser API the UI's streaming client uses by default |
| Next.js 16 App Router documentation (vercel/next.js) | Pages, layouts, route groups, streaming responses |
| React 19 documentation (facebook/react) | `use` hook, Server Components, Actions, Suspense for streaming |
| shadcn/ui component documentation | `Popover`, `HoverCard`, `Sheet`, `Dialog`, `Skeleton`, `Toast`, `Command` |
| Radix UI primitives (underpins shadcn/ui) | A11y semantics for popovers, tooltips, dialogs |
| Tailwind CSS responsive design + accessibility documentation | Breakpoint set, focus-visible, prefers-reduced-motion |
| WCAG 2.1 AA quick reference (w3.org) | A11y baseline checklist |
| Perplexity, ChatGPT Search, Anthropic Citations (product references) | Citation interaction pattern survey |
| TanStack Query v5 (if selected for non-streaming queries) | Data fetching client library |

---

## 13. Open Questions (for annotation cycle)

| ID | Question | Default if unanswered | Where it surfaces in spec.md |
|----|----------|------------------------|-------------------------------|
| OQ-1 | Does the synthesis endpoint extend `event: done` payload with full `[]NormalizedDoc`, OR does the UI fetch sources separately, OR does V1 render citation-only source cards? | Citation-only source cards (V1, option c) | EARS REQ-UI-005 source detail scope; §"Exclusions" lists rich-source-card features as out-of-V1 |
| OQ-2 | Is "synthesis language matches query language" a SPEC-SYN-001 HARD requirement, or an LLM-policy convention? | Convention only; UI does not enforce | Affects REQ-UI-008 Korean-first display assumptions |
| OQ-3 | When does `/moai design` path B run to populate brand tokens? | After SPEC-UI-001 ships; UI v1 uses shadcn/ui slate defaults; brand tokens swap in via CSS variables in V1.1 | Affects `spec.md` §"Exclusions" — brand visual identity is out-of-V1 |
| OQ-4 | Data fetching library: TanStack Query, SWR, plain `fetch`? | Plain `fetch` + native `EventSource` for V1; TanStack Query is a recommended addition for non-streaming endpoints in V1.1 if needed | Marked _TBD_ in spec.md §Technical Approach |
| OQ-5 | State management: React Context + reducer, Zustand, Jotai? | React Context + `useReducer` for V1 (small state surface) | Marked _TBD_ in spec.md §Technical Approach |
| OQ-6 | i18n library: next-intl, lingui, dictionary pattern? | Dictionary pattern (TypeScript object) for V1 | Affects REQ-UI-008 Korean chrome scope |
| OQ-7 | Where does history persist? localStorage, IndexedDB, server (after AUTH-001)? | localStorage in V1 (single-device, anonymous); migration path to server-side recorded for AUTH-001 | Affects REQ-UI-009 history page scope |
| OQ-8 | Bundle size / perf budget enforcement: hard CI gate or advisory? | Advisory in V1; hard gate in V1.1 once we have a baseline measurement | Affects NFR-UI-002 in spec.md |
| OQ-9 | Is the `/deep` mode toggle in-scope for SPEC-UI-001 or deferred? | In-scope as a UI toggle (no backend wiring beyond passing a flag to the existing synthesis endpoint); the backend `/deep` pipeline is M5 SPEC-DEEP-* | Affects REQ-UI-003 query input scope |

---

## 14. Risk register

| Severity | Risk | Mitigation |
|----------|------|------------|
| High | Streaming UX feels janky if React re-renders the full paragraph on every `event: sentence` (worst case: O(n²) DOM work for a 20-sentence paragraph). | Render each sentence as an independent keyed `<span>`; append-only state updates; `React.memo` on the sentence component. |
| High | Citation popover stacks above other UI on mobile and traps the user (no visible close affordance). | Use shadcn `Popover` (Radix), which provides ESC + outside-click dismissal and visible close button. |
| Medium | EventSource auto-reconnects on transient network error, re-charging an `/deep` LLM call. | After 1 auto-reconnect attempt fails, surface "Retry?" affordance instead of letting the browser loop. Per §7.2 above. |
| Medium | Korean text mis-renders due to font fallback gaps. | Tailwind font-sans stack includes Korean fallback (§6); manual visual test on a Korean query at SPEC review. |
| Medium | Brand tokens land mid-V1 cycle and require a UI re-paint. | UI v1 uses CSS variables driven by shadcn/ui defaults; brand tokens override the variable definitions without touching component code. |
| Medium | The synthesis endpoint URL is owned by SPEC-IR-001 which is not yet implemented at SPEC-UI-001 SPEC-time. | SPEC-UI-001 declares the URL behind an env var (`NEXT_PUBLIC_USEARCH_API_BASE`); implementation can land against a mock server until SPEC-IR-001 ships. |
| Low | Advanced filters URL serialization exceeds 2 KB (EventSource GET limit). | Cap query length at 1024 chars (REQ-UI-002); advanced filters are a small enum set for V1. |
| Low | Citation popover Z-index conflicts with the page header. | Radix Portal renders popovers at document root; Z-index conflicts are rare. |

---

## 15. References (internal)

- `.moai/project/product.md` — V1 scope, personas, success metrics
- `.moai/project/roadmap.md` line 96 — SPEC-UI-001 row
- `.moai/project/tech.md` §3 Surfaces — Next.js 16 + shadcn/ui + Tailwind
- `.moai/project/brand/{brand-voice,target-audience,visual-identity}.md` — _TBD_ at research time
- `.claude/rules/moai/design/constitution.md` — design system constitutional rules (Section 3.1 Brand Context)
- `.moai/specs/SPEC-BOOT-001/spec.md` — frontend scaffold (REQ-BOOT-003)
- `.moai/specs/SPEC-SYN-004/spec.md` — SSE wire contract (REQ-SYN4-001a/b/c through 006; NFR-SYN4-001/002/003)
- `.moai/specs/SPEC-SYN-002/spec.md` — citation faithfulness invariant
- `.moai/specs/SPEC-SYN-001/spec.md` — `Result` JSON shape (preserved by SYN-004 fallback)
- `.moai/specs/SPEC-IR-001/` — `cmd/usearch-api` HTTP server scaffolding (prerequisite for runtime, not for SPEC drafting)
- `.moai/specs/SPEC-AUTH-001/spec.md` — login deferral target (M6)
- `web/package.json`, `web/components.json`, `web/tsconfig.json`,
  `web/tailwind.config.ts`, `web/eslint.config.mjs`,
  `web/src/app/{layout,page}.tsx` — confirmed frontend stack

---

*End of SPEC-UI-001 research.md (draft v0.1).*
