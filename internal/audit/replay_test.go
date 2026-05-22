package audit

import (
	"context"
	"testing"
)

// mockEventFetcher implements EventFetcher for testing.
type mockEventFetcher struct {
	event *AuditEvent
	err   error
}

func (m *mockEventFetcher) FetchByRequestID(_ context.Context, _ string) (*AuditEvent, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.event, nil
}

// mockRateLimiter implements ReplayRateLimiter for testing.
type mockRateLimiter struct {
	allow bool
}

func (m *mockRateLimiter) Allow(_ string) bool { return m.allow }

// TestReplayRequiresRateLimit verifies rate limit triggers 429.
// REQ-AUTH3-004: rate limit 1/min, 429 + Retry-After.
func TestReplayRequiresRateLimit(t *testing.T) {
	store := &mockEventStore{}
	emitter := NewEmitter(store, DefaultConfig(), nil)
	fetcher := &mockEventFetcher{}
	rateLimit := &mockRateLimiter{allow: false}

	handler := NewReplayHandler(emitter, fetcher, rateLimit, nil, DefaultConfig())

	_, err := handler.Replay(context.Background(), "admin@test.com", ReplayRequest{RequestID: "req_001"})
	if err == nil {
		t.Fatal("Expected error for rate limit exceeded, got nil")
	}

	replayErr, ok := err.(*ReplayError)
	if !ok {
		t.Fatalf("Expected *ReplayError, got %T", err)
	}
	if replayErr.Code != 429 {
		t.Errorf("Error code = %d, want 429", replayErr.Code)
	}

	// A deny event should be emitted.
	events := store.Events()
	if len(events) != 1 {
		t.Fatalf("Expected 1 deny event, got %d", len(events))
	}
	if events[0].Decision != DecisionDeny {
		t.Errorf("Decision = %q, want %q", events[0].Decision, DecisionDeny)
	}
}

// TestReplayUnknownRequestId verifies 400 for unknown request.
// REQ-AUTH3-004: fetch event, 400 if unknown.
func TestReplayUnknownRequestIdReturns400(t *testing.T) {
	store := &mockEventStore{}
	emitter := NewEmitter(store, DefaultConfig(), nil)
	fetcher := &mockEventFetcher{event: nil} // nil = not found
	rateLimit := &mockRateLimiter{allow: true}

	handler := NewReplayHandler(emitter, fetcher, rateLimit, nil, DefaultConfig())

	_, err := handler.Replay(context.Background(), "admin@test.com", ReplayRequest{RequestID: "nonexistent"})
	if err == nil {
		t.Fatal("Expected error for unknown request, got nil")
	}

	replayErr, ok := err.(*ReplayError)
	if !ok {
		t.Fatalf("Expected *ReplayError, got %T", err)
	}
	if replayErr.Code != 400 {
		t.Errorf("Error code = %d, want 400", replayErr.Code)
	}
	if replayErr.Message != "unknown_request_id" {
		t.Errorf("Message = %q, want %q", replayErr.Message, "unknown_request_id")
	}
}

// TestReplayNonReplayableEvent verifies 400 for non-replayable events.
// REQ-AUTH3-004: only query.submit and deep.start are replayable.
func TestReplayNonReplayableEventReturns400(t *testing.T) {
	store := &mockEventStore{}
	emitter := NewEmitter(store, DefaultConfig(), nil)
	fetcher := &mockEventFetcher{
		event: &AuditEvent{
			EventType: EventRBACDeny,
			RequestID: "req_rbac",
		},
	}
	rateLimit := &mockRateLimiter{allow: true}

	handler := NewReplayHandler(emitter, fetcher, rateLimit, nil, DefaultConfig())

	_, err := handler.Replay(context.Background(), "admin@test.com", ReplayRequest{RequestID: "req_rbac"})
	if err == nil {
		t.Fatal("Expected error for non-replayable event, got nil")
	}

	replayErr, ok := err.(*ReplayError)
	if !ok {
		t.Fatalf("Expected *ReplayError, got %T", err)
	}
	if replayErr.Code != 400 {
		t.Errorf("Error code = %d, want 400", replayErr.Code)
	}
	if replayErr.Message != "event_not_replayable" {
		t.Errorf("Message = %q, want %q", replayErr.Message, "event_not_replayable")
	}
}

