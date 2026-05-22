package tools

import (
	"context"
	"sort"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/elymas/universal-search/internal/adapters"
	"github.com/elymas/universal-search/pkg/types"
)

// ListSourcesHandler returns a ToolHandlerFor that lists registered adapters.
func ListSourcesHandler(reg *adapters.Registry) func(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, ListSourcesOutput, error) {
	return func(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, ListSourcesOutput, error) {
		names := reg.List()
		sources := make([]SourceEntry, 0, len(names))

		for _, name := range names {
			adapter, ok := reg.Get(name)
			if !ok {
				continue
			}
			caps := adapter.Capabilities()
			sources = append(sources, SourceEntry{
				Name:         name,
				Category:     firstDocType(caps.DocTypes),
				Language:     caps.SupportedLangs,
				AuthRequired: caps.RequiresAuth,
				Description:  caps.DisplayName,
			})
		}

		// Sort by name for determinism (REQ-MCP-011).
		sort.Slice(sources, func(i, j int) bool {
			return sources[i].Name < sources[j].Name
		})

		return nil, ListSourcesOutput{Sources: sources}, nil
	}
}

// ListSourcesTool returns the MCP tool definition for list_sources.
func ListSourcesTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "list_sources",
		Description: "List all registered search adapters with their capabilities",
	}
}

// firstDocType returns the first doc type or "other" if empty.
func firstDocType(docTypes []types.DocType) string {
	if len(docTypes) > 0 {
		return string(docTypes[0])
	}
	return "other"
}
