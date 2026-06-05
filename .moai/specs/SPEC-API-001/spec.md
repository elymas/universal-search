---
id: SPEC-API-001
title: usearch-api HTTP Server v0
version: 0.2.0
status: draft
created: 2026-06-04
updated: 2026-06-04
author: limbowl
owner: expert-backend
methodology: tdd
priority: P0
issue_number: null
depends_on: [SPEC-IR-001, SPEC-FAN-001, SPEC-SYN-004, SPEC-CORE-001]
---

# SPEC-API-001: usearch-api HTTP Server v0

## HISTORY

- 2026-06-04 (v0.2.0): Applied plan-auditor APPROVE-WITH-FIXES (3 MAJOR + 1 MINOR).
  M-1: rewrote REQ-API-009 (and REQ-API-006/008) — streamsynth emits only
  `sentence`/`done`/`error`; the `citation` event is server-DERIVED from embedded
  `CitationRef`, with an explicit field-mapping table (`marker`→`index`, `doc_id`→
  `NormalizedDoc.SourceID`/`Snippet` lookup for `source`/`snippet`). M-2: rewrote
  REQ-API-011 + added REQ-API-011a — `/api/sources` derives `name`/`category` from
  `Capabilities()` (which has no `category`/`enabled` fields) and reports
  `enabled: true` for all registered adapters in v0. M-3: added acceptance
  Scenarios 12 (REQ-API-013 deadline/request-ID) and 13 (REQ-API-014 observability).
  MINOR: converted the 5 NFRs to EARS form.
- 2026-06-04 (v0.1.0): Initial draft. Owns the HTTP API server that the Next.js frontend
  consumes. Corrects the stale "SPEC-IR-001" ownership reference embedded in
  `cmd/usearch-api/main.go` (SPEC-IR-001 is library-only — see research.md §1).
  Scope locked by user to the "search path first" (Decision Point D1): start the
  server, wire the production search pipeline, implement the search endpoints the
  frontend calls, plus graceful shutdown and healthcheck. Admin, audit, and deep
  research are explicitly deferred.

## Overview

`cmd/usearch-api/main.go` is currently a non-functional stub. It builds an
`http.ServeMux`, initializes observability, registers a `/query/stream` handler
with a **nil** synthesis client and an empty admin registry, then prints
`usearch-api: not implemented (see SPEC-IR-001)` and calls `os.Exit(0)` — it
**never calls `ListenAndServe`**. As a result the Next.js frontend
(`web/src/lib/api.ts`) has no backend: every fetch fails. The only working search
path today is the CLI (`cmd/usearch/query.go`).

This SPEC makes the server real for the search path. The production search
pipeline already exists and is proven inside the CLI `Execute` function
(Classify → Dispatch → Synthesize). SPEC-API-001 reuses that exact pipeline —
without divergence — behind the HTTP endpoints the frontend already calls.

## Goal

Stand up a working `usearch-api` HTTP server that serves the frontend's search
contract by reusing the CLI's proven search pipeline as a single source of truth.

## Glossary

- **Search pipeline**: the Classify → Dispatch → Synthesize flow that produces an
  answer with citations from a free-text query.
- **Production wiring**: the registry/router/fanout/synthesis assembly currently
  living in `cmd/usearch/query.go` (`buildProductionRegistry`, `buildRouter`,
  `buildProductionSynth`).
- **Frontend contract**: the routes, query params, and JSON/SSE shapes defined in
  `web/src/lib/api.ts` and `web/src/lib/sse-client.ts`.

## Environment

- Go 1.24 (per `deploy/Dockerfile.usearch-api`), Go 1.23+ language baseline.
- Existing libraries reused as-is: `internal/adapters`, `internal/router`,
  `internal/fanout`, `internal/synthesis`, `internal/streamsynth`, `internal/sse`,
  `internal/obs`, `pkg/types`.
- Server reads existing env vars: `LOG_LEVEL`, `OTLP_ENDPOINT`,
  `USEARCH_ADMIN_PORT`, plus all adapter/synthesis env vars already consumed by
  the CLI build helpers (`RESEARCHER_BASE_URL`, `USEARCH_SEARXNG_URL`,
  `USEARCH_GITHUB_TOKEN`, etc.). A new `USEARCH_API_PORT` (default `8080`) selects
  the public listen address.

## Assumptions

1. The CLI build helpers (`buildProductionRegistry`, `buildRouter`,
   `buildProductionSynth`) are the canonical pipeline assembly and must NOT be
   duplicated. Because they live in `package main` under `cmd/usearch/`, they
   cannot be imported by `cmd/usearch-api/` — they must be extracted into a shared
   `internal/` package (see plan.md §Refactor; recorded as Decision Point D2).
