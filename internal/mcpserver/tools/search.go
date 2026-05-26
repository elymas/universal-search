package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/elymas/universal-search/internal/adapters"
	"github.com/elymas/universal-search/internal/orchestrator"
	"github.com/elymas/universal-search/pkg/types"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// SearchHandler returns a ToolHandlerFor that wraps the shared orchestrator.
func SearchHandler(reg *adapters.Registry, cache *DocCache) func(_ context.Context, _ *mcp.CallToolRequest, input SearchInput) (*mcp.CallToolResult, SearchOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input SearchInput) (*mcp.CallToolResult, SearchOutput, error) {
		start := time.Now()

		params := orchestrator.SearchParams{
			Query:   input.Query,
			Sources: input.Source,
			Timeout: 30 * time.Second,
		}

		result, err := orchestrator.Search(ctx, reg, params, synthFunc())
		if err != nil {
			return nil, SearchOutput{}, mapOrchestratorError(err, result)
		}

		// Store docs in cache for get_citation resolution.
		if cache != nil {
			cache.Store(result.Docs)
		}

		// Map citations.
		citations := make([]Citation, len(result.Citations))
		for i, c := range result.Citations {
			citations[i] = Citation{
				DocID:  c.DocID,
				Title:  c.Title,
				URL:    c.URL,
				Source: c.Source,
			}
		}

		latencyMs := time.Since(start).Milliseconds()
		output := SearchOutput{
			Summary:   result.Summary,
			Citations: citations,
			Stats: SearchStats{
				LatencyMs:   latencyMs,
				SourceCount: len(result.AdapterSet),
			},
		}

		return nil, output, nil
	}
}

// SearchTool returns the MCP tool definition for search.
func SearchTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "search",
		Description: "Search across multiple sources and synthesize a cited summary",
	}
}

// synthFunc returns a synthesis function for the orchestrator.
// V1: uses the real synthesis client when available, or returns a no-op.
// TBD: wire to actual synthesis.Client based on configuration.
func synthFunc() orchestrator.SynthFunc {
	return func(ctx context.Context, query, lang string, docs []types.NormalizedDoc) (string, []orchestrator.Citation, error) {
		// V1: return a basic summary without LLM synthesis.
		// Real synthesis will be wired through the shared orchestrator.
		b, _ := json.Marshal(map[string]any{
			"query": query,
			"docs":  len(docs),
		})
		return string(b), nil, nil
	}
}

// mapOrchestratorError maps orchestrator errors to MCP errors per REQ-MCP-016.
func mapOrchestratorError(err error, result *orchestrator.SearchResult) *MCPError {
	errMsg := err.Error()
	switch errMsg {
	case "orchestrator: no adapters matched":
		cat := ""
		if result != nil {
			cat = result.Category
		}
		return NoAdaptersMatchedError(cat)
	case "orchestrator: all adapters failed":
		errs := make([]string, 0)
		if result != nil {
			for name, aerr := range result.AdapterErrors {
				errs = append(errs, fmt.Sprintf("%s: %s", name, aerr.Error()))
			}
		}
		return AllAdaptersFailedError(errs)
	default:
		return MapError(err)
	}
}
