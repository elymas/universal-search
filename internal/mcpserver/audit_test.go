package mcpserver

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

// TestNewAuditLoggerDefaults verifies that NewAuditLogger defaults to os.Stdout
// when writer is nil and slog.Default when logger is nil.
func TestNewAuditLoggerDefaults(t *testing.T) {
	al := NewAuditLogger(nil, nil)
	if al == nil {
		t.Fatal("expected non-nil AuditLogger")
	}
	if al.w == nil {
		t.Error("expected writer to default to os.Stdout, got nil")
	}
	if al.logger == nil {
		t.Error("expected logger to default to slog.Default, got nil")
	}
}

// TestNewAuditLoggerCustomWriter verifies that a custom writer is preserved.
func TestNewAuditLoggerCustomWriter(t *testing.T) {
	var buf bytes.Buffer
	al := NewAuditLogger(&buf, slog.Default())
	if al == nil {
		t.Fatal("expected non-nil AuditLogger")
	}
}

// TestAuditLoggerWrite verifies that Write marshals the event and writes it
// as a single JSON line.
func TestAuditLoggerWrite(t *testing.T) {
	var buf bytes.Buffer
	al := NewAuditLogger(&buf, slog.Default())

	event := AuditEvent{
		EventType:    "mcp.tool_call",
		ToolName:     "search",
		MCPTransport: "stdio",
		ClientName:   "test-client",
		Timestamp:    "2026-01-01T00:00:00Z",
		RequestID:    "req-001",
		Outcome:      "success",
		DurationMs:   42,
	}

	if err := al.Write(event); err != nil {
		t.Fatalf("Write: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, `"tool_name":"search"`) {
		t.Errorf("expected tool_name in output, got %q", output)
	}
	if !strings.HasSuffix(output, "\n") {
		t.Error("expected output to end with newline")
	}

	// Verify the output is valid JSON.
	var parsed map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%q", err, output)
	}
	if parsed["event_type"] != "mcp.tool_call" {
		t.Errorf("event_type: got %v, want mcp.tool_call", parsed["event_type"])
	}
	if parsed["outcome"] != "success" {
		t.Errorf("outcome: got %v, want success", parsed["outcome"])
	}
}

// TestAuditLoggerWriteWithOptionalFields verifies that optional fields
// (client_version, session_id) are included when set.
func TestAuditLoggerWriteWithOptionalFields(t *testing.T) {
	var buf bytes.Buffer
	al := NewAuditLogger(&buf, slog.Default())

	event := AuditEvent{
		EventType:     "mcp.tool_call",
		ToolName:      "deep_research",
		MCPTransport:  "http",
		ClientName:    "test-client",
		ClientVersion: "1.0.0",
		Timestamp:     "2026-01-01T00:00:00Z",
		RequestID:     "req-002",
		Outcome:       "capped",
		DurationMs:    10,
		SessionID:     "sess-abc",
	}

	if err := al.Write(event); err != nil {
		t.Fatalf("Write: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed["client_version"] != "1.0.0" {
		t.Errorf("client_version: got %v, want 1.0.0", parsed["client_version"])
	}
	if parsed["session_id"] != "sess-abc" {
		t.Errorf("session_id: got %v, want sess-abc", parsed["session_id"])
	}
}

// TestAuditLoggerWriteConcurrent verifies that concurrent Write calls do not
// interleave output lines.
func TestAuditLoggerWriteConcurrent(t *testing.T) {
	var buf bytes.Buffer
	al := NewAuditLogger(&buf, slog.Default())

	const n = 50
	done := make(chan struct{})

	for i := range n {
		go func() {
			defer func() { done <- struct{}{} }()
			_ = al.Write(AuditEvent{
				EventType: "mcp.tool_call",
				ToolName:  "search",
				RequestID: "concurrent",
				Outcome:   "success",
			})
		}()
		_ = i
	}

	for range n {
		<-done
	}

	// Each line should be valid JSON.
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != n {
		t.Fatalf("expected %d lines, got %d", n, len(lines))
	}
	for i, line := range lines {
		var parsed map[string]any
		if err := json.Unmarshal([]byte(line), &parsed); err != nil {
			t.Errorf("line %d is not valid JSON: %v\n%q", i, err, line)
		}
	}
}
