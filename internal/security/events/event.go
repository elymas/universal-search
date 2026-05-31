// Package events implements the SPEC-SEC-001 security event taxonomy.
//
// REQ-SEC-017: this package does NOT implement a new hash chain, audit table,
// or verify job. It maps the 7-type security taxonomy onto the EXISTING
// SPEC-AUTH-003 audit EventType constants and emits each event through the
// EXISTING audit.Emitter. Chain integrity, append-only storage, and the daily
// chain_verify job are all provided by internal/audit and are reused unchanged.
//
// The genuinely-new delta is: (1) the 7-type taxonomy + mapping, (2) the
// usearch_security_event_total{type, severity} metric, (3) severity → slog
// level mapping.
package events

import (
	"context"
	"log/slog"

	"github.com/elymas/universal-search/internal/audit"
)

// Type is one of the seven SPEC-SEC-001 security event types.
type Type string

const (
	TypeAuthFailed        Type = "auth.failed"
	TypeAuthSuccess       Type = "auth.success"
	TypeSSRFBlocked       Type = "ssrf.blocked"
	TypeSecretScanFinding Type = "secret.scan.finding"
	TypeRateLimitExceeded Type = "ratelimit.exceeded"
	TypeRBACDenied        Type = "rbac.denied"
	TypePromptSanitized   Type = "prompt.sanitized"
)

// Severity classifies the event for metric labelling and slog level mapping.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
)

// AllTypes returns the seven security event types (taxonomy completeness).
func AllTypes() []Type {
	return []Type{
		TypeAuthFailed, TypeAuthSuccess, TypeSSRFBlocked, TypeSecretScanFinding,
		TypeRateLimitExceeded, TypeRBACDenied, TypePromptSanitized,
	}
}

// auditEventType maps a SEC-001 Type onto the AUTH-003 audit.EventType.
// Three types reuse existing AUTH-003 constants; four use the new SEC-001
// constants added to internal/audit/types.go (coordinated with AUTH-003 owner).
var auditEventType = map[Type]audit.EventType{
	TypeAuthFailed:        audit.EventAuthFail,
	TypeAuthSuccess:       audit.EventAuthLogin,
	TypeRBACDenied:        audit.EventRBACDeny,
	TypeSSRFBlocked:       audit.EventSecuritySSRFBlocked,
	TypeSecretScanFinding: audit.EventSecuritySecretFinding,
	TypeRateLimitExceeded: audit.EventSecurityRateLimit,
	TypePromptSanitized:   audit.EventSecurityPromptSanitized,
}

// AuditEventType resolves the AUTH-003 audit.EventType for a SEC-001 Type, or
// returns false if the type is unknown.
func AuditEventType(t Type) (audit.EventType, bool) {
	et, ok := auditEventType[t]
	return et, ok
}

// auditDecision maps a Type to the AUTH-003 Decision. Denials/blocks/failures
// are DecisionDeny; successes are DecisionAllow; the rest are DecisionNone.
func auditDecision(t Type) audit.Decision {
	switch t {
	case TypeAuthFailed, TypeSSRFBlocked, TypeRateLimitExceeded, TypeRBACDenied:
		return audit.DecisionDeny
	case TypeAuthSuccess:
		return audit.DecisionAllow
	default: // TypeSecretScanFinding, TypePromptSanitized
		return audit.DecisionNone
	}
}

// MetricRecorder is the minimal interface the events package needs from the
// metrics registry — kept local to avoid an events -> obs/metrics import.
// Satisfied by *metrics.SecurityCollectors.
type MetricRecorder interface {
	RecordSecurityEvent(eventType, severity string)
}

// Event is a single security event to emit.
type Event struct {
	Type     Type
	Severity Severity
	// Audit carries the AUTH-003 fields (tenant, user, request id, payload).
	// EventType/Decision are set by Emit from Type — callers must not set them.
	Audit audit.AuditEvent
}

// Emitter funnels SEC-001 security events into the existing AUTH-003 audit
// subsystem, increments the security metric, and emits a leveled slog record.
//
// @MX:ANCHOR: [AUTO] SEC-001 security event funnel; reused by SSRF, ratelimit,
// prompt-sanitize, auth, and RBAC emission sites.
// @MX:REASON: fan_in >= 3 across security subsystems; delegates persistence to
// the AUTH-003 emitter (no new chain) — a regression here breaks the audit trail.
// @MX:SPEC: SPEC-SEC-001
type Emitter struct {
	audit   *audit.Emitter
	metrics MetricRecorder
	log     *slog.Logger
}

// NewEmitter constructs a security event Emitter. auditEmitter is the EXISTING
// AUTH-003 emitter (required); metrics and log are optional (nil-safe).
func NewEmitter(auditEmitter *audit.Emitter, mr MetricRecorder, log *slog.Logger) *Emitter {
	return &Emitter{audit: auditEmitter, metrics: mr, log: log}
}

// Emit records a security event: it resolves the AUTH-003 EventType + Decision,
// hands the event to the existing audit emitter (which writes prev_hash/
// this_hash when the chain is enabled), increments the security_event_total
// metric, and logs at the severity-mapped slog level.
func (e *Emitter) Emit(ctx context.Context, ev Event) error {
	et, ok := AuditEventType(ev.Type)
	if !ok {
		// Unknown type — refuse rather than poison the bounded metric/audit.
		if e.log != nil {
			e.log.Error("security: unknown event type", "type", string(ev.Type))
		}
		return nil
	}

	// Metric first (cheap, never fails); bounded label values only.
	if e.metrics != nil {
		e.metrics.RecordSecurityEvent(string(ev.Type), string(ev.Severity))
	}

	// slog at the severity-mapped level.
	e.logEvent(ev)

	// Persist into the EXISTING AUTH-003 chain via its emitter.
	if e.audit == nil {
		return nil
	}
	a := ev.Audit
	a.EventType = et
	a.Decision = auditDecision(ev.Type)
	return e.audit.EmitEvent(ctx, a)
}

// logEvent emits a structured log at the level mapped from severity:
// critical → ERROR, high → WARN, medium/low → INFO.
func (e *Emitter) logEvent(ev Event) {
	if e.log == nil {
		return
	}
	attrs := []any{"type", string(ev.Type), "severity", string(ev.Severity)}
	switch ev.Severity {
	case SeverityCritical:
		e.log.Error("security event", attrs...)
	case SeverityHigh:
		e.log.Warn("security event", attrs...)
	default:
		e.log.Info("security event", attrs...)
	}
}
