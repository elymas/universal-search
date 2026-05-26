---
id: SPEC-MCP-001
version: 1.0.0
status: implemented
created: 2026-05-22
updated: 2026-05-26
author: limbowl
priority: P0
issue_number: 0
title: MCP Server — external host integration (Claude Code / Codex / Gemini CLI)
milestone: M7 — Surfaces
owner: expert-backend
methodology: tdd
coverage_target: 85
depends_on: [SPEC-BOOT-001, SPEC-OBS-001, SPEC-LLM-001, SPEC-IR-001, SPEC-FAN-001, SPEC-SYN-001, SPEC-SYN-004, SPEC-DEEP-001, SPEC-DEEP-002, SPEC-DEEP-003, SPEC-DEEP-004, SPEC-IDX-001, SPEC-CORE-001]
blocks: [SPEC-SKILL-001]
related: [SPEC-AUTH-001, SPEC-AUTH-002, SPEC-AUTH-003]
---

# SPEC-MCP-001: MCP Server — external host integration

## HISTORY

- 2026-05-22 (initial draft v0.1.0, limbowl via manager-spec):
  First EARS-formatted SPEC for the M7 milestone. SPEC-MCP-001 replaces
  the SPEC-BOOT-001 stub at `cmd/usearch-mcp/main.go` (currently prints
  `"usearch-mcp: not implemented (see SPEC-MCP-001)"` and exits 0) with a
  full Model Context Protocol server exposing four tools (`search`,
  `deep_research`, `list_sources`, `get_citation`) over two transports
  (stdio default, Streamable HTTP opt-in) per the 2025-06-18 MCP spec
  revision. The server is the canonical programmatic surface of Universal
  Search per `.moai/project/tech.md` §1 principle 7; CLI and Claude Skill
  (SPEC-SKILL-001) become thin wrappers over it.

  Pinned decisions:
  (D1) Library: `github.com/modelcontextprotocol/go-sdk` v1.x primary;
       `github.com/mark3labs/mcp-go` reserved fallback. See research §2.
  (D2) Default transport: stdio. Streamable HTTP is opt-in via
       `--transport http` flag + config. See research §3.
  (D3) HTTP transport HARD-requires Origin validation + localhost-only
       default bind + auth-required default mode (delegates to
       SPEC-AUTH-001 when M6 ships). See research §3.2 + §6.2.
  (D4) Tool surface for V1: `search`, `deep_research`, `list_sources`,
       `get_citation`. Team-memory tools deferred to V1.1 (research §10
       Q5).
  (D5) `deep_research` tool MUST route through the SAME
       `costguard.CapCheckMiddleware` as `cmd/usearch-api/handlers/
       deep.go` so quota counters are shared. See research §6.3.
  (D6) Error mapping: usearch errors → JSON-RPC `-32000..-32099`
       server-defined range with `usearch.*` namespace in `data.namespace`
       field. See research §7.
  (D7) Streaming: `deep_research` MUST stream progress via
       `notifications/message` (stdio) or SSE events (HTTP). `search`
       streaming OPTIONAL — implemented when transport is HTTP, single
       response on stdio.

  M7 release gate per `.moai/project/roadmap.md` §5 M9 exit criterion:
  "MCP server connects from Claude Code + Codex + Gemini CLI". SPEC-MCP-001
  ships the server side of that criterion; SPEC-SKILL-001 ships the Claude
  Skill marketplace package on top of it.

  Companion artifacts:
  - `.moai/specs/SPEC-MCP-001/research.md` — Phase 0.5 research (11
    sections: protocol summary, Go SDK survey, transport tradeoffs,
    tool surface design, integration with internals, auth handoff to
    AUTH-001, error mapping, storage/config, 10 risks, 7 open questions,
    references)
  - `.moai/specs/SPEC-MCP-001/plan.md` — phased implementation plan

  16 EARS REQs (13 × P0 + 3 × P1), 8 NFRs, 5 modules (Server Lifecycle /
  Transports / Tools / Auth & Observability / Error Mapping). Methodology:
  TDD, coverage target 85%, harness: standard. Owner: expert-backend.

---

## 1. Overview

SPEC-MCP-001 builds the Model Context Protocol server that exposes
Universal Search's query and synthesis capabilities to external LLM host
applications (Claude Code, Claude Desktop, Codex CLI, Gemini CLI). The
MCP server is the canonical programmatic surface per `.moai/project/
tech.md` §1 principle 7: every other client (CLI, Skill, Web UI) is a
thin wrapper over the same internal pipeline that the MCP server exposes.

The server packages four tools over two transports:

1. **Tools** (research §4.1):
   - `search` — basic-mode synthesis (fanout → synthesis → cited paragraph).
   - `deep_research` — `/deep` multi-agent pipeline.
   - `list_sources` — adapter registry enumeration.
   - `get_citation` — resolve a `doc_id` to its `NormalizedDoc`.

