# SPEC-MCP-001 Plan — phased implementation

Status: draft companion to spec.md
Author: limbowl via manager-spec
Date: 2026-05-22
Methodology: TDD (per `.moai/config/sections/quality.yaml` `development_mode: tdd`)
Coverage target: 85%
Harness: standard (per `.moai/config/sections/harness.yaml` auto-routing —
16 REQs (13 × P0 + 3 × P1) + 8 NFRs + 1 new cmd directory + 1 new internal
package; Sprint Contract optional)

This plan sequences the SPEC-MCP-001 implementation into priority-ordered
phases. Per `.claude/rules/moai/core/agent-common-protocol.md`, time
estimates are PROHIBITED — phases use priority + ordering, never duration.

---

## 1. Implementation principle

The MCP server is a thin protocol-translation layer over existing
internals. The plan favours:

1. **Reuse over reimplementation** — `search` shares the CLI
   orchestrator; `deep_research` shares the `/deep` HTTP handler's cap
   guard.
2. **Transport-first scaffolding** — get a minimal stdio surface
   answering `initialize` + `tools/list` before any tool body is wired.
3. **HTTP transport behind a feature flag** — V1 ships stdio as the
   verified-end-to-end path; HTTP is gated and tested independently but
   not the M7 exit-criterion blocker.
4. **Cap and audit on day one** — `deep_research` MUST route through
   `costguard.CapCheckMiddleware` and emit decision-event JSON lines from
   the first iteration; no "we'll wire auth later" anti-pattern.
5. **TDD with stub adapters and stub deepagent** — integration tests use
   `httptest.Server` stubs in line with SPEC-CLI-001 D4 / ADP-001 NFR.

---

## 2. Phase ordering

Priority labels per MoAI rule (no time estimates).

### Phase 0 — Plan-auditor PASS (Priority High)

- Plan-auditor reviews spec.md + research.md + plan.md + acceptance.md
  (the latter authored alongside this plan).
- Address MAJOR / MINOR / NIT findings via amendment commits.
- Status transition: `draft → approved` once PASS.
- Block: no implementation work begins until Phase 0 completes.

### Phase 1 — Scaffolding (Priority High)

Goal: SDK pinned, package skeleton compiles, stdio transport answers
`initialize`.

Tasks:
1. Pin `github.com/modelcontextprotocol/go-sdk` exact version in
   `go.mod`; `go mod tidy`.
2. Create `internal/mcpserver/` package skeleton: `server.go`,
   `config.go`, `transport_stdio.go`. Empty handlers; package compiles.
3. Replace `cmd/usearch-mcp/main.go` stub body with `mcpserver.New(...).
   Start(ctx)`; preserve obs.Init lifecycle and `--version` flag.
4. Wire `--transport` flag (`stdio` | `http`); default `stdio`.
5. Wire SIGINT / SIGTERM handler (REQ-MCP-003 scaffolding; full
   grace-period behaviour deferred to Phase 6).
6. RED test: `TestStdioInitializeRoundTrip` (REQ-MCP-001, 002, 004).
7. GREEN: minimal stdio loop returning empty `tools/list`.
8. REFACTOR: extract transport adapter interface so HTTP can plug in
   later without churn.

Exit criterion:
- `go build ./...` green.
- `usearch-mcp` (stdio) completes MCP `initialize` round-trip from the
  reference SDK test client.
- `tools/list` returns empty array (no tools registered yet).

### Phase 2 — Tool surface, read-only tools first (Priority High)

Goal: `list_sources` and `get_citation` (both read-only, no cap) wired
end-to-end.

Tasks:
1. `internal/mcpserver/tools/types.go` — `SearchInput/Output`,
   `DeepResearchInput/Output`, `Citation`, `Stats`, `ListSourcesOutput`,
   `GetCitationInput/Output` with jsonschema tags.
2. `internal/mcpserver/tools/list_sources.go` — wraps
   `internal/adapters.Registry.List()`; deterministic sort.
3. `internal/mcpserver/tools/get_citation.go` — wraps the chosen
   `internal/index` resolver (note: REQ-MCP-012 _TBD_; pick the
   currently-available method, document the choice in tool comment).
4. Register both tools in `server.go`; verify `tools/list` returns 2
   tools with matching golden fixture (REQ-MCP-007).
5. RED tests for both tools: input validation, sort order, not-found
   path (REQ-MCP-011, 012).
6. GREEN + REFACTOR.

Exit criterion:
- `tools/list` golden fixture established and checked in.
- Both read-only tools callable end-to-end from stdio reference client.

### Phase 3 — `search` tool with shared orchestrator (Priority High)

Goal: `search` tool reuses CLI orchestration; no duplicate pipeline
code.

