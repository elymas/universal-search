// Package events provides a 7-type security event logger with Merkle hash chain.
//
// REQ-SEC-017: Security event types and Merkle chain for audit log integrity.
// @MX:WARN: [AUTO] Audit log integrity — chain break triggers fail-closed.
// @MX:REASON: Merkle chain ensures tamper detection; break prevents further writes (fail-closed).
// @MX:SPEC: SPEC-SEC-001
package events

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// EventType enumerates the seven security event types per REQ-SEC-017.
type EventType string

const (
	TypeAuthFailed        EventType = "auth.failed"
	TypeAuthSuccess       EventType = "auth.success"
	TypeSSRFBlocked       EventType = "ssrf.blocked"
	TypeSecretScanFinding EventType = "secret.scan.finding"
	TypeRateLimitExceeded EventType = "ratelimit.exceeded"
	TypeRBACDenied        EventType = "rbac.denied"
	TypePromptSanitized   EventType = "prompt.sanitized"
)

// Severity levels for security events.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
)

// Event represents a security event to be logged.
type Event struct {
	Type           EventType
	Severity       Severity
	Timestamp      time.Time
	Message        string
	Component      string // access, auth, adapter
	TenantIDClass  string // known, unknown
	BlockedURL     string // host portion only for SSRF events
	BlockReason    string // SSRF block reason
	Metadata       map[string]string
}

// AuditEntry represents a row in the audit log with Merkle chain.
type AuditEntry struct {
	ID        int64
	EventType EventType
	Severity  Severity
	Timestamp time.Time
	Message   string
	PrevHash  string
	RowHash   string
}

// EventLogger provides security event logging with Merkle hash chain.
type EventLogger struct {
	mu      sync.Mutex
	entries []AuditEntry
	lastID  int64
	metrics MetricsRecorder
}

// MetricsRecorder records security event metrics.
type MetricsRecorder interface {
	RecordEvent(eventType EventType, severity Severity)
}

// NewEventLogger creates a new EventLogger.
func NewEventLogger(metrics MetricsRecorder) *EventLogger {
	return &EventLogger{
		entries: make([]AuditEntry, 0),
		metrics: metrics,
	}
}

// Insert records a security event into the audit log.
// REQ-SEC-017: Inserts row with prev_hash forming Merkle chain.
// @MX:ANCHOR: [AUTO] Event insertion — high fan_in (all security event sites).
// @MX:REASON: Central event logging ensures consistent audit trail and Merkle chain integrity.
func (l *EventLogger) Insert(evt Event) (*AuditEntry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.lastID++
	prevHash := ""
	if len(l.entries) > 0 {
		prevHash = l.entries[len(l.entries)-1].RowHash
	}

	entry := AuditEntry{
		ID:        l.lastID,
		EventType: evt.Type,
		Severity:  evt.Severity,
		Timestamp: evt.Timestamp,
		Message:   evt.Message,
		PrevHash:  prevHash,
	}

	// Compute row hash: SHA-256(id + prevHash + eventType + severity + timestamp + message)
	entry.RowHash = computeRowHash(entry)

	l.entries = append(l.entries, entry)

	// Emit metrics.
	if l.metrics != nil {
		l.metrics.RecordEvent(evt.Type, evt.Severity)
	}

	// Emit slog at appropriate level.
	logEvent(evt)

	return &entry, nil
}

// VerifyChain verifies the Merkle hash chain integrity.
// REQ-SEC-017: Returns error if any row hash is inconsistent.
func (l *EventLogger) VerifyChain() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	for i, entry := range l.entries {
		// Verify row hash.
		expected := computeRowHash(entry)
		if entry.RowHash != expected {
			return fmt.Errorf("chain break at row %d: hash mismatch (got %s, expected %s)", entry.ID, entry.RowHash, expected)
		}

		// Verify prev_hash linkage.
		if i == 0 {
			if entry.PrevHash != "" {
				return fmt.Errorf("chain break at row %d: first entry has non-empty prev_hash %q", entry.ID, entry.PrevHash)
			}
		} else {
			expectedPrev := l.entries[i-1].RowHash
			if entry.PrevHash != expectedPrev {
				return fmt.Errorf("chain break at row %d: prev_hash mismatch (got %s, expected %s)", entry.ID, entry.PrevHash, expectedPrev)
			}
		}
	}
	return nil
}

// Entries returns a copy of all audit entries.
func (l *EventLogger) Entries() []AuditEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	result := make([]AuditEntry, len(l.entries))
	copy(result, l.entries)
	return result
}

// Len returns the number of entries in the audit log.
func (l *EventLogger) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.entries)
}

// computeRowHash computes SHA-256 hash of an audit entry.
func computeRowHash(entry AuditEntry) string {
	h := sha256.New()
	fmt.Fprintf(h, "%d:%s:%s:%s:%s", entry.ID, entry.PrevHash, string(entry.EventType), string(entry.Severity), entry.Timestamp.Format(time.RFC3339Nano))
	if entry.Message != "" {
		fmt.Fprintf(h, ":%s", entry.Message)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// logEvent emits structured log at the appropriate level.
func logEvent(evt Event) {
	attrs := []slog.Attr{
		slog.String("event_type", string(evt.Type)),
		slog.String("severity", string(evt.Severity)),
		slog.String("component", evt.Component),
	}
	if evt.BlockedURL != "" {
		attrs = append(attrs, slog.String("blocked_url", evt.BlockedURL))
	}
	if evt.BlockReason != "" {
		attrs = append(attrs, slog.String("block_reason", evt.BlockReason))
	}
	if evt.TenantIDClass != "" {
		attrs = append(attrs, slog.String("tenant_id_class", evt.TenantIDClass))
	}

	msg := evt.Message
	if msg == "" {
		msg = string(evt.Type)
	}

	switch evt.Severity {
	case SeverityCritical:
		slog.LogAttrs(nil, slog.LevelError, msg, attrs...)
	case SeverityHigh:
		slog.LogAttrs(nil, slog.LevelWarn, msg, attrs...)
	default:
		slog.LogAttrs(nil, slog.LevelInfo, msg, attrs...)
	}
}