2. **Transports** (research §3):
   - **stdio** (default) — for local CLI clients launching the server as
     a subprocess. Zero auth, OS-process-boundary trust.
   - **Streamable HTTP** (opt-in) — for remote team deployment with
     JWT auth delegated to SPEC-AUTH-001.

### 1.1 Pinned Architectural Decisions

The 7 decisions in HISTORY's "Pinned decisions" block are restated here
for in-document reference and treated as constraints in the EARS
requirements below. They are not re-litigated.

### 1.2 Motivation

Without an MCP server, Universal Search cannot be reached from the LLM
host applications that the V1 personas use day-to-day:

- The "Research-heavy engineer" persona (`.moai/project/product.md` §3)
  wants `usearch` callable from inside Claude Code conversations, not as
  a separate terminal command.
- The M9 exit criterion gates V1.0.0 on "MCP server connects from Claude
  Code + Codex + Gemini CLI".
- SPEC-SKILL-001 (Claude Skill marketplace package) is a thin wrapper
  over the MCP server and is BLOCKED until this SPEC ships.
- Web UI (SPEC-UI-001), Admin UI (SPEC-UI-002), and a future SDK-style
  package all benefit from the MCP server being the single contract
  point for "how does an external caller invoke a Universal Search
  query?".

### 1.3 Forward-compatibility commitment with AUTH-001 / AUTH-002 / AUTH-003

This SPEC commits to consuming — never re-implementing — the
authentication primitives that SPEC-AUTH-001, SPEC-AUTH-002, and
SPEC-AUTH-003 (all M6) ship. Concretely:

- The HTTP transport's JWT validation MUST use the middleware exported by
  SPEC-AUTH-001 (e.g., `auth.NewJWTMiddleware(cfg)`); no parallel JWT
  parsing in `internal/mcpserver/`.
- Per-tool RBAC (when SPEC-AUTH-002 ships) layers ON TOP of the existing
  `costguard.CapCheckMiddleware` invocation; no replacement.
- Tool-call audit log entries MUST conform to the SPEC-DEEP-004
  REQ-DEEP4-010 decision-event JSON line schema so that SPEC-AUTH-003's
  audit subsystem can consume them additively.

V1 of this SPEC ships an `--auth-mode=trust-headers` operator escape
hatch for HTTP transport deployments that pre-date AUTH-001 GA; the
operator promises an upstream proxy validates the headers. This is
identical to the SPEC-DEEP-004 §1.1 D1 / REQ-DEEP4-001 `X-User-Id`
trust model. M6 GA of AUTH-001 flips the default to `--auth-mode=jwt`.

---

## 2. EARS Requirements

### 2.1 Server Lifecycle Module

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-MCP-001** | Ubiquitous | The `usearch-mcp` binary SHALL replace the SPEC-BOOT-001 stub body in `cmd/usearch-mcp/main.go` with a functional Model Context Protocol server. The binary SHALL preserve the existing `obs.Init` lifecycle (admin port, slog, OTel) and the `--version` / `-v` flag semantics established by sibling binaries `cmd/usearch` and `cmd/usearch-api`. | P0 | Binary starts, completes `initialize` handshake from a reference MCP client, responds to `tools/list`, gracefully shuts down on SIGINT. |
| **REQ-MCP-002** | Ubiquitous | The MCP server SHALL implement protocol version `2025-06-18` of the Model Context Protocol as the primary negotiated version. The server SHALL declare its supported version in the `initialize` response and SHALL reject `initialize` requests carrying an unsupported version with a JSON-RPC error mapped per §2.5. | P0 | Reference client `initialize` with `2025-06-18` succeeds; `initialize` with `2024-01-01` (synthetic unsupported) is rejected with the documented error. |
| **REQ-MCP-003** | Event-Driven | WHEN the MCP server receives `SIGINT` or `SIGTERM`, the server SHALL stop accepting new requests, allow in-flight tool calls up to `mcp.shutdown.grace_period_seconds` (default 30) to complete, cancel any remaining contexts, and exit with code 0. The shutdown SHALL be observable via a single `usearch.mcp.shutdown` slog record carrying `reason`, `inflight_at_signal`, and `duration_ms` fields. | P0 | Signal-driven shutdown test asserts grace-period behaviour, context cancellation propagation, and shutdown log emission. |