Tasks:
1. Decide on shared orchestrator location (per spec.md §8 Q5):
   - Option A: extract `internal/orchestrator/search.go` and refactor
     `cmd/usearch/query.go::Execute` to call it.
   - Option B: keep helper in `cmd/usearch/`, expose via internal
     package; refactor `Execute` to thin wrapper.
   - Recommendation: Option A — cleaner separation and avoids the
     anti-pattern of importing from `cmd/` packages.
2. RED test for the shared orchestrator: `TestSharedSearchOrchestrator`
   covering router → fanout → synthesis happy path + partial-failure +
   no-adapters paths.
3. GREEN: extract the orchestrator function, update CLI to call it.
4. Verify CLI test suite still green (no regression — SPEC-CLI-001 tests
   must continue to pass with the refactor).
5. `internal/mcpserver/tools/search.go` — wraps the shared orchestrator;
   marshals `synthesis.Response` → `SearchOutput`.
6. RED + GREEN + REFACTOR for `search` tool tests (REQ-MCP-008).
7. Verify NFR-MCP-003 search-latency parity benchmark.

Exit criterion:
- One orchestrator function callable from both CLI and MCP `search`.
- Both surfaces produce equivalent results for the same input.
- Latency-parity benchmark within 15% in CI.

### Phase 4 — `deep_research` tool with cap guard and streaming (Priority High)

Goal: `deep_research` routes through SPEC-DEEP-004 cap; emits progress
events; quota shared with HTTP `/deep`.

Tasks:
1. Wire `costguard.CapCheckMiddleware` invocation at the tool-handler
   boundary (NOT inside `internal/deepagent`). The middleware was built
   for chi-style HTTP context; adapt invocation pattern for MCP
   tool-call context. Document the adaptation in `internal/mcpserver/
   tools/deep_research.go` header comment.
2. Identity adapter: derive `user_id` / `tenant_id` from MCP request
   context per transport:
   - stdio: default to `user_id="local"`, `tenant_id=<config-default>`
     (research §6.1 promise).
   - HTTP `auth_mode=jwt`: read from `costguard.UserIDKey` injected by
     SPEC-AUTH-001 middleware.
   - HTTP `auth_mode=trust-headers`: read from `X-User-Id` / `X-Tenant-Id`
     headers (mirrors DEEP-004 D1).
3. Pipeline-stage progress emission: hook into deepagent stage
   transitions; emit `notifications/message` per REQ-MCP-010.
4. Audit log emission: REQ-MCP-015 decision-event JSON line per tool
   call, conforming to DEEP-004 schema.
5. RED test: `TestDeepResearchSharesQuotaWithHTTP` — cap via HTTP,
   verify MCP rejects with `-32000 usearch.cap_exceeded`.
6. RED tests for stage-boundary progress count, audit-line schema
   conformance, error-mapping table coverage.
7. GREEN + REFACTOR.

Exit criterion:
- Cap-sharing integration test green.
- ≥4 progress events emitted per `deep_research` call across both
  transports.
- Audit JSON schema conformance test green.

### Phase 5 — HTTP transport (Priority Medium)

Goal: Streamable HTTP transport with Origin validation, bind policy,
auth-mode dispatch.

Tasks:
1. `internal/mcpserver/transport_http.go` — POST/GET handlers per MCP
   2025-06-18 §Transports.
2. `Mcp-Session-Id` issuance + validation; `MCP-Protocol-Version` header
   handling.
3. SSE writer for streaming responses; reuse SPEC-SYN-004 SSE patterns
   (frame format, heartbeat, slow-client write timeout) where applicable.
4. Origin validation per REQ-MCP-006 + NFR-MCP-007; CI test matrix
   (missing, null, off-allowlist, case-folded variants).
5. Bind policy: refuse non-loopback bind unless `bind_public: true` AND
   `auth_mode` ∈ {jwt, trust-headers}. Startup-refusal test.
6. Auth mode dispatcher (`internal/mcpserver/auth.go`):
   - `none` (dev only; loopback-only enforced).
   - `trust-headers` (V1 default until AUTH-001 ships).
   - `jwt` (delegates to AUTH-001 middleware; gated behind interface so
     tests can stub).
7. RED tests for each auth mode path.
8. NFR-MCP-002 streaming-first-byte latency benchmark.

Exit criterion:
- HTTP `initialize` → `tools/list` → `tools/call` over POST works.
- SSE stream emits final response after progress events.
- Origin / bind / auth tests green.
- Pre-AUTH-001 ships, HTTP transport is functional via
  `trust-headers` mode only.

### Phase 6 — Lifecycle hardening (Priority Medium)

Goal: graceful shutdown, observability completeness, NFR closeout.

