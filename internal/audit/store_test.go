package audit

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
)

// mockEventStore captures emitted events for test assertions.
type mockEventStore struct {
	mu     sync.Mutex
	events []AuditEvent
	err    error // if set, Insert returns this error
}

func (m *mockEventStore) Insert(ctx context.Context, evt AuditEvent) error {
	if m.err != nil {
		return m.err
	}
	m.mu.Lock()
	m.events = append(m.events, evt)
	m.mu.Unlock()
	return nil
}

func (m *mockEventStore) Events() []AuditEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]AuditEvent, len(m.events))
	copy(out, m.events)
	return out
}

// TestEmitEventInsertsRow verifies that EmitEvent calls the store Insert.
// REQ-AUTH3-002: single EmitEvent emitter funnels all audit writes.
func TestEmitEventInsertsRow(t *testing.T) {
	store := &mockEventStore{}
	emitter := NewEmitter(store, DefaultConfig(), nil)

	evt := AuditEvent{
		EventType: EventAuthLogin,
		Decision:  DecisionAllow,
		UserID:    "alice@example.com",
		TenantID:  "default",
		TeamID:    "engineering",
		RequestID: "req_001",
		Source:    SourceGo,
		Payload:   map[string]interface{}{"method": "oidc"},
	}

	err := emitter.EmitEvent(context.Background(), evt)
	if err != nil {
		t.Fatalf("EmitEvent returned error: %v", err)
	}

	events := store.Events()
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	got := events[0]
	if got.EventType != EventAuthLogin {
		t.Errorf("EventType = %q, want %q", got.EventType, EventAuthLogin)
	}
	if got.Decision != DecisionAllow {
		t.Errorf("Decision = %q, want %q", got.Decision, DecisionAllow)
	}
	if got.UserID != "alice@example.com" {
		t.Errorf("UserID = %q, want %q", got.UserID, "alice@example.com")
	}
	if got.Source != SourceGo {
		t.Errorf("Source = %q, want %q", got.Source, SourceGo)
	}
}

// TestEmitEventNoPanicOnNilCtx verifies graceful handling of nil context.
// REQ-AUTH3-002: robust even without context.
func TestEmitEventNoPanicOnNilCtx(t *testing.T) {
	store := &mockEventStore{}
	emitter := NewEmitter(store, DefaultConfig(), nil)

	evt := AuditEvent{
		EventType: EventAuthLogin,
		Decision:  DecisionAllow,
		UserID:    "test",
	}

	// Should not panic. Passing a literal nil here is the point of the test
	// (the emitter must defend against callers who skip context plumbing).
	//nolint:staticcheck // SA1012: intentional nil ctx — testing graceful handling per REQ-AUTH3-002
	err := emitter.EmitEvent(nil, evt)
	if err != nil {
		t.Logf("EmitEvent with nil ctx returned error (expected): %v", err)
	}
}

// TestEmitEventInvalidEventType verifies rejection of unknown event types.
// NFR-AUTH3-008: cardinality safety.
func TestEmitEventInvalidEventType(t *testing.T) {
	store := &mockEventStore{}
	emitter := NewEmitter(store, DefaultConfig(), nil)

	evt := AuditEvent{
		EventType: EventType("unknown.event"),
		Decision:  DecisionAllow,
		UserID:    "test",
	}

	err := emitter.EmitEvent(context.Background(), evt)
	if err == nil {
		t.Error("Expected error for invalid event type, got nil")
	}
}

