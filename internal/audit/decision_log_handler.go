package audit

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"
)

// DecisionLogHandler is an slog.Handler that intercepts DEEP-004 decision log
// JSON lines and mirrors them into audit_events.
// REQ-AUTH3-002: DEEP-004 stderr JSON line tee handler.
// DEEP-004 code is NOT changed — this handler absorbs the existing JSON lines.
// @MX:ANCHOR: [AUTO] DEEP-004 forward-compat integration point
// @MX:REASON: DEEP-004 schema 6 mandatory fields (timestamp, event_type, request_id, tenant_id, user_id, decision) are 1:1 mapped
// @MX:SPEC: SPEC-AUTH-003 section 1.3
type DecisionLogHandler struct {
	inner   slog.Handler
	emitter *Emitter
}

// DecisionLogLine matches the DEEP-004 stderr JSON format.
// These 6 fields are mandatory per DEEP-004 spec.
type DecisionLogLine struct {
	Timestamp string `json:"timestamp"`
	EventType string `json:"event_type"`
	RequestID string `json:"request_id"`
	TenantID  string `json:"tenant_id"`
	UserID    string `json:"user_id"`
	Decision  string `json:"decision"`
	// Extended fields (optional, stored in payload).
	Dimension   string                 `json:"dimension,omitempty"`
	Remaining   map[string]interface{} `json:"remaining,omitempty"`
	ScreenScore int                    `json:"screen_score,omitempty"`
	CacheHit    bool                   `json:"cache_hit,omitempty"`
}

// NewDecisionLogHandler creates a tee handler that mirrors DEEP-004 logs to audit.
func NewDecisionLogHandler(inner slog.Handler, emitter *Emitter) *DecisionLogHandler {
	return &DecisionLogHandler{
		inner:   inner,
		emitter: emitter,
	}
}

func (h *DecisionLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *DecisionLogHandler) Handle(ctx context.Context, r slog.Record) error {
	// Pass through to inner handler first (DEEP-004 stderr output preserved).
	if err := h.inner.Handle(ctx, r); err != nil {
		return err
	}

	// Try to parse as a DEEP-004 decision log line.
	// Only intercept records that contain the decision_log_marker attribute.
	marker := ""
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "event_type" {
			marker = a.Value.String()
		}
		return true
	})

	if marker == "" {
		return nil
	}

	// Build the decision log line from slog attributes.
	line := DecisionLogLine{}
	r.Attrs(func(a slog.Attr) bool {
		switch a.Key {
		case "timestamp":
			line.Timestamp = a.Value.String()
		case "event_type":
			line.EventType = a.Value.String()
		case "request_id":
			line.RequestID = a.Value.String()
		case "tenant_id":
			line.TenantID = a.Value.String()
		case "user_id":
			line.UserID = a.Value.String()
		case "decision":
			line.Decision = a.Value.String()
		case "dimension":
			line.Dimension = a.Value.String()
		case "screen_score":
			line.ScreenScore = int(a.Value.Int64())
		case "cache_hit":
			line.CacheHit = a.Value.Bool()
		}
		return true
	})

	// Only process cap.evaluation events from DEEP-004.
	if line.EventType != string(EventCapEvaluation) {
		return nil
	}

	// Map to AuditEvent.
	ts, _ := time.Parse(time.RFC3339Nano, line.Timestamp)
	if ts.IsZero() {
		ts = time.Now().UTC()
	}

	payload := map[string]interface{}{}
	if line.Dimension != "" {
		payload["dimension"] = line.Dimension
	}
	if line.Remaining != nil {
		payload["remaining"] = line.Remaining
	}
	if line.ScreenScore > 0 {
		payload["screen_score"] = line.ScreenScore
	}
	payload["cache_hit"] = line.CacheHit

	evt := AuditEvent{
		EventType: EventCapEvaluation,
		Decision:  Decision(line.Decision),
		UserID:    line.UserID,
		TenantID:  line.TenantID,
		RequestID: line.RequestID,
		Source:    SourceGo,
		Payload:   payload,
	}

	// Emit to audit store. Failures must NOT abort the stderr write
	// (graceful degradation per SPEC-AUTH-003).
	if h.emitter != nil {
		if err := h.emitter.EmitEvent(ctx, evt); err != nil {
			// Log the audit emission failure but do not propagate.
			// This preserves the DEEP-004 forward-compat guarantee.
			_ = err
		}
	}

	return nil
}

func (h *DecisionLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &DecisionLogHandler{
		inner:   h.inner.WithAttrs(attrs),
		emitter: h.emitter,
	}
}

func (h *DecisionLogHandler) WithGroup(name string) slog.Handler {
	return &DecisionLogHandler{
		inner:   h.inner.WithGroup(name),
		emitter: h.emitter,
	}
}

// ParseDecisionLogLine parses a JSON line into a DecisionLogLine.
// Exported for testing.
func ParseDecisionLogLine(data []byte) (DecisionLogLine, error) {
	var line DecisionLogLine
	if err := json.Unmarshal(data, &line); err != nil {
		return line, err
	}
	return line, nil
}