Tasks:
1. SIGINT / SIGTERM grace period (REQ-MCP-003): track in-flight
   requests, allow `grace_period_seconds` for completion, cancel
   thereafter, single shutdown slog line.
2. OTel span instrumentation per REQ-MCP-014: root span + parent-child
   relationships + single-end invariant.
3. Prometheus metrics: `usearch_mcp_tool_calls_total{tool,outcome,
   transport}`, `usearch_mcp_request_duration_seconds{tool,transport}`,
   `usearch_mcp_active_sessions{transport}` (HTTP only).
4. NFR closeout: NFR-MCP-001 cold-start benchmark, NFR-MCP-004 goleak
   gate in `TestMain`, NFR-MCP-006 binary-size CI gate, NFR-MCP-005
   label allowlist extension in `TestNoUnboundedLabels`.

Exit criterion:
- All NFRs measurable and gated in CI.
- Shutdown test green (grace period elapses cleanly).
- No goroutine leaks reported by goleak.

### Phase 7 — Cross-client integration verification (Priority Medium)

Goal: M9 exit-criterion preflight — verify the server actually connects
from each target host.

Tasks:
1. Document config snippets in `.moai/specs/SPEC-MCP-001/` for:
   - Claude Code `~/.claude.json` MCP server entry.
   - Codex CLI MCP config.
   - Gemini CLI MCP config.
2. Manual verification: each client successfully completes `initialize`
   + `tools/list` + at least one `tools/call search` against the local
   binary.
3. Capture wire traces for the test fixtures used by Phase 1-6 tests
   (ensures CI tests match real client behaviour).
4. File any discovered gaps as follow-up issues.

Exit criterion:
- Manual three-client verification documented and green.
- Sets the stage for SPEC-SKILL-001 (M7) to wrap the verified server.

### Phase 8 — Sync phase (Priority Low)

Goal: documentation + PR.

Tasks:
1. `manager-docs` updates user-facing docs:
   - `README.md` MCP install section.
   - Add MCP setup page to docs site (when SPEC-DOC-001 ships).
2. CHANGELOG entry.
3. `manager-git` opens PR per V1 release process.
4. Status transition: `approved → implemented` after merge.

---

## 3. Test inventory (TDD checkpoints)

Per-phase RED tests:

- Phase 1: `TestStdioInitializeRoundTrip`,
  `TestVersionFlagPreserved`,
  `TestTransportFlagDefaultsStdio`.

- Phase 2: `TestListSourcesReturnsRegisteredAdapters`,
  `TestListSourcesSortDeterministic`,
  `TestGetCitationResolvesValidDocID`,
  `TestGetCitationNotFoundError`,
  `TestToolsListGoldenFixture`.

- Phase 3: `TestSharedSearchOrchestratorHappyPath`,
  `TestSharedSearchOrchestratorPartialFailure`,
  `TestSharedSearchOrchestratorNoAdapters`,
  `TestSearchToolWrapsSharedOrchestrator`,
  `TestCLIRegressionPostExtraction` (run existing SPEC-CLI-001 suite),
  `BenchmarkSearchLatencyParityCLIvsMCP`.

- Phase 4: `TestDeepResearchRoutesCapMiddleware`,
  `TestDeepResearchSharesQuotaWithHTTP`,
  `TestDeepResearchProgressStageCount`,
  `TestDeepResearchAuditLineSchema`,
  `TestErrorMappingTableComplete`.

- Phase 5: `TestHTTPInitializeWithMcpSessionId`,
  `TestHTTPSSEStreamOrdering`,
  `TestOriginRejectionMatrix`,
  `TestBindPublicRequiresAuth`,
  `TestAuthModeJWTDelegatesToAuth001Stub`,
  `TestAuthModeTrustHeadersPath`,
  `TestAuthModeNoneRefusesPublicBind`,
  `BenchmarkStreamingFirstByteLatency`.

- Phase 6: `TestGracefulShutdownInflight`,
  `TestSpanParentChild`,
  `TestSpanSingleEnd`,
  `TestNoUnboundedLabelsMCP`,
  `TestBinarySizeUnderCap`,
  `TestColdStartUnderCap` (cold-start benchmark in CI gate).

- Phase 7: manual three-client verification log.

`TestMain` invokes `goleak.VerifyTestMain(m)` across all
`internal/mcpserver/` test files per NFR-MCP-004.

---

## 4. MX tag plan

Per `.claude/rules/moai/workflow/mx-tag-protocol.md` (code_comments: en
per `.moai/config/sections/language.yaml`):

