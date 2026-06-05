# SPEC-API-001 Acceptance Criteria

All scenarios use `httptest` with injected fakes (fake registry, fake router,
fake synthesis client). No live network in CI (REQ-API-017).

## Scenario 1: Server starts and listens (REQ-API-001, REQ-API-003)

- **Given** the `usearch-api` binary built from the new `main()`,
- **When** it is started without `--healthcheck`,
- **Then** it binds the address from `USEARCH_API_PORT` (default `:8080`) and
  serves requests (does not `os.Exit(0)` immediately),
- **And When** it receives `SIGTERM`,
- **Then** it stops accepting new connections, drains in-flight requests, runs the
  obs shutdown hook, and exits `0`.

## Scenario 2: Buffered query returns the frontend SearchResult shape (REQ-API-006)

- **Given** a server wired with a fake pipeline returning one synthesized answer
  and two citations,
- **When** a client sends `GET /api/query?q=rust%20async`,
- **Then** the response is `200` with `Content-Type: application/json` and a body
  matching `{answer, citations[], query, sources_used[], elapsed_ms}`,
- **And** each citation has exactly `{index, title, url, snippet, source}`,
- **And** `query` echoes `"rust async"` and `sources_used` lists the dispatched
  adapters.

## Scenario 3: Source filter intersection and unknown adapter (REQ-API-007)

- **Given** a registry containing `reddit` and `hn`,
- **When** a client sends `GET /api/query?q=x&sources=reddit`,
- **Then** only `reddit` is dispatched and `sources_used` contains only `reddit`,
- **And When** a client sends `GET /api/query?q=x&sources=ghost`,
- **Then** the response is `400` with a message naming the unknown adapter.

## Scenario 4: SSE streaming with frontend event names (REQ-API-008, REQ-API-009)

- **Given** a server wired with a fake synthesis client returning multi-sentence,
  cited text whose `Citation.marker` values reference docs in the dispatched
  `[]NormalizedDoc` set,
- **When** a client sends `GET /api/query/stream?q=x` with
  `Accept: text/event-stream`,
- **Then** the response sets SSE headers and emits, in order, one or more
  `event: sentence` frames (payload `{text, citations[]}`), each immediately
  followed by one `event: citation` frame per distinct `CitationRef`, then exactly
  one `event: complete` frame (payload includes `elapsed_ms`),
- **And** the streamsynth `done` event is surfaced as `complete` (NOT `done` — a
  raw `done` would never fire the frontend's completion listener),
- **And** each derived `citation` payload is `{index, title, url, snippet, source}`
  where `index == CitationRef.marker`, `title`/`url` come from the citation, and
  `source`/`snippet` are resolved from the matching `NormalizedDoc` by `doc_id`
  (`source == NormalizedDoc.SourceID`, `snippet == NormalizedDoc.Snippet`),
- **And When** a `CitationRef.doc_id` matches no dispatched doc,
- **Then** the `citation` frame is still emitted with `source` and `snippet` as
  empty strings (no dropped event, no error),
- **And** no uncited sentence is emitted (SYN-002 invariant preserved).

## Scenario 5: Degraded synthesis does not break the connection (REQ-API-010)

- **Given** a synthesis client that returns an unavailable/error result,
- **When** a client streams `GET /api/query/stream?q=x`,
- **Then** the server emits an `event: error` frame with a JSON `{message}` payload
  rather than closing abruptly or returning an unhandled `500`,
- **And When** the same failure occurs on `GET /api/query` (buffered),
- **Then** the response is a degraded JSON body (answer empty/notice set), not a
  bare `500` crash.

## Scenario 6: Sources listing (REQ-API-011, REQ-API-011a, NFR-API-005)

- **Given** a registry with three registered adapters (each with known
  `Capabilities().SourceID` and `DocTypes`),
- **When** a client sends `GET /api/sources`,
- **Then** the response is `200` with a JSON array of three elements, each
  `{name, category, enabled}` where `name == Adapter.Name()`, `category` is derived
  from the adapter's `Capabilities().DocTypes`, and `enabled == true` (v0 reports
  every registered adapter as enabled — REQ-API-011a),
- **And** `latency_ms` is absent (not emitted in v0),
- **And** the response is built from `registry.List()` + `Capabilities()`, NOT from
  `SnapshotForAdmin()`,
- **And** no element contains any secret value, env-var name, or `secret_source`/
  `key_set`-style admin field.