2. `synthesis.Client` and the obs bundle are nil-safe (REQ-SYN-006), so a degraded
   server (no researcher sidecar) still serves results without crashing.
3. The frontend's `/api/...` route prefix is canonical. The stub's bare
   `/query/stream` route is wrong and is replaced (Decision Point D3).
4. `/api/history` has no backing store in scope; returning an empty list with HTTP
   200 satisfies the frontend (Decision Point D4).

## Requirements (EARS)

### Server lifecycle

- **REQ-API-001** (Event-Driven): **When** the `usearch-api` process starts
  without the `--healthcheck` flag, the system **shall** build the production
  search pipeline (registry, router, fanout dispatcher, synthesis client) using
  the shared extracted assembly and **shall** call `ListenAndServe` on the address
  derived from `USEARCH_API_PORT` (default `:8080`).

- **REQ-API-002** (Ubiquitous): The system **shall** reuse the exact CLI search
  pipeline assembly (Classify → Dispatch → Synthesize) as the single source of
  truth, sharing one extracted `internal/` package between `cmd/usearch` and
  `cmd/usearch-api` so the two entry points cannot diverge.

- **REQ-API-003** (Event-Driven): **When** the process receives `SIGINT` or
  `SIGTERM`, the system **shall** stop accepting new connections, drain in-flight
  requests within a bounded shutdown timeout, run the observability shutdown hook,
  and exit `0`.

- **REQ-API-004** (Event-Driven): **When** the process is invoked with the
  `--healthcheck` flag, the system **shall** probe its own health surface and exit
  `0` if healthy or non-zero otherwise, satisfying the Dockerfile `HEALTHCHECK`
  directive without starting the full server.

- **REQ-API-005** (Event-Driven): **When** a client requests the health route
  (e.g. `GET /healthz`), the system **shall** respond `200 OK` with a minimal JSON
  body once the server is ready to serve search traffic.

### Search endpoints (frontend contract)

- **REQ-API-006** (Event-Driven): **When** a client sends `GET /api/query?q=...`
  (optional `&sources=a,b`), the system **shall** run the search pipeline and
  respond with a JSON `SearchResult` of shape
  `{answer, citations[], query, sources_used[], elapsed_ms}`, where each citation
  is `{index, title, url, snippet, source}` derived using the same field mapping as
  REQ-API-009 (`index`←`Citation.marker`, `title`/`url` from the synthesis citation,
  `source`←`NormalizedDoc.SourceID` and `snippet`←`NormalizedDoc.Snippet` resolved by
  `doc_id`). `answer` is `synthesis.Result.Text`; `elapsed_ms` is the measured
  pipeline wall-clock; `sources_used` is the effective dispatched adapter set.

- **REQ-API-007** (State-Driven): **While** a `sources` query parameter is
  present, the system **shall** restrict the dispatched adapter set to the
  intersection of the router decision and the named adapters, and **shall** respond
  `400` if a named adapter is not registered (mirroring the CLI `REQ-CLI-003`
  unknown-adapter behavior).

- **REQ-API-008** (Event-Driven): **When** a client sends
  `GET /api/query/stream?q=...` (optional `&sources=a,b`), the system **shall**
  stream synthesis over SSE driven by `internal/streamsynth`, exposing to the client
  the four event names the frontend listens for — `sentence`, `citation`,
  `complete`, `error` — via the translation defined in REQ-API-009 (streamsynth
  itself emits only `sentence`/`done`/`error`; `citation` is server-derived and
  `complete` is the renamed `done`). Each event carries a JSON payload.