| File | Tag | Reason |
|------|-----|--------|
| `internal/mcpserver/server.go::Start` | `@MX:ANCHOR` | fan_in ≥ 2 (cmd/usearch-mcp, tests). `@MX:REASON`: sole server lifecycle entry; signature stability matters for SDK upgrade and future transport plugins. |
| `internal/mcpserver/tools/deep_research.go::Handle` | `@MX:ANCHOR` | fan_in ≥ 3 (server, tests, future tool-RBAC layer from AUTH-002). `@MX:REASON`: quota-sharing contract with HTTP `/deep` is load-bearing; changes affect billing-adjacent behaviour. |
| `internal/mcpserver/tools/deep_research.go::Handle` | `@MX:WARN` | Routes through cost guard middleware; failure to invoke = uncapped LLM spend. `@MX:REASON`: business-critical invariant — cap guard MUST run before deepagent dispatch. |
| `internal/mcpserver/transport_http.go::ServeHTTP` | `@MX:WARN` | Network-facing; Origin validation + bind policy + auth mode dispatch all converge here. `@MX:REASON`: DNS-rebinding mitigation lives in this handler; bypass = silent security regression. |
| `internal/mcpserver/errors.go::MapError` | `@MX:NOTE` | Error-code namespace is the load-bearing client contract; FROZEN per REQ-MCP-016. Behaviour change ripples to every host client. |
| `internal/mcpserver/audit.go::Emit` | `@MX:NOTE` | Decision-event JSON schema additive-only per REQ-MCP-015; SPEC-AUTH-003 downstream consumer. |
| `internal/orchestrator/search.go::Search` (if extracted) | `@MX:ANCHOR` | fan_in ≥ 2 (CLI, MCP). `@MX:REASON`: single source of truth for basic-mode pipeline; both surfaces depend on identical behaviour. |

All agent-generated tags carry the `[AUTO]` prefix and reference
`@MX:SPEC: SPEC-MCP-001`.

---

## 5. Risk-driven sequencing notes

Risks from research.md §9 with their mitigation phase:

- DNS rebinding attack → Phase 5 (HTTP transport ships with Origin
  validation tests).
- Tool input schema drift → Phase 2 (golden fixture established before
  more tools added).
- `deep_research` bypassing cap → Phase 4 (quota-sharing test is RED
  before tool handler exists).
- stdio cold start → Phase 6 (NFR-MCP-001 benchmark in CI gate).
- Auth path drift between MCP HTTP and `cmd/usearch-api` → Phase 5
  (auth dispatcher delegates; no parallel JWT impl).
- MCP SDK version churn → Phase 1 (pinning policy applied at SDK import).
- Tool output size limits → Phase 4 (`deep_research` final response
  carries only `summary` + `citations[]`; full report retrievable via
  follow-up tool call).
- Origin spoofing via proxy headers → Phase 5 (Origin validation reads
  ONLY the `Origin` header; proxies must set `X-Forwarded-Origin` —
  reserved for a future config extension).
- Client mid-call disconnection → Phase 4 (context propagation test
  ensures `deepagent` observes cancellation).
- Multiple concurrent stdio clients sharing one OS user → No mitigation
  needed (one subprocess per client by MCP construction).

---

## 6. Sync-phase deliverables (Phase 8)

- README.md: add "Use as MCP server" section with copy-paste configs for
  Claude Code, Codex CLI, Gemini CLI.
- CHANGELOG.md: SPEC-MCP-001 entry under M7.
- PR title: `feat(mcp): implement SPEC-MCP-001 — MCP server v1 (stdio +
  Streamable HTTP)`.
- PR body: links to spec.md, research.md, acceptance.md; checklist of
  REQ acceptance; cross-client verification log from Phase 7.
- Status transition: `approved → implemented` on merge.
- Notify SPEC-SKILL-001 owner that the blocker is cleared.

---

## 7. Open factoring decisions deferred to run phase

These items are explicitly NOT decided at plan time — they are
implementation-detail choices the run-phase agent will make:

1. Shared orchestrator location: `internal/orchestrator/search.go`
   (recommended in Phase 3) vs alternative module placement.
2. Exact MCP SDK version to pin (latest stable at run-start; subject to
   research §2.3 upgrade policy).
3. Cost guard middleware adaptation pattern: chi-context vs
   transport-agnostic context (the middleware was built for chi; MCP
   tool handlers don't use chi).
4. `internal/index.GetByDocID` resolver name (depends on SPEC-IDX-001
   final API).
5. Whether to extract a `internal/auth/headers.go` helper to share
   `X-User-Id` / `X-Tenant-Id` header parsing with SPEC-DEEP-004 (DRY
   opportunity but adds a refactor scope).
6. Heartbeat interval and slow-client write timeout for the HTTP
   transport (defer to defaults reused from SPEC-SYN-004 unless
   testing reveals a need).

These are scope-bounded — none change the SPEC contract; all are
mechanical implementation choices.

---

*End of SPEC-MCP-001 plan v0.1.0 (draft).*