## Scenario 7: History is an empty list in v0 (REQ-API-012)

- **Given** the running server,
- **When** a client sends `GET /api/history`,
- **Then** the response is `200` with body `[]`.

## Scenario 8: Healthcheck flag and route (REQ-API-004, REQ-API-005)

- **Given** the binary,
- **When** it is invoked as `usearch-api --healthcheck` against a healthy server,
- **Then** it exits `0` (and non-zero when unhealthy) without starting the full
  server,
- **And When** a client sends `GET /healthz` to a ready server,
- **Then** the response is `200` with a minimal JSON body.

## Scenario 9: Stale SPEC reference corrected (REQ-API-015)

- **Given** the new `cmd/usearch-api/main.go`,
- **When** its source is inspected,
- **Then** the package comment no longer claims "Full implementation lands in
  SPEC-IR-001" and instead references `SPEC-API-001`,
- **And** the `usearch-api: not implemented (see SPEC-IR-001)` stderr string is
  removed (the server now runs).

## Scenario 10: Single source of truth preserved (REQ-API-002, NFR-API-001)

- **Given** the M0 refactor extracting the pipeline into a shared `internal/`
  package,
- **When** the existing CLI test suite runs,
- **Then** it passes unchanged,
- **And** both `cmd/usearch` and `cmd/usearch-api` import the same extracted
  assembly (verified by absence of duplicated `buildProductionRegistry`/
  `buildRouter`/`buildProductionSynth` in `cmd/usearch-api`).

## Scenario 11: Container deployment (REQ-API-016)

- **Given** `deploy/docker-compose.yml` with the new `usearch-api` service,
- **When** the stack is brought up,
- **Then** `usearch-api` starts, exposes `8080`, depends on `researcher` and
  `searxng` being healthy, and its `HEALTHCHECK --healthcheck` reports healthy.

## Scenario 12: Per-request deadline and request ID propagation (REQ-API-013, NFR-API-002)

- **Given** a server wired with a fake router/fanout/synthesis that records the
  `context.Context` it receives,
- **When** a client sends `GET /api/query?q=x`,
- **Then** the same deadline-bound context (non-zero deadline set) is observed at
  the Classify, Dispatch, and Synthesize call sites (deadline propagated, not reset
  per stage),
- **And** a non-empty request ID is attached to the request context and is visible
  to the pipeline stages (e.g. via the `reqid` helper used by the CLI),
- **And When** a fake stage blocks past the deadline,
- **Then** the request is cancelled (the handler observes `ctx.Err()`) rather than
  hanging indefinitely.

## Scenario 13: Observability is wired on every served path (REQ-API-014, NFR-API-003)

- **Given** a server started with an in-test OTel span recorder (exporter) via the
  `obs` bundle,
- **When** a client sends `GET /api/query?q=x` and separately
  `GET /api/query/stream?q=x`,
- **Then** at least one span is recorded for each request path (no served search
  path bypasses span creation),
- **And** the server reuses the existing `obs.Init` wiring — the test introduces no
  new observability infrastructure beyond the in-test recorder,
- **And** the Prometheus admin surface remains reachable when `USEARCH_ADMIN_PORT`
  is configured (unchanged from the stub's already-wired behavior).

## Edge Cases

- Empty/whitespace `q` → `400` (mirror CLI `REQ-CLI-007`).
- All adapters fail with zero docs → degraded/empty answer, not a crash.
- Client disconnects mid-SSE → pipeline context is cancelled (streamsynth honors
  `ctx.Done()`).
- `sources` param with duplicate names → deduplicated before dispatch.

## Quality Gate / Definition of Done

- [ ] All 13 scenarios have passing `httptest` tests; no live network in CI.
- [ ] CLI suite remains green after the M0 extraction (characterization gate).
- [ ] `go vet`, `golangci-lint`, `go test -race ./...` clean for touched packages.
- [ ] Coverage >= 85% on new handler/server code (TRUST 5 Tested).
- [ ] No duplicated pipeline-assembly code between the two `cmd/` mains.
- [ ] Frontend `web/src/lib/api.ts` calls succeed against the running server for
      `/api/query`, `/api/query/stream`, `/api/sources`, `/api/history`.
- [ ] Stale `SPEC-IR-001` references corrected (Scenario 9).
- [ ] `@MX` anchors added on the shared `Build*` boundary and the SSE GET handler.