- **REQ-API-009** (Ubiquitous): The system **shall** translate the streamsynth wire
  protocol (which emits only `sentence` / `done` / `error`, with citations embedded
  inside each sentence payload as `streamsynth.CitationRef{marker, doc_id, url,
  title}` — there is NO standalone citation event on the wire) into the four event
  names the frontend's `onSentence`/`onCitation`/`onComplete`/`onError` listeners
  register for (`web/src/lib/sse-client.ts`), as follows:
  - **`sentence` → `sentence`**: forward `{text, citations[]}` (the sentence text
    plus its embedded `CitationRef[]`). The frontend `onSentence` reads `parsed.text`.
  - **derived `citation`**: because no source emits a standalone `citation` event but
    the frontend DOES register an `onCitation` listener, the server **shall**
    synthesize one `citation` event per distinct `CitationRef` carried by sentence
    payloads, emitted immediately after the owning `sentence` event. The frontend
    `Citation` shape `{index, title, url, snippet, source}` is derived as:

    | Frontend `Citation` field | Derived from | Provenance / mapping |
    |---|---|---|
    | `index` | `CitationRef.marker` | the inline `[N]` marker number (note: `marker`, NOT a zero-based array index) |
    | `title` | `CitationRef.title` | from the synthesis citation |
    | `url` | `CitationRef.url` | from the synthesis citation |
    | `source` | resolved via `CitationRef.doc_id` | the `SourceID` of the `pkg/types.NormalizedDoc` whose `ID == doc_id` (the adapter that produced the doc) |
    | `snippet` | resolved via `CitationRef.doc_id` | the `Snippet` field of that same matching `NormalizedDoc` |

    `CitationRef` carries no `snippet` and no `source`; both MUST be resolved by
    doc-id lookup. The streaming handler **shall** retain the fanout-dispatched
    `[]NormalizedDoc` (indexed by `ID`) for the duration of the stream and look each
    `doc_id` up against it. **If** a `doc_id` has no matching doc, **then** `source`
    and `snippet` **shall** be empty strings (never an error, never a dropped event).
  - **`done` → `complete`**: re-emit under the event name `complete` (the frontend
    has NO `done` listener — leaving it as `done` means completion never fires) with
    payload `{elapsed_ms}` sourced from `streamsynth.DonePayload.latency_ms`.
  - **`error` → `error`**: re-emit with payload `{message}` sourced from
    `streamsynth.ErrorPayload.error_message` (the frontend `onError` reads
    `parsed.message`).

- **REQ-API-010** (Unwanted): **If** the upstream synthesis client is unavailable
  or returns an error, **then** the system **shall** emit an SSE `error` event
  (streaming path) or a degraded JSON response (buffered path) rather than dropping
  the connection or returning an unhandled `500`, preserving the CLI's degraded-mode
  behavior (`REQ-CLI-009`).