### 2.2 Transports Module

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-MCP-004** | Ubiquitous | The MCP server SHALL implement the stdio transport per MCP 2025-06-18 §Transports: read newline-delimited JSON-RPC frames from stdin, write JSON-RPC responses to stdout, reserve stderr for slog records and operator messages, and SHALL NOT write any non-MCP byte to stdout. The stdio transport SHALL be the DEFAULT transport when `--transport` is unspecified. | P0 | Reference stdio client (subprocess pattern) completes `initialize` → `tools/list` → `tools/call search` round-trip. Stdout byte audit confirms no non-MCP content. |
| **REQ-MCP-005** | Optional | WHERE the operator passes `--transport http` or sets `mcp.transport: http` in config, the MCP server SHALL implement the Streamable HTTP transport per MCP 2025-06-18 §Transports: expose a single endpoint path (default `/mcp`) that accepts POST (JSON-RPC requests) and GET (server-initiated SSE stream), parse and emit `Mcp-Session-Id` headers for session continuity, honour the `MCP-Protocol-Version` header, and support both `application/json` and `text/event-stream` response content types based on the request `Accept` header. | P0 | HTTP transport test asserts: POST `initialize` returns `Mcp-Session-Id` header; subsequent POST without the header is rejected per spec; GET with `Accept: text/event-stream` opens an SSE stream; `MCP-Protocol-Version` echoed and validated. |
| **REQ-MCP-006** | Unwanted | IF the HTTP transport is enabled, the server SHALL reject any POST or GET request whose `Origin` header is not in the configured `mcp.http.allowed_origins` list with HTTP `403 Forbidden`. The server SHALL default to binding `127.0.0.1` and SHALL refuse to bind a non-loopback address unless BOTH `mcp.http.bind_public: true` AND `mcp.http.auth_mode` ∈ {`jwt`, `trust-headers`} are set. Startup with `bind_public: true` AND `auth_mode: none` SHALL fail with a non-zero exit code and an error message naming the violated invariant. | P0 | Origin-rejection test, default-bind test, bind-public + auth-none startup-refusal test all green. |

### 2.3 Tools Module

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-MCP-007** | Ubiquitous | The MCP server SHALL register the following four tools and SHALL respond to `tools/list` with all four when enabled (subject to `mcp.tools.enabled` filter): `search`, `deep_research`, `list_sources`, `get_citation`. Each tool SHALL declare a typed `inputSchema` AND `outputSchema` (JSON Schema Draft 2020-12) and the schemas SHALL be deterministic across server restarts (same input → same generated schema bytes). | P0 | `tools/list` response shape matches the golden fixture stored at `internal/mcpserver/testdata/tools_list.golden.json`. |
| **REQ-MCP-008** | Event-Driven | WHEN a client calls `tools/call` with tool name `search` and a valid `SearchInput`, the MCP server SHALL invoke the same orchestration pipeline used by `cmd/usearch/query.go::Execute` (router → fanout → synthesis) and SHALL return a `SearchOutput` containing `summary`, `citations[]` with `doc_id` references, and `stats` (request_id, latency_ms, source_count). The `search` tool handler SHALL NOT duplicate the orchestration logic; the run phase MUST extract or share an orchestrator helper. | P0 | Integration test with stub adapters asserts response shape; coverage test confirms the MCP `search` handler invokes the same `internal/` packages as the CLI path (no parallel pipeline). |
| **REQ-MCP-009** | Event-Driven | WHEN a client calls `tools/call` with tool name `deep_research` and a valid `DeepResearchInput`, the MCP server SHALL invoke `internal/deepagent.RunPipeline` (or its successor function) AND SHALL route the call through the same `costguard.CapCheckMiddleware` invoked by `cmd/usearch-api/handlers/deep.go`. Quota counters MUST be shared with the HTTP `/deep` surface: a tenant capped via HTTP-`/deep` MUST also be capped via MCP-`deep_research` within the same 24h sliding window, and vice versa. | P0 | Quota-sharing integration test: cap a tenant via HTTP `/deep`, immediately call `deep_research` via MCP from the same tenant, assert MCP returns the documented `usearch.cap_exceeded` error (§2.5) without invoking the deepagent pipeline. |
| **REQ-MCP-010** | State-Driven | WHILE a `deep_research` tool call is in progress, the MCP server SHALL emit progress events to the client at minimum at each pipeline-stage boundary (Researcher → Reviewer → Writer → Verifier per SPEC-DEEP-002). On stdio transport, progress events SHALL be sent as JSON-RPC `notifications/message` with the originating request ID in the notification payload. On Streamable HTTP transport, progress events SHALL be sent as SSE events on the response stream alongside the eventual final `tools/call` response. | P0 | Test asserts `notifications/message` count ≥ 4 (one per pipeline stage) across both transports; SSE event ordering test confirms final response is the last event. |
| **REQ-MCP-011** | Event-Driven | WHEN a client calls `tools/call` with tool name `list_sources`, the MCP server SHALL return the registered adapter set from `internal/adapters.Registry.List()` with each entry's name, category, language support, auth-required flag, and human-readable description. The response SHALL be sorted by adapter name to ensure determinism. | P1 | Test asserts shape and sort order against fixture. |
| **REQ-MCP-012** | Event-Driven | WHEN a client calls `tools/call` with tool name `get_citation` and a valid `doc_id` previously returned by `search` or `deep_research`, the MCP server SHALL resolve the citation to the full `NormalizedDoc` (title, URL, snippet, source, score, fetched_at) and return it. IF the `doc_id` is unknown to the server (expired cache or never issued), the server SHALL return the documented `usearch.citation_not_found` error per §2.5. | P1 | Resolution test + not-found test. |

