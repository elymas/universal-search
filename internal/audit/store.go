package audit

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"sync"
)

// EventStore is the interface for persisting audit events.
// Implementations may write to Postgres, a mock, or an async queue.
type EventStore interface {
	Insert(ctx context.Context, evt AuditEvent) error
}

// Emitter is the single funnel for all audit event emissions.
// REQ-AUTH3-002: all audit writes go through EmitEvent.
// @MX:ANCHOR: [AUTO] Single audit emission funnel; fan_in >= 7
// @MX:REASON: all audit event emission points call this function; changing signature requires updating all callers
// @MX:SPEC: SPEC-AUTH-003
type Emitter struct {
	store   EventStore
	cfg     Config
	metrics *Metrics
	mu      sync.Mutex
}

// NewEmitter creates a new audit event emitter.
func NewEmitter(store EventStore, cfg Config, metrics *Metrics) *Emitter {
	return &Emitter{
		store:   store,
		cfg:     cfg,
		metrics: metrics,
	}
}

// EmitEvent emits a single audit event through the configured store.
// REQ-AUTH3-002: single emitter funnels all audit writes.
func (e *Emitter) EmitEvent(ctx context.Context, evt AuditEvent) error {
	// Validate event type (NFR-AUTH3-008: cardinality safety).
	if !evt.EventType.IsValid() {
		return fmt.Errorf("audit: invalid event_type %q", evt.EventType)
	}

	// Validate decision.
	if evt.EventType != "" && !evt.Decision.IsValid() {
		return fmt.Errorf("audit: invalid decision %q", evt.Decision)
	}

	// Apply PII masking (REQ-AUTH3-006).
	evt = e.applyPIIMasking(evt)

	// Set defaults.
	if evt.Source == "" {
		evt.Source = SourceGo
	}
	if evt.TenantID == "" {
		evt.TenantID = "default"
	}

	// Persist.
	if e.store == nil {
		slog.Warn("audit: no store configured, event dropped")
		return nil
	}

	err := e.store.Insert(ctx, evt)
	if err != nil {
		if e.metrics != nil {
			e.metrics.EventsTotal.WithLabelValues(string(evt.EventType), string(evt.Decision), string(evt.Source)).Inc()
		}
		return fmt.Errorf("audit: emit event: %w", err)
	}

	// Record metrics (REQ-AUTH3-009).
	if e.metrics != nil {
		e.metrics.EventsTotal.WithLabelValues(string(evt.EventType), string(evt.Decision), string(evt.Source)).Inc()
	}

	return nil
}

// applyPIIMasking applies PII masking rules to the event.
// REQ-AUTH3-006: replace query.text with text_sha256 when MaskQueryText is true.
// Identity fields (user_id, tenant_id, request_id) are NEVER masked.
func (e *Emitter) applyPIIMasking(evt AuditEvent) AuditEvent {
	if e.cfg.MaskQueryText {
		evt.Payload = maskQueryText(evt.Payload)
	}

	if e.cfg.MaskIP {
		evt.IP = ""
	}

	return evt
}

// maskQueryText replaces query.text with text_sha256 in the payload.
func maskQueryText(payload map[string]interface{}) map[string]interface{} {
	if payload == nil {
		return payload
	}

	queryRaw, exists := payload["query"]
	if !exists {
		return payload
	}

	query, ok := queryRaw.(map[string]interface{})
	if !ok {
		return payload
	}

	text, hasText := query["text"]
	if !hasText {
		return payload
	}

	textStr, ok := text.(string)
	if !ok {
		return payload
	}

	// Replace text with sha256.
	hash := sha256.Sum256([]byte(textStr))
	query["text_sha256"] = fmt.Sprintf("%x", hash)
	delete(query, "text")

	return payload
}
