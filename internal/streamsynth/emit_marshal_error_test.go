package streamsynth_test

// Coverage for the EmitAgentEvent marshal-error branch: a payload that cannot
// be JSON-encoded must surface a wrapped error rather than writing an event.
//
// @MX:SPEC: SPEC-REL-001 — G1 release coverage gate

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/elymas/universal-search/internal/sse"
	"github.com/elymas/universal-search/internal/streamsynth"
)

func TestEmitAgentEventMarshalError(t *testing.T) {
	sw := sse.NewWriter(httptest.NewRecorder())

	// A channel value is not JSON-serializable, forcing json.Marshal to fail.
	err := streamsynth.EmitAgentEvent(sw, "agent_started", make(chan int))
	if err == nil {
		t.Fatal("expected marshal error, got nil")
	}
	if !strings.Contains(err.Error(), "marshal") {
		t.Errorf("error = %v, want a marshal error", err)
	}
}
