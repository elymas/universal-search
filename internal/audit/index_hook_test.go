package audit

import (
	"context"
	"testing"
)

// TestIndexWriteEmitsAuditEvent verifies that index write operations emit audit events.
// REQ-AUTH3-002: index.write event emission.
func TestIndexWriteEmitsAuditEvent(t *testing.T) {
	store := &mockEventStore{}
	emitter := NewEmitter(store, DefaultConfig(), nil)

	err := EmitIndexWrite(context.Background(), emitter, "upsert", "qdrant",
		map[string]interface{}{"doc_count": 5, "errors": 0})
	if err != nil {
		t.Fatalf("EmitIndexWrite returned error: %v", err)
	}

	events := store.Events()
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	got := events[0]
	if got.EventType != EventIndexWrite {
		t.Errorf("EventType = %q, want %q", got.EventType, EventIndexWrite)
	}
	if got.Source != SourceGo {
		t.Errorf("Source = %q, want %q", got.Source, SourceGo)
	}

	payload := got.Payload
	if payload["op"] != "upsert" {
		t.Errorf("payload.op = %v, want %q", payload["op"], "upsert")
	}
	if payload["store"] != "qdrant" {
		t.Errorf("payload.store = %v, want %q", payload["store"], "qdrant")
	}
}

// TestIndexWriteToggleControlsEmission verifies the toggle.
// D4: index.write events controlled by config toggle.
func TestIndexWriteToggleControlsEmission(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IndexWriteEnabled = false

	store := &mockEventStore{}
	emitter := NewEmitter(store, cfg, nil)

	err := EmitIndexWrite(context.Background(), emitter, "upsert", "qdrant", nil)
	if err != nil {
		t.Fatalf("EmitIndexWrite returned error: %v", err)
	}

	events := store.Events()
	if len(events) != 0 {
		t.Errorf("Expected 0 events when disabled, got %d", len(events))
	}
}

// TestIndexWriteDeleteEmitsCorrectly verifies index.delete event type.
func TestIndexWriteDeleteEmitsCorrectly(t *testing.T) {
	store := &mockEventStore{}
	emitter := NewEmitter(store, DefaultConfig(), nil)

	err := EmitIndexDelete(context.Background(), emitter, "meili",
		map[string]interface{}{"doc_ids": []string{"doc1", "doc2"}})
	if err != nil {
		t.Fatalf("EmitIndexDelete returned error: %v", err)
	}

	events := store.Events()
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	got := events[0]
	if got.EventType != EventIndexDelete {
		t.Errorf("EventType = %q, want %q", got.EventType, EventIndexDelete)
	}
	if got.Payload["store"] != "meili" {
		t.Errorf("payload.store = %v, want %q", got.Payload["store"], "meili")
	}
}
