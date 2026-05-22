package mcpserver

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
)

// AuditLogger writes structured decision-event JSON lines to a writer.
// REQ-DEEP4-010: audit line schema conformance.
type AuditLogger struct {
	mu     sync.Mutex
	w      io.Writer
	logger *slog.Logger
}

// NewAuditLogger creates an AuditLogger that writes to w.
// If w is nil, defaults to os.Stdout.
func NewAuditLogger(w io.Writer, logger *slog.Logger) *AuditLogger {
	if w == nil {
		w = os.Stdout
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &AuditLogger{w: w, logger: logger}
}

// AuditEvent is a structured audit event for MCP tool calls.
type AuditEvent struct {
	EventType     string `json:"event_type"`
	ToolName      string `json:"tool_name"`
	MCPTransport  string `json:"mcp_transport"`
	ClientName    string `json:"client_name"`
	ClientVersion string `json:"client_version,omitempty"`
	Timestamp     string `json:"timestamp"`
	RequestID     string `json:"request_id"`
	Outcome       string `json:"outcome"`
	DurationMs    int64  `json:"duration_ms"`
	SessionID     string `json:"session_id,omitempty"`
}

// Write marshals the event and writes it as a single JSON line.
func (a *AuditLogger) Write(event AuditEvent) error {
	line, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("audit marshal: %w", err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if _, err := fmt.Fprintf(a.w, "%s\n", line); err != nil {
		return fmt.Errorf("audit write: %w", err)
	}
	return nil
}
