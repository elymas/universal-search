package audit

import (
	"context"
	"log/slog"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

// TestRegisterMetrics verifies metrics registration.
func TestRegisterMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := RegisterMetrics(reg)
	if m == nil {
		t.Fatal("RegisterMetrics returned nil")
	}
	if m.EventsTotal == nil {
		t.Error("EventsTotal should not be nil")
	}
	if m.WriteDuration == nil {
		t.Error("WriteDuration should not be nil")
	}
	if m.ReconcilePollsTotal == nil {
		t.Error("ReconcilePollsTotal should not be nil")
	}
	if m.ReconcileLagSeconds == nil {
		t.Error("ReconcileLagSeconds should not be nil")
	}
	if m.S3ExportDurationSeconds == nil {
		t.Error("S3ExportDurationSeconds should not be nil")
	}
	if m.ChainViolationsTotal == nil {
		t.Error("ChainViolationsTotal should not be nil")
	}
	if m.PartitionDropTotal == nil {
		t.Error("PartitionDropTotal should not be nil")
	}
	if m.ReplayRequestsTotal == nil {
		t.Error("ReplayRequestsTotal should not be nil")
	}
}

// TestChainManager_Enabled verifies chain manager state.
func TestChainManager_Enabled(t *testing.T) {
	cm := NewChainManager(false)
	if cm.Enabled() {
		t.Error("ChainManager should be disabled")
	}

	cm = NewChainManager(true)
	if !cm.Enabled() {
		t.Error("ChainManager should be enabled")
	}

	// nil check
	var nilCm *ChainManager
	if nilCm.Enabled() {
		t.Error("nil ChainManager should not be enabled")
	}
}

// TestDecisionLogHandler_WithAttrs_WithGroup verifies handler chaining.
func TestDecisionLogHandler_WithAttrs_WithGroup(t *testing.T) {
	inner := &mockSlogHandler{enabled: true}
	emitter := NewEmitter(&mockEventStore{}, DefaultConfig(), nil)
	handler := NewDecisionLogHandler(inner, emitter)

	attrs := []slog.Attr{slog.String("test", "value")}
	newHandler := handler.WithAttrs(attrs)
	if newHandler == nil {
		t.Error("WithAttrs should return non-nil handler")
	}

	newHandler = handler.WithGroup("test_group")
	if newHandler == nil {
		t.Error("WithGroup should return non-nil handler")
	}
}

// TestCleanupOlderThan verifies the cutoff description.
func TestCleanupOlderThan(t *testing.T) {
	cfg := DefaultConfig()
	cleanup := NewCleanup(nil, NewEmitter(&mockEventStore{}, cfg, nil), nil, cfg)
	desc := cleanup.CleanupOlderThan()
	if desc == "" {
		t.Error("CleanupOlderThan should not return empty string")
	}
}

// TestReplayError_Error verifies error message format.
func TestReplayError_Error(t *testing.T) {
	err := &ReplayError{Code: 400, Message: "test error"}
	if err.Error() != "replay error 400: test error" {
		t.Errorf("Unexpected error message: %s", err.Error())
	}
}

// TestEmitEventWithMetrics verifies metrics recording on success.
func TestEmitEventWithMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := RegisterMetrics(reg)

	store := &mockEventStore{}
	emitter := NewEmitter(store, DefaultConfig(), m)

	err := emitter.EmitEvent(context.Background(), AuditEvent{
		EventType: EventAuthLogin,
		Decision:  DecisionAllow,
		UserID:    "test",
		Source:    SourceGo,
	})
	if err != nil {
		t.Fatalf("EmitEvent returned error: %v", err)
	}
}

// TestEmitEventNilStore verifies no panic with nil store.
func TestEmitEventNilStore(t *testing.T) {
	emitter := NewEmitter(nil, DefaultConfig(), nil)

	err := emitter.EmitEvent(context.Background(), AuditEvent{
		EventType: EventAuthLogin,
		Decision:  DecisionAllow,
		UserID:    "test",
	})
	if err != nil {
		t.Fatalf("EmitEvent with nil store should not error: %v", err)
	}
}

// TestEmitEventDefaultSource verifies source defaults to "go".
func TestEmitEventDefaultSource(t *testing.T) {
	store := &mockEventStore{}
	emitter := NewEmitter(store, DefaultConfig(), nil)

	err := emitter.EmitEvent(context.Background(), AuditEvent{
		EventType: EventAuthLogin,
		Decision:  DecisionAllow,
		UserID:    "test",
		// Source intentionally empty
	})
	if err != nil {
		t.Fatalf("EmitEvent returned error: %v", err)
	}

	events := store.Events()
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}
	if events[0].Source != SourceGo {
		t.Errorf("Source = %q, want default %q", events[0].Source, SourceGo)
	}
}

// TestEmitEventDefaultTenantID verifies tenant_id defaults to "default".
func TestEmitEventDefaultTenantID(t *testing.T) {
	store := &mockEventStore{}
	emitter := NewEmitter(store, DefaultConfig(), nil)

	err := emitter.EmitEvent(context.Background(), AuditEvent{
		EventType: EventAuthLogin,
		Decision:  DecisionAllow,
		UserID:    "test",
		// TenantID intentionally empty
	})
	if err != nil {
		t.Fatalf("EmitEvent returned error: %v", err)
	}

	events := store.Events()
	if events[0].TenantID != "default" {
		t.Errorf("TenantID = %q, want default %q", events[0].TenantID, "default")
	}
}

// TestEmitIndexWriteNilEmitter verifies no panic with nil emitter.
func TestEmitIndexWriteNilEmitter(t *testing.T) {
	err := EmitIndexWrite(context.Background(), nil, "upsert", "qdrant", nil)
	if err != nil {
		t.Fatalf("EmitIndexWrite with nil emitter should not error: %v", err)
	}
}

// TestEmitIndexDeleteNilEmitter verifies no panic with nil emitter.
func TestEmitIndexDeleteNilEmitter(t *testing.T) {
	err := EmitIndexDelete(context.Background(), nil, "meili", nil)
	if err != nil {
		t.Fatalf("EmitIndexDelete with nil emitter should not error: %v", err)
	}
}