### 2.4 Auth & Observability Module

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-MCP-013** | State-Driven | WHILE the HTTP transport is enabled AND `mcp.http.auth_mode: jwt` is set, every JSON-RPC request other than `initialize` SHALL pass through the JWT validation middleware exported by SPEC-AUTH-001 (`auth.NewJWTMiddleware` or its successor). On validation success, the validated `user_id` and `tenant_id` SHALL be injected into the request context under the keys `costguard.UserIDKey` and `auth.ClaimsKey` (forward-compat with SPEC-DEEP-004 §6.3). On validation failure, the server SHALL return the documented `usearch.unauthorized` error per §2.5. _TBD until SPEC-AUTH-001 ships: the exact middleware constructor name and import path; the SPEC commits to using whatever AUTH-001 exports without re-implementing JWT logic._ | P0 | When AUTH-001 stub is present, integration test asserts a request with an expired JWT returns `usearch.unauthorized`. Pre-AUTH-001, the SPEC ships with `--auth-mode=trust-headers` as the only HTTP auth path. |
| **REQ-MCP-014** | Ubiquitous | For each `tools/call` invocation, the MCP server SHALL open a root OTel span named `mcp.tool.{tool_name}` carrying the attributes `mcp.transport`, `mcp.protocol_version`, `mcp.client_name`, `mcp.client_version`, `mcp.session_id` (only when HTTP transport AND session ID is from a bounded source per NFR-MCP-005), `mcp.tool_name`, `mcp.outcome`. The span SHALL be the parent of any downstream router / fanout / synthesis / deepagent spans. The span SHALL be ended exactly once before the JSON-RPC response is written. | P0 | OTel exporter test asserts span attribute set, parent-child relationship, single-end invariant. |
| **REQ-MCP-015** | Ubiquitous | For each `tools/call` invocation, the MCP server SHALL emit a single decision-event JSON line to stderr (slog) with fields conforming to the SPEC-DEEP-004 REQ-DEEP4-010 decision-event schema, extended with `event_type: "mcp.tool_call"`, `tool_name`, `mcp_transport`, and `client_name`. The schema SHALL be additive only: SPEC-AUTH-003 (M6) audit subsystem SHALL consume these lines without rename or removal of existing fields. | P0 | Schema-conformance test against a JSON Schema fixture co-located in `internal/mcpserver/testdata/`. |

### 2.5 Error Mapping Module

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-MCP-016** | Ubiquitous | The MCP server SHALL map Universal Search internal errors to JSON-RPC error objects per the table below. Every server-defined error code SHALL be in the JSON-RPC reserved range `-32099 .. -32000` and SHALL carry a `data.namespace` field whose value starts with `"usearch."` so that clients can branch on error families without parsing free-text. Server-defined error codes and namespaces are listed in this requirement and are FROZEN for V1 (additive only — new codes may be added; existing codes MUST NOT be renumbered or renamed within V1.x). Mapping table: (a) cap exceeded → `-32000 usearch.cap_exceeded` carrying `dimension`, `remaining`, `reset_at`, `retry_after_s`; (b) deep-not-warranted (Haiku reject) → `-32001 usearch.deep_not_warranted` carrying `screen_score`, `suggested_mode`, `rationale`; (c) unauthorized → `-32002 usearch.unauthorized` carrying `reason`; (d) no adapters matched → `-32003 usearch.no_adapters_matched` carrying `query_category`; (e) all adapters failed → `-32004 usearch.all_adapters_failed` carrying `errors[]`; (f) synthesis degraded → `-32005 usearch.synthesis_degraded` carrying `degraded_reason` AND partial result in the response (NOT in the error); (g) timeout → `-32006 usearch.timeout` carrying `stage`, `deadline_ms`; (h) citation not found → `-32007 usearch.citation_not_found` carrying `doc_id`; (i) tool not enabled → `-32601 MethodNotFound`; (j) input schema violation → `-32602 InvalidParams` carrying `field`, `reason`. Internal panics SHALL surface as `-32603 InternalError` carrying only `request_id` (NO stack trace per SPEC-CLI-001 NFR-CLI-004 readability invariant). | P0 | Table-driven test asserts every documented error path returns the correct code + namespace + data fields. |

---

