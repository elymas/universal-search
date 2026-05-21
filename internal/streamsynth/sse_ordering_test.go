package streamsynth_test

import (
	"testing"

	"github.com/elymas/universal-search/internal/sse"
	"github.com/elymas/universal-search/internal/streamsynth"
)

// T-M5-007 [RED]: SSE event ordering sequence tests.
// REQ-DEEP2-007: agent_started -> agent_completed -> retry_started -> verifier_result ->
// pipeline_failed/pipeline_cancelled -> section events after PASS.

func TestSSEEventOrderAgentLifecycle(t *testing.T) {
	rw := newFakeRW()
	sw := sse.NewWriter(rw)
	sw.SetHeaders()

	// Simulate: researcher started -> completed -> reviewer started -> completed.
	evts := []struct {
		eventType string
		payload   any
	}{
		{"agent_started", streamsynth.AgentStartedPayload{RequestID: "r1", Agent: "researcher", SchemaVersion: 1}},
		{"agent_completed", streamsynth.AgentCompletedPayload{RequestID: "r1", Agent: "researcher", Outcome: "success", DurationMs: 500, SchemaVersion: 1}},
		{"agent_started", streamsynth.AgentStartedPayload{RequestID: "r1", Agent: "reviewer", SchemaVersion: 1}},
		{"agent_completed", streamsynth.AgentCompletedPayload{RequestID: "r1", Agent: "reviewer", Outcome: "success", DurationMs: 300, SchemaVersion: 1}},
	}

	for _, e := range evts {
		if err := streamsynth.EmitAgentEvent(sw, e.eventType, e.payload); err != nil {
			t.Fatalf("EmitAgentEvent %s: %v", e.eventType, err)
		}
	}

	events := parseSSEEvents(rw.buf.String())
	expected := []string{"agent_started", "agent_completed", "agent_started", "agent_completed"}
	if len(events) != len(expected) {
		t.Fatalf("event count = %d, want %d", len(events), len(expected))
	}
	for i, ev := range events {
		if ev["event"] != expected[i] {
			t.Errorf("event[%d] = %q, want %q", i, ev["event"], expected[i])
		}
	}
}

func TestSSEEventOrderRetrySequence(t *testing.T) {
	rw := newFakeRW()
	sw := sse.NewWriter(rw)
	sw.SetHeaders()

	// Simulate: writer started -> verifier result (fail) -> retry_started -> writer started -> verifier result (pass).
	evts := []struct {
		eventType string
		payload   any
	}{
		{"agent_started", streamsynth.AgentStartedPayload{RequestID: "r2", Agent: "writer", SchemaVersion: 1}},
		{"verifier_result", streamsynth.VerifierResultPayload{RequestID: "r2", Pass: false, UncitedCount: 2, SchemaVersion: 1}},
		{"retry_started", streamsynth.RetryStartedPayload{RequestID: "r2", Agent: "writer", Attempt: 2, MaxAttempts: 3, SchemaVersion: 1}},
		{"agent_started", streamsynth.AgentStartedPayload{RequestID: "r2", Agent: "writer", SchemaVersion: 1}},
		{"verifier_result", streamsynth.VerifierResultPayload{RequestID: "r2", Pass: true, UncitedCount: 0, SchemaVersion: 1}},
	}

	for _, e := range evts {
		if err := streamsynth.EmitAgentEvent(sw, e.eventType, e.payload); err != nil {
			t.Fatalf("EmitAgentEvent %s: %v", e.eventType, err)
		}
	}

	events := parseSSEEvents(rw.buf.String())
	expected := []string{"agent_started", "verifier_result", "retry_started", "agent_started", "verifier_result"}
	if len(events) != len(expected) {
		t.Fatalf("event count = %d, want %d", len(events), len(expected))
	}
	for i, ev := range events {
		if ev["event"] != expected[i] {
			t.Errorf("event[%d] = %q, want %q", i, ev["event"], expected[i])
		}
	}
}

func TestSSEEventOrderPipelineFailedTerminal(t *testing.T) {
	rw := newFakeRW()
	sw := sse.NewWriter(rw)
	sw.SetHeaders()

	evts := []struct {
		eventType string
		payload   any
	}{
		{"agent_started", streamsynth.AgentStartedPayload{RequestID: "r3", Agent: "writer", SchemaVersion: 1}},
		{"verifier_result", streamsynth.VerifierResultPayload{RequestID: "r3", Pass: false, UncitedCount: 5, SchemaVersion: 1}},
		{"retry_started", streamsynth.RetryStartedPayload{RequestID: "r3", Agent: "writer", Attempt: 2, MaxAttempts: 3, SchemaVersion: 1}},
		{"agent_started", streamsynth.AgentStartedPayload{RequestID: "r3", Agent: "writer", SchemaVersion: 1}},
		{"verifier_result", streamsynth.VerifierResultPayload{RequestID: "r3", Pass: false, UncitedCount: 3, SchemaVersion: 1}},
		{"retry_started", streamsynth.RetryStartedPayload{RequestID: "r3", Agent: "writer", Attempt: 3, MaxAttempts: 3, SchemaVersion: 1}},
		{"agent_started", streamsynth.AgentStartedPayload{RequestID: "r3", Agent: "writer", SchemaVersion: 1}},
		{"verifier_result", streamsynth.VerifierResultPayload{RequestID: "r3", Pass: false, UncitedCount: 1, SchemaVersion: 1}},
		{"pipeline_failed", streamsynth.PipelineFailedPayload{RequestID: "r3", FailedAgent: "writer", Reason: "verifier_rejection_exhausted", Attempts: 3, RetryCount: 2, SchemaVersion: 1}},
	}

	for _, e := range evts {
		if err := streamsynth.EmitAgentEvent(sw, e.eventType, e.payload); err != nil {
			t.Fatalf("EmitAgentEvent %s: %v", e.eventType, err)
		}
	}

	events := parseSSEEvents(rw.buf.String())
	lastEvent := events[len(events)-1]
	if lastEvent["event"] != "pipeline_failed" {
		t.Errorf("last event = %q, want pipeline_failed", lastEvent["event"])
	}
}

func TestSSEEventOrderPipelineCancelledTerminal(t *testing.T) {
	rw := newFakeRW()
	sw := sse.NewWriter(rw)
	sw.SetHeaders()

	evts := []struct {
		eventType string
		payload   any
	}{
		{"agent_started", streamsynth.AgentStartedPayload{RequestID: "r4", Agent: "researcher", SchemaVersion: 1}},
		{"pipeline_cancelled", streamsynth.PipelineCancelledPayload{RequestID: "r4", AtAgent: "researcher", SchemaVersion: 1}},
	}

	for _, e := range evts {
		if err := streamsynth.EmitAgentEvent(sw, e.eventType, e.payload); err != nil {
			t.Fatalf("EmitAgentEvent %s: %v", e.eventType, err)
		}
	}

	events := parseSSEEvents(rw.buf.String())
	if len(events) != 2 {
		t.Fatalf("event count = %d, want 2", len(events))
	}
	if events[1]["event"] != "pipeline_cancelled" {
		t.Errorf("event[1] = %q, want pipeline_cancelled", events[1]["event"])
	}
}
