package audit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mockDedupChecker implements DedupChecker for testing.
type mockDedupChecker struct {
	costLedgerIDs map[string]bool
	auditEventIDs map[string]bool
	costErr       error
	auditErr      error
}

func (m *mockDedupChecker) ExistsInCostLedger(_ context.Context, requestID string) (bool, error) {
	if m.costErr != nil {
		return false, m.costErr
	}
	return m.costLedgerIDs[requestID], nil
}

func (m *mockDedupChecker) ExistsInAuditEvents(_ context.Context, litellmRequestID string) (bool, error) {
	if m.auditErr != nil {
		return false, m.auditErr
	}
	return m.auditEventIDs[litellmRequestID], nil
}

// TestReconcileFetchesSpendLogs verifies the reconciliation fetches from LiteLLM.
// REQ-AUTH3-003: poll LiteLLM GET /spend/logs.
func TestReconcileFetchesSpendLogs(t *testing.T) {
	logs := []SpendLog{
		{
			RequestID:        "litellm-req-001",
			CallType:         "acompletion",
			Model:            "claude-haiku-4-5",
			PromptTokens:     800,
			CompletionTokens: 300,
			Spend:            0.00184,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/spend/logs" {
			t.Errorf("Expected path /spend/logs, got %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("Expected GET, got %s", r.Method)
		}
		// Verify date parameters are present.
		if r.URL.Query().Get("start_date") == "" || r.URL.Query().Get("end_date") == "" {
			t.Error("Missing start_date or end_date query parameters")
		}
		json.NewEncoder(w).Encode(logs)
	}))
	defer server.Close()

	store := &mockEventStore{}
	emitter := NewEmitter(store, DefaultConfig(), nil)
	client := NewHTTPLiteLLMClient(server.URL)
	dedup := &mockDedupChecker{costLedgerIDs: map[string]bool{}, auditEventIDs: map[string]bool{}}
	reconciler := NewReconciler(client, emitter, dedup, nil, DefaultConfig())

	err := reconciler.Reconcile(context.Background())
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}

	events := store.Events()
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	got := events[0]
	if got.EventType != EventCostReconciled {
		t.Errorf("EventType = %q, want %q", got.EventType, EventCostReconciled)
	}
	if got.Source != SourcePython {
		t.Errorf("Source = %q, want %q", got.Source, SourcePython)
	}
	if got.Payload["litellm_request_id"] != "litellm-req-001" {
		t.Errorf("payload.litellm_request_id = %v, want %q", got.Payload["litellm_request_id"], "litellm-req-001")
	}
}

// TestReconcileDedupByCostLedger verifies dedup against cost_ledger.
// REQ-AUTH3-003: dedup against cost_ledger.request_id.
func TestReconcileDedupByCostLedger(t *testing.T) {
	logs := []SpendLog{
		{RequestID: "existing-in-ledger", Model: "gpt-4", Spend: 0.01},
		{RequestID: "new-req", Model: "gpt-4", Spend: 0.02},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(logs)
	}))
	defer server.Close()

	store := &mockEventStore{}
	emitter := NewEmitter(store, DefaultConfig(), nil)
	client := NewHTTPLiteLLMClient(server.URL)
	dedup := &mockDedupChecker{
		costLedgerIDs: map[string]bool{"existing-in-ledger": true},
		auditEventIDs: map[string]bool{},
	}
	reconciler := NewReconciler(client, emitter, dedup, nil, DefaultConfig())

	err := reconciler.Reconcile(context.Background())
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}

	events := store.Events()
	if len(events) != 1 {
		t.Fatalf("Expected 1 event (deduped), got %d", len(events))
	}
	if events[0].Payload["litellm_request_id"] != "new-req" {
		t.Errorf("Expected new-req event, got %v", events[0].Payload["litellm_request_id"])
	}
}

// TestReconcileDedupByAuditEvents verifies dedup against existing audit_events.
// REQ-AUTH3-003: idempotent reconciliation.
func TestReconcileDedupByAuditEvents(t *testing.T) {
	logs := []SpendLog{
		{RequestID: "already-reconciled", Model: "gpt-4", Spend: 0.01},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(logs)
	}))
	defer server.Close()

	store := &mockEventStore{}
	emitter := NewEmitter(store, DefaultConfig(), nil)
	client := NewHTTPLiteLLMClient(server.URL)
	dedup := &mockDedupChecker{
		costLedgerIDs: map[string]bool{},
		auditEventIDs: map[string]bool{"already-reconciled": true},
	}
	reconciler := NewReconciler(client, emitter, dedup, nil, DefaultConfig())

	err := reconciler.Reconcile(context.Background())
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}

	events := store.Events()
	if len(events) != 0 {
		t.Errorf("Expected 0 events (all deduped), got %d", len(events))
	}
}

// TestReconcileEmitsErrorOnLiteLLMFailure verifies error counter on 5xx.
// REQ-AUTH3-003, REQ-AUTH3-009.
func TestReconcileEmitsErrorOnLiteLLMFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal"}`))
	}))
	defer server.Close()

	store := &mockEventStore{}
	emitter := NewEmitter(store, DefaultConfig(), nil)
	client := NewHTTPLiteLLMClient(server.URL)
	reconciler := NewReconciler(client, emitter, nil, nil, DefaultConfig())

	err := reconciler.Reconcile(context.Background())
	if err == nil {
		t.Fatal("Expected error for 5xx response, got nil")
	}

	events := store.Events()
	if len(events) != 0 {
		t.Errorf("Expected 0 events on failure, got %d", len(events))
	}
}

// TestReconcileNilClient verifies graceful skip when client is nil.
func TestReconcileNilClient(t *testing.T) {
	store := &mockEventStore{}
	emitter := NewEmitter(store, DefaultConfig(), nil)
	reconciler := NewReconciler(nil, emitter, nil, nil, DefaultConfig())

	err := reconciler.Reconcile(context.Background())
	if err != nil {
		t.Fatalf("Reconcile with nil client should not error: %v", err)
	}

	events := store.Events()
	if len(events) != 0 {
		t.Errorf("Expected 0 events, got %d", len(events))
	}
}