## 3. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| **NFR-MCP-001** | Cold-start latency (stdio) | The wall-clock time from `usearch-mcp` process spawn (stdio transport) to readiness for the first `initialize` request SHALL be ≤ 500ms p95 on a developer machine baseline (Apple Silicon M-series or equivalent). Measured by an in-process timer recorded as a startup metric and by a CI cold-start benchmark. |
| **NFR-MCP-002** | Streaming first-byte latency (HTTP) | When the HTTP transport handles a `deep_research` tool call, the first SSE event (typically the first `notifications/message` progress event) SHALL be emitted within 1000ms of receiving the POST request — independent of how long the underlying pipeline runs. Measured by SSE wire capture in integration tests. |
| **NFR-MCP-003** | Search latency parity with CLI | The MCP `search` tool's median end-to-end latency SHALL be within 15% of `cmd/usearch query` median latency for the same input, transport overhead aside. Measured by paired benchmark in CI. |
| **NFR-MCP-004** | Goroutine hygiene | Every JSON-RPC request handling path (including streaming `deep_research` with `notifications/message` emission) SHALL terminate all spawned goroutines within 200ms of the parent context being cancelled. Verified by `go.uber.org/goleak` in `TestMain` of `internal/mcpserver/*_test.go`. |
| **NFR-MCP-005** | No PII in metric labels | All Prometheus metric labels emitted by this SPEC SHALL be bounded enumerable sets and SHALL NOT include `user_id`, `session_id`, `query` text, `tenant_id` outside a deploy-time allowlist, `client_ip`, or `rationale` text. `mcp.session_id` is permitted ONLY in OTel span attributes (cardinality safe), NEVER in Prometheus labels. Verified by extending SPEC-OBS-001 `TestNoUnboundedLabels` allowlist. |
| **NFR-MCP-006** | Binary size | The release-mode `usearch-mcp` binary (`go build -ldflags "-s -w" -trimpath ./cmd/usearch-mcp`) SHALL be ≤ 40 MB on linux/amd64. CI gates this. The 10 MB delta above the SPEC-CLI-001 `usearch` 30 MB cap accommodates the MCP SDK + HTTP transport handlers. |
| **NFR-MCP-007** | DNS rebinding mitigation | The HTTP transport's Origin validation (REQ-MCP-006) SHALL reject ALL of the following at the integration-test boundary: missing `Origin` header (when configured to require it), `Origin: null`, `Origin: <not-in-allowlist>`, `Origin: <case-folded variant of allowlisted entry>`. Allowlist matching SHALL be case-sensitive and exact-match on scheme + host + port. |
| **NFR-MCP-008** | Backwards-compat stub for v1.x | All public types in `internal/mcpserver/` (handler signatures, config struct fields, error namespace strings) introduced in V1.0 SHALL remain stable across V1.x patch and minor releases. Removals require a major-version bump; additions (new tools, new error codes, new config fields with defaults) are SHALL be additive. |

---

## 4. Exclusions (What NOT to Build)

[HARD] This SPEC explicitly excludes the following items. Each has a known
destination SPEC or rationale; this list prevents scope creep into
MCP-001.

- **MCP `resources` feature surface** (addressable context blobs). → V1.1+
  consideration. Could expose recent team queries; depends on
  SPEC-AUTH-003 audit log + IDX-005 team-shared index.

- **MCP `prompts` feature surface** (templated user-facing flows). → V1.1+
  consideration. Curated query templates.

- **MCP `sampling` feature** (server-initiated LLM calls back through the
  client). → Out of scope for V1. Universal Search owns its own LLM
  router via SPEC-LLM-001; sampling would create a parallel routing
  path with unclear cost-attribution semantics.

- **Tag-based `sources` filter** in the `search` tool input (e.g.,
  `sources: ["social"]` expanding to all social adapters). → Adapter
  name allowlist only for V1, matching SPEC-CLI-001 §2.2. Future
  enhancement gated on user demand.

- **Backwards compatibility with the deprecated MCP HTTP+SSE transport**
  (protocol version 2024-11-05). → V1 supports only the 2025-06-18
  Streamable HTTP transport. Operators with clients pinned to the
  legacy transport must upgrade their clients. Reserved for a future
  SPEC if real-world need surfaces.

- **`search_team_memory` and `list_team_queries` tools**. → V1.1; depend
  on SPEC-IDX-005 + SPEC-AUTH-003.

- **Per-tool RBAC**. → SPEC-AUTH-002 (M6) layers Casbin policy on top of
  the cap enforcement this SPEC ships. V1 ships uniform tenant-level cap
  enforcement only.

- **OAuth flow inside the MCP server** for HTTP transport bootstrap. →
  V1 HTTP transport accepts an already-issued JWT (from the OIDC
  provider configured by SPEC-AUTH-001) in the `Authorization: Bearer`
  header. The OAuth dance is the client's responsibility, as is the
  norm for MCP HTTP servers in the ecosystem.

