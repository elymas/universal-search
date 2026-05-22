package tools

import (
	"encoding/json"
	"fmt"
	"testing"
)

// TestMCPErrorError verifies the Error() method format.
func TestMCPErrorError(t *testing.T) {
	err := &MCPError{
		Code:      -32000,
		Message:   "Cap exceeded",
		Namespace: "usearch.cap_exceeded",
	}
	got := err.Error()
	want := "MCP error -32000: Cap exceeded (usearch.cap_exceeded)"
	if got != want {
		t.Errorf("Error(): got %q, want %q", got, want)
	}
}

// TestMCPErrorToJSONRPC verifies ToJSONRPC output structure.
func TestMCPErrorToJSONRPC(t *testing.T) {
	tests := []struct {
		name       string
		err        *MCPError
		wantKeys   []string
		wantNoData bool
	}{
		{
			name:     "with_namespace_and_data",
			err:      &MCPError{Code: -32000, Message: "test", Namespace: "ns", Data: map[string]any{"key": "val"}},
			wantKeys: []string{"code", "message", "data"},
		},
		{
			name:     "with_namespace_only",
			err:      &MCPError{Code: -32001, Message: "test", Namespace: "ns"},
			wantKeys: []string{"code", "message", "data"},
		},
		{
			name:     "with_data_only",
			err:      &MCPError{Code: -32002, Message: "test", Data: map[string]any{"k": "v"}},
			wantKeys: []string{"code", "message", "data"},
		},
		{
			name:       "no_namespace_no_data",
			err:        &MCPError{Code: -32601, Message: "Method not found"},
			wantNoData: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := tt.err.ToJSONRPC()
			if obj["code"] != tt.err.Code {
				t.Errorf("code: got %v, want %d", obj["code"], tt.err.Code)
			}
			if obj["message"] != tt.err.Message {
				t.Errorf("message: got %v, want %q", obj["message"], tt.err.Message)
			}

			if tt.wantNoData {
				if _, ok := obj["data"]; ok {
					t.Error("expected no data field")
				}
			} else {
				data, ok := obj["data"].(map[string]any)
				if !ok {
					t.Fatal("expected data to be map[string]any")
				}
				if tt.err.Namespace != "" {
					if data["namespace"] != tt.err.Namespace {
						t.Errorf("data.namespace: got %v, want %q", data["namespace"], tt.err.Namespace)
					}
				}
			}
		})
	}
}

// TestMCPErrorMarshalJSON verifies MarshalJSON produces valid JSON matching ToJSONRPC.
func TestMCPErrorMarshalJSON(t *testing.T) {
	err := &MCPError{
		Code:      -32000,
		Message:   "Cap exceeded",
		Namespace: "usearch.cap_exceeded",
		Data:      map[string]any{"dimension": "calls"},
	}

	b, marshalErr := json.Marshal(err)
	if marshalErr != nil {
		t.Fatalf("MarshalJSON: %v", marshalErr)
	}

	var parsed map[string]any
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if int(parsed["code"].(float64)) != -32000 {
		t.Errorf("code: got %v, want -32000", parsed["code"])
	}
	if parsed["message"] != "Cap exceeded" {
		t.Errorf("message: got %v", parsed["message"])
	}

	data, _ := parsed["data"].(map[string]any)
	if data["namespace"] != "usearch.cap_exceeded" {
		t.Errorf("data.namespace: got %v", data["namespace"])
	}
}

// TestMapErrorNil verifies MapError returns nil for nil input.
func TestMapErrorNil(t *testing.T) {
	if result := MapError(nil); result != nil {
		t.Error("expected nil for nil input")
	}
}

// TestMapErrorGeneric verifies MapError wraps arbitrary errors.
func TestMapErrorGeneric(t *testing.T) {
	inner := fmt.Errorf("something broke")
	result := MapError(inner)
	if result == nil {
		t.Fatal("expected non-nil MCPError")
	}
	if result.Code != -32603 {
		t.Errorf("code: got %d, want -32603", result.Code)
	}
	if result.Message != "Internal error" {
		t.Errorf("message: got %q", result.Message)
	}
	if result.Namespace != "usearch.internal" {
		t.Errorf("namespace: got %q", result.Namespace)
	}
	if result.Data["reason"] != "something broke" {
		t.Errorf("reason: got %v", result.Data["reason"])
	}
}

// TestErrorConstructors verifies all error constructor functions return MCPError
// with the correct code and namespace.
func TestErrorConstructors(t *testing.T) {
	tests := []struct {
		name    string
		err     *MCPError
		code    int
		ns      string
		dataKey string
	}{
		{"deep_not_warranted", DeepNotWarrantedError(0.3, "basic", "too simple"), -32001, "usearch.deep_not_warranted", "screen_score"},
		{"unauthorized", UnauthorizedError("bad token"), -32002, "usearch.unauthorized", "reason"},
		{"no_adapters_matched", NoAdaptersMatchedError("tech"), -32003, "usearch.no_adapters_matched", "query_category"},
		{"all_adapters_failed", AllAdaptersFailedError([]string{"e1"}), -32004, "usearch.all_adapters_failed", "errors"},
		{"synthesis_degraded", SynthesisDegradedError("timeout"), -32005, "usearch.synthesis_degraded", "degraded_reason"},
		{"timeout", TimeoutError("fanout", 30000), -32006, "usearch.timeout", "stage"},
		{"citation_not_found", CitationNotFoundError("doc-1"), -32007, "usearch.citation_not_found", "doc_id"},
		{"tool_not_enabled", ToolNotEnabledError("search"), -32601, "", "tool_name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.code {
				t.Errorf("code: got %d, want %d", tt.err.Code, tt.code)
			}
			if tt.err.Namespace != tt.ns {
				t.Errorf("namespace: got %q, want %q", tt.err.Namespace, tt.ns)
			}
			if _, ok := tt.err.Data[tt.dataKey]; !ok {
				t.Errorf("missing data key %q", tt.dataKey)
			}
			// Verify Error() method works.
			if tt.err.Error() == "" {
				t.Error("Error() returned empty string")
			}
			// Verify MarshalJSON works.
			b, err := json.Marshal(tt.err)
			if err != nil {
				t.Fatalf("MarshalJSON: %v", err)
			}
			if len(b) == 0 {
				t.Error("MarshalJSON returned empty bytes")
			}
		})
	}
}

// TestToJSONRPCDataCopy verifies that ToJSONRPC creates a copy of the Data map,
// not a reference.
func TestToJSONRPCDataCopy(t *testing.T) {
	err := &MCPError{
		Code:    -32000,
		Message: "test",
		Data:    map[string]any{"key": "original"},
	}

	obj := err.ToJSONRPC()
	data, _ := obj["data"].(map[string]any)
	data["key"] = "modified"

	// Original should be unchanged.
	if err.Data["key"] != "original" {
		t.Error("ToJSONRPC should not mutate original Data map")
	}
}
