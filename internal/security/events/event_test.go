package events

import (
	"context"
	"sync"
	"testing"

	"github.com/elymas/universal-search/internal/audit"
)

// captureStore is a fake audit.EventStore that records inserted events.
type captureStore struct {
	mu   sync.Mutex
	rows []audit.AuditEvent
}

func (c *captureStore) Insert(_ context.Context, evt audit.AuditEvent) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rows = append(c.rows, evt)
	return nil
}

func (c *captureStore) last() audit.AuditEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.rows[len(c.rows)-1]
}

// recorderSpy records metric increments.
type recorderSpy struct {
	mu    sync.Mutex
	calls [][2]string
}

func (r *recorderSpy) RecordSecurityEvent(t, sev string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, [2]string{t, sev})
}

func TestSecurityEventMapsToAuditEventType(t *testing.T) {
	t.Parallel()
	// Every one of the 7 SEC-001 types must resolve to a valid AUTH-003
	// EventType that passes the enum-lock validity check.
	for _, typ := range AllTypes() {
		et, ok := AuditEventType(typ)
		if !ok {
			t.Errorf("type %q has no audit.EventType mapping", typ)
			continue
		}
		if !et.IsValid() {
			t.Errorf("type %q maps to %q which is not a registered audit EventType (enum lock)", typ, et)
		}
	}
}

func TestSecurityEventEmittedToAuditStore(t *testing.T) {
	t.Parallel()
	store := &captureStore{}
	auditEmitter := audit.NewEmitter(store, audit.Config{}, nil)
	spy := &recorderSpy{}
	em := NewEmitter(auditEmitter, spy, nil)

	err := em.Emit(context.Background(), Event{
		Type:     TypeSSRFBlocked,
		Severity: SeverityMedium,
		Audit: audit.AuditEvent{
			TenantID:  "tenant-a",
			RequestID: "req-1",
			Resource:  "http://169.254.169.254",
		},
	})
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}

	// The event reached the AUTH-003 store with the mapped EventType + Decision.
	got := store.last()
	if got.EventType != audit.EventSecuritySSRFBlocked {
		t.Errorf("stored EventType = %q, want %q", got.EventType, audit.EventSecuritySSRFBlocked)
	}
	if got.Decision != audit.DecisionDeny {
		t.Errorf("stored Decision = %q, want deny", got.Decision)
	}
	// The metric was incremented with bounded labels.
	if len(spy.calls) != 1 || spy.calls[0] != [2]string{"ssrf.blocked", "medium"} {
		t.Errorf("metric calls = %v, want one [ssrf.blocked medium]", spy.calls)
	}
}

func TestSecurityEventDecisionMapping(t *testing.T) {
	t.Parallel()
	cases := map[Type]audit.Decision{
		TypeAuthFailed:        audit.DecisionDeny,
		TypeAuthSuccess:       audit.DecisionAllow,
		TypeSSRFBlocked:       audit.DecisionDeny,
		TypeRBACDenied:        audit.DecisionDeny,
		TypeRateLimitExceeded: audit.DecisionDeny,
		TypeSecretScanFinding: audit.DecisionNone,
		TypePromptSanitized:   audit.DecisionNone,
	}
	for typ, want := range cases {
		if got := auditDecision(typ); got != want {
			t.Errorf("auditDecision(%q) = %q, want %q", typ, got, want)
		}
	}
}

func TestSecurityEventUnknownTypeIsNoop(t *testing.T) {
	t.Parallel()
	store := &captureStore{}
	em := NewEmitter(audit.NewEmitter(store, audit.Config{}, nil), nil, nil)
	if err := em.Emit(context.Background(), Event{Type: Type("bogus"), Severity: SeverityLow}); err != nil {
		t.Errorf("unknown type must be a no-op, got %v", err)
	}
	if len(store.rows) != 0 {
		t.Errorf("unknown type must not write to the audit store; got %d rows", len(store.rows))
	}
}

func TestSecurityEventEmitsIntoExistingChain(t *testing.T) {
	t.Parallel()
	// Confirm SEC-001 introduces no new chain: the event flows through the
	// existing audit.Emitter and the row is verifiable by the existing
	// audit.VerifyChain with hashes computed by the existing audit.ComputeThisHash.
	store := &captureStore{}
	em := NewEmitter(audit.NewEmitter(store, audit.Config{}, nil), nil, nil)
	if err := em.Emit(context.Background(), Event{Type: TypeSecretScanFinding, Severity: SeverityCritical, Audit: audit.AuditEvent{TenantID: "t"}}); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	row := store.last()
	// Recompute the chain hash for the single-row chain using the EXISTING
	// AUTH-003 primitive and confirm VerifyChain accepts it.
	row.ThisHash = audit.ComputeThisHash("", row)
	if v := audit.VerifyChain([]audit.AuditEvent{row}); v != 0 {
		t.Errorf("existing audit.VerifyChain reported %d violations on a freshly hashed row", v)
	}
}
