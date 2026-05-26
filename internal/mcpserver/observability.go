package mcpserver

import (
	"fmt"
)

// toolSpanName returns the OTel span name for an MCP tool call.
// REQ-MCP-012: name mcp.tool.{tool_name}.
func toolSpanName(toolName string) string {
	return fmt.Sprintf("mcp.tool.%s", toolName)
}

// isValidMCPLabel checks whether a label name is a recognized bounded MCP label.
// NFR-OBS-002: no unbounded Prometheus labels.
func isValidMCPLabel(label string) bool {
	validLabels := map[string]bool{
		"mcp_transport": true, // bounded: {stdio, http}
		"mcp_tool_name": true, // bounded: {search, deep_research, list_sources, get_citation}
		"mcp_outcome":   true, // bounded: {success, error, capped}
	}
	return validLabels[label]
}
