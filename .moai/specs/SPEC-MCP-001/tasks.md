# SPEC-MCP-001 Task Decomposition

Status: in-progress
Methodology: TDD (RED-GREEN-REFACTOR)
Coverage target: 85%

## Task Table

| Task | Description | Status | RED Tests | Coverage |
|------|-------------|--------|-----------|----------|
| T1 | SDK Pin + Server Scaffolding | DONE | TestStdioInitializeRoundTrip, TestTransportFlagDefaultsStdio, TestVersionFlagPreserved | PASS |
| T2 | Tool Types + Error Mapping | DONE | TestErrorMappingTableComplete, TestToolSchemaDeterminism, TestErrorCodesInRange | PASS |
| T3 | list_sources Tool | DONE | TestListSourcesReturnsRegisteredAdapters, TestListSourcesSortDeterministic | PASS |
| T4 | get_citation Tool | DONE | TestGetCitationResolvesValidDocID, TestGetCitationNotFoundError | PASS |
| T5 | Shared Orchestrator Extraction | DONE | TestSharedSearchOrchestratorHappyPath, TestSharedSearchOrchestratorPartialFailure, TestSharedSearchOrchestratorNoAdapters | PASS |
| T6 | search Tool | DONE | TestSearchToolWrapsSharedOrchestrator, TestSearchToolEmptyRegistry | PASS |
| T7 | deep_research Tool + Cap Guard + Streaming | DONE | TestDeepResearchRoutesCapMiddleware, TestDeepResearchSharesQuotaWithHTTP, TestDeepResearchProgressStageCount, TestDeepResearchAuditLineSchema, TestDeepResearchEmptyQueryRejected, TestDeepResearchPipelineErrorMapped, TestDeepResearchOutputSchema, TestDeepResearchCappedAuditLine | PASS |
| T8 | HTTP Transport + Auth | DONE | TestHTTPInitializeWithMcpSessionId, TestHTTPSSEStreamOrdering, TestOriginRejectionMatrix(4 subtests), TestBindPublicRequiresAuth, TestAuthModeTrustHeadersPath | PASS |
| T9 | Observability | DONE | TestSpanParentChild, TestSpanSingleEnd, TestNoUnboundedLabelsMCP | PASS |
| T10 | Graceful Shutdown + NFR | DONE | TestGracefulShutdownInflight, TestColdStartUnderCap, TestShutdownLogRecord, TestGracePeriodConfig | PASS |

## Dependency Order

T1 (foundation) -> T2 (types) -> T3, T4 (read-only tools) -> T5 (orchestrator) -> T6 (search tool) -> T7 (deep_research) -> T8 (HTTP) -> T9 (obs) -> T10 (shutdown)

## Files Created

- internal/mcpserver/server.go
- internal/mcpserver/config.go
- internal/mcpserver/errors.go
- internal/mcpserver/audit.go
- internal/mcpserver/observability.go
- internal/mcpserver/shutdown.go
- internal/mcpserver/transport_http.go
- internal/mcpserver/tools/types.go
- internal/mcpserver/tools/list_sources.go
- internal/mcpserver/tools/get_citation.go
- internal/mcpserver/tools/search.go
- internal/mcpserver/tools/deep_research.go
- internal/mcpserver/tools/errors.go
- internal/orchestrator/search.go
- internal/mcpserver/server_test.go
- internal/mcpserver/transport_http_test.go
- internal/mcpserver/observability_test.go
- internal/mcpserver/shutdown_test.go
- internal/mcpserver/tools/types_test.go
- internal/mcpserver/tools/list_sources_test.go
- internal/mcpserver/tools/get_citation_test.go
- internal/mcpserver/tools/search_test.go
- internal/mcpserver/tools/deep_research_test.go

## Files Modified

- cmd/usearch-mcp/main.go (replace stub)
- cmd/usearch/query.go (thin wrapper over shared orchestrator)
- go.mod / go.sum (pin MCP SDK)
- internal/obs/metrics/metrics_test.go (extend allowlist)