- **Per-MCP-tool quota** (e.g., quota X for `search`, quota Y for
  `deep_research` separately from the HTTP `/deep` quota). → V1 shares
  the SPEC-DEEP-004 cap; `search` is uncapped (matches CLI behaviour);
  `deep_research` shares the `/deep` cap. Per-tool quota considered if
  abuse patterns emerge.

- **Resource hot-reload of `mcp.tools.enabled`** without server restart. →
  V1 reads config at startup only. Hot-reload (consistent with
  SPEC-DEEP-004 NFR-DEEP4-008 deep.yaml hot-reload) reserved for a
  later iteration once the tool surface stabilises.

- **Stdio-transport authentication** beyond the OS process boundary. → By
  spec, stdio MCP servers inherit trust from the launching process.
  Adding a stdio auth layer would diverge from MCP convention and
  every other production stdio MCP server.

- **GitHub Issue tracking on this SPEC**. → Skipped per project pattern
  for M7 milestone gates (`issue_number: 0`).

- **Standalone Helm chart** for the MCP server. → Delegated to
  SPEC-DEPLOY-001 (M9) which owns the full deployment matrix.

- **Custom transport beyond stdio and Streamable HTTP**. → MCP allows
  custom transports; V1 implements only the two standard ones. WebSocket
  / named-pipe / Unix-socket transports reserved for future SPECs if
  measured value warrants.

---

## 5. Acceptance Criteria

