// Package audit provides the immutable audit trail subsystem for Universal Search.
// SPEC-AUTH-003: audit_events table, EmitEvent funnel, query replay, S3 export,
// LiteLLM cost reconciliation, PII masking, and hash chain.
//
// @MX:NOTE: [AUTO] DEEP-004 forward-compat per spec.md section 1.3. DEEP-004 code and
// cost_ledger schema remain unchanged. This package absorbs DEEP-004 stderr JSON lines
// into structured storage via a slog tee handler.
package audit

// EventType enumerates all audit event types. Adding a new type requires SPEC amendment.
// D1: 11 categories x ~20 event_types, startup enum lock.
// NFR-AUTH3-008: cardinality safety.
type EventType string

const (
	// Auth category (AUTH-001).
	EventAuthLogin  EventType = "auth.login"
	EventAuthLogout EventType = "auth.logout"
	EventAuthFail   EventType = "auth.fail"

	// RBAC category (AUTH-002).
	EventRBACAllow        EventType = "rbac.allow"
	EventRBACDeny         EventType = "rbac.deny"
	EventRBACPolicyChange EventType = "rbac.policy_change"

	// Query category (synthesis handler).
	EventQuerySubmit   EventType = "query.submit"
	EventQueryComplete EventType = "query.complete"
	EventQueryFail     EventType = "query.fail"

	// Deep category (DEEP pipeline).
	EventDeepStart    EventType = "deep.start"
	EventDeepComplete EventType = "deep.complete"
	EventDeepFail     EventType = "deep.fail"

	// Cost category (DEEP-004 / LiteLLM).
	EventCapEvaluation  EventType = "cap.evaluation"
	EventCostRecorded   EventType = "cost.recorded"
	EventCostReconciled EventType = "cost.reconciled"

	// Index category (IDX-001).
	EventIndexWrite  EventType = "index.write"
	EventIndexDelete EventType = "index.delete"

	// Admin category.
	EventAdminReplay       EventType = "admin.replay"
	EventAdminConfigChange EventType = "admin.config_change"

	// System category (audit lifecycle).
	EventAuditExport        EventType = "audit.export"
	EventAuditPartitionDrop EventType = "audit.partition_drop"

	// Security category (SPEC-SEC-001 REQ-SEC-017). These four constants are
	// the genuinely-new delta of SEC-001's 7-type taxonomy; the other three
	// types (auth.failed, auth.success, rbac.denied) reuse the existing
	// EventAuthFail / EventAuthLogin / EventRBACDeny constants.
	//
	// @MX:NOTE: [AUTO] Cross-SPEC: these EventType constants extend the AUTH-003
	// enum lock (AllEventTypes + allEventTypesSet). Activation of any fail-closed
	// audit-write lockdown for these types requires AUTH-003 owner sign-off
	// (Phase 5 coordination gate, plan.md). Adding constants here is the agreed
	// scope; lockdown stays default OFF.
	EventSecuritySSRFBlocked     EventType = "ssrf.blocked"
	EventSecuritySecretFinding   EventType = "secret.scan.finding"
	EventSecurityRateLimit       EventType = "ratelimit.exceeded"
	EventSecurityPromptSanitized EventType = "prompt.sanitized"
)

// Decision enumerates the outcome of the audited action.
type Decision string

const (
	DecisionAllow Decision = "allow"
	DecisionDeny  Decision = "deny"
	DecisionNone  Decision = "none"
)

// Source identifies where the event originated.
type Source string

const (
	SourceGo      Source = "go"
	SourcePython  Source = "python"
	SourceTrigger Source = "trigger"
)

// AuditEvent is the canonical struct for all audit emissions.
// REQ-AUTH3-002: single EmitEvent funnel accepts this struct.
type AuditEvent struct {
	EventType EventType              `json:"event_type"`
	Decision  Decision               `json:"decision"`
	UserID    string                 `json:"user_id"`
	TenantID  string                 `json:"tenant_id"`
	TeamID    string                 `json:"team_id"`
	RequestID string                 `json:"request_id"`
	Source    Source                 `json:"source"`
	Resource  string                 `json:"resource,omitempty"`
	Action    string                 `json:"action,omitempty"`
	IP        string                 `json:"ip,omitempty"`
	Payload   map[string]interface{} `json:"payload,omitempty"`

	// Hash chain fields (populated when hash_chain.enabled).
	PrevHash string `json:"prev_hash,omitempty"`
	ThisHash string `json:"this_hash,omitempty"`
}

// IsValid returns true if the EventType is a known registered type.
func (et EventType) IsValid() bool {
	return allEventTypesSet[et]
}

// IsReplayable returns true if the event type supports query replay.
// REQ-AUTH3-004: only query.submit and deep.start are replayable.
func (et EventType) IsReplayable() bool {
	return replayableEventTypes[et]
}

// IsValid returns true if the Decision is a known value.
func (d Decision) IsValid() bool {
	switch d {
	case DecisionAllow, DecisionDeny, DecisionNone:
		return true
	}
	return false
}

// IsValid returns true if the Source is a known value.
func (s Source) IsValid() bool {
	switch s {
	case SourceGo, SourcePython, SourceTrigger:
		return true
	}
	return false
}

// allEventTypesSet is the lookup map for IsValid.
var allEventTypesSet = make(map[EventType]bool)

// replayableEventTypes defines which events can be replayed.
var replayableEventTypes = map[EventType]bool{
	EventQuerySubmit: true,
	EventDeepStart:   true,
}

func init() {
	for _, et := range AllEventTypes() {
		allEventTypesSet[et] = true
	}
}

// AllEventTypes returns all registered event types.
func AllEventTypes() []EventType {
	return []EventType{
		EventAuthLogin,
		EventAuthLogout,
		EventAuthFail,
		EventRBACAllow,
		EventRBACDeny,
		EventRBACPolicyChange,
		EventQuerySubmit,
		EventQueryComplete,
		EventQueryFail,
		EventDeepStart,
		EventDeepComplete,
		EventDeepFail,
		EventCapEvaluation,
		EventCostRecorded,
		EventCostReconciled,
		EventIndexWrite,
		EventIndexDelete,
		EventAdminReplay,
		EventAdminConfigChange,
		EventAuditExport,
		EventAuditPartitionDrop,
		// Security category (SPEC-SEC-001 REQ-SEC-017).
		EventSecuritySSRFBlocked,
		EventSecuritySecretFinding,
		EventSecurityRateLimit,
		EventSecurityPromptSanitized,
	}
}
