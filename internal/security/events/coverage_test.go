package events

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/elymas/universal-search/internal/audit"
)

func TestEmit_NilAuditEmitterIsNoop(t *testing.T) {
	t.Parallel()
	// With no audit emitter, Emit still records metric + log and returns nil.
	spy := &recorderSpy{}
	em := NewEmitter(nil, spy, nil)
	if err := em.Emit(context.Background(), Event{Type: TypeAuthSuccess, Severity: SeverityLow}); err != nil {
		t.Fatalf("Emit with nil audit must be a no-op: %v", err)
	}
	if len(spy.calls) != 1 {
		t.Errorf("metric must still increment; calls=%v", spy.calls)
	}
}

func TestLogEvent_SeverityLevels(t *testing.T) {
	t.Parallel()
	cases := []struct {
		sev   Severity
		level string
	}{
		{SeverityCritical, "ERROR"},
		{SeverityHigh, "WARN"},
		{SeverityMedium, "INFO"},
		{SeverityLow, "INFO"},
	}
	for _, c := range cases {
		var buf bytes.Buffer
		log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
		em := NewEmitter(nil, nil, log)
		if err := em.Emit(context.Background(), Event{Type: TypeSSRFBlocked, Severity: c.sev}); err != nil {
			t.Fatalf("Emit: %v", err)
		}
		out := buf.String()
		if !strings.Contains(out, "level="+c.level) {
			t.Errorf("severity %q: expected level=%s in log, got %q", c.sev, c.level, out)
		}
	}
}

func TestAllTypesCount(t *testing.T) {
	t.Parallel()
	if got := len(AllTypes()); got != 7 {
		t.Errorf("AllTypes() = %d types, want 7", got)
	}
}

func TestEmit_DefaultSourceAppliedByAuditEmitter(t *testing.T) {
	t.Parallel()
	store := &captureStore{}
	em := NewEmitter(audit.NewEmitter(store, audit.Config{}, nil), nil, nil)
	if err := em.Emit(context.Background(), Event{Type: TypeRBACDenied, Severity: SeverityHigh, Audit: audit.AuditEvent{TenantID: "t"}}); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	got := store.last()
	if got.Source != audit.SourceGo {
		t.Errorf("audit emitter must default Source to go, got %q", got.Source)
	}
}
