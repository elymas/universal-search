package rbac

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"
)

// AuditEntry represents a single audit log entry for RBAC decisions.
// AUTH-003 forward-compat: schema stable for future log aggregation.
type AuditEntry struct {
	Timestamp   string `json:"ts"`
	UserID      string `json:"user_id"`
	TeamID      string `json:"team_id"`
	Resource    string `json:"resource"`
	Action      string `json:"action"`
	Allowed     bool   `json:"allowed"`
	ReasonClass string `json:"reason_class"`
}

// AuditEmitter writes structured JSON audit entries to a writer.
// NFR-AUTH2-004: Audit log uses isolated stderr output (not hot-path pgxpool).
type AuditEmitter struct {
	mu     sync.Mutex
	w      io.Writer
	enabled bool
}

// NewAuditEmitter creates an audit emitter that writes JSON lines to w.
// If w is nil, defaults to os.Stderr.
func NewAuditEmitter(w io.Writer, enabled bool) *AuditEmitter {
	if w == nil {
		w = os.Stderr
	}
	return &AuditEmitter{w: w, enabled: enabled}
}

// Emit writes an audit entry as a JSON line.
// No-op when emitter is disabled (AuditToStderr = false).
func (e *AuditEmitter) Emit(d Decision) {
	if e == nil || !e.enabled {
		return
	}

	entry := AuditEntry{
		Timestamp:   time.Now().UTC().Format(time.RFC3339Nano),
		UserID:      d.UserID,
		TeamID:      d.TeamID,
		Resource:    d.Resource,
		Action:      d.Action,
		Allowed:     d.Allowed,
		ReasonClass: d.ReasonClass,
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	data, err := json.Marshal(entry)
	if err != nil {
		slog.Error("rbac: audit marshal failed", "error", err)
		return
	}
	_, _ = e.w.Write(append(data, '\n'))
}

// Enabled reports whether audit logging is active.
func (e *AuditEmitter) Enabled() bool {
	if e == nil {
		return false
	}
	return e.enabled
}
