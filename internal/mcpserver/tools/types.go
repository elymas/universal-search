// Package tools contains the MCP tool definitions and handlers for Universal Search.
package tools

import "time"

// SearchInput is the input schema for the search tool.
type SearchInput struct {
	Query  string   `json:"query" jsonschema:"required,The search query text"`
	Source []string `json:"source,omitempty" jsonschema:"Optional list of adapter names to restrict search to"`
	Lang   string   `json:"lang,omitempty" jsonschema:"Optional language hint (ISO 639-1)"`
}

// SearchOutput is the output schema for the search tool.
type SearchOutput struct {
	Summary   string      `json:"summary"`
	Citations []Citation  `json:"citations"`
	Stats     SearchStats `json:"stats"`
}

// Citation represents a single citation in search results.
type Citation struct {
	DocID   string `json:"doc_id"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	Source  string `json:"source"`
	Snippet string `json:"snippet,omitempty"`
}

// SearchStats contains statistics about a search invocation.
type SearchStats struct {
	RequestID   string `json:"request_id"`
	LatencyMs   int64  `json:"latency_ms"`
	SourceCount int    `json:"source_count"`
}

// DeepResearchInput is the input schema for the deep_research tool.
type DeepResearchInput struct {
	Query string `json:"query" jsonschema:"required,The research query to investigate deeply"`
	Lang  string `json:"lang,omitempty" jsonschema:"Optional language hint (ISO 639-1)"`
}

// DeepResearchOutput is the output schema for the deep_research tool.
type DeepResearchOutput struct {
	Summary   string      `json:"summary"`
	Citations []Citation  `json:"citations"`
	Stats     SearchStats `json:"stats"`
	Report    string      `json:"report,omitempty"`
}

// ListSourcesOutput is the output schema for the list_sources tool.
type ListSourcesOutput struct {
	Sources []SourceEntry `json:"sources"`
}

// SourceEntry describes a registered adapter.
type SourceEntry struct {
	Name         string   `json:"name"`
	Category     string   `json:"category"`
	Language     []string `json:"language_support"`
	AuthRequired bool     `json:"auth_required"`
	Description  string   `json:"description"`
}

// GetCitationInput is the input schema for the get_citation tool.
type GetCitationInput struct {
	DocID string `json:"doc_id" jsonschema:"required,The document ID to resolve"`
}

// GetCitationOutput is the output schema for the get_citation tool.
type GetCitationOutput struct {
	DocID       string    `json:"doc_id"`
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	Source      string    `json:"source"`
	Snippet     string    `json:"snippet"`
	Score       float64   `json:"score"`
	RetrievedAt time.Time `json:"retrieved_at"`
}
