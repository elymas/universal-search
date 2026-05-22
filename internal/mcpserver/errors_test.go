package mcpserver

import (
	"encoding/json"
	"testing"

	"github.com/elymas/universal-search/internal/mcpserver/tools"
)

// TestErrorMappingTableComplete verifies every error mapping from REQ-MCP-016.
func TestErrorMappingTableComplete(t *testing.T) {
	tests := []struct {
		name       string
		err        *tools.MCPError
		wantCode   int
		wantNS     string
		wantFields []string
	}{
		{
			name: "cap_exceeded", err: tools.CapExceededError("deep_research", 0, "2026-01-01T00:00:00Z", 86400),
			wantCode: -32000, wantNS: "usearch.cap_exceeded",
			wantFields: []string{"namespace", "dimension", "remaining", "reset_at", "retry_after_s"},
		},
		{
			name: "deep_not_warranted", err: tools.DeepNotWarrantedError(0.3, "basic", "query too simple"),
			wantCode: -32001, wantNS: "usearch.deep_not_warranted",
			wantFields: []string{"namespace", "screen_score", "suggested_mode", "rationale"},
		},
		{
			name: "unauthorized", err: tools.UnauthorizedError("expired JWT"),
			wantCode: -32002, wantNS: "usearch.unauthorized",
			wantFields: []string{"namespace", "reason"},
		},
		{
			name: "no_adapters_matched", err: tools.NoAdaptersMatchedError("technical"),
			wantCode: -32003, wantNS: "usearch.no_adapters_matched",
			wantFields: []string{"namespace", "query_category"},
		},
		{
			name: "all_adapters_failed", err: tools.AllAdaptersFailedError([]string{"reddit: timeout", "hn: 500"}),
			wantCode: -32004, wantNS: "usearch.all_adapters_failed",
			wantFields: []string{"namespace", "errors"},
		},
		{
			name: "synthesis_degraded", err: tools.SynthesisDegradedError("LLM timeout"),
			wantCode: -32005, wantNS: "usearch.synthesis_degraded",
			wantFields: []string{"namespace", "degraded_reason"},
		},
		{
			name: "timeout", err: tools.TimeoutError("fanout", 30000),
			wantCode: -32006, wantNS: "usearch.timeout",
			wantFields: []string{"namespace", "stage", "deadline_ms"},
		},
		{
			name: "citation_not_found", err: tools.CitationNotFoundError("doc-123"),
			wantCode: -32007, wantNS: "usearch.citation_not_found",
			wantFields: []string{"namespace", "doc_id"},
		},
		{
			name: "tool_not_enabled", err: tools.ToolNotEnabledError("search"),
			wantCode: -32601, wantNS: "",
			wantFields: []string{"tool_name"},
		},
		{
			name: "input_schema_violation", err: tools.InputSchemaViolationError("query", "required"),
			wantCode: -32602, wantNS: "",
			wantFields: []string{"field", "reason"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.wantCode {
				t.Errorf("code: got %d, want %d", tt.err.Code, tt.wantCode)
			}
			if tt.err.Namespace != tt.wantNS {
				t.Errorf("namespace: got %q, want %q", tt.err.Namespace, tt.wantNS)
			}
			data := tt.err.Data
			for _, field := range tt.wantFields {
				if _, ok := data[field]; !ok {
					t.Errorf("missing data field %q", field)
				}
			}
			if tt.wantNS != "" {
				if ns, _ := data["namespace"].(string); ns != tt.wantNS {
					t.Errorf("data.namespace: got %q, want %q", ns, tt.wantNS)
				}
			}
			b, err := json.Marshal(tt.err)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var parsed map[string]any
			if err := json.Unmarshal(b, &parsed); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			code, _ := parsed["code"].(float64)
			if int(code) != tt.wantCode {
				t.Errorf("JSON code: got %v, want %d", code, tt.wantCode)
			}
		})
	}
}

// TestErrorCodesInRange verifies all server-defined codes are in -32099..-32000.
func TestErrorCodesInRange(t *testing.T) {
	codes := []int{
		tools.ErrCodeCapExceeded,
		tools.ErrCodeDeepNotWarranted,
		tools.ErrCodeUnauthorized,
		tools.ErrCodeNoAdaptersMatched,
		tools.ErrCodeAllAdaptersFailed,
		tools.ErrCodeSynthesisDegraded,
		tools.ErrCodeTimeout,
		tools.ErrCodeCitationNotFound,
	}
	for _, code := range codes {
		if code < -32099 || code > -32000 {
			t.Errorf("code %d outside range [-32099, -32000]", code)
		}
	}
}
