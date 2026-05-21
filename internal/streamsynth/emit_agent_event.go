package streamsynth

import (
	"encoding/json"
	"fmt"

	"github.com/elymas/universal-search/internal/sse"
)

// EmitAgentEvent marshals an agent event payload and writes it as an SSE event
// with immediate flush. Uses the SSE Writer's built-in mutex for thread safety.
// REQ-DEEP2-007: All agent lifecycle events go through this function.
func EmitAgentEvent(w *sse.Writer, eventType string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("emit agent event %s: marshal: %w", eventType, err)
	}
	if err := w.WriteEvent(eventType, data); err != nil {
		return fmt.Errorf("emit agent event %s: write: %w", eventType, err)
	}
	return w.Flush()
}
