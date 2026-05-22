package audit

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

// mockSlogHandler captures records passed to it.
type mockSlogHandler struct {
	enabled bool
	records []string
}

func (m *mockSlogHandler) Enabled(_ context.Context, _ slog.Level) bool { return m.enabled }
func (m *mockSlogHandler) Handle(_ context.Context, r slog.Record) error {
	m.records = append(m.records, r.Message)
	return nil
}
func (m *mockSlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return m }
func (m *mockSlogHandler) WithGroup(name string) slog.Handler       { return m }

// TestParseDecisionLogLine verifies JSON parsing of DEEP-004 decision log lines.
func TestParseDecisionLogLine(t *testing.T) {
	data := []byte(`{
		"timestamp":"2026-05-22T13:45:00.123Z",
		"event_type":"cap.evaluation",
		"request_id":"req_capped_001",
		"tenant_id":"default",
		"user_id":"alice@example.com",
		"decision":"deny",
		"dimension":"calls",
		"remaining":{"calls":0,"usd":4.23},
		"screen_score":7,
		"cache_hit":false
	}`)

	line, err := ParseDecisionLogLine(data)
	if err != nil {
		t.Fatalf("ParseDecisionLogLine returned error: %v", err)
	}

	if line.EventType != "cap.evaluation" {
		t.Errorf("EventType = %q, want %q", line.EventType, "cap.evaluation")
	}
	if line.RequestID != "req_capped_001" {
		t.Errorf("RequestID = %q, want %q", line.RequestID, "req_capped_001")
	}
	if line.UserID != "alice@example.com" {
		t.Errorf("UserID = %q, want %q", line.UserID, "alice@example.com")
	}
	if line.Decision != "deny" {
		t.Errorf("Decision = %q, want %q", line.Decision, "deny")
	}
	if line.Dimension != "calls" {
		t.Errorf("Dimension = %q, want %q", line.Dimension, "calls")
	}
	if line.ScreenScore != 7 {
		t.Errorf("ScreenScore = %d, want 7", line.ScreenScore)
	}
	if line.CacheHit != false {
		t.Error("CacheHit = true, want false")
	}
}

// TestParseDecisionLogLine_invalidJSON verifies error on bad JSON.
func TestParseDecisionLogLine_invalidJSON(t *testing.T) {
	_, err := ParseDecisionLogLine([]byte(`not json`))
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

// TestDecisionLogHandler_Handle_passthrough verifies non-decision records pass through.
func TestDecisionLogHandler_Handle_passthrough(t *testing.T) {
	inner := &mockSlogHandler{enabled: true}
	store := &mockEventStore{}
	emitter := NewEmitter(store, DefaultConfig(), nil)
	handler := NewDecisionLogHandler(inner, emitter)

	// Regular log record without decision log attributes.
	logger := slog.New(handler)
	logger.Info("regular log message")

	if len(inner.records) != 1 {
		t.Errorf("Expected 1 inner record, got %d", len(inner.records))
	}

	// No audit event should be emitted.
	events := store.Events()
	if len(events) != 0 {
		t.Errorf("Expected 0 audit events, got %d", len(events))
	}
}

// TestDecisionLogHandler_Handle_capEvaluation verifies DEEP-004 cap.evaluation tee.
// REQ-AUTH3-002: DEEP-004 decision log mirrored to audit_events.
func TestDecisionLogHandler_Handle_capEvaluation(t *testing.T) {
	inner := &mockSlogHandler{enabled: true}
	store := &mockEventStore{}
	emitter := NewEmitter(store, DefaultConfig(), nil)
	handler := NewDecisionLogHandler(inner, emitter)

	ctx := context.Background()

	// Simulate a DEEP-004 cap.evaluation log record.
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "cap evaluation", 0)
	record.AddAttrs(
		slog.String("event_type", "cap.evaluation"),
		slog.String("request_id", "req_capped_001"),
		slog.String("tenant_id", "default"),
		slog.String("user_id", "alice@example.com"),
		slog.String("decision", "deny"),
		slog.String("dimension", "calls"),
		slog.Int("screen_score", 7),
		slog.Bool("cache_hit", false),
	)

	err := handler.Handle(ctx, record)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	// Inner handler should also receive the record.
	if len(inner.records) != 1 {
		t.Errorf("Expected 1 inner record, got %d", len(inner.records))
	}

	// Audit event should be emitted.
	events := store.Events()
	if len(events) != 1 {
		t.Fatalf("Expected 1 audit event, got %d", len(events))
	}

	got := events[0]
	if got.EventType != EventCapEvaluation {
		t.Errorf("EventType = %q, want %q", got.EventType, EventCapEvaluation)
	}
	if got.Decision != DecisionDeny {
		t.Errorf("Decision = %q, want %q", got.Decision, DecisionDeny)
	}
	if got.UserID != "alice@example.com" {
		t.Errorf("UserID = %q, want %q", got.UserID, "alice@example.com")
	}
	if got.RequestID != "req_capped_001" {
		t.Errorf("RequestID = %q, want %q", got.RequestID, "req_capped_001")
	}
}

// TestDecisionLogHandler_failOpen verifies audit failure does not block inner handler.
// REQ-AUTH3-002: audit handler failure SHALL NOT abort stderr write.
func TestDecisionLogHandler_failOpen(t *testing.T) {
	inner := &mockSlogHandler{enabled: true}
	store := &mockEventStore{err: context.DeadlineExceeded}
	emitter := NewEmitter(store, DefaultConfig(), nil)
	handler := NewDecisionLogHandler(inner, emitter)

	ctx := context.Background()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "cap evaluation", 0)
	record.AddAttrs(
		slog.String("event_type", "cap.evaluation"),
		slog.String("request_id", "req_fail"),
		slog.String("tenant_id", "default"),
		slog.String("user_id", "test"),
		slog.String("decision", "deny"),
	)

	err := handler.Handle(ctx, record)
	if err != nil {
		t.Errorf("Handle should not return error when audit fails: %v", err)
	}

	// Inner handler should still receive the record.
	if len(inner.records) != 1 {
		t.Errorf("Expected 1 inner record even when audit fails, got %d", len(inner.records))
	}
}
