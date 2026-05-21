package streamsynth_test

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/elymas/universal-search/internal/sse"
	"github.com/elymas/universal-search/internal/streamsynth"
)

// T-M5-005 [RED]: EmitAgentEvent concurrent-write safety tests.
// REQ-DEEP2-007: SSE events must be thread-safe under concurrent agent completions.

func TestEmitAgentEventWritesAgentStarted(t *testing.T) {
	rw := newFakeRW()
	sw := sse.NewWriter(rw)
	sw.SetHeaders()

	payload := streamsynth.AgentStartedPayload{
		RequestID:     "test-123",
		Agent:         "researcher",
		SchemaVersion: 1,
	}

	err := streamsynth.EmitAgentEvent(sw, "agent_started", payload)
	if err != nil {
		t.Fatalf("EmitAgentEvent error: %v", err)
	}

	raw := rw.buf.String()
	if raw == "" {
		t.Fatal("expected SSE output, got empty string")
	}

	events := parseSSEEvents(raw)
	if len(events) == 0 {
		t.Fatal("no events parsed")
	}
	if events[0]["event"] != "agent_started" {
		t.Errorf("event type = %q, want %q", events[0]["event"], "agent_started")
	}

	var decoded streamsynth.AgentStartedPayload
	if err := json.Unmarshal([]byte(events[0]["data"]), &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Agent != "researcher" {
		t.Errorf("Agent = %q, want %q", decoded.Agent, "researcher")
	}
}

func TestEmitAgentEventWritesAgentCompleted(t *testing.T) {
	rw := newFakeRW()
	sw := sse.NewWriter(rw)
	sw.SetHeaders()

	payload := streamsynth.AgentCompletedPayload{
		RequestID:     "test-456",
		Agent:         "writer",
		Outcome:       "success",
		DurationMs:    1500,
		CostUSD:       0.05,
		SchemaVersion: 1,
	}

	err := streamsynth.EmitAgentEvent(sw, "agent_completed", payload)
	if err != nil {
		t.Fatalf("EmitAgentEvent error: %v", err)
	}

	events := parseSSEEvents(rw.buf.String())
	if len(events) == 0 {
		t.Fatal("no events parsed")
	}
	if events[0]["event"] != "agent_completed" {
		t.Errorf("event type = %q, want %q", events[0]["event"], "agent_completed")
	}
}

func TestEmitAgentEventConcurrentWritesSafe(t *testing.T) {
	rw := newFakeRW()
	sw := sse.NewWriter(rw)
	sw.SetHeaders()

	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			payload := streamsynth.AgentStartedPayload{
				RequestID:     "concurrent-test",
				Agent:         "agent",
				SchemaVersion: 1,
			}
			_ = streamsynth.EmitAgentEvent(sw, "agent_started", payload)
		}(i)
	}
	wg.Wait()

	raw := rw.buf.String()
	events := parseSSEEvents(raw)

	agentStartedCount := 0
	for _, ev := range events {
		if ev["event"] == "agent_started" {
			agentStartedCount++
		}
	}
	if agentStartedCount != 10 {
		t.Errorf("agent_started count = %d, want 10 (raw length=%d)", agentStartedCount, len(raw))
	}
}

func TestEmitAgentEventFlushesAfterWrite(t *testing.T) {
	rw := newFakeRW()
	sw := sse.NewWriter(rw)
	sw.SetHeaders()

	payload := streamsynth.PipelineFailedPayload{
		RequestID:     "test-flush",
		FailedAgent:   "writer",
		Reason:        "exhausted",
		Attempts:      3,
		RetryCount:    2,
		SchemaVersion: 1,
	}

	err := streamsynth.EmitAgentEvent(sw, "pipeline_failed", payload)
	if err != nil {
		t.Fatalf("EmitAgentEvent error: %v", err)
	}

	if rw.buf.Len() == 0 {
		t.Error("expected flushed data, got empty buffer")
	}
}
