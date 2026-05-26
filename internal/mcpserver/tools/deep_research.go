package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/elymas/universal-search/internal/deepagent"
	"github.com/elymas/universal-search/internal/deepagent/costguard"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// CapChecker is the interface for quota enforcement. Matches costguard.CapChecker
// signature for duck-typing; MCP server injects the real implementation.
type CapChecker interface {
	EvaluateAtomic(ctx context.Context, tenantID, userID string, costUSD float64) (costguard.CapResult, error)
}

// PipelineFn runs the deep agent multi-agent pipeline.
type PipelineFn func(ctx context.Context, req deepagent.PipelineRequest) (deepagent.PipelineResult, error)

// NotifyFn emits a progress notification (notifications/message).
type NotifyFn func(method string, data map[string]any)

// AuditFn writes a structured audit log line.
type AuditFn func(line string) error

// DeepResearchTool returns the MCP tool definition for deep_research.
func DeepResearchTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "deep_research",
		Description: "Perform deep multi-agent research with citation-backed synthesis",
	}
}

// DeepResearchHandler returns a tool handler that:
// 1. Validates input
// 2. Checks quota via CapChecker (shares counters with HTTP /deep)
// 3. Runs the multi-agent pipeline
// 4. Emits progress notifications at each stage boundary
// 5. Writes an audit log line
//
// REQ-MCP-009: deep_research tool with cap guard integration.
// REQ-MCP-010: progress events at pipeline stage boundaries.
// REQ-DEEP4-010: audit line schema conformance.
func DeepResearchHandler(capCheck CapChecker, pipeline PipelineFn, notify NotifyFn, audit AuditFn) func(_ context.Context, _ *mcp.CallToolRequest, input DeepResearchInput) (*mcp.CallToolResult, DeepResearchOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input DeepResearchInput) (*mcp.CallToolResult, DeepResearchOutput, error) {
		start := time.Now()
		requestID := fmt.Sprintf("mcp-dr-%d", start.UnixMilli())

		// Stage 0: validate input.
		if input.Query == "" {
			_ = writeAuditLine(audit, auditEntry{
				EventType:    "mcp.tool_call",
				ToolName:     "deep_research",
				MCPTransport: "stdio",
				ClientName:   "unknown",
				Timestamp:    start.UTC().Format(time.RFC3339),
				RequestID:    requestID,
				Outcome:      "error_invalid",
				DurationMs:   time.Since(start).Milliseconds(),
			})
			return nil, DeepResearchOutput{}, InputSchemaViolationError("query", "required")
		}

		// Emit progress: validation complete.
		emitProgress(notify, "validation_complete", requestID)

		// Stage 1: cap check (shares quota with HTTP /deep endpoint).
		capResult, err := capCheck.EvaluateAtomic(ctx, "default", "", 0.05)
		if err != nil {
			// Redis failure: fail-closed (do not proceed).
			return nil, DeepResearchOutput{}, MapError(err)
		}

		if !capResult.Allowed {
			_ = writeAuditLine(audit, auditEntry{
				EventType:    "mcp.tool_call",
				ToolName:     "deep_research",
				MCPTransport: "stdio",
				ClientName:   "unknown",
				Timestamp:    start.UTC().Format(time.RFC3339),
				RequestID:    requestID,
				Outcome:      "capped",
				DurationMs:   time.Since(start).Milliseconds(),
				CapDimension: string(capResult.Exceeded),
				Remaining:    capResult.RemainingCalls,
			})
			return nil, DeepResearchOutput{}, CapExceededError(
				string(capResult.Exceeded),
				capResult.RemainingCalls,
				"", // reset_at not available from CapResult
				86400,
			)
		}

		// Emit progress: cap check passed.
		emitProgress(notify, "cap_check_passed", requestID)

		// Stage 2: run pipeline.
		emitProgress(notify, "pipeline_started", requestID)

		req := deepagent.PipelineRequest{
			RequestID: requestID,
			Query:     input.Query,
			Lang:      input.Lang,
		}

		result, err := pipeline(ctx, req)
		if err != nil {
			_ = writeAuditLine(audit, auditEntry{
				EventType:    "mcp.tool_call",
				ToolName:     "deep_research",
				MCPTransport: "stdio",
				ClientName:   "unknown",
				Timestamp:    start.UTC().Format(time.RFC3339),
				RequestID:    requestID,
				Outcome:      "error_pipeline_failed",
				DurationMs:   time.Since(start).Milliseconds(),
			})
			return nil, DeepResearchOutput{}, MapError(err)
		}

		// Emit progress: pipeline complete.
		emitProgress(notify, "pipeline_complete", requestID)

		// Stage 3: build output.
		output := buildDeepResearchOutput(result, start)

		// Emit progress: synthesis complete.
		emitProgress(notify, "synthesis_complete", requestID)

		// Write audit line.
		outcome := "success"
		if result.IsEmpty {
			outcome = "empty_corpus"
		}
		_ = writeAuditLine(audit, auditEntry{
			EventType:    "mcp.tool_call",
			ToolName:     "deep_research",
			MCPTransport: "stdio",
			ClientName:   "unknown",
			Timestamp:    start.UTC().Format(time.RFC3339),
			RequestID:    requestID,
			Outcome:      outcome,
			DurationMs:   time.Since(start).Milliseconds(),
		})

		return nil, output, nil
	}
}

// emitProgress sends a notifications/message progress event.
func emitProgress(notify NotifyFn, stage, requestID string) {
	notify("notifications/message", map[string]any{
		"stage":      stage,
		"request_id": requestID,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	})
}

// auditEntry is the structured audit log entry.
type auditEntry struct {
	EventType    string `json:"event_type"`
	ToolName     string `json:"tool_name"`
	MCPTransport string `json:"mcp_transport"`
	ClientName   string `json:"client_name"`
	Timestamp    string `json:"timestamp"`
	RequestID    string `json:"request_id"`
	Outcome      string `json:"outcome"`
	DurationMs   int64  `json:"duration_ms"`
	CapDimension string `json:"cap_dimension,omitempty"`
	Remaining    int    `json:"remaining,omitempty"`
}

// writeAuditLine marshals the audit entry and writes it.
func writeAuditLine(audit AuditFn, entry auditEntry) error {
	line, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("audit marshal: %w", err)
	}
	return audit(string(line))
}

// buildDeepResearchOutput maps a PipelineResult to a DeepResearchOutput.
func buildDeepResearchOutput(result deepagent.PipelineResult, start time.Time) DeepResearchOutput {
	output := DeepResearchOutput{
		Stats: SearchStats{
			RequestID: result.RequestID,
			LatencyMs: time.Since(start).Milliseconds(),
		},
	}

	if result.Draft != nil {
		// Build report from sections.
		report := ""
		for _, sec := range result.Draft.Sections {
			report += fmt.Sprintf("## %s\n\n%s\n\n", sec.Heading, sec.Text)
		}
		output.Report = report
		output.Summary = report // Summary is the same as report for now.

		// Map citations.
		output.Citations = make([]Citation, len(result.Draft.Citations))
		for i, c := range result.Draft.Citations {
			output.Citations[i] = Citation{
				DocID:  c.DocID,
				Title:  c.Title,
				URL:    c.URL,
				Source: "deep_research",
			}
		}
	}

	return output
}

// noopLogger returns an slog.Logger that discards all output.
func noopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(nil, nil))
}
