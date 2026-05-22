package audit

import (
	"context"
	"fmt"
)

// EmitIndexWrite emits an index.write audit event if the toggle is enabled.
// D4: V1 all-in + audit.events.index_write_enabled toggle.
func EmitIndexWrite(ctx context.Context, emitter *Emitter, op, store string, outcomes map[string]interface{}) error {
	if emitter == nil {
		return nil
	}

	// Check toggle.
	if !emitter.cfg.IndexWriteEnabled {
		return nil
	}

	payload := map[string]interface{}{
		"op":    op,
		"store": store,
	}
	for k, v := range outcomes {
		payload[k] = v
	}

	evt := AuditEvent{
		EventType: EventIndexWrite,
		Decision:  DecisionNone,
		Source:    SourceGo,
		Resource:  fmt.Sprintf("index:%s", store),
		Action:    op,
		Payload:   payload,
	}

	return emitter.EmitEvent(ctx, evt)
}

// EmitIndexDelete emits an index.delete audit event if the toggle is enabled.
func EmitIndexDelete(ctx context.Context, emitter *Emitter, store string, outcomes map[string]interface{}) error {
	if emitter == nil {
		return nil
	}

	if !emitter.cfg.IndexWriteEnabled {
		return nil
	}

	payload := map[string]interface{}{
		"op":    "delete",
		"store": store,
	}
	for k, v := range outcomes {
		payload[k] = v
	}

	evt := AuditEvent{
		EventType: EventIndexDelete,
		Decision:  DecisionNone,
		Source:    SourceGo,
		Resource:  fmt.Sprintf("index:%s", store),
		Action:    "delete",
		Payload:   payload,
	}

	return emitter.EmitEvent(ctx, evt)
}
