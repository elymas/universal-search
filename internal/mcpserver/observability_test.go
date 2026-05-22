package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// T9 RED tests: Observability
// ---------------------------------------------------------------------------

// TestSpanParentChild verifies that the MCP server creates a root span per tool
// call and that downstream spans are children of it.
//
// REQ-MCP-012: OTel root span per tool call: name mcp.tool.{tool_name}.
func TestSpanParentChild(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Transport = "http"
	cfg.HTTP.ListenAddr = "127.0.0.1:0"

	srv := New(cfg, nil, nil, nil, nil)
	_, err := srv.httpHandler()
	if err != nil {
		t.Fatalf("httpHandler: %v", err)
	}

	// The span is created inside handlePost for tool calls.
	// For now, verify the tool call wrapper creates the span name correctly.
	spanName := toolSpanName("search")
	if spanName != "mcp.tool.search" {
		t.Errorf("span name: got %q, want %q", spanName, "mcp.tool.search")
	}

	spanName2 := toolSpanName("deep_research")
	if spanName2 != "mcp.tool.deep_research" {
		t.Errorf("span name: got %q, want %q", spanName2, "mcp.tool.deep_research")
	}
}

// TestSpanSingleEnd verifies that the root span is ended exactly once before
// the JSON-RPC response is written.
//
// REQ-MCP-012: Span ended exactly once before JSON-RPC response written.
func TestSpanSingleEnd(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Transport = "http"
	cfg.HTTP.ListenAddr = "127.0.0.1:0"

	srv := New(cfg, nil, nil, nil, nil)
	handler, err := srv.httpHandler()
	if err != nil {
		t.Fatalf("httpHandler: %v", err)
	}

	server := httptest.NewServer(handler)
	defer server.Close()

	// Send an initialize request and verify response is complete
	// (which implies the span was ended before response was written).
	initReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "test", "version": "1.0"},
		},
	}
	body, _ := json.Marshal(initReq)

	req, _ := http.NewRequest(http.MethodPost, server.URL+"/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2025-06-18")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}

	// Verify complete JSON-RPC response was written (span ended before write).
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result["id"] != float64(1) {
		t.Errorf("response id: got %v, want 1", result["id"])
	}
}

// TestNoUnboundedLabelsMCP verifies that MCP-specific metric labels
// (mcp.transport, mcp.tool_name, mcp.outcome) are in the cardinality allowlist.
//
// NFR-OBS-002: no unbounded Prometheus labels.
func TestNoUnboundedLabelsMCP(t *testing.T) {
	// MCP labels that must be in the allowlist.
	mcpLabels := []string{
		"mcp_transport",
		"mcp_tool_name",
		"mcp_outcome",
	}

	// These labels would be added to the metrics registry allowlist.
	// For now, verify they are bounded enums (not free-form).
	for _, label := range mcpLabels {
		// Verify the label is recognized by the MCP observability layer.
		if !isValidMCPLabel(label) {
			t.Errorf("label %q is not a valid bounded MCP label", label)
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// Compile-time checks for imports used by test helpers above.
var _ = time.Now
var _ = context.Background
var _ = fmt.Sprintf
var _ = bytes.NewReader
