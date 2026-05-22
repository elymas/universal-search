# SPEC-MCP-001 Research — MCP Server Surface

Status: draft companion to spec.md
Author: limbowl via manager-spec
Date: 2026-05-22

This research artifact is the Phase 0.5 deep-dive that informed SPEC-MCP-001.
It captures the protocol survey, library options, transport tradeoffs,
integration points with existing Universal Search internals, auth handoff
strategy, and the open questions the SPEC explicitly defers.

---

## 1. Model Context Protocol — protocol summary

The Model Context Protocol (MCP) is a JSON-RPC 2.0 based protocol from
Anthropic for connecting LLM clients (Claude Code, Claude Desktop, Codex,
Gemini CLI) to external capability servers. The protocol version reference
target for V1 of this SPEC is `2025-06-18` (the current spec at the time of
this research; the prior `2024-11-05` revision is supported only as a
backwards-compatibility path — see §3.3).

Key concepts (from the official spec at https://modelcontextprotocol.io/
specification/2025-06-18):

- **Hosts**: LLM applications (Claude Code, Claude Desktop, Codex CLI,
  Gemini CLI) that initiate connections.
- **Clients**: Per-server connectors hosted inside the host application.
- **Servers**: External processes (this SPEC) that expose capabilities.
- **Features the server may expose**:
  - **Tools** — model-callable functions (this SPEC's primary surface).
  - **Resources** — addressable context blobs (deferred to a follow-up
    SPEC; see §10 Q3).
  - **Prompts** — templated user-facing flows (deferred; §10 Q3).
- **Transports** — see §3.

Each MCP session starts with a JSON-RPC `initialize` request from the
client. The client and server negotiate protocol version, supported
features, and capabilities. After `initialized` notification, the client
may call `tools/list`, `tools/call`, etc. Sessions are stateful: the server
SHOULD maintain context for `Mcp-Session-Id` on Streamable HTTP, and the
stdio subprocess for stdio transport.

---

## 2. Go SDK survey

### 2.1 Candidates

| Library | Status (2026-05) | License | Notes |
|---------|------------------|---------|-------|
| `github.com/modelcontextprotocol/go-sdk` | Official; v1.x stable | MIT | Primary candidate. Maintained by the MCP working group; covers stdio + Streamable HTTP server + client; JSON Schema + JSON-RPC 2.0 codecs included; ~180+ documented snippets via Context7. |
| `github.com/mark3labs/mcp-go` | Community; widely used | MIT | Predates the official SDK; many production servers (incl. early `github-mcp-server`) used this; still maintained but the official SDK has feature parity now and is the recommended path forward. |
| `github.com/metoro-io/mcp-golang` | Community | MIT | Smaller surface; not maintained as actively. |
| Hand-rolled JSON-RPC on top of `sourcegraph/jsonrpc2` | n/a | n/a | Used by `charmbracelet/x/powernap` for LSP. Possible but loses MCP-specific lifecycle, capability negotiation, and tool schema validation. Rejected — duplicates the official SDK without benefit. |

### 2.2 Recommendation

**Primary**: `github.com/modelcontextprotocol/go-sdk` (latest v1.x at pin time).

**Rationale**:
- Official, in lock-step with spec revisions.
- Built-in JSON Schema validation for tool inputs/outputs (matches MCP
  `inputSchema` / `outputSchema` requirements).
- Both transports out of the box: stdio (subprocess) and Streamable HTTP.
- Idiomatic Go: `mcp.Server`, `mcp.RegisterTool[Input, Output]`, typed
  handlers with `context.Context`.
- Aligns with the project's "single binary, single canonical library"
  preference (cf. `.claude/rules/moai/core/lsp-client.md` for the same
  principle applied to powernap).

**Fallback (if blocking bug found)**: `mark3labs/mcp-go` v0.x — drop-in
replacement at the handler layer; transport semantics are identical at the
wire level.

### 2.3 Pinning policy

Mirror the upgrade policy from SPEC-LSP-CORE-002 / `.claude/rules/moai/
core/lsp-client.md`:
- Pin exact `vX.Y.Z` in `go.mod`.
- Before bumping: run the full MCP integration suite against three host
  clients (Claude Code, Codex CLI, Gemini CLI).
- Record the new pin and date here.

---

## 3. Transport tradeoffs

The current MCP spec (2025-06-18) defines two standard transports. The
previous 2024-11-05 revision defined a third (HTTP+SSE) which is now
deprecated but retained for backwards compatibility.

### 3.1 stdio

Behaviour:
- Client launches the server as a subprocess.
- Server reads JSON-RPC frames (newline-delimited UTF-8) from stdin and
  writes responses to stdout.
- stderr is free-form (slog records, panic dumps).

Pros:
- Zero network surface; no auth needed (client-process boundary).
- Trivial install: client launches `usearch-mcp` with stdio config.
- Aligns with Claude Code / Codex / Gemini CLI default install pattern
  ("command + args" in `~/.claude.json`).
- Single-tenant by construction: each client gets its own process.

Cons:
- Per-process startup cost (~100-500ms cold start of Go binary including
  obs.Init + LSP-less footprint).
- One client = one process; no shared state across clients on the same
  host.
- Cannot expose to remote clients.

When to use: local CLI clients (Claude Code, Codex CLI, Gemini CLI). This
is the V1 default per the SPEC.

### 3.2 Streamable HTTP

Behaviour (per https://modelcontextprotocol.io/specification/2025-06-18/
basic/transports):
- Server provides a single HTTP endpoint (POST + GET).
- Client POSTs JSON-RPC messages with `Accept: application/json,
  text/event-stream`.
- Server may respond with `application/json` (single response) OR
  `text/event-stream` (SSE stream for long-running or multi-event
  responses).
- Optional GET request opens an SSE stream for server-initiated messages.
- Optional `Mcp-Session-Id` header for stateful sessions.
- `MCP-Protocol-Version` header required on all post-initialize requests.

Security requirements (HARD per the spec):
- Server MUST validate `Origin` header on all incoming connections (DNS
  rebinding mitigation).
- Server SHOULD bind to `127.0.0.1` when running locally.
- Server SHOULD implement proper authentication.

Pros:
- Remote access (team-deployed server with multiple host clients).
- Bidirectional streaming via SSE — natural pair with SPEC-SYN-004's
  existing SSE writer (sentence-level incremental synthesis).
- Resumable streams via `Last-Event-ID` header.
- Multi-tenant via auth headers (`Authorization: Bearer <JWT>`).

Cons:
- Auth + Origin validation + session management add surface area.
- Clients must support 2025-06-18; older clients fall back to deprecated
  HTTP+SSE (see §3.3).

When to use: remote team-deployment (V1.1 / V1.2 path). Default-off at V1;
operator enables via `--transport http --listen 127.0.0.1:7080` flag.

### 3.3 Legacy HTTP+SSE (2024-11-05)

Deprecated by the 2025-06-18 spec but documented for backwards-compat.
Servers wishing to support older clients host both the legacy SSE+POST
endpoint pair AND the new MCP endpoint. The SPEC explicitly DEFERS this
backwards-compat dual-mode operation; see §10 Q4.

### 3.4 Transport selection matrix

| Client | Recommended transport | Reasoning |
|--------|----------------------|-----------|
| Claude Code (local) | stdio | Default install pattern; ships with first-class stdio support in `~/.claude.json`. |
| Claude Desktop (local) | stdio | Same as Claude Code. |
| Codex CLI (local) | stdio | Same as above. |
| Gemini CLI (local) | stdio | Same as above. |
| Claude Code (remote team) | Streamable HTTP | Operator deploys `usearch-mcp` server on team network; users add HTTP endpoint to their client config. |
| Browser-based clients | Streamable HTTP | Only HTTP transport works in browsers. |

V1 ships stdio as the primary surface. HTTP is feature-flagged for
operator opt-in.

---

## 4. Tool surface design

### 4.1 Tools to expose

Based on `.moai/project/product.md` §4 (V1 scope) and the existing internal
package surface (`internal/synthesis`, `internal/deepagent`,
`internal/index`, `internal/adapters/registry`), the natural V1 tool set is:

| Tool | Purpose | Calls into |
|------|---------|------------|
| `search` | Basic-mode synthesis: query → fanout → synthesize → return cited paragraph. Streaming preferred when transport supports it. | `internal/fanout`, `internal/synthesis` (already invoked by `cmd/usearch/query.go`). |
| `deep_research` | `/deep` multi-agent pipeline (M5). Long-running (≤5min p50). MUST stream progress events when transport supports it. | `internal/deepagent.RunPipeline`. |
| `list_sources` | Enumerate available adapters with metadata (name, category, auth-required flag, lang support). Useful for client UI to build source filters. | `internal/adapters.Registry.List`. |
| `get_citation` | Resolve a citation `doc_id` (returned in a prior `search` / `deep_research` response) to the full `NormalizedDoc` (title, URL, snippet, source, score, fetched_at). Round-trips for citation drill-down. | `internal/index.Index.GetByDocID` (or equivalent — exact method name owned by the run phase). |

Optional V1.1 tools (deferred; §10 Q5):
- `search_team_memory` — query the shared team index directly without
  fanning out to external adapters. Depends on SPEC-IDX-005.
- `list_team_queries` — return recent team queries for an authenticated
  user. Depends on SPEC-AUTH-003 audit log.

### 4.2 Tool schema

Each tool MUST declare a typed `inputSchema` and `outputSchema` (JSON
Schema Draft 2020-12 per MCP spec). The Go SDK provides
`mcp.RegisterTool[Input, Output]` which generates the schema from Go
struct tags. Example shape (illustrative — final form owned by run phase):

```
type SearchInput struct {
  Query       string   `json:"query" jsonschema:"required,description=The user query."`
  Sources     []string `json:"sources,omitempty" jsonschema:"description=Optional adapter name allowlist."`
  MaxResults  int      `json:"max_results,omitempty" jsonschema:"default=10,minimum=1,maximum=50"`
  Lang        string   `json:"lang,omitempty" jsonschema:"description=BCP47 language code; auto-detected if omitted."`
}

type SearchOutput struct {
  Summary   string     `json:"summary"`
  Citations []Citation `json:"citations"`
  Stats     Stats      `json:"stats"` // request_id, latency_ms, source_count, ...
}

type Citation struct {
  Index  int    `json:"index"`
  Title  string `json:"title"`
  URL    string `json:"url"`
  Source string `json:"source"`
  DocID  string `json:"doc_id"`
}
```

The MCP `Citation.DocID` is the key consumed by `get_citation`. This
matches the schema already established in `internal/synthesis/types.go`
(`type Citation struct{...}`).

### 4.3 Streaming for `deep_research`

`deep_research` calls `internal/deepagent.RunPipeline` which can take up
to 5min p50. The MCP server MUST stream progress for this tool whenever
the transport supports it:

- **stdio**: MCP does not natively stream from the tool itself; the
  recommended pattern is to emit `notifications/message` JSON-RPC
  notifications from the server during the long-running call, then return
  the final `tools/call` response. The Go SDK supports
  `ServerSession.NotifyProgress(...)` for this purpose.
- **Streamable HTTP**: Server returns `Content-Type: text/event-stream`
  and emits SSE events. Progress events flow on the same stream as the
  final response.

The mapping from `internal/deepagent` pipeline stages → MCP progress
events is owned by the run phase. The SPEC fixes the contract: progress
events MUST be emitted at minimum at each agent stage transition
(Researcher → Reviewer → Writer → Verifier), and a final response MUST
be returned within the SPEC-DEEP-004 cost guard's quota window.

The basic `search` tool is short (p95 ≤ 20s per `.moai/project/product.md`
§6 success metrics). Streaming for `search` is OPTIONAL — when the
transport is Streamable HTTP, the server SHOULD emit sentence-level events
that re-use SPEC-SYN-004's existing segmenter, mapping each SSE sentence
event onto an MCP `notifications/message`. When the transport is stdio,
`search` returns a single tool result.

---

## 5. Integration with existing Universal Search internals

### 5.1 Stub already in place

`cmd/usearch-mcp/main.go` is a SPEC-BOOT-001 stub that:
- Calls `obs.Init` (same lifecycle as `cmd/usearch` and `cmd/usearch-api`).
- Emits a startup log line and exits 0.
- Prints `"usearch-mcp: not implemented (see SPEC-MCP-001)"` to stderr.

SPEC-MCP-001 replaces the placeholder body but **preserves the obs.Init
contract** so that admin-port metrics + slog + OTel work identically across
all three binaries.

### 5.2 Reused packages (read-only consumers)

The MCP server is a thin adapter over existing internal packages:

| Tool | Internal package | Existing fan_in |
|------|------------------|-----------------|
| `search` | `internal/fanout` + `internal/synthesis` | fanout: CLI + tests + future MCP (per existing `// @MX:ANCHOR` at `internal/fanout/fanout.go`); synthesis: CLI + API + future MCP. |
| `deep_research` | `internal/deepagent` | API + future MCP. |
| `list_sources` | `internal/adapters` | CLI + future MCP. |
| `get_citation` | `internal/index` | CLI + API + CACHE-001 + IDX-005 + future MCP (per existing `// @MX:ANCHOR` at `internal/index/index.go`). |

The MCP entry points become a NEW caller in each ANCHOR; SPEC-MCP-001 run
phase MUST update `@MX:REASON` lines accordingly.

### 5.3 Reuse of `cmd/usearch/query.go` orchestration

`cmd/usearch/query.go::Execute` already wires router → fanout → synthesis
end-to-end. The MCP `search` tool handler SHOULD extract a shared
orchestrator helper from `cmd/usearch/query.go` into a new
`internal/orchestrator/search.go` package so that both `cmd/usearch` and
`cmd/usearch-mcp` invoke identical pipeline logic. This avoids
divergence between the CLI and MCP surfaces.

The exact factoring is owned by the run phase (the SPEC fixes the
contract; the helper extraction is implementation detail).

### 5.4 Observability reuse

The MCP server MUST register the standard `obs.Tracer` + `obs.Logger` per
the existing pattern. Each tool invocation gets a root OTel span named
`mcp.tool.{tool_name}` carrying attributes:

- `mcp.transport` — `stdio` | `streamable_http`
- `mcp.session_id` — only when transport is Streamable HTTP and session
  ID is bounded (avoid PII per SPEC-DEEP-004 NFR-DEEP4-007)
- `mcp.tool_name`
- `mcp.protocol_version`
- `mcp.client_name` / `mcp.client_version` (from `initialize` request)

Per-tool spans become children of the root MCP span and parent the
downstream router / fanout / synthesis spans.

---

## 6. Authentication strategy — handoff to SPEC-AUTH-001

V1 ships with two authentication modes, both delegating verification to
SPEC-AUTH-001 (M6) when it lands:

### 6.1 stdio transport (V1 default)

No authentication. The OS process boundary IS the trust boundary: the
client (Claude Code) launches the server subprocess and has full local
trust. Identity is inherited from the launching shell user. This matches
how every other MCP server in the ecosystem operates over stdio.

The MCP server MAY emit a single ledger row tagged `user_id="local"`,
`tenant_id="<config-default>"` for ledger continuity with SPEC-DEEP-004 —
exact behaviour owned by the run phase.

### 6.2 Streamable HTTP transport (V1 opt-in, V1.1+ default)

Authentication delegated to SPEC-AUTH-001:

- HTTP `Authorization: Bearer <token>` header REQUIRED.
- Token format: JWT issued by the OIDC IdP configured in SPEC-AUTH-001
  (Keycloak / Authentik).
- The MCP server middleware re-uses the JWT validation middleware shipped
  by SPEC-AUTH-001 (`internal/auth/middleware.go`).
- On successful validation, `costguard.UserIDKey` and `auth.ClaimsKey`
  are injected into the request context — identical contract to the
  `cmd/usearch-api` HTTP path.
- On validation failure (missing / expired / invalid signature), the MCP
  server returns a JSON-RPC error with code mapping per §7.

**Forward-compat commitment**: This SPEC is a downstream consumer of
SPEC-AUTH-001; it MUST NOT re-implement JWT validation. SPEC-MCP-001 ships
the HTTP transport in a "auth-required" mode that hard-fails until
AUTH-001 lands. Operators who want HTTP transport before AUTH-001 ships
MUST opt-in via `--auth-mode=trust-headers` (which trusts an upstream
proxy to validate — same pattern as DEEP-004 `X-User-Id` trust model).

### 6.3 Tool-level authorization (SPEC-AUTH-002, M6)

Per-tool RBAC (e.g., restrict `deep_research` to specific teams) is
deferred to SPEC-AUTH-002 (Casbin policy). V1 MCP server applies the SAME
quota / cap enforcement that SPEC-DEEP-004 applies at the HTTP layer —
the `deep_research` tool handler MUST go through `costguard.
CapCheckMiddleware` so that `/deep`-via-MCP and `/deep`-via-HTTP share
quota counters.

### 6.4 Audit log integration (SPEC-AUTH-003, M6)

Every MCP tool invocation MUST emit an audit log entry compatible with
the decision event log schema from SPEC-DEEP-004 REQ-DEEP4-010 / §6.3
forward-compat commitment. Fields: `timestamp`, `event_type=
"mcp.tool_call"`, `request_id`, `tenant_id`, `user_id`, `tool_name`,
`outcome` (`success` | `error` | `capped` | `unauthorized`).

When SPEC-AUTH-003 lands, this stream is consumed by the audit subsystem
with no schema change.

---

## 7. Error mapping — usearch errors → MCP error codes

MCP uses JSON-RPC 2.0 error codes. The reserved ranges:

- `-32700` to `-32603` — JSON-RPC standard (parse error, invalid request,
  method not found, invalid params, internal error).
- `-32000` to `-32099` — Server-defined errors (implementation-specific).

MCP-specific conventions (per SDK):
- `MethodNotFound` = `-32601` (tool name unknown).
- `InvalidParams` = `-32602` (tool input fails schema validation).
- `InternalError` = `-32603` (server-side panic / unrecoverable).

Universal Search error → MCP code mapping (the SPEC fixes the contract;
extension reserved for future error types):

| Source | usearch error | MCP code | data fields |
|--------|---------------|----------|-------------|
| Tool routing | unknown tool name | `-32601 MethodNotFound` | `tool_name` |
| Input validation | empty `query`, oversize `max_results`, invalid `sources` | `-32602 InvalidParams` | `field`, `reason` |
| Cost guard | DEEP-004 cap exceeded | `-32000` (server-defined, namespace `usearch.cap_exceeded`) | `dimension`, `remaining`, `reset_at`, `retry_after_s` — mirror DEEP-004 REQ-DEEP4-010 body |
| Cost guard | DEEP-004 Haiku screen reject | `-32001 usearch.deep_not_warranted` | `screen_score`, `suggested_mode`, `rationale` |
| Auth (HTTP) | missing / expired / invalid JWT | `-32002 usearch.unauthorized` | `reason` |
| Adapter | no adapters matched query | `-32003 usearch.no_adapters_matched` | `query_category` |
| Fanout | all adapters failed | `-32004 usearch.all_adapters_failed` | `errors[]` |
| Synthesis | synthesis unavailable / degraded | `-32005 usearch.synthesis_degraded` | `degraded_reason` (returns partial result alongside error code) |
| Pipeline | context cancelled / deadline exceeded | `-32006 usearch.timeout` | `stage`, `deadline_ms` |
| Internal | panic / unrecoverable | `-32603 InternalError` | `request_id` (for log lookup) — MUST NOT include stack trace per SPEC-CLI-001 NFR-CLI-004 |

The full code table is owned by the run phase; the SPEC fixes the
namespace `usearch.*` for data field `error.namespace` so that clients
can branch on error families without parsing free-text messages.

---

## 8. Storage / config

V1 MCP server is stateless — no new persistent storage beyond what the
underlying tools already use (Postgres audit, Redis quota counters,
Qdrant/Meili/PG index).

Configuration (per `internal/config` koanf layered loader):

- `mcp.transport` — `stdio` (default) | `http`
- `mcp.http.listen_addr` — `127.0.0.1:7080` (default; rejects `0.0.0.0`
  unless `mcp.http.bind_public: true` AND `mcp.http.auth_mode` ∈
  `{jwt, trust-headers}`)
- `mcp.http.session_ttl_minutes` — `60` (default)
- `mcp.http.auth_mode` — `jwt` (default; requires AUTH-001) |
  `trust-headers` (proxy-validated) | `none` (dev only)
- `mcp.tools.enabled` — `[search, deep_research, list_sources,
  get_citation]` (default; operator may disable individual tools)
- `mcp.deep_research.requires_quota` — `true` (default; routes through
  DEEP-004 cap)

---

## 9. Risks

| Risk | Severity | Mitigation |
|------|----------|------------|
| Streamable HTTP DNS rebinding attack on local-bound server | High | Origin header validation MUST be implemented (REQ); CI test exercises rejection of off-origin POSTs. |
| Tool input schema drift between MCP SDK and our struct tags | Medium | CI verifies generated JSON Schema matches a golden fixture; run phase owns the fixture path. |
| `deep_research` MCP call bypasses DEEP-004 cap if wired wrong | High | REQ: the tool handler MUST invoke the same `costguard.CapCheckMiddleware`; integration test asserts a capped tenant calling `deep_research` via MCP receives `-32000` (not HTTP 429). |
| stdio subprocess startup cost dominates short queries | Medium | Cold-start budget ≤ 500ms (NFR); document long-running-process best practice for clients that support it. |
| Auth path drift between HTTP MCP and `cmd/usearch-api` | High | REQ: HTTP MCP uses the SAME middleware constructor exported by SPEC-AUTH-001 (`auth.NewJWTMiddleware`); no parallel implementation. |
| MCP SDK version churn (pre-1.0 → 1.x → 2.x) | Medium | Pin exact version per §2.3 policy; upgrade gated on three-client integration suite. |
| Tool output exceeds MCP response size limits | Medium | Per-tool `max_results` upper bound; large `deep_research` outputs streamed via progress events with the final response carrying only `summary` + `citations`; full report retrievable via `get_citation`-style follow-up. |
| Origin spoofing via proxy headers | Medium | Origin validation reads ONLY the `Origin` header set by the browser; proxies in front MUST set `X-Forwarded-Origin` — explicit allow-list config. |
| Client disconnection mid-`deep_research` orphans pipeline | High | MCP server propagates `ctx.Done()` from the JSON-RPC connection into the deepagent pipeline; pipeline MUST respect context cancellation (SPEC-DEEP-002 REQ). |
| Multiple concurrent stdio clients sharing one OS user trust boundary | Low | Each stdio client gets its own subprocess; concurrency limit per-subprocess is OS-process scoped. |

---

## 10. Open questions (deferred decisions)

These items are explicitly UNRESOLVED at SPEC-draft time. The SPEC marks
each with `_TBD_` and a rationale; they do not block plan-auditor PASS.

1. **`get_citation` storage source**: should it read from the hybrid
   index (`internal/index`) or from the per-session `synthesis.Response`
   cache? Open until SPEC-IDX-001 finalizes `GetByDocID` semantics.
   `_TBD_` — Run phase decides based on SPEC-IDX-001 implementation status.

2. **MCP `resources` and `prompts` features**: out of scope for V1.
   Resources could expose recent team queries (depends on SPEC-AUTH-003);
   prompts could expose curated query templates. Reserved for V1.1+.

3. **Tag-based `sources` filtering**: V1 accepts adapter names only
   (matches SPEC-CLI-001 §2.2). Tag-based filtering (`sources=["social"]`
   → all social adapters) deferred to CLI-002 / a future MCP-002.

4. **Backwards-compat with deprecated HTTP+SSE transport (2024-11-05)**:
   V1 supports only the 2025-06-18 Streamable HTTP transport for HTTP.
   Operators with clients pinned to 2024-11-05 must upgrade. Reserved for
   MCP-002 if a real-world need surfaces.

5. **Team-memory tools (`search_team_memory`, `list_team_queries`)**:
   deferred to V1.1; depend on SPEC-IDX-005 and SPEC-AUTH-003.

6. **MCP `sampling` feature** (server-initiated LLM calls back through the
   client): out of scope for V1. Universal Search owns its own LLM router
   via SPEC-LLM-001; sampling would require a parallel routing path.

7. **`mcp.http.bind_public` operational guidance**: do we ship Helm chart
   defaults for HTTP transport in V1, or wait for AUTH-001 + AUTH-002 to
   be production-ready? `_TBD_` — runway dependent. SPEC-DEPLOY-001 (M9)
   owns the Helm chart; V1 MCP HTTP defaults to localhost-only, deploy
   SPEC may extend.

---

## 11. References

External (verified URLs):

- MCP specification (2025-06-18): https://modelcontextprotocol.io/specification/2025-06-18
- MCP transports section: https://modelcontextprotocol.io/specification/2025-06-18/basic/transports
- Official Go SDK: https://github.com/modelcontextprotocol/go-sdk
- mark3labs/mcp-go (fallback library): https://github.com/mark3labs/mcp-go
- JSON-RPC 2.0 spec: https://www.jsonrpc.org/specification
- BCP 14 (RFC 2119 / 8174 keyword conventions): https://datatracker.ietf.org/doc/html/bcp14

Internal (project files):

- `.moai/project/product.md` — V1 scope, success metrics, MCP listed as a V1 surface.
- `.moai/project/roadmap.md` — M7 row defining SPEC-MCP-001 scope and M9 exit criterion.
- `.moai/project/tech.md` — §3 surfaces table listing MCP server as Go + official MCP Go SDK.
- `.moai/specs/SPEC-CLI-001/spec.md` — structural template for this SPEC; established the orchestration pattern that MCP `search` tool reuses.
- `.moai/specs/SPEC-SYN-004/spec.md` — SSE streaming pattern; informs MCP Streamable HTTP streaming for `search` and `deep_research`.
- `.moai/specs/SPEC-DEEP-004/spec.md` — cost guard cap enforcement; `deep_research` tool MUST route through.
- `.moai/specs/SPEC-AUTH-001/spec.md` — JWT middleware; HTTP transport delegates auth here.
- `.moai/specs/SPEC-AUTH-003/spec.md` — audit log; MCP tool calls emit compatible decision-event JSON lines.
- `cmd/usearch-mcp/main.go` — current SPEC-BOOT-001 stub; replaced by this SPEC.
- `cmd/usearch/query.go` — orchestration pattern for `search` tool.
- `cmd/usearch-api/handlers/synthesis.go` — SSE streaming handler (SPEC-SYN-004) reused as a pattern.
- `cmd/usearch-api/handlers/deep.go` — `/deep` handler (SPEC-DEEP-001/002/003); MCP `deep_research` invokes the same pipeline entry point.
- `internal/synthesis/client.go` + `internal/synthesis/types.go` — Citation / Request / Response shapes.
- `internal/deepagent/orchestrator.go` — `RunPipeline` entry point.
- `internal/index/index.go` — `GetByDocID` (or equivalent) for `get_citation`.
- `internal/adapters/registry.go` — `List` for `list_sources`.
- `.claude/rules/moai/core/lsp-client.md` — pinning policy template (powernap) re-used for MCP SDK in §2.3.

---

*End of SPEC-MCP-001 research v0.1.0 (draft).*
