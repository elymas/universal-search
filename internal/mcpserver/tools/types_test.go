package tools

import (
	"encoding/json"
	"testing"
)

// TestToolSchemaDeterminism verifies that serializing tool types twice
// produces the same bytes (REQ-MCP-007: deterministic schemas).
func TestToolSchemaDeterminism(t *testing.T) {
	types := []struct {
		name string
		v    any
	}{
		{"SearchInput", SearchInput{Query: "test query", Source: []string{"reddit"}, Lang: "en"}},
		{"SearchOutput", SearchOutput{Summary: "result", Citations: []Citation{{DocID: "d1", Title: "t1", URL: "http://example.com", Source: "reddit"}}, Stats: SearchStats{RequestID: "r1", LatencyMs: 100, SourceCount: 3}}},
		{"DeepResearchInput", DeepResearchInput{Query: "deep query"}},
		{"ListSourcesOutput", ListSourcesOutput{Sources: []SourceEntry{{Name: "reddit", Category: "social", Language: []string{"en"}, AuthRequired: false, Description: "Reddit search"}}}},
		{"GetCitationInput", GetCitationInput{DocID: "doc-123"}},
		{"GetCitationOutput", GetCitationOutput{DocID: "doc-123", Title: "Test", URL: "http://example.com", Source: "reddit", Snippet: "text", Score: 0.95}},
	}

	for _, tt := range types {
		t.Run(tt.name, func(t *testing.T) {
			first, err := json.Marshal(tt.v)
			if err != nil {
				t.Fatalf("first marshal: %v", err)
			}
			second, err := json.Marshal(tt.v)
			if err != nil {
				t.Fatalf("second marshal: %v", err)
			}
			if string(first) != string(second) {
				t.Errorf("non-deterministic serialization:\nfirst:  %s\nsecond: %s", first, second)
			}
		})
	}
}
