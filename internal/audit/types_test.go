package audit

import (
	"testing"
)

// TestEventTypeBoundedEnum verifies that only predefined event types are valid.
// NFR-AUTH3-008: cardinality safety — startup-locked enums.
// REQ-AUTH3-002: 11 categories x ~20 event_types.
func TestEventTypeBoundedEnum(t *testing.T) {
	tests := []struct {
		name      string
		eventType EventType
		wantValid bool
	}{
		{"auth_login is valid", EventAuthLogin, true},
		{"auth_logout is valid", EventAuthLogout, true},
		{"auth_fail is valid", EventAuthFail, true},
		{"rbac_allow is valid", EventRBACAllow, true},
		{"rbac_deny is valid", EventRBACDeny, true},
		{"query_submit is valid", EventQuerySubmit, true},
		{"query_complete is valid", EventQueryComplete, true},
		{"deep_start is valid", EventDeepStart, true},
		{"cap_evaluation is valid", EventCapEvaluation, true},
		{"cost_recorded is valid", EventCostRecorded, true},
		{"cost_reconciled is valid", EventCostReconciled, true},
		{"index_write is valid", EventIndexWrite, true},
		{"admin_replay is valid", EventAdminReplay, true},
		{"audit_export is valid", EventAuditExport, true},
		{"audit_partition_drop is valid", EventAuditPartitionDrop, true},
		{"unknown type is invalid", EventType("unknown.event"), false},
		{"empty type is invalid", EventType(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.eventType.IsValid()
			if got != tt.wantValid {
				t.Errorf("EventType(%q).IsValid() = %v, want %v", tt.eventType, got, tt.wantValid)
			}
		})
	}
}

// TestDecisionBoundedEnum verifies that only predefined decisions are valid.
func TestDecisionBoundedEnum(t *testing.T) {
	tests := []struct {
		name      string
		decision  Decision
		wantValid bool
	}{
		{"allow is valid", DecisionAllow, true},
		{"deny is valid", DecisionDeny, true},
		{"none is valid", DecisionNone, true},
		{"invalid decision", Decision("maybe"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.decision.IsValid()
			if got != tt.wantValid {
				t.Errorf("Decision(%q).IsValid() = %v, want %v", tt.decision, got, tt.wantValid)
			}
		})
	}
}

// TestSourceBoundedEnum verifies source enum.
func TestSourceBoundedEnum(t *testing.T) {
	tests := []struct {
		name      string
		source    Source
		wantValid bool
	}{
		{"go source is valid", SourceGo, true},
		{"python source is valid", SourcePython, true},
		{"trigger source is valid", SourceTrigger, true},
		{"invalid source", Source("ruby"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.source.IsValid()
			if got != tt.wantValid {
				t.Errorf("Source(%q).IsValid() = %v, want %v", tt.source, got, tt.wantValid)
			}
		})
	}
}

// TestAllEventTypes_returnsKnownCount verifies the total number of registered event types.
func TestAllEventTypes_returnsKnownCount(t *testing.T) {
	all := AllEventTypes()
	if len(all) < 15 {
		t.Errorf("AllEventTypes() returned %d types, want at least 15", len(all))
	}
	// Verify all returned types are valid.
	for _, et := range all {
		if !et.IsValid() {
			t.Errorf("AllEventTypes() contains invalid type: %q", et)
		}
	}
}

// TestAuditEvent_fields verifies AuditEvent struct field mapping.
func TestAuditEvent_fields(t *testing.T) {
	evt := AuditEvent{
		EventType:  EventAuthLogin,
		Decision:   DecisionAllow,
		UserID:     "alice@example.com",
		TenantID:   "default",
		TeamID:     "engineering",
		RequestID:  "req_001",
		Source:     SourceGo,
		Resource:   "auth:login",
		Action:     "login",
		IP:         "192.168.1.1",
		Payload:    map[string]interface{}{"method": "oidc"},
	}

	if evt.EventType != EventAuthLogin {
		t.Errorf("EventType = %q, want %q", evt.EventType, EventAuthLogin)
	}
	if evt.Decision != DecisionAllow {
		t.Errorf("Decision = %q, want %q", evt.Decision, DecisionAllow)
	}
	if evt.UserID != "alice@example.com" {
		t.Errorf("UserID = %q, want %q", evt.UserID, "alice@example.com")
	}
	if evt.Source != SourceGo {
		t.Errorf("Source = %q, want %q", evt.Source, SourceGo)
	}
}

// TestReplayableEventTypes verifies which events are replayable.
// REQ-AUTH3-004: only query.submit, deep.start are replayable.
func TestReplayableEventTypes(t *testing.T) {
	tests := []struct {
		name          string
		eventType     EventType
		wantReplayable bool
	}{
		{"query.submit is replayable", EventQuerySubmit, true},
		{"deep.start is replayable", EventDeepStart, true},
		{"auth.login is not replayable", EventAuthLogin, false},
		{"rbac.deny is not replayable", EventRBACDeny, false},
		{"admin.replay is not replayable", EventAdminReplay, false},
		{"cost.recorded is not replayable", EventCostRecorded, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.eventType.IsReplayable()
			if got != tt.wantReplayable {
				t.Errorf("EventType(%q).IsReplayable() = %v, want %v", tt.eventType, got, tt.wantReplayable)
			}
		})
	}
}