- **REQ-API-011** (Event-Driven): **When** a client sends `GET /api/sources`, the
  system **shall** respond with a JSON array, one element per registered adapter,
  built ONLY from non-secret-bearing registry data (`registry.List()` + each
  adapter's `Capabilities()` from `pkg/types`). The admin snapshot
  (`SnapshotForAdmin`) MUST NOT be used here — it is out of scope and secret-aware.
  `Capabilities` has no `category` and no `enabled` field, so each element is derived
  as:

    | Frontend `AdapterInfo` field | Derived from | Provenance / mapping |
    |---|---|---|
    | `name` | `Adapter.Name()` (== `Capabilities.SourceID`) | the stable adapter identifier; `DisplayName` MAY be used for a human label if the frontend later needs it |
    | `category` | `Capabilities.DocTypes` | the adapter's primary `DocType` rendered as its string (e.g. first/dominant DocType); the router's category mapping is NOT required here since `/api/sources` describes adapters, not a query |
    | `latency_ms` | omitted in v0 | optional in the frontend type; the registry exposes no per-adapter latency without the admin/telemetry surface, so it is left unset |

  - **REQ-API-011a** (Decision — `enabled`): because no non-admin "enabled" signal
    exists (registration is binary: a registered adapter is, by definition, active;
    the `enabled`/disabled distinction lives only in the out-of-scope admin
    snapshot), every element returned by `/api/sources` **shall** report
    `enabled: true` for v0. A real enabled/disabled toggle is deferred to the admin
    SPEC (see Out of Scope). This keeps the field present for frontend compatibility
    without leaking admin state.

- **REQ-API-012** (Event-Driven): **When** a client sends `GET /api/history`, the
  system **shall** respond `200 OK` with an empty JSON array (no history store is
  in scope for v0).

### Cross-cutting

- **REQ-API-013** (Ubiquitous): The system **shall** propagate a per-request
  context with a bounded pipeline deadline through Classify, Dispatch, and
  Synthesize, mirroring the CLI timeout handling, and **shall** attach a request ID
  for tracing.

- **REQ-API-014** (Ubiquitous): The system **shall** preserve the already-wired
  observability (OTel spans, Prometheus admin surface via `obs.Init`) for every
  served request without requiring new observability infrastructure.

- **REQ-API-015** (Ubiquitous): The system **shall** correct the stale source
  reference in `cmd/usearch-api/main.go` — the package comment "Full
  implementation lands in SPEC-IR-001" and the stderr string
  "usearch-api: not implemented (see SPEC-IR-001)" **shall** be replaced to
  reference `SPEC-API-001` (and the obsolete stderr string removed once the server
  runs).

- **REQ-API-016** (Event-Driven): **When** the project's container stack is
  brought up, the system **shall** be deployable as a `usearch-api` service in
  `deploy/docker-compose.yml` exposing port `8080`, depending on the `researcher`
  and adapter sidecars (e.g. `searxng`) it needs, with the relevant env wired.

- **REQ-API-017** (Ubiquitous): The system **shall** be covered by
  `httptest`-based handler tests that inject fake registry/router/synthesis
  doubles, with no live network access required in CI.

## Out of Scope (Exclusions — What NOT to Build)

- **`/api/admin/*` endpoints** (adapter listing, toggle, resync). Handlers already
  exist (`internal/api/admin`) behind loopback middleware; they remain wired but
  are NOT part of this SPEC's acceptance. Deferred to a follow-up admin SPEC.
- **`AuditQuerier` implementation.** The audit handler is constructed with `nil`
  today; implementing a real `AuditQuerier` (and the audit store `QueryEntries`
  method) is deferred.
- **`/deep` deep-research endpoint.** `cmd/usearch-api/handlers/deep.go` exists but
  is unregistered with nil deps; deep research stays deferred.
- **Real `/api/history` persistence.** v0 returns an empty list only (REQ-API-012);
  a history store is out of scope.
- **gRPC exposure.** HTTP/JSON + SSE only.
- **Authentication / multi-tenant access control** on the public search endpoints.
  Loopback restriction applies only to the (out-of-scope) admin group.
- **MCP exposure** (owned by SPEC-MCP-001).

## Non-Functional Requirements

- **NFR-API-001 (Single source of truth, Ubiquitous):** The system **shall** drive
  both the HTTP search path and the CLI search path from one shared pipeline-assembly
  package; any duplicated assembly is a defect. (Enforced by REQ-API-002.)
- **NFR-API-002 (Context propagation, Event-Driven):** **When** a search request is
  served, the system **shall** carry a deadline-bound context through every pipeline
  stage; **and when** the client disconnects, the system **shall** cancel the
  in-flight pipeline (the SSE path honors this via streamsynth `ctx.Done()`).
- **NFR-API-003 (Observability continuity, Unwanted):** The system **shall not**
  serve any request path that bypasses OTel span creation; OTel and the Prometheus
  surface **shall** remain wired through `obs.Init` for every served request.
- **NFR-API-004 (Degraded resilience, Unwanted):** **If** the researcher sidecar is
  unavailable, **then** the system **shall** return a degraded answer and **shall not**
  crash (relying on the nil-safe synthesis client).
- **NFR-API-005 (No secret leakage, Unwanted):** The system **shall not** expose any
  secret value via `/api/sources`; it **shall** expose only non-secret adapter
  metadata (consistent with the admin snapshot's no-secret invariant).

## Decision Points

- **D1 (Scope — user-locked):** "Search path first." Server start + production
  pipeline wiring + frontend search endpoints + graceful shutdown + healthcheck.
  Admin/audit/deep/history-persistence deferred.
- **D2 (Refactor):** extract CLI `build*` helpers into a shared `internal/` package
  (cmd packages cannot import each other). See plan.md.
- **D3 (Route prefix):** adopt the frontend's `/api/...` prefix as canonical;
  replace the stub's bare `/query/stream`.
- **D4 (`/api/history`):** return an empty list (200) — no store in v0.

## Traceability

| Requirement | Frontend contract | Reused component |
|-------------|-------------------|------------------|
| REQ-API-006/007 | `GET /api/query` (`api.ts` searchQuery) | router.Classify, fanout.Dispatch, synthesis.Synthesize |
| REQ-API-008/009/010 | `GET /api/query/stream` (`api.ts` searchStream, `sse-client.ts`) | streamsynth.StreamSynthesize, handlers.SynthesisHandler |
| REQ-API-011/011a | `GET /api/sources` (`api.ts` fetchSources) | registry.List() + Capabilities() (NOT SnapshotForAdmin) |
| REQ-API-012 | `GET /api/history` (`api.ts` fetchHistory) | — (empty list) |
| REQ-API-004/005 | Dockerfile `HEALTHCHECK --healthcheck` | new health route + flag |
| REQ-API-013/014 | — (cross-cutting; Scenarios 12, 13) | reqid, context deadline, obs.Init spans |
| REQ-API-016 | container deploy | docker-compose researcher/searxng deps |