Per-REQ acceptance summaries are documented in §2 alongside each
requirement. The full Given-When-Then scenarios are owned by
`.moai/specs/SPEC-MCP-001/acceptance.md` (to be authored alongside this
SPEC's plan-auditor cycle). The scenario index:

| Scenario | Description | Coverage |
|----------|-------------|----------|
| §5.1 | Local Claude Code launches `usearch-mcp` via stdio, completes `initialize` + `tools/list` + `tools/call search` round-trip. | REQ-MCP-001, 002, 004, 007, 008 |
| §5.2 | Codex CLI launches `usearch-mcp` via stdio, calls `deep_research`, receives ≥4 `notifications/message` progress events plus final response. | REQ-MCP-004, 009, 010 |
| §5.3 | Gemini CLI launches `usearch-mcp` via stdio, calls `list_sources` and `get_citation` round-trip. | REQ-MCP-004, 011, 012 |
| §5.4 | Operator starts `usearch-mcp --transport http --listen 127.0.0.1:7080` with `--auth-mode=trust-headers` (pre-AUTH-001), remote test client completes `initialize` over HTTP with `Mcp-Session-Id` round-trip and SSE stream. | REQ-MCP-005, 006 |
| §5.5 | Tenant capped via HTTP `/deep` immediately calls `deep_research` via MCP — receives `-32000 usearch.cap_exceeded` with shared quota counters. | REQ-MCP-009, 016 |
| §5.6 | HTTP transport rejects POST with off-allowlist `Origin` header → 403. | REQ-MCP-006, NFR-MCP-007 |
| §5.7 | Operator attempts `bind_public: true` + `auth_mode: none` startup → server refuses with non-zero exit and named-invariant error. | REQ-MCP-006 |
| §5.8 | `usearch-mcp` receives SIGINT mid-`deep_research` → grace period elapses, context cancelled, deepagent pipeline observed terminating, exit code 0. | REQ-MCP-003, NFR-MCP-004 |
| §5.9 | Tool input schema validation: `search` called with empty `query` returns `-32602 InvalidParams` carrying `field=query, reason=required`. | REQ-MCP-016 |
| §5.10 | `tools/list` golden fixture match across server restarts. | REQ-MCP-007 |

---

## 6. Dependencies & Blocks

### 6.1 Upstream SPEC dependencies (depends_on)

- **SPEC-BOOT-001** (implemented) — `cmd/usearch-mcp/main.go` stub being
  replaced; obs.Init lifecycle inherited.
- **SPEC-OBS-001** (implemented) — slog + OTel + admin port + Prometheus
  registry reused.
- **SPEC-LLM-001** (implemented) — `llm.Client` (transitive via
  synthesis / deepagent).
- **SPEC-IR-001** (implemented) — `router.Router.Classify` consumed by
  `search` tool.
- **SPEC-FAN-001** (implemented) — `fanout.Dispatch` consumed by `search`
  tool.
- **SPEC-SYN-001** (implemented) — synthesis client; `search` tool returns
  citations from this layer.
- **SPEC-SYN-004** (implemented) — SSE streaming pattern reused for HTTP
  transport progress events (`search` streaming, where applicable).
- **SPEC-DEEP-001 / 002 / 003** — `internal/deepagent.RunPipeline`
  consumed by `deep_research` tool; pipeline-stage boundaries source
  the progress events.
- **SPEC-DEEP-004** (implemented) — `costguard.CapCheckMiddleware`
  invoked by `deep_research` tool handler; decision-event JSON schema
  extended by REQ-MCP-015.
- **SPEC-IDX-001** (implemented) — `internal/index.Index.GetByDocID` (or
  equivalent) consumed by `get_citation` tool. _TBD: exact method name
  pending IDX-001 final API; see research §10 Q1._
- **SPEC-CORE-001** (implemented) — `pkg/types.NormalizedDoc` /
  `Citation` / `Query` typed contracts reused.

### 6.2 Related but soft (related)

- **SPEC-AUTH-001 (draft, M6)** — JWT validation middleware. HTTP
  transport's `auth-mode=jwt` HARD-requires this; until AUTH-001 ships,
  HTTP transport defaults to `--auth-mode=trust-headers` per REQ-MCP-013
  fallback clause. See research §6.2.
- **SPEC-AUTH-002 (draft, M6)** — per-tool RBAC. Layers on top after V1
  ship; no schema change needed in this SPEC.
- **SPEC-AUTH-003 (draft, M6)** — audit subsystem. Consumes REQ-MCP-015
  decision-event JSON lines additively.

### 6.3 Downstream blocked SPECs (blocks)

- **SPEC-SKILL-001 (M7)** — Claude Skill marketplace package wraps the
  MCP server; BLOCKED until SPEC-MCP-001 ships.

### 6.4 External dependencies (run-phase pins)

- `github.com/modelcontextprotocol/go-sdk` — pin exact version in run
  phase per research §2.3 policy.
- Indirect: `github.com/google/uuid` (already pinned), `golang.org/x/
  sync/errgroup` (already pinned), `go.opentelemetry.io/otel/*` (already
  pinned).

No new direct heavyweight dependency outside the MCP SDK and its
transitive footprint.

---

## 7. Files to Create / Modify

### 7.1 Created (estimated; final list owned by run phase)

| Marker | Path | Purpose |
|--------|------|---------|
| [NEW] | `internal/mcpserver/server.go` | Server lifecycle (`New`, `Start`, `Shutdown`); transport agnostic. |
| [NEW] | `internal/mcpserver/transport_stdio.go` | stdio transport adapter wiring SDK to obs / config. |
| [NEW] | `internal/mcpserver/transport_http.go` | Streamable HTTP transport adapter; Origin validation; bind policy. |
| [NEW] | `internal/mcpserver/tools/search.go` | `search` tool handler; delegates to shared orchestrator. |
| [NEW] | `internal/mcpserver/tools/deep_research.go` | `deep_research` tool handler; cap-guarded; emits progress. |
| [NEW] | `internal/mcpserver/tools/list_sources.go` | `list_sources` tool handler. |
| [NEW] | `internal/mcpserver/tools/get_citation.go` | `get_citation` tool handler. |
| [NEW] | `internal/mcpserver/tools/types.go` | `SearchInput`, `SearchOutput`, `DeepResearchInput`, `DeepResearchOutput`, `Citation`, `Stats` structs with jsonschema tags. |
| [NEW] | `internal/mcpserver/errors.go` | Error mapping (REQ-MCP-016 table) — central `MapError(err) *jsonrpc.Error`. |
| [NEW] | `internal/mcpserver/auth.go` | Auth-mode dispatcher: `jwt` (delegates to SPEC-AUTH-001), `trust-headers`, `none`. |
| [NEW] | `internal/mcpserver/audit.go` | Decision-event JSON line emitter (REQ-MCP-015) reusing DEEP-004 schema. |
| [NEW] | `internal/mcpserver/config.go` | Config struct + koanf loader for `mcp.*` keys. |
| [NEW] | `internal/mcpserver/server_test.go` | Lifecycle + transport tests with goleak. |
| [NEW] | `internal/mcpserver/tools/*_test.go` | Per-tool table-driven tests. |
| [NEW] | `internal/mcpserver/testdata/tools_list.golden.json` | REQ-MCP-007 deterministic schema fixture. |
| [NEW] | `internal/mcpserver/integration_test.go` | Build-tag `integration`; spawns binary, runs reference clients. |
| [NEW] | `internal/orchestrator/search.go` (or chosen path) | Extract shared `search` orchestration from `cmd/usearch/query.go` so CLI + MCP share one implementation. (Run phase decides exact factoring.) |
| [NEW] | `.moai/config/sections/mcp.yaml` | `mcp.*` config block defaults. |

### 7.2 Modified

| Path | Change |
|------|--------|
| `cmd/usearch-mcp/main.go` | Replace stub body with `mcpserver.New(cfg, obs).Start(ctx)`; preserve obs.Init contract and `--version` flag. |
| `cmd/usearch/query.go` | Refactor to call shared orchestrator helper (if §7.1 extraction is chosen). Behaviour unchanged. |
| `internal/fanout/fanout.go` | Update `@MX:ANCHOR @MX:REASON` to include MCP server as a caller (currently lists "future SPEC-MCP-001"). |
| `internal/index/index.go` | Update `@MX:ANCHOR @MX:REASON` to include MCP server as a caller (currently lists "MCP" but commentary points to MCP-001). |
| `internal/llm/llm.go` | Update `@MX:ANCHOR` caller list (already lists `cmd/usearch-mcp`). |
| `internal/obs/metrics/metrics_test.go` | Extend `TestNoUnboundedLabels` allowlist with `mcp.transport`, `mcp.tool_name`, `mcp.outcome` labels (NFR-MCP-005). |
| `internal/obs/obs.go` | Re-export new MCP-domain collectors. |
| `go.mod` / `go.sum` | Pin official MCP Go SDK version. |
| `.env.example` | Document new MCP env vars (`MCP_TRANSPORT`, `MCP_HTTP_LISTEN_ADDR`, `MCP_HTTP_AUTH_MODE`, `MCP_HTTP_ALLOWED_ORIGINS`). |

### 7.3 Existing — Unchanged

- `internal/synthesis/*` — read-only consumer for `search` and citation
  enrichment.
- `internal/deepagent/*` — read-only consumer for `deep_research`.
- `internal/adapters/*` — read-only consumer for `list_sources`.
- `internal/router/*` — read-only consumer for `search`.
- `internal/auth/*` (when SPEC-AUTH-001 ships) — read-only consumer.

---

## 8. Open Questions

The SPEC's _TBD_ markers and the research artifact's §10 are the
canonical list. Restated here for plan-auditor convenience:

1. **`get_citation` storage source** — `internal/index.GetByDocID`
   semantics depend on SPEC-IDX-001 finalisation. Marked _TBD_ in
   REQ-MCP-012's underlying call site. Run phase decides.

2. **SPEC-AUTH-001 middleware constructor name** — REQ-MCP-013 commits
   to consuming whatever AUTH-001 exports; the exact identifier is
   _TBD_ until that SPEC is implemented.

3. **HTTP transport in Helm chart defaults** — SPEC-DEPLOY-001 (M9) owns
   the deployment matrix; whether HTTP transport is on or off by default
   in production manifests is _TBD_ pending operator feedback after V1
   ships locally.

4. **Tag-based `sources` filtering, MCP `resources`/`prompts`/`sampling`
   features, `search_team_memory` / `list_team_queries` tools, legacy
   2024-11-05 HTTP+SSE backwards compat** — all explicitly excluded per
   §4; documented for traceability.

5. **Whether to extract shared orchestrator helper to
   `internal/orchestrator/` or keep it co-located with `cmd/usearch/`** —
   factoring decision deferred to run phase per REQ-MCP-008's contract
   ("MUST NOT duplicate orchestration; method TBD by run phase").

These items do NOT block plan-auditor PASS; they are tagged as known
unresolved scope edges with rationale.

---

## 9. References

External (cited in research.md §11):

- MCP specification (2025-06-18): https://modelcontextprotocol.io/specification/2025-06-18
- MCP transports section: https://modelcontextprotocol.io/specification/2025-06-18/basic/transports
- Official Go SDK: https://github.com/modelcontextprotocol/go-sdk
- JSON-RPC 2.0 specification: https://www.jsonrpc.org/specification

Internal (project files):

- `.moai/project/product.md` §4 (V1 surfaces include MCP server)
- `.moai/project/roadmap.md` §M7 row + §5 M9 exit criterion
- `.moai/project/tech.md` §1 principle 7 + §3 surfaces table
- `.moai/specs/SPEC-CLI-001/spec.md` — orchestration pattern reused
- `.moai/specs/SPEC-SYN-004/spec.md` — SSE streaming pattern reused
- `.moai/specs/SPEC-DEEP-004/spec.md` — cap enforcement + decision-event schema
- `.moai/specs/SPEC-AUTH-001/spec.md` — JWT middleware (forward dep)
- `cmd/usearch-mcp/main.go` — current SPEC-BOOT-001 stub
- `cmd/usearch/query.go` — orchestration source to share
- `cmd/usearch-api/handlers/deep.go` — `/deep` HTTP handler (quota
  sharing target)
- `internal/synthesis/{client.go,types.go}` — Citation / Request /
  Response shapes
- `internal/deepagent/orchestrator.go` — `RunPipeline` entry point
- `internal/index/index.go` — `GetByDocID` target
- `internal/adapters/registry.go` — `List` for `list_sources`
- `.claude/rules/moai/core/lsp-client.md` — pinning policy template
  reused for the MCP SDK

---

*End of SPEC-MCP-001 v0.1.0 (draft).*