// TestEmitEventPIIMasking verifies query text masking.
// REQ-AUTH3-006: replace payload.query.text with text_sha256.
func TestEmitEventPIIMasking(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaskQueryText = true

	store := &mockEventStore{}
	emitter := NewEmitter(store, cfg, nil)

	evt := AuditEvent{
		EventType: EventQuerySubmit,
		Decision:  DecisionAllow,
		UserID:    "alice@example.com",
		Payload: map[string]interface{}{
			"query": map[string]interface{}{
				"text": "aspirin overdose treatment",
				"lang": "en",
			},
		},
	}

	err := emitter.EmitEvent(context.Background(), evt)
	if err != nil {
		t.Fatalf("EmitEvent returned error: %v", err)
	}

	events := store.Events()
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	got := events[0]
	query, ok := got.Payload["query"].(map[string]interface{})
	if !ok {
		t.Fatal("payload.query is not a map")
	}

	// Original text should be removed.
	if _, exists := query["text"]; exists {
		t.Error("payload.query.text should be removed when masking is enabled")
	}

	// text_sha256 should be present.
	sha, exists := query["text_sha256"]
	if !exists {
		t.Fatal("payload.query.text_sha256 should be present when masking is enabled")
	}
	if sha == "" {
		t.Error("text_sha256 should not be empty")
	}

	// lang should be preserved.
	if query["lang"] != "en" {
		t.Errorf("payload.query.lang = %v, want %q", query["lang"], "en")
	}
}

// TestEmitEventPIIMaskingPreservesIdentity verifies identity fields are not masked.
// REQ-AUTH3-006: user_id, tenant_id, request_id NEVER masked.
func TestEmitEventPIIMaskingPreservesIdentity(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaskQueryText = true
	cfg.MaskIP = true

	store := &mockEventStore{}
	emitter := NewEmitter(store, cfg, nil)

	evt := AuditEvent{
		EventType: EventQuerySubmit,
		Decision:  DecisionAllow,
		UserID:    "alice@example.com",
		TenantID:  "default",
		RequestID: "req_001",
		IP:        "192.168.1.1",
		Payload: map[string]interface{}{
			"query": map[string]interface{}{
				"text": "secret query",
			},
		},
	}

	err := emitter.EmitEvent(context.Background(), evt)
	if err != nil {
		t.Fatalf("EmitEvent returned error: %v", err)
	}

	events := store.Events()
	got := events[0]

	// Identity fields preserved.
	if got.UserID != "alice@example.com" {
		t.Errorf("UserID = %q, want %q", got.UserID, "alice@example.com")
	}
	if got.TenantID != "default" {
		t.Errorf("TenantID = %q, want %q", got.TenantID, "default")
	}
	if got.RequestID != "req_001" {
		t.Errorf("RequestID = %q, want %q", got.RequestID, "req_001")
	}

	// IP should be cleared when MaskIP is true.
	if got.IP != "" {
		t.Errorf("IP = %q, want empty when MaskIP=true", got.IP)
	}
}

// TestEmitEventStoreError verifies error propagation from store.
func TestEmitEventStoreError(t *testing.T) {
	store := &mockEventStore{err: context.DeadlineExceeded}
	emitter := NewEmitter(store, DefaultConfig(), nil)

	evt := AuditEvent{
		EventType: EventAuthLogin,
		Decision:  DecisionAllow,
		UserID:    "test",
	}

	err := emitter.EmitEvent(context.Background(), evt)
	if err == nil {
		t.Error("Expected error from store, got nil")
	}
}

// TestEmitEventConcurrent verifies thread safety.
func TestEmitEventConcurrent(t *testing.T) {
	store := &mockEventStore{}
	emitter := NewEmitter(store, DefaultConfig(), nil)

	var wg sync.WaitGroup
	var errors atomic.Int64

	for i := range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			evt := AuditEvent{
				EventType: EventQuerySubmit,
				Decision:  DecisionAllow,
				UserID:    "test",
				RequestID: "req_concurrent",
				Payload:   map[string]interface{}{"i": i},
			}
			if err := emitter.EmitEvent(context.Background(), evt); err != nil {
				errors.Add(1)
			}
		}()
	}

	wg.Wait()

	events := store.Events()
	if len(events) != 100 {
		t.Errorf("Expected 100 events, got %d (errors: %d)", len(events), errors.Load())
	}
}