// TestReplayActorIdentityPropagation verifies admin actor identity.
// REQ-AUTH3-004: admin actor identity, NOT original user.
func TestReplayActorIdentityPropagation(t *testing.T) {
	store := &mockEventStore{}
	emitter := NewEmitter(store, DefaultConfig(), nil)
	fetcher := &mockEventFetcher{
		event: &AuditEvent{
			EventType: EventQuerySubmit,
			RequestID: "req_orig_001",
			UserID:    "alice@example.com",
			TenantID:  "default",
			TeamID:    "engineering",
			Payload: map[string]interface{}{
				"query": map[string]interface{}{
					"text": "quantum computing 2025 milestones",
					"lang": "en",
				},
			},
		},
	}
	rateLimit := &mockRateLimiter{allow: true}

	handler := NewReplayHandler(emitter, fetcher, rateLimit, nil, DefaultConfig())

	resp, err := handler.Replay(context.Background(), "bob@example.com", ReplayRequest{RequestID: "req_orig_001"})
	if err != nil {
		t.Fatalf("Replay returned error: %v", err)
	}

	if resp.Status != "submitted" {
		t.Errorf("Status = %q, want %q", resp.Status, "submitted")
	}
	if resp.NewRequestID == "" {
		t.Error("NewRequestID should not be empty")
	}

	// Should emit 2 events: admin.replay + query.submit
	events := store.Events()
	if len(events) != 2 {
		t.Fatalf("Expected 2 events, got %d", len(events))
	}

	// First event: admin.replay
	if events[0].EventType != EventAdminReplay {
		t.Errorf("First event EventType = %q, want %q", events[0].EventType, EventAdminReplay)
	}
	if events[0].UserID != "bob@example.com" {
		t.Errorf("admin.replay UserID = %q, want %q", events[0].UserID, "bob@example.com")
	}
	if events[0].Decision != DecisionAllow {
		t.Errorf("admin.replay Decision = %q, want %q", events[0].Decision, DecisionAllow)
	}

	// Second event: query.submit with admin actor identity.
	if events[1].EventType != EventQuerySubmit {
		t.Errorf("Second event EventType = %q, want %q", events[1].EventType, EventQuerySubmit)
	}
	if events[1].UserID != "bob@example.com" {
		t.Errorf("query.submit UserID = %q, want admin bob@example.com, NOT original alice", events[1].UserID)
	}
	if events[1].RequestID == "req_orig_001" {
		t.Error("query.submit RequestID should be new, not the original")
	}

	// Verify payload has replay metadata.
	payload := events[1].Payload
	if payload["replayed_from"] != "req_orig_001" {
		t.Errorf("replayed_from = %v, want %q", payload["replayed_from"], "req_orig_001")
	}
	if payload["replayed_by"] != "bob@example.com" {
		t.Errorf("replayed_by = %v, want %q", payload["replayed_by"], "bob@example.com")
	}
}

// TestReplayWithPIIMaskedQueryReturns400 verifies masked query rejection.
// REQ-AUTH3-006: query text masked, replay returns 400.
func TestReplayWithPIIMaskedQueryReturns400(t *testing.T) {
	store := &mockEventStore{}
	emitter := NewEmitter(store, DefaultConfig(), nil)
	fetcher := &mockEventFetcher{
		event: &AuditEvent{
			EventType: EventQuerySubmit,
			RequestID: "req_masked",
			Payload: map[string]interface{}{
				"query": map[string]interface{}{
					"text_sha256": "abc123hash",
					"lang":        "en",
					// "text" is missing = masked
				},
			},
		},
	}
	rateLimit := &mockRateLimiter{allow: true}

	handler := NewReplayHandler(emitter, fetcher, rateLimit, nil, DefaultConfig())

	_, err := handler.Replay(context.Background(), "admin@test.com", ReplayRequest{RequestID: "req_masked"})
	if err == nil {
		t.Fatal("Expected error for masked query, got nil")
	}

	replayErr, ok := err.(*ReplayError)
	if !ok {
		t.Fatalf("Expected *ReplayError, got %T", err)
	}
	if replayErr.Code != 400 {
		t.Errorf("Error code = %d, want 400", replayErr.Code)
	}
	if replayErr.Message != "query_text_masked" {
		t.Errorf("Message = %q, want %q", replayErr.Message, "query_text_masked")
	}
}
